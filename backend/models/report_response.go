package models

import (
	"time"
)

// StandardReportResponse provides consistent response structure for all reports
type StandardReportResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data"`
	Metadata  Metadata    `json:"metadata"`
	Error     *ErrorInfo  `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// Metadata contains report generation information
type Metadata struct {
	ReportType      string                 `json:"report_type"`
	GeneratedAt     time.Time              `json:"generated_at"`
	GeneratedBy     string                 `json:"generated_by"`
	Parameters      map[string]interface{} `json:"parameters"`
	GenerationTime  string                 `json:"generation_time"`
	RecordCount     int                    `json:"record_count"`
	Version         string                 `json:"version"`
	Format          string                 `json:"format"`
	HasMoreData     bool                   `json:"has_more_data"`
	NextPageToken   string                 `json:"next_page_token,omitempty"`
}

// ErrorInfo provides detailed error information
type ErrorInfo struct {
	Code        string                 `json:"code"`
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Suggestions []string               `json:"suggestions,omitempty"`
}

// ReportSection represents a section in financial reports
type ReportSection struct {
	Name        string        `json:"name"`
	Items       []ReportItem  `json:"items"`
	Subtotal    float64       `json:"subtotal"`
	Order       int           `json:"order"`
	Type        string        `json:"type"`
	Subsections []ReportSection `json:"subsections,omitempty"`
}

// ReportItem represents individual items in report sections
type ReportItem struct {
	AccountID       uint    `json:"account_id,omitempty"`
	AccountCode     string  `json:"account_code,omitempty"`
	Name            string  `json:"name"`
	Amount          float64 `json:"amount"`
	Balance         float64 `json:"balance,omitempty"`
	DebitAmount     float64 `json:"debit_amount,omitempty"`
	CreditAmount    float64 `json:"credit_amount,omitempty"`
	Percentage      float64 `json:"percentage,omitempty"`
	PreviousAmount  float64 `json:"previous_amount,omitempty"`
	Variance        float64 `json:"variance,omitempty"`
	VariancePercent float64 `json:"variance_percent,omitempty"`
	Level           int     `json:"level,omitempty"`
	IsHeader        bool    `json:"is_header,omitempty"`
	IsTotal         bool    `json:"is_total,omitempty"`
	Date            *time.Time `json:"date,omitempty"`
	Description     string  `json:"description,omitempty"`
	Reference       string  `json:"reference,omitempty"`
}

// CompanyInfo represents company information in reports
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
	Logo       string `json:"logo,omitempty"`
}

// FinancialReportData represents the standard structure for financial reports
type FinancialReportData struct {
	Company     CompanyInfo     `json:"company"`
	ReportTitle string          `json:"report_title"`
	Period      string          `json:"period"`
	AsOfDate    *time.Time      `json:"as_of_date,omitempty"`
	StartDate   *time.Time      `json:"start_date,omitempty"`
	EndDate     *time.Time      `json:"end_date,omitempty"`
	Currency    string          `json:"currency"`
	Sections    []ReportSection `json:"sections"`
	Totals      map[string]float64 `json:"totals"`
	Ratios      map[string]float64 `json:"ratios,omitempty"`
	Summary     map[string]interface{} `json:"summary,omitempty"`
	Notes       []string        `json:"notes,omitempty"`
}

// BalanceSheetData represents balance sheet specific structure
type BalanceSheetData struct {
	FinancialReportData
	Assets              ReportSection `json:"assets"`
	Liabilities         ReportSection `json:"liabilities"`  
	Equity              ReportSection `json:"equity"`
	TotalAssets         float64       `json:"total_assets"`
	TotalLiabilities    float64       `json:"total_liabilities"`
	TotalEquity         float64       `json:"total_equity"`
	IsBalanced          bool          `json:"is_balanced"`
	BalanceDifference   float64       `json:"balance_difference,omitempty"`
}

// ProfitLossData represents profit & loss specific structure
type ProfitLossData struct {
	FinancialReportData
	Revenue            ReportSection `json:"revenue"`
	CostOfGoodsSold    ReportSection `json:"cost_of_goods_sold"`
	GrossProfit        float64       `json:"gross_profit"`
	GrossProfitMargin  float64       `json:"gross_profit_margin"`
	OperatingExpenses  ReportSection `json:"operating_expenses"`
	OperatingIncome    float64       `json:"operating_income"`
	OperatingMargin    float64       `json:"operating_margin"`
	OtherIncome        ReportSection `json:"other_income"`
	OtherExpenses      ReportSection `json:"other_expenses"`
	NetOtherIncome     float64       `json:"net_other_income"`
	IncomeBeforeTax    float64       `json:"income_before_tax"`
	TaxExpense         float64       `json:"tax_expense"`
	NetIncome          float64       `json:"net_income"`
	NetIncomeMargin    float64       `json:"net_income_margin"`
	EBITDA             float64       `json:"ebitda,omitempty"`
	EBITDAMargin       float64       `json:"ebitda_margin,omitempty"`
}

// CashFlowData represents cash flow specific structure
// NOTE: Use the section subtotals as the single source of truth for activity totals.
type CashFlowData struct {
	FinancialReportData
	OperatingActivities  ReportSection `json:"operating_activities"`
	InvestingActivities  ReportSection `json:"investing_activities"`
	FinancingActivities  ReportSection `json:"financing_activities"`
	NetCashFlow          float64       `json:"net_cash_flow"`
	BeginningCashBalance float64       `json:"beginning_cash_balance"`
	EndingCashBalance    float64       `json:"ending_cash_balance"`
}

// TrialBalanceData represents trial balance specific structure
type TrialBalanceData struct {
	FinancialReportData
	Accounts         []TrialBalanceAccount `json:"accounts"`
	TotalDebits      float64               `json:"total_debits"`
	TotalCredits     float64               `json:"total_credits"`
	IsBalanced       bool                  `json:"is_balanced"`
	BalanceDifference float64              `json:"balance_difference,omitempty"`
}

// TrialBalanceAccount represents an account in trial balance
type TrialBalanceAccount struct {
	AccountID     uint    `json:"account_id"`
	AccountCode   string  `json:"account_code"`
	AccountName   string  `json:"account_name"`
	AccountType   string  `json:"account_type"`
	DebitBalance  float64 `json:"debit_balance"`
	CreditBalance float64 `json:"credit_balance"`
	Level         int     `json:"level"`
	IsHeader      bool    `json:"is_header"`
}

// GeneralLedgerData represents general ledger specific structure
type GeneralLedgerData struct {
	FinancialReportData
	Accounts []GeneralLedgerAccount `json:"accounts"`
}

// GeneralLedgerAccount represents an account with its transactions
type GeneralLedgerAccount struct {
	AccountID       uint                         `json:"account_id"`
	AccountCode     string                       `json:"account_code"`
	AccountName     string                       `json:"account_name"`
	AccountType     string                       `json:"account_type"`
	OpeningBalance  float64                      `json:"opening_balance"`
	ClosingBalance  float64                      `json:"closing_balance"`
	TotalDebits     float64                      `json:"total_debits"`
	TotalCredits    float64                      `json:"total_credits"`
	Transactions    []GeneralLedgerTransaction   `json:"transactions"`
}

// GeneralLedgerTransaction represents a transaction in general ledger
type GeneralLedgerTransaction struct {
	Date         time.Time `json:"date"`
	Reference    string    `json:"reference"`
	Description  string    `json:"description"`
	DebitAmount  float64   `json:"debit_amount"`
	CreditAmount float64   `json:"credit_amount"`
	Balance      float64   `json:"balance"`
	JournalID    uint      `json:"journal_id,omitempty"`
}

// SalesSummaryData represents sales summary specific structure
type SalesSummaryData struct {
	FinancialReportData
	SalesByPeriod    []PeriodData    `json:"sales_by_period"`
	SalesByCustomer  []CustomerData  `json:"sales_by_customer"`
	SalesByProduct   []ProductData   `json:"sales_by_product"`
	TopCustomers     []CustomerData  `json:"top_customers"`
	TopProducts      []ProductData   `json:"top_products"`
	TotalRevenue     float64         `json:"total_revenue"`
	TotalTransactions int            `json:"total_transactions"`
	AverageOrderValue float64        `json:"average_order_value"`
	GrowthRate       float64         `json:"growth_rate,omitempty"`
}

// VendorAnalysisData represents vendor analysis specific structure
type VendorAnalysisData struct {
	FinancialReportData
	PurchasesByPeriod  []PeriodData  `json:"purchases_by_period"`
	PurchasesByVendor  []VendorData  `json:"purchases_by_vendor"`
	TopVendors         []VendorData  `json:"top_vendors"`
	TotalPurchases     float64       `json:"total_purchases"`
	TotalTransactions  int           `json:"total_transactions"`
	AverageOrderValue  float64       `json:"average_order_value"`
	GrowthRate         float64       `json:"growth_rate,omitempty"`
}

// Supporting data structures
type PeriodData struct {
	Period     string  `json:"period"`
	Amount     float64 `json:"amount"`
	Count      int     `json:"count"`
	GrowthRate float64 `json:"growth_rate,omitempty"`
}

type CustomerData struct {
	CustomerID   uint    `json:"customer_id"`
	CustomerName string  `json:"customer_name"`
	Amount       float64 `json:"amount"`
	Count        int     `json:"count"`
	Percentage   float64 `json:"percentage"`
}

type ProductData struct {
	ProductID    uint    `json:"product_id"`
	ProductName  string  `json:"product_name"`
	Amount       float64 `json:"amount"`
	Quantity     int     `json:"quantity"`
	Percentage   float64 `json:"percentage"`
}

type VendorData struct {
	VendorID     uint    `json:"vendor_id"`
	VendorName   string  `json:"vendor_name"`
	Amount       float64 `json:"amount"`
	Count        int     `json:"count"`
	Percentage   float64 `json:"percentage"`
}

// ReportPreviewData represents a simplified preview structure
type ReportPreviewData struct {
	Title    string          `json:"title"`
	Period   string          `json:"period"`
	Sections []ReportSection `json:"sections"`
	Summary  map[string]float64 `json:"summary,omitempty"`
}
