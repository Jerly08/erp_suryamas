package models

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type CashBank struct {
	ID                      uint           `json:"id" gorm:"primaryKey"`
	Code                    string         `json:"code" gorm:"unique;not null;size:20"`
	Name                    string         `json:"name" gorm:"not null;size:100"`
	Type                    string         `json:"type" gorm:"not null;size:20;check:type IN ('CASH','BANK')"`
	AccountID               uint           `json:"account_id" gorm:"index"`
	BankName                string         `json:"bank_name" gorm:"size:100"`
	AccountNo               string         `json:"account_no" gorm:"size:50"`
	AccountHolderName       string         `json:"account_holder_name" gorm:"size:100"`
	Branch                  string         `json:"branch" gorm:"size:100"`
	Currency                string         `json:"currency" gorm:"size:5;default:'IDR';not null"`
	Balance                 float64        `json:"balance" gorm:"type:decimal(15,2);default:0;not null"`
	BookBalanceIDR          float64        `json:"book_balance_idr" gorm:"type:decimal(20,2);default:0;not null"`
	LastExchangeRate        float64        `json:"last_exchange_rate" gorm:"type:decimal(15,6);default:1;not null"`
	MinBalance              float64        `json:"min_balance" gorm:"type:decimal(15,2);default:0"`
	MaxBalance              float64        `json:"max_balance" gorm:"type:decimal(15,2);default:0"`
	DailyLimit              float64        `json:"daily_limit" gorm:"type:decimal(15,2);default:0"`
	MonthlyLimit            float64        `json:"monthly_limit" gorm:"type:decimal(15,2);default:0"`
	IsActive                bool           `json:"is_active" gorm:"default:true;not null"`
	IsRestricted            bool           `json:"is_restricted" gorm:"default:false;not null"`
	IsHoldingAccount        bool           `json:"is_holding_account" gorm:"default:false;not null"`
	AllowDirectTransactions bool           `json:"allow_direct_transactions" gorm:"default:true;not null"`
	FrozenAmount            float64        `json:"frozen_amount" gorm:"type:decimal(15,2);default:0;not null"`
	FrozenReason            string         `json:"frozen_reason" gorm:"size:255"`
	FrozenAt                *time.Time     `json:"frozen_at"`
	FrozenBy                uint           `json:"frozen_by"`
	UserID                  uint           `json:"user_id" gorm:"index"`
	Description             string         `json:"description" gorm:"type:text"`
	CreatedAt               time.Time      `json:"created_at"`
	UpdatedAt               time.Time      `json:"updated_at"`
	DeletedAt               gorm.DeletedAt `json:"-" gorm:"index"`

	Account      Account               `json:"account" gorm:"foreignKey:AccountID"`
	Transactions []CashBankTransaction `json:"transactions" gorm:"foreignKey:CashBankID"`
}

// AvailableBalance returns the balance minus frozen amount.
func (cb *CashBank) AvailableBalance() float64 {
	available := cb.Balance - cb.FrozenAmount
	if available < 0 {
		return 0
	}
	return available
}

// IsForeignCurrency reports whether the account is maintained in non-IDR currency.
func (cb *CashBank) IsForeignCurrency() bool {
	currency := strings.ToUpper(strings.TrimSpace(cb.Currency))
	return currency != "" && currency != "IDR"
}

// EffectiveBookBalanceIDR returns the IDR value that should feed the balance sheet.
func (cb *CashBank) EffectiveBookBalanceIDR() float64 {
	if !cb.IsForeignCurrency() {
		return cb.Balance
	}
	return cb.BookBalanceIDR
}

// EffectiveExchangeRate returns a safe exchange rate for display and book calculations.
func (cb *CashBank) EffectiveExchangeRate() float64 {
	if cb.LastExchangeRate > 0 {
		return cb.LastExchangeRate
	}
	return 1
}

type CashBankTransaction struct {
	ID              uint           `json:"id" gorm:"primaryKey"`
	CashBankID      uint           `json:"cash_bank_id" gorm:"not null;index"`
	ReferenceType   string         `json:"reference_type" gorm:"size:50"`
	ReferenceID     uint           `json:"reference_id" gorm:"index"`
	Amount          float64        `json:"amount" gorm:"type:decimal(20,2);default:0"`
	Currency        string         `json:"currency" gorm:"size:5;default:'IDR';not null"`
	ExchangeRate    float64        `json:"exchange_rate" gorm:"type:decimal(15,6);default:1;not null"`
	BookAmountIDR   float64        `json:"book_amount_idr" gorm:"type:decimal(20,2);default:0;not null"`
	BalanceAfter    float64        `json:"balance_after" gorm:"type:decimal(20,2);default:0"`
	TransactionDate time.Time      `json:"transaction_date"`
	Notes           string         `json:"notes" gorm:"type:text"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`

	CashBank CashBank `json:"cash_bank" gorm:"foreignKey:CashBankID"`
}

const (
	CashBankTypeCash = "CASH"
	CashBankTypeBank = "BANK"
)

const (
	FrozenActionFreeze   = "FREEZE"
	FrozenActionUnfreeze = "UNFREEZE"
)

// CashBankFrozenHistory tracks freeze/unfreeze history for audit trail.
type CashBankFrozenHistory struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	CashBankID     uint      `json:"cash_bank_id" gorm:"not null;index"`
	Action         string    `json:"action" gorm:"not null;size:20"`
	Amount         float64   `json:"amount" gorm:"type:decimal(15,2);not null"`
	PreviousFrozen float64   `json:"previous_frozen" gorm:"type:decimal(15,2);not null"`
	NewFrozen      float64   `json:"new_frozen" gorm:"type:decimal(15,2);not null"`
	Reason         string    `json:"reason" gorm:"size:255"`
	UserID         uint      `json:"user_id" gorm:"index"`
	CreatedAt      time.Time `json:"created_at"`

	CashBank CashBank `json:"cash_bank" gorm:"foreignKey:CashBankID"`
}
