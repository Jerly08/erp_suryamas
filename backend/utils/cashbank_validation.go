package utils

import (
	"app-sistem-akuntansi/models"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// CashBank validation constants
const (
	// Balance limits
	MinCashBalance = 0.0
	MaxCashBalance = 999999999.99
	MinBankBalance = -10000.0 // Allow small overdraft for bank accounts
	MaxBankBalance = 999999999.99

	// Transaction limits
	MaxDailyTransactionLimit   = 100000000.0  // 100 million
	MaxMonthlyTransactionLimit = 1000000000.0 // 1 billion
	MaxSingleTransactionLimit  = 50000000.0   // 50 million

	// Code generation
	CashCodePrefix = "CSH"
	BankCodePrefix = "BNK"
	CodeYearFormat = "2006"

	// Currency validation
	DefaultCurrency = "IDR"
)

// Supported currencies
var SupportedCurrencies = []string{"IDR", "USD", "EUR", "SGD", "JPY", "CNY", "AUD"}

// CashBankValidator provides validation for cash and bank operations
type CashBankValidator struct{}

// NewCashBankValidator creates a new validator instance
func NewCashBankValidator() *CashBankValidator {
	return &CashBankValidator{}
}

// ValidateCreateRequest validates cash bank creation request
func (v *CashBankValidator) ValidateCreateRequest(req interface{}) error {
	// This would be implemented based on your specific request structure
	// For now, returning nil as placeholder
	return nil
}

// ValidateUpdateRequest validates cash bank update request
func (v *CashBankValidator) ValidateUpdateRequest(req interface{}) error {
	return nil
}

// ValidateTransferRequest validates transfer request
func (v *CashBankValidator) ValidateTransferRequest(fromAccount, toAccount *models.CashBank, amount float64) error {
	if fromAccount == nil {
		return errors.New("source account not found")
	}

	if toAccount == nil {
		return errors.New("destination account not found")
	}

	if fromAccount.ID == toAccount.ID {
		return errors.New("cannot transfer to the same account")
	}

	if !fromAccount.IsActive {
		return fmt.Errorf("source account '%s' is inactive", fromAccount.Name)
	}

	if !toAccount.IsActive {
		return fmt.Errorf("destination account '%s' is inactive", toAccount.Name)
	}

	if fromAccount.IsRestricted {
		return fmt.Errorf("source account '%s' is restricted", fromAccount.Name)
	}

	if toAccount.IsRestricted {
		return fmt.Errorf("destination account '%s' is restricted", toAccount.Name)
	}

	if amount <= 0 {
		return errors.New("transfer amount must be greater than zero")
	}

	if amount > MaxSingleTransactionLimit {
		return fmt.Errorf("transfer amount exceeds maximum limit of %.2f", MaxSingleTransactionLimit)
	}

	// Check balance constraints
	if err := v.ValidateBalance(fromAccount, -amount); err != nil {
		return fmt.Errorf("insufficient balance: %v", err)
	}

	// Check daily limit if set
	if fromAccount.DailyLimit > 0 && amount > fromAccount.DailyLimit {
		return fmt.Errorf("transfer amount exceeds daily limit of %.2f", fromAccount.DailyLimit)
	}

	return nil
}

// ValidateBalance validates if the balance operation is allowed
func (v *CashBankValidator) ValidateBalance(account *models.CashBank, amountChange float64) error {
	newBalance := account.Balance + amountChange

	// Check minimum balance
	if account.MinBalance > 0 && newBalance < account.MinBalance {
		return fmt.Errorf("balance would fall below minimum required balance of %.2f", account.MinBalance)
	}

	// Check maximum balance
	if account.MaxBalance > 0 && newBalance > account.MaxBalance {
		return fmt.Errorf("balance would exceed maximum allowed balance of %.2f", account.MaxBalance)
	}

	// Account type specific validation
	switch account.Type {
	case models.CashBankTypeCash:
		if newBalance < MinCashBalance {
			return errors.New("cash account cannot have negative balance")
		}
		if newBalance > MaxCashBalance {
			return fmt.Errorf("balance exceeds maximum cash limit of %.2f", MaxCashBalance)
		}
	case models.CashBankTypeBank:
		if newBalance < MinBankBalance {
			return fmt.Errorf("balance would exceed overdraft limit of %.2f", -MinBankBalance)
		}
		if newBalance > MaxBankBalance {
			return fmt.Errorf("balance exceeds maximum bank limit of %.2f", MaxBankBalance)
		}
	}

	return nil
}

// ValidateAccountCode validates account code format
func (v *CashBankValidator) ValidateAccountCode(code string) error {
	if code == "" {
		return errors.New("account code is required")
	}

	// Check format: PREFIX-YYYY-NNNN
	pattern := `^(CSH|BNK)-\d{4}-\d{4}$`
	matched, err := regexp.MatchString(pattern, code)
	if err != nil {
		return err
	}

	if !matched {
		return errors.New("invalid account code format. Expected: CSH-YYYY-NNNN or BNK-YYYY-NNNN")
	}

	return nil
}

// ValidateAccountName validates account name
func (v *CashBankValidator) ValidateAccountName(name string) error {
	if name == "" {
		return errors.New("account name is required")
	}

	name = strings.TrimSpace(name)
	if len(name) < 3 {
		return errors.New("account name must be at least 3 characters")
	}

	if len(name) > 100 {
		return errors.New("account name must not exceed 100 characters")
	}

	// Check for valid characters (letters, numbers, spaces, hyphens, underscores)
	pattern := `^[a-zA-Z0-9\s\-_]+$`
	matched, err := regexp.MatchString(pattern, name)
	if err != nil {
		return err
	}

	if !matched {
		return errors.New("account name contains invalid characters")
	}

	return nil
}

// ValidateCurrency validates currency code
func (v *CashBankValidator) ValidateCurrency(currency string) error {
	if currency == "" {
		return nil // Will use default
	}

	currency = strings.ToUpper(currency)
	for _, supported := range SupportedCurrencies {
		if currency == supported {
			return nil
		}
	}

	return fmt.Errorf("unsupported currency: %s. Supported currencies: %s",
		currency, strings.Join(SupportedCurrencies, ", "))
}

// ValidateBankAccountNumber validates bank account number format
func (v *CashBankValidator) ValidateBankAccountNumber(accountNo string) error {
	if accountNo == "" {
		return nil // Optional for cash accounts
	}

	// Remove spaces and hyphens for validation
	cleaned := strings.ReplaceAll(strings.ReplaceAll(accountNo, " ", ""), "-", "")

	// Check length (Indonesian bank accounts are typically 10-16 digits)
	if len(cleaned) < 6 || len(cleaned) > 20 {
		return errors.New("bank account number must be 6-20 digits")
	}

	// Check that it contains only digits
	pattern := `^\d+$`
	matched, err := regexp.MatchString(pattern, cleaned)
	if err != nil {
		return err
	}

	if !matched {
		return errors.New("bank account number must contain only digits")
	}

	return nil
}

// ValidateTransactionAmount validates transaction amount
func (v *CashBankValidator) ValidateTransactionAmount(amount float64, transactionType string) error {
	if amount <= 0 {
		return fmt.Errorf("%s amount must be greater than zero", transactionType)
	}

	if amount > MaxSingleTransactionLimit {
		return fmt.Errorf("%s amount exceeds maximum transaction limit of %.2f",
			transactionType, MaxSingleTransactionLimit)
	}

	// Check for reasonable precision (max 2 decimal places)
	if amount*100 != float64(int64(amount*100)) {
		return fmt.Errorf("%s amount can have at most 2 decimal places", transactionType)
	}

	return nil
}

// IsValidCashBankType checks if the provided type is valid
func IsValidCashBankType(accountType string) bool {
	return accountType == models.CashBankTypeCash || accountType == models.CashBankTypeBank
}

// FormatCurrency formats amount with currency symbol
func FormatCurrency(amount float64, currency string) string {
	switch currency {
	case "IDR":
		return fmt.Sprintf("Rp %.2f", amount)
	case "USD":
		return fmt.Sprintf("$%.2f", amount)
	case "CNY":
		return fmt.Sprintf("¥%.2f", amount)
	case "EUR":
		return fmt.Sprintf("€%.2f", amount)
	default:
		return fmt.Sprintf("%.2f %s", amount, currency)
	}
}

// SanitizeAccountCode ensures the account code follows proper format
func SanitizeAccountCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// SanitizeAccountName ensures the account name is properly formatted
func SanitizeAccountName(name string) string {
	// Trim whitespace and normalize spaces
	name = strings.TrimSpace(name)
	// Replace multiple spaces with single space
	pattern := regexp.MustCompile(`\s+`)
	name = pattern.ReplaceAllString(name, " ")
	return name
}
