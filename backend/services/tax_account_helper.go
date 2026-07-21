package services

import (
	"fmt"
	"log"
	"strings"

	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

// TaxAccountHelper provides methods to resolve tax accounts from configuration.
// It always reads the latest active configuration from the database so journal
// posting does not depend on a stale in-memory snapshot.
type TaxAccountHelper struct {
	db                *gorm.DB
	taxAccountService *TaxAccountService
	cachedSettings    *models.TaxAccountSettings
}

// NewTaxAccountHelper creates a new instance of TaxAccountHelper.
func NewTaxAccountHelper(db *gorm.DB) *TaxAccountHelper {
	return &TaxAccountHelper{
		db:                db,
		taxAccountService: NewTaxAccountService(db),
	}
}

// LoadSettings loads the current tax account settings.
func (h *TaxAccountHelper) LoadSettings() error {
	settings, err := h.loadFreshSettings(nil)
	if err != nil {
		log.Printf("WARNING [TaxAccountHelper] Failed to load tax account settings: %v", err)
		return err
	}
	h.cachedSettings = settings
	return nil
}

func (h *TaxAccountHelper) loadFreshSettings(tx *gorm.DB) (*models.TaxAccountSettings, error) {
	dbToUse := h.db
	if tx != nil {
		dbToUse = tx
	}

	var settings models.TaxAccountSettings
	if err := dbToUse.
		Where("is_active = ?", true).
		Order("updated_at DESC, id DESC").
		First(&settings).Error; err != nil {
		return nil, err
	}

	h.cachedSettings = &settings
	return &settings, nil
}

func isPurchaseInputVATAccountAllowed(account *models.Account) bool {
	if account == nil {
		return false
	}

	accountType := strings.ToUpper(strings.TrimSpace(account.Type))
	return accountType == models.AccountTypeAsset || accountType == models.AccountTypeLiability
}

func isSalesOutputVATAccountAllowed(account *models.Account) bool {
	if account == nil {
		return false
	}

	return strings.EqualFold(strings.TrimSpace(account.Type), models.AccountTypeLiability)
}

func isPurchaseWithholdingPayableAccountAllowed(account *models.Account) bool {
	if account == nil {
		return false
	}

	return strings.EqualFold(strings.TrimSpace(account.Type), models.AccountTypeLiability)
}

func (h *TaxAccountHelper) loadConfiguredAccount(tx *gorm.DB, accountID uint) (*models.Account, error) {
	dbToUse := h.db
	if tx != nil {
		dbToUse = tx
	}

	var account models.Account
	if err := dbToUse.First(&account, accountID).Error; err != nil {
		return nil, err
	}

	return &account, nil
}

// GetWithholdingTax21Account returns the configured PPh 21 account or fallback.
func (h *TaxAccountHelper) GetWithholdingTax21Account(tx *gorm.DB) (*models.Account, error) {
	settings, err := h.loadFreshSettings(tx)
	if err != nil {
		return h.getAccountByCode(tx, "1105-001")
	}

	if settings.WithholdingTax21AccountID != nil && *settings.WithholdingTax21AccountID > 0 {
		if account, loadErr := h.loadConfiguredAccount(tx, *settings.WithholdingTax21AccountID); loadErr == nil {
			log.Printf("INFO [TaxAccountHelper] Using configured PPh21 account: %s (%s)", account.Code, account.Name)
			return account, nil
		}
	}

	log.Printf("WARNING [TaxAccountHelper] PPh21 account not configured, using default (1105-001)")
	return h.getAccountByCode(tx, "1105-001")
}

// GetWithholdingTax23Account returns the configured PPh 23 account or fallback.
func (h *TaxAccountHelper) GetWithholdingTax23Account(tx *gorm.DB) (*models.Account, error) {
	settings, err := h.loadFreshSettings(tx)
	if err != nil {
		return h.getAccountByCode(tx, "1105-002")
	}

	if settings.WithholdingTax23AccountID != nil && *settings.WithholdingTax23AccountID > 0 {
		if account, loadErr := h.loadConfiguredAccount(tx, *settings.WithholdingTax23AccountID); loadErr == nil {
			log.Printf("INFO [TaxAccountHelper] Using configured PPh23 account: %s (%s)", account.Code, account.Name)
			return account, nil
		}
	}

	log.Printf("WARNING [TaxAccountHelper] PPh23 account not configured, using default (1105-002)")
	return h.getAccountByCode(tx, "1105-002")
}

func (h *TaxAccountHelper) getPurchaseWithholdingPayableAccount(tx *gorm.DB, taxType, fallbackCode string) (*models.Account, error) {
	if h.taxAccountService != nil {
		if accountID, err := h.taxAccountService.GetPurchaseTaxAccountID(taxType); err == nil && accountID > 0 {
			if account, loadErr := h.loadConfiguredAccount(tx, accountID); loadErr == nil {
				if isPurchaseWithholdingPayableAccountAllowed(account) {
					log.Printf("INFO [TaxAccountHelper] Using configured purchase %s payable account: %s (%s)", taxType, account.Code, account.Name)
					return account, nil
				}

				log.Printf(
					"WARNING [TaxAccountHelper] Configured purchase %s account %s (%s) has invalid type %s, falling back to %s",
					taxType,
					account.Code,
					account.Name,
					account.Type,
					fallbackCode,
				)
			}
		}
	}

	log.Printf("WARNING [TaxAccountHelper] Purchase %s payable account not configured, using default (%s)", taxType, fallbackCode)
	return h.getAccountByCode(tx, fallbackCode)
}

// GetPurchaseWithholdingTax21PayableAccount returns the purchase-side PPh 21 payable account.
func (h *TaxAccountHelper) GetPurchaseWithholdingTax21PayableAccount(tx *gorm.DB) (*models.Account, error) {
	return h.getPurchaseWithholdingPayableAccount(tx, "pph21", "2102-003")
}

// GetPurchaseWithholdingTax23PayableAccount returns the purchase-side PPh 23 payable account.
func (h *TaxAccountHelper) GetPurchaseWithholdingTax23PayableAccount(tx *gorm.DB) (*models.Account, error) {
	return h.getPurchaseWithholdingPayableAccount(tx, "pph23", "2102-004")
}

// GetPPh23TaxCreditAccount returns the dedicated PPh23 tax credit reclass account.
func (h *TaxAccountHelper) GetPPh23TaxCreditAccount(tx *gorm.DB) (*models.Account, error) {
	return h.getAccountByCode(tx, "1105-004")
}

// GetWithholdingTax25Account returns the configured PPh 25 account or fallback.
func (h *TaxAccountHelper) GetWithholdingTax25Account(tx *gorm.DB) (*models.Account, error) {
	settings, err := h.loadFreshSettings(tx)
	if err != nil {
		return h.getAccountByCode(tx, "1105-003")
	}

	if settings.WithholdingTax25AccountID != nil && *settings.WithholdingTax25AccountID > 0 {
		if account, loadErr := h.loadConfiguredAccount(tx, *settings.WithholdingTax25AccountID); loadErr == nil {
			log.Printf("INFO [TaxAccountHelper] Using configured PPh25 account: %s (%s)", account.Code, account.Name)
			return account, nil
		}
	}

	log.Printf("WARNING [TaxAccountHelper] PPh25 account not configured, using default (1105-003)")
	return h.getAccountByCode(tx, "1105-003")
}

// GetTaxPayableAccount returns the configured Tax Payable account or fallback.
func (h *TaxAccountHelper) GetTaxPayableAccount(tx *gorm.DB) (*models.Account, error) {
	settings, err := h.loadFreshSettings(tx)
	if err != nil {
		return h.getAccountByCode(tx, "2102-003")
	}

	if settings.TaxPayableAccountID != nil && *settings.TaxPayableAccountID > 0 {
		if account, loadErr := h.loadConfiguredAccount(tx, *settings.TaxPayableAccountID); loadErr == nil {
			log.Printf("INFO [TaxAccountHelper] Using configured Tax Payable account: %s (%s)", account.Code, account.Name)
			return account, nil
		}
	}

	log.Printf("WARNING [TaxAccountHelper] Tax Payable account not configured, using default (2102-003)")
	return h.getAccountByCode(tx, "2102-003")
}

// GetInventoryAccount returns the configured Inventory account or fallback.
func (h *TaxAccountHelper) GetInventoryAccount(tx *gorm.DB) (*models.Account, error) {
	settings, err := h.loadFreshSettings(tx)
	if err != nil {
		return h.getAccountByCode(tx, "1103-001")
	}

	if settings.InventoryAccountID != nil && *settings.InventoryAccountID > 0 {
		if account, loadErr := h.loadConfiguredAccount(tx, *settings.InventoryAccountID); loadErr == nil {
			log.Printf("INFO [TaxAccountHelper] Using configured Inventory account: %s (%s)", account.Code, account.Name)
			return account, nil
		}
	}

	log.Printf("WARNING [TaxAccountHelper] Inventory account not configured, using default (1103-001)")
	return h.getAccountByCode(tx, "1103-001")
}

// GetCOGSAccount returns the configured COGS account or fallback.
func (h *TaxAccountHelper) GetCOGSAccount(tx *gorm.DB) (*models.Account, error) {
	settings, err := h.loadFreshSettings(tx)
	if err != nil {
		return h.getAccountByCode(tx, "5101")
	}

	if settings.COGSAccountID != nil && *settings.COGSAccountID > 0 {
		if account, loadErr := h.loadConfiguredAccount(tx, *settings.COGSAccountID); loadErr == nil {
			log.Printf("INFO [TaxAccountHelper] Using configured COGS account: %s (%s)", account.Code, account.Name)
			return account, nil
		}
	}

	log.Printf("WARNING [TaxAccountHelper] COGS account not configured, using default (5101)")
	return h.getAccountByCode(tx, "5101")
}

// GetPurchaseInputVATAccount returns the configured PPN Masukan account or a safe legacy fallback.
func (h *TaxAccountHelper) GetPurchaseInputVATAccount(tx *gorm.DB) (*models.Account, error) {
	settings, err := h.loadFreshSettings(tx)
	if err != nil {
		log.Printf("WARNING [TaxAccountHelper] Failed to load tax settings for PPN Masukan, using fallback: %v", err)
		return h.getFirstAvailableValidPurchaseInputVATAccount(tx, "1105-005", "1240", "2102-002")
	}

	if settings.PurchaseInputVATAccountID > 0 {
		if account, loadErr := h.loadConfiguredAccount(tx, settings.PurchaseInputVATAccountID); loadErr == nil {
			if isPurchaseInputVATAccountAllowed(account) {
				log.Printf("INFO [TaxAccountHelper] Using configured PPN Masukan account: %s (%s)", account.Code, account.Name)
				return account, nil
			}

			log.Printf(
				"WARNING [TaxAccountHelper] Configured PPN Masukan account %s (%s) has invalid type %s, falling back to a valid VAT account",
				account.Code,
				account.Name,
				account.Type,
			)
		} else {
			log.Printf(
				"WARNING [TaxAccountHelper] Configured PPN Masukan account ID %d could not be loaded, trying fallback codes",
				settings.PurchaseInputVATAccountID,
			)
		}
	}

	log.Printf("WARNING [TaxAccountHelper] PPN Masukan account not configured, using fallback (1105-005/1240/2102-002)")
	return h.getFirstAvailableValidPurchaseInputVATAccount(tx, "1105-005", "1240", "2102-002")
}

// GetSalesOutputVATAccount returns the configured PPN Keluaran account or a safe legacy fallback.
func (h *TaxAccountHelper) GetSalesOutputVATAccount(tx *gorm.DB) (*models.Account, error) {
	settings, err := h.loadFreshSettings(tx)
	if err != nil {
		log.Printf("WARNING [TaxAccountHelper] Failed to load tax settings for PPN Keluaran, using fallback: %v", err)
		return h.getFirstAvailableValidSalesOutputVATAccount(tx, "2103", "2102-001")
	}

	if settings.SalesOutputVATAccountID > 0 {
		if account, loadErr := h.loadConfiguredAccount(tx, settings.SalesOutputVATAccountID); loadErr == nil {
			if isSalesOutputVATAccountAllowed(account) {
				log.Printf("INFO [TaxAccountHelper] Using configured PPN Keluaran account: %s (%s)", account.Code, account.Name)
				return account, nil
			}

			log.Printf(
				"WARNING [TaxAccountHelper] Configured PPN Keluaran account %s (%s) has invalid type %s, falling back to a valid VAT account",
				account.Code,
				account.Name,
				account.Type,
			)
		} else {
			log.Printf(
				"WARNING [TaxAccountHelper] Configured PPN Keluaran account ID %d could not be loaded, trying fallback codes",
				settings.SalesOutputVATAccountID,
			)
		}
	}

	log.Printf("WARNING [TaxAccountHelper] PPN Keluaran account not configured, using fallback (2103/2102-001)")
	return h.getFirstAvailableValidSalesOutputVATAccount(tx, "2103", "2102-001")
}

// GetSalesShippingRevenueAccount returns the configured shipping revenue
// account. The dedicated 4101-010 account is the only automatic fallback;
// 4102 is reserved for sales returns and discounts.
func (h *TaxAccountHelper) GetSalesShippingRevenueAccount(tx *gorm.DB) (*models.Account, error) {
	settings, err := h.loadFreshSettings(tx)
	if err == nil && settings.SalesShippingRevenueAccountID != nil && *settings.SalesShippingRevenueAccountID > 0 {
		account, loadErr := h.loadConfiguredAccount(tx, *settings.SalesShippingRevenueAccountID)
		if loadErr == nil && strings.EqualFold(strings.TrimSpace(account.Type), models.AccountTypeRevenue) && !account.IsHeader && account.IsActive {
			return account, nil
		}
	}

	return h.getAccountByCode(tx, "4101-010")
}

// getAccountByCode is a helper to fetch account by code.
func (h *TaxAccountHelper) getAccountByCode(tx *gorm.DB, code string) (*models.Account, error) {
	dbToUse := h.db
	if tx != nil {
		dbToUse = tx
	}

	var account models.Account
	if err := dbToUse.Where("code = ? AND is_active = ?", code, true).First(&account).Error; err != nil {
		return nil, fmt.Errorf("account with code %s not found: %v", code, err)
	}
	return &account, nil
}

func (h *TaxAccountHelper) getFirstAvailableAccountByCode(tx *gorm.DB, codes ...string) (*models.Account, error) {
	var lastErr error
	for _, code := range codes {
		account, err := h.getAccountByCode(tx, code)
		if err == nil {
			return account, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no account code candidates provided")
	}

	return nil, lastErr
}

func (h *TaxAccountHelper) getFirstAvailableValidPurchaseInputVATAccount(tx *gorm.DB, codes ...string) (*models.Account, error) {
	var lastErr error
	for _, code := range codes {
		account, err := h.getAccountByCode(tx, code)
		if err == nil && isPurchaseInputVATAccountAllowed(account) {
			return account, nil
		}
		if err == nil {
			lastErr = fmt.Errorf("account %s has invalid type %s for purchase input VAT", account.Code, account.Type)
			continue
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no purchase input VAT fallback account candidates provided")
	}

	return nil, lastErr
}

func (h *TaxAccountHelper) getFirstAvailableValidSalesOutputVATAccount(tx *gorm.DB, codes ...string) (*models.Account, error) {
	var lastErr error
	for _, code := range codes {
		account, err := h.getAccountByCode(tx, code)
		if err == nil && isSalesOutputVATAccountAllowed(account) {
			return account, nil
		}
		if err == nil {
			lastErr = fmt.Errorf("account %s has invalid type %s for sales output VAT", account.Code, account.Type)
			continue
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no sales output VAT fallback account candidates provided")
	}

	return nil, lastErr
}

// RefreshSettings forces a reload of settings from database.
func (h *TaxAccountHelper) RefreshSettings() error {
	return h.LoadSettings()
}
