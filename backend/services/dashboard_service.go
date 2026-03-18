package services

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type DashboardService struct {
	DB *gorm.DB
}

func NewDashboardService(db *gorm.DB) *DashboardService {
	return &DashboardService{DB: db}
}

// AnalyticsData represents the complete analytics data with growth calculations
type AnalyticsData struct {
	TotalSales         float64 `json:"totalSales"`
	TotalPurchases     float64 `json:"totalPurchases"`
	AccountsReceivable float64 `json:"accountsReceivable"`
	AccountsPayable    float64 `json:"accountsPayable"`

	// Growth percentages
	SalesGrowth       float64 `json:"salesGrowth"`
	PurchasesGrowth   float64 `json:"purchasesGrowth"`
	ReceivablesGrowth float64 `json:"receivablesGrowth"`
	PayablesGrowth    float64 `json:"payablesGrowth"`

	// Monthly data
	MonthlySales       []MonthlyData           `json:"monthlySales"`
	MonthlyPurchases   []MonthlyData           `json:"monthlyPurchases"`
	CashFlow           []DashboardCashFlowData `json:"cashFlow"`
	TopAccounts        []AccountData           `json:"topAccounts"`
	RecentTransactions []TransactionData       `json:"recentTransactions"`
}

type MonthlyData struct {
	Month string  `json:"month"`
	Value float64 `json:"value"`
}

type DashboardCashFlowData struct {
	Month   string  `json:"month"`
	Inflow  float64 `json:"inflow"`
	Outflow float64 `json:"outflow"`
	Balance float64 `json:"balance"`
}

type monthlyValueQueryResult struct {
	Period string  `json:"period"`
	Month  string  `json:"month"`
	Value  float64 `json:"value"`
}

type monthlyCashFlowQueryResult struct {
	Period  string  `json:"period"`
	Month   string  `json:"month"`
	Inflow  float64 `json:"inflow"`
	Outflow float64 `json:"outflow"`
}

type AccountData struct {
	Name    string  `json:"name"`
	Balance float64 `json:"balance"`
	Type    string  `json:"type"`
}

type TransactionData struct {
	ID            uint    `json:"id"`
	TransactionID string  `json:"transaction_id"`
	Description   string  `json:"description"`
	Amount        float64 `json:"amount"`
	Date          string  `json:"date"`
	Type          string  `json:"type"`
	AccountName   string  `json:"account_name"`
	ContactName   *string `json:"contact_name"`
	Status        string  `json:"status"`
}

// GetDashboardAnalytics returns comprehensive dashboard analytics with real growth calculations
func (ds *DashboardService) GetDashboardAnalytics() (*AnalyticsData, error) {
	return ds.GetDashboardAnalyticsForRole("")
}

// GetDashboardAnalyticsForRole returns dashboard analytics filtered by user role
func (ds *DashboardService) GetDashboardAnalyticsForRole(role string) (*AnalyticsData, error) {
	analytics := &AnalyticsData{}

	// Get current period data (this month)
	now := time.Now()
	currentStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	currentEnd := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location()).AddDate(0, 0, -1).Format("2006-01-02")

	// Get previous period data (last month)
	lastMonth := now.AddDate(0, -1, 0)
	previousStart := time.Date(lastMonth.Year(), lastMonth.Month(), 1, 0, 0, 0, 0, lastMonth.Location()).Format("2006-01-02")
	previousEnd := time.Date(lastMonth.Year(), lastMonth.Month()+1, 1, 0, 0, 0, 0, lastMonth.Location()).AddDate(0, 0, -1).Format("2006-01-02")

	// Calculate current totals
	currentTotals, err := ds.getCurrentPeriodTotals(currentStart, currentEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to get current period totals: %v", err)
	}

	// Calculate previous totals for growth comparison
	previousTotals, err := ds.getCurrentPeriodTotals(previousStart, previousEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to get previous period totals: %v", err)
	}

	// Set current values
	analytics.TotalSales = currentTotals["sales"]
	analytics.TotalPurchases = currentTotals["purchases"]
	analytics.AccountsReceivable = currentTotals["receivables"]
	analytics.AccountsPayable = currentTotals["payables"]

	// Calculate growth percentages
	analytics.SalesGrowth = ds.calculateGrowthPercentage(previousTotals["sales"], currentTotals["sales"])
	analytics.PurchasesGrowth = ds.calculateGrowthPercentage(previousTotals["purchases"], currentTotals["purchases"])
	analytics.ReceivablesGrowth = ds.calculateGrowthPercentage(previousTotals["receivables"], currentTotals["receivables"])
	analytics.PayablesGrowth = ds.calculateGrowthPercentage(previousTotals["payables"], currentTotals["payables"])

	// Get monthly data for charts
	analytics.MonthlySales, err = ds.getMonthlySalesData()
	if err != nil {
		return nil, fmt.Errorf("failed to get monthly sales data: %v", err)
	}

	analytics.MonthlyPurchases, err = ds.getMonthlyPurchasesData()
	if err != nil {
		return nil, fmt.Errorf("failed to get monthly purchases data: %v", err)
	}

	// Calculate cash movement from recorded cash/bank transactions.
	analytics.CashFlow, err = ds.getMonthlyCashMovementData()
	if err != nil {
		return nil, fmt.Errorf("failed to get monthly cash movement data: %v", err)
	}

	// Get top accounts
	analytics.TopAccounts, err = ds.getTopAccounts()
	if err != nil {
		return nil, fmt.Errorf("failed to get top accounts: %v", err)
	}

	// Get recent transactions
	analytics.RecentTransactions, err = ds.getRecentTransactions()
	if err != nil {
		return nil, fmt.Errorf("failed to get recent transactions: %v", err)
	}

	return analytics, nil
}

// getCurrentPeriodTotals calculates totals for a given period
func (ds *DashboardService) getCurrentPeriodTotals(startDate, endDate string) (map[string]float64, error) {
	totals := make(map[string]float64)

	// Total sales for the period from approved/completed commercial documents.
	var totalSales float64
	err := ds.DB.Model(&models.Sale{}).
		Where("date >= ? AND date <= ? AND deleted_at IS NULL AND status IN (?)",
			startDate, endDate, []string{"CONFIRMED", "COMPLETED", "INVOICED", "OVERDUE", "PAID"}).
		Select("COALESCE(SUM(net_before_tax), 0)").
		Scan(&totalSales).Error
	if err != nil {
		return nil, err
	}
	totals["sales"] = totalSales

	// Total purchases for the period from approved/completed purchasing documents.
	var totalPurchases float64
	err = ds.DB.Model(&models.Purchase{}).
		Where("date >= ? AND date <= ? AND deleted_at IS NULL AND status IN (?)",
			startDate, endDate, []string{"APPROVED", "COMPLETED", "PAID"}).
		Select("COALESCE(SUM(net_before_tax), 0)").
		Scan(&totalPurchases).Error
	if err != nil {
		return nil, err
	}
	totals["purchases"] = totalPurchases

	// Accounts receivable from collectible sales only.
	var accountsReceivable float64
	err = ds.DB.Model(&models.Sale{}).
		Where("date >= ? AND date <= ? AND status IN (?) AND outstanding_amount > 0 AND deleted_at IS NULL",
			startDate, endDate, []string{"INVOICED", "OVERDUE"}).
		Select("COALESCE(SUM(outstanding_amount), 0)").
		Scan(&accountsReceivable).Error
	if err != nil {
		return nil, err
	}
	totals["receivables"] = accountsReceivable

	// Accounts payable from approved vendor bills only.
	var accountsPayable float64
	err = ds.DB.Model(&models.Purchase{}).
		Where("date >= ? AND date <= ? AND status IN (?) AND outstanding_amount > 0 AND deleted_at IS NULL",
			startDate, endDate, []string{"APPROVED", "COMPLETED", "PAID"}).
		Select("COALESCE(SUM(outstanding_amount), 0)").
		Scan(&accountsPayable).Error
	if err != nil {
		return nil, err
	}
	totals["payables"] = accountsPayable

	return totals, nil
}

// calculateGrowthPercentage calculates percentage growth between two periods
func (ds *DashboardService) calculateGrowthPercentage(previous, current float64) float64 {
	if previous == 0 {
		if current > 0 {
			return 100.0 // If previous was 0 and current > 0, it's 100% growth
		}
		return 0.0 // Both are 0, no growth
	}

	return ((current - previous) / previous) * 100
}

// getMonthlySalesData gets recognized sales data for the last 7 months.
func (ds *DashboardService) getMonthlySalesData() ([]MonthlyData, error) {
	var results []monthlyValueQueryResult
	err := ds.DB.Raw(`
		SELECT 
			TO_CHAR(DATE_TRUNC('month', date), 'YYYY-MM') as period,
			TO_CHAR(DATE_TRUNC('month', date), 'Mon') as month,
			COALESCE(SUM(net_before_tax), 0) as value
		FROM sales 
		WHERE date >= DATE_TRUNC('month', CURRENT_DATE) - INTERVAL '6 months'
			AND deleted_at IS NULL
			AND status IN ('CONFIRMED', 'COMPLETED', 'INVOICED', 'OVERDUE', 'PAID')
		GROUP BY DATE_TRUNC('month', date), TO_CHAR(DATE_TRUNC('month', date), 'YYYY-MM'), TO_CHAR(DATE_TRUNC('month', date), 'Mon')
		ORDER BY DATE_TRUNC('month', date)
	`).Scan(&results).Error

	if err != nil {
		return nil, err
	}

	return fillMonthlyValueWindow(results), nil
}

// getMonthlyPurchasesData gets approved purchase data for the last 7 months.
func (ds *DashboardService) getMonthlyPurchasesData() ([]MonthlyData, error) {
	var results []monthlyValueQueryResult
	err := ds.DB.Raw(`
		SELECT 
			TO_CHAR(DATE_TRUNC('month', date), 'YYYY-MM') as period,
			TO_CHAR(DATE_TRUNC('month', date), 'Mon') as month,
			COALESCE(SUM(net_before_tax), 0) as value
		FROM purchases 
		WHERE date >= DATE_TRUNC('month', CURRENT_DATE) - INTERVAL '6 months'
			AND deleted_at IS NULL
			AND status IN ('APPROVED', 'COMPLETED', 'PAID')
		GROUP BY DATE_TRUNC('month', date), TO_CHAR(DATE_TRUNC('month', date), 'YYYY-MM'), TO_CHAR(DATE_TRUNC('month', date), 'Mon')
		ORDER BY DATE_TRUNC('month', date)
	`).Scan(&results).Error

	if err != nil {
		return nil, err
	}

	return fillMonthlyValueWindow(results), nil
}

// getMonthlyCashMovementData gets enterprise cash movement for the last 7 months.
// Internal transfers and opening balances are excluded because they do not change total cash.
func (ds *DashboardService) getMonthlyCashMovementData() ([]DashboardCashFlowData, error) {
	var results []monthlyCashFlowQueryResult
	err := ds.DB.Raw(`
		SELECT
			TO_CHAR(DATE_TRUNC('month', transaction_date), 'YYYY-MM') as period,
			TO_CHAR(DATE_TRUNC('month', transaction_date), 'Mon') as month,
			COALESCE(SUM(CASE WHEN "book_amount_id_r" > 0 THEN "book_amount_id_r" ELSE 0 END), 0) as inflow,
			COALESCE(SUM(CASE WHEN "book_amount_id_r" < 0 THEN ABS("book_amount_id_r") ELSE 0 END), 0) as outflow
		FROM cash_bank_transactions
		WHERE transaction_date >= DATE_TRUNC('month', CURRENT_DATE) - INTERVAL '6 months'
			AND deleted_at IS NULL
			AND reference_type NOT IN ('TRANSFER', 'OPENING_BALANCE')
		GROUP BY DATE_TRUNC('month', transaction_date), TO_CHAR(DATE_TRUNC('month', transaction_date), 'YYYY-MM'), TO_CHAR(DATE_TRUNC('month', transaction_date), 'Mon')
		ORDER BY DATE_TRUNC('month', transaction_date)
	`).Scan(&results).Error
	if err != nil {
		return nil, err
	}

	return fillMonthlyCashFlowWindow(results), nil
}

func fillMonthlyValueWindow(results []monthlyValueQueryResult) []MonthlyData {
	periodMap := make(map[string]monthlyValueQueryResult, len(results))
	for _, result := range results {
		periodMap[result.Period] = result
	}

	window := buildRecentMonthWindow(7)
	data := make([]MonthlyData, 0, len(window))
	for _, monthStart := range window {
		period := monthStart.Format("2006-01")
		label := monthStart.Format("Jan")
		if result, exists := periodMap[period]; exists {
			label = result.Month
			data = append(data, MonthlyData{Month: label, Value: result.Value})
			continue
		}
		data = append(data, MonthlyData{Month: label, Value: 0})
	}

	return data
}

func fillMonthlyCashFlowWindow(results []monthlyCashFlowQueryResult) []DashboardCashFlowData {
	periodMap := make(map[string]monthlyCashFlowQueryResult, len(results))
	for _, result := range results {
		periodMap[result.Period] = result
	}

	window := buildRecentMonthWindow(7)
	data := make([]DashboardCashFlowData, 0, len(window))
	for _, monthStart := range window {
		period := monthStart.Format("2006-01")
		label := monthStart.Format("Jan")
		if result, exists := periodMap[period]; exists {
			label = result.Month
			data = append(data, DashboardCashFlowData{
				Month:   label,
				Inflow:  result.Inflow,
				Outflow: result.Outflow,
				Balance: result.Inflow - result.Outflow,
			})
			continue
		}
		data = append(data, DashboardCashFlowData{Month: label})
	}

	return data
}

func buildRecentMonthWindow(count int) []time.Time {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -(count - 1), 0)
	window := make([]time.Time, 0, count)
	for i := 0; i < count; i++ {
		window = append(window, start.AddDate(0, i, 0))
	}
	return window
}

// getTopAccounts gets the top 5 accounts by balance
func (ds *DashboardService) getTopAccounts() ([]AccountData, error) {
	type QueryResult struct {
		Name    string  `json:"name"`
		Balance float64 `json:"balance"`
		Type    string  `json:"type"`
	}

	var results []QueryResult
	err := ds.DB.Raw(`
		SELECT 
			name,
			ABS(balance) as balance,
			type
		FROM accounts 
		WHERE deleted_at IS NULL 
			AND is_active = true
			AND balance != 0
			AND is_header = false
		ORDER BY ABS(balance) DESC
		LIMIT 5
	`).Scan(&results).Error

	if err != nil {
		return nil, err
	}

	var data []AccountData
	for _, result := range results {
		data = append(data, AccountData{
			Name:    result.Name,
			Balance: result.Balance,
			Type:    result.Type,
		})
	}

	return data, nil
}

// getRecentTransactions gets recent transactions
func (ds *DashboardService) getRecentTransactions() ([]TransactionData, error) {
	var data []TransactionData

	// Get recent sales data
	var salesResults []TransactionData
	err := ds.DB.Raw(`
		SELECT 
			s.id,
			s.code as transaction_id,
			COALESCE(s.notes, 'Sales Transaction') as description,
			s.net_before_tax as amount,
			TO_CHAR(s.date, 'YYYY-MM-DD') as date,
			'SALE' as type,
			'Sales Transaction' as account_name,
			c.name as contact_name,
			s.status
		FROM sales s 
		LEFT JOIN contacts c ON s.customer_id = c.id
		WHERE s.deleted_at IS NULL
		ORDER BY s.created_at DESC
		LIMIT 5
	`).Scan(&salesResults).Error

	if err != nil {
		return nil, err
	}

	// Get recent purchases data
	var purchaseResults []TransactionData
	err = ds.DB.Raw(`
		SELECT 
			p.id,
			p.code as transaction_id,
			COALESCE(p.notes, 'Purchase Transaction') as description,
			p.net_before_tax as amount,
			TO_CHAR(p.date, 'YYYY-MM-DD') as date,
			'PURCHASE' as type,
			'Purchase Transaction' as account_name,
			c.name as contact_name,
			p.status
		FROM purchases p 
		LEFT JOIN contacts c ON p.vendor_id = c.id
		WHERE p.deleted_at IS NULL
		ORDER BY p.created_at DESC
		LIMIT 5
	`).Scan(&purchaseResults).Error

	if err != nil {
		return nil, err
	}

	// Combine both sales and purchases
	data = append(data, salesResults...)
	data = append(data, purchaseResults...)

	return data, nil
}

// GetQuickStats returns quick statistics for dashboard widgets
func (ds *DashboardService) GetQuickStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total products
	var totalProducts int64
	err := ds.DB.Model(&models.Product{}).Where("is_active = ?", true).Count(&totalProducts).Error
	if err != nil {
		return nil, err
	}
	stats["total_products"] = totalProducts

	// Low stock products
	var lowStockCount int64
	err = ds.DB.Model(&models.Product{}).
		Where("stock <= min_stock AND min_stock > 0 AND is_active = ?", true).
		Count(&lowStockCount).Error
	if err != nil {
		return nil, err
	}
	stats["low_stock_count"] = lowStockCount

	// Out of stock products
	var outOfStockCount int64
	err = ds.DB.Model(&models.Product{}).
		Where("stock = 0 AND is_active = ?", true).
		Count(&outOfStockCount).Error
	if err != nil {
		return nil, err
	}
	stats["out_of_stock_count"] = outOfStockCount

	// Total categories
	var totalCategories int64
	err = ds.DB.Model(&models.ProductCategory{}).Where("is_active = ?", true).Count(&totalCategories).Error
	if err != nil {
		return nil, err
	}
	stats["total_categories"] = totalCategories

	// Today's sales
	var todaySales float64
	err = ds.DB.Model(&models.Sale{}).
		Where("DATE(created_at) = CURRENT_DATE").
		Select("COALESCE(SUM(net_before_tax), 0)").
		Scan(&todaySales).Error
	if err != nil {
		return nil, err
	}
	stats["today_sales"] = todaySales

	// Today's purchases (only approved)
	var todayPurchases float64
	err = ds.DB.Model(&models.Purchase{}).
		Where("DATE(created_at) = CURRENT_DATE AND status IN (?)", []string{"APPROVED", "COMPLETED"}).
		Select("COALESCE(SUM(net_before_tax), 0)").
		Scan(&todayPurchases).Error
	if err != nil {
		return nil, err
	}
	stats["today_purchases"] = todayPurchases

	return stats, nil
}
