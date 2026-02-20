package main

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type ClosingLine struct {
	JournalID    uint64
	EntryDate    string
	LineNumber   int
	AccountCode  string
	AccountName  string
	AccountType  string
	DebitAmount  float64
	CreditAmount float64
}

func main() {
	dsn := "host=localhost user=postgres password=postgres dbname=sistem_akuntansi port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	dates := []string{"2027-02-02", "2027-12-31"}

	for _, dateStr := range dates {
		fmt.Printf("\n========== CLOSING ENTRY FOR: %s ==========\n", dateStr)

		query := `
			SELECT 
				ujl.id as journal_id,
				ujl.entry_date::text,
				jl.line_number,
				a.code as account_code,
				a.name as account_name,
				a.type as account_type,
				jl.debit_amount,
				jl.credit_amount
			FROM unified_journal_ledger ujl
			JOIN unified_journal_lines jl ON jl.journal_id = ujl.id
			JOIN accounts a ON a.id = jl.account_id
			WHERE ujl.entry_date = ?
			AND ujl.source_type = 'CLOSING'
			AND ujl.status = 'POSTED'
			AND ujl.deleted_at IS NULL
			ORDER BY jl.line_number
		`

		var lines []ClosingLine
		err = db.Raw(query, dateStr).Scan(&lines).Error
		if err != nil {
			log.Printf("Query error: %v", err)
			continue
		}

		if len(lines) == 0 {
			fmt.Println("❌ NO CLOSING ENTRY FOUND!")
			continue
		}

		fmt.Println("\nClosing Entry Lines:")
		for _, line := range lines {
			side := "DEBIT "
			amount := line.DebitAmount
			if line.CreditAmount > 0 {
				side = "CREDIT"
				amount = line.CreditAmount
			}
			fmt.Printf("  Line %d: [%s] %s - %s (%s) = %.2f\n",
				line.LineNumber, side, line.AccountCode, line.AccountName, line.AccountType, amount)
		}

		// Check if Revenue accounts are properly closed
		fmt.Println("\n--- Checking Revenue/Expense Closure ---")
		for _, line := range lines {
			if line.AccountType == "REVENUE" {
				if line.DebitAmount > 0 {
					fmt.Printf("✅ Revenue %s DEBITED (closed): %.2f\n", line.AccountCode, line.DebitAmount)
				} else {
					fmt.Printf("❌ Revenue %s CREDITED (NOT closed!): %.2f\n", line.AccountCode, line.CreditAmount)
				}
			}
			if line.AccountType == "EXPENSE" {
				if line.CreditAmount > 0 {
					fmt.Printf("✅ Expense %s CREDITED (closed): %.2f\n", line.AccountCode, line.CreditAmount)
				} else {
					fmt.Printf("❌ Expense %s DEBITED (NOT closed!): %.2f\n", line.AccountCode, line.DebitAmount)
				}
			}
		}

		// Calculate net income from closing entry
		var revenueDebits, expenseCredits float64
		for _, line := range lines {
			if line.AccountType == "REVENUE" && line.DebitAmount > 0 {
				revenueDebits += line.DebitAmount
			}
			if line.AccountType == "EXPENSE" && line.CreditAmount > 0 {
				expenseCredits += line.CreditAmount
			}
		}
		netIncome := revenueDebits - expenseCredits
		fmt.Printf("\nNet Income from closing entry: %.2f\n", netIncome)

		// Check if Laba Ditahan received the net income
		for _, line := range lines {
			if line.AccountCode == "3201" {
				if line.CreditAmount > 0 {
					fmt.Printf("✅ Laba Ditahan CREDITED: %.2f\n", line.CreditAmount)
				} else if line.DebitAmount > 0 {
					fmt.Printf("⚠️  Laba Ditahan DEBITED: %.2f\n", line.DebitAmount)
				}
			}
		}
	}

	// Check if there are transactions BEFORE the closing entry dates
	fmt.Println("\n\n========== CHECKING TRANSACTIONS BEFORE CLOSING ==========")
	for _, dateStr := range dates {
		fmt.Printf("\nBefore %s:\n", dateStr)
		
		query := `
			SELECT 
				COUNT(DISTINCT uje.id) as count,
				COALESCE(SUM(CASE WHEN UPPER(a.type) = 'REVENUE' THEN ujl.credit_amount - ujl.debit_amount ELSE 0 END), 0) as total_revenue,
				COALESCE(SUM(CASE WHEN UPPER(a.type) = 'EXPENSE' THEN ujl.debit_amount - ujl.credit_amount ELSE 0 END), 0) as total_expense
			FROM unified_journal_ledger uje
			JOIN unified_journal_lines ujl ON ujl.journal_id = uje.id
			JOIN accounts a ON a.id = ujl.account_id
			WHERE uje.entry_date < ?
			AND UPPER(a.type) IN ('REVENUE', 'EXPENSE')
			AND uje.source_type != 'CLOSING'
			AND uje.status = 'POSTED'
			AND uje.deleted_at IS NULL
		`

		var result struct {
			Count        int64
			TotalRevenue float64
			TotalExpense float64
		}

		err = db.Raw(query, dateStr).Scan(&result).Error
		if err != nil {
			log.Printf("Query error: %v", err)
			continue
		}

		fmt.Printf("  Transactions: %d\n", result.Count)
		fmt.Printf("  Total Revenue: %.2f\n", result.TotalRevenue)
		fmt.Printf("  Total Expense: %.2f\n", result.TotalExpense)
		fmt.Printf("  Expected Net Income: %.2f\n", result.TotalRevenue-result.TotalExpense)
	}
}
