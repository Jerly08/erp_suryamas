package services

import (
	"app-sistem-akuntansi/models"
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Enhanced Accounting Methods for Purchase Module

// sanitizeFloat ensures value is a valid, finite number within safe DB range
// decimal(15,2) max is 999,999,999,999,999.99 (~1e15). We clamp conservatively.
func sanitizeFloat(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	const limit = 9.0e14 // conservative cap well within decimal(15,2)
	if v > limit {
		return limit
	}
	if v < -limit {
		return -limit
	}
	return v
}

// clampPercent keeps percentage between 0 and 100
func clampPercent(p float64) float64 {
	if math.IsNaN(p) || math.IsInf(p, 0) {
		return 0
	}
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

// clampNonNegative returns max(0, v)
func clampNonNegative(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v < 0 {
		return 0
	}
	return v
}

// setApprovalBasisAndBase determines approval basis for purchase
func (s *PurchaseService) setApprovalBasisAndBase(purchase *models.Purchase) {
	// Set approval basis - what amount will be used for approval
	purchase.ApprovalAmountBasis = "TOTAL_AMOUNT"
	purchase.ApprovalBaseAmount = purchase.TotalAmount

	// Debug logging to track amount discrepancies
	fmt.Printf("🔍 ApprovalBaseAmount Debug: Purchase %d\n", purchase.ID)
	fmt.Printf("   TotalAmount: %.2f\n", purchase.TotalAmount)
	fmt.Printf("   ApprovalBaseAmount: %.2f\n", purchase.ApprovalBaseAmount)

	// NEW LOGIC: All purchases require approval regardless of amount
	// This ensures Employee → Finance → (optional Director) workflow
	purchase.RequiresApproval = true
	fmt.Printf("✅ All purchases require approval - RequiresApproval set to true\n")
}

func validateUniquePurchaseRequestItemIDs(items []models.PurchaseItemRequest) error {
	selectedPurchaseRequestItems := make(map[uint]struct{}, len(items))
	for _, itemReq := range items {
		if itemReq.PurchaseRequestItemID == nil {
			continue
		}

		purchaseRequestItemID := *itemReq.PurchaseRequestItemID
		if purchaseRequestItemID == 0 {
			return fmt.Errorf("purchase_request_item_id must be greater than zero")
		}
		if _, exists := selectedPurchaseRequestItems[purchaseRequestItemID]; exists {
			return fmt.Errorf("purchase request item %d is selected more than once", purchaseRequestItemID)
		}
		selectedPurchaseRequestItems[purchaseRequestItemID] = struct{}{}
	}

	return nil
}

// calculatePurchaseTotals calculates all purchase totals with proper accounting
func (s *PurchaseService) calculatePurchaseTotals(purchase *models.Purchase, items []models.PurchaseItemRequest) error {
	fmt.Printf("ℹ Starting purchase totals calculation for %d items\n", len(items))
	if err := validateUniquePurchaseRequestItemIDs(items); err != nil {
		return err
	}

	subtotalBeforeDiscount := 0.0
	itemDiscountAmount := 0.0
	hasInventoryValuationItem := false

	purchase.PurchaseItems = []models.PurchaseItem{}

	for i, itemReq := range items {
		// Determine item type (default: INVENTORY for backward compatibility)
		itemType := itemReq.ItemType
		if itemType == "" {
			itemType = models.PurchaseItemTypeInventory
		}
		isAssetItem := itemType == models.PurchaseItemTypeAsset || itemReq.IsFixedAsset
		isPrepaidItem := itemType == models.PurchaseItemTypePrepaid
		isExpenseItem := itemType == models.PurchaseItemTypeExpense

		if isPrepaidItem && (itemReq.AmortizationMonth < 1 || itemReq.AmortizationMonth > 120) {
			return fmt.Errorf("item %d: durasi amortisasi prepaid harus antara 1 dan 120 bulan", i+1)
		}

		if isAssetItem || isPrepaidItem || isExpenseItem {
			itemTypeName := string(itemType)
			fmt.Printf("ℹ Processing item %d/%d: %s item '%s', Qty %d, Price %.2f\n", i+1, len(items), itemTypeName, itemReq.Description, itemReq.Quantity, itemReq.UnitPrice)
		} else {
			productID := uint(0)
			if itemReq.ProductID != nil {
				productID = *itemReq.ProductID
			}
			fmt.Printf("ℹ Processing item %d/%d: Product ID %d, Qty %d, Price %.2f\n", i+1, len(items), productID, itemReq.Quantity, itemReq.UnitPrice)
		}

		// Validate product exists only for INVENTORY items
		if !isAssetItem && !isPrepaidItem && !isExpenseItem {
			if itemReq.ProductID == nil || *itemReq.ProductID == 0 {
				return fmt.Errorf("item %d: product_id is required for inventory items", i+1)
			}
			product, err := s.productRepo.FindByID(*itemReq.ProductID)
			if err != nil {
				fmt.Printf("❌ Failed to find product with ID %d: %v\n", *itemReq.ProductID, err)
				return fmt.Errorf("product %d not found: %v", *itemReq.ProductID, err)
			}
			fmt.Printf("✅ Product found: %s (ID: %d)\n", product.Name, product.ID)
			if !product.IsService && !product.IsAssembly {
				hasInventoryValuationItem = true
			}
		} else {
			// Non-inventory items use a transaction description instead of a product master.
			if strings.TrimSpace(itemReq.Description) == "" {
				return fmt.Errorf("item %d: description is required for non-inventory items", i+1)
			}
		}

		// Create purchase item
		item := models.PurchaseItem{
			ProductID:             itemReq.ProductID,
			PurchaseRequestItemID: itemReq.PurchaseRequestItemID,
			ItemType:              itemType,
			IsFixedAsset:          isAssetItem,
			Description:           itemReq.Description,
			AssetCategoryID:       itemReq.AssetCategoryID,
			AmortizationMonth:     itemReq.AmortizationMonth,
			Quantity:              itemReq.Quantity,
			UnitPrice:             sanitizeFloat(clampNonNegative(itemReq.UnitPrice)),
			Discount:              sanitizeFloat(clampNonNegative(itemReq.Discount)),
			Tax:                   sanitizeFloat(clampNonNegative(itemReq.Tax)),
			ExpenseAccountID:      itemReq.ExpenseAccountID,
		}

		// Calculate line totals with guards
		lineSubtotal := float64(item.Quantity) * item.UnitPrice
		// Ensure discount never exceeds line subtotal
		if item.Discount > lineSubtotal {
			item.Discount = lineSubtotal
		}
		item.TotalPrice = clampNonNegative(lineSubtotal - item.Discount) // discount reduces cost, never below 0
		item.TotalPrice = sanitizeFloat(item.TotalPrice)
		fmt.Printf("ℹ Item calculation: %d x %.2f = %.2f (Discount: %.2f, Net: %.2f)\n",
			item.Quantity, item.UnitPrice, lineSubtotal, item.Discount, item.TotalPrice)

		subtotalBeforeDiscount += lineSubtotal
		itemDiscountAmount += item.Discount

		// Note: Stock will be updated when purchase is approved, not during creation
		// This ensures stock only changes when purchase is actually approved

		purchase.PurchaseItems = append(purchase.PurchaseItems, item)
	}

	// Calculate order-level discount
	orderDiscountAmount := 0.0
	// Clamp header discount percent to [0,100]
	purchase.Discount = clampPercent(purchase.Discount)
	if purchase.Discount > 0 {
		baseForOrderDisc := subtotalBeforeDiscount - itemDiscountAmount
		if baseForOrderDisc < 0 {
			baseForOrderDisc = 0
		}
		orderDiscountAmount = baseForOrderDisc * purchase.Discount / 100
	}

	// Set basic calculated fields with sanitization
	purchase.FreightCost = sanitizeFloat(clampNonNegative(purchase.FreightCost))
	if purchase.FreightCost > 0 && !hasInventoryValuationItem {
		return fmt.Errorf("freight cost hanya bisa ditambahkan ke purchase yang memiliki item inventory")
	}
	purchase.OtherCostAmount = sanitizeFloat(clampNonNegative(purchase.OtherCostAmount))
	if purchase.OtherCostAmount <= 0 {
		purchase.OtherCostDescription = ""
		purchase.OtherCostAccountID = nil
	}
	purchase.SubtotalBeforeDiscount = sanitizeFloat(clampNonNegative(subtotalBeforeDiscount))
	purchase.ItemDiscountAmount = sanitizeFloat(clampNonNegative(itemDiscountAmount))
	purchase.OrderDiscountAmount = sanitizeFloat(clampNonNegative(orderDiscountAmount))
	purchase.NetBeforeTax = sanitizeFloat(clampNonNegative(subtotalBeforeDiscount - itemDiscountAmount - orderDiscountAmount))

	fmt.Printf("ℹ Calculating tax additions with NetBeforeTax=%.2f\n", purchase.NetBeforeTax)
	// Calculate tax additions (Penambahan)
	// 1. PPN (VAT) calculation - only default to 11% if rate was not provided in request
	purchase.PPNRate = clampPercent(purchase.PPNRate)
	if purchase.PPNRate > 0 {
		purchase.PPNAmount = sanitizeFloat(clampNonNegative(purchase.NetBeforeTax * purchase.PPNRate / 100))
		fmt.Printf("✅ PPN calculated: %.1f%% x %.2f = %.2f\n", purchase.PPNRate, purchase.NetBeforeTax, purchase.PPNAmount)
	} else {
		// If PPNRate is 0, respect it (no VAT case)
		purchase.PPNAmount = 0
		fmt.Printf("ℹ PPN rate is 0%%, no VAT applied\n")
	}
	if err := applyPurchaseVATTreatment(purchase); err != nil {
		return err
	}

	// 2. Other tax additions (absolute amounts)
	purchase.OtherTaxAdditions = sanitizeFloat(clampNonNegative(purchase.OtherTaxAdditions))
	purchase.TotalTaxAdditions = sanitizeFloat(clampNonNegative(purchase.PPNAmount + purchase.OtherTaxAdditions))

	fmt.Printf("ℹ Calculating tax deductions with NetBeforeTax=%.2f\n", purchase.NetBeforeTax)
	// Calculate tax deductions (Pemotongan)
	// 1. PPh 21 calculation
	purchase.PPh21Rate = clampPercent(purchase.PPh21Rate)
	if purchase.PPh21Rate > 0 {
		purchase.PPh21Amount = sanitizeFloat(clampNonNegative(purchase.NetBeforeTax * purchase.PPh21Rate / 100))
		fmt.Printf("✅ PPh 21 calculated: %.1f%% x %.2f = %.2f\n", purchase.PPh21Rate, purchase.NetBeforeTax, purchase.PPh21Amount)
	} else {
		purchase.PPh21Amount = 0
	}

	// 2. PPh 23 calculation
	purchase.PPh23Rate = clampPercent(purchase.PPh23Rate)
	if purchase.PPh23Rate > 0 {
		purchase.PPh23Amount = sanitizeFloat(clampNonNegative(purchase.NetBeforeTax * purchase.PPh23Rate / 100))
		fmt.Printf("✅ PPh 23 calculated: %.1f%% x %.2f = %.2f\n", purchase.PPh23Rate, purchase.NetBeforeTax, purchase.PPh23Amount)
	} else {
		purchase.PPh23Amount = 0
	}

	// Absolute other tax deductions
	purchase.OtherTaxDeductions = sanitizeFloat(clampNonNegative(purchase.OtherTaxDeductions))
	// 3. Total tax deductions
	purchase.TotalTaxDeductions = sanitizeFloat(clampNonNegative(purchase.PPh21Amount + purchase.PPh23Amount + purchase.OtherTaxDeductions))

	fmt.Printf("ℹ Calculating final total amount\n")
	fmt.Printf("   NetBeforeTax: %.2f\n", purchase.NetBeforeTax)
	fmt.Printf("   FreightCost: %.2f\n", purchase.FreightCost)
	fmt.Printf("   OtherCostAmount: %.2f\n", purchase.OtherCostAmount)
	fmt.Printf("   TotalTaxAdditions: %.2f\n", purchase.TotalTaxAdditions)
	fmt.Printf("   TotalTaxDeductions: %.2f\n", purchase.TotalTaxDeductions)
	// Calculate final total amount (never negative)
	// Total = Net Before Tax + Freight Cost + Other Costs + Tax Additions - Tax Deductions
	purchase.TotalAmount = sanitizeFloat(clampNonNegative(purchase.NetBeforeTax + purchase.FreightCost + purchase.OtherCostAmount + purchase.TotalTaxAdditions - purchase.TotalTaxDeductions))
	fmt.Printf("✅ Final total calculated: %.2f + %.2f + %.2f + %.2f - %.2f = %.2f\n",
		purchase.NetBeforeTax, purchase.FreightCost, purchase.OtherCostAmount, purchase.TotalTaxAdditions, purchase.TotalTaxDeductions, purchase.TotalAmount)

	// For legacy compatibility, set TaxAmount to PPN amount
	purchase.TaxAmount = purchase.PPNAmount

	// NOTE: OutstandingAmount and PaidAmount are NOT set here.
	// They remain 0 until the purchase is APPROVED (set by OnPurchaseApproved).
	// Before approval, no financial obligation exists, so Sisa should be 0.
	// Final sanitize (keep existing values if already set by approval)
	purchase.PaidAmount = sanitizeFloat(clampNonNegative(purchase.PaidAmount))
	purchase.OutstandingAmount = sanitizeFloat(clampNonNegative(purchase.OutstandingAmount))
	syncPurchasePPhPaymentStatus(purchase)

	return nil
}

// updateProductCostPrice method has been removed as it was deprecated
// Stock and price updates are now handled in updateProductStockOnApproval()
// when purchase status changes to APPROVED

// recalculatePurchaseTotals recalculates purchase totals
func (s *PurchaseService) recalculatePurchaseTotals(purchase *models.Purchase) error {
	subtotalBeforeDiscount := 0.0
	itemDiscountAmount := 0.0
	hasInventoryValuationItem := false

	for _, item := range purchase.PurchaseItems {
		// Sanitize item fields
		if item.UnitPrice < 0 || math.IsNaN(item.UnitPrice) || math.IsInf(item.UnitPrice, 0) {
			item.UnitPrice = 0
		}
		if item.Discount < 0 || math.IsNaN(item.Discount) || math.IsInf(item.Discount, 0) {
			item.Discount = 0
		}
		lineSubtotal := float64(item.Quantity) * item.UnitPrice
		if item.Discount > lineSubtotal {
			item.Discount = lineSubtotal
		}
		item.TotalPrice = clampNonNegative(lineSubtotal - item.Discount)
		item.TotalPrice = sanitizeFloat(item.TotalPrice)

		subtotalBeforeDiscount += lineSubtotal
		itemDiscountAmount += item.Discount
		itemForValuation := item
		if isInventoryValuationPurchaseItem(&itemForValuation) {
			hasInventoryValuationItem = true
		}
	}

	// Calculate order-level discount
	orderDiscountAmount := 0.0
	purchase.Discount = clampPercent(purchase.Discount)
	if purchase.Discount > 0 {
		baseForOrderDisc := subtotalBeforeDiscount - itemDiscountAmount
		if baseForOrderDisc < 0 {
			baseForOrderDisc = 0
		}
		orderDiscountAmount = baseForOrderDisc * purchase.Discount / 100
	}

	// Set basic calculated fields (sanitized)
	purchase.FreightCost = sanitizeFloat(clampNonNegative(purchase.FreightCost))
	if purchase.FreightCost > 0 && !hasInventoryValuationItem {
		return fmt.Errorf("freight cost hanya bisa ditambahkan ke purchase yang memiliki item inventory")
	}
	purchase.OtherCostAmount = sanitizeFloat(clampNonNegative(purchase.OtherCostAmount))
	if purchase.OtherCostAmount <= 0 {
		purchase.OtherCostDescription = ""
		purchase.OtherCostAccountID = nil
	}
	purchase.SubtotalBeforeDiscount = sanitizeFloat(clampNonNegative(subtotalBeforeDiscount))
	purchase.ItemDiscountAmount = sanitizeFloat(clampNonNegative(itemDiscountAmount))
	purchase.OrderDiscountAmount = sanitizeFloat(clampNonNegative(orderDiscountAmount))
	purchase.NetBeforeTax = sanitizeFloat(clampNonNegative(subtotalBeforeDiscount - itemDiscountAmount - orderDiscountAmount))

	// Recalculate tax additions (Penambahan)
	// 1. PPN (VAT) calculation - only apply rate that's already set
	purchase.PPNRate = clampPercent(purchase.PPNRate)
	if purchase.PPNRate > 0 {
		purchase.PPNAmount = sanitizeFloat(clampNonNegative(purchase.NetBeforeTax * purchase.PPNRate / 100))
	} else {
		// If PPNRate is 0, respect it (no VAT case)
		purchase.PPNAmount = 0
	}
	if err := applyPurchaseVATTreatment(purchase); err != nil {
		return err
	}

	// 2. Other tax additions
	purchase.OtherTaxAdditions = sanitizeFloat(clampNonNegative(purchase.OtherTaxAdditions))
	purchase.TotalTaxAdditions = sanitizeFloat(clampNonNegative(purchase.PPNAmount + purchase.OtherTaxAdditions))

	// Recalculate tax deductions (Pemotongan)
	// 1. PPh 21 calculation
	purchase.PPh21Rate = clampPercent(purchase.PPh21Rate)
	if purchase.PPh21Rate > 0 {
		purchase.PPh21Amount = sanitizeFloat(clampNonNegative(purchase.NetBeforeTax * purchase.PPh21Rate / 100))
	} else {
		purchase.PPh21Amount = 0
	}

	// 2. PPh 23 calculation
	purchase.PPh23Rate = clampPercent(purchase.PPh23Rate)
	if purchase.PPh23Rate > 0 {
		purchase.PPh23Amount = sanitizeFloat(clampNonNegative(purchase.NetBeforeTax * purchase.PPh23Rate / 100))
	} else {
		purchase.PPh23Amount = 0
	}

	// 3. Total tax deductions
	purchase.OtherTaxDeductions = sanitizeFloat(clampNonNegative(purchase.OtherTaxDeductions))
	purchase.TotalTaxDeductions = sanitizeFloat(clampNonNegative(purchase.PPh21Amount + purchase.PPh23Amount + purchase.OtherTaxDeductions))

	// Calculate final total amount (never negative)
	// Total = Net Before Tax + Freight Cost + Other Costs + Tax Additions - Tax Deductions
	purchase.TotalAmount = sanitizeFloat(clampNonNegative(purchase.NetBeforeTax + purchase.FreightCost + purchase.OtherCostAmount + purchase.TotalTaxAdditions - purchase.TotalTaxDeductions))

	// For legacy compatibility, set TaxAmount to PPN amount
	purchase.TaxAmount = purchase.PPNAmount

	// CRITICAL FIX: If purchase is NOT yet approved, OutstandingAmount and PaidAmount
	// should be 0. These are only set during OnPurchaseApproved.
	// This also fixes existing purchases created before the fix.
	approvedStatuses := map[string]bool{
		models.PurchaseStatusApproved:  true,
		models.PurchaseStatusCompleted: true,
	}
	if !approvedStatuses[purchase.Status] {
		purchase.OutstandingAmount = 0
		purchase.PaidAmount = 0
	}
	syncPurchasePPhPaymentStatus(purchase)

	return nil
}

// updatePurchaseItems updates purchase items
func (s *PurchaseService) updatePurchaseItems(purchase *models.Purchase, items []models.PurchaseItemRequest) error {
	if err := validateUniquePurchaseRequestItemIDs(items); err != nil {
		return err
	}

	// Clear existing items
	purchase.PurchaseItems = []models.PurchaseItem{}

	for i, itemReq := range items {
		// Determine item type
		itemType := itemReq.ItemType
		if itemType == "" {
			itemType = models.PurchaseItemTypeInventory
		}
		isAssetItem := itemType == models.PurchaseItemTypeAsset || itemReq.IsFixedAsset
		isPrepaidItem := itemType == models.PurchaseItemTypePrepaid
		isExpenseItem := itemType == models.PurchaseItemTypeExpense

		if isPrepaidItem && (itemReq.AmortizationMonth < 1 || itemReq.AmortizationMonth > 120) {
			return fmt.Errorf("item %d: durasi amortisasi prepaid harus antara 1 dan 120 bulan", i+1)
		}

		if !isAssetItem && !isPrepaidItem && !isExpenseItem {
			// Validate product exists for inventory items
			if itemReq.ProductID == nil || *itemReq.ProductID == 0 {
				return fmt.Errorf("product_id is required for inventory items")
			}
			_, err := s.productRepo.FindByID(*itemReq.ProductID)
			if err != nil {
				return fmt.Errorf("product %d not found", *itemReq.ProductID)
			}
		} else if strings.TrimSpace(itemReq.Description) == "" {
			return fmt.Errorf("description is required for non-inventory items")
		}

		item := models.PurchaseItem{
			ProductID:             itemReq.ProductID,
			PurchaseRequestItemID: itemReq.PurchaseRequestItemID,
			ItemType:              itemType,
			IsFixedAsset:          isAssetItem,
			Description:           itemReq.Description,
			AssetCategoryID:       itemReq.AssetCategoryID,
			AmortizationMonth:     itemReq.AmortizationMonth,
			Quantity:              itemReq.Quantity,
			UnitPrice:             itemReq.UnitPrice,
			Discount:              itemReq.Discount,
			Tax:                   itemReq.Tax,
			ExpenseAccountID:      itemReq.ExpenseAccountID,
		}

		// Calculate totals
		item.TotalPrice = float64(item.Quantity)*item.UnitPrice - item.Discount // Remove duplicate tax addition
		purchase.PurchaseItems = append(purchase.PurchaseItems, item)
	}

	return nil
}

// createPurchaseAccountingEntriesWithLines creates proper journal entries with individual lines for purchase
func (s *PurchaseService) createPurchaseAccountingEntriesWithLines(purchase *models.Purchase, userID uint) (*models.JournalEntry, error) {
	// Get required account IDs from chart of accounts
	accountIDs, err := s.getPurchaseAccountIDs()
	if err != nil {
		return nil, fmt.Errorf("failed to get account IDs: %v", err)
	}

	// Create journal entry with lines
	journalEntry := &models.JournalEntry{
		JournalID:       nil,
		AccountID:       &accountIDs.PrimaryAccountID, // Primary account (inventory or expense)
		Code:            s.generatePurchaseJournalCode(),
		EntryDate:       purchase.Date,
		Description:     fmt.Sprintf("Purchase %s - %s", purchase.Code, purchase.Vendor.Name),
		ReferenceType:   models.JournalRefPurchase,
		ReferenceID:     &purchase.ID,
		Reference:       purchase.Code,
		UserID:          userID,
		Status:          models.JournalStatusDraft,
		IsAutoGenerated: true,
	}

	// Create journal lines
	journalLines, err := s.createPurchaseJournalLines(purchase, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to create journal lines: %v", err)
	}

	// Calculate totals from lines
	totalDebit := 0.0
	totalCredit := 0.0
	for _, line := range journalLines {
		totalDebit += line.DebitAmount
		totalCredit += line.CreditAmount
	}

	journalEntry.TotalDebit = totalDebit
	journalEntry.TotalCredit = totalCredit
	journalEntry.IsBalanced = totalDebit == totalCredit && totalDebit > 0
	journalEntry.JournalLines = journalLines

	// Create journal entry with lines
	if err := s.db.Create(journalEntry).Error; err != nil {
		return nil, fmt.Errorf("failed to create journal entry: %v", err)
	}

	fmt.Printf("✅ Purchase journal entry created with %d lines: ID=%d, Debit=%.2f, Credit=%.2f\n",
		len(journalLines), journalEntry.ID, journalEntry.TotalDebit, journalEntry.TotalCredit)

	return journalEntry, nil
}

func (s *PurchaseService) resolvePurchaseOtherCostAccountForJournal(purchase *models.Purchase) (uint, string, error) {
	if purchase.OtherCostAccountID != nil && *purchase.OtherCostAccountID > 0 {
		var selected models.Account
		if err := s.db.First(&selected, *purchase.OtherCostAccountID).Error; err != nil {
			return 0, "", fmt.Errorf("COA biaya lain-lain pembelian tidak ditemukan: %v", err)
		}
		if selected.IsHeader || !strings.EqualFold(strings.TrimSpace(selected.Type), models.AccountTypeExpense) {
			return 0, "", fmt.Errorf("COA biaya lain-lain pembelian harus akun EXPENSE non-header")
		}
		return selected.ID, selected.Name, nil
	}

	for _, code := range []string{"6106", "8102", "6102"} {
		if account, err := s.accountRepo.FindByCode(context.TODO(), code); err == nil && account != nil && !account.IsHeader && strings.EqualFold(strings.TrimSpace(account.Type), models.AccountTypeExpense) {
			return account.ID, account.Name, nil
		}
	}

	var fallback models.Account
	if err := s.db.Where("type = ? AND is_header = ? AND is_active = ? AND deleted_at IS NULL", models.AccountTypeExpense, false, true).
		Order("code ASC").
		First(&fallback).Error; err == nil {
		return fallback.ID, fallback.Name, nil
	}

	return 0, "", fmt.Errorf("COA default biaya lain-lain pembelian tidak ditemukan")
}

// createPurchaseAccountingEntries creates journal entries for purchase
// Adapted to match database schema that expects account_id in journal_entries table
func (s *PurchaseService) createPurchaseAccountingEntries(purchase *models.Purchase, userID uint) (*models.JournalEntry, error) {
	// Get the correct inventory account ID (Persediaan Barang Dagangan - 1301)
	inventoryAccount, err := s.accountRepo.FindByCode(context.TODO(), "1103-001")
	inventoryAccountID := uint(4) // Fallback default
	if err != nil {
		fmt.Printf("⚠️ Warning: Could not find inventory account 1301, using fallback ID 4: %v\n", err)
	} else {
		inventoryAccountID = inventoryAccount.ID
		fmt.Printf("✅ Found inventory account 1301 with ID: %d\n", inventoryAccountID)
	}

	// For this database schema, we'll create one primary journal entry
	// Primary account should be inventory (asset) for merchandise purchases

	// Use inventory account as primary (this is an asset, not expense)
	var primaryAccountID uint = inventoryAccountID
	// Note: ExpenseAccountID in items is used for non-inventory purchases
	// For inventory items, we should always use the inventory account
	if len(purchase.PurchaseItems) > 0 {
		// Check if this is a non-inventory purchase (services, etc.)
		hasNonInventoryItems := false
		for _, item := range purchase.PurchaseItems {
			if item.ExpenseAccountID != 0 {
				hasNonInventoryItems = true
				primaryAccountID = item.ExpenseAccountID // Use expense for non-inventory
				break
			}
		}
		if !hasNonInventoryItems {
			fmt.Printf("📦 Using inventory account %d for merchandise purchase\n", inventoryAccountID)
		} else {
			fmt.Printf("📋 Using expense account %d for non-inventory purchase\n", primaryAccountID)
		}
	}

	// Calculate proper accounting totals
	// Debit side: Inventory/Asset accounts + PPN Masukan + other costs
	totalDebits := purchase.NetBeforeTax + purchase.PPNAmount + purchase.OtherCostAmount

	// Credit side depends on payment method
	var totalCredits float64
	if isImmediatePayment(purchase.PaymentMethod) {
		// For cash/transfer: Credit to Bank Account
		totalCredits = purchase.TotalAmount
	} else {
		// For credit: Credit to Accounts Payable
		totalCredits = purchase.TotalAmount
	}

	// Debug logging
	fmt.Printf("🧮 Purchase Calculation Debug:\n")
	fmt.Printf("   NetBeforeTax: %.2f\n", purchase.NetBeforeTax)
	fmt.Printf("   PPNAmount: %.2f\n", purchase.PPNAmount)
	fmt.Printf("   OtherCostAmount: %.2f\n", purchase.OtherCostAmount)
	fmt.Printf("   TotalAmount: %.2f\n", purchase.TotalAmount)
	fmt.Printf("   Calculated totalDebits: %.2f\n", totalDebits)
	fmt.Printf("   Calculated totalCredits: %.2f\n", totalCredits)

	// Ensure balanced entry - if totals don't match, use the larger amount for both
	if totalDebits != totalCredits {
		// In purchase accounting: Debit (Expense + PPN) = Credit (Payable)
		// Use the purchase total as the balanced amount
		balancedAmount := purchase.TotalAmount
		totalDebits = balancedAmount
		totalCredits = balancedAmount
		fmt.Printf("   ⚖️ Balanced both to: %.2f\n", balancedAmount)
	}

	// Create main journal entry with primary account
	journalEntry := &models.JournalEntry{
		JournalID:       nil,               // Auto-generated entries don't need a parent journal
		AccountID:       &primaryAccountID, // Required by database schema
		Code:            s.generatePurchaseJournalCode(),
		EntryDate:       purchase.Date,
		Description:     fmt.Sprintf("Purchase %s - %s", purchase.Code, purchase.Vendor.Name),
		ReferenceType:   models.JournalRefPurchase,
		ReferenceID:     &purchase.ID,
		Reference:       purchase.Code,
		UserID:          userID,
		Status:          models.JournalStatusDraft,
		TotalDebit:      totalDebits,
		TotalCredit:     totalCredits,
		IsBalanced:      totalDebits == totalCredits && totalDebits > 0,
		IsAutoGenerated: true,
	}

	// Since database doesn't have journal_lines table, we'll store summary information in description
	// OPTIMIZATION: Batch load all account names to avoid N+1 queries
	accountIDs := []uint{inventoryAccountID}
	for _, item := range purchase.PurchaseItems {
		if item.ExpenseAccountID != 0 {
			accountIDs = append(accountIDs, item.ExpenseAccountID)
		}
	}

	// Load all accounts in one query
	accountMap := make(map[uint]string)
	var accounts []models.Account
	if err := s.db.Where("id IN ?", accountIDs).Find(&accounts).Error; err == nil {
		for _, account := range accounts {
			accountMap[account.ID] = account.Name
		}
	}

	// Create detailed description for the journal entry
	var detailsBuilder strings.Builder
	detailsBuilder.WriteString(fmt.Sprintf("Purchase %s - %s\n", purchase.Code, purchase.Vendor.Name))

	// Add line items details in description using cached account names
	for _, item := range purchase.PurchaseItems {
		// Determine which account to use
		accountID := item.ExpenseAccountID
		var accountName string
		if accountID == 0 {
			// This is inventory/merchandise
			accountID = inventoryAccountID
			accountName = accountMap[accountID]
			if accountName == "" {
				accountName = "Persediaan Barang Dagangan" // fallback
			}
		} else {
			// This is non-inventory (expense) - get from cache
			accountName = accountMap[accountID]
			if accountName == "" {
				accountName = "Expense Account" // fallback
			}
		}

		detailsBuilder.WriteString(fmt.Sprintf("Dr. %s: %.2f\n", accountName, item.TotalPrice))
	}

	if purchase.PPNAmount > 0 {
		detailsBuilder.WriteString(fmt.Sprintf("Dr. PPN Receivable: %.2f\n", purchase.PPNAmount))
	}

	detailsBuilder.WriteString(fmt.Sprintf("Cr. Accounts Payable: %.2f", purchase.TotalAmount))

	// Update journal entry description with details
	journalEntry.Description = detailsBuilder.String()

	// Create journal entry without separate lines since database doesn't have journal_lines table
	// The database schema appears to store all journal information in the journal_entries table itself
	if err := s.db.Create(journalEntry).Error; err != nil {
		return nil, fmt.Errorf("failed to create journal entry: %v", err)
	}

	// Log successful journal entry creation
	fmt.Printf("✅ Journal entry created successfully: ID=%d, Debit=%.2f, Credit=%.2f\n",
		journalEntry.ID, journalEntry.TotalDebit, journalEntry.TotalCredit)

	return journalEntry, nil
}

// ProcessPurchaseReceipt processes goods receipt
func (s *PurchaseService) ProcessPurchaseReceipt(purchaseID uint, request models.PurchaseReceiptRequest, userID uint) (*models.PurchaseReceipt, error) {
	purchase, err := s.purchaseRepo.FindByID(purchaseID)
	if err != nil {
		return nil, err
	}

	if purchase.Status != models.PurchaseStatusApproved && purchase.Status != models.PurchaseStatusPending {
		return nil, errors.New("purchase must be approved before receiving goods")
	}

	// Create receipt record
	receipt := &models.PurchaseReceipt{
		PurchaseID:    purchaseID,
		ReceiptNumber: s.generateReceiptNumber(),
		ReceivedDate:  request.ReceivedDate,
		ReceivedBy:    userID,
		Status:        models.ReceiptStatusPending,
		Notes:         request.Notes,
	}

	createdReceipt, err := s.purchaseRepo.CreateReceipt(receipt)
	if err != nil {
		return nil, err
	}

	// Process receipt items and update inventory
	allReceived := true
	for _, itemReq := range request.ReceiptItems {
		purchaseItem, err := s.purchaseRepo.GetPurchaseItemByID(itemReq.PurchaseItemID)
		if err != nil {
			return nil, err
		}

		if purchaseItem.PurchaseID != purchaseID {
			return nil, errors.New("purchase item does not belong to this purchase")
		}

		// Create receipt item
		receiptItem := &models.PurchaseReceiptItem{
			ReceiptID:        createdReceipt.ID,
			PurchaseItemID:   itemReq.PurchaseItemID,
			QuantityReceived: itemReq.QuantityReceived,
			Condition:        itemReq.Condition,
			Notes:            itemReq.Notes,
		}

		err = s.purchaseRepo.CreateReceiptItem(receiptItem)
		if err != nil {
			return nil, err
		}

		// Handle damaged goods by adjusting stock accordingly
		// FIXED: Now properly handles damaged goods during receipt processing
		if receiptItem.Condition == models.ReceiptConditionDamaged || receiptItem.Condition == models.ReceiptConditionDefected {
			// Skip stock adjustment for ASSET items
			if purchaseItem.IsFixedAsset || purchaseItem.ItemType == models.PurchaseItemTypeAsset {
				fmt.Printf("ℹ️ Skipping damaged goods stock adjustment for ASSET item (PurchaseItem:%d)\n", purchaseItem.ID)
			} else if purchaseItem.ProductID != nil && *purchaseItem.ProductID > 0 {
				// Reduce stock for damaged/defective items
				product, err := s.productRepo.FindByID(*purchaseItem.ProductID)
				if err != nil {
					fmt.Printf("Warning: Could not find product %d to adjust stock for damaged goods: %v\n", *purchaseItem.ProductID, err)
				} else {
					damagedQty := itemReq.QuantityReceived
					if receiptItem.Condition == models.ReceiptConditionDamaged {
						// Damaged goods - reduce by full quantity
						product.Stock -= damagedQty
						fmt.Printf("📦 Reducing stock by %d for damaged goods (Product: %s)\n", damagedQty, product.Name)
					} else if receiptItem.Condition == models.ReceiptConditionDefected {
						// Defective goods - reduce by full quantity
						product.Stock -= damagedQty
						fmt.Printf("⚠️ Reducing stock by %d for defective goods (Product: %s)\n", damagedQty, product.Name)
					}

					// Ensure stock doesn't go negative
					if product.Stock < 0 {
						fmt.Printf("🛑 Warning: Stock for product %s went negative (%d), setting to 0\n", product.Name, product.Stock)
						product.Stock = 0
					}

					// Update product stock
					err = s.productRepo.Update(context.Background(), product)
					if err != nil {
						fmt.Printf("Error updating stock for damaged/defective goods: %v\n", err)
					}
				}
			}
		}

		// Note: For good condition items, stock was already updated when purchase was approved.

		// Check if all items are fully received
		if itemReq.QuantityReceived < purchaseItem.Quantity {
			allReceived = false
		}
	}

	// Update receipt and purchase status
	if allReceived {
		receipt.Status = models.ReceiptStatusComplete
		purchase.Status = models.PurchaseStatusCompleted
	} else {
		receipt.Status = models.ReceiptStatusPartial
	}

	s.purchaseRepo.UpdateReceipt(receipt)
	s.purchaseRepo.Update(purchase)

	return createdReceipt, nil
}

// generatePurchaseCode generates unique purchase code with concurrency control
// FIXED: Uses database transaction with row-level locking to prevent race conditions
// NOW: Uses settings for purchase prefix
func (s *PurchaseService) generatePurchaseCode() (string, error) {
	// Get settings to use configured prefix
	settingsService := NewSettingsService(s.db)
	settings, err := settingsService.GetSettings()
	if err != nil {
		fmt.Printf("❌ Failed to get settings for purchase code generation: %v\n", err)
		return "", fmt.Errorf("failed to get settings: %v", err)
	}

	year := time.Now().Year()
	month := time.Now().Month()
	fmt.Printf("ℹ Generating purchase code for %04d/%02d with prefix %s\n", year, month, settings.PurchasePrefix)

	var code string
	fmt.Printf("ℹ Starting database transaction for purchase code generation\n")
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Use row-level locking to prevent concurrent access
		// Lock a dummy row or create a sequence table for purchase codes
		fmt.Printf("ℹ Acquiring row lock for purchase sequence\n")
		type PurchaseSequence struct {
			Year       int `gorm:"primaryKey"`
			Month      int `gorm:"primaryKey"`
			LastNumber int `gorm:"default:0"`
		}

		// Ensure the sequence table exists (this should be in migrations, but adding safety)
		tx.AutoMigrate(&PurchaseSequence{})

		var sequence PurchaseSequence
		// Use SELECT FOR UPDATE to lock the row and prevent race conditions
		result := tx.Set("gorm:query_option", "FOR UPDATE").Where("year = ? AND month = ?", year, month).First(&sequence)

		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				// Create new sequence record for this year/month
				fmt.Printf("ℹ Creating new purchase sequence for %04d/%02d\n", year, month)
				sequence = PurchaseSequence{
					Year:       year,
					Month:      int(month),
					LastNumber: 0,
				}
				if err := tx.Create(&sequence).Error; err != nil {
					fmt.Printf("❌ Failed to create purchase sequence: %v\n", err)
					return fmt.Errorf("failed to create purchase sequence: %v", err)
				}
				fmt.Printf("✅ Purchase sequence created for %04d/%02d\n", year, month)
			} else {
				fmt.Printf("❌ Failed to get purchase sequence: %v\n", result.Error)
				return fmt.Errorf("failed to get purchase sequence: %v", result.Error)
			}
		} else {
			fmt.Printf("✅ Found existing purchase sequence: LastNumber=%d\n", sequence.LastNumber)
		}

		// Increment the sequence number
		sequence.LastNumber++
		fmt.Printf("ℹ Incrementing sequence number to %d\n", sequence.LastNumber)

		// Generate the code using settings prefix
		code = fmt.Sprintf("%s/%04d/%02d/%04d", settings.PurchasePrefix, year, month, sequence.LastNumber)
		fmt.Printf("ℹ Generated purchase code: %s\n", code)

		// Double-check that this code doesn't exist in purchases table
		fmt.Printf("ℹ Verifying code uniqueness\n")
		var existingCount int64
		tx.Model(&models.Purchase{}).Where("code = ?", code).Count(&existingCount)

		if existingCount > 0 {
			// This should not happen with proper locking, but handle it
			fmt.Printf("❌ Generated code %s already exists (count: %d)\n", code, existingCount)
			return fmt.Errorf("generated code %s already exists, possible concurrency issue", code)
		}
		fmt.Printf("✅ Code uniqueness verified\n")

		// Update the sequence record
		fmt.Printf("ℹ Saving updated sequence record\n")
		if err := tx.Save(&sequence).Error; err != nil {
			fmt.Printf("❌ Failed to update purchase sequence: %v\n", err)
			return fmt.Errorf("failed to update purchase sequence: %v", err)
		}
		fmt.Printf("✅ Purchase sequence updated successfully\n")

		return nil
	})

	if err != nil {
		fmt.Printf("❌ Transaction failed for purchase code generation: %v\n", err)
		return "", err
	}

	fmt.Printf("✅ Purchase code generated successfully: %s\n", code)
	return code, nil
}

// generateReceiptNumber generates unique receipt number
func (s *PurchaseService) generateReceiptNumber() string {
	year := time.Now().Year()
	month := time.Now().Month()
	count, _ := s.purchaseRepo.CountReceiptsByMonth(year, int(month))
	return fmt.Sprintf("GR/%04d/%02d/%04d", year, month, count+1)
}

// generatePurchaseJournalCode generates unique journal code for purchase
func (s *PurchaseService) generatePurchaseJournalCode() string {
	year := time.Now().Year()
	month := time.Now().Month()
	count, _ := s.purchaseRepo.CountJournalsByMonth(year, int(month))

	// Keep trying to generate a unique code
	for i := 1; i <= 100; i++ {
		nextNumber := count + int64(i)
		code := fmt.Sprintf("PJ/%04d/%02d/%04d", year, month, nextNumber)

		// Check if this code already exists in the journal_entries table
		var existingCount int64
		s.db.Model(&models.JournalEntry{}).Where("code = ?", code).Count(&existingCount)

		if existingCount == 0 {
			return code
		}
	}

	// Final fallback - use timestamp
	return fmt.Sprintf("PJ/%04d/%02d/%d", year, month, time.Now().Unix())
}

// GetPurchaseSummary gets purchase summary with analytics
func (s *PurchaseService) GetPurchaseSummary(startDate, endDate string) (*models.PurchaseSummary, error) {
	return s.purchaseRepo.GetPurchaseSummary(startDate, endDate)
}

// GetPayablesReport gets accounts payable report
func (s *PurchaseService) GetPayablesReport() (*models.PayablesReportResponse, error) {
	return s.purchaseRepo.GetPayablesReport()
}

// CreatePurchaseReturn creates a purchase return
// TODO: Uncomment when PurchaseReturn and PurchaseReturnItem models are created
/*
func (s *PurchaseService) CreatePurchaseReturn(purchaseID uint, reason string, items []models.PurchaseReturnItem, userID uint) (*models.PurchaseReturn, error) {
	purchase, err := s.purchaseRepo.FindByID(purchaseID)
	if err != nil {
		return nil, err
	}

	if purchase.Status != models.PurchaseStatusCompleted {
		return nil, errors.New("purchase must be completed before creating return")
	}

	// Calculate return amount
	totalAmount := 0.0
	for _, item := range items {
		purchaseItem, _ := s.purchaseRepo.GetPurchaseItemByID(item.PurchaseItemID)
		if purchaseItem != nil {
			totalAmount += float64(item.Quantity) * purchaseItem.UnitPrice
		}
	}

	// Create return record
	purchaseReturn := &models.PurchaseReturn{
		PurchaseID:    purchaseID,
		ReturnNumber:  s.generatePurchaseReturnNumber(),
		Date:          time.Now(),
		Reason:        reason,
		TotalAmount:   totalAmount,
		Status:        models.PurchaseReturnStatusPending,
		UserID:        userID,
	}

	createdReturn, err := s.purchaseRepo.CreateReturn(purchaseReturn)
	if err != nil {
		return nil, err
	}

	// Create return items and adjust inventory
	for _, item := range items {
		item.PurchaseReturnID = createdReturn.ID
		err = s.purchaseRepo.CreateReturnItem(&item)
		if err != nil {
			return nil, err
		}

		// Reduce inventory
		purchaseItem, _ := s.purchaseRepo.GetPurchaseItemByID(item.PurchaseItemID)
		if purchaseItem != nil && purchaseItem.ProductID != nil && *purchaseItem.ProductID > 0 {
			product, _ := s.productRepo.FindByID(*purchaseItem.ProductID)
			if product != nil {
				product.StockQuantity -= item.Quantity
				s.productRepo.Update(product)
			}
		}
	}

	// Create reversal journal entries
	s.createPurchaseReturnJournalEntries(createdReturn, userID)

	return createdReturn, nil
}

// createPurchaseReturnJournalEntries creates journal entries for purchase return
func (s *PurchaseService) createPurchaseReturnJournalEntries(purchaseReturn *models.PurchaseReturn, userID uint) error {
	// This would create the reverse entries of the original purchase
	// Debit: Accounts Payable
	// Credit: Inventory/Expense accounts
	// Credit: PPN Receivable (if applicable)

	// Implementation details would mirror the purchase entries but in reverse
	return nil
}

// generatePurchaseReturnNumber generates unique return number
func (s *PurchaseService) generatePurchaseReturnNumber() string {
	year := time.Now().Year()
	month := time.Now().Month()
	count, _ := s.purchaseRepo.CountReturnsByMonth(year, int(month))
	return fmt.Sprintf("PR/%04d/%02d/%04d", year, month, count+1)
}

*/

// PurchaseAccountIDs holds the account IDs needed for purchase journal entries
type PurchaseAccountIDs struct {
	PrimaryAccountID   uint // Inventory or main expense account
	InventoryAccountID uint // 1103-001 - Persediaan Barang Dagangan
	PPNInputAccountID  uint // 1105-005 - PPN Masukan
	AccountsPayableID  uint // 2101 - Utang Usaha
	PPh21PayableID     uint // 2102-003 - Utang PPh 21
	PPh23PayableID     uint // 2102-004 - Utang PPh 23
}

// getPurchaseAccountIDs retrieves all required account IDs for purchase journal entries
// Company-type-aware: SERVICE companies use expense account, TRADING/MANUFACTURING use inventory
func (s *PurchaseService) getPurchaseAccountIDs() (*PurchaseAccountIDs, error) {
	accountIDs := &PurchaseAccountIDs{}

	// Check company type to determine the primary debit account
	validator := NewCompanyTypeValidator(s.db)
	purchaseAccountCode := validator.GetPurchaseDebitAccountCode()

	// Get primary purchase account (inventory or expense based on company type)
	if primaryAccount, err := s.accountRepo.FindByCode(context.TODO(), purchaseAccountCode); err == nil {
		accountIDs.PrimaryAccountID = primaryAccount.ID
		if validator.IsInventoryAllowed() {
			accountIDs.InventoryAccountID = primaryAccount.ID
		}
		log.Printf("📊 Purchase primary account: %s (%s) [company type: %s]", primaryAccount.Name, primaryAccount.Code, validator.GetCompanyType())
	} else {
		// Fallback: try inventory account anyway (might exist from manual creation)
		if inventoryAccount, err := s.accountRepo.FindByCode(context.TODO(), "1103-001"); err == nil {
			accountIDs.InventoryAccountID = inventoryAccount.ID
			accountIDs.PrimaryAccountID = inventoryAccount.ID
			log.Printf("⚠️ Using fallback inventory account 1103-001 (primary %s not found)", purchaseAccountCode)
		} else {
			return nil, fmt.Errorf("purchase debit account %s not found: %v", purchaseAccountCode, err)
		}
	}

	// Get PPN Input account from tax settings first
	taxAccountHelper := NewTaxAccountHelper(s.db)
	if ppnAccount, err := taxAccountHelper.GetPurchaseInputVATAccount(s.db); err == nil {
		accountIDs.PPNInputAccountID = ppnAccount.ID
	} else {
		return nil, fmt.Errorf("PPN input account from tax settings not found: %v", err)
	}

	// Get Accounts Payable (2101)
	if apAccount, err := s.accountRepo.FindByCode(context.TODO(), "2101"); err == nil {
		accountIDs.AccountsPayableID = apAccount.ID
	} else {
		return nil, fmt.Errorf("accounts payable account 2101 not found: %v", err)
	}

	// Get PPh 21 Payable (2111) - optional
	if pph21Account, err := s.accountRepo.FindByCode(context.TODO(), "2102-003"); err == nil {
		accountIDs.PPh21PayableID = pph21Account.ID
	}

	// Get PPh 23 Payable (2112) - optional
	if pph23Account, err := s.accountRepo.FindByCode(context.TODO(), "2102-004"); err == nil {
		accountIDs.PPh23PayableID = pph23Account.ID
	}

	return accountIDs, nil
}

// createPurchaseJournalLines creates individual journal lines for purchase transaction
func (s *PurchaseService) createPurchaseJournalLines(purchase *models.Purchase, accountIDs *PurchaseAccountIDs) ([]models.JournalLine, error) {
	var lines []models.JournalLine
	lineNumber := 1

	// DEBIT SIDE - Assets and Tax Receivables

	// 1. Debit Inventory/Expense accounts for each item
	for _, item := range purchase.PurchaseItems {
		accountID := accountIDs.InventoryAccountID // Default to inventory
		accountName := "Persediaan Barang Dagangan"

		// Use specific expense account if provided
		if item.ExpenseAccountID != 0 {
			accountID = item.ExpenseAccountID
			if account, err := s.accountRepo.FindByID(nil, accountID); err == nil && account != nil {
				accountName = account.Name
			} else {
				accountName = "Expense Account"
			}
		}

		landedTotal := purchaseItemLandedTotal(purchase, &item)
		lines = append(lines, models.JournalLine{
			AccountID:    accountID,
			Description:  fmt.Sprintf("%s - %s", accountName, item.Product.Name),
			DebitAmount:  landedTotal,
			CreditAmount: 0,
			LineNumber:   lineNumber,
		})
		lineNumber++
	}

	// 2. Debit PPN Masukan (Input VAT) if applicable
	if purchase.PPNAmount > 0 {
		lines = append(lines, models.JournalLine{
			AccountID:    accountIDs.PPNInputAccountID,
			Description:  "PPN Masukan (Input VAT)",
			DebitAmount:  purchase.PPNAmount,
			CreditAmount: 0,
			LineNumber:   lineNumber,
		})
		lineNumber++
	}

	if purchase.OtherCostAmount > 0 {
		otherCostAccountID, otherCostAccountName, err := s.resolvePurchaseOtherCostAccountForJournal(purchase)
		if err != nil {
			return nil, err
		}
		description := strings.TrimSpace(purchase.OtherCostDescription)
		if description == "" {
			description = "Biaya lain-lain pembelian"
		}
		lines = append(lines, models.JournalLine{
			AccountID:    otherCostAccountID,
			Description:  fmt.Sprintf("%s - %s", otherCostAccountName, description),
			DebitAmount:  purchase.OtherCostAmount,
			CreditAmount: 0,
			LineNumber:   lineNumber,
		})
		lineNumber++
	}

	// CREDIT SIDE - Liabilities

	// 3. Credit side based on payment method
	if isImmediatePayment(purchase.PaymentMethod) {
		// Immediate payment: credit Cash/Bank with net amount (TotalAmount)
		var cashAccountID uint
		if purchase.BankAccountID != nil && *purchase.BankAccountID != 0 {
			var cashBank models.CashBank
			if err := s.db.Select("account_id").First(&cashBank, *purchase.BankAccountID).Error; err == nil && cashBank.AccountID != 0 {
				cashAccountID = cashBank.AccountID
			}
		}
		// Fallback to default cash account 1101
		if cashAccountID == 0 {
			if acc, err := s.accountRepo.FindByCode(context.TODO(), "1101"); err == nil {
				cashAccountID = acc.ID
			}
		}
		lines = append(lines, models.JournalLine{
			AccountID:    cashAccountID,
			Description:  fmt.Sprintf("Pembayaran langsung - %s", purchase.Vendor.Name),
			DebitAmount:  0,
			CreditAmount: purchase.TotalAmount,
			LineNumber:   lineNumber,
		})
		lineNumber++
	} else {
		// Credit purchase: credit Accounts Payable with net payable (TotalAmount)
		apAccountID := accountIDs.AccountsPayableID
		if purchase.CreditAccountID != nil && *purchase.CreditAccountID != 0 {
			var selectedAccount models.Account
			if err := s.db.Select("id", "type", "is_header").First(&selectedAccount, *purchase.CreditAccountID).Error; err == nil &&
				!selectedAccount.IsHeader &&
				strings.EqualFold(strings.TrimSpace(selectedAccount.Type), models.AccountTypeLiability) {
				apAccountID = selectedAccount.ID
			}
		}
		lines = append(lines, models.JournalLine{
			AccountID:    apAccountID,
			Description:  fmt.Sprintf("Utang Usaha - %s", purchase.Vendor.Name),
			DebitAmount:  0,
			CreditAmount: purchase.TotalAmount,
			LineNumber:   lineNumber,
		})
		lineNumber++
	}

	// 4. Credit PPh 21 Payable if applicable
	if purchase.PPh21Amount > 0 && accountIDs.PPh21PayableID != 0 {
		lines = append(lines, models.JournalLine{
			AccountID:    accountIDs.PPh21PayableID,
			Description:  "Utang PPh 21",
			DebitAmount:  0,
			CreditAmount: purchase.PPh21Amount,
			LineNumber:   lineNumber,
		})
		lineNumber++
	}

	// 5. Credit PPh 23 Payable if applicable
	if purchase.PPh23Amount > 0 && accountIDs.PPh23PayableID != 0 {
		lines = append(lines, models.JournalLine{
			AccountID:    accountIDs.PPh23PayableID,
			Description:  "Utang PPh 23",
			DebitAmount:  0,
			CreditAmount: purchase.PPh23Amount,
			LineNumber:   lineNumber,
		})
		lineNumber++
	}

	// Validation: Ensure balanced entry
	totalDebit := 0.0
	totalCredit := 0.0
	for _, line := range lines {
		totalDebit += line.DebitAmount
		totalCredit += line.CreditAmount
	}

	if totalDebit != totalCredit {
		return nil, fmt.Errorf("unbalanced journal entry: debit=%.2f, credit=%.2f", totalDebit, totalCredit)
	}

	fmt.Printf("📋 Created %d journal lines: Dr=%.2f, Cr=%.2f\n", len(lines), totalDebit, totalCredit)
	return lines, nil
}

// createAndPostPurchaseJournalEntries creates journal entries for approved purchase and posts them to GL
func (s *PurchaseService) createAndPostPurchaseJournalEntries(purchase *models.Purchase, userID uint) error {
	if isComponentPurchaseVAT(purchase) {
		return fmt.Errorf("jalur jurnal legacy tidak mendukung PPN per komponen; gunakan PurchaseJournalServiceSSOT")
	}
	// First, check if journal entries already exist for this purchase
	existingEntry, err := s.journalRepo.FindByReferenceID(
		context.Background(),
		models.JournalRefPurchase,
		purchase.ID,
	)

	// Handle any unexpected database errors
	if err != nil {
		return fmt.Errorf("failed to check existing journal entries: %v", err)
	}

	// If journal entry already exists and is posted, don't create another one
	if existingEntry != nil {
		if existingEntry.Status == models.JournalStatusPosted {
			fmt.Printf("Journal entry already posted for purchase %d\n", purchase.ID)
			return nil
		}
		// If exists but not posted, post the existing entry
		err = s.journalRepo.PostJournalEntry(context.Background(), existingEntry.ID, userID)
		if err != nil {
			return fmt.Errorf("failed to post existing journal entry: %v", err)
		}
		fmt.Printf("Posted existing journal entry for purchase %d\n", purchase.ID)
		return nil
	}

	// Create new journal entries using proper journal lines implementation
	journalEntry, err := s.createPurchaseAccountingEntriesWithLines(purchase, userID)
	if err != nil {
		return fmt.Errorf("failed to create journal entries: %v", err)
	}

	// Post the journal entry to update account balances
	err = s.journalRepo.PostJournalEntry(context.Background(), journalEntry.ID, userID)
	if err != nil {
		return fmt.Errorf("failed to post journal entry: %v", err)
	}

	fmt.Printf("Successfully created and posted journal entry for purchase %d\n", purchase.ID)
	return nil
}
