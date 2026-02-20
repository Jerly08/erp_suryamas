package main

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type RevenueAnalysis struct {
	Source          string  `json:"source"`
	AccountCode     string  `json:"account_code"`
	AccountName     string  `json:"account_name"`
	TotalCredit     float64 `json:"total_credit"`
	TotalDebit      float64 `json:"total_debit"`
	NetAmount       float64 `json:"net_amount"`
	LineCount       int     `json:"line_count"`
	JournalCount    int     `json:"journal_count"`
}

type JournalDetail struct {
	JournalID       uint    `json:"journal_id"`
	EntryDate       string  `json:"entry_date"`
	Description     string  `json:"description"`
	SourceType      string  `json:"source_type"`
	SourceID        uint    `json:"source_id"`
	ReferenceNumber string  `json:"reference_number"`
	Status          string  `json:"status"`
	AccountID       uint    `json:"account_id"`
	AccountCode     string  `json:"account_code"`
	AccountName     string  `json:"account_name"`
	DebitAmount     float64 `json:"debit_amount"`
	CreditAmount    float64 `json:"credit_amount"`
}

type AccountBalance struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Balance  float64 `json:"balance"`
	IsHeader bool    `json:"is_header"`
}

func main() {
	fmt.Println("=== REVENUE DUPLICATION INVESTIGATION ===")
	fmt.Println("Expected: Rp 10,000,000 (from Chart of Accounts)")
	fmt.Println("Actual: Rp 20,000,000 (from P&L Report)")
	fmt.Println("Investigating root cause...\n")

	// Database connection
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		// Default DSN - adjust as needed
		dsn = "root:root@tcp(localhost:3306)/accounting_db?charset=utf8mb4&parseTime=True&loc=Local"
		fmt.Printf("[INFO] Using default DSN. Set DB_DSN environment variable to override.\n")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Get raw DB connection
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Failed to get database connection:", err)
	}
	defer sqlDB.Close()

	fmt.Println("=== STEP 1: Account Balance Check ===")
	checkAccountBalance(db)

	fmt.Println("\n=== STEP 2: Unified Journal Analysis (SSOT) ===")
	checkUnifiedJournals(db)

	fmt.Println("\n=== STEP 3: Legacy Journal Analysis ===")
	checkLegacyJournals(db)

	fmt.Println("\n=== STEP 4: Detailed Journal Entries for 4101 ===")
	checkJournalDetails(db)

	fmt.Println("\n=== STEP 5: Check for Duplicate Journals ===")
	checkDuplicateJournals(db)

	fmt.Println("\n=== STEP 6: Combined Systems Analysis ===")
	checkCombinedSystems(db)

	fmt.Println("\n=== FINAL ANALYSIS ===")
	provideDiagnosis()
}

func checkAccountBalance(db *gorm.DB) {
	var accounts []AccountBalance
	query := `SELECT code, name, type, balance, COALESCE(is_header, false) as is_header 
	          FROM accounts 
	          WHERE code LIKE '4%' 
	          ORDER BY code`
	
	if err := db.Raw(query).Scan(&accounts).Error; err != nil {
		log.Printf("[ERROR] Failed to fetch accounts: %v", err)
		return
	}

	totalBalance := 0.0
	fmt.Println("Revenue Accounts from Chart of Accounts:")
	for _, acc := range accounts {
		if !acc.IsHeader {
			fmt.Printf("  %s - %s: Rp %.2f\n", acc.Code, acc.Name, acc.Balance)
			totalBalance += acc.Balance
		}
	}
	fmt.Printf("\n[TOTAL ACCOUNT BALANCE]: Rp %.2f\n", totalBalance)
}

func checkUnifiedJournals(db *gorm.DB) {
	var results []RevenueAnalysis
	query := `
		SELECT 
			'Unified Journal' as source,
			a.code as account_code,
			a.name as account_name,
			COALESCE(SUM(ujl.credit_amount), 0) as total_credit,
			COALESCE(SUM(ujl.debit_amount), 0) as total_debit,
			COALESCE(SUM(ujl.credit_amount - ujl.debit_amount), 0) as net_amount,
			COUNT(*) as line_count,
			COUNT(DISTINCT ujl.journal_id) as journal_count
		FROM accounts a
		LEFT JOIN unified_journal_lines ujl ON ujl.account_id = a.id
		LEFT JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id 
			AND uje.status = 'POSTED' 
			AND uje.deleted_at IS NULL
		WHERE a.code LIKE '4%'
			AND uje.entry_date >= '2025-01-01' 
			AND uje.entry_date <= '2025-12-31'
			AND COALESCE(a.is_header, false) = false
		GROUP BY a.id, a.code, a.name
		HAVING (COALESCE(SUM(ujl.debit_amount), 0) != 0 OR COALESCE(SUM(ujl.credit_amount), 0) != 0)
		ORDER BY a.code`

	if err := db.Raw(query).Scan(&results).Error; err != nil {
		log.Printf("[ERROR] Failed to fetch unified journals: %v", err)
		return
	}

	totalRevenue := 0.0
	fmt.Println("Revenue from Unified Journal System (SSOT):")
	for _, r := range results {
		fmt.Printf("  %s - %s:\n", r.AccountCode, r.AccountName)
		fmt.Printf("    Credit: Rp %.2f | Debit: Rp %.2f | Net: Rp %.2f\n", r.TotalCredit, r.TotalDebit, r.NetAmount)
		fmt.Printf("    Lines: %d | Journals: %d\n", r.LineCount, r.JournalCount)
		totalRevenue += r.NetAmount
	}
	fmt.Printf("\n[TOTAL FROM UNIFIED JOURNALS]: Rp %.2f\n", totalRevenue)
}

func checkLegacyJournals(db *gorm.DB) {
	var results []RevenueAnalysis
	query := `
		SELECT 
			'Legacy Journal' as source,
			a.code as account_code,
			a.name as account_name,
			COALESCE(SUM(jl.credit_amount), 0) as total_credit,
			COALESCE(SUM(jl.debit_amount), 0) as total_debit,
			COALESCE(SUM(jl.credit_amount - jl.debit_amount), 0) as net_amount,
			COUNT(*) as line_count
		FROM accounts a
		LEFT JOIN journal_lines jl ON jl.account_id = a.id
		LEFT JOIN journal_entries je ON je.id = jl.journal_entry_id 
			AND je.status = 'POSTED' 
			AND je.deleted_at IS NULL
		WHERE a.code LIKE '4%'
			AND je.entry_date >= '2025-01-01' 
			AND je.entry_date <= '2025-12-31'
		GROUP BY a.id, a.code, a.name
		HAVING (COALESCE(SUM(jl.debit_amount), 0) != 0 OR COALESCE(SUM(jl.credit_amount), 0) != 0)
		ORDER BY a.code`

	if err := db.Raw(query).Scan(&results).Error; err != nil {
		log.Printf("[ERROR] Failed to fetch legacy journals: %v", err)
		return
	}

	totalRevenue := 0.0
	fmt.Println("Revenue from Legacy Journal System:")
	if len(results) == 0 {
		fmt.Println("  [NO DATA] No legacy journal entries found")
	} else {
		for _, r := range results {
			fmt.Printf("  %s - %s:\n", r.AccountCode, r.AccountName)
			fmt.Printf("    Credit: Rp %.2f | Debit: Rp %.2f | Net: Rp %.2f\n", r.TotalCredit, r.TotalDebit, r.NetAmount)
			fmt.Printf("    Lines: %d\n", r.LineCount)
			totalRevenue += r.NetAmount
		}
	}
	fmt.Printf("\n[TOTAL FROM LEGACY JOURNALS]: Rp %.2f\n", totalRevenue)
}

func checkJournalDetails(db *gorm.DB) {
	var details []JournalDetail
	query := `
		SELECT 
			uje.id as journal_id,
			DATE(uje.entry_date) as entry_date,
			uje.description,
			uje.source_type,
			uje.source_id,
			uje.reference_number,
			uje.status,
			ujl.account_id,
			a.code as account_code,
			a.name as account_name,
			ujl.debit_amount,
			ujl.credit_amount
		FROM unified_journal_ledger uje
		INNER JOIN unified_journal_lines ujl ON uje.id = ujl.journal_id
		INNER JOIN accounts a ON ujl.account_id = a.id
		WHERE a.code = '4101'
			AND uje.entry_date >= '2025-01-01' 
			AND uje.entry_date <= '2025-12-31'
			AND uje.status = 'POSTED'
			AND uje.deleted_at IS NULL
		ORDER BY uje.entry_date, uje.id, ujl.id`

	if err := db.Raw(query).Scan(&details).Error; err != nil {
		log.Printf("[ERROR] Failed to fetch journal details: %v", err)
		return
	}

	fmt.Printf("Journal Entries for Account 4101 (PENDAPATAN PENJUALAN):\n")
	fmt.Printf("Total Entries: %d\n\n", len(details))

	totalRevenue := 0.0
	for i, d := range details {
		fmt.Printf("[%d] Journal ID: %d | Date: %s\n", i+1, d.JournalID, d.EntryDate)
		fmt.Printf("    Source: %s #%d | Ref: %s\n", d.SourceType, d.SourceID, d.ReferenceNumber)
		fmt.Printf("    Description: %s\n", d.Description)
		fmt.Printf("    Account: %s - %s\n", d.AccountCode, d.AccountName)
		fmt.Printf("    Debit: Rp %.2f | Credit: Rp %.2f\n", d.DebitAmount, d.CreditAmount)
		netAmount := d.CreditAmount - d.DebitAmount
		totalRevenue += netAmount
		fmt.Println()
	}
	fmt.Printf("[TOTAL REVENUE FROM DETAILED ENTRIES]: Rp %.2f\n", totalRevenue)
}

func checkDuplicateJournals(db *gorm.DB) {
	type DuplicateCheck struct {
		SourceType       string  `json:"source_type"`
		SourceID         uint    `json:"source_id"`
		ReferenceNumber  string  `json:"reference_number"`
		JournalCount     int     `json:"journal_count"`
		JournalIDs       string  `json:"journal_ids"`
		TotalRevenue4101 float64 `json:"total_revenue_4101"`
	}

	var duplicates []DuplicateCheck
	query := `
		SELECT 
			uje.source_type,
			uje.source_id,
			uje.reference_number,
			COUNT(DISTINCT uje.id) as journal_count,
			GROUP_CONCAT(DISTINCT uje.id) as journal_ids,
			SUM(CASE WHEN ujl.account_code = '4101' THEN ujl.credit_amount ELSE 0 END) as total_revenue_4101
		FROM unified_journal_ledger uje
		INNER JOIN unified_journal_lines ujl ON uje.id = ujl.journal_id
		WHERE uje.source_type IN ('SALE', 'PAYMENT', 'MANUAL')
			AND uje.entry_date >= '2025-01-01' 
			AND uje.entry_date <= '2025-12-31'
			AND uje.status = 'POSTED'
			AND uje.deleted_at IS NULL
		GROUP BY uje.source_type, uje.source_id, uje.reference_number
		HAVING COUNT(DISTINCT uje.id) > 1
		ORDER BY journal_count DESC`

	if err := db.Raw(query).Scan(&duplicates).Error; err != nil {
		log.Printf("[ERROR] Failed to check duplicates: %v", err)
		return
	}

	if len(duplicates) == 0 {
		fmt.Println("[OK] No duplicate journals found")
	} else {
		fmt.Printf("[WARNING] Found %d potential duplicate journal groups:\n", len(duplicates))
		for i, dup := range duplicates {
			fmt.Printf("\n[%d] Source: %s #%d | Ref: %s\n", i+1, dup.SourceType, dup.SourceID, dup.ReferenceNumber)
			fmt.Printf("    Journal Count: %d (SHOULD BE 1!)\n", dup.JournalCount)
			fmt.Printf("    Journal IDs: %s\n", dup.JournalIDs)
			fmt.Printf("    Total Revenue (4101): Rp %.2f\n", dup.TotalRevenue4101)
		}
	}
}

func checkCombinedSystems(db *gorm.DB) {
	type SystemTotal struct {
		Source       string  `json:"source"`
		System       string  `json:"system"`
		TotalRevenue float64 `json:"total_revenue"`
	}

	var totals []SystemTotal
	query := `
		SELECT 
			'Combined' as source,
			'Unified Journal (SSOT)' as system,
			COALESCE(SUM(ujl.credit_amount - ujl.debit_amount), 0) as total_revenue
		FROM unified_journal_lines ujl
		INNER JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
		INNER JOIN accounts a ON ujl.account_id = a.id
		WHERE a.code LIKE '4%'
			AND uje.entry_date >= '2025-01-01' 
			AND uje.entry_date <= '2025-12-31'
			AND uje.status = 'POSTED'
			AND uje.deleted_at IS NULL

		UNION ALL

		SELECT 
			'Combined' as source,
			'Legacy Journal' as system,
			COALESCE(SUM(jl.credit_amount - jl.debit_amount), 0) as total_revenue
		FROM journal_lines jl
		INNER JOIN journal_entries je ON je.id = jl.journal_entry_id
		INNER JOIN accounts a ON jl.account_id = a.id
		WHERE a.code LIKE '4%'
			AND je.entry_date >= '2025-01-01' 
			AND je.entry_date <= '2025-12-31'
			AND je.status = 'POSTED'
			AND je.deleted_at IS NULL`

	if err := db.Raw(query).Scan(&totals).Error; err != nil {
		log.Printf("[ERROR] Failed to check combined systems: %v", err)
		return
	}

	grandTotal := 0.0
	fmt.Println("Combined System Analysis:")
	for _, t := range totals {
		fmt.Printf("  %s: Rp %.2f\n", t.System, t.TotalRevenue)
		grandTotal += t.TotalRevenue
	}
	fmt.Printf("\n[GRAND TOTAL FROM BOTH SYSTEMS]: Rp %.2f\n", grandTotal)

	if grandTotal == 20000000 && totals[0].TotalRevenue == 10000000 && totals[1].TotalRevenue == 10000000 {
		fmt.Println("\n[DIAGNOSIS] BOTH SYSTEMS ARE ACTIVE!")
		fmt.Println("Root Cause: Backend is counting revenue from BOTH Unified and Legacy journals")
		fmt.Println("This explains the 100% duplication (10M + 10M = 20M)")
	}
}

func provideDiagnosis() {
	fmt.Println("\n╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║           REVENUE DUPLICATION DIAGNOSIS                  ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Println("\nBased on the analysis above, check for these scenarios:")
	fmt.Println("")
	fmt.Println("1. BOTH SYSTEMS ACTIVE (Most Likely)")
	fmt.Println("   - If Unified + Legacy totals = 20M")
	fmt.Println("   - Fix: Backend should use ONLY ONE system")
	fmt.Println("   - Action: Check fallback logic in ssot_profit_loss_service.go")
	fmt.Println("")
	fmt.Println("2. DUPLICATE JOURNAL ENTRIES")
	fmt.Println("   - If duplicate journals found with same source_id")
	fmt.Println("   - Fix: Delete or cancel duplicate journal entries")
	fmt.Println("   - Action: Review journal creation logic")
	fmt.Println("")
	fmt.Println("3. INCORRECT GROUPING")
	fmt.Println("   - If same account appears multiple times in analysis")
	fmt.Println("   - Fix: Update GROUP BY clause in backend query")
	fmt.Println("   - Action: Modify ssot_profit_loss_service.go query")
	fmt.Println("")
	fmt.Println("4. ACCOUNT NAME VARIATIONS")
	fmt.Println("   - If account_name differs in journal lines vs accounts")
	fmt.Println("   - Fix: Standardize account names or group by ID only")
	fmt.Println("   - Action: Update data or modify grouping logic")
	fmt.Println("")
	fmt.Println("To fix, run the appropriate SQL from investigate_revenue_duplication.sql")
	fmt.Println("Then restart backend and regenerate P&L report.")
}

