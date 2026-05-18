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
		&models.PurchaseRequestDocument{},
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

func seedPurchaseRequestTestVendor(t *testing.T, db *gorm.DB, suffix string) models.Contact {
	t.Helper()

	vendor := models.Contact{
		Code:     "VEN-" + suffix,
		Name:     "Vendor " + suffix,
		Type:     models.ContactTypeVendor,
		IsActive: true,
	}
	require.NoError(t, db.Create(&vendor).Error)

	return vendor
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

func TestCreatePurchaseRequestWithVendor(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	requester := seedPurchaseRequestTestUser(t, db, "staff-pr-vendor", models.RoleEmployee)
	product := seedPurchaseRequestTestProduct(t, db, "VEN")
	vendor := seedPurchaseRequestTestVendor(t, db, "PR")

	pr, err := service.CreatePurchaseRequest(models.PurchaseRequestCreatePayload{
		Department: "Operations",
		VendorID:   &vendor.ID,
		Items: []models.PurchaseRequestItemPayload{
			{ProductID: product.ID, Quantity: 1, EstimatedPrice: 100000},
		},
	}, requester.ID, requester.Role)
	require.NoError(t, err)
	require.NotNil(t, pr.Vendor)
	require.Equal(t, vendor.ID, pr.Vendor.ID)
	require.Equal(t, vendor.Name, pr.Vendor.Name)
}

func TestPurchaseRequestDocumentsArePersistedAndPreloaded(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	requester := seedPurchaseRequestTestUser(t, db, "staff-pr-doc", models.RoleEmployee)
	product := seedPurchaseRequestTestProduct(t, db, "DOC")

	pr, err := service.CreatePurchaseRequest(models.PurchaseRequestCreatePayload{
		Department: "Operations",
		Items: []models.PurchaseRequestItemPayload{
			{ProductID: product.ID, Quantity: 1, EstimatedPrice: 100000},
		},
	}, requester.ID, requester.Role)
	require.NoError(t, err)

	document, err := service.UploadPurchaseRequestDocument(
		pr.ID,
		"",
		"quotation.pdf",
		"/uploads/purchase-requests/quotation.pdf",
		2048,
		"application/pdf",
		requester.ID,
		requester.Role,
	)
	require.NoError(t, err)
	require.Equal(t, models.PurchaseRequestDocumentSupporting, document.DocumentType)

	updated, err := service.GetPurchaseRequestByID(pr.ID, requester.ID, requester.Role)
	require.NoError(t, err)
	require.Len(t, updated.Documents, 1)
	require.Equal(t, "quotation.pdf", updated.Documents[0].FileName)

	documents, err := service.GetPurchaseRequestDocuments(pr.ID, requester.ID, requester.Role)
	require.NoError(t, err)
	require.Len(t, documents, 1)
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

func TestFinanceManagerFinalReviewStoresApprovedItemBudget(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	requester := seedPurchaseRequestTestUser(t, db, "staff-budget", models.RoleEmployee)
	financeStaff := seedPurchaseRequestTestUser(t, db, "finance-staff-budget", models.RoleFinanceStaff)
	financeManager := seedPurchaseRequestTestUser(t, db, "finance-manager-budget", models.RoleFinanceManager)
	productA := seedPurchaseRequestTestProduct(t, db, "BUD-A")
	productB := seedPurchaseRequestTestProduct(t, db, "BUD-B")
	productC := seedPurchaseRequestTestProduct(t, db, "BUD-C")

	pr, err := service.CreatePurchaseRequest(models.PurchaseRequestCreatePayload{
		Department: "Operations",
		Items: []models.PurchaseRequestItemPayload{
			{ProductID: productA.ID, Quantity: 1, EstimatedPrice: 45000},
			{ProductID: productB.ID, Quantity: 1, EstimatedPrice: 800000},
			{ProductID: productC.ID, Quantity: 1, EstimatedPrice: 12000000},
		},
	}, requester.ID, requester.Role)
	require.NoError(t, err)
	require.InDelta(t, 12845000, pr.TotalEstimatedAmount, 0.01)
	require.Len(t, pr.Items, 3)

	staffPayload := models.PurchaseRequestReviewPayload{
		Items: []models.PurchaseRequestReviewItemPayload{
			{ItemID: pr.Items[0].ID, Decision: models.PRItemDecisionApproved},
			{ItemID: pr.Items[1].ID, Decision: models.PRItemDecisionRejected},
			{ItemID: pr.Items[2].ID, Decision: models.PRItemDecisionApproved},
		},
	}
	_, err = service.ReviewPurchaseRequest(pr.ID, financeStaff.ID, financeStaff.Role, staffPayload)
	require.NoError(t, err)

	managerPayload := models.PurchaseRequestReviewPayload{
		Items: []models.PurchaseRequestReviewItemPayload{
			{ItemID: pr.Items[0].ID, Decision: models.PRItemDecisionApproved},
			{ItemID: pr.Items[1].ID, Decision: models.PRItemDecisionRejected},
			{ItemID: pr.Items[2].ID, Decision: models.PRItemDecisionApproved},
		},
	}
	message, err := service.ReviewPurchaseRequest(pr.ID, financeManager.ID, financeManager.Role, managerPayload)
	require.NoError(t, err)
	require.Equal(t, "Final review selesai: 2 item disetujui dan 1 item ditolak", message)

	updated, err := service.GetPurchaseRequestByID(pr.ID, financeManager.ID, financeManager.Role)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusApproved, updated.Status)
	require.Equal(t, models.PRApprovalStageFinalized, updated.CurrentApprovalStage)
	require.InDelta(t, 12045000, updated.TotalEstimatedAmount, 0.01)
	require.Equal(t, models.PRItemDecisionApproved, updated.Items[0].ApprovalStatus)
	require.Equal(t, models.PRItemDecisionRejected, updated.Items[1].ApprovalStatus)
	require.Equal(t, models.PRItemDecisionApproved, updated.Items[2].ApprovalStatus)

	list, err := service.GetPurchaseRequests(financeManager.ID, financeManager.Role, "", "", false, 0)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.InDelta(t, 12045000, list[0].TotalEstimatedAmount, 0.01)
}

func TestGetPurchaseRequestsAvailableForPurchaseExcludesLinkedRequests(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	requester := seedPurchaseRequestTestUser(t, db, "staff-pr-available", models.RoleEmployee)
	financeManager := seedPurchaseRequestTestUser(t, db, "finance-manager-pr-available", models.RoleFinanceManager)
	product := seedPurchaseRequestTestProduct(t, db, "AVAILABLE")
	vendor := seedPurchaseRequestTestVendor(t, db, "AVAILABLE")

	approvedBy := financeManager.ID
	availablePR := models.PurchaseRequest{
		PRNumber:                 "PR-AVAILABLE",
		RequesterID:              requester.ID,
		Department:               "ops",
		Status:                   models.PRStatusApproved,
		CurrentApprovalStage:     models.PRApprovalStageFinalized,
		TotalEstimatedAmount:     100000,
		ApprovedBy:               &approvedBy,
		FinanceManagerApprovedBy: &approvedBy,
		Items: []models.PurchaseRequestItem{
			{
				ProductID:              product.ID,
				Quantity:               1,
				EstimatedPrice:         100000,
				TotalEstimatedPrice:    100000,
				ApprovalStatus:         models.PRItemDecisionApproved,
				FinanceManagerDecision: models.PRItemDecisionApproved,
			},
		},
	}
	require.NoError(t, db.Create(&availablePR).Error)

	usedPR := models.PurchaseRequest{
		PRNumber:                 "PR-USED",
		RequesterID:              requester.ID,
		Department:               "ops",
		Status:                   models.PRStatusApproved,
		CurrentApprovalStage:     models.PRApprovalStageFinalized,
		TotalEstimatedAmount:     100000,
		ApprovedBy:               &approvedBy,
		FinanceManagerApprovedBy: &approvedBy,
		Items: []models.PurchaseRequestItem{
			{
				ProductID:              product.ID,
				Quantity:               1,
				EstimatedPrice:         100000,
				TotalEstimatedPrice:    100000,
				ApprovalStatus:         models.PRItemDecisionApproved,
				FinanceManagerDecision: models.PRItemDecisionApproved,
			},
		},
	}
	require.NoError(t, db.Create(&usedPR).Error)

	linkedPurchase := models.Purchase{
		Code:              "PO-USED-PR",
		VendorID:          vendor.ID,
		UserID:            requester.ID,
		Status:            models.PurchaseStatusDraft,
		PurchaseRequestID: &usedPR.ID,
	}
	require.NoError(t, db.Create(&linkedPurchase).Error)

	list, err := service.GetPurchaseRequests(financeManager.ID, financeManager.Role, "", "APPROVED", true, 0)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "PR-AVAILABLE", list[0].PRNumber)

	editList, err := service.GetPurchaseRequests(financeManager.ID, financeManager.Role, "", "APPROVED", true, linkedPurchase.ID)
	require.NoError(t, err)
	require.Len(t, editList, 2)
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
