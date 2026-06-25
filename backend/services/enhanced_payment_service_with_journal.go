package services

import (
	"fmt"
	"log"
	"math"
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
	ExpenseAccountID  uint      `json:"expense_account_id"`
	CashBankID        uint      `json:"cash_bank_id" binding:"required"`
	Date              time.Time `json:"date" binding:"required"`
	Amount            float64   `json:"amount" binding:"required,min=0.01"`
	ExchangeRate      float64   `json:"exchange_rate"`
	Method            string    `json:"method" binding:"required"`
	Reference         string    `json:"reference"`
	Notes             string    `json:"notes"`
	Description       string    `json:"description"`
	AutoCreateJournal bool      `json:"auto_create_journal"`
	UserID            uint      `json:"user_id"`

	// SaveAsDraft = true means save payment with status DRAFT without creating journal.
	// Used when submitted by employee role; finance must approve later.
	SaveAsDraft bool `json:"save_as_draft"`
	// RequireDirectorApproval routes the payment to director after finance manager review.
	RequireDirectorApproval bool `json:"require_director_approval"`
	// RequestedByFinanceManager is set by controller to enable finance-manager-specific routing.
	RequestedByFinanceManager bool `json:"-"`

	// PPh withholding tax fields (optional)
	PPHType   string  `json:"pph_type"`   // PPH_21, PPH_23, PPH_4_2, PPH_22, or empty
	PPHRate   float64 `json:"pph_rate"`   // Tax rate percentage (e.g. 2 for 2%)
	PPHAmount float64 `json:"pph_amount"` // Calculated tax amount

	// AllowLiabilityOverpayment permits paying a LIABILITY account beyond its
	// current credit balance, intentionally creating a debit-balance overpayment.
	AllowLiabilityOverpayment bool `json:"allow_liability_overpayment"`
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
	ContactID    uint      `json:"contact_id" binding:"required"`
	CashBankID   uint      `json:"cash_bank_id" binding:"required"`
	Date         time.Time `json:"date" binding:"required"`
	Amount       float64   `json:"amount" binding:"required,min=0.01"`
	ExchangeRate float64   `json:"exchange_rate"`
	Method       string    `json:"method" binding:"required"`
	Reference    string    `json:"reference"`
	Notes        string    `json:"notes"`

	// Journal integration options
	AutoCreateJournal       bool `json:"auto_create_journal" default:"true"`
	PreviewJournal          bool `json:"preview_journal" default:"false"`
	SaveAsDraft             bool `json:"save_as_draft"` // DRAFT mode for employee submissions
	RequireDirectorApproval bool `json:"require_director_approval"`

	// Allocation options (single target - legacy)
	TargetInvoiceID *uint `json:"target_invoice_id,omitempty"`
	TargetBillID    *uint `json:"target_bill_id,omitempty"`

	// Allocation arrays (multiple targets - preferred)
	InvoiceAllocations []AllocationItem `json:"invoice_allocations,omitempty"`
	BillAllocations    []AllocationItem `json:"bill_allocations,omitempty"`

	// User context
	UserID uint `json:"user_id"`
	// RequestedByFinanceManager is injected by controller to bypass finance-staff lane.
	RequestedByFinanceManager bool `json:"-"`
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

type expensePaymentCalculation struct {
	NormalizedCurrency  string
	AppliedExchangeRate float64
	NetCashPaymentIDR   float64
	NativeCashOut       float64
	BankBookOutIDR      float64
	ForexDiff           float64
	ForexGainLossAmount float64
	ForexGainLossType   string
}

func isPaymentDirectorApprovalPending(payment *models.Payment) bool {
	if payment == nil {
		return false
	}

	return payment.Status == models.PaymentStatusPending &&
		payment.RequireDirectorApproval &&
		strings.EqualFold(strings.TrimSpace(payment.ApprovalStage), models.PaymentApprovalStageDirector)
}

func routePaymentToDirector(tx *gorm.DB, payment *models.Payment, financeManagerUserID uint) error {
	if payment == nil {
		return fmt.Errorf("payment is required")
	}

	now := time.Now()
	payment.Status = models.PaymentStatusPending
	payment.RequireDirectorApproval = true
	payment.ApprovalStage = models.PaymentApprovalStageDirector
	payment.Reference = models.MarkPendingDraftAllocationsReference(payment.Reference)
	payment.FinanceManagerApprovedBy = &financeManagerUserID
	payment.FinanceManagerApprovedAt = &now
	payment.DirectorApprovedBy = nil
	payment.DirectorApprovedAt = nil
	payment.ApprovedBy = nil
	payment.ApprovedAt = nil
	payment.RejectedBy = nil
	payment.RejectedAt = nil
	payment.RejectionReason = ""

	if err := tx.Save(payment).Error; err != nil {
		return fmt.Errorf("failed to route payment to director approval: %w", err)
	}

	return nil
}

func routePaymentToAccounting(tx *gorm.DB, payment *models.Payment, approverUserID uint, directorApproval bool) error {
	if payment == nil {
		return fmt.Errorf("payment is required")
	}

	now := time.Now()
	payment.Status = models.PaymentStatusPendingAccounting
	payment.ApprovalStage = models.PaymentApprovalStageAccounting
	if directorApproval {
		payment.DirectorApprovedBy = &approverUserID
		payment.DirectorApprovedAt = &now
	} else {
		payment.FinanceManagerApprovedBy = &approverUserID
		payment.FinanceManagerApprovedAt = &now
	}
	payment.ApprovedBy = nil
	payment.ApprovedAt = nil
	payment.RejectedBy = nil
	payment.RejectedAt = nil
	payment.RejectionReason = ""

	if err := tx.Save(payment).Error; err != nil {
		return fmt.Errorf("failed to route payment to accounting review: %w", err)
	}

	return nil
}

func isPaymentAwaitingPostingReview(payment *models.Payment) bool {
	if payment == nil {
		return false
	}

	return payment.Status == models.PaymentStatusPendingFinalReview
}

func validateExpensePaymentAccountUsage(account models.Account, amount, pphAmount float64, allowLiabilityOverpayment bool) error {
	if account.Type != models.AccountTypeLiability {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(account.Code), "2102-003") {
		return fmt.Errorf("Hutang PPh 21 payroll wajib diselesaikan dengan memilih kewajiban payroll agar status payroll, jurnal, dan kas/bank tetap sinkron")
	}

	if pphAmount > 0 {
		return fmt.Errorf("PPh withholding is only valid for direct expense accounts; for liability settlement, leave PPh disabled")
	}

	if amount <= account.Balance+0.01 {
		return nil
	}

	if allowLiabilityOverpayment {
		return nil
	}

	return fmt.Errorf(
		"liability account %s - %s has balance %.2f, payment %.2f would create an overpayment/debit balance; choose an expense/prepaid asset account or set allow_liability_overpayment=true",
		account.Code,
		account.Name,
		account.Balance,
		amount,
	)
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
	if req.ExpenseAccountID == 0 {
		defaultExpenseAccountID, err := findDefaultExpenseAccountID(eps.db)
		if err != nil {
			return nil, fmt.Errorf("expense account ID is required: %w", err)
		}
		req.ExpenseAccountID = uint(defaultExpenseAccountID)
	}

	// DRAFT MODE: save payment without creating journal — for employee role submissions
	if req.SaveAsDraft || (req.RequestedByFinanceManager && req.RequireDirectorApproval) {
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
		if expenseAccount.Type != models.AccountTypeExpense && expenseAccount.Type != models.AccountTypeLiability {
			return fmt.Errorf("expense payment account must be EXPENSE or LIABILITY, got %s", expenseAccount.Type)
		}
		if err := validateExpensePaymentAccountUsage(expenseAccount, req.Amount, req.PPHAmount, req.AllowLiabilityOverpayment); err != nil {
			return err
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

		calculation, err := eps.calculateExpensePaymentAmounts(cashBank, req.Amount, req.PPHAmount, req.ExchangeRate)
		if err != nil {
			return err
		}

		// Build payment model (cash out, treated similar to PAY prefix)
		var financeManagerApprovedBy *uint
		var financeManagerApprovedAt *time.Time
		var approvedBy *uint
		var approvedAt *time.Time
		if req.RequestedByFinanceManager {
			now := time.Now()
			financeManagerApprovedBy = &req.UserID
			financeManagerApprovedAt = &now
			approvedBy = &req.UserID
			approvedAt = &now
		}

		payment = &models.Payment{
			ContactID:                req.ContactID,
			UserID:                   req.UserID,
			Date:                     req.Date,
			Amount:                   req.Amount,
			NativeAmount:             calculation.NativeCashOut,
			Currency:                 calculation.NormalizedCurrency,
			ExchangeRate:             calculation.AppliedExchangeRate,
			BookAmountIDR:            calculation.BankBookOutIDR,
			ForexGainLossAmount:      calculation.ForexGainLossAmount,
			ForexGainLossType:        calculation.ForexGainLossType,
			Method:                   req.Method,
			ExpenseAccountID:         &req.ExpenseAccountID,
			CashBankID:               &req.CashBankID,
			Reference:                req.Reference,
			Status:                   models.PaymentStatusCompleted,
			RequireDirectorApproval:  false,
			FinanceManagerApprovedBy: financeManagerApprovedBy,
			FinanceManagerApprovedAt: financeManagerApprovedAt,
			ApprovedBy:               approvedBy,
			ApprovedAt:               approvedAt,
			PaymentType:              models.PaymentTypeExpense,
			Notes:                    fmt.Sprintf("%s [Contact: %s]", req.Notes, contactName),
			PPHType:                  req.PPHType,
			PPHRate:                  req.PPHRate,
			PPHAmount:                req.PPHAmount,
		}
		snapshot, snapshotErr := loadCashBankSnapshot(tx, req.CashBankID)
		if snapshotErr != nil {
			return snapshotErr
		}
		applyCashBankSnapshotToPayment(payment, snapshot)

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
		bankCreditAmount := decimal.NewFromFloat(calculation.BankBookOutIDR)

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
					CreditAmount: bankCreditAmount,
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
					CreditAmount: bankCreditAmount,
					Description:  fmt.Sprintf("Expense Payment %s - Cash/Bank", payment.Code),
				},
			}
		}
		if fxLine, err := eps.buildRealizedFXJournalLine(tx, calculation.ForexDiff, true, payment.Code); err != nil {
			return err
		} else if fxLine != nil {
			journalLines = append(journalLines, *fxLine)
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
		if err := eps.applyCashBankMovement(tx, cashBank, "VENDOR", calculation.NetCashPaymentIDR, payment, req.Date); err != nil {
			return fmt.Errorf("failed to update cash/bank balances: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return payment, nil
}

// createExpensePaymentDraft saves an expense payment without creating a journal.
// It supports both finance-manager review drafts and director-pending submissions.
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
		if expenseAccount.Type != models.AccountTypeExpense && expenseAccount.Type != models.AccountTypeLiability {
			return fmt.Errorf("expense payment account must be EXPENSE or LIABILITY, got %s", expenseAccount.Type)
		}
		if err := validateExpensePaymentAccountUsage(expenseAccount, req.Amount, req.PPHAmount, req.AllowLiabilityOverpayment); err != nil {
			return err
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

		calculation, err := eps.calculateExpensePaymentAmounts(cashBank, req.Amount, req.PPHAmount, req.ExchangeRate)
		if err != nil {
			return err
		}

		status := models.PaymentStatusDraft
		approvalStage := models.PaymentApprovalStageFinanceManager
		var financeManagerApprovedBy *uint
		var financeManagerApprovedAt *time.Time
		if req.RequestedByFinanceManager && req.RequireDirectorApproval {
			status = models.PaymentStatusPending
			approvalStage = models.PaymentApprovalStageDirector
			now := time.Now()
			financeManagerApprovedBy = &req.UserID
			financeManagerApprovedAt = &now
		}

		reference := req.Reference
		if req.AllowLiabilityOverpayment &&
			expenseAccount.Type == models.AccountTypeLiability &&
			req.Amount > expenseAccount.Balance+0.01 {
			reference = models.MarkExpenseLiabilityOverpaymentReference(reference)
		}

		payment = &models.Payment{
			ContactID:                req.ContactID,
			UserID:                   req.UserID,
			Date:                     req.Date,
			Amount:                   req.Amount,
			NativeAmount:             calculation.NativeCashOut,
			Currency:                 calculation.NormalizedCurrency,
			ExchangeRate:             calculation.AppliedExchangeRate,
			BookAmountIDR:            calculation.BankBookOutIDR,
			ForexGainLossAmount:      calculation.ForexGainLossAmount,
			ForexGainLossType:        calculation.ForexGainLossType,
			Method:                   req.Method,
			ExpenseAccountID:         &req.ExpenseAccountID,
			CashBankID:               &req.CashBankID,
			Reference:                reference,
			Status:                   status,
			ApprovalStage:            approvalStage,
			RequireDirectorApproval:  req.RequireDirectorApproval,
			FinanceManagerApprovedBy: financeManagerApprovedBy,
			FinanceManagerApprovedAt: financeManagerApprovedAt,
			PaymentType:              models.PaymentTypeExpense,
			Notes:                    fmt.Sprintf("%s [Contact: %s] [Beban: %s] [CashBank: %s]", req.Notes, contactName, expenseAccount.Name, cashBank.Name),
			PPHType:                  req.PPHType,
			PPHRate:                  req.PPHRate,
			PPHAmount:                req.PPHAmount,
		}
		snapshot, snapshotErr := loadCashBankSnapshot(tx, req.CashBankID)
		if snapshotErr != nil {
			return snapshotErr
		}
		applyCashBankSnapshotToPayment(payment, snapshot)

		code, err := eps.generatePaymentCode(tx, req.Method, "VENDOR")
		if err != nil {
			return fmt.Errorf("failed to generate payment code: %w", err)
		}
		payment.Code = code

		if err := tx.Create(payment).Error; err != nil {
			return fmt.Errorf("failed to create draft expense payment: %w", err)
		}

		log.Printf("Expense payment saved for approval: ID=%d, Code=%s, Status=%s, Amount=%.2f, ExpenseAcc=%s",
			payment.ID, payment.Code, payment.Status, req.Amount, expenseAccount.Name)
		return nil
	})

	if err != nil {
		return nil, err
	}
	return payment, nil
}

// ApproveExpensePayment processes expense payment approval.
// Finance manager can either post directly or route to director; director finalizes routed payments.
func (eps *EnhancedPaymentServiceWithJournal) ApproveExpensePayment(paymentID uint, approverUserID uint, requireDirectorApproval bool) (*models.Payment, error) {
	var payment models.Payment
	if err := eps.db.First(&payment, paymentID).Error; err != nil {
		return nil, fmt.Errorf("payment not found: %w", err)
	}

	if payment.Status != models.PaymentStatusDraft && !isPaymentDirectorApprovalPending(&payment) && !isPaymentAwaitingPostingReview(&payment) {
		return nil, fmt.Errorf("payment is not awaiting approval (current: %s)", payment.Status)
	}
	if payment.PaymentType != models.PaymentTypeExpense {
		return nil, fmt.Errorf("only expense payments can be approved via this endpoint")
	}

	if err := CheckClosedPeriod(eps.db, payment.Date); err != nil {
		return nil, err
	}

	err := eps.db.Transaction(func(tx *gorm.DB) error {
		directorApprovalStep := isPaymentDirectorApprovalPending(&payment)
		if payment.Status == models.PaymentStatusDraft && (payment.RequireDirectorApproval || requireDirectorApproval) {
			return routePaymentToDirector(tx, &payment, approverUserID)
		}
		if payment.Status == models.PaymentStatusDraft {
			return routePaymentToAccounting(tx, &payment, approverUserID, false)
		}
		if directorApprovalStep {
			return routePaymentToAccounting(tx, &payment, approverUserID, true)
		}
		if !isPaymentAwaitingPostingReview(&payment) {
			return fmt.Errorf("payment is not ready for final finance manager review (current: %s)", payment.Status)
		}

		expenseAccountID := uint(0)
		cashBankID := uint(0)
		if payment.ExpenseAccountID != nil {
			expenseAccountID = *payment.ExpenseAccountID
		}
		if payment.CashBankID != nil {
			cashBankID = *payment.CashBankID
		}
		if expenseAccountID == 0 || cashBankID == 0 {
			legacyExpenseAccountID, legacyCashBankID := models.ExtractExpenseDraftRoutingFromReference(payment.Reference)
			if expenseAccountID == 0 {
				expenseAccountID = legacyExpenseAccountID
			}
			if cashBankID == 0 {
				cashBankID = legacyCashBankID
			}
		}
		if expenseAccountID == 0 || cashBankID == 0 {
			return fmt.Errorf("cannot find expense account or cash bank info in draft payment %s; please update and re-submit", payment.Code)
		}

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
		if expenseAccount.Type != models.AccountTypeExpense && expenseAccount.Type != models.AccountTypeLiability {
			return fmt.Errorf("expense payment account must be EXPENSE or LIABILITY, got %s", expenseAccount.Type)
		}
		allowLiabilityOverpayment := models.HasExpenseLiabilityOverpaymentReference(payment.Reference)
		if err := validateExpensePaymentAccountUsage(expenseAccount, payment.Amount, payment.PPHAmount, allowLiabilityOverpayment); err != nil {
			return err
		}

		calculation, err := eps.calculateExpensePaymentAmounts(cashBank, payment.Amount, payment.PPHAmount, payment.ExchangeRate)
		if err != nil {
			return err
		}
		payment.Currency = calculation.NormalizedCurrency
		payment.ExchangeRate = calculation.AppliedExchangeRate
		payment.NativeAmount = calculation.NativeCashOut
		payment.BookAmountIDR = calculation.BankBookOutIDR
		payment.ForexGainLossAmount = calculation.ForexGainLossAmount
		payment.ForexGainLossType = calculation.ForexGainLossType
		payment.Reference = models.StripInternalPaymentReference(payment.Reference)
		payment.ExpenseAccountID = &expenseAccountID
		payment.CashBankID = &cashBankID

		grossAmount := decimal.NewFromFloat(payment.Amount)
		bankCreditAmount := decimal.NewFromFloat(calculation.BankBookOutIDR)
		var journalLines []JournalLineRequest

		if payment.PPHAmount > 0 && payment.PPHType != "" {
			pphAmount := decimal.NewFromFloat(payment.PPHAmount)

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
					CreditAmount: bankCreditAmount,
					Description:  fmt.Sprintf("Pembayaran neto via %s - %s", cashBank.Name, payment.Code),
				},
				{
					AccountID:    pphAccountID,
					DebitAmount:  decimal.Zero,
					CreditAmount: pphAmount,
					Description:  fmt.Sprintf("Hutang %s dipotong - %s (%.1f%%)", payment.PPHType, payment.Code, payment.PPHRate),
				},
			}
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
					CreditAmount: bankCreditAmount,
					Description:  fmt.Sprintf("Expense Payment %s - Cash/Bank", payment.Code),
				},
			}
		}
		if fxLine, err := eps.buildRealizedFXJournalLine(tx, calculation.ForexDiff, true, payment.Code); err != nil {
			return err
		} else if fxLine != nil {
			journalLines = append(journalLines, *fxLine)
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
		now := time.Now()
		payment.ApprovalStage = ""
		if payment.FinanceManagerApprovedBy == nil {
			payment.FinanceManagerApprovedBy = &approverUserID
			payment.FinanceManagerApprovedAt = &now
		}
		payment.ApprovedBy = &approverUserID
		payment.ApprovedAt = &now
		payment.RejectedBy = nil
		payment.RejectedAt = nil
		payment.RejectionReason = ""

		if err := tx.Save(&payment).Error; err != nil {
			return fmt.Errorf("failed to update payment after approval: %w", err)
		}

		if err := eps.applyCashBankMovement(tx, cashBank, "VENDOR", calculation.NetCashPaymentIDR, &payment, payment.Date); err != nil {
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

func (eps *EnhancedPaymentServiceWithJournal) calculateExpensePaymentAmounts(cashBank *models.CashBank, grossAmount, pphAmount, exchangeRate float64) (*expensePaymentCalculation, error) {
	if cashBank == nil {
		return nil, fmt.Errorf("cash/bank account is required")
	}
	if grossAmount <= 0 {
		return nil, fmt.Errorf("payment amount must be positive")
	}
	if pphAmount < 0 {
		return nil, fmt.Errorf("pph amount cannot be negative")
	}
	if pphAmount >= grossAmount {
		return nil, fmt.Errorf("pph amount must be smaller than gross expense amount")
	}

	netCashPaymentIDR := grossAmount - pphAmount
	normalizedCurrency := normalizeCashBankCurrency(cashBank.Currency)
	if normalizedCurrency == "" {
		normalizedCurrency = "IDR"
	}

	calculation := &expensePaymentCalculation{
		NormalizedCurrency:  normalizedCurrency,
		AppliedExchangeRate: 1,
		NetCashPaymentIDR:   netCashPaymentIDR,
		NativeCashOut:       netCashPaymentIDR,
		BankBookOutIDR:      netCashPaymentIDR,
	}

	switch normalizedCurrency {
	case "IDR":
		return calculation, nil
	case "USD":
		if exchangeRate <= 0 {
			return nil, fmt.Errorf("exchange rate is required when paying expense from USD cash/bank account")
		}
		nativeCashOut := netCashPaymentIDR / exchangeRate
		bookAmountIDR, _, err := calculatePaymentReductionBookAmount(cashBank, nativeCashOut)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate USD carrying book amount: %w", err)
		}
		calculation.AppliedExchangeRate = exchangeRate
		calculation.NativeCashOut = nativeCashOut
		calculation.BankBookOutIDR = bookAmountIDR
		calculation.ForexDiff = bookAmountIDR - netCashPaymentIDR
		if calculation.ForexDiff > 0.005 {
			calculation.ForexGainLossAmount = calculation.ForexDiff
			calculation.ForexGainLossType = "LOSS"
		} else if calculation.ForexDiff < -0.005 {
			calculation.ForexGainLossAmount = -calculation.ForexDiff
			calculation.ForexGainLossType = "GAIN"
		}
		return calculation, nil
	default:
		return nil, fmt.Errorf("expense payment currently only supports IDR and USD cash/bank accounts")
	}
}

// ApproveStandardPayment processes receivable/payable approval routing.
// Finance manager can route to director; director finalizes routed approvals.
func (eps *EnhancedPaymentServiceWithJournal) ApproveStandardPayment(paymentID uint, approverUserID uint, requireDirectorApproval bool) (*PaymentWithJournalResponse, error) {
	var payment models.Payment
	if err := eps.db.
		Preload("Contact").
		Preload("User").
		Preload("FinanceManagerApprover").
		Preload("DirectorApprover").
		Preload("AccountingReviewer").
		Preload("ExpenseAccount").
		Preload("CashBank").
		Preload("CashBank.Account").
		First(&payment, paymentID).Error; err != nil {
		return nil, fmt.Errorf("payment not found: %w", err)
	}

	if payment.Status != models.PaymentStatusDraft && !isPaymentDirectorApprovalPending(&payment) && !isPaymentAwaitingPostingReview(&payment) {
		return nil, fmt.Errorf("payment is not awaiting approval (current: %s)", payment.Status)
	}
	if payment.PaymentType == models.PaymentTypeExpense || strings.HasPrefix(payment.PaymentType, "TAX_PPN") {
		return nil, fmt.Errorf("only receivable/payable payments can be approved via this endpoint")
	}

	if err := CheckClosedPeriod(eps.db, payment.Date); err != nil {
		return nil, err
	}

	var journalResult *PaymentJournalResult
	response := &PaymentWithJournalResponse{
		Success: false,
	}

	err := eps.db.Transaction(func(tx *gorm.DB) error {
		directorApprovalStep := isPaymentDirectorApprovalPending(&payment)
		if payment.Status == models.PaymentStatusDraft && (payment.RequireDirectorApproval || requireDirectorApproval) {
			return routePaymentToDirector(tx, &payment, approverUserID)
		}
		if payment.Status == models.PaymentStatusDraft {
			return routePaymentToAccounting(tx, &payment, approverUserID, false)
		}
		if directorApprovalStep {
			return routePaymentToAccounting(tx, &payment, approverUserID, true)
		}
		if !isPaymentAwaitingPostingReview(&payment) {
			return fmt.Errorf("payment is not ready for final finance manager review (current: %s)", payment.Status)
		}

		cashBankID := uint(0)
		if payment.CashBankID != nil {
			cashBankID = *payment.CashBankID
		}
		if cashBankID == 0 {
			cashBankID = models.ExtractCashBankDraftRoutingFromReference(payment.Reference)
		}
		if cashBankID == 0 {
			return fmt.Errorf("cannot find cash bank info in draft payment %s; please update and re-submit", payment.Code)
		}
		pendingDraftAllocations := models.HasPendingDraftAllocationsReference(payment.Reference)

		cashBank, err := eps.cashBankRepo.FindByID(cashBankID)
		if err != nil {
			return fmt.Errorf("cash/bank account not found: %w", err)
		}
		if cashBank.AccountID == 0 {
			return fmt.Errorf("cash/bank account %s has no linked GL account", cashBank.Name)
		}

		var journalLines []JournalLineRequest
		var journalDescription string
		if strings.ToUpper(payment.Contact.Type) == "VENDOR" && normalizeCashBankCurrency(payment.Currency) == "USD" && payment.NativeAmount > 0 {
			bookAmountIDR, _, err := calculatePaymentReductionBookAmount(cashBank, payment.NativeAmount)
			if err != nil {
				return fmt.Errorf("failed to calculate USD carrying book amount during approval: %w", err)
			}
			payment.BookAmountIDR = bookAmountIDR
			payment.ForexGainLossAmount = 0
			payment.ForexGainLossType = ""
			forexDiff := payment.BookAmountIDR - payment.Amount
			if forexDiff > 0.005 {
				payment.ForexGainLossAmount = forexDiff
				payment.ForexGainLossType = "LOSS"
			} else if forexDiff < -0.005 {
				payment.ForexGainLossAmount = -forexDiff
				payment.ForexGainLossType = "GAIN"
			}
		}
		payment.Reference = models.StripInternalPaymentReference(payment.Reference)
		payment.CashBankID = &cashBankID
		bankBookAmount := payment.BookAmountIDR
		if bankBookAmount == 0 {
			bankBookAmount = payment.Amount
		}
		receivableSettlementBookAmount := deriveReceivableSettlementBookAmount(&payment)

		if payment.Contact.Type == "CUSTOMER" {
			// Receivable Payment Journal: Debit Cash/Bank, Credit Piutang Usaha
			arAccountID, arErr := eps.resolveReceivableAccountID(tx)
			if arErr != nil {
				return fmt.Errorf("failed to resolve accounts receivable: %w", arErr)
			}
			journalLines = []JournalLineRequest{
				{
					AccountID:    uint64(cashBank.AccountID),
					DebitAmount:  decimal.NewFromFloat(bankBookAmount),
					CreditAmount: decimal.Zero,
					Description:  fmt.Sprintf("Payment received from %s - %s", payment.Contact.Name, payment.Code),
				},
				{
					AccountID:    arAccountID,
					DebitAmount:  decimal.Zero,
					CreditAmount: decimal.NewFromFloat(receivableSettlementBookAmount),
					Description:  fmt.Sprintf("AR reduction - %s", payment.Code),
				},
			}
			if fxLine, err := eps.buildRealizedFXJournalLine(tx, bankBookAmount-receivableSettlementBookAmount, false, payment.Code); err != nil {
				return err
			} else if fxLine != nil {
				journalLines = append(journalLines, *fxLine)
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
					CreditAmount: decimal.NewFromFloat(bankBookAmount),
					Description:  fmt.Sprintf("Payment made to %s - %s", payment.Contact.Name, payment.Code),
				},
			}
			if fxLine, err := eps.buildRealizedFXJournalLine(tx, bankBookAmount-payment.Amount, true, payment.Code); err != nil {
				return err
			} else if fxLine != nil {
				journalLines = append(journalLines, *fxLine)
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
		allocations, err := eps.applyStoredDraftAllocations(tx, &payment, payment.Contact.Type, pendingDraftAllocations)
		if err != nil {
			return fmt.Errorf("failed to apply draft allocations: %w", err)
		}
		payment.Status = models.PaymentStatusCompleted
		now := time.Now()
		payment.ApprovalStage = ""
		if payment.FinanceManagerApprovedBy == nil {
			payment.FinanceManagerApprovedBy = &approverUserID
			payment.FinanceManagerApprovedAt = &now
		}
		payment.ApprovedBy = &approverUserID
		payment.ApprovedAt = &now
		payment.RejectedBy = nil
		payment.RejectedAt = nil
		payment.RejectionReason = ""

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
		response.Allocations = allocations
		response.Success = true
		response.Message = fmt.Sprintf("Payment %s approved successfully", payment.Code)

		log.Printf("✅ Payment APPROVED: ID=%d, Code=%s, JournalID=%d", payment.ID, payment.Code, journalEntryID)

		return nil
	})

	if err != nil {
		return nil, err
	}
	if response.Payment == nil {
		response.Payment = &payment
		response.Contact = &payment.Contact
		response.Success = true
		switch payment.Status {
		case models.PaymentStatusPending:
			response.Message = fmt.Sprintf("Payment %s approved and sent to director", payment.Code)
		case models.PaymentStatusPendingAccounting:
			response.Message = fmt.Sprintf("Payment %s approved and sent to accounting", payment.Code)
		default:
			response.Message = fmt.Sprintf("Payment %s updated", payment.Code)
		}
	}
	return response, nil
}

func (eps *EnhancedPaymentServiceWithJournal) RejectPayment(paymentID uint, approverUserID uint, reason string) (*models.Payment, error) {
	rejectionReason := strings.TrimSpace(reason)
	if rejectionReason == "" {
		return nil, fmt.Errorf("rejection reason is required")
	}

	var payment models.Payment
	err := eps.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&payment, paymentID).Error; err != nil {
			return fmt.Errorf("payment not found: %w", err)
		}

		canReject := payment.Status == models.PaymentStatusDraft ||
			payment.Status == models.PaymentStatusPendingAccounting ||
			payment.Status == models.PaymentStatusPendingFinalReview ||
			isPaymentDirectorApprovalPending(&payment)
		if !canReject {
			return fmt.Errorf("payment is not awaiting approval (current: %s)", payment.Status)
		}

		now := time.Now()
		payment.Status = models.PaymentStatusRejected
		payment.Reference = models.StripInternalPaymentReference(payment.Reference)
		payment.ApprovalStage = ""
		payment.FinanceManagerApprovedBy = nil
		payment.FinanceManagerApprovedAt = nil
		payment.DirectorApprovedBy = nil
		payment.DirectorApprovedAt = nil
		payment.ApprovedBy = nil
		payment.ApprovedAt = nil
		payment.RejectedBy = &approverUserID
		payment.RejectedAt = &now
		payment.RejectionReason = rejectionReason

		if err := tx.Save(&payment).Error; err != nil {
			return fmt.Errorf("failed to reject payment: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := eps.db.
		Preload("Contact").
		Preload("User").
		Preload("FinanceManagerApprover").
		Preload("DirectorApprover").
		Preload("AccountingReviewer").
		Preload("Approver").
		Preload("Rejector").
		Preload("ExpenseAccount").
		Preload("CashBank").
		Preload("CashBank.Account").
		First(&payment, paymentID).Error; err != nil {
		return nil, fmt.Errorf("failed to reload rejected payment: %w", err)
	}

	return &payment, nil
}

// UpdateDraftPayment allows modifying fields before a payment is posted to journal.
func (eps *EnhancedPaymentServiceWithJournal) UpdateDraftPayment(paymentID uint, actorUserID uint, req func(tx *gorm.DB, p *models.Payment) error) (*models.Payment, error) {
	var payment models.Payment
	err := eps.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&payment, paymentID).Error; err != nil {
			return fmt.Errorf("payment not found: %w", err)
		}

		originalStatus := payment.Status
		if originalStatus != models.PaymentStatusDraft &&
			originalStatus != models.PaymentStatusPendingAccounting &&
			originalStatus != models.PaymentStatusPendingFinalReview {
			return fmt.Errorf("only payments before journal posting can be updated (current status: %s)", payment.Status)
		}

		if err := req(tx, &payment); err != nil {
			return fmt.Errorf("failed to apply updates: %w", err)
		}
		if originalStatus == models.PaymentStatusPendingAccounting {
			now := time.Now()
			payment.Status = models.PaymentStatusPendingFinalReview
			payment.ApprovalStage = models.PaymentApprovalStageFinanceManagerFinal
			payment.AccountingReviewedBy = &actorUserID
			payment.AccountingReviewedAt = &now
		}

		if err := tx.Save(&payment).Error; err != nil {
			return fmt.Errorf("failed to save updated payment: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := eps.db.
		Preload("Contact").
		Preload("User").
		Preload("FinanceManagerApprover").
		Preload("DirectorApprover").
		Preload("AccountingReviewer").
		Preload("Approver").
		Preload("Rejector").
		Preload("ExpenseAccount").
		Preload("CashBank").
		Preload("CashBank.Account").
		First(&payment, paymentID).Error; err != nil {
		return nil, fmt.Errorf("failed to reload updated payment: %w", err)
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
	if !req.SaveAsDraft {
		if err := CheckClosedPeriod(eps.db, req.Date); err != nil {
			return nil, err
		}
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
		normalizedCurrency := normalizeCashBankCurrency(cashBank.Currency)

		// Step 3: Auto-detect payment method if needed
		if req.Method == "" {
			req.Method = eps.autoDetectPaymentMethod(contact.Type)
		}

		nativeAmount := req.Amount
		bookAmountIDR := req.Amount
		exchangeRate := 1.0
		receivableSettlementBookIDR := req.Amount
		forexGainLossAmount := 0.0
		forexGainLossType := ""
		if req.ExchangeRate > 0 {
			exchangeRate = req.ExchangeRate
		}
		if strings.ToUpper(contact.Type) == "CUSTOMER" {
			calculatedSettlementBookIDR, err := eps.calculateReceivableSettlementBookAmount(tx, req)
			if err != nil {
				return err
			}
			receivableSettlementBookIDR = calculatedSettlementBookIDR
			if normalizedCurrency == "USD" {
				if req.ExchangeRate <= 0 {
					return fmt.Errorf("exchange rate is required when receiving payment into USD cash/bank account")
				}
				nativeAmount = req.Amount / req.ExchangeRate
				bookAmountIDR = req.Amount
			}
			forexDiff := bookAmountIDR - receivableSettlementBookIDR
			if forexDiff > 0.005 {
				forexGainLossAmount = forexDiff
				forexGainLossType = "GAIN"
			} else if forexDiff < -0.005 {
				forexGainLossAmount = -forexDiff
				forexGainLossType = "LOSS"
			}
		} else if strings.ToUpper(contact.Type) == "VENDOR" && normalizedCurrency == "USD" {
			if req.ExchangeRate <= 0 {
				return fmt.Errorf("exchange rate is required when paying from USD cash/bank account")
			}
			exchangeRate = req.ExchangeRate
			nativeAmount = req.Amount / req.ExchangeRate
			calculatedBookAmountIDR, _, err := calculatePaymentReductionBookAmount(cashBank, nativeAmount)
			if err != nil {
				return fmt.Errorf("failed to calculate USD carrying book amount: %w", err)
			}
			bookAmountIDR = calculatedBookAmountIDR
			forexDiff := bookAmountIDR - req.Amount
			if forexDiff > 0.005 {
				forexGainLossAmount = forexDiff
				forexGainLossType = "LOSS"
			} else if forexDiff < -0.005 {
				forexGainLossAmount = -forexDiff
				forexGainLossType = "GAIN"
			}
		}

		// Step 4: Create payment record
		status := models.PaymentStatusCompleted
		approvalStage := ""
		var financeManagerApprovedBy *uint
		var financeManagerApprovedAt *time.Time
		var approvedBy *uint
		var approvedAt *time.Time
		approvalPending := req.SaveAsDraft
		if req.RequestedByFinanceManager && req.RequireDirectorApproval {
			status = models.PaymentStatusPending
			approvalStage = models.PaymentApprovalStageDirector
			approvalPending = true
			now := time.Now()
			financeManagerApprovedBy = &req.UserID
			financeManagerApprovedAt = &now
		} else if req.SaveAsDraft {
			status = models.PaymentStatusDraft
			approvalStage = models.PaymentApprovalStageFinanceManager
		} else if req.RequestedByFinanceManager {
			now := time.Now()
			financeManagerApprovedBy = &req.UserID
			financeManagerApprovedAt = &now
			approvedBy = &req.UserID
			approvedAt = &now
		}
		cashBankID := req.CashBankID

		payment = &models.Payment{
			ContactID:                req.ContactID,
			UserID:                   req.UserID,
			Date:                     req.Date,
			Amount:                   req.Amount,
			NativeAmount:             nativeAmount,
			Currency:                 normalizedCurrency,
			ExchangeRate:             exchangeRate,
			BookAmountIDR:            bookAmountIDR,
			ForexGainLossAmount:      forexGainLossAmount,
			ForexGainLossType:        forexGainLossType,
			Method:                   req.Method,
			CashBankID:               &cashBankID,
			Reference:                req.Reference,
			Notes:                    req.Notes,
			Status:                   status,
			ApprovalStage:            approvalStage,
			RequireDirectorApproval:  req.RequireDirectorApproval,
			FinanceManagerApprovedBy: financeManagerApprovedBy,
			FinanceManagerApprovedAt: financeManagerApprovedAt,
			ApprovedBy:               approvedBy,
			ApprovedAt:               approvedAt,
		}
		snapshot, snapshotErr := loadCashBankSnapshot(tx, cashBankID)
		if snapshotErr != nil {
			return snapshotErr
		}
		applyCashBankSnapshotToPayment(payment, snapshot)
		hasRequestedAllocations := len(req.InvoiceAllocations) > 0 || len(req.BillAllocations) > 0 || req.TargetInvoiceID != nil || req.TargetBillID != nil
		if approvalPending && hasRequestedAllocations {
			payment.Reference = models.MarkPendingDraftAllocationsReference(payment.Reference)
		} else {
			payment.Reference = models.ClearPendingDraftAllocationsReference(payment.Reference)
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
		if req.AutoCreateJournal && !approvalPending {
			var journalLines []JournalLineRequest
			var journalDescription string
			bankBookAmount := payment.BookAmountIDR
			if bankBookAmount == 0 {
				bankBookAmount = req.Amount
			}

			if contact.Type == "CUSTOMER" {
				// Receivable Payment Journal: Debit Cash/Bank, Credit Piutang Usaha
				arAccountID, arErr := eps.resolveReceivableAccountID(tx)
				if arErr != nil {
					return fmt.Errorf("failed to resolve accounts receivable: %w", arErr)
				}
				journalLines = []JournalLineRequest{
					{
						AccountID:    uint64(cashBank.AccountID),
						DebitAmount:  decimal.NewFromFloat(bankBookAmount),
						CreditAmount: decimal.Zero,
						Description:  fmt.Sprintf("Payment received from %s - %s", contact.Name, payment.Code),
					},
					{
						AccountID:    arAccountID,
						DebitAmount:  decimal.Zero,
						CreditAmount: decimal.NewFromFloat(receivableSettlementBookIDR),
						Description:  fmt.Sprintf("AR reduction - %s", payment.Code),
					},
				}
				if fxLine, err := eps.buildRealizedFXJournalLine(tx, bankBookAmount-receivableSettlementBookIDR, false, payment.Code); err != nil {
					return err
				} else if fxLine != nil {
					journalLines = append(journalLines, *fxLine)
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
						CreditAmount: decimal.NewFromFloat(bankBookAmount),
						Description:  fmt.Sprintf("Payment made to %s - %s", contact.Name, payment.Code),
					},
				}
				if fxLine, err := eps.buildRealizedFXJournalLine(tx, bankBookAmount-req.Amount, true, payment.Code); err != nil {
					return err
				} else if fxLine != nil {
					journalLines = append(journalLines, *fxLine)
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
		allocations, err := eps.createPaymentAllocations(tx, payment, req, contact.Type, !approvalPending)
		if err != nil {
			return fmt.Errorf("failed to create allocations: %w", err)
		}

		// Step 7: Update payment status
		// Already handled during creation if SaveAsDraft is true
		if !approvalPending {
			payment.Status = models.PaymentStatusCompleted
			if err := tx.Save(payment).Error; err != nil {
				return fmt.Errorf("failed to finalize payment: %w", err)
			}
		}

		// Step 8: Reflect movement in Cash & Bank module (keeps UI Kas/Bank in sync)
		// IMPORTANT: We only update cash_banks and insert a transaction record if NOT DRAFT.
		// We DO NOT create another journal here to avoid double-posting because the SSOT journal above
		// already debits/credits the GL bank account.
		if !approvalPending {
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
	if err := eps.db.
		Preload("Contact").
		Preload("User").
		Preload("FinanceManagerApprover").
		Preload("DirectorApprover").
		Preload("AccountingReviewer").
		Preload("Approver").
		Preload("Rejector").
		Preload("ExpenseAccount").
		Preload("CashBank").
		Preload("CashBank.Account").
		First(&payment, paymentID).Error; err != nil {
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
func (eps *EnhancedPaymentServiceWithJournal) ReversePayment(paymentID uint, reason string, reversalDate time.Time, userID uint) (*PaymentWithJournalResponse, error) {
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

		if reversalDate.IsZero() {
			reversalDate = time.Now()
		}

		// Guard: reversal must land in an open accounting period
		if err := CheckClosedPeriod(eps.db, reversalDate); err != nil {
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
						EntryDate:   reversalDate,
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

		var allocations []models.PaymentAllocation
		if err := tx.Where("payment_id = ?", payment.ID).Find(&allocations).Error; err == nil {
			if err := eps.reversePaymentAllocationEffects(tx, allocations); err != nil {
				return err
			}
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
			guard := NewCashBankCurrencyGuard(eps.db)
			if err := guard.EnforceAfterTransaction(tx, originalCBTx.CashBankID); err != nil {
				return fmt.Errorf("failed to sync cash/bank state before reversal: %w", err)
			}
			var cashBank models.CashBank
			if err := tx.First(&cashBank, originalCBTx.CashBankID).Error; err != nil {
				log.Printf("⚠️ Warning: Could not find cash/bank account %d for reversal: %v", originalCBTx.CashBankID, err)
			} else {
				// The original movement amount is stored on the CashBankTransaction.
				// To reverse, we simply negate it.
				reverseMovement := -originalCBTx.Amount
				reverseBookAmountIDR := -originalCBTx.BookAmountIDR
				refType := "PAYMENT_REVERSAL"

				newBalance := cashBank.Balance + reverseMovement
				newBookBalanceIDR := cashBank.BookBalanceIDR + reverseBookAmountIDR
				if !cashBank.IsForeignCurrency() {
					newBookBalanceIDR = newBalance
				}
				cashBank.Balance = newBalance
				if cashBank.IsForeignCurrency() {
					cashBank.BookBalanceIDR = newBookBalanceIDR
					if originalCBTx.ExchangeRate > 0 {
						cashBank.LastExchangeRate = originalCBTx.ExchangeRate
					}
				}
				if err := tx.Save(&cashBank).Error; err != nil {
					return fmt.Errorf("failed to reverse cash/bank balance: %w", err)
				}

				// ✅ CRITICAL: Sync linked COA account balance after reversal
				if cashBank.AccountID > 0 {
					if err := tx.Exec("UPDATE accounts SET balance = ?, updated_at = NOW() WHERE id = ? AND deleted_at IS NULL",
						cashBank.EffectiveBookBalanceIDR(), cashBank.AccountID).Error; err != nil {
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
					Currency:        originalCBTx.Currency,
					ExchangeRate:    originalCBTx.ExchangeRate,
					BookAmountIDR:   reverseBookAmountIDR,
					BalanceAfter:    newBalance,
					TransactionDate: reversalDate,
					Notes:           fmt.Sprintf("%s %s - Reversal: %s", refType, payment.Code, reason),
				}
				if err := tx.Create(txRow).Error; err != nil {
					return fmt.Errorf("failed to create reversal cashbank transaction: %w", err)
				}

				if err := guard.EnforceAfterTransaction(tx, cashBank.ID); err != nil {
					return fmt.Errorf("failed to enforce cash/bank invariant after reversal: %w", err)
				}

				var syncedCashBank models.CashBank
				if err := tx.First(&syncedCashBank, cashBank.ID).Error; err != nil {
					return fmt.Errorf("failed to reload cash/bank after reversal: %w", err)
				}
				if math.Abs(txRow.BalanceAfter-syncedCashBank.Balance) > 0.01 {
					if err := tx.Model(txRow).Update("balance_after", syncedCashBank.Balance).Error; err != nil {
						return fmt.Errorf("failed to align reversal transaction balance_after: %w", err)
					}
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
	guard := NewCashBankCurrencyGuard(eps.db)
	if err := guard.EnforceAfterTransaction(tx, cashBank.ID); err != nil {
		return fmt.Errorf("failed to sync cash/bank state before movement: %w", err)
	}
	if err := tx.First(cashBank, cashBank.ID).Error; err != nil {
		return fmt.Errorf("reload cash bank before movement: %w", err)
	}
	normalizedCurrency := normalizeCashBankCurrency(cashBank.Currency)
	nativeDelta := amount
	bookDeltaIDR := amount
	exchangeRate := 1.0
	refType := "PAYMENT"
	if strings.ToUpper(contactType) == "CUSTOMER" {
		refType = "PAYMENT_RECEIVED"
	} else if strings.ToUpper(contactType) == "VENDOR" {
		nativeDelta = -amount
		bookDeltaIDR = -amount
		refType = "PAYMENT_MADE"
	}

	if payment != nil {
		if payment.NativeAmount > 0 {
			nativeDelta = payment.NativeAmount
			if strings.ToUpper(contactType) == "VENDOR" {
				nativeDelta = -payment.NativeAmount
			}
		}
		if payment.BookAmountIDR > 0 {
			bookDeltaIDR = payment.BookAmountIDR
			if strings.ToUpper(contactType) == "VENDOR" {
				bookDeltaIDR = -payment.BookAmountIDR
			}
		}
		if payment.ExchangeRate > 0 {
			exchangeRate = payment.ExchangeRate
		}
	}

	if nativeDelta < 0 && cashBank.Balance < math.Abs(nativeDelta) {
		return fmt.Errorf("insufficient balance. Available: %.6f, Required: %.6f", cashBank.Balance, math.Abs(nativeDelta))
	}

	newBalance := cashBank.Balance + nativeDelta
	if newBalance < 0 {
		return fmt.Errorf("cash/bank movement would result in negative native balance")
	}
	newBookBalanceIDR := cashBank.BookBalanceIDR + bookDeltaIDR
	if !cashBank.IsForeignCurrency() {
		newBookBalanceIDR = newBalance
	}
	if newBookBalanceIDR < 0 {
		return fmt.Errorf("cash/bank movement would result in negative book balance")
	}

	cashBank.Balance = newBalance
	if cashBank.IsForeignCurrency() {
		cashBank.BookBalanceIDR = newBookBalanceIDR
		if exchangeRate > 0 {
			cashBank.LastExchangeRate = exchangeRate
		}
	}
	if err := tx.Save(cashBank).Error; err != nil {
		return fmt.Errorf("save cash bank: %w", err)
	}

	// ✅ CRITICAL: Sync linked COA account balance to maintain COA <> CashBank consistency
	if cashBank.AccountID > 0 {
		if err := tx.Exec("UPDATE accounts SET balance = ?, updated_at = NOW() WHERE id = ? AND deleted_at IS NULL",
			cashBank.EffectiveBookBalanceIDR(), cashBank.AccountID).Error; err != nil {
			log.Printf("⚠️ Warning: Failed to sync COA balance for GL account %d: %v", cashBank.AccountID, err)
		} else {
			log.Printf("🔄 Synced COA balance for GL account %d to %.2f", cashBank.AccountID, cashBank.EffectiveBookBalanceIDR())
		}
	}

	// Insert transaction row for Cash&Bank UI/history
	txRow := &models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   refType,
		ReferenceID:     payment.ID,
		Amount:          nativeDelta,
		Currency:        normalizedCurrency,
		ExchangeRate:    exchangeRate,
		BookAmountIDR:   bookDeltaIDR,
		BalanceAfter:    newBalance,
		TransactionDate: date,
		Notes:           fmt.Sprintf("%s %s", refType, payment.Code),
	}
	if err := tx.Create(txRow).Error; err != nil {
		return fmt.Errorf("create cashbank transaction: %w", err)
	}

	return guard.EnforceAfterTransaction(tx, cashBank.ID)
}

func (eps *EnhancedPaymentServiceWithJournal) buildRealizedFXJournalLine(tx *gorm.DB, forexDiff float64, isPayable bool, paymentCode string) (*JournalLineRequest, error) {
	if math.Abs(forexDiff) <= 0.005 {
		return nil, nil
	}

	if isPayable {
		if forexDiff > 0.005 {
			accountID, err := eps.resolveForexAccountID(tx, false)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve forex loss account: %w", err)
			}
			return &JournalLineRequest{
				AccountID:    accountID,
				DebitAmount:  decimal.NewFromFloat(forexDiff),
				CreditAmount: decimal.Zero,
				Description:  fmt.Sprintf("Rugi selisih kurs terealisasi - %s", paymentCode),
			}, nil
		}

		accountID, err := eps.resolveForexAccountID(tx, true)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve forex gain account: %w", err)
		}
		return &JournalLineRequest{
			AccountID:    accountID,
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.NewFromFloat(-forexDiff),
			Description:  fmt.Sprintf("Laba selisih kurs terealisasi - %s", paymentCode),
		}, nil
	}

	if forexDiff > 0.005 {
		accountID, err := eps.resolveForexAccountID(tx, true)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve forex gain account: %w", err)
		}
		return &JournalLineRequest{
			AccountID:    accountID,
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.NewFromFloat(forexDiff),
			Description:  fmt.Sprintf("Laba selisih kurs terealisasi - %s", paymentCode),
		}, nil
	}

	accountID, err := eps.resolveForexAccountID(tx, false)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve forex loss account: %w", err)
	}
	return &JournalLineRequest{
		AccountID:    accountID,
		DebitAmount:  decimal.NewFromFloat(-forexDiff),
		CreditAmount: decimal.Zero,
		Description:  fmt.Sprintf("Rugi selisih kurs terealisasi - %s", paymentCode),
	}, nil
}

func (eps *EnhancedPaymentServiceWithJournal) resolveForexAccountID(tx *gorm.DB, isGain bool) (uint64, error) {
	type candidate struct {
		Code        string
		NamePattern string
	}

	var candidates []candidate
	if isGain {
		candidates = []candidate{
			{Code: "7103", NamePattern: "LABA SELISIH KURS"},
			{Code: "7102", NamePattern: "PENDAPATAN LAIN"},
		}
	} else {
		candidates = []candidate{
			{Code: "8103", NamePattern: "RUGI SELISIH KURS"},
			{Code: "8102", NamePattern: "BEBAN LAIN"},
		}
	}

	for _, candidate := range candidates {
		var account models.Account
		if err := tx.Where("code = ?", candidate.Code).First(&account).Error; err == nil {
			return uint64(account.ID), nil
		}
		if err := tx.Where("UPPER(name) LIKE ?", "%"+candidate.NamePattern+"%").Order("id ASC").First(&account).Error; err == nil {
			return uint64(account.ID), nil
		}
	}

	if isGain {
		return 0, fmt.Errorf("forex gain account not found")
	}
	return 0, fmt.Errorf("forex loss account not found")
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

func (eps *EnhancedPaymentServiceWithJournal) calculateReceivableSettlementBookAmount(tx *gorm.DB, req *PaymentWithJournalRequest) (float64, error) {
	totalSettlementBookIDR := 0.0
	remainingAmount := req.Amount
	hasAllocation := false

	applySaleSettlement := func(sale *models.Sale, requestedAmount float64) error {
		if sale == nil || requestedAmount <= 0 || remainingAmount <= 0 {
			return nil
		}

		allocatedAmount := requestedAmount
		if allocatedAmount > remainingAmount {
			allocatedAmount = remainingAmount
		}
		if allocatedAmount > sale.OutstandingAmount {
			allocatedAmount = sale.OutstandingAmount
		}
		if allocatedAmount <= 0 {
			return nil
		}

		settlementBookIDR, _, err := calculateSaleReceivableSettlementBookAmount(sale, allocatedAmount, req.ExchangeRate)
		if err != nil {
			return fmt.Errorf("failed to calculate receivable settlement for sale %d: %w", sale.ID, err)
		}

		totalSettlementBookIDR += settlementBookIDR
		remainingAmount -= allocatedAmount
		hasAllocation = true
		return nil
	}

	if len(req.InvoiceAllocations) > 0 {
		for _, alloc := range req.InvoiceAllocations {
			if alloc.InvoiceID == nil {
				continue
			}

			var sale models.Sale
			if err := tx.First(&sale, *alloc.InvoiceID).Error; err != nil {
				return 0, fmt.Errorf("invoice %d not found: %w", *alloc.InvoiceID, err)
			}
			if err := applySaleSettlement(&sale, alloc.Amount); err != nil {
				return 0, err
			}
		}
	} else if req.TargetInvoiceID != nil {
		var sale models.Sale
		if err := tx.First(&sale, *req.TargetInvoiceID).Error; err != nil {
			return 0, fmt.Errorf("invoice %d not found: %w", *req.TargetInvoiceID, err)
		}
		if err := applySaleSettlement(&sale, req.Amount); err != nil {
			return 0, err
		}
	}

	if !hasAllocation {
		return req.Amount, nil
	}
	if remainingAmount > 0 {
		totalSettlementBookIDR += remainingAmount
	}

	return totalSettlementBookIDR, nil
}

func (eps *EnhancedPaymentServiceWithJournal) applyStoredDraftAllocations(
	tx *gorm.DB,
	payment *models.Payment,
	contactType string,
	applyOutstanding bool,
) ([]models.PaymentAllocation, error) {
	var allocations []models.PaymentAllocation
	if err := tx.Where("payment_id = ?", payment.ID).Find(&allocations).Error; err != nil {
		return nil, fmt.Errorf("failed to load payment allocations: %w", err)
	}

	if !applyOutstanding {
		return allocations, nil
	}

	for i := range allocations {
		allocation := &allocations[i]

		switch strings.ToUpper(strings.TrimSpace(contactType)) {
		case "CUSTOMER":
			if allocation.InvoiceID == nil || *allocation.InvoiceID == 0 {
				continue
			}

			var sale models.Sale
			if err := tx.First(&sale, *allocation.InvoiceID).Error; err != nil {
				return nil, fmt.Errorf("invoice %d not found: %w", *allocation.InvoiceID, err)
			}
			if sale.CustomerID != payment.ContactID {
				return nil, fmt.Errorf("invoice %d does not belong to this customer", sale.ID)
			}
			if err := eps.statusValidator.ValidatePaymentAllocation(sale.Status, sale.ID); err != nil {
				return nil, err
			}

			appliedAmount := allocation.AllocatedAmount
			if appliedAmount > sale.OutstandingAmount {
				appliedAmount = sale.OutstandingAmount
			}
			if appliedAmount < 0 {
				appliedAmount = 0
			}
			if appliedAmount != allocation.AllocatedAmount {
				allocation.AllocatedAmount = appliedAmount
				if err := tx.Save(allocation).Error; err != nil {
					return nil, fmt.Errorf("failed to normalize invoice allocation: %w", err)
				}
			}
			if appliedAmount == 0 {
				continue
			}
			if err := eps.updateSaleOutstanding(tx, sale.ID, appliedAmount); err != nil {
				return nil, fmt.Errorf("failed to apply invoice allocation: %w", err)
			}

		case "VENDOR":
			if allocation.BillID == nil || *allocation.BillID == 0 {
				continue
			}

			var purchase models.Purchase
			if err := tx.First(&purchase, *allocation.BillID).Error; err != nil {
				return nil, fmt.Errorf("bill %d not found: %w", *allocation.BillID, err)
			}
			if purchase.VendorID != payment.ContactID {
				return nil, fmt.Errorf("bill %d does not belong to this vendor", purchase.ID)
			}

			appliedAmount := allocation.AllocatedAmount
			if appliedAmount > purchase.OutstandingAmount {
				appliedAmount = purchase.OutstandingAmount
			}
			if appliedAmount < 0 {
				appliedAmount = 0
			}
			if appliedAmount != allocation.AllocatedAmount {
				allocation.AllocatedAmount = appliedAmount
				if err := tx.Save(allocation).Error; err != nil {
					return nil, fmt.Errorf("failed to normalize bill allocation: %w", err)
				}
			}
			if appliedAmount == 0 {
				continue
			}
			if err := eps.updatePurchaseOutstanding(tx, purchase.ID, appliedAmount); err != nil {
				return nil, fmt.Errorf("failed to apply bill allocation: %w", err)
			}
		}
	}

	return allocations, nil
}

func (eps *EnhancedPaymentServiceWithJournal) SyncDraftStandardPaymentAllocations(
	tx *gorm.DB,
	payment *models.Payment,
	req *PaymentWithJournalRequest,
) error {
	if tx == nil || payment == nil || req == nil {
		return fmt.Errorf("draft allocation sync requires transaction, payment, and request")
	}

	hasAllocationPayload := req.InvoiceAllocations != nil || req.BillAllocations != nil || req.TargetInvoiceID != nil || req.TargetBillID != nil
	if !hasAllocationPayload {
		return nil
	}

	var contact models.Contact
	if err := tx.Select("id", "type").First(&contact, payment.ContactID).Error; err != nil {
		return fmt.Errorf("failed to load payment contact: %w", err)
	}

	var existingAllocations []models.PaymentAllocation
	if err := tx.Where("payment_id = ?", payment.ID).Find(&existingAllocations).Error; err != nil {
		return fmt.Errorf("failed to load existing draft allocations: %w", err)
	}

	if len(existingAllocations) > 0 && !models.HasPendingDraftAllocationsReference(payment.Reference) {
		if err := eps.reversePaymentAllocationEffects(tx, existingAllocations); err != nil {
			return err
		}
	}

	if err := tx.Where("payment_id = ?", payment.ID).Delete(&models.PaymentAllocation{}).Error; err != nil {
		return fmt.Errorf("failed to replace draft allocations: %w", err)
	}

	allocations, err := eps.createPaymentAllocations(tx, payment, req, contact.Type, false)
	if err != nil {
		return err
	}

	if len(allocations) > 0 {
		payment.Reference = models.MarkPendingDraftAllocationsReference(payment.Reference)
	} else {
		payment.Reference = models.ClearPendingDraftAllocationsReference(payment.Reference)
	}

	return nil
}

func (eps *EnhancedPaymentServiceWithJournal) reversePaymentAllocationEffects(
	tx *gorm.DB,
	allocations []models.PaymentAllocation,
) error {
	for _, allocation := range allocations {
		if allocation.InvoiceID != nil && *allocation.InvoiceID > 0 {
			var sale models.Sale
			if err := tx.First(&sale, *allocation.InvoiceID).Error; err != nil {
				return fmt.Errorf("failed to load sale %d during payment reversal: %w", *allocation.InvoiceID, err)
			}

			sale.PaidAmount -= allocation.AllocatedAmount
			if sale.PaidAmount < 0 {
				sale.PaidAmount = 0
			}
			sale.OutstandingAmount += allocation.AllocatedAmount
			if sale.OutstandingAmount < 0 {
				sale.OutstandingAmount = 0
			}
			if sale.OutstandingAmount > 0 && sale.Status == models.SaleStatusPaid {
				sale.Status = models.SaleStatusInvoiced
			}

			if err := tx.Save(&sale).Error; err != nil {
				return fmt.Errorf("failed to reverse sale outstanding for sale %d: %w", sale.ID, err)
			}
			if err := syncSalePaymentScheduleProgress(tx, &sale); err != nil {
				return fmt.Errorf("failed to sync sale payment schedules during payment reversal: %w", err)
			}
			continue
		}

		if allocation.BillID != nil && *allocation.BillID > 0 {
			var purchase models.Purchase
			if err := tx.First(&purchase, *allocation.BillID).Error; err != nil {
				return fmt.Errorf("failed to load purchase %d during payment reversal: %w", *allocation.BillID, err)
			}

			purchase.PaidAmount -= allocation.AllocatedAmount
			if purchase.PaidAmount < 0 {
				purchase.PaidAmount = 0
			}
			purchase.OutstandingAmount += allocation.AllocatedAmount
			if purchase.OutstandingAmount < 0 {
				purchase.OutstandingAmount = 0
			}

			switch {
			case purchase.PaidAmount <= 0.01:
				purchase.MatchingStatus = models.PurchaseMatchingPending
			case purchase.OutstandingAmount <= 0.01:
				purchase.MatchingStatus = models.PurchaseMatchingMatched
				purchase.OutstandingAmount = 0
			default:
				purchase.MatchingStatus = models.PurchaseMatchingPartial
			}

			if err := tx.Save(&purchase).Error; err != nil {
				return fmt.Errorf("failed to reverse purchase outstanding for purchase %d: %w", purchase.ID, err)
			}
		}
	}

	return nil
}

// createPaymentAllocations creates payment allocations to invoices/bills
func (eps *EnhancedPaymentServiceWithJournal) createPaymentAllocations(
	tx *gorm.DB,
	payment *models.Payment,
	req *PaymentWithJournalRequest,
	contactType string,
	applyOutstanding bool,
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
				if applyOutstanding {
					if err := eps.updateSaleOutstanding(tx, *alloc.InvoiceID, allocatedAmount); err != nil {
						log.Printf("⚠️ Warning: Failed to update sale outstanding: %v", err)
					}
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

			if applyOutstanding {
				if err := eps.updateSaleOutstanding(tx, *req.TargetInvoiceID, payment.Amount); err != nil {
					log.Printf("⚠️ Warning: Failed to update sale outstanding: %v", err)
				}
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
				if applyOutstanding {
					if err := eps.updatePurchaseOutstanding(tx, *alloc.BillID, allocatedAmount); err != nil {
						log.Printf("❌ CRITICAL: Failed to update purchase outstanding: %v", err)
						return nil, fmt.Errorf("failed to update purchase outstanding: %w", err)
					}
					log.Printf("✅ Purchase outstanding updated for bill %d", *alloc.BillID)
				}

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

			if applyOutstanding {
				if err := eps.updatePurchaseOutstanding(tx, *req.TargetBillID, payment.Amount); err != nil {
					log.Printf("❌ CRITICAL: Failed to update purchase outstanding: %v", err)
					return nil, fmt.Errorf("failed to update purchase outstanding: %w", err)
				}
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

	if err := tx.Save(&sale).Error; err != nil {
		return err
	}

	return syncSalePaymentScheduleProgress(tx, &sale)
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
