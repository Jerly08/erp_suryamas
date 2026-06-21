package controllers

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"app-sistem-akuntansi/services"
	"app-sistem-akuntansi/utils"

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
	endDate = normalizeInclusiveEndDate(endDateStr, endDate)

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
// @Param project_id query int false "Filter by project ID"
// @Param format query string false "Output format (json, pdf, csv)" default(json)
// @Success 200 {object} services.SalesSummaryData
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/ssot-reports/sales-summary [get]
func (c *SSOTReportIntegrationController) GetSSOTSalesSummary(ctx *gin.Context) {
	startDateStr := ctx.Query("start_date")
	endDateStr := ctx.Query("end_date")
	projectIDStr := ctx.Query("project_id")
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
	endDate = normalizeInclusiveEndDate(endDateStr, endDate)

	var projectID *uint
	if projectIDStr != "" {
		parsedProjectID, err := strconv.ParseUint(projectIDStr, 10, 64)
		if err != nil || parsedProjectID == 0 {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "project_id must be a valid positive integer",
			})
			return
		}

		projectIDValue := uint(parsedProjectID)
		projectID = &projectIDValue
	}

	// Generate sales summary from SSOT
	salesSummary, err := c.integrationService.GenerateSalesSummaryFromSSot(startDate, endDate, projectID)
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
		csvBytes, err := c.salesExportService.ExportToCSV(salesSummary, 0)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to generate Sales Summary CSV",
				"error":   err.Error(),
			})
			return
		}
		ctx.Header("Content-Type", "text/csv; charset=utf-8")
		ctx.Header("Content-Disposition", "attachment; filename=SSOT_Sales_Summary.csv")
		ctx.Header("Content-Length", strconv.Itoa(len(csvBytes)))
		ctx.Data(http.StatusOK, "text/csv; charset=utf-8", csvBytes)
	default:
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Unsupported format. Use json, pdf, or csv",
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
	ctx.JSON(http.StatusGone, gin.H{
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

	// Generate purchase report from SSOT
	purchaseService := services.NewSSOTPurchaseReportService(c.integrationService.GetDB())
	purchaseReport, err := purchaseService.GeneratePurchaseReport(ctx, startDate, endDate, nil)
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
		csvBytes, err := c.pdfService.GenerateTrialBalanceCSV(trialBalance, asOfDate.Format("2006-01-02"))
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to generate CSV", "error": err.Error()})
			return
		}
		ctx.Header("Content-Type", "text/csv; charset=utf-8")
		ctx.Header("Content-Disposition", "attachment; filename=SSOT_Trial_Balance.csv")
		ctx.Header("Content-Length", strconv.Itoa(len(csvBytes)))
		ctx.Data(http.StatusOK, "text/csv; charset=utf-8", csvBytes)
	default:
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Unsupported format. Use json, pdf, or csv",
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
		accountLabel := "All Accounts Activity"
		if strings.TrimSpace(generalLedger.Account.Code) != "" {
			accountLabel = strings.TrimSpace(generalLedger.Account.Code)
			if strings.TrimSpace(generalLedger.Account.Name) != "" {
				accountLabel += " - " + strings.TrimSpace(generalLedger.Account.Name)
			}
		}

		pdfBytes, err := c.pdfService.GenerateGeneralLedgerPDF(generalLedger, accountLabel, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to generate PDF", "error": err.Error()})
			return
		}
		ctx.Header("Content-Type", "application/pdf")
		ctx.Header("Content-Disposition", "attachment; filename=SSOT_General_Ledger.pdf")
		ctx.Header("Content-Length", strconv.Itoa(len(pdfBytes)))
		ctx.Data(http.StatusOK, "application/pdf", pdfBytes)
	case "csv":
		csvBytes, err := c.generateGeneralLedgerCSV(generalLedger, startDate, endDate)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to generate CSV",
				"error":   err.Error(),
			})
			return
		}
		ctx.Header("Content-Type", "text/csv; charset=utf-8")
		ctx.Header("Content-Disposition", "attachment; filename=SSOT_General_Ledger.csv")
		ctx.Data(http.StatusOK, "text/csv", csvBytes)
	default:
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Unsupported format. Use json, pdf, or csv",
		})
	}
}

func (c *SSOTReportIntegrationController) generateGeneralLedgerCSV(data *services.GeneralLedgerData, startDate, endDate time.Time) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\uFEFF")
	buf.WriteString("sep=;\n")

	writer := utils.NewSanitizedCSVWriter(&buf)
	writer.Comma = ';'

	write := func(record []string) error {
		return writer.Write(record)
	}

	accountLabel := "All Accounts Activity"
	isSingleAccount := strings.TrimSpace(data.Account.Code) != ""
	if isSingleAccount {
		accountLabel = strings.TrimSpace(data.Account.Code)
		if strings.TrimSpace(data.Account.Name) != "" {
			accountLabel += " - " + strings.TrimSpace(data.Account.Name)
		}
	}

	if err := write([]string{data.Company.Name}); err != nil {
		return nil, err
	}
	if strings.TrimSpace(data.Company.Address) != "" {
		if err := write([]string{data.Company.Address}); err != nil {
			return nil, err
		}
	}
	if err := write([]string{}); err != nil {
		return nil, err
	}
	if err := write([]string{"GENERAL LEDGER"}); err != nil {
		return nil, err
	}
	if err := write([]string{"Account", accountLabel}); err != nil {
		return nil, err
	}
	if err := write([]string{"Period", fmt.Sprintf("%s to %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))}); err != nil {
		return nil, err
	}
	if err := write([]string{"Generated", time.Now().Format("2006-01-02 15:04")}); err != nil {
		return nil, err
	}
	if err := write([]string{}); err != nil {
		return nil, err
	}

	if isSingleAccount {
		if err := write([]string{"Opening Balance", fmt.Sprintf("%.2f", data.OpeningBalance)}); err != nil {
			return nil, err
		}
		if err := write([]string{"Total Debit", fmt.Sprintf("%.2f", data.TotalDebits)}); err != nil {
			return nil, err
		}
		if err := write([]string{"Total Credit", fmt.Sprintf("%.2f", data.TotalCredits)}); err != nil {
			return nil, err
		}
		if err := write([]string{"Closing Balance", fmt.Sprintf("%.2f", data.ClosingBalance)}); err != nil {
			return nil, err
		}
	} else {
		if err := write([]string{"Period Debit", fmt.Sprintf("%.2f", data.TotalDebits)}); err != nil {
			return nil, err
		}
		if err := write([]string{"Period Credit", fmt.Sprintf("%.2f", data.TotalCredits)}); err != nil {
			return nil, err
		}
	}

	if err := write([]string{}); err != nil {
		return nil, err
	}

	headers := []string{"Date", "Reference", "Account Code", "Account Name", "Description", "Debit", "Credit"}
	if isSingleAccount {
		headers = append(headers, "Balance")
	}
	if err := write(headers); err != nil {
		return nil, err
	}

	for _, tx := range data.Transactions {
		row := []string{
			tx.Date.Format("2006-01-02"),
			tx.JournalCode,
			tx.AccountCode,
			tx.AccountName,
			tx.Description,
			fmt.Sprintf("%.2f", tx.DebitAmount),
			fmt.Sprintf("%.2f", tx.CreditAmount),
		}
		if isSingleAccount {
			row = append(row, fmt.Sprintf("%.2f", tx.Balance))
		}
		if err := write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
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

func normalizeInclusiveEndDate(raw string, parsed time.Time) time.Time {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return parsed
	}

	if !strings.Contains(trimmed, "T") && !strings.Contains(trimmed, " ") {
		return time.Date(
			parsed.Year(),
			parsed.Month(),
			parsed.Day(),
			23, 59, 59, int(time.Second-time.Nanosecond),
			parsed.Location(),
		)
	}

	return parsed
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
