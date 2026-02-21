package main

import (
	"fmt"
	"log"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"

	"gorm.io/gorm"
)

func main() {
	config.LoadConfig()
	db := database.ConnectDB()

	err := db.Transaction(func(tx *gorm.DB) error {
		// Dump EXACT contents of unified_journal_lines for 1101-005
		var a models.Account
		tx.Where("code = '1101-005'").First(&a)

		fmt.Printf("OCBC Account %s ID: %d\n", a.Code, a.ID)

		type LineResult struct {
			JournalID    uint
			EntryNumber  string
			DebitAmount  float64
			CreditAmount float64
		}
		var lines []LineResult
		tx.Table("unified_journal_lines").
			Select("unified_journal_lines.journal_id, unified_journal_ledger.entry_number, unified_journal_lines.debit_amount, unified_journal_lines.credit_amount").
			Joins("JOIN unified_journal_ledger ON unified_journal_ledger.id = unified_journal_lines.journal_id").
			Where("account_id = ? AND unified_journal_ledger.status = 'POSTED' AND unified_journal_ledger.deleted_at IS NULL AND unified_journal_ledger.entry_date <= '2026-02-21'", a.ID).
			Find(&lines)

		var totalD, totalC float64
		for _, l := range lines {
			totalD += l.DebitAmount
			totalC += l.CreditAmount
			fmt.Printf("Journal ID: %d (%s) -> D: %.2f | C: %.2f\n", l.JournalID, l.EntryNumber, l.DebitAmount, l.CreditAmount)
		}

		fmt.Printf("TOTAL D: %.2f | TOTAL C: %.2f | NET: %.2f\n", totalD, totalC, totalD-totalC)

		fmt.Printf("\n--- Now let's call the actual SSOT method ---\n")
		svc := services.NewSSOTBalanceSheetService(tx)
		data, err := svc.GenerateSSOTBalanceSheet("2026-02-21")
		if err != nil {
			return err
		}

		for _, b := range data.AccountDetails {
			if b.AccountCode == "1101-005" {
				fmt.Printf("SSOT Service returned for OCBC:\n")
				fmt.Printf("Debit: %.2f | Credit: %.2f | Net: %.2f\n", b.DebitTotal, b.CreditTotal, b.NetBalance)
				break
			}
		}

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
