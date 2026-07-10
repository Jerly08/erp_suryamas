package models

import (
	"fmt"
	"gorm.io/gorm"
	"time"
)

// TaxAccountSettings represents the tax account configuration for the application
type TaxAccountSettings struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Sales Account Configuration
	SalesReceivableAccountID      uint  `json:"sales_receivable_account_id" gorm:"not null;index"` // Piutang Usaha
	SalesCashAccountID            uint  `json:"sales_cash_account_id" gorm:"not null;index"`       // Kas untuk Penjualan
	SalesBankAccountID            uint  `json:"sales_bank_account_id" gorm:"not null;index"`       // Bank untuk Penjualan
	SalesRevenueAccountID         uint  `json:"sales_revenue_account_id" gorm:"not null;index"`    // Pendapatan Penjualan
	SalesShippingRevenueAccountID *uint `json:"sales_shipping_revenue_account_id" gorm:"index"`    // Pendapatan Ongkir
	SalesOutputVATAccountID       uint  `json:"sales_output_vat_account_id" gorm:"not null;index"` // PPN Keluaran

	// Purchase Account Configuration
	PurchasePayableAccountID  uint `json:"purchase_payable_account_id" gorm:"not null;index"`   // Hutang Usaha
	PurchaseCashAccountID     uint `json:"purchase_cash_account_id" gorm:"not null;index"`      // Kas untuk Pembelian
	PurchaseBankAccountID     uint `json:"purchase_bank_account_id" gorm:"not null;index"`      // Bank untuk Pembelian
	PurchaseInputVATAccountID uint `json:"purchase_input_vat_account_id" gorm:"not null;index"` // PPN Masukan
	PurchaseExpenseAccountID  uint `json:"purchase_expense_account_id" gorm:"not null;index"`   // Default Beban Pembelian

	// Other Tax Accounts
	WithholdingTax21AccountID *uint `json:"withholding_tax21_account_id" gorm:"index"` // PPh 21
	WithholdingTax23AccountID *uint `json:"withholding_tax23_account_id" gorm:"index"` // PPh 23
	WithholdingTax25AccountID *uint `json:"withholding_tax25_account_id" gorm:"index"` // PPh 25
	TaxPayableAccountID       *uint `json:"tax_payable_account_id" gorm:"index"`       // Hutang Pajak

	// Inventory Account (optional, for when purchase is inventory)
	InventoryAccountID *uint `json:"inventory_account_id" gorm:"index"` // Persediaan
	COGSAccountID      *uint `json:"cogs_account_id" gorm:"index"`      // Harga Pokok Penjualan

	// Configuration flags
	IsActive            bool `json:"is_active" gorm:"default:true"`
	ApplyToAllCompanies bool `json:"apply_to_all_companies" gorm:"default:true"`

	// Metadata
	UpdatedBy uint   `json:"updated_by" gorm:"not null;index"`
	Notes     string `json:"notes" gorm:"type:text"`

	// Relations
	SalesReceivableAccount      Account  `json:"sales_receivable_account" gorm:"foreignKey:SalesReceivableAccountID"`
	SalesCashAccount            Account  `json:"sales_cash_account" gorm:"foreignKey:SalesCashAccountID"`
	SalesBankAccount            Account  `json:"sales_bank_account" gorm:"foreignKey:SalesBankAccountID"`
	SalesRevenueAccount         Account  `json:"sales_revenue_account" gorm:"foreignKey:SalesRevenueAccountID"`
	SalesShippingRevenueAccount *Account `json:"sales_shipping_revenue_account" gorm:"foreignKey:SalesShippingRevenueAccountID"`
	SalesOutputVATAccount       Account  `json:"sales_output_vat_account" gorm:"foreignKey:SalesOutputVATAccountID"`

	PurchasePayableAccount  Account `json:"purchase_payable_account" gorm:"foreignKey:PurchasePayableAccountID"`
	PurchaseCashAccount     Account `json:"purchase_cash_account" gorm:"foreignKey:PurchaseCashAccountID"`
	PurchaseBankAccount     Account `json:"purchase_bank_account" gorm:"foreignKey:PurchaseBankAccountID"`
	PurchaseInputVATAccount Account `json:"purchase_input_vat_account" gorm:"foreignKey:PurchaseInputVATAccountID"`
	PurchaseExpenseAccount  Account `json:"purchase_expense_account" gorm:"foreignKey:PurchaseExpenseAccountID"`

	WithholdingTax21Account *Account `json:"withholding_tax21_account" gorm:"foreignKey:WithholdingTax21AccountID"`
	WithholdingTax23Account *Account `json:"withholding_tax23_account" gorm:"foreignKey:WithholdingTax23AccountID"`
	WithholdingTax25Account *Account `json:"withholding_tax25_account" gorm:"foreignKey:WithholdingTax25AccountID"`
	TaxPayableAccount       *Account `json:"tax_payable_account" gorm:"foreignKey:TaxPayableAccountID"`

	InventoryAccount *Account `json:"inventory_account" gorm:"foreignKey:InventoryAccountID"`
	COGSAccount      *Account `json:"cogs_account" gorm:"foreignKey:COGSAccountID"`

	UpdatedByUser User `json:"updated_by_user" gorm:"foreignKey:UpdatedBy"`
}

// TableName overrides the table name
func (TaxAccountSettings) TableName() string {
	return "tax_account_settings"
}

// TaxAccountSettingsCreateRequest for creating new tax account settings
type TaxAccountSettingsCreateRequest struct {
	// Sales Account Configuration
	SalesReceivableAccountID      uint  `json:"sales_receivable_account_id" binding:"required"`
	SalesCashAccountID            uint  `json:"sales_cash_account_id" binding:"required"`
	SalesBankAccountID            uint  `json:"sales_bank_account_id" binding:"required"`
	SalesRevenueAccountID         uint  `json:"sales_revenue_account_id" binding:"required"`
	SalesShippingRevenueAccountID *uint `json:"sales_shipping_revenue_account_id"`
	SalesOutputVATAccountID       uint  `json:"sales_output_vat_account_id" binding:"required"`

	// Purchase Account Configuration
	PurchasePayableAccountID  uint `json:"purchase_payable_account_id" binding:"required"`
	PurchaseCashAccountID     uint `json:"purchase_cash_account_id" binding:"required"`
	PurchaseBankAccountID     uint `json:"purchase_bank_account_id" binding:"required"`
	PurchaseInputVATAccountID uint `json:"purchase_input_vat_account_id" binding:"required"`
	PurchaseExpenseAccountID  uint `json:"purchase_expense_account_id" binding:"required"`

	// Other Tax Accounts (optional)
	WithholdingTax21AccountID *uint `json:"withholding_tax21_account_id"`
	WithholdingTax23AccountID *uint `json:"withholding_tax23_account_id"`
	WithholdingTax25AccountID *uint `json:"withholding_tax25_account_id"`
	TaxPayableAccountID       *uint `json:"tax_payable_account_id"`

	// Inventory Account (optional)
	InventoryAccountID *uint `json:"inventory_account_id"`
	COGSAccountID      *uint `json:"cogs_account_id"`

	// Configuration flags
	ApplyToAllCompanies *bool  `json:"apply_to_all_companies"`
	Notes               string `json:"notes"`
}

// TaxAccountSettingsUpdateRequest for updating tax account settings
type TaxAccountSettingsUpdateRequest struct {
	// Sales Account Configuration
	SalesReceivableAccountID      *uint `json:"sales_receivable_account_id"`
	SalesCashAccountID            *uint `json:"sales_cash_account_id"`
	SalesBankAccountID            *uint `json:"sales_bank_account_id"`
	SalesRevenueAccountID         *uint `json:"sales_revenue_account_id"`
	SalesShippingRevenueAccountID *uint `json:"sales_shipping_revenue_account_id"`
	SalesOutputVATAccountID       *uint `json:"sales_output_vat_account_id"`

	// Purchase Account Configuration
	PurchasePayableAccountID  *uint `json:"purchase_payable_account_id"`
	PurchaseCashAccountID     *uint `json:"purchase_cash_account_id"`
	PurchaseBankAccountID     *uint `json:"purchase_bank_account_id"`
	PurchaseInputVATAccountID *uint `json:"purchase_input_vat_account_id"`
	PurchaseExpenseAccountID  *uint `json:"purchase_expense_account_id"`

	// Other Tax Accounts (optional)
	WithholdingTax21AccountID *uint `json:"withholding_tax21_account_id"`
	WithholdingTax23AccountID *uint `json:"withholding_tax23_account_id"`
	WithholdingTax25AccountID *uint `json:"withholding_tax25_account_id"`
	TaxPayableAccountID       *uint `json:"tax_payable_account_id"`

	// Inventory Account (optional)
	InventoryAccountID *uint `json:"inventory_account_id"`
	COGSAccountID      *uint `json:"cogs_account_id"`

	// Configuration flags
	IsActive            *bool   `json:"is_active"`
	ApplyToAllCompanies *bool   `json:"apply_to_all_companies"`
	Notes               *string `json:"notes"`
}

// TaxAccountSettingsResponse for API responses
type TaxAccountSettingsResponse struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Sales Account Configuration
	SalesReceivableAccount      AccountResponse  `json:"sales_receivable_account"`
	SalesCashAccount            AccountResponse  `json:"sales_cash_account"`
	SalesBankAccount            AccountResponse  `json:"sales_bank_account"`
	SalesRevenueAccount         AccountResponse  `json:"sales_revenue_account"`
	SalesShippingRevenueAccount *AccountResponse `json:"sales_shipping_revenue_account"`
	SalesOutputVATAccount       AccountResponse  `json:"sales_output_vat_account"`

	// Purchase Account Configuration
	PurchasePayableAccount  AccountResponse `json:"purchase_payable_account"`
	PurchaseCashAccount     AccountResponse `json:"purchase_cash_account"`
	PurchaseBankAccount     AccountResponse `json:"purchase_bank_account"`
	PurchaseInputVATAccount AccountResponse `json:"purchase_input_vat_account"`
	PurchaseExpenseAccount  AccountResponse `json:"purchase_expense_account"`

	// Other Tax Accounts (optional)
	WithholdingTax21Account *AccountResponse `json:"withholding_tax21_account"`
	WithholdingTax23Account *AccountResponse `json:"withholding_tax23_account"`
	WithholdingTax25Account *AccountResponse `json:"withholding_tax25_account"`
	TaxPayableAccount       *AccountResponse `json:"tax_payable_account"`

	// Inventory Account (optional)
	InventoryAccount *AccountResponse `json:"inventory_account"`
	COGSAccount      *AccountResponse `json:"cogs_account"`

	// Configuration flags
	IsActive            bool   `json:"is_active"`
	ApplyToAllCompanies bool   `json:"apply_to_all_companies"`
	Notes               string `json:"notes"`

	// Metadata
	UpdatedByUser UserResponse `json:"updated_by_user"`
}

// Note: AccountResponse and UserResponse are defined in swagger_models.go

// ToResponse converts TaxAccountSettings to TaxAccountSettingsResponse
func (t *TaxAccountSettings) ToResponse() TaxAccountSettingsResponse {
	response := TaxAccountSettingsResponse{
		ID:        t.ID,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,

		SalesReceivableAccount: AccountResponse{
			ID:       t.SalesReceivableAccount.ID,
			Code:     t.SalesReceivableAccount.Code,
			Name:     t.SalesReceivableAccount.Name,
			Type:     t.SalesReceivableAccount.Type,
			Category: t.SalesReceivableAccount.Category,
			IsActive: t.SalesReceivableAccount.IsActive,
		},
		SalesCashAccount: AccountResponse{
			ID:       t.SalesCashAccount.ID,
			Code:     t.SalesCashAccount.Code,
			Name:     t.SalesCashAccount.Name,
			Type:     t.SalesCashAccount.Type,
			Category: t.SalesCashAccount.Category,
			IsActive: t.SalesCashAccount.IsActive,
		},
		SalesBankAccount: AccountResponse{
			ID:       t.SalesBankAccount.ID,
			Code:     t.SalesBankAccount.Code,
			Name:     t.SalesBankAccount.Name,
			Type:     t.SalesBankAccount.Type,
			Category: t.SalesBankAccount.Category,
			IsActive: t.SalesBankAccount.IsActive,
		},
		SalesRevenueAccount: AccountResponse{
			ID:       t.SalesRevenueAccount.ID,
			Code:     t.SalesRevenueAccount.Code,
			Name:     t.SalesRevenueAccount.Name,
			Type:     t.SalesRevenueAccount.Type,
			Category: t.SalesRevenueAccount.Category,
			IsActive: t.SalesRevenueAccount.IsActive,
		},
		SalesOutputVATAccount: AccountResponse{
			ID:       t.SalesOutputVATAccount.ID,
			Code:     t.SalesOutputVATAccount.Code,
			Name:     t.SalesOutputVATAccount.Name,
			Type:     t.SalesOutputVATAccount.Type,
			Category: t.SalesOutputVATAccount.Category,
			IsActive: t.SalesOutputVATAccount.IsActive,
		},

		PurchasePayableAccount: AccountResponse{
			ID:       t.PurchasePayableAccount.ID,
			Code:     t.PurchasePayableAccount.Code,
			Name:     t.PurchasePayableAccount.Name,
			Type:     t.PurchasePayableAccount.Type,
			Category: t.PurchasePayableAccount.Category,
			IsActive: t.PurchasePayableAccount.IsActive,
		},
		PurchaseCashAccount: AccountResponse{
			ID:       t.PurchaseCashAccount.ID,
			Code:     t.PurchaseCashAccount.Code,
			Name:     t.PurchaseCashAccount.Name,
			Type:     t.PurchaseCashAccount.Type,
			Category: t.PurchaseCashAccount.Category,
			IsActive: t.PurchaseCashAccount.IsActive,
		},
		PurchaseBankAccount: AccountResponse{
			ID:       t.PurchaseBankAccount.ID,
			Code:     t.PurchaseBankAccount.Code,
			Name:     t.PurchaseBankAccount.Name,
			Type:     t.PurchaseBankAccount.Type,
			Category: t.PurchaseBankAccount.Category,
			IsActive: t.PurchaseBankAccount.IsActive,
		},
		PurchaseInputVATAccount: AccountResponse{
			ID:       t.PurchaseInputVATAccount.ID,
			Code:     t.PurchaseInputVATAccount.Code,
			Name:     t.PurchaseInputVATAccount.Name,
			Type:     t.PurchaseInputVATAccount.Type,
			Category: t.PurchaseInputVATAccount.Category,
			IsActive: t.PurchaseInputVATAccount.IsActive,
		},
		PurchaseExpenseAccount: AccountResponse{
			ID:       t.PurchaseExpenseAccount.ID,
			Code:     t.PurchaseExpenseAccount.Code,
			Name:     t.PurchaseExpenseAccount.Name,
			Type:     t.PurchaseExpenseAccount.Type,
			Category: t.PurchaseExpenseAccount.Category,
			IsActive: t.PurchaseExpenseAccount.IsActive,
		},

		IsActive:            t.IsActive,
		ApplyToAllCompanies: t.ApplyToAllCompanies,
		Notes:               t.Notes,
		UpdatedByUser: UserResponse{
			ID:       t.UpdatedByUser.ID,
			Name:     t.UpdatedByUser.GetDisplayName(),
			Username: t.UpdatedByUser.Username,
		},
	}

	// Handle optional accounts
	if t.WithholdingTax21Account != nil {
		response.WithholdingTax21Account = &AccountResponse{
			ID:       t.WithholdingTax21Account.ID,
			Code:     t.WithholdingTax21Account.Code,
			Name:     t.WithholdingTax21Account.Name,
			Type:     t.WithholdingTax21Account.Type,
			Category: t.WithholdingTax21Account.Category,
			IsActive: t.WithholdingTax21Account.IsActive,
		}
	}

	if t.SalesShippingRevenueAccount != nil {
		response.SalesShippingRevenueAccount = &AccountResponse{
			ID:       t.SalesShippingRevenueAccount.ID,
			Code:     t.SalesShippingRevenueAccount.Code,
			Name:     t.SalesShippingRevenueAccount.Name,
			Type:     t.SalesShippingRevenueAccount.Type,
			Category: t.SalesShippingRevenueAccount.Category,
			IsActive: t.SalesShippingRevenueAccount.IsActive,
		}
	}

	if t.WithholdingTax23Account != nil {
		response.WithholdingTax23Account = &AccountResponse{
			ID:       t.WithholdingTax23Account.ID,
			Code:     t.WithholdingTax23Account.Code,
			Name:     t.WithholdingTax23Account.Name,
			Type:     t.WithholdingTax23Account.Type,
			Category: t.WithholdingTax23Account.Category,
			IsActive: t.WithholdingTax23Account.IsActive,
		}
	}

	if t.WithholdingTax25Account != nil {
		response.WithholdingTax25Account = &AccountResponse{
			ID:       t.WithholdingTax25Account.ID,
			Code:     t.WithholdingTax25Account.Code,
			Name:     t.WithholdingTax25Account.Name,
			Type:     t.WithholdingTax25Account.Type,
			Category: t.WithholdingTax25Account.Category,
			IsActive: t.WithholdingTax25Account.IsActive,
		}
	}

	if t.TaxPayableAccount != nil {
		response.TaxPayableAccount = &AccountResponse{
			ID:       t.TaxPayableAccount.ID,
			Code:     t.TaxPayableAccount.Code,
			Name:     t.TaxPayableAccount.Name,
			Type:     t.TaxPayableAccount.Type,
			Category: t.TaxPayableAccount.Category,
			IsActive: t.TaxPayableAccount.IsActive,
		}
	}

	if t.InventoryAccount != nil {
		response.InventoryAccount = &AccountResponse{
			ID:       t.InventoryAccount.ID,
			Code:     t.InventoryAccount.Code,
			Name:     t.InventoryAccount.Name,
			Type:     t.InventoryAccount.Type,
			Category: t.InventoryAccount.Category,
			IsActive: t.InventoryAccount.IsActive,
		}
	}

	if t.COGSAccount != nil {
		response.COGSAccount = &AccountResponse{
			ID:       t.COGSAccount.ID,
			Code:     t.COGSAccount.Code,
			Name:     t.COGSAccount.Name,
			Type:     t.COGSAccount.Type,
			Category: t.COGSAccount.Category,
			IsActive: t.COGSAccount.IsActive,
		}
	}

	return response
}

// ValidateAccountSettings validates tax account settings before saving
func (t *TaxAccountSettings) ValidateAccountSettings() error {
	// Basic validation - ensure required accounts are set
	if t.SalesReceivableAccountID == 0 {
		return fmt.Errorf("sales receivable account is required")
	}
	if t.SalesCashAccountID == 0 {
		return fmt.Errorf("sales cash account is required")
	}
	if t.SalesBankAccountID == 0 {
		return fmt.Errorf("sales bank account is required")
	}
	if t.SalesRevenueAccountID == 0 {
		return fmt.Errorf("sales revenue account is required")
	}
	if t.SalesOutputVATAccountID == 0 {
		return fmt.Errorf("sales output VAT account is required")
	}
	if t.PurchasePayableAccountID == 0 {
		return fmt.Errorf("purchase payable account is required")
	}
	if t.PurchaseCashAccountID == 0 {
		return fmt.Errorf("purchase cash account is required")
	}
	if t.PurchaseBankAccountID == 0 {
		return fmt.Errorf("purchase bank account is required")
	}
	if t.PurchaseInputVATAccountID == 0 {
		return fmt.Errorf("purchase input VAT account is required")
	}
	if t.PurchaseExpenseAccountID == 0 {
		return fmt.Errorf("purchase expense account is required")
	}

	return nil
}

// GetDefaultTaxAccountSettings returns default account settings based on existing accounts
func GetDefaultTaxAccountSettings() *TaxAccountSettings {
	return &TaxAccountSettings{
		// Sales defaults
		SalesReceivableAccountID: 1201, // Piutang Usaha
		SalesCashAccountID:       1101, // Kas
		SalesBankAccountID:       1102, // Bank
		SalesRevenueAccountID:    4101, // Pendapatan Penjualan
		SalesOutputVATAccountID:  2103, // PPN Keluaran

		// Purchase defaults
		PurchasePayableAccountID:  2001, // Hutang Usaha
		PurchaseCashAccountID:     1101, // Kas
		PurchaseBankAccountID:     1102, // Bank
		PurchaseInputVATAccountID: 1240, // PPN Masukan (standardized)
		PurchaseExpenseAccountID:  6001, // Beban Operasional

		// Flags
		IsActive:            true,
		ApplyToAllCompanies: true,
	}
}
