package services

import (
	"fmt"
	"math"
	"time"

	"app-sistem-akuntansi/models"
	"gorm.io/gorm"
)

const cashBankInvariantTolerance = 0.01

// CashBankCurrencyGuard keeps native cash/bank balances and book IDR balances aligned
// with CashBankTransaction history. This is the authoritative guard for SSOT flows.
type CashBankCurrencyGuard struct {
	db *gorm.DB
}

func NewCashBankCurrencyGuard(db *gorm.DB) *CashBankCurrencyGuard {
	return &CashBankCurrencyGuard{db: db}
}

func (g *CashBankCurrencyGuard) EnforceAfterTransaction(tx *gorm.DB, cashBankIDs ...uint) error {
	executor := g.db
	if tx != nil {
		executor = tx
	}

	seen := make(map[uint]struct{})
	for _, cashBankID := range cashBankIDs {
		if cashBankID == 0 {
			continue
		}
		if _, exists := seen[cashBankID]; exists {
			continue
		}
		seen[cashBankID] = struct{}{}

		if err := g.enforceSingle(executor, cashBankID); err != nil {
			return err
		}
	}

	return nil
}

func (g *CashBankCurrencyGuard) enforceSingle(tx *gorm.DB, cashBankID uint) error {
	var cashBank models.CashBank
	if err := tx.Preload("Account").First(&cashBank, cashBankID).Error; err != nil {
		return fmt.Errorf("cash bank not found for invariant enforcement: %w", err)
	}

	var totals struct {
		NativeBalance float64
		BookBalance   float64
	}
	if err := tx.Table("cash_bank_transactions").
		Where("cash_bank_id = ? AND deleted_at IS NULL", cashBankID).
		Select("COALESCE(SUM(amount), 0) AS native_balance, COALESCE(SUM(book_amount_id_r), 0) AS book_balance").
		Scan(&totals).Error; err != nil {
		return fmt.Errorf("failed to sum cash/bank transactions: %w", err)
	}

	expectedNative := totals.NativeBalance
	expectedBook := expectedNative
	expectedRate := 1.0
	if cashBank.IsForeignCurrency() {
		expectedBook = totals.BookBalance
		if math.Abs(expectedNative) > 0.0001 {
			expectedRate = expectedBook / expectedNative
		} else if cashBank.LastExchangeRate > 0 {
			expectedRate = cashBank.LastExchangeRate
		}
	}

	cashBankUpdates := map[string]interface{}{
		"balance":            expectedNative,
		"book_balance_id_r":  expectedBook,
		"last_exchange_rate": expectedRate,
		"updated_at":         time.Now(),
	}
	if !cashBank.IsForeignCurrency() {
		cashBankUpdates["last_exchange_rate"] = 1
	}

	if err := tx.Model(&models.CashBank{}).
		Where("id = ? AND deleted_at IS NULL", cashBankID).
		Updates(cashBankUpdates).Error; err != nil {
		return fmt.Errorf("failed to enforce cash/bank invariant: %w", err)
	}

	if cashBank.AccountID > 0 {
		if err := tx.Model(&models.Account{}).
			Where("id = ? AND deleted_at IS NULL", cashBank.AccountID).
			Update("balance", expectedBook).Error; err != nil {
			return fmt.Errorf("failed to enforce linked GL invariant: %w", err)
		}
	}

	var reloaded models.CashBank
	if err := tx.First(&reloaded, cashBankID).Error; err != nil {
		return fmt.Errorf("failed to reload cash bank after invariant enforcement: %w", err)
	}

	if math.Abs(reloaded.Balance-expectedNative) > cashBankInvariantTolerance {
		return fmt.Errorf("cash bank native balance invariant failed for account %d", cashBankID)
	}
	if math.Abs(reloaded.EffectiveBookBalanceIDR()-expectedBook) > cashBankInvariantTolerance {
		return fmt.Errorf("cash bank book balance invariant failed for account %d", cashBankID)
	}

	if cashBank.AccountID > 0 {
		var account models.Account
		if err := tx.First(&account, cashBank.AccountID).Error; err != nil {
			return fmt.Errorf("failed to reload linked GL after invariant enforcement: %w", err)
		}
		if math.Abs(account.Balance-expectedBook) > cashBankInvariantTolerance {
			return fmt.Errorf("linked GL balance invariant failed for account %d", cashBankID)
		}
	}

	return nil
}
