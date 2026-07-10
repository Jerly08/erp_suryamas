package services

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	manualSaleProductCode = "MANUAL-SALE-ITEM"
	manualSaleProductName = "Manual Sale Item"
)

// SalesServiceV2 handles all sales operations with clean business logic
type SalesServiceV2 struct {
	db                      *gorm.DB
	salesRepo               *repositories.SalesRepository
	salesJournalService     *SalesJournalServiceV2   // Legacy simple_ssot_journals (deprecated)
	salesJournalServiceSSOT *SalesJournalServiceSSOT // NEW: unified_journal_ledger (for Balance Sheet)
	cogsService             *InventoryCOGSService    // NEW: COGS recording service
	stockService            *StockService
	stockMonitoringService  *StockMonitoringService
	notificationService     *NotificationService
	settingsService         *SettingsService
	invoiceNumberService    *InvoiceNumberService
}

// NewSalesServiceV2 creates a new instance of SalesServiceV2
func NewSalesServiceV2(
	db *gorm.DB,
	salesRepo *repositories.SalesRepository,
	salesJournalService *SalesJournalServiceV2,
	salesJournalServiceSSOT *SalesJournalServiceSSOT,
	stockService *StockService,
	stockMonitoringService *StockMonitoringService,
	notificationService *NotificationService,
	settingsService *SettingsService,
	invoiceNumberService *InvoiceNumberService,
) *SalesServiceV2 {
	// Initialize COGS service for automatic COGS recording
	coaService := NewCOAService(db)
	cogsService := NewInventoryCOGSService(db, coaService)

	return &SalesServiceV2{
		db:                      db,
		salesRepo:               salesRepo,
		salesJournalService:     salesJournalService,
		salesJournalServiceSSOT: salesJournalServiceSSOT,
		cogsService:             cogsService,
		stockService:            stockService,
		stockMonitoringService:  stockMonitoringService,
		notificationService:     notificationService,
		settingsService:         settingsService,
		invoiceNumberService:    invoiceNumberService,
	}
}

func normalizeSaleItemProductName(item *models.SaleItemRequest) string {
	if item == nil {
		return ""
	}

	if name := strings.TrimSpace(item.ProductName); name != "" {
		return name
	}

	if item.ProductID == 0 {
		return strings.TrimSpace(item.Description)
	}

	return ""
}

func (s *SalesServiceV2) ensureManualSaleProduct(tx *gorm.DB) (*models.Product, error) {
	var product models.Product
	err := tx.Unscoped().Where("code = ?", manualSaleProductCode).First(&product).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to load manual sale product: %v", err)
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		product = models.Product{
			Code:        manualSaleProductCode,
			Name:        manualSaleProductName,
			Description: "System service product for manual sales items",
			Unit:        "pcs",
			IsActive:    true,
			IsService:   true,
			Taxable:     true,
		}
		if err := tx.Create(&product).Error; err != nil {
			return nil, fmt.Errorf("failed to create manual sale product: %v", err)
		}
		return &product, nil
	}

	updates := map[string]interface{}{}
	if product.DeletedAt.Valid {
		updates["deleted_at"] = nil
	}
	if !product.IsActive {
		updates["is_active"] = true
	}
	if !product.IsService {
		updates["is_service"] = true
	}
	if strings.TrimSpace(product.Name) == "" {
		updates["name"] = manualSaleProductName
	}
	if strings.TrimSpace(product.Unit) == "" {
		updates["unit"] = "pcs"
	}

	if len(updates) > 0 {
		if err := tx.Unscoped().Model(&product).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("failed to update manual sale product: %v", err)
		}
		if err := tx.Unscoped().Where("id = ?", product.ID).First(&product).Error; err != nil {
			return nil, fmt.Errorf("failed to reload manual sale product: %v", err)
		}
	}

	return &product, nil
}

func (s *SalesServiceV2) prepareSaleItemRequest(tx *gorm.DB, item *models.SaleItemRequest) error {
	if item == nil {
		return fmt.Errorf("sale item is required")
	}

	manualName := normalizeSaleItemProductName(item)
	if item.ProductID == 0 {
		if manualName == "" {
			return fmt.Errorf("manual sale item must include a product name")
		}

		manualProduct, err := s.ensureManualSaleProduct(tx)
		if err != nil {
			return err
		}

		item.ProductID = manualProduct.ID
		item.ProductName = manualName
		return nil
	}

	var product models.Product
	if err := tx.Select("id", "name").First(&product, item.ProductID).Error; err != nil {
		return fmt.Errorf("failed to load product %d: %v", item.ProductID, err)
	}

	if manualName != "" {
		item.ProductName = manualName
	} else {
		item.ProductName = product.Name
	}

	return nil
}

func buildImmediateSaleCashBankMutation(sale *models.Sale, cashBank *models.CashBank) (*cashBankBalanceMutation, error) {
	if sale == nil {
		return nil, fmt.Errorf("sale is required")
	}
	if cashBank == nil {
		return nil, fmt.Errorf("cash/bank account is required")
	}

	currency := normalizeCashBankCurrency(cashBank.Currency)
	mutation := &cashBankBalanceMutation{
		NativeAmount:    sale.TotalAmount,
		BookAmountIDR:   sale.TotalAmount,
		Currency:        currency,
		ExchangeRate:    1,
		TransactionDate: sale.Date,
		Notes:           fmt.Sprintf("Immediate payment - Invoice #%s", sale.InvoiceNumber),
	}

	if !cashBank.IsForeignCurrency() {
		return mutation, nil
	}

	exchangeRate := 0.0
	if strings.ToUpper(strings.TrimSpace(sale.Currency)) == currency && sale.ExchangeRate > 0 {
		exchangeRate = sale.ExchangeRate
	}
	if exchangeRate <= 0 && cashBank.LastExchangeRate > 0 {
		exchangeRate = cashBank.LastExchangeRate
	}
	if exchangeRate <= 1 {
		return nil, fmt.Errorf("exchange rate is required for immediate payment into foreign cash/bank account %s", cashBank.Name)
	}

	mutation.ExchangeRate = exchangeRate
	mutation.BookAmountIDR = sale.TotalAmount
	mutation.NativeAmount = sale.TotalAmount / exchangeRate
	if mutation.NativeAmount <= 0 {
		return nil, fmt.Errorf("failed to derive native foreign amount for immediate payment")
	}

	return mutation, nil
}

func (s *SalesServiceV2) enforceCashBankGuards(tx *gorm.DB, cashBankIDs map[uint]struct{}) error {
	if len(cashBankIDs) == 0 {
		return nil
	}

	guard := NewCashBankCurrencyGuard(s.db)
	for cashBankID := range cashBankIDs {
		if err := guard.EnforceAfterTransaction(tx, cashBankID); err != nil {
			return fmt.Errorf("failed to enforce cash/bank currency invariant for account %d: %w", cashBankID, err)
		}
	}

	return nil
}

func (s *SalesServiceV2) reverseSaleCashBankMovement(tx *gorm.DB, referenceType string, referenceID uint, reason string) error {
	var originalCBTx models.CashBankTransaction
	if err := tx.Where("reference_type = ? AND reference_id = ? AND deleted_at IS NULL", referenceType, referenceID).
		Order("id DESC").
		First(&originalCBTx).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("failed to load cash/bank transaction for %s #%d: %w", referenceType, referenceID, err)
	}

	guard := NewCashBankCurrencyGuard(s.db)
	if err := guard.EnforceAfterTransaction(tx, originalCBTx.CashBankID); err != nil {
		return fmt.Errorf("failed to sync cash/bank state before %s reversal: %w", referenceType, err)
	}

	var cashBank models.CashBank
	if err := tx.First(&cashBank, originalCBTx.CashBankID).Error; err != nil {
		return fmt.Errorf("failed to load cash/bank account %d for %s reversal: %w", originalCBTx.CashBankID, referenceType, err)
	}

	reverseMovement := -originalCBTx.Amount
	reverseBookAmountIDR := -originalCBTx.BookAmountIDR
	newBalance := cashBank.Balance + reverseMovement
	newBookBalanceIDR := cashBank.BookBalanceIDR + reverseBookAmountIDR
	if !cashBank.IsForeignCurrency() {
		newBookBalanceIDR = newBalance
	}

	if newBalance < -0.01 {
		return fmt.Errorf("%s cancellation would result in negative cash/bank native balance", referenceType)
	}
	if newBookBalanceIDR < -0.01 {
		return fmt.Errorf("%s cancellation would result in negative cash/bank book balance", referenceType)
	}

	cashBank.Balance = newBalance
	cashBank.BookBalanceIDR = newBookBalanceIDR
	if cashBank.IsForeignCurrency() {
		if originalCBTx.ExchangeRate > 0 {
			cashBank.LastExchangeRate = originalCBTx.ExchangeRate
		}
	} else {
		cashBank.LastExchangeRate = 1
	}

	if err := tx.Save(&cashBank).Error; err != nil {
		return fmt.Errorf("failed to reverse cash/bank balance for %s #%d: %w", referenceType, referenceID, err)
	}

	if cashBank.AccountID > 0 {
		if err := tx.Model(&models.Account{}).
			Where("id = ? AND deleted_at IS NULL", cashBank.AccountID).
			Updates(map[string]interface{}{
				"balance":    cashBank.EffectiveBookBalanceIDR(),
				"updated_at": time.Now(),
			}).Error; err != nil {
			return fmt.Errorf("failed to sync linked GL account for %s #%d reversal: %w", referenceType, referenceID, err)
		}
	}

	reversalTx := &models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   referenceType + "_REVERSAL",
		ReferenceID:     referenceID,
		Amount:          reverseMovement,
		Currency:        originalCBTx.Currency,
		ExchangeRate:    originalCBTx.ExchangeRate,
		BookAmountIDR:   reverseBookAmountIDR,
		BalanceAfter:    newBalance,
		TransactionDate: time.Now(),
		Notes:           fmt.Sprintf("%s reversal - %s", referenceType, reason),
	}
	if err := tx.Create(reversalTx).Error; err != nil {
		return fmt.Errorf("failed to create cash/bank reversal transaction for %s #%d: %w", referenceType, referenceID, err)
	}

	if err := guard.EnforceAfterTransaction(tx, cashBank.ID); err != nil {
		return fmt.Errorf("failed to enforce cash/bank invariant after %s reversal: %w", referenceType, err)
	}

	return nil
}

func (s *SalesServiceV2) syncStockAlertsForProducts(productIDs map[uint]struct{}) {
	if s.stockMonitoringService == nil || len(productIDs) == 0 {
		return
	}

	affectedProductIDs := make([]uint, 0, len(productIDs))
	for productID := range productIDs {
		affectedProductIDs = append(affectedProductIDs, productID)
	}

	if err := s.stockMonitoringService.SyncProductAlerts(affectedProductIDs...); err != nil {
		log.Printf("⚠️ Warning: Failed to sync stock alerts for %d products: %v", len(affectedProductIDs), err)
	}
}

// CreateSale creates a new sale
func (s *SalesServiceV2) CreateSale(request models.SaleCreateRequest, userID uint) (*models.Sale, error) {
	// Start transaction
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// ✅ VALIDATE STOCK BEFORE CREATING SALE
	// Build stock validation request from sale items
	stockValidationReq := models.StockValidationRequest{
		Items: make([]models.SaleItemRequest, 0, len(request.Items)),
	}
	for i := range request.Items {
		if err := s.prepareSaleItemRequest(tx, &request.Items[i]); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("invalid sale item #%d: %v", i+1, err)
		}
		item := request.Items[i]
		stockValidationReq.Items = append(stockValidationReq.Items, models.SaleItemRequest{
			ProductID:   item.ProductID,
			ProductName: item.ProductName,
			Quantity:    item.Quantity,
		})
	}

	// Validate stock availability
	stockValidation, err := s.ValidateStockForCreate(stockValidationReq)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to validate stock: %v", err)
	}

	// Check if there are any stock issues
	if false && stockValidation.HasInsufficient {
		// Build detailed error message
		var insufficientItems []string
		for _, item := range stockValidation.Items {
			if !item.IsSufficient && !item.IsService {
				insufficientItems = append(insufficientItems,
					fmt.Sprintf("❌ %s: Tersedia %d, Diminta %d",
						item.ProductName, item.AvailableQty, item.RequestedQty))
			}
		}
		tx.Rollback()
		return nil, fmt.Errorf("stock tidak mencukupi:\n%s", strings.Join(insufficientItems, "\n"))
	}

	if false && stockValidation.HasZeroStock {
		// Find zero stock items
		var zeroStockItems []string
		for _, item := range stockValidation.Items {
			if item.AvailableQty == 0 && !item.IsService {
				zeroStockItems = append(zeroStockItems,
					fmt.Sprintf("❌ %s: Stock habis (0)", item.ProductName))
			}
		}
		tx.Rollback()
		return nil, fmt.Errorf("beberapa produk stock-nya habis:\n%s", strings.Join(zeroStockItems, "\n"))
	}

	// Log warnings for low stock (but allow creation)
	if stockValidation.HasInsufficient || stockValidation.HasZeroStock || stockValidation.HasMinStockAlerts || stockValidation.HasReorderAlerts {
		for _, item := range stockValidation.Items {
			if item.Warning != "" {
				log.Printf("⚠️ Stock warning for %s: %s", item.ProductName, item.Warning)
			}
		}
	}

	log.Printf("✅ Stock validation passed for all items")

	// Generate sale code using settings service
	saleCode, err := s.settingsService.GetNextSalesNumber()
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to generate sales code: %v", err)
	}

	// Tax should follow the saved transaction payload.
	// Do not inject a default tax rate from backend settings here.
	ppnPercent := request.PPNPercent
	if ppnPercent == nil {
		if request.PPNRate != nil {
			ppnPercent = request.PPNRate
		} else {
			defaultPPN := 0.0
			ppnPercent = &defaultPPN
		}
	}

	// Create sale with DRAFT status by default
	log.Printf("📝 Creating sale with invoice_type_id: %v", request.InvoiceTypeID)
	sale := &models.Sale{
		Code:            saleCode,
		CustomerID:      request.CustomerID,
		UserID:          userID,
		SalesPersonID:   request.SalesPersonID,
		InvoiceTypeID:   request.InvoiceTypeID,
		ProjectID:       request.ProjectID,
		Type:            request.Type,
		Status:          "DRAFT", // Always start with DRAFT
		Date:            request.Date,
		DueDate:         request.DueDate,
		ValidUntil:      request.ValidUntil,
		Currency:        getOrDefaultStr(request.Currency, "IDR"),
		ExchangeRate:    getOrDefault(request.ExchangeRate, 1.0),
		DiscountPercent: request.DiscountPercent,
		PPNPercent:      *ppnPercent,
		PPhPercent:      request.PPhPercent,
		PPhType:         request.PPhType,
		// Enhanced tax fields
		PPNRate:             *ppnPercent,
		OtherTaxAdditions:   request.OtherTaxAdditions,
		PPh21Rate:           request.PPh21Rate,
		PPh23Rate:           request.PPh23Rate,
		OtherTaxDeductions:  request.OtherTaxDeductions,
		PaymentTerms:        request.PaymentTerms,
		PaymentMethod:       request.PaymentMethod,
		PaymentMethodType:   request.PaymentMethodType, // CASH, BANK, or CREDIT
		CashBankID:          request.CashBankID,
		ReceivableAccountID: request.ReceivableAccountID,
		ShippingMethod:      request.ShippingMethod,
		ShippingCost:        request.ShippingCost,
		ShippingTaxable:     getOrDefault(request.ShippingTaxable, true),
		BillingAddress:      request.BillingAddress,
		ShippingAddress:     request.ShippingAddress,
		Notes:               request.Notes,
		InternalNotes:       request.InternalNotes,
		Reference:           request.Reference,
	}

	if err := validateSalePaymentScheduleMode(sale.PaymentTerms, sale.PaymentMethodType); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := assignAndValidateSaleReceivableAccount(tx, sale, true); err != nil {
		tx.Rollback()
		return nil, err
	}

	sale.DueDate = deriveSaleDueDate(sale.PaymentTerms, sale.DueDate, request.PaymentSchedules)

	remainingProductStock := make(map[uint]int)

	// Process sale items
	for _, itemRequest := range request.Items {
		// Handle discount from frontend (can come as 'discount' or 'discount_percent')
		discountPercent := itemRequest.DiscountPercent
		if discountPercent == nil && itemRequest.Discount != nil {
			discountPercent = itemRequest.Discount
		}
		if discountPercent == nil {
			defaultDiscount := 0.0
			discountPercent = &defaultDiscount
		}

		revenueAccountID := itemRequest.RevenueAccountID
		if revenueAccountID == 0 {
			defaultRevenueAccountID, err := findDefaultRevenueAccountID(tx)
			if err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("revenue account / COA wajib dipilih untuk item '%s': %v", normalizeSaleItemProductName(&itemRequest), err)
			}
			revenueAccountID = defaultRevenueAccountID
			log.Printf("[CreateSale] Item '%s' uses default revenue account ID=%d", normalizeSaleItemProductName(&itemRequest), revenueAccountID)
		}

		// Redirect header account to its leaf child when needed.
		if false && revenueAccountID != 0 {
			var revAcc models.Account
			if err := tx.First(&revAcc, revenueAccountID).Error; err == nil && revAcc.IsHeader {
				log.Printf("⚠️ Revenue account %s (ID=%d) is a HEADER, redirecting to leaf child", revAcc.Code, revAcc.ID)
				var leafAcc models.Account
				if err := tx.Where("parent_id = ? AND is_header = false AND deleted_at IS NULL", revAcc.ID).First(&leafAcc).Error; err == nil {
					revenueAccountID = leafAcc.ID
					log.Printf("✅ Redirected to leaf revenue account %s (ID=%d)", leafAcc.Code, leafAcc.ID)
				} else {
					tx.Rollback()
					return nil, fmt.Errorf("akun COA pendapatan '%s' adalah header dan tidak memiliki akun detail aktif", revAcc.Code)
				}
			} else if err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("revenue account / COA item tidak ditemukan: %v", err)
			}
		}

		normalizedRevenueAccountID, err := normalizeToDetailRevenueAccountID(tx, revenueAccountID)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("revenue account / COA item tidak valid untuk '%s': %v", normalizeSaleItemProductName(&itemRequest), err)
		}
		revenueAccountID = normalizedRevenueAccountID

		var product models.Product
		if err := tx.First(&product, itemRequest.ProductID).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to load product %d: %v", itemRequest.ProductID, err)
		}

		if itemRequest.EstimatedUnitCost != nil && *itemRequest.EstimatedUnitCost < 0 {
			tx.Rollback()
			return nil, fmt.Errorf("estimated HPP tidak boleh negatif untuk item '%s'", normalizeSaleItemProductName(&itemRequest))
		}

		fulfillmentSnapshot := buildSaleItemFulfillmentSnapshotWithRemainingStock(&product, int(itemRequest.Quantity), remainingProductStock)
		applySaleEstimatedUnitCostOverride(&fulfillmentSnapshot, int(itemRequest.Quantity), itemRequest.EstimatedUnitCost)

		item := models.SaleItem{
			ProductID:        itemRequest.ProductID,
			ProductName:      itemRequest.ProductName,
			Description:      itemRequest.Description,
			Location:         itemRequest.Location,
			Quantity:         int(itemRequest.Quantity),
			UnitPrice:        itemRequest.UnitPrice,
			DiscountPercent:  *discountPercent,
			Taxable:          getOrDefault(itemRequest.Taxable, true),
			RevenueAccountID: revenueAccountID,
		}

		// Calculate item totals
		lineTotal := float64(item.Quantity) * item.UnitPrice
		discountAmount := lineTotal * (item.DiscountPercent / 100)
		item.DiscountAmount = discountAmount
		item.LineTotal = lineTotal - discountAmount

		// Calculate taxes if taxable
		if item.Taxable {
			item.PPNAmount = item.LineTotal * (1 - sale.DiscountPercent/100) * (sale.PPNPercent / 100)
			item.PPhAmount = item.LineTotal * (1 - sale.DiscountPercent/100) * (sale.PPhPercent / 100)
		}

		item.TotalTax = item.PPNAmount + item.PPhAmount
		item.FinalAmount = item.LineTotal + item.PPNAmount - item.PPhAmount

		applySaleItemFulfillmentSnapshot(&item, fulfillmentSnapshot)

		sale.SaleItems = append(sale.SaleItems, item)
	}

	applyCalculatedSalesAmounts(sale, sale.SaleItems)

	log.Printf("📊 [CALC] Subtotal=%.2f, Discount=%.2f, TaxableAmt=%.2f", sale.Subtotal, sale.DiscountAmount, sale.TaxableAmount)
	log.Printf("📊 [CALC] PPN=%.2f, Additions=%.2f, Deductions=%.2f, Shipping=%.2f",
		sale.PPN, sale.OtherTaxAdditions, sale.TotalTaxDeductions, sale.ShippingCost)
	log.Printf("📊 [CALC] TotalAmount=%.2f", sale.TotalAmount)

	// Save sale
	if err := tx.Create(&sale).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create sale: %v", err)
	}

	if err := s.syncSalePaymentSchedules(tx, sale, request.PaymentSchedules); err != nil {
		tx.Rollback()
		return nil, err
	}

	// Load relationships
	if err := tx.Preload("Customer").Preload("SaleItems.Product").Preload("PaymentSchedules", func(db *gorm.DB) *gorm.DB {
		return db.Order("sequence ASC")
	}).First(&sale, sale.ID).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to load sale relationships: %v", err)
	}
	hydrateSalePaymentScheduleProgress(sale)

	// IMPORTANT: NO JOURNAL ENTRIES FOR DRAFT STATUS
	// Journal will only be created when status changes to INVOICED or PAID
	log.Printf("✅ Created sale #%d with DRAFT status (no journal posting)", sale.ID)

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	// Recalculate project sales revenue if sale is linked to a project
	if sale.ProjectID != nil && *sale.ProjectID > 0 {
		s.recalculateProjectSalesRevenue(*sale.ProjectID)
	}

	return sale, nil
}

// UpdateSale updates an existing sale
func (s *SalesServiceV2) UpdateSale(saleID uint, request models.SaleUpdateRequest, userID uint) (*models.Sale, error) {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get existing sale
	var sale models.Sale
	if err := tx.Preload("SaleItems").First(&sale, saleID).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("sale not found")
	}

	// Guard: check if sale date is in a closed accounting period
	if err := CheckClosedPeriod(s.db, sale.Date); err != nil {
		tx.Rollback()
		return nil, err
	}

	// Store old status for journal update logic
	oldStatus := sale.Status

	// ✅ GUARD: For non-DRAFT sales, only allow updating non-financial fields
	// Financial fields (items, customer, tax, discount) are locked after invoicing
	if sale.Status != "DRAFT" {
		log.Printf("📝 [UpdateSale] Partial edit mode for %s sale #%d", sale.Status, sale.ID)

		// Block financial field changes
		if request.CustomerID != nil {
			tx.Rollback()
			return nil, fmt.Errorf("tidak bisa mengubah customer pada penjualan yang sudah difakturkan")
		}
		if request.Items != nil {
			tx.Rollback()
			return nil, fmt.Errorf("tidak bisa mengubah item pada penjualan yang sudah difakturkan")
		}
		if request.DiscountPercent != nil {
			tx.Rollback()
			return nil, fmt.Errorf("tidak bisa mengubah diskon pada penjualan yang sudah difakturkan")
		}
		if request.PPNPercent != nil || request.PPNRate != nil || request.PPhPercent != nil || request.PPhType != nil {
			tx.Rollback()
			return nil, fmt.Errorf("tidak bisa mengubah pajak pada penjualan yang sudah difakturkan")
		}
		if request.PPh21Rate != nil || request.PPh23Rate != nil || request.OtherTaxAdditions != nil || request.OtherTaxDeductions != nil {
			tx.Rollback()
			return nil, fmt.Errorf("tidak bisa mengubah pajak pada penjualan yang sudah difakturkan")
		}
		if request.ShippingCost != nil || request.ShippingTaxable != nil {
			tx.Rollback()
			return nil, fmt.Errorf("tidak bisa mengubah biaya atau status pajak pengiriman pada penjualan yang sudah difakturkan")
		}

		// Only allow non-financial field updates
		if request.DueDate != nil {
			sale.DueDate = *request.DueDate
		}
		if request.PaymentTerms != nil {
			sale.PaymentTerms = *request.PaymentTerms
		}
		if request.PaymentMethod != nil {
			sale.PaymentMethod = *request.PaymentMethod
		}
		if request.PaymentMethodType != nil {
			sale.PaymentMethodType = *request.PaymentMethodType
		}
		if request.CashBankID != nil {
			sale.CashBankID = request.CashBankID
		}
		if request.ReceivableAccountID != nil {
			tx.Rollback()
			return nil, fmt.Errorf("tidak bisa mengubah akun piutang pada penjualan yang sudah difakturkan")
		}
		if request.ShippingMethod != nil {
			sale.ShippingMethod = *request.ShippingMethod
		}
		if request.BillingAddress != nil {
			sale.BillingAddress = *request.BillingAddress
		}
		if request.ShippingAddress != nil {
			sale.ShippingAddress = *request.ShippingAddress
		}
		if request.Notes != nil {
			sale.Notes = *request.Notes
		}
		if request.InternalNotes != nil {
			sale.InternalNotes = *request.InternalNotes
		}
		if request.Reference != nil {
			sale.Reference = *request.Reference
		}
		if request.SalesPersonID != nil {
			sale.SalesPersonID = request.SalesPersonID
		}
		if request.ProjectID != nil {
			if *request.ProjectID == 0 {
				sale.ProjectID = nil
			} else {
				sale.ProjectID = request.ProjectID
			}
		}

		if err := validateSalePaymentScheduleMode(sale.PaymentTerms, sale.PaymentMethodType); err != nil {
			tx.Rollback()
			return nil, err
		}

		if request.PaymentSchedules != nil {
			sale.DueDate = deriveSaleDueDate(sale.PaymentTerms, sale.DueDate, request.PaymentSchedules)
		}

		// Save and return (skip item/tax recalculation)
		if err := tx.Save(&sale).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to update sale: %v", err)
		}

		if request.PaymentSchedules != nil || strings.ToUpper(strings.TrimSpace(sale.PaymentTerms)) != "CUSTOM" {
			if err := s.syncSalePaymentSchedules(tx, &sale, request.PaymentSchedules); err != nil {
				tx.Rollback()
				return nil, err
			}
		}

		if err := tx.Commit().Error; err != nil {
			return nil, fmt.Errorf("failed to commit transaction: %v", err)
		}

		// Reload with relationships
		s.db.Preload("Customer").Preload("SaleItems.Product").Preload("InvoiceType").Preload("PaymentSchedules", func(db *gorm.DB) *gorm.DB {
			return db.Order("sequence ASC")
		}).First(&sale, sale.ID)
		hydrateSalePaymentScheduleProgress(&sale)

		log.Printf("✅ [UpdateSale] Partial edit completed for %s sale #%d", sale.Status, sale.ID)
		return &sale, nil
	}

	// === DRAFT-ONLY: Full edit allowed below ===

	// Update fields if provided
	if request.CustomerID != nil {
		sale.CustomerID = *request.CustomerID
	}
	if request.SalesPersonID != nil {
		sale.SalesPersonID = request.SalesPersonID
	}
	if request.ProjectID != nil {
		if *request.ProjectID == 0 {
			sale.ProjectID = nil // Allow unsetting the project
		} else {
			sale.ProjectID = request.ProjectID
		}
	}
	if request.Date != nil {
		sale.Date = *request.Date
	}
	if request.DueDate != nil {
		sale.DueDate = *request.DueDate
	}
	if request.PaymentMethodType != nil {
		sale.PaymentMethodType = *request.PaymentMethodType
	}
	if request.CashBankID != nil {
		sale.CashBankID = request.CashBankID
	}
	if request.ReceivableAccountID != nil {
		sale.ReceivableAccountID = request.ReceivableAccountID
	}

	// Update legacy tax fields
	if request.PPNPercent != nil {
		sale.PPNPercent = *request.PPNPercent
	}
	if request.PPhPercent != nil {
		sale.PPhPercent = *request.PPhPercent
	}
	if request.PPhType != nil {
		sale.PPhType = *request.PPhType
	}

	// Update enhanced tax fields
	if request.PPNRate != nil {
		sale.PPNRate = *request.PPNRate
		// Also sync to legacy field for backward compatibility
		sale.PPNPercent = *request.PPNRate
	}
	if request.OtherTaxAdditions != nil {
		sale.OtherTaxAdditions = *request.OtherTaxAdditions
	}
	if request.PPh21Rate != nil {
		sale.PPh21Rate = *request.PPh21Rate
	}
	if request.PPh23Rate != nil {
		sale.PPh23Rate = *request.PPh23Rate
	}
	if request.OtherTaxDeductions != nil {
		sale.OtherTaxDeductions = *request.OtherTaxDeductions
	}

	// Update other fields
	if request.DiscountPercent != nil {
		sale.DiscountPercent = *request.DiscountPercent
	}
	if request.PaymentTerms != nil {
		sale.PaymentTerms = *request.PaymentTerms
	}
	if request.PaymentMethod != nil {
		sale.PaymentMethod = *request.PaymentMethod
	}
	if request.ShippingMethod != nil {
		sale.ShippingMethod = *request.ShippingMethod
	}
	if request.ShippingCost != nil {
		sale.ShippingCost = *request.ShippingCost
	}
	if request.ShippingTaxable != nil {
		sale.ShippingTaxable = *request.ShippingTaxable
	}
	if request.BillingAddress != nil {
		sale.BillingAddress = *request.BillingAddress
	}
	if request.ShippingAddress != nil {
		sale.ShippingAddress = *request.ShippingAddress
	}
	if request.Notes != nil {
		sale.Notes = *request.Notes
	}
	if request.InternalNotes != nil {
		sale.InternalNotes = *request.InternalNotes
	}
	if request.Reference != nil {
		sale.Reference = *request.Reference
	}

	if err := validateSalePaymentScheduleMode(sale.PaymentTerms, sale.PaymentMethodType); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := assignAndValidateSaleReceivableAccount(tx, &sale, true); err != nil {
		tx.Rollback()
		return nil, err
	}

	if request.PaymentSchedules != nil {
		sale.DueDate = deriveSaleDueDate(sale.PaymentTerms, sale.DueDate, request.PaymentSchedules)
	}

	// Recalculate if items are updated
	if request.Items != nil {
		for i := range request.Items {
			if err := s.prepareSaleItemRequest(tx, &request.Items[i]); err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("invalid sale item #%d: %v", i+1, err)
			}
		}

		// Delete old items
		if err := tx.Where("sale_id = ?", sale.ID).Delete(&models.SaleItem{}).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to delete old items: %v", err)
		}

		// Add new items and recalculate
		remainingProductStock := make(map[uint]int)
		sale.SaleItems = []models.SaleItem{}

		for _, itemRequest := range request.Items {
			// Handle discount from frontend
			discountPercent := itemRequest.DiscountPercent
			if discountPercent == nil && itemRequest.Discount != nil {
				discountPercent = itemRequest.Discount
			}
			if discountPercent == nil {
				defaultDiscount := 0.0
				discountPercent = &defaultDiscount
			}

			revenueAccountID := itemRequest.RevenueAccountID
			if revenueAccountID == 0 {
				defaultRevenueAccountID, err := findDefaultRevenueAccountID(tx)
				if err != nil {
					tx.Rollback()
					return nil, fmt.Errorf("revenue account / COA wajib dipilih untuk item '%s': %v", normalizeSaleItemProductName(&itemRequest), err)
				}
				revenueAccountID = defaultRevenueAccountID
				log.Printf("[UpdateSale] Item '%s' uses default revenue account ID=%d", normalizeSaleItemProductName(&itemRequest), revenueAccountID)
			}

			if false && revenueAccountID != 0 {
				var revAcc models.Account
				if err := tx.First(&revAcc, revenueAccountID).Error; err == nil && revAcc.IsHeader {
					log.Printf("⚠️ [UpdateSale] Revenue account %s (ID=%d) is a HEADER, redirecting to leaf", revAcc.Code, revAcc.ID)
					var leafAcc models.Account
					if err := tx.Where("parent_id = ? AND is_header = false AND deleted_at IS NULL", revAcc.ID).First(&leafAcc).Error; err == nil {
						revenueAccountID = leafAcc.ID
						log.Printf("✅ Redirected to leaf revenue account %s (ID=%d)", leafAcc.Code, leafAcc.ID)
					} else {
						tx.Rollback()
						return nil, fmt.Errorf("akun COA pendapatan '%s' adalah header dan tidak memiliki akun detail aktif", revAcc.Code)
					}
				} else if err != nil {
					tx.Rollback()
					return nil, fmt.Errorf("revenue account / COA item tidak ditemukan: %v", err)
				}
			}

			normalizedRevenueAccountID, err := normalizeToDetailRevenueAccountID(tx, revenueAccountID)
			if err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("revenue account / COA item tidak valid untuk '%s': %v", normalizeSaleItemProductName(&itemRequest), err)
			}
			revenueAccountID = normalizedRevenueAccountID

			var product models.Product
			if err := tx.First(&product, itemRequest.ProductID).Error; err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to load product %d: %v", itemRequest.ProductID, err)
			}

			if itemRequest.EstimatedUnitCost != nil && *itemRequest.EstimatedUnitCost < 0 {
				tx.Rollback()
				return nil, fmt.Errorf("estimated HPP tidak boleh negatif untuk item '%s'", normalizeSaleItemProductName(&itemRequest))
			}

			fulfillmentSnapshot := buildSaleItemFulfillmentSnapshotWithRemainingStock(&product, int(itemRequest.Quantity), remainingProductStock)
			applySaleEstimatedUnitCostOverride(&fulfillmentSnapshot, int(itemRequest.Quantity), itemRequest.EstimatedUnitCost)

			item := models.SaleItem{
				SaleID:           sale.ID,
				ProductID:        itemRequest.ProductID,
				ProductName:      itemRequest.ProductName,
				Description:      itemRequest.Description,
				Location:         itemRequest.Location,
				Quantity:         int(itemRequest.Quantity),
				UnitPrice:        itemRequest.UnitPrice,
				DiscountPercent:  *discountPercent,
				Taxable:          getOrDefault(itemRequest.Taxable, true),
				RevenueAccountID: revenueAccountID,
			}

			// Calculate item totals
			lineTotal := float64(item.Quantity) * item.UnitPrice
			discountAmount := lineTotal * (item.DiscountPercent / 100)
			item.DiscountAmount = discountAmount
			item.LineTotal = lineTotal - discountAmount

			if item.Taxable {
				// Use PPNRate if available, otherwise fall back to PPNPercent
				ppnRate := sale.PPNRate
				if ppnRate == 0 {
					ppnRate = sale.PPNPercent
				}
				item.PPNAmount = item.LineTotal * (1 - sale.DiscountPercent/100) * (ppnRate / 100)
				item.PPhAmount = item.LineTotal * (1 - sale.DiscountPercent/100) * (sale.PPhPercent / 100)
			}

			item.TotalTax = item.PPNAmount + item.PPhAmount
			item.FinalAmount = item.LineTotal + item.PPNAmount - item.PPhAmount

			applySaleItemFulfillmentSnapshot(&item, fulfillmentSnapshot)

			sale.SaleItems = append(sale.SaleItems, item)
		}

		applyCalculatedSalesAmounts(&sale, sale.SaleItems)

		log.Printf("📊 [UPDATE] TotalAmount=%.2f, Outstanding=%.2f", sale.TotalAmount, sale.OutstandingAmount)
	} else {
		applyCalculatedSalesAmounts(&sale, sale.SaleItems)

		log.Printf("📊 [UPDATE-TAX-ONLY] TaxableAmt=%.2f, PPh23=%.2f, Deductions=%.2f, TotalAmount=%.2f",
			sale.TaxableAmount, sale.PPh23Amount, sale.TotalTaxDeductions, sale.TotalAmount)
	}

	// Save updated sale
	if err := tx.Save(&sale).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update sale: %v", err)
	}

	if request.PaymentSchedules != nil || strings.ToUpper(strings.TrimSpace(sale.PaymentTerms)) != "CUSTOM" {
		if err := s.syncSalePaymentSchedules(tx, &sale, request.PaymentSchedules); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// Handle journal updates based on status
	if err := s.salesJournalService.UpdateSalesJournal(&sale, oldStatus, tx); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update journal: %v", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	// Reload with relationships
	s.db.Preload("Customer").Preload("SaleItems.Product").Preload("PaymentSchedules", func(db *gorm.DB) *gorm.DB {
		return db.Order("sequence ASC")
	}).First(&sale, sale.ID)
	hydrateSalePaymentScheduleProgress(&sale)

	// Recalculate project sales revenue if sale is linked to a project
	if sale.ProjectID != nil && *sale.ProjectID > 0 {
		s.recalculateProjectSalesRevenue(*sale.ProjectID)
	}

	return &sale, nil
}

// ConfirmSale changes sale status from DRAFT to CONFIRMED
func (s *SalesServiceV2) ConfirmSale(saleID uint, userID uint) (*models.Sale, error) {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var sale models.Sale
	if err := tx.First(&sale, saleID).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("sale not found")
	}

	if sale.Status != "DRAFT" {
		tx.Rollback()
		return nil, fmt.Errorf("only DRAFT sales can be confirmed")
	}

	_ = sale.Status // oldStatus not needed for CONFIRMED
	sale.Status = "CONFIRMED"
	sale.UpdatedAt = time.Now()

	if err := tx.Save(&sale).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to confirm sale: %v", err)
	}

	// NO JOURNAL for CONFIRMED status - still no accounting impact
	log.Printf("✅ Sale #%d confirmed (status: DRAFT → CONFIRMED) - No journal posting", sale.ID)

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	// Reload sale with relationships after commit for proper JSON marshalling
	if err := s.db.Preload("Customer").Preload("SaleItems.Product").First(&sale, sale.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to reload sale after commit: %v", err)
	}

	return &sale, nil
}

// CreateInvoice changes sale status to INVOICED and generates invoice number
func (s *SalesServiceV2) CreateInvoice(saleID uint, userID uint) (*models.Sale, error) {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var sale models.Sale
	if err := tx.Preload("Customer").Preload("SaleItems").First(&sale, saleID).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("sale not found")
	}

	// ✅ CRITICAL FIX: Prevent duplicate invoice creation
	// Check if invoice already created (has invoice number or status is INVOICED/PAID)
	if err := CheckClosedPeriod(tx, sale.Date); err != nil {
		tx.Rollback()
		return nil, err
	}

	if sale.InvoiceNumber != "" || sale.Status == "INVOICED" || sale.Status == "PAID" {
		tx.Rollback()
		return nil, fmt.Errorf("invoice already created for this sale (Invoice #%s, Status: %s)",
			sale.InvoiceNumber, sale.Status)
	}

	if sale.Status != "CONFIRMED" && sale.Status != "DRAFT" {
		tx.Rollback()
		return nil, fmt.Errorf("only DRAFT or CONFIRMED sales can be invoiced (current status: %s)", sale.Status)
	}

	log.Printf("ðŸ” Recalculating fulfillment snapshot before invoice for Sale #%d", sale.ID)
	remainingProductStock := make(map[uint]int)
	for idx := range sale.SaleItems {
		item := &sale.SaleItems[idx]

		var product models.Product
		if err := tx.First(&product, item.ProductID).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to load product %d: %v", item.ProductID, err)
		}

		snapshot := buildSaleItemFulfillmentSnapshotWithRemainingStock(&product, item.Quantity, remainingProductStock)
		if item.EstimatedUnitCost > 0 {
			estimatedUnitCost := item.EstimatedUnitCost
			applySaleEstimatedUnitCostOverride(&snapshot, item.Quantity, &estimatedUnitCost)
		}
		applySaleItemFulfillmentSnapshot(item, snapshot)

		if err := tx.Model(&models.SaleItem{}).Where("id = ?", item.ID).Updates(map[string]interface{}{
			"available_quantity":    item.AvailableQuantity,
			"fulfilled_quantity":    item.FulfilledQuantity,
			"backorder_quantity":    item.BackorderQuantity,
			"estimated_unit_cost":   item.EstimatedUnitCost,
			"estimated_cogs_amount": item.EstimatedCOGSAmount,
			"actual_cogs_amount":    item.ActualCOGSAmount,
			"cogs_variance_amount":  item.COGSVarianceAmount,
			"fulfillment_status":    item.FulfillmentStatus,
			"updated_at":            time.Now(),
		}).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to refresh fulfillment snapshot for sale item %d: %v", item.ID, err)
		}
	}

	// ✅ VALIDATE STOCK BEFORE INVOICE (Early Check)
	// Prevent transaction rollback in the middle of journal creation
	log.Printf("🔍 Validating stock availability before invoice for Sale #%d", sale.ID)
	for _, item := range []models.SaleItem{} {
		var product models.Product
		if err := tx.First(&product, item.ProductID).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to load product %d: %v", item.ProductID, err)
		}

		// Skip stock check for service products and assembly products
		if product.IsService || product.IsAssembly {
			log.Printf("ℹ️ Product %d (%s) is a service or assembly, skipping direct stock validation", product.ID, product.Name)
			continue
		}

		// Check stock availability BEFORE starting any journal/COGS process
		if product.Stock == 0 {
			tx.Rollback()
			return nil, fmt.Errorf("stock tidak cukup untuk product '%s'. Tersedia: %d, Diminta: %d (Stock habis, tidak bisa membuat invoice)",
				product.Name, product.Stock, item.Quantity)
		}

		if product.Stock < item.Quantity {
			tx.Rollback()
			return nil, fmt.Errorf("stock tidak cukup untuk product '%s'. Tersedia: %d, Diminta: %d",
				product.Name, product.Stock, item.Quantity)
		}

		log.Printf("✅ Stock check passed: %s (Available: %d, Required: %d)",
			product.Name, product.Stock, item.Quantity)
	}
	log.Printf("✅ All stock validations passed for Sale #%d", sale.ID)

	oldStatus := sale.Status
	sale.Status = "INVOICED"
	applyCalculatedSalesAmounts(&sale, sale.SaleItems)
	if err := assignAndValidateSaleReceivableAccount(tx, &sale, false); err != nil {
		tx.Rollback()
		return nil, err
	}

	// Generate invoice number using new service
	if sale.InvoiceTypeID != nil {
		log.Printf("🔧 Generating invoice number for sale #%d with invoice type ID: %d", sale.ID, *sale.InvoiceTypeID)
		invoiceResp, err := s.invoiceNumberService.GenerateInvoiceNumber(*sale.InvoiceTypeID, sale.Date)
		if err != nil {
			log.Printf("❌ Failed to generate invoice number with type ID %d: %v", *sale.InvoiceTypeID, err)
			tx.Rollback()
			return nil, fmt.Errorf("failed to generate invoice number: %v", err)
		}
		sale.InvoiceNumber = invoiceResp.InvoiceNumber
		log.Printf("✅ Generated invoice number: %s (Counter: %d, Type: %s)",
			invoiceResp.InvoiceNumber, invoiceResp.Counter, invoiceResp.TypeCode)
	} else {
		// Fallback to old method if no invoice type specified
		log.Printf("⚠️ No invoice type specified for sale #%d, using fallback method", sale.ID)
		sale.InvoiceNumber = s.generateInvoiceNumber()
		log.Printf("📄 Generated fallback invoice number: %s", sale.InvoiceNumber)
	}

	sale.UpdatedAt = time.Now()

	if err := tx.Save(&sale).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create invoice: %v", err)
	}

	// CREATE JOURNAL ENTRIES for INVOICED status using SSOT service
	// This writes to unified_journal_ledger which is read by Balance Sheet
	// ✅ IMPORTANT: SSOT service ALREADY INCLUDES COGS posting, so we don't need separate COGS service call
	if s.salesJournalServiceSSOT != nil {
		if err := s.salesJournalServiceSSOT.CreateSalesJournal(&sale, tx); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create SSOT journal entries: %v", err)
		}
		log.Printf("✅ [SSOT] Journal entries created (including COGS) for Sale #%d", sale.ID)
	} else {
		// Fallback to legacy service if SSOT service not available
		log.Printf("⚠️ SSOT journal service not available, using legacy service")
		if err := s.salesJournalService.CreateSalesJournal(&sale, tx); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create journal entries: %v", err)
		}

		// 💰 ONLY CREATE COGS IF USING LEGACY SERVICE
		// Legacy service doesn't include COGS, so we need to create it separately
		// SSOT service already includes COGS, so we skip this to prevent DOUBLE POSTING
		if s.cogsService != nil {
			log.Printf("💰 [Auto-COGS] Recording COGS for Sale #%d (legacy mode)", sale.ID)
			if err := s.cogsService.RecordCOGSForSale(&sale, tx); err != nil {
				// Log warning but don't fail the transaction - COGS can be backfilled later
				log.Printf("⚠️ Warning: Failed to auto-record COGS for sale #%d: %v", sale.ID, err)
				log.Printf("   COGS can be backfilled later using: go run cmd/scripts/backfill_cogs_entries.go")
			} else {
				log.Printf("✅ [Auto-COGS] Successfully recorded COGS for Sale #%d", sale.ID)
			}
		}
	}

	// Auto-mark as PAID for immediate payment methods (CASH/BANK)
	pm := strings.ToUpper(strings.TrimSpace(sale.PaymentMethodType))
	if pm == "CASH" || strings.HasPrefix(pm, "BANK") {
		// No additional payment journal is created here to avoid double posting,
		// because CreateSalesJournal already debits Cash/Bank for CASH/BANK.
		sale.PaidAmount = sale.TotalAmount
		sale.OutstandingAmount = 0
		sale.Status = "PAID"
		if err := tx.Save(&sale).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to auto-set sale as PAID: %v", err)
		}

		// ✅ REMOVED MANUAL CASH/BANK UPDATE - Now handled by auto-sync in journal service
		// The SalesJournalServiceSSOT.syncCashBankBalance() automatically syncs cash_banks.balance
		// with accounts.balance after each journal line is posted, ensuring consistency.

		// Create cash/bank transaction record for audit trail
		if sale.CashBankID != nil && *sale.CashBankID > 0 {
			var cashBankForBalance models.CashBank
			if err := tx.First(&cashBankForBalance, *sale.CashBankID).Error; err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to load cash/bank for immediate payment sale: %v", err)
			}

			mutation, err := buildImmediateSaleCashBankMutation(&sale, &cashBankForBalance)
			if err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to prepare immediate payment cash/bank mutation: %v", err)
			}

			balanceAfter := cashBankForBalance.Balance
			if cashBankForBalance.IsForeignCurrency() {
				balanceAfter += mutation.NativeAmount
			}

			guard := NewCashBankCurrencyGuard(s.db)
			cashBankTransaction := &models.CashBankTransaction{
				CashBankID:      *sale.CashBankID,
				ReferenceType:   "SALE",
				ReferenceID:     sale.ID,
				Amount:          mutation.NativeAmount,
				Currency:        mutation.Currency,
				ExchangeRate:    mutation.ExchangeRate,
				BookAmountIDR:   mutation.BookAmountIDR,
				BalanceAfter:    balanceAfter,
				TransactionDate: sale.Date,
				Notes:           mutation.Notes,
			}

			if err := tx.Create(cashBankTransaction).Error; err != nil {
				log.Printf("⚠️ Warning: Failed to create cash/bank transaction record: %v", err)
				// Don't fail transaction, this is just for audit trail
			} else if err := guard.EnforceAfterTransaction(tx, *sale.CashBankID); err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to enforce cash/bank currency invariant after sale payment: %v", err)
			}
		}

		log.Printf("💡 Auto-PAID applied: Sale #%d marked as PAID for payment method %s", sale.ID, sale.PaymentMethodType)
		log.Printf("✅ Sale #%d invoiced (status: %s → PAID) - Journal posted, cash/bank auto-synced", sale.ID, oldStatus)
	} else {
		log.Printf("✅ Sale #%d invoiced (status: %s → INVOICED) - Journal posted", sale.ID, oldStatus)
	}

	// Update stock - CRITICAL: Block transaction if stock insufficient
	affectedProductIDs := make(map[uint]struct{})
	if s.stockService != nil {
		for _, item := range sale.SaleItems {
			// Load product to check if it's a service
			var product models.Product
			if err := tx.First(&product, item.ProductID).Error; err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to load product %d: %v", item.ProductID, err)
			}

			// Skip stock check for service products
			if product.IsService {
				log.Printf("ℹ️ Product %d (%s) is a service, skipping stock reduction", product.ID, product.Name)
				continue
			}

			// 🚀 AUTO-KITTING LOGIC: If it's an assembly, reduce component stock instead
			if product.IsAssembly {
				log.Printf("⚙️ Product %d (%s) is an assembly, performing auto-kitting...", product.ID, product.Name)

				var bom models.BOM
				if err := tx.Preload("Items.ComponentProduct").Where("target_product_id = ?", product.ID).First(&bom).Error; err != nil {
					tx.Rollback()
					return nil, fmt.Errorf("Resep (BOM) tidak ditemukan untuk produk rakitan '%s' (ID: %d): tidak bisa melakukan auto-kitting", product.Name, product.ID)
				}

				for _, bomItem := range bom.Items {
					// Calculate required quantity (bom ratio * sold quantity)
					// Handle as int since stock is int
					requiredQty := int(bomItem.RequirementQuantity * float64(item.Quantity))
					if requiredQty <= 0 {
						continue
					}

					// Ensure component stock is sufficient (optional early check, but ReduceStock will also check)
					if bomItem.ComponentProduct.Stock < requiredQty {
						tx.Rollback()
						return nil, fmt.Errorf("Stock bahan baku '%s' tidak cukup untuk merakit '%s'. Tersedia: %d, Dibutuhkan: %d",
							bomItem.ComponentProduct.Name, product.Name, bomItem.ComponentProduct.Stock, requiredQty)
					}

					if err := s.stockService.ReduceStock(bomItem.ComponentProductID, requiredQty, tx); err != nil {
						tx.Rollback()
						return nil, fmt.Errorf("Gagal memotong stok komponen '%s' untuk perakitan '%s': %v", bomItem.ComponentProduct.Name, product.Name, err)
					}
					affectedProductIDs[bomItem.ComponentProductID] = struct{}{}

					log.Printf("✅ Kitting: Stock reduced for component %d (%s): %d → %d",
						bomItem.ComponentProductID, bomItem.ComponentProduct.Name, bomItem.ComponentProduct.Stock, bomItem.ComponentProduct.Stock-requiredQty)
				}
				log.Printf("✅ Auto-kitting completed for assembly %d (%s)", product.ID, product.Name)
				continue
			}

			if item.AvailableQuantity <= 0 {
				log.Printf("â„¹ï¸ Product %d (%s) has no stock to reduce on invoice; qty %d kept as backorder", product.ID, product.Name, item.BackorderQuantity)
				continue
			}

			// Check stock availability BEFORE reducing
			if false && product.Stock < item.Quantity {
				tx.Rollback()
				return nil, fmt.Errorf("stock tidak cukup untuk product '%s'. Tersedia: %d, Diminta: %d",
					product.Name, product.Stock, item.Quantity)
			}

			// Check if stock is zero
			if false && product.Stock == 0 {
				tx.Rollback()
				return nil, fmt.Errorf("product '%s' habis (stock = 0), tidak bisa dijual", product.Name)
			}

			// Reduce stock
			if err := s.stockService.ReduceStock(item.ProductID, item.AvailableQuantity, tx); err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("gagal mengurangi stock untuk product '%s': %v", product.Name, err)
			}
			affectedProductIDs[item.ProductID] = struct{}{}

			log.Printf("✅ Stock reduced for product %d (%s): %d → %d",
				product.ID, product.Name, product.Stock, product.Stock-item.AvailableQuantity)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	// Reload sale with all relationships after commit for proper JSON marshalling
	if err := s.db.Preload("Customer").Preload("SaleItems.Product").First(&sale, sale.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to reload sale after commit: %v", err)
	}

	// Recalculate project sales revenue if sale is linked to a project
	if sale.ProjectID != nil && *sale.ProjectID > 0 {
		s.recalculateProjectSalesRevenue(*sale.ProjectID)
	}

	s.syncStockAlertsForProducts(affectedProductIDs)

	return &sale, nil
}

// ProcessPayment records a payment for the sale
func (s *SalesServiceV2) ProcessPayment(saleID uint, paymentRequest models.SalePaymentRequest, userID uint) (*models.SalePayment, error) {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var sale models.Sale
	if err := tx.First(&sale, saleID).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("sale not found")
	}

	// Only allow payment for INVOICED sales
	if sale.Status != "INVOICED" && sale.Status != "PAID" {
		tx.Rollback()
		return nil, fmt.Errorf("payment can only be made for INVOICED sales")
	}

	// ✅ ACCOUNTING GUARD: Prevent over-payment (piutang tidak boleh negatif)
	// If sale is already fully paid, reject
	if sale.OutstandingAmount <= 0 {
		tx.Rollback()
		return nil, fmt.Errorf("penjualan ini sudah lunas (outstanding: Rp 0), tidak bisa menerima pembayaran lagi")
	}

	// Cap payment amount to outstanding to prevent negative receivables
	actualAmount := paymentRequest.Amount
	if actualAmount > sale.OutstandingAmount {
		log.Printf("⚠️ [OVER-PAY] Payment amount %.2f exceeds outstanding %.2f for Sale #%d. Capping to outstanding.",
			actualAmount, sale.OutstandingAmount, sale.ID)
		actualAmount = sale.OutstandingAmount
	}

	bankCurrency := "IDR"
	bankExchangeRate := 1.0
	bankNativeAmount := actualAmount
	bankBookAmountIDR := actualAmount
	receivableSettlementBookIDR, _, err := calculateSaleReceivableSettlementBookAmount(&sale, actualAmount, paymentRequest.ExchangeRate)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to calculate receivable historical book amount: %v", err)
	}
	forexGainLossAmount := 0.0
	forexGainLossType := ""
	var selectedCashBankSnapshot *cashBankSnapshot

	var cashBank *models.CashBank
	if paymentRequest.CashBankID != nil && *paymentRequest.CashBankID > 0 {
		var loadedCashBank models.CashBank
		if err := tx.First(&loadedCashBank, *paymentRequest.CashBankID).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("cash/bank account not found: %v", err)
		}

		bankCurrency = normalizeCashBankCurrency(loadedCashBank.Currency)
		if bankCurrency == "USD" {
			if paymentRequest.ExchangeRate <= 0 {
				tx.Rollback()
				return nil, fmt.Errorf("exchange rate is required when receiving payment into USD cash/bank account")
			}
			bankExchangeRate = paymentRequest.ExchangeRate
			bankNativeAmount = actualAmount / paymentRequest.ExchangeRate
			bankBookAmountIDR = actualAmount
		}

		cashBank = &loadedCashBank
		snapshot, snapshotErr := loadCashBankSnapshot(tx, *paymentRequest.CashBankID)
		if snapshotErr != nil {
			tx.Rollback()
			return nil, snapshotErr
		}
		selectedCashBankSnapshot = snapshot
	}

	forexGainLossAmount, forexGainLossType = classifyReceivableRealizedFX(bankBookAmountIDR, receivableSettlementBookIDR)

	// Create payment record
	payment := &models.SalePayment{
		SaleID:              saleID,
		PaymentDate:         paymentRequest.PaymentDate,
		Amount:              actualAmount,
		NativeAmount:        bankNativeAmount,
		Currency:            bankCurrency,
		ExchangeRate:        bankExchangeRate,
		BookAmountIDR:       bankBookAmountIDR,
		ForexGainLossAmount: forexGainLossAmount,
		ForexGainLossType:   forexGainLossType,
		PaymentMethod:       paymentRequest.PaymentMethod,
		Reference:           paymentRequest.Reference,
		CashBankID:          paymentRequest.CashBankID,
		Notes:               paymentRequest.Notes,
		UserID:              userID,
	}
	applyCashBankSnapshotToSalePayment(payment, selectedCashBankSnapshot)

	if err := tx.Create(&payment).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create payment: %v", err)
	}

	// Update sale payment amounts
	sale.PaidAmount += payment.Amount
	sale.OutstandingAmount = sale.TotalAmount - sale.PaidAmount

	// Update status if fully paid
	if sale.OutstandingAmount <= 0 {
		sale.Status = "PAID"
		sale.OutstandingAmount = 0
	}
	sale.SettlementStatus = models.ResolveSaleSettlementStatus(sale.Status, sale.PaidAmount, sale.OutstandingAmount)

	if err := tx.Save(&sale).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update sale: %v", err)
	}
	if err := syncSalePaymentScheduleProgress(tx, &sale); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to sync sale payment schedules: %v", err)
	}

	// Create payment journal entries
	if err := s.salesJournalService.CreateSalesPaymentJournal(payment, &sale, tx); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create payment journal: %v", err)
	}

	// ✅ CRITICAL FIX: Update cash_banks.balance for Cash & Bank Management view
	// This is SEPARATE from accounts.balance (COA tree view) which is updated by journal entries
	if cashBank != nil && payment.CashBankID != nil && *payment.CashBankID > 0 {
		// Increase cash/bank balance for payment IN (customer paying us)
		cashBank.Balance += payment.NativeAmount
		if cashBank.IsForeignCurrency() {
			cashBank.BookBalanceIDR += payment.BookAmountIDR
			if payment.ExchangeRate > 0 {
				cashBank.LastExchangeRate = payment.ExchangeRate
			}
		}
		if err := tx.Save(cashBank).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to update cash/bank balance: %v", err)
		}
		if cashBank.AccountID > 0 {
			if err := tx.Exec("UPDATE accounts SET balance = ?, updated_at = NOW() WHERE id = ? AND deleted_at IS NULL",
				cashBank.EffectiveBookBalanceIDR(), cashBank.AccountID).Error; err != nil {
				log.Printf("⚠️ Warning: failed to sync GL balance for cash/bank account %d: %v", cashBank.AccountID, err)
			}
		}

		// Create cash/bank transaction record for audit trail
		guard := NewCashBankCurrencyGuard(s.db)
		cashBankTransaction := &models.CashBankTransaction{
			CashBankID:      *payment.CashBankID,
			ReferenceType:   "SALES_PAYMENT",
			ReferenceID:     payment.ID,
			Amount:          payment.NativeAmount, // Positive for incoming payment
			Currency:        bankCurrency,
			ExchangeRate:    payment.ExchangeRate,
			BookAmountIDR:   payment.BookAmountIDR,
			BalanceAfter:    cashBank.Balance,
			TransactionDate: payment.PaymentDate,
			Notes:           fmt.Sprintf("Customer payment - Invoice #%s", sale.InvoiceNumber),
		}

		if err := tx.Create(cashBankTransaction).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create cash/bank transaction: %v", err)
		}

		if err := guard.EnforceAfterTransaction(tx, *payment.CashBankID); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to enforce cash/bank currency invariant after customer payment: %v", err)
		}

		log.Printf("💰 Updated Cash/Bank #%d balance: +%.2f (new balance: %.2f)",
			*payment.CashBankID, payment.NativeAmount, cashBank.Balance)
	} else {
		log.Printf("⚠️ Warning: No CashBankID provided for payment #%d, cash_banks balance not updated", payment.ID)
	}

	log.Printf("✅ Payment #%d created for Sale #%d (Amount: %.2f)", payment.ID, sale.ID, payment.Amount)

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	// Recalculate project sales revenue if sale is linked to a project
	if sale.ProjectID != nil && *sale.ProjectID > 0 {
		s.recalculateProjectSalesRevenue(*sale.ProjectID)
	}

	return payment, nil
}

func (s *SalesServiceV2) SettlePPh23(saleID uint, request models.SalePPh23SettlementRequest, userID uint) (*models.Sale, error) {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var sale models.Sale
	if err := tx.First(&sale, saleID).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("sale not found")
	}

	if sale.Status == models.SaleStatusCancelled {
		tx.Rollback()
		return nil, fmt.Errorf("penjualan yang dibatalkan tidak bisa dipost PPh23")
	}

	if sale.PPh23Amount <= 0 {
		tx.Rollback()
		return nil, fmt.Errorf("invoice ini tidak memiliki PPh23 untuk dipost")
	}

	if sale.PPh23SettlementJournalEntryID != nil || sale.PPh23SettlementDate != nil {
		tx.Rollback()
		return nil, fmt.Errorf("PPh23 untuk invoice ini sudah pernah dipost")
	}

	taxHelper := NewTaxAccountHelper(tx)

	pph23CreditAccount, err := taxHelper.GetPPh23TaxCreditAccount(tx)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("akun Kredit Pajak PPh23 tidak ditemukan: %v", err)
	}

	receivableSettlementAmount := 0.0
	prepaidReclassAmount := roundScheduleMoney(sale.PPh23Amount)
	if request.ReduceOutstanding {
		reductionAmount, reductionErr := applySalePPh23OutstandingReduction(&sale, sale.PPh23Amount)
		if reductionErr != nil {
			tx.Rollback()
			return nil, reductionErr
		}
		receivableSettlementAmount = reductionAmount
		prepaidReclassAmount = roundScheduleMoney(sale.PPh23Amount - receivableSettlementAmount)
	}

	var pph23PrepaidAccount *models.Account
	if prepaidReclassAmount > 0.005 {
		pph23PrepaidAccount, err = taxHelper.GetWithholdingTax23Account(tx)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("akun sumber PPh23 tidak ditemukan: %v", err)
		}
	}

	var receivableAccount *models.Account
	if receivableSettlementAmount > 0.005 {
		receivableAccount, err = resolveSalesReceivableAccount(tx, &sale)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	referenceNumber := strings.TrimSpace(sale.InvoiceNumber)
	if referenceNumber == "" {
		referenceNumber = strings.TrimSpace(sale.Code)
	}

	journalReference := strings.TrimSpace(request.Reference)
	if journalReference == "" {
		journalReference = fmt.Sprintf("PPH23-%s", referenceNumber)
	}

	journalLines := []JournalLineRequest{
		{
			AccountID:    uint64(pph23CreditAccount.ID),
			DebitAmount:  decimal.NewFromFloat(sale.PPh23Amount),
			CreditAmount: decimal.Zero,
			Description:  fmt.Sprintf("Kredit Pajak PPh23 - %s", referenceNumber),
		},
	}
	if prepaidReclassAmount > 0.005 {
		journalLines = append(journalLines, JournalLineRequest{
			AccountID:    uint64(pph23PrepaidAccount.ID),
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.NewFromFloat(prepaidReclassAmount),
			Description:  fmt.Sprintf("Reclass PPh23 Dibayar Dimuka - %s", referenceNumber),
		})
	}
	if receivableSettlementAmount > 0.005 {
		journalLines = append(journalLines, JournalLineRequest{
			AccountID:    uint64(receivableAccount.ID),
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.NewFromFloat(receivableSettlementAmount),
			Description:  fmt.Sprintf("Kurangi Piutang Usaha untuk PPh23 - %s", referenceNumber),
		})
	}

	journalService := NewUnifiedJournalService(tx)
	adjustmentOf := findPostedSourceJournalIDForAdjustment(tx, models.SSOTSourceTypeSale, uint64(sale.ID), request.Date)
	journalRequest := &JournalEntryRequest{
		EntryDate:              request.Date,
		Reference:              journalReference,
		Description:            fmt.Sprintf("Posting PPh23 Sales - %s", referenceNumber),
		Notes:                  strings.TrimSpace(request.Notes),
		SourceType:             models.SSOTSourceTypeAdjustment,
		SourceID:               uint64(sale.ID),
		AdjustmentOf:           adjustmentOf,
		AdjustmentCategoryCode: models.JournalAdjustmentCategorySalesPPh23Settlement,
		AutoPost:               true,
		SystemGenerated:        true,
		CreatedBy:              uint64(userID),
		Lines:                  journalLines,
	}

	journalEntry, err := journalService.CreateJournalEntryWithTx(tx, journalRequest)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create PPh23 journal: %v", err)
	}

	journalEntryID := uint(journalEntry.ID)
	settlementDate := request.Date
	sale.PPh23SettlementDate = &settlementDate
	sale.PPh23SettlementReference = journalReference
	sale.PPh23SettlementNotes = strings.TrimSpace(request.Notes)
	sale.PPh23SettlementJournalEntryID = &journalEntryID
	sale.PPh23SettlementUserID = &userID

	if err := tx.Save(&sale).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update sale PPh23 settlement: %v", err)
	}
	if err := rebalanceExistingSalePaymentSchedules(tx, &sale); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := syncSalePaymentScheduleProgress(tx, &sale); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to sync sale payment schedules: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit PPh23 settlement: %v", err)
	}

	return s.GetSaleByID(saleID)
}

// CancelSale cancels a sale
func (s *SalesServiceV2) CancelSale(saleID uint, reason string, userID uint) error {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var sale models.Sale
	if err := tx.Preload("SaleItems.Product").First(&sale, saleID).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("sale not found")
	}

	if err := CheckClosedPeriod(s.db, sale.Date); err != nil {
		tx.Rollback()
		return err
	}

	oldStatus := sale.Status
	sale.Status = "CANCELLED"
	sale.InternalNotes = fmt.Sprintf("Cancelled: %s", reason)
	sale.UpdatedAt = time.Now()

	log.Printf("Resetting payment tracking for cancelled Sale #%d (Outstanding: %.2f->0, Paid: %.2f->0)",
		sale.ID, sale.OutstandingAmount, sale.PaidAmount)
	sale.OutstandingAmount = 0
	sale.PaidAmount = 0

	if err := tx.Save(&sale).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to cancel sale: %v", err)
	}

	if s.salesJournalService.ShouldPostToJournal(oldStatus) {
		if err := s.salesJournalService.DeleteSalesJournal(sale.ID, tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to remove V2 journal entries: %v", err)
		}
		log.Printf("Removed V2 SALES journal entries for cancelled Sale #%d", sale.ID)

		if s.salesJournalServiceSSOT != nil {
			if err := s.salesJournalServiceSSOT.DeleteSalesJournal(sale.ID, tx); err != nil {
				log.Printf("Warning: Failed to remove SSOT journal entries for Sale #%d: %v", sale.ID, err)
			} else {
				log.Printf("Removed SSOT SALE journal entries for cancelled Sale #%d", sale.ID)
			}
		}
	}

	var salePayments []models.SalePayment
	affectedCashBankIDs := make(map[uint]struct{})
	if err := tx.Where("sale_id = ?", sale.ID).Find(&salePayments).Error; err == nil && len(salePayments) > 0 {
		log.Printf("Found %d payment records for cancelled Sale #%d, cleaning up...", len(salePayments), sale.ID)
		for _, payment := range salePayments {
			sourceID := uint64(payment.ID)
			var paymentJournal models.SSOTJournalEntry
			if err := tx.Where("source_type = ? AND source_id = ?", "SALES_PAYMENT", sourceID).
				First(&paymentJournal).Error; err == nil {
				var journalLines []models.SSOTJournalLine
				if err := tx.Where("journal_id = ?", paymentJournal.ID).Find(&journalLines).Error; err == nil {
					for _, line := range journalLines {
						negDebit := line.DebitAmount.Neg()
						negCredit := line.CreditAmount.Neg()
						if err := s.salesJournalService.updateAccountBalanceForPayment(tx, line.AccountID, negDebit, negCredit); err != nil {
							log.Printf("Warning: Failed to reverse COA balance for payment journal account %d: %v", line.AccountID, err)
						}
						if err := s.salesJournalService.updateParentAccountBalances(tx, uint(line.AccountID)); err != nil {
							log.Printf("Warning: Failed to update parent balances for account %d: %v", line.AccountID, err)
						}
					}
				}

				tx.Where("journal_id = ?", paymentJournal.ID).Delete(&models.SSOTJournalLine{})
				tx.Delete(&paymentJournal)
				log.Printf("Deleted SALES_PAYMENT journal for payment #%d", payment.ID)
			}

			if payment.CashBankID != nil && *payment.CashBankID > 0 {
				affectedCashBankIDs[*payment.CashBankID] = struct{}{}
			}

			if err := s.reverseSaleCashBankMovement(tx, "SALES_PAYMENT", payment.ID, fmt.Sprintf("Delete sale %s - %s", sale.Code, reason)); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to reverse SALES_PAYMENT cash/bank movement for payment #%d: %v", payment.ID, err)
			}

			payment.Status = "CANCELLED"
			payment.Notes = strings.TrimSpace(payment.Notes + "\nCancelled because sale was deleted: " + reason)
			if err := tx.Save(&payment).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to mark sale payment #%d as cancelled: %v", payment.ID, err)
			}
		}
	}

	pm := strings.ToUpper(strings.TrimSpace(sale.PaymentMethodType))
	if (oldStatus == "INVOICED" || oldStatus == "PAID") && (pm == "CASH" || strings.HasPrefix(pm, "BANK")) {
		if sale.CashBankID != nil && *sale.CashBankID > 0 {
			affectedCashBankIDs[*sale.CashBankID] = struct{}{}
		}

		if err := s.reverseSaleCashBankMovement(tx, "SALE", sale.ID, fmt.Sprintf("Delete sale %s - %s", sale.Code, reason)); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to reverse SALE cash/bank movement: %v", err)
		}
	}

	if err := s.enforceCashBankGuards(tx, affectedCashBankIDs); err != nil {
		tx.Rollback()
		return err
	}

	affectedProductIDs := make(map[uint]struct{})
	if s.stockService != nil && (oldStatus == "INVOICED" || oldStatus == "PAID") {
		for _, item := range sale.SaleItems {
			if item.Product.IsService {
				continue
			}

			if item.Product.IsAssembly {
				var bom models.BOM
				if err := tx.Preload("Items").Where("target_product_id = ?", item.ProductID).First(&bom).Error; err == nil {
					for _, bomItem := range bom.Items {
						requiredQty := int(bomItem.RequirementQuantity * float64(item.Quantity))
						if requiredQty > 0 {
							if err := s.stockService.RestoreStock(bomItem.ComponentProductID, requiredQty, tx); err != nil {
								log.Printf("Warning: Failed to restore stock for component %d of assembly %d: %v", bomItem.ComponentProductID, item.ProductID, err)
							} else {
								affectedProductIDs[bomItem.ComponentProductID] = struct{}{}
							}
						}
					}
					log.Printf("Kitting Restore: Component stocks restored for cancelled assembly %d (%s)", item.ProductID, item.Product.Name)
				} else {
					log.Printf("Warning: BOM not found for assembly %d during stock restoration", item.ProductID)
				}
				continue
			}

			if err := s.stockService.RestoreStock(item.ProductID, item.Quantity, tx); err != nil {
				log.Printf("Warning: Failed to restore stock for product %d: %v", item.ProductID, err)
			} else {
				affectedProductIDs[item.ProductID] = struct{}{}
			}
		}
	}

	log.Printf("Sale #%d cancelled (status: %s -> CANCELLED) - Full accounting cleanup completed", sale.ID, oldStatus)

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	s.syncStockAlertsForProducts(affectedProductIDs)

	return nil
}

// GetSales retrieves sales with filters
func (s *SalesServiceV2) GetSales(filter models.SalesFilter) (*models.SalesResult, error) {
	query := s.db.Model(&models.Sale{}).Preload("Customer").Preload("SaleItems").Preload("Project")

	// Apply filters
	if filter.Status != "" {
		switch filter.Status {
		case models.SaleStatusDraft:
			query = query.Where("status IN ?", []string{models.SaleStatusDraft, models.SaleStatusPending, models.SaleStatusConfirmed})
		case models.SaleStatusInvoiced:
			query = query.Where("status IN ?", []string{models.SaleStatusInvoiced, models.SaleStatusOverdue})
		default:
			query = query.Where("status = ?", filter.Status)
		}
	}
	if filter.CustomerID != "" {
		query = query.Where("customer_id = ?", filter.CustomerID)
	}
	if filter.StartDate != "" {
		// Ensure start date is inclusive from 00:00:00 local time
		if t, err := time.Parse("2006-01-02", filter.StartDate); err == nil {
			query = query.Where("date >= ?", t)
		} else {
			// Fallback to raw string if parsing fails
			query = query.Where("date >= ?", filter.StartDate)
		}
	}
	if filter.EndDate != "" {
		// IMPORTANT: make end date inclusive for the whole day
		// We compare using '< nextDay' to avoid missing records on the end date due to time components
		if t, err := time.Parse("2006-01-02", filter.EndDate); err == nil {
			nextDay := t.AddDate(0, 0, 1)
			query = query.Where("date < ?", nextDay)
		} else {
			// Fallback to '<= endDate 23:59:59' style by using the raw string
			query = query.Where("date <= ?", filter.EndDate)
		}
	}
	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		// Join contacts to allow searching by customer name as well
		query = query.Joins("LEFT JOIN contacts ON contacts.id = sales.customer_id").
			Where("sales.invoice_number ILIKE ? OR sales.code ILIKE ? OR sales.reference ILIKE ? OR contacts.name ILIKE ?",
				searchPattern, searchPattern, searchPattern, searchPattern)
	}

	// Count total
	var total int64
	query.Count(&total)

	// Apply pagination
	offset := (filter.Page - 1) * filter.Limit
	var sales []models.Sale
	if err := query.Offset(offset).Limit(filter.Limit).Order("created_at DESC").Find(&sales).Error; err != nil {
		return nil, fmt.Errorf("failed to get sales: %v", err)
	}

	return &models.SalesResult{
		Data:       sales,
		Total:      int(total),
		Page:       filter.Page,
		Limit:      filter.Limit,
		TotalPages: int((total + int64(filter.Limit) - 1) / int64(filter.Limit)),
	}, nil
}

// GetSaleByID retrieves a single sale by ID
func (s *SalesServiceV2) GetSaleByID(saleID uint) (*models.Sale, error) {
	var sale models.Sale
	if err := s.db.Preload("Customer").
		Preload("User").
		Preload("SalesPerson").
		Preload("Project").
		Preload("InvoiceType").
		Preload("SaleItems.Product").
		Preload("PaymentSchedules", func(db *gorm.DB) *gorm.DB {
			return db.Order("sequence ASC")
		}).
		Preload("SalePayments").
		First(&sale, saleID).Error; err != nil {
		return nil, fmt.Errorf("sale not found")
	}
	hydrateSalePaymentScheduleProgress(&sale)
	return &sale, nil
}

// DeleteSale cancels a sale instead of deleting it (proper accounting: maintain audit trail)
func (s *SalesServiceV2) DeleteSale(saleID uint) error {
	// Check if sale exists
	var sale models.Sale
	if err := s.db.First(&sale, saleID).Error; err != nil {
		return fmt.Errorf("sale not found")
	}

	// If already cancelled, nothing to do
	if sale.Status == "CANCELLED" {
		log.Printf("ℹ️ Sale #%d is already CANCELLED, nothing to do", saleID)
		return nil
	}

	// Guard: check if sale date is in a closed accounting period
	if err := CheckClosedPeriod(s.db, sale.Date); err != nil {
		return err
	}

	// For INVOICED/PAID sales, use CancelSale which handles full accounting reversal
	// (journal, stock, cash/bank, payments, auto-kitting restore)
	if sale.Status == "INVOICED" || sale.Status == "PAID" {
		log.Printf("🔄 Sale #%d has status %s, performing full accounting reversal via CancelSale", saleID, sale.Status)
		return s.CancelSale(saleID, "Dibatalkan oleh admin", 0)
	}

	// For DRAFT/CONFIRMED sales (no journal entries yet), just set status to CANCELLED
	log.Printf("🔄 Sale #%d has status %s, setting status to CANCELLED (no journal reversal needed)", saleID, sale.Status)

	sale.Status = "CANCELLED"
	sale.InternalNotes = "Dibatalkan oleh admin"
	sale.UpdatedAt = time.Now()
	sale.OutstandingAmount = 0
	sale.PaidAmount = 0

	if err := s.db.Save(&sale).Error; err != nil {
		return fmt.Errorf("failed to cancel sale: %v", err)
	}

	log.Printf("✅ Sale #%d cancelled successfully (status: CANCELLED, audit trail preserved)", saleID)
	return nil
}

// GetSalePayments retrieves all payments for a sale
func (s *SalesServiceV2) GetSalePayments(saleID uint) ([]models.SalePayment, error) {
	var payments []models.SalePayment
	if err := s.db.Where("sale_id = ?", saleID).
		Preload("User").
		Preload("CashBank").
		Find(&payments).Error; err != nil {
		return nil, fmt.Errorf("failed to get sale payments: %v", err)
	}
	return payments, nil
}

// ValidateStockForCreate validates stock for each item in the sale create request and returns warnings without mutating state
func (s *SalesServiceV2) ValidateStockForCreate(req models.StockValidationRequest) (*models.StockValidationResponse, error) {
	result := &models.StockValidationResponse{Items: []models.StockValidationItem{}}
	if len(req.Items) == 0 {
		return result, nil
	}

	// Preload all products referenced to minimize queries
	productIDs := make([]uint, 0, len(req.Items))
	for _, it := range req.Items {
		if it.Quantity <= 0 {
			continue
		}
		productIDs = append(productIDs, it.ProductID)
	}
	if len(productIDs) == 0 {
		return result, nil
	}

	var products []models.Product
	if err := s.db.Where("id IN ?", productIDs).Find(&products).Error; err != nil {
		return nil, fmt.Errorf("failed to load products for stock validation: %v", err)
	}

	// Build lookup
	prodMap := map[uint]models.Product{}
	for _, p := range products {
		prodMap[p.ID] = p
	}

	remainingProductStock := make(map[uint]int)

	for _, it := range req.Items {
		manualName := normalizeSaleItemProductName(&it)
		if it.ProductID == 0 && manualName != "" {
			result.Items = append(result.Items, models.StockValidationItem{
				ProductID:    0,
				ProductCode:  manualSaleProductCode,
				ProductName:  manualName,
				RequestedQty: int(it.Quantity),
				AvailableQty: 0,
				IsService:    true,
				IsSufficient: true,
			})
			continue
		}

		if it.ProductID == 0 {
			itemRes := models.StockValidationItem{
				ProductID:    0,
				ProductName:  manualName,
				RequestedQty: int(it.Quantity),
				IsSufficient: false,
				Warning:      "Nama produk manual wajib diisi",
			}
			result.Items = append(result.Items, itemRes)
			result.HasInsufficient = true
			continue
		}

		p, ok := prodMap[it.ProductID]
		if !ok {
			// Unknown product, mark as insufficient
			itemRes := models.StockValidationItem{
				ProductID:    it.ProductID,
				RequestedQty: int(it.Quantity),
				IsSufficient: false,
				Warning:      "Produk tidak ditemukan",
			}
			result.Items = append(result.Items, itemRes)
			result.HasInsufficient = true
			continue
		}

		// Services and Assemblies do not require direct stock numbers
		if p.IsService || p.IsAssembly {
			// For assemblies, we technically bypass root stock warnings
			// but could emit warnings for components later.
			result.Items = append(result.Items, models.StockValidationItem{
				ProductID:    p.ID,
				ProductCode:  p.Code,
				ProductName:  p.Name,
				RequestedQty: int(it.Quantity),
				AvailableQty: 0,
				MinStock:     p.MinStock,
				ReorderLevel: p.ReorderLevel,
				IsService:    p.IsService,
				IsSufficient: true,
				Warning:      "",
			})
			continue
		}

		available := currentRemainingSaleProductStock(&p, remainingProductStock)
		reqQty := int(it.Quantity)
		isSufficient := available >= reqQty
		lowStock := available <= p.MinStock && p.MinStock > 0
		atOrBelowMin := available <= p.MinStock && p.MinStock > 0
		atOrBelowReorder := available <= p.ReorderLevel && p.ReorderLevel > 0
		isZeroStock := available == 0

		warning := ""
		if isZeroStock {
			// Explicit hard alert when stock is 0
			result.HasZeroStock = true
			if warning == "" {
				warning = "Stok 0: qty akan dicatat penuh sebagai backorder"
			} else {
				warning = "Stok 0: qty akan dicatat penuh sebagai backorder; " + warning
			}
		}
		if !isSufficient {
			if warning == "" {
				warning = fmt.Sprintf("Stock tidak cukup. Tersedia %d, diminta %d. Selisih akan dicatat sebagai backorder", available, reqQty)
			} else {
				warning += fmt.Sprintf("; stok tidak cukup (tersedia %d, diminta %d) dan selisih akan dicatat sebagai backorder", available, reqQty)
			}
			result.HasInsufficient = true
		}
		if atOrBelowMin {
			result.HasMinStockAlerts = true
			if warning == "" {
				warning = fmt.Sprintf("Di bawah stok minimum (%d)", p.MinStock)
			} else {
				warning += fmt.Sprintf("; di bawah stok minimum (%d)", p.MinStock)
			}
		}
		if atOrBelowReorder {
			result.HasReorderAlerts = true
			if warning == "" {
				warning = fmt.Sprintf("Di bawah level reorder (%d)", p.ReorderLevel)
			} else {
				warning += fmt.Sprintf("; di bawah level reorder (%d)", p.ReorderLevel)
			}
		}
		if available <= p.MinStock && p.MinStock > 0 {
			result.HasLowStock = true
		}

		result.Items = append(result.Items, models.StockValidationItem{
			ProductID:        p.ID,
			ProductCode:      p.Code,
			ProductName:      p.Name,
			RequestedQty:     reqQty,
			AvailableQty:     available,
			MinStock:         p.MinStock,
			ReorderLevel:     p.ReorderLevel,
			IsService:        false,
			IsSufficient:     isSufficient,
			LowStock:         lowStock,
			AtOrBelowMin:     atOrBelowMin,
			AtOrBelowReorder: atOrBelowReorder,
			IsZeroStock:      isZeroStock,
			Warning:          warning,
		})

		allocateSaleProductStock(&p, reqQty, remainingProductStock)
	}

	return result, nil
}

// Helper functions
// Deprecated: generateSaleCode is no longer used. Sales codes are now generated
// through the settings service using GetNextSalesNumber() for consistent prefixing.
func (s *SalesServiceV2) generateSaleCode() string {
	var count int64
	s.db.Model(&models.Sale{}).Count(&count)
	return fmt.Sprintf("SO-%s-%05d", time.Now().Format("200601"), count+1)
}

func (s *SalesServiceV2) generateInvoiceNumber() string {
	var count int64
	s.db.Model(&models.Sale{}).Where("invoice_number IS NOT NULL").Count(&count)
	return fmt.Sprintf("INV-%s-%05d", time.Now().Format("200601"), count+1)
}

func (s *SalesServiceV2) generatePaymentNumber() string {
	var count int64
	s.db.Model(&models.SalePayment{}).Count(&count)
	return fmt.Sprintf("PAY-%s-%05d", time.Now().Format("200601"), count+1)
}

func getOrDefault[T any](ptr *T, defaultValue T) T {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

func getOrDefaultStr(value string, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func saleScheduleIsZero(value float64) bool {
	return math.Abs(value) < 0.0001
}

func (s *SalesServiceV2) validateSalePaymentSchedules(schedules []models.SalePaymentScheduleRequest) error {
	if len(schedules) == 0 {
		return fmt.Errorf("jadwal termin wajib diisi untuk payment terms custom")
	}

	totalPercentage := 0.0
	for index, schedule := range schedules {
		if schedule.Percentage <= 0 {
			return fmt.Errorf("persentase termin #%d harus lebih besar dari 0", index+1)
		}
		if schedule.DueDate.IsZero() {
			return fmt.Errorf("jatuh tempo termin #%d wajib diisi", index+1)
		}
		totalPercentage += schedule.Percentage
	}

	if math.Abs(totalPercentage-100) > 0.01 {
		return fmt.Errorf("total persentase termin harus 100%%, saat ini %.2f%%", totalPercentage)
	}

	return nil
}

func validateSalePaymentScheduleMode(paymentTerms, paymentMethodType string) error {
	if strings.ToUpper(strings.TrimSpace(paymentTerms)) == "CUSTOM" &&
		strings.ToUpper(strings.TrimSpace(paymentMethodType)) != "CREDIT" {
		return fmt.Errorf("jadwal termin hanya dapat digunakan untuk penjualan kredit/piutang")
	}

	return nil
}

func buildSalePaymentSchedules(saleID uint, totalAmount float64, schedules []models.SalePaymentScheduleRequest) []models.SalePaymentSchedule {
	result := make([]models.SalePaymentSchedule, 0, len(schedules))
	allocated := 0.0

	for index, schedule := range schedules {
		amount := totalAmount * schedule.Percentage / 100
		if index == len(schedules)-1 {
			amount = totalAmount - allocated
		} else {
			allocated += amount
		}

		sequence := schedule.Sequence
		if sequence <= 0 {
			sequence = index + 1
		}

		result = append(result, models.SalePaymentSchedule{
			SaleID:      saleID,
			Sequence:    sequence,
			Percentage:  schedule.Percentage,
			Amount:      amount,
			DueDate:     schedule.DueDate,
			Description: strings.TrimSpace(schedule.Description),
			Status:      "PENDING",
		})
	}

	return result
}

func (s *SalesServiceV2) syncSalePaymentSchedules(tx *gorm.DB, sale *models.Sale, scheduleRequests []models.SalePaymentScheduleRequest) error {
	if sale == nil {
		return fmt.Errorf("sale is required")
	}

	if err := tx.Where("sale_id = ?", sale.ID).Delete(&models.SalePaymentSchedule{}).Error; err != nil {
		return fmt.Errorf("failed to clear previous payment schedules: %v", err)
	}

	sale.PaymentSchedules = []models.SalePaymentSchedule{}

	if strings.ToUpper(strings.TrimSpace(sale.PaymentTerms)) != "CUSTOM" {
		return nil
	}

	if err := s.validateSalePaymentSchedules(scheduleRequests); err != nil {
		return err
	}

	schedules := buildSalePaymentSchedules(sale.ID, sale.TotalAmount, scheduleRequests)
	if len(schedules) == 0 {
		return nil
	}

	if err := tx.Create(&schedules).Error; err != nil {
		return fmt.Errorf("failed to create payment schedules: %v", err)
	}

	sale.PaymentSchedules = schedules
	hydrateSalePaymentScheduleProgress(sale)
	return nil
}

func deriveSaleDueDate(paymentTerms string, dueDate time.Time, schedules []models.SalePaymentScheduleRequest) time.Time {
	if strings.ToUpper(strings.TrimSpace(paymentTerms)) != "CUSTOM" || len(schedules) == 0 {
		return dueDate
	}

	latest := dueDate
	for _, schedule := range schedules {
		if schedule.DueDate.IsZero() {
			continue
		}
		if latest.IsZero() || schedule.DueDate.After(latest) {
			latest = schedule.DueDate
		}
	}

	return latest
}

func (s *SalesServiceV2) applySalesSummaryFilter(query *gorm.DB, filter models.SalesFilter) *gorm.DB {
	if filter.Status != "" {
		switch filter.Status {
		case models.SaleStatusDraft:
			query = query.Where("sales.status IN ?", []string{models.SaleStatusDraft, models.SaleStatusPending, models.SaleStatusConfirmed})
		case models.SaleStatusInvoiced:
			query = query.Where("sales.status IN ?", []string{models.SaleStatusInvoiced, models.SaleStatusOverdue})
		default:
			query = query.Where("sales.status = ?", filter.Status)
		}
	} else {
		query = query.Where("sales.status IN ?", []string{models.SaleStatusInvoiced, models.SaleStatusOverdue, models.SaleStatusPaid})
	}

	if filter.CustomerID != "" {
		query = query.Where("sales.customer_id = ?", filter.CustomerID)
	}

	if filter.StartDate != "" {
		if t, err := time.Parse("2006-01-02", filter.StartDate); err == nil {
			query = query.Where("sales.date >= ?", t)
		} else {
			query = query.Where("sales.date >= ?", filter.StartDate)
		}
	}

	if filter.EndDate != "" {
		if t, err := time.Parse("2006-01-02", filter.EndDate); err == nil {
			nextDay := t.AddDate(0, 0, 1)
			query = query.Where("sales.date < ?", nextDay)
		} else {
			query = query.Where("sales.date <= ?", filter.EndDate)
		}
	}

	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		query = query.Joins("LEFT JOIN contacts ON contacts.id = sales.customer_id").
			Where("sales.invoice_number ILIKE ? OR sales.code ILIKE ? OR sales.reference ILIKE ? OR contacts.name ILIKE ?",
				searchPattern, searchPattern, searchPattern, searchPattern)
	}

	return query
}

// GetSalesSummary gets sales summary with filter awareness
func (s *SalesServiceV2) GetSalesSummary(filter models.SalesFilter) (*models.SalesSummaryResponse, error) {
	log.Printf("📊 Getting sales summary with filters: status=%s, customer=%s, start=%s, end=%s, search=%s",
		filter.Status, filter.CustomerID, filter.StartDate, filter.EndDate, filter.Search)

	query := s.applySalesSummaryFilter(s.db.Model(&models.Sale{}), filter)

	// Get basic counts and totals
	var totalSales int64
	var totalAmount, totalPaid, totalOutstanding float64

	query.Count(&totalSales)

	// Get sum of amounts
	type SumResult struct {
		TotalAmount      float64 `json:"total_amount"`
		TotalPaid        float64 `json:"total_paid"`
		TotalOutstanding float64 `json:"total_outstanding"`
	}

	var sumResult SumResult
	err := query.Select(
		"COALESCE(SUM(total_amount), 0) as total_amount, " +
			"COALESCE(SUM(paid_amount), 0) as total_paid, " +
			"COALESCE(SUM(outstanding_amount), 0) as total_outstanding",
	).Scan(&sumResult).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get sales summary: %v", err)
	}

	totalAmount = sumResult.TotalAmount
	totalPaid = sumResult.TotalPaid
	totalOutstanding = sumResult.TotalOutstanding

	// Calculate average order value
	avgOrderValue := 0.0
	if totalSales > 0 {
		avgOrderValue = totalAmount / float64(totalSales)
	}

	// Get top customers
	topCustomers, err := s.getTopCustomers(filter)
	if err != nil {
		log.Printf("⚠️ Warning: Failed to get top customers: %v", err)
		topCustomers = []models.CustomerSales{} // Empty slice instead of nil
	}

	summary := &models.SalesSummaryResponse{
		TotalSales:       totalSales,
		TotalAmount:      totalAmount,
		TotalPaid:        totalPaid,
		TotalOutstanding: totalOutstanding,
		AvgOrderValue:    avgOrderValue,
		TopCustomers:     topCustomers,
	}

	log.Printf("✅ Sales summary generated: %d sales, total: %.2f", totalSales, totalAmount)
	return summary, nil
}

// getTopCustomers gets top 5 customers by total sales amount
func (s *SalesServiceV2) getTopCustomers(filter models.SalesFilter) ([]models.CustomerSales, error) {
	query := s.db.Table("sales").
		Select("sales.customer_id, contacts.name as customer_name, COUNT(sales.id) as total_orders, SUM(sales.total_amount) as total_amount").
		Joins("LEFT JOIN contacts ON contacts.id = sales.customer_id").
		Where("sales.deleted_at IS NULL").
		Group("sales.customer_id, contacts.name").
		Order("total_amount DESC").
		Limit(5)

	query = s.applySalesSummaryFilter(query, filter)

	var topCustomers []models.CustomerSales
	if err := query.Find(&topCustomers).Error; err != nil {
		return nil, fmt.Errorf("failed to get top customers: %v", err)
	}

	return topCustomers, nil
}

// recalculateProjectSalesRevenue recalculates the total sales revenue for a project
// by aggregating TotalAmount from all sales linked to this project
func (s *SalesServiceV2) recalculateProjectSalesRevenue(projectID uint) {
	var totalRevenue float64
	err := s.db.Model(&models.Sale{}).
		Where("project_id = ? AND deleted_at IS NULL AND status IN ('INVOICED', 'OVERDUE', 'PAID')", projectID).
		Select("COALESCE(SUM(total_amount), 0)").
		Scan(&totalRevenue).Error

	if err != nil {
		log.Printf("⚠️ Failed to recalculate project sales revenue for project %d: %v", projectID, err)
		return
	}

	if err := s.db.Model(&models.Project{}).Where("id = ?", projectID).
		Update("total_sales_revenue", totalRevenue).Error; err != nil {
		log.Printf("⚠️ Failed to update project sales revenue for project %d: %v", projectID, err)
		return
	}

	log.Printf("✅ Updated project %d sales revenue: %.2f", projectID, totalRevenue)
}
