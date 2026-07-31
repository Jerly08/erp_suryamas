package database

import (
	"strings"
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestStripLeadingSQLComments(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "line comments before create table",
			input: "-- Preserve audit history\n-- Schema-only migration\nCREATE TABLE audit_lines (id BIGINT)",
			want:  "CREATE TABLE audit_lines (id BIGINT)",
		},
		{
			name:  "mixed comments before create index",
			input: "/* Initial description */\n-- Keep this index unique\nCREATE INDEX idx_audit_lines ON audit_lines(id)",
			want:  "CREATE INDEX idx_audit_lines ON audit_lines(id)",
		},
		{
			name:  "comments only",
			input: "-- no SQL follows\n/* still no SQL */",
			want:  "",
		},
		{
			name:  "sql without comments",
			input: "  ALTER TABLE purchases ADD COLUMN note TEXT  ",
			want:  "ALTER TABLE purchases ADD COLUMN note TEXT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, stripLeadingSQLComments(tt.input))
		})
	}
}

func TestSyncStandardPurchaseApprovalWorkflowReroutesLegacyFinanceStaffAction(t *testing.T) {
	db := newAutoMigrationWorkflowTestDB(t)
	workflow, steps := createLegacyStandardPurchaseApprovalWorkflow(t, db)
	request := createLegacyPurchaseApprovalRequest(t, db, workflow.ID)

	require.NoError(t, db.Create(&[]models.ApprovalAction{
		{RequestID: request.ID, StepID: steps.employee.ID, Status: models.ApprovalStatusApproved, IsActive: false},
		{RequestID: request.ID, StepID: steps.financeStaff.ID, Status: models.ApprovalStatusPending, IsActive: true},
		{RequestID: request.ID, StepID: steps.financeManager.ID, Status: models.ApprovalStatusPending, IsActive: false},
		{RequestID: request.ID, StepID: steps.director.ID, Status: models.ApprovalStatusPending, IsActive: false},
	}).Error)

	require.NoError(t, syncStandardPurchaseApprovalWorkflowSteps(db, workflow.ID))

	var financeStep models.ApprovalStep
	require.NoError(t, db.Where("workflow_id = ? AND step_order = ? AND approver_role = ?", workflow.ID, 2, "finance").
		First(&financeStep).Error)

	var activeAction models.ApprovalAction
	require.NoError(t, db.Where("request_id = ? AND status = ? AND is_active = ?", request.ID, models.ApprovalStatusPending, true).
		First(&activeAction).Error)
	require.Equal(t, financeStep.ID, activeAction.StepID)

	var duplicateFinanceAction models.ApprovalAction
	require.NoError(t, db.Where("request_id = ? AND step_id = ?", request.ID, steps.financeManager.ID).
		First(&duplicateFinanceAction).Error)
	require.Equal(t, models.ApprovalActionSkipped, duplicateFinanceAction.Status)
	require.False(t, duplicateFinanceAction.IsActive)

	var pendingStaffActionCount int64
	require.NoError(t, db.Model(&models.ApprovalAction{}).
		Where("step_id = ? AND status = ?", steps.financeStaff.ID, models.ApprovalStatusPending).
		Count(&pendingStaffActionCount).Error)
	require.Equal(t, int64(0), pendingStaffActionCount)
}

func TestSyncStandardPurchaseApprovalWorkflowPreservesActiveLegacyFinanceManagerAction(t *testing.T) {
	db := newAutoMigrationWorkflowTestDB(t)
	workflow, steps := createLegacyStandardPurchaseApprovalWorkflow(t, db)
	request := createLegacyPurchaseApprovalRequest(t, db, workflow.ID)

	require.NoError(t, db.Create(&[]models.ApprovalAction{
		{RequestID: request.ID, StepID: steps.employee.ID, Status: models.ApprovalStatusApproved, IsActive: false},
		{RequestID: request.ID, StepID: steps.financeStaff.ID, Status: models.ApprovalStatusApproved, IsActive: false},
		{RequestID: request.ID, StepID: steps.financeManager.ID, Status: models.ApprovalStatusPending, IsActive: true},
		{RequestID: request.ID, StepID: steps.director.ID, Status: models.ApprovalStatusPending, IsActive: false},
	}).Error)

	require.NoError(t, syncStandardPurchaseApprovalWorkflowSteps(db, workflow.ID))

	var activeAction models.ApprovalAction
	require.NoError(t, db.Where("request_id = ? AND status = ? AND is_active = ?", request.ID, models.ApprovalStatusPending, true).
		First(&activeAction).Error)
	require.Equal(t, steps.financeManager.ID, activeAction.StepID)

	var legacyFinanceStep models.ApprovalStep
	require.NoError(t, db.First(&legacyFinanceStep, steps.financeManager.ID).Error)
	require.Equal(t, "finance", legacyFinanceStep.ApproverRole)
	require.GreaterOrEqual(t, legacyFinanceStep.StepOrder, 999)
}

func TestSyncStandardPurchaseApprovalWorkflowRepairsUnrequestedDirectorApproval(t *testing.T) {
	db := newAutoMigrationWorkflowTestDB(t)
	workflow, purchase, request, directorAction, managerID, approvedAt := createPendingDirectorPurchaseApproval(t, db, false)

	require.NoError(t, syncStandardPurchaseApprovalWorkflowSteps(db, workflow.ID))

	var updatedPurchase models.Purchase
	require.NoError(t, db.First(&updatedPurchase, purchase.ID).Error)
	require.Equal(t, models.PurchaseStatusPendingAccounting, updatedPurchase.Status)
	require.Equal(t, models.PurchaseApprovalApproved, updatedPurchase.ApprovalStatus)
	require.NotNil(t, updatedPurchase.ApprovedBy)
	require.Equal(t, managerID, *updatedPurchase.ApprovedBy)
	require.NotNil(t, updatedPurchase.ApprovedAt)
	require.WithinDuration(t, approvedAt, *updatedPurchase.ApprovedAt, time.Second)

	var updatedRequest models.ApprovalRequest
	require.NoError(t, db.First(&updatedRequest, request.ID).Error)
	require.Equal(t, models.ApprovalStatusApproved, updatedRequest.Status)
	require.NotNil(t, updatedRequest.CompletedAt)

	var updatedDirectorAction models.ApprovalAction
	require.NoError(t, db.First(&updatedDirectorAction, directorAction.ID).Error)
	require.Equal(t, models.ApprovalStatusPending, updatedDirectorAction.Status)
	require.False(t, updatedDirectorAction.IsActive)
	require.Contains(t, updatedDirectorAction.Comments, "Skipped unrequested optional Director")
}

func TestSyncStandardPurchaseApprovalWorkflowKeepsExplicitDirectorEscalation(t *testing.T) {
	db := newAutoMigrationWorkflowTestDB(t)
	workflow, purchase, request, directorAction, _, _ := createPendingDirectorPurchaseApproval(t, db, true)

	require.NoError(t, syncStandardPurchaseApprovalWorkflowSteps(db, workflow.ID))

	var updatedPurchase models.Purchase
	require.NoError(t, db.First(&updatedPurchase, purchase.ID).Error)
	require.Equal(t, models.PurchaseStatusPending, updatedPurchase.Status)
	require.Equal(t, models.PurchaseApprovalPending, updatedPurchase.ApprovalStatus)

	var updatedRequest models.ApprovalRequest
	require.NoError(t, db.First(&updatedRequest, request.ID).Error)
	require.Equal(t, models.ApprovalStatusPending, updatedRequest.Status)
	require.Nil(t, updatedRequest.CompletedAt)

	var updatedDirectorAction models.ApprovalAction
	require.NoError(t, db.First(&updatedDirectorAction, directorAction.ID).Error)
	require.Equal(t, models.ApprovalStatusPending, updatedDirectorAction.Status)
	require.True(t, updatedDirectorAction.IsActive)
}

type legacyPurchaseApprovalSteps struct {
	employee       models.ApprovalStep
	financeStaff   models.ApprovalStep
	financeManager models.ApprovalStep
	director       models.ApprovalStep
}

func newAutoMigrationWorkflowTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Purchase{},
		&models.ApprovalWorkflow{},
		&models.ApprovalStep{},
		&models.ApprovalRequest{},
		&models.ApprovalAction{},
		&models.ApprovalHistory{},
	))

	return db
}

func createLegacyStandardPurchaseApprovalWorkflow(t *testing.T, db *gorm.DB) (models.ApprovalWorkflow, legacyPurchaseApprovalSteps) {
	t.Helper()

	workflow := models.ApprovalWorkflow{
		Name:            "Standard Purchase Approval",
		Module:          models.ApprovalModulePurchase,
		MinAmount:       0,
		MaxAmount:       999999999999,
		IsActive:        true,
		RequireDirector: false,
		RequireFinance:  true,
	}
	require.NoError(t, db.Create(&workflow).Error)

	steps := legacyPurchaseApprovalSteps{
		employee: models.ApprovalStep{
			WorkflowID:   workflow.ID,
			StepOrder:    1,
			StepName:     "Employee Submission",
			ApproverRole: "employee",
			IsOptional:   false,
			TimeLimit:    24,
		},
		financeStaff: models.ApprovalStep{
			WorkflowID:   workflow.ID,
			StepOrder:    2,
			StepName:     "Finance Staff Review",
			ApproverRole: "purchase_finance_staff",
			IsOptional:   false,
			TimeLimit:    48,
		},
		financeManager: models.ApprovalStep{
			WorkflowID:   workflow.ID,
			StepOrder:    3,
			StepName:     "Finance Manager Approval",
			ApproverRole: "finance",
			IsOptional:   false,
			TimeLimit:    48,
		},
		director: models.ApprovalStep{
			WorkflowID:   workflow.ID,
			StepOrder:    4,
			StepName:     "Director Approval",
			ApproverRole: "director",
			IsOptional:   true,
			TimeLimit:    72,
		},
	}
	require.NoError(t, db.Create(&steps.employee).Error)
	require.NoError(t, db.Create(&steps.financeStaff).Error)
	require.NoError(t, db.Create(&steps.financeManager).Error)
	require.NoError(t, db.Create(&steps.director).Error)

	return workflow, steps
}

func createLegacyPurchaseApprovalRequest(t *testing.T, db *gorm.DB, workflowID uint) models.ApprovalRequest {
	t.Helper()

	request := models.ApprovalRequest{
		RequestCode:    strings.ReplaceAll(t.Name(), "/", "-"),
		WorkflowID:     workflowID,
		RequesterID:    1,
		EntityType:     models.EntityTypePurchase,
		EntityID:       1,
		Amount:         100000,
		Status:         models.ApprovalStatusPending,
		Priority:       models.ApprovalPriorityNormal,
		RequestTitle:   "Purchase Approval Test",
		RequestMessage: "Purchase approval migration test",
	}
	require.NoError(t, db.Create(&request).Error)

	return request
}

func createPendingDirectorPurchaseApproval(t *testing.T, db *gorm.DB, withEscalationHistory bool) (models.ApprovalWorkflow, models.Purchase, models.ApprovalRequest, models.ApprovalAction, uint, time.Time) {
	t.Helper()

	requester := models.User{
		Username: "migration-requester-" + strings.ReplaceAll(t.Name(), "/", "-"),
		Email:    "migration-requester@test.local",
		Password: "secret",
		Role:     models.RoleEmployee,
		IsActive: true,
	}
	manager := models.User{
		Username: "migration-manager-" + strings.ReplaceAll(t.Name(), "/", "-"),
		Email:    "migration-manager@test.local",
		Password: "secret",
		Role:     models.RoleFinanceManager,
		IsActive: true,
	}
	vendor := models.Contact{
		Code:     "V-" + strings.ReplaceAll(t.Name(), "/", "-"),
		Name:     "Migration Vendor",
		Type:     models.ContactTypeVendor,
		IsActive: true,
	}
	require.NoError(t, db.Create(&requester).Error)
	require.NoError(t, db.Create(&manager).Error)
	require.NoError(t, db.Create(&vendor).Error)

	workflow := models.ApprovalWorkflow{
		Name:            "Standard Purchase Approval",
		Module:          models.ApprovalModulePurchase,
		MinAmount:       0,
		MaxAmount:       999999999999,
		IsActive:        true,
		RequireDirector: false,
		RequireFinance:  true,
	}
	require.NoError(t, db.Create(&workflow).Error)
	steps := standardPurchaseApprovalSteps(workflow.ID)
	require.NoError(t, db.Create(&steps).Error)

	var employeeStep models.ApprovalStep
	require.NoError(t, db.Where("workflow_id = ? AND step_order = ?", workflow.ID, 1).First(&employeeStep).Error)
	var financeStep models.ApprovalStep
	require.NoError(t, db.Where("workflow_id = ? AND step_order = ?", workflow.ID, 2).First(&financeStep).Error)
	var directorStep models.ApprovalStep
	require.NoError(t, db.Where("workflow_id = ? AND step_order = ?", workflow.ID, 3).First(&directorStep).Error)

	purchase := models.Purchase{
		Code:           "PO-MIG-" + strings.ReplaceAll(t.Name(), "/", "-"),
		VendorID:       vendor.ID,
		UserID:         requester.ID,
		Date:           time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC),
		DueDate:        time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
		TotalAmount:    125000,
		Status:         models.PurchaseStatusPending,
		ApprovalStatus: models.PurchaseApprovalPending,
	}
	require.NoError(t, db.Create(&purchase).Error)

	request := models.ApprovalRequest{
		RequestCode:    "REQ-" + strings.ReplaceAll(t.Name(), "/", "-"),
		WorkflowID:     workflow.ID,
		RequesterID:    requester.ID,
		EntityType:     models.EntityTypePurchase,
		EntityID:       purchase.ID,
		Amount:         purchase.TotalAmount,
		Status:         models.ApprovalStatusPending,
		Priority:       models.ApprovalPriorityNormal,
		RequestTitle:   "Purchase Approval Migration Test",
		RequestMessage: "Purchase approval migration test",
	}
	require.NoError(t, db.Create(&request).Error)

	purchase.ApprovalRequestID = &request.ID
	require.NoError(t, db.Save(&purchase).Error)

	approvedAt := time.Date(2026, 6, 24, 12, 42, 8, 0, time.UTC)
	actions := []models.ApprovalAction{
		{RequestID: request.ID, StepID: employeeStep.ID, ApproverID: &requester.ID, Status: models.ApprovalStatusApproved, ActionDate: &approvedAt, IsActive: false},
		{RequestID: request.ID, StepID: financeStep.ID, ApproverID: &manager.ID, Status: models.ApprovalStatusApproved, ActionDate: &approvedAt, IsActive: false},
		{RequestID: request.ID, StepID: directorStep.ID, Status: models.ApprovalStatusPending, IsActive: true},
	}
	require.NoError(t, db.Create(&actions).Error)

	if withEscalationHistory {
		require.NoError(t, db.Create(&models.ApprovalHistory{
			RequestID: request.ID,
			UserID:    manager.ID,
			Action:    "ESCALATED_TO_DIRECTOR",
			Comments:  "Requires Director approval as requested by Finance",
			Metadata:  "{}",
		}).Error)
	}

	return workflow, purchase, request, actions[2], manager.ID, approvedAt
}
