package main

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Fix unified_journal_lines account_id for revenue entries from header 4101 to leaf 4101-001
func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=sistem_akuntansi port=5432 sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("DB connect failed:", err)
	}

	// Get header and leaf account IDs
	type AccInfo struct {
		ID uint
	}
	var headerAcc, leafAcc AccInfo
	db.Raw("SELECT id FROM accounts WHERE code = '4101' AND deleted_at IS NULL").Scan(&headerAcc)
	db.Raw("SELECT id FROM accounts WHERE code = '4101-001' AND deleted_at IS NULL").Scan(&leafAcc)
	fmt.Printf("Header 4101 ID=%d, Leaf 4101-001 ID=%d\n", headerAcc.ID, leafAcc.ID)

	// Fix unified_journal_lines - only update account_id (no account_code/name columns)
	result := db.Exec(`
		UPDATE unified_journal_lines 
		SET account_id = ?, updated_at = NOW()
		WHERE account_id = ? 
		  AND credit_amount > 0
		  AND description LIKE 'Pendapatan Penjualan%'
	`, leafAcc.ID, headerAcc.ID)
	if result.Error != nil {
		log.Printf("Failed: %v", result.Error)
	} else {
		fmt.Printf("Updated %d unified_journal_lines from header to leaf\n", result.RowsAffected)
	}

	// Verify
	var remaining int64
	db.Raw("SELECT COUNT(*) FROM unified_journal_lines WHERE account_id = ? AND credit_amount > 0 AND description LIKE 'Pendapatan Penjualan%'", headerAcc.ID).Scan(&remaining)
	fmt.Printf("Remaining header revenue entries: %d (expected: 0)\n", remaining)

	// Also check leaf count
	var leafCount int64
	db.Raw("SELECT COUNT(*) FROM unified_journal_lines WHERE account_id = ? AND credit_amount > 0", leafAcc.ID).Scan(&leafCount)
	fmt.Printf("Leaf 4101-001 credit entries: %d\n", leafCount)
}
