package database

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
)

const journalReversalFutureDateAlignmentMigrationID = "journal_reversal_future_date_alignment_v1.0"

// JournalReversalFutureDateAlignmentMigration repairs legacy reversal journals
// whose accounting date was earlier than the original journal date.
// A reversal must never become effective before the entry it reverses, otherwise
// historical balance-sheet snapshots can become unbalanced.
func JournalReversalFutureDateAlignmentMigration(db *gorm.DB) {
	if db == nil {
		return
	}

	log.Println("Starting Journal Reversal Future Date Alignment Migration...")

	var existingMigration models.MigrationRecord
	if err := db.Where("migration_id = ?", journalReversalFutureDateAlignmentMigrationID).First(&existingMigration).Error; err == nil {
		log.Printf("Journal Reversal Future Date Alignment Migration already applied at %v", existingMigration.AppliedAt)
		return
	}

	tx := db.Begin()
	if tx.Error != nil {
		log.Printf("Failed to start Journal Reversal Future Date Alignment Migration: %v", tx.Error)
		return
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Printf("Journal Reversal Future Date Alignment Migration rolled back due to panic: %v", r)
		}
	}()

	result := tx.Exec(`
		UPDATE unified_journal_ledger AS reversal
		SET
			entry_date = original.entry_date,
			updated_at = NOW()
		FROM unified_journal_ledger AS original
		WHERE reversal.deleted_at IS NULL
			AND original.deleted_at IS NULL
			AND reversal.status = 'POSTED'
			AND original.status = 'POSTED'
			AND UPPER(COALESCE(reversal.source_type, '')) = 'REVERSAL'
			AND reversal.reversed_from = original.id
			AND reversal.entry_date < original.entry_date
	`)
	if result.Error != nil {
		tx.Rollback()
		log.Printf("Failed to repair reversal journal entry dates: %v", result.Error)
		return
	}

	migrationRecord := models.MigrationRecord{
		MigrationID: journalReversalFutureDateAlignmentMigrationID,
		Description: fmt.Sprintf("Aligned %d reversal journal entry dates so they are not earlier than their original journals", result.RowsAffected),
		Version:     "1.0",
		AppliedAt:   time.Now(),
	}
	if err := tx.Create(&migrationRecord).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "uni_migration_records_migration_id") {
			log.Printf("Journal Reversal Future Date Alignment Migration record already exists")
		} else {
			tx.Rollback()
			log.Printf("Failed to record Journal Reversal Future Date Alignment Migration: %v", err)
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		log.Printf("Failed to commit Journal Reversal Future Date Alignment Migration: %v", err)
		return
	}

	log.Printf("Journal Reversal Future Date Alignment Migration completed successfully. Repaired rows: %d", result.RowsAffected)
}
