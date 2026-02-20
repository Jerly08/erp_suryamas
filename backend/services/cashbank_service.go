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
func (s *CashBankService) GetCashBankAccounts() ([]models.CashBank, error) {
	return s.cashBankRepo.FindAll()
}

// GetCashBankByID retrieves cash/bank account by ID
func (s *CashBankService) GetCashBankByID(id uint) (*models.CashBank, error) {
	return s.cashBankRepo.FindByID(id)
}

// CreateCashBankAccount creates a new cash or bank account
func (s *CashBankService) CreateCashBankAccount(request CashBankCreateRequest, userID uint) (result *models.CashBank, err error) {
	// Log method entry with parameters
	log.Printf("🔄 [CASHBANK SERVICE] Creating cash bank account: Name=%s, Type=%s, AccountID=%d, OpeningBalance=%.2f, UserID=%d",
		request.Name, request.Type, request.AccountID, request.OpeningBalance, userID)

	// Start transaction
	tx := s.db.Begin()

	// Panic recovery with transaction rollback
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			stackTrace := string(debug.Stack())
			log.Printf("❌ [CASHBANK SERVICE] PANIC in CreateCashBankAccount: %v", r)
			log.Printf("❌ [CASHBANK SERVICE] Stack trace:\n%s", stackTrace)
			err = fmt.Errorf("internal error during cash bank account creation: %v", r)
			result = nil
		}
	}()

	// Validate GL account if provided
	log.Printf("🔍 [CASHBANK SERVICE] Step 1: Validating GL account (AccountID=%d)", request.AccountID)
	var glAccount *models.Account
	if request.AccountID > 0 {
		account, findErr := s.accountRepo.FindByID(context.Background(), request.AccountID)
		if findErr != nil {
			tx.Rollback()
			log.Printf("❌ [CASHBANK SERVICE] GL account validation failed for ID %d: %v", request.AccountID, findErr)
			return nil, fmt.Errorf("GL account validation failed for ID %d: GL account not found", request.AccountID)
		}

		// ✅ FIX ROOT CAUSE: Prevent multiple cash/bank accounts sharing same GL
		// Check if this GL is already linked to another cash/bank account
		log.Printf("🔍 [CASHBANK SERVICE] Checking if GL account %d is already linked", request.AccountID)
		var existingCashBank models.CashBank
		findErr = tx.Where("account_id = ? AND deleted_at IS NULL", request.AccountID).Limit(1).Find(&existingCashBank).Error
		if findErr != nil {
			// Unexpected database error
			tx.Rollback()
			log.Printf("❌ [CASHBANK SERVICE] Failed to check GL account linkage: %v", findErr)
			return nil, fmt.Errorf("failed to check GL account linkage: %v", findErr)
		}
		if existingCashBank.ID != 0 {
			// GL already linked to another cash/bank account
			tx.Rollback()
			log.Printf("❌ [CASHBANK SERVICE] GL account %s is already linked to cash/bank account %s", account.Code, existingCashBank.Name)
			return nil, fmt.Errorf("GL account '%s - %s' is already linked to cash/bank account '%s'. Each cash/bank account must have its own unique GL account for proper balance tracking",
				account.Code, account.Name, existingCashBank.Name)
		}
		// GL is available, proceed
		glAccount = account
		log.Printf("✅ [CASHBANK SERVICE] GL account validated: %s - %s (verified unique)", account.Code, account.Name)
	} else {
		// ✅ FIX ROOT CAUSE: Always create UNIQUE GL account for each cash/bank
		// Never reuse existing GL to prevent balance confusion
		log.Printf("🔨 [CASHBANK SERVICE] Step 2: Creating new GL account for cash/bank: %s", request.Name)
		ctx := context.Background()

		// Generate UNIQUE account code with timestamp to ensure uniqueness
		baseCode := s.generateAccountCode(request.Type)
		accountCode := s.generateUniqueAccountCode(baseCode, request.Name)

		log.Printf("📋 [CASHBANK SERVICE] Generated GL account code: %s", accountCode)
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
				log.Printf("⚠️ [CASHBANK SERVICE] GL account code collision, retrying with unique code")
				// Retry with nano timestamp
				accountCode = fmt.Sprintf("%s-%d", baseCode, time.Now().UnixNano()%100000)
				accountRequest.Code = accountCode
				createdAccount, createGLErr = s.accountRepo.Create(ctx, accountRequest)
				if createGLErr != nil {
					tx.Rollback()
					log.Printf("❌ [CASHBANK SERVICE] GL account creation failed after retry: %v", createGLErr)
					return nil, fmt.Errorf("GL account creation failed: failed to create unique GL account after retry: %v", createGLErr)
				}
			} else {
				tx.Rollback()
				log.Printf("❌ [CASHBANK SERVICE] GL account creation failed: %v", createGLErr)
				return nil, fmt.Errorf("GL account creation failed: %v", createGLErr)
			}
		}
		glAccount = createdAccount
		log.Printf("✅ [CASHBANK SERVICE] GL account created: %s - %s", glAccount.Code, glAccount.Name)
	}

	// Generate code and create account with retry to avoid unique constraint conflicts
	log.Printf("🔨 [CASHBANK SERVICE] Step 3: Creating cash bank record")
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

		log.Printf("📋 [CASHBANK SERVICE] Attempting to create cash bank with code: %s (attempt %d)", code, retry+1)

		cashBank = &models.CashBank{
			Code:              code,
			Name:              request.Name,
			Type:              request.Type,
			AccountID:         glAccount.ID,
			BankName:          request.BankName,
			AccountNo:         request.AccountNo,
			AccountHolderName: request.AccountHolderName,
			Branch:            request.Branch,
			Currency:          request.Currency,
			Balance:           0, // Will be set via opening balance transaction
			IsActive:          true,
			Description:       request.Description,
		}

		// Create a savepoint before attempting to insert
		savePointName := fmt.Sprintf("sp_create_cashbank_%d", retry)
		if err := tx.SavePoint(savePointName).Error; err != nil {
			log.Printf("❌ [CASHBANK SERVICE] Failed to create savepoint: %v", err)
			tx.Rollback()
			return nil, fmt.Errorf("failed to create savepoint: %v", err)
		}

		createErr = tx.Create(cashBank).Error
		if createErr == nil {
			log.Printf("✅ [CASHBANK SERVICE] Cash bank record created: ID=%d, Code=%s", cashBank.ID, code)
			break
		}

		// Check if it's a duplicate code error
		errMsg := createErr.Error()
		if strings.Contains(errMsg, "uni_cash_banks_code") || strings.Contains(errMsg, "duplicate key") || strings.Contains(errMsg, "23505") {
			log.Printf("⚠️ [CASHBANK SERVICE] Duplicate code detected: %s (attempt %d), rolling back to savepoint and retrying...", code, retry+1)

			// Rollback to savepoint to clear the error state
			if rbErr := tx.RollbackTo(savePointName).Error; rbErr != nil {
				log.Printf("❌ [CASHBANK SERVICE] Failed to rollback to savepoint: %v", rbErr)
				tx.Rollback() // Fatal error, rollback entire transaction
				return nil, fmt.Errorf("failed to rollback to savepoint: %v", rbErr)
			}

			// Add small delay to reduce collision probability
			time.Sleep(time.Millisecond * time.Duration(10*(retry+1)))
			continue
		}

		// Other errors: abort immediately
		log.Printf("❌ [CASHBANK SERVICE] Cash bank creation failed: %v", createErr)
		tx.Rollback()
		return nil, fmt.Errorf("cash bank record creation failed: %v", createErr)
	}

	if createErr != nil {
		tx.Rollback()
		log.Printf("❌ [CASHBANK SERVICE] Failed to generate unique cash-bank code after 10 retries")
		return nil, fmt.Errorf("cash bank record creation failed: failed to generate unique code after 10 retries: %w", createErr)
	}

	// Create opening balance transaction if provided
	if request.OpeningBalance > 0 {
		log.Printf("💰 [CASHBANK SERVICE] Step 4: Creating opening balance transaction: Amount=%.2f", request.OpeningBalance)
		openingDate := request.OpeningDate.ToTime()
		if openingDate.IsZero() {
			openingDate = time.Now() // Use current time if no date provided
		}
		openingErr := s.createOpeningBalanceTransaction(tx, cashBank, request.OpeningBalance, openingDate, userID, request.SourceAccountID)
		if openingErr != nil {
			tx.Rollback()
			log.Printf("❌ [CASHBANK SERVICE] Opening balance creation failed: %v", openingErr)
			return nil, fmt.Errorf("opening balance creation failed: %v", openingErr)
		}
		log.Printf("✅ [CASHBANK SERVICE] Opening balance transaction created successfully")
	} else {
		log.Printf("ℹ️ [CASHBANK SERVICE] No opening balance specified, skipping opening balance transaction")
	}

	// Commit transaction
	log.Printf("💾 [CASHBANK SERVICE] Step 5: Committing transaction")
	if commitErr := tx.Commit().Error; commitErr != nil {
		log.Printf("❌ [CASHBANK SERVICE] Transaction commit failed: %v", commitErr)
		return nil, fmt.Errorf("transaction commit failed: %v", commitErr)
	}

	log.Printf("🎉 [CASHBANK SERVICE] Cash bank account created successfully: ID=%d, Code=%s, Name=%s, Balance=%.2f",
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

	updatedAccount, err := s.cashBankRepo.Update(cashBank)
	if err != nil {
		log.Printf("[CASHBANK SERVICE UPDATE] ❌ Failed to update: %v", err)
		return nil, err
	}

	log.Printf("[CASHBANK SERVICE UPDATE] ✅ SUCCESS: ID=%d, Final AccountHolderName='%s', Branch='%s'",
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

// ProcessTransfer processes transfer between cash/bank accounts
func (s *CashBankService) ProcessTransfer(request TransferRequest, userID uint) (*CashBankTransfer, error) {
	// Start transaction
	tx := s.db.Begin()

	// Validate source account
	sourceAccount, err := s.cashBankRepo.FindByID(request.FromAccountID)
	if err != nil {
		tx.Rollback()
		return nil, errors.New("source account not found")
	}

	// Validate destination account
	destAccount, err := s.cashBankRepo.FindByID(request.ToAccountID)
	if err != nil {
		tx.Rollback()
		return nil, errors.New("destination account not found")
	}

	// Ensure account integrity for both source and destination accounts
	if err := database.EnsureCashBankAccountIntegrity(s.db, sourceAccount.ID); err != nil {
		log.Printf("Warning: Failed to ensure source account integrity for ID %d: %v", sourceAccount.ID, err)
		// Reload source account after integrity fix
		sourceAccount, err = s.cashBankRepo.FindByID(request.FromAccountID)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to reload source account after integrity fix: %v", err)
		}
	}

	if err := database.EnsureCashBankAccountIntegrity(s.db, destAccount.ID); err != nil {
		log.Printf("Warning: Failed to ensure destination account integrity for ID %d: %v", destAccount.ID, err)
		// Reload destination account after integrity fix
		destAccount, err = s.cashBankRepo.FindByID(request.ToAccountID)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to reload destination account after integrity fix: %v", err)
		}
	}

	// Check balance
	if sourceAccount.Balance < request.Amount {
		tx.Rollback()
		return nil, fmt.Errorf("insufficient balance. Available: %.2f", sourceAccount.Balance)
	}

	// Apply exchange rate if different currencies
	transferAmount := request.Amount
	if sourceAccount.Currency != destAccount.Currency {
		if request.ExchangeRate <= 0 {
			tx.Rollback()
			return nil, errors.New("exchange rate required for different currencies")
		}
		transferAmount = request.Amount * request.ExchangeRate
	}

	// Create transfer record with retry mechanism for unique constraint violations
	var transfer *CashBankTransfer
	for retryAttempt := 0; retryAttempt < 5; retryAttempt++ {
		transfer = &CashBankTransfer{
			TransferNumber:  s.generateTransferNumber(),
			FromAccountID:   request.FromAccountID,
			ToAccountID:     request.ToAccountID,
			Date:            request.Date.ToTimeWithCurrentTime(),
			Amount:          request.Amount,
			ExchangeRate:    request.ExchangeRate,
			ConvertedAmount: transferAmount,
			Reference:       request.Reference,
			Notes:           request.Notes,
			Status:          "COMPLETED",
			UserID:          userID,
		}

		if err := tx.Create(transfer).Error; err != nil {
			// Check if it's a duplicate key constraint violation
			if strings.Contains(err.Error(), "duplicate key value violates unique constraint") &&
				strings.Contains(err.Error(), "transfer_number_key") {
				log.Printf("⚠️ Transfer number collision detected (attempt %d), retrying...", retryAttempt+1)
				time.Sleep(time.Millisecond * time.Duration(50*(retryAttempt+1)))
				continue // Retry with a new transfer number
			}
			// Other errors should not be retried
			tx.Rollback()
			return nil, fmt.Errorf("failed to create transfer record: %v", err)
		}
		// Success, break out of retry loop
		log.Printf("✅ Transfer created successfully with number: %s", transfer.TransferNumber)
		break
	}

	// If we've exhausted all retries
	if transfer == nil || transfer.ID == 0 {
		tx.Rollback()
		return nil, errors.New("failed to create transfer after multiple attempts")
	}

	// Update source account balance (essential since SSOT skips cash-bank accounts)
	originalSourceBalance := sourceAccount.Balance
	sourceAccount.Balance -= request.Amount
	if err := tx.Save(sourceAccount).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update source account balance: %v", err)
	}
	log.Printf("✅ Source account balance updated from %.2f to %.2f (transfer out: %.2f)",
		originalSourceBalance, sourceAccount.Balance, request.Amount)

	// Sync linked COA (GL) account balance for source after transfer out
	if sourceAccount.AccountID > 0 {
		if err := tx.Exec("UPDATE accounts SET balance = ? , updated_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL", sourceAccount.Balance, sourceAccount.AccountID).Error; err != nil {
			log.Printf("⚠️ Warning: Failed to sync COA balance for source GL account %d: %v", sourceAccount.AccountID, err)
		} else {
			log.Printf("🔄 Synced COA balance for source GL account %d to %.2f", sourceAccount.AccountID, sourceAccount.Balance)
		}
	}

	// Create source transaction record
	sourceTx := &models.CashBankTransaction{
		CashBankID:      request.FromAccountID,
		ReferenceType:   "TRANSFER",
		ReferenceID:     transfer.ID,
		Amount:          -request.Amount,
		BalanceAfter:    sourceAccount.Balance,
		TransactionDate: request.Date.ToTimeWithCurrentTime(),
		Notes:           fmt.Sprintf("Transfer to %s", destAccount.Name),
	}

	if err := tx.Create(sourceTx).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Update destination account balance (essential since SSOT skips cash-bank accounts)
	originalDestBalance := destAccount.Balance
	destAccount.Balance += transferAmount
	if err := tx.Save(destAccount).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update destination account balance: %v", err)
	}
	log.Printf("✅ Destination account balance updated from %.2f to %.2f (transfer in: %.2f)",
		originalDestBalance, destAccount.Balance, transferAmount)

	// Sync linked COA (GL) account balance for destination after transfer in
	if destAccount.AccountID > 0 {
		if err := tx.Exec("UPDATE accounts SET balance = ? , updated_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL", destAccount.Balance, destAccount.AccountID).Error; err != nil {
			log.Printf("⚠️ Warning: Failed to sync COA balance for destination GL account %d: %v", destAccount.AccountID, err)
		} else {
			log.Printf("🔄 Synced COA balance for destination GL account %d to %.2f", destAccount.AccountID, destAccount.Balance)
		}
	}

	// Create destination transaction record
	destTx := &models.CashBankTransaction{
		CashBankID:      request.ToAccountID,
		ReferenceType:   "TRANSFER",
		ReferenceID:     transfer.ID,
		Amount:          transferAmount,
		BalanceAfter:    destAccount.Balance,
		TransactionDate: request.Date.ToTimeWithCurrentTime(),
		Notes:           fmt.Sprintf("Transfer from %s", sourceAccount.Name),
	}

	if err := tx.Create(destTx).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Create SSOT journal entries using adapter
	journalRequest := &CashBankJournalRequest{
		TransactionType: TransactionTypeTransfer,
		Amount:          decimal.NewFromFloat(request.Amount),
		Date:            request.Date.ToTimeWithCurrentTime(),
		Reference:       request.Reference,
		Description:     fmt.Sprintf("Transfer from %s to %s", sourceAccount.Name, destAccount.Name),
		Notes:           request.Notes,
		FromCashBankID:  func() *uint64 { id := uint64(sourceAccount.ID); return &id }(),
		ToCashBankID:    func() *uint64 { id := uint64(destAccount.ID); return &id }(),
		CreatedBy:       uint64(userID),
	}

	_, err = s.sSOTJournalAdapter.CreateTransferJournalEntryWithTx(
		tx, sourceAccount, destAccount, sourceTx, journalRequest)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create SSOT transfer journal entry: %v", err)
	}

	return transfer, tx.Commit().Error
}

// ProcessDeposit processes a deposit transaction
func (s *CashBankService) ProcessDeposit(request DepositRequest, userID uint) (*models.CashBankTransaction, error) {
	log.Printf("🔄 Processing deposit: AccountID=%d, Amount=%.2f, UserID=%d",
		request.AccountID, request.Amount, userID)

	tx := s.db.Begin()

	// Validate account
	log.Printf("🔍 Step 1: Finding cash-bank account %d", request.AccountID)
	account, err := s.cashBankRepo.FindByID(request.AccountID)
	if err != nil {
		log.Printf("❌ Account %d not found: %v", request.AccountID, err)
		tx.Rollback()
		return nil, errors.New("account not found")
	}
	log.Printf("✅ Found account: %s (Balance: %.2f)", account.Name, account.Balance)

	// Ensure account integrity
	if err := database.EnsureCashBankAccountIntegrity(s.db, account.ID); err != nil {
		log.Printf("Warning: Failed to ensure account integrity for deposit ID %d: %v", account.ID, err)
		// Reload account after integrity fix
		account, err = s.cashBankRepo.FindByID(request.AccountID)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to reload account after integrity fix: %v", err)
		}
	}

	// Update cash bank balance (essential since SSOT skips cash-bank accounts)
	originalBalance := account.Balance
	account.Balance += request.Amount
	if err := tx.Save(account).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update cash bank balance: %v", err)
	}
	log.Printf("✅ Step 2: Updated cash bank balance from %.2f to %.2f (deposit: %.2f)",
		originalBalance, account.Balance, request.Amount)

	// Sync linked COA (GL) account balance to maintain COA <> CashBank consistency
	// UnifiedJournalService intentionally skips cash bank GL balance updates to avoid double posting,
	// so we ensure COA balance mirrors the cash_banks.balance here.
	if account.AccountID > 0 {
		if err := tx.Exec("UPDATE accounts SET balance = ? , updated_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL", account.Balance, account.AccountID).Error; err != nil {
			log.Printf("⚠️ Warning: Failed to sync COA balance for GL account %d: %v", account.AccountID, err)
		} else {
			log.Printf("🔄 Synced COA balance for GL account %d to %.2f", account.AccountID, account.Balance)
		}
	}

	// Create transaction record
	transaction := &models.CashBankTransaction{
		CashBankID:      request.AccountID,
		ReferenceType:   TransactionTypeDeposit,
		ReferenceID:     0, // No specific reference for direct deposit
		Amount:          request.Amount,
		BalanceAfter:    account.Balance,
		TransactionDate: request.Date.ToTimeWithCurrentTime(),
		Notes:           request.Notes,
	}

	if err := tx.Create(transaction).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Create SSOT journal entries using adapter
	log.Printf("📋 Step 3: Creating SSOT journal entry for deposit")
	journalRequest := &CashBankJournalRequest{
		TransactionType: TransactionTypeDeposit,
		CashBankID:      uint64(request.AccountID),
		Amount:          decimal.NewFromFloat(request.Amount),
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

	log.Printf("📋 Creating SSOT journal entry...")
	journalResult, err := s.sSOTJournalAdapter.CreateDepositJournalEntryWithTx(
		tx, account, transaction, journalRequest)
	if err != nil {
		log.Printf("❌ SSOT journal creation failed: %v", err)
		tx.Rollback()
		return nil, fmt.Errorf("failed to create SSOT deposit journal entry: %v", err)
	}
	log.Printf("✅ SSOT journal created: %s", journalResult.JournalEntry.EntryNumber)

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("❌ Transaction commit failed: %v", err)
		return nil, fmt.Errorf("failed to commit deposit transaction: %v", err)
	}

	log.Printf("🎉 Deposit completed successfully: TransactionID=%d, FinalBalance=%.2f",
		transaction.ID, account.Balance)

	return transaction, nil
}

// ProcessWithdrawal processes a withdrawal transaction
func (s *CashBankService) ProcessWithdrawal(request WithdrawalRequest, userID uint) (*models.CashBankTransaction, error) {
	log.Printf("🔄 Processing withdrawal: AccountID=%d, Amount=%.2f, UserID=%d",
		request.AccountID, request.Amount, userID)

	tx := s.db.Begin()

	// Validate account
	log.Printf("🔍 Step 1: Finding cash-bank account %d", request.AccountID)
	account, err := s.cashBankRepo.FindByID(request.AccountID)
	if err != nil {
		log.Printf("❌ Account %d not found: %v", request.AccountID, err)
		tx.Rollback()
		return nil, errors.New("account not found")
	}
	log.Printf("✅ Found account: %s (Balance: %.2f)", account.Name, account.Balance)

	// Ensure account integrity
	if err := database.EnsureCashBankAccountIntegrity(s.db, account.ID); err != nil {
		log.Printf("Warning: Failed to ensure account integrity for withdrawal ID %d: %v", account.ID, err)
		// Reload account after integrity fix
		account, err = s.cashBankRepo.FindByID(request.AccountID)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to reload account after integrity fix: %v", err)
		}
	}

	// Check balance
	log.Printf("💰 Step 2: Checking balance %.2f vs requested %.2f", account.Balance, request.Amount)
	if account.Balance < request.Amount {
		log.Printf("❌ Insufficient balance: %.2f < %.2f", account.Balance, request.Amount)
		tx.Rollback()
		return nil, fmt.Errorf("insufficient balance. Available: %.2f", account.Balance)
	}

	// Update cash bank balance (essential since SSOT skips cash-bank accounts)
	originalBalance := account.Balance
	account.Balance -= request.Amount
	if err := tx.Save(account).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update cash bank balance: %v", err)
	}
	log.Printf("✅ Step 3: Updated cash bank balance from %.2f to %.2f (withdrawal: %.2f)",
		originalBalance, account.Balance, request.Amount)

	// Sync linked COA (GL) account balance after withdrawal
	if account.AccountID > 0 {
		if err := tx.Exec("UPDATE accounts SET balance = ? , updated_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL", account.Balance, account.AccountID).Error; err != nil {
			log.Printf("⚠️ Warning: Failed to sync COA balance for GL account %d: %v", account.AccountID, err)
		} else {
			log.Printf("🔄 Synced COA balance for GL account %d to %.2f", account.AccountID, account.Balance)
		}
	}

	// Create transaction record
	transaction := &models.CashBankTransaction{
		CashBankID:      request.AccountID,
		ReferenceType:   TransactionTypeWithdrawal,
		ReferenceID:     0,
		Amount:          -request.Amount,
		BalanceAfter:    account.Balance,
		TransactionDate: request.Date.ToTimeWithCurrentTime(),
		Notes:           request.Notes,
	}

	if err := tx.Create(transaction).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Create SSOT journal entries using adapter
	log.Printf("📋 Step 4: Creating SSOT journal entry for withdrawal")
	journalRequest := &CashBankJournalRequest{
		TransactionType: TransactionTypeWithdrawal,
		CashBankID:      uint64(request.AccountID),
		Amount:          decimal.NewFromFloat(request.Amount),
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

	log.Printf("📋 Creating SSOT journal entry...")
	_, err = s.sSOTJournalAdapter.CreateWithdrawalJournalEntryWithTx(
		tx, account, transaction, journalRequest)
	if err != nil {
		log.Printf("❌ SSOT journal creation failed: %v", err)
		tx.Rollback()
		return nil, fmt.Errorf("failed to create SSOT withdrawal journal entry: %v", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("❌ Transaction commit failed: %v", err)
		return nil, fmt.Errorf("failed to commit withdrawal transaction: %v", err)
	}

	log.Printf("🎉 Withdrawal completed successfully: TransactionID=%d, FinalBalance=%.2f",
		transaction.ID, account.Balance)

	return transaction, nil
}

// GetTransactions retrieves transactions for a cash/bank account
func (s *CashBankService) GetTransactions(accountID uint, filter TransactionFilter) (*TransactionResult, error) {
	// Convert service filter to repository filter
	repoFilter := repositories.TransactionFilter{
		StartDate: filter.StartDate,
		EndDate:   filter.EndDate,
		Type:      filter.Type,
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
	// Use the repository method which correctly handles active accounts
	// and avoids double counting issues
	repoSummary, err := s.cashBankRepo.GetBalanceSummary()
	if err != nil {
		return nil, err
	}

	// Get individual account details for ByAccount field
	accounts, err := s.cashBankRepo.FindAll()
	if err != nil {
		return nil, err
	}

	// Convert to service format
	summary := &BalanceSummary{
		TotalCash:    repoSummary.TotalCash,
		TotalBank:    repoSummary.TotalBank,
		TotalBalance: repoSummary.TotalBalance,
		ByAccount:    []AccountBalance{},
		ByCurrency:   repoSummary.ByCurrency,
	}

	// Add individual account details (ONLY ACTIVE ACCOUNTS)
	for _, account := range accounts {
		if account.IsActive { // Only include active accounts
			summary.ByAccount = append(summary.ByAccount, AccountBalance{
				AccountID:   account.ID,
				AccountName: account.Name,
				AccountType: account.Type,
				Balance:     account.Balance,
				Currency:    account.Currency,
			})
		}
	}

	return summary, nil
}

// GetPaymentAccounts gets active cash and bank accounts for payment processing
func (s *CashBankService) GetPaymentAccounts() ([]models.CashBank, error) {
	accounts, err := s.cashBankRepo.FindAll()
	if err != nil {
		return nil, err
	}

	// Filter only active accounts
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
// ✅ FIX ROOT CAUSE: Ensures each cash/bank gets its own unique GL
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

	log.Printf("✅ Created parent account: %s - %s", parentAccount.Code, parentAccount.Name)
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

func (s *CashBankService) createOpeningBalanceTransaction(tx *gorm.DB, cashBank *models.CashBank, amount float64, date time.Time, userID uint, sourceAccountID *uint) error {
	log.Printf("🔄 [OPENING BALANCE] Creating opening balance transaction: CashBankID=%d, Amount=%.2f, Date=%s, UserID=%d",
		cashBank.ID, amount, date.Format("2006-01-02"), userID)

	// Step 1: Update cash bank balance
	log.Printf("💰 [OPENING BALANCE] Step 1: Updating cash bank balance to %.2f", amount)
	cashBank.Balance = amount
	if err := tx.Save(cashBank).Error; err != nil {
		log.Printf("❌ [OPENING BALANCE] Failed to update cash bank balance: %v", err)
		return fmt.Errorf("failed to update cash bank balance: %v", err)
	}
	log.Printf("✅ [OPENING BALANCE] Cash bank balance updated successfully")

	// Step 2: Create transaction record
	log.Printf("📝 [OPENING BALANCE] Step 2: Creating transaction record")
	transaction := &models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   TransactionTypeOpeningBalance,
		ReferenceID:     0,
		Amount:          amount,
		BalanceAfter:    amount,
		TransactionDate: date,
		Notes:           "Opening Balance",
	}

	if err := tx.Create(transaction).Error; err != nil {
		log.Printf("❌ [OPENING BALANCE] Failed to create transaction record: %v", err)
		return fmt.Errorf("failed to create transaction record: %v", err)
	}
	log.Printf("✅ [OPENING BALANCE] Transaction record created: ID=%d", transaction.ID)

	// Step 3: Create SSOT journal entries for opening balance
	log.Printf("📋 [OPENING BALANCE] Step 3: Creating SSOT journal entry")
	journalRequest := &CashBankJournalRequest{
		TransactionType: TransactionTypeOpeningBalance,
		CashBankID:      uint64(cashBank.ID),
		Amount:          decimal.NewFromFloat(amount),
		Date:            date,
		Reference:       fmt.Sprintf("OB-%s", cashBank.Code),
		Description:     fmt.Sprintf("Opening balance for %s", cashBank.Name),
		Notes:           "Opening Balance",

		CreatedBy: uint64(userID),
		CounterAccountID: func() *uint64 {
			if sourceAccountID != nil {
				id := uint64(*sourceAccountID)
				return &id
			}
			return nil
		}(),
	}

	journalEntry, err := s.sSOTJournalAdapter.CreateOpeningBalanceJournalEntryWithTx(
		tx, cashBank, transaction, journalRequest)
	if err != nil {
		log.Printf("❌ [OPENING BALANCE] SSOT journal entry creation failed: %v", err)
		return fmt.Errorf("SSOT journal entry creation failed: %v", err)
	}

	if journalEntry != nil {
		log.Printf("✅ [OPENING BALANCE] SSOT journal entry created: ID=%d, EntryNumber=%s",
			journalEntry.ID, journalEntry.EntryNumber)
	} else {
		log.Printf("✅ [OPENING BALANCE] SSOT journal entry created successfully")
	}

	log.Printf("🎉 [OPENING BALANCE] Opening balance transaction completed successfully")
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
	Name              string     `json:"name" binding:"required"`
	Type              string     `json:"type" binding:"required,oneof=CASH BANK"`
	AccountID         uint       `json:"account_id"`
	BankName          string     `json:"bank_name"`
	AccountNo         string     `json:"account_no"`
	AccountHolderName string     `json:"account_holder_name"`
	Branch            string     `json:"branch"`
	Currency          string     `json:"currency"`
	OpeningBalance    float64    `json:"opening_balance"`
	OpeningDate       CustomDate `json:"opening_date"`
	Description       string     `json:"description"`
	SourceAccountID   *uint      `json:"source_account_id"` // Optional: Equity account for opening balance
}

type CashBankUpdateRequest struct {
	Name              string `json:"name"`
	BankName          string `json:"bank_name"`
	AccountNo         string `json:"account_no"`
	AccountHolderName string `json:"account_holder_name"`
	Branch            string `json:"branch"`
	Description       string `json:"description"`
	IsActive          *bool  `json:"is_active"`
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
	ID              uint   `gorm:"primaryKey"`
	TransferNumber  string `gorm:"unique;not null;size:50"`
	FromAccountID   uint   `gorm:"not null;index"`
	ToAccountID     uint   `gorm:"not null;index"`
	Date            time.Time
	Amount          float64 `gorm:"type:decimal(15,2)"`
	ExchangeRate    float64 `gorm:"type:decimal(12,6);default:1"`
	ConvertedAmount float64 `gorm:"type:decimal(15,2)"`
	Reference       string  `gorm:"size:100"`
	Notes           string  `gorm:"type:text"`
	Status          string  `gorm:"size:20"`
	UserID          uint    `gorm:"not null;index"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
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
	log.Printf("🔍 [CASHBANK SERVICE] Getting available GL accounts for type: %s", accountType)

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
		log.Printf("❌ [CASHBANK SERVICE] Failed to get available GL accounts: %v", err)
		return nil, fmt.Errorf("failed to get available GL accounts: %v", err)
	}

	log.Printf("✅ [CASHBANK SERVICE] Found %d available GL accounts for type %s", len(accounts), accountType)
	return accounts, nil
}
