package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type RevenueCheck struct {
	AccountCode         string  `gorm:"column:account_code"`
	AccountName         string  `gorm:"column:account_name"`
	TotalLines          int     `gorm:"column:total_lines"`
	UniqueJournals      int     `gorm:"column:unique_journals"`
	TotalCredit         float64 `gorm:"column:total_credit"`
	TotalDebit          float64 `gorm:"column:total_debit"`
	NetRevenue          float64 `gorm:"column:net_revenue"`
}

type JournalDetail struct {
	JournalID      int     `gorm:"column:journal_id"`
	JournalNumber  string  `gorm:"column:journal_number"`
	EntryDate      string  `gorm:"column:entry_date"`
	SourceType     string  `gorm:"column:source_type"`
	SourceID       int     `gorm:"column:source_id"`
	CreditAmount   float64 `gorm:"column:credit_amount"`
	Description    string  `gorm:"column:description"`
}

type SalesDuplicate struct {
	SalesID        int    `gorm:"column:sales_id"`
	InvoiceNumber  string `gorm:"column:invoice_number"`
	TotalAmount    float64 `gorm:"column:total_amount"`
	JournalCount   int    `gorm:"column:journal_count"`
	JournalNumbers string `gorm:"column:journal_numbers"`
}

func main() {
	// Load database connection
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Jakarta",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASSWORD", "postgres"),
		getEnv("DB_NAME", "sistem_akuntansi"),
		getEnv("DB_PORT", "5432"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	fmt.Println("=" + string(make([]byte, 60)))
	fmt.Println("REVENUE DUPLICATION ANALYSIS")
	fmt.Println("=" + string(make([]byte, 60)))
	fmt.Println()

	// Query 1: Account 4101 Summary
	fmt.Println("1. ACCOUNT 4101 SUMMARY")
	fmt.Println("-" + string(make([]byte, 60)))
	var summary []RevenueCheck
	query1 := `
		SELECT 
			a.code as account_code,
			a.name as account_name,
			COUNT(ujl.id) as total_lines,
			COUNT(DISTINCT ujl.journal_id) as unique_journals,
			COALESCE(SUM(ujl.credit_amount), 0) as total_credit,
			COALESCE(SUM(ujl.debit_amount), 0) as total_debit,
			COALESCE(SUM(ujl.credit_amount), 0) - COALESCE(SUM(ujl.debit_amount), 0) as net_revenue
		FROM accounts a
		LEFT JOIN unified_journal_lines ujl ON ujl.account_id = a.id
		LEFT JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id 
			AND uje.status = 'POSTED' 
			AND uje.deleted_at IS NULL
			AND uje.entry_date >= '2025-01-01' 
			AND uje.entry_date <= '2025-12-31'
		WHERE a.code = '4101'
		GROUP BY a.code, a.name
	`
	db.Raw(query1).Scan(&summary)
	printJSON("Account Summary", summary)

	// Query 2: Check for duplicate sales journals
	fmt.Println("\n2. SALES WITH MULTIPLE JOURNAL ENTRIES (DUPLICATES)")
	fmt.Println("-" + string(make([]byte, 60)))
	var duplicates []SalesDuplicate
	query2 := `
		SELECT 
			s.id as sales_id,
			s.invoice_number,
			s.total_amount,
			COUNT(DISTINCT uje.id) as journal_count,
			STRING_AGG(uje.journal_number, ', ' ORDER BY uje.id) as journal_numbers
		FROM sales s
		INNER JOIN unified_journal_ledger uje ON uje.source_type = 'SALES' 
			AND uje.source_id = s.id 
			AND uje.status = 'POSTED'
			AND uje.deleted_at IS NULL
		WHERE s.created_at >= '2025-01-01' 
		GROUP BY s.id, s.invoice_number, s.total_amount
		HAVING COUNT(DISTINCT uje.id) > 1
	`
	db.Raw(query2).Scan(&duplicates)
	if len(duplicates) > 0 {
		printJSON("Duplicate Sales Journals", duplicates)
		fmt.Println("\n⚠️  WARNING: Found sales with multiple journal entries!")
		fmt.Println("This is the source of revenue duplication.")
	} else {
		fmt.Println("✅ No duplicate sales journals found.")
	}

	// Query 3: Journal details for account 4101
	fmt.Println("\n3. ALL JOURNAL ENTRIES FOR ACCOUNT 4101")
	fmt.Println("-" + string(make([]byte, 60)))
	var details []JournalDetail
	query3 := `
		SELECT 
			uje.id as journal_id,
			uje.journal_number,
			uje.entry_date,
			uje.source_type,
			uje.source_id,
			ujl.credit_amount,
			ujl.description
		FROM unified_journal_ledger uje
		INNER JOIN unified_journal_lines ujl ON ujl.journal_id = uje.id
		INNER JOIN accounts a ON a.id = ujl.account_id
		WHERE a.code = '4101'
		  AND uje.entry_date >= '2025-01-01' 
		  AND uje.entry_date <= '2025-12-31'
		  AND uje.status = 'POSTED'
		  AND uje.deleted_at IS NULL
		ORDER BY uje.entry_date, uje.id
	`
	db.Raw(query3).Scan(&details)
	printJSON("Journal Details", details)

	fmt.Println("\n" + string(make([]byte, 60)))
	fmt.Println("ANALYSIS COMPLETE")
	fmt.Println(string(make([]byte, 60)))
}

func printJSON(title string, data interface{}) {
	jsonData, _ := json.MarshalIndent(data, "", "  ")
	fmt.Printf("%s:\n%s\n", title, string(jsonData))
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

