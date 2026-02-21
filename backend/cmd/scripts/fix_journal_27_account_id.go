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
		// Update the journal line of ID 27 from the incorrect 136 (1102 Header) to 270 (1102-001)
		res := tx.Exec("UPDATE unified_journal_lines SET account_id = 270 WHERE journal_id = 27 AND account_id = 136")
		if res.Error != nil {
			return res.Error
		}

		fmt.Printf("Updated %d journal lines for Journal ID 27 back to Account 270 (1102-001)\n", res.RowsAffected)

		// I will also query `GenerateSSOTBalanceSheet` query right here directly to calculate the diff.
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
		  AND uje.entry_date <= '2026-02-21'
		GROUP BY a.code, UPPER(a.type)
			HAVING COALESCE(SUM(ujl.debit_amount), 0) <> 0 
				OR COALESCE(SUM(ujl.credit_amount), 0) <> 0
		`

		type result struct {
			AccountType string
			NetBalance  float64
		}
		var results []result
		tx.Raw(journalQuery).Scan(&results)

		var totalAssets, totalLiabEquities float64
		for _, r := range results {
			if r.AccountType == "ASSET" {
				totalAssets += r.NetBalance
			} else {
				totalLiabEquities += r.NetBalance
			}
		}

		// Don't forget PPN and Net Income
		var ppn_keluaran, ppn_masukan float64
		tx.Raw(`
			SELECT 
				COALESCE(SUM(ujl.credit_amount) - SUM(ujl.debit_amount), 0)
			FROM accounts a
			INNER JOIN unified_journal_lines ujl ON ujl.account_id = a.id
			INNER JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
			WHERE a.code = '2102-001' AND uje.status = 'POSTED' AND uje.deleted_at IS NULL
		`).Scan(&ppn_keluaran)

		tx.Raw(`
			SELECT 
				COALESCE(SUM(ujl.credit_amount) - SUM(ujl.debit_amount), 0)
			FROM accounts a
			INNER JOIN unified_journal_lines ujl ON ujl.account_id = a.id
			INNER JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
			WHERE a.code = '2102-002' AND uje.status = 'POSTED' AND uje.deleted_at IS NULL
		`).Scan(&ppn_masukan)

		// Remove PPN from raw LiabEquities because they are netted
		totalLiabEquities = totalLiabEquities - ppn_keluaran - ppn_masukan

		netPPN := ppn_keluaran + ppn_masukan
		if netPPN > 0 {
			totalLiabEquities += netPPN
		} else {
			totalAssets += -netPPN
		}

		var netIncome float64
		tx.Raw(`
			SELECT 
				COALESCE(SUM(
					CASE 
						WHEN TRIM(a.type) ILIKE 'REVENUE' OR TRIM(a.type) ILIKE 'PENDAPATAN%' THEN 
							COALESCE(ujl.credit_amount, 0) - COALESCE(ujl.debit_amount, 0)
						WHEN TRIM(a.type) ILIKE 'EXPENSE' OR TRIM(a.type) ILIKE 'BEBAN%' THEN 
							COALESCE(ujl.credit_amount, 0) - COALESCE(ujl.debit_amount, 0)
						ELSE 0
					END
				), 0) as net_income
			FROM accounts a
			INNER JOIN unified_journal_lines ujl ON ujl.account_id = a.id
			INNER JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
			WHERE uje.status = 'POSTED' AND uje.deleted_at IS NULL
			AND uje.entry_date <= '2026-02-21'
			AND COALESCE(a.is_header, false) = false
			AND (
				TRIM(a.type) ILIKE 'REVENUE' OR TRIM(a.type) ILIKE 'PENDAPATAN%' OR 
				TRIM(a.type) ILIKE 'EXPENSE' OR TRIM(a.type) ILIKE 'BEBAN%'
			)
			AND a.deleted_at IS NULL
		`).Scan(&netIncome)

		totalLiabEquities += netIncome

		diff := totalAssets - totalLiabEquities
		fmt.Printf("Total Assets: %.2f\n", totalAssets)
		fmt.Printf("Total L+E:    %.2f\n", totalLiabEquities)
		fmt.Printf("Diff:         %.2f\n", diff)

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
