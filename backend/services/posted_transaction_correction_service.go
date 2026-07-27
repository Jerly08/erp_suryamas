package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrPostedTransactionCorrectionNotFound = errors.New("koreksi transaksi tidak ditemukan")
	ErrPostedTransactionCorrectionDenied   = errors.New("akses koreksi transaksi tidak diizinkan")
)

type PostedTransactionCorrectionService struct {
	db *gorm.DB
}

func NewPostedTransactionCorrectionService(db *gorm.DB) *PostedTransactionCorrectionService {
	return &PostedTransactionCorrectionService{db: db}
}

func (s *PostedTransactionCorrectionService) Create(
	request models.PostedTransactionCorrectionCreateRequest,
	userID uint,
	role string,
) (*models.PostedTransactionCorrection, error) {
	if !canRequestPostedTransactionCorrection(role) {
		return nil, ErrPostedTransactionCorrectionDenied
	}
	if err := validatePostedTransactionCorrectionType(request.CorrectionType); err != nil {
		return nil, err
	}
	reason := strings.TrimSpace(request.Reason)
	if reason == "" {
		return nil, fmt.Errorf("alasan koreksi wajib diisi")
	}
	if (request.PurchaseID == nil && request.PaymentID == nil) || (request.PurchaseID != nil && request.PaymentID != nil) {
		return nil, fmt.Errorf("pilih tepat satu transaksi pembelian atau pembayaran")
	}

	requestedDate, err := parseRequestedCorrectionDate(request.RequestedDate)
	if err != nil {
		return nil, err
	}
	if request.RequestedAmount != nil && *request.RequestedAmount < 0 {
		return nil, fmt.Errorf("nominal koreksi tidak boleh negatif")
	}

	var correction models.PostedTransactionCorrection
	err = s.db.Transaction(func(tx *gorm.DB) error {
		target, err := s.resolvePostedCorrectionTarget(tx, request.PurchaseID, request.PaymentID)
		if err != nil {
			return err
		}

		var activeCount int64
		duplicateQuery := tx.Model(&models.PostedTransactionCorrection{}).
			Where("correction_type = ? AND status IN ?", strings.ToUpper(strings.TrimSpace(request.CorrectionType)), activeCorrectionStatuses())
		if target.purchaseID != nil {
			duplicateQuery = duplicateQuery.Where("purchase_id = ?", *target.purchaseID)
		} else {
			duplicateQuery = duplicateQuery.Where("payment_id = ?", *target.paymentID)
		}
		if err := duplicateQuery.Count(&activeCount).Error; err != nil {
			return err
		}
		if activeCount > 0 {
			return fmt.Errorf("masih ada pengajuan koreksi aktif untuk jenis koreksi ini")
		}

		now := time.Now().UTC()
		correction = models.PostedTransactionCorrection{
			TargetType:                    target.targetType,
			TargetCode:                    target.targetCode,
			PurchaseID:                    target.purchaseID,
			PaymentID:                     target.paymentID,
			PurchasePaymentID:             target.purchasePaymentID,
			OriginalJournalEntryID:        target.originalJournalID,
			SourceSnapshot:                target.sourceSnapshot,
			CorrectionType:                strings.ToUpper(strings.TrimSpace(request.CorrectionType)),
			Status:                        models.PostedTransactionCorrectionStatusPendingAccounting,
			Reason:                        reason,
			RequestedDate:                 requestedDate,
			RequestedAmount:               request.RequestedAmount,
			RequestedCashBankReference:    strings.TrimSpace(request.RequestedCashBankReference),
			RequestedDestinationReference: strings.TrimSpace(request.RequestedDestinationReference),
			RequestedReference:            strings.TrimSpace(request.RequestedReference),
			RequestedBy:                   userID,
			RequestedAt:                   now,
		}
		if err := tx.Create(&correction).Error; err != nil {
			return err
		}
		return s.createEvent(tx, correction.ID, models.PostedTransactionCorrectionEventSubmitted, reason, userID, role)
	})
	if err != nil {
		return nil, err
	}

	return s.GetByID(correction.ID, userID, role)
}

func (s *PostedTransactionCorrectionService) List(
	filter models.PostedTransactionCorrectionFilter,
	userID uint,
	role string,
) ([]models.PostedTransactionCorrection, int64, error) {
	if !canViewAllPostedTransactionCorrections(role) && userID == 0 {
		return nil, 0, ErrPostedTransactionCorrectionDenied
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	query := s.withCorrectionRelations(s.db.Model(&models.PostedTransactionCorrection{}))
	if filter.PurchaseID != nil {
		query = query.Where("purchase_id = ?", *filter.PurchaseID)
	}
	if filter.PaymentID != nil {
		query = query.Where("payment_id = ?", *filter.PaymentID)
	}
	if status := strings.ToUpper(strings.TrimSpace(filter.Status)); status != "" {
		query = query.Where("status = ?", status)
	}
	if !canViewAllPostedTransactionCorrections(role) {
		query = query.Where("requested_by = ?", userID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var corrections []models.PostedTransactionCorrection
	if err := query.
		Order("created_at DESC, id DESC").
		Offset((filter.Page - 1) * filter.Limit).
		Limit(filter.Limit).
		Find(&corrections).Error; err != nil {
		return nil, 0, err
	}
	return corrections, total, nil
}

func (s *PostedTransactionCorrectionService) GetByID(
	id uint,
	userID uint,
	role string,
) (*models.PostedTransactionCorrection, error) {
	var correction models.PostedTransactionCorrection
	if err := s.withCorrectionRelations(s.db).First(&correction, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPostedTransactionCorrectionNotFound
		}
		return nil, err
	}
	if !canViewAllPostedTransactionCorrections(role) && correction.RequestedBy != userID {
		return nil, ErrPostedTransactionCorrectionDenied
	}
	return &correction, nil
}

func (s *PostedTransactionCorrectionService) ReviewAccounting(
	id uint,
	request models.PostedTransactionCorrectionAccountingReviewRequest,
	userID uint,
	role string,
) (*models.PostedTransactionCorrection, error) {
	if !canReviewPostedTransactionCorrectionAccounting(role) {
		return nil, ErrPostedTransactionCorrectionDenied
	}
	resolution := strings.ToUpper(strings.TrimSpace(request.Resolution))
	if err := validatePostedTransactionCorrectionResolution(resolution); err != nil {
		return nil, err
	}
	notes := strings.TrimSpace(request.Notes)
	if notes == "" {
		return nil, fmt.Errorf("catatan Accounting wajib diisi")
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var correction models.PostedTransactionCorrection
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&correction, id).Error; err != nil {
			return normalizePostedTransactionCorrectionNotFound(err)
		}
		if correction.Status != models.PostedTransactionCorrectionStatusPendingAccounting {
			return fmt.Errorf("koreksi ini tidak sedang menunggu review Accounting")
		}
		if err := s.validateProposedAccount(tx, request.ProposedAccountID); err != nil {
			return err
		}

		now := time.Now().UTC()
		updates := map[string]interface{}{
			"status":                 models.PostedTransactionCorrectionStatusPendingFinanceManager,
			"accounting_resolution":  resolution,
			"accounting_notes":       notes,
			"proposed_account_id":    request.ProposedAccountID,
			"accounting_reviewed_by": userID,
			"accounting_reviewed_at": now,
		}
		if err := tx.Model(&correction).Updates(updates).Error; err != nil {
			return err
		}
		return s.createEvent(tx, correction.ID, models.PostedTransactionCorrectionEventAccountingReviewed, notes, userID, role)
	})
	if err != nil {
		return nil, err
	}
	return s.GetByID(id, userID, role)
}

func (s *PostedTransactionCorrectionService) DecideFinanceManager(
	id uint,
	request models.PostedTransactionCorrectionFinanceManagerDecisionRequest,
	userID uint,
	role string,
) (*models.PostedTransactionCorrection, error) {
	if !canReviewPostedTransactionCorrectionFinanceManager(role) {
		return nil, ErrPostedTransactionCorrectionDenied
	}
	notes := strings.TrimSpace(request.Notes)
	if !request.Approve && notes == "" {
		return nil, fmt.Errorf("alasan penolakan wajib diisi")
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var correction models.PostedTransactionCorrection
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&correction, id).Error; err != nil {
			return normalizePostedTransactionCorrectionNotFound(err)
		}
		if correction.Status != models.PostedTransactionCorrectionStatusPendingFinanceManager {
			return fmt.Errorf("koreksi ini tidak sedang menunggu keputusan Finance Manager")
		}

		now := time.Now().UTC()
		status := models.PostedTransactionCorrectionStatusRejected
		eventType := models.PostedTransactionCorrectionEventFinanceRejected
		if request.Approve {
			status = models.PostedTransactionCorrectionStatusApprovedForExecution
			eventType = models.PostedTransactionCorrectionEventFinanceApproved
		}
		if err := tx.Model(&correction).Updates(map[string]interface{}{
			"status":                         status,
			"finance_manager_decision_notes": notes,
			"finance_manager_approved_by":    userID,
			"finance_manager_approved_at":    now,
		}).Error; err != nil {
			return err
		}
		return s.createEvent(tx, correction.ID, eventType, notes, userID, role)
	})
	if err != nil {
		return nil, err
	}
	return s.GetByID(id, userID, role)
}

// Complete records the outcome after the approved correction is executed.
// Financial entries are intentionally not generated here because their proper
// treatment depends on the approved accounting resolution and bank evidence.
func (s *PostedTransactionCorrectionService) Complete(
	id uint,
	request models.PostedTransactionCorrectionCompleteRequest,
	userID uint,
	role string,
) (*models.PostedTransactionCorrection, error) {
	if !canCompletePostedTransactionCorrection(role) {
		return nil, ErrPostedTransactionCorrectionDenied
	}
	notes := strings.TrimSpace(request.Notes)
	if notes == "" {
		return nil, fmt.Errorf("catatan penyelesaian wajib diisi")
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var correction models.PostedTransactionCorrection
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&correction, id).Error; err != nil {
			return normalizePostedTransactionCorrectionNotFound(err)
		}
		if correction.Status != models.PostedTransactionCorrectionStatusApprovedForExecution {
			return fmt.Errorf("koreksi ini belum disetujui untuk dieksekusi")
		}
		if correctionRequiresFinancialLink(correction.AccountingResolution) && request.CompletionJournalEntryID == nil && request.ReplacementPaymentID == nil {
			return fmt.Errorf("jurnal koreksi atau pembayaran pengganti wajib ditautkan untuk rencana penyelesaian ini")
		}
		if err := s.validateCompletionLinks(tx, correction.OriginalJournalEntryID, request.CompletionJournalEntryID, request.ReplacementPaymentID); err != nil {
			return err
		}

		now := time.Now().UTC()
		if err := tx.Model(&correction).Updates(map[string]interface{}{
			"status":                      models.PostedTransactionCorrectionStatusCompleted,
			"completion_notes":            notes,
			"completion_journal_entry_id": request.CompletionJournalEntryID,
			"replacement_payment_id":      request.ReplacementPaymentID,
			"completed_by":                userID,
			"completed_at":                now,
		}).Error; err != nil {
			return err
		}
		return s.createEvent(tx, correction.ID, models.PostedTransactionCorrectionEventCompleted, notes, userID, role)
	})
	if err != nil {
		return nil, err
	}
	return s.GetByID(id, userID, role)
}

func (s *PostedTransactionCorrectionService) AddDocument(
	correctionID uint,
	fileName string,
	filePath string,
	fileSize int64,
	mimeType string,
	userID uint,
	role string,
) (*models.PostedTransactionCorrectionDocument, error) {
	correction, err := s.GetByID(correctionID, userID, role)
	if err != nil {
		return nil, err
	}
	if correction.Status == models.PostedTransactionCorrectionStatusCancelled {
		return nil, fmt.Errorf("dokumen tidak dapat ditambahkan ke koreksi yang dibatalkan")
	}
	if !canAddPostedTransactionCorrectionDocument(correction, userID, role) {
		return nil, ErrPostedTransactionCorrectionDenied
	}

	document := models.PostedTransactionCorrectionDocument{
		CorrectionID: correctionID,
		FileName:     strings.TrimSpace(fileName),
		FilePath:     strings.TrimSpace(filePath),
		FileSize:     fileSize,
		MimeType:     strings.TrimSpace(mimeType),
		UploadedBy:   userID,
		UploadedAt:   time.Now().UTC(),
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&document).Error; err != nil {
			return err
		}
		return s.createEvent(tx, correctionID, models.PostedTransactionCorrectionEventDocumentAdded, document.FileName, userID, role)
	}); err != nil {
		return nil, err
	}
	return &document, nil
}

func (s *PostedTransactionCorrectionService) GetDocument(
	documentID uint,
	userID uint,
	role string,
) (*models.PostedTransactionCorrectionDocument, error) {
	var document models.PostedTransactionCorrectionDocument
	if err := s.db.First(&document, documentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPostedTransactionCorrectionNotFound
		}
		return nil, err
	}
	if _, err := s.GetByID(document.CorrectionID, userID, role); err != nil {
		return nil, err
	}
	return &document, nil
}

type postedCorrectionTarget struct {
	targetType        string
	targetCode        string
	purchaseID        *uint
	paymentID         *uint
	purchasePaymentID *uint
	originalJournalID uint64
	sourceSnapshot    string
}

func (s *PostedTransactionCorrectionService) resolvePostedCorrectionTarget(
	tx *gorm.DB,
	purchaseID *uint,
	paymentID *uint,
) (*postedCorrectionTarget, error) {
	if purchaseID != nil {
		var purchase models.Purchase
		if err := tx.Preload("Vendor").Preload("BankAccount").Preload("PurchasePayments.Payment").First(&purchase, *purchaseID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("pembelian tidak ditemukan")
			}
			return nil, err
		}
		journal, purchasePaymentID, linkedPaymentID := findPostedJournalForPurchase(tx, &purchase)
		if journal == nil {
			return nil, fmt.Errorf("pembelian ini belum memiliki jurnal terposting sehingga belum memerlukan koreksi transaksi terposting")
		}
		snapshot, err := marshalCorrectionSnapshot(map[string]interface{}{
			"purchase_code":     purchase.Code,
			"purchase_date":     purchase.Date,
			"vendor":            purchase.Vendor.Name,
			"total_amount":      purchase.TotalAmount,
			"payment_method":    purchase.PaymentMethod,
			"payment_reference": purchase.PaymentReference,
			"bank_account":      purchase.BankAccount,
			"journal_entry_id":  journal.ID,
		})
		if err != nil {
			return nil, err
		}
		return &postedCorrectionTarget{
			targetType:        models.PostedTransactionCorrectionTargetPurchase,
			targetCode:        purchase.Code,
			purchaseID:        &purchase.ID,
			paymentID:         linkedPaymentID,
			purchasePaymentID: purchasePaymentID,
			originalJournalID: journal.ID,
			sourceSnapshot:    snapshot,
		}, nil
	}

	var payment models.Payment
	if err := tx.Preload("Contact").Preload("CashBank").First(&payment, *paymentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("pembayaran tidak ditemukan")
		}
		return nil, err
	}
	journal := findPostedJournalForPayment(tx, &payment)
	if journal == nil {
		return nil, fmt.Errorf("pembayaran ini belum memiliki jurnal terposting sehingga belum memerlukan koreksi transaksi terposting")
	}
	snapshot, err := marshalCorrectionSnapshot(map[string]interface{}{
		"payment_code":     payment.Code,
		"payment_date":     payment.Date,
		"contact":          payment.Contact.Name,
		"amount":           payment.Amount,
		"method":           payment.Method,
		"reference":        payment.Reference,
		"cash_bank":        payment.CashBank,
		"journal_entry_id": journal.ID,
	})
	if err != nil {
		return nil, err
	}
	return &postedCorrectionTarget{
		targetType:        models.PostedTransactionCorrectionTargetPayment,
		targetCode:        payment.Code,
		paymentID:         &payment.ID,
		originalJournalID: journal.ID,
		sourceSnapshot:    snapshot,
	}, nil
}

func findPostedJournalForPurchase(tx *gorm.DB, purchase *models.Purchase) (*models.SSOTJournalEntry, *uint, *uint) {
	sourceID := uint64(purchase.ID)
	var journal models.SSOTJournalEntry
	if err := tx.Where("source_type = ? AND source_id = ? AND status = ? AND deleted_at IS NULL", models.SSOTSourceTypePurchase, sourceID, models.SSOTStatusPosted).
		Order("id DESC").First(&journal).Error; err == nil {
		return &journal, nil, nil
	}

	for _, purchasePayment := range purchase.PurchasePayments {
		if purchasePayment.Payment != nil {
			if paymentJournal := findPostedJournalForPayment(tx, purchasePayment.Payment); paymentJournal != nil {
				purchasePaymentID := purchasePayment.ID
				paymentID := purchasePayment.Payment.ID
				return paymentJournal, &purchasePaymentID, &paymentID
			}
		}
	}
	return nil, nil, nil
}

func findPostedJournalForPayment(tx *gorm.DB, payment *models.Payment) *models.SSOTJournalEntry {
	var journal models.SSOTJournalEntry
	if payment.JournalEntryID != nil {
		if err := tx.Where("id = ? AND status = ? AND deleted_at IS NULL", *payment.JournalEntryID, models.SSOTStatusPosted).First(&journal).Error; err == nil {
			return &journal
		}
	}

	sourceID := uint64(payment.ID)
	if err := tx.Where("source_type IN ? AND source_id = ? AND status = ? AND deleted_at IS NULL", []string{models.SSOTSourceTypePayment, "PURCHASE_PAYMENT"}, sourceID, models.SSOTStatusPosted).
		Order("id DESC").First(&journal).Error; err == nil {
		return &journal
	}
	return nil
}

func (s *PostedTransactionCorrectionService) validateProposedAccount(tx *gorm.DB, accountID *uint) error {
	if accountID == nil {
		return nil
	}
	var account models.Account
	if err := tx.First(&account, *accountID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("COA yang dipilih tidak ditemukan")
		}
		return err
	}
	if !account.IsActive || account.IsHeader {
		return fmt.Errorf("COA yang dipilih harus akun aktif dan bukan akun header")
	}
	return nil
}

func (s *PostedTransactionCorrectionService) validateCompletionLinks(tx *gorm.DB, originalJournalID uint64, journalID *uint64, paymentID *uint) error {
	if journalID != nil {
		if *journalID == originalJournalID {
			return fmt.Errorf("jurnal asal tidak dapat digunakan sebagai jurnal koreksi")
		}
		var journal models.SSOTJournalEntry
		if err := tx.Where("id = ? AND status = ? AND deleted_at IS NULL", *journalID, models.SSOTStatusPosted).First(&journal).Error; err != nil {
			return fmt.Errorf("jurnal penyelesaian harus merupakan jurnal terposting yang valid")
		}
	}
	if paymentID != nil {
		var payment models.Payment
		if err := tx.First(&payment, *paymentID).Error; err != nil {
			return fmt.Errorf("pembayaran pengganti tidak ditemukan")
		}
	}
	return nil
}

func correctionRequiresFinancialLink(resolution string) bool {
	switch strings.ToUpper(strings.TrimSpace(resolution)) {
	case models.PostedTransactionCorrectionResolutionJournalReclass,
		models.PostedTransactionCorrectionResolutionReverseRecreate,
		models.PostedTransactionCorrectionResolutionManualAdjustment:
		return true
	default:
		return false
	}
}

func (s *PostedTransactionCorrectionService) createEvent(tx *gorm.DB, correctionID uint, eventType string, notes string, actorID uint, role string) error {
	return tx.Create(&models.PostedTransactionCorrectionEvent{
		CorrectionID: correctionID,
		EventType:    eventType,
		Notes:        strings.TrimSpace(notes),
		ActorID:      actorID,
		ActorRole:    models.NormalizeRole(role),
		CreatedAt:    time.Now().UTC(),
	}).Error
}

func (s *PostedTransactionCorrectionService) withCorrectionRelations(db *gorm.DB) *gorm.DB {
	return db.
		Preload("Purchase").
		Preload("Purchase.Vendor").
		Preload("Payment").
		Preload("Payment.Contact").
		Preload("PurchasePayment").
		Preload("Requester").
		Preload("AccountingUser").
		Preload("FinanceManager").
		Preload("Completer").
		Preload("ProposedAccount").
		Preload("Documents", func(db *gorm.DB) *gorm.DB { return db.Order("uploaded_at ASC, id ASC") }).
		Preload("Documents.Uploader").
		Preload("Events", func(db *gorm.DB) *gorm.DB { return db.Order("created_at ASC, id ASC") }).
		Preload("Events.Actor")
}

func activeCorrectionStatuses() []string {
	return []string{
		models.PostedTransactionCorrectionStatusPendingAccounting,
		models.PostedTransactionCorrectionStatusPendingFinanceManager,
		models.PostedTransactionCorrectionStatusApprovedForExecution,
	}
}

func parseRequestedCorrectionDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, fmt.Errorf("tanggal koreksi harus menggunakan format YYYY-MM-DD")
	}
	return &parsed, nil
}

func marshalCorrectionSnapshot(value interface{}) (string, error) {
	bytes, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("gagal menyimpan snapshot transaksi: %w", err)
	}
	return string(bytes), nil
}

func validatePostedTransactionCorrectionType(value string) error {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case models.PostedTransactionCorrectionTypeMissingPaymentProof,
		models.PostedTransactionCorrectionTypePaymentDate,
		models.PostedTransactionCorrectionTypePaymentAmount,
		models.PostedTransactionCorrectionTypeSourceBank,
		models.PostedTransactionCorrectionTypeDestinationAccount,
		models.PostedTransactionCorrectionTypePurchaseDate,
		models.PostedTransactionCorrectionTypeOther:
		return nil
	default:
		return fmt.Errorf("jenis koreksi transaksi tidak dikenali")
	}
}

func validatePostedTransactionCorrectionResolution(value string) error {
	switch value {
	case models.PostedTransactionCorrectionResolutionSupplementalProof,
		models.PostedTransactionCorrectionResolutionJournalReclass,
		models.PostedTransactionCorrectionResolutionReverseRecreate,
		models.PostedTransactionCorrectionResolutionBankReconciliation,
		models.PostedTransactionCorrectionResolutionManualAdjustment:
		return nil
	default:
		return fmt.Errorf("rencana penyelesaian Accounting tidak dikenali")
	}
}

func normalizePostedTransactionCorrectionNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrPostedTransactionCorrectionNotFound
	}
	return err
}

func canRequestPostedTransactionCorrection(role string) bool {
	return models.IsFinanceStaffRole(role) || models.IsFinanceManagerRole(role) || isCorrectionAdmin(role)
}

func canViewAllPostedTransactionCorrections(role string) bool {
	return models.IsFinanceTeamRole(role) || isCorrectionAdmin(role) || models.NormalizeRole(role) == models.RoleDirector
}

func canReviewPostedTransactionCorrectionAccounting(role string) bool {
	return models.IsAccountingRole(role) || isCorrectionAdmin(role)
}

func canReviewPostedTransactionCorrectionFinanceManager(role string) bool {
	return models.IsFinanceManagerRole(role) || isCorrectionAdmin(role)
}

func canCompletePostedTransactionCorrection(role string) bool {
	return models.IsAccountingRole(role) || isCorrectionAdmin(role)
}

func canAddPostedTransactionCorrectionDocument(correction *models.PostedTransactionCorrection, userID uint, role string) bool {
	if correction == nil {
		return false
	}
	if models.IsAccountingRole(role) || models.IsFinanceManagerRole(role) || isCorrectionAdmin(role) {
		return true
	}
	return models.IsFinanceStaffRole(role) && correction.RequestedBy == userID
}

func isCorrectionAdmin(role string) bool {
	return models.NormalizeRole(role) == models.RoleAdmin
}
