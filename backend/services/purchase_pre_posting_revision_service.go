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

var purchasePrePostingRevisionTypes = map[string]struct{}{
	"TRANSACTION_DATE":      {},
	"VENDOR_OR_PROJECT":     {},
	"ITEMS_AND_AMOUNT":      {},
	"TAX_AND_COST":          {},
	"PAYMENT_CONFIGURATION": {},
	"OTHER":                 {},
}

// ListPurchasePrePostingRevisions returns the auditable history for a
// purchase that was returned to Finance Staff before any accounting posting.
func (s *PurchaseService) ListPurchasePrePostingRevisions(
	purchaseID uint,
	userID uint,
	role string,
) ([]models.PurchasePrePostingRevision, error) {
	if !canViewPurchasePrePostingRevisions(role) {
		return nil, fmt.Errorf("Anda tidak memiliki akses untuk melihat riwayat revisi data purchase")
	}

	var revisions []models.PurchasePrePostingRevision
	if err := s.purchasePrePostingRevisionQuery(s.db).
		Where("purchase_id = ?", purchaseID).
		Order("created_at DESC, id DESC").
		Find(&revisions).Error; err != nil {
		return nil, err
	}
	return revisions, nil
}

// RequestPurchasePrePostingRevision either returns an initial approval to
// Finance Staff immediately, or asks Finance Manager to reopen a purchase
// that has already reached Accounting/final review. This path is strictly
// pre-posting and never mutates journals, cash/bank, stock, or payments.
func (s *PurchaseService) RequestPurchasePrePostingRevision(
	purchaseID uint,
	request models.PurchasePrePostingRevisionCreateRequest,
	userID uint,
	role string,
) (*models.PurchasePrePostingRevision, error) {
	if !canRequestPurchasePrePostingRevision(role) {
		return nil, fmt.Errorf("hanya Finance Staff yang dapat mengajukan revisi data purchase sebelum jurnal diposting")
	}

	revisionTypes, err := normalizePurchasePrePostingRevisionTypes(request.RevisionTypes)
	if err != nil {
		return nil, err
	}
	reason := strings.TrimSpace(request.Reason)
	if reason == "" {
		return nil, fmt.Errorf("alasan revisi wajib diisi")
	}

	var revision models.PurchasePrePostingRevision
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		purchase, err := loadPurchaseForPrePostingRevision(tx, purchaseID)
		if err != nil {
			return err
		}
		if err := s.validatePurchaseEligibleForPrePostingRevision(tx, purchase); err != nil {
			return err
		}

		var activeCount int64
		if err := tx.Model(&models.PurchasePrePostingRevision{}).
			Where("purchase_id = ? AND status IN ?", purchaseID, []string{
				models.PurchasePrePostingRevisionStatusPendingFinanceManager,
				models.PurchasePrePostingRevisionStatusOpenForFinance,
			}).
			Count(&activeCount).Error; err != nil {
			return err
		}
		if activeCount > 0 {
			return fmt.Errorf("masih ada pengajuan revisi data purchase yang aktif")
		}

		var pendingCOARevisionCount int64
		if err := tx.Model(&models.PurchaseCOARevision{}).
			Where("purchase_id = ? AND status = ?", purchaseID, models.PurchaseCOARevisionStatusPendingFinanceManager).
			Count(&pendingCOARevisionCount).Error; err != nil {
			return err
		}
		if pendingCOARevisionCount > 0 {
			return fmt.Errorf("masih ada pengajuan revisi COA yang menunggu keputusan Finance Manager")
		}

		revisionTypesJSON, err := json.Marshal(revisionTypes)
		if err != nil {
			return fmt.Errorf("gagal menyimpan jenis revisi: %w", err)
		}

		now := time.Now().UTC()
		revision = models.PurchasePrePostingRevision{
			PurchaseID:                purchase.ID,
			Status:                    models.PurchasePrePostingRevisionStatusPendingFinanceManager,
			RevisionTypes:             string(revisionTypesJSON),
			Reason:                    reason,
			OriginalApprovalRequestID: purchase.ApprovalRequestID,
			RequestedBy:               userID,
			RequestedAt:               now,
		}
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}

		if purchase.Status == models.PurchaseStatusPending ||
			purchase.Status == models.PurchaseStatusPendingApproval {
			return s.openPurchaseForFinanceRevision(
				tx,
				purchase,
				&revision,
				userID,
				"Finance Staff menarik purchase untuk revisi sebelum approval Finance Manager selesai: "+reason,
			)
		}

		return recordPurchasePrePostingRevisionHistory(
			tx,
			purchase,
			userID,
			models.ApprovalActionCreated,
			"Finance Staff mengajukan revisi data purchase sebelum jurnal diposting: "+reason,
			"{\"purchase_pre_posting_revision_requested\":true,\"waiting_finance_manager\":true}",
		)
	}); err != nil {
		return nil, err
	}

	return s.getPurchasePrePostingRevisionByID(revision.ID)
}

// DecidePurchasePrePostingRevision makes Finance Manager the gate for a
// purchase that already passed the first approval and is waiting for
// Accounting or final approval.
func (s *PurchaseService) DecidePurchasePrePostingRevision(
	purchaseID uint,
	revisionID uint,
	request models.PurchasePrePostingRevisionDecisionRequest,
	userID uint,
	role string,
) (*models.PurchasePrePostingRevision, error) {
	if !canDecidePurchasePrePostingRevision(role) {
		return nil, fmt.Errorf("hanya Finance Manager yang dapat memutuskan revisi data purchase")
	}

	notes := strings.TrimSpace(request.Notes)
	if !request.Approve && notes == "" {
		return nil, fmt.Errorf("alasan penolakan revisi wajib diisi")
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var revision models.PurchasePrePostingRevision
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND purchase_id = ?", revisionID, purchaseID).
			First(&revision).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("pengajuan revisi data purchase tidak ditemukan")
			}
			return err
		}
		if revision.Status != models.PurchasePrePostingRevisionStatusPendingFinanceManager {
			return fmt.Errorf("pengajuan revisi data purchase ini sudah memiliki keputusan")
		}

		purchase, err := loadPurchaseForPrePostingRevision(tx, purchaseID)
		if err != nil {
			return err
		}
		if err := s.validatePurchaseEligibleForPrePostingRevision(tx, purchase); err != nil {
			return err
		}

		now := time.Now().UTC()
		if !request.Approve {
			if err := tx.Model(&models.PurchasePrePostingRevision{}).
				Where("id = ?", revision.ID).
				Updates(map[string]interface{}{
					"status":                         models.PurchasePrePostingRevisionStatusRejected,
					"finance_manager_decision_notes": notes,
					"finance_manager_decision_by":    userID,
					"finance_manager_decision_at":    now,
					"updated_at":                     now,
				}).Error; err != nil {
				return err
			}
			return recordPurchasePrePostingRevisionHistory(
				tx,
				purchase,
				userID,
				models.ApprovalActionRejected,
				"Finance Manager menolak revisi data purchase: "+notes,
				"{\"purchase_pre_posting_revision_rejected\":true}",
			)
		}

		if err := tx.Model(&models.PurchasePrePostingRevision{}).
			Where("id = ?", revision.ID).
			Updates(map[string]interface{}{
				"finance_manager_decision_notes": notes,
				"finance_manager_decision_by":    userID,
				"finance_manager_decision_at":    now,
				"updated_at":                     now,
			}).Error; err != nil {
			return err
		}

		return s.openPurchaseForFinanceRevision(
			tx,
			purchase,
			&revision,
			userID,
			"Finance Manager menyetujui revisi data purchase sebelum jurnal diposting.",
		)
	}); err != nil {
		return nil, err
	}

	return s.getPurchasePrePostingRevisionByID(revisionID)
}

func (s *PurchaseService) HasActivePurchasePrePostingRevision(purchaseID uint) (bool, error) {
	count, err := countActivePurchasePrePostingRevisions(s.db, purchaseID)
	return count > 0, err
}

func countActivePurchasePrePostingRevisions(db *gorm.DB, purchaseID uint) (int64, error) {
	var count int64
	err := db.Model(&models.PurchasePrePostingRevision{}).
		Where("purchase_id = ? AND status IN ?", purchaseID, []string{
			models.PurchasePrePostingRevisionStatusPendingFinanceManager,
			models.PurchasePrePostingRevisionStatusOpenForFinance,
		}).
		Count(&count).Error
	return count, err
}

func ensureNoActivePurchasePrePostingRevision(db *gorm.DB, purchaseID uint) error {
	count, err := countActivePurchasePrePostingRevisions(db, purchaseID)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("purchase sedang dalam revisi Finance Staff")
	}
	return nil
}

func (s *PurchaseService) markPurchasePrePostingRevisionResubmitted(
	purchaseID uint,
	userID uint,
	approvalRequestID *uint,
) error {
	now := time.Now().UTC()
	result := s.db.Model(&models.PurchasePrePostingRevision{}).
		Where("purchase_id = ? AND status = ?", purchaseID, models.PurchasePrePostingRevisionStatusOpenForFinance).
		Updates(map[string]interface{}{
			"status":                          models.PurchasePrePostingRevisionStatusResubmitted,
			"resubmitted_by":                  userID,
			"resubmitted_at":                  now,
			"resubmitted_approval_request_id": approvalRequestID,
			"updated_at":                      now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("tidak ada revisi Finance Staff aktif untuk disubmit ulang")
	}
	return nil
}

func (s *PurchaseService) getPurchasePrePostingRevisionByID(
	revisionID uint,
) (*models.PurchasePrePostingRevision, error) {
	var revision models.PurchasePrePostingRevision
	if err := s.purchasePrePostingRevisionQuery(s.db).First(&revision, revisionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("pengajuan revisi data purchase tidak ditemukan")
		}
		return nil, err
	}
	return &revision, nil
}

func (s *PurchaseService) purchasePrePostingRevisionQuery(db *gorm.DB) *gorm.DB {
	return db.
		Preload("Requester").
		Preload("FinanceManager").
		Preload("OpenedBy").
		Preload("ResubmittedByUser")
}

func loadPurchaseForPrePostingRevision(tx *gorm.DB, purchaseID uint) (*models.Purchase, error) {
	var purchase models.Purchase
	if err := tx.Preload("PurchaseItems").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&purchase, purchaseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("purchase tidak ditemukan")
		}
		return nil, err
	}
	return &purchase, nil
}

func (s *PurchaseService) validatePurchaseEligibleForPrePostingRevision(
	tx *gorm.DB,
	purchase *models.Purchase,
) error {
	if purchase == nil {
		return fmt.Errorf("purchase tidak ditemukan")
	}

	switch purchase.Status {
	case models.PurchaseStatusPending,
		models.PurchaseStatusPendingApproval,
		models.PurchaseStatusPendingAccounting,
		models.PurchaseStatusPendingFinalApproval:
	default:
		return fmt.Errorf("purchase tidak dapat direvisi pada status %s", purchase.Status)
	}

	if err := CheckClosedPeriod(tx, purchase.Date); err != nil {
		return err
	}

	var postedCount int64
	if err := tx.Model(&models.SSOTJournalEntry{}).
		Where("source_type = ? AND source_id = ? AND status = ? AND deleted_at IS NULL",
			models.SSOTSourceTypePurchase,
			uint64(purchase.ID),
			models.SSOTStatusPosted,
		).
		Count(&postedCount).Error; err != nil {
		return err
	}
	if postedCount > 0 {
		return fmt.Errorf("purchase sudah memiliki jurnal terposting; gunakan Ajukan Koreksi Transaksi Terposting")
	}

	if tx.Migrator().HasTable(&models.PurchasePayment{}) {
		var purchasePaymentCount int64
		if err := tx.Model(&models.PurchasePayment{}).
			Where("purchase_id = ?", purchase.ID).
			Count(&purchasePaymentCount).Error; err != nil {
			return err
		}
		if purchasePaymentCount > 0 {
			return fmt.Errorf("purchase sudah memiliki pembayaran; gunakan Ajukan Koreksi Transaksi Terposting")
		}
	}

	return nil
}

func (s *PurchaseService) openPurchaseForFinanceRevision(
	tx *gorm.DB,
	purchase *models.Purchase,
	revision *models.PurchasePrePostingRevision,
	actorID uint,
	historyComment string,
) error {
	if purchase == nil || revision == nil {
		return fmt.Errorf("purchase atau pengajuan revisi tidak ditemukan")
	}

	now := time.Now().UTC()
	if purchase.ApprovalRequestID != nil {
		if err := recordPurchasePrePostingRevisionHistory(
			tx,
			purchase,
			actorID,
			models.ApprovalActionApproved,
			historyComment,
			"{\"purchase_pre_posting_revision_opened\":true,\"returned_to_finance_staff\":true}",
		); err != nil {
			return err
		}

		var approvalRequest models.ApprovalRequest
		if err := tx.First(&approvalRequest, *purchase.ApprovalRequestID).Error; err == nil &&
			approvalRequest.Status == models.ApprovalStatusPending {
			if err := tx.Model(&models.ApprovalAction{}).
				Where("request_id = ? AND status = ?", approvalRequest.ID, models.ApprovalStatusPending).
				Updates(map[string]interface{}{
					"status":      models.ApprovalStatusCancelled,
					"comments":    "Purchase ditarik untuk revisi Finance Staff sebelum jurnal diposting",
					"action_date": now,
					"is_active":   false,
				}).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.ApprovalRequest{}).
				Where("id = ? AND status = ?", approvalRequest.ID, models.ApprovalStatusPending).
				Updates(map[string]interface{}{
					"status":        models.ApprovalStatusCancelled,
					"completed_at":  now,
					"reject_reason": "Purchase ditarik untuk revisi Finance Staff sebelum jurnal diposting",
					"updated_at":    now,
				}).Error; err != nil {
				return err
			}
		}
	}

	if err := tx.Model(&models.PurchaseItem{}).
		Where("purchase_id = ?", purchase.ID).
		Updates(map[string]interface{}{
			"expense_account_id": 0,
			"updated_at":         now,
		}).Error; err != nil {
		return fmt.Errorf("gagal mengosongkan COA item purchase untuk revisi: %w", err)
	}

	if err := tx.Model(&models.Purchase{}).
		Where("id = ?", purchase.ID).
		Updates(map[string]interface{}{
			"status":                models.PurchaseStatusPendingFinanceRevision,
			"approval_status":       models.PurchaseApprovalNotStarted,
			"approval_request_id":   nil,
			"approved_by":           nil,
			"approved_at":           nil,
			"credit_account_id":     nil,
			"other_cost_account_id": nil,
			"requires_approval":     true,
			"updated_at":            now,
		}).Error; err != nil {
		return fmt.Errorf("gagal membuka purchase untuk revisi Finance Staff: %w", err)
	}

	if err := tx.Model(&models.PurchasePrePostingRevision{}).
		Where("id = ?", revision.ID).
		Updates(map[string]interface{}{
			"status":                 models.PurchasePrePostingRevisionStatusOpenForFinance,
			"opened_for_revision_by": actorID,
			"opened_for_revision_at": now,
			"updated_at":             now,
		}).Error; err != nil {
		return fmt.Errorf("gagal memperbarui status revisi purchase: %w", err)
	}

	purchase.Status = models.PurchaseStatusPendingFinanceRevision
	purchase.ApprovalStatus = models.PurchaseApprovalNotStarted
	purchase.ApprovalRequestID = nil
	purchase.ApprovedBy = nil
	purchase.ApprovedAt = nil
	purchase.CreditAccountID = nil
	purchase.OtherCostAccountID = nil
	return nil
}

func normalizePurchasePrePostingRevisionTypes(values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.ToUpper(strings.TrimSpace(value))
		if _, ok := purchasePrePostingRevisionTypes[normalized]; !ok {
			return nil, fmt.Errorf("jenis revisi %q tidak didukung", value)
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("minimal satu jenis revisi wajib dipilih")
	}
	return result, nil
}

func recordPurchasePrePostingRevisionHistory(
	tx *gorm.DB,
	purchase *models.Purchase,
	userID uint,
	action string,
	comments string,
	metadata string,
) error {
	if purchase == nil || purchase.ApprovalRequestID == nil {
		return nil
	}
	return tx.Create(&models.ApprovalHistory{
		RequestID: *purchase.ApprovalRequestID,
		UserID:    userID,
		Action:    action,
		Comments:  comments,
		Metadata:  metadata,
	}).Error
}

func canViewPurchasePrePostingRevisions(role string) bool {
	return models.IsFinanceTeamRole(role) || models.NormalizeRole(role) == models.RoleAdmin
}

func canRequestPurchasePrePostingRevision(role string) bool {
	return models.IsFinanceStaffRole(role) || models.NormalizeRole(role) == models.RoleAdmin
}

func canDecidePurchasePrePostingRevision(role string) bool {
	return models.IsFinanceManagerRole(role) || models.NormalizeRole(role) == models.RoleAdmin
}
