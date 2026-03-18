package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"app-sistem-akuntansi/models"
	"gorm.io/gorm"
)

// PeriodArchiveService handles period archive and snapshot management
type PeriodArchiveService struct {
	db                  *gorm.DB
	balanceSheetService *SSOTBalanceSheetService
	profitLossService   *SSOTProfitLossService
}

// NewPeriodArchiveService creates a new period archive service
func NewPeriodArchiveService(db *gorm.DB) *PeriodArchiveService {
	return &PeriodArchiveService{
		db:                  db,
		balanceSheetService: NewSSOTBalanceSheetService(db),
		profitLossService:   NewSSOTProfitLossService(db),
	}
}

// PeriodArchiveListItem represents a summary of archived period
type PeriodArchiveListItem struct {
	ID               uint       `json:"id"`
	StartDate        time.Time  `json:"start_date"`
	EndDate          time.Time  `json:"end_date"`
	Description      string     `json:"description"`
	PeriodType       string     `json:"period_type"`
	FiscalYear       *int       `json:"fiscal_year"`
	TotalRevenue     float64    `json:"total_revenue"`
	TotalExpense     float64    `json:"total_expense"`
	NetIncome        float64    `json:"net_income"`
	AccountCount     int        `json:"account_count"`
	TransactionCount int        `json:"transaction_count"`
	IsClosed         bool       `json:"is_closed"`
	IsLocked         bool       `json:"is_locked"`
	ClosedBy         *uint      `json:"closed_by"`
	ClosedAt         *time.Time `json:"closed_at"`
	ClosedByName     string     `json:"closed_by_name,omitempty"`
	HasSnapshot      bool       `json:"has_snapshot"`
	SnapshotDate     *time.Time `json:"snapshot_generated_at"`
}

// PeriodArchiveDetail represents full details of archived period with snapshots
type PeriodArchiveDetail struct {
	Period           models.AccountingPeriod `json:"period"`
	BalanceSheet     *SSOTBalanceSheetData   `json:"balance_sheet,omitempty"`
	ProfitLoss       *SSOTProfitLossData     `json:"profit_loss,omitempty"`
	FinancialMetrics map[string]interface{}  `json:"financial_metrics,omitempty"`
	CanRegenerate    bool                    `json:"can_regenerate"`
}

// PeriodComparisonResult represents comparison between two periods
type PeriodComparisonResult struct {
	FromPeriod       *PeriodArchiveListItem `json:"from_period"`
	ToPeriod         *PeriodArchiveListItem `json:"to_period"`
	RevenueChange    float64                `json:"revenue_change"`
	RevenueChangeP   float64                `json:"revenue_change_percent"`
	ExpenseChange    float64                `json:"expense_change"`
	ExpenseChangeP   float64                `json:"expense_change_percent"`
	NetIncomeChange  float64                `json:"net_income_change"`
	NetIncomeChangeP float64                `json:"net_income_change_percent"`
	ComparisonDate   time.Time              `json:"comparison_date"`
}

// CreatePeriodSnapshot generates and saves snapshots for a closed period
func (s *PeriodArchiveService) CreatePeriodSnapshot(ctx context.Context, periodID uint) error {
	db := s.db.WithContext(ctx)

	// Get the period
	var period models.AccountingPeriod
	if err := db.First(&period, periodID).Error; err != nil {
		return fmt.Errorf("period not found: %v", err)
	}

	if !period.IsClosed {
		return fmt.Errorf("cannot create snapshot for open period")
	}

	// Generate Balance Sheet snapshot
	bsData, err := s.balanceSheetService.GenerateSSOTBalanceSheet(period.EndDate.Format("2006-01-02"))
	if err != nil {
		return fmt.Errorf("failed to generate balance sheet: %v", err)
	}

	// Generate Profit & Loss snapshot
	plData, err := s.profitLossService.GenerateSSOTProfitLoss(
		period.StartDate.Format("2006-01-02"),
		period.EndDate.Format("2006-01-02"),
	)
	if err != nil {
		return fmt.Errorf("failed to generate profit loss: %v", err)
	}

	// Convert to JSON maps
	bsJSON, err := structToJSONMap(bsData)
	if err != nil {
		return fmt.Errorf("failed to serialize balance sheet: %v", err)
	}

	plJSON, err := structToJSONMap(plData)
	if err != nil {
		return fmt.Errorf("failed to serialize profit loss: %v", err)
	}

	// Calculate financial metrics
	metrics := s.calculateFinancialMetrics(plData, bsData)
	metricsJSON := models.JSONMap(metrics)

	// Update period with snapshots
	now := time.Now()
	updates := map[string]interface{}{
		"balance_sheet_snapshot": bsJSON,
		"profit_loss_snapshot":   plJSON,
		"financial_metrics":      metricsJSON,
		"snapshot_generated_at":  now,
	}

	if err := db.Model(&period).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to save snapshot: %v", err)
	}

	return nil
}

// GetPeriodArchiveList returns list of archived periods with optional filters
func (s *PeriodArchiveService) GetPeriodArchiveList(ctx context.Context, filters map[string]interface{}) ([]PeriodArchiveListItem, error) {
	var periods []models.AccountingPeriod
	query := s.db.Preload("ClosedByUser").Where("is_closed = ?", true)

	// Apply filters
	if fiscalYear, ok := filters["fiscal_year"].(int); ok {
		query = query.Where("fiscal_year = ?", fiscalYear)
	}
	if periodType, ok := filters["period_type"].(string); ok {
		query = query.Where("period_type = ?", periodType)
	}
	if year, ok := filters["year"].(int); ok {
		query = query.Where("EXTRACT(YEAR FROM end_date) = ?", year)
	}

	// Order by end_date descending (most recent first)
	if err := query.Order("end_date DESC").Find(&periods).Error; err != nil {
		return nil, fmt.Errorf("failed to get period list: %v", err)
	}

	// Convert to list items
	result := make([]PeriodArchiveListItem, len(periods))
	for i, p := range periods {
		result[i] = PeriodArchiveListItem{
			ID:               p.ID,
			StartDate:        p.StartDate,
			EndDate:          p.EndDate,
			Description:      p.Description,
			PeriodType:       p.PeriodType,
			FiscalYear:       p.FiscalYear,
			TotalRevenue:     p.TotalRevenue,
			TotalExpense:     p.TotalExpense,
			NetIncome:        p.NetIncome,
			AccountCount:     p.AccountCount,
			TransactionCount: p.TransactionCount,
			IsClosed:         p.IsClosed,
			IsLocked:         p.IsLocked,
			ClosedBy:         p.ClosedBy,
			ClosedAt:         p.ClosedAt,
			HasSnapshot:      p.BalanceSheetSnapshot != nil,
			SnapshotDate:     p.SnapshotGeneratedAt,
		}

		if p.ClosedByUser != nil {
			result[i].ClosedByName = p.ClosedByUser.GetDisplayName()
		}
	}

	return result, nil
}

// GetPeriodArchiveDetail returns full details of an archived period including snapshots
func (s *PeriodArchiveService) GetPeriodArchiveDetail(ctx context.Context, periodID uint) (*PeriodArchiveDetail, error) {
	var period models.AccountingPeriod
	if err := s.db.Preload("ClosedByUser").Preload("ClosingJournal").First(&period, periodID).Error; err != nil {
		return nil, fmt.Errorf("period not found: %v", err)
	}

	detail := &PeriodArchiveDetail{
		Period:        period,
		CanRegenerate: period.IsClosed,
	}

	// Parse Balance Sheet snapshot if exists
	if period.BalanceSheetSnapshot != nil {
		var bsData SSOTBalanceSheetData
		if err := jsonMapToStruct(*period.BalanceSheetSnapshot, &bsData); err == nil {
			detail.BalanceSheet = &bsData
		}
	}

	// Parse Profit & Loss snapshot if exists
	if period.ProfitLossSnapshot != nil {
		var plData SSOTProfitLossData
		if err := jsonMapToStruct(*period.ProfitLossSnapshot, &plData); err == nil {
			detail.ProfitLoss = &plData
		}
	}

	// Parse Financial Metrics if exists
	if period.FinancialMetrics != nil {
		detail.FinancialMetrics = map[string]interface{}(*period.FinancialMetrics)
	}

	return detail, nil
}

// ComparePeriods compares two archived periods
func (s *PeriodArchiveService) ComparePeriods(ctx context.Context, fromPeriodID, toPeriodID uint) (*PeriodComparisonResult, error) {
	// Get both periods
	var fromPeriod, toPeriod models.AccountingPeriod

	if err := s.db.First(&fromPeriod, fromPeriodID).Error; err != nil {
		return nil, fmt.Errorf("from period not found: %v", err)
	}
	if err := s.db.First(&toPeriod, toPeriodID).Error; err != nil {
		return nil, fmt.Errorf("to period not found: %v", err)
	}

	// Create summary items
	fromItem := &PeriodArchiveListItem{
		ID:           fromPeriod.ID,
		StartDate:    fromPeriod.StartDate,
		EndDate:      fromPeriod.EndDate,
		Description:  fromPeriod.Description,
		PeriodType:   fromPeriod.PeriodType,
		TotalRevenue: fromPeriod.TotalRevenue,
		TotalExpense: fromPeriod.TotalExpense,
		NetIncome:    fromPeriod.NetIncome,
	}

	toItem := &PeriodArchiveListItem{
		ID:           toPeriod.ID,
		StartDate:    toPeriod.StartDate,
		EndDate:      toPeriod.EndDate,
		Description:  toPeriod.Description,
		PeriodType:   toPeriod.PeriodType,
		TotalRevenue: toPeriod.TotalRevenue,
		TotalExpense: toPeriod.TotalExpense,
		NetIncome:    toPeriod.NetIncome,
	}

	// Calculate changes
	result := &PeriodComparisonResult{
		FromPeriod:      fromItem,
		ToPeriod:        toItem,
		RevenueChange:   toPeriod.TotalRevenue - fromPeriod.TotalRevenue,
		ExpenseChange:   toPeriod.TotalExpense - fromPeriod.TotalExpense,
		NetIncomeChange: toPeriod.NetIncome - fromPeriod.NetIncome,
		ComparisonDate:  time.Now(),
	}

	// Calculate percentage changes
	if fromPeriod.TotalRevenue != 0 {
		result.RevenueChangeP = (result.RevenueChange / fromPeriod.TotalRevenue) * 100
	}
	if fromPeriod.TotalExpense != 0 {
		result.ExpenseChangeP = (result.ExpenseChange / fromPeriod.TotalExpense) * 100
	}
	if fromPeriod.NetIncome != 0 {
		result.NetIncomeChangeP = (result.NetIncomeChange / fromPeriod.NetIncome) * 100
	}

	return result, nil
}

// RegeneratePeriodSnapshot regenerates snapshots for an existing period
func (s *PeriodArchiveService) RegeneratePeriodSnapshot(ctx context.Context, periodID uint) error {
	return s.CreatePeriodSnapshot(ctx, periodID)
}

// DeletePeriodArchive deletes a period archive (soft delete)
func (s *PeriodArchiveService) DeletePeriodArchive(ctx context.Context, periodID uint) error {
	var period models.AccountingPeriod
	if err := s.db.First(&period, periodID).Error; err != nil {
		return fmt.Errorf("period not found: %v", err)
	}

	if period.IsLocked {
		return fmt.Errorf("cannot delete locked period")
	}

	return s.db.Delete(&period).Error
}

// calculateFinancialMetrics calculates key financial metrics from P&L and Balance Sheet
func (s *PeriodArchiveService) calculateFinancialMetrics(pl *SSOTProfitLossData, bs *SSOTBalanceSheetData) map[string]interface{} {
	metrics := map[string]interface{}{
		// Profitability Ratios
		"gross_profit_margin": pl.GrossProfitMargin,
		"operating_margin":    pl.OperatingMargin,
		"net_profit_margin":   pl.NetIncomeMargin,

		// Amounts
		"gross_profit":     pl.GrossProfit,
		"operating_income": pl.OperatingIncome,
		"net_income":       pl.NetIncome,
		"total_revenue":    pl.Revenue.TotalRevenue,
		"total_cogs":       pl.COGS.TotalCOGS,
		"total_opex":       pl.OperatingExpenses.TotalOpEx,

		// Balance Sheet Ratios
		"current_ratio":        0.0,
		"quick_ratio":          0.0,
		"debt_to_equity_ratio": 0.0,

		// Balance Sheet Amounts
		"total_assets":        bs.Assets.TotalAssets,
		"total_liabilities":   bs.Liabilities.TotalLiabilities,
		"total_equity":        bs.Equity.TotalEquity,
		"current_assets":      bs.Assets.CurrentAssets.TotalCurrentAssets,
		"current_liabilities": bs.Liabilities.CurrentLiabilities.TotalCurrentLiabilities,
	}

	// Calculate Current Ratio = Current Assets / Current Liabilities
	if bs.Liabilities.CurrentLiabilities.TotalCurrentLiabilities != 0 {
		metrics["current_ratio"] = bs.Assets.CurrentAssets.TotalCurrentAssets / bs.Liabilities.CurrentLiabilities.TotalCurrentLiabilities
	}

	// Calculate Quick Ratio = (Current Assets - Inventory) / Current Liabilities
	quickAssets := bs.Assets.CurrentAssets.TotalCurrentAssets - bs.Assets.CurrentAssets.Inventory
	if bs.Liabilities.CurrentLiabilities.TotalCurrentLiabilities != 0 {
		metrics["quick_ratio"] = quickAssets / bs.Liabilities.CurrentLiabilities.TotalCurrentLiabilities
	}

	// Calculate Debt to Equity Ratio = Total Liabilities / Total Equity
	if bs.Equity.TotalEquity != 0 {
		metrics["debt_to_equity_ratio"] = bs.Liabilities.TotalLiabilities / bs.Equity.TotalEquity
	}

	// Calculate ROA (Return on Assets) = Net Income / Total Assets
	if bs.Assets.TotalAssets != 0 {
		metrics["roa"] = (pl.NetIncome / bs.Assets.TotalAssets) * 100
	}

	// Calculate ROE (Return on Equity) = Net Income / Total Equity
	if bs.Equity.TotalEquity != 0 {
		metrics["roe"] = (pl.NetIncome / bs.Equity.TotalEquity) * 100
	}

	return metrics
}

// Helper functions
func structToJSONMap(v interface{}) (*models.JSONMap, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var m models.JSONMap
	if err := json.Unmarshal(bytes, &m); err != nil {
		return nil, err
	}

	return &m, nil
}

func jsonMapToStruct(m models.JSONMap, v interface{}) error {
	bytes, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, v)
}
