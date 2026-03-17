package models

import (
	"database/sql/driver"
	"encoding/json"
	"gorm.io/gorm"
	"time"
)

// AccountingPeriod represents a closed accounting period with snapshots
type AccountingPeriod struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	StartDate   time.Time  `json:"start_date" gorm:"not null;index"`
	EndDate     time.Time  `json:"end_date" gorm:"not null;index"`
	Description string     `json:"description" gorm:"type:text"`
	IsClosed    bool       `json:"is_closed" gorm:"default:false"`
	IsLocked    bool       `json:"is_locked" gorm:"default:false"` // Hard lock - cannot be reopened easily
	ClosedBy    *uint      `json:"closed_by" gorm:"index"`
	ClosedAt    *time.Time `json:"closed_at"`

	// Closing summary
	TotalRevenue     float64 `json:"total_revenue" gorm:"type:decimal(20,2);default:0"`
	TotalExpense     float64 `json:"total_expense" gorm:"type:decimal(20,2);default:0"`
	NetIncome        float64 `json:"net_income" gorm:"type:decimal(20,2);default:0"`
	ClosingJournalID *uint   `json:"closing_journal_id" gorm:"index"` // Reference to closing journal entry

	// Archive snapshots (stored as JSONB in PostgreSQL)
	BalanceSheetSnapshot *JSONMap   `json:"balance_sheet_snapshot,omitempty" gorm:"type:jsonb"`
	ProfitLossSnapshot   *JSONMap   `json:"profit_loss_snapshot,omitempty" gorm:"type:jsonb"`
	FinancialMetrics     *JSONMap   `json:"financial_metrics,omitempty" gorm:"type:jsonb"`
	SnapshotGeneratedAt  *time.Time `json:"snapshot_generated_at,omitempty"`

	// Period categorization
	PeriodType       string `json:"period_type" gorm:"type:varchar(20);default:'CUSTOM'"` // MONTHLY, QUARTERLY, SEMESTER, ANNUAL, CUSTOM
	FiscalYear       *int   `json:"fiscal_year,omitempty" gorm:"index"`
	AccountCount     int    `json:"account_count" gorm:"default:0"`
	TransactionCount int    `json:"transaction_count" gorm:"default:0"`

	ReopenedBy         *uint      `json:"reopened_by,omitempty" gorm:"index"`
	ReopenedAt         *time.Time `json:"reopened_at,omitempty"`
	ReopenReason       string     `json:"reopen_reason" gorm:"type:text"`
	ReopeningJournalID *uint      `json:"reopening_journal_id,omitempty" gorm:"index"`
	ReopenCount        int        `json:"reopen_count" gorm:"default:0"`

	Notes     string         `json:"notes" gorm:"type:text"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	ClosedByUser   *User         `json:"closed_by_user,omitempty" gorm:"foreignKey:ClosedBy"`
	ReopenedByUser *User         `json:"reopened_by_user,omitempty" gorm:"foreignKey:ReopenedBy"`
	ClosingJournal *JournalEntry `json:"closing_journal,omitempty" gorm:"foreignKey:ClosingJournalID"`
}

// JSONMap is a type for storing JSON data in JSONB columns
type JSONMap map[string]interface{}

// Scan implements the sql.Scanner interface for JSONMap
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// Value implements the driver.Valuer interface for JSONMap
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// TableName specifies the table name for AccountingPeriod
func (AccountingPeriod) TableName() string {
	return "accounting_periods"
}

// PeriodClosingPreview contains preview data for period closing
type PeriodClosingPreview struct {
	StartDate          time.Time              `json:"start_date"`
	EndDate            time.Time              `json:"end_date"`
	TotalRevenue       float64                `json:"total_revenue"`
	TotalExpense       float64                `json:"total_expense"`
	NetIncome          float64                `json:"net_income"`
	RetainedEarningsID uint                   `json:"retained_earnings_id"`
	RevenueAccounts    []PeriodAccountBalance `json:"revenue_accounts"`
	ExpenseAccounts    []PeriodAccountBalance `json:"expense_accounts"`
	ClosingEntries     []ClosingEntryPreview  `json:"closing_entries"`
	CanClose           bool                   `json:"can_close"`
	ValidationMessages []string               `json:"validation_messages"`

	// Additional info
	TransactionCount int64      `json:"transaction_count"`
	LastClosingDate  *time.Time `json:"last_closing_date,omitempty"`
	PeriodDays       int        `json:"period_days"`
}

// PeriodAccountBalance represents account balance for period closing
type PeriodAccountBalance struct {
	ID      uint    `json:"id"`
	Code    string  `json:"code"`
	Name    string  `json:"name"`
	Balance float64 `json:"balance"`
	Type    string  `json:"type"`
}

// ClosingEntryPreview represents a preview of closing journal entry
type ClosingEntryPreview struct {
	Description   string  `json:"description"`
	DebitAccount  string  `json:"debit_account"`
	CreditAccount string  `json:"credit_account"`
	Amount        float64 `json:"amount"`
}

// PeriodClosingRequest represents the request to close a period
type PeriodClosingRequest struct {
	StartDate   string `json:"start_date" binding:"required"` // YYYY-MM-DD
	EndDate     string `json:"end_date" binding:"required"`   // YYYY-MM-DD
	Description string `json:"description"`
	Notes       string `json:"notes"`
}

// LastClosingInfo contains info about the last closed period
type LastClosingInfo struct {
	HasPreviousClosing bool       `json:"has_previous_closing"`
	LastClosingDate    *time.Time `json:"last_closing_date"`
	NextStartDate      *time.Time `json:"next_start_date"`
	PeriodStartDate    *time.Time `json:"period_start_date"` // Earliest transaction date if no previous closing
}

// PeriodReopenRequest represents the request to reopen a closed period
type PeriodReopenRequest struct {
	StartDate string `json:"start_date" binding:"required"` // YYYY-MM-DD
	EndDate   string `json:"end_date" binding:"required"`   // YYYY-MM-DD
	Reason    string `json:"reason" binding:"required"`     // Reason for reopening
}
