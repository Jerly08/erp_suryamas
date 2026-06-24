package database

import (
	"strings"
	"testing"

	"app-sistem-akuntansi/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

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
		&models.ApprovalWorkflow{},
		&models.ApprovalStep{},
		&models.ApprovalRequest{},
		&models.ApprovalAction{},
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
