package services

import (
	"fmt"

	"app-sistem-akuntansi/models"
	"gorm.io/gorm"
)

// CashBankValidationService provides validation and repair utilities around CashBank⇄COA sync.
type CashBankValidationService struct {
	db                *gorm.DB
	accountingService *CashBankAccountingService
}

func NewCashBankValidationService(db *gorm.DB, accountingService *CashBankAccountingService) *CashBankValidationService {
	return &CashBankValidationService{db: db, accountingService: accountingService}
}

// SyncDiscrepancy describes a mismatch between CashBank and COA balances.
type SyncDiscrepancy struct {
	CashBankID      uint    `json:"cash_bank_id"`
	CashBankName    string  `json:"cash_bank_name"`
	CashBankCode    string  `json:"cash_bank_code"`
	COAAccountID    uint    `json:"coa_account_id"`
	COAAccountCode  string  `json:"coa_account_code"`
	COAAccountName  string  `json:"coa_account_name"`
	CashBankBalance float64 `json:"cash_bank_balance"`
	COABalance      float64 `json:"coa_balance"`
	TransactionSum  float64 `json:"transaction_sum"`
	Discrepancy     float64 `json:"discrepancy"`
	Issue           string  `json:"issue"`
}

// FindSyncDiscrepancies detects discrepancies between CashBank and COA balances.
func (s *CashBankValidationService) FindSyncDiscrepancies() ([]SyncDiscrepancy, error) {
	var discrepancies []SyncDiscrepancy

	err := s.db.Raw(`
        SELECT 
            cb.id as cash_bank_id,
            cb.name as cash_bank_name,
            cb.code as cash_bank_code,
            COALESCE(a.id, 0) as coa_account_id,
            COALESCE(a.code, 'NOT_LINKED') as coa_account_code,
            COALESCE(a.name, 'NOT_LINKED') as coa_account_name,
            cb.balance as cash_bank_balance,
            COALESCE(a.balance, 0) as coa_balance,
            COALESCE(tx_sum.transaction_sum, 0) as transaction_sum,
            (cb.balance - COALESCE(a.balance, 0)) as discrepancy,
            CASE 
                WHEN cb.account_id = 0 OR cb.account_id IS NULL THEN 'NOT_LINKED'
                WHEN cb.balance != COALESCE(a.balance, 0) THEN 'BALANCE_MISMATCH'
                WHEN COALESCE(tx_sum.transaction_sum, 0) != cb.balance THEN 'TRANSACTION_SUM_MISMATCH'
                ELSE 'SYNC_OK'
            END as issue
        FROM cash_banks cb
        LEFT JOIN accounts a ON cb.account_id = a.id AND a.deleted_at IS NULL
        LEFT JOIN (
            SELECT 
                cash_bank_id,
                SUM(amount) as transaction_sum
            FROM cash_bank_transactions 
            WHERE deleted_at IS NULL 
            GROUP BY cash_bank_id
        ) tx_sum ON cb.id = tx_sum.cash_bank_id
        WHERE cb.deleted_at IS NULL 
          AND cb.is_active = true
          AND (
              cb.account_id = 0 OR cb.account_id IS NULL OR
              cb.balance != COALESCE(a.balance, 0) OR
              COALESCE(tx_sum.transaction_sum, 0) != cb.balance
          )
        ORDER BY cb.name
    `).Scan(&discrepancies).Error

	return discrepancies, err
}

// ValidateAllSync validates that all cash banks are in sync with their COA accounts.
func (s *CashBankValidationService) ValidateAllSync() error {
	discrepancies, err := s.FindSyncDiscrepancies()
	if err != nil {
		return fmt.Errorf("failed to check sync discrepancies: %v", err)
	}

	if len(discrepancies) > 0 {
		issueCount := make(map[string]int)
		for _, d := range discrepancies {
			issueCount[d.Issue]++
		}
		return fmt.Errorf("found %d cash bank/COA sync issues: %v", len(discrepancies), issueCount)
	}

	return nil
}

// AutoFixDiscrepancies automatically fixes discrepancies where possible.
func (s *CashBankValidationService) AutoFixDiscrepancies() (int, error) {
	discrepancies, err := s.FindSyncDiscrepancies()
	if err != nil {
		return 0, err
	}

	fixedCount := 0
	for _, d := range discrepancies {
		switch d.Issue {
		case "NOT_LINKED":
			// Cannot auto-fix: requires manual account linking
			continue
		case "BALANCE_MISMATCH":
			// Use transaction sum as source of truth
			if err := s.fixBalanceMismatch(d.CashBankID, d.COAAccountID, d.TransactionSum); err != nil {
				return fixedCount, fmt.Errorf("failed to fix balance mismatch for %s: %v", d.CashBankName, err)
			}
			fixedCount++
		case "TRANSACTION_SUM_MISMATCH":
			// Recalculate cash bank balance from transactions
			if err := s.accountingService.RecalculateCashBankBalance(d.CashBankID); err != nil {
				return fixedCount, fmt.Errorf("failed to recalculate balance for %s: %v", d.CashBankName, err)
			}
			fixedCount++
		}
	}

	return fixedCount, nil
}

// fixBalanceMismatch applies the same corrected balance to both CB and COA accounts.
func (s *CashBankValidationService) fixBalanceMismatch(cashBankID, coaAccountID uint, correctBalance float64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Update CashBank balance
		if err := tx.Model(&models.CashBank{}).
			Where("id = ?", cashBankID).
			Update("balance", correctBalance).Error; err != nil {
			return fmt.Errorf("failed to update cash bank balance: %v", err)
		}

		// Update COA balance
		if err := tx.Model(&models.Account{}).
			Where("id = ?", coaAccountID).
			Update("balance", correctBalance).Error; err != nil {
			return fmt.Errorf("failed to update COA balance: %v", err)
		}
		return nil
	})
}

// GetSyncStatus returns overall sync status summary for dashboards.
func (s *CashBankValidationService) GetSyncStatus() (map[string]interface{}, error) {
	discrepancies, err := s.FindSyncDiscrepancies()
	if err != nil {
		return nil, err
	}

	// Count by issue type
	issueCount := make(map[string]int)
	totalDiscrepancy := 0.0
	for _, d := range discrepancies {
		issueCount[d.Issue]++
		totalDiscrepancy += d.Discrepancy
	}

	// Totals
	var totalCashBanks int64
	s.db.Model(&models.CashBank{}).Where("deleted_at IS NULL AND is_active = true").Count(&totalCashBanks)
	var linkedCashBanks int64
	s.db.Model(&models.CashBank{}).Where("deleted_at IS NULL AND is_active = true AND account_id > 0").Count(&linkedCashBanks)

	status := "healthy"
	if len(discrepancies) > 0 {
		status = "unhealthy"
	}

	return map[string]interface{}{
		"status":              status,
		"total_cash_banks":    totalCashBanks,
		"linked_cash_banks":   linkedCashBanks,
		"unlinked_cash_banks": totalCashBanks - linkedCashBanks,
		"discrepancies_count": len(discrepancies),
		"issue_breakdown":     issueCount,
		"total_discrepancy":   totalDiscrepancy,
		"discrepancies":       discrepancies,
	}, nil
}

// ValidateCashBankIntegrity validates a specific CashBank account integrity.
func (s *CashBankValidationService) ValidateCashBankIntegrity(cashBankID uint) (*SyncDiscrepancy, error) {
	var discrepancy SyncDiscrepancy
	err := s.db.Raw(`
        SELECT 
            cb.id as cash_bank_id,
            cb.name as cash_bank_name,
            cb.code as cash_bank_code,
            COALESCE(a.id, 0) as coa_account_id,
            COALESCE(a.code, 'NOT_LINKED') as coa_account_code,
            COALESCE(a.name, 'NOT_LINKED') as coa_account_name,
            cb.balance as cash_bank_balance,
            COALESCE(a.balance, 0) as coa_balance,
            COALESCE(tx_sum.transaction_sum, 0) as transaction_sum,
            (cb.balance - COALESCE(a.balance, 0)) as discrepancy,
            CASE 
                WHEN cb.account_id = 0 OR cb.account_id IS NULL THEN 'NOT_LINKED'
                WHEN cb.balance != COALESCE(a.balance, 0) THEN 'BALANCE_MISMATCH'
                WHEN COALESCE(tx_sum.transaction_sum, 0) != cb.balance THEN 'TRANSACTION_SUM_MISMATCH'
                ELSE 'SYNC_OK'
            END as issue
        FROM cash_banks cb
        LEFT JOIN accounts a ON cb.account_id = a.id AND a.deleted_at IS NULL
        LEFT JOIN (
            SELECT 
                cash_bank_id,
                SUM(amount) as transaction_sum
            FROM cash_bank_transactions 
            WHERE deleted_at IS NULL AND cash_bank_id = ?
            GROUP BY cash_bank_id
        ) tx_sum ON cb.id = tx_sum.cash_bank_id
        WHERE cb.id = ? AND cb.deleted_at IS NULL
    `, cashBankID, cashBankID).Scan(&discrepancy).Error

	if err != nil {
		return nil, fmt.Errorf("failed to validate cash bank integrity: %v", err)
	}
	return &discrepancy, nil
}

// LinkCashBankToAccount links a CashBank to a COA account (must be Asset).
func (s *CashBankValidationService) LinkCashBankToAccount(cashBankID, accountID uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Validate account exists and is an asset account
		var account models.Account
		if err := tx.First(&account, accountID).Error; err != nil {
			return fmt.Errorf("account not found: %v", err)
		}
		if account.Type != models.AccountTypeAsset {
			return fmt.Errorf("cash/bank accounts must be linked to asset accounts")
		}

		// Validate cash bank exists
		var cashBank models.CashBank
		if err := tx.First(&cashBank, cashBankID).Error; err != nil {
			return fmt.Errorf("cash bank not found: %v", err)
		}

		// Link the accounts
		if err := tx.Model(&cashBank).Update("account_id", accountID).Error; err != nil {
			return fmt.Errorf("failed to link cash bank to account: %v", err)
		}

		// Sync the initial book balance into GL. Foreign-currency accounts must post IDR book value.
		if err := tx.Model(&account).Update("balance", cashBank.EffectiveBookBalanceIDR()).Error; err != nil {
			return fmt.Errorf("failed to sync initial balance: %v", err)
		}
		return nil
	})
}

// UnlinkCashBankFromAccount unlinks a cash bank from its COA account.
func (s *CashBankValidationService) UnlinkCashBankFromAccount(cashBankID uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Get cash bank with current account
		var cashBank models.CashBank
		if err := tx.First(&cashBank, cashBankID).Error; err != nil {
			return fmt.Errorf("cash bank not found: %v", err)
		}

		// Reset linked account balance to 0 if it was linked
		if cashBank.AccountID > 0 {
			if err := tx.Model(&models.Account{}).
				Where("id = ?", cashBank.AccountID).
				Update("balance", 0).Error; err != nil {
				return fmt.Errorf("failed to reset account balance: %v", err)
			}
		}

		// Unlink the accounts
		if err := tx.Model(&cashBank).Update("account_id", 0).Error; err != nil {
			return fmt.Errorf("failed to unlink cash bank from account: %v", err)
		}
		return nil
	})
}

// ResetAllCashBankBalances resets all cash bank balances from their transactions via accounting service.
func (s *CashBankValidationService) ResetAllCashBankBalances() error {
	return s.accountingService.SyncAllCashBankBalances()
}
