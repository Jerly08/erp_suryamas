package models

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// PaymentDocument is an attachment kept with a payment request or a posted
// payment. It is deliberately independent from Purchase/PO documents because
// a non-invoice payment can have its own invoice, transfer proof, tax invoice,
// and other supporting files.
//
// Files are never silently deleted. A document that must be superseded is
// marked INVALIDATED and its event history remains available for audit.
type PaymentDocument struct {
	ID                 uint           `json:"id" gorm:"primaryKey"`
	PaymentID          uint           `json:"payment_id" gorm:"not null;index"`
	DocumentType       string         `json:"document_type" gorm:"size:50;not null;default:'SUPPORTING_DOCUMENT'"`
	ReferenceNumber    string         `json:"reference_number" gorm:"size:100"`
	FileName           string         `json:"file_name" gorm:"not null;size:255"`
	FilePath           string         `json:"-" gorm:"not null;size:500"`
	FileSize           int64          `json:"file_size"`
	MimeType           string         `json:"mime_type" gorm:"size:100"`
	Notes              string         `json:"notes" gorm:"type:text"`
	Status             string         `json:"status" gorm:"size:30;not null;default:'ACTIVE';index"`
	UploadedBy         uint           `json:"uploaded_by" gorm:"not null;index"`
	InvalidatedBy      *uint          `json:"invalidated_by,omitempty" gorm:"index"`
	InvalidatedAt      *time.Time     `json:"invalidated_at,omitempty"`
	InvalidationReason string         `json:"invalidation_reason" gorm:"type:text"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `json:"-" gorm:"index"`

	Payment     Payment                `json:"-" gorm:"foreignKey:PaymentID"`
	Uploader    User                   `json:"uploader" gorm:"foreignKey:UploadedBy"`
	Invalidator *User                  `json:"invalidator,omitempty" gorm:"foreignKey:InvalidatedBy"`
	Events      []PaymentDocumentEvent `json:"events,omitempty" gorm:"foreignKey:PaymentDocumentID"`
}

// PaymentDocumentEvent is an immutable event trail for the document itself.
// It makes a correction visible without changing a payment, allocation, or
// journal entry.
type PaymentDocumentEvent struct {
	ID                uint      `json:"id" gorm:"primaryKey"`
	PaymentDocumentID uint      `json:"payment_document_id" gorm:"not null;index"`
	PaymentID         uint      `json:"payment_id" gorm:"not null;index"`
	Action            string    `json:"action" gorm:"size:50;not null"`
	Notes             string    `json:"notes" gorm:"type:text"`
	ActorID           uint      `json:"actor_id" gorm:"not null;index"`
	OccurredAt        time.Time `json:"occurred_at" gorm:"not null;index"`
	CreatedAt         time.Time `json:"created_at"`

	Document PaymentDocument `json:"-" gorm:"foreignKey:PaymentDocumentID"`
	Actor    User            `json:"actor" gorm:"foreignKey:ActorID"`
}

const (
	PaymentDocumentTypeSupporting    = "SUPPORTING_DOCUMENT"
	PaymentDocumentTypeVendorInvoice = "VENDOR_INVOICE"
	PaymentDocumentTypePaymentProof  = "PAYMENT_PROOF"
	PaymentDocumentTypeTaxInvoice    = "TAX_INVOICE"
	PaymentDocumentTypeOther         = "OTHER"

	PaymentDocumentStatusActive      = "ACTIVE"
	PaymentDocumentStatusInvalidated = "INVALIDATED"

	PaymentDocumentEventUploaded    = "UPLOADED"
	PaymentDocumentEventInvalidated = "INVALIDATED"
)

// NormalizePaymentDocumentType accepts only the document categories exposed by
// the payment workflow. Empty input is a normal supporting document.
func NormalizePaymentDocumentType(documentType string) (string, bool) {
	normalized := strings.ToUpper(strings.TrimSpace(documentType))
	if normalized == "" {
		return PaymentDocumentTypeSupporting, true
	}

	switch normalized {
	case PaymentDocumentTypeSupporting,
		PaymentDocumentTypeVendorInvoice,
		PaymentDocumentTypePaymentProof,
		PaymentDocumentTypeTaxInvoice,
		PaymentDocumentTypeOther:
		return normalized, true
	default:
		return "", false
	}
}
