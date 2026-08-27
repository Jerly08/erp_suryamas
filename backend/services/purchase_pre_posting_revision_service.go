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
	"VENDOR":                {},
	"PROJECT":               {},
	"VENDOR_OR_PROJECT":     {},
	"ITEMS_AND_AMOUNT":      {},
	"TAX_AND_COST":          {},
	"PAYMENT_CONFIGURATION": {},
	"OTHER":                 {},
}

// purchasePrePostingRevisionBusinessSnapshot captures the business-facing
// values that may change while a PO is returned to Finance Staff. It is stored
// per save in PurchasePrePostingRevisionChange so the evidence remains
// append-only even when the user saves more than once before resubmitting.
type purchasePrePostingRevisionBusinessSnapshot struct {
	PurchaseCode      string                                          `json:"purchase_code"`
	PurchaseRequestID *uint                                           `json:"purchase_request_id,omitempty"`
	Vendor            purchasePrePostingRevisionVendorSnapshot        `json:"vendor"`
	Project           *purchasePrePostingRevisionProjectSnapshot      `json:"project,omitempty"`
	TransactionDate   time.Time                                       `json:"transaction_date"`
	DueDate           time.Time                                       `json:"due_date"`
	Discount          float64                                         `json:"discount"`
	TaxAndCost        purchasePrePostingRevisionTaxAndCostSnapshot    `json:"tax_and_cost"`
	Payment           purchasePrePostingRevisionPaymentSnapshot       `json:"payment"`
	Notes             string                                          `json:"notes"`
	Items             []purchasePrePostingRevisionItemSnapshot        `json:"items"`
	PaymentTerms      []purchasePrePostingRevisionPaymentTermSnapshot `json:"payment_terms"`
}

type purchasePrePostingRevisionVendorSnapshot struct {
	ID   uint   `json:"id"`
	Code string `json:"code,omitempty"`
	Name string `json:"name,omitempty"`
}

type purchasePrePostingRevisionProjectSnapshot struct {
	ID     uint   `json:"id"`
	Code   string `json:"code,omitempty"`
	Name   string `json:"name,omitempty"`
	Status string `json:"status,omitempty"`
}

type purchasePrePostingRevisionTaxAndCostSnapshot struct {
	VATTreatment         string                                            `json:"vat_treatment"`
	PPNRate              float64                                           `json:"ppn_rate"`
	PPNAmount            float64                                           `json:"ppn_amount"`
	TaxAllocations       []purchasePrePostingRevisionTaxAllocationSnapshot `json:"tax_allocations,omitempty"`
	OtherTaxAdditions    float64                                           `json:"other_tax_additions"`
	PPh21Rate            float64                                           `json:"pph21_rate"`
	PPh23Rate            float64                                           `json:"pph23_rate"`
	OtherTaxDeductions   float64                                           `json:"other_tax_deductions"`
	FreightCost          float64                                           `json:"freight_cost"`
	OtherCostAmount      float64                                           `json:"other_cost_amount"`
	OtherCostDescription string                                            `json:"other_cost_description"`
	OtherCostAccountID   *uint                                             `json:"other_cost_account_id,omitempty"`
}

// purchasePrePostingRevisionTaxAllocationSnapshot preserves the complete tax
// treatment selected for a component-based PPN purchase.  It deliberately
// keeps the source item identity and its business description together so an
// approver can review a later item replacement without having to infer which
// component was used as the DPP basis.
type purchasePrePostingRevisionTaxAllocationSnapshot struct {
	ID                      uint       `json:"id"`
	PurchaseItemID          uint       `json:"purchase_item_id"`
	PurchaseItemDescription string     `json:"purchase_item_description,omitempty"`
	TaxTreatment            string     `json:"tax_treatment"`
	SourceAmount            float64    `json:"source_amount"`
	DPPAmount               float64    `json:"dpp_amount"`
	PPNRate                 float64    `json:"ppn_rate"`
	PPNAmount               float64    `json:"ppn_amount"`
	IsCreditable            bool       `json:"is_creditable"`
	TaxInvoiceNumber        string     `json:"tax_invoice_number,omitempty"`
	TaxInvoiceDate          *time.Time `json:"tax_invoice_date,omitempty"`
	Notes                   string     `json:"notes,omitempty"`
	VerifiedBy              *uint      `json:"verified_by,omitempty"`
	VerifiedAt              *time.Time `json:"verified_at,omitempty"`
}

type purchasePrePostingRevisionPaymentSnapshot struct {
	Method           string `json:"method"`
	BankAccountID    *uint  `json:"bank_account_id,omitempty"`
	CreditAccountID  *uint  `json:"credit_account_id,omitempty"`
	PaymentReference string `json:"payment_reference"`
}

type purchasePrePostingRevisionItemSnapshot struct {
	ID                    uint    `json:"id"`
	ProductID             *uint   `json:"product_id,omitempty"`
	PurchaseRequestItemID *uint   `json:"purchase_request_item_id,omitempty"`
	ItemType              string  `json:"item_type"`
	Description           string  `json:"description"`
	Quantity              int     `json:"quantity"`
	UnitPrice             float64 `json:"unit_price"`
	Discount              float64 `json:"discount"`
	Tax                   float64 `json:"tax"`
	ExpenseAccountID      uint    `json:"expense_account_id"`
}

type purchasePrePostingRevisionPaymentTermSnapshot struct {
	Sequence    int       `json:"sequence"`
	Percentage  float64   `json:"percentage"`
	Amount      float64   `json:"amount"`
	DueDate     time.Time `json:"due_date"`
	Description string    `json:"description"`
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
	query := db.
		Preload("Requester").
		Preload("FinanceManager").
		Preload("OpenedBy").
		Preload("ResubmittedByUser")

	// Deployments that have not yet run the append-only audit migration must
	// still be able to read older revision headers. New deployments create this
	// table through migration 088 before the API is used.
	if db.Migrator().HasTable(&models.PurchasePrePostingRevisionChange{}) {
		query = query.
			Preload("Changes", func(changeQuery *gorm.DB) *gorm.DB {
				return changeQuery.Order("created_at ASC, id ASC")
			}).
			Preload("Changes.ChangedByUser")
	}

	return query
}

// purchasePrePostingRevisionUpdateScope maps sparse PUT payload fields to the
// business approval selected when Finance Manager opened the revision. Vendor
// and Project are deliberately separate; VENDOR_OR_PROJECT remains a legacy
// compatibility grant for revisions created before the split.
type purchasePrePostingRevisionUpdateScope struct {
	TransactionDate      bool
	Vendor               bool
	Project              bool
	ItemsAndAmount       bool
	TaxAndCost           bool
	PaymentConfiguration bool
	Other                bool
}

func purchasePrePostingRevisionScopeFromRequest(request models.PurchaseUpdateRequest) purchasePrePostingRevisionUpdateScope {
	// A recalculated credit-payment schedule is a consequence of changing tax
	// or cost totals, not a request to change payment method, bank/credit COA,
	// or payment reference. Treat it as TAX_AND_COST only when it accompanies
	// such a change; a standalone payment_terms edit remains PAYMENT_CONFIGURATION.
	paymentTermsFollowTaxAndCost := request.PaymentTerms != nil &&
		(request.Tax != nil ||
			request.PPNRate != nil ||
			request.VATTreatment != nil ||
			request.TaxAllocations != nil ||
			request.OtherTaxAdditions != nil ||
			request.PPh21Rate != nil ||
			request.PPh23Rate != nil ||
			request.OtherTaxDeductions != nil ||
			request.FreightCost != nil ||
			request.OtherCostAmount != nil ||
			request.OtherCostDescription != nil ||
			request.OtherCostAccountID != nil) &&
		request.PaymentMethod == nil &&
		request.BankAccountID == nil &&
		request.CreditAccountID == nil &&
		request.PaymentReference == nil

	return purchasePrePostingRevisionUpdateScope{
		TransactionDate: request.Date != nil || request.DueDate != nil,
		Vendor:          request.VendorID != nil,
		Project:         request.ProjectID != nil || request.ClearProject,
		ItemsAndAmount:  request.Discount != nil || len(request.Items) > 0,
		TaxAndCost: request.Tax != nil ||
			request.PPNRate != nil ||
			request.VATTreatment != nil ||
			request.TaxAllocations != nil ||
			request.OtherTaxAdditions != nil ||
			request.PPh21Rate != nil ||
			request.PPh23Rate != nil ||
			request.OtherTaxDeductions != nil ||
			request.FreightCost != nil ||
			request.OtherCostAmount != nil ||
			request.OtherCostDescription != nil ||
			request.OtherCostAccountID != nil ||
			paymentTermsFollowTaxAndCost,
		PaymentConfiguration: request.PaymentMethod != nil ||
			request.BankAccountID != nil ||
			request.CreditAccountID != nil ||
			request.PaymentReference != nil ||
			(request.PaymentTerms != nil && !paymentTermsFollowTaxAndCost),
		Other: request.Notes != nil,
	}
}

func parsePurchasePrePostingRevisionTypes(serialized string) (map[string]struct{}, error) {
	var values []string
	if err := json.Unmarshal([]byte(serialized), &values); err != nil {
		return nil, fmt.Errorf("jenis revisi purchase tidak dapat dibaca: %w", err)
	}
	normalized, err := normalizePurchasePrePostingRevisionTypes(values)
	if err != nil {
		return nil, err
	}

	types := make(map[string]struct{}, len(normalized))
	for _, value := range normalized {
		types[value] = struct{}{}
	}
	return types, nil
}

func purchasePrePostingRevisionAllowsType(types map[string]struct{}, revisionType string) bool {
	if _, ok := types[revisionType]; ok {
		return true
	}

	// Older records intentionally use one combined selection. Keep them fully
	// compatible, while new VENDOR and PROJECT selections stay independent.
	if revisionType == "VENDOR" || revisionType == "PROJECT" {
		_, legacyAllowed := types["VENDOR_OR_PROJECT"]
		return legacyAllowed
	}
	return false
}

func requirePurchasePrePostingRevisionType(types map[string]struct{}, revisionType string, label string) error {
	if purchasePrePostingRevisionAllowsType(types, revisionType) {
		return nil
	}
	return fmt.Errorf("perubahan %s tidak diizinkan karena tidak dipilih pada jenis revisi yang disetujui Finance Manager", label)
}

func validatePurchasePrePostingRevisionUpdateScope(
	revision *models.PurchasePrePostingRevision,
	request models.PurchaseUpdateRequest,
) ([]string, error) {
	if revision == nil {
		return nil, fmt.Errorf("revisi Finance Staff aktif tidak ditemukan")
	}
	types, err := parsePurchasePrePostingRevisionTypes(revision.RevisionTypes)
	if err != nil {
		return nil, err
	}

	scope := purchasePrePostingRevisionScopeFromRequest(request)
	changedFields := make([]string, 0, 7)
	if scope.TransactionDate {
		if err := requirePurchasePrePostingRevisionType(types, "TRANSACTION_DATE", "tanggal transaksi atau jatuh tempo"); err != nil {
			return nil, err
		}
		changedFields = append(changedFields, "transaction_date")
	}
	if scope.Vendor {
		if err := requirePurchasePrePostingRevisionType(types, "VENDOR", "vendor"); err != nil {
			return nil, err
		}
		changedFields = append(changedFields, "vendor")
	}
	if scope.Project {
		if err := requirePurchasePrePostingRevisionType(types, "PROJECT", "proyek"); err != nil {
			return nil, err
		}
		changedFields = append(changedFields, "project")
	}
	if scope.ItemsAndAmount {
		if err := requirePurchasePrePostingRevisionType(types, "ITEMS_AND_AMOUNT", "item, kuantitas, harga, atau diskon"); err != nil {
			return nil, err
		}
		changedFields = append(changedFields, "items_and_amount")
	}
	if scope.TaxAndCost {
		if err := requirePurchasePrePostingRevisionType(types, "TAX_AND_COST", "pajak atau biaya"); err != nil {
			return nil, err
		}
		changedFields = append(changedFields, "tax_and_cost")
	}
	if scope.PaymentConfiguration {
		if err := requirePurchasePrePostingRevisionType(types, "PAYMENT_CONFIGURATION", "konfigurasi pembayaran"); err != nil {
			return nil, err
		}
		changedFields = append(changedFields, "payment_configuration")
	}
	if scope.Other {
		if err := requirePurchasePrePostingRevisionType(types, "OTHER", "catatan lain"); err != nil {
			return nil, err
		}
		changedFields = append(changedFields, "other")
	}

	return changedFields, nil
}

func (s *PurchaseService) preparePurchasePrePostingRevisionUpdate(
	purchaseID uint,
	request models.PurchaseUpdateRequest,
) (*models.PurchasePrePostingRevision, []string, error) {
	var revision models.PurchasePrePostingRevision
	if err := s.db.
		Where("purchase_id = ? AND status = ?", purchaseID, models.PurchasePrePostingRevisionStatusOpenForFinance).
		Order("id DESC").
		First(&revision).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, fmt.Errorf("purchase tidak memiliki revisi Finance Staff aktif")
		}
		return nil, nil, err
	}

	changedFields, err := validatePurchasePrePostingRevisionUpdateScope(&revision, request)
	if err != nil {
		return nil, nil, err
	}
	return &revision, changedFields, nil
}

func buildPurchasePrePostingRevisionBusinessSnapshot(db *gorm.DB, purchase *models.Purchase) (string, error) {
	if purchase == nil {
		return "", fmt.Errorf("purchase tidak ditemukan untuk snapshot revisi")
	}

	snapshot := purchasePrePostingRevisionBusinessSnapshot{
		PurchaseCode:      purchase.Code,
		PurchaseRequestID: purchase.PurchaseRequestID,
		Vendor: purchasePrePostingRevisionVendorSnapshot{
			ID: purchase.VendorID,
		},
		TransactionDate: purchase.Date,
		DueDate:         purchase.DueDate,
		Discount:        purchase.Discount,
		TaxAndCost: purchasePrePostingRevisionTaxAndCostSnapshot{
			VATTreatment:         purchase.VATTreatment,
			PPNRate:              purchase.PPNRate,
			PPNAmount:            purchase.PPNAmount,
			OtherTaxAdditions:    purchase.OtherTaxAdditions,
			PPh21Rate:            purchase.PPh21Rate,
			PPh23Rate:            purchase.PPh23Rate,
			OtherTaxDeductions:   purchase.OtherTaxDeductions,
			FreightCost:          purchase.FreightCost,
			OtherCostAmount:      purchase.OtherCostAmount,
			OtherCostDescription: purchase.OtherCostDescription,
			OtherCostAccountID:   purchase.OtherCostAccountID,
		},
		Payment: purchasePrePostingRevisionPaymentSnapshot{
			Method:           purchase.PaymentMethod,
			BankAccountID:    purchase.BankAccountID,
			CreditAccountID:  purchase.CreditAccountID,
			PaymentReference: purchase.PaymentReference,
		},
		Notes: purchase.Notes,
	}

	var vendor models.Contact
	if err := db.Select("id", "code", "name").First(&vendor, purchase.VendorID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", fmt.Errorf("gagal memuat vendor untuk snapshot revisi: %w", err)
	} else if err == nil {
		snapshot.Vendor.Code = vendor.Code
		snapshot.Vendor.Name = vendor.Name
	}

	if purchase.ProjectID != nil {
		projectSnapshot := &purchasePrePostingRevisionProjectSnapshot{ID: *purchase.ProjectID}
		var project models.Project
		if err := db.Select("id", "code", "name", "status").First(&project, *purchase.ProjectID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", fmt.Errorf("gagal memuat proyek untuk snapshot revisi: %w", err)
		} else if err == nil {
			projectSnapshot.Code = project.Code
			projectSnapshot.Name = project.Name
			projectSnapshot.Status = project.Status
		}
		snapshot.Project = projectSnapshot
	}

	var items []models.PurchaseItem
	if err := db.Where("purchase_id = ?", purchase.ID).
		Order("id ASC").
		Find(&items).Error; err != nil {
		return "", fmt.Errorf("gagal memuat item untuk snapshot revisi: %w", err)
	}
	snapshot.Items = make([]purchasePrePostingRevisionItemSnapshot, 0, len(items))
	for _, item := range items {
		snapshot.Items = append(snapshot.Items, purchasePrePostingRevisionItemSnapshot{
			ID:                    item.ID,
			ProductID:             item.ProductID,
			PurchaseRequestItemID: item.PurchaseRequestItemID,
			ItemType:              item.ItemType,
			Description:           item.Description,
			Quantity:              item.Quantity,
			UnitPrice:             item.UnitPrice,
			Discount:              item.Discount,
			Tax:                   item.Tax,
			ExpenseAccountID:      item.ExpenseAccountID,
		})
	}

	// Tax allocations were added after the original revision feature.  The
	// table can be absent temporarily on a rolling deployment, so never query
	// it until the database confirms it exists.  This keeps historical
	// revisions readable while still recording component-level PPN evidence as
	// soon as migration 089 is available.
	if db.Migrator().HasTable(&models.PurchaseTaxAllocation{}) {
		var allocations []models.PurchaseTaxAllocation
		if err := db.Where("purchase_id = ?", purchase.ID).
			Order("purchase_item_id ASC, id ASC").
			Find(&allocations).Error; err != nil {
			return "", fmt.Errorf("gagal memuat detail PPN per komponen untuk snapshot revisi: %w", err)
		}

		itemDescriptions := make(map[uint]string, len(items))
		for _, item := range items {
			description := strings.TrimSpace(item.Description)
			if description == "" && item.ProductID != nil {
				description = fmt.Sprintf("Product #%d", *item.ProductID)
			}
			itemDescriptions[item.ID] = description
		}

		snapshot.TaxAndCost.TaxAllocations = make([]purchasePrePostingRevisionTaxAllocationSnapshot, 0, len(allocations))
		for _, allocation := range allocations {
			snapshot.TaxAndCost.TaxAllocations = append(snapshot.TaxAndCost.TaxAllocations, purchasePrePostingRevisionTaxAllocationSnapshot{
				ID:                      allocation.ID,
				PurchaseItemID:          allocation.PurchaseItemID,
				PurchaseItemDescription: itemDescriptions[allocation.PurchaseItemID],
				TaxTreatment:            allocation.TaxTreatment,
				SourceAmount:            allocation.SourceAmount,
				DPPAmount:               allocation.DPPAmount,
				PPNRate:                 allocation.PPNRate,
				PPNAmount:               allocation.PPNAmount,
				IsCreditable:            allocation.IsCreditable,
				TaxInvoiceNumber:        allocation.TaxInvoiceNumber,
				TaxInvoiceDate:          allocation.TaxInvoiceDate,
				Notes:                   allocation.Notes,
				VerifiedBy:              allocation.VerifiedBy,
				VerifiedAt:              allocation.VerifiedAt,
			})
		}
	}

	if db.Migrator().HasTable(&models.PurchasePaymentTerm{}) {
		var paymentTerms []models.PurchasePaymentTerm
		if err := db.Where("purchase_id = ?", purchase.ID).
			Order("sequence ASC, due_date ASC, id ASC").
			Find(&paymentTerms).Error; err != nil {
			return "", fmt.Errorf("gagal memuat termin pembayaran untuk snapshot revisi: %w", err)
		}
		snapshot.PaymentTerms = make([]purchasePrePostingRevisionPaymentTermSnapshot, 0, len(paymentTerms))
		for _, term := range paymentTerms {
			snapshot.PaymentTerms = append(snapshot.PaymentTerms, purchasePrePostingRevisionPaymentTermSnapshot{
				Sequence:    term.Sequence,
				Percentage:  term.Percentage,
				Amount:      term.Amount,
				DueDate:     term.DueDate,
				Description: term.Description,
			})
		}
	}

	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("gagal menyusun snapshot revisi purchase: %w", err)
	}
	return string(encoded), nil
}

func recordPurchasePrePostingRevisionChange(
	db *gorm.DB,
	revision *models.PurchasePrePostingRevision,
	purchaseID uint,
	changedBy uint,
	changedFields []string,
	beforeSnapshot string,
	afterSnapshot string,
) error {
	if revision == nil || len(changedFields) == 0 || beforeSnapshot == afterSnapshot {
		return nil
	}

	encodedFields, err := json.Marshal(changedFields)
	if err != nil {
		return fmt.Errorf("gagal menyimpan daftar perubahan revisi purchase: %w", err)
	}

	return db.Create(&models.PurchasePrePostingRevisionChange{
		PurchasePrePostingRevisionID: revision.ID,
		PurchaseID:                   purchaseID,
		ChangedBy:                    changedBy,
		ChangedFields:                string(encodedFields),
		BeforeSnapshot:               beforeSnapshot,
		AfterSnapshot:                afterSnapshot,
	}).Error
}

func (s *PurchaseService) validateActivePurchaseVendor(vendorID uint) error {
	if vendorID == 0 {
		return fmt.Errorf("vendor wajib dipilih")
	}

	var vendor models.Contact
	if err := s.db.Select("id", "type", "is_active").First(&vendor, vendorID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("vendor tidak ditemukan")
		}
		return err
	}
	if strings.ToUpper(strings.TrimSpace(vendor.Type)) != models.ContactTypeVendor {
		return fmt.Errorf("kontak yang dipilih bukan vendor")
	}
	if !vendor.IsActive {
		return fmt.Errorf("vendor yang dipilih tidak aktif")
	}
	return nil
}

func (s *PurchaseService) validateActivePurchaseProject(projectID uint) error {
	if projectID == 0 {
		return fmt.Errorf("proyek wajib dipilih")
	}

	var project models.Project
	if err := s.db.Select("id", "status").First(&project, projectID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("proyek tidak ditemukan")
		}
		return err
	}
	if strings.ToUpper(strings.TrimSpace(project.Status)) != models.ProjectStatusActive {
		return fmt.Errorf("proyek yang dipilih tidak aktif")
	}
	return nil
}

// validateVendorChangeHasConfirmedProvisionalProof prevents a vendor switch
// from silently reinterpreting a proof that was uploaded before a payment was
// linked. It is only called for an actual vendor change during an open
// pre-posting revision; posted/linked payments are already blocked earlier in
// the revision workflow.
func validateVendorChangeHasConfirmedProvisionalProof(
	db *gorm.DB,
	purchaseID uint,
	confirmed bool,
) error {
	if confirmed || !db.Migrator().HasTable(&models.PurchasePayment{}) {
		return nil
	}

	var provisionalProofCount int64
	if err := db.Model(&models.PurchasePayment{}).
		Where("purchase_id = ? AND payment_id IS NULL AND deleted_at IS NULL", purchaseID).
		Where("(COALESCE(payment_proof_file_name, '') <> '' OR COALESCE(payment_proof_file_path, '') <> '' OR payment_proof_uploaded_at IS NOT NULL)").
		Count(&provisionalProofCount).Error; err != nil {
		return err
	}
	if provisionalProofCount > 0 {
		return fmt.Errorf("vendor tidak dapat diubah karena masih ada bukti pembayaran provisional; periksa bukti tersebut lalu kirim confirm_provisional_payment_proof=true untuk melanjutkan")
	}
	return nil
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
		var linkedPurchasePaymentCount int64
		if err := tx.Model(&models.PurchasePayment{}).
			Where("purchase_id = ? AND payment_id IS NOT NULL AND deleted_at IS NULL", purchase.ID).
			Count(&linkedPurchasePaymentCount).Error; err != nil {
			return err
		}
		if linkedPurchasePaymentCount > 0 {
			return fmt.Errorf("purchase sudah memiliki pembayaran; gunakan Ajukan Koreksi Transaksi Terposting")
		}
	}

	// A preliminary PurchasePayment can exist only to retain an uploaded proof
	// while Finance/Accounting have not posted anything. It must not block a
	// pre-posting revision. A cash/bank transaction, on the other hand, is a
	// financial side effect and must go through the auditable correction flow.
	if tx.Migrator().HasTable(&models.CashBankTransaction{}) {
		var cashBankTransactionCount int64
		if err := tx.Model(&models.CashBankTransaction{}).
			Where("reference_type = ? AND reference_id = ? AND deleted_at IS NULL", "PURCHASE", purchase.ID).
			Count(&cashBankTransactionCount).Error; err != nil {
			return err
		}
		if cashBankTransactionCount > 0 {
			return fmt.Errorf("purchase sudah memiliki mutasi kas/bank; gunakan Ajukan Koreksi Transaksi Terposting")
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
