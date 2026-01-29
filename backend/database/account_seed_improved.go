package database

import (
	"fmt"
	"log"
	"strings"
	
	"app-sistem-akuntansi/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SeedAccountsImproved creates initial chart of accounts with improved duplicate handling
// This version includes:
// - Explicit soft-delete filtering
// - Better error messages
// - Duplicate detection and reporting
// - Transaction support for atomic operations
func SeedAccountsImproved(db *gorm.DB) error {
	log.Println("🔒 PRODUCTION MODE: Seeding accounts with improved duplicate handling...")
	
	// Start a transaction for atomic operations
	return db.Transaction(func(tx *gorm.DB) error {
		// First, check for existing duplicates
		if err := checkExistingDuplicates(tx); err != nil {
			return fmt.Errorf("pre-seed duplicate check failed: %v", err)
		}
		
		// Check Company Type settings
		var settings models.Settings
		if err := tx.First(&settings).Error; err != nil {
			return fmt.Errorf("failed to get settings for COA seeding: %v", err)
		}

		if settings.CompanyType == "" {
			log.Println("⚠️  Company Type not set. Skipping COA seeding. Waiting for user selection.")
			return nil
		}
		
		log.Printf("🏭 Seeding COA for Company Type: %s", settings.CompanyType)

		// Get standard accounts based on company type
		accounts := GetStandardCOA(settings.CompanyType)

		// Verify no duplicates in seed data itself
		if err := verifyNoDuplicatesInSeed(accounts); err != nil {
			return err
		}

		accountMap := make(map[string]uint)

		// Create accounts with improved duplicate handling
		for _, account := range accounts {
			accountID, created, err := upsertAccount(tx, account)
			if err != nil {
				return fmt.Errorf("failed to upsert account %s: %v", account.Code, err)
			}
			
			if created {
				log.Printf("✅ Created new account: %s - %s", account.Code, account.Name)
			} else {
				log.Printf("🔒 Account exists: %s - %s (preserving balance)", account.Code, account.Name)
			}
			
			accountMap[account.Code] = accountID
		}

		// Set parent relationships
		if err := setParentRelationships(tx, accountMap); err != nil {
			return fmt.Errorf("failed to set parent relationships: %v", err)
		}

		log.Println("✅ Account seeding completed successfully")
		return nil
	})
}

// checkExistingDuplicates checks for duplicate accounts before seeding
func checkExistingDuplicates(tx *gorm.DB) error {
	var duplicates []struct {
		Code  string
		Count int64
	}
	
	err := tx.Model(&models.Account{}).
		Select("code, COUNT(*) as count").
		Where("deleted_at IS NULL").
		Group("code").
		Having("COUNT(*) > 1").
		Scan(&duplicates).Error
	
	if err != nil {
		return err
	}
	
	if len(duplicates) > 0 {
		log.Println("⚠️  WARNING: Found duplicate accounts in database:")
		for _, dup := range duplicates {
			log.Printf("   - Code %s has %d instances", dup.Code, dup.Count)
		}
		return fmt.Errorf("database has %d duplicate account codes - please clean up first", len(duplicates))
	}
	
	return nil
}

// verifyNoDuplicatesInSeed checks seed data for duplicates
func verifyNoDuplicatesInSeed(accounts []models.Account) error {
	seen := make(map[string]bool)
	duplicates := []string{}
	
	for _, account := range accounts {
		if seen[account.Code] {
			duplicates = append(duplicates, account.Code)
		}
		seen[account.Code] = true
	}
	
	if len(duplicates) > 0 {
		return fmt.Errorf("seed data contains duplicate codes: %v", duplicates)
	}
	
	return nil
}

// upsertAccount creates or updates an account atomically
func upsertAccount(tx *gorm.DB, account models.Account) (uint, bool, error) {
	var existingAccount models.Account
	
	// Always normalize name to UPPERCASE
	normalizedName := strings.ToUpper(account.Name)
	
	// Use FOR UPDATE lock to prevent race conditions
	// Suppress logging for expected "record not found" by using Session
	err := tx.Session(&gorm.Session{Logger: tx.Logger.LogMode(4)}). // 4 = Silent mode
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("code = ?", account.Code).
		Where("deleted_at IS NULL").
		First(&existingAccount).Error
	
	if err == gorm.ErrRecordNotFound {
		// Account doesn't exist, create it
		newAccount := models.Account{
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
		
		if err := tx.Create(&newAccount).Error; err != nil {
			return 0, false, err
		}
		
		return newAccount.ID, true, nil
	} else if err != nil {
		return 0, false, err
	}
	
	// Account exists - check if it's system critical before updating protected fields
	updates := map[string]interface{}{
		"name":        normalizedName,
		"level":       account.Level,
		"is_header":   account.IsHeader,
		"description": account.Description,
	}
	
	// Only update type, category, and is_active if account is NOT system critical
	// to avoid triggering the database trigger that blocks critical account changes
	if !existingAccount.IsSystemCritical {
		updates["type"] = account.Type
		updates["category"] = account.Category
		updates["is_active"] = account.IsActive
	}
	
	if err := tx.Model(&existingAccount).Updates(updates).Error; err != nil {
		return 0, false, err
	}
	
	return existingAccount.ID, false, nil
}

// setParentRelationships sets parent-child relationships for accounts
func setParentRelationships(tx *gorm.DB, accountMap map[string]uint) error {
	// Create a dynamic parent map based on code structure
	parentChildMap := make(map[string]string)
	
	// Iterate over all known account codes to find their parents
	for code := range accountMap {
		var parentCode string
		
		// If code has dash (e.g. 1101-001), parent is before dash (1101)
		if strings.Contains(code, "-") {
			parts := strings.Split(code, "-")
			parentCode = parts[0]
		} else {
			// If 4 digits (e.g. 1100, 1101)
			if len(code) == 4 {
				if code[1:] == "000" { 
					// Level 1: 1000 - No parent in this logic
					continue
				} else if code[2:] == "00" {
					// Level 2: 1100 -> Parent 1000
					parentCode = code[:1] + "000"
				} else {
					// Level 3: 1101 -> Parent 1100
					parentCode = code[:2] + "00"
				}
			}
		}
		
		if parentCode != "" {
			parentChildMap[code] = parentCode
		}
	}

	for childCode, parentCode := range parentChildMap {
		childID, childExists := accountMap[childCode]
		parentID, parentExists := accountMap[parentCode]
		
		if !childExists {
			log.Printf("⚠️  Child account %s not found in map, skipping relationship", childCode)
			continue
		}
		
		if !parentExists {
			log.Printf("⚠️  Parent account %s not found in map, skipping relationship", parentCode)
			continue
		}
		
		err := tx.Model(&models.Account{}).
			Where("id = ?", childID).
			Update("parent_id", parentID).Error
		
		if err != nil {
			return fmt.Errorf("failed to set parent for %s -> %s: %v", childCode, parentCode, err)
		}
	}
	
	return nil
}

// CleanDuplicateAccounts removes duplicate accounts, keeping the oldest one
// WARNING: This should only be run after backing up the database!
func CleanDuplicateAccounts(db *gorm.DB, dryRun bool) error {
	log.Println("🧹 Starting duplicate account cleanup...")
	
	if dryRun {
		log.Println("📋 DRY RUN MODE - No changes will be made")
	}
	
	// Find duplicates
	var duplicates []struct {
		Code string
		IDs  string
	}
	
	err := db.Raw(`
		SELECT 
			code,
			STRING_AGG(id::text, ',') as ids
		FROM accounts
		WHERE deleted_at IS NULL
		GROUP BY code
		HAVING COUNT(*) > 1
	`).Scan(&duplicates).Error
	
	if err != nil {
		return fmt.Errorf("failed to find duplicates: %v", err)
	}
	
	if len(duplicates) == 0 {
		log.Println("✅ No duplicates found!")
		return nil
	}
	
	log.Printf("⚠️  Found %d duplicate account codes", len(duplicates))
	
	for _, dup := range duplicates {
		ids := strings.Split(dup.IDs, ",")
		if len(ids) <= 1 {
			continue
		}
		
		// Keep the first (oldest) ID, delete the rest
		keepID := ids[0]
		deleteIDs := ids[1:]
		
		log.Printf("   Code %s: Keeping ID %s, deleting %v", dup.Code, keepID, deleteIDs)
		
		if !dryRun {
			// Soft delete duplicates
			err := db.Model(&models.Account{}).
				Where("code = ?", dup.Code).
				Where("id IN ?", deleteIDs).
				Update("deleted_at", gorm.Expr("NOW()")).Error
			
			if err != nil {
				return fmt.Errorf("failed to delete duplicates for code %s: %v", dup.Code, err)
			}
		}
	}
	
	if dryRun {
		log.Println("📋 DRY RUN COMPLETE - Run with dryRun=false to actually clean")
	} else {
		log.Println("✅ Duplicate cleanup completed!")
	}
	
	return nil
}
