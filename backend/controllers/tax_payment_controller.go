package controllers

import (
	"log"
	"net/http"
	"strings"
	"time"

	"app-sistem-akuntansi/services"

	"github.com/gin-gonic/gin"
)

// Helper function to check if string contains substring (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// TaxPaymentController handles PPN payment operations
type TaxPaymentController struct {
	taxPaymentService *services.TaxPaymentService
}

// NewTaxPaymentController creates a new TaxPaymentController instance
func NewTaxPaymentController(taxPaymentService *services.TaxPaymentService) *TaxPaymentController {
	return &TaxPaymentController{
		taxPaymentService: taxPaymentService,
	}
}

// CreatePPNPayment godoc
// @Summary Create PPN Payment
// @Description Create payment for PPN (either PPN Masukan or PPN Keluaran)
// @Tags Tax Payments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body services.CreatePPNPaymentRequest true "PPN Payment Request"
// @Success 201 {object} map[string]interface{} "success"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/v1/tax-payments/ppn [post]
func (c *TaxPaymentController) CreatePPNPayment(ctx *gin.Context) {
	// Get user ID from context
	userID, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Parse request body
	var req services.CreatePPNPaymentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Failed to bind PPN payment request: %v", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate PPN type
	if req.PPNType != "INPUT" && req.PPNType != "OUTPUT" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "ppn_type must be INPUT or OUTPUT"})
		return
	}

	// Create payment
	payment, err := c.taxPaymentService.CreatePPNPayment(req, userID.(uint))
	if err != nil {
		log.Printf("❌ Failed to create PPN payment: %v", err)

		// Check if it's a validation error (user-friendly errors)
		errMsg := err.Error()
		if contains(errMsg, "tidak ada PPN yang harus dibayar") ||
			contains(errMsg, "PPN Terutang: 0") ||
			contains(errMsg, "cannot exceed PPN Terutang") ||
			contains(errMsg, "insufficient cash/bank balance") {
			// Return 400 for validation errors (not server errors)
			ctx.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
				"type":  "validation_error",
			})
			return
		}

		// Return 500 for actual server errors
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ PPN payment created successfully: %s", payment.Code)
	ctx.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "PPN payment created successfully",
		"data":    payment,
	})
}

// GetPPNPayments godoc
// @Summary Get PPN Payments
// @Description Retrieve PPN payments filtered by type and date range
// @Tags Tax Payments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payment_type query string false "Payment Type (TAX_PPN_INPUT or TAX_PPN_OUTPUT)"
// @Param start_date query string false "Start Date (YYYY-MM-DD)"
// @Param end_date query string false "End Date (YYYY-MM-DD)"
// @Success 200 {object} map[string]interface{} "success"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/v1/tax-payments/ppn [get]
func (c *TaxPaymentController) GetPPNPayments(ctx *gin.Context) {
	// Get query parameters
	paymentType := ctx.Query("payment_type")
	startDateStr := ctx.Query("start_date")
	endDateStr := ctx.Query("end_date")

	// Parse dates
	var startDate, endDate time.Time
	var err error

	if startDateStr != "" {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date format. Use YYYY-MM-DD"})
			return
		}
	}

	if endDateStr != "" {
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format. Use YYYY-MM-DD"})
			return
		}
	}

	// If no payment type specified, return both types
	if paymentType == "" {
		// Get summary
		summary, err := c.taxPaymentService.GetPPNPaymentSummary(startDate, endDate)
		if err != nil {
			log.Printf("❌ Failed to get PPN payment summary: %v", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    nil,
			"summary": summary,
		})
		return
	}

	// Validate payment type
	if paymentType != "TAX_PPN_INPUT" && paymentType != "TAX_PPN_OUTPUT" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "payment_type must be TAX_PPN_INPUT or TAX_PPN_OUTPUT"})
		return
	}

	// Get payments by type
	payments, err := c.taxPaymentService.GetPPNPaymentsByType(paymentType, startDate, endDate)
	if err != nil {
		log.Printf("❌ Failed to get PPN payments: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    payments,
		"count":   len(payments),
	})
}

// GetPPNPaymentSummary godoc
// @Summary Get PPN Payment Summary
// @Description Get summary of PPN payments (both Input and Output)
// @Tags Tax Payments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param start_date query string false "Start Date (YYYY-MM-DD)"
// @Param end_date query string false "End Date (YYYY-MM-DD)"
// @Success 200 {object} map[string]interface{} "success"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/v1/tax-payments/ppn/summary [get]
func (c *TaxPaymentController) GetPPNPaymentSummary(ctx *gin.Context) {
	// Get query parameters
	startDateStr := ctx.Query("start_date")
	endDateStr := ctx.Query("end_date")

	// Parse dates
	var startDate, endDate time.Time
	var err error

	if startDateStr != "" {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date format. Use YYYY-MM-DD"})
			return
		}
	}

	if endDateStr != "" {
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format. Use YYYY-MM-DD"})
			return
		}
	}

	// Get summary
	summary, err := c.taxPaymentService.GetPPNPaymentSummary(startDate, endDate)
	if err != nil {
		log.Printf("❌ Failed to get PPN payment summary: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    summary,
	})
}

// GetPPNMasukanPayments godoc
// @Summary Get PPN Masukan Payments
// @Description Retrieve PPN Masukan (Purchase VAT) payments
// @Tags Tax Payments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param start_date query string false "Start Date (YYYY-MM-DD)"
// @Param end_date query string false "End Date (YYYY-MM-DD)"
// @Success 200 {object} map[string]interface{} "success"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/v1/tax-payments/ppn/masukan [get]
func (c *TaxPaymentController) GetPPNMasukanPayments(ctx *gin.Context) {
	ctx.Set("payment_type", "TAX_PPN_INPUT")
	c.GetPPNPayments(ctx)
}

// GetPPNKeluaranPayments godoc
// @Summary Get PPN Keluaran Payments
// @Description Retrieve PPN Keluaran (Sales VAT) payments
// @Tags Tax Payments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param start_date query string false "Start Date (YYYY-MM-DD)"
// @Param end_date query string false "End Date (YYYY-MM-DD)"
// @Success 200 {object} map[string]interface{} "success"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/v1/tax-payments/ppn/keluaran [get]
func (c *TaxPaymentController) GetPPNKeluaranPayments(ctx *gin.Context) {
	ctx.Set("payment_type", "TAX_PPN_OUTPUT")
	c.GetPPNPayments(ctx)
}

// GetPPNBalance godoc
// @Summary Get PPN Balance
// @Description Get current PPN balance (either Input or Output)
// @Tags Tax Payments
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param type query string true "PPN Type (INPUT or OUTPUT)"
// @Success 200 {object} map[string]interface{} "success"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/v1/tax-payments/ppn/balance [get]
func (c *TaxPaymentController) GetPPNBalance(ctx *gin.Context) {
	ppnType := ctx.Query("type")

	// Validate PPN type
	if ppnType != "INPUT" && ppnType != "OUTPUT" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "type must be INPUT or OUTPUT"})
		return
	}

	// Get balance from service
	balance, err := c.taxPaymentService.GetPPNBalance(ppnType)
	if err != nil {
		log.Printf("❌ Failed to get PPN balance: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"balance": balance,
		"type":    ppnType,
	})
}
