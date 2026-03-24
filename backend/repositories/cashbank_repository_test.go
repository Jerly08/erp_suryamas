package repositories

import (
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetTransactionsRespectsChronologicalSortOrder(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	require.NoError(t, db.AutoMigrate(&models.CashBankTransaction{}))

	repo := NewCashBankRepository(db)

	baseDate := time.Date(2026, 3, 24, 0, 0, 0, 0, time.UTC)
	transactions := []models.CashBankTransaction{
		{
			CashBankID:      1,
			ReferenceType:   "DEPOSIT",
			ReferenceID:     1,
			Amount:          500000000,
			Currency:        "IDR",
			ExchangeRate:    1,
			BookAmountIDR:   500000000,
			BalanceAfter:    500000000,
			TransactionDate: baseDate.Add(11*time.Hour + 24*time.Minute + 31*time.Second),
			CreatedAt:       baseDate.Add(11*time.Hour + 24*time.Minute + 31*time.Second),
		},
		{
			CashBankID:      1,
			ReferenceType:   "SALE",
			ReferenceID:     2,
			Amount:          22200000,
			Currency:        "IDR",
			ExchangeRate:    1,
			BookAmountIDR:   22200000,
			BalanceAfter:    522200000,
			TransactionDate: baseDate,
			CreatedAt:       baseDate.Add(11*time.Hour + 31*time.Minute + 42*time.Second),
		},
		{
			CashBankID:      1,
			ReferenceType:   "SALE_REVERSAL",
			ReferenceID:     3,
			Amount:          -22200000,
			Currency:        "IDR",
			ExchangeRate:    1,
			BookAmountIDR:   -22200000,
			BalanceAfter:    500000000,
			TransactionDate: baseDate.Add(11*time.Hour + 33*time.Minute + 18*time.Second),
			CreatedAt:       baseDate.Add(11*time.Hour + 33*time.Minute + 18*time.Second),
		},
		{
			CashBankID:      1,
			ReferenceType:   "PURCHASE",
			ReferenceID:     4,
			Amount:          -111000000,
			Currency:        "IDR",
			ExchangeRate:    1,
			BookAmountIDR:   -111000000,
			BalanceAfter:    389000000,
			TransactionDate: baseDate,
			CreatedAt:       baseDate.Add(11*time.Hour + 54*time.Minute + 10*time.Second),
		},
		{
			CashBankID:      1,
			ReferenceType:   "PURCHASE_REVERSAL",
			ReferenceID:     5,
			Amount:          111000000,
			Currency:        "IDR",
			ExchangeRate:    1,
			BookAmountIDR:   111000000,
			BalanceAfter:    500000000,
			TransactionDate: baseDate.Add(11*time.Hour + 54*time.Minute + 34*time.Second),
			CreatedAt:       baseDate.Add(11*time.Hour + 54*time.Minute + 34*time.Second),
		},
	}

	for _, tx := range transactions {
		require.NoError(t, db.Create(&tx).Error)
	}

	ascResult, err := repo.GetTransactions(1, TransactionFilter{
		Page:      1,
		Limit:     10,
		SortOrder: "asc",
	})
	require.NoError(t, err)
	require.Len(t, ascResult.Data, 5)
	require.Equal(t, uint(1), ascResult.Data[0].ReferenceID)
	require.Equal(t, uint(2), ascResult.Data[1].ReferenceID)
	require.Equal(t, uint(3), ascResult.Data[2].ReferenceID)
	require.Equal(t, uint(4), ascResult.Data[3].ReferenceID)
	require.Equal(t, uint(5), ascResult.Data[4].ReferenceID)

	descResult, err := repo.GetTransactions(1, TransactionFilter{
		Page:      1,
		Limit:     10,
		SortOrder: "desc",
	})
	require.NoError(t, err)
	require.Len(t, descResult.Data, 5)
	require.Equal(t, uint(5), descResult.Data[0].ReferenceID)
	require.Equal(t, uint(4), descResult.Data[1].ReferenceID)
	require.Equal(t, uint(3), descResult.Data[2].ReferenceID)
	require.Equal(t, uint(2), descResult.Data[3].ReferenceID)
	require.Equal(t, uint(1), descResult.Data[4].ReferenceID)
}
