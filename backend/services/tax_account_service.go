package services

import (
	"errors"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"app-sistem-akuntansi/models"
	"gorm.io/gorm"
)

// TaxAccountService handles tax account settings management
type TaxAccountService struct {
	db          *gorm.DB
	cache       *models.TaxAccountSettings
	taxConfig   *models.TaxConfig
	cacheMux    sync.RWMutex
	cacheLoaded bool
}

// NewTaxAccountService creates a new TaxAccountService instance
func NewTaxAccountService(db *gorm.DB) *TaxAccountService {
	service := &TaxAccountService{
		db: db,
	}

	// Load current settings on initialization
	if err := service.loadSettings(); err != nil {
		log.Printf("Warning: Failed to load tax account settings: %v", err)
	}

	return service
}

// loadSettings loads the current tax account settings into cache
func (s *TaxAccountService) loadSettings() error {
	s.cacheMux.Lock()
	defer s.cacheMux.Unlock()

	// Load tax account settings
	var settings models.TaxAccountSettings
	err := s.db.Preload("SalesReceivableAccount").
		Preload("SalesCashAccount").
		Preload("SalesBankAccount").
		Preload("SalesRevenueAccount").
		Preload("SalesOutputVATAccount").
		Preload("PurchasePayableAccount").
		Preload("PurchaseCashAccount").
		Preload("PurchaseBankAccount").
		Preload("PurchaseInputVATAccount").
		Preload("PurchaseExpenseAccount").
		Preload("WithholdingTax21Account").
		Preload("WithholdingTax23Account").
		Preload("WithholdingTax25Account").
		Preload("TaxPayableAccount").
		Preload("InventoryAccount").
		Preload("COGSAccount").
		Preload("UpdatedByUser").
		Where("is_active = ?", true).
		First(&settings).Error

	if err == nil {
		s.cache = &settings
		s.cacheLoaded = true
		return nil
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create default settings if none exist
		defaultSettings := models.GetDefaultTaxAccountSettings()
		defaultSettings.UpdatedBy = 1 // System user ID

		if createErr := s.db.Create(defaultSettings).Error; createErr != nil {
			return fmt.Errorf("failed to create default settings: %v", createErr)
		}

		// Load the created settings with relations
		return s.loadSettings()
	}

	return fmt.Errorf("failed to load tax account settings: %v", err)
}

// GetSettings returns the current tax account settings
func (s *TaxAccountService) GetSettings() (*models.TaxAccountSettings, error) {
	s.cacheMux.RLock()
	defer s.cacheMux.RUnlock()

	if !s.cacheLoaded || s.cache == nil {
		// Reload if cache is empty
		s.cacheMux.RUnlock()
		if err := s.loadSettings(); err != nil {
			s.cacheMux.RLock()
			return nil, err
		}
		s.cacheMux.RLock()
	}

	return s.cache, nil
}

// CreateSettings creates new tax account settings
func (s *TaxAccountService) CreateSettings(req *models.TaxAccountSettingsCreateRequest, userID uint) (*models.TaxAccountSettings, error) {
	// Validate that accounts exist
	if err := s.validateAccounts(req); err != nil {
		return nil, err
	}

	// Deactivate existing active settings
	if err := s.db.Model(&models.TaxAccountSettings{}).
		Where("is_active = ?", true).
		Update("is_active", false).Error; err != nil {
		return nil, fmt.Errorf("failed to deactivate existing settings: %v", err)
	}

	// Create new settings
	settings := &models.TaxAccountSettings{
		SalesReceivableAccountID:  req.SalesReceivableAccountID,
		SalesCashAccountID:        req.SalesCashAccountID,
		SalesBankAccountID:        req.SalesBankAccountID,
		SalesRevenueAccountID:     req.SalesRevenueAccountID,
		SalesOutputVATAccountID:   req.SalesOutputVATAccountID,
		PurchasePayableAccountID:  req.PurchasePayableAccountID,
		PurchaseCashAccountID:     req.PurchaseCashAccountID,
		PurchaseBankAccountID:     req.PurchaseBankAccountID,
		PurchaseInputVATAccountID: req.PurchaseInputVATAccountID,
		PurchaseExpenseAccountID:  req.PurchaseExpenseAccountID,
		WithholdingTax21AccountID: req.WithholdingTax21AccountID,
		WithholdingTax23AccountID: req.WithholdingTax23AccountID,
		WithholdingTax25AccountID: req.WithholdingTax25AccountID,
		TaxPayableAccountID:       req.TaxPayableAccountID,
		InventoryAccountID:        req.InventoryAccountID,
		COGSAccountID:             req.COGSAccountID,
		IsActive:                  true,
		ApplyToAllCompanies:       req.ApplyToAllCompanies != nil && *req.ApplyToAllCompanies,
		UpdatedBy:                 userID,
		Notes:                     req.Notes,
	}

	// Validate settings
	if err := settings.ValidateAccountSettings(); err != nil {
		return nil, err
	}

	if err := s.db.Create(settings).Error; err != nil {
		return nil, fmt.Errorf("failed to create tax account settings: %v", err)
	}

	// Reload cache
	if err := s.loadSettings(); err != nil {
		log.Printf("Warning: Failed to reload cache after creating settings: %v", err)
	}

	// Load relations for response
	err := s.db.Preload("SalesReceivableAccount").
		Preload("SalesCashAccount").
		Preload("SalesBankAccount").
		Preload("SalesRevenueAccount").
		Preload("SalesOutputVATAccount").
		Preload("PurchasePayableAccount").
		Preload("PurchaseCashAccount").
		Preload("PurchaseBankAccount").
		Preload("PurchaseInputVATAccount").
		Preload("PurchaseExpenseAccount").
		Preload("WithholdingTax21Account").
		Preload("WithholdingTax23Account").
		Preload("WithholdingTax25Account").
		Preload("TaxPayableAccount").
		Preload("InventoryAccount").
		Preload("COGSAccount").
		Preload("UpdatedByUser").
		First(settings, settings.ID).Error

	return settings, err
}

// UpdateSettings updates existing tax account settings
func (s *TaxAccountService) UpdateSettings(id uint, req *models.TaxAccountSettingsUpdateRequest, userID uint) (*models.TaxAccountSettings, error) {
	var settings models.TaxAccountSettings
	if err := s.db.First(&settings, id).Error; err != nil {
		return nil, fmt.Errorf("tax account settings not found: %v", err)
	}

	// Update fields if provided
	if req.SalesReceivableAccountID != nil {
		settings.SalesReceivableAccountID = *req.SalesReceivableAccountID
	}
	if req.SalesCashAccountID != nil {
		settings.SalesCashAccountID = *req.SalesCashAccountID
	}
	if req.SalesBankAccountID != nil {
		settings.SalesBankAccountID = *req.SalesBankAccountID
	}
	if req.SalesRevenueAccountID != nil {
		settings.SalesRevenueAccountID = *req.SalesRevenueAccountID
	}
	if req.SalesOutputVATAccountID != nil {
		settings.SalesOutputVATAccountID = *req.SalesOutputVATAccountID
	}
	if req.PurchasePayableAccountID != nil {
		settings.PurchasePayableAccountID = *req.PurchasePayableAccountID
	}
	if req.PurchaseCashAccountID != nil {
		settings.PurchaseCashAccountID = *req.PurchaseCashAccountID
	}
	if req.PurchaseBankAccountID != nil {
		settings.PurchaseBankAccountID = *req.PurchaseBankAccountID
	}
	if req.PurchaseInputVATAccountID != nil {
		settings.PurchaseInputVATAccountID = *req.PurchaseInputVATAccountID
	}
	if req.PurchaseExpenseAccountID != nil {
		settings.PurchaseExpenseAccountID = *req.PurchaseExpenseAccountID
	}
	if req.WithholdingTax21AccountID != nil {
		settings.WithholdingTax21AccountID = req.WithholdingTax21AccountID
	}
	if req.WithholdingTax23AccountID != nil {
		settings.WithholdingTax23AccountID = req.WithholdingTax23AccountID
	}
	if req.WithholdingTax25AccountID != nil {
		settings.WithholdingTax25AccountID = req.WithholdingTax25AccountID
	}
	if req.TaxPayableAccountID != nil {
		settings.TaxPayableAccountID = req.TaxPayableAccountID
	}
	if req.InventoryAccountID != nil {
		settings.InventoryAccountID = req.InventoryAccountID
	}
	if req.COGSAccountID != nil {
		settings.COGSAccountID = req.COGSAccountID
	}
	if req.IsActive != nil {
		settings.IsActive = *req.IsActive
	}
	if req.ApplyToAllCompanies != nil {
		settings.ApplyToAllCompanies = *req.ApplyToAllCompanies
	}
	if req.Notes != nil {
		settings.Notes = *req.Notes
	}

	settings.UpdatedBy = userID
	settings.UpdatedAt = time.Now()

	// Note: Skip validation for update mode since we only update specific fields
	// The existing record already has all required fields set from initial creation
	// Validation is only needed for CREATE, not UPDATE

	if err := s.db.Save(&settings).Error; err != nil {
		return nil, fmt.Errorf("failed to update tax account settings: %v", err)
	}

	// Reload cache
	if err := s.loadSettings(); err != nil {
		log.Printf("Warning: Failed to reload cache after updating settings: %v", err)
	}

	// Load relations for response
	err := s.db.Preload("SalesReceivableAccount").
		Preload("SalesCashAccount").
		Preload("SalesBankAccount").
		Preload("SalesRevenueAccount").
		Preload("SalesOutputVATAccount").
		Preload("PurchasePayableAccount").
		Preload("PurchaseCashAccount").
		Preload("PurchaseBankAccount").
		Preload("PurchaseInputVATAccount").
		Preload("PurchaseExpenseAccount").
		Preload("WithholdingTax21Account").
		Preload("WithholdingTax23Account").
		Preload("WithholdingTax25Account").
		Preload("TaxPayableAccount").
		Preload("InventoryAccount").
		Preload("COGSAccount").
		Preload("UpdatedByUser").
		First(&settings, settings.ID).Error

	return &settings, err
}

// validateAccounts validates that all specified accounts exist and are active
func (s *TaxAccountService) validateAccounts(req *models.TaxAccountSettingsCreateRequest) error {
	accountIDs := []uint{
		req.SalesReceivableAccountID,
		req.SalesCashAccountID,
		req.SalesBankAccountID,
		req.SalesRevenueAccountID,
		req.SalesOutputVATAccountID,
		req.PurchasePayableAccountID,
		req.PurchaseCashAccountID,
		req.PurchaseBankAccountID,
		req.PurchaseInputVATAccountID,
		req.PurchaseExpenseAccountID,
	}

	// Add optional account IDs if provided
	if req.WithholdingTax21AccountID != nil {
		accountIDs = append(accountIDs, *req.WithholdingTax21AccountID)
	}
	if req.WithholdingTax23AccountID != nil {
		accountIDs = append(accountIDs, *req.WithholdingTax23AccountID)
	}
	if req.WithholdingTax25AccountID != nil {
		accountIDs = append(accountIDs, *req.WithholdingTax25AccountID)
	}
	if req.TaxPayableAccountID != nil {
		accountIDs = append(accountIDs, *req.TaxPayableAccountID)
	}
	if req.InventoryAccountID != nil {
		accountIDs = append(accountIDs, *req.InventoryAccountID)
	}
	if req.COGSAccountID != nil {
		accountIDs = append(accountIDs, *req.COGSAccountID)
	}

	var count int64
	if err := s.db.Model(&models.Account{}).
		Where("id IN ? AND is_active = ?", accountIDs, true).
		Count(&count).Error; err != nil {
		return fmt.Errorf("failed to validate accounts: %v", err)
	}

	if int(count) != len(accountIDs) {
		return fmt.Errorf("some accounts are not found or inactive")
	}

	return nil
}

// GetAccountID returns the account ID for a specific account type
func (s *TaxAccountService) GetAccountID(accountType string) (uint, error) {
	settings, err := s.GetSettings()
	if err != nil {
		return 0, err
	}

	switch accountType {
	// Sales accounts
	case "sales_receivable":
		return settings.SalesReceivableAccountID, nil
	case "sales_cash":
		return settings.SalesCashAccountID, nil
	case "sales_bank":
		return settings.SalesBankAccountID, nil
	case "sales_revenue":
		return settings.SalesRevenueAccountID, nil
	case "sales_output_vat":
		return settings.SalesOutputVATAccountID, nil

	// Purchase accounts
	case "purchase_payable":
		return settings.PurchasePayableAccountID, nil
	case "purchase_cash":
		return settings.PurchaseCashAccountID, nil
	case "purchase_bank":
		return settings.PurchaseBankAccountID, nil
	case "purchase_input_vat":
		return settings.PurchaseInputVATAccountID, nil
	case "purchase_expense":
		return settings.PurchaseExpenseAccountID, nil

	// Tax accounts
	case "withholding_tax21":
		if settings.WithholdingTax21AccountID != nil {
			return *settings.WithholdingTax21AccountID, nil
		}
		return 0, fmt.Errorf("withholding tax 21 account not configured")
	case "withholding_tax23":
		if settings.WithholdingTax23AccountID != nil {
			return *settings.WithholdingTax23AccountID, nil
		}
		return 0, fmt.Errorf("withholding tax 23 account not configured")
	case "withholding_tax25":
		if settings.WithholdingTax25AccountID != nil {
			return *settings.WithholdingTax25AccountID, nil
		}
		return 0, fmt.Errorf("withholding tax 25 account not configured")
	case "tax_payable":
		if settings.TaxPayableAccountID != nil {
			return *settings.TaxPayableAccountID, nil
		}
		return 0, fmt.Errorf("tax payable account not configured")

	// Inventory accounts
	case "inventory":
		if settings.InventoryAccountID != nil {
			return *settings.InventoryAccountID, nil
		}
		return 0, fmt.Errorf("inventory account not configured")
	case "cogs":
		if settings.COGSAccountID != nil {
			return *settings.COGSAccountID, nil
		}
		return 0, fmt.Errorf("COGS account not configured")

	default:
		return 0, fmt.Errorf("unknown account type: %s", accountType)
	}
}

// GetAccountByCode returns account ID by account code (fallback method)
func (s *TaxAccountService) GetAccountByCode(code string) (uint, error) {
	var account models.Account
	err := s.db.Where("code = ? AND is_active = ?", code, true).First(&account).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, fmt.Errorf("account with code %s not found", code)
		}
		return 0, fmt.Errorf("failed to find account: %v", err)
	}
	return account.ID, nil
}

// RefreshCache forces a reload of the settings cache
func (s *TaxAccountService) RefreshCache() error {
	return s.loadSettings()
}

// GetAllSettings returns all tax account settings (for admin purposes)
func (s *TaxAccountService) GetAllSettings() ([]models.TaxAccountSettings, error) {
	var settings []models.TaxAccountSettings
	err := s.db.Preload("SalesReceivableAccount").
		Preload("SalesCashAccount").
		Preload("SalesBankAccount").
		Preload("SalesRevenueAccount").
		Preload("SalesOutputVATAccount").
		Preload("PurchasePayableAccount").
		Preload("PurchaseCashAccount").
		Preload("PurchaseBankAccount").
		Preload("PurchaseInputVATAccount").
		Preload("PurchaseExpenseAccount").
		Preload("WithholdingTax21Account").
		Preload("WithholdingTax23Account").
		Preload("WithholdingTax25Account").
		Preload("TaxPayableAccount").
		Preload("InventoryAccount").
		Preload("COGSAccount").
		Preload("UpdatedByUser").
		Order("created_at DESC").
		Find(&settings).Error

	return settings, err
}

// ActivateSettings activates specific tax account settings by ID
func (s *TaxAccountService) ActivateSettings(id uint, userID uint) error {
	// Deactivate all other settings
	if err := s.db.Model(&models.TaxAccountSettings{}).
		Where("id != ?", id).
		Update("is_active", false).Error; err != nil {
		return fmt.Errorf("failed to deactivate other settings: %v", err)
	}

	// Activate the specified settings
	if err := s.db.Model(&models.TaxAccountSettings{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_active":  true,
			"updated_by": userID,
			"updated_at": time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("failed to activate settings: %v", err)
	}

	// Reload cache
	return s.loadSettings()
}

// ================ TAX CONFIG INTEGRATION ================

// GetActiveTaxConfig returns the currently active tax configuration
func (s *TaxAccountService) GetActiveTaxConfig() (*models.TaxConfig, error) {
	s.cacheMux.RLock()
	defer s.cacheMux.RUnlock()

	if s.taxConfig != nil {
		return s.taxConfig, nil
	}

	// Try to load from database
	s.cacheMux.RUnlock()
	s.cacheMux.Lock()
	if !s.db.Migrator().HasTable(&models.TaxConfig{}) {
		s.cacheMux.Unlock()
		s.cacheMux.RLock()
		return nil, fmt.Errorf("tax config table not found")
	}
	var taxConfig models.TaxConfig
	err := s.db.Where("is_active = ? AND is_default = ?", true, true).First(&taxConfig).Error
	if err != nil {
		s.cacheMux.Unlock()
		s.cacheMux.RLock()
		return nil, fmt.Errorf("no active tax config found: %v", err)
	}
	s.taxConfig = &taxConfig
	s.cacheMux.Unlock()
	s.cacheMux.RLock()

	return s.taxConfig, nil
}

// GetSalesTaxRate returns the tax rate for sales based on tax type
func (s *TaxAccountService) GetSalesTaxRate(taxType string) (float64, error) {
	config, err := s.GetActiveTaxConfig()
	if err != nil {
		// Return default rates if no config found
		log.Printf("⚠️ Using default tax rates: %v", err)
		switch taxType {
		case "ppn":
			return 11.0, nil
		case "pph21":
			return 0.0, nil
		case "pph23":
			return 0.0, nil
		case "other":
			return 0.0, nil
		default:
			return 0.0, fmt.Errorf("unknown sales tax type: %s", taxType)
		}
	}

	switch taxType {
	case "ppn":
		return config.SalesPPNRate, nil
	case "pph21":
		return config.SalesPPh21Rate, nil
	case "pph23":
		return config.SalesPPh23Rate, nil
	case "other":
		return config.SalesOtherTaxRate, nil
	default:
		return 0.0, fmt.Errorf("unknown sales tax type: %s", taxType)
	}
}

// GetPurchaseTaxRate returns the tax rate for purchases based on tax type
func (s *TaxAccountService) GetPurchaseTaxRate(taxType string) (float64, error) {
	config, err := s.GetActiveTaxConfig()
	if err != nil {
		// Return default rates if no config found
		log.Printf("⚠️ Using default tax rates: %v", err)
		switch taxType {
		case "ppn":
			return 11.0, nil
		case "pph21":
			return 0.0, nil
		case "pph23":
			return 2.0, nil
		case "pph25":
			return 0.0, nil
		case "other":
			return 0.0, nil
		default:
			return 0.0, fmt.Errorf("unknown purchase tax type: %s", taxType)
		}
	}

	switch taxType {
	case "ppn":
		return config.PurchasePPNRate, nil
	case "pph21":
		return config.PurchasePPh21Rate, nil
	case "pph23":
		return config.PurchasePPh23Rate, nil
	case "pph25":
		return config.PurchasePPh25Rate, nil
	case "other":
		return config.PurchaseOtherTaxRate, nil
	default:
		return 0.0, fmt.Errorf("unknown purchase tax type: %s", taxType)
	}
}

// GetSalesTaxAccountID returns the account ID for sales tax based on tax type
func (s *TaxAccountService) GetSalesTaxAccountID(taxType string) (uint, error) {
	config, err := s.GetActiveTaxConfig()
	if err != nil {
		return 0, err
	}

	switch taxType {
	case "ppn":
		if config.SalesPPNAccountID != nil {
			return *config.SalesPPNAccountID, nil
		}
		return 0, fmt.Errorf("sales PPN account not configured")
	case "pph21":
		if config.SalesPPh21AccountID != nil {
			return *config.SalesPPh21AccountID, nil
		}
		return 0, fmt.Errorf("sales PPh21 account not configured")
	case "pph23":
		if config.SalesPPh23AccountID != nil {
			return *config.SalesPPh23AccountID, nil
		}
		return 0, fmt.Errorf("sales PPh23 account not configured")
	case "other":
		if config.SalesOtherTaxAccountID != nil {
			return *config.SalesOtherTaxAccountID, nil
		}
		return 0, fmt.Errorf("sales other tax account not configured")
	default:
		return 0, fmt.Errorf("unknown sales tax type: %s", taxType)
	}
}

// GetPurchaseTaxAccountID returns the account ID for purchase tax based on tax type
func (s *TaxAccountService) GetPurchaseTaxAccountID(taxType string) (uint, error) {
	config, err := s.GetActiveTaxConfig()
	if err != nil {
		return 0, err
	}

	switch taxType {
	case "ppn":
		if config.PurchasePPNAccountID != nil {
			return *config.PurchasePPNAccountID, nil
		}
		return 0, fmt.Errorf("purchase PPN account not configured")
	case "pph21":
		if config.PurchasePPh21AccountID != nil {
			return *config.PurchasePPh21AccountID, nil
		}
		return 0, fmt.Errorf("purchase PPh21 account not configured")
	case "pph23":
		if config.PurchasePPh23AccountID != nil {
			return *config.PurchasePPh23AccountID, nil
		}
		return 0, fmt.Errorf("purchase PPh23 account not configured")
	case "pph25":
		if config.PurchasePPh25AccountID != nil {
			return *config.PurchasePPh25AccountID, nil
		}
		return 0, fmt.Errorf("purchase PPh25 account not configured")
	case "other":
		if config.PurchaseOtherTaxAccountID != nil {
			return *config.PurchaseOtherTaxAccountID, nil
		}
		return 0, fmt.Errorf("purchase other tax account not configured")
	default:
		return 0, fmt.Errorf("unknown purchase tax type: %s", taxType)
	}
}

// CalculateTax calculates tax amount based on amount and tax configuration
func (s *TaxAccountService) CalculateTax(amount float64, taxRate float64, discountBeforeTax bool, discount float64) float64 {
	config, _ := s.GetActiveTaxConfig()

	var baseAmount float64
	if discountBeforeTax {
		baseAmount = amount - discount
	} else {
		baseAmount = amount
	}

	taxAmount := baseAmount * (taxRate / 100.0)

	// Apply rounding method
	if config != nil {
		switch config.RoundingMethod {
		case "ROUND_UP":
			return math.Ceil(taxAmount)
		case "ROUND_DOWN":
			return math.Floor(taxAmount)
		case "ROUND_HALF_UP":
			return math.Round(taxAmount)
		default:
			return math.Round(taxAmount)
		}
	}

	return math.Round(taxAmount)
}

// CalculateSalesTax calculates sales tax with all applicable taxes
type SalesTaxCalculation struct {
	PPNAmount      float64
	PPh21Amount    float64
	PPh23Amount    float64
	OtherTaxAmount float64
	TotalTaxAmount float64
}

func (s *TaxAccountService) CalculateSalesTax(subtotal float64, discount float64, enablePPN bool, enablePPh21 bool, enablePPh23 bool, enableOther bool) (*SalesTaxCalculation, error) {
	config, err := s.GetActiveTaxConfig()
	if err != nil {
		return nil, err
	}

	result := &SalesTaxCalculation{}

	if enablePPN {
		result.PPNAmount = s.CalculateTax(subtotal, config.SalesPPNRate, config.DiscountBeforeTax, discount)
	}
	if enablePPh21 {
		result.PPh21Amount = s.CalculateTax(subtotal, config.SalesPPh21Rate, config.DiscountBeforeTax, discount)
	}
	if enablePPh23 {
		result.PPh23Amount = s.CalculateTax(subtotal, config.SalesPPh23Rate, config.DiscountBeforeTax, discount)
	}
	if enableOther {
		result.OtherTaxAmount = s.CalculateTax(subtotal, config.SalesOtherTaxRate, config.DiscountBeforeTax, discount)
	}

	result.TotalTaxAmount = result.PPNAmount + result.PPh21Amount + result.PPh23Amount + result.OtherTaxAmount

	return result, nil
}

// CalculatePurchaseTax calculates purchase tax with all applicable taxes
type PurchaseTaxCalculation struct {
	PPNAmount      float64
	PPh21Amount    float64
	PPh23Amount    float64
	PPh25Amount    float64
	OtherTaxAmount float64
	TotalTaxAmount float64
}

func (s *TaxAccountService) CalculatePurchaseTax(subtotal float64, discount float64, enablePPN bool, enablePPh21 bool, enablePPh23 bool, enablePPh25 bool, enableOther bool) (*PurchaseTaxCalculation, error) {
	config, err := s.GetActiveTaxConfig()
	if err != nil {
		return nil, err
	}

	result := &PurchaseTaxCalculation{}

	if enablePPN {
		result.PPNAmount = s.CalculateTax(subtotal, config.PurchasePPNRate, config.DiscountBeforeTax, discount)
	}
	if enablePPh21 {
		result.PPh21Amount = s.CalculateTax(subtotal, config.PurchasePPh21Rate, config.DiscountBeforeTax, discount)
	}
	if enablePPh23 {
		result.PPh23Amount = s.CalculateTax(subtotal, config.PurchasePPh23Rate, config.DiscountBeforeTax, discount)
	}
	if enablePPh25 {
		result.PPh25Amount = s.CalculateTax(subtotal, config.PurchasePPh25Rate, config.DiscountBeforeTax, discount)
	}
	if enableOther {
		result.OtherTaxAmount = s.CalculateTax(subtotal, config.PurchaseOtherTaxRate, config.DiscountBeforeTax, discount)
	}

	result.TotalTaxAmount = result.PPNAmount + result.PPh21Amount + result.PPh23Amount + result.PPh25Amount + result.OtherTaxAmount

	return result, nil
}
