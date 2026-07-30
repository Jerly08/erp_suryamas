package services

import (
	"fmt"
	"strings"

	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

const purchasePaymentTermTolerance = 0.005

// validatePurchasePaymentTermAllocation keeps an installment payment tied to
// one explicit term. A generic vendor-bill flow may still settle a normal
// single-term purchase, but it must never guess between multiple terms.
func validatePurchasePaymentTermAllocation(
	tx *gorm.DB,
	purchase *models.Purchase,
	paymentTermID *uint,
	amount float64,
	requireExplicitTermForMultiple bool,
) error {
	if tx == nil || purchase == nil || purchase.ID == 0 {
		return fmt.Errorf("purchase is required to validate payment term")
	}
	if amount <= 0 {
		return fmt.Errorf("payment amount must be greater than zero")
	}
	if !tx.Migrator().HasTable(&models.PurchasePaymentTerm{}) {
		return nil
	}

	var terms []models.PurchasePaymentTerm
	if err := tx.Where("purchase_id = ?", purchase.ID).
		Order("sequence ASC, due_date ASC, id ASC").
		Find(&terms).Error; err != nil {
		return fmt.Errorf("failed to load payment terms for purchase %s: %w", purchase.Code, err)
	}

	if len(terms) <= 1 && (paymentTermID == nil || *paymentTermID == 0) {
		return nil
	}

	if (paymentTermID == nil || *paymentTermID == 0) && requireExplicitTermForMultiple && len(terms) > 1 {
		return fmt.Errorf(
			"PO %s memiliki %d termin. Catat pembayaran melalui menu Bayar PO / Termin dan pilih termin yang dibayar",
			purchase.Code,
			len(terms),
		)
	}

	if paymentTermID == nil || *paymentTermID == 0 {
		return nil
	}

	var selectedTerm *models.PurchasePaymentTerm
	for index := range terms {
		if terms[index].ID == *paymentTermID {
			selectedTerm = &terms[index]
			break
		}
	}
	if selectedTerm == nil {
		return fmt.Errorf("termin pembayaran %d tidak termasuk dalam PO %s", *paymentTermID, purchase.Code)
	}

	var paidAmount float64
	if tx.Migrator().HasTable(&models.PaymentAllocation{}) {
		if err := tx.Model(&models.PaymentAllocation{}).
			Where("purchase_payment_term_id = ?", selectedTerm.ID).
			Select("COALESCE(SUM(allocated_amount), 0)").
			Scan(&paidAmount).Error; err != nil {
			return fmt.Errorf("failed to calculate payment term outstanding for PO %s: %w", purchase.Code, err)
		}
	}

	outstanding := selectedTerm.Amount - paidAmount
	if outstanding < 0 && outstanding > -purchasePaymentTermTolerance {
		outstanding = 0
	}
	if outstanding <= purchasePaymentTermTolerance {
		return fmt.Errorf("termin %d pada PO %s sudah lunas", selectedTerm.Sequence, purchase.Code)
	}
	if amount-outstanding > purchasePaymentTermTolerance {
		return fmt.Errorf(
			"nominal pembayaran melebihi sisa termin %d pada PO %s (sisa %.2f)",
			selectedTerm.Sequence,
			purchase.Code,
			outstanding,
		)
	}

	return nil
}

type purchasePaymentJournalTarget struct {
	PurchaseID uint
	TermID     *uint
}

// resolvePurchasePaymentJournalTag returns a concise PO/term label used in
// journal metadata and line descriptions. It makes the accounting trail
// reviewable without changing the journal's reference key or debit/credit.
func resolvePurchasePaymentJournalTag(tx *gorm.DB, paymentID uint64) (string, error) {
	if tx == nil || paymentID == 0 ||
		!tx.Migrator().HasTable(&models.PaymentAllocation{}) ||
		!tx.Migrator().HasTable(&models.Purchase{}) {
		return "", nil
	}

	var allocations []models.PaymentAllocation
	if err := tx.Where("payment_id = ? AND bill_id IS NOT NULL", paymentID).
		Order("id ASC").
		Find(&allocations).Error; err != nil {
		return "", fmt.Errorf("failed to load payment allocations for journal tag: %w", err)
	}

	targets := make([]purchasePaymentJournalTarget, 0, len(allocations))
	for _, allocation := range allocations {
		if allocation.BillID == nil || *allocation.BillID == 0 {
			continue
		}
		targets = append(targets, purchasePaymentJournalTarget{
			PurchaseID: *allocation.BillID,
			TermID:     allocation.PurchasePaymentTermID,
		})
	}

	return resolvePurchasePaymentJournalTagForTargets(tx, targets)
}

func resolvePurchasePaymentJournalTagForBillAllocations(
	tx *gorm.DB,
	allocations []AllocationItem,
) (string, error) {
	targets := make([]purchasePaymentJournalTarget, 0, len(allocations))
	for _, allocation := range allocations {
		if allocation.BillID == nil || *allocation.BillID == 0 {
			continue
		}
		targets = append(targets, purchasePaymentJournalTarget{
			PurchaseID: *allocation.BillID,
			TermID:     allocation.PaymentTermID,
		})
	}
	return resolvePurchasePaymentJournalTagForTargets(tx, targets)
}

func resolvePurchasePaymentJournalTagForTargets(
	tx *gorm.DB,
	targets []purchasePaymentJournalTarget,
) (string, error) {
	if tx == nil || len(targets) == 0 || !tx.Migrator().HasTable(&models.Purchase{}) {
		return "", nil
	}

	labels := make([]string, 0, len(targets))
	seen := make(map[string]struct{})
	for _, target := range targets {
		if target.PurchaseID == 0 {
			continue
		}

		var purchase models.Purchase
		if err := tx.Select("id", "code").First(&purchase, target.PurchaseID).Error; err != nil {
			return "", fmt.Errorf("failed to load purchase %d for journal tag: %w", target.PurchaseID, err)
		}

		label := strings.TrimSpace(purchase.Code)
		if target.TermID != nil && *target.TermID > 0 && tx.Migrator().HasTable(&models.PurchasePaymentTerm{}) {
			var term models.PurchasePaymentTerm
			if err := tx.Select("id", "purchase_id", "sequence").
				Where("id = ? AND purchase_id = ?", *target.TermID, purchase.ID).
				First(&term).Error; err != nil {
				return "", fmt.Errorf("failed to load payment term for PO %s: %w", purchase.Code, err)
			}
			label = fmt.Sprintf("%s | Termin %d", label, term.Sequence)
		}

		if label == "" {
			continue
		}
		if _, exists := seen[label]; exists {
			continue
		}
		seen[label] = struct{}{}
		labels = append(labels, label)
	}

	if len(labels) == 0 {
		return "", nil
	}
	if len(labels) <= 2 {
		return strings.Join(labels, " | "), nil
	}
	return fmt.Sprintf("%s | +%d PO/termin lainnya", strings.Join(labels[:2], " | "), len(labels)-2), nil
}

func appendPurchasePaymentJournalTag(text string, tag string) string {
	if strings.TrimSpace(tag) == "" {
		return text
	}
	return fmt.Sprintf("%s | %s", text, tag)
}

func purchasePaymentJournalNotes(tag string) string {
	if strings.TrimSpace(tag) == "" {
		return ""
	}
	return fmt.Sprintf("Tag pembayaran: %s", tag)
}
