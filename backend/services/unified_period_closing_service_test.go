package services

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newPeriodClosingTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	return db
}

func TestCountOperationalJournalEntriesExcludesSystemJournals(t *testing.T) {
	db := newPeriodClosingTestDB(t)

	if err := db.Exec(`
		CREATE TABLE unified_journal_ledger (
			id INTEGER PRIMARY KEY,
			entry_number TEXT,
			source_type TEXT,
			entry_date DATETIME,
			status TEXT,
			deleted_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create unified_journal_ledger: %v", err)
	}

	entryDate := normalizeClosingDayStart(time.Date(2026, time.March, 18, 0, 0, 0, 0, time.Local))
	rows := []struct {
		id         int
		entryNo    string
		sourceType string
	}{
		{1, "SALE-1", "SALE"},
		{2, "CLS-1", "CLOSING"},
		{3, "REV-1", "REVERSAL"},
	}
	for _, row := range rows {
		if err := db.Exec(
			`INSERT INTO unified_journal_ledger (id, entry_number, source_type, entry_date, status) VALUES (?, ?, ?, ?, ?)`,
			row.id, row.entryNo, row.sourceType, entryDate, "POSTED",
		).Error; err != nil {
			t.Fatalf("insert journal %s: %v", row.entryNo, err)
		}
	}

	count, err := countOperationalJournalEntries(db, entryDate, normalizeClosingDayEnd(entryDate))
	if err != nil {
		t.Fatalf("count operational journals: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 operational journal, got %d", count)
	}
}

func TestAppendPeriodNoteDeduplicatesLines(t *testing.T) {
	existing := "Validasi ok\nDitutup pada 19 Mar 2026 10:00 oleh user #1. Jurnal closing ID: 10"
	addition := "Validasi ok\nDibuka kembali pada 19 Mar 2026 11:00 oleh user #1. Alasan: koreksi"

	got := appendPeriodNote(existing, addition)

	lines := splitPeriodNoteLines(got)
	if len(lines) != 3 {
		t.Fatalf("expected 3 unique note lines, got %d: %v", len(lines), lines)
	}
	if strings.Count(got, "Validasi ok") != 1 {
		t.Fatalf("expected duplicate validation note to be removed, got %q", got)
	}
}

func TestBuildPeriodHistoryEventsParsesStructuredNotes(t *testing.T) {
	closedAt := normalizeClosingDayEnd(time.Date(2026, time.March, 19, 0, 0, 0, 0, time.Local))
	reopenedAt := closedAt.Add(2 * time.Hour)
	snapshotAt := reopenedAt.Add(time.Hour)
	closingJournalID := uint(33)

	period := models.AccountingPeriod{
		Notes:               "Validasi ok\nDitutup pada 19 Mar 2026 17:55 oleh user #2. Jurnal closing ID: 33\nDibuka kembali pada 19 Mar 2026 19:55 oleh user #2. Alasan: koreksi",
		ClosedAt:            &closedAt,
		ReopenedAt:          &reopenedAt,
		ReopenReason:        "koreksi",
		SnapshotGeneratedAt: &snapshotAt,
		ClosingJournalID:    &closingJournalID,
	}

	events := buildPeriodHistoryEvents(period)
	if len(events) < 4 {
		t.Fatalf("expected at least 4 history events, got %d: %#v", len(events), events)
	}

	if events[0]["type"] != "SNAPSHOT" {
		t.Fatalf("expected latest event to be snapshot, got %#v", events[0])
	}

	var hasClosed bool
	var hasReopened bool
	for _, event := range events {
		switch event["type"] {
		case "CLOSED":
			hasClosed = true
		case "REOPENED":
			hasReopened = true
		}
	}

	if !hasClosed || !hasReopened {
		t.Fatalf("expected closed and reopened events, got %#v", events)
	}
}

func TestFindAccountingPeriodByCanonicalRangeRejectsDuplicates(t *testing.T) {
	db := newPeriodClosingTestDB(t)

	if err := db.Exec(`
		CREATE TABLE accounting_periods (
			id INTEGER PRIMARY KEY,
			start_date DATETIME NOT NULL,
			end_date DATETIME NOT NULL,
			is_closed BOOLEAN DEFAULT FALSE,
			deleted_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create accounting_periods: %v", err)
	}

	start := normalizeClosingDayStart(time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local))
	end := normalizeClosingDayEnd(time.Date(2026, time.March, 31, 0, 0, 0, 0, time.Local))

	for i := 1; i <= 2; i++ {
		if err := db.Exec(
			`INSERT INTO accounting_periods (id, start_date, end_date, is_closed) VALUES (?, ?, ?, ?)`,
			i, start, end, true,
		).Error; err != nil {
			t.Fatalf("insert accounting period %d: %v", i, err)
		}
	}

	_, err := findAccountingPeriodByCanonicalRange(db, start, normalizeClosingDayStart(end), true)
	if err == nil {
		t.Fatal("expected duplicate period error, got nil")
	}
	if !strings.Contains(err.Error(), "multiple accounting periods found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFindSingleActiveClosingJournalUsesOnlyUnreversedEntry(t *testing.T) {
	db := newPeriodClosingTestDB(t)

	if err := db.Exec(`
		CREATE TABLE unified_journal_ledger (
			id INTEGER PRIMARY KEY,
			entry_number TEXT,
			source_type TEXT,
			source_code TEXT,
			entry_date DATETIME,
			status TEXT,
			reversed_by BIGINT,
			deleted_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create unified_journal_ledger: %v", err)
	}

	entryDate := normalizeClosingDayEnd(time.Date(2026, time.March, 31, 0, 0, 0, 0, time.Local))
	sourceCode := buildClosingPeriodSourceCode(entryDate, entryDate)

	if err := db.Exec(
		`INSERT INTO unified_journal_ledger (id, entry_number, source_type, source_code, entry_date, status, reversed_by) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		1, "CLOSING-OLD", "CLOSING", sourceCode, entryDate.Add(-time.Hour), "POSTED", 99,
	).Error; err != nil {
		t.Fatalf("insert reversed closing journal: %v", err)
	}
	if err := db.Exec(
		`INSERT INTO unified_journal_ledger (id, entry_number, source_type, source_code, entry_date, status) VALUES (?, ?, ?, ?, ?, ?)`,
		2, "CLOSING-NEW", "CLOSING", sourceCode, entryDate, "POSTED",
	).Error; err != nil {
		t.Fatalf("insert active closing journal: %v", err)
	}

	journal, err := findSingleActiveClosingJournal(
		db,
		"test-source-code",
		db.Where("source_type = ? AND source_code = ? AND status = ?", "CLOSING", sourceCode, "POSTED"),
	)
	if err != nil {
		t.Fatalf("find active closing journal: %v", err)
	}
	if journal.ID != 2 {
		t.Fatalf("expected active closing journal id 2, got %d", journal.ID)
	}
}

func TestFindSingleActiveClosingJournalErrorsWhenMultipleActiveEntriesExist(t *testing.T) {
	db := newPeriodClosingTestDB(t)

	if err := db.Exec(`
		CREATE TABLE unified_journal_ledger (
			id INTEGER PRIMARY KEY,
			entry_number TEXT,
			source_type TEXT,
			source_code TEXT,
			entry_date DATETIME,
			status TEXT,
			reversed_by BIGINT,
			deleted_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create unified_journal_ledger: %v", err)
	}

	entryDate := normalizeClosingDayEnd(time.Date(2026, time.March, 31, 0, 0, 0, 0, time.Local))
	sourceCode := buildClosingPeriodSourceCode(entryDate, entryDate)

	for i := 1; i <= 2; i++ {
		if err := db.Exec(
			`INSERT INTO unified_journal_ledger (id, entry_number, source_type, source_code, entry_date, status) VALUES (?, ?, ?, ?, ?, ?)`,
			i, fmt.Sprintf("CLOSING-DUP-%d", i), "CLOSING", sourceCode, entryDate, "POSTED",
		).Error; err != nil {
			t.Fatalf("insert duplicate closing journal %d: %v", i, err)
		}
	}

	_, err := findSingleActiveClosingJournal(
		db,
		"test-source-code",
		db.Where("source_type = ? AND source_code = ? AND status = ?", "CLOSING", sourceCode, "POSTED"),
	)
	if err == nil {
		t.Fatal("expected multiple active closing journals error, got nil")
	}
	if !strings.Contains(err.Error(), "lebih dari satu jurnal closing aktif") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSSOTProfitLossExcludesClosingAndReversalEntries(t *testing.T) {
	db := newPeriodClosingTestDB(t)

	if err := db.Exec(`
		CREATE TABLE accounts (
			id INTEGER PRIMARY KEY,
			code TEXT,
			name TEXT,
			type TEXT,
			category TEXT,
			balance DECIMAL(20,2),
			deleted_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create accounts: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE unified_journal_ledger (
			id INTEGER PRIMARY KEY,
			source_type TEXT,
			entry_date DATETIME,
			status TEXT,
			deleted_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create unified_journal_ledger: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE unified_journal_lines (
			id INTEGER PRIMARY KEY,
			journal_id BIGINT,
			account_id BIGINT,
			debit_amount DECIMAL(20,2),
			credit_amount DECIMAL(20,2)
		)
	`).Error; err != nil {
		t.Fatalf("create unified_journal_lines: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE journal_entries (
			id INTEGER PRIMARY KEY,
			entry_date DATETIME,
			status TEXT,
			deleted_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create journal_entries: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE journal_lines (
			id INTEGER PRIMARY KEY,
			journal_entry_id BIGINT,
			account_id BIGINT,
			debit_amount DECIMAL(20,2),
			credit_amount DECIMAL(20,2)
		)
	`).Error; err != nil {
		t.Fatalf("create journal_lines: %v", err)
	}

	if err := db.Exec(
		`INSERT INTO accounts (id, code, name, type, category, balance) VALUES (?, ?, ?, ?, ?, ?)`,
		1, "4101", "Penjualan", "REVENUE", "OPERATING_REVENUE", 0,
	).Error; err != nil {
		t.Fatalf("insert revenue account: %v", err)
	}

	entryDate := normalizeClosingDayStart(time.Date(2026, time.March, 30, 0, 0, 0, 0, time.Local))
	journals := []struct {
		id         int
		sourceType string
		credit     float64
		debit      float64
	}{
		{1, "SALE", 100, 0},
		{2, "CLOSING", 0, 100},
		{3, "REVERSAL", 100, 0},
	}
	for _, journal := range journals {
		if err := db.Exec(
			`INSERT INTO unified_journal_ledger (id, source_type, entry_date, status) VALUES (?, ?, ?, ?)`,
			journal.id, journal.sourceType, entryDate, "POSTED",
		).Error; err != nil {
			t.Fatalf("insert journal %d: %v", journal.id, err)
		}
		if err := db.Exec(
			`INSERT INTO unified_journal_lines (id, journal_id, account_id, debit_amount, credit_amount) VALUES (?, ?, ?, ?, ?)`,
			journal.id, journal.id, 1, journal.debit, journal.credit,
		).Error; err != nil {
			t.Fatalf("insert journal line %d: %v", journal.id, err)
		}
	}

	svc := NewSSOTProfitLossService(db)
	startDate, err := time.Parse("2006-01-02", "2026-03-01")
	if err != nil {
		t.Fatalf("parse start date: %v", err)
	}
	endDate, err := time.Parse("2006-01-02", "2026-03-31")
	if err != nil {
		t.Fatalf("parse end date: %v", err)
	}
	balances, source, err := svc.getAccountBalancesFromSSOT(startDate, endDate.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("get account balances from ssot: %v", err)
	}
	if source != "SSOT" {
		t.Fatalf("expected SSOT source, got %s", source)
	}
	if len(balances) != 1 {
		t.Fatalf("expected 1 balance row, got %d", len(balances))
	}
	if balances[0].CreditTotal != 100 || balances[0].DebitTotal != 0 || balances[0].NetBalance != 100 {
		t.Fatalf("expected only operational sale amounts to remain, got debit=%.2f credit=%.2f net=%.2f",
			balances[0].DebitTotal, balances[0].CreditTotal, balances[0].NetBalance)
	}
}

func TestCurrentEarningsStartDateUsesDayAfterLastClosedPeriod(t *testing.T) {
	db := newPeriodClosingTestDB(t)

	if err := db.Exec(`
		CREATE TABLE accounting_periods (
			id INTEGER PRIMARY KEY,
			start_date DATETIME NOT NULL,
			end_date DATETIME NOT NULL,
			is_closed BOOLEAN DEFAULT FALSE,
			deleted_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create accounting_periods: %v", err)
	}

	start := normalizeClosingDayStart(time.Date(2026, time.March, 18, 0, 0, 0, 0, time.Local))
	end := normalizeClosingDayEnd(time.Date(2026, time.April, 18, 0, 0, 0, 0, time.Local))
	if err := db.Exec(
		`INSERT INTO accounting_periods (id, start_date, end_date, is_closed) VALUES (?, ?, ?, ?)`,
		1, start, end, true,
	).Error; err != nil {
		t.Fatalf("insert accounting period: %v", err)
	}

	svc := NewSSOTBalanceSheetService(db)
	asOf := normalizeClosingDayStart(time.Date(2026, time.April, 20, 0, 0, 0, 0, time.Local))
	currentStart := svc.currentEarningsStartDate(asOf)
	if currentStart == nil {
		t.Fatal("expected current earnings start date, got nil")
	}
	expected := normalizeClosingDayStart(time.Date(2026, time.April, 19, 0, 0, 0, 0, time.Local))
	if !currentStart.Equal(expected) {
		t.Fatalf("expected current earnings start %s, got %s", expected.Format(time.RFC3339), currentStart.Format(time.RFC3339))
	}
}

func TestCurrentEarningsStartDateReturnsNilWithoutClosedPeriods(t *testing.T) {
	db := newPeriodClosingTestDB(t)

	if err := db.Exec(`
		CREATE TABLE accounting_periods (
			id INTEGER PRIMARY KEY,
			start_date DATETIME NOT NULL,
			end_date DATETIME NOT NULL,
			is_closed BOOLEAN DEFAULT FALSE,
			deleted_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create accounting_periods: %v", err)
	}

	svc := NewSSOTBalanceSheetService(db)
	asOf := normalizeClosingDayStart(time.Date(2026, time.April, 20, 0, 0, 0, 0, time.Local))
	if got := svc.currentEarningsStartDate(asOf); got != nil {
		t.Fatalf("expected nil current earnings start date, got %s", got.Format(time.RFC3339))
	}
}

func TestValidateClosingWindowFirstClosingIncludesBlockingMessageInValidationMessages(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("GO_ENV", "")
	t.Setenv("ENVIRONMENT", "")
	t.Setenv("ALLOW_SAME_DAY_PERIOD_CLOSING", "")

	db := newPeriodClosingTestDB(t)

	if err := db.Exec(`
		CREATE TABLE accounting_periods (
			id INTEGER PRIMARY KEY,
			start_date DATETIME NOT NULL,
			end_date DATETIME NOT NULL,
			is_closed BOOLEAN DEFAULT FALSE,
			deleted_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create accounting_periods: %v", err)
	}

	svc := NewUnifiedPeriodClosingService(db)
	startDate := normalizeClosingDayStart(time.Now().AddDate(0, 0, -1))
	endDate := normalizeClosingDayStart(time.Now())

	validationMessages, blockingMessages, lastClosedPeriod, err := svc.validateClosingWindow(db, startDate, endDate)
	if err != nil {
		t.Fatalf("validate closing window: %v", err)
	}
	if lastClosedPeriod != nil {
		t.Fatalf("expected no last closed period, got %#v", lastClosedPeriod)
	}
	if len(blockingMessages) == 0 {
		t.Fatal("expected blocking message for end date that has not fully ended")
	}

	if !strings.Contains(strings.Join(validationMessages, " | "), "First period closing - no previous closed period found.") {
		t.Fatalf("expected first closing informational message, got %v", validationMessages)
	}
	if !strings.Contains(strings.Join(validationMessages, " | "), "End date must be before") {
		t.Fatalf("expected blocking message to be surfaced in validation messages, got %v", validationMessages)
	}
}

func TestValidateClosingWindowAllowsSameDayClosingInDevelopment(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("GO_ENV", "")
	t.Setenv("ENVIRONMENT", "")
	t.Setenv("ALLOW_SAME_DAY_PERIOD_CLOSING", "")

	db := newPeriodClosingTestDB(t)

	if err := db.Exec(`
		CREATE TABLE accounting_periods (
			id INTEGER PRIMARY KEY,
			start_date DATETIME NOT NULL,
			end_date DATETIME NOT NULL,
			is_closed BOOLEAN DEFAULT FALSE,
			deleted_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create accounting_periods: %v", err)
	}

	svc := NewUnifiedPeriodClosingService(db)
	startDate := normalizeClosingDayStart(time.Now().AddDate(0, 0, -1))
	endDate := normalizeClosingDayStart(time.Now())

	validationMessages, blockingMessages, lastClosedPeriod, err := svc.validateClosingWindow(db, startDate, endDate)
	if err != nil {
		t.Fatalf("validate closing window: %v", err)
	}
	if lastClosedPeriod != nil {
		t.Fatalf("expected no last closed period, got %#v", lastClosedPeriod)
	}
	if len(blockingMessages) != 0 {
		t.Fatalf("expected same-day closing to be allowed in development, got %v", blockingMessages)
	}
	if !strings.Contains(strings.Join(validationMessages, " | "), "First period closing - no previous closed period found.") {
		t.Fatalf("expected first closing informational message, got %v", validationMessages)
	}
}

func TestIsEffectiveClosedAccountingPeriodProductionRequiresPreviousDay(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("GO_ENV", "")
	t.Setenv("ENVIRONMENT", "")
	t.Setenv("ALLOW_SAME_DAY_PERIOD_CLOSING", "")

	now := normalizeClosingDayStart(time.Date(2026, time.March, 21, 10, 0, 0, 0, time.Local))
	sameDayClosed := models.AccountingPeriod{
		IsClosed: true,
		EndDate:  normalizeClosingDayEnd(time.Date(2026, time.March, 21, 0, 0, 0, 0, time.Local)),
	}
	previousDayClosed := models.AccountingPeriod{
		IsClosed: true,
		EndDate:  normalizeClosingDayEnd(time.Date(2026, time.March, 20, 0, 0, 0, 0, time.Local)),
	}

	if isEffectiveClosedAccountingPeriod(sameDayClosed, now) {
		t.Fatal("expected same-day closed period to remain pending in production")
	}
	if !isEffectiveClosedAccountingPeriod(previousDayClosed, now) {
		t.Fatal("expected previous-day closed period to be effective in production")
	}
}

func TestIsEffectiveClosedAccountingPeriodDevelopmentAllowsSameDay(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("GO_ENV", "")
	t.Setenv("ENVIRONMENT", "")
	t.Setenv("ALLOW_SAME_DAY_PERIOD_CLOSING", "")

	now := normalizeClosingDayStart(time.Date(2026, time.March, 21, 10, 0, 0, 0, time.Local))
	sameDayClosed := models.AccountingPeriod{
		IsClosed: true,
		EndDate:  normalizeClosingDayEnd(time.Date(2026, time.March, 21, 0, 0, 0, 0, time.Local)),
	}

	if !isEffectiveClosedAccountingPeriod(sameDayClosed, now) {
		t.Fatal("expected same-day closed period to be effective in development")
	}
}
