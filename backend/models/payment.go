package models

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Payment struct {
	ID                  uint      `json:"id" gorm:"primaryKey"`
	Code                string    `json:"code" gorm:"unique;not null;size:30"`
	ContactID           uint      `json:"contact_id" gorm:"not null;index"`
	UserID              uint      `json:"user_id" gorm:"not null;index"`
	Date                time.Time `json:"date"`
	Amount              float64   `json:"amount" gorm:"type:decimal(15,2);default:0"`
	NativeAmount        float64   `json:"native_amount" gorm:"type:decimal(20,6);default:0"`
	Currency            string    `json:"currency" gorm:"size:5;default:'IDR';not null"`
	ExchangeRate        float64   `json:"exchange_rate" gorm:"type:decimal(15,6);default:1;not null"`
	BookAmountIDR       float64   `json:"book_amount_idr" gorm:"type:decimal(20,2);default:0;not null"`
	ForexGainLossAmount float64   `json:"forex_gain_loss_amount" gorm:"type:decimal(20,2);default:0"`
	ForexGainLossType   string    `json:"forex_gain_loss_type" gorm:"size:10"`
	// LegacyNonInvoiceEligible is calculated by the payment service. It is never
	// persisted and only exposes the narrowly-scoped compatibility action to Accounting.
	LegacyNonInvoiceEligible bool       `json:"legacy_non_invoice_eligible" gorm:"-"`
	Method                   string     `json:"method" gorm:"size:20"` // CASH, BANK_TRANSFER, CHECK, etc.
	Reference                string     `json:"reference" gorm:"size:50"`
	Status                   string     `json:"status" gorm:"size:20"`             // DRAFT, COMPLETED, FAILED, REVERSED (PENDING is legacy transitional data)
	PaymentType              string     `json:"payment_type" gorm:"size:30;index"` // REGULAR, TAX_PPN, TAX_PPN_INPUT, TAX_PPN_OUTPUT
	ExpenseAccountID         *uint      `json:"expense_account_id" gorm:"index"`
	CashBankID               *uint      `json:"cash_bank_id" gorm:"index"`
	CashBankCode             string     `json:"cash_bank_code" gorm:"size:50"`
	CashBankName             string     `json:"cash_bank_name" gorm:"size:120"`
	CashBankCurrency         string     `json:"cash_bank_currency" gorm:"size:5"`
	CashBankGLAccountID      *uint      `json:"cash_bank_gl_account_id" gorm:"index"`
	CashBankGLAccountCode    string     `json:"cash_bank_gl_account_code" gorm:"size:20"`
	CashBankGLAccountName    string     `json:"cash_bank_gl_account_name" gorm:"size:120"`
	ApprovalStage            string     `json:"approval_stage" gorm:"size:30;index"`
	RequireDirectorApproval  bool       `json:"require_director_approval" gorm:"not null;default:false"`
	FinanceManagerApprovedBy *uint      `json:"finance_manager_approved_by" gorm:"index"`
	FinanceManagerApprovedAt *time.Time `json:"finance_manager_approved_at"`
	DirectorApprovedBy       *uint      `json:"director_approved_by" gorm:"index"`
	DirectorApprovedAt       *time.Time `json:"director_approved_at"`
	AccountingReviewedBy     *uint      `json:"accounting_reviewed_by" gorm:"index"`
	AccountingReviewedAt     *time.Time `json:"accounting_reviewed_at"`
	ApprovedBy               *uint      `json:"approved_by" gorm:"index"`
	ApprovedAt               *time.Time `json:"approved_at"`
	RejectedBy               *uint      `json:"rejected_by" gorm:"index"`
	RejectedAt               *time.Time `json:"rejected_at"`
	RejectionReason          string     `json:"rejection_reason" gorm:"type:text"`
	Notes                    string     `json:"notes" gorm:"type:text"`
	JournalEntryID           *uint      `json:"journal_entry_id" gorm:"index"` // Link to SSOT journal entry
	// Settlement fields keep the commercial invoice immutable when the actual
	// bank receipt differs from the invoice balance.  A positive difference is
	// normally recorded as customer advance; a small approved difference may be
	// closed as rounding through the Accounting -> Finance Manager workflow.
	SettlementTreatment  string  `json:"settlement_treatment" gorm:"size:30;default:'OPEN_BALANCE';index"`
	SettlementDifference float64 `json:"settlement_difference" gorm:"type:decimal(15,2);default:0"`
	SettlementReason     string  `json:"settlement_reason" gorm:"type:text"`
	RoundingThreshold    float64 `json:"rounding_threshold" gorm:"type:decimal(15,2);default:0"`
	// PPh withholding tax fields (optional, only for expense payments with PPh)
	PPHType   string         `json:"pph_type" gorm:"size:20"`                        // PPH_21, PPH_23, PPH_4_2, PPH_22
	PPHRate   float64        `json:"pph_rate" gorm:"type:decimal(5,2);default:0"`    // e.g. 5.0 for 5%
	PPHAmount float64        `json:"pph_amount" gorm:"type:decimal(15,2);default:0"` // calculated tax amount
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	Contact                Contact           `json:"contact" gorm:"foreignKey:ContactID"`
	User                   User              `json:"user" gorm:"foreignKey:UserID"`
	FinanceManagerApprover *User             `json:"finance_manager_approver,omitempty" gorm:"foreignKey:FinanceManagerApprovedBy"`
	DirectorApprover       *User             `json:"director_approver,omitempty" gorm:"foreignKey:DirectorApprovedBy"`
	AccountingReviewer     *User             `json:"accounting_reviewer,omitempty" gorm:"foreignKey:AccountingReviewedBy"`
	Approver               *User             `json:"approver,omitempty" gorm:"foreignKey:ApprovedBy"`
	Rejector               *User             `json:"rejector,omitempty" gorm:"foreignKey:RejectedBy"`
	ExpenseAccount         *Account          `json:"expense_account,omitempty" gorm:"foreignKey:ExpenseAccountID"`
	CashBank               *CashBank         `json:"cash_bank,omitempty" gorm:"foreignKey:CashBankID"`
	CustomerAdvances       []CustomerAdvance `json:"customer_advances,omitempty" gorm:"foreignKey:PaymentID"`
}

// PaymentAllocation represents payment allocation to invoices or bills
type PaymentAllocation struct {
	ID                    uint      `json:"id" gorm:"primaryKey"`
	PaymentID             uint64    `json:"payment_id" gorm:"not null;index"`
	InvoiceID             *uint     `json:"invoice_id" gorm:"index"`
	BillID                *uint     `json:"bill_id" gorm:"index"`
	PurchasePaymentTermID *uint     `json:"purchase_payment_term_id" gorm:"index"`
	AllocatedAmount       float64   `json:"allocated_amount" gorm:"type:decimal(15,2);default:0"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`

	// Relations
	Payment             Payment              `json:"payment" gorm:"foreignKey:PaymentID"`
	Sale                *Sale                `json:"sale,omitempty" gorm:"foreignKey:InvoiceID"`
	PurchasePaymentTerm *PurchasePaymentTerm `json:"purchase_payment_term,omitempty" gorm:"foreignKey:PurchasePaymentTermID"`
}

// Payment Status Constants
const (
	PaymentStatusDraft              = "DRAFT"              // Waiting finance manager approval
	PaymentStatusPending            = "PENDING"            // Waiting director approval
	PaymentStatusPendingAccounting  = "PENDING_ACCOUNTING" // Waiting accounting to review/select COA
	PaymentStatusPendingFinalReview = "PENDING_FINAL_REVIEW"
	PaymentStatusCompleted          = "COMPLETED"
	PaymentStatusRejected           = "REJECTED"
	PaymentStatusFailed             = "FAILED"
	PaymentStatusReversed           = "REVERSED"
)

const (
	PaymentApprovalStageFinanceManager      = "FINANCE_MANAGER"
	PaymentApprovalStageAccounting          = "ACCOUNTING"
	PaymentApprovalStageFinanceManagerFinal = "FINANCE_MANAGER_FINAL"
	PaymentApprovalStageDirector            = "DIRECTOR"
)

// Payment Method Constants
const (
	PaymentMethodCash          = "CASH"
	PaymentMethodBankTransfer  = "BANK_TRANSFER"
	PaymentMethodCheck         = "CHECK"
	PaymentMethodCreditCard    = "CREDIT_CARD"
	PaymentMethodDebitCard     = "DEBIT_CARD"
	PaymentMethodDigitalWallet = "DIGITAL_WALLET"
)

const pendingDraftAllocationReferencePrefix = "__ALLOC_PENDING__"
const expenseLiabilityOverpaymentReferencePrefix = "__EXP_LIAB_OVERPAY__"

// Payment Type Constants
const (
	PaymentTypeRegular      = "REGULAR"
	PaymentTypeTaxPPN       = "TAX_PPN"
	PaymentTypeTaxPPNInput  = "TAX_PPN_INPUT"
	PaymentTypeTaxPPNOutput = "TAX_PPN_OUTPUT"
	// Custom type for direct expense payments created from COA (no specific vendor/customer)
	PaymentTypeExpense = "EXPENSE"
)

// Customer payment settlement treatments.  OPEN_BALANCE leaves a short
// receipt as receivable, CUSTOMER_ADVANCE is the default treatment for an
// excess receipt, and ROUNDING requires Accounting review plus final Finance
// Manager posting.
const (
	PaymentSettlementOpenBalance     = "OPEN_BALANCE"
	PaymentSettlementCustomerAdvance = "CUSTOMER_ADVANCE"
	PaymentSettlementRounding        = "ROUNDING"
)

// IsTaxPayment returns true if this payment is a PPN/tax payment
// Tax payments have their own dedicated journal entries and should NOT
// be processed as regular purchase payments (which debit UTANG USAHA)
func (p *Payment) IsTaxPayment() bool {
	return p.PaymentType == PaymentTypeTaxPPN ||
		p.PaymentType == PaymentTypeTaxPPNInput ||
		p.PaymentType == PaymentTypeTaxPPNOutput
}

// GetPayeeName returns the payee name for display (e.g., in PDF)
// For PPN tax payments, it returns "Negara" instead of contact name
func (p *Payment) GetPayeeName() string {
	// For PPN tax payments, payee is always "Negara" (Government)
	if p.PaymentType == PaymentTypeTaxPPN ||
		p.PaymentType == PaymentTypeTaxPPNInput ||
		p.PaymentType == PaymentTypeTaxPPNOutput {
		return "Negara"
	}

	// For regular payments, return contact name if available
	if p.Contact.Name != "" {
		return p.Contact.Name
	}

	return "Unknown"
}

func ExtractExpenseDraftRoutingFromReference(reference string) (uint, uint) {
	reference = ClearExpenseLiabilityOverpaymentReference(reference)

	var expenseAccountID, cashBankID uint
	n, _ := fmt.Sscanf(reference, "__EXP_ACC:%d,CB:%d__", &expenseAccountID, &cashBankID)
	if n != 2 {
		return 0, 0
	}

	return expenseAccountID, cashBankID
}

func ExtractCashBankDraftRoutingFromReference(reference string) uint {
	reference = ClearExpenseLiabilityOverpaymentReference(reference)

	var cashBankID uint
	n, _ := fmt.Sscanf(reference, "__CB:%d__", &cashBankID)
	if n != 1 {
		return 0
	}

	return cashBankID
}

func HasPendingDraftAllocationsReference(reference string) bool {
	return strings.HasPrefix(strings.TrimSpace(reference), pendingDraftAllocationReferencePrefix)
}

func MarkPendingDraftAllocationsReference(reference string) string {
	cleanedReference := ClearPendingDraftAllocationsReference(reference)
	if cleanedReference == "" {
		return pendingDraftAllocationReferencePrefix
	}

	return pendingDraftAllocationReferencePrefix + " " + cleanedReference
}

func ClearPendingDraftAllocationsReference(reference string) string {
	trimmedReference := strings.TrimSpace(reference)
	if !strings.HasPrefix(trimmedReference, pendingDraftAllocationReferencePrefix) {
		return trimmedReference
	}

	return strings.TrimSpace(strings.TrimPrefix(trimmedReference, pendingDraftAllocationReferencePrefix))
}

func HasExpenseLiabilityOverpaymentReference(reference string) bool {
	return strings.HasPrefix(strings.TrimSpace(reference), expenseLiabilityOverpaymentReferencePrefix)
}

func MarkExpenseLiabilityOverpaymentReference(reference string) string {
	cleanedReference := ClearExpenseLiabilityOverpaymentReference(reference)
	if cleanedReference == "" {
		return expenseLiabilityOverpaymentReferencePrefix
	}

	return expenseLiabilityOverpaymentReferencePrefix + " " + cleanedReference
}

func ClearExpenseLiabilityOverpaymentReference(reference string) string {
	trimmedReference := strings.TrimSpace(reference)
	if !strings.HasPrefix(trimmedReference, expenseLiabilityOverpaymentReferencePrefix) {
		return trimmedReference
	}

	return strings.TrimSpace(strings.TrimPrefix(trimmedReference, expenseLiabilityOverpaymentReferencePrefix))
}

func StripInternalPaymentReference(reference string) string {
	reference = ClearPendingDraftAllocationsReference(reference)
	reference = ClearExpenseLiabilityOverpaymentReference(reference)
	reference = ClearPendingDraftAllocationsReference(reference)

	expenseAccountID, cashBankID := ExtractExpenseDraftRoutingFromReference(reference)
	if expenseAccountID != 0 && cashBankID != 0 {
		return strings.TrimSpace(strings.Replace(reference, fmt.Sprintf("__EXP_ACC:%d,CB:%d__", expenseAccountID, cashBankID), "", 1))
	}

	cashBankID = ExtractCashBankDraftRoutingFromReference(reference)
	if cashBankID != 0 {
		return strings.TrimSpace(strings.Replace(reference, fmt.Sprintf("__CB:%d__", cashBankID), "", 1))
	}

	return reference
}
