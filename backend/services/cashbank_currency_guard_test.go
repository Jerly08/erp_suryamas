package services

import (
	"strings"
	"testing"
	"time"

	"app-sistem-akuntansi/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupCashBankCurrencyGuardTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		if strings.Contains(err.Error(), "requires cgo") {
			t.Skip("sqlite test skipped because this environment runs with CGO disabled")
		}
		require.NoError(t, err)
	}

	err = db.AutoMigrate(&models.Account{}, &models.CashBank{}, &models.CashBankTransaction{})
	require.NoError(t, err)

	return db
}

func TestCashBankCurrencyGuardRepairsForeignCurrencyBalances(t *testing.T) {
	db := setupCashBankCurrencyGuardTestDB(t)

	account := models.Account{
		Code:     "1101-USD",
		Name:     "Bank USD",
		Type:     models.AccountTypeAsset,
		Category: models.CategoryCurrentAsset,
		IsActive: true,
		Balance:  0,
	}
	require.NoError(t, db.Create(&account).Error)

	cashBank := models.CashBank{
		Code:             "BNK-USD-001",
		Name:             "Bank USD",
		Type:             models.CashBankTypeBank,
		AccountID:        account.ID,
		Currency:         "USD",
		Balance:          16860910,
		BookBalanceIDR:   16860910,
		LastExchangeRate: 16860.91,
		IsActive:         true,
	}
	require.NoError(t, db.Create(&cashBank).Error)

	transaction := models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   "OPENING_BALANCE",
		Amount:          1000,
		Currency:        "USD",
		ExchangeRate:    16860.91,
		BookAmountIDR:   16860910,
		BalanceAfter:    1000,
		TransactionDate: time.Now(),
		Notes:           "Opening Balance",
	}
	require.NoError(t, db.Create(&transaction).Error)

	guard := NewCashBankCurrencyGuard(db)
	require.NoError(t, guard.EnforceAfterTransaction(db, cashBank.ID))

	var repaired models.CashBank
	require.NoError(t, db.First(&repaired, cashBank.ID).Error)
	require.InDelta(t, 1000, repaired.Balance, 0.0001)
	require.InDelta(t, 16860910, repaired.BookBalanceIDR, 0.01)
	require.InDelta(t, 16860.91, repaired.LastExchangeRate, 0.0001)

	var repairedAccount models.Account
	require.NoError(t, db.First(&repairedAccount, account.ID).Error)
	require.InDelta(t, 16860910, repairedAccount.Balance, 0.01)
}

func TestCashBankCurrencyGuardRepairsIDRBalances(t *testing.T) {
	db := setupCashBankCurrencyGuardTestDB(t)

	account := models.Account{
		Code:     "1101-IDR",
		Name:     "Bank IDR",
		Type:     models.AccountTypeAsset,
		Category: models.CategoryCurrentAsset,
		IsActive: true,
		Balance:  0,
	}
	require.NoError(t, db.Create(&account).Error)

	cashBank := models.CashBank{
		Code:             "BNK-IDR-001",
		Name:             "Bank IDR",
		Type:             models.CashBankTypeBank,
		AccountID:        account.ID,
		Currency:         "IDR",
		Balance:          0,
		BookBalanceIDR:   0,
		LastExchangeRate: 1,
		IsActive:         true,
	}
	require.NoError(t, db.Create(&cashBank).Error)

	transaction := models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   "DEPOSIT",
		Amount:          2500000,
		Currency:        "IDR",
		ExchangeRate:    1,
		BookAmountIDR:   2500000,
		BalanceAfter:    2500000,
		TransactionDate: time.Now(),
		Notes:           "Deposit",
	}
	require.NoError(t, db.Create(&transaction).Error)

	guard := NewCashBankCurrencyGuard(db)
	require.NoError(t, guard.EnforceAfterTransaction(db, cashBank.ID))

	var repaired models.CashBank
	require.NoError(t, db.First(&repaired, cashBank.ID).Error)
	require.InDelta(t, 2500000, repaired.Balance, 0.01)
	require.InDelta(t, 2500000, repaired.BookBalanceIDR, 0.01)
	require.InDelta(t, 1, repaired.LastExchangeRate, 0.0001)

	var repairedAccount models.Account
	require.NoError(t, db.First(&repairedAccount, account.ID).Error)
	require.InDelta(t, 2500000, repairedAccount.Balance, 0.01)
}
