package database

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

// SeedAccounts creates initial chart of accounts
func SeedAccounts(db *gorm.DB) error {
	log.Println("🌱 Starting account seeding (idempotent mode)...")
	log.Println("   Note: Accounts from migrations will be preserved, not recreated")
	// Normalize all account names to UPPERCASE to ensure consistency
	accounts := []models.Account{
		// ASSETS (1xxx)
		{Code: "1000", Name: strings.ToUpper("ASSETS"), Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 1, IsHeader: true, IsActive: true},
		{Code: "1100", Name: strings.ToUpper("CURRENT ASSETS"), Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 2, IsHeader: true, IsActive: true},

		// FIXED ASSETS - Header only
		{Code: "1200", Name: strings.ToUpper("FIXED ASSETS"), Type: models.AccountTypeAsset, Category: models.CategoryFixedAsset, Level: 2, IsHeader: true, IsActive: true},

		// LIABILITIES (2xxx)
		{Code: "2000", Name: strings.ToUpper("LIABILITIES"), Type: models.AccountTypeLiability, Category: models.CategoryCurrentLiability, Level: 1, IsHeader: true, IsActive: true},
		{Code: "2100", Name: strings.ToUpper("CURRENT LIABILITIES"), Type: models.AccountTypeLiability, Category: models.CategoryCurrentLiability, Level: 2, IsHeader: true, IsActive: true},

		// EQUITY (3xxx)
		{Code: "3000", Name: strings.ToUpper("EQUITY"), Type: models.AccountTypeEquity, Category: models.CategoryEquity, Level: 1, IsHeader: true, IsActive: true},

		// REVENUE (4xxx)
		{Code: "4000", Name: strings.ToUpper("REVENUE"), Type: models.AccountTypeRevenue, Category: models.CategoryOperatingRevenue, Level: 1, IsHeader: true, IsActive: true},

		// EXPENSES (5xxx)
		{Code: "5000", Name: strings.ToUpper("EXPENSES"), Type: models.AccountTypeExpense, Category: models.CategoryOperatingExpense, Level: 1, IsHeader: true, IsActive: true},
		{Code: "6000", Name: strings.ToUpper("OPERATING EXPENSES"), Type: models.AccountTypeExpense, Category: models.CategoryOperatingExpense, Level: 1, IsHeader: true, IsActive: true},
	}

	// Set parent relationships based on account hierarchy
	accountMap := make(map[string]uint)

	// First pass: create accounts to get IDs (avoid DB errors in logs by pre-checking)
	for _, account := range accounts {
		var existingAccount models.Account
		// Normalize name to UPPERCASE consistently
		normalizedName := strings.ToUpper(account.Name)

		findErr := db.Where("code = ? AND deleted_at IS NULL", account.Code).First(&existingAccount).Error
		if findErr == nil {
			// Already exists -> skip without causing an INSERT error
			log.Printf("   ⏭️  Skipping account %s - already exists", account.Code)
			accountMap[account.Code] = existingAccount.ID
			continue
		}
		if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to lookup account %s: %v", account.Code, findErr)
		}

		// Not found -> create
		toCreate := models.Account{
			Code:        account.Code,
			Name:        normalizedName,
			Type:        account.Type,
			Category:    account.Category,
			Level:       account.Level,
			IsHeader:    account.IsHeader,
			IsActive:    account.IsActive,
			Balance:     account.Balance,
			Description: account.Description,
		}
		if err := db.Create(&toCreate).Error; err != nil {
			return fmt.Errorf("failed to create account %s: %v", account.Code, err)
		}
		log.Printf("✅ Created account: %s - %s", toCreate.Code, toCreate.Name)
		accountMap[account.Code] = toCreate.ID
	}

	// Define parent-child relationships for SKELETON only
	parentChildMap := map[string]string{
		"1100": "1000", // CURRENT ASSETS -> ASSETS
		"1200": "1000", // FIXED ASSETS -> ASSETS (Changed parent from 1100 to 1000 in cleanup)
		"2100": "2000", // CURRENT LIABILITIES -> LIABILITIES
		"6000": "5000", // OPERATING EXPENSES -> EXPENSES
	}

	// Second pass: set parent relationships
	for childCode, parentCode := range parentChildMap {
		if childID, childExists := accountMap[childCode]; childExists {
			if parentID, parentExists := accountMap[parentCode]; parentExists {
				if err := db.Model(&models.Account{}).Where("id = ?", childID).Update("parent_id", parentID).Error; err != nil {
					return err
				}
			}
		}
	}

	// Count how many accounts exist vs created
	var totalAccounts int64
	db.Model(&models.Account{}).Where("deleted_at IS NULL").Count(&totalAccounts)

	log.Printf("✅ Account seeding completed: %d accounts ready", totalAccounts)

	// Capitalize existing account names that are not already uppercase
	if err := CapitalizeExistingAccounts(db); err != nil {
		log.Printf("⚠️  Warning: Failed to capitalize existing accounts: %v", err)
	}

	return nil
}

// FixAccountHierarchies fixes incorrect account hierarchies in existing databases
// NOTE: Legacy fixes removed as they referred to deleted accounts.
func FixAccountHierarchies(db *gorm.DB) error {
	log.Println("🔧 Fixing account hierarchies (Skeleton check only)...")
	// Legacy fixes cleared.
	return nil
}

// CapitalizeExistingAccounts converts all existing account names to UPPERCASE
// This ensures consistency across all accounts, including those created before this update
func CapitalizeExistingAccounts(db *gorm.DB) error {
	log.Println("🔤 Capitalizing existing account names to ensure consistency...")

	// Get all non-deleted accounts
	var accounts []models.Account
	if err := db.Where("deleted_at IS NULL").Find(&accounts).Error; err != nil {
		return fmt.Errorf("failed to fetch accounts: %v", err)
	}

	updatedCount := 0
	for _, account := range accounts {
		// Check if name needs capitalization
		upperName := strings.ToUpper(account.Name)
		if account.Name != upperName {
			// Update to UPPERCASE
			if err := db.Model(&account).Update("name", upperName).Error; err != nil {
				log.Printf("⚠️  Failed to capitalize account %s (%s): %v", account.Code, account.Name, err)
				continue
			}
			log.Printf("✅ Capitalized: %s - %s → %s", account.Code, account.Name, upperName)
			updatedCount++
		}
	}

	if updatedCount > 0 {
		log.Printf("✅ Capitalized %d account names", updatedCount)
	} else {
		log.Println("✅ All account names are already capitalized")
	}

	return nil
}
