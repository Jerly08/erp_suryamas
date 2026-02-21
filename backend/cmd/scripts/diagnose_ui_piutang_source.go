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
		var journals []models.SSOTJournalEntry
		tx.Select("DISTINCT source_type").Find(&journals)
		fmt.Println("Unique source types in unified_journal_ledger:")
		for _, j := range journals {
			fmt.Printf("- %s\n", j.SourceType)
		}

		// Also let's find the journal that has the 28.8M debit exactly
		var lines []models.SSOTJournalLine
		tx.Where("account_id = ? AND debit_amount > 0", 136).Find(&lines)

		fmt.Println("\n--- DEBIT LINES FOR 1102-001 (Account ID: 136) ---")
		for _, l := range lines {
			d, _ := l.DebitAmount.Float64()
			fmt.Printf("JournalID: %d | LineID: %d | Debit: %.2f | Desc: %s\n", l.JournalID, l.ID, d, l.Description)

			var journal models.SSOTJournalEntry
			if err := tx.First(&journal, l.JournalID).Error; err == nil {
				fmt.Printf("   -> Journal Source: %s, ID: %d, Code: %s\n", journal.SourceType, *journal.SourceID, journal.EntryNumber)
			}
		}

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
