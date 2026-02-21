package main

import (
	"fmt"
	"log"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/services"

	"gorm.io/gorm"
)

func main() {
	config.LoadConfig()
	db := database.ConnectDB()

	err := db.Transaction(func(tx *gorm.DB) error {
		svc := services.NewSSOTBalanceSheetService(tx)

		// Create a small wrapper function or just replicate the exact query call
		// Let's use reflection or just replicate the query since it's private. Oh wait, we can just run the query from the literal go code.
		var rawBalances []struct {
			AccountID   uint    `gorm:"column:account_id"`
			AccountCode string  `gorm:"column:account_code"`
			AccountName string  `gorm:"column:account_name"`
			AccountType string  `gorm:"column:account_type"`
			DebitTotal  float64 `gorm:"column:debit_total"`
			CreditTotal float64 `gorm:"column:credit_total"`
			NetBalance  float64 `gorm:"column:net_balance"`
		}

		journalQuery := `
			SELECT 
				MIN(a.id) as account_id,
				a.code as account_code,
				MAX(a.name) as account_name,
				UPPER(a.type) as account_type,
				COALESCE(SUM(ujl.debit_amount), 0) as debit_total,
				COALESCE(SUM(ujl.credit_amount), 0) as credit_total,
				CASE 
					WHEN UPPER(a.type) IN ('ASSET', 'EXPENSE') THEN 
						COALESCE(SUM(ujl.debit_amount), 0) - COALESCE(SUM(ujl.credit_amount), 0)
					ELSE 
						COALESCE(SUM(ujl.credit_amount), 0) - COALESCE(SUM(ujl.debit_amount), 0)
				END as net_balance
			FROM accounts a
			INNER JOIN unified_journal_lines ujl ON ujl.account_id = a.id
			INNER JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
		WHERE COALESCE(a.is_header, false) = false
		  AND UPPER(a.type) IN ('ASSET', 'LIABILITY', 'EQUITY')
		  AND a.deleted_at IS NULL
		  AND uje.status = 'POSTED'
		  AND uje.deleted_at IS NULL
		  AND uje.entry_date <= ?
		GROUP BY a.code, UPPER(a.type)
			HAVING COALESCE(SUM(ujl.debit_amount), 0) <> 0 
				OR COALESCE(SUM(ujl.credit_amount), 0) <> 0
			ORDER BY a.code
		`

		if err := tx.Raw(journalQuery, "2026-02-21").Scan(&rawBalances).Error; err != nil {
			return err
		}

		fmt.Printf("Results:\n")
		for _, b := range rawBalances {
			if b.AccountCode == "1101-005" || b.AccountCode == "1103-001" || b.AccountCode == "1102-001" {
				fmt.Printf("Account %s | Debit: %.2f | Credit: %.2f | Net: %.2f\n",
					b.AccountCode, b.DebitTotal, b.CreditTotal, b.NetBalance)
			}
		}

		// What about generating the BS data directly? Let's check what UI receives.
		data, err := svc.GenerateSSOTBalanceSheet("2026-02-21")
		if err != nil {
			return err
		}

		fmt.Printf("\n--- Balance Sheet Output ---\n")
		for _, item := range data.Assets.CurrentAssets.Items {
			fmt.Printf("Current Asset: %s | %.2f\n", item.AccountCode, item.Amount)
		}
		for _, item := range data.Assets.NonCurrentAssets.Items {
			fmt.Printf("Non-Current Asset: %s | %.2f\n", item.AccountCode, item.Amount)
		}

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
