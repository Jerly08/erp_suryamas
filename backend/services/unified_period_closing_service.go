package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/utils"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

var ErrNoBalancesToClose = errors.New("no revenue or expense balances to close")

// UnifiedPeriodClosingService handles period closing using unified journal system (SSOT)
type UnifiedPeriodClosingService struct {
	db                    *gorm.DB
	unifiedJournalService *UnifiedJournalService
	logger                *utils.JournalLogger
}

// NewUnifiedPeriodClosingService creates a new unified period closing service
func NewUnifiedPeriodClosingService(db *gorm.DB) *UnifiedPeriodClosingService {
	return &UnifiedPeriodClosingService{
		db:                    db,
		unifiedJournalService: NewUnifiedJournalService(db),
		logger:                utils.NewJournalLogger(db),
	}
}

func normalizeClosingDayStart(date time.Time) time.Time {
	location := date.Location()
	if location == nil {
		location = time.Local
	}
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, location)
}

func normalizeClosingDayEnd(date time.Time) time.Time {
	start := normalizeClosingDayStart(date)
	return start.AddDate(0, 0, 1).Add(-time.Nanosecond)
}

func parseDateOnlyInLocation(dateStr string, location *time.Location) (time.Time, error) {
	if location == nil {
		location = time.Local
	}
	return time.ParseInLocation("2006-01-02", dateStr, location)
}

func mustParseDateOnlyInLocation(dateStr string, location *time.Location) time.Time {
	date, err := parseDateOnlyInLocation(dateStr, location)
	if err != nil {
		panic(err)
	}
	return date
}

func isStartOfDay(date time.Time) bool {
	return date.Hour() == 0 && date.Minute() == 0 && date.Second() == 0 && date.Nanosecond() == 0
}

func isEndOfDay(date time.Time) bool {
	return date.Hour() == 23 && date.Minute() == 59 && date.Second() == 59
}

func periodBoundaryDateString(date time.Time, boundary string) string {
	local := date.In(time.Local)
	utc := date.UTC()

	switch boundary {
	case "start":
		if !isStartOfDay(local) && isStartOfDay(utc) {
			return utc.Format("2006-01-02")
		}
	case "end":
		if !isEndOfDay(local) && isEndOfDay(utc) {
			return utc.Format("2006-01-02")
		}
	}

	return local.Format("2006-01-02")
}

func canonicalPeriodStartDate(date time.Time) time.Time {
	return mustParseDateOnlyInLocation(periodBoundaryDateString(date, "start"), time.Local)
}

func canonicalPeriodEndDate(date time.Time) time.Time {
	return mustParseDateOnlyInLocation(periodBoundaryDateString(date, "end"), time.Local)
}

func dateOnlyRangeOverlaps(startA, endA, startB, endB time.Time) bool {
	return !endA.Before(startB) && !endB.Before(startA)
}

func nextClosingStartDate(date time.Time) time.Time {
	return canonicalPeriodEndDate(date).AddDate(0, 0, 1)
}

func sameClosingDay(left, right time.Time) bool {
	return canonicalPeriodStartDate(left).Equal(canonicalPeriodStartDate(right))
}

func buildClosingPeriodSourceCode(startDate, endDate time.Time) string {
	return fmt.Sprintf("CLOSING-%s-%s", normalizeClosingDayStart(startDate).Format("20060102"), normalizeClosingDayStart(endDate).Format("20060102"))
}

func nonSystemJournalSourceTypes() []string {
	return []string{
		models.SSOTSourceTypeClosing,
		models.SSOTSourceTypeReversal,
	}
}

func countOperationalJournalEntries(tx *gorm.DB, startDate, endDate time.Time) (int64, error) {
	var transactionCount int64
	err := tx.Model(&models.SSOTJournalEntry{}).
		Where("entry_date BETWEEN ? AND ? AND status = ? AND source_type NOT IN ? AND deleted_at IS NULL",
			startDate, endDate, models.SSOTStatusPosted, nonSystemJournalSourceTypes()).
		Count(&transactionCount).Error
	return transactionCount, err
}

func clearPeriodSnapshotFields(values map[string]interface{}) map[string]interface{} {
	values["balance_sheet_snapshot"] = nil
	values["profit_loss_snapshot"] = nil
	values["financial_metrics"] = nil
	values["snapshot_generated_at"] = nil
	return values
}

func (s *UnifiedPeriodClosingService) refreshPeriodSnapshot(ctx context.Context, tx *gorm.DB, periodID uint) error {
	archiveService := NewPeriodArchiveService(tx.WithContext(ctx))
	if err := archiveService.CreatePeriodSnapshot(ctx, periodID); err != nil {
		return fmt.Errorf("failed to refresh period snapshot: %w", err)
	}
	return nil
}

func appendPeriodNote(existing, addition string) string {
	lines := splitPeriodNoteLines(existing)
	additions := splitPeriodNoteLines(addition)

	if len(lines) == 0 {
		return strings.Join(additions, "\n")
	}
	if len(additions) == 0 {
		return strings.Join(lines, "\n")
	}

	seen := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		seen[line] = struct{}{}
	}

	for _, line := range additions {
		if _, exists := seen[line]; exists {
			continue
		}
		lines = append(lines, line)
		seen[line] = struct{}{}
	}

	return strings.Join(lines, "\n")
}

func splitPeriodNoteLines(notes string) []string {
	normalized := strings.ReplaceAll(notes, "\r\n", "\n")
	rawLines := strings.Split(normalized, "\n")
	lines := make([]string, 0, len(rawLines))

	for _, line := range rawLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lines = append(lines, trimmed)
	}

	return lines
}

func formatPeriodHistoryTimestamp(timestamp time.Time) string {
	return timestamp.In(time.Local).Format(time.RFC3339)
}

func formatPeriodNoteTimestamp(timestamp time.Time) string {
	return timestamp.In(time.Local).Format("02 Jan 2006 15:04")
}

func parsePeriodNoteTimestamp(raw string) (*time.Time, bool) {
	parsed, err := time.ParseInLocation("02 Jan 2006 15:04", strings.TrimSpace(raw), time.Local)
	if err != nil {
		return nil, false
	}
	return &parsed, true
}

func buildClosedPeriodNote(now time.Time, userID uint64, closingJournalID uint) string {
	return fmt.Sprintf("Ditutup pada %s oleh user #%d. Jurnal closing ID: %d", formatPeriodNoteTimestamp(now), userID, closingJournalID)
}

func buildReopenedPeriodNote(now time.Time, userID uint64, reason string) string {
	return fmt.Sprintf("Dibuka kembali pada %s oleh user #%d. Alasan: %s", formatPeriodNoteTimestamp(now), userID, reason)
}

type periodHistoryEvent struct {
	Type        string
	Title       string
	Description string
	At          *time.Time
}

func buildPeriodHistoryEvents(period models.AccountingPeriod) []map[string]interface{} {
	events := make([]periodHistoryEvent, 0, 6)
	seen := make(map[string]struct{})

	addEvent := func(event periodHistoryEvent) {
		if event.At != nil {
			key := fmt.Sprintf("%s|%s", event.Type, event.At.UTC().Format(time.RFC3339Nano))
			if _, exists := seen[key]; exists {
				return
			}
			seen[key] = struct{}{}
			events = append(events, event)
			return
		}

		key := fmt.Sprintf("%s|%s", event.Type, event.Description)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		events = append(events, event)
	}

	for _, line := range splitPeriodNoteLines(period.Notes) {
		switch {
		case strings.HasPrefix(line, "Ditutup pada "):
			timestampSegment := line[len("Ditutup pada "):]
			if idx := strings.Index(timestampSegment, " oleh "); idx >= 0 {
				timestampSegment = timestampSegment[:idx]
			}
			timestamp, _ := parsePeriodNoteTimestamp(timestampSegment)
			addEvent(periodHistoryEvent{
				Type:        "CLOSED",
				Title:       "Periode ditutup",
				Description: line,
				At:          timestamp,
			})
		case strings.HasPrefix(line, "Dibuka kembali pada "):
			timestampSegment := line[len("Dibuka kembali pada "):]
			if idx := strings.Index(timestampSegment, " oleh "); idx >= 0 {
				timestampSegment = timestampSegment[:idx]
			}
			timestamp, _ := parsePeriodNoteTimestamp(timestampSegment)
			addEvent(periodHistoryEvent{
				Type:        "REOPENED",
				Title:       "Periode dibuka kembali",
				Description: line,
				At:          timestamp,
			})
		default:
			addEvent(periodHistoryEvent{
				Type:        "NOTE",
				Title:       "Catatan closing",
				Description: line,
			})
		}
	}

	if period.ClosedAt != nil {
		description := "Period closing berhasil diposting"
		if period.ClosingJournalID != nil {
			description = fmt.Sprintf("%s dengan jurnal closing ID %d", description, *period.ClosingJournalID)
		}
		addEvent(periodHistoryEvent{
			Type:        "CLOSED",
			Title:       "Periode ditutup",
			Description: description,
			At:          period.ClosedAt,
		})
	}

	if period.ReopenedAt != nil {
		description := "Period closing dibuka kembali"
		if period.ReopenReason != "" {
			description = fmt.Sprintf("%s. Alasan: %s", description, period.ReopenReason)
		}
		addEvent(periodHistoryEvent{
			Type:        "REOPENED",
			Title:       "Periode dibuka kembali",
			Description: description,
			At:          period.ReopenedAt,
		})
	}

	if period.SnapshotGeneratedAt != nil {
		addEvent(periodHistoryEvent{
			Type:        "SNAPSHOT",
			Title:       "Snapshot archive diperbarui",
			Description: "Snapshot laporan archive berhasil direfresh setelah perubahan periode",
			At:          period.SnapshotGeneratedAt,
		})
	}

	sort.SliceStable(events, func(i, j int) bool {
		left := events[i]
		right := events[j]

		switch {
		case left.At == nil && right.At == nil:
			return left.Title < right.Title
		case left.At == nil:
			return false
		case right.At == nil:
			return true
		default:
			return left.At.After(*right.At)
		}
	})

	result := make([]map[string]interface{}, 0, len(events))
	for _, event := range events {
		item := map[string]interface{}{
			"type":        event.Type,
			"title":       event.Title,
			"description": event.Description,
		}
		if event.At != nil {
			item["at"] = formatPeriodHistoryTimestamp(*event.At)
		}
		result = append(result, item)
	}

	return result
}

func findAccountingPeriodByCanonicalRange(tx *gorm.DB, startDate, endDate time.Time, onlyClosed bool) (*models.AccountingPeriod, error) {
	var periods []models.AccountingPeriod
	query := tx.Where("deleted_at IS NULL")
	if onlyClosed {
		query = query.Where("is_closed = ?", true)
	}

	if err := query.Order("end_date DESC").Find(&periods).Error; err != nil {
		return nil, err
	}

	inputStart := normalizeClosingDayStart(startDate)
	inputEnd := normalizeClosingDayStart(endDate)
	var matched *models.AccountingPeriod

	for _, period := range periods {
		periodStart := canonicalPeriodStartDate(period.StartDate)
		periodEnd := canonicalPeriodEndDate(period.EndDate)
		if periodStart.Equal(inputStart) && periodEnd.Equal(inputEnd) {
			if matched != nil {
				return nil, fmt.Errorf("multiple accounting periods found for %s to %s", inputStart.Format("2006-01-02"), inputEnd.Format("2006-01-02"))
			}
			candidate := period
			matched = &candidate
		}
	}

	if matched != nil {
		return matched, nil
	}

	return nil, gorm.ErrRecordNotFound
}

func findSingleActiveClosingJournal(tx *gorm.DB, label string, query *gorm.DB) (*models.SSOTJournalEntry, error) {
	var closingJournals []models.SSOTJournalEntry
	if err := query.
		Where("reversed_by IS NULL").
		Order("entry_date DESC, id DESC").
		Limit(2).
		Find(&closingJournals).Error; err != nil {
		return nil, err
	}

	if len(closingJournals) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	if len(closingJournals) > 1 {
		return nil, fmt.Errorf("ditemukan lebih dari satu jurnal closing aktif untuk %s", label)
	}

	return &closingJournals[0], nil
}

func syncCurrentAccountBalances(tx *gorm.DB, accountIDs []uint64) error {
	if len(accountIDs) == 0 {
		return nil
	}

	type balanceRow struct {
		AccountID uint64
		Correct   float64
	}

	var rows []balanceRow
	if err := tx.Raw(`
		SELECT
			a.id AS account_id,
			CASE
				WHEN UPPER(TRIM(a.type)) IN ('ASSET', 'EXPENSE') THEN
					COALESCE(SUM(ujl.debit_amount), 0) - COALESCE(SUM(ujl.credit_amount), 0)
				ELSE
					COALESCE(SUM(ujl.credit_amount), 0) - COALESCE(SUM(ujl.debit_amount), 0)
			END AS correct
		FROM accounts a
		LEFT JOIN unified_journal_lines ujl ON ujl.account_id = a.id
		LEFT JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
		WHERE a.id IN ? AND a.deleted_at IS NULL
			AND (uje.id IS NULL OR (uje.status = 'POSTED' AND uje.deleted_at IS NULL))
		GROUP BY a.id, a.type
	`, accountIDs).Scan(&rows).Error; err != nil {
		return fmt.Errorf("failed to sync current balances from posted journals: %w", err)
	}

	for _, row := range rows {
		if err := tx.Model(&models.Account{}).
			Where("id = ?", row.AccountID).
			Update("balance", row.Correct).Error; err != nil {
			return fmt.Errorf("failed to update current balance for account %d: %w", row.AccountID, err)
		}
	}

	return nil
}

func syncAllHeaderAccountBalances(tx *gorm.DB) error {
	for pass := 0; pass < 5; pass++ {
		if err := tx.Exec(`
			UPDATE accounts
			SET balance = (
				SELECT COALESCE(SUM(child.balance), 0)
				FROM accounts child
				WHERE child.parent_id = accounts.id AND child.deleted_at IS NULL
			)
			WHERE is_header = true AND deleted_at IS NULL
		`).Error; err != nil {
			return fmt.Errorf("failed to sync header account balances on pass %d: %w", pass+1, err)
		}
	}

	return nil
}

func (s *UnifiedPeriodClosingService) createClosingReversalEntry(
	tx *gorm.DB,
	original models.SSOTJournalEntry,
	lines []models.SSOTJournalLine,
	period models.AccountingPeriod,
	reason string,
	reversalDate time.Time,
	userID uint64,
) (*models.SSOTJournalEntry, error) {
	reversalLines := make([]models.SSOTJournalLine, 0, len(lines))
	for _, line := range lines {
		reversalLines = append(reversalLines, models.SSOTJournalLine{
			AccountID:    line.AccountID,
			LineNumber:   line.LineNumber,
			Description:  fmt.Sprintf("Reverse closing line: %s", line.Description),
			DebitAmount:  line.CreditAmount,
			CreditAmount: line.DebitAmount,
		})
	}

	reversalSourceID := original.ID
	reversalEntry := &models.SSOTJournalEntry{
		SourceType:      models.SSOTSourceTypeReversal,
		SourceID:        &reversalSourceID,
		SourceCode:      fmt.Sprintf("REOPEN-%d", original.ID),
		EntryDate:       reversalDate,
		Description:     fmt.Sprintf("Reopen period reversal: %s", original.Description),
		Reference:       fmt.Sprintf("REOPEN-%s", original.EntryNumber),
		Notes:           fmt.Sprintf("Auto reversal for reopened period %s - %s. Original closing journal ID: %d. Reason: %s", normalizeClosingDayStart(period.StartDate).Format("2006-01-02"), normalizeClosingDayStart(period.EndDate).Format("2006-01-02"), original.ID, reason),
		TotalDebit:      original.TotalCredit,
		TotalCredit:     original.TotalDebit,
		Status:          models.SSOTStatusPosted,
		IsBalanced:      true,
		IsAutoGenerated: true,
		CreatedBy:       userID,
		ReversedFrom:    &original.ID,
		Lines:           reversalLines,
	}

	now := time.Now()
	reversalEntry.PostedAt = &now
	reversalEntry.PostedBy = &userID

	if err := tx.Create(reversalEntry).Error; err != nil {
		return nil, fmt.Errorf("failed to create closing reversal journal: %w", err)
	}

	if err := tx.Model(&models.SSOTJournalEntry{}).
		Where("id = ?", original.ID).
		Updates(map[string]interface{}{
			"reversed_by":     reversalEntry.ID,
			"reversal_reason": reason,
			"updated_at":      now,
		}).Error; err != nil {
		return nil, fmt.Errorf("failed to update original closing journal reversal metadata: %w", err)
	}

	return reversalEntry, nil
}

func (s *UnifiedPeriodClosingService) validateClosingWindow(tx *gorm.DB, startDate, endDate time.Time) ([]string, []string, *models.AccountingPeriod, error) {
	var validationMessages []string
	var blockingMessages []string

	startDate = normalizeClosingDayStart(startDate)
	endDate = normalizeClosingDayEnd(endDate)

	if endDate.Before(startDate) {
		return nil, nil, nil, fmt.Errorf("end_date must be on or after start_date")
	}

	var closedPeriods []models.AccountingPeriod
	if err := tx.Where("is_closed = ? AND deleted_at IS NULL", true).Order("end_date DESC").Find(&closedPeriods).Error; err != nil {
		return nil, nil, nil, fmt.Errorf("failed to query closed periods: %v", err)
	}
	if len(closedPeriods) == 0 {
		validationMessages = append(validationMessages, "First period closing - no previous closed period found.")
		return validationMessages, blockingMessages, nil, nil
	}

	lastPeriod := closedPeriods[0]
	expectedStart := nextClosingStartDate(lastPeriod.EndDate)
	if !sameClosingDay(startDate, expectedStart) {
		blockingMessages = append(blockingMessages,
			fmt.Sprintf("Start date must be %s based on the latest closed period, but got %s.",
				expectedStart.Format("2006-01-02"), startDate.Format("2006-01-02")))
	}

	if !normalizeClosingDayStart(endDate).After(canonicalPeriodEndDate(lastPeriod.EndDate)) {
		blockingMessages = append(blockingMessages, "End date must be after last closing date.")
	}

	inputStart := normalizeClosingDayStart(startDate)
	inputEnd := normalizeClosingDayStart(endDate)
	for _, period := range closedPeriods {
		periodStart := canonicalPeriodStartDate(period.StartDate)
		periodEnd := canonicalPeriodEndDate(period.EndDate)
		if dateOnlyRangeOverlaps(inputStart, inputEnd, periodStart, periodEnd) {
			blockingMessages = append(blockingMessages, "Selected period overlaps an already closed period.")
			break
		}
	}

	validationMessages = append(validationMessages, blockingMessages...)
	return validationMessages, blockingMessages, &lastPeriod, nil
}

// ExecutePeriodClosing performs period closing using unified journal system
func (s *UnifiedPeriodClosingService) ExecutePeriodClosing(ctx context.Context, startDate, endDate time.Time, description string, userID uint64) error {
	startDate = normalizeClosingDayStart(startDate)
	endDate = normalizeClosingDayEnd(endDate)
	log.Printf("[UNIFIED CLOSING] Starting period closing: %s to %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	return s.db.Transaction(func(tx *gorm.DB) error {
		validationMessages, blockingMessages, lastClosedPeriod, err := s.validateClosingWindow(tx, startDate, endDate)
		if err != nil {
			return err
		}
		if len(blockingMessages) > 0 {
			return fmt.Errorf("period closing validation failed: %s", strings.Join(blockingMessages, " "))
		}

		// 1. Get retained earnings account
		var retainedEarnings models.Account
		if err := tx.Where("code = ? AND type = ?", "3102", "EQUITY").First(&retainedEarnings).Error; err != nil {
			return fmt.Errorf("retained earnings account (3102) not found: %v", err)
		}

		// 2. Get the last closed period (for logging & validation only)
		//    Saldo yang ditutup tetap dihitung dari SSOT sampai endDate, sehingga tidak tergantung startDate,
		//    dan tidak akan terjadi double-closing untuk periode sebelumnya.
		var periodStartDate time.Time
		if lastClosedPeriod == nil {
			// No previous closed periods, use the start_date provided
			periodStartDate = startDate
			log.Printf("[UNIFIED CLOSING] No previous closed periods found, using provided start date: %s", startDate.Format("2006-01-02"))
		} else {
			// Start from the day after last closed period
			periodStartDate = nextClosingStartDate(lastClosedPeriod.EndDate)
			log.Printf("[UNIFIED CLOSING] Last closed period end: %s, starting from: %s",
				lastClosedPeriod.EndDate.Format("2006-01-02"), periodStartDate.Format("2006-01-02"))

			// Validate that startDate matches periodStartDate
			if !sameClosingDay(startDate, periodStartDate) {
				log.Printf("[UNIFIED CLOSING] ⚠️ WARNING: Provided startDate (%s) doesn't match expected (%s)",
					startDate.Format("2006-01-02"), periodStartDate.Format("2006-01-02"))
			}
		}

		// 3. Get all revenue accounts (including headers if they have balance)
		var revenueAccounts []models.Account
		// CRITICAL FIX: Removed is_header=false and added localized type support with ILIKE
		if err := tx.Where("TRIM(type) ILIKE 'REVENUE' OR TRIM(type) ILIKE 'PENDAPATAN%'").
			Find(&revenueAccounts).Error; err != nil {
			return fmt.Errorf("failed to get revenue accounts: %v", err)
		}

		// 4. Get all expense accounts (including headers if they have balance)
		var expenseAccounts []models.Account
		// CRITICAL FIX: Removed is_header=false and added localized type support with ILIKE
		if err := tx.Where("TRIM(type) ILIKE 'EXPENSE' OR TRIM(type) ILIKE 'BEBAN%'").
			Find(&expenseAccounts).Error; err != nil {
			return fmt.Errorf("failed to get expense accounts: %v", err)
		}

		// 5. Hitung saldo SEBENARNYA per akun sampai tanggal closing (endDate)
		//    Menggunakan SSOT (unified_journal_ledger + unified_journal_lines)
		//    Rumus umum: saldo = SUM(debit) - SUM(kredit) untuk SEMUA transaksi (termasuk closing sebelumnya)
		//    Dengan begitu, yang kita tutup hanyalah saldo TEMPORARY yang masih tersisa.
		type AccountBalance struct {
			AccountID uint64
			AccType   string
			Net       float64 // Debit - Credit sampai endDate
		}

		// Kumpulkan semua ID akun revenue + expense untuk query SSOT
		revenueAccountIDs := make([]uint64, 0, len(revenueAccounts))
		for _, acc := range revenueAccounts {
			revenueAccountIDs = append(revenueAccountIDs, uint64(acc.ID))
		}

		expenseAccountIDs := make([]uint64, 0, len(expenseAccounts))
		for _, acc := range expenseAccounts {
			expenseAccountIDs = append(expenseAccountIDs, uint64(acc.ID))
		}

		allTempAccountIDs := append(revenueAccountIDs, expenseAccountIDs...)

		var balances []AccountBalance
		if len(allTempAccountIDs) > 0 {
			// Query saldo nyata dari unified journals sampai endDate
			if err := tx.Raw(`
				SELECT 
					a.id AS account_id,
					a.type AS acc_type,
					COALESCE(SUM(ujl.debit_amount), 0) - COALESCE(SUM(ujl.credit_amount), 0) AS net
				FROM accounts a
				LEFT JOIN unified_journal_lines ujl ON ujl.account_id = a.id
				LEFT JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
				WHERE a.id IN ? 
					AND a.deleted_at IS NULL
					AND (uje.id IS NULL OR (uje.status = 'POSTED' AND uje.deleted_at IS NULL AND uje.entry_date <= ?))
				GROUP BY a.id, a.type
			`, allTempAccountIDs, endDate).Scan(&balances).Error; err != nil {
				return fmt.Errorf("failed to calculate account balances for closing: %v", err)
			}
		}

		// 6. Dari saldo nyata, hitung berapa yang harus ditutup per akun.
		//    saldo_sekarang = SUM(debit) - SUM(kredit)
		//    delta_closing  = -saldo_sekarang  (agar saldo baru = 0)
		//    Untuk baris jurnal: delta_closing = debit_closing - credit_closing
		closingDeltaMap := make(map[uint64]decimal.Decimal) // debit_closing - credit_closing per akun

		threshold := decimal.NewFromFloat(0.01)
		var totalRevenue, totalExpense decimal.Decimal

		for _, bal := range balances {
			currentBalance := decimal.NewFromFloat(bal.Net) // bisa negatif (credit) atau positif (debit)
			if currentBalance.Abs().LessThan(threshold) {
				continue // akun sudah effectively 0
			}

			// Delta yang perlu diposting supaya saldo menjadi 0
			closingDelta := currentBalance.Neg() // debit_closing - credit_closing
			closingDeltaMap[bal.AccountID] = closingDelta

			amountToClose := closingDelta.Abs() // nilai positif untuk laporan (Rp)
			// Normalize type for checking
			accTypeNormalized := strings.ToUpper(strings.TrimSpace(bal.AccType))

			if accTypeNormalized == "REVENUE" || strings.HasPrefix(accTypeNormalized, "PENDAPATAN") {
				// Revenue bersifat kredit normal → total revenue pakai nilai absolut
				totalRevenue = totalRevenue.Add(amountToClose)
			} else {
				// Expense & others
				totalExpense = totalExpense.Add(amountToClose)
			}
		}

		if totalRevenue.Abs().LessThan(threshold) && totalExpense.Abs().LessThan(threshold) {
			log.Println("[UNIFIED CLOSING] No revenue or expense balances to close")
			return ErrNoBalancesToClose
		}

		netIncome := totalRevenue.Sub(totalExpense)

		log.Printf("[UNIFIED CLOSING] Period Start Date: %s", periodStartDate.Format("2006-01-02"))
		log.Printf("[UNIFIED CLOSING] Period End Date (Closing): %s", endDate.Format("2006-01-02"))
		log.Printf("[UNIFIED CLOSING] Period Revenue: %.2f, Period Expense: %.2f, Net Income: %.2f",
			totalRevenue.InexactFloat64(), totalExpense.InexactFloat64(), netIncome.InexactFloat64())

		// 7. Buat baris jurnal closing berdasarkan delta_closing per akun
		var journalLines []models.SSOTJournalLine
		lineNum := 1

		// Helper kecil untuk membuat baris jurnal satu akun berdasarkan delta_closing
		buildLine := func(acc models.Account, closingDelta decimal.Decimal, desc string) *models.SSOTJournalLine {
			amount := closingDelta.Abs()
			if amount.LessThan(threshold) {
				return nil
			}

			debitAmount := decimal.Zero
			creditAmount := decimal.Zero
			if closingDelta.GreaterThan(decimal.Zero) {
				// debit_closing - credit_closing = +amount → letakkan di DEBIT
				debitAmount = amount
			} else {
				// delta negatif → butuh CREDIT lebih besar
				creditAmount = amount
			}

			return &models.SSOTJournalLine{
				AccountID:    uint64(acc.ID),
				LineNumber:   lineNum,
				Description:  desc,
				DebitAmount:  debitAmount,
				CreditAmount: creditAmount,
			}
		}

		// Tutup akun Revenue
		for _, acc := range revenueAccounts {
			closingDelta, exists := closingDeltaMap[uint64(acc.ID)]
			if !exists {
				continue
			}
			line := buildLine(acc, closingDelta, fmt.Sprintf("Close period revenue: %s", acc.Name))
			if line != nil {
				journalLines = append(journalLines, *line)
				log.Printf("[UNIFIED CLOSING] Closing Revenue: %s (ID: %d) Delta: %.2f", acc.Name, acc.ID, closingDelta.Abs().InexactFloat64())
				lineNum++
			}
		}

		// Tutup akun Expense
		for _, acc := range expenseAccounts {
			closingDelta, exists := closingDeltaMap[uint64(acc.ID)]
			if !exists {
				continue
			}
			line := buildLine(acc, closingDelta, fmt.Sprintf("Close period expense: %s", acc.Name))
			if line != nil {
				journalLines = append(journalLines, *line)
				log.Printf("[UNIFIED CLOSING] Closing Expense: %s (ID: %d) Delta: %.2f", acc.Name, acc.ID, closingDelta.Abs().InexactFloat64())
				lineNum++
			}
		}

		// Pindahkan total revenue & expense ke Retained Earnings
		if totalRevenue.GreaterThan(decimal.Zero) {
			journalLines = append(journalLines, models.SSOTJournalLine{
				AccountID:    uint64(retainedEarnings.ID),
				LineNumber:   lineNum,
				Description:  "Transfer revenue to retained earnings",
				DebitAmount:  decimal.Zero,
				CreditAmount: totalRevenue,
			})
			lineNum++
		}

		if totalExpense.GreaterThan(decimal.Zero) {
			journalLines = append(journalLines, models.SSOTJournalLine{
				AccountID:    uint64(retainedEarnings.ID),
				LineNumber:   lineNum,
				Description:  "Transfer expense from retained earnings",
				DebitAmount:  totalExpense,
				CreditAmount: decimal.Zero,
			})
			lineNum++
		}

		closingSourceCode := buildClosingPeriodSourceCode(startDate, endDate)
		validationNote := strings.Join(validationMessages, " | ")

		// 8. Create unified journal entry
		closingEntry := &models.SSOTJournalEntry{
			SourceType:      "CLOSING",
			SourceCode:      closingSourceCode,
			EntryDate:       endDate,
			Description:     description,
			Reference:       closingSourceCode,
			Notes:           validationNote,
			TotalDebit:      totalRevenue.Add(totalExpense),
			TotalCredit:     totalRevenue.Add(totalExpense),
			Status:          "POSTED",
			IsBalanced:      true,
			IsAutoGenerated: true,
			CreatedBy:       userID,
			Lines:           journalLines,
		}

		now := time.Now()
		closingEntry.PostedAt = &now
		closingEntry.PostedBy = &userID

		// Create journal entry in unified system (this will automatically update balances)
		if err := tx.Create(closingEntry).Error; err != nil {
			return fmt.Errorf("failed to create unified closing journal: %v", err)
		}

		log.Printf("[UNIFIED CLOSING] Created unified journal entry ID: %d with %d lines", closingEntry.ID, len(journalLines))

		// 7. Sync current account balances from all POSTED journals.
		// This keeps `accounts.balance` as current balance even if newer open-period transactions already exist.
		affectedIDsMap := make(map[uint64]struct{})
		for _, line := range journalLines {
			affectedIDsMap[line.AccountID] = struct{}{}
		}
		var affectedIDs []uint64
		for id := range affectedIDsMap {
			affectedIDs = append(affectedIDs, id)
		}

		if len(affectedIDs) > 0 {
			if err := syncCurrentAccountBalances(tx, affectedIDs); err != nil {
				return err
			}
			if err := syncAllHeaderAccountBalances(tx); err != nil {
				return err
			}

			// VALIDATION: Verify that all Revenue and Expense balances are zero as of the closed period end date.
			var nonZeroCount int64
			if err := tx.Raw(`
				SELECT COUNT(*)
				FROM (
					SELECT a.id
					FROM accounts a
					LEFT JOIN unified_journal_lines ujl ON ujl.account_id = a.id
					LEFT JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
					WHERE (
						TRIM(a.type) ILIKE 'REVENUE' OR TRIM(a.type) ILIKE 'PENDAPATAN%' OR
						TRIM(a.type) ILIKE 'EXPENSE' OR TRIM(a.type) ILIKE 'BEBAN%'
					)
						AND a.deleted_at IS NULL
						AND (uje.id IS NULL OR (uje.status = 'POSTED' AND uje.deleted_at IS NULL AND uje.entry_date <= ?))
					GROUP BY a.id
					HAVING ABS(COALESCE(SUM(ujl.debit_amount), 0) - COALESCE(SUM(ujl.credit_amount), 0)) > 0.01
				) AS non_zero_accounts
			`, endDate).Scan(&nonZeroCount).Error; err != nil {
				return fmt.Errorf("failed to validate zero balances at period end: %v", err)
			}

			if nonZeroCount > 0 {
				type problemAccount struct {
					Code    string
					Name    string
					Balance float64
				}

				var problemAccounts []problemAccount
				if err := tx.Raw(`
					SELECT
						a.code,
						a.name,
						COALESCE(SUM(ujl.debit_amount), 0) - COALESCE(SUM(ujl.credit_amount), 0) AS balance
					FROM accounts a
					LEFT JOIN unified_journal_lines ujl ON ujl.account_id = a.id
					LEFT JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
					WHERE (
						TRIM(a.type) ILIKE 'REVENUE' OR TRIM(a.type) ILIKE 'PENDAPATAN%' OR
						TRIM(a.type) ILIKE 'EXPENSE' OR TRIM(a.type) ILIKE 'BEBAN%'
					)
						AND a.deleted_at IS NULL
						AND (uje.id IS NULL OR (uje.status = 'POSTED' AND uje.deleted_at IS NULL AND uje.entry_date <= ?))
					GROUP BY a.id, a.code, a.name
					HAVING ABS(COALESCE(SUM(ujl.debit_amount), 0) - COALESCE(SUM(ujl.credit_amount), 0)) > 0.01
				`, endDate).Scan(&problemAccounts).Error; err == nil {
					for _, acc := range problemAccounts {
						log.Printf("[UNIFIED CLOSING] WARNING: Account %s (%s) still has historical balance at close date: %.2f",
							acc.Code, acc.Name, acc.Balance)
					}
				}
				return fmt.Errorf("closing validation failed: %d revenue/expense accounts still have non-zero balance at closing date", nonZeroCount)
			}

			log.Printf("[UNIFIED CLOSING] Validation passed: All Revenue and Expense balances are zero at closing date")
		}

		transactionCount, err := countOperationalJournalEntries(tx, startDate, endDate)
		if err != nil {
			return fmt.Errorf("failed to count transactions for accounting period: %v", err)
		}

		closingJournalID := uint(closingEntry.ID)
		periodNotes := appendPeriodNote(validationNote, buildClosedPeriodNote(now, userID, closingJournalID))

		// 9. Create or refresh accounting period record
		userIDUint := uint(userID)
		accountingPeriodValues := clearPeriodSnapshotFields(map[string]interface{}{
			"description":          description,
			"is_closed":            true,
			"is_locked":            false,
			"closed_by":            &userIDUint,
			"closed_at":            &now,
			"total_revenue":        totalRevenue.InexactFloat64(),
			"total_expense":        totalExpense.InexactFloat64(),
			"net_income":           netIncome.InexactFloat64(),
			"closing_journal_id":   &closingJournalID,
			"account_count":        len(closingDeltaMap),
			"transaction_count":    int(transactionCount),
			"reopened_by":          nil,
			"reopened_at":          nil,
			"reopen_reason":        "",
			"reopening_journal_id": nil,
			"notes":                periodNotes,
			"updated_at":           now,
		})

		existingPeriod, err := findAccountingPeriodByCanonicalRange(tx, startDate, normalizeClosingDayStart(endDate), false)
		var accountingPeriodID uint
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			accountingPeriod := models.AccountingPeriod{
				StartDate:        startDate,
				EndDate:          endDate,
				Description:      description,
				IsClosed:         true,
				IsLocked:         false,
				ClosedBy:         &userIDUint,
				ClosedAt:         &now,
				TotalRevenue:     totalRevenue.InexactFloat64(),
				TotalExpense:     totalExpense.InexactFloat64(),
				NetIncome:        netIncome.InexactFloat64(),
				ClosingJournalID: &closingJournalID,
				AccountCount:     len(closingDeltaMap),
				TransactionCount: int(transactionCount),
				Notes:            periodNotes,
			}

			if err := tx.Create(&accountingPeriod).Error; err != nil {
				return fmt.Errorf("failed to create accounting period: %v", err)
			}
			accountingPeriodID = accountingPeriod.ID
		case err != nil:
			return fmt.Errorf("failed to query existing accounting period: %v", err)
		default:
			accountingPeriodValues["notes"] = appendPeriodNote(existingPeriod.Notes, periodNotes)
			accountingPeriodValues["start_date"] = startDate
			accountingPeriodValues["end_date"] = endDate
			if err := tx.Model(existingPeriod).Updates(accountingPeriodValues).Error; err != nil {
				return fmt.Errorf("failed to update accounting period: %v", err)
			}
			accountingPeriodID = existingPeriod.ID
		}

		if accountingPeriodID == 0 {
			return fmt.Errorf("failed to determine accounting period id for snapshot refresh")
		}
		if err := s.refreshPeriodSnapshot(ctx, tx, accountingPeriodID); err != nil {
			return err
		}

		log.Printf("[UNIFIED CLOSING] ✅ Period closing completed successfully")
		log.Printf("[UNIFIED CLOSING] Net Income: %.2f transferred to Retained Earnings", netIncome.InexactFloat64())

		return nil
	})
}

// PreviewPeriodClosing generates preview of period closing
// IMPORTANT: Logika perhitungan HARUS sama dengan ExecutePeriodClosing agar angka preview = angka real
func (s *UnifiedPeriodClosingService) PreviewPeriodClosing(ctx context.Context, startDate, endDate time.Time) (map[string]interface{}, error) {
	startDate = normalizeClosingDayStart(startDate)
	endDate = normalizeClosingDayEnd(endDate)

	if endDate.Before(startDate) {
		return nil, fmt.Errorf("end_date must be on or after start_date")
	}

	tx := s.db.WithContext(ctx)

	// 1. Retained earnings account
	var retainedEarnings models.Account
	if err := tx.Where("code = ? AND type = ?", "3102", models.AccountTypeEquity).First(&retainedEarnings).Error; err != nil {
		return nil, fmt.Errorf("retained earnings account (3102) not found: %v", err)
	}

	// 2. Info last closed period (untuk warning & auto-detection di UI)
	validationMessages, blockingMessages, _, err := s.validateClosingWindow(tx, startDate, endDate)
	if err != nil {
		return nil, err
	}

	var transactionCount int64
	var periodDays int

	// 3. Hitung jumlah transaksi operasional dalam periode untuk informasi tambahan
	if transactionCount, err = countOperationalJournalEntries(tx, startDate, endDate); err != nil {
		return nil, fmt.Errorf("failed to count transactions for preview: %v", err)
	}

	// +1 karena periode inklusif (start & end)
	periodDays = int(endDate.Sub(startDate).Hours()/24) + 1
	if periodDays < 0 {
		periodDays = 0
	}

	// 4. Ambil semua akun revenue & expense (bukan hanya yang non-zero)
	var revenueAccounts []models.Account
	// CRITICAL FIX: Removed is_header=false and added localized type support with ILIKE
	if err := tx.Where("TRIM(type) ILIKE 'REVENUE' OR TRIM(type) ILIKE 'PENDAPATAN%'").Find(&revenueAccounts).Error; err != nil {
		return nil, fmt.Errorf("failed to get revenue accounts: %v", err)
	}

	var expenseAccounts []models.Account
	// CRITICAL FIX: Removed is_header=false and added localized type support with ILIKE
	if err := tx.Where("TRIM(type) ILIKE 'EXPENSE' OR TRIM(type) ILIKE 'BEBAN%'").Find(&expenseAccounts).Error; err != nil {
		return nil, fmt.Errorf("failed to get expense accounts: %v", err)
	}

	// Build map untuk akses cepat account by ID
	revenueMap := make(map[uint64]models.Account)
	for _, acc := range revenueAccounts {
		revenueMap[uint64(acc.ID)] = acc
	}

	expenseMap := make(map[uint64]models.Account)
	for _, acc := range expenseAccounts {
		expenseMap[uint64(acc.ID)] = acc
	}

	// 5. Hitung saldo nyata dari SSOT (exact sama seperti di ExecutePeriodClosing)
	type AccountBalance struct {
		AccountID uint64
		AccType   string
		Net       float64 // Debit - Credit sampai endDate
	}

	revenueIDs := make([]uint64, 0, len(revenueAccounts))
	for _, acc := range revenueAccounts {
		revenueIDs = append(revenueIDs, uint64(acc.ID))
	}

	expenseIDs := make([]uint64, 0, len(expenseAccounts))
	for _, acc := range expenseAccounts {
		expenseIDs = append(expenseIDs, uint64(acc.ID))
	}

	allTempIDs := append(revenueIDs, expenseIDs...)

	var balances []AccountBalance
	if len(allTempIDs) > 0 {
		if err := tx.Raw(`
			SELECT 
				a.id AS account_id,
				a.type AS acc_type,
				COALESCE(SUM(ujl.debit_amount), 0) - COALESCE(SUM(ujl.credit_amount), 0) AS net
			FROM accounts a
			LEFT JOIN unified_journal_lines ujl ON ujl.account_id = a.id
			LEFT JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
			WHERE a.id IN ?
				AND a.deleted_at IS NULL
				AND (uje.id IS NULL OR (uje.status = 'POSTED' AND uje.deleted_at IS NULL AND uje.entry_date <= ?))
			GROUP BY a.id, a.type
		`, allTempIDs, endDate).Scan(&balances).Error; err != nil {
			return nil, fmt.Errorf("failed to calculate account balances for preview: %v", err)
		}
	}

	threshold := decimal.NewFromFloat(0.01)
	var totalRevenueDec, totalExpenseDec decimal.Decimal

	var revenuePreview []models.PeriodAccountBalance
	var expensePreview []models.PeriodAccountBalance

	for _, bal := range balances {
		currentBalance := decimal.NewFromFloat(bal.Net)
		if currentBalance.Abs().LessThan(threshold) {
			continue
		}

		// Delta closing = -saldo sekarang → nilai absolut = jumlah yang akan ditutup
		amountToClose := currentBalance.Neg().Abs()
		amountFloat := amountToClose.InexactFloat64()

		// Gunakan logic deteksi tipe yang robust
		accTypeUpper := strings.ToUpper(strings.TrimSpace(bal.AccType))

		if accTypeUpper == "REVENUE" || strings.HasPrefix(accTypeUpper, "PENDAPATAN") {
			acc, ok := revenueMap[bal.AccountID]
			if !ok {
				continue
			}
			totalRevenueDec = totalRevenueDec.Add(amountToClose)
			revenuePreview = append(revenuePreview, models.PeriodAccountBalance{
				ID:      acc.ID,
				Code:    acc.Code,
				Name:    acc.Name,
				Balance: amountFloat,
				Type:    acc.Type,
			})
		} else {
			// Assume Expense
			acc, ok := expenseMap[bal.AccountID]
			if !ok {
				continue
			}
			totalExpenseDec = totalExpenseDec.Add(amountToClose)
			expensePreview = append(expensePreview, models.PeriodAccountBalance{
				ID:      acc.ID,
				Code:    acc.Code,
				Name:    acc.Name,
				Balance: amountFloat,
				Type:    acc.Type,
			})
		}
	}

	canClose := len(blockingMessages) == 0 && (len(revenuePreview) > 0 || len(expensePreview) > 0)
	if !canClose {
		if len(blockingMessages) == 0 {
			validationMessages = append(validationMessages, "No revenue or expense balances to close.")
		}
	}

	totalRevenue := totalRevenueDec.InexactFloat64()
	totalExpense := totalExpenseDec.InexactFloat64()
	netIncome := totalRevenueDec.Sub(totalExpenseDec).InexactFloat64()

	// 6. Preview jurnal closing (high level)
	retainedName := fmt.Sprintf("%s - %s", retainedEarnings.Code, retainedEarnings.Name)
	closingEntries := []models.ClosingEntryPreview{}

	if totalRevenueDec.GreaterThan(decimal.Zero) {
		closingEntries = append(closingEntries, models.ClosingEntryPreview{
			Description:   "Close Revenue Accounts to Retained Earnings",
			DebitAccount:  "Revenue Accounts (Total)",
			CreditAccount: retainedName,
			Amount:        totalRevenue,
		})
	}
	if totalExpenseDec.GreaterThan(decimal.Zero) {
		closingEntries = append(closingEntries, models.ClosingEntryPreview{
			Description:   "Close Expense Accounts to Retained Earnings",
			DebitAccount:  retainedName,
			CreditAccount: "Expense Accounts (Total)",
			Amount:        totalExpense,
		})
	}

	// 7. Susun response map untuk kompatibilitas dengan frontend (SettingsPage)
	return map[string]interface{}{
		"start_date":           startDate.Format("2006-01-02"),
		"end_date":             endDate.Format("2006-01-02"),
		"total_revenue":        totalRevenue,
		"total_expense":        totalExpense,
		"net_income":           netIncome,
		"retained_earnings":    retainedName,
		"retained_earnings_id": retainedEarnings.ID,
		"revenue_accounts":     revenuePreview,
		"expense_accounts":     expensePreview,
		"closing_entries":      closingEntries,
		"can_close":            canClose,
		"has_blocking_issues":  len(blockingMessages) > 0,
		"blocking_messages":    blockingMessages,
		"validation_messages":  validationMessages,
		"transaction_count":    transactionCount,
		"period_days":          periodDays,
	}, nil
}

// GetLastClosingInfo returns information about the last closed period
// Digunakan frontend untuk:
//   - Menentukan start date otomatis (day-after-last-closing)
//   - Menentukan start date pertama kali (earliest transaction date)
func (s *UnifiedPeriodClosingService) GetLastClosingInfo(ctx context.Context) (map[string]interface{}, error) {
	tx := s.db.WithContext(ctx)

	var lastPeriod models.AccountingPeriod
	err := tx.Where("is_closed = ?", true).
		Order("end_date DESC").
		First(&lastPeriod).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Belum pernah closing sama sekali → cari tanggal transaksi pertama dari SSOT
			var count int64
			if err := tx.Model(&models.SSOTJournalEntry{}).
				Where("status = ? AND source_type NOT IN ?", models.SSOTStatusPosted, nonSystemJournalSourceTypes()).
				Count(&count).Error; err != nil {
				return nil, fmt.Errorf("failed to count transactions: %v", err)
			}

			resp := map[string]interface{}{
				"has_previous_closing": false,
			}

			if count > 0 {
				var earliestTxn time.Time
				if err := tx.Model(&models.SSOTJournalEntry{}).
					Where("status = ? AND source_type NOT IN ?", models.SSOTStatusPosted, nonSystemJournalSourceTypes()).
					Select("MIN(entry_date)").
					Scan(&earliestTxn).Error; err != nil {
					return nil, fmt.Errorf("failed to query earliest transaction date: %v", err)
				}
				resp["period_start_date"] = earliestTxn.Format("2006-01-02")
			}

			return resp, nil
		}
		return nil, fmt.Errorf("failed to query last closing period: %v", err)
	}

	nextStart := nextClosingStartDate(lastPeriod.EndDate)

	return map[string]interface{}{
		"has_previous_closing":   true,
		"last_closing_date":      periodBoundaryDateString(lastPeriod.EndDate, "end"),
		"next_start_date":        nextStart.Format("2006-01-02"),
		"last_net_income":        lastPeriod.NetIncome,
		"last_period_start_date": periodBoundaryDateString(lastPeriod.StartDate, "start"),
		"last_period_end_date":   periodBoundaryDateString(lastPeriod.EndDate, "end"),
	}, nil
}

// IsDateInClosedPeriod checks if a given date falls within a closed period
func (s *UnifiedPeriodClosingService) IsDateInClosedPeriod(ctx context.Context, date time.Time) (bool, error) {
	var periods []models.AccountingPeriod
	if err := s.db.WithContext(ctx).
		Where("is_closed = ? AND deleted_at IS NULL", true).
		Find(&periods).Error; err != nil {
		return false, fmt.Errorf("failed to check closed period: %v", err)
	}

	targetDate := normalizeClosingDayStart(date)
	for _, period := range periods {
		start := canonicalPeriodStartDate(period.StartDate)
		end := canonicalPeriodEndDate(period.EndDate)
		if !targetDate.Before(start) && !targetDate.After(end) {
			return true, nil
		}
	}

	return false, nil
}

// GetPeriodInfoForDate returns period information for a specific date
func (s *UnifiedPeriodClosingService) GetPeriodInfoForDate(ctx context.Context, date time.Time) map[string]interface{} {
	var periods []models.AccountingPeriod
	err := s.db.WithContext(ctx).
		Preload("ClosedByUser").
		Where("is_closed = ? AND deleted_at IS NULL", true).
		Order("end_date DESC").
		Find(&periods).Error
	if err != nil {
		return nil
	}

	targetDate := normalizeClosingDayStart(date)
	var period *models.AccountingPeriod
	for i := range periods {
		start := canonicalPeriodStartDate(periods[i].StartDate)
		end := canonicalPeriodEndDate(periods[i].EndDate)
		if !targetDate.Before(start) && !targetDate.After(end) {
			period = &periods[i]
			break
		}
	}
	if period == nil {
		return nil
	}

	info := map[string]interface{}{
		"start_date":  periodBoundaryDateString(period.StartDate, "start"),
		"end_date":    periodBoundaryDateString(period.EndDate, "end"),
		"description": period.Description,
		"is_locked":   period.IsLocked,
	}

	if period.ClosedBy != nil {
		info["closed_by"] = *period.ClosedBy
	}

	if period.ClosedByUser.ID != 0 {
		info["closed_by_name"] = period.ClosedByUser.GetDisplayName()
	}

	return info
}

// ReopenPeriod reopens a previously closed accounting period
// This will reverse the closing journal entry and mark the period as open
func (s *UnifiedPeriodClosingService) ReopenPeriod(ctx context.Context, startDate, endDate time.Time, reason string, userID uint64) error {
	startDate = normalizeClosingDayStart(startDate)
	endDate = normalizeClosingDayEnd(endDate)
	log.Printf("[UNIFIED CLOSING] Reopening period: %s to %s, reason: %s",
		startDate.Format("2006-01-02"), endDate.Format("2006-01-02"), reason)

	return s.db.Transaction(func(tx *gorm.DB) error {
		// 1. Find the matching closed period
		period, err := findAccountingPeriodByCanonicalRange(tx, startDate, normalizeClosingDayStart(endDate), true)

		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("tidak ditemukan periode tertutup dari %s sampai %s",
					startDate.Format("02 Jan 2006"), endDate.Format("02 Jan 2006"))
			}
			return fmt.Errorf("gagal mencari periode: %v", err)
		}

		// 2. Check if it's locked (hard lock — cannot be reopened)
		if period.IsLocked {
			return fmt.Errorf("periode %s sampai %s sedang hard locked dan tidak dapat dibuka kembali",
				periodBoundaryDateString(period.StartDate, "start"), periodBoundaryDateString(period.EndDate, "end"))
		}

		// 3. Ensure this is the most recent closed period.
		var closedPeriods []models.AccountingPeriod
		if err := tx.Where("is_closed = ? AND deleted_at IS NULL", true).Order("end_date DESC").Find(&closedPeriods).Error; err != nil {
			return fmt.Errorf("gagal memuat daftar periode tertutup: %v", err)
		}
		selectedEnd := canonicalPeriodEndDate(period.EndDate)
		for _, candidate := range closedPeriods {
			if candidate.ID == period.ID {
				continue
			}
			if canonicalPeriodEndDate(candidate.EndDate).After(selectedEnd) {
				return fmt.Errorf("tidak dapat membuka periode ini karena ada periode yang lebih baru sudah ditutup (%s - %s). Buka periode terbaru terlebih dahulu",
					periodBoundaryDateString(candidate.StartDate, "start"), periodBoundaryDateString(candidate.EndDate, "end"))
			}
		}

		// 4. Find and reverse the active closing journal entry
		var closingJournal *models.SSOTJournalEntry
		if period.ClosingJournalID != nil {
			closingJournal, err = findSingleActiveClosingJournal(
				tx,
				fmt.Sprintf("closing_journal_id=%d", *period.ClosingJournalID),
				tx.Where(
					"id = ? AND source_type = ? AND status = ? AND deleted_at IS NULL",
					uint64(*period.ClosingJournalID), models.SSOTSourceTypeClosing, models.SSOTStatusPosted,
				),
			)
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("gagal mencari jurnal closing via closing_journal_id: %v", err)
			}
		}

		if closingJournal == nil {
			closingSourceCode := buildClosingPeriodSourceCode(period.StartDate, period.EndDate)
			closingJournal, err = findSingleActiveClosingJournal(
				tx,
				fmt.Sprintf("source_code=%s", closingSourceCode),
				tx.Where(
					"source_type = ? AND source_code = ? AND status = ? AND deleted_at IS NULL",
					models.SSOTSourceTypeClosing, closingSourceCode, models.SSOTStatusPosted,
				),
			)
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("gagal mencari jurnal closing via source code: %v", err)
			}
		}

		if closingJournal == nil {
			closingJournal, err = findSingleActiveClosingJournal(
				tx,
				fmt.Sprintf("tanggal akhir %s", periodBoundaryDateString(period.EndDate, "end")),
				tx.Where(
					"source_type = ? AND entry_date >= ? AND entry_date <= ? AND status = ? AND deleted_at IS NULL",
					models.SSOTSourceTypeClosing,
					normalizeClosingDayStart(period.EndDate),
					normalizeClosingDayEnd(period.EndDate),
					models.SSOTStatusPosted,
				),
			)
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("gagal mencari jurnal closing via tanggal akhir periode: %v", err)
			}
		}

		if closingJournal == nil {
			return fmt.Errorf("gagal mencari jurnal closing aktif untuk periode %s sampai %s",
				periodBoundaryDateString(period.StartDate, "start"), periodBoundaryDateString(period.EndDate, "end"))
		}

		// Collect affected account IDs for balance recalculation
		affectedAccountIDs := make(map[uint64]struct{})
		var reopeningJournalID *uint
		reopenAt := normalizeClosingDayEnd(period.EndDate)

		// Get journal lines to know which accounts to recalculate
		var lines []models.SSOTJournalLine
		if err := tx.Where("journal_id = ?", closingJournal.ID).Order("line_number ASC").Find(&lines).Error; err != nil {
			return fmt.Errorf("gagal mengambil baris jurnal closing: %v", err)
		}
		for _, line := range lines {
			affectedAccountIDs[line.AccountID] = struct{}{}
		}

		reversalEntry, err := s.createClosingReversalEntry(tx, *closingJournal, lines, *period, reason, reopenAt, userID)
		if err != nil {
			return fmt.Errorf("gagal membuat jurnal pembalik closing: %v", err)
		}
		reversalID := uint(reversalEntry.ID)
		reopeningJournalID = &reversalID

		log.Printf("[UNIFIED CLOSING] Reversed closing journal ID: %d with reversal journal ID: %d", closingJournal.ID, reversalEntry.ID)

		// 5. Recalculate current balances for all affected accounts and header chains
		if len(affectedAccountIDs) > 0 {
			var accountIDs []uint64
			for id := range affectedAccountIDs {
				accountIDs = append(accountIDs, id)
			}

			if err := syncCurrentAccountBalances(tx, accountIDs); err != nil {
				return fmt.Errorf("gagal sinkron saldo akun current: %v", err)
			}
			if err := syncAllHeaderAccountBalances(tx); err != nil {
				return fmt.Errorf("gagal sinkron saldo akun header: %v", err)
			}
		}

		// 6. Mark period as open
		now := time.Now()
		reopenedBy := uint(userID)
		reopenNote := buildReopenedPeriodNote(now, userID, reason)
		if err := tx.Model(period).Updates(clearPeriodSnapshotFields(map[string]interface{}{
			"is_closed":            false,
			"is_locked":            false,
			"closing_journal_id":   nil,
			"reopened_by":          &reopenedBy,
			"reopened_at":          &now,
			"reopen_reason":        reason,
			"reopening_journal_id": reopeningJournalID,
			"reopen_count":         gorm.Expr("reopen_count + ?", 1),
			"notes":                appendPeriodNote(period.Notes, reopenNote),
			"updated_at":           now,
		})).Error; err != nil {
			return fmt.Errorf("gagal membuka periode: %v", err)
		}

		log.Printf("[UNIFIED CLOSING] ✅ Period reopened successfully: %s to %s",
			startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

		return nil
	})
}

// GetClosingHistory returns the history of all period closings
func (s *UnifiedPeriodClosingService) GetClosingHistory(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 20
	}

	var latestClosedPeriod models.AccountingPeriod
	latestClosedPeriodID := uint(0)
	err := s.db.WithContext(ctx).
		Where("deleted_at IS NULL AND is_closed = ?", true).
		Order("end_date DESC, id DESC").
		Limit(1).
		First(&latestClosedPeriod).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to determine latest closed period: %v", err)
	}
	if err == nil {
		latestClosedPeriodID = latestClosedPeriod.ID
	}

	var periods []models.AccountingPeriod
	err = s.db.WithContext(ctx).
		Where("deleted_at IS NULL AND (is_closed = ? OR reopen_count > 0 OR reopened_at IS NOT NULL)", true).
		Preload("ClosedByUser").
		Preload("ReopenedByUser").
		Order("end_date DESC").
		Limit(limit).
		Find(&periods).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get closing history: %v", err)
	}

	result := make([]map[string]interface{}, 0, len(periods))
	for _, p := range periods {
		noteLines := splitPeriodNoteLines(p.Notes)
		item := map[string]interface{}{
			"id":                p.ID,
			"start_date":        periodBoundaryDateString(p.StartDate, "start"),
			"end_date":          periodBoundaryDateString(p.EndDate, "end"),
			"description":       p.Description,
			"is_closed":         p.IsClosed,
			"status":            "CLOSED",
			"is_locked":         p.IsLocked,
			"total_revenue":     p.TotalRevenue,
			"total_expense":     p.TotalExpense,
			"net_income":        p.NetIncome,
			"period_type":       p.PeriodType,
			"account_count":     p.AccountCount,
			"transaction_count": p.TransactionCount,
			"reopen_count":      p.ReopenCount,
			"notes":             p.Notes,
			"note_lines":        noteLines,
			"history_events":    buildPeriodHistoryEvents(p),
			"can_reopen":        p.IsClosed && !p.IsLocked && p.ID == latestClosedPeriodID,
		}
		if !p.IsClosed {
			item["status"] = "REOPENED"
		}

		if p.ClosedAt != nil {
			item["closed_at"] = formatPeriodHistoryTimestamp(*p.ClosedAt)
		}

		if p.ClosedByUser != nil && p.ClosedByUser.ID != 0 {
			item["closed_by_name"] = p.ClosedByUser.GetDisplayName()
		}
		if p.ReopenedAt != nil {
			item["reopened_at"] = formatPeriodHistoryTimestamp(*p.ReopenedAt)
		}
		if p.ReopenReason != "" {
			item["reopen_reason"] = p.ReopenReason
		}
		if p.ReopenedByUser != nil && p.ReopenedByUser.ID != 0 {
			item["reopened_by_name"] = p.ReopenedByUser.GetDisplayName()
		}
		if p.ClosingJournalID != nil {
			item["closing_journal_id"] = *p.ClosingJournalID
		}
		if p.ReopeningJournalID != nil {
			item["reopening_journal_id"] = *p.ReopeningJournalID
		}
		if p.SnapshotGeneratedAt != nil {
			item["snapshot_generated_at"] = formatPeriodHistoryTimestamp(*p.SnapshotGeneratedAt)
		}

		result = append(result, item)
	}

	return result, nil
}
