package models

import (
	"time"

	"gorm.io/gorm"
)

// PurchaseRequest represents the header for a purchase request (Tahap 1 pengadaan)
type PurchaseRequest struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	PRNumber    string `json:"pr_number" gorm:"unique;not null;size:50"`
	RequesterID uint   `json:"requester_id" gorm:"not null;index"` // employee who created it
	Department  string `json:"department" gorm:"size:100"`
	ProjectID   *uint  `json:"project_id" gorm:"index"` // Optional tracking cost center

	TotalEstimatedAmount float64 `json:"total_estimated_amount" gorm:"type:decimal(15,2);default:0"`
	Status               string  `json:"status" gorm:"size:30;default:'DRAFT'"` // DRAFT, PENDING_APPROVAL, APPROVED, REJECTED
	CurrentApprovalStage string  `json:"current_approval_stage" gorm:"size:30;default:'FINANCE_STAFF'"`
	Notes                string  `json:"notes" gorm:"type:text"`

	// Approval Tracking
	ApprovedBy               *uint      `json:"approved_by" gorm:"index"`
	ApprovedAt               *time.Time `json:"approved_at"`
	FinanceStaffApprovedBy   *uint      `json:"finance_staff_approved_by" gorm:"index"`
	FinanceStaffApprovedAt   *time.Time `json:"finance_staff_approved_at"`
	FinanceManagerApprovedBy *uint      `json:"finance_manager_approved_by" gorm:"index"`
	FinanceManagerApprovedAt *time.Time `json:"finance_manager_approved_at"`
	RejectionReason          string     `json:"rejection_reason" gorm:"type:text"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	Requester              User                  `json:"requester" gorm:"foreignKey:RequesterID"`
	Project                *Project              `json:"project,omitempty" gorm:"foreignKey:ProjectID"`
	Items                  []PurchaseRequestItem `json:"items" gorm:"foreignKey:PurchaseRequestID"`
	Approver               *User                 `json:"approver,omitempty" gorm:"foreignKey:ApprovedBy"`
	FinanceStaffApprover   *User                 `json:"finance_staff_approver,omitempty" gorm:"foreignKey:FinanceStaffApprovedBy"`
	FinanceManagerApprover *User                 `json:"finance_manager_approver,omitempty" gorm:"foreignKey:FinanceManagerApprovedBy"`
}

// PurchaseRequestItem represents the details of items requested
type PurchaseRequestItem struct {
	ID                     uint       `json:"id" gorm:"primaryKey"`
	PurchaseRequestID      uint       `json:"purchase_request_id" gorm:"not null;index"`
	ProductID              uint       `json:"product_id" gorm:"not null;index"`
	Quantity               int        `json:"quantity" gorm:"not null;default:1"`
	EstimatedPrice         float64    `json:"estimated_price" gorm:"type:decimal(15,2);default:0"` // Referensi / Taksiran harga awal
	TotalEstimatedPrice    float64    `json:"total_estimated_price" gorm:"type:decimal(15,2);default:0"`
	ApprovalStatus         string     `json:"approval_status" gorm:"size:20;default:'PENDING'"`
	FinanceStaffDecision   string     `json:"finance_staff_decision" gorm:"size:20;default:'PENDING'"`
	FinanceManagerDecision string     `json:"finance_manager_decision" gorm:"size:20;default:'PENDING'"`
	PurchaseID             *uint      `json:"purchase_id" gorm:"index"`
	PurchasedAt            *time.Time `json:"purchased_at"`
	Notes                  string     `json:"notes" gorm:"type:text"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	PurchaseRequest PurchaseRequest `json:"purchase_request" gorm:"foreignKey:PurchaseRequestID"`
	Product         Product         `json:"product" gorm:"foreignKey:ProductID"`
	Purchase        *Purchase       `json:"purchase,omitempty" gorm:"foreignKey:PurchaseID"`
}

// PurchaseRequest Status Constants
const (
	PRStatusDraft           = "DRAFT"
	PRStatusPendingApproval = "PENDING_APPROVAL" // Menunggu Finance
	PRStatusApproved        = "APPROVED"
	PRStatusRejected        = "REJECTED"
	PRStatusCompleted       = "COMPLETED" // Sudah terealisasi jadi Purchase/Invoice sepenuhnya
)

// Purchase request approval stages
const (
	PRApprovalStageFinanceStaff   = "FINANCE_STAFF"
	PRApprovalStageFinanceManager = "FINANCE_MANAGER"
	PRApprovalStageFinalized      = "FINALIZED"
)

// Purchase request item decision constants
const (
	PRItemDecisionPending  = "PENDING"
	PRItemDecisionApproved = "APPROVED"
	PRItemDecisionRejected = "REJECTED"
)

// Request DTOs
type PurchaseRequestCreatePayload struct {
	Department string                       `json:"department"`
	ProjectID  *uint                        `json:"project_id"`
	Notes      string                       `json:"notes"`
	Items      []PurchaseRequestItemPayload `json:"items" binding:"required,min=1"`
}

type PurchaseRequestItemPayload struct {
	ProductID      uint    `json:"product_id" binding:"required"`
	Quantity       int     `json:"quantity" binding:"required,min=1"`
	EstimatedPrice float64 `json:"estimated_price" binding:"required,min=0"`
	Notes          string  `json:"notes"`
}

type PurchaseRequestReviewItemPayload struct {
	ItemID   uint   `json:"item_id" binding:"required"`
	Decision string `json:"decision" binding:"required"`
}

type PurchaseRequestReviewPayload struct {
	Items  []PurchaseRequestReviewItemPayload `json:"items" binding:"required,min=1"`
	Reason string                             `json:"reason"`
}
