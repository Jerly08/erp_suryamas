package models

import (
	"time"
	"gorm.io/gorm"
)

// Company Types
const (
	CompanyTypeService       = "SERVICE"
	CompanyTypeTrading       = "TRADING"
	CompanyTypeManufacturing = "MANUFACTURING"
)

// Settings represents system-wide configuration
type Settings struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
	
	// Company Information
	CompanyName    string `json:"company_name" gorm:"not null"`
	CompanyAddress string `json:"company_address"`
	CompanyPhone   string `json:"company_phone"`
	CompanyEmail   string `json:"company_email"`
	CompanyLogo    string `json:"company_logo"`
	CompanyLogoScale int  `json:"company_logo_scale" gorm:"default:100"`
	CompanyIcon    string `json:"company_icon"`
	PoweredByText  string `json:"powered_by_text" gorm:"column:powered_by_text;type:varchar(160);not null;default:'Powered by'"`
	CompanyWebsite string `json:"company_website"`
	
	// Financial Settings
	Currency         string  `json:"currency" gorm:"default:'IDR'"`
	DateFormat       string  `json:"date_format" gorm:"default:'DD/MM/YYYY'"`
	FiscalYearStart  string  `json:"fiscal_year_start" gorm:"default:'January 1'"`
	TaxNumber        string  `json:"tax_number"`
	DefaultTaxRate   float64 `json:"default_tax_rate" gorm:"default:11"`
	
	// System Configuration
	Language          string `json:"language" gorm:"default:'id'"`
	Timezone          string `json:"timezone" gorm:"default:'Asia/Jakarta'"`
	ThousandSeparator string `json:"thousand_separator" gorm:"default:'.'"`
	DecimalSeparator  string `json:"decimal_separator" gorm:"default:','"`
	DecimalPlaces     int    `json:"decimal_places" gorm:"default:2"`
	
	// Company Type (Standard COA)
	CompanyType string `json:"company_type" gorm:"size:20;default:null"` // SERVICE, TRADING, MANUFACTURING

	
	// Invoice/Document Prefix Settings
	InvoicePrefix      string `json:"invoice_prefix" gorm:"default:'INV'"`
	SalesPrefix        string `json:"sales_prefix" gorm:"default:'SOA'"`
	QuotePrefix        string `json:"quote_prefix" gorm:"default:'QT'"`
	PurchasePrefix     string `json:"purchase_prefix" gorm:"default:'PO'"`
	// Sequence counters for documents
	InvoiceNextNumber  int    `json:"invoice_next_number" gorm:"default:1"`
	SalesNextNumber    int    `json:"sales_next_number" gorm:"default:1"`
	QuoteNextNumber    int    `json:"quote_next_number" gorm:"default:1"`
	PurchaseNextNumber int    `json:"purchase_next_number" gorm:"default:1"`

	// Payment Settings
	PaymentReceivablePrefix string `json:"payment_receivable_prefix" gorm:"default:'RCV'"`
	PaymentPayablePrefix    string `json:"payment_payable_prefix" gorm:"default:'PAY'"`

	// Journal Settings (Unified with static config)
	JournalPrefix         string `json:"journal_prefix" gorm:"default:'JE'"`
	JournalNextNumber     int    `json:"journal_next_number" gorm:"default:1"`
	RequireJournalApproval bool  `json:"require_journal_approval" gorm:"default:false"`
	
	// Additional Settings
	UpdatedBy uint `json:"updated_by"` // User ID who last updated
}

// TableName overrides the table name
func (Settings) TableName() string {
	return "settings"
}

// SettingsResponse for API responses (without sensitive data)
type SettingsResponse struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	
	// Company Information
	CompanyName    string `json:"company_name"`
	CompanyAddress string `json:"company_address"`
	CompanyPhone   string `json:"company_phone"`
	CompanyEmail   string `json:"company_email"`
	CompanyLogo    string `json:"company_logo"`
	CompanyLogoScale int  `json:"company_logo_scale"`
	CompanyIcon    string `json:"company_icon"`
	PoweredByText  string `json:"powered_by_text"`
	CompanyWebsite string `json:"company_website"`
	
	// Financial Settings
	Currency         string  `json:"currency"`
	DateFormat       string  `json:"date_format"`
	FiscalYearStart  string  `json:"fiscal_year_start"`
	TaxNumber        string  `json:"tax_number"`
	DefaultTaxRate   float64 `json:"default_tax_rate"`
	
	// System Configuration
	Language          string `json:"language"`
	Timezone          string `json:"timezone"`
	ThousandSeparator string `json:"thousand_separator"`
	DecimalSeparator  string `json:"decimal_separator"`
	DecimalPlaces     int    `json:"decimal_places"`
	
	// Invoice/Document Prefix Settings
	InvoicePrefix      string `json:"invoice_prefix"`
	SalesPrefix        string `json:"sales_prefix"`
	QuotePrefix        string `json:"quote_prefix"`
	PurchasePrefix     string `json:"purchase_prefix"`
	// Sequence counters for documents
	InvoiceNextNumber  int    `json:"invoice_next_number"`
	SalesNextNumber    int    `json:"sales_next_number"`
	QuoteNextNumber    int    `json:"quote_next_number"`
	PurchaseNextNumber int    `json:"purchase_next_number"`

	// Payment Settings
	PaymentReceivablePrefix string `json:"payment_receivable_prefix"`
	PaymentPayablePrefix    string `json:"payment_payable_prefix"`

	// Journal Settings
	JournalPrefix          string `json:"journal_prefix"`
	JournalNextNumber      int    `json:"journal_next_number"`
	RequireJournalApproval bool   `json:"require_journal_approval"`
	
	// Company Type
	CompanyType string `json:"company_type"`
}

// ToResponse converts Settings to SettingsResponse
func (s *Settings) ToResponse() SettingsResponse {
	return SettingsResponse{
		ID:                 s.ID,
		CreatedAt:          s.CreatedAt,
		UpdatedAt:          s.UpdatedAt,
		CompanyName:        s.CompanyName,
		CompanyAddress:     s.CompanyAddress,
		CompanyPhone:       s.CompanyPhone,
		CompanyEmail:       s.CompanyEmail,
		CompanyLogo:        s.CompanyLogo,
		CompanyLogoScale:   s.CompanyLogoScale,
		CompanyIcon:        s.CompanyIcon,
		PoweredByText:      s.PoweredByText,
		CompanyWebsite:     s.CompanyWebsite,
		Currency:           s.Currency,
		DateFormat:         s.DateFormat,
		FiscalYearStart:    s.FiscalYearStart,
		TaxNumber:          s.TaxNumber,
		DefaultTaxRate:     s.DefaultTaxRate,
		Language:           s.Language,
		Timezone:           s.Timezone,
		ThousandSeparator:  s.ThousandSeparator,
		DecimalSeparator:   s.DecimalSeparator,
		DecimalPlaces:      s.DecimalPlaces,
		CompanyType:        s.CompanyType, // Added field
		InvoicePrefix:      s.InvoicePrefix,
		SalesPrefix:        s.SalesPrefix,
		QuotePrefix:        s.QuotePrefix,
		PurchasePrefix:     s.PurchasePrefix,
		InvoiceNextNumber:  s.InvoiceNextNumber,
		SalesNextNumber:    s.SalesNextNumber,
		QuoteNextNumber:    s.QuoteNextNumber,
		PurchaseNextNumber: s.PurchaseNextNumber,
		PaymentReceivablePrefix: s.PaymentReceivablePrefix,
		PaymentPayablePrefix:    s.PaymentPayablePrefix,
		JournalPrefix:          s.JournalPrefix,
		JournalNextNumber:      s.JournalNextNumber,
		RequireJournalApproval: s.RequireJournalApproval,
	}
}
