package repositories

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// AccountRepository defines account-related database operations
type AccountRepository interface {
	Create(ctx context.Context, req *models.AccountCreateRequest) (*models.Account, error)
	Update(ctx context.Context, code string, req *models.AccountUpdateRequest) (*models.Account, error)
	Delete(ctx context.Context, code string) error
	AdminDelete(ctx context.Context, code string, cascadeDelete bool, newParentID *uint) error // Admin-only delete with cascade options
	FindByCode(ctx context.Context, code string) (*models.Account, error)
	GetAccountByCode(code string) (*models.Account, error)
	FindByID(ctx context.Context, id uint) (*models.Account, error)
	FindAll(ctx context.Context) ([]models.Account, error)
	FindByType(ctx context.Context, accountType string) ([]models.Account, error)
	FindWithFilter(ctx context.Context, accountType *string, category *string) ([]models.Account, error)
	GetHierarchy(ctx context.Context) ([]models.Account, error)
	BulkImport(ctx context.Context, accounts []models.AccountImportRequest) error
	CalculateBalance(ctx context.Context, accountID uint) (float64, error)
	UpdateBalance(ctx context.Context, accountID uint, debitAmount, creditAmount float64) error
	GetBalanceSummary(ctx context.Context) ([]models.AccountSummaryResponse, error)
}

// AccountRepo implements AccountRepository
type AccountRepo struct {
	*BaseRepo
}

// NewAccountRepository creates a new account repository
func NewAccountRepository(db *gorm.DB) AccountRepository {
	return &AccountRepo{
		BaseRepo: &BaseRepo{DB: db},
	}
}

// NewAccountRepo creates a new account repository returning concrete type
func NewAccountRepo(db *gorm.DB) *AccountRepo {
	return &AccountRepo{
		BaseRepo: &BaseRepo{DB: db},
	}
}

// Create creates a new account
func (r *AccountRepo) Create(ctx context.Context, req *models.AccountCreateRequest) (*models.Account, error) {
	// Validate account type
	if !models.IsValidAccountType(string(req.Type)) {
		return nil, utils.NewValidationError("Invalid account type", map[string]string{
			"type": "Must be one of: ASSET, LIABILITY, EQUITY, REVENUE, EXPENSE",
		})
	}

	// Check if code already exists (only check non-deleted accounts)
	var existingAccount models.Account
	if err := r.DB.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Where("code = ? AND deleted_at IS NULL", req.Code).First(&existingAccount).Error; err == nil {
		errorMsg := fmt.Sprintf("Account code '%s' already exists (used by: %s)", req.Code, existingAccount.Name)
		return nil, utils.NewConflictError(errorMsg)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, utils.NewDatabaseError("check existing code", err)
	}

	// Validate header account specific rules
	if req.IsHeader != nil && *req.IsHeader {
		// Header accounts cannot have opening balance
		if req.OpeningBalance != 0 {
			return nil, utils.NewValidationError("Header accounts cannot have opening balance", map[string]string{
				"opening_balance": "Header accounts must have zero opening balance",
			})
		}
	}

	// Calculate level if parent exists
	level := 1
	if req.ParentID != nil {
		var parent models.Account
		if err := r.DB.WithContext(ctx).First(&parent, *req.ParentID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, utils.NewNotFoundError("Parent account")
			}
			return nil, utils.NewDatabaseError("find parent account", err)
		}
		level = parent.Level + 1
		
		// Set parent as header since it now has children
		if err := r.DB.WithContext(ctx).Model(&parent).Update("is_header", true).Error; err != nil {
			return nil, utils.NewDatabaseError("update parent header status", err)
		}
	}

	// Determine if this is a header account
	isHeader := false
	if req.IsHeader != nil {
		isHeader = *req.IsHeader
	}

	account := &models.Account{
		Code:        req.Code,
		Name:        req.Name,
		Type:        string(req.Type),
		Category:    req.Category,
		ParentID:    req.ParentID,
		Level:       level,
		Description: req.Description,
		Balance:     req.OpeningBalance,
		IsActive:    true,
		IsHeader:    isHeader,
	}

	if err := r.DB.WithContext(ctx).Create(account).Error; err != nil {
		return nil, utils.NewDatabaseError("create account", err)
	}

	return account, nil
}

// Update updates an account
func (r *AccountRepo) Update(ctx context.Context, code string, req *models.AccountUpdateRequest) (*models.Account, error) {
	// Add timeout to prevent hanging
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	
	log.Printf("UpdateAccount called with code: %s", code)
	log.Printf("Update request data: %+v", req)
	
	var account models.Account
	if err := r.DB.WithContext(ctx).Where("code = ?", code).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, utils.NewNotFoundError("Account")
		}
		return nil, utils.NewDatabaseError("find account", err)
	}

	// Skip code validation if code is not being changed
	if req.Code != "" && req.Code != code {
		log.Printf("Code change detected: %s -> %s", code, req.Code)
		var existingAccount models.Account
		if err := r.DB.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Where("code = ? AND id != ?", req.Code, account.ID).First(&existingAccount).Error; err == nil {
			errorMsg := fmt.Sprintf("Account code '%s' already exists (used by: %s)", req.Code, existingAccount.Name)
			return nil, utils.NewConflictError(errorMsg)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, utils.NewDatabaseError("check existing code", err)
		}
	}

	// Validate header account specific rules for updates
	if req.IsHeader != nil && *req.IsHeader {
		// If changing to header, check if balance would be zero
		balanceToSet := account.Balance
		if req.OpeningBalance != nil {
			balanceToSet = *req.OpeningBalance
		}
		if balanceToSet != 0 {
			return nil, utils.NewValidationError("Header accounts cannot have balance", map[string]string{
				"is_header": "Account must have zero balance to become a header account",
			})
		}
	}

	// Store old parent for cleanup
	oldParentID := account.ParentID
	
	// Update fields
	if req.Code != "" {
		account.Code = req.Code
	}
	account.Name = req.Name
	account.Description = req.Description
	account.Category = req.Category
	if req.Type != "" {
		// Validate account type before updating
		if !models.IsValidAccountType(string(req.Type)) {
			return nil, utils.NewValidationError("Invalid account type", map[string]string{
				"type": "Must be one of: ASSET, LIABILITY, EQUITY, REVENUE, EXPENSE",
			})
		}
		account.Type = string(req.Type)
	}
	if req.IsActive != nil {
		account.IsActive = *req.IsActive
	}
	if req.OpeningBalance != nil {
		account.Balance = *req.OpeningBalance
	}
	if req.IsHeader != nil {
		account.IsHeader = *req.IsHeader
	}
	
	// Fast path: if only updating simple fields (name, description) without parent/structure changes
	if req.ParentID == nil && (req.Code == "" || req.Code == code) && req.Type == "" {
		log.Printf("Fast path update for account %s - only updating metadata", code)
		if err := r.DB.WithContext(ctx).Save(&account).Error; err != nil {
			return nil, utils.NewDatabaseError("update account", err)
		}
		return &account, nil
	}
	
	// Handle parent change and level recalculation
	if req.ParentID != nil {
		log.Printf("Parent change requested for account %s", code)
		// Check if parent is valid and different from current
		if oldParentID == nil || *oldParentID != *req.ParentID {
			// Validate new parent exists
			var newParent models.Account
			if err := r.DB.WithContext(ctx).First(&newParent, *req.ParentID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, utils.NewNotFoundError("New parent account")
				}
				return nil, utils.NewDatabaseError("find new parent account", err)
			}
			
			// Check for circular reference
			if err := r.checkCircularReference(ctx, account.ID, *req.ParentID); err != nil {
				return nil, err
			}
			
			// Validate parent-child type compatibility
			if newParent.Type != account.Type {
				return nil, utils.NewValidationError("Parent and child accounts must be of the same type", map[string]string{
					"parent_type": newParent.Type,
					"child_type":  account.Type,
					"message":     fmt.Sprintf("Cannot assign %s account to %s parent", account.Type, newParent.Type),
				})
			}
			
			// Update parent and level
			account.ParentID = req.ParentID
			account.Level = newParent.Level + 1
			
			// Set new parent as header if it's not already
			if !newParent.IsHeader {
				if err := r.DB.WithContext(ctx).Model(&newParent).Update("is_header", true).Error; err != nil {
					return nil, utils.NewDatabaseError("update new parent header status", err)
				}
			}
		}
	} else {
		// Setting parent to null (making it a root account)
		if oldParentID != nil {
			account.ParentID = nil
			account.Level = 1
		}
	}
	
	// Update child levels if this account has children and level changed
	if oldParentID != account.ParentID {
		if err := r.updateChildrenLevels(ctx, account.ID, account.Level); err != nil {
			return nil, utils.NewDatabaseError("update children levels", err)
		}
	}

	if err := r.DB.WithContext(ctx).Save(&account).Error; err != nil {
		return nil, utils.NewDatabaseError("update account", err)
	}
	
	// Clean up old parent header status if needed
	if oldParentID != nil && (account.ParentID == nil || (account.ParentID != nil && *oldParentID != *account.ParentID)) {
		// Check if old parent still has other children
		var siblingCount int64
		if err := r.DB.WithContext(ctx).Model(&models.Account{}).Where("parent_id = ? AND id != ?", *oldParentID, account.ID).Count(&siblingCount).Error; err != nil {
			// Log error but don't fail the update
			fmt.Printf("[WARNING] Failed to count siblings for old parent cleanup: %v\n", err)
		} else if siblingCount == 0 {
			// Old parent has no more children, unset header status
			if err := r.DB.WithContext(ctx).Model(&models.Account{}).Where("id = ?", *oldParentID).Update("is_header", false).Error; err != nil {
				// Log error but don't fail the update
				fmt.Printf("[WARNING] Failed to update old parent header status: %v\n", err)
			}
		}
	}

	return &account, nil
}

// Delete deletes an account
func (r *AccountRepo) Delete(ctx context.Context, code string) error {
	var account models.Account
	if err := r.DB.WithContext(ctx).Where("code = ?", code).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return utils.NewNotFoundError("Account")
		}
		return utils.NewDatabaseError("find account", err)
	}

	// Check if account has children
	var childrenCount int64
	if err := r.DB.WithContext(ctx).Model(&models.Account{}).Where("parent_id = ?", account.ID).Count(&childrenCount).Error; err != nil {
		return utils.NewDatabaseError("count children", err)
	}

	if childrenCount > 0 {
		return utils.NewBadRequestError("Cannot delete account that has child accounts")
	}

	// Check if account has transactions
	var transactionCount int64
	if err := r.DB.WithContext(ctx).Model(&models.Transaction{}).Where("account_id = ?", account.ID).Count(&transactionCount).Error; err != nil {
		return utils.NewDatabaseError("count transactions", err)
	}

	if transactionCount > 0 {
		return utils.NewBadRequestError("Cannot delete account that has transactions")
	}

	if err := r.DB.WithContext(ctx).Delete(&account).Error; err != nil {
		return utils.NewDatabaseError("delete account", err)
	}

	return nil
}

// AdminDelete deletes an account with admin privileges and cascade options
func (r *AccountRepo) AdminDelete(ctx context.Context, code string, cascadeDelete bool, newParentID *uint) error {
	var account models.Account
	if err := r.DB.WithContext(ctx).Where("code = ?", code).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return utils.NewNotFoundError("Account")
		}
		return utils.NewDatabaseError("find account", err)
	}

	// Get children
	var children []models.Account
	if err := r.DB.WithContext(ctx).Where("parent_id = ?", account.ID).Find(&children).Error; err != nil {
		return utils.NewDatabaseError("find child accounts", err)
	}

	// Handle children if they exist
	if len(children) > 0 {
		if cascadeDelete {
			// Recursively delete all children
			for _, child := range children {
				if err := r.AdminDelete(ctx, child.Code, true, nil); err != nil {
					return utils.NewDatabaseError(fmt.Sprintf("cascade delete child %s", child.Code), err)
				}
			}
		} else if newParentID != nil {
			// Transfer children to new parent
			var newParent models.Account
			if err := r.DB.WithContext(ctx).First(&newParent, *newParentID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return utils.NewNotFoundError("New parent account for transfer")
				}
				return utils.NewDatabaseError("find new parent account", err)
			}

			// Update all children to new parent
			for _, child := range children {
				child.ParentID = newParentID
				child.Level = newParent.Level + 1
				if err := r.DB.WithContext(ctx).Save(&child).Error; err != nil {
					return utils.NewDatabaseError(fmt.Sprintf("transfer child %s to new parent", child.Code), err)
				}
			}

			// Set new parent as header if it's not already
			if !newParent.IsHeader {
				if err := r.DB.WithContext(ctx).Model(&newParent).Update("is_header", true).Error; err != nil {
					return utils.NewDatabaseError("update new parent header status", err)
				}
			}
		} else {
			// Move children to root level (no parent)
			for _, child := range children {
				child.ParentID = nil
				child.Level = 1
				if err := r.DB.WithContext(ctx).Save(&child).Error; err != nil {
					return utils.NewDatabaseError(fmt.Sprintf("move child %s to root", child.Code), err)
				}
			}
		}
	}

	// Check if account has transactions (admin can still delete, but warn)
	var transactionCount int64
	if err := r.DB.WithContext(ctx).Model(&models.Transaction{}).Where("account_id = ?", account.ID).Count(&transactionCount).Error; err != nil {
		return utils.NewDatabaseError("count transactions", err)
	}

	if transactionCount > 0 {
		// For admin delete, we allow deletion but this should be logged/warned
		fmt.Printf("[WARNING] Admin deleting account %s with %d transactions\n", code, transactionCount)
	}

	// Delete the account
	if err := r.DB.WithContext(ctx).Delete(&account).Error; err != nil {
		return utils.NewDatabaseError("delete account", err)
	}

	// Clean up old parent header status if needed
	if account.ParentID != nil {
		// Check if old parent still has other children
		var siblingCount int64
		if err := r.DB.WithContext(ctx).Model(&models.Account{}).Where("parent_id = ? AND id != ?", *account.ParentID, account.ID).Count(&siblingCount).Error; err != nil {
			// Log error but don't fail the delete
			fmt.Printf("[WARNING] Failed to count siblings for parent cleanup: %v\n", err)
		} else if siblingCount == 0 {
			// Old parent has no more children, unset header status
			if err := r.DB.WithContext(ctx).Model(&models.Account{}).Where("id = ?", *account.ParentID).Update("is_header", false).Error; err != nil {
				// Log error but don't fail the delete
				fmt.Printf("[WARNING] Failed to update parent header status: %v\n", err)
			}
		}
	}

	return nil
}

// FindByCode finds account by code
func (r *AccountRepo) FindByCode(ctx context.Context, code string) (*models.Account, error) {
	var account models.Account
	if err := r.DB.WithContext(ctx).Preload("Parent").Preload("Children").Where("code = ?", code).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, utils.NewNotFoundError("Account")
		}
		return nil, utils.NewDatabaseError("find account", err)
	}
	return &account, nil
}

// FindByID finds account by ID
func (r *AccountRepo) FindByID(ctx context.Context, id uint) (*models.Account, error) {
	var account models.Account
	if err := r.DB.WithContext(ctx).Preload("Parent").Preload("Children").First(&account, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, utils.NewNotFoundError("Account")
		}
		return nil, utils.NewDatabaseError("find account", err)
	}
	return &account, nil
}

// FindAll finds all accounts, now derived from GetHierarchy for consistency
func (r *AccountRepo) FindAll(ctx context.Context) ([]models.Account, error) {
	hierarchy, err := r.GetHierarchy(ctx)
	if err != nil {
		return nil, utils.NewDatabaseError("find all accounts via hierarchy", err)
	}

	// Flatten the hierarchy into a simple list
	var flattened []models.Account
	var flatten func(accounts []models.Account)
	flatten = func(accounts []models.Account) {
		for _, acc := range accounts {
			// Create a copy without the children to avoid circular references in JSON
			accountCopy := acc
			accountCopy.Children = nil 
			flattened = append(flattened, accountCopy)
			if len(acc.Children) > 0 {
				flatten(acc.Children)
			}
		}
	}

	flatten(hierarchy)
	return flattened, nil
}

// FindByType finds accounts by type
func (r *AccountRepo) FindByType(ctx context.Context, accountType string) ([]models.Account, error) {
	var accounts []models.Account
	if err := r.DB.WithContext(ctx).Where("type = ?", accountType).Order("code").Find(&accounts).Error; err != nil {
		return nil, utils.NewDatabaseError("find accounts by type", err)
	}
	return accounts, nil
}

// FindWithFilter finds accounts based on optional filters
func (r *AccountRepo) FindWithFilter(ctx context.Context, accountType *string, category *string) ([]models.Account, error) {
	var accounts []models.Account
	query := r.DB.WithContext(ctx)

	if accountType != nil && *accountType != "" {
		query = query.Where("type = ?", *accountType)
	}
	if category != nil && *category != "" {
		query = query.Where("category = ?", *category)
	}

	if err := query.Order("code").Find(&accounts).Error; err != nil {
		return nil, utils.NewDatabaseError("find accounts with filter", err)
	}
	return accounts, nil
}

// GetHierarchy gets account hierarchy with calculated balances
func (r *AccountRepo) GetHierarchy(ctx context.Context) ([]models.Account, error) {
	var allAccounts []models.Account
	if err := r.DB.WithContext(ctx).Order("code").Find(&allAccounts).Error; err != nil {
		return nil, utils.NewDatabaseError("get all accounts for hierarchy", err)
	}

	// Build a map for quick parent lookups
	accountMap := make(map[uint]*models.Account)
	for i := range allAccounts {
		allAccounts[i].Children = []models.Account{}
		accountMap[allAccounts[i].ID] = &allAccounts[i]
	}

	// Build hierarchy recursively
	var rootAccounts []models.Account
	for i := range allAccounts {
		if allAccounts[i].ParentID == nil {
			// This is a root account, build its complete hierarchy
			rootAccount := r.buildAccountHierarchy(&allAccounts[i], accountMap)
			rootAccounts = append(rootAccounts, rootAccount)
		}
	}

	// Calculate balances starting from the roots
	for i := range rootAccounts {
		r.calculateTotalBalanceRecursive(ctx, &rootAccounts[i])
	}

	return rootAccounts, nil
}

// calculateTotalBalance recursively calculates the total balance for parent accounts.
// It sums up the balances of its children.
func (r *AccountRepo) calculateTotalBalance(ctx context.Context, account *models.Account) {
	// 🔧 DISABLED SSOT: Use stored balance from database directly
	if len(account.Children) == 0 {
		// Keep the original balance from database without SSOT override
		account.TotalBalance = account.Balance
		account.ChildCount = 0
		return
	}

	// If it's a header account, recursively call for children and sum up their balances
	var childrenTotal float64
	for i := range account.Children {
		child := &account.Children[i]
		r.calculateTotalBalance(ctx, child)
		childrenTotal += child.TotalBalance
	}

	account.TotalBalance = childrenTotal
	account.ChildCount = len(account.Children)
	
	// 🔧 DISABLED: Do not overwrite balance for header accounts
	// Keep the original balance from database instead of calculating from children
	// if account.IsHeader {
	// 	account.Balance = childrenTotal
	// }
}


// BulkImport imports multiple accounts
func (r *AccountRepo) BulkImport(ctx context.Context, accounts []models.AccountImportRequest) error {
	tx := r.DB.WithContext(ctx).Begin()
	
	// Create a map to track parent codes
	parentCodeMap := make(map[string]*uint)
	
	for _, req := range accounts {
		var parentID *uint
		
		// Find parent if parent code is specified
		if req.ParentCode != "" {
			if cachedParentID, exists := parentCodeMap[req.ParentCode]; exists {
				parentID = cachedParentID
			} else {
				var parent models.Account
				if err := tx.Where("code = ?", req.ParentCode).First(&parent).Error; err != nil {
					tx.Rollback()
					return utils.NewDatabaseError("find parent account for import", err)
				}
				parentCodeMap[req.ParentCode] = &parent.ID
				parentID = &parent.ID
			}
		}
		
		// Calculate level
		level := 1
		if parentID != nil {
			var parent models.Account
			if err := tx.First(&parent, *parentID).Error; err != nil {
				tx.Rollback()
				return utils.NewDatabaseError("find parent for level calculation", err)
			}
			level = parent.Level + 1
		}
		
		account := models.Account{
			Code:        req.Code,
			Name:        req.Name,
			Type:        string(req.Type),
			Category:    req.Category,
			ParentID:    parentID,
			Level:       level,
			Description: req.Description,
			Balance:     req.OpeningBalance,
			IsActive:    true,
			IsHeader:    false,
		}
		
		if err := tx.Create(&account).Error; err != nil {
			tx.Rollback()
			return utils.NewDatabaseError("create account in bulk import", err)
		}
	}
	
	return tx.Commit().Error
}

// CalculateBalance calculates account balance including opening balance
func (r *AccountRepo) CalculateBalance(ctx context.Context, accountID uint) (float64, error) {
	// Get account to determine type and opening balance
	var account models.Account
	if err := r.DB.WithContext(ctx).First(&account, accountID).Error; err != nil {
		return 0, utils.NewDatabaseError("find account for balance calculation", err)
	}

	// Get transaction totals
	var result struct {
		DebitSum  float64
		CreditSum float64
	}

	if err := r.DB.WithContext(ctx).Model(&models.Transaction{}).
		Select("COALESCE(SUM(debit_amount), 0) as debit_sum, COALESCE(SUM(credit_amount), 0) as credit_sum").
		Where("account_id = ?", accountID).
		Scan(&result).Error; err != nil {
		return 0, utils.NewDatabaseError("calculate balance", err)
	}

	// Calculate transaction balance based on account type
	var transactionBalance float64
	if account.Type == models.AccountTypeAsset || account.Type == models.AccountTypeExpense {
		// Debit balance accounts: Assets and Expenses
		transactionBalance = result.DebitSum - result.CreditSum
	} else {
		// Credit balance accounts: Liabilities, Equity, Revenue
		transactionBalance = result.CreditSum - result.DebitSum
	}

	// For accounts with normal debit balance (Assets, Expenses):
	// Final Balance = Opening Balance + Transaction Balance
	// For accounts with normal credit balance (Liabilities, Equity, Revenue):
	// Final Balance = Opening Balance + Transaction Balance
return account.Balance + transactionBalance, nil
}

// CalculateBalanceSSOT calculates account balance using SSOT (unified_journal_ledger/lines)
// Rules:
// - Only POSTED journals are included
// - For SALES source_type, include only sales with status INVOICED or PAID
// - Non-sales source types are included without extra filter
// - Netting uses account type: ASSET/EXPENSE => debit - credit, others => credit - debit
// - Fallback to accounts.balance when no SSOT data found
func (r *AccountRepo) CalculateBalanceSSOT(ctx context.Context, accountID uint) (float64, error) {
	// Load account to know its type and current stored balance (fallback)
	var account models.Account
	if err := r.DB.WithContext(ctx).First(&account, accountID).Error; err != nil {
		return 0, utils.NewDatabaseError("find account for SSOT calculation", err)
	}

	type sums struct {
		DebitSum  float64
		CreditSum float64
	}
	var result sums

	query := `
		SELECT
			COALESCE(SUM(ujl.debit_amount), 0)  AS debit_sum,
			COALESCE(SUM(ujl.credit_amount), 0) AS credit_sum
		FROM unified_journal_lines ujl
		JOIN unified_journal_ledger ujd ON ujd.id = ujl.journal_id
		LEFT JOIN sales s ON ujd.source_type = 'SALE' AND ujd.source_id = s.id
		WHERE ujl.account_id = ?
		  AND ujd.status = 'POSTED'
		  AND ujd.deleted_at IS NULL
		  AND (
				(ujd.source_type = 'SALE' AND s.status IN ('INVOICED','PAID'))
				OR (ujd.source_type <> 'SALE')
			)
	`

	if err := r.DB.WithContext(ctx).Raw(query, accountID).Scan(&result).Error; err != nil {
		return 0, utils.NewDatabaseError("calculate SSOT sums", err)
	}

	// If unified_journal has no movement, try simple_ssot_journals as fallback
	if result.DebitSum == 0 && result.CreditSum == 0 {
		var simple sums
		simpleQry := `
			SELECT 
				COALESCE(SUM(ssi.debit), 0)  AS debit_sum,
				COALESCE(SUM(ssi.credit), 0) AS credit_sum
			FROM simple_ssot_journal_items ssi
			JOIN simple_ssot_journals ssj ON ssj.id = ssi.journal_id
			WHERE ssi.account_id = ?
			  AND ssj.status = 'POSTED'
			  AND ssj.deleted_at IS NULL
		`
		if err := r.DB.WithContext(ctx).Raw(simpleQry, accountID).Scan(&simple).Error; err != nil {
			return 0, utils.NewDatabaseError("calculate simple SSOT sums", err)
		}
		result = simple
	}

	// If still zero (no SSOT data at all), fallback to stored balance
	if result.DebitSum == 0 && result.CreditSum == 0 {
		return account.Balance, nil
	}

	var net float64
	if account.Type == models.AccountTypeAsset || account.Type == models.AccountTypeExpense {
		net = result.DebitSum - result.CreditSum
	} else {
		net = result.CreditSum - result.DebitSum
	}
	return net, nil
}

// UpdateBalance updates the balance of an account based on a transaction
func (r *AccountRepo) UpdateBalance(ctx context.Context, accountID uint, debitAmount, creditAmount float64) error {
	var account models.Account
	if err := r.DB.WithContext(ctx).First(&account, accountID).Error; err != nil {
		return utils.NewDatabaseError("find account for balance update", err)
	}

	// Get the normal balance type for the account
	normalBalance := account.GetNormalBalance()
	
	var balanceChange float64
	if normalBalance == models.NormalBalanceDebit {
		balanceChange = debitAmount - creditAmount
	} else {
		balanceChange = creditAmount - debitAmount
	}

	// Atomically update the balance using optimized raw SQL
	err := r.DB.WithContext(ctx).Exec(
		"UPDATE accounts SET balance = balance + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL", 
		balanceChange, accountID).Error
		
	if err != nil {
		return utils.NewDatabaseError("update account balance", err)
	}

	// After updating, we might need to update parent balances.
	// This can be complex, so for now, we leave it to be recalculated on the next hierarchy fetch.
	return nil
}

// GetBalanceSummary gets balance summary by account type
func (r *AccountRepo) GetBalanceSummary(ctx context.Context) ([]models.AccountSummaryResponse, error) {
	var summaries []models.AccountSummaryResponse

	accountTypes := []string{
		models.AccountTypeAsset,
		models.AccountTypeLiability,
		models.AccountTypeEquity,
		models.AccountTypeRevenue,
		models.AccountTypeExpense,
	}

	for _, accountType := range accountTypes {
		var result struct {
			TotalAccounts  int64
			ActiveAccounts int64
			TotalBalance   float64
		}

		if err := r.DB.WithContext(ctx).Model(&models.Account{}).
			Select("COUNT(*) as total_accounts, SUM(CASE WHEN is_active THEN 1 ELSE 0 END) as active_accounts, COALESCE(SUM(balance), 0) as total_balance").
			Where("type = ?", accountType).
			Scan(&result).Error; err != nil {
			return nil, utils.NewDatabaseError("get balance summary", err)
		}

		summaries = append(summaries, models.AccountSummaryResponse{
			Type:           models.AccountType(accountType),
			TotalAccounts:  result.TotalAccounts,
			ActiveAccounts: result.ActiveAccounts,
			TotalBalance:   result.TotalBalance,
		})
	}

	return summaries, nil
}

// buildAccountHierarchy recursively builds the account hierarchy starting from a root account
func (r *AccountRepo) buildAccountHierarchy(account *models.Account, accountMap map[uint]*models.Account) models.Account {
	result := *account // Create a copy
	result.Children = []models.Account{}

	// Find and add children recursively
	for _, acc := range accountMap {
		if acc.ParentID != nil && *acc.ParentID == account.ID {
			child := r.buildAccountHierarchy(acc, accountMap)
			result.Children = append(result.Children, child)
		}
	}

	// Update child count
	result.ChildCount = len(result.Children)

	return result
}

// GetAccountByCode is a convenience method that wraps FindByCode without requiring context
func (r *AccountRepo) GetAccountByCode(code string) (*models.Account, error) {
	return r.FindByCode(context.Background(), code)
}

// FixAccountHeaderStatus fixes is_header status for all accounts based on whether they have children
func (r *AccountRepo) FixAccountHeaderStatus(ctx context.Context) error {
	// Get all accounts that have children
	var parentAccounts []struct {
		ID       uint
		IsHeader bool
	}
	
	if err := r.DB.WithContext(ctx).Raw(`
		SELECT a.id, a.is_header 
		FROM accounts a 
		WHERE EXISTS (
			SELECT 1 FROM accounts child 
			WHERE child.parent_id = a.id AND child.deleted_at IS NULL
		) AND a.deleted_at IS NULL
	`).Scan(&parentAccounts).Error; err != nil {
		return utils.NewDatabaseError("find parent accounts", err)
	}
	
	// Update accounts that should be headers but aren't
	for _, parent := range parentAccounts {
		if !parent.IsHeader {
			if err := r.DB.WithContext(ctx).Model(&models.Account{}).Where("id = ?", parent.ID).Update("is_header", true).Error; err != nil {
				return utils.NewDatabaseError("update header status", err)
			}
		}
	}
	
	// Get all accounts that don't have children but are marked as headers
	var nonParentHeaders []uint
	if err := r.DB.WithContext(ctx).Raw(`
		SELECT a.id 
		FROM accounts a 
		WHERE a.is_header = true 
			AND NOT EXISTS (
				SELECT 1 FROM accounts child 
				WHERE child.parent_id = a.id AND child.deleted_at IS NULL
			) 
			AND a.deleted_at IS NULL
	`).Pluck("id", &nonParentHeaders).Error; err != nil {
		return utils.NewDatabaseError("find non-parent headers", err)
	}
	
	// Update accounts that shouldn't be headers
	for _, accountID := range nonParentHeaders {
		if err := r.DB.WithContext(ctx).Model(&models.Account{}).Where("id = ?", accountID).Update("is_header", false).Error; err != nil {
			return utils.NewDatabaseError("update non-header status", err)
		}
	}
	
	return nil
}

// calculateTotalBalanceRecursive recursively calculates balances for the entire hierarchy
func (r *AccountRepo) calculateTotalBalanceRecursive(ctx context.Context, account *models.Account) {
	// 🔧 DISABLED SSOT: Use stored balance from database directly
	if len(account.Children) == 0 {
		// Keep the original balance from database without SSOT override
		account.TotalBalance = account.Balance
		return
	}

	// If it's a parent account, recursively calculate children's balances first
	var childrenTotal float64
	for i := range account.Children {
		r.calculateTotalBalanceRecursive(ctx, &account.Children[i])
		childrenTotal += account.Children[i].TotalBalance
	}

	account.TotalBalance = childrenTotal

	// 🔧 DISABLED: Do not overwrite balance for header accounts
	// Keep the original balance from database instead of calculating from children
	// if account.IsHeader {
	// 	account.Balance = childrenTotal
	// }
}

// checkCircularReference checks if setting parentID would create a circular reference
func (r *AccountRepo) checkCircularReference(ctx context.Context, accountID uint, newParentID uint) error {
	// An account cannot be its own parent
	if accountID == newParentID {
		return utils.NewBadRequestError("An account cannot be its own parent")
	}
	
	// Check if newParentID is a descendant of accountID
	currentParentID := newParentID
	for currentParentID != 0 {
		var parent models.Account
		if err := r.DB.WithContext(ctx).Select("id, parent_id").First(&parent, currentParentID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				break // Parent doesn't exist, no circular reference
			}
			return utils.NewDatabaseError("check circular reference", err)
		}
		
		if parent.ID == accountID {
			return utils.NewBadRequestError("Cannot create circular reference: new parent is a descendant of this account")
		}
		
		if parent.ParentID == nil {
			break // Reached root, no circular reference
		}
		currentParentID = *parent.ParentID
	}
	
	return nil
}

// updateChildrenLevels recursively updates the level of all children when parent level changes
func (r *AccountRepo) updateChildrenLevels(ctx context.Context, accountID uint, parentLevel int) error {
	// Get all direct children
	var children []models.Account
	if err := r.DB.WithContext(ctx).Where("parent_id = ?", accountID).Find(&children).Error; err != nil {
		return err
	}
	
	// Update each child's level and recursively update their children
	for _, child := range children {
		newLevel := parentLevel + 1
		if err := r.DB.WithContext(ctx).Model(&child).Update("level", newLevel).Error; err != nil {
			return err
		}
		
		// Recursively update grandchildren
		if err := r.updateChildrenLevels(ctx, child.ID, newLevel); err != nil {
			return err
		}
	}
	
	return nil
}
