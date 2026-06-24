package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPurchaseFinalApprovalQueueIsFinanceManagerOnly(t *testing.T) {
	handler := &PurchaseApprovalHandler{}
	purchases := []models.Purchase{
		{
			ID:             1,
			Code:           "PO-FINAL-QUEUE",
			Status:         models.PurchaseStatusPendingFinalApproval,
			ApprovalStatus: models.PurchaseApprovalApproved,
			TotalAmount:    100000,
		},
	}

	financeManagerItems := handler.filterPurchasesByApprovalAuthority(purchases, models.RoleFinanceManager, 10)
	require.Len(t, financeManagerItems, 1)
	require.Equal(t, "PO-FINAL-QUEUE", financeManagerItems[0].Code)

	for _, role := range []string{models.RoleFinanceStaff, models.RoleAccounting, models.RoleDirector, models.RoleAdmin, models.RoleEmployee} {
		t.Run(role, func(t *testing.T) {
			items := handler.filterPurchasesByApprovalAuthority(purchases, role, 10)
			require.Empty(t, items)
		})
	}
}

func TestFinanceManagerApprovePurchaseWithoutDirectorRoutesToAccounting(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := newPurchaseApprovalHandlerTestDB(t)
	handlerService := newPurchaseApprovalHandlerTestService(db)
	handler := NewPurchaseApprovalHandler(handlerService.purchaseService, handlerService.approvalService)

	requester := seedPurchaseApprovalHandlerTestUser(t, db, "handler-staff", models.RoleEmployee)
	manager := seedPurchaseApprovalHandlerTestUser(t, db, "handler-finance-manager", models.RoleFinanceManager)
	vendor := models.Contact{Code: "V-HANDLER", Name: "Handler Vendor", Type: models.ContactTypeVendor, IsActive: true}
	require.NoError(t, db.Create(&vendor).Error)
	liabilityAccount := models.Account{Code: "2101-HANDLER", Name: "Handler Payable", Type: models.AccountTypeLiability, IsActive: true}
	expenseAccount := models.Account{Code: "6101-HANDLER", Name: "Handler Expense", Type: models.AccountTypeExpense, IsActive: true}
	require.NoError(t, db.Create(&liabilityAccount).Error)
	require.NoError(t, db.Create(&expenseAccount).Error)

	zeroRate := 0.0
	purchase, err := handlerService.purchaseService.CreatePurchase(models.PurchaseCreateRequest{
		VendorID:        vendor.ID,
		Date:            time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC),
		DueDate:         time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
		PaymentMethod:   models.PurchasePaymentCredit,
		CreditAccountID: &liabilityAccount.ID,
		PPNRate:         &zeroRate,
		Items: []models.PurchaseItemRequest{
			{
				ItemType:         models.PurchaseItemTypeExpense,
				Description:      "Handler approval expense",
				Quantity:         1,
				UnitPrice:        125000,
				ExpenseAccountID: expenseAccount.ID,
			},
		},
	}, requester.ID)
	require.NoError(t, err)
	require.NoError(t, handlerService.purchaseService.SubmitForApproval(purchase.ID, requester.ID))

	body, err := json.Marshal(map[string]interface{}{
		"comments":             "Approved by FM without Director",
		"escalate_to_director": false,
	})
	require.NoError(t, err)

	router := gin.New()
	router.POST("/purchases/:id/approve", func(c *gin.Context) {
		c.Set("user_id", manager.ID)
		c.Set("role", manager.Role)
		handler.ApprovePurchase(c)
	})

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/purchases/%d/approve", purchase.ID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	updated, err := handlerService.purchaseService.GetPurchaseByID(purchase.ID)
	require.NoError(t, err)
	require.Equal(t, models.PurchaseStatusPendingAccounting, updated.Status)
	require.Equal(t, models.PurchaseApprovalApproved, updated.ApprovalStatus)

	directorAction := purchaseApprovalHandlerTestActionByRole(t, db, *updated.ApprovalRequestID, models.RoleDirector)
	require.Equal(t, models.ApprovalStatusPending, directorAction.Status)
	require.False(t, directorAction.IsActive)
	require.Nil(t, directorAction.ApproverID)
}

type purchaseApprovalHandlerTestService struct {
	purchaseService *services.PurchaseService
	approvalService *services.ApprovalService
}

func newPurchaseApprovalHandlerTestService(db *gorm.DB) purchaseApprovalHandlerTestService {
	approvalService := services.NewApprovalService(db)
	purchaseService := services.NewPurchaseService(
		db,
		repositories.NewPurchaseRepository(db),
		repositories.NewProductRepository(db),
		repositories.NewContactRepository(db),
		repositories.NewAccountRepository(db),
		approvalService,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	return purchaseApprovalHandlerTestService{
		purchaseService: purchaseService,
		approvalService: approvalService,
	}
}

func newPurchaseApprovalHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.ContactAddress{},
		&models.Project{},
		&models.Account{},
		&models.Settings{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
		&models.Product{},
		&models.PurchaseRequest{},
		&models.PurchaseRequestItem{},
		&models.Purchase{},
		&models.PurchaseItem{},
		&models.ApprovalWorkflow{},
		&models.ApprovalRequest{},
		&models.ApprovalStep{},
		&models.ApprovalAction{},
		&models.ApprovalHistory{},
	))

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
	steps := []models.ApprovalStep{
		{WorkflowID: workflow.ID, StepOrder: 1, StepName: "Employee Submission", ApproverRole: models.RoleEmployee, IsOptional: false},
		{WorkflowID: workflow.ID, StepOrder: 2, StepName: "Finance Manager Approval", ApproverRole: models.RoleFinance, IsOptional: false},
		{WorkflowID: workflow.ID, StepOrder: 3, StepName: "Director Approval", ApproverRole: models.RoleDirector, IsOptional: true},
	}
	require.NoError(t, db.Create(&steps).Error)

	return db
}

func seedPurchaseApprovalHandlerTestUser(t *testing.T, db *gorm.DB, username string, role string) models.User {
	t.Helper()
	user := models.User{
		Username:  username,
		Email:     username + "@test.local",
		Password:  "secret",
		Role:      role,
		FirstName: username,
		IsActive:  true,
	}
	require.NoError(t, db.Create(&user).Error)
	return user
}

func purchaseApprovalHandlerTestActionByRole(t *testing.T, db *gorm.DB, requestID uint, role string) models.ApprovalAction {
	t.Helper()

	var action models.ApprovalAction
	require.NoError(t, db.Preload("Step").
		Joins("JOIN approval_steps ON approval_steps.id = approval_actions.step_id").
		Where("approval_actions.request_id = ? AND approval_steps.approver_role = ?", requestID, role).
		First(&action).Error)
	return action
}
