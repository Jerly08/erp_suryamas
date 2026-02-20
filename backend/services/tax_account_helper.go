package services

import (
	"fmt"
	"log"

	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

// TaxAccountHelper provides methods to resolve tax accounts from configuration
type TaxAccountHelper struct {
	db                *gorm.DB
	taxAccountService *TaxAccountService
	cachedSettings    *models.TaxAccountSettings
}

// NewTaxAccountHelper creates a new instance of TaxAccountHelper
func NewTaxAccountHelper(db *gorm.DB) *TaxAccountHelper {
	return &TaxAccountHelper{
		db:                db,
		taxAccountService: NewTaxAccountService(db),
	}
}

// LoadSettings loads the current tax account settings
func (h *TaxAccountHelper) LoadSettings() error {
	settings, err := h.taxAccountService.GetSettings()
	if err != nil {
		log.Printf("⚠️ [TaxAccountHelper] Failed to load tax account settings: %v", err)
		return err
	}
	h.cachedSettings = settings
	return nil
}

// GetWithholdingTax21Account returns the configured PPh 21 account or fallback
func (h *TaxAccountHelper) GetWithholdingTax21Account(tx *gorm.DB) (*models.Account, error) {
	// Ensure settings are loaded
	if h.cachedSettings == nil {
		if err := h.LoadSettings(); err != nil {
			// Fallback to COA template account
			return h.getAccountByCode(tx, "1105-001") // PPH 21 DIBAYAR DIMUKA (COA template)
		}
	}

	// Use configured account if available
	if h.cachedSettings.WithholdingTax21AccountID != nil && *h.cachedSettings.WithholdingTax21AccountID > 0 {
		var account models.Account
		if err := tx.First(&account, *h.cachedSettings.WithholdingTax21AccountID).Error; err == nil {
			log.Printf("✅ [TaxAccountHelper] Using configured PPh21 account: %s (%s)", account.Code, account.Name)
			return &account, nil
		}
	}

	// Fallback to default
	log.Printf("⚠️ [TaxAccountHelper] PPh21 account not configured, using default (1105-001)")
	return h.getAccountByCode(tx, "1105-001")
}

// GetWithholdingTax23Account returns the configured PPh 23 account or fallback
func (h *TaxAccountHelper) GetWithholdingTax23Account(tx *gorm.DB) (*models.Account, error) {
	// Ensure settings are loaded
	if h.cachedSettings == nil {
		if err := h.LoadSettings(); err != nil {
			// Fallback to COA template account
			return h.getAccountByCode(tx, "1105-002") // PPH 23 DIBAYAR DIMUKA (COA template)
		}
	}

	// Use configured account if available
	if h.cachedSettings.WithholdingTax23AccountID != nil && *h.cachedSettings.WithholdingTax23AccountID > 0 {
		var account models.Account
		if err := tx.First(&account, *h.cachedSettings.WithholdingTax23AccountID).Error; err == nil {
			log.Printf("✅ [TaxAccountHelper] Using configured PPh23 account: %s (%s)", account.Code, account.Name)
			return &account, nil
		}
	}

	// Fallback to default
	log.Printf("⚠️ [TaxAccountHelper] PPh23 account not configured, using default (1105-002)")
	return h.getAccountByCode(tx, "1105-002")
}

// GetWithholdingTax25Account returns the configured PPh 25 account or fallback
func (h *TaxAccountHelper) GetWithholdingTax25Account(tx *gorm.DB) (*models.Account, error) {
	// Ensure settings are loaded
	if h.cachedSettings == nil {
		if err := h.LoadSettings(); err != nil {
			// Fallback to COA template account
			return h.getAccountByCode(tx, "1105-003") // PPH FINAL DIBAYAR DIMUKA (COA template)
		}
	}

	// Use configured account if available
	if h.cachedSettings.WithholdingTax25AccountID != nil && *h.cachedSettings.WithholdingTax25AccountID > 0 {
		var account models.Account
		if err := tx.First(&account, *h.cachedSettings.WithholdingTax25AccountID).Error; err == nil {
			log.Printf("✅ [TaxAccountHelper] Using configured PPh25 account: %s (%s)", account.Code, account.Name)
			return &account, nil
		}
	}

	// Fallback to default
	log.Printf("⚠️ [TaxAccountHelper] PPh25 account not configured, using default (1105-003)")
	return h.getAccountByCode(tx, "1105-003")
}

// GetTaxPayableAccount returns the configured Tax Payable account or fallback
func (h *TaxAccountHelper) GetTaxPayableAccount(tx *gorm.DB) (*models.Account, error) {
	// Ensure settings are loaded
	if h.cachedSettings == nil {
		if err := h.LoadSettings(); err != nil {
			// Fallback to COA template account
			return h.getAccountByCode(tx, "2102-003") // PPH PASAL 21 utang (COA template)
		}
	}

	// Use configured account if available
	if h.cachedSettings.TaxPayableAccountID != nil && *h.cachedSettings.TaxPayableAccountID > 0 {
		var account models.Account
		if err := tx.First(&account, *h.cachedSettings.TaxPayableAccountID).Error; err == nil {
			log.Printf("✅ [TaxAccountHelper] Using configured Tax Payable account: %s (%s)", account.Code, account.Name)
			return &account, nil
		}
	}

	// Fallback to default
	log.Printf("⚠️ [TaxAccountHelper] Tax Payable account not configured, using default (2102-003)")
	return h.getAccountByCode(tx, "2102-003")
}

// GetInventoryAccount returns the configured Inventory account or fallback
func (h *TaxAccountHelper) GetInventoryAccount(tx *gorm.DB) (*models.Account, error) {
	// Ensure settings are loaded
	if h.cachedSettings == nil {
		if err := h.LoadSettings(); err != nil {
			// Fallback to hardcoded account
			return h.getAccountByCode(tx, "1103-001") // Default Persediaan Barang Dagang (Trading)
		}
	}

	// Use configured account if available
	if h.cachedSettings.InventoryAccountID != nil && *h.cachedSettings.InventoryAccountID > 0 {
		var account models.Account
		if err := tx.First(&account, *h.cachedSettings.InventoryAccountID).Error; err == nil {
			log.Printf("✅ [TaxAccountHelper] Using configured Inventory account: %s (%s)", account.Code, account.Name)
			return &account, nil
		}
	}

	// Fallback to default
	log.Printf("⚠️ [TaxAccountHelper] Inventory account not configured, using default (1103-001)")
	return h.getAccountByCode(tx, "1103-001")
}

// GetCOGSAccount returns the configured COGS account or fallback
func (h *TaxAccountHelper) GetCOGSAccount(tx *gorm.DB) (*models.Account, error) {
	// Ensure settings are loaded
	if h.cachedSettings == nil {
		if err := h.LoadSettings(); err != nil {
			// Fallback to hardcoded account
			return h.getAccountByCode(tx, "5101") // Default Pembelian Barang Dagang (Trading) - COGS Header is 5000, 5101 is detail
		}
	}

	// Use configured account if available
	if h.cachedSettings.COGSAccountID != nil && *h.cachedSettings.COGSAccountID > 0 {
		var account models.Account
		if err := tx.First(&account, *h.cachedSettings.COGSAccountID).Error; err == nil {
			log.Printf("✅ [TaxAccountHelper] Using configured COGS account: %s (%s)", account.Code, account.Name)
			return &account, nil
		}
	}

	// Fallback to default
	log.Printf("⚠️ [TaxAccountHelper] COGS account not configured, using default (5101)")
	return h.getAccountByCode(tx, "5101")
}

// getAccountByCode is a helper to fetch account by code
func (h *TaxAccountHelper) getAccountByCode(tx *gorm.DB, code string) (*models.Account, error) {
	var account models.Account
	if err := tx.Where("code = ? AND is_active = ?", code, true).First(&account).Error; err != nil {
		return nil, fmt.Errorf("account with code %s not found: %v", code, err)
	}
	return &account, nil
}

// RefreshSettings forces a reload of settings from database
func (h *TaxAccountHelper) RefreshSettings() error {
	return h.LoadSettings()
}
