package models

import (
	"time"

	"gorm.io/gorm"
)

const (
	PurchaseCOARevisionStatusPendingFinanceManager = "PENDING_FINANCE_MANAGER"
	PurchaseCOARevisionStatusApproved              = "APPROVED"
	PurchaseCOARevisionStatusRejected              = "REJECTED"
)

// PurchaseCOARevision preserves the request and decision trail when Accounting
// needs to revise a PO's COA after its initial input but before final posting.
// It never changes an existing posted journal.
type PurchaseCOARevision struct {
	ID         uint   `json:"id" gorm:"primaryKey"`
	PurchaseID uint   `json:"purchase_id" gorm:"not null;index"`
	Status     string `json:"status" gorm:"not null;size:40;index"`
	Reason     string `json:"reason" gorm:"not null;type:text"`

	COASnapshot string `json:"coa_snapshot" gorm:"not null;type:text"`

	RequestedBy uint      `json:"requested_by" gorm:"not null;index"`
	RequestedAt time.Time `json:"requested_at" gorm:"not null;index"`

	FinanceManagerDecisionNotes string     `json:"finance_manager_decision_notes" gorm:"type:text"`
	FinanceManagerDecisionBy    *uint      `json:"finance_manager_decision_by,omitempty" gorm:"index"`
	FinanceManagerDecisionAt    *time.Time `json:"finance_manager_decision_at,omitempty" gorm:"index"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	Purchase       *Purchase `json:"purchase,omitempty" gorm:"foreignKey:PurchaseID"`
	Requester      *User     `json:"requester,omitempty" gorm:"foreignKey:RequestedBy"`
	FinanceManager *User     `json:"finance_manager,omitempty" gorm:"foreignKey:FinanceManagerDecisionBy"`
}

type PurchaseCOARevisionCreateRequest struct {
	Reason string `json:"reason" binding:"required"`
}

type PurchaseCOARevisionDecisionRequest struct {
	Approve bool   `json:"approve"`
	Notes   string `json:"notes"`
}
