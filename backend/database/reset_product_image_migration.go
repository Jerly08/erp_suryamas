package database

import (
	"app-sistem-akuntansi/models"
	"log"

	"gorm.io/gorm"
)

// ResetProductImageMigration resets the product image fix migration record
func ResetProductImageMigration(db *gorm.DB) {
	log.Println("🔧 Resetting product image fix migration record...")

	migrationID := "product_image_fix_v1.0"

	// Delete existing migration record to allow re-running
	result := db.Where("migration_id = ?", migrationID).Delete(&models.MigrationRecord{})
	if result.Error != nil {
		log.Printf("Warning: Failed to reset product image migration record: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("✅ Reset product image migration record (deleted %d records)", result.RowsAffected)
	} else {
		log.Println("ℹ️  Product image migration reset not needed")
	}

	log.Println("✅ Product image migration reset completed")
}
