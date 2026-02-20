package main

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type PeriodSummary struct {
	AsOfDate    string
	Assets      float64
	Liabilities float64
	Equity      float64
	Difference  float64
}

type AccountBalance struct {
	AccountCode  string
	AccountName  string
	AccountType  string
	DebitTotal   float64
	CreditTotal  float64
	NetBalance   float64
}

func main() {
	dsn := "host=localhost user=postgres password=postgres dbname=sistem_akuntansi port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	dates := []string{"2025-12-01", "2026-12-31", "2027-02-02", "2027-12-31"}
	
	fmt.Println("=== BALANCE SHEET COMPARISON ACROSS PERIODS ===\n")

	for _, dateStr := range dates {
		fmt.Printf("\n========== AS OF: %s ==========\n", dateStr)
		
		// Query to get all account balances
		query := `
			SELECT 
				a.code as account_code,
				a.name as account_name,
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
			  AND a.is_active = true
			  AND a.deleted_at IS NULL
			  AND uje.status = 'POSTED'
			  AND uje.deleted_at IS NULL
			  AND uje.entry_date <= ?
			GROUP BY a.code, a.name, UPPER(a.type)
			HAVING COALESCE(SUM(ujl.debit_amount), 0) <> 0 
			    OR COALESCE(SUM(ujl.credit_amount), 0) <> 0
			ORDER BY a.code
		`

		var balances []AccountBalance
		err = db.Raw(query, dateStr).Scan(&balances).Error
		if err != nil {
			log.Printf("Query error: %v", err)
			continue
		}

		var totalAssets, totalLiabilities, totalEquity float64
		var revenueAccounts, expenseAccounts []AccountBalance

		fmt.Println("\n--- ALL ACCOUNTS ---")
		for _, b := range balances {
			fmt.Printf("%s - %s (%s): %.2f (D: %.2f, C: %.2f)\n",
				b.AccountCode, b.AccountName, b.AccountType, b.NetBalance, b.DebitTotal, b.CreditTotal)

			switch b.AccountType {
			case "ASSET":
				totalAssets += b.NetBalance
			case "LIABILITY":
				totalLiabilities += b.NetBalance
			case "EQUITY":
				totalEquity += b.NetBalance
			case "REVENUE":
				revenueAccounts = append(revenueAccounts, b)
			case "EXPENSE":
				expenseAccounts = append(expenseAccounts, b)
			}
		}

		fmt.Println("\n--- REVENUE/EXPENSE ACCOUNTS (Should be 0 after closing) ---")
		if len(revenueAccounts) > 0 || len(expenseAccounts) > 0 {
			fmt.Println("⚠️  WARNING: Period NOT properly closed!")
			for _, r := range revenueAccounts {
				fmt.Printf("  Revenue: %s - %s = %.2f\n", r.AccountCode, r.AccountName, r.NetBalance)
			}
			for _, e := range expenseAccounts {
				fmt.Printf("  Expense: %s - %s = %.2f\n", e.AccountCode, e.AccountName, e.NetBalance)
			}
		} else {
			fmt.Println("✅ Period properly closed - no revenue/expense balances")
		}

		fmt.Println("\n--- BALANCE SUMMARY ---")
		fmt.Printf("Total Assets:      %.2f\n", totalAssets)
		fmt.Printf("Total Liabilities: %.2f\n", totalLiabilities)
		fmt.Printf("Total Equity:      %.2f\n", totalEquity)
		fmt.Printf("L + E:             %.2f\n", totalLiabilities+totalEquity)
		difference := totalAssets - (totalLiabilities + totalEquity)
		fmt.Printf("Difference:        %.2f\n", difference)
		
		if difference > 0.01 || difference < -0.01 {
			fmt.Printf("❌ NOT BALANCED (diff: %.2f)\n", difference)
		} else {
			fmt.Println("✅ BALANCED")
		}

		// Check for closing entries on this date
		var closingCount int64
		db.Raw(`
			SELECT COUNT(*) 
			FROM unified_journal_ledger 
			WHERE source_type = 'CLOSING' 
			AND entry_date = ?
			AND status = 'POSTED'
			AND deleted_at IS NULL
		`, dateStr).Scan(&closingCount)
		
		fmt.Printf("\nClosing entries on this date: %d\n", closingCount)
	}

	// Check if there are any REVENUE/EXPENSE entries AFTER each closing date
	fmt.Println("\n\n=== CHECKING FOR TRANSACTIONS AFTER CLOSING ===")
	for i, dateStr := range dates {
		if i < len(dates)-1 {
			nextDate := dates[i+1]
			
			var count int64
			db.Raw(`
				SELECT COUNT(DISTINCT uje.id)
				FROM unified_journal_ledger uje
				JOIN unified_journal_lines ujl ON ujl.journal_id = uje.id
				JOIN accounts a ON a.id = ujl.account_id
				WHERE uje.entry_date > ? AND uje.entry_date <= ?
				AND UPPER(a.type) IN ('REVENUE', 'EXPENSE')
				AND uje.source_type != 'CLOSING'
				AND uje.status = 'POSTED'
				AND uje.deleted_at IS NULL
			`, dateStr, nextDate).Scan(&count)
			
			fmt.Printf("Between %s and %s: %d revenue/expense transactions\n", dateStr, nextDate, count)
		}
	}
}
