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

		var totalD, totalC float64
		for _, l := range lines {
			totalD += l.DebitAmount

			// Is it a soft-deleted journal or unposted?
			if l.Status == "POSTED" && l.DeletedAt == nil {
				totalC += l.CreditAmount
			}

			fmt.Printf("Journal ID %d [%s / %s / SrcID: %v] - Status: %s, Deleted: %v | D: %.2f | C: %.2f\n",
				l.JournalID, l.EntryNumber, l.SourceType, l.SourceID, l.Status, l.DeletedAt != nil, l.DebitAmount, l.CreditAmount)
		}

		fmt.Printf("TOTAL DEBIT: %.2f | TOTAL VALID CREDIT: %.2f | NET: %.2f\n", totalD, totalC, totalD-totalC)
		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
