package controllers

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/services"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PaymentController struct {
	paymentService *services.PaymentService
}

func NewPaymentController(paymentService *services.PaymentService) *PaymentController {
	return &PaymentController{
		paymentService: paymentService,
	}
}

func restrictPaymentToOwnData(ctx *gin.Context) bool {
	return models.IsStaffRole(ctx.GetString("role"))
}

func paymentScopedUserID(ctx *gin.Context) uint {
	if restrictPaymentToOwnData(ctx) {
		return ctx.GetUint("user_id")
	}
	return 0
}

// GetPayments godoc
// @Summary [DEPRECATED] Get payments list
// @Description DEPRECATED: This endpoint may cause double posting. Use SSOT Payment routes instead. Get paginated list of payments with filters
// @Tags Deprecated-Payments
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Param contact_id query int false "Filter by contact ID"
// @Param status query string false "Filter by status"
// @Param method query string false "Filter by payment method"
// @Param start_date query string false "Start date (YYYY-MM-DD)"
// @Param end_date query string false "End date (YYYY-MM-DD)"
// @Success 200 {object} models.APIResponse
// @Router /api/payments [get]
// @deprecated
func (c *PaymentController) GetPayments(ctx *gin.Context) {
	// Parse query parameters
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "10"))
	contactID, _ := strconv.Atoi(ctx.Query("contact_id"))

	var startDate, endDate time.Time
	if sd := ctx.Query("start_date"); sd != "" {
		startDate, _ = time.Parse("2006-01-02", sd)
	}
	if ed := ctx.Query("end_date"); ed != "" {
		endDate, _ = time.Parse("2006-01-02", ed)
	}

	filter := repositories.PaymentFilter{
		ContactID: uint(contactID),
		StartDate: startDate,
		EndDate:   endDate,
		Status:    ctx.Query("status"),
		Method:    ctx.Query("method"),
		Search:    ctx.Query("search"),
		Page:      page,
		Limit:     limit,
	}
	if restrictPaymentToOwnData(ctx) {
		filter.UserID = ctx.GetUint("user_id")
	}

	result, err := c.paymentService.GetPayments(filter)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve payments",
			"details": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, result)
}

// GetPaymentByID godoc
// @Summary [DEPRECATED] Get payment by ID
// @Description DEPRECATED: This endpoint may cause double posting. Use SSOT Payment routes instead. Get single payment details
// @Tags Deprecated-Payments
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Payment ID"
// @Success 200 {object} models.Payment
// @Router /api/payments/{id} [get]
// @deprecated
func (c *PaymentController) GetPaymentByID(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid payment ID",
		})
		return
	}

	payment, err := c.paymentService.GetPaymentByID(uint(id))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"error": "Payment not found",
		})
		return
	}
	if restrictPaymentToOwnData(ctx) && payment.UserID != ctx.GetUint("user_id") {
		ctx.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied: you can only view your own payments",
		})
		return
	}

	ctx.JSON(http.StatusOK, payment)
}

// CreateReceivablePayment godoc
// @Summary [DEPRECATED] Create receivable payment
// @Description DEPRECATED: This endpoint may cause double posting. Use SSOT Payment routes instead. Create payment from customer (receivable)
// @Tags Deprecated-Payments
// @Accept json
// @Produce json
// @Security Bearer
// @Param payment body services.PaymentCreateRequest true "Payment data"
// @Success 201 {object} models.Payment
// @Router /api/payments/receivable [post]
// @deprecated
func (c *PaymentController) CreateReceivablePayment(ctx *gin.Context) {
	var request services.PaymentCreateRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	// Get user ID from context (set by auth middleware)
	userID := ctx.GetUint("user_id")
	if userID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	payment, err := c.paymentService.CreateReceivablePayment(request, userID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to create payment",
			"details": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusCreated, payment)
}

// CreatePayablePayment godoc
// @Summary [DEPRECATED] Create payable payment
// @Description DEPRECATED: This endpoint may cause double posting. Use SSOT Payment routes instead. Create payment to vendor (payable)
// @Tags Deprecated-Payments
// @Accept json
// @Produce json
// @Security Bearer
// @Param payment body services.PaymentCreateRequest true "Payment data"
// @Success 201 {object} models.Payment
// @Router /api/payments/payable [post]
// @deprecated
func (c *PaymentController) CreatePayablePayment(ctx *gin.Context) {
	var request services.PaymentCreateRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	// Get user ID from context
	userID := ctx.GetUint("user_id")
	if userID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	payment, err := c.paymentService.CreatePayablePayment(request, userID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to create payment",
			"details": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusCreated, payment)
}

// CancelPayment godoc
// @Summary Cancel payment
// @Description Cancel payment and reverse journal entries
// @Tags Payments
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Payment ID"
// @Param reason body map[string]string true "Cancellation reason"
// @Success 200 {object} map[string]string
// @Router /api/payments/{id}/cancel [post]
func (c *PaymentController) CancelPayment(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid payment ID",
		})
		return
	}

	var request struct {
		Reason string `json:"reason" binding:"required"`
	}

	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	userID := ctx.GetUint("user_id")
	if restrictPaymentToOwnData(ctx) {
		payment, paymentErr := c.paymentService.GetPaymentByID(uint(id))
		if paymentErr != nil {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Payment not found"})
			return
		}
		if payment.UserID != userID {
			ctx.JSON(http.StatusForbidden, gin.H{"error": "Access denied: you can only cancel your own payments"})
			return
		}
	}

	err = c.paymentService.CancelPayment(uint(id), request.Reason, userID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to cancel payment",
			"details": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Payment cancelled successfully",
	})
}

// DeletePayment godoc
// @Summary Delete payment
// @Description Delete payment and reverse if needed (Admin Only)
// @Tags Payments
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Payment ID"
// @Param reason body map[string]string true "Deletion reason"
// @Success 200 {object} map[string]string
// @Router /api/payments/{id} [delete]
func (c *PaymentController) DeletePayment(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid payment ID",
		})
		return
	}

	var request struct {
		Reason string `json:"reason"`
	}

	if err := ctx.ShouldBindJSON(&request); err != nil {
		// If no JSON body, use default reason
		request.Reason = "Deleted by admin"
	}

	userID := ctx.GetUint("user_id")
	if userID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}
	if restrictPaymentToOwnData(ctx) {
		payment, paymentErr := c.paymentService.GetPaymentByID(uint(id))
		if paymentErr != nil {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Payment not found"})
			return
		}
		if payment.UserID != userID {
			ctx.JSON(http.StatusForbidden, gin.H{"error": "Access denied: you can only delete your own payments"})
			return
		}
	}

	err = c.paymentService.DeletePayment(uint(id), request.Reason, userID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":   err.Error(),
			"details": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Payment deleted successfully",
	})
}

// GetUnpaidInvoices godoc
// @Summary Get unpaid invoices
// @Description Get list of unpaid invoices for a customer
// @Tags Payments
// @Accept json
// @Produce json
// @Security Bearer
// @Param customer_id path int true "Customer ID"
// @Success 200 {array} models.Sale
// @Router /api/payments/unpaid-invoices/{customer_id} [get]
func (c *PaymentController) GetUnpaidInvoices(ctx *gin.Context) {
	customerID, err := strconv.ParseUint(ctx.Param("customer_id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid customer ID",
		})
		return
	}

	// Get unpaid invoices from payment service
	invoices, err := c.paymentService.GetUnpaidInvoices(uint(customerID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve unpaid invoices",
			"details": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, invoices)
}

// GetUnpaidBills godoc
// @Summary Get unpaid bills
// @Description Get list of unpaid bills for a vendor
// @Tags Payments
// @Accept json
// @Produce json
// @Security Bearer
// @Param vendor_id path int true "Vendor ID"
// @Success 200 {array} models.Purchase
// @Router /api/payments/unpaid-bills/{vendor_id} [get]
func (c *PaymentController) GetUnpaidBills(ctx *gin.Context) {
	vendorID, err := strconv.ParseUint(ctx.Param("vendor_id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid vendor ID",
		})
		return
	}

	// Get unpaid bills from payment service
	bills, err := c.paymentService.GetUnpaidBills(uint(vendorID), paymentScopedUserID(ctx))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve unpaid bills",
			"details": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, bills)
}

// GetPaymentSummary godoc
// @Summary Get payment summary
// @Description Get payment summary statistics
// @Tags Payments
// @Accept json
// @Produce json
// @Security Bearer
// @Param start_date query string true "Start date (YYYY-MM-DD)"
// @Param end_date query string true "End date (YYYY-MM-DD)"
// @Success 200 {object} map[string]interface{}
// @Router /api/payments/summary [get]
func (c *PaymentController) GetPaymentSummary(ctx *gin.Context) {
	startDate := ctx.Query("start_date")
	endDate := ctx.Query("end_date")

	if startDate == "" || endDate == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Start date and end date are required",
		})
		return
	}

	// Get payment summary from service
	summary, err := c.paymentService.GetPaymentSummary(startDate, endDate, paymentScopedUserID(ctx))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve payment summary",
			"details": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, summary)
}

// GetPaymentAnalytics godoc
// @Summary Get payment analytics
// @Description Get detailed payment analytics for dashboard
// @Tags Payments
// @Accept json
// @Produce json
// @Security Bearer
// @Param start_date query string true "Start date (YYYY-MM-DD)"
// @Param end_date query string true "End date (YYYY-MM-DD)"
// @Success 200 {object} map[string]interface{}
// @Router /api/payments/analytics [get]
func (c *PaymentController) GetPaymentAnalytics(ctx *gin.Context) {
	startDate := ctx.Query("start_date")
	endDate := ctx.Query("end_date")

	if startDate == "" || endDate == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Start date and end date are required",
		})
		return
	}

	// Get payment analytics from service
	analytics, err := c.paymentService.GetPaymentAnalytics(startDate, endDate, paymentScopedUserID(ctx))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve payment analytics",
			"details": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, analytics)
}

// ExportPaymentReportPDF exports payment report as PDF
// @Summary Export payment report as PDF
// @Description Export payments report in PDF format with filters
// @Tags Payments
// @Accept json
// @Produce application/pdf
// @Security Bearer
// @Param start_date query string false "Start date (YYYY-MM-DD)"
// @Param end_date query string false "End date (YYYY-MM-DD)"
// @Param status query string false "Filter by status"
// @Param method query string false "Filter by payment method"
// @Success 200 {file} application/pdf
// @Router /api/payments/report/pdf [get]
func (c *PaymentController) ExportPaymentReportPDF(ctx *gin.Context) {
	startDate := ctx.Query("start_date")
	endDate := ctx.Query("end_date")
	status := ctx.Query("status")
	method := ctx.Query("method")

	pdfData, filename, err := c.paymentService.ExportPaymentReportPDF(startDate, endDate, status, method, paymentScopedUserID(ctx))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.Header("Content-Type", "application/pdf")
	ctx.Header("Content-Disposition", "attachment; filename="+filename)
	ctx.Data(http.StatusOK, "application/pdf", pdfData)
}

// ExportPaymentDetailPDF exports single payment detail as PDF
// @Summary Export payment detail as PDF
// @Description Export single payment details in PDF format
// @Tags Payments
// @Accept json
// @Produce application/pdf
// @Security Bearer
// @Param id path int true "Payment ID"
// @Success 200 {file} application/pdf
// @Router /api/payments/{id}/pdf [get]
func (c *PaymentController) ExportPaymentDetailPDF(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment ID"})
		return
	}

	if restrictPaymentToOwnData(ctx) {
		payment, paymentErr := c.paymentService.GetPaymentByID(uint(id))
		if paymentErr != nil {
			if errors.Is(paymentErr, gorm.ErrRecordNotFound) {
				ctx.JSON(http.StatusNotFound, gin.H{"error": "Payment not found"})
				return
			}
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load payment", "details": paymentErr.Error()})
			return
		}
		if payment.UserID != ctx.GetUint("user_id") {
			ctx.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied: you can only view your own payments",
			})
			return
		}
	}

	pdfData, filename, err := c.paymentService.ExportPaymentDetailPDF(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Payment not found"})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate PDF", "details": err.Error()})
		return
	}

	ctx.Header("Content-Type", "application/pdf")
	ctx.Header("Content-Disposition", "attachment; filename="+filename)
	ctx.Data(http.StatusOK, "application/pdf", pdfData)
}

// ExportPaymentReportExcel exports payment report as Excel
// @Summary Export payment report as Excel
// @Description Export payments report in Excel format with filters
// @Tags Payments
// @Accept json
// @Produce application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Security Bearer
// @Param start_date query string false "Start date (YYYY-MM-DD)"
// @Param end_date query string false "End date (YYYY-MM-DD)"
// @Param status query string false "Filter by status"
// @Param method query string false "Filter by payment method"
// @Success 200 {file} application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Router /api/payments/export/excel [get]
func (c *PaymentController) ExportPaymentReportExcel(ctx *gin.Context) {
	startDate := ctx.Query("start_date")
	endDate := ctx.Query("end_date")
	status := ctx.Query("status")
	method := ctx.Query("method")

	excelData, filename, err := c.paymentService.ExportPaymentReportExcel(startDate, endDate, status, method, paymentScopedUserID(ctx))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	ctx.Header("Content-Disposition", "attachment; filename="+filename)
	ctx.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", excelData)
}

// Sales Integration endpoints

// CreateSalesPayment creates payment specifically for sales invoices with pre-filled data
func (c *PaymentController) CreateSalesPayment(ctx *gin.Context) {
	var request struct {
		SaleID     uint      `json:"sale_id" binding:"required"`
		Amount     float64   `json:"amount" binding:"required,min=0"`
		Date       time.Time `json:"date" binding:"required"`
		Method     string    `json:"method" binding:"required"`
		CashBankID uint      `json:"cash_bank_id" binding:"required"`
		Reference  string    `json:"reference"`
		Notes      string    `json:"notes"`
	}

	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	// First get sale details to validate and get customer ID
	sale, err := c.paymentService.GetSaleByID(request.SaleID) // Need to add this method
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"error":   "Sale not found",
			"details": err.Error(),
		})
		return
	}

	// Get user ID from context
	userID := ctx.GetUint("user_id")
	if userID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	// Create payment request for receivables
	paymentRequest := services.PaymentCreateRequest{
		ContactID:  sale.CustomerID,
		CashBankID: request.CashBankID,
		Date:       request.Date,
		Amount:     request.Amount,
		Method:     request.Method,
		Reference:  request.Reference,
		Notes:      fmt.Sprintf("Payment for Invoice %s - %s", sale.InvoiceNumber, request.Notes),
		Allocations: []services.InvoiceAllocation{
			{
				InvoiceID: request.SaleID,
				Amount:    request.Amount,
			},
		},
	}

	payment, err := c.paymentService.CreateReceivablePayment(paymentRequest, userID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to create payment",
			"details": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"payment": payment,
		"message": "Payment created successfully for sales invoice",
	})
}

// GetSalesUnpaidInvoices gets unpaid invoices for sales payment creation
func (c *PaymentController) GetSalesUnpaidInvoices(ctx *gin.Context) {
	customerID, err := strconv.ParseUint(ctx.Param("customer_id"), 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid customer ID",
		})
		return
	}

	// Get unpaid invoices from payment service
	invoices, err := c.paymentService.GetUnpaidInvoices(uint(customerID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve unpaid invoices",
			"details": err.Error(),
		})
		return
	}

	// Format for frontend consumption
	response := gin.H{
		"customer_id": customerID,
		"invoices":    invoices,
		"count":       len(invoices),
	}

	ctx.JSON(http.StatusOK, response)
}

// GetRecentPayments godoc
// @Summary Get recent payments for debugging integration
// @Description Get recent payments to verify Sales-Payment integration
// @Tags Payments
// @Accept json
// @Produce json
// @Security Bearer
// @Param hours query int false "Hours back to check (default: 24)"
// @Success 200 {object} map[string]interface{}
// @Router /api/payments/debug/recent [get]
func (c *PaymentController) GetRecentPayments(ctx *gin.Context) {
	hours := 24 // Default to last 24 hours
	if h := ctx.Query("hours"); h != "" {
		if parsed, err := strconv.Atoi(h); err == nil && parsed > 0 {
			hours = parsed
		}
	}

	// Get all recent payments without filters
	filter := repositories.PaymentFilter{
		Page:  1,
		Limit: 50, // Get up to 50 recent payments
	}
	if restrictPaymentToOwnData(ctx) {
		filter.UserID = ctx.GetUint("user_id")
	}

	result, err := c.paymentService.GetPayments(filter)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve payments",
			"details": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message":        fmt.Sprintf("Recent payments from last %d hours", hours),
		"total_payments": result.Total,
		"payments":       result.Data,
		"debug_info": gin.H{
			"query_time":     time.Now().Format("2006-01-02 15:04:05"),
			"filter_applied": filter,
		},
	})
}
