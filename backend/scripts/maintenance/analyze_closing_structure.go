package main

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type ClosingEntry struct {
	JournalID    uint      `gorm:"column:journal_id"`
	EntryNumber  string    `gorm:"column:entry_number"`
	EntryDate    time.Time `gorm:"column:entry_date"`
	LineNumber   int       `gorm:"column:line_number"`
	AccountCode  string    `gorm:"column:account_code"`
	AccountName  string    `gorm:"column:account_name"`
	AccountType  string    `gorm:"column:account_type"`
	DebitAmount  float64   `gorm:"column:debit_amount"`
	CreditAmount float64   `gorm:"column:credit_amount"`
}

func main() {
	dsn := "host=localhost user=postgres password=postgres dbname=sistem_akuntansi port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	fmt.Println("=== DETAILED CLOSING JOURNAL STRUCTURE ===\n")

	query := `
		SELECT 
			ujl.id as journal_id,
			ujl.entry_number,
			ujl.entry_date,
			jl.line_number,
			a.code as account_code,
			a.name as account_name,
			a.type as account_type,
			jl.debit_amount,
			jl.credit_amount
		FROM unified_journal_ledger ujl
		JOIN unified_journal_lines jl ON jl.journal_id = ujl.id
		JOIN accounts a ON a.id = jl.account_id
		WHERE ujl.source_type = 'CLOSING'
		AND ujl.status = 'POSTED'
		AND ujl.deleted_at IS NULL
		ORDER BY ujl.entry_date, jl.line_number
	`

	var entries []ClosingEntry
	err = db.Raw(query).Scan(&entries).Error
	if err != nil {
		log.Fatal("Query error:", err)
	}

	// Group by journal
	currentJournal := uint(0)
	for _, entry := range entries {
		if entry.JournalID != currentJournal {
			if currentJournal != 0 {
				fmt.Println() // Blank line between journals
			}
			fmt.Printf("=== JOURNAL: %s (ID: %d) ===\n", entry.EntryNumber, entry.JournalID)
			fmt.Printf("Entry Date: %s\n", entry.EntryDate.Format("2006-01-02"))
			fmt.Println("Lines:")
			currentJournal = entry.JournalID
		}

		side := "DEBIT "
		amount := entry.DebitAmount
		if entry.CreditAmount > 0 {
			side = "CREDIT"
			amount = entry.CreditAmount
		}

		fmt.Printf("  Line %d: [%s] %s - %s (%s) = %.2f\n",
			entry.LineNumber, side, entry.AccountCode, entry.AccountName, entry.AccountType, amount)
	}

	// Now test query with date filter
	fmt.Println("\n\n=== TESTING DATE FILTER QUERY ===")
	dates := []string{"2025-12-01", "2026-12-31", "2027-02-02"}
	
	for _, dateStr := range dates {
		fmt.Printf("\n--- As of: %s ---\n", dateStr)
		
		testQuery := `
			SELECT 
				a.code as account_code,
				a.name as account_name,
				COALESCE(SUM(jl.debit_amount), 0) as debit_amount,
				COALESCE(SUM(jl.credit_amount), 0) as credit_amount,
				CASE 
					WHEN UPPER(a.type) IN ('ASSET', 'EXPENSE') THEN 
						COALESCE(SUM(jl.debit_amount), 0) - COALESCE(SUM(jl.credit_amount), 0)
					ELSE 
						COALESCE(SUM(jl.credit_amount), 0) - COALESCE(SUM(jl.debit_amount), 0)
				END as net_balance
			FROM accounts a
			LEFT JOIN unified_journal_lines jl ON jl.account_id = a.id
			LEFT JOIN unified_journal_ledger uj ON uj.id = jl.journal_id 
				AND uj.status = 'POSTED' 
				AND uj.deleted_at IS NULL 
				AND uj.entry_date <= ?
			WHERE a.code = '3201'
			AND a.deleted_at IS NULL
			GROUP BY a.code, a.name, a.type
		`
		
		var result struct {
			AccountCode  string
			AccountName  string
			DebitAmount  float64
			CreditAmount float64
			NetBalance   float64
		}
		
		err = db.Raw(testQuery, dateStr).Scan(&result).Error
		if err != nil {
			log.Printf("Query error: %v", err)
			continue
		}
		
		fmt.Printf("Account: %s - %s\n", result.AccountCode, result.AccountName)
		fmt.Printf("Debit: %.2f | Credit: %.2f | Balance: %.2f\n", 
			result.DebitAmount, result.CreditAmount, result.NetBalance)
	}
}
