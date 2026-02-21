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
		// Find any journal lines with exactly 7,215,000 for Utang Usaha (2101-001)
		var lines []models.SSOTJournalLine
		tx.Table("unified_journal_lines").
			Joins("JOIN unified_journal_ledger ON unified_journal_ledger.id = unified_journal_lines.journal_id").
			Select("unified_journal_lines.*").
			Where("account_id = (SELECT id FROM accounts WHERE code = '2101-001')").
			Where("debit_amount = 7215000 OR credit_amount = 7215000").
			Find(&lines)

		if len(lines) == 0 {
			fmt.Println("No 7,215,000 lines found on Utang Usaha (2101-001)!")
		}

		for _, l := range lines {
			d, _ := l.DebitAmount.Float64()
			c, _ := l.CreditAmount.Float64()
			var je models.SSOTJournalEntry
			tx.First(&je, l.JournalID)

			fmt.Printf("BINGO -> Journal ID %d [%s / %s] D: %.2f | C: %.2f\n",
				l.JournalID, je.EntryNumber, je.SourceType, d, c)

			// Print all lines for this journal to see what exactly it did
			var fullLines []models.SSOTJournalLine
			tx.Where("journal_id = ?", l.JournalID).Find(&fullLines)
			for _, fl := range fullLines {
				var a models.Account
				tx.First(&a, fl.AccountID)
				fd, _ := fl.DebitAmount.Float64()
				fc, _ := fl.CreditAmount.Float64()
				fmt.Printf("   Line -> Acc: %s %s | D: %.2f | C: %.2f\n", a.Code, a.Name, fd, fc)
			}
		}

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
