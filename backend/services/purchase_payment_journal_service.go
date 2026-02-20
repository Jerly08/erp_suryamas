package services

import (
	"fmt"
	"log"
	"os"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"

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

// CreatePaymentJournalEntryWithTx creates payment journal entry using existing transaction (PaymentService integration)
// This is a wrapper method that uses the existing transaction from PaymentService
func (s *PurchasePaymentJournalService) CreatePaymentJournalEntryWithTx(
	tx *gorm.DB,
	payment *models.Payment,
	cashBankID uint,
	userID uint,
) error {
	// For now, we'll use the legacy approach with the provided transaction
	// TODO: In future, integrate with SSOT system properly
	fmt.Printf("📝 Creating purchase payment journal via legacy COA update...\n")

	return s.createLegacyPaymentJournalWithTx(tx, payment, cashBankID, userID)
}

// createLegacyPaymentJournalWithTx creates legacy journal entry using existing transaction
func (s *PurchasePaymentJournalService) createLegacyPaymentJournalWithTx(
	tx *gorm.DB,
	payment *models.Payment,
	cashBankID uint,
	userID uint,
) error {
	// Get Accounts Payable account (2101 - Utang Usaha)
	var apAccount models.Account
	if err := tx.Where("code = ?", "2101").First(&apAccount).Error; err != nil {
		// Fallback by name pattern
		if err := tx.Where("LOWER(name) LIKE ?", "%hutang%usaha%").First(&apAccount).Error; err != nil {
			return fmt.Errorf("accounts payable (Utang Usaha - 2101) not found: %v", err)
		}
	}

	// Resolve Cash/Bank GL account
	var cashBankAccountID uint
	if cashBankID > 0 {
		var cashBank models.CashBank
		if err := tx.Select("account_id").First(&cashBank, cashBankID).Error; err != nil {
			return fmt.Errorf("cash/bank account not found: %v", err)
		}
		if cashBank.AccountID == 0 {
			return fmt.Errorf("cash/bank %d has no linked GL account", cashBankID)
		}
		cashBankAccountID = cashBank.AccountID
	} else {
		// Fallback to default Kas account (1101)
		var kasAccount models.Account
		if err := tx.Where("code = ?", "1101").First(&kasAccount).Error; err != nil {
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
		return fmt.Errorf("failed to create payment journal entry: %v", err)
	}

	// ⚠️ DO NOT manually UPDATE accounts.balance here!
	// PostgreSQL trigger `trg_update_account_balance` on `journal_lines` table
	// automatically recalculates accounts.balance when journal lines are inserted
	// (which happens in tx.Create(journalEntry) above).
	// Manual UPDATE here would cause DOUBLE POSTING.
	log.Printf("✅ Journal entry created - accounts.balance will be updated automatically by DB trigger")

	fmt.Printf("✅ Legacy purchase payment journal created (balance auto-synced by DB trigger)\n")

	return nil
}
