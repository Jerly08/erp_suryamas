package models

import (
	"time"

	"gorm.io/gorm"
)

const (
	PostedTransactionCorrectionTargetPurchase = "PURCHASE"
	PostedTransactionCorrectionTargetPayment  = "PAYMENT"
	PostedTransactionCorrectionTargetSale     = "SALE"
)

const (
	PostedTransactionCorrectionStatusPendingAccounting     = "PENDING_ACCOUNTING"
	PostedTransactionCorrectionStatusPendingFinanceManager = "PENDING_FINANCE_MANAGER"
	PostedTransactionCorrectionStatusApprovedForExecution  = "APPROVED_FOR_EXECUTION"
	PostedTransactionCorrectionStatusRejected              = "REJECTED"
	PostedTransactionCorrectionStatusCompleted             = "COMPLETED"
	PostedTransactionCorrectionStatusCancelled             = "CANCELLED"
)

const (
	PostedTransactionCorrectionTypeMissingPaymentProof = "MISSING_PAYMENT_PROOF"
	PostedTransactionCorrectionTypePaymentDate         = "PAYMENT_DATE"
	PostedTransactionCorrectionTypePaymentAmount       = "PAYMENT_AMOUNT"
	PostedTransactionCorrectionTypeSourceBank          = "SOURCE_BANK"
	PostedTransactionCorrectionTypeDestinationAccount  = "DESTINATION_ACCOUNT"
	PostedTransactionCorrectionTypePurchaseDate        = "PURCHASE_DATE"
	// SalesCreditClassification corrects a legacy cash/bank recognition for a
	// credit sale by linking the existing customer receipt to the invoice.
	// It never edits the original sale or payment journal.
	PostedTransactionCorrectionTypeSalesCreditClassification = "SALES_CREDIT_CLASSIFICATION"
	// COAReclassification moves one non cash/bank journal line to the correct
	// active COA through a new adjustment journal. The original journal stays
	// immutable and the correction is approved directly by Finance Manager.
	PostedTransactionCorrectionTypeCOAReclassification = "COA_RECLASSIFICATION"
	PostedTransactionCorrectionTypeOther               = "OTHER"
)

const (
	PostedTransactionCorrectionResolutionSupplementalProof  = "SUPPLEMENTAL_PROOF"
	PostedTransactionCorrectionResolutionJournalReclass     = "JOURNAL_RECLASSIFICATION"
	PostedTransactionCorrectionResolutionReverseRecreate    = "REVERSE_AND_RECREATE"
	PostedTransactionCorrectionResolutionBankReconciliation = "BANK_RECONCILIATION"
	PostedTransactionCorrectionResolutionManualAdjustment   = "MANUAL_ADJUSTMENT"
)

const (
	PostedTransactionCorrectionEventSubmitted           = "SUBMITTED"
	PostedTransactionCorrectionEventDocumentAdded       = "DOCUMENT_ADDED"
	PostedTransactionCorrectionEventAccountingReviewed  = "ACCOUNTING_REVIEWED"
	PostedTransactionCorrectionEventFinanceApproved     = "FINANCE_MANAGER_APPROVED"
	PostedTransactionCorrectionEventFinanceRejected     = "FINANCE_MANAGER_REJECTED"
	PostedTransactionCorrectionEventCompleted           = "COMPLETED"
	PostedTransactionCorrectionEventSalesCreditExecuted = "SALES_CREDIT_CLASSIFICATION_EXECUTED"
	PostedTransactionCorrectionEventCOAReclassified     = "COA_RECLASSIFICATION_EXECUTED"
	PostedTransactionCorrectionEventCancelled           = "CANCELLED"
)

// PostedTransactionCorrection records the approved handling of an error found
// after the original purchase or payment journal has been posted. It never
// changes the original transaction directly.
type PostedTransactionCorrection struct {
	ID uint `json:"id" gorm:"primaryKey"`

	TargetType string `json:"target_type" gorm:"not null;size:20;index"`
	TargetCode string `json:"target_code" gorm:"not null;size:100;index"`

	PurchaseID        *uint `json:"purchase_id,omitempty" gorm:"index"`
	PaymentID         *uint `json:"payment_id,omitempty" gorm:"index"`
	PurchasePaymentID *uint `json:"purchase_payment_id,omitempty" gorm:"index"`
	SaleID            *uint `json:"sale_id,omitempty" gorm:"index"`
	LinkedPaymentID   *uint `json:"linked_payment_id,omitempty" gorm:"index"`

	OriginalJournalEntryID uint64  `json:"original_journal_entry_id" gorm:"not null;index"`
	OriginalJournalLineID  *uint64 `json:"original_journal_line_id,omitempty" gorm:"index"`
	OriginalAccountID      *uint   `json:"original_account_id,omitempty" gorm:"index"`
	SourceSnapshot         string  `json:"source_snapshot" gorm:"type:text"`

	CorrectionType string `json:"correction_type" gorm:"not null;size:60;index"`
	Status         string `json:"status" gorm:"not null;size:40;index"`
	Reason         string `json:"reason" gorm:"not null;type:text"`

	RequestedDate                 *time.Time `json:"requested_date,omitempty"`
	RequestedAmount               *float64   `json:"requested_amount,omitempty" gorm:"type:decimal(20,2)"`
	RequestedCashBankReference    string     `json:"requested_cash_bank_reference" gorm:"size:255"`
	RequestedDestinationReference string     `json:"requested_destination_reference" gorm:"size:255"`
	RequestedReference            string     `json:"requested_reference" gorm:"size:255"`

	RequestedBy uint      `json:"requested_by" gorm:"not null;index"`
	RequestedAt time.Time `json:"requested_at" gorm:"not null;index"`

	AccountingResolution string     `json:"accounting_resolution" gorm:"size:60"`
	AccountingNotes      string     `json:"accounting_notes" gorm:"type:text"`
	ProposedAccountID    *uint      `json:"proposed_account_id,omitempty" gorm:"index"`
	AccountingReviewedBy *uint      `json:"accounting_reviewed_by,omitempty" gorm:"index"`
	AccountingReviewedAt *time.Time `json:"accounting_reviewed_at,omitempty"`

	FinanceManagerDecisionNotes string     `json:"finance_manager_decision_notes" gorm:"type:text"`
	FinanceManagerApprovedBy    *uint      `json:"finance_manager_approved_by,omitempty" gorm:"index"`
	FinanceManagerApprovedAt    *time.Time `json:"finance_manager_approved_at,omitempty"`

	CompletionNotes          string     `json:"completion_notes" gorm:"type:text"`
	CompletionJournalEntryID *uint64    `json:"completion_journal_entry_id,omitempty" gorm:"index"`
	ReplacementPaymentID     *uint      `json:"replacement_payment_id,omitempty" gorm:"index"`
	CompletedBy              *uint      `json:"completed_by,omitempty" gorm:"index"`
	CompletedAt              *time.Time `json:"completed_at,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	Purchase        *Purchase                             `json:"purchase,omitempty" gorm:"foreignKey:PurchaseID"`
	Payment         *Payment                              `json:"payment,omitempty" gorm:"foreignKey:PaymentID"`
	PurchasePayment *PurchasePayment                      `json:"purchase_payment,omitempty" gorm:"foreignKey:PurchasePaymentID"`
	Sale            *Sale                                 `json:"sale,omitempty" gorm:"foreignKey:SaleID"`
	LinkedPayment   *Payment                              `json:"linked_payment,omitempty" gorm:"foreignKey:LinkedPaymentID"`
	Requester       *User                                 `json:"requester,omitempty" gorm:"foreignKey:RequestedBy"`
	AccountingUser  *User                                 `json:"accounting_user,omitempty" gorm:"foreignKey:AccountingReviewedBy"`
	FinanceManager  *User                                 `json:"finance_manager,omitempty" gorm:"foreignKey:FinanceManagerApprovedBy"`
	Completer       *User                                 `json:"completer,omitempty" gorm:"foreignKey:CompletedBy"`
	OriginalAccount *Account                              `json:"original_account,omitempty" gorm:"foreignKey:OriginalAccountID"`
	ProposedAccount *Account                              `json:"proposed_account,omitempty" gorm:"foreignKey:ProposedAccountID"`
	Documents       []PostedTransactionCorrectionDocument `json:"documents,omitempty" gorm:"foreignKey:CorrectionID"`
	Events          []PostedTransactionCorrectionEvent    `json:"events,omitempty" gorm:"foreignKey:CorrectionID"`
}

type PostedTransactionCorrectionDocument struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	CorrectionID uint      `json:"correction_id" gorm:"not null;index"`
	FileName     string    `json:"file_name" gorm:"not null;size:255"`
	FilePath     string    `json:"file_path" gorm:"not null;size:500"`
	FileSize     int64     `json:"file_size"`
	MimeType     string    `json:"mime_type" gorm:"size:100"`
	UploadedBy   uint      `json:"uploaded_by" gorm:"not null;index"`
	UploadedAt   time.Time `json:"uploaded_at" gorm:"not null"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	Uploader *User `json:"uploader,omitempty" gorm:"foreignKey:UploadedBy"`
}

type PostedTransactionCorrectionEvent struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	CorrectionID uint      `json:"correction_id" gorm:"not null;index"`
	EventType    string    `json:"event_type" gorm:"not null;size:60;index"`
	Notes        string    `json:"notes" gorm:"type:text"`
	ActorID      uint      `json:"actor_id" gorm:"not null;index"`
	ActorRole    string    `json:"actor_role" gorm:"size:60"`
	CreatedAt    time.Time `json:"created_at"`

	Actor *User `json:"actor,omitempty" gorm:"foreignKey:ActorID"`
}

type PostedTransactionCorrectionCreateRequest struct {
	PurchaseID                    *uint    `json:"purchase_id"`
	PaymentID                     *uint    `json:"payment_id"`
	SaleID                        *uint    `json:"sale_id"`
	LinkedPaymentID               *uint    `json:"linked_payment_id"`
	OriginalJournalLineID         *uint64  `json:"original_journal_line_id"`
	ProposedAccountID             *uint    `json:"proposed_account_id"`
	CorrectionType                string   `json:"correction_type" binding:"required"`
	Reason                        string   `json:"reason" binding:"required"`
	RequestedDate                 string   `json:"requested_date"`
	RequestedAmount               *float64 `json:"requested_amount"`
	RequestedCashBankReference    string   `json:"requested_cash_bank_reference"`
	RequestedDestinationReference string   `json:"requested_destination_reference"`
	RequestedReference            string   `json:"requested_reference"`
}

type PostedTransactionCorrectionAccountingReviewRequest struct {
	Resolution        string `json:"resolution" binding:"required"`
	Notes             string `json:"notes" binding:"required"`
	ProposedAccountID *uint  `json:"proposed_account_id"`
}

type PostedTransactionCorrectionFinanceManagerDecisionRequest struct {
	Approve bool   `json:"approve"`
	Notes   string `json:"notes"`
}

type PostedTransactionCorrectionCompleteRequest struct {
	Notes                    string  `json:"notes" binding:"required"`
	CompletionJournalEntryID *uint64 `json:"completion_journal_entry_id"`
	ReplacementPaymentID     *uint   `json:"replacement_payment_id"`
}

type PostedTransactionCorrectionFilter struct {
	PurchaseID *uint
	PaymentID  *uint
	SaleID     *uint
	Status     string
	Page       int
	Limit      int
}
