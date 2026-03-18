package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SSOTBalanceSheetController handles Balance Sheet reports from SSOT Journal System
type SSOTBalanceSheetController struct {
	ssotBalanceSheetService *services.SSOTBalanceSheetService
	periodArchiveService    *services.PeriodArchiveService
	pdfService              services.PDFServiceInterface
	settingsService         *services.SettingsService
	db                      *gorm.DB
}

// NewSSOTBalanceSheetController creates a new SSOT Balance Sheet controller
func NewSSOTBalanceSheetController(db *gorm.DB) *SSOTBalanceSheetController {
	return &SSOTBalanceSheetController{
		ssotBalanceSheetService: services.NewSSOTBalanceSheetService(db),
		periodArchiveService:    services.NewPeriodArchiveService(db),
		pdfService:              services.NewPDFService(db),
		settingsService:         services.NewSettingsService(db),
		db:                      db,
	}
}

// GenerateSSOTBalanceSheet generates a comprehensive Balance Sheet from SSOT journal system
// @Summary Generate SSOT Balance Sheet
// @Description Generate a comprehensive Balance Sheet using Single Source of Truth (SSOT) journal system with real-time data integration and automatic balance validation
// @Tags SSOT Reports
// @Accept json
// @Produce json
// @Param as_of_date query string false "As of date (YYYY-MM-DD)" example(2025-12-31)
// @Param format query string false "Output format" Enums(json,summary,pdf,csv) default(json)
// @Success 200 {object} map[string]interface{} "Balance Sheet generated successfully"
// @Failure 400 {object} map[string]interface{} "Invalid request parameters"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /reports/ssot/balance-sheet [get]
func (ctrl *SSOTBalanceSheetController) GenerateSSOTBalanceSheet(c *gin.Context) {
	// Get parameters
	asOfDate := c.DefaultQuery("as_of_date", time.Now().Format("2006-01-02"))
	format := c.DefaultQuery("format", "json")

	// Log request for debugging
	fmt.Printf("GenerateSSOTBalanceSheet called with asOfDate: %s, format: %s\n", asOfDate, format)
	fmt.Printf("Request URL: %s\n", c.Request.URL.String())
	fmt.Printf("Request headers: %v\n", c.Request.Header)

	// Log additional debugging information
	fmt.Printf("User-Agent: %s\n", c.GetHeader("User-Agent"))
	fmt.Printf("Referer: %s\n", c.GetHeader("Referer"))
	fmt.Printf("Origin: %s\n", c.GetHeader("Origin"))
	fmt.Printf("Remote IP: %s\n", c.ClientIP())

	// Validate date format using system settings (supports DD/MM/YYYY, MM/DD/YYYY, etc.)
	if _, err := ctrl.parseDateBySettings(asOfDate); err != nil {
		fmt.Printf("Date validation error: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid as_of_date format",
			"message": err.Error(),
			"example": "2024-12-31",
		})
		return
	}

	// Generate Balance Sheet
	balanceSheetData, err := ctrl.ssotBalanceSheetService.GenerateSSOTBalanceSheet(asOfDate)
	if err != nil {
		fmt.Printf("Balance sheet generation error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to generate SSOT Balance Sheet",
			"message": err.Error(),
		})
		return
	}

	// Handle different output formats
	switch format {
	case "json":
		fmt.Println("Returning JSON format")
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "SSOT Balance Sheet generated successfully",
			"data":    balanceSheetData,
		})
	case "summary":
		fmt.Println("Returning summary format")
		// Return a simplified summary view
		summary := createBalanceSheetSummary(balanceSheetData)
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "SSOT Balance Sheet summary generated successfully",
			"data":    summary,
		})
	case "pdf":
		fmt.Println("Generating PDF format")
		pdfBytes, err := ctrl.pdfService.GenerateBalanceSheetPDF(balanceSheetData, asOfDate)
		if err != nil {
			fmt.Printf("PDF generation error: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to generate Balance Sheet PDF",
				"error":   err.Error(),
			})
			return
		}
		filename := fmt.Sprintf("SSOT_BalanceSheet_%s.pdf", asOfDate)
		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		c.Header("Content-Length", strconv.Itoa(len(pdfBytes)))
		c.Data(http.StatusOK, "application/pdf", pdfBytes)
	case "csv":
		fmt.Println("Generating CSV format")
		// Generate actual CSV content instead of JSON metadata
		// Create CSV content from balance sheet data
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("CSV generation panic: %v\n", r)
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":  "error",
					"message": "Failed to generate Balance Sheet CSV due to internal error",
					"error":   fmt.Sprintf("Panic occurred: %v", r),
				})
			}
		}()

		csvContent := ctrl.generateCSVContent(balanceSheetData, asOfDate)
		fmt.Printf("Generated CSV content length: %d\n", len(csvContent))

		// Set appropriate headers for CSV download
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=SSOT_BalanceSheet_%s.csv", asOfDate))
		c.Data(http.StatusOK, "text/csv", []byte(csvContent))

	default:
		fmt.Printf("Invalid format requested: %s\n", format)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid format",
			"message": "Supported formats: json, summary, pdf, csv",
		})
	}
}

// GetArchivedSSOTBalanceSheet returns a balance sheet for a closed period.
// It prefers a stored snapshot when available and falls back to live generation
// at the period end date to keep archive mode usable for older closings.
func (ctrl *SSOTBalanceSheetController) GetArchivedSSOTBalanceSheet(c *gin.Context) {
	periodIDStr := strings.TrimSpace(c.Query("period_id"))
	endDateStr := strings.TrimSpace(c.Query("end_date"))

	if periodIDStr == "" && endDateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "period_id or end_date is required",
			"message": "Provide a closed accounting period to open archived balance sheet mode",
		})
		return
	}

	var period models.AccountingPeriod
	query := ctrl.db.Where("is_closed = ? AND deleted_at IS NULL", true)

	if periodIDStr != "" {
		periodID, err := strconv.ParseUint(periodIDStr, 10, 64)
		if err != nil || periodID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid period_id",
				"message": "period_id must be a positive integer",
			})
			return
		}
		query = query.Where("id = ?", periodID)
	} else {
		endDate, err := ctrl.parseDateBySettings(endDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid end_date format",
				"message": err.Error(),
			})
			return
		}
		query = query.Where("DATE(end_date) = ?", endDate.Format("2006-01-02"))
	}

	if err := query.Order("end_date DESC").First(&period).Error; err != nil {
		status := http.StatusInternalServerError
		message := err.Error()
		if err == gorm.ErrRecordNotFound {
			status = http.StatusNotFound
			message = "Closed accounting period not found"
		}
		c.JSON(status, gin.H{
			"error":   "Failed to load archived balance sheet",
			"message": message,
		})
		return
	}

	detail, err := ctrl.periodArchiveService.GetPeriodArchiveDetail(c.Request.Context(), period.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to read archived period detail",
			"message": err.Error(),
		})
		return
	}

	balanceSheetData := detail.BalanceSheet
	snapshotAvailable := balanceSheetData != nil
	if balanceSheetData == nil {
		balanceSheetData, err = ctrl.ssotBalanceSheetService.GenerateSSOTBalanceSheet(period.EndDate.Format("2006-01-02"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to generate archived balance sheet",
				"message": err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Archived SSOT Balance Sheet generated successfully",
		"data":    balanceSheetData,
		"archive": gin.H{
			"period_id":           period.ID,
			"start_date":          period.StartDate.Format("2006-01-02"),
			"end_date":            period.EndDate.Format("2006-01-02"),
			"description":         period.Description,
			"period_type":         period.PeriodType,
			"is_closed":           period.IsClosed,
			"closed_at":           period.ClosedAt,
			"snapshot_available":  snapshotAvailable,
			"snapshot_generated":  period.SnapshotGeneratedAt,
			"calculation_basis":   "period_end_date",
			"report_display_mode": "closed_period_archive",
		},
	})
}

// GetSSOTBalanceSheetAccountDetails provides detailed account information for drilldown
// @Summary Get SSOT Balance Sheet Account Details
// @Description Get detailed account information for Balance Sheet drilldown analysis
// @Tags SSOT Reports
// @Accept json
// @Produce json
// @Param as_of_date query string false "As of date (YYYY-MM-DD)" example(2025-12-31)
// @Param account_type query string false "Filter by account type" Enums(ASSET,LIABILITY,EQUITY)
// @Success 200 {object} map[string]interface{} "Account details retrieved successfully"
// @Failure 400 {object} map[string]interface{} "Invalid request parameters"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security BearerAuth
// @Router /reports/ssot/balance-sheet/account-details [get]
func (ctrl *SSOTBalanceSheetController) GetSSOTBalanceSheetAccountDetails(c *gin.Context) {
	asOfDate := c.DefaultQuery("as_of_date", time.Now().Format("2006-01-02"))
	accountType := c.Query("account_type") // ASSET, LIABILITY, or EQUITY

	// Validate parameters using system date format
	if _, err := ctrl.parseDateBySettings(asOfDate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid as_of_date format",
			"message": err.Error(),
		})
		return
	}

	if accountType != "" && accountType != "ASSET" && accountType != "LIABILITY" && accountType != "EQUITY" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid account_type. Must be ASSET, LIABILITY, or EQUITY",
		})
		return
	}

	// Generate full balance sheet to get account details
	balanceSheetData, err := ctrl.ssotBalanceSheetService.GenerateSSOTBalanceSheet(asOfDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get Balance Sheet account details",
			"message": err.Error(),
		})
		return
	}

	// Filter account details by type if specified
	accountDetails := balanceSheetData.AccountDetails
	if accountType != "" {
		filteredDetails := []services.SSOTAccountBalance{}
		for _, detail := range accountDetails {
			if detail.AccountType == accountType {
				filteredDetails = append(filteredDetails, detail)
			}
		}
		accountDetails = filteredDetails
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Balance Sheet account details retrieved successfully",
		"data": map[string]interface{}{
			"as_of_date":      asOfDate,
			"account_type":    accountType,
			"account_details": accountDetails,
			"total_accounts":  len(accountDetails),
		},
	})
}

// ValidateSSOTBalanceSheet validates if the balance sheet balances correctly
func (ctrl *SSOTBalanceSheetController) ValidateSSOTBalanceSheet(c *gin.Context) {
	asOfDate := c.DefaultQuery("as_of_date", time.Now().Format("2006-01-02"))

	// Validate date format using system settings
	if _, err := ctrl.parseDateBySettings(asOfDate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid as_of_date format",
			"message": err.Error(),
		})
		return
	}

	// Generate Balance Sheet
	balanceSheetData, err := ctrl.ssotBalanceSheetService.GenerateSSOTBalanceSheet(asOfDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to validate Balance Sheet",
			"message": err.Error(),
		})
		return
	}

	// Create validation result
	validationResult := map[string]interface{}{
		"as_of_date":                   asOfDate,
		"is_balanced":                  balanceSheetData.IsBalanced,
		"total_assets":                 balanceSheetData.Assets.TotalAssets,
		"total_liabilities_and_equity": balanceSheetData.TotalLiabilitiesAndEquity,
		"balance_difference":           balanceSheetData.BalanceDifference,
		"tolerance":                    0.01,
		"validation_status":            "PASS",
		"generated_at":                 balanceSheetData.GeneratedAt,
	}

	if !balanceSheetData.IsBalanced {
		validationResult["validation_status"] = "FAIL"
		validationResult["issue"] = "Assets do not equal Liabilities + Equity"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Balance Sheet validation completed",
		"data":    validationResult,
	})
}

// GetSSOTBalanceSheetComparison compares balance sheets between two dates
func (ctrl *SSOTBalanceSheetController) GetSSOTBalanceSheetComparison(c *gin.Context) {
	fromDate := c.Query("from_date")
	toDate := c.Query("to_date")

	// Default to comparing current date with 1 year ago
	if fromDate == "" {
		fromDate = time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
	}
	if toDate == "" {
		toDate = time.Now().Format("2006-01-02")
	}

	// Validate date formats
	if _, err := time.Parse("2006-01-02", fromDate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid from_date format",
		})
		return
	}
	if _, err := time.Parse("2006-01-02", toDate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid to_date format",
		})
		return
	}

	// Generate both balance sheets
	fromBS, err := ctrl.ssotBalanceSheetService.GenerateSSOTBalanceSheet(fromDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to generate from_date Balance Sheet",
			"message": err.Error(),
		})
		return
	}

	toBS, err := ctrl.ssotBalanceSheetService.GenerateSSOTBalanceSheet(toDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to generate to_date Balance Sheet",
			"message": err.Error(),
		})
		return
	}

	// Create comparison
	comparison := map[string]interface{}{
		"from_date": fromDate,
		"to_date":   toDate,
		"comparison": map[string]interface{}{
			"total_assets": map[string]interface{}{
				"from":           fromBS.Assets.TotalAssets,
				"to":             toBS.Assets.TotalAssets,
				"change":         toBS.Assets.TotalAssets - fromBS.Assets.TotalAssets,
				"change_percent": calculatePercentChange(fromBS.Assets.TotalAssets, toBS.Assets.TotalAssets),
			},
			"total_liabilities": map[string]interface{}{
				"from":           fromBS.Liabilities.TotalLiabilities,
				"to":             toBS.Liabilities.TotalLiabilities,
				"change":         toBS.Liabilities.TotalLiabilities - fromBS.Liabilities.TotalLiabilities,
				"change_percent": calculatePercentChange(fromBS.Liabilities.TotalLiabilities, toBS.Liabilities.TotalLiabilities),
			},
			"total_equity": map[string]interface{}{
				"from":           fromBS.Equity.TotalEquity,
				"to":             toBS.Equity.TotalEquity,
				"change":         toBS.Equity.TotalEquity - fromBS.Equity.TotalEquity,
				"change_percent": calculatePercentChange(fromBS.Equity.TotalEquity, toBS.Equity.TotalEquity),
			},
		},
		"balance_sheet_from": fromBS,
		"balance_sheet_to":   toBS,
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Balance Sheet comparison completed",
		"data":    comparison,
	})
}

// Helper function to create a summary view of the balance sheet
func createBalanceSheetSummary(bs *services.SSOTBalanceSheetData) map[string]interface{} {
	return map[string]interface{}{
		"company":    bs.Company,
		"as_of_date": bs.AsOfDate.Format("2006-01-02"),
		"currency":   bs.Currency,
		"assets": map[string]interface{}{
			"current_assets":     bs.Assets.CurrentAssets.TotalCurrentAssets,
			"non_current_assets": bs.Assets.NonCurrentAssets.TotalNonCurrentAssets,
			"total_assets":       bs.Assets.TotalAssets,
		},
		"liabilities": map[string]interface{}{
			"current_liabilities":     bs.Liabilities.CurrentLiabilities.TotalCurrentLiabilities,
			"non_current_liabilities": bs.Liabilities.NonCurrentLiabilities.TotalNonCurrentLiabilities,
			"total_liabilities":       bs.Liabilities.TotalLiabilities,
		},
		"equity": map[string]interface{}{
			"total_equity": bs.Equity.TotalEquity,
		},
		"balance_check": map[string]interface{}{
			"is_balanced":        bs.IsBalanced,
			"balance_difference": bs.BalanceDifference,
		},
		"generated_at": bs.GeneratedAt,
		"enhanced":     bs.Enhanced,
	}
}

// parseDateBySettings parses a date string according to Settings.DateFormat with safe fallbacks.
func (ctrl *SSOTBalanceSheetController) parseDateBySettings(s string) (time.Time, error) {
	layouts := []string{"2006-01-02", "02/01/2006", "01/02/2006", "02-01-2006", time.RFC3339}
	if ctrl.settingsService != nil {
		if st, err := ctrl.settingsService.GetSettings(); err == nil {
			switch st.DateFormat {
			case "DD/MM/YYYY":
				layouts = append([]string{"02/01/2006"}, layouts...)
			case "MM/DD/YYYY":
				layouts = append([]string{"01/02/2006"}, layouts...)
			case "YYYY-MM-DD":
				layouts = append([]string{"2006-01-02"}, layouts...)
			case "DD-MM-YYYY":
				layouts = append([]string{"02-01-2006"}, layouts...)
			}
		}
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date: %s", s)
}

// Helper function to calculate percentage change
func calculatePercentChange(from, to float64) float64 {
	if from == 0 {
		if to == 0 {
			return 0
		}
		return 100 // Arbitrary large percentage for new items
	}
	return ((to - from) / from) * 100
}

// generateCSVContent creates CSV content from balance sheet data
func (ctrl *SSOTBalanceSheetController) generateCSVContent(data *services.SSOTBalanceSheetData, asOfDate string) string {
	var csvLines []string

	// Helper function to escape CSV values
	escapeCSV := func(value string) string {
		if strings.Contains(value, ",") || strings.Contains(value, "\"") || strings.Contains(value, "\n") {
			return "\"" + strings.ReplaceAll(value, "\"", "\"\"") + "\""
		}
		return value
	}

	// Helper function to format currency
	formatCurrency := func(amount float64) string {
		return fmt.Sprintf("%.0f", amount)
	}

	// Header section
	csvLines = append(csvLines, escapeCSV(data.Company.Name))
	csvLines = append(csvLines, "BALANCE SHEET")
	csvLines = append(csvLines, fmt.Sprintf("As of: %s", asOfDate))
	csvLines = append(csvLines, fmt.Sprintf("Generated on: %s", time.Now().Format("2006-01-02 15:04:05")))
	csvLines = append(csvLines, "") // Empty line

	// Summary section
	csvLines = append(csvLines, "FINANCIAL SUMMARY")
	csvLines = append(csvLines, "Category,Amount")
	csvLines = append(csvLines, fmt.Sprintf("Total Assets,%s", formatCurrency(data.Assets.TotalAssets)))
	csvLines = append(csvLines, fmt.Sprintf("Total Liabilities,%s", formatCurrency(data.Liabilities.TotalLiabilities)))
	csvLines = append(csvLines, fmt.Sprintf("Total Equity,%s", formatCurrency(data.Equity.TotalEquity)))
	csvLines = append(csvLines, fmt.Sprintf("Total Liabilities + Equity,%s", formatCurrency(data.TotalLiabilitiesAndEquity)))
	csvLines = append(csvLines, fmt.Sprintf("Balanced,%s", map[bool]string{true: "Yes", false: "No"}[data.IsBalanced]))
	if !data.IsBalanced && data.BalanceDifference != 0 {
		csvLines = append(csvLines, fmt.Sprintf("Balance Difference,%s", formatCurrency(data.BalanceDifference)))
	}
	csvLines = append(csvLines, "") // Empty line

	// Detailed account breakdown
	csvLines = append(csvLines, "DETAILED BREAKDOWN")
	csvLines = append(csvLines, "Account Code,Account Name,Category,Amount")

	// Assets section
	csvLines = append(csvLines, "ASSETS,,,")

	// Current Assets
	if len(data.Assets.CurrentAssets.Items) > 0 {
		csvLines = append(csvLines, "Current Assets,,,")

		for _, item := range data.Assets.CurrentAssets.Items {
			csvLines = append(csvLines, fmt.Sprintf("%s,%s,Current Asset,%s",
				escapeCSV(item.AccountCode),
				escapeCSV(item.AccountName),
				formatCurrency(item.Amount)))
		}
		csvLines = append(csvLines, fmt.Sprintf("Subtotal Current Assets,,,%s", formatCurrency(data.Assets.CurrentAssets.TotalCurrentAssets)))
		csvLines = append(csvLines, "")
	}

	// Non-Current Assets
	if len(data.Assets.NonCurrentAssets.Items) > 0 {
		csvLines = append(csvLines, "Non-Current Assets,,,")

		for _, item := range data.Assets.NonCurrentAssets.Items {
			csvLines = append(csvLines, fmt.Sprintf("%s,%s,Non-Current Asset,%s",
				escapeCSV(item.AccountCode),
				escapeCSV(item.AccountName),
				formatCurrency(item.Amount)))
		}
		csvLines = append(csvLines, fmt.Sprintf("Subtotal Non-Current Assets,,,%s", formatCurrency(data.Assets.NonCurrentAssets.TotalNonCurrentAssets)))
		csvLines = append(csvLines, "")
	}

	csvLines = append(csvLines, fmt.Sprintf("TOTAL ASSETS,,,%s", formatCurrency(data.Assets.TotalAssets)))
	csvLines = append(csvLines, "") // Empty line

	// Liabilities section
	csvLines = append(csvLines, "LIABILITIES,,,")

	// Current Liabilities
	if len(data.Liabilities.CurrentLiabilities.Items) > 0 {
		csvLines = append(csvLines, "Current Liabilities,,,")

		for _, item := range data.Liabilities.CurrentLiabilities.Items {
			csvLines = append(csvLines, fmt.Sprintf("%s,%s,Current Liability,%s",
				escapeCSV(item.AccountCode),
				escapeCSV(item.AccountName),
				formatCurrency(item.Amount)))
		}
		csvLines = append(csvLines, fmt.Sprintf("Subtotal Current Liabilities,,,%s", formatCurrency(data.Liabilities.CurrentLiabilities.TotalCurrentLiabilities)))
		csvLines = append(csvLines, "")
	}

	// Non-Current Liabilities
	if len(data.Liabilities.NonCurrentLiabilities.Items) > 0 {
		csvLines = append(csvLines, "Non-Current Liabilities,,,")

		for _, item := range data.Liabilities.NonCurrentLiabilities.Items {
			csvLines = append(csvLines, fmt.Sprintf("%s,%s,Non-Current Liability,%s",
				escapeCSV(item.AccountCode),
				escapeCSV(item.AccountName),
				formatCurrency(item.Amount)))
		}
		csvLines = append(csvLines, fmt.Sprintf("Subtotal Non-Current Liabilities,,,%s", formatCurrency(data.Liabilities.NonCurrentLiabilities.TotalNonCurrentLiabilities)))
		csvLines = append(csvLines, "")
	}

	csvLines = append(csvLines, fmt.Sprintf("TOTAL LIABILITIES,,,%s", formatCurrency(data.Liabilities.TotalLiabilities)))
	csvLines = append(csvLines, "") // Empty line

	// Equity section
	if len(data.Equity.Items) > 0 {
		csvLines = append(csvLines, "EQUITY,,,")

		for _, item := range data.Equity.Items {
			csvLines = append(csvLines, fmt.Sprintf("%s,%s,Equity,%s",
				escapeCSV(item.AccountCode),
				escapeCSV(item.AccountName),
				formatCurrency(item.Amount)))
		}
		csvLines = append(csvLines, "")
	}

	csvLines = append(csvLines, fmt.Sprintf("TOTAL EQUITY,,,%s", formatCurrency(data.Equity.TotalEquity)))
	csvLines = append(csvLines, "") // Empty line
	csvLines = append(csvLines, fmt.Sprintf("TOTAL LIABILITIES + EQUITY,,,%s", formatCurrency(data.TotalLiabilitiesAndEquity)))

	// Footer
	csvLines = append(csvLines, "")
	csvLines = append(csvLines, "Generated by Sistem Akuntansi")
	csvLines = append(csvLines, fmt.Sprintf("Report Date: %s", time.Now().Format("2006-01-02 15:04:05")))

	// Safer way to handle the data source mapping
	dataSource := "SSOT Standard"
	if data.Enhanced {
		dataSource = "SSOT Enhanced"
	}
	csvLines = append(csvLines, fmt.Sprintf("Data Source: %s", dataSource))

	return strings.Join(csvLines, "\n")
}
