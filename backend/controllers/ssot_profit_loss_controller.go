package controllers

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
	"app-sistem-akuntansi/services"
	"app-sistem-akuntansi/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SSOTProfitLossController handles SSOT-based P&L report generation
type SSOTProfitLossController struct {
	db            *gorm.DB
	ssotPLService *services.SSOTProfitLossService
	pdfService    services.PDFServiceInterface
}

// NewSSOTProfitLossController creates a new SSOT P&L controller
func NewSSOTProfitLossController(db *gorm.DB) *SSOTProfitLossController {
	return &SSOTProfitLossController{
		db:            db,
		ssotPLService: services.NewSSOTProfitLossService(db),
		pdfService:    services.NewPDFService(db),
	}
}

// GetSSOTProfitLoss generates P&L report from SSOT journal system with frontend-compatible format
// @Summary Generate SSOT Profit & Loss Report
// @Description Generate a comprehensive Profit & Loss statement using Single Source of Truth (SSOT) journal system with real-time data integration
// @Tags SSOT Reports
// @Accept json
// @Produce json
// @Param start_date query string true "Start date (YYYY-MM-DD)" example(2025-01-01)
// @Param end_date query string true "End date (YYYY-MM-DD)" example(2025-12-31)
// @Param format query string false "Output format" Enums(json,pdf,excel,csv) default(json)
// @Success 200 {object} map[string]interface{} "P&L report generated successfully"
// @Failure 400 {object} map[string]interface{} "Invalid request parameters"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /reports/ssot-profit-loss [get]
func (c *SSOTProfitLossController) GetSSOTProfitLoss(ctx *gin.Context) {
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

	// Generate SSOT P&L data
	ssotData, err := c.ssotPLService.GenerateSSOTProfitLoss(startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to generate SSOT P&L report",
			"error":   err.Error(),
		})
		return
	}

	// Get language setting from database
	language := utils.GetUserLanguageFromSettings(c.db)

	// Send data in the format the frontend expects (sections format)
	responseData := c.TransformToFrontendFormat(ssotData, language)
	
	// Log response data for debugging
	log.Printf("P&L Response data: %+v", responseData)

	switch format {
	case "json":
		ctx.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data":   responseData,
		})
	case "pdf":
		// Generate actual PDF using the PDF service
		pdfBytes, err := c.pdfService.GenerateSSOTProfitLossPDF(ssotData)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to generate SSOT P&L PDF",
				"error":   err.Error(),
			})
			return
		}
		
		// Set headers for PDF download
		filename := fmt.Sprintf("SSOT_ProfitLoss_%s_to_%s.pdf", 
			ssotData.StartDate.Format("2006-01-02"), 
			ssotData.EndDate.Format("2006-01-02"))
		
		ctx.Header("Content-Type", "application/pdf")
		ctx.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		ctx.Header("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
		
		ctx.Data(http.StatusOK, "application/pdf", pdfBytes)
case "excel", "csv":
		if format == "csv" {
			// Build CSV on the fly from responseData sections
			var buf bytes.Buffer
			w := csv.NewWriter(&buf)
			// header
			_ = w.Write([]string{"Section", "Subsection", "Account Code", "Account Name", "Amount"})
			// flatten sections
			if secs, ok := responseData["sections"].([]gin.H); ok {
				for _, sec := range secs {
					secName, _ := sec["name"].(string)
					// direct items
					if items, ok := sec["items"].([]gin.H); ok {
						for _, it := range items {
							code, _ := it["account_code"].(string)
							name, _ := it["name"].(string)
							amt := fmt.Sprintf("%v", it["amount"])
							_ = w.Write([]string{secName, "", code, name, amt})
						}
					}
					// subsections if any
					if subs, ok := sec["subsections"].([]gin.H); ok {
						for _, sub := range subs {
							subName, _ := sub["name"].(string)
							if sits, ok := sub["items"].([]gin.H); ok {
								for _, it := range sits {
									code, _ := it["account_code"].(string)
									name, _ := it["name"].(string)
									amt := fmt.Sprintf("%v", it["amount"])
									_ = w.Write([]string{secName, subName, code, name, amt})
								}
							}
						}
					}
				}
			}
			w.Flush()
			filename := fmt.Sprintf("SSOT_ProfitLoss_%s_to_%s.csv", ssotData.StartDate.Format("2006-01-02"), ssotData.EndDate.Format("2006-01-02"))
			ctx.Header("Content-Type", "text/csv")
			ctx.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
			ctx.Header("Content-Length", strconv.Itoa(buf.Len()))
			ctx.Data(http.StatusOK, "text/csv", buf.Bytes())
			return
		}
		// For Excel export (placeholder JSON contract)
		responseData["export_format"] = format
		responseData["export_ready"] = true
		ctx.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data":   responseData,
			"format": format,
			"message": "P&L data formatted for " + format + " export",
		})
	default:
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Unsupported format. Use json, pdf, or excel",
		})
	}
}

// TransformToFrontendFormat transforms SSOT data to the format expected by EnhancedProfitLossModal
func (c *SSOTProfitLossController) TransformToFrontendFormat(ssotData *services.SSOTProfitLossData, language string) gin.H {
	sections := []gin.H{}
	
	// 1. REVENUE Section
	if ssotData.Revenue.TotalRevenue != 0 || len(ssotData.Revenue.Items) > 0 {
		revenueItems := []gin.H{}
		
		// If we have detailed items, use them
		if len(ssotData.Revenue.Items) > 0 {
			// Deduplicate by account code and sum amounts
			itemsByCode := make(map[string]gin.H)
			for _, item := range ssotData.Revenue.Items {
				if existing, found := itemsByCode[item.AccountCode]; found {
					// Account code already exists, sum the amounts
					existingAmount := existing["amount"].(float64)
					itemsByCode[item.AccountCode] = gin.H{
						"name":         existing["name"],
						"amount":       existingAmount + item.Amount,
						"account_code": item.AccountCode,
					}
				} else {
					// First occurrence of this account code
					itemsByCode[item.AccountCode] = gin.H{
						"name":         item.AccountName,
						"amount":       item.Amount,
						"account_code": item.AccountCode,
					}
				}
			}
			
			// Convert map to slice
			for _, item := range itemsByCode {
				revenueItems = append(revenueItems, item)
			}
		} else if ssotData.Revenue.TotalRevenue > 0 {
			// If no detailed items but has total revenue, create a generic item
			revenueItems = append(revenueItems, gin.H{
				"name":         utils.T("sales_revenue", language),
				"amount":       ssotData.Revenue.TotalRevenue,
				"account_code": "4101",
			})
		}
		
		// Recalculate total from deduplicated items to ensure consistency
		actualTotal := 0.0
		for _, item := range revenueItems {
			actualTotal += item["amount"].(float64)
		}
		
		sections = append(sections, gin.H{
			"name":  utils.T("revenue", language),
			"total": actualTotal,  // Use actual total from deduplicated items
			"items": revenueItems,
		})
	}

	// 2. COST OF GOODS SOLD Section
	if ssotData.COGS.TotalCOGS != 0 || len(ssotData.COGS.Items) > 0 {
		cogsSubsections := []gin.H{}
		
		if ssotData.COGS.DirectMaterials != 0 {
			materialItems := c.filterItemsByPrefix(ssotData.COGS.Items, "510")
			if len(materialItems) > 0 {
				cogsSubsections = append(cogsSubsections, gin.H{
					"name":  utils.T("direct_materials", language),
					"total": ssotData.COGS.DirectMaterials,
					"items": materialItems,
				})
			}
		}
		
		if ssotData.COGS.DirectLabor != 0 {
			laborItems := c.filterItemsByPrefix(ssotData.COGS.Items, "511")
			if len(laborItems) > 0 {
				cogsSubsections = append(cogsSubsections, gin.H{
					"name":  utils.T("direct_labor", language),
					"total": ssotData.COGS.DirectLabor,
					"items": laborItems,
				})
			}
		}
		
		if ssotData.COGS.Manufacturing != 0 {
			mfgItems := c.filterItemsByPrefix(ssotData.COGS.Items, "512")
			if len(mfgItems) > 0 {
				cogsSubsections = append(cogsSubsections, gin.H{
					"name":  utils.T("manufacturing_overhead", language),
					"total": ssotData.COGS.Manufacturing,
					"items": mfgItems,
				})
			}
		}
		
		if ssotData.COGS.OtherCOGS != 0 {
			// Deduplicate other COGS items
			itemsByCode := make(map[string]gin.H)
			for _, item := range ssotData.COGS.Items {
				if len(item.AccountCode) >= 3 && (item.AccountCode[:3] == "513" || item.AccountCode[:3] == "514" || item.AccountCode[:3] == "519") {
					if existing, found := itemsByCode[item.AccountCode]; found {
						existingAmount := existing["amount"].(float64)
						itemsByCode[item.AccountCode] = gin.H{
							"name":         existing["name"],
							"amount":       existingAmount + item.Amount,
							"account_code": item.AccountCode,
						}
					} else {
						itemsByCode[item.AccountCode] = gin.H{
							"name":         item.AccountName,
							"amount":       item.Amount,
							"account_code": item.AccountCode,
						}
					}
				}
			}
			
			// Convert map to slice
			otherItems := []gin.H{}
			for _, item := range itemsByCode {
				otherItems = append(otherItems, item)
			}
			
			if len(otherItems) > 0 {
				cogsSubsections = append(cogsSubsections, gin.H{
					"name":  utils.T("other_cogs", language),
					"total": ssotData.COGS.OtherCOGS,
					"items": otherItems,
				})
			}
		}
		
		sections = append(sections, gin.H{
			"name":        utils.T("cost_of_goods_sold", language),
			"total":       ssotData.COGS.TotalCOGS,
			"subsections": cogsSubsections,
		})
	}

	// 3. GROSS PROFIT Section (calculated)
	sections = append(sections, gin.H{
		"name":          utils.T("gross_profit", language),
		"total":         ssotData.GrossProfit,
		"is_calculated": true,
		"items": []gin.H{
			{
				"name":          utils.T("gross_profit_label", language),
				"amount":        ssotData.GrossProfit,
				"is_percentage": false,
			},
			{
				"name":          utils.T("gross_profit_margin", language),
				"amount":        ssotData.GrossProfitMargin,
				"is_percentage": true,
			},
		},
	})

	// 4. OPERATING EXPENSES Section
	if ssotData.OperatingExpenses.TotalOpEx != 0 {
		opexSubsections := []gin.H{}
		
		if ssotData.OperatingExpenses.Administrative.Subtotal != 0 {
			adminItems := c.deduplicateItems(ssotData.OperatingExpenses.Administrative.Items)
			opexSubsections = append(opexSubsections, gin.H{
				"name":  utils.T("administrative_expenses", language),
				"total": ssotData.OperatingExpenses.Administrative.Subtotal,
				"items": adminItems,
			})
		}
		
		if ssotData.OperatingExpenses.SellingMarketing.Subtotal != 0 {
			sellItems := c.deduplicateItems(ssotData.OperatingExpenses.SellingMarketing.Items)
			opexSubsections = append(opexSubsections, gin.H{
				"name":  utils.T("selling_marketing", language),
				"total": ssotData.OperatingExpenses.SellingMarketing.Subtotal,
				"items": sellItems,
			})
		}
		
		if ssotData.OperatingExpenses.General.Subtotal != 0 {
			genItems := c.deduplicateItems(ssotData.OperatingExpenses.General.Items)
			opexSubsections = append(opexSubsections, gin.H{
				"name":  utils.T("general_expenses", language),
				"total": ssotData.OperatingExpenses.General.Subtotal,
				"items": genItems,
			})
		}
		
		sections = append(sections, gin.H{
			"name":        utils.T("operating_expenses", language),
			"total":       ssotData.OperatingExpenses.TotalOpEx,
			"subsections": opexSubsections,
		})
	}

	// 5. OPERATING INCOME Section (calculated)
	sections = append(sections, gin.H{
		"name":          utils.T("operating_income", language),
		"total":         ssotData.OperatingIncome,
		"is_calculated": true,
		"items": []gin.H{
			{
				"name":          utils.T("operating_income_label", language),
				"amount":        ssotData.OperatingIncome,
				"is_percentage": false,
			},
			{
				"name":          utils.T("operating_margin", language),
				"amount":        ssotData.OperatingMargin,
				"is_percentage": true,
			},
		},
	})

	// 6. OTHER INCOME/EXPENSES Section (if any)
	if ssotData.OtherIncome != 0 || ssotData.OtherExpenses != 0 {
		otherItems := []gin.H{}
		if ssotData.OtherIncome != 0 {
			otherItems = append(otherItems, gin.H{
				"name":   utils.T("other_income", language),
				"amount": ssotData.OtherIncome,
			})
		}
		if ssotData.OtherExpenses != 0 {
			otherItems = append(otherItems, gin.H{
				"name":   utils.T("other_expenses", language),
				"amount": ssotData.OtherExpenses,
			})
		}
		
		sections = append(sections, gin.H{
			"name":  utils.T("other_income_expenses", language),
			"total": ssotData.OtherIncome - ssotData.OtherExpenses,
			"items": otherItems,
		})
	}

	// 7. NET INCOME Section (calculated)
	sections = append(sections, gin.H{
		"name":          utils.T("net_income", language),
		"total":         ssotData.NetIncome,
		"is_calculated": true,
		"items": []gin.H{
			{
				"name":   utils.T("income_before_tax", language),
				"amount": ssotData.IncomeBeforeTax,
			},
			{
				"name":   utils.T("tax_expense", language) + " (25%)",
				"amount": ssotData.TaxExpense,
			},
			{
				"name":          utils.T("net_income_label", language),
				"amount":        ssotData.NetIncome,
				"is_percentage": false,
			},
			{
				"name":          utils.T("net_income_margin", language),
				"amount":        ssotData.NetIncomeMargin,
				"is_percentage": true,
			},
		},
	})

	// Prepare account details for other income and expenses
	otherIncomeItems := []gin.H{}
	otherExpenseItems := []gin.H{}
	
	for _, item := range ssotData.OtherIncomeItems {
		otherIncomeItems = append(otherIncomeItems, gin.H{
			"account_code": item.AccountCode,
			"account_name": item.AccountName,
			"amount":       item.Amount,
		})
	}
	
	for _, item := range ssotData.OtherExpenseItems {
		otherExpenseItems = append(otherExpenseItems, gin.H{
			"account_code": item.AccountCode,
			"account_name": item.AccountName,
			"amount":       item.Amount,
		})
	}

	// Calculate total expenses for summary
	totalExpenses := ssotData.COGS.TotalCOGS + ssotData.OperatingExpenses.TotalOpEx + ssotData.OtherExpenses
	
	// Calculate net profit and net loss (mutually exclusive)
	var netProfit, netLoss float64
	if ssotData.NetIncome > 0 {
		netProfit = ssotData.NetIncome
		netLoss = 0
	} else {
		netProfit = 0
		netLoss = -ssotData.NetIncome
	}
	
	// Create the frontend-compatible response
	return gin.H{
		"title":   utils.T("enhanced_profit_loss", language),
		"period":  ssotData.StartDate.Format("2006-01-02") + " - " + ssotData.EndDate.Format("2006-01-02"),
		"company": gin.H{
			"name":    ssotData.Company.Name,
			"address": ssotData.Company.Address,
			"city":    ssotData.Company.City,
			"phone":   ssotData.Company.Phone,
			"email":   ssotData.Company.Email,
			"period":  ssotData.StartDate.Format("02/01/2006") + " - " + ssotData.EndDate.Format("02/01/2006"),
		},
		"sections": sections,
		"enhanced": ssotData.Enhanced,
		"hasData":  len(sections) > 0,
		"financialMetrics": gin.H{
			"grossProfit":        ssotData.GrossProfit,
			"grossProfitMargin":  ssotData.GrossProfitMargin,
			"operatingIncome":    ssotData.OperatingIncome,
			"operatingMargin":    ssotData.OperatingMargin,
			"ebitda":             ssotData.EBITDA,
			"ebitdaMargin":       ssotData.EBITDAMargin,
			"netIncome":          ssotData.NetIncome,
			"netIncomeMargin":    ssotData.NetIncomeMargin,
		},
		// Summary fields for frontend display
		"total_revenue":  ssotData.Revenue.TotalRevenue,
		"total_expenses": totalExpenses,
		"net_profit":     netProfit,
		"net_loss":       netLoss,
		// Date and metadata fields
		"start_date":      ssotData.StartDate.Format("2006-01-02"),
		"end_date":        ssotData.EndDate.Format("2006-01-02"),
		"generated_at":    ssotData.GeneratedAt.Format(time.RFC3339),
		"account_details": ssotData.AccountDetails,
		"data_source":     ssotData.DataSource,
		"data_source_label": func() string {
			if ssotData.DataSource == "SSOT" {
				return "SSOT Journal System"
			} else if ssotData.DataSource == "LEGACY" {
				return "Legacy Journals"
			}
			return "Accounts Balance Fallback"
		}(),
		"message":         c.generateAnalysisMessage(ssotData),
		// Add account details for other income and expenses
		"other_income_items":  otherIncomeItems,
		"other_expense_items": otherExpenseItems,
	}
}

// deduplicateItems deduplicates items by account code and sums amounts
func (c *SSOTProfitLossController) deduplicateItems(items []services.PLSectionItem) []gin.H {
	itemsByCode := make(map[string]gin.H)
	
	for _, item := range items {
		if existing, found := itemsByCode[item.AccountCode]; found {
			// Account code already exists, sum the amounts
			existingAmount := existing["amount"].(float64)
			itemsByCode[item.AccountCode] = gin.H{
				"name":         existing["name"],
				"amount":       existingAmount + item.Amount,
				"account_code": item.AccountCode,
			}
		} else {
			// First occurrence of this account code
			itemsByCode[item.AccountCode] = gin.H{
				"name":         item.AccountName,
				"amount":       item.Amount,
				"account_code": item.AccountCode,
			}
		}
	}
	
	// Convert map to slice
	var result []gin.H
	for _, item := range itemsByCode {
		result = append(result, item)
	}
	
	return result
}

// filterItemsByPrefix filters items by account code prefix with deduplication
func (c *SSOTProfitLossController) filterItemsByPrefix(items []services.PLSectionItem, prefix string) []gin.H {
	// Use map to deduplicate by account code
	itemsByCode := make(map[string]gin.H)
	
	for _, item := range items {
		if len(item.AccountCode) >= len(prefix) && item.AccountCode[:len(prefix)] == prefix {
			if existing, found := itemsByCode[item.AccountCode]; found {
				// Account code already exists, sum the amounts
				existingAmount := existing["amount"].(float64)
				itemsByCode[item.AccountCode] = gin.H{
					"name":         existing["name"],
					"amount":       existingAmount + item.Amount,
					"account_code": item.AccountCode,
				}
			} else {
				// First occurrence of this account code
				itemsByCode[item.AccountCode] = gin.H{
					"name":         item.AccountName,
					"amount":       item.Amount,
					"account_code": item.AccountCode,
				}
			}
		}
	}
	
	// Convert map to slice
	var filtered []gin.H
	for _, item := range itemsByCode {
		filtered = append(filtered, item)
	}
	
	return filtered
}

// generateAnalysisMessage creates a contextual message based on the P&L data
func (c *SSOTProfitLossController) generateAnalysisMessage(ssotData *services.SSOTProfitLossData) string {
	if ssotData.Revenue.TotalRevenue == 0 {
		if ssotData.COGS.TotalCOGS > 0 || ssotData.OperatingExpenses.TotalOpEx > 0 {
			return "Expenses and costs have been recorded but no revenue transactions were found in journal entries. This may indicate that payment transactions are not yet reflected in the journal system."
		}
		return "No revenue or expense transactions found for this period. The report shows data from SSOT journal system but no income-generating or cost activities were recorded."
	}
	
	if ssotData.COGS.TotalCOGS == 0 && ssotData.OperatingExpenses.TotalOpEx == 0 {
		return "Revenue recorded but no costs or expenses found. This indicates either a service business model or missing cost allocation in the journal entries."
	}
	
	if ssotData.NetIncomeMargin > 50 {
		return "Exceptional profitability detected. Net income margin exceeds 50%, indicating very strong financial performance."
	}
	
	if ssotData.NetIncome < 0 {
		return "The company shows a net loss for this period. Review operating expenses and cost structure for optimization opportunities."
	}
	
	return "P&L statement successfully generated from SSOT journal system with complete financial analysis."
}

// transformToEnhancedFormat transforms SSOT data to the enhanced format expected by the frontend
func (c *SSOTProfitLossController) transformToEnhancedFormat(ssotData *services.SSOTProfitLossData) gin.H {
	// Transform to the enhanced format that the frontend expects
	response := gin.H{
		"company": gin.H{
			"name":    ssotData.Company.Name,
			"address": ssotData.Company.Address,
			"city":    ssotData.Company.City,
			"phone":   ssotData.Company.Phone,
			"email":   ssotData.Company.Email,
		},
		"start_date": ssotData.StartDate.Format("2006-01-02"),
		"end_date":   ssotData.EndDate.Format("2006-01-02"),
		"generated_at": ssotData.GeneratedAt.Format(time.RFC3339),
		
		// Revenue section in the format frontend expects
		"revenue": c.transformRevenueSection(ssotData),
		
		// Cost of goods sold section
		"cost_of_goods_sold": gin.H{
			"total_cogs": ssotData.COGS.TotalCOGS,
			"items": c.transformItemsToFrontendFormat(ssotData.COGS.Items),
		},
		
		// Operating expenses section
		"operating_expenses": gin.H{
			"total_opex": ssotData.OperatingExpenses.TotalOpEx,
			"administrative": gin.H{
				"subtotal": ssotData.OperatingExpenses.Administrative.Subtotal,
				"items": c.transformItemsToFrontendFormat(ssotData.OperatingExpenses.Administrative.Items),
			},
			"selling_marketing": gin.H{
				"subtotal": ssotData.OperatingExpenses.SellingMarketing.Subtotal,
				"items": c.transformItemsToFrontendFormat(ssotData.OperatingExpenses.SellingMarketing.Items),
			},
			"general": gin.H{
				"subtotal": ssotData.OperatingExpenses.General.Subtotal,
				"items": c.transformItemsToFrontendFormat(ssotData.OperatingExpenses.General.Items),
			},
		},
		
		// Financial metrics
		"gross_profit":        ssotData.GrossProfit,
		"gross_profit_margin": ssotData.GrossProfitMargin,
		"operating_income":    ssotData.OperatingIncome,
		"operating_margin":    ssotData.OperatingMargin,
		"ebitda":              ssotData.EBITDA,
		"ebitda_margin":       ssotData.EBITDAMargin,
		"income_before_tax":   ssotData.IncomeBeforeTax,
		"tax_expense":         ssotData.TaxExpense,
		"net_income":          ssotData.NetIncome,
		"net_income_margin":   ssotData.NetIncomeMargin,
	}
	
	return response
}

// transformItemsToFrontendFormat transforms PLSectionItem to frontend format
func (c *SSOTProfitLossController) transformItemsToFrontendFormat(items []services.PLSectionItem) []gin.H {
	var transformed []gin.H
	for _, item := range items {
		transformed = append(transformed, gin.H{
			"code":   item.AccountCode,
			"name":   item.AccountName,
			"amount": item.Amount,
		})
	}
	return transformed
}

// transformRevenueSection transforms revenue data to match frontend expectations
func (c *SSOTProfitLossController) transformRevenueSection(ssotData *services.SSOTProfitLossData) gin.H {
	revenueSection := gin.H{
		"total_revenue": ssotData.Revenue.TotalRevenue,
	}
	
	// Check if we have individual revenue items
	if len(ssotData.Revenue.Items) > 0 {
		// Transform items to the format frontend expects
		revenueItems := []gin.H{}
		for _, item := range ssotData.Revenue.Items {
			revenueItems = append(revenueItems, gin.H{
				"code":   item.AccountCode,
				"name":   item.AccountName,
				"amount": item.Amount,
			})
		}
		revenueSection["items"] = revenueItems
	} else {
		// If no items but has total revenue, create a generic item
		if ssotData.Revenue.TotalRevenue > 0 {
			revenueSection["items"] = []gin.H{
				{
					"code":   "4000",
					"name":   "Total Revenue",
					"amount": ssotData.Revenue.TotalRevenue,
				},
			}
		}
	}
	
	return revenueSection
}
