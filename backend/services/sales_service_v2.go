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

// SalesServiceV2 handles all sales operations with clean business logic
type SalesServiceV2 struct {
	db                      *gorm.DB
	salesRepo               *repositories.SalesRepository
	salesJournalService     *SalesJournalServiceV2   // Legacy simple_ssot_journals (deprecated)
	salesJournalServiceSSOT *SalesJournalServiceSSOT // NEW: unified_journal_ledger (for Balance Sheet)
	cogsService             *InventoryCOGSService    // NEW: COGS recording service
	stockService            *StockService
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
		notificationService:     notificationService,
		settingsService:         settingsService,
		invoiceNumberService:    invoiceNumberService,
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
	for _, item := range request.Items {
		stockValidationReq.Items = append(stockValidationReq.Items, models.SaleItemRequest{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		})
	}

	// Validate stock availability
	stockValidation, err := s.ValidateStockForCreate(stockValidationReq)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to validate stock: %v", err)
	}

	// Check if there are any stock issues
	if stockValidation.HasInsufficient {
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

	if stockValidation.HasZeroStock {
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
	if stockValidation.HasMinStockAlerts || stockValidation.HasReorderAlerts {
		for _, item := range stockValidation.Items {
			if item.Warning != "" && item.IsSufficient {
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

	// Handle enhanced tax configuration
	ppnPercent := request.PPNPercent
	if ppnPercent == nil {
		// Use PPNRate if PPNPercent is not provided
		if request.PPNRate != nil {
			ppnPercent = request.PPNRate
		} else {
			// Only default to 11 if absolutely no PPN info is provided
			defaultPPN := 11.0
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
		PPNRate:            *ppnPercent,
		OtherTaxAdditions:  request.OtherTaxAdditions,
		PPh21Rate:          request.PPh21Rate,
		PPh23Rate:          request.PPh23Rate,
		OtherTaxDeductions: request.OtherTaxDeductions,
		PaymentTerms:       request.PaymentTerms,
		PaymentMethod:      request.PaymentMethod,
		PaymentMethodType:  request.PaymentMethodType, // CASH, BANK, or CREDIT
		CashBankID:         request.CashBankID,
		ShippingMethod:     request.ShippingMethod,
		ShippingCost:       request.ShippingCost,
		ShippingTaxable:    request.ShippingTaxable,
		BillingAddress:     request.BillingAddress,
		ShippingAddress:    request.ShippingAddress,
		Notes:              request.Notes,
		InternalNotes:      request.InternalNotes,
		Reference:          request.Reference,
	}

	// Calculate totals
	var subtotal float64 = 0
	var totalPPN float64 = 0
	var totalPPH float64 = 0

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

		// Set default revenue account if not provided or 0
		revenueAccountID := itemRequest.RevenueAccountID
		if revenueAccountID == 0 {
			// ✅ FIX: Use LEAF account 4101-001 (Penjualan Lokal) instead of HEADER 4101
			// Posting to header account causes balance to be overwritten to 0 by
			// RecalculateAllParentBalances() which recalculates headers as SUM(children)
			var defaultAccount models.Account
			if err := tx.Where("code = ? AND deleted_at IS NULL", "4101-001").First(&defaultAccount).Error; err != nil {
				// Fallback: try header 4101 if leaf doesn't exist, then any revenue account
				if err := tx.Where("code = ? AND deleted_at IS NULL", "4101").First(&defaultAccount).Error; err != nil {
					if err := tx.Where("type = ? AND deleted_at IS NULL", "REVENUE").First(&defaultAccount).Error; err != nil {
						tx.Rollback()
						return nil, fmt.Errorf("failed to find default revenue account: %v", err)
					}
				}
			}
			revenueAccountID = defaultAccount.ID
		}

		// ✅ FIX: Guard against header accounts — ALWAYS redirect to leaf child
		// This catches cases where the frontend sends a header account ID (e.g., from tax_account_settings)
		if revenueAccountID != 0 {
			var revAcc models.Account
			if err := tx.First(&revAcc, revenueAccountID).Error; err == nil && revAcc.IsHeader {
				log.Printf("⚠️ Revenue account %s (ID=%d) is a HEADER, redirecting to leaf child", revAcc.Code, revAcc.ID)
				var leafAcc models.Account
				if err := tx.Where("parent_id = ? AND is_header = false AND deleted_at IS NULL", revAcc.ID).First(&leafAcc).Error; err == nil {
					revenueAccountID = leafAcc.ID
					log.Printf("✅ Redirected to leaf revenue account %s (ID=%d)", leafAcc.Code, leafAcc.ID)
				} else {
					// Use 4101-001 as absolute fallback
					if err := tx.Where("code = ? AND deleted_at IS NULL", "4101-001").First(&leafAcc).Error; err == nil {
						revenueAccountID = leafAcc.ID
						log.Printf("✅ Fallback to 4101-001 (ID=%d)", leafAcc.ID)
					}
				}
			}
		}

		item := models.SaleItem{
			ProductID:        itemRequest.ProductID,
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
			item.PPNAmount = item.LineTotal * (sale.PPNPercent / 100)
			item.PPhAmount = item.LineTotal * (sale.PPhPercent / 100)
			totalPPN += item.PPNAmount
			totalPPH += item.PPhAmount
		}

		item.TotalTax = item.PPNAmount + item.PPhAmount
		item.FinalAmount = item.LineTotal + item.PPNAmount - item.PPhAmount

		subtotal += item.LineTotal
		sale.SaleItems = append(sale.SaleItems, item)
	}

	// Calculate sale totals
	sale.Subtotal = subtotal
	sale.SubTotal = subtotal // Compatibility field

	// ✅ FIX: Apply sale-level discount
	sale.DiscountAmount = subtotal * (sale.DiscountPercent / 100)
	sale.TaxableAmount = subtotal - sale.DiscountAmount
	sale.NetBeforeTax = sale.TaxableAmount // Net amount before any taxes

	// ✅ FIX: Recalculate PPN from TaxableAmount (after sale-level discount)
	// Item-level PPN was calculated before sale discount, so recalculate
	if sale.PPNPercent > 0 || sale.PPNRate > 0 {
		ppnRate := sale.PPNPercent
		if ppnRate == 0 {
			ppnRate = sale.PPNRate
		}
		sale.PPN = sale.TaxableAmount * (ppnRate / 100)
		sale.PPNAmount = sale.PPN // Enhanced field
	} else {
		sale.PPN = totalPPN // Use item-level PPN if no rate specified
		sale.PPNAmount = totalPPN
	}

	// ✅ FIX: Calculate enhanced tax fields
	// PPh21 and PPh23 are calculated from TaxableAmount if rates are provided
	if sale.PPh21Rate > 0 {
		sale.PPh21Amount = sale.TaxableAmount * (sale.PPh21Rate / 100)
	}
	if sale.PPh23Rate > 0 {
		sale.PPh23Amount = sale.TaxableAmount * (sale.PPh23Rate / 100)
	}

	// Legacy PPh field
	sale.PPh = totalPPH

	// Total tax additions (added to invoice total)
	sale.TotalTaxAdditions = sale.OtherTaxAdditions

	// Total tax deductions (subtracted from invoice total)
	sale.TotalTaxDeductions = sale.PPh + sale.PPh21Amount + sale.PPh23Amount + sale.OtherTaxDeductions

	// Total tax for display
	sale.TotalTax = sale.PPN + sale.TotalTaxAdditions - sale.TotalTaxDeductions
	sale.Tax = sale.TotalTax // Compatibility field

	// ✅ FIX: Calculate final TotalAmount correctly
	// Formula: TaxableAmount + PPN + OtherTaxAdditions + ShippingCost - TotalTaxDeductions
	sale.TotalAmount = sale.TaxableAmount + sale.PPN + sale.OtherTaxAdditions + sale.ShippingCost - sale.TotalTaxDeductions
	sale.OutstandingAmount = sale.TotalAmount // Initially all outstanding

	log.Printf("📊 [CALC] Subtotal=%.2f, Discount=%.2f, TaxableAmt=%.2f", sale.Subtotal, sale.DiscountAmount, sale.TaxableAmount)
	log.Printf("📊 [CALC] PPN=%.2f, Additions=%.2f, Deductions=%.2f, Shipping=%.2f",
		sale.PPN, sale.OtherTaxAdditions, sale.TotalTaxDeductions, sale.ShippingCost)
	log.Printf("📊 [CALC] TotalAmount=%.2f", sale.TotalAmount)

	// Save sale
	if err := tx.Create(&sale).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create sale: %v", err)
	}

	// Load relationships
	if err := tx.Preload("Customer").Preload("SaleItems.Product").First(&sale, sale.ID).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to load sale relationships: %v", err)
	}

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
		if request.ShippingCost != nil {
			tx.Rollback()
			return nil, fmt.Errorf("tidak bisa mengubah biaya pengiriman pada penjualan yang sudah difakturkan")
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

		// Save and return (skip item/tax recalculation)
		if err := tx.Save(&sale).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to update sale: %v", err)
		}

		if err := tx.Commit().Error; err != nil {
			return nil, fmt.Errorf("failed to commit transaction: %v", err)
		}

		// Reload with relationships
		s.db.Preload("Customer").Preload("SaleItems.Product").Preload("InvoiceType").First(&sale, sale.ID)

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

	// Recalculate if items are updated
	if request.Items != nil {
		// Delete old items
		if err := tx.Where("sale_id = ?", sale.ID).Delete(&models.SaleItem{}).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to delete old items: %v", err)
		}

		// Add new items and recalculate
		var subtotal float64 = 0
		var totalPPN float64 = 0
		var totalPPH float64 = 0
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

			// Set default revenue account if not provided or 0
			revenueAccountID := itemRequest.RevenueAccountID
			if revenueAccountID == 0 {
				// ✅ FIX: Use LEAF account 4101-001 instead of HEADER 4101
				var defaultAccount models.Account
				if err := tx.Where("code = ? AND deleted_at IS NULL", "4101-001").First(&defaultAccount).Error; err != nil {
					// Fallback: try header 4101 if leaf doesn't exist, then any revenue account
					if err := tx.Where("code = ? AND deleted_at IS NULL", "4101").First(&defaultAccount).Error; err != nil {
						if err := tx.Where("type = ? AND deleted_at IS NULL", "REVENUE").First(&defaultAccount).Error; err != nil {
							tx.Rollback()
							return nil, fmt.Errorf("failed to find default revenue account: %v", err)
						}
					}
				}
				revenueAccountID = defaultAccount.ID
			}

			// ✅ FIX: Guard against header accounts — ALWAYS redirect to leaf child
			if revenueAccountID != 0 {
				var revAcc models.Account
				if err := tx.First(&revAcc, revenueAccountID).Error; err == nil && revAcc.IsHeader {
					log.Printf("⚠️ [UpdateSale] Revenue account %s (ID=%d) is a HEADER, redirecting to leaf", revAcc.Code, revAcc.ID)
					var leafAcc models.Account
					if err := tx.Where("parent_id = ? AND is_header = false AND deleted_at IS NULL", revAcc.ID).First(&leafAcc).Error; err == nil {
						revenueAccountID = leafAcc.ID
						log.Printf("✅ Redirected to leaf revenue account %s (ID=%d)", leafAcc.Code, leafAcc.ID)
					} else {
						if err := tx.Where("code = ? AND deleted_at IS NULL", "4101-001").First(&leafAcc).Error; err == nil {
							revenueAccountID = leafAcc.ID
						}
					}
				}
			}

			item := models.SaleItem{
				SaleID:           sale.ID,
				ProductID:        itemRequest.ProductID,
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
				item.PPNAmount = item.LineTotal * (ppnRate / 100)
				item.PPhAmount = item.LineTotal * (sale.PPhPercent / 100)
				totalPPN += item.PPNAmount
				totalPPH += item.PPhAmount
			}

			item.TotalTax = item.PPNAmount + item.PPhAmount
			item.FinalAmount = item.LineTotal + item.PPNAmount - item.PPhAmount

			subtotal += item.LineTotal
			sale.SaleItems = append(sale.SaleItems, item)
		}

		// Apply same calculation logic as CreateSale
		sale.Subtotal = subtotal
		sale.SubTotal = subtotal

		// Apply sale-level discount
		sale.DiscountAmount = subtotal * (sale.DiscountPercent / 100)
		sale.TaxableAmount = subtotal - sale.DiscountAmount
		sale.NetBeforeTax = sale.TaxableAmount

		// Recalculate PPN from TaxableAmount
		if sale.PPNPercent > 0 || sale.PPNRate > 0 {
			ppnRate := sale.PPNPercent
			if ppnRate == 0 {
				ppnRate = sale.PPNRate
			}
			sale.PPN = sale.TaxableAmount * (ppnRate / 100)
			sale.PPNAmount = sale.PPN
		} else {
			sale.PPN = totalPPN
			sale.PPNAmount = totalPPN
		}

		// Calculate enhanced tax fields
		if sale.PPh21Rate > 0 {
			sale.PPh21Amount = sale.TaxableAmount * (sale.PPh21Rate / 100)
		} else {
			sale.PPh21Amount = 0
		}
		if sale.PPh23Rate > 0 {
			sale.PPh23Amount = sale.TaxableAmount * (sale.PPh23Rate / 100)
		} else {
			sale.PPh23Amount = 0
		}

		sale.PPh = totalPPH
		sale.TotalTaxAdditions = sale.OtherTaxAdditions
		sale.TotalTaxDeductions = sale.PPh + sale.PPh21Amount + sale.PPh23Amount + sale.OtherTaxDeductions
		sale.TotalTax = sale.PPN + sale.TotalTaxAdditions - sale.TotalTaxDeductions
		sale.Tax = sale.TotalTax

		// Calculate final TotalAmount
		sale.TotalAmount = sale.TaxableAmount + sale.PPN + sale.OtherTaxAdditions + sale.ShippingCost - sale.TotalTaxDeductions
		sale.OutstandingAmount = sale.TotalAmount - sale.PaidAmount

		log.Printf("📊 [UPDATE] TotalAmount=%.2f, Outstanding=%.2f", sale.TotalAmount, sale.OutstandingAmount)
	} else {
		// ✅ FIX: Recalculate totals even if items didn't change (tax fields may have changed)
		// This ensures PPh23, PPh21, and other tax fields are properly applied

		// Recalculate from existing subtotal
		sale.DiscountAmount = sale.Subtotal * (sale.DiscountPercent / 100)
		sale.TaxableAmount = sale.Subtotal - sale.DiscountAmount
		sale.NetBeforeTax = sale.TaxableAmount

		// Recalculate PPN
		if sale.PPNPercent > 0 || sale.PPNRate > 0 {
			ppnRate := sale.PPNPercent
			if ppnRate == 0 {
				ppnRate = sale.PPNRate
			}
			sale.PPN = sale.TaxableAmount * (ppnRate / 100)
			sale.PPNAmount = sale.PPN
		}

		// Recalculate enhanced tax fields
		if sale.PPh21Rate > 0 {
			sale.PPh21Amount = sale.TaxableAmount * (sale.PPh21Rate / 100)
		} else {
			sale.PPh21Amount = 0
		}
		if sale.PPh23Rate > 0 {
			sale.PPh23Amount = sale.TaxableAmount * (sale.PPh23Rate / 100)
		} else {
			sale.PPh23Amount = 0
		}

		// Recalculate totals
		sale.TotalTaxAdditions = sale.OtherTaxAdditions
		sale.TotalTaxDeductions = sale.PPh + sale.PPh21Amount + sale.PPh23Amount + sale.OtherTaxDeductions
		sale.TotalTax = sale.PPN + sale.TotalTaxAdditions - sale.TotalTaxDeductions
		sale.Tax = sale.TotalTax

		// Calculate final TotalAmount
		sale.TotalAmount = sale.TaxableAmount + sale.PPN + sale.OtherTaxAdditions + sale.ShippingCost - sale.TotalTaxDeductions
		sale.OutstandingAmount = sale.TotalAmount - sale.PaidAmount

		log.Printf("📊 [UPDATE-TAX-ONLY] TaxableAmt=%.2f, PPh23=%.2f, Deductions=%.2f, TotalAmount=%.2f",
			sale.TaxableAmount, sale.PPh23Amount, sale.TotalTaxDeductions, sale.TotalAmount)
	}

	// Save updated sale
	if err := tx.Save(&sale).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update sale: %v", err)
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
	s.db.Preload("Customer").Preload("SaleItems.Product").First(&sale, sale.ID)

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
	if sale.InvoiceNumber != "" || sale.Status == "INVOICED" || sale.Status == "PAID" {
		tx.Rollback()
		return nil, fmt.Errorf("invoice already created for this sale (Invoice #%s, Status: %s)",
			sale.InvoiceNumber, sale.Status)
	}

	if sale.Status != "CONFIRMED" && sale.Status != "DRAFT" {
		tx.Rollback()
		return nil, fmt.Errorf("only DRAFT or CONFIRMED sales can be invoiced (current status: %s)", sale.Status)
	}

	// ✅ VALIDATE STOCK BEFORE INVOICE (Early Check)
	// Prevent transaction rollback in the middle of journal creation
	log.Printf("🔍 Validating stock availability before invoice for Sale #%d", sale.ID)
	for _, item := range sale.SaleItems {
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

	// ✅ FIX: Recalculate PPN if it was lost (e.g., sale created before PPN fix)
	// This ensures PPN Keluaran is always posted for taxable credit sales
	if sale.PPN == 0 && sale.PPNPercent > 0 {
		ppnRate := sale.PPNPercent
		if ppnRate == 0 {
			ppnRate = sale.PPNRate
		}

		// Calculate TaxableAmount if not set
		taxableAmount := sale.TaxableAmount
		if taxableAmount == 0 {
			taxableAmount = sale.Subtotal - sale.DiscountAmount
		}

		if taxableAmount > 0 && ppnRate > 0 {
			sale.PPN = taxableAmount * (ppnRate / 100)
			sale.PPNAmount = sale.PPN
			sale.PPNRate = ppnRate
			sale.TotalTax = sale.PPN + sale.TotalTaxAdditions - sale.TotalTaxDeductions
			sale.TotalAmount = taxableAmount + sale.PPN + sale.OtherTaxAdditions + sale.ShippingCost - sale.TotalTaxDeductions
			sale.OutstandingAmount = sale.TotalAmount - sale.PaidAmount

			log.Printf("⚠️ [CreateInvoice] PPN recalculated: PPN=%.2f (PPNPercent=%.2f, TaxableAmount=%.2f)", sale.PPN, ppnRate, taxableAmount)

			// Save updated PPN to DB
			if err := tx.Model(&sale).Updates(map[string]interface{}{
				"ppn":                sale.PPN,
				"ppn_amount":         sale.PPNAmount,
				"ppn_rate":           sale.PPNRate,
				"total_tax":          sale.TotalTax,
				"total_amount":       sale.TotalAmount,
				"outstanding_amount": sale.OutstandingAmount,
			}).Error; err != nil {
				log.Printf("⚠️ Warning: Failed to save recalculated PPN: %v", err)
			} else {
				log.Printf("✅ [CreateInvoice] PPN saved: PPN=%.2f, TotalAmount=%.2f", sale.PPN, sale.TotalAmount)
			}
		}
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
			// Read actual cash/bank balance for accurate balance_after
			var cashBankForBalance models.CashBank
			var balanceAfter float64
			if err := tx.First(&cashBankForBalance, *sale.CashBankID).Error; err != nil {
				log.Printf("⚠️ Warning: Failed to load cash/bank for balance_after: %v", err)
				balanceAfter = 0
			} else {
				balanceAfter = cashBankForBalance.Balance
			}

			cashBankTransaction := &models.CashBankTransaction{
				CashBankID:      *sale.CashBankID,
				ReferenceType:   "SALE",
				ReferenceID:     sale.ID,
				Amount:          sale.TotalAmount,
				BalanceAfter:    balanceAfter,
				TransactionDate: sale.Date,
				Notes:           fmt.Sprintf("Immediate payment - Invoice #%s", sale.InvoiceNumber),
			}

			if err := tx.Create(cashBankTransaction).Error; err != nil {
				log.Printf("⚠️ Warning: Failed to create cash/bank transaction record: %v", err)
				// Don't fail transaction, this is just for audit trail
			}
		}

		log.Printf("💡 Auto-PAID applied: Sale #%d marked as PAID for payment method %s", sale.ID, sale.PaymentMethodType)
		log.Printf("✅ Sale #%d invoiced (status: %s → PAID) - Journal posted, cash/bank auto-synced", sale.ID, oldStatus)
	} else {
		log.Printf("✅ Sale #%d invoiced (status: %s → INVOICED) - Journal posted", sale.ID, oldStatus)
	}

	// Update stock - CRITICAL: Block transaction if stock insufficient
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

					log.Printf("✅ Kitting: Stock reduced for component %d (%s): %d → %d",
						bomItem.ComponentProductID, bomItem.ComponentProduct.Name, bomItem.ComponentProduct.Stock, bomItem.ComponentProduct.Stock-requiredQty)
				}
				log.Printf("✅ Auto-kitting completed for assembly %d (%s)", product.ID, product.Name)
				continue
			}

			// Check stock availability BEFORE reducing
			if product.Stock < item.Quantity {
				tx.Rollback()
				return nil, fmt.Errorf("stock tidak cukup untuk product '%s'. Tersedia: %d, Diminta: %d",
					product.Name, product.Stock, item.Quantity)
			}

			// Check if stock is zero
			if product.Stock == 0 {
				tx.Rollback()
				return nil, fmt.Errorf("product '%s' habis (stock = 0), tidak bisa dijual", product.Name)
			}

			// Reduce stock
			if err := s.stockService.ReduceStock(item.ProductID, item.Quantity, tx); err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("gagal mengurangi stock untuk product '%s': %v", product.Name, err)
			}

			log.Printf("✅ Stock reduced for product %d (%s): %d → %d",
				product.ID, product.Name, product.Stock, product.Stock-item.Quantity)
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

	// Create payment record
	payment := &models.SalePayment{
		SaleID:        saleID,
		PaymentDate:   paymentRequest.PaymentDate,
		Amount:        actualAmount,
		PaymentMethod: paymentRequest.PaymentMethod,
		Reference:     paymentRequest.Reference,
		CashBankID:    paymentRequest.CashBankID,
		Notes:         paymentRequest.Notes,
		UserID:        userID,
	}

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

	if err := tx.Save(&sale).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update sale: %v", err)
	}

	// Create payment journal entries
	if err := s.salesJournalService.CreateSalesPaymentJournal(payment, &sale, tx); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create payment journal: %v", err)
	}

	// ✅ CRITICAL FIX: Update cash_banks.balance for Cash & Bank Management view
	// This is SEPARATE from accounts.balance (COA tree view) which is updated by journal entries
	if payment.CashBankID != nil && *payment.CashBankID > 0 {
		var cashBank models.CashBank
		if err := tx.First(&cashBank, *payment.CashBankID).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("cash/bank account not found: %v", err)
		}

		// Increase cash/bank balance for payment IN (customer paying us)
		cashBank.Balance += payment.Amount
		if err := tx.Save(&cashBank).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to update cash/bank balance: %v", err)
		}

		// Create cash/bank transaction record for audit trail
		cashBankTransaction := &models.CashBankTransaction{
			CashBankID:      *payment.CashBankID,
			ReferenceType:   "SALES_PAYMENT",
			ReferenceID:     payment.ID,
			Amount:          payment.Amount, // Positive for incoming payment
			BalanceAfter:    cashBank.Balance,
			TransactionDate: payment.PaymentDate,
			Notes:           fmt.Sprintf("Customer payment - Invoice #%s", sale.InvoiceNumber),
		}

		if err := tx.Create(cashBankTransaction).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create cash/bank transaction: %v", err)
		}

		log.Printf("💰 Updated Cash/Bank #%d balance: +%.2f (new balance: %.2f)",
			*payment.CashBankID, payment.Amount, cashBank.Balance)
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

	// Guard: check if sale date is in a closed accounting period
	if err := CheckClosedPeriod(s.db, sale.Date); err != nil {
		tx.Rollback()
		return err
	}

	oldStatus := sale.Status
	sale.Status = "CANCELLED"
	sale.InternalNotes = fmt.Sprintf("Cancelled: %s", reason)
	sale.UpdatedAt = time.Now()

	// 1. Reset payment tracking amounts
	log.Printf("💰 Resetting payment tracking for cancelled Sale #%d (Outstanding: %.2f→0, Paid: %.2f→0)",
		sale.ID, sale.OutstandingAmount, sale.PaidAmount)
	sale.OutstandingAmount = 0
	sale.PaidAmount = 0

	if err := tx.Save(&sale).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to cancel sale: %v", err)
	}

	// 2. Remove SALES journal entries if they exist (for INVOICED/PAID status)
	if s.salesJournalService.ShouldPostToJournal(oldStatus) {
		// Delete V2 journal entries (source_type='SALES') — includes COA balance reversal
		if err := s.salesJournalService.DeleteSalesJournal(sale.ID, tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to remove V2 journal entries: %v", err)
		}
		log.Printf("🗑️ Removed V2 SALES journal entries for cancelled Sale #%d", sale.ID)

		// CRITICAL FIX: Also delete SSOT journal entries (source_type='SALE')
		// The SSOT service uses 'SALE' (not 'SALES') as source_type
		// Without this, Balance Sheet and Neraca Saldo would still show the cancelled sale
		if s.salesJournalServiceSSOT != nil {
			if err := s.salesJournalServiceSSOT.DeleteSalesJournal(sale.ID, tx); err != nil {
				log.Printf("⚠️ Warning: Failed to remove SSOT journal entries for Sale #%d: %v", sale.ID, err)
				// Continue - V2 journal was already cleaned up
			} else {
				log.Printf("🗑️ Removed SSOT SALE journal entries for cancelled Sale #%d", sale.ID)
			}
		}
	}

	// 3. Remove SALES_PAYMENT journal entries (from ProcessPayment)
	var salePayments []models.SalePayment
	if err := tx.Where("sale_id = ?", sale.ID).Find(&salePayments).Error; err == nil && len(salePayments) > 0 {
		log.Printf("🗑️ Found %d payment records for cancelled Sale #%d, cleaning up...", len(salePayments), sale.ID)
		for _, payment := range salePayments {
			// Delete the payment journal entry (SALES_PAYMENT type)
			sourceID := uint64(payment.ID)
			var paymentJournal models.SSOTJournalEntry
			if err := tx.Where("source_type = ? AND source_id = ?", "SALES_PAYMENT", sourceID).
				First(&paymentJournal).Error; err == nil {
				// Get journal lines for COA balance reversal
				var journalLines []models.SSOTJournalLine
				if err := tx.Where("journal_id = ?", paymentJournal.ID).Find(&journalLines).Error; err == nil {
					for _, line := range journalLines {
						// Reverse COA balance
						negDebit := line.DebitAmount.Neg()
						negCredit := line.CreditAmount.Neg()
						if err := s.salesJournalService.updateAccountBalanceForPayment(tx, line.AccountID, negDebit, negCredit); err != nil {
							log.Printf("⚠️ Warning: Failed to reverse COA balance for payment journal account %d: %v", line.AccountID, err)
						}
						if err := s.salesJournalService.updateParentAccountBalances(tx, uint(line.AccountID)); err != nil {
							log.Printf("⚠️ Warning: Failed to update parent balances for account %d: %v", line.AccountID, err)
						}
					}
				}

				// Delete journal lines and entry
				tx.Where("journal_id = ?", paymentJournal.ID).Delete(&models.SSOTJournalLine{})
				tx.Delete(&paymentJournal)
				log.Printf("🗑️ Deleted SALES_PAYMENT journal for payment #%d", payment.ID)
			}

			// 4. Reverse cash/bank balance for this payment
			if payment.CashBankID != nil && *payment.CashBankID > 0 {
				var cashBank models.CashBank
				if err := tx.First(&cashBank, *payment.CashBankID).Error; err == nil {
					oldBalance := cashBank.Balance
					cashBank.Balance -= payment.Amount // Reverse: subtract the payment that was added
					if err := tx.Save(&cashBank).Error; err != nil {
						log.Printf("⚠️ Warning: Failed to reverse cash/bank balance for payment #%d: %v", payment.ID, err)
					} else {
						log.Printf("✅ Cash/bank balance reversed for payment #%d: %s %.2f → %.2f (-%.2f)",
							payment.ID, cashBank.Name, oldBalance, cashBank.Balance, payment.Amount)
					}
				}
			}
		}

		// Soft-delete cash/bank transaction records for SALES_PAYMENT
		tx.Exec(`
			UPDATE cash_bank_transactions 
			SET deleted_at = CURRENT_TIMESTAMP
			WHERE reference_type = 'SALES_PAYMENT' AND reference_id IN (
				SELECT id FROM sale_payments WHERE sale_id = ?
			) AND deleted_at IS NULL
		`, sale.ID)
		log.Printf("🗑️ Soft-deleted SALES_PAYMENT cash/bank transactions for Sale #%d", sale.ID)
	}

	// 5. Reverse cash/bank for immediate-payment sales (CASH/BANK auto-paid on invoice)
	pm := strings.ToUpper(strings.TrimSpace(sale.PaymentMethodType))
	if (oldStatus == "INVOICED" || oldStatus == "PAID") && (pm == "CASH" || strings.HasPrefix(pm, "BANK")) {
		if sale.CashBankID != nil && *sale.CashBankID > 0 {
			var cashBank models.CashBank
			if err := tx.First(&cashBank, *sale.CashBankID).Error; err == nil {
				oldBalance := cashBank.Balance
				cashBank.Balance -= sale.TotalAmount // Reverse: subtract the auto-payment
				if err := tx.Save(&cashBank).Error; err != nil {
					log.Printf("⚠️ Warning: Failed to reverse cash/bank for immediate-payment sale: %v", err)
				} else {
					log.Printf("✅ Cash/bank reversed for auto-paid Sale #%d: %s %.2f → %.2f (-%.2f)",
						sale.ID, cashBank.Name, oldBalance, cashBank.Balance, sale.TotalAmount)
				}
			}
		}

		// Soft-delete the SALE cash/bank transaction record
		tx.Exec(`
			UPDATE cash_bank_transactions 
			SET deleted_at = CURRENT_TIMESTAMP
			WHERE reference_type = 'SALE' AND reference_id = ? AND deleted_at IS NULL
		`, sale.ID)
		log.Printf("🗑️ Soft-deleted SALE cash/bank transaction for Sale #%d", sale.ID)
	}

	// 6. Restore stock if needed
	if s.stockService != nil && (oldStatus == "INVOICED" || oldStatus == "PAID") {
		for _, item := range sale.SaleItems {
			if item.Product.IsService {
				// Services don't need stock restoration
				continue
			}

			if item.Product.IsAssembly {
				// 🚀 AUTO-KITTING RESTORE LOGIC: Restore component stocks
				var bom models.BOM
				if err := tx.Preload("Items").Where("target_product_id = ?", item.ProductID).First(&bom).Error; err == nil {
					for _, bomItem := range bom.Items {
						requiredQty := int(bomItem.RequirementQuantity * float64(item.Quantity))
						if requiredQty > 0 {
							if err := s.stockService.RestoreStock(bomItem.ComponentProductID, requiredQty, tx); err != nil {
								log.Printf("⚠️ Warning: Failed to restore stock for component %d of assembly %d: %v", bomItem.ComponentProductID, item.ProductID, err)
							}
						}
					}
					log.Printf("✅ Kitting Restore: Component stocks restored for cancelled assembly %d (%s)", item.ProductID, item.Product.Name)
				} else {
					log.Printf("⚠️ Warning: BOM not found for assembly %d during stock restoration", item.ProductID)
				}
				continue
			}

			// Standard stock restoration
			if err := s.stockService.RestoreStock(item.ProductID, item.Quantity, tx); err != nil {
				log.Printf("⚠️ Warning: Failed to restore stock for product %d: %v", item.ProductID, err)
			}
		}
	}

	log.Printf("❌ Sale #%d cancelled (status: %s → CANCELLED) - Full accounting cleanup completed", sale.ID, oldStatus)

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

// GetSales retrieves sales with filters
func (s *SalesServiceV2) GetSales(filter models.SalesFilter) (*models.SalesResult, error) {
	query := s.db.Model(&models.Sale{}).Preload("Customer").Preload("SaleItems").Preload("Project")

	// Apply filters
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
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
		Preload("SalePayments").
		First(&sale, saleID).Error; err != nil {
		return nil, fmt.Errorf("sale not found")
	}
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

	for _, it := range req.Items {
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

		available := p.Stock
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
				warning = "Stok 0: produk tidak bisa dijual"
			} else {
				warning = "Stok 0: produk tidak bisa dijual; " + warning
			}
		}
		if !isSufficient {
			if warning == "" {
				warning = fmt.Sprintf("Stock tidak cukup. Tersedia %d, diminta %d", available, reqQty)
			} else {
				warning += fmt.Sprintf("; stok tidak cukup (tersedia %d, diminta %d)", available, reqQty)
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

// GetSalesSummary gets sales summary with date filtering
func (s *SalesServiceV2) GetSalesSummary(startDate, endDate *time.Time) (*models.SalesSummaryResponse, error) {
	log.Printf("📊 Getting sales summary (start: %v, end: %v)", startDate, endDate)

	query := s.db.Model(&models.Sale{})

	// Apply date filters if provided
	if startDate != nil {
		query = query.Where("date >= ?", *startDate)
	}
	if endDate != nil {
		query = query.Where("date <= ?", *endDate)
	}

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
	topCustomers, err := s.getTopCustomers(startDate, endDate)
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
func (s *SalesServiceV2) getTopCustomers(startDate, endDate *time.Time) ([]models.CustomerSales, error) {
	query := s.db.Table("sales").
		Select("sales.customer_id, contacts.name as customer_name, COUNT(sales.id) as total_orders, SUM(sales.total_amount) as total_amount").
		Joins("LEFT JOIN contacts ON contacts.id = sales.customer_id").
		Where("sales.deleted_at IS NULL").
		Group("sales.customer_id, contacts.name").
		Order("total_amount DESC").
		Limit(5)

	// Apply date filters if provided
	if startDate != nil {
		query = query.Where("sales.date >= ?", *startDate)
	}
	if endDate != nil {
		query = query.Where("sales.date <= ?", *endDate)
	}

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
		Where("project_id = ? AND deleted_at IS NULL AND status IN ('INVOICED', 'PAID', 'CONFIRMED', 'DRAFT')", projectID).
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
