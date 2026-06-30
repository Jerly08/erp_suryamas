package database

import (
	"app-sistem-akuntansi/models"
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

const purchaseRequestInvoiceDueDateMigrationID = "purchase_request_invoice_due_date_v1.0"

type purchaseRequestDueDateBackfillRow struct {
	PurchaseRequestID uint      `gorm:"column:purchase_request_id"`
	DueDate           time.Time `gorm:"column:due_date"`
}

// PurchaseRequestInvoiceDueDateMigration backfills PR due dates from linked
// purchases for legacy data created before PR-level invoice terms existed.
func PurchaseRequestInvoiceDueDateMigration(db *gorm.DB) {
	log.Println("Starting Purchase Request Invoice Due Date Migration...")

	var existingMigration models.MigrationRecord
	if err := db.Where("migration_id = ?", purchaseRequestInvoiceDueDateMigrationID).First(&existingMigration).Error; err == nil {
		log.Printf("Purchase Request Invoice Due Date Migration already applied at %v", existingMigration.AppliedAt)
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("Warning: failed to check Purchase Request Invoice Due Date Migration status: %v", err)
		return
	}

	zeroDate := time.Date(1, time.January, 2, 0, 0, 0, 0, time.UTC)
	var rows []purchaseRequestDueDateBackfillRow
	if err := db.Raw(`
		WITH linked_purchase_dates AS (
			SELECT p.purchase_request_id AS purchase_request_id, p.due_date AS due_date
			FROM purchases p
			WHERE p.purchase_request_id IS NOT NULL
				AND p.deleted_at IS NULL
				AND p.status <> ?
				AND p.due_date > ?

			UNION ALL

			SELECT pri.purchase_request_id AS purchase_request_id, p.due_date AS due_date
			FROM purchase_request_items pri
			JOIN purchases p
				ON p.id = pri.purchase_id
				AND p.deleted_at IS NULL
				AND p.status <> ?
				AND p.due_date > ?
			WHERE pri.deleted_at IS NULL

			UNION ALL

			SELECT pri.purchase_request_id AS purchase_request_id, p.due_date AS due_date
			FROM purchase_request_items pri
			JOIN purchase_items pi
				ON pi.purchase_request_item_id = pri.id
				AND pi.deleted_at IS NULL
			JOIN purchases p
				ON p.id = pi.purchase_id
				AND p.deleted_at IS NULL
				AND p.status <> ?
				AND p.due_date > ?
			WHERE pri.deleted_at IS NULL
		)
		SELECT lpd.purchase_request_id, MIN(lpd.due_date) AS due_date
		FROM linked_purchase_dates lpd
		JOIN purchase_requests pr
			ON pr.id = lpd.purchase_request_id
			AND pr.deleted_at IS NULL
		WHERE pr.due_date IS NULL
		GROUP BY lpd.purchase_request_id
	`, models.PurchaseStatusCancelled, zeroDate, models.PurchaseStatusCancelled, zeroDate, models.PurchaseStatusCancelled, zeroDate).
		Scan(&rows).Error; err != nil {
		log.Printf("Failed to scan purchase request due date backfill rows: %v", err)
		return
	}

	tx := db.Begin()
	if tx.Error != nil {
		log.Printf("Failed to start Purchase Request Invoice Due Date Migration: %v", tx.Error)
		return
	}

	var repaired int64
	for _, row := range rows {
		if row.PurchaseRequestID == 0 || row.DueDate.IsZero() {
			continue
		}

		if err := tx.Model(&models.PurchaseRequest{}).
			Where("id = ? AND due_date IS NULL", row.PurchaseRequestID).
			Update("due_date", row.DueDate).Error; err != nil {
			tx.Rollback()
			log.Printf("Failed to backfill due date for purchase request %d: %v", row.PurchaseRequestID, err)
			return
		}
		repaired++
	}

	migrationRecord := models.MigrationRecord{
		MigrationID: purchaseRequestInvoiceDueDateMigrationID,
		Description: fmt.Sprintf("Backfilled purchase request due dates from linked purchases; repaired rows: %d", repaired),
		Version:     "1.0",
		AppliedAt:   time.Now(),
	}
	if err := tx.Create(&migrationRecord).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to record Purchase Request Invoice Due Date Migration: %v", err)
		return
	}

	if err := tx.Commit().Error; err != nil {
		log.Printf("Failed to commit Purchase Request Invoice Due Date Migration: %v", err)
		return
	}

	log.Printf("Purchase Request Invoice Due Date Migration completed successfully. Repaired rows: %d", repaired)
}
