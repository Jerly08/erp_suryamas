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

func setupSSOTProfitLossTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		if strings.Contains(err.Error(), "requires cgo") {
			t.Skip("sqlite test skipped because this environment runs with CGO disabled")
		}
		require.NoError(t, err)
	}

	require.NoError(t, db.AutoMigrate(&models.Settings{}))
	require.NoError(t, db.Create(&models.Settings{
		CompanyName:     "PT Test ERP",
		CompanyAddress:  "Jl Test",
		CompanyPhone:    "021000000",
		CompanyEmail:    "test@example.com",
		Currency:        "IDR",
		FiscalYearStart: "January 1",
		Language:        "id",
		Timezone:        "Asia/Jakarta",
		DecimalPlaces:   2,
		InvoicePrefix:   "INV",
		SalesPrefix:     "SOA",
		QuotePrefix:     "QT",
		PurchasePrefix:  "PO",
		JournalPrefix:   "JE",
		DefaultTaxRate:  11,
		CompanyType:     models.CompanyTypeTrading,
	}).Error)

	settingsCacheMutex.Lock()
	settingsCache = nil
	settingsCacheAt = time.Time{}
	settingsCacheMutex.Unlock()

	return db
}

func TestGenerateProfitLossFromBalances_ClassifiesNonOperatingAndTaxAccounts(t *testing.T) {
	db := setupSSOTProfitLossTestDB(t)
	service := NewSSOTProfitLossService(db)

	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	balances := []SSOTAccountBalance{
		{
			AccountID:       1,
			AccountCode:     "4101",
			AccountName:     "Penjualan Lokal",
			AccountType:     models.AccountTypeRevenue,
			AccountCategory: models.CategoryOperatingRevenue,
			NetBalance:      1000,
		},
		{
			AccountID:       2,
			AccountCode:     "5101",
			AccountName:     "Pembelian Barang Dagang",
			AccountType:     models.AccountTypeExpense,
			AccountCategory: models.CategoryCostOfGoodsSold,
			NetBalance:      400,
		},
		{
			AccountID:       3,
			AccountCode:     "6102",
			AccountName:     "Beban Administrasi",
			AccountType:     models.AccountTypeExpense,
			AccountCategory: models.CategoryAdministrativeExp,
			NetBalance:      300,
		},
		{
			AccountID:       4,
			AccountCode:     "7103",
			AccountName:     "Laba Selisih Kurs",
			AccountType:     models.AccountTypeRevenue,
			AccountCategory: models.CategoryOtherIncome,
			NetBalance:      50,
		},
		{
			AccountID:       5,
			AccountCode:     "8103",
			AccountName:     "Rugi Selisih Kurs",
			AccountType:     models.AccountTypeExpense,
			AccountCategory: models.CategoryOtherExpense,
			NetBalance:      200,
		},
		{
			AccountID:       6,
			AccountCode:     "9101",
			AccountName:     "Beban Pajak Penghasilan",
			AccountType:     models.AccountTypeExpense,
			AccountCategory: models.CategoryTaxExpense,
			NetBalance:      100,
		},
	}

	report := service.generateProfitLossFromBalances(balances, start, end)

	require.Equal(t, 1000.0, report.Revenue.TotalRevenue)
	require.Equal(t, 400.0, report.COGS.TotalCOGS)
	require.Equal(t, 300.0, report.OperatingExpenses.TotalOpEx)
	require.Equal(t, 300.0, report.OperatingIncome)
	require.Equal(t, 50.0, report.OtherIncome)
	require.Equal(t, 200.0, report.OtherExpenses)
	require.Equal(t, 150.0, report.IncomeBeforeTax)
	require.Equal(t, 100.0, report.TaxExpense)
	require.Equal(t, 50.0, report.NetIncome)

	require.Len(t, report.OtherIncomeItems, 1)
	require.Equal(t, "7103", report.OtherIncomeItems[0].AccountCode)

	require.Len(t, report.OtherExpenseItems, 1)
	require.Equal(t, "8103", report.OtherExpenseItems[0].AccountCode)

	require.Len(t, report.TaxExpenseItems, 1)
	require.Equal(t, "9101", report.TaxExpenseItems[0].AccountCode)
}
