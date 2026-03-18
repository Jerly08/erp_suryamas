package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"app-sistem-akuntansi/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type duplicateRangeRow struct {
	StartDate string
	EndDate   string
	Rows      int
	IDs       string
}

type periodIssueRow struct {
	ID                 uint
	StartDate          string
	EndDate            string
	IsClosed           bool
	ClosingJournalID   *uint
	ReopeningJournalID *uint
	ReopenCount        int
	SnapshotGenerated  *string
}

type journalIssueRow struct {
	ID          uint64
	EntryNumber string
	SourceCode  string
	EntryDate   string
	Extra       string
}

func main() {
	cfg := config.LoadConfig()

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	findingCount := 0

	fmt.Println("============================================================================")
	fmt.Println("PERIOD CLOSING AUDIT")
	fmt.Println("============================================================================")
	fmt.Println()

	duplicateRanges := auditDuplicateRanges(db)
	findingCount += printDuplicateRanges(duplicateRanges)

	staleClosedPeriods := auditClosedPeriodsWithReopenMetadata(db)
	findingCount += printPeriodIssues(
		"Closed periods with stale reopen metadata",
		"Closed periods should not keep reopened_at / reopening_journal_id / reopen_reason fields once reclosed.",
		staleClosedPeriods,
	)

	openPeriodsWithSnapshots := auditOpenPeriodsWithSnapshots(db)
	findingCount += printPeriodIssues(
		"Open periods with stale snapshots",
		"Open periods should not retain archived snapshots because their balances can still change.",
		openPeriodsWithSnapshots,
	)

	closedPeriodsWithoutSnapshots := auditClosedPeriodsWithoutSnapshots(db)
	findingCount += printPeriodIssues(
		"Closed periods without snapshots",
		"Closed periods should have archive snapshots once the refreshed closing flow runs.",
		closedPeriodsWithoutSnapshots,
	)

	invalidPeriodJournalLinks := auditInvalidPeriodJournalLinks(db)
	findingCount += printJournalIssues(
		"Periods linked to missing or non-closing journals",
		"Closing journal references should always point to an existing CLOSING journal entry.",
		invalidPeriodJournalLinks,
	)

	ambiguousActiveClosings := auditAmbiguousActiveClosingJournals(db)
	findingCount += printJournalIssues(
		"Ambiguous active closing journals",
		"There should be at most one unreversed active closing journal per source_code.",
		ambiguousActiveClosings,
	)

	orphanReversals := auditOrphanReversals(db)
	findingCount += printJournalIssues(
		"Orphan or mismatched reversal journals",
		"Reversal journals should always point back to an existing closing journal, and closing journals should reciprocate via reversed_by.",
		orphanReversals,
	)

	fmt.Println("----------------------------------------------------------------------------")
	if findingCount == 0 {
		fmt.Println("No audit findings detected.")
	} else {
		fmt.Printf("Audit finished with %d finding(s).\n", findingCount)
	}
	fmt.Println("----------------------------------------------------------------------------")

	if findingCount > 0 {
		os.Exit(1)
	}
}

func auditDuplicateRanges(db *gorm.DB) []duplicateRangeRow {
	var rows []duplicateRangeRow
	err := db.Raw(`
		SELECT
			TO_CHAR(DATE(start_date), 'YYYY-MM-DD') AS start_date,
			TO_CHAR(DATE(end_date), 'YYYY-MM-DD') AS end_date,
			COUNT(*) AS rows,
			STRING_AGG(id::text, ', ' ORDER BY id) AS ids
		FROM accounting_periods
		WHERE deleted_at IS NULL
		GROUP BY DATE(start_date), DATE(end_date)
		HAVING COUNT(*) > 1
		ORDER BY DATE(end_date) DESC, DATE(start_date) DESC
	`).Scan(&rows).Error
	if err != nil {
		log.Fatalf("failed to audit duplicate ranges: %v", err)
	}
	return rows
}

func auditClosedPeriodsWithReopenMetadata(db *gorm.DB) []periodIssueRow {
	var rows []periodIssueRow
	err := db.Raw(`
		SELECT
			id,
			TO_CHAR(start_date, 'YYYY-MM-DD') AS start_date,
			TO_CHAR(end_date, 'YYYY-MM-DD') AS end_date,
			is_closed,
			closing_journal_id,
			reopening_journal_id,
			reopen_count,
			CASE WHEN snapshot_generated_at IS NULL THEN NULL ELSE TO_CHAR(snapshot_generated_at, 'YYYY-MM-DD HH24:MI:SS') END AS snapshot_generated
		FROM accounting_periods
		WHERE deleted_at IS NULL
		  AND is_closed = TRUE
		  AND (
			reopened_at IS NOT NULL OR
			reopened_by IS NOT NULL OR
			COALESCE(TRIM(reopen_reason), '') <> '' OR
			reopening_journal_id IS NOT NULL
		  )
		ORDER BY end_date DESC, id DESC
	`).Scan(&rows).Error
	if err != nil {
		log.Fatalf("failed to audit stale reopen metadata: %v", err)
	}
	return rows
}

func auditOpenPeriodsWithSnapshots(db *gorm.DB) []periodIssueRow {
	var rows []periodIssueRow
	err := db.Raw(`
		SELECT
			id,
			TO_CHAR(start_date, 'YYYY-MM-DD') AS start_date,
			TO_CHAR(end_date, 'YYYY-MM-DD') AS end_date,
			is_closed,
			closing_journal_id,
			reopening_journal_id,
			reopen_count,
			CASE WHEN snapshot_generated_at IS NULL THEN NULL ELSE TO_CHAR(snapshot_generated_at, 'YYYY-MM-DD HH24:MI:SS') END AS snapshot_generated
		FROM accounting_periods
		WHERE deleted_at IS NULL
		  AND is_closed = FALSE
		  AND (
			snapshot_generated_at IS NOT NULL OR
			balance_sheet_snapshot IS NOT NULL OR
			profit_loss_snapshot IS NOT NULL OR
			financial_metrics IS NOT NULL
		  )
		ORDER BY end_date DESC, id DESC
	`).Scan(&rows).Error
	if err != nil {
		log.Fatalf("failed to audit open periods with snapshots: %v", err)
	}
	return rows
}

func auditClosedPeriodsWithoutSnapshots(db *gorm.DB) []periodIssueRow {
	var rows []periodIssueRow
	err := db.Raw(`
		SELECT
			id,
			TO_CHAR(start_date, 'YYYY-MM-DD') AS start_date,
			TO_CHAR(end_date, 'YYYY-MM-DD') AS end_date,
			is_closed,
			closing_journal_id,
			reopening_journal_id,
			reopen_count,
			CASE WHEN snapshot_generated_at IS NULL THEN NULL ELSE TO_CHAR(snapshot_generated_at, 'YYYY-MM-DD HH24:MI:SS') END AS snapshot_generated
		FROM accounting_periods
		WHERE deleted_at IS NULL
		  AND is_closed = TRUE
		  AND snapshot_generated_at IS NULL
		ORDER BY end_date DESC, id DESC
	`).Scan(&rows).Error
	if err != nil {
		log.Fatalf("failed to audit closed periods without snapshots: %v", err)
	}
	return rows
}

func auditInvalidPeriodJournalLinks(db *gorm.DB) []journalIssueRow {
	var rows []journalIssueRow
	err := db.Raw(`
		SELECT
			ap.id::bigint AS id,
			COALESCE(uje.entry_number, '(missing)') AS entry_number,
			COALESCE(uje.source_code, '(missing)') AS source_code,
			TO_CHAR(ap.end_date, 'YYYY-MM-DD') AS entry_date,
			CASE
				WHEN ap.closing_journal_id IS NULL THEN 'closing_journal_id is NULL'
				WHEN uje.id IS NULL THEN 'closing_journal_id points to missing journal'
				WHEN uje.source_type <> 'CLOSING' THEN 'linked journal source_type=' || uje.source_type
				ELSE 'invalid closing journal link'
			END AS extra
		FROM accounting_periods ap
		LEFT JOIN unified_journal_ledger uje ON uje.id = ap.closing_journal_id
		WHERE ap.deleted_at IS NULL
		  AND ap.is_closed = TRUE
		  AND (
			ap.closing_journal_id IS NULL OR
			uje.id IS NULL OR
			uje.source_type <> 'CLOSING'
		  )
		ORDER BY ap.end_date DESC, ap.id DESC
	`).Scan(&rows).Error
	if err != nil {
		log.Fatalf("failed to audit invalid period journal links: %v", err)
	}
	return rows
}

func auditAmbiguousActiveClosingJournals(db *gorm.DB) []journalIssueRow {
	var rows []journalIssueRow
	err := db.Raw(`
		SELECT
			MIN(id) AS id,
			MIN(entry_number) AS entry_number,
			source_code,
			TO_CHAR(MAX(entry_date), 'YYYY-MM-DD HH24:MI:SS') AS entry_date,
			'active_closing_count=' || COUNT(*)::text || ', ids=' || STRING_AGG(id::text, ', ' ORDER BY id) AS extra
		FROM unified_journal_ledger
		WHERE deleted_at IS NULL
		  AND status = 'POSTED'
		  AND source_type = 'CLOSING'
		  AND reversed_by IS NULL
		GROUP BY source_code
		HAVING COUNT(*) > 1
		ORDER BY MAX(entry_date) DESC, source_code
	`).Scan(&rows).Error
	if err != nil {
		log.Fatalf("failed to audit ambiguous active closings: %v", err)
	}
	return rows
}

func auditOrphanReversals(db *gorm.DB) []journalIssueRow {
	var rows []journalIssueRow
	err := db.Raw(`
		SELECT
			r.id,
			r.entry_number,
			COALESCE(r.source_code, '') AS source_code,
			TO_CHAR(r.entry_date, 'YYYY-MM-DD HH24:MI:SS') AS entry_date,
			CASE
				WHEN c.id IS NULL THEN 'reversed_from points to missing journal'
				WHEN c.source_type <> 'CLOSING' THEN 'reversed_from points to non-closing journal'
				WHEN c.reversed_by IS DISTINCT FROM r.id THEN 'closing.reversed_by does not point back to reversal'
				ELSE 'invalid reversal link'
			END AS extra
		FROM unified_journal_ledger r
		LEFT JOIN unified_journal_ledger c ON c.id = r.reversed_from
		WHERE r.deleted_at IS NULL
		  AND r.source_type = 'REVERSAL'
		  AND (
			r.reversed_from IS NULL OR
			c.id IS NULL OR
			c.source_type <> 'CLOSING' OR
			c.reversed_by IS DISTINCT FROM r.id
		  )
		ORDER BY r.entry_date DESC, r.id DESC
	`).Scan(&rows).Error
	if err != nil {
		log.Fatalf("failed to audit orphan reversals: %v", err)
	}
	return rows
}

func printDuplicateRanges(rows []duplicateRangeRow) int {
	fmt.Println("Duplicate canonical ranges")
	fmt.Println("Duplicate rows for the same start/end day will break period matching and reopen safety.")
	if len(rows) == 0 {
		fmt.Println("  none")
		fmt.Println()
		return 0
	}
	for _, row := range rows {
		fmt.Printf("  %s -> %s | rows=%d | ids=%s\n", row.StartDate, row.EndDate, row.Rows, row.IDs)
	}
	fmt.Println()
	return len(rows)
}

func printPeriodIssues(title, description string, rows []periodIssueRow) int {
	fmt.Println(title)
	fmt.Println(description)
	if len(rows) == 0 {
		fmt.Println("  none")
		fmt.Println()
		return 0
	}
	for _, row := range rows {
		snapshot := "-"
		if row.SnapshotGenerated != nil && strings.TrimSpace(*row.SnapshotGenerated) != "" {
			snapshot = *row.SnapshotGenerated
		}

		closingID := "-"
		if row.ClosingJournalID != nil {
			closingID = fmt.Sprintf("%d", *row.ClosingJournalID)
		}

		reopeningID := "-"
		if row.ReopeningJournalID != nil {
			reopeningID = fmt.Sprintf("%d", *row.ReopeningJournalID)
		}

		fmt.Printf(
			"  period_id=%d | %s -> %s | closed=%t | closing_journal_id=%s | reopening_journal_id=%s | reopen_count=%d | snapshot=%s\n",
			row.ID, row.StartDate, row.EndDate, row.IsClosed, closingID, reopeningID, row.ReopenCount, snapshot,
		)
	}
	fmt.Println()
	return len(rows)
}

func printJournalIssues(title, description string, rows []journalIssueRow) int {
	fmt.Println(title)
	fmt.Println(description)
	if len(rows) == 0 {
		fmt.Println("  none")
		fmt.Println()
		return 0
	}
	for _, row := range rows {
		fmt.Printf(
			"  id=%d | entry=%s | source_code=%s | entry_date=%s | %s\n",
			row.ID, row.EntryNumber, row.SourceCode, row.EntryDate, row.Extra,
		)
	}
	fmt.Println()
	return len(rows)
}

func init() {
	// Keep timestamps stable in console output.
	time.Local = time.FixedZone("Local", 7*60*60)
}
