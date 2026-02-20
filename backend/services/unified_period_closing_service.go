package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/utils"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

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

// ExecutePeriodClosing performs period closing using unified journal system
func (s *UnifiedPeriodClosingService) ExecutePeriodClosing(ctx context.Context, startDate, endDate time.Time, description string, userID uint64) error {
	log.Printf("[UNIFIED CLOSING] Starting period closing: %s to %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	return s.db.Transaction(func(tx *gorm.DB) error {
		// 1. Get retained earnings account
		var retainedEarnings models.Account
		if err := tx.Where("code = ? AND type = ?", "3102", "EQUITY").First(&retainedEarnings).Error; err != nil {
			return fmt.Errorf("retained earnings account (3102) not found: %v", err)
		}

		// 2. Get the last closed period (for logging & validation only)
		//    Saldo yang ditutup tetap dihitung dari SSOT sampai endDate, sehingga tidak tergantung startDate,
		//    dan tidak akan terjadi double-closing untuk periode sebelumnya.
		var lastClosedPeriod models.AccountingPeriod
		err := tx.Where("is_closed = ?", true).Order("end_date DESC").First(&lastClosedPeriod).Error
		var periodStartDate time.Time
		if err != nil {
			// No previous closed periods, use the start_date provided
			periodStartDate = startDate
			log.Printf("[UNIFIED CLOSING] No previous closed periods found, using provided start date: %s", startDate.Format("2006-01-02"))
		} else {
			// Start from the day after last closed period
			periodStartDate = lastClosedPeriod.EndDate.AddDate(0, 0, 1)
			log.Printf("[UNIFIED CLOSING] Last closed period end: %s, starting from: %s",
				lastClosedPeriod.EndDate.Format("2006-01-02"), periodStartDate.Format("2006-01-02"))

			// Validate that startDate matches periodStartDate
			if !startDate.Equal(periodStartDate) {
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
					AND (uje.id IS NULL OR (uje.status = 'POSTED' AND uje.entry_date <= ?))
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
			return nil
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

		// 8. Create unified journal entry
		closingEntry := &models.SSOTJournalEntry{
			SourceType:      "CLOSING",
			EntryDate:       endDate,
			Description:     description,
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

		// 7. Robust balance update: recalculate exact balances for affected accounts from SSOT lines
		// Rationale: DB triggers are disabled to avoid double-posting; manual per-line +/- is error-prone due to sign conventions.
		// We recalc the true balance for each affected account and set it explicitly.
		affectedIDsMap := make(map[uint64]struct{})
		for _, line := range journalLines {
			affectedIDsMap[line.AccountID] = struct{}{}
		}
		var affectedIDs []uint64
		for id := range affectedIDsMap {
			affectedIDs = append(affectedIDs, id)
		}

		if len(affectedIDs) > 0 {
			// Query correct balances from unified journals (POSTED only)
			// IMPORTANT: gunakan konvensi saldo yang sama dengan updateAccountBalance()
			// - ASSET / EXPENSE  : saldo = Debit - Credit  (debit menambah)
			// - LIABILITY/EQUITY/REVENUE : saldo = Credit - Debit (kredit menambah)
			type BalanceRow struct {
				AccountID uint64
				AccType   string
				Correct   float64
			}
			var rows []BalanceRow
			if err := tx.Raw(`
				SELECT a.id as account_id, a.type as acc_type,
					CASE
						WHEN a.type IN ('ASSET', 'EXPENSE') THEN
							COALESCE(SUM(ujl.debit_amount),0) - COALESCE(SUM(ujl.credit_amount),0)
						ELSE
							COALESCE(SUM(ujl.credit_amount),0) - COALESCE(SUM(ujl.debit_amount),0)
					END AS correct
				FROM accounts a
				LEFT JOIN unified_journal_lines ujl ON ujl.account_id = a.id
				LEFT JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
				WHERE a.id IN ? AND a.deleted_at IS NULL
					AND (uje.id IS NULL OR (uje.status = 'POSTED' AND uje.entry_date <= ?))
				GROUP BY a.id, a.type
			`, affectedIDs, endDate).Scan(&rows).Error; err != nil {
				return fmt.Errorf("failed to recalc balances for affected accounts: %v", err)
			}

			for _, r := range rows {
				if err := tx.Model(&models.Account{}).
					Where("id = ?", r.AccountID).
					Update("balance", r.Correct).Error; err != nil {
					return fmt.Errorf("failed to set recalculated balance for account %d: %v", r.AccountID, err)
				}
				log.Printf("[UNIFIED CLOSING] Recalculated balance set: account_id=%d type=%s new_balance=%.2f", r.AccountID, r.AccType, r.Correct)
			}

			// VALIDATION: Verify that all Revenue and Expense accounts are now ZERO
			var nonZeroCount int64
			if err := tx.Model(&models.Account{}).
				Where("type IN (?) AND ABS(balance) > 0.01", []string{"REVENUE", "EXPENSE"}).
				Count(&nonZeroCount).Error; err != nil {
				return fmt.Errorf("failed to validate zero balances: %v", err)
			}

			if nonZeroCount > 0 {
				// Log which accounts are not zero for debugging
				var problemAccounts []models.Account
				tx.Where("type IN (?) AND ABS(balance) > 0.01", []string{"REVENUE", "EXPENSE"}).
					Find(&problemAccounts)
				for _, acc := range problemAccounts {
					log.Printf("[UNIFIED CLOSING] ⚠️ WARNING: Account %s (%s) still has balance: %.2f",
						acc.Code, acc.Name, acc.Balance)
				}
				return fmt.Errorf("closing validation failed: %d revenue/expense accounts still have non-zero balance", nonZeroCount)
			}

			log.Printf("[UNIFIED CLOSING] ✅ Validation passed: All Revenue and Expense accounts are ZERO")
		}

		// 8. Create accounting period record
		userIDUint := uint(userID)
		accountingPeriod := models.AccountingPeriod{
			StartDate:    startDate,
			EndDate:      endDate,
			Description:  description,
			IsClosed:     true,
			IsLocked:     true,
			ClosedBy:     &userIDUint,
			ClosedAt:     &now,
			TotalRevenue: totalRevenue.InexactFloat64(),
			TotalExpense: totalExpense.InexactFloat64(),
			NetIncome:    netIncome.InexactFloat64(),
		}

		if err := tx.Create(&accountingPeriod).Error; err != nil {
			return fmt.Errorf("failed to create accounting period: %v", err)
		}

		log.Printf("[UNIFIED CLOSING] ✅ Period closing completed successfully")
		log.Printf("[UNIFIED CLOSING] Net Income: %.2f transferred to Retained Earnings", netIncome.InexactFloat64())

		return nil
	})
}

// PreviewPeriodClosing generates preview of period closing
// IMPORTANT: Logika perhitungan HARUS sama dengan ExecutePeriodClosing agar angka preview = angka real
func (s *UnifiedPeriodClosingService) PreviewPeriodClosing(ctx context.Context, startDate, endDate time.Time) (map[string]interface{}, error) {
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
	var validationMessages []string
	var transactionCount int64
	var periodDays int

	var lastPeriod models.AccountingPeriod
	if err := tx.Where("is_closed = ?", true).Order("end_date DESC").First(&lastPeriod).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			validationMessages = append(validationMessages, "First period closing - no previous closed period found.")
		} else {
			return nil, fmt.Errorf("failed to query last closed period: %v", err)
		}
	} else {
		// Expected start date = last end date + 1 hari
		expectedStart := lastPeriod.EndDate.AddDate(0, 0, 1)
		if !startDate.Equal(expectedStart) {
			validationMessages = append(validationMessages,
				fmt.Sprintf("Warning: expected start date %s based on last closing, but got %s",
					expectedStart.Format("2006-01-02"), startDate.Format("2006-01-02")))
		}

		if !endDate.After(lastPeriod.EndDate) {
			validationMessages = append(validationMessages, "End date must be after last closing date.")
		}
	}

	// 3. Hitung jumlah transaksi non-closing dalam periode untuk informasi tambahan
	if err := tx.Model(&models.SSOTJournalEntry{}).
		Where("entry_date BETWEEN ? AND ? AND status = ? AND source_type != ?",
			startDate, endDate, models.SSOTStatusPosted, models.SSOTSourceTypeClosing).
		Count(&transactionCount).Error; err != nil {
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
				AND (uje.id IS NULL OR (uje.status = 'POSTED' AND uje.entry_date <= ?))
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

	canClose := len(revenuePreview) > 0 || len(expensePreview) > 0
	if !canClose {
		validationMessages = append(validationMessages, "No revenue or expense balances to close.")
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
				Where("status = ? AND source_type != ?", models.SSOTStatusPosted, models.SSOTSourceTypeClosing).
				Count(&count).Error; err != nil {
				return nil, fmt.Errorf("failed to count transactions: %v", err)
			}

			resp := map[string]interface{}{
				"has_previous_closing": false,
			}

			if count > 0 {
				var earliestTxn time.Time
				if err := tx.Model(&models.SSOTJournalEntry{}).
					Where("status = ? AND source_type != ?", models.SSOTStatusPosted, models.SSOTSourceTypeClosing).
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

	nextStart := lastPeriod.EndDate.AddDate(0, 0, 1)

	return map[string]interface{}{
		"has_previous_closing": true,
		"last_closing_date":    lastPeriod.EndDate.Format("2006-01-02"),
		"next_start_date":      nextStart.Format("2006-01-02"),
		"last_net_income":      lastPeriod.NetIncome,
	}, nil
}

// IsDateInClosedPeriod checks if a given date falls within a closed period
func (s *UnifiedPeriodClosingService) IsDateInClosedPeriod(ctx context.Context, date time.Time) (bool, error) {
	var count int64
	err := s.db.Model(&models.AccountingPeriod{}).
		Where("is_closed = ? AND ? BETWEEN start_date AND end_date", true, date).
		Count(&count).Error

	if err != nil {
		return false, fmt.Errorf("failed to check closed period: %v", err)
	}

	return count > 0, nil
}

// GetPeriodInfoForDate returns period information for a specific date
func (s *UnifiedPeriodClosingService) GetPeriodInfoForDate(ctx context.Context, date time.Time) map[string]interface{} {
	var period models.AccountingPeriod
	err := s.db.Preload("ClosedByUser").Where("? BETWEEN start_date AND end_date AND is_closed = ?", date, true).
		First(&period).Error

	if err != nil {
		return nil
	}

	info := map[string]interface{}{
		"start_date":  period.StartDate,
		"end_date":    period.EndDate,
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
