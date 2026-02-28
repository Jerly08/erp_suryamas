package controllers

import (
	"net/http"
	"strconv"
	"time"

	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/services"

	"github.com/gin-gonic/gin"
)

// SSOTPaymentController handles payments using the Single Source of Truth posting service
type SSOTPaymentController struct {
	enhancedPaymentService *services.EnhancedPaymentServiceWithJournal
}

// NewSSOTPaymentController creates a new SSOT payment controller
func NewSSOTPaymentController(enhancedPaymentService *services.EnhancedPaymentServiceWithJournal) *SSOTPaymentController {
	return &SSOTPaymentController{
		enhancedPaymentService: enhancedPaymentService,
	}
}

// CreateReceivablePayment creates a customer payment with SSOT journal integration
func (ctrl *SSOTPaymentController) CreateReceivablePayment(c *gin.Context) {
	var req services.PaymentWithJournalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Set defaults for receivable payment
	req.Method = "RECEIVABLE"
	req.AutoCreateJournal = true

	// Get user ID from JWT context
	userID := getSSOTUserIDFromContext(c)
	req.UserID = userID

	// Process payment using SSOT enhanced service
	response, err := ctrl.enhancedPaymentService.CreatePaymentWithJournal(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create receivable payment",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Receivable payment created successfully",
		"data":    response,
	})
}

// CreatePayablePayment creates a vendor payment with SSOT journal integration
func (ctrl *SSOTPaymentController) CreatePayablePayment(c *gin.Context) {
	var req services.PaymentWithJournalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Set defaults for payable payment
	req.Method = "PAYABLE"
	req.AutoCreateJournal = true

	// Get user ID from JWT context
	userID := getSSOTUserIDFromContext(c)
	req.UserID = userID

	// Process payment using SSOT enhanced service
	response, err := ctrl.enhancedPaymentService.CreatePaymentWithJournal(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create payable payment",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Payable payment created successfully",
		"data":    response,
	})
}

// CreateExpensePayment creates a direct expense payment (from COA) with SSOT journal integration
func (ctrl *SSOTPaymentController) CreateExpensePayment(c *gin.Context) {
	var req services.ExpensePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Attach user ID from JWT context
	userID := getSSOTUserIDFromContext(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	req.UserID = userID

	payment, err := ctrl.enhancedPaymentService.CreateExpensePayment(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to create expense payment",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Expense payment created successfully",
		"data":    payment,
	})
}

// GetPaymentWithJournal retrieves a payment with its journal entry details
func (ctrl *SSOTPaymentController) GetPaymentWithJournal(c *gin.Context) {
	paymentIDStr := c.Param("id")
	paymentID, err := strconv.ParseUint(paymentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid payment ID",
		})
		return
	}

	response, err := ctrl.enhancedPaymentService.GetPaymentWithJournal(uint(paymentID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Payment not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": response,
	})
}

// ReversePayment reverses a payment and its journal entries
func (ctrl *SSOTPaymentController) ReversePayment(c *gin.Context) {
	paymentIDStr := c.Param("id")
	paymentID, err := strconv.ParseUint(paymentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid payment ID",
		})
		return
	}

	var request struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	userID := getSSOTUserIDFromContext(c)

	response, err := ctrl.enhancedPaymentService.ReversePayment(uint(paymentID), request.Reason, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to reverse payment",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Payment reversed successfully",
		"data":    response,
	})
}

// PreviewPaymentJournal previews what journal entry would be created for a payment
func (ctrl *SSOTPaymentController) PreviewPaymentJournal(c *gin.Context) {
	var req services.PaymentWithJournalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Get user ID from JWT context
	userID := getSSOTUserIDFromContext(c)
	req.UserID = userID

	// Preview journal entry
	preview, err := ctrl.enhancedPaymentService.PreviewPaymentJournal(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to preview journal entry",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Journal preview generated successfully",
		"data":    preview,
	})
}

// GetAccountBalanceUpdates retrieves account balance updates from a payment's journal entry
func (ctrl *SSOTPaymentController) GetAccountBalanceUpdates(c *gin.Context) {
	paymentIDStr := c.Param("id")
	paymentID, err := strconv.ParseUint(paymentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid payment ID",
		})
		return
	}

	updates, err := ctrl.enhancedPaymentService.GetAccountBalanceUpdates(uint(paymentID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Account balance updates not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": updates,
	})
}

// GetPayments retrieves payments list with filters (SSOT list)
func (ctrl *SSOTPaymentController) GetPayments(c *gin.Context) {
	// Parse query params
	var filter repositories.PaymentFilter

	if pageStr := c.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil {
			filter.Page = page
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = limit
		}
	}
	if contactIDStr := c.Query("contact_id"); contactIDStr != "" {
		if cid64, err := strconv.ParseUint(contactIDStr, 10, 32); err == nil {
			filter.ContactID = uint(cid64)
		}
	}
	if status := c.Query("status"); status != "" {
		filter.Status = status
	}
	if method := c.Query("method"); method != "" {
		filter.Method = method
	}
	if search := c.Query("search"); search != "" {
		filter.Search = search
	}
	if startDateStr := c.Query("start_date"); startDateStr != "" {
		if t, err := time.Parse("2006-01-02", startDateStr); err == nil {
			filter.StartDate = t
		}
	}
	if endDateStr := c.Query("end_date"); endDateStr != "" {
		if t, err := time.Parse("2006-01-02", endDateStr); err == nil {
			filter.EndDate = t
		}
	}

	result, err := ctrl.enhancedPaymentService.ListPayments(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch payments",
			"details": err.Error(),
		})
		return
	}

	// Return the paginated result directly as expected by frontend
	c.JSON(http.StatusOK, result)
}

// getSSOTUserIDFromContext extracts user ID from JWT context for SSOT payment controller
func getSSOTUserIDFromContext(c *gin.Context) uint {
	if userID, exists := c.Get("user_id"); exists {
		if id, ok := userID.(uint); ok {
			return id
		}
	}
	return 0
}
