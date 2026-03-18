package services

import (
	"fmt"
	"log"
	"time"

	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

type BalanceSyncService struct {
	db *gorm.DB
}

func NewBalanceSyncService(db *gorm.DB) *BalanceSyncService {
	return &BalanceSyncService{db: db}
}

// SyncAccountBalancesFromSSOT automatically synchronizes account balances from SSOT journal entries
func (s *BalanceSyncService) SyncAccountBalancesFromSSOT() error {
	log.Println("🔄 Starting automatic SSOT balance synchronization...")

	// 1. Calculate balances from SSOT journal lines
	var accountUpdates []struct {
		AccountID         uint    `json:"account_id"`
		CalculatedBalance float64 `json:"calculated_balance"`
	}

	err := s.db.Raw(`
		SELECT 
			account_id,
			SUM(debit_amount) - SUM(credit_amount) as calculated_balance
		FROM unified_journal_lines ujl
		JOIN unified_journal_ledger uj ON ujl.journal_id = uj.id
		WHERE uj.status = 'POSTED' AND uj.deleted_at IS NULL
		GROUP BY account_id
	`).Scan(&accountUpdates).Error

	if err != nil {
		return fmt.Errorf("error calculating SSOT balances: %v", err)
	}

	// 2. Update account balances that have SSOT transactions
	updatedCount := 0
	for _, update := range accountUpdates {
		err = s.db.Model(&models.Account{}).
			Where("id = ?", update.AccountID).
			Update("balance", update.CalculatedBalance).Error

		if err != nil {
			log.Printf("Warning: Failed to update account %d balance: %v", update.AccountID, err)
		} else {
			updatedCount++
		}
	}

	// 3. Update parent account balances recursively
	err = s.syncParentAccountBalances()
	if err != nil {
		log.Printf("Warning: Failed to update parent balances: %v", err)
	}

	log.Printf("✅ SSOT balance sync completed: %d accounts updated", updatedCount)
	return nil
}

// syncParentAccountBalances updates header account balances from their children
func (s *BalanceSyncService) syncParentAccountBalances() error {
	// Update parent accounts in multiple passes to handle deep hierarchies
	for pass := 0; pass < 5; pass++ {
		err := s.db.Exec(`
			UPDATE accounts 
			SET balance = (
				SELECT COALESCE(SUM(child.balance), 0)
				FROM accounts child 
				WHERE child.parent_id = accounts.id AND child.deleted_at IS NULL
			)
			WHERE is_header = true AND deleted_at IS NULL
		`).Error

		if err != nil {
			return fmt.Errorf("error updating parent balances (pass %d): %v", pass+1, err)
		}
	}

	return nil
}

// RecalculateAllParentBalances is a public wrapper to recalculate all header account balances
func (s *BalanceSyncService) RecalculateAllParentBalances() (int64, error) {
	log.Println("🔄 Recalculating all parent/header account balances...")

	// Count header accounts before update
	var headerCount int64
	s.db.Model(&models.Account{}).Where("is_header = true AND deleted_at IS NULL").Count(&headerCount)

	err := s.syncParentAccountBalances()
	if err != nil {
		return 0, err
	}

	log.Printf("✅ Recalculated balances for %d header accounts", headerCount)
	return headerCount, nil
}

// AutoSyncAfterJournalPost automatically syncs balances after journal posting
func (s *BalanceSyncService) AutoSyncAfterJournalPost(journalID uint) error {
	log.Printf("🔄 Auto-syncing balances after journal %d posting...", journalID)

	// Get affected accounts from this journal
	var affectedAccounts []uint
	err := s.db.Raw(`
		SELECT DISTINCT account_id 
		FROM unified_journal_lines 
		WHERE journal_id = ?
	`, journalID).Scan(&affectedAccounts).Error

	if err != nil {
		return fmt.Errorf("error finding affected accounts: %v", err)
	}

	// Update only affected accounts and their parents
	for _, accountID := range affectedAccounts {
		// Calculate new balance for this account
		var newBalance float64
		err = s.db.Raw(`
			SELECT COALESCE(SUM(debit_amount) - SUM(credit_amount), 0)
			FROM unified_journal_lines ujl
			JOIN unified_journal_ledger uj ON ujl.journal_id = uj.id
			WHERE ujl.account_id = ? AND uj.status = 'POSTED' AND uj.deleted_at IS NULL
		`, accountID).Scan(&newBalance).Error

		if err != nil {
			log.Printf("Warning: Failed to calculate balance for account %d: %v", accountID, err)
			continue
		}

		// Update the account balance
		err = s.db.Model(&models.Account{}).
			Where("id = ?", accountID).
			Update("balance", newBalance).Error

		if err != nil {
			log.Printf("Warning: Failed to update account %d balance: %v", accountID, err)
		}

		// Update parent chain
		s.updateParentChain(accountID)
	}

	log.Printf("✅ Auto-sync completed for journal %d", journalID)
	return nil
}

// updateParentChain updates the parent account chain for a given account
func (s *BalanceSyncService) updateParentChain(accountID uint) {
	var parentID *uint

	// Get parent ID
	s.db.Raw("SELECT parent_id FROM accounts WHERE id = ?", accountID).Scan(&parentID)

	// If has parent, update parent and continue up the chain
	if parentID != nil {
		// Calculate parent balance as sum of children
		var parentBalance float64
		s.db.Raw(`
			SELECT COALESCE(SUM(balance), 0)
			FROM accounts 
			WHERE parent_id = ? AND deleted_at IS NULL
		`, *parentID).Scan(&parentBalance)

		// Update parent balance
		err := s.db.Model(&models.Account{}).
			Where("id = ?", *parentID).
			Update("balance", parentBalance).Error

		if err != nil {
			log.Printf("Warning: Failed to update parent account %d balance: %v", *parentID, err)
		} else {
			log.Printf("✅ Updated parent account %d balance to %.2f", *parentID, parentBalance)
		}

		// Recursively update grandparent chain
		s.updateParentChain(*parentID)
	}
}

// VerifyBalanceIntegrity checks if all balances are consistent
func (s *BalanceSyncService) VerifyBalanceIntegrity() (bool, error) {
	log.Println("🔍 Verifying balance integrity...")

	var mismatches []struct {
		AccountID   uint    `json:"account_id"`
		AccountCode string  `json:"account_code"`
		CoaBalance  float64 `json:"coa_balance"`
		SsotBalance float64 `json:"ssot_balance"`
		Difference  float64 `json:"difference"`
	}

	err := s.db.Raw(`
		SELECT 
			a.id as account_id,
			a.code as account_code,
			a.balance as coa_balance,
			COALESCE(
				(SELECT SUM(ujl.debit_amount) - SUM(ujl.credit_amount)
				 FROM unified_journal_lines ujl 
				 JOIN unified_journal_ledger uj ON ujl.journal_id = uj.id
				 WHERE ujl.account_id = a.id AND uj.status = 'POSTED' AND uj.deleted_at IS NULL),
				0
			) as ssot_balance,
			a.balance - COALESCE(
				(SELECT SUM(ujl.debit_amount) - SUM(ujl.credit_amount)
				 FROM unified_journal_lines ujl 
				 JOIN unified_journal_ledger uj ON ujl.journal_id = uj.id
				 WHERE ujl.account_id = a.id AND uj.status = 'POSTED' AND uj.deleted_at IS NULL),
				0
			) as difference
		FROM accounts a
		WHERE a.id IN (
			SELECT DISTINCT account_id 
			FROM unified_journal_lines
		)
		AND ABS(a.balance - COALESCE(
			(SELECT SUM(ujl.debit_amount) - SUM(ujl.credit_amount)
			 FROM unified_journal_lines ujl 
			 JOIN unified_journal_ledger uj ON ujl.journal_id = uj.id
			 WHERE ujl.account_id = a.id AND uj.status = 'POSTED'),
			0
		)) > 0.01
	`).Scan(&mismatches).Error

	if err != nil {
		return false, fmt.Errorf("error checking balance integrity: %v", err)
	}

	if len(mismatches) > 0 {
		log.Printf("❌ Found %d balance mismatches:", len(mismatches))
		for _, mismatch := range mismatches {
			log.Printf("  Account %s: COA=%.2f, SSOT=%.2f, Diff=%.2f",
				mismatch.AccountCode, mismatch.CoaBalance, mismatch.SsotBalance, mismatch.Difference)
		}
		return false, nil
	}

	log.Println("✅ All balances are consistent")
	return true, nil
}

// SchedulePeriodicSync runs periodic balance synchronization
func (s *BalanceSyncService) SchedulePeriodicSync(intervalMinutes int) {
	ticker := time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		log.Printf("🔄 Running periodic balance sync (every %d minutes)...", intervalMinutes)

		// Verify integrity first
		isConsistent, err := s.VerifyBalanceIntegrity()
		if err != nil {
			log.Printf("Error during periodic integrity check: %v", err)
			continue
		}

		// If inconsistent, run full sync
		if !isConsistent {
			log.Println("⚠️ Inconsistencies detected, running full balance sync...")
			err = s.SyncAccountBalancesFromSSOT()
			if err != nil {
				log.Printf("Error during periodic sync: %v", err)
			}
		}
	}
}
