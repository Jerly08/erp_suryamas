package main

import (
	"fmt"
	"log"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"

	"gorm.io/gorm"
)

type SSOTAccountBalance struct {
	AccountID   uint    `gorm:"column:account_id"`
	AccountCode string  `gorm:"column:account_code"`
	AccountName string  `gorm:"column:account_name"`
	AccountType string  `gorm:"column:account_type"`
	DebitTotal  float64 `gorm:"column:debit_total"`
	CreditTotal float64 `gorm:"column:credit_total"`
	NetBalance  float64 `gorm:"column:net_balance"`
}

func main() {
	config.LoadConfig()
	db := database.ConnectDB()

	asOfDate := "2026-02-21"

	var rawBalances []SSOTAccountBalance

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

	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(journalQuery, asOfDate).Scan(&rawBalances).Error; err != nil {
			return err
		}

		fmt.Printf("Retrieved %d accounts\n", len(rawBalances))
		for _, b := range rawBalances {
			if b.AccountCode == "1102-001" || b.AccountID == 136 {
				fmt.Printf("!!! BINGO !!!\n")
				fmt.Printf("ID: %d | %s - %s | Type: %s | Debit: %.2f | Credit: %.2f | Net: %.2f\n",
					b.AccountID, b.AccountCode, b.AccountName, b.AccountType, b.DebitTotal, b.CreditTotal, b.NetBalance)
				break
			}
		}

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
