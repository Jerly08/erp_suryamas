package services

import (
	"context"
	"fmt"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// CashBankIntegratedService handles integration between CashBank system and SSOT Journal system
type CashBankIntegratedService struct {
	db                    *gorm.DB
	cashBankService       *CashBankService
	unifiedJournalService *UnifiedJournalService
	accountRepo           repositories.AccountRepository
}

// NewCashBankIntegratedService creates a new integrated service
func NewCashBankIntegratedService(
	db *gorm.DB,
	cashBankService *CashBankService,
	unifiedJournalService *UnifiedJournalService,
	accountRepo repositories.AccountRepository,
) *CashBankIntegratedService {
	return &CashBankIntegratedService{
		db:                    db,
		cashBankService:       cashBankService,
		unifiedJournalService: unifiedJournalService,
		accountRepo:           accountRepo,
	}
}

// IntegratedAccountResponse represents response for integrated account details
type IntegratedAccountResponse struct {
	Account              *IntegratedAccountDetail `json:"account"`
	RecentTransactions   []IntegratedTransaction  `json:"recent_transactions"`
	RecentJournalEntries []IntegratedJournalEntry `json:"recent_journal_entries"`
	LastSyncedAt         *time.Time               `json:"last_synced_at"`
}

// IntegratedAccountDetail represents account with calculated fields
type IntegratedAccountDetail struct {
	ID                   uint            `json:"id"`
	AccountID            uint            `json:"account_id"`
	Name                 string          `json:"name"`
	Code                 string          `json:"code"`
	Type                 string          `json:"type"`
	Balance              decimal.Decimal `json:"balance"`
	SSOTBalance          decimal.Decimal `json:"ssot_balance"`
	Variance             decimal.Decimal `json:"variance"`
	ReconciliationStatus string          `json:"reconciliation_status"`
	LastTransactionDate  *time.Time      `json:"last_transaction_date"`
	TotalJournalEntries  int             `json:"total_journal_entries"`
	IsActive             bool            `json:"is_active"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

// IntegratedTransaction represents transaction with journal reference
type IntegratedTransaction struct {
	ID                 uint            `json:"id"`
	Amount             decimal.Decimal `json:"amount"`
	Type               string          `json:"type"`
	Number             string          `json:"number"`
	CreatedAt          time.Time       `json:"created_at"`
	Description        string          `json:"description"`
	BalanceAfter       decimal.Decimal `json:"balance_after"`
	ReferenceNumber    string          `json:"reference_number"`
	JournalEntryID     *uint64         `json:"journal_entry_id,omitempty"`
	JournalEntryNumber string          `json:"journal_entry_number,omitempty"`
}

// IntegratedJournalEntry represents journal entry with cash/bank context
type IntegratedJournalEntry struct {
	ID          uint64                  `json:"id"`
	EntryNumber string                  `json:"entry_number"`
	EntryDate   time.Time               `json:"entry_date"`
	Description string                  `json:"description"`
	SourceType  string                  `json:"source_type"`
	TotalDebit  decimal.Decimal         `json:"total_debit"`
	TotalCredit decimal.Decimal         `json:"total_credit"`
	Status      string                  `json:"status"`
	Lines       []IntegratedJournalLine `json:"lines"`
	CreatedAt   time.Time               `json:"created_at"`
}

// IntegratedJournalLine represents journal line with account details
type IntegratedJournalLine struct {
	ID           uint64          `json:"id"`
	AccountID    uint64          `json:"account_id"`
	AccountCode  string          `json:"account_code"`
	AccountName  string          `json:"account_name"`
	Description  string          `json:"description"`
	DebitAmount  decimal.Decimal `json:"debit_amount"`
	CreditAmount decimal.Decimal `json:"credit_amount"`
}

// IntegratedSummaryResponse represents summary of all cash/bank accounts with SSOT data
type IntegratedSummaryResponse struct {
	Summary          IntegratedBalanceSummary   `json:"summary"`
	Accounts         []IntegratedAccountSummary `json:"accounts"`
	RecentActivities []IntegratedActivity       `json:"recent_activities"`
	SyncStatus       SyncStatus                 `json:"sync_status"`
}

// IntegratedBalanceSummary represents overall balance summary
type IntegratedBalanceSummary struct {
	TotalCash        decimal.Decimal `json:"total_cash"`
	TotalBank        decimal.Decimal `json:"total_bank"`
	TotalBalance     decimal.Decimal `json:"total_balance"`
	TotalSSOTBalance decimal.Decimal `json:"total_ssot_balance"`
	BalanceVariance  decimal.Decimal `json:"balance_variance"`
	VarianceCount    int             `json:"variance_count"`
}

// IntegratedAccountSummary represents account summary with SSOT data
type IntegratedAccountSummary struct {
	ID                   uint            `json:"id"`
	Name                 string          `json:"name"`
	Code                 string          `json:"code"`
	Type                 string          `json:"type"`
	Balance              decimal.Decimal `json:"balance"`
	SSOTBalance          decimal.Decimal `json:"ssot_balance"`
	Variance             decimal.Decimal `json:"variance"`
	LastTransactionDate  *time.Time      `json:"last_transaction_date"`
	TotalJournalEntries  int             `json:"total_journal_entries"`
	ReconciliationStatus string          `json:"reconciliation_status"`
}

// IntegratedActivity represents recent activity across both systems
type IntegratedActivity struct {
	Type        string          `json:"type"`
	ID          uint64          `json:"id"`
	Number      string          `json:"number"`
	Description string          `json:"description"`
	Amount      decimal.Decimal `json:"amount"`
	AccountName string          `json:"account_name"`
	CreatedAt   time.Time       `json:"created_at"`
}

// SyncStatus represents synchronization status
type SyncStatus struct {
	LastSyncAt       *time.Time `json:"last_sync_at"`
	SyncStatus       string     `json:"sync_status"`
	TotalAccounts    int        `json:"total_accounts"`
	SyncedAccounts   int        `json:"synced_accounts"`
	VarianceAccounts int        `json:"variance_accounts"`
}

// Reconcile request/response types
type ReconcileRequest struct {
	Strategy   string `json:"strategy"` // to_ssot | to_transactions | to_coa
	DryRun     bool   `json:"dry_run"`
	AccountIDs []uint `json:"account_ids"`
}

type ReconcileItem struct {
	CashBankID     uint            `json:"cash_bank_id"`
	COAAccountID   uint            `json:"coa_account_id"`
	CashBankBefore decimal.Decimal `json:"cashbank_before"`
	COABefore      decimal.Decimal `json:"coa_before"`
	SSOTBalance    decimal.Decimal `json:"ssot_balance"`
	TransactionSum decimal.Decimal `json:"transaction_sum"`
	NewBalance     decimal.Decimal `json:"new_balance"`
	Applied        bool            `json:"applied"`
}

type ReconcileResult struct {
	Strategy string          `json:"strategy"`
	DryRun   bool            `json:"dry_run"`
	Items    []ReconcileItem `json:"items"`
}

// ReconciliationData represents detailed reconciliation information
type ReconciliationData struct {
	AccountID            uint            `json:"account_id"`
	AccountName          string          `json:"account_name"`
	CashBankBalance      decimal.Decimal `json:"cashbank_balance"`
	SSOTBalance          decimal.Decimal `json:"ssot_balance"`
	Difference           decimal.Decimal `json:"difference"`
	HasDiscrepancy       bool            `json:"has_discrepancy"`
	ReconciliationStatus string          `json:"reconciliation_status"`
	LastReconciledAt     time.Time       `json:"last_reconciled_at"`
	Details              []string        `json:"details"`
	Recommendations      []string        `json:"recommendations"`
}

// PaginationInfo represents pagination metadata
type PaginationInfo struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// JournalEntriesResponse represents paginated journal entries response
type JournalEntriesResponse struct {
	Entries    []IntegratedJournalEntry `json:"entries"`
	Pagination PaginationInfo           `json:"pagination"`
}

// TransactionHistoryResponse represents paginated transaction history response
type TransactionHistoryResponse struct {
	Transactions []IntegratedTransaction `json:"transactions"`
	Pagination   PaginationInfo          `json:"pagination"`
}

// CashBankJournalEntryDetail represents detailed journal entry for account detail page
type CashBankJournalEntryDetail struct {
	ID           uint64          `json:"id"`
	EntryID      uint64          `json:"entry_id"`
	LineID       uint64          `json:"line_id"`
	EntryDate    time.Time       `json:"entry_date"`
	Description  string          `json:"description"`
	DebitAmount  decimal.Decimal `json:"debit_amount"`
	CreditAmount decimal.Decimal `json:"credit_amount"`
	Status       string          `json:"status"`
}

// TransactionEntry represents transaction entry for account detail page
type TransactionEntry struct {
	ID              uint            `json:"id"`
	Number          string          `json:"number"`
	Type            string          `json:"type"`
	Amount          decimal.Decimal `json:"amount"`
	Description     string          `json:"description"`
	ReferenceNumber string          `json:"reference_number"`
	CreatedAt       time.Time       `json:"created_at"`
}

// GetIntegratedAccountDetails gets integrated account details with SSOT data
func (s *CashBankIntegratedService) GetIntegratedAccountDetails(accountID uint) (*IntegratedAccountResponse, error) {
	// 1. Get CashBank account details
	account, err := s.cashBankService.GetCashBankByID(accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cash bank account: %w", err)
	}

	// 2. Get GL account details (not needed for current implementation but validates AccountID exists)
	_, err = s.accountRepo.FindByID(context.Background(), account.AccountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get GL account: %w", err)
	}

	// 3. Calculate SSOT balance from account_balances view
	ssotBalance, err := s.calculateSSOTBalance(account.AccountID)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate SSOT balance: %w", err)
	}

	// 4. Get recent transactions with journal references
	recentTransactions, err := s.getRecentTransactions(accountID, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent transactions: %w", err)
	}

	// 5. Get related journal entries
	journalEntries, err := s.getJournalEntriesForAccount(account.AccountID, 20)
	if err != nil {
		return nil, fmt.Errorf("failed to get journal entries: %w", err)
	}

	// 6. Calculate balance difference
	cashBankBalance := decimal.NewFromFloat(account.Balance)
	balanceDifference := cashBankBalance.Sub(ssotBalance)

	// 7. Determine reconciliation status
	reconciliationStatus := s.determineReconciliationStatus(balanceDifference)

	// Build integrated account detail
	accountDetail := &IntegratedAccountDetail{
		ID:                   account.ID,
		AccountID:            account.AccountID,
		Name:                 account.Name,
		Code:                 account.Code,
		Type:                 account.Type,
		Balance:              cashBankBalance,
		SSOTBalance:          ssotBalance,
		Variance:             balanceDifference,
		ReconciliationStatus: reconciliationStatus,
		IsActive:             account.IsActive,
		CreatedAt:            account.CreatedAt,
		UpdatedAt:            account.UpdatedAt,
	}

	// Add optional fields
	if lastTxDate, err := s.getLastTransactionDate(account.ID); err == nil {
		accountDetail.LastTransactionDate = lastTxDate
	}
	if journalCount, err := s.countJournalEntriesForAccount(account.AccountID); err == nil {
		accountDetail.TotalJournalEntries = journalCount
	}

	now := time.Now()
	return &IntegratedAccountResponse{
		Account:              accountDetail,
		RecentTransactions:   recentTransactions,
		RecentJournalEntries: journalEntries,
		LastSyncedAt:         &now,
	}, nil
}

// GetIntegratedSummary gets summary of all cash/bank accounts with SSOT integration
func (s *CashBankIntegratedService) GetIntegratedSummary() (*IntegratedSummaryResponse, error) {
	// 1. Get all cash/bank accounts
	accounts, err := s.cashBankService.GetCashBankAccounts()
	if err != nil {
		return nil, fmt.Errorf("failed to get cash bank accounts: %w", err)
	}

	var integratedAccounts []IntegratedAccountSummary
	var totalCash, totalBank, totalBalance, totalSSOTBalance decimal.Decimal
	var varianceCount int

	// 2. Process each account
	for _, account := range accounts {
		// Get SSOT balance
		ssotBalance, err := s.calculateSSOTBalance(account.AccountID)
		if err != nil {
			ssotBalance = decimal.Zero // Default to zero if error
		}

		// Calculate variance
		accountBalance := decimal.NewFromFloat(account.Balance)
		variance := accountBalance.Sub(ssotBalance)

		// Count journal entries
		journalCount, err := s.countJournalEntriesForAccount(account.AccountID)
		if err != nil {
			journalCount = 0
		}

		// Get last transaction date
		lastTxDate, err := s.getLastTransactionDate(account.ID)
		if err != nil {
			lastTxDate = nil
		}

		// Determine reconciliation status
		reconciliationStatus := s.determineReconciliationStatus(variance)
		if !variance.IsZero() {
			varianceCount++
		}

		// Add to summary
		integratedAccounts = append(integratedAccounts, IntegratedAccountSummary{
			ID:                   account.ID,
			Name:                 account.Name,
			Code:                 account.Code,
			Type:                 account.Type,
			Balance:              accountBalance,
			SSOTBalance:          ssotBalance,
			Variance:             variance,
			LastTransactionDate:  lastTxDate,
			TotalJournalEntries:  journalCount,
			ReconciliationStatus: reconciliationStatus,
		})

		// Aggregate totals
		if account.Type == "CASH" {
			totalCash = totalCash.Add(accountBalance)
		} else {
			totalBank = totalBank.Add(accountBalance)
		}
		totalBalance = totalBalance.Add(accountBalance)
		totalSSOTBalance = totalSSOTBalance.Add(ssotBalance)
	}

	// 3. Get recent activities
	recentActivities, err := s.getRecentActivities(20)
	if err != nil {
		recentActivities = []IntegratedActivity{} // Default to empty if error
	}

	// 4. Build sync status
	now := time.Now()
	syncStatus := SyncStatus{
		LastSyncAt:       &now,
		SyncStatus:       "ACTIVE",
		TotalAccounts:    len(accounts),
		SyncedAccounts:   len(accounts) - varianceCount,
		VarianceAccounts: varianceCount,
	}

	return &IntegratedSummaryResponse{
		Summary: IntegratedBalanceSummary{
			TotalCash:        totalCash,
			TotalBank:        totalBank,
			TotalBalance:     totalBalance,
			TotalSSOTBalance: totalSSOTBalance,
			BalanceVariance:  totalBalance.Sub(totalSSOTBalance),
			VarianceCount:    varianceCount,
		},
		Accounts:         integratedAccounts,
		RecentActivities: recentActivities,
		SyncStatus:       syncStatus,
	}, nil
}

// calculateSSOTBalance calculates balance from SSOT journal system
func (s *CashBankIntegratedService) calculateSSOTBalance(accountID uint) (decimal.Decimal, error) {
	var balance struct {
		CurrentBalance decimal.Decimal `gorm:"column:current_balance"`
	}

	err := s.db.Table("account_balances").
		Where("account_id = ?", accountID).
		Select("current_balance").
		Scan(&balance).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return decimal.Zero, nil
		}
		return decimal.Zero, err
	}

	return balance.CurrentBalance, nil
}

// getRecentTransactions gets recent transactions with journal references
func (s *CashBankIntegratedService) getRecentTransactions(accountID uint, limit int) ([]IntegratedTransaction, error) {
	var transactions []models.CashBankTransaction
	err := s.db.Where("cash_bank_id = ?", accountID).
		Order("transaction_date DESC").
		Limit(limit).
		Find(&transactions).Error

	if err != nil {
		return nil, err
	}

	var integrated []IntegratedTransaction
	for _, tx := range transactions {
		// Build reference number from available fields
		referenceNumber := fmt.Sprintf("%s-%d", tx.ReferenceType, tx.ReferenceID)
		integratedTx := IntegratedTransaction{
			ID:              tx.ID,
			Amount:          decimal.NewFromFloat(tx.Amount),
			Type:            tx.ReferenceType,
			Number:          referenceNumber,
			CreatedAt:       tx.TransactionDate,
			Description:     tx.Notes,
			BalanceAfter:    decimal.NewFromFloat(tx.BalanceAfter),
			ReferenceNumber: referenceNumber,
		}

		// Try to find related journal entry
		if journalEntry, journalNum := s.findRelatedJournalEntry(tx); journalEntry != nil {
			integratedTx.JournalEntryID = &journalEntry.ID
			integratedTx.JournalEntryNumber = journalNum
		}

		integrated = append(integrated, integratedTx)
	}

	return integrated, nil
}

// getJournalEntriesForAccount gets journal entries for specific GL account
func (s *CashBankIntegratedService) getJournalEntriesForAccount(accountID uint, limit int) ([]IntegratedJournalEntry, error) {
	var entries []models.SSOTJournalEntry
	err := s.db.Joins("JOIN unified_journal_lines ON unified_journal_ledger.id = unified_journal_lines.journal_id").
		Where("unified_journal_lines.account_id = ?", accountID).
		Preload("Lines").
		Preload("Lines.Account").
		Order("unified_journal_ledger.entry_date DESC").
		Limit(limit).
		Find(&entries).Error

	if err != nil {
		return nil, err
	}

	var integrated []IntegratedJournalEntry
	for _, entry := range entries {
		integratedEntry := IntegratedJournalEntry{
			ID:          entry.ID,
			EntryNumber: entry.EntryNumber,
			EntryDate:   entry.EntryDate,
			Description: entry.Description,
			SourceType:  entry.SourceType,
			TotalDebit:  entry.TotalDebit,
			TotalCredit: entry.TotalCredit,
			Status:      entry.Status,
			CreatedAt:   entry.CreatedAt,
		}

		// Convert lines
		for _, line := range entry.Lines {
			integratedLine := IntegratedJournalLine{
				ID:           line.ID,
				AccountID:    line.AccountID,
				Description:  line.Description,
				DebitAmount:  line.DebitAmount,
				CreditAmount: line.CreditAmount,
			}

			// Add account details
			if line.Account != nil {
				integratedLine.AccountCode = line.Account.Code
				integratedLine.AccountName = line.Account.Name
			}

			integratedEntry.Lines = append(integratedEntry.Lines, integratedLine)
		}

		integrated = append(integrated, integratedEntry)
	}

	return integrated, nil
}

// Helper methods

// ReconcileBalances reconciles CashBank and COA balances according to strategy.
// strategy: to_ssot (default) => set both to SSOT account_balances.current_balance
//
//	to_transactions => set both to sum(cash_bank_transactions.amount)
//	to_coa => set both to current COA balance
func (s *CashBankIntegratedService) ReconcileBalances(req ReconcileRequest) (*ReconcileResult, error) {
	if req.Strategy == "" {
		req.Strategy = "to_ssot"
	}
	res := &ReconcileResult{Strategy: req.Strategy, DryRun: req.DryRun}

	// Collect accounts to process
	type pair struct {
		ID        uint
		AccountID uint
		CB        float64
		COA       float64
	}
	q := s.db.Table("cash_banks cb").
		Select("cb.id as id, cb.account_id as account_id, cb.balance as cb, a.balance as coa").
		Joins("JOIN accounts a ON a.id = cb.account_id").
		Where("cb.deleted_at IS NULL AND a.deleted_at IS NULL")
	if len(req.AccountIDs) > 0 {
		q = q.Where("cb.id IN ?", req.AccountIDs)
	}
	var pairs []pair
	if err := q.Scan(&pairs).Error; err != nil {
		return nil, err
	}

	returnErr := s.db.Transaction(func(tx *gorm.DB) error {
		for _, p := range pairs {
			item := ReconcileItem{CashBankID: p.ID, COAAccountID: p.AccountID,
				CashBankBefore: decimal.NewFromFloat(p.CB), COABefore: decimal.NewFromFloat(p.COA)}
			var target decimal.Decimal

			switch req.Strategy {
			case "to_transactions":
				var sum float64
				if err := tx.Table("cash_bank_transactions").
					Where("cash_bank_id = ? AND deleted_at IS NULL", p.ID).
					Select("COALESCE(SUM(amount), 0)").Scan(&sum).Error; err != nil {
					return err
				}
				item.TransactionSum = decimal.NewFromFloat(sum)
				target = item.TransactionSum
			case "to_coa":
				target = item.COABefore
			default: // to_ssot
				var ssot struct{ CurrentBalance decimal.Decimal }
				if err := tx.Table("account_balances").
					Where("account_id = ?", p.AccountID).
					Select("current_balance").Scan(&ssot).Error; err != nil {
					return err
				}
				item.SSOTBalance = ssot.CurrentBalance
				target = ssot.CurrentBalance
			}

			item.NewBalance = target
			if !req.DryRun {
				// Apply to both tables
				if err := tx.Exec("UPDATE cash_banks SET balance = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", target.InexactFloat64(), p.ID).Error; err != nil {
					return err
				}
				if err := tx.Exec("UPDATE accounts SET balance = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", target.InexactFloat64(), p.AccountID).Error; err != nil {
					return err
				}
				item.Applied = true
			}
			res.Items = append(res.Items, item)
		}
		return nil
	})

	return res, returnErr
}

func (s *CashBankIntegratedService) determineReconciliationStatus(difference decimal.Decimal) string {
	if difference.IsZero() {
		return "MATCHED"
	} else if difference.Abs().LessThan(decimal.NewFromFloat(0.01)) {
		return "MINOR_VARIANCE"
	} else {
		return "VARIANCE"
	}
}

func (s *CashBankIntegratedService) countJournalEntriesForAccount(accountID uint) (int, error) {
	var count int64
	err := s.db.Table("unified_journal_lines").
		Where("account_id = ?", accountID).
		Count(&count).Error
	return int(count), err
}

func (s *CashBankIntegratedService) getLastTransactionDate(cashBankID uint) (*time.Time, error) {
	var tx models.CashBankTransaction
	err := s.db.Where("cash_bank_id = ?", cashBankID).
		Order("transaction_date DESC").
		First(&tx).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &tx.TransactionDate, nil
}

func (s *CashBankIntegratedService) findRelatedJournalEntry(tx models.CashBankTransaction) (*models.SSOTJournalEntry, string) {
	// This is a simplified implementation
	// In practice, you'd need more sophisticated matching logic
	var entry models.SSOTJournalEntry

	// Try to match by reference or timing
	err := s.db.Where("source_type = ? AND entry_date::date = ?", "CASH_BANK", tx.TransactionDate.Format("2006-01-02")).
		First(&entry).Error

	if err != nil {
		return nil, ""
	}

	return &entry, entry.EntryNumber
}

func (s *CashBankIntegratedService) getRecentActivities(limit int) ([]IntegratedActivity, error) {
	// Get recent journal entries related to cash/bank accounts
	var activities []IntegratedActivity

	query := `
		SELECT 
			'JOURNAL_ENTRY' as type,
			je.id,
			je.entry_number as number,
			je.description,
			COALESCE(jl.debit_amount, jl.credit_amount) as amount,
			a.name as account_name,
			je.created_at
		FROM unified_journal_ledger je
		JOIN unified_journal_lines jl ON je.id = jl.journal_id
		JOIN accounts a ON jl.account_id = a.id
		JOIN cashbanks cb ON a.id = cb.account_id
		WHERE je.status = 'POSTED'
		ORDER BY je.created_at DESC
		LIMIT ?`

	err := s.db.Raw(query, limit).Scan(&activities).Error
	return activities, err
}

// GetAccountReconciliation gets detailed reconciliation data for account
func (s *CashBankIntegratedService) GetAccountReconciliation(accountID uint) (*ReconciliationData, error) {
	// Get CashBank account details
	account, err := s.cashBankService.GetCashBankByID(accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cash bank account: %w", err)
	}

	// Calculate balances
	cashBankBalance := decimal.NewFromFloat(account.Balance)
	ssotBalance, err := s.calculateSSOTBalance(account.AccountID)
	if err != nil {
		ssotBalance = decimal.Zero
	}

	difference := cashBankBalance.Sub(ssotBalance)
	hasDiscrepancy := !difference.IsZero()
	status := s.determineReconciliationStatus(difference)

	// Build reconciliation data
	return &ReconciliationData{
		AccountID:            accountID,
		AccountName:          account.Name,
		CashBankBalance:      cashBankBalance,
		SSOTBalance:          ssotBalance,
		Difference:           difference,
		HasDiscrepancy:       hasDiscrepancy,
		ReconciliationStatus: status,
		LastReconciledAt:     time.Now(),
		Details:              s.buildReconciliationDetails(difference, hasDiscrepancy),
		Recommendations:      s.buildReconciliationRecommendations(difference, hasDiscrepancy),
	}, nil
}

// GetJournalEntriesPaginated gets journal entries with pagination
func (s *CashBankIntegratedService) GetJournalEntriesPaginated(accountID uint, page, limit int) (*JournalEntriesResponse, error) {
	// Calculate offset
	offset := (page - 1) * limit

	// Get total count
	var totalCount int64
	err := s.db.Table("unified_journal_ledger je").
		Joins("JOIN unified_journal_lines jl ON je.id = jl.journal_id").
		Where("jl.account_id = ?", accountID).
		Count(&totalCount).Error
	if err != nil {
		return nil, err
	}

	// Get paginated entries
	var entries []models.SSOTJournalEntry
	err = s.db.Joins("JOIN unified_journal_lines ON unified_journal_ledger.id = unified_journal_lines.journal_id").
		Where("unified_journal_lines.account_id = ?", accountID).
		Preload("Lines").
		Preload("Lines.Account").
		Order("unified_journal_ledger.entry_date DESC").
		Offset(offset).
		Limit(limit).
		Find(&entries).Error
	if err != nil {
		return nil, err
	}

	// Convert to integrated journal entries
	var integratedEntries []IntegratedJournalEntry
	for _, entry := range entries {
		integratedEntry := IntegratedJournalEntry{
			ID:          entry.ID,
			EntryNumber: entry.EntryNumber,
			EntryDate:   entry.EntryDate,
			Description: entry.Description,
			SourceType:  entry.SourceType,
			TotalDebit:  entry.TotalDebit,
			TotalCredit: entry.TotalCredit,
			Status:      entry.Status,
			CreatedAt:   entry.CreatedAt,
		}

		// Convert lines
		for _, line := range entry.Lines {
			integratedLine := IntegratedJournalLine{
				ID:           line.ID,
				AccountID:    line.AccountID,
				Description:  line.Description,
				DebitAmount:  line.DebitAmount,
				CreditAmount: line.CreditAmount,
			}

			// Add account details
			if line.Account != nil {
				integratedLine.AccountCode = line.Account.Code
				integratedLine.AccountName = line.Account.Name
			}

			integratedEntry.Lines = append(integratedEntry.Lines, integratedLine)
		}

		integratedEntries = append(integratedEntries, integratedEntry)
	}

	// Calculate pagination info
	totalPages := int((totalCount + int64(limit) - 1) / int64(limit))

	return &JournalEntriesResponse{
		Entries: integratedEntries,
		Pagination: PaginationInfo{
			Page:       page,
			Limit:      limit,
			Total:      int(totalCount),
			TotalPages: totalPages,
		},
	}, nil
}

// GetTransactionHistoryPaginated gets transaction history with pagination
func (s *CashBankIntegratedService) GetTransactionHistoryPaginated(accountID uint, page, limit int) (*TransactionHistoryResponse, error) {
	// Calculate offset
	offset := (page - 1) * limit

	// Get total count
	var totalCount int64
	err := s.db.Model(&models.CashBankTransaction{}).
		Where("cash_bank_id = ?", accountID).
		Count(&totalCount).Error
	if err != nil {
		return nil, err
	}

	// Get paginated transactions
	var transactions []models.CashBankTransaction
	err = s.db.Where("cash_bank_id = ?", accountID).
		Order("created_at DESC, id DESC").
		Offset(offset).
		Limit(limit).
		Find(&transactions).Error
	if err != nil {
		return nil, err
	}

	// Convert to integrated transactions
	var integratedTransactions []IntegratedTransaction
	for _, tx := range transactions {
		// Build reference number from available fields
		referenceNumber := fmt.Sprintf("%s-%d", tx.ReferenceType, tx.ReferenceID)
		integratedTx := IntegratedTransaction{
			ID:              tx.ID,
			Amount:          decimal.NewFromFloat(tx.Amount),
			Type:            tx.ReferenceType,
			Number:          referenceNumber,
			CreatedAt:       tx.TransactionDate,
			Description:     tx.Notes,
			BalanceAfter:    decimal.NewFromFloat(tx.BalanceAfter),
			ReferenceNumber: referenceNumber,
		}

		// Try to find related journal entry
		if journalEntry, journalNum := s.findRelatedJournalEntry(tx); journalEntry != nil {
			integratedTx.JournalEntryID = &journalEntry.ID
			integratedTx.JournalEntryNumber = journalNum
		}

		integratedTransactions = append(integratedTransactions, integratedTx)
	}

	// Calculate pagination info
	totalPages := int((totalCount + int64(limit) - 1) / int64(limit))

	return &TransactionHistoryResponse{
		Transactions: integratedTransactions,
		Pagination: PaginationInfo{
			Page:       page,
			Limit:      limit,
			Total:      int(totalCount),
			TotalPages: totalPages,
		},
	}, nil
}

// Helper methods for reconciliation
func (s *CashBankIntegratedService) buildReconciliationDetails(difference decimal.Decimal, hasDiscrepancy bool) []string {
	var details []string
	if hasDiscrepancy {
		if difference.IsPositive() {
			details = append(details, "CashBank balance is higher than SSOT balance")
			details = append(details, "Possible missing expense entries in SSOT")
		} else {
			details = append(details, "SSOT balance is higher than CashBank balance")
			details = append(details, "Possible missing income entries in CashBank")
		}
	} else {
		details = append(details, "Account balances are in sync")
	}
	return details
}

func (s *CashBankIntegratedService) buildReconciliationRecommendations(difference decimal.Decimal, hasDiscrepancy bool) []string {
	var recommendations []string
	if hasDiscrepancy {
		recommendations = append(recommendations, "Review recent transactions for accuracy")
		recommendations = append(recommendations, "Check for pending journal entries")
		if difference.Abs().GreaterThan(decimal.NewFromFloat(100)) {
			recommendations = append(recommendations, "Large variance detected - requires immediate attention")
		}
	} else {
		recommendations = append(recommendations, "Account is properly reconciled")
		recommendations = append(recommendations, "Continue regular monitoring")
	}
	return recommendations
}
