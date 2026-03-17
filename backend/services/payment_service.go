package services

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type PaymentService struct {
	db                            *gorm.DB
	paymentRepo                   *repositories.PaymentRepository
	salesRepo                     *repositories.SalesRepository
	purchaseRepo                  *repositories.PurchaseRepository
	cashBankRepo                  *repositories.CashBankRepository
	accountRepo                   repositories.AccountRepository
	contactRepo                   repositories.ContactRepository
	statusValidator               *StatusValidationHelper        // NEW: Konsistensi dengan SalesJournalServiceV2
	purchasePaymentJournalService *PurchasePaymentJournalService // NEW: SSOT payment journal integration
}

func NewPaymentService(
	db *gorm.DB,
	paymentRepo *repositories.PaymentRepository,
	salesRepo *repositories.SalesRepository,
	purchaseRepo *repositories.PurchaseRepository,
	cashBankRepo *repositories.CashBankRepository,
	accountRepo repositories.AccountRepository,
	contactRepo repositories.ContactRepository,
	purchasePaymentJournalService *PurchasePaymentJournalService,
) *PaymentService {
	return &PaymentService{
		db:                            db,
		paymentRepo:                   paymentRepo,
		salesRepo:                     salesRepo,
		purchaseRepo:                  purchaseRepo,
		cashBankRepo:                  cashBankRepo,
		accountRepo:                   accountRepo,
		contactRepo:                   contactRepo,
		statusValidator:               NewStatusValidationHelper(),   // Initialize status validator
		purchasePaymentJournalService: purchasePaymentJournalService, // NEW: SSOT payment journal integration
	}
}

// Payment Types
const (
	PaymentTypeReceivable = "RECEIVABLE" // Payment from customer
	PaymentTypePayable    = "PAYABLE"    // Payment to vendor
	PaymentTypeAdvance    = "ADVANCE"    // Advance payment
	PaymentTypeRefund     = "REFUND"     // Refund payment
)

// CreateReceivablePayment creates payment for sales/receivables (Fixed version)
func (s *PaymentService) CreateReceivablePayment(request PaymentCreateRequest, userID uint) (*models.Payment, error) {
	// Use the fixed version with better logging and error handling
	return s.CreateReceivablePaymentFixed(request, userID)
}

// CreateReceivablePaymentFixed - Fixed version with better error handling and timeout
func (s *PaymentService) CreateReceivablePaymentFixed(request PaymentCreateRequest, userID uint) (*models.Payment, error) {
	startTime := time.Now()
	// Input validation
	if request.ContactID == 0 {
		return nil, fmt.Errorf("contact_id is required")
	}
	if request.Amount <= 0 {
		return nil, fmt.Errorf("amount must be greater than zero, got: %.2f", request.Amount)
	}
	if request.Method == "" {
		return nil, fmt.Errorf("payment method is required")
	}
	if request.Date.IsZero() {
		return nil, fmt.Errorf("payment date is required")
	}
	if request.Date.After(time.Now()) {
		return nil, fmt.Errorf("payment date cannot be in the future")
	}
	if err := CheckClosedPeriod(s.db, request.Date); err != nil {
		return nil, err
	}

	log.Printf("🚀 Starting CreateReceivablePayment: ContactID=%d, Amount=%.2f, Allocations=%d",
		request.ContactID, request.Amount, len(request.Allocations))

	// Start transaction with proper timeout and cancellation handling
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second) // Reduced from 2 minutes to 90 seconds
	defer cancel()

	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("context cancelled before transaction start: %v", ctx.Err())
	default:
		// Continue with transaction
	}

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %v", tx.Error)
	}

	var selectedCashBank *models.CashBank
	cashBankMutation := &cashBankBalanceMutation{
		NativeAmount:    request.Amount,
		BookAmountIDR:   request.Amount,
		Currency:        "IDR",
		ExchangeRate:    1,
		TransactionDate: request.Date,
		Notes:           "Customer payment receipt",
	}

	if request.CashBankID > 0 {
		var cashBank models.CashBank
		if err := tx.First(&cashBank, request.CashBankID).Error; err != nil {
			return nil, fmt.Errorf("cash/bank account not found: %v", err)
		}

		normalizedCurrency := normalizeCashBankCurrency(cashBank.Currency)
		cashBankMutation.Currency = normalizedCurrency

		if normalizedCurrency != "IDR" && normalizedCurrency != "USD" {
			return nil, fmt.Errorf("sales receivable payment currently only supports IDR and USD cash/bank accounts")
		}

		if request.ExchangeRate > 0 {
			cashBankMutation.ExchangeRate = request.ExchangeRate
		}

		if normalizedCurrency == "USD" {
			if request.ExchangeRate <= 0 {
				return nil, fmt.Errorf("exchange rate is required when receiving payment into USD cash/bank account")
			}

			cashBankMutation.ExchangeRate = request.ExchangeRate
			cashBankMutation.NativeAmount = request.Amount / request.ExchangeRate
			cashBankMutation.BookAmountIDR = request.Amount
		}

		selectedCashBank = &cashBank
	}

	// Robust transaction handling with automatic rollback
	var committed bool
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ PANIC in CreateReceivablePayment: %v", r)
			if !committed {
				if rbErr := tx.Rollback().Error; rbErr != nil {
					log.Printf("❌ CRITICAL: Failed to rollback after panic: %v", rbErr)
				}
			}
			// Don't re-panic, return error instead via channel or global error var
			log.Printf("❌ PANIC handled, transaction rolled back")
		} else if !committed {
			// Auto-rollback if not committed
			if rbErr := tx.Rollback().Error; rbErr != nil {
				log.Printf("❌ WARNING: Auto-rollback failed: %v", rbErr)
			}
		}
	}()

	// Step 1: Validate customer
	log.Printf("📝 Step 1: Validating customer...")
	stepStart := time.Now()
	_, err := s.contactRepo.GetByID(request.ContactID)
	if err != nil {
		log.Printf("❌ Customer validation failed: %v (%.2fms)", err, float64(time.Since(stepStart).Nanoseconds())/1000000)
		return nil, fmt.Errorf("customer not found: %v", err)
	}
	log.Printf("✅ Customer validated (%.2fms)", float64(time.Since(stepStart).Nanoseconds())/1000000)

	// Step 2: Generate payment code
	log.Printf("📝 Step 2: Generating payment code...")
	stepStart = time.Now()
	// Get prefix from settings (default RCV)
	settingsSvc := NewSettingsService(s.db)
	settings, _ := settingsSvc.GetSettings()
	prefix := "RCV"
	if settings != nil && settings.PaymentReceivablePrefix != "" {
		prefix = settings.PaymentReceivablePrefix
	}
	code := s.generatePaymentCode(prefix)
	log.Printf("✅ Payment code generated: %s (%.2fms)", code, float64(time.Since(stepStart).Nanoseconds())/1000000)

	// Step 3: Create payment record
	log.Printf("📝 Step 3: Creating payment record...")
	stepStart = time.Now()
	payment := &models.Payment{
		Code:                code,
		ContactID:           request.ContactID,
		UserID:              userID,
		Date:                request.Date,
		Amount:              request.Amount,
		NativeAmount:        math.Abs(cashBankMutation.NativeAmount),
		Currency:            cashBankMutation.Currency,
		ExchangeRate:        cashBankMutation.ExchangeRate,
		BookAmountIDR:       math.Abs(cashBankMutation.BookAmountIDR),
		ForexGainLossAmount: 0,
		Method:              request.Method,
		Reference:           request.Reference,
		Status:              models.PaymentStatusPending,
		Notes:               request.Notes,
	}
	if payment.Currency == "" {
		payment.Currency = "IDR"
	}
	if payment.ExchangeRate <= 0 {
		if request.ExchangeRate > 0 {
			payment.ExchangeRate = request.ExchangeRate
		} else {
			payment.ExchangeRate = 1
		}
	}

	if err := tx.Create(payment).Error; err != nil {
		log.Printf("❌ Failed to create payment: %v (%.2fms)", err, float64(time.Since(stepStart).Nanoseconds())/1000000)
		return nil, fmt.Errorf("failed to create payment: %v", err)
	}
	log.Printf("✅ Payment record created: ID=%d (%.2fms)", payment.ID, float64(time.Since(stepStart).Nanoseconds())/1000000)

	// Step 4: Process allocations
	log.Printf("📝 Step 4: Processing %d allocations...", len(request.Allocations))
	stepStart = time.Now()
	remainingAmount := request.Amount
	totalAllocatedAmount := 0.0 // Track actual allocated amount
	totalReceivableSettlementBookIDR := 0.0
	affectedSaleIDs := make(map[uint]struct{})

	for i, allocation := range request.Allocations {
		if remainingAmount <= 0 {
			log.Printf("⚠️ No remaining amount, skipping allocation %d", i+1)
			break
		}

		log.Printf("📝 Processing allocation %d: InvoiceID=%d, Amount=%.2f", i+1, allocation.InvoiceID, allocation.Amount)

		// Get sale
		sale, err := s.salesRepo.FindByID(allocation.InvoiceID)
		if err != nil {
			log.Printf("❌ Invoice %d not found: %v", allocation.InvoiceID, err)
			return nil, fmt.Errorf("invoice %d not found: %v", allocation.InvoiceID, err)
		}

		// Validate ownership
		if sale.CustomerID != request.ContactID {
			log.Printf("❌ Invoice ownership mismatch: Sale.CustomerID=%d, Request.ContactID=%d", sale.CustomerID, request.ContactID)
			return nil, fmt.Errorf("invoice does not belong to this customer")
		}

		// 🔒 CRITICAL: Validate invoice status (consistent with SalesJournalServiceV2.ShouldPostToJournal)
		if err := s.statusValidator.ValidatePaymentAllocation(sale.Status, allocation.InvoiceID); err != nil {
			log.Printf("❌ Payment allocation blocked: %v", err)
			return nil, err
		}
		log.Printf("✅ Invoice #%d status '%s' is valid for payment allocation", allocation.InvoiceID, sale.Status)

		// Calculate allocated amount
		allocatedAmount := allocation.Amount
		if allocatedAmount > remainingAmount {
			allocatedAmount = remainingAmount
			log.Printf("⚠️ Adjusting amount to remaining: %.2f -> %.2f", allocation.Amount, allocatedAmount)
		}
		if allocatedAmount > sale.OutstandingAmount {
			allocatedAmount = sale.OutstandingAmount
			log.Printf("⚠️ Adjusting amount to outstanding: %.2f -> %.2f", allocatedAmount, sale.OutstandingAmount)
		}

		// Create payment allocation
		paymentAllocation := &models.PaymentAllocation{
			PaymentID:       uint64(payment.ID),
			InvoiceID:       &allocation.InvoiceID,
			AllocatedAmount: allocatedAmount,
		}

		if err := tx.Create(paymentAllocation).Error; err != nil {
			log.Printf("❌ Failed to create payment allocation: %v", err)
			return nil, fmt.Errorf("failed to create payment allocation: %v", err)
		}
		log.Printf("✅ Payment allocation created: %.2f", allocatedAmount)

		// Update sale amounts
		log.Printf("📝 Updating sale amounts: PaidAmount %.2f -> %.2f, Outstanding %.2f -> %.2f",
			sale.PaidAmount, sale.PaidAmount+allocatedAmount,
			sale.OutstandingAmount, sale.OutstandingAmount-allocatedAmount)

		sale.PaidAmount += allocatedAmount
		sale.OutstandingAmount -= allocatedAmount

		// Update status
		if sale.OutstandingAmount <= 0 {
			sale.Status = models.SaleStatusPaid
			log.Printf("✅ Sale status updated to PAID")
		} else if sale.PaidAmount > 0 && sale.Status == models.SaleStatusInvoiced {
			sale.Status = models.SaleStatusInvoiced
			log.Printf("✅ Sale status remains INVOICED (partial payment)")
		}

		// Save sale changes
		if err := tx.Save(sale).Error; err != nil {
			log.Printf("❌ Failed to save sale: %v", err)
			return nil, fmt.Errorf("failed to update sale: %v", err)
		}
		log.Printf("✅ Sale updated successfully")

		affectedSaleIDs[sale.ID] = struct{}{}

		// Create SalePayment cross-reference
		nativeAmount := allocatedAmount
		bankBookAmountIDR := allocatedAmount
		exchangeRate := 1.0
		currency := "IDR"

		if request.CashBankID > 0 && selectedCashBank != nil {
			currency = cashBankMutation.Currency
			exchangeRate = cashBankMutation.ExchangeRate
			if request.Amount > 0 {
				bankBookAmountIDR = payment.BookAmountIDR * (allocatedAmount / request.Amount)
				nativeAmount = cashBankMutation.NativeAmount * (allocatedAmount / request.Amount)
			}
		}

		receivableSettlementBookIDR, _, err := calculateSaleReceivableSettlementBookAmount(sale, allocatedAmount, request.ExchangeRate)
		if err != nil {
			log.Printf("âŒ Failed to calculate receivable settlement book amount for sale %d: %v", sale.ID, err)
			return nil, fmt.Errorf("failed to calculate receivable settlement book amount for sale %d: %v", sale.ID, err)
		}
		totalReceivableSettlementBookIDR += receivableSettlementBookIDR

		forexDiff := bankBookAmountIDR - receivableSettlementBookIDR
		forexGainLossAmount := 0.0
		forexGainLossType := ""
		if forexDiff > 0.005 {
			forexGainLossAmount = forexDiff
			forexGainLossType = "GAIN"
		} else if forexDiff < -0.005 {
			forexGainLossAmount = -forexDiff
			forexGainLossType = "LOSS"
		}

		salePayment := &models.SalePayment{
			SaleID:              sale.ID,
			Amount:              allocatedAmount,
			NativeAmount:        nativeAmount,
			Currency:            currency,
			ExchangeRate:        exchangeRate,
			BookAmountIDR:       bankBookAmountIDR,
			ForexGainLossAmount: forexGainLossAmount,
			ForexGainLossType:   forexGainLossType,
			PaymentDate:         payment.Date,
			PaymentMethod:       payment.Method,
			Reference:           fmt.Sprintf("Payment ID: %d", payment.ID),
			Notes:               fmt.Sprintf("Created from Payment Management - %s", payment.Notes),
			CashBankID:          &request.CashBankID,
			UserID:              userID,
			Status:              models.SalePaymentStatusCompleted,
		}

		if err := tx.Create(salePayment).Error; err != nil {
			log.Printf("❌ CRITICAL: Failed to create sale payment cross-reference for payment %d, sale %d: %v", payment.ID, sale.ID, err)
			// This is critical - if this fails, return error for auto-rollback
			return nil, fmt.Errorf("failed to create sale payment record: %v", err)
		} else {
			log.Printf("✅ Sale payment cross-reference created: payment_id=%d, sale_id=%d, amount=%.2f", payment.ID, sale.ID, allocatedAmount)
		}

		remainingAmount -= allocatedAmount
		totalAllocatedAmount += allocatedAmount // Accumulate allocated amount
		log.Printf("✅ Allocation %d complete. Remaining: %.2f, Total Allocated: %.2f", i+1, remainingAmount, totalAllocatedAmount)
	}
	log.Printf("✅ All allocations processed. Total allocated: %.2f (%.2fms)", totalAllocatedAmount, float64(time.Since(stepStart).Nanoseconds())/1000000)

	for saleID := range affectedSaleIDs {
		var syncedSale models.Sale
		if err := tx.First(&syncedSale, saleID).Error; err != nil {
			log.Printf("warning: failed to reload sale %d for payment schedule sync: %v", saleID, err)
			return nil, fmt.Errorf("failed to reload sale %d for payment schedule sync: %v", saleID, err)
		}
		if err := syncSalePaymentScheduleProgress(tx, &syncedSale); err != nil {
			log.Printf("warning: failed to sync sale payment schedules for sale %d: %v", saleID, err)
			return nil, fmt.Errorf("failed to sync sale payment schedules for sale %d: %v", saleID, err)
		}
	}

	payment.ForexGainLossAmount = 0
	payment.ForexGainLossType = ""
	overallForexDiff := payment.BookAmountIDR - totalReceivableSettlementBookIDR
	if overallForexDiff > 0.005 {
		payment.ForexGainLossAmount = overallForexDiff
		payment.ForexGainLossType = "GAIN"
	} else if overallForexDiff < -0.005 {
		payment.ForexGainLossAmount = -overallForexDiff
		payment.ForexGainLossType = "LOSS"
	}
	if err := tx.Save(payment).Error; err != nil {
		log.Printf("âŒ Failed to update payment forex summary: %v", err)
		return nil, fmt.Errorf("failed to update payment forex summary: %v", err)
	}

	// Step 5: Update Cash/Bank balance and record transaction (IN flow)
	// 🔥 FIX: Use request.Amount (full payment amount) instead of totalAllocatedAmount
	// Payment record should always reflect the full amount received from customer
	// Allocation amount is for invoice tracking only, not for cash/bank balance
	if request.CashBankID > 0 && request.Amount > 0 {
		log.Printf("🏦 Updating cash/bank balance for receivable payment: amount=%.2f, cashBankID=%d", request.Amount, request.CashBankID)
		if err := s.updateCashBankBalance(tx, request.CashBankID, cashBankMutation.NativeAmount, "IN", payment.ID, userID, cashBankMutation); err != nil {
			log.Printf("❌ Cash/Bank balance update failed: %v", err)
			return nil, fmt.Errorf("cash/bank balance update failed: %v", err)
		}
		log.Printf("✅ Cash/Bank balance updated and transaction recorded")
	} else {
		if request.CashBankID == 0 {
			log.Printf("⚠️ CashBankID not provided; skipping Cash/Bank balance update")
		}
		if request.Amount <= 0 {
			log.Printf("⚠️ No payment amount to record in Cash/Bank; skipping")
		}
	}

	// Step 6: Create proper journals via SalesJournalServiceV2 (no ultra-fast posting)
	log.Printf("📝 Step 6: Creating journals via SalesJournalServiceV2 (no ultra-fast posting)...")
	stepStart = time.Now()
	coaSvc := NewCOAService(s.db)
	journalRepo := repositories.NewJournalEntryRepository(s.db)
	salesJournal := NewSalesJournalServiceV2(s.db, journalRepo, coaSvc)

	// Create payment journals for each allocation cross-reference we just created
	// We will query sale payments created within this transaction for this payment code reference
	var salePayments []models.SalePayment
	if err := tx.Where("reference = ?", fmt.Sprintf("Payment ID: %d", payment.ID)).Find(&salePayments).Error; err == nil {
		for _, sp := range salePayments {
			// Load sale for each sp.SaleID
			sale, err := s.salesRepo.FindByID(sp.SaleID)
			if err != nil {
				log.Printf("⚠️ Warning: Skipping journal for SalePayment %d (sale not found): %v", sp.ID, err)
				continue
			}
			if postErr := salesJournal.CreateSalesPaymentJournal(&sp, sale, tx); postErr != nil {
				log.Printf("⚠️ Warning: Failed to create sales payment journal for sp=%d: %v", sp.ID, postErr)
			}
		}
	} else {
		log.Printf("⚠️ Warning: Could not load sale payments for journaling: %v", err)
	}
	log.Printf("✅ Payment journals created (%.2fms)", float64(time.Since(stepStart).Nanoseconds())/1000000)

	// Step 6: Continue
	log.Printf("📝 Step 6: Finalizing...")
	stepStart = time.Now()
	log.Printf("✅ Proceeding to status update (%.2fms)", float64(time.Since(stepStart).Nanoseconds())/1000000)

	// Step 7: Update payment status
	log.Printf("📝 Step 7: Updating payment status to COMPLETED...")
	stepStart = time.Now()
	payment.Status = models.PaymentStatusCompleted
	if err := tx.Save(payment).Error; err != nil {
		log.Printf("❌ Failed to save payment status: %v (%.2fms)", err, float64(time.Since(stepStart).Nanoseconds())/1000000)
		return nil, fmt.Errorf("failed to update payment status: %v", err)
	}
	log.Printf("✅ Payment status updated (%.2fms)", float64(time.Since(stepStart).Nanoseconds())/1000000)

	// Step 8: Commit transaction
	log.Printf("📋 Step 8: Committing transaction...")
	stepStart = time.Now()
	commitErr := tx.Commit().Error
	if commitErr != nil {
		log.Printf("❌ CRITICAL: Failed to commit transaction: %v (%.2fms)", commitErr, float64(time.Since(stepStart).Nanoseconds())/1000000)
		return nil, fmt.Errorf("transaction commit failed: %v", commitErr)
	}
	committed = true // Mark as committed to prevent auto-rollback
	log.Printf("✅ Transaction committed successfully (%.2fms)", float64(time.Since(stepStart).Nanoseconds())/1000000)

	totalTime := time.Since(startTime)
	log.Printf("🎉 CreateReceivablePayment COMPLETED: ID=%d, Code=%s, ContactID=%d, Amount=%.2f, TotalTime=%.2fms",
		payment.ID, payment.Code, payment.ContactID, payment.Amount, float64(totalTime.Nanoseconds())/1000000)

	return payment, nil
}

// GetSaleByID gets sale by ID - CRITICAL FIX for payment controller
func (s *PaymentService) GetSaleByID(saleID uint) (*models.Sale, error) {
	if s.salesRepo == nil {
		return nil, fmt.Errorf("sales repository not initialized")
	}

	sale, err := s.salesRepo.FindByID(saleID)
	if err != nil {
		return nil, err
	}

	hydrateSalePaymentScheduleProgress(sale)
	return sale, nil
}

// CreatePayablePayment creates payment for purchases/payables
func (s *PaymentService) CreatePayablePayment(request PaymentCreateRequest, userID uint) (*models.Payment, error) {
	start := time.Now()
	log.Printf("Starting CreatePayablePayment: ContactID=%d, Amount=%.2f", request.ContactID, request.Amount)
	if request.Date.IsZero() {
		return nil, fmt.Errorf("payment date is required")
	}
	if err := CheckClosedPeriod(s.db, request.Date); err != nil {
		return nil, err
	}

	// Start transaction with timeout
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in CreatePayablePayment: %v", r)
			tx.Rollback()
		}
	}()

	// Validate vendor with optimized query
	var contact models.Contact
	if err := tx.Select("id, name, type").First(&contact, request.ContactID).Error; err != nil {
		tx.Rollback()
		log.Printf("Vendor validation failed: %v", err)
		return nil, errors.New("vendor not found")
	}
	log.Printf("Vendor validated: %s (ID: %d)", contact.Name, contact.ID)

	cashBankMutation := &cashBankBalanceMutation{
		NativeAmount:    -request.Amount,
		BookAmountIDR:   -request.Amount,
		Currency:        "IDR",
		ExchangeRate:    1,
		TransactionDate: request.Date,
		Notes:           "Vendor payment disbursement",
	}

	// Check cash/bank balance
	log.Printf("Checking balance for CashBankID: %d", request.CashBankID)
	balanceCheckStart := time.Now()
	if request.CashBankID > 0 {
		cashBank, err := s.cashBankRepo.FindByID(request.CashBankID)
		if err != nil {
			tx.Rollback()
			log.Printf("Cash/bank account not found: %v", err)
			return nil, errors.New("cash/bank account not found")
		}

		normalizedCurrency := normalizeCashBankCurrency(cashBank.Currency)
		cashBankMutation.Currency = normalizedCurrency

		if normalizedCurrency != "IDR" && normalizedCurrency != "USD" {
			tx.Rollback()
			return nil, fmt.Errorf("payable payment currently only supports IDR and USD cash/bank accounts")
		}

		requiredNativeAmount := request.Amount
		bookReductionIDR := request.Amount
		if normalizedCurrency == "USD" {
			if request.ExchangeRate <= 0 {
				tx.Rollback()
				return nil, fmt.Errorf("exchange rate is required when paying from USD cash/bank account")
			}

			requiredNativeAmount = request.Amount / request.ExchangeRate
			calculatedBookReductionIDR, _, err := calculatePaymentReductionBookAmount(cashBank, requiredNativeAmount)
			if err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to calculate USD carrying book amount: %v", err)
			}
			bookReductionIDR = calculatedBookReductionIDR
			cashBankMutation.NativeAmount = -requiredNativeAmount
			cashBankMutation.BookAmountIDR = -bookReductionIDR
			cashBankMutation.ExchangeRate = request.ExchangeRate
		}

		if cashBank.Balance < requiredNativeAmount {
			tx.Rollback()
			log.Printf("Insufficient balance: Available=%.6f, Required=%.6f (%s)", cashBank.Balance, requiredNativeAmount, normalizedCurrency)
			return nil, fmt.Errorf("insufficient balance. Available: %.6f, Required: %.6f", cashBank.Balance, requiredNativeAmount)
		}
		log.Printf("Balance check passed: %.6f available (%s) (%.2fms)", cashBank.Balance, normalizedCurrency, float64(time.Since(balanceCheckStart).Nanoseconds())/1000000)
	}

	// Generate payment code (prefix from settings)
	codeGenStart := time.Now()
	settingsSvc := NewSettingsService(s.db)
	settings, _ := settingsSvc.GetSettings()
	prefix := "PAY"
	if settings != nil && settings.PaymentPayablePrefix != "" {
		prefix = settings.PaymentPayablePrefix
	}
	code := s.generatePaymentCode(prefix)
	log.Printf("Payment code generated: %s (%.2fms)", code, float64(time.Since(codeGenStart).Nanoseconds())/1000000)

	// Create payment record
	payment := &models.Payment{
		Code:          code,
		ContactID:     request.ContactID,
		UserID:        userID,
		Date:          request.Date,
		Amount:        request.Amount,
		NativeAmount:  math.Abs(cashBankMutation.NativeAmount),
		Currency:      cashBankMutation.Currency,
		ExchangeRate:  cashBankMutation.ExchangeRate,
		BookAmountIDR: math.Abs(cashBankMutation.BookAmountIDR),
		Method:        request.Method,
		Reference:     request.Reference,
		Status:        models.PaymentStatusPending,
		Notes:         request.Notes,
	}
	if payment.Currency == "" {
		payment.Currency = "IDR"
	}
	if payment.ExchangeRate <= 0 {
		if request.ExchangeRate > 0 {
			payment.ExchangeRate = request.ExchangeRate
		} else {
			payment.ExchangeRate = 1
		}
	}
	if payment.BookAmountIDR == 0 {
		payment.BookAmountIDR = request.Amount
	}
	forexDiff := payment.BookAmountIDR - payment.Amount
	if forexDiff > 0.005 {
		payment.ForexGainLossAmount = forexDiff
		payment.ForexGainLossType = "LOSS"
	} else if forexDiff < -0.005 {
		payment.ForexGainLossAmount = -forexDiff
		payment.ForexGainLossType = "GAIN"
	}

	if err := tx.Create(payment).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Process allocations to bills
	remainingAmount := request.Amount
	for _, allocation := range request.BillAllocations {
		if remainingAmount <= 0 {
			break
		}

		var purchase models.Purchase
		if err := tx.First(&purchase, allocation.BillID).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("bill %d not found", allocation.BillID)
		}

		if purchase.VendorID != request.ContactID {
			tx.Rollback()
			return nil, errors.New("bill does not belong to this vendor")
		}

		allocatedAmount := allocation.Amount
		if allocatedAmount > remainingAmount {
			allocatedAmount = remainingAmount
		}

		// Calculate outstanding using OutstandingAmount field (proper tracking)
		if allocatedAmount > purchase.OutstandingAmount {
			allocatedAmount = purchase.OutstandingAmount
			log.Printf("⚠️ Adjusting allocated amount to outstanding: %.2f -> %.2f", allocation.Amount, allocatedAmount)
		}

		// Create payment allocation
		paymentAllocation := &models.PaymentAllocation{
			PaymentID:       uint64(payment.ID),
			BillID:          &allocation.BillID,
			AllocatedAmount: allocatedAmount,
		}

		if err := tx.Create(paymentAllocation).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		// 🔥 FIX: Update purchase paid amount and outstanding amount (same as receivable payment logic)
		log.Printf("📝 Updating purchase amounts: PaidAmount %.2f -> %.2f, Outstanding %.2f -> %.2f",
			purchase.PaidAmount, purchase.PaidAmount+allocatedAmount,
			purchase.OutstandingAmount, purchase.OutstandingAmount-allocatedAmount)

		purchase.PaidAmount += allocatedAmount
		purchase.OutstandingAmount -= allocatedAmount

		// Update status if fully paid
		if purchase.OutstandingAmount <= 0 {
			purchase.MatchingStatus = models.PurchaseMatchingMatched
			log.Printf("✅ Purchase fully paid, status updated to MATCHED")
		} else {
			purchase.MatchingStatus = models.PurchaseMatchingPartial
			log.Printf("✅ Purchase partially paid (Outstanding: %.2f)", purchase.OutstandingAmount)
		}

		// Save purchase changes
		if err := tx.Save(&purchase).Error; err != nil {
			log.Printf("❌ Failed to save purchase: %v", err)
			tx.Rollback()
			return nil, fmt.Errorf("failed to update purchase: %v", err)
		}
		log.Printf("✅ Purchase updated successfully")

		remainingAmount -= allocatedAmount
	}

	// Update cash/bank account balance and record transaction safely (OUT flow)
	if request.CashBankID > 0 {
		log.Printf("🏦 Updating cash/bank balance for payable payment: amount=%.2f, cashBankID=%d", request.Amount, request.CashBankID)
		// For payable (outgoing) payments, pass negative amount and direction OUT
		if err := s.updateCashBankBalance(tx, request.CashBankID, cashBankMutation.NativeAmount, "OUT", payment.ID, userID, cashBankMutation); err != nil {
			log.Printf("❌ Cash/Bank balance update failed: %v", err)
			tx.Rollback()
			return nil, fmt.Errorf("cash/bank balance update failed: %v", err)
		}
		log.Printf("✅ Cash/Bank balance updated and transaction recorded")
	}

	// 🔥 SSOT Integration: Create payment journal entry via PurchasePaymentJournalService
	// This ensures proper double-entry accounting: Debit Accounts Payable, Credit Cash/Bank
	log.Printf("📝 Creating SSOT payment journal entry via PurchasePaymentJournalService...")
	if s.purchasePaymentJournalService != nil {
		// Create journal entry using SSOT payment journal service with existing transaction
		if err := s.purchasePaymentJournalService.CreatePaymentJournalEntryWithTx(tx, payment, request.CashBankID, userID); err != nil {
			log.Printf("❌ SSOT payment journal creation failed: %v", err)
			tx.Rollback()
			return nil, fmt.Errorf("payment journal creation failed: %v", err)
		}
		log.Printf("✅ SSOT payment journal created successfully")
	} else {
		log.Printf("⚠️ PurchasePaymentJournalService not initialized - SSOT journal NOT created")
	}

	// 🔧 Optional Legacy COA Journal (deprecated)
	// If ENABLE_LEGACY_PAYMENT_JOURNALS=true, also create legacy COA journal (AP↓, Bank↓)
	if os.Getenv("ENABLE_LEGACY_PAYMENT_JOURNALS") == "true" {
		log.Printf("📝 Creating legacy payment journal as well (deprecated)...")
		if err := s.createLegacyPayableJournalInTx(tx, payment, request.CashBankID, userID); err != nil {
			log.Printf("⚠️ Legacy payment journal creation failed: %v", err)
		} else {
			log.Printf("📗 Legacy payment journal created (AP debit, Bank credit)")
		}
	}

	payment.Status = models.PaymentStatusCompleted
	if err := tx.Save(payment).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to save payment status: %v", err)
		return nil, err
	}

	log.Printf("Payment creation completed, committing transaction...")
	if err := tx.Commit().Error; err != nil {
		log.Printf("Failed to commit payment transaction: %v", err)
		return nil, err
	}

	totalTime := time.Since(start)
	log.Printf("✅ CreatePayablePayment completed successfully: ID=%d, Code=%s, Amount=%.2f, TotalTime=%.2fms",
		payment.ID, payment.Code, payment.Amount, float64(totalTime.Nanoseconds())/1000000)

	return payment, nil
}

// updateCashBankBalance updates cash/bank balance and creates transaction record
func (s *PaymentService) updateCashBankBalance(tx *gorm.DB, cashBankID uint, amount float64, direction string, referenceID uint, userID uint, mutation *cashBankBalanceMutation) error {
	guard := NewCashBankCurrencyGuard(s.db)
	var cashBank models.CashBank
	if err := tx.First(&cashBank, cashBankID).Error; err != nil {
		return fmt.Errorf("cash/bank account not found: %v", err)
	}

	appliedMutation := mutation
	if appliedMutation == nil {
		appliedMutation = &cashBankBalanceMutation{
			NativeAmount:    amount,
			BookAmountIDR:   amount,
			Currency:        normalizeCashBankCurrency(cashBank.Currency),
			ExchangeRate:    1,
			TransactionDate: time.Now(),
			Notes:           fmt.Sprintf("Payment %s", direction),
		}
	}
	if appliedMutation.TransactionDate.IsZero() {
		appliedMutation.TransactionDate = time.Now()
	}
	if err := CheckClosedPeriod(tx, appliedMutation.TransactionDate); err != nil {
		return err
	}
	if appliedMutation.Currency == "" {
		appliedMutation.Currency = normalizeCashBankCurrency(cashBank.Currency)
	}
	if appliedMutation.Notes == "" {
		appliedMutation.Notes = fmt.Sprintf("Payment %s", direction)
	}

	log.Printf("Updating Cash/Bank Balance: ID=%d, Name=%s, CurrentNative=%.6f, NativeDelta=%.6f, BookDeltaIDR=%.2f, Direction=%s, Currency=%s",
		cashBankID, cashBank.Name, cashBank.Balance, amount, appliedMutation.BookAmountIDR, direction, appliedMutation.Currency)

	// For receivable payments (IN), amount should be positive, balance increases
	// For payable payments (OUT), amount should be negative, balance decreases

	// For outgoing payments, validate sufficient balance BEFORE updating
	if direction == "OUT" && amount < 0 {
		requiredAmount := -amount // Convert negative to positive
		if cashBank.Balance < requiredAmount {
			return fmt.Errorf("insufficient balance. Available: %.2f, Required: %.2f", cashBank.Balance, requiredAmount)
		}
	}

	newBalance := cashBank.Balance + amount
	if newBalance < 0 {
		return fmt.Errorf("transaction would result in negative balance. Current: %.6f, Change: %.6f, Result: %.6f",
			cashBank.Balance, amount, newBalance)
	}

	newBookBalanceIDR := cashBank.BookBalanceIDR + appliedMutation.BookAmountIDR
	if !cashBank.IsForeignCurrency() {
		newBookBalanceIDR = newBalance
	}
	if newBookBalanceIDR < 0 {
		return fmt.Errorf("transaction would result in negative book balance. Current: %.2f, Change: %.2f, Result: %.2f",
			cashBank.BookBalanceIDR, appliedMutation.BookAmountIDR, newBookBalanceIDR)
	}

	cashBank.Balance = newBalance
	if cashBank.IsForeignCurrency() {
		cashBank.BookBalanceIDR = newBookBalanceIDR
		if appliedMutation.ExchangeRate > 0 {
			cashBank.LastExchangeRate = appliedMutation.ExchangeRate
		}
	}
	log.Printf("Balance updated successfully: native=%.6f, book_idr=%.2f", cashBank.Balance, cashBank.EffectiveBookBalanceIDR())

	if err := tx.Save(&cashBank).Error; err != nil {
		return err
	}

	// ✅ CRITICAL: Sync linked COA account balance to maintain COA <> CashBank consistency
	if cashBank.AccountID > 0 {
		if err := tx.Exec("UPDATE accounts SET balance = ?, updated_at = NOW() WHERE id = ? AND deleted_at IS NULL",
			cashBank.EffectiveBookBalanceIDR(), cashBank.AccountID).Error; err != nil {
			log.Printf("⚠️ Warning: Failed to sync COA balance for GL account %d: %v", cashBank.AccountID, err)
		} else {
			log.Printf("🔄 Synced COA balance for GL account %d to %.2f", cashBank.AccountID, newBalance)
		}
	}

	// Create transaction record
	transaction := &models.CashBankTransaction{
		CashBankID:      cashBankID,
		ReferenceType:   "PAYMENT",
		ReferenceID:     referenceID,
		Amount:          appliedMutation.NativeAmount,
		Currency:        appliedMutation.Currency,
		ExchangeRate:    appliedMutation.ExchangeRate,
		BookAmountIDR:   appliedMutation.BookAmountIDR,
		BalanceAfter:    cashBank.Balance,
		TransactionDate: appliedMutation.TransactionDate,
		Notes:           appliedMutation.Notes,
	}

	if err := tx.Create(transaction).Error; err != nil {
		return err
	}

	return guard.EnforceAfterTransaction(tx, cashBankID)
}

// createLegacyPayableJournalInTx creates legacy COA journal for vendor payable payment (AP↓, Bank↓)
func (s *PaymentService) createLegacyPayableJournalInTx(tx *gorm.DB, payment *models.Payment, cashBankID uint, userID uint) error {
	// Get Accounts Payable account (2101)
	var apAccount models.Account
	if err := tx.Where("code = ?", "2101").First(&apAccount).Error; err != nil {
		// Fallback by name contains 'hutang usaha'
		if err := tx.Where("LOWER(name) LIKE ?", "%hutang%usaha%").First(&apAccount).Error; err != nil {
			return fmt.Errorf("accounts payable (2101) not found: %v", err)
		}
	}
	// Resolve Cash/Bank GL account
	var cashBank models.CashBank
	if err := tx.Preload("Account").First(&cashBank, cashBankID).Error; err != nil {
		return fmt.Errorf("cash/bank account not found: %v", err)
	}
	if cashBank.AccountID == 0 {
		return fmt.Errorf("cash/bank %d has no linked GL account", cashBankID)
	}
	// Create legacy journal entry (posted)
	entry := &models.JournalEntry{
		EntryDate:       payment.Date,
		Description:     fmt.Sprintf("Vendor Payment %s", payment.Code),
		ReferenceType:   models.JournalRefPayment,
		ReferenceID:     &payment.ID,
		Reference:       payment.Code,
		UserID:          userID,
		Status:          models.JournalStatusPosted,
		TotalDebit:      payment.Amount,
		TotalCredit:     payment.Amount,
		IsAutoGenerated: true,
	}
	lines := []models.JournalLine{
		{AccountID: apAccount.ID, Description: fmt.Sprintf("AP reduction - %s", payment.Code), DebitAmount: payment.Amount, CreditAmount: 0, LineNumber: 1},
		{AccountID: cashBank.AccountID, Description: fmt.Sprintf("Bank payment - %s", payment.Code), DebitAmount: 0, CreditAmount: payment.Amount, LineNumber: 2},
	}
	entry.JournalLines = lines
	if err := tx.Create(entry).Error; err != nil {
		return fmt.Errorf("failed to create legacy payment journal: %v", err)
	}
	// NOTE: Do NOT update accounts.balance here!
	// The SSOT system (AutoPost=true) handles balance updates.
	// Manual updates here would cause double-posting.
	log.Printf("✅ Legacy payment journal created (balance updates handled by SSOT AutoPost)")
	return nil
}

// createReceivablePaymentJournal creates journal entries for receivable payment
func (s *PaymentService) createReceivablePaymentJournal(tx *gorm.DB, payment *models.Payment, cashBankID uint, userID uint) error {
	// Get accounts
	var cashBankAccountID uint
	if cashBankID > 0 {
		var cashBank models.CashBank
		if err := tx.First(&cashBank, cashBankID).Error; err != nil {
			return err
		}
		cashBankAccountID = cashBank.AccountID
	} else {
		// If no specific bank account, use default Kas account (1101)
		var kasAccount models.Account
		if err := tx.Where("code = ?", "1101").First(&kasAccount).Error; err != nil {
			return fmt.Errorf("default cash account (1101) not found: %v", err)
		}
		cashBankAccountID = kasAccount.ID
	}

	// Get Accounts Receivable account (Piutang Usaha - Try 1102-001 first, then 1102, then 1201)
	var arAccount models.Account
	// Try primary active AR account first
	if err := tx.Where("code = ?", "1102-001").First(&arAccount).Error; err != nil {
		// Try general AR account second
		if err := tx.Where("code = ?", "1102").First(&arAccount).Error; err != nil {
			// Try legacy AR account last
			if err := tx.Where("code = ?", "1201").First(&arAccount).Error; err != nil {
				log.Printf("Warning: Piutang Usaha exact accounts not found, using name fallback")
				// Fallback: try to find by name pattern
				if err := tx.Where("LOWER(name) LIKE ?", "%piutang%usaha%").First(&arAccount).Error; err != nil {
					return fmt.Errorf("accounts receivable account not found: %v", err)
				}
			}
		}
	}
	arAccountID := arAccount.ID

	log.Printf("Journal Entry Mapping: CashBank AccountID=%d, AR AccountID=%d (Code=%s)", cashBankAccountID, arAccountID, arAccount.Code)

	// Create journal entry
	journalEntry := &models.JournalEntry{
		// Code will be auto-generated by BeforeCreate hook
		EntryDate:       payment.Date,
		Description:     fmt.Sprintf("Customer Payment %s", payment.Code),
		ReferenceType:   models.JournalRefPayment,
		ReferenceID:     &payment.ID,
		Reference:       payment.Code,
		UserID:          userID,
		Status:          models.JournalStatusPosted,
		TotalDebit:      payment.Amount,
		TotalCredit:     payment.Amount,
		IsAutoGenerated: true,
	}

	// Journal lines
	journalLines := []models.JournalLine{
		// Debit: Cash/Bank
		{
			AccountID:    cashBankAccountID,
			Description:  fmt.Sprintf("Payment received - %s", payment.Code),
			DebitAmount:  payment.Amount,
			CreditAmount: 0,
		},
		// Credit: Accounts Receivable
		{
			AccountID:    arAccountID,
			Description:  fmt.Sprintf("AR reduction - %s", payment.Code),
			DebitAmount:  0,
			CreditAmount: payment.Amount,
		},
	}

	journalEntry.JournalLines = journalLines

	// Create journal entry
	if err := tx.Create(journalEntry).Error; err != nil {
		return err
	}

	// NOTE: Account balance updates are now handled automatically by the SSOT Journal system
	// Manual balance updates are removed to prevent double posting
	log.Printf("✅ Journal entry created, balance updates handled by SSOT system")

	return nil
}

// createPayablePaymentJournal creates journal entries for payable payment
func (s *PaymentService) createPayablePaymentJournal(tx *gorm.DB, payment *models.Payment, cashBankID uint, userID uint) error {
	// Get accounts with optimized queries
	var cashBankAccountID uint
	if cashBankID > 0 {
		var cashBank models.CashBank
		if err := tx.Select("account_id").First(&cashBank, cashBankID).Error; err != nil {
			return fmt.Errorf("cash/bank account not found: %v", err)
		}
		cashBankAccountID = cashBank.AccountID
	} else {
		// Get default cash account (Kas - 1101)
		var kasAccount models.Account
		if err := tx.Select("id").Where("code = ?", "1101").First(&kasAccount).Error; err != nil {
			return fmt.Errorf("default cash account (1101) not found: %v", err)
		}
		cashBankAccountID = kasAccount.ID
	}

	// Get Accounts Payable account (Hutang Usaha - 2101) with optimized query
	var apAccount models.Account
	if err := tx.Select("id").Where("code = ?", "2101").First(&apAccount).Error; err != nil {
		log.Printf("Warning: Hutang Usaha account (2101) not found, trying fallback")
		// Fallback: try to find by name pattern
		if err := tx.Select("id").Where("LOWER(name) LIKE ?", "%hutang%usaha%").First(&apAccount).Error; err != nil {
			return fmt.Errorf("accounts payable account not found: %v", err)
		}
	}
	apAccountID := apAccount.ID

	// Create journal entry
	journalEntry := &models.JournalEntry{
		// Code will be auto-generated by BeforeCreate hook
		EntryDate:       payment.Date,
		Description:     fmt.Sprintf("Vendor Payment %s", payment.Code),
		ReferenceType:   models.JournalRefPayment,
		ReferenceID:     &payment.ID,
		Reference:       payment.Code,
		UserID:          userID,
		Status:          models.JournalStatusPosted,
		TotalDebit:      payment.Amount,
		TotalCredit:     payment.Amount,
		IsAutoGenerated: true,
	}

	// Journal lines
	journalLines := []models.JournalLine{
		// Debit: Accounts Payable
		{
			AccountID:    apAccountID,
			Description:  fmt.Sprintf("AP reduction - %s", payment.Code),
			DebitAmount:  payment.Amount,
			CreditAmount: 0,
		},
		// Credit: Cash/Bank
		{
			AccountID:    cashBankAccountID,
			Description:  fmt.Sprintf("Payment made - %s", payment.Code),
			DebitAmount:  0,
			CreditAmount: payment.Amount,
		},
	}

	journalEntry.JournalLines = journalLines

	// Create journal entry
	if err := tx.Create(journalEntry).Error; err != nil {
		return err
	}

	// NOTE: Account balance updates are now handled automatically by the SSOT Journal system
	// Manual balance updates are removed to prevent double posting
	log.Printf("✅ Journal entry created, balance updates handled by SSOT system")

	return nil
}

// GetPayments retrieves payments with filters
func (s *PaymentService) GetPayments(filter repositories.PaymentFilter) (*repositories.PaymentResult, error) {
	return s.paymentRepo.FindWithFilter(filter)
}

// GetPaymentByID retrieves payment by ID
func (s *PaymentService) GetPaymentByID(id uint) (*models.Payment, error) {
	return s.paymentRepo.FindByID(id)
}

// DeletePayment deletes a payment (admin only)
func (s *PaymentService) DeletePayment(id uint, reason string, userID uint) error {
	// Start transaction
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get payment to verify it exists
	var payment models.Payment
	if err := tx.First(&payment, id).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("payment not found: %v", err)
	}

	// Guard: check if payment date is in a closed accounting period
	if err := CheckClosedPeriod(s.db, payment.Date); err != nil {
		tx.Rollback()
		return err
	}

	// Check if payment is already failed/cancelled
	if payment.Status == models.PaymentStatusFailed {
		// If already failed, just soft delete
		if err := tx.Delete(&payment).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to delete payment: %v", err)
		}
		log.Printf("Deleted failed payment %d (no reversal needed)", id)
	} else {
		// If payment is completed, we need to reverse it first
		log.Printf("Canceling payment %d before deletion", id)
		if err := s.cancelPaymentTransaction(tx, &payment, fmt.Sprintf("Deleted by admin: %s", reason), userID); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to cancel payment before deletion: %v", err)
		}

		// Now soft delete the payment
		if err := tx.Delete(&payment).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to delete payment after cancellation: %v", err)
		}
		log.Printf("Payment %d canceled and deleted successfully", id)
	}

	return tx.Commit().Error
}

// CancelPayment cancels a payment and reverses entries
func (s *PaymentService) CancelPayment(id uint, reason string, userID uint) error {
	tx := s.db.Begin()

	var payment models.Payment
	if err := tx.First(&payment, id).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Guard: check if payment date is in a closed accounting period
	if err := CheckClosedPeriod(s.db, payment.Date); err != nil {
		tx.Rollback()
		return err
	}

	if payment.Status == models.PaymentStatusFailed {
		tx.Rollback()
		return errors.New("payment is already cancelled")
	}

	// Use helper method to cancel payment
	if err := s.cancelPaymentTransaction(tx, &payment, reason, userID); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// cancelPaymentTransaction handles the cancellation logic (reusable helper)
func (s *PaymentService) cancelPaymentTransaction(tx *gorm.DB, payment *models.Payment, reason string, userID uint) error {
	// Reverse allocations
	var allocations []models.PaymentAllocation
	tx.Where("payment_id = ?", payment.ID).Find(&allocations)

	for _, allocation := range allocations {
		if allocation.InvoiceID != nil && *allocation.InvoiceID > 0 {
			// Reverse invoice payment
			var sale models.Sale
			if err := tx.First(&sale, allocation.InvoiceID).Error; err == nil {
				sale.PaidAmount -= allocation.AllocatedAmount
				sale.OutstandingAmount += allocation.AllocatedAmount

				if sale.Status == models.SaleStatusPaid {
					sale.Status = models.SaleStatusInvoiced
				}

				if err := tx.Save(&sale).Error; err != nil {
					return fmt.Errorf("failed to reverse sale payment allocation: %v", err)
				}
				if err := syncSalePaymentScheduleProgress(tx, &sale); err != nil {
					return fmt.Errorf("failed to sync sale payment schedules during payment cancellation: %v", err)
				}
			}
		}

		if allocation.BillID != nil && *allocation.BillID > 0 {
			// Reverse bill payment - would need proper tracking
			// This is simplified
		}
	}

	// Reverse cash/bank transaction
	var cashBankTx models.CashBankTransaction
	if err := tx.Where("reference_type = ? AND reference_id = ?", "PAYMENT", payment.ID).First(&cashBankTx).Error; err == nil {
		var cashBank models.CashBank
		if err := tx.First(&cashBank, cashBankTx.CashBankID).Error; err == nil {
			// Reverse the balance change
			cashBank.Balance -= cashBankTx.Amount
			tx.Save(&cashBank)
		}
	}

	// Create reversal journal entries
	if err := s.createReversalJournal(tx, payment, reason, userID); err != nil {
		return err
	}

	// CRITICAL FIX: Reverse COA balances BEFORE soft-deleting SSOT journal entries
	// This ensures accounts.balance (COA Tree View) is consistent after cancellation
	var ssotEntries []models.SSOTJournalEntry
	if err := tx.Where("source_type IN (?, ?) AND source_id = ? AND deleted_at IS NULL",
		"PAYMENT", "PURCHASE_PAYMENT", payment.ID).Find(&ssotEntries).Error; err == nil {
		for _, entry := range ssotEntries {
			var journalLines []models.SSOTJournalLine
			if err := tx.Where("journal_id = ? AND deleted_at IS NULL", entry.ID).Find(&journalLines).Error; err == nil {
				for _, line := range journalLines {
					// Reverse the COA balance: negate the original effect
					var account models.Account
					if err := tx.First(&account, line.AccountID).Error; err != nil {
						log.Printf("⚠️ Warning: Account %d not found for COA reversal: %v", line.AccountID, err)
						continue
					}

					debit := line.DebitAmount.InexactFloat64()
					credit := line.CreditAmount.InexactFloat64()
					var reverseChange float64

					switch account.Type {
					case "ASSET", "EXPENSE":
						reverseChange = -(debit - credit) // Negate the original effect
					case "LIABILITY", "EQUITY", "REVENUE":
						reverseChange = -(credit - debit) // Negate the original effect
					}

					if err := tx.Model(&models.Account{}).Where("id = ?", line.AccountID).
						UpdateColumn("balance", gorm.Expr("balance + ?", reverseChange)).Error; err != nil {
						log.Printf("⚠️ Warning: Failed to reverse COA balance for account %d: %v", line.AccountID, err)
					}
				}
				log.Printf("✅ Reversed COA balances for %d journal lines of payment %d (entry #%d)", len(journalLines), payment.ID, entry.ID)
			}
		}
	}

	// Soft-delete SSOT unified_journal_ledger entries linked to this payment
	// This prevents the Balance Sheet and Neraca Saldo from counting cancelled payments
	if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).
		Model(&models.SSOTJournalEntry{}).
		Where("source_type IN (?, ?) AND source_id = ?", "PAYMENT", "PURCHASE_PAYMENT", payment.ID).
		Update("deleted_at", time.Now()).Error; err != nil {
		log.Printf("Warning: failed to soft-delete SSOT journal entries for payment %d: %v", payment.ID, err)
	}

	// Also soft-delete any unified_journal_lines
	if err := tx.Exec(`
		UPDATE unified_journal_lines 
		SET deleted_at = CURRENT_TIMESTAMP
		WHERE deleted_at IS NULL AND journal_id IN (
			SELECT id FROM unified_journal_ledger 
			WHERE source_type IN ('PAYMENT', 'PURCHASE_PAYMENT') AND source_id = ?
		)
	`, payment.ID).Error; err != nil {
		log.Printf("Warning: failed to soft-delete SSOT journal lines for payment %d: %v", payment.ID, err)
	}

	// Update payment status
	payment.Status = models.PaymentStatusFailed
	payment.Notes += fmt.Sprintf("\nCancelled on %s. Reason: %s", time.Now().Format("2006-01-02"), reason)

	return tx.Save(payment).Error
}

// createReversalJournal creates reversal journal entries
func (s *PaymentService) createReversalJournal(tx *gorm.DB, payment *models.Payment, reason string, userID uint) error {
	// Find original journal entry
	var originalJournalEntry models.JournalEntry
	if err := tx.Where("reference_type = ? AND reference_id = ?", models.JournalRefPayment, payment.ID).First(&originalJournalEntry).Error; err != nil {
		return err
	}

	// Get original journal lines
	var originalLines []models.JournalLine
	if err := tx.Where("journal_entry_id = ?", originalJournalEntry.ID).Find(&originalLines).Error; err != nil {
		return err
	}

	// Create reversal journal entry
	reversalEntry := &models.JournalEntry{
		// Code will be auto-generated by BeforeCreate hook
		EntryDate:       time.Now(),
		Description:     fmt.Sprintf("Reversal of %s - %s", payment.Code, reason),
		ReferenceType:   models.JournalRefPayment,
		ReferenceID:     &payment.ID,
		Reference:       fmt.Sprintf("REV-%s", payment.Code),
		UserID:          userID,
		Status:          models.JournalStatusPosted,
		TotalDebit:      originalJournalEntry.TotalCredit, // Swap totals
		TotalCredit:     originalJournalEntry.TotalDebit,
		ReversedID:      &originalJournalEntry.ID,
		IsAutoGenerated: true,
	}

	// Create the journal entry first
	if err := tx.Create(reversalEntry).Error; err != nil {
		return err
	}

	// Create reversed journal lines
	for i, original := range originalLines {
		reversalLine := models.JournalLine{
			JournalEntryID: reversalEntry.ID,
			AccountID:      original.AccountID,
			Description:    fmt.Sprintf("Reversal - %s", original.Description),
			DebitAmount:    original.CreditAmount, // Swap debit and credit
			CreditAmount:   original.DebitAmount,
			LineNumber:     i + 1,
		}
		if err := tx.Create(&reversalLine).Error; err != nil {
			return err
		}

		// NOTE: Account balance updates for reversal are handled automatically by the SSOT Journal system
		// Manual balance updates are removed to prevent double posting
		log.Printf("✅ Reversal journal line created, balance updates handled by SSOT system")
	}

	// Update original entry to mark as reversed
	originalJournalEntry.ReversalID = &reversalEntry.ID
	originalJournalEntry.Status = models.JournalStatusReversed
	if err := tx.Save(&originalJournalEntry).Error; err != nil {
		return err
	}

	return nil
}

// Helper functions
func (s *PaymentService) generatePaymentCode(prefix string) string {
	year := time.Now().Year()
	month := time.Now().Month()
	return s.generatePaymentCodeAtomic(prefix, year, int(month))
}

// generatePaymentCodeAtomic generates payment code using atomic database operations
func (s *PaymentService) generatePaymentCodeAtomic(prefix string, year, month int) string {
	// Use atomic UPSERT operation to get next sequence number
	sequenceNum, err := s.getNextSequenceNumber(prefix, year, month)
	if err != nil {
		// Try to fix the issue by ensuring table exists and retry once
		log.Printf("First attempt failed, trying to ensure sequence table exists: %v", err)
		if tableErr := s.ensureSequenceTableExists(); tableErr != nil {
			log.Printf("Failed to ensure sequence table exists: %v", tableErr)
		} else {
			// Retry once after ensuring table exists
			sequenceNum, err = s.getNextSequenceNumber(prefix, year, month)
			if err == nil {
				return fmt.Sprintf("%s/%04d/%02d/%04d", prefix, year, month, sequenceNum)
			}
		}

		// Last resort: Use a simple counter-based approach
		log.Printf("All sequence generation attempts failed, using simple counter fallback: %v", err)
		fallbackNum := s.getSimpleFallbackNumber(prefix, year, month)
		return fmt.Sprintf("%s/%04d/%02d/%04d", prefix, year, month, fallbackNum)
	}

	return fmt.Sprintf("%s/%04d/%02d/%04d", prefix, year, month, sequenceNum)
}

// getNextSequenceNumber atomically gets the next sequence number for payment codes
func (s *PaymentService) getNextSequenceNumber(prefix string, year, month int) (int, error) {
	// Ensure payment_code_sequences table exists
	if err := s.ensureSequenceTableExists(); err != nil {
		log.Printf("Warning: Failed to ensure sequence table exists: %v", err)
		// Don't return error, try to continue with existing logic
	}

	// Try using database sequence table with atomic operations
	var sequenceRecord models.PaymentCodeSequence

	// Use a transaction with retry logic
	for attempt := 0; attempt < 3; attempt++ {
		tx := s.db.Begin()
		if tx.Error != nil {
			log.Printf("Failed to begin transaction (attempt %d): %v", attempt+1, tx.Error)
			continue
		}

		// Try to find existing record with row lock
		// Suppress "record not found" log by using silent logger session
		err := tx.Session(&gorm.Session{Logger: tx.Logger.LogMode(logger.Silent)}).
			Omit("created_at", "updated_at").
			Set("gorm:query_option", "FOR UPDATE").
			Where("prefix = ? AND year = ? AND month = ?", prefix, year, month).
			First(&sequenceRecord).Error

		if err != nil {
			if err == gorm.ErrRecordNotFound {
				// Create new sequence record (this is normal for first payment of the month)
				log.Printf("ℹ️  Creating new payment sequence for %s/%d/%02d", prefix, year, month)
				sequenceRecord = models.PaymentCodeSequence{
					Prefix:         prefix,
					Year:           year,
					Month:          month,
					SequenceNumber: 1,
				}

				if err := tx.Create(&sequenceRecord).Error; err != nil {
					tx.Rollback()
					log.Printf("Failed to create sequence record (attempt %d): %v", attempt+1, err)
					continue
				}

				if err := tx.Commit().Error; err != nil {
					log.Printf("Failed to commit create sequence (attempt %d): %v", attempt+1, err)
					continue
				}

				return 1, nil
			} else {
				tx.Rollback()
				log.Printf("Failed to query sequence record (attempt %d): %v", attempt+1, err)
				continue
			}
		}

		// Increment sequence number
		nextNum := sequenceRecord.SequenceNumber + 1
		sequenceRecord.SequenceNumber = nextNum

		if err := tx.Save(&sequenceRecord).Error; err != nil {
			tx.Rollback()
			log.Printf("Failed to save sequence record (attempt %d): %v", attempt+1, err)
			continue
		}

		if err := tx.Commit().Error; err != nil {
			log.Printf("Failed to commit sequence update (attempt %d): %v", attempt+1, err)
			continue
		}

		return nextNum, nil
	}

	// If all attempts failed, return error instead of falling back to timestamp
	return 0, fmt.Errorf("failed to generate sequence number after 3 attempts")
}

// ensureSequenceTableExists ensures the payment_code_sequences table exists
func (s *PaymentService) ensureSequenceTableExists() error {
	// Try to create the table structure if it doesn't exist
	if err := s.db.AutoMigrate(&models.PaymentCodeSequence{}); err != nil {
		return fmt.Errorf("failed to migrate PaymentCodeSequence table: %v", err)
	}

	// Ensure unique index exists
	if err := s.db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_sequence_unique 
		ON payment_code_sequences (prefix, year, month)
	`).Error; err != nil {
		// Log warning but don't fail - index might already exist
		log.Printf("Warning: Could not create unique index: %v", err)
	}

	return nil
}

// getSimpleFallbackNumber generates a fallback sequence number using simple counting
func (s *PaymentService) getSimpleFallbackNumber(prefix string, year, month int) int {
	// Count existing payments with this prefix/year/month pattern
	pattern := fmt.Sprintf("%s/%04d/%02d/%%", prefix, year, month)
	var count int64

	err := s.db.Model(&models.Payment{}).
		Where("code LIKE ? AND deleted_at IS NULL", pattern).
		Count(&count).Error

	if err != nil {
		log.Printf("Warning: Failed to count existing payments for fallback: %v", err)
		// Return current timestamp-based number as last resort
		return int(time.Now().UnixNano() % 9999)
	}

	// Return count + 1 as the next number
	return int(count) + 1
}

// checkPaymentCodeExists checks if a payment code already exists
func (s *PaymentService) checkPaymentCodeExists(code string) bool {
	var count int64
	s.db.Model(&models.Payment{}).Where("code = ?", code).Count(&count)
	return count > 0
}

// generateJournalCode is no longer used - journal codes are auto-generated by the JournalEntry BeforeCreate hook

// DTOs
type PaymentCreateRequest struct {
	ContactID       uint                `json:"contact_id" binding:"required"`
	CashBankID      uint                `json:"cash_bank_id"`
	Date            time.Time           `json:"date" binding:"required"`
	Amount          float64             `json:"amount" binding:"required,min=0"`
	ExchangeRate    float64             `json:"exchange_rate"`
	Method          string              `json:"method" binding:"required"`
	Reference       string              `json:"reference"`
	Notes           string              `json:"notes"`
	Allocations     []InvoiceAllocation `json:"allocations"`
	BillAllocations []BillAllocation    `json:"bill_allocations"`
}

type cashBankBalanceMutation struct {
	NativeAmount    float64
	BookAmountIDR   float64
	Currency        string
	ExchangeRate    float64
	TransactionDate time.Time
	Notes           string
}

func calculatePaymentReductionBookAmount(account *models.CashBank, nativeAmount float64) (float64, float64, error) {
	if account == nil {
		return 0, 0, fmt.Errorf("cash/bank account is required")
	}
	if !account.IsForeignCurrency() {
		return nativeAmount, 1, nil
	}
	if nativeAmount <= 0 {
		return 0, 0, fmt.Errorf("native amount must be greater than zero")
	}
	if account.Balance <= 0 {
		return 0, 0, fmt.Errorf("foreign currency balance is not available")
	}
	if nativeAmount-account.Balance > 0.000001 {
		return 0, 0, fmt.Errorf("native amount %.6f exceeds available balance %.6f", nativeAmount, account.Balance)
	}

	bookBalance := account.EffectiveBookBalanceIDR()
	carryRate := 0.0
	if bookBalance > 0 {
		carryRate = bookBalance / account.Balance
	}
	bookAmountIDR := bookBalance * (nativeAmount / account.Balance)
	if bookAmountIDR > bookBalance {
		bookAmountIDR = bookBalance
	}
	return bookAmountIDR, carryRate, nil
}

type InvoiceAllocation struct {
	InvoiceID uint    `json:"invoice_id"`
	Amount    float64 `json:"amount"`
}

type BillAllocation struct {
	BillID uint    `json:"bill_id"`
	Amount float64 `json:"amount"`
}

// PaymentAllocation is defined in repositories package

// GetUnpaidInvoices gets outstanding invoices for a customer
func (s *PaymentService) GetUnpaidInvoices(customerID uint) ([]OutstandingInvoice, error) {
	// Get sales from sales repository where customer_id = customerID and outstanding_amount > 0
	var sales []models.Sale
	err := s.db.Where("customer_id = ? AND outstanding_amount > ?", customerID, 0).Find(&sales).Error
	if err != nil {
		return nil, err
	}

	// Convert to OutstandingInvoice format
	var invoices []OutstandingInvoice
	for _, sale := range sales {
		invoice := OutstandingInvoice{
			ID:                sale.ID,
			Code:              sale.Code,
			Date:              sale.Date.Format("2006-01-02"),
			TotalAmount:       sale.TotalAmount,
			OutstandingAmount: sale.OutstandingAmount,
			Currency:          sale.Currency,
			ExchangeRate:      sale.ExchangeRate,
		}

		// Add due date if available (sales usually don't have due date, but we can calculate it)
		// For now, we'll use a 30-day payment term from invoice date
		dueDate := sale.Date.AddDate(0, 0, 30).Format("2006-01-02")
		invoice.DueDate = &dueDate

		invoices = append(invoices, invoice)
	}

	return invoices, nil
}

// GetUnpaidBills gets outstanding bills for a vendor
func (s *PaymentService) GetUnpaidBills(vendorID uint) ([]OutstandingBill, error) {
	// Get purchases from purchase repository where vendor_id = vendorID and outstanding_amount > 0
	// Filter by APPROVED status and outstanding_amount > 0 to only show bills that need payment
	var purchases []models.Purchase
	err := s.db.Where("vendor_id = ? AND status = ? AND outstanding_amount > ?", vendorID, "APPROVED", 0).Find(&purchases).Error
	if err != nil {
		return nil, err
	}

	log.Printf("GetUnpaidBills - Found %d purchases for vendor %d", len(purchases), vendorID)

	// Convert to OutstandingBill format
	var bills []OutstandingBill
	for _, purchase := range purchases {
		// Use the actual outstanding_amount from the purchase record
		// This reflects total_amount - paid_amount (managed by payment allocations)
		bill := OutstandingBill{
			ID:                purchase.ID,
			Code:              purchase.Code,
			Date:              purchase.Date.Format("2006-01-02"),
			TotalAmount:       purchase.TotalAmount,
			OutstandingAmount: purchase.OutstandingAmount, // Use actual outstanding amount from DB
		}

		log.Printf("  - Purchase %s: Total=%.2f, Outstanding=%.2f, Paid=%.2f",
			purchase.Code, purchase.TotalAmount, purchase.OutstandingAmount, purchase.PaidAmount)

		// Add due date if available
		// For purchases, we can use the DueDate field
		dueDate := purchase.DueDate.Format("2006-01-02")
		bill.DueDate = &dueDate

		bills = append(bills, bill)
	}

	return bills, nil
}

// Outstanding item types
type OutstandingInvoice struct {
	ID                uint    `json:"id"`
	Code              string  `json:"code"`
	Date              string  `json:"date"`
	TotalAmount       float64 `json:"total_amount"`
	OutstandingAmount float64 `json:"outstanding_amount"`
	Currency          string  `json:"currency,omitempty"`
	ExchangeRate      float64 `json:"exchange_rate,omitempty"`
	DueDate           *string `json:"due_date,omitempty"`
}

type OutstandingBill struct {
	ID                uint    `json:"id"`
	Code              string  `json:"code"`
	Date              string  `json:"date"`
	TotalAmount       float64 `json:"total_amount"`
	OutstandingAmount float64 `json:"outstanding_amount"`
	DueDate           *string `json:"due_date,omitempty"`
}

// GetPaymentSummary gets payment summary statistics
func (s *PaymentService) GetPaymentSummary(startDate, endDate string) (*repositories.PaymentSummary, error) {
	return s.paymentRepo.GetPaymentSummary(startDate, endDate)
}

// GetPaymentAnalytics gets comprehensive payment analytics for dashboard
func (s *PaymentService) GetPaymentAnalytics(startDate, endDate string) (*PaymentAnalytics, error) {
	// Get basic summary first
	summary, err := s.paymentRepo.GetPaymentSummary(startDate, endDate)
	if err != nil {
		return nil, err
	}

	// Get recent payments for analytics
	recentPayments, err := s.paymentRepo.GetPaymentsByDateRange(
		time.Now().AddDate(0, 0, -30), // Last 30 days
		time.Now(),
	)
	if err != nil {
		return nil, err
	}

	// Create analytics response
	analytics := &PaymentAnalytics{
		TotalReceived:    summary.TotalReceived,
		TotalPaid:        summary.TotalPaid,
		NetFlow:          summary.NetFlow,
		ReceivedGrowth:   0, // TODO: Calculate growth percentage
		PaidGrowth:       0, // TODO: Calculate growth percentage
		FlowGrowth:       0, // TODO: Calculate growth percentage
		TotalOutstanding: 0, // TODO: Calculate outstanding amount
		ByMethod:         summary.ByMethod,
		DailyTrend:       s.generateDailyTrend(startDate, endDate),
		RecentPayments:   recentPayments,
		AvgPaymentTime:   2.5,  // TODO: Calculate actual processing time
		SuccessRate:      95.0, // TODO: Calculate actual success rate
	}

	return analytics, nil
}

// generateDailyTrend generates daily payment trend data
func (s *PaymentService) generateDailyTrend(startDate, endDate string) []DailyTrend {
	// Parse dates
	start, _ := time.Parse("2006-01-02", startDate)
	end, _ := time.Parse("2006-01-02", endDate)

	var trends []DailyTrend

	// Generate daily data points
	for d := start; d.Before(end) || d.Equal(end); d = d.AddDate(0, 0, 1) {
		// TODO: Get actual daily payment data from database
		// For now, generate mock data
		trends = append(trends, DailyTrend{
			Date:     d.Format("2006-01-02"),
			Received: 0, // TODO: Get actual received amount
			Paid:     0, // TODO: Get actual paid amount
		})
	}

	return trends
}

// PaymentAnalytics struct for analytics response
type PaymentAnalytics struct {
	TotalReceived    float64            `json:"total_received"`
	TotalPaid        float64            `json:"total_paid"`
	NetFlow          float64            `json:"net_flow"`
	ReceivedGrowth   float64            `json:"received_growth"`
	PaidGrowth       float64            `json:"paid_growth"`
	FlowGrowth       float64            `json:"flow_growth"`
	TotalOutstanding float64            `json:"total_outstanding"`
	ByMethod         map[string]float64 `json:"by_method"`
	DailyTrend       []DailyTrend       `json:"daily_trend"`
	RecentPayments   []models.Payment   `json:"recent_payments"`
	AvgPaymentTime   float64            `json:"avg_payment_time"`
	SuccessRate      float64            `json:"success_rate"`
}

// DailyTrend represents daily payment trend data
type DailyTrend struct {
	Date     string  `json:"date"`
	Received float64 `json:"received"`
	Paid     float64 `json:"paid"`
}

// PaymentFilter and PaymentResult are defined in repositories package

// Export functions

// ExportPaymentReportExcel generates an Excel report for payments
func (s *PaymentService) ExportPaymentReportExcel(startDate, endDate, status, method string) ([]byte, string, error) {
	// Create filter for payments
	filter := repositories.PaymentFilter{
		Status: status,
		Method: method,
		Page:   1,
		Limit:  10000, // Get all payments for report
	}

	// Parse dates if provided
	if startDate != "" {
		if sd, err := time.Parse("2006-01-02", startDate); err == nil {
			filter.StartDate = sd
		}
	}
	if endDate != "" {
		if ed, err := time.Parse("2006-01-02", endDate); err == nil {
			filter.EndDate = ed
		}
	}

	// Get payments data
	result, err := s.paymentRepo.FindWithFilter(filter)
	if err != nil {
		return nil, "", err
	}

	// Generate Excel using existing export service
	excelData, err := s.generatePaymentExcel(result.Data, startDate, endDate, status, method)
	if err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("Payment_Report_%s_to_%s.xlsx", startDate, endDate)
	if startDate == "" {
		filename = "Payment_Report_All_Time.xlsx"
	}

	return excelData, filename, nil
}

// generatePaymentExcel creates Excel file for payments
func (s *PaymentService) generatePaymentExcel(payments []models.Payment, startDate, endDate, status, method string) ([]byte, error) {
	// Import excelize
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			// Log error
		}
	}()

	sheetName := "Payment Report"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to create Excel sheet: %v", err)
	}

	// Set active sheet
	f.SetActiveSheet(index)

	// Set title
	f.SetCellValue(sheetName, "A1", "PAYMENT REPORT")
	f.SetCellValue(sheetName, "A2", fmt.Sprintf("Generated on: %s", time.Now().Format("2006-01-02 15:04:05")))

	// Add filter information
	row := 3
	if startDate != "" && endDate != "" {
		f.SetCellValue(sheetName, "A"+strconv.Itoa(row), fmt.Sprintf("Period: %s to %s", startDate, endDate))
		row++
	}
	if status != "" {
		f.SetCellValue(sheetName, "A"+strconv.Itoa(row), fmt.Sprintf("Status Filter: %s", status))
		row++
	}
	if method != "" {
		f.SetCellValue(sheetName, "A"+strconv.Itoa(row), fmt.Sprintf("Method Filter: %s", method))
		row++
	}

	// Headers row
	headerRow := row + 1
	headers := []string{"Date", "Payment Code", "Contact", "Contact Type", "Amount", "Method", "Status", "Reference", "Notes", "Created At"}
	for i, header := range headers {
		cell := string(rune('A'+i)) + strconv.Itoa(headerRow)
		f.SetCellValue(sheetName, cell, header)
	}

	// Style for headers
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Size:  12,
			Color: "FFFFFF",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#4472C4"},
			Pattern: 1,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create header style: %v", err)
	}

	// Apply style to headers
	f.SetCellStyle(sheetName, "A"+strconv.Itoa(headerRow), "J"+strconv.Itoa(headerRow), headerStyle)

	// Data style
	dataStyle, err := f.NewStyle(&excelize.Style{
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create data style: %v", err)
	}

	// Currency style
	currencyStyle, err := f.NewStyle(&excelize.Style{
		NumFmt: 4, // Currency format
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create currency style: %v", err)
	}

	// Fill data
	totalAmount := 0.0
	completedCount := 0
	pendingCount := 0
	failedCount := 0

	for i, payment := range payments {
		dataRow := headerRow + 1 + i

		contactName := "N/A"
		contactType := "N/A"
		if payment.Contact.ID != 0 {
			contactName = payment.Contact.Name
			contactType = payment.Contact.Type
		}

		// Set cell values
		f.SetCellValue(sheetName, "A"+strconv.Itoa(dataRow), payment.Date.Format("2006-01-02"))
		f.SetCellValue(sheetName, "B"+strconv.Itoa(dataRow), payment.Code)
		f.SetCellValue(sheetName, "C"+strconv.Itoa(dataRow), contactName)
		f.SetCellValue(sheetName, "D"+strconv.Itoa(dataRow), contactType)
		f.SetCellValue(sheetName, "E"+strconv.Itoa(dataRow), payment.Amount)
		f.SetCellValue(sheetName, "F"+strconv.Itoa(dataRow), payment.Method)
		f.SetCellValue(sheetName, "G"+strconv.Itoa(dataRow), payment.Status)
		f.SetCellValue(sheetName, "H"+strconv.Itoa(dataRow), payment.Reference)
		f.SetCellValue(sheetName, "I"+strconv.Itoa(dataRow), payment.Notes)
		f.SetCellValue(sheetName, "J"+strconv.Itoa(dataRow), payment.CreatedAt.Format("2006-01-02 15:04:05"))

		// Apply styles
		f.SetCellStyle(sheetName, "A"+strconv.Itoa(dataRow), "D"+strconv.Itoa(dataRow), dataStyle)
		f.SetCellStyle(sheetName, "E"+strconv.Itoa(dataRow), "E"+strconv.Itoa(dataRow), currencyStyle)
		f.SetCellStyle(sheetName, "F"+strconv.Itoa(dataRow), "J"+strconv.Itoa(dataRow), dataStyle)

		// Accumulate statistics
		totalAmount += payment.Amount
		switch payment.Status {
		case "COMPLETED":
			completedCount++
		case "PENDING":
			pendingCount++
		case "FAILED":
			failedCount++
		}
	}

	// Summary section
	summaryRow := headerRow + len(payments) + 3
	f.SetCellValue(sheetName, "A"+strconv.Itoa(summaryRow), "SUMMARY")
	f.SetCellStyle(sheetName, "A"+strconv.Itoa(summaryRow), "A"+strconv.Itoa(summaryRow), headerStyle)

	summaryRow++
	f.SetCellValue(sheetName, "A"+strconv.Itoa(summaryRow), "Total Payments:")
	f.SetCellValue(sheetName, "B"+strconv.Itoa(summaryRow), len(payments))

	summaryRow++
	f.SetCellValue(sheetName, "A"+strconv.Itoa(summaryRow), "Total Amount:")
	f.SetCellValue(sheetName, "B"+strconv.Itoa(summaryRow), totalAmount)
	f.SetCellStyle(sheetName, "B"+strconv.Itoa(summaryRow), "B"+strconv.Itoa(summaryRow), currencyStyle)

	summaryRow++
	f.SetCellValue(sheetName, "A"+strconv.Itoa(summaryRow), "Completed:")
	f.SetCellValue(sheetName, "B"+strconv.Itoa(summaryRow), completedCount)

	summaryRow++
	f.SetCellValue(sheetName, "A"+strconv.Itoa(summaryRow), "Pending:")
	f.SetCellValue(sheetName, "B"+strconv.Itoa(summaryRow), pendingCount)

	summaryRow++
	f.SetCellValue(sheetName, "A"+strconv.Itoa(summaryRow), "Failed:")
	f.SetCellValue(sheetName, "B"+strconv.Itoa(summaryRow), failedCount)

	if len(payments) > 0 {
		summaryRow++
		f.SetCellValue(sheetName, "A"+strconv.Itoa(summaryRow), "Average Amount:")
		f.SetCellValue(sheetName, "B"+strconv.Itoa(summaryRow), totalAmount/float64(len(payments)))
		f.SetCellStyle(sheetName, "B"+strconv.Itoa(summaryRow), "B"+strconv.Itoa(summaryRow), currencyStyle)
	}

	// Auto-fit columns
	cols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J"}
	for _, col := range cols {
		f.SetColWidth(sheetName, col, col, 15)
	}

	// Make specific columns wider
	f.SetColWidth(sheetName, "C", "C", 25) // Contact name
	f.SetColWidth(sheetName, "H", "H", 20) // Reference
	f.SetColWidth(sheetName, "I", "I", 30) // Notes
	f.SetColWidth(sheetName, "J", "J", 20) // Created at

	// Delete default Sheet1 if it exists
	if f.GetSheetName(0) == "Sheet1" {
		f.DeleteSheet("Sheet1")
	}

	// Save to buffer
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("failed to write Excel file: %v", err)
	}

	return buf.Bytes(), nil
}

// ExportPaymentReportPDF generates a PDF report for payments
func (s *PaymentService) ExportPaymentReportPDF(startDate, endDate, status, method string) ([]byte, string, error) {
	// Create filter for payments
	filter := repositories.PaymentFilter{
		Status: status,
		Method: method,
		Page:   1,
		Limit:  1000, // Get all payments for report
	}

	// Parse dates if provided
	if startDate != "" {
		if sd, err := time.Parse("2006-01-02", startDate); err == nil {
			filter.StartDate = sd
		}
	}
	if endDate != "" {
		if ed, err := time.Parse("2006-01-02", endDate); err == nil {
			filter.EndDate = ed
		}
	}

	// Get payments data
	result, err := s.paymentRepo.FindWithFilter(filter)
	if err != nil {
		return nil, "", err
	}

	// Generate PDF using existing PDF service
	pdfService := NewPDFService(s.db)
	pdfData, err := pdfService.GeneratePaymentReportPDF(result.Data)
	if err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("Payment_Report_%s_to_%s.pdf", startDate, endDate)
	if startDate == "" {
		filename = "Payment_Report_All_Time.pdf"
	}

	return pdfData, filename, nil
}

// ExportPaymentDetailPDF generates a PDF for a single payment detail
func (s *PaymentService) ExportPaymentDetailPDF(paymentID uint) ([]byte, string, error) {
	// Get payment details
	payment, err := s.paymentRepo.FindByID(paymentID)
	if err != nil {
		return nil, "", err
	}

	// Generate PDF using existing PDF service
	pdfService := NewPDFService(s.db)
	pdfData, err := pdfService.GeneratePaymentDetailPDF(payment)
	if err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("Payment_%s.pdf", payment.Code)
	return pdfData, filename, nil
}

// updateCashBankBalanceWithLogging - DEPRECATED: Use SingleSourcePostingService instead
// This method is kept for backward compatibility but should NOT be used for new code
// WARNING: Using this method directly can cause double posting!
func (s *PaymentService) updateCashBankBalanceWithLogging(tx *gorm.DB, cashBankID uint, amount float64, direction string, referenceID uint, userID uint) error {
	// 🔒 PRODUCTION GUARD: Block deprecated methods in production
	statusValidator := NewStatusValidationHelper()
	if err := statusValidator.ValidateDeprecatedMethodUsage("updateCashBankBalanceWithLogging"); err != nil {
		log.Printf("❌ Deprecated method blocked: %v", err)
		return err
	}

	log.Printf("⚠️ WARNING: Using deprecated updateCashBankBalanceWithLogging - use SalesJournalServiceV2 instead!")
	log.Printf("💰 Updating Cash/Bank Balance: ID=%d, Amount=%.2f, Direction=%s", cashBankID, amount, direction)

	var cashBank models.CashBank
	if err := tx.First(&cashBank, cashBankID).Error; err != nil {
		return fmt.Errorf("cash/bank account not found: %v", err)
	}

	log.Printf("💰 Current balance: %.2f -> %.2f", cashBank.Balance, cashBank.Balance+amount)

	// Update balance
	newBalance := cashBank.Balance + amount

	// Safety check - only prevent negative balance for outgoing payments (withdrawals)
	// For incoming payments (receivables), allow negative balance to become positive
	if newBalance < 0 && amount < 0 {
		// Only block if this is a withdrawal/payment OUT that would make balance negative
		return fmt.Errorf("insufficient balance for withdrawal. Current: %.2f, Required: %.2f, Shortfall: %.2f",
			cashBank.Balance, -amount, -newBalance)
	}

	cashBank.Balance = newBalance

	if err := tx.Save(&cashBank).Error; err != nil {
		return fmt.Errorf("failed to save cash/bank balance: %v", err)
	}

	log.Printf("✅ Balance updated successfully: %.2f", cashBank.Balance)

	// Create transaction record
	transaction := &models.CashBankTransaction{
		CashBankID:      cashBankID,
		ReferenceType:   "PAYMENT",
		ReferenceID:     referenceID,
		Amount:          amount,
		BalanceAfter:    cashBank.Balance,
		TransactionDate: time.Now(),
		Notes:           fmt.Sprintf("Payment %s", direction),
	}

	if err := tx.Create(transaction).Error; err != nil {
		return fmt.Errorf("failed to create cash/bank transaction: %v", err)
	}

	log.Printf("✅ Cash/bank transaction recorded")
	return nil
}

// createReceivablePaymentJournalWithSSOT - DEPRECATED: Use SingleSourcePostingService instead
// WARNING: This method can cause double posting if used with manual balance updates!
func (s *PaymentService) createReceivablePaymentJournalWithSSOT(tx *gorm.DB, payment *models.Payment, cashBankID uint, userID uint) error {
	// 🔒 PRODUCTION GUARD: Block deprecated methods in production
	statusValidator := NewStatusValidationHelper()
	if err := statusValidator.ValidateDeprecatedMethodUsage("createReceivablePaymentJournalWithSSOT"); err != nil {
		log.Printf("❌ Deprecated method blocked: %v", err)
		return err
	}

	log.Printf("⚠️ WARNING: Using deprecated createReceivablePaymentJournalWithSSOT - use SalesJournalServiceV2 instead!")
	log.Printf("📋 Creating SSOT journal entries for payment %d", payment.ID)

	// Initialize unified journal service - using transaction-safe approach
	journalService := NewUnifiedJournalService(s.db)

	// Get accounts
	var cashBankAccountID uint64
	if cashBankID > 0 {
		var cashBank models.CashBank
		if err := tx.First(&cashBank, cashBankID).Error; err != nil {
			return fmt.Errorf("cash/bank account not found: %v", err)
		}
		cashBankAccountID = uint64(cashBank.AccountID)
		log.Printf("📋 Using Cash/Bank Account ID: %d", cashBankAccountID)
	} else {
		var kasAccount models.Account
		if err := tx.Where("code = ?", "1101").First(&kasAccount).Error; err != nil {
			return fmt.Errorf("default cash account (1101) not found: %v", err)
		}
		cashBankAccountID = uint64(kasAccount.ID)
		log.Printf("📋 Using default Cash Account ID: %d", cashBankAccountID)
	}

	// Get AR account (Piutang Usaha - Try 1102-001 first, then 1102, then 1201)
	var arAccount models.Account
	// Try primary active AR account first
	if err := tx.Where("code = ?", "1102-001").First(&arAccount).Error; err != nil {
		// Try general AR account second
		if err := tx.Where("code = ?", "1102").First(&arAccount).Error; err != nil {
			// Try legacy AR account last
			if err := tx.Where("code = ?", "1201").First(&arAccount).Error; err != nil {
				log.Printf("⚠️ AR account exact codes not found, trying fallback")
				// Fallback: try to find by name pattern
				if err := tx.Where("LOWER(name) LIKE ?", "%piutang%usaha%").First(&arAccount).Error; err != nil {
					return fmt.Errorf("accounts receivable account not found: %v", err)
				}
			}
		}
	}
	log.Printf("📋 Using AR Account ID: %d (Code: %s)", arAccount.ID, arAccount.Code)

	// Create journal lines for SSOT system
	journalLines := []JournalLineRequest{
		{
			AccountID:    cashBankAccountID,
			Description:  fmt.Sprintf("Payment received - %s", payment.Code),
			DebitAmount:  decimal.NewFromFloat(payment.Amount),
			CreditAmount: decimal.Zero,
		},
		{
			AccountID:    uint64(arAccount.ID),
			Description:  fmt.Sprintf("AR reduction - %s", payment.Code),
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.NewFromFloat(payment.Amount),
		},
	}

	// Create SSOT journal entry request
	paymentID := uint64(payment.ID)
	journalRequest := &JournalEntryRequest{
		SourceType:  models.SSOTSourceTypePayment,
		SourceID:    paymentID,
		Reference:   payment.Code,
		EntryDate:   payment.Date,
		Description: fmt.Sprintf("Customer Payment %s", payment.Code),
		Lines:       journalLines,
		AutoPost:    true,
		CreatedBy:   uint64(userID),
	}

	// Create the SSOT journal entry using existing transaction to prevent deadlock
	log.Printf("🗺️ Creating SSOT journal entry within existing transaction")
	journalResponse, err := journalService.CreateJournalEntryWithTx(tx, journalRequest)
	if err != nil {
		return fmt.Errorf("failed to create SSOT journal entry: %v", err)
	}

	log.Printf("✅ SSOT Journal entry created: ID=%d, EntryNumber=%s", journalResponse.ID, journalResponse.EntryNumber)

	// Update payment with journal entry reference
	journalEntryID := uint(journalResponse.ID)
	payment.JournalEntryID = &journalEntryID
	if err := tx.Save(payment).Error; err != nil {
		log.Printf("⚠️ Warning: Failed to update payment with journal reference: %v", err)
		// Don't fail the transaction for this, as journal entry is already created
	}

	return nil
}

// createReceivablePaymentJournalWithSSOTFixed - DEPRECATED: Use SingleSourcePostingService instead
// This method was created to fix double posting but is now superseded by SingleSourcePostingService
// WARNING: Complex logic that can still cause issues - use SingleSourcePostingService for all new code!
func (s *PaymentService) createReceivablePaymentJournalWithSSOTFixed(tx *gorm.DB, payment *models.Payment, cashBankID uint, userID uint, cashBankAlreadyUpdated bool) error {
	// 🔒 PRODUCTION GUARD: Block deprecated methods in production
	statusValidator := NewStatusValidationHelper()
	if err := statusValidator.ValidateDeprecatedMethodUsage("createReceivablePaymentJournalWithSSOTFixed"); err != nil {
		log.Printf("❌ Deprecated method blocked: %v", err)
		return err
	}

	log.Printf("⚠️ WARNING: Using deprecated createReceivablePaymentJournalWithSSOTFixed - use SalesJournalServiceV2 instead!")
	log.Printf("📋 Creating SSOT journal entries for payment %d (cash bank updated: %v)", payment.ID, cashBankAlreadyUpdated)

	// Initialize unified journal service - using transaction-safe approach
	journalService := NewUnifiedJournalService(s.db)

	// Get accounts
	var cashBankAccountID uint64
	if cashBankID > 0 {
		var cashBank models.CashBank
		if err := tx.First(&cashBank, cashBankID).Error; err != nil {
			return fmt.Errorf("cash/bank account not found: %v", err)
		}
		cashBankAccountID = uint64(cashBank.AccountID)
		log.Printf("📋 Using Cash/Bank Account ID: %d", cashBankAccountID)
	} else {
		var kasAccount models.Account
		if err := tx.Where("code = ?", "1101").First(&kasAccount).Error; err != nil {
			return fmt.Errorf("default cash account (1101) not found: %v", err)
		}
		cashBankAccountID = uint64(kasAccount.ID)
		log.Printf("📋 Using default Cash Account ID: %d", cashBankAccountID)
	}

	// Get AR account (Piutang Usaha - Try 1102-001 first, then 1102, then 1201)
	var arAccount models.Account
	// Try primary active AR account first
	if err := tx.Where("code = ?", "1102-001").First(&arAccount).Error; err != nil {
		// Try general AR account second
		if err := tx.Where("code = ?", "1102").First(&arAccount).Error; err != nil {
			// Try legacy AR account last
			if err := tx.Where("code = ?", "1201").First(&arAccount).Error; err != nil {
				log.Printf("⚠️ AR account exact codes not found, trying fallback")
				// Fallback: try to find by name pattern
				if err := tx.Where("LOWER(name) LIKE ?", "%piutang%usaha%").First(&arAccount).Error; err != nil {
					return fmt.Errorf("accounts receivable account not found: %v", err)
				}
			}
		}
	}
	log.Printf("📋 Using AR Account ID: %d (Code: %s)", arAccount.ID, arAccount.Code)

	// Create journal lines for SSOT system
	journalLines := []JournalLineRequest{
		{
			AccountID:    cashBankAccountID,
			Description:  fmt.Sprintf("Payment received - %s", payment.Code),
			DebitAmount:  decimal.NewFromFloat(payment.Amount),
			CreditAmount: decimal.Zero,
		},
		{
			AccountID:    uint64(arAccount.ID),
			Description:  fmt.Sprintf("AR reduction - %s", payment.Code),
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.NewFromFloat(payment.Amount),
		},
	}

	// Create SSOT journal entry request
	paymentID := uint64(payment.ID)
	journalRequest := &JournalEntryRequest{
		SourceType:  models.SSOTSourceTypePayment,
		SourceID:    paymentID,
		Reference:   payment.Code,
		EntryDate:   payment.Date,
		Description: fmt.Sprintf("Customer Payment %s", payment.Code),
		Lines:       journalLines,
		AutoPost:    !cashBankAlreadyUpdated, // Disable AutoPost if cash bank already updated
		CreatedBy:   uint64(userID),
	}

	// If CashBank was already updated, we need to sync the GL account balance
	// to match the CashBank balance to prevent inconsistency
	if cashBankAlreadyUpdated {
		log.Printf("🔄 CashBank already updated, syncing GL account balance after journal creation")
	}

	// Create the SSOT journal entry using existing transaction to prevent deadlock
	log.Printf("🗺️ Creating SSOT journal entry within existing transaction")
	journalResponse, err := journalService.CreateJournalEntryWithTx(tx, journalRequest)
	if err != nil {
		return fmt.Errorf("failed to create SSOT journal entry: %v", err)
	}

	log.Printf("✅ SSOT Journal entry created: ID=%d, EntryNumber=%s", journalResponse.ID, journalResponse.EntryNumber)

	// BALANCE SYNC LOGIC: Handle different scenarios
	if cashBankAlreadyUpdated {
		// Manual CashBank update was done, but SSOT should NOT update balances
		// to prevent double counting. Just log this case.
		log.Printf("🔄 CashBank was manually updated, SSOT journal created for audit trail only")
		log.Printf("⚠️ WARNING: Manual + SSOT update may cause double balance - this should be avoided")
	} else {
		// Normal flow: SSOT system handles all balance updates via AutoPost
		log.Printf("🔄 AutoPost enabled: SSOT system handling all account balance updates")
		log.Printf("✅ This is the correct flow to prevent double counting")
	}

	// Update payment with journal entry reference
	journalEntryID := uint(journalResponse.ID)
	payment.JournalEntryID = &journalEntryID
	if err := tx.Save(payment).Error; err != nil {
		log.Printf("⚠️ Warning: Failed to update payment with journal reference: %v", err)
		// Don't fail the transaction for this, as journal entry is already created
	}

	return nil
}

// Legacy method - kept for backward compatibility but now uses SSOT
func (s *PaymentService) createReceivablePaymentJournalWithLogging(tx *gorm.DB, payment *models.Payment, cashBankID uint, userID uint) error {
	// Delegate to SSOT implementation
	return s.createReceivablePaymentJournalWithSSOT(tx, payment, cashBankID, userID)
}
