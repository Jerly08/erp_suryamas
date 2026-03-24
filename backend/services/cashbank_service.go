package services

import (
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type CashBankService struct {
	db                    *gorm.DB
	cashBankRepo          *repositories.CashBankRepository
	accountRepo           repositories.AccountRepository
	unifiedJournalService *UnifiedJournalService      // New SSOT dependency
	sSOTJournalAdapter    *CashBankSSOTJournalAdapter // SSOT Integration
}

func NewCashBankService(
	db *gorm.DB,
	cashBankRepo *repositories.CashBankRepository,
	accountRepo repositories.AccountRepository,
) *CashBankService {
	// Initialize UnifiedJournalService
	unifiedJournalService := NewUnifiedJournalService(db)

	// Initialize SSOT Journal Adapter
	sSOTJournalAdapter := NewCashBankSSOTJournalAdapter(db)

	return &CashBankService{
		db:                    db,
		cashBankRepo:          cashBankRepo,
		accountRepo:           accountRepo,
		unifiedJournalService: unifiedJournalService,
		sSOTJournalAdapter:    sSOTJournalAdapter,
	}
}

// Transaction Types
const (
	TransactionTypeDeposit        = "DEPOSIT"
	TransactionTypeWithdrawal     = "WITHDRAWAL"
	TransactionTypeTransfer       = "TRANSFER"
	TransactionTypeAdjustment     = "ADJUSTMENT"
	TransactionTypeOpeningBalance = "OPENING_BALANCE"
)

// GetCashBankAccounts retrieves all cash and bank accounts
func (s *CashBankService) GetCashBankAccounts(includeHolding ...bool) ([]models.CashBank, error) {
	include := false
	if len(includeHolding) > 0 {
		include = includeHolding[0]
	}
	accounts, err := s.cashBankRepo.FindAll(include)
	if err != nil {
		return nil, err
	}

	for i := range accounts {
		if repairErr := s.repairForeignCurrencyBalanceState(&accounts[i]); repairErr != nil {
			log.Printf("⚠️ [CASHBANK SERVICE] Failed to repair foreign currency balance state for cash bank %d: %v", accounts[i].ID, repairErr)
		}
	}

	return accounts, nil
}

// GetCashBankByID retrieves cash/bank account by ID
func (s *CashBankService) GetCashBankByID(id uint) (*models.CashBank, error) {
	account, err := s.cashBankRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if repairErr := s.repairForeignCurrencyBalanceState(account); repairErr != nil {
		log.Printf("⚠️ [CASHBANK SERVICE] Failed to repair foreign currency balance state for cash bank %d: %v", account.ID, repairErr)
	}

	return account, nil
}

// CreateCashBankAccount creates a new cash or bank account
func (s *CashBankService) CreateCashBankAccount(request CashBankCreateRequest, userID uint) (result *models.CashBank, err error) {
	// Log method entry with parameters
	log.Printf("Ã°Å¸â€â€ž [CASHBANK SERVICE] Creating cash bank account: Name=%s, Type=%s, AccountID=%d, OpeningBalance=%.2f, UserID=%d",
		request.Name, request.Type, request.AccountID, request.OpeningBalance, userID)

	currency := normalizeCashBankCurrency(request.Currency)
	allowDirectTransactions := true
	if request.AllowDirectTransactions != nil {
		allowDirectTransactions = *request.AllowDirectTransactions
	}
	if request.IsHoldingAccount && request.AllowDirectTransactions == nil {
		allowDirectTransactions = false
	}
	initialExchangeRate := request.OpeningExchangeRate
	if currency == "IDR" {
		initialExchangeRate = 1
	} else if initialExchangeRate <= 0 {
		initialExchangeRate = 1
	}

	// Start transaction
	tx := s.db.Begin()

	// Panic recovery with transaction rollback
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			stackTrace := string(debug.Stack())
			log.Printf("Ã¢ÂÅ’ [CASHBANK SERVICE] PANIC in CreateCashBankAccount: %v", r)
			log.Printf("Ã¢ÂÅ’ [CASHBANK SERVICE] Stack trace:\n%s", stackTrace)
			err = fmt.Errorf("internal error during cash bank account creation: %v", r)
			result = nil
		}
	}()

	// Validate GL account if provided
	log.Printf("Ã°Å¸â€Â [CASHBANK SERVICE] Step 1: Validating GL account (AccountID=%d)", request.AccountID)
	var glAccount *models.Account
	if request.AccountID > 0 {
		account, findErr := s.accountRepo.FindByID(context.Background(), request.AccountID)
		if findErr != nil {
			tx.Rollback()
			log.Printf("Ã¢ÂÅ’ [CASHBANK SERVICE] GL account validation failed for ID %d: %v", request.AccountID, findErr)
			return nil, fmt.Errorf("GL account validation failed for ID %d: GL account not found", request.AccountID)
		}

		// Ã¢Å“â€¦ FIX ROOT CAUSE: Prevent multiple cash/bank accounts sharing same GL
		// Check if this GL is already linked to another cash/bank account
		log.Printf("Ã°Å¸â€Â [CASHBANK SERVICE] Checking if GL account %d is already linked", request.AccountID)
		var existingCashBank models.CashBank
		findErr = tx.Where("account_id = ? AND deleted_at IS NULL", request.AccountID).Limit(1).Find(&existingCashBank).Error
		if findErr != nil {
			// Unexpected database error
			tx.Rollback()
			log.Printf("Ã¢ÂÅ’ [CASHBANK SERVICE] Failed to check GL account linkage: %v", findErr)
			return nil, fmt.Errorf("failed to check GL account linkage: %v", findErr)
		}
		if existingCashBank.ID != 0 {
			// GL already linked to another cash/bank account
			tx.Rollback()
			log.Printf("Ã¢ÂÅ’ [CASHBANK SERVICE] GL account %s is already linked to cash/bank account %s", account.Code, existingCashBank.Name)
			return nil, fmt.Errorf("GL account '%s - %s' is already linked to cash/bank account '%s'. Each cash/bank account must have its own unique GL account for proper balance tracking",
				account.Code, account.Name, existingCashBank.Name)
		}
		// GL is available, proceed
		glAccount = account
		log.Printf("Ã¢Å“â€¦ [CASHBANK SERVICE] GL account validated: %s - %s (verified unique)", account.Code, account.Name)
	} else {
		// Ã¢Å“â€¦ FIX ROOT CAUSE: Always create UNIQUE GL account for each cash/bank
		// Never reuse existing GL to prevent balance confusion
		log.Printf("Ã°Å¸â€Â¨ [CASHBANK SERVICE] Step 2: Creating new GL account for cash/bank: %s", request.Name)
		ctx := context.Background()

		// Generate UNIQUE account code with timestamp to ensure uniqueness
		baseCode := s.generateAccountCode(request.Type)
		accountCode := s.generateUniqueAccountCode(baseCode, request.Name)

		log.Printf("Ã°Å¸â€œâ€¹ [CASHBANK SERVICE] Generated GL account code: %s", accountCode)
		// Create unique GL account
		isHeader := false
		accountRequest := &models.AccountCreateRequest{
			Code:           accountCode,
			Name:           request.Name,
			Type:           models.AccountTypeAsset,
			Category:       s.getAccountCategory(request.Type),
			Description:    fmt.Sprintf("GL for %s: %s", request.Type, request.Name),
			IsHeader:       &isHeader,
			OpeningBalance: 0,
		}

		createdAccount, createGLErr := s.accountRepo.Create(ctx, accountRequest)
		if createGLErr != nil {
			// If duplicate, retry with even more unique code
			if strings.Contains(createGLErr.Error(), "already exists") || strings.Contains(createGLErr.Error(), "duplicate") {
				log.Printf("Ã¢Å¡Â Ã¯Â¸Â [CASHBANK SERVICE] GL account code collision, retrying with unique code")
				// Retry with nano timestamp
				accountCode = fmt.Sprintf("%s-%d", baseCode, time.Now().UnixNano()%100000)
				accountRequest.Code = accountCode
				createdAccount, createGLErr = s.accountRepo.Create(ctx, accountRequest)
				if createGLErr != nil {
					tx.Rollback()
					log.Printf("Ã¢ÂÅ’ [CASHBANK SERVICE] GL account creation failed after retry: %v", createGLErr)
					return nil, fmt.Errorf("GL account creation failed: failed to create unique GL account after retry: %v", createGLErr)
				}
			} else {
				tx.Rollback()
				log.Printf("Ã¢ÂÅ’ [CASHBANK SERVICE] GL account creation failed: %v", createGLErr)
				return nil, fmt.Errorf("GL account creation failed: %v", createGLErr)
			}
		}
		glAccount = createdAccount
		log.Printf("Ã¢Å“â€¦ [CASHBANK SERVICE] GL account created: %s - %s", glAccount.Code, glAccount.Name)
	}

	// Generate code and create account with retry to avoid unique constraint conflicts
	log.Printf("Ã°Å¸â€Â¨ [CASHBANK SERVICE] Step 3: Creating cash bank record")
	var cashBank *models.CashBank
	var createErr error
	for retry := 0; retry < 10; retry++ {
		// Generate fresh code on each retry to handle concurrent requests
		code := s.generateCashBankCode(request.Type)

		// Add random suffix for better collision avoidance on retries
		if retry > 0 {
			randomSuffix := time.Now().UnixNano() % 1000
			code = fmt.Sprintf("%s-R%d-%03d", code, retry, randomSuffix)
		}

		log.Printf("Ã°Å¸â€œâ€¹ [CASHBANK SERVICE] Attempting to create cash bank with code: %s (attempt %d)", code, retry+1)

		cashBank = &models.CashBank{
			Code:                    code,
			Name:                    request.Name,
			Type:                    request.Type,
			AccountID:               glAccount.ID,
			BankName:                request.BankName,
			AccountNo:               request.AccountNo,
			AccountHolderName:       request.AccountHolderName,
			Branch:                  request.Branch,
			Currency:                currency,
			Balance:                 0, // Will be set via opening balance transaction
			BookBalanceIDR:          0,
			LastExchangeRate:        initialExchangeRate,
			IsActive:                true,
			IsHoldingAccount:        request.IsHoldingAccount,
			AllowDirectTransactions: allowDirectTransactions,
			Description:             request.Description,
		}

		// Create a savepoint before attempting to insert
		savePointName := fmt.Sprintf("sp_create_cashbank_%d", retry)
		if err := tx.SavePoint(savePointName).Error; err != nil {
			log.Printf("Ã¢ÂÅ’ [CASHBANK SERVICE] Failed to create savepoint: %v", err)
			tx.Rollback()
			return nil, fmt.Errorf("failed to create savepoint: %v", err)
		}

		createErr = tx.Create(cashBank).Error
		if createErr == nil {
			log.Printf("Ã¢Å“â€¦ [CASHBANK SERVICE] Cash bank record created: ID=%d, Code=%s", cashBank.ID, code)
			break
		}

		// Check if it's a duplicate code error
		errMsg := createErr.Error()
		if strings.Contains(errMsg, "uni_cash_banks_code") || strings.Contains(errMsg, "duplicate key") || strings.Contains(errMsg, "23505") {
			log.Printf("Ã¢Å¡Â Ã¯Â¸Â [CASHBANK SERVICE] Duplicate code detected: %s (attempt %d), rolling back to savepoint and retrying...", code, retry+1)

			// Rollback to savepoint to clear the error state
			if rbErr := tx.RollbackTo(savePointName).Error; rbErr != nil {
				log.Printf("Ã¢ÂÅ’ [CASHBANK SERVICE] Failed to rollback to savepoint: %v", rbErr)
				tx.Rollback() // Fatal error, rollback entire transaction
				return nil, fmt.Errorf("failed to rollback to savepoint: %v", rbErr)
			}

			// Add small delay to reduce collision probability
			time.Sleep(time.Millisecond * time.Duration(10*(retry+1)))
			continue
		}

		// Other errors: abort immediately
		log.Printf("Ã¢ÂÅ’ [CASHBANK SERVICE] Cash bank creation failed: %v", createErr)
		tx.Rollback()
		return nil, fmt.Errorf("cash bank record creation failed: %v", createErr)
	}

	if createErr != nil {
		tx.Rollback()
		log.Printf("Ã¢ÂÅ’ [CASHBANK SERVICE] Failed to generate unique cash-bank code after 10 retries")
		return nil, fmt.Errorf("cash bank record creation failed: failed to generate unique code after 10 retries: %w", createErr)
	}

	// Create opening balance transaction if provided
	if request.OpeningBalance > 0 {
		log.Printf("Ã°Å¸â€™Â° [CASHBANK SERVICE] Step 4: Creating opening balance transaction: Amount=%.2f", request.OpeningBalance)
		openingDate := request.OpeningDate.ToTime()
		if openingDate.IsZero() {
			openingDate = time.Now() // Use current time if no date provided
		}
		openingErr := s.createOpeningBalanceTransaction(tx, cashBank, request.OpeningBalance, openingDate, userID, request.SourceAccountID)
		if openingErr != nil {
			tx.Rollback()
			log.Printf("Ã¢ÂÅ’ [CASHBANK SERVICE] Opening balance creation failed: %v", openingErr)
			return nil, fmt.Errorf("opening balance creation failed: %v", openingErr)
		}
		log.Printf("Ã¢Å“â€¦ [CASHBANK SERVICE] Opening balance transaction created successfully")
	} else {
		log.Printf("Ã¢â€žÂ¹Ã¯Â¸Â [CASHBANK SERVICE] No opening balance specified, skipping opening balance transaction")
	}

	// Commit transaction
	log.Printf("Ã°Å¸â€™Â¾ [CASHBANK SERVICE] Step 5: Committing transaction")
	if commitErr := tx.Commit().Error; commitErr != nil {
		log.Printf("Ã¢ÂÅ’ [CASHBANK SERVICE] Transaction commit failed: %v", commitErr)
		return nil, fmt.Errorf("transaction commit failed: %v", commitErr)
	}

	log.Printf("Ã°Å¸Å½â€° [CASHBANK SERVICE] Cash bank account created successfully: ID=%d, Code=%s, Name=%s, Balance=%.2f",
		cashBank.ID, cashBank.Code, cashBank.Name, cashBank.Balance)

	return cashBank, nil
}

// UpdateCashBankAccount updates cash/bank account details
func (s *CashBankService) UpdateCashBankAccount(id uint, request CashBankUpdateRequest) (*models.CashBank, error) {
	// DEBUG LOG
	log.Printf("[CASHBANK SERVICE UPDATE] ID=%d, Request received: Name='%s', BankName='%s', AccountNo='%s', AccountHolderName='%s', Branch='%s', Description='%s'",
		id, request.Name, request.BankName, request.AccountNo, request.AccountHolderName, request.Branch, request.Description)

	cashBank, err := s.cashBankRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	log.Printf("[CASHBANK SERVICE UPDATE] BEFORE: ID=%d, AccountHolderName='%s', Branch='%s'",
		id, cashBank.AccountHolderName, cashBank.Branch)

	// Ensure account integrity using the database function
	if err := database.EnsureCashBankAccountIntegrity(s.db, cashBank.ID); err != nil {
		log.Printf("Warning: Failed to ensure cash bank account integrity for ID %d: %v", cashBank.ID, err)
		// Continue with update, but log the issue
	}

	// Reload cash bank to get updated account_id after integrity check
	cashBank, err = s.cashBankRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	// Update fields - always update to allow clearing values
	// Only update Name if not empty to prevent accidental clearing of required field
	if request.Name != "" {
		cashBank.Name = request.Name
	}

	// Allow empty values for optional fields (to support clearing)
	cashBank.BankName = request.BankName
	cashBank.AccountNo = request.AccountNo
	cashBank.AccountHolderName = request.AccountHolderName
	cashBank.Branch = request.Branch
	cashBank.Description = request.Description

	log.Printf("[CASHBANK SERVICE UPDATE] AFTER ASSIGNMENT: ID=%d, AccountHolderName='%s', Branch='%s'",
		id, cashBank.AccountHolderName, cashBank.Branch)

	if request.IsActive != nil {
		cashBank.IsActive = *request.IsActive
	}

	if request.IsHoldingAccount != nil {
		cashBank.IsHoldingAccount = *request.IsHoldingAccount
		if request.AllowDirectTransactions == nil && *request.IsHoldingAccount {
			cashBank.AllowDirectTransactions = false
		}
	}

	if request.AllowDirectTransactions != nil {
		cashBank.AllowDirectTransactions = *request.AllowDirectTransactions
	}

	updatedAccount, err := s.cashBankRepo.Update(cashBank)
	if err != nil {
		log.Printf("[CASHBANK SERVICE UPDATE] Ã¢ÂÅ’ Failed to update: %v", err)
		return nil, err
	}

	log.Printf("[CASHBANK SERVICE UPDATE] Ã¢Å“â€¦ SUCCESS: ID=%d, Final AccountHolderName='%s', Branch='%s'",
		id, updatedAccount.AccountHolderName, updatedAccount.Branch)

	return updatedAccount, nil
}

// DeleteCashBankAccount deletes (soft delete) cash/bank account
func (s *CashBankService) DeleteCashBankAccount(id uint) error {
	// Check if account exists
	cashBank, err := s.cashBankRepo.FindByID(id)
	if err != nil {
		return errors.New("account not found")
	}

	// Check if account has balance
	if cashBank.Balance != 0 {
		return fmt.Errorf("cannot delete account with non-zero balance: %.2f", cashBank.Balance)
	}

	// Check if account has transactions
	// In a real implementation, you might want to check for recent transactions
	// For now, we'll allow deletion if balance is zero

	return s.cashBankRepo.Delete(id)
}

func normalizeCashBankCurrency(currency string) string {
	normalized := strings.ToUpper(strings.TrimSpace(currency))
	if normalized == "" {
		return "IDR"
	}
	return normalized
}

func supportsUsdIdrTransfer(sourceCurrency, destinationCurrency string) bool {
	if sourceCurrency == destinationCurrency {
		return true
	}

	return (sourceCurrency == "USD" && destinationCurrency == "IDR") ||
		(sourceCurrency == "IDR" && destinationCurrency == "USD")
}

func (s *CashBankService) resolveAppliedExchangeRate(account *models.CashBank, providedRate float64) (float64, error) {
	if !account.IsForeignCurrency() {
		return 1, nil
	}
	if providedRate > 0 {
		return providedRate, nil
	}
	if account.LastExchangeRate > 0 {
		return account.LastExchangeRate, nil
	}
	return 0, fmt.Errorf("exchange rate required for %s account", account.Currency)
}

func (s *CashBankService) calculateIncreaseBookAmount(account *models.CashBank, amount, providedRate float64) (float64, float64, error) {
	if !account.IsForeignCurrency() {
		return amount, 1, nil
	}
	rate, err := s.resolveAppliedExchangeRate(account, providedRate)
	if err != nil {
		return 0, 0, err
	}
	return amount * rate, rate, nil
}

func (s *CashBankService) calculateReductionBookAmount(account *models.CashBank, amount float64) (float64, float64, error) {
	if !account.IsForeignCurrency() {
		return amount, 1, nil
	}
	if amount <= 0 {
		return 0, 0, fmt.Errorf("amount must be greater than zero")
	}
	if account.Balance <= 0 {
		return 0, 0, fmt.Errorf("source foreign balance is not available")
	}
	bookAmount := account.BookBalanceIDR * (amount / account.Balance)
	rate := 1.0
	if amount > 0 {
		rate = bookAmount / amount
	}
	return bookAmount, rate, nil
}

func (s *CashBankService) syncCashBankBookBalanceToGL(tx *gorm.DB, account *models.CashBank) error {
	if account.AccountID == 0 {
		return nil
	}
	targetBalance := account.EffectiveBookBalanceIDR()
	return tx.Exec("UPDATE accounts SET balance = ? , updated_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL", targetBalance, account.AccountID).Error
}

func (s *CashBankService) repairForeignCurrencyBalanceState(account *models.CashBank) error {
	if account == nil || !account.IsForeignCurrency() || account.ID == 0 {
		return nil
	}
	guard := NewCashBankCurrencyGuard(s.db)
	beforeNative := account.Balance
	beforeBook := account.BookBalanceIDR
	if err := guard.EnforceAfterTransaction(nil, account.ID); err != nil {
		return err
	}
	reloaded, err := s.cashBankRepo.FindByID(account.ID)
	if err != nil {
		return fmt.Errorf("failed to reload repaired cash bank: %w", err)
	}

	if math.Abs(beforeNative-reloaded.Balance) > 0.0001 || math.Abs(beforeBook-reloaded.BookBalanceIDR) > 0.01 {
		log.Printf("🔧 [CASHBANK SERVICE] Repaired foreign currency cash bank %d: native %.2f -> %.2f, book %.2f -> %.2f",
			account.ID, beforeNative, reloaded.Balance, beforeBook, reloaded.BookBalanceIDR)
	}

	*account = *reloaded
	return nil
}

func (s *CashBankService) resolveForexAccountID(tx *gorm.DB, isGain bool) (uint64, error) {
	dbToUse := tx
	if dbToUse == nil {
		dbToUse = s.db
	}

	preferredCodes := []string{"8103", "8102"}
	fallbackType := models.AccountTypeExpense
	fallbackCategories := []string{models.CategoryOtherExpense, models.CategoryNonOperatingExpense}
	if isGain {
		preferredCodes = []string{"7103", "7102"}
		fallbackType = models.AccountTypeRevenue
		fallbackCategories = []string{models.CategoryOtherIncome}
	}

	for _, code := range preferredCodes {
		var account models.Account
		if err := dbToUse.
			Where("code = ? AND deleted_at IS NULL AND is_active = ?", code, true).
			First(&account).Error; err == nil {
			return uint64(account.ID), nil
		}
	}

	var fallback models.Account
	if err := dbToUse.
		Where("type = ? AND category IN ? AND is_header = ? AND deleted_at IS NULL AND is_active = ?",
			fallbackType, fallbackCategories, false, true).
		Order("code ASC").
		First(&fallback).Error; err == nil {
		return uint64(fallback.ID), nil
	}

	accountKind := "loss"
	expectedCodes := "8103/8102"
	if isGain {
		accountKind = "gain"
		expectedCodes = "7103/7102"
	}

	return 0, fmt.Errorf("failed to resolve forex %s account from COA. Expected account code %s or matching other income/expense leaf account", accountKind, expectedCodes)
}

func (s *CashBankService) validateCounterAccountWithTx(tx *gorm.DB, counterAccountID uint, cashBankAccountID uint, allowedTypes ...string) (*models.Account, error) {
	var account models.Account
	if err := tx.Where("id = ? AND deleted_at IS NULL", counterAccountID).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("counter account %d not found", counterAccountID)
		}
		return nil, fmt.Errorf("failed to load counter account %d: %w", counterAccountID, err)
	}

	if !account.IsActive {
		return nil, fmt.Errorf("counter account %s is inactive", account.Code)
	}
	if account.IsHeader {
		return nil, fmt.Errorf("counter account %s is a header account and cannot be posted", account.Code)
	}
	if cashBankAccountID > 0 && account.ID == cashBankAccountID {
		return nil, fmt.Errorf("counter account %s must be different from the cash/bank GL account", account.Code)
	}
	var linkedCashBank models.CashBank
	if err := tx.Where("account_id = ? AND deleted_at IS NULL", counterAccountID).First(&linkedCashBank).Error; err == nil {
		return nil, fmt.Errorf("counter account %s is linked to cash/bank account %s. Use transfer instead of direct deposit/withdrawal", account.Code, linkedCashBank.Name)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to validate cash/bank link for counter account %s: %w", account.Code, err)
	}

	if len(allowedTypes) > 0 {
		allowed := make(map[string]bool, len(allowedTypes))
		for _, accountType := range allowedTypes {
			allowed[strings.ToUpper(accountType)] = true
		}
		if !allowed[strings.ToUpper(account.Type)] {
			return nil, fmt.Errorf("counter account %s has invalid type %s", account.Code, account.Type)
		}
	}

	return &account, nil
}

func (s *CashBankService) createTransferSSOTJournalEntryWithTx(
	tx *gorm.DB,
	transfer *CashBankTransfer,
	sourceAccount, destAccount *models.CashBank,
	sourceBookAmountIDR, destinationBookAmountIDR, forexGainLossAmount float64,
	userID uint,
	request TransferRequest,
) error {
	if sourceAccount.AccountID == 0 || destAccount.AccountID == 0 {
		return fmt.Errorf("both transfer accounts must be linked to GL accounts")
	}

	journalReference := strings.TrimSpace(request.Reference)
	if journalReference == "" {
		journalReference = transfer.TransferNumber
	}

	totalDebit := decimal.NewFromFloat(destinationBookAmountIDR)
	totalCredit := decimal.NewFromFloat(sourceBookAmountIDR)
	lines := []models.SSOTJournalLine{
		{
			AccountID:    uint64(destAccount.AccountID),
			LineNumber:   1,
			Description:  fmt.Sprintf("Transfer in to %s", destAccount.Name),
			DebitAmount:  decimal.NewFromFloat(destinationBookAmountIDR),
			CreditAmount: decimal.Zero,
		},
		{
			AccountID:    uint64(sourceAccount.AccountID),
			LineNumber:   2,
			Description:  fmt.Sprintf("Transfer out from %s", sourceAccount.Name),
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.NewFromFloat(sourceBookAmountIDR),
		},
	}

	lineNumber := 3
	if forexGainLossAmount > 0.005 {
		gainAccountID, err := s.resolveForexAccountID(tx, true)
		if err != nil {
			return fmt.Errorf("failed to resolve forex gain account: %v", err)
		}
		amount := decimal.NewFromFloat(forexGainLossAmount)
		totalCredit = totalCredit.Add(amount)
		lines = append(lines, models.SSOTJournalLine{
			AccountID:    gainAccountID,
			LineNumber:   lineNumber,
			Description:  "Realized foreign exchange gain",
			DebitAmount:  decimal.Zero,
			CreditAmount: amount,
		})
		lineNumber++
	} else if forexGainLossAmount < -0.005 {
		lossAccountID, err := s.resolveForexAccountID(tx, false)
		if err != nil {
			return fmt.Errorf("failed to resolve forex loss account: %v", err)
		}
		amount := decimal.NewFromFloat(-forexGainLossAmount)
		totalDebit = totalDebit.Add(amount)
		lines = append(lines, models.SSOTJournalLine{
			AccountID:    lossAccountID,
			LineNumber:   lineNumber,
			Description:  "Realized foreign exchange loss",
			DebitAmount:  amount,
			CreditAmount: decimal.Zero,
		})
		lineNumber++
	}

	sourceID := uint64(transfer.ID)
	now := time.Now()
	journal := &models.SSOTJournalEntry{
		EntryNumber:     fmt.Sprintf("TRF-%d", time.Now().UnixNano()),
		SourceType:      models.SSOTSourceTypeTransfer,
		SourceID:        &sourceID,
		SourceCode:      transfer.TransferNumber,
		EntryDate:       request.Date.ToTimeWithCurrentTime(),
		Description:     fmt.Sprintf("Cash/bank transfer %s from %s to %s", transfer.TransferNumber, sourceAccount.Name, destAccount.Name),
		Reference:       journalReference,
		Notes:           request.Notes,
		TotalDebit:      totalDebit,
		TotalCredit:     totalCredit,
		Status:          models.SSOTStatusDraft,
		IsBalanced:      true,
		IsAutoGenerated: true,
		CreatedBy:       uint64(userID),
	}

	if err := tx.Create(journal).Error; err != nil {
		return fmt.Errorf("failed to create transfer SSOT journal entry: %v", err)
	}

	for i := range lines {
		lines[i].JournalID = journal.ID
		if err := tx.Create(&lines[i]).Error; err != nil {
			return fmt.Errorf("failed to create transfer SSOT journal line: %v", err)
		}
	}

	if err := tx.Model(journal).Updates(map[string]interface{}{
		"status":    models.SSOTStatusPosted,
		"posted_at": &now,
		"posted_by": uint64(userID),
	}).Error; err != nil {
		return fmt.Errorf("failed to post transfer SSOT journal entry: %v", err)
	}

	return nil
}

// ProcessTransfer processes transfer between cash/bank accounts
func (s *CashBankService) ProcessTransfer(request TransferRequest, userID uint) (*CashBankTransfer, error) {
	tx := s.db.Begin()
	guard := NewCashBankCurrencyGuard(s.db)

	if err := CheckClosedPeriod(tx, request.Date.ToTimeWithCurrentTime()); err != nil {
		tx.Rollback()
		return nil, err
	}

	sourceAccount, err := s.cashBankRepo.FindByID(request.FromAccountID)
	if err != nil {
		tx.Rollback()
		return nil, errors.New("source account not found")
	}

	destAccount, err := s.cashBankRepo.FindByID(request.ToAccountID)
	if err != nil {
		tx.Rollback()
		return nil, errors.New("destination account not found")
	}

	if sourceAccount.ID == destAccount.ID {
		tx.Rollback()
		return nil, errors.New("source and destination accounts must be different")
	}
	if !sourceAccount.IsActive || !destAccount.IsActive {
		tx.Rollback()
		return nil, errors.New("both accounts must be active")
	}

	if err := database.EnsureCashBankAccountIntegrity(s.db, sourceAccount.ID); err != nil {
		log.Printf("Warning: Failed to ensure source account integrity for ID %d: %v", sourceAccount.ID, err)
		sourceAccount, err = s.cashBankRepo.FindByID(request.FromAccountID)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to reload source account after integrity fix: %v", err)
		}
	}

	if err := database.EnsureCashBankAccountIntegrity(s.db, destAccount.ID); err != nil {
		log.Printf("Warning: Failed to ensure destination account integrity for ID %d: %v", destAccount.ID, err)
		destAccount, err = s.cashBankRepo.FindByID(request.ToAccountID)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to reload destination account after integrity fix: %v", err)
		}
	}

	if sourceAccount.AvailableBalance() < request.Amount {
		tx.Rollback()
		if sourceAccount.FrozenAmount > 0 {
			return nil, fmt.Errorf("saldo tersedia tidak cukup. Saldo tersedia: %.2f (Total: %.2f, Dana terblokir: %.2f)", sourceAccount.AvailableBalance(), sourceAccount.Balance, sourceAccount.FrozenAmount)
		}
		return nil, fmt.Errorf("insufficient balance. Available: %.2f", sourceAccount.AvailableBalance())
	}

	sourceCurrency := normalizeCashBankCurrency(sourceAccount.Currency)
	destinationCurrency := normalizeCashBankCurrency(destAccount.Currency)
	sameCurrency := sourceCurrency == destinationCurrency
	if !supportsUsdIdrTransfer(sourceCurrency, destinationCurrency) {
		tx.Rollback()
		return nil, errors.New("different currency transfer is currently supported only between USD and IDR accounts")
	}

	transferAmount := request.Amount
	exchangeRateUsed := 1.0
	if !sameCurrency {
		if request.ExchangeRate <= 0 {
			tx.Rollback()
			return nil, errors.New("exchange rate is required for USD/IDR transfers")
		}
		exchangeRateUsed = request.ExchangeRate
		switch {
		case sourceCurrency == "USD" && destinationCurrency == "IDR":
			transferAmount = request.Amount * exchangeRateUsed
		case sourceCurrency == "IDR" && destinationCurrency == "USD":
			transferAmount = request.Amount / exchangeRateUsed
		default:
			tx.Rollback()
			return nil, errors.New("unsupported USD/IDR transfer direction")
		}
	}

	sourceBookAmountIDR, sourceCarryRate, err := s.calculateReductionBookAmount(sourceAccount, request.Amount)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	destinationBookAmountIDR := sourceBookAmountIDR
	destinationRate := 1.0
	forexGainLossAmount := 0.0
	forexGainLossType := ""
	if !sameCurrency {
		switch {
		case sourceCurrency == "USD" && destinationCurrency == "IDR":
			destinationBookAmountIDR = transferAmount
			forexGainLossAmount = destinationBookAmountIDR - sourceBookAmountIDR
			if forexGainLossAmount > 0.005 {
				forexGainLossType = "GAIN"
			} else if forexGainLossAmount < -0.005 {
				forexGainLossType = "LOSS"
			}
		case sourceCurrency == "IDR" && destinationCurrency == "USD":
			destinationBookAmountIDR = request.Amount
			destinationRate = exchangeRateUsed
		}
	} else if destAccount.IsForeignCurrency() {
		destinationRate = sourceCarryRate
	}

	var transfer *CashBankTransfer
	for retryAttempt := 0; retryAttempt < 5; retryAttempt++ {
		transfer = &CashBankTransfer{
			TransferNumber:           s.generateTransferNumber(),
			FromAccountID:            request.FromAccountID,
			ToAccountID:              request.ToAccountID,
			Date:                     request.Date.ToTimeWithCurrentTime(),
			Amount:                   request.Amount,
			ExchangeRate:             exchangeRateUsed,
			ConvertedAmount:          transferAmount,
			SourceBookAmountIDR:      sourceBookAmountIDR,
			DestinationBookAmountIDR: destinationBookAmountIDR,
			ForexGainLossAmount:      forexGainLossAmount,
			ForexGainLossType:        forexGainLossType,
			Reference:                request.Reference,
			Notes:                    request.Notes,
			Status:                   "COMPLETED",
			UserID:                   userID,
		}

		if err := tx.Create(transfer).Error; err != nil {
			if strings.Contains(err.Error(), "duplicate key value violates unique constraint") && strings.Contains(err.Error(), "transfer_number_key") {
				log.Printf("Transfer number collision detected (attempt %d), retrying...", retryAttempt+1)
				time.Sleep(time.Millisecond * time.Duration(50*(retryAttempt+1)))
				continue
			}
			if strings.Contains(err.Error(), "duplicate key value violates unique constraint") && strings.Contains(err.Error(), "cash_bank_transfers_pkey") {
				log.Printf("Cash bank transfer ID sequence mismatch detected (attempt %d), syncing sequence and retrying...", retryAttempt+1)
				if syncErr := s.syncCashBankTransferSequence(tx); syncErr != nil {
					tx.Rollback()
					return nil, fmt.Errorf("failed to sync cash bank transfer sequence: %v", syncErr)
				}
				time.Sleep(time.Millisecond * time.Duration(50*(retryAttempt+1)))
				continue
			}
			tx.Rollback()
			return nil, fmt.Errorf("failed to create transfer record: %v", err)
		}
		break
	}

	if transfer == nil || transfer.ID == 0 {
		tx.Rollback()
		return nil, errors.New("failed to create transfer after multiple attempts")
	}

	sourceAccount.Balance -= request.Amount
	if sourceAccount.IsForeignCurrency() {
		sourceAccount.BookBalanceIDR -= sourceBookAmountIDR
		if sourceAccount.BookBalanceIDR < 0 && sourceAccount.BookBalanceIDR > -0.01 {
			sourceAccount.BookBalanceIDR = 0
		}
		if exchangeRateUsed > 0 {
			sourceAccount.LastExchangeRate = exchangeRateUsed
		}
	}
	if err := tx.Save(sourceAccount).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update source account balance: %v", err)
	}
	if err := s.syncCashBankBookBalanceToGL(tx, sourceAccount); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to sync source GL balance: %v", err)
	}

	sourceTx := &models.CashBankTransaction{
		CashBankID:      request.FromAccountID,
		ReferenceType:   TransactionTypeTransfer,
		ReferenceID:     transfer.ID,
		Amount:          -request.Amount,
		Currency:        normalizeCashBankCurrency(sourceAccount.Currency),
		ExchangeRate:    sourceCarryRate,
		BookAmountIDR:   -sourceBookAmountIDR,
		BalanceAfter:    sourceAccount.Balance,
		TransactionDate: request.Date.ToTimeWithCurrentTime(),
		Notes:           fmt.Sprintf("Transfer to %s", destAccount.Name),
	}
	if err := tx.Create(sourceTx).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	destAccount.Balance += transferAmount
	if destAccount.IsForeignCurrency() {
		destAccount.BookBalanceIDR += destinationBookAmountIDR
		if destinationRate > 0 {
			destAccount.LastExchangeRate = destinationRate
		}
	}
	if err := tx.Save(destAccount).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update destination account balance: %v", err)
	}
	if err := s.syncCashBankBookBalanceToGL(tx, destAccount); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to sync destination GL balance: %v", err)
	}

	destTx := &models.CashBankTransaction{
		CashBankID:      request.ToAccountID,
		ReferenceType:   TransactionTypeTransfer,
		ReferenceID:     transfer.ID,
		Amount:          transferAmount,
		Currency:        normalizeCashBankCurrency(destAccount.Currency),
		ExchangeRate:    destinationRate,
		BookAmountIDR:   destinationBookAmountIDR,
		BalanceAfter:    destAccount.Balance,
		TransactionDate: request.Date.ToTimeWithCurrentTime(),
		Notes:           fmt.Sprintf("Transfer from %s", sourceAccount.Name),
	}
	if err := tx.Create(destTx).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := s.createTransferSSOTJournalEntryWithTx(tx, transfer, sourceAccount, destAccount, sourceBookAmountIDR, destinationBookAmountIDR, forexGainLossAmount, userID, request); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create SSOT transfer journal entry: %v", err)
	}

	if err := guard.EnforceAfterTransaction(tx, sourceAccount.ID, destAccount.ID); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to enforce cash/bank currency invariant after transfer: %v", err)
	}

	return transfer, tx.Commit().Error
}

// ProcessDeposit processes a deposit transaction
func (s *CashBankService) ProcessDeposit(request DepositRequest, userID uint) (*models.CashBankTransaction, error) {
	log.Printf("Processing deposit: AccountID=%d, Amount=%.2f, UserID=%d", request.AccountID, request.Amount, userID)

	tx := s.db.Begin()
	guard := NewCashBankCurrencyGuard(s.db)

	if err := CheckClosedPeriod(tx, request.Date.ToTimeWithCurrentTime()); err != nil {
		tx.Rollback()
		return nil, err
	}

	account, err := s.cashBankRepo.FindByID(request.AccountID)
	if err != nil {
		tx.Rollback()
		return nil, errors.New("account not found")
	}

	if err := database.EnsureCashBankAccountIntegrity(s.db, account.ID); err != nil {
		log.Printf("Warning: Failed to ensure account integrity for deposit ID %d: %v", account.ID, err)
		account, err = s.cashBankRepo.FindByID(request.AccountID)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to reload account after integrity fix: %v", err)
		}
	}

	if request.SourceAccountID != nil {
		if _, err := s.validateCounterAccountWithTx(tx, *request.SourceAccountID, account.AccountID, models.AccountTypeRevenue, models.AccountTypeEquity, models.AccountTypeLiability); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	bookAmountIDR, appliedRate, err := s.calculateIncreaseBookAmount(account, request.Amount, request.ExchangeRate)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	account.Balance += request.Amount
	if account.IsForeignCurrency() {
		account.BookBalanceIDR += bookAmountIDR
		account.LastExchangeRate = appliedRate
	}
	if err := tx.Save(account).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update cash bank balance: %v", err)
	}
	if err := s.syncCashBankBookBalanceToGL(tx, account); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to sync COA balance for GL account %d: %v", account.AccountID, err)
	}

	transaction := &models.CashBankTransaction{
		CashBankID:      request.AccountID,
		ReferenceType:   TransactionTypeDeposit,
		ReferenceID:     0,
		Amount:          request.Amount,
		Currency:        normalizeCashBankCurrency(account.Currency),
		ExchangeRate:    appliedRate,
		BookAmountIDR:   bookAmountIDR,
		BalanceAfter:    account.Balance,
		TransactionDate: request.Date.ToTimeWithCurrentTime(),
		Notes:           request.Notes,
	}
	if err := tx.Create(transaction).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	journalRequest := &CashBankJournalRequest{
		TransactionType: TransactionTypeDeposit,
		CashBankID:      uint64(request.AccountID),
		Amount:          decimal.NewFromFloat(bookAmountIDR),
		Date:            request.Date.ToTimeWithCurrentTime(),
		Reference:       request.Reference,
		Description:     fmt.Sprintf("Deposit to %s", account.Name),
		Notes:           request.Notes,
		CounterAccountID: func() *uint64 {
			if request.SourceAccountID != nil {
				id := uint64(*request.SourceAccountID)
				return &id
			}
			return nil
		}(),
		CreatedBy: uint64(userID),
	}

	if _, err = s.sSOTJournalAdapter.CreateDepositJournalEntryWithTx(tx, account, transaction, journalRequest); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create SSOT deposit journal entry: %v", err)
	}

	if err := guard.EnforceAfterTransaction(tx, account.ID); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to enforce cash/bank currency invariant after deposit: %v", err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit deposit transaction: %v", err)
	}

	return transaction, nil
}

// ProcessWithdrawal processes a withdrawal transaction
func (s *CashBankService) ProcessWithdrawal(request WithdrawalRequest, userID uint) (*models.CashBankTransaction, error) {
	log.Printf("Processing withdrawal: AccountID=%d, Amount=%.2f, UserID=%d", request.AccountID, request.Amount, userID)

	tx := s.db.Begin()
	guard := NewCashBankCurrencyGuard(s.db)

	if err := CheckClosedPeriod(tx, request.Date.ToTimeWithCurrentTime()); err != nil {
		tx.Rollback()
		return nil, err
	}

	account, err := s.cashBankRepo.FindByID(request.AccountID)
	if err != nil {
		tx.Rollback()
		return nil, errors.New("account not found")
	}

	if err := database.EnsureCashBankAccountIntegrity(s.db, account.ID); err != nil {
		log.Printf("Warning: Failed to ensure account integrity for withdrawal ID %d: %v", account.ID, err)
		account, err = s.cashBankRepo.FindByID(request.AccountID)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to reload account after integrity fix: %v", err)
		}
	}

	if request.TargetAccountID != nil {
		if _, err := s.validateCounterAccountWithTx(tx, *request.TargetAccountID, account.AccountID, models.AccountTypeExpense, models.AccountTypeLiability, models.AccountTypeAsset); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	availableBalance := account.AvailableBalance()
	if availableBalance < request.Amount {
		tx.Rollback()
		if account.FrozenAmount > 0 {
			return nil, fmt.Errorf("saldo tersedia tidak cukup. Saldo tersedia: %.2f (Total: %.2f, Dana terblokir: %.2f)", availableBalance, account.Balance, account.FrozenAmount)
		}
		return nil, fmt.Errorf("insufficient balance. Available: %.2f", availableBalance)
	}

	bookAmountIDR, appliedRate, err := s.calculateReductionBookAmount(account, request.Amount)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	account.Balance -= request.Amount
	if account.IsForeignCurrency() {
		account.BookBalanceIDR -= bookAmountIDR
		if account.BookBalanceIDR < 0 && account.BookBalanceIDR > -0.01 {
			account.BookBalanceIDR = 0
		}
	}
	if err := tx.Save(account).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update cash bank balance: %v", err)
	}
	if err := s.syncCashBankBookBalanceToGL(tx, account); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to sync COA balance for GL account %d: %v", account.AccountID, err)
	}

	transaction := &models.CashBankTransaction{
		CashBankID:      request.AccountID,
		ReferenceType:   TransactionTypeWithdrawal,
		ReferenceID:     0,
		Amount:          -request.Amount,
		Currency:        normalizeCashBankCurrency(account.Currency),
		ExchangeRate:    appliedRate,
		BookAmountIDR:   -bookAmountIDR,
		BalanceAfter:    account.Balance,
		TransactionDate: request.Date.ToTimeWithCurrentTime(),
		Notes:           request.Notes,
	}
	if err := tx.Create(transaction).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	journalRequest := &CashBankJournalRequest{
		TransactionType: TransactionTypeWithdrawal,
		CashBankID:      uint64(request.AccountID),
		Amount:          decimal.NewFromFloat(bookAmountIDR),
		Date:            request.Date.ToTimeWithCurrentTime(),
		Reference:       request.Reference,
		Description:     fmt.Sprintf("Withdrawal from %s", account.Name),
		Notes:           request.Notes,
		CounterAccountID: func() *uint64 {
			if request.TargetAccountID != nil {
				id := uint64(*request.TargetAccountID)
				return &id
			}
			return nil
		}(),
		CreatedBy: uint64(userID),
	}

	if _, err = s.sSOTJournalAdapter.CreateWithdrawalJournalEntryWithTx(tx, account, transaction, journalRequest); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create SSOT withdrawal journal entry: %v", err)
	}

	if err := guard.EnforceAfterTransaction(tx, account.ID); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to enforce cash/bank currency invariant after withdrawal: %v", err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit withdrawal transaction: %v", err)
	}

	return transaction, nil
}

// GetTransactions retrieves transactions for a cash/bank account
func (s *CashBankService) GetTransactions(accountID uint, filter TransactionFilter) (*TransactionResult, error) {
	// Convert service filter to repository filter
	repoFilter := repositories.TransactionFilter{
		StartDate: filter.StartDate,
		EndDate:   filter.EndDate,
		Type:      filter.Type,
		SortOrder: filter.SortOrder,
		Page:      filter.Page,
		Limit:     filter.Limit,
	}

	// Get transactions from repository
	result, err := s.cashBankRepo.GetTransactions(accountID, repoFilter)
	if err != nil {
		return nil, err
	}

	// Convert repository result to service result
	return &TransactionResult{
		Data:       result.Data,
		Total:      result.Total,
		Page:       result.Page,
		Limit:      result.Limit,
		TotalPages: result.TotalPages,
	}, nil
}

// GetBalanceSummary gets balance summary for all accounts
func (s *CashBankService) GetBalanceSummary() (*BalanceSummary, error) {
	repoSummary, err := s.cashBankRepo.GetBalanceSummary()
	if err != nil {
		return nil, err
	}

	accounts, err := s.cashBankRepo.FindAll(true)
	if err != nil {
		return nil, err
	}

	summary := &BalanceSummary{
		TotalCash:    repoSummary.TotalCash,
		TotalBank:    repoSummary.TotalBank,
		TotalBalance: repoSummary.TotalBalance,
		ByAccount:    []AccountBalance{},
		ByCurrency:   repoSummary.ByCurrency,
	}

	for _, account := range accounts {
		if account.IsActive {
			summary.ByAccount = append(summary.ByAccount, AccountBalance{
				AccountID:   account.ID,
				AccountName: account.Name,
				AccountType: account.Type,
				Balance:     account.EffectiveBookBalanceIDR(),
				Currency:    account.Currency,
			})
		}
	}

	return summary, nil
}

// GetPaymentAccounts gets active cash and bank accounts for payment processing
func (s *CashBankService) GetPaymentAccounts() ([]models.CashBank, error) {
	accounts, err := s.cashBankRepo.FindAll(false)
	if err != nil {
		return nil, err
	}

	var paymentAccounts []models.CashBank
	for _, account := range accounts {
		if account.IsActive {
			paymentAccounts = append(paymentAccounts, account)
		}
	}

	return paymentAccounts, nil
}

// ReconcileAccount reconciles bank account with statement
func (s *CashBankService) ReconcileAccount(accountID uint, request ReconciliationRequest, userID uint) (*BankReconciliation, error) {
	tx := s.db.Begin()

	account, err := s.cashBankRepo.FindByID(accountID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if account.Type != models.CashBankTypeBank {
		tx.Rollback()
		return nil, errors.New("reconciliation only for bank accounts")
	}

	// Create reconciliation record
	reconciliation := &BankReconciliation{
		CashBankID:       accountID,
		ReconcileDate:    request.Date.ToTime(),
		StatementBalance: request.StatementBalance,
		SystemBalance:    account.Balance,
		Difference:       request.StatementBalance - account.Balance,
		Status:           "PENDING",
		UserID:           userID,
	}

	if err := tx.Create(reconciliation).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Process reconciliation items
	for _, item := range request.Items {
		recItem := &ReconciliationItem{
			ReconciliationID: reconciliation.ID,
			TransactionID:    item.TransactionID,
			IsCleared:        item.IsCleared,
			Notes:            item.Notes,
		}

		if err := tx.Create(recItem).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// If difference is zero and all items cleared, mark as completed
	if reconciliation.Difference == 0 {
		reconciliation.Status = "COMPLETED"
		if err := tx.Save(reconciliation).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	return reconciliation, tx.Commit().Error
}

// Helper functions

func (s *CashBankService) generateCashBankCode(accountType string) string {
	prefix := "CSH"
	if accountType == models.CashBankTypeBank {
		prefix = "BNK"
	}

	year := time.Now().Year()

	// Try to get next sequence number using safer method
	sequenceNum, err := s.cashBankRepo.GetNextSequenceNumber(accountType, year)
	if err != nil {
		// Fallback to count method with microsecond timestamp
		microsecond := time.Now().UnixMicro() % 10000
		count, _ := s.cashBankRepo.CountByType(accountType)
		return fmt.Sprintf("%s-%04d-%04d-%04d", prefix, year, count+1, microsecond)
	}

	return fmt.Sprintf("%s-%04d-%04d", prefix, year, sequenceNum)
}

func (s *CashBankService) generateAccountCode(accountType string) string {
	// Use proper PSAK compliant account codes with sequential numbering
	var parentCode string
	if accountType == models.CashBankTypeCash {
		parentCode = "1101" // Cash accounts
	} else {
		// Bank accounts - also under 1101 (KAS DAN SETARA KAS)
		// Per COA template: 1101-003 = BANK GIRO, 1101-004 = BANK TABUNGAN
		parentCode = "1101" // Bank accounts under KAS DAN SETARA KAS
	}

	// Find the highest existing child account code for this parent
	childCode := s.generateSequentialChildCode(parentCode)
	return childCode
}

// generateUniqueAccountCode generates truly unique account code with name-based suffix
// Ã¢Å“â€¦ FIX ROOT CAUSE: Ensures each cash/bank gets its own unique GL
func (s *CashBankService) generateUniqueAccountCode(baseCode, accountName string) string {
	// Try sequential first
	sequentialCode := s.generateSequentialChildCode(baseCode)

	// Verify it's actually unique by checking database
	if existingAcc, err := s.accountRepo.GetAccountByCode(sequentialCode); err == nil && existingAcc != nil {
		// Code already exists, add timestamp suffix for uniqueness
		timestamp := time.Now().Unix() % 10000
		return fmt.Sprintf("%s-%04d", baseCode, timestamp)
	}

	return sequentialCode
}

// generateSequentialChildCode generates the next sequential child code for a parent account
func (s *CashBankService) generateSequentialChildCode(parentCode string) string {
	// Ensure parent account exists and is properly configured
	if err := s.ensureParentAccountExists(parentCode); err != nil {
		log.Printf("Warning: Failed to ensure parent account %s exists: %v", parentCode, err)
	}

	// Get all existing accounts to find existing child codes
	allAccounts, err := s.accountRepo.FindAll(context.Background())
	if err != nil {
		log.Printf("Warning: Failed to get accounts for sequential numbering: %v", err)
		// Fallback to timestamp-based approach
		return fmt.Sprintf("%s-%03d", parentCode, time.Now().Unix()%1000)
	}

	// Find the highest existing child number for this parent
	maxChildNumber := 0
	childPrefix := parentCode + "-"

	for _, account := range allAccounts {
		if strings.HasPrefix(account.Code, childPrefix) {
			// Extract child number (e.g., "001" from "1101-001")
			childPart := strings.TrimPrefix(account.Code, childPrefix)
			if len(childPart) == 3 {
				if num, err := strconv.Atoi(childPart); err == nil && num > maxChildNumber {
					maxChildNumber = num
				}
			}
		}
	}

	// Generate next child code with 3-digit format
	nextChildNumber := maxChildNumber + 1
	if nextChildNumber > 999 {
		log.Printf("Warning: Child account limit reached for parent %s, using fallback", parentCode)
		// Fallback to timestamp-based approach if we've exhausted 999 accounts
		return fmt.Sprintf("%s-%03d", parentCode, time.Now().Unix()%1000)
	}

	return fmt.Sprintf("%s-%03d", parentCode, nextChildNumber)
}

// ensureParentAccountExists creates parent account if it doesn't exist
func (s *CashBankService) ensureParentAccountExists(parentCode string) error {
	// Check if parent exists
	_, err := s.accountRepo.FindByCode(context.Background(), parentCode)
	if err == nil {
		return nil // Parent already exists
	}

	// Create parent account based on code
	var parentAccount *models.Account

	switch parentCode {
	case "1101":
		// Cash parent
		parentAccount = &models.Account{
			Code:        "1101",
			Name:        "KAS",
			Type:        models.AccountTypeAsset,
			Category:    models.CategoryCurrentAsset,
			Level:       3,
			IsHeader:    true, // Set as header since we'll create children
			IsActive:    true,
			Description: "Parent account for all cash accounts",
		}
		// Set parent to 1100 (CURRENT ASSETS) if exists
		if currentAssetsParent, err := s.accountRepo.FindByCode(context.Background(), "1100"); err == nil {
			parentAccount.ParentID = &currentAssetsParent.ID
		}

	// Note: case "1102" removed - bank accounts now use 1101 as parent
	// 1102 is PIUTANG USAHA in the standard COA template

	default:
		return fmt.Errorf("unsupported parent code: %s", parentCode)
	}

	// Create parent account
	tx := s.db.Begin()
	if err := tx.Create(parentAccount).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create parent account %s: %w", parentCode, err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit parent account creation: %w", err)
	}

	log.Printf("Ã¢Å“â€¦ Created parent account: %s - %s", parentAccount.Code, parentAccount.Name)
	return nil
}

func (s *CashBankService) getAccountCategory(cashBankType string) string {
	if cashBankType == models.CashBankTypeCash {
		return "CURRENT_ASSET"
	}
	return "CURRENT_ASSET"
}

func (s *CashBankService) generateTransferNumber() string {
	year := time.Now().Year()
	month := time.Now().Month()

	// Get the count of transfers for this month with retry logic
	var count int64
	for retry := 0; retry < 5; retry++ {
		err := s.db.Raw(`
			SELECT COALESCE(COUNT(*), 0) 
			FROM cash_bank_transfers 
			WHERE EXTRACT(YEAR FROM created_at) = ? 
			  AND EXTRACT(MONTH FROM created_at) = ?
			  AND deleted_at IS NULL
		`, year, month).Scan(&count).Error

		if err == nil {
			break
		}

		// Log warning and retry
		log.Printf("Warning: Failed to get transfer count (attempt %d): %v", retry+1, err)
		time.Sleep(time.Millisecond * time.Duration(100*(retry+1)))
	}

	// Increment count to get next number
	count++

	// Add microsecond timestamp to ensure uniqueness in case of concurrent requests
	microsecond := time.Now().UnixMicro() % 10000

	return fmt.Sprintf("TRF/%04d/%02d/%04d-%04d", year, month, count, microsecond)
}

func (s *CashBankService) syncCashBankTransferSequence(tx *gorm.DB) error {
	return tx.Exec(`
		SELECT setval(
			pg_get_serial_sequence('cash_bank_transfers', 'id'),
			COALESCE((SELECT MAX(id) FROM cash_bank_transfers), 1),
			true
		)
	`).Error
}

func (s *CashBankService) createOpeningBalanceTransaction(tx *gorm.DB, cashBank *models.CashBank, amount float64, date time.Time, userID uint, sourceAccountID *uint) error {
	log.Printf("[OPENING BALANCE] Creating opening balance transaction: CashBankID=%d, Amount=%.2f, Date=%s, UserID=%d", cashBank.ID, amount, date.Format("2006-01-02"), userID)
	guard := NewCashBankCurrencyGuard(s.db)

	bookAmountIDR, appliedRate, err := s.calculateIncreaseBookAmount(cashBank, amount, cashBank.LastExchangeRate)
	if err != nil {
		return fmt.Errorf("failed to calculate opening book amount: %v", err)
	}

	cashBank.Balance = amount
	if cashBank.IsForeignCurrency() {
		cashBank.BookBalanceIDR = bookAmountIDR
		cashBank.LastExchangeRate = appliedRate
	}
	if err := tx.Save(cashBank).Error; err != nil {
		return fmt.Errorf("failed to update cash bank balance: %v", err)
	}
	if err := s.syncCashBankBookBalanceToGL(tx, cashBank); err != nil {
		return fmt.Errorf("failed to sync opening GL balance: %v", err)
	}

	transaction := &models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   TransactionTypeOpeningBalance,
		ReferenceID:     0,
		Amount:          amount,
		Currency:        normalizeCashBankCurrency(cashBank.Currency),
		ExchangeRate:    appliedRate,
		BookAmountIDR:   bookAmountIDR,
		BalanceAfter:    amount,
		TransactionDate: date,
		Notes:           "Opening Balance",
	}
	if err := tx.Create(transaction).Error; err != nil {
		return fmt.Errorf("failed to create transaction record: %v", err)
	}

	journalRequest := &CashBankJournalRequest{
		TransactionType: TransactionTypeOpeningBalance,
		CashBankID:      uint64(cashBank.ID),
		Amount:          decimal.NewFromFloat(bookAmountIDR),
		Date:            date,
		Reference:       fmt.Sprintf("OB-%s", cashBank.Code),
		Description:     fmt.Sprintf("Opening balance for %s", cashBank.Name),
		Notes:           "Opening Balance",
		CreatedBy:       uint64(userID),
		CounterAccountID: func() *uint64 {
			if sourceAccountID != nil {
				id := uint64(*sourceAccountID)
				return &id
			}
			return nil
		}(),
	}

	if _, err := s.sSOTJournalAdapter.CreateOpeningBalanceJournalEntryWithTx(tx, cashBank, transaction, journalRequest); err != nil {
		return fmt.Errorf("SSOT journal entry creation failed: %v", err)
	}

	if err := guard.EnforceAfterTransaction(tx, cashBank.ID); err != nil {
		return fmt.Errorf("failed to enforce cash/bank currency invariant after opening balance: %v", err)
	}

	return nil
}

// ensureValidAccountID creates or finds a valid account_id for cash bank record
func (s *CashBankService) ensureValidAccountID(cashBank *models.CashBank) (uint, error) {
	// Try to find an existing account by name
	allAccounts, err := s.accountRepo.FindAll(context.Background())
	if err == nil {
		for _, account := range allAccounts {
			if account.Name == cashBank.Name && account.Type == "ASSET" && account.Category == "CURRENT_ASSET" {
				// Use the matching account
				return account.ID, nil
			}
		}
	}

	// If no account found, create a new one
	accountCode := s.generateAccountCode(cashBank.Type)
	newAccount := &models.Account{
		Code:        accountCode,
		Name:        cashBank.Name,
		Type:        "ASSET",
		Category:    s.getAccountCategory(cashBank.Type),
		IsActive:    true,
		Description: fmt.Sprintf("Auto-created GL account for %s: %s (%s)", cashBank.Type, cashBank.Name, cashBank.Code),
		Level:       3,
	}

	// Start transaction for account creation
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(newAccount).Error; err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("failed to create GL account: %v", err)
	}

	if err := tx.Commit().Error; err != nil {
		return 0, fmt.Errorf("failed to commit account creation: %v", err)
	}

	log.Printf("Created new GL account %d (%s) for cash bank %d", newAccount.ID, newAccount.Code, cashBank.ID)
	return newAccount.ID, nil
}

// DTOs and Models

// CustomDate for handling date-only formats from frontend
type CustomDate time.Time

// UnmarshalJSON handles multiple date formats
func (cd *CustomDate) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "null" || s == "" {
		return nil
	}

	// Try multiple date formats
	formats := []string{
		"2006-01-02",                // YYYY-MM-DD from frontend
		"2006-01-02T15:04:05Z",      // Full ISO format
		"2006-01-02T15:04:05Z07:00", // RFC3339
		"2006-01-02 15:04:05",       // MySQL datetime
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			*cd = CustomDate(t)
			return nil
		}
	}

	return fmt.Errorf("cannot parse date: %s", s)
}

// MarshalJSON converts to JSON
func (cd CustomDate) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(cd).Format("2006-01-02"))
}

// ToTime converts to time.Time
func (cd CustomDate) ToTime() time.Time {
	return time.Time(cd)
}

// ToTimeWithCurrentTime converts date to current time (preserving date but using current time)
func (cd CustomDate) ToTimeWithCurrentTime() time.Time {
	baseDate := time.Time(cd)
	now := time.Now()
	// Combine the date from request with current time
	return time.Date(
		baseDate.Year(),
		baseDate.Month(),
		baseDate.Day(),
		now.Hour(),
		now.Minute(),
		now.Second(),
		now.Nanosecond(),
		now.Location(),
	)
}

type CashBankCreateRequest struct {
	Name                    string     `json:"name" binding:"required"`
	Type                    string     `json:"type" binding:"required,oneof=CASH BANK"`
	AccountID               uint       `json:"account_id"`
	BankName                string     `json:"bank_name"`
	AccountNo               string     `json:"account_no"`
	AccountHolderName       string     `json:"account_holder_name"`
	Branch                  string     `json:"branch"`
	Currency                string     `json:"currency"`
	OpeningBalance          float64    `json:"opening_balance"`
	OpeningExchangeRate     float64    `json:"opening_exchange_rate"`
	OpeningDate             CustomDate `json:"opening_date"`
	Description             string     `json:"description"`
	IsHoldingAccount        bool       `json:"is_holding_account"`
	AllowDirectTransactions *bool      `json:"allow_direct_transactions"`
	SourceAccountID         *uint      `json:"source_account_id"` // Optional: Equity account for opening balance
}

type CashBankUpdateRequest struct {
	Name                    string `json:"name"`
	BankName                string `json:"bank_name"`
	AccountNo               string `json:"account_no"`
	AccountHolderName       string `json:"account_holder_name"`
	Branch                  string `json:"branch"`
	Description             string `json:"description"`
	IsActive                *bool  `json:"is_active"`
	IsHoldingAccount        *bool  `json:"is_holding_account"`
	AllowDirectTransactions *bool  `json:"allow_direct_transactions"`
}

type TransferRequest struct {
	FromAccountID uint       `json:"from_account_id" binding:"required"`
	ToAccountID   uint       `json:"to_account_id" binding:"required"`
	Date          CustomDate `json:"date" binding:"required"`
	Amount        float64    `json:"amount" binding:"required,min=0"`
	ExchangeRate  float64    `json:"exchange_rate"`
	Reference     string     `json:"reference"`
	Notes         string     `json:"notes"`
}

type ManualJournalEntry struct {
	AccountID    uint    `json:"account_id" binding:"required"`
	Description  string  `json:"description"`
	DebitAmount  float64 `json:"debit_amount"`
	CreditAmount float64 `json:"credit_amount"`
}

type DepositRequest struct {
	AccountID       uint       `json:"account_id" binding:"required"`
	Date            CustomDate `json:"date" binding:"required"`
	Amount          float64    `json:"amount" binding:"required,min=0"`
	ExchangeRate    float64    `json:"exchange_rate"`
	Reference       string     `json:"reference"`
	Notes           string     `json:"notes"`
	SourceAccountID *uint      `json:"source_account_id"` // Optional: Revenue account for deposit source
}

type WithdrawalRequest struct {
	AccountID       uint       `json:"account_id" binding:"required"`
	Date            CustomDate `json:"date" binding:"required"`
	Amount          float64    `json:"amount" binding:"required,min=0"`
	Reference       string     `json:"reference"`
	Notes           string     `json:"notes"`
	TargetAccountID *uint      `json:"target_account_id"` // Optional: Expense account for withdrawal target
}

type ReconciliationRequest struct {
	Date             CustomDate                  `json:"date" binding:"required"`
	StatementBalance float64                     `json:"statement_balance" binding:"required"`
	Items            []ReconciliationItemRequest `json:"items"`
}

type ReconciliationItemRequest struct {
	TransactionID uint   `json:"transaction_id"`
	IsCleared     bool   `json:"is_cleared"`
	Notes         string `json:"notes"`
}

type TransactionFilter struct {
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Type      string    `json:"type"`
	SortOrder string    `json:"sort_order"`
	Page      int       `json:"page"`
	Limit     int       `json:"limit"`
}

type TransactionResult struct {
	Data       []models.CashBankTransaction `json:"data"`
	Total      int64                        `json:"total"`
	Page       int                          `json:"page"`
	Limit      int                          `json:"limit"`
	TotalPages int                          `json:"total_pages"`
}

type BalanceSummary struct {
	TotalCash    float64            `json:"total_cash"`
	TotalBank    float64            `json:"total_bank"`
	TotalBalance float64            `json:"total_balance"`
	ByAccount    []AccountBalance   `json:"by_account"`
	ByCurrency   map[string]float64 `json:"by_currency"`
}

type AccountBalance struct {
	AccountID   uint    `json:"account_id"`
	AccountName string  `json:"account_name"`
	AccountType string  `json:"account_type"`
	Balance     float64 `json:"balance"`
	Currency    string  `json:"currency"`
}

type CashBankTransfer struct {
	ID                       uint   `gorm:"primaryKey"`
	TransferNumber           string `gorm:"unique;not null;size:50"`
	FromAccountID            uint   `gorm:"not null;index"`
	ToAccountID              uint   `gorm:"not null;index"`
	Date                     time.Time
	Amount                   float64 `gorm:"type:decimal(15,2)"`
	ExchangeRate             float64 `gorm:"type:decimal(12,6);default:1"`
	ConvertedAmount          float64 `gorm:"type:decimal(15,2)"`
	SourceBookAmountIDR      float64 `gorm:"type:decimal(20,2);default:0"`
	DestinationBookAmountIDR float64 `gorm:"type:decimal(20,2);default:0"`
	ForexGainLossAmount      float64 `gorm:"type:decimal(20,2);default:0"`
	ForexGainLossType        string  `gorm:"size:10"`
	Reference                string  `gorm:"size:100"`
	Notes                    string  `gorm:"type:text"`
	Status                   string  `gorm:"size:20"`
	UserID                   uint    `gorm:"not null;index"`
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

type BankReconciliation struct {
	ID               uint `gorm:"primaryKey"`
	CashBankID       uint `gorm:"not null;index"`
	ReconcileDate    time.Time
	StatementBalance float64 `gorm:"type:decimal(15,2)"`
	SystemBalance    float64 `gorm:"type:decimal(15,2)"`
	Difference       float64 `gorm:"type:decimal(15,2)"`
	Status           string  `gorm:"size:20"`
	UserID           uint    `gorm:"not null;index"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ReconciliationItem struct {
	ID               uint `gorm:"primaryKey"`
	ReconciliationID uint `gorm:"not null;index"`
	TransactionID    uint `gorm:"not null;index"`
	IsCleared        bool
	Notes            string `gorm:"type:text"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// AvailableGLAccount represents a GL account available for cash/bank linking
type AvailableGLAccount struct {
	ID       uint    `json:"id"`
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	Balance  float64 `json:"balance"`
	Category string  `json:"category"`
}

// GetAvailableGLAccounts returns GL accounts that are:
// 1. Not already linked to any cash/bank account
// 2. Filtered by type (CASH shows accounts with "KAS" in name, BANK shows accounts with "BANK" in name)
// 3. Only ASSET type accounts
// 4. Only active, non-header accounts
func (s *CashBankService) GetAvailableGLAccounts(accountType string) ([]AvailableGLAccount, error) {
	log.Printf("Ã°Å¸â€Â [CASHBANK SERVICE] Getting available GL accounts for type: %s", accountType)

	// Build query to get ASSET accounts that are:
	// - Not headers
	// - Active
	// - Not already linked to any cash_banks record
	// - Match the type filter (KAS for CASH, BANK for BANK)

	var nameFilter string
	if accountType == "CASH" {
		nameFilter = "KAS"
	} else {
		nameFilter = "BANK"
	}

	query := `
		SELECT a.id, a.code, a.name, a.balance, a.category
		FROM accounts a
		WHERE a.type = 'ASSET'
		  AND a.is_active = true
		  AND (a.is_header = false OR a.is_header IS NULL)
		  AND a.deleted_at IS NULL
		  AND UPPER(a.name) LIKE ?
		  AND a.id NOT IN (
			SELECT cb.account_id 
			FROM cash_banks cb 
			WHERE cb.deleted_at IS NULL AND cb.account_id IS NOT NULL
		  )
		ORDER BY a.code ASC
	`

	var accounts []AvailableGLAccount
	err := s.db.Raw(query, "%"+nameFilter+"%").Scan(&accounts).Error
	if err != nil {
		log.Printf("Ã¢ÂÅ’ [CASHBANK SERVICE] Failed to get available GL accounts: %v", err)
		return nil, fmt.Errorf("failed to get available GL accounts: %v", err)
	}

	log.Printf("Ã¢Å“â€¦ [CASHBANK SERVICE] Found %d available GL accounts for type %s", len(accounts), accountType)
	return accounts, nil
}

// ============================================================
// Frozen Fund Management
// ============================================================

// FreezeAmountRequest represents a request to freeze funds
type FreezeAmountRequest struct {
	Amount float64 `json:"amount" binding:"required,gt=0"`
	Reason string  `json:"reason"`
}

// UnfreezeAmountRequest represents a request to unfreeze funds
type UnfreezeAmountRequest struct {
	Amount float64 `json:"amount" binding:"required,gt=0"`
	Reason string  `json:"reason"`
}

// FreezeAmount freezes (blocks) a portion of funds in a cash/bank account
func (s *CashBankService) FreezeAmount(cashBankID uint, request FreezeAmountRequest, userID uint) (*models.CashBank, error) {
	log.Printf("Ã°Å¸â€â€™ [CASHBANK SERVICE] Freezing funds: AccountID=%d, Amount=%.2f, Reason=%s, UserID=%d",
		cashBankID, request.Amount, request.Reason, userID)

	account, err := s.cashBankRepo.FindByID(cashBankID)
	if err != nil {
		return nil, fmt.Errorf("akun tidak ditemukan: %v", err)
	}

	// Validate: new frozen amount cannot exceed balance
	newFrozenAmount := account.FrozenAmount + request.Amount
	if newFrozenAmount > account.Balance {
		return nil, fmt.Errorf("dana yang diblokir (%.2f) tidak boleh melebihi saldo total (%.2f). Saat ini sudah terblokir: %.2f",
			newFrozenAmount, account.Balance, account.FrozenAmount)
	}

	previousFrozen := account.FrozenAmount

	// Update frozen amount
	if err := s.cashBankRepo.UpdateFrozenAmount(cashBankID, newFrozenAmount, request.Reason, userID); err != nil {
		return nil, fmt.Errorf("gagal memblokir dana: %v", err)
	}

	// Create history record
	history := &models.CashBankFrozenHistory{
		CashBankID:     cashBankID,
		Action:         models.FrozenActionFreeze,
		Amount:         request.Amount,
		PreviousFrozen: previousFrozen,
		NewFrozen:      newFrozenAmount,
		Reason:         request.Reason,
		UserID:         userID,
	}
	if err := s.cashBankRepo.CreateFrozenHistory(history); err != nil {
		log.Printf("Ã¢Å¡Â Ã¯Â¸Â Failed to create frozen history: %v", err)
	}

	log.Printf("Ã¢Å“â€¦ [CASHBANK SERVICE] Funds frozen successfully: AccountID=%d, FrozenAmount=%.2f -> %.2f",
		cashBankID, previousFrozen, newFrozenAmount)

	return s.cashBankRepo.FindByID(cashBankID)
}

// UnfreezeAmount unfreezes (unblocks) a portion of frozen funds
func (s *CashBankService) UnfreezeAmount(cashBankID uint, request UnfreezeAmountRequest, userID uint) (*models.CashBank, error) {
	log.Printf("Ã°Å¸â€â€œ [CASHBANK SERVICE] Unfreezing funds: AccountID=%d, Amount=%.2f, Reason=%s, UserID=%d",
		cashBankID, request.Amount, request.Reason, userID)

	account, err := s.cashBankRepo.FindByID(cashBankID)
	if err != nil {
		return nil, fmt.Errorf("akun tidak ditemukan: %v", err)
	}

	// Validate: cannot unfreeze more than what's frozen
	if request.Amount > account.FrozenAmount {
		return nil, fmt.Errorf("jumlah yang dibuka (%.2f) melebihi dana yang terblokir (%.2f)",
			request.Amount, account.FrozenAmount)
	}

	previousFrozen := account.FrozenAmount
	newFrozenAmount := account.FrozenAmount - request.Amount

	// Update frozen amount
	if err := s.cashBankRepo.UpdateFrozenAmount(cashBankID, newFrozenAmount, request.Reason, userID); err != nil {
		return nil, fmt.Errorf("gagal membuka blokir dana: %v", err)
	}

	// Create history record
	history := &models.CashBankFrozenHistory{
		CashBankID:     cashBankID,
		Action:         models.FrozenActionUnfreeze,
		Amount:         request.Amount,
		PreviousFrozen: previousFrozen,
		NewFrozen:      newFrozenAmount,
		Reason:         request.Reason,
		UserID:         userID,
	}
	if err := s.cashBankRepo.CreateFrozenHistory(history); err != nil {
		log.Printf("Ã¢Å¡Â Ã¯Â¸Â Failed to create frozen history: %v", err)
	}

	log.Printf("Ã¢Å“â€¦ [CASHBANK SERVICE] Funds unfrozen successfully: AccountID=%d, FrozenAmount=%.2f -> %.2f",
		cashBankID, previousFrozen, newFrozenAmount)

	return s.cashBankRepo.FindByID(cashBankID)
}

// GetFrozenHistory retrieves the freeze/unfreeze history for a cash/bank account
func (s *CashBankService) GetFrozenHistory(cashBankID uint) ([]models.CashBankFrozenHistory, error) {
	// Validate account exists
	if _, err := s.cashBankRepo.FindByID(cashBankID); err != nil {
		return nil, fmt.Errorf("akun tidak ditemukan: %v", err)
	}
	return s.cashBankRepo.GetFrozenHistory(cashBankID)
}
