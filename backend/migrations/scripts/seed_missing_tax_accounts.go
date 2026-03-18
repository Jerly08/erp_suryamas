//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"
	"gorm.io/gorm"
)

// SeedMissingTaxAccounts adds missing tax accounts to existing databases
// This migration script adds:
// - 1114: PPh 21 Dibayar Dimuka
// - 1115: PPh 23 Dibayar Dimuka
// - 2104: PPh Yang Dipotong
//
// These accounts are required for proper tax journal entries in purchase and sales
//
// Usage:
//   go run backend/migrations/scripts/seed_missing_tax_accounts.go
func main() {
	log.Println("🚀 Starting missing tax accounts migration...")

	// Load environment configuration
	if err := loadEnv(); err != nil {
		log.Fatalf("❌ Failed to load environment: %v", err)
	}

	// Initialize database connection
	cfg := config.GetConfig()
	db, err := config.InitDB(cfg)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}

	log.Println("✅ Database connection established")

	// Run migration
	if err := seedMissingTaxAccounts(db); err != nil {
		log.Fatalf("❌ Migration failed: %v", err)
	}

	// Run hierarchy fixes
	if err := database.FixAccountHierarchies(db); err != nil {
		log.Printf("⚠️  Warning: Failed to fix hierarchies: %v", err)
	}

	log.Println("✅ Migration completed successfully!")
	log.Println("")
	log.Println("📋 Summary of added accounts:")
	log.Println("  • 1114 - PPh 21 DIBAYAR DIMUKA (Asset/Current Asset)")
	log.Println("  • 1115 - PPh 23 DIBAYAR DIMUKA (Asset/Current Asset)")
	log.Println("  • 2104 - PPh YANG DIPOTONG (Liability/Current Liability)")
	log.Println("")
	log.Println("💡 Next steps:")
	log.Println("  1. Restart your backend server")
	log.Println("  2. Test purchase transactions with PPN")
	log.Println("  3. Test sales transactions with PPh")
	log.Println("  4. Verify Balance Sheet and P&L reports")
}

func seedMissingTaxAccounts(db *gorm.DB) error {
	log.Println("📝 Adding missing tax accounts...")

	// Define missing accounts
	taxAccounts := []struct {
		Code     string
		Name     string
		Type     string
		Category string
		ParentCode string
	}{
		{
			Code:     "1114",
			Name:     "PPh 21 DIBAYAR DIMUKA",
			Type:     models.AccountTypeAsset,
			Category: models.CategoryCurrentAsset,
			ParentCode: "1200", // ACCOUNTS RECEIVABLE
		},
		{
			Code:     "1115",
			Name:     "PPh 23 DIBAYAR DIMUKA",
			Type:     models.AccountTypeAsset,
			Category: models.CategoryCurrentAsset,
			ParentCode: "1200", // ACCOUNTS RECEIVABLE
		},
		{
			Code:     "2104",
			Name:     "PPh YANG DIPOTONG",
			Type:     models.AccountTypeLiability,
			Category: models.CategoryCurrentLiability,
			ParentCode: "2100", // CURRENT LIABILITIES
		},
	}

	// Find parent accounts
	parentMap := make(map[string]uint)
	for _, acc := range taxAccounts {
		var parent models.Account
		if err := db.Where("code = ?", acc.ParentCode).First(&parent).Error; err != nil {
			log.Printf("⚠️  Warning: Parent account %s not found for %s", acc.ParentCode, acc.Code)
			continue
		}
		parentMap[acc.ParentCode] = parent.ID
	}

	// Create or update tax accounts
	for _, acc := range taxAccounts {
		var existingAccount models.Account
		
		// Check if account already exists
		result := db.Where("code = ?", acc.Code).First(&existingAccount)
		
		if result.Error == gorm.ErrRecordNotFound {
			// Account doesn't exist, create it
			parentID := parentMap[acc.ParentCode]
			var parent models.Account
			db.First(&parent, parentID)
			
			newAccount := models.Account{
				Code:     acc.Code,
				Name:     acc.Name,
				Type:     acc.Type,
				Category: acc.Category,
				ParentID: &parentID,
				Level:    parent.Level + 1,
				IsHeader: false,
				IsActive: true,
				Balance:  0,
			}

			if err := db.Create(&newAccount).Error; err != nil {
				return fmt.Errorf("failed to create account %s: %v", acc.Code, err)
			}

			log.Printf("✅ Created account: %s - %s", acc.Code, acc.Name)
		} else if result.Error != nil {
			return fmt.Errorf("failed to check account %s: %v", acc.Code, result.Error)
		} else {
			// Account exists, update if needed
			updated := false
			
			if existingAccount.Name != acc.Name {
				existingAccount.Name = acc.Name
				updated = true
			}
			
			if existingAccount.Type != acc.Type {
				existingAccount.Type = acc.Type
				updated = true
			}
			
			if existingAccount.Category != acc.Category {
				existingAccount.Category = acc.Category
				updated = true
			}
			
			// Check parent relationship
			parentID := parentMap[acc.ParentCode]
			if existingAccount.ParentID == nil || *existingAccount.ParentID != parentID {
				existingAccount.ParentID = &parentID
				var parent models.Account
				db.First(&parent, parentID)
				existingAccount.Level = parent.Level + 1
				updated = true
			}
			
			if updated {
				if err := db.Save(&existingAccount).Error; err != nil {
					return fmt.Errorf("failed to update account %s: %v", acc.Code, err)
				}
				log.Printf("🔄 Updated account: %s - %s", acc.Code, acc.Name)
			} else {
				log.Printf("ℹ️  Account already exists: %s - %s", acc.Code, acc.Name)
			}
		}
	}

	return nil
}

func loadEnv() error {
	// Try to find .env file
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %v", err)
	}

	// Look for .env in current directory and parent directories
	envPaths := []string{
		filepath.Join(currentDir, ".env"),
		filepath.Join(currentDir, "..", ".env"),
		filepath.Join(currentDir, "..", "..", ".env"),
	}

	for _, path := range envPaths {
		if _, err := os.Stat(path); err == nil {
			log.Printf("📄 Found .env file: %s", path)
			// Note: You may need to manually load env vars or use a library like godotenv
			return nil
		}
	}

	log.Println("⚠️  No .env file found, using environment variables")
	return nil
}
