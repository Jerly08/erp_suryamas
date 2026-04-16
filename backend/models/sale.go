package models

import (
	"time"

	"gorm.io/gorm"
)

type Sale struct {
	ID                uint       `json:"id" gorm:"primaryKey"`
	Code              string     `json:"code" gorm:"unique;not null;size:20"`
	CustomerID        uint       `json:"customer_id" gorm:"not null;index"`
	UserID            uint       `json:"user_id" gorm:"not null;index"`
	SalesPersonID     *uint      `json:"sales_person_id" gorm:"index"`
	InvoiceTypeID     *uint      `json:"invoice_type_id" gorm:"index"`
	ProjectID         *uint      `json:"project_id" gorm:"index"`
	Type              string     `json:"type" gorm:"size:20;default:'INVOICE'"` // QUOTATION, ORDER, INVOICE
	Date              time.Time  `json:"date"`
	DueDate           time.Time  `json:"due_date"`
	ValidUntil        *time.Time `json:"valid_until"`
	QuotationNumber   string     `json:"quotation_number" gorm:"size:50"`
	InvoiceNumber     string     `json:"invoice_number" gorm:"size:50"`
	Currency          string     `json:"currency" gorm:"size:5;default:'IDR'"`
	ExchangeRate      float64    `json:"exchange_rate" gorm:"type:decimal(12,6);default:1"`
	TotalAmount       float64    `json:"total_amount" gorm:"type:decimal(15,2);default:0"`
	PaidAmount        float64    `json:"paid_amount" gorm:"type:decimal(15,2);default:0"`
	OutstandingAmount float64    `json:"outstanding_amount" gorm:"type:decimal(15,2);default:0"`
	Subtotal          float64    `json:"subtotal" gorm:"type:decimal(15,2);default:0"`
	SubTotal          float64    `json:"sub_total" gorm:"-"` // Read-only alias for frontend consistency
	DiscountPercent   float64    `json:"discount_percent" gorm:"type:decimal(5,2);default:0"`
	DiscountAmount    float64    `json:"discount_amount" gorm:"type:decimal(15,2);default:0"`
	TaxableAmount     float64    `json:"taxable_amount" gorm:"type:decimal(15,2);default:0"`
	// Legacy tax fields (kept for backward compatibility)
	Tax        float64 `json:"tax" gorm:"type:decimal(15,2);default:0"`
	PPN        float64 `json:"ppn" gorm:"type:decimal(15,2);default:0"`
	PPNPercent float64 `json:"ppn_percent" gorm:"type:decimal(5,2);default:0"`
	PPh        float64 `json:"pph" gorm:"type:decimal(15,2);default:0"`
	PPhPercent float64 `json:"pph_percent" gorm:"type:decimal(5,2);default:0"`
	PPhType    string  `json:"pph_type" gorm:"size:20"`
	TotalTax   float64 `json:"total_tax" gorm:"type:decimal(15,2);default:0"`

	// Enhanced tax configuration (similar to Purchase model)
	NetBeforeTax float64 `json:"net_before_tax" gorm:"type:decimal(15,2);default:0"`

	// Tax additions (Penambahan) - PPN, etc
	PPNRate           float64 `json:"ppn_rate" gorm:"type:decimal(8,2);default:0"`             // PPN percentage
	PPNAmount         float64 `json:"ppn_amount" gorm:"type:decimal(15,2);default:0"`          // Calculated PPN amount
	OtherTaxAdditions float64 `json:"other_tax_additions" gorm:"type:decimal(15,2);default:0"` // Other tax additions
	TotalTaxAdditions float64 `json:"total_tax_additions" gorm:"type:decimal(15,2);default:0"` // Total penambahan

	// Tax deductions (Pemotongan) - PPh, etc
	PPh21Rate                     float64        `json:"pph21_rate" gorm:"type:decimal(8,2);default:0"`            // PPh 21 percentage
	PPh21Amount                   float64        `json:"pph21_amount" gorm:"type:decimal(15,2);default:0"`         // Calculated PPh 21 amount
	PPh23Rate                     float64        `json:"pph23_rate" gorm:"type:decimal(8,2);default:0"`            // PPh 23 percentage
	PPh23Amount                   float64        `json:"pph23_amount" gorm:"type:decimal(15,2);default:0"`         // Calculated PPh 23 amount
	OtherTaxDeductions            float64        `json:"other_tax_deductions" gorm:"type:decimal(15,2);default:0"` // Other tax deductions
	TotalTaxDeductions            float64        `json:"total_tax_deductions" gorm:"type:decimal(15,2);default:0"` // Total pemotongan
	PPh23SettlementDate           *time.Time     `json:"pph23_settlement_date" gorm:"index"`
	PPh23SettlementReference      string         `json:"pph23_settlement_reference" gorm:"size:100"`
	PPh23SettlementNotes          string         `json:"pph23_settlement_notes" gorm:"type:text"`
	PPh23SettlementJournalEntryID *uint          `json:"pph23_settlement_journal_entry_id" gorm:"index"`
	PPh23SettlementUserID         *uint          `json:"pph23_settlement_user_id" gorm:"index"`
	PaymentTerms                  string         `json:"payment_terms" gorm:"size:50"`
	PaymentMethod                 string         `json:"payment_method" gorm:"size:50"`
	PaymentMethodType             string         `json:"payment_method_type" gorm:"size:20"` // CASH, BANK, CREDIT
	CashBankID                    *uint          `json:"cash_bank_id" gorm:"index"`
	ShippingMethod                string         `json:"shipping_method" gorm:"size:50"`
	ShippingCost                  float64        `json:"shipping_cost" gorm:"type:decimal(15,2);default:0"`
	ShippingTaxable               bool           `json:"shipping_taxable" gorm:"default:false"`
	BillingAddress                string         `json:"billing_address" gorm:"type:text"`
	ShippingAddress               string         `json:"shipping_address" gorm:"type:text"`
	Status                        string         `json:"status" gorm:"size:20"` // DRAFT, PENDING, CONFIRMED, CANCELLED
	SettlementStatus              string         `json:"settlement_status" gorm:"-"`
	Notes                         string         `json:"notes" gorm:"type:text"`
	InternalNotes                 string         `json:"internal_notes" gorm:"type:text"`
	Reference                     string         `json:"reference" gorm:"size:100"`
	CreatedAt                     time.Time      `json:"created_at"`
	UpdatedAt                     time.Time      `json:"updated_at"`
	DeletedAt                     gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	Customer         Contact               `json:"customer" gorm:"foreignKey:CustomerID"`
	User             User                  `json:"user" gorm:"foreignKey:UserID"`
	SalesPerson      *Contact              `json:"sales_person" gorm:"foreignKey:SalesPersonID"`
	InvoiceType      *InvoiceType          `json:"invoice_type" gorm:"foreignKey:InvoiceTypeID"`
	Project          *Project              `json:"project,omitempty" gorm:"foreignKey:ProjectID"`
	SaleItems        []SaleItem            `json:"sale_items" gorm:"foreignKey:SaleID"`
	PaymentSchedules []SalePaymentSchedule `json:"payment_schedules" gorm:"foreignKey:SaleID"`
	SalePayments     []SalePayment         `json:"sale_payments" gorm:"foreignKey:SaleID"`
	SaleReturns      []SaleReturn          `json:"sale_returns" gorm:"foreignKey:SaleID"`
}

// AfterFind hook to ensure computed fields are set correctly
func (s *Sale) AfterFind(tx *gorm.DB) (err error) {
	s.refreshComputedFields()
	return
}

type SaleItem struct {
	ID                  uint    `json:"id" gorm:"primaryKey"`
	SaleID              uint    `json:"sale_id" gorm:"not null;index"`
	ProductID           uint    `json:"product_id" gorm:"not null;index"`
	ProductName         string  `json:"product_name" gorm:"size:255;default:''"`
	Description         string  `json:"description" gorm:"type:text"`
	Location            string  `json:"location" gorm:"size:255"`
	Quantity            int     `json:"quantity" gorm:"not null"`
	UnitPrice           float64 `json:"unit_price" gorm:"type:decimal(15,2);default:0"`
	DiscountPercent     float64 `json:"discount_percent" gorm:"type:decimal(5,2);default:0"`
	DiscountAmount      float64 `json:"discount_amount" gorm:"type:decimal(15,2);default:0"`
	LineTotal           float64 `json:"line_total" gorm:"type:decimal(15,2);default:0"`
	Taxable             bool    `json:"taxable" gorm:"default:true"`
	PPNAmount           float64 `json:"ppn_amount" gorm:"type:decimal(15,2);default:0"`
	PPhAmount           float64 `json:"pph_amount" gorm:"type:decimal(15,2);default:0"`
	TotalTax            float64 `json:"total_tax" gorm:"type:decimal(15,2);default:0"`
	FinalAmount         float64 `json:"final_amount" gorm:"type:decimal(15,2);default:0"`
	AvailableQuantity   int     `json:"available_quantity" gorm:"default:0"`
	FulfilledQuantity   int     `json:"fulfilled_quantity" gorm:"default:0"`
	BackorderQuantity   int     `json:"backorder_quantity" gorm:"default:0"`
	EstimatedUnitCost   float64 `json:"estimated_unit_cost" gorm:"type:decimal(15,2);default:0"`
	EstimatedCOGSAmount float64 `json:"estimated_cogs_amount" gorm:"type:decimal(15,2);default:0"`
	ActualCOGSAmount    float64 `json:"actual_cogs_amount" gorm:"type:decimal(15,2);default:0"`
	COGSVarianceAmount  float64 `json:"cogs_variance_amount" gorm:"type:decimal(15,2);default:0"`
	FulfillmentStatus   string  `json:"fulfillment_status" gorm:"size:30;default:'IN_STOCK'"`
	// Computed fields - these are calculated and should not be set directly
	TotalPrice       float64        `json:"total_price" gorm:"type:decimal(15,2);default:0;->"` // Read-only: Same as LineTotal
	Discount         float64        `json:"discount" gorm:"type:decimal(15,2);default:0;->"`    // Read-only: Legacy field
	Tax              float64        `json:"tax" gorm:"type:decimal(15,2);default:0;->"`         // Read-only: Legacy field
	RevenueAccountID uint           `json:"revenue_account_id" gorm:"index"`
	TaxAccountID     *uint          `json:"tax_account_id" gorm:"index"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	Sale           Sale                      `json:"-" gorm:"foreignKey:SaleID"` // Exclude to prevent circular reference
	Product        Product                   `json:"product" gorm:"foreignKey:ProductID"`
	RevenueAccount Account                   `json:"revenue_account" gorm:"foreignKey:RevenueAccountID"`
	TaxAccount     *Account                  `json:"tax_account" gorm:"foreignKey:TaxAccountID"`
	Allocations    []SaleBackorderAllocation `json:"allocations,omitempty" gorm:"foreignKey:SaleItemID"`
}

type SalePaymentSchedule struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	SaleID      uint           `json:"sale_id" gorm:"not null;index"`
	Sequence    int            `json:"sequence" gorm:"not null"`
	Percentage  float64        `json:"percentage" gorm:"type:decimal(8,4);default:0"`
	Amount      float64        `json:"amount" gorm:"type:decimal(15,2);default:0"`
	PaidAmount  float64        `json:"paid_amount" gorm:"-"`
	Outstanding float64        `json:"outstanding_amount" gorm:"-"`
	DueDate     time.Time      `json:"due_date" gorm:"not null"`
	Description string         `json:"description" gorm:"size:255"`
	Status      string         `json:"status" gorm:"size:20;default:'PENDING'"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	Sale Sale `json:"-" gorm:"foreignKey:SaleID"`
}

// Sale Type Constants
const (
	SaleTypeQuotation = "QUOTATION"
	SaleTypeOrder     = "ORDER"
	SaleTypeInvoice   = "INVOICE"
)

// Sale item fulfillment constants
const (
	SaleFulfillmentStatusService          = "SERVICE"
	SaleFulfillmentStatusInStock          = "IN_STOCK"
	SaleFulfillmentStatusPartialBackorder = "PARTIAL_BACKORDER"
	SaleFulfillmentStatusBackordered      = "BACKORDERED"
	SaleFulfillmentStatusFulfilled        = "FULFILLED"
)

// Sale Status Constants
const (
	SaleStatusDraft         = "DRAFT"
	SaleStatusPending       = "PENDING"
	SaleStatusConfirmed     = "CONFIRMED"
	SaleStatusCompleted     = "COMPLETED"
	SaleStatusCancelled     = "CANCELLED"
	SaleStatusInvoiced      = "INVOICED"
	SaleStatusOverdue       = "OVERDUE"
	SaleStatusPaid          = "PAID"
	SaleStatusPartiallyPaid = "PARTIALLY_PAID"
)

// Filter and Request DTOs
type SalesFilter struct {
	Status     string `json:"status"`
	CustomerID string `json:"customer_id"`
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`
	Search     string `json:"search"`
	Page       int    `json:"page"`
	Limit      int    `json:"limit"`
}

type SaleCreateRequest struct {
	CustomerID      uint       `json:"customer_id" binding:"required"`
	SalesPersonID   *uint      `json:"sales_person_id"`
	InvoiceTypeID   *uint      `json:"invoice_type_id"`
	ProjectID       *uint      `json:"project_id"`
	Type            string     `json:"type" binding:"required"`
	Date            time.Time  `json:"date" binding:"required"`
	DueDate         time.Time  `json:"due_date"`
	ValidUntil      *time.Time `json:"valid_until"`
	Currency        string     `json:"currency"`
	ExchangeRate    *float64   `json:"exchange_rate"`
	DiscountPercent float64    `json:"discount_percent"`
	// Legacy tax fields (for backward compatibility)
	PPNPercent *float64 `json:"ppn_percent"`
	PPhPercent float64  `json:"pph_percent"`
	PPhType    string   `json:"pph_type"`

	// Enhanced tax configuration (similar to Purchase model)
	PPNRate            *float64                     `json:"ppn_rate"`             // PPN percentage (pointer to allow 0)
	OtherTaxAdditions  float64                      `json:"other_tax_additions"`  // Other tax additions
	PPh21Rate          float64                      `json:"pph21_rate"`           // PPh 21 percentage
	PPh23Rate          float64                      `json:"pph23_rate"`           // PPh 23 percentage
	OtherTaxDeductions float64                      `json:"other_tax_deductions"` // Other tax deductions
	PaymentTerms       string                       `json:"payment_terms"`
	PaymentMethod      string                       `json:"payment_method"`
	PaymentMethodType  string                       `json:"payment_method_type"` // CASH, BANK, CREDIT
	CashBankID         *uint                        `json:"cash_bank_id"`
	ShippingMethod     string                       `json:"shipping_method"`
	ShippingCost       float64                      `json:"shipping_cost"`
	ShippingTaxable    bool                         `json:"shipping_taxable"`
	BillingAddress     string                       `json:"billing_address"`
	ShippingAddress    string                       `json:"shipping_address"`
	Notes              string                       `json:"notes"`
	InternalNotes      string                       `json:"internal_notes"`
	Reference          string                       `json:"reference"`
	Items              []SaleItemRequest            `json:"items" binding:"required,min=1"`
	PaymentSchedules   []SalePaymentScheduleRequest `json:"payment_schedules"`
}

type SaleUpdateRequest struct {
	CustomerID      *uint      `json:"customer_id"`
	SalesPersonID   *uint      `json:"sales_person_id"`
	ProjectID       *uint      `json:"project_id"`
	Date            *time.Time `json:"date"`
	DueDate         *time.Time `json:"due_date"`
	ValidUntil      *time.Time `json:"valid_until"`
	DiscountPercent *float64   `json:"discount_percent"`
	// Legacy tax fields (for backward compatibility)
	PPNPercent *float64 `json:"ppn_percent"`
	PPhPercent *float64 `json:"pph_percent"`
	PPhType    *string  `json:"pph_type"`

	// Enhanced tax configuration (similar to Purchase model)
	PPNRate            *float64                     `json:"ppn_rate"`             // PPN percentage
	OtherTaxAdditions  *float64                     `json:"other_tax_additions"`  // Other tax additions
	PPh21Rate          *float64                     `json:"pph21_rate"`           // PPh 21 percentage
	PPh23Rate          *float64                     `json:"pph23_rate"`           // PPh 23 percentage
	OtherTaxDeductions *float64                     `json:"other_tax_deductions"` // Other tax deductions
	PaymentTerms       *string                      `json:"payment_terms"`
	PaymentMethod      *string                      `json:"payment_method"`
	PaymentMethodType  *string                      `json:"payment_method_type"` // CASH, BANK, CREDIT
	CashBankID         *uint                        `json:"cash_bank_id"`
	ShippingMethod     *string                      `json:"shipping_method"`
	ShippingCost       *float64                     `json:"shipping_cost"`
	ShippingTaxable    *bool                        `json:"shipping_taxable"`
	BillingAddress     *string                      `json:"billing_address"`
	ShippingAddress    *string                      `json:"shipping_address"`
	Notes              *string                      `json:"notes"`
	InternalNotes      *string                      `json:"internal_notes"`
	Reference          *string                      `json:"reference"`
	Items              []SaleItemRequest            `json:"items"`
	PaymentSchedules   []SalePaymentScheduleRequest `json:"payment_schedules"`
}

type SaleItemRequest struct {
	ProductID       uint     `json:"product_id"`
	ProductName     string   `json:"product_name"`
	Description     string   `json:"description"`
	Location        string   `json:"location"`
	Quantity        float64  `json:"quantity" binding:"required,min=1"` // Accept float from frontend
	UnitPrice       float64  `json:"unit_price" binding:"required,min=0"`
	DiscountPercent *float64 `json:"discount_percent"`
	DiscountAmount  *float64 `json:"discount_amount"`
	// Legacy fields for backward compatibility
	Discount         *float64 `json:"discount"` // Can come from frontend
	Tax              *float64 `json:"tax"`
	Taxable          *bool    `json:"taxable"`
	RevenueAccountID uint     `json:"revenue_account_id"`
	TaxAccountID     *uint    `json:"tax_account_id"`
}

type SalePaymentScheduleRequest struct {
	Sequence    int       `json:"sequence"`
	Percentage  float64   `json:"percentage"`
	DueDate     time.Time `json:"due_date"`
	Description string    `json:"description"`
}

// Return related to a Sale
type SaleReturn struct {
	ID               uint           `json:"id" gorm:"primaryKey"`
	SaleID           uint           `json:"sale_id" gorm:"not null;index"`
	UserID           uint           `json:"user_id" gorm:"not null;index"`
	ApproverID       *uint          `json:"approver_id" gorm:"index"`
	ReturnNumber     string         `json:"return_number" gorm:"size:50"`
	Type             string         `json:"type" gorm:"size:20"`
	Date             time.Time      `json:"date"`
	Reason           string         `json:"reason" gorm:"type:text"`
	CreditNoteNumber string         `json:"credit_note_number" gorm:"size:50"`
	TotalAmount      float64        `json:"total_amount" gorm:"type:decimal(15,2);default:0"`
	Status           string         `json:"status" gorm:"size:20;default:'PENDING'"`
	Notes            string         `json:"notes" gorm:"type:text"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	Sale        Sale             `json:"sale" gorm:"foreignKey:SaleID"`
	User        User             `json:"user" gorm:"foreignKey:UserID"`
	Approver    *User            `json:"approver" gorm:"foreignKey:ApproverID"`
	ReturnItems []SaleReturnItem `json:"return_items" gorm:"foreignKey:SaleReturnID"`
}

type SaleReturnItem struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	SaleReturnID uint           `json:"sale_return_id" gorm:"not null;index"`
	SaleItemID   uint           `json:"sale_item_id" gorm:"not null;index"`
	Quantity     int            `json:"quantity" gorm:"not null"`
	Reason       string         `json:"reason" gorm:"size:255"`
	UnitPrice    float64        `json:"unit_price" gorm:"type:decimal(15,2);default:0"`
	TotalAmount  float64        `json:"total_amount" gorm:"type:decimal(15,2);default:0"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	SaleReturn SaleReturn `json:"sale_return" gorm:"foreignKey:SaleReturnID"`
	SaleItem   SaleItem   `json:"sale_item" gorm:"foreignKey:SaleItemID"`
}

// Return types and statuses
const (
	ReturnTypeCreditNote = "CREDIT_NOTE"
	ReturnTypeRefund     = "REFUND"
)

const (
	ReturnStatusPending  = "PENDING"
	ReturnStatusApproved = "APPROVED"
	ReturnStatusRejected = "REJECTED"
)

// Sale Payment Status Constants
const (
	SalePaymentStatusCompleted = "COMPLETED"
	SalePaymentStatusPending   = "PENDING"
	SalePaymentStatusCancelled = "CANCELLED"
)

// Result DTOs
type SalesResult struct {
	Data       []Sale `json:"data"`
	Total      int    `json:"total"`
	Page       int    `json:"page"`
	Limit      int    `json:"limit"`
	TotalPages int    `json:"total_pages"`
}

// Reporting/Analytics DTOs

type CustomerSales struct {
	CustomerID   uint    `json:"customer_id"`
	CustomerName string  `json:"customer_name"`
	TotalAmount  float64 `json:"total_amount"`
	TotalOrders  int64   `json:"total_orders"`
}

type SalesSummaryResponse struct {
	TotalSales       int64           `json:"total_sales"`
	TotalAmount      float64         `json:"total_amount"`
	TotalPaid        float64         `json:"total_paid"`
	TotalOutstanding float64         `json:"total_outstanding"`
	AvgOrderValue    float64         `json:"avg_order_value"`
	TopCustomers     []CustomerSales `json:"top_customers"`
}

type SalesAnalyticsData struct {
	Period      string  `json:"period"`
	TotalSales  int64   `json:"total_sales"`
	TotalAmount float64 `json:"total_amount"`
	GrowthRate  float64 `json:"growth_rate"`
}

type SalesAnalyticsResponse struct {
	Period string               `json:"period"`
	Data   []SalesAnalyticsData `json:"data"`
}

type ReceivableItem struct {
	SaleID            uint      `json:"sale_id"`
	InvoiceNumber     string    `json:"invoice_number"`
	CustomerName      string    `json:"customer_name"`
	Date              time.Time `json:"date"`
	DueDate           time.Time `json:"due_date"`
	TotalAmount       float64   `json:"total_amount"`
	PaidAmount        float64   `json:"paid_amount"`
	OutstandingAmount float64   `json:"outstanding_amount"`
	DaysOverdue       int       `json:"days_overdue"`
	Status            string    `json:"status"`
}

type ReceivablesReportResponse struct {
	TotalOutstanding float64          `json:"total_outstanding"`
	OverdueAmount    float64          `json:"overdue_amount"`
	Receivables      []ReceivableItem `json:"receivables"`
}

// Payment and Return DTOs
type SalePaymentRequest struct {
	SaleID        uint      `json:"sale_id"` // Make optional, will be set by controller
	Amount        float64   `json:"amount" binding:"required,min=0"`
	PaymentDate   time.Time `json:"payment_date" binding:"required"`
	PaymentMethod string    `json:"payment_method" binding:"required"`
	ExchangeRate  float64   `json:"exchange_rate"`
	Reference     string    `json:"reference"`
	Notes         string    `json:"notes"`
	CashBankID    *uint     `json:"cash_bank_id"` // Add cashbank integration
	AccountID     *uint     `json:"account_id"`   // Add account integration
}

type SalePPh23SettlementRequest struct {
	Date      time.Time `json:"date" binding:"required"`
	Reference string    `json:"reference"`
	Notes     string    `json:"notes"`
}

type SaleReturnRequest struct {
	SaleID      uint                    `json:"sale_id" binding:"required"`
	ReturnDate  time.Time               `json:"return_date" binding:"required"`
	Reason      string                  `json:"reason" binding:"required"`
	Notes       string                  `json:"notes"`
	ReturnItems []SaleReturnItemRequest `json:"return_items" binding:"required,min=1"`
}

type SaleReturnItemRequest struct {
	SaleItemID uint   `json:"sale_item_id" binding:"required"`
	Quantity   int    `json:"quantity" binding:"required,min=1"`
	Reason     string `json:"reason"`
}

// Payment Summary DTO
type SalePaymentSummary struct {
	SaleID            uint       `json:"sale_id"`
	TotalAmount       float64    `json:"total_amount"`
	PaidAmount        float64    `json:"paid_amount"`
	OutstandingAmount float64    `json:"outstanding_amount"`
	PaymentCount      int        `json:"payment_count"`
	LastPaymentDate   *time.Time `json:"last_payment_date"`
}

// SalePayment model enhancement
type SalePayment struct {
	ID                    uint           `json:"id" gorm:"primaryKey"`
	SaleID                uint           `json:"sale_id" gorm:"not null;index"`
	Amount                float64        `json:"amount" gorm:"type:decimal(15,2);not null"`
	NativeAmount          float64        `json:"native_amount" gorm:"type:decimal(20,6);default:0"`
	Currency              string         `json:"currency" gorm:"size:5;default:'IDR';not null"`
	ExchangeRate          float64        `json:"exchange_rate" gorm:"type:decimal(15,6);default:1;not null"`
	BookAmountIDR         float64        `json:"book_amount_idr" gorm:"type:decimal(20,2);default:0;not null"`
	ForexGainLossAmount   float64        `json:"forex_gain_loss_amount" gorm:"type:decimal(20,2);default:0"`
	ForexGainLossType     string         `json:"forex_gain_loss_type" gorm:"size:10"`
	PaymentDate           time.Time      `json:"payment_date" gorm:"not null;index"`
	PaymentMethod         string         `json:"payment_method" gorm:"size:50;not null"`
	Reference             string         `json:"reference" gorm:"size:100"`
	Notes                 string         `json:"notes" gorm:"type:text"`
	CashBankID            *uint          `json:"cash_bank_id" gorm:"index"`
	AccountID             *uint          `json:"account_id" gorm:"index"`
	CashBankCode          string         `json:"cash_bank_code" gorm:"size:50"`
	CashBankName          string         `json:"cash_bank_name" gorm:"size:120"`
	CashBankCurrency      string         `json:"cash_bank_currency" gorm:"size:5"`
	CashBankGLAccountID   *uint          `json:"cash_bank_gl_account_id" gorm:"index"`
	CashBankGLAccountCode string         `json:"cash_bank_gl_account_code" gorm:"size:20"`
	CashBankGLAccountName string         `json:"cash_bank_gl_account_name" gorm:"size:120"`
	UserID                uint           `json:"user_id" gorm:"not null;index"`
	Status                string         `json:"status" gorm:"size:20;default:'COMPLETED'"` // COMPLETED, PENDING, CANCELLED
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
	DeletedAt             gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	Sale     Sale      `json:"sale" gorm:"foreignKey:SaleID"`
	CashBank *CashBank `json:"cash_bank" gorm:"foreignKey:CashBankID"`
	Account  *Account  `json:"account" gorm:"foreignKey:AccountID"`
	User     User      `json:"user" gorm:"foreignKey:UserID"`
}

// Item Create/Update DTOs
type SaleItemCreateRequest struct {
	SaleID           uint    `json:"sale_id" binding:"required"`
	ProductID        uint    `json:"product_id"`
	ProductName      string  `json:"product_name"`
	Quantity         int     `json:"quantity" binding:"required,min=1"`
	UnitPrice        float64 `json:"unit_price" binding:"required,min=0"`
	Discount         float64 `json:"discount"`
	Tax              float64 `json:"tax"`
	RevenueAccountID uint    `json:"revenue_account_id"`
}

type SaleItemUpdateRequest struct {
	ProductName      *string  `json:"product_name"`
	Quantity         *int     `json:"quantity"`
	UnitPrice        *float64 `json:"unit_price"`
	Discount         *float64 `json:"discount"`
	Tax              *float64 `json:"tax"`
	RevenueAccountID *uint    `json:"revenue_account_id"`
}

// GORM hooks untuk update computed fields (removed duplicate - see line 82)

func (s *Sale) BeforeCreate(tx *gorm.DB) (err error) {
	s.refreshComputedFields()
	return
}

func (s *Sale) BeforeUpdate(tx *gorm.DB) (err error) {
	s.refreshComputedFields()
	return
}

func (s *Sale) AfterCreate(tx *gorm.DB) (err error) {
	s.refreshComputedFields()
	return
}

func ResolveSaleSettlementStatus(status string, paidAmount, outstandingAmount float64) string {
	switch status {
	case SaleStatusCancelled:
		return SaleStatusCancelled
	case SaleStatusDraft, SaleStatusPending, SaleStatusConfirmed:
		return status
	}

	if outstandingAmount <= 0.005 {
		return SaleStatusPaid
	}

	if paidAmount > 0.005 {
		return SaleStatusPartiallyPaid
	}

	return status
}

func (s *Sale) refreshComputedFields() {
	if s == nil {
		return
	}

	s.SubTotal = s.Subtotal
	s.SettlementStatus = ResolveSaleSettlementStatus(s.Status, s.PaidAmount, s.OutstandingAmount)
}

// GORM hooks untuk SaleItem computed fields
func (si *SaleItem) AfterFind(tx *gorm.DB) (err error) {
	// Update legacy fields to match new fields
	si.TotalPrice = si.LineTotal
	si.Discount = si.DiscountAmount
	si.Tax = si.TotalTax
	return
}
