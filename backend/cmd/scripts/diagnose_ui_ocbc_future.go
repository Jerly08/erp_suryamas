package main

import (
	"fmt"
	"log"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"

	"gorm.io/gorm"
)

func main() {
	config.LoadConfig()
	db := database.ConnectDB()

	err := db.Transaction(func(tx *gorm.DB) error {
		type line struct {
			JournalID    uint
			EntryNumber  string
			EntryDate    string
			DebitAmount  float64
			CreditAmount float64
		}
		var lines []line

		// Note NO entry_date filter here
		tx.Raw(`
			SELECT 
				uje.id as journal_id, 
				uje.entry_number, 
				CAST(uje.entry_date AS VARCHAR) as entry_date,
				ujl.debit_amount, 
				ujl.credit_amount
			FROM accounts a
			INNER JOIN unified_journal_lines ujl ON ujl.account_id = a.id
			INNER JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
			WHERE a.code = '1101-005'
			ORDER BY uje.id
		`).Scan(&lines)

		fmt.Printf("--- ALL OCBC LINES (ANY DATE) ---\n")
		var totalC float64
		for _, l := range lines {
			if l.CreditAmount > 0 {
				fmt.Printf("Journal ID %d [%s / %s] C: %.2f\n",
					l.JournalID, l.EntryNumber, l.EntryDate, l.CreditAmount)
				totalC += l.CreditAmount
			}
		}
		fmt.Printf("Total Credit regardless of date: %.2f\n", totalC)

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
