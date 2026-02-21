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

		fmt.Printf("--- ALL DEBITS TO UTANG USAHA (2101-001) ---\n")
		var totalD float64
		for _, l := range lines {
			d, _ := l.DebitAmount.Float64()
			totalD += d

			var je models.SSOTJournalEntry
			tx.First(&je, l.JournalID)

			// Show if it's POSTED and not deleted
			status := je.Status
			if je.DeletedAt != nil {
				status = "DELETED"
			}

			// Look for the source ID if it's a PURCHASE_PAYMENT
			var purchaseID string
			if je.SourceType == "PURCHASE_PAYMENT" && je.SourceID != nil {
				var pp models.PurchasePayment
				if err := tx.First(&pp, *je.SourceID).Error; err == nil {
					purchaseID = fmt.Sprintf("-> Payment for Purchase ID: %d", pp.PurchaseID)
				} else {
					purchaseID = "-> Payment not found in DB!"
				}
			}

			fmt.Printf("Journal ID %d [%s / %s] Status: %v | D: %.2f %s\n",
				l.JournalID, je.EntryNumber, je.SourceType, status, d, purchaseID)
		}

		fmt.Printf("Total Valid Debit: %.2f\n", totalD)

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
