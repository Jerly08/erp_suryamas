package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"app-sistem-akuntansi/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SSOTReportIntegrationController handles SSOT report integration endpoints
type SSOTReportIntegrationController struct {
	integrationService *services.SSOTReportIntegrationService
	pdfService         services.PDFServiceInterface
	settingsService    *services.SettingsService
	salesExportService *services.SalesReportExportService
}

// NewSSOTReportIntegrationController creates a new SSOT report integration controller
func NewSSOTReportIntegrationController(integrationService *services.SSOTReportIntegrationService, db *gorm.DB) *SSOTReportIntegrationController {
	return &SSOTReportIntegrationController{
		integrationService: integrationService,
		pdfService:         services.NewPDFService(db),
		settingsService:    services.NewSettingsService(db),
		salesExportService: services.NewSalesReportExportService(db),
	}
}

// GetIntegratedFinancialReports generates all financial reports integrated with SSOT journal
// @Summary Get all integrated financial reports
// @Description Generate all financial reports (P&L, Balance Sheet, Cash Flow, etc.) integrated with SSOT journal system
// @Tags SSOT Reports
// @Produce json
// @Param start_date query string true "Start date (YYYY-MM-DD)"
// @Param end_date query string true "End date (YYYY-MM-DD)"
// @Success 200 {object} services.IntegratedFinancialReports
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/ssot-reports/integrated [get]
func (c *SSOTReportIntegrationController) GetIntegratedFinancialReports(ctx *gin.Context) {
	startDateStr := ctx.Query("start_date")
	endDateStr := ctx.Query("end_date")

	if startDateStr == "" || endDateStr == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "start_date and end_date are required",
		})
		return
	}

	startDate, err := c.parseDateBySettings(startDateStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	endDate, err := c.parseDateBySettings(endDateStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// Generate integrated reports
	reports, err := c.integrationService.GenerateIntegratedReports(ctx, startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to generate integrated financial reports",
			"error":   err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   reports,
	})
}

// GetSSOTSalesSummary generates sales summary report integrated with SSOT journal
// @Summary Get sales summary integrated with SSOT journal
// @Description Generate comprehensive sales summary using SSOT journal data
// @Tags SSOT Reports
// @Produce json
// @Param start_date query string true "Start date (YYYY-MM-DD)"
// @Param end_date query string true "End date (YYYY-MM-DD)"
// @Param format query string false "Output format (json, pdf, csv)" default(json)
// @Success 200 {object} services.SalesSummaryData
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/ssot-reports/sales-summary [get]
func (c *SSOTReportIntegrationController) GetSSOTSalesSummary(ctx *gin.Context) {
	startDateStr := ctx.Query("start_date")
	endDateStr := ctx.Query("end_date")
	format := ctx.DefaultQuery("format", "json")

	if startDateStr == "" || endDateStr == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "start_date and end_date are required",
		})
		return
	}

	startDate, err := c.parseDateBySettings(startDateStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	endDate, err := c.parseDateBySettings(endDateStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// Generate sales summary from SSOT
	salesSummary, err := c.integrationService.GenerateSalesSummaryFromSSot(startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to generate sales summary",
			"error":   err.Error(),
		})
		return
	}

	// Handle different formats
	switch format {
	case "json":
		ctx.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data":   salesSummary,
		})
	case "pdf":
		// Generate PDF using Sales Export service (similar to Purchase Report)
		pdfBytes, err := c.salesExportService.ExportToPDF(salesSummary, 0)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to generate Sales Summary PDF",
				"error":   err.Error(),
			})
			return
		}
		ctx.Header("Content-Type", "application/pdf")
		ctx.Header("Content-Disposition", "attachment; filename=SSOT_Sales_Summary.pdf")
		ctx.Header("Content-Length", strconv.Itoa(len(pdfBytes)))
		ctx.Data(http.StatusOK, "application/pdf", pdfBytes)
	case "csv":
		// Follow SSOT P&L style: return JSON metadata for client-side CSV
		meta := gin.H{
			"start_date":    startDate.Format("2006-01-02"),
			"end_date":      endDate.Format("2006-01-02"),
			"data":          salesSummary,
			"export_format": "csv",
			"export_ready":  true,
			"csv_headers":   []string{"Section", "Name", "Value"},
			"report_title":  "SSOT Sales Summary",
		}
		ctx.JSON(http.StatusOK, gin.H{"status": "success", "data": meta, "format": "csv"})
	default:
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Unsupported format. Use json, pdf, or excel",
		})
	}
}

// GetSSOTVendorAnalysis generates vendor analysis report integrated with SSOT journal
// @Summary Get vendor analysis integrated with SSOT journal
// @Description Generate comprehensive vendor analysis using SSOT journal data
// @Tags SSOT Reports
// @Produce json
// @Param start_date query string true "Start date (YYYY-MM-DD)"
// @Param end_date query string true "End date (YYYY-MM-DD)"
// @Param format query string false "Output format (json, pdf)" default(json)
// @Success 200 {object} services.VendorAnalysisData
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/ssot-reports/vendor-analysis [get]
func (c *SSOTReportIntegrationController) GetSSOTVendorAnalysis(ctx *gin.Context) {
	// Deprecated: Use GetSSOTPurchaseReport instead
	ctx.JSON(http.StatusMovedPermanently, gin.H{
		"status":       "deprecated",
		"message":      "This endpoint has been replaced by /api/v1/ssot-reports/purchase-report",
		"redirect_url": "/api/v1/ssot-reports/purchase-report",
	})
}

// GetSSOTPurchaseReport generates purchase report integrated with SSOT journal
// @Summary Get purchase report integrated with SSOT journal
// @Description Generate comprehensive purchase analysis using SSOT journal data
// @Tags SSOT Reports
// @Produce json
// @Param start_date query string true "Start date (YYYY-MM-DD)"
// @Param end_date query string true "End date (YYYY-MM-DD)"
// @Param format query string false "Output format (json, pdf)" default(json)
// @Success 200 {object} services.PurchaseReportData
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/ssot-reports/purchase-report [get]
func (c *SSOTReportIntegrationController) GetSSOTPurchaseReport(ctx *gin.Context) {
	startDateStr := ctx.Query("start_date")
	endDateStr := ctx.Query("end_date")
	format := ctx.DefaultQuery("format", "json")

	if startDateStr == "" || endDateStr == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "start_date and end_date are required",
		})
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid start_date format. Use YYYY-MM-DD",
		})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid end_date format. Use YYYY-MM-DD",
		})
		return
	}

	// Generate purchase report from SSOT
	purchaseService := services.NewSSOTPurchaseReportService(c.integrationService.GetDB())
	purchaseReport, err := purchaseService.GeneratePurchaseReport(ctx, startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to generate purchase report",
			"error":   err.Error(),
		})
		return
	}

	// Handle different formats
	switch format {
	case "json":
		ctx.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data":   purchaseReport,
		})
	case "pdf":
		// TODO: Implement PDF export
		ctx.JSON(http.StatusNotImplemented, gin.H{
			"status":  "error",
			"message": "PDF export not yet implemented for SSOT purchase report",
		})
	default:
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Unsupported format. Use json or pdf",
		})
	}

	// Generate vendor analysis from SSOT
	vendorData, err := c.integrationService.GenerateVendorAnalysisFromSSot(startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to generate vendor analysis",
			"error":   err.Error(),
		})
		return
	}

	// Handle different formats
	switch format {
	case "json":
		ctx.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data":   vendorData,
		})
	case "pdf":
		// TODO: Implement PDF export
		ctx.JSON(http.StatusNotImplemented, gin.H{
			"status":  "error",
			"message": "PDF export not yet implemented for SSOT vendor analysis",
		})
	default:
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Unsupported format. Use json or pdf",
		})
	}
}

// GetSSOTTrialBalance generates trial balance integrated with SSOT journal
// @Summary Get trial balance integrated with SSOT journal
// @Description Generate trial balance using SSOT journal data
// @Tags SSOT Reports
// @Produce json
// @Param as_of_date query string false "As of date (YYYY-MM-DD)" default(today)
// @Param format query string false "Output format (json, pdf, csv)" default(json)
// @Success 200 {object} services.TrialBalanceData
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/ssot-reports/trial-balance [get]
func (c *SSOTReportIntegrationController) GetSSOTTrialBalance(ctx *gin.Context) {
	asOfDateStr := ctx.Query("as_of_date")
	format := ctx.DefaultQuery("format", "json")

	var asOfDate time.Time
	var err error
	if asOfDateStr == "" {
		asOfDate = time.Now()
	} else {
		asOfDate, err = time.Parse("2006-01-02", asOfDateStr)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Invalid as_of_date format. Use YYYY-MM-DD",
			})
			return
		}
	}

	// Generate trial balance from SSOT
	trialBalance, err := c.integrationService.GenerateTrialBalanceFromSSot(asOfDate)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to generate trial balance",
			"error":   err.Error(),
		})
		return
	}

	// Handle different formats
	switch format {
	case "json":
		ctx.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data":   trialBalance,
		})
	case "pdf":
		pdfBytes, err := c.pdfService.GenerateTrialBalancePDF(trialBalance, asOfDate.Format("2006-01-02"))
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to generate PDF", "error": err.Error()})
			return
		}
		ctx.Header("Content-Type", "application/pdf")
		ctx.Header("Content-Disposition", "attachment; filename=SSOT_Trial_Balance.pdf")
		ctx.Header("Content-Length", strconv.Itoa(len(pdfBytes)))
		ctx.Data(http.StatusOK, "application/pdf", pdfBytes)
	case "csv":
		meta := gin.H{
			"as_of_date":    asOfDate.Format("2006-01-02"),
			"data":          trialBalance,
			"export_ready":  true,
			"export_format": "csv",
			"csv_headers":   []string{"Account Code", "Account Name", "Debit Balance", "Credit Balance"},
			"report_title":  "SSOT Trial Balance",
		}
		ctx.JSON(http.StatusOK, gin.H{"status": "success", "data": meta, "format": "csv"})
	default:
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Unsupported format. Use json, pdf, or excel",
		})
	}
}

// GetSSOTGeneralLedger generates general ledger integrated with SSOT journal
// @Summary Get general ledger integrated with SSOT journal
// @Description Generate general ledger using SSOT journal data
// @Tags SSOT Reports
// @Produce json
// @Param account_id query string false "Account ID (specific account or 'all' for all accounts)"
// @Param start_date query string true "Start date (YYYY-MM-DD)"
// @Param end_date query string true "End date (YYYY-MM-DD)"
// @Param format query string false "Output format (json, pdf, csv)" default(json)
// @Success 200 {object} services.GeneralLedgerData
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/ssot-reports/general-ledger [get]
func (c *SSOTReportIntegrationController) GetSSOTGeneralLedger(ctx *gin.Context) {
	accountIDStr := ctx.Query("account_id")
	startDateStr := ctx.Query("start_date")
	endDateStr := ctx.Query("end_date")
	format := ctx.DefaultQuery("format", "json")

	if startDateStr == "" || endDateStr == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "start_date and end_date are required",
		})
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid start_date format. Use YYYY-MM-DD",
		})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid end_date format. Use YYYY-MM-DD",
		})
		return
	}

	// Handle account ID parameter
	var accountID *uint64
	if accountIDStr != "" && accountIDStr != "all" {
		id, err := strconv.ParseUint(accountIDStr, 10, 64)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Invalid account_id format. Use numeric ID or 'all'",
			})
			return
		}
		accountID = &id
	}

	// Generate general ledger from SSOT
	generalLedger, err := c.integrationService.GenerateGeneralLedgerFromSSot(startDate, endDate, accountID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to generate general ledger",
			"error":   err.Error(),
		})
		return
	}

	// Handle different formats
	switch format {
	case "json":
		ctx.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data":   generalLedger,
		})
	case "pdf":
		pdfBytes, err := c.pdfService.GenerateGeneralLedgerPDF(generalLedger, "All Accounts", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to generate PDF", "error": err.Error()})
			return
		}
		ctx.Header("Content-Type", "application/pdf")
		ctx.Header("Content-Disposition", "attachment; filename=SSOT_General_Ledger.pdf")
		ctx.Header("Content-Length", strconv.Itoa(len(pdfBytes)))
		ctx.Data(http.StatusOK, "application/pdf", pdfBytes)
	case "csv":
		meta := gin.H{
			"start_date":    startDate.Format("2006-01-02"),
			"end_date":      endDate.Format("2006-01-02"),
			"data":          generalLedger,
			"export_ready":  true,
			"export_format": "csv",
			"csv_headers":   []string{"Date", "Account Code", "Account Name", "Description", "Debit", "Credit", "Balance"},
			"report_title":  "SSOT General Ledger",
		}
		ctx.JSON(http.StatusOK, gin.H{"status": "success", "data": meta, "format": "csv"})
	default:
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Unsupported format. Use json, pdf, or excel",
		})
	}
}

// GetSSOTJournalAnalysis generates journal analysis integrated with SSOT journal
// @Summary Get journal entry analysis integrated with SSOT
// @Description Generate comprehensive journal entry analysis using SSOT data
// @Tags SSOT Reports
// @Produce json
// @Param start_date query string true "Start date (YYYY-MM-DD)"
// @Param end_date query string true "End date (YYYY-MM-DD)"
// @Param format query string false "Output format (json, pdf, csv)" default(json)
// @Success 200 {object} services.JournalAnalysisData
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/ssot-reports/journal-analysis [get]
func (c *SSOTReportIntegrationController) GetSSOTJournalAnalysis(ctx *gin.Context) {
	startDateStr := ctx.Query("start_date")
	endDateStr := ctx.Query("end_date")
	format := ctx.DefaultQuery("format", "json")

	if startDateStr == "" || endDateStr == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "start_date and end_date are required",
		})
		return
	}

	startDate, err := c.parseDateBySettings(startDateStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	endDate, err := c.parseDateBySettings(endDateStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	// Generate journal analysis from SSOT
	journalAnalysis, err := c.integrationService.GenerateJournalAnalysisFromSSot(startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to generate journal analysis",
			"error":   err.Error(),
		})
		return
	}

	// Handle different formats
	switch format {
	case "json":
		ctx.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data":   journalAnalysis,
		})
	case "pdf":
		pdfBytes, err := c.pdfService.GenerateJournalAnalysisPDF(journalAnalysis, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to generate PDF", "error": err.Error()})
			return
		}
		ctx.Header("Content-Type", "application/pdf")
		ctx.Header("Content-Disposition", "attachment; filename=SSOT_Journal_Analysis.pdf")
		ctx.Header("Content-Length", strconv.Itoa(len(pdfBytes)))
		ctx.Data(http.StatusOK, "application/pdf", pdfBytes)
	case "csv":
		meta := gin.H{
			"start_date":    startDate.Format("2006-01-02"),
			"end_date":      endDate.Format("2006-01-02"),
			"data":          journalAnalysis,
			"export_ready":  true,
			"export_format": "csv",
			"csv_headers":   []string{"Journal ID", "Date", "Account", "Debit", "Credit", "Description"},
			"report_title":  "SSOT Journal Entry Analysis",
		}
		ctx.JSON(http.StatusOK, gin.H{"status": "success", "data": meta, "format": "csv"})
	default:
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Unsupported format. Use json or pdf",
		})
	}
}

// parseDateBySettings parses a date string according to the system DateFormat setting, with safe fallbacks to common formats.
func (c *SSOTReportIntegrationController) parseDateBySettings(s string) (time.Time, error) {
	// Default layouts to try
	layouts := []string{"2006-01-02", "02/01/2006", "01/02/2006", "02-01-2006", time.RFC3339}

	// If settings available, put the configured layout first
	if c.settingsService != nil {
		if st, err := c.settingsService.GetSettings(); err == nil {
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
	return time.Time{}, fmt.Errorf("invalid date format: %s (expected formats include YYYY-MM-DD or configured system date format)", s)
}

// GetSSOTReportStatus provides status and health information about SSOT report integration
// @Summary Get SSOT report integration status
// @Description Get status information about SSOT journal integration and report generation
// @Tags SSOT Reports
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /api/v1/ssot-reports/status [get]
func (c *SSOTReportIntegrationController) GetSSOTReportStatus(ctx *gin.Context) {
	// This would typically include health checks, statistics, etc.
	status := gin.H{
		"ssot_integration": gin.H{
			"status":      "active",
			"version":     "1.0",
			"last_update": time.Now(),
		},
		"available_reports": []string{
			"profit_loss",
			"balance_sheet",
			"cash_flow",
			"sales_summary",
			"vendor_analysis",
			"trial_balance",
			"general_ledger",
			"journal_analysis",
		},
		"features": gin.H{
			"concurrent_generation": true,
			"data_integrity_check":  true,
			"audit_trail":           true,
		},
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   status,
	})
}

// TriggerReportRefresh triggers manual refresh of reports and broadcasts via websocket
// @Summary Trigger manual report refresh
// @Description Manually trigger report refresh and broadcast updates via websocket
// @Tags SSOT Reports
// @Accept json
// @Produce json
// @Param refresh_request body map[string]interface{} false "Refresh request parameters"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/ssot-reports/refresh [post]
func (c *SSOTReportIntegrationController) TriggerReportRefresh(ctx *gin.Context) {
	var refreshRequest map[string]interface{}
	if err := ctx.ShouldBindJSON(&refreshRequest); err != nil {
		// Use default refresh if no body provided
		refreshRequest = make(map[string]interface{})
	}

	// Trigger balance refresh (WebSocket removed for stability)

	// Trigger report refresh notification
	c.integrationService.OnJournalPosted(0) // General refresh without specific journal ID

	ctx.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Report refresh triggered successfully",
		"data": gin.H{
			"triggered_at": time.Now(),
			"refresh_type": "manual",
		},
	})
}
