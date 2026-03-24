package services

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// TaxPaymentService handles tax-related payment operations (PPN Input/Output payments)
type TaxPaymentService struct {
	db                *gorm.DB
	taxAccountService *TaxAccountService
	taxAccountHelper  *TaxAccountHelper
	journalService    *UnifiedJournalService
}

// NewTaxPaymentService creates a new TaxPaymentService instance
func NewTaxPaymentService(db *gorm.DB) *TaxPaymentService {
	return &TaxPaymentService{
		db:                db,
		taxAccountService: NewTaxAccountService(db),
		taxAccountHelper:  NewTaxAccountHelper(db),
		journalService:    NewUnifiedJournalService(db),
	}
}

// CreatePPNPaymentRequest represents a request to remit PPN (Setor PPN)
type CreatePPNPaymentRequest struct {
	PPNType     string    `json:"ppn_type" binding:"required"` // REMIT (Setor PPN) - deprecated but kept for compatibility
	Amount      float64   `json:"amount"`                      // Amount to remit (optional, will be calculated if 0)
	Date        time.Time `json:"date" binding:"required"`
	CashBankID  uint      `json:"cash_bank_id" binding:"required"`
	Reference   string    `json:"reference"`
	Notes       string    `json:"notes"`
	PeriodMonth int       `json:"period_month"` // Tax period month
	PeriodYear  int       `json:"period_year"`  // Tax period year
}

// CreatePPNPayment creates PPN remittance (Setor PPN) with proper PSAK-compliant double-entry
// Logic per PSAK:
// - Calculate: PPN Terutang = PPN Keluaran - PPN Masukan
// - If positive (Kurang Bayar): Debit PPN Keluaran, Credit PPN Masukan, Credit Cash/Bank
// - If negative (Lebih Bayar): No payment needed, create adjustment entry
func (s *TaxPaymentService) CreatePPNPayment(req CreatePPNPaymentRequest, userID uint) (*models.Payment, error) {
	log.Printf("🏦 Starting PPN Remittance (Setor PPN): Amount=%.2f", req.Amount)

	// Note: ppn_type is deprecated but kept for backward compatibility
	// Modern implementation auto-calculates from balances

	// Amount validation removed - will be calculated from PPN balances if not provided

	// Validate date
	if req.Date.IsZero() {
		return nil, fmt.Errorf("payment date is required")
	}

	// Begin transaction
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %v", tx.Error)
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Printf("❌ PANIC in CreatePPNPayment: %v", r)
		}
	}()

	// Get Cash/Bank account
	var cashBank models.CashBank
	if err := tx.Preload("Account").First(&cashBank, req.CashBankID).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("cash/bank account not found: %v", err)
	}
	log.Printf("📋 Cash/Bank Account: %s (ID: %d)", cashBank.Account.Name, cashBank.AccountID)

	ppnMasukanAccount, err := s.taxAccountHelper.GetPurchaseInputVATAccount(tx)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("PPN Masukan account not found: %v", err)
	}

	ppnKeluaranAccount, err := s.taxAccountHelper.GetSalesOutputVATAccount(tx)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("PPN Keluaran account not found: %v", err)
	}

	ppnMasukanSignedBalance, err := s.resolveTaxAccountBalance(tx, ppnMasukanAccount)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to resolve PPN Masukan balance: %v", err)
	}

	log.Printf("📋 PPN Masukan: %s - %s (Balance: %.2f)", ppnMasukanAccount.Code, ppnMasukanAccount.Name, ppnMasukanAccount.Balance)
	log.Printf("📋 PPN Keluaran: %s - %s (Balance: %.2f)", ppnKeluaranAccount.Code, ppnKeluaranAccount.Name, ppnKeluaranAccount.Balance)

	ppnKeluaranSignedBalance, err := s.resolveTaxAccountBalance(tx, ppnKeluaranAccount)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to resolve PPN Keluaran balance: %v", err)
	}

	log.Printf("INFO [TaxPaymentService] Resolved PPN balances => Masukan: %s, Keluaran: %s", ppnMasukanSignedBalance.StringFixed(2), ppnKeluaranSignedBalance.StringFixed(2))

	// Calculate PPN Terutang (PPN yang harus dibayar) using decimal for precision
	// PPN Terutang = PPN Keluaran - PPN Masukan
	//
	// ⚠️ IMPORTANT: PPN Masukan can be configured either as a standard asset account
	// or an older contra-liability account in legacy data. We use Abs() so the
	// remittance calculation follows the configured account without depending on
	// legacy account-type sign conventions.
	//
	// ⚠️ CRITICAL: All amounts use decimal.Decimal to avoid floating-point rounding
	// errors that cause unbalanced journal entries (e.g., 0.04 difference).
	ppnMasukanDec := ppnMasukanSignedBalance.Abs()
	ppnKeluaranDec := ppnKeluaranSignedBalance.Abs()
	ppnTerutangDec := ppnKeluaranDec.Sub(ppnMasukanDec)

	log.Printf("📊 Calculated PPN Terutang: %s (Keluaran: %s [raw: %.2f] - Masukan: %s [raw: %.2f])",
		ppnTerutangDec.StringFixed(2), ppnKeluaranDec.StringFixed(2), ppnKeluaranSignedBalance.InexactFloat64(), ppnMasukanDec.StringFixed(2), ppnMasukanSignedBalance.InexactFloat64())

	// Validate PPN Terutang
	if ppnTerutangDec.LessThanOrEqual(decimal.Zero) {
		tx.Rollback()
		return nil, fmt.Errorf("tidak ada PPN yang harus dibayar. PPN Terutang: %s (PPN Masukan lebih besar dari PPN Keluaran)", ppnTerutangDec.StringFixed(2))
	}

	// Payment amount = exact PPN Terutang (no rounding to ensure journal balance)
	// Cash/Bank credit = PPN Keluaran debit - PPN Masukan credit (always balanced)
	paymentAmountDec := ppnTerutangDec
	paymentAmount := paymentAmountDec.InexactFloat64()
	log.Printf("✅ Full PPN Terutang payment: %s", paymentAmountDec.StringFixed(2))

	// Validate cash/bank balance
	if decimal.NewFromFloat(cashBank.Balance).LessThan(paymentAmountDec) {
		tx.Rollback()
		return nil, fmt.Errorf("insufficient cash/bank balance. Available: %.2f, Required: %s", cashBank.Balance, paymentAmountDec.StringFixed(2))
	}

	// Generate payment code for PPN remittance
	code := s.generatePaymentCode("SETOR-PPN", tx)

	// Use default system contact (Tax Office)
	contactID := uint(1)

	// Create payment record
	payment := &models.Payment{
		Code:        code,
		ContactID:   contactID,
		UserID:      userID,
		Date:        req.Date,
		Amount:      paymentAmount,
		Method:      models.PaymentMethodBankTransfer, // Default untuk setor PPN
		Reference:   req.Reference,
		Status:      models.PaymentStatusCompleted, // PPN remittance langsung completed
		Notes:       fmt.Sprintf("Setor PPN - Terutang: %s. %s", ppnTerutangDec.StringFixed(2), req.Notes),
		PaymentType: models.PaymentTypeTaxPPN,
	}
	snapshot, snapshotErr := loadCashBankSnapshot(tx, req.CashBankID)
	if snapshotErr != nil {
		tx.Rollback()
		return nil, snapshotErr
	}
	applyCashBankSnapshotToPayment(payment, snapshot)

	if err := tx.Create(payment).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create payment record: %v", err)
	}
	log.Printf("✅ Payment record created: ID=%d, Code=%s", payment.ID, payment.Code)

	// Create journal entry per PSAK (FULL payment only - no proportional kompensasi)
	// Debit:  PPN Keluaran (full balance - zero out liability)
	// Credit: PPN Masukan (full balance - zero out kompensasi)
	// Credit: Cash/Bank (pembayaran neto = PPN Keluaran - PPN Masukan, always balanced)
	//
	// Example: PPN Keluaran 50jt, PPN Masukan 30jt, Terutang 20jt
	// Debit:  PPN Keluaran 50jt (zero out)
	// Credit: PPN Masukan 30jt (zero out)
	// Credit: Cash/Bank 20jt (bayar neto = 50jt - 30jt)
	ppnKeluaranDebitDec := ppnKeluaranDec                         // Full PPN Keluaran
	ppnMasukanCreditDec := ppnMasukanDec                          // Full PPN Masukan
	cashCreditDec := ppnKeluaranDebitDec.Sub(ppnMasukanCreditDec) // Exact balancing amount

	log.Printf("📋 Journal Entry breakdown: PPN Keluaran Debit=%s, PPN Masukan Credit=%s, Cash Credit=%s",
		ppnKeluaranDebitDec.StringFixed(2), ppnMasukanCreditDec.StringFixed(2), cashCreditDec.StringFixed(2))

	// Build journal lines (only include non-zero amounts)
	journalLines := []JournalLineRequest{}

	// Debit PPN Keluaran (liability berkurang) - always include if > 0
	if ppnKeluaranDebitDec.GreaterThan(decimal.Zero) {
		journalLines = append(journalLines, JournalLineRequest{
			AccountID:    uint64(ppnKeluaranAccount.ID),
			Description:  fmt.Sprintf("Setor PPN - %s", payment.Code),
			DebitAmount:  ppnKeluaranDebitDec,
			CreditAmount: decimal.Zero,
		})
	}

	// Credit PPN Masukan (asset berkurang - kompensasi) - only if > 0
	if ppnMasukanCreditDec.GreaterThan(decimal.Zero) {
		journalLines = append(journalLines, JournalLineRequest{
			AccountID:    uint64(ppnMasukanAccount.ID),
			Description:  fmt.Sprintf("Setor PPN - Kompensasi - %s", payment.Code),
			DebitAmount:  decimal.Zero,
			CreditAmount: ppnMasukanCreditDec,
		})
	}

	// Credit Cash/Bank (pembayaran neto) - exact balancing amount
	journalLines = append(journalLines, JournalLineRequest{
		AccountID:    uint64(cashBank.AccountID),
		Description:  fmt.Sprintf("Setor PPN - Pembayaran Neto - %s", payment.Code),
		DebitAmount:  decimal.Zero,
		CreditAmount: cashCreditDec,
	})

	// Create SSOT journal entry
	journalRequest := &JournalEntryRequest{
		SourceType:  models.SSOTSourceTypePayment,
		SourceID:    uint64(payment.ID),
		Reference:   payment.Code,
		EntryDate:   payment.Date,
		Description: fmt.Sprintf("Setor PPN ke Negara - %s (Terutang: %s)", payment.Code, ppnTerutangDec.StringFixed(2)),
		Lines:       journalLines,
		AutoPost:    true,
		CreatedBy:   uint64(userID),
	}

	journalService := NewUnifiedJournalService(tx)
	journalResponse, err := journalService.CreateJournalEntryWithTx(tx, journalRequest)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create journal entry: %v", err)
	}

	log.Printf("✅ Journal entry created: ID=%d, EntryNumber=%s", journalResponse.ID, journalResponse.EntryNumber)

	// NOTE: Account balances (PPN Masukan, PPN Keluaran) are automatically
	// recalculated by the DB trigger `sync_account_balance_from_ssot` whenever
	// journal lines are inserted. No manual balance correction needed.

	// Update payment with journal reference
	journalEntryID := uint(journalResponse.ID)
	payment.JournalEntryID = &journalEntryID
	if err := tx.Save(payment).Error; err != nil {
		log.Printf("⚠️ Warning: Failed to update payment with journal reference: %v", err)
	}

	guard := NewCashBankCurrencyGuard(s.db)

	// Update CashBank balance (must sync with Account balance updated by journal)
	oldCashBankBalance := cashBank.Balance
	cashBank.Balance -= paymentAmount
	if err := tx.Save(&cashBank).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update cash/bank balance: %v", err)
	}
	log.Printf("🏦 CashBank balance updated: %.2f -> %.2f (-%.2f)", oldCashBankBalance, cashBank.Balance, paymentAmount)

	// Create CashBankTransaction record for audit trail
	cashBankTx := &models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   "PAYMENT",
		ReferenceID:     payment.ID,
		Amount:          -paymentAmount, // Negative = outflow
		BalanceAfter:    cashBank.Balance,
		TransactionDate: payment.Date,
		Notes:           fmt.Sprintf("Setor PPN ke Negara - %s", payment.Code),
	}
	if err := tx.Create(cashBankTx).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create cash/bank transaction: %v", err)
	}
	log.Printf("✅ CashBankTransaction recorded: Amount=%.2f, BalanceAfter=%.2f", -paymentAmount, cashBank.Balance)

	if err := guard.EnforceAfterTransaction(tx, cashBank.ID); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to enforce cash/bank currency invariant after tax payment: %v", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	log.Printf("✅ PPN Remittance (Setor PPN) successfully created: %s (Amount: %.2f, PPN Terutang: %s)", payment.Code, paymentAmount, ppnTerutangDec.StringFixed(2))

	// Load relations for response
	if err := s.db.Preload("Contact").Preload("User").First(payment, payment.ID).Error; err != nil {
		log.Printf("⚠️ Warning: Failed to load payment relations: %v", err)
	}

	return payment, nil
}

// generatePaymentCode generates unique payment code with prefix
func (s *TaxPaymentService) generatePaymentCode(prefix string, tx *gorm.DB) string {
	now := time.Now()
	yearMonth := now.Format("0601") // YYMM format

	// Get last sequence for this month
	var lastPayment models.Payment
	pattern := fmt.Sprintf("%s-%s-%%", prefix, yearMonth)

	err := tx.Where("code LIKE ?", pattern).Order("code DESC").First(&lastPayment).Error

	sequence := 1
	if err == nil && lastPayment.Code != "" {
		// Extract sequence from last code more reliably
		// Expected format: PREFIX-YYMM-NNNN
		// Example: SETOR-PPN-2511-0001
		parts := strings.Split(lastPayment.Code, "-")
		if len(parts) >= 3 {
			// Last part should be the sequence number
			lastPart := parts[len(parts)-1]
			if lastSeq, err := strconv.Atoi(lastPart); err == nil {
				sequence = lastSeq + 1
			}
		}
	}

	// Format: PREFIX-YYMM-NNNN (max 30 chars)
	// Example: SETOR-PPN-2511-0001 (20 chars for SETOR-PPN)
	return fmt.Sprintf("%s-%s-%04d", prefix, yearMonth, sequence)
}

// GetPPNPaymentsByType retrieves PPN payments filtered by type
func (s *TaxPaymentService) GetPPNPaymentsByType(paymentType string, startDate, endDate time.Time) ([]models.Payment, error) {
	var payments []models.Payment

	query := s.db.Preload("Contact").Preload("User").Preload("TaxAccount").Preload("CashBank").
		Where("payment_type = ?", paymentType)

	if !startDate.IsZero() {
		query = query.Where("date >= ?", startDate)
	}

	if !endDate.IsZero() {
		query = query.Where("date <= ?", endDate)
	}

	if err := query.Order("date DESC, created_at DESC").Find(&payments).Error; err != nil {
		return nil, fmt.Errorf("failed to get PPN payments: %v", err)
	}

	return payments, nil
}

// GetPPNPaymentSummary returns summary of PPN payments
func (s *TaxPaymentService) GetPPNPaymentSummary(startDate, endDate time.Time) (map[string]interface{}, error) {
	type SummaryResult struct {
		PaymentType string
		TotalAmount float64
		Count       int64
	}

	var results []SummaryResult

	query := s.db.Model(&models.Payment{}).
		Select("payment_type, SUM(amount) as total_amount, COUNT(*) as count").
		Where("payment_type IN ?", []string{models.PaymentTypeTaxPPN, models.PaymentTypeTaxPPNInput, models.PaymentTypeTaxPPNOutput}).
		Where("status <> ?", models.PaymentStatusFailed)

	if !startDate.IsZero() {
		query = query.Where("date >= ?", startDate)
	}

	if !endDate.IsZero() {
		query = query.Where("date <= ?", endDate)
	}

	if err := query.Group("payment_type").Find(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get PPN payment summary: %v", err)
	}

	summary := map[string]interface{}{
		"ppn_masukan": map[string]interface{}{
			"total_amount": 0.0,
			"count":        0,
		},
		"ppn_keluaran": map[string]interface{}{
			"total_amount": 0.0,
			"count":        0,
		},
	}

	for _, result := range results {
		if result.PaymentType == models.PaymentTypeTaxPPNInput {
			summary["ppn_masukan"] = map[string]interface{}{
				"total_amount": result.TotalAmount,
				"count":        result.Count,
			}
		} else if result.PaymentType == models.PaymentTypeTaxPPNOutput {
			summary["ppn_keluaran"] = map[string]interface{}{
				"total_amount": result.TotalAmount,
				"count":        result.Count,
			}
		}
	}

	return summary, nil
}

func (s *TaxPaymentService) resolveTaxAccountBalance(db *gorm.DB, account *models.Account) (decimal.Decimal, error) {
	if account == nil {
		return decimal.Zero, fmt.Errorf("tax account is nil")
	}

	dbToUse := s.db
	if db != nil {
		dbToUse = db
	}

	balanceExpr := "COALESCE(SUM(ujl.debit_amount - ujl.credit_amount), 0)"
	switch strings.ToUpper(strings.TrimSpace(account.Type)) {
	case models.AccountTypeLiability, models.AccountTypeEquity, models.AccountTypeRevenue:
		balanceExpr = "COALESCE(SUM(ujl.credit_amount - ujl.debit_amount), 0)"
	}

	type ssotBalanceRow struct {
		LineCount int64
		Balance   float64
	}

	var row ssotBalanceRow
	if err := dbToUse.Table("unified_journal_lines AS ujl").
		Joins("JOIN unified_journal_ledger AS uje ON uje.id = ujl.journal_id").
		Where("ujl.account_id = ? AND uje.status = ? AND uje.deleted_at IS NULL", account.ID, models.SSOTStatusPosted).
		Select(fmt.Sprintf("COUNT(ujl.id) AS line_count, %s AS balance", balanceExpr)).
		Scan(&row).Error; err != nil {
		return decimal.Zero, fmt.Errorf("failed to calculate SSOT balance for account %s: %w", account.Code, err)
	}

	if row.LineCount > 0 {
		ssotBalance := decimal.NewFromFloat(row.Balance)
		if ssotBalance.Sub(decimal.NewFromFloat(account.Balance)).Abs().GreaterThan(decimal.NewFromFloat(0.01)) {
			log.Printf("WARNING [TaxPaymentService] Using SSOT balance for account %s (%s): %s instead of stored balance %.2f",
				account.Code, account.Name, ssotBalance.StringFixed(2), account.Balance)
		}
		return ssotBalance, nil
	}

	return decimal.NewFromFloat(account.Balance), nil
}

// GetPPNBalance retrieves current balance of PPN account (Masukan or Keluaran)
func (s *TaxPaymentService) GetPPNBalance(ppnType string) (float64, error) {
	// Validate PPN type
	if ppnType != "INPUT" && ppnType != "OUTPUT" {
		return 0, fmt.Errorf("ppn_type must be INPUT or OUTPUT")
	}

	var (
		account *models.Account
		err     error
	)
	if ppnType == "INPUT" {
		account, err = s.taxAccountHelper.GetPurchaseInputVATAccount(nil)
	} else {
		account, err = s.taxAccountHelper.GetSalesOutputVATAccount(nil)
	}
	if err != nil {
		return 0, err
	}

	// ⚠️ IMPORTANT: Always return absolute value of balance.
	// PPN Masukan can be asset-based in the current configuration or negative in
	// older contra-liability setups. Consumers expect the recoverable/owed amount
	// as a positive number.
	resolvedBalance, err := s.resolveTaxAccountBalance(nil, account)
	if err != nil {
		return 0, err
	}

	absBalance := resolvedBalance.Abs().InexactFloat64()
	log.Printf("📊 PPN %s Balance: %.2f (raw: %.2f, Account: %s - %s, Type: %s)",
		ppnType, absBalance, resolvedBalance.InexactFloat64(), account.Code, account.Name, account.Type)

	return absBalance, nil
}
