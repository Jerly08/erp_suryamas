package controllers

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/services"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"strconv"
	"time"
)

type EnhancedPaymentController struct {
	db             *gorm.DB
	paymentService *services.PaymentService
	salesRepo      *repositories.SalesRepository
	purchaseRepo   *repositories.PurchaseRepository
	contactRepo    repositories.ContactRepository
	cashBankRepo   *repositories.CashBankRepository
}

func NewEnhancedPaymentController(
	db *gorm.DB,
	paymentService *services.PaymentService,
	salesRepo *repositories.SalesRepository,
	purchaseRepo *repositories.PurchaseRepository,
	contactRepo repositories.ContactRepository,
	cashBankRepo *repositories.CashBankRepository,
) *EnhancedPaymentController {
	return &EnhancedPaymentController{
		db:             db,
		paymentService: paymentService,
		salesRepo:      salesRepo,
		purchaseRepo:   purchaseRepo,
		contactRepo:    contactRepo,
		cashBankRepo:   cashBankRepo,
	}
}

// Enhanced Payment Request dengan auto-detection dan validation
type EnhancedPaymentRequest struct {
	// Core payment info
	ContactID  uint      `json:"contact_id" binding:"required"`
	CashBankID uint      `json:"cash_bank_id"`
	Date       time.Time `json:"date" binding:"required"`
	Amount     float64   `json:"amount" binding:"required,min=0.01"`
	Reference  string    `json:"reference"`
	Notes      string    `json:"notes"`

	// Auto-filled based on contact type (optional in request)
	Method string `json:"method,omitempty"`

	// Specific invoice/bill to allocate to (optional)
	TargetInvoiceID *uint `json:"target_invoice_id,omitempty"`
	TargetBillID    *uint `json:"target_bill_id,omitempty"`

	// Advanced options
	AutoAllocate     bool `json:"auto_allocate" default:"true"`
	SkipBalanceCheck bool `json:"skip_balance_check,omitempty"`
	ForceProcess     bool `json:"force_process,omitempty"`
}

// Enhanced Payment Response with full allocation details
type EnhancedPaymentResponse struct {
	Payment     models.Payment             `json:"payment"`
	Allocations []models.PaymentAllocation `json:"allocations"`
	CashBank    *models.CashBank           `json:"cash_bank,omitempty"`
	Contact     models.Contact             `json:"contact"`
	Summary     PaymentProcessingSummary   `json:"summary"`
	Warnings    []string                   `json:"warnings,omitempty"`
}

type PaymentProcessingSummary struct {
	TotalProcessed      float64 `json:"total_processed"`
	AllocatedAmount     float64 `json:"allocated_amount"`
	UnallocatedAmount   float64 `json:"unallocated_amount"`
	InvoicesUpdated     int     `json:"invoices_updated"`
	BillsUpdated        int     `json:"bills_updated"`
	CashBankUpdated     bool    `json:"cash_bank_updated"`
	JournalEntriesCount int     `json:"journal_entries_count"`
	ProcessingTime      string  `json:"processing_time"`
}

// 🎯 MAIN ENDPOINT: Record Enhanced Payment
func (ctrl *EnhancedPaymentController) RecordEnhancedPayment(c *gin.Context) {
	startTime := time.Now()

	var req EnhancedPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Get user ID from context
	userID := getUserIDFromContext(c)

	// 🔍 STEP 1: Enhanced Validation & Auto-Detection
	validatedReq, contact, warnings, err := ctrl.validateAndEnhanceRequest(req, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Request validation failed",
			"details": err.Error(),
		})
		return
	}

	// 🚀 STEP 2: Process Payment using Enhanced Service
	payment, allocations, summary, err := ctrl.processEnhancedPayment(validatedReq, userID, contact)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Payment processing failed",
			"details": err.Error(),
		})
		return
	}

	// 📊 STEP 3: Build Response
	response := EnhancedPaymentResponse{
		Payment:     *payment,
		Allocations: allocations,
		Contact:     *contact,
		Summary:     *summary,
		Warnings:    warnings,
	}

	// Add cash bank info if available (from form data)
	if formData, exists := c.Get("request_data"); exists {
		if data, ok := formData.(map[string]interface{}); ok {
			if cashBankID, exists := data["cash_bank_id"]; exists && cashBankID != nil {
				if id, ok := parseUintFromInterface(cashBankID); ok {
					if cashBank, err := ctrl.cashBankRepo.FindByID(id); err == nil {
						response.CashBank = cashBank
					}
				}
			}
		}
	}

	// Calculate processing time
	response.Summary.ProcessingTime = time.Since(startTime).String()

	c.JSON(http.StatusCreated, gin.H{
		"message": "Payment recorded successfully",
		"data":    response,
	})
}

// 🔍 Validate and enhance request with auto-detection
func (ctrl *EnhancedPaymentController) validateAndEnhanceRequest(req EnhancedPaymentRequest, userID uint) (*EnhancedPaymentRequest, *models.Contact, []string, error) {
	var warnings []string

	// Get contact info
	contact, err := ctrl.contactRepo.GetByID(req.ContactID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("contact not found: %v", err)
	}

	if !contact.IsActive {
		return nil, nil, nil, fmt.Errorf("contact %s is inactive", contact.Name)
	}

	// 🎯 AUTO-DETECT PAYMENT METHOD based on contact type
	if req.Method == "" {
		if contact.Type == "CUSTOMER" {
			req.Method = "RECEIVABLE"
		} else if contact.Type == "VENDOR" {
			req.Method = "PAYABLE"
		} else {
			return nil, nil, nil, fmt.Errorf("unsupported contact type: %s", contact.Type)
		}
		warnings = append(warnings, fmt.Sprintf("Auto-detected payment method: %s", req.Method))
	}

	// 🔒 VALIDATE METHOD vs CONTACT TYPE consistency
	if contact.Type == "CUSTOMER" && req.Method != "RECEIVABLE" {
		return nil, nil, nil, fmt.Errorf("customer payments must use RECEIVABLE method, not %s", req.Method)
	}
	if contact.Type == "VENDOR" && req.Method != "PAYABLE" {
		return nil, nil, nil, fmt.Errorf("vendor payments must use PAYABLE method, not %s", req.Method)
	}

	// 🎯 VALIDATE TARGET ALLOCATION consistency
	if req.TargetInvoiceID != nil && req.TargetBillID != nil {
		return nil, nil, nil, fmt.Errorf("cannot specify both target_invoice_id and target_bill_id")
	}

	if req.TargetInvoiceID != nil && contact.Type != "CUSTOMER" {
		return nil, nil, nil, fmt.Errorf("target_invoice_id can only be used for customer payments")
	}

	if req.TargetBillID != nil && contact.Type != "VENDOR" {
		return nil, nil, nil, fmt.Errorf("target_bill_id can only be used for vendor payments")
	}

	// Validate target invoice/bill exists and belongs to contact
	if req.TargetInvoiceID != nil {
		sale, err := ctrl.salesRepo.FindByID(*req.TargetInvoiceID)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("target invoice not found: %v", err)
		}
		if sale.CustomerID != req.ContactID {
			return nil, nil, nil, fmt.Errorf("target invoice does not belong to selected customer")
		}
		if sale.OutstandingAmount <= 0 {
			return nil, nil, nil, fmt.Errorf("target invoice is already fully paid")
		}
		if req.Amount > sale.OutstandingAmount {
			warnings = append(warnings, fmt.Sprintf("Payment amount (%.2f) exceeds invoice outstanding (%.2f). Will be adjusted.",
				req.Amount, sale.OutstandingAmount))
		}
	}

	if req.TargetBillID != nil {
		purchase, err := ctrl.purchaseRepo.FindByID(*req.TargetBillID)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("target bill not found: %v", err)
		}
		if purchase.VendorID != req.ContactID {
			return nil, nil, nil, fmt.Errorf("target bill does not belong to selected vendor")
		}
		if purchase.OutstandingAmount <= 0 {
			return nil, nil, nil, fmt.Errorf("target bill is already fully paid")
		}
		if req.Amount > purchase.OutstandingAmount {
			warnings = append(warnings, fmt.Sprintf("Payment amount (%.2f) exceeds bill outstanding (%.2f). Will be adjusted.",
				req.Amount, purchase.OutstandingAmount))
		}
	}

	// 🏦 AUTO-SELECT CASH/BANK if not specified
	if req.CashBankID == 0 {
		cashBankID, err := ctrl.autoSelectCashBank(req.Method, req.Amount)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to auto-select cash/bank account: %v", err)
		}
		req.CashBankID = cashBankID

		cashBank, _ := ctrl.cashBankRepo.FindByID(cashBankID)
		warnings = append(warnings, fmt.Sprintf("Auto-selected cash/bank account: %s", cashBank.Name))
	}

	return &req, contact, warnings, nil
}

// 🚀 Process enhanced payment with full integration
func (ctrl *EnhancedPaymentController) processEnhancedPayment(req *EnhancedPaymentRequest, userID uint, contact *models.Contact) (*models.Payment, []models.PaymentAllocation, *PaymentProcessingSummary, error) {
	summary := &PaymentProcessingSummary{
		TotalProcessed: req.Amount,
	}

	// Start transaction
	tx := ctrl.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 📝 STEP 1: Create payment record
	payment := &models.Payment{
		ContactID: req.ContactID,
		UserID:    userID,
		Date:      req.Date,
		Amount:    req.Amount,
		Method:    req.Method,
		Reference: req.Reference,
		Notes:     req.Notes,
		Status:    models.PaymentStatusCompleted,
	}

	// Generate unique code
	code, err := ctrl.generatePaymentCode(tx, req.Method)
	if err != nil {
		tx.Rollback()
		return nil, nil, nil, fmt.Errorf("failed to generate payment code: %v", err)
	}
	payment.Code = code

	if err := tx.Create(payment).Error; err != nil {
		tx.Rollback()
		return nil, nil, nil, fmt.Errorf("failed to create payment: %v", err)
	}

	// 🎯 STEP 2: Create smart allocations
	allocations, err := ctrl.createSmartAllocations(tx, payment, req, contact)
	if err != nil {
		tx.Rollback()
		return nil, nil, nil, fmt.Errorf("failed to create allocations: %v", err)
	}

	// Calculate allocation summary
	allocatedAmount := 0.0
	for _, alloc := range allocations {
		allocatedAmount += alloc.AllocatedAmount
		if alloc.InvoiceID != nil {
			summary.InvoicesUpdated++
		}
		if alloc.BillID != nil {
			summary.BillsUpdated++
		}
	}
	summary.AllocatedAmount = allocatedAmount
	summary.UnallocatedAmount = req.Amount - allocatedAmount

	// 💰 STEP 3: Update cash/bank balance
	if err := ctrl.updateCashBankBalance(tx, payment, req.CashBankID); err != nil {
		tx.Rollback()
		return nil, nil, nil, fmt.Errorf("failed to update cash/bank balance: %v", err)
	}
	summary.CashBankUpdated = true

	// 📊 STEP 4: Create journal entries
	journalCount, err := ctrl.createPaymentJournalEntries(tx, payment, allocations, contact)
	if err != nil {
		tx.Rollback()
		return nil, nil, nil, fmt.Errorf("failed to create journal entries: %v", err)
	}
	summary.JournalEntriesCount = journalCount

	// ✅ STEP 5: Finalize payment
	payment.Status = models.PaymentStatusCompleted
	if err := tx.Save(payment).Error; err != nil {
		tx.Rollback()
		return nil, nil, nil, fmt.Errorf("failed to finalize payment: %v", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, nil, nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	return payment, allocations, summary, nil
}

// 🎯 Create smart allocations based on contact type and targets
func (ctrl *EnhancedPaymentController) createSmartAllocations(tx *gorm.DB, payment *models.Payment, req *EnhancedPaymentRequest, contact *models.Contact) ([]models.PaymentAllocation, error) {
	var allocations []models.PaymentAllocation
	remainingAmount := payment.Amount

	// CUSTOMER PAYMENT - Allocate to invoices
	if contact.Type == "CUSTOMER" {
		if req.TargetInvoiceID != nil {
			// Specific invoice allocation
			allocation, allocated, err := ctrl.allocateToSpecificInvoice(tx, payment.ID, *req.TargetInvoiceID, remainingAmount)
			if err != nil {
				return nil, err
			}
			allocations = append(allocations, allocation)
			remainingAmount -= allocated
		} else if req.AutoAllocate && remainingAmount > 0 {
			// Auto-allocate to unpaid invoices
			autoAllocations, allocated, err := ctrl.autoAllocateToInvoices(tx, payment.ID, req.ContactID, remainingAmount)
			if err != nil {
				return nil, err
			}
			allocations = append(allocations, autoAllocations...)
			remainingAmount -= allocated
		}
	}

	// VENDOR PAYMENT - Allocate to bills
	if contact.Type == "VENDOR" {
		if req.TargetBillID != nil {
			// Specific bill allocation
			allocation, allocated, err := ctrl.allocateToSpecificBill(tx, payment.ID, *req.TargetBillID, remainingAmount)
			if err != nil {
				return nil, err
			}
			allocations = append(allocations, allocation)
			remainingAmount -= allocated
		} else if req.AutoAllocate && remainingAmount > 0 {
			// Auto-allocate to unpaid bills
			autoAllocations, allocated, err := ctrl.autoAllocateToBills(tx, payment.ID, req.ContactID, remainingAmount)
			if err != nil {
				return nil, err
			}
			allocations = append(allocations, autoAllocations...)
			remainingAmount -= allocated
		}
	}

	// Create generic allocation for remaining amount
	if remainingAmount > 0.01 {
		genericAllocation := models.PaymentAllocation{
			PaymentID:       uint64(payment.ID),
			AllocatedAmount: remainingAmount,
		}

		if err := tx.Create(&genericAllocation).Error; err != nil {
			return nil, fmt.Errorf("failed to create generic allocation: %v", err)
		}
		allocations = append(allocations, genericAllocation)
	}

	return allocations, nil
}

// Continue with helper functions...
func (ctrl *EnhancedPaymentController) allocateToSpecificInvoice(tx *gorm.DB, paymentID uint, invoiceID uint, maxAmount float64) (models.PaymentAllocation, float64, error) {
	// Get invoice with lock
	var sale models.Sale
	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&sale, invoiceID).Error; err != nil {
		return models.PaymentAllocation{}, 0, fmt.Errorf("invoice not found: %v", err)
	}

	// Calculate allocation amount
	allocAmount := maxAmount
	if allocAmount > sale.OutstandingAmount {
		allocAmount = sale.OutstandingAmount
	}

	// Create allocation
	allocation := models.PaymentAllocation{
		PaymentID:       uint64(paymentID),
		InvoiceID:       &invoiceID,
		AllocatedAmount: allocAmount,
	}

	if err := tx.Create(&allocation).Error; err != nil {
		return models.PaymentAllocation{}, 0, fmt.Errorf("failed to create invoice allocation: %v", err)
	}

	// Update sale outstanding
	newOutstanding := sale.OutstandingAmount - allocAmount
	status := sale.Status
	if newOutstanding <= 0.01 {
		newOutstanding = 0
		status = models.SaleStatusPaid
	}

	if err := tx.Model(&sale).Updates(map[string]interface{}{
		"outstanding_amount": newOutstanding,
		"status":             status,
	}).Error; err != nil {
		return models.PaymentAllocation{}, 0, fmt.Errorf("failed to update invoice outstanding: %v", err)
	}

	return allocation, allocAmount, nil
}

func (ctrl *EnhancedPaymentController) allocateToSpecificBill(tx *gorm.DB, paymentID uint, billID uint, maxAmount float64) (models.PaymentAllocation, float64, error) {
	// Get bill with lock
	var purchase models.Purchase
	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&purchase, billID).Error; err != nil {
		return models.PaymentAllocation{}, 0, fmt.Errorf("bill not found: %v", err)
	}

	// Calculate allocation amount
	allocAmount := maxAmount
	if allocAmount > purchase.OutstandingAmount {
		allocAmount = purchase.OutstandingAmount
	}

	// Create allocation
	allocation := models.PaymentAllocation{
		PaymentID:       uint64(paymentID),
		BillID:          &billID,
		AllocatedAmount: allocAmount,
	}

	if err := tx.Create(&allocation).Error; err != nil {
		return models.PaymentAllocation{}, 0, fmt.Errorf("failed to create bill allocation: %v", err)
	}

	// Update purchase outstanding
	newOutstanding := purchase.OutstandingAmount - allocAmount
	status := purchase.Status
	if newOutstanding <= 0.01 {
		newOutstanding = 0
		status = models.PurchaseStatusPaid
	}

	if err := tx.Model(&purchase).Updates(map[string]interface{}{
		"outstanding_amount": newOutstanding,
		"status":             status,
	}).Error; err != nil {
		return models.PaymentAllocation{}, 0, fmt.Errorf("failed to update bill outstanding: %v", err)
	}

	return allocation, allocAmount, nil
}

// Get user ID from context (implement according to your auth system)
func getUserIDFromContext(c *gin.Context) uint {
	// Replace with your actual auth implementation
	if userID, exists := c.Get("user_id"); exists {
		if id, ok := userID.(uint); ok {
			return id
		}
	}
	return 1 // Default user ID for testing
}

// Auto-select cash/bank account
func (ctrl *EnhancedPaymentController) autoSelectCashBank(method string, amount float64) (uint, error) {
	var cashBank models.CashBank

	accountType := "CASH"
	if method == "PAYABLE" && amount > 1000000 { // Large payments prefer bank
		accountType = "BANK"
	}

	// Find account with sufficient balance for outgoing payments
	query := ctrl.db.Where("type = ? AND is_active = ?", accountType, true)
	if method == "PAYABLE" {
		query = query.Where("balance >= ?", amount)
	}

	if err := query.Order("balance DESC").First(&cashBank).Error; err != nil {
		// Fallback to any active account
		if err := ctrl.db.Where("is_active = ?", true).Order("balance DESC").First(&cashBank).Error; err != nil {
			return 0, fmt.Errorf("no active cash/bank account found")
		}
	}

	return cashBank.ID, nil
}

// Additional helper methods would continue here...
// (generatePaymentCode, updateCashBankBalance, createPaymentJournalEntries, etc.)

func (ctrl *EnhancedPaymentController) generatePaymentCode(tx *gorm.DB, method string) (string, error) {
	prefix := "PAY"
	if method == "RECEIVABLE" {
		prefix = "RCV"
	}

	now := time.Now()
	year := now.Year()
	month := int(now.Month())

	// Get next sequence
	var maxSeq int
	tx.Model(&models.Payment{}).
		Where("EXTRACT(YEAR FROM date) = ? AND EXTRACT(MONTH FROM date) = ? AND code LIKE ?",
			year, month, prefix+"-%").
		Select("COALESCE(MAX(CAST(SUBSTRING(code FROM '[0-9]+$') AS INTEGER)), 0)").
		Scan(&maxSeq)

	seq := maxSeq + 1
	return fmt.Sprintf("%s-%04d/%02d/%04d", prefix, year, month, seq), nil
}

func (ctrl *EnhancedPaymentController) updateCashBankBalance(tx *gorm.DB, payment *models.Payment, cashBankID uint) error {
	if cashBankID == 0 {
		return nil
	}

	var cashBank models.CashBank
	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&cashBank, cashBankID).Error; err != nil {
		return fmt.Errorf("cash/bank account not found: %v", err)
	}

	// Calculate amount (positive for incoming, negative for outgoing)
	amount := payment.Amount
	if payment.Method == "PAYABLE" {
		amount = -payment.Amount
	}

	newBalance := cashBank.Balance + amount
	cashBank.Balance = newBalance

	if err := tx.Save(&cashBank).Error; err != nil {
		return fmt.Errorf("failed to update balance: %v", err)
	}

	// Create transaction record (if CashBankTransaction model exists)
	// TODO: Implement CashBankTransaction model if needed

	return nil
}

// Placeholder implementations for remaining helper methods
func (ctrl *EnhancedPaymentController) autoAllocateToInvoices(tx *gorm.DB, paymentID uint, contactID uint, maxAmount float64) ([]models.PaymentAllocation, float64, error) {
	// Implementation for auto-allocating to customer invoices
	return []models.PaymentAllocation{}, 0, nil
}

func (ctrl *EnhancedPaymentController) autoAllocateToBills(tx *gorm.DB, paymentID uint, contactID uint, maxAmount float64) ([]models.PaymentAllocation, float64, error) {
	// Implementation for auto-allocating to vendor bills
	return []models.PaymentAllocation{}, 0, nil
}

func (ctrl *EnhancedPaymentController) createPaymentJournalEntries(tx *gorm.DB, payment *models.Payment, allocations []models.PaymentAllocation, contact *models.Contact) (int, error) {
	// Implementation for creating journal entries
	return 1, nil
}

// parseUintFromInterface converts interface{} to uint
func parseUintFromInterface(value interface{}) (uint, bool) {
	switch v := value.(type) {
	case uint:
		return v, true
	case int:
		if v >= 0 {
			return uint(v), true
		}
	case float64:
		if v >= 0 {
			return uint(v), true
		}
	case string:
		if parsed, err := strconv.ParseUint(v, 10, 64); err == nil {
			return uint(parsed), true
		}
	}
	return 0, false
}
