package services

import (
	"errors"
	"fmt"
	"log"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"gorm.io/gorm"
)

// AutoBalanceSyncService handles automatic balance synchronization between cash banks and COA accounts
type AutoBalanceSyncService struct {
	db          *gorm.DB
	accountRepo repositories.AccountRepository
}

// NewAutoBalanceSyncService creates a new AutoBalanceSyncService instance
func NewAutoBalanceSyncService(db *gorm.DB, accountRepo repositories.AccountRepository) *AutoBalanceSyncService {
	return &AutoBalanceSyncService{
		db:          db,
		accountRepo: accountRepo,
	}
}

// SyncCashBankToAccount ensures cash bank balance matches its linked COA account
func (s *AutoBalanceSyncService) SyncCashBankToAccount(cashBankID uint) error {
	log.Printf("🔄 Starting balance sync for cash bank ID: %d", cashBankID)

	var cashBank models.CashBank
	if err := s.db.Where("id = ? AND deleted_at IS NULL", cashBankID).First(&cashBank).Error; err != nil {
		return fmt.Errorf("cash bank not found: %w", err)
	}

	// Recalculate native and book balances from transaction history.
	var totals struct {
		NativeBalance float64
		BookBalance   float64
	}
	balanceQuery := `
		SELECT
			COALESCE(SUM(amount), 0) AS native_balance,
			COALESCE(SUM(book_amount_id_r), 0) AS book_balance
		FROM cash_bank_transactions
		WHERE cash_bank_id = ? AND deleted_at IS NULL
	`

	if err := s.db.Raw(balanceQuery, cashBankID).Scan(&totals).Error; err != nil {
		return fmt.Errorf("failed to calculate actual balance: %w", err)
	}

	actualBalance := totals.NativeBalance
	actualBookBalance := actualBalance
	if cashBank.IsForeignCurrency() {
		actualBookBalance = totals.BookBalance
	}

	// Start transaction for atomic updates
	tx := s.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	cashBankUpdates := map[string]interface{}{
		"balance":    actualBalance,
		"updated_at": time.Now(),
	}
	if cashBank.IsForeignCurrency() {
		cashBankUpdates["book_balance_id_r"] = actualBookBalance
		if actualBalance > 0 && actualBookBalance > 0 {
			cashBankUpdates["last_exchange_rate"] = actualBookBalance / actualBalance
		}
	}

	// Update cash bank balance
	if err := tx.Model(&cashBank).Where("id = ?", cashBankID).Updates(cashBankUpdates).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update cash bank balance: %w", err)
	}

	// Update linked COA account balance
	if cashBank.AccountID > 0 {
		if err := tx.Model(&models.Account{}).
			Where("id = ? AND deleted_at IS NULL", cashBank.AccountID).
			Update("balance", actualBookBalance).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to update COA account balance: %w", err)
		}

		// Update parent account balances recursively
		if err := s.updateParentAccountBalances(tx, cashBank.AccountID); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to update parent balances: %w", err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit balance sync: %w", err)
	}

	log.Printf("✅ Balance sync completed for cash bank ID: %d (Native: %.2f, Book IDR: %.2f)", cashBankID, actualBalance, actualBookBalance)
	return nil
}

// updateParentAccountBalances recursively updates parent account balances
func (s *AutoBalanceSyncService) updateParentAccountBalances(tx *gorm.DB, accountID uint) error {
	// Get account details
	var account models.Account
	if err := tx.Where("id = ? AND deleted_at IS NULL", accountID).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // Account doesn't exist, nothing to update
		}
		return err
	}

	// If account has no parent, we're done
	if account.ParentID == nil {
		return nil
	}

	parentID := *account.ParentID

	// Calculate sum of all children balances
	var childrenSum float64
	if err := tx.Model(&models.Account{}).
		Where("parent_id = ? AND deleted_at IS NULL", parentID).
		Select("COALESCE(SUM(balance), 0)").
		Scan(&childrenSum).Error; err != nil {
		return fmt.Errorf("failed to calculate children sum for parent ID %d: %w", parentID, err)
	}

	// Get current parent balance for comparison
	var currentParentBalance float64
	tx.Model(&models.Account{}).
		Where("id = ? AND deleted_at IS NULL", parentID).
		Select("balance").
		Scan(&currentParentBalance)

	// Update parent balance
	if err := tx.Model(&models.Account{}).
		Where("id = ? AND deleted_at IS NULL", parentID).
		Update("balance", childrenSum).Error; err != nil {
		return fmt.Errorf("failed to update parent balance for ID %d: %w", parentID, err)
	}

	// Log the update for debugging
	if currentParentBalance != childrenSum {
		log.Printf("🔄 Updated parent account %d balance: %.2f → %.2f", parentID, currentParentBalance, childrenSum)
	}

	// Recursively update parent's parent
	return s.updateParentAccountBalances(tx, parentID)
}

// ValidateBalanceConsistency checks if all balances are consistent
func (s *AutoBalanceSyncService) ValidateBalanceConsistency() (*BalanceConsistencyReport, error) {
	log.Println("🔍 Starting balance consistency validation...")

	report := &BalanceConsistencyReport{
		Timestamp:         time.Now(),
		CashBankIssues:    []CashBankBalanceIssue{},
		ParentChildIssues: []ParentChildBalanceIssue{},
		IsConsistent:      true,
	}

	// Check cash bank vs COA consistency
	var cashBankIssues []CashBankBalanceIssue
	cashBankQuery := `
		SELECT 
			cb.id,
			cb.code,
			cb.name,
			cb.balance as cash_bank_balance,
			a.balance as coa_balance,
			(cb.balance - a.balance) as difference,
			COALESCE(tx_sum.transaction_sum, 0) as transaction_sum
		FROM cash_banks cb
		JOIN accounts a ON cb.account_id = a.id
		LEFT JOIN (
			SELECT 
				cash_bank_id,
				SUM(amount) as transaction_sum
			FROM cash_bank_transactions 
			WHERE deleted_at IS NULL 
			GROUP BY cash_bank_id
		) tx_sum ON cb.id = tx_sum.cash_bank_id
		WHERE cb.deleted_at IS NULL 
		AND a.deleted_at IS NULL
		AND cb.is_active = true
		AND ABS(cb.balance - a.balance) > 0.01
		ORDER BY ABS(cb.balance - a.balance) DESC
	`

	if err := s.db.Raw(cashBankQuery).Scan(&cashBankIssues).Error; err != nil {
		return nil, fmt.Errorf("failed to check cash bank consistency: %w", err)
	}

	report.CashBankIssues = cashBankIssues
	if len(cashBankIssues) > 0 {
		report.IsConsistent = false
	}

	// Check parent-child balance consistency
	var parentChildIssues []ParentChildBalanceIssue
	parentChildQuery := `
		SELECT 
			p.id,
			p.code,
			p.name,
			p.balance as parent_balance,
			COALESCE(children_sum.total, 0) as children_sum,
			(p.balance - COALESCE(children_sum.total, 0)) as difference
		FROM accounts p
		LEFT JOIN (
			SELECT 
				parent_id,
				SUM(balance) as total
			FROM accounts 
			WHERE deleted_at IS NULL 
			GROUP BY parent_id
		) children_sum ON p.id = children_sum.parent_id
		WHERE p.is_header = true 
		AND p.deleted_at IS NULL
		AND ABS(p.balance - COALESCE(children_sum.total, 0)) > 0.01
		ORDER BY ABS(p.balance - COALESCE(children_sum.total, 0)) DESC
	`

	if err := s.db.Raw(parentChildQuery).Scan(&parentChildIssues).Error; err != nil {
		return nil, fmt.Errorf("failed to check parent-child consistency: %w", err)
	}

	report.ParentChildIssues = parentChildIssues
	if len(parentChildIssues) > 0 {
		report.IsConsistent = false
	}

	// Calculate balance sheet totals
	var balanceTotals BalanceSheetTotals
	balanceQuery := `
		SELECT 
			COALESCE(SUM(CASE WHEN type = 'ASSET' THEN balance ELSE 0 END), 0) as assets,
			COALESCE(SUM(CASE WHEN type = 'LIABILITY' THEN balance ELSE 0 END), 0) as liabilities,
			COALESCE(SUM(CASE WHEN type = 'EQUITY' THEN balance ELSE 0 END), 0) as equity,
			COALESCE(SUM(CASE WHEN type = 'REVENUE' THEN balance ELSE 0 END), 0) as revenue,
			COALESCE(SUM(CASE WHEN type = 'EXPENSE' THEN balance ELSE 0 END), 0) as expenses
		FROM accounts 
		WHERE deleted_at IS NULL
	`

	if err := s.db.Raw(balanceQuery).Scan(&balanceTotals).Error; err != nil {
		return nil, fmt.Errorf("failed to calculate balance sheet totals: %w", err)
	}

	report.BalanceTotals = balanceTotals

	// Calculate balance equation (Assets + Expenses + Liabilities + Equity + Revenue should = 0)
	balanceEquation := balanceTotals.Assets + balanceTotals.Expenses +
		balanceTotals.Liabilities + balanceTotals.Equity + balanceTotals.Revenue
	report.BalanceEquationDifference = balanceEquation

	if balanceEquation > 0.01 || balanceEquation < -0.01 {
		report.IsConsistent = false
	}

	log.Printf("✅ Balance consistency validation completed. Issues found: CashBank=%d, ParentChild=%d, BalanceSheet=%.2f",
		len(cashBankIssues), len(parentChildIssues), balanceEquation)

	return report, nil
}

// FixAllBalanceIssues attempts to fix all detected balance issues
func (s *AutoBalanceSyncService) FixAllBalanceIssues() error {
	log.Println("🔧 Starting automatic balance issue resolution...")

	// Get all cash banks that need syncing
	var cashBanks []struct {
		ID        uint `json:"id"`
		AccountID uint `json:"account_id"`
	}

	if err := s.db.Raw(`
		SELECT cb.id, cb.account_id
		FROM cash_banks cb
		WHERE cb.deleted_at IS NULL 
		AND cb.is_active = true
		ORDER BY cb.id
	`).Scan(&cashBanks).Error; err != nil {
		return fmt.Errorf("failed to get cash banks for syncing: %w", err)
	}

	// Sync each cash bank
	for _, cb := range cashBanks {
		if err := s.SyncCashBankToAccount(cb.ID); err != nil {
			log.Printf("⚠️ Failed to sync cash bank ID %d: %v", cb.ID, err)
			// Continue with other cash banks
		}
	}

	log.Printf("✅ Completed automatic balance sync for %d cash banks", len(cashBanks))
	return nil
}

// Balance consistency report structures
type BalanceConsistencyReport struct {
	Timestamp                 time.Time                 `json:"timestamp"`
	IsConsistent              bool                      `json:"is_consistent"`
	CashBankIssues            []CashBankBalanceIssue    `json:"cash_bank_issues"`
	ParentChildIssues         []ParentChildBalanceIssue `json:"parent_child_issues"`
	BalanceTotals             BalanceSheetTotals        `json:"balance_totals"`
	BalanceEquationDifference float64                   `json:"balance_equation_difference"`
}

type CashBankBalanceIssue struct {
	ID              uint    `json:"id"`
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	CashBankBalance float64 `json:"cash_bank_balance"`
	COABalance      float64 `json:"coa_balance"`
	Difference      float64 `json:"difference"`
	TransactionSum  float64 `json:"transaction_sum"`
}

type ParentChildBalanceIssue struct {
	ID            uint    `json:"id"`
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	ParentBalance float64 `json:"parent_balance"`
	ChildrenSum   float64 `json:"children_sum"`
	Difference    float64 `json:"difference"`
}

type BalanceSheetTotals struct {
	Assets      float64 `json:"assets"`
	Liabilities float64 `json:"liabilities"`
	Equity      float64 `json:"equity"`
	Revenue     float64 `json:"revenue"`
	Expenses    float64 `json:"expenses"`
}

// Enhanced CashBankService methods to use auto sync

// SyncCashBankBalance manually triggers balance sync for a specific cash bank
func (s *CashBankService) SyncCashBankBalance(cashBankID uint) error {
	autoSyncService := NewAutoBalanceSyncService(s.db, s.accountRepo)
	return autoSyncService.SyncCashBankToAccount(cashBankID)
}

// ValidateAllBalances validates consistency of all cash bank and account balances
func (s *CashBankService) ValidateAllBalances() (*BalanceConsistencyReport, error) {
	autoSyncService := NewAutoBalanceSyncService(s.db, s.accountRepo)
	return autoSyncService.ValidateBalanceConsistency()
}

// FixAllBalanceIssues automatically fixes detected balance issues
func (s *CashBankService) FixAllBalanceIssues() error {
	autoSyncService := NewAutoBalanceSyncService(s.db, s.accountRepo)
	return autoSyncService.FixAllBalanceIssues()
}
