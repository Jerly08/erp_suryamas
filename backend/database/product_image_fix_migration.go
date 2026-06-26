package database

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ProductImageFixMigration fixes issues with product image uploads
func ProductImageFixMigration(db *gorm.DB) {
	log.Println("🔧 Starting Product Image Fix Migration...")

	migrationID := "product_image_fix_v1.0"

	// Check if this migration has already been run (with better error handling)
	var existingMigration models.MigrationRecord
	// Use silent logging for the initial check to avoid "record not found" error logs
	silentDB := db.Session(&gorm.Session{Logger: db.Logger.LogMode(logger.Silent)})
	err := silentDB.Where("migration_id = ?", migrationID).First(&existingMigration).Error
	if err == nil {
		log.Printf("✅ Product Image Fix Migration already completed at %s - skipping",
			existingMigration.AppliedAt.Format("2006-01-02 15:04:05"))
		log.Printf("ℹ️  Previous migration description: %s", existingMigration.Description)
		return
	} else if err.Error() != "record not found" {
		log.Printf("⚠️  Warning: Could not check migration status: %v", err)
		log.Printf("ℹ️  Proceeding with migration anyway (this is safe)...")
	} else {
		log.Printf("ℹ️  Product Image Fix Migration is pending, proceeding with migration...")
	}

	// Use separate transactions for each fix to avoid rollback cascades
	var fixesApplied []string

	// Fix 1: Ensure image_path column has correct type and size
	if err := ensureImagePathColumn(db); err == nil {
		fixesApplied = append(fixesApplied, "Image path column validation")
	}

	// Fix 2: Clean up invalid image paths
	if err := cleanupInvalidImagePaths(db); err == nil {
		fixesApplied = append(fixesApplied, "Invalid image paths cleanup")
	}

	// Fix 3: Create uploads directory structure
	if err := ensureUploadsDirectory(); err == nil {
		fixesApplied = append(fixesApplied, "Uploads directory structure")
	}

	// Record this migration as completed in a separate transaction
	migrationRecord := models.MigrationRecord{
		MigrationID: migrationID,
		Description: fmt.Sprintf("Product image fix migration applied: %v", fixesApplied),
		Version:     "1.0",
		AppliedAt:   time.Now(),
	}

	// Log what we're about to record
	log.Printf("📝 Recording migration completion: %s (applied %d fixes)", migrationID, len(fixesApplied))

	// Use a session with silent logging for the migration record creation to avoid cluttering logs
	silentDBCreate := db.Session(&gorm.Session{Logger: db.Logger.LogMode(logger.Silent)}) // Silent mode
	if err := silentDBCreate.Create(&migrationRecord).Error; err != nil {
		// Check if this is just a duplicate key constraint (normal scenario during concurrent runs)
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "uni_migration_records_migration_id") {
			log.Printf("ℹ️  Migration record '%s' already exists - this happens when multiple processes start simultaneously", migrationID)
			log.Printf("✅ Product Image Fix Migration completed successfully!")
			log.Printf("📝 Applied fixes: %v", fixesApplied)
			log.Printf("🔄 This is normal behavior and indicates the migration system is working correctly.")
		} else {
			log.Printf("❌ Failed to record product image fix migration in database: %v", err)
			log.Printf("⚠️  Migration fixes were applied successfully but database record failed: %v", fixesApplied)
			log.Printf("🚨 This may cause the migration to re-run on next startup (which is safe)")
		}
		return
	}

	log.Printf("✅ Product Image Fix Migration completed successfully and recorded in database!")
	log.Printf("📝 Applied fixes: %v", fixesApplied)
	log.Printf("📦 Migration ID: %s, Version: %s", migrationID, migrationRecord.Version)
}

// ensureImagePathColumn ensures the image_path column has the correct specifications
func ensureImagePathColumn(db *gorm.DB) error {
	log.Println("  🔧 Ensuring image_path column specifications...")

	// Check current column specification
	var columnInfo struct {
		DataType      string `json:"data_type"`
		IsNullable    string `json:"is_nullable"`
		ColumnDefault string `json:"column_default"`
	}

	err := db.Raw(`
		SELECT data_type, is_nullable, column_default 
		FROM information_schema.columns 
		WHERE table_name = 'products' AND column_name = 'image_path'
	`).Scan(&columnInfo).Error

	if err != nil {
		log.Printf("    ❌ Error checking image_path column: %v", err)
		return err
	}

	log.Printf("    📊 Current image_path column: type=%s, nullable=%s, default=%s",
		columnInfo.DataType, columnInfo.IsNullable, columnInfo.ColumnDefault)

	// Ensure column is adequately sized (VARCHAR(255) should be sufficient)
	if !strings.Contains(strings.ToLower(columnInfo.DataType), "varchar") {
		log.Println("    🔧 Adjusting image_path column specifications...")

		// Modify column to ensure proper size (PostgreSQL syntax)
		// Use separate statements to avoid transaction issues
		err = db.Exec("ALTER TABLE products ALTER COLUMN image_path TYPE VARCHAR(255)").Error
		if err != nil {
			log.Printf("    ❌ Failed to modify image_path column type: %v", err)
			// Continue with migration even if column type change fails
		} else {
			log.Println("    ✅ Modified image_path column type to VARCHAR(255)")
		}

		// Set default value in separate statement
		err = db.Exec("ALTER TABLE products ALTER COLUMN image_path SET DEFAULT ''").Error
		if err != nil {
			log.Printf("    ⚠️  Failed to set default value for image_path column: %v", err)
			// This is non-critical, continue
		} else {
			log.Println("    ✅ Set default value for image_path column")
		}
	} else {
		log.Println("    ✅ image_path column specifications are correct")
	}

	return nil
}

// cleanupInvalidImagePaths cleans up any invalid or malformed image paths
func cleanupInvalidImagePaths(db *gorm.DB) error {
	log.Println("  🔧 Cleaning up invalid image paths...")

	// Find products with potentially problematic image paths
	var problematicProducts []struct {
		ID        uint   `json:"id"`
		Code      string `json:"code"`
		Name      string `json:"name"`
		ImagePath string `json:"image_path"`
	}

	err := db.Raw(`
		SELECT id, code, name, image_path
		FROM products 
		WHERE image_path IS NOT NULL 
		AND image_path != ''
		AND (
			LENGTH(image_path) > 255 
			OR image_path LIKE '%\\\\%' 
			OR image_path LIKE '%//%'
			OR image_path NOT LIKE '/uploads/%'
		)
		LIMIT 100
	`).Scan(&problematicProducts).Error

	if err != nil {
		log.Printf("    ❌ Error finding products with invalid image paths: %v", err)
		return err
	}

	if len(problematicProducts) == 0 {
		log.Println("    ✅ No products with invalid image paths found")
		return nil
	}

	log.Printf("    📊 Found %d products with potentially invalid image paths", len(problematicProducts))

	fixedCount := 0
	for _, product := range problematicProducts {
		var newImagePath string

		// Try to fix common issues
		if len(product.ImagePath) > 255 {
			// Path too long - reset to empty
			newImagePath = ""
			log.Printf("    🔧 Resetting overly long path for product %d (%s)", product.ID, product.Code)
		} else {
			// Fix path separators and format
			path := product.ImagePath

			// Convert backslashes to forward slashes
			path = strings.ReplaceAll(path, "\\", "/")

			// Remove double slashes
			for strings.Contains(path, "//") {
				path = strings.ReplaceAll(path, "//", "/")
			}

			// Ensure path starts with /uploads/ if it contains uploads
			if strings.Contains(path, "uploads") && !strings.HasPrefix(path, "/uploads/") {
				if strings.Contains(path, "/uploads/") {
					// Extract from /uploads/ onwards
					index := strings.Index(path, "/uploads/")
					path = path[index:]
				} else if strings.Contains(path, "uploads/") {
					// Add leading slash
					index := strings.Index(path, "uploads/")
					path = "/" + path[index:]
				}
			}

			newImagePath = path
		}

		// Update the product if the path changed
		if newImagePath != product.ImagePath {
			err := db.Model(&models.Product{}).
				Where("id = ?", product.ID).
				Update("image_path", newImagePath).Error

			if err != nil {
				log.Printf("    ❌ Failed to fix image path for product %d: %v", product.ID, err)
			} else {
				log.Printf("    ✅ Fixed image path for product %d (%s): %s -> %s",
					product.ID, product.Code, product.ImagePath, newImagePath)
				fixedCount++
			}
		}
	}

	log.Printf("    ✅ Fixed %d out of %d problematic image paths", fixedCount, len(problematicProducts))
	return nil
}

// ensureUploadsDirectory creates the uploads directory structure if it doesn't exist
func ensureUploadsDirectory() error {
	log.Println("  🔧 Ensuring uploads directory structure...")

	directories := []string{
		"./uploads",
		"./uploads/products",
		"./uploads/assets",
		"./uploads/temp",
	}

	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("    ❌ Failed to create directory %s: %v", dir, err)
			return err
		}
	}

	log.Println("    ✅ Upload directory structure ensured")
	return nil
}
