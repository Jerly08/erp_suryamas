package models

import (
	"time"

	"gorm.io/gorm"
)

const (
	CustomerAdvanceStatusOpen      = "OPEN"
	CustomerAdvanceStatusApplied   = "APPLIED"
	CustomerAdvanceStatusCancelled = "CANCELLED"
)

// CustomerAdvance records the liability created by an excess customer receipt.
// It is deliberately separate from the invoice so the invoice amount, tax, and
// sales history remain unchanged.
type CustomerAdvance struct {
	ID              uint           `json:"id" gorm:"primaryKey"`
	CustomerID      uint           `json:"customer_id" gorm:"not null;index"`
	PaymentID       uint           `json:"payment_id" gorm:"not null;uniqueIndex"`
	SaleID          *uint          `json:"sale_id,omitempty" gorm:"index"`
	JournalEntryID  *uint64        `json:"journal_entry_id,omitempty" gorm:"index"`
	OriginalAmount  float64        `json:"original_amount" gorm:"type:decimal(15,2);not null"`
	RemainingAmount float64        `json:"remaining_amount" gorm:"type:decimal(15,2);not null"`
	Currency        string         `json:"currency" gorm:"size:5;default:'IDR';not null"`
	ExchangeRate    float64        `json:"exchange_rate" gorm:"type:decimal(15,6);default:1;not null"`
	Status          string         `json:"status" gorm:"size:20;default:'OPEN';index"`
	Notes           string         `json:"notes" gorm:"type:text"`
	CreatedBy       uint           `json:"created_by" gorm:"not null;index"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`

	Customer Contact `json:"customer" gorm:"foreignKey:CustomerID"`
	Payment  Payment `json:"payment" gorm:"foreignKey:PaymentID"`
	Sale     *Sale   `json:"sale,omitempty" gorm:"foreignKey:SaleID"`
	Creator  User    `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
}

func (CustomerAdvance) TableName() string {
	return "customer_advances"
}
