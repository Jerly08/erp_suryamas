package models

import (
	"time"
)

type PrepaidExpense struct {
	ID                uint              `json:"id" gorm:"primaryKey"`
	PurchaseID        *uint             `json:"purchase_id" gorm:"index"`
	PurchaseItemID    *uint             `json:"purchase_item_id" gorm:"index"`
	Name              string            `json:"name" gorm:"type:varchar(255);not null"`
	Description       string            `json:"description" gorm:"type:text"`
	TotalAmount       float64           `json:"total_amount" gorm:"type:decimal(15,2);not null"`
	AmortizationMonth int               `json:"amortization_month" gorm:"not null"` // Durasi dalam bulan
	MonthlyAmount     float64           `json:"monthly_amount" gorm:"type:decimal(15,2);not null"`
	StartDate         time.Time         `json:"start_date" gorm:"not null"`
	EndDate           time.Time         `json:"end_date" gorm:"not null"`
	Status            string            `json:"status" gorm:"type:varchar(20);default:'ACTIVE'"` // ACTIVE, COMPLETED, TERMINATED
	PrepaidAccountID  uint              `json:"prepaid_account_id" gorm:"not null"` // Akun Aset Lancar
	ExpenseAccountID  uint              `json:"expense_account_id" gorm:"not null"` // Akun Beban tujuan

	// Relasi
	Schedules []PrepaidSchedule `json:"schedules,omitempty" gorm:"foreignKey:PrepaidID"`
	Purchase  *Purchase         `json:"purchase,omitempty" gorm:"foreignKey:PurchaseID"`

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

type PrepaidSchedule struct {
	ID                 uint      `json:"id" gorm:"primaryKey"`
	PrepaidID          uint      `json:"prepaid_id" gorm:"not null;index"`
	MonthNumber        int       `json:"month_number" gorm:"not null"`
	ScheduledDate      time.Time `json:"scheduled_date" gorm:"not null"`
	Amount             float64   `json:"amount" gorm:"type:decimal(15,2);not null"`
	Status             string    `json:"status" gorm:"type:varchar(20);default:'PENDING'"` // PENDING, POSTED, CANCELLED
	JournalEntryID     *uint     `json:"journal_entry_id" gorm:"index"`

	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}
