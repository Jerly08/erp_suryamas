package services

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"context"
	"fmt"
	"gorm.io/gorm"
	"strings"
)

// CashBankEnhancedService wraps existing CashBankService with new accounting integration
type CashBankEnhancedService struct {
	*CashBankService  // Embed existing service
	accountingService *CashBankAccountingService
	validationService *CashBankValidationService
}

func NewCashBankEnhancedService(
	db *gorm.DB,
	cashBankRepo *repositories.CashBankRepository,
	accountRepo repositories.AccountRepository,
) *CashBankEnhancedService {
	// Create base service
	baseService := NewCashBankService(db, cashBankRepo, accountRepo)

	// Create accounting service
	accountingService := NewCashBankAccountingService(db)

	// Create validation service
	validationService := NewCashBankValidationService(db, accountingService)

	return &CashBankEnhancedService{
		CashBankService:   baseService,
		accountingService: accountingService,
		validationService: validationService,
	}
}

// ProcessDepositV2 uses the new accounting service for automatic journal entries
func (s *CashBankEnhancedService) ProcessDepositV2(request DepositRequest, userID uint, sourceAccountID uint) (*models.CashBankTransaction, error) {
	// Validate that the cash bank account is linked to COA
	integrity, err := s.validationService.ValidateCashBankIntegrity(request.AccountID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate cash bank integrity: %v", err)
	}

	if integrity.Issue == "NOT_LINKED" {
		return nil, fmt.Errorf("cash bank account must be linked to COA before processing transactions")
	}

	// Use new accounting service for automatic journal entries and balance sync
	ctx := context.Background() // Create a background context
	err = s.accountingService.ProcessCashBankDeposit(
		ctx,
		request.AccountID,
		request.Amount,
		sourceAccountID,
		models.JournalRefDeposit,
		0, // No specific reference for direct deposit
		request.Notes,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to process deposit: %v", err)
	}

	// Get the latest transaction that was created
	var transaction models.CashBankTransaction
	err = s.db.Where("cash_bank_id = ? AND reference_type = ? AND amount = ?",
		request.AccountID, models.JournalRefDeposit, request.Amount).
		Order("created_at DESC").First(&transaction).Error

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve created transaction: %v", err)
	}

	return &transaction, nil
}

// ProcessWithdrawalV2 is retained for legacy callers. Direct generic
// withdrawals are no longer allowed, so this method delegates to the
// policy-enforcing CashBankService implementation instead of using the lower
// level accounting withdrawal helper (which remains valid for payment flows).
//
// legacyTargetAccountID is accepted only to preserve older OWNER_DRAWING
// callers that supplied the counter account as the third argument. The normal
// request.TargetAccountID field takes precedence when it is supplied.
func (s *CashBankEnhancedService) ProcessWithdrawalV2(request WithdrawalRequest, userID uint, legacyTargetAccountID uint) (*models.CashBankTransaction, error) {
	if request.TargetAccountID == nil && legacyTargetAccountID > 0 && strings.EqualFold(strings.TrimSpace(request.Purpose), WithdrawalPurposeOwnerDrawing) {
		targetAccountID := legacyTargetAccountID
		request.TargetAccountID = &targetAccountID
	}

	return s.CashBankService.ProcessWithdrawal(request, userID)
}

// ProcessTransferV2 uses the new accounting service for automatic journal entries
func (s *CashBankEnhancedService) ProcessTransferV2(request TransferRequest, userID uint) error {
	// Validate both accounts are linked to COA
	sourceIntegrity, err := s.validationService.ValidateCashBankIntegrity(request.FromAccountID)
	if err != nil {
		return fmt.Errorf("failed to validate source account integrity: %v", err)
	}

	destIntegrity, err := s.validationService.ValidateCashBankIntegrity(request.ToAccountID)
	if err != nil {
		return fmt.Errorf("failed to validate destination account integrity: %v", err)
	}

	if sourceIntegrity.Issue == "NOT_LINKED" || destIntegrity.Issue == "NOT_LINKED" {
		return fmt.Errorf("both cash bank accounts must be linked to COA before processing transfer")
	}

	// Check balance
	if sourceIntegrity.CashBankBalance < request.Amount {
		return fmt.Errorf("insufficient balance. Available: %.2f", sourceIntegrity.CashBankBalance)
	}

	// Use new accounting service for automatic transfer
	ctx := context.Background() // Create a background context
	err = s.accountingService.ProcessCashBankTransfer(
		ctx,
		request.FromAccountID,
		request.ToAccountID,
		request.Amount,
		request.Notes,
	)

	if err != nil {
		return fmt.Errorf("failed to process transfer: %v", err)
	}

	return nil
}

// AutoFixSyncIssues fixes any synchronization issues found
func (s *CashBankEnhancedService) AutoFixSyncIssues() (int, error) {
	return s.validationService.AutoFixDiscrepancies()
}

// GetSyncStatus returns current sync status for monitoring
func (s *CashBankEnhancedService) GetSyncStatus() (map[string]interface{}, error) {
	return s.validationService.GetSyncStatus()
}

// LinkCashBankToAccount links a cash bank to a COA account
func (s *CashBankEnhancedService) LinkCashBankToAccount(cashBankID, accountID uint) error {
	return s.validationService.LinkCashBankToAccount(cashBankID, accountID)
}

// UnlinkCashBankFromAccount unlinks a cash bank from its COA account
func (s *CashBankEnhancedService) UnlinkCashBankFromAccount(cashBankID uint) error {
	return s.validationService.UnlinkCashBankFromAccount(cashBankID)
}

// RecalculateBalance recalculates cash bank balance from transactions
func (s *CashBankEnhancedService) RecalculateBalance(cashBankID uint) error {
	return s.accountingService.RecalculateCashBankBalance(cashBankID)
}

// SyncAllBalances syncs all cash bank balances with COA
func (s *CashBankEnhancedService) SyncAllBalances() error {
	return s.accountingService.SyncAllCashBankBalances()
}

// ProcessPayment processes payment transaction with automatic journal entries
func (s *CashBankEnhancedService) ProcessPayment(cashBankID uint, paymentID uint, amount float64, payableAccountID uint, notes string) error {
	// Validate cash bank is linked
	integrity, err := s.validationService.ValidateCashBankIntegrity(cashBankID)
	if err != nil {
		return fmt.Errorf("failed to validate cash bank integrity: %v", err)
	}

	if integrity.Issue == "NOT_LINKED" {
		return fmt.Errorf("cash bank account must be linked to COA before processing payment")
	}

	// Check balance
	if integrity.CashBankBalance < amount {
		return fmt.Errorf("insufficient balance. Available: %.2f", integrity.CashBankBalance)
	}

	// Process payment as withdrawal to payable account
	ctx := context.Background() // Create a background context
	return s.accountingService.ProcessCashBankWithdrawal(
		ctx,
		cashBankID,
		amount,
		payableAccountID,
		models.JournalRefPayment,
		paymentID,
		notes,
	)
}

// ProcessReceipt processes receipt transaction with automatic journal entries
func (s *CashBankEnhancedService) ProcessReceipt(cashBankID uint, receiptID uint, amount float64, receivableAccountID uint, notes string) error {
	// Validate cash bank is linked
	integrity, err := s.validationService.ValidateCashBankIntegrity(cashBankID)
	if err != nil {
		return fmt.Errorf("failed to validate cash bank integrity: %v", err)
	}

	if integrity.Issue == "NOT_LINKED" {
		return fmt.Errorf("cash bank account must be linked to COA before processing receipt")
	}

	// Process receipt as deposit from receivable account
	ctx := context.Background() // Create a background context
	return s.accountingService.ProcessCashBankDeposit(
		ctx,
		cashBankID,
		amount,
		receivableAccountID,
		"RECEIPT",
		receiptID,
		notes,
	)
}

// CreateCashBankAccountV2 creates cash bank account with automatic COA linking
func (s *CashBankEnhancedService) CreateCashBankAccountV2(request CashBankCreateRequest, userID uint) (*models.CashBank, error) {
	// Create using base service first
	cashBank, err := s.CashBankService.CreateCashBankAccount(request, userID)
	if err != nil {
		return nil, err
	}

	// Ensure it's properly linked and synced
	if cashBank.AccountID > 0 {
		err = s.validationService.LinkCashBankToAccount(cashBank.ID, cashBank.AccountID)
		if err != nil {
			// Log warning but don't fail
			fmt.Printf("Warning: Failed to ensure COA linking for new cash bank %d: %v\n", cashBank.ID, err)
		}
	}

	return cashBank, nil
}

// ValidateAndFixCashBank validates and fixes a specific cash bank account
func (s *CashBankEnhancedService) ValidateAndFixCashBank(cashBankID uint) (*SyncDiscrepancy, error) {
	// Get current status
	integrity, err := s.validationService.ValidateCashBankIntegrity(cashBankID)
	if err != nil {
		return nil, err
	}

	// Try to auto-fix if needed
	if integrity.Issue != "SYNC_OK" {
		switch integrity.Issue {
		case "BALANCE_MISMATCH":
			err = s.validationService.fixBalanceMismatch(cashBankID, integrity.COAAccountID, integrity.TransactionSum)
		case "TRANSACTION_SUM_MISMATCH":
			err = s.accountingService.RecalculateCashBankBalance(cashBankID)
		}

		if err != nil {
			return integrity, fmt.Errorf("failed to fix issue: %v", err)
		}

		// Re-validate after fix
		integrity, err = s.validationService.ValidateCashBankIntegrity(cashBankID)
		if err != nil {
			return nil, err
		}
	}

	return integrity, nil
}

// GetHealthStatus returns detailed health status for monitoring
func (s *CashBankEnhancedService) GetHealthStatus() (map[string]interface{}, error) {
	status, err := s.validationService.GetSyncStatus()
	if err != nil {
		return nil, err
	}

	// Add additional health metrics
	healthStatus := make(map[string]interface{})
	healthStatus["sync_status"] = status

	// Check for any unlinked accounts
	unlinkedCount := 0
	if issueBreakdown, ok := status["issue_breakdown"].(map[string]int); ok {
		if count, exists := issueBreakdown["NOT_LINKED"]; exists {
			unlinkedCount = count
		}
	}

	healthStatus["needs_attention"] = unlinkedCount > 0 || status["status"] != "healthy"
	healthStatus["recommendations"] = s.getRecommendations(status)

	return healthStatus, nil
}

// getRecommendations provides recommendations based on sync status
func (s *CashBankEnhancedService) getRecommendations(status map[string]interface{}) []string {
	var recommendations []string

	if issueBreakdown, ok := status["issue_breakdown"].(map[string]int); ok {
		if count, exists := issueBreakdown["NOT_LINKED"]; exists && count > 0 {
			recommendations = append(recommendations,
				fmt.Sprintf("Link %d unlinked cash/bank accounts to COA accounts", count))
		}

		if count, exists := issueBreakdown["BALANCE_MISMATCH"]; exists && count > 0 {
			recommendations = append(recommendations,
				fmt.Sprintf("Fix %d balance mismatches using AutoFixSyncIssues()", count))
		}

		if count, exists := issueBreakdown["TRANSACTION_SUM_MISMATCH"]; exists && count > 0 {
			recommendations = append(recommendations,
				fmt.Sprintf("Recalculate %d accounts with transaction sum mismatches", count))
		}
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "All cash/bank accounts are properly synchronized")
	}

	return recommendations
}

// Middleware function to validate sync before operations
func (s *CashBankEnhancedService) ValidateSyncMiddleware() error {
	discrepancies, err := s.validationService.FindSyncDiscrepancies()
	if err != nil {
		return fmt.Errorf("sync validation failed: %v", err)
	}

	// Count critical issues (excluding NOT_LINKED which is a setup issue)
	criticalCount := 0
	for _, d := range discrepancies {
		if d.Issue == "BALANCE_MISMATCH" || d.Issue == "TRANSACTION_SUM_MISMATCH" {
			criticalCount++
		}
	}

	if criticalCount > 0 {
		return fmt.Errorf("found %d critical sync issues that must be fixed before proceeding", criticalCount)
	}

	return nil
}
