package services

import (
	"fmt"
	"log"
	"strings"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"

	"gorm.io/gorm"
)

// SalesJournalServiceEnhanced handles sales journal entries using configurable tax account settings
// Now also supports TaxConfig for tax rates and calculation rules
type SalesJournalServiceEnhanced struct {
	db                *gorm.DB
	journalRepo       repositories.JournalEntryRepository
	coaService        *COAService
	taxAccountService *TaxAccountService
}

// NewSalesJournalServiceEnhanced creates a new instance of SalesJournalServiceEnhanced
func NewSalesJournalServiceEnhanced(
	db *gorm.DB,
	journalRepo repositories.JournalEntryRepository,
	coaService *COAService,
	taxAccountService *TaxAccountService,
) *SalesJournalServiceEnhanced {
	return &SalesJournalServiceEnhanced{
		db:                db,
		journalRepo:       journalRepo,
		coaService:        coaService,
		taxAccountService: taxAccountService,
	}
}

// ShouldPostToJournal checks if a status should create journal entries
func (s *SalesJournalServiceEnhanced) ShouldPostToJournal(status string) bool {
	// KRITERIA POSTING JURNAL - HANYA STATUS INVOICED ATAU PAID
	allowedStatuses := []string{"INVOICED", "PAID"}
	for _, allowed := range allowedStatuses {
		if status == allowed {
			return true
		}
	}
	return false
}

// CreateSalesJournal creates journal entries for a sale using configurable account settings
func (s *SalesJournalServiceEnhanced) CreateSalesJournal(sale *models.Sale, tx *gorm.DB) error {
	// VALIDASI STATUS - HANYA INVOICED/PAID YANG BOLEH POSTING
	if !s.ShouldPostToJournal(sale.Status) {
		log.Printf("⚠️ Skipping journal creation for Sale #%d with status: %s (only INVOICED/PAID allowed)", sale.ID, sale.Status)
		return nil
	}

	log.Printf("📝 Creating enhanced journal entries for Sale #%d (Status: %s, Payment Method: %s)",
		sale.ID, sale.Status, sale.PaymentMethodType)

	// Tentukan database yang akan digunakan
	dbToUse := s.db
	if tx != nil {
		dbToUse = tx
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

	// Prepare journal items using tax account settings
	var journalItems []models.SimpleSSOTJournalItem

	// Determine DEBIT side based on payment method using tax account settings
	switch strings.ToUpper(strings.TrimSpace(sale.PaymentMethodType)) {
	case "CASH":
		// CASH → Debit Kas (from settings)
		accountID, err := s.taxAccountService.GetAccountID("sales_cash")
		if err != nil {
			log.Printf("⚠️ Failed to get sales cash account from settings: %v", err)
			// Fallback to hardcoded value
			accountID, err = s.taxAccountService.GetAccountByCode("1101")
			if err != nil {
				return fmt.Errorf("failed to find cash account: %v", err)
			}
		}

		var account models.Account
		if err := dbToUse.First(&account, accountID).Error; err != nil {
			return fmt.Errorf("failed to load cash account: %v", err)
		}

		journalItems = append(journalItems, models.SimpleSSOTJournalItem{
			JournalID:   ssotEntry.ID,
			AccountID:   account.ID,
			AccountCode: account.Code,
			AccountName: account.Name,
			Debit:       sale.TotalAmount,
			Credit:      0,
			Description: fmt.Sprintf("Cash payment for Invoice #%s", sale.InvoiceNumber),
		})
		log.Printf("💵 CASH Payment: Debit to %s (%s) = %.2f", account.Name, account.Code, sale.TotalAmount)

	case "BANK":
		// BANK → Debit ke Bank (specific cash_bank account if available, otherwise from settings)
		var accountID uint
		if sale.CashBankID != nil && *sale.CashBankID > 0 {
			// Use specific bank account from sale
			var cashBank models.CashBank
			if err := dbToUse.First(&cashBank, *sale.CashBankID).Error; err == nil {
				accountID = cashBank.AccountID
			}
		}

		if accountID == 0 {
			// Use default bank account from settings
			var err error
			accountID, err = s.taxAccountService.GetAccountID("sales_bank")
			if err != nil {
				log.Printf("⚠️ Failed to get sales bank account from settings: %v", err)
				// Fallback: 1101-003 = BANK GIRO (under KAS DAN SETARA KAS)
				accountID, err = s.taxAccountService.GetAccountByCode("1101-003")
				if err != nil {
					return fmt.Errorf("failed to find bank account: %v", err)
				}
			}
		}

		var account models.Account
		if err := dbToUse.First(&account, accountID).Error; err != nil {
			return fmt.Errorf("failed to load bank account: %v", err)
		}

		journalItems = append(journalItems, models.SimpleSSOTJournalItem{
			JournalID:   ssotEntry.ID,
			AccountID:   account.ID,
			AccountCode: account.Code,
			AccountName: account.Name,
			Debit:       sale.TotalAmount,
			Credit:      0,
			Description: fmt.Sprintf("Bank payment for Invoice #%s", sale.InvoiceNumber),
		})
		log.Printf("🏦 BANK Payment: Debit to %s (%s) = %.2f", account.Name, account.Code, sale.TotalAmount)

	default:
		// CREDIT or unknown → Debit Piutang Usaha (from settings)
		accountID, err := s.taxAccountService.GetAccountID("sales_receivable")
		if err != nil {
			log.Printf("⚠️ Failed to get sales receivable account from settings: %v", err)
			// Fallback to hardcoded value
			accountID, err = s.taxAccountService.GetAccountByCode("1201")
			if err != nil {
				return fmt.Errorf("failed to find receivable account: %v", err)
			}
		}

		var account models.Account
		if err := dbToUse.First(&account, accountID).Error; err != nil {
			return fmt.Errorf("failed to load receivable account: %v", err)
		}

		journalItems = append(journalItems, models.SimpleSSOTJournalItem{
			JournalID:   ssotEntry.ID,
			AccountID:   account.ID,
			AccountCode: account.Code,
			AccountName: account.Name,
			Debit:       sale.TotalAmount,
			Credit:      0,
			Description: fmt.Sprintf("Account receivable for Invoice #%s", sale.InvoiceNumber),
		})
		log.Printf("📋 CREDIT Payment: Debit to %s (%s) = %.2f", account.Name, account.Code, sale.TotalAmount)
	}

	// CREDIT SIDE - Revenue dan PPN using tax account settings
	// ✅ FIX: Revenue per SaleItem based on RevenueAccountID (same as SSOT/V2 services)
	// Previously posted ALL revenue to a single account from taxAccountService settings,
	// which was set to header 4101 — causing balance to be overwritten by RecalculateAllParentBalances
	if len(sale.SaleItems) == 0 {
		// Safety: preload SaleItems if not loaded
		var saleWithItems models.Sale
		if err := dbToUse.Preload("SaleItems").First(&saleWithItems, sale.ID).Error; err == nil {
			sale.SaleItems = saleWithItems.SaleItems
		} else {
			log.Printf("⚠️ Failed to preload SaleItems for Sale #%d: %v", sale.ID, err)
		}
	}

	// Group revenue totals per account
	revenueTotals := make(map[uint]float64)
	var defaultRevenueAcc *models.Account

	for _, item := range sale.SaleItems {
		amount := item.LineTotal
		if amount == 0 {
			continue
		}

		accID := item.RevenueAccountID
		if accID != 0 {
			// ✅ FIX: Check if the account is a header, redirect to leaf child
			var acc models.Account
			if err := dbToUse.First(&acc, accID).Error; err == nil && acc.IsHeader {
				log.Printf("⚠️ [Enhanced] SaleItem #%d has header account %s (ID=%d), finding leaf child", item.ID, acc.Code, acc.ID)
				var leaf models.Account
				if err := dbToUse.Where("parent_id = ? AND is_header = false AND deleted_at IS NULL", acc.ID).First(&leaf).Error; err == nil {
					accID = leaf.ID
					log.Printf("✅ [Enhanced] Redirected to leaf account %s (ID=%d)", leaf.Code, leaf.ID)
				} else {
					log.Printf("⚠️ [Enhanced] No leaf child found for header %s, using default", acc.Code)
					accID = 0 // Force fallback
				}
			}
		}

		if accID == 0 {
			// Fallback: use LEAF account 4101-001 (NOT header 4101)
			if defaultRevenueAcc == nil {
				var acc models.Account
				if err := dbToUse.Where("code = ? AND deleted_at IS NULL", "4101-001").First(&acc).Error; err != nil {
					// Last resort: try any non-header REVENUE account
					if err := dbToUse.Where("type = ? AND is_header = false AND deleted_at IS NULL", "REVENUE").First(&acc).Error; err != nil {
						log.Printf("⚠️ [Enhanced] No revenue account found!")
						continue
					}
				}
				defaultRevenueAcc = &acc
			}
			accID = defaultRevenueAcc.ID
		}

		revenueTotals[accID] += amount
	}

	// If no SaleItems found, fall back to subtotal on single account
	if len(revenueTotals) == 0 && sale.Subtotal > 0 {
		if defaultRevenueAcc == nil {
			var acc models.Account
			if err := dbToUse.Where("code = ? AND deleted_at IS NULL", "4101-001").First(&acc).Error; err == nil {
				defaultRevenueAcc = &acc
			}
		}
		if defaultRevenueAcc != nil {
			revenueTotals[defaultRevenueAcc.ID] = sale.Subtotal
		}
	}

	// Create journal items for each revenue account
	for accID, total := range revenueTotals {
		var account models.Account
		if err := dbToUse.First(&account, accID).Error; err != nil {
			log.Printf("⚠️ [Enhanced] Failed to load revenue account %d: %v", accID, err)
			continue
		}
		journalItems = append(journalItems, models.SimpleSSOTJournalItem{
			JournalID:   ssotEntry.ID,
			AccountID:   account.ID,
			AccountCode: account.Code,
			AccountName: account.Name,
			Debit:       0,
			Credit:      total,
			Description: fmt.Sprintf("Sales revenue for Invoice #%s", sale.InvoiceNumber),
		})
		log.Printf("💰 Revenue: Credit to %s (%s) = %.2f", account.Name, account.Code, total)
	}

	// PPN Keluaran (from settings) - Credit jika ada
	// ✅ FIX: Safety net — recalculate PPN if sale.PPN=0 but PPNPercent > 0
	ppnValue := sale.PPN
	if ppnValue == 0 && sale.PPNPercent > 0 {
		taxableAmt := sale.TaxableAmount
		if taxableAmt == 0 {
			taxableAmt = sale.Subtotal - sale.DiscountAmount
		}
		if taxableAmt > 0 {
			ppnValue = taxableAmt * (sale.PPNPercent / 100)
			log.Printf("⚠️ [Enhanced] PPN was 0 but PPNPercent=%.2f, recalculated PPN=%.2f", sale.PPNPercent, ppnValue)
		}
	}

	if ppnValue > 0 {
		accountID, err := s.taxAccountService.GetAccountID("sales_output_vat")
		if err != nil {
			log.Printf("⚠️ Failed to get sales output VAT account from settings: %v", err)
			// Fallback: 2102-001 = PPN KELUARAN (under UTANG PAJAK)
			accountID, err = s.taxAccountService.GetAccountByCode("2102-001")
			if err != nil {
				return fmt.Errorf("failed to find output VAT account: %v", err)
			}
		}

		var account models.Account
		if err := dbToUse.First(&account, accountID).Error; err != nil {
			return fmt.Errorf("failed to load output VAT account: %v", err)
		}

		journalItems = append(journalItems, models.SimpleSSOTJournalItem{
			JournalID:   ssotEntry.ID,
			AccountID:   account.ID,
			AccountCode: account.Code,
			AccountName: account.Name,
			Debit:       0,
			Credit:      ppnValue,
			Description: fmt.Sprintf("Output VAT for Invoice #%s", sale.InvoiceNumber),
		})
		log.Printf("📊 PPN: Credit to %s (%s) = %.2f", account.Name, account.Code, ppnValue)
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
		}
	}

	log.Printf("✅ Successfully created enhanced journal entries for Sale #%d", sale.ID)
	return nil
}

// UpdateSalesJournal updates journal entries when sale is updated
func (s *SalesJournalServiceEnhanced) UpdateSalesJournal(sale *models.Sale, oldStatus string, tx *gorm.DB) error {
	dbToUse := s.db
	if tx != nil {
		dbToUse = tx
	}

	// Check if we need to create or delete journal based on status change
	oldShouldPost := s.ShouldPostToJournal(oldStatus)
	newShouldPost := s.ShouldPostToJournal(sale.Status)

	if !oldShouldPost && newShouldPost {
		// Status changed from DRAFT/CONFIRMED to INVOICED/PAID - Create journal
		log.Printf("📈 Status changed from %s to %s - Creating enhanced journal entries", oldStatus, sale.Status)
		return s.CreateSalesJournal(sale, dbToUse)
	} else if oldShouldPost && !newShouldPost {
		// Status changed from INVOICED/PAID to DRAFT/CONFIRMED - Delete journal
		log.Printf("📉 Status changed from %s to %s - Removing journal entries", oldStatus, sale.Status)
		return s.DeleteSalesJournal(sale.ID, dbToUse)
	} else if oldShouldPost && newShouldPost {
		// Both statuses require journal - Update existing
		log.Printf("🔄 Updating existing enhanced journal entries for Sale #%d", sale.ID)

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
func (s *SalesJournalServiceEnhanced) DeleteSalesJournal(saleID uint, tx *gorm.DB) error {
	dbToUse := s.db
	if tx != nil {
		dbToUse = tx
	}

	log.Printf("🗑️ Deleting enhanced journal entries for Sale #%d", saleID)

	// Find the SSOT journal entry
	var ssotJournal models.SimpleSSOTJournal
	err := dbToUse.Where("transaction_type = ? AND transaction_id = ?", "SALES", saleID).First(&ssotJournal).Error
	if err != nil {
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

	log.Printf("🗑️ Deleted enhanced journal entries for Sale #%d", saleID)
	return nil
}

// CreateSalesPaymentJournal creates journal entries for sales payment using configurable accounts
func (s *SalesJournalServiceEnhanced) CreateSalesPaymentJournal(payment *models.SalePayment, sale *models.Sale, tx *gorm.DB) error {
	// Only create journal if sale status is INVOICED or PAID
	if !s.ShouldPostToJournal(sale.Status) {
		log.Printf("⚠️ Skipping payment journal for Sale #%d with status: %s", sale.ID, sale.Status)
		return nil
	}

	dbToUse := s.db
	if tx != nil {
		dbToUse = tx
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

	// DEBIT: Cash/Bank based on payment method using tax account settings
	method := strings.ToUpper(strings.TrimSpace(payment.PaymentMethod))
	var debitAccountID uint

	if payment.CashBankID != nil && *payment.CashBankID > 0 {
		// Use specific cash/bank account when provided
		var cashBank models.CashBank
		if err := dbToUse.First(&cashBank, *payment.CashBankID).Error; err == nil {
			debitAccountID = cashBank.AccountID
		}
	}

	if debitAccountID == 0 {
		// Use tax account settings
		var err error
		if method == "BANK" || strings.HasPrefix(method, "BANK") {
			// Use bank account from settings
			debitAccountID, err = s.taxAccountService.GetAccountID("sales_bank")
			if err != nil {
				// Fallback: 1101-003 = BANK GIRO
				debitAccountID, err = s.taxAccountService.GetAccountByCode("1101-003")
			}
		} else {
			// Use cash account from settings
			debitAccountID, err = s.taxAccountService.GetAccountID("sales_cash")
			if err != nil {
				// Fallback to hardcoded
				debitAccountID, err = s.taxAccountService.GetAccountByCode("1101")
			}
		}

		if err != nil {
			return fmt.Errorf("failed to find payment account: %v", err)
		}
	}

	// Load debit account
	var debitAccount models.Account
	if err := dbToUse.First(&debitAccount, debitAccountID).Error; err != nil {
		return fmt.Errorf("failed to load payment account: %v", err)
	}

	journalItems = append(journalItems, models.SimpleSSOTJournalItem{
		JournalID:   ssotEntry.ID,
		AccountID:   debitAccount.ID,
		AccountCode: debitAccount.Code,
		AccountName: debitAccount.Name,
		Debit:       payment.Amount,
		Credit:      0,
		Description: fmt.Sprintf("Payment received for Invoice #%s", sale.InvoiceNumber),
	})

	// CREDIT: Reduce Piutang Usaha using tax account settings
	receivableAccountID, err := s.taxAccountService.GetAccountID("sales_receivable")
	if err != nil {
		log.Printf("⚠️ Failed to get sales receivable account from settings: %v", err)
		// Fallback to hardcoded
		receivableAccountID, err = s.taxAccountService.GetAccountByCode("1201")
		if err != nil {
			return fmt.Errorf("failed to find receivable account: %v", err)
		}
	}

	var receivableAccount models.Account
	if err := dbToUse.First(&receivableAccount, receivableAccountID).Error; err != nil {
		return fmt.Errorf("failed to load receivable account: %v", err)
	}

	journalItems = append(journalItems, models.SimpleSSOTJournalItem{
		JournalID:   ssotEntry.ID,
		AccountID:   receivableAccount.ID,
		AccountCode: receivableAccount.Code,
		AccountName: receivableAccount.Name,
		Debit:       0,
		Credit:      payment.Amount,
		Description: fmt.Sprintf("Reduce receivable for Invoice #%s", sale.InvoiceNumber),
	})

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

	log.Printf("✅ Created enhanced payment journal for Payment #%d (Amount: %.2f)", payment.ID, payment.Amount)
	return nil
}

// Helper function to update COA balance
func (s *SalesJournalServiceEnhanced) updateCOABalance(db *gorm.DB, accountID uint, debit, credit float64) error {
	var coa models.COA
	if err := db.First(&coa, accountID).Error; err != nil {
		return fmt.Errorf("COA account %d not found: %v", accountID, err)
	}

	// Update balance based on account type
	netChange := debit - credit

	switch coa.Type {
	case "ASSET", "EXPENSE":
		// Assets and Expenses increase with debit
		coa.Balance += netChange
	case "LIABILITY", "EQUITY", "REVENUE":
		// Liabilities, Equity, and Revenue increase with credit
		coa.Balance -= netChange
	}

	return db.Save(&coa).Error
}

// GetDisplayBalance returns the balance formatted for frontend display
func (s *SalesJournalServiceEnhanced) GetDisplayBalance(accountID uint) (float64, error) {
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

// updateParentAccountBalances updates parent account balances for a given account
func (s *SalesJournalServiceEnhanced) updateParentAccountBalances(db *gorm.DB, accountID uint) error {
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
