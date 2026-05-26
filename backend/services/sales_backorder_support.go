package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const salesBackorderSettlementNotes = "SALE_BACKORDER_SETTLEMENT"

const (
	salesBackorderClearingAccountCode = "2103-009"
	salesBackorderClearingAccountName = "ESTIMATED HPP BACKORDER"
)

var trackedSalesBackorderStatuses = []string{
	models.SaleStatusInvoiced,
	models.SaleStatusPaid,
	models.SaleStatusOverdue,
	models.SaleStatusPartiallyPaid,
	models.SaleStatusCompleted,
}

type saleItemFulfillmentSnapshot struct {
	availableQuantity   int
	fulfilledQuantity   int
	backorderQuantity   int
	estimatedUnitCost   float64
	estimatedCOGSAmount float64
	actualCOGSAmount    float64
	fulfillmentStatus   string
}

type BackorderBackfillAllocationResult struct {
	SaleID          uint    `json:"sale_id"`
	SaleItemID      uint    `json:"sale_item_id"`
	SaleCode        string  `json:"sale_code"`
	Quantity        int     `json:"quantity"`
	EstimatedAmount float64 `json:"estimated_amount"`
	ActualAmount    float64 `json:"actual_amount"`
	VarianceAmount  float64 `json:"variance_amount"`
}

type ProductBackorderBackfillResult struct {
	ProductID                uint                                `json:"product_id"`
	ProductCode              string                              `json:"product_code"`
	ProductName              string                              `json:"product_name"`
	AllocatedQuantity        int                                 `json:"allocated_quantity"`
	RemainingStock           int                                 `json:"remaining_stock"`
	PendingBackorderQuantity int                                 `json:"pending_backorder_quantity"`
	JournalID                *uint64                             `json:"journal_id,omitempty"`
	AffectedSaleIDs          []uint                              `json:"affected_sale_ids"`
	Allocations              []BackorderBackfillAllocationResult `json:"allocations"`
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func resolveSaleEstimatedUnitCost(product *models.Product) float64 {
	if product == nil {
		return 0
	}
	if product.CostPrice > 0 {
		return product.CostPrice
	}
	if product.PurchasePrice > 0 {
		return product.PurchasePrice
	}
	return 0
}

func resolveSaleFulfillmentStatus(item *models.SaleItem, product *models.Product) string {
	if product != nil && (product.IsService || product.IsAssembly) {
		return models.SaleFulfillmentStatusService
	}
	if item == nil {
		return models.SaleFulfillmentStatusInStock
	}
	if item.BackorderQuantity <= 0 {
		if item.AvailableQuantity >= item.Quantity {
			return models.SaleFulfillmentStatusInStock
		}
		return models.SaleFulfillmentStatusFulfilled
	}
	if item.FulfilledQuantity <= 0 {
		return models.SaleFulfillmentStatusBackordered
	}
	return models.SaleFulfillmentStatusPartialBackorder
}

func buildSaleItemFulfillmentSnapshot(product *models.Product, requestedQty int, availableQty int) saleItemFulfillmentSnapshot {
	if requestedQty < 0 {
		requestedQty = 0
	}
	if product != nil && (product.IsService || product.IsAssembly) {
		return saleItemFulfillmentSnapshot{
			availableQuantity:   requestedQty,
			fulfilledQuantity:   requestedQty,
			backorderQuantity:   0,
			estimatedUnitCost:   resolveSaleEstimatedUnitCost(product),
			estimatedCOGSAmount: float64(requestedQty) * resolveSaleEstimatedUnitCost(product),
			actualCOGSAmount:    float64(requestedQty) * resolveSaleEstimatedUnitCost(product),
			fulfillmentStatus:   models.SaleFulfillmentStatusService,
		}
	}

	availableQty = maxInt(0, minInt(availableQty, requestedQty))
	backorderQty := requestedQty - availableQty
	estimatedUnitCost := resolveSaleEstimatedUnitCost(product)

	snapshot := saleItemFulfillmentSnapshot{
		availableQuantity:   availableQty,
		fulfilledQuantity:   availableQty,
		backorderQuantity:   backorderQty,
		estimatedUnitCost:   estimatedUnitCost,
		estimatedCOGSAmount: float64(requestedQty) * estimatedUnitCost,
		actualCOGSAmount:    float64(availableQty) * estimatedUnitCost,
	}

	tempItem := &models.SaleItem{
		Quantity:          requestedQty,
		AvailableQuantity: availableQty,
		FulfilledQuantity: availableQty,
		BackorderQuantity: backorderQty,
	}
	snapshot.fulfillmentStatus = resolveSaleFulfillmentStatus(tempItem, product)
	return snapshot
}

func currentRemainingSaleProductStock(product *models.Product, remainingProductStock map[uint]int) int {
	if product == nil {
		return 0
	}
	if product.IsService || product.IsAssembly {
		return 0
	}
	if remainingProductStock == nil {
		return maxInt(0, product.Stock)
	}

	if remainingQty, exists := remainingProductStock[product.ID]; exists {
		return maxInt(0, remainingQty)
	}

	remainingQty := maxInt(0, product.Stock)
	remainingProductStock[product.ID] = remainingQty
	return remainingQty
}

func allocateSaleProductStock(product *models.Product, requestedQty int, remainingProductStock map[uint]int) int {
	if requestedQty <= 0 {
		return 0
	}
	if product == nil {
		return 0
	}
	if product.IsService || product.IsAssembly {
		return requestedQty
	}

	availableQty := currentRemainingSaleProductStock(product, remainingProductStock)
	allocatedQty := minInt(requestedQty, availableQty)

	if remainingProductStock != nil {
		remainingProductStock[product.ID] = maxInt(0, availableQty-allocatedQty)
	}

	return allocatedQty
}

func buildSaleItemFulfillmentSnapshotWithRemainingStock(product *models.Product, requestedQty int, remainingProductStock map[uint]int) saleItemFulfillmentSnapshot {
	availableQty := allocateSaleProductStock(product, requestedQty, remainingProductStock)
	return buildSaleItemFulfillmentSnapshot(product, requestedQty, availableQty)
}

func applySaleItemFulfillmentSnapshot(item *models.SaleItem, snapshot saleItemFulfillmentSnapshot) {
	if item == nil {
		return
	}
	item.AvailableQuantity = snapshot.availableQuantity
	item.FulfilledQuantity = snapshot.fulfilledQuantity
	item.BackorderQuantity = snapshot.backorderQuantity
	item.EstimatedUnitCost = snapshot.estimatedUnitCost
	item.EstimatedCOGSAmount = snapshot.estimatedCOGSAmount
	item.ActualCOGSAmount = snapshot.actualCOGSAmount
	item.COGSVarianceAmount = item.ActualCOGSAmount - item.EstimatedCOGSAmount
	item.FulfillmentStatus = snapshot.fulfillmentStatus
}

func applySaleEstimatedUnitCostOverride(snapshot *saleItemFulfillmentSnapshot, requestedQty int, estimatedUnitCost *float64) {
	if snapshot == nil || estimatedUnitCost == nil || *estimatedUnitCost <= 0 {
		return
	}
	if requestedQty < 0 {
		requestedQty = 0
	}

	snapshot.estimatedUnitCost = *estimatedUnitCost
	snapshot.estimatedCOGSAmount = float64(requestedQty) * snapshot.estimatedUnitCost
	snapshot.actualCOGSAmount = float64(snapshot.fulfilledQuantity) * snapshot.estimatedUnitCost
}

func syncSaleFulfillmentStatusesForSales(tx *gorm.DB, saleIDs map[uint]struct{}) error {
	if tx == nil || len(saleIDs) == 0 {
		return nil
	}

	ids := make([]uint, 0, len(saleIDs))
	for saleID := range saleIDs {
		if saleID > 0 {
			ids = append(ids, saleID)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	var saleItems []models.SaleItem
	if err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Product").
		Where("sale_id IN ? AND deleted_at IS NULL", ids).
		Find(&saleItems).Error; err != nil {
		return fmt.Errorf("failed to reload sale fulfillment statuses: %w", err)
	}

	for _, item := range saleItems {
		var product *models.Product
		if item.Product.ID != 0 {
			product = &item.Product
		}
		nextStatus := resolveSaleFulfillmentStatus(&item, product)
		if nextStatus == item.FulfillmentStatus {
			continue
		}

		if err := tx.Model(&models.SaleItem{}).
			Where("id = ?", item.ID).
			Updates(map[string]interface{}{
				"fulfillment_status": nextStatus,
				"updated_at":         time.Now(),
			}).Error; err != nil {
			return fmt.Errorf("failed to sync fulfillment status for sale item %d: %w", item.ID, err)
		}
	}

	return nil
}

func ensureSalesBackorderClearingAccountTx(tx *gorm.DB) (*models.Account, error) {
	var account models.Account
	err := tx.Where("code = ? AND deleted_at IS NULL", salesBackorderClearingAccountCode).First(&account).Error
	if err == nil {
		return &account, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to load backorder clearing account: %w", err)
	}

	var parent models.Account
	if err := tx.Where("code = ? AND deleted_at IS NULL", "2103").First(&parent).Error; err != nil {
		return nil, fmt.Errorf("failed to load liability parent 2103: %w", err)
	}

	account = models.Account{
		Code:             salesBackorderClearingAccountCode,
		Name:             salesBackorderClearingAccountName,
		Description:      "Liability clearing account for estimated sales backorders",
		Type:             models.AccountTypeLiability,
		Category:         models.CategoryCurrentLiability,
		ParentID:         &parent.ID,
		Level:            4,
		IsHeader:         false,
		IsActive:         true,
		IsSystemCritical: true,
	}
	if err := tx.Create(&account).Error; err != nil {
		return nil, fmt.Errorf("failed to create backorder clearing account: %w", err)
	}
	return &account, nil
}

func resolveBackorderSettlementAccounts(tx *gorm.DB, helper *TaxAccountHelper) (*models.Account, *models.Account, *models.Account, error) {
	if helper == nil {
		helper = NewTaxAccountHelper(tx)
	}

	cogsAccount, err := helper.GetCOGSAccount(tx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to resolve COGS account: %w", err)
	}
	inventoryAccount, err := helper.GetInventoryAccount(tx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to resolve inventory account: %w", err)
	}
	clearingAccount, err := ensureSalesBackorderClearingAccountTx(tx)
	if err != nil {
		return nil, nil, nil, err
	}

	return cogsAccount, inventoryAccount, clearingAccount, nil
}

func effectivePurchaseUnitCost(item *models.PurchaseItem) float64 {
	if item == nil {
		return 0
	}
	if item.Quantity > 0 && item.TotalPrice > 0 {
		return item.TotalPrice / float64(item.Quantity)
	}
	return item.UnitPrice
}

func (s *PurchaseService) settleBackordersForApprovedPurchase(purchase *models.Purchase) (map[uint]struct{}, error) {
	affectedProductIDs := make(map[uint]struct{})
	if purchase == nil {
		return affectedProductIDs, nil
	}
	switch purchase.Status {
	case models.PurchaseStatusApproved, models.PurchaseStatusCompleted, models.PurchaseStatusPaid:
	default:
		return affectedProductIDs, nil
	}
	if s.unifiedJournalService == nil {
		log.Printf("⚠️ [BACKORDER] Unified journal service not available, skipping backorder settlement for purchase %s", purchase.Code)
		return affectedProductIDs, nil
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Fix sequence desync: ensure sale_backorder_allocations_id_seq is ahead of max existing ID
		tx.Exec(`SELECT setval('sale_backorder_allocations_id_seq', COALESCE((SELECT MAX(id) FROM sale_backorder_allocations), 0) + 1, false)`)

		cogsAccount, inventoryAccount, clearingAccount, err := resolveBackorderSettlementAccounts(tx, NewTaxAccountHelper(tx))
		if err != nil {
			return err
		}

		var totalEstimated float64
		var totalActual float64
		totalAllocatedQty := 0
		affectedSaleIDs := make(map[uint]struct{})

		for _, purchaseItem := range purchase.PurchaseItems {
			if purchaseItem.ProductID == nil || *purchaseItem.ProductID == 0 {
				continue
			}
			if purchaseItem.IsFixedAsset || purchaseItem.ItemType == models.PurchaseItemTypeAsset {
				continue
			}

			var product models.Product
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&product, *purchaseItem.ProductID).Error; err != nil {
				return fmt.Errorf("failed to lock product %d for backorder settlement: %w", *purchaseItem.ProductID, err)
			}
			if product.IsService || product.IsAssembly {
				continue
			}

			remainingQty := minInt(purchaseItem.Quantity, product.Stock)
			if remainingQty <= 0 {
				continue
			}

			var saleItems []models.SaleItem
			if err := tx.
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Joins("JOIN sales ON sales.id = sale_items.sale_id").
				Preload("Sale").
				Where("sale_items.product_id = ? AND sale_items.backorder_quantity > 0 AND sale_items.deleted_at IS NULL", product.ID).
				Where("sales.status IN ? AND sales.deleted_at IS NULL", trackedSalesBackorderStatuses).
				Order("sales.date ASC, sales.id ASC, sale_items.id ASC").
				Find(&saleItems).Error; err != nil {
				return fmt.Errorf("failed to load open sale backorders for product %s: %w", product.Name, err)
			}

			if len(saleItems) == 0 {
				continue
			}

			actualUnitCost := purchaseItemLandedUnitCost(purchase, &purchaseItem)
			allocatedForProduct := 0

			for _, saleItem := range saleItems {
				if remainingQty <= 0 {
					break
				}

				allocateQty := minInt(saleItem.BackorderQuantity, remainingQty)
				if allocateQty <= 0 {
					continue
				}

				estimatedAmount := float64(allocateQty) * saleItem.EstimatedUnitCost
				actualAmount := float64(allocateQty) * actualUnitCost
				now := time.Now()

				allocation := models.SaleBackorderAllocation{
					SaleID:            saleItem.SaleID,
					SaleItemID:        saleItem.ID,
					ProductID:         product.ID,
					PurchaseID:        &purchase.ID,
					PurchaseItemID:    &purchaseItem.ID,
					Quantity:          allocateQty,
					EstimatedUnitCost: saleItem.EstimatedUnitCost,
					ActualUnitCost:    actualUnitCost,
					EstimatedAmount:   estimatedAmount,
					ActualAmount:      actualAmount,
					VarianceAmount:    actualAmount - estimatedAmount,
					Status:            models.SaleBackorderAllocationStatusApplied,
					AppliedAt:         &now,
					Notes:             fmt.Sprintf("Settled from purchase %s", purchase.Code),
				}
				if err := tx.Create(&allocation).Error; err != nil {
					return fmt.Errorf("failed to create backorder allocation for sale item %d: %w", saleItem.ID, err)
				}

				saleItem.BackorderQuantity -= allocateQty
				saleItem.FulfilledQuantity += allocateQty
				saleItem.ActualCOGSAmount += actualAmount
				saleItem.COGSVarianceAmount = saleItem.ActualCOGSAmount - saleItem.EstimatedCOGSAmount
				saleItem.FulfillmentStatus = resolveSaleFulfillmentStatus(&saleItem, &product)

				if err := tx.Model(&models.SaleItem{}).Where("id = ?", saleItem.ID).Updates(map[string]interface{}{
					"backorder_quantity":   saleItem.BackorderQuantity,
					"fulfilled_quantity":   saleItem.FulfilledQuantity,
					"actual_cogs_amount":   saleItem.ActualCOGSAmount,
					"cogs_variance_amount": saleItem.COGSVarianceAmount,
					"fulfillment_status":   saleItem.FulfillmentStatus,
					"updated_at":           time.Now(),
				}).Error; err != nil {
					return fmt.Errorf("failed to update sale item %d after backorder settlement: %w", saleItem.ID, err)
				}

				remainingQty -= allocateQty
				allocatedForProduct += allocateQty
				totalAllocatedQty += allocateQty
				totalEstimated += estimatedAmount
				totalActual += actualAmount
				affectedSaleIDs[saleItem.SaleID] = struct{}{}
			}

			if allocatedForProduct > 0 {
				if product.Stock < allocatedForProduct {
					return fmt.Errorf("insufficient post-purchase stock to settle backorder for product '%s': available %d, allocated %d", product.Name, product.Stock, allocatedForProduct)
				}
				product.Stock -= allocatedForProduct
				if err := tx.Model(&models.Product{}).Where("id = ?", product.ID).Updates(map[string]interface{}{
					"stock":      product.Stock,
					"updated_at": time.Now(),
				}).Error; err != nil {
					return fmt.Errorf("failed to reduce product stock for backorder settlement on product %d: %w", product.ID, err)
				}
				affectedProductIDs[product.ID] = struct{}{}
			}
		}

		if totalAllocatedQty == 0 {
			return nil
		}

		if err := syncSaleFulfillmentStatusesForSales(tx, affectedSaleIDs); err != nil {
			return err
		}

		decEstimated := decimal.NewFromFloat(totalEstimated).Round(2)
		decActual := decimal.NewFromFloat(totalActual).Round(2)
		decVariance := decActual.Sub(decEstimated)

		journalLines := []JournalLineRequest{
			{
				AccountID:    uint64(clearingAccount.ID),
				DebitAmount:  decEstimated,
				CreditAmount: decimal.Zero,
				Description:  fmt.Sprintf("Backorder clearing settlement - %s", purchase.Code),
			},
			{
				AccountID:    uint64(inventoryAccount.ID),
				DebitAmount:  decimal.Zero,
				CreditAmount: decActual,
				Description:  fmt.Sprintf("Inventory consumed by backorder settlement - %s", purchase.Code),
			},
		}

		tolerance := decimal.NewFromFloat(0.005)
		if decVariance.GreaterThan(tolerance) {
			journalLines = append(journalLines, JournalLineRequest{
				AccountID:    uint64(cogsAccount.ID),
				DebitAmount:  decVariance,
				CreditAmount: decimal.Zero,
				Description:  fmt.Sprintf("COGS variance on backorder settlement - %s", purchase.Code),
			})
		} else if decVariance.LessThan(tolerance.Neg()) {
			journalLines = append(journalLines, JournalLineRequest{
				AccountID:    uint64(cogsAccount.ID),
				DebitAmount:  decimal.Zero,
				CreditAmount: decVariance.Abs(),
				Description:  fmt.Sprintf("COGS variance on backorder settlement - %s", purchase.Code),
			})
		}

		sourceID := uint64(purchase.ID)
		var adjustmentOf *uint64
		if len(affectedSaleIDs) == 1 {
			for saleID := range affectedSaleIDs {
				adjustmentOf = findPostedSourceJournalIDForAdjustment(tx, models.SSOTSourceTypeSale, uint64(saleID), purchase.Date)
			}
		}
		req := &JournalEntryRequest{
			EntryDate:              purchase.Date,
			Reference:              fmt.Sprintf("BO-HPP-%s", purchase.Code),
			Description:            fmt.Sprintf("Sales backorder settlement - %s", purchase.Code),
			Notes:                  salesBackorderSettlementNotes,
			Lines:                  journalLines,
			CreatedBy:              uint64(purchase.UserID),
			SourceType:             models.SSOTSourceTypeAdjustment,
			SourceID:               sourceID,
			AdjustmentOf:           adjustmentOf,
			AdjustmentCategoryCode: models.JournalAdjustmentCategorySalesBackorderHPP,
			AutoPost:               true,
			SystemGenerated:        true,
		}

		if _, err := s.unifiedJournalService.CreateJournalEntryWithTx(tx, req); err != nil {
			return fmt.Errorf("failed to create sales backorder settlement journal for purchase %s: %w", purchase.Code, err)
		}

		return nil
	})

	return affectedProductIDs, err
}

func BackfillSalesBackordersForProduct(db *gorm.DB, productID uint, userID uint) (*ProductBackorderBackfillResult, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}
	if productID == 0 {
		return nil, fmt.Errorf("product ID is required")
	}

	result := &ProductBackorderBackfillResult{ProductID: productID}
	journalService := NewUnifiedJournalService(db)

	err := db.Transaction(func(tx *gorm.DB) error {
		// Fix sequence desync: ensure sale_backorder_allocations_id_seq is ahead of max existing ID
		tx.Exec(`SELECT setval('sale_backorder_allocations_id_seq', COALESCE((SELECT MAX(id) FROM sale_backorder_allocations), 0) + 1, false)`)

		cogsAccount, inventoryAccount, clearingAccount, err := resolveBackorderSettlementAccounts(tx, NewTaxAccountHelper(tx))
		if err != nil {
			return err
		}

		var product models.Product
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&product, productID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("product not found")
			}
			return fmt.Errorf("failed to lock product %d for backorder backfill: %w", productID, err)
		}

		result.ProductID = product.ID
		result.ProductCode = product.Code
		result.ProductName = product.Name
		result.RemainingStock = product.Stock

		if product.IsService || product.IsAssembly {
			return fmt.Errorf("manual fulfillment backfill is only allowed for stock-managed products")
		}

		if product.Stock <= 0 {
			return populateBackfillPendingQuantity(tx, result)
		}

		var saleItems []models.SaleItem
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Joins("JOIN sales ON sales.id = sale_items.sale_id").
			Preload("Sale").
			Where("sale_items.product_id = ? AND sale_items.backorder_quantity > 0 AND sale_items.deleted_at IS NULL", product.ID).
			Where("sales.status IN ? AND sales.deleted_at IS NULL", trackedSalesBackorderStatuses).
			Order("sales.date ASC, sales.id ASC, sale_items.id ASC").
			Find(&saleItems).Error; err != nil {
			return fmt.Errorf("failed to load open sale backorders for product %s: %w", product.Name, err)
		}

		if len(saleItems) == 0 {
			return populateBackfillPendingQuantity(tx, result)
		}

		remainingQty := product.Stock
		actualUnitCost := resolveSaleEstimatedUnitCost(&product)
		totalEstimated := 0.0
		totalActual := 0.0
		affectedSaleIDs := make(map[uint]struct{})
		now := time.Now()

		for _, saleItem := range saleItems {
			if remainingQty <= 0 {
				break
			}

			allocateQty := minInt(saleItem.BackorderQuantity, remainingQty)
			if allocateQty <= 0 {
				continue
			}

			itemActualUnitCost := actualUnitCost
			if itemActualUnitCost <= 0 {
				itemActualUnitCost = saleItem.EstimatedUnitCost
			}

			estimatedAmount := float64(allocateQty) * saleItem.EstimatedUnitCost
			actualAmount := float64(allocateQty) * itemActualUnitCost
			allocation := models.SaleBackorderAllocation{
				SaleID:            saleItem.SaleID,
				SaleItemID:        saleItem.ID,
				ProductID:         product.ID,
				Quantity:          allocateQty,
				EstimatedUnitCost: saleItem.EstimatedUnitCost,
				ActualUnitCost:    itemActualUnitCost,
				EstimatedAmount:   estimatedAmount,
				ActualAmount:      actualAmount,
				VarianceAmount:    actualAmount - estimatedAmount,
				Status:            models.SaleBackorderAllocationStatusApplied,
				AppliedAt:         &now,
				Notes:             "Manual fulfillment backfill from available product stock",
			}
			if err := tx.Create(&allocation).Error; err != nil {
				return fmt.Errorf("failed to create backorder allocation for sale item %d: %w", saleItem.ID, err)
			}

			saleItem.BackorderQuantity -= allocateQty
			saleItem.FulfilledQuantity += allocateQty
			saleItem.ActualCOGSAmount += actualAmount
			saleItem.COGSVarianceAmount = saleItem.ActualCOGSAmount - saleItem.EstimatedCOGSAmount
			saleItem.FulfillmentStatus = resolveSaleFulfillmentStatus(&saleItem, &product)

			if err := tx.Model(&models.SaleItem{}).Where("id = ?", saleItem.ID).Updates(map[string]interface{}{
				"backorder_quantity":   saleItem.BackorderQuantity,
				"fulfilled_quantity":   saleItem.FulfilledQuantity,
				"actual_cogs_amount":   saleItem.ActualCOGSAmount,
				"cogs_variance_amount": saleItem.COGSVarianceAmount,
				"fulfillment_status":   saleItem.FulfillmentStatus,
				"updated_at":           time.Now(),
			}).Error; err != nil {
				return fmt.Errorf("failed to update sale item %d after manual backfill: %w", saleItem.ID, err)
			}

			remainingQty -= allocateQty
			result.AllocatedQuantity += allocateQty
			totalEstimated += estimatedAmount
			totalActual += actualAmount
			affectedSaleIDs[saleItem.SaleID] = struct{}{}
			result.Allocations = append(result.Allocations, BackorderBackfillAllocationResult{
				SaleID:          saleItem.SaleID,
				SaleItemID:      saleItem.ID,
				SaleCode:        saleItem.Sale.Code,
				Quantity:        allocateQty,
				EstimatedAmount: estimatedAmount,
				ActualAmount:    actualAmount,
				VarianceAmount:  actualAmount - estimatedAmount,
			})
		}

		if result.AllocatedQuantity == 0 {
			return populateBackfillPendingQuantity(tx, result)
		}

		product.Stock = remainingQty
		if err := tx.Model(&models.Product{}).Where("id = ?", product.ID).Updates(map[string]interface{}{
			"stock":      product.Stock,
			"updated_at": time.Now(),
		}).Error; err != nil {
			return fmt.Errorf("failed to reduce product stock for manual backfill on product %d: %w", product.ID, err)
		}
		result.RemainingStock = product.Stock

		if err := syncSaleFulfillmentStatusesForSales(tx, affectedSaleIDs); err != nil {
			return err
		}

		for saleID := range affectedSaleIDs {
			result.AffectedSaleIDs = append(result.AffectedSaleIDs, saleID)
		}

		decEstimated := decimal.NewFromFloat(totalEstimated).Round(2)
		decActual := decimal.NewFromFloat(totalActual).Round(2)
		decVariance := decActual.Sub(decEstimated)

		journalLines := []JournalLineRequest{
			{
				AccountID:    uint64(clearingAccount.ID),
				DebitAmount:  decEstimated,
				CreditAmount: decimal.Zero,
				Description:  fmt.Sprintf("Backorder clearing manual backfill - %s", product.Code),
			},
			{
				AccountID:    uint64(inventoryAccount.ID),
				DebitAmount:  decimal.Zero,
				CreditAmount: decActual,
				Description:  fmt.Sprintf("Inventory consumed by manual backfill - %s", product.Code),
			},
		}

		tolerance := decimal.NewFromFloat(0.005)
		if decVariance.GreaterThan(tolerance) {
			journalLines = append(journalLines, JournalLineRequest{
				AccountID:    uint64(cogsAccount.ID),
				DebitAmount:  decVariance,
				CreditAmount: decimal.Zero,
				Description:  fmt.Sprintf("COGS variance on manual backfill - %s", product.Code),
			})
		} else if decVariance.LessThan(tolerance.Neg()) {
			journalLines = append(journalLines, JournalLineRequest{
				AccountID:    uint64(cogsAccount.ID),
				DebitAmount:  decimal.Zero,
				CreditAmount: decVariance.Abs(),
				Description:  fmt.Sprintf("COGS variance on manual backfill - %s", product.Code),
			})
		}

		entryDate := time.Now()
		var adjustmentOf *uint64
		if len(affectedSaleIDs) == 1 {
			for saleID := range affectedSaleIDs {
				adjustmentOf = findPostedSourceJournalIDForAdjustment(tx, models.SSOTSourceTypeSale, uint64(saleID), entryDate)
			}
		}

		sourceID := uint64(product.ID)
		journal, err := journalService.CreateJournalEntryWithTx(tx, &JournalEntryRequest{
			EntryDate:              entryDate,
			Reference:              fmt.Sprintf("BO-HPP-MANUAL-%s-%s", product.Code, entryDate.Format("20060102150405")),
			Description:            fmt.Sprintf("Manual sales backorder fulfillment backfill - %s", product.Name),
			Notes:                  salesBackorderSettlementNotes,
			Lines:                  journalLines,
			CreatedBy:              uint64(userID),
			SourceType:             models.SSOTSourceTypeAdjustment,
			SourceID:               sourceID,
			AdjustmentOf:           adjustmentOf,
			AdjustmentCategoryCode: models.JournalAdjustmentCategorySalesBackorderHPP,
			AutoPost:               true,
			SystemGenerated:        true,
		})
		if err != nil {
			return fmt.Errorf("failed to create manual backorder fulfillment journal for product %s: %w", product.Code, err)
		}
		result.JournalID = &journal.ID

		return populateBackfillPendingQuantity(tx, result)
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func populateBackfillPendingQuantity(tx *gorm.DB, result *ProductBackorderBackfillResult) error {
	if tx == nil || result == nil || result.ProductID == 0 {
		return nil
	}

	type pendingRow struct {
		PendingBackorderQty int
	}
	var row pendingRow
	if err := tx.Table("sale_items").
		Select("COALESCE(SUM(sale_items.backorder_quantity), 0) AS pending_backorder_qty").
		Joins("JOIN sales ON sales.id = sale_items.sale_id").
		Where("sale_items.product_id = ? AND sale_items.backorder_quantity > 0 AND sale_items.deleted_at IS NULL", result.ProductID).
		Where("sales.status IN ? AND sales.deleted_at IS NULL", trackedSalesBackorderStatuses).
		Scan(&row).Error; err != nil {
		return fmt.Errorf("failed to query remaining backorder quantity for product %d: %w", result.ProductID, err)
	}

	result.PendingBackorderQuantity = row.PendingBackorderQty
	return nil
}

func (s *PurchaseService) reverseBackorderSettlementsForRejectedPurchase(purchase *models.Purchase) (map[uint]struct{}, error) {
	affectedProductIDs := make(map[uint]struct{})
	if purchase == nil {
		return affectedProductIDs, nil
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var allocations []models.SaleBackorderAllocation
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("purchase_id = ? AND status = ? AND deleted_at IS NULL", purchase.ID, models.SaleBackorderAllocationStatusApplied).
			Order("id ASC").
			Find(&allocations).Error; err != nil {
			return fmt.Errorf("failed to load applied sale backorder allocations for purchase %s: %w", purchase.Code, err)
		}

		if len(allocations) == 0 {
			return nil
		}

		for _, allocation := range allocations {
			var saleItem models.SaleItem
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&saleItem, allocation.SaleItemID).Error; err != nil {
				return fmt.Errorf("failed to load sale item %d for backorder reversal: %w", allocation.SaleItemID, err)
			}

			var product models.Product
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&product, allocation.ProductID).Error; err != nil {
				return fmt.Errorf("failed to load product %d for backorder reversal: %w", allocation.ProductID, err)
			}

			saleItem.BackorderQuantity += allocation.Quantity
			saleItem.FulfilledQuantity -= allocation.Quantity
			if saleItem.FulfilledQuantity < saleItem.AvailableQuantity {
				saleItem.FulfilledQuantity = saleItem.AvailableQuantity
			}
			saleItem.ActualCOGSAmount -= allocation.ActualAmount
			if saleItem.ActualCOGSAmount < 0 {
				saleItem.ActualCOGSAmount = 0
			}
			saleItem.COGSVarianceAmount = saleItem.ActualCOGSAmount - saleItem.EstimatedCOGSAmount
			saleItem.FulfillmentStatus = resolveSaleFulfillmentStatus(&saleItem, &product)

			if err := tx.Model(&models.SaleItem{}).Where("id = ?", saleItem.ID).Updates(map[string]interface{}{
				"backorder_quantity":   saleItem.BackorderQuantity,
				"fulfilled_quantity":   saleItem.FulfilledQuantity,
				"actual_cogs_amount":   saleItem.ActualCOGSAmount,
				"cogs_variance_amount": saleItem.COGSVarianceAmount,
				"fulfillment_status":   saleItem.FulfillmentStatus,
				"updated_at":           time.Now(),
			}).Error; err != nil {
				return fmt.Errorf("failed to reopen sale item %d backorder: %w", saleItem.ID, err)
			}

			product.Stock += allocation.Quantity
			if err := tx.Model(&models.Product{}).Where("id = ?", product.ID).Updates(map[string]interface{}{
				"stock":      product.Stock,
				"updated_at": time.Now(),
			}).Error; err != nil {
				return fmt.Errorf("failed to restore product stock for allocation %d: %w", allocation.ID, err)
			}
			affectedProductIDs[product.ID] = struct{}{}

			now := time.Now()
			if err := tx.Model(&models.SaleBackorderAllocation{}).Where("id = ?", allocation.ID).Updates(map[string]interface{}{
				"status":      models.SaleBackorderAllocationStatusReversed,
				"reversed_at": &now,
				"updated_at":  now,
			}).Error; err != nil {
				return fmt.Errorf("failed to mark sale backorder allocation %d as reversed: %w", allocation.ID, err)
			}
		}

		return nil
	}); err != nil {
		return affectedProductIDs, err
	}

	if s.unifiedJournalService == nil {
		return affectedProductIDs, nil
	}

	var journals []models.SSOTJournalEntry
	if err := s.db.
		Where("source_type = ? AND source_id = ? AND notes = ? AND deleted_at IS NULL AND reversed_by IS NULL", models.SSOTSourceTypeAdjustment, uint64(purchase.ID), salesBackorderSettlementNotes).
		Order("id ASC").
		Find(&journals).Error; err != nil {
		return affectedProductIDs, fmt.Errorf("failed to load backorder settlement journals for purchase %s: %w", purchase.Code, err)
	}

	for _, journal := range journals {
		if journal.Status == models.SSOTStatusDraft {
			if err := s.unifiedJournalService.DeleteJournalEntry(journal.ID); err != nil {
				return affectedProductIDs, fmt.Errorf("failed to delete draft backorder settlement journal %d: %w", journal.ID, err)
			}
			continue
		}

		if _, err := s.unifiedJournalService.ReverseJournalEntry(context.Background(), journal.ID, uint64(purchase.UserID), fmt.Sprintf("Purchase %s rejected", purchase.Code)); err != nil {
			return affectedProductIDs, fmt.Errorf("failed to reverse backorder settlement journal %d: %w", journal.ID, err)
		}
	}

	return affectedProductIDs, nil
}

func PopulateProductBackorderMetrics(tx *gorm.DB, products []models.Product) error {
	if tx == nil || len(products) == 0 {
		return nil
	}

	productIDs := make([]uint, 0, len(products))
	for _, product := range products {
		if product.ID > 0 {
			productIDs = append(productIDs, product.ID)
		}
	}
	if len(productIDs) == 0 {
		return nil
	}

	type metricRow struct {
		ProductID             uint
		PendingBackorderQty   int
		FulfilledBackorderQty int
		BackorderOrderCount   int
	}

	var rows []metricRow
	if err := tx.Table("sale_items").
		Select(`
			sale_items.product_id AS product_id,
			COALESCE(SUM(CASE WHEN sale_items.backorder_quantity > 0 THEN sale_items.backorder_quantity ELSE 0 END), 0) AS pending_backorder_qty,
			COALESCE(SUM(CASE
				WHEN sale_items.available_quantity < sale_items.fulfilled_quantity
				THEN sale_items.fulfilled_quantity - sale_items.available_quantity
				ELSE 0
			END), 0) AS fulfilled_backorder_qty,
			COUNT(DISTINCT CASE WHEN sale_items.backorder_quantity > 0 THEN sale_items.sale_id END) AS backorder_order_count
		`).
		Joins("JOIN sales ON sales.id = sale_items.sale_id").
		Where("sale_items.product_id IN ? AND sale_items.deleted_at IS NULL", productIDs).
		Where("sales.status IN ? AND sales.deleted_at IS NULL", trackedSalesBackorderStatuses).
		Group("sale_items.product_id").
		Scan(&rows).Error; err != nil {
		return fmt.Errorf("failed to query product backorder metrics: %w", err)
	}

	metricsByProduct := make(map[uint]metricRow, len(rows))
	for _, row := range rows {
		metricsByProduct[row.ProductID] = row
	}

	for idx := range products {
		metric := metricsByProduct[products[idx].ID]
		products[idx].PendingBackorderQty = metric.PendingBackorderQty
		products[idx].FulfilledBackorderQty = metric.FulfilledBackorderQty
		products[idx].BackorderOrderCount = metric.BackorderOrderCount
		products[idx].NetAvailableStock = products[idx].Stock - metric.PendingBackorderQty
	}

	return nil
}

func PopulateSingleProductBackorderMetrics(tx *gorm.DB, product *models.Product) error {
	if product == nil {
		return nil
	}
	products := []models.Product{*product}
	if err := PopulateProductBackorderMetrics(tx, products); err != nil {
		return err
	}
	*product = products[0]
	return nil
}
