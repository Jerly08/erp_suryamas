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
		var accounts []models.Account
		// use Unscoped to include deleted ones, just in case the query is joining them?
		// No, the query says `a.deleted_at IS NULL`. Let's just do that.
		if err := tx.Where("code = ?", "1102-001").Find(&accounts).Error; err != nil {
			return err
		}

		fmt.Printf("Total accounts with code '1102-001' (not softly deleted): %d\n", len(accounts))
		for _, a := range accounts {
			fmt.Printf("ID: %d | Code: %s | Balance: %.2f | CreatedAt: %s\n", a.ID, a.Code, a.Balance, a.CreatedAt)

			// Also checking what lines map to THIS specific ID
			var lines []models.SSOTJournalLine
			tx.Where("account_id = ?", a.ID).Find(&lines)
			var totalD, totalC float64
			for _, l := range lines {
				d, _ := l.DebitAmount.Float64()
				c, _ := l.CreditAmount.Float64()
				totalD += d
				totalC += c

				var je models.SSOTJournalEntry
				tx.First(&je, l.JournalID)
				fmt.Printf("  -> Journal %s (Src: %s, ID: %v) Line Debit: %.2f | Credit: %.2f\n",
					je.EntryNumber, je.SourceType, je.SourceID, d, c)
			}
			fmt.Printf("  Total lines for ID %d -> Debit: %.2f, Credit: %.2f\n", a.ID, totalD, totalC)
		}

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
