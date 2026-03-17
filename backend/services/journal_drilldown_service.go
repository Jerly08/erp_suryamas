package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"app-sistem-akuntansi/models"
	"gorm.io/gorm"
)

type JournalDrilldownService struct {
	db *gorm.DB
}

func NewJournalDrilldownService(db *gorm.DB) *JournalDrilldownService {
	return &JournalDrilldownService{db: db}
}

// JournalDrilldownRequest represents the request for journal drill-down.
type JournalDrilldownRequest struct {
	AccountCodes     []string  `json:"account_codes"`
	AccountIDs       []uint    `json:"account_ids"`
	StartDate        time.Time `json:"start_date"`
	EndDate          time.Time `json:"end_date"`
	ReportType       string    `json:"report_type"`
	LineItemName     string    `json:"line_item_name"`
	MinAmount        *float64  `json:"min_amount"`
	MaxAmount        *float64  `json:"max_amount"`
	TransactionType  string    `json:"transaction_type"`
	TransactionTypes []string  `json:"transaction_types"`
	Page             int       `json:"page"`
	Limit            int       `json:"limit"`
}

type JournalDrilldownCreator struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}

type JournalDrilldownAccount struct {
	ID   uint64 `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type JournalDrilldownLine struct {
	ID           uint64                  `json:"id"`
	AccountID    uint64                  `json:"account_id"`
	Description  string                  `json:"description"`
	DebitAmount  float64                 `json:"debit_amount"`
	CreditAmount float64                 `json:"credit_amount"`
	Account      JournalDrilldownAccount `json:"account"`
}

type JournalDrilldownEntry struct {
	ID            uint64                  `json:"id"`
	Code          string                  `json:"code"`
	Description   string                  `json:"description"`
	Reference     string                  `json:"reference"`
	ReferenceType string                  `json:"reference_type"`
	EntryDate     time.Time               `json:"entry_date"`
	Status        string                  `json:"status"`
	TotalDebit    float64                 `json:"total_debit"`
	TotalCredit   float64                 `json:"total_credit"`
	IsBalanced    bool                    `json:"is_balanced"`
	Creator       JournalDrilldownCreator `json:"creator"`
	JournalLines  []JournalDrilldownLine  `json:"journal_lines,omitempty"`
}

// JournalDrilldownResponse represents the response with journal entries.
type JournalDrilldownResponse struct {
	JournalEntries []JournalDrilldownEntry `json:"journal_entries"`
	Total          int64                   `json:"total"`
	Summary        JournalEntrySummary     `json:"summary"`
	Metadata       DrilldownMetadata       `json:"metadata"`
}

type JournalEntrySummary struct {
	TotalDebit       float64   `json:"total_debit"`
	TotalCredit      float64   `json:"total_credit"`
	NetAmount        float64   `json:"net_amount"`
	EntryCount       int64     `json:"entry_count"`
	DateRangeStart   time.Time `json:"date_range_start"`
	DateRangeEnd     time.Time `json:"date_range_end"`
	AccountsInvolved []string  `json:"accounts_involved"`
}

type DrilldownMetadata struct {
	ReportType     string    `json:"report_type"`
	LineItemName   string    `json:"line_item_name"`
	FilterCriteria string    `json:"filter_criteria"`
	GeneratedAt    time.Time `json:"generated_at"`
}

// GetJournalEntriesForDrilldown retrieves journal entries for financial report drill-down.
func (s *JournalDrilldownService) GetJournalEntriesForDrilldown(ctx context.Context, req *JournalDrilldownRequest) (*JournalDrilldownResponse, error) {
	log.Printf("Journal Drilldown Request (SSOT): %+v", req)

	query := s.baseSSOTQuery(ctx, req)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count journal entries: %w", err)
	}

	page := req.Page
	if page < 1 {
		page = 1
	}
	limit := req.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset := (page - 1) * limit

	var entries []models.SSOTJournalEntry
	if err := query.
		Preload("Lines").
		Preload("Lines.Account").
		Preload("Creator").
		Order("entry_date DESC, id DESC").
		Offset(offset).
		Limit(limit).
		Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch SSOT journal entries: %w", err)
	}

	summary, err := s.calculateDrilldownSummary(ctx, req)
	if err != nil {
		log.Printf("failed to calculate drilldown summary: %v", err)
	}

	return &JournalDrilldownResponse{
		JournalEntries: mapSSOTEntries(entries),
		Total:          total,
		Summary:        summary,
		Metadata: DrilldownMetadata{
			ReportType:     req.ReportType,
			LineItemName:   req.LineItemName,
			FilterCriteria: s.buildFilterCriteriaText(req),
			GeneratedAt:    time.Now(),
		},
	}, nil
}

func (s *JournalDrilldownService) baseSSOTQuery(ctx context.Context, req *JournalDrilldownRequest) *gorm.DB {
	query := s.db.WithContext(ctx).
		Model(&models.SSOTJournalEntry{}).
		Where("deleted_at IS NULL").
		Where("entry_date >= ? AND entry_date <= ?", req.StartDate, req.EndDate).
		Where("status = ?", models.SSOTStatusPosted)

	accountIDs := s.resolveAccountIDs(req)
	if len(accountIDs) > 0 {
		subQuery := s.db.WithContext(ctx).
			Model(&models.SSOTJournalLine{}).
			Select("journal_id").
			Where("account_id IN ?", accountIDs)
		query = query.Where("id IN (?)", subQuery)
	}

	if req.MinAmount != nil {
		query = query.Where("(total_debit >= ? OR total_credit >= ?)", *req.MinAmount, *req.MinAmount)
	}
	if req.MaxAmount != nil {
		query = query.Where("(total_debit <= ? OR total_credit <= ?)", *req.MaxAmount, *req.MaxAmount)
	}

	transactionTypes := normalizedTransactionTypes(req)
	if len(transactionTypes) > 0 {
		query = query.Where("source_type IN ?", transactionTypes)
	}

	return query
}

func (s *JournalDrilldownService) resolveAccountIDs(req *JournalDrilldownRequest) []uint64 {
	accountIDs := make([]uint64, 0, len(req.AccountIDs))
	for _, id := range req.AccountIDs {
		accountIDs = append(accountIDs, uint64(id))
	}

	if len(req.AccountCodes) == 0 {
		return accountIDs
	}

	var codeIDs []uint64
	if err := s.db.Model(&models.Account{}).
		Where("code IN ?", req.AccountCodes).
		Pluck("id", &codeIDs).Error; err == nil {
		accountIDs = append(accountIDs, codeIDs...)
	}

	return dedupeUint64s(accountIDs)
}

func normalizedTransactionTypes(req *JournalDrilldownRequest) []string {
	raw := make([]string, 0, len(req.TransactionTypes)+1)
	raw = append(raw, req.TransactionTypes...)
	if strings.TrimSpace(req.TransactionType) != "" {
		raw = append(raw, req.TransactionType)
	}

	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		normalized := strings.ToUpper(strings.TrimSpace(item))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func mapSSOTEntries(entries []models.SSOTJournalEntry) []JournalDrilldownEntry {
	out := make([]JournalDrilldownEntry, 0, len(entries))
	for _, entry := range entries {
		totalDebit := entry.TotalDebit.InexactFloat64()
		totalCredit := entry.TotalCredit.InexactFloat64()

		creator := JournalDrilldownCreator{}
		if entry.Creator != nil {
			creator = JournalDrilldownCreator{
				ID:   uint64(entry.Creator.ID),
				Name: entry.Creator.GetDisplayName(),
			}
		}

		lines := make([]JournalDrilldownLine, 0, len(entry.Lines))
		for _, line := range entry.Lines {
			debitAmount := line.DebitAmount.InexactFloat64()
			creditAmount := line.CreditAmount.InexactFloat64()

			account := JournalDrilldownAccount{ID: line.AccountID}
			if line.Account != nil {
				account.Code = line.Account.Code
				account.Name = line.Account.Name
			}

			lines = append(lines, JournalDrilldownLine{
				ID:           line.ID,
				AccountID:    line.AccountID,
				Description:  line.Description,
				DebitAmount:  debitAmount,
				CreditAmount: creditAmount,
				Account:      account,
			})
		}

		out = append(out, JournalDrilldownEntry{
			ID:            entry.ID,
			Code:          entry.EntryNumber,
			Description:   entry.Description,
			Reference:     entry.Reference,
			ReferenceType: entry.SourceType,
			EntryDate:     entry.EntryDate,
			Status:        entry.Status,
			TotalDebit:    totalDebit,
			TotalCredit:   totalCredit,
			IsBalanced:    entry.IsBalanced,
			Creator:       creator,
			JournalLines:  lines,
		})
	}
	return out
}

func (s *JournalDrilldownService) calculateDrilldownSummary(ctx context.Context, req *JournalDrilldownRequest) (JournalEntrySummary, error) {
	query := s.baseSSOTQuery(ctx, req)

	var summary struct {
		TotalDebit  float64
		TotalCredit float64
		Count       int64
	}
	if err := query.
		Select("COALESCE(SUM(total_debit), 0) as total_debit, COALESCE(SUM(total_credit), 0) as total_credit, COUNT(*) as count").
		Scan(&summary).Error; err != nil {
		return JournalEntrySummary{}, err
	}

	var accountNames []string
	accountIDs := s.resolveAccountIDs(req)
	if len(accountIDs) > 0 {
		_ = s.db.Model(&models.Account{}).
			Where("id IN ?", accountIDs).
			Order("code").
			Pluck("name", &accountNames).Error
	}

	return JournalEntrySummary{
		TotalDebit:       summary.TotalDebit,
		TotalCredit:      summary.TotalCredit,
		NetAmount:        summary.TotalDebit - summary.TotalCredit,
		EntryCount:       summary.Count,
		DateRangeStart:   req.StartDate,
		DateRangeEnd:     req.EndDate,
		AccountsInvolved: accountNames,
	}, nil
}

func (s *JournalDrilldownService) buildFilterCriteriaText(req *JournalDrilldownRequest) string {
	var criteria []string

	if len(req.AccountCodes) > 0 {
		criteria = append(criteria, fmt.Sprintf("Account Codes: %s", strings.Join(req.AccountCodes, ", ")))
	}

	transactionTypes := normalizedTransactionTypes(req)
	if len(transactionTypes) > 0 {
		criteria = append(criteria, fmt.Sprintf("Transaction Types: %s", strings.Join(transactionTypes, ", ")))
	}

	if req.MinAmount != nil || req.MaxAmount != nil {
		amountFilter := "Amount: "
		if req.MinAmount != nil && req.MaxAmount != nil {
			amountFilter += fmt.Sprintf("%.2f - %.2f", *req.MinAmount, *req.MaxAmount)
		} else if req.MinAmount != nil {
			amountFilter += fmt.Sprintf(">= %.2f", *req.MinAmount)
		} else {
			amountFilter += fmt.Sprintf("<= %.2f", *req.MaxAmount)
		}
		criteria = append(criteria, amountFilter)
	}

	criteria = append(criteria, fmt.Sprintf("Date Range: %s - %s",
		req.StartDate.Format("2006-01-02"),
		req.EndDate.Format("2006-01-02")))

	if len(criteria) == 0 {
		return "No specific filters applied"
	}
	return strings.Join(criteria, " | ")
}

// GetJournalEntryDetail gets detailed information for a specific SSOT journal entry.
func (s *JournalDrilldownService) GetJournalEntryDetail(ctx context.Context, journalEntryID uint64) (*JournalDrilldownEntry, error) {
	var entry models.SSOTJournalEntry
	if err := s.db.WithContext(ctx).
		Preload("Lines").
		Preload("Lines.Account").
		Preload("Creator").
		Where("id = ? AND deleted_at IS NULL", journalEntryID).
		First(&entry).Error; err != nil {
		return nil, fmt.Errorf("failed to get SSOT journal entry detail: %w", err)
	}

	mapped := mapSSOTEntries([]models.SSOTJournalEntry{entry})
	if len(mapped) == 0 {
		return nil, fmt.Errorf("journal entry not found")
	}
	return &mapped[0], nil
}

// GetAccountsForPeriod gets all accounts that have journal activity in a specific period.
func (s *JournalDrilldownService) GetAccountsForPeriod(ctx context.Context, startDate, endDate time.Time) ([]models.Account, error) {
	var accounts []models.Account
	query := s.db.WithContext(ctx).Model(&models.Account{}).
		Joins("JOIN unified_journal_lines ON accounts.id = unified_journal_lines.account_id").
		Joins("JOIN unified_journal_ledger ON unified_journal_lines.journal_id = unified_journal_ledger.id").
		Where("unified_journal_ledger.entry_date >= ? AND unified_journal_ledger.entry_date <= ?", startDate, endDate).
		Where("unified_journal_ledger.status = ?", models.SSOTStatusPosted).
		Where("unified_journal_ledger.deleted_at IS NULL").
		Group("accounts.id").
		Order("accounts.code")

	if err := query.Find(&accounts).Error; err != nil {
		return nil, fmt.Errorf("failed to get active accounts for period from SSOT: %w", err)
	}
	return accounts, nil
}

func dedupeUint64s(items []uint64) []uint64 {
	seen := make(map[uint64]struct{}, len(items))
	out := make([]uint64, 0, len(items))
	for _, item := range items {
		if item == 0 {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
