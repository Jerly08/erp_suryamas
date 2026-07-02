package services

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type PurchaseRequestService interface {
	CreatePurchaseRequest(payload models.PurchaseRequestCreatePayload, userID uint, userRole string) (*models.PurchaseRequest, error)
	GetPurchaseRequests(userID uint, userRole string, department string, status string, availableForPurchase bool, currentPurchaseID uint) ([]models.PurchaseRequest, error)
	GetPurchaseRequestByID(id uint, userID uint, userRole string) (*models.PurchaseRequest, error)
	UpdatePurchaseRequestProject(id uint, userID uint, userRole string, payload models.PurchaseRequestProjectUpdatePayload) (*models.PurchaseRequest, error)
	UpdatePurchaseRequestInvoiceTerms(id uint, userID uint, userRole string, payload models.PurchaseRequestInvoiceTermsUpdatePayload) (*models.PurchaseRequest, error)
	OpenPurchaseRequestRequesterRevision(id uint, userID uint, userRole string, payload models.PurchaseRequestOpenRevisionPayload) (*models.PurchaseRequest, error)
	SubmitPurchaseRequestRequesterRevision(id uint, userID uint, userRole string, payload models.PurchaseRequestRequesterRevisionPayload, documents []models.PurchaseRequestDocument) (*models.PurchaseRequest, error)
	SubmitPurchaseRequestInvoiceRevision(id uint, userID uint, userRole string, payload models.PurchaseRequestInvoiceRevisionPayload, documents []models.PurchaseRequestDocument) (*models.PurchaseRequest, error)
	ReviewPurchaseRequest(id uint, reviewerID uint, reviewerRole string, payload models.PurchaseRequestReviewPayload) (string, error)
	ApprovePurchaseRequest(id uint, approverID uint, approverRole string) (string, error)
	RejectPurchaseRequest(id uint, approverID uint, approverRole string, reason string) (string, error)
	CancelPurchaseRequest(id uint, userID uint, userRole string, reason string) (*models.PurchaseRequest, error)
	DeletePurchaseRequest(id uint) error
	UploadPurchaseRequestDocument(purchaseRequestID uint, documentType, fileName, filePath string, fileSize int64, mimeType string, userID uint, userRole string) (*models.PurchaseRequestDocument, error)
	UploadPurchaseRequestRealizationDocument(purchaseRequestID uint, fileName, filePath string, fileSize int64, mimeType string, userID uint, userRole string) (*models.PurchaseRequestDocument, error)
	SubmitAdditionalDocumentsForApproval(purchaseRequestID uint, userID uint, userRole string, documentCount int) (bool, error)
	GetPurchaseRequestDocuments(purchaseRequestID uint, userID uint, userRole string) ([]models.PurchaseRequestDocument, error)
	GetPurchaseRequestDocumentForDownload(documentID uint, userID uint, userRole string) (*models.PurchaseRequestDocument, error)
	DeletePurchaseRequestDocument(documentID uint, userID uint, userRole string) error
	SyncPurchaseRequestItemsWithPurchase(purchase *models.Purchase) error
	ReleasePurchaseRequestItemsFromPurchase(purchase *models.Purchase) error
}

type purchaseRequestService struct {
	repo                repositories.PurchaseRequestRepository
	productRepo         *repositories.ProductRepository
	notificationService *NotificationService
	db                  *gorm.DB
}

func NewPurchaseRequestService(db *gorm.DB, repo repositories.PurchaseRequestRepository, notificationService *NotificationService) PurchaseRequestService {
	return &purchaseRequestService{
		repo:                repo,
		productRepo:         repositories.NewProductRepository(db),
		notificationService: notificationService,
		db:                  db,
	}
}

// Generate PR Number (Format: PR-YYYYMMDD-ID)
func generatePRNumber(id uint) string {
	now := time.Now()
	return fmt.Sprintf("PR-%s-%04d", now.Format("20060102"), id)
}

func (s *purchaseRequestService) CreatePurchaseRequest(payload models.PurchaseRequestCreatePayload, userID uint, userRole string) (*models.PurchaseRequest, error) {
	var totalEstimatedAmount float64
	var items []models.PurchaseRequestItem
	initialStage := resolveInitialPurchaseRequestStage(userRole)

	if len(payload.Items) == 0 {
		return nil, fmt.Errorf("items are required")
	}

	if payload.VendorID != nil {
		var vendor models.Contact
		if err := s.db.Where("id = ? AND type = ?", *payload.VendorID, models.ContactTypeVendor).First(&vendor).Error; err != nil {
			return nil, fmt.Errorf("invalid vendor id: %d", *payload.VendorID)
		}
		if payload.PaymentTermsDays == nil && vendor.PaymentTerms >= 0 {
			terms := vendor.PaymentTerms
			payload.PaymentTermsDays = &terms
		}
	}

	invoiceReceivedDate, paymentTermsDays, dueDate, err := normalizePurchaseRequestInvoiceTerms(
		payload.VendorInvoiceReceivedDate,
		payload.PaymentTermsDays,
		payload.DueDate,
	)
	if err != nil {
		return nil, err
	}

	for _, rawItem := range payload.Items {
		if rawItem.Quantity < 1 {
			return nil, fmt.Errorf("quantity must be at least 1")
		}
		if rawItem.EstimatedPrice < 0 {
			return nil, fmt.Errorf("estimated_price cannot be negative")
		}

		var productID *uint
		itemName := strings.TrimSpace(rawItem.ItemName)
		if rawItem.ProductID > 0 {
			product, err := s.productRepo.FindByID(rawItem.ProductID)
			if err != nil {
				return nil, fmt.Errorf("invalid product id: %d", rawItem.ProductID)
			}
			productID = &product.ID
			itemName = strings.TrimSpace(product.Name)
		} else if itemName == "" {
			return nil, fmt.Errorf("item_name is required when product_id is not provided")
		}

		totalItemEst := float64(rawItem.Quantity) * rawItem.EstimatedPrice
		totalEstimatedAmount += totalItemEst

		item := models.PurchaseRequestItem{
			ProductID:              productID,
			ItemName:               itemName,
			Quantity:               rawItem.Quantity,
			EstimatedPrice:         rawItem.EstimatedPrice,
			TotalEstimatedPrice:    totalItemEst,
			ApprovalStatus:         models.PRItemDecisionPending,
			FinanceStaffDecision:   models.PRItemDecisionPending,
			FinanceManagerDecision: models.PRItemDecisionPending,
			DirectorDecision:       models.PRItemDecisionPending,
			Notes:                  rawItem.Notes,
		}
		items = append(items, item)
	}

	pr := &models.PurchaseRequest{
		RequesterID:               userID,
		Department:                strings.TrimSpace(payload.Department),
		Title:                     strings.TrimSpace(payload.Title),
		ProjectID:                 payload.ProjectID,
		VendorID:                  payload.VendorID,
		VendorInvoiceReceivedDate: invoiceReceivedDate,
		PaymentTermsDays:          paymentTermsDays,
		DueDate:                   dueDate,
		TotalEstimatedAmount:      totalEstimatedAmount,
		Status:                    models.PRStatusPendingApproval,
		CurrentApprovalStage:      initialStage,
		ApprovalRevision:          1,
		VendorInvoiceStatus:       models.PRVendorInvoiceStatusNotRequired,
		Notes:                     payload.Notes,
		Items:                     items,
	}

	if err := s.repo.Create(pr); err != nil {
		return nil, err
	}

	pr.PRNumber = generatePRNumber(pr.ID)
	if err := s.repo.Update(pr); err != nil {
		return nil, err
	}

	s.notifyPurchaseRequestCreated(pr)

	return s.repo.GetByID(pr.ID)
}

func normalizePurchaseRequestInvoiceTerms(invoiceReceivedDate *time.Time, paymentTermsDays *int, dueDate *time.Time) (*time.Time, *int, *time.Time, error) {
	normalizedInvoiceReceivedDate := normalizeDatePointer(invoiceReceivedDate)
	normalizedDueDate := normalizeDatePointer(dueDate)

	var normalizedPaymentTermsDays *int
	if paymentTermsDays != nil {
		if *paymentTermsDays < 0 {
			return nil, nil, nil, fmt.Errorf("payment_terms_days cannot be negative")
		}
		if *paymentTermsDays > 3650 {
			return nil, nil, nil, fmt.Errorf("payment_terms_days cannot exceed 3650 days")
		}
		terms := *paymentTermsDays
		normalizedPaymentTermsDays = &terms
	}

	if normalizedDueDate == nil && normalizedInvoiceReceivedDate != nil && normalizedPaymentTermsDays != nil {
		calculatedDueDate := normalizedInvoiceReceivedDate.AddDate(0, 0, *normalizedPaymentTermsDays)
		normalizedDueDate = &calculatedDueDate
	}

	if normalizedInvoiceReceivedDate != nil && normalizedDueDate != nil && normalizedDueDate.Before(*normalizedInvoiceReceivedDate) {
		return nil, nil, nil, fmt.Errorf("due_date cannot be before vendor_invoice_received_date")
	}

	return normalizedInvoiceReceivedDate, normalizedPaymentTermsDays, normalizedDueDate, nil
}

func normalizeDatePointer(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}

	normalized := time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
	return &normalized
}

func (s *purchaseRequestService) GetPurchaseRequests(userID uint, userRole string, department string, status string, availableForPurchase bool, currentPurchaseID uint) ([]models.PurchaseRequest, error) {
	var requesterID uint
	if !models.CanViewAllTransactionData(userRole) {
		requesterID = userID
	}

	return s.repo.GetAll(department, status, requesterID, availableForPurchase, currentPurchaseID)
}

func (s *purchaseRequestService) GetPurchaseRequestByID(id uint, userID uint, userRole string) (*models.PurchaseRequest, error) {
	pr, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if !models.CanViewAllTransactionData(userRole) && pr.RequesterID != userID {
		return nil, gorm.ErrRecordNotFound
	}

	return pr, nil
}

func (s *purchaseRequestService) UpdatePurchaseRequestProject(id uint, userID uint, userRole string, payload models.PurchaseRequestProjectUpdatePayload) (*models.PurchaseRequest, error) {
	if !canUpdatePurchaseRequestProject(userRole) {
		return nil, fmt.Errorf("only finance staff, accounting, finance manager, or admin can update purchase request project")
	}

	pr, err := s.GetPurchaseRequestByID(id, userID, userRole)
	if err != nil {
		return nil, err
	}
	if pr.Status == models.PRStatusCancelled {
		return nil, fmt.Errorf("purchase request project cannot be updated in current status: %s", pr.Status)
	}
	if pr.Status == models.PRStatusAwaitingRevision {
		return nil, fmt.Errorf("purchase request project cannot be updated while requester revision is open")
	}

	if payload.ProjectID != nil {
		var project models.Project
		if err := s.db.First(&project, *payload.ProjectID).Error; err != nil {
			return nil, fmt.Errorf("invalid project id: %d", *payload.ProjectID)
		}
	}

	pr.ProjectID = payload.ProjectID
	pr.Project = nil
	if err := s.repo.Update(pr); err != nil {
		return nil, err
	}

	if payload.ProjectID != nil {
		if err := s.syncUnsetLinkedPurchaseProjects(pr.ID, *payload.ProjectID); err != nil {
			return nil, err
		}
	}

	return s.repo.GetByID(pr.ID)
}

func (s *purchaseRequestService) UpdatePurchaseRequestInvoiceTerms(id uint, userID uint, userRole string, payload models.PurchaseRequestInvoiceTermsUpdatePayload) (*models.PurchaseRequest, error) {
	if !canUpdatePurchaseRequestProject(userRole) {
		return nil, fmt.Errorf("only finance staff, accounting, finance manager, or admin can update purchase request invoice terms")
	}

	pr, err := s.GetPurchaseRequestByID(id, userID, userRole)
	if err != nil {
		return nil, err
	}

	if pr.Status == models.PRStatusRejected || pr.Status == models.PRStatusCompleted || pr.Status == models.PRStatusCancelled || pr.Status == models.PRStatusAwaitingRevision {
		return nil, fmt.Errorf("purchase request invoice terms cannot be updated in current status: %s", pr.Status)
	}

	invoiceReceivedDate, paymentTermsDays, dueDate, err := normalizePurchaseRequestInvoiceTerms(
		payload.VendorInvoiceReceivedDate,
		payload.PaymentTermsDays,
		payload.DueDate,
	)
	if err != nil {
		return nil, err
	}

	linkedPurchases, err := s.getLinkedPurchasesForPurchaseRequest(pr.ID)
	if err != nil {
		return nil, err
	}
	requiresDueDate := false
	for _, purchase := range linkedPurchases {
		if purchaseRequiresDueDate(purchase) {
			requiresDueDate = true
			break
		}
	}
	if requiresDueDate && dueDate == nil {
		return nil, fmt.Errorf("due_date is required because purchase request %s is already linked to a purchase", pr.PRNumber)
	}
	if !requiresDueDate && payload.DueDate == nil {
		dueDate = nil
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		for _, purchase := range linkedPurchases {
			if dueDate != nil && purchaseRequiresDueDate(purchase) && !canSyncPurchaseDueDateFromPurchaseRequest(purchase.Status) {
				return fmt.Errorf("purchase %s sudah melewati tahap koreksi due date (status: %s)", purchase.Code, purchase.Status)
			}
		}

		if err := tx.Model(&models.PurchaseRequest{}).
			Where("id = ?", pr.ID).
			Updates(map[string]interface{}{
				"vendor_invoice_received_date": invoiceReceivedDate,
				"payment_terms_days":           paymentTermsDays,
				"due_date":                     dueDate,
			}).Error; err != nil {
			return err
		}

		if dueDate != nil {
			for _, purchase := range linkedPurchases {
				if !purchaseRequiresDueDate(purchase) {
					continue
				}
				if err := tx.Model(&models.Purchase{}).
					Where("id = ?", purchase.ID).
					Update("due_date", *dueDate).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.repo.GetByID(pr.ID)
}

func (s *purchaseRequestService) OpenPurchaseRequestRequesterRevision(id uint, userID uint, userRole string, payload models.PurchaseRequestOpenRevisionPayload) (*models.PurchaseRequest, error) {
	if !canOpenPurchaseRequestRequesterRevision(userRole) {
		return nil, fmt.Errorf("only finance staff, accounting, finance manager, or admin can open purchase request revision")
	}

	reason := strings.TrimSpace(payload.Reason)
	if reason == "" {
		return nil, fmt.Errorf("revision reason is required")
	}

	pr, err := s.GetPurchaseRequestByID(id, userID, userRole)
	if err != nil {
		return nil, err
	}
	if pr.Status != models.PRStatusApproved {
		return nil, fmt.Errorf("purchase request %s can only be opened for requester revision after finance manager approval", pr.PRNumber)
	}

	linkedPurchases, err := s.getLinkedPurchasesForPurchaseRequest(pr.ID)
	if err != nil {
		return nil, err
	}
	if len(linkedPurchases) > 0 {
		return nil, fmt.Errorf("purchase request %s cannot be revised because it is already linked to purchase: %s", pr.PRNumber, joinPurchaseCodes(linkedPurchases))
	}

	now := time.Now()
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var current models.PurchaseRequest
		if err := tx.Preload("Items.Product").
			Preload("Requester").
			First(&current, pr.ID).Error; err != nil {
			return err
		}
		if current.Status != models.PRStatusApproved {
			return fmt.Errorf("purchase request %s can only be opened for requester revision after approval", current.PRNumber)
		}

		snapshot, err := buildPurchaseRequestRevisionSnapshot(&current, userID, reason, now)
		if err != nil {
			return err
		}
		if err := tx.Create(snapshot).Error; err != nil {
			return err
		}

		return tx.Model(&models.PurchaseRequest{}).
			Where("id = ?", current.ID).
			Updates(map[string]interface{}{
				"status":                 models.PRStatusAwaitingRevision,
				"current_approval_stage": models.PRApprovalStageFinalized,
				"reapproval_reason":      models.PRReapprovalReasonRequesterRevision,
				"revision_opened_by":     userID,
				"revision_opened_at":     now,
				"revision_reason":        reason,
				"revision_submitted_by":  nil,
				"revision_submitted_at":  nil,
			}).Error
	})
	if err != nil {
		return nil, err
	}

	updated, err := s.repo.GetByID(pr.ID)
	if err != nil {
		return nil, err
	}
	s.notifyRequesterRevisionOpened(updated)
	return updated, nil
}

func (s *purchaseRequestService) SubmitPurchaseRequestRequesterRevision(id uint, userID uint, userRole string, payload models.PurchaseRequestRequesterRevisionPayload, documents []models.PurchaseRequestDocument) (*models.PurchaseRequest, error) {
	if len(documents) == 0 {
		return nil, fmt.Errorf("supporting document is required for requester revision")
	}

	pr, err := s.GetPurchaseRequestByID(id, userID, userRole)
	if err != nil {
		return nil, err
	}
	if !canSubmitPurchaseRequestRequesterRevision(pr, userID, userRole) {
		return nil, fmt.Errorf("only the requester or admin can submit revision for this purchase request")
	}
	if pr.Status != models.PRStatusAwaitingRevision || pr.ReapprovalReason != models.PRReapprovalReasonRequesterRevision {
		return nil, fmt.Errorf("purchase request %s is not waiting for requester revision", pr.PRNumber)
	}

	linkedPurchases, err := s.getLinkedPurchasesForPurchaseRequest(pr.ID)
	if err != nil {
		return nil, err
	}
	if len(linkedPurchases) > 0 {
		return nil, fmt.Errorf("requester revision cannot be submitted because this purchase request is already linked to purchase: %s", joinPurchaseCodes(linkedPurchases))
	}

	itemUpdates, err := buildPurchaseRequestRequesterRevisionMap(payload.Items, pr.Items)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	previousRevision := pr.ApprovalRevision
	nextRevision := previousRevision + 1
	if nextRevision < 2 {
		nextRevision = 2
	}
	revisionNotes := strings.TrimSpace(payload.Notes)
	var totalEstimatedAmount float64

	err = s.db.Transaction(func(tx *gorm.DB) error {
		var current models.PurchaseRequest
		if err := tx.Preload("Items").First(&current, pr.ID).Error; err != nil {
			return err
		}
		if current.Status != models.PRStatusAwaitingRevision || current.ReapprovalReason != models.PRReapprovalReasonRequesterRevision {
			return fmt.Errorf("purchase request %s is not waiting for requester revision", current.PRNumber)
		}

		for index := range current.Items {
			item := &current.Items[index]
			update, exists := itemUpdates[item.ID]
			if !exists {
				return fmt.Errorf("requester revision must include every purchase request item")
			}

			item.Quantity = update.Quantity
			item.EstimatedPrice = update.EstimatedPrice
			item.TotalEstimatedPrice = float64(update.Quantity) * update.EstimatedPrice
			item.Notes = update.Notes
			item.ApprovalStatus = models.PRItemDecisionPending
			item.FinanceStaffDecision = models.PRItemDecisionPending
			item.FinanceManagerDecision = models.PRItemDecisionPending
			item.DirectorDecision = models.PRItemDecisionPending
			item.PurchaseID = nil
			item.PurchasedAt = nil

			totalEstimatedAmount += item.TotalEstimatedPrice
		}

		current.TotalEstimatedAmount = totalEstimatedAmount
		current.Status = models.PRStatusPendingApproval
		current.CurrentApprovalStage = models.PRApprovalStageFinanceStaff
		current.ApprovalRevision = nextRevision
		current.ReapprovalReason = models.PRReapprovalReasonRequesterRevision
		current.ApprovedBy = nil
		current.ApprovedAt = nil
		current.FinanceStaffApprovedBy = nil
		current.FinanceStaffApprovedAt = nil
		current.FinanceStaffReviewNotes = ""
		current.FinanceManagerApprovedBy = nil
		current.FinanceManagerApprovedAt = nil
		current.FinanceManagerReviewNotes = ""
		current.DirectorApprovedBy = nil
		current.DirectorApprovedAt = nil
		current.RejectionReason = ""
		current.RevisionSubmittedBy = &userID
		current.RevisionSubmittedAt = &now

		for index := range documents {
			documents[index].PurchaseRequestID = pr.ID
			documents[index].DocumentType = models.PurchaseRequestDocumentRequesterRevision
			documents[index].UploadedBy = userID
			documents[index].ApprovalRevision = nextRevision
		}
		if err := tx.Create(&documents).Error; err != nil {
			return err
		}

		var snapshot models.PurchaseRequestRevisionSnapshot
		if err := tx.Where("purchase_request_id = ? AND revision_number = ? AND reapproval_reason = ?", pr.ID, previousRevision, models.PRReapprovalReasonRequesterRevision).
			Order("id desc").
			First(&snapshot).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.PurchaseRequestRevisionSnapshot{}).
			Where("id = ?", snapshot.ID).
			Updates(map[string]interface{}{
				"submitted_by":               userID,
				"submitted_at":               now,
				"submitted_notes":            revisionNotes,
				"new_total_estimated_amount": totalEstimatedAmount,
			}).Error; err != nil {
			return err
		}

		return tx.Session(&gorm.Session{FullSaveAssociations: true}).Save(&current).Error
	})
	if err != nil {
		return nil, err
	}

	updated, err := s.repo.GetByID(pr.ID)
	if err != nil {
		return nil, err
	}

	s.notifyPurchaseRequestRequesterRevision(updated, len(documents))
	return updated, nil
}

func (s *purchaseRequestService) SubmitPurchaseRequestInvoiceRevision(id uint, userID uint, userRole string, payload models.PurchaseRequestInvoiceRevisionPayload, documents []models.PurchaseRequestDocument) (*models.PurchaseRequest, error) {
	if len(documents) == 0 {
		return nil, fmt.Errorf("vendor invoice document is required")
	}

	pr, err := s.GetPurchaseRequestByID(id, userID, userRole)
	if err != nil {
		return nil, err
	}
	if !canSubmitPurchaseRequestInvoiceRevision(pr, userID, userRole) {
		return nil, fmt.Errorf("only the requester or admin can submit invoice revision for this purchase request")
	}
	if !pr.RequiresVendorInvoice {
		return nil, fmt.Errorf("purchase request %s was not marked as requiring vendor invoice revision by finance staff", pr.PRNumber)
	}
	if pr.Status != models.PRStatusApproved {
		return nil, fmt.Errorf("invoice revision can only be submitted after purchase request is approved")
	}
	if pr.Status == models.PRStatusCompleted || pr.Status == models.PRStatusRejected {
		return nil, fmt.Errorf("invoice revision cannot be submitted in current status: %s", pr.Status)
	}

	linkedPurchases, err := s.getLinkedPurchasesForPurchaseRequest(pr.ID)
	if err != nil {
		return nil, err
	}
	if len(linkedPurchases) > 0 {
		codes := make([]string, 0, len(linkedPurchases))
		for _, purchase := range linkedPurchases {
			if strings.TrimSpace(purchase.Code) != "" {
				codes = append(codes, purchase.Code)
			}
		}
		if len(codes) == 0 {
			return nil, fmt.Errorf("invoice revision cannot be submitted because this purchase request is already linked to purchase transactions")
		}
		return nil, fmt.Errorf("invoice revision cannot be submitted because this purchase request is already linked to purchase: %s", strings.Join(codes, ", "))
	}

	itemUpdates, err := buildPurchaseRequestInvoiceRevisionMap(payload.Items, pr.Items)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	nextRevision := pr.ApprovalRevision + 1
	if nextRevision < 2 {
		nextRevision = 2
	}
	revisionNotes := strings.TrimSpace(payload.Notes)

	err = s.db.Transaction(func(tx *gorm.DB) error {
		var current models.PurchaseRequest
		if err := tx.Preload("Items").First(&current, pr.ID).Error; err != nil {
			return err
		}

		var totalEstimatedAmount float64
		for index := range current.Items {
			item := &current.Items[index]
			update, exists := itemUpdates[item.ID]
			if !exists {
				return fmt.Errorf("invoice revision must include every purchase request item")
			}

			item.Quantity = update.Quantity
			item.EstimatedPrice = update.EstimatedPrice
			item.TotalEstimatedPrice = float64(update.Quantity) * update.EstimatedPrice
			item.Notes = update.Notes
			item.ApprovalStatus = models.PRItemDecisionPending
			item.FinanceStaffDecision = models.PRItemDecisionPending
			item.FinanceManagerDecision = models.PRItemDecisionPending
			item.DirectorDecision = models.PRItemDecisionPending
			item.PurchaseID = nil
			item.PurchasedAt = nil

			totalEstimatedAmount += item.TotalEstimatedPrice
		}

		current.TotalEstimatedAmount = totalEstimatedAmount
		current.Status = models.PRStatusPendingApproval
		current.CurrentApprovalStage = models.PRApprovalStageFinanceStaff
		current.ApprovalRevision = nextRevision
		current.ReapprovalReason = models.PRReapprovalReasonInvoiceRevision
		current.ApprovedBy = nil
		current.ApprovedAt = nil
		current.FinanceStaffApprovedBy = nil
		current.FinanceStaffApprovedAt = nil
		current.FinanceStaffReviewNotes = ""
		current.FinanceManagerApprovedBy = nil
		current.FinanceManagerApprovedAt = nil
		current.FinanceManagerReviewNotes = ""
		current.DirectorApprovedBy = nil
		current.DirectorApprovedAt = nil
		current.RejectionReason = ""
		current.RequiresVendorInvoice = true
		current.VendorInvoiceStatus = models.PRVendorInvoiceStatusPendingApproval
		current.VendorInvoiceSubmittedBy = &userID
		current.VendorInvoiceSubmittedAt = &now
		current.VendorInvoiceRevisionNotes = revisionNotes

		for index := range documents {
			documents[index].PurchaseRequestID = pr.ID
			documents[index].DocumentType = models.PurchaseRequestDocumentVendorInvoice
			documents[index].UploadedBy = userID
			documents[index].ApprovalRevision = nextRevision
		}
		if err := tx.Create(&documents).Error; err != nil {
			return err
		}

		return tx.Session(&gorm.Session{FullSaveAssociations: true}).Save(&current).Error
	})
	if err != nil {
		return nil, err
	}

	updated, err := s.repo.GetByID(pr.ID)
	if err != nil {
		return nil, err
	}

	s.notifyPurchaseRequestInvoiceRevision(updated, len(documents))
	return updated, nil
}

func (s *purchaseRequestService) getLinkedPurchasesForPurchaseRequest(purchaseRequestID uint) ([]models.Purchase, error) {
	var purchases []models.Purchase
	err := s.db.Raw(`
		SELECT DISTINCT p.*
		FROM purchases p
		WHERE p.deleted_at IS NULL
			AND p.status <> ?
			AND (
				p.purchase_request_id = ?
				OR EXISTS (
					SELECT 1
					FROM purchase_request_items pri
					WHERE pri.purchase_request_id = ?
						AND pri.purchase_id = p.id
						AND pri.deleted_at IS NULL
				)
				OR EXISTS (
					SELECT 1
					FROM purchase_items pi
					JOIN purchase_request_items pri
						ON pri.id = pi.purchase_request_item_id
						AND pri.deleted_at IS NULL
					WHERE pi.purchase_id = p.id
						AND pi.deleted_at IS NULL
						AND pri.purchase_request_id = ?
				)
			)
		ORDER BY p.id ASC
	`, models.PurchaseStatusCancelled, purchaseRequestID, purchaseRequestID, purchaseRequestID).Scan(&purchases).Error

	return purchases, err
}

func purchaseRequiresDueDate(purchase models.Purchase) bool {
	method := strings.ToUpper(strings.TrimSpace(purchase.PaymentMethod))
	return method == "" || method == models.PurchasePaymentCredit
}

func canSyncPurchaseDueDateFromPurchaseRequest(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case models.PurchaseStatusDraft,
		models.PurchaseStatusPending,
		models.PurchaseStatusPendingApproval,
		models.PurchaseStatusPendingAccounting,
		models.PurchaseStatusPendingFinalApproval:
		return true
	default:
		return false
	}
}

func (s *purchaseRequestService) ReviewPurchaseRequest(id uint, reviewerID uint, reviewerRole string, payload models.PurchaseRequestReviewPayload) (string, error) {
	pr, err := s.repo.GetByID(id)
	if err != nil {
		return "", err
	}

	stage, err := resolvePurchaseRequestReviewStage(pr, reviewerRole)
	if err != nil {
		return "", err
	}

	decisionByItemID, err := buildPurchaseRequestDecisionMap(payload.Items)
	if err != nil {
		return "", err
	}

	now := time.Now()
	reviewNotes := strings.TrimSpace(payload.Reason)

	switch stage {
	case models.PRApprovalStageFinanceStaff:
		for index := range pr.Items {
			item := &pr.Items[index]
			decision, exists := decisionByItemID[item.ID]
			if !exists {
				return "", fmt.Errorf("finance staff must decide every item before forwarding to finance manager")
			}
			item.FinanceStaffDecision = decision
		}

		pr.Status = models.PRStatusPendingApproval
		pr.CurrentApprovalStage = models.PRApprovalStageFinanceManager
		pr.FinanceStaffApprovedBy = &reviewerID
		pr.FinanceStaffApprovedAt = &now
		pr.FinanceStaffReviewNotes = reviewNotes
		pr.RejectionReason = ""
		if payload.RequireVendorInvoice && !pr.RequiresVendorInvoice {
			pr.RequiresVendorInvoice = true
			pr.VendorInvoiceStatus = models.PRVendorInvoiceStatusAwaitingInvoice
			pr.VendorInvoiceRequestedBy = &reviewerID
			pr.VendorInvoiceRequestedAt = &now
		} else if !pr.RequiresVendorInvoice && pr.VendorInvoiceStatus == "" {
			pr.VendorInvoiceStatus = models.PRVendorInvoiceStatusNotRequired
		}

		if err := s.repo.Update(pr); err != nil {
			return "", err
		}

		s.notifyFinanceManagerReview(pr)
		return "Review finance staff berhasil dikirim ke finance manager", nil

	case models.PRApprovalStageFinanceManager:
		approvedCount := 0
		rejectedCount := 0

		for index := range pr.Items {
			item := &pr.Items[index]
			decision, exists := decisionByItemID[item.ID]
			if !exists {
				return "", fmt.Errorf("finance manager must decide every item before final approval")
			}

			item.FinanceManagerDecision = decision
			if payload.EscalateToDirector {
				item.ApprovalStatus = models.PRItemDecisionPending
			} else {
				item.ApprovalStatus = decision
			}

			switch decision {
			case models.PRItemDecisionApproved:
				approvedCount++
			case models.PRItemDecisionRejected:
				rejectedCount++
				item.PurchaseID = nil
				item.PurchasedAt = nil
			}
		}

		pr.FinanceManagerApprovedBy = &reviewerID
		pr.FinanceManagerApprovedAt = &now
		pr.FinanceManagerReviewNotes = reviewNotes
		if payload.EscalateToDirector {
			pr.Status = models.PRStatusPendingApproval
			pr.CurrentApprovalStage = models.PRApprovalStageDirector
			pr.ApprovedBy = nil
			pr.ApprovedAt = nil
			pr.RejectionReason = ""

			if err := s.repo.Update(pr); err != nil {
				return "", err
			}

			s.notifyDirectorReview(pr)
			return "Review finance manager berhasil dikirim ke director", nil
		}

		pr.CurrentApprovalStage = models.PRApprovalStageFinalized

		if approvedCount > 0 {
			pr.Status = models.PRStatusApproved
			pr.ApprovedBy = &reviewerID
			pr.ApprovedAt = &now
			pr.RejectionReason = reviewNotes
			applyPurchaseRequestVendorInvoiceApprovalState(pr)
		} else {
			pr.Status = models.PRStatusRejected
			pr.ApprovedBy = nil
			pr.ApprovedAt = nil
			if reviewNotes != "" {
				pr.RejectionReason = reviewNotes
			} else {
				pr.RejectionReason = "Semua item ditolak oleh finance manager"
			}
		}

		pr.TotalEstimatedAmount = calculateFinalPurchaseRequestTotal(pr.Items)

		if err := s.repo.Update(pr); err != nil {
			return "", err
		}

		s.notifyRequesterAfterFinalReview(pr, approvedCount, rejectedCount)

		if approvedCount == 0 {
			return "Purchase request ditolak seluruhnya oleh finance manager", nil
		}

		if rejectedCount > 0 {
			return fmt.Sprintf("Final review selesai: %d item disetujui dan %d item ditolak", approvedCount, rejectedCount), nil
		}

		return "Purchase request disetujui seluruhnya oleh finance manager", nil

	case models.PRApprovalStageDirector:
		approvedCount := 0
		rejectedCount := 0

		for index := range pr.Items {
			item := &pr.Items[index]
			decision, exists := decisionByItemID[item.ID]
			if !exists {
				return "", fmt.Errorf("director must decide every item before final approval")
			}

			item.DirectorDecision = decision
			item.ApprovalStatus = decision

			switch decision {
			case models.PRItemDecisionApproved:
				approvedCount++
			case models.PRItemDecisionRejected:
				rejectedCount++
				item.PurchaseID = nil
				item.PurchasedAt = nil
			}
		}

		pr.CurrentApprovalStage = models.PRApprovalStageFinalized
		pr.DirectorApprovedBy = &reviewerID
		pr.DirectorApprovedAt = &now

		if approvedCount > 0 {
			pr.Status = models.PRStatusApproved
			pr.ApprovedBy = &reviewerID
			pr.ApprovedAt = &now
			pr.RejectionReason = reviewNotes
			applyPurchaseRequestVendorInvoiceApprovalState(pr)
		} else {
			pr.Status = models.PRStatusRejected
			pr.ApprovedBy = nil
			pr.ApprovedAt = nil
			if reviewNotes != "" {
				pr.RejectionReason = reviewNotes
			} else {
				pr.RejectionReason = "Semua item ditolak oleh director"
			}
		}

		pr.TotalEstimatedAmount = calculateFinalPurchaseRequestTotal(pr.Items)

		if err := s.repo.Update(pr); err != nil {
			return "", err
		}

		s.notifyRequesterAfterFinalReview(pr, approvedCount, rejectedCount)

		if approvedCount == 0 {
			return "Purchase request ditolak seluruhnya oleh director", nil
		}

		if rejectedCount > 0 {
			return fmt.Sprintf("Final review director selesai: %d item disetujui dan %d item ditolak", approvedCount, rejectedCount), nil
		}

		return "Purchase request disetujui seluruhnya oleh director", nil

	default:
		return "", fmt.Errorf("purchase request %s is not in a reviewable stage", pr.PRNumber)
	}
}

func (s *purchaseRequestService) ApprovePurchaseRequest(id uint, approverID uint, approverRole string) (string, error) {
	pr, err := s.repo.GetByID(id)
	if err != nil {
		return "", err
	}

	payload := buildBulkPurchaseRequestReviewPayload(pr, models.PRItemDecisionApproved)
	return s.ReviewPurchaseRequest(id, approverID, approverRole, payload)
}

func (s *purchaseRequestService) RejectPurchaseRequest(id uint, approverID uint, approverRole string, reason string) (string, error) {
	pr, err := s.repo.GetByID(id)
	if err != nil {
		return "", err
	}

	payload := buildBulkPurchaseRequestReviewPayload(pr, models.PRItemDecisionRejected)
	payload.Reason = reason

	return s.ReviewPurchaseRequest(id, approverID, approverRole, payload)
}

func (s *purchaseRequestService) CancelPurchaseRequest(id uint, userID uint, userRole string, reason string) (*models.PurchaseRequest, error) {
	roleNorm := models.NormalizeRole(userRole)
	if roleNorm != models.RoleAdmin && !models.IsFinanceManagerRole(roleNorm) {
		return nil, fmt.Errorf("only finance manager or admin can cancel purchase requests")
	}

	cancelReason := strings.TrimSpace(reason)
	if cancelReason == "" {
		return nil, fmt.Errorf("cancel reason is required")
	}

	pr, err := s.GetPurchaseRequestByID(id, userID, userRole)
	if err != nil {
		return nil, err
	}

	switch pr.Status {
	case models.PRStatusApproved:
		// Cancellation is intentionally limited to approved PRs that have not
		// been realized yet, so the approval history remains intact.
	case models.PRStatusCancelled:
		return nil, fmt.Errorf("purchase request %s is already cancelled", pr.PRNumber)
	case models.PRStatusCompleted:
		return nil, fmt.Errorf("completed purchase request %s cannot be cancelled from PR menu", pr.PRNumber)
	case models.PRStatusRejected:
		return nil, fmt.Errorf("rejected purchase request %s does not need cancellation", pr.PRNumber)
	default:
		return nil, fmt.Errorf("purchase request %s can only be cancelled after finance manager approval", pr.PRNumber)
	}

	linkedPurchases, err := s.getLinkedPurchasesForPurchaseRequest(pr.ID)
	if err != nil {
		return nil, err
	}
	if len(linkedPurchases) > 0 {
		codes := make([]string, 0, len(linkedPurchases))
		for _, purchase := range linkedPurchases {
			if strings.TrimSpace(purchase.Code) != "" {
				codes = append(codes, purchase.Code)
			}
		}
		if len(codes) == 0 {
			return nil, fmt.Errorf("purchase request %s is already linked to purchase transactions", pr.PRNumber)
		}
		return nil, fmt.Errorf("purchase request %s is already linked to purchase: %s", pr.PRNumber, strings.Join(codes, ", "))
	}

	now := time.Now()
	pr.Status = models.PRStatusCancelled
	pr.CurrentApprovalStage = models.PRApprovalStageFinalized
	pr.CancelledBy = &userID
	pr.CancelledAt = &now
	pr.CancelledReason = cancelReason
	pr.ReapprovalReason = ""

	if err := s.repo.Update(pr); err != nil {
		return nil, err
	}

	return s.repo.GetByID(pr.ID)
}

func (s *purchaseRequestService) DeletePurchaseRequest(id uint) error {
	return s.repo.Delete(id)
}

func (s *purchaseRequestService) UploadPurchaseRequestDocument(purchaseRequestID uint, documentType, fileName, filePath string, fileSize int64, mimeType string, userID uint, userRole string) (*models.PurchaseRequestDocument, error) {
	pr, err := s.GetPurchaseRequestByID(purchaseRequestID, userID, userRole)
	if err != nil {
		return nil, err
	}
	if pr.Status == models.PRStatusCompleted {
		return nil, fmt.Errorf("supporting documents cannot be added to a completed purchase request")
	}
	if pr.Status == models.PRStatusRejected {
		return nil, fmt.Errorf("supporting documents cannot be added to a rejected purchase request")
	}
	if pr.Status == models.PRStatusCancelled {
		return nil, fmt.Errorf("supporting documents cannot be added to a cancelled purchase request")
	}
	if pr.Status == models.PRStatusAwaitingRevision {
		return nil, fmt.Errorf("supporting documents for requester revision must be submitted through requester revision workflow")
	}
	if pr.Status == models.PRStatusPendingApproval && pr.ReapprovalReason == models.PRReapprovalReasonAdditionalDocuments {
		return nil, fmt.Errorf("additional documents have already been submitted and are awaiting approval")
	}

	if documentType == "" {
		documentType = models.PurchaseRequestDocumentSupporting
	}
	documentType = strings.ToUpper(strings.TrimSpace(documentType))
	if documentType == models.PurchaseRequestDocumentVendorInvoice {
		return nil, fmt.Errorf("vendor invoice documents must be submitted through invoice revision workflow")
	}
	if documentType == models.PurchaseRequestDocumentRealization {
		return nil, fmt.Errorf("realization documents must be submitted through realization document workflow")
	}

	revision := pr.ApprovalRevision
	if revision < 1 {
		revision = 1
	}
	if purchaseRequestApprovalHasProgressed(pr) {
		revision++
	}

	document := &models.PurchaseRequestDocument{
		PurchaseRequestID: purchaseRequestID,
		DocumentType:      documentType,
		FileName:          fileName,
		FilePath:          filePath,
		FileSize:          fileSize,
		MimeType:          mimeType,
		UploadedBy:        userID,
		ApprovalRevision:  revision,
	}

	if err := s.repo.CreateDocument(document); err != nil {
		return nil, err
	}

	return s.repo.GetDocumentByID(document.ID)
}

func (s *purchaseRequestService) UploadPurchaseRequestRealizationDocument(purchaseRequestID uint, fileName, filePath string, fileSize int64, mimeType string, userID uint, userRole string) (*models.PurchaseRequestDocument, error) {
	pr, err := s.GetPurchaseRequestByID(purchaseRequestID, userID, userRole)
	if err != nil {
		return nil, err
	}

	roleNorm := models.NormalizeRole(userRole)
	canUpload := roleNorm == models.RoleAdmin || models.IsFinanceTeamRole(roleNorm) || pr.RequesterID == userID
	if !canUpload {
		return nil, fmt.Errorf("only the requester, finance team, or admin can upload realization documents")
	}
	if pr.Status != models.PRStatusApproved && pr.Status != models.PRStatusCompleted {
		return nil, fmt.Errorf("realization documents can only be uploaded after purchase request approval")
	}

	revision := pr.ApprovalRevision
	if revision < 1 {
		revision = 1
	}

	document := &models.PurchaseRequestDocument{
		PurchaseRequestID: purchaseRequestID,
		DocumentType:      models.PurchaseRequestDocumentRealization,
		FileName:          fileName,
		FilePath:          filePath,
		FileSize:          fileSize,
		MimeType:          mimeType,
		UploadedBy:        userID,
		ApprovalRevision:  revision,
	}

	if err := s.repo.CreateDocument(document); err != nil {
		return nil, err
	}

	return s.repo.GetDocumentByID(document.ID)
}

// SubmitAdditionalDocumentsForApproval reopens a PR approval when documents
// are added after Finance Staff or Finance Manager has already reviewed it.
// The existing item decisions are retained as the reviewers' starting point,
// while all approval identities/timestamps are reset for a fresh sign-off.
func (s *purchaseRequestService) SubmitAdditionalDocumentsForApproval(purchaseRequestID uint, userID uint, userRole string, documentCount int) (bool, error) {
	if documentCount < 1 {
		return false, fmt.Errorf("at least one supporting document is required")
	}

	pr, err := s.GetPurchaseRequestByID(purchaseRequestID, userID, userRole)
	if err != nil {
		return false, err
	}
	if pr.Status == models.PRStatusCompleted {
		return false, fmt.Errorf("supporting documents cannot be submitted for a completed purchase request")
	}
	if pr.Status == models.PRStatusRejected {
		return false, fmt.Errorf("supporting documents cannot be submitted for a rejected purchase request")
	}
	if pr.Status == models.PRStatusCancelled {
		return false, fmt.Errorf("supporting documents cannot be submitted for a cancelled purchase request")
	}
	if pr.Status == models.PRStatusAwaitingRevision {
		return false, fmt.Errorf("supporting documents for requester revision must be submitted through requester revision workflow")
	}
	if pr.Status == models.PRStatusPendingApproval && pr.ReapprovalReason == models.PRReapprovalReasonAdditionalDocuments {
		return false, fmt.Errorf("additional documents have already been submitted and are awaiting approval")
	}

	reapprovalStarted := purchaseRequestApprovalHasProgressed(pr)
	if pr.ApprovalRevision < 1 {
		pr.ApprovalRevision = 1
	}
	if reapprovalStarted {
		pr.ApprovalRevision++
		pr.Status = models.PRStatusPendingApproval
		pr.CurrentApprovalStage = models.PRApprovalStageFinanceStaff
		pr.ApprovedBy = nil
		pr.ApprovedAt = nil
		pr.Approver = nil
		pr.FinanceStaffApprovedBy = nil
		pr.FinanceStaffApprovedAt = nil
		pr.FinanceStaffReviewNotes = ""
		pr.FinanceStaffApprover = nil
		pr.FinanceManagerApprovedBy = nil
		pr.FinanceManagerApprovedAt = nil
		pr.FinanceManagerReviewNotes = ""
		pr.FinanceManagerApprover = nil
		pr.DirectorApprovedBy = nil
		pr.DirectorApprovedAt = nil
		pr.DirectorApprover = nil
		pr.RejectionReason = ""
	}
	pr.ReapprovalReason = models.PRReapprovalReasonAdditionalDocuments

	if err := s.repo.Update(pr); err != nil {
		return false, err
	}

	s.notifyAdditionalPurchaseRequestDocuments(pr, documentCount, reapprovalStarted)
	return reapprovalStarted, nil
}

func purchaseRequestApprovalHasProgressed(pr *models.PurchaseRequest) bool {
	if pr == nil {
		return false
	}
	if pr.Status == models.PRStatusApproved {
		return true
	}
	if pr.CurrentApprovalStage == models.PRApprovalStageDirector ||
		pr.CurrentApprovalStage == models.PRApprovalStageFinalized {
		return true
	}
	return pr.FinanceStaffApprovedBy != nil || pr.FinanceManagerApprovedBy != nil || pr.DirectorApprovedBy != nil
}

func canUpdatePurchaseRequestProject(role string) bool {
	roleNorm := models.NormalizeRole(role)
	return roleNorm == models.RoleAdmin || models.IsFinanceTeamRole(roleNorm)
}

func (s *purchaseRequestService) syncUnsetLinkedPurchaseProjects(purchaseRequestID uint, projectID uint) error {
	if err := s.db.Model(&models.Purchase{}).
		Where("purchase_request_id = ? AND project_id IS NULL", purchaseRequestID).
		Update("project_id", projectID).Error; err != nil {
		return fmt.Errorf("failed to sync linked purchase project: %w", err)
	}

	if err := s.db.Model(&models.Purchase{}).
		Where(`
			project_id IS NULL
			AND id IN (
				SELECT purchase_id
				FROM purchase_request_items
				WHERE purchase_request_id = ?
					AND purchase_id IS NOT NULL
					AND deleted_at IS NULL
			)
		`, purchaseRequestID).
		Update("project_id", projectID).Error; err != nil {
		return fmt.Errorf("failed to sync linked purchase item projects: %w", err)
	}

	return nil
}

func (s *purchaseRequestService) GetPurchaseRequestDocuments(purchaseRequestID uint, userID uint, userRole string) ([]models.PurchaseRequestDocument, error) {
	if _, err := s.GetPurchaseRequestByID(purchaseRequestID, userID, userRole); err != nil {
		return nil, err
	}

	return s.repo.GetDocumentsByPurchaseRequestID(purchaseRequestID)
}

func (s *purchaseRequestService) GetPurchaseRequestDocumentForDownload(documentID uint, userID uint, userRole string) (*models.PurchaseRequestDocument, error) {
	document, err := s.repo.GetDocumentByID(documentID)
	if err != nil {
		return nil, err
	}

	if _, err := s.GetPurchaseRequestByID(document.PurchaseRequestID, userID, userRole); err != nil {
		return nil, err
	}

	return document, nil
}

func (s *purchaseRequestService) DeletePurchaseRequestDocument(documentID uint, userID uint, userRole string) error {
	document, err := s.repo.GetDocumentByID(documentID)
	if err != nil {
		return err
	}

	if _, err := s.GetPurchaseRequestByID(document.PurchaseRequestID, userID, userRole); err != nil {
		return err
	}
	if document.PurchaseRequest.Status == models.PRStatusPendingApproval &&
		(document.PurchaseRequest.ReapprovalReason == models.PRReapprovalReasonAdditionalDocuments ||
			document.PurchaseRequest.ReapprovalReason == models.PRReapprovalReasonInvoiceRevision ||
			document.PurchaseRequest.ReapprovalReason == models.PRReapprovalReasonRequesterRevision) {
		return fmt.Errorf("submitted supporting documents cannot be deleted while approval is in progress")
	}

	return s.repo.DeleteDocument(documentID)
}

func (s *purchaseRequestService) SyncPurchaseRequestItemsWithPurchase(purchase *models.Purchase) error {
	if purchase == nil || purchase.PurchaseRequestID == nil {
		return nil
	}

	selectedItemIDs := make(map[uint]struct{})
	for _, item := range purchase.PurchaseItems {
		if item.PurchaseRequestItemID != nil {
			selectedItemIDs[*item.PurchaseRequestItemID] = struct{}{}
		}
	}

	if len(selectedItemIDs) == 0 {
		return nil
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var pr models.PurchaseRequest
		if err := tx.Preload("Items").First(&pr, *purchase.PurchaseRequestID).Error; err != nil {
			return err
		}

		now := time.Now()
		for index := range pr.Items {
			item := &pr.Items[index]

			if item.PurchaseID != nil && *item.PurchaseID == purchase.ID {
				if _, stillSelected := selectedItemIDs[item.ID]; !stillSelected {
					item.PurchaseID = nil
					item.PurchasedAt = nil
				}
			}

			if _, shouldReserve := selectedItemIDs[item.ID]; shouldReserve {
				if item.ApprovalStatus != models.PRItemDecisionApproved {
					return fmt.Errorf("purchase request item %d is not approved", item.ID)
				}
				if item.PurchaseID != nil && *item.PurchaseID != purchase.ID {
					return fmt.Errorf("purchase request item %d is already linked to another purchase", item.ID)
				}
				item.PurchaseID = &purchase.ID
				item.PurchasedAt = &now
			}
		}

		return tx.Session(&gorm.Session{FullSaveAssociations: true}).Save(&pr).Error
	})
}

func (s *purchaseRequestService) ReleasePurchaseRequestItemsFromPurchase(purchase *models.Purchase) error {
	if purchase == nil || purchase.PurchaseRequestID == nil {
		return nil
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var pr models.PurchaseRequest
		if err := tx.Preload("Items").First(&pr, *purchase.PurchaseRequestID).Error; err != nil {
			return err
		}

		changed := false
		for index := range pr.Items {
			item := &pr.Items[index]
			if item.PurchaseID != nil && *item.PurchaseID == purchase.ID {
				item.PurchaseID = nil
				item.PurchasedAt = nil
				changed = true
			}
		}

		if !changed {
			return nil
		}

		return tx.Session(&gorm.Session{FullSaveAssociations: true}).Save(&pr).Error
	})
}

type purchaseRequestInvoiceRevisionItemUpdate struct {
	Quantity       int
	EstimatedPrice float64
	Notes          string
}

func canSubmitPurchaseRequestInvoiceRevision(pr *models.PurchaseRequest, userID uint, userRole string) bool {
	if pr == nil {
		return false
	}
	roleNorm := models.NormalizeRole(userRole)
	if roleNorm == models.RoleAdmin {
		return true
	}
	return pr.RequesterID == userID
}

func canOpenPurchaseRequestRequesterRevision(role string) bool {
	roleNorm := models.NormalizeRole(role)
	return roleNorm == models.RoleAdmin || models.IsFinanceTeamRole(roleNorm)
}

func canSubmitPurchaseRequestRequesterRevision(pr *models.PurchaseRequest, userID uint, userRole string) bool {
	if pr == nil {
		return false
	}
	roleNorm := models.NormalizeRole(userRole)
	if roleNorm == models.RoleAdmin {
		return true
	}
	return pr.RequesterID == userID
}

type purchaseRequestRevisionSnapshotItem struct {
	ID                     uint    `json:"id"`
	ProductID              *uint   `json:"product_id,omitempty"`
	ProductName            string  `json:"product_name,omitempty"`
	ItemName               string  `json:"item_name"`
	Quantity               int     `json:"quantity"`
	EstimatedPrice         float64 `json:"estimated_price"`
	TotalEstimatedPrice    float64 `json:"total_estimated_price"`
	ApprovalStatus         string  `json:"approval_status"`
	FinanceStaffDecision   string  `json:"finance_staff_decision"`
	FinanceManagerDecision string  `json:"finance_manager_decision"`
	DirectorDecision       string  `json:"director_decision"`
	Notes                  string  `json:"notes"`
}

func buildPurchaseRequestRevisionSnapshot(pr *models.PurchaseRequest, openedBy uint, reason string, openedAt time.Time) (*models.PurchaseRequestRevisionSnapshot, error) {
	if pr == nil {
		return nil, fmt.Errorf("purchase request is required")
	}

	items := make([]purchaseRequestRevisionSnapshotItem, 0, len(pr.Items))
	for _, item := range pr.Items {
		productName := ""
		if item.Product != nil {
			productName = item.Product.Name
		}
		items = append(items, purchaseRequestRevisionSnapshotItem{
			ID:                     item.ID,
			ProductID:              item.ProductID,
			ProductName:            productName,
			ItemName:               item.ItemName,
			Quantity:               item.Quantity,
			EstimatedPrice:         item.EstimatedPrice,
			TotalEstimatedPrice:    item.TotalEstimatedPrice,
			ApprovalStatus:         item.ApprovalStatus,
			FinanceStaffDecision:   item.FinanceStaffDecision,
			FinanceManagerDecision: item.FinanceManagerDecision,
			DirectorDecision:       item.DirectorDecision,
			Notes:                  item.Notes,
		})
	}
	itemSnapshot, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("failed to capture purchase request revision snapshot: %w", err)
	}

	revisionNumber := pr.ApprovalRevision
	if revisionNumber < 1 {
		revisionNumber = 1
	}

	return &models.PurchaseRequestRevisionSnapshot{
		PurchaseRequestID:            pr.ID,
		RevisionNumber:               revisionNumber,
		ReapprovalReason:             models.PRReapprovalReasonRequesterRevision,
		PreviousStatus:               pr.Status,
		PreviousApprovalStage:        pr.CurrentApprovalStage,
		PreviousTotalEstimatedAmount: pr.TotalEstimatedAmount,
		ItemSnapshot:                 string(itemSnapshot),
		OpenedBy:                     openedBy,
		OpenedAt:                     openedAt,
		OpenReason:                   reason,
		ApprovedBy:                   pr.ApprovedBy,
		ApprovedAt:                   pr.ApprovedAt,
		FinanceStaffApprovedBy:       pr.FinanceStaffApprovedBy,
		FinanceStaffApprovedAt:       pr.FinanceStaffApprovedAt,
		FinanceManagerApprovedBy:     pr.FinanceManagerApprovedBy,
		FinanceManagerApprovedAt:     pr.FinanceManagerApprovedAt,
		DirectorApprovedBy:           pr.DirectorApprovedBy,
		DirectorApprovedAt:           pr.DirectorApprovedAt,
	}, nil
}

func buildPurchaseRequestInvoiceRevisionMap(items []models.PurchaseRequestInvoiceRevisionItemPayload, existingItems []models.PurchaseRequestItem) (map[uint]purchaseRequestInvoiceRevisionItemUpdate, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("invoice revision items are required")
	}

	existingByID := make(map[uint]struct{}, len(existingItems))
	for _, item := range existingItems {
		existingByID[item.ID] = struct{}{}
	}

	updates := make(map[uint]purchaseRequestInvoiceRevisionItemUpdate, len(items))
	for _, item := range items {
		if item.ItemID == 0 {
			return nil, fmt.Errorf("item_id is required")
		}
		if _, exists := existingByID[item.ItemID]; !exists {
			return nil, fmt.Errorf("purchase request item %d does not belong to this purchase request", item.ItemID)
		}
		if _, duplicate := updates[item.ItemID]; duplicate {
			return nil, fmt.Errorf("duplicate invoice revision submitted for item %d", item.ItemID)
		}
		if item.Quantity < 1 {
			return nil, fmt.Errorf("quantity must be at least 1 for item %d", item.ItemID)
		}
		if item.EstimatedPrice < 0 {
			return nil, fmt.Errorf("estimated_price cannot be negative for item %d", item.ItemID)
		}

		updates[item.ItemID] = purchaseRequestInvoiceRevisionItemUpdate{
			Quantity:       item.Quantity,
			EstimatedPrice: item.EstimatedPrice,
			Notes:          strings.TrimSpace(item.Notes),
		}
	}

	if len(updates) != len(existingByID) {
		return nil, fmt.Errorf("invoice revision must include every purchase request item")
	}

	return updates, nil
}

func buildPurchaseRequestRequesterRevisionMap(items []models.PurchaseRequestRequesterRevisionItemPayload, existingItems []models.PurchaseRequestItem) (map[uint]purchaseRequestInvoiceRevisionItemUpdate, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("requester revision items are required")
	}

	existingByID := make(map[uint]struct{}, len(existingItems))
	for _, item := range existingItems {
		existingByID[item.ID] = struct{}{}
	}

	updates := make(map[uint]purchaseRequestInvoiceRevisionItemUpdate, len(items))
	for _, item := range items {
		if item.ItemID == 0 {
			return nil, fmt.Errorf("item_id is required")
		}
		if _, exists := existingByID[item.ItemID]; !exists {
			return nil, fmt.Errorf("purchase request item %d does not belong to this purchase request", item.ItemID)
		}
		if _, duplicate := updates[item.ItemID]; duplicate {
			return nil, fmt.Errorf("duplicate requester revision submitted for item %d", item.ItemID)
		}
		if item.Quantity < 1 {
			return nil, fmt.Errorf("quantity must be at least 1 for item %d", item.ItemID)
		}
		if item.EstimatedPrice < 0 {
			return nil, fmt.Errorf("estimated_price cannot be negative for item %d", item.ItemID)
		}

		updates[item.ItemID] = purchaseRequestInvoiceRevisionItemUpdate{
			Quantity:       item.Quantity,
			EstimatedPrice: item.EstimatedPrice,
			Notes:          strings.TrimSpace(item.Notes),
		}
	}

	if len(updates) != len(existingByID) {
		return nil, fmt.Errorf("requester revision must include every purchase request item")
	}

	return updates, nil
}

func joinPurchaseCodes(purchases []models.Purchase) string {
	codes := make([]string, 0, len(purchases))
	for _, purchase := range purchases {
		if strings.TrimSpace(purchase.Code) != "" {
			codes = append(codes, purchase.Code)
		}
	}
	if len(codes) == 0 {
		return "purchase transactions"
	}
	return strings.Join(codes, ", ")
}

func applyPurchaseRequestVendorInvoiceApprovalState(pr *models.PurchaseRequest) {
	if pr == nil {
		return
	}
	if !pr.RequiresVendorInvoice {
		if pr.VendorInvoiceStatus == "" {
			pr.VendorInvoiceStatus = models.PRVendorInvoiceStatusNotRequired
		}
		return
	}
	if pr.ReapprovalReason == models.PRReapprovalReasonInvoiceRevision {
		pr.VendorInvoiceStatus = models.PRVendorInvoiceStatusApproved
		return
	}
	if pr.VendorInvoiceStatus == "" || pr.VendorInvoiceStatus == models.PRVendorInvoiceStatusNotRequired {
		pr.VendorInvoiceStatus = models.PRVendorInvoiceStatusAwaitingInvoice
	}
}

func resolvePurchaseRequestReviewStage(pr *models.PurchaseRequest, reviewerRole string) (string, error) {
	if pr.Status != models.PRStatusPendingApproval && pr.Status != models.PRStatusDraft {
		return "", fmt.Errorf("purchase request %s is already %s", pr.PRNumber, pr.Status)
	}

	stage := pr.CurrentApprovalStage
	if stage == "" {
		stage = resolveInitialPurchaseRequestStage(pr.Requester.Role)
	}

	roleNorm := models.NormalizeRole(reviewerRole)
	switch stage {
	case models.PRApprovalStageFinanceStaff:
		if models.IsFinanceStaffRole(roleNorm) || models.IsAccountingRole(roleNorm) {
			return stage, nil
		}
		return "", fmt.Errorf("only finance staff or accounting can review purchase requests at the first stage")
	case models.PRApprovalStageFinanceManager:
		if models.IsFinanceManagerRole(roleNorm) {
			return stage, nil
		}
		return "", fmt.Errorf("only finance manager can review purchase requests at the finance manager stage")
	case models.PRApprovalStageDirector:
		if roleNorm == models.RoleDirector {
			return stage, nil
		}
		return "", fmt.Errorf("only director can review purchase requests at the director stage")
	default:
		return "", fmt.Errorf("purchase request %s is not in a reviewable stage", pr.PRNumber)
	}
}

func resolveInitialPurchaseRequestStage(requesterRole string) string {
	if models.IsFinanceManagerRole(requesterRole) {
		return models.PRApprovalStageFinanceManager
	}

	return models.PRApprovalStageFinanceStaff
}

func buildPurchaseRequestDecisionMap(items []models.PurchaseRequestReviewItemPayload) (map[uint]string, error) {
	decisionByItemID := make(map[uint]string, len(items))

	for _, item := range items {
		if item.ItemID == 0 {
			return nil, fmt.Errorf("item_id is required")
		}

		decision := normalizePurchaseRequestDecision(item.Decision)
		if decision == "" {
			return nil, fmt.Errorf("invalid decision for item %d", item.ItemID)
		}
		if _, exists := decisionByItemID[item.ItemID]; exists {
			return nil, fmt.Errorf("duplicate decision submitted for item %d", item.ItemID)
		}

		decisionByItemID[item.ItemID] = decision
	}

	return decisionByItemID, nil
}

func buildBulkPurchaseRequestReviewPayload(pr *models.PurchaseRequest, decision string) models.PurchaseRequestReviewPayload {
	payload := models.PurchaseRequestReviewPayload{
		Items: make([]models.PurchaseRequestReviewItemPayload, 0, len(pr.Items)),
	}

	for _, item := range pr.Items {
		payload.Items = append(payload.Items, models.PurchaseRequestReviewItemPayload{
			ItemID:   item.ID,
			Decision: decision,
		})
	}

	return payload
}

func normalizePurchaseRequestDecision(decision string) string {
	switch models.NormalizeRole(decision) {
	case "approved", "approve":
		return models.PRItemDecisionApproved
	case "rejected", "reject":
		return models.PRItemDecisionRejected
	default:
		return ""
	}
}

func calculateFinalPurchaseRequestTotal(items []models.PurchaseRequestItem) float64 {
	var total float64
	for _, item := range items {
		if item.ApprovalStatus == models.PRItemDecisionApproved {
			total += item.TotalEstimatedPrice
		}
	}
	return total
}

func (s *purchaseRequestService) notifyPurchaseRequestCreated(pr *models.PurchaseRequest) {
	if s.notificationService == nil {
		return
	}

	switch pr.CurrentApprovalStage {
	case models.PRApprovalStageFinanceManager:
		s.notifyFinanceManagerReview(pr)
		return
	case models.PRApprovalStageDirector:
		s.notifyDirectorReview(pr)
		return
	}

	message := fmt.Sprintf("Purchase request %s needs finance staff review (Amount: %s)",
		pr.PRNumber, fmt.Sprintf("Rp %.2f", pr.TotalEstimatedAmount))

	if err := s.notificationService.smartService.CreateSmartNotification(
		models.NotificationTypeApprovalPending,
		"Purchase Request Finance Staff Review",
		message,
		map[string]interface{}{
			"pr_id":                  pr.ID,
			"pr_number":              pr.PRNumber,
			"total_amount":           pr.TotalEstimatedAmount,
			"department":             pr.Department,
			"current_approval_stage": pr.CurrentApprovalStage,
			"action_type":            "finance_staff_review_required",
		},
		pr.TotalEstimatedAmount,
		models.RoleFinanceStaff,
		0,
		pr.Department,
	); err != nil {
		fmt.Printf("Failed to create PR notification: %v\n", err)
	}
}

func (s *purchaseRequestService) notifyFinanceManagerReview(pr *models.PurchaseRequest) {
	if s.notificationService == nil {
		return
	}

	title := "Purchase Request Final Approval Required"
	message := fmt.Sprintf("Purchase request %s is ready for finance manager review", pr.PRNumber)
	actionType := "finance_manager_review_required"
	if pr.ReapprovalReason == models.PRReapprovalReasonAdditionalDocuments {
		title = "Purchase Request Document Reapproval Required"
		message = fmt.Sprintf("Purchase request %s has additional supporting documents and has been reviewed again by Finance Staff", pr.PRNumber)
		actionType = "finance_manager_document_reapproval_required"
	}
	if pr.ReapprovalReason == models.PRReapprovalReasonRequesterRevision {
		title = "Purchase Request Revision Approval Required"
		message = fmt.Sprintf("Purchase request %s has been revised by requester and is ready for finance manager review", pr.PRNumber)
		actionType = "finance_manager_requester_revision_required"
	}

	if err := s.notificationService.smartService.CreateSmartNotification(
		models.NotificationTypeApprovalPending,
		title,
		message,
		map[string]interface{}{
			"pr_id":                  pr.ID,
			"pr_number":              pr.PRNumber,
			"total_amount":           pr.TotalEstimatedAmount,
			"department":             pr.Department,
			"current_approval_stage": pr.CurrentApprovalStage,
			"approval_revision":      pr.ApprovalRevision,
			"reapproval_reason":      pr.ReapprovalReason,
			"action_type":            actionType,
		},
		pr.TotalEstimatedAmount,
		models.RoleFinance,
		0,
		pr.Department,
	); err != nil {
		fmt.Printf("Failed to create finance manager PR notification: %v\n", err)
	}
}

func (s *purchaseRequestService) notifyAdditionalPurchaseRequestDocuments(pr *models.PurchaseRequest, documentCount int, reapprovalStarted bool) {
	if s.notificationService == nil {
		return
	}

	title := "Purchase Request Supporting Documents Added"
	message := fmt.Sprintf("%d supporting document(s) were added to purchase request %s", documentCount, pr.PRNumber)
	if reapprovalStarted {
		title = "Purchase Request Reapproval Required"
		message = fmt.Sprintf("%d new supporting document(s) were added to purchase request %s after approval. Finance Staff review is required again.", documentCount, pr.PRNumber)
	}

	if err := s.notificationService.smartService.CreateSmartNotification(
		models.NotificationTypeApprovalPending,
		title,
		message,
		map[string]interface{}{
			"pr_id":                  pr.ID,
			"pr_number":              pr.PRNumber,
			"document_count":         documentCount,
			"approval_revision":      pr.ApprovalRevision,
			"reapproval_started":     reapprovalStarted,
			"current_approval_stage": pr.CurrentApprovalStage,
			"action_type":            "additional_documents_review_required",
		},
		pr.TotalEstimatedAmount,
		models.RoleFinanceStaff,
		0,
		pr.Department,
	); err != nil {
		fmt.Printf("Failed to create additional PR document notification: %v\n", err)
	}
}

func (s *purchaseRequestService) notifyPurchaseRequestInvoiceRevision(pr *models.PurchaseRequest, documentCount int) {
	if s.notificationService == nil || pr == nil {
		return
	}

	if err := s.notificationService.smartService.CreateSmartNotification(
		models.NotificationTypeApprovalPending,
		"Purchase Request Invoice Revision Required",
		fmt.Sprintf("Invoice aktual untuk purchase request %s sudah diunggah dengan %d dokumen. Finance Staff perlu review ulang nominal PR.", pr.PRNumber, documentCount),
		map[string]interface{}{
			"pr_id":                  pr.ID,
			"pr_number":              pr.PRNumber,
			"document_count":         documentCount,
			"approval_revision":      pr.ApprovalRevision,
			"reapproval_started":     true,
			"reapproval_reason":      pr.ReapprovalReason,
			"current_approval_stage": pr.CurrentApprovalStage,
			"action_type":            "vendor_invoice_revision_review_required",
		},
		pr.TotalEstimatedAmount,
		models.RoleFinanceStaff,
		0,
		pr.Department,
	); err != nil {
		fmt.Printf("Failed to create PR invoice revision notification: %v\n", err)
	}
}

func (s *purchaseRequestService) notifyRequesterRevisionOpened(pr *models.PurchaseRequest) {
	if s.notificationService == nil || pr == nil {
		return
	}

	if err := s.notificationService.smartService.CreateSmartNotification(
		models.NotificationTypeApprovalPending,
		"Purchase Request Revision Opened",
		fmt.Sprintf("Purchase request %s sudah dibuka untuk revisi nominal/dokumen oleh PIC.", pr.PRNumber),
		map[string]interface{}{
			"pr_id":             pr.ID,
			"pr_number":         pr.PRNumber,
			"revision_reason":   pr.RevisionReason,
			"reapproval_reason": pr.ReapprovalReason,
			"action_type":       "requester_revision_opened",
		},
		pr.TotalEstimatedAmount,
		models.RoleEmployee,
		pr.RequesterID,
		"",
	); err != nil {
		fmt.Printf("Failed to create requester revision opened notification: %v\n", err)
	}
}

func (s *purchaseRequestService) notifyPurchaseRequestRequesterRevision(pr *models.PurchaseRequest, documentCount int) {
	if s.notificationService == nil || pr == nil {
		return
	}

	if err := s.notificationService.smartService.CreateSmartNotification(
		models.NotificationTypeApprovalPending,
		"Purchase Request Revision Review Required",
		fmt.Sprintf("Purchase request %s sudah direvisi oleh PIC dengan %d dokumen pendukung. Finance Staff perlu review ulang.", pr.PRNumber, documentCount),
		map[string]interface{}{
			"pr_id":                  pr.ID,
			"pr_number":              pr.PRNumber,
			"document_count":         documentCount,
			"approval_revision":      pr.ApprovalRevision,
			"reapproval_started":     true,
			"reapproval_reason":      pr.ReapprovalReason,
			"current_approval_stage": pr.CurrentApprovalStage,
			"action_type":            "requester_revision_review_required",
		},
		pr.TotalEstimatedAmount,
		models.RoleFinanceStaff,
		0,
		pr.Department,
	); err != nil {
		fmt.Printf("Failed to create PR requester revision notification: %v\n", err)
	}
}

func (s *purchaseRequestService) notifyDirectorReview(pr *models.PurchaseRequest) {
	if s.notificationService == nil {
		return
	}

	if err := s.notificationService.smartService.CreateSmartNotification(
		models.NotificationTypeApprovalPending,
		"Purchase Request Director Approval Required",
		fmt.Sprintf("Purchase request %s is ready for director review", pr.PRNumber),
		map[string]interface{}{
			"pr_id":                  pr.ID,
			"pr_number":              pr.PRNumber,
			"total_amount":           pr.TotalEstimatedAmount,
			"department":             pr.Department,
			"current_approval_stage": pr.CurrentApprovalStage,
			"action_type":            "director_review_required",
		},
		pr.TotalEstimatedAmount,
		models.RoleDirector,
		0,
		pr.Department,
	); err != nil {
		fmt.Printf("Failed to create director PR notification: %v\n", err)
	}
}

func (s *purchaseRequestService) notifyRequesterAfterFinalReview(pr *models.PurchaseRequest, approvedCount int, rejectedCount int) {
	if s.notificationService == nil {
		return
	}

	title := "Purchase Request Approved"
	message := fmt.Sprintf("Your purchase request %s has been approved", pr.PRNumber)
	notificationType := models.NotificationTypeApprovalApproved

	if approvedCount == 0 {
		title = "Purchase Request Rejected"
		message = fmt.Sprintf("Your purchase request %s has been rejected", pr.PRNumber)
		notificationType = models.NotificationTypeApprovalRejected
	} else if rejectedCount > 0 {
		message = fmt.Sprintf("Your purchase request %s has been finalized: %d item approved, %d item rejected", pr.PRNumber, approvedCount, rejectedCount)
	}

	if pr.RejectionReason != "" {
		message = fmt.Sprintf("%s. Reason: %s", message, pr.RejectionReason)
	}

	if err := s.notificationService.smartService.CreateSmartNotification(
		notificationType,
		title,
		message,
		map[string]interface{}{
			"pr_id":          pr.ID,
			"pr_number":      pr.PRNumber,
			"approved_count": approvedCount,
			"rejected_count": rejectedCount,
			"approved_at":    pr.ApprovedAt,
			"action_type":    "final_review_completed",
		},
		pr.TotalEstimatedAmount,
		models.RoleEmployee,
		pr.RequesterID,
		"",
	); err != nil {
		fmt.Printf("Failed to create requester PR notification: %v\n", err)
	}
}
