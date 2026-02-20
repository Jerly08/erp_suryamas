package services

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
)

// SalesJournalServiceV2 handles all sales-related journal entries with proper status validation
type SalesJournalServiceV2 struct {
	db          *gorm.DB
	journalRepo repositories.JournalEntryRepository
	coaService  *COAService
}

// NewSalesJournalServiceV2 creates a new instance of SalesJournalServiceV2
func NewSalesJournalServiceV2(
	db *gorm.DB,
	journalRepo repositories.JournalEntryRepository,
	coaService *COAService,
) *SalesJournalServiceV2 {
	return &SalesJournalServiceV2{
		db:          db,
		journalRepo: journalRepo,
		coaService:  coaService,
	}
}

// ShouldPostToJournal checks if a status should create journal entries
// KRITERIA POSTING JURNAL - HANYA STATUS INVOICED ATAU PAID
func (s *SalesJournalServiceV2) ShouldPostToJournal(status string) bool {
	// DRAFT dan CONFIRMED TIDAK BOLEH POSTING
	allowedStatuses := []string{"INVOICED", "PAID"}
	for _, allowed := range allowedStatuses {
		if status == allowed {
			return true
		}
	}
	return false
}

// CreateSalesJournal creates journal entries for a sale based on status
func (s *SalesJournalServiceV2) CreateSalesJournal(sale *models.Sale, tx *gorm.DB) error {
	// VALIDASI STATUS - HANYA INVOICED/PAID YANG BOLEH POSTING
	if !s.ShouldPostToJournal(sale.Status) {
		log.Printf("⚠️ Skipping journal creation for Sale #%d with status: %s (only INVOICED/PAID allowed)", sale.ID, sale.Status)
		return nil // Return nil karena ini bukan error, tapi by design
	}

	log.Printf("📝 Creating journal entries for Sale #%d (Status: %s, Payment Method: %s)",
		sale.ID, sale.Status, sale.PaymentMethodType)

	// Tentukan database yang akan digunakan
	dbToUse := s.db
	if tx != nil {
		dbToUse = tx
	}

	// ✅ CRITICAL FIX: Check if journal already exists for this sale
	// Prevent duplicate journal entries which cause double posting to COA
	var existingCount int64
	if err := dbToUse.Model(&models.SimpleSSOTJournal{}).
		Where("transaction_type = ? AND transaction_id = ?", "SALES", sale.ID).
		Count(&existingCount).Error; err == nil && existingCount > 0 {
		log.Printf("⚠️ Journal already exists for Sale #%d (found %d entries), skipping creation to prevent duplicate",
			sale.ID, existingCount)
		return nil // Don't create duplicate journal
	}

	// Create Simple SSOT Journal Entry
	ssotEntry := &models.SimpleSSOTJournal{
		EntryNumber:       fmt.Sprintf("SALES-%d", sale.ID),
		TransactionType:   "SALES",
		TransactionID:     sale.ID,
		TransactionNumber: sale.InvoiceNumber,
		Date:              sale.Date,
		Description:       fmt.Sprintf("Sales Invoice #%s - %s", sale.InvoiceNumber, sale.Customer.Name),
		TotalAmount:       sale.TotalAmount,
		Status:            "POSTED",
		CreatedBy:         sale.UserID,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	// Create SSOT Journal Entry
	if err := dbToUse.Create(ssotEntry).Error; err != nil {
		return fmt.Errorf("failed to create SSOT journal: %v", err)
	}

	// Helper to resolve account by code
	resolveByCode := func(code string) (*models.Account, error) {
		var acc models.Account
		if err := dbToUse.Where("code = ?", code).First(&acc).Error; err != nil {
			return nil, fmt.Errorf("account code %s not found: %v", code, err)
		}
		return &acc, nil
	}

	// Prepare journal items
	var journalItems []models.SimpleSSOTJournalItem

	// Determine DEBIT side based on payment method
	switch strings.ToUpper(strings.TrimSpace(sale.PaymentMethodType)) {
	case "CASH":
		// CASH → Debit Kas (1101)
		if acc, err := resolveByCode("1101"); err == nil {
			journalItems = append(journalItems, models.SimpleSSOTJournalItem{
				JournalID:   ssotEntry.ID,
				AccountID:   acc.ID,
				AccountCode: acc.Code,
				AccountName: acc.Name,
				Debit:       sale.TotalAmount,
				Credit:      0,
				Description: fmt.Sprintf("Cash payment for Invoice #%s", sale.InvoiceNumber),
			})
			log.Printf("💵 CASH Payment: Debit to %s (%s) = %.2f", acc.Name, acc.Code, sale.TotalAmount)
		} else {
			log.Printf("⚠️ Missing cash account 1101: %v", err)
		}
	case "BANK":
		// BANK → Debit ke Bank (specific cash_bank account if available, otherwise 1101-003 BANK GIRO)
		var acc *models.Account
		if sale.CashBankID != nil && *sale.CashBankID > 0 {
			var cashBank models.CashBank
			if err := dbToUse.First(&cashBank, *sale.CashBankID).Error; err == nil {
				// Load account for code/name
				var tmp models.Account
				if e2 := dbToUse.First(&tmp, cashBank.AccountID).Error; e2 == nil {
					acc = &tmp
				}
			}
		}
		if acc == nil {
			// Fallback: 1101-003 = BANK GIRO (under KAS DAN SETARA KAS)
			if resolved, err := resolveByCode("1101-003"); err == nil {
				acc = resolved
			} else {
				log.Printf("⚠️ Default bank account 1101-003 not found: %v", err)
			}
		}
		if acc != nil {
			journalItems = append(journalItems, models.SimpleSSOTJournalItem{
				JournalID:   ssotEntry.ID,
				AccountID:   acc.ID,
				AccountCode: acc.Code,
				AccountName: acc.Name,
				Debit:       sale.TotalAmount,
				Credit:      0,
				Description: fmt.Sprintf("Bank payment for Invoice #%s", sale.InvoiceNumber),
			})
			log.Printf("🏦 BANK Payment: Debit to %s (%s) = %.2f", acc.Name, acc.Code, sale.TotalAmount)
		}
	default:
		// CREDIT or unknown → Debit Piutang Usaha (1201)
		if acc, err := resolveByCode("1201"); err == nil {
			journalItems = append(journalItems, models.SimpleSSOTJournalItem{
				JournalID:   ssotEntry.ID,
				AccountID:   acc.ID,
				AccountCode: acc.Code,
				AccountName: acc.Name,
				Debit:       sale.TotalAmount,
				Credit:      0,
				Description: fmt.Sprintf("Account receivable for Invoice #%s", sale.InvoiceNumber),
			})
			log.Printf("📋 CREDIT Payment: Debit to %s (%s) = %.2f", acc.Name, acc.Code, sale.TotalAmount)
		} else {
			log.Printf("⚠️ Missing receivable account 1201: %v", err)
		}
	}

	// ✅ SAFETY CHECK: Ensure we have at least one debit line
	if len(journalItems) == 0 {
		log.Printf("❌ ERROR: No journal items created for Sale #%d, cannot proceed", sale.ID)
		return fmt.Errorf("no journal items created for sale #%d - check payment method configuration", sale.ID)
	}

	// CREDIT SIDE - Revenue dan PPN
	// ✅ FIX: Revenue per akun berdasarkan RevenueAccountID di SaleItem
	if len(sale.SaleItems) == 0 {
		// Safety: kalau SaleItems belum ter-load, coba preload dari DB
		var saleWithItems models.Sale
		if err := dbToUse.Preload("SaleItems").First(&saleWithItems, sale.ID).Error; err == nil {
			// gunakan slice dari hasil preload
			sale.SaleItems = saleWithItems.SaleItems
		} else {
			log.Printf("⚠️ Failed to preload SaleItems for Sale #%d: %v", sale.ID, err)
		}
	}

	// Kelompokkan subtotal per akun pendapatan
	revenueTotals := make(map[uint]float64)
	var defaultRevenueAcc *models.Account

	for _, item := range sale.SaleItems {
		amount := item.LineTotal
		if amount == 0 {
			continue
		}

		accID := item.RevenueAccountID
		if accID == 0 {
			// ✅ FIX: Fallback ke LEAF account 4101-001 (bukan header 4101)
			// Posting ke header menyebabkan balance di-overwrite oleh RecalculateAllParentBalances()
			if defaultRevenueAcc == nil {
				acc, err := resolveByCode("4101-001")
				if err != nil {
					// Fallback ke header jika leaf tidak ada
					acc, err = resolveByCode("4101")
					if err != nil {
						log.Printf("⚠️ Missing default revenue account 4101-001/4101: %v", err)
						continue
					}
				}
				defaultRevenueAcc = acc
			}
			accID = defaultRevenueAcc.ID
		}

		revenueTotals[accID] += amount
	}

	if len(revenueTotals) == 0 {
		// ✅ FIX: Fallback ke LEAF account 4101-001 (bukan header 4101)
		var fallbackAcc *models.Account
		if acc, err := resolveByCode("4101-001"); err == nil {
			fallbackAcc = acc
		} else if acc, err := resolveByCode("4101"); err == nil {
			fallbackAcc = acc
		}
		if fallbackAcc != nil {
			journalItems = append(journalItems, models.SimpleSSOTJournalItem{
				JournalID:   ssotEntry.ID,
				AccountID:   fallbackAcc.ID,
				AccountCode: fallbackAcc.Code,
				AccountName: fallbackAcc.Name,
				Debit:       0,
				Credit:      sale.Subtotal,
				Description: fmt.Sprintf("Sales revenue for Invoice #%s", sale.InvoiceNumber),
			})
			log.Printf("💰 Revenue: Credit to %s (%s) = %.2f", fallbackAcc.Name, fallbackAcc.Code, sale.Subtotal)
		} else {
			log.Printf("⚠️ Missing revenue account 4101-001/4101: no revenue account found")
		}
	} else {
		// Buat satu baris kredit per akun pendapatan
		for accID, amount := range revenueTotals {
			var acc models.Account
			if err := dbToUse.First(&acc, accID).Error; err != nil {
				log.Printf("⚠️ Failed to load revenue account %d: %v", accID, err)
				continue
			}

			journalItems = append(journalItems, models.SimpleSSOTJournalItem{
				JournalID:   ssotEntry.ID,
				AccountID:   acc.ID,
				AccountCode: acc.Code,
				AccountName: acc.Name,
				Debit:       0,
				Credit:      amount,
				Description: fmt.Sprintf("Sales revenue for Invoice #%s", sale.InvoiceNumber),
			})
			log.Printf("💰 Revenue: Credit to %s (%s) = %.2f", acc.Name, acc.Code, amount)
		}
	}

	// PPN Keluaran (2102-001) - Credit jika ada
	if sale.PPN > 0 {
		if acc, err := resolveByCode("2102-001"); err == nil {
			journalItems = append(journalItems, models.SimpleSSOTJournalItem{
				JournalID:   ssotEntry.ID,
				AccountID:   acc.ID,
				AccountCode: acc.Code,
				AccountName: acc.Name,
				Debit:       0,
				Credit:      sale.PPN,
				Description: fmt.Sprintf("Output VAT for Invoice #%s", sale.InvoiceNumber),
			})
			log.Printf("📊 PPN: Credit to %s (%s) = %.2f", acc.Name, acc.Code, sale.PPN)
		} else {
			log.Printf("⚠️ Missing PPN Keluaran account 2102-001: %v", err)
		}
	}

	// Create all journal items
	for _, item := range journalItems {
		if err := dbToUse.Create(&item).Error; err != nil {
			return fmt.Errorf("failed to create journal item: %v", err)
		}
	}

	// Update COA balances
	for _, item := range journalItems {
		if err := s.updateCOABalance(dbToUse, item.AccountID, item.Debit, item.Credit); err != nil {
			log.Printf("⚠️ Warning: Failed to update COA balance for account %d: %v", item.AccountID, err)
			// Continue processing, don't fail the entire transaction
		}

		// Update parent account balances after each account balance change
		if err := s.updateParentAccountBalances(dbToUse, item.AccountID); err != nil {
			log.Printf("⚠️ Warning: Failed to update parent balances for account %d: %v", item.AccountID, err)
			// Continue processing, don't fail the entire transaction
		}
	}

	log.Printf("✅ Successfully created journal entries for Sale #%d", sale.ID)
	return nil
}

// UpdateSalesJournal updates journal entries when sale is updated
func (s *SalesJournalServiceV2) UpdateSalesJournal(sale *models.Sale, oldStatus string, tx *gorm.DB) error {
	dbToUse := s.db
	if tx != nil {
		dbToUse = tx
	}

	// Check if we need to create or delete journal based on status change
	oldShouldPost := s.ShouldPostToJournal(oldStatus)
	newShouldPost := s.ShouldPostToJournal(sale.Status)

	if !oldShouldPost && newShouldPost {
		// Status changed from DRAFT/CONFIRMED to INVOICED/PAID - Create journal
		log.Printf("📈 Status changed from %s to %s - Creating journal entries", oldStatus, sale.Status)
		return s.CreateSalesJournal(sale, dbToUse)
	} else if oldShouldPost && !newShouldPost {
		// Status changed from INVOICED/PAID to DRAFT/CONFIRMED - Delete journal
		log.Printf("📉 Status changed from %s to %s - Removing journal entries", oldStatus, sale.Status)
		return s.DeleteSalesJournal(sale.ID, dbToUse)
	} else if oldShouldPost && newShouldPost {
		// Both statuses require journal - Update existing
		log.Printf("🔄 Updating existing journal entries for Sale #%d", sale.ID)

		// Delete old entries
		if err := s.DeleteSalesJournal(sale.ID, dbToUse); err != nil {
			return err
		}

		// Create new entries
		return s.CreateSalesJournal(sale, dbToUse)
	}

	log.Printf("ℹ️ No journal update needed for Sale #%d (Status: %s)", sale.ID, sale.Status)
	return nil
}

// DeleteSalesJournal deletes all journal entries for a sale
func (s *SalesJournalServiceV2) DeleteSalesJournal(saleID uint, tx *gorm.DB) error {
	dbToUse := s.db
	if tx != nil {
		dbToUse = tx
	}

	// Find SSOT journal entry
	var ssotJournal models.SimpleSSOTJournal
	if err := dbToUse.Where("transaction_type = ? AND transaction_id = ?", "SALES", saleID).
		First(&ssotJournal).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Printf("ℹ️ No journal entry found for Sale #%d", saleID)
			return nil
		}
		return fmt.Errorf("failed to find journal entry: %v", err)
	}

	// Get all journal items for balance reversal
	var journalItems []models.SimpleSSOTJournalItem
	if err := dbToUse.Where("journal_id = ?", ssotJournal.ID).Find(&journalItems).Error; err != nil {
		return fmt.Errorf("failed to find journal items: %v", err)
	}

	// Reverse COA balances
	for _, item := range journalItems {
		// Reverse the effect: subtract debit, add credit back
		if err := s.updateCOABalance(dbToUse, item.AccountID, -item.Debit, -item.Credit); err != nil {
			log.Printf("⚠️ Warning: Failed to reverse COA balance for account %d: %v", item.AccountID, err)
		}
	}

	// Delete journal items
	if err := dbToUse.Where("journal_id = ?", ssotJournal.ID).Delete(&models.SimpleSSOTJournalItem{}).Error; err != nil {
		return fmt.Errorf("failed to delete journal items: %v", err)
	}

	// Delete journal entry
	if err := dbToUse.Delete(&ssotJournal).Error; err != nil {
		return fmt.Errorf("failed to delete journal entry: %v", err)
	}

	log.Printf("🗑️ Deleted journal entries for Sale #%d", saleID)
	return nil
}

// CreateSalesPaymentJournal creates journal entries for sales payment
func (s *SalesJournalServiceV2) CreateSalesPaymentJournal(payment *models.SalePayment, sale *models.Sale, tx *gorm.DB) error {
	// Only create journal if sale status is INVOICED or PAID
	if !s.ShouldPostToJournal(sale.Status) {
		log.Printf("⚠️ Skipping payment journal for Sale #%d with status: %s", sale.ID, sale.Status)
		return nil
	}

	dbToUse := s.db
	if tx != nil {
		dbToUse = tx
	}

	// ✅ CRITICAL FIX: Check if payment journal already exists
	// Prevent duplicate payment journal entries which cause double posting to COA
	var existingCount int64
	if err := dbToUse.Model(&models.SimpleSSOTJournal{}).
		Where("transaction_type = ? AND transaction_id = ?", "SALES_PAYMENT", payment.ID).
		Count(&existingCount).Error; err == nil && existingCount > 0 {
		log.Printf("⚠️ Payment journal already exists for Payment #%d (found %d entries), skipping creation to prevent duplicate",
			payment.ID, existingCount)
		return nil // Don't create duplicate journal
	}

	// Helper to resolve account by code
	resolveByCode := func(code string) (*models.Account, error) {
		var acc models.Account
		if err := dbToUse.Where("code = ?", code).First(&acc).Error; err != nil {
			return nil, fmt.Errorf("account code %s not found: %v", code, err)
		}
		return &acc, nil
	}

	// Create SSOT Journal Entry for payment
	ssotEntry := &models.SimpleSSOTJournal{
		EntryNumber:       fmt.Sprintf("PAYMENT-%d", payment.ID),
		TransactionType:   "SALES_PAYMENT",
		TransactionID:     payment.ID,
		TransactionNumber: fmt.Sprintf("PAY-%d", payment.ID),
		Date:              payment.PaymentDate,
		Description:       fmt.Sprintf("Payment for Invoice #%s", sale.InvoiceNumber),
		TotalAmount:       payment.Amount,
		Status:            "POSTED",
		CreatedBy:         payment.UserID,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	if err := dbToUse.Create(ssotEntry).Error; err != nil {
		return fmt.Errorf("failed to create payment journal: %v", err)
	}

	// Prepare journal items
	var journalItems []models.SimpleSSOTJournalItem

	// DEBIT: Cash/Bank based on payment method
	var debitAcc *models.Account
	method := strings.ToUpper(strings.TrimSpace(payment.PaymentMethod))
	if payment.CashBankID != nil && *payment.CashBankID > 0 {
		// Use specific cash/bank account when provided
		var cashBank models.CashBank
		if err := dbToUse.First(&cashBank, *payment.CashBankID).Error; err == nil {
			var tmp models.Account
			if e2 := dbToUse.First(&tmp, cashBank.AccountID).Error; e2 == nil {
				debitAcc = &tmp
			}
		}
	}
	if debitAcc == nil {
		if method == "BANK" || strings.HasPrefix(method, "BANK") {
			// Default bank account if method indicates bank but cash_bank_id missing
			if acc, err := resolveByCode("1101-003"); err == nil {
				debitAcc = acc
			}
		} else {
			// Default cash account
			if acc, err := resolveByCode("1101"); err == nil {
				debitAcc = acc
			}
		}
	}
	if debitAcc != nil {
		journalItems = append(journalItems, models.SimpleSSOTJournalItem{
			JournalID:   ssotEntry.ID,
			AccountID:   debitAcc.ID,
			AccountCode: debitAcc.Code,
			AccountName: debitAcc.Name,
			Debit:       payment.Amount,
			Credit:      0,
			Description: fmt.Sprintf("Payment received for Invoice #%s", sale.InvoiceNumber),
		})
	}

	// CREDIT: Reduce Piutang Usaha (1201)
	if ar, err := resolveByCode("1201"); err == nil {
		journalItems = append(journalItems, models.SimpleSSOTJournalItem{
			JournalID:   ssotEntry.ID,
			AccountID:   ar.ID,
			AccountCode: ar.Code,
			AccountName: ar.Name,
			Debit:       0,
			Credit:      payment.Amount,
			Description: fmt.Sprintf("Reduce receivable for Invoice #%s", sale.InvoiceNumber),
		})
	}

	// Create all journal items
	for _, item := range journalItems {
		if err := dbToUse.Create(&item).Error; err != nil {
			return fmt.Errorf("failed to create payment journal item: %v", err)
		}
	}

	// Update COA balances
	for _, item := range journalItems {
		if err := s.updateCOABalance(dbToUse, item.AccountID, item.Debit, item.Credit); err != nil {
			log.Printf("⚠️ Warning: Failed to update COA balance for account %d: %v", item.AccountID, err)
		}

		// Update parent account balances after each account balance change
		if err := s.updateParentAccountBalances(dbToUse, item.AccountID); err != nil {
			log.Printf("⚠️ Warning: Failed to update parent balances for account %d: %v", item.AccountID, err)
		}
	}

	log.Printf("✅ Created payment journal for Payment #%d (Amount: %.2f)", payment.ID, payment.Amount)
	return nil
}

// Helper function to update COA balance
func (s *SalesJournalServiceV2) updateCOABalance(db *gorm.DB, accountID uint, debit, credit float64) error {
	var account models.Account
	if err := db.First(&account, accountID).Error; err != nil {
		return fmt.Errorf("Account %d not found: %v", accountID, err)
	}

	// Update balance based on account type
	netChange := debit - credit

	switch account.Type {
	case "ASSET", "EXPENSE":
		// Assets and Expenses increase with debit
		account.Balance += netChange
	case "LIABILITY", "EQUITY", "REVENUE":
		// Liabilities, Equity, and Revenue increase with credit
		account.Balance -= netChange
	}

	return db.Save(&account).Error
}

// updateParentAccountBalances updates parent account balances for a given account
func (s *SalesJournalServiceV2) updateParentAccountBalances(db *gorm.DB, accountID uint) error {
	var parentID *uint

	// Get parent ID
	if err := db.Raw("SELECT parent_id FROM accounts WHERE id = ? AND deleted_at IS NULL", accountID).Scan(&parentID).Error; err != nil {
		return fmt.Errorf("failed to get parent ID for account %d: %w", accountID, err)
	}

	// If has parent, update parent and continue up the chain
	if parentID != nil {
		// Calculate parent balance as sum of children
		var parentBalance float64
		if err := db.Raw(`
			SELECT COALESCE(SUM(balance), 0)
			FROM accounts 
			WHERE parent_id = ? AND deleted_at IS NULL
		`, *parentID).Scan(&parentBalance).Error; err != nil {
			return fmt.Errorf("failed to calculate parent balance for account %d: %w", *parentID, err)
		}

		// Update parent balance
		if err := db.Model(&models.Account{}).
			Where("id = ? AND deleted_at IS NULL", *parentID).
			Update("balance", parentBalance).Error; err != nil {
			return fmt.Errorf("failed to update parent balance for account %d: %w", *parentID, err)
		}

		// Recursively update grandparent chain
		return s.updateParentAccountBalances(db, *parentID)
	}

	return nil
}

// GetDisplayBalance returns the balance formatted for frontend display
func (s *SalesJournalServiceV2) GetDisplayBalance(accountID uint) (float64, error) {
	var coa models.COA
	if err := s.db.First(&coa, accountID).Error; err != nil {
		return 0, err
	}

	// For REVENUE and LIABILITY accounts (like PPN Keluaran),
	// return positive value for display
	if coa.Type == "REVENUE" || coa.Type == "LIABILITY" {
		return -coa.Balance, nil // Convert negative to positive for display
	}

	return coa.Balance, nil
}
