package models

import (
	"time"

	"gorm.io/gorm"
)

const (
	PurchasePrePostingRevisionStatusPendingFinanceManager = "PENDING_FINANCE_MANAGER"
	PurchasePrePostingRevisionStatusOpenForFinance        = "OPEN_FOR_FINANCE"
	PurchasePrePostingRevisionStatusRejected              = "REJECTED"
	PurchasePrePostingRevisionStatusResubmitted           = "RESUBMITTED"
)

// PurchasePrePostingRevision records a controlled return of a purchase to
// Finance Staff before it has any accounting posting. It is intentionally
// separate from posted-transaction corrections: this flow never changes a
// journal, cash/bank movement, stock, payment, or project actual cost.
type PurchasePrePostingRevision struct {
	ID         uint   `json:"id" gorm:"primaryKey"`
	PurchaseID uint   `json:"purchase_id" gorm:"not null;index"`
	Status     string `json:"status" gorm:"not null;size:40;index"`

	RevisionTypes string `json:"revision_types" gorm:"not null;type:text"`
	Reason        string `json:"reason" gorm:"not null;type:text"`

	OriginalApprovalRequestID    *uint `json:"original_approval_request_id,omitempty" gorm:"index"`
	ResubmittedApprovalRequestID *uint `json:"resubmitted_approval_request_id,omitempty" gorm:"index"`

	RequestedBy uint      `json:"requested_by" gorm:"not null;index"`
	RequestedAt time.Time `json:"requested_at" gorm:"not null;index"`

	FinanceManagerDecisionNotes string     `json:"finance_manager_decision_notes" gorm:"type:text"`
	FinanceManagerDecisionBy    *uint      `json:"finance_manager_decision_by,omitempty" gorm:"index"`
	FinanceManagerDecisionAt    *time.Time `json:"finance_manager_decision_at,omitempty" gorm:"index"`

	OpenedForRevisionBy *uint      `json:"opened_for_revision_by,omitempty" gorm:"index"`
	OpenedForRevisionAt *time.Time `json:"opened_for_revision_at,omitempty" gorm:"index"`
	ResubmittedBy       *uint      `json:"resubmitted_by,omitempty" gorm:"index"`
	ResubmittedAt       *time.Time `json:"resubmitted_at,omitempty" gorm:"index"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	Purchase          *Purchase `json:"purchase,omitempty" gorm:"foreignKey:PurchaseID"`
	Requester         *User     `json:"requester,omitempty" gorm:"foreignKey:RequestedBy"`
	FinanceManager    *User     `json:"finance_manager,omitempty" gorm:"foreignKey:FinanceManagerDecisionBy"`
	OpenedBy          *User     `json:"opened_by,omitempty" gorm:"foreignKey:OpenedForRevisionBy"`
	ResubmittedByUser *User     `json:"resubmitted_by_user,omitempty" gorm:"foreignKey:ResubmittedBy"`
}

type PurchasePrePostingRevisionCreateRequest struct {
	RevisionTypes []string `json:"revision_types" binding:"required,min=1"`
	Reason        string   `json:"reason" binding:"required"`
}

type PurchasePrePostingRevisionDecisionRequest struct {
	Approve bool   `json:"approve"`
	Notes   string `json:"notes"`
}
