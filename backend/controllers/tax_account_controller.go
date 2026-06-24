package controllers

import (
	"net/http"
	"strconv"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"

	"github.com/gin-gonic/gin"
)

// TaxAccountController handles tax account settings endpoints
type TaxAccountController struct {
	taxAccountService *services.TaxAccountService
	accountService    *services.COAService
}

// NewTaxAccountController creates a new TaxAccountController instance
func NewTaxAccountController(
	taxAccountService *services.TaxAccountService,
	accountService *services.COAService,
) *TaxAccountController {
	return &TaxAccountController{
		taxAccountService: taxAccountService,
		accountService:    accountService,
	}
}

// GetCurrentSettings gets the current active tax account settings
// @Summary Get withholding tax and inventory settings
// @Description Retrieve the currently active withholding tax and inventory account settings
// @Tags Withholding Tax & Inventory Settings
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} models.TaxAccountSettingsResponse
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/tax-accounts/current [get]
func (c *TaxAccountController) GetCurrentSettings(ctx *gin.Context) {
	settings, err := c.taxAccountService.GetSettings()
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"error":   "Tax account settings not found",
			"details": err.Error(),
		})
		return
	}

	response := settings.ToResponse()
	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
	})
}

// GetAllSettings gets all tax account settings (for admin)
// @Summary Get all tax account settings
// @Description Retrieve all tax account settings for administration
// @Tags Tax Account Settings
// @Accept json
// @Produce json
// @Success 200 {array} models.TaxAccountSettingsResponse
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/settings/tax-accounts/all [get]
func (c *TaxAccountController) GetAllSettings(ctx *gin.Context) {
	settings, err := c.taxAccountService.GetAllSettings()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve tax account settings",
			"details": err.Error(),
		})
		return
	}

	var responses []models.TaxAccountSettingsResponse
	for _, setting := range settings {
		responses = append(responses, setting.ToResponse())
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    responses,
	})
}

// CreateSettings creates new tax account settings
// @Summary Create withholding tax and inventory settings
// @Description Create new withholding tax and inventory account configuration
// @Tags Withholding Tax & Inventory Settings
// @Accept json
// @Produce json
// @Security Bearer
// @Param settings body models.TaxAccountSettingsCreateRequest true "Withholding tax and inventory settings data"
// @Success 201 {object} models.TaxAccountSettingsResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/tax-accounts [post]
func (c *TaxAccountController) CreateSettings(ctx *gin.Context) {
	var req models.TaxAccountSettingsCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	// Get user ID from context (assuming middleware sets it)
	userID, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	settings, err := c.taxAccountService.CreateSettings(&req, userID.(uint))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to create tax account settings",
			"details": err.Error(),
		})
		return
	}

	response := settings.ToResponse()
	ctx.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Tax account settings created successfully",
		"data":    response,
	})
}

// UpdateSettings updates existing tax account settings
// @Summary Update withholding tax and inventory settings
// @Description Update existing withholding tax and inventory account settings
// @Tags Withholding Tax & Inventory Settings
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Settings ID"
// @Param settings body models.TaxAccountSettingsUpdateRequest true "Withholding tax and inventory update data"
// @Success 200 {object} models.TaxAccountSettingsResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/tax-accounts/{id} [put]
func (c *TaxAccountController) UpdateSettings(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid settings ID",
		})
		return
	}

	var req models.TaxAccountSettingsUpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	// Get user ID from context
	userID, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	settings, err := c.taxAccountService.UpdateSettings(uint(id), &req, userID.(uint))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to update tax account settings",
			"details": err.Error(),
		})
		return
	}

	response := settings.ToResponse()
	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Tax account settings updated successfully",
		"data":    response,
	})
}

// ActivateSettings activates specific tax account settings
// @Summary Activate tax account settings
// @Description Activate a specific tax account configuration
// @Tags Tax Account Settings
// @Accept json
// @Produce json
// @Param id path int true "Settings ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/settings/tax-accounts/{id}/activate [post]
func (c *TaxAccountController) ActivateSettings(ctx *gin.Context) {
	idParam := ctx.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid settings ID",
		})
		return
	}

	// Get user ID from context
	userID, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	err = c.taxAccountService.ActivateSettings(uint(id), userID.(uint))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to activate tax account settings",
			"details": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Tax account settings activated successfully",
	})
}

// GetAvailableAccounts gets available accounts for dropdown selection
// @Summary Get available accounts for configuration
// @Description Get available accounts for withholding tax and inventory configuration
// @Tags Withholding Tax & Inventory Settings
// @Accept json
// @Produce json
// @Security Bearer
// @Param type query string false "Filter by account type (ASSET, LIABILITY)"
// @Param category query string false "Filter by category (CURRENT_ASSET, CURRENT_LIABILITY)"
// @Success 200 {array} models.AccountResponse
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/tax-accounts/accounts [get]
func (c *TaxAccountController) GetAvailableAccounts(ctx *gin.Context) {
	// Get query parameters
	accountType := ctx.Query("type")
	category := ctx.Query("category")

	// Build filter
	filter := make(map[string]interface{})
	filter["is_active"] = true
	filter["is_header"] = false // Exclude header accounts

	if accountType != "" {
		filter["type"] = accountType
	}
	if category != "" {
		filter["category"] = category
	}

	// Get accounts using COA service
	accounts, err := c.accountService.GetAccountsWithFilter(filter)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve available accounts",
			"details": err.Error(),
		})
		return
	}

	// Convert to response format
	var responses []models.AccountResponse
	for _, account := range accounts {
		responses = append(responses, models.AccountResponse{
			ID:       account.ID,
			Code:     account.Code,
			Name:     account.Name,
			Type:     account.Type,
			Category: account.Category,
			IsActive: account.IsActive,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    responses,
	})
}

// ValidateAccountConfiguration validates tax account configuration
// @Summary Validate tax account configuration
// @Description Validate the tax account configuration before saving
// @Tags Tax Account Settings
// @Accept json
// @Produce json
// @Param settings body models.TaxAccountSettingsCreateRequest true "Tax account settings data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/settings/tax-accounts/validate [post]
func (c *TaxAccountController) ValidateAccountConfiguration(ctx *gin.Context) {
	var req models.TaxAccountSettingsCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	// Create temporary settings object for validation
	settings := &models.TaxAccountSettings{
		SalesReceivableAccountID:  req.SalesReceivableAccountID,
		SalesCashAccountID:        req.SalesCashAccountID,
		SalesBankAccountID:        req.SalesBankAccountID,
		SalesRevenueAccountID:     req.SalesRevenueAccountID,
		SalesOutputVATAccountID:   req.SalesOutputVATAccountID,
		PurchasePayableAccountID:  req.PurchasePayableAccountID,
		PurchaseCashAccountID:     req.PurchaseCashAccountID,
		PurchaseBankAccountID:     req.PurchaseBankAccountID,
		PurchaseInputVATAccountID: req.PurchaseInputVATAccountID,
		PurchaseExpenseAccountID:  req.PurchaseExpenseAccountID,
		WithholdingTax21AccountID: req.WithholdingTax21AccountID,
		WithholdingTax23AccountID: req.WithholdingTax23AccountID,
		WithholdingTax25AccountID: req.WithholdingTax25AccountID,
		TaxPayableAccountID:       req.TaxPayableAccountID,
		InventoryAccountID:        req.InventoryAccountID,
		COGSAccountID:             req.COGSAccountID,
	}

	// Validate settings
	if err := settings.ValidateAccountSettings(); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":   "Validation failed",
			"details": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Configuration is valid",
	})
}

// RefreshCache refreshes the tax account settings cache
// @Summary Refresh cache
// @Description Refresh the tax account settings cache
// @Tags Tax Account Settings
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/settings/tax-accounts/refresh-cache [post]
func (c *TaxAccountController) RefreshCache(ctx *gin.Context) {
	err := c.taxAccountService.RefreshCache()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to refresh cache",
			"details": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Cache refreshed successfully",
	})
}

// GetAccountSuggestions provides account suggestions based on account type for setup wizard
// @Summary Get account configuration suggestions
// @Description Get suggestions for withholding tax and inventory account configuration
// @Tags Withholding Tax & Inventory Settings
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/tax-accounts/suggestions [get]
func (c *TaxAccountController) GetAccountSuggestions(ctx *gin.Context) {
	// Define account type mappings for suggestions
	suggestions := map[string]interface{}{
		"sales": map[string]interface{}{
			"receivable_account": map[string]interface{}{
				"recommended_types":      []string{"ASSET"},
				"recommended_categories": []string{"CURRENT_ASSET"},
				"suggested_codes":        []string{"1102-001", "1102"},
				"description":            "Piutang Usaha - untuk mencatat piutang dari penjualan kredit",
			},
			"cash_account": map[string]interface{}{
				"recommended_types":      []string{"ASSET"},
				"recommended_categories": []string{"CURRENT_ASSET"},
				"suggested_codes":        []string{"1101-002", "1101"},
				"description":            "Kas Besar - untuk penjualan tunai",
			},
			"bank_account": map[string]interface{}{
				"recommended_types":      []string{"ASSET"},
				"recommended_categories": []string{"CURRENT_ASSET"},
				"suggested_codes":        []string{"1101-003", "1101-004"},
				"description":            "Bank (di bawah KAS DAN SETARA KAS 1101) - untuk penjualan via transfer bank",
			},
			"revenue_account": map[string]interface{}{
				"recommended_types":      []string{"REVENUE"},
				"recommended_categories": []string{"SALES_REVENUE", "OPERATING_REVENUE"},
				"suggested_codes":        []string{"4101", "4100"},
				"description":            "Pendapatan Penjualan - untuk mencatat pendapatan dari penjualan",
			},
			"output_vat_account": map[string]interface{}{
				"recommended_types":      []string{"LIABILITY"},
				"recommended_categories": []string{"CURRENT_LIABILITY"},
				"suggested_codes":        []string{"2102-001", "2103"},
				"description":            "PPN Keluaran - untuk mencatat kewajiban PPN penjualan",
			},
		},
		"purchase": map[string]interface{}{
			"payable_account": map[string]interface{}{
				"recommended_types":      []string{"LIABILITY"},
				"recommended_categories": []string{"CURRENT_LIABILITY"},
				"suggested_codes":        []string{"2101-001", "2101"},
				"description":            "Hutang Usaha - untuk mencatat hutang dari pembelian kredit",
			},
			"cash_account": map[string]interface{}{
				"recommended_types":      []string{"ASSET"},
				"recommended_categories": []string{"CURRENT_ASSET"},
				"suggested_codes":        []string{"1101-002", "1101"},
				"description":            "Kas Besar - untuk pembelian tunai",
			},
			"bank_account": map[string]interface{}{
				"recommended_types":      []string{"ASSET"},
				"recommended_categories": []string{"CURRENT_ASSET"},
				"suggested_codes":        []string{"1101-003", "1101-004"},
				"description":            "Bank (di bawah KAS DAN SETARA KAS 1101) - untuk pembelian via transfer bank",
			},
			"input_vat_account": map[string]interface{}{
				"recommended_types":      []string{"LIABILITY"},
				"recommended_categories": []string{"CURRENT_LIABILITY"},
				"suggested_codes":        []string{"2102-002"},
				"description":            "PPN Masukan - untuk mencatat PPN yang dapat dikreditkan dari pembelian",
			},
			"expense_account": map[string]interface{}{
				"recommended_types":      []string{"EXPENSE"},
				"recommended_categories": []string{"OPERATING_EXPENSE", "ADMINISTRATIVE_EXPENSE"},
				"suggested_codes":        []string{"6102", "6001", "6000", "8102", "5100"},
				"description":            "Beban Operasional - untuk mencatat biaya operasional dari pembelian",
			},
		},
		"tax": map[string]interface{}{
			"withholding_tax21": map[string]interface{}{
				"recommended_types":      []string{"ASSET"},
				"recommended_categories": []string{"CURRENT_ASSET"},
				"suggested_codes":        []string{"1105-001"},
				"description":            "PPh 21 Dibayar Dimuka - pajak yang dipotong dari karyawan/dipotong oleh customer",
			},
			"withholding_tax23": map[string]interface{}{
				"recommended_types":      []string{"ASSET"},
				"recommended_categories": []string{"CURRENT_ASSET"},
				"suggested_codes":        []string{"1105-002"},
				"description":            "PPh 23 Dibayar Dimuka - pajak yang dipotong dari vendor/dipotong oleh customer",
			},
			"withholding_tax25": map[string]interface{}{
				"recommended_types":      []string{"ASSET"},
				"recommended_categories": []string{"CURRENT_ASSET"},
				"suggested_codes":        []string{"1105-003"},
				"description":            "PPh Final/25 Dibayar Dimuka - pajak final atau angsuran bulanan",
			},
			"tax_payable": map[string]interface{}{
				"recommended_types":      []string{"LIABILITY"},
				"recommended_categories": []string{"CURRENT_LIABILITY"},
				"suggested_codes":        []string{"2102-003", "2102-004"},
				"description":            "Hutang Pajak Penghasilan - untuk kewajiban pajak yang harus disetor (PPh 21/23/25)",
			},
		},
		"inventory": map[string]interface{}{
			"inventory_account": map[string]interface{}{
				"recommended_types":      []string{"ASSET"},
				"recommended_categories": []string{"CURRENT_ASSET"},
				"suggested_codes":        []string{"1103-001", "1103"},
				"description":            "Persediaan Barang Dagangan - untuk melacak nilai stok barang",
			},
			"cogs_account": map[string]interface{}{
				"recommended_types":      []string{"EXPENSE"},
				"recommended_categories": []string{"COST_OF_GOODS_SOLD"},
				"suggested_codes":        []string{"5101", "5100"},
				"description":            "Harga Pokok Penjualan (HPP) - untuk mencatat biaya barang yang terjual",
			},
		},
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    suggestions,
		"message": "Account suggestions retrieved successfully",
	})
}
