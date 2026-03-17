package services

import (
	"errors"
	"fmt"
	"time"

	"app-sistem-akuntansi/models"
	"gorm.io/gorm"
)

// CashBankCOASyncService keeps Cash & Bank accounts synchronized with COA (Asset) accounts.
//
// Responsibilities:
// - Link CashBank account to a COA account (Asset only)
// - Keep balances in sync based on CashBankTransactions sum (SSOT for balance)
// - Validate and auto-fix discrepancies
//
// Notes:
// - This service assumes balances derive from transactions. Manual balance edits should be avoided.
// - Call SyncAfterTransaction from deposit/withdraw/transfer handlers after committing a transaction.
// - If you already have an SSOT unified journal, this service remains safe: it only updates balances to
//   the transaction sum and does not create journal entries.
//
// Integration points:
// - After creating a CashBankTransaction: call SyncAfterTransaction(tx, cashBankID)
// - Nightly job can call SyncAllAssets() to enforce consistency
// - Admin tool can call AutoFixAll() when discrepancies are found

type CashBankCOASyncService struct {
	db *gorm.DB
}

func NewCashBankCOASyncService(db *gorm.DB) *CashBankCOASyncService {
	return &CashBankCOASyncService{db: db}
}

// LinkCashBankToCOA links a cash/bank to a COA account, validating that COA is an ASSET.
func (s *CashBankCOASyncService) LinkCashBankToCOA(cashBankID, coaAccountID uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Load COA account and validate type Asset
		var acc models.Account
		if err := tx.First(&acc, coaAccountID).Error; err != nil {
			return fmt.Errorf("coa account not found: %w", err)
		}
		if acc.Type != models.AccountTypeAsset {
			return fmt.Errorf("only ASSET accounts can be linked to Cash/Bank (got: %s)", acc.Type)
		}

		// Update link
		if err := tx.Model(&models.CashBank{}).Where("id = ?", cashBankID).Update("account_id", coaAccountID).Error; err != nil {
			return fmt.Errorf("failed linking cashbank to COA: %w", err)
		}

		// Initial sync so both balances match transaction sum
		return s.syncBalancesTx(tx, cashBankID)
	})
}

// SyncAfterTransaction should be called right after inserting/updating/deleting a cash bank transaction
// within the same DB transaction (when possible). If not possible, call SyncSingle outside.
func (s *CashBankCOASyncService) SyncAfterTransaction(tx *gorm.DB, cashBankID uint) error {
	if tx == nil {
		return errors.New("transaction handle is nil; use SyncSingle instead")
	}
	return s.syncBalancesTx(tx, cashBankID)
}

// SyncSingle recalculates and updates balances for one CashBank and its linked COA account.
func (s *CashBankCOASyncService) SyncSingle(cashBankID uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error { return s.syncBalancesTx(tx, cashBankID) })
}

// SyncAllAssets enforces sync for all CashBank accounts that are linked to ASSET COA accounts.
func (s *CashBankCOASyncService) SyncAllAssets() (int, error) {
	synced := 0
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var pairs []struct {
			ID          uint
			AccountID   uint
			AccountType string
		}
		if err := tx.Table("cash_banks cb").
			Select("cb.id as id, cb.account_id as account_id, a.type as account_type").
			Joins("JOIN accounts a ON a.id = cb.account_id").
			Where("cb.deleted_at IS NULL AND a.deleted_at IS NULL AND a.type = ?", models.AccountTypeAsset).
			Scan(&pairs).Error; err != nil {
			return err
		}
		for _, p := range pairs {
			if err := s.syncBalancesTx(tx, p.ID); err != nil {
				return err
			}
			synced++
		}
		return nil
	})
	return synced, err
}

// FindDiscrepancies lists cashbank/COA pairs where balances diverge.
func (s *CashBankCOASyncService) FindDiscrepancies() ([]SyncDiscrepancy, error) {
	var out []SyncDiscrepancy
	err := s.db.Raw(`
        SELECT 
            cb.id AS cash_bank_id,
            cb.name AS cash_bank_name,
            cb.code AS cash_bank_code,
            a.id AS coa_account_id,
            a.code AS coa_account_code,
            cb.balance AS cash_bank_balance,
            a.balance AS coa_balance,
            COALESCE(tx_sum.transaction_sum, 0) AS transaction_sum,
            (cb.balance - a.balance) AS discrepancy
        FROM cash_banks cb
        JOIN accounts a ON cb.account_id = a.id
        LEFT JOIN (
            SELECT cash_bank_id, SUM(amount) AS transaction_sum
            FROM cash_bank_transactions
            WHERE deleted_at IS NULL
            GROUP BY cash_bank_id
        ) tx_sum ON tx_sum.cash_bank_id = cb.id
        WHERE cb.deleted_at IS NULL
          AND a.deleted_at IS NULL
          AND a.type = ?
          AND cb.balance != a.balance
    `, models.AccountTypeAsset).Scan(&out).Error
	return out, err
}

// AutoFixAll sets both balances to the transaction sum for all asset-linked pairs.
func (s *CashBankCOASyncService) AutoFixAll() (int, error) {
	fixed := 0
	err := s.db.Transaction(func(tx *gorm.DB) error {
		discs, err := s.FindDiscrepancies()
		if err != nil {
			return err
		}
		for _, d := range discs {
			if err := s.syncBalancesTx(tx, d.CashBankID); err != nil {
				return err
			}
			fixed++
		}
		return nil
	})
	return fixed, err
}

// syncBalancesTx recalculates transaction sum, then updates both CashBank and COA balances.
func (s *CashBankCOASyncService) syncBalancesTx(tx *gorm.DB, cashBankID uint) error {
	// Get CashBank + linked COA account
	var cb models.CashBank
	if err := tx.Preload("Account").First(&cb, cashBankID).Error; err != nil {
		return fmt.Errorf("cash bank not found: %w", err)
	}
	if cb.AccountID == 0 {
		// Not linked; skip silently
		return nil
	}
	if cb.Account.Type != models.AccountTypeAsset {
		return fmt.Errorf("linked COA account must be ASSET (got: %s)", cb.Account.Type)
	}

	// Sum all non-deleted transactions for this cash bank.
	var totals struct {
		NativeBalance float64
		BookBalance   float64
	}
	if err := tx.Table("cash_bank_transactions").
		Where("cash_bank_id = ? AND deleted_at IS NULL", cashBankID).
		Select("COALESCE(SUM(amount), 0) AS native_balance, COALESCE(SUM(book_amount_id_r), 0) AS book_balance").
		Scan(&totals).Error; err != nil {
		return fmt.Errorf("failed to sum transactions: %w", err)
	}

	bookBalance := totals.NativeBalance
	if cb.IsForeignCurrency() {
		bookBalance = totals.BookBalance
	}

	return s.applyBalance(tx, &cb, totals.NativeBalance, bookBalance)
}

func (s *CashBankCOASyncService) applyBalance(tx *gorm.DB, cashBank *models.CashBank, nativeBalance, bookBalance float64) error {
	// Update balances atomically.
	cashBankUpdates := map[string]interface{}{
		"balance":    nativeBalance,
		"updated_at": time.Now(),
	}
	if cashBank.IsForeignCurrency() {
		cashBankUpdates["book_balance_id_r"] = bookBalance
		if nativeBalance > 0 && bookBalance > 0 {
			cashBankUpdates["last_exchange_rate"] = bookBalance / nativeBalance
		}
	}

	if err := tx.Model(&models.CashBank{}).
		Where("id = ?", cashBank.ID).
		Updates(cashBankUpdates).Error; err != nil {
		return err
	}
	if err := tx.Model(&models.Account{}).
		Where("id = ?", cashBank.AccountID).
		Updates(map[string]interface{}{
			"balance":    bookBalance,
			"updated_at": time.Now(),
		}).Error; err != nil {
		return err
	}
	return nil
}
