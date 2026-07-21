package services

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"
)

// AccountResolver provides dynamic account resolution with caching
type AccountResolver struct {
	db          *gorm.DB
	accountRepo repositories.AccountRepository
	cache       map[string]*models.Account
	cacheMutex  sync.RWMutex
	cacheExpiry time.Duration
	lastUpdate  time.Time
}

// AccountType represents different types of accounts we need to resolve
type AccountType string

const (
	// Asset accounts
	AccountTypeCash               AccountType = "CASH"
	AccountTypeBank               AccountType = "BANK"
	AccountTypeBankBCA            AccountType = "BANK_BCA"
	AccountTypeBankMandiri        AccountType = "BANK_MANDIRI"
	AccountTypeAccountsReceivable AccountType = "ACCOUNTS_RECEIVABLE"
	AccountTypePPNReceivable      AccountType = "PPN_RECEIVABLE"

	// Liability accounts
	AccountTypeAccountsPayable AccountType = "ACCOUNTS_PAYABLE"
	AccountTypePPNPayable      AccountType = "PPN_PAYABLE"
	AccountTypeSalesTax        AccountType = "SALES_TAX"

	// Revenue accounts
	AccountTypeSalesRevenue   AccountType = "SALES_REVENUE"
	AccountTypeServiceRevenue AccountType = "SERVICE_REVENUE"

	// Expense accounts
	AccountTypeCOGS             AccountType = "COGS"
	AccountTypeOperatingExpense AccountType = "OPERATING_EXPENSE"
)

// AccountMapping defines possible account codes and names for each type
type AccountMapping struct {
	Codes       []string           `json:"codes"`        // Possible account codes
	Names       []string           `json:"names"`        // Possible account names
	AccountType models.AccountType `json:"account_type"` // Expected account type
	Description string             `json:"description"`
}

var DefaultAccountMappings = map[AccountType]AccountMapping{
	AccountTypeCash: {
		Codes:       []string{"1101", "1100", "1001"},
		Names:       []string{"Kas", "Cash", "Petty Cash", "Kas Kecil"},
		AccountType: models.AccountTypeAsset,
		Description: "Cash account",
	},
	AccountTypeBank: {
		Codes:       []string{"1101-003", "1101-004", "1101-002"},
		Names:       []string{"Bank", "Bank Account", "Bank BCA", "Bank Mandiri", "BANK-GIRO", "BANK-TABUNGAN"},
		AccountType: models.AccountTypeAsset,
		Description: "General bank account (under 1101 KAS DAN SETARA KAS)",
	},
	AccountTypeBankBCA: {
		Codes:       []string{"1101-003"},
		Names:       []string{"Bank BCA", "BCA", "BANK-GIRO"},
		AccountType: models.AccountTypeAsset,
		Description: "Bank BCA account (under 1101 KAS DAN SETARA KAS)",
	},
	AccountTypeBankMandiri: {
		Codes:       []string{"1101-004"},
		Names:       []string{"Bank Mandiri", "Mandiri", "BANK-TABUNGAN"},
		AccountType: models.AccountTypeAsset,
		Description: "Bank Mandiri account (under 1101 KAS DAN SETARA KAS)",
	},
	AccountTypeAccountsReceivable: {
		Codes:       []string{"1201", "1200", "1210"},
		Names:       []string{"Piutang Usaha", "Accounts Receivable", "Piutang", "AR"},
		AccountType: models.AccountTypeAsset,
		Description: "Accounts receivable",
	},
	AccountTypePPNReceivable: {
		Codes:       []string{"1105-005", "1240", "2102-002", "1106", "1105"},
		Names:       []string{"PPN Masukan", "PPN Receivable", "Pajak Masukan", "Input VAT"},
		AccountType: models.AccountTypeAsset,
		Description: "PPN receivable (input tax)",
	},
	AccountTypeAccountsPayable: {
		Codes:       []string{"2001", "2101", "2100"},
		Names:       []string{"Hutang Usaha", "Accounts Payable", "Hutang", "AP"},
		AccountType: models.AccountTypeLiability,
		Description: "Accounts payable",
	},
	AccountTypePPNPayable: {
		Codes:       []string{"2102-001", "2103", "2105"},
		Names:       []string{"PPN Keluaran", "PPN Payable", "Pajak Keluaran", "Sales Tax"},
		AccountType: models.AccountTypeLiability,
		Description: "PPN payable (output tax)",
	},
	AccountTypeSalesRevenue: {
		Codes:       []string{"4101", "4100", "4001", "4000"}, // Prioritize leaf accounts first
		Names:       []string{"Pendapatan Penjualan", "Sales Revenue", "Penjualan", "Revenue", "Sales"},
		AccountType: models.AccountTypeRevenue,
		Description: "Sales revenue",
	},
	AccountTypeServiceRevenue: {
		Codes:       []string{"4200", "4201", "4100"},
		Names:       []string{"Service Revenue", "Pendapatan Jasa", "Service Income"},
		AccountType: models.AccountTypeRevenue,
		Description: "Service revenue",
	},
	AccountTypeCOGS: {
		Codes:       []string{"5000", "5001", "5100"},
		Names:       []string{"Cost of Goods Sold", "Harga Pokok Penjualan", "COGS", "HPP"},
		AccountType: models.AccountTypeExpense,
		Description: "Cost of goods sold",
	},
	AccountTypeOperatingExpense: {
		Codes:       []string{"6000", "6001", "6100"},
		Names:       []string{"Operating Expenses", "Beban Operasional", "Operating Expense"},
		AccountType: models.AccountTypeExpense,
		Description: "Operating expenses",
	},
}

func NewAccountResolver(db *gorm.DB) *AccountResolver {
	return &AccountResolver{
		db:          db,
		accountRepo: repositories.NewAccountRepository(db),
		cache:       make(map[string]*models.Account),
		cacheExpiry: 15 * time.Minute, // Cache for 15 minutes
	}
}

// GetAccount resolves account by type with fallback logic
func (ar *AccountResolver) GetAccount(accountType AccountType) (*models.Account, error) {
	// Check cache first
	if account := ar.getFromCache(string(accountType)); account != nil {
		return account, nil
	}

	mapping, exists := DefaultAccountMappings[accountType]
	if !exists {
		return nil, fmt.Errorf("unknown account type: %s", accountType)
	}

	var account *models.Account
	var err error

	// Try to find by codes first (most specific)
	for _, code := range mapping.Codes {
		account, err = ar.findAccountByCode(code)
		if err == nil && account != nil {
			ar.addToCache(string(accountType), account)
			return account, nil
		}
	}

	// If not found by code, try to find by names
	for _, name := range mapping.Names {
		account, err = ar.findAccountByName(name)
		if err == nil && account != nil {
			ar.addToCache(string(accountType), account)
			return account, nil
		}
	}

	// If still not found, try to find any active account of the correct type
	account, err = ar.findAccountByType(mapping.AccountType)
	if err == nil && account != nil {
		log.Printf("⚠️ Warning: Using fallback account for %s: %s (%s)",
			accountType, account.Name, account.Code)
		ar.addToCache(string(accountType), account)
		return account, nil
	}

	// Last resort: create account if it doesn't exist (for development/setup)
	if ar.shouldCreateAccount() {
		account, err = ar.createDefaultAccount(accountType, mapping)
		if err == nil && account != nil {
			log.Printf("✅ Created default account for %s: %s (%s)",
				accountType, account.Name, account.Code)
			ar.addToCache(string(accountType), account)
			return account, nil
		}
	}

	return nil, fmt.Errorf("account not found for type %s", accountType)
}

// GetAccountID is a convenience method to get account ID
func (ar *AccountResolver) GetAccountID(accountType AccountType) (uint, error) {
	account, err := ar.GetAccount(accountType)
	if err != nil {
		return 0, err
	}
	return account.ID, nil
}

// GetCashAccount gets the primary cash account
func (ar *AccountResolver) GetCashAccount() (*models.Account, error) {
	return ar.GetAccount(AccountTypeCash)
}

// GetBankAccount gets a bank account (tries BCA first, then any bank)
func (ar *AccountResolver) GetBankAccount() (*models.Account, error) {
	// Try BCA first
	account, err := ar.GetAccount(AccountTypeBankBCA)
	if err == nil {
		return account, nil
	}

	// Try Mandiri
	account, err = ar.GetAccount(AccountTypeBankMandiri)
	if err == nil {
		return account, nil
	}

	// Try any bank account
	return ar.GetAccount(AccountTypeBank)
}

// GetBankAccountForPaymentMethod gets appropriate bank account based on payment method
func (ar *AccountResolver) GetBankAccountForPaymentMethod(paymentMethod string) (*models.Account, error) {
	switch paymentMethod {
	case "BANK_TRANSFER":
		return ar.GetBankAccount()
	case "CREDIT_CARD":
		// Try to find credit card clearing account, fallback to bank
		account, err := ar.GetAccount(AccountTypeBank)
		return account, err
	case "CASH":
		return ar.GetCashAccount()
	default:
		return ar.GetBankAccount()
	}
}

// findAccountByCode finds account by exact code match
func (ar *AccountResolver) findAccountByCode(code string) (*models.Account, error) {
	ctx := context.Background()
	account, err := ar.accountRepo.FindByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	// Check if account is active
	if !account.IsActive {
		return nil, gorm.ErrRecordNotFound
	}

	// 🛡️ CRITICAL: Prevent parent accounts from being used in journal entries
	if account.IsHeader {
		log.Printf("⚠️ Skipping header account %s (%s) - cannot be used for journal entries", account.Code, account.Name)
		return nil, gorm.ErrRecordNotFound
	}

	return account, nil
}

// findAccountByName finds account by name (partial match)
func (ar *AccountResolver) findAccountByName(name string) (*models.Account, error) {
	var account models.Account

	err := ar.db.Where("name ILIKE ? AND is_active = true", "%"+name+"%").
		First(&account).Error

	if err != nil {
		return nil, err
	}

	return &account, nil
}

// findAccountByType finds any account of the specified type
func (ar *AccountResolver) findAccountByType(accountType models.AccountType) (*models.Account, error) {
	var account models.Account

	err := ar.db.Where("type = ? AND is_active = true AND is_header = false").
		First(&account).Error

	if err != nil {
		return nil, err
	}

	return &account, nil
}

// shouldCreateAccount determines if we should auto-create missing accounts
func (ar *AccountResolver) shouldCreateAccount() bool {
	// Only create in development/setup mode
	// Check for environment variable or database setting
	return true // For now, allow creation
}

// createDefaultAccount creates a default account for the type
func (ar *AccountResolver) createDefaultAccount(accountType AccountType, mapping AccountMapping) (*models.Account, error) {
	if len(mapping.Codes) == 0 || len(mapping.Names) == 0 {
		return nil, fmt.Errorf("cannot create account: no codes or names defined for %s", accountType)
	}

	// Use the first code and name as defaults
	code := mapping.Codes[0]
	name := mapping.Names[0]

	// Check if code already exists
	existing, err := ar.findAccountByCode(code)
	if err == nil && existing != nil {
		return existing, nil
	}

	// Create account request
	isHeaderFlag := false
	accountReq := &models.AccountCreateRequest{
		Code:           code,
		Name:           name,
		Type:           mapping.AccountType,
		Description:    mapping.Description,
		IsHeader:       &isHeaderFlag,
		OpeningBalance: 0,
	}

	// Create account using repository
	ctx := context.Background()
	account, err := ar.accountRepo.Create(ctx, accountReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create default account: %v", err)
	}

	return account, nil
}

// getFromCache retrieves account from cache
func (ar *AccountResolver) getFromCache(key string) *models.Account {
	ar.cacheMutex.RLock()
	defer ar.cacheMutex.RUnlock()

	// Check if cache is expired
	if time.Since(ar.lastUpdate) > ar.cacheExpiry {
		return nil
	}

	return ar.cache[key]
}

// addToCache adds account to cache
func (ar *AccountResolver) addToCache(key string, account *models.Account) {
	ar.cacheMutex.Lock()
	defer ar.cacheMutex.Unlock()

	ar.cache[key] = account
	ar.lastUpdate = time.Now()
}

// ClearCache clears the account cache
func (ar *AccountResolver) ClearCache() {
	ar.cacheMutex.Lock()
	defer ar.cacheMutex.Unlock()

	ar.cache = make(map[string]*models.Account)
	ar.lastUpdate = time.Time{}

	log.Printf("🧹 Account resolver cache cleared")
}

// RefreshCache refreshes all cached accounts
func (ar *AccountResolver) RefreshCache() error {
	ar.ClearCache()

	// Pre-load common accounts
	commonTypes := []AccountType{
		AccountTypeCash,
		AccountTypeBank,
		AccountTypeAccountsReceivable,
		AccountTypeAccountsPayable,
		AccountTypeSalesRevenue,
		AccountTypePPNReceivable,
		AccountTypePPNPayable,
	}

	for _, accountType := range commonTypes {
		_, err := ar.GetAccount(accountType)
		if err != nil {
			log.Printf("⚠️ Warning: Could not pre-load account type %s: %v", accountType, err)
		}
	}

	log.Printf("🔄 Account resolver cache refreshed")
	return nil
}

// GetAccountsByType returns all accounts of a specific type
func (ar *AccountResolver) GetAccountsByType(accountType models.AccountType) ([]models.Account, error) {
	ctx := context.Background()
	return ar.accountRepo.FindByType(ctx, string(accountType))
}

// ValidateAccountExists checks if an account exists and is active
func (ar *AccountResolver) ValidateAccountExists(accountID uint) error {
	ctx := context.Background()
	account, err := ar.accountRepo.FindByID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("account not found: %v", err)
	}

	if !account.IsActive {
		return fmt.Errorf("account %s (%s) is not active", account.Name, account.Code)
	}

	if account.IsHeader {
		return fmt.Errorf("cannot post to header account %s (%s)", account.Name, account.Code)
	}

	return nil
}
