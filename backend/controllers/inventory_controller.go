package controllers

import (
	"app-sistem-akuntansi/models"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type InventoryController struct {
	DB *gorm.DB
}

func NewInventoryController(db *gorm.DB) *InventoryController {
	return &InventoryController{DB: db}
}

// GetInventoryMovements retrieves inventory movements with filters
func (ic *InventoryController) GetInventoryMovements(c *gin.Context) {
	var movements []models.Inventory
	query := ic.DB.Preload("Product")

	// Filter by product ID
	if productID := c.Query("product_id"); productID != "" {
		query = query.Where("product_id = ?", productID)
	}

	// Filter by date range
	if startDate := c.Query("start_date"); startDate != "" {
		query = query.Where("transaction_date >= ?", startDate)
	}
	if endDate := c.Query("end_date"); endDate != "" {
		query = query.Where("transaction_date <= ?", endDate)
	}

	// Filter by type (IN/OUT)
	if movementType := c.Query("type"); movementType != "" {
		query = query.Where("type = ?", movementType)
	}

	if err := query.Order("transaction_date DESC").Find(&movements).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch inventory movements"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Inventory movements retrieved successfully",
		"data":    movements,
	})
}

// GetLowStockProducts retrieves products with stock below minimum level
func (ic *InventoryController) GetLowStockProducts(c *gin.Context) {
	var products []models.Product
	if err := ic.DB.Where("stock <= min_stock AND min_stock > 0 AND is_active = ? AND is_assembly = ?", true, false).Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch low stock products"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Low stock products retrieved successfully",
		"data":    products,
		"count":   len(products),
	})
}

// GetStockValuation calculates stock valuation using specified method
func (ic *InventoryController) GetStockValuation(c *gin.Context) {
	method := c.DefaultQuery("method", models.ValuationFIFO)
	productIDStr := c.Query("product_id")

	var result interface{}
	var err error

	if productIDStr != "" {
		productID, parseErr := strconv.ParseUint(productIDStr, 10, 32)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
			return
		}
		result, err = ic.calculateProductValuation(uint(productID), method)
	} else {
		result, err = ic.calculateTotalValuation(method)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Stock valuation calculated successfully",
		"method":  method,
		"data":    result,
	})
}

// BulkPriceUpdate updates prices for multiple products
func (ic *InventoryController) BulkPriceUpdate(c *gin.Context) {
	var input struct {
		Updates []struct {
			ProductID     uint     `json:"product_id" binding:"required"`
			PurchasePrice *float64 `json:"purchase_price,omitempty"`
			SalePrice     *float64 `json:"sale_price,omitempty"`
		} `json:"updates" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx := ic.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	updatedCount := 0
	for _, update := range input.Updates {
		var product models.Product
		if err := tx.First(&product, update.ProductID).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found", "product_id": update.ProductID})
			return
		}

		updateData := make(map[string]interface{})
		if update.PurchasePrice != nil {
			updateData["purchase_price"] = *update.PurchasePrice
		}
		if update.SalePrice != nil {
			updateData["sale_price"] = *update.SalePrice
		}

		if len(updateData) > 0 {
			if err := tx.Model(&product).Updates(updateData).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product prices"})
				return
			}
			updatedCount++
		}
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit price updates"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Bulk price update completed successfully",
		"updated_count": updatedCount,
	})
}

// GetStockReport generates comprehensive stock report
func (ic *InventoryController) GetStockReport(c *gin.Context) {
	var products []models.Product
	var totalValue float64

	query := ic.DB.Where("is_active = ?", true)

	// Filter by category if provided
	if categoryID := c.Query("category_id"); categoryID != "" {
		query = query.Where("category_id = ?", categoryID)
	}

	if err := query.Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products for report"})
		return
	}

	var reportData []map[string]interface{}
	for _, product := range products {
		value := float64(product.Stock) * product.PurchasePrice
		totalValue += value

		reportData = append(reportData, map[string]interface{}{
			"product_id":    product.ID,
			"code":          product.Code,
			"name":          product.Name,
			"category":      product.Category,
			"current_stock": product.Stock,
			"min_stock":     product.MinStock,
			"unit_cost":     product.PurchasePrice,
			"total_value":   value,
			"status":        ic.getStockStatus(product),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Stock report generated successfully",
		"data":        reportData,
		"total_value": totalValue,
		"total_items": len(products),
	})
}

// Helper methods

func (ic *InventoryController) calculateProductValuation(productID uint, method string) (interface{}, error) {
	var movements []models.Inventory
	if err := ic.DB.Where("product_id = ? AND type = ?", productID, models.InventoryTypeIn).
		Order("transaction_date ASC").Find(&movements).Error; err != nil {
		return nil, err
	}

	var product models.Product
	if err := ic.DB.First(&product, productID).Error; err != nil {
		return nil, err
	}

	switch method {
	case models.ValuationFIFO:
		return ic.calculateFIFO(movements, product.Stock)
	case models.ValuationLIFO:
		return ic.calculateLIFO(movements, product.Stock)
	case models.ValuationAverage:
		return ic.calculateAverage(movements, product.Stock)
	default:
		return ic.calculateFIFO(movements, product.Stock)
	}
}

func (ic *InventoryController) calculateTotalValuation(method string) (interface{}, error) {
	var products []models.Product
	if err := ic.DB.Where("is_active = ?", true).Find(&products).Error; err != nil {
		return nil, err
	}

	totalValue := 0.0
	var details []map[string]interface{}

	for _, product := range products {
		valuation, err := ic.calculateProductValuation(product.ID, method)
		if err != nil {
			continue
		}

		if val, ok := valuation.(map[string]interface{}); ok {
			if totalVal, exists := val["total_value"].(float64); exists {
				totalValue += totalVal
				details = append(details, map[string]interface{}{
					"product_id":   product.ID,
					"product_name": product.Name,
					"stock":        product.Stock,
					"valuation":    val,
				})
			}
		}
	}

	return map[string]interface{}{
		"total_value": totalValue,
		"details":     details,
		"method":      method,
	}, nil
}

func (ic *InventoryController) calculateFIFO(movements []models.Inventory, currentStock int) (interface{}, error) {
	if len(movements) == 0 || currentStock <= 0 {
		return map[string]interface{}{
			"total_value": 0.0,
			"method":      "FIFO",
		}, nil
	}

	totalValue := 0.0
	remainingStock := currentStock

	// Start from the earliest movements
	for _, movement := range movements {
		if remainingStock <= 0 {
			break
		}

		qtyToUse := movement.Quantity
		if qtyToUse > remainingStock {
			qtyToUse = remainingStock
		}

		totalValue += float64(qtyToUse) * movement.UnitCost
		remainingStock -= qtyToUse
	}

	return map[string]interface{}{
		"total_value": totalValue,
		"method":      "FIFO",
	}, nil
}

func (ic *InventoryController) calculateLIFO(movements []models.Inventory, currentStock int) (interface{}, error) {
	if len(movements) == 0 || currentStock <= 0 {
		return map[string]interface{}{
			"total_value": 0.0,
			"method":      "LIFO",
		}, nil
	}

	totalValue := 0.0
	remainingStock := currentStock

	// Start from the latest movements (reverse order)
	for i := len(movements) - 1; i >= 0; i-- {
		if remainingStock <= 0 {
			break
		}

		movement := movements[i]
		qtyToUse := movement.Quantity
		if qtyToUse > remainingStock {
			qtyToUse = remainingStock
		}

		totalValue += float64(qtyToUse) * movement.UnitCost
		remainingStock -= qtyToUse
	}

	return map[string]interface{}{
		"total_value": totalValue,
		"method":      "LIFO",
	}, nil
}

func (ic *InventoryController) calculateAverage(movements []models.Inventory, currentStock int) (interface{}, error) {
	if len(movements) == 0 || currentStock <= 0 {
		return map[string]interface{}{
			"total_value": 0.0,
			"method":      "Average",
		}, nil
	}

	totalCost := 0.0
	totalQuantity := 0

	for _, movement := range movements {
		totalCost += movement.TotalCost
		totalQuantity += movement.Quantity
	}

	if totalQuantity == 0 {
		return map[string]interface{}{
			"total_value": 0.0,
			"method":      "Average",
		}, nil
	}

	averageCost := totalCost / float64(totalQuantity)
	totalValue := averageCost * float64(currentStock)

	return map[string]interface{}{
		"total_value":  totalValue,
		"average_cost": averageCost,
		"method":       "Average",
	}, nil
}

func (ic *InventoryController) getStockStatus(product models.Product) string {
	if product.Stock <= 0 {
		return "out_of_stock"
	} else if product.Stock <= product.MinStock {
		return "low_stock"
	} else if product.Stock >= product.MaxStock && product.MaxStock > 0 {
		return "overstock"
	}
	return "normal"
}
