package services

import (
	"fmt"
	"math"
	"strings"

	"app-sistem-akuntansi/models"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const customerSettlementTolerance = 0.005

// CustomerPaymentSettlementSummary is returned with a receipt so the UI can
// explain exactly how a bank-vs-invoice difference was handled.
type CustomerPaymentSettlementSummary struct {
	Treatment               string  `json:"treatment"`
	InvoiceSettlementAmount float64 `json:"invoice_settlement_amount"`
	Difference              float64 `json:"difference"`
	CustomerAdvanceAmount   float64 `json:"customer_advance_amount"`
	RoundingAmount          float64 `json:"rounding_amount"`
	RoundingDirection       string  `json:"rounding_direction,omitempty"`
	RequiresApproval        bool    `json:"requires_approval"`
	RoundingThreshold       float64 `json:"rounding_threshold"`
}

type customerSettlementPlan struct {
	CustomerPaymentSettlementSummary
	primaryInvoiceID *uint
	allocationCount  int
	hasNonIDRInvoice bool
}

func normalizeCustomerSettlementTreatment(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case models.PaymentSettlementCustomerAdvance, "ADVANCE":
		return models.PaymentSettlementCustomerAdvance
	case models.PaymentSettlementRounding, "ROUNDING_REQUEST":
		return models.PaymentSettlementRounding
	case "", models.PaymentSettlementOpenBalance, "OPEN", "KEEP_OPEN":
		return models.PaymentSettlementOpenBalance
	default:
		return strings.ToUpper(strings.TrimSpace(value))
	}
}

func (eps *EnhancedPaymentServiceWithJournal) customerPaymentRoundingThreshold(tx *gorm.DB) (float64, error) {
	const defaultThreshold = 1000.0
	var settings models.Settings
	err := tx.First(&settings).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return 0, fmt.Errorf("load payment rounding settings: %w", err)
	}
	if err == gorm.ErrRecordNotFound {
		return defaultThreshold, nil
	}

	// Zero is intentional: it disables the rounding option while keeping the
	// safe default of customer advance available for excess receipts.
	return settings.PaymentRoundingThreshold, nil
}

// planCustomerSettlement derives the commercial settlement target from the
// requested invoice allocations before any data is posted.  The invoice is
// never edited; a positive difference becomes advance by default and a
// rounding request is limited to the configured small-difference threshold.
func (eps *EnhancedPaymentServiceWithJournal) planCustomerSettlement(
	tx *gorm.DB,
	req *PaymentWithJournalRequest,
	contact *models.Contact,
) (*customerSettlementPlan, error) {
	if tx == nil || req == nil || contact == nil {
		return nil, fmt.Errorf("customer settlement requires transaction, request, and contact")
	}

	plan := &customerSettlementPlan{}
	threshold, err := eps.customerPaymentRoundingThreshold(tx)
	if err != nil {
		return nil, err
	}
	plan.RoundingThreshold = threshold
	treatment := normalizeCustomerSettlementTreatment(req.SettlementTreatment)
	if treatment != models.PaymentSettlementOpenBalance &&
		treatment != models.PaymentSettlementCustomerAdvance &&
		treatment != models.PaymentSettlementRounding {
		return nil, fmt.Errorf("invalid customer settlement treatment: %s", req.SettlementTreatment)
	}

	seenInvoices := make(map[uint]struct{})
	addInvoiceTarget := func(invoiceID uint, requestedAmount float64) error {
		if invoiceID == 0 {
			return fmt.Errorf("invoice ID is required for customer settlement")
		}
		if _, exists := seenInvoices[invoiceID]; exists {
			return fmt.Errorf("invoice %d appears more than once in one receipt", invoiceID)
		}

		var sale models.Sale
		if err := tx.First(&sale, invoiceID).Error; err != nil {
			return fmt.Errorf("invoice %d not found: %w", invoiceID, err)
		}
		if sale.CustomerID != contact.ID {
			return fmt.Errorf("invoice %d does not belong to customer %d", invoiceID, contact.ID)
		}
		if err := eps.statusValidator.ValidatePaymentAllocation(sale.Status, sale.ID); err != nil {
			return err
		}
		if currency := strings.ToUpper(strings.TrimSpace(sale.Currency)); currency != "" && currency != "IDR" {
			plan.hasNonIDRInvoice = true
		}

		// A short-receipt rounding request still needs the full invoice balance
		// as its commercial settlement target. The allocation itself remains
		// capped at the actual bank receipt later, and the approved rounding
		// line closes only the remaining difference.
		target := requestedAmount
		if treatment == models.PaymentSettlementRounding {
			target = sale.OutstandingAmount
		} else {
			if target <= 0 {
				target = sale.OutstandingAmount
			}
			if target > sale.OutstandingAmount {
				target = sale.OutstandingAmount
			}
		}
		if target <= customerSettlementTolerance {
			return fmt.Errorf("invoice %d no longer has an outstanding balance", invoiceID)
		}

		seenInvoices[invoiceID] = struct{}{}
		plan.InvoiceSettlementAmount += target
		plan.allocationCount++
		if plan.primaryInvoiceID == nil {
			copyID := invoiceID
			plan.primaryInvoiceID = &copyID
		}
		return nil
	}

	if len(req.InvoiceAllocations) > 0 {
		for _, allocation := range req.InvoiceAllocations {
			if allocation.InvoiceID == nil {
				return nil, fmt.Errorf("invoice allocation is missing invoice_id")
			}
			if err := addInvoiceTarget(*allocation.InvoiceID, allocation.Amount); err != nil {
				return nil, err
			}
		}
	} else if req.TargetInvoiceID != nil {
		if err := addInvoiceTarget(*req.TargetInvoiceID, req.Amount); err != nil {
			return nil, err
		}
	}

	plan.Difference = req.Amount - plan.InvoiceSettlementAmount

	switch {
	case plan.Difference > customerSettlementTolerance:
		if treatment == models.PaymentSettlementOpenBalance {
			treatment = models.PaymentSettlementCustomerAdvance
		}
		if treatment == models.PaymentSettlementCustomerAdvance {
			plan.CustomerAdvanceAmount = plan.Difference
		}
	case plan.Difference < -customerSettlementTolerance:
		// OPEN_BALANCE is the safe default: the invoice stays partially paid.
		if treatment == models.PaymentSettlementCustomerAdvance {
			return nil, fmt.Errorf("customer advance can only be used for an excess receipt")
		}
	default:
		if treatment == models.PaymentSettlementCustomerAdvance || treatment == models.PaymentSettlementRounding {
			return nil, fmt.Errorf("settlement treatment is only available when bank amount differs from invoice")
		}
	}

	if treatment == models.PaymentSettlementRounding {
		if math.Abs(plan.Difference) <= customerSettlementTolerance {
			return nil, fmt.Errorf("rounding is not needed because the receipt equals the invoice settlement")
		}
		if plan.allocationCount != 1 || plan.primaryInvoiceID == nil {
			return nil, fmt.Errorf("rounding settlement is only allowed for one invoice at a time")
		}
		if plan.hasNonIDRInvoice {
			return nil, fmt.Errorf("rounding settlement is currently only available for IDR invoices")
		}
		if math.Abs(plan.Difference)-threshold > customerSettlementTolerance {
			return nil, fmt.Errorf("rounding difference %.2f exceeds configured threshold %.2f", math.Abs(plan.Difference), threshold)
		}
		if strings.TrimSpace(req.SettlementReason) == "" {
			return nil, fmt.Errorf("reason is required when requesting rounding settlement")
		}
		plan.RoundingAmount = math.Abs(plan.Difference)
		if plan.Difference > 0 {
			plan.RoundingDirection = "OVERPAYMENT"
		} else {
			plan.RoundingDirection = "SHORTFALL"
		}
		plan.RequiresApproval = true
	}

	plan.Treatment = treatment
	return plan, nil
}

func (eps *EnhancedPaymentServiceWithJournal) resolveCustomerAdvanceAccountID(tx *gorm.DB) (uint64, error) {
	return eps.resolveCustomerSettlementAccountID(tx, "2190-001", models.AccountTypeLiability, "customer advance")
}

func (eps *EnhancedPaymentServiceWithJournal) resolveRoundingAccountID(tx *gorm.DB, isIncome bool) (uint64, error) {
	if isIncome {
		return eps.resolveCustomerSettlementAccountID(tx, "7106", models.AccountTypeRevenue, "rounding income")
	}
	return eps.resolveCustomerSettlementAccountID(tx, "8106", models.AccountTypeExpense, "rounding expense")
}

func (eps *EnhancedPaymentServiceWithJournal) resolveCustomerSettlementAccountID(
	tx *gorm.DB,
	code string,
	expectedType string,
	label string,
) (uint64, error) {
	var account models.Account
	if err := tx.Where("code = ? AND deleted_at IS NULL", code).First(&account).Error; err != nil {
		return 0, fmt.Errorf("%s account %s is not configured; restart backend migration or create the COA", label, code)
	}
	if !account.IsActive || account.IsHeader || !strings.EqualFold(account.Type, expectedType) {
		return 0, fmt.Errorf("%s account %s is not a usable %s leaf account", label, code, expectedType)
	}
	return uint64(account.ID), nil
}

func (eps *EnhancedPaymentServiceWithJournal) loadStoredReceivableSettlement(
	tx *gorm.DB,
	payment *models.Payment,
) (allocatedAmount float64, receivableBookAmount float64, primaryInvoiceID *uint, allocationCount int, err error) {
	if payment == nil {
		err = fmt.Errorf("payment is required")
		return
	}

	var allocations []models.PaymentAllocation
	if err = tx.Where("payment_id = ?", payment.ID).Order("id ASC").Find(&allocations).Error; err != nil {
		err = fmt.Errorf("load payment allocations: %w", err)
		return
	}

	for _, allocation := range allocations {
		if allocation.InvoiceID == nil || *allocation.InvoiceID == 0 || allocation.AllocatedAmount <= customerSettlementTolerance {
			continue
		}

		var sale models.Sale
		if err = tx.First(&sale, *allocation.InvoiceID).Error; err != nil {
			err = fmt.Errorf("load invoice %d for payment settlement: %w", *allocation.InvoiceID, err)
			return
		}
		if sale.CustomerID != payment.ContactID {
			err = fmt.Errorf("invoice %d does not belong to payment customer", sale.ID)
			return
		}

		settlementBook, _, calculationErr := calculateSaleReceivableSettlementBookAmount(&sale, allocation.AllocatedAmount, payment.ExchangeRate)
		if calculationErr != nil {
			err = fmt.Errorf("calculate invoice settlement %d: %w", sale.ID, calculationErr)
			return
		}
		allocatedAmount += allocation.AllocatedAmount
		receivableBookAmount += settlementBook
		allocationCount++
		if primaryInvoiceID == nil {
			copyID := sale.ID
			primaryInvoiceID = &copyID
		}
	}

	return
}

func customerSettlementBookAmount(payment *models.Payment, amount float64) float64 {
	if payment == nil || amount <= customerSettlementTolerance || payment.Amount <= customerSettlementTolerance {
		return 0
	}
	bankBookAmount := payment.BookAmountIDR
	if bankBookAmount == 0 {
		bankBookAmount = payment.Amount
	}
	return bankBookAmount * (amount / payment.Amount)
}

func (eps *EnhancedPaymentServiceWithJournal) createCustomerAdvance(
	tx *gorm.DB,
	payment *models.Payment,
	plan *customerSettlementPlan,
	journalEntryID *uint64,
	saleID *uint,
) (*models.CustomerAdvance, error) {
	if payment == nil || plan == nil || plan.CustomerAdvanceAmount <= customerSettlementTolerance {
		return nil, nil
	}

	advance := &models.CustomerAdvance{
		CustomerID:      payment.ContactID,
		PaymentID:       payment.ID,
		SaleID:          saleID,
		JournalEntryID:  journalEntryID,
		OriginalAmount:  plan.CustomerAdvanceAmount,
		RemainingAmount: plan.CustomerAdvanceAmount,
		// Payment.Amount and the liability journal are held in the base ledger
		// currency (IDR), even when the selected cash/bank account is USD.
		// Do not label this IDR book balance as USD merely because the bank
		// account has a native USD balance.
		Currency:     "IDR",
		ExchangeRate: 1,
		Status:       models.CustomerAdvanceStatusOpen,
		Notes:        fmt.Sprintf("Kelebihan penerimaan %s dicatat sebagai uang muka pelanggan", payment.Code),
		CreatedBy:    payment.UserID,
	}
	if err := tx.Create(advance).Error; err != nil {
		return nil, fmt.Errorf("create customer advance: %w", err)
	}
	return advance, nil
}

// buildCustomerReceiptJournalLines keeps the cash receipt and customer
// settlement in one balanced journal.  It deliberately never credits AR for
// receipt value that is not allocated to an invoice.
func (eps *EnhancedPaymentServiceWithJournal) buildCustomerReceiptJournalLines(
	tx *gorm.DB,
	payment *models.Payment,
	cashBank *models.CashBank,
	contactName string,
	receivableSettlementBookAmount float64,
	allocatedAmount float64,
) ([]JournalLineRequest, string, error) {
	if payment == nil || cashBank == nil || cashBank.AccountID == 0 {
		return nil, "", fmt.Errorf("payment and linked cash/bank account are required")
	}

	bankBookAmount := payment.BookAmountIDR
	if bankBookAmount <= customerSettlementTolerance {
		bankBookAmount = payment.Amount
	}
	if bankBookAmount <= customerSettlementTolerance {
		return nil, "", fmt.Errorf("payment book amount must be positive")
	}
	contactName = strings.TrimSpace(contactName)
	if contactName == "" {
		contactName = "customer"
	}

	treatment := normalizeCustomerSettlementTreatment(payment.SettlementTreatment)
	journalLines := []JournalLineRequest{
		{
			AccountID:    uint64(cashBank.AccountID),
			DebitAmount:  decimal.NewFromFloat(bankBookAmount),
			CreditAmount: decimal.Zero,
			Description:  fmt.Sprintf("Payment received from %s - %s", contactName, payment.Code),
		},
	}

	if treatment == models.PaymentSettlementRounding {
		if !strings.EqualFold(strings.TrimSpace(payment.Currency), "IDR") {
			return nil, "", fmt.Errorf("rounding settlement is only available for IDR receipts")
		}
		if receivableSettlementBookAmount <= customerSettlementTolerance {
			return nil, "", fmt.Errorf("rounding settlement requires an invoice balance")
		}

		arAccountID, err := eps.resolveReceivableAccountID(tx)
		if err != nil {
			return nil, "", fmt.Errorf("failed to resolve accounts receivable: %w", err)
		}
		journalLines = append(journalLines, JournalLineRequest{
			AccountID:    arAccountID,
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.NewFromFloat(receivableSettlementBookAmount),
			Description:  fmt.Sprintf("AR reduction including approved rounding - %s", payment.Code),
		})

		roundingDifference := bankBookAmount - receivableSettlementBookAmount
		if math.Abs(roundingDifference) <= customerSettlementTolerance {
			return nil, "", fmt.Errorf("rounding settlement is not needed because the receipt equals the invoice")
		}
		roundingAccountID, err := eps.resolveRoundingAccountID(tx, roundingDifference > 0)
		if err != nil {
			return nil, "", err
		}
		if roundingDifference > 0 {
			journalLines = append(journalLines, JournalLineRequest{
				AccountID:    roundingAccountID,
				DebitAmount:  decimal.Zero,
				CreditAmount: decimal.NewFromFloat(roundingDifference),
				Description:  fmt.Sprintf("Approved receipt rounding income - %s", payment.Code),
			})
		} else {
			journalLines = append(journalLines, JournalLineRequest{
				AccountID:    roundingAccountID,
				DebitAmount:  decimal.NewFromFloat(-roundingDifference),
				CreditAmount: decimal.Zero,
				Description:  fmt.Sprintf("Approved receipt rounding expense - %s", payment.Code),
			})
		}

		return journalLines, fmt.Sprintf("Customer payment rounding approved %s - %s", payment.Code, contactName), nil
	}

	if receivableSettlementBookAmount > customerSettlementTolerance {
		arAccountID, err := eps.resolveReceivableAccountID(tx)
		if err != nil {
			return nil, "", fmt.Errorf("failed to resolve accounts receivable: %w", err)
		}
		journalLines = append(journalLines, JournalLineRequest{
			AccountID:    arAccountID,
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.NewFromFloat(receivableSettlementBookAmount),
			Description:  fmt.Sprintf("AR reduction - %s", payment.Code),
		})
	}

	advanceAmount := payment.Amount - allocatedAmount
	if advanceAmount > customerSettlementTolerance {
		// A customer receipt without a matching invoice is also an advance. This
		// covers legacy drafts created before settlement fields existed.
		if treatment == models.PaymentSettlementOpenBalance {
			treatment = models.PaymentSettlementCustomerAdvance
			payment.SettlementTreatment = treatment
			payment.SettlementDifference = advanceAmount
		}
		if treatment != models.PaymentSettlementCustomerAdvance {
			return nil, "", fmt.Errorf("unallocated customer receipt must be classified as customer advance")
		}

		advanceBookAmount := bankBookAmount - customerSettlementBookAmount(payment, allocatedAmount)
		if advanceBookAmount <= customerSettlementTolerance {
			return nil, "", fmt.Errorf("customer advance book amount must be positive")
		}
		advanceAccountID, err := eps.resolveCustomerAdvanceAccountID(tx)
		if err != nil {
			return nil, "", err
		}
		journalLines = append(journalLines, JournalLineRequest{
			AccountID:    advanceAccountID,
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.NewFromFloat(advanceBookAmount),
			Description:  fmt.Sprintf("Customer advance - %s", payment.Code),
		})
	} else if treatment == models.PaymentSettlementCustomerAdvance {
		return nil, "", fmt.Errorf("customer advance treatment requires an excess receipt")
	}

	allocatedBankBookAmount := customerSettlementBookAmount(payment, allocatedAmount)
	if receivableSettlementBookAmount > customerSettlementTolerance {
		fxLine, err := eps.buildRealizedFXJournalLine(
			tx,
			allocatedBankBookAmount-receivableSettlementBookAmount,
			false,
			payment.Code,
		)
		if err != nil {
			return nil, "", err
		}
		if fxLine != nil {
			journalLines = append(journalLines, *fxLine)
		}
	}

	return journalLines, fmt.Sprintf("Customer payment %s - %s", payment.Code, contactName), nil
}

// buildStoredCustomerSettlementPlan derives the actual settlement from saved
// allocations. This prevents a draft that was edited during Accounting review
// from posting an old amount or classifying an unallocated receipt as AR.
func (eps *EnhancedPaymentServiceWithJournal) buildStoredCustomerSettlementPlan(
	tx *gorm.DB,
	payment *models.Payment,
) (*customerSettlementPlan, float64, error) {
	allocatedAmount, receivableBookAmount, primaryInvoiceID, allocationCount, err := eps.loadStoredReceivableSettlement(tx, payment)
	if err != nil {
		return nil, 0, err
	}

	plan := &customerSettlementPlan{
		CustomerPaymentSettlementSummary: CustomerPaymentSettlementSummary{
			Treatment:               normalizeCustomerSettlementTreatment(payment.SettlementTreatment),
			InvoiceSettlementAmount: allocatedAmount,
			Difference:              payment.Amount - allocatedAmount,
			RoundingThreshold:       payment.RoundingThreshold,
		},
		primaryInvoiceID: primaryInvoiceID,
		allocationCount:  allocationCount,
	}
	if plan.Treatment == "" {
		plan.Treatment = models.PaymentSettlementOpenBalance
	}
	if plan.Treatment != models.PaymentSettlementOpenBalance &&
		plan.Treatment != models.PaymentSettlementCustomerAdvance &&
		plan.Treatment != models.PaymentSettlementRounding {
		return nil, 0, fmt.Errorf("invalid customer settlement treatment: %s", payment.SettlementTreatment)
	}

	if plan.Treatment == models.PaymentSettlementRounding {
		return eps.validateStoredRoundingSettlement(tx, payment, plan, receivableBookAmount)
	}

	if plan.Difference > customerSettlementTolerance {
		if plan.Treatment == models.PaymentSettlementOpenBalance {
			plan.Treatment = models.PaymentSettlementCustomerAdvance
		}
		if plan.Treatment != models.PaymentSettlementCustomerAdvance {
			return nil, 0, fmt.Errorf("invalid customer settlement treatment: %s", plan.Treatment)
		}
		plan.CustomerAdvanceAmount = plan.Difference
	} else if plan.Treatment == models.PaymentSettlementCustomerAdvance {
		return nil, 0, fmt.Errorf("customer advance treatment requires an excess receipt")
	}

	payment.SettlementTreatment = plan.Treatment
	payment.SettlementDifference = plan.Difference
	payment.RoundingThreshold = plan.RoundingThreshold
	return plan, receivableBookAmount, nil
}

// validateStoredRoundingSettlement validates the still-open invoice at final
// approval. Rounding is deliberately limited to one IDR invoice, with the
// current threshold acting as a hard cap even if settings changed after the
// request was submitted.
func (eps *EnhancedPaymentServiceWithJournal) validateStoredRoundingSettlement(
	tx *gorm.DB,
	payment *models.Payment,
	plan *customerSettlementPlan,
	_ float64,
) (*customerSettlementPlan, float64, error) {
	if !strings.EqualFold(strings.TrimSpace(payment.Currency), "IDR") {
		return nil, 0, fmt.Errorf("rounding settlement is only available for IDR receipts")
	}
	if plan.allocationCount != 1 || plan.primaryInvoiceID == nil {
		return nil, 0, fmt.Errorf("rounding settlement requires exactly one invoice allocation")
	}
	if strings.TrimSpace(payment.SettlementReason) == "" {
		return nil, 0, fmt.Errorf("reason is required for rounding settlement")
	}

	var sale models.Sale
	if err := tx.First(&sale, *plan.primaryInvoiceID).Error; err != nil {
		return nil, 0, fmt.Errorf("load invoice for rounding settlement: %w", err)
	}
	if sale.CustomerID != payment.ContactID {
		return nil, 0, fmt.Errorf("rounding invoice does not belong to payment customer")
	}
	if currency := strings.ToUpper(strings.TrimSpace(sale.Currency)); currency != "" && currency != "IDR" {
		return nil, 0, fmt.Errorf("rounding settlement is only available for IDR invoices")
	}
	if err := eps.statusValidator.ValidatePaymentAllocation(sale.Status, sale.ID); err != nil {
		return nil, 0, err
	}

	expectedAllocatedAmount := math.Min(payment.Amount, sale.OutstandingAmount)
	if math.Abs(plan.InvoiceSettlementAmount-expectedAllocatedAmount) > customerSettlementTolerance {
		return nil, 0, fmt.Errorf("rounding allocation must match the actual receipt up to the invoice balance")
	}

	currentThreshold, err := eps.customerPaymentRoundingThreshold(tx)
	if err != nil {
		return nil, 0, err
	}
	effectiveThreshold := currentThreshold
	if payment.RoundingThreshold > 0 && (effectiveThreshold <= 0 || payment.RoundingThreshold < effectiveThreshold) {
		effectiveThreshold = payment.RoundingThreshold
	}
	if effectiveThreshold <= customerSettlementTolerance {
		return nil, 0, fmt.Errorf("rounding settlement is disabled by configuration")
	}

	roundingDifference := payment.Amount - sale.OutstandingAmount
	if math.Abs(roundingDifference) <= customerSettlementTolerance {
		return nil, 0, fmt.Errorf("rounding settlement is not needed because the receipt equals the invoice")
	}
	if math.Abs(roundingDifference)-effectiveThreshold > customerSettlementTolerance {
		return nil, 0, fmt.Errorf("rounding difference %.2f exceeds configured threshold %.2f", math.Abs(roundingDifference), effectiveThreshold)
	}

	plan.InvoiceSettlementAmount = sale.OutstandingAmount
	plan.Difference = roundingDifference
	plan.RoundingAmount = math.Abs(roundingDifference)
	plan.RoundingThreshold = effectiveThreshold
	plan.RequiresApproval = true
	if roundingDifference > 0 {
		plan.RoundingDirection = "OVERPAYMENT"
	} else {
		plan.RoundingDirection = "SHORTFALL"
	}
	payment.SettlementDifference = roundingDifference
	payment.RoundingThreshold = effectiveThreshold
	return plan, sale.OutstandingAmount, nil
}

func (eps *EnhancedPaymentServiceWithJournal) ensureSalePaymentRecord(
	tx *gorm.DB,
	payment *models.Payment,
	saleID uint,
	amount float64,
) error {
	if payment == nil || saleID == 0 || amount <= customerSettlementTolerance {
		return nil
	}

	reference := fmt.Sprintf("Payment ID: %d", payment.ID)
	var existing models.SalePayment
	err := tx.Where("sale_id = ? AND reference = ? AND deleted_at IS NULL", saleID, reference).First(&existing).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("check sale payment record: %w", err)
	}

	bookAmount := customerSettlementBookAmount(payment, amount)
	if bookAmount <= customerSettlementTolerance {
		bookAmount = amount
	}
	cashBankID := payment.CashBankID
	salePayment := &models.SalePayment{
		SaleID:                saleID,
		Amount:                amount,
		NativeAmount:          payment.NativeAmount * (amount / payment.Amount),
		Currency:              payment.Currency,
		ExchangeRate:          payment.ExchangeRate,
		BookAmountIDR:         bookAmount,
		PaymentDate:           payment.Date,
		PaymentMethod:         payment.Method,
		Reference:             reference,
		Notes:                 fmt.Sprintf("Created from SSOT payment %s", payment.Code),
		CashBankID:            cashBankID,
		CashBankCode:          payment.CashBankCode,
		CashBankName:          payment.CashBankName,
		CashBankCurrency:      payment.CashBankCurrency,
		CashBankGLAccountID:   payment.CashBankGLAccountID,
		CashBankGLAccountCode: payment.CashBankGLAccountCode,
		CashBankGLAccountName: payment.CashBankGLAccountName,
		UserID:                payment.UserID,
		Status:                models.SalePaymentStatusCompleted,
	}
	if salePayment.Currency == "" {
		salePayment.Currency = "IDR"
	}
	if salePayment.ExchangeRate <= 0 {
		salePayment.ExchangeRate = 1
	}
	if err := tx.Create(salePayment).Error; err != nil {
		return fmt.Errorf("create sale payment record: %w", err)
	}
	return nil
}

// cancelCustomerSettlementForReversal keeps the customer-advance subledger and
// sales payment history consistent with the reversal journal. An advance that
// has already been applied must be unwound through its own controlled process
// before the originating receipt can be reversed.
func (eps *EnhancedPaymentServiceWithJournal) cancelCustomerSettlementForReversal(
	tx *gorm.DB,
	payment *models.Payment,
	reason string,
) error {
	if tx == nil || payment == nil || !strings.EqualFold(payment.Contact.Type, "CUSTOMER") {
		return nil
	}

	var advance models.CustomerAdvance
	err := tx.Where("payment_id = ? AND deleted_at IS NULL", payment.ID).First(&advance).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("load customer advance for reversal: %w", err)
	}
	if err == nil {
		if advance.Status == models.CustomerAdvanceStatusApplied ||
			math.Abs(advance.OriginalAmount-advance.RemainingAmount) > customerSettlementTolerance {
			return fmt.Errorf("customer advance from payment %s has been applied and must be unwound before reversal", payment.Code)
		}
		advance.Status = models.CustomerAdvanceStatusCancelled
		advance.RemainingAmount = 0
		advance.Notes = strings.TrimSpace(fmt.Sprintf("%s [DIBATALKAN: %s]", advance.Notes, reason))
		if err := tx.Save(&advance).Error; err != nil {
			return fmt.Errorf("cancel customer advance for reversal: %w", err)
		}
	}

	// SSOT receipts create a SalePayment row solely for the invoice amount.
	// Keep it in the audit trail but make it non-effective after a reversal.
	reference := fmt.Sprintf("Payment ID: %d", payment.ID)
	var salePayments []models.SalePayment
	if err := tx.Where("reference = ? AND deleted_at IS NULL", reference).Find(&salePayments).Error; err != nil {
		return fmt.Errorf("load sale payment records for reversal: %w", err)
	}
	for i := range salePayments {
		salePayments[i].Status = models.SalePaymentStatusCancelled
		salePayments[i].Notes = strings.TrimSpace(fmt.Sprintf("%s [DIBATALKAN: %s]", salePayments[i].Notes, reason))
		if err := tx.Save(&salePayments[i]).Error; err != nil {
			return fmt.Errorf("cancel sale payment record for reversal: %w", err)
		}
	}

	if payment.SettlementTreatment != models.PaymentSettlementRounding ||
		payment.SettlementDifference >= -customerSettlementTolerance {
		return nil
	}

	// A shortfall that was approved as rounding added an extra settlement amount
	// to the invoice after its actual receipt allocation. Reverse that separate
	// effect too; otherwise the invoice would reopen only partially.
	var allocation models.PaymentAllocation
	if err := tx.Where("payment_id = ? AND invoice_id IS NOT NULL", payment.ID).
		Order("id ASC").First(&allocation).Error; err != nil {
		return fmt.Errorf("load rounding invoice allocation for reversal: %w", err)
	}
	if allocation.InvoiceID == nil || *allocation.InvoiceID == 0 {
		return fmt.Errorf("rounding payment %s has no invoice allocation", payment.Code)
	}

	var sale models.Sale
	if err := tx.First(&sale, *allocation.InvoiceID).Error; err != nil {
		return fmt.Errorf("load rounding invoice for reversal: %w", err)
	}
	roundingAmount := math.Abs(payment.SettlementDifference)
	sale.PaidAmount -= roundingAmount
	if sale.PaidAmount < 0 {
		sale.PaidAmount = 0
	}
	sale.OutstandingAmount += roundingAmount
	if sale.OutstandingAmount > customerSettlementTolerance && sale.Status == models.SaleStatusPaid {
		sale.Status = models.SaleStatusInvoiced
	}
	if err := tx.Save(&sale).Error; err != nil {
		return fmt.Errorf("reverse rounding settlement effect: %w", err)
	}
	if err := syncSalePaymentScheduleProgress(tx, &sale); err != nil {
		return fmt.Errorf("sync sale payment schedules after rounding reversal: %w", err)
	}

	return nil
}
