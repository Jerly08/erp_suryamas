package main

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := "postgres://postgres:postgres@localhost/sistem_akuntansi?sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	fmt.Println("=== UNDO ACCIDENTAL REVERSAL & FIX DEPRECIATION ===")
	fmt.Println()

	// Step 1: Restore Journal #1 (Capital Injection wrongly soft-deleted)
	fmt.Println("[1] Restoring Journal #1 (Capital Injection - NOT depreciation)...")
	db.Exec("UPDATE unified_journal_ledger SET deleted_at = NULL, status = 'POSTED' WHERE id = 1")
	db.Exec("UPDATE unified_journal_lines SET deleted_at = NULL WHERE journal_id = 1")
	// Restore balance impact (Bank +100M, Modal -100M for capital injection)
	db.Exec("UPDATE accounts SET balance = balance + 100000000 WHERE code = '1101-005' AND deleted_at IS NULL") // Bank OCBC
	db.Exec("UPDATE accounts SET balance = balance - 100000000 WHERE code = '3101' AND deleted_at IS NULL") // Modal Disetor
	fmt.Println("   Journal #1 restored ✓")
	fmt.Println("   Bank OCBC balance restored ✓")
	fmt.Println("   Modal Disetor balance restored ✓")

	// Step 2: Find ACTUAL depreciation journals (search more carefully)
	fmt.Println()
	fmt.Println("[2] Searching for actual depreciation journals...")
	type JournalInfo struct {
		ID          uint
		EntryNumber string
		SourceType  string
		Reference   string
		Status      string
		TotalDebit  float64
	}

	// Search by source_type = DEPRECIATION specifically
	var depJournals []JournalInfo
	db.Raw(`
		SELECT id, entry_number, source_type, reference, status, total_debit
		FROM unified_journal_ledger
		WHERE source_type = 'DEPRECIATION'
		ORDER BY id
	`).Scan(&depJournals)
	fmt.Printf("   Found %d with source_type='DEPRECIATION'\n", len(depJournals))

	// Also search by reference pattern AS-*-DEP-*
	var depJournals2 []JournalInfo
	db.Raw(`
		SELECT id, entry_number, source_type, reference, status, total_debit
		FROM unified_journal_ledger
		WHERE reference LIKE 'AS-%-DEP-%'
		ORDER BY id
	`).Scan(&depJournals2)
	fmt.Printf("   Found %d with reference LIKE 'AS-%%DEP-%%'\n", len(depJournals2))

	// Also search by entry_number pattern DEP-1-*
	var depJournals3 []JournalInfo
	db.Raw(`
		SELECT id, entry_number, source_type, reference, status, total_debit
		FROM unified_journal_ledger
		WHERE entry_number LIKE 'DEP-1-%'
		ORDER BY id
	`).Scan(&depJournals3)
	fmt.Printf("   Found %d with entry_number LIKE 'DEP-1-%%'\n", len(depJournals3))

	// Combine all findings
	allDep := map[uint]JournalInfo{}
	for _, j := range depJournals { allDep[j.ID] = j }
	for _, j := range depJournals2 { allDep[j.ID] = j }
	for _, j := range depJournals3 { allDep[j.ID] = j }

	fmt.Printf("\n   Total unique depreciation journals: %d\n", len(allDep))
	for _, j := range allDep {
		fmt.Printf("   ID:%d | type:%s | ref:%s | entry:%s | status:%s | amount:%.0f\n",
			j.ID, j.SourceType, j.Reference, j.EntryNumber, j.Status, j.TotalDebit)

		// Show lines
		type LineInfo struct {
			AccountID    uint64
			Code         string
			Name         string
			DebitAmount  float64
			CreditAmount float64
		}
		var lines []LineInfo
		db.Raw(`
			SELECT ujl.account_id, a.code, a.name, ujl.debit_amount, ujl.credit_amount
			FROM unified_journal_lines ujl
			JOIN accounts a ON a.id = ujl.account_id
			WHERE ujl.journal_id = ?
		`, j.ID).Scan(&lines)
		for _, l := range lines {
			marker := ""
			if l.DebitAmount > 0 && (l.Code == "1202-003" || l.Code == "1202-004" || l.Code == "1202-005") {
				marker = " *** WRONG DEBIT ***"
			}
			fmt.Printf("     %s (%s): Dr=%.0f Cr=%.0f%s\n", l.Code, l.Name, l.DebitAmount, l.CreditAmount, marker)
		}

		// Delete wrong journal and reverse balances
		if j.Status != "REVERSED" {
			for _, l := range lines {
				if l.DebitAmount > 0 {
					db.Exec("UPDATE accounts SET balance = balance - ? WHERE id = ? AND deleted_at IS NULL", l.DebitAmount, l.AccountID)
				}
				if l.CreditAmount > 0 {
					db.Exec("UPDATE accounts SET balance = balance + ? WHERE id = ? AND deleted_at IS NULL", l.CreditAmount, l.AccountID)
				}
			}
			db.Exec("UPDATE unified_journal_lines SET deleted_at = NOW() WHERE journal_id = ?", j.ID)
			db.Exec("UPDATE unified_journal_ledger SET deleted_at = NOW(), status = 'REVERSED' WHERE id = ?", j.ID)
			fmt.Printf("   -> Journal #%d reversed and deleted ✓\n", j.ID)
		}
	}

	// Step 3: Make sure asset data is correct
	fmt.Println()
	fmt.Println("[3] Ensuring asset data is correct...")
	db.Exec("UPDATE assets SET depreciation_account_id = 64, accumulated_depreciation = 0 WHERE id = 1")
	fmt.Println("   depreciation_account_id = 64 (6103 BEBAN PENYUSUTAN) ✓")
	fmt.Println("   accumulated_depreciation = 0 ✓")

	// Step 4: Show final account balances
	fmt.Println()
	fmt.Println("[4] Final account balances:")
	type AccCheck struct {
		ID      uint
		Code    string
		Name    string
		Balance float64
	}
	var accounts []AccCheck
	db.Raw(`
		SELECT id, code, name, balance FROM accounts 
		WHERE code IN ('1101-005','3101','1202-003','1202-004','6103') 
		AND deleted_at IS NULL 
		ORDER BY code
	`).Scan(&accounts)
	for _, a := range accounts {
		fmt.Printf("   %s (%s): Balance = %.0f\n", a.Code, a.Name, a.Balance)
	}

	fmt.Println()
	fmt.Println("=== DONE ===")
	fmt.Println("Now run: go run scripts/check_depreciation.go --run")
	fmt.Println("This will create CORRECT journal entries using 6103 - BEBAN PENYUSUTAN")
}
