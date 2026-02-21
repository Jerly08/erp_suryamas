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
		var lines []models.SSOTJournalLine
		tx.Table("unified_journal_lines").
			Joins("JOIN unified_journal_ledger ON unified_journal_ledger.id = unified_journal_lines.journal_id").
			Select("unified_journal_lines.*, unified_journal_ledger.entry_number, unified_journal_ledger.source_type").
			Where("unified_journal_lines.account_id = (SELECT id FROM accounts WHERE code = '2101-001' LIMIT 1)").
			Find(&lines)

		var totalD, totalC float64
		fmt.Printf("--- TRANSACTIONS FOR 2101-001 (UTANG USAHA - LOKAL) ---\n")
		for _, l := range lines {
			d, _ := l.DebitAmount.Float64()
			c, _ := l.CreditAmount.Float64()
			totalD += d
			totalC += c

			// Try to get entry number and source type using a raw query for simplicity
			var je struct {
				EntryNumber string
				SourceType  string
			}
			tx.Raw("SELECT entry_number, source_type FROM unified_journal_ledger WHERE id = ?", l.JournalID).Scan(&je)

			fmt.Printf("Journal %d [%s / %s]: D: %.2f | C: %.2f\n",
				l.JournalID, je.EntryNumber, je.SourceType, d, c)
		}

		fmt.Printf("Total Debit: %.2f | Total Credit: %.2f | Net (C-D): %.2f\n", totalD, totalC, totalC-totalD)

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
