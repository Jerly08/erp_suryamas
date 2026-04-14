package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"app-sistem-akuntansi/models"
)

// AccountingConfig holds all accounting-related configurations
type AccountingConfig struct {
	DefaultAccounts  DefaultAccountMapping `json:"default_accounts"`
	TaxRates         TaxConfiguration      `json:"tax_rates"`
	CurrencySettings CurrencyConfiguration `json:"currency_settings"`
	JournalSettings  JournalConfiguration  `json:"journal_settings"`
	PeriodSettings   PeriodConfiguration   `json:"period_settings"`
	AuditSettings    AuditConfiguration    `json:"audit_settings"`
}

// DefaultAccountMapping defines default account mappings for different transaction types
type DefaultAccountMapping struct {
	// Asset Accounts
	Cash                    uint `json:"cash"`                     // Default: 1101
	Bank                    uint `json:"bank"`                     // Default: 1102
	AccountsReceivable      uint `json:"accounts_receivable"`      // Default: 1201
	Inventory               uint `json:"inventory"`                // Default: 1301
	PPNReceivable           uint `json:"ppn_receivable"`           // Default: 1240 (PPN Masukan)
	FixedAssets             uint `json:"fixed_assets"`             // Default: 1501
	AccumulatedDepreciation uint `json:"accumulated_depreciation"` // Default: 1502

	// Liability Accounts
	AccountsPayable uint `json:"accounts_payable"` // Default: 2101
	PPNPayable      uint `json:"ppn_payable"`      // Default: 2103
	TaxPayable      uint `json:"tax_payable"`      // Default: 2103
	AccruedExpenses uint `json:"accrued_expenses"` // Default: 2201

	// Equity Accounts
	ShareCapital     uint `json:"share_capital"`     // Default: 3001
	RetainedEarnings uint `json:"retained_earnings"` // Default: 3101

	// Revenue Accounts
	SalesRevenue   uint `json:"sales_revenue"`   // Default: 4101
	ServiceRevenue uint `json:"service_revenue"` // Default: 4102
	OtherIncome    uint `json:"other_income"`    // Default: 4901

	// Expense Accounts
	COGS                uint `json:"cogs"`                 // Default: 5101
	OperatingExpenses   uint `json:"operating_expenses"`   // Default: 6001
	DepreciationExpense uint `json:"depreciation_expense"` // Default: 6201
	InterestExpense     uint `json:"interest_expense"`     // Default: 7001
}

// TaxConfiguration defines tax rates and settings
type TaxConfiguration struct {
	DefaultPPN     float64            `json:"default_ppn"`   // Default: 11.0
	DefaultPPh21   float64            `json:"default_pph21"` // Default: 5.0
	DefaultPPh23   float64            `json:"default_pph23"` // Default: 2.0
	TaxRatesByType map[string]float64 `json:"tax_rates_by_type"`
	TaxExemptions  []string           `json:"tax_exemptions"`
}

// CurrencyConfiguration defines currency settings
type CurrencyConfiguration struct {
	BaseCurrency        string            `json:"base_currency"` // Default: IDR
	SupportedCurrencies []string          `json:"supported_currencies"`
	ExchangeRateAPI     string            `json:"exchange_rate_api"`
	DecimalPlaces       int               `json:"decimal_places"` // Default: 2
	CurrencySymbols     map[string]string `json:"currency_symbols"`
}

// JournalConfiguration defines journal entry settings
type JournalConfiguration struct {
	CodePrefix           string `json:"code_prefix"`            // Default: JE
	AutoGenerateCode     bool   `json:"auto_generate_code"`     // Default: true
	RequireBalancedEntry bool   `json:"require_balanced_entry"` // Default: true
	AllowFutureDates     bool   `json:"allow_future_dates"`     // Default: false
	MaxFutureDays        int    `json:"max_future_days"`        // Default: 7
	RequireApproval      bool   `json:"require_approval"`       // Default: false
	BatchSize            int    `json:"batch_size"`             // Default: 100
}

// PeriodConfiguration defines accounting period settings
type PeriodConfiguration struct {
	FiscalYearStart      int  `json:"fiscal_year_start"`        // Default: 1 (January)
	AutoClosePeriods     bool `json:"auto_close_periods"`       // Default: false
	WarnBeforeClose      bool `json:"warn_before_close"`        // Default: true
	AllowPostToOldPeriod bool `json:"allow_post_to_old_period"` // Default: false
	MaxOldPeriodMonths   int  `json:"max_old_period_months"`    // Default: 24
}

// AuditConfiguration defines audit settings
type AuditConfiguration struct {
	EnableAuditTrail    bool     `json:"enable_audit_trail"`    // Default: true
	AuditJournalChanges bool     `json:"audit_journal_changes"` // Default: true
	AuditAccountChanges bool     `json:"audit_account_changes"` // Default: true
	RetentionPeriodDays int      `json:"retention_period_days"` // Default: 2555 (7 years)
	SensitiveFields     []string `json:"sensitive_fields"`
	ExcludeUsers        []uint   `json:"exclude_users"`
}

var (
	accountingConfig *AccountingConfig
	configMutex      sync.RWMutex
	configLoaded     bool
)

// LoadAccountingConfig loads accounting configuration from file
func LoadAccountingConfig(configPath string) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	if configPath == "" {
		configPath = "config/accounting_config.json"
	}

	// Load from file if exists
	if _, err := os.Stat(configPath); err == nil {
		file, err := os.Open(configPath)
		if err != nil {
			return fmt.Errorf("failed to open config file: %v", err)
		}
		defer file.Close()

		decoder := json.NewDecoder(file)
		accountingConfig = &AccountingConfig{}
		if err := decoder.Decode(accountingConfig); err != nil {
			return fmt.Errorf("failed to decode config: %v", err)
		}
	} else {
		// Use default configuration
		accountingConfig = getDefaultConfig()
	}

	configLoaded = true
	return nil
}

// GetAccountingConfig returns the current accounting configuration
func GetAccountingConfig() *AccountingConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()

	if !configLoaded {
		// Load default config if not loaded
		accountingConfig = getDefaultConfig()
		configLoaded = true
	}

	return accountingConfig
}

// SaveAccountingConfig saves current configuration to file
func SaveAccountingConfig(configPath string) error {
	configMutex.RLock()
	config := accountingConfig
	configMutex.RUnlock()

	if configPath == "" {
		configPath = "config/accounting_config.json"
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll("config", 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	file, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("failed to encode config: %v", err)
	}

	return nil
}

// getDefaultConfig returns default accounting configuration
func getDefaultConfig() *AccountingConfig {
	return &AccountingConfig{
		DefaultAccounts: DefaultAccountMapping{
			// Asset Accounts
			Cash:                    1101,
			Bank:                    1102,
			AccountsReceivable:      1201,
			Inventory:               1301,
			PPNReceivable:           1240,
			FixedAssets:             1501,
			AccumulatedDepreciation: 1502,

			// Liability Accounts
			AccountsPayable: 2101,
			PPNPayable:      2103,
			TaxPayable:      2103,
			AccruedExpenses: 2201,

			// Equity Accounts
			ShareCapital:     3001,
			RetainedEarnings: 3201, // LABA DITAHAN - Fixed from 3101 (Modal Pemilik)

			// Revenue Accounts
			SalesRevenue:   4101,
			ServiceRevenue: 4102,
			OtherIncome:    4901,

			// Expense Accounts
			COGS:                5101,
			OperatingExpenses:   6001,
			DepreciationExpense: 6201,
			InterestExpense:     7001,
		},
		TaxRates: TaxConfiguration{
			DefaultPPN:   11.0,
			DefaultPPh21: 5.0,
			DefaultPPh23: 2.0,
			TaxRatesByType: map[string]float64{
				"PPN":   11.0,
				"PPh21": 5.0,
				"PPh23": 2.0,
				"PPh25": 25.0,
				"PPh29": 29.0,
			},
			TaxExemptions: []string{"medical", "education", "religious"},
		},
		CurrencySettings: CurrencyConfiguration{
			BaseCurrency:        "IDR",
			SupportedCurrencies: []string{"IDR", "USD", "CNY", "EUR", "SGD"},
			ExchangeRateAPI:     "https://api.exchangerate-api.com/v4/latest/",
			DecimalPlaces:       2,
			CurrencySymbols: map[string]string{
				"IDR": "Rp",
				"USD": "$",
				"CNY": "¥",
				"EUR": "€",
				"SGD": "S$",
			},
		},
		JournalSettings: JournalConfiguration{
			CodePrefix:           "JE",
			AutoGenerateCode:     true,
			RequireBalancedEntry: true,
			AllowFutureDates:     false,
			MaxFutureDays:        7,
			RequireApproval:      false,
			BatchSize:            100,
		},
		PeriodSettings: PeriodConfiguration{
			FiscalYearStart:      1, // January
			AutoClosePeriods:     false,
			WarnBeforeClose:      true,
			AllowPostToOldPeriod: false,
			MaxOldPeriodMonths:   24,
		},
		AuditSettings: AuditConfiguration{
			EnableAuditTrail:    true,
			AuditJournalChanges: true,
			AuditAccountChanges: true,
			RetentionPeriodDays: 2555, // 7 years
			SensitiveFields:     []string{"balance", "amount", "salary"},
			ExcludeUsers:        []uint{}, // No excluded users by default
		},
	}
}

// GetDefaultAccountID returns the default account ID for a given account type
func GetDefaultAccountID(accountType string) (uint, error) {
	config := GetAccountingConfig()

	switch accountType {
	case "cash":
		return config.DefaultAccounts.Cash, nil
	case "bank":
		return config.DefaultAccounts.Bank, nil
	case "accounts_receivable":
		return config.DefaultAccounts.AccountsReceivable, nil
	case "inventory":
		return config.DefaultAccounts.Inventory, nil
	case "ppn_receivable":
		return config.DefaultAccounts.PPNReceivable, nil
	case "fixed_assets":
		return config.DefaultAccounts.FixedAssets, nil
	case "accumulated_depreciation":
		return config.DefaultAccounts.AccumulatedDepreciation, nil
	case "accounts_payable":
		return config.DefaultAccounts.AccountsPayable, nil
	case "ppn_payable":
		return config.DefaultAccounts.PPNPayable, nil
	case "tax_payable":
		return config.DefaultAccounts.TaxPayable, nil
	case "accrued_expenses":
		return config.DefaultAccounts.AccruedExpenses, nil
	case "share_capital":
		return config.DefaultAccounts.ShareCapital, nil
	case "retained_earnings":
		return config.DefaultAccounts.RetainedEarnings, nil
	case "sales_revenue":
		return config.DefaultAccounts.SalesRevenue, nil
	case "service_revenue":
		return config.DefaultAccounts.ServiceRevenue, nil
	case "other_income":
		return config.DefaultAccounts.OtherIncome, nil
	case "cogs":
		return config.DefaultAccounts.COGS, nil
	case "operating_expenses":
		return config.DefaultAccounts.OperatingExpenses, nil
	case "depreciation_expense":
		return config.DefaultAccounts.DepreciationExpense, nil
	case "interest_expense":
		return config.DefaultAccounts.InterestExpense, nil
	default:
		return 0, fmt.Errorf("unknown account type: %s", accountType)
	}
}

// GetDefaultTaxRate returns the default tax rate for a given tax type
func GetDefaultTaxRate(taxType string) (float64, error) {
	config := GetAccountingConfig()

	switch taxType {
	case "PPN", "ppn":
		return config.TaxRates.DefaultPPN, nil
	case "PPh21", "pph21":
		return config.TaxRates.DefaultPPh21, nil
	case "PPh23", "pph23":
		return config.TaxRates.DefaultPPh23, nil
	default:
		if rate, exists := config.TaxRates.TaxRatesByType[taxType]; exists {
			return rate, nil
		}
		return 0, fmt.Errorf("unknown tax type: %s", taxType)
	}
}

// UpdateAccountMapping updates a specific account mapping
func UpdateAccountMapping(accountType string, accountID uint) error {
	configMutex.Lock()
	defer configMutex.Unlock()

	if accountingConfig == nil {
		accountingConfig = getDefaultConfig()
	}

	switch accountType {
	case "cash":
		accountingConfig.DefaultAccounts.Cash = accountID
	case "bank":
		accountingConfig.DefaultAccounts.Bank = accountID
	case "accounts_receivable":
		accountingConfig.DefaultAccounts.AccountsReceivable = accountID
	case "inventory":
		accountingConfig.DefaultAccounts.Inventory = accountID
	case "ppn_receivable":
		accountingConfig.DefaultAccounts.PPNReceivable = accountID
	case "fixed_assets":
		accountingConfig.DefaultAccounts.FixedAssets = accountID
	case "accumulated_depreciation":
		accountingConfig.DefaultAccounts.AccumulatedDepreciation = accountID
	case "accounts_payable":
		accountingConfig.DefaultAccounts.AccountsPayable = accountID
	case "ppn_payable":
		accountingConfig.DefaultAccounts.PPNPayable = accountID
	case "tax_payable":
		accountingConfig.DefaultAccounts.TaxPayable = accountID
	case "accrued_expenses":
		accountingConfig.DefaultAccounts.AccruedExpenses = accountID
	case "share_capital":
		accountingConfig.DefaultAccounts.ShareCapital = accountID
	case "retained_earnings":
		accountingConfig.DefaultAccounts.RetainedEarnings = accountID
	case "sales_revenue":
		accountingConfig.DefaultAccounts.SalesRevenue = accountID
	case "service_revenue":
		accountingConfig.DefaultAccounts.ServiceRevenue = accountID
	case "other_income":
		accountingConfig.DefaultAccounts.OtherIncome = accountID
	case "cogs":
		accountingConfig.DefaultAccounts.COGS = accountID
	case "operating_expenses":
		accountingConfig.DefaultAccounts.OperatingExpenses = accountID
	case "depreciation_expense":
		accountingConfig.DefaultAccounts.DepreciationExpense = accountID
	case "interest_expense":
		accountingConfig.DefaultAccounts.InterestExpense = accountID
	default:
		return fmt.Errorf("unknown account type: %s", accountType)
	}

	return nil
}

// ValidateAccountConfiguration validates if configured accounts exist and are appropriate
func ValidateAccountConfiguration(db *models.Account) []string {
	config := GetAccountingConfig()
	var errors []string

	accountMappings := map[string]uint{
		"Cash":                     config.DefaultAccounts.Cash,
		"Bank":                     config.DefaultAccounts.Bank,
		"Accounts Receivable":      config.DefaultAccounts.AccountsReceivable,
		"Inventory":                config.DefaultAccounts.Inventory,
		"PPN Receivable":           config.DefaultAccounts.PPNReceivable,
		"Fixed Assets":             config.DefaultAccounts.FixedAssets,
		"Accumulated Depreciation": config.DefaultAccounts.AccumulatedDepreciation,
		"Accounts Payable":         config.DefaultAccounts.AccountsPayable,
		"PPN Payable":              config.DefaultAccounts.PPNPayable,
		"Tax Payable":              config.DefaultAccounts.TaxPayable,
		"Accrued Expenses":         config.DefaultAccounts.AccruedExpenses,
		"Share Capital":            config.DefaultAccounts.ShareCapital,
		"Retained Earnings":        config.DefaultAccounts.RetainedEarnings,
		"Sales Revenue":            config.DefaultAccounts.SalesRevenue,
		"Service Revenue":          config.DefaultAccounts.ServiceRevenue,
		"Other Income":             config.DefaultAccounts.OtherIncome,
		"COGS":                     config.DefaultAccounts.COGS,
		"Operating Expenses":       config.DefaultAccounts.OperatingExpenses,
		"Depreciation Expense":     config.DefaultAccounts.DepreciationExpense,
		"Interest Expense":         config.DefaultAccounts.InterestExpense,
	}

	// TODO: Add actual database validation
	// For now, just check if account IDs are non-zero
	for accountName, accountID := range accountMappings {
		if accountID == 0 {
			errors = append(errors, fmt.Sprintf("%s account is not configured", accountName))
		}
	}

	return errors
}
