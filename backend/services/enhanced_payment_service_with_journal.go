package services

import (
	"fmt"
	"log"
	"strings"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// EnhancedPaymentServiceWithJournal extends payment service with SSOT journal integration
type EnhancedPaymentServiceWithJournal struct {
	db              *gorm.DB
	paymentRepo     repositories.PaymentRepository
	contactRepo     repositories.ContactRepository
	cashBankRepo    *repositories.CashBankRepository
	salesRepo       *repositories.SalesRepository
	purchaseRepo    *repositories.PurchaseRepository
	journalFactory  *PaymentJournalFactory
	journalService  *UnifiedJournalService
	statusValidator *StatusValidationHelper // NEW: Konsistensi dengan SalesJournalServiceV2
}

// ExpensePaymentRequest represents a direct expense payment mapped from COA.
// Contact is optional — for bulk payments like salaries, no specific contact is needed.
type ExpensePaymentRequest struct {
	ContactID         uint      `json:"contact_id"`
	ExpenseAccountID  uint      `json:"expense_account_id" binding:"required"`
	CashBankID        uint      `json:"cash_bank_id" binding:"required"`
	Date              time.Time `json:"date" binding:"required"`
	Amount            float64   `json:"amount" binding:"required,min=0.01"`
	Method            string    `json:"method" binding:"required"`
	Reference         string    `json:"reference"`
	Notes             string    `json:"notes"`
	Description       string    `json:"description"`
	AutoCreateJournal bool      `json:"auto_create_journal"`
	UserID            uint      `json:"user_id"`

	// SaveAsDraft = true means save payment with status DRAFT without creating journal.
	// Used when submitted by employee role; finance must approve later.
	SaveAsDraft bool `json:"save_as_draft"`

	// PPh withholding tax fields (optional)
	PPHType   string  `json:"pph_type"`   // PPH_21, PPH_23, PPH_4_2, PPH_22, or empty
	PPHRate   float64 `json:"pph_rate"`   // Tax rate percentage (e.g. 2 for 2%)
	PPHAmount float64 `json:"pph_amount"` // Calculated tax amount
}

// ListPayments returns paginated payments with filters (SSOT-compatible)
func (eps *EnhancedPaymentServiceWithJournal) ListPayments(filter repositories.PaymentFilter) (*repositories.PaymentResult, error) {
	return eps.paymentRepo.FindWithFilter(filter)
}

// NewEnhancedPaymentServiceWithJournal creates a new instance with journal integration
func NewEnhancedPaymentServiceWithJournal(
	db *gorm.DB,
	paymentRepo repositories.PaymentRepository,
	contactRepo repositories.ContactRepository,
	cashBankRepo *repositories.CashBankRepository,
	salesRepo *repositories.SalesRepository,
	purchaseRepo *repositories.PurchaseRepository,
	journalService *UnifiedJournalService,
) *EnhancedPaymentServiceWithJournal {
	journalFactory := NewPaymentJournalFactory(db)

	return &EnhancedPaymentServiceWithJournal{
		db:              db,
		paymentRepo:     paymentRepo,
		contactRepo:     contactRepo,
		cashBankRepo:    cashBankRepo,
		salesRepo:       salesRepo,
		purchaseRepo:    purchaseRepo,
		journalFactory:  journalFactory,
		journalService:  journalService,
		statusValidator: NewStatusValidationHelper(), // Initialize status validator
	}
}

// PaymentWithJournalRequest extends payment request with journal integration options
type PaymentWithJournalRequest struct {
	ContactID  uint      `json:"contact_id" binding:"required"`
	CashBankID uint      `json:"cash_bank_id" binding:"required"`
	Date       time.Time `json:"date" binding:"required"`
	Amount     float64   `json:"amount" binding:"required,min=0.01"`
	Method     string    `json:"method" binding:"required"`
	Reference  string    `json:"reference"`
	Notes      string    `json:"notes"`

	// Journal integration options
	AutoCreateJournal bool `json:"auto_create_journal" default:"true"`
	PreviewJournal    bool `json:"preview_journal" default:"false"`
	SaveAsDraft       bool `json:"save_as_draft"` // DRAFT mode for employee submissions

	// Allocation options (single target - legacy)
	TargetInvoiceID *uint `json:"target_invoice_id,omitempty"`
	TargetBillID    *uint `json:"target_bill_id,omitempty"`

	// Allocation arrays (multiple targets - preferred)
	InvoiceAllocations []AllocationItem `json:"invoice_allocations,omitempty"`
	BillAllocations    []AllocationItem `json:"bill_allocations,omitempty"`

	// User context
	UserID uint `json:"user_id"`
}

// AllocationItem represents a single allocation to an invoice or bill
type AllocationItem struct {
	InvoiceID *uint   `json:"invoice_id,omitempty"`
	BillID    *uint   `json:"bill_id,omitempty"`
	Amount    float64 `json:"amount" binding:"required,min=0.01"`
}

// PaymentWithJournalResponse extends payment response with journal information
type PaymentWithJournalResponse struct {
	Payment       *models.Payment            `json:"payment"`
	JournalResult *PaymentJournalResult      `json:"journal_result,omitempty"`
	Contact       *models.Contact            `json:"contact"`
	CashBank      *models.CashBank           `json:"cash_bank"`
	Allocations   []models.PaymentAllocation `json:"allocations,omitempty"`
	Summary       *PaymentProcessingSummary  `json:"summary"`
	Success       bool                       `json:"success"`
	Message       string                     `json:"message"`
	Warnings      []string                   `json:"warnings,omitempty"`
}

// PaymentProcessingSummary provides comprehensive processing information
type PaymentProcessingSummary struct {
	TotalAmount            decimal.Decimal `json:"total_amount"`
	ProcessingTime         string          `json:"processing_time"`
	JournalEntryCreated    bool            `json:"journal_entry_created"`
	AccountBalancesUpdated bool            `json:"account_balances_updated"`
	AllocationsCreated     int             `json:"allocations_created"`
	TransactionID          string          `json:"transaction_id"`
}

// CreateExpensePayment creates a direct expense payment (Expense/Liability from COA) with SSOT journal integration.
// It records a row in payments table (PaymentTypeExpense) AND posts a journal entry:
//
//	Debit:  expense account (from COA)
//	Credit: cash/bank GL account linked to selected CashBank.
//
// It also updates CashBank balance & history via applyCashBankMovement.
// If req.SaveAsDraft is true, the payment is saved with status DRAFT and no journal is created.
// Finance role must call ApproveExpensePayment to post the journal and set status COMPLETED.
func (eps *EnhancedPaymentServiceWithJournal) CreateExpensePayment(req *ExpensePaymentRequest) (*models.Payment, error) {
	// Basic validation
	if req.Amount <= 0 {
		return nil, fmt.Errorf("payment amount must be positive")
	}
	if req.CashBankID == 0 {
		return nil, fmt.Errorf("cash/bank account ID is required")
	}
	if req.ExpenseAccountID == 0 {
		return nil, fmt.Errorf("expense account ID is required")
	}
	if req.UserID == 0 {
		return nil, fmt.Errorf("user ID is required")
	}
	if req.Date.IsZero() {
		req.Date = time.Now()
	}
	if req.Method == "" {
		// Default to bank transfer for cash out if method not specified
		req.Method = models.PaymentMethodBankTransfer
	}
	if req.Description == "" {
		req.Description = "Expense Payment"
	}

	// DRAFT MODE: save payment without creating journal — for employee role submissions
	if req.SaveAsDraft {
		return eps.createExpensePaymentDraft(req)
	}

	var payment *models.Payment

	// Execute everything in a single transaction
	err := eps.db.Transaction(func(tx *gorm.DB) error {
		// Load cash/bank account (must have linked GL account)
		cashBank, err := eps.cashBankRepo.FindByID(req.CashBankID)
		if err != nil {
			return fmt.Errorf("cash/bank account not found: %w", err)
		}
		if cashBank.AccountID == 0 {
			return fmt.Errorf("cash/bank account %s has no linked GL account", cashBank.Name)
		}

		// Load expense account from COA
		var expenseAccount models.Account
		if err := tx.First(&expenseAccount, req.ExpenseAccountID).Error; err != nil {
			return fmt.Errorf("expense account not found: %w", err)
		}

		// Load contact if provided (optional for bulk payments like salaries)
		contactName := "Umum"
		if req.ContactID > 0 {
			contact, err := eps.contactRepo.GetByID(req.ContactID)
			if err != nil {
				return fmt.Errorf("contact not found: %w", err)
			}
			contactName = contact.Name
		}

		// Build payment model (cash out, treated similar to PAY prefix)
		payment = &models.Payment{
			ContactID:   req.ContactID,
			UserID:      req.UserID,
			Date:        req.Date,
			Amount:      req.Amount,
			Method:      req.Method,
			Reference:   req.Reference,
			Status:      models.PaymentStatusCompleted,
			PaymentType: models.PaymentTypeExpense,
			Notes:       fmt.Sprintf("%s [Contact: %s]", req.Notes, contactName),
			PPHType:     req.PPHType,
			PPHRate:     req.PPHRate,
			PPHAmount:   req.PPHAmount,
		}

		// Generate payment code using vendor-style prefix (PAY-YYYY/MM-XXXX)
		code, err := eps.generatePaymentCode(tx, req.Method, "VENDOR")
		if err != nil {
			return fmt.Errorf("failed to generate payment code: %w", err)
		}
		payment.Code = code

		// Persist payment
		if err := tx.Create(payment).Error; err != nil {
			return fmt.Errorf("failed to create expense payment: %w", err)
		}

		// Prepare journal entry
		var journalLines []JournalLineRequest
		grossAmount := decimal.NewFromFloat(req.Amount)

		if req.PPHAmount > 0 && req.PPHType != "" {
			// === 3-LINE JOURNAL WITH PPh WITHHOLDING ===
			// Debit: Beban (gross)  = Full expense amount
			// Credit: Bank (net)    = Amount actually paid out
			// Credit: Hutang PPh    = Tax withheld (to be remitted to government)
			pphAmount := decimal.NewFromFloat(req.PPHAmount)
			netAmount := grossAmount.Sub(pphAmount) // Amount actually leaving bank

			// Resolve PPh liability account based on type
			pphAccountID, err := eps.resolvePPHAccountID(tx, req.PPHType)
			if err != nil {
				return fmt.Errorf("failed to resolve PPh account: %w", err)
			}

			journalLines = []JournalLineRequest{
				{
					AccountID:    uint64(expenseAccount.ID),
					DebitAmount:  grossAmount,
					CreditAmount: decimal.Zero,
					Description:  fmt.Sprintf("Beban %s - %s (bruto)", expenseAccount.Name, payment.Code),
				},
				{
					AccountID:    uint64(cashBank.AccountID),
					DebitAmount:  decimal.Zero,
					CreditAmount: netAmount,
					Description:  fmt.Sprintf("Pembayaran neto via %s - %s", cashBank.Name, payment.Code),
				},
				{
					AccountID:    pphAccountID,
					DebitAmount:  decimal.Zero,
					CreditAmount: pphAmount,
					Description:  fmt.Sprintf("Hutang %s dipotong - %s (%.1f%%)", req.PPHType, payment.Code, req.PPHRate),
				},
			}

			log.Printf("📋 Expense with PPh: Gross=%.2f, PPh(%s %.1f%%)=%.2f, Net=%.2f",
				req.Amount, req.PPHType, req.PPHRate, req.PPHAmount, netAmount.InexactFloat64())

			// IMPORTANT: CashBank movement should use NET amount (what actually leaves the bank)
			req.Amount = netAmount.InexactFloat64()
		} else {
			// === STANDARD 2-LINE JOURNAL (no tax) ===
			// Debit: Expense, Credit: Cash/Bank
			journalLines = []JournalLineRequest{
				{
					AccountID:    uint64(expenseAccount.ID),
					DebitAmount:  grossAmount,
					CreditAmount: decimal.Zero,
					Description:  fmt.Sprintf("Expense Payment %s - %s", payment.Code, req.Description),
				},
				{
					AccountID:    uint64(cashBank.AccountID),
					DebitAmount:  decimal.Zero,
					CreditAmount: grossAmount,
					Description:  fmt.Sprintf("Expense Payment %s - Cash/Bank", payment.Code),
				},
			}
		}

		journalRequest := &JournalEntryRequest{
			SourceType:  models.SSOTSourceTypePayment,
			SourceID:    uint64(payment.ID),
			Reference:   payment.Code,
			EntryDate:   payment.Date,
			Description: req.Description,
			Lines:       journalLines,
			AutoPost:    req.AutoCreateJournal,
			CreatedBy:   uint64(req.UserID),
		}

		journalService := NewUnifiedJournalService(tx)
		journalResponse, err := journalService.CreateJournalEntryWithTx(tx, journalRequest)
		if err != nil {
			return fmt.Errorf("failed to create expense journal entry: %w", err)
		}

		// Link payment to journal entry
		journalEntryID := uint(journalResponse.ID)
		payment.JournalEntryID = &journalEntryID
		if err := tx.Save(payment).Error; err != nil {
			return fmt.Errorf("failed to update payment with journal reference: %w", err)
		}

		// Apply cash/bank movement (treat as vendor/outgoing)
		if err := eps.applyCashBankMovement(tx, cashBank, "VENDOR", req.Amount, payment, req.Date); err != nil {
			return fmt.Errorf("failed to update cash/bank balances: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return payment, nil
}

// createExpensePaymentDraft saves an expense payment with status DRAFT without creating a journal.
// Called when employee submits; finance must approve via ApproveExpensePayment.
func (eps *EnhancedPaymentServiceWithJournal) createExpensePaymentDraft(req *ExpensePaymentRequest) (*models.Payment, error) {
	var payment *models.Payment

	err := eps.db.Transaction(func(tx *gorm.DB) error {
		// Validate cash/bank exists
		cashBank, err := eps.cashBankRepo.FindByID(req.CashBankID)
		if err != nil {
			return fmt.Errorf("cash/bank account not found: %w", err)
		}

		// Validate expense account exists
		var expenseAccount models.Account
		if err := tx.First(&expenseAccount, req.ExpenseAccountID).Error; err != nil {
			return fmt.Errorf("expense account not found: %w", err)
		}

		// Load contact name if provided
		contactName := "Umum"
		if req.ContactID > 0 {
			contact, err := eps.contactRepo.GetByID(req.ContactID)
			if err != nil {
				return fmt.Errorf("contact not found: %w", err)
			}
			contactName = contact.Name
		}

		payment = &models.Payment{
			ContactID:   req.ContactID,
			UserID:      req.UserID,
			Date:        req.Date,
			Amount:      req.Amount,
			Method:      req.Method,
			Reference:   fmt.Sprintf("__EXP_ACC:%d,CB:%d__%s", req.ExpenseAccountID, req.CashBankID, req.Reference),
			Status:      models.PaymentStatusDraft,
			PaymentType: models.PaymentTypeExpense,
			Notes:       fmt.Sprintf("%s [Contact: %s] [Beban: %s] [CashBank: %s]", req.Notes, contactName, expenseAccount.Name, cashBank.Name),
			PPHType:     req.PPHType,
			PPHRate:     req.PPHRate,
			PPHAmount:   req.PPHAmount,
		}

		code, err := eps.generatePaymentCode(tx, req.Method, "VENDOR")
		if err != nil {
			return fmt.Errorf("failed to generate payment code: %w", err)
		}
		payment.Code = code

		if err := tx.Create(payment).Error; err != nil {
			return fmt.Errorf("failed to create draft expense payment: %w", err)
		}

		log.Printf("📝 Expense payment saved as DRAFT: ID=%d, Code=%s, Amount=%.2f, ExpenseAcc=%s",
			payment.ID, payment.Code, req.Amount, expenseAccount.Name)
		return nil
	})

	if err != nil {
		return nil, err
	}
	return payment, nil
}

// ApproveExpensePayment approves a DRAFT expense payment: creates the SSOT journal and updates status to COMPLETED.
// Only finance/admin role should call this.
func (eps *EnhancedPaymentServiceWithJournal) ApproveExpensePayment(paymentID uint, approverUserID uint) (*models.Payment, error) {
	var payment models.Payment
	if err := eps.db.First(&payment, paymentID).Error; err != nil {
		return nil, fmt.Errorf("payment not found: %w", err)
	}

	if payment.Status != models.PaymentStatusDraft {
		return nil, fmt.Errorf("payment is not in DRAFT status (current: %s)", payment.Status)
	}
	if payment.PaymentType != models.PaymentTypeExpense {
		return nil, fmt.Errorf("only expense payments can be approved via this endpoint")
	}

	// Parse expense account and cash bank IDs from Notes is not reliable.
	// Instead we need to store them. For now we query by looking at the journal
	// request fields stored in Notes — but the cleanest solution is to add columns.
	// Since we don't want schema changes right now, we look up via SSOT journal preview:
	// we re-build the request from persisted data using a helper.
	//
	// APPROACH: We store expense_account_id and cash_bank_id in a dedicated approach.
	// The payment.Notes field encodes "[Beban: <name>] [CashBank: <name>]" which isn't
	// query-friendly. Instead we save expense_account_id/cash_bank_id in payment.Reference
	// as "EXP_ACC:<id>,CB:<id>" when creating draft. Let's parse it.
	var expenseAccountID, cashBankID uint
	refStr := payment.Reference
	// Try to parse embedded IDs (set by createExpensePaymentDraft below)
	n, _ := fmt.Sscanf(refStr, "__EXP_ACC:%d,CB:%d__", &expenseAccountID, &cashBankID)
	if n != 2 || expenseAccountID == 0 || cashBankID == 0 {
		return nil, fmt.Errorf("cannot find expense account or cash bank info in draft payment (ref: %s); please delete and re-submit", payment.Reference)
	}

	err := eps.db.Transaction(func(tx *gorm.DB) error {
		cashBank, err := eps.cashBankRepo.FindByID(cashBankID)
		if err != nil {
			return fmt.Errorf("cash/bank account not found: %w", err)
		}
		if cashBank.AccountID == 0 {
			return fmt.Errorf("cash/bank account %s has no linked GL account", cashBank.Name)
		}

		var expenseAccount models.Account
		if err := tx.First(&expenseAccount, expenseAccountID).Error; err != nil {
			return fmt.Errorf("expense account not found: %w", err)
		}

		grossAmount := decimal.NewFromFloat(payment.Amount)
		var journalLines []JournalLineRequest

		if payment.PPHAmount > 0 && payment.PPHType != "" {
			pphAmount := decimal.NewFromFloat(payment.PPHAmount)
			netAmount := grossAmount.Sub(pphAmount)

			pphAccountID, err := eps.resolvePPHAccountID(tx, payment.PPHType)
			if err != nil {
				return fmt.Errorf("failed to resolve PPh account: %w", err)
			}

			journalLines = []JournalLineRequest{
				{
					AccountID:    uint64(expenseAccount.ID),
					DebitAmount:  grossAmount,
					CreditAmount: decimal.Zero,
					Description:  fmt.Sprintf("Beban %s - %s (bruto)", expenseAccount.Name, payment.Code),
				},
				{
					AccountID:    uint64(cashBank.AccountID),
					DebitAmount:  decimal.Zero,
					CreditAmount: netAmount,
					Description:  fmt.Sprintf("Pembayaran neto via %s - %s", cashBank.Name, payment.Code),
				},
				{
					AccountID:    pphAccountID,
					DebitAmount:  decimal.Zero,
					CreditAmount: pphAmount,
					Description:  fmt.Sprintf("Hutang %s dipotong - %s (%.1f%%)", payment.PPHType, payment.Code, payment.PPHRate),
				},
			}

			// CashBank moves net amount
			payment.Amount = netAmount.InexactFloat64()
		} else {
			journalLines = []JournalLineRequest{
				{
					AccountID:    uint64(expenseAccount.ID),
					DebitAmount:  grossAmount,
					CreditAmount: decimal.Zero,
					Description:  fmt.Sprintf("Expense Payment %s - %s", payment.Code, expenseAccount.Name),
				},
				{
					AccountID:    uint64(cashBank.AccountID),
					DebitAmount:  decimal.Zero,
					CreditAmount: grossAmount,
					Description:  fmt.Sprintf("Expense Payment %s - Cash/Bank", payment.Code),
				},
			}
		}

		journalRequest := &JournalEntryRequest{
			SourceType:  models.SSOTSourceTypePayment,
			SourceID:    uint64(payment.ID),
			Reference:   payment.Code,
			EntryDate:   payment.Date,
			Description: fmt.Sprintf("Expense Payment approved - %s", payment.Code),
			Lines:       journalLines,
			AutoPost:    true,
			CreatedBy:   uint64(approverUserID),
		}

		journalService := NewUnifiedJournalService(tx)
		journalResponse, err := journalService.CreateJournalEntryWithTx(tx, journalRequest)
		if err != nil {
			return fmt.Errorf("failed to create expense journal entry: %w", err)
		}

		journalEntryID := uint(journalResponse.ID)
		payment.JournalEntryID = &journalEntryID
		payment.Status = models.PaymentStatusCompleted

		if err := tx.Save(&payment).Error; err != nil {
			return fmt.Errorf("failed to update payment after approval: %w", err)
		}

		if err := eps.applyCashBankMovement(tx, cashBank, "VENDOR", payment.Amount, &payment, payment.Date); err != nil {
			return fmt.Errorf("failed to update cash/bank balances: %w", err)
		}

		log.Printf("✅ Expense payment APPROVED: ID=%d, Code=%s, JournalID=%d, ApprovedBy=%d",
			payment.ID, payment.Code, journalEntryID, approverUserID)
		return nil
	})

	if err != nil {
		return nil, err
	}
	return &payment, nil
}

// ApproveStandardPayment approves a DRAFT receivable/payable payment: creates the SSOT journal and updates status to COMPLETED.
// Only finance/admin role should call this.
func (eps *EnhancedPaymentServiceWithJournal) ApproveStandardPayment(paymentID uint, approverUserID uint) (*PaymentWithJournalResponse, error) {
	var payment models.Payment
	if err := eps.db.Preload("Contact").Preload("User").First(&payment, paymentID).Error; err != nil {
		return nil, fmt.Errorf("payment not found: %w", err)
	}

	if payment.Status != models.PaymentStatusDraft {
		return nil, fmt.Errorf("payment is not in DRAFT status (current: %s)", payment.Status)
	}
	if payment.PaymentType == models.PaymentTypeExpense || strings.HasPrefix(payment.PaymentType, "TAX_PPN") {
		return nil, fmt.Errorf("only receivable/payable payments can be approved via this endpoint")
	}

	var cashBankID uint
	refStr := payment.Reference
	n, _ := fmt.Sscanf(refStr, "__CB:%d__", &cashBankID)
	if n != 1 || cashBankID == 0 {
		return nil, fmt.Errorf("cannot find cash bank info in draft payment (ref: %s); please delete and re-submit", payment.Reference)
	}

	var journalResult *PaymentJournalResult
	response := &PaymentWithJournalResponse{
		Success: false,
	}

	err := eps.db.Transaction(func(tx *gorm.DB) error {
		cashBank, err := eps.cashBankRepo.FindByID(cashBankID)
		if err != nil {
			return fmt.Errorf("cash/bank account not found: %w", err)
		}
		if cashBank.AccountID == 0 {
			return fmt.Errorf("cash/bank account %s has no linked GL account", cashBank.Name)
		}

		var journalLines []JournalLineRequest
		var journalDescription string

		if payment.Contact.Type == "CUSTOMER" {
			// Receivable Payment Journal: Debit Cash/Bank, Credit Piutang Usaha
			arAccountID, arErr := eps.resolveReceivableAccountID(tx)
			if arErr != nil {
				return fmt.Errorf("failed to resolve accounts receivable: %w", arErr)
			}
			journalLines = []JournalLineRequest{
				{
					AccountID:    uint64(cashBank.AccountID),
					DebitAmount:  decimal.NewFromFloat(payment.Amount),
					CreditAmount: decimal.Zero,
					Description:  fmt.Sprintf("Payment received from %s - %s", payment.Contact.Name, payment.Code),
				},
				{
					AccountID:    arAccountID,
					DebitAmount:  decimal.Zero,
					CreditAmount: decimal.NewFromFloat(payment.Amount),
					Description:  fmt.Sprintf("AR reduction - %s", payment.Code),
				},
			}
			journalDescription = fmt.Sprintf("Customer Payment approved %s - %s", payment.Code, payment.Contact.Name)

		} else if payment.Contact.Type == "VENDOR" {
			// Payable Payment Journal: Debit Utang Usaha, Credit Cash/Bank
			apAccountID, apErr := eps.resolvePayableAccountID(tx)
			if apErr != nil {
				return fmt.Errorf("failed to resolve accounts payable: %w", apErr)
			}
			journalLines = []JournalLineRequest{
				{
					AccountID:    apAccountID,
					DebitAmount:  decimal.NewFromFloat(payment.Amount),
					CreditAmount: decimal.Zero,
					Description:  fmt.Sprintf("AP reduction - %s", payment.Code),
				},
				{
					AccountID:    uint64(cashBank.AccountID),
					DebitAmount:  decimal.Zero,
					CreditAmount: decimal.NewFromFloat(payment.Amount),
					Description:  fmt.Sprintf("Payment made to %s - %s", payment.Contact.Name, payment.Code),
				},
			}
			journalDescription = fmt.Sprintf("Vendor Payment approved %s - %s", payment.Code, payment.Contact.Name)

		} else {
			return fmt.Errorf("unsupported contact type for journal creation: %s", payment.Contact.Type)
		}

		journalRequest := &JournalEntryRequest{
			SourceType:  models.SSOTSourceTypePayment,
			SourceID:    uint64(payment.ID),
			Reference:   payment.Code,
			EntryDate:   payment.Date,
			Description: journalDescription,
			Lines:       journalLines,
			AutoPost:    true,
			CreatedBy:   uint64(approverUserID),
		}

		journalService := NewUnifiedJournalService(tx)
		journalEntry, err := journalService.CreateJournalEntryWithTx(tx, journalRequest)
		if err != nil {
			return fmt.Errorf("failed to create payment journal entry: %w", err)
		}

		journalEntryID := uint(journalEntry.ID)
		payment.JournalEntryID = &journalEntryID
		payment.Status = models.PaymentStatusCompleted

		if err := tx.Save(&payment).Error; err != nil {
			return fmt.Errorf("failed to finalize payment approval: %w", err)
		}

		// Reflect movement in Cash & Bank module
		if err := eps.applyCashBankMovement(tx, cashBank, payment.Contact.Type, payment.Amount, &payment, payment.Date); err != nil {
			return fmt.Errorf("failed to update Cash & Bank balances: %w", err)
		}

		journalResult = &PaymentJournalResult{
			Success:      true,
			JournalEntry: journalEntry,
			Message:      fmt.Sprintf("Journal entry %s created and posted", journalEntry.EntryNumber),
		}

		response.Payment = &payment
		response.JournalResult = journalResult
		response.Contact = &payment.Contact
		response.CashBank = cashBank
		response.Success = true
		response.Message = fmt.Sprintf("Payment %s approved successfully", payment.Code)

		log.Printf("✅ Payment APPROVED: ID=%d, Code=%s, JournalID=%d", payment.ID, payment.Code, journalEntryID)

		return nil
	})

	if err != nil {
		return nil, err
	}
	return response, nil
}

// UpdateDraftPayment allows modifying fields of a payment that is in DRAFT status without creating journals
func (eps *EnhancedPaymentServiceWithJournal) UpdateDraftPayment(paymentID uint, req func(*models.Payment) error) (*models.Payment, error) {
	var payment models.Payment
	if err := eps.db.First(&payment, paymentID).Error; err != nil {
		return nil, fmt.Errorf("payment not found: %w", err)
	}

	if payment.Status != models.PaymentStatusDraft {
		return nil, fmt.Errorf("only payments in DRAFT status can be updated (current status: %s)", payment.Status)
	}

	// Apply updates from request
	if err := req(&payment); err != nil {
		return nil, fmt.Errorf("failed to apply updates: %w", err)
	}

	if err := eps.db.Save(&payment).Error; err != nil {
		return nil, fmt.Errorf("failed to save updated draft payment: %w", err)
	}

	return &payment, nil
}

// CreatePaymentWithJournal creates a payment with automatic journal entry creation
func (eps *EnhancedPaymentServiceWithJournal) CreatePaymentWithJournal(req *PaymentWithJournalRequest) (*PaymentWithJournalResponse, error) {
	startTime := time.Now()

	// 🔍 DEBUG: Log received request
	log.Printf("🔍 DEBUG CreatePaymentWithJournal request:")
	log.Printf("  - ContactID: %d", req.ContactID)
	log.Printf("  - Amount: %.2f", req.Amount)
	log.Printf("  - Method: %s", req.Method)
	log.Printf("  - TargetBillID: %v", req.TargetBillID)
	log.Printf("  - BillAllocations count: %d", len(req.BillAllocations))
	if len(req.BillAllocations) > 0 {
		for i, alloc := range req.BillAllocations {
			log.Printf("    [%d] BillID=%v, Amount=%.2f", i, alloc.BillID, alloc.Amount)
		}
	}
	log.Printf("  - InvoiceAllocations count: %d", len(req.InvoiceAllocations))
	if len(req.InvoiceAllocations) > 0 {
		for i, alloc := range req.InvoiceAllocations {
			log.Printf("    [%d] InvoiceID=%v, Amount=%.2f", i, alloc.InvoiceID, alloc.Amount)
		}
	}

	// Validate request
	if err := eps.validatePaymentRequest(req); err != nil {
		return nil, fmt.Errorf("payment validation failed: %w", err)
	}

	var (
		payment       *models.Payment
		journalResult *PaymentJournalResult
		response      = &PaymentWithJournalResponse{
			Success: false,
		}
	)

	// Execute payment creation in transaction
	err := eps.db.Transaction(func(tx *gorm.DB) error {
		// Step 1: Get and validate contact
		contact, err := eps.contactRepo.GetByID(req.ContactID)
		if err != nil {
			return fmt.Errorf("contact not found: %w", err)
		}

		// Step 2: Get and validate cash/bank account
		cashBank, err := eps.cashBankRepo.FindByID(req.CashBankID)
		if err != nil {
			return fmt.Errorf("cash/bank account not found: %w", err)
		}

		// Step 3: Auto-detect payment method if needed
		if req.Method == "" {
			req.Method = eps.autoDetectPaymentMethod(contact.Type)
		}

		// Step 4: Create payment record
		status := models.PaymentStatusCompleted
		ref := req.Reference
		if req.SaveAsDraft {
			status = models.PaymentStatusDraft
			ref = fmt.Sprintf("__CB:%d__%s", req.CashBankID, req.Reference)
		}

		payment = &models.Payment{
			ContactID: req.ContactID,
			UserID:    req.UserID,
			Date:      req.Date,
			Amount:    req.Amount,
			Method:    req.Method,
			Reference: ref,
			Notes:     req.Notes,
			Status:    status,
		}

		// Generate payment code
		code, err := eps.generatePaymentCode(tx, req.Method, contact.Type)
		if err != nil {
			return fmt.Errorf("failed to generate payment code: %w", err)
		}
		payment.Code = code

		// Create payment in database
		if err := tx.Create(payment).Error; err != nil {
			return fmt.Errorf("failed to create payment: %w", err)
		}

		log.Printf("✅ Payment created: ID=%d, Code=%s", payment.ID, payment.Code)

		// Step 5: Create SSOT journal entry directly (bypass disabled PaymentJournalFactory)
		// Only create journal if AutoCreateJournal is true AND SaveAsDraft is false
		if req.AutoCreateJournal && !req.SaveAsDraft {
			var journalLines []JournalLineRequest
			var journalDescription string

			if contact.Type == "CUSTOMER" {
				// Receivable Payment Journal: Debit Cash/Bank, Credit Piutang Usaha
				arAccountID, arErr := eps.resolveReceivableAccountID(tx)
				if arErr != nil {
					return fmt.Errorf("failed to resolve accounts receivable: %w", arErr)
				}
				journalLines = []JournalLineRequest{
					{
						AccountID:    uint64(cashBank.AccountID),
						DebitAmount:  decimal.NewFromFloat(req.Amount),
						CreditAmount: decimal.Zero,
						Description:  fmt.Sprintf("Payment received from %s - %s", contact.Name, payment.Code),
					},
					{
						AccountID:    arAccountID,
						DebitAmount:  decimal.Zero,
						CreditAmount: decimal.NewFromFloat(req.Amount),
						Description:  fmt.Sprintf("AR reduction - %s", payment.Code),
					},
				}
				journalDescription = fmt.Sprintf("Customer Payment %s - %s", payment.Code, contact.Name)

			} else if contact.Type == "VENDOR" {
				// Payable Payment Journal: Debit Utang Usaha, Credit Cash/Bank
				apAccountID, apErr := eps.resolvePayableAccountID(tx)
				if apErr != nil {
					return fmt.Errorf("failed to resolve accounts payable: %w", apErr)
				}
				journalLines = []JournalLineRequest{
					{
						AccountID:    apAccountID,
						DebitAmount:  decimal.NewFromFloat(req.Amount),
						CreditAmount: decimal.Zero,
						Description:  fmt.Sprintf("AP reduction - %s", payment.Code),
					},
					{
						AccountID:    uint64(cashBank.AccountID),
						DebitAmount:  decimal.Zero,
						CreditAmount: decimal.NewFromFloat(req.Amount),
						Description:  fmt.Sprintf("Payment made to %s - %s", contact.Name, payment.Code),
					},
				}
				journalDescription = fmt.Sprintf("Vendor Payment %s - %s", payment.Code, contact.Name)

			} else {
				return fmt.Errorf("unsupported contact type for journal creation: %s", contact.Type)
			}

			// Create SSOT journal entry using UnifiedJournalService
			journalRequest := &JournalEntryRequest{
				SourceType:  models.SSOTSourceTypePayment,
				SourceID:    uint64(payment.ID),
				Reference:   payment.Code,
				EntryDate:   payment.Date,
				Description: journalDescription,
				Lines:       journalLines,
				AutoPost:    true,
				CreatedBy:   uint64(req.UserID),
			}

			journalService := NewUnifiedJournalService(tx)
			journalEntry, err := journalService.CreateJournalEntryWithTx(tx, journalRequest)
			if err != nil {
				return fmt.Errorf("failed to create payment journal entry: %w", err)
			}

			// Update payment with journal entry ID
			journalEntryID := uint(journalEntry.ID)
			payment.JournalEntryID = &journalEntryID
			if err := tx.Save(payment).Error; err != nil {
				return fmt.Errorf("failed to update payment with journal reference: %w", err)
			}
			log.Printf("✅ SSOT Journal entry created: ID=%d, EntryNumber=%s", journalEntry.ID, journalEntry.EntryNumber)

			// Build result for response
			journalResult = &PaymentJournalResult{
				Success:      true,
				JournalEntry: journalEntry,
				Message:      fmt.Sprintf("Journal entry %s created and posted", journalEntry.EntryNumber),
			}
		}

		// Step 6: Create allocations if specified
		allocations, err := eps.createPaymentAllocations(tx, payment, req, contact.Type)
		if err != nil {
			return fmt.Errorf("failed to create allocations: %w", err)
		}

		// Step 7: Update payment status
		// Already handled during creation if SaveAsDraft is true
		if !req.SaveAsDraft {
			payment.Status = models.PaymentStatusCompleted
			if err := tx.Save(payment).Error; err != nil {
				return fmt.Errorf("failed to finalize payment: %w", err)
			}
		}

		// Step 8: Reflect movement in Cash & Bank module (keeps UI Kas/Bank in sync)
		// IMPORTANT: We only update cash_banks and insert a transaction record if NOT DRAFT.
		// We DO NOT create another journal here to avoid double-posting because the SSOT journal above
		// already debits/credits the GL bank account.
		if !req.SaveAsDraft {
			if err := eps.applyCashBankMovement(tx, cashBank, contact.Type, req.Amount, payment, req.Date); err != nil {
				return fmt.Errorf("failed to update Cash & Bank balances: %w", err)
			}
		}

		// Build response
		response.Payment = payment
		response.JournalResult = journalResult
		response.Contact = contact
		response.CashBank = cashBank
		response.Allocations = allocations
		response.Success = true
		response.Message = fmt.Sprintf("Payment %s created successfully", payment.Code)

		return nil
	})

	if err != nil {
		log.Printf("❌ Payment creation failed: %v", err)
		return nil, err
	}

	// Build summary
	response.Summary = &PaymentProcessingSummary{
		TotalAmount:            decimal.NewFromFloat(payment.Amount),
		ProcessingTime:         time.Since(startTime).String(),
		JournalEntryCreated:    journalResult != nil && journalResult.Success,
		AccountBalancesUpdated: journalResult != nil && len(journalResult.AccountUpdates) > 0,
		AllocationsCreated:     len(response.Allocations),
		TransactionID:          payment.Code,
	}

	log.Printf("🎉 Payment with journal created successfully: %s", payment.Code)
	return response, nil
}

// PreviewPaymentJournal previews the journal entry that would be created for a payment
func (eps *EnhancedPaymentServiceWithJournal) PreviewPaymentJournal(req *PaymentWithJournalRequest) (*PaymentJournalResult, error) {
	// Get contact information
	contact, err := eps.contactRepo.GetByID(req.ContactID)
	if err != nil {
		return nil, fmt.Errorf("contact not found: %w", err)
	}

	// Get cash/bank information
	_, err = eps.cashBankRepo.FindByID(req.CashBankID)
	if err != nil {
		return nil, fmt.Errorf("cash/bank account not found: %w", err)
	}

	// Create journal request for preview
	journalReq := &PaymentJournalRequest{
		PaymentID:   0, // Preview doesn't have payment ID yet
		ContactID:   uint64(req.ContactID),
		Amount:      decimal.NewFromFloat(req.Amount),
		Date:        req.Date,
		Method:      req.Method,
		Reference:   req.Reference,
		Notes:       req.Notes,
		CashBankID:  uint64(req.CashBankID),
		CreatedBy:   uint64(req.UserID),
		ContactType: contact.Type,
		ContactName: contact.Name,
	}

	// Generate preview
	return eps.journalFactory.PreviewPaymentJournalEntry(journalReq)
}

// GetPaymentWithJournal retrieves a payment with its journal entry details
func (eps *EnhancedPaymentServiceWithJournal) GetPaymentWithJournal(paymentID uint) (*PaymentWithJournalResponse, error) {
	// Get payment with relations
	var payment models.Payment
	if err := eps.db.Preload("Contact").Preload("User").First(&payment, paymentID).Error; err != nil {
		return nil, fmt.Errorf("payment not found: %w", err)
	}

	response := &PaymentWithJournalResponse{
		Payment: &payment,
		Success: true,
	}

	// Get journal entry if exists
	if payment.JournalEntryID != nil {
		journalEntry, err := eps.journalService.GetJournalEntry(uint64(*payment.JournalEntryID))
		if err != nil {
			log.Printf("⚠️ Warning: Failed to load journal entry %d: %v", *payment.JournalEntryID, err)
		} else {
			response.JournalResult = &PaymentJournalResult{
				JournalEntry: journalEntry,
				Success:      true,
				Message:      "Journal entry loaded successfully",
			}
		}
	}

	// Get contact
	if payment.Contact.ID == 0 {
		contact, err := eps.contactRepo.GetByID(payment.ContactID)
		if err == nil {
			response.Contact = contact
		}
	} else {
		response.Contact = &payment.Contact
	}

	// Get allocations
	var allocations []models.PaymentAllocation
	if err := eps.db.Where("payment_id = ?", paymentID).Find(&allocations).Error; err == nil {
		response.Allocations = allocations
	}

	return response, nil
}

// ReversePayment reverses a payment and its journal entry
func (eps *EnhancedPaymentServiceWithJournal) ReversePayment(paymentID uint, reason string, userID uint) (*PaymentWithJournalResponse, error) {
	var response *PaymentWithJournalResponse
	err := eps.db.Transaction(func(tx *gorm.DB) error {
		// Get payment
		var payment models.Payment
		if err := tx.First(&payment, paymentID).Error; err != nil {
			return fmt.Errorf("payment not found: %w", err)
		}

		if payment.Status != models.PaymentStatusCompleted {
			return fmt.Errorf("only completed payments can be reversed")
		}

		// Guard: check if payment date is in a closed accounting period
		if err := CheckClosedPeriod(eps.db, payment.Date); err != nil {
			return err
		}

		// Reverse SSOT journal entry if exists
		var journalResult *PaymentJournalResult
		if payment.JournalEntryID != nil {
			// Load original journal entry with lines
			var originalJournal models.SSOTJournalEntry
			if err := tx.Preload("Lines").First(&originalJournal, *payment.JournalEntryID).Error; err != nil {
				log.Printf("⚠️ Warning: Could not load original journal entry %d for reversal: %v", *payment.JournalEntryID, err)
			} else {
				// Build reversed journal lines (swap debit/credit)
				var reversalLines []JournalLineRequest
				for _, line := range originalJournal.Lines {
					reversalLines = append(reversalLines, JournalLineRequest{
						AccountID:    line.AccountID,
						DebitAmount:  line.CreditAmount, // swap
						CreditAmount: line.DebitAmount,  // swap
						Description:  fmt.Sprintf("Reversal - %s", line.Description),
					})
				}

				if len(reversalLines) >= 2 {
					reversalRequest := &JournalEntryRequest{
						SourceType:  models.SSOTSourceTypePayment,
						SourceID:    uint64(payment.ID),
						Reference:   fmt.Sprintf("REV-%s", payment.Code),
						EntryDate:   time.Now(),
						Description: fmt.Sprintf("Reversal of %s - %s", payment.Code, reason),
						Lines:       reversalLines,
						AutoPost:    true,
						CreatedBy:   uint64(userID),
					}

					journalService := NewUnifiedJournalService(tx)
					reversalEntry, revErr := journalService.CreateJournalEntryWithTx(tx, reversalRequest)
					if revErr != nil {
						log.Printf("⚠️ Warning: Failed to create reversal journal entry: %v", revErr)
					} else {
						log.Printf("✅ Reversal journal created: ID=%d", reversalEntry.ID)
						journalResult = &PaymentJournalResult{
							Success:      true,
							JournalEntry: reversalEntry,
							Message:      fmt.Sprintf("Reversal journal %s created", reversalEntry.EntryNumber),
						}
					}
				}
			}
		}

		// Update payment status
		payment.Status = models.PaymentStatusReversed
		payment.Notes = fmt.Sprintf("%s [REVERSED: %s]", payment.Notes, reason)
		if err := tx.Save(&payment).Error; err != nil {
			return fmt.Errorf("failed to update payment status: %w", err)
		}

		// Reverse Cash & Bank movement to keep balances in sync.
		// The Payment model does not store CashBankID, so we look up the
		// original CashBankTransaction that was recorded during payment creation.
		var originalCBTx models.CashBankTransaction
		cbTxErr := tx.Where("reference_id = ? AND reference_type IN (?)", payment.ID,
			[]string{"PAYMENT", "PAYMENT_RECEIVED", "PAYMENT_MADE"}).
			Order("id DESC").First(&originalCBTx).Error
		if cbTxErr != nil {
			log.Printf("⚠️ Warning: Could not find original CashBankTransaction for payment %d: %v", payment.ID, cbTxErr)
		} else {
			var cashBank models.CashBank
			if err := tx.First(&cashBank, originalCBTx.CashBankID).Error; err != nil {
				log.Printf("⚠️ Warning: Could not find cash/bank account %d for reversal: %v", originalCBTx.CashBankID, err)
			} else {
				// The original movement amount is stored on the CashBankTransaction.
				// To reverse, we simply negate it.
				reverseMovement := -originalCBTx.Amount
				refType := "PAYMENT_REVERSAL"

				newBalance := cashBank.Balance + reverseMovement
				cashBank.Balance = newBalance
				if err := tx.Save(&cashBank).Error; err != nil {
					return fmt.Errorf("failed to reverse cash/bank balance: %w", err)
				}

				// ✅ CRITICAL: Sync linked COA account balance after reversal
				if cashBank.AccountID > 0 {
					if err := tx.Exec("UPDATE accounts SET balance = ?, updated_at = NOW() WHERE id = ? AND deleted_at IS NULL",
						newBalance, cashBank.AccountID).Error; err != nil {
						log.Printf("⚠️ Warning: Failed to sync COA balance for GL account %d on reversal: %v", cashBank.AccountID, err)
					} else {
						log.Printf("🔄 Synced COA balance for GL account %d to %.2f (reversal)", cashBank.AccountID, newBalance)
					}
				}

				// Insert reversal transaction row for Cash & Bank UI/history
				txRow := &models.CashBankTransaction{
					CashBankID:      cashBank.ID,
					ReferenceType:   refType,
					ReferenceID:     payment.ID,
					Amount:          reverseMovement,
					BalanceAfter:    newBalance,
					TransactionDate: time.Now(),
					Notes:           fmt.Sprintf("%s %s - Reversal: %s", refType, payment.Code, reason),
				}
				if err := tx.Create(txRow).Error; err != nil {
					return fmt.Errorf("failed to create reversal cashbank transaction: %w", err)
				}

				log.Printf("✅ Cash/Bank balance reversed for account %d: movement=%.2f, newBalance=%.2f", cashBank.ID, reverseMovement, newBalance)
			}
		}

		response = &PaymentWithJournalResponse{
			Payment:       &payment,
			JournalResult: journalResult,
			Success:       true,
			Message:       fmt.Sprintf("Payment %s reversed successfully", payment.Code),
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return response, nil
}

// Helper methods

// resolveReceivableAccountID finds the Accounts Receivable (Piutang Usaha) GL account ID
// using the standard fallback chain: 1102-001 → 1102 → 1201 → name search
func (eps *EnhancedPaymentServiceWithJournal) resolveReceivableAccountID(tx *gorm.DB) (uint64, error) {
	var account models.Account

	// Try specific AR leaf account first
	if err := tx.Where("code = ? AND deleted_at IS NULL", "1102-001").First(&account).Error; err == nil {
		return uint64(account.ID), nil
	}

	// Try general AR account
	if err := tx.Where("code = ? AND deleted_at IS NULL", "1102").First(&account).Error; err == nil {
		return uint64(account.ID), nil
	}

	// Try legacy AR account
	if err := tx.Where("code = ? AND deleted_at IS NULL", "1201").First(&account).Error; err == nil {
		return uint64(account.ID), nil
	}

	// Fallback: search by name
	if err := tx.Where("LOWER(name) LIKE ? AND deleted_at IS NULL", "%piutang%usaha%").First(&account).Error; err == nil {
		return uint64(account.ID), nil
	}

	return 0, fmt.Errorf("accounts receivable (Piutang Usaha) account not found - please create account with code 1102 or 1102-001")
}

// resolvePayableAccountID finds the Accounts Payable (Utang Usaha) GL account ID
// using the standard fallback chain: 2101-001 → 2101 → name search
func (eps *EnhancedPaymentServiceWithJournal) resolvePayableAccountID(tx *gorm.DB) (uint64, error) {
	var account models.Account

	// Try specific AP leaf account first
	if err := tx.Where("code = ? AND deleted_at IS NULL", "2101-001").First(&account).Error; err == nil {
		return uint64(account.ID), nil
	}

	// Try general AP account
	if err := tx.Where("code = ? AND deleted_at IS NULL", "2101").First(&account).Error; err == nil {
		return uint64(account.ID), nil
	}

	// Fallback: search by name (hutang usaha / utang usaha)
	if err := tx.Where("LOWER(name) LIKE ? AND deleted_at IS NULL", "%utang%usaha%").First(&account).Error; err == nil {
		return uint64(account.ID), nil
	}
	if err := tx.Where("LOWER(name) LIKE ? AND deleted_at IS NULL", "%hutang%usaha%").First(&account).Error; err == nil {
		return uint64(account.ID), nil
	}

	return 0, fmt.Errorf("accounts payable (Utang Usaha) account not found - please create account with code 2101 or 2101-001")
}

// resolvePPHAccountID resolves the liability account for PPh withholding tax based on type.
// Maps: PPH_21→2102-003, PPH_23→2102-004, PPH_4_2→2102-005, PPH_22→2102-006
func (eps *EnhancedPaymentServiceWithJournal) resolvePPHAccountID(tx *gorm.DB, pphType string) (uint64, error) {
	// Map PPh type to account code (aligned with account_templates.go base COA)
	pphAccountCodes := map[string]struct {
		code string
		name string
	}{
		"PPH_21":  {code: "2102-003", name: "Hutang PPh Pasal 21"},
		"PPH_23":  {code: "2102-004", name: "Hutang PPh Pasal 23"},
		"PPH_4_2": {code: "2102-005", name: "Hutang PPh Pasal 4(2) Final"},
		"PPH_22":  {code: "2102-006", name: "Hutang PPh Pasal 22"},
	}

	mapping, exists := pphAccountCodes[pphType]
	if !exists {
		return 0, fmt.Errorf("unsupported PPh type: %s (supported: PPH_21, PPH_23, PPH_4_2, PPH_22)", pphType)
	}

	// Try to find the specific PPh account
	var account models.Account
	if err := tx.Where("code = ? AND deleted_at IS NULL", mapping.code).First(&account).Error; err == nil {
		return uint64(account.ID), nil
	}

	// Try parent account 2102 (Utang Pajak) as fallback
	if err := tx.Where("code = ? AND deleted_at IS NULL", "2102").First(&account).Error; err == nil {
		return uint64(account.ID), nil
	}

	// Fallback: search by name
	if err := tx.Where("LOWER(name) LIKE ? AND deleted_at IS NULL", "%hutang%pph%").First(&account).Error; err == nil {
		return uint64(account.ID), nil
	}

	return 0, fmt.Errorf("PPh liability account %s (%s) not found - please create it in Chart of Accounts under code %s",
		mapping.name, mapping.code, mapping.code)
}

// applyCashBankMovement updates cash_banks.balance and records a CashBankTransaction for the payment
// without generating additional journal entries (SSOT already handled journal posting).
func (eps *EnhancedPaymentServiceWithJournal) applyCashBankMovement(
	tx *gorm.DB,
	cashBank *models.CashBank,
	contactType string,
	amount float64,
	payment *models.Payment,
	date time.Time,
) error {
	movement := amount
	refType := "PAYMENT"
	if strings.ToUpper(contactType) == "CUSTOMER" {
		// Incoming money
		movement = amount
		refType = "PAYMENT_RECEIVED"
	} else if strings.ToUpper(contactType) == "VENDOR" {
		// Outgoing money
		movement = -amount
		refType = "PAYMENT_MADE"
	}

	// Update balance
	newBalance := cashBank.Balance + movement
	cashBank.Balance = newBalance
	if err := tx.Save(cashBank).Error; err != nil {
		return fmt.Errorf("save cash bank: %w", err)
	}

	// ✅ CRITICAL: Sync linked COA account balance to maintain COA <> CashBank consistency
	if cashBank.AccountID > 0 {
		if err := tx.Exec("UPDATE accounts SET balance = ?, updated_at = NOW() WHERE id = ? AND deleted_at IS NULL",
			newBalance, cashBank.AccountID).Error; err != nil {
			log.Printf("⚠️ Warning: Failed to sync COA balance for GL account %d: %v", cashBank.AccountID, err)
		} else {
			log.Printf("🔄 Synced COA balance for GL account %d to %.2f", cashBank.AccountID, newBalance)
		}
	}

	// Insert transaction row for Cash&Bank UI/history
	txRow := &models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   refType,
		ReferenceID:     payment.ID,
		Amount:          movement,
		BalanceAfter:    newBalance,
		TransactionDate: date,
		Notes:           fmt.Sprintf("%s %s", refType, payment.Code),
	}
	if err := tx.Create(txRow).Error; err != nil {
		return fmt.Errorf("create cashbank transaction: %w", err)
	}

	return nil
}

// autoDetectPaymentMethod determines payment method based on contact type
func (eps *EnhancedPaymentServiceWithJournal) autoDetectPaymentMethod(contactType string) string {
	if contactType == "CUSTOMER" {
		return "RECEIVABLE"
	} else if contactType == "VENDOR" {
		return "PAYABLE"
	}
	return "CASH"
}

// generatePaymentCode generates unique payment code
func (eps *EnhancedPaymentServiceWithJournal) generatePaymentCode(tx *gorm.DB, method, contactType string) (string, error) {
	prefix := "PAY"
	if contactType == "CUSTOMER" {
		prefix = "RCV"
	} else if contactType == "VENDOR" {
		prefix = "PAY"
	}

	// Get current date
	now := time.Now()
	datePrefix := now.Format("2006/01")

	// Find next sequence number
	var count int64
	tx.Model(&models.Payment{}).
		Where("code LIKE ?", fmt.Sprintf("%s-%s-%%", prefix, datePrefix)).
		Count(&count)

	// Generate code
	return fmt.Sprintf("%s-%s-%04d", prefix, datePrefix, count+1), nil
}

// createPaymentAllocations creates payment allocations to invoices/bills
func (eps *EnhancedPaymentServiceWithJournal) createPaymentAllocations(
	tx *gorm.DB,
	payment *models.Payment,
	req *PaymentWithJournalRequest,
	contactType string,
) ([]models.PaymentAllocation, error) {
	var allocations []models.PaymentAllocation

	// 🔥 FIX: Handle array-based allocations (preferred method)
	if contactType == "CUSTOMER" {
		// Handle multiple invoice allocations
		if len(req.InvoiceAllocations) > 0 {
			log.Printf("📝 Processing %d invoice allocations", len(req.InvoiceAllocations))
			remainingAmount := payment.Amount

			for i, alloc := range req.InvoiceAllocations {
				if alloc.InvoiceID == nil {
					log.Printf("⚠️ Skipping allocation %d: no invoice ID", i)
					continue
				}

				if remainingAmount <= 0 {
					log.Printf("⚠️ No remaining amount for allocation %d", i)
					break
				}

				// Validate invoice
				var sale models.Sale
				if err := tx.First(&sale, *alloc.InvoiceID).Error; err != nil {
					return nil, fmt.Errorf("invoice %d not found: %w", *alloc.InvoiceID, err)
				}

				// Validate status
				if err := eps.statusValidator.ValidatePaymentAllocation(sale.Status, *alloc.InvoiceID); err != nil {
					log.Printf("❌ Payment allocation blocked: %v", err)
					return nil, err
				}

				// Calculate allocated amount
				allocatedAmount := alloc.Amount
				if allocatedAmount > remainingAmount {
					allocatedAmount = remainingAmount
					log.Printf("⚠️ Adjusting amount to remaining: %.2f -> %.2f", alloc.Amount, allocatedAmount)
				}
				if allocatedAmount > sale.OutstandingAmount {
					allocatedAmount = sale.OutstandingAmount
					log.Printf("⚠️ Adjusting amount to outstanding: %.2f -> %.2f", allocatedAmount, sale.OutstandingAmount)
				}

				// Create allocation
				allocation := models.PaymentAllocation{
					PaymentID:       uint64(payment.ID),
					InvoiceID:       alloc.InvoiceID,
					AllocatedAmount: allocatedAmount,
				}

				if err := tx.Create(&allocation).Error; err != nil {
					return nil, fmt.Errorf("failed to create invoice allocation: %w", err)
				}

				allocations = append(allocations, allocation)

				// Update sale outstanding
				if err := eps.updateSaleOutstanding(tx, *alloc.InvoiceID, allocatedAmount); err != nil {
					log.Printf("⚠️ Warning: Failed to update sale outstanding: %v", err)
				}

				remainingAmount -= allocatedAmount
				log.Printf("✅ Invoice allocation %d complete. Remaining: %.2f", i+1, remainingAmount)
			}
		} else if req.TargetInvoiceID != nil {
			// Legacy: single invoice allocation
			log.Printf("📝 Processing single invoice allocation (legacy)")
			var sale models.Sale
			if err := tx.First(&sale, *req.TargetInvoiceID).Error; err != nil {
				return nil, fmt.Errorf("invoice not found: %w", err)
			}

			if err := eps.statusValidator.ValidatePaymentAllocation(sale.Status, *req.TargetInvoiceID); err != nil {
				log.Printf("❌ Payment allocation blocked: %v", err)
				return nil, err
			}

			allocation := models.PaymentAllocation{
				PaymentID:       uint64(payment.ID),
				InvoiceID:       req.TargetInvoiceID,
				AllocatedAmount: payment.Amount,
			}

			if err := tx.Create(&allocation).Error; err != nil {
				return nil, fmt.Errorf("failed to create invoice allocation: %w", err)
			}

			allocations = append(allocations, allocation)

			if err := eps.updateSaleOutstanding(tx, *req.TargetInvoiceID, payment.Amount); err != nil {
				log.Printf("⚠️ Warning: Failed to update sale outstanding: %v", err)
			}
		}
	} else if contactType == "VENDOR" {
		// Handle multiple bill allocations
		if len(req.BillAllocations) > 0 {
			log.Printf("📝 Processing %d bill allocations", len(req.BillAllocations))
			remainingAmount := payment.Amount

			for i, alloc := range req.BillAllocations {
				if alloc.BillID == nil {
					log.Printf("⚠️ Skipping allocation %d: no bill ID", i)
					continue
				}

				if remainingAmount <= 0 {
					log.Printf("⚠️ No remaining amount for allocation %d", i)
					break
				}

				// Validate purchase
				var purchase models.Purchase
				if err := tx.First(&purchase, *alloc.BillID).Error; err != nil {
					return nil, fmt.Errorf("bill %d not found: %w", *alloc.BillID, err)
				}

				// Validate ownership
				if purchase.VendorID != req.ContactID {
					return nil, fmt.Errorf("bill %d does not belong to this vendor", *alloc.BillID)
				}

				// Calculate allocated amount
				allocatedAmount := alloc.Amount
				if allocatedAmount > remainingAmount {
					allocatedAmount = remainingAmount
					log.Printf("⚠️ Adjusting amount to remaining: %.2f -> %.2f", alloc.Amount, allocatedAmount)
				}
				if allocatedAmount > purchase.OutstandingAmount {
					allocatedAmount = purchase.OutstandingAmount
					log.Printf("⚠️ Adjusting amount to outstanding: %.2f -> %.2f", allocatedAmount, purchase.OutstandingAmount)
				}

				// Create allocation
				allocation := models.PaymentAllocation{
					PaymentID:       uint64(payment.ID),
					BillID:          alloc.BillID,
					AllocatedAmount: allocatedAmount,
				}

				if err := tx.Create(&allocation).Error; err != nil {
					return nil, fmt.Errorf("failed to create bill allocation: %w", err)
				}

				allocations = append(allocations, allocation)

				// 🔥 FIX: Update purchase outstanding
				if err := eps.updatePurchaseOutstanding(tx, *alloc.BillID, allocatedAmount); err != nil {
					log.Printf("❌ CRITICAL: Failed to update purchase outstanding: %v", err)
					return nil, fmt.Errorf("failed to update purchase outstanding: %w", err)
				}
				log.Printf("✅ Purchase outstanding updated for bill %d", *alloc.BillID)

				remainingAmount -= allocatedAmount
				log.Printf("✅ Bill allocation %d complete. Remaining: %.2f", i+1, remainingAmount)
			}
		} else if req.TargetBillID != nil {
			// Legacy: single bill allocation
			log.Printf("📝 Processing single bill allocation (legacy)")
			allocation := models.PaymentAllocation{
				PaymentID:       uint64(payment.ID),
				BillID:          req.TargetBillID,
				AllocatedAmount: payment.Amount,
			}

			if err := tx.Create(&allocation).Error; err != nil {
				return nil, fmt.Errorf("failed to create bill allocation: %w", err)
			}

			allocations = append(allocations, allocation)

			if err := eps.updatePurchaseOutstanding(tx, *req.TargetBillID, payment.Amount); err != nil {
				log.Printf("❌ CRITICAL: Failed to update purchase outstanding: %v", err)
				return nil, fmt.Errorf("failed to update purchase outstanding: %w", err)
			}
		}
	}

	return allocations, nil
}

// updateSaleOutstanding updates sale outstanding amount after payment allocation
func (eps *EnhancedPaymentServiceWithJournal) updateSaleOutstanding(tx *gorm.DB, saleID uint, paidAmount float64) error {
	var sale models.Sale
	if err := tx.First(&sale, saleID).Error; err != nil {
		return fmt.Errorf("sale not found: %w", err)
	}

	// Update amounts
	sale.PaidAmount += paidAmount
	sale.OutstandingAmount -= paidAmount

	// Update status if fully paid
	if sale.OutstandingAmount <= 0.01 { // Allow small rounding differences
		sale.Status = models.SaleStatusPaid
		sale.OutstandingAmount = 0 // Ensure exact zero
	}

	return tx.Save(&sale).Error
}

// updatePurchaseOutstanding updates purchase outstanding amount after payment allocation
func (eps *EnhancedPaymentServiceWithJournal) updatePurchaseOutstanding(tx *gorm.DB, purchaseID uint, paidAmount float64) error {
	var purchase models.Purchase
	if err := tx.First(&purchase, purchaseID).Error; err != nil {
		return fmt.Errorf("purchase not found: %w", err)
	}

	log.Printf("📝 Updating purchase amounts: PaidAmount %.2f -> %.2f, Outstanding %.2f -> %.2f",
		purchase.PaidAmount, purchase.PaidAmount+paidAmount,
		purchase.OutstandingAmount, purchase.OutstandingAmount-paidAmount)

	// Update amounts
	purchase.PaidAmount += paidAmount
	purchase.OutstandingAmount -= paidAmount

	// Update matching status if fully paid
	if purchase.OutstandingAmount <= 0.01 { // Allow small rounding differences
		purchase.MatchingStatus = models.PurchaseMatchingMatched
		purchase.OutstandingAmount = 0 // Ensure exact zero
		log.Printf("✅ Purchase fully paid, matching status updated to MATCHED")
	} else {
		purchase.MatchingStatus = models.PurchaseMatchingPartial
		log.Printf("✅ Purchase partially paid (Outstanding: %.2f)", purchase.OutstandingAmount)
	}

	if err := tx.Save(&purchase).Error; err != nil {
		log.Printf("❌ Failed to save purchase: %v", err)
		return err
	}
	log.Printf("✅ Purchase updated successfully")

	return nil
}

// validatePaymentRequest validates the payment request
func (eps *EnhancedPaymentServiceWithJournal) validatePaymentRequest(req *PaymentWithJournalRequest) error {
	if req.ContactID == 0 {
		return fmt.Errorf("contact ID is required")
	}

	if req.CashBankID == 0 {
		return fmt.Errorf("cash/bank account ID is required")
	}

	if req.Amount <= 0 {
		return fmt.Errorf("payment amount must be positive")
	}

	if req.Date.IsZero() {
		return fmt.Errorf("payment date is required")
	}

	if req.UserID == 0 {
		return fmt.Errorf("user ID is required")
	}

	// Validate date is not in future (allow today)
	if req.Date.After(time.Now().AddDate(0, 0, 1)) {
		return fmt.Errorf("payment date cannot be more than 1 day in the future")
	}

	return nil
}

// GetAccountBalanceUpdates retrieves account balance updates from journal integration
func (eps *EnhancedPaymentServiceWithJournal) GetAccountBalanceUpdates(paymentID uint) ([]AccountBalanceUpdate, error) {
	// Get payment
	var payment models.Payment
	if err := eps.db.First(&payment, paymentID).Error; err != nil {
		return nil, fmt.Errorf("payment not found: %w", err)
	}

	// If payment has journal entry, get the account updates from journal lines
	if payment.JournalEntryID != nil {
		journalEntry, err := eps.journalService.GetJournalEntry(uint64(*payment.JournalEntryID))
		if err != nil {
			return nil, fmt.Errorf("failed to get journal entry: %w", err)
		}

		// Convert journal lines to account balance updates
		var updates []AccountBalanceUpdate
		for _, line := range journalEntry.Lines {
			update := AccountBalanceUpdate{
				AccountID: line.AccountID,
				Change:    line.DebitAmount.Sub(line.CreditAmount),
			}

			if line.DebitAmount.GreaterThan(decimal.Zero) {
				update.ChangeType = "INCREASE"
			} else {
				update.ChangeType = "DECREASE"
			}

			updates = append(updates, update)
		}

		return updates, nil
	}

	return nil, fmt.Errorf("payment has no journal entry")
}
