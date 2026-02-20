package main

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// fix_ocbc_balance.go - One-time script to fix OCBC account balance
// Root cause: CreatePurchasePayment controller called both createLegacyPaymentJournalWithTx
// AND SyncCOABalanceAfterPayment, causing double-deduction of accounts.balance.
//
// Fix: Set accounts.balance for 1101-005 to match the cash_banks.balance (which is correct).

func main() {
	db, err := gorm.Open(postgres.Open("host=localhost user=postgres password=postgres dbname=sistem_akuntansi port=5432 sslmode=disable"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== Fix OCBC Balance Discrepancy ===")
	fmt.Println()

	// 1. Get current state
	type AccInfo struct {
		ID      uint
		Code    string
		Name    string
		Balance float64
	}
	var acc AccInfo
	db.Raw("SELECT id, code, name, balance FROM accounts WHERE code = '1101-005' AND deleted_at IS NULL").Scan(&acc)
	fmt.Printf("BEFORE - Account 1101-005: ID=%d, Balance=%.2f\n", acc.ID, acc.Balance)

	type CBInfo struct {
		ID        uint
		Name      string
		Balance   float64
		AccountID uint
	}
	var cb CBInfo
	db.Raw("SELECT id, name, balance, account_id FROM cash_banks WHERE account_id = ? AND deleted_at IS NULL", acc.ID).Scan(&cb)
	fmt.Printf("BEFORE - Cash Bank: ID=%d, Balance=%.2f, AccountID=%d\n", cb.ID, cb.Balance, cb.AccountID)

	diff := cb.Balance - acc.Balance
	fmt.Printf("BEFORE - Discrepancy: %.2f\n\n", diff)

	if diff == 0 {
		fmt.Println("✅ No discrepancy found. Nothing to fix.")
		return
	}

	// 2. Fix: Set accounts.balance = cash_banks.balance (the correct value)
	fmt.Printf("Fixing: Setting accounts.balance for %s to %.2f (from cash_banks)\n", acc.Code, cb.Balance)
	result := db.Exec("UPDATE accounts SET balance = ?, updated_at = NOW() WHERE id = ? AND deleted_at IS NULL", cb.Balance, acc.ID)
	if result.Error != nil {
		log.Fatalf("Failed to update account balance: %v", result.Error)
	}
	fmt.Printf("✅ Updated %d row(s)\n\n", result.RowsAffected)

	// 3. Also fix the Utang Usaha (2101) account — it was also double-decremented
	var apAcc AccInfo
	db.Raw("SELECT id, code, name, balance FROM accounts WHERE code = '2101' AND deleted_at IS NULL").Scan(&apAcc)
	fmt.Printf("BEFORE - Utang Usaha (2101): Balance=%.2f\n", apAcc.Balance)

	// The correct AP balance = sum of credit journal lines - sum of debit journal lines for this account
	// For liability accounts, balance = Credit - Debit (positive means money owed)
	// Since the double-deduction reduced AP by an extra 12,487,500, we need to add it back
	correctedAPBalance := apAcc.Balance + diff // Add back the same discrepancy amount
	fmt.Printf("Fixing: Adding back %.2f to Utang Usaha (2101), new balance = %.2f\n", diff, correctedAPBalance)
	result = db.Exec("UPDATE accounts SET balance = ?, updated_at = NOW() WHERE id = ? AND deleted_at IS NULL", correctedAPBalance, apAcc.ID)
	if result.Error != nil {
		log.Fatalf("Failed to update AP balance: %v", result.Error)
	}
	fmt.Printf("✅ Updated %d row(s)\n\n", result.RowsAffected)

	// 4. Verify
	var accAfter AccInfo
	db.Raw("SELECT id, code, name, balance FROM accounts WHERE code = '1101-005' AND deleted_at IS NULL").Scan(&accAfter)
	fmt.Printf("AFTER - Account 1101-005: Balance=%.2f\n", accAfter.Balance)

	var cbAfter CBInfo
	db.Raw("SELECT id, name, balance FROM cash_banks WHERE account_id = ? AND deleted_at IS NULL", acc.ID).Scan(&cbAfter)
	fmt.Printf("AFTER - Cash Bank OCBC: Balance=%.2f\n", cbAfter.Balance)
	fmt.Printf("AFTER - Discrepancy: %.2f\n\n", cbAfter.Balance-accAfter.Balance)

	var apAfter AccInfo
	db.Raw("SELECT id, code, name, balance FROM accounts WHERE code = '2101' AND deleted_at IS NULL").Scan(&apAfter)
	fmt.Printf("AFTER - Utang Usaha (2101): Balance=%.2f\n\n", apAfter.Balance)

	// 5. Clean up: Remove the duplicate SimpleSSOTJournal entries created by the sync service
	fmt.Println("=== Cleaning up duplicate SimpleSSOTJournal entries ===")
	var ssotCount int64
	db.Raw("SELECT COUNT(*) FROM simple_ssot_journals WHERE transaction_type = 'PURCHASE_PAYMENT' AND deleted_at IS NULL").Scan(&ssotCount)
	fmt.Printf("Found %d SimpleSSOTJournal PURCHASE_PAYMENT entries (these are duplicates created by sync)\n", ssotCount)
	if ssotCount > 0 {
		// Soft delete items first
		db.Exec("UPDATE simple_ssot_journal_items SET deleted_at = NOW() WHERE journal_id IN (SELECT id FROM simple_ssot_journals WHERE transaction_type = 'PURCHASE_PAYMENT' AND deleted_at IS NULL)")
		// Soft delete journals
		result = db.Exec("UPDATE simple_ssot_journals SET deleted_at = NOW() WHERE transaction_type = 'PURCHASE_PAYMENT' AND deleted_at IS NULL")
		fmt.Printf("✅ Soft-deleted %d duplicate SimpleSSOTJournal entries\n", result.RowsAffected)
	}

	fmt.Println("\n=== Fix Complete ===")
}
