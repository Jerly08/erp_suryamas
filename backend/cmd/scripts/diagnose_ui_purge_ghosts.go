package main

import (
	"fmt"
	"log"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

func main() {
	config.LoadConfig()
	db := database.ConnectDB()

	err := db.Transaction(func(tx *gorm.DB) error {
		// List all journals that debit Utang Usaha (2101-001)
		var lines []models.SSOTJournalLine
		tx.Table("unified_journal_lines").
			Joins("JOIN unified_journal_ledger ON unified_journal_ledger.id = unified_journal_lines.journal_id").
			Select("unified_journal_lines.*").
			Where("account_id = (SELECT id FROM accounts WHERE code = '2101-001')").
			Where("debit_amount > 0").
			Find(&lines)

		var totalPurged float64
		for _, l := range lines {
			d, _ := l.DebitAmount.Float64()
			var je models.SSOTJournalEntry
			tx.First(&je, l.JournalID)

			// If it's a purchase payment or general payment...
			if (je.SourceType == "PURCHASE_PAYMENT" || je.SourceType == "PAYMENT") && je.SourceID != nil {
				var payment models.Payment
				// Use Unscoped to find soft-deleted payments
				err := tx.Unscoped().First(&payment, *je.SourceID).Error

				// Purge if payment is missing entirely OR if it is soft-deleted
				if err != nil || payment.DeletedAt.Valid {
					fmt.Printf("Purging Soft-Deleted/Orphaned Journal ID %d [%s] for %.2f\n", je.ID, je.EntryNumber, d)
					// Delete all lines
					tx.Where("journal_id = ?", je.ID).Delete(&models.SSOTJournalLine{})
					// Delete journal
					tx.Delete(&je)
					totalPurged += d
				}
			}
		}

		fmt.Printf("Total Soft-Deleted/Orphaned Utang debits purged: %.2f\n", totalPurged)

		// Run a quick tally to see what's left
		var remainingD, remainingC float64
		var left []models.SSOTJournalLine
		tx.Table("unified_journal_lines").
			Joins("JOIN unified_journal_ledger ON unified_journal_ledger.id = unified_journal_lines.journal_id").
			Select("unified_journal_lines.*").
			Where("account_id = (SELECT id FROM accounts WHERE code = '2101-001')").
			Find(&left)

		for _, l := range left {
			d, _ := l.DebitAmount.Float64()
			c, _ := l.CreditAmount.Float64()
			remainingD += d
			remainingC += c
		}
		fmt.Printf("REMAINING ACTIVE UTANG: D=%.2f, C=%.2f, NET=%.2f\n", remainingD, remainingC, remainingC-remainingD)

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
