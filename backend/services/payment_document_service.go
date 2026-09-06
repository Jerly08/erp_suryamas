package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	maxPaymentDocumentReferenceNumberLength = 100
	maxPaymentDocumentNotesLength           = 4000
	maxPaymentDocumentInvalidationLength    = 2000
)

// PaymentDocumentUploadInput contains file metadata after the controller has
// validated and safely stored an uploaded file. The service deliberately does
// not receive a multipart file, so no file content can influence payment or
// journal posting logic.
type PaymentDocumentUploadInput struct {
	DocumentType    string
	ReferenceNumber string
	Notes           string
	FileName        string
	FilePath        string
	FileSize        int64
	MimeType        string
}

// PaymentDocumentService owns the document-only workflow for payments. It
// never updates payment amount, allocation, cash/bank balance, or journals.
type PaymentDocumentService struct {
	db *gorm.DB
}

func NewPaymentDocumentService(db *gorm.DB) *PaymentDocumentService {
	return &PaymentDocumentService{db: db}
}

// EnsureCanUploadDocuments is a lightweight preflight used before a multipart
// body is read or files are written. UploadDocuments repeats the same check in
// its transaction to keep the operation race-safe.
func (s *PaymentDocumentService) EnsureCanUploadDocuments(paymentID uint, actorID uint, actorRole string) error {
	_, err := ensurePaymentDocumentUploadAllowed(s.db, paymentID, actorID, actorRole, false)
	return err
}

// UploadDocuments stores one or more already-saved files as the immutable
// attachment history of a payment. The payment creator and Finance team may
// add evidence during review; Director remains view/download only.
func (s *PaymentDocumentService) UploadDocuments(paymentID uint, actorID uint, actorRole string, inputs []PaymentDocumentUploadInput) ([]models.PaymentDocument, error) {
	if paymentID == 0 {
		return nil, errors.New("payment ID is required")
	}
	if actorID == 0 {
		return nil, errors.New("user authentication is required")
	}
	if len(inputs) == 0 {
		return nil, errors.New("at least one supporting document is required")
	}

	validatedInputs := make([]PaymentDocumentUploadInput, 0, len(inputs))
	for _, input := range inputs {
		normalized, err := validatePaymentDocumentUploadInput(input)
		if err != nil {
			return nil, err
		}
		validatedInputs = append(validatedInputs, normalized)
	}

	var created []models.PaymentDocument
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if _, err := ensurePaymentDocumentUploadAllowed(tx, paymentID, actorID, actorRole, true); err != nil {
			return err
		}

		now := time.Now()
		created = make([]models.PaymentDocument, 0, len(validatedInputs))
		for _, input := range validatedInputs {
			document := models.PaymentDocument{
				PaymentID:       paymentID,
				DocumentType:    input.DocumentType,
				ReferenceNumber: input.ReferenceNumber,
				FileName:        input.FileName,
				FilePath:        input.FilePath,
				FileSize:        input.FileSize,
				MimeType:        input.MimeType,
				Notes:           input.Notes,
				Status:          models.PaymentDocumentStatusActive,
				UploadedBy:      actorID,
			}
			if err := tx.Create(&document).Error; err != nil {
				return fmt.Errorf("save payment document: %w", err)
			}

			event := models.PaymentDocumentEvent{
				PaymentDocumentID: document.ID,
				PaymentID:         paymentID,
				Action:            models.PaymentDocumentEventUploaded,
				Notes:             input.Notes,
				ActorID:           actorID,
				OccurredAt:        now,
			}
			if err := tx.Create(&event).Error; err != nil {
				return fmt.Errorf("save payment document history: %w", err)
			}
			created = append(created, document)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.loadDocumentsByIDs(created)
}

// ListDocuments returns the attachment history when the caller is the payment
// creator or holds a finance/accounting role that can view all transactions.
func (s *PaymentDocumentService) ListDocuments(paymentID uint, actorID uint, actorRole string) ([]models.PaymentDocument, error) {
	if paymentID == 0 {
		return nil, errors.New("payment ID is required")
	}
	if actorID == 0 {
		return nil, errors.New("user authentication is required")
	}
	if _, err := getPaymentForDocumentAccess(s.db, paymentID, actorID, actorRole); err != nil {
		return nil, err
	}

	return s.loadDocuments(paymentID)
}

// GetDocumentForDownload applies the same payment visibility policy before a
// controller resolves the file path under the upload root.
func (s *PaymentDocumentService) GetDocumentForDownload(documentID uint, actorID uint, actorRole string) (*models.PaymentDocument, error) {
	if documentID == 0 {
		return nil, errors.New("payment document ID is required")
	}
	if actorID == 0 {
		return nil, errors.New("user authentication is required")
	}

	var document models.PaymentDocument
	if err := s.db.
		Preload("Uploader").
		Preload("Invalidator").
		Preload("Events", func(db *gorm.DB) *gorm.DB { return db.Order("occurred_at ASC, id ASC") }).
		Preload("Events.Actor").
		First(&document, documentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("payment document not found")
		}
		return nil, fmt.Errorf("load payment document: %w", err)
	}

	if _, err := getPaymentForDocumentAccess(s.db, document.PaymentID, actorID, actorRole); err != nil {
		return nil, err
	}
	return &document, nil
}

// InvalidateDocument preserves a document and its physical file while making
// its correction explicit. This is intentionally limited to Accounting,
// Finance Manager, or Admin and does not modify the payment itself.
func (s *PaymentDocumentService) InvalidateDocument(documentID uint, actorID uint, actorRole string, reason string) (*models.PaymentDocument, error) {
	if documentID == 0 {
		return nil, errors.New("payment document ID is required")
	}
	if actorID == 0 {
		return nil, errors.New("user authentication is required")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, errors.New("invalidation reason is required")
	}
	if len(reason) > maxPaymentDocumentInvalidationLength {
		return nil, fmt.Errorf("invalidation reason cannot exceed %d characters", maxPaymentDocumentInvalidationLength)
	}

	role := models.NormalizeRole(actorRole)
	if role != models.RoleAdmin && !models.IsAccountingRole(role) && !models.IsFinanceManagerRole(role) {
		return nil, errors.New("only Accounting, Finance Manager, or Admin can invalidate a payment document")
	}

	var updatedID uint
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var document models.PaymentDocument
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&document, documentID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("payment document not found")
			}
			return fmt.Errorf("load payment document: %w", err)
		}
		if _, err := getPaymentForDocumentAccess(tx, document.PaymentID, actorID, actorRole); err != nil {
			return err
		}
		if document.Status == models.PaymentDocumentStatusInvalidated {
			return errors.New("payment document is already invalidated")
		}

		now := time.Now()
		if err := tx.Model(&document).Updates(map[string]interface{}{
			"status":              models.PaymentDocumentStatusInvalidated,
			"invalidated_by":      actorID,
			"invalidated_at":      now,
			"invalidation_reason": reason,
			"updated_at":          now,
		}).Error; err != nil {
			return fmt.Errorf("invalidate payment document: %w", err)
		}

		if err := tx.Create(&models.PaymentDocumentEvent{
			PaymentDocumentID: document.ID,
			PaymentID:         document.PaymentID,
			Action:            models.PaymentDocumentEventInvalidated,
			Notes:             reason,
			ActorID:           actorID,
			OccurredAt:        now,
		}).Error; err != nil {
			return fmt.Errorf("save payment document invalidation history: %w", err)
		}
		updatedID = document.ID
		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.getDocumentByID(updatedID)
}

func validatePaymentDocumentUploadInput(input PaymentDocumentUploadInput) (PaymentDocumentUploadInput, error) {
	documentType, valid := models.NormalizePaymentDocumentType(input.DocumentType)
	if !valid {
		return input, errors.New("invalid payment document type")
	}

	input.DocumentType = documentType
	input.ReferenceNumber = strings.TrimSpace(input.ReferenceNumber)
	input.Notes = strings.TrimSpace(input.Notes)
	input.FileName = strings.TrimSpace(input.FileName)
	input.FilePath = strings.TrimSpace(input.FilePath)
	input.MimeType = strings.TrimSpace(input.MimeType)

	if len(input.ReferenceNumber) > maxPaymentDocumentReferenceNumberLength {
		return input, fmt.Errorf("reference number cannot exceed %d characters", maxPaymentDocumentReferenceNumberLength)
	}
	if len(input.Notes) > maxPaymentDocumentNotesLength {
		return input, fmt.Errorf("notes cannot exceed %d characters", maxPaymentDocumentNotesLength)
	}
	if input.FileName == "" || input.FilePath == "" {
		return input, errors.New("payment document file is required")
	}
	if input.FileSize <= 0 {
		return input, errors.New("payment document file is empty")
	}

	return input, nil
}

func paymentAllowsNewDocuments(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case models.PaymentStatusRejected, models.PaymentStatusReversed, models.PaymentStatusFailed, "CANCELLED":
		return false
	default:
		return true
	}
}

func ensurePaymentDocumentUploadAllowed(db *gorm.DB, paymentID uint, actorID uint, actorRole string, lockPayment bool) (*models.Payment, error) {
	payment, err := getPaymentForDocumentAccessWithLock(db, paymentID, actorID, actorRole, lockPayment)
	if err != nil {
		return nil, err
	}
	role := models.NormalizeRole(actorRole)
	if role == models.RoleDirector {
		return nil, errors.New("Director can only view or download payment documents")
	}
	if payment.UserID != actorID && role != models.RoleAdmin && !models.IsFinanceTeamRole(role) {
		return nil, errors.New("only the payment creator or Finance team can add supporting documents")
	}
	if !paymentAllowsNewDocuments(payment.Status) {
		return nil, fmt.Errorf("supporting documents cannot be added to a %s payment", strings.ToLower(strings.TrimSpace(payment.Status)))
	}
	return payment, nil
}

func getPaymentForDocumentAccess(db *gorm.DB, paymentID uint, actorID uint, actorRole string) (*models.Payment, error) {
	return getPaymentForDocumentAccessWithLock(db, paymentID, actorID, actorRole, false)
}

func getPaymentForDocumentAccessWithLock(db *gorm.DB, paymentID uint, actorID uint, actorRole string, lockPayment bool) (*models.Payment, error) {
	var payment models.Payment
	query := db
	if lockPayment {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&payment, paymentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("payment not found")
		}
		return nil, fmt.Errorf("load payment: %w", err)
	}

	if payment.UserID == actorID || models.CanViewAllTransactionData(actorRole) {
		return &payment, nil
	}
	return nil, errors.New("access denied: you can only access your own payment documents")
}

func (s *PaymentDocumentService) loadDocuments(paymentID uint) ([]models.PaymentDocument, error) {
	var documents []models.PaymentDocument
	if err := s.db.
		Where("payment_id = ?", paymentID).
		Preload("Uploader").
		Preload("Invalidator").
		Preload("Events", func(db *gorm.DB) *gorm.DB { return db.Order("occurred_at ASC, id ASC") }).
		Preload("Events.Actor").
		Order("created_at ASC, id ASC").
		Find(&documents).Error; err != nil {
		return nil, fmt.Errorf("load payment documents: %w", err)
	}
	return documents, nil
}

func (s *PaymentDocumentService) loadDocumentsByIDs(documents []models.PaymentDocument) ([]models.PaymentDocument, error) {
	if len(documents) == 0 {
		return []models.PaymentDocument{}, nil
	}

	ids := make([]uint, 0, len(documents))
	for _, document := range documents {
		ids = append(ids, document.ID)
	}

	var loaded []models.PaymentDocument
	if err := s.db.
		Where("id IN ?", ids).
		Preload("Uploader").
		Preload("Invalidator").
		Preload("Events", func(db *gorm.DB) *gorm.DB { return db.Order("occurred_at ASC, id ASC") }).
		Preload("Events.Actor").
		Order("created_at ASC, id ASC").
		Find(&loaded).Error; err != nil {
		return nil, fmt.Errorf("load uploaded payment documents: %w", err)
	}
	return loaded, nil
}

func (s *PaymentDocumentService) getDocumentByID(documentID uint) (*models.PaymentDocument, error) {
	var document models.PaymentDocument
	if err := s.db.
		Preload("Uploader").
		Preload("Invalidator").
		Preload("Events", func(db *gorm.DB) *gorm.DB { return db.Order("occurred_at ASC, id ASC") }).
		Preload("Events.Actor").
		First(&document, documentID).Error; err != nil {
		return nil, fmt.Errorf("load payment document: %w", err)
	}
	return &document, nil
}
