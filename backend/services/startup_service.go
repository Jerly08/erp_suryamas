package services

import (
	"context"
	"log"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"

	"gorm.io/gorm"
)

type StartupService struct {
	DB          *gorm.DB
	AccountRepo *repositories.AccountRepo
}

func NewStartupService(db *gorm.DB) *StartupService {
	return &StartupService{
		DB:          db,
		AccountRepo: repositories.NewAccountRepo(db),
	}
}

// RunStartupTasks runs all necessary startup tasks
func (s *StartupService) RunStartupTasks() {
	log.Println("🚀 Running startup tasks...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Task 1: Fix Account Header Status
	s.fixAccountHeaderStatus(ctx)

	// Task 2: Validate Account Hierarchy
	s.validateAccountHierarchy(ctx)

	// Task 3: Fix SaleItems with revenue posted to header account 4101
	s.fixRevenueHeaderPostings()

	// Task 4: Recalculate equity accounts (Modal Disetor 3101) from journal entries
	s.fixEquityBalancesFromJournals()

	log.Println("✅ Startup tasks completed successfully")
}

// fixAccountHeaderStatus fixes account header status on startup
func (s *StartupService) fixAccountHeaderStatus(ctx context.Context) {
	log.Println("🔧 Fixing account header status...")

	startTime := time.Now()

	err := s.AccountRepo.FixAccountHeaderStatus(ctx)
	if err != nil {
		log.Printf("❌ Failed to fix account header status: %v", err)
		return
	}

	duration := time.Since(startTime)
	log.Printf("✅ Account header status fixed successfully in %v", duration)
}

// validateAccountHierarchy validates the account hierarchy integrity
func (s *StartupService) validateAccountHierarchy(ctx context.Context) {
	log.Println("🔍 Validating account hierarchy...")

	startTime := time.Now()

	// Get hierarchy to validate
	hierarchy, err := s.AccountRepo.GetHierarchy(ctx)
	if err != nil {
		log.Printf("❌ Failed to validate account hierarchy: %v", err)
		return
	}

	totalAccounts := s.countAccountsInHierarchy(hierarchy)
	duration := time.Since(startTime)

	log.Printf("✅ Account hierarchy validated successfully - %d accounts processed in %v", totalAccounts, duration)
}

// countAccountsInHierarchy recursively counts accounts in hierarchy
func (s *StartupService) countAccountsInHierarchy(accounts []models.Account) int {
	count := len(accounts)
	for _, account := range accounts {
		count += s.countAccountsInHierarchy(account.Children)
	}
	return count
}

// GetStartupStatus returns current startup status information
func (s *StartupService) GetStartupStatus(ctx context.Context) (map[string]interface{}, error) {
	status := make(map[string]interface{})

	// Check account hierarchy health
	hierarchy, err := s.AccountRepo.GetHierarchy(ctx)
	if err != nil {
		status["hierarchy_status"] = "error"
		status["hierarchy_error"] = err.Error()
	} else {
		status["hierarchy_status"] = "healthy"
		status["total_accounts"] = s.countAccountsInHierarchy(hierarchy)
	}

	// Check header status consistency
	var inconsistentHeaders int64
	err = s.DB.WithContext(ctx).Raw(`
		SELECT COUNT(*) FROM (
			-- Accounts that should be headers but aren't
			SELECT a.id FROM accounts a 
			WHERE EXISTS (
				SELECT 1 FROM accounts child 
				WHERE child.parent_id = a.id AND child.deleted_at IS NULL
			) AND a.is_header = false AND a.deleted_at IS NULL
			
			UNION
			
			-- Accounts that are headers but shouldn't be
			SELECT a.id FROM accounts a 
			WHERE a.is_header = true 
				AND NOT EXISTS (
					SELECT 1 FROM accounts child 
					WHERE child.parent_id = a.id AND child.deleted_at IS NULL
				) 
				AND a.deleted_at IS NULL
		) AS inconsistent
	`).Scan(&inconsistentHeaders).Error

	if err != nil {
		status["header_consistency"] = "error"
		status["header_error"] = err.Error()
	} else {
		status["header_consistency"] = "healthy"
		status["inconsistent_headers"] = inconsistentHeaders
	}

	status["last_checked"] = time.Now().Format(time.RFC3339)

	return status, nil
}

// fixRevenueHeaderPostings fixes revenue data that was incorrectly posted to
// header account 4101 instead of leaf account 4101-001.
// This fixes: SaleItems, unified_journal_lines, simple_ssot_journal_items, and COA balances.
// This fix is idempotent — it only updates records that need fixing.
func (s *StartupService) fixRevenueHeaderPostings() {
	log.Println("🔧 Checking revenue data posted to header account 4101...")

	// Step 1: Find the header account (4101) and leaf account (4101-001)
	var headerAccount models.Account
	if err := s.DB.Where("code = ? AND deleted_at IS NULL", "4101").First(&headerAccount).Error; err != nil {
		log.Printf("ℹ️ Account 4101 not found, skipping revenue header fix")
		return
	}

	var leafAccount models.Account
	if err := s.DB.Where("code = ? AND deleted_at IS NULL", "4101-001").First(&leafAccount).Error; err != nil {
		log.Printf("⚠️ Leaf account 4101-001 not found, cannot fix revenue header postings")
		return
	}

	// Verify 4101 is actually a header
	if !headerAccount.IsHeader {
		log.Printf("ℹ️ Account 4101 is not a header, no fix needed")
		return
	}

	fixApplied := false

	// Step 2: Fix SaleItems with RevenueAccountID pointing to header
	result := s.DB.Model(&models.SaleItem{}).
		Where("revenue_account_id = ?", headerAccount.ID).
		Update("revenue_account_id", leafAccount.ID)
	if result.Error != nil {
		log.Printf("⚠️ Failed to fix SaleItem revenue accounts: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("✅ Fixed %d SaleItems: RevenueAccountID %d → %d", result.RowsAffected, headerAccount.ID, leafAccount.ID)
		fixApplied = true
	}

	// Step 3: Fix unified_journal_lines (SSOT journal) — revenue lines for SALE transactions
	ujlResult := s.DB.Exec(`
		UPDATE unified_journal_lines 
		SET account_id = ? 
		WHERE account_id = ? 
		AND credit_amount > 0 
		AND journal_id IN (
			SELECT id FROM unified_journal_ledger 
			WHERE source_type = 'SALE' AND status = 'POSTED'
		)
	`, leafAccount.ID, headerAccount.ID)
	if ujlResult.Error != nil {
		log.Printf("⚠️ Failed to fix unified_journal_lines: %v", ujlResult.Error)
	} else if ujlResult.RowsAffected > 0 {
		log.Printf("✅ Fixed %d unified_journal_lines: account_id %d → %d", ujlResult.RowsAffected, headerAccount.ID, leafAccount.ID)
		fixApplied = true
	}

	// Step 4: Fix simple_ssot_journal_items (legacy V2 journal) — revenue lines for SALES transactions
	ssotResult := s.DB.Exec(`
		UPDATE simple_ssot_journal_items 
		SET account_id = ?, account_code = ?, account_name = ?
		WHERE account_id = ? 
		AND credit > 0 
		AND journal_id IN (
			SELECT id FROM simple_ssot_journals 
			WHERE transaction_type = 'SALES'
		)
	`, leafAccount.ID, leafAccount.Code, leafAccount.Name, headerAccount.ID)
	if ssotResult.Error != nil {
		log.Printf("⚠️ Failed to fix simple_ssot_journal_items: %v", ssotResult.Error)
	} else if ssotResult.RowsAffected > 0 {
		log.Printf("✅ Fixed %d simple_ssot_journal_items: account_id %d → %d", ssotResult.RowsAffected, headerAccount.ID, leafAccount.ID)
		fixApplied = true
	}

	if !fixApplied {
		log.Printf("ℹ️ No revenue data pointing to header 4101 found, no fix needed")
		return
	}

	// Step 5: Recalculate leaf account 4101-001 balance from SSOT journal entries
	var newBalance float64
	err := s.DB.Raw(`
		SELECT COALESCE(SUM(ujl.debit_amount) - SUM(ujl.credit_amount), 0)
		FROM unified_journal_lines ujl
		JOIN unified_journal_ledger uj ON ujl.journal_id = uj.id
		WHERE ujl.account_id = ? AND uj.status = 'POSTED'
	`, leafAccount.ID).Scan(&newBalance).Error

	if err != nil {
		log.Printf("⚠️ Failed to recalculate leaf balance from SSOT: %v", err)
	} else {
		// For REVENUE accounts: balance = -(debit - credit) = credit - debit
		revenueBalance := -newBalance

		if err := s.DB.Model(&models.Account{}).
			Where("id = ?", leafAccount.ID).
			Update("balance", revenueBalance).Error; err != nil {
			log.Printf("⚠️ Failed to update leaf balance: %v", err)
		} else {
			log.Printf("✅ Updated account 4101-001 balance to %.2f", revenueBalance)
		}
	}

	// Step 6: Recalculate header 4101 balance as SUM(children)
	var headerBalance float64
	s.DB.Raw(`
		SELECT COALESCE(SUM(balance), 0)
		FROM accounts 
		WHERE parent_id = ? AND deleted_at IS NULL
	`, headerAccount.ID).Scan(&headerBalance)

	if err := s.DB.Model(&models.Account{}).
		Where("id = ?", headerAccount.ID).
		Update("balance", headerBalance).Error; err != nil {
		log.Printf("⚠️ Failed to update header balance: %v", err)
	} else {
		log.Printf("✅ Updated account 4101 header balance to %.2f (sum of children)", headerBalance)
	}

	// Step 7: Propagate upward — update parent of 4101 (which is 4000 PENDAPATAN)
	if headerAccount.ParentID != nil {
		var grandparentBalance float64
		s.DB.Raw(`
			SELECT COALESCE(SUM(balance), 0)
			FROM accounts 
			WHERE parent_id = ? AND deleted_at IS NULL
		`, *headerAccount.ParentID).Scan(&grandparentBalance)

		if err := s.DB.Model(&models.Account{}).
			Where("id = ?", *headerAccount.ParentID).
			Update("balance", grandparentBalance).Error; err != nil {
			log.Printf("⚠️ Failed to update grandparent balance: %v", err)
		} else {
			log.Printf("✅ Updated parent account (4000) balance to %.2f", grandparentBalance)
		}
	}

	log.Println("✅ Revenue header postings fix completed")
}

// fixEquityBalancesFromJournals recalculates equity account (3101 Modal Disetor) balance
// from SSOT journal entries to fix duplicate opening balance issues.
// This is idempotent — the balance is always recalculated from the source of truth (journals).
func (s *StartupService) fixEquityBalancesFromJournals() {
	log.Println("🔧 Recalculating equity account balances from SSOT journals...")

	// Find account 3101 MODAL DISETOR
	var equityAccount models.Account
	if err := s.DB.Where("code = ? AND deleted_at IS NULL", "3101").First(&equityAccount).Error; err != nil {
		log.Printf("ℹ️ Account 3101 not found, skipping equity balance fix")
		return
	}

	// Recalculate balance from unified_journal_lines (SSOT)
	// For EQUITY accounts: Credit increases balance, Debit decreases
	// balance = SUM(credit) - SUM(debit) = -(SUM(debit) - SUM(credit))
	var rawBalance float64
	err := s.DB.Raw(`
		SELECT COALESCE(SUM(ujl.credit_amount) - SUM(ujl.debit_amount), 0)
		FROM unified_journal_lines ujl
		JOIN unified_journal_ledger uj ON ujl.journal_id = uj.id
		WHERE ujl.account_id = ? AND uj.status = 'POSTED'
	`, equityAccount.ID).Scan(&rawBalance).Error

	if err != nil {
		log.Printf("⚠️ Failed to recalculate equity balance from SSOT: %v", err)
		return
	}

	// Only fix if the balance is different
	if rawBalance == equityAccount.Balance {
		log.Printf("ℹ️ Account 3101 balance is already correct: %.2f", equityAccount.Balance)
		return
	}

	oldBalance := equityAccount.Balance
	if err := s.DB.Model(&models.Account{}).
		Where("id = ?", equityAccount.ID).
		Update("balance", rawBalance).Error; err != nil {
		log.Printf("⚠️ Failed to update equity balance: %v", err)
		return
	}

	log.Printf("✅ Fixed account 3101 MODAL DISETOR balance: %.2f → %.2f (recalculated from journals)",
		oldBalance, rawBalance)

	// Propagate up to parent (3000 EKUITAS)
	if equityAccount.ParentID != nil {
		var parentBalance float64
		s.DB.Raw(`
			SELECT COALESCE(SUM(balance), 0)
			FROM accounts 
			WHERE parent_id = ? AND deleted_at IS NULL
		`, *equityAccount.ParentID).Scan(&parentBalance)

		if err := s.DB.Model(&models.Account{}).
			Where("id = ?", *equityAccount.ParentID).
			Update("balance", parentBalance).Error; err != nil {
			log.Printf("⚠️ Failed to update parent balance: %v", err)
		} else {
			log.Printf("✅ Updated parent account (3000) balance to %.2f", parentBalance)
		}
	}
}
