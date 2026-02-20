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

	fmt.Println("=== COA Balance Discrepancy Diagnosis & Fix ===")
	fmt.Println()

	// 1. Show all cash_banks with their linked accounts and compare balances
	fmt.Println("📊 Step 1: Compare cash_banks.balance vs accounts.balance")
	fmt.Println("─────────────────────────────────────────────────────────")

	type CashBankComparison struct {
		CashBankID   uint
		CashBankName string
		CashBankBal  float64
		AccountID    uint
		AccountCode  string
		AccountName  string
		AccountBal   float64
		Discrepancy  float64
	}
	var comparisons []CashBankComparison
	db.Raw(`
		SELECT 
			cb.id as cash_bank_id,
			cb.name as cash_bank_name,
			cb.balance as cash_bank_bal,
			a.id as account_id,
			a.code as account_code,
			a.name as account_name,
			a.balance as account_bal,
			(cb.balance - a.balance) as discrepancy
		FROM cash_banks cb
		JOIN accounts a ON cb.account_id = a.id
		WHERE cb.deleted_at IS NULL AND a.deleted_at IS NULL
		ORDER BY ABS(cb.balance - a.balance) DESC
	`).Scan(&comparisons)

	hasDiscrepancy := false
	for _, c := range comparisons {
		status := "✅"
		if c.Discrepancy != 0 {
			status = "❌"
			hasDiscrepancy = true
		}
		fmt.Printf("%s CashBank '%s' (ID=%d): balance=%.2f | Account '%s' [%s] (ID=%d): balance=%.2f | Diff=%.2f\n",
			status, c.CashBankName, c.CashBankID, c.CashBankBal,
			c.AccountName, c.AccountCode, c.AccountID, c.AccountBal, c.Discrepancy)
	}

	if !hasDiscrepancy {
		fmt.Println("\n✅ All balances are in sync! No fix needed.")
		return
	}

	fmt.Println()

	// 2. Count how many duplicate SSOT journal entries exist for payments
	fmt.Println("📊 Step 2: Check for duplicate SSOT payment journal entries")
	fmt.Println("───────────────────────────────────────────────────────────")

	type DuplicateCheck struct {
		SourceType string
		Count      int64
	}
	var dupChecks []DuplicateCheck
	db.Raw(`
		SELECT source_type, COUNT(*) as count 
		FROM ssot_journal_entries 
		WHERE deleted_at IS NULL 
		AND source_type LIKE '%PAYMENT%'
		GROUP BY source_type
		ORDER BY count DESC
	`).Scan(&dupChecks)

	for _, d := range dupChecks {
		fmt.Printf("  Source Type: %s → %d entries\n", d.SourceType, d.Count)
	}

	// 3. Check legacy journal entries for payments
	fmt.Println()
	fmt.Println("📊 Step 3: Check legacy journal entries for payments")
	fmt.Println("────────────────────────────────────────────────────")

	type LegacyJournalCheck struct {
		RefType string
		Count   int64
	}
	var legacyChecks []LegacyJournalCheck
	db.Raw(`
		SELECT reference_type as ref_type, COUNT(*) as count
		FROM journal_entries
		WHERE deleted_at IS NULL
		AND (reference_type LIKE '%PAYMENT%' OR reference_type LIKE '%payment%')
		GROUP BY reference_type
		ORDER BY count DESC
	`).Scan(&legacyChecks)

	for _, l := range legacyChecks {
		fmt.Printf("  Ref Type: %s → %d entries\n", l.RefType, l.Count)
	}

	// 4. FIX: Set accounts.balance = cash_banks.balance for all linked accounts
	fmt.Println()
	fmt.Println("🔧 Step 4: Fixing COA balances to match cash_banks balances")
	fmt.Println("────────────────────────────────────────────────────────────")

	for _, c := range comparisons {
		if c.Discrepancy == 0 {
			continue
		}

		fmt.Printf("  Fixing Account '%s' [%s] (ID=%d): %.2f → %.2f (adjusting by %.2f)\n",
			c.AccountName, c.AccountCode, c.AccountID, c.AccountBal, c.CashBankBal, c.Discrepancy)

		// Set accounts.balance = cash_banks.balance (cash_banks is the source of truth)
		result := db.Exec(`
			UPDATE accounts 
			SET balance = ?, updated_at = CURRENT_TIMESTAMP 
			WHERE id = ?
		`, c.CashBankBal, c.AccountID)

		if result.Error != nil {
			fmt.Printf("    ❌ Error: %v\n", result.Error)
		} else {
			fmt.Printf("    ✅ Fixed! Rows affected: %d\n", result.RowsAffected)
		}
	}

	// 5. Verify fix
	fmt.Println()
	fmt.Println("📊 Step 5: Verification after fix")
	fmt.Println("──────────────────────────────────")

	var verifications []CashBankComparison
	db.Raw(`
		SELECT 
			cb.id as cash_bank_id,
			cb.name as cash_bank_name,
			cb.balance as cash_bank_bal,
			a.id as account_id,
			a.code as account_code,
			a.name as account_name,
			a.balance as account_bal,
			(cb.balance - a.balance) as discrepancy
		FROM cash_banks cb
		JOIN accounts a ON cb.account_id = a.id
		WHERE cb.deleted_at IS NULL AND a.deleted_at IS NULL
		ORDER BY cb.id
	`).Scan(&verifications)

	allFixed := true
	for _, v := range verifications {
		status := "✅"
		if v.Discrepancy != 0 {
			status = "❌"
			allFixed = false
		}
		fmt.Printf("  %s CashBank '%s': %.2f | Account '%s' [%s]: %.2f | Diff=%.2f\n",
			status, v.CashBankName, v.CashBankBal, v.AccountName, v.AccountCode, v.AccountBal, v.Discrepancy)
	}

	fmt.Println()
	if allFixed {
		fmt.Println("✅ All COA balances have been corrected to match cash_banks balances!")
	} else {
		fmt.Println("⚠️ Some discrepancies remain - please check manually")
	}

	// 6. Also check and fix AP (Accounts Payable) balance
	fmt.Println()
	fmt.Println("📊 Step 6: Check Accounts Payable (Utang Usaha) balance")
	fmt.Println("───────────────────────────────────────────────────────")

	type APInfo struct {
		ID      uint
		Code    string
		Name    string
		Balance float64
	}
	var apAccounts []APInfo
	db.Raw(`
		SELECT id, code, name, balance 
		FROM accounts 
		WHERE deleted_at IS NULL 
		AND (code = '2101' OR LOWER(name) LIKE '%utang%usaha%')
		ORDER BY code
	`).Scan(&apAccounts)

	for _, ap := range apAccounts {
		fmt.Printf("  Account '%s' [%s] (ID=%d): Balance = %.2f\n", ap.Name, ap.Code, ap.ID, ap.Balance)
	}

	// Calculate what AP balance SHOULD be based on outstanding purchases
	type OutstandingInfo struct {
		TotalOutstanding float64
	}
	var outstanding OutstandingInfo
	db.Raw(`
		SELECT COALESCE(SUM(outstanding_amount), 0) as total_outstanding
		FROM purchases
		WHERE deleted_at IS NULL
		AND payment_method = 'CREDIT'
		AND outstanding_amount > 0
	`).Scan(&outstanding)
	fmt.Printf("  Total outstanding purchase amount (credit purchases): %.2f\n", outstanding.TotalOutstanding)
	fmt.Println("  (AP balance should roughly match total outstanding, but may differ due to initial balances)")

	fmt.Println()
	fmt.Println("=== Done! ===")
}
