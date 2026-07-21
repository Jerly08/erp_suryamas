package services

import (
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
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

type taxAccountRole struct {
	FieldName    string
	ColumnName   string
	Label        string
	AccountID    uint
	AllowedTypes []string
	DefaultCodes []string
	Required     bool
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

	settings, err := s.loadActiveSettings()

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create default settings if none exist
		defaultSettings, buildErr := s.buildDefaultTaxAccountSettings()
		if buildErr != nil {
			return buildErr
		}

		if createErr := s.db.Create(defaultSettings).Error; createErr != nil {
			return fmt.Errorf("failed to create default settings: %v", createErr)
		}

		// Load the created settings with relations
		settings, err = s.loadActiveSettings()
	}

	if err != nil {
		return fmt.Errorf("failed to load tax account settings: %v", err)
	}

	repaired, repairErr := s.repairInvalidRequiredAccounts(settings)
	if repairErr != nil {
		log.Printf("Warning: Failed to repair tax account settings: %v", repairErr)
	}
	if repaired {
		settings, err = s.loadActiveSettings()
		if err != nil {
			return fmt.Errorf("failed to reload repaired tax account settings: %v", err)
		}
	}

	s.cache = settings
	s.cacheLoaded = true
	return nil
}

func (s *TaxAccountService) loadActiveSettings() (*models.TaxAccountSettings, error) {
	var settings models.TaxAccountSettings
	err := s.preloadTaxAccountRelations(s.db).
		Where("is_active = ?", true).
		Order("updated_at DESC, id DESC").
		First(&settings).Error
	if err != nil {
		return nil, err
	}

	return &settings, nil
}

func (s *TaxAccountService) preloadTaxAccountRelations(db *gorm.DB) *gorm.DB {
	return db.Preload("SalesReceivableAccount").
		Preload("SalesCashAccount").
		Preload("SalesBankAccount").
		Preload("SalesRevenueAccount").
		Preload("SalesShippingRevenueAccount").
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
		Preload("UpdatedByUser")
}

func (s *TaxAccountService) buildDefaultTaxAccountSettings() (*models.TaxAccountSettings, error) {
	settings := &models.TaxAccountSettings{
		IsActive:            true,
		ApplyToAllCompanies: true,
		UpdatedBy:           1,
		Notes:               "Default configuration created by system",
	}

	for _, role := range s.requiredAccountRoles(settings) {
		accountID, err := s.findDefaultAccountID(role.DefaultCodes, role.AllowedTypes)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve default %s: %w", role.Label, err)
		}
		s.assignRoleAccountID(settings, role.FieldName, accountID)
	}

	return settings, nil
}

func (s *TaxAccountService) requiredAccountRoles(settings *models.TaxAccountSettings) []taxAccountRole {
	return []taxAccountRole{
		{
			FieldName:    "sales_receivable",
			ColumnName:   "sales_receivable_account_id",
			Label:        "Sales Receivable",
			AccountID:    settings.SalesReceivableAccountID,
			AllowedTypes: []string{models.AccountTypeAsset},
			DefaultCodes: []string{"1102-001", "1102", "1201"},
			Required:     true,
		},
		{
			FieldName:    "sales_cash",
			ColumnName:   "sales_cash_account_id",
			Label:        "Sales Cash",
			AccountID:    settings.SalesCashAccountID,
			AllowedTypes: []string{models.AccountTypeAsset},
			DefaultCodes: []string{"1101-002", "1101-001", "1101"},
			Required:     true,
		},
		{
			FieldName:    "sales_bank",
			ColumnName:   "sales_bank_account_id",
			Label:        "Sales Bank",
			AccountID:    settings.SalesBankAccountID,
			AllowedTypes: []string{models.AccountTypeAsset},
			DefaultCodes: []string{"1101-003-100", "1101-004-100", "1101-003", "1102", "1101-002"},
			Required:     true,
		},
		{
			FieldName:    "sales_revenue",
			ColumnName:   "sales_revenue_account_id",
			Label:        "Sales Revenue",
			AccountID:    settings.SalesRevenueAccountID,
			AllowedTypes: []string{models.AccountTypeRevenue},
			DefaultCodes: []string{"4101-001", "4101"},
			Required:     true,
		},
		{
			FieldName:    "sales_output_vat",
			ColumnName:   "sales_output_vat_account_id",
			Label:        "Sales Output VAT",
			AccountID:    settings.SalesOutputVATAccountID,
			AllowedTypes: []string{models.AccountTypeLiability},
			DefaultCodes: []string{"2102-001", "2103"},
			Required:     true,
		},
		{
			FieldName:    "purchase_payable",
			ColumnName:   "purchase_payable_account_id",
			Label:        "Purchase Payable",
			AccountID:    settings.PurchasePayableAccountID,
			AllowedTypes: []string{models.AccountTypeLiability},
			DefaultCodes: []string{"2101-001", "2101", "2001"},
			Required:     true,
		},
		{
			FieldName:    "purchase_cash",
			ColumnName:   "purchase_cash_account_id",
			Label:        "Purchase Cash",
			AccountID:    settings.PurchaseCashAccountID,
			AllowedTypes: []string{models.AccountTypeAsset},
			DefaultCodes: []string{"1101-002", "1101-001", "1101"},
			Required:     true,
		},
		{
			FieldName:    "purchase_bank",
			ColumnName:   "purchase_bank_account_id",
			Label:        "Purchase Bank",
			AccountID:    settings.PurchaseBankAccountID,
			AllowedTypes: []string{models.AccountTypeAsset},
			DefaultCodes: []string{"1101-003-100", "1101-004-100", "1101-003", "1102", "1101-002"},
			Required:     true,
		},
		{
			FieldName:    "purchase_input_vat",
			ColumnName:   "purchase_input_vat_account_id",
			Label:        "Purchase Input VAT",
			AccountID:    settings.PurchaseInputVATAccountID,
			AllowedTypes: []string{models.AccountTypeLiability, models.AccountTypeAsset},
			DefaultCodes: []string{"1105-005", "1240", "2102-002", "2102"},
			Required:     true,
		},
		{
			FieldName:    "purchase_expense",
			ColumnName:   "purchase_expense_account_id",
			Label:        "Purchase Expense",
			AccountID:    settings.PurchaseExpenseAccountID,
			AllowedTypes: []string{models.AccountTypeExpense},
			DefaultCodes: []string{"6102", "6001", "6000", "8102", "5100"},
			Required:     true,
		},
	}
}

func (s *TaxAccountService) optionalAccountRoles(settings *models.TaxAccountSettings) []taxAccountRole {
	roles := []taxAccountRole{}
	if settings.SalesShippingRevenueAccountID != nil && *settings.SalesShippingRevenueAccountID > 0 {
		roles = append(roles, taxAccountRole{
			FieldName:    "sales_shipping_revenue",
			ColumnName:   "sales_shipping_revenue_account_id",
			Label:        "Sales Shipping Revenue",
			AccountID:    *settings.SalesShippingRevenueAccountID,
			AllowedTypes: []string{models.AccountTypeRevenue},
		})
	}
	if settings.WithholdingTax21AccountID != nil && *settings.WithholdingTax21AccountID > 0 {
		roles = append(roles, taxAccountRole{
			FieldName:    "withholding_tax21",
			ColumnName:   "withholding_tax21_account_id",
			Label:        "Withholding Tax 21",
			AccountID:    *settings.WithholdingTax21AccountID,
			AllowedTypes: []string{models.AccountTypeAsset},
		})
	}
	if settings.WithholdingTax23AccountID != nil && *settings.WithholdingTax23AccountID > 0 {
		roles = append(roles, taxAccountRole{
			FieldName:    "withholding_tax23",
			ColumnName:   "withholding_tax23_account_id",
			Label:        "Withholding Tax 23",
			AccountID:    *settings.WithholdingTax23AccountID,
			AllowedTypes: []string{models.AccountTypeAsset},
		})
	}
	if settings.WithholdingTax25AccountID != nil && *settings.WithholdingTax25AccountID > 0 {
		roles = append(roles, taxAccountRole{
			FieldName:    "withholding_tax25",
			ColumnName:   "withholding_tax25_account_id",
			Label:        "Withholding Tax 25",
			AccountID:    *settings.WithholdingTax25AccountID,
			AllowedTypes: []string{models.AccountTypeAsset},
		})
	}
	if settings.TaxPayableAccountID != nil && *settings.TaxPayableAccountID > 0 {
		roles = append(roles, taxAccountRole{
			FieldName:    "tax_payable",
			ColumnName:   "tax_payable_account_id",
			Label:        "Tax Payable",
			AccountID:    *settings.TaxPayableAccountID,
			AllowedTypes: []string{models.AccountTypeLiability},
		})
	}
	if settings.InventoryAccountID != nil && *settings.InventoryAccountID > 0 {
		roles = append(roles, taxAccountRole{
			FieldName:    "inventory",
			ColumnName:   "inventory_account_id",
			Label:        "Inventory",
			AccountID:    *settings.InventoryAccountID,
			AllowedTypes: []string{models.AccountTypeAsset},
		})
	}
	if settings.COGSAccountID != nil && *settings.COGSAccountID > 0 {
		roles = append(roles, taxAccountRole{
			FieldName:    "cogs",
			ColumnName:   "cogs_account_id",
			Label:        "COGS",
			AccountID:    *settings.COGSAccountID,
			AllowedTypes: []string{models.AccountTypeExpense},
		})
	}
	return roles
}

func (s *TaxAccountService) assignRoleAccountID(settings *models.TaxAccountSettings, fieldName string, accountID uint) {
	switch fieldName {
	case "sales_receivable":
		settings.SalesReceivableAccountID = accountID
	case "sales_cash":
		settings.SalesCashAccountID = accountID
	case "sales_bank":
		settings.SalesBankAccountID = accountID
	case "sales_revenue":
		settings.SalesRevenueAccountID = accountID
	case "sales_output_vat":
		settings.SalesOutputVATAccountID = accountID
	case "purchase_payable":
		settings.PurchasePayableAccountID = accountID
	case "purchase_cash":
		settings.PurchaseCashAccountID = accountID
	case "purchase_bank":
		settings.PurchaseBankAccountID = accountID
	case "purchase_input_vat":
		settings.PurchaseInputVATAccountID = accountID
	case "purchase_expense":
		settings.PurchaseExpenseAccountID = accountID
	}
}

func (s *TaxAccountService) repairInvalidRequiredAccounts(settings *models.TaxAccountSettings) (bool, error) {
	updates := map[string]interface{}{}
	for _, role := range s.requiredAccountRoles(settings) {
		if role.AccountID == 0 {
			replacementID, err := s.findDefaultAccountID(role.DefaultCodes, role.AllowedTypes)
			if err != nil {
				return false, fmt.Errorf("%s is empty and no valid replacement account was found: %w", role.Label, err)
			}
			updates[role.ColumnName] = replacementID
			continue
		}

		account, err := s.loadAccount(role.AccountID)
		if err == nil && accountMatchesRole(account, role.AllowedTypes) {
			continue
		}

		replacementID, replaceErr := s.findDefaultAccountID(role.DefaultCodes, role.AllowedTypes)
		if replaceErr != nil {
			if err != nil {
				return false, fmt.Errorf("%s account %d cannot be loaded and no replacement was found: %w", role.Label, role.AccountID, replaceErr)
			}
			return false, fmt.Errorf("%s account %s (%s) has invalid type %s and no replacement was found: %w", role.Label, account.Code, account.Name, account.Type, replaceErr)
		}
		if replacementID != role.AccountID {
			updates[role.ColumnName] = replacementID
		}
	}

	if len(updates) == 0 {
		return false, nil
	}

	updates["updated_at"] = time.Now()
	updates["notes"] = strings.TrimSpace(settings.Notes + "\nAuto-repaired invalid tax account mapping types during service startup.")
	if err := s.db.Model(&models.TaxAccountSettings{}).
		Where("id = ?", settings.ID).
		Updates(updates).Error; err != nil {
		return false, err
	}

	log.Printf("Repaired tax account settings %d fields: %v", settings.ID, updates)
	return true, nil
}

func (s *TaxAccountService) validateSettingsAccounts(settings *models.TaxAccountSettings) error {
	roles := append(s.requiredAccountRoles(settings), s.optionalAccountRoles(settings)...)
	for _, role := range roles {
		if role.AccountID == 0 {
			if role.Required {
				return fmt.Errorf("%s account is required", role.Label)
			}
			continue
		}

		account, err := s.loadAccount(role.AccountID)
		if err != nil {
			return fmt.Errorf("%s account not found or inactive", role.Label)
		}
		if !accountMatchesRole(account, role.AllowedTypes) {
			return fmt.Errorf("%s account must be %s account, got %s (%s - %s)", role.Label, strings.Join(role.AllowedTypes, " or "), account.Type, account.Code, account.Name)
		}
	}

	return nil
}

func (s *TaxAccountService) loadAccount(accountID uint) (*models.Account, error) {
	var account models.Account
	if err := s.db.Where("id = ? AND deleted_at IS NULL", accountID).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func accountMatchesRole(account *models.Account, allowedTypes []string) bool {
	if account == nil || !account.IsActive || account.IsHeader {
		return false
	}

	accountType := strings.ToUpper(strings.TrimSpace(account.Type))
	for _, allowedType := range allowedTypes {
		if accountType == strings.ToUpper(strings.TrimSpace(allowedType)) {
			return true
		}
	}
	return false
}

func (s *TaxAccountService) findDefaultAccountID(codes []string, allowedTypes []string) (uint, error) {
	for _, code := range codes {
		var account models.Account
		err := s.db.
			Where("code = ? AND type IN ? AND is_active = ? AND is_header = ? AND deleted_at IS NULL", code, allowedTypes, true, false).
			First(&account).Error
		if err == nil {
			return account.ID, nil
		}
	}

	var account models.Account
	err := s.db.
		Where("type IN ? AND is_active = ? AND is_header = ? AND deleted_at IS NULL", allowedTypes, true, false).
		Order("code ASC, id ASC").
		First(&account).Error
	if err != nil {
		return 0, err
	}

	return account.ID, nil
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

	// Create new settings
	settings := &models.TaxAccountSettings{
		SalesReceivableAccountID:      req.SalesReceivableAccountID,
		SalesCashAccountID:            req.SalesCashAccountID,
		SalesBankAccountID:            req.SalesBankAccountID,
		SalesRevenueAccountID:         req.SalesRevenueAccountID,
		SalesShippingRevenueAccountID: req.SalesShippingRevenueAccountID,
		SalesOutputVATAccountID:       req.SalesOutputVATAccountID,
		PurchasePayableAccountID:      req.PurchasePayableAccountID,
		PurchaseCashAccountID:         req.PurchaseCashAccountID,
		PurchaseBankAccountID:         req.PurchaseBankAccountID,
		PurchaseInputVATAccountID:     req.PurchaseInputVATAccountID,
		PurchaseExpenseAccountID:      req.PurchaseExpenseAccountID,
		WithholdingTax21AccountID:     req.WithholdingTax21AccountID,
		WithholdingTax23AccountID:     req.WithholdingTax23AccountID,
		WithholdingTax25AccountID:     req.WithholdingTax25AccountID,
		TaxPayableAccountID:           req.TaxPayableAccountID,
		InventoryAccountID:            req.InventoryAccountID,
		COGSAccountID:                 req.COGSAccountID,
		IsActive:                      true,
		ApplyToAllCompanies:           req.ApplyToAllCompanies != nil && *req.ApplyToAllCompanies,
		UpdatedBy:                     userID,
		Notes:                         req.Notes,
	}

	// Validate settings
	if err := settings.ValidateAccountSettings(); err != nil {
		return nil, err
	}
	if err := s.validateSettingsAccounts(settings); err != nil {
		return nil, err
	}

	// Deactivate existing active settings only after the replacement is proven valid.
	if err := s.db.Model(&models.TaxAccountSettings{}).
		Where("is_active = ?", true).
		Update("is_active", false).Error; err != nil {
		return nil, fmt.Errorf("failed to deactivate existing settings: %v", err)
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
		Preload("SalesShippingRevenueAccount").
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
	if req.SalesShippingRevenueAccountID != nil {
		settings.SalesShippingRevenueAccountID = req.SalesShippingRevenueAccountID
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

	if err := settings.ValidateAccountSettings(); err != nil {
		return nil, err
	}
	if err := s.validateSettingsAccounts(&settings); err != nil {
		return nil, err
	}

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
		Preload("SalesShippingRevenueAccount").
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
	if req.SalesShippingRevenueAccountID != nil {
		accountIDs = append(accountIDs, *req.SalesShippingRevenueAccountID)
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

	uniqueAccountIDs := make([]uint, 0, len(accountIDs))
	seenAccountIDs := map[uint]struct{}{}
	for _, accountID := range accountIDs {
		if accountID == 0 {
			continue
		}
		if _, seen := seenAccountIDs[accountID]; seen {
			continue
		}
		seenAccountIDs[accountID] = struct{}{}
		uniqueAccountIDs = append(uniqueAccountIDs, accountID)
	}

	var count int64
	if err := s.db.Model(&models.Account{}).
		Where("id IN ? AND is_active = ?", uniqueAccountIDs, true).
		Count(&count).Error; err != nil {
		return fmt.Errorf("failed to validate accounts: %v", err)
	}

	if int(count) != len(uniqueAccountIDs) {
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
	case "sales_shipping_revenue":
		if settings.SalesShippingRevenueAccountID != nil {
			return *settings.SalesShippingRevenueAccountID, nil
		}
		return 0, fmt.Errorf("sales shipping revenue account not configured")
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
		Preload("SalesShippingRevenueAccount").
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
