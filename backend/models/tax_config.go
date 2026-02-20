package models

import (
	"time"

	"gorm.io/gorm"
)

// TaxConfig represents configurable tax settings for sales and purchases
// This allows users to configure tax rates from UI instead of hardcoding in backend
type TaxConfig struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Tax Configuration Name
	ConfigName  string `json:"config_name" gorm:"size:100;not null;unique" example:"Default Tax Config"`
	Description string `json:"description" gorm:"type:text" example:"Default tax configuration for Indonesia"`

	// Sales Tax Configuration
	SalesPPNRate      float64 `json:"sales_ppn_rate" gorm:"default:11.0" example:"11.0"`     // PPN Keluaran (%)
	SalesPPh21Rate    float64 `json:"sales_pph21_rate" gorm:"default:0.0" example:"0.0"`     // PPh 21 dipotong customer (%)
	SalesPPh23Rate    float64 `json:"sales_pph23_rate" gorm:"default:0.0" example:"0.0"`     // PPh 23 dipotong customer (%)
	SalesOtherTaxRate float64 `json:"sales_other_tax_rate" gorm:"default:0.0" example:"0.0"` // Pajak lainnya (%)

	// Sales Account Mapping for Taxes
	SalesPPNAccountID      *uint `json:"sales_ppn_account_id" gorm:"index" example:"166"`       // 2103 - PPN Keluaran
	SalesPPh21AccountID    *uint `json:"sales_pph21_account_id" gorm:"index" example:"254"`     // 1114 - PPh 21 Dibayar Dimuka
	SalesPPh23AccountID    *uint `json:"sales_pph23_account_id" gorm:"index" example:"255"`     // 1115 - PPh 23 Dibayar Dimuka
	SalesOtherTaxAccountID *uint `json:"sales_other_tax_account_id" gorm:"index" example:"254"` // 1116 - Potongan Pajak Lainnya

	// Purchase Tax Configuration
	PurchasePPNRate      float64 `json:"purchase_ppn_rate" gorm:"default:11.0" example:"11.0"`     // PPN Masukan (%)
	PurchasePPh21Rate    float64 `json:"purchase_pph21_rate" gorm:"default:0.0" example:"0.0"`     // PPh 21 yang kita potong (%)
	PurchasePPh23Rate    float64 `json:"purchase_pph23_rate" gorm:"default:2.0" example:"2.0"`     // PPh 23 yang kita potong (%)
	PurchasePPh25Rate    float64 `json:"purchase_pph25_rate" gorm:"default:0.0" example:"0.0"`     // PPh 25 (%)
	PurchaseOtherTaxRate float64 `json:"purchase_other_tax_rate" gorm:"default:0.0" example:"0.0"` // Pajak lainnya (%)

	// Purchase Account Mapping for Taxes
	PurchasePPNAccountID      *uint `json:"purchase_ppn_account_id" gorm:"index" example:"164"`       // 1240 - PPN Masukan
	PurchasePPh21AccountID    *uint `json:"purchase_pph21_account_id" gorm:"index" example:"167"`     // 2104 - PPh 21 Yang Dipotong
	PurchasePPh23AccountID    *uint `json:"purchase_pph23_account_id" gorm:"index" example:"168"`     // 2105 - PPh 23 Yang Dipotong
	PurchasePPh25AccountID    *uint `json:"purchase_pph25_account_id" gorm:"index" example:"169"`     // 2106 - PPh 25
	PurchaseOtherTaxAccountID *uint `json:"purchase_other_tax_account_id" gorm:"index" example:"170"` // 2107 - Pemotongan Pajak Lainnya

	// Additional Tax Configurations
	ShippingTaxable   bool   `json:"shipping_taxable" gorm:"default:true"`                                           // Apakah shipping kena pajak
	DiscountBeforeTax bool   `json:"discount_before_tax" gorm:"default:true"`                                        // Discount before atau after tax
	RoundingMethod    string `json:"rounding_method" gorm:"size:20;default:'ROUND_HALF_UP'" example:"ROUND_HALF_UP"` // ROUND_UP, ROUND_DOWN, ROUND_HALF_UP

	// Status
	IsActive  bool `json:"is_active" gorm:"default:true"`
	IsDefault bool `json:"is_default" gorm:"default:false;index"` // Only one config can be default

	// Metadata
	UpdatedBy uint   `json:"updated_by" gorm:"not null;index"`
	Notes     string `json:"notes" gorm:"type:text"`

	// Relations
	UpdatedByUser User `json:"updated_by_user" gorm:"foreignKey:UpdatedBy"`
}

// TableName overrides the table name
func (TaxConfig) TableName() string {
	return "tax_configs"
}

// TaxConfigCreateRequest for creating new tax configuration
type TaxConfigCreateRequest struct {
	ConfigName  string `json:"config_name" binding:"required" example:"Default Tax Config"`
	Description string `json:"description" example:"Default tax configuration for Indonesia"`

	// Sales Tax Rates
	SalesPPNRate      *float64 `json:"sales_ppn_rate" example:"11.0"`
	SalesPPh21Rate    *float64 `json:"sales_pph21_rate" example:"0.0"`
	SalesPPh23Rate    *float64 `json:"sales_pph23_rate" example:"0.0"`
	SalesOtherTaxRate *float64 `json:"sales_other_tax_rate" example:"0.0"`

	// Sales Tax Accounts
	SalesPPNAccountID      *uint `json:"sales_ppn_account_id" example:"166"`
	SalesPPh21AccountID    *uint `json:"sales_pph21_account_id" example:"254"`
	SalesPPh23AccountID    *uint `json:"sales_pph23_account_id" example:"255"`
	SalesOtherTaxAccountID *uint `json:"sales_other_tax_account_id" example:"254"`

	// Purchase Tax Rates
	PurchasePPNRate      *float64 `json:"purchase_ppn_rate" example:"11.0"`
	PurchasePPh21Rate    *float64 `json:"purchase_pph21_rate" example:"0.0"`
	PurchasePPh23Rate    *float64 `json:"purchase_pph23_rate" example:"2.0"`
	PurchasePPh25Rate    *float64 `json:"purchase_pph25_rate" example:"0.0"`
	PurchaseOtherTaxRate *float64 `json:"purchase_other_tax_rate" example:"0.0"`

	// Purchase Tax Accounts
	PurchasePPNAccountID      *uint `json:"purchase_ppn_account_id" example:"164"`
	PurchasePPh21AccountID    *uint `json:"purchase_pph21_account_id" example:"167"`
	PurchasePPh23AccountID    *uint `json:"purchase_pph23_account_id" example:"168"`
	PurchasePPh25AccountID    *uint `json:"purchase_pph25_account_id" example:"169"`
	PurchaseOtherTaxAccountID *uint `json:"purchase_other_tax_account_id" example:"170"`

	// Additional Settings
	ShippingTaxable   *bool   `json:"shipping_taxable" example:"true"`
	DiscountBeforeTax *bool   `json:"discount_before_tax" example:"true"`
	RoundingMethod    *string `json:"rounding_method" example:"ROUND_HALF_UP"`
	IsDefault         *bool   `json:"is_default" example:"false"`
	Notes             string  `json:"notes"`
}

// TaxConfigUpdateRequest for updating tax configuration
type TaxConfigUpdateRequest struct {
	ConfigName  *string `json:"config_name"`
	Description *string `json:"description"`

	// Sales Tax Rates
	SalesPPNRate      *float64 `json:"sales_ppn_rate"`
	SalesPPh21Rate    *float64 `json:"sales_pph21_rate"`
	SalesPPh23Rate    *float64 `json:"sales_pph23_rate"`
	SalesOtherTaxRate *float64 `json:"sales_other_tax_rate"`

	// Sales Tax Accounts
	SalesPPNAccountID      *uint `json:"sales_ppn_account_id"`
	SalesPPh21AccountID    *uint `json:"sales_pph21_account_id"`
	SalesPPh23AccountID    *uint `json:"sales_pph23_account_id"`
	SalesOtherTaxAccountID *uint `json:"sales_other_tax_account_id"`

	// Purchase Tax Rates
	PurchasePPNRate      *float64 `json:"purchase_ppn_rate"`
	PurchasePPh21Rate    *float64 `json:"purchase_pph21_rate"`
	PurchasePPh23Rate    *float64 `json:"purchase_pph23_rate"`
	PurchasePPh25Rate    *float64 `json:"purchase_pph25_rate"`
	PurchaseOtherTaxRate *float64 `json:"purchase_other_tax_rate"`

	// Purchase Tax Accounts
	PurchasePPNAccountID      *uint `json:"purchase_ppn_account_id"`
	PurchasePPh21AccountID    *uint `json:"purchase_pph21_account_id"`
	PurchasePPh23AccountID    *uint `json:"purchase_pph23_account_id"`
	PurchasePPh25AccountID    *uint `json:"purchase_pph25_account_id"`
	PurchaseOtherTaxAccountID *uint `json:"purchase_other_tax_account_id"`

	// Additional Settings
	ShippingTaxable   *bool   `json:"shipping_taxable"`
	DiscountBeforeTax *bool   `json:"discount_before_tax"`
	RoundingMethod    *string `json:"rounding_method"`
	IsActive          *bool   `json:"is_active"`
	IsDefault         *bool   `json:"is_default"`
	Notes             *string `json:"notes"`
}

// TaxConfigResponse for API responses
type TaxConfigResponse struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	ConfigName  string `json:"config_name"`
	Description string `json:"description"`

	// Sales Configuration
	SalesPPNRate      float64 `json:"sales_ppn_rate"`
	SalesPPh21Rate    float64 `json:"sales_pph21_rate"`
	SalesPPh23Rate    float64 `json:"sales_pph23_rate"`
	SalesOtherTaxRate float64 `json:"sales_other_tax_rate"`

	SalesPPNAccountID      *uint `json:"sales_ppn_account_id"`
	SalesPPh21AccountID    *uint `json:"sales_pph21_account_id"`
	SalesPPh23AccountID    *uint `json:"sales_pph23_account_id"`
	SalesOtherTaxAccountID *uint `json:"sales_other_tax_account_id"`

	// Purchase Configuration
	PurchasePPNRate      float64 `json:"purchase_ppn_rate"`
	PurchasePPh21Rate    float64 `json:"purchase_pph21_rate"`
	PurchasePPh23Rate    float64 `json:"purchase_pph23_rate"`
	PurchasePPh25Rate    float64 `json:"purchase_pph25_rate"`
	PurchaseOtherTaxRate float64 `json:"purchase_other_tax_rate"`

	PurchasePPNAccountID      *uint `json:"purchase_ppn_account_id"`
	PurchasePPh21AccountID    *uint `json:"purchase_pph21_account_id"`
	PurchasePPh23AccountID    *uint `json:"purchase_pph23_account_id"`
	PurchasePPh25AccountID    *uint `json:"purchase_pph25_account_id"`
	PurchaseOtherTaxAccountID *uint `json:"purchase_other_tax_account_id"`

	// Additional Settings
	ShippingTaxable   bool   `json:"shipping_taxable"`
	DiscountBeforeTax bool   `json:"discount_before_tax"`
	RoundingMethod    string `json:"rounding_method"`

	// Status
	IsActive  bool   `json:"is_active"`
	IsDefault bool   `json:"is_default"`
	Notes     string `json:"notes"`

	// Metadata
	UpdatedByUser UserResponse `json:"updated_by_user"`
}

// ToResponse converts TaxConfig to TaxConfigResponse
func (t *TaxConfig) ToResponse() TaxConfigResponse {
	return TaxConfigResponse{
		ID:        t.ID,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,

		ConfigName:  t.ConfigName,
		Description: t.Description,

		// Sales
		SalesPPNRate:           t.SalesPPNRate,
		SalesPPh21Rate:         t.SalesPPh21Rate,
		SalesPPh23Rate:         t.SalesPPh23Rate,
		SalesOtherTaxRate:      t.SalesOtherTaxRate,
		SalesPPNAccountID:      t.SalesPPNAccountID,
		SalesPPh21AccountID:    t.SalesPPh21AccountID,
		SalesPPh23AccountID:    t.SalesPPh23AccountID,
		SalesOtherTaxAccountID: t.SalesOtherTaxAccountID,

		// Purchase
		PurchasePPNRate:           t.PurchasePPNRate,
		PurchasePPh21Rate:         t.PurchasePPh21Rate,
		PurchasePPh23Rate:         t.PurchasePPh23Rate,
		PurchasePPh25Rate:         t.PurchasePPh25Rate,
		PurchaseOtherTaxRate:      t.PurchaseOtherTaxRate,
		PurchasePPNAccountID:      t.PurchasePPNAccountID,
		PurchasePPh21AccountID:    t.PurchasePPh21AccountID,
		PurchasePPh23AccountID:    t.PurchasePPh23AccountID,
		PurchasePPh25AccountID:    t.PurchasePPh25AccountID,
		PurchaseOtherTaxAccountID: t.PurchaseOtherTaxAccountID,

		// Additional
		ShippingTaxable:   t.ShippingTaxable,
		DiscountBeforeTax: t.DiscountBeforeTax,
		RoundingMethod:    t.RoundingMethod,
		IsActive:          t.IsActive,
		IsDefault:         t.IsDefault,
		Notes:             t.Notes,

		UpdatedByUser: UserResponse{
			ID:       t.UpdatedByUser.ID,
			Name:     t.UpdatedByUser.GetDisplayName(),
			Username: t.UpdatedByUser.Username,
		},
	}
}

// GetDefaultTaxConfig returns default tax configuration for seeding
func GetDefaultTaxConfig() *TaxConfig {
	ppnAccountID := uint(166)      // 2103 - PPN Keluaran
	pph21AccountID := uint(254)    // 1114 - PPh 21 Dibayar Dimuka
	pph23AccountID := uint(255)    // 1115 - PPh 23 Dibayar Dimuka
	otherTaxAccountID := uint(254) // 1116 - Potongan Pajak Lainnya (fallback to 1114)

	ppnInputAccountID := uint(164)        // 1240 - PPN Masukan
	pph21PayableAccountID := uint(167)    // 2104 - PPh 21 Yang Dipotong
	pph23PayableAccountID := uint(168)    // 2105 - PPh 23 Yang Dipotong
	pph25PayableAccountID := uint(169)    // 2106 - PPh 25
	otherTaxPayableAccountID := uint(170) // 2107 - Pemotongan Pajak Lainnya

	return &TaxConfig{
		ConfigName:  "Indonesia Standard",
		Description: "Standard tax configuration for Indonesia (PPN 11%)",

		// Sales (company memungut pajak dari customer)
		SalesPPNRate:           11.0,
		SalesPPh21Rate:         0.0, // Default 0, customer bisa potong jika applicable
		SalesPPh23Rate:         0.0, // Default 0, customer bisa potong jika applicable
		SalesOtherTaxRate:      0.0,
		SalesPPNAccountID:      &ppnAccountID,
		SalesPPh21AccountID:    &pph21AccountID,
		SalesPPh23AccountID:    &pph23AccountID,
		SalesOtherTaxAccountID: &otherTaxAccountID,

		// Purchase (company memotong pajak vendor)
		PurchasePPNRate:           11.0,
		PurchasePPh21Rate:         0.0, // Default 0, biasanya untuk jasa tertentu
		PurchasePPh23Rate:         2.0, // Standard PPh 23 untuk jasa
		PurchasePPh25Rate:         0.0,
		PurchaseOtherTaxRate:      0.0,
		PurchasePPNAccountID:      &ppnInputAccountID,
		PurchasePPh21AccountID:    &pph21PayableAccountID,
		PurchasePPh23AccountID:    &pph23PayableAccountID,
		PurchasePPh25AccountID:    &pph25PayableAccountID,
		PurchaseOtherTaxAccountID: &otherTaxPayableAccountID,

		// Additional settings
		ShippingTaxable:   true,
		DiscountBeforeTax: true, // Discount apply before tax calculation
		RoundingMethod:    "ROUND_HALF_UP",
		IsActive:          true,
		IsDefault:         true,
	}
}
