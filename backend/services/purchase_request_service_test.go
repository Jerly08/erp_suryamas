package services

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"testing"
	"time"

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
		&models.PurchaseItem{},
		&models.PurchaseRequest{},
		&models.PurchaseRequestItem{},
		&models.PurchaseRequestDocument{},
		&models.PurchaseRequestRevisionSnapshot{},
		&models.PurchaseRequestRealizationReview{},
		&models.Notification{},
		&models.NotificationPreference{},
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

func seedApprovedPurchaseRequestForCancelTest(t *testing.T, db *gorm.DB, service PurchaseRequestService, suffix string) (*models.PurchaseRequest, models.User, models.User, models.User, models.Contact) {
	t.Helper()

	requester := seedPurchaseRequestTestUser(t, db, "cancel-requester-"+suffix, models.RoleEmployee)
	financeStaff := seedPurchaseRequestTestUser(t, db, "cancel-staff-"+suffix, models.RoleFinanceStaff)
	financeManager := seedPurchaseRequestTestUser(t, db, "cancel-fm-"+suffix, models.RoleFinanceManager)
	product := seedPurchaseRequestTestProduct(t, db, "CANCEL-"+suffix)
	vendor := seedPurchaseRequestTestVendor(t, db, "CANCEL-"+suffix)

	pr, err := service.CreatePurchaseRequest(models.PurchaseRequestCreatePayload{
		Department: "Operations",
		Title:      "PR untuk cancel " + suffix,
		VendorID:   &vendor.ID,
		Items: []models.PurchaseRequestItemPayload{
			{ProductID: product.ID, Quantity: 1, EstimatedPrice: 100000},
		},
	}, requester.ID, requester.Role)
	require.NoError(t, err)
	require.Len(t, pr.Items, 1)

	_, err = service.ReviewPurchaseRequest(pr.ID, financeStaff.ID, financeStaff.Role, models.PurchaseRequestReviewPayload{
		Items: []models.PurchaseRequestReviewItemPayload{
			{ItemID: pr.Items[0].ID, Decision: models.PRItemDecisionApproved},
		},
		Reason: "Staff approve",
	})
	require.NoError(t, err)

	_, err = service.ReviewPurchaseRequest(pr.ID, financeManager.ID, financeManager.Role, models.PurchaseRequestReviewPayload{
		Items: []models.PurchaseRequestReviewItemPayload{
			{ItemID: pr.Items[0].ID, Decision: models.PRItemDecisionApproved},
		},
		Reason: "FM approve",
	})
	require.NoError(t, err)

	approved, err := service.GetPurchaseRequestByID(pr.ID, financeManager.ID, financeManager.Role)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusApproved, approved.Status)

	return approved, requester, financeStaff, financeManager, vendor
}

func TestPurchaseRequestRealizationWorkflowKeepsOriginalApproval(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	approved, requester, financeStaff, financeManager, _ := seedApprovedPurchaseRequestForCancelTest(t, db, service, "REALIZATION")
	now := time.Now()
	require.NoError(t, db.Model(&models.PurchaseRequest{}).Where("id = ?", approved.ID).Updates(map[string]interface{}{
		"requires_realization_document": true,
		"realization_status":            models.PRRealizationStatusAwaitingUpload,
		"realization_requested_by":      financeStaff.ID,
		"realization_requested_at":      now,
	}).Error)
	require.NoError(t, db.Create(&models.PurchaseRequestDocument{
		PurchaseRequestID: approved.ID,
		DocumentType:      models.PurchaseRequestDocumentRealization,
		FileName:          "billing.pdf",
		FilePath:          "/uploads/billing.pdf",
		FileSize:          1024,
		MimeType:          "application/pdf",
		UploadedBy:        requester.ID,
		ApprovalRevision:  approved.ApprovalRevision,
	}).Error)

	submitted, err := service.SubmitPurchaseRequestRealizationForReview(approved.ID, requester.ID, requester.Role, 1)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusApproved, submitted.Status)
	require.Equal(t, models.PRRealizationStatusPendingFinanceStaffReview, submitted.RealizationStatus)

	forwarded, err := service.ReviewPurchaseRequestRealization(approved.ID, financeStaff.ID, financeStaff.Role, models.PurchaseRequestRealizationVerificationPayload{
		Decision: models.PRRealizationReviewDecisionApproved,
	})
	require.NoError(t, err)
	require.Equal(t, models.PRStatusApproved, forwarded.Status)
	require.Equal(t, models.PRRealizationStatusPendingFinanceManagerReview, forwarded.RealizationStatus)

	verified, err := service.ReviewPurchaseRequestRealization(approved.ID, financeManager.ID, financeManager.Role, models.PurchaseRequestRealizationVerificationPayload{
		Decision: models.PRRealizationReviewDecisionVerified,
	})
	require.NoError(t, err)
	require.Equal(t, models.PRStatusApproved, verified.Status)
	require.Equal(t, models.PRRealizationStatusVerified, verified.RealizationStatus)
	require.Len(t, verified.RealizationReviews, 3)
	require.Equal(t, models.PRRealizationReviewDecisionSubmitted, verified.RealizationReviews[0].Decision)
	require.Equal(t, models.PRRealizationReviewDecisionApproved, verified.RealizationReviews[1].Decision)
	require.Equal(t, models.PRRealizationReviewDecisionVerified, verified.RealizationReviews[2].Decision)
}

func TestCreatePurchaseRequestCalculatesDueDateFromVendorInvoiceDate(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	requester := seedPurchaseRequestTestUser(t, db, "requester-invoice-date", models.RoleFinanceStaff)
	vendor := seedPurchaseRequestTestVendor(t, db, "DUE")
	product := seedPurchaseRequestTestProduct(t, db, "DUE")
	invoiceReceivedDate := time.Date(2026, time.June, 10, 0, 0, 0, 0, time.UTC)
	termsDays := 14

	pr, err := service.CreatePurchaseRequest(models.PurchaseRequestCreatePayload{
		Department:                "Finance",
		VendorID:                  &vendor.ID,
		VendorInvoiceReceivedDate: &invoiceReceivedDate,
		PaymentTermsDays:          &termsDays,
		Items: []models.PurchaseRequestItemPayload{
			{
				ProductID:      product.ID,
				Quantity:       1,
				EstimatedPrice: 100000,
			},
		},
	}, requester.ID, requester.Role)
	require.NoError(t, err)
	require.NotNil(t, pr.VendorInvoiceReceivedDate)
	require.NotNil(t, pr.PaymentTermsDays)
	require.NotNil(t, pr.DueDate)
	require.Equal(t, invoiceReceivedDate, *pr.VendorInvoiceReceivedDate)
	require.Equal(t, termsDays, *pr.PaymentTermsDays)
	require.Equal(t, invoiceReceivedDate.AddDate(0, 0, termsDays), *pr.DueDate)
}

func TestUpdatePurchaseRequestInvoiceTermsAllowsImmediateLinkedPurchaseWithoutDueDate(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	financeManager := seedPurchaseRequestTestUser(t, db, "fm-immediate-due-date", models.RoleFinanceManager)
	vendor := seedPurchaseRequestTestVendor(t, db, "IMMEDIATE")
	product := seedPurchaseRequestTestProduct(t, db, "IMMEDIATE")

	pr, err := service.CreatePurchaseRequest(models.PurchaseRequestCreatePayload{
		Department: "Finance",
		VendorID:   &vendor.ID,
		Items: []models.PurchaseRequestItemPayload{
			{
				ProductID:      product.ID,
				Quantity:       1,
				EstimatedPrice: 100000,
			},
		},
	}, financeManager.ID, financeManager.Role)
	require.NoError(t, err)

	purchase := models.Purchase{
		Code:              "PO-IMMEDIATE-PR",
		VendorID:          vendor.ID,
		UserID:            financeManager.ID,
		Status:            models.PurchaseStatusApproved,
		PaymentMethod:     models.PurchasePaymentTransfer,
		PurchaseRequestID: &pr.ID,
	}
	require.NoError(t, db.Create(&purchase).Error)

	invoiceReceivedDate := time.Date(2026, time.June, 30, 0, 0, 0, 0, time.UTC)
	termsDays := 14

	updated, err := service.UpdatePurchaseRequestInvoiceTerms(pr.ID, financeManager.ID, financeManager.Role, models.PurchaseRequestInvoiceTermsUpdatePayload{
		VendorInvoiceReceivedDate: &invoiceReceivedDate,
		PaymentTermsDays:          &termsDays,
	})
	require.NoError(t, err)
	require.NotNil(t, updated.VendorInvoiceReceivedDate)
	require.NotNil(t, updated.PaymentTermsDays)
	require.Nil(t, updated.DueDate)

	var persistedPurchase models.Purchase
	require.NoError(t, db.First(&persistedPurchase, purchase.ID).Error)
	require.True(t, persistedPurchase.DueDate.IsZero())
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
		Title:      "  Vendor onboarding request  ",
		VendorID:   &vendor.ID,
		Items: []models.PurchaseRequestItemPayload{
			{ProductID: product.ID, Quantity: 1, EstimatedPrice: 100000},
		},
	}, requester.ID, requester.Role)
	require.NoError(t, err)
	require.Equal(t, "Vendor onboarding request", pr.Title)
	require.NotNil(t, pr.Vendor)
	require.Equal(t, vendor.ID, pr.Vendor.ID)
	require.Equal(t, vendor.Name, pr.Vendor.Name)
}

func TestCreatePurchaseRequestSupportsCatalogAndManualItems(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	requester := seedPurchaseRequestTestUser(t, db, "staff-pr-manual", models.RoleEmployee)
	product := seedPurchaseRequestTestProduct(t, db, "MIXED")

	pr, err := service.CreatePurchaseRequest(models.PurchaseRequestCreatePayload{
		Department: "Operations",
		Items: []models.PurchaseRequestItemPayload{
			{ProductID: product.ID, Quantity: 2, EstimatedPrice: 100000},
			{ItemName: "  Jasa servis AC kantor  ", Quantity: 1, EstimatedPrice: 750000},
		},
	}, requester.ID, requester.Role)
	require.NoError(t, err)
	require.Len(t, pr.Items, 2)
	require.Equal(t, float64(950000), pr.TotalEstimatedAmount)

	var catalogItem *models.PurchaseRequestItem
	var manualItem *models.PurchaseRequestItem
	for index := range pr.Items {
		item := &pr.Items[index]
		if item.ProductID == nil {
			manualItem = item
		} else {
			catalogItem = item
		}
	}

	require.NotNil(t, catalogItem)
	require.Equal(t, product.ID, *catalogItem.ProductID)
	require.Equal(t, product.Name, catalogItem.ItemName)
	require.NotNil(t, catalogItem.Product)
	require.NotNil(t, manualItem)
	require.Equal(t, "Jasa servis AC kantor", manualItem.ItemName)
	require.Nil(t, manualItem.Product)
}

func TestCreatePurchaseRequestRejectsItemWithoutProductOrManualName(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)
	requester := seedPurchaseRequestTestUser(t, db, "staff-pr-empty-item", models.RoleEmployee)

	_, err := service.CreatePurchaseRequest(models.PurchaseRequestCreatePayload{
		Items: []models.PurchaseRequestItemPayload{
			{ItemName: "   ", Quantity: 1, EstimatedPrice: 1000},
		},
	}, requester.ID, requester.Role)
	require.ErrorContains(t, err, "item_name is required")
}

func TestManualPurchaseRequestItemCanLinkToExpensePurchaseItem(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	requester := seedPurchaseRequestTestUser(t, db, "staff-pr-expense-link", models.RoleEmployee)

	pr := models.PurchaseRequest{
		PRNumber:             "PR-MANUAL-LINK",
		RequesterID:          requester.ID,
		Status:               models.PRStatusApproved,
		CurrentApprovalStage: models.PRApprovalStageFinalized,
		Items: []models.PurchaseRequestItem{
			{
				ItemName:               "Jasa servis AC kantor",
				Quantity:               1,
				EstimatedPrice:         750000,
				TotalEstimatedPrice:    750000,
				ApprovalStatus:         models.PRItemDecisionApproved,
				FinanceManagerDecision: models.PRItemDecisionApproved,
			},
		},
	}
	require.NoError(t, db.Create(&pr).Error)

	requestItemID := pr.Items[0].ID
	purchase := &models.Purchase{
		PurchaseRequestID: &pr.ID,
		PurchaseItems: []models.PurchaseItem{
			{
				PurchaseRequestItemID: &requestItemID,
				ItemType:              models.PurchaseItemTypeExpense,
				Description:           "Jasa servis AC kantor",
				Quantity:              1,
				UnitPrice:             750000,
				TotalPrice:            750000,
			},
		},
	}

	purchaseService := &PurchaseService{db: db}
	_, budget, err := purchaseService.validateLinkedPurchaseRequestSelection(purchase, false)
	require.NoError(t, err)
	require.Equal(t, float64(750000), budget)

	purchase.PurchaseItems[0].Description = "Kebutuhan lain"
	_, _, err = purchaseService.validateLinkedPurchaseRequestSelection(purchase, false)
	require.ErrorContains(t, err, "must keep the same item name")
}

func TestCalculatePurchaseTotalsSupportsManualExpenseItem(t *testing.T) {
	service := &PurchaseService{}
	purchase := &models.Purchase{}

	err := service.calculatePurchaseTotals(purchase, []models.PurchaseItemRequest{
		{
			ItemType:    models.PurchaseItemTypeExpense,
			Description: "Jasa servis AC kantor",
			Quantity:    1,
			UnitPrice:   750000,
		},
	})
	require.NoError(t, err)
	require.Len(t, purchase.PurchaseItems, 1)
	require.Nil(t, purchase.PurchaseItems[0].ProductID)
	require.Equal(t, models.PurchaseItemTypeExpense, purchase.PurchaseItems[0].ItemType)
	require.Equal(t, "Jasa servis AC kantor", purchase.PurchaseItems[0].Description)
	require.Equal(t, float64(750000), purchase.SubtotalBeforeDiscount)
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
	require.Equal(t, 1, document.ApprovalRevision)

	updated, err := service.GetPurchaseRequestByID(pr.ID, requester.ID, requester.Role)
	require.NoError(t, err)
	require.Len(t, updated.Documents, 1)
	require.Equal(t, "quotation.pdf", updated.Documents[0].FileName)

	documents, err := service.GetPurchaseRequestDocuments(pr.ID, requester.ID, requester.Role)
	require.NoError(t, err)
	require.Len(t, documents, 1)
}

func TestAdditionalDocumentsAfterApprovalRestartWorkflowAndNotifyReviewers(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	notificationRepo := repositories.NewNotificationRepository(db)
	notificationService := NewNotificationService(db, notificationRepo)
	service := NewPurchaseRequestService(db, repo, notificationService)

	requester := seedPurchaseRequestTestUser(t, db, "staff-pr-reapproval", models.RoleEmployee)
	financeStaff := seedPurchaseRequestTestUser(t, db, "finance-staff-reapproval", models.RoleFinanceStaff)
	financeManager := seedPurchaseRequestTestUser(t, db, "finance-manager-reapproval", models.RoleFinanceManager)
	product := seedPurchaseRequestTestProduct(t, db, "REAPPROVAL")

	pr, err := service.CreatePurchaseRequest(models.PurchaseRequestCreatePayload{
		Department: "Operations",
		Items: []models.PurchaseRequestItemPayload{
			{ProductID: product.ID, Quantity: 1, EstimatedPrice: 250000},
		},
	}, requester.ID, requester.Role)
	require.NoError(t, err)

	approveItems := []models.PurchaseRequestReviewItemPayload{
		{ItemID: pr.Items[0].ID, Decision: models.PRItemDecisionApproved},
	}
	_, err = service.ReviewPurchaseRequest(pr.ID, financeStaff.ID, financeStaff.Role, models.PurchaseRequestReviewPayload{
		Items:  approveItems,
		Reason: "Dokumen pendukung awal sudah sesuai",
	})
	require.NoError(t, err)
	_, err = service.ReviewPurchaseRequest(pr.ID, financeManager.ID, financeManager.Role, models.PurchaseRequestReviewPayload{
		Items:  approveItems,
		Reason: "Disetujui finance manager",
	})
	require.NoError(t, err)

	approved, err := service.GetPurchaseRequestByID(pr.ID, requester.ID, requester.Role)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusApproved, approved.Status)
	require.Equal(t, 1, approved.ApprovalRevision)
	require.Equal(t, "Dokumen pendukung awal sudah sesuai", approved.FinanceStaffReviewNotes)
	require.Equal(t, "Disetujui finance manager", approved.FinanceManagerReviewNotes)

	require.NoError(t, db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.Notification{}).Error)

	document, err := service.UploadPurchaseRequestDocument(
		pr.ID,
		models.PurchaseRequestDocumentSupporting,
		"quotation-revised.pdf",
		"/uploads/purchase-requests/quotation-revised.pdf",
		4096,
		"application/pdf",
		requester.ID,
		requester.Role,
	)
	require.NoError(t, err)
	require.Equal(t, 2, document.ApprovalRevision)

	reapprovalStarted, err := service.SubmitAdditionalDocumentsForApproval(pr.ID, requester.ID, requester.Role, 1)
	require.NoError(t, err)
	require.True(t, reapprovalStarted)

	reopened, err := service.GetPurchaseRequestByID(pr.ID, requester.ID, requester.Role)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusPendingApproval, reopened.Status)
	require.Equal(t, models.PRApprovalStageFinanceStaff, reopened.CurrentApprovalStage)
	require.Equal(t, 2, reopened.ApprovalRevision)
	require.Equal(t, models.PRReapprovalReasonAdditionalDocuments, reopened.ReapprovalReason)
	require.Nil(t, reopened.FinanceStaffApprovedBy)
	require.Nil(t, reopened.FinanceManagerApprovedBy)
	require.Nil(t, reopened.ApprovedBy)
	require.Empty(t, reopened.FinanceStaffReviewNotes)
	require.Empty(t, reopened.FinanceManagerReviewNotes)
	require.Equal(t, models.PRItemDecisionApproved, reopened.Items[0].FinanceStaffDecision)
	require.Equal(t, models.PRItemDecisionApproved, reopened.Items[0].FinanceManagerDecision)

	var staffNotifications []models.Notification
	require.NoError(t, db.Where("user_id = ?", financeStaff.ID).Find(&staffNotifications).Error)
	require.Len(t, staffNotifications, 1)
	require.Contains(t, staffNotifications[0].Title, "Reapproval")
	require.Contains(t, staffNotifications[0].Data, "additional_documents_review_required")

	err = service.DeletePurchaseRequestDocument(document.ID, requester.ID, requester.Role)
	require.ErrorContains(t, err, "cannot be deleted while approval is in progress")

	require.NoError(t, db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.Notification{}).Error)
	_, err = service.ReviewPurchaseRequest(pr.ID, financeStaff.ID, financeStaff.Role, models.PurchaseRequestReviewPayload{Items: approveItems})
	require.NoError(t, err)

	var managerNotifications []models.Notification
	require.NoError(t, db.Where("user_id = ?", financeManager.ID).Find(&managerNotifications).Error)
	require.Len(t, managerNotifications, 1)
	require.Contains(t, managerNotifications[0].Title, "Document Reapproval")
	require.Contains(t, managerNotifications[0].Data, "finance_manager_document_reapproval_required")
}

func TestVendorInvoiceRevisionRestartsApprovalWhenMarkedByFinanceStaff(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	requester := seedPurchaseRequestTestUser(t, db, "staff-pr-invoice-revision", models.RoleEmployee)
	financeStaff := seedPurchaseRequestTestUser(t, db, "finance-staff-invoice-revision", models.RoleFinanceStaff)
	financeManager := seedPurchaseRequestTestUser(t, db, "finance-manager-invoice-revision", models.RoleFinanceManager)
	product := seedPurchaseRequestTestProduct(t, db, "INVREV")

	pr, err := service.CreatePurchaseRequest(models.PurchaseRequestCreatePayload{
		Department: "Operations",
		Items: []models.PurchaseRequestItemPayload{
			{ProductID: product.ID, Quantity: 1, EstimatedPrice: 100000},
		},
	}, requester.ID, requester.Role)
	require.NoError(t, err)

	approveItems := []models.PurchaseRequestReviewItemPayload{
		{ItemID: pr.Items[0].ID, Decision: models.PRItemDecisionApproved},
	}
	_, err = service.ReviewPurchaseRequest(pr.ID, financeStaff.ID, financeStaff.Role, models.PurchaseRequestReviewPayload{
		Items:                approveItems,
		Reason:               "Butuh invoice asli",
		RequireVendorInvoice: true,
	})
	require.NoError(t, err)

	staffReviewed, err := service.GetPurchaseRequestByID(pr.ID, requester.ID, requester.Role)
	require.NoError(t, err)
	require.True(t, staffReviewed.RequiresVendorInvoice)
	require.Equal(t, models.PRVendorInvoiceStatusAwaitingInvoice, staffReviewed.VendorInvoiceStatus)
	require.NotNil(t, staffReviewed.VendorInvoiceRequestedBy)
	require.Equal(t, financeStaff.ID, *staffReviewed.VendorInvoiceRequestedBy)

	_, err = service.ReviewPurchaseRequest(pr.ID, financeManager.ID, financeManager.Role, models.PurchaseRequestReviewPayload{
		Items:  approveItems,
		Reason: "Approved sambil tunggu invoice",
	})
	require.NoError(t, err)

	approved, err := service.GetPurchaseRequestByID(pr.ID, requester.ID, requester.Role)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusApproved, approved.Status)
	require.Equal(t, models.PRVendorInvoiceStatusAwaitingInvoice, approved.VendorInvoiceStatus)

	revised, err := service.SubmitPurchaseRequestInvoiceRevision(
		pr.ID,
		requester.ID,
		requester.Role,
		models.PurchaseRequestInvoiceRevisionPayload{
			Notes: "Invoice aktual lebih tinggi",
			Items: []models.PurchaseRequestInvoiceRevisionItemPayload{
				{ItemID: pr.Items[0].ID, Quantity: 1, EstimatedPrice: 125000, Notes: "Sesuai invoice vendor"},
			},
		},
		[]models.PurchaseRequestDocument{
			{
				FileName: "invoice-actual.pdf",
				FilePath: "/uploads/purchase-requests/invoice-actual.pdf",
				FileSize: 2048,
				MimeType: "application/pdf",
			},
		},
	)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusPendingApproval, revised.Status)
	require.Equal(t, models.PRApprovalStageFinanceStaff, revised.CurrentApprovalStage)
	require.Equal(t, models.PRReapprovalReasonInvoiceRevision, revised.ReapprovalReason)
	require.Equal(t, models.PRVendorInvoiceStatusPendingApproval, revised.VendorInvoiceStatus)
	require.Equal(t, 2, revised.ApprovalRevision)
	require.NotNil(t, revised.VendorInvoiceSubmittedBy)
	require.Equal(t, requester.ID, *revised.VendorInvoiceSubmittedBy)
	require.Equal(t, "Invoice aktual lebih tinggi", revised.VendorInvoiceRevisionNotes)
	require.Equal(t, float64(125000), revised.TotalEstimatedAmount)
	require.Equal(t, float64(125000), revised.Items[0].EstimatedPrice)
	require.Equal(t, float64(125000), revised.Items[0].TotalEstimatedPrice)
	require.Equal(t, models.PRItemDecisionPending, revised.Items[0].ApprovalStatus)
	require.Equal(t, models.PRItemDecisionPending, revised.Items[0].FinanceStaffDecision)
	require.Len(t, revised.Documents, 1)
	require.Equal(t, models.PurchaseRequestDocumentVendorInvoice, revised.Documents[0].DocumentType)
	require.Equal(t, 2, revised.Documents[0].ApprovalRevision)

	_, err = service.ReviewPurchaseRequest(pr.ID, financeStaff.ID, financeStaff.Role, models.PurchaseRequestReviewPayload{
		Items: approveItems,
	})
	require.NoError(t, err)
	_, err = service.ReviewPurchaseRequest(pr.ID, financeManager.ID, financeManager.Role, models.PurchaseRequestReviewPayload{
		Items: approveItems,
	})
	require.NoError(t, err)

	final, err := service.GetPurchaseRequestByID(pr.ID, requester.ID, requester.Role)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusApproved, final.Status)
	require.Equal(t, models.PRVendorInvoiceStatusApproved, final.VendorInvoiceStatus)
}

func TestVendorInvoiceRevisionRejectedWhenPurchaseAlreadyLinked(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	requester := seedPurchaseRequestTestUser(t, db, "staff-pr-invoice-linked", models.RoleEmployee)
	financeStaff := seedPurchaseRequestTestUser(t, db, "finance-staff-invoice-linked", models.RoleFinanceStaff)
	financeManager := seedPurchaseRequestTestUser(t, db, "finance-manager-invoice-linked", models.RoleFinanceManager)
	vendor := seedPurchaseRequestTestVendor(t, db, "INVLINK")
	product := seedPurchaseRequestTestProduct(t, db, "INVLINK")

	pr, err := service.CreatePurchaseRequest(models.PurchaseRequestCreatePayload{
		Department: "Operations",
		VendorID:   &vendor.ID,
		Items: []models.PurchaseRequestItemPayload{
			{ProductID: product.ID, Quantity: 1, EstimatedPrice: 100000},
		},
	}, requester.ID, requester.Role)
	require.NoError(t, err)

	approveItems := []models.PurchaseRequestReviewItemPayload{
		{ItemID: pr.Items[0].ID, Decision: models.PRItemDecisionApproved},
	}
	_, err = service.ReviewPurchaseRequest(pr.ID, financeStaff.ID, financeStaff.Role, models.PurchaseRequestReviewPayload{
		Items:                approveItems,
		RequireVendorInvoice: true,
	})
	require.NoError(t, err)
	_, err = service.ReviewPurchaseRequest(pr.ID, financeManager.ID, financeManager.Role, models.PurchaseRequestReviewPayload{
		Items: approveItems,
	})
	require.NoError(t, err)

	purchase := models.Purchase{
		Code:              "PO-INV-LINK",
		VendorID:          vendor.ID,
		UserID:            financeManager.ID,
		PurchaseRequestID: &pr.ID,
		Status:            models.PurchaseStatusDraft,
	}
	require.NoError(t, db.Create(&purchase).Error)

	_, err = service.SubmitPurchaseRequestInvoiceRevision(
		pr.ID,
		requester.ID,
		requester.Role,
		models.PurchaseRequestInvoiceRevisionPayload{
			Items: []models.PurchaseRequestInvoiceRevisionItemPayload{
				{ItemID: pr.Items[0].ID, Quantity: 1, EstimatedPrice: 125000},
			},
		},
		[]models.PurchaseRequestDocument{
			{FileName: "invoice.pdf", FilePath: "/uploads/purchase-requests/invoice.pdf", FileSize: 1024, MimeType: "application/pdf"},
		},
	)
	require.ErrorContains(t, err, "already linked to purchase")
}

func TestPostPurchaseVendorInvoiceVerificationPreservesFinancialWorkflow(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	requester := seedPurchaseRequestTestUser(t, db, "post-po-invoice-requester", models.RoleEmployee)
	financeStaff := seedPurchaseRequestTestUser(t, db, "post-po-invoice-staff", models.RoleFinanceStaff)
	financeManager := seedPurchaseRequestTestUser(t, db, "post-po-invoice-manager", models.RoleFinanceManager)
	vendor := seedPurchaseRequestTestVendor(t, db, "POST-PO-INVOICE")
	product := seedPurchaseRequestTestProduct(t, db, "POST-PO-INVOICE")

	pr, err := service.CreatePurchaseRequest(models.PurchaseRequestCreatePayload{
		Department: "Operations",
		VendorID:   &vendor.ID,
		Items: []models.PurchaseRequestItemPayload{
			{ProductID: product.ID, Quantity: 1, EstimatedPrice: 100000},
		},
	}, requester.ID, requester.Role)
	require.NoError(t, err)

	approveItems := []models.PurchaseRequestReviewItemPayload{
		{ItemID: pr.Items[0].ID, Decision: models.PRItemDecisionApproved},
	}
	_, err = service.ReviewPurchaseRequest(pr.ID, financeStaff.ID, financeStaff.Role, models.PurchaseRequestReviewPayload{
		Items:                approveItems,
		RequireVendorInvoice: true,
	})
	require.NoError(t, err)
	_, err = service.ReviewPurchaseRequest(pr.ID, financeManager.ID, financeManager.Role, models.PurchaseRequestReviewPayload{
		Items: approveItems,
	})
	require.NoError(t, err)

	linkedPurchase := models.Purchase{
		Code:              "PO-POST-PO-INVOICE",
		VendorID:          vendor.ID,
		UserID:            financeManager.ID,
		PurchaseRequestID: &pr.ID,
		Status:            models.PurchaseStatusDraft,
		TotalAmount:       100000,
		PaymentMethod:     "CREDIT",
	}
	require.NoError(t, db.Create(&linkedPurchase).Error)

	beforeUpload, err := service.GetPurchaseRequestByID(pr.ID, requester.ID, requester.Role)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusApproved, beforeUpload.Status)
	require.Equal(t, models.PRVendorInvoiceStatusAwaitingInvoice, beforeUpload.VendorInvoiceStatus)

	uploaded, err := service.UploadPostPurchaseVendorInvoiceDocuments(
		pr.ID,
		requester.ID,
		requester.Role,
		[]models.PurchaseRequestDocument{{
			FileName: "invoice-actual.pdf",
			FilePath: "/uploads/purchase-requests/invoice-actual.pdf",
			FileSize: 2048,
			MimeType: "application/pdf",
		}},
	)
	require.NoError(t, err)
	require.Equal(t, beforeUpload.Status, uploaded.Status)
	require.Equal(t, beforeUpload.CurrentApprovalStage, uploaded.CurrentApprovalStage)
	require.Equal(t, beforeUpload.TotalEstimatedAmount, uploaded.TotalEstimatedAmount)
	require.Equal(t, beforeUpload.ApprovalRevision, uploaded.ApprovalRevision)
	require.Equal(t, models.PRVendorInvoiceStatusPendingVerification, uploaded.VendorInvoiceStatus)
	require.NotNil(t, uploaded.VendorInvoiceSubmittedBy)
	require.Equal(t, requester.ID, *uploaded.VendorInvoiceSubmittedBy)
	require.Nil(t, uploaded.VendorInvoiceReviewedBy)
	require.Empty(t, uploaded.VendorInvoiceReviewNotes)
	require.Len(t, uploaded.Documents, 1)
	require.Equal(t, models.PurchaseRequestDocumentPostPurchaseVendorInvoice, uploaded.Documents[0].DocumentType)
	require.Equal(t, beforeUpload.ApprovalRevision, uploaded.Documents[0].ApprovalRevision)

	var purchaseAfterUpload models.Purchase
	require.NoError(t, db.First(&purchaseAfterUpload, linkedPurchase.ID).Error)
	require.Equal(t, linkedPurchase.Status, purchaseAfterUpload.Status)
	require.Equal(t, linkedPurchase.TotalAmount, purchaseAfterUpload.TotalAmount)
	require.Equal(t, linkedPurchase.PaymentMethod, purchaseAfterUpload.PaymentMethod)

	_, err = service.ReviewPostPurchaseVendorInvoice(
		pr.ID,
		financeManager.ID,
		financeManager.Role,
		models.PurchaseRequestVendorInvoiceVerificationPayload{
			Decision: models.PRVendorInvoiceStatusNeedsClarification,
		},
	)
	require.ErrorContains(t, err, "verification notes are required")

	clarified, err := service.ReviewPostPurchaseVendorInvoice(
		pr.ID,
		financeManager.ID,
		financeManager.Role,
		models.PurchaseRequestVendorInvoiceVerificationPayload{
			Decision: models.PRVendorInvoiceStatusNeedsClarification,
			Notes:    "Mohon unggah invoice dengan nomor vendor yang terlihat.",
		},
	)
	require.NoError(t, err)
	require.Equal(t, models.PRVendorInvoiceStatusNeedsClarification, clarified.VendorInvoiceStatus)
	require.Equal(t, "Mohon unggah invoice dengan nomor vendor yang terlihat.", clarified.VendorInvoiceReviewNotes)
	require.NotNil(t, clarified.VendorInvoiceReviewedBy)
	require.Equal(t, financeManager.ID, *clarified.VendorInvoiceReviewedBy)
	require.Equal(t, beforeUpload.TotalEstimatedAmount, clarified.TotalEstimatedAmount)
	require.Equal(t, beforeUpload.Status, clarified.Status)

	pendingVerification, err := service.UploadPostPurchaseVendorInvoiceDocuments(
		pr.ID,
		requester.ID,
		requester.Role,
		[]models.PurchaseRequestDocument{{
			FileName: "invoice-actual-revised.pdf",
			FilePath: "/uploads/purchase-requests/invoice-actual-revised.pdf",
			FileSize: 4096,
			MimeType: "application/pdf",
		}},
	)
	require.NoError(t, err)
	require.Equal(t, models.PRVendorInvoiceStatusPendingVerification, pendingVerification.VendorInvoiceStatus)
	require.Nil(t, pendingVerification.VendorInvoiceReviewedBy)
	require.Empty(t, pendingVerification.VendorInvoiceReviewNotes)
	require.Len(t, pendingVerification.Documents, 2)

	verified, err := service.ReviewPostPurchaseVendorInvoice(
		pr.ID,
		financeStaff.ID,
		financeStaff.Role,
		models.PurchaseRequestVendorInvoiceVerificationPayload{
			Decision: models.PRVendorInvoiceStatusVerified,
			Notes:    "Invoice aktual sesuai dokumen vendor.",
		},
	)
	require.NoError(t, err)
	require.Equal(t, models.PRVendorInvoiceStatusVerified, verified.VendorInvoiceStatus)
	require.NotNil(t, verified.VendorInvoiceReviewedBy)
	require.Equal(t, financeStaff.ID, *verified.VendorInvoiceReviewedBy)
	require.Equal(t, beforeUpload.Status, verified.Status)
	require.Equal(t, beforeUpload.CurrentApprovalStage, verified.CurrentApprovalStage)
	require.Equal(t, beforeUpload.TotalEstimatedAmount, verified.TotalEstimatedAmount)

	var purchaseAfterVerification models.Purchase
	require.NoError(t, db.First(&purchaseAfterVerification, linkedPurchase.ID).Error)
	require.Equal(t, linkedPurchase.Status, purchaseAfterVerification.Status)
	require.Equal(t, linkedPurchase.TotalAmount, purchaseAfterVerification.TotalAmount)
	require.Equal(t, linkedPurchase.PaymentMethod, purchaseAfterVerification.PaymentMethod)
}

func TestFinanceManagerInitialStageIsNotTreatedAsPriorApproval(t *testing.T) {
	pr := &models.PurchaseRequest{
		Status:               models.PRStatusPendingApproval,
		CurrentApprovalStage: models.PRApprovalStageFinanceManager,
	}

	require.False(t, purchaseRequestApprovalHasProgressed(pr))

	approverID := uint(9)
	pr.FinanceStaffApprovedBy = &approverID
	require.True(t, purchaseRequestApprovalHasProgressed(pr))
}

func TestFinanceTeamCanUpdatePurchaseRequestProject(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	requester := seedPurchaseRequestTestUser(t, db, "staff-pr-project", models.RoleEmployee)
	financeStaff := seedPurchaseRequestTestUser(t, db, "finance-staff-pr-project", models.RoleFinanceStaff)
	product := seedPurchaseRequestTestProduct(t, db, "PROJECT")
	vendor := seedPurchaseRequestTestVendor(t, db, "PROJECT")
	project := models.Project{
		Code:   "PRJ-PR-EDIT",
		Name:   "Project PR Edit",
		Status: models.ProjectStatusActive,
	}
	require.NoError(t, db.Create(&project).Error)

	pr, err := service.CreatePurchaseRequest(models.PurchaseRequestCreatePayload{
		Department: "Operations",
		VendorID:   &vendor.ID,
		Items: []models.PurchaseRequestItemPayload{
			{ProductID: product.ID, Quantity: 1, EstimatedPrice: 100000},
		},
	}, requester.ID, requester.Role)
	require.NoError(t, err)
	require.Nil(t, pr.ProjectID)

	linkedPurchase := models.Purchase{
		Code:              "PO-PRJ-EDIT",
		VendorID:          vendor.ID,
		UserID:            financeStaff.ID,
		PurchaseRequestID: &pr.ID,
		Status:            models.PurchaseStatusDraft,
	}
	require.NoError(t, db.Create(&linkedPurchase).Error)

	updated, err := service.UpdatePurchaseRequestProject(pr.ID, financeStaff.ID, financeStaff.Role, models.PurchaseRequestProjectUpdatePayload{
		ProjectID: &project.ID,
	})
	require.NoError(t, err)
	require.NotNil(t, updated.ProjectID)
	require.Equal(t, project.ID, *updated.ProjectID)
	require.NotNil(t, updated.Project)
	require.Equal(t, project.Name, updated.Project.Name)

	var syncedPurchase models.Purchase
	require.NoError(t, db.First(&syncedPurchase, linkedPurchase.ID).Error)
	require.NotNil(t, syncedPurchase.ProjectID)
	require.Equal(t, project.ID, *syncedPurchase.ProjectID)

	_, err = service.UpdatePurchaseRequestProject(pr.ID, requester.ID, requester.Role, models.PurchaseRequestProjectUpdatePayload{
		ProjectID: nil,
	})
	require.ErrorContains(t, err, "only finance staff")
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
		Reason: "Staff menyetujui dua item, satu item tidak sesuai",
	}
	_, err = service.ReviewPurchaseRequest(pr.ID, financeStaff.ID, financeStaff.Role, staffPayload)
	require.NoError(t, err)

	managerPayload := models.PurchaseRequestReviewPayload{
		Items: []models.PurchaseRequestReviewItemPayload{
			{ItemID: pr.Items[0].ID, Decision: models.PRItemDecisionApproved},
			{ItemID: pr.Items[1].ID, Decision: models.PRItemDecisionRejected},
			{ItemID: pr.Items[2].ID, Decision: models.PRItemDecisionApproved},
		},
		Reason: "Manager approve parsial",
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
	require.Equal(t, "Staff menyetujui dua item, satu item tidak sesuai", updated.FinanceStaffReviewNotes)
	require.Equal(t, "Manager approve parsial", updated.FinanceManagerReviewNotes)
	require.Equal(t, "Manager approve parsial", updated.RejectionReason)

	list, err := service.GetPurchaseRequests(financeManager.ID, financeManager.Role, "", "", false, 0)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.InDelta(t, 12045000, list[0].TotalEstimatedAmount, 0.01)
}

func TestFinanceManagerCanCancelApprovedPurchaseRequestBeforePurchase(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	approved, _, _, financeManager, _ := seedApprovedPurchaseRequestForCancelTest(t, db, service, "SUCCESS")

	cancelled, err := service.CancelPurchaseRequest(approved.ID, financeManager.ID, financeManager.Role, "Tidak jadi dilanjutkan ke pembelian")
	require.NoError(t, err)
	require.Equal(t, models.PRStatusCancelled, cancelled.Status)
	require.Equal(t, models.PRApprovalStageFinalized, cancelled.CurrentApprovalStage)
	require.Equal(t, "Tidak jadi dilanjutkan ke pembelian", cancelled.CancelledReason)
	require.NotNil(t, cancelled.CancelledBy)
	require.Equal(t, financeManager.ID, *cancelled.CancelledBy)
	require.NotNil(t, cancelled.CancelledAt)
	require.NotNil(t, cancelled.ApprovedBy)
	require.NotNil(t, cancelled.ApprovedAt)
	require.NotNil(t, cancelled.FinanceManagerApprovedBy)
	require.Equal(t, financeManager.ID, *cancelled.FinanceManagerApprovedBy)
	require.NotNil(t, cancelled.Canceller)
	require.Equal(t, financeManager.Username, cancelled.Canceller.Username)
}

func TestFinanceManagerCanCancelUnsubmittedRequesterRevisionBeforePurchase(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	approved, _, financeStaff, financeManager, _ := seedApprovedPurchaseRequestForCancelTest(t, db, service, "UNSUBMITTED-REQUESTER-REVISION")
	opened, err := service.OpenPurchaseRequestRequesterRevision(
		approved.ID,
		financeStaff.ID,
		financeStaff.Role,
		models.PurchaseRequestOpenRevisionPayload{Reason: "Kebutuhan dibatalkan sebelum PIC mengirim revisi"},
	)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusAwaitingRevision, opened.Status)
	require.Equal(t, models.PRReapprovalReasonRequesterRevision, opened.ReapprovalReason)
	require.Nil(t, opened.RevisionSubmittedBy)
	require.Nil(t, opened.RevisionSubmittedAt)

	cancelled, err := service.CancelPurchaseRequest(
		opened.ID,
		financeManager.ID,
		financeManager.Role,
		"Kebutuhan dibatalkan saat menunggu revisi PIC",
	)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusCancelled, cancelled.Status)
	require.Equal(t, models.PRApprovalStageFinalized, cancelled.CurrentApprovalStage)
	require.Equal(t, "Kebutuhan dibatalkan saat menunggu revisi PIC", cancelled.CancelledReason)
	require.NotNil(t, cancelled.FinanceManagerApprovedBy)
	require.NotNil(t, cancelled.FinanceManagerApprovedAt)
	require.NotNil(t, cancelled.RevisionOpenedAt)
	require.Nil(t, cancelled.RevisionSubmittedAt)
	require.Len(t, cancelled.RevisionSnapshots, 1)
}

func TestCancelPurchaseRequestRejectsUnsubmittedRequesterRevisionWhenPurchaseAlreadyLinked(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	approved, _, financeStaff, financeManager, vendor := seedApprovedPurchaseRequestForCancelTest(t, db, service, "UNSUBMITTED-REQUESTER-REVISION-LINKED")
	opened, err := service.OpenPurchaseRequestRequesterRevision(
		approved.ID,
		financeStaff.ID,
		financeStaff.Role,
		models.PurchaseRequestOpenRevisionPayload{Reason: "PIC diminta revisi sebelum pembatalan"},
	)
	require.NoError(t, err)

	purchase := models.Purchase{
		Code:              "PO-CANCEL-REVISION-LINKED",
		VendorID:          vendor.ID,
		UserID:            financeManager.ID,
		Status:            models.PurchaseStatusDraft,
		PurchaseRequestID: &opened.ID,
	}
	require.NoError(t, db.Create(&purchase).Error)

	_, err = service.CancelPurchaseRequest(
		opened.ID,
		financeManager.ID,
		financeManager.Role,
		"Tidak boleh membatalkan PR revisi yang sudah terealisasi",
	)
	require.ErrorContains(t, err, "already linked to purchase")
	require.ErrorContains(t, err, "PO-CANCEL-REVISION-LINKED")

	refreshed, err := service.GetPurchaseRequestByID(opened.ID, financeManager.ID, financeManager.Role)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusAwaitingRevision, refreshed.Status)
	require.Nil(t, refreshed.CancelledBy)
	require.Nil(t, refreshed.CancelledAt)
	require.Empty(t, refreshed.CancelledReason)
}

func TestCancelPurchaseRequestRejectsSubmittedRequesterRevision(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	approved, requester, financeStaff, financeManager, _ := seedApprovedPurchaseRequestForCancelTest(t, db, service, "SUBMITTED-REQUESTER-REVISION")
	opened, err := service.OpenPurchaseRequestRequesterRevision(
		approved.ID,
		financeStaff.ID,
		financeStaff.Role,
		models.PurchaseRequestOpenRevisionPayload{Reason: "PIC perlu memperbarui nominal"},
	)
	require.NoError(t, err)

	revised, err := service.SubmitPurchaseRequestRequesterRevision(
		opened.ID,
		requester.ID,
		requester.Role,
		models.PurchaseRequestRequesterRevisionPayload{
			Notes: "PIC mengirim revisi untuk ditinjau ulang",
			Items: []models.PurchaseRequestRequesterRevisionItemPayload{
				{
					ItemID:         opened.Items[0].ID,
					Quantity:       1,
					EstimatedPrice: 100000,
				},
			},
		},
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusPendingApproval, revised.Status)
	require.NotNil(t, revised.RevisionSubmittedAt)

	_, err = service.CancelPurchaseRequest(
		revised.ID,
		financeManager.ID,
		financeManager.Role,
		"Tidak boleh membatalkan revisi yang sudah dikirim",
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "can only be cancelled after finance manager approval")
}

func TestCancelPurchaseRequestRejectedForRequester(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	approved, requester, _, _, _ := seedApprovedPurchaseRequestForCancelTest(t, db, service, "REQUESTER")

	_, err := service.CancelPurchaseRequest(approved.ID, requester.ID, requester.Role, "Requester coba cancel")
	require.Error(t, err)
	require.Contains(t, err.Error(), "only finance manager or admin")
}

func TestCancelPurchaseRequestRejectedWhenPurchaseAlreadyLinked(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	approved, _, _, financeManager, vendor := seedApprovedPurchaseRequestForCancelTest(t, db, service, "LINKED")
	purchase := models.Purchase{
		Code:              "PO-CANCEL-LINKED",
		VendorID:          vendor.ID,
		UserID:            financeManager.ID,
		Status:            models.PurchaseStatusDraft,
		PurchaseRequestID: &approved.ID,
	}
	require.NoError(t, db.Create(&purchase).Error)

	_, err := service.CancelPurchaseRequest(approved.ID, financeManager.ID, financeManager.Role, "Tidak jadi")
	require.Error(t, err)
	require.Contains(t, err.Error(), "already linked to purchase")
	require.Contains(t, err.Error(), "PO-CANCEL-LINKED")

	refreshed, err := service.GetPurchaseRequestByID(approved.ID, financeManager.ID, financeManager.Role)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusApproved, refreshed.Status)
	require.Nil(t, refreshed.CancelledBy)
	require.Nil(t, refreshed.CancelledAt)
	require.Empty(t, refreshed.CancelledReason)
}

func TestRequesterRevisionPreservesApprovedSnapshotAndRestartsApproval(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	approved, requester, financeStaff, financeManager, _ := seedApprovedPurchaseRequestForCancelTest(t, db, service, "REQUESTER-REVISION")
	require.InDelta(t, 100000, approved.TotalEstimatedAmount, 0.01)
	require.NotNil(t, approved.ApprovedBy)
	require.NotNil(t, approved.FinanceManagerApprovedBy)

	opened, err := service.OpenPurchaseRequestRequesterRevision(
		approved.ID,
		financeStaff.ID,
		financeStaff.Role,
		models.PurchaseRequestOpenRevisionPayload{Reason: "PIC perlu revisi nominal sesuai informasi terbaru"},
	)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusAwaitingRevision, opened.Status)
	require.Equal(t, models.PRReapprovalReasonRequesterRevision, opened.ReapprovalReason)
	require.NotNil(t, opened.RevisionOpenedBy)
	require.Equal(t, financeStaff.ID, *opened.RevisionOpenedBy)
	require.Len(t, opened.RevisionSnapshots, 1)
	require.Equal(t, 1, opened.RevisionSnapshots[0].RevisionNumber)
	require.InDelta(t, 100000, opened.RevisionSnapshots[0].PreviousTotalEstimatedAmount, 0.01)
	require.Nil(t, opened.RevisionSnapshots[0].NewTotalEstimatedAmount)
	require.Contains(t, opened.RevisionSnapshots[0].ItemSnapshot, `"estimated_price":100000`)
	require.NotNil(t, opened.RevisionSnapshots[0].FinanceManagerApprovedBy)

	revised, err := service.SubmitPurchaseRequestRequesterRevision(
		opened.ID,
		requester.ID,
		requester.Role,
		models.PurchaseRequestRequesterRevisionPayload{
			Notes: "Nominal naik sesuai dokumen baru",
			Items: []models.PurchaseRequestRequesterRevisionItemPayload{
				{
					ItemID:         opened.Items[0].ID,
					Quantity:       1,
					EstimatedPrice: 200000,
					Notes:          "Harga terbaru",
				},
			},
		},
		[]models.PurchaseRequestDocument{
			{
				FileName: "dokumen-revisi.pdf",
				FilePath: "/uploads/purchase-requests/dokumen-revisi.pdf",
				FileSize: 1024,
				MimeType: "application/pdf",
			},
		},
	)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusPendingApproval, revised.Status)
	require.Equal(t, models.PRApprovalStageFinanceStaff, revised.CurrentApprovalStage)
	require.Equal(t, models.PRReapprovalReasonRequesterRevision, revised.ReapprovalReason)
	require.Equal(t, 2, revised.ApprovalRevision)
	require.InDelta(t, 200000, revised.TotalEstimatedAmount, 0.01)
	require.Nil(t, revised.ApprovedBy)
	require.Nil(t, revised.FinanceManagerApprovedBy)
	require.NotNil(t, revised.RevisionSubmittedBy)
	require.Equal(t, requester.ID, *revised.RevisionSubmittedBy)
	require.Len(t, revised.Documents, 1)
	require.Equal(t, models.PurchaseRequestDocumentRequesterRevision, revised.Documents[0].DocumentType)
	require.Equal(t, 2, revised.Documents[0].ApprovalRevision)
	require.Len(t, revised.RevisionSnapshots, 1)
	require.NotNil(t, revised.RevisionSnapshots[0].NewTotalEstimatedAmount)
	require.InDelta(t, 200000, *revised.RevisionSnapshots[0].NewTotalEstimatedAmount, 0.01)
	require.Equal(t, "Nominal naik sesuai dokumen baru", revised.RevisionSnapshots[0].SubmittedNotes)
	require.Contains(t, revised.RevisionSnapshots[0].NewItemSnapshot, `"estimated_price":200000`)

	_, err = service.ReviewPurchaseRequest(revised.ID, financeStaff.ID, financeStaff.Role, models.PurchaseRequestReviewPayload{
		Items: []models.PurchaseRequestReviewItemPayload{
			{ItemID: revised.Items[0].ID, Decision: models.PRItemDecisionApproved},
		},
		Reason: "Staff approve revisi",
	})
	require.NoError(t, err)

	_, err = service.ReviewPurchaseRequest(revised.ID, financeManager.ID, financeManager.Role, models.PurchaseRequestReviewPayload{
		Items: []models.PurchaseRequestReviewItemPayload{
			{ItemID: revised.Items[0].ID, Decision: models.PRItemDecisionApproved},
		},
		Reason: "FM approve revisi",
	})
	require.NoError(t, err)

	final, err := service.GetPurchaseRequestByID(revised.ID, financeManager.ID, financeManager.Role)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusApproved, final.Status)
	require.InDelta(t, 200000, final.TotalEstimatedAmount, 0.01)
	require.Equal(t, 2, final.ApprovalRevision)
	require.NotNil(t, final.ApprovedBy)
	require.Len(t, final.RevisionSnapshots, 1)
	require.InDelta(t, 100000, final.RevisionSnapshots[0].PreviousTotalEstimatedAmount, 0.01)
	require.NotNil(t, final.RevisionSnapshots[0].NewTotalEstimatedAmount)
	require.InDelta(t, 200000, *final.RevisionSnapshots[0].NewTotalEstimatedAmount, 0.01)
}

func TestRequesterRevisionAllowsItemAddRemoveWithoutNewDocument(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	requester := seedPurchaseRequestTestUser(t, db, "requester-revision-items", models.RoleEmployee)
	financeStaff := seedPurchaseRequestTestUser(t, db, "requester-revision-items-staff", models.RoleFinanceStaff)
	financeManager := seedPurchaseRequestTestUser(t, db, "requester-revision-items-manager", models.RoleFinanceManager)
	firstProduct := seedPurchaseRequestTestProduct(t, db, "REQUESTER-REVISION-ITEMS-ONE")
	secondProduct := seedPurchaseRequestTestProduct(t, db, "REQUESTER-REVISION-ITEMS-TWO")
	vendor := seedPurchaseRequestTestVendor(t, db, "REQUESTER-REVISION-ITEMS")

	pr, err := service.CreatePurchaseRequest(models.PurchaseRequestCreatePayload{
		Department: "Operations",
		Title:      "PR revisi tambah dan hapus item",
		VendorID:   &vendor.ID,
		Items: []models.PurchaseRequestItemPayload{
			{ProductID: firstProduct.ID, Quantity: 1, EstimatedPrice: 100000},
			{ProductID: secondProduct.ID, Quantity: 1, EstimatedPrice: 75000},
		},
	}, requester.ID, requester.Role)
	require.NoError(t, err)
	require.Len(t, pr.Items, 2)

	_, err = service.ReviewPurchaseRequest(pr.ID, financeStaff.ID, financeStaff.Role, models.PurchaseRequestReviewPayload{
		Items: []models.PurchaseRequestReviewItemPayload{
			{ItemID: pr.Items[0].ID, Decision: models.PRItemDecisionApproved},
			{ItemID: pr.Items[1].ID, Decision: models.PRItemDecisionApproved},
		},
		Reason: "Review staff awal",
	})
	require.NoError(t, err)
	_, err = service.ReviewPurchaseRequest(pr.ID, financeManager.ID, financeManager.Role, models.PurchaseRequestReviewPayload{
		Items: []models.PurchaseRequestReviewItemPayload{
			{ItemID: pr.Items[0].ID, Decision: models.PRItemDecisionApproved},
			{ItemID: pr.Items[1].ID, Decision: models.PRItemDecisionApproved},
		},
		Reason: "Review manager awal",
	})
	require.NoError(t, err)

	approved, err := service.GetPurchaseRequestByID(pr.ID, financeManager.ID, financeManager.Role)
	require.NoError(t, err)
	opened, err := service.OpenPurchaseRequestRequesterRevision(
		approved.ID,
		financeStaff.ID,
		financeStaff.Role,
		models.PurchaseRequestOpenRevisionPayload{Reason: "Kebutuhan operasional berubah"},
	)
	require.NoError(t, err)
	removedItemID := opened.Items[1].ID

	_, err = service.SubmitPurchaseRequestRequesterRevision(
		opened.ID,
		requester.ID,
		requester.Role,
		models.PurchaseRequestRequesterRevisionPayload{
			Items: []models.PurchaseRequestRequesterRevisionItemPayload{
				{ItemID: opened.Items[0].ID, Quantity: 2, EstimatedPrice: 100000, Notes: "Qty disesuaikan"},
			},
		},
		nil,
	)
	require.ErrorContains(t, err, "revision notes are required")

	revised, err := service.SubmitPurchaseRequestRequesterRevision(
		opened.ID,
		requester.ID,
		requester.Role,
		models.PurchaseRequestRequesterRevisionPayload{
			Notes: "Qty diperbarui, item kedua dibatalkan, dan jasa instalasi ditambahkan.",
			Items: []models.PurchaseRequestRequesterRevisionItemPayload{
				{ItemID: opened.Items[0].ID, Quantity: 2, EstimatedPrice: 100000, Notes: "Qty disesuaikan"},
				{ItemName: "Jasa instalasi", Quantity: 1, EstimatedPrice: 50000, Notes: "Kebutuhan tambahan"},
			},
		},
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusPendingApproval, revised.Status)
	require.Equal(t, models.PRApprovalStageFinanceStaff, revised.CurrentApprovalStage)
	require.Len(t, revised.Items, 2)
	require.InDelta(t, 250000, revised.TotalEstimatedAmount, 0.01)
	require.Empty(t, revised.Documents)
	require.NotNil(t, revised.RevisionSnapshots[0].NewTotalEstimatedAmount)
	require.InDelta(t, 250000, *revised.RevisionSnapshots[0].NewTotalEstimatedAmount, 0.01)
	require.Contains(t, revised.RevisionSnapshots[0].NewItemSnapshot, "Jasa instalasi")

	var removedItem models.PurchaseRequestItem
	require.NoError(t, db.Unscoped().First(&removedItem, removedItemID).Error)
	require.True(t, removedItem.DeletedAt.Valid)
}

func TestFinanceStaffCanRequestRequesterRevisionBeforeInitialReview(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	requester := seedPurchaseRequestTestUser(t, db, "revision-before-review-requester", models.RoleEmployee)
	financeStaff := seedPurchaseRequestTestUser(t, db, "revision-before-review-staff", models.RoleFinanceStaff)
	financeManager := seedPurchaseRequestTestUser(t, db, "revision-before-review-manager", models.RoleFinanceManager)
	product := seedPurchaseRequestTestProduct(t, db, "REVISION-BEFORE-REVIEW")
	vendor := seedPurchaseRequestTestVendor(t, db, "REVISION-BEFORE-REVIEW")

	pr, err := service.CreatePurchaseRequest(models.PurchaseRequestCreatePayload{
		Department: "Operations",
		Title:      "PR perlu perbaikan sebelum review",
		VendorID:   &vendor.ID,
		Items: []models.PurchaseRequestItemPayload{
			{ProductID: product.ID, Quantity: 1, EstimatedPrice: 100000},
		},
	}, requester.ID, requester.Role)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusPendingApproval, pr.Status)
	require.Equal(t, models.PRApprovalStageFinanceStaff, pr.CurrentApprovalStage)

	_, err = service.OpenPurchaseRequestRequesterRevision(
		pr.ID,
		financeManager.ID,
		financeManager.Role,
		models.PurchaseRequestOpenRevisionPayload{Reason: "Finance Manager mencoba membuka tahap Finance Staff"},
	)
	require.ErrorContains(t, err, "only the active purchase request reviewer")

	opened, err := service.OpenPurchaseRequestRequesterRevision(
		pr.ID,
		financeStaff.ID,
		financeStaff.Role,
		models.PurchaseRequestOpenRevisionPayload{Reason: "Mohon lengkapi spesifikasi dan dokumen pendukung"},
	)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusAwaitingRevision, opened.Status)
	require.Equal(t, models.PRApprovalStageFinalized, opened.CurrentApprovalStage)
	require.Nil(t, opened.FinanceStaffApprovedBy)
	require.Nil(t, opened.FinanceManagerApprovedBy)
	require.Len(t, opened.RevisionSnapshots, 1)
	require.Equal(t, models.PRStatusPendingApproval, opened.RevisionSnapshots[0].PreviousStatus)
	require.Equal(t, models.PRApprovalStageFinanceStaff, opened.RevisionSnapshots[0].PreviousApprovalStage)
	require.Nil(t, opened.RevisionSnapshots[0].FinanceStaffApprovedBy)
	require.Nil(t, opened.RevisionSnapshots[0].FinanceManagerApprovedBy)

	revised, err := service.SubmitPurchaseRequestRequesterRevision(
		opened.ID,
		requester.ID,
		requester.Role,
		models.PurchaseRequestRequesterRevisionPayload{
			Notes: "Spesifikasi dan nominal sudah diperbaiki PIC",
			Items: []models.PurchaseRequestRequesterRevisionItemPayload{
				{ItemID: opened.Items[0].ID, Quantity: 1, EstimatedPrice: 125000},
			},
		},
		[]models.PurchaseRequestDocument{
			{
				FileName: "perbaikan-pic.pdf",
				FilePath: "/uploads/purchase-requests/perbaikan-pic.pdf",
				FileSize: 1024,
				MimeType: "application/pdf",
			},
		},
	)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusPendingApproval, revised.Status)
	require.Equal(t, models.PRApprovalStageFinanceStaff, revised.CurrentApprovalStage)
	require.Equal(t, 2, revised.ApprovalRevision)
	require.Nil(t, revised.FinanceStaffApprovedBy)
	require.Nil(t, revised.FinanceManagerApprovedBy)
	require.NotNil(t, revised.RevisionSubmittedBy)
	require.Equal(t, requester.ID, *revised.RevisionSubmittedBy)
	require.InDelta(t, 125000, revised.TotalEstimatedAmount, 0.01)
	require.NotNil(t, revised.RevisionSnapshots[0].NewTotalEstimatedAmount)
	require.InDelta(t, 125000, *revised.RevisionSnapshots[0].NewTotalEstimatedAmount, 0.01)
}

func TestRequesterRevisionDuringApprovalAllowsFinanceStaffBeforeManagerReview(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	requester := seedPurchaseRequestTestUser(t, db, "revision-current-stage-requester", models.RoleEmployee)
	financeStaff := seedPurchaseRequestTestUser(t, db, "revision-current-stage-staff", models.RoleFinanceStaff)
	financeManager := seedPurchaseRequestTestUser(t, db, "revision-current-stage-manager", models.RoleFinanceManager)
	product := seedPurchaseRequestTestProduct(t, db, "REVISION-CURRENT-STAGE")
	vendor := seedPurchaseRequestTestVendor(t, db, "REVISION-CURRENT-STAGE")

	pr, err := service.CreatePurchaseRequest(models.PurchaseRequestCreatePayload{
		Department: "Operations",
		Title:      "PR revisi pada tahap Finance Manager",
		VendorID:   &vendor.ID,
		Items: []models.PurchaseRequestItemPayload{
			{ProductID: product.ID, Quantity: 1, EstimatedPrice: 100000},
		},
	}, requester.ID, requester.Role)
	require.NoError(t, err)

	_, err = service.ReviewPurchaseRequest(pr.ID, financeStaff.ID, financeStaff.Role, models.PurchaseRequestReviewPayload{
		Items: []models.PurchaseRequestReviewItemPayload{
			{ItemID: pr.Items[0].ID, Decision: models.PRItemDecisionApproved},
		},
		Reason: "Lanjut ke Finance Manager",
	})
	require.NoError(t, err)
	reviewed, err := service.GetPurchaseRequestByID(pr.ID, financeStaff.ID, financeStaff.Role)
	require.NoError(t, err)
	require.Equal(t, models.PRApprovalStageFinanceManager, reviewed.CurrentApprovalStage)

	require.True(t, canOpenPurchaseRequestRequesterRevision(reviewed, financeStaff.Role))
	require.True(t, canOpenPurchaseRequestRequesterRevision(reviewed, financeManager.Role))

	opened, err := service.OpenPurchaseRequestRequesterRevision(
		pr.ID,
		financeStaff.ID,
		financeStaff.Role,
		models.PurchaseRequestOpenRevisionPayload{Reason: "Nominal dan dokumen perlu diperbaiki PIC sebelum review Finance Manager"},
	)
	require.NoError(t, err)
	require.Equal(t, models.PRStatusAwaitingRevision, opened.Status)
	require.Len(t, opened.RevisionSnapshots, 1)
	require.Equal(t, models.PRApprovalStageFinanceManager, opened.RevisionSnapshots[0].PreviousApprovalStage)
	require.NotNil(t, opened.RevisionSnapshots[0].FinanceStaffApprovedBy)
	require.Nil(t, opened.RevisionSnapshots[0].FinanceManagerApprovedBy)
}

func TestFinanceStaffCannotOpenRequesterRevisionAfterManagerReviewStarts(t *testing.T) {
	managerReviewAt := time.Now()
	pr := &models.PurchaseRequest{
		Status:                    models.PRStatusPendingApproval,
		CurrentApprovalStage:      models.PRApprovalStageFinanceManager,
		FinanceManagerApprovedAt:  &managerReviewAt,
		FinanceManagerReviewNotes: "Perlu penyesuaian",
		Items: []models.PurchaseRequestItem{
			{FinanceManagerDecision: models.PRItemDecisionApproved},
		},
	}

	require.False(t, canOpenPurchaseRequestRequesterRevision(pr, models.RoleFinanceStaff))
	require.True(t, canOpenPurchaseRequestRequesterRevision(pr, models.RoleFinanceManager))
}

func TestOpenRequesterRevisionRejectedWhenPurchaseAlreadyLinked(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	approved, _, financeStaff, financeManager, vendor := seedApprovedPurchaseRequestForCancelTest(t, db, service, "REQUESTER-LINKED")
	purchase := models.Purchase{
		Code:              "PO-REQUESTER-REVISION-LINKED",
		VendorID:          vendor.ID,
		UserID:            financeManager.ID,
		Status:            models.PurchaseStatusDraft,
		PurchaseRequestID: &approved.ID,
	}
	require.NoError(t, db.Create(&purchase).Error)

	_, err := service.OpenPurchaseRequestRequesterRevision(
		approved.ID,
		financeStaff.ID,
		financeStaff.Role,
		models.PurchaseRequestOpenRevisionPayload{Reason: "Butuh revisi"},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already linked to purchase")
	require.Contains(t, err.Error(), "PO-REQUESTER-REVISION-LINKED")
}

func TestSubmitRequesterRevisionRejectedWithoutFinanceOpen(t *testing.T) {
	db := newPurchaseRequestTestDB(t)
	repo := repositories.NewPurchaseRequestRepository(db)
	service := NewPurchaseRequestService(db, repo, nil)

	approved, requester, _, _, _ := seedApprovedPurchaseRequestForCancelTest(t, db, service, "REQUESTER-NOT-OPEN")

	_, err := service.SubmitPurchaseRequestRequesterRevision(
		approved.ID,
		requester.ID,
		requester.Role,
		models.PurchaseRequestRequesterRevisionPayload{
			Items: []models.PurchaseRequestRequesterRevisionItemPayload{
				{ItemID: approved.Items[0].ID, Quantity: 1, EstimatedPrice: 200000},
			},
		},
		[]models.PurchaseRequestDocument{
			{
				FileName: "dokumen-revisi.pdf",
				FilePath: "/uploads/purchase-requests/dokumen-revisi.pdf",
				FileSize: 1024,
				MimeType: "application/pdf",
			},
		},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not waiting for requester revision")
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
				ProductID:              uintPtr(product.ID),
				ItemName:               product.Name,
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
				ProductID:              uintPtr(product.ID),
				ItemName:               product.Name,
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

	usedByItemPR := models.PurchaseRequest{
		PRNumber:                 "PR-USED-BY-ITEM",
		RequesterID:              requester.ID,
		Department:               "ops",
		Status:                   models.PRStatusApproved,
		CurrentApprovalStage:     models.PRApprovalStageFinalized,
		TotalEstimatedAmount:     45000,
		ApprovedBy:               &approvedBy,
		FinanceManagerApprovedBy: &approvedBy,
		Items: []models.PurchaseRequestItem{
			{
				ProductID:              uintPtr(product.ID),
				ItemName:               product.Name,
				Quantity:               1,
				EstimatedPrice:         45000,
				TotalEstimatedPrice:    45000,
				ApprovalStatus:         models.PRItemDecisionApproved,
				FinanceManagerDecision: models.PRItemDecisionApproved,
			},
		},
	}
	require.NoError(t, db.Create(&usedByItemPR).Error)

	itemLinkedPurchase := models.Purchase{
		Code:     "PO-USED-PR-ITEM",
		VendorID: vendor.ID,
		UserID:   requester.ID,
		Status:   models.PurchaseStatusDraft,
	}
	require.NoError(t, db.Create(&itemLinkedPurchase).Error)
	usedByItemID := usedByItemPR.Items[0].ID
	require.NoError(t, db.Create(&models.PurchaseItem{
		PurchaseID:            itemLinkedPurchase.ID,
		ProductID:             &product.ID,
		PurchaseRequestItemID: &usedByItemID,
		Quantity:              1,
		UnitPrice:             45000,
		TotalPrice:            45000,
	}).Error)

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
