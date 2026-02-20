package services

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

// AccountCodeGenerator handles automatic generation of sequential account codes
type AccountCodeGenerator struct {
	db *gorm.DB
}

// NewAccountCodeGenerator creates a new account code generator
func NewAccountCodeGenerator(db *gorm.DB) *AccountCodeGenerator {
	return &AccountCodeGenerator{db: db}
}

// AccountCodeRule defines the structure for account code generation rules
type AccountCodeRule struct {
	AccountType   string
	StartRange    int
	EndRange      int
	DefaultDigits int
	Description   string
}

// Standard accounting code structure
var standardCodeRules = []AccountCodeRule{
	{AccountType: "ASSET", StartRange: 1000, EndRange: 1999, DefaultDigits: 4, Description: "Asset accounts"},
	{AccountType: "LIABILITY", StartRange: 2000, EndRange: 2999, DefaultDigits: 4, Description: "Liability accounts"},
	{AccountType: "EQUITY", StartRange: 3000, EndRange: 3999, DefaultDigits: 4, Description: "Equity accounts"},
	{AccountType: "REVENUE", StartRange: 4000, EndRange: 4999, DefaultDigits: 4, Description: "Revenue accounts"},
	{AccountType: "EXPENSE", StartRange: 5000, EndRange: 5999, DefaultDigits: 4, Description: "Expense accounts"},
}

// GenerateNextCode generates the next sequential account code
func (g *AccountCodeGenerator) GenerateNextCode(accountType string, parentCode string) (string, error) {
	// Get the rule for this account type
	rule, err := g.getRuleForAccountType(accountType)
	if err != nil {
		return "", err
	}

	// If there's a parent account, generate a child code
	if parentCode != "" {
		return g.generateChildCode(parentCode)
	}

	// Generate a main account code
	return g.generateMainAccountCode(rule)
}

// generateMainAccountCode generates a main account code for the given rule
func (g *AccountCodeGenerator) generateMainAccountCode(rule AccountCodeRule) (string, error) {
	// Get all existing codes in this range
	var existingCodes []string
	err := g.db.Model(&models.Account{}).
		Where("code LIKE ? AND LENGTH(code) = ?",
			fmt.Sprintf("%d%%", rule.StartRange/1000), rule.DefaultDigits).
		Pluck("code", &existingCodes).Error

	if err != nil {
		return "", fmt.Errorf("failed to get existing codes: %v", err)
	}

	// Find the next available code
	for i := rule.StartRange; i <= rule.EndRange; i += 100 {
		candidate := fmt.Sprintf("%d", i)
		if !g.codeExists(existingCodes, candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("no available codes in range %d-%d", rule.StartRange, rule.EndRange)
}

// generateChildCode generates a child code based on parent code
func (g *AccountCodeGenerator) generateChildCode(parentCode string) (string, error) {
	// Get all existing child codes for this parent
	var existingCodes []string
	err := g.db.Model(&models.Account{}).
		Where("code LIKE ?", parentCode+"%").
		Where("code != ?", parentCode).
		Pluck("code", &existingCodes).Error

	if err != nil {
		return "", fmt.Errorf("failed to get existing child codes: %v", err)
	}

	// Determine the next child code format
	parentLen := len(parentCode)

	// For 4-digit parent (e.g., 1100), child should be 1101, 1102, etc.
	if parentLen == 4 {
		return g.generateSequentialChild(parentCode, existingCodes, 1)
	}

	// For 3-digit parent (e.g., 110), child should be 1101, 1102, etc.
	if parentLen == 3 {
		return g.generateSequentialChild(parentCode, existingCodes, 1)
	}

	// Default: add sequential number
	return g.generateSequentialChild(parentCode, existingCodes, 1)
}

// generateSequentialChild generates sequential child codes
func (g *AccountCodeGenerator) generateSequentialChild(parentCode string, existingCodes []string, increment int) (string, error) {
	// Parse existing child numbers
	var childNumbers []int

	for _, code := range existingCodes {
		if strings.HasPrefix(code, parentCode) {
			suffix := strings.TrimPrefix(code, parentCode)

			// Handle different formats
			if strings.HasPrefix(suffix, "-") {
				// Handle dash format (e.g., 1100-001)
				numberStr := strings.TrimPrefix(suffix, "-")
				if num, err := strconv.Atoi(numberStr); err == nil {
					childNumbers = append(childNumbers, num)
				}
			} else {
				// Handle direct numeric format (e.g., 1101, 1102)
				if num, err := strconv.Atoi(suffix); err == nil {
					childNumbers = append(childNumbers, num)
				} else if len(suffix) > 0 {
					// Handle single digit increment (1100 -> 1101)
					if num, err := strconv.Atoi(code); err == nil {
						parentNum, err2 := strconv.Atoi(parentCode)
						if err2 == nil && num > parentNum {
							childNumbers = append(childNumbers, num-parentNum)
						}
					}
				}
			}
		}
	}

	// Find next available number
	sort.Ints(childNumbers)

	nextNum := increment
	for _, num := range childNumbers {
		if num == nextNum {
			nextNum++
		} else {
			break
		}
	}

	// Determine format based on parent code structure and existing patterns
	hasChildWithDash := g.hasChildWithDashFormat(existingCodes, parentCode)

	if hasChildWithDash {
		// Use dash format with zero padding
		return fmt.Sprintf("%s-%03d", parentCode, nextNum), nil
	}

	// Use direct sequential format
	parentNum, err := strconv.Atoi(parentCode)
	if err != nil {
		return "", fmt.Errorf("invalid parent code format: %s", parentCode)
	}

	return fmt.Sprintf("%d", parentNum+nextNum), nil
}

// hasChildWithDashFormat checks if existing children use dash format
func (g *AccountCodeGenerator) hasChildWithDashFormat(existingCodes []string, parentCode string) bool {
	for _, code := range existingCodes {
		if strings.HasPrefix(code, parentCode+"-") {
			return true
		}
	}
	return false
}

// ValidateAccountCode validates if a code follows proper hierarchy rules
func (g *AccountCodeGenerator) ValidateAccountCode(code string, accountType string, parentCode string) error {
	// Check if code format is valid
	if len(code) == 0 {
		return fmt.Errorf("account code cannot be empty")
	}

	// Check account type range
	rule, err := g.getRuleForAccountType(accountType)
	if err != nil {
		return err
	}

	// Parse first digits to check range
	var firstDigits int
	if len(code) >= 1 {
		firstDigits, _ = strconv.Atoi(string(code[0]))
	}

	expectedFirstDigit := rule.StartRange / 1000
	if firstDigits != expectedFirstDigit {
		return fmt.Errorf("account code %s does not match account type %s (should start with %d)",
			code, accountType, expectedFirstDigit)
	}

	// Check if parent-child relationship is valid
	if parentCode != "" {
		if !strings.HasPrefix(code, parentCode) && !strings.HasPrefix(code, parentCode+"-") {
			return fmt.Errorf("child account code %s does not properly inherit from parent %s",
				code, parentCode)
		}
	}

	// Check if code already exists
	exists, err := g.CodeExists(code)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("account code %s already exists", code)
	}

	return nil
}

// CodeExists checks if an account code already exists
func (g *AccountCodeGenerator) CodeExists(code string) (bool, error) {
	var count int64
	err := g.db.Model(&models.Account{}).Where("code = ?", code).Count(&count).Error
	return count > 0, err
}

// codeExists checks if code exists in the provided slice
func (g *AccountCodeGenerator) codeExists(codes []string, code string) bool {
	for _, c := range codes {
		if c == code {
			return true
		}
	}
	return false
}

// getRuleForAccountType gets the code generation rule for an account type
func (g *AccountCodeGenerator) getRuleForAccountType(accountType string) (AccountCodeRule, error) {
	for _, rule := range standardCodeRules {
		if rule.AccountType == accountType {
			return rule, nil
		}
	}
	return AccountCodeRule{}, fmt.Errorf("no rule found for account type: %s", accountType)
}

// GetRecommendedCode gets a recommended code for new account
func (g *AccountCodeGenerator) GetRecommendedCode(accountType string, parentCode string, accountName string) (string, error) {
	code, err := g.GenerateNextCode(accountType, parentCode)
	if err != nil {
		return "", err
	}

	// Add some intelligence based on account name
	code = g.optimizeCodeBasedOnName(code, accountName, accountType)

	return code, nil
}

// optimizeCodeBasedOnName provides code suggestions based on account name
func (g *AccountCodeGenerator) optimizeCodeBasedOnName(code string, name string, accountType string) string {
	name = strings.ToLower(name)

	// Special cases for common account names
	switch accountType {
	case "ASSET":
		if strings.Contains(name, "kas") && !strings.Contains(name, "kecil") {
			return g.suggestSpecificCode("1101", code)
		}
		if strings.Contains(name, "kas") && strings.Contains(name, "kecil") {
			return g.suggestSpecificCode("1101-001", code)
		}
		if strings.Contains(name, "bank") {
			return g.suggestSpecificCode("1101-003", code)
		}
		if strings.Contains(name, "piutang") {
			return g.suggestSpecificCode("1201", code)
		}
		if strings.Contains(name, "persediaan") || strings.Contains(name, "inventory") {
			return g.suggestSpecificCode("1103-001", code)
		}

	case "LIABILITY":
		if strings.Contains(name, "utang") {
			return g.suggestSpecificCode("2101", code)
		}
		if strings.Contains(name, "ppn") && strings.Contains(name, "keluaran") {
			return g.suggestSpecificCode("2102-001", code)
		}

	case "REVENUE":
		if strings.Contains(name, "penjualan") || strings.Contains(name, "sales") {
			return g.suggestSpecificCode("4101", code)
		}

	case "EXPENSE":
		if strings.Contains(name, "gaji") || strings.Contains(name, "salary") {
			return g.suggestSpecificCode("5101", code)
		}
		if strings.Contains(name, "listrik") || strings.Contains(name, "electricity") {
			return g.suggestSpecificCode("5201", code)
		}
	}

	return code
}

// suggestSpecificCode suggests a specific code if available, otherwise returns the generated code
func (g *AccountCodeGenerator) suggestSpecificCode(preferred string, fallback string) string {
	exists, err := g.CodeExists(preferred)
	if err != nil || exists {
		return fallback
	}
	return preferred
}

// FixNonStandardCodes identifies and suggests fixes for non-standard codes
func (g *AccountCodeGenerator) FixNonStandardCodes() ([]AccountCodeFix, error) {
	var fixes []AccountCodeFix

	// Find accounts with non-standard codes
	var accounts []models.Account
	err := g.db.Find(&accounts).Error
	if err != nil {
		return nil, err
	}

	for _, account := range accounts {
		if g.isNonStandardCode(account.Code, account.Type) {
			suggestedCode, err := g.GetRecommendedCode(account.Type, "", account.Name)
			if err == nil {
				fixes = append(fixes, AccountCodeFix{
					CurrentCode:   account.Code,
					SuggestedCode: suggestedCode,
					AccountName:   account.Name,
					AccountType:   account.Type,
					Reason:        g.getNonStandardReason(account.Code, account.Type),
				})
			}
		}
	}

	return fixes, nil
}

// AccountCodeFix represents a suggested fix for non-standard codes
type AccountCodeFix struct {
	CurrentCode   string `json:"current_code"`
	SuggestedCode string `json:"suggested_code"`
	AccountName   string `json:"account_name"`
	AccountType   string `json:"account_type"`
	Reason        string `json:"reason"`
}

// isNonStandardCode checks if a code doesn't follow standard patterns
func (g *AccountCodeGenerator) isNonStandardCode(code string, accountType string) bool {
	// Check for dash in main account codes (non-standard)
	if strings.Contains(code, "-") {
		return true
	}

	// Check if first digit matches account type
	rule, err := g.getRuleForAccountType(accountType)
	if err != nil {
		return true
	}

	if len(code) == 0 {
		return true
	}

	firstDigit, err := strconv.Atoi(string(code[0]))
	if err != nil {
		return true
	}

	expectedFirstDigit := rule.StartRange / 1000
	return firstDigit != expectedFirstDigit
}

// getNonStandardReason provides reason for non-standard code
func (g *AccountCodeGenerator) getNonStandardReason(code string, accountType string) string {
	if strings.Contains(code, "-") {
		return "Contains dash in main account code (should use sequential numbers)"
	}

	rule, err := g.getRuleForAccountType(accountType)
	if err != nil {
		return "Unknown account type"
	}

	if len(code) == 0 {
		return "Empty code"
	}

	firstDigit, err := strconv.Atoi(string(code[0]))
	if err != nil {
		return "Invalid numeric format"
	}

	expectedFirstDigit := rule.StartRange / 1000
	if firstDigit != expectedFirstDigit {
		return fmt.Sprintf("Should start with %d for %s accounts", expectedFirstDigit, accountType)
	}

	return "Unknown issue"
}
