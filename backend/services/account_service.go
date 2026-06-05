package services

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/utils"
	"context"
	"fmt"
	"strconv"
	"strings"
)

// AccountService handles account business logic
type AccountService interface {
	CreateAccount(ctx context.Context, req *models.AccountCreateRequest) (*models.Account, error)
	UpdateAccount(ctx context.Context, code string, req *models.AccountUpdateRequest) (*models.Account, error)
	DeleteAccount(ctx context.Context, code string) error
	GetAccount(ctx context.Context, code string) (*models.Account, error)
	ListAccounts(ctx context.Context, accountType string) ([]models.Account, error)
	GetAccountHierarchy(ctx context.Context) ([]models.Account, error)
	GetBalanceSummary(ctx context.Context) ([]models.AccountSummaryResponse, error)
	BulkImportAccounts(ctx context.Context, accounts []models.AccountImportRequest) error
	GenerateAccountCode(ctx context.Context, accountType, parentCode string) (string, error)
	ValidateAccountHierarchy(ctx context.Context, parentID *uint, accountType string) error
	GetRevenueAccounts(ctx context.Context) ([]models.Account, error)
	GetEquityAccounts(ctx context.Context) ([]models.Account, error)
}

// AccountServiceImpl implements AccountService
type AccountServiceImpl struct {
	accountRepo repositories.AccountRepository
}

// NewAccountService creates a new account service
func NewAccountService(accountRepo repositories.AccountRepository) AccountService {
	return &AccountServiceImpl{
		accountRepo: accountRepo,
	}
}

// CreateAccount creates a new account with validation
func (s *AccountServiceImpl) CreateAccount(ctx context.Context, req *models.AccountCreateRequest) (*models.Account, error) {
	// Validate account type
	if !models.IsValidAccountType(string(req.Type)) {
		return nil, utils.NewValidationError("Invalid account type", map[string]string{
			"type": "Must be one of: ASSET, LIABILITY, EQUITY, REVENUE, EXPENSE",
		})
	}

	// Validate parent hierarchy if parent is specified
	var parent *models.Account
	if req.ParentID != nil {
		if err := s.ValidateAccountHierarchy(ctx, req.ParentID, string(req.Type)); err != nil {
			return nil, err
		}

		resolvedParent, err := s.accountRepo.FindByID(ctx, *req.ParentID)
		if err != nil {
			return nil, err
		}
		parent = resolvedParent
	}

	// Generate account code if not provided
	if req.Code == "" {
		var parentCode string
		if parent != nil {
			parentCode = parent.Code
		}

		code, err := s.GenerateAccountCode(ctx, string(req.Type), parentCode)
		if err != nil {
			return nil, err
		}
		req.Code = code
	} else {
		req.Code = strings.TrimSpace(req.Code)
		if parent != nil {
			normalizedCode, err := normalizeAccountCodeWithParentCode(req.Code, parent.Code)
			if err != nil {
				return nil, err
			}
			req.Code = normalizedCode
		}

		// Validate provided code format
		if err := s.ValidateAccountCodeFormat(req.Code, string(req.Type)); err != nil {
			return nil, err
		}
	}

	return s.accountRepo.Create(ctx, req)
}

// UpdateAccount updates an account
func (s *AccountServiceImpl) UpdateAccount(ctx context.Context, code string, req *models.AccountUpdateRequest) (*models.Account, error) {
	// Check if account exists
	existingAccount, err := s.accountRepo.FindByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	// Prevent deactivating accounts with children or transactions
	if req.IsActive != nil && !*req.IsActive {
		// Check for child accounts
		if len(existingAccount.Children) > 0 {
			return nil, utils.NewBadRequestError("Cannot deactivate account that has child accounts")
		}

		// Check for transactions (this would need to be implemented based on your transaction model)
		// For now, we'll skip this check
	}

	return s.accountRepo.Update(ctx, code, req)
}

// DeleteAccount deletes an account with validation
func (s *AccountServiceImpl) DeleteAccount(ctx context.Context, code string) error {
	return s.accountRepo.Delete(ctx, code)
}

// GetAccount gets a single account
func (s *AccountServiceImpl) GetAccount(ctx context.Context, code string) (*models.Account, error) {
	return s.accountRepo.FindByCode(ctx, code)
}

// ListAccounts lists accounts with optional filtering
func (s *AccountServiceImpl) ListAccounts(ctx context.Context, accountType string) ([]models.Account, error) {
	if accountType != "" {
		return s.accountRepo.FindByType(ctx, accountType)
	}
	return s.accountRepo.FindAll(ctx)
}

// GetAccountHierarchy gets account hierarchy
func (s *AccountServiceImpl) GetAccountHierarchy(ctx context.Context) ([]models.Account, error) {
	return s.accountRepo.GetHierarchy(ctx)
}

// GetBalanceSummary gets balance summary
func (s *AccountServiceImpl) GetBalanceSummary(ctx context.Context) ([]models.AccountSummaryResponse, error) {
	return s.accountRepo.GetBalanceSummary(ctx)
}

// BulkImportAccounts imports multiple accounts with validation
func (s *AccountServiceImpl) BulkImportAccounts(ctx context.Context, accounts []models.AccountImportRequest) error {
	// Validate all accounts before importing
	codeMap := make(map[string]bool)

	for i, account := range accounts {
		// Check for duplicate codes in the import
		if codeMap[account.Code] {
			return utils.NewBadRequestError(fmt.Sprintf("Duplicate account code in import: %s at row %d", account.Code, i+1))
		}
		codeMap[account.Code] = true

		// Validate account type
		if !models.IsValidAccountType(string(account.Type)) {
			return utils.NewValidationError(fmt.Sprintf("Invalid account type at row %d", i+1), map[string]string{
				"type": "Must be one of: ASSET, LIABILITY, EQUITY, REVENUE, EXPENSE",
			})
		}
	}

	return s.accountRepo.BulkImport(ctx, accounts)
}

// GenerateAccountCode generates account code based on type and parent following PSAK standards
func (s *AccountServiceImpl) GenerateAccountCode(ctx context.Context, accountType, parentCode string) (string, error) {
	// If no parent code, generate main account code
	if parentCode == "" {
		return s.generateMainAccountCode(ctx, accountType)
	}

	// If parent code exists, generate child account code with PSAK format
	return s.generateChildAccountCode(ctx, parentCode, accountType)
}

// generateMainAccountCode generates main account codes (4 digits) based on account type
func (s *AccountServiceImpl) generateMainAccountCode(ctx context.Context, accountType string) (string, error) {
	var startRange, endRange int

	// Define account type ranges following PSAK
	switch accountType {
	case models.AccountTypeAsset:
		startRange, endRange = 1000, 1999
	case models.AccountTypeLiability:
		startRange, endRange = 2000, 2999
	case models.AccountTypeEquity:
		startRange, endRange = 3000, 3999
	case models.AccountTypeRevenue:
		startRange, endRange = 4000, 4999 // Auto-generate in primary range (4xxx)
	case models.AccountTypeExpense:
		startRange, endRange = 5000, 5999 // Auto-generate in primary range (5xxx)
	default:
		return "", utils.NewValidationError("Invalid account type for code generation", nil)
	}

	// Get all accounts of this type to find the next available code
	accounts, err := s.accountRepo.FindByType(ctx, accountType)
	if err != nil {
		return "", err
	}

	// Find the highest existing main account code
	maxCode := startRange
	for _, account := range accounts {
		// Only consider main account codes (4 digits, no dash)
		if !strings.Contains(account.Code, "-") && len(account.Code) == 4 {
			if code, err := strconv.Atoi(account.Code); err == nil {
				if code >= startRange && code < endRange && code > maxCode {
					maxCode = code
				}
			}
		}
	}

	// Generate next available main account code
	nextCode := maxCode + 1
	if nextCode > endRange {
		return "", utils.NewValidationError(fmt.Sprintf("Account code range exhausted for type %s", accountType), nil)
	}

	return fmt.Sprintf("%04d", nextCode), nil
}

// generateChildAccountCode generates child account codes following PSAK format (parent-xxx)
func (s *AccountServiceImpl) generateChildAccountCode(ctx context.Context, parentCode, accountType string) (string, error) {
	// Get all accounts of this type to find existing child codes
	accounts, err := s.accountRepo.FindByType(ctx, accountType)
	if err != nil {
		return "", err
	}

	// Find the highest existing child code for this parent
	maxChildNumber := 0
	childPrefix := parentCode + "-"

	for _, account := range accounts {
		if strings.HasPrefix(account.Code, childPrefix) {
			// Extract child number (e.g., "001" from "1101-001")
			childPart := strings.TrimPrefix(account.Code, childPrefix)
			if len(childPart) == 3 { // Expected format: xxx
				if num, err := strconv.Atoi(childPart); err == nil && num > maxChildNumber {
					maxChildNumber = num
				}
			}
		}
	}

	// Generate next child code with 3-digit format
	nextChildNumber := maxChildNumber + 1
	if nextChildNumber > 999 {
		return "", utils.NewValidationError(fmt.Sprintf("Child account limit reached for parent %s", parentCode), nil)
	}

	return fmt.Sprintf("%s-%03d", parentCode, nextChildNumber), nil
}

// getValidCodeRanges returns all valid code ranges for a given account type.
// PSAK-compliant: 7xxx = Other Income (REVENUE), 8xxx = Other Expense, 9xxx = Tax.
func getValidCodeRanges(accountType string) []struct{ Min, Max int } {
	switch accountType {
	case models.AccountTypeAsset:
		return []struct{ Min, Max int }{{1000, 1999}}
	case models.AccountTypeLiability:
		return []struct{ Min, Max int }{{2000, 2999}}
	case models.AccountTypeEquity:
		return []struct{ Min, Max int }{{3000, 3999}}
	case models.AccountTypeRevenue:
		return []struct{ Min, Max int }{
			{4000, 4999}, // Operating Revenue
			{7000, 7999}, // Other Income (PENDAPATAN LAIN-LAIN)
		}
	case models.AccountTypeExpense:
		return []struct{ Min, Max int }{
			{5000, 5999}, // COGS / Beban Pokok
			{6000, 6999}, // Operating Expenses
			{8000, 8999}, // Other Expenses (BEBAN LAIN-LAIN)
			{9000, 9999}, // Tax Expenses (PAJAK PENGHASILAN)
		}
	default:
		return nil
	}
}

// isCodeInValidRanges checks if a numeric code falls within any valid range for the account type
func isCodeInValidRanges(codeNum int, accountType string) bool {
	ranges := getValidCodeRanges(accountType)
	for _, r := range ranges {
		if codeNum >= r.Min && codeNum <= r.Max {
			return true
		}
	}
	return false
}

// getValidPrefixes returns all valid first-digit prefixes for an account type
func getValidPrefixes(accountType string) []string {
	switch accountType {
	case models.AccountTypeAsset:
		return []string{"1"}
	case models.AccountTypeLiability:
		return []string{"2"}
	case models.AccountTypeEquity:
		return []string{"3"}
	case models.AccountTypeRevenue:
		return []string{"4", "7"}
	case models.AccountTypeExpense:
		return []string{"5", "6", "8", "9"}
	default:
		return nil
	}
}

// ValidateAccountCodeFormat validates account code format according to PSAK standards
func (s *AccountServiceImpl) ValidateAccountCodeFormat(code, accountType string) error {
	code = strings.TrimSpace(code)

	// Check if code is empty
	if code == "" {
		return utils.NewValidationError("Account code cannot be empty", nil)
	}

	// Check if code starts with a valid prefix for this account type
	validPrefixes := getValidPrefixes(accountType)
	if validPrefixes == nil {
		return utils.NewValidationError("Invalid account type for code validation", nil)
	}

	hasValidPrefix := false
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(code, prefix) {
			hasValidPrefix = true
			break
		}
	}
	if !hasValidPrefix {
		return utils.NewValidationError(fmt.Sprintf("Account code for %s must start with one of: %s", accountType, strings.Join(validPrefixes, ", ")), nil)
	}

	if len(code) > 20 {
		return utils.NewValidationError("Account code cannot exceed 20 characters", nil)
	}

	if strings.Contains(code, "-") {
		return nil
	}

	if len(code) != 4 {
		return utils.NewValidationError("Root account code must be 4 digits", nil)
	}

	codeNum, err := strconv.Atoi(code)
	if err != nil {
		return utils.NewValidationError("Root account code must be numeric", nil)
	}

	if !isCodeInValidRanges(codeNum, accountType) {
		ranges := getValidCodeRanges(accountType)
		rangeStrs := make([]string, len(ranges))
		for i, r := range ranges {
			rangeStrs[i] = fmt.Sprintf("%d-%d", r.Min, r.Max)
		}
		return utils.NewValidationError(fmt.Sprintf("%s account codes must be in ranges: %s", accountType, strings.Join(rangeStrs, ", ")), nil)
	}

	return nil
}

func normalizeAccountCodeWithParentCode(code, parentCode string) (string, error) {
	code = strings.TrimSpace(code)

	if code == parentCode || isCompleteNumericChildCodeForParent(code, parentCode) {
		return code, nil
	}

	parentPrefix := parentCode + "-"
	if strings.HasPrefix(code, parentPrefix) {
		return code, nil
	}

	return parentPrefix + code, nil
}

func isCompleteNumericChildCodeForParent(code, parentCode string) bool {
	code = strings.TrimSpace(code)
	parentCode = strings.TrimSpace(parentCode)
	if len(code) != 4 || len(parentCode) != 4 || !isNumericSegment(code) || !isNumericSegment(parentCode) || code == parentCode {
		return false
	}

	if strings.HasSuffix(parentCode, "000") {
		return strings.HasPrefix(code, parentCode[:1])
	}

	if strings.HasSuffix(parentCode, "00") {
		return strings.HasPrefix(code, parentCode[:2])
	}

	return false
}

func isNumericSegment(value string) bool {
	if value == "" {
		return false
	}

	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

// ValidateAccountHierarchy validates parent-child account relationships
func (s *AccountServiceImpl) ValidateAccountHierarchy(ctx context.Context, parentID *uint, accountType string) error {
	if parentID == nil {
		return nil
	}

	parent, err := s.accountRepo.FindByID(ctx, *parentID)
	if err != nil {
		return err
	}

	// Validate that parent and child have compatible types
	// Asset accounts can have asset children
	// Liability accounts can have liability children
	// etc.
	if parent.Type != accountType {
		return utils.NewValidationError("Parent and child accounts must be of the same type", map[string]string{
			"parent_type": parent.Type,
			"child_type":  accountType,
		})
	}

	// Prevent creating deep hierarchies (max 4 levels)
	if parent.Level >= 4 {
		return utils.NewValidationError("Maximum account hierarchy depth exceeded", map[string]string{
			"max_depth":    "4",
			"parent_level": fmt.Sprintf("%d", parent.Level),
		})
	}

	return nil
}

// GetRevenueAccounts gets all active revenue accounts for deposit source selection
func (s *AccountServiceImpl) GetRevenueAccounts(ctx context.Context) ([]models.Account, error) {
	return s.accountRepo.FindByType(ctx, models.AccountTypeRevenue)
}

// GetEquityAccounts gets all active equity accounts for capital investment source selection
func (s *AccountServiceImpl) GetEquityAccounts(ctx context.Context) ([]models.Account, error) {
	return s.accountRepo.FindByType(ctx, models.AccountTypeEquity)
}
