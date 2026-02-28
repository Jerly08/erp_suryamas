package services

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"log"

	"gorm.io/gorm"
)

// BOMService handles complex business logic for Bills of Materials
type BOMService struct {
	DB *gorm.DB
}

// NewBOMService creates a new BOMService
func NewBOMService(db *gorm.DB) *BOMService {
	return &BOMService{DB: db}
}

// SyncBOMCostsForProduct synchronizes the unit_price of all BOM items that use the given product,
// recalculates the item total cost, and then recalculates the parent BOM's total cost.
func (s *BOMService) SyncBOMCostsForProduct(productID uint, newPrice float64) error {
	log.Printf("Starting BOM cost sync for product %d with new price %.2f", productID, newPrice)

	return s.DB.Transaction(func(tx *gorm.DB) error {
		// 1. Find all BOMItems that use this product
		var affectedItems []models.BOMItem
		if err := tx.Where("component_product_id = ?", productID).Find(&affectedItems).Error; err != nil {
			return fmt.Errorf("failed to find affected BOM items: %v", err)
		}

		if len(affectedItems) == 0 {
			// No BOM uses this product, we're done
			return nil
		}

		// Keep track of which BOMs need their totals recalculated
		affectedBOMIDs := make(map[uint]bool)

		// 2. Update the unit_price and total_cost of the affected items
		for _, item := range affectedItems {
			// Only update if the price actually changed
			if item.UnitPrice != newPrice {
				newTotalCost := item.RequirementQuantity * newPrice

				if err := tx.Model(&item).Updates(map[string]interface{}{
					"unit_price": newPrice,
					"total_cost": newTotalCost,
				}).Error; err != nil {
					return fmt.Errorf("failed to update BOM item %d: %v", item.ID, err)
				}

				affectedBOMIDs[item.BOMID] = true
			}
		}

		// 3. For each affected BOM, recalculate its TotalCost
		for bomID := range affectedBOMIDs {
			// Fetch all items for this BOM to calculate the new total
			var allItems []models.BOMItem
			if err := tx.Where("bom_id = ?", bomID).Find(&allItems).Error; err != nil {
				return fmt.Errorf("failed to fetch items for BOM %d: %v", bomID, err)
			}

			var newBOMTotalCost float64 = 0
			for _, item := range allItems {
				newBOMTotalCost += item.TotalCost
			}

			// Update the BOM's total cost (stored as cost per 1 output unit)
			if err := tx.Model(&models.BOM{}).Where("id = ?", bomID).Update("total_cost", newBOMTotalCost).Error; err != nil {
				return fmt.Errorf("failed to update BOM %d total cost: %v", bomID, err)
			}

			// 4. Update the cost percentages for all items in this BOM based on the new total
			for _, item := range allItems {
				newPercentage := 0.0
				if newBOMTotalCost > 0 {
					newPercentage = (item.TotalCost / newBOMTotalCost) * 100
				}
				if err := tx.Model(&item).Update("cost_percentage", newPercentage).Error; err != nil {
					return fmt.Errorf("failed to update cost percentage for BOM item %d: %v", item.ID, err)
				}
			}

			// 5. Update the Target Product's Cost/Purchase Price
			// newBOMTotalCost = sum(reqQty * unitPrice) = per-unit cost.
			// TargetQuantity is a batch size, not a divisor of unit cost.
			var bom models.BOM
			if err := tx.Select("target_product_id").First(&bom, bomID).Error; err != nil {
				return fmt.Errorf("failed to fetch BOM %d to update target product: %v", bomID, err)
			}

			if err := tx.Model(&models.Product{}).Where("id = ?", bom.TargetProductID).Updates(map[string]interface{}{
				"cost_price":     newBOMTotalCost,
				"purchase_price": newBOMTotalCost,
			}).Error; err != nil {
				return fmt.Errorf("failed to update target product %d for BOM %d: %v", bom.TargetProductID, bomID, err)
			}

			log.Printf("Successfully synced BOM %d to new total cost %.2f", bomID, newBOMTotalCost)
		}

		log.Printf("Successfully synced costs for %d BOM items using product %d", len(affectedItems), productID)
		return nil
	})
}
