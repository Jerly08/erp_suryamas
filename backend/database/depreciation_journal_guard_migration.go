package database

import (
	"app-sistem-akuntansi/models"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
)

const depreciationJournalGuardMigrationID = "depreciation_journal_guard_v1.0"
const depreciationJournalGuardIndexName = "idx_unified_journal_depreciation_reference_unique"

// DepreciationJournalGuardMigration prevents duplicate depreciation journals for the same asset-period.
// It is safe to run multiple times.
func DepreciationJournalGuardMigration(db *gorm.DB) {
	log.Println("🔒 Starting depreciation journal guard migration...")

	var existingMigration models.MigrationRecord
	if err := db.Where("migration_id = ?", depreciationJournalGuardMigrationID).First(&existingMigration).Error; err == nil {
		log.Printf("✅ Depreciation journal guard migration already applied at %v", existingMigration.AppliedAt)
		return
	}

	var duplicateCount int64
	if err := db.Raw(`
		SELECT COUNT(*) FROM (
			SELECT reference
			FROM unified_journal_ledger
			WHERE source_type = ?
			  AND deleted_at IS NULL
			  AND reference IS NOT NULL
			GROUP BY reference
			HAVING COUNT(*) > 1
		) d
	`, models.SSOTSourceTypeDepreciation).Scan(&duplicateCount).Error; err != nil {
		log.Printf("⚠️ Failed to validate depreciation duplicates before migration: %v", err)
		return
	}

	if duplicateCount > 0 {
		log.Printf("⚠️ Depreciation journal guard migration skipped: found %d duplicate depreciation references that must be resolved first", duplicateCount)
		return
	}

	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_unified_journal_depreciation_reference_unique
		ON unified_journal_ledger (reference)
		WHERE source_type = 'DEPRECIATION'
		  AND deleted_at IS NULL
		  AND reference IS NOT NULL
	`).Error; err != nil {
		log.Printf("⚠️ Failed to create depreciation unique index: %v", err)
		return
	}

	record := models.MigrationRecord{
		MigrationID: depreciationJournalGuardMigrationID,
		Description: strings.Join([]string{
			"Added unique partial index for depreciation journals",
			"Prevents duplicate source_type=DEPRECIATION posting for the same reference",
		}, ". "),
		Version:   "1.0",
		AppliedAt: time.Now(),
	}
	if err := db.Create(&record).Error; err != nil {
		log.Printf("⚠️ Failed to record depreciation journal guard migration: %v", err)
		return
	}

	log.Println("✅ Depreciation journal guard migration completed successfully")
}
