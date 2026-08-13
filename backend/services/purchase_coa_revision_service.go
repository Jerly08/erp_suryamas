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

type purchaseCOARevisionSnapshot struct {
	PurchaseCode    string                            `json:"purchase_code"`
	PurchaseStatus  string                            `json:"purchase_status"`
	PaymentMethod   string                            `json:"payment_method"`
	CreditAccountID *uint                             `json:"credit_account_id,omitempty"`
	Items           []purchaseCOARevisionSnapshotItem `json:"items"`
}

type purchaseCOARevisionSnapshotItem struct {
	PurchaseItemID   uint   `json:"purchase_item_id"`
	Description      string `json:"description"`
	ExpenseAccountID uint   `json:"expense_account_id"`
}

func (s *PurchaseService) ListPurchaseCOARevisions(
	purchaseID uint,
	userID uint,
	role string,
) ([]models.PurchaseCOARevision, error) {
	if !canViewPurchaseCOARevisions(role) {
		return nil, fmt.Errorf("Anda tidak memiliki akses untuk melihat riwayat revisi COA")
	}

	var revisions []models.PurchaseCOARevision
	err := s.purchaseCOARevisionQuery(s.db).
		Where("purchase_id = ?", purchaseID).
		Order("created_at DESC, id DESC").
		Find(&revisions).Error
	if err != nil {
		return nil, err
	}
	return revisions, nil
}

func (s *PurchaseService) RequestPurchaseCOARevision(
	purchaseID uint,
	request models.PurchaseCOARevisionCreateRequest,
	userID uint,
	role string,
) (*models.PurchaseCOARevision, error) {
	if !canRequestPurchaseCOARevision(role) {
		return nil, fmt.Errorf("hanya Accounting yang dapat mengajukan revisi COA sebelum jurnal diposting")
	}

	reason := strings.TrimSpace(request.Reason)
	if reason == "" {
		return nil, fmt.Errorf("alasan revisi COA wajib diisi")
	}

	var revision models.PurchaseCOARevision
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		purchase, err := loadPurchaseForCOARevision(tx, purchaseID)
		if err != nil {
			return err
		}
		if err := s.validatePurchaseEligibleForCOARevision(tx, purchase); err != nil {
			return err
		}
		if err := ensureNoActivePurchasePrePostingRevision(tx, purchaseID); err != nil {
			return err
		}

		var activeCount int64
		if err := tx.Model(&models.PurchaseCOARevision{}).
			Where("purchase_id = ? AND status = ?", purchaseID, models.PurchaseCOARevisionStatusPendingFinanceManager).
			Count(&activeCount).Error; err != nil {
			return err
		}
		if activeCount > 0 {
			return fmt.Errorf("masih ada pengajuan revisi COA yang menunggu keputusan Finance Manager")
		}

		snapshot, err := buildPurchaseCOARevisionSnapshot(purchase)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		revision = models.PurchaseCOARevision{
			PurchaseID:  purchase.ID,
			Status:      models.PurchaseCOARevisionStatusPendingFinanceManager,
			Reason:      reason,
			COASnapshot: snapshot,
			RequestedBy: userID,
			RequestedAt: now,
		}
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}

		return recordPurchaseCOARevisionHistory(
			tx,
			purchase,
			userID,
			models.ApprovalActionCreated,
			"Accounting mengajukan revisi COA sebelum jurnal diposting: "+reason,
			"{\"purchase_coa_revision_requested\":true,\"waiting_finance_manager\":true}",
		)
	}); err != nil {
		return nil, err
	}

	return s.getPurchaseCOARevisionByID(revision.ID)
}

func (s *PurchaseService) DecidePurchaseCOARevision(
	purchaseID uint,
	revisionID uint,
	request models.PurchaseCOARevisionDecisionRequest,
	userID uint,
	role string,
) (*models.PurchaseCOARevision, error) {
	if !canDecidePurchaseCOARevision(role) {
		return nil, fmt.Errorf("hanya Finance Manager yang dapat memutuskan revisi COA")
	}

	notes := strings.TrimSpace(request.Notes)
	if !request.Approve && notes == "" {
		return nil, fmt.Errorf("alasan penolakan revisi COA wajib diisi")
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var revision models.PurchaseCOARevision
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND purchase_id = ?", revisionID, purchaseID).
			First(&revision).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("pengajuan revisi COA tidak ditemukan")
			}
			return err
		}
		if revision.Status != models.PurchaseCOARevisionStatusPendingFinanceManager {
			return fmt.Errorf("pengajuan revisi COA ini sudah memiliki keputusan")
		}

		purchase, err := loadPurchaseForCOARevision(tx, purchaseID)
		if err != nil {
			return err
		}
		if err := s.validatePurchaseEligibleForCOARevision(tx, purchase); err != nil {
			return err
		}

		now := time.Now().UTC()
		updates := map[string]interface{}{
			"finance_manager_decision_notes": notes,
			"finance_manager_decision_by":    userID,
			"finance_manager_decision_at":    now,
			"updated_at":                     now,
		}

		if request.Approve {
			updates["status"] = models.PurchaseCOARevisionStatusApproved
		} else {
			updates["status"] = models.PurchaseCOARevisionStatusRejected
		}
		if err := tx.Model(&models.PurchaseCOARevision{}).Where("id = ?", revision.ID).Updates(updates).Error; err != nil {
			return err
		}

		if request.Approve {
			if err := tx.Model(&models.Purchase{}).Where("id = ?", purchase.ID).Updates(map[string]interface{}{
				"status":     models.PurchaseStatusPendingAccounting,
				"updated_at": now,
			}).Error; err != nil {
				return fmt.Errorf("gagal mengembalikan purchase ke Accounting: %w", err)
			}
			return recordPurchaseCOARevisionHistory(
				tx,
				purchase,
				userID,
				models.ApprovalActionApproved,
				"Finance Manager menyetujui revisi COA; purchase dikembalikan ke Accounting untuk pengisian ulang COA.",
				"{\"purchase_coa_revision_approved\":true,\"returned_to_accounting\":true}",
			)
		}

		return recordPurchaseCOARevisionHistory(
			tx,
			purchase,
			userID,
			models.ApprovalActionRejected,
			"Finance Manager menolak revisi COA: "+notes,
			"{\"purchase_coa_revision_rejected\":true}",
		)
	}); err != nil {
		return nil, err
	}

	return s.getPurchaseCOARevisionByID(revisionID)
}

func (s *PurchaseService) HasPendingPurchaseCOARevision(purchaseID uint) (bool, error) {
	var count int64
	err := s.db.Model(&models.PurchaseCOARevision{}).
		Where("purchase_id = ? AND status = ?", purchaseID, models.PurchaseCOARevisionStatusPendingFinanceManager).
		Count(&count).Error
	return count > 0, err
}

func (s *PurchaseService) getPurchaseCOARevisionByID(revisionID uint) (*models.PurchaseCOARevision, error) {
	var revision models.PurchaseCOARevision
	if err := s.purchaseCOARevisionQuery(s.db).First(&revision, revisionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("pengajuan revisi COA tidak ditemukan")
		}
		return nil, err
	}
	return &revision, nil
}

func (s *PurchaseService) purchaseCOARevisionQuery(db *gorm.DB) *gorm.DB {
	return db.
		Preload("Requester").
		Preload("FinanceManager")
}

func loadPurchaseForCOARevision(tx *gorm.DB, purchaseID uint) (*models.Purchase, error) {
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

func (s *PurchaseService) validatePurchaseEligibleForCOARevision(tx *gorm.DB, purchase *models.Purchase) error {
	if purchase == nil {
		return fmt.Errorf("purchase tidak ditemukan")
	}
	if purchase.ApprovalStatus != models.PurchaseApprovalApproved {
		return fmt.Errorf("revisi COA hanya dapat diajukan setelah approval Finance Manager awal selesai")
	}
	if purchase.Status != models.PurchaseStatusPendingFinalApproval {
		return fmt.Errorf("revisi COA hanya dapat diajukan saat purchase menunggu final approval (status saat ini: %s)", purchase.Status)
	}
	if s.purchaseNeedsAccountingCOA(purchase) {
		return fmt.Errorf("COA purchase belum lengkap sehingga belum dapat diajukan sebagai revisi")
	}
	if err := CheckClosedPeriod(tx, purchase.Date); err != nil {
		return err
	}

	var postedCount int64
	if err := tx.Model(&models.SSOTJournalEntry{}).
		Where("source_type = ? AND source_id = ? AND status = ? AND deleted_at IS NULL", models.SSOTSourceTypePurchase, uint64(purchase.ID), models.SSOTStatusPosted).
		Count(&postedCount).Error; err != nil {
		return err
	}
	if postedCount > 0 {
		return fmt.Errorf("purchase sudah memiliki jurnal terposting; gunakan Ajukan Koreksi Transaksi Terposting")
	}
	return nil
}

func buildPurchaseCOARevisionSnapshot(purchase *models.Purchase) (string, error) {
	if purchase == nil {
		return "", fmt.Errorf("purchase tidak ditemukan")
	}

	snapshot := purchaseCOARevisionSnapshot{
		PurchaseCode:    purchase.Code,
		PurchaseStatus:  purchase.Status,
		PaymentMethod:   purchase.PaymentMethod,
		CreditAccountID: purchase.CreditAccountID,
		Items:           make([]purchaseCOARevisionSnapshotItem, 0, len(purchase.PurchaseItems)),
	}
	for _, item := range purchase.PurchaseItems {
		snapshot.Items = append(snapshot.Items, purchaseCOARevisionSnapshotItem{
			PurchaseItemID:   item.ID,
			Description:      purchaseItemLabelForAccounting(item),
			ExpenseAccountID: item.ExpenseAccountID,
		})
	}

	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("gagal menyimpan snapshot COA purchase: %w", err)
	}
	return string(encoded), nil
}

func recordPurchaseCOARevisionHistory(
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

func canViewPurchaseCOARevisions(role string) bool {
	return models.IsFinanceTeamRole(role) || models.NormalizeRole(role) == models.RoleAdmin
}

func canRequestPurchaseCOARevision(role string) bool {
	return models.IsAccountingRole(role) || models.NormalizeRole(role) == models.RoleAdmin
}

func canDecidePurchaseCOARevision(role string) bool {
	return models.IsFinanceManagerRole(role) || models.NormalizeRole(role) == models.RoleAdmin
}
