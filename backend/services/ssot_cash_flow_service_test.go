package services

import (
	"math"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSSOTCashFlowOpeningBalanceWithinPeriodIsBeginningCash(t *testing.T) {
	db := setupSSOTCashFlowTestDB(t)
	service := NewSSOTCashFlowService(db)

	openingDate := mustParseCashFlowDate(t, "2026-06-22")
	insertCashFlowTestAccount(t, db, 1, "1101-001", "KAS KECIL", "ASSET", "")
	insertCashFlowTestAccount(t, db, 2, "3000", "EKUITAS", "EQUITY", "")
	insertCashFlowTestJournal(t, db, 1, "OPENING", openingDate, 1000, 1000)
	insertCashFlowTestLine(t, db, 1, 1, 1, 1000, 0)
	insertCashFlowTestLine(t, db, 2, 1, 2, 0, 1000)

	start := mustParseCashFlowDate(t, "2026-06-22")
	endExclusive := mustParseCashFlowDate(t, "2026-06-30")

	beginningCash, endingCash, err := service.getCashBalances(start, endExclusive)
	if err != nil {
		t.Fatalf("getCashBalances returned error: %v", err)
	}

	assertCashFlowAmount(t, beginningCash, 1000, "beginning cash")
	assertCashFlowAmount(t, endingCash, 1000, "ending cash")

	transactions, err := service.getCashFlowTransactions(start, endExclusive)
	if err != nil {
		t.Fatalf("getCashFlowTransactions returned error: %v", err)
	}
	if len(transactions) != 0 {
		t.Fatalf("expected opening balance to be excluded from cash flow activity, got %d transaction(s): %#v", len(transactions), transactions)
	}
}

func TestSSOTCashFlowOpeningBalanceBeforePeriodRemainsBeginningCash(t *testing.T) {
	db := setupSSOTCashFlowTestDB(t)
	service := NewSSOTCashFlowService(db)

	openingDate := mustParseCashFlowDate(t, "2026-06-22")
	insertCashFlowTestAccount(t, db, 1, "1101-001", "KAS KECIL", "ASSET", "")
	insertCashFlowTestAccount(t, db, 2, "3000", "EKUITAS", "EQUITY", "")
	insertCashFlowTestJournal(t, db, 1, "OPENING", openingDate, 2500, 2500)
	insertCashFlowTestLine(t, db, 1, 1, 1, 2500, 0)
	insertCashFlowTestLine(t, db, 2, 1, 2, 0, 2500)

	start := mustParseCashFlowDate(t, "2026-06-24")
	endExclusive := mustParseCashFlowDate(t, "2026-06-30")

	beginningCash, endingCash, err := service.getCashBalances(start, endExclusive)
	if err != nil {
		t.Fatalf("getCashBalances returned error: %v", err)
	}

	assertCashFlowAmount(t, beginningCash, 2500, "beginning cash")
	assertCashFlowAmount(t, endingCash, 2500, "ending cash")
}

func setupSSOTCashFlowTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}

	statements := []string{
		`CREATE TABLE accounts (
			id INTEGER PRIMARY KEY,
			code TEXT,
			name TEXT,
			type TEXT,
			category TEXT,
			deleted_at DATETIME,
			is_header BOOLEAN DEFAULT false
		)`,
		`CREATE TABLE unified_journal_ledger (
			id INTEGER PRIMARY KEY,
			source_type TEXT,
			entry_date DATETIME,
			status TEXT,
			deleted_at DATETIME,
			total_debit NUMERIC,
			total_credit NUMERIC
		)`,
		`CREATE TABLE unified_journal_lines (
			id INTEGER PRIMARY KEY,
			journal_id INTEGER,
			account_id INTEGER,
			debit_amount NUMERIC,
			credit_amount NUMERIC
		)`,
	}

	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("failed to run schema statement: %v", err)
		}
	}

	return db
}

func insertCashFlowTestAccount(t *testing.T, db *gorm.DB, id int, code, name, accountType, category string) {
	t.Helper()

	if err := db.Exec(
		`INSERT INTO accounts (id, code, name, type, category, is_header) VALUES (?, ?, ?, ?, ?, false)`,
		id, code, name, accountType, category,
	).Error; err != nil {
		t.Fatalf("failed to insert account %s: %v", code, err)
	}
}

func insertCashFlowTestJournal(t *testing.T, db *gorm.DB, id int, sourceType string, entryDate time.Time, debit, credit float64) {
	t.Helper()

	if err := db.Exec(
		`INSERT INTO unified_journal_ledger (id, source_type, entry_date, status, total_debit, total_credit) VALUES (?, ?, ?, 'POSTED', ?, ?)`,
		id, sourceType, entryDate, debit, credit,
	).Error; err != nil {
		t.Fatalf("failed to insert journal %d: %v", id, err)
	}
}

func insertCashFlowTestLine(t *testing.T, db *gorm.DB, id, journalID, accountID int, debit, credit float64) {
	t.Helper()

	if err := db.Exec(
		`INSERT INTO unified_journal_lines (id, journal_id, account_id, debit_amount, credit_amount) VALUES (?, ?, ?, ?, ?)`,
		id, journalID, accountID, debit, credit,
	).Error; err != nil {
		t.Fatalf("failed to insert journal line %d: %v", id, err)
	}
}

func mustParseCashFlowDate(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		t.Fatalf("failed to parse date %s: %v", value, err)
	}
	return parsed
}

func assertCashFlowAmount(t *testing.T, actual, expected float64, label string) {
	t.Helper()

	if math.Abs(actual-expected) > 0.0001 {
		t.Fatalf("expected %s %.4f, got %.4f", label, expected, actual)
	}
}
