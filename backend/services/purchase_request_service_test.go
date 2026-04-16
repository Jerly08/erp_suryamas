package services

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newPurchaseRequestTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Account{},
		&models.ProductCategory{},
		&models.WarehouseLocation{},
		&models.Contact{},
		&models.CashBank{},
		&models.ApprovalWorkflow{},
		&models.ApprovalStep{},
		&models.ApprovalRequest{},
		&models.Project{},
		&models.Product{},
		&models.Purchase{},
		&models.PurchaseRequest{},
		&models.PurchaseRequestItem{},
	))

	return db
}

func seedPurchaseRequestTestProduct(t *testing.T, db *gorm.DB, suffix string) models.Product {
	t.Helper()

	product := models.Product{
		Code:          "PRD-" + suffix,
		Name:          "Product " + suffix,
		Unit:          "pcs",
		PurchasePrice: 100000,
		CostPrice:     100000,
		SalePrice:     120000,
		IsActive:      true,
	}
	require.NoError(t, db.Create(&product).Error)

	return product
}

func seedPurchaseRequestTestUser(t *testing.T, db *gorm.DB, username string, role string) models.User {
	t.Helper()

	user := models.User{
		Username:  username,
		Email:     username + "@example.com",
		Password:  "hashed",
		FirstName: username,
		LastName:  "User",
		Role:      role,
		IsActive:  true,
	}
	require.NoError(t, db.Create(&user).Error)

	return user
}

func TestCreatePurchaseRequestByFinanceManagerStartsAtFinanceManagerStage(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	financeManager := seedPurchaseRequestTestUser(t, db, "finance-manager-pr", models.RoleFinanceManager)
	product := seedPurchaseRequestTestProduct(t, db, "FM")

	pr, err := service.CreatePurchaseRequest(models.PurchaseRequestCreatePayload{
		Department: "Finance",
		Items: []models.PurchaseRequestItemPayload{
			{
				ProductID:      product.ID,
				Quantity:       2,
				EstimatedPrice: 150000,
			},
		},
	}, financeManager.ID, financeManager.Role)
	require.NoError(t, err)

	require.Equal(t, models.PRStatusPendingApproval, pr.Status)
	require.Equal(t, models.PRApprovalStageFinanceManager, pr.CurrentApprovalStage)
	require.Nil(t, pr.FinanceStaffApprovedBy)
	require.Len(t, pr.Items, 1)
	require.Equal(t, models.PRItemDecisionPending, pr.Items[0].FinanceManagerDecision)
	require.Equal(t, models.PRItemDecisionPending, pr.Items[0].DirectorDecision)
}

func TestFinanceManagerReviewCanEscalatePurchaseRequestToDirector(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	financeManager := seedPurchaseRequestTestUser(t, db, "finance-manager-escalate", models.RoleFinanceManager)
	product := seedPurchaseRequestTestProduct(t, db, "ESC")

	pr, err := service.CreatePurchaseRequest(models.PurchaseRequestCreatePayload{
		Department: "Finance",
		Items: []models.PurchaseRequestItemPayload{
			{
				ProductID:      product.ID,
				Quantity:       1,
				EstimatedPrice: 250000,
			},
		},
	}, financeManager.ID, financeManager.Role)
	require.NoError(t, err)

	message, err := service.ReviewPurchaseRequest(pr.ID, financeManager.ID, financeManager.Role, models.PurchaseRequestReviewPayload{
		Items: []models.PurchaseRequestReviewItemPayload{
			{ItemID: pr.Items[0].ID, Decision: models.PRItemDecisionApproved},
		},
		EscalateToDirector: true,
	})
	require.NoError(t, err)
	require.Equal(t, "Review finance manager berhasil dikirim ke director", message)

	updated, err := service.GetPurchaseRequestByID(pr.ID, financeManager.ID, financeManager.Role)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusPendingApproval, updated.Status)
	require.Equal(t, models.PRApprovalStageDirector, updated.CurrentApprovalStage)
	require.NotNil(t, updated.FinanceManagerApprovedBy)
	require.Len(t, updated.Items, 1)
	require.Equal(t, models.PRItemDecisionApproved, updated.Items[0].FinanceManagerDecision)
	require.Equal(t, models.PRItemDecisionPending, updated.Items[0].ApprovalStatus)
	require.Equal(t, models.PRItemDecisionPending, updated.Items[0].DirectorDecision)
}

func TestDirectorReviewFinalizesEscalatedPurchaseRequest(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	financeManager := seedPurchaseRequestTestUser(t, db, "finance-manager-final", models.RoleFinanceManager)
	director := seedPurchaseRequestTestUser(t, db, "director-pr", models.RoleDirector)
	product := seedPurchaseRequestTestProduct(t, db, "DIR")

	pr, err := service.CreatePurchaseRequest(models.PurchaseRequestCreatePayload{
		Department: "Finance",
		Items: []models.PurchaseRequestItemPayload{
			{
				ProductID:      product.ID,
				Quantity:       3,
				EstimatedPrice: 175000,
			},
		},
	}, financeManager.ID, financeManager.Role)
	require.NoError(t, err)

	_, err = service.ReviewPurchaseRequest(pr.ID, financeManager.ID, financeManager.Role, models.PurchaseRequestReviewPayload{
		Items: []models.PurchaseRequestReviewItemPayload{
			{ItemID: pr.Items[0].ID, Decision: models.PRItemDecisionApproved},
		},
		EscalateToDirector: true,
	})
	require.NoError(t, err)

	message, err := service.ReviewPurchaseRequest(pr.ID, director.ID, director.Role, models.PurchaseRequestReviewPayload{
		Items: []models.PurchaseRequestReviewItemPayload{
			{ItemID: pr.Items[0].ID, Decision: models.PRItemDecisionApproved},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "Purchase request disetujui seluruhnya oleh director", message)

	updated, err := service.GetPurchaseRequestByID(pr.ID, director.ID, director.Role)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusApproved, updated.Status)
	require.Equal(t, models.PRApprovalStageFinalized, updated.CurrentApprovalStage)
	require.NotNil(t, updated.DirectorApprovedBy)
	require.NotNil(t, updated.ApprovedBy)
	require.Len(t, updated.Items, 1)
	require.Equal(t, models.PRItemDecisionApproved, updated.Items[0].DirectorDecision)
	require.Equal(t, models.PRItemDecisionApproved, updated.Items[0].ApprovalStatus)
}
