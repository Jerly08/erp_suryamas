package controllers

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/services"
	"app-sistem-akuntansi/utils"
	"bytes"
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SalesController struct {
	salesServiceV2        *services.SalesServiceV2 // NEW: Clean sales service
	paymentService        *services.PaymentService
	unifiedPaymentService *services.UnifiedSalesPaymentService
	pdfService            services.PDFServiceInterface
	db                    *gorm.DB
	accountRepo           repositories.AccountRepository
}

func mapSaleWriteError(err error) (int, string, map[string]string, map[string]interface{}, bool) {
	if err == nil {
		return 0, "", nil, nil, false
	}

	errorMsg := strings.TrimSpace(err.Error())

	switch {
	case strings.Contains(errorMsg, "customer not found"):
		return http.StatusNotFound, "Customer not found", nil, nil, true
	case strings.Contains(errorMsg, "sale not found"):
		return http.StatusNotFound, "Sale not found", nil, nil, true
	case strings.Contains(errorMsg, "stock tidak mencukupi"),
		strings.Contains(errorMsg, "stock habis"),
		strings.Contains(errorMsg, "stock tidak cukup"),
		strings.Contains(errorMsg, "stok 0"):
		return http.StatusBadRequest, "Stock validation failed", map[string]string{
			"items": errorMsg,
		}, nil, true
	case strings.Contains(errorMsg, "manual sale item"),
		strings.Contains(errorMsg, "product name"),
		strings.Contains(errorMsg, "invalid sale item"),
		strings.Contains(errorMsg, "validation"):
		return http.StatusBadRequest, "Sale validation failed", map[string]string{
			"items": errorMsg,
		}, nil, true
	case strings.Contains(errorMsg, "inventory"),
		strings.Contains(errorMsg, "tidak bisa mengubah"),
		strings.Contains(errorMsg, "closed period"),
		strings.Contains(errorMsg, "CLOSED_PERIOD:"):
		return http.StatusBadRequest, "Sale update is not allowed", nil, map[string]interface{}{
			"details": errorMsg,
		}, true
	default:
		return 0, "", nil, nil, false
	}
}

func sendSaleWriteError(c *gin.Context, err error) {
	status, message, validationErrors, context, handled := mapSaleWriteError(err)
	if !handled {
		utils.SendInternalError(c, "Failed to process sale", err.Error())
		return
	}

	switch status {
	case http.StatusNotFound:
		utils.SendNotFound(c, message)
	case http.StatusBadRequest:
		if len(validationErrors) > 0 {
			utils.SendValidationError(c, message, validationErrors)
			return
		}
		utils.SendBusinessRuleError(c, message, context)
	default:
		utils.SendInternalError(c, message, err.Error())
	}
}

func NewSalesController(salesServiceV2 *services.SalesServiceV2, paymentService *services.PaymentService, unifiedPaymentService *services.UnifiedSalesPaymentService, pdfService services.PDFServiceInterface, db *gorm.DB, accountRepo repositories.AccountRepository) *SalesController {
	return &SalesController{
		salesServiceV2:        salesServiceV2, // Use V2 service
		paymentService:        paymentService,
		unifiedPaymentService: unifiedPaymentService,
		pdfService:            pdfService,
		db:                    db,
		accountRepo:           accountRepo,
	}
}

// Sales Management

// GetSales godoc
// @Summary Get all sales
// @Description Get paginated list of sales with filters
// @Tags Sales
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Param status query string false "Sales status filter"
// @Param customer_id query string false "Customer ID filter"
// @Param start_date query string false "Start date filter (YYYY-MM-DD)"
// @Param end_date query string false "End date filter (YYYY-MM-DD)"
// @Param search query string false "Search term"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/sales [get]
func (sc *SalesController) GetSales(c *gin.Context) {
	log.Printf("📋 Getting sales list with filters")

	// Parse and validate pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	// Validate pagination bounds
	if page < 1 {
		utils.SendValidationError(c, "Invalid pagination parameters", map[string]string{
			"page": "Page must be greater than 0",
		})
		return
	}
	if limit < 1 || limit > 100 {
		utils.SendValidationError(c, "Invalid pagination parameters", map[string]string{
			"limit": "Limit must be between 1 and 100",
		})
		return
	}

	// Get filter parameters
	status := c.Query("status")
	customerID := c.Query("customer_id")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	search := c.Query("search")

	filter := models.SalesFilter{
		Status:     status,
		CustomerID: customerID,
		StartDate:  startDate,
		EndDate:    endDate,
		Search:     search,
		Page:       page,
		Limit:      limit,
	}

	log.Printf("🔍 Fetching sales with filters: page=%d, limit=%d, status=%s, customer_id=%s",
		page, limit, status, customerID)

	result, err := sc.salesServiceV2.GetSales(filter)
	if err != nil {
		log.Printf("❌ Failed to get sales: %v", err)
		utils.SendInternalError(c, "Failed to retrieve sales data", err.Error())
		return
	}

	log.Printf("✅ Retrieved %d sales (total: %d)", len(result.Data), result.Total)

	// Send paginated success response
	utils.SendPaginatedSuccess(c,
		"Sales retrieved successfully",
		result.Data,
		result.Page,
		result.Limit,
		int64(result.Total))
}

// GetSale godoc
// @Summary Get sale by ID
// @Description Get a single sale by ID
// @Tags Sales
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Sale ID"
// @Success 200 {object} models.Sale
// @Failure 400 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Router /api/v1/sales/{id} [get]
func (sc *SalesController) GetSale(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		log.Printf("❌ Invalid sale ID parameter: %v", err)
		utils.SendValidationError(c, "Invalid sale ID", map[string]string{
			"id": "Sale ID must be a valid positive number",
		})
		return
	}

	log.Printf("🔍 Getting sale details for ID: %d", id)

	sale, err := sc.salesServiceV2.GetSaleByID(uint(id))
	if err != nil {
		log.Printf("❌ Sale %d not found: %v", id, err)
		utils.SendSaleNotFound(c, uint(id))
		return
	}

	// Debug: Log key fields that might be null
	log.Printf("🔍 Sale %d debug info:", id)
	log.Printf("  Customer: %+v", sale.Customer)
	log.Printf("  Sales Person ID: %v", sale.SalesPersonID)
	log.Printf("  Sales Person: %+v", sale.SalesPerson)
	log.Printf("  Invoice Number: '%s'", sale.InvoiceNumber)
	log.Printf("  Due Date: %v", sale.DueDate)
	log.Printf("  Payment Terms: '%s'", sale.PaymentTerms)

	log.Printf("✅ Retrieved sale %d details successfully", id)
	utils.SendSuccess(c, "Sale retrieved successfully", sale)
}

// ValidateSaleStock godoc
// @Summary Validate stock for sale
// @Description Validate stock levels for items in the sales create form without creating a sale
// @Tags Sales
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body models.StockValidationRequest true "Stock validation request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/sales/validate-stock [post]
func (sc *SalesController) ValidateSaleStock(c *gin.Context) {
	var req models.StockValidationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendValidationError(c, "Invalid request payload", map[string]string{
			"request": "Please provide items to validate",
		})
		return
	}

	res, err := sc.salesServiceV2.ValidateStockForCreate(req)
	if err != nil {
		utils.SendInternalError(c, "Failed to validate stock", err.Error())
		return
	}

	utils.SendSuccess(c, "Stock validation completed", res)
}

// CreateSale godoc
// @Summary Create new sale
// @Description Create a new sale
// @Tags Sales
// @Accept json
// @Produce json
// @Security Bearer
// @Param sale body models.SaleCreateRequest true "Sale data"
// @Success 201 {object} models.Sale
// @Failure 400 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/sales [post]
func (sc *SalesController) CreateSale(c *gin.Context) {
	log.Printf("🎆 Creating new sale")

	var request models.SaleCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Printf("❌ Invalid sale creation request: %v", err)
		utils.SendValidationError(c, "Invalid sale data", utils.FormatValidationError(err))
		return
	}

	if request.ProjectID != nil {
		log.Printf("📥 Check CreateSale Request ProjectID: %d", *request.ProjectID)
	} else {
		log.Printf("📥 Check CreateSale Request ProjectID is NIL!")
	}

	// Get user ID from context with error handling
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		log.Printf("❌ User authentication missing for sale creation")
		utils.SendUnauthorized(c, "User authentication required")
		return
	}
	userID, ok := userIDInterface.(uint)
	if !ok {
		log.Printf("❌ Invalid user ID type: %T", userIDInterface)
		utils.SendUnauthorized(c, "Invalid user authentication")
		return
	}

	log.Printf("📄 Creating sale for customer %d by user %d", request.CustomerID, userID)

	sale, err := sc.salesServiceV2.CreateSale(request, userID)
	if err != nil {
		log.Printf("❌ Failed to create sale: %v", err)
		sendSaleWriteError(c, err)
		return
	}

	log.Printf("✅ Sale created successfully: ID=%d, Code=%s", sale.ID, sale.Code)
	utils.SendCreated(c, "Sale created successfully", sale)
}

// UpdateSale godoc
// @Summary Update sale
// @Description Update an existing sale
// @Tags Sales
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Sale ID"
// @Param sale body models.SaleUpdateRequest true "Sale update data"
// @Success 200 {object} models.Sale
// @Failure 400 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/sales/{id} [put]
func (sc *SalesController) UpdateSale(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.SendValidationError(c, "Invalid sale ID", map[string]string{
			"id": "Sale ID must be a valid positive number",
		})
		return
	}

	var request models.SaleUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.SendValidationError(c, "Invalid sale update data", utils.FormatValidationError(err))
		return
	}

	if request.ProjectID != nil {
		log.Printf("📥 Check UpdateSale Request ProjectID: %d", *request.ProjectID)
	} else {
		log.Printf("📥 Check UpdateSale Request ProjectID is NIL!")
	}

	userID := c.MustGet("user_id").(uint)

	sale, err := sc.salesServiceV2.UpdateSale(uint(id), request, userID)
	if err != nil {
		log.Printf("❌ Failed to update sale %d: %v", id, err)
		sendSaleWriteError(c, err)
		return
	}

	utils.SendSuccess(c, "Sale updated successfully", sale)
}

// DeleteSale godoc
// @Summary Delete sale
// @Description Delete a sale
// @Tags Sales
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Sale ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/sales/{id} [delete]
func (sc *SalesController) DeleteSale(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid sale ID"})
		return
	}

	// For V2, we'll delete the sale directly (role checks can be re-enabled if needed)
	if err := sc.salesServiceV2.DeleteSale(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Sale cancelled successfully"})
}

// Sales Status Management

// ConfirmSale godoc
// @Summary Confirm sale
// @Description Confirm a sale (changes status to CONFIRMED)
// @Tags Sales
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Sale ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/sales/{id}/confirm [post]
func (sc *SalesController) ConfirmSale(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid sale ID"})
		return
	}

	userID := c.MustGet("user_id").(uint)

	sale, err := sc.salesServiceV2.ConfirmSale(uint(id), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Sale confirmed successfully", "data": sale})
}

// InvoiceSale godoc
// @Summary Create invoice from sale
// @Description Create an invoice from a sale
// @Tags Sales
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Sale ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/sales/{id}/invoice [post]
func (sc *SalesController) InvoiceSale(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid sale ID"})
		return
	}

	userID := c.MustGet("user_id").(uint)

	sale, err := sc.salesServiceV2.GetSaleByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Sale Not Found",
			"message": "Sale dengan ID tersebut tidak ditemukan",
		})
		return
	}
	if err := services.CheckClosedPeriod(sc.db, sale.Date); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Periode Sudah Ditutup",
			"message": err.Error(),
			"code":    "PERIOD_CLOSED",
		})
		return
	}

	invoice, err := sc.salesServiceV2.CreateInvoice(uint(id), userID)
	if err != nil {
		log.Printf("❌ Failed to create invoice for sale %d: %v", id, err)

		// Handle specific error types with appropriate status codes and messages
		errorMsg := err.Error()
		switch {
		case strings.Contains(errorMsg, "invoice already created"):
			// Duplicate invoice attempt - return 400 with clear message
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invoice Sudah Dibuat",
				"message": errorMsg,
				"code":    "DUPLICATE_INVOICE",
				"details": "Invoice untuk penjualan ini sudah dibuat sebelumnya. Silakan refresh halaman.",
			})
		case strings.Contains(errorMsg, "stock tidak cukup") ||
			strings.Contains(errorMsg, "stock habis") ||
			strings.Contains(errorMsg, "gagal mengurangi stock"):
			// Stock validation failed - return 400 with clear message
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Stock Tidak Mencukupi",
				"message": errorMsg,
				"code":    "INSUFFICIENT_STOCK",
				"details": "Silakan periksa ketersediaan stock produk sebelum membuat invoice.",
			})
		case strings.Contains(errorMsg, "sale not found"):
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Sale Not Found",
				"message": "Sale dengan ID tersebut tidak ditemukan",
				"code":    "SALE_NOT_FOUND",
			})
		case strings.Contains(errorMsg, "only DRAFT or CONFIRMED"):
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid Sale Status",
				"message": errorMsg,
				"code":    "INVALID_STATUS",
			})
		case strings.Contains(errorMsg, "failed to generate invoice number"):
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Invoice Number Generation Failed",
				"message": "Gagal generate nomor invoice. Silakan coba lagi.",
				"code":    "INVOICE_NUMBER_ERROR",
				"details": errorMsg,
			})
		default:
			// Generic internal error
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to Create Invoice",
				"message": "Terjadi kesalahan saat membuat invoice. Silakan coba lagi.",
				"code":    "INVOICE_CREATION_ERROR",
				"details": errorMsg,
			})
		}
		return
	}

	log.Printf("✅ Invoice created successfully for sale %d", id)
	c.JSON(http.StatusOK, invoice)
}

// CancelSale godoc
// @Summary Cancel sale
// @Description Cancel a sale with reason
// @Tags Sales
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Sale ID"
// @Param request body map[string]interface{} false "Cancel request with reason"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/sales/{id}/cancel [post]
func (sc *SalesController) CancelSale(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid sale ID"})
		return
	}

	var request struct {
		Reason string `json:"reason"`
	}
	c.ShouldBindJSON(&request)

	userID := c.MustGet("user_id").(uint)

	if err := sc.salesServiceV2.CancelSale(uint(id), request.Reason, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Sale cancelled successfully"})
}

// Payment Management

// GetSalePayments gets all payments for a sale
func (sc *SalesController) GetSalePayments(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid sale ID"})
		return
	}

	payments, err := sc.salesServiceV2.GetSalePayments(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, payments)
}

// CreateSalePayment creates a payment for a sale with proper race condition protection
func (sc *SalesController) CreateSalePayment(c *gin.Context) {
	log.Printf("🚀 Starting payment creation process")

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		log.Printf("❌ Invalid sale ID: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"error":   "Invalid sale ID",
			"details": err.Error(),
			"code":    "INVALID_SALE_ID",
		})
		return
	}

	var request models.SalePaymentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Printf("❌ Invalid request data for sale %d: %v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status":           "error",
			"error":            "Invalid request data",
			"details":          err.Error(),
			"code":             "VALIDATION_ERROR",
			"validation_error": true,
		})
		return
	}

	// Set the sale ID from the URL parameter
	request.SaleID = uint(id)
	log.Printf("💰 Processing payment request for sale %d: amount=%.2f, method=%s",
		id, request.Amount, request.PaymentMethod)

	// Get user ID from context with proper error handling
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		log.Printf("❌ User authentication missing for sale %d payment", id)
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"error":   "User not authenticated",
			"details": "user_id not found in context",
			"code":    "AUTH_MISSING",
		})
		return
	}
	userID, ok := userIDInterface.(uint)
	if !ok {
		log.Printf("❌ Invalid user ID type for sale %d payment: %T", id, userIDInterface)
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"error":   "Invalid user authentication",
			"details": "user_id has invalid type",
			"code":    "AUTH_INVALID",
		})
		return
	}

	// ✅ FIX: Use SalesServiceV2.ProcessPayment instead of disabled UnifiedSalesPaymentService
	// The UnifiedSalesPaymentService is disabled in stub_services.go to prevent auto-posting
	// We should use the working SalesServiceV2.ProcessPayment() which creates proper journals
	log.Printf("💡 Using SalesServiceV2.ProcessPayment for partial payment support")

	payment, err := sc.salesServiceV2.ProcessPayment(uint(id), request, userID)
	if err != nil {
		log.Printf("❌ Payment creation failed for sale %d: %v", id, err)

		// Determine appropriate HTTP status based on error type
		status := http.StatusInternalServerError
		code := "PAYMENT_CREATION_ERROR"

		errorMsg := err.Error()
		switch {
		case strings.Contains(errorMsg, "not found"):
			status = http.StatusNotFound
			code = "SALE_NOT_FOUND"
		case strings.Contains(errorMsg, "exceeds outstanding"):
			status = http.StatusBadRequest
			code = "AMOUNT_EXCEEDS_OUTSTANDING"
		case strings.Contains(errorMsg, "cannot receive payments"):
			status = http.StatusBadRequest
			code = "INVALID_SALE_STATUS"
		case strings.Contains(errorMsg, "no outstanding amount"):
			status = http.StatusBadRequest
			code = "NO_OUTSTANDING_AMOUNT"
		case strings.Contains(errorMsg, "validation"):
			status = http.StatusBadRequest
			code = "VALIDATION_ERROR"
		}

		c.JSON(status, gin.H{
			"status":  "error",
			"error":   "Failed to create payment",
			"details": errorMsg,
			"code":    code,
			"sale_id": id,
			"user_id": userID,
		})
		return
	}

	log.Printf("✅ Payment created successfully for sale %d: payment_id=%d, amount=%.2f",
		id, payment.ID, payment.Amount)

	// Return success response with comprehensive data
	c.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"message": "Payment created successfully with race condition protection",
		"data":    payment,
		"meta": gin.H{
			"sale_id":    payment.SaleID,
			"payment_id": payment.ID,
			"user_id":    userID,
			"created_at": payment.CreatedAt,
		},
	})
}

// Integrated Payment Management - uses Payment Service for comprehensive payment tracking

// CreateIntegratedPayment creates payment via Payment Management for better tracking
func (sc *SalesController) CreateIntegratedPayment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid sale ID"})
		return
	}

	var request struct {
		Amount       float64   `json:"amount" binding:"required,min=0"`
		Date         time.Time `json:"date" binding:"required"`
		Method       string    `json:"method" binding:"required"`
		CashBankID   uint      `json:"cash_bank_id" binding:"required"`
		ExchangeRate float64   `json:"exchange_rate"`
		Reference    string    `json:"reference"`
		Notes        string    `json:"notes"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		log.Printf("Payment creation validation error for sale %d: %v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
			"expected_fields": map[string]string{
				"amount":        "number (required, min=0)",
				"date":          "datetime string (required, ISO format)",
				"method":        "string (required)",
				"cash_bank_id":  "number (required)",
				"exchange_rate": "number (optional, required when cash/bank currency is USD)",
				"reference":     "string (optional)",
				"notes":         "string (optional)",
			},
		})
		return
	}

	// Log successful request parsing
	log.Printf("Received integrated payment request for sale %d: amount=%.2f, method=%s, cash_bank_id=%d", id, request.Amount, request.Method, request.CashBankID)

	// Get sale details to validate and get customer ID
	sale, err := sc.salesServiceV2.GetSaleByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Sale not found",
			"details": err.Error(),
		})
		return
	}

	// Validate sale status
	if sale.Status != models.SaleStatusInvoiced && sale.Status != models.SaleStatusOverdue {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "Sale must be invoiced to receive payments",
			"sale_status": sale.Status,
		})
		return
	}

	// Validate payment amount
	if request.Amount > sale.OutstandingAmount {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":              "Payment amount exceeds outstanding amount",
			"outstanding_amount": sale.OutstandingAmount,
			"requested_amount":   request.Amount,
		})
		return
	}

	// Get user ID with error handling
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		log.Printf("Error: user_id not found in context for sale %d payment", id)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "User not authenticated",
			"details": "user_id not found in context",
		})
		return
	}
	userID, ok := userIDInterface.(uint)
	if !ok {
		log.Printf("Error: user_id has invalid type for sale %d payment: %T", id, userIDInterface)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Invalid user authentication",
			"details": "user_id has invalid type",
		})
		return
	}

	// Create payment request for Payment Management service
	paymentRequest := services.PaymentCreateRequest{
		ContactID:    sale.CustomerID,
		CashBankID:   request.CashBankID,
		Date:         request.Date,
		Amount:       request.Amount,
		Method:       request.Method,
		ExchangeRate: request.ExchangeRate,
		Reference:    request.Reference,
		Notes:        fmt.Sprintf("Payment for Invoice %s - %s", sale.InvoiceNumber, request.Notes),
		Allocations: []services.InvoiceAllocation{
			{
				InvoiceID: uint(id),
				Amount:    request.Amount,
			},
		},
	}

	// Use Payment Management service (needs to be injected)
	log.Printf("Calling PaymentService.CreateReceivablePayment for sale %d with amount %.2f", id, paymentRequest.Amount)
	payment, err := sc.paymentService.CreateReceivablePayment(paymentRequest, userID)
	if err != nil {
		log.Printf("Error in CreateReceivablePayment for sale %d: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to create payment",
			"details": err.Error(),
			"status":  "error",
		})
		return
	}
	log.Printf("✅ Payment created successfully: ID=%d, Code=%s", payment.ID, payment.Code)

	// ❌ REMOVED: Double COA sync - PaymentService.CreateReceivablePayment() already handles journal entries and COA updates
	// The previous code here was causing DOUBLE POSTING to COA because:
	// 1. PaymentService creates journal entry → COA updated
	// 2. SyncCOABalanceAfterPayment() updates COA AGAIN → DOUBLE!
	// FIX: Trust PaymentService to handle everything correctly

	// Return response with both payment info and updated sale status
	updatedSale, err := sc.salesServiceV2.GetSaleByID(uint(id))
	if err != nil {
		// If we can't get updated sale info, still return success but with basic info
		log.Printf("Warning: Could not fetch updated sale info after payment creation: %v", err)
		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"payment": payment,
			"message": "Payment created successfully via Payment Management",
			"note":    "Payment created but updated sale info unavailable",
			"status":  "success",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"payment": payment,
		"updated_sale": gin.H{
			"id":                 updatedSale.ID,
			"status":             updatedSale.Status,
			"paid_amount":        updatedSale.PaidAmount,
			"outstanding_amount": updatedSale.OutstandingAmount,
		},
		"message": "Payment created successfully via Payment Management",
		"status":  "success",
	})
}

// GetSaleForPayment gets sale details formatted for payment creation
func (sc *SalesController) GetSaleForPayment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid sale ID"})
		return
	}

	sale, err := sc.salesServiceV2.GetSaleByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Sale not found"})
		return
	}

	// Format response for payment creation
	response := gin.H{
		"sale_id":        sale.ID,
		"invoice_number": sale.InvoiceNumber,
		"customer": gin.H{
			"id":   sale.Customer.ID,
			"name": sale.Customer.Name,
			"type": sale.Customer.Type,
		},
		"total_amount":           sale.TotalAmount,
		"paid_amount":            sale.PaidAmount,
		"outstanding_amount":     sale.OutstandingAmount,
		"status":                 sale.Status,
		"date":                   sale.Date.Format("2006-01-02"),
		"due_date":               sale.DueDate.Format("2006-01-02"),
		"can_receive_payment":    sale.Status == models.SaleStatusInvoiced || sale.Status == models.SaleStatusOverdue,
		"payment_url_suggestion": fmt.Sprintf("/api/sales/%d/integrated-payment", sale.ID),
	}

	c.JSON(http.StatusOK, response)
}

// Sales Returns

// CreateSaleReturn creates a return for a sale
func (sc *SalesController) CreateSaleReturn(c *gin.Context) {
	_, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid sale ID"})
		return
	}

	var request models.SaleReturnRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_ = c.MustGet("user_id").(uint)

	// Not implemented in V2 service yet
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Sale returns are not implemented in this build"})
}

// GetSaleReturns gets all returns
func (sc *SalesController) GetSaleReturns(c *gin.Context) {
	_, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	_, _ = strconv.Atoi(c.DefaultQuery("limit", "10"))

	c.JSON(http.StatusNotImplemented, gin.H{"error": "GetSaleReturns is not implemented in this build"})
}

// Reporting and Analytics

// GetSalesSummary gets sales summary statistics
func (sc *SalesController) GetSalesSummary(c *gin.Context) {
	log.Printf("📊 Getting sales summary with filters")

	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	// Parse dates if provided
	var startDate, endDate *time.Time
	if startDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", startDateStr); err == nil {
			startDate = &parsed
		} else {
			log.Printf("⚠️ Invalid start_date format: %s (expected: YYYY-MM-DD)", startDateStr)
			utils.SendValidationError(c, "Invalid date format", map[string]string{
				"start_date": "Date must be in YYYY-MM-DD format",
			})
			return
		}
	}
	if endDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", endDateStr); err == nil {
			endDate = &parsed
		} else {
			log.Printf("⚠️ Invalid end_date format: %s (expected: YYYY-MM-DD)", endDateStr)
			utils.SendValidationError(c, "Invalid date format", map[string]string{
				"end_date": "Date must be in YYYY-MM-DD format",
			})
			return
		}
	}

	log.Printf("📅 Date filters: start=%v, end=%v", startDate, endDate)

	// Get summary from service
	summary, err := sc.salesServiceV2.GetSalesSummary(startDate, endDate)
	if err != nil {
		log.Printf("❌ Failed to get sales summary: %v", err)
		utils.SendInternalError(c, "Failed to retrieve sales summary", err.Error())
		return
	}

	log.Printf("✅ Sales summary retrieved: %d sales, %.2f total", summary.TotalSales, summary.TotalAmount)
	utils.SendSuccess(c, "Sales summary retrieved successfully", summary)
}

// GetSalesAnalytics gets sales analytics data
func (sc *SalesController) GetSalesAnalytics(c *gin.Context) {
	_, _ = c.GetQuery("period")
	_, _ = c.GetQuery("year")

	c.JSON(http.StatusNotImplemented, gin.H{"error": "GetSalesAnalytics is not implemented in this build"})
}

// GetReceivablesReport gets accounts receivable report
func (sc *SalesController) GetReceivablesReport(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "GetReceivablesReport is not implemented in this build"})
}

// PDF Export

// ExportSaleInvoicePDF exports sale invoice as PDF
// Accepts optional query parameter: lang (en or id)
func (sc *SalesController) ExportSaleInvoicePDF(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid sale ID"})
		return
	}

	// Load sale and generate PDF via pdfService
	sale, err := sc.salesServiceV2.GetSaleByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Sale not found"})
		return
	}

	if sale.Status == "DRAFT" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Cannot generate invoice for draft sale",
			"message": "Please confirm the sale first before generating invoice",
		})
		return
	}

	if sale.Status == "CANCELLED" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Cannot generate invoice for cancelled sale",
			"message": "Cancelled sales should not be exported as active invoices",
		})
		return
	}

	// Get language from query parameter (defaults to user's language from settings)
	lang := c.Query("lang")
	var pdfBytes []byte
	var genErr error

	if lang != "" {
		// Use specified language
		pdfBytes, genErr = sc.pdfService.GenerateInvoicePDFWithLanguage(sale, lang)
	} else {
		// Use default language from settings
		pdfBytes, genErr = sc.pdfService.GenerateInvoicePDF(sale)
	}

	if genErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": genErr.Error()})
		return
	}
	filename := sale.InvoiceNumber
	if filename == "" {
		filename = sale.Code
	}
	filename = filename + ".pdf"
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}

// ExportSaleReceiptPDF exports a sales payment receipt as PDF (available when fully paid)
func (sc *SalesController) ExportSaleReceiptPDF(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid sale ID"})
		return
	}

	// Load sale details (with payments if available)
	sale, err := sc.salesServiceV2.GetSaleByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Sale not found"})
		return
	}

	// Validate that sale is fully paid
	if !(strings.EqualFold(sale.Status, models.SaleStatusPaid) || sale.OutstandingAmount == 0) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "Receipt can only be generated for fully paid sales",
			"sale_status": sale.Status,
		})
		return
	}

	// Get current user ID for signature
	userIDInterface, exists := c.Get("user_id")
	var userID uint = 0
	if exists {
		if uid, ok := userIDInterface.(uint); ok {
			userID = uid
		}
	}

	// Generate PDF with user context for proper signature
	var pdfBytes []byte
	var genErr error
	if userID > 0 {
		pdfBytes, genErr = sc.pdfService.GenerateReceiptPDFWithUser(sale, userID)
	} else {
		pdfBytes, genErr = sc.pdfService.GenerateReceiptPDF(sale)
	}
	if genErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": genErr.Error()})
		return
	}

	filename := sale.InvoiceNumber
	if filename == "" {
		filename = sale.Code
	}
	filename = filename + "_receipt.pdf"
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}

// ExportSalesReportPDF exports sales report as PDF
func (sc *SalesController) ExportSalesReportPDF(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	status := c.Query("status")
	customerID := c.Query("customer_id")
	search := c.Query("search")

	// If no dates provided, default to last 30 days to keep report size reasonable
	if startDate == "" && endDate == "" {
		end := time.Now()
		start := end.AddDate(0, 0, -30)
		startDate = start.Format("2006-01-02")
		endDate = end.Format("2006-01-02")
	}

	// Collect sales data via service and generate PDF
	filter := models.SalesFilter{StartDate: startDate, EndDate: endDate, Status: status, CustomerID: customerID, Search: search, Page: 1, Limit: 10000}
	res, err := sc.salesServiceV2.GetSales(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	pdfBytes, genErr := sc.pdfService.GenerateSalesReportPDF(res.Data, startDate, endDate)
	if genErr != nil {
		log.Printf("❌ Error generating sales report PDF (start=%s, end=%s): %v", startDate, endDate, genErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate sales report PDF", "details": genErr.Error()})
		return
	}
	filename := fmt.Sprintf("sales-report_%s_to_%s.pdf", startDate, endDate)
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}

// ExportSalesReportCSV exports sales report as CSV
func (sc *SalesController) ExportSalesReportCSV(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	// If no dates provided, default to last 30 days to keep report size reasonable
	if startDate == "" && endDate == "" {
		end := time.Now()
		start := end.AddDate(0, 0, -30)
		startDate = start.Format("2006-01-02")
		endDate = end.Format("2006-01-02")
	}

	// Collect sales data via service
	status := c.Query("status")
	customerID := c.Query("customer_id")
	search := c.Query("search")
	filter := models.SalesFilter{StartDate: startDate, EndDate: endDate, Status: status, CustomerID: customerID, Search: search, Page: 1, Limit: 10000}
	res, err := sc.salesServiceV2.GetSales(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	language := "id"
	if sc.pdfService != nil {
		language = sc.pdfService.Language()
	}
	loc := func(key, fallback string) string {
		t := utils.T(key, language)
		if t == key {
			return fallback
		}
		return t
	}
	// Header lines
	_ = w.Write([]string{loc("sales_report", "SALES REPORT")})
	_ = w.Write([]string{loc("period", "Period") + ":", startDate, loc("to", "to"), endDate})
	_ = w.Write([]string{loc("generated_on", "Generated on") + ":", time.Now().Format("2006-01-02 15:04:05")})
	_ = w.Write([]string{})

	// Table header
	_ = w.Write([]string{loc("date", "Date"), "Sale Code", loc("invoice_number", "Invoice No."), loc("customer", "Customer"), loc("type", "Type"), loc("status", "Status"), loc("amount", "Amount"), loc("paid", "Paid"), loc("outstanding", "Outstanding")})

	var totalAmount, totalPaid, totalOutstanding float64
	for _, sale := range res.Data {
		dateStr := ""
		if !sale.Date.IsZero() {
			dateStr = sale.Date.Format("2006-01-02")
		}
		invoice := sale.InvoiceNumber
		customerName := ""
		if sale.Customer.ID != 0 {
			customerName = sale.Customer.Name
		}
		_ = w.Write([]string{
			dateStr,
			sale.Code,
			invoice,
			customerName,
			sale.Type,
			sale.Status,
			fmt.Sprintf("%.2f", sale.TotalAmount),
			fmt.Sprintf("%.2f", sale.PaidAmount),
			fmt.Sprintf("%.2f", sale.OutstandingAmount),
		})
		totalAmount += sale.TotalAmount
		totalPaid += sale.PaidAmount
		totalOutstanding += sale.OutstandingAmount
	}

	// Totals row
	_ = w.Write([]string{strings.ToUpper(loc("total", "TOTAL")), "", "", "", "", "", fmt.Sprintf("%.2f", totalAmount), fmt.Sprintf("%.2f", totalPaid), fmt.Sprintf("%.2f", totalOutstanding)})
	_ = w.Write([]string{})

	// Summary section
	_ = w.Write([]string{strings.ToUpper(loc("sales_summary", "SUMMARY STATISTICS"))})
	_ = w.Write([]string{loc("total_sales", "Total Sales") + ":", fmt.Sprintf("%d", len(res.Data))})
	_ = w.Write([]string{loc("total_amount", "Total Amount") + ":", fmt.Sprintf("%.2f", totalAmount)})
	_ = w.Write([]string{loc("total_paid", "Total Paid") + ":", fmt.Sprintf("%.2f", totalPaid)})
	_ = w.Write([]string{loc("outstanding", "Outstanding") + ":", fmt.Sprintf("%.2f", totalOutstanding)})

	w.Flush()
	if err := w.Error(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate CSV", "details": err.Error()})
		return
	}

	filename := fmt.Sprintf("sales-report_%s_to_%s.csv", startDate, endDate)
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "text/csv", buf.Bytes())
}

// Customer Portal

// GetCustomerSales gets sales for a specific customer (for customer portal)
func (sc *SalesController) GetCustomerSales(c *gin.Context) {
	_, err := strconv.ParseUint(c.Param("customer_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	_, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	_, _ = strconv.Atoi(c.DefaultQuery("limit", "10"))

	c.JSON(http.StatusNotImplemented, gin.H{"error": "GetCustomerSales is not implemented in this build"})
}

// GetCustomerInvoices gets invoices for a specific customer
func (sc *SalesController) GetCustomerInvoices(c *gin.Context) {
	_, err := strconv.ParseUint(c.Param("customer_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	c.JSON(http.StatusNotImplemented, gin.H{"error": "GetCustomerInvoices is not implemented in this build"})
}
