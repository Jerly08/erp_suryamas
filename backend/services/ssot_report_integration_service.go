package services

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// SSOTReportIntegrationService integrates SSOT journal with all financial reports
type SSOTReportIntegrationService struct {
	db                    *gorm.DB
	unifiedJournalService *UnifiedJournalService
	enhancedReportService *EnhancedReportService
	mu                    sync.RWMutex
	lastRefreshTime       time.Time
}

// NewSSOTReportIntegrationService creates a new SSOT report integration service
func NewSSOTReportIntegrationService(
	db *gorm.DB,
	unifiedJournalService *UnifiedJournalService,
	enhancedReportService *EnhancedReportService,
) *SSOTReportIntegrationService {
	return &SSOTReportIntegrationService{
		db:                    db,
		unifiedJournalService: unifiedJournalService,
		enhancedReportService: enhancedReportService,
		lastRefreshTime:       time.Now(),
	}
}

// ReportUpdateEvent represents real-time report updates via websocket
type ReportUpdateEvent struct {
	Type        string                 `json:"type"` // PROFIT_LOSS, BALANCE_SHEET, CASH_FLOW, etc.
	ReportData  map[string]interface{} `json:"report_data"`
	UpdatedAt   time.Time              `json:"updated_at"`
	TriggeredBy string                 `json:"triggered_by"` // JOURNAL_POSTED, TRANSACTION_CREATED, etc.
	JournalID   *uint64                `json:"journal_id,omitempty"`
}

// IntegratedFinancialReports represents all reports integrated with SSOT journal
type IntegratedFinancialReports struct {
	ProfitLoss      *ProfitLossData      `json:"profit_loss"`
	BalanceSheet    *BalanceSheetData    `json:"balance_sheet"`
	CashFlow        *CashFlowData        `json:"cash_flow"`
	SalesSummary    *SalesSummaryData    `json:"sales_summary"`
	VendorAnalysis  *VendorAnalysisData  `json:"vendor_analysis"`
	TrialBalance    *TrialBalanceData    `json:"trial_balance"`
	GeneralLedger   *GeneralLedgerData   `json:"general_ledger"`
	JournalAnalysis *JournalAnalysisData `json:"journal_analysis"`
	GeneratedAt     time.Time            `json:"generated_at"`
	DataSourceInfo  DataSourceInfo       `json:"data_source_info"`
}

// DataSourceInfo provides information about the SSOT integration
type DataSourceInfo struct {
	SSOTVersion         string    `json:"ssot_version"`
	LastJournalSync     time.Time `json:"last_journal_sync"`
	TotalJournalEntries int64     `json:"total_journal_entries"`
	PostedEntries       int64     `json:"posted_entries"`
	DraftEntries        int64     `json:"draft_entries"`
	DataIntegrityCheck  bool      `json:"data_integrity_check"`
}

// SSOT-specific extensions of existing types (removing duplicates)

// JournalAnalysisData represents comprehensive journal entry analysis
type JournalAnalysisData struct {
	Company            CompanyInfo          `json:"company"`
	StartDate          time.Time            `json:"start_date"`
	EndDate            time.Time            `json:"end_date"`
	Currency           string               `json:"currency"`
	TotalEntries       int64                `json:"total_entries"`
	PostedEntries      int64                `json:"posted_entries"`
	DraftEntries       int64                `json:"draft_entries"`
	ReversedEntries    int64                `json:"reversed_entries"`
	TotalAmount        decimal.Decimal      `json:"total_amount"`
	EntriesByType      []EntryTypeBreakdown `json:"entries_by_type"`
	EntriesByAccount   []AccountBreakdown   `json:"entries_by_account"`
	EntriesByPeriod    []PeriodBreakdown    `json:"entries_by_period"`
	ComplianceCheck    ComplianceReport     `json:"compliance_check"`
	DataQualityMetrics DataQualityMetrics   `json:"data_quality_metrics"`
	GeneratedAt        time.Time            `json:"generated_at"`
}

// Supporting data structures
type VendorDetail struct {
	VendorID            uint64          `json:"vendor_id"`
	VendorName          string          `json:"vendor_name"`
	TotalPurchases      decimal.Decimal `json:"total_purchases"`
	TotalPayments       decimal.Decimal `json:"total_payments"`
	Outstanding         decimal.Decimal `json:"outstanding"`
	LastTransactionDate time.Time       `json:"last_transaction_date"`
	PaymentTerms        string          `json:"payment_terms"`
	AveragePaymentDays  float64         `json:"average_payment_days"`
}

type PaymentAnalysis struct {
	OnTimePayments int64           `json:"on_time_payments"`
	LatePayments   int64           `json:"late_payments"`
	AveragePayDays float64         `json:"average_pay_days"`
	TotalDiscounts decimal.Decimal `json:"total_discounts"`
	PenaltyFees    decimal.Decimal `json:"penalty_fees"`
}

type AgingBucket struct {
	Description string          `json:"description"`
	DaysRange   string          `json:"days_range"`
	Amount      decimal.Decimal `json:"amount"`
	Count       int64           `json:"count"`
	Percentage  float64         `json:"percentage"`
}

// Using existing types from enhanced_report_service.go
// SSOTTrialBalanceAccount extends TrialBalanceItem
type SSOTTrialBalanceAccount struct {
	AccountID     uint64          `json:"account_id"`
	AccountCode   string          `json:"account_code"`
	AccountName   string          `json:"account_name"`
	AccountType   string          `json:"account_type"`
	DebitBalance  decimal.Decimal `json:"debit_balance"`
	CreditBalance decimal.Decimal `json:"credit_balance"`
	NormalBalance string          `json:"normal_balance"`
	SSOTBalance   decimal.Decimal `json:"ssot_balance"`
	LastUpdated   time.Time       `json:"last_updated"`
}

// SSOTGeneralLedgerEntry extends GeneralLedgerEntry
type SSOTGeneralLedgerEntry struct {
	JournalID      uint64          `json:"journal_id"`
	EntryNumber    string          `json:"entry_number"`
	EntryDate      time.Time       `json:"entry_date"`
	Description    string          `json:"description"`
	Reference      string          `json:"reference"`
	DebitAmount    decimal.Decimal `json:"debit_amount"`
	CreditAmount   decimal.Decimal `json:"credit_amount"`
	RunningBalance decimal.Decimal `json:"running_balance"`
	Status         string          `json:"status"`
	SourceType     string          `json:"source_type"`
	SSOTJournalID  uint64          `json:"ssot_journal_id"`
	SSOTLineID     uint64          `json:"ssot_line_id"`
}

type EntryTypeBreakdown struct {
	SourceType  string          `json:"source_type"`
	Count       int64           `json:"count"`
	TotalAmount decimal.Decimal `json:"total_amount"`
	Percentage  float64         `json:"percentage"`
}

type AccountBreakdown struct {
	AccountID   uint64          `json:"account_id"`
	AccountCode string          `json:"account_code"`
	AccountName string          `json:"account_name"`
	Count       int64           `json:"count"`
	TotalDebit  decimal.Decimal `json:"total_debit"`
	TotalCredit decimal.Decimal `json:"total_credit"`
}

type PeriodBreakdown struct {
	Period      string          `json:"period"`
	StartDate   time.Time       `json:"start_date"`
	EndDate     time.Time       `json:"end_date"`
	Count       int64           `json:"count"`
	TotalAmount decimal.Decimal `json:"total_amount"`
}

type ComplianceReport struct {
	TotalChecks     int               `json:"total_checks"`
	PassedChecks    int               `json:"passed_checks"`
	FailedChecks    int               `json:"failed_checks"`
	ComplianceScore float64           `json:"compliance_score"`
	Issues          []ComplianceIssue `json:"issues"`
	Recommendations []string          `json:"recommendations"`
}

type ComplianceIssue struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	JournalID   uint64 `json:"journal_id"`
}

type DataQualityMetrics struct {
	OverallScore      float64                `json:"overall_score"`
	CompletenessScore float64                `json:"completeness_score"`
	AccuracyScore     float64                `json:"accuracy_score"`
	ConsistencyScore  float64                `json:"consistency_score"`
	Issues            []DataQualityIssue     `json:"issues"`
	DetailedMetrics   map[string]interface{} `json:"detailed_metrics"`
}

type DataQualityIssue struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Count       int64  `json:"count"`
	Severity    string `json:"severity"`
}

// GenerateIntegratedReports generates all financial reports integrated with SSOT journal
func (s *SSOTReportIntegrationService) GenerateIntegratedReports(ctx context.Context, startDate, endDate time.Time) (*IntegratedFinancialReports, error) {
	reports := &IntegratedFinancialReports{
		GeneratedAt: time.Now(),
	}

	// Generate data source info
	dataSourceInfo, err := s.generateDataSourceInfo(ctx)
	if err != nil {
		log.Printf("Error generating data source info: %v", err)
		// Continue with empty data source info
		dataSourceInfo = &DataSourceInfo{}
	}
	reports.DataSourceInfo = *dataSourceInfo

	// Generate all reports concurrently
	// Use a local mutex to protect the local reports variable,
	// so concurrent calls to GenerateIntegratedReports don't interfere.
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, 8)

	// Profit & Loss Statement
	wg.Add(1)
	go func() {
		defer wg.Done()
		pl, err := s.enhancedReportService.GenerateProfitLoss(startDate, endDate)
		if err != nil {
			errChan <- fmt.Errorf("profit loss generation failed: %w", err)
			return
		}
		mu.Lock()
		reports.ProfitLoss = pl
		mu.Unlock()
	}()

	// Balance Sheet
	wg.Add(1)
	go func() {
		defer wg.Done()
		bs, err := s.enhancedReportService.GenerateBalanceSheet(endDate)
		if err != nil {
			errChan <- fmt.Errorf("balance sheet generation failed: %w", err)
			return
		}
		mu.Lock()
		reports.BalanceSheet = bs
		mu.Unlock()
	}()

	// Cash Flow Statement
	wg.Add(1)
	go func() {
		defer wg.Done()
		cf, err := s.enhancedReportService.GenerateCashFlow(startDate, endDate)
		if err != nil {
			errChan <- fmt.Errorf("cash flow generation failed: %w", err)
			return
		}
		mu.Lock()
		reports.CashFlow = cf
		mu.Unlock()
	}()

	// Sales Summary Report
	wg.Add(1)
	go func() {
		defer wg.Done()
		ss, err := s.enhancedReportService.GenerateSalesSummary(startDate, endDate, "month", nil)
		if err != nil {
			errChan <- fmt.Errorf("sales summary generation failed: %w", err)
			return
		}
		mu.Lock()
		reports.SalesSummary = ss
		mu.Unlock()
	}()

	// Vendor Analysis Report
	wg.Add(1)
	go func() {
		defer wg.Done()
		va, err := s.generateVendorAnalysis(ctx, startDate, endDate)
		if err != nil {
			errChan <- fmt.Errorf("vendor analysis generation failed: %w", err)
			return
		}
		mu.Lock()
		reports.VendorAnalysis = va
		mu.Unlock()
	}()

	// Trial Balance
	wg.Add(1)
	go func() {
		defer wg.Done()
		tb, err := s.generateTrialBalance(ctx, endDate)
		if err != nil {
			errChan <- fmt.Errorf("trial balance generation failed: %w", err)
			return
		}
		mu.Lock()
		reports.TrialBalance = tb
		mu.Unlock()
	}()

	// General Ledger
	wg.Add(1)
	go func() {
		defer wg.Done()
		gl, err := s.generateGeneralLedger(ctx, startDate, endDate, nil)
		if err != nil {
			errChan <- fmt.Errorf("general ledger generation failed: %w", err)
			return
		}
		mu.Lock()
		reports.GeneralLedger = gl
		mu.Unlock()
	}()

	// Journal Analysis
	wg.Add(1)
	go func() {
		defer wg.Done()
		ja, err := s.generateJournalAnalysis(ctx, startDate, endDate)
		if err != nil {
			errChan <- fmt.Errorf("journal analysis generation failed: %w", err)
			return
		}
		mu.Lock()
		reports.JournalAnalysis = ja
		mu.Unlock()
	}()

	// Wait for all reports to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		// Return partial results with warning instead of failing entirely
		log.Printf("WARNING: Report generation completed with %d error(s): %v", len(errors), errors)
		// Still return the reports that succeeded, along with the error for logging
		return reports, fmt.Errorf("report generation completed with %d error(s): %v", len(errors), errors)
	}

	// Update last refresh time (WebSocket broadcasting removed)
	s.mu.Lock()
	s.lastRefreshTime = time.Now()
	s.mu.Unlock()

	// Log report generation completion
	log.Printf("Generated integrated reports: profit_loss, balance_sheet, cash_flow, sales_summary, vendor_analysis, trial_balance, general_ledger, journal_analysis")

	return reports, nil
}

// GenerateSalesSummaryFromSSot generates sales summary using SSOT journal data
func (s *SSOTReportIntegrationService) GenerateSalesSummaryFromSSot(startDate, endDate time.Time, projectID *uint) (*SalesSummaryData, error) {
	return s.enhancedReportService.GenerateSalesSummary(startDate, endDate, "month", projectID)
}

// GenerateVendorAnalysisFromSSot generates vendor analysis using SSOT journal data
func (s *SSOTReportIntegrationService) GenerateVendorAnalysisFromSSot(startDate, endDate time.Time) (*VendorAnalysisData, error) {
	ctx := context.Background()
	return s.generateVendorAnalysis(ctx, startDate, endDate)
}

// GenerateTrialBalanceFromSSot generates trial balance using SSOT journal data
func (s *SSOTReportIntegrationService) GenerateTrialBalanceFromSSot(asOfDate time.Time) (*TrialBalanceData, error) {
	ctx := context.Background()
	return s.generateTrialBalance(ctx, asOfDate)
}

// GenerateGeneralLedgerFromSSot generates general ledger using SSOT journal data
func (s *SSOTReportIntegrationService) GenerateGeneralLedgerFromSSot(startDate, endDate time.Time, accountID *uint64) (*GeneralLedgerData, error) {
	ctx := context.Background()
	return s.generateGeneralLedger(ctx, startDate, endDate, accountID)
}

// GenerateJournalAnalysisFromSSot generates journal analysis using SSOT journal data
func (s *SSOTReportIntegrationService) GenerateJournalAnalysisFromSSot(startDate, endDate time.Time) (*JournalAnalysisData, error) {
	ctx := context.Background()
	return s.generateJournalAnalysis(ctx, startDate, endDate)
}

// OnJournalPosted handles updates when a journal is posted (WebSocket removed for stability)
func (s *SSOTReportIntegrationService) OnJournalPosted(journalID uint64) {
	// Update last refresh time
	s.mu.Lock()
	s.lastRefreshTime = time.Now()
	s.mu.Unlock()

	// Log journal posting event (WebSocket broadcasting removed)
	log.Printf("Journal %d posted, reports may need refresh", journalID)
}

// OnTransactionCreated handles updates when transactions are created (WebSocket removed for stability)
func (s *SSOTReportIntegrationService) OnTransactionCreated(transactionType string, transactionID uint64, affectedAccounts []uint64) {
	// Update last refresh time
	s.mu.Lock()
	s.lastRefreshTime = time.Now()
	s.mu.Unlock()

	// Log transaction creation event (WebSocket broadcasting removed)
	log.Printf("Transaction %s %d created, reports may need refresh", transactionType, transactionID)
}

// Private helper methods

func (s *SSOTReportIntegrationService) generateDataSourceInfo(ctx context.Context) (*DataSourceInfo, error) {
	var stats struct {
		TotalEntries  int64 `json:"total_entries"`
		PostedEntries int64 `json:"posted_entries"`
		DraftEntries  int64 `json:"draft_entries"`
	}

	// Get journal statistics
	if err := s.db.WithContext(ctx).Model(&models.SSOTJournalEntry{}).
		Select(`
			COUNT(*) as total_entries,
			COUNT(CASE WHEN status = ? THEN 1 END) as posted_entries,
			COUNT(CASE WHEN status = ? THEN 1 END) as draft_entries
		`, models.SSOTStatusPosted, models.SSOTStatusDraft).
		Scan(&stats).Error; err != nil {
		return nil, fmt.Errorf("failed to get journal statistics: %w", err)
	}

	// Get last sync time (last updated journal entry)
	var lastSync time.Time
	s.db.WithContext(ctx).Model(&models.SSOTJournalEntry{}).
		Select("MAX(updated_at)").
		Where("status = ?", models.SSOTStatusPosted).
		Scan(&lastSync)

	return &DataSourceInfo{
		SSOTVersion:         "1.0",
		LastJournalSync:     lastSync,
		TotalJournalEntries: stats.TotalEntries,
		PostedEntries:       stats.PostedEntries,
		DraftEntries:        stats.DraftEntries,
		DataIntegrityCheck:  true, // This could be a more complex check
	}, nil
}

// mapSourceTypeToFriendlyName converts internal source types to user-friendly names
func (s *SSOTReportIntegrationService) mapSourceTypeToFriendlyName(sourceType string) string {
	switch sourceType {
	case models.SSOTSourceTypeSale:
		return "Sales Transaction"
	case models.SSOTSourceTypePurchase:
		return "Purchase Transaction"
	case models.SSOTSourceTypePayment:
		return "Payment Transaction"
	case models.SSOTSourceTypeCashBank:
		return "Cash & Bank Transaction"
	case models.SSOTSourceTypeAsset:
		return "Asset Transaction"
	case models.SSOTSourceTypeManual:
		return "Manual Journal Entry"
	case models.SSOTSourceTypeOpening:
		return "Opening Balance"
	case models.SSOTSourceTypeClosing:
		return "Closing Entry"
	case models.SSOTSourceTypeAdjustment:
		return "Adjustment Entry"
	case models.SSOTSourceTypeTransfer:
		return "Transfer Transaction"
	case models.SSOTSourceTypeDepreciation:
		return "Depreciation Entry"
	case models.SSOTSourceTypeReversal:
		return "Reversal Entry"
	default:
		// strings.Title is deprecated; manually capitalize first letter of each word
		words := strings.Fields(strings.ToLower(strings.ReplaceAll(sourceType, "_", " ")))
		for i, w := range words {
			if len(w) > 0 {
				words[i] = strings.ToUpper(w[:1]) + w[1:]
			}
		}
		return strings.Join(words, " ")
	}
}

// generateAccountBreakdown creates account-wise breakdown of journal entries
func (s *SSOTReportIntegrationService) generateAccountBreakdown(ctx context.Context, startDate, endDate time.Time) []AccountBreakdown {
	var accountBreakdown []AccountBreakdown

	query := `
		SELECT 
			a.id as account_id,
			a.code as account_code,
			a.name as account_name,
			COUNT(DISTINCT ujl.journal_id) as count,
			COALESCE(SUM(ujl.debit_amount), 0) as total_debit,
			COALESCE(SUM(ujl.credit_amount), 0) as total_credit
		FROM unified_journal_lines ujl
		JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
		JOIN accounts a ON a.id = ujl.account_id
		WHERE uje.entry_date >= ? AND uje.entry_date <= ? 
			AND uje.status = ?
			AND uje.deleted_at IS NULL
			AND a.deleted_at IS NULL
		GROUP BY a.id, a.code, a.name
		ORDER BY total_debit + total_credit DESC
		LIMIT 20
	`

	if err := s.db.WithContext(ctx).Raw(query, startDate, endDate, models.SSOTStatusPosted).Scan(&accountBreakdown).Error; err != nil {
		log.Printf("Error scanning account breakdown: %v", err)
	}

	return accountBreakdown
}

// generatePeriodBreakdown creates period-wise breakdown of journal entries
func (s *SSOTReportIntegrationService) generatePeriodBreakdown(ctx context.Context, startDate, endDate time.Time) []PeriodBreakdown {
	var periodBreakdown []PeriodBreakdown

	// Determine period type based on date range
	daysDiff := int(endDate.Sub(startDate).Hours() / 24)
	var groupBy, dateFormat string

	if daysDiff <= 31 {
		// Daily breakdown for month or less
		groupBy = "DATE(entry_date)"
		dateFormat = "2006-01-02"
	} else if daysDiff <= 365 {
		// Weekly breakdown for year or less
		groupBy = "DATE_TRUNC('week', entry_date)"
		dateFormat = "2006-01-02"
	} else {
		// Monthly breakdown for more than a year
		groupBy = "DATE_TRUNC('month', entry_date)"
		dateFormat = "2006-01"
	}

	query := fmt.Sprintf(`
		SELECT 
			%s as period,
			%s as start_date,
			%s as end_date,
			COUNT(*) as count,
			COALESCE(SUM(total_debit), 0) as total_amount
		FROM unified_journal_ledger
		WHERE entry_date >= ? AND entry_date <= ? AND status = ? AND deleted_at IS NULL
		GROUP BY %s
		ORDER BY %s
	`, groupBy, groupBy, groupBy, groupBy, groupBy)

	type periodResult struct {
		Period      time.Time       `json:"period"`
		StartDate   time.Time       `json:"start_date"`
		EndDate     time.Time       `json:"end_date"`
		Count       int64           `json:"count"`
		TotalAmount decimal.Decimal `json:"total_amount"`
	}

	var results []periodResult
	if err := s.db.WithContext(ctx).Raw(query, startDate, endDate, models.SSOTStatusPosted).Scan(&results).Error; err != nil {
		log.Printf("Error scanning period breakdown results: %v", err)
	}

	for _, result := range results {
		periodBreakdown = append(periodBreakdown, PeriodBreakdown{
			Period:      result.Period.Format(dateFormat),
			StartDate:   result.StartDate,
			EndDate:     result.EndDate,
			Count:       result.Count,
			TotalAmount: result.TotalAmount,
		})
	}

	return periodBreakdown
}

// generateComplianceCheck creates basic compliance reporting
func (s *SSOTReportIntegrationService) generateComplianceCheck(ctx context.Context, startDate, endDate time.Time) ComplianceReport {
	var totalEntries, balancedEntries, unbalancedEntries int64

	// Check for balanced entries
	s.db.WithContext(ctx).Model(&models.SSOTJournalEntry{}).
		Where("entry_date >= ? AND entry_date <= ? AND status = ?", startDate, endDate, models.SSOTStatusPosted).
		Count(&totalEntries)

	s.db.WithContext(ctx).Model(&models.SSOTJournalEntry{}).
		Where("entry_date >= ? AND entry_date <= ? AND status = ? AND is_balanced = ?", startDate, endDate, models.SSOTStatusPosted, true).
		Count(&balancedEntries)

	unbalancedEntries = totalEntries - balancedEntries

	var issues []ComplianceIssue
	if unbalancedEntries > 0 {
		issues = append(issues, ComplianceIssue{
			Type:        "BALANCE_MISMATCH",
			Description: fmt.Sprintf("%d journal entries are not balanced", unbalancedEntries),
			Severity:    "HIGH",
		})
	}

	passedChecks := 1
	totalChecks := 1
	if unbalancedEntries > 0 {
		passedChecks = 0
	}

	complianceScore := float64(passedChecks) / float64(totalChecks) * 100

	return ComplianceReport{
		TotalChecks:     totalChecks,
		PassedChecks:    passedChecks,
		FailedChecks:    totalChecks - passedChecks,
		ComplianceScore: complianceScore,
		Issues:          issues,
		Recommendations: []string{"Review and correct unbalanced journal entries", "Implement automated balance validation"},
	}
}

// generateDataQualityMetrics creates data quality assessment
func (s *SSOTReportIntegrationService) generateDataQualityMetrics(ctx context.Context, startDate, endDate time.Time) DataQualityMetrics {
	var totalEntries, entriesWithDescription, entriesWithReference int64

	// Get total entries
	s.db.WithContext(ctx).Model(&models.SSOTJournalEntry{}).
		Where("entry_date >= ? AND entry_date <= ? AND status = ?", startDate, endDate, models.SSOTStatusPosted).
		Count(&totalEntries)

	// Check entries with descriptions
	s.db.WithContext(ctx).Model(&models.SSOTJournalEntry{}).
		Where("entry_date >= ? AND entry_date <= ? AND status = ? AND description != '' AND description IS NOT NULL", startDate, endDate, models.SSOTStatusPosted).
		Count(&entriesWithDescription)

	// Check entries with references
	s.db.WithContext(ctx).Model(&models.SSOTJournalEntry{}).
		Where("entry_date >= ? AND entry_date <= ? AND status = ? AND reference != '' AND reference IS NOT NULL", startDate, endDate, models.SSOTStatusPosted).
		Count(&entriesWithReference)

	var issues []DataQualityIssue
	completenessScore := 100.0
	accuracyScore := 100.0
	consistencyScore := 100.0

	if totalEntries > 0 {
		descriptionCompleteness := float64(entriesWithDescription) / float64(totalEntries) * 100
		referenceCompleteness := float64(entriesWithReference) / float64(totalEntries) * 100
		completenessScore = (descriptionCompleteness + referenceCompleteness) / 2

		if descriptionCompleteness < 80 {
			issues = append(issues, DataQualityIssue{
				Type:        "MISSING_DESCRIPTION",
				Description: fmt.Sprintf("%.1f%% of entries lack proper descriptions", 100-descriptionCompleteness),
				Severity:    "MEDIUM",
				Count:       totalEntries - entriesWithDescription,
			})
		}

		if referenceCompleteness < 60 {
			issues = append(issues, DataQualityIssue{
				Type:        "MISSING_REFERENCE",
				Description: fmt.Sprintf("%.1f%% of entries lack reference information", 100-referenceCompleteness),
				Severity:    "LOW",
				Count:       totalEntries - entriesWithReference,
			})
		}
	}

	overallScore := (completenessScore + accuracyScore + consistencyScore) / 3

	return DataQualityMetrics{
		OverallScore:      overallScore,
		CompletenessScore: completenessScore,
		AccuracyScore:     accuracyScore,
		ConsistencyScore:  consistencyScore,
		Issues:            issues,
		DetailedMetrics: map[string]interface{}{
			"total_entries":            totalEntries,
			"entries_with_description": entriesWithDescription,
			"entries_with_reference":   entriesWithReference,
			"description_completeness": func() float64 {
				if totalEntries > 0 {
					return float64(entriesWithDescription) / float64(totalEntries) * 100
				}
				return 0
			}(),
			"reference_completeness": func() float64 {
				if totalEntries > 0 {
					return float64(entriesWithReference) / float64(totalEntries) * 100
				}
				return 0
			}(),
		},
	}
}

// GetDB returns the database instance (for external access)
func (s *SSOTReportIntegrationService) GetDB() *gorm.DB {
	return s.db
}

// getCompanyInfo returns company information for reports
func (s *SSOTReportIntegrationService) getCompanyInfo() CompanyInfo {
	// Prefer Settings table (admin-configured company information)
	var settings models.Settings
	if err := s.db.First(&settings).Error; err == nil {
		return CompanyInfo{
			Name:      settings.CompanyName,
			Address:   settings.CompanyAddress,
			City:      "", // City can be part of address; keep separate empty if not structured
			State:     "",
			Phone:     settings.CompanyPhone,
			Email:     settings.CompanyEmail,
			Website:   settings.CompanyWebsite,
			TaxNumber: settings.TaxNumber,
		}
	}
	// Fallback defaults
	return CompanyInfo{
		Name:      "PT. Default Company",
		Address:   "Jalan Default No. 1",
		City:      "Jakarta",
		State:     "DKI Jakarta",
		Phone:     "+62-21-12345678",
		Email:     "info@defaultcompany.com",
		TaxNumber: "01.234.567.8-901.000",
	}
}

// getCurrencyFromSettings returns Settings.Currency or IDR
func (s *SSOTReportIntegrationService) getCurrencyFromSettings() string {
	var settings models.Settings
	if err := s.db.First(&settings).Error; err == nil && settings.Currency != "" {
		return settings.Currency
	}
	return "IDR"
}

// Implementation of report generation methods
func (s *SSOTReportIntegrationService) generateVendorAnalysis(ctx context.Context, startDate, endDate time.Time) (*VendorAnalysisData, error) {
	var result VendorAnalysisData
	result.Company = s.getCompanyInfo()
	result.StartDate = startDate
	result.EndDate = endDate
	result.Currency = s.getCurrencyFromSettings()
	result.GeneratedAt = time.Now()

	// Get vendor analysis from SSOT journal data
	vendorStats := make(map[uint64]*VendorDetail)

	// Query vendor transactions from SSOT journal entries with account information
	type VendorJournalLine struct {
		JournalID    uint64
		VendorID     uint64
		VendorName   string
		DebitAmount  decimal.Decimal
		CreditAmount decimal.Decimal
		EntryDate    time.Time
	}

	var journalLines []VendorJournalLine
	query := `
		SELECT 
			sjl.journal_id,
			c.id as vendor_id,
			c.name as vendor_name,
			sjl.debit_amount,
			sjl.credit_amount,
			sje.entry_date
		FROM unified_journal_lines sjl
		JOIN unified_journal_ledger sje ON sje.id = sjl.journal_id
		JOIN accounts a ON a.id = sjl.account_id
		LEFT JOIN purchases p
			ON sje.source_type = 'PURCHASE'
			AND p.id = sje.source_id
			AND p.deleted_at IS NULL
		LEFT JOIN payments pay
			ON sje.source_type IN ('PAYMENT', 'PURCHASE_PAYMENT')
			AND pay.id = sje.source_id
			AND pay.deleted_at IS NULL
		LEFT JOIN contacts c
			ON c.id = COALESCE(p.vendor_id, pay.contact_id)
			AND c.type = 'VENDOR'
			AND c.deleted_at IS NULL
		WHERE sje.entry_date BETWEEN ? AND ?
			AND sje.status = ?
			AND sje.deleted_at IS NULL
			AND a.code LIKE '2101%'
			AND sje.source_type IN ('PURCHASE', 'PAYMENT', 'PURCHASE_PAYMENT')
			AND c.id IS NOT NULL
		ORDER BY sje.entry_date, sjl.journal_id
	`

	err := s.db.WithContext(ctx).Raw(query, startDate, endDate, models.SSOTStatusPosted).Scan(&journalLines).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch vendor journal entries: %w", err)
	}

	// Process journal lines to build vendor analysis
	for _, line := range journalLines {
		if line.VendorID == 0 {
			continue
		}

		if _, exists := vendorStats[line.VendorID]; !exists {
			vendorStats[line.VendorID] = &VendorDetail{
				VendorID:            line.VendorID,
				VendorName:          line.VendorName,
				LastTransactionDate: line.EntryDate,
			}
		}

		vendor := vendorStats[line.VendorID]
		// For accounts payable: Credit increases payable (purchases), Debit decreases payable (payments)
		if line.CreditAmount.GreaterThan(decimal.Zero) {
			vendor.TotalPurchases = vendor.TotalPurchases.Add(line.CreditAmount)
		} else if line.DebitAmount.GreaterThan(decimal.Zero) {
			vendor.TotalPayments = vendor.TotalPayments.Add(line.DebitAmount)
		}

		// Update last transaction date
		if line.EntryDate.After(vendor.LastTransactionDate) {
			vendor.LastTransactionDate = line.EntryDate
		}
	}

	// Calculate totals and convert to slice
	var vendors []VendorDetail
	var totalPurchases, totalPayments decimal.Decimal

	for _, vendor := range vendorStats {
		vendor.Outstanding = vendor.TotalPurchases.Sub(vendor.TotalPayments)
		vendors = append(vendors, *vendor)
		totalPurchases = totalPurchases.Add(vendor.TotalPurchases)
		totalPayments = totalPayments.Add(vendor.TotalPayments)
	}

	result.TotalVendors = int64(len(vendors))
	result.ActiveVendors = result.TotalVendors // Simplified - all vendors are considered active
	result.TotalPurchases, _ = totalPurchases.Float64()
	result.TotalPayments, _ = totalPayments.Float64()
	result.OutstandingPayables, _ = totalPurchases.Sub(totalPayments).Float64()

	// Create vendor performance data
	result.VendorsByPerformance = []VendorPerformanceData{}
	for _, vendor := range vendors {
		purchases, _ := vendor.TotalPurchases.Float64()
		payments, _ := vendor.TotalPayments.Float64()
		outstanding, _ := vendor.Outstanding.Float64()

		perfData := VendorPerformanceData{
			VendorID:           uint(vendor.VendorID),
			VendorName:         vendor.VendorName,
			TotalPurchases:     purchases,
			TotalPayments:      payments,
			Outstanding:        outstanding,
			AveragePaymentDays: 30.0,   // Default value
			PaymentScore:       85.0,   // Default score
			Rating:             "Good", // Default rating
		}
		result.VendorsByPerformance = append(result.VendorsByPerformance, perfData)
	}

	return &result, nil
}

func (s *SSOTReportIntegrationService) generateTrialBalance(ctx context.Context, asOfDate time.Time) (*TrialBalanceData, error) {
	result := &TrialBalanceData{
		Company:     s.getCompanyInfo(),
		AsOfDate:    asOfDate,
		Currency:    "IDR",
		Accounts:    []TrialBalanceItem{},
		GeneratedAt: time.Now(),
	}

	// Get all accounts with their balances from SSOT journal
	type AccountBalance struct {
		AccountID   uint64
		AccountCode string
		AccountName string
		AccountType string
		TotalDebit  decimal.Decimal
		TotalCredit decimal.Decimal
	}

	var balances []AccountBalance
	query := `
		SELECT 
			sjl.account_id,
			a.code as account_code,
			a.name as account_name,
			CASE 
				WHEN UPPER(TRIM(COALESCE(a.type, ''))) = 'ASSET' THEN 'Asset'
				WHEN UPPER(TRIM(COALESCE(a.type, ''))) = 'LIABILITY' THEN 'Liability'
				WHEN UPPER(TRIM(COALESCE(a.type, ''))) = 'EQUITY' THEN 'Equity'
				WHEN UPPER(TRIM(COALESCE(a.type, ''))) = 'REVENUE' THEN 'Revenue'
				WHEN UPPER(TRIM(COALESCE(a.type, ''))) = 'EXPENSE' THEN 'Expense'
				WHEN a.code LIKE '1%' THEN 'Asset'
				WHEN a.code LIKE '2%' THEN 'Liability'
				WHEN a.code LIKE '3%' THEN 'Equity'
				WHEN a.code LIKE '4%' THEN 'Revenue'
				WHEN a.code LIKE '5%' THEN 'Expense'
				WHEN a.code LIKE '6%' THEN 'Expense'
				WHEN a.code LIKE '7%' THEN 'Revenue'
				WHEN a.code LIKE '8%' THEN 'Expense'
				WHEN a.code LIKE '9%' THEN 'Expense'
				ELSE 'Other'
			END as account_type,
			COALESCE(SUM(sjl.debit_amount), 0) as total_debit,
			COALESCE(SUM(sjl.credit_amount), 0) as total_credit
		FROM unified_journal_lines sjl
		JOIN unified_journal_ledger sje ON sje.id = sjl.journal_id
		LEFT JOIN accounts a ON a.id = sjl.account_id
		WHERE DATE(sje.entry_date) <= ?
			AND sje.status = ?
			AND sje.deleted_at IS NULL
		GROUP BY sjl.account_id, a.code, a.name, a.type
		ORDER BY a.code
	`

	err := s.db.WithContext(ctx).Raw(query, asOfDate, models.SSOTStatusPosted).Scan(&balances).Error
	if err != nil {
		return nil, fmt.Errorf("failed to calculate trial balance: %w", err)
	}

	var totalDebits, totalCredits decimal.Decimal
	var accounts []TrialBalanceItem

	// Process balances
	for _, bal := range balances {
		item := TrialBalanceItem{
			AccountID:     uint(bal.AccountID),
			AccountCode:   bal.AccountCode,
			AccountName:   bal.AccountName,
			AccountType:   bal.AccountType,
			DebitBalance:  bal.TotalDebit.InexactFloat64(),
			CreditBalance: bal.TotalCredit.InexactFloat64(),
		}

		// Calculate net balance based on account type
		netBalance := bal.TotalDebit.Sub(bal.TotalCredit)

		// Create item without pre-assigning debit/credit balances here
		item = TrialBalanceItem{
			AccountID:   uint(bal.AccountID),
			AccountCode: bal.AccountCode,
			AccountName: bal.AccountName,
			AccountType: bal.AccountType,
		}

		// Calculate initial debit/credit balances as raw values
		var debitAmount, creditAmount float64

		// For display purposes, show balance in normal side
		switch bal.AccountType {
		case "Asset", "Expense":
			// Normal debit accounts
			if netBalance.GreaterThanOrEqual(decimal.Zero) {
				debitAmount = netBalance.InexactFloat64()
				creditAmount = 0
			} else {
				debitAmount = 0
				creditAmount = netBalance.Abs().InexactFloat64()
			}
		case "Liability", "Equity", "Revenue":
			// Normal credit accounts
			netBalance = netBalance.Neg() // Reverse for credit accounts
			if netBalance.GreaterThanOrEqual(decimal.Zero) {
				creditAmount = netBalance.InexactFloat64()
				debitAmount = 0
			} else {
				creditAmount = 0
				debitAmount = netBalance.Abs().InexactFloat64()
			}
		}

		// Handle negative balances by swapping sides using math.Abs
		if debitAmount < 0 {
			creditAmount += math.Abs(debitAmount)
			debitAmount = 0
		}
		if creditAmount < 0 {
			debitAmount += math.Abs(creditAmount)
			creditAmount = 0
		}

		// Assign final values to item
		item.DebitBalance = debitAmount
		item.CreditBalance = creditAmount

		totalDebits = totalDebits.Add(decimal.NewFromFloat(debitAmount))
		totalCredits = totalCredits.Add(decimal.NewFromFloat(creditAmount))
		accounts = append(accounts, item)
	}

	result.Accounts = accounts
	result.TotalDebits, _ = totalDebits.Float64()
	result.TotalCredits, _ = totalCredits.Float64()
	result.IsBalanced = totalDebits.Equal(totalCredits)
	result.Difference, _ = totalDebits.Sub(totalCredits).Float64()

	return result, nil
}

func (s *SSOTReportIntegrationService) generateGeneralLedger(ctx context.Context, startDate, endDate time.Time, accountID *uint64) (*GeneralLedgerData, error) {
	result := &GeneralLedgerData{
		Company:        s.getCompanyInfo(),
		StartDate:      startDate,
		EndDate:        endDate,
		Currency:       "IDR",
		OpeningBalance: 0.0,
		ClosingBalance: 0.0,
		TotalDebits:    0.0,
		TotalCredits:   0.0,
		Transactions:   []GeneralLedgerEntry{},
		MonthlySummary: []MonthlyLedgerSummary{},
		GeneratedAt:    time.Now(),
	}

	if accountID != nil {
		var selectedAccount models.Account
		if err := s.db.WithContext(ctx).
			Where("id = ? AND deleted_at IS NULL", *accountID).
			First(&selectedAccount).Error; err == nil {
			result.Account = selectedAccount
		}
	}

	// Build query for general ledger
	type ledgerRow struct {
		JournalID    uint64
		EntryNumber  string
		EntryDate    time.Time
		Description  string
		Reference    string
		AccountID    uint64
		AccountCode  string
		AccountName  string
		DebitAmount  decimal.Decimal
		CreditAmount decimal.Decimal
		Status       string
		SourceType   string
	}

	var query string
	var args []interface{}

	if accountID != nil {
		// Query ONLY lines for the selected account
		// This gives proper per-account debit/credit/balance view (Buku Besar)
		query = `
			SELECT
				sje.id as journal_id,
				sje.entry_number,
				sje.entry_date,
				sjl.description,
				sje.reference,
				sjl.account_id,
				a.code as account_code,
				a.name as account_name,
				sjl.debit_amount,
				sjl.credit_amount,
				sje.status,
				sje.source_type
			FROM unified_journal_lines sjl
			JOIN unified_journal_ledger sje ON sje.id = sjl.journal_id
			LEFT JOIN accounts a ON a.id = sjl.account_id
			WHERE sje.status = ?
				AND DATE(sje.entry_date) BETWEEN ? AND ?
				AND sje.deleted_at IS NULL
				AND sjl.account_id = ?
			ORDER BY sje.entry_date, sje.id, sjl.line_number
		`
		args = []interface{}{models.SSOTStatusPosted, startDate, endDate, *accountID}
	} else {
		query = `
			SELECT 
				sje.id as journal_id,
				sje.entry_number,
				sje.entry_date,
				sjl.description,
				sje.reference,
				sjl.account_id,
				a.code as account_code,
				a.name as account_name,
				sjl.debit_amount,
				sjl.credit_amount,
				sje.status,
				sje.source_type
			FROM unified_journal_lines sjl
			JOIN unified_journal_ledger sje ON sje.id = sjl.journal_id
			LEFT JOIN accounts a ON a.id = sjl.account_id
			WHERE sje.status = ? 
				AND DATE(sje.entry_date) BETWEEN ? AND ?
				AND sje.deleted_at IS NULL
			ORDER BY sje.entry_date, sje.id, sjl.line_number
		`
		args = []interface{}{models.SSOTStatusPosted, startDate, endDate}
	}

	var rows []ledgerRow
	err := s.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query general ledger: %w", err)
	}

	// DEBUG LOGGING
	log.Printf("[DEBUG GL] Query executed - Rows returned: %d", len(rows))
	log.Printf("[DEBUG GL] Date range: %s to %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	log.Printf("[DEBUG GL] Status filter: %s", models.SSOTStatusPosted)
	if accountID != nil {
		log.Printf("[DEBUG GL] Account ID filter: %d", *accountID)
	} else {
		log.Printf("[DEBUG GL] No account filter (showing all)")
	}
	if len(rows) == 0 {
		log.Printf("[DEBUG GL] ⚠️  WARNING: No rows returned from query!")
	}

	// Group by account for organized display
	accountEntries := make(map[uint64][]GeneralLedgerEntry)
	accountInfo := make(map[uint64]struct{ Code, Name string })

	for _, row := range rows {
		debitAmt := row.DebitAmount.InexactFloat64()
		creditAmt := row.CreditAmount.InexactFloat64()

		if debitAmt > 0 && creditAmt > 0 {
			netDiff := debitAmt - creditAmt
			if netDiff > 0 {
				debitAmt = netDiff
				creditAmt = 0
			} else if netDiff < 0 {
				creditAmt = -netDiff
				debitAmt = 0
			} else {
				debitAmt = 0
				creditAmt = 0
			}
		}

		entry := GeneralLedgerEntry{
			JournalID:    row.JournalID,
			Date:         row.EntryDate,
			JournalCode:  row.EntryNumber,
			Description:  row.Description,
			Reference:    row.Reference,
			AccountCode:  row.AccountCode,
			AccountName:  row.AccountName,
			DebitAmount:  debitAmt,
			CreditAmount: creditAmt,
			EntryType:    row.SourceType,
		}

		accountEntries[row.AccountID] = append(accountEntries[row.AccountID], entry)
		accountInfo[row.AccountID] = struct{ Code, Name string }{
			Code: row.AccountCode,
			Name: row.AccountName,
		}
	}

	// Calculate opening balance (balance before start date) for all accounts
	var openingBalance decimal.Decimal
	var openingQuery string
	var openingArgs []interface{}

	if accountID != nil {
		// Opening balance for specific account
		openingQuery = `
			SELECT 
				COALESCE(SUM(sjl.debit_amount), 0) - COALESCE(SUM(sjl.credit_amount), 0) as balance
			FROM unified_journal_lines sjl
			JOIN unified_journal_ledger sje ON sje.id = sjl.journal_id
			WHERE sje.status = ? 
				AND DATE(sje.entry_date) < ?
				AND sje.deleted_at IS NULL
				AND sjl.account_id = ?
		`
		openingArgs = []interface{}{models.SSOTStatusPosted, startDate, *accountID}
	} else {
		// Opening balance for all accounts
		openingQuery = `
			SELECT 
				COALESCE(SUM(sjl.debit_amount), 0) - COALESCE(SUM(sjl.credit_amount), 0) as balance
			FROM unified_journal_lines sjl
			JOIN unified_journal_ledger sje ON sje.id = sjl.journal_id
			WHERE sje.status = ? 
				AND DATE(sje.entry_date) < ?
				AND sje.deleted_at IS NULL
		`
		openingArgs = []interface{}{models.SSOTStatusPosted, startDate}
	}

	var openingBalanceFloat float64
	if err := s.db.WithContext(ctx).Raw(openingQuery, openingArgs...).Scan(&openingBalanceFloat).Error; err != nil {
		log.Printf("[DEBUG GL] Warning: Failed to calculate opening balance: %v", err)
		openingBalance = decimal.Zero
	} else {
		openingBalance = decimal.NewFromFloat(openingBalanceFloat)
		log.Printf("[DEBUG GL] Opening balance calculated: %s", openingBalance.String())
	}

	// Calculate running balances, organize entries, and compute totals
	var totalDebits, totalCredits decimal.Decimal
	accountBalances := make(map[uint64]decimal.Decimal)        // Track final balance per account
	accountOpeningBalances := make(map[uint64]decimal.Decimal) // Track opening balance per account

	// Calculate opening balance for each account if showing all accounts
	if accountID == nil {
		for accID := range accountEntries {
			var accOpeningQuery = `
				SELECT 
					COALESCE(SUM(sjl.debit_amount), 0) - COALESCE(SUM(sjl.credit_amount), 0) as balance
				FROM unified_journal_lines sjl
				JOIN unified_journal_ledger sje ON sje.id = sjl.journal_id
				WHERE sje.status = ? 
					AND DATE(sje.entry_date) < ?
					AND sje.deleted_at IS NULL
					AND sjl.account_id = ?
			`
			var accOpeningFloat float64
			if err := s.db.WithContext(ctx).Raw(accOpeningQuery, models.SSOTStatusPosted, startDate, accID).Scan(&accOpeningFloat).Error; err != nil {
				accountOpeningBalances[accID] = decimal.Zero
			} else {
				accountOpeningBalances[accID] = decimal.NewFromFloat(accOpeningFloat)
			}
		}
	}

	for accID, entries := range accountEntries {
		info := accountInfo[accID]
		runningBalance := accountOpeningBalances[accID] // Start with opening balance for this account

		for i, entry := range entries {
			// Add to overall totals
			totalDebits = totalDebits.Add(decimal.NewFromFloat(entry.DebitAmount))
			totalCredits = totalCredits.Add(decimal.NewFromFloat(entry.CreditAmount))

			// Calculate running balance for this entry
			runningBalance = runningBalance.Add(decimal.NewFromFloat(entry.DebitAmount)).Sub(decimal.NewFromFloat(entry.CreditAmount))
			entries[i].Balance = runningBalance.InexactFloat64()
		}

		// Store final balance for this account
		accountBalances[accID] = runningBalance

		// Add account header info to description if needed
		if len(entries) > 0 {
			accountHeader := fmt.Sprintf("Account: %s - %s", info.Code, info.Name)
			entries[0].Description = accountHeader + " | " + entries[0].Description
		}

		result.Transactions = append(result.Transactions, entries...)
	}

	// Sort transactions by date, then by journal_id to ensure same journal entries are grouped together
	sort.Slice(result.Transactions, func(i, j int) bool {
		// First, sort by date
		if !result.Transactions[i].Date.Equal(result.Transactions[j].Date) {
			return result.Transactions[i].Date.Before(result.Transactions[j].Date)
		}
		// Then by journal_id to group same journal entries together
		if result.Transactions[i].JournalID != result.Transactions[j].JournalID {
			return result.Transactions[i].JournalID < result.Transactions[j].JournalID
		}
		// Finally by debit (debit first, then credit) for proper journal display
		return result.Transactions[i].DebitAmount > result.Transactions[j].DebitAmount
	})

	// Recalculate running balance only for a single-account ledger.
	// In all-accounts activity mode each entry should retain its own account balance,
	// not a synthetic balance aggregated across unrelated accounts.
	if accountID != nil {
		runningBalance := openingBalance
		for i := range result.Transactions {
			runningBalance = runningBalance.Add(decimal.NewFromFloat(result.Transactions[i].DebitAmount)).Sub(decimal.NewFromFloat(result.Transactions[i].CreditAmount))
			result.Transactions[i].Balance = runningBalance.InexactFloat64()
		}
	}

	// Calculate enhanced UI metrics
	var totalAccountBalances decimal.Decimal
	var cashAccountBalance decimal.Decimal

	// Sum all account balances and calculate cash impact
	for accID, balance := range accountBalances {
		totalAccountBalances = totalAccountBalances.Add(balance)

		// Calculate cash impact (accounts starting with 11xx are typically cash accounts)
		if info, exists := accountInfo[accID]; exists {
			if strings.HasPrefix(info.Code, "11") { // Cash and cash equivalents
				cashAccountBalance = cashAccountBalance.Add(balance)
			}
		}
	}

	// Calculate proper balances
	netChange := totalDebits.Sub(totalCredits)
	closingBalance := openingBalance.Add(netChange)
	totalTransactionVolume := totalDebits.InexactFloat64() // Total transaction activity
	cashImpact := cashAccountBalance.InexactFloat64()
	isBalanced := totalDebits.Equal(totalCredits)

	// Determine status messages
	var netPositionStatus string
	if netChange.IsZero() {
		netPositionStatus = "Balanced"
	} else if netChange.IsPositive() {
		netPositionStatus = "Net Debit Position"
	} else {
		netPositionStatus = "Net Credit Position"
	}

	var cashImpactStatus string
	if cashImpact > 0 {
		cashImpactStatus = "Cash Increased"
	} else if cashImpact < 0 {
		cashImpactStatus = "Cash Decreased"
	} else {
		cashImpactStatus = "No Cash Impact"
	}

	// Update result with calculated totals and enhanced UI fields
	result.TotalDebits = totalDebits.InexactFloat64()
	result.TotalCredits = totalCredits.InexactFloat64()
	result.OpeningBalance = openingBalance.InexactFloat64()
	result.ClosingBalance = closingBalance.InexactFloat64()
	result.NetPositionChange = netChange.InexactFloat64()
	result.NetPositionStatus = netPositionStatus
	result.TotalTransactionVol = totalTransactionVolume
	result.CashImpact = cashImpact
	result.CashImpactStatus = cashImpactStatus
	result.IsBalanced = isBalanced

	log.Printf("[DEBUG GL] Calculated balances - Opening: %s, Debits: %s, Credits: %s, Net: %s, Closing: %s",
		openingBalance.String(), totalDebits.String(), totalCredits.String(), netChange.String(), closingBalance.String())

	return result, nil
}

func (s *SSOTReportIntegrationService) generateJournalAnalysis(ctx context.Context, startDate, endDate time.Time) (*JournalAnalysisData, error) {
	// Get basic statistics - using POSTED entries for total amount calculation
	var stats struct {
		TotalEntries    int64           `json:"total_entries"`
		PostedEntries   int64           `json:"posted_entries"`
		DraftEntries    int64           `json:"draft_entries"`
		ReversedEntries int64           `json:"reversed_entries"`
		TotalAmount     decimal.Decimal `json:"total_amount"`
	}

	if err := s.db.WithContext(ctx).Model(&models.SSOTJournalEntry{}).
		Select(`
			COUNT(*) as total_entries,
			COUNT(CASE WHEN status = ? THEN 1 END) as posted_entries,
			COUNT(CASE WHEN status = ? THEN 1 END) as draft_entries,
			COUNT(CASE WHEN status = ? THEN 1 END) as reversed_entries,
			COALESCE(SUM(CASE WHEN status = ? THEN total_debit ELSE 0 END), 0) as total_amount
		`, models.SSOTStatusPosted, models.SSOTStatusDraft, models.SSOTStatusReversed, models.SSOTStatusPosted).
		Where("entry_date >= ? AND entry_date <= ?", startDate, endDate).
		Scan(&stats).Error; err != nil {
		return nil, fmt.Errorf("failed to get journal statistics: %w", err)
	}

	// Get entries by type with user-friendly labels
	var entriesByTypeRaw []struct {
		SourceType  string          `json:"source_type"`
		Count       int64           `json:"count"`
		TotalAmount decimal.Decimal `json:"total_amount"`
	}

	s.db.WithContext(ctx).Model(&models.SSOTJournalEntry{}).
		Select("source_type, COUNT(*) as count, COALESCE(SUM(total_debit), 0) as total_amount").
		Where("entry_date >= ? AND entry_date <= ? AND status = ?", startDate, endDate, models.SSOTStatusPosted).
		Group("source_type").
		Scan(&entriesByTypeRaw)

	// Map source types to user-friendly labels and calculate percentages
	var entriesByType []EntryTypeBreakdown
	for _, entry := range entriesByTypeRaw {
		friendlyType := s.mapSourceTypeToFriendlyName(entry.SourceType)
		percentage := 0.0
		if stats.TotalAmount.GreaterThan(decimal.Zero) {
			percentage = entry.TotalAmount.Div(stats.TotalAmount).Mul(decimal.NewFromInt(100)).InexactFloat64()
		}

		entriesByType = append(entriesByType, EntryTypeBreakdown{
			SourceType:  friendlyType,
			Count:       entry.Count,
			TotalAmount: entry.TotalAmount,
			Percentage:  percentage,
		})
	}

	// Get entries by account (implement the missing feature)
	entriesByAccount := s.generateAccountBreakdown(ctx, startDate, endDate)

	// Get entries by period (implement the missing feature)
	entriesByPeriod := s.generatePeriodBreakdown(ctx, startDate, endDate)

	// Generate compliance and quality checks
	complianceCheck := s.generateComplianceCheck(ctx, startDate, endDate)
	dataQualityMetrics := s.generateDataQualityMetrics(ctx, startDate, endDate)

	return &JournalAnalysisData{
		Company:            s.getCompanyInfo(),
		StartDate:          startDate,
		EndDate:            endDate,
		Currency:           s.getCurrencyFromSettings(),
		TotalEntries:       stats.TotalEntries,
		PostedEntries:      stats.PostedEntries,
		DraftEntries:       stats.DraftEntries,
		ReversedEntries:    stats.ReversedEntries,
		TotalAmount:        stats.TotalAmount,
		EntriesByType:      entriesByType,
		EntriesByAccount:   entriesByAccount,
		EntriesByPeriod:    entriesByPeriod,
		ComplianceCheck:    complianceCheck,
		DataQualityMetrics: dataQualityMetrics,
		GeneratedAt:        time.Now(),
	}, nil
}
