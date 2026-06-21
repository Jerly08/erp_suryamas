package database

import (
	"errors"
	"log"
	"math"

	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

// AlignPayrollRecognitionDatesWithJournals repairs legacy metadata where the
// payroll row still points to period end although its accrual journal was
// posted on an earlier approved posting date. The posted journal is the
// authoritative accounting date.
func AlignPayrollRecognitionDatesWithJournals(db *gorm.DB) {
	var runs []models.PayrollRun
	if err := db.Where("pph_payable_journal_entry_id IS NOT NULL").Find(&runs).Error; err != nil {
		log.Printf("Warning: payroll recognition-date alignment scan failed: %v", err)
		return
	}

	for i := range runs {
		run := runs[i]
		if run.PPhPayableJournalEntryID == nil {
			continue
		}
		var journal models.SSOTJournalEntry
		if err := db.Select("id", "entry_date").First(&journal, *run.PPhPayableJournalEntryID).Error; err != nil {
			continue
		}
		if run.PPhRecognitionDate != nil && run.PPhRecognitionDate.Equal(journal.EntryDate) {
			continue
		}
		if err := db.Model(&models.PayrollRun{}).Where("id = ?", run.ID).
			Update("pph_recognition_date", journal.EntryDate).Error; err != nil {
			log.Printf("Warning: payroll %s recognition-date alignment failed: %v", run.PayrollNumber, err)
		}
	}
}

// BackfillPayrollBPJSSettlementStatus initializes the new BPJS settlement
// columns for payroll rows created before BPJS was tracked separately from PPh
// and salary settlements.
func BackfillPayrollBPJSSettlementStatus(db *gorm.DB) {
	if err := db.Model(&models.PayrollRun{}).
		Where("bpjs_payment_status IS NULL OR bpjs_payment_status = ''").
		Update("bpjs_payment_status", models.PayrollSalaryPaymentStatusUnpaid).Error; err != nil {
		log.Printf("Warning: payroll BPJS status backfill failed: %v", err)
	}

	if err := db.Model(&models.PayrollRun{}).
		Where("COALESCE(total_bpjs, 0) <= 0.01 AND COALESCE(bpjs_paid_amount, 0) <= 0.01").
		Update("bpjs_payment_status", models.PayrollSalaryPaymentStatusPaid).Error; err != nil {
		log.Printf("Warning: payroll zero-BPJS status backfill failed: %v", err)
	}
}

// MigrateLegacyPayrollCombinedJournals only backfills settlement metadata.
// It deliberately does not edit, reverse, or repost historical journals.
func MigrateLegacyPayrollCombinedJournals(db *gorm.DB) {
	var runs []models.PayrollRun
	if err := db.
		Where("status = ? AND pph_payable_journal_entry_id IS NOT NULL AND COALESCE(salary_paid_amount, 0) <= 0.01", models.PayrollStatusApproved).
		Find(&runs).Error; err != nil {
		log.Printf("Warning: legacy payroll settlement scan failed: %v", err)
		return
	}

	for i := range runs {
		run := runs[i]
		if run.PPhPayableJournalEntryID == nil || run.TotalOverall <= 0.01 {
			continue
		}

		var journal models.SSOTJournalEntry
		if err := db.Preload("Lines.Account").First(&journal, *run.PPhPayableJournalEntryID).Error; err != nil {
			continue
		}
		if journal.SourceType != "PAYROLL_PPH_PAYABLE" {
			continue
		}

		var cashBank models.CashBank
		paidAmount := 0.0
		for _, line := range journal.Lines {
			credit := line.CreditAmount.InexactFloat64()
			if credit <= 0.01 {
				continue
			}
			if err := db.Where("account_id = ? AND deleted_at IS NULL", line.AccountID).First(&cashBank).Error; err == nil {
				paidAmount += credit
				break
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				break
			}
		}
		if paidAmount <= 0.01 || math.Abs(paidAmount-run.TotalOverall) > 0.01 || cashBank.ID == 0 {
			continue
		}

		createdByID := run.CreatedByID
		if run.ApprovedByID != nil {
			createdByID = *run.ApprovedByID
		}
		settlement := models.PayrollSettlement{
			PayrollRunID:   run.ID,
			SettlementType: models.PayrollSettlementTypeSalary,
			Amount:         paidAmount,
			PaymentDate:    journal.EntryDate,
			CashBankID:     cashBank.ID,
			JournalEntryID: journal.ID,
			Reference:      journal.Reference,
			Notes:          "Backfill dari jurnal payroll gabungan legacy; jurnal asli tidak diubah.",
			Source:         "LEGACY_COMBINED",
			CreatedByID:    createdByID,
		}

		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("journal_entry_id = ? AND settlement_type = ?", journal.ID, models.PayrollSettlementTypeSalary).
				FirstOrCreate(&settlement).Error; err != nil {
				return err
			}
			return tx.Model(&models.PayrollRun{}).Where("id = ? AND COALESCE(salary_paid_amount, 0) <= 0.01", run.ID).Updates(map[string]interface{}{
				"salary_paid_amount":              paidAmount,
				"salary_payment_status":           models.PayrollSalaryPaymentStatusPaid,
				"salary_payment_date":             journal.EntryDate,
				"salary_payment_reference":        journal.Reference,
				"salary_payment_notes":            settlement.Notes,
				"salary_payment_journal_entry_id": journal.ID,
			}).Error
		}); err != nil {
			log.Printf("Warning: legacy payroll %s settlement backfill failed: %v", run.PayrollNumber, err)
		}
	}
}
