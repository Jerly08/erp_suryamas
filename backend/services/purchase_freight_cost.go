package services

import (
	"app-sistem-akuntansi/models"
	"math"
)

func isInventoryValuationPurchaseItem(item *models.PurchaseItem) bool {
	if item == nil {
		return false
	}
	if item.IsFixedAsset || item.ItemType == models.PurchaseItemTypeAsset || item.ItemType == models.PurchaseItemTypePrepaid {
		return false
	}
	if item.ProductID == nil || *item.ProductID == 0 {
		return false
	}
	if item.Product != nil && (item.Product.IsService || item.Product.IsAssembly) {
		return false
	}
	return true
}

func purchaseFreightAllocationByItem(purchase *models.Purchase) map[uint]float64 {
	allocations := make(map[uint]float64)
	if purchase == nil || purchase.FreightCost <= 0 {
		return allocations
	}

	eligibleItems := make([]*models.PurchaseItem, 0)
	baseTotal := 0.0
	totalQty := 0
	for i := range purchase.PurchaseItems {
		item := &purchase.PurchaseItems[i]
		if !isInventoryValuationPurchaseItem(item) {
			continue
		}
		eligibleItems = append(eligibleItems, item)
		baseTotal += clampNonNegative(item.TotalPrice)
		if item.Quantity > 0 {
			totalQty += item.Quantity
		}
	}

	if len(eligibleItems) == 0 {
		return allocations
	}

	freightCost := sanitizeFloat(clampNonNegative(purchase.FreightCost))
	remaining := freightCost
	for idx, item := range eligibleItems {
		var allocation float64
		isLast := idx == len(eligibleItems)-1

		if isLast {
			allocation = remaining
		} else if baseTotal > 0 {
			allocation = freightCost * (clampNonNegative(item.TotalPrice) / baseTotal)
		} else if totalQty > 0 && item.Quantity > 0 {
			allocation = freightCost * (float64(item.Quantity) / float64(totalQty))
		} else {
			allocation = freightCost / float64(len(eligibleItems))
		}

		allocation = math.Round(allocation*100) / 100
		if !isLast {
			remaining -= allocation
		}
		allocations[item.ID] = sanitizeFloat(clampNonNegative(allocation))
	}

	return allocations
}

func purchaseItemFreightAllocation(purchase *models.Purchase, item *models.PurchaseItem) float64 {
	if purchase == nil || item == nil || item.ID == 0 {
		return 0
	}
	return purchaseFreightAllocationByItem(purchase)[item.ID]
}

func purchaseItemLandedTotal(purchase *models.Purchase, item *models.PurchaseItem) float64 {
	if item == nil {
		return 0
	}
	return sanitizeFloat(clampNonNegative(item.TotalPrice + purchaseItemFreightAllocation(purchase, item)))
}

func purchaseItemLandedUnitCost(purchase *models.Purchase, item *models.PurchaseItem) float64 {
	if item == nil {
		return 0
	}
	if item.Quantity > 0 {
		landedTotal := purchaseItemLandedTotal(purchase, item)
		if landedTotal > 0 {
			return landedTotal / float64(item.Quantity)
		}
	}
	return effectivePurchaseUnitCost(item)
}
