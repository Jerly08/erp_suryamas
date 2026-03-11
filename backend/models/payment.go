package models

import (
	"time"

	"gorm.io/gorm"
)

type Payment struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	Code           string    `json:"code" gorm:"unique;not null;size:30"`
	ContactID      uint      `json:"contact_id" gorm:"not null;index"`
	UserID         uint      `json:"user_id" gorm:"not null;index"`
	Date           time.Time `json:"date"`
	Amount         float64   `json:"amount" gorm:"type:decimal(15,2);default:0"`
	Method         string    `json:"method" gorm:"size:20"` // CASH, BANK_TRANSFER, CHECK, etc.
	Reference      string    `json:"reference" gorm:"size:50"`
	Status         string    `json:"status" gorm:"size:20"`             // PENDING, COMPLETED, FAILED, REVERSED
	PaymentType    string    `json:"payment_type" gorm:"size:30;index"` // REGULAR, TAX_PPN, TAX_PPN_INPUT, TAX_PPN_OUTPUT
	Notes          string    `json:"notes" gorm:"type:text"`
	JournalEntryID *uint     `json:"journal_entry_id" gorm:"index"` // Link to SSOT journal entry
	// PPh withholding tax fields (optional, only for expense payments with PPh)
	PPHType   string         `json:"pph_type" gorm:"size:20"`                        // PPH_21, PPH_23, PPH_4_2, PPH_22
	PPHRate   float64        `json:"pph_rate" gorm:"type:decimal(5,2);default:0"`    // e.g. 5.0 for 5%
	PPHAmount float64        `json:"pph_amount" gorm:"type:decimal(15,2);default:0"` // calculated tax amount
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	Contact Contact `json:"contact" gorm:"foreignKey:ContactID"`
	User    User    `json:"user" gorm:"foreignKey:UserID"`
}

// PaymentAllocation represents payment allocation to invoices or bills
type PaymentAllocation struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	PaymentID       uint64    `json:"payment_id" gorm:"not null;index"`
	InvoiceID       *uint     `json:"invoice_id" gorm:"index"`
	BillID          *uint     `json:"bill_id" gorm:"index"`
	AllocatedAmount float64   `json:"allocated_amount" gorm:"type:decimal(15,2);default:0"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	// Relations
	Payment Payment `json:"payment" gorm:"foreignKey:PaymentID"`
	Sale    *Sale   `json:"sale,omitempty" gorm:"foreignKey:InvoiceID"`
}

// Payment Status Constants
const (
	PaymentStatusDraft     = "DRAFT"     // Submitted by employee, waiting finance approval
	PaymentStatusPending   = "PENDING"
	PaymentStatusCompleted = "COMPLETED"
	PaymentStatusFailed    = "FAILED"
	PaymentStatusReversed  = "REVERSED"
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

// Payment Type Constants
const (
	PaymentTypeRegular      = "REGULAR"
	PaymentTypeTaxPPN       = "TAX_PPN"
	PaymentTypeTaxPPNInput  = "TAX_PPN_INPUT"
	PaymentTypeTaxPPNOutput = "TAX_PPN_OUTPUT"
	// Custom type for direct expense payments created from COA (no specific vendor/customer)
	PaymentTypeExpense = "EXPENSE"
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
