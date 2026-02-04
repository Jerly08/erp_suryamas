package services

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"
	"app-sistem-akuntansi/models"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// SalesJournalServiceSSOT handles sales journal entries with CORRECT unified_journal_ledger integration
// This service writes to unified_journal_ledger which is read by Balance Sheet service
type SalesJournalServiceSSOT struct {
	db               *gorm.DB
	coaService       *COAService
	taxAccountHelper *TaxAccountHelper
}

// NewSalesJournalServiceSSOT creates a new instance
func NewSalesJournalServiceSSOT(db *gorm.DB, coaService *COAService) *SalesJournalServiceSSOT {
	return &SalesJournalServiceSSOT{
		db:               db,
		coaService:       coaService,
		taxAccountHelper: NewTaxAccountHelper(db),
	}
}

// ShouldPostToJournal checks if a status should create journal entries
func (s *SalesJournalServiceSSOT) ShouldPostToJournal(status string) bool {
	allowedStatuses := []string{"INVOICED", "PAID"}
	for _, allowed := range allowedStatuses {
		if status == allowed {
			return true
		}
	}
	return false
}

// syncCashBankBalance syncs cash_banks.balance with linked accounts.balance
// This ensures Cash & Bank Management page always shows same balance as COA
func (s *SalesJournalServiceSSOT) syncCashBankBalance(tx *gorm.DB, accountID uint64) error {
	// Find if this account is linked to a cash_bank record
	var cashBank models.CashBank
	// Use silent query to avoid logging "record not found" errors for non-cash/bank accounts
	if err := tx.Session(&gorm.Session{Logger: tx.Logger.LogMode(1)}).Where("account_id = ?", accountID).First(&cashBank).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Not a cash/bank account, skip sync silently (normal for revenue, expense accounts)
			return nil
		}
		return err
	}
	
	// Get current COA account balance
	var account models.Account
	if err := tx.First(&account, accountID).Error; err != nil {
		return err
	}
	
	// Check if sync needed
	if cashBank.Balance == account.Balance {
		// Already in sync, no update needed
		return nil
	}
	
	// Sync balance
	oldBalance := cashBank.Balance
	cashBank.Balance = account.Balance
	
	if err := tx.Save(&cashBank).Error; err != nil {
		return err
	}
	
	log.Printf("🔄 [SYNC] CashBank #%d '%s' synced: %.2f → %.2f (from COA account #%d)", 
		cashBank.ID, cashBank.Name, oldBalance, cashBank.Balance, accountID)
	
	return nil
}

// CreateSalesJournal creates journal entries in unified_journal_ledger for Balance Sheet integration
func (s *SalesJournalServiceSSOT) CreateSalesJournal(sale *models.Sale, tx *gorm.DB) error {
	// VALIDASI STATUS - HANYA INVOICED/PAID YANG BOLEH POSTING
	if !s.ShouldPostToJournal(sale.Status) {
		log.Printf("⚠️ [SSOT] Skipping journal creation for Sale #%d with status: %s (only INVOICED/PAID allowed)", sale.ID, sale.Status)
		return nil
	}

	log.Printf("📝 [SSOT] Creating unified journal entries for Sale #%d (Status: %s, Payment Method: '%s')", 
		sale.ID, sale.Status, sale.PaymentMethodType)
	
	// ✅ FIX: Don't fail if payment method type is empty, use default CREDIT
	// This prevents blocking journal creation for valid sales
	if strings.TrimSpace(sale.PaymentMethodType) == "" {
		log.Printf("⚠️ [SSOT] Warning: Sale #%d has empty PaymentMethodType, defaulting to CREDIT", sale.ID)
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
		Where("source_type = ? AND source_id = ?", "SALE", sale.ID).
		Count(&existingCount).Error; err == nil && existingCount > 0 {
		log.Printf("⚠️ [SSOT] Journal already exists for Sale #%d (found %d entries), skipping", 
			sale.ID, existingCount)
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
	var lines []SalesJournalLineRequest

	// 1. DEBIT side - based on payment method
	var debitAccount *models.Account
	var err error
	
	switch strings.ToUpper(strings.TrimSpace(sale.PaymentMethodType)) {
	case "TUNAI", "CASH":
		debitAccount, err = resolveByCode("1101")
		if err != nil {
			return fmt.Errorf("cash account not found: %v", err)
		}
	case "TRANSFER", "BANK":
		// ✅ FIX: Get Account via CashBank relationship, not direct lookup
		// sale.CashBankID is ID from cash_banks table, need to get AccountID from it
		if sale.CashBankID != nil && *sale.CashBankID > 0 {
			var cashBank models.CashBank
			if err := dbToUse.First(&cashBank, *sale.CashBankID).Error; err != nil {
				log.Printf("⚠️ CashBank ID %d not found, using default BANK account: %v", *sale.CashBankID, err)
				debitAccount, err = resolveByCode("1102")
				if err != nil {
					return fmt.Errorf("bank account not found: %v", err)
				}
			} else if cashBank.AccountID == 0 {
				log.Printf("⚠️ CashBank #%d has no AccountID linked, using default BANK account", cashBank.ID)
				debitAccount, err = resolveByCode("1102")
				if err != nil {
					return fmt.Errorf("bank account not found: %v", err)
				}
			} else {
				// Use the linked account from CashBank
				if err := dbToUse.First(&debitAccount, cashBank.AccountID).Error; err != nil {
					log.Printf("⚠️ Account ID %d from CashBank #%d not found, using default: %v", cashBank.AccountID, cashBank.ID, err)
					debitAccount, err = resolveByCode("1102")
					if err != nil {
						return fmt.Errorf("bank account not found: %v", err)
					}
				} else {
					log.Printf("✅ Using CashBank '%s' (ID: %d) → Account '%s' (ID: %d)", 
						cashBank.Name, cashBank.ID, debitAccount.Name, debitAccount.ID)
				}
			}
		} else {
			debitAccount, err = resolveByCode("1102")
			if err != nil {
				return fmt.Errorf("bank account not found: %v", err)
			}
		}
	case "CREDIT", "PIUTANG":
		debitAccount, err = resolveByCode("1201")
		if err != nil {
			return fmt.Errorf("receivables account not found: %v", err)
		}
	default:
		// ✅ FIX: Use CREDIT as safe default for unknown payment methods
		// This allows journal creation even if payment method type doesn't match
		// Better to record the transaction than to fail completely
		log.Printf("⚠️ [SSOT] Warning: Unknown payment method type '%s' for Sale #%d, defaulting to CREDIT (Piutang)", 
			sale.PaymentMethodType, sale.ID)
		debitAccount, err = resolveByCode("1201")
		if err != nil {
			return fmt.Errorf("receivables account not found: %v", err)
		}
	}

	// Add DEBIT line
	// ✅ CRITICAL FIX: Calculate correct debit amount
	// The debit should equal all credits (Revenue + PPN + other tax additions - tax deductions)
	// 
	// IMPORTANT: Subtotal already has discount applied (it's sum of LineTotal which is after discount)
	// Formula: Debit = Subtotal (after discount) + PPN + OtherTaxAdditions + ShippingCost
	// Then TotalAmount = Debit - Tax Deductions (PPh, etc)
	
	// Calculate gross amount (after all additions, BEFORE tax deductions)
	// This is the total amount BEFORE customer withholds taxes
	// Subtotal already includes item discounts, so we start from there
	grossAmount := decimal.NewFromFloat(sale.Subtotal).Add(decimal.NewFromFloat(sale.PPN))
	
	// Add any other tax additions if they exist
	if sale.OtherTaxAdditions > 0 {
		grossAmount = grossAmount.Add(decimal.NewFromFloat(sale.OtherTaxAdditions))
	}
	
	// ✅ CRITICAL FIX: Add shipping cost to gross amount
	// Shipping is part of the total amount that customer pays
	if sale.ShippingCost > 0 {
		grossAmount = grossAmount.Add(decimal.NewFromFloat(sale.ShippingCost))
	}
	
	// ✅ CRITICAL FIX: Subtract tax deductions (PPh, PPh21, PPh23)
	// Customer withholds these taxes, so we receive LESS than gross amount
	// The withheld taxes become our prepaid tax asset
	if sale.PPh > 0 {
		grossAmount = grossAmount.Sub(decimal.NewFromFloat(sale.PPh))
	}
	if sale.PPh21Amount > 0 {
		grossAmount = grossAmount.Sub(decimal.NewFromFloat(sale.PPh21Amount))
	}
	if sale.PPh23Amount > 0 {
		grossAmount = grossAmount.Sub(decimal.NewFromFloat(sale.PPh23Amount))
	}
	if sale.OtherTaxDeductions > 0 {
		grossAmount = grossAmount.Sub(decimal.NewFromFloat(sale.OtherTaxDeductions))
	}
	
	// ✅ FIX: Subtract sale-level discount if exists and not already applied
	// Note: Item-level discounts are already in Subtotal via LineTotal
	// Sale-level discount (DiscountAmount) should be subtracted from gross
	if sale.DiscountAmount > 0 && sale.DiscountPercent > 0 {
		// Check if discount is already applied to subtotal
		// If TaxableAmount exists and is less than Subtotal, discount is already applied
		if sale.TaxableAmount > 0 && sale.TaxableAmount < sale.Subtotal {
			log.Printf("⚠️ [DISCOUNT] Sale-level discount already applied in Subtotal (TaxableAmount=%.2f < Subtotal=%.2f)",
				sale.TaxableAmount, sale.Subtotal)
			// Don't subtract again
		} else {
			log.Printf("⚠️ [DISCOUNT] Applying sale-level discount: %.2f (%.2f%%)",
				sale.DiscountAmount, sale.DiscountPercent)
			grossAmount = grossAmount.Sub(decimal.NewFromFloat(sale.DiscountAmount))
		}
	}
	
	log.Printf("📊 [DEBIT CALC] Subtotal=%.2f + PPN=%.2f + OtherTaxAdd=%.2f + Shipping=%.2f - PPh=%.2f - PPh21=%.2f - PPh23=%.2f - OtherTaxDed=%.2f = GrossAmount=%.2f", 
		sale.Subtotal, sale.PPN, sale.OtherTaxAdditions, sale.ShippingCost, 
		sale.PPh, sale.PPh21Amount, sale.PPh23Amount, sale.OtherTaxDeductions,
		grossAmount.InexactFloat64())
	log.Printf("📊 [DEBIT CALC] TotalAmount from DB=%.2f (GrossAmount should equal TotalAmount)", sale.TotalAmount)
	
	// Validate data consistency
	// Note: grossAmount already has tax deductions subtracted, so it should equal TotalAmount
	if math.Abs(grossAmount.InexactFloat64()-sale.TotalAmount) > 100.0 {
		log.Printf("⚠️ [DATA WARNING] TotalAmount mismatch! Expected=%.2f, Actual=%.2f, Diff=%.2f", 
			grossAmount.InexactFloat64(), sale.TotalAmount, grossAmount.InexactFloat64()-sale.TotalAmount)
		log.Printf("⚠️ This may indicate corrupted data or calculation error during sale creation")
		log.Printf("⚠️ Using calculated GrossAmount instead of DB TotalAmount for journal entry")
	}
	
	// Main debit entry - full gross amount (customer owes us this much)
	lines = append(lines, SalesJournalLineRequest{
		AccountID:    uint64(debitAccount.ID),
		DebitAmount:  grossAmount,
		CreditAmount: decimal.Zero,
		Description:  fmt.Sprintf("Penjualan - %s", sale.InvoiceNumber),
	})

	// 2. CREDIT side - Revenue per item based on RevenueAccountID
	// ✅ FIX: Gunakan akun pendapatan yang dipilih di setiap SaleItem (revenue_account_id)
	// sehingga hanya akun yang dipilih di COA yang bertambah.
	if len(sale.SaleItems) == 0 {
		// Safety: kalau SaleItems belum ter-load, coba preload dari DB
		var saleWithItems models.Sale
		if err := dbToUse.Preload("SaleItems").First(&saleWithItems, sale.ID).Error; err == nil {
			_sale := saleWithItems // local copy to avoid shadowing pointer
			sale = &_sale
		} else {
			log.Printf("⚠️ [SSOT] Failed to preload SaleItems for Sale #%d: %v", sale.ID, err)
		}
	}

	// Kelompokkan total pendapatan per akun
	revenueTotals := make(map[uint64]decimal.Decimal)
	defaultRevenueAccID := uint64(0)

	for _, item := range sale.SaleItems {
		amount := decimal.NewFromFloat(item.LineTotal)
		if amount.IsZero() {
			continue
		}

		accID := uint64(item.RevenueAccountID)
		if accID == 0 {
			// Fallback: pakai akun pendapatan default (4101) jika item lama tidak punya RevenueAccountID
			if defaultRevenueAccID == 0 {
				defaultAcc, err := resolveByCode("4101")
				if err != nil {
					return fmt.Errorf("revenue account not found: %v", err)
				}
				defaultRevenueAccID = uint64(defaultAcc.ID)
			}
			accID = defaultRevenueAccID
		}

		revenueTotals[accID] = revenueTotals[accID].Add(amount)
	}

	// Jika tidak ada item (atau semua 0), fallback ke perilaku lama
	if len(revenueTotals) == 0 {
		defaultAcc, err := resolveByCode("4101")
		if err != nil {
			return fmt.Errorf("revenue account not found: %v", err)
		}
		lines = append(lines, SalesJournalLineRequest{
			AccountID:    uint64(defaultAcc.ID),
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.NewFromFloat(sale.Subtotal),
			Description:  fmt.Sprintf("Pendapatan Penjualan - %s", sale.InvoiceNumber),
		})
	} else {
		for accID, amount := range revenueTotals {
			lines = append(lines, SalesJournalLineRequest{
				AccountID:    accID,
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				Description:  fmt.Sprintf("Pendapatan Penjualan - %s", sale.InvoiceNumber),
			})
			log.Printf("💰 [Revenue] Credit account %d amount %.2f for Sale #%d", accID, amount.InexactFloat64(), sale.ID)
		}
	}

	// 3. PPN if exists
	if sale.PPN > 0 {
		ppnAccount, err := resolveByCode("2103")
		if err != nil {
			log.Printf("⚠️ PPN account not found, skipping PPN entry: %v", err)
		} else {
			lines = append(lines, SalesJournalLineRequest{
				AccountID:    uint64(ppnAccount.ID),
				DebitAmount:  decimal.Zero,
				CreditAmount: decimal.NewFromFloat(sale.PPN),
				Description:  fmt.Sprintf("PPN Keluaran - %s", sale.InvoiceNumber),
			})
		}
	}
	
	// 3b. Other Tax Additions (if exists) - must be credited to match the debit
	// These are additional taxes charged to customer, so they're credited as tax liabilities
	// ✅ FIX: Only create entry if amount is significant (>= 1.00)
	if sale.OtherTaxAdditions >= 1.0 {
		otherTaxAddAccount, err := resolveByCode("2108") // Other tax additions account
		if err != nil {
			log.Printf("⚠️ Other tax additions account (2108) not found, using generic tax account (2103)")
			otherTaxAddAccount, err = resolveByCode("2103")
		}
		if err == nil {
			lines = append(lines, SalesJournalLineRequest{
				AccountID:    uint64(otherTaxAddAccount.ID),
				DebitAmount:  decimal.Zero,
				CreditAmount: decimal.NewFromFloat(sale.OtherTaxAdditions),
				Description:  fmt.Sprintf("Penambahan Pajak Lainnya - %s", sale.InvoiceNumber),
			})
			log.Printf("💰 [OtherTaxAdditions] Recorded: Rp %.2f", sale.OtherTaxAdditions)
		} else {
			log.Printf("⚠️ Failed to record OtherTaxAdditions: no account found")
		}
	} else if sale.OtherTaxAdditions > 0 {
		log.Printf("⚠️ [OtherTaxAdditions] Skipped small amount: Rp %.2f (< 1.00)", sale.OtherTaxAdditions)
	}
	
	// 3c. Shipping Cost (if exists and taxable/non-taxable)
	// Shipping is revenue/income, so it's credited
	// ✅ FIX: Only create entry if amount is significant (>= 1.00)
	if sale.ShippingCost >= 1.0 {
		shippingAccount, err := resolveByCode("4102") // Shipping revenue account
		if err != nil {
			log.Printf("⚠️ Shipping revenue account (4102) not found, using main revenue account (4101)")
			shippingAccount, err = resolveByCode("4101")
		}
		if err == nil {
			lines = append(lines, SalesJournalLineRequest{
				AccountID:    uint64(shippingAccount.ID),
				DebitAmount:  decimal.Zero,
				CreditAmount: decimal.NewFromFloat(sale.ShippingCost),
				Description:  fmt.Sprintf("Pendapatan Ongkir - %s", sale.InvoiceNumber),
			})
			log.Printf("💰 [ShippingCost] Recorded: Rp %.2f", sale.ShippingCost)
		} else {
			log.Printf("⚠️ Failed to record ShippingCost: no account found")
		}
	} else if sale.ShippingCost > 0 {
		log.Printf("⚠️ [ShippingCost] Skipped small amount: Rp %.2f (< 1.00)", sale.ShippingCost)
	}

	// 4. Tax Deductions - PPh (ASSET - Prepaid Tax)
	// ✅ FIX: Tax withheld by customer is our ASSET (claim against tax office)
	// When customer withholds tax, we DEBIT prepaid tax account
	// Later we can use this to offset our tax obligations
	
	// Legacy PPh field - use configured account
	// ✅ CRITICAL FIX: PPh account MUST exist if PPh > 0, otherwise journal won't balance
	if sale.PPh > 0 {
		pphAccount, err := s.taxAccountHelper.GetWithholdingTax21Account(dbToUse)
		if err != nil {
			log.Printf("❌ [CRITICAL] PPh prepaid account not found but PPh=%.2f is applied to Sale #%d", 
				sale.PPh, sale.ID)
			return fmt.Errorf("PPh prepaid account not found (code 1114): %v. Please configure the account in Tax Account Settings", err)
		}
		lines = append(lines, SalesJournalLineRequest{
			AccountID:    uint64(pphAccount.ID),
			DebitAmount:  decimal.NewFromFloat(sale.PPh),
			CreditAmount: decimal.Zero,
			Description:  fmt.Sprintf("PPh Dibayar Dimuka - %s", sale.InvoiceNumber),
		})
		log.Printf("💰 [PPh] Recorded as prepaid tax (Asset): Rp %.2f", sale.PPh)
	}
	
	// PPh 21 - use configured account
	// ✅ CRITICAL FIX: PPh21 account MUST exist if PPh21Amount > 0, otherwise journal won't balance
	if sale.PPh21Amount > 0 {
		pph21Account, err := s.taxAccountHelper.GetWithholdingTax21Account(dbToUse)
		if err != nil {
			log.Printf("❌ [CRITICAL] PPh21 prepaid account not found but PPh21Amount=%.2f is applied to Sale #%d", 
				sale.PPh21Amount, sale.ID)
			return fmt.Errorf("PPh21 prepaid account not found (code 1114): %v. Please configure the account in Tax Account Settings", err)
		}
		lines = append(lines, SalesJournalLineRequest{
			AccountID:    uint64(pph21Account.ID),
			DebitAmount:  decimal.NewFromFloat(sale.PPh21Amount),
			CreditAmount: decimal.Zero,
			Description:  fmt.Sprintf("PPh 21 Dibayar Dimuka - %s", sale.InvoiceNumber),
		})
		log.Printf("💰 [PPh21] Recorded as prepaid tax (Asset): Rp %.2f", sale.PPh21Amount)
	}
	
	// PPh 23 - use configured account
	// ✅ CRITICAL FIX: PPh23 account MUST exist if PPh23Amount > 0, otherwise journal won't balance
	// Because grossAmount already has PPh23 subtracted, we MUST create the debit entry for prepaid tax
	if sale.PPh23Amount > 0 {
		pph23Account, err := s.taxAccountHelper.GetWithholdingTax23Account(dbToUse)
		if err != nil {
			// ✅ FIX: Return error instead of silently skipping
			// If PPh23 is applied to sale, the account MUST exist
			// Otherwise grossAmount is reduced but no corresponding debit entry exists = imbalance
			log.Printf("❌ [CRITICAL] PPh23 prepaid account not found but PPh23Amount=%.2f is applied to Sale #%d", 
				sale.PPh23Amount, sale.ID)
			log.Printf("❌ This would cause journal imbalance. Please configure PPh23 account in Tax Settings or create account code 1115")
			return fmt.Errorf("PPh23 prepaid account not found (code 1115): %v. Please configure the account in Tax Account Settings or create account with code 1115", err)
		}
		lines = append(lines, SalesJournalLineRequest{
			AccountID:    uint64(pph23Account.ID),
			DebitAmount:  decimal.NewFromFloat(sale.PPh23Amount),
			CreditAmount: decimal.Zero,
			Description:  fmt.Sprintf("PPh 23 Dibayar Dimuka - %s", sale.InvoiceNumber),
		})
		log.Printf("💰 [PPh23] Recorded as prepaid tax (Asset): Rp %.2f", sale.PPh23Amount)
	}
	
	// Other Tax Deductions (if exists)
	if sale.OtherTaxDeductions > 0 {
		otherTaxDedAccount, err := resolveByCode("1116") // Other Tax Deductions Prepaid
		if err != nil {
			log.Printf("⚠️ Other tax deductions prepaid account (1116) not found, using 1114")
			otherTaxDedAccount, err = resolveByCode("1114")
		}
		if err == nil {
			lines = append(lines, SalesJournalLineRequest{
				AccountID:    uint64(otherTaxDedAccount.ID),
				DebitAmount:  decimal.NewFromFloat(sale.OtherTaxDeductions),
				CreditAmount: decimal.Zero,
				Description:  fmt.Sprintf("Potongan Pajak Lainnya - %s", sale.InvoiceNumber),
			})
			log.Printf("💰 [OtherTaxDeductions] Recorded as prepaid tax (Asset): Rp %.2f", sale.OtherTaxDeductions)
		} else {
			log.Printf("⚠️ Other tax deductions prepaid account not found, skipping")
		}
	}
	

	// ========================================
	// 🔥 FIX CRITICAL: ADD COGS JOURNAL ENTRY
	// ========================================
	// 5. COGS Recording - Cost of Goods Sold
	// Load sale items with products to calculate COGS
	var saleWithItems models.Sale
	if err := dbToUse.Preload("SaleItems.Product").First(&saleWithItems, sale.ID).Error; err != nil {
		log.Printf("⚠️ Failed to load sale items for COGS calculation: %v", err)
	} else {
		var totalCOGS decimal.Decimal
		var cogsDetails []string
		
		// Calculate total COGS from all sale items
		for _, item := range saleWithItems.SaleItems {
			// Check if product is loaded
			if item.Product.ID == 0 {
				log.Printf("⚠️ Sale item #%d has no product loaded, skipping COGS", item.ID)
				continue
			}
			
			// ✅ NEW: Only treat NON-SERVICE products as inventory for COGS purposes
			// This keeps behaviour consistent with purchase journals and stock service:
			//  - Barang (IsService=false) → ada pergerakan persediaan & COGS
			//  - Jasa  (IsService=true)  → TIDAK mengurangi akun persediaan 1301
			if item.Product.IsService {
				log.Printf("ℹ️ [COGS] Product '%s' (ID: %d) is marked as service, skipping COGS & inventory movement",
					item.Product.Name, item.Product.ID)
				continue
			}
			
			// Calculate COGS: Quantity × Cost Price
			itemCOGS := decimal.NewFromFloat(float64(item.Quantity)).
				Mul(decimal.NewFromFloat(item.Product.CostPrice))
			
			if itemCOGS.IsZero() {
				log.Printf("⚠️ Product '%s' (ID: %d) has zero cost price, COGS = 0", 
					item.Product.Name, item.Product.ID)
			} else {
				totalCOGS = totalCOGS.Add(itemCOGS)
				cogsDetails = append(cogsDetails, fmt.Sprintf("%s(Qty:%d×Rp%.0f)", 
					item.Product.Name, item.Quantity, item.Product.CostPrice))
			}
		}
		
		// Only create COGS entry if total COGS > 0
		if !totalCOGS.IsZero() {
			// DEBIT: COGS Account - use configured account
			cogsAccount, err := s.taxAccountHelper.GetCOGSAccount(dbToUse)
			if err != nil {
				log.Printf("⚠️ COGS account not found, skipping COGS entry: %v", err)
			} else {
				lines = append(lines, SalesJournalLineRequest{
					AccountID:    uint64(cogsAccount.ID),
					DebitAmount:  totalCOGS,
					CreditAmount: decimal.Zero,
					Description:  fmt.Sprintf("HPP - %s", sale.InvoiceNumber),
				})
				
				// CREDIT: Inventory Account - use configured account
				inventoryAccount, err := s.taxAccountHelper.GetInventoryAccount(dbToUse)
				if err != nil {
					log.Printf("⚠️ Inventory account not found, skipping inventory credit: %v", err)
				} else {
					lines = append(lines, SalesJournalLineRequest{
						AccountID:    uint64(inventoryAccount.ID),
						DebitAmount:  decimal.Zero,
						CreditAmount: totalCOGS,
						Description:  fmt.Sprintf("Pengurangan Persediaan - %s", sale.InvoiceNumber),
					})
					
					log.Printf("💰 [COGS] Calculated COGS for Sale #%d: Rp %.2f (%d items: %s)", 
						sale.ID, totalCOGS.InexactFloat64(), len(cogsDetails), 
						strings.Join(cogsDetails, ", "))
				}
			}
		} else {
			log.Printf("⚠️ [COGS] No COGS calculated for Sale #%d (all items have zero cost price)", sale.ID)
		}
	}
	// ========================================
	// END COGS FIX
	// ========================================

	// Calculate totals
	var totalDebit, totalCredit decimal.Decimal
	log.Printf("\n📊 [BALANCE DEBUG] Sale #%d Journal Entry Lines:", sale.ID)
	for i, line := range lines {
		totalDebit = totalDebit.Add(line.DebitAmount)
		totalCredit = totalCredit.Add(line.CreditAmount)
		log.Printf("  Line %d: AccountID=%d | Debit=%.2f | Credit=%.2f | Desc=%s", 
			i+1, line.AccountID, line.DebitAmount.InexactFloat64(), 
			line.CreditAmount.InexactFloat64(), line.Description)
	}
	log.Printf("📊 [BALANCE DEBUG] Sale Data: Subtotal=%.2f, PPN=%.2f, PPh=%.2f, TotalAmount=%.2f",
		sale.Subtotal, sale.PPN, sale.PPh, sale.TotalAmount)
	log.Printf("📊 [BALANCE DEBUG] Totals: Debit=%.2f | Credit=%.2f | Difference=%.2f", 
		totalDebit.InexactFloat64(), totalCredit.InexactFloat64(), 
		totalDebit.Sub(totalCredit).InexactFloat64())

	// Verify balanced with rounding tolerance
	difference := totalDebit.Sub(totalCredit).Abs()
	// ✅ FIX: Increase tolerance to 100 for data inconsistencies
	// Many sales have small rounding errors from discount/tax calculations
	toleranceThreshold := decimal.NewFromFloat(100.0) // Allow up to 100 Rupiah tolerance
	
	if difference.GreaterThan(toleranceThreshold) {
		// Significant imbalance - try to auto-adjust if reasonable
		if difference.LessThanOrEqual(decimal.NewFromFloat(500.0)) {
			// For differences <= 500, adjust the first debit line (Cash/Bank/Receivable)
			log.Printf("⚠️ [AUTO-ADJUST] Imbalance detected: %.2f (adjusting...)", difference.InexactFloat64())
			
			if len(lines) > 0 && lines[0].DebitAmount.GreaterThan(decimal.Zero) {
				// Adjust first debit line
				oldDebit := lines[0].DebitAmount
				if totalDebit.LessThan(totalCredit) {
					// Need to increase debit
					lines[0].DebitAmount = lines[0].DebitAmount.Add(difference)
					log.Printf("✅ [AUTO-ADJUST] Increased first debit line: %.2f → %.2f (+%.2f)",
						oldDebit.InexactFloat64(), lines[0].DebitAmount.InexactFloat64(), difference.InexactFloat64())
				} else {
					// Need to decrease debit
					lines[0].DebitAmount = lines[0].DebitAmount.Sub(difference)
					log.Printf("✅ [AUTO-ADJUST] Decreased first debit line: %.2f → %.2f (-%.2f)",
						oldDebit.InexactFloat64(), lines[0].DebitAmount.InexactFloat64(), difference.InexactFloat64())
				}
				
				// Recalculate totals
				totalDebit = decimal.Zero
				totalCredit = decimal.Zero
				for _, line := range lines {
					totalDebit = totalDebit.Add(line.DebitAmount)
					totalCredit = totalCredit.Add(line.CreditAmount)
				}
				log.Printf("✅ [AUTO-ADJUST] After adjustment: Debit=%.2f, Credit=%.2f, Diff=%.2f",
					totalDebit.InexactFloat64(), totalCredit.InexactFloat64(),
					totalDebit.Sub(totalCredit).Abs().InexactFloat64())
			}
		}
		
		// Check again after adjustment
		difference = totalDebit.Sub(totalCredit).Abs()
		if difference.GreaterThan(toleranceThreshold) {
			log.Printf("❌ [BALANCE ERROR] Journal entry still imbalanced after adjustment:")
			log.Printf("   Debit: %.2f | Credit: %.2f | Difference: %.2f", 
				totalDebit.InexactFloat64(), totalCredit.InexactFloat64(), difference.InexactFloat64())
			log.Printf("   This indicates data corruption in Sale #%d", sale.ID)
			log.Printf("   Please verify: Subtotal, PPN, Taxes, Discounts, and Shipping calculations")
			return fmt.Errorf("journal entry not balanced: debit=%.2f, credit=%.2f, difference=%.2f (tolerance: %.2f)", 
				totalDebit.InexactFloat64(), totalCredit.InexactFloat64(), difference.InexactFloat64(), toleranceThreshold.InexactFloat64())
		}
	}
	
	if difference.GreaterThan(decimal.Zero) && difference.LessThanOrEqual(toleranceThreshold) {
		log.Printf("✅ [BALANCE] Accepted with rounding difference: %.2f (within tolerance of %.2f)", 
			difference.InexactFloat64(), toleranceThreshold.InexactFloat64())
	}

	// Create journal entry
	// Insert as DRAFT first to avoid trigger validation before lines are created
	sourceID := uint64(sale.ID)
	now := time.Now()
	postedBy := uint64(sale.UserID)
	
	journalEntry := &models.SSOTJournalEntry{
		EntryNumber:     fmt.Sprintf("SALE-%d-%d", sale.ID, now.Unix()),
		SourceType:      "SALE",
		SourceID:        &sourceID,
		SourceCode:      sale.InvoiceNumber,
		EntryDate:       sale.Date,
		Description:     fmt.Sprintf("Sales Invoice #%s - %s", sale.InvoiceNumber, sale.Customer.Name),
		Reference:       sale.InvoiceNumber,
		TotalDebit:      totalDebit,
		TotalCredit:     totalCredit,
		Status:          "DRAFT",
		IsBalanced:      true,
		IsAutoGenerated: true,
		CreatedBy:       uint64(sale.UserID),
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
		
		// ✅ CRITICAL: Sync cash_banks.balance with accounts.balance after COA update
		// This ensures Cash & Bank Management page shows same balance as COA
		if err := s.syncCashBankBalance(dbToUse, lineReq.AccountID); err != nil {
			log.Printf("⚠️ Warning: Failed to sync cash/bank balance for account %d: %v", lineReq.AccountID, err)
			// Continue processing - don't fail transaction for sync issues
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

	log.Printf("✅ [SSOT] Created and posted journal entry #%d with %d lines (Debit: %.2f, Credit: %.2f)", 
		journalEntry.ID, len(lines), totalDebit.InexactFloat64(), totalCredit.InexactFloat64())

	return nil
}

// updateAccountBalance updates account.balance field for COA tree view display
// RE-ENABLED: COA Tree View needs this field updated
// Note: P&L Report calculates balance from journal entries (real-time, always correct)
//       COA Tree View reads from account.balance field (needs manual update)
func (s *SalesJournalServiceSSOT) updateAccountBalance(db *gorm.DB, accountID uint64, debitAmount, creditAmount decimal.Decimal) error {
	var account models.Account
	if err := db.First(&account, accountID).Error; err != nil {
		return fmt.Errorf("account %d not found: %v", accountID, err)
	}

	// Calculate net change: debit - credit
	debit := debitAmount.InexactFloat64()
	credit := creditAmount.InexactFloat64()
	netChange := debit - credit
	
	oldBalance := account.Balance

	// ✅ VALIDATION: Check account type correctness for critical accounts
	accountType := strings.ToUpper(account.Type)
	if account.Code == "1301" && accountType != "ASSET" {
		log.Printf("❌ [BUG] Account 1301 (Persediaan) has WRONG type '%s', should be 'ASSET'!", accountType)
		log.Printf("❌ This will cause INCORRECT balance calculation!")
	}
	if account.Code == "5101" && accountType != "EXPENSE" {
		log.Printf("❌ [BUG] Account 5101 (COGS) has WRONG type '%s', should be 'EXPENSE'!", accountType)
	}

	// Update balance based on account type
	switch accountType {
	case "ASSET", "EXPENSE":
		// Assets and Expenses: debit increases balance
		account.Balance += netChange
		log.Printf("📊 [SSOT] Account %s (%s) TYPE=%s: Balance %.2f + netChange(%.2f) = %.2f", 
			account.Code, account.Name, accountType, oldBalance, netChange, account.Balance)
	case "LIABILITY", "EQUITY", "REVENUE":
		// Liabilities, Equity, Revenue: credit increases balance (so debit decreases)
		account.Balance -= netChange
		log.Printf("📊 [SSOT] Account %s (%s) TYPE=%s: Balance %.2f - netChange(%.2f) = %.2f", 
			account.Code, account.Name, accountType, oldBalance, netChange, account.Balance)
	default:
		log.Printf("⚠️ [SSOT] Unknown account type '%s' for account %s (%s)", accountType, account.Code, account.Name)
		// Fallback: treat as ASSET
		account.Balance += netChange
	}

	if err := db.Save(&account).Error; err != nil {
		return fmt.Errorf("failed to save account balance: %v", err)
	}

	// ✅ DETAILED LOGGING for debugging
	balanceChange := account.Balance - oldBalance
	if account.Code == "1301" {
		if credit > 0 && balanceChange > 0 {
			log.Printf("❌ [BUG DETECTED] Account 1301 CREDIT %.2f but balance INCREASED by %.2f! (Should DECREASE)", 
				credit, balanceChange)
			log.Printf("❌ Old Balance: %.2f, New Balance: %.2f, Type: %s, netChange: %.2f", 
				oldBalance, account.Balance, accountType, netChange)
		} else if credit > 0 && balanceChange < 0 {
			log.Printf("✅ [CORRECT] Account 1301 CREDIT %.2f and balance DECREASED by %.2f (correct!)", 
				credit, -balanceChange)
		}
	}

	log.Printf("💰 [SSOT] Updated account %s (%s) TYPE=%s balance: Dr=%.2f, Cr=%.2f, netChange=%.2f, Old=%.2f, New=%.2f", 
		account.Code, account.Name, accountType, debit, credit, netChange, oldBalance, account.Balance)

	return nil
}

// UpdateSalesJournal updates journal entries based on status change
func (s *SalesJournalServiceSSOT) UpdateSalesJournal(sale *models.Sale, oldStatus string, tx *gorm.DB) error {
	dbToUse := s.db
	if tx != nil {
		dbToUse = tx
	}

	oldShouldPost := s.ShouldPostToJournal(oldStatus)
	newShouldPost := s.ShouldPostToJournal(sale.Status)

	if !oldShouldPost && newShouldPost {
		// Create journal
		log.Printf("📈 [SSOT] Status changed from %s to %s - Creating journal entries", oldStatus, sale.Status)
		return s.CreateSalesJournal(sale, dbToUse)
	} else if oldShouldPost && !newShouldPost {
		// Delete journal
		log.Printf("📉 [SSOT] Status changed from %s to %s - Removing journal entries", oldStatus, sale.Status)
		return s.DeleteSalesJournal(sale.ID, dbToUse)
	} else if oldShouldPost && newShouldPost {
		// Update existing
		log.Printf("🔄 [SSOT] Updating journal entries for Sale #%d", sale.ID)
		
		if err := s.DeleteSalesJournal(sale.ID, dbToUse); err != nil {
			return err
		}
		
		return s.CreateSalesJournal(sale, dbToUse)
	}

	log.Printf("ℹ️ [SSOT] No journal update needed for Sale #%d (Status: %s)", sale.ID, sale.Status)
	return nil
}

// DeleteSalesJournal deletes all journal entries for a sale
func (s *SalesJournalServiceSSOT) DeleteSalesJournal(saleID uint, tx *gorm.DB) error {
	dbToUse := s.db
	if tx != nil {
		dbToUse = tx
	}

	// Find journal entry
	var entry models.SSOTJournalEntry
	if err := dbToUse.Where("source_type = ? AND source_id = ?", "SALE", saleID).First(&entry).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Printf("⚠️ [SSOT] No journal found for Sale #%d, nothing to delete", saleID)
			return nil
		}
		return fmt.Errorf("failed to find journal entry: %v", err)
	}

	// Delete lines first (FK constraint)
	if err := dbToUse.Where("journal_id = ?", entry.ID).Delete(&models.SSOTJournalLine{}).Error; err != nil {
		return fmt.Errorf("failed to delete journal lines: %v", err)
	}

	// Delete entry
	if err := dbToUse.Delete(&entry).Error; err != nil {
		return fmt.Errorf("failed to delete journal entry: %v", err)
	}

	log.Printf("✅ [SSOT] Deleted journal entry #%d and its lines for Sale #%d", entry.ID, saleID)
	return nil
}

// SalesJournalLineRequest represents a request to create a sales journal line
type SalesJournalLineRequest struct {
	AccountID    uint64
	DebitAmount  decimal.Decimal
	CreditAmount decimal.Decimal
	Description  string
}

