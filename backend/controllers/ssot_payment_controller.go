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
	"gorm.io/gorm"
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

func (ctrl *SSOTPaymentController) getAccessiblePaymentResponse(c *gin.Context, paymentID uint) (*services.PaymentWithJournalResponse, bool) {
	response, err := ctrl.enhancedPaymentService.GetPaymentWithJournal(paymentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Payment not found",
			"details": err.Error(),
		})
		return nil, false
	}

	if models.IsStaffRole(c.GetString("role")) {
		userID := getSSOTUserIDFromContext(c)
		if response.Payment == nil || response.Payment.UserID != userID {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied: you can only view your own payments",
			})
			return nil, false
		}
	}

	return response, true
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
	req.AutoCreateJournal = true

	// Get user ID from JWT context
	userID := getSSOTUserIDFromContext(c)
	req.UserID = userID

	// Only finance manager can post immediately; everyone else goes through draft approval.
	role := ""
	if r, exists := c.Get("role"); exists {
		if rs, ok := r.(string); ok {
			role = strings.ToLower(rs)
		}
	}
	isFinanceManager := models.IsFinanceManagerRole(role)
	req.SaveAsDraft = !isFinanceManager
	req.RequestedByFinanceManager = isFinanceManager

	// Process payment using SSOT enhanced service
	response, err := ctrl.enhancedPaymentService.CreatePaymentWithJournal(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create receivable payment",
			"details": err.Error(),
		})
		return
	}

	message := "Receivable payment created successfully"
	if response != nil && response.Payment != nil {
		if response.Payment.Status == models.PaymentStatusDraft {
			message = "Receivable payment submitted for finance manager approval"
		} else if response.Payment.Status == models.PaymentStatusPending {
			message = "Receivable payment submitted for director approval"
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": message,
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
	req.AutoCreateJournal = true

	// Get user ID from JWT context
	userID := getSSOTUserIDFromContext(c)
	req.UserID = userID

	// Only finance manager can post immediately; everyone else goes through draft approval.
	role := ""
	if r, exists := c.Get("role"); exists {
		if rs, ok := r.(string); ok {
			role = strings.ToLower(rs)
		}
	}
	isFinanceManager := models.IsFinanceManagerRole(role)
	req.SaveAsDraft = !isFinanceManager
	req.RequestedByFinanceManager = isFinanceManager

	// Process payment using SSOT enhanced service
	response, err := ctrl.enhancedPaymentService.CreatePaymentWithJournal(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create payable payment",
			"details": err.Error(),
		})
		return
	}

	message := "Payable payment created successfully"
	if response != nil && response.Payment != nil {
		if response.Payment.Status == models.PaymentStatusDraft {
			message = "Payable payment submitted for finance manager approval"
		} else if response.Payment.Status == models.PaymentStatusPending {
			message = "Payable payment submitted for director approval"
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": message,
		"data":    response,
	})
}

// CreateExpensePayment creates a non-invoice payment request for the approval workflow.
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

	// Finance owns the payment facts only. Contra COA and PPh are deliberately
	// discarded here and can only be supplied by Accounting after FM approval.
	req.SaveAsDraft = true
	req.RequestedByFinanceManager = false
	req.ExpenseAccountID = 0
	req.PPHType = ""
	req.PPHRate = 0
	req.PPHAmount = 0
	req.AllowLiabilityOverpayment = false

	payment, err := ctrl.enhancedPaymentService.CreateExpensePayment(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to create expense payment",
			"details": err.Error(),
		})
		return
	}

	statusCode := http.StatusCreated
	message := "Pembayaran non-invoice tersimpan sebagai draft dan menunggu approval Finance Manager"

	c.JSON(statusCode, gin.H{
		"message": message,
		"data":    payment,
	})
}

// ApproveExpensePayment advances an expense payment through approval/accounting/posting stages.
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

	var request struct {
		RequireDirectorApproval bool `json:"require_director_approval"`
	}
	_ = c.ShouldBindJSON(&request)

	paymentResponse, ok := ctrl.getAccessiblePaymentResponse(c, uint(paymentID))
	if !ok {
		return
	}

	roleNorm := models.NormalizeRole(c.GetString("role"))
	payment := paymentResponse.Payment
	if payment == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Payment not found"})
		return
	}

	isDirectorApprovalStep := payment.Status == models.PaymentStatusPending &&
		payment.RequireDirectorApproval &&
		strings.EqualFold(payment.ApprovalStage, models.PaymentApprovalStageDirector)

	if isDirectorApprovalStep {
		if roleNorm != models.RoleDirector {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only Director can approve this payment at the director stage"})
			return
		}
	} else if !models.CanManageRequiredApprovals(roleNorm) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only Finance Manager can approve payments"})
		return
	}

	approvedPayment, err := ctrl.enhancedPaymentService.ApproveExpensePayment(uint(paymentID), approverUserID, request.RequireDirectorApproval)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to approve expense payment",
			"details": err.Error(),
		})
		return
	}

	message := "Expense payment reviewed by finance manager and journal posted successfully"
	if approvedPayment.Status == models.PaymentStatusPending {
		message = "Expense payment approved by finance manager and sent to director"
	} else if approvedPayment.Status == models.PaymentStatusPendingAccounting {
		message = "Expense payment approved and sent to accounting for COA review"
	} else if approvedPayment.Status == models.PaymentStatusPendingFinalReview {
		message = "Expense payment COA reviewed and sent to finance manager for journal posting"
	}

	c.JSON(http.StatusOK, gin.H{
		"message": message,
		"data":    approvedPayment,
	})
}

// ApproveStandardPayment advances a receivable/payable payment through approval/accounting/posting stages.
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

	var request struct {
		RequireDirectorApproval bool `json:"require_director_approval"`
	}
	_ = c.ShouldBindJSON(&request)

	paymentResponse, ok := ctrl.getAccessiblePaymentResponse(c, uint(paymentID))
	if !ok {
		return
	}

	roleNorm := models.NormalizeRole(c.GetString("role"))
	paymentRecord := paymentResponse.Payment
	if paymentRecord == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Payment not found"})
		return
	}

	isDirectorApprovalStep := paymentRecord.Status == models.PaymentStatusPending &&
		paymentRecord.RequireDirectorApproval &&
		strings.EqualFold(paymentRecord.ApprovalStage, models.PaymentApprovalStageDirector)

	if isDirectorApprovalStep {
		if roleNorm != models.RoleDirector {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only Director can approve this payment at the director stage"})
			return
		}
	} else if !models.CanManageRequiredApprovals(roleNorm) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only Finance Manager can approve payments"})
		return
	}

	payment, err := ctrl.enhancedPaymentService.ApproveStandardPayment(uint(paymentID), approverUserID, request.RequireDirectorApproval)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to approve standard payment",
			"details": err.Error(),
		})
		return
	}

	message := "Payment reviewed by finance manager and journal posted successfully"
	if payment != nil && payment.Payment != nil {
		switch payment.Payment.Status {
		case models.PaymentStatusPending:
			message = "Payment approved by finance manager and sent to director"
		case models.PaymentStatusPendingAccounting:
			message = "Payment approved and sent to accounting for COA review"
		case models.PaymentStatusPendingFinalReview:
			message = "Payment COA reviewed and sent to finance manager for journal posting"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": message,
		"data":    payment,
	})
}

// RejectPayment rejects a DRAFT payment request (finance manager only)
func (ctrl *SSOTPaymentController) RejectPayment(c *gin.Context) {
	paymentIDStr := c.Param("id")
	paymentID, err := strconv.ParseUint(paymentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment ID"})
		return
	}

	var request struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	approverUserID := getSSOTUserIDFromContext(c)
	if approverUserID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	paymentResponse, ok := ctrl.getAccessiblePaymentResponse(c, uint(paymentID))
	if !ok {
		return
	}

	roleNorm := models.NormalizeRole(c.GetString("role"))
	payment := paymentResponse.Payment
	if payment == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Payment not found"})
		return
	}

	isDirectorApprovalStep := payment.Status == models.PaymentStatusPending &&
		payment.RequireDirectorApproval &&
		strings.EqualFold(payment.ApprovalStage, models.PaymentApprovalStageDirector)

	if isDirectorApprovalStep {
		if roleNorm != models.RoleDirector {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only Director can reject this payment at the director stage"})
			return
		}
	} else if !models.CanManageRequiredApprovals(roleNorm) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only Finance Manager can reject payments"})
		return
	}

	rejectedPayment, err := ctrl.enhancedPaymentService.RejectPayment(uint(paymentID), approverUserID, request.Reason)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to reject payment",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Payment rejected successfully",
		"data":    rejectedPayment,
	})
}

// UpdateDraftPayment updates a payment before journal posting.
func (ctrl *SSOTPaymentController) UpdateDraftPayment(c *gin.Context) {
	paymentIDStr := c.Param("id")
	paymentID, err := strconv.ParseUint(paymentIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment ID"})
		return
	}

	var reqData struct {
		ContactID          uint                      `json:"contact_id"`
		ExpenseAccountID   *uint                     `json:"expense_account_id"`
		CashBankID         uint                      `json:"cash_bank_id"`
		Date               string                    `json:"date"`
		Amount             float64                   `json:"amount"`
		ExchangeRate       float64                   `json:"exchange_rate"`
		Method             string                    `json:"method"`
		Reference          string                    `json:"reference"`
		Notes              string                    `json:"notes"`
		Description        string                    `json:"description"`
		PPHType            string                    `json:"pph_type"`
		PPHRate            float64                   `json:"pph_rate"`
		PPHAmount          float64                   `json:"pph_amount"`
		AllowOverpayment   bool                      `json:"allow_liability_overpayment"`
		TargetBillID       *uint                     `json:"target_bill_id"`
		TargetInvoiceID    *uint                     `json:"target_invoice_id"`
		InvoiceAllocations []services.AllocationItem `json:"invoice_allocations"`
		BillAllocations    []services.AllocationItem `json:"bill_allocations"`
	}

	if err := c.ShouldBindJSON(&reqData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
		})
		return
	}
	paymentResponse, ok := ctrl.getAccessiblePaymentResponse(c, uint(paymentID))
	if !ok {
		return
	}

	actorUserID := getSSOTUserIDFromContext(c)
	if actorUserID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	roleNorm := models.NormalizeRole(c.GetString("role"))
	currentPayment := paymentResponse.Payment
	if currentPayment == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Payment not found"})
		return
	}
	if currentPayment.Status == models.PaymentStatusPendingAccounting &&
		roleNorm != models.RoleAdmin &&
		!models.IsAccountingRole(roleNorm) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only Accounting can set COA for this payment stage"})
		return
	}
	if currentPayment.Status == models.PaymentStatusPendingFinalReview &&
		roleNorm != models.RoleAdmin &&
		!models.IsFinanceManagerRole(roleNorm) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only Finance Manager can revise COA before journal posting"})
		return
	}
	if currentPayment.Status == models.PaymentStatusPendingAccounting &&
		currentPayment.PaymentType == models.PaymentTypeExpense &&
		currentPayment.ExpenseAccountID == nil {
		if reqData.ExpenseAccountID == nil || *reqData.ExpenseAccountID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "COA transaksi wajib dipilih oleh Accounting"})
			return
		}

		payment, err := ctrl.enhancedPaymentService.CompleteExpensePaymentAccountingReview(
			uint(paymentID),
			actorUserID,
			services.ExpensePaymentAccountingReviewRequest{
				ExpenseAccountID:          *reqData.ExpenseAccountID,
				PPHType:                   reqData.PPHType,
				PPHRate:                   reqData.PPHRate,
				PPHAmount:                 reqData.PPHAmount,
				AllowLiabilityOverpayment: reqData.AllowOverpayment,
			},
		)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Failed to complete accounting COA",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "COA kontra disimpan dan pembayaran dikirim ke Finance Manager untuk review akhir",
			"data":    payment,
		})
		return
	}

	payment, err := ctrl.enhancedPaymentService.UpdateDraftPayment(uint(paymentID), actorUserID, func(tx *gorm.DB, p *models.Payment) error {
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
		if reqData.ExchangeRate > 0 {
			p.ExchangeRate = reqData.ExchangeRate
		} else if p.PaymentType == models.PaymentTypeExpense {
			p.ExchangeRate = 1
		}
		if reqData.CashBankID != 0 {
			var cashBank models.CashBank
			if err := tx.Preload("Account").First(&cashBank, reqData.CashBankID).Error; err != nil {
				return fmt.Errorf("cash/bank account not found: %w", err)
			}
			p.CashBankID = &reqData.CashBankID
			p.CashBankCode = cashBank.Code
			p.CashBankName = cashBank.Name
			p.CashBankCurrency = cashBank.Currency
			if cashBank.AccountID != 0 {
				p.CashBankGLAccountID = &cashBank.AccountID
			}
			if cashBank.Account.ID != 0 {
				p.CashBankGLAccountCode = cashBank.Account.Code
				p.CashBankGLAccountName = cashBank.Account.Name
			}
		}
		if reqData.Method != "" {
			p.Method = reqData.Method
		}
		if p.PaymentType == models.PaymentTypeExpense {
			if reqData.ExpenseAccountID != nil {
				p.ExpenseAccountID = reqData.ExpenseAccountID
			}
			p.PPHType = reqData.PPHType
			p.PPHRate = reqData.PPHRate
			p.PPHAmount = reqData.PPHAmount
			p.NativeAmount = 0
			p.BookAmountIDR = 0
			p.ForexGainLossAmount = 0
			p.ForexGainLossType = ""
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
		if reqData.Reference != "" {
			p.Reference = reqData.Reference
		} else {
			p.Reference = models.StripInternalPaymentReference(p.Reference)
		}
		if p.PaymentType == models.PaymentTypeExpense {
			if reqData.AllowOverpayment {
				p.Reference = models.MarkExpenseLiabilityOverpaymentReference(p.Reference)
			} else {
				p.Reference = models.ClearExpenseLiabilityOverpaymentReference(p.Reference)
			}
		}

		if p.PaymentType != models.PaymentTypeExpense && !strings.HasPrefix(p.PaymentType, "TAX_PPN") {
			allocationReq := &services.PaymentWithJournalRequest{
				ContactID:          p.ContactID,
				TargetBillID:       reqData.TargetBillID,
				TargetInvoiceID:    reqData.TargetInvoiceID,
				InvoiceAllocations: reqData.InvoiceAllocations,
				BillAllocations:    reqData.BillAllocations,
			}
			if err := ctrl.enhancedPaymentService.SyncDraftStandardPaymentAllocations(tx, p, allocationReq); err != nil {
				return err
			}
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

	message := "Draft payment updated successfully"
	if payment.Status == models.PaymentStatusPendingFinalReview {
		message = "Payment COA updated and sent to finance manager for journal posting"
	}

	c.JSON(http.StatusOK, gin.H{
		"message": message,
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

	response, ok := ctrl.getAccessiblePaymentResponse(c, uint(paymentID))
	if !ok {
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
		Reason       string `json:"reason" binding:"required"`
		ReversalDate string `json:"reversal_date"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	var reversalDate time.Time
	if strings.TrimSpace(request.ReversalDate) != "" {
		reversalDate, err = time.Parse("2006-01-02", request.ReversalDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid reversal_date format. Use YYYY-MM-DD",
			})
			return
		}
	}

	userID := getSSOTUserIDFromContext(c)
	if _, ok := ctrl.getAccessiblePaymentResponse(c, uint(paymentID)); !ok {
		return
	}

	response, err := ctrl.enhancedPaymentService.ReversePayment(uint(paymentID), request.Reason, reversalDate, userID)
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

	if _, ok := ctrl.getAccessiblePaymentResponse(c, uint(paymentID)); !ok {
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
	if models.IsStaffRole(c.GetString("role")) {
		filter.UserID = getSSOTUserIDFromContext(c)
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
