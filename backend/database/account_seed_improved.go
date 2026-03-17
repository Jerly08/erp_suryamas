package database

import (
	"fmt"
	"log"
	"os"
	"strings"

	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AccountSeedOptions struct {
	UpdateExistingTemplateAccounts  bool
	CleanupObsoleteTemplateAccounts bool
}

const (
	destructiveCOATemplateSyncEnv      = "ALLOW_DESTRUCTIVE_COA_TEMPLATE_SYNC"
	destructiveCOATemplateSyncApproval = "YES_I_UNDERSTAND"
)

// SeedAccountsImproved creates initial chart of accounts with improved duplicate handling
// This version includes:
// - Explicit soft-delete filtering
// - Better error messages
// - Duplicate detection and reporting
// - Transaction support for atomic operations
func SeedAccountsImproved(db *gorm.DB) error {
	return seedAccountsWithOptions(db, AccountSeedOptions{})
}

// SeedAccountsTemplateSync force-syncs the live COA to the current template.
// WARNING: This mode may deactivate or soft-delete accounts outside the template,
// so it must never run automatically at startup.
func SeedAccountsTemplateSync(db *gorm.DB) error {
	if err := requireExplicitTemplateSyncApproval(); err != nil {
		return err
	}

	return seedAccountsWithOptions(db, AccountSeedOptions{
		UpdateExistingTemplateAccounts:  true,
		CleanupObsoleteTemplateAccounts: true,
	})
}

func requireExplicitTemplateSyncApproval() error {
	if os.Getenv(destructiveCOATemplateSyncEnv) == destructiveCOATemplateSyncApproval {
		return nil
	}

	return fmt.Errorf(
		"destructive COA template sync blocked: set %s=%s to run this operation explicitly",
		destructiveCOATemplateSyncEnv,
		destructiveCOATemplateSyncApproval,
	)
}

func seedAccountsWithOptions(db *gorm.DB, opts AccountSeedOptions) error {
	log.Println("🔒 PRODUCTION MODE: Seeding accounts with improved duplicate handling...")
	if !opts.UpdateExistingTemplateAccounts {
		log.Println("🛡️ Safe seed mode enabled: existing COA records will be preserved")
	}
	if !opts.CleanupObsoleteTemplateAccounts {
		log.Println("🛡️ Obsolete-template cleanup disabled: custom/user COA will not be deleted on startup")
	}

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
			accountID, created, err := upsertAccount(tx, account, opts)
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

		// Cleanup accounts from old company type that are NOT in the new template
		if opts.CleanupObsoleteTemplateAccounts {
			if err := cleanupOldTemplateAccounts(tx, accounts); err != nil {
				log.Printf("⚠️  Warning: cleanup old template accounts: %v", err)
				// Non-fatal — don't block seeding
			}
		} else {
			log.Println("⏭️ Skipping obsolete-template cleanup to preserve user-created COA")
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

// verifyNoDuplicatesInSeed checks seed data for duplicates (both code and name)
func verifyNoDuplicatesInSeed(accounts []models.Account) error {
	seenCode := make(map[string]bool)
	seenName := make(map[string]string) // name -> first code that used it
	codeDuplicates := []string{}
	nameDuplicates := []string{}

	for _, account := range accounts {
		// Check code duplicates
		if seenCode[account.Code] {
			codeDuplicates = append(codeDuplicates, account.Code)
		}
		seenCode[account.Code] = true

		// Check name duplicates (skip headers as they may legitimately share names across levels)
		if !account.IsHeader {
			normalizedName := strings.ToUpper(account.Name)
			if existingCode, exists := seenName[normalizedName]; exists {
				nameDuplicates = append(nameDuplicates, fmt.Sprintf("%s and %s both named '%s'", existingCode, account.Code, normalizedName))
			}
			seenName[normalizedName] = account.Code
		}
	}

	if len(codeDuplicates) > 0 {
		return fmt.Errorf("seed data contains duplicate codes: %v", codeDuplicates)
	}

	if len(nameDuplicates) > 0 {
		log.Printf("⚠️  WARNING: Seed data contains duplicate names (different codes, same name):")
		for _, dup := range nameDuplicates {
			log.Printf("   - %s", dup)
		}
		// Warning only, not an error — different codes with same name is allowed but suspicious
	}

	return nil
}

// upsertAccount creates or updates an account atomically.
// In safe seed mode, existing accounts are preserved exactly as-is.
func upsertAccount(tx *gorm.DB, account models.Account, opts AccountSeedOptions) (uint, bool, error) {
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

	updates, shouldUpdate := buildExistingAccountUpdates(existingAccount, account, normalizedName, opts)
	if !shouldUpdate {
		return existingAccount.ID, false, nil
	}

	if err := tx.Model(&existingAccount).Updates(updates).Error; err != nil {
		return 0, false, err
	}

	return existingAccount.ID, false, nil
}

func buildExistingAccountUpdates(existingAccount models.Account, account models.Account, normalizedName string, opts AccountSeedOptions) (map[string]interface{}, bool) {
	if !opts.UpdateExistingTemplateAccounts {
		return nil, false
	}

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

	return updates, true
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
			candidate := parts[0]
			if _, exists := accountMap[candidate]; exists {
				parentCode = candidate
			} else {
				// Fallback: try xx00 then x000
				if len(candidate) == 4 {
					fallback1 := candidate[:2] + "00"
					fallback2 := candidate[:1] + "000"
					if _, exists := accountMap[fallback1]; exists {
						parentCode = fallback1
					} else if _, exists := accountMap[fallback2]; exists {
						parentCode = fallback2
					}
				}
			}
		} else {
			// If 4 digits (e.g. 1100, 1101)
			if len(code) == 4 {
				if code[1:] == "000" {
					// Level 1: 1000 - No parent
					continue
				} else if code[2:] == "00" {
					// Level 2: 1100 -> Parent 1000
					parentCode = code[:1] + "000"
				} else {
					// Level 3: 1101 -> try Parent xx00 first, fallback to x000
					candidate := code[:2] + "00"
					if _, exists := accountMap[candidate]; exists {
						parentCode = candidate
					} else {
						// Fallback: parent is root level (e.g., 3101 -> 3000 since 3100 doesn't exist)
						fallback := code[:1] + "000"
						if _, exists := accountMap[fallback]; exists {
							parentCode = fallback
							log.Printf("ℹ️  Account %s: intermediate parent %s not found, using root %s", code, candidate, fallback)
						}
					}
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

// cleanupOldTemplateAccounts finds accounts in the database that are NOT part of
// the current company type template and either soft-deletes them (if unused) or
// deactivates them (if they have existing transactions/journals).
// This ensures that switching company types (e.g., Trading → Service) removes
// accounts like HPP/Inventory that are irrelevant to the new type.
//
// ✅ FIX: Skip user-created accounts (e.g., Bank OCBC 1101-005) — only target
// accounts that were previously seeded by a different template.
func cleanupOldTemplateAccounts(tx *gorm.DB, templateAccounts []models.Account) error {
	log.Println("🧹 Checking for accounts from previous company type to clean up...")

	// Build a set of template account codes for fast lookup
	templateCodeSet := make(map[string]bool, len(templateAccounts))
	for _, acc := range templateAccounts {
		templateCodeSet[acc.Code] = true
	}

	// Get all active (non-deleted) accounts from the database
	var existingAccounts []models.Account
	if err := tx.Where("deleted_at IS NULL").Find(&existingAccounts).Error; err != nil {
		return fmt.Errorf("failed to fetch existing accounts: %v", err)
	}

	// Build a set of account IDs that are linked to cash_banks
	// These are definitely user-configured and should NEVER be deactivated
	cashBankAccountIDs := make(map[uint]bool)
	type CashBankRef struct {
		AccountID uint
	}
	var cashBankRefs []CashBankRef
	tx.Raw("SELECT DISTINCT account_id FROM cash_banks WHERE deleted_at IS NULL AND account_id IS NOT NULL").Scan(&cashBankRefs)
	for _, ref := range cashBankRefs {
		cashBankAccountIDs[ref.AccountID] = true
	}

	var softDeleted, deactivated, skipped int

	for _, existing := range existingAccounts {
		// Skip if account IS in the current template
		if templateCodeSet[existing.Code] {
			continue
		}

		// ✅ FIX: Skip system-critical accounts
		if existing.IsSystemCritical {
			continue
		}

		// ✅ FIX: Skip accounts linked to cash_banks (user-configured bank accounts)
		if cashBankAccountIDs[existing.ID] {
			log.Printf("   ⏭️  Skipping account %s (%s) — linked to cash_bank entry", existing.Code, existing.Name)
			skipped++
			continue
		}

		// ✅ FIX: Skip accounts whose PARENT is linked to cash_banks
		// (e.g., 1101-005 whose parent 1101 might be in the template but 1101-005 is user-added)
		if existing.ParentID != nil && cashBankAccountIDs[*existing.ParentID] {
			log.Printf("   ⏭️  Skipping account %s (%s) — parent linked to cash_bank", existing.Code, existing.Name)
			skipped++
			continue
		}

		// ✅ FIX: Skip any account that has a non-zero balance and journal entries
		// These are actively used accounts and should not be deactivated
		// Only deactivate truly orphaned template accounts (zero balance, no recent usage)
		if existing.Balance != 0 {
			var ssotLineCount int64
			tx.Model(&models.SSOTJournalLine{}).Where("account_id = ?", existing.ID).Count(&ssotLineCount)
			if ssotLineCount > 0 {
				log.Printf("   ⏭️  Skipping account %s (%s) — has balance=%.2f and %d journal lines (actively used)",
					existing.Code, existing.Name, existing.Balance, ssotLineCount)
				skipped++
				continue
			}
		}

		// Check if account has any journal lines referencing it
		var journalLineCount int64
		tx.Model(&models.JournalLine{}).Where("account_id = ?", existing.ID).Count(&journalLineCount)

		// Check if account has any SSOT journal lines referencing it (by account_id)
		var ssotLineCount int64
		tx.Model(&models.SSOTJournalLine{}).Where("account_id = ?", existing.ID).Count(&ssotLineCount)

		// Check if account has any transactions
		var transactionCount int64
		tx.Model(&models.Transaction{}).Where("account_id = ?", existing.ID).Count(&transactionCount)

		hasData := journalLineCount > 0 || ssotLineCount > 0 || transactionCount > 0
		hasBalance := existing.Balance != 0

		if hasData || hasBalance {
			// Account has transactions or balance — DON'T delete, just deactivate
			if existing.IsActive {
				if err := tx.Model(&existing).Update("is_active", false).Error; err != nil {
					log.Printf("   ⚠️  Failed to deactivate account %s (%s): %v", existing.Code, existing.Name, err)
				} else {
					log.Printf("   🔒 Deactivated account %s (%s) — has %d journal lines, %d SSOT lines, %d transactions, balance=%.2f",
						existing.Code, existing.Name, journalLineCount, ssotLineCount, transactionCount, existing.Balance)
					deactivated++
				}
			}
		} else {
			// Account has no data and no balance — safe to soft delete
			if err := tx.Delete(&existing).Error; err != nil {
				log.Printf("   ⚠️  Failed to soft-delete account %s (%s): %v", existing.Code, existing.Name, err)
			} else {
				log.Printf("   🗑️  Soft-deleted account %s (%s) — not in current template, no data",
					existing.Code, existing.Name)
				softDeleted++
			}
		}
	}

	if softDeleted > 0 || deactivated > 0 || skipped > 0 {
		log.Printf("🧹 Cleanup complete: %d soft-deleted, %d deactivated, %d skipped (user-created/active)", softDeleted, deactivated, skipped)
	} else {
		log.Println("✅ No old template accounts to clean up")
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
