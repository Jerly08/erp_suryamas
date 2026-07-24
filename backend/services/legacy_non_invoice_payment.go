package services

import (
	"fmt"
	"strings"

	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

// markLegacyNonInvoiceEligibility exposes a compatibility action only for
// unposted legacy payments that can safely enter the non-invoice workflow.
func (eps *EnhancedPaymentServiceWithJournal) markLegacyNonInvoiceEligibility(payment *models.Payment) error {
	if payment == nil {
		return nil
	}

	eligible, err := eps.isLegacyNonInvoicePaymentEligible(eps.db, payment)
	if err != nil {
		return err
	}
	payment.LegacyNonInvoiceEligible = eligible
	return nil
}

func (eps *EnhancedPaymentServiceWithJournal) markLegacyNonInvoiceEligibilityForPayments(payments []models.Payment) error {
	for i := range payments {
		if err := eps.markLegacyNonInvoiceEligibility(&payments[i]); err != nil {
			return err
		}
	}
	return nil
}

// isLegacyNonInvoicePaymentEligible is deliberately fail-closed. A payment is
// eligible only when it is an old, untyped payment at Accounting review with
// no journal, allocation, purchase link, or cash/bank movement.
func (eps *EnhancedPaymentServiceWithJournal) isLegacyNonInvoicePaymentEligible(tx *gorm.DB, payment *models.Payment) (bool, error) {
	if tx == nil || payment == nil || payment.ID == 0 {
		return false, nil
	}
	if strings.TrimSpace(payment.PaymentType) != "" ||
		!strings.EqualFold(payment.Status, models.PaymentStatusPendingAccounting) ||
		!strings.EqualFold(payment.ApprovalStage, models.PaymentApprovalStageAccounting) ||
		payment.ExpenseAccountID != nil ||
		payment.JournalEntryID != nil ||
		payment.CashBankID == nil || *payment.CashBankID == 0 ||
		payment.Amount <= 0 ||
		models.HasPendingDraftAllocationsReference(payment.Reference) {
		return false, nil
	}

	if !legacyNonInvoiceEligibilityTablesPresent(tx) {
		return false, nil
	}

	var count int64
	if err := tx.Model(&models.PaymentAllocation{}).
		Where("payment_id = ?", payment.ID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check legacy payment allocations: %w", err)
	}
	if count > 0 {
		return false, nil
	}

	count = 0
	if err := tx.Model(&models.PurchasePayment{}).
		Where("payment_id = ?", payment.ID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check legacy purchase payment link: %w", err)
	}
	if count > 0 {
		return false, nil
	}

	count = 0
	if err := tx.Model(&models.SSOTJournalEntry{}).
		Where("UPPER(source_type) IN ? AND source_id = ? AND deleted_at IS NULL", []string{
			models.SSOTSourceTypePayment,
			"PURCHASE_PAYMENT",
		}, uint64(payment.ID)).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check legacy SSOT journal: %w", err)
	}
	if count > 0 {
		return false, nil
	}

	count = 0
	if err := tx.Model(&models.JournalEntry{}).
		Where("UPPER(reference_type) = ? AND reference_id = ?", models.JournalRefPayment, payment.ID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check legacy journal entry: %w", err)
	}
	if count > 0 {
		return false, nil
	}

	count = 0
	if err := tx.Model(&models.CashBankTransaction{}).
		Where("UPPER(reference_type) IN ? AND reference_id = ?", []string{
			"PAYMENT",
			"PAYMENT_MADE",
			"PAYMENT_RECEIVED",
			"SALES_PAYMENT",
		}, payment.ID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check legacy cash/bank movement: %w", err)
	}

	return count == 0, nil
}

func legacyNonInvoiceEligibilityTablesPresent(tx *gorm.DB) bool {
	for _, model := range []interface{}{
		&models.PaymentAllocation{},
		&models.PurchasePayment{},
		&models.SSOTJournalEntry{},
		&models.JournalEntry{},
		&models.CashBankTransaction{},
	} {
		if !tx.Migrator().HasTable(model) {
			return false
		}
	}
	return true
}
