package services

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/utils"
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"gorm.io/gorm"
)

// EnhancedReportService provides comprehensive financial and operational reporting
// with proper accounting logic and business analytics
type EnhancedReportService struct {
	db                *gorm.DB
	accountRepo       repositories.AccountRepository
	salesRepo         *repositories.SalesRepository
	purchaseRepo      *repositories.PurchaseRepository
	productRepo       *repositories.ProductRepository
	contactRepo       repositories.ContactRepository
	paymentRepo       *repositories.PaymentRepository
	cashBankRepo      *repositories.CashBankRepository
	companyProfile    *models.CompanyProfile
	cacheService      *ReportCacheService
	validationService *ReportValidationService
}

// NewEnhancedReportService creates a new enhanced report service with caching and validation
func NewEnhancedReportService(
	db *gorm.DB,
	accountRepo repositories.AccountRepository,
	salesRepo *repositories.SalesRepository,
	purchaseRepo *repositories.PurchaseRepository,
	productRepo *repositories.ProductRepository,
	contactRepo repositories.ContactRepository,
	paymentRepo *repositories.PaymentRepository,
	cashBankRepo *repositories.CashBankRepository,
	cacheService *ReportCacheService,
) *EnhancedReportService {
	// Get company profile
	var companyProfile models.CompanyProfile
	db.First(&companyProfile)

	// Initialize validation service
	validationService := NewReportValidationService(db)

	return &EnhancedReportService{
		db:                db,
		accountRepo:       accountRepo,
		salesRepo:         salesRepo,
		purchaseRepo:      purchaseRepo,
		productRepo:       productRepo,
		contactRepo:       contactRepo,
		paymentRepo:       paymentRepo,
		cashBankRepo:      cashBankRepo,
		companyProfile:    &companyProfile,
		cacheService:      cacheService,
		validationService: validationService,
	}
}

// BalanceSheetData represents a comprehensive balance sheet structure
type BalanceSheetData struct {
	Company     CompanyInfo         `json:"company"`
	AsOfDate    time.Time           `json:"as_of_date"`
	Currency    string              `json:"currency"`
	Assets      BalanceSheetSection `json:"assets"`
	Liabilities BalanceSheetSection `json:"liabilities"`
	Equity      BalanceSheetSection `json:"equity"`
	TotalAssets float64             `json:"total_assets"`
	TotalEquity float64             `json:"total_equity"`
	IsBalanced  bool                `json:"is_balanced"`
	Difference  float64             `json:"difference"`
	GeneratedAt time.Time           `json:"generated_at"`
}

// ProfitLossData represents a comprehensive P&L statement structure
type ProfitLossData struct {
	Company            CompanyInfo `json:"company"`
	StartDate          time.Time   `json:"start_date"`
	EndDate            time.Time   `json:"end_date"`
	Currency           string      `json:"currency"`
	Revenue            PLSection   `json:"revenue"`
	CostOfGoodsSold    PLSection   `json:"cost_of_goods_sold"`
	GrossProfit        float64     `json:"gross_profit"`
	GrossProfitMargin  float64     `json:"gross_profit_margin"`
	OperatingExpenses  PLSection   `json:"operating_expenses"`
	OperatingIncome    float64     `json:"operating_income"`
	OtherIncome        PLSection   `json:"other_income"`
	OtherExpenses      PLSection   `json:"other_expenses"`
	EBITDA             float64     `json:"ebitda"`
	EBIT               float64     `json:"ebit"`
	NetIncomeBeforeTax float64     `json:"net_income_before_tax"`
	TaxExpense         float64     `json:"tax_expense"`
	NetIncome          float64     `json:"net_income"`
	NetIncomeMargin    float64     `json:"net_income_margin"`
	EarningsPerShare   float64     `json:"earnings_per_share"`
	DilutedEPS         float64     `json:"diluted_eps"`
	SharesOutstanding  float64     `json:"shares_outstanding"`
	GeneratedAt        time.Time   `json:"generated_at"`
	// Data Quality and Validation Fields
	ValidationReport    *ValidationReport `json:"validation_report,omitempty"`
	DataQualityScore    float64           `json:"data_quality_score,omitempty"`
	DataQualityWarnings []string          `json:"data_quality_warnings,omitempty"`
}

// ProfitLossComparative represents comparative P&L analysis
type ProfitLossComparative struct {
	CurrentPeriod ProfitLossData  `json:"current_period"`
	PriorPeriod   ProfitLossData  `json:"prior_period"`
	Variances     PLVarianceData  `json:"variances"`
	TrendAnalysis PLTrendAnalysis `json:"trend_analysis"`
	GeneratedAt   time.Time       `json:"generated_at"`
}

// PLVarianceData represents variance analysis between periods
type PLVarianceData struct {
	Revenue           PLVariance `json:"revenue"`
	CostOfGoodsSold   PLVariance `json:"cost_of_goods_sold"`
	GrossProfit       PLVariance `json:"gross_profit"`
	OperatingExpenses PLVariance `json:"operating_expenses"`
	OperatingIncome   PLVariance `json:"operating_income"`
	OtherIncome       PLVariance `json:"other_income"`
	OtherExpenses     PLVariance `json:"other_expenses"`
	NetIncome         PLVariance `json:"net_income"`
	EBITDA            PLVariance `json:"ebitda"`
	EBIT              PLVariance `json:"ebit"`
}

// PLVariance represents individual line item variance
type PLVariance struct {
	Current        float64 `json:"current"`
	Prior          float64 `json:"prior"`
	AbsoluteChange float64 `json:"absolute_change"`
	PercentChange  float64 `json:"percent_change"`
	Trend          string  `json:"trend"` // INCREASING, DECREASING, STABLE
}

// PLTrendAnalysis represents trend analysis over multiple periods
type PLTrendAnalysis struct {
	RevenueGrowthRate     float64 `json:"revenue_growth_rate"`
	ProfitabilityTrend    string  `json:"profitability_trend"`
	CostManagementIndex   float64 `json:"cost_management_index"`
	OperationalEfficiency float64 `json:"operational_efficiency"`
	MarginStability       string  `json:"margin_stability"`
}

// CashFlowData represents a comprehensive cash flow statement structure
type CashFlowData struct {
	Company             CompanyInfo     `json:"company"`
	StartDate           time.Time       `json:"start_date"`
	EndDate             time.Time       `json:"end_date"`
	Currency            string          `json:"currency"`
	OperatingActivities CashFlowSection `json:"operating_activities"`
	InvestingActivities CashFlowSection `json:"investing_activities"`
	FinancingActivities CashFlowSection `json:"financing_activities"`
	NetCashFlow         float64         `json:"net_cash_flow"`
	BeginningCash       float64         `json:"beginning_cash"`
	EndingCash          float64         `json:"ending_cash"`
	GeneratedAt         time.Time       `json:"generated_at"`
}

// SalesSummaryData represents comprehensive sales analytics
type SalesSummaryData struct {
	Company            CompanyInfo         `json:"company"`
	StartDate          time.Time           `json:"start_date"`
	EndDate            time.Time           `json:"end_date"`
	Currency           string              `json:"currency"`
	TotalRevenue       float64             `json:"total_revenue"`
	TotalSales         float64             `json:"total_sales"`
	TotalTransactions  int64               `json:"total_transactions"`
	TotalTax           float64             `json:"total_tax"`
	TotalDiscount      float64             `json:"total_discount"`
	TotalReturns       float64             `json:"total_returns"`
	NetRevenue         float64             `json:"net_revenue"`
	AverageOrderValue  float64             `json:"average_order_value"`
	TotalCustomers     int64               `json:"total_customers"`
	NewCustomers       int64               `json:"new_customers"`
	ReturningCustomers int64               `json:"returning_customers"`
	SalesByPeriod      []PeriodData        `json:"sales_by_period"`
	SalesByCustomer    []CustomerSalesData `json:"sales_by_customer"`
	SalesByProduct     []ProductSalesData  `json:"sales_by_product"`
	SalesByStatus      []StatusData        `json:"sales_by_status"`
	Items              []SalesSummaryItem  `json:"items"`
	TopPerformers      TopPerformersData   `json:"top_performers"`
	GrowthAnalysis     GrowthAnalysisData  `json:"growth_analysis"`
	GeneratedAt        time.Time           `json:"generated_at"`
	// Enhanced debugging and monitoring fields
	DebugInfo        map[string]interface{} `json:"debug_info,omitempty"`
	DataQualityScore float64                `json:"data_quality_score,omitempty"`
	ProcessingTime   string                 `json:"processing_time,omitempty"`
}

// SalesSummaryItem represents an individual sale transaction in the summary
type SalesSummaryItem struct {
	Date         string  `json:"date"`
	Code         string  `json:"code"`
	CustomerName string  `json:"customer_name"`
	TotalAmount  float64 `json:"total_amount"`
	Status       string  `json:"status"`
}

// PurchaseSummaryData represents comprehensive purchase analytics
type PurchaseSummaryData struct {
	Company              CompanyInfo            `json:"company"`
	StartDate            time.Time              `json:"start_date"`
	EndDate              time.Time              `json:"end_date"`
	Currency             string                 `json:"currency"`
	TotalPurchases       float64                `json:"total_purchases"`
	TotalTransactions    int64                  `json:"total_transactions"`
	AveragePurchaseValue float64                `json:"average_purchase_value"`
	TotalVendors         int64                  `json:"total_vendors"`
	NewVendors           int64                  `json:"new_vendors"`
	PurchasesByPeriod    []PeriodData           `json:"purchases_by_period"`
	PurchasesByVendor    []VendorPurchaseData   `json:"purchases_by_vendor"`
	PurchasesByCategory  []CategoryPurchaseData `json:"purchases_by_category"`
	PurchasesByStatus    []StatusData           `json:"purchases_by_status"`
	TopVendors           TopVendorsData         `json:"top_vendors"`
	CostAnalysis         CostAnalysisData       `json:"cost_analysis"`
	GeneratedAt          time.Time              `json:"generated_at"`
}

// Supporting data structures
type CompanyInfo struct {
	Name       string `json:"name"`
	Address    string `json:"address"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Phone      string `json:"phone"`
	Email      string `json:"email"`
	Website    string `json:"website"`
	TaxNumber  string `json:"tax_number"`
}

// VendorAnalysisData represents comprehensive vendor analysis
type VendorAnalysisData struct {
	Company              CompanyInfo             `json:"company"`
	StartDate            time.Time               `json:"start_date"`
	EndDate              time.Time               `json:"end_date"`
	Currency             string                  `json:"currency"`
	TotalVendors         int64                   `json:"total_vendors"`
	ActiveVendors        int64                   `json:"active_vendors"`
	TotalPurchases       float64                 `json:"total_purchases"`
	TotalPayments        float64                 `json:"total_payments"`
	OutstandingPayables  float64                 `json:"outstanding_payables"`
	VendorsByPerformance []VendorPerformanceData `json:"vendors_by_performance"`
	PaymentAnalysis      PaymentAnalysisData     `json:"payment_analysis"`
	TopVendorsBySpend    []VendorSpendData       `json:"top_vendors_by_spend"`
	VendorPaymentHistory []VendorPaymentHistory  `json:"vendor_payment_history"`
	GeneratedAt          time.Time               `json:"generated_at"`
}

// TrialBalanceData represents comprehensive trial balance
type TrialBalanceData struct {
	Company          CompanyInfo        `json:"company"`
	AsOfDate         time.Time          `json:"as_of_date"`
	Currency         string             `json:"currency"`
	Accounts         []TrialBalanceItem `json:"accounts"`
	TotalDebits      float64            `json:"total_debits"`
	TotalCredits     float64            `json:"total_credits"`
	IsBalanced       bool               `json:"is_balanced"`
	Difference       float64            `json:"difference"`
	AssetSummary     AccountTypeSummary `json:"asset_summary"`
	LiabilitySummary AccountTypeSummary `json:"liability_summary"`
	EquitySummary    AccountTypeSummary `json:"equity_summary"`
	RevenueSummary   AccountTypeSummary `json:"revenue_summary"`
	ExpenseSummary   AccountTypeSummary `json:"expense_summary"`
	GeneratedAt      time.Time          `json:"generated_at"`
}

// GeneralLedgerData represents detailed general ledger report
type GeneralLedgerData struct {
	Company        CompanyInfo    `json:"company"`
	Account        models.Account `json:"account"`
	StartDate      time.Time      `json:"start_date"`
	EndDate        time.Time      `json:"end_date"`
	Currency       string         `json:"currency"`
	OpeningBalance float64        `json:"opening_balance"`
	ClosingBalance float64        `json:"closing_balance"`
	TotalDebits    float64        `json:"total_debits"`
	TotalCredits   float64        `json:"total_credits"`
	// Enhanced UI fields
	NetPositionChange   float64                `json:"net_position_change"`
	NetPositionStatus   string                 `json:"net_position_status"`
	TotalTransactionVol float64                `json:"total_transaction_volume"`
	CashImpact          float64                `json:"cash_impact"`
	CashImpactStatus    string                 `json:"cash_impact_status"`
	IsBalanced          bool                   `json:"is_balanced"`
	Transactions        []GeneralLedgerEntry   `json:"transactions"`
	MonthlySummary      []MonthlyLedgerSummary `json:"monthly_summary"`
	GeneratedAt         time.Time              `json:"generated_at"`
}

// GeneralLedgerAllData represents general ledger report for all accounts
type GeneralLedgerAllData struct {
	Company           CompanyInfo         `json:"company"`
	StartDate         time.Time           `json:"start_date"`
	EndDate           time.Time           `json:"end_date"`
	Currency          string              `json:"currency"`
	AccountCount      int                 `json:"account_count"`
	TotalDebits       float64             `json:"total_debits"`
	TotalCredits      float64             `json:"total_credits"`
	TotalTransactions int                 `json:"total_transactions"`
	Accounts          []GeneralLedgerData `json:"accounts"`
	GeneratedAt       time.Time           `json:"generated_at"`
}

// JournalEntryAnalysisData represents comprehensive journal entry analysis
type JournalEntryAnalysisData struct {
	Company           CompanyInfo           `json:"company"`
	StartDate         time.Time             `json:"start_date"`
	EndDate           time.Time             `json:"end_date"`
	Currency          string                `json:"currency"`
	TotalEntries      int64                 `json:"total_entries"`
	TotalDebitAmount  float64               `json:"total_debit_amount"`
	TotalCreditAmount float64               `json:"total_credit_amount"`
	EntriesByType     []JournalTypeData     `json:"entries_by_type"`
	EntriesByStatus   []JournalStatusData   `json:"entries_by_status"`
	EntriesByUser     []JournalUserData     `json:"entries_by_user"`
	RecentEntries     []JournalEntryDetail  `json:"recent_entries"`
	LargestEntries    []JournalEntryDetail  `json:"largest_entries"`
	UnbalancedEntries []JournalEntryDetail  `json:"unbalanced_entries"`
	ComplianceCheck   JournalComplianceData `json:"compliance_check"`
	GeneratedAt       time.Time             `json:"generated_at"`
}

// Supporting structures for new report types
type VendorPerformanceData struct {
	VendorID           uint    `json:"vendor_id"`
	VendorName         string  `json:"vendor_name"`
	TotalPurchases     float64 `json:"total_purchases"`
	TotalPayments      float64 `json:"total_payments"`
	Outstanding        float64 `json:"outstanding"`
	AveragePaymentDays float64 `json:"average_payment_days"`
	PaymentScore       float64 `json:"payment_score"`
	Rating             string  `json:"rating"`
}

type PaymentAnalysisData struct {
	OnTimePayments     int64   `json:"on_time_payments"`
	LatePayments       int64   `json:"late_payments"`
	OverduePayments    int64   `json:"overdue_payments"`
	AveragePaymentDays float64 `json:"average_payment_days"`
	PaymentEfficiency  float64 `json:"payment_efficiency"`
}

type VendorSpendData struct {
	VendorID     uint    `json:"vendor_id"`
	VendorName   string  `json:"vendor_name"`
	TotalSpend   float64 `json:"total_spend"`
	Percentage   float64 `json:"percentage"`
	Transactions int64   `json:"transactions"`
}

type VendorPaymentHistory struct {
	Month       string  `json:"month"`
	Purchases   float64 `json:"purchases"`
	Payments    float64 `json:"payments"`
	Outstanding float64 `json:"outstanding"`
}

type TrialBalanceItem struct {
	AccountID     uint    `json:"account_id"`
	AccountCode   string  `json:"account_code"`
	AccountName   string  `json:"account_name"`
	AccountType   string  `json:"account_type"`
	Category      string  `json:"category"`
	DebitBalance  float64 `json:"debit_balance"`
	CreditBalance float64 `json:"credit_balance"`
	Level         int     `json:"level"`
	IsHeader      bool    `json:"is_header"`
}

type AccountTypeSummary struct {
	AccountType  string  `json:"account_type"`
	TotalDebits  float64 `json:"total_debits"`
	TotalCredits float64 `json:"total_credits"`
	NetBalance   float64 `json:"net_balance"`
	AccountCount int64   `json:"account_count"`
}

type GeneralLedgerEntry struct {
	JournalID    uint64    `json:"journal_id"`
	Date         time.Time `json:"date"`
	JournalCode  string    `json:"journal_code"`
	Description  string    `json:"description"`
	Reference    string    `json:"reference"`
	DebitAmount  float64   `json:"debit_amount"`
	CreditAmount float64   `json:"credit_amount"`
	Balance      float64   `json:"balance"`
	EntryType    string    `json:"entry_type"`
}

type MonthlyLedgerSummary struct {
	Month       string  `json:"month"`
	Year        int     `json:"year"`
	Debits      float64 `json:"debits"`
	Credits     float64 `json:"credits"`
	NetMovement float64 `json:"net_movement"`
	EndBalance  float64 `json:"end_balance"`
}

type JournalTypeData struct {
	ReferenceType string  `json:"reference_type"`
	Count         int64   `json:"count"`
	TotalAmount   float64 `json:"total_amount"`
	Percentage    float64 `json:"percentage"`
}

type JournalStatusData struct {
	Status      string  `json:"status"`
	Count       int64   `json:"count"`
	TotalAmount float64 `json:"total_amount"`
	Percentage  float64 `json:"percentage"`
}

type JournalUserData struct {
	UserID      uint    `json:"user_id"`
	UserName    string  `json:"user_name"`
	Count       int64   `json:"count"`
	TotalAmount float64 `json:"total_amount"`
	Percentage  float64 `json:"percentage"`
}

type JournalEntryDetail struct {
	ID           uint      `json:"id"`
	Code         string    `json:"code"`
	Date         time.Time `json:"date"`
	Description  string    `json:"description"`
	Reference    string    `json:"reference"`
	DebitAmount  float64   `json:"debit_amount"`
	CreditAmount float64   `json:"credit_amount"`
	Status       string    `json:"status"`
	User         string    `json:"user"`
}

type JournalComplianceData struct {
	BalancedEntries    int64    `json:"balanced_entries"`
	UnbalancedEntries  int64    `json:"unbalanced_entries"`
	ComplianceRate     float64  `json:"compliance_rate"`
	MissingReferences  int64    `json:"missing_references"`
	FutureDatedEntries int64    `json:"future_dated_entries"`
	ComplianceIssues   []string `json:"compliance_issues"`
}

type BalanceSheetSection struct {
	Name      string                 `json:"name"`
	Items     []BalanceSheetItem     `json:"items"`
	Subtotals []BalanceSheetSubtotal `json:"subtotals"`
	Total     float64                `json:"total"`
}

type BalanceSheetItem struct {
	AccountID uint    `json:"account_id"`
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Balance   float64 `json:"balance"`
	Category  string  `json:"category"`
	Level     int     `json:"level"`
	IsHeader  bool    `json:"is_header"`
}

type BalanceSheetSubtotal struct {
	Name     string  `json:"name"`
	Amount   float64 `json:"amount"`
	Category string  `json:"category"`
}

type PLSection struct {
	Name     string   `json:"name"`
	Items    []PLItem `json:"items"`
	Subtotal float64  `json:"subtotal"`
}

type PLItem struct {
	AccountID  uint    `json:"account_id"`
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Amount     float64 `json:"amount"`
	Category   string  `json:"category"`
	Percentage float64 `json:"percentage"`
}

type CashFlowSection struct {
	Name  string         `json:"name"`
	Items []CashFlowItem `json:"items"`
	Total float64        `json:"total"`
}

type CashFlowItem struct {
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Category    string  `json:"category"`
}

type PeriodData struct {
	Period           string    `json:"period"`
	StartDate        time.Time `json:"start_date"`
	EndDate          time.Time `json:"end_date"`
	Amount           float64   `json:"amount"`
	TotalSales       float64   `json:"total_sales"`
	Transactions     int64     `json:"transactions"`
	TransactionCount int64     `json:"transaction_count"`
	GrowthRate       float64   `json:"growth_rate"`
}

type CustomerSalesData struct {
	CustomerID        uint             `json:"customer_id"`
	CustomerName      string           `json:"customer_name"`
	TotalAmount       float64          `json:"total_sales"`
	TransactionCount  int64            `json:"transaction_count"`
	AverageOrder      float64          `json:"average_transaction"`
	PercentageOfTotal float64          `json:"percentage_of_total"`
	LastOrderDate     time.Time        `json:"last_order_date"`
	FirstOrderDate    time.Time        `json:"first_order_date"`
	Items             []SaleItemDetail `json:"items,omitempty"`
}

type SaleItemDetail struct {
	ProductID     uint      `json:"product_id"`
	ProductCode   string    `json:"product_code"`
	ProductName   string    `json:"product_name"`
	Quantity      float64   `json:"quantity"`
	UnitPrice     float64   `json:"unit_price"`
	TotalPrice    float64   `json:"total_price"`
	Unit          string    `json:"unit"`
	SaleDate      time.Time `json:"sale_date"`
	InvoiceNumber string    `json:"invoice_number,omitempty"`
}

type ProductSalesData struct {
	ProductID         uint    `json:"product_id"`
	ProductName       string  `json:"product_name"`
	QuantitySold      int64   `json:"quantity_sold"`
	TotalAmount       float64 `json:"total_amount"`
	TotalRevenue      float64 `json:"total_revenue"`
	AveragePrice      float64 `json:"average_price"`
	PercentageOfTotal float64 `json:"percentage_of_total"`
	TransactionCount  int64   `json:"transaction_count"`
}

type VendorPurchaseData struct {
	VendorID         uint      `json:"vendor_id"`
	VendorName       string    `json:"vendor_name"`
	TotalAmount      float64   `json:"total_amount"`
	TransactionCount int64     `json:"transaction_count"`
	AverageOrder     float64   `json:"average_order"`
	LastOrderDate    time.Time `json:"last_order_date"`
	FirstOrderDate   time.Time `json:"first_order_date"`
}

type CategoryPurchaseData struct {
	CategoryID       uint    `json:"category_id"`
	CategoryName     string  `json:"category_name"`
	TotalAmount      float64 `json:"total_amount"`
	TransactionCount int64   `json:"transaction_count"`
	Percentage       float64 `json:"percentage"`
}

type StatusData struct {
	Status     string  `json:"status"`
	Count      int64   `json:"count"`
	Amount     float64 `json:"amount"`
	Percentage float64 `json:"percentage"`
}

type TopPerformersData struct {
	TopCustomers   []CustomerSalesData `json:"top_customers"`
	TopProducts    []ProductSalesData  `json:"top_products"`
	TopSalespeople []SalespersonData   `json:"top_salespeople"`
}

type TopVendorsData struct {
	TopVendors    []VendorPurchaseData   `json:"top_vendors"`
	TopCategories []CategoryPurchaseData `json:"top_categories"`
	TopProducts   []ProductPurchaseData  `json:"top_products"`
}

type SalespersonData struct {
	SalespersonID    uint    `json:"salesperson_id"`
	SalespersonName  string  `json:"salesperson_name"`
	TotalSales       float64 `json:"total_sales"`
	TransactionCount int64   `json:"transaction_count"`
	Commission       float64 `json:"commission"`
}

type ProductPurchaseData struct {
	ProductID         uint    `json:"product_id"`
	ProductName       string  `json:"product_name"`
	QuantityPurchased int64   `json:"quantity_purchased"`
	TotalAmount       float64 `json:"total_amount"`
	AveragePrice      float64 `json:"average_price"`
}

type GrowthAnalysisData struct {
	MonthOverMonth     float64 `json:"month_over_month"`
	QuarterOverQuarter float64 `json:"quarter_over_quarter"`
	YearOverYear       float64 `json:"year_over_year"`
	TrendDirection     string  `json:"trend_direction"`
	SeasonalityIndex   float64 `json:"seasonality_index"`
}

type CostAnalysisData struct {
	TotalCostOfGoods   float64 `json:"total_cost_of_goods"`
	AverageCostPerUnit float64 `json:"average_cost_per_unit"`
	CostVariance       float64 `json:"cost_variance"`
	InflationImpact    float64 `json:"inflation_impact"`
}

// loadCompanyProfile loads the company profile for report headers
func (ers *EnhancedReportService) loadCompanyProfile() {
	var profile models.CompanyProfile
	if err := ers.db.First(&profile).Error; err != nil {
		// Check for existing company data from user account or other sources
		// This is a placeholder - in production, you might want to:
		// 1. Load from environment variables
		// 2. Load from initial setup wizard data
		// 3. Load from user registration information
		// 4. Provide a setup interface for users to configure

		// Create default profile using environment variables or fallbacks
		profile = models.CompanyProfile{
			Name:            ers.getDefaultCompanyName(),
			Address:         ers.getDefaultCompanyAddress(),
			City:            ers.getDefaultCompanyCity(),
			State:           ers.getDefaultState(),
			Country:         ers.getDefaultCountry(),
			PostalCode:      ers.getDefaultPostalCode(),
			Phone:           ers.getDefaultCompanyPhone(),
			Email:           ers.getDefaultCompanyEmail(),
			Website:         ers.getDefaultCompanyWebsite(),
			Currency:        ers.getDefaultCurrency(),
			FiscalYearStart: "01-01", // January 1st (standard in Indonesia)
			TaxNumber:       ers.getDefaultTaxNumber(),
		}

		// Save the default profile to database
		ers.db.Create(&profile)
	}
	ers.companyProfile = &profile
}

// getCompanyInfo returns company information structure
func (ers *EnhancedReportService) getCompanyInfo() CompanyInfo {
	// Prefer admin-configured Settings if available so all reports are consistent
	var settings models.Settings
	if err := ers.db.First(&settings).Error; err == nil && settings.CompanyName != "" {
		return CompanyInfo{
			Name:       settings.CompanyName,
			Address:    settings.CompanyAddress,
			City:       "", // Address may already include city; keep empty if not structured
			State:      "",
			PostalCode: "",
			Phone:      settings.CompanyPhone,
			Email:      settings.CompanyEmail,
			Website:    settings.CompanyWebsite,
			TaxNumber:  settings.TaxNumber,
		}
	}
	// Fallback to CompanyProfile if Settings is not set
	if ers.companyProfile != nil {
		return CompanyInfo{
			Name:       ers.companyProfile.Name,
			Address:    ers.companyProfile.Address,
			City:       ers.companyProfile.City,
			State:      ers.companyProfile.State,
			PostalCode: ers.companyProfile.PostalCode,
			Phone:      ers.companyProfile.Phone,
			Email:      ers.companyProfile.Email,
			Website:    ers.companyProfile.Website,
			TaxNumber:  ers.companyProfile.TaxNumber,
		}
	}
	// Ultimate fallback to sensible defaults
	return CompanyInfo{
		Name:       ers.getDefaultCompanyName(),
		Address:    ers.getDefaultCompanyAddress(),
		City:       ers.getDefaultCompanyCity(),
		State:      ers.getDefaultState(),
		PostalCode: ers.getDefaultPostalCode(),
		Phone:      ers.getDefaultCompanyPhone(),
		Email:      ers.getDefaultCompanyEmail(),
		Website:    ers.getDefaultCompanyWebsite(),
		TaxNumber:  ers.getDefaultTaxNumber(),
	}
}

// GenerateBalanceSheet creates a comprehensive balance sheet with proper accounting logic
func (ers *EnhancedReportService) GenerateBalanceSheet(asOfDate time.Time) (*BalanceSheetData, error) {
	// Get all accounts with their balances
	ctx := context.Background()
	accounts, err := ers.accountRepo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch accounts: %v", err)
	}

	// Initialize balance sheet structure
	balanceSheet := &BalanceSheetData{
		Company:     ers.getCompanyInfo(),
		AsOfDate:    asOfDate,
		Currency:    ers.companyProfile.Currency,
		GeneratedAt: time.Now(),
	}

	// Process each account and calculate balances
	var assets, liabilities, equity []BalanceSheetItem

	for _, account := range accounts {
		if !account.IsActive {
			continue
		}
		// Skip header/parent accounts - reports should only show detail accounts
		if account.IsHeader {
			continue
		}

		balance := ers.calculateAccountBalance(account.ID, asOfDate)

		// Skip accounts with zero balance (optional - can be configurable)
		if balance == 0 {
			continue
		}

		item := BalanceSheetItem{
			AccountID: account.ID,
			Code:      account.Code,
			Name:      account.Name,
			Balance:   balance,
			Category:  account.Category,
			Level:     account.Level,
			IsHeader:  account.IsHeader,
		}

		switch account.Type {
		case models.AccountTypeAsset:
			assets = append(assets, item)
			balanceSheet.TotalAssets += balance
		case models.AccountTypeLiability:
			liabilities = append(liabilities, item)
		case models.AccountTypeEquity:
			equity = append(equity, item)
			balanceSheet.TotalEquity += balance
		}
	}

	// Sort items by account code
	sort.Slice(assets, func(i, j int) bool { return assets[i].Code < assets[j].Code })
	sort.Slice(liabilities, func(i, j int) bool { return liabilities[i].Code < liabilities[j].Code })
	sort.Slice(equity, func(i, j int) bool { return equity[i].Code < equity[j].Code })

	// Build assets section with subtotals
	balanceSheet.Assets = ers.buildAssetsSection(assets)

	// Build liabilities section with subtotals
	balanceSheet.Liabilities = ers.buildLiabilitiesSection(liabilities)

	// Build equity section
	balanceSheet.Equity = ers.buildEquitySection(equity)

	// Calculate total liabilities and equity
	totalLiabilitiesAndEquity := balanceSheet.Liabilities.Total + balanceSheet.Equity.Total
	balanceSheet.TotalEquity = totalLiabilitiesAndEquity

	// Check if balance sheet is balanced
	balanceSheet.Difference = balanceSheet.TotalAssets - totalLiabilitiesAndEquity
	balanceSheet.IsBalanced = math.Abs(balanceSheet.Difference) < 0.01 // Allow for small rounding differences

	return balanceSheet, nil
}

// GenerateProfitLoss creates a comprehensive P&L statement with proper accounting logic
func (ers *EnhancedReportService) GenerateProfitLoss(startDate, endDate time.Time) (*ProfitLossData, error) {
	// Generate cache key
	if ers.cacheService != nil {
		cacheParams := map[string]interface{}{
			"start_date": startDate.Format("2006-01-02"),
			"end_date":   endDate.Format("2006-01-02"),
		}
		cacheKey := ers.cacheService.GenerateCacheKey("profit-loss", cacheParams)

		// Try to get from cache first
		if cachedData, found := ers.cacheService.Get(cacheKey); found {
			if profitLoss, ok := cachedData.(*ProfitLossData); ok {
				return profitLoss, nil
			}
		}

		// Generate report
		profitLoss, err := ers.generateProfitLossData(startDate, endDate)
		if err != nil {
			return nil, err
		}

		// Add validation warnings if validation service is available
		if ers.validationService != nil {
			validationReport, err := ers.validationService.ValidateReportData(startDate, endDate)
			if err == nil && validationReport != nil {
				// Add validation info to the profit & loss data
				profitLoss.ValidationReport = validationReport
				profitLoss.DataQualityScore = validationReport.HealthScore
				profitLoss.DataQualityWarnings = validationReport.Recommendations
			}
		}

		// Cache the result
		ers.cacheService.Set(cacheKey, "profit-loss", profitLoss)
		return profitLoss, nil
	}

	// Fallback without caching
	return ers.generateProfitLossData(startDate, endDate)
}

// generateProfitLossData is the actual implementation separated from caching logic
func (ers *EnhancedReportService) generateProfitLossData(startDate, endDate time.Time) (*ProfitLossData, error) {
	// Get all accounts with their period balances
	ctx := context.Background()
	accounts, err := ers.accountRepo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch accounts: %v", err)
	}

	// Initialize P&L structure
	profitLoss := &ProfitLossData{
		Company:     ers.getCompanyInfo(),
		StartDate:   startDate,
		EndDate:     endDate,
		Currency:    ers.companyProfile.Currency,
		GeneratedAt: time.Now(),
	}

	// Initialize categorized items
	var operatingRevenueItems, nonOperatingRevenueItems []PLItem
	var cogsItems, operatingExpenseItems, nonOperatingExpenseItems, taxExpenseItems []PLItem
	var totalOperatingRevenue, totalNonOperatingRevenue, totalCOGS float64
	var totalOperatingExpenses, totalNonOperatingExpenses, totalTaxExpenses float64

	for _, account := range accounts {
		if !account.IsActive {
			continue
		}
		// Skip header/parent accounts - reports should only show detail accounts
		if account.IsHeader {
			continue
		}

		// Calculate activity for the period
		balance := ers.calculateAccountBalanceForPeriod(account.ID, startDate, endDate)

		// Skip accounts with no activity
		if balance == 0 {
			continue
		}

		item := PLItem{
			AccountID: account.ID,
			Code:      account.Code,
			Name:      account.Name,
			Amount:    balance,
			Category:  account.Category,
		}

		switch account.Type {
		case models.AccountTypeRevenue:
			if ers.isOperatingRevenue(account.Category) {
				operatingRevenueItems = append(operatingRevenueItems, item)
				totalOperatingRevenue += balance
			} else {
				nonOperatingRevenueItems = append(nonOperatingRevenueItems, item)
				totalNonOperatingRevenue += balance
			}
		case models.AccountTypeExpense:
			if ers.isCOGS(account.Category) {
				cogsItems = append(cogsItems, item)
				totalCOGS += balance
			} else if ers.isTaxExpense(account.Category) {
				taxExpenseItems = append(taxExpenseItems, item)
				totalTaxExpenses += balance
			} else if ers.isOperatingExpense(account.Category) {
				operatingExpenseItems = append(operatingExpenseItems, item)
				totalOperatingExpenses += balance
			} else {
				nonOperatingExpenseItems = append(nonOperatingExpenseItems, item)
				totalNonOperatingExpenses += balance
			}
		}
	}

	// Calculate total revenue
	totalRevenue := totalOperatingRevenue + totalNonOperatingRevenue

	// Calculate percentages for all items
	ers.calculateItemPercentages(operatingRevenueItems, totalRevenue)
	ers.calculateItemPercentages(nonOperatingRevenueItems, totalRevenue)
	ers.calculateItemPercentages(cogsItems, totalRevenue)
	ers.calculateItemPercentages(operatingExpenseItems, totalRevenue)
	ers.calculateItemPercentages(nonOperatingExpenseItems, totalRevenue)
	ers.calculateItemPercentages(taxExpenseItems, totalRevenue)

	// Build comprehensive P&L sections
	profitLoss.Revenue = PLSection{
		Name:     "Revenue",
		Items:    append(operatingRevenueItems, nonOperatingRevenueItems...),
		Subtotal: totalRevenue,
	}

	profitLoss.CostOfGoodsSold = PLSection{
		Name:     "Cost of Goods Sold",
		Items:    cogsItems,
		Subtotal: totalCOGS,
	}

	profitLoss.OperatingExpenses = PLSection{
		Name:     "Operating Expenses",
		Items:    operatingExpenseItems,
		Subtotal: totalOperatingExpenses,
	}

	profitLoss.OtherIncome = PLSection{
		Name:     "Other Income",
		Items:    nonOperatingRevenueItems,
		Subtotal: totalNonOperatingRevenue,
	}

	profitLoss.OtherExpenses = PLSection{
		Name:     "Other Expenses",
		Items:    nonOperatingExpenseItems,
		Subtotal: totalNonOperatingExpenses,
	}

	// Calculate comprehensive financial metrics
	profitLoss.GrossProfit = totalOperatingRevenue - totalCOGS
	if totalOperatingRevenue != 0 {
		profitLoss.GrossProfitMargin = (profitLoss.GrossProfit / totalOperatingRevenue) * 100
	}

	// Calculate EBITDA (approximation without depreciation detail)
	profitLoss.EBITDA = profitLoss.GrossProfit - totalOperatingExpenses + totalNonOperatingRevenue - totalNonOperatingExpenses

	// Calculate EBIT
	profitLoss.EBIT = profitLoss.EBITDA // Simplified - would subtract depreciation and amortization

	// Calculate Operating Income
	profitLoss.OperatingIncome = profitLoss.GrossProfit - totalOperatingExpenses

	// Calculate Net Income Before Tax
	profitLoss.NetIncomeBeforeTax = profitLoss.OperatingIncome + totalNonOperatingRevenue - totalNonOperatingExpenses

	// Tax Expenses
	profitLoss.TaxExpense = totalTaxExpenses

	// Calculate Net Income
	profitLoss.NetIncome = profitLoss.NetIncomeBeforeTax - totalTaxExpenses

	if totalRevenue != 0 {
		profitLoss.NetIncomeMargin = (profitLoss.NetIncome / totalRevenue) * 100
	}

	// Calculate EPS with proper shares outstanding
	profitLoss.SharesOutstanding = ers.getSharesOutstanding()
	if profitLoss.SharesOutstanding > 0 {
		profitLoss.EarningsPerShare = profitLoss.NetIncome / profitLoss.SharesOutstanding
		profitLoss.DilutedEPS = profitLoss.EarningsPerShare // Simplified - would account for dilutive securities
	} else {
		profitLoss.EarningsPerShare = profitLoss.NetIncome
		profitLoss.DilutedEPS = profitLoss.NetIncome
	}

	return profitLoss, nil
}

// GenerateCashFlow creates a comprehensive cash flow statement
func (ers *EnhancedReportService) GenerateCashFlow(startDate, endDate time.Time) (*CashFlowData, error) {
	// Initialize cash flow structure
	cashFlow := &CashFlowData{
		Company:     ers.getCompanyInfo(),
		StartDate:   startDate,
		EndDate:     endDate,
		Currency:    ers.companyProfile.Currency,
		GeneratedAt: time.Now(),
	}

	// Get cash and cash equivalent accounts
	cashAccounts := ers.getCashAccounts()

	// Calculate beginning and ending cash balances
	prevDay := startDate.AddDate(0, 0, -1)
	cashFlow.BeginningCash = ers.calculateTotalCashBalance(cashAccounts, prevDay)
	cashFlow.EndingCash = ers.calculateTotalCashBalance(cashAccounts, endDate)

	// Build operating activities section
	operatingItems := ers.calculateOperatingCashFlow(startDate, endDate)
	cashFlow.OperatingActivities = CashFlowSection{
		Name:  "Operating Activities",
		Items: operatingItems,
		Total: ers.sumCashFlowItems(operatingItems),
	}

	// Build investing activities section
	investingItems := ers.calculateInvestingCashFlow(startDate, endDate)
	cashFlow.InvestingActivities = CashFlowSection{
		Name:  "Investing Activities",
		Items: investingItems,
		Total: ers.sumCashFlowItems(investingItems),
	}

	// Build financing activities section
	financingItems := ers.calculateFinancingCashFlow(startDate, endDate)
	cashFlow.FinancingActivities = CashFlowSection{
		Name:  "Financing Activities",
		Items: financingItems,
		Total: ers.sumCashFlowItems(financingItems),
	}

	// Calculate net cash flow
	cashFlow.NetCashFlow = cashFlow.OperatingActivities.Total +
		cashFlow.InvestingActivities.Total +
		cashFlow.FinancingActivities.Total

	return cashFlow, nil
}

// GenerateSalesSummary creates a comprehensive sales summary report with enhanced analytics and growth analysis
func (ers *EnhancedReportService) GenerateSalesSummary(startDate, endDate time.Time, groupBy string) (*SalesSummaryData, error) {
	// Enhanced date handling with timezone awareness
	dateUtils := utils.NewDateUtils()

	// Log the operation start
	startTime := time.Now()
	utils.ReportLog.WithFields(utils.Fields{
		"operation":  "GenerateSalesSummary",
		"start_date": startDate.In(utils.JakartaTZ).Format("2006-01-02 15:04:05 MST"),
		"end_date":   endDate.In(utils.JakartaTZ).Format("2006-01-02 15:04:05 MST"),
		"group_by":   groupBy,
	}).Info("Starting sales summary generation with timezone awareness")

	// Log input parameters for debugging
	utils.ReportLog.WithFields(utils.Fields{
		"input_start_date_utc": startDate.UTC().Format("2006-01-02 15:04:05 UTC"),
		"input_end_date_utc":   endDate.UTC().Format("2006-01-02 15:04:05 UTC"),
		"input_start_date_jkt": startDate.In(utils.JakartaTZ).Format("2006-01-02 15:04:05 MST"),
		"input_end_date_jkt":   endDate.In(utils.JakartaTZ).Format("2006-01-02 15:04:05 MST"),
		"formatted_range":      dateUtils.FormatDateRange(startDate, endDate),
	}).Debug("Sales summary date parameter details")

	// Validate date range
	if err := dateUtils.ValidateDateRange(startDate, endDate); err != nil {
		utils.ReportLog.LogReportError("GenerateSalesSummary", err, utils.Fields{
			"start_date": startDate.In(utils.JakartaTZ).Format("2006-01-02"),
			"end_date":   endDate.In(utils.JakartaTZ).Format("2006-01-02"),
		})
		return nil, fmt.Errorf("invalid date range: %v", err)
	}

	// Query sales data with detailed logging and enhanced timezone handling
	var sales []models.Sale
	queryStartTime := time.Now()

	query := ers.db.Preload("Customer").
		Preload("SaleItems").
		Preload("SaleItems.Product").
		Preload("SalesPerson")

	// Log the exact query parameters
	utils.ReportLog.WithFields(utils.Fields{
		"query_start_date": startDate.Format("2006-01-02 15:04:05"),
		"query_end_date":   endDate.Format("2006-01-02 15:04:05"),
		"query_sql_filter": fmt.Sprintf("date >= '%s' AND date <= '%s'",
			startDate.Format("2006-01-02 15:04:05"),
			endDate.Format("2006-01-02 15:04:05")),
	}).Debug("Executing sales data query")

	// Use timezone-aware date range for better accuracy
	queryResult := query.Where("date >= ? AND date <= ?", startDate, endDate).Find(&sales)

	queryDuration := time.Since(queryStartTime)
	utils.ReportLog.LogQueryPerformance("SalesQuery", queryDuration, len(sales))

	if queryResult.Error != nil {
		utils.ReportLog.LogReportError("GenerateSalesSummary", queryResult.Error, utils.Fields{
			"start_date":     startDate.In(utils.JakartaTZ).Format("2006-01-02"),
			"end_date":       endDate.In(utils.JakartaTZ).Format("2006-01-02"),
			"group_by":       groupBy,
			"query_duration": queryDuration.String(),
		})
		return nil, fmt.Errorf("failed to fetch sales data: %v", queryResult.Error)
	}

	// Log query results with detailed analysis
	totalAmount := float64(0)
	uniqueCustomers := make(map[uint]bool)
	statusBreakdown := make(map[string]int)
	for _, sale := range sales {
		totalAmount += sale.TotalAmount
		uniqueCustomers[sale.CustomerID] = true
		statusBreakdown[sale.Status]++
	}

	utils.ReportLog.LogSalesQuery("SalesDataQuery", startDate, endDate, groupBy, len(sales), totalAmount)

	// Log additional insights about the data retrieved
	utils.ReportLog.WithFields(utils.Fields{
		"total_records":      len(sales),
		"total_amount":       totalAmount,
		"unique_customers":   len(uniqueCustomers),
		"status_breakdown":   statusBreakdown,
		"data_quality_score": calculateDataQualityScore(sales),
	}).Info("Sales data query completed with analysis")

	// Handle case when no data is found - provide informative response
	if len(sales) == 0 {
		utils.ReportLog.WithFields(utils.Fields{
			"start_date": startDate.Format("2006-01-02"),
			"end_date":   endDate.Format("2006-01-02"),
			"group_by":   groupBy,
		}).Warn("No sales data found for the specified period")

		// Return empty but valid response with helpful message
		emptySummary := &SalesSummaryData{
			Company:           ers.getCompanyInfo(),
			StartDate:         startDate,
			EndDate:           endDate,
			Currency:          ers.getCurrencyFromSettings(),
			TotalTransactions: 0,
			TotalRevenue:      0,
			TotalCustomers:    0,
			AverageOrderValue: 0,
			SalesByPeriod:     make([]PeriodData, 0),
			SalesByCustomer:   make([]CustomerSalesData, 0),
			SalesByProduct:    make([]ProductSalesData, 0),
			SalesByStatus:     make([]StatusData, 0),
			GeneratedAt:       time.Now().In(utils.JakartaTZ),
		}

		// Add helpful debugging information
		emptySummary.DebugInfo = map[string]interface{}{
			"message": fmt.Sprintf("No sales data found for period %s to %s",
				startDate.In(utils.JakartaTZ).Format("2006-01-02"),
				endDate.In(utils.JakartaTZ).Format("2006-01-02")),
			"suggestions": []string{
				"Check if there are any sales records in the database for this period",
				"Verify the date range is correct",
				"Ensure sales records have the correct date format",
				"Check if there are any timezone-related issues with date filtering",
			},
			"date_range_info": map[string]interface{}{
				"start_date_jakarta": startDate.In(utils.JakartaTZ).Format("2006-01-02 15:04:05 MST"),
				"end_date_jakarta":   endDate.In(utils.JakartaTZ).Format("2006-01-02 15:04:05 MST"),
				"query_used": fmt.Sprintf("date >= '%s' AND date <= '%s'",
					startDate.Format("2006-01-02 15:04:05"),
					endDate.Format("2006-01-02 15:04:05")),
				"timezone": "Asia/Jakarta (WIB)",
			},
			"query_performance": map[string]interface{}{
				"query_duration": queryDuration.String(),
				"records_found":  0,
			},
		}

		processingTime := time.Since(startTime)
		utils.ReportLog.LogReportGeneration("SalesSummary", utils.Fields{
			"start_date":    startDate.Format("2006-01-02"),
			"end_date":      endDate.Format("2006-01-02"),
			"group_by":      groupBy,
			"records_found": 0,
			"has_data":      false,
		}, processingTime, true)

		return emptySummary, nil
	}

	// Initialize sales summary with data
	summary := &SalesSummaryData{
		Company:           ers.getCompanyInfo(),
		StartDate:         startDate,
		EndDate:           endDate,
		Currency:          ers.getCurrencyFromSettings(),
		TotalTransactions: int64(len(sales)),
		GeneratedAt:       time.Now().In(utils.JakartaTZ),
	}

	// Calculate basic metrics
	customerSet := make(map[uint]bool)
	productMap := make(map[uint]*ProductSalesData)
	customerMap := make(map[uint]*CustomerSalesData)
	periodMap := make(map[string]*PeriodData)
	statusMap := make(map[string]*StatusData)

	for _, sale := range sales {
		summary.TotalRevenue += sale.TotalAmount
		// Sum tax: prioritize PPNAmount, fallback to TotalTax, then Tax/PPN
		taxForSale := sale.PPNAmount
		if taxForSale == 0 {
			taxForSale = sale.TotalTax
		}
		if taxForSale == 0 {
			taxForSale = sale.PPN
		}
		if taxForSale == 0 {
			taxForSale = sale.Tax
		}
		summary.TotalTax += taxForSale
		// Sum discount
		summary.TotalDiscount += sale.DiscountAmount
		customerSet[sale.CustomerID] = true

		// Process customer data
		if customerData, exists := customerMap[sale.CustomerID]; exists {
			customerData.TotalAmount += sale.TotalAmount
			customerData.TransactionCount++
			if sale.Date.After(customerData.LastOrderDate) {
				customerData.LastOrderDate = sale.Date
			}
			if sale.Date.Before(customerData.FirstOrderDate) {
				customerData.FirstOrderDate = sale.Date
			}
		} else {
			customerMap[sale.CustomerID] = &CustomerSalesData{
				CustomerID:       sale.CustomerID,
				CustomerName:     sale.Customer.Name,
				TotalAmount:      sale.TotalAmount,
				TransactionCount: 1,
				LastOrderDate:    sale.Date,
				FirstOrderDate:   sale.Date,
			}
		}

		// Process period data using enhanced date utilities
		dateUtils := utils.NewDateUtils()
		period := dateUtils.FormatPeriodWithTZ(sale.Date, groupBy)
		if periodData, exists := periodMap[period]; exists {
			periodData.Amount += sale.TotalAmount
			periodData.Transactions++
		} else {
			startDate, endDate := dateUtils.GetPeriodBounds(sale.Date, groupBy)
			periodMap[period] = &PeriodData{
				Period:       period,
				Amount:       sale.TotalAmount,
				Transactions: 1,
				StartDate:    startDate,
				EndDate:      endDate,
			}
		}

		// Process status data
		if statusData, exists := statusMap[sale.Status]; exists {
			statusData.Count++
			statusData.Amount += sale.TotalAmount
		} else {
			statusMap[sale.Status] = &StatusData{
				Status: sale.Status,
				Count:  1,
				Amount: sale.TotalAmount,
			}
		}

		// Process product data
		for _, item := range sale.SaleItems {
			if productData, exists := productMap[item.ProductID]; exists {
				productData.QuantitySold += int64(item.Quantity)
				productData.TotalAmount += item.LineTotal
				productData.TransactionCount++
			} else {
				productMap[item.ProductID] = &ProductSalesData{
					ProductID:        item.ProductID,
					ProductName:      item.Product.Name,
					QuantitySold:     int64(item.Quantity),
					TotalAmount:      item.LineTotal,
					TransactionCount: 1,
				}
			}
		}
	}

	// Calculate derived metrics
	summary.TotalCustomers = int64(len(customerSet))
	if summary.TotalTransactions > 0 {
		summary.AverageOrderValue = summary.TotalRevenue / float64(summary.TotalTransactions)
	}

	// Calculate TotalSales (bruto = includes tax) and NetRevenue (after tax+discount)
	summary.TotalSales = summary.TotalRevenue
	summary.NetRevenue = summary.TotalRevenue - summary.TotalTax - summary.TotalDiscount

	// Calculate average prices and order values
	for _, customerData := range customerMap {
		if customerData.TransactionCount > 0 {
			customerData.AverageOrder = customerData.TotalAmount / float64(customerData.TransactionCount)
		}
		// Calculate percentage of total
		if summary.TotalRevenue > 0 {
			customerData.PercentageOfTotal = (customerData.TotalAmount / summary.TotalRevenue) * 100
		}
	}

	// Calculate total product amount for percentage denominator (line totals, before tax)
	totalProductAmount := float64(0)
	for _, productData := range productMap {
		totalProductAmount += productData.TotalAmount
	}

	for _, productData := range productMap {
		if productData.QuantitySold > 0 {
			productData.AveragePrice = productData.TotalAmount / float64(productData.QuantitySold)
		}
		// Sync TotalRevenue with TotalAmount
		productData.TotalRevenue = productData.TotalAmount
		// Calculate percentage of total (against sum of all product line totals, not total revenue with tax)
		if totalProductAmount > 0 {
			productData.PercentageOfTotal = (productData.TotalAmount / totalProductAmount) * 100
		}
	}

	// Calculate percentages for status data
	for _, statusData := range statusMap {
		if summary.TotalRevenue > 0 {
			statusData.Percentage = (statusData.Amount / summary.TotalRevenue) * 100
		}
	}

	// Convert maps to slices and sort
	summary.SalesByCustomer = ers.sortCustomersByRevenue(customerMap)

	// Fetch items for each customer
	for i := range summary.SalesByCustomer {
		items, err := ers.getSaleItemsForCustomer(summary.SalesByCustomer[i].CustomerID, startDate, endDate)
		if err != nil {
			utils.ReportLog.WithFields(utils.Fields{
				"customer_id":   summary.SalesByCustomer[i].CustomerID,
				"customer_name": summary.SalesByCustomer[i].CustomerName,
				"error":         err.Error(),
			}).Warn("Failed to fetch sale items for customer")
			continue
		}
		summary.SalesByCustomer[i].Items = items
		if len(items) > 0 {
			utils.ReportLog.WithFields(utils.Fields{
				"customer_name": summary.SalesByCustomer[i].CustomerName,
				"items_count":   len(items),
			}).Debug("Loaded sale items for customer")
		}
	}

	summary.SalesByProduct = ers.sortProductsBySales(productMap)
	summary.SalesByPeriod = ers.sortPeriodsByDate(periodMap)
	summary.SalesByStatus = ers.convertStatusMapToSlice(statusMap)

	// Sync PeriodData fields: copy Amount to TotalSales, Transactions to TransactionCount
	for i := range summary.SalesByPeriod {
		summary.SalesByPeriod[i].TotalSales = summary.SalesByPeriod[i].Amount
		summary.SalesByPeriod[i].TransactionCount = summary.SalesByPeriod[i].Transactions
	}

	// Calculate growth rates for periods
	for i := 1; i < len(summary.SalesByPeriod); i++ {
		prev := summary.SalesByPeriod[i-1].Amount
		if prev > 0 {
			summary.SalesByPeriod[i].GrowthRate = ((summary.SalesByPeriod[i].Amount - prev) / prev) * 100
		}
	}

	// Build items list from sales
	for _, sale := range sales {
		custName := ""
		if sale.Customer.Name != "" {
			custName = sale.Customer.Name
		}
		summary.Items = append(summary.Items, SalesSummaryItem{
			Date:         sale.Date.Format("2006-01-02"),
			Code:         sale.InvoiceNumber,
			CustomerName: custName,
			TotalAmount:  sale.TotalAmount,
			Status:       sale.Status,
		})
	}

	// Build top performers data
	summary.TopPerformers = ers.buildTopPerformers(summary.SalesByCustomer, summary.SalesByProduct)

	// Calculate growth analysis
	summary.GrowthAnalysis = ers.calculateGrowthAnalysis(startDate, endDate, summary.TotalRevenue)

	// Enhanced processing with data quality analysis
	processingTime := time.Since(startTime)
	summary.ProcessingTime = processingTime.String()

	// Analyze data quality
	dataQualityIssues := ers.analyzeDataQuality(sales)
	validRecords := len(sales) - len(dataQualityIssues)
	summary.DataQualityScore = (float64(validRecords) / float64(len(sales))) * 100

	if len(dataQualityIssues) > 0 {
		utils.ReportLog.LogDataQuality("SalesSummary", dataQualityIssues, len(sales), validRecords)
	}

	// Add debug information for successful generation
	summary.DebugInfo = map[string]interface{}{
		"message": fmt.Sprintf("Successfully generated sales summary for %d records", len(sales)),
		"date_range_info": map[string]interface{}{
			"start_date_jakarta": startDate.In(utils.JakartaTZ).Format("2006-01-02 15:04:05 MST"),
			"end_date_jakarta":   endDate.In(utils.JakartaTZ).Format("2006-01-02 15:04:05 MST"),
			"timezone":           "Asia/Jakarta (WIB)",
		},
		"query_performance": map[string]interface{}{
			"query_duration":        queryDuration.String(),
			"total_processing_time": processingTime.String(),
			"records_processed":     len(sales),
		},
		"data_summary": map[string]interface{}{
			"total_revenue":    summary.TotalRevenue,
			"total_customers":  summary.TotalCustomers,
			"avg_order_value":  summary.AverageOrderValue,
			"periods_analyzed": len(summary.SalesByPeriod),
		},
	}

	// Log successful generation
	utils.ReportLog.LogReportGeneration("SalesSummary", utils.Fields{
		"start_date":         startDate.Format("2006-01-02"),
		"end_date":           endDate.Format("2006-01-02"),
		"group_by":           groupBy,
		"records_found":      len(sales),
		"total_revenue":      summary.TotalRevenue,
		"total_customers":    summary.TotalCustomers,
		"data_quality_score": summary.DataQualityScore,
		"has_data":           true,
	}, processingTime, true)

	return summary, nil
}

// calculateDataQualityScore calculates a data quality score for sales records
func calculateDataQualityScore(sales []models.Sale) float64 {
	if len(sales) == 0 {
		return 100.0 // Perfect score for empty data
	}

	var issues int
	totalChecks := len(sales) * 5 // 5 checks per sale: code, customer, amount, date, status

	for _, sale := range sales {
		// Check for missing sale code
		if sale.Code == "" {
			issues++
		}

		// Check for missing customer
		if sale.CustomerID == 0 {
			issues++
		}

		// Check for negative amounts
		if sale.TotalAmount < 0 {
			issues++
		}

		// Check for future dates
		if sale.Date.After(time.Now().In(utils.JakartaTZ)) {
			issues++
		}

		// Check for valid status
		validStatuses := []string{"DRAFT", "PENDING", "CONFIRMED", "COMPLETED", "CANCELLED", "INVOICED", "OVERDUE", "PAID"}
		validStatus := false
		for _, status := range validStatuses {
			if sale.Status == status {
				validStatus = true
				break
			}
		}
		if !validStatus {
			issues++
		}
	}

	if totalChecks == 0 {
		return 100.0
	}

	qualityScore := ((float64(totalChecks - issues)) / float64(totalChecks)) * 100
	if qualityScore < 0 {
		return 0.0
	}
	return qualityScore
}

// GenerateVendorAnalysis creates comprehensive vendor analysis report
func (ers *EnhancedReportService) GenerateVendorAnalysis(startDate, endDate time.Time) (*VendorAnalysisData, error) {
	// Query purchase data
	var purchases []models.Purchase
	if err := ers.db.Preload("Vendor").
		Preload("PurchaseItems").
		Preload("PurchaseItems.Product").
		Where("date BETWEEN ? AND ?", startDate, endDate).
		Find(&purchases).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch purchase data: %v", err)
	}

	// Query payment data for vendor analysis through purchase payments
	var purchasePayments []models.PurchasePayment
	if err := ers.db.Preload("Payment").
		Where("date BETWEEN ? AND ?", startDate, endDate).
		Find(&purchasePayments).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch purchase payment data: %v", err)
	}

	// Initialize vendor analysis
	analysis := &VendorAnalysisData{
		Company:     ers.getCompanyInfo(),
		StartDate:   startDate,
		EndDate:     endDate,
		Currency:    ers.companyProfile.Currency,
		GeneratedAt: time.Now(),
	}

	// Process vendor data
	vendorMap := make(map[uint]*VendorPerformanceData)
	vendorSpendMap := make(map[uint]*VendorSpendData)
	totalPurchases := 0.0
	totalPayments := 0.0

	for _, purchase := range purchases {
		analysis.TotalPurchases += purchase.TotalAmount
		totalPurchases += purchase.TotalAmount

		// Track vendor performance
		if vendorData, exists := vendorMap[purchase.VendorID]; exists {
			vendorData.TotalPurchases += purchase.TotalAmount
		} else {
			vendorMap[purchase.VendorID] = &VendorPerformanceData{
				VendorID:       purchase.VendorID,
				VendorName:     purchase.Vendor.Name,
				TotalPurchases: purchase.TotalAmount,
			}
		}

		// Track vendor spend
		if spendData, exists := vendorSpendMap[purchase.VendorID]; exists {
			spendData.TotalSpend += purchase.TotalAmount
			spendData.Transactions++
		} else {
			vendorSpendMap[purchase.VendorID] = &VendorSpendData{
				VendorID:     purchase.VendorID,
				VendorName:   purchase.Vendor.Name,
				TotalSpend:   purchase.TotalAmount,
				Transactions: 1,
			}
		}
	}

	// Process payment data from purchase payments
	for _, purchasePayment := range purchasePayments {
		totalPayments += purchasePayment.Amount

		// Get the purchase to find the vendor
		var purchase models.Purchase
		if err := ers.db.First(&purchase, purchasePayment.PurchaseID).Error; err != nil {
			continue // Skip if purchase not found
		}

		if vendorData, exists := vendorMap[purchase.VendorID]; exists {
			vendorData.TotalPayments += purchasePayment.Amount
		}
	}

	analysis.TotalPayments = totalPayments
	analysis.OutstandingPayables = totalPurchases - totalPayments

	// Calculate vendor metrics
	for _, vendor := range vendorMap {
		vendor.Outstanding = vendor.TotalPurchases - vendor.TotalPayments
		vendor.PaymentScore = ers.calculatePaymentScore(vendor)
		vendor.Rating = ers.getVendorRating(vendor.PaymentScore)
	}

	// Calculate percentages for spend data
	for _, spendData := range vendorSpendMap {
		if totalPurchases > 0 {
			spendData.Percentage = (spendData.TotalSpend / totalPurchases) * 100
		}
	}

	// Convert maps to sorted slices
	analysis.VendorsByPerformance = ers.sortVendorsByPerformance(vendorMap)
	analysis.TopVendorsBySpend = ers.sortVendorsBySpend(vendorSpendMap)

	// Calculate payment analysis using purchase payments
	analysis.PaymentAnalysis = ers.calculatePaymentAnalysisFromPurchasePayments(purchasePayments)

	// Build vendor payment history
	analysis.VendorPaymentHistory = ers.buildVendorPaymentHistory(startDate, endDate)

	// Calculate totals
	analysis.TotalVendors = int64(len(vendorMap))
	analysis.ActiveVendors = ers.countActiveVendors(vendorMap)

	return analysis, nil
}

// analyzeDataQuality performs data quality analysis on sales records
func (ers *EnhancedReportService) analyzeDataQuality(sales []models.Sale) []string {
	var issues []string

	for _, sale := range sales {
		// Check for missing required fields
		if sale.Code == "" {
			issues = append(issues, fmt.Sprintf("Sale ID %d: Missing sale code", sale.ID))
		}

		if sale.CustomerID == 0 {
			issues = append(issues, fmt.Sprintf("Sale %s: Missing customer ID", sale.Code))
		}

		// Check for unrealistic amounts
		if sale.TotalAmount < 0 {
			issues = append(issues, fmt.Sprintf("Sale %s: Negative total amount (%.2f)", sale.Code, sale.TotalAmount))
		}

		if sale.TotalAmount > 1000000000 { // More than 1 billion IDR might be suspicious
			issues = append(issues, fmt.Sprintf("Sale %s: Unusually large amount (%.2f)", sale.Code, sale.TotalAmount))
		}

		// Check for future dates
		if sale.Date.After(time.Now().In(utils.JakartaTZ)) {
			issues = append(issues, fmt.Sprintf("Sale %s: Future date (%s)", sale.Code, sale.Date.Format("2006-01-02")))
		}

		// Check for very old dates (more than 10 years ago)
		tenYearsAgo := time.Now().In(utils.JakartaTZ).AddDate(-10, 0, 0)
		if sale.Date.Before(tenYearsAgo) {
			issues = append(issues, fmt.Sprintf("Sale %s: Very old date (%s)", sale.Code, sale.Date.Format("2006-01-02")))
		}

		// Check for inconsistent status
		validStatuses := []string{"DRAFT", "PENDING", "CONFIRMED", "COMPLETED", "CANCELLED", "INVOICED", "OVERDUE", "PAID"}
		validStatus := false
		for _, status := range validStatuses {
			if sale.Status == status {
				validStatus = true
				break
			}
		}
		if !validStatus {
			issues = append(issues, fmt.Sprintf("Sale %s: Invalid status (%s)", sale.Code, sale.Status))
		}

		// Check for inconsistent amounts
		if sale.PaidAmount > sale.TotalAmount {
			issues = append(issues, fmt.Sprintf("Sale %s: Paid amount (%.2f) exceeds total amount (%.2f)", sale.Code, sale.PaidAmount, sale.TotalAmount))
		}

		expectedOutstanding := sale.TotalAmount - sale.PaidAmount
		if math.Abs(sale.OutstandingAmount-expectedOutstanding) > 0.01 { // Allow for small rounding errors
			issues = append(issues, fmt.Sprintf("Sale %s: Outstanding amount mismatch (expected %.2f, got %.2f)", sale.Code, expectedOutstanding, sale.OutstandingAmount))
		}
	}

	return issues
}

// GenerateTrialBalance creates comprehensive trial balance report
func (ers *EnhancedReportService) GenerateTrialBalance(asOfDate time.Time) (*TrialBalanceData, error) {
	// Get all accounts
	ctx := context.Background()
	accounts, err := ers.accountRepo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch accounts: %v", err)
	}

	// Initialize trial balance
	trialBalance := &TrialBalanceData{
		Company:     ers.getCompanyInfo(),
		AsOfDate:    asOfDate,
		Currency:    ers.companyProfile.Currency,
		GeneratedAt: time.Now(),
	}

	// Track account type summaries
	accountTypeSummaries := make(map[string]*AccountTypeSummary)

	// Process each account
	for _, account := range accounts {
		if !account.IsActive {
			continue
		}
		// Skip header/parent accounts - reports should only show detail accounts
		if account.IsHeader {
			continue
		}

		balance := ers.calculateAccountBalance(account.ID, asOfDate)
		normalBalance := account.GetNormalBalance()

		item := TrialBalanceItem{
			AccountID:   account.ID,
			AccountCode: account.Code,
			AccountName: account.Name,
			AccountType: account.Type,
			Category:    account.Category,
			Level:       account.Level,
			IsHeader:    account.IsHeader,
		}

		// Set debit or credit balance based on normal balance type
		if normalBalance == models.NormalBalanceDebit {
			item.DebitBalance = balance
			trialBalance.TotalDebits += balance
		} else {
			item.CreditBalance = balance
			trialBalance.TotalCredits += balance
		}

		trialBalance.Accounts = append(trialBalance.Accounts, item)

		// Update account type summary
		if summary, exists := accountTypeSummaries[account.Type]; exists {
			if normalBalance == models.NormalBalanceDebit {
				summary.TotalDebits += balance
			} else {
				summary.TotalCredits += balance
			}
			summary.AccountCount++
		} else {
			summary := &AccountTypeSummary{
				AccountType:  account.Type,
				AccountCount: 1,
			}
			if normalBalance == models.NormalBalanceDebit {
				summary.TotalDebits = balance
			} else {
				summary.TotalCredits = balance
			}
			accountTypeSummaries[account.Type] = summary
		}
	}

	// Calculate net balance for each account type
	for _, summary := range accountTypeSummaries {
		summary.NetBalance = summary.TotalDebits - summary.TotalCredits
	}

	// Set account type summaries
	if assetSummary, exists := accountTypeSummaries[models.AccountTypeAsset]; exists {
		trialBalance.AssetSummary = *assetSummary
	}
	if liabilitySummary, exists := accountTypeSummaries[models.AccountTypeLiability]; exists {
		trialBalance.LiabilitySummary = *liabilitySummary
	}
	if equitySummary, exists := accountTypeSummaries[models.AccountTypeEquity]; exists {
		trialBalance.EquitySummary = *equitySummary
	}
	if revenueSummary, exists := accountTypeSummaries[models.AccountTypeRevenue]; exists {
		trialBalance.RevenueSummary = *revenueSummary
	}
	if expenseSummary, exists := accountTypeSummaries[models.AccountTypeExpense]; exists {
		trialBalance.ExpenseSummary = *expenseSummary
	}

	// Check if trial balance is balanced
	trialBalance.Difference = trialBalance.TotalDebits - trialBalance.TotalCredits
	trialBalance.IsBalanced = math.Abs(trialBalance.Difference) < 0.01

	// Sort accounts by code
	sort.Slice(trialBalance.Accounts, func(i, j int) bool {
		return trialBalance.Accounts[i].AccountCode < trialBalance.Accounts[j].AccountCode
	})

	return trialBalance, nil
}

// GenerateGeneralLedger creates detailed general ledger report for specific account
func (ers *EnhancedReportService) GenerateGeneralLedger(accountID uint, startDate, endDate time.Time) (*GeneralLedgerData, error) {
	// Get account details
	ctx := context.Background()
	account, err := ers.accountRepo.FindByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch account: %v", err)
	}

	// Query journal lines from SSOT for this account
	var journalLines []models.SSOTJournalLine
	if err := ers.db.Preload("Journal").
		Preload("Journal.Creator").
		Joins("JOIN unified_journal_ledger ON unified_journal_lines.journal_id = unified_journal_ledger.id").
		Where("unified_journal_lines.account_id = ? AND unified_journal_ledger.entry_date BETWEEN ? AND ?", accountID, startDate, endDate).
		Where("unified_journal_ledger.status = ? AND unified_journal_ledger.deleted_at IS NULL", models.SSOTStatusPosted).
		Order("unified_journal_ledger.entry_date ASC, unified_journal_ledger.created_at ASC").
		Find(&journalLines).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch journal entries: %v", err)
	}

	// Initialize general ledger
	generalLedger := &GeneralLedgerData{
		Company:     ers.getCompanyInfo(),
		Account:     *account,
		StartDate:   startDate,
		EndDate:     endDate,
		Currency:    ers.companyProfile.Currency,
		GeneratedAt: time.Now(),
	}

	// Calculate opening balance
	prevDay := startDate.AddDate(0, 0, -1)
	generalLedger.OpeningBalance = ers.calculateAccountBalance(accountID, prevDay)

	// Process journal entries
	runningBalance := generalLedger.OpeningBalance
	monthlyMap := make(map[string]*MonthlyLedgerSummary)

	for _, line := range journalLines {
		if line.Journal == nil {
			continue
		}
		debitAmt := line.DebitAmount.InexactFloat64()
		creditAmt := line.CreditAmount.InexactFloat64()

		// Update running balance
		if debitAmt > 0 {
			if account.GetNormalBalance() == models.NormalBalanceDebit {
				runningBalance += debitAmt
			} else {
				runningBalance -= debitAmt
			}
			generalLedger.TotalDebits += debitAmt
		} else {
			if account.GetNormalBalance() == models.NormalBalanceCredit {
				runningBalance += creditAmt
			} else {
				runningBalance -= creditAmt
			}
			generalLedger.TotalCredits += creditAmt
		}

		// Create transaction entry
		entry := GeneralLedgerEntry{
			Date:         line.Journal.EntryDate,
			JournalCode:  line.Journal.EntryNumber,
			Description:  line.Description,
			Reference:    line.Journal.Reference,
			DebitAmount:  debitAmt,
			CreditAmount: creditAmt,
			Balance:      runningBalance,
			EntryType:    line.Journal.SourceType,
		}
		generalLedger.Transactions = append(generalLedger.Transactions, entry)

		// Update monthly summary
		monthKey := line.Journal.EntryDate.Format("2006-01")
		if summary, exists := monthlyMap[monthKey]; exists {
			summary.Debits += debitAmt
			summary.Credits += creditAmt
			summary.NetMovement = summary.Debits - summary.Credits
			summary.EndBalance = runningBalance
		} else {
			monthlyMap[monthKey] = &MonthlyLedgerSummary{
				Month:       line.Journal.EntryDate.Format("January"),
				Year:        line.Journal.EntryDate.Year(),
				Debits:      debitAmt,
				Credits:     creditAmt,
				NetMovement: debitAmt - creditAmt,
				EndBalance:  runningBalance,
			}
		}
	}

	generalLedger.ClosingBalance = runningBalance

	// Convert monthly map to sorted slice
	for _, summary := range monthlyMap {
		generalLedger.MonthlySummary = append(generalLedger.MonthlySummary, *summary)
	}
	sort.Slice(generalLedger.MonthlySummary, func(i, j int) bool {
		return generalLedger.MonthlySummary[i].Year < generalLedger.MonthlySummary[j].Year ||
			(generalLedger.MonthlySummary[i].Year == generalLedger.MonthlySummary[j].Year &&
				generalLedger.MonthlySummary[i].Month < generalLedger.MonthlySummary[j].Month)
	})

	return generalLedger, nil
}

// GenerateGeneralLedgerAll creates general ledger report for all accounts with transactions
func (ers *EnhancedReportService) GenerateGeneralLedgerAll(startDate, endDate time.Time) (*GeneralLedgerAllData, error) {
	// Query accounts that have SSOT journal entries in the period
	var accounts []models.Account
	if err := ers.db.Select("DISTINCT accounts.*").
		Joins("JOIN unified_journal_lines ON accounts.id = unified_journal_lines.account_id").
		Joins("JOIN unified_journal_ledger ON unified_journal_lines.journal_id = unified_journal_ledger.id").
		Where("unified_journal_ledger.entry_date BETWEEN ? AND ?", startDate, endDate).
		Where("unified_journal_ledger.status = ? AND unified_journal_ledger.deleted_at IS NULL", models.SSOTStatusPosted).
		Where("accounts.is_active = true AND accounts.is_header = false").
		Order("accounts.code ASC").
		Find(&accounts).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch accounts: %v", err)
	}

	// Initialize general ledger for all accounts
	generalLedgerAll := &GeneralLedgerAllData{
		Company:      ers.getCompanyInfo(),
		StartDate:    startDate,
		EndDate:      endDate,
		Currency:     ers.companyProfile.Currency,
		AccountCount: len(accounts),
		GeneratedAt:  time.Now(),
	}

	// Generate individual general ledgers for each account
	for _, account := range accounts {
		accountLedger, err := ers.GenerateGeneralLedger(account.ID, startDate, endDate)
		if err != nil {
			// Log error but continue with other accounts
			fmt.Printf("Warning: Failed to generate ledger for account %s: %v\n", account.Code, err)
			continue
		}

		// Only include accounts with transactions
		if len(accountLedger.Transactions) > 0 {
			generalLedgerAll.Accounts = append(generalLedgerAll.Accounts, *accountLedger)
			generalLedgerAll.TotalDebits += accountLedger.TotalDebits
			generalLedgerAll.TotalCredits += accountLedger.TotalCredits
			generalLedgerAll.TotalTransactions += len(accountLedger.Transactions)
		}
	}

	// Update account count to reflect only accounts with transactions
	generalLedgerAll.AccountCount = len(generalLedgerAll.Accounts)

	return generalLedgerAll, nil
}

// GetValidationReport provides data quality validation for the specified period
func (ers *EnhancedReportService) GetValidationReport(startDate, endDate time.Time) (*ValidationReport, error) {
	if ers.validationService == nil {
		return nil, fmt.Errorf("validation service not available")
	}

	return ers.validationService.ValidateReportData(startDate, endDate)
}

// GenerateJournalEntryAnalysis creates comprehensive journal entry analysis
func (ers *EnhancedReportService) GenerateJournalEntryAnalysis(startDate, endDate time.Time) (*JournalEntryAnalysisData, error) {
	return ers.GenerateJournalEntryAnalysisWithFilters(startDate, endDate, "ALL", "ALL")
}

// GenerateJournalEntryAnalysisWithFilters creates comprehensive journal entry analysis with optional filters
func (ers *EnhancedReportService) GenerateJournalEntryAnalysisWithFilters(startDate, endDate time.Time, status, referenceType string) (*JournalEntryAnalysisData, error) {
	// Build query with optional filters
	query := ers.db.Preload("Creator").
		Preload("JournalLines").
		Where("entry_date BETWEEN ? AND ?", startDate, endDate)

	// Add status filter if specified
	if status != "ALL" && status != "" {
		query = query.Where("status = ?", status)
	}

	// Add reference type filter if specified
	if referenceType != "ALL" && referenceType != "" {
		query = query.Where("reference_type = ?", referenceType)
	}

	// Execute query
	var journalEntries []models.JournalEntry
	if err := query.Find(&journalEntries).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch journal entries: %v", err)
	}

	// Initialize analysis
	analysis := &JournalEntryAnalysisData{
		Company:     ers.getCompanyInfo(),
		StartDate:   startDate,
		EndDate:     endDate,
		Currency:    ers.companyProfile.Currency,
		GeneratedAt: time.Now(),
	}

	// Process entries
	typeMap := make(map[string]*JournalTypeData)
	statusMap := make(map[string]*JournalStatusData)
	userMap := make(map[uint]*JournalUserData)
	var recentEntries, largestEntries, unbalancedEntries []JournalEntryDetail
	balancedCount := int64(0)
	unbalancedCount := int64(0)
	missingReferences := int64(0)
	futureDated := int64(0)

	for _, entry := range journalEntries {
		analysis.TotalEntries++
		analysis.TotalDebitAmount += entry.TotalDebit
		analysis.TotalCreditAmount += entry.TotalCredit

		// Check balance
		if entry.IsBalanced {
			balancedCount++
		} else {
			unbalancedCount++
			unbalancedEntries = append(unbalancedEntries, JournalEntryDetail{
				ID:           entry.ID,
				Code:         entry.Code,
				Date:         entry.EntryDate,
				Description:  entry.Description,
				Reference:    entry.Reference,
				DebitAmount:  entry.TotalDebit,
				CreditAmount: entry.TotalCredit,
				Status:       entry.Status,
				User:         entry.Creator.FirstName + " " + entry.Creator.LastName,
			})
		}

		// Check compliance issues
		if entry.Reference == "" {
			missingReferences++
		}
		if entry.EntryDate.After(time.Now()) {
			futureDated++
		}

		// Process by type
		if typeData, exists := typeMap[entry.ReferenceType]; exists {
			typeData.Count++
			typeData.TotalAmount += entry.TotalDebit
		} else {
			typeMap[entry.ReferenceType] = &JournalTypeData{
				ReferenceType: entry.ReferenceType,
				Count:         1,
				TotalAmount:   entry.TotalDebit,
			}
		}

		// Process by status
		if statusData, exists := statusMap[entry.Status]; exists {
			statusData.Count++
			statusData.TotalAmount += entry.TotalDebit
		} else {
			statusMap[entry.Status] = &JournalStatusData{
				Status:      entry.Status,
				Count:       1,
				TotalAmount: entry.TotalDebit,
			}
		}

		// Process by user
		if userData, exists := userMap[entry.UserID]; exists {
			userData.Count++
			userData.TotalAmount += entry.TotalDebit
		} else {
			userMap[entry.UserID] = &JournalUserData{
				UserID:      entry.UserID,
				UserName:    entry.Creator.FirstName + " " + entry.Creator.LastName,
				Count:       1,
				TotalAmount: entry.TotalDebit,
			}
		}

		// Collect recent entries (last 10)
		if len(recentEntries) < 10 {
			recentEntries = append(recentEntries, JournalEntryDetail{
				ID:           entry.ID,
				Code:         entry.Code,
				Date:         entry.EntryDate,
				Description:  entry.Description,
				Reference:    entry.Reference,
				DebitAmount:  entry.TotalDebit,
				CreditAmount: entry.TotalCredit,
				Status:       entry.Status,
				User:         entry.Creator.FirstName + " " + entry.Creator.LastName,
			})
		}

		// Collect largest entries
		if entry.TotalDebit >= 1000000 { // Entries over 1 million
			largestEntries = append(largestEntries, JournalEntryDetail{
				ID:           entry.ID,
				Code:         entry.Code,
				Date:         entry.EntryDate,
				Description:  entry.Description,
				Reference:    entry.Reference,
				DebitAmount:  entry.TotalDebit,
				CreditAmount: entry.TotalCredit,
				Status:       entry.Status,
				User:         entry.Creator.FirstName + " " + entry.Creator.LastName,
			})
		}
	}

	// Calculate percentages
	for _, typeData := range typeMap {
		if analysis.TotalDebitAmount > 0 {
			typeData.Percentage = (typeData.TotalAmount / analysis.TotalDebitAmount) * 100
		}
	}
	for _, statusData := range statusMap {
		if analysis.TotalDebitAmount > 0 {
			statusData.Percentage = (statusData.TotalAmount / analysis.TotalDebitAmount) * 100
		}
	}
	for _, userData := range userMap {
		if analysis.TotalDebitAmount > 0 {
			userData.Percentage = (userData.TotalAmount / analysis.TotalDebitAmount) * 100
		}
	}

	// Convert maps to slices
	analysis.EntriesByType = ers.convertJournalTypeMapToSlice(typeMap)
	analysis.EntriesByStatus = ers.convertJournalStatusMapToSlice(statusMap)
	analysis.EntriesByUser = ers.convertJournalUserMapToSlice(userMap)
	analysis.RecentEntries = recentEntries
	analysis.LargestEntries = largestEntries
	analysis.UnbalancedEntries = unbalancedEntries

	// Build compliance check
	complianceRate := float64(0)
	if analysis.TotalEntries > 0 {
		complianceRate = (float64(balancedCount) / float64(analysis.TotalEntries)) * 100
	}

	var complianceIssues []string
	if unbalancedCount > 0 {
		complianceIssues = append(complianceIssues, fmt.Sprintf("%d unbalanced entries found", unbalancedCount))
	}
	if missingReferences > 0 {
		complianceIssues = append(complianceIssues, fmt.Sprintf("%d entries missing references", missingReferences))
	}
	if futureDated > 0 {
		complianceIssues = append(complianceIssues, fmt.Sprintf("%d future-dated entries", futureDated))
	}

	analysis.ComplianceCheck = JournalComplianceData{
		BalancedEntries:    balancedCount,
		UnbalancedEntries:  unbalancedCount,
		ComplianceRate:     complianceRate,
		MissingReferences:  missingReferences,
		FutureDatedEntries: futureDated,
		ComplianceIssues:   complianceIssues,
	}

	return analysis, nil
}

// GeneratePurchaseSummary creates comprehensive purchase analytics
func (ers *EnhancedReportService) GeneratePurchaseSummary(startDate, endDate time.Time, groupBy string) (*PurchaseSummaryData, error) {
	// Query purchase data
	var purchases []models.Purchase
	if err := ers.db.Preload("Vendor").
		Preload("PurchaseItems").
		Preload("PurchaseItems.Product").
		Where("date BETWEEN ? AND ?", startDate, endDate).
		Find(&purchases).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch purchase data: %v", err)
	}

	// Initialize purchase summary
	summary := &PurchaseSummaryData{
		Company:           ers.getCompanyInfo(),
		StartDate:         startDate,
		EndDate:           endDate,
		Currency:          ers.companyProfile.Currency,
		TotalTransactions: int64(len(purchases)),
		GeneratedAt:       time.Now(),
	}

	// Calculate metrics similar to sales summary
	vendorSet := make(map[uint]bool)
	vendorMap := make(map[uint]*VendorPurchaseData)
	periodMap := make(map[string]*PeriodData)
	statusMap := make(map[string]*StatusData)
	categoryMap := make(map[uint]*CategoryPurchaseData)

	for _, purchase := range purchases {
		summary.TotalPurchases += purchase.TotalAmount
		vendorSet[purchase.VendorID] = true

		// Process vendor data
		if vendorData, exists := vendorMap[purchase.VendorID]; exists {
			vendorData.TotalAmount += purchase.TotalAmount
			vendorData.TransactionCount++
			if purchase.Date.After(vendorData.LastOrderDate) {
				vendorData.LastOrderDate = purchase.Date
			}
			if purchase.Date.Before(vendorData.FirstOrderDate) {
				vendorData.FirstOrderDate = purchase.Date
			}
		} else {
			vendorMap[purchase.VendorID] = &VendorPurchaseData{
				VendorID:         purchase.VendorID,
				VendorName:       purchase.Vendor.Name,
				TotalAmount:      purchase.TotalAmount,
				TransactionCount: 1,
				LastOrderDate:    purchase.Date,
				FirstOrderDate:   purchase.Date,
			}
		}

		// Process period data
		period := ers.formatPeriod(purchase.Date, groupBy)
		if periodData, exists := periodMap[period]; exists {
			periodData.Amount += purchase.TotalAmount
			periodData.Transactions++
		} else {
			periodMap[period] = &PeriodData{
				Period:       period,
				Amount:       purchase.TotalAmount,
				Transactions: 1,
				StartDate:    ers.getPeriodStart(purchase.Date, groupBy),
				EndDate:      ers.getPeriodEnd(purchase.Date, groupBy),
			}
		}

		// Process status data
		if statusData, exists := statusMap[purchase.Status]; exists {
			statusData.Count++
			statusData.Amount += purchase.TotalAmount
		} else {
			statusMap[purchase.Status] = &StatusData{
				Status: purchase.Status,
				Count:  1,
				Amount: purchase.TotalAmount,
			}
		}
	}

	// Calculate derived metrics
	summary.TotalVendors = int64(len(vendorSet))
	if summary.TotalTransactions > 0 {
		summary.AveragePurchaseValue = summary.TotalPurchases / float64(summary.TotalTransactions)
	}

	// Calculate average order values
	for _, vendorData := range vendorMap {
		if vendorData.TransactionCount > 0 {
			vendorData.AverageOrder = vendorData.TotalAmount / float64(vendorData.TransactionCount)
		}
	}

	// Convert maps to slices and sort
	summary.PurchasesByVendor = ers.sortVendorPurchaseData(vendorMap)
	summary.PurchasesByPeriod = ers.sortPeriodsByDate(periodMap)
	summary.PurchasesByStatus = ers.convertStatusMapToSlice(statusMap)

	// Build top vendors (simplified)
	summary.TopVendors = TopVendorsData{
		TopVendors:    ers.getTopVendorPurchases(summary.PurchasesByVendor),
		TopCategories: ers.getCategoryPurchaseData(categoryMap),
	}

	// Calculate cost analysis (simplified)
	summary.CostAnalysis = ers.calculateSimpleCostAnalysis(purchases)

	return summary, nil
}
