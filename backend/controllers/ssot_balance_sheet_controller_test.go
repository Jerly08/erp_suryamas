package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGenerateSSOTBalanceSheet_KeepsCurrentYearEarningsAfterPriorClosing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		if strings.Contains(err.Error(), "requires cgo") {
			t.Skip("sqlite driver is unavailable in this environment")
		}
		require.NoError(t, err)
	}

	mustExec := func(sql string, values ...interface{}) {
		require.NoError(t, db.Exec(sql, values...).Error)
	}

	mustExec(`
		CREATE TABLE settings (
			id INTEGER PRIMARY KEY,
			company_name TEXT,
			company_address TEXT,
			company_phone TEXT,
			company_email TEXT,
			date_format TEXT,
			deleted_at DATETIME
		)
	`)
	mustExec(`
		INSERT INTO settings (id, company_name, company_address, company_phone, company_email, date_format, deleted_at)
		VALUES (1, 'PT Test ERP', '', '', '', 'YYYY-MM-DD', NULL)
	`)
	mustExec(`
		CREATE TABLE accounts (
			id INTEGER PRIMARY KEY,
			code TEXT,
			name TEXT,
			type TEXT,
			category TEXT,
			is_header BOOLEAN,
			deleted_at DATETIME,
			balance DECIMAL(20,2)
		)
	`)
	mustExec(`
		CREATE TABLE unified_journal_ledger (
			id INTEGER PRIMARY KEY,
			entry_number TEXT,
			source_type TEXT,
			source_code TEXT,
			entry_date DATETIME,
			description TEXT,
			status TEXT,
			deleted_at DATETIME,
			total_debit DECIMAL(20,2),
			total_credit DECIMAL(20,2),
			is_balanced BOOLEAN,
			is_auto_generated BOOLEAN,
			created_by INTEGER
		)
	`)
	mustExec(`
		CREATE TABLE unified_journal_lines (
			id INTEGER PRIMARY KEY,
			journal_id INTEGER,
			account_id INTEGER,
			line_number INTEGER,
			description TEXT,
			debit_amount DECIMAL(20,2),
			credit_amount DECIMAL(20,2)
		)
	`)

	mustExec(`INSERT INTO accounts (id, code, name, type, category, is_header, deleted_at, balance) VALUES
		(1, '1101', 'Kas', 'ASSET', 'CURRENT_ASSET', 0, NULL, 0),
		(2, '3101', 'Modal Disetor', 'EQUITY', 'SHARE_CAPITAL', 0, NULL, 0),
		(3, '3201', 'Laba Ditahan', 'EQUITY', 'RETAINED_EARNINGS', 0, NULL, 0),
		(4, '4101', 'Pendapatan Penjualan', 'REVENUE', 'OPERATING_REVENUE', 0, NULL, 0),
		(5, '6101', 'Beban Operasional', 'EXPENSE', 'OPERATING_EXPENSE', 0, NULL, 0)
	`)

	mustExec(`INSERT INTO unified_journal_ledger (id, entry_number, source_type, source_code, entry_date, description, status, deleted_at, total_debit, total_credit, is_balanced, is_auto_generated, created_by) VALUES
		(1, 'JE-OPEN', 'MANUAL', 'OPENING', '2025-01-01', 'Opening capital', 'POSTED', NULL, 100, 100, 1, 0, 1),
		(2, 'JE-SALES-2025', 'MANUAL', 'SALE2025', '2025-06-01', 'Sales 2025', 'POSTED', NULL, 100, 100, 1, 0, 1),
		(3, 'JE-EXP-2025', 'MANUAL', 'EXP2025', '2025-09-01', 'Expense 2025', 'POSTED', NULL, 60, 60, 1, 0, 1),
		(4, 'JE-CLOSE-2025', 'CLOSING', 'CLOSE2025', '2025-12-31', 'Close 2025', 'POSTED', NULL, 100, 100, 1, 1, 1),
		(5, 'JE-SALES-2026', 'MANUAL', 'SALE2026', '2026-03-10', 'Sales 2026', 'POSTED', NULL, 50, 50, 1, 0, 1)
	`)

	mustExec(`INSERT INTO unified_journal_lines (id, journal_id, account_id, line_number, description, debit_amount, credit_amount) VALUES
		(1, 1, 1, 1, 'Kas', 100, 0),
		(2, 1, 2, 2, 'Modal', 0, 100),
		(3, 2, 1, 1, 'Kas dari penjualan', 100, 0),
		(4, 2, 4, 2, 'Pendapatan penjualan', 0, 100),
		(5, 3, 5, 1, 'Beban operasional', 60, 0),
		(6, 3, 1, 2, 'Kas keluar', 0, 60),
		(7, 4, 4, 1, 'Tutup pendapatan', 100, 0),
		(8, 4, 5, 2, 'Tutup beban', 0, 60),
		(9, 4, 3, 3, 'Laba ditahan', 0, 40),
		(10, 5, 1, 1, 'Kas dari penjualan 2026', 50, 0),
		(11, 5, 4, 2, 'Pendapatan penjualan 2026', 0, 50)
	`)

	controller := NewSSOTBalanceSheetController(db)
	router := gin.New()
	router.GET("/balance-sheet", controller.GenerateSSOTBalanceSheet)

	req := httptest.NewRequest(http.MethodGet, "/balance-sheet?as_of_date=2026-03-31", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Data struct {
			IsBalanced                bool    `json:"is_balanced"`
			BalanceDifference         float64 `json:"balance_difference"`
			TotalLiabilitiesAndEquity float64 `json:"total_liabilities_and_equity"`
			Assets                    struct {
				TotalAssets float64 `json:"total_assets"`
			} `json:"assets"`
			Equity struct {
				TotalEquity float64 `json:"total_equity"`
			} `json:"equity"`
		} `json:"data"`
	}

	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Data.IsBalanced)
	require.InDelta(t, 0.0, response.Data.BalanceDifference, 0.0001)
	require.InDelta(t, 190.0, response.Data.Assets.TotalAssets, 0.0001)
	require.InDelta(t, 190.0, response.Data.TotalLiabilitiesAndEquity, 0.0001)
	require.InDelta(t, 190.0, response.Data.Equity.TotalEquity, 0.0001)
}
