package controllers

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/services"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AssetController struct {
	assetService    services.AssetServiceInterface
	db              *gorm.DB
	assetJournalSvc *services.AssetJournalServiceSSOT
}

type AssetCreateRequest struct {
	Code                             string    `json:"code"`
	Name                             string    `json:"name" binding:"required"`
	CategoryID                       *uint     `json:"category_id"`
	Category                         string    `json:"category" binding:"required"`
	Status                           string    `json:"status"`
	PurchaseDate                     time.Time `json:"purchase_date" binding:"required"`
	PurchasePrice                    float64   `json:"purchase_price" binding:"required,gt=0"`
	SalvageValue                     float64   `json:"salvage_value"`
	UsefulLife                       int       `json:"useful_life" binding:"gte=0"`
	DepreciationMethod               string    `json:"depreciation_method"`
	IsActive                         bool      `json:"is_active"`
	Notes                            string    `json:"notes"`
	Location                         string    `json:"location"`
	Coordinates                      string    `json:"coordinates"`
	SerialNumber                     string    `json:"serial_number"`
	Condition                        string    `json:"condition"`
	AssetAccountID                   *uint     `json:"asset_account_id"`
	DepreciationAccountID            *uint     `json:"depreciation_account_id"`
	AccumulatedDepreciationAccountID *uint     `json:"accumulated_depreciation_account_id"`
	OpeningAccumulatedDepreciation   float64   `json:"opening_accumulated_depreciation"`
	PaymentMethod                    string    `json:"payment_method"`     // CASH, BANK, CREDIT
	PaymentAccountID                 *uint     `json:"payment_account_id"` // Specific CASH/BANK account
	CreditAccountID                  *uint     `json:"credit_account_id"`  // Specific LIABILITY account
	UserID                           uint      `json:"user_id" binding:"required"`
}

type AssetUpdateRequest struct {
	Name                             string    `json:"name" binding:"required"`
	CategoryID                       *uint     `json:"category_id"`
	Category                         string    `json:"category" binding:"required"`
	Status                           string    `json:"status"`
	PurchaseDate                     time.Time `json:"purchase_date" binding:"required"`
	PurchasePrice                    float64   `json:"purchase_price" binding:"required,gt=0"`
	SalvageValue                     float64   `json:"salvage_value"`
	UsefulLife                       int       `json:"useful_life" binding:"gte=0"`
	DepreciationMethod               string    `json:"depreciation_method"`
	IsActive                         bool      `json:"is_active"`
	Notes                            string    `json:"notes"`
	Location                         string    `json:"location"`
	Coordinates                      string    `json:"coordinates"`
	SerialNumber                     string    `json:"serial_number"`
	Condition                        string    `json:"condition"`
	AssetAccountID                   *uint     `json:"asset_account_id"`
	DepreciationAccountID            *uint     `json:"depreciation_account_id"`
	AccumulatedDepreciationAccountID *uint     `json:"accumulated_depreciation_account_id"`
}

func NewAssetController(db *gorm.DB) *AssetController {
	assetRepo := repositories.NewAssetRepository(db)
	assetService := services.NewAssetService(assetRepo, db)
	assetJournalSvc := services.NewAssetJournalServiceSSOT(db)

	return &AssetController{
		assetService:    assetService,
		db:              db,
		assetJournalSvc: assetJournalSvc,
	}
}

func assetFloatChanged(current, next float64) bool {
	return math.Abs(current-next) > 0.0001
}

func sameAssetDate(current, next time.Time) bool {
	return current.Format("2006-01-02") == next.Format("2006-01-02")
}

func sameUintPointer(current, next *uint) bool {
	if current == nil && next == nil {
		return true
	}
	if current == nil || next == nil {
		return false
	}
	return *current == *next
}

func isPurchaseLinkedAsset(asset *models.Asset) bool {
	return strings.Contains(asset.Notes, "Auto-created from Purchase")
}

func (ac *AssetController) hasPostedAssetJournal(assetID uint, sourceTypes ...string) (bool, error) {
	var count int64
	err := ac.db.Model(&models.SSOTJournalEntry{}).
		Where("source_id = ? AND status = ? AND deleted_at IS NULL", assetID, "POSTED").
		Where("source_type IN ?", sourceTypes).
		Count(&count).Error
	return count > 0, err
}

// GetAssets retrieves all assets with optional filtering
func (ac *AssetController) GetAssets(c *gin.Context) {
	assets, err := ac.assetService.GetAllAssets()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve assets",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Assets retrieved successfully",
		"data":    assets,
		"count":   len(assets),
	})
}

// GetAsset retrieves a specific asset by ID
func (ac *AssetController) GetAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid asset ID"})
		return
	}

	asset, err := ac.assetService.GetAssetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Asset not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Asset retrieved successfully",
		"data":    asset,
	})
}

// CreateAsset creates a new asset
func (ac *AssetController) CreateAsset(c *gin.Context) {
	var req AssetCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	// Convert request to asset model
	asset := &models.Asset{
		Code:                             req.Code,
		Name:                             req.Name,
		CategoryID:                       req.CategoryID,
		Category:                         req.Category,
		Status:                           req.Status,
		PurchaseDate:                     req.PurchaseDate,
		PurchasePrice:                    req.PurchasePrice,
		SalvageValue:                     req.SalvageValue,
		UsefulLife:                       req.UsefulLife,
		DepreciationMethod:               req.DepreciationMethod,
		IsActive:                         req.IsActive,
		Notes:                            req.Notes,
		Location:                         req.Location,
		Coordinates:                      req.Coordinates,
		MapsURL:                          generateMapsURL(req.Coordinates),
		SerialNumber:                     req.SerialNumber,
		Condition:                        req.Condition,
		AssetAccountID:                   req.AssetAccountID,
		DepreciationAccountID:            req.DepreciationAccountID,
		AccumulatedDepreciationAccountID: req.AccumulatedDepreciationAccountID,
		OpeningAccumulatedDepreciation:   req.OpeningAccumulatedDepreciation,
		AccumulatedDepreciation:          req.OpeningAccumulatedDepreciation,
	}

	// Set defaults
	if asset.Status == "" {
		asset.Status = models.AssetStatusActive
	}
	switch asset.Status {
	case models.AssetStatusActive:
		asset.IsActive = true
	default:
		asset.IsActive = false
	}

	// Default payment method if not provided
	paymentMethod := req.PaymentMethod
	if paymentMethod == "" {
		paymentMethod = "CREDIT" // Default to credit purchase
	}

	// Atomically create asset AND acquisition journal in one transaction
	if err := ac.assetJournalSvc.CreateAssetWithAcquisitionJournal(
		asset, req.UserID, paymentMethod, req.PaymentAccountID, req.CreditAccountID, ac.assetService,
	); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to create asset",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Asset created successfully",
		"data":    asset,
	})
}

// CapitalizeAsset creates capitalization journal entries for a given asset
func (ac *AssetController) CapitalizeAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid asset ID"})
		return
	}
	var req struct {
		Amount              float64    `json:"amount" binding:"required,gt=0"`
		Date                *time.Time `json:"date"`
		Description         string     `json:"description"`
		Reference           string     `json:"reference"`
		SourceAccountID     *uint      `json:"source_account_id"`
		FixedAssetAccountID *uint      `json:"fixed_asset_account_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data", "details": err.Error()})
		return
	}
	// Use SSOT-based capitalization via AssetJournalServiceSSOT
	date := time.Now()
	if req.Date != nil {
		date = *req.Date
	}

	// Resolve source account (default to 1103-001 Persediaan Barang Dagang)
	var sourceID uint
	if req.SourceAccountID != nil {
		sourceID = *req.SourceAccountID
	} else {
		var acc models.Account
		for _, code := range []string{"1103-001", "1103", "1301"} {
			if err := ac.db.Where("code = ? AND deleted_at IS NULL", code).First(&acc).Error; err == nil {
				sourceID = acc.ID
				break
			}
		}
	}

	// Resolve fixed asset account (default to 1201-004 Peralatan)
	var faID uint
	if req.FixedAssetAccountID != nil {
		faID = *req.FixedAssetAccountID
	} else {
		var acc models.Account
		if err := ac.db.Where("code = ? AND deleted_at IS NULL", "1201-004").First(&acc).Error; err == nil {
			faID = acc.ID
		}
	}

	if sourceID == 0 || faID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unable to resolve accounts. Provide source_account_id and fixed_asset_account_id explicitly."})
		return
	}

	userID := c.MustGet("user_id").(uint)
	input := services.CapitalizationInput{
		AssetID:             uint(id),
		Amount:              req.Amount,
		Date:                date,
		Description:         req.Description,
		Reference:           req.Reference,
		SourceAccountID:     sourceID,
		FixedAssetAccountID: faID,
		UserID:              userID,
		ReferenceType:       models.JournalRefAsset,
		ReferenceID:         uint(id),
	}

	// Use SSOT service for capitalization journal
	if err := ac.assetJournalSvc.CreateCapitalizationJournal(input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to capitalize asset", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Asset capitalized successfully via SSOT"})
}

// UpdateAsset updates an existing asset
func (ac *AssetController) UpdateAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid asset ID"})
		return
	}

	var req AssetUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	// Get existing asset
	existingAsset, err := ac.assetService.GetAssetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Asset not found",
			"details": err.Error(),
		})
		return
	}

	acquisitionFieldsChanged := assetFloatChanged(existingAsset.PurchasePrice, req.PurchasePrice) ||
		existingAsset.Category != req.Category ||
		!sameAssetDate(existingAsset.PurchaseDate, req.PurchaseDate) ||
		(req.AssetAccountID != nil && !sameUintPointer(existingAsset.AssetAccountID, req.AssetAccountID))

	depreciationFieldsChanged := assetFloatChanged(existingAsset.SalvageValue, req.SalvageValue) ||
		existingAsset.UsefulLife != req.UsefulLife ||
		existingAsset.DepreciationMethod != req.DepreciationMethod ||
		!sameAssetDate(existingAsset.PurchaseDate, req.PurchaseDate)

	if acquisitionFieldsChanged {
		hasAssetJournal, journalErr := ac.hasPostedAssetJournal(uint(id), models.SSOTSourceTypeAsset, models.SSOTSourceTypeOpening)
		if journalErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate asset journals", "details": journalErr.Error()})
			return
		}
		if hasAssetJournal || isPurchaseLinkedAsset(existingAsset) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Nilai perolehan, tanggal perolehan, kategori, atau akun aset tidak bisa diubah karena jurnal perolehan sudah ada. Gunakan jurnal adjustment jika perlu koreksi.",
			})
			return
		}
	}

	if depreciationFieldsChanged {
		hasDepreciationJournal, journalErr := ac.hasPostedAssetJournal(uint(id), models.SSOTSourceTypeDepreciation)
		if journalErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate depreciation journals", "details": journalErr.Error()})
			return
		}
		if hasDepreciationJournal {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Masa manfaat, nilai sisa, metode penyusutan, atau tanggal dasar penyusutan tidak bisa diubah setelah jurnal penyusutan terposting.",
			})
			return
		}
	}

	// Update fields
	existingAsset.Name = req.Name
	existingAsset.CategoryID = req.CategoryID
	existingAsset.Category = req.Category
	existingAsset.Status = req.Status
	existingAsset.PurchaseDate = req.PurchaseDate
	existingAsset.PurchasePrice = req.PurchasePrice
	existingAsset.SalvageValue = req.SalvageValue
	existingAsset.UsefulLife = req.UsefulLife
	existingAsset.DepreciationMethod = req.DepreciationMethod
	existingAsset.IsActive = req.IsActive
	existingAsset.Notes = req.Notes
	existingAsset.Location = req.Location
	existingAsset.Coordinates = req.Coordinates
	existingAsset.MapsURL = generateMapsURL(req.Coordinates)
	existingAsset.SerialNumber = req.SerialNumber
	existingAsset.Condition = req.Condition

	// Only overwrite account IDs if explicitly provided in the request.
	// When the frontend omits these fields, Go decodes them as nil pointers,
	// and GORM Save() would write NULL to the database — wiping existing data.
	if req.AssetAccountID != nil {
		existingAsset.AssetAccountID = req.AssetAccountID
	}
	if req.DepreciationAccountID != nil {
		existingAsset.DepreciationAccountID = req.DepreciationAccountID
	}
	if req.AccumulatedDepreciationAccountID != nil {
		existingAsset.AccumulatedDepreciationAccountID = req.AccumulatedDepreciationAccountID
	}

	err = ac.assetService.UpdateAsset(existingAsset)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to update asset",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Asset updated successfully",
		"data":    existingAsset,
	})
}

// DeleteAsset deletes an asset and reverses all related journals
func (ac *AssetController) DeleteAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid asset ID"})
		return
	}

	// Step 0: Check closed period BEFORE any mutations
	asset, err := ac.assetService.GetAssetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Asset not found", "details": err.Error()})
		return
	}
	if err := services.CheckClosedPeriod(ac.db, asset.PurchaseDate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Step 1: Reverse all SSOT journals before deleting the asset
	if err := ac.assetJournalSvc.ReverseAssetJournals(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to reverse asset journals. Asset was NOT deleted.",
			"details": err.Error(),
		})
		return
	}

	// Step 2: Soft-delete the asset
	err = ac.assetService.DeleteAsset(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete asset (journals were reversed)",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Asset deleted successfully and all journals reversed",
	})
}

// GetAssetsSummary returns summary statistics for assets
func (ac *AssetController) GetAssetsSummary(c *gin.Context) {
	summary, err := ac.assetService.GetAssetsSummary()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get assets summary",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Assets summary retrieved successfully",
		"data":    summary,
	})
}

// GetDepreciationReport returns depreciation report for all assets
func (ac *AssetController) GetDepreciationReport(c *gin.Context) {
	report, err := ac.assetService.GetAssetsForDepreciationReport()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to generate depreciation report",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Depreciation report generated successfully",
		"data":    report,
		"count":   len(report),
	})
}

// GetDepreciationSchedule returns depreciation schedule for a specific asset
func (ac *AssetController) GetDepreciationSchedule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid asset ID"})
		return
	}

	asset, err := ac.assetService.GetAssetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Asset not found",
			"details": err.Error(),
		})
		return
	}

	schedule, err := ac.assetService.GetDepreciationSchedule(asset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to generate depreciation schedule",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Depreciation schedule generated successfully",
		"data": gin.H{
			"asset":    asset,
			"schedule": schedule,
		},
	})
}

// CalculateCurrentDepreciation calculates current depreciation for an asset
func (ac *AssetController) CalculateCurrentDepreciation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid asset ID"})
		return
	}

	asset, err := ac.assetService.GetAssetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Asset not found",
			"details": err.Error(),
		})
		return
	}

	// Get date parameter (optional, defaults to now)
	dateStr := c.DefaultQuery("as_of_date", "")
	var asOfDate time.Time
	if dateStr != "" {
		asOfDate, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format. Use YYYY-MM-DD"})
			return
		}
	} else {
		asOfDate = time.Now()
	}

	depreciation, err := ac.assetService.CalculateDepreciation(asset, asOfDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to calculate depreciation",
			"details": err.Error(),
		})
		return
	}

	currentBookValue := asset.PurchasePrice - depreciation

	c.JSON(http.StatusOK, gin.H{
		"message": "Depreciation calculated successfully",
		"data": gin.H{
			"asset_id":                 asset.ID,
			"asset_name":               asset.Name,
			"as_of_date":               asOfDate.Format("2006-01-02"),
			"purchase_price":           asset.PurchasePrice,
			"salvage_value":            asset.SalvageValue,
			"accumulated_depreciation": depreciation,
			"current_book_value":       currentBookValue,
			"depreciation_method":      asset.DepreciationMethod,
			"useful_life_years":        asset.UsefulLife,
		},
	})
}

// UploadAssetImage handles asset image upload
func (ac *AssetController) UploadAssetImage(c *gin.Context) {
	assetID, err := strconv.Atoi(c.PostForm("asset_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid asset ID"})
		return
	}

	// Check if asset exists
	asset, err := ac.assetService.GetAssetByID(uint(assetID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Asset not found"})
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
		"image/webp": true,
	}

	if !allowedTypes[file.Header.Get("Content-Type")] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type. Only JPEG, PNG, GIF, and WebP are allowed"})
		return
	}

	// Validate file size (max 5MB)
	maxSize := int64(5 * 1024 * 1024) // 5MB
	if file.Size > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File size too large. Maximum 5MB allowed"})
		return
	}

	// Create uploads directory if it doesn't exist
	uploadPath := "./uploads/assets/"
	if err := os.MkdirAll(uploadPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload directory"})
		return
	}

	// Generate unique filename
	filename := fmt.Sprintf("%d_%d_%s", assetID, time.Now().Unix(), file.Filename)
	filePath := uploadPath + filename

	// Save the file
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	// Update asset with image path
	relativeImagePath := "/uploads/assets/" + filename
	asset.ImagePath = relativeImagePath

	if err := ac.assetService.UpdateAsset(asset); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update asset image path"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Image uploaded successfully",
		"filename": filename,
		"path":     relativeImagePath,
		"asset":    asset,
	})
}

// GetAssetCategories retrieves all asset categories
func (ac *AssetController) GetAssetCategories(c *gin.Context) {
	categories, err := ac.assetService.GetAssetCategories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve asset categories",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Asset categories retrieved successfully",
		"data":    categories,
		"count":   len(categories),
	})
}

// CreateAssetCategory creates a new asset category
func (ac *AssetController) CreateAssetCategory(c *gin.Context) {
	var req struct {
		Code                               string `json:"code" binding:"required"`
		Name                               string `json:"name" binding:"required"`
		Description                        string `json:"description"`
		ParentID                           *uint  `json:"parent_id"`
		IsActive                           bool   `json:"is_active"`
		FixedAssetAccountCode              string `json:"fixed_asset_account_code"`
		AccumulatedDepreciationAccountCode string `json:"accumulated_depreciation_account_code"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	category := &models.AssetCategory{
		Code:                               req.Code,
		Name:                               req.Name,
		Description:                        req.Description,
		ParentID:                           req.ParentID,
		IsActive:                           req.IsActive,
		FixedAssetAccountCode:              req.FixedAssetAccountCode,
		AccumulatedDepreciationAccountCode: req.AccumulatedDepreciationAccountCode,
	}

	// Set default IsActive to true if not specified
	if !req.IsActive {
		category.IsActive = true
	}

	if err := ac.assetService.CreateAssetCategory(category); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to create asset category",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Asset category created successfully",
		"data":    category,
	})
}

// RunDepreciation runs monthly depreciation for all active assets via SSOT journal
func (ac *AssetController) RunDepreciation(c *gin.Context) {
	var req struct {
		Year   int  `json:"year" binding:"required"`
		Month  int  `json:"month" binding:"required,min=1,max=12"`
		UserID uint `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data. Provide year and month (1-12).",
			"details": err.Error(),
		})
		return
	}

	// Use authenticated user if user_id not provided
	userID := req.UserID
	if userID == 0 {
		if uid, exists := c.Get("user_id"); exists {
			userID = uid.(uint)
		}
	}
	if userID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	result, err := ac.assetJournalSvc.RunMonthlyDepreciation(req.Year, req.Month, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to run depreciation",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Depreciation run completed for %s", result.Period),
		"data":    result,
	})
}

// ActivateAsset activates a PENDING_SETUP asset after user has set depreciation parameters.
// Validates that useful_life, depreciation_method are set before activating.
func (ac *AssetController) ActivateAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid asset ID"})
		return
	}

	asset, err := ac.assetService.GetAssetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Asset not found", "details": err.Error()})
		return
	}

	// Allow activation from PENDING_SETUP or INACTIVE status
	if asset.Status != models.AssetStatusPendingSetup && asset.Status != models.AssetStatusInactive {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Asset status is '%s'. Only PENDING_SETUP or INACTIVE assets can be activated.", asset.Status),
		})
		return
	}

	// Validate required depreciation fields are set (skip for Tanah/Land)
	categoryLower := strings.ToLower(strings.TrimSpace(asset.Category))
	isLand := categoryLower == "tanah" || categoryLower == "land"

	if !isLand {
		if asset.UsefulLife <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Masa manfaat (useful_life) harus diisi sebelum mengaktifkan aset. Silakan edit aset terlebih dahulu.",
			})
			return
		}
		if asset.DepreciationMethod == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Metode penyusutan (depreciation_method) harus dipilih. Pilih STRAIGHT_LINE atau DECLINING_BALANCE.",
			})
			return
		}
		if asset.DepreciationAccountID == nil || *asset.DepreciationAccountID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Akun beban penyusutan (depreciation_account) harus dipilih sebelum mengaktifkan aset. Silakan edit aset terlebih dahulu.",
			})
			return
		}
		if asset.AccumulatedDepreciationAccountID == nil || *asset.AccumulatedDepreciationAccountID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Akun akumulasi penyusutan (accumulated_depreciation_account) harus dipilih sebelum mengaktifkan aset. Silakan edit aset terlebih dahulu.",
			})
			return
		}
	}

	// Activate the asset
	asset.Status = models.AssetStatusActive
	asset.IsActive = true

	if err := ac.db.Model(asset).Select("Status", "IsActive").Updates(asset).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to activate asset", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Aset %s (%s) berhasil diaktifkan. Depresiasi akan berjalan mulai bulan depan.", asset.Code, asset.Name),
		"data":    asset,
	})
}

// Helper function to generate Google Maps URL from coordinates
func generateMapsURL(coordinates string) string {
	if coordinates == "" {
		return ""
	}

	// Validate coordinate format (lat,lng)
	parts := strings.Split(coordinates, ",")
	if len(parts) != 2 {
		return ""
	}

	// Generate Google Maps URL
	return fmt.Sprintf("https://www.google.com/maps?q=%s", coordinates)
}
