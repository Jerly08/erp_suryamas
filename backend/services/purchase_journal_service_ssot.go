package services

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// PurchaseJournalServiceSSOT handles purchase journal entries with CORRECT unified_journal_ledger integration
// This service writes to unified_journal_ledger which is read by Balance Sheet service
type PurchaseJournalServiceSSOT struct {
	db               *gorm.DB
	coaService       *COAService
	settingsService  *SettingsService
	taxAccountHelper *TaxAccountHelper
}

// NewPurchaseJournalServiceSSOT creates a new instance
func NewPurchaseJournalServiceSSOT(db *gorm.DB, coaService *COAService, settingsService *SettingsService) *PurchaseJournalServiceSSOT {
	return &PurchaseJournalServiceSSOT{
		db:               db,
		coaService:       coaService,
		settingsService:  settingsService,
		taxAccountHelper: NewTaxAccountHelper(db),
	}
}

// ShouldPostToJournal checks if a status should create journal entries
func (s *PurchaseJournalServiceSSOT) ShouldPostToJournal(status string) bool {
	// Purchase posting criteria: APPROVED, COMPLETED, or PAID
	allowedStatuses := []string{"APPROVED", "COMPLETED", "PAID"}
	for _, allowed := range allowedStatuses {
		if strings.EqualFold(status, allowed) {
			return true
		}
	}
	return false
}

// CreatePurchaseJournal creates journal entries in unified_journal_ledger for Balance Sheet integration
func (s *PurchaseJournalServiceSSOT) CreatePurchaseJournal(purchase *models.Purchase, tx *gorm.DB) error {
	// VALIDASI STATUS
	if !s.ShouldPostToJournal(purchase.Status) {
		log.Printf("⚠️ [SSOT] Skipping purchase journal creation for Purchase #%d with status: %s (only APPROVED/COMPLETED/PAID allowed)",
			purchase.ID, purchase.Status)
		return nil
	}

	log.Printf("📝 [SSOT] Creating unified journal entries for Purchase #%d (Status: %s, Payment Method: '%s')",
		purchase.ID, purchase.Status, purchase.PaymentMethod)

	// ✅ FIX: Don't fail if payment method is empty, use default CREDIT
	// This prevents blocking journal creation for valid purchases
	if strings.TrimSpace(purchase.PaymentMethod) == "" {
		log.Printf("⚠️ [SSOT] Warning: Purchase #%d has empty PaymentMethod, defaulting to CREDIT", purchase.ID)
		// Don't return error, allow journal creation with default
	}

	// Tentukan database yang akan digunakan
	dbToUse := s.db
	if tx != nil {
		dbToUse = tx
	}

	// Check if journal already exists
	var existingCount int64
	if err := dbToUse.Model(&models.SSOTJournalEntry{}).
		Where("source_type = ? AND source_id = ?", "PURCHASE", purchase.ID).
		Count(&existingCount).Error; err == nil && existingCount > 0 {
		log.Printf("⚠️ [SSOT] Journal already exists for Purchase #%d (found %d entries), skipping",
			purchase.ID, existingCount)
		return nil
	}

	// Helper to resolve account by code
	resolveByCode := func(code string) (*models.Account, error) {
		var acc models.Account
		if err := dbToUse.Where("code = ?", code).First(&acc).Error; err != nil {
			return nil, fmt.Errorf("account code %s not found: %v", code, err)
		}
		return &acc, nil
	}

	// Prepare journal lines
	var lines []PurchaseJournalLineRequest

	// Calculate totals
	var subtotal float64
	for _, item := range purchase.PurchaseItems {
		subtotal += item.TotalPrice
	}

	// 1. DEBIT SIDE - Inventory/Expense/Fixed Asset accounts for each item
	for _, item := range purchase.PurchaseItems {
		var debitAccount *models.Account

		// ✅ FIX: Jika user memilih akun Fixed Asset (mis. TRUK 1509) di Purchase Item,
		// gunakan akun tersebut langsung sebagai debit, meskipun produknya bertipe persediaan.
		// Ini memastikan saldo aset tetap (COA) bertambah sesuai harapan user.
		if item.ExpenseAccountID != 0 {
			var configuredAcc models.Account
			if err := dbToUse.First(&configuredAcc, item.ExpenseAccountID).Error; err == nil {
				if strings.EqualFold(configuredAcc.Category, models.CategoryFixedAsset) {
					debitAccount = &configuredAcc
					itemName := "Aset"
					if item.Product != nil {
						itemName = item.Product.Name
					} else if item.Description != "" {
						itemName = item.Description
					}
					log.Printf("🏗️ [SSOT] Fixed asset purchase detected for item '%s' (item %d); debiting fixed asset account %s (%s)",
						itemName, item.ID, configuredAcc.Code, configuredAcc.Name)
				}
			} else {
				log.Printf("⚠️ [SSOT] Failed to load configured expense account %d for item %d: %v",
					item.ExpenseAccountID, item.ID, err)
			}
		}

		// Jika bukan fixed asset, pakai aturan persediaan/beban seperti biasa
		if debitAccount == nil {
			// Check company type
			settings, err := s.settingsService.GetSettings()
			isServiceCompany := false
			if err == nil && settings.CompanyType == "SERVICE" {
				isServiceCompany = true
			}

			// Treat physical goods (non-service products) as inventory purchases
			isInventoryItem := item.Product != nil && item.Product.ID != 0 && !item.Product.IsService

			if isServiceCompany {
				// ✅ SERVICE COMPANY LOGIC:
				// Everything is an expense. No inventory tracking.
				// If specific expense account provided, use it.
				// If not, fallback to Cost of Services (5000) or Project Cost (5102)

				if item.ExpenseAccountID != 0 {
					var expenseAcc models.Account
					if err := dbToUse.First(&expenseAcc, item.ExpenseAccountID).Error; err == nil {
						debitAccount = &expenseAcc
					}
				}

				if debitAccount == nil {
					// Fallback: 5102-001 (Biaya Proyek) or 5000 (Beban Pokok Jasa)
					// Try to find a suitable expense account
					var fallbackAcc models.Account
					// Try "5102-001" (Biaya Proyek) first
					if err := dbToUse.Where("code = ?", "5102-001").First(&fallbackAcc).Error; err == nil {
						debitAccount = &fallbackAcc
					} else {
						// Try "5000" (Header - Beban Pokok Jasa) as last resort
						if err := dbToUse.Where("code = ?", "5000").First(&fallbackAcc).Error; err == nil {
							debitAccount = &fallbackAcc
						} else {
							// If all else fails, log warning and use 1103 (Inventory) just to allow transaction
							// but this shouldn't happen with proper COA
							log.Printf("⚠️ [SSOT] Service company but could not find expense account for purchase. Falling back to default logic.")
						}
					}
				}
			}

			// If still nil (Trading/Mfg OR Service fallback failed), run standard inventory logic
			if debitAccount == nil {
				if isInventoryItem {
					// For stock items, ALWAYS use configured inventory account so purchase debits
					// match the same account that COGS will credit later
					invAcc, err := s.getInventoryAccount(dbToUse)
					if err != nil {
						return fmt.Errorf("inventory account not found: %v", err)
					}
					debitAccount = invAcc

					// If an expense account was configured for an inventory product, ignore it but log for visibility
					if item.ExpenseAccountID != 0 {
						log.Printf("⚠️ [SSOT] Inventory product '%s' (item %d) has ExpenseAccountID=%d configured; ignoring it and using inventory account %s (%s) to keep COGS in sync",
							item.Product.Name, item.ID, item.ExpenseAccountID, invAcc.Code, invAcc.Name)
					}
				} else {
					// Non-stock / service purchases: respect explicit expense account when provided
					if item.ExpenseAccountID != 0 {
						var expenseAcc models.Account
						if err := dbToUse.First(&expenseAcc, item.ExpenseAccountID).Error; err != nil {
							log.Printf("⚠️ Expense account ID %d not found for item %d, falling back to inventory account",
								item.ExpenseAccountID, item.ID)
							invAcc, err := s.getInventoryAccount(dbToUse)
							if err != nil {
								return fmt.Errorf("inventory account not found: %v", err)
							}
							debitAccount = invAcc
						} else {
							debitAccount = &expenseAcc
						}
					} else {
						// Default to inventory account when no specific expense account configured
						// Try standard template code first (1103 Persediaan)
						invAcc, err := s.getInventoryAccount(dbToUse)
						if err != nil {
							return fmt.Errorf("inventory account not found: %v", err)
						}
						debitAccount = invAcc
					}
				}
			}
		}

		itemName := "Item"
		if item.Product != nil && item.Product.Name != "" {
			itemName = item.Product.Name
		} else if item.Description != "" {
			itemName = item.Description
		}

		lines = append(lines, PurchaseJournalLineRequest{
			AccountID:    uint64(debitAccount.ID),
			DebitAmount:  decimal.NewFromFloat(item.TotalPrice),
			CreditAmount: decimal.Zero,
			Description:  fmt.Sprintf("Pembelian - %s", itemName),
		})
	}

	// 2. DEBIT SIDE - PPN Masukan (Input VAT) if exists
	if purchase.PPNAmount > 0 {
		ppnMasukanAccount, err := s.taxAccountHelper.GetPurchaseInputVATAccount(dbToUse)
		if err != nil {
			log.Printf("⚠️ PPN Masukan account from tax settings not found, skipping PPN entry: %v", err)
		} else {
			lines = append(lines, PurchaseJournalLineRequest{
				AccountID:    uint64(ppnMasukanAccount.ID),
				DebitAmount:  decimal.NewFromFloat(purchase.PPNAmount),
				CreditAmount: decimal.Zero,
				Description:  fmt.Sprintf("PPN Masukan - %s", purchase.Code),
			})
		}
	}

	// 3. CREDIT SIDE - Based on payment method
	var creditAccount *models.Account
	var err error

	paymentMethod := strings.ToUpper(strings.TrimSpace(purchase.PaymentMethod))

	// Normalize payment method using domain constants to avoid mismatches
	switch paymentMethod {
	// Immediate cash payments -> always use default Kas (1101)
	case strings.ToUpper(models.PurchasePaymentCash), "TUNAI", "CASH":
		creditAccount, err = resolveByCode("1101-002") // Kas Besar (Leaf)

		if err != nil {
			return fmt.Errorf("cash account not found: %v", err)
		}
	// Immediate bank-based payments (transfer & cheque)
	case strings.ToUpper(models.PurchasePaymentTransfer), strings.ToUpper(models.PurchasePaymentCheck), "TRANSFER", "BANK":
		// ✅ NEW LOGIC: gunakan GL account dari bank yang dipilih (CashBank.AccountID)
		if purchase.BankAccountID != nil && *purchase.BankAccountID > 0 {
			var cashBank models.CashBank
			if err := dbToUse.First(&cashBank, *purchase.BankAccountID).Error; err == nil && cashBank.AccountID != 0 {
				var bankAcc models.Account
				if e2 := dbToUse.First(&bankAcc, cashBank.AccountID).Error; e2 == nil {
					creditAccount = &bankAcc
					log.Printf("🏦 [SSOT] Using linked bank GL account %s (%s) for Purchase #%d (CashBank: %s)", bankAcc.Code, bankAcc.Name, purchase.ID, cashBank.Name)
				}
			} else if err != nil {
				log.Printf("⚠️ [SSOT] Failed to load CashBank ID %d for Purchase #%d: %v", *purchase.BankAccountID, purchase.ID, err)
			}
		}
		// Fallback: gunakan akun BANK default 1101-004 bila tidak berhasil resolve dari CashBank
		if creditAccount == nil {
			creditAccount, err = resolveByCode("1101-004") // Bank Tabungan (Leaf)

			if err != nil {
				return fmt.Errorf("bank account not found: %v", err)
			}
			log.Printf("🏦 [SSOT] Fallback to default bank account 1102 for Purchase #%d", purchase.ID)
		}
	// Credit purchases / payables
	case strings.ToUpper(models.PurchasePaymentCredit), "KREDIT", "CREDIT", "HUTANG":
		creditAccount, err = resolveByCode("2101-001") // Hutang Usaha - Lokal (Leaf)

		if err != nil {
			return fmt.Errorf("payables account not found: %v", err)
		}
	default:
		// ✅ SAFE DEFAULT: Treat unknown methods as CREDIT (AP) to avoid mis-posting to cash/bank
		log.Printf("⚠️ [SSOT] Warning: Unknown payment method '%s' for Purchase #%d, defaulting to CREDIT (Hutang)",
			purchase.PaymentMethod, purchase.ID)
		creditAccount, err = resolveByCode("2101-001") // Hutang Usaha - Lokal (Leaf)

		if err != nil {
			return fmt.Errorf("payables account not found: %v", err)
		}
	}

	// Calculate total credit (subtotal + PPN - any withholdings)
	totalCredit := subtotal + purchase.PPNAmount

	// Handle withholding taxes (reduce the credit amount) - use configured account
	if purchase.PPh21Amount > 0 {
		pph21Account, err := s.taxAccountHelper.GetWithholdingTax21Account(dbToUse)
		if err != nil {
			log.Printf("⚠️ PPh 21 account not found, skipping PPh21 entry: %v", err)
		} else {
			lines = append(lines, PurchaseJournalLineRequest{
				AccountID:    uint64(pph21Account.ID),
				DebitAmount:  decimal.NewFromFloat(purchase.PPh21Amount),
				CreditAmount: decimal.Zero,
				Description:  fmt.Sprintf("PPh 21 - %s", purchase.Code),
			})
			totalCredit -= purchase.PPh21Amount
		}
	}

	if purchase.PPh23Amount > 0 {
		pph23Account, err := s.taxAccountHelper.GetWithholdingTax23Account(dbToUse)
		if err != nil {
			log.Printf("⚠️ PPh 23 account not found, skipping PPh23 entry: %v", err)
		} else {
			lines = append(lines, PurchaseJournalLineRequest{
				AccountID:    uint64(pph23Account.ID),
				DebitAmount:  decimal.NewFromFloat(purchase.PPh23Amount),
				CreditAmount: decimal.Zero,
				Description:  fmt.Sprintf("PPh 23 - %s", purchase.Code),
			})
			totalCredit -= purchase.PPh23Amount
		}
	}

	// Add the credit line
	lines = append(lines, PurchaseJournalLineRequest{
		AccountID:    uint64(creditAccount.ID),
		DebitAmount:  decimal.Zero,
		CreditAmount: decimal.NewFromFloat(totalCredit),
		Description:  fmt.Sprintf("Pembayaran Pembelian - %s", purchase.Code),
	})

	// Calculate totals
	var totalDebit, totalCreditCalc decimal.Decimal
	for _, line := range lines {
		totalDebit = totalDebit.Add(line.DebitAmount)
		totalCreditCalc = totalCreditCalc.Add(line.CreditAmount)
	}

	// Verify balanced
	if !totalDebit.Equal(totalCreditCalc) {
		return fmt.Errorf("journal entry not balanced: debit=%.2f, credit=%.2f",
			totalDebit.InexactFloat64(), totalCreditCalc.InexactFloat64())
	}

	// Create journal entry
	// Insert as DRAFT first to avoid trigger validation before lines are created
	sourceID := uint64(purchase.ID)
	now := time.Now()
	postedBy := uint64(purchase.UserID)

	journalEntry := &models.SSOTJournalEntry{
		EntryNumber:     fmt.Sprintf("PURCHASE-%d-%d", purchase.ID, now.Unix()),
		SourceType:      "PURCHASE",
		SourceID:        &sourceID,
		SourceCode:      purchase.Code,
		EntryDate:       purchase.Date,
		Description:     fmt.Sprintf("Purchase Order #%s - %s", purchase.Code, purchase.Vendor.Name),
		Reference:       purchase.Code,
		TotalDebit:      totalDebit,
		TotalCredit:     totalCreditCalc,
		Status:          "DRAFT",
		IsBalanced:      true,
		IsAutoGenerated: true,
		CreatedBy:       uint64(purchase.UserID),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := dbToUse.Create(journalEntry).Error; err != nil {
		return fmt.Errorf("failed to create SSOT journal entry: %v", err)
	}

	// Create journal lines
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

		if err := dbToUse.Create(journalLine).Error; err != nil {
			return fmt.Errorf("failed to create SSOT journal line: %v", err)
		}

		// ✅ RE-ENABLED: Update account balance for COA tree view
		// P&L uses journal entries (correct), but COA Tree View uses account.balance field
		if err := s.updateAccountBalance(dbToUse, lineReq.AccountID, lineReq.DebitAmount, lineReq.CreditAmount); err != nil {
			log.Printf("⚠️ Warning: Failed to update account balance for account %d: %v", lineReq.AccountID, err)
			// Continue processing - don't fail transaction for balance update issues
		}
	}

	// Now update status to POSTED after all lines are created
	if err := dbToUse.Model(journalEntry).Updates(map[string]interface{}{
		"status":    "POSTED",
		"posted_at": &now,
		"posted_by": &postedBy,
	}).Error; err != nil {
		return fmt.Errorf("failed to post journal entry: %v", err)
	}
	journalEntry.Status = "POSTED" // Update in-memory object

	log.Printf("✅ [SSOT] Created and posted purchase journal entry #%d with %d lines (Debit: %.2f, Credit: %.2f)",
		journalEntry.ID, len(lines), totalDebit.InexactFloat64(), totalCreditCalc.InexactFloat64())

	return nil
}

// updateAccountBalance updates account.balance field for COA tree view display
// RE-ENABLED: COA Tree View needs this field updated
// Note: P&L Report calculates balance from journal entries (real-time, always correct)
//
//	COA Tree View reads from account.balance field (needs manual update)
func (s *PurchaseJournalServiceSSOT) updateAccountBalance(db *gorm.DB, accountID uint64, debitAmount, creditAmount decimal.Decimal) error {
	var account models.Account
	if err := db.First(&account, accountID).Error; err != nil {
		return fmt.Errorf("account %d not found: %v", accountID, err)
	}

	// Calculate net change: debit - credit
	debit := debitAmount.InexactFloat64()
	credit := creditAmount.InexactFloat64()
	netChange := debit - credit

	// Update balance based on account type
	switch strings.ToUpper(account.Type) {
	case "ASSET", "EXPENSE":
		// Assets and Expenses: debit increases balance
		account.Balance += netChange
	case "LIABILITY", "EQUITY", "REVENUE":
		// Liabilities, Equity, Revenue: credit increases balance (so debit decreases)
		account.Balance -= netChange
	}

	if err := db.Model(&account).Select("Balance").Updates(models.Account{Balance: account.Balance}).Error; err != nil {
		return fmt.Errorf("failed to save account balance: %v", err)
	}

	log.Printf("💰 [SSOT] Updated account %s (%s) balance: Dr=%.2f, Cr=%.2f, Change=%.2f, New Balance=%.2f",
		account.Code, account.Name, debit, credit, netChange, account.Balance)

	return nil
}

// UpdatePurchaseJournal updates journal entries based on status change
func (s *PurchaseJournalServiceSSOT) UpdatePurchaseJournal(purchase *models.Purchase, oldStatus string, tx *gorm.DB) error {
	dbToUse := s.db
	if tx != nil {
		dbToUse = tx
	}

	oldShouldPost := s.ShouldPostToJournal(oldStatus)
	newShouldPost := s.ShouldPostToJournal(purchase.Status)

	if !oldShouldPost && newShouldPost {
		// Create journal
		log.Printf("📈 [SSOT] Status changed from %s to %s - Creating purchase journal entries", oldStatus, purchase.Status)
		return s.CreatePurchaseJournal(purchase, dbToUse)
	} else if oldShouldPost && !newShouldPost {
		// Delete journal
		log.Printf("📉 [SSOT] Status changed from %s to %s - Removing purchase journal entries", oldStatus, purchase.Status)
		return s.DeletePurchaseJournal(purchase.ID, dbToUse)
	} else if oldShouldPost && newShouldPost {
		// Update existing
		log.Printf("🔄 [SSOT] Updating purchase journal entries for Purchase #%d", purchase.ID)

		if err := s.DeletePurchaseJournal(purchase.ID, dbToUse); err != nil {
			return err
		}

		return s.CreatePurchaseJournal(purchase, dbToUse)
	}

	log.Printf("ℹ️ [SSOT] No journal update needed for Purchase #%d (Status: %s)", purchase.ID, purchase.Status)
	return nil
}

// DeletePurchaseJournal deletes all journal entries for a purchase
func (s *PurchaseJournalServiceSSOT) DeletePurchaseJournal(purchaseID uint, tx *gorm.DB) error {
	dbToUse := s.db
	if tx != nil {
		dbToUse = tx
	}

	// Find journal entry
	var entry models.SSOTJournalEntry
	if err := dbToUse.Where("source_type = ? AND source_id = ?", "PURCHASE", purchaseID).First(&entry).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Printf("⚠️ [SSOT] No journal found for Purchase #%d, nothing to delete", purchaseID)
			return nil
		}
		return fmt.Errorf("failed to find journal entry: %v", err)
	}

	// CRITICAL FIX: Reverse COA balances before deleting journal lines
	// This ensures accounts.balance (COA Tree View) is consistent after deletion
	var journalLines []models.SSOTJournalLine
	if err := dbToUse.Where("journal_id = ?", entry.ID).Find(&journalLines).Error; err != nil {
		log.Printf("⚠️ Warning: Failed to load journal lines for COA reversal: %v", err)
	} else {
		for _, line := range journalLines {
			// Negate debit/credit to reverse the effect on COA balances
			negDebit := line.DebitAmount.Neg()
			negCredit := line.CreditAmount.Neg()

			if err := s.updateAccountBalance(dbToUse, line.AccountID, negDebit, negCredit); err != nil {
				log.Printf("⚠️ Warning: Failed to reverse COA balance for account %d: %v", line.AccountID, err)
			}
		}
		log.Printf("✅ [SSOT] Reversed COA balances for %d journal lines of Purchase #%d", len(journalLines), purchaseID)
	}

	// Delete lines first (FK constraint)
	if err := dbToUse.Where("journal_id = ?", entry.ID).Delete(&models.SSOTJournalLine{}).Error; err != nil {
		return fmt.Errorf("failed to delete journal lines: %v", err)
	}

	// Delete entry
	if err := dbToUse.Delete(&entry).Error; err != nil {
		return fmt.Errorf("failed to delete journal entry: %v", err)
	}

	log.Printf("✅ [SSOT] Deleted journal entry #%d and its lines for Purchase #%d (COA balances reversed)", entry.ID, purchaseID)
	return nil
}

// PurchaseJournalLineRequest represents a request to create a purchase journal line
type PurchaseJournalLineRequest struct {
	AccountID    uint64
	DebitAmount  decimal.Decimal
	CreditAmount decimal.Decimal
	Description  string
}

// getInventoryAccount attempts to resolve inventory account using template code (1103) then legacy (1301)
func (s *PurchaseJournalServiceSSOT) getInventoryAccount(db *gorm.DB) (*models.Account, error) {
	// 1. Try TaxAccountHelper (Configured)
	acc, err := s.taxAccountHelper.GetInventoryAccount(db)
	if err == nil {
		return acc, nil
	}

	// 2. Try Standard Template Code (1103)
	var account models.Account
	if err := db.Where("code LIKE ? AND is_active = ?", "1103%", true).First(&account).Error; err == nil {
		return &account, nil
	}

	// 3. Try Legacy Code (1301)
	if err := db.Where("code = ? AND is_active = ?", "1103-001", true).First(&account).Error; err == nil {
		return &account, nil
	}

	return nil, fmt.Errorf("inventory account not found (tried configured, 1103*, 1301)")
}
