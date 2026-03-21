package database

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
)

const cashBankBookAmountRepairMigrationID = "cashbank_book_amount_repair_v1.0"

// CashBankBookAmountRepairMigration backfills legacy IDR cash/bank transaction
// rows where book_amount_id_r was left blank or diverged from the native amount.
// For IDR transactions, the book amount must always mirror the native amount.
func CashBankBookAmountRepairMigration(db *gorm.DB) {
	log.Println("🔧 Starting CashBank Book Amount Repair Migration...")

	var existingMigration models.MigrationRecord
	if err := db.Where("migration_id = ?", cashBankBookAmountRepairMigrationID).First(&existingMigration).Error; err == nil {
		log.Printf("✅ CashBank Book Amount Repair Migration already applied at %v", existingMigration.AppliedAt)
		return
	}

	tx := db.Begin()
	if tx.Error != nil {
		log.Printf("❌ Failed to start CashBank Book Amount Repair Migration: %v", tx.Error)
		return
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Printf("❌ CashBank Book Amount Repair Migration rolled back due to panic: %v", r)
		}
	}()

	result := tx.Exec(`
		UPDATE cash_bank_transactions
		SET
			book_amount_id_r = amount,
			currency = COALESCE(NULLIF(currency, ''), 'IDR'),
			exchange_rate = CASE
				WHEN exchange_rate IS NULL OR ABS(exchange_rate) < 0.000001 THEN 1
				ELSE exchange_rate
			END,
			updated_at = NOW()
		WHERE deleted_at IS NULL
			AND UPPER(COALESCE(NULLIF(currency, ''), 'IDR')) = 'IDR'
			AND ABS(COALESCE(book_amount_id_r, 0) - amount) > 0.01
	`)
	if result.Error != nil {
		tx.Rollback()
		log.Printf("❌ Failed to repair legacy CashBank book amounts: %v", result.Error)
		return
	}

	migrationRecord := models.MigrationRecord{
		MigrationID: cashBankBookAmountRepairMigrationID,
		Description: fmt.Sprintf("Repaired %d legacy IDR cash_bank_transactions book_amount_id_r rows", result.RowsAffected),
		Version:     "1.0",
		AppliedAt:   time.Now(),
	}
	if err := tx.Create(&migrationRecord).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "uni_migration_records_migration_id") {
			log.Printf("ℹ️ CashBank Book Amount Repair Migration record already exists - repair already applied")
		} else {
			tx.Rollback()
			log.Printf("❌ Failed to record CashBank Book Amount Repair Migration: %v", err)
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		log.Printf("❌ Failed to commit CashBank Book Amount Repair Migration: %v", err)
		return
	}

	log.Printf("✅ CashBank Book Amount Repair Migration completed successfully. Repaired rows: %d", result.RowsAffected)
}
