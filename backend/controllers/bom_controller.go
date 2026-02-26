package controllers

import (
	"app-sistem-akuntansi/models"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BOMController struct {
	DB *gorm.DB
}

func NewBOMController(db *gorm.DB) *BOMController {
	return &BOMController{DB: db}
}

// GetBOMs gets a list of Master BOMs
func (bc *BOMController) GetBOMs(c *gin.Context) {
	var boms []models.BOM

	// Preload the target product to show in listings
	if err := bc.DB.Preload("TargetProduct").Preload("Items").Find(&boms).Error; err != nil {
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

	err := bc.DB.Transaction(func(tx *gorm.DB) error {
		// Calculate total cost from items
		var totalCost float64 = 0
		var items []models.BOMItem

		for _, itemPayload := range payload.Items {
			// Validate component product exists
			var componentProduct models.Product
			if err := tx.First(&componentProduct, itemPayload.ComponentProductID).Error; err != nil {
				return err
			}

			// Calculation
			itemTotalCost := itemPayload.RequirementQuantity * itemPayload.UnitPrice
			totalCost += itemTotalCost

			items = append(items, models.BOMItem{
				ComponentProductID:  itemPayload.ComponentProductID,
				CategoryStr:         itemPayload.CategoryStr,
				BaseQuantity:        itemPayload.BaseQuantity,
				BaseUnit:            itemPayload.BaseUnit,
				Notes:               itemPayload.Notes,
				UnitPrice:           itemPayload.UnitPrice,
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
		bom := models.BOM{
			Code:            payload.Code,
			TargetProductID: payload.TargetProductID,
			TargetQuantity:  payload.TargetQuantity,
			Name:            payload.Name,
			Description:     payload.Description,
			TotalCost:       totalCost,
			Items:           items,
		}

		if err := tx.Create(&bom).Error; err != nil {
			return err
		}

		// Update target product's cost price
		if err := tx.Model(&targetProduct).Updates(map[string]interface{}{
			"cost_price":     totalCost,
			"purchase_price": totalCost,
		}).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create BOM: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "BOM created successfully"})
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
