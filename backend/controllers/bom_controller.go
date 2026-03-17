package controllers

import (
	"app-sistem-akuntansi/models"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BOMController struct {
	DB *gorm.DB
}

var errBOMAssemblyComponent = errors.New("assembly product cannot be used as bom component")

func NewBOMController(db *gorm.DB) *BOMController {
	return &BOMController{DB: db}
}

func (bc *BOMController) findBOMByTargetProduct(targetProductID uint, excludeBOMID ...uint) (*models.BOM, error) {
	var bom models.BOM
	query := bc.DB.Where("target_product_id = ?", targetProductID)

	if len(excludeBOMID) > 0 && excludeBOMID[0] != 0 {
		query = query.Where("id <> ?", excludeBOMID[0])
	}

	if err := query.First(&bom).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &bom, nil
}

// GetBOMs gets a list of Master BOMs
func (bc *BOMController) GetBOMs(c *gin.Context) {
	var boms []models.BOM

	query := bc.DB.Preload("TargetProduct").Preload("Items.ComponentProduct")

	if targetProductID := c.Query("target_product_id"); targetProductID != "" {
		query = query.Where("target_product_id = ?", targetProductID)
	}

	// Preload the target product to show in listings
	if err := query.Find(&boms).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch BOMs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": boms})
}

// GetBOM gets a single BOM with full items and component details
func (bc *BOMController) GetBOM(c *gin.Context) {
	id := c.Param("id")
	var bom models.BOM

	if err := bc.DB.Preload("TargetProduct").Preload("Items.ComponentProduct").First(&bom, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "BOM not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": bom})
}

// CreateBOM creates a new Master BOM Template
func (bc *BOMController) CreateBOM(c *gin.Context) {
	var payload models.BOMCreateRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate Target Product exists
	var targetProduct models.Product
	if err := bc.DB.First(&targetProduct, payload.TargetProductID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Target product not found"})
		return
	}

	if !targetProduct.IsAssembly {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Target product must be a target assembly product"})
		return
	}

	existingBOMForTarget, err := bc.findBOMByTargetProduct(payload.TargetProductID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate target product BOM linkage"})
		return
	}
	if existingBOMForTarget != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Target assembly product is already linked to another BOM template",
		})
		return
	}

	err = bc.DB.Transaction(func(tx *gorm.DB) error {
		// Calculate total cost from items
		var totalCost float64 = 0
		var items []models.BOMItem

		for _, itemPayload := range payload.Items {
			// Validate component product exists
			var componentProduct models.Product
			if err := tx.First(&componentProduct, itemPayload.ComponentProductID).Error; err != nil {
				return err
			}

			if componentProduct.IsAssembly {
				return errBOMAssemblyComponent
			}

			// Always use DB price — never trust frontend-supplied unit_price
			// This prevents price manipulation via raw API requests
			actualUnitPrice := componentProduct.PurchasePrice
			itemTotalCost := itemPayload.RequirementQuantity * actualUnitPrice
			totalCost += itemTotalCost

			items = append(items, models.BOMItem{
				ComponentProductID:  itemPayload.ComponentProductID,
				CategoryStr:         itemPayload.CategoryStr,
				BaseQuantity:        float64(componentProduct.Stock), // snapshot stok saat BOM dibuat
				BaseUnit:            itemPayload.BaseUnit,
				Notes:               itemPayload.Notes,
				UnitPrice:           actualUnitPrice, // dari DB, bukan dari frontend
				RequirementQuantity: itemPayload.RequirementQuantity,
				TotalCost:           itemTotalCost,
			})
		}

		// Calculate percentages
		for i := range items {
			if totalCost > 0 {
				items[i].CostPercentage = (items[i].TotalCost / totalCost) * 100
			}
		}

		// Create BOM
		// TotalCost = sum(RequirementQty * UnitPrice) = cost to produce 1 unit.
		// TargetQuantity is the batch size (how many units per production run).
		// It does NOT affect unit cost — it is used downstream for procurement scaling.
		bom := models.BOM{
			Code:            payload.Code,
			TargetProductID: payload.TargetProductID,
			TargetQuantity:  payload.TargetQuantity,
			Name:            payload.Name,
			Description:     payload.Description,
			TotalCost:       totalCost, // cost per 1 output unit
			Items:           items,
		}

		if err := tx.Create(&bom).Error; err != nil {
			return err
		}

		// Update target product's cost price to match BOM unit cost
		if err := tx.Model(&targetProduct).Updates(map[string]interface{}{
			"cost_price":     totalCost,
			"purchase_price": totalCost,
		}).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		if errors.Is(err, errBOMAssemblyComponent) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Assembly products cannot be used as BOM components"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create BOM: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "BOM created successfully"})
}

// UpdateBOM updates an existing Master BOM Template
func (bc *BOMController) UpdateBOM(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid BOM ID"})
		return
	}

	var payload models.BOMCreateRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check BOM exists
	var existingBOM models.BOM
	if err := bc.DB.First(&existingBOM, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "BOM not found"})
		return
	}

	// Validate Target Product exists
	var targetProduct models.Product
	if err := bc.DB.First(&targetProduct, payload.TargetProductID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Target product not found"})
		return
	}

	if !targetProduct.IsAssembly {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Target product must be a target assembly product"})
		return
	}

	existingBOMForTarget, err := bc.findBOMByTargetProduct(payload.TargetProductID, uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate target product BOM linkage"})
		return
	}
	if existingBOMForTarget != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Target assembly product is already linked to another BOM template",
		})
		return
	}

	err = bc.DB.Transaction(func(tx *gorm.DB) error {
		// Delete old items first
		if err := tx.Where("bom_id = ?", id).Delete(&models.BOMItem{}).Error; err != nil {
			return err
		}

		// Calculate total cost from items
		var totalCost float64 = 0
		var items []models.BOMItem

		for _, itemPayload := range payload.Items {
			// Validate component product exists
			var componentProduct models.Product
			if err := tx.First(&componentProduct, itemPayload.ComponentProductID).Error; err != nil {
				return err
			}

			if componentProduct.IsAssembly {
				return errBOMAssemblyComponent
			}

			// Always use DB price — never trust frontend-supplied unit_price
			actualUnitPrice := componentProduct.PurchasePrice
			itemTotalCost := itemPayload.RequirementQuantity * actualUnitPrice
			totalCost += itemTotalCost

			items = append(items, models.BOMItem{
				BOMID:               uint(id),
				ComponentProductID:  itemPayload.ComponentProductID,
				CategoryStr:         itemPayload.CategoryStr,
				BaseQuantity:        float64(componentProduct.Stock),
				BaseUnit:            itemPayload.BaseUnit,
				Notes:               itemPayload.Notes,
				UnitPrice:           actualUnitPrice,
				RequirementQuantity: itemPayload.RequirementQuantity,
				TotalCost:           itemTotalCost,
			})
		}

		// Calculate percentages
		for i := range items {
			if totalCost > 0 {
				items[i].CostPercentage = (items[i].TotalCost / totalCost) * 100
			}
		}

		// Create new items
		if len(items) > 0 {
			if err := tx.Create(&items).Error; err != nil {
				return err
			}
		}

		// Update BOM header
		if err := tx.Model(&existingBOM).Updates(map[string]interface{}{
			"code":              payload.Code,
			"target_product_id": payload.TargetProductID,
			"target_quantity":   payload.TargetQuantity,
			"name":              payload.Name,
			"description":       payload.Description,
			"total_cost":        totalCost,
		}).Error; err != nil {
			return err
		}

		// Update target product's cost price to match BOM unit cost
		if err := tx.Model(&targetProduct).Updates(map[string]interface{}{
			"cost_price":     totalCost,
			"purchase_price": totalCost,
		}).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		if errors.Is(err, errBOMAssemblyComponent) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Assembly products cannot be used as BOM components"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update BOM: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "BOM updated successfully"})
}

// DeleteBOM deletes a BOM
func (bc *BOMController) DeleteBOM(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid BOM ID"})
		return
	}

	// Because of OnDelete:CASCADE in models, items will be deleted automatically if DB supports it.
	// But in GORM, soft deletes require a manual cascading approach.
	err = bc.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("bom_id = ?", id).Delete(&models.BOMItem{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&models.BOM{}, id).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete BOM"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "BOM deleted successfully"})
}
