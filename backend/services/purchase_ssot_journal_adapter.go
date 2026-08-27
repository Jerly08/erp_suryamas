package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// PurchaseSSOTJournalAdapter integrates Purchases with the SSOT Journal System
// This replaces the previous stub and enables real retrieval/creation of purchase-related journal entries.
type PurchaseSSOTJournalAdapter struct {
	db                    *gorm.DB
	unifiedJournalService *UnifiedJournalService
	accountRepo           repositories.AccountRepository
	taxService            *TaxAccountService
}

// NewPurchaseSSOTJournalAdapter creates an adapter instance wired to UnifiedJournalService
func NewPurchaseSSOTJournalAdapter(
	db *gorm.DB,
	unifiedJournalService *UnifiedJournalService,
	accountRepo repositories.AccountRepository,
	taxService *TaxAccountService,
) *PurchaseSSOTJournalAdapter {
	return &PurchaseSSOTJournalAdapter{
		db:                    db,
		unifiedJournalService: unifiedJournalService,
		accountRepo:           accountRepo,
		taxService:            taxService,
	}
}

// CreatePurchaseJournalEntry creates an SSOT journal entry for a purchase transaction
func (adapter *PurchaseSSOTJournalAdapter) CreatePurchaseJournalEntry(
	ctx context.Context,
	purchase *models.Purchase,
	userID uint64,
) (*models.SSOTJournalEntry, error) {
	if isComponentPurchaseVAT(purchase) {
		return nil, fmt.Errorf("adapter jurnal lama tidak mendukung PPN per komponen; gunakan PurchaseJournalServiceSSOT")
	}
	if adapter.unifiedJournalService == nil {
		return nil, fmt.Errorf("unified journal service not available")
	}

	// Resolve required accounts
	accountIDs, err := adapter.getPurchaseAccountIDs()
	if err != nil {
		return nil, err
	}

	// Build request
	req := &JournalEntryRequest{
		SourceType: models.SSOTSourceTypePurchase,
		SourceID:   uint64(purchase.ID),
		Reference:  purchase.Code,
		EntryDate:  purchase.Date,
		Description: fmt.Sprintf(
			"Purchase Order %s - %s",
			purchase.Code, purchase.Vendor.Name,
		),
		Lines:     adapter.buildPurchaseJournalLines(purchase, accountIDs),
		AutoPost:  true,
		CreatedBy: userID,
	}

	entry, err := adapter.unifiedJournalService.CreateJournalEntry(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSOT journal entry: %v", err)
	}
	return entry, nil
}

// GetPurchaseJournalEntries returns all SSOT journal entries for a purchase (only source_type=PURCHASE)
func (adapter *PurchaseSSOTJournalAdapter) GetPurchaseJournalEntries(
	ctx context.Context,
	purchaseID uint64,
) ([]models.SSOTJournalEntry, error) {
	if adapter.unifiedJournalService == nil {
		return nil, fmt.Errorf("unified journal service not available")
	}

	filters := JournalFilters{
		SourceType: models.SSOTSourceTypePurchase,
		SourceID:   &purchaseID,
		Page:       1,
		Limit:      100,
	}

	resp, err := adapter.unifiedJournalService.GetJournalEntries(filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get purchase journal entries: %v", err)
	}
	return resp.Data, nil
}

// GetPurchaseRelatedJournalEntries returns journals for PURCHASE and optionally PAYMENT
// plus any purchase-linked ADJUSTMENT journals such as backorder HPP settlements.
func (adapter *PurchaseSSOTJournalAdapter) GetPurchaseRelatedJournalEntries(
	ctx context.Context,
	purchaseID uint64,
	includePayments bool,
) ([]models.SSOTJournalEntry, error) {
	if adapter.unifiedJournalService == nil {
		return nil, fmt.Errorf("unified journal service not available")
	}

	journalByID := make(map[uint64]models.SSOTJournalEntry)
	collectEntries := func(sourceType string) error {
		filters := JournalFilters{
			SourceType: sourceType,
			SourceID:   &purchaseID,
			Page:       1,
			Limit:      200,
		}
		resp, err := adapter.unifiedJournalService.GetJournalEntries(filters)
		if err != nil {
			return err
		}
		for _, entry := range resp.Data {
			journalByID[entry.ID] = entry
		}
		return nil
	}

	// 1) PURCHASE journals
	if err := collectEntries(models.SSOTSourceTypePurchase); err != nil {
		return nil, fmt.Errorf("failed to get purchase journals: %v", err)
	}

	// 2) PAYMENT journals (optional)
	if includePayments {
		if err := collectEntries(models.SSOTSourceTypePayment); err != nil {
			return nil, fmt.Errorf("failed to get purchase payment journals: %v", err)
		}
	}

	// 3) ADJUSTMENT journals tied to the purchase, such as backorder HPP settlements.
	if err := collectEntries(models.SSOTSourceTypeAdjustment); err != nil {
		return nil, fmt.Errorf("failed to get purchase adjustment journals: %v", err)
	}

	// 4) Purchase PPh settlement journals are tied to the purchase ID as source_id.
	if err := collectEntries(models.SSOTSourceTypePurchasePPhPayment); err != nil {
		return nil, fmt.Errorf("failed to get purchase PPh payment journals: %v", err)
	}

	result := make([]models.SSOTJournalEntry, 0, len(journalByID))
	for _, entry := range journalByID {
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].EntryDate.Equal(result[j].EntryDate) {
			return result[i].ID > result[j].ID
		}
		return result[i].EntryDate.After(result[j].EntryDate)
	})

	return result, nil
}

// CreatePurchasePaymentJournalEntry creates SSOT journal entry for purchase payment
func (adapter *PurchaseSSOTJournalAdapter) CreatePurchasePaymentJournalEntry(
	ctx context.Context,
	purchase *models.Purchase,
	paymentAmount decimal.Decimal,
	bankAccountID uint64,
	userID uint64,
	reference string,
	notes string,
) (*models.SSOTJournalEntry, error) {
	if adapter.unifiedJournalService == nil {
		return nil, fmt.Errorf("unified journal service not available")
	}

	// Determine actual cash/bank account ID
	actualBankAccountID, err := adapter.resolveCashBankAccountID(bankAccountID)
	if err != nil {
		return nil, err
	}

	accountIDs, err := adapter.getPurchaseAccountIDs()
	if err != nil {
		return nil, err
	}

	lines := []JournalLineRequest{
		{
			AccountID:    accountIDs.AccountsPayableID,
			Description:  fmt.Sprintf("Payment to %s - %s", purchase.Vendor.Name, reference),
			DebitAmount:  paymentAmount,
			CreditAmount: decimal.Zero,
		},
		{
			AccountID:    actualBankAccountID,
			Description:  fmt.Sprintf("Bank payment - %s", reference),
			DebitAmount:  decimal.Zero,
			CreditAmount: paymentAmount,
		},
	}

	req := &JournalEntryRequest{
		SourceType:  models.SSOTSourceTypePayment,
		SourceID:    uint64(purchase.ID),
		Reference:   reference,
		EntryDate:   time.Now(),
		Description: fmt.Sprintf("Purchase Payment %s - %s (%s)", purchase.Code, purchase.Vendor.Name, paymentAmount.String()),
		Lines:       lines,
		AutoPost:    true,
		CreatedBy:   userID,
	}

	entry, err := adapter.unifiedJournalService.CreateJournalEntry(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create payment journal entry: %v", err)
	}
	return entry, nil
}

// resolveCashBankAccountID resolves the account ID to use for cash/bank side
func (adapter *PurchaseSSOTJournalAdapter) resolveCashBankAccountID(bankAccountID uint64) (uint64, error) {
	if bankAccountID > 0 {
		var cashBank models.CashBank
		if err := adapter.db.Select("account_id").Where("id = ?", bankAccountID).First(&cashBank).Error; err != nil {
			return 0, fmt.Errorf("cash/bank account not found: %v", err)
		}
		if cashBank.AccountID != 0 {
			return uint64(cashBank.AccountID), nil
		}
	}
	// Fallback to default Cash account 1101
	acc, err := adapter.accountRepo.FindByCode(context.TODO(), "1101")
	if err != nil {
		return 0, fmt.Errorf("default Kas account (1101) not found: %v", err)
	}
	return uint64(acc.ID), nil
}

// buildPurchaseJournalLines constructs lines for a purchase entry
func (adapter *PurchaseSSOTJournalAdapter) buildPurchaseJournalLines(
	purchase *models.Purchase,
	accountIDs *SSOTPurchaseAccountIDs,
) []JournalLineRequest {
	var lines []JournalLineRequest

	// Debit inventory/expense/prepaid per item
	for _, item := range purchase.PurchaseItems {
		itemName := "Unknown Item"
		if item.Product != nil {
			itemName = item.Product.Name
		} else if item.Description != "" {
			itemName = item.Description
		}

		accountID := accountIDs.InventoryAccountID
		if item.ItemType == models.PurchaseItemTypePrepaid {
			// Jika PREPAID: Debit harus masuk ke Aset Lancar Biaya Dibayar Dimuka.
			// Memprioritaskan ExpenseAccountID dari user
			if item.ExpenseAccountID != 0 {
				accountID = uint64(item.ExpenseAccountID)
			}
		} else if item.ExpenseAccountID != 0 {
			accountID = uint64(item.ExpenseAccountID)
		}

		amount := decimal.NewFromFloat(purchaseItemLandedTotal(purchase, &item))
		lines = append(lines, JournalLineRequest{
			AccountID:    accountID,
			Description:  fmt.Sprintf("Purchase %s - %s", item.ItemType, itemName),
			DebitAmount:  amount,
			CreditAmount: decimal.Zero,
		})
	}

	// Debit PPN Masukan
	if purchase.PPNAmount > 0 {
		lines = append(lines, JournalLineRequest{
			AccountID:    accountIDs.PPNInputAccountID,
			Description:  "PPN Masukan (Input VAT)",
			DebitAmount:  decimal.NewFromFloat(purchase.PPNAmount),
			CreditAmount: decimal.Zero,
		})
	}

	// TotalAmount is already net of purchase withholdings. PPh payable lines are
	// posted separately below.
	netAmount := purchase.TotalAmount

	// Credit side based on payment method
	if purchase.PaymentMethod == models.PurchasePaymentCash ||
		purchase.PaymentMethod == models.PurchasePaymentTransfer ||
		purchase.PaymentMethod == models.PurchasePaymentCheck {
		// Immediate payment: credit cash/bank
		// Prefer the actual bank account linked to the purchase when available
		var creditAccountID uint64
		if purchase.BankAccountID != nil && *purchase.BankAccountID != 0 {
			if resolvedID, err := adapter.resolveCashBankAccountID(uint64(*purchase.BankAccountID)); err == nil {
				creditAccountID = resolvedID
			} else {
				// Fallback to default Cash account (1101) if resolution fails
				creditAccountID, _ = adapter.resolveCashBankAccountID(0)
			}
		} else {
			// Fallback to default Cash account (1101) when bank account is not provided
			creditAccountID, _ = adapter.resolveCashBankAccountID(0)
		}
		lines = append(lines, JournalLineRequest{
			AccountID:    creditAccountID,
			Description:  fmt.Sprintf("%s Payment - %s", purchase.PaymentMethod, purchase.Vendor.Name),
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.NewFromFloat(netAmount),
		})
	} else {
		// Credit purchase: credit AP (net of withholdings)
		creditAccountID := accountIDs.AccountsPayableID
		if purchase.CreditAccountID != nil && *purchase.CreditAccountID > 0 {
			var selectedAccount models.Account
			if err := adapter.db.Select("id", "type", "is_header").First(&selectedAccount, *purchase.CreditAccountID).Error; err == nil &&
				!selectedAccount.IsHeader &&
				strings.EqualFold(strings.TrimSpace(selectedAccount.Type), models.AccountTypeLiability) {
				creditAccountID = uint64(selectedAccount.ID)
			}
		}
		lines = append(lines, JournalLineRequest{
			AccountID:    creditAccountID,
			Description:  fmt.Sprintf("Accounts Payable - %s", purchase.Vendor.Name),
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.NewFromFloat(netAmount),
		})
	}

	// Optional withholdings
	if purchase.PPh21Amount > 0 && accountIDs.PPh21PayableID != 0 {
		lines = append(lines, JournalLineRequest{
			AccountID:    accountIDs.PPh21PayableID,
			Description:  "PPh 21 Withholding",
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.NewFromFloat(purchase.PPh21Amount),
		})
	}
	if purchase.PPh23Amount > 0 && accountIDs.PPh23PayableID != 0 {
		lines = append(lines, JournalLineRequest{
			AccountID:    accountIDs.PPh23PayableID,
			Description:  "PPh 23 Withholding",
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.NewFromFloat(purchase.PPh23Amount),
		})
	}

	return lines
}

// getPurchaseAccountIDs loads the account IDs required for purchase journal entries
func (adapter *PurchaseSSOTJournalAdapter) getPurchaseAccountIDs() (*SSOTPurchaseAccountIDs, error) {
	ids := &SSOTPurchaseAccountIDs{}

	// Prefer dynamic settings from TaxAccountService when available
	if adapter.taxService != nil {
		if invID, err := adapter.taxService.GetAccountID("inventory"); err == nil && invID != 0 {
			ids.InventoryAccountID = uint64(invID)
			ids.PrimaryAccountID = uint64(invID)
		}
		if ppnInID, err := adapter.taxService.GetAccountID("purchase_input_vat"); err == nil && ppnInID != 0 {
			ids.PPNInputAccountID = uint64(ppnInID)
		}
		if apID, err := adapter.taxService.GetAccountID("purchase_payable"); err == nil && apID != 0 {
			ids.AccountsPayableID = uint64(apID)
		}
		if p21ID, err := adapter.taxService.GetPurchaseTaxAccountID("pph21"); err == nil && p21ID != 0 {
			ids.PPh21PayableID = uint64(p21ID)
		}
		if p23ID, err := adapter.taxService.GetPurchaseTaxAccountID("pph23"); err == nil && p23ID != 0 {
			ids.PPh23PayableID = uint64(p23ID)
		}
	}

	// Fallback by standard codes when settings not available
	if ids.InventoryAccountID == 0 {
		inv, err := adapter.accountRepo.FindByCode(context.TODO(), "1103-001")
		if err != nil {
			return nil, fmt.Errorf("inventory account 1103-001 not found: %v", err)
		}
		ids.InventoryAccountID = uint64(inv.ID)
		ids.PrimaryAccountID = uint64(inv.ID)
	}
	if ids.PPNInputAccountID == 0 {
		ppn, err := adapter.accountRepo.FindByCode(context.TODO(), "1105-005")
		if err != nil {
			ppn, err = adapter.accountRepo.FindByCode(context.TODO(), "1240")
		}
		if err != nil {
			ppn, err = adapter.accountRepo.FindByCode(context.TODO(), "2102-002")
		}
		if err != nil {
			return nil, fmt.Errorf("PPN input account from settings fallback (1105-005/1240/2102-002) not found: %v", err)
		}
		ids.PPNInputAccountID = uint64(ppn.ID)
	}
	if ids.AccountsPayableID == 0 {
		ap, err := adapter.accountRepo.FindByCode(context.TODO(), "2101")
		if err != nil {
			return nil, fmt.Errorf("accounts payable account 2101 not found: %v", err)
		}
		ids.AccountsPayableID = uint64(ap.ID)
	}
	// Optional withholdings fallback
	if ids.PPh21PayableID == 0 {
		if p21, err := adapter.accountRepo.FindByCode(context.TODO(), "2102-003"); err == nil {
			ids.PPh21PayableID = uint64(p21.ID)
		}
	}
	if ids.PPh23PayableID == 0 {
		if p23, err := adapter.accountRepo.FindByCode(context.TODO(), "2102-004"); err == nil {
			ids.PPh23PayableID = uint64(p23.ID)
		}
	}
	return ids, nil
}

// SSOTPurchaseAccountIDs represents required account IDs for purchases
type SSOTPurchaseAccountIDs struct {
	PrimaryAccountID   uint64
	InventoryAccountID uint64 // 1103-001 (from settings or COA template)
	PPNInputAccountID  uint64 // PPN MASUKAN (from settings or COA template)
	AccountsPayableID  uint64 // 2101-001 (from settings or COA template)
	PPh21PayableID     uint64 // 2102-003 PPH PASAL 21 (optional, from settings)
	PPh23PayableID     uint64 // 2102-004 PPH PASAL 23 (optional, from settings)
}
