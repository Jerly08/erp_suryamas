package database

import (
	"app-sistem-akuntansi/models"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"gorm.io/gorm"
)

const purchaseRequestBudgetTotalMigrationID = "purchase_request_budget_total_v1.0"

// PurchaseRequestBudgetTotalMigration backfills approved PR totals so legacy
// finalized requests exclude item rows that were rejected during review.
func PurchaseRequestBudgetTotalMigration(db *gorm.DB) {
	log.Println("Starting Purchase Request Budget Total Migration...")

	var existingMigration models.MigrationRecord
	if err := db.Where("migration_id = ?", purchaseRequestBudgetTotalMigrationID).First(&existingMigration).Error; err == nil {
		log.Printf("Purchase Request Budget Total Migration already applied at %v", existingMigration.AppliedAt)
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("Warning: failed to check Purchase Request Budget Total Migration status: %v", err)
		return
	}

	tx := db.Begin()
	if tx.Error != nil {
		log.Printf("Failed to start Purchase Request Budget Total Migration: %v", tx.Error)
		return
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Printf("Purchase Request Budget Total Migration rolled back due to panic: %v", r)
		}
	}()

	var requests []models.PurchaseRequest
	if err := tx.Preload("Items").
		Where("status IN ? OR current_approval_stage = ?",
			[]string{models.PRStatusApproved, models.PRStatusRejected, models.PRStatusCompleted},
			models.PRApprovalStageFinalized,
		).
		Find(&requests).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to load purchase requests for budget total migration: %v", err)
		return
	}

	var repaired int64
	for _, pr := range requests {
		total, hasFinalDecision := calculateApprovedPurchaseRequestItemTotal(pr.Items)
		if !hasFinalDecision {
			continue
		}

		if math.Abs(pr.TotalEstimatedAmount-total) < 0.01 {
			continue
		}

		if err := tx.Model(&models.PurchaseRequest{}).
			Where("id = ?", pr.ID).
			Update("total_estimated_amount", total).Error; err != nil {
			tx.Rollback()
			log.Printf("Failed to update purchase request %s budget total: %v", pr.PRNumber, err)
			return
		}
		repaired++
	}

	migrationRecord := models.MigrationRecord{
		MigrationID: purchaseRequestBudgetTotalMigrationID,
		Description: fmt.Sprintf("Backfilled approved purchase request totals; repaired rows: %d", repaired),
		Version:     "1.0",
		AppliedAt:   time.Now(),
	}
	if err := tx.Create(&migrationRecord).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to record Purchase Request Budget Total Migration: %v", err)
		return
	}

	if err := tx.Commit().Error; err != nil {
		log.Printf("Failed to commit Purchase Request Budget Total Migration: %v", err)
		return
	}

	log.Printf("Purchase Request Budget Total Migration completed successfully. Repaired rows: %d", repaired)
}

func calculateApprovedPurchaseRequestItemTotal(items []models.PurchaseRequestItem) (float64, bool) {
	var total float64
	hasFinalDecision := false

	for _, item := range items {
		switch item.ApprovalStatus {
		case models.PRItemDecisionApproved:
			total += item.TotalEstimatedPrice
			hasFinalDecision = true
		case models.PRItemDecisionRejected:
			hasFinalDecision = true
		}
	}

	return total, hasFinalDecision
}
