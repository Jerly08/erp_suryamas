package controllers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"app-sistem-akuntansi/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SSOTCashFlowController handles SSOT-based Cash Flow report generation
type SSOTCashFlowController struct {
	db                  *gorm.DB
	ssotCashFlowService *services.SSOTCashFlowService
	exportService       *services.CashFlowExportService
}

// NewSSOTCashFlowController creates a new SSOT Cash Flow controller
func NewSSOTCashFlowController(db *gorm.DB) *SSOTCashFlowController {
	return &SSOTCashFlowController{
		db:                  db,
		ssotCashFlowService: services.NewSSOTCashFlowService(db),
		exportService:       services.NewCashFlowExportService(db),
	}
}

// GetSSOTCashFlow generates Cash Flow statement from SSOT journal system
// @Summary Generate SSOT Cash Flow Statement
// @Description Generate a comprehensive Cash Flow statement using Single Source of Truth (SSOT) journal system with operating, investing, and financing activities analysis
// @Tags SSOT Reports
// @Accept json
// @Produce json
// @Param start_date query string true "Start date (YYYY-MM-DD)" example(2025-01-01)
// @Param end_date query string true "End date (YYYY-MM-DD)" example(2025-12-31)
// @Param format query string false "Output format" Enums(json,pdf,excel,csv) default(json)
// @Success 200 {object} map[string]interface{} "Cash Flow statement generated successfully"
// @Failure 400 {object} map[string]interface{} "Invalid request parameters"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /reports/ssot/cash-flow [get]
func (c *SSOTCashFlowController) GetSSOTCashFlow(ctx *gin.Context) {
	startDate := ctx.Query("start_date")
	endDate := ctx.Query("end_date")
	format := ctx.DefaultQuery("format", "json")

	// Validate required parameters
	if startDate == "" || endDate == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "start_date and end_date are required",
		})
		return
	}

	// Validate date formats
	if _, err := time.Parse("2006-01-02", startDate); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid start_date format. Use YYYY-MM-DD",
			"error":   err.Error(),
		})
		return
	}

	if _, err := time.Parse("2006-01-02", endDate); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid end_date format. Use YYYY-MM-DD",
			"error":   err.Error(),
		})
		return
	}

	// Generate SSOT Cash Flow data
	ssotData, err := c.ssotCashFlowService.GenerateSSOTCashFlow(startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to generate SSOT Cash Flow statement",
			"error":   err.Error(),
		})
		return
	}

	// Transform to frontend-compatible format
	responseData := c.TransformToFrontendFormat(ssotData)

	// Log response data for debugging
	log.Printf("Cash Flow Response data: %+v", responseData)

	switch format {
	case "json":
		ctx.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data":   responseData,
		})
	case "pdf":
		// Export as PDF file
		pdfData, err := c.exportService.ExportToPDF(ssotData)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to generate PDF",
				"error":   err.Error(),
			})
			return
		}

		filename := c.exportService.GetPDFFilename(ssotData)
		ctx.Header("Content-Type", "application/pdf")
		ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		ctx.Header("Content-Length", strconv.Itoa(len(pdfData)))
		ctx.Data(http.StatusOK, "application/pdf", pdfData)

	case "csv":
		userID := ctx.GetUint("user_id")
		csvData, err := c.exportService.ExportToCSV(ssotData, userID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to generate CSV",
				"error":   err.Error(),
			})
			return
		}

		filename := c.exportService.GetCSVFilename(ssotData)
		ctx.Header("Content-Type", "text/csv; charset=utf-8")
		ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		ctx.Header("Content-Length", strconv.Itoa(len(csvData)))
		ctx.Data(http.StatusOK, "text/csv; charset=utf-8", csvData)

	default:
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Unsupported format. Use json, pdf, or csv",
		})
	}
}

// TransformToFrontendFormat transforms SSOT cash flow data to the format expected by the frontend
func (c *SSOTCashFlowController) TransformToFrontendFormat(ssotData *services.SSOTCashFlowData) gin.H {
	sections := []gin.H{}

	// 1. OPERATING ACTIVITIES Section
	if ssotData.OperatingActivities.NetIncome != 0 ||
		len(ssotData.OperatingActivities.Adjustments.Items) > 0 ||
		len(ssotData.OperatingActivities.WorkingCapitalChanges.Items) > 0 {

		operatingItems := []gin.H{}

		// Net Income
		if ssotData.OperatingActivities.NetIncome != 0 {
			operatingItems = append(operatingItems, gin.H{
				"name":         "Net Income",
				"amount":       ssotData.OperatingActivities.NetIncome,
				"account_code": "",
				"type":         "base",
			})
		}

		// Non-cash adjustments
		if len(ssotData.OperatingActivities.Adjustments.Items) > 0 {
			adjustmentSubsection := gin.H{
				"name":  "Adjustments for Non-Cash Items",
				"total": ssotData.OperatingActivities.Adjustments.TotalAdjustments,
				"items": c.transformCFItemsToFrontendFormat(ssotData.OperatingActivities.Adjustments.Items),
			}
			operatingItems = append(operatingItems, adjustmentSubsection)
		}

		// Working capital changes
		if len(ssotData.OperatingActivities.WorkingCapitalChanges.Items) > 0 {
			workingCapitalSubsection := gin.H{
				"name":  "Changes in Working Capital",
				"total": ssotData.OperatingActivities.WorkingCapitalChanges.TotalWorkingCapitalChanges,
				"items": c.transformCFItemsToFrontendFormat(ssotData.OperatingActivities.WorkingCapitalChanges.Items),
			}
			operatingItems = append(operatingItems, workingCapitalSubsection)
		}

		sections = append(sections, gin.H{
			"name":  "OPERATING ACTIVITIES",
			"total": ssotData.OperatingActivities.TotalOperatingCashFlow,
			"items": operatingItems,
			"summary": gin.H{
				"net_income":                    ssotData.OperatingActivities.NetIncome,
				"total_adjustments":             ssotData.OperatingActivities.Adjustments.TotalAdjustments,
				"total_working_capital_changes": ssotData.OperatingActivities.WorkingCapitalChanges.TotalWorkingCapitalChanges,
			},
		})
	}

	// 2. INVESTING ACTIVITIES Section
	if ssotData.InvestingActivities.TotalInvestingCashFlow != 0 || len(ssotData.InvestingActivities.Items) > 0 {
		investingItems := []gin.H{}

		for _, item := range ssotData.InvestingActivities.Items {
			investingItems = append(investingItems, gin.H{
				"name":         item.AccountName,
				"amount":       item.Amount,
				"account_code": item.AccountCode,
				"type":         item.Type,
			})
		}

		sections = append(sections, gin.H{
			"name":  "INVESTING ACTIVITIES",
			"total": ssotData.InvestingActivities.TotalInvestingCashFlow,
			"items": investingItems,
			"summary": gin.H{
				"purchase_of_fixed_assets":   ssotData.InvestingActivities.PurchaseOfFixedAssets,
				"sale_of_fixed_assets":       ssotData.InvestingActivities.SaleOfFixedAssets,
				"purchase_of_investments":    ssotData.InvestingActivities.PurchaseOfInvestments,
				"sale_of_investments":        ssotData.InvestingActivities.SaleOfInvestments,
				"intangible_asset_purchases": ssotData.InvestingActivities.IntangibleAssetPurchases,
				"other_investing_activities": ssotData.InvestingActivities.OtherInvestingActivities,
			},
		})
	}

	// 3. FINANCING ACTIVITIES Section
	if ssotData.FinancingActivities.TotalFinancingCashFlow != 0 || len(ssotData.FinancingActivities.Items) > 0 {
		financingItems := []gin.H{}

		for _, item := range ssotData.FinancingActivities.Items {
			financingItems = append(financingItems, gin.H{
				"name":         item.AccountName,
				"amount":       item.Amount,
				"account_code": item.AccountCode,
				"type":         item.Type,
			})
		}

		sections = append(sections, gin.H{
			"name":  "FINANCING ACTIVITIES",
			"total": ssotData.FinancingActivities.TotalFinancingCashFlow,
			"items": financingItems,
			"summary": gin.H{
				"share_capital_increase":     ssotData.FinancingActivities.ShareCapitalIncrease,
				"share_capital_decrease":     ssotData.FinancingActivities.ShareCapitalDecrease,
				"long_term_debt_increase":    ssotData.FinancingActivities.LongTermDebtIncrease,
				"long_term_debt_decrease":    ssotData.FinancingActivities.LongTermDebtDecrease,
				"short_term_debt_increase":   ssotData.FinancingActivities.ShortTermDebtIncrease,
				"short_term_debt_decrease":   ssotData.FinancingActivities.ShortTermDebtDecrease,
				"dividends_paid":             ssotData.FinancingActivities.DividendsPaid,
				"other_financing_activities": ssotData.FinancingActivities.OtherFinancingActivities,
			},
		})
	}

	// 4. NET CASH FLOW Summary
	sections = append(sections, gin.H{
		"name":          "NET CASH FLOW",
		"total":         ssotData.NetCashFlow,
		"is_calculated": true,
		"items": []gin.H{
			{
				"name":   "Cash at Beginning of Period",
				"amount": ssotData.CashAtBeginning,
			},
			{
				"name":   "Net Cash Flow from Activities",
				"amount": ssotData.NetCashFlow,
			},
			{
				"name":   "Cash at End of Period",
				"amount": ssotData.CashAtEnd,
			},
		},
	})

	// Create the frontend-compatible response
	return gin.H{
		"title":      "Cash Flow Statement",
		"period":     ssotData.StartDate.Format("2006-01-02") + " - " + ssotData.EndDate.Format("2006-01-02"),
		"start_date": ssotData.StartDate.Format("2006-01-02"),
		"end_date":   ssotData.EndDate.Format("2006-01-02"),
		"company": gin.H{
			"name":   ssotData.Company.Name,
			"period": ssotData.StartDate.Format("02/01/2006") + " - " + ssotData.EndDate.Format("02/01/2006"),
		},
		"sections": sections,
		"enhanced": ssotData.Enhanced,
		"hasData":  len(sections) > 0 && ssotData.NetCashFlow != 0,
		// Top-level fields for frontend compatibility
		"net_cash_flow":              ssotData.NetCashFlow,
		"cash_at_beginning":          ssotData.CashAtBeginning,
		"cash_at_end":                ssotData.CashAtEnd,
		"has_reconciliation_warning": ssotData.HasReconciliationWarning,
		"reconciliation_difference":  ssotData.ReconciliationDifference,
		"currency":                   ssotData.Currency,
		"generated_at":               ssotData.GeneratedAt.Format(time.RFC3339),
		"data_source":                "SSOT Journal System",
		// Add structured activity sections for direct frontend access
		"operating_activities": gin.H{
			"net_income": ssotData.OperatingActivities.NetIncome,
			"adjustments": gin.H{
				"depreciation":                ssotData.OperatingActivities.Adjustments.Depreciation,
				"amortization":                ssotData.OperatingActivities.Adjustments.Amortization,
				"bad_debt_expense":            ssotData.OperatingActivities.Adjustments.BadDebtExpense,
				"gain_loss_on_asset_disposal": ssotData.OperatingActivities.Adjustments.GainLossOnAssetDisposal,
				"other_non_cash_items":        ssotData.OperatingActivities.Adjustments.OtherNonCashItems,
				"total_adjustments":           ssotData.OperatingActivities.Adjustments.TotalAdjustments,
				"items":                       c.transformCFItemsToFrontendFormat(ssotData.OperatingActivities.Adjustments.Items),
			},
			"working_capital_changes": gin.H{
				"accounts_receivable_change":    ssotData.OperatingActivities.WorkingCapitalChanges.AccountsReceivableChange,
				"inventory_change":              ssotData.OperatingActivities.WorkingCapitalChanges.InventoryChange,
				"prepaid_expenses_change":       ssotData.OperatingActivities.WorkingCapitalChanges.PrepaidExpensesChange,
				"accounts_payable_change":       ssotData.OperatingActivities.WorkingCapitalChanges.AccountsPayableChange,
				"accrued_liabilities_change":    ssotData.OperatingActivities.WorkingCapitalChanges.AccruedLiabilitiesChange,
				"other_working_capital_change":  ssotData.OperatingActivities.WorkingCapitalChanges.OtherWorkingCapitalChange,
				"total_working_capital_changes": ssotData.OperatingActivities.WorkingCapitalChanges.TotalWorkingCapitalChanges,
				"items":                         c.transformCFItemsToFrontendFormat(ssotData.OperatingActivities.WorkingCapitalChanges.Items),
			},
			"total_operating_cash_flow": ssotData.OperatingActivities.TotalOperatingCashFlow,
		},
		"investing_activities": gin.H{
			"purchase_of_fixed_assets":   ssotData.InvestingActivities.PurchaseOfFixedAssets,
			"sale_of_fixed_assets":       ssotData.InvestingActivities.SaleOfFixedAssets,
			"purchase_of_investments":    ssotData.InvestingActivities.PurchaseOfInvestments,
			"sale_of_investments":        ssotData.InvestingActivities.SaleOfInvestments,
			"intangible_asset_purchases": ssotData.InvestingActivities.IntangibleAssetPurchases,
			"other_investing_activities": ssotData.InvestingActivities.OtherInvestingActivities,
			"total_investing_cash_flow":  ssotData.InvestingActivities.TotalInvestingCashFlow,
			"items":                      c.transformCFItemsToFrontendFormat(ssotData.InvestingActivities.Items),
		},
		"financing_activities": gin.H{
			"share_capital_increase":     ssotData.FinancingActivities.ShareCapitalIncrease,
			"share_capital_decrease":     ssotData.FinancingActivities.ShareCapitalDecrease,
			"long_term_debt_increase":    ssotData.FinancingActivities.LongTermDebtIncrease,
			"long_term_debt_decrease":    ssotData.FinancingActivities.LongTermDebtDecrease,
			"short_term_debt_increase":   ssotData.FinancingActivities.ShortTermDebtIncrease,
			"short_term_debt_decrease":   ssotData.FinancingActivities.ShortTermDebtDecrease,
			"dividends_paid":             ssotData.FinancingActivities.DividendsPaid,
			"other_financing_activities": ssotData.FinancingActivities.OtherFinancingActivities,
			"total_financing_cash_flow":  ssotData.FinancingActivities.TotalFinancingCashFlow,
			"items":                      c.transformCFItemsToFrontendFormat(ssotData.FinancingActivities.Items),
		},
		"cash_flow_ratios": gin.H{
			"operating_cash_flow_ratio": ssotData.CashFlowRatios.OperatingCashFlowRatio,
			"cash_flow_to_debt_ratio":   ssotData.CashFlowRatios.CashFlowToDebtRatio,
			"free_cash_flow":            ssotData.CashFlowRatios.FreeCashFlow,
			"cash_flow_per_share":       ssotData.CashFlowRatios.CashFlowPerShare,
		},
		"account_details": c.transformAccountDetailsToFrontendFormat(ssotData.AccountDetails),
		"summary": gin.H{
			"operating_cash_flow": ssotData.OperatingActivities.TotalOperatingCashFlow,
			"investing_cash_flow": ssotData.InvestingActivities.TotalInvestingCashFlow,
			"financing_cash_flow": ssotData.FinancingActivities.TotalFinancingCashFlow,
			"net_cash_flow":       ssotData.NetCashFlow,
			"cash_at_beginning":   ssotData.CashAtBeginning,
			"cash_at_end":         ssotData.CashAtEnd,
		},
		"cashFlowRatios": gin.H{
			"operating_cash_flow_ratio": ssotData.CashFlowRatios.OperatingCashFlowRatio,
			"cash_flow_to_debt_ratio":   ssotData.CashFlowRatios.CashFlowToDebtRatio,
			"free_cash_flow":            ssotData.CashFlowRatios.FreeCashFlow,
			"cash_flow_per_share":       ssotData.CashFlowRatios.CashFlowPerShare,
		},
		"message": c.generateAnalysisMessage(ssotData),
	}
}

// transformCFItemsToFrontendFormat transforms CFSectionItem to frontend format
func (c *SSOTCashFlowController) transformCFItemsToFrontendFormat(items []services.CFSectionItem) []gin.H {
	var transformed []gin.H
	for _, item := range items {
		transformed = append(transformed, gin.H{
			"account_code": item.AccountCode,
			"account_name": item.AccountName,
			"amount":       item.Amount,
			"type":         item.Type,
		})
	}
	return transformed
}

// transformAccountDetailsToFrontendFormat transforms account balance details to frontend format
func (c *SSOTCashFlowController) transformAccountDetailsToFrontendFormat(accountDetails []services.SSOTAccountBalance) []gin.H {
	var transformed []gin.H
	for _, account := range accountDetails {
		transformed = append(transformed, gin.H{
			"account_id":   account.AccountID,
			"account_code": account.AccountCode,
			"account_name": account.AccountName,
			"account_type": account.AccountType,
			"debit_total":  account.DebitTotal,
			"credit_total": account.CreditTotal,
			"net_balance":  account.NetBalance,
		})
	}
	return transformed
}

// generateAnalysisMessage creates a contextual message based on the cash flow data
func (c *SSOTCashFlowController) generateAnalysisMessage(ssotData *services.SSOTCashFlowData) string {
	if ssotData.CashAtEnd == ssotData.CashAtBeginning {
		return "No net change in cash position detected. Review transactions to ensure all cash activities are properly recorded in the SSOT journal system."
	}

	if ssotData.OperatingActivities.TotalOperatingCashFlow < 0 {
		return "Operating activities generated negative cash flow. This may indicate operational challenges or seasonal timing differences."
	}

	if ssotData.NetCashFlow > 0 && ssotData.OperatingActivities.TotalOperatingCashFlow > 0 {
		return "Positive cash generation from operations with overall positive net cash flow indicates healthy cash management."
	}

	if ssotData.InvestingActivities.TotalInvestingCashFlow < 0 {
		return "Investment activities show cash outflow, indicating capital expenditure or asset purchases which may signal business expansion."
	}

	if ssotData.FinancingActivities.TotalFinancingCashFlow > 0 {
		return "Financing activities generated positive cash flow, indicating new capital raised or debt financing obtained."
	}

	return "Cash Flow statement successfully generated from SSOT journal system with comprehensive analysis of all cash activities."
}

// GetSSOTCashFlowSummary provides a simplified summary view of the cash flow statement
func (c *SSOTCashFlowController) GetSSOTCashFlowSummary(ctx *gin.Context) {
	startDate := ctx.Query("start_date")
	endDate := ctx.Query("end_date")

	// Validate required parameters
	if startDate == "" || endDate == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "start_date and end_date are required",
		})
		return
	}

	// Generate full cash flow data
	ssotData, err := c.ssotCashFlowService.GenerateSSOTCashFlow(startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to generate cash flow summary",
			"error":   err.Error(),
		})
		return
	}

	// Create summary response
	summary := gin.H{
		"company":  ssotData.Company,
		"period":   startDate + " to " + endDate,
		"currency": ssotData.Currency,
		"activities": gin.H{
			"operating": gin.H{
				"cash_flow":  ssotData.OperatingActivities.TotalOperatingCashFlow,
				"net_income": ssotData.OperatingActivities.NetIncome,
			},
			"investing": gin.H{
				"cash_flow": ssotData.InvestingActivities.TotalInvestingCashFlow,
			},
			"financing": gin.H{
				"cash_flow": ssotData.FinancingActivities.TotalFinancingCashFlow,
			},
		},
		"cash_position": gin.H{
			"beginning_cash": ssotData.CashAtBeginning,
			"ending_cash":    ssotData.CashAtEnd,
			"net_change":     ssotData.NetCashFlow,
		},
		"ratios": gin.H{
			"operating_cash_flow_ratio": ssotData.CashFlowRatios.OperatingCashFlowRatio,
			"free_cash_flow":            ssotData.CashFlowRatios.FreeCashFlow,
		},
		"generated_at": ssotData.GeneratedAt,
		"enhanced":     ssotData.Enhanced,
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Cash Flow summary generated successfully",
		"data":    summary,
	})
}

// ValidateSSOTCashFlow validates if the cash flow statement balances correctly
func (c *SSOTCashFlowController) ValidateSSOTCashFlow(ctx *gin.Context) {
	startDate := ctx.Query("start_date")
	endDate := ctx.Query("end_date")

	// Validate required parameters
	if startDate == "" || endDate == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "start_date and end_date are required",
		})
		return
	}

	// Generate cash flow data
	ssotData, err := c.ssotCashFlowService.GenerateSSOTCashFlow(startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to validate cash flow statement",
			"error":   err.Error(),
		})
		return
	}

	// Create validation result
	expectedEndingCash := ssotData.CashAtBeginning + ssotData.NetCashFlow
	tolerance := 0.01
	isBalanced := (expectedEndingCash >= ssotData.CashAtEnd-tolerance && expectedEndingCash <= ssotData.CashAtEnd+tolerance)

	validationResult := gin.H{
		"period":               startDate + " to " + endDate,
		"is_balanced":          isBalanced,
		"cash_at_beginning":    ssotData.CashAtBeginning,
		"net_cash_flow":        ssotData.NetCashFlow,
		"expected_ending_cash": expectedEndingCash,
		"actual_ending_cash":   ssotData.CashAtEnd,
		"difference":           ssotData.CashAtEnd - expectedEndingCash,
		"tolerance":            tolerance,
		"validation_status":    map[bool]string{true: "PASS", false: "FAIL"}[isBalanced],
		"generated_at":         ssotData.GeneratedAt,
	}

	if !isBalanced {
		validationResult["issue"] = "Cash flow does not reconcile with beginning and ending cash balances"
		validationResult["recommendation"] = "Review journal entries for missing or incorrect cash transactions"
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Cash Flow validation completed",
		"data":    validationResult,
	})
}
