package main

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	dsn := "host=localhost user=postgres password=postgres dbname=sistem_akuntansi port=5432 sslmode=disable"

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// 1. Show ALL ssot_journal_entries (not filtered by account) - most recent
	fmt.Println("=== ALL SSOT_JOURNAL_ENTRIES (most recent 10) ===")
	type SSOTEntry struct {
		ID         uint
		SourceType string
		SourceID   *uint64
		Ref        string
		Desc       string
		Status     string
		Created    string
	}
	var allEntries []SSOTEntry
	db.Raw(`SELECT id, source_type, source_id, reference as ref, description as "desc", status, 
	        TO_CHAR(created_at, 'YYYY-MM-DD HH24:MI:SS') as created 
	        FROM ssot_journal_entries WHERE deleted_at IS NULL 
	        ORDER BY created_at DESC LIMIT 10`).Scan(&allEntries)
	fmt.Printf("Total entries found: %d\n", len(allEntries))
	for _, e := range allEntries {
		srcID := uint64(0)
		if e.SourceID != nil {
			srcID = *e.SourceID
		}
		fmt.Printf("  #%d [%s] type=%s srcID=%d status=%s ref=%s | %s\n",
			e.ID, e.Desc, e.SourceType, srcID, e.Status, e.Ref, e.Created)

		// Show lines
		type LineInfo struct {
			AccountID uint64
			Code      string
			Name      string
			Debit     float64
			Credit    float64
		}
		var lines []LineInfo
		db.Raw(`SELECT sjl.account_id, COALESCE(a.code,'') as code, COALESCE(a.name,'') as name,
		        sjl.debit_amount as debit, sjl.credit_amount as credit
		        FROM ssot_journal_lines sjl
		        LEFT JOIN accounts a ON a.id = sjl.account_id
		        WHERE sjl.journal_id = ?`, e.ID).Scan(&lines)
		for _, l := range lines {
			fmt.Printf("      acc=%d (%s %s) D=%.0f C=%.0f\n", l.AccountID, l.Code, l.Name, l.Debit, l.Credit)
		}
	}

	// 2. Show ALL legacy journal entries for account 448 (OCBC)
	fmt.Println("\n=== LEGACY JE FOR ACCOUNT 448 (OCBC) ===")
	type LegacyEntry struct {
		ID      uint
		Code    string
		Ref     string
		RefType string
		Desc    string
		Status  string
		Debit   float64
		Credit  float64
		Created string
	}
	var legacyEntries []LegacyEntry
	db.Raw(`SELECT je.id, je.code, je.reference as ref, je.reference_type as ref_type,
	        je.description as "desc", je.status,
	        jl.debit_amount as debit, jl.credit_amount as credit,
	        TO_CHAR(je.created_at, 'YYYY-MM-DD HH24:MI:SS') as created
	        FROM journal_entries je 
	        JOIN journal_lines jl ON jl.journal_entry_id = je.id
	        WHERE jl.account_id = 448 AND je.deleted_at IS NULL AND jl.deleted_at IS NULL
	        ORDER BY je.created_at DESC LIMIT 10`).Scan(&legacyEntries)
	fmt.Printf("Total: %d\n", len(legacyEntries))
	for _, e := range legacyEntries {
		fmt.Printf("  JE#%d [%s] %s | %s/%s | D=%.0f C=%.0f | %s\n",
			e.ID, e.Code, e.Desc, e.RefType, e.Status, e.Debit, e.Credit, e.Created)
	}

	// 3. Count total tables
	fmt.Println("\n=== TABLE COUNTS ===")
	var totalSSOT, totalLegacy, totalSimple int64
	db.Raw("SELECT COUNT(*) FROM ssot_journal_entries WHERE deleted_at IS NULL").Scan(&totalSSOT)
	db.Raw("SELECT COUNT(*) FROM journal_entries WHERE deleted_at IS NULL").Scan(&totalLegacy)
	db.Raw("SELECT COUNT(*) FROM simple_ssot_journals WHERE deleted_at IS NULL").Scan(&totalSimple)
	fmt.Printf("  ssot_journal_entries: %d\n  journal_entries: %d\n  simple_ssot_journals: %d\n", totalSSOT, totalLegacy, totalSimple)

	// 4. Current balances
	fmt.Println("\n=== CURRENT BALANCES ===")
	var accBal, cbBal float64
	db.Raw("SELECT balance FROM accounts WHERE id = 448 AND deleted_at IS NULL").Scan(&accBal)
	db.Raw("SELECT balance FROM cash_banks WHERE account_id = 448 AND deleted_at IS NULL").Scan(&cbBal)
	fmt.Printf("  accounts.balance (448): %.0f\n  cash_banks.balance:     %.0f\n  discrepancy:            %.0f\n", accBal, cbBal, cbBal-accBal)
}
