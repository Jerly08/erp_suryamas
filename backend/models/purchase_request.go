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
	Title       string `json:"title" gorm:"size:255"`
	ProjectID   *uint  `json:"project_id" gorm:"index"` // Optional tracking cost center
	VendorID    *uint  `json:"vendor_id" gorm:"index"`  // Optional preferred/target vendor

	VendorInvoiceReceivedDate *time.Time `json:"vendor_invoice_received_date" gorm:"column:vendor_invoice_received_date"`
	PaymentTermsDays          *int       `json:"payment_terms_days" gorm:"column:payment_terms_days"`
	DueDate                   *time.Time `json:"due_date" gorm:"column:due_date;index"`

	TotalEstimatedAmount float64 `json:"total_estimated_amount" gorm:"type:decimal(15,2);default:0"`
	Status               string  `json:"status" gorm:"size:30;default:'DRAFT'"` // DRAFT, PENDING_APPROVAL, APPROVED, REJECTED
	CurrentApprovalStage string  `json:"current_approval_stage" gorm:"size:30;default:'FINANCE_STAFF'"`
	ApprovalRevision     int     `json:"approval_revision" gorm:"default:1"`
	ReapprovalReason     string  `json:"reapproval_reason" gorm:"size:50"`
	Notes                string  `json:"notes" gorm:"type:text"`

	RevisionOpenedBy    *uint      `json:"revision_opened_by" gorm:"index"`
	RevisionOpenedAt    *time.Time `json:"revision_opened_at"`
	RevisionReason      string     `json:"revision_reason" gorm:"type:text"`
	RevisionSubmittedBy *uint      `json:"revision_submitted_by" gorm:"index"`
	RevisionSubmittedAt *time.Time `json:"revision_submitted_at"`

	RequiresVendorInvoice      bool       `json:"requires_vendor_invoice" gorm:"default:false"`
	VendorInvoiceStatus        string     `json:"vendor_invoice_status" gorm:"size:30;default:'NOT_REQUIRED'"`
	VendorInvoiceRequestedBy   *uint      `json:"vendor_invoice_requested_by" gorm:"index"`
	VendorInvoiceRequestedAt   *time.Time `json:"vendor_invoice_requested_at"`
	VendorInvoiceSubmittedBy   *uint      `json:"vendor_invoice_submitted_by" gorm:"index"`
	VendorInvoiceSubmittedAt   *time.Time `json:"vendor_invoice_submitted_at"`
	VendorInvoiceRevisionNotes string     `json:"vendor_invoice_revision_notes" gorm:"type:text"`
	VendorInvoiceReviewedBy    *uint      `json:"vendor_invoice_reviewed_by" gorm:"index"`
	VendorInvoiceReviewedAt    *time.Time `json:"vendor_invoice_reviewed_at"`
	VendorInvoiceReviewNotes   string     `json:"vendor_invoice_review_notes" gorm:"type:text"`

	// Billing/realisasi has its own post-approval review cycle. It must not
	// reset the original PR approval or affect the linked purchase/journal.
	RequiresRealizationDocument bool       `json:"requires_realization_document" gorm:"default:false"`
	RealizationStatus           string     `json:"realization_status" gorm:"size:40;default:'NOT_REQUIRED'"`
	RealizationRequestedBy      *uint      `json:"realization_requested_by" gorm:"index"`
	RealizationRequestedAt      *time.Time `json:"realization_requested_at"`
	RealizationSubmittedBy      *uint      `json:"realization_submitted_by" gorm:"index"`
	RealizationSubmittedAt      *time.Time `json:"realization_submitted_at"`
	RealizationReviewedBy       *uint      `json:"realization_reviewed_by" gorm:"index"`
	RealizationReviewedAt       *time.Time `json:"realization_reviewed_at"`
	RealizationReviewNotes      string     `json:"realization_review_notes" gorm:"type:text"`
	RealizationReviewCycle      int        `json:"realization_review_cycle" gorm:"default:0"`

	// Approval Tracking
	ApprovedBy                *uint      `json:"approved_by" gorm:"index"`
	ApprovedAt                *time.Time `json:"approved_at"`
	FinanceStaffApprovedBy    *uint      `json:"finance_staff_approved_by" gorm:"index"`
	FinanceStaffApprovedAt    *time.Time `json:"finance_staff_approved_at"`
	FinanceStaffReviewNotes   string     `json:"finance_staff_review_notes" gorm:"type:text"`
	FinanceManagerApprovedBy  *uint      `json:"finance_manager_approved_by" gorm:"index"`
	FinanceManagerApprovedAt  *time.Time `json:"finance_manager_approved_at"`
	FinanceManagerReviewNotes string     `json:"finance_manager_review_notes" gorm:"type:text"`
	DirectorApprovedBy        *uint      `json:"director_approved_by" gorm:"index"`
	DirectorApprovedAt        *time.Time `json:"director_approved_at"`
	RejectionReason           string     `json:"rejection_reason" gorm:"type:text"`
	CancelledBy               *uint      `json:"cancelled_by" gorm:"index"`
	CancelledAt               *time.Time `json:"cancelled_at"`
	CancelledReason           string     `json:"cancelled_reason" gorm:"type:text"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	Requester              User                               `json:"requester" gorm:"foreignKey:RequesterID"`
	Project                *Project                           `json:"project,omitempty" gorm:"foreignKey:ProjectID"`
	Vendor                 *Contact                           `json:"vendor,omitempty" gorm:"foreignKey:VendorID"`
	Items                  []PurchaseRequestItem              `json:"items" gorm:"foreignKey:PurchaseRequestID"`
	Documents              []PurchaseRequestDocument          `json:"documents" gorm:"foreignKey:PurchaseRequestID"`
	RevisionSnapshots      []PurchaseRequestRevisionSnapshot  `json:"revision_snapshots" gorm:"foreignKey:PurchaseRequestID"`
	RealizationReviews     []PurchaseRequestRealizationReview `json:"realization_reviews" gorm:"foreignKey:PurchaseRequestID"`
	Approver               *User                              `json:"approver,omitempty" gorm:"foreignKey:ApprovedBy"`
	FinanceStaffApprover   *User                              `json:"finance_staff_approver,omitempty" gorm:"foreignKey:FinanceStaffApprovedBy"`
	FinanceManagerApprover *User                              `json:"finance_manager_approver,omitempty" gorm:"foreignKey:FinanceManagerApprovedBy"`
	DirectorApprover       *User                              `json:"director_approver,omitempty" gorm:"foreignKey:DirectorApprovedBy"`
	Canceller              *User                              `json:"canceller,omitempty" gorm:"foreignKey:CancelledBy"`
	RevisionOpener         *User                              `json:"revision_opener,omitempty" gorm:"foreignKey:RevisionOpenedBy"`
	RevisionSubmitter      *User                              `json:"revision_submitter,omitempty" gorm:"foreignKey:RevisionSubmittedBy"`
	VendorInvoiceRequester *User                              `json:"vendor_invoice_requester,omitempty" gorm:"foreignKey:VendorInvoiceRequestedBy"`
	VendorInvoiceSubmitter *User                              `json:"vendor_invoice_submitter,omitempty" gorm:"foreignKey:VendorInvoiceSubmittedBy"`
	VendorInvoiceReviewer  *User                              `json:"vendor_invoice_reviewer,omitempty" gorm:"foreignKey:VendorInvoiceReviewedBy"`
	RealizationRequester   *User                              `json:"realization_requester,omitempty" gorm:"foreignKey:RealizationRequestedBy"`
	RealizationSubmitter   *User                              `json:"realization_submitter,omitempty" gorm:"foreignKey:RealizationSubmittedBy"`
	RealizationReviewer    *User                              `json:"realization_reviewer,omitempty" gorm:"foreignKey:RealizationReviewedBy"`
}

// PurchaseRequestItem represents the details of items requested
type PurchaseRequestItem struct {
	ID                     uint       `json:"id" gorm:"primaryKey"`
	PurchaseRequestID      uint       `json:"purchase_request_id" gorm:"not null;index"`
	ProductID              *uint      `json:"product_id,omitempty" gorm:"index"`
	ItemName               string     `json:"item_name" gorm:"not null;size:255;default:''"`
	Quantity               int        `json:"quantity" gorm:"not null;default:1"`
	EstimatedPrice         float64    `json:"estimated_price" gorm:"type:decimal(15,2);default:0"` // Referensi / Taksiran harga awal
	TotalEstimatedPrice    float64    `json:"total_estimated_price" gorm:"type:decimal(15,2);default:0"`
	ApprovalStatus         string     `json:"approval_status" gorm:"size:20;default:'PENDING'"`
	FinanceStaffDecision   string     `json:"finance_staff_decision" gorm:"size:20;default:'PENDING'"`
	FinanceManagerDecision string     `json:"finance_manager_decision" gorm:"size:20;default:'PENDING'"`
	DirectorDecision       string     `json:"director_decision" gorm:"size:20;default:'PENDING'"`
	PurchaseID             *uint      `json:"purchase_id" gorm:"index"`
	PurchasedAt            *time.Time `json:"purchased_at"`
	Notes                  string     `json:"notes" gorm:"type:text"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	PurchaseRequest PurchaseRequest `json:"purchase_request" gorm:"foreignKey:PurchaseRequestID"`
	Product         *Product        `json:"product,omitempty" gorm:"foreignKey:ProductID"`
	Purchase        *Purchase       `json:"purchase,omitempty" gorm:"foreignKey:PurchaseID"`
}

// PurchaseRequestDocument stores supporting attachments uploaded with a PR.
type PurchaseRequestDocument struct {
	ID                uint           `json:"id" gorm:"primaryKey"`
	PurchaseRequestID uint           `json:"purchase_request_id" gorm:"not null;index"`
	DocumentType      string         `json:"document_type" gorm:"size:50;default:'SUPPORTING_DOCUMENT'"`
	FileName          string         `json:"file_name" gorm:"not null;size:255"`
	FilePath          string         `json:"file_path" gorm:"not null;size:500"`
	FileSize          int64          `json:"file_size"`
	MimeType          string         `json:"mime_type" gorm:"size:100"`
	UploadedBy        uint           `json:"uploaded_by" gorm:"not null;index"`
	ApprovalRevision  int            `json:"approval_revision" gorm:"default:1;index"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	PurchaseRequest PurchaseRequest `json:"purchase_request" gorm:"foreignKey:PurchaseRequestID"`
	Uploader        User            `json:"uploader" gorm:"foreignKey:UploadedBy"`
}

type PurchaseRequestRevisionSnapshot struct {
	ID                           uint           `json:"id" gorm:"primaryKey"`
	PurchaseRequestID            uint           `json:"purchase_request_id" gorm:"not null;index"`
	RevisionNumber               int            `json:"revision_number" gorm:"not null;index"`
	ReapprovalReason             string         `json:"reapproval_reason" gorm:"size:50"`
	PreviousStatus               string         `json:"previous_status" gorm:"size:30"`
	PreviousApprovalStage        string         `json:"previous_approval_stage" gorm:"size:30"`
	PreviousTotalEstimatedAmount float64        `json:"previous_total_estimated_amount" gorm:"type:decimal(15,2);default:0"`
	NewTotalEstimatedAmount      *float64       `json:"new_total_estimated_amount" gorm:"type:decimal(15,2)"`
	ItemSnapshot                 string         `json:"item_snapshot" gorm:"type:text"`
	NewItemSnapshot              string         `json:"new_item_snapshot" gorm:"type:text"`
	OpenedBy                     uint           `json:"opened_by" gorm:"not null;index"`
	OpenedAt                     time.Time      `json:"opened_at" gorm:"not null"`
	OpenReason                   string         `json:"open_reason" gorm:"type:text"`
	SubmittedBy                  *uint          `json:"submitted_by" gorm:"index"`
	SubmittedAt                  *time.Time     `json:"submitted_at"`
	SubmittedNotes               string         `json:"submitted_notes" gorm:"type:text"`
	ApprovedBy                   *uint          `json:"approved_by" gorm:"index"`
	ApprovedAt                   *time.Time     `json:"approved_at"`
	FinanceStaffApprovedBy       *uint          `json:"finance_staff_approved_by" gorm:"index"`
	FinanceStaffApprovedAt       *time.Time     `json:"finance_staff_approved_at"`
	FinanceManagerApprovedBy     *uint          `json:"finance_manager_approved_by" gorm:"index"`
	FinanceManagerApprovedAt     *time.Time     `json:"finance_manager_approved_at"`
	DirectorApprovedBy           *uint          `json:"director_approved_by" gorm:"index"`
	DirectorApprovedAt           *time.Time     `json:"director_approved_at"`
	CreatedAt                    time.Time      `json:"created_at"`
	UpdatedAt                    time.Time      `json:"updated_at"`
	DeletedAt                    gorm.DeletedAt `json:"-" gorm:"index"`

	PurchaseRequest PurchaseRequest `json:"-" gorm:"foreignKey:PurchaseRequestID"`
	Opener          User            `json:"opener" gorm:"foreignKey:OpenedBy"`
	Submitter       *User           `json:"submitter,omitempty" gorm:"foreignKey:SubmittedBy"`
}

// PurchaseRequestRealizationReview records every post-approval billing/realisasi
// action independently from the original PR approval history.
type PurchaseRequestRealizationReview struct {
	ID                uint           `json:"id" gorm:"primaryKey"`
	PurchaseRequestID uint           `json:"purchase_request_id" gorm:"not null;index"`
	CycleNumber       int            `json:"cycle_number" gorm:"not null;default:1;index"`
	Stage             string         `json:"stage" gorm:"size:40;not null"`
	Decision          string         `json:"decision" gorm:"size:40;not null"`
	Notes             string         `json:"notes" gorm:"type:text"`
	ActorID           uint           `json:"actor_id" gorm:"not null;index"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `json:"-" gorm:"index"`

	PurchaseRequest PurchaseRequest `json:"-" gorm:"foreignKey:PurchaseRequestID"`
	Actor           User            `json:"actor" gorm:"foreignKey:ActorID"`
}

// PurchaseRequest Status Constants
const (
	PRStatusDraft            = "DRAFT"
	PRStatusPendingApproval  = "PENDING_APPROVAL" // Menunggu Finance
	PRStatusApproved         = "APPROVED"
	PRStatusRejected         = "REJECTED"
	PRStatusCompleted        = "COMPLETED"         // Sudah terealisasi jadi Purchase/Invoice sepenuhnya
	PRStatusCancelled        = "CANCELLED"         // Dibatalkan setelah approved, tanpa menghapus histori
	PRStatusAwaitingRevision = "AWAITING_REVISION" // Menunggu PIC/Requester submit revisi
)

// Purchase request approval stages
const (
	PRApprovalStageFinanceStaff   = "FINANCE_STAFF"
	PRApprovalStageFinanceManager = "FINANCE_MANAGER"
	PRApprovalStageDirector       = "DIRECTOR"
	PRApprovalStageFinalized      = "FINALIZED"
)

// Purchase request item decision constants
const (
	PRItemDecisionPending  = "PENDING"
	PRItemDecisionApproved = "APPROVED"
	PRItemDecisionRejected = "REJECTED"
)

// Purchase request document type constants
const (
	PurchaseRequestDocumentSupporting                = "SUPPORTING_DOCUMENT"
	PurchaseRequestDocumentVendorInvoice             = "VENDOR_INVOICE"
	PurchaseRequestDocumentPostPurchaseVendorInvoice = "POST_PURCHASE_VENDOR_INVOICE"
	PurchaseRequestDocumentRequesterRevision         = "REQUESTER_REVISION_DOCUMENT"
	PurchaseRequestDocumentRealization               = "REALIZATION_DOCUMENT"
)

// Purchase request reapproval reasons
const (
	PRReapprovalReasonAdditionalDocuments = "ADDITIONAL_DOCUMENTS"
	PRReapprovalReasonInvoiceRevision     = "INVOICE_REVISION"
	PRReapprovalReasonRequesterRevision   = "REQUESTER_REVISION"
)

// Purchase request vendor invoice status constants
const (
	PRVendorInvoiceStatusNotRequired         = "NOT_REQUIRED"
	PRVendorInvoiceStatusAwaitingInvoice     = "AWAITING_INVOICE"
	PRVendorInvoiceStatusPendingApproval     = "PENDING_APPROVAL"
	PRVendorInvoiceStatusApproved            = "APPROVED"
	PRVendorInvoiceStatusPendingVerification = "PENDING_VERIFICATION"
	PRVendorInvoiceStatusVerified            = "VERIFIED"
	PRVendorInvoiceStatusNeedsClarification  = "NEEDS_CLARIFICATION"
)

// Purchase request billing/realisasi status constants. This review cycle is
// intentionally separate from the PR's original approval workflow.
const (
	PRRealizationStatusNotRequired                 = "NOT_REQUIRED"
	PRRealizationStatusPendingOriginalApproval     = "PENDING_ORIGINAL_APPROVAL"
	PRRealizationStatusAwaitingUpload              = "AWAITING_UPLOAD"
	PRRealizationStatusPendingFinanceStaffReview   = "PENDING_FINANCE_STAFF_REVIEW"
	PRRealizationStatusPendingFinanceManagerReview = "PENDING_FINANCE_MANAGER_REVIEW"
	PRRealizationStatusVerified                    = "VERIFIED"
	PRRealizationStatusNeedsClarification          = "NEEDS_CLARIFICATION"
)

const (
	PRRealizationReviewStageSubmission     = "SUBMISSION"
	PRRealizationReviewStageFinanceStaff   = "FINANCE_STAFF"
	PRRealizationReviewStageFinanceManager = "FINANCE_MANAGER"

	PRRealizationReviewDecisionSubmitted          = "SUBMITTED"
	PRRealizationReviewDecisionApproved           = "APPROVED"
	PRRealizationReviewDecisionVerified           = "VERIFIED"
	PRRealizationReviewDecisionNeedsClarification = "NEEDS_CLARIFICATION"
)

// Request DTOs
type PurchaseRequestCreatePayload struct {
	Department                string                       `json:"department"`
	Title                     string                       `json:"title"`
	ProjectID                 *uint                        `json:"project_id"`
	VendorID                  *uint                        `json:"vendor_id"`
	VendorInvoiceReceivedDate *time.Time                   `json:"vendor_invoice_received_date"`
	PaymentTermsDays          *int                         `json:"payment_terms_days"`
	DueDate                   *time.Time                   `json:"due_date"`
	Notes                     string                       `json:"notes"`
	Items                     []PurchaseRequestItemPayload `json:"items" binding:"required,min=1"`
}

type PurchaseRequestItemPayload struct {
	ProductID      uint    `json:"product_id"`
	ItemName       string  `json:"item_name"`
	Quantity       int     `json:"quantity" binding:"required,min=1"`
	EstimatedPrice float64 `json:"estimated_price" binding:"required,min=0"`
	Notes          string  `json:"notes"`
}

type PurchaseRequestReviewItemPayload struct {
	ItemID   uint   `json:"item_id" binding:"required"`
	Decision string `json:"decision" binding:"required"`
}

type PurchaseRequestReviewPayload struct {
	Items                      []PurchaseRequestReviewItemPayload `json:"items" binding:"required,min=1"`
	Reason                     string                             `json:"reason"`
	EscalateToDirector         bool                               `json:"escalate_to_director"`
	RequireVendorInvoice       bool                               `json:"require_vendor_invoice"`
	RequireRealizationDocument bool                               `json:"require_realization_document"`
}

type PurchaseRequestProjectUpdatePayload struct {
	ProjectID *uint `json:"project_id"`
}

type PurchaseRequestInvoiceTermsUpdatePayload struct {
	VendorInvoiceReceivedDate *time.Time `json:"vendor_invoice_received_date"`
	PaymentTermsDays          *int       `json:"payment_terms_days"`
	DueDate                   *time.Time `json:"due_date"`
}

type PurchaseRequestInvoiceRevisionItemPayload struct {
	ItemID         uint    `json:"item_id" binding:"required"`
	Quantity       int     `json:"quantity" binding:"required,min=1"`
	EstimatedPrice float64 `json:"estimated_price" binding:"required,min=0"`
	Notes          string  `json:"notes"`
}

type PurchaseRequestInvoiceRevisionPayload struct {
	Notes string                                      `json:"notes"`
	Items []PurchaseRequestInvoiceRevisionItemPayload `json:"items" binding:"required,min=1"`
}

type PurchaseRequestVendorInvoiceVerificationPayload struct {
	Decision string `json:"decision" binding:"required"`
	Notes    string `json:"notes"`
}

type PurchaseRequestRealizationVerificationPayload struct {
	Decision string `json:"decision" binding:"required"`
	Notes    string `json:"notes"`
}

type PurchaseRequestOpenRevisionPayload struct {
	Reason string `json:"reason" binding:"required"`
}

type PurchaseRequestRequesterRevisionItemPayload struct {
	ItemID         uint    `json:"item_id"`
	ProductID      uint    `json:"product_id"`
	ItemName       string  `json:"item_name"`
	Quantity       int     `json:"quantity" binding:"required,min=1"`
	EstimatedPrice float64 `json:"estimated_price" binding:"required,min=0"`
	Notes          string  `json:"notes"`
}

type PurchaseRequestRequesterRevisionPayload struct {
	Notes string                                        `json:"notes"`
	Items []PurchaseRequestRequesterRevisionItemPayload `json:"items" binding:"required,min=1"`
}

type PurchaseRequestCancelPayload struct {
	Reason string `json:"reason" binding:"required"`
}
