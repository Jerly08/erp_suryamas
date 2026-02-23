package controllers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"app-sistem-akuntansi/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SSOTPurchaseReportController handles purchase report requests
type SSOTPurchaseReportController struct {
	db            *gorm.DB
	reportService *services.SSOTPurchaseReportService
	exportService *services.PurchaseReportExportService
}

// NewSSOTPurchaseReportController creates a new SSOT purchase report controller
func NewSSOTPurchaseReportController(db *gorm.DB) *SSOTPurchaseReportController {
	return &SSOTPurchaseReportController{
		db:            db,
		reportService: services.NewSSOTPurchaseReportService(db),
		exportService: services.NewPurchaseReportExportService(db),
	}
}

// GetPurchaseReportResponse represents the API response structure
type GetPurchaseReportResponse struct {
	Success   bool                         `json:"success"`
	Message   string                       `json:"message"`
	Data      *services.PurchaseReportData `json:"data,omitempty"`
	Error     string                       `json:"error,omitempty"`
	Timestamp time.Time                    `json:"timestamp"`
}

// @Summary Generate Purchase Report
// @Description Generate comprehensive purchase report from SSOT journal data with accurate financial analysis
// @Tags Purchase Reports
// @Accept json
// @Produce json
// @Param start_date query string true "Start date (YYYY-MM-DD format)"
// @Param end_date query string true "End date (YYYY-MM-DD format)"
// @Param format query string false "Output format (json, pdf, csv)" default(json)
// @Success 200 {object} GetPurchaseReportResponse "Purchase report generated successfully"
// @Failure 400 {object} GetPurchaseReportResponse "Invalid date parameters"
// @Failure 500 {object} GetPurchaseReportResponse "Internal server error"
// @Router /api/v1/ssot-reports/purchase-report [get]
func (c *SSOTPurchaseReportController) GetPurchaseReport(ctx *gin.Context) {
	// Parse query parameters
	startDateStr := ctx.Query("start_date")
	endDateStr := ctx.Query("end_date")
	format := ctx.DefaultQuery("format", "json")

	if startDateStr == "" || endDateStr == "" {
		ctx.JSON(http.StatusBadRequest, GetPurchaseReportResponse{
			Success:   false,
			Message:   "Both start_date and end_date parameters are required",
			Error:     "Missing required date parameters",
			Timestamp: time.Now(),
		})
		return
	}

	// Parse dates
	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, GetPurchaseReportResponse{
			Success:   false,
			Message:   "Invalid start_date format. Use YYYY-MM-DD",
			Error:     err.Error(),
			Timestamp: time.Now(),
		})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, GetPurchaseReportResponse{
			Success:   false,
			Message:   "Invalid end_date format. Use YYYY-MM-DD",
			Error:     err.Error(),
			Timestamp: time.Now(),
		})
		return
	}

	// Adjust end date to include the entire day
	endDate = endDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	// Generate report
	reportContext := context.Background()
	report, err := c.reportService.GeneratePurchaseReport(reportContext, startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, GetPurchaseReportResponse{
			Success:   false,
			Message:   "Failed to generate purchase report",
			Error:     err.Error(),
			Timestamp: time.Now(),
		})
		return
	}

	// Handle output formats consistently with SSOT P&L
	switch format {
	case "json":
		ctx.JSON(http.StatusOK, GetPurchaseReportResponse{
			Success:   true,
			Message:   "Purchase report generated successfully",
			Data:      report,
			Timestamp: time.Now(),
		})
	case "pdf":
		// Get user ID from context (assuming it's set by auth middleware)
		userID := uint(1) // Default fallback, should be extracted from JWT/session
		if userIDValue, exists := ctx.Get("user_id"); exists {
			if uid, ok := userIDValue.(uint); ok {
				userID = uid
			}
		}

		pdfBytes, err := c.exportService.ExportToPDF(report, userID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, GetPurchaseReportResponse{Success: false, Message: "Failed to generate PDF", Error: err.Error(), Timestamp: time.Now()})
			return
		}
		filename := fmt.Sprintf("SSOT_Purchase_Report_%s_to_%s.pdf", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
		ctx.Header("Content-Type", "application/pdf")
		ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		ctx.Header("Content-Length", strconv.Itoa(len(pdfBytes)))
		ctx.Data(http.StatusOK, "application/pdf", pdfBytes)
	case "csv":
		// Get user ID from context (assuming it's set by auth middleware)
		userID := uint(1) // Default fallback, should be extracted from JWT/session
		if userIDValue, exists := ctx.Get("user_id"); exists {
			if uid, ok := userIDValue.(uint); ok {
				userID = uid
			}
		}

		csvBytes, err := c.exportService.ExportToCSV(report, userID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, GetPurchaseReportResponse{Success: false, Message: "Failed to generate CSV", Error: err.Error(), Timestamp: time.Now()})
			return
		}
		filename := fmt.Sprintf("SSOT_Purchase_Report_%s_to_%s.csv", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
		ctx.Header("Content-Type", "text/csv")
		ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		ctx.Header("Content-Length", strconv.Itoa(len(csvBytes)))
		ctx.Data(http.StatusOK, "text/csv", csvBytes)
	default:
		ctx.JSON(http.StatusBadRequest, GetPurchaseReportResponse{Success: false, Message: "Unsupported format. Use json, pdf, or csv", Timestamp: time.Now()})
	}
}

// @Summary Get Purchase Report Summary
// @Description Get a quick summary of purchase report data
// @Tags Purchase Reports
// @Accept json
// @Produce json
// @Param start_date query string true "Start date (YYYY-MM-DD format)"
// @Param end_date query string true "End date (YYYY-MM-DD format)"
// @Success 200 {object} map[string]interface{} "Purchase summary data"
// @Failure 400 {object} GetPurchaseReportResponse "Invalid parameters"
// @Failure 500 {object} GetPurchaseReportResponse "Internal server error"
// @Router /api/v1/ssot-reports/purchase-summary [get]
func (c *SSOTPurchaseReportController) GetPurchaseSummary(ctx *gin.Context) {
	// Parse query parameters
	startDateStr := ctx.Query("start_date")
	endDateStr := ctx.Query("end_date")

	if startDateStr == "" || endDateStr == "" {
		ctx.JSON(http.StatusBadRequest, GetPurchaseReportResponse{
			Success:   false,
			Message:   "Both start_date and end_date parameters are required",
			Error:     "Missing required date parameters",
			Timestamp: time.Now(),
		})
		return
	}

	// Parse dates
	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, GetPurchaseReportResponse{
			Success:   false,
			Message:   "Invalid start_date format. Use YYYY-MM-DD",
			Error:     err.Error(),
			Timestamp: time.Now(),
		})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, GetPurchaseReportResponse{
			Success:   false,
			Message:   "Invalid end_date format. Use YYYY-MM-DD",
			Error:     err.Error(),
			Timestamp: time.Now(),
		})
		return
	}

	// Adjust end date to include the entire day
	endDate = endDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	// Generate report
	reportContext := context.Background()
	report, err := c.reportService.GeneratePurchaseReport(reportContext, startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, GetPurchaseReportResponse{
			Success:   false,
			Message:   "Failed to generate purchase summary",
			Error:     err.Error(),
			Timestamp: time.Now(),
		})
		return
	}

	// Create summary response
	summary := map[string]interface{}{
		"success":   true,
		"message":   "Purchase summary generated successfully",
		"timestamp": time.Now(),
		"summary": map[string]interface{}{
			"total_purchases":      report.TotalPurchases,
			"completed_purchases":  report.CompletedPurchases,
			"total_amount":         report.TotalAmount,
			"total_paid":           report.TotalPaid,
			"outstanding_payables": report.OutstandingPayables,
			"currency":             report.Currency,
			"period": map[string]interface{}{
				"start_date": report.StartDate.Format("2006-01-02"),
				"end_date":   report.EndDate.Format("2006-01-02"),
			},
		},
		"payment_breakdown": map[string]interface{}{
			"cash_purchases":    report.PaymentAnalysis.CashPurchases,
			"credit_purchases":  report.PaymentAnalysis.CreditPurchases,
			"cash_amount":       report.PaymentAnalysis.CashAmount,
			"credit_amount":     report.PaymentAnalysis.CreditAmount,
			"cash_percentage":   report.PaymentAnalysis.CashPercentage,
			"credit_percentage": report.PaymentAnalysis.CreditPercentage,
		},
		"vendor_count":   len(report.PurchasesByVendor),
		"category_count": len(report.PurchasesByCategory),
		"monthly_count":  len(report.PurchasesByMonth),
	}

	ctx.JSON(http.StatusOK, summary)
}

// @Summary Validate Purchase Report Data
// @Description Validate the integrity and accuracy of purchase report data
// @Tags Purchase Reports
// @Accept json
// @Produce json
// @Param start_date query string true "Start date (YYYY-MM-DD format)"
// @Param end_date query string true "End date (YYYY-MM-DD format)"
// @Success 200 {object} map[string]interface{} "Validation results"
// @Failure 400 {object} GetPurchaseReportResponse "Invalid parameters"
// @Failure 500 {object} GetPurchaseReportResponse "Internal server error"
// @Router /api/v1/ssot-reports/purchase-report/validate [get]
func (c *SSOTPurchaseReportController) ValidatePurchaseReport(ctx *gin.Context) {
	// Parse query parameters
	startDateStr := ctx.Query("start_date")
	endDateStr := ctx.Query("end_date")

	if startDateStr == "" || endDateStr == "" {
		ctx.JSON(http.StatusBadRequest, GetPurchaseReportResponse{
			Success:   false,
			Message:   "Both start_date and end_date parameters are required",
			Error:     "Missing required date parameters",
			Timestamp: time.Now(),
		})
		return
	}

	// Parse dates
	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, GetPurchaseReportResponse{
			Success:   false,
			Message:   "Invalid start_date format. Use YYYY-MM-DD",
			Error:     err.Error(),
			Timestamp: time.Now(),
		})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, GetPurchaseReportResponse{
			Success:   false,
			Message:   "Invalid end_date format. Use YYYY-MM-DD",
			Error:     err.Error(),
			Timestamp: time.Now(),
		})
		return
	}

	// Adjust end date to include the entire day
	endDate = endDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	// Generate report
	reportContext := context.Background()
	report, err := c.reportService.GeneratePurchaseReport(reportContext, startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, GetPurchaseReportResponse{
			Success:   false,
			Message:   "Failed to validate purchase report",
			Error:     err.Error(),
			Timestamp: time.Now(),
		})
		return
	}

	// Perform validation checks
	validationResults := map[string]interface{}{
		"success":   true,
		"message":   "Purchase report validation completed",
		"timestamp": time.Now(),
		"period": map[string]string{
			"start_date": startDate.Format("2006-01-02"),
			"end_date":   endDate.Format("2006-01-02"),
		},
		"validation_checks": map[string]interface{}{},
	}

	checks := validationResults["validation_checks"].(map[string]interface{})

	// Check 1: Outstanding calculation
	expectedOutstanding := report.TotalAmount - report.TotalPaid
	outstandingValid := abs(report.OutstandingPayables-expectedOutstanding) <= 0.01
	checks["outstanding_calculation"] = map[string]interface{}{
		"valid":       outstandingValid,
		"expected":    expectedOutstanding,
		"actual":      report.OutstandingPayables,
		"description": "Outstanding = Total Amount - Total Paid",
	}

	// Check 2: Payment percentage totals
	totalPercentage := report.PaymentAnalysis.CashPercentage + report.PaymentAnalysis.CreditPercentage
	percentageValid := totalPercentage == 0 || abs(totalPercentage-100.0) <= 0.1
	checks["payment_percentages"] = map[string]interface{}{
		"valid":             percentageValid,
		"cash_percentage":   report.PaymentAnalysis.CashPercentage,
		"credit_percentage": report.PaymentAnalysis.CreditPercentage,
		"total_percentage":  totalPercentage,
		"description":       "Cash % + Credit % should equal 100% (or 0% if no purchases)",
	}

	// Check 3: Vendor totals consistency
	vendorTotalAmount := 0.0
	for _, vendor := range report.PurchasesByVendor {
		vendorTotalAmount += vendor.TotalAmount
	}
	vendorTotalsValid := len(report.PurchasesByVendor) == 0 || abs(vendorTotalAmount-report.TotalAmount) <= 0.01
	checks["vendor_totals"] = map[string]interface{}{
		"valid":         vendorTotalsValid,
		"vendor_total":  vendorTotalAmount,
		"summary_total": report.TotalAmount,
		"vendor_count":  len(report.PurchasesByVendor),
		"description":   "Sum of vendor amounts should equal summary total",
	}

	// Check 4: Payment amount consistency
	paymentTotalAmount := report.PaymentAnalysis.CashAmount + report.PaymentAnalysis.CreditAmount
	paymentAmountValid := paymentTotalAmount == 0 || abs(paymentTotalAmount-report.TotalAmount) <= 0.01
	checks["payment_amounts"] = map[string]interface{}{
		"valid":         paymentAmountValid,
		"cash_amount":   report.PaymentAnalysis.CashAmount,
		"credit_amount": report.PaymentAnalysis.CreditAmount,
		"payment_total": paymentTotalAmount,
		"summary_total": report.TotalAmount,
		"description":   "Cash amount + Credit amount should equal total amount",
	}

	// Overall validation status
	allValid := outstandingValid && percentageValid && vendorTotalsValid && paymentAmountValid
	validationResults["overall_valid"] = allValid
	validationResults["validation_score"] = map[string]interface{}{
		"passed": func() int {
			count := 0
			if outstandingValid {
				count++
			}
			if percentageValid {
				count++
			}
			if vendorTotalsValid {
				count++
			}
			if paymentAmountValid {
				count++
			}
			return count
		}(),
		"total": 4,
	}

	ctx.JSON(http.StatusOK, validationResults)
}

// Helper function to calculate absolute value
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
