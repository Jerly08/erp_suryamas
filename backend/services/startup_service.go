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
// NOTE: Only safe, read-only or idempotent structural tasks should run here.
// Do NOT add tasks that modify SSOT journal entries or override account balances.
func (s *StartupService) RunStartupTasks() {
	log.Println("🚀 Running startup tasks...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Task 1: Fix Account Header Status (structural flag only, no balance changes)
	s.fixAccountHeaderStatus(ctx)

	// Task 2: Validate Account Hierarchy (read-only validation)
	s.validateAccountHierarchy(ctx)

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
