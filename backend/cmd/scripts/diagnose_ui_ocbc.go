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
		var uje []map[string]interface{}
		tx.Table("unified_journal_lines").
			Joins("JOIN unified_journal_ledger ON unified_journal_ledger.id = unified_journal_lines.journal_id").
			Joins("JOIN accounts ON accounts.id = unified_journal_lines.account_id").
			Select("unified_journal_ledger.id as journal_id, unified_journal_ledger.entry_number, unified_journal_ledger.source_type, accounts.code, unified_journal_lines.debit_amount, unified_journal_lines.credit_amount").
			Where("accounts.code = '1101-005'").
			Order("unified_journal_ledger.id").
			Find(&uje)

		var totalDebit, totalCredit float64
		for _, j := range uje {
			d := j["debit_amount"].(float64)
			c := j["credit_amount"].(float64)
			totalDebit += d
			totalCredit += c
			fmt.Printf("Journal %v (%s): Debit: %.2f | Credit: %.2f\n", j["journal_id"], j["source_type"], d, c)
		}

		fmt.Printf("Total Debit: %.2f | Total Credit: %.2f | Net: %.2f\n", totalDebit, totalCredit, totalDebit-totalCredit)

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
