package controllers

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"

	"github.com/gin-gonic/gin"
)

type SettingsController struct {
	settingsService *services.SettingsService
}

// NewSettingsController creates a new instance of SettingsController
func NewSettingsController(settingsService *services.SettingsService) *SettingsController {
	return &SettingsController{
		settingsService: settingsService,
	}
}

// GetSettings handles GET /api/v1/settings
func (sc *SettingsController) GetSettings(c *gin.Context) {
	// Get settings from service
	settings, err := sc.settingsService.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve settings",
			"details": err.Error(),
		})
		return
	}

	// Convert to response format (without sensitive data)
	response := settings.ToResponse()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
	})
}

// GetPublicSettings handles GET /api/v1/settings/public
// Returns only non-sensitive settings accessible to all authenticated users
func (sc *SettingsController) GetPublicSettings(c *gin.Context) {
	settings, err := sc.settingsService.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve settings",
			"details": err.Error(),
		})
		return
	}

	// Return only public, non-sensitive fields
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"company_name":       settings.CompanyName,
			"company_address":    settings.CompanyAddress,
			"company_phone":      settings.CompanyPhone,
			"company_email":      settings.CompanyEmail,
			"company_logo":       settings.CompanyLogo,
			"company_logo_scale": settings.CompanyLogoScale,
			"company_icon":       settings.CompanyIcon,
			"company_type":       settings.CompanyType,
			"currency":           settings.Currency,
			"date_format":        settings.DateFormat,
			"fiscal_year_start":  settings.FiscalYearStart,
			"default_tax_rate":   settings.DefaultTaxRate,
			"language":           settings.Language,
			"timezone":           settings.Timezone,
			"thousand_separator": settings.ThousandSeparator,
			"decimal_separator":  settings.DecimalSeparator,
			"decimal_places":     settings.DecimalPlaces,
		},
	})
}

// GetPublicBranding returns only the fields needed to brand public application screens.
func (sc *SettingsController) GetPublicBranding(c *gin.Context) {
	settings, err := sc.settingsService.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve branding"})
		return
	}

	c.Header("Cache-Control", "no-store, max-age=0")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"company_name":       settings.CompanyName,
			"company_logo":       settings.CompanyLogo,
			"company_logo_scale": settings.CompanyLogoScale,
			"company_icon":       settings.CompanyIcon,
			"updated_at":         settings.UpdatedAt,
		},
	})
}

// GetPublicBrandingIcon intentionally serves a blank browser favicon.
func (sc *SettingsController) GetPublicBrandingIcon(c *gin.Context) {
	c.Header("Cache-Control", "no-store, max-age=0")
	c.Redirect(http.StatusTemporaryRedirect, "/favicon.svg?v=no-avery")
}

// GetPublicBrandingLogo redirects a stable loading-screen URL to the current uploaded logo.
func (sc *SettingsController) GetPublicBrandingLogo(c *gin.Context) {
	settings, err := sc.settingsService.GetSettings()
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	logoPath := strings.TrimSpace(settings.CompanyLogo)
	if !strings.HasPrefix(logoPath, "/uploads/company/") {
		logoPath = "/logo_avery.png"
	}

	c.Header("Cache-Control", "no-store, max-age=0")
	c.Redirect(http.StatusTemporaryRedirect, logoPath)
}

// GetPublicBrandingManifest serves an installable manifest using the current company branding.
func (sc *SettingsController) GetPublicBrandingManifest(c *gin.Context) {
	settings, err := sc.settingsService.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve branding manifest"})
		return
	}

	companyName := strings.TrimSpace(settings.CompanyName)
	if companyName == "" {
		companyName = "Finance Management"
	}

	c.Header("Cache-Control", "no-store, max-age=0")
	c.Header("Content-Type", "application/manifest+json")
	c.JSON(http.StatusOK, gin.H{
		"name":             companyName + " Finance Management",
		"short_name":       companyName,
		"description":      companyName + " accounting and finance management application",
		"start_url":        "/dashboard",
		"scope":            "/",
		"display":          "standalone",
		"orientation":      "portrait-primary",
		"background_color": "#f8f9fa",
		"theme_color":      "#2196F3",
		"icons": []gin.H{
			{
				"src":   "/favicon.svg?v=no-avery",
				"sizes": "32x32",
				"type":  "image/svg+xml",
			},
		},
	})
}

// UpdateSettings handles PUT /api/v1/settings
func (sc *SettingsController) UpdateSettings(c *gin.Context) {
	// Get user ID from context (assuming it's set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	// Check if user has admin role
	userRole, _ := c.Get("role")
	if userRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Only administrators can update settings",
		})
		return
	}

	// Parse request body
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Remove any sensitive fields that shouldn't be updated directly
	delete(updates, "id")
	delete(updates, "created_at")
	delete(updates, "updated_at")
	delete(updates, "deleted_at")

	// Convert userID to uint
	uid, ok := userID.(uint)
	if !ok {
		// Try to convert from float64 (common in JWT claims)
		if floatID, ok := userID.(float64); ok {
			uid = uint(floatID)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Invalid user ID format",
			})
			return
		}
	}

	// Update settings through service
	if err := sc.settingsService.UpdateSettings(updates, uid); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to update settings",
			"details": err.Error(),
		})
		return
	}

	// Get updated settings
	settings, err := sc.settingsService.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve updated settings",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Settings updated successfully",
		"data":    settings.ToResponse(),
	})
}

// UpdateCompanyInfo handles PUT /api/v1/settings/company
func (sc *SettingsController) UpdateCompanyInfo(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	// Check if user has admin role
	userRole, _ := c.Get("role")
	if userRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Only administrators can update company information",
		})
		return
	}

	// Parse request body
	var companyInfo struct {
		CompanyName    string `json:"company_name"`
		CompanyAddress string `json:"company_address"`
		CompanyPhone   string `json:"company_phone"`
		CompanyEmail   string `json:"company_email"`
		CompanyWebsite string `json:"company_website"`
		TaxNumber      string `json:"tax_number"`
	}

	if err := c.ShouldBindJSON(&companyInfo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Convert to map for update
	updates := map[string]interface{}{
		"company_name":    companyInfo.CompanyName,
		"company_address": companyInfo.CompanyAddress,
		"company_phone":   companyInfo.CompanyPhone,
		"company_email":   companyInfo.CompanyEmail,
		"company_website": companyInfo.CompanyWebsite,
		"tax_number":      companyInfo.TaxNumber,
	}

	// Remove empty values
	for key, value := range updates {
		if str, ok := value.(string); ok && str == "" {
			delete(updates, key)
		}
	}

	// Convert userID to uint
	uid, ok := userID.(uint)
	if !ok {
		if floatID, ok := userID.(float64); ok {
			uid = uint(floatID)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Invalid user ID format",
			})
			return
		}
	}

	// Update settings
	if err := sc.settingsService.UpdateSettings(updates, uid); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to update company information",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Company information updated successfully",
	})
}

// UpdateSystemConfig handles PUT /api/v1/settings/system
func (sc *SettingsController) UpdateSystemConfig(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	// Check if user has admin role
	userRole, _ := c.Get("role")
	if userRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Only administrators can update system configuration",
		})
		return
	}

	// Parse request body
	var systemConfig struct {
		Currency          string  `json:"currency"`
		DateFormat        string  `json:"date_format"`
		FiscalYearStart   string  `json:"fiscal_year_start"`
		DefaultTaxRate    float64 `json:"default_tax_rate"`
		Language          string  `json:"language"`
		Timezone          string  `json:"timezone"`
		ThousandSeparator string  `json:"thousand_separator"`
		DecimalSeparator  string  `json:"decimal_separator"`
		DecimalPlaces     int     `json:"decimal_places"`
	}

	if err := c.ShouldBindJSON(&systemConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Convert to map for update
	updates := make(map[string]interface{})

	if systemConfig.Currency != "" {
		updates["currency"] = systemConfig.Currency
	}
	if systemConfig.DateFormat != "" {
		updates["date_format"] = systemConfig.DateFormat
	}
	if systemConfig.FiscalYearStart != "" {
		updates["fiscal_year_start"] = systemConfig.FiscalYearStart
	}
	if systemConfig.DefaultTaxRate >= 0 {
		updates["default_tax_rate"] = systemConfig.DefaultTaxRate
	}
	if systemConfig.Language != "" {
		updates["language"] = systemConfig.Language
	}
	if systemConfig.Timezone != "" {
		updates["timezone"] = systemConfig.Timezone
	}
	if systemConfig.ThousandSeparator != "" {
		updates["thousand_separator"] = systemConfig.ThousandSeparator
	}
	if systemConfig.DecimalSeparator != "" {
		updates["decimal_separator"] = systemConfig.DecimalSeparator
	}
	if systemConfig.DecimalPlaces >= 0 {
		updates["decimal_places"] = systemConfig.DecimalPlaces
	}

	// Convert userID to uint
	uid, ok := userID.(uint)
	if !ok {
		if floatID, ok := userID.(float64); ok {
			uid = uint(floatID)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Invalid user ID format",
			})
			return
		}
	}

	// Update settings
	if err := sc.settingsService.UpdateSettings(updates, uid); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to update system configuration",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "System configuration updated successfully",
	})
}

// InitializeCOA handles POST /api/v1/settings/initialize-coa
func (sc *SettingsController) InitializeCOA(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	// Check if user has admin role
	userRole, _ := c.Get("role")
	if userRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Only administrators can initialize COA",
		})
		return
	}

	// Parse request body
	var request struct {
		CompanyType string `json:"company_type" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Validate type
	if request.CompanyType != models.CompanyTypeService &&
		request.CompanyType != models.CompanyTypeTrading &&
		request.CompanyType != models.CompanyTypeManufacturing {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid company type. Must be SERVICE, TRADING, or MANUFACTURING.",
		})
		return
	}

	// Convert userID to uint
	uid, ok := userID.(uint)
	if !ok {
		if floatID, ok := userID.(float64); ok {
			uid = uint(floatID)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Invalid user ID format",
			})
			return
		}
	}

	// Initialize COA
	if err := sc.settingsService.InitializeCOA(request.CompanyType, uid); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to initialize COA",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Chart of Accounts initialized successfully",
	})
}

// ResetToDefaults handles POST /api/v1/settings/reset
func (sc *SettingsController) ResetToDefaults(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	// Check if user has admin role
	userRole, _ := c.Get("role")
	if userRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Only administrators can reset settings",
		})
		return
	}

	// Convert userID to uint
	uid, ok := userID.(uint)
	if !ok {
		if floatID, ok := userID.(float64); ok {
			uid = uint(floatID)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Invalid user ID format",
			})
			return
		}
	}

	// Reset settings to defaults
	if err := sc.settingsService.ResetToDefaults(uid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to reset settings to defaults",
			"details": err.Error(),
		})
		return
	}

	// Get updated settings
	settings, err := sc.settingsService.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve updated settings",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Settings reset to defaults successfully",
		"data":    settings.ToResponse(),
	})
}

// GetValidationRules handles GET /api/v1/settings/validation-rules
func (sc *SettingsController) GetValidationRules(c *gin.Context) {
	rules := sc.settingsService.GetValidationRules()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    rules,
	})
}

// GetSettingsHistory handles GET /api/v1/settings/history
func (sc *SettingsController) GetSettingsHistory(c *gin.Context) {
	// Check if user has admin role
	userRole, _ := c.Get("role")
	if userRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Only administrators can view settings history",
		})
		return
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	field := c.Query("field")
	action := c.Query("action")
	changedBy := c.Query("changed_by")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	filter := models.SettingsHistoryFilter{
		Field:     field,
		Action:    action,
		ChangedBy: changedBy,
		StartDate: startDate,
		EndDate:   endDate,
		Page:      page,
		Limit:     limit,
	}

	result, err := sc.settingsService.GetSettingsHistory(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve settings history",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// UploadCompanyLogo handles POST /api/v1/settings/company/logo.
func (sc *SettingsController) UploadCompanyLogo(c *gin.Context) {
	sc.uploadCompanyBrandImage(c, "company_logo", "Company logo uploaded successfully")
}

// UploadCompanyIcon handles POST /api/v1/settings/company/icon.
func (sc *SettingsController) UploadCompanyIcon(c *gin.Context) {
	sc.uploadCompanyBrandImage(c, "company_icon", "Company icon uploaded successfully")
}

func (sc *SettingsController) uploadCompanyBrandImage(c *gin.Context, field, successMessage string) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Ensure admin (route also protected by middleware, this is an extra check)
	userRole, _ := c.Get("role")
	if userRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only administrators can upload company branding"})
		return
	}

	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get uploaded file"})
		return
	}

	uploadPath := "./uploads/company/"
	filename, filePath, err := prepareValidatedImageUpload(file, 0, uploadPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	// Update settings with relative web path
	relativePath := "/uploads/company/" + filename

	// Convert userID to uint
	uid, ok := userID.(uint)
	if !ok {
		if floatID, ok := userID.(float64); ok {
			uid = uint(floatID)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
			return
		}
	}

	if err := sc.settingsService.UpdateSettings(map[string]interface{}{field: relativePath}, uid); err != nil {
		// Cleanup file on failure
		_ = os.Remove(filePath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update company branding", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": successMessage,
		"path":    relativePath,
		"field":   field,
	})
}
