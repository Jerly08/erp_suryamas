package controllers

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ProductController struct {
	DB                     *gorm.DB
	stockMonitoringService *services.StockMonitoringService
}

func NewProductController(db *gorm.DB, stockMonitoringService *services.StockMonitoringService) *ProductController {
	return &ProductController{
		DB:                     db,
		stockMonitoringService: stockMonitoringService,
	}
}

func (pc *ProductController) GetProducts(c *gin.Context) {
	var products []models.Product

	query := pc.DB.Where("is_active = ?", true).Preload("Category").Preload("WarehouseLocation")

	// Add search functionality
	if search := c.Query("search"); search != "" {
		query = query.Where("name ILIKE ? OR code ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// Add category filter
	if categoryID := c.Query("category"); categoryID != "" {
		query = query.Where("category_id = ?", categoryID)
	}

	// Add is_assembly filter for Tab separation
	if isAssemblyStr := c.Query("is_assembly"); isAssemblyStr != "" {
		isAssembly, _ := strconv.ParseBool(isAssemblyStr)
		query = query.Where("is_assembly = ?", isAssembly)
	}

	// Count total records before pagination
	var total int64
	if err := query.Model(&models.Product{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count products"})
		return
	}

	// Pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit < 1 {
		limit = 50
	}

	offset := (page - 1) * limit

	// Fetch data with pagination
	if err := query.Limit(limit).Offset(offset).Order("name ASC").Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
		return
	}

	// Calculate total pages
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	c.JSON(http.StatusOK, gin.H{
		"message": "Products retrieved successfully",
		"data":    products,
		"meta": gin.H{
			"current_page": page,
			"per_page":     limit,
			"total":        total,
			"total_pages":  totalPages,
		},
	})
}

func (pc *ProductController) GetProduct(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	var product models.Product
	if err := pc.DB.Preload("Category").Preload("WarehouseLocation").First(&product, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Product retrieved successfully",
		"data":    product,
	})
}

// AdjustStock handles stock adjustments for products
func (pc *ProductController) AdjustStock(c *gin.Context) {
	var input struct {
		ProductID uint   `json:"product_id" binding:"required"`
		Quantity  int    `json:"quantity" binding:"required"`
		Type      string `json:"type" binding:"required,oneof=IN OUT"`
		Notes     string `json:"notes"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	product := models.Product{}
	if err := pc.DB.First(&product, input.ProductID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	inventory := models.Inventory{
		ProductID:       input.ProductID,
		Type:            input.Type,
		Quantity:        input.Quantity,
		TransactionDate: time.Now(),
		Notes:           input.Notes,
	}

	if err := pc.DB.Save(&inventory).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save inventory adjustment"})
		return
	}

	if input.Type == models.InventoryTypeIn {
		product.Stock += input.Quantity
	} else if input.Type == models.InventoryTypeOut {
		product.Stock -= input.Quantity
	}

	if err := pc.DB.Save(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product stock"})
		return
	}

	// Check stock levels after adjustment and trigger notifications if needed
	if pc.stockMonitoringService != nil {
		if err := pc.stockMonitoringService.CheckSingleProductStock(input.ProductID); err != nil {
			log.Printf("Failed to check stock levels for product %d: %v", input.ProductID, err)
		}
		// Also resolve any existing alerts if stock is now above threshold
		if err := pc.stockMonitoringService.ResolveStockAlerts(); err != nil {
			log.Printf("Failed to resolve stock alerts: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Stock adjusted successfully", "product": product})
}

// Opname processes stock opname
func (pc *ProductController) Opname(c *gin.Context) {
	var input struct {
		ProductID uint   `json:"product_id" binding:"required"`
		NewStock  int    `json:"new_stock" binding:"required"`
		Notes     string `json:"notes"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	product := models.Product{}
	if err := pc.DB.First(&product, input.ProductID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	inventory := models.Inventory{
		ProductID:       input.ProductID,
		Type:            models.InventoryTypeIn,
		Quantity:        input.NewStock - product.Stock,
		TransactionDate: time.Now(),
		Notes:           input.Notes,
	}

	if err := pc.DB.Save(&inventory).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save inventory opname"})
		return
	}

	product.Stock = input.NewStock

	if err := pc.DB.Save(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product stock"})
		return
	}

	// Check stock levels after opname and trigger notifications if needed
	if pc.stockMonitoringService != nil {
		if err := pc.stockMonitoringService.CheckSingleProductStock(input.ProductID); err != nil {
			log.Printf("Failed to check stock levels for product %d: %v", input.ProductID, err)
		}
		// Also resolve any existing alerts if stock is now above threshold
		if err := pc.stockMonitoringService.ResolveStockAlerts(); err != nil {
			log.Printf("Failed to resolve stock alerts: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Stock opname processed successfully", "product": product})
}

func (pc *ProductController) CreateProduct(c *gin.Context) {
	var product models.Product
	if err := c.ShouldBindJSON(&product); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// ✅ VALIDATE PRICES - Prevent unreasonable values
	// Maximum reasonable price: 10 Billion (10,000,000,000)
	const maxReasonablePrice = 10000000000.0

	// ✅ AUTO-SET: If cost_price is 0, default to purchase_price
	if product.CostPrice == 0 && product.PurchasePrice > 0 {
		product.CostPrice = product.PurchasePrice
		log.Printf("ℹ️ Auto-set cost_price = purchase_price (%.2f) for product '%s'", product.CostPrice, product.Name)
	}

	if product.CostPrice < 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Cost price cannot be negative",
			"field": "cost_price",
		})
		return
	}

	if product.CostPrice > maxReasonablePrice {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Cost price is unreasonably high",
			"message": fmt.Sprintf("Cost price (Rp %.0f) exceeds maximum allowed (Rp %.0f / 10 Miliar)",
				product.CostPrice, maxReasonablePrice),
			"field": "cost_price",
			"value": product.CostPrice,
			"max":   maxReasonablePrice,
		})
		return
	}

	if product.PurchasePrice < 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Purchase price cannot be negative",
			"field": "purchase_price",
		})
		return
	}

	if product.PurchasePrice > maxReasonablePrice {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Purchase price is unreasonably high",
			"message": fmt.Sprintf("Purchase price (Rp %.0f) exceeds maximum allowed (Rp %.0f / 10 Miliar)",
				product.PurchasePrice, maxReasonablePrice),
			"field": "purchase_price",
		})
		return
	}

	if product.SalePrice < 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Sale price cannot be negative",
			"field": "sale_price",
		})
		return
	}

	if product.SalePrice > maxReasonablePrice {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Sale price is unreasonably high",
			"message": fmt.Sprintf("Sale price (Rp %.0f) exceeds maximum allowed (Rp %.0f / 10 Miliar)",
				product.SalePrice, maxReasonablePrice),
			"field": "sale_price",
		})
		return
	}

	log.Printf("✅ Price validation passed for product '%s': CostPrice=%.2f, PurchasePrice=%.2f, SalePrice=%.2f",
		product.Name, product.CostPrice, product.PurchasePrice, product.SalePrice)

	// Check if product code already exists
	var existingProduct models.Product
	if err := pc.DB.Where("code = ?", product.Code).First(&existingProduct).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Product code already exists"})
		return
	}

	if err := pc.DB.Create(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product"})
		return
	}

	// Load the relations
	if err := pc.DB.Preload("Category").Preload("WarehouseLocation").First(&product, product.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load product relations"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Product created successfully",
		"data":    product,
	})
}

func (pc *ProductController) UpdateProduct(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	var product models.Product
	if err := pc.DB.First(&product, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	var updateData models.Product
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// ✅ VALIDATE PRICES - Prevent unreasonable values
	// Maximum reasonable price: 10 Billion (10,000,000,000)
	const maxReasonablePrice = 10000000000.0

	// ✅ AUTO-SET: If cost_price is 0, default to purchase_price
	if updateData.CostPrice == 0 && updateData.PurchasePrice > 0 {
		updateData.CostPrice = updateData.PurchasePrice
		log.Printf("ℹ️ Auto-set cost_price = purchase_price (%.2f) for product update ID=%d", updateData.CostPrice, id)
	}

	if updateData.CostPrice < 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Cost price cannot be negative",
			"field": "cost_price",
		})
		return
	}

	if updateData.CostPrice > maxReasonablePrice {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Cost price is unreasonably high",
			"message": fmt.Sprintf("Cost price (Rp %.0f) exceeds maximum allowed (Rp %.0f / 10 Miliar)",
				updateData.CostPrice, maxReasonablePrice),
			"field": "cost_price",
			"value": updateData.CostPrice,
			"max":   maxReasonablePrice,
		})
		return
	}

	if updateData.PurchasePrice < 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Purchase price cannot be negative",
			"field": "purchase_price",
		})
		return
	}

	if updateData.PurchasePrice > maxReasonablePrice {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Purchase price is unreasonably high",
			"message": fmt.Sprintf("Purchase price (Rp %.0f) exceeds maximum allowed (Rp %.0f / 10 Miliar)",
				updateData.PurchasePrice, maxReasonablePrice),
			"field": "purchase_price",
		})
		return
	}

	if updateData.SalePrice < 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Sale price cannot be negative",
			"field": "sale_price",
		})
		return
	}

	if updateData.SalePrice > maxReasonablePrice {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Sale price is unreasonably high",
			"message": fmt.Sprintf("Sale price (Rp %.0f) exceeds maximum allowed (Rp %.0f / 10 Miliar)",
				updateData.SalePrice, maxReasonablePrice),
			"field": "sale_price",
		})
		return
	}

	log.Printf("✅ Price validation passed for product update ID=%d: CostPrice=%.2f, PurchasePrice=%.2f, SalePrice=%.2f",
		id, updateData.CostPrice, updateData.PurchasePrice, updateData.SalePrice)

	// Check if new code conflicts with existing products
	if updateData.Code != product.Code {
		var existingProduct models.Product
		if err := pc.DB.Where("code = ? AND id != ?", updateData.Code, id).First(&existingProduct).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Product code already exists"})
			return
		}
	}

	// Make sure service products don't have a warehouse assigned
	if updateData.IsService {
		updateData.WarehouseLocationID = nil
	}

	// FIXED: Use Select() with Updates() to ensure zero values are updated
	// This fixes the issue where stock=0 or prices=0 would not be updated
	// Only update specific fields to avoid foreign key constraint issues
	updateFields := []string{
		"code", "name", "description", "category_id", "warehouse_location_id",
		"brand", "model", "unit", "purchase_price", "cost_price", "sale_price",
		"pricing_tier", "stock", "min_stock", "max_stock", "reorder_level",
		"barcode", "sku", "weight", "dimensions", "is_active", "is_service",
		"taxable", "image_path", "notes", "default_expense_account_id",
	}
	if err := pc.DB.Model(&product).Select(updateFields).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product"})
		return
	}

	// Load the updated product with relations
	if err := pc.DB.Preload("Category").Preload("WarehouseLocation").First(&product, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load updated product relations"})
		return
	}

	// Check stock levels after update and trigger notifications if needed
	if pc.stockMonitoringService != nil {
		if err := pc.stockMonitoringService.CheckSingleProductStock(uint(id)); err != nil {
			// Log error but don't fail the update
			log.Printf("Failed to check stock levels for product %d: %v", id, err)
		}
		// Also resolve any existing alerts if stock is now above threshold
		if err := pc.stockMonitoringService.ResolveStockAlerts(); err != nil {
			log.Printf("Failed to resolve stock alerts: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Product updated successfully",
		"data":    product,
	})
}

func (pc *ProductController) DeleteProduct(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	var product models.Product
	if err := pc.DB.First(&product, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	// Soft delete
	if err := pc.DB.Delete(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete product"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Product deleted successfully",
	})
}

// UploadProductImage handles product image upload
func (pc *ProductController) UploadProductImage(c *gin.Context) {
	productID, err := strconv.Atoi(c.PostForm("product_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	// Check if product exists
	var product models.Product
	if err := pc.DB.First(&product, productID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	// Get uploaded file
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get uploaded file"})
		return
	}

	// Validate file type
	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/jpg":  true,
		"image/png":  true,
		"image/gif":  true,
	}

	if !allowedTypes[file.Header.Get("Content-Type")] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type. Only JPEG, PNG, and GIF are allowed"})
		return
	}

	// Validate file size (max 5MB)
	maxSize := int64(5 * 1024 * 1024) // 5MB
	if file.Size > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File size too large. Maximum 5MB allowed"})
		return
	}

	// Create uploads directory if it doesn't exist
	uploadPath := "./uploads/products/"
	if err := os.MkdirAll(uploadPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload directory"})
		return
	}

	// Generate unique filename
	filename := fmt.Sprintf("%d_%d_%s", productID, time.Now().Unix(), file.Filename)
	filePath := uploadPath + filename

	// Store old image path for cleanup if needed
	oldImagePath := product.ImagePath

	// Save the file
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	// Update product with image path
	relativeImagePath := "/uploads/products/" + filename
	if err := pc.DB.Model(&product).Update("image_path", relativeImagePath).Error; err != nil {
		// Database update failed, cleanup the uploaded file
		if removeErr := os.Remove(filePath); removeErr != nil {
			log.Printf("Failed to cleanup uploaded file after DB error: %v", removeErr)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product image path"})
		return
	}

	// Success: cleanup old image file if it exists
	if oldImagePath != "" {
		oldFilePath := "./uploads/products/" + filepath.Base(oldImagePath)
		if _, err := os.Stat(oldFilePath); err == nil {
			if removeErr := os.Remove(oldFilePath); removeErr != nil {
				log.Printf("Failed to cleanup old image file: %v", removeErr)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Image uploaded successfully",
		"filename": filename,
		"path":     relativeImagePath,
	})
}

// ImportProducts bulk imports products from JSON
func (pc *ProductController) ImportProducts(c *gin.Context) {
	var inputProducts []models.Product
	if err := c.ShouldBindJSON(&inputProducts); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid format, expected JSON array of products"})
		return
	}

	importedCount := 0
	updatedCount := 0

	// Use a transaction for bulk operations
	err := pc.DB.Transaction(func(tx *gorm.DB) error {
		for i, p := range inputProducts {
			// Ensure basic validation
			if p.Name == "" {
				continue
			}

			// Auto-set cost_price if missing
			if p.CostPrice == 0 && p.PurchasePrice > 0 {
				p.CostPrice = p.PurchasePrice
			}

			// Try to find if the product exists by code (if provided) or name
			var existing models.Product
			var query *gorm.DB
			if p.Code != "" {
				query = tx.Where("code = ? OR name = ?", p.Code, p.Name)
			} else {
				query = tx.Where("name = ?", p.Name)
			}

			err := query.First(&existing).Error

			if err == nil {
				// Product exists, update pricing and other provided fields
				updateFields := map[string]interface{}{}
				if p.PurchasePrice > 0 {
					updateFields["purchase_price"] = p.PurchasePrice
				}
				if p.CostPrice > 0 {
					updateFields["cost_price"] = p.CostPrice
				}
				if p.SalePrice > 0 {
					updateFields["sale_price"] = p.SalePrice
				}
				if p.Code != "" && existing.Code == "" {
					updateFields["code"] = p.Code
				}
				if p.Unit != "" {
					updateFields["unit"] = p.Unit
				}
				if p.CategoryID != nil && *p.CategoryID != 0 {
					updateFields["category_id"] = p.CategoryID
				}
				if p.Description != "" {
					updateFields["description"] = p.Description
				}

				if len(updateFields) > 0 {
					if err := tx.Model(&existing).Updates(updateFields).Error; err != nil {
						return err
					}
					updatedCount++
				}
			} else {
				// Product doesn't exist, create it
				if p.Code == "" {
					// Generate a simple unique code based on time and loop index to prevent duplicates in fast loop
					p.Code = fmt.Sprintf("PRD-%d-%d", time.Now().Unix(), i)
				}
				if p.Unit == "" {
					p.Unit = "Pcs" // default unit
				}
				p.IsActive = true
				if err := tx.Create(&p).Error; err != nil {
					return fmt.Errorf("failed to create product %s: %v", p.Name, err)
				}
				importedCount++
			}
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to import products: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Import completed successfully",
		"imported": importedCount,
		"updated":  updatedCount,
	})
}
