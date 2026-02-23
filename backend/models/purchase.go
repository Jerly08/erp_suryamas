package models

import (
	"time"

	"gorm.io/gorm"
)

type Purchase struct {
	ID       uint      `json:"id" gorm:"primaryKey"`
	Code     string    `json:"code" gorm:"unique;not null;size:20"`
	VendorID uint      `json:"vendor_id" gorm:"not null;index"`
	UserID   uint      `json:"user_id" gorm:"not null;index"`
	Date     time.Time `json:"date"`
	DueDate  time.Time `json:"due_date"`
	// Monetary breakdown
	SubtotalBeforeDiscount float64 `json:"subtotal_before_discount" gorm:"type:decimal(15,2);default:0"`
	ItemDiscountAmount     float64 `json:"item_discount_amount" gorm:"type:decimal(15,2);default:0"`
	Discount               float64 `json:"discount" gorm:"type:decimal(8,2);default:0"` // order-level discount percent
	OrderDiscountAmount    float64 `json:"order_discount_amount" gorm:"type:decimal(15,2);default:0"`
	NetBeforeTax           float64 `json:"net_before_tax" gorm:"type:decimal(15,2);default:0"`

	// Tax additions (Penambahan) - PPN, etc
	PPNRate           float64 `json:"ppn_rate" gorm:"type:decimal(8,2);default:0"`             // PPN percentage
	PPNAmount         float64 `json:"ppn_amount" gorm:"type:decimal(15,2);default:0"`          // Calculated PPN amount
	OtherTaxAdditions float64 `json:"other_tax_additions" gorm:"type:decimal(15,2);default:0"` // Other tax additions
	TotalTaxAdditions float64 `json:"total_tax_additions" gorm:"type:decimal(15,2);default:0"` // Total penambahan

	// Tax deductions (Pemotongan) - PPh, etc
	PPh21Rate          float64 `json:"pph21_rate" gorm:"type:decimal(8,2);default:0"`            // PPh 21 percentage
	PPh21Amount        float64 `json:"pph21_amount" gorm:"type:decimal(15,2);default:0"`         // Calculated PPh 21 amount
	PPh23Rate          float64 `json:"pph23_rate" gorm:"type:decimal(8,2);default:0"`            // PPh 23 percentage
	PPh23Amount        float64 `json:"pph23_amount" gorm:"type:decimal(15,2);default:0"`         // Calculated PPh 23 amount
	OtherTaxDeductions float64 `json:"other_tax_deductions" gorm:"type:decimal(15,2);default:0"` // Other tax deductions
	TotalTaxDeductions float64 `json:"total_tax_deductions" gorm:"type:decimal(15,2);default:0"` // Total pemotongan

	// Legacy tax field (kept for backward compatibility)
	TaxAmount   float64 `json:"tax_amount" gorm:"type:decimal(15,2);default:0"`
	TotalAmount float64 `json:"total_amount" gorm:"type:decimal(15,2);default:0"` // kept for compatibility (grand total)

	// Payment tracking fields
	PaidAmount        float64 `json:"paid_amount" gorm:"type:decimal(15,2);default:0"`
	OutstandingAmount float64 `json:"outstanding_amount" gorm:"type:decimal(15,2);default:0"`
	MatchingStatus    string  `json:"matching_status" gorm:"size:20;default:'PENDING'"`

	// Bank/Payment method fields
	PaymentMethod    string `json:"payment_method" gorm:"size:20;default:'CREDIT'"` // CASH, CREDIT, TRANSFER
	BankAccountID    *uint  `json:"bank_account_id" gorm:"index"`                   // For cash/transfer purchases
	CreditAccountID  *uint  `json:"credit_account_id" gorm:"index"`                 // For credit purchases - liability account
	PaymentReference string `json:"payment_reference" gorm:"size:100"`              // Check number, transfer reference, etc.

	Status string `json:"status" gorm:"size:20"` // DRAFT, PENDING_APPROVAL, APPROVED, COMPLETED, CANCELLED
	Notes  string `json:"notes" gorm:"type:text"`

	// Approval fields
	RequiresApproval    bool       `json:"requires_approval" gorm:"default:false"`
	ApprovalStatus      string     `json:"approval_status" gorm:"size:20;default:'NOT_STARTED'"`
	ApprovalAmountBasis string     `json:"approval_amount_basis" gorm:"size:40;default:'SUBTOTAL_BEFORE_DISCOUNT'"`
	ApprovalBaseAmount  float64    `json:"approval_base_amount" gorm:"type:decimal(15,2);default:0"`
	ApprovalRequestID   *uint      `json:"approval_request_id" gorm:"index"`
	ApprovedBy          *uint      `json:"approved_by" gorm:"index"`
	ApprovedAt          *time.Time `json:"approved_at"`

	// Purchase Request & Cost Center (Project) Integration
	PurchaseRequestID     *uint  `json:"purchase_request_id" gorm:"index"`
	ProjectID             *uint  `json:"project_id" gorm:"index"`
	InvoiceMatchingStatus string `json:"invoice_matching_status" gorm:"size:20;default:'NOT_APPLICABLE'"` // NOT_APPLICABLE, PENDING, MATCHED, MISMATCHED

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	Vendor          Contact          `json:"vendor" gorm:"foreignKey:VendorID"`
	User            User             `json:"user" gorm:"foreignKey:UserID"`
	PurchaseItems   []PurchaseItem   `json:"purchase_items" gorm:"foreignKey:PurchaseID"`
	BankAccount     *CashBank        `json:"bank_account,omitempty" gorm:"foreignKey:BankAccountID"`
	CreditAccount   *Account         `json:"credit_account,omitempty" gorm:"foreignKey:CreditAccountID"`
	ApprovalRequest *ApprovalRequest `json:"approval_request,omitempty" gorm:"foreignKey:ApprovalRequestID"`
	Approver        *User            `json:"approver,omitempty" gorm:"foreignKey:ApprovedBy"`
	PurchaseRequest *PurchaseRequest `json:"purchase_request,omitempty" gorm:"foreignKey:PurchaseRequestID"`
	Project         *Project         `json:"project,omitempty" gorm:"foreignKey:ProjectID"`
}

type PurchaseItem struct {
	ID               uint           `json:"id" gorm:"primaryKey"`
	PurchaseID       uint           `json:"purchase_id" gorm:"not null;index"`
	ProductID        uint           `json:"product_id" gorm:"not null;index"`
	Quantity         int            `json:"quantity" gorm:"not null"`
	UnitPrice        float64        `json:"unit_price" gorm:"type:decimal(15,2);default:0"`
	TotalPrice       float64        `json:"total_price" gorm:"type:decimal(15,2);default:0"`
	Discount         float64        `json:"discount" gorm:"type:decimal(15,2);default:0"`
	Tax              float64        `json:"tax" gorm:"type:decimal(15,2);default:0"`
	ExpenseAccountID uint           `json:"expense_account_id" gorm:"index"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	Purchase       Purchase `json:"purchase" gorm:"foreignKey:PurchaseID"`
	Product        Product  `json:"product" gorm:"foreignKey:ProductID"`
	ExpenseAccount Account  `json:"expense_account" gorm:"foreignKey:ExpenseAccountID"`
}

// PurchasePayment represents payments made for purchases (cross-reference with payments table)
type PurchasePayment struct {
	ID            uint           `json:"id" gorm:"primaryKey"`
	PurchaseID    uint           `json:"purchase_id" gorm:"not null;index"`
	PaymentNumber string         `json:"payment_number" gorm:"size:50"`
	Date          time.Time      `json:"date"`
	Amount        float64        `json:"amount" gorm:"type:decimal(15,2);default:0"`
	Method        string         `json:"method" gorm:"size:20"`
	Reference     string         `json:"reference" gorm:"size:100"`
	Notes         string         `json:"notes" gorm:"type:text"`
	CashBankID    *uint          `json:"cash_bank_id" gorm:"index"`
	UserID        uint           `json:"user_id" gorm:"not null;index"`
	PaymentID     *uint          `json:"payment_id" gorm:"index"` // Cross-reference to payments table
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	Purchase Purchase  `json:"purchase" gorm:"foreignKey:PurchaseID"`
	CashBank *CashBank `json:"cash_bank,omitempty" gorm:"foreignKey:CashBankID"`
	User     User      `json:"user" gorm:"foreignKey:UserID"`
	Payment  *Payment  `json:"payment,omitempty" gorm:"foreignKey:PaymentID"`
}

// Purchase Status Constants
const (
	PurchaseStatusDraft           = "DRAFT"
	PurchaseStatusPending         = "PENDING"
	PurchaseStatusPendingApproval = "PENDING_APPROVAL"
	PurchaseStatusApproved        = "APPROVED"
	PurchaseStatusCompleted       = "COMPLETED"
	PurchaseStatusPaid            = "PAID"
	PurchaseStatusCancelled       = "CANCELLED"
)

// Purchase Approval Status Constants
const (
	PurchaseApprovalNotStarted  = "NOT_STARTED"
	PurchaseApprovalNotRequired = "NOT_REQUIRED"
	PurchaseApprovalPending     = "PENDING"
	PurchaseApprovalApproved    = "APPROVED"
	PurchaseApprovalRejected    = "REJECTED"
)

// Purchase Matching Status Constants
const (
	PurchaseMatchingPending  = "PENDING"
	PurchaseMatchingPartial  = "PARTIAL"
	PurchaseMatchingMatched  = "MATCHED"
	PurchaseMatchingMismatch = "MISMATCH"
)

// Purchase Payment Method Constants
const (
	PurchasePaymentCredit   = "CREDIT"        // Credit purchase - pay later (creates accounts payable)
	PurchasePaymentCash     = "CASH"          // Cash purchase - immediate payment from cash/bank
	PurchasePaymentTransfer = "BANK_TRANSFER" // Bank transfer - immediate payment via bank transfer
	PurchasePaymentCheck    = "CHECK"         // Check payment - immediate payment via check
)

// Filter and Request DTOs
type PurchaseFilter struct {
	Status           string `json:"status"`
	VendorID         string `json:"vendor_id"`
	ProjectID        string `json:"project_id"`
	UserID           uint   `json:"user_id"` // Filter by requester user ID (for employee-only access)
	StartDate        string `json:"start_date"`
	EndDate          string `json:"end_date"`
	Search           string `json:"search"`
	ApprovalStatus   string `json:"approval_status"`
	RequiresApproval *bool  `json:"requires_approval"`
	Page             int    `json:"page"`
	Limit            int    `json:"limit"`
}

type PurchaseCreateRequest struct {
	VendorID uint      `json:"vendor_id" binding:"required"`
	Date     time.Time `json:"date" binding:"required"`
	DueDate  time.Time `json:"due_date"`
	Discount float64   `json:"discount"`

	// Link fields
	PurchaseRequestID *uint `json:"purchase_request_id"`
	ProjectID         *uint `json:"project_id"`

	// Payment method fields
	PaymentMethod    string `json:"payment_method"`    // CASH, CREDIT, TRANSFER
	BankAccountID    *uint  `json:"bank_account_id"`   // Required if payment_method is CASH or TRANSFER
	CreditAccountID  *uint  `json:"credit_account_id"` // Required if payment_method is CREDIT
	PaymentReference string `json:"payment_reference"`

	// Legacy tax field (for backward compatibility - ignored in calculation)
	Tax float64 `json:"tax"`

	// Tax additions (Penambahan) - using pointers to distinguish null vs zero
	PPNRate           *float64 `json:"ppn_rate"` // nil = default to 11%, 0 = no VAT
	OtherTaxAdditions float64  `json:"other_tax_additions"`

	// Tax deductions (Pemotongan) - using pointers to distinguish null vs zero
	PPh21Rate          *float64 `json:"pph21_rate"` // nil = 0%, explicit value otherwise
	PPh23Rate          *float64 `json:"pph23_rate"` // nil = 0%, explicit value otherwise
	OtherTaxDeductions float64  `json:"other_tax_deductions"`

	Notes string                `json:"notes"`
	Items []PurchaseItemRequest `json:"items" binding:"required,min=1"`
}

type PurchaseUpdateRequest struct {
	VendorID *uint      `json:"vendor_id"`
	Date     *time.Time `json:"date"`
	DueDate  *time.Time `json:"due_date"`
	Discount *float64   `json:"discount"`

	// Payment method fields
	PaymentMethod    *string `json:"payment_method"`
	BankAccountID    *uint   `json:"bank_account_id"`
	CreditAccountID  *uint   `json:"credit_account_id"`
	PaymentReference *string `json:"payment_reference"`

	// Legacy tax field (for backward compatibility - ignored in calculation)
	Tax *float64 `json:"tax"`

	// Tax additions (Penambahan) - pointers to distinguish null vs zero
	PPNRate           **float64 `json:"ppn_rate"` // double pointer for update operations
	OtherTaxAdditions *float64  `json:"other_tax_additions"`

	// Tax deductions (Pemotongan) - pointers to distinguish null vs zero
	PPh21Rate          **float64 `json:"pph21_rate"` // double pointer for update operations
	PPh23Rate          **float64 `json:"pph23_rate"` // double pointer for update operations
	OtherTaxDeductions *float64  `json:"other_tax_deductions"`

	Notes *string               `json:"notes"`
	Items []PurchaseItemRequest `json:"items"`
}

type PurchaseItemRequest struct {
	ProductID        uint    `json:"product_id" binding:"required"`
	Quantity         int     `json:"quantity" binding:"required,min=1"`
	UnitPrice        float64 `json:"unit_price" binding:"required,min=0"`
	Discount         float64 `json:"discount"`
	Tax              float64 `json:"tax"`
	ExpenseAccountID uint    `json:"expense_account_id"`
}

// Document Management
type PurchaseDocument struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	PurchaseID   uint           `json:"purchase_id" gorm:"not null;index"`
	DocumentType string         `json:"document_type" gorm:"size:50"`
	FileName     string         `json:"file_name" gorm:"not null;size:255"`
	FilePath     string         `json:"file_path" gorm:"not null;size:500"`
	FileSize     int64          `json:"file_size"`
	MimeType     string         `json:"mime_type" gorm:"size:100"`
	UploadedBy   uint           `json:"uploaded_by" gorm:"not null;index"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	Purchase Purchase `json:"purchase" gorm:"foreignKey:PurchaseID"`
	Uploader User     `json:"uploader" gorm:"foreignKey:UploadedBy"`
}

// Receipt Management
type PurchaseReceipt struct {
	ID            uint           `json:"id" gorm:"primaryKey"`
	PurchaseID    uint           `json:"purchase_id" gorm:"not null;index"`
	ReceiptNumber string         `json:"receipt_number" gorm:"size:50"`
	ReceivedDate  time.Time      `json:"received_date"`
	ReceivedBy    uint           `json:"received_by" gorm:"not null;index"`
	Status        string         `json:"status" gorm:"size:20;default:'PENDING'"`
	Notes         string         `json:"notes" gorm:"type:text"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	Purchase     Purchase              `json:"purchase" gorm:"foreignKey:PurchaseID"`
	Receiver     User                  `json:"receiver" gorm:"foreignKey:ReceivedBy"`
	ReceiptItems []PurchaseReceiptItem `json:"receipt_items" gorm:"foreignKey:ReceiptID"`
}

type PurchaseReceiptItem struct {
	ID               uint           `json:"id" gorm:"primaryKey"`
	ReceiptID        uint           `json:"receipt_id" gorm:"not null;index"`
	PurchaseItemID   uint           `json:"purchase_item_id" gorm:"not null;index"`
	QuantityReceived int            `json:"quantity_received" gorm:"not null"`
	Condition        string         `json:"condition" gorm:"size:50"`
	Notes            string         `json:"notes" gorm:"type:text"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	PurchaseReceipt PurchaseReceipt `json:"purchase_receipt" gorm:"foreignKey:ReceiptID"`
	PurchaseItem    PurchaseItem    `json:"purchase_item" gorm:"foreignKey:PurchaseItemID"`
}

// Analytics and Reporting
type PurchaseSummary struct {
	TotalPurchases       int64                   `json:"total_purchases"`
	TotalAmount          float64                 `json:"total_amount"`
	TotalApprovedAmount  float64                 `json:"total_approved_amount"`
	TotalPaid            float64                 `json:"total_paid"`
	TotalOutstanding     float64                 `json:"total_outstanding"`
	AvgOrderValue        float64                 `json:"avg_order_value"`
	TopVendors           []VendorPurchaseSummary `json:"top_vendors"`
	StatusCounts         map[string]int64        `json:"status_counts"`
	ApprovalStatusCounts map[string]int64        `json:"approval_status_counts"`
}

type VendorPurchaseSummary struct {
	VendorID    uint    `json:"vendor_id"`
	VendorName  string  `json:"vendor_name"`
	TotalAmount float64 `json:"total_amount"`
	TotalOrders int64   `json:"total_orders"`
}

type PurchaseAnalyticsData struct {
	Period         string  `json:"period"`
	TotalPurchases int64   `json:"total_purchases"`
	TotalAmount    float64 `json:"total_amount"`
	GrowthRate     float64 `json:"growth_rate"`
}

type PurchaseAnalyticsResponse struct {
	Period string                  `json:"period"`
	Data   []PurchaseAnalyticsData `json:"data"`
}

type PurchaseMatchingData struct {
	Purchase  Purchase           `json:"purchase"`
	Receipts  []PurchaseReceipt  `json:"receipts"`
	Documents []PurchaseDocument `json:"documents"`
}

type PayablesReportData struct {
	PurchaseID        uint      `json:"purchase_id"`
	PurchaseCode      string    `json:"purchase_code"`
	VendorName        string    `json:"vendor_name"`
	Date              time.Time `json:"date"`
	DueDate           time.Time `json:"due_date"`
	TotalAmount       float64   `json:"total_amount"`
	PaidAmount        float64   `json:"paid_amount"`
	OutstandingAmount float64   `json:"outstanding_amount"`
	DaysOverdue       int       `json:"days_overdue"`
	Status            string    `json:"status"`
}

type PayablesReportResponse struct {
	TotalOutstanding float64              `json:"total_outstanding"`
	OverdueAmount    float64              `json:"overdue_amount"`
	Payables         []PayablesReportData `json:"payables"`
}

// Request DTOs for Receipt Management
type PurchaseReceiptRequest struct {
	PurchaseID   uint                         `json:"purchase_id" binding:"required"`
	ReceivedDate time.Time                    `json:"received_date" binding:"required"`
	Notes        string                       `json:"notes"`
	ReceiptItems []PurchaseReceiptItemRequest `json:"receipt_items" binding:"required,min=1"`
}

type PurchaseReceiptItemRequest struct {
	PurchaseItemID   uint   `json:"purchase_item_id" binding:"required"`
	QuantityReceived int    `json:"quantity_received" binding:"required,min=1"`
	Condition        string `json:"condition"`
	Notes            string `json:"notes"`
	// Optional: trigger asset capitalization journal for this item
	CapitalizeAsset       bool  `json:"capitalize_asset"`
	FixedAssetAccountID   *uint `json:"fixed_asset_account_id"`
	SourceAccountOverride *uint `json:"source_account_id"` // override source (defaults to inventory 1301 or item expense)
}

// Receipt Status Constants
const (
	ReceiptStatusPending  = "PENDING"
	ReceiptStatusPartial  = "PARTIAL"
	ReceiptStatusComplete = "COMPLETE"
	ReceiptStatusRejected = "REJECTED"
)

// Receipt Condition Constants
const (
	ReceiptConditionGood     = "GOOD"
	ReceiptConditionDamaged  = "DAMAGED"
	ReceiptConditionDefected = "DEFECTED"
)

// Document Type Constants
const (
	PurchaseDocumentInvoice       = "INVOICE"
	PurchaseDocumentReceipt       = "RECEIPT"
	PurchaseDocumentPurchaseOrder = "PURCHASE_ORDER"
	PurchaseDocumentContract      = "CONTRACT"
)
