package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"app-sistem-akuntansi/models"
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

	// Role check: if not finance or admin, save as DRAFT
	role := ""
	if r, exists := c.Get("role"); exists {
		if rs, ok := r.(string); ok {
			role = strings.ToLower(rs)
		}
	}
	isFinanceOrAdmin := role == "finance" || role == "admin"
	req.SaveAsDraft = !isFinanceOrAdmin

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

	// Role check: if not finance or admin, save as DRAFT
	role := ""
	if r, exists := c.Get("role"); exists {
		if rs, ok := r.(string); ok {
			role = strings.ToLower(rs)
		}
	}
	isFinanceOrAdmin := role == "finance" || role == "admin"
	req.SaveAsDraft = !isFinanceOrAdmin

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

	// Role check: if not finance or admin, save as DRAFT
	role := ""
	if r, exists := c.Get("role"); exists {
		if rs, ok := r.(string); ok {
			role = strings.ToLower(rs)
		}
	}
	isFinanceOrAdmin := role == "finance" || role == "admin"
	req.SaveAsDraft = !isFinanceOrAdmin

	payment, err := ctrl.enhancedPaymentService.CreateExpensePayment(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to create expense payment",
			"details": err.Error(),
		})
		return
	}

	statusCode := http.StatusCreated
	message := "Expense payment created successfully"
	if payment.Status == "DRAFT" {
		message = "Expense payment submitted as DRAFT — waiting for finance approval"
	}

	c.JSON(statusCode, gin.H{
		"message": message,
		"data":    payment,
	})
}

// ApproveExpensePayment approves a DRAFT expense payment (finance/admin only)
func (ctrl *SSOTPaymentController) ApproveExpensePayment(c *gin.Context) {
	paymentIDStr := c.Param("id")
	paymentID, err := strconv.ParseUint(paymentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment ID"})
		return
	}

	approverUserID := getSSOTUserIDFromContext(c)
	if approverUserID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	payment, err := ctrl.enhancedPaymentService.ApproveExpensePayment(uint(paymentID), approverUserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to approve expense payment",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Expense payment approved and journal posted successfully",
		"data":    payment,
	})
}

// ApproveStandardPayment approves a DRAFT receivable/payable payment (finance/admin only)
func (ctrl *SSOTPaymentController) ApproveStandardPayment(c *gin.Context) {
	paymentIDStr := c.Param("id")
	paymentID, err := strconv.ParseUint(paymentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment ID"})
		return
	}

	approverUserID := getSSOTUserIDFromContext(c)
	if approverUserID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	payment, err := ctrl.enhancedPaymentService.ApproveStandardPayment(uint(paymentID), approverUserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to approve standard payment",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Payment approved and journal posted successfully",
		"data":    payment,
	})
}

// UpdateDraftPayment updates a payment in DRAFT status
func (ctrl *SSOTPaymentController) UpdateDraftPayment(c *gin.Context) {
	paymentIDStr := c.Param("id")
	paymentID, err := strconv.ParseUint(paymentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment ID"})
		return
	}

	var reqData struct {
		ContactID          uint    `json:"contact_id"`
		ExpenseAccountID   *uint   `json:"expense_account_id"`
		CashBankID         uint    `json:"cash_bank_id"`
		Date               string  `json:"date"`
		Amount             float64 `json:"amount"`
		Method             string  `json:"method"`
		Reference          string  `json:"reference"`
		Notes              string  `json:"notes"`
		Description        string  `json:"description"`
		TargetBillID       *uint   `json:"target_bill_id"`
		TargetInvoiceID    *uint   `json:"target_invoice_id"`
	}

	if err := c.ShouldBindJSON(&reqData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	payment, err := ctrl.enhancedPaymentService.UpdateDraftPayment(uint(paymentID), func(p *models.Payment) error {
		// Update fields if provided
		if reqData.ContactID != 0 {
			p.ContactID = reqData.ContactID
		}
		if reqData.Date != "" {
			if parsedDate, err := time.Parse(time.RFC3339, reqData.Date); err == nil {
				p.Date = parsedDate
			} else if parsedDate, err := time.Parse("2006-01-02", reqData.Date); err == nil {
				p.Date = parsedDate
			}
		}
		if reqData.Amount > 0 {
			p.Amount = reqData.Amount
		}
		if reqData.Method != "" {
			p.Method = reqData.Method
		}
		// Combine notes and description
		if reqData.Notes != "" {
			p.Notes = reqData.Notes
		}
		if reqData.Description != "" {
			if p.Notes != "" {
				p.Notes += " - " + reqData.Description
			} else {
				p.Notes = reqData.Description
			}
		}
		
		// Parse and reconstruct reference for DRAFT approval mappings
		var existingExpID, existingCBID uint
		n, _ := fmt.Sscanf(p.Reference, "__EXP_ACC:%d,CB:%d__", &existingExpID, &existingCBID)
		if n != 2 {
			existingExpID = 0
			n, _ = fmt.Sscanf(p.Reference, "__CB:%d__", &existingCBID)
			if n != 1 {
				existingCBID = 0
			}
		}
		
		cleanRef := p.Reference
		if existingExpID != 0 && existingCBID != 0 {
			cleanRef = strings.Replace(cleanRef, fmt.Sprintf("__EXP_ACC:%d,CB:%d__", existingExpID, existingCBID), "", 1)
		} else if existingCBID != 0 {
			cleanRef = strings.Replace(cleanRef, fmt.Sprintf("__CB:%d__", existingCBID), "", 1)
		}

		newCBID := existingCBID
		if reqData.CashBankID != 0 {
			newCBID = reqData.CashBankID
		}
		newExpID := existingExpID
		if reqData.ExpenseAccountID != nil {
			newExpID = *reqData.ExpenseAccountID
		}
		
		userRef := cleanRef
		if reqData.Reference != "" {
			userRef = reqData.Reference
		}

		if newExpID != 0 {
			p.Reference = fmt.Sprintf("__EXP_ACC:%d,CB:%d__%s", newExpID, newCBID, userRef)
		} else if newCBID != 0 {
			p.Reference = fmt.Sprintf("__CB:%d__%s", newCBID, userRef)
		} else {
			p.Reference = userRef
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to update draft payment",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Draft payment updated successfully",
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
