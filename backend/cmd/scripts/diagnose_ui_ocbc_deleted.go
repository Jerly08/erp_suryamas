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
			SourceType   string
			SourceID     *uint
			DebitAmount  float64
			CreditAmount float64
			Status       string
			DeletedAt    *string
			Reference    string
		}
		var lines []line

		// Check deleted journals too
		tx.Raw(`
			SELECT 
				uje.id as journal_id, 
				uje.entry_number, 
				uje.source_type, 
				uje.source_id, 
				ujl.debit_amount, 
				ujl.credit_amount, 
				uje.status, 
				CAST(uje.deleted_at AS VARCHAR) as deleted_at,
				uje.reference
			FROM accounts a
			INNER JOIN unified_journal_lines ujl ON ujl.account_id = a.id
			INNER JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
			WHERE a.code = '1101-005'
			ORDER BY uje.id
		`).Scan(&lines)

		fmt.Printf("--- ALL OCBC LINES (Included Soft-Deleted) ---\n")
		for _, l := range lines {
			if l.DeletedAt != nil || l.Status != "POSTED" {
				fmt.Printf("DELETED/UNPOSTED -> Journal ID %d [%s] D: %.2f | C: %.2f\n",
					l.JournalID, l.EntryNumber, l.DebitAmount, l.CreditAmount)
			}
		}

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
