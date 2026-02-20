package main

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type JournalEntry struct {
	ID          uint      `gorm:"column:id"`
	EntryNumber string    `gorm:"column:entry_number"`
	SourceCode  string    `gorm:"column:source_code"`
	SourceType  string    `gorm:"column:source_type"`
	EntryDate   time.Time `gorm:"column:entry_date"`
	Description string    `gorm:"column:description"`
	Status      string    `gorm:"column:status"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

type JournalLine struct {
	JournalID    uint      `gorm:"column:journal_id"`
	EntryNumber  string    `gorm:"column:entry_number"`
	SourceCode   string    `gorm:"column:source_code"`
	EntryDate    time.Time `gorm:"column:entry_date"`
	AccountCode  string    `gorm:"column:account_code"`
	AccountName  string    `gorm:"column:account_name"`
	DebitAmount  float64   `gorm:"column:debit_amount"`
	CreditAmount float64   `gorm:"column:credit_amount"`
}

func main() {
	// Connect to database
	dsn := "host=localhost user=postgres password=postgres dbname=sistem_akuntansi port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	fmt.Println("=== CLOSING JOURNAL ENTRIES ===")
	var closingEntries []JournalEntry
	err = db.Table("unified_journal_ledger").
		Where("source_type = 'CLOSING' OR source_code LIKE 'CLO-%' OR entry_number LIKE 'CLO-%'").
		Order("entry_date").
		Find(&closingEntries).Error
	if err != nil {
		log.Fatal("Query error:", err)
	}

	for _, entry := range closingEntries {
		fmt.Printf("ID: %d | Entry Number: %s | Source Code: %s | Type: %s | Entry Date: %s | Description: %s\n",
			entry.ID, entry.EntryNumber, entry.SourceCode, entry.SourceType, entry.EntryDate.Format("2006-01-02"), entry.Description)
	}

	fmt.Println("\n=== JOURNAL LINES FOR ACCOUNT 3201 (LABA DITAHAN) ===")
	var labaLines []JournalLine
	query := `
		SELECT 
			ujl.id as journal_id,
			ujl.entry_number,
			ujl.source_code,
			ujl.entry_date,
			a.code as account_code,
			a.name as account_name,
			jl.debit_amount,
			jl.credit_amount
		FROM unified_journal_ledger ujl
		JOIN unified_journal_lines jl ON jl.journal_id = ujl.id
		JOIN accounts a ON a.id = jl.account_id
		WHERE a.code = '3201'
		AND ujl.status = 'POSTED'
		AND ujl.deleted_at IS NULL
		ORDER BY ujl.entry_date
	`
	err = db.Raw(query).Scan(&labaLines).Error
	if err != nil {
		log.Fatal("Query error:", err)
	}

	totalDebit := 0.0
	totalCredit := 0.0
	for _, line := range labaLines {
		fmt.Printf("Entry: %s | Source: %s | Entry Date: %s | Debit: %.2f | Credit: %.2f\n",
			line.EntryNumber, line.SourceCode, line.EntryDate.Format("2006-01-02"), line.DebitAmount, line.CreditAmount)
		totalDebit += line.DebitAmount
		totalCredit += line.CreditAmount
	}
	
	fmt.Printf("\nTotal Debit: %.2f | Total Credit: %.2f | Net Balance: %.2f\n",
		totalDebit, totalCredit, totalCredit-totalDebit)

	fmt.Println("\n=== BALANCE UP TO SPECIFIC DATES ===")
	dates := []string{"2025-12-01", "2026-12-31", "2027-02-02"}
	for _, dateStr := range dates {
		var result struct {
			TotalDebit  float64
			TotalCredit float64
		}
		
		query := `
			SELECT 
				COALESCE(SUM(jl.debit_amount), 0) as total_debit,
				COALESCE(SUM(jl.credit_amount), 0) as total_credit
			FROM unified_journal_ledger ujl
			JOIN unified_journal_lines jl ON jl.journal_id = ujl.id
			JOIN accounts a ON a.id = jl.account_id
			WHERE a.code = '3201'
			AND ujl.status = 'POSTED'
			AND ujl.deleted_at IS NULL
			AND ujl.entry_date <= ?
		`
		err = db.Raw(query, dateStr).Scan(&result).Error
		if err != nil {
			log.Printf("Query error for %s: %v", dateStr, err)
			continue
		}
		
		balance := result.TotalCredit - result.TotalDebit
		fmt.Printf("Up to %s: Debit: %.2f | Credit: %.2f | Balance: %.2f\n",
			dateStr, result.TotalDebit, result.TotalCredit, balance)
	}
}
