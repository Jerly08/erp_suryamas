package main

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// This script fixes existing data where revenue accounts were incorrectly set to header account 4101
// instead of leaf account 4101-001. It also regenerates stale journal entries.
func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=sistem_akuntansi port=5432 sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("DB connect failed:", err)
	}

	fmt.Println("=== Fix Revenue Header Account Bug ===")
	fmt.Println()

	// Step 1: Find header account 4101 and leaf account 4101-001
	type AccInfo struct {
		ID       uint
		Code     string
		IsHeader bool
	}
	var headerAcc, leafAcc AccInfo
	db.Raw("SELECT id, code, is_header FROM accounts WHERE code = '4101' AND deleted_at IS NULL").Scan(&headerAcc)
	db.Raw("SELECT id, code, is_header FROM accounts WHERE code = '4101-001' AND deleted_at IS NULL").Scan(&leafAcc)

	if headerAcc.ID == 0 || leafAcc.ID == 0 {
		log.Fatal("Cannot find both 4101 and 4101-001 accounts")
	}
	fmt.Printf("Header account: ID=%d (4101, is_header=%v)\n", headerAcc.ID, headerAcc.IsHeader)
	fmt.Printf("Leaf account:   ID=%d (4101-001, is_header=%v)\n", leafAcc.ID, leafAcc.IsHeader)
	fmt.Println()

	// Step 2: Fix tax_account_settings
	fmt.Println("--- FIX 1: tax_account_settings ---")
	result := db.Exec(`
		UPDATE tax_account_settings 
		SET sales_revenue_account_id = ?, updated_at = NOW()
		WHERE sales_revenue_account_id = ?
	`, leafAcc.ID, headerAcc.ID)
	if result.Error != nil {
		log.Printf("⚠️ Failed: %v", result.Error)
	} else {
		fmt.Printf("Updated %d rows in tax_account_settings\n", result.RowsAffected)
	}

	// Step 3: Fix sale_items.revenue_account_id
	fmt.Println("\n--- FIX 2: sale_items ---")
	result = db.Exec(`
		UPDATE sale_items 
		SET revenue_account_id = ?, updated_at = NOW()
		WHERE revenue_account_id = ? AND deleted_at IS NULL
	`, leafAcc.ID, headerAcc.ID)
	if result.Error != nil {
		log.Printf("⚠️ Failed: %v", result.Error)
	} else {
		fmt.Printf("Updated %d sale_items from header to leaf\n", result.RowsAffected)
	}

	// Step 4: Fix unified_journal_lines pointing to header revenue
	fmt.Println("\n--- FIX 3: unified_journal_lines ---")
	result = db.Exec(`
		UPDATE unified_journal_lines 
		SET account_id = ?, account_code = '4101-001', 
		    account_name = (SELECT name FROM accounts WHERE id = ?),
		    updated_at = NOW()
		WHERE account_id = ? 
		  AND credit_amount > 0
		  AND description LIKE 'Pendapatan Penjualan%'
	`, leafAcc.ID, leafAcc.ID, headerAcc.ID)
	if result.Error != nil {
		log.Printf("⚠️ Failed: %v", result.Error)
	} else {
		fmt.Printf("Updated %d unified_journal_lines from header to leaf\n", result.RowsAffected)
	}

	// Step 5: Fix simple_ssot_journal_items pointing to header revenue
	fmt.Println("\n--- FIX 4: simple_ssot_journal_items ---")
	result = db.Exec(`
		UPDATE simple_ssot_journal_items 
		SET account_id = ?, account_code = '4101-001', 
		    account_name = (SELECT name FROM accounts WHERE id = ?)
		WHERE account_id = ? 
		  AND credit > 0
		  AND description LIKE 'Sales revenue%'
	`, leafAcc.ID, leafAcc.ID, headerAcc.ID)
	if result.Error != nil {
		log.Printf("⚠️ Failed: %v", result.Error)
	} else {
		fmt.Printf("Updated %d simple_ssot_journal_items from header to leaf\n", result.RowsAffected)
	}

	// Step 6: Recalculate parent balances
	fmt.Println("\n--- FIX 5: Recalculate parent account balances ---")
	result = db.Exec(`
		UPDATE accounts 
		SET balance = COALESCE((
			SELECT SUM(balance) FROM accounts children 
			WHERE children.parent_id = accounts.id AND children.deleted_at IS NULL
		), 0),
		updated_at = NOW()
		WHERE is_header = true AND deleted_at IS NULL
		  AND code LIKE '4%'
	`)
	if result.Error != nil {
		log.Printf("⚠️ Failed: %v", result.Error)
	} else {
		fmt.Printf("Recalculated %d parent account balances\n", result.RowsAffected)
	}

	// Verification
	fmt.Println("\n=== Verification ===")

	// Check tax settings
	var settingsRevID uint
	db.Raw("SELECT sales_revenue_account_id FROM tax_account_settings WHERE is_active = true LIMIT 1").Scan(&settingsRevID)
	fmt.Printf("tax_account_settings.sales_revenue_account_id = %d (expected: %d) ", settingsRevID, leafAcc.ID)
	if settingsRevID == leafAcc.ID {
		fmt.Println("✅")
	} else {
		fmt.Println("❌")
	}

	// Check sale items
	var headerCount int64
	db.Raw("SELECT COUNT(*) FROM sale_items WHERE revenue_account_id = ? AND deleted_at IS NULL", headerAcc.ID).Scan(&headerCount)
	fmt.Printf("sale_items with header account = %d (expected: 0) ", headerCount)
	if headerCount == 0 {
		fmt.Println("✅")
	} else {
		fmt.Println("❌")
	}

	// Check unified journal lines
	var ujlHeaderCount int64
	db.Raw("SELECT COUNT(*) FROM unified_journal_lines WHERE account_id = ? AND credit_amount > 0 AND description LIKE 'Pendapatan Penjualan%'", headerAcc.ID).Scan(&ujlHeaderCount)
	fmt.Printf("unified_journal_lines with header revenue = %d (expected: 0) ", ujlHeaderCount)
	if ujlHeaderCount == 0 {
		fmt.Println("✅")
	} else {
		fmt.Println("❌")
	}

	fmt.Println("\n=== Done! ===")
	fmt.Println("Please restart the backend and refresh the Neraca page.")
}
