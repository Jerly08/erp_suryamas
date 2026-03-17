package services

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// PurchasePaymentJournalService handles journal creation for purchase payments
// This service integrates with SSOT journal system and provides fallback to legacy system
type PurchasePaymentJournalService struct {
	db                    *gorm.DB
	accountRepo           repositories.AccountRepository
	unifiedJournalService *UnifiedJournalService
	purchaseService       *PurchaseService
}

// NewPurchasePaymentJournalService creates a new purchase payment journal service
func NewPurchasePaymentJournalService(
	db *gorm.DB,
	accountRepo repositories.AccountRepository,
	unifiedJournalService *UnifiedJournalService,
	purchaseService *PurchaseService,
) *PurchasePaymentJournalService {
	return &PurchasePaymentJournalService{
		db:                    db,
		accountRepo:           accountRepo,
		unifiedJournalService: unifiedJournalService,
		purchaseService:       purchaseService,
	}
}

// CreatePaymentJournalEntry creates journal entries for purchase payment
// This uses SSOT system with legacy fallback for maximum reliability
func (s *PurchasePaymentJournalService) CreatePaymentJournalEntry(
	payment *models.Payment,
	purchase *models.Purchase,
	cashBankID uint,
	userID uint,
) error {
	// Try SSOT journal creation first if enabled
	if os.Getenv("ENABLE_SSOT_PAYMENT_JOURNALS") == "true" {
		if err := s.createSSOTPaymentJournal(payment, purchase, cashBankID, userID); err != nil {
			fmt.Printf("⚠️ SSOT payment journal creation failed: %v\n", err)
			fmt.Printf("💡 Falling back to legacy journal creation\n")
		} else {
			fmt.Printf("✅ SSOT payment journal created successfully\n")

			// If SSOT succeeds, also create legacy journal for immediate balance update if enabled
			if os.Getenv("ENABLE_LEGACY_PAYMENT_JOURNALS") == "true" {
				if err := s.createLegacyPaymentJournal(payment, cashBankID, userID); err != nil {
					fmt.Printf("⚠️ Legacy journal creation failed (non-critical): %v\n", err)
				} else {
					fmt.Printf("✅ Legacy payment journal created as backup\n")
				}
			}
			return nil
		}
	}

	// Fallback to legacy journal creation
	if os.Getenv("ENABLE_LEGACY_PAYMENT_JOURNALS") == "true" {
		if err := s.createLegacyPaymentJournal(payment, cashBankID, userID); err != nil {
			return fmt.Errorf("both SSOT and legacy journal creation failed: %v", err)
		}
		fmt.Printf("✅ Legacy payment journal created successfully\n")
		return nil
	}

	return fmt.Errorf("no payment journal creation method is enabled")
}

// createSSOTPaymentJournal creates SSOT journal entry for purchase payment
func (s *PurchasePaymentJournalService) createSSOTPaymentJournal(
	payment *models.Payment,
	purchase *models.Purchase,
	cashBankID uint,
	userID uint,
) error {
	if s.unifiedJournalService == nil {
		return fmt.Errorf("unified journal service not available")
	}

	if s.purchaseService == nil {
		return fmt.Errorf("purchase service not available")
	}

	// Use the existing SSOT adapter from purchase service
	err := s.purchaseService.CreatePurchasePaymentJournal(
		purchase.ID,
		payment.Amount,
		cashBankID,
		fmt.Sprintf("PAY-%s", purchase.Code),
		fmt.Sprintf("Payment for Purchase %s", purchase.Code),
		userID,
	)

	if err != nil {
		return fmt.Errorf("failed to create SSOT purchase payment journal: %v", err)
	}

	return nil
}

// createLegacyPaymentJournal creates legacy journal entry for immediate COA balance update
func (s *PurchasePaymentJournalService) createLegacyPaymentJournal(
	payment *models.Payment,
	cashBankID uint,
	userID uint,
) error {
	// Start transaction for legacy journal creation
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get Accounts Payable account (2101 - Utang Usaha)
	var apAccount models.Account
	if err := tx.Where("code = ?", "2101").First(&apAccount).Error; err != nil {
		// Fallback by name pattern
		if err := tx.Where("LOWER(name) LIKE ?", "%hutang%usaha%").First(&apAccount).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("accounts payable (Utang Usaha - 2101) not found: %v", err)
		}
	}

	// Resolve Cash/Bank GL account
	var cashBankAccountID uint
	if cashBankID > 0 {
		var cashBank models.CashBank
		if err := tx.Select("account_id").First(&cashBank, cashBankID).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("cash/bank account not found: %v", err)
		}
		if cashBank.AccountID == 0 {
			tx.Rollback()
			return fmt.Errorf("cash/bank %d has no linked GL account", cashBankID)
		}
		cashBankAccountID = cashBank.AccountID
	} else {
		// Fallback to default Kas account (1101)
		var kasAccount models.Account
		if err := tx.Where("code = ?", "1101").First(&kasAccount).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("default cash account (1101) not found: %v", err)
		}
		cashBankAccountID = kasAccount.ID
	}

	// Create posted journal entry
	journalEntry := &models.JournalEntry{
		EntryDate:       payment.Date,
		Description:     fmt.Sprintf("Purchase Payment %s", payment.Code),
		ReferenceType:   models.JournalRefPayment,
		ReferenceID:     &payment.ID,
		Reference:       payment.Code,
		UserID:          userID,
		Status:          models.JournalStatusPosted,
		TotalDebit:      payment.Amount,
		TotalCredit:     payment.Amount,
		IsAutoGenerated: true,
	}

	// Create journal lines
	journalLines := []models.JournalLine{
		{
			AccountID:    apAccount.ID,
			Description:  fmt.Sprintf("Reduce Utang Usaha - %s", payment.Code),
			DebitAmount:  payment.Amount,
			CreditAmount: 0,
			LineNumber:   1,
		},
		{
			AccountID:    cashBankAccountID,
			Description:  fmt.Sprintf("Cash/Bank payment - %s", payment.Code),
			DebitAmount:  0,
			CreditAmount: payment.Amount,
			LineNumber:   2,
		},
	}

	journalEntry.JournalLines = journalLines

	// Create journal entry
	if err := tx.Create(journalEntry).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create payment journal entry: %v", err)
	}

	// NOTE: Do NOT update accounts.balance here!
	// The SSOT system (AutoPost=true) handles balance updates.
	// Manual updates here would cause double-posting.

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit payment journal transaction: %v", err)
	}

	fmt.Printf("✅ Legacy payment journal created (balance updates handled by SSOT AutoPost)\n")

	return nil
}

// GetPaymentJournalEntries retrieves journal entries for a payment
func (s *PurchasePaymentJournalService) GetPaymentJournalEntries(paymentID uint) ([]models.JournalEntry, error) {
	var journalEntries []models.JournalEntry
	err := s.db.Where("reference_type = ? AND reference_id = ?", models.JournalRefPayment, paymentID).Find(&journalEntries).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get payment journal entries: %v", err)
	}
	return journalEntries, nil
}

// ValidatePaymentJournal validates that journal entries are balanced and correct
func (s *PurchasePaymentJournalService) ValidatePaymentJournal(paymentID uint) error {
	journalEntries, err := s.GetPaymentJournalEntries(paymentID)
	if err != nil {
		return err
	}

	if len(journalEntries) == 0 {
		return fmt.Errorf("no journal entries found for payment %d", paymentID)
	}

	for _, entry := range journalEntries {
		if entry.TotalDebit != entry.TotalCredit {
			return fmt.Errorf("journal entry %s is not balanced: debit=%.2f, credit=%.2f",
				entry.Code, entry.TotalDebit, entry.TotalCredit)
		}

		if entry.Status != models.JournalStatusPosted {
			return fmt.Errorf("journal entry %s is not posted: status=%s", entry.Code, entry.Status)
		}
	}

	return nil
}

// CreatePaymentJournalEntryWithTx creates SSOT payment journal entry using existing transaction
// 🔥 CRITICAL FIX: Now writes to unified_journal_ledger (SSOT) instead of legacy journal_entries
// This ensures payment entries are visible to the Balance Sheet which reads from SSOT tables
func (s *PurchasePaymentJournalService) CreatePaymentJournalEntryWithTx(
	tx *gorm.DB,
	payment *models.Payment,
	cashBankID uint,
	userID uint,
) error {
	// ✅ GUARD: Skip PPN/tax payments — they already have their own journal
	// created by tax_payment_service.go (Debit PPN Keluaran, Credit PPN Masukan, Credit Bank)
	// Creating a PURCHASE_PAYMENT journal for them would incorrectly debit UTANG USAHA
	if payment.IsTaxPayment() {
		log.Printf("⚠️ [SSOT] Skipping purchase payment journal for Payment #%d (PaymentType: %s) — tax payments have their own journal", payment.ID, payment.PaymentType)
		return nil
	}

	// ✅ GUARD: Skip EXPENSE payments — they already have their own journal
	// (Debit Beban, Credit Bank) created by CreateExpensePayment
	if payment.PaymentType == models.PaymentTypeExpense {
		log.Printf("⚠️ [SSOT] Skipping purchase payment journal for Payment #%d (PaymentType: EXPENSE) — expense payments have their own journal", payment.ID)
		return nil
	}

	// ✅ GUARD: Skip SALES/RECEIVABLE payments — these already have SALES_PAYMENT journals
	// created by SalesJournalServiceV2.CreateSalesPaymentJournal (Debit Bank, Credit Piutang)
	// Creating a PURCHASE_PAYMENT journal would incorrectly debit UTANG USAHA and credit Bank
	paymentID := uint64(payment.ID)
	var salesJournalCount int64
	if err := tx.Model(&models.SSOTJournalEntry{}).
		Where("source_type = ? AND source_id = ?", "SALES_PAYMENT", paymentID).
		Count(&salesJournalCount).Error; err == nil && salesJournalCount > 0 {
		log.Printf("⚠️ [SSOT] Skipping purchase payment journal for Payment #%d — already has %d SALES_PAYMENT journal(s)", payment.ID, salesJournalCount)
		return nil
	}

	log.Printf("📝 [SSOT] Creating purchase payment SSOT journal for Payment #%d (Amount: %.2f)...", payment.ID, payment.Amount)

	// Check if SSOT journal already exists for this payment (purchase type)
	var existingCount int64
	if err := tx.Model(&models.SSOTJournalEntry{}).
		Where("source_type = ? AND source_id = ?", "PURCHASE_PAYMENT", paymentID).
		Count(&existingCount).Error; err == nil && existingCount > 0 {
		log.Printf("⚠️ [SSOT] SSOT journal already exists for Payment #%d (found %d entries), skipping",
			payment.ID, existingCount)
		return nil
	}

	// 1. Resolve Accounts Payable account (2101-001 Hutang Usaha - Lokal)
	var apAccount models.Account
	if err := tx.Where("code = ?", "2101-001").First(&apAccount).Error; err != nil {
		// Fallback to parent 2101
		if err := tx.Where("code = ?", "2101").First(&apAccount).Error; err != nil {
			// Fallback by name pattern
			if err := tx.Where("LOWER(name) LIKE ?", "%hutang%usaha%").First(&apAccount).Error; err != nil {
				return fmt.Errorf("accounts payable (Utang Usaha) not found: %v", err)
			}
		}
	}

	// 2. Resolve Cash/Bank GL account from CashBank record
	var bankAccount models.Account
	if cashBankID > 0 {
		var cashBank models.CashBank
		if err := tx.First(&cashBank, cashBankID).Error; err != nil {
			return fmt.Errorf("cash/bank record %d not found: %v", cashBankID, err)
		}
		if cashBank.AccountID == 0 {
			return fmt.Errorf("cash/bank %d (%s) has no linked GL account", cashBankID, cashBank.Name)
		}
		if err := tx.First(&bankAccount, cashBank.AccountID).Error; err != nil {
			return fmt.Errorf("GL account %d linked to cash/bank %d not found: %v", cashBank.AccountID, cashBankID, err)
		}
		log.Printf("🏦 [SSOT] Using bank GL account %s (%s) for Payment #%d", bankAccount.Code, bankAccount.Name, payment.ID)
	} else {
		// Fallback to default Kas account
		if err := tx.Where("code = ?", "1101-002").First(&bankAccount).Error; err != nil {
			if err := tx.Where("code = ?", "1101").First(&bankAccount).Error; err != nil {
				return fmt.Errorf("default cash account not found: %v", err)
			}
		}
		log.Printf("🏦 [SSOT] Using default cash account %s (%s) for Payment #%d", bankAccount.Code, bankAccount.Name, payment.ID)
	}

	// 3. Build journal lines: DR Hutang Usaha / CR Bank
	payableSettlementValue := payment.Amount
	bankBookAmountValue := payment.BookAmountIDR
	if bankBookAmountValue == 0 {
		bankBookAmountValue = payment.Amount
	}
	forexGainLossValue := bankBookAmountValue - payableSettlementValue
	payableSettlement := decimal.NewFromFloat(payableSettlementValue)
	bankBookAmount := decimal.NewFromFloat(bankBookAmountValue)

	// 4. Create SSOT journal entry (unified_journal_ledger)
	now := time.Now()
	postedBy := uint64(userID)

	journalEntry := &models.SSOTJournalEntry{
		EntryNumber:     fmt.Sprintf("PURCHASE-PAY-%d-%d", payment.ID, now.Unix()),
		SourceType:      "PURCHASE_PAYMENT",
		SourceID:        &paymentID,
		SourceCode:      payment.Code,
		EntryDate:       payment.Date,
		Description:     fmt.Sprintf("Purchase Payment %s", payment.Code),
		Reference:       payment.Code,
		TotalDebit:      decimal.Zero,
		TotalCredit:     decimal.Zero,
		Status:          "DRAFT",
		IsBalanced:      true,
		IsAutoGenerated: true,
		CreatedBy:       uint64(userID),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := tx.Create(journalEntry).Error; err != nil {
		return fmt.Errorf("failed to create SSOT payment journal entry: %v", err)
	}

	// 5. Create journal lines
	lines := []struct {
		AccountID    uint64
		DebitAmount  decimal.Decimal
		CreditAmount decimal.Decimal
		Description  string
	}{
		{
			AccountID:    uint64(apAccount.ID),
			DebitAmount:  payableSettlement,
			CreditAmount: decimal.Zero,
			Description:  fmt.Sprintf("Pelunasan Hutang Usaha - %s", payment.Code),
		},
		{
			AccountID:    uint64(bankAccount.ID),
			DebitAmount:  decimal.Zero,
			CreditAmount: bankBookAmount,
			Description:  fmt.Sprintf("Pembayaran via %s - %s", bankAccount.Name, payment.Code),
		},
	}

	if forexGainLossValue > 0.005 {
		lossAccountID, err := s.resolveForexAccountID(tx, false)
		if err != nil {
			return fmt.Errorf("failed to resolve forex loss account: %v", err)
		}
		lines = append(lines, struct {
			AccountID    uint64
			DebitAmount  decimal.Decimal
			CreditAmount decimal.Decimal
			Description  string
		}{
			AccountID:    lossAccountID,
			DebitAmount:  decimal.NewFromFloat(forexGainLossValue),
			CreditAmount: decimal.Zero,
			Description:  fmt.Sprintf("Rugi selisih kurs terealisasi - %s", payment.Code),
		})
	} else if forexGainLossValue < -0.005 {
		gainAccountID, err := s.resolveForexAccountID(tx, true)
		if err != nil {
			return fmt.Errorf("failed to resolve forex gain account: %v", err)
		}
		lines = append(lines, struct {
			AccountID    uint64
			DebitAmount  decimal.Decimal
			CreditAmount decimal.Decimal
			Description  string
		}{
			AccountID:    gainAccountID,
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.NewFromFloat(-forexGainLossValue),
			Description:  fmt.Sprintf("Laba selisih kurs terealisasi - %s", payment.Code),
		})
	}

	var actualTotalDebit decimal.Decimal
	var actualTotalCredit decimal.Decimal
	for i, lineReq := range lines {
		journalLine := &models.SSOTJournalLine{
			JournalID:    journalEntry.ID,
			AccountID:    lineReq.AccountID,
			LineNumber:   i + 1,
			Description:  lineReq.Description,
			DebitAmount:  lineReq.DebitAmount,
			CreditAmount: lineReq.CreditAmount,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		actualTotalDebit = actualTotalDebit.Add(lineReq.DebitAmount)
		actualTotalCredit = actualTotalCredit.Add(lineReq.CreditAmount)

		if err := tx.Create(journalLine).Error; err != nil {
			return fmt.Errorf("failed to create SSOT payment journal line: %v", err)
		}

		// Update account.balance for COA tree view
		if err := s.updateAccountBalanceForPayment(tx, lineReq.AccountID, lineReq.DebitAmount, lineReq.CreditAmount); err != nil {
			log.Printf("⚠️ Warning: Failed to update account balance for account %d: %v", lineReq.AccountID, err)
		}
	}

	if err := tx.Model(journalEntry).Updates(map[string]interface{}{
		"total_debit":  actualTotalDebit,
		"total_credit": actualTotalCredit,
		"is_balanced":  actualTotalDebit.Equal(actualTotalCredit),
	}).Error; err != nil {
		return fmt.Errorf("failed to update payment journal totals: %v", err)
	}

	// 6. Post the journal entry
	if err := tx.Model(journalEntry).Updates(map[string]interface{}{
		"status":    "POSTED",
		"posted_at": &now,
		"posted_by": &postedBy,
	}).Error; err != nil {
		return fmt.Errorf("failed to post payment journal entry: %v", err)
	}

	log.Printf("✅ [SSOT] Created and posted purchase payment journal entry #%d (DR %s %.2f / CR %s %.2f)",
		journalEntry.ID, apAccount.Code, payableSettlement.InexactFloat64(), bankAccount.Code, bankBookAmount.InexactFloat64())

	return nil
}

func (s *PurchasePaymentJournalService) resolveForexAccountID(tx *gorm.DB, isGain bool) (uint64, error) {
	type candidate struct {
		Code        string
		NamePattern string
	}

	var candidates []candidate
	if isGain {
		candidates = []candidate{
			{Code: "7103"},
			{Code: "7102"},
			{NamePattern: "%laba%selisih%kurs%"},
			{NamePattern: "%pendapatan%lain%"},
		}
	} else {
		candidates = []candidate{
			{Code: "8103"},
			{Code: "8102"},
			{NamePattern: "%rugi%selisih%kurs%"},
			{NamePattern: "%beban%lain%"},
		}
	}

	for _, c := range candidates {
		var account models.Account
		query := tx.Model(&models.Account{})
		if c.Code != "" {
			query = query.Where("code = ?", c.Code)
		} else {
			query = query.Where("LOWER(name) LIKE ?", strings.ToLower(c.NamePattern))
		}
		if err := query.First(&account).Error; err == nil {
			return uint64(account.ID), nil
		}
	}

	if isGain {
		return 0, fmt.Errorf("forex gain account not found")
	}
	return 0, fmt.Errorf("forex loss account not found")
}

// updateAccountBalanceForPayment updates account.balance field for COA tree view display
func (s *PurchasePaymentJournalService) updateAccountBalanceForPayment(db *gorm.DB, accountID uint64, debitAmount, creditAmount decimal.Decimal) error {
	var account models.Account
	if err := db.First(&account, accountID).Error; err != nil {
		return fmt.Errorf("account %d not found: %v", accountID, err)
	}

	debit := debitAmount.InexactFloat64()
	credit := creditAmount.InexactFloat64()
	netChange := debit - credit

	switch strings.ToUpper(account.Type) {
	case "ASSET", "EXPENSE":
		account.Balance += netChange
	case "LIABILITY", "EQUITY", "REVENUE":
		account.Balance -= netChange
	}

	if err := db.Model(&account).Select("Balance").Updates(models.Account{Balance: account.Balance}).Error; err != nil {
		return fmt.Errorf("failed to save account balance: %v", err)
	}

	log.Printf("💰 [SSOT] Updated account %s (%s) balance: Dr=%.2f, Cr=%.2f, Change=%.2f, New Balance=%.2f",
		account.Code, account.Name, debit, credit, netChange, account.Balance)

	return nil
}

// MigrateExistingPaymentsToSSOT retroactively creates SSOT journal entries for all
// existing payments that currently only have legacy journal entries.
// This is an idempotent migration that runs at startup.
func (s *PurchasePaymentJournalService) MigrateExistingPaymentsToSSOT() (int, int, error) {
	log.Printf("🔄 [MIGRATION] Starting migration of existing payments to SSOT journal system...")

	// Step 0: Clean up duplicate payment journals from previous buggy migrations
	// Old code used source_id=purchaseID instead of paymentID, causing the
	// idempotency check to miss existing entries and create duplicates.
	s.cleanupDuplicatePaymentJournals()

	// Step 1: Fix existing SSOT payment entries that reference header accounts
	// (e.g. entries that debit 2101 instead of 2101-001)
	s.fixHeaderAccountReferences()

	// Step 2: Find all completed payments that DON'T have ANY SSOT journal entries
	var payments []models.Payment
	if err := s.db.Where("status = ? AND deleted_at IS NULL", models.PaymentStatusCompleted).Find(&payments).Error; err != nil {
		return 0, 0, fmt.Errorf("failed to query payments: %v", err)
	}

	log.Printf("🔄 [MIGRATION] Found %d completed payments to check", len(payments))

	migrated := 0
	skipped := 0

	for _, payment := range payments {
		// ✅ GUARD: Skip PPN/tax payments — they have their own dedicated journal
		if payment.IsTaxPayment() {
			skipped++
			continue
		}

		// ✅ GUARD: Skip EXPENSE payments — they already have their own journal
		// (Debit Beban, Credit Bank) created by CreateExpensePayment
		// Creating a PURCHASE_PAYMENT journal would incorrectly debit UTANG USAHA
		if payment.PaymentType == models.PaymentTypeExpense {
			skipped++
			continue
		}

		// ✅ GUARD: Skip SALES/RECEIVABLE payments — they already have SALES_PAYMENT journals
		// created by SalesJournalServiceV2.CreateSalesPaymentJournal (Debit Bank, Credit Piutang)
		// Creating a PURCHASE_PAYMENT journal would incorrectly debit UTANG USAHA and credit Bank
		paymentID := uint64(payment.ID)
		var salesJournalCount int64
		if err := s.db.Model(&models.SSOTJournalEntry{}).
			Where("source_type = ? AND source_id = ? AND deleted_at IS NULL", "SALES_PAYMENT", paymentID).
			Count(&salesJournalCount).Error; err == nil && salesJournalCount > 0 {
			log.Printf("⚠️ [MIGRATION] Skipping Payment #%d (%s) — already has %d SALES_PAYMENT journal(s)", payment.ID, payment.Code, salesJournalCount)
			skipped++
			continue
		}

		// Check if SSOT journal already exists (check ALL possible source types and patterns)
		var existingCount int64
		if err := s.db.Model(&models.SSOTJournalEntry{}).
			Where("source_id = ? AND source_type IN ('PAYMENT', 'PURCHASE_PAYMENT') AND deleted_at IS NULL", paymentID).
			Count(&existingCount).Error; err == nil && existingCount > 0 {
			skipped++
			continue
		}

		// Also check by description pattern (catches old entries with wrong source_id)
		if err := s.db.Model(&models.SSOTJournalEntry{}).
			Where("source_type IN ('PAYMENT', 'PURCHASE_PAYMENT') AND deleted_at IS NULL AND description LIKE ?",
				fmt.Sprintf("%%Payment %s%%", payment.Code)).
			Count(&existingCount).Error; err == nil && existingCount > 0 {
			skipped++
			continue
		}

		// Resolve CashBankID from cash_bank_transactions
		var cashBankTx models.CashBankTransaction
		var cashBankID uint
		if err := s.db.Where("reference_type = ? AND reference_id = ?", "PAYMENT", payment.ID).
			First(&cashBankTx).Error; err == nil {
			cashBankID = cashBankTx.CashBankID
		}

		// Create SSOT journal in a transaction
		tx := s.db.Begin()
		if err := s.CreatePaymentJournalEntryWithTx(tx, &payment, cashBankID, uint(payment.UserID)); err != nil {
			tx.Rollback()
			log.Printf("❌ [MIGRATION] Failed to migrate payment %d (%s): %v", payment.ID, payment.Code, err)
			continue
		}

		if err := tx.Commit().Error; err != nil {
			log.Printf("❌ [MIGRATION] Failed to commit migration for payment %d: %v", payment.ID, err)
			continue
		}

		migrated++
		log.Printf("✅ [MIGRATION] Migrated payment %d (%s) to SSOT", payment.ID, payment.Code)
	}

	log.Printf("🔄 [MIGRATION] Migration complete: %d migrated, %d skipped (already in SSOT)", migrated, skipped)
	return migrated, skipped, nil
}

// cleanupDuplicatePaymentJournals removes orphaned and duplicate SSOT payment journal entries.
// This fixes a bug where old code used source_id=purchaseID instead of paymentID,
// causing the migration to re-create entries for payments that already had SSOT journals.
//
// Strategy: for each PAYMENT-type journal, verify that its source_id matches a real
// payment with the SAME amount. If not, it's an orphan from old buggy code — soft-delete it.
func (s *PurchasePaymentJournalService) cleanupDuplicatePaymentJournals() {
	log.Printf("🧹 [CLEANUP] Checking for orphaned/duplicate payment journal entries...")

	// === Phase 0: Clean up phantom PURCHASE_PAYMENT entries created for sales/PPN payments ===
	// These were incorrectly created by the migration for payments that have SALES_PAYMENT journals
	log.Printf("🧹 [CLEANUP] Phase 0: Removing phantom PURCHASE_PAYMENT entries for sales/PPN payments...")
	var purchasePaymentEntries []struct {
		ID         uint64 `gorm:"column:id"`
		SourceID   uint64 `gorm:"column:source_id"`
		SourceCode string `gorm:"column:source_code"`
	}
	if err := s.db.Raw(`SELECT id, source_id, source_code FROM unified_journal_ledger 
		WHERE source_type = 'PURCHASE_PAYMENT' AND deleted_at IS NULL`).Scan(&purchasePaymentEntries).Error; err == nil {
		phantomDeleted := 0
		for _, entry := range purchasePaymentEntries {
			// Check if this payment also has a SALES_PAYMENT journal
			var salesCount int64
			s.db.Raw(`SELECT COUNT(*) FROM unified_journal_ledger 
				WHERE source_type = 'SALES_PAYMENT' AND source_id = ? AND deleted_at IS NULL`, entry.SourceID).Scan(&salesCount)

			if salesCount > 0 {
				// This is a phantom! Soft-delete it
				s.db.Exec("UPDATE unified_journal_lines SET deleted_at = CURRENT_TIMESTAMP WHERE journal_entry_id = ? AND deleted_at IS NULL", entry.ID)
				s.db.Exec("UPDATE unified_journal_ledger SET deleted_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL", entry.ID)
				log.Printf("🗑️ [CLEANUP] Removed phantom PURCHASE_PAYMENT entry ID=%d (Code: %s) — payment has SALES_PAYMENT journal", entry.ID, entry.SourceCode)
				phantomDeleted++
			}

			// Also check for SETOR-PPN codes (tax payment processed as purchase payment)
			if strings.HasPrefix(entry.SourceCode, "SETOR-PPN") {
				var taxCount int64
				s.db.Raw(`SELECT COUNT(*) FROM unified_journal_ledger
					WHERE source_type = 'PAYMENT' AND source_code = ? AND deleted_at IS NULL`, entry.SourceCode).Scan(&taxCount)
				if taxCount > 0 {
					s.db.Exec("UPDATE unified_journal_lines SET deleted_at = CURRENT_TIMESTAMP WHERE journal_entry_id = ? AND deleted_at IS NULL", entry.ID)
					s.db.Exec("UPDATE unified_journal_ledger SET deleted_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL", entry.ID)
					log.Printf("🗑️ [CLEANUP] Removed phantom PURCHASE_PAYMENT entry ID=%d (Code: %s) — PPN tax payment", entry.ID, entry.SourceCode)
					phantomDeleted++
				}
			}
		}
		if phantomDeleted > 0 {
			log.Printf("🧹 [CLEANUP] Phase 0 complete: Removed %d phantom PURCHASE_PAYMENT entries", phantomDeleted)
		} else {
			log.Printf("✅ [CLEANUP] Phase 0 complete: No phantom entries found")
		}
	}

	// Get all real payments and build a map of paymentID → amount
	var payments []models.Payment
	if err := s.db.Where("status = ? AND deleted_at IS NULL", models.PaymentStatusCompleted).Find(&payments).Error; err != nil {
		log.Printf("⚠️ [CLEANUP] Failed to query payments: %v", err)
		return
	}

	paymentAmountMap := make(map[uint64]int64) // paymentID → amount in cents
	for _, p := range payments {
		pid := uint64(p.ID)
		paymentAmountMap[pid] = int64(p.Amount * 100) // convert to cents for precise comparison
	}

	// Find ALL active PAYMENT-type journal entries
	var allJournals []models.SSOTJournalEntry
	if err := s.db.Where(
		"source_type IN ('PAYMENT', 'PURCHASE_PAYMENT') AND status = 'POSTED' AND deleted_at IS NULL",
	).Order("id ASC").Find(&allJournals).Error; err != nil {
		log.Printf("⚠️ [CLEANUP] Failed to query payment journals: %v", err)
		return
	}

	log.Printf("🧹 [CLEANUP] Validating %d payment journals against %d real payments", len(allJournals), len(payments))

	totalDeleted := 0

	for _, j := range allJournals {
		journalAmountCents := j.TotalDebit.Mul(decimal.NewFromInt(100)).IntPart()

		// Check if this journal's source_id matches a real payment with the same amount
		if j.SourceID != nil {
			if expectedAmount, exists := paymentAmountMap[*j.SourceID]; exists {
				if expectedAmount == journalAmountCents {
					// This journal correctly references a valid payment with matching amount — keep it
					continue
				}
			}
		}

		// This journal has no valid source_id → amount match — it's an orphan
		// Soft-delete it and its lines
		now := time.Now()
		s.db.Model(&models.SSOTJournalLine{}).
			Where("journal_id = ? AND deleted_at IS NULL", j.ID).
			Update("deleted_at", now)
		s.db.Model(&models.SSOTJournalEntry{}).
			Where("id = ? AND deleted_at IS NULL", j.ID).
			Update("deleted_at", now)
		totalDeleted++

		sourceID := "nil"
		if j.SourceID != nil {
			sourceID = fmt.Sprintf("%d", *j.SourceID)
		}
		log.Printf("🧹 [CLEANUP] Soft-deleted orphan journal #%d (source_id=%s, amount=%s) — no matching payment",
			j.ID, sourceID, j.TotalDebit.String())
	}

	if totalDeleted > 0 {
		log.Printf("🧹 [CLEANUP] Removed %d orphaned payment journal entries", totalDeleted)
	} else {
		log.Printf("✅ [CLEANUP] No orphaned payment journals found")
	}
}

// fixHeaderAccountReferences fixes existing SSOT payment journal lines that incorrectly
// reference header accounts (e.g. 2101) instead of child accounts (e.g. 2101-001).
// Header accounts are excluded from the balance sheet query, so entries referencing
// headers are invisible to the report.
func (s *PurchasePaymentJournalService) fixHeaderAccountReferences() {
	// Find AP child account (2101-001)
	var apChild models.Account
	if err := s.db.Where("code = ? AND COALESCE(is_header, false) = false", "2101-001").First(&apChild).Error; err != nil {
		return // No child found, nothing to fix
	}

	// Find AP header account (2101)
	var apHeader models.Account
	if err := s.db.Where("code = ? AND COALESCE(is_header, false) = true", "2101").First(&apHeader).Error; err != nil {
		return // No header found, nothing to fix
	}

	if apHeader.ID == apChild.ID {
		return // Same account, nothing to fix
	}

	// Fix journal lines that reference the header instead of the child
	result := s.db.Model(&models.SSOTJournalLine{}).
		Where("account_id = ? AND journal_id IN (SELECT id FROM unified_journal_ledger WHERE source_type IN ('PAYMENT', 'PURCHASE_PAYMENT') AND status = 'POSTED' AND deleted_at IS NULL)", apHeader.ID).
		Update("account_id", apChild.ID)

	if result.Error == nil && result.RowsAffected > 0 {
		log.Printf("🔧 [MIGRATION] Fixed %d payment journal lines: 2101(header) → 2101-001(child)", result.RowsAffected)
	}
}
