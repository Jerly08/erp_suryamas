package models

import (
	"time"
	"gorm.io/gorm"
)

type CashBank struct {
	ID            uint           `json:"id" gorm:"primaryKey"`
	Code          string         `json:"code" gorm:"unique;not null;size:20"`
	Name          string         `json:"name" gorm:"not null;size:100"`
	Type          string         `json:"type" gorm:"not null;size:20;check:type IN ('CASH','BANK')"` // CASH, BANK
	AccountID          uint           `json:"account_id" gorm:"index"`
	BankName           string         `json:"bank_name" gorm:"size:100"`
	AccountNo          string         `json:"account_no" gorm:"size:50"`
	AccountHolderName  string         `json:"account_holder_name" gorm:"size:100"`
	Branch             string         `json:"branch" gorm:"size:100"`
	Currency           string         `json:"currency" gorm:"size:5;default:'IDR';not null"`
	Balance       float64        `json:"balance" gorm:"type:decimal(15,2);default:0;not null"`
	MinBalance    float64        `json:"min_balance" gorm:"type:decimal(15,2);default:0"`
	MaxBalance    float64        `json:"max_balance" gorm:"type:decimal(15,2);default:0"`
	DailyLimit    float64        `json:"daily_limit" gorm:"type:decimal(15,2);default:0"`
	MonthlyLimit  float64        `json:"monthly_limit" gorm:"type:decimal(15,2);default:0"`
	IsActive      bool           `json:"is_active" gorm:"default:true;not null"`
	IsRestricted  bool           `json:"is_restricted" gorm:"default:false;not null"`
	FrozenAmount  float64        `json:"frozen_amount" gorm:"type:decimal(15,2);default:0;not null"`
	FrozenReason  string         `json:"frozen_reason" gorm:"size:255"`
	FrozenAt      *time.Time     `json:"frozen_at"`
	FrozenBy      uint           `json:"frozen_by"`
	UserID        uint           `json:"user_id" gorm:"index"`
	Description   string         `json:"description" gorm:"type:text"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	Account      Account                `json:"account" gorm:"foreignKey:AccountID"`
	Transactions []CashBankTransaction  `json:"transactions" gorm:"foreignKey:CashBankID"`
}

// AvailableBalance returns the balance minus frozen amount
func (cb *CashBank) AvailableBalance() float64 {
	available := cb.Balance - cb.FrozenAmount
	if available < 0 {
		return 0
	}
	return available
}

type CashBankTransaction struct {
	ID              uint           `json:"id" gorm:"primaryKey"`
	CashBankID      uint           `json:"cash_bank_id" gorm:"not null;index"`
	ReferenceType   string         `json:"reference_type" gorm:"size:50"` // PAYMENT, TRANSFER
	ReferenceID     uint           `json:"reference_id" gorm:"index"`
	Amount          float64        `json:"amount" gorm:"type:decimal(20,2);default:0"`
	BalanceAfter    float64        `json:"balance_after" gorm:"type:decimal(20,2);default:0"`
	TransactionDate time.Time      `json:"transaction_date"`
	Notes           string         `json:"notes" gorm:"type:text"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	CashBank CashBank `json:"cash_bank" gorm:"foreignKey:CashBankID"`
}

// CashBank Types
const (
	CashBankTypeCash = "CASH"
	CashBankTypeBank = "BANK"
)

// Frozen Action Types
const (
	FrozenActionFreeze   = "FREEZE"
	FrozenActionUnfreeze = "UNFREEZE"
)

// CashBankFrozenHistory tracks freeze/unfreeze history for audit trail
type CashBankFrozenHistory struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	CashBankID      uint      `json:"cash_bank_id" gorm:"not null;index"`
	Action          string    `json:"action" gorm:"not null;size:20"` // FREEZE or UNFREEZE
	Amount          float64   `json:"amount" gorm:"type:decimal(15,2);not null"`
	PreviousFrozen  float64   `json:"previous_frozen" gorm:"type:decimal(15,2);not null"`
	NewFrozen       float64   `json:"new_frozen" gorm:"type:decimal(15,2);not null"`
	Reason          string    `json:"reason" gorm:"size:255"`
	UserID          uint      `json:"user_id" gorm:"index"`
	CreatedAt       time.Time `json:"created_at"`

	// Relations
	CashBank CashBank `json:"cash_bank" gorm:"foreignKey:CashBankID"`
}
