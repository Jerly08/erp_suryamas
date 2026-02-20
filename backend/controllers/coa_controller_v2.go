package controllers

import (
	"app-sistem-akuntansi/services"
	"app-sistem-akuntansi/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

// COAControllerV2 handles COA endpoints with proper display formatting
type COAControllerV2 struct {
	coaDisplayService *services.COADisplayServiceV2
}

// NewCOAControllerV2 creates a new instance of COAControllerV2
func NewCOAControllerV2(coaDisplayService *services.COADisplayServiceV2) *COAControllerV2 {
	return &COAControllerV2{
		coaDisplayService: coaDisplayService,
	}
}

// GetCOAWithDisplay returns all COA with proper display formatting
func (c *COAControllerV2) GetCOAWithDisplay(ctx *gin.Context) {
	coas, err := c.coaDisplayService.GetCOAForDisplay()
	if err != nil {
		utils.SendInternalError(ctx, "Failed to retrieve COA", err.Error())
		return
	}

	utils.SendSuccess(ctx, "COA retrieved successfully", coas)
}

// GetCOAByID returns a single COA with proper display formatting
func (c *COAControllerV2) GetCOAByID(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		utils.SendValidationError(ctx, "Invalid COA ID", map[string]string{"id": "must be a valid number"})
		return
	}

	coa, err := c.coaDisplayService.GetSingleCOAForDisplay(uint(id))
	if err != nil {
		utils.SendNotFound(ctx, "COA not found")
		return
	}

	utils.SendSuccess(ctx, "COA retrieved successfully", coa)
}

// GetCOABalancesByType returns COA balances grouped by type with display formatting
func (c *COAControllerV2) GetCOABalancesByType(ctx *gin.Context) {
	balances, err := c.coaDisplayService.GetAccountBalancesByType()
	if err != nil {
		utils.SendInternalError(ctx, "Failed to retrieve COA balances", err.Error())
		return
	}

	utils.SendSuccess(ctx, "COA balances retrieved successfully", balances)
}

// GetSpecificAccounts returns specific accounts for sales display
func (c *COAControllerV2) GetSpecificAccounts(ctx *gin.Context) {
	// Default accounts for sales display
	accountCodes := []string{
		"1101",     // Kas
		"1102",     // Piutang Usaha
		"2102-001", // PPN Keluaran
		"4101",     // Pendapatan Penjualan
	}

	accounts, err := c.coaDisplayService.GetSpecificAccountsForDisplay(accountCodes)
	if err != nil {
		utils.SendInternalError(ctx, "Failed to retrieve specific accounts", err.Error())
		return
	}

	utils.SendSuccess(ctx, "Specific accounts retrieved successfully", accounts)
}

// GetSalesRelatedAccounts returns accounts related to sales transactions
func (c *COAControllerV2) GetSalesRelatedAccounts(ctx *gin.Context) {
	// Accounts specifically for sales transactions
	accountCodes := []string{
		"1101",     // Kas
		"1102",     // Piutang Usaha
		"2102-001", // PPN Keluaran
		"4101",     // Pendapatan Penjualan
		"4102",     // Pendapatan Jasa
	}

	accounts, err := c.coaDisplayService.GetSpecificAccountsForDisplay(accountCodes)
	if err != nil {
		utils.SendInternalError(ctx, "Failed to retrieve sales accounts", err.Error())
		return
	}

	// Format response with additional metadata
	response := gin.H{
		"accounts": accounts,
		"metadata": gin.H{
			"display_rules": gin.H{
				"revenue":   "Always show as positive",
				"liability": "Always show as positive (PPN Keluaran)",
				"asset":     "Show as stored",
			},
		},
	}

	utils.SendSuccess(ctx, "Sales related accounts retrieved successfully", response)
}
