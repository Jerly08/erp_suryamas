package services

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"app-sistem-akuntansi/models"
)

// ===== COMPANY PROFILE HELPERS =====

// getDefaultCompanyName gets company name from environment or returns default
func (ers *EnhancedReportService) getDefaultCompanyName() string {
	if name := os.Getenv("COMPANY_NAME"); name != "" {
		return name
	}
	return "PT. Sistema Akuntansi Digital"
}

// getDefaultCompanyAddress gets company address from environment or returns default
func (ers *EnhancedReportService) getDefaultCompanyAddress() string {
	if address := os.Getenv("COMPANY_ADDRESS"); address != "" {
		return address
	}
	return "Jl. Teknologi Digital No. 123"
}

// getDefaultCompanyCity gets company city from environment or returns default
func (ers *EnhancedReportService) getDefaultCompanyCity() string {
	if city := os.Getenv("COMPANY_CITY"); city != "" {
		return city
	}
	return "Jakarta"
}

// getDefaultState gets company state from environment or returns default
func (ers *EnhancedReportService) getDefaultState() string {
	if state := os.Getenv("COMPANY_STATE"); state != "" {
		return state
	}
	return "DKI Jakarta"
}

// getDefaultCountry gets company country from environment or returns default
func (ers *EnhancedReportService) getDefaultCountry() string {
	if country := os.Getenv("COMPANY_COUNTRY"); country != "" {
		return country
	}
	return "Indonesia"
}

// getDefaultPostalCode gets company postal code from environment or returns default
func (ers *EnhancedReportService) getDefaultPostalCode() string {
	if postal := os.Getenv("COMPANY_POSTAL_CODE"); postal != "" {
		return postal
	}
	return "12345"
}

// getDefaultCompanyPhone gets company phone from environment or returns default
func (ers *EnhancedReportService) getDefaultCompanyPhone() string {
	if phone := os.Getenv("COMPANY_PHONE"); phone != "" {
		return phone
	}
	return "+62-21-1234567"
}

// getDefaultCompanyEmail gets company email from environment or returns default
func (ers *EnhancedReportService) getDefaultCompanyEmail() string {
	if email := os.Getenv("COMPANY_EMAIL"); email != "" {
		return email
	}
	return "info@sistemaakuntansi.com"
}

// getDefaultCompanyWebsite gets company website from environment or returns default
func (ers *EnhancedReportService) getDefaultCompanyWebsite() string {
	if website := os.Getenv("COMPANY_WEBSITE"); website != "" {
		return website
	}
	return "www.sistemaakuntansi.com"
}

// getDefaultCurrency gets default currency from environment or returns IDR
func (ers *EnhancedReportService) getDefaultCurrency() string {
	if currency := os.Getenv("DEFAULT_CURRENCY"); currency != "" {
		return currency
	}
	return "IDR"
}

// getCurrencyFromSettings returns configured currency (Settings),
// falling back to CompanyProfile then environment default
func (ers *EnhancedReportService) getCurrencyFromSettings() string {
	var settings models.Settings
	if err := ers.db.First(&settings).Error; err == nil && settings.Currency != "" {
		return settings.Currency
	}
	if ers.companyProfile != nil && ers.companyProfile.Currency != "" {
		return ers.companyProfile.Currency
	}
	return ers.getDefaultCurrency()
}

// getDefaultTaxNumber gets company tax number from environment or returns default
func (ers *EnhancedReportService) getDefaultTaxNumber() string {
	if taxNum := os.Getenv("COMPANY_TAX_NUMBER"); taxNum != "" {
		return taxNum
	}
	return "12.345.678.9-012.000"
}

// ===== ACCOUNT CALCULATION HELPERS =====

// calculateAccountBalance calculates account balance up to a specific date
// Uses SSOT (unified_journal_ledger + unified_journal_lines) as the source of truth
// Includes ALL journal entries including CLOSING entries (for balance sheet)
func (ers *EnhancedReportService) calculateAccountBalance(accountID uint, asOfDate time.Time) float64 {
	var totalDebit, totalCredit float64

	// Query from SSOT unified journal tables
	ers.db.Table("unified_journal_lines").
		Joins("JOIN unified_journal_ledger ON unified_journal_lines.journal_id = unified_journal_ledger.id").
		Where("unified_journal_lines.account_id = ? AND unified_journal_ledger.entry_date <= ? AND unified_journal_ledger.status = ? AND unified_journal_ledger.deleted_at IS NULL",
			accountID, asOfDate, models.SSOTStatusPosted).
		Select("COALESCE(SUM(unified_journal_lines.debit_amount), 0) as total_debit, COALESCE(SUM(unified_journal_lines.credit_amount), 0) as total_credit").
		Row().Scan(&totalDebit, &totalCredit)

	// Get account to determine normal balance type
	ctx := context.Background()
	account, err := ers.accountRepo.FindByID(ctx, accountID)
	if err != nil {
		return 0
	}

	// Calculate balance based on normal balance type
	if account.GetNormalBalance() == models.NormalBalanceDebit {
		return totalDebit - totalCredit
	} else {
		return totalCredit - totalDebit
	}
}

// calculateAccountBalanceForPeriod calculates account balance for a specific period
// Uses SSOT and EXCLUDES CLOSING entries so P&L shows actual activity
func (ers *EnhancedReportService) calculateAccountBalanceForPeriod(accountID uint, startDate, endDate time.Time) float64 {
	var totalDebit, totalCredit float64

	// Query from SSOT unified journal tables, EXCLUDING CLOSING entries
	// CLOSING entries zero out revenue/expense; P&L must show pre-closing activity
	ers.db.Table("unified_journal_lines").
		Joins("JOIN unified_journal_ledger ON unified_journal_lines.journal_id = unified_journal_ledger.id").
		Where("unified_journal_lines.account_id = ? AND unified_journal_ledger.entry_date BETWEEN ? AND ? AND unified_journal_ledger.status = ? AND unified_journal_ledger.deleted_at IS NULL AND unified_journal_ledger.source_type != ?",
			accountID, startDate, endDate, models.SSOTStatusPosted, models.SSOTSourceTypeClosing).
		Select("COALESCE(SUM(unified_journal_lines.debit_amount), 0) as total_debit, COALESCE(SUM(unified_journal_lines.credit_amount), 0) as total_credit").
		Row().Scan(&totalDebit, &totalCredit)

	// Get account to determine normal balance direction
	ctx := context.Background()
	account, err := ers.accountRepo.FindByID(ctx, accountID)
	if err != nil {
		return 0
	}

	// Return net balance based on normal balance type
	if account.GetNormalBalance() == models.NormalBalanceDebit {
		return totalDebit - totalCredit // Expenses, Assets
	}
	return totalCredit - totalDebit // Revenue, Liabilities, Equity
}

// ===== BALANCE SHEET HELPERS =====

// buildAssetsSection builds the assets section of balance sheet
func (ers *EnhancedReportService) buildAssetsSection(assets []BalanceSheetItem) BalanceSheetSection {
	// Group assets by category
	categoryGroups := make(map[string][]BalanceSheetItem)

	for _, asset := range assets {
		category := ers.getAssetCategory(asset.Category)
		categoryGroups[category] = append(categoryGroups[category], asset)
	}

	var subtotals []BalanceSheetSubtotal
	var total float64

	// Calculate subtotals for each category
	for category, items := range categoryGroups {
		subtotal := 0.0
		for _, item := range items {
			subtotal += item.Balance
		}
		subtotals = append(subtotals, BalanceSheetSubtotal{
			Name:     category,
			Amount:   subtotal,
			Category: category,
		})
		total += subtotal
	}

	return BalanceSheetSection{
		Name:      "Assets",
		Items:     assets,
		Subtotals: subtotals,
		Total:     total,
	}
}

// buildLiabilitiesSection builds the liabilities section of balance sheet
func (ers *EnhancedReportService) buildLiabilitiesSection(liabilities []BalanceSheetItem) BalanceSheetSection {
	// Group liabilities by category
	categoryGroups := make(map[string][]BalanceSheetItem)

	for _, liability := range liabilities {
		category := ers.getLiabilityCategory(liability.Category)
		categoryGroups[category] = append(categoryGroups[category], liability)
	}

	var subtotals []BalanceSheetSubtotal
	var total float64

	// Calculate subtotals for each category
	for category, items := range categoryGroups {
		subtotal := 0.0
		for _, item := range items {
			subtotal += item.Balance
		}
		subtotals = append(subtotals, BalanceSheetSubtotal{
			Name:     category,
			Amount:   subtotal,
			Category: category,
		})
		total += subtotal
	}

	return BalanceSheetSection{
		Name:      "Liabilities",
		Items:     liabilities,
		Subtotals: subtotals,
		Total:     total,
	}
}

// buildEquitySection builds the equity section of balance sheet
func (ers *EnhancedReportService) buildEquitySection(equity []BalanceSheetItem) BalanceSheetSection {
	var total float64
	for _, item := range equity {
		total += item.Balance
	}

	return BalanceSheetSection{
		Name:  "Equity",
		Items: equity,
		Total: total,
	}
}

// ===== P&L HELPERS =====

// isOperatingRevenue checks if an account category is operating revenue
func (ers *EnhancedReportService) isOperatingRevenue(category string) bool {
	operatingCategories := []string{
		models.CategoryOperatingRevenue,
		models.CategoryServiceRevenue,
		models.CategorySalesRevenue,
	}

	for _, opCat := range operatingCategories {
		if category == opCat {
			return true
		}
	}
	return false
}

// isCOGS checks if an account category is Cost of Goods Sold
func (ers *EnhancedReportService) isCOGS(category string) bool {
	cogsCategories := []string{
		models.CategoryCostOfGoodsSold,
		models.CategoryDirectMaterial,
		models.CategoryDirectLabor,
		models.CategoryManufacturingOverhead,
		models.CategoryFreightIn,
	}

	for _, cogsCat := range cogsCategories {
		if category == cogsCat {
			return true
		}
	}
	return false
}

// isOperatingExpense checks if an account category is operating expense
func (ers *EnhancedReportService) isOperatingExpense(category string) bool {
	operatingExpenseCategories := []string{
		models.CategoryOperatingExpense,
		models.CategoryAdministrativeExp,
		models.CategorySellingExpense,
		models.CategoryMarketingExpense,
		models.CategoryGeneralExpense,
		models.CategoryDepreciationExp,
		models.CategoryAmortizationExp,
		models.CategoryBadDebtExpense,
	}

	for _, opExpCat := range operatingExpenseCategories {
		if category == opExpCat {
			return true
		}
	}
	return false
}

// isTaxExpense checks if an account category is tax expense
func (ers *EnhancedReportService) isTaxExpense(category string) bool {
	return category == models.CategoryTaxExpense
}

// calculateItemPercentages calculates percentages for P&L items
func (ers *EnhancedReportService) calculateItemPercentages(items []PLItem, totalRevenue float64) {
	for i := range items {
		if totalRevenue != 0 {
			items[i].Percentage = (items[i].Amount / totalRevenue) * 100
		}
	}
}

// getSharesOutstanding gets number of shares outstanding from company profile
func (ers *EnhancedReportService) getSharesOutstanding() float64 {
	if ers.companyProfile != nil && ers.companyProfile.SharesOutstanding > 0 {
		return ers.companyProfile.SharesOutstanding
	}
	return 1000000 // Default 1 million shares if not configured
}

// ===== CASH FLOW HELPERS =====

// getCashAccounts gets all cash and cash equivalent accounts
func (ers *EnhancedReportService) getCashAccounts() []models.Account {
	var accounts []models.Account
	ers.db.Where("type = ? AND (category = ? OR category = ?) AND is_active = ?",
		models.AccountTypeAsset,
		models.CategoryCurrentAsset,
		"CASH_AND_EQUIVALENTS",
		true).
		Where("code LIKE ? OR code LIKE ? OR name LIKE ? OR name LIKE ?",
			"11%", "1101%", "%kas%", "%bank%").
		Find(&accounts)
	return accounts
}

// calculateTotalCashBalance calculates total cash balance from all cash accounts
func (ers *EnhancedReportService) calculateTotalCashBalance(cashAccounts []models.Account, date time.Time) float64 {
	total := 0.0
	for _, account := range cashAccounts {
		balance := ers.calculateAccountBalance(account.ID, date)
		total += balance
	}
	return total
}

// calculateOperatingCashFlow calculates cash flow from operating activities
func (ers *EnhancedReportService) calculateOperatingCashFlow(startDate, endDate time.Time) []CashFlowItem {
	// This is a simplified implementation - in practice, you'd calculate based on:
	// Net income + adjustments for non-cash items + changes in working capital

	var items []CashFlowItem

	// Get net income (simplified)
	netIncome := ers.calculateNetIncome(startDate, endDate)
	if netIncome != 0 {
		items = append(items, CashFlowItem{
			Description: "Net Income",
			Amount:      netIncome,
			Category:    "OPERATING",
		})
	}

	// Add depreciation (non-cash expense)
	depreciation := ers.calculateDepreciationExpense(startDate, endDate)
	if depreciation != 0 {
		items = append(items, CashFlowItem{
			Description: "Depreciation",
			Amount:      depreciation,
			Category:    "OPERATING",
		})
	}

	return items
}

// calculateInvestingCashFlow calculates cash flow from investing activities
func (ers *EnhancedReportService) calculateInvestingCashFlow(startDate, endDate time.Time) []CashFlowItem {
	var items []CashFlowItem

	// Get asset purchases/sales
	assetTransactions := ers.calculateAssetTransactions(startDate, endDate)
	items = append(items, assetTransactions...)

	return items
}

// calculateFinancingCashFlow calculates cash flow from financing activities
func (ers *EnhancedReportService) calculateFinancingCashFlow(startDate, endDate time.Time) []CashFlowItem {
	var items []CashFlowItem

	// Get loan transactions, capital contributions, dividends, etc.
	financingTransactions := ers.calculateFinancingTransactions(startDate, endDate)
	items = append(items, financingTransactions...)

	return items
}

// sumCashFlowItems sums up cash flow items
func (ers *EnhancedReportService) sumCashFlowItems(items []CashFlowItem) float64 {
	total := 0.0
	for _, item := range items {
		total += item.Amount
	}
	return total
}

// ===== SALES SUMMARY HELPERS =====

// formatPeriod formats date according to grouping
func (ers *EnhancedReportService) formatPeriod(date time.Time, groupBy string) string {
	switch strings.ToLower(groupBy) {
	case "day":
		return date.Format("2006-01-02")
	case "week":
		year, week := date.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	case "month":
		return date.Format("2006-01")
	case "quarter":
		quarter := (int(date.Month())-1)/3 + 1
		return fmt.Sprintf("%d-Q%d", date.Year(), quarter)
	case "year":
		return fmt.Sprintf("%d", date.Year())
	default:
		return date.Format("2006-01")
	}
}

// getPeriodStart gets the start date of a period
func (ers *EnhancedReportService) getPeriodStart(date time.Time, groupBy string) time.Time {
	switch strings.ToLower(groupBy) {
	case "day":
		return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	case "week":
		// Start of week (Monday)
		days := int(date.Weekday() - time.Monday)
		if days < 0 {
			days += 7
		}
		return date.AddDate(0, 0, -days)
	case "month":
		return time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
	case "quarter":
		month := ((int(date.Month())-1)/3)*3 + 1
		return time.Date(date.Year(), time.Month(month), 1, 0, 0, 0, 0, date.Location())
	case "year":
		return time.Date(date.Year(), 1, 1, 0, 0, 0, 0, date.Location())
	default:
		return time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
	}
}

// getPeriodEnd gets the end date of a period
func (ers *EnhancedReportService) getPeriodEnd(date time.Time, groupBy string) time.Time {
	switch strings.ToLower(groupBy) {
	case "day":
		return time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 999999999, date.Location())
	case "week":
		// End of week (Sunday)
		days := int(time.Sunday - date.Weekday())
		if days <= 0 {
			days += 7
		}
		return date.AddDate(0, 0, days-1)
	case "month":
		return time.Date(date.Year(), date.Month()+1, 1, 0, 0, 0, 0, date.Location()).AddDate(0, 0, -1)
	case "quarter":
		month := ((int(date.Month())-1)/3+1)*3 + 1
		return time.Date(date.Year(), time.Month(month), 1, 0, 0, 0, 0, date.Location()).AddDate(0, 0, -1)
	case "year":
		return time.Date(date.Year(), 12, 31, 23, 59, 59, 999999999, date.Location())
	default:
		return time.Date(date.Year(), date.Month()+1, 1, 0, 0, 0, 0, date.Location()).AddDate(0, 0, -1)
	}
}

// ===== SORTING HELPERS =====

// sortCustomersByRevenue sorts customers by total revenue
func (ers *EnhancedReportService) sortCustomersByRevenue(customerMap map[uint]*CustomerSalesData) []CustomerSalesData {
	var customers []CustomerSalesData
	for _, customer := range customerMap {
		customers = append(customers, *customer)
	}

	sort.Slice(customers, func(i, j int) bool {
		return customers[i].TotalAmount > customers[j].TotalAmount
	})

	return customers
}

// getSaleItemsForCustomer fetches detailed sale items for a specific customer
func (ers *EnhancedReportService) getSaleItemsForCustomer(customerID uint, startDate, endDate time.Time) ([]SaleItemDetail, error) {
	query := `
		SELECT 
			COALESCE(si.product_id, 0) as product_id,
			COALESCE(p.code, 'N/A') as product_code,
			COALESCE(p.name, 'Unknown Product') as product_name,
			si.quantity,
			si.unit_price as unit_price,
			si.line_total as total_price,
			COALESCE(p.unit, 'pcs') as unit,
			s.date as sale_date,
			COALESCE(s.invoice_number, s.code, '') as invoice_number
		FROM sales s
		INNER JOIN sale_items si ON si.sale_id = s.id
		LEFT JOIN products p ON p.id = si.product_id
		WHERE s.customer_id = ?
		  AND s.date BETWEEN ? AND ?
		  AND s.deleted_at IS NULL
		  AND si.deleted_at IS NULL
		ORDER BY s.date DESC, si.id
	`

	var items []SaleItemDetail
	err := ers.db.Raw(query, customerID, startDate, endDate).Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query sale items: %w", err)
	}

	return items, nil
}

// sortProductsBySales sorts products by sales amount
func (ers *EnhancedReportService) sortProductsBySales(productMap map[uint]*ProductSalesData) []ProductSalesData {
	var products []ProductSalesData
	for _, product := range productMap {
		products = append(products, *product)
	}

	sort.Slice(products, func(i, j int) bool {
		return products[i].TotalAmount > products[j].TotalAmount
	})

	return products
}

// sortPeriodsByDate sorts periods by date
func (ers *EnhancedReportService) sortPeriodsByDate(periodMap map[string]*PeriodData) []PeriodData {
	var periods []PeriodData
	for _, period := range periodMap {
		periods = append(periods, *period)
	}

	sort.Slice(periods, func(i, j int) bool {
		return periods[i].StartDate.Before(periods[j].StartDate)
	})

	return periods
}

// convertStatusMapToSlice converts status map to sorted slice
func (ers *EnhancedReportService) convertStatusMapToSlice(statusMap map[string]*StatusData) []StatusData {
	var statuses []StatusData
	for _, status := range statusMap {
		statuses = append(statuses, *status)
	}

	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].Amount > statuses[j].Amount
	})

	return statuses
}

// buildTopPerformers builds top performers data
func (ers *EnhancedReportService) buildTopPerformers(customers []CustomerSalesData, products []ProductSalesData) TopPerformersData {
	// Take top 10 customers
	topCustomers := customers
	if len(topCustomers) > 10 {
		topCustomers = topCustomers[:10]
	}

	// Take top 10 products
	topProducts := products
	if len(topProducts) > 10 {
		topProducts = topProducts[:10]
	}

	// TODO: Add salespeople data when available

	return TopPerformersData{
		TopCustomers:   topCustomers,
		TopProducts:    topProducts,
		TopSalespeople: []SalespersonData{}, // Empty for now
	}
}

// calculateGrowthAnalysis calculates growth analysis
func (ers *EnhancedReportService) calculateGrowthAnalysis(startDate, endDate time.Time, currentRevenue float64) GrowthAnalysisData {
	// Calculate previous period
	duration := endDate.Sub(startDate)
	prevStart := startDate.Add(-duration)
	prevEnd := startDate.AddDate(0, 0, -1)

	// Get previous period revenue (simplified)
	previousRevenue := ers.calculateRevenueForPeriod(prevStart, prevEnd)

	// Calculate growth rates
	monthOverMonth := 0.0
	if previousRevenue != 0 {
		monthOverMonth = ((currentRevenue - previousRevenue) / previousRevenue) * 100
	}

	trend := "STABLE"
	if monthOverMonth > 5 {
		trend = "INCREASING"
	} else if monthOverMonth < -5 {
		trend = "DECREASING"
	}

	return GrowthAnalysisData{
		MonthOverMonth:     monthOverMonth,
		QuarterOverQuarter: monthOverMonth, // Simplified
		YearOverYear:       monthOverMonth, // Simplified
		TrendDirection:     trend,
		SeasonalityIndex:   1.0, // Default
	}
}

// ===== VENDOR ANALYSIS HELPERS =====

// calculatePaymentScore calculates payment performance score for vendor
func (ers *EnhancedReportService) calculatePaymentScore(vendor *VendorPerformanceData) float64 {
	if vendor.TotalPurchases == 0 {
		return 0
	}

	// Simple scoring based on payment ratio and days
	paymentRatio := vendor.TotalPayments / vendor.TotalPurchases

	// Base score from payment ratio (0-70 points)
	score := paymentRatio * 70

	// Bonus points for quick payment (up to 30 points)
	if vendor.AveragePaymentDays <= 30 {
		score += 30
	} else if vendor.AveragePaymentDays <= 45 {
		score += 20
	} else if vendor.AveragePaymentDays <= 60 {
		score += 10
	}

	return math.Min(score, 100)
}

// getVendorRating converts payment score to rating
func (ers *EnhancedReportService) getVendorRating(score float64) string {
	if score >= 90 {
		return "EXCELLENT"
	} else if score >= 80 {
		return "GOOD"
	} else if score >= 70 {
		return "FAIR"
	} else if score >= 60 {
		return "POOR"
	} else {
		return "BAD"
	}
}

// ===== ADDITIONAL HELPER METHODS =====

// These methods need to be implemented based on your specific business logic

// getAssetCategory categorizes asset accounts
func (ers *EnhancedReportService) getAssetCategory(category string) string {
	switch category {
	case models.CategoryCurrentAsset:
		return "Current Assets"
	case models.CategoryFixedAsset:
		return "Fixed Assets"
	case models.CategoryIntangibleAsset:
		return "Intangible Assets"
	case models.CategoryInvestmentAsset:
		return "Investment Assets"
	default:
		return "Other Assets"
	}
}

// getLiabilityCategory categorizes liability accounts
func (ers *EnhancedReportService) getLiabilityCategory(category string) string {
	switch category {
	case models.CategoryCurrentLiability:
		return "Current Liabilities"
	case models.CategoryLongTermLiability:
		return "Long-term Liabilities"
	default:
		return "Other Liabilities"
	}
}

// calculateNetIncome calculates net income for a period (simplified)
// Uses SSOT and EXCLUDES CLOSING entries to show actual period performance
func (ers *EnhancedReportService) calculateNetIncome(startDate, endDate time.Time) float64 {
	var totalRevenue, totalExpense float64

	// Get revenue from SSOT, excluding CLOSING entries
	ers.db.Table("unified_journal_lines").
		Joins("JOIN unified_journal_ledger ON unified_journal_lines.journal_id = unified_journal_ledger.id").
		Joins("JOIN accounts ON unified_journal_lines.account_id = accounts.id").
		Where("accounts.type = ? AND unified_journal_ledger.entry_date BETWEEN ? AND ? AND unified_journal_ledger.status = ? AND unified_journal_ledger.deleted_at IS NULL AND unified_journal_ledger.source_type != ?",
			models.AccountTypeRevenue, startDate, endDate, models.SSOTStatusPosted, models.SSOTSourceTypeClosing).
		Select("COALESCE(SUM(unified_journal_lines.credit_amount - unified_journal_lines.debit_amount), 0)").
		Row().Scan(&totalRevenue)

	// Get expense from SSOT, excluding CLOSING entries
	ers.db.Table("unified_journal_lines").
		Joins("JOIN unified_journal_ledger ON unified_journal_lines.journal_id = unified_journal_ledger.id").
		Joins("JOIN accounts ON unified_journal_lines.account_id = accounts.id").
		Where("accounts.type = ? AND unified_journal_ledger.entry_date BETWEEN ? AND ? AND unified_journal_ledger.status = ? AND unified_journal_ledger.deleted_at IS NULL AND unified_journal_ledger.source_type != ?",
			models.AccountTypeExpense, startDate, endDate, models.SSOTStatusPosted, models.SSOTSourceTypeClosing).
		Select("COALESCE(SUM(unified_journal_lines.debit_amount - unified_journal_lines.credit_amount), 0)").
		Row().Scan(&totalExpense)

	return totalRevenue - totalExpense
}

// calculateDepreciationExpense calculates depreciation expense for period
// Uses SSOT tables
func (ers *EnhancedReportService) calculateDepreciationExpense(startDate, endDate time.Time) float64 {
	var depreciation float64

	ers.db.Table("unified_journal_lines").
		Joins("JOIN unified_journal_ledger ON unified_journal_lines.journal_id = unified_journal_ledger.id").
		Joins("JOIN accounts ON unified_journal_lines.account_id = accounts.id").
		Where("accounts.category = ? AND unified_journal_ledger.entry_date BETWEEN ? AND ? AND unified_journal_ledger.status = ? AND unified_journal_ledger.deleted_at IS NULL",
			models.CategoryDepreciationExp, startDate, endDate, models.SSOTStatusPosted).
		Select("COALESCE(SUM(unified_journal_lines.debit_amount), 0)").
		Row().Scan(&depreciation)

	return depreciation
}

// calculateAssetTransactions calculates asset purchase/sale transactions
func (ers *EnhancedReportService) calculateAssetTransactions(startDate, endDate time.Time) []CashFlowItem {
	var items []CashFlowItem

	// This is a placeholder - implement based on your asset transaction logic
	// You might want to track asset purchases and sales separately

	return items
}

// calculateFinancingTransactions calculates financing activity transactions
func (ers *EnhancedReportService) calculateFinancingTransactions(startDate, endDate time.Time) []CashFlowItem {
	var items []CashFlowItem

	// This is a placeholder - implement based on your financing transaction logic
	// Track loan proceeds, loan repayments, capital contributions, dividends, etc.

	return items
}

// calculateRevenueForPeriod calculates total revenue for a period
// Uses SSOT and EXCLUDES CLOSING entries
func (ers *EnhancedReportService) calculateRevenueForPeriod(startDate, endDate time.Time) float64 {
	var revenue float64

	ers.db.Table("unified_journal_lines").
		Joins("JOIN unified_journal_ledger ON unified_journal_lines.journal_id = unified_journal_ledger.id").
		Joins("JOIN accounts ON unified_journal_lines.account_id = accounts.id").
		Where("accounts.type = ? AND unified_journal_ledger.entry_date BETWEEN ? AND ? AND unified_journal_ledger.status = ? AND unified_journal_ledger.deleted_at IS NULL AND unified_journal_ledger.source_type != ?",
			models.AccountTypeRevenue, startDate, endDate, models.SSOTStatusPosted, models.SSOTSourceTypeClosing).
		Select("COALESCE(SUM(unified_journal_lines.credit_amount - unified_journal_lines.debit_amount), 0)").
		Row().Scan(&revenue)

	return revenue
}

// Vendor analysis helper methods

// sortVendorsByPerformance sorts vendors by performance score
func (ers *EnhancedReportService) sortVendorsByPerformance(vendorMap map[uint]*VendorPerformanceData) []VendorPerformanceData {
	var vendors []VendorPerformanceData
	for _, vendor := range vendorMap {
		vendors = append(vendors, *vendor)
	}

	sort.Slice(vendors, func(i, j int) bool {
		return vendors[i].PaymentScore > vendors[j].PaymentScore
	})

	return vendors
}

// sortVendorsBySpend sorts vendors by spend amount
func (ers *EnhancedReportService) sortVendorsBySpend(vendorSpendMap map[uint]*VendorSpendData) []VendorSpendData {
	var vendors []VendorSpendData
	for _, vendor := range vendorSpendMap {
		vendors = append(vendors, *vendor)
	}

	sort.Slice(vendors, func(i, j int) bool {
		return vendors[i].TotalSpend > vendors[j].TotalSpend
	})

	return vendors
}

// calculatePaymentAnalysis calculates payment analysis metrics
func (ers *EnhancedReportService) calculatePaymentAnalysis(payments []models.Payment) PaymentAnalysisData {
	// This is a simplified implementation
	totalPayments := int64(len(payments))
	onTime := totalPayments * 80 / 100 // Assume 80% on time
	late := totalPayments * 15 / 100   // Assume 15% late
	overdue := totalPayments - onTime - late

	return PaymentAnalysisData{
		OnTimePayments:     onTime,
		LatePayments:       late,
		OverduePayments:    overdue,
		AveragePaymentDays: 35.0, // Default assumption
		PaymentEfficiency:  80.0, // Default assumption
	}
}

// buildVendorPaymentHistory builds vendor payment history
func (ers *EnhancedReportService) buildVendorPaymentHistory(startDate, endDate time.Time) []VendorPaymentHistory {
	var history []VendorPaymentHistory

	// Generate monthly history for the period
	current := startDate
	for current.Before(endDate) || current.Equal(endDate) {
		monthStart := time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, current.Location())
		monthEnd := monthStart.AddDate(0, 1, -1)

		if monthEnd.After(endDate) {
			monthEnd = endDate
		}

		// Calculate purchases and payments for this month
		purchases := ers.calculatePurchasesForPeriod(monthStart, monthEnd)
		payments := ers.calculatePaymentsForPeriod(monthStart, monthEnd)

		history = append(history, VendorPaymentHistory{
			Month:       monthStart.Format("2006-01"),
			Purchases:   purchases,
			Payments:    payments,
			Outstanding: purchases - payments,
		})

		current = current.AddDate(0, 1, 0)
	}

	return history
}

// countActiveVendors counts active vendors
func (ers *EnhancedReportService) countActiveVendors(vendorMap map[uint]*VendorPerformanceData) int64 {
	active := int64(0)
	for _, vendor := range vendorMap {
		if vendor.TotalPurchases > 0 {
			active++
		}
	}
	return active
}

// calculatePurchasesForPeriod calculates total purchases for period
func (ers *EnhancedReportService) calculatePurchasesForPeriod(startDate, endDate time.Time) float64 {
	var total float64
	ers.db.Model(&models.Purchase{}).
		Where("date BETWEEN ? AND ?", startDate, endDate).
		Select("COALESCE(SUM(total_amount), 0)").
		Row().Scan(&total)
	return total
}

// calculatePaymentsForPeriod calculates total payments for period using purchase payments
func (ers *EnhancedReportService) calculatePaymentsForPeriod(startDate, endDate time.Time) float64 {
	var total float64
	ers.db.Model(&models.PurchasePayment{}).
		Where("date BETWEEN ? AND ?", startDate, endDate).
		Select("COALESCE(SUM(amount), 0)").
		Row().Scan(&total)
	return total
}

// calculatePaymentAnalysisFromPurchasePayments calculates payment analysis from purchase payments
func (ers *EnhancedReportService) calculatePaymentAnalysisFromPurchasePayments(purchasePayments []models.PurchasePayment) PaymentAnalysisData {
	// This is a simplified implementation based on purchase payments
	totalPayments := int64(len(purchasePayments))
	onTime := totalPayments * 80 / 100 // Assume 80% on time
	late := totalPayments * 15 / 100   // Assume 15% late
	overdue := totalPayments - onTime - late

	return PaymentAnalysisData{
		OnTimePayments:     onTime,
		LatePayments:       late,
		OverduePayments:    overdue,
		AveragePaymentDays: 35.0, // Default assumption
		PaymentEfficiency:  80.0, // Default assumption
	}
}

// Journal entry analysis helper methods

// convertJournalTypeMapToSlice converts journal type map to slice
func (ers *EnhancedReportService) convertJournalTypeMapToSlice(typeMap map[string]*JournalTypeData) []JournalTypeData {
	var types []JournalTypeData
	for _, typeData := range typeMap {
		types = append(types, *typeData)
	}

	sort.Slice(types, func(i, j int) bool {
		return types[i].TotalAmount > types[j].TotalAmount
	})

	return types
}

// convertJournalStatusMapToSlice converts journal status map to slice
func (ers *EnhancedReportService) convertJournalStatusMapToSlice(statusMap map[string]*JournalStatusData) []JournalStatusData {
	var statuses []JournalStatusData
	for _, statusData := range statusMap {
		statuses = append(statuses, *statusData)
	}

	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].TotalAmount > statuses[j].TotalAmount
	})

	return statuses
}

// convertJournalUserMapToSlice converts journal user map to slice
func (ers *EnhancedReportService) convertJournalUserMapToSlice(userMap map[uint]*JournalUserData) []JournalUserData {
	var users []JournalUserData
	for _, userData := range userMap {
		users = append(users, *userData)
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].TotalAmount > users[j].TotalAmount
	})

	return users
}

// Purchase summary helper methods

// sortVendorPurchaseData sorts vendor purchase data by amount
func (ers *EnhancedReportService) sortVendorPurchaseData(vendorMap map[uint]*VendorPurchaseData) []VendorPurchaseData {
	var vendors []VendorPurchaseData
	for _, vendor := range vendorMap {
		vendors = append(vendors, *vendor)
	}

	sort.Slice(vendors, func(i, j int) bool {
		return vendors[i].TotalAmount > vendors[j].TotalAmount
	})

	return vendors
}

// getTopVendorPurchases gets top vendor purchases
func (ers *EnhancedReportService) getTopVendorPurchases(vendors []VendorPurchaseData) []VendorPurchaseData {
	if len(vendors) <= 10 {
		return vendors
	}
	return vendors[:10]
}

// getCategoryPurchaseData converts category map to slice
func (ers *EnhancedReportService) getCategoryPurchaseData(categoryMap map[uint]*CategoryPurchaseData) []CategoryPurchaseData {
	var categories []CategoryPurchaseData
	for _, category := range categoryMap {
		categories = append(categories, *category)
	}

	sort.Slice(categories, func(i, j int) bool {
		return categories[i].TotalAmount > categories[j].TotalAmount
	})

	return categories
}

// calculateSimpleCostAnalysis calculates basic cost analysis
func (ers *EnhancedReportService) calculateSimpleCostAnalysis(purchases []models.Purchase) CostAnalysisData {
	totalCost := 0.0
	totalQuantity := 0.0

	for _, purchase := range purchases {
		totalCost += purchase.TotalAmount
		// Sum quantity from purchase items
		for _, item := range purchase.PurchaseItems {
			totalQuantity += float64(item.Quantity)
		}
	}

	averageUnit := 0.0
	if totalQuantity > 0 {
		averageUnit = totalCost / totalQuantity
	}

	return CostAnalysisData{
		TotalCostOfGoods:   totalCost,
		AverageCostPerUnit: averageUnit,
		CostVariance:       0.0, // Simplified
		InflationImpact:    0.0, // Simplified
	}
}
