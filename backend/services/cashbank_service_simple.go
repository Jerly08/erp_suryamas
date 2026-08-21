package services

import (
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"
	"errors"
	"fmt"
	"log"
)

// ProcessDepositSimple processes a deposit transaction WITHOUT SSOT journal creation
// Use this as a fallback when SSOT journal creation causes timeout
func (s *CashBankService) ProcessDepositSimple(request DepositRequest, userID uint) (*models.CashBankTransaction, error) {
	log.Printf("🔄 [SIMPLE MODE] Processing deposit: AccountID=%d, Amount=%.2f, UserID=%d",
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

	// Update balance
	log.Printf("💰 Step 2: Updating balance from %.2f to %.2f", account.Balance, account.Balance+request.Amount)
	account.Balance += request.Amount
	if err := tx.Save(account).Error; err != nil {
		log.Printf("❌ Failed to update account balance: %v", err)
		tx.Rollback()
		return nil, err
	}

	// Create transaction record
	transaction := &models.CashBankTransaction{
		CashBankID:      request.AccountID,
		ReferenceType:   TransactionTypeDeposit,
		ReferenceID:     0, // No specific reference for direct deposit
		Amount:          request.Amount,
		BalanceAfter:    account.Balance,
		TransactionDate: request.Date.ToTime(),
		Notes:           fmt.Sprintf("[SIMPLE MODE] %s", request.Notes),
	}

	if err := tx.Create(transaction).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	log.Printf("⚠️ SKIPPING SSOT journal creation for instant processing")

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("❌ Transaction commit failed: %v", err)
		return nil, fmt.Errorf("failed to commit deposit transaction: %v", err)
	}

	log.Printf("🎉 Deposit completed successfully [SIMPLE MODE]: TransactionID=%d, FinalBalance=%.2f",
		transaction.ID, account.Balance)

	return transaction, nil
}

// ProcessWithdrawalSimple is retained for legacy callers, but no longer offers
// an unjournaled generic withdrawal path. It delegates to ProcessWithdrawal so
// the OWNER_DRAWING-only policy, authority checks, and SSOT journal are always
// applied.
func (s *CashBankService) ProcessWithdrawalSimple(request WithdrawalRequest, userID uint) (*models.CashBankTransaction, error) {
	log.Printf("[DEPRECATED] ProcessWithdrawalSimple delegates to the controlled withdrawal workflow: AccountID=%d, Amount=%.2f, UserID=%d", request.AccountID, request.Amount, userID)
	return s.ProcessWithdrawal(request, userID)
}
