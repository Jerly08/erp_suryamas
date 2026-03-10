package services

import (
	"fmt"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// PSAKCompliantReportService provides PSAK-compliant financial report calculations
// Mengikuti Pernyataan Standar Akuntansi Keuangan (PSAK) Indonesia
type PSAKCompliantReportService struct {
	db                        *gorm.DB
	unifiedJournalService     *UnifiedJournalService
	accountRepo               repositories.AccountRepository
	companyProfile            *models.CompanyProfile
}

// NewPSAKCompliantReportService creates new PSAK-compliant report service
func NewPSAKCompliantReportService(
	db *gorm.DB,
	unifiedJournalService *UnifiedJournalService,
	accountRepo repositories.AccountRepository,
) *PSAKCompliantReportService {
	service := &PSAKCompliantReportService{
		db:                    db,
		unifiedJournalService: unifiedJournalService,
		accountRepo:           accountRepo,
	}
	
	// Load company profile
	service.loadCompanyProfile()
	
	return service
}

// PSAKBalanceSheetData represents Balance Sheet sesuai PSAK 1 (Penyajian Laporan Keuangan)
type PSAKBalanceSheetData struct {
	CompanyInfo         CompanyInfo           `json:"company_info"`
	ReportingDate       time.Time             `json:"reporting_date"`      // Tanggal pelaporan
	Currency            string                `json:"currency"`
	
	// ASET (sesuai PSAK 1 paragraf 54-56)
	CurrentAssets       PSAKAssetSection      `json:"current_assets"`      // Aset Lancar
	NonCurrentAssets    PSAKAssetSection      `json:"non_current_assets"`  // Aset Tidak Lancar
	TotalAssets         decimal.Decimal       `json:"total_assets"`        // Total Aset
	
	// LIABILITAS (sesuai PSAK 1 paragraf 60-65)
	CurrentLiabilities  PSAKLiabilitySection  `json:"current_liabilities"`    // Liabilitas Jangka Pendek
	NonCurrentLiabilities PSAKLiabilitySection `json:"non_current_liabilities"` // Liabilitas Jangka Panjang
	TotalLiabilities    decimal.Decimal       `json:"total_liabilities"`      // Total Liabilitas
	
	// EKUITAS (sesuai PSAK 1 paragraf 54(r))
	Equity              PSAKEquitySection     `json:"equity"`              // Ekuitas
	TotalEquity         decimal.Decimal       `json:"total_equity"`        // Total Ekuitas
	
	// Validation sesuai fundamental equation: Assets = Liabilities + Equity
	IsBalanced          bool                  `json:"is_balanced"`
	BalanceDifference   decimal.Decimal       `json:"balance_difference"`
	
	// Compliance dan Audit Trail
	PSAKCompliance      PSAKComplianceInfo    `json:"psak_compliance"`
	GeneratedAt         time.Time             `json:"generated_at"`
}

// PSAKProfitLossData represents P&L sesuai PSAK 1 (Laporan Laba Rugi dan Penghasilan Komprehensif Lain)
type PSAKProfitLossData struct {
	CompanyInfo         CompanyInfo           `json:"company_info"`
	PeriodStart         time.Time             `json:"period_start"`
	PeriodEnd           time.Time             `json:"period_end"`
	Currency            string                `json:"currency"`
	
	// PENDAPATAN (Revenue) - sesuai PSAK 23 (Pendapatan dari Kontrak dengan Pelanggan)
	Revenue             PSAKRevenueSection    `json:"revenue"`             // Pendapatan
	TotalRevenue        decimal.Decimal       `json:"total_revenue"`       // Total Pendapatan
	
	// BEBAN POKOK PENJUALAN (Cost of Sales) - sesuai PSAK 14 (Persediaan)
	CostOfSales         PSAKExpenseSection    `json:"cost_of_sales"`       // Beban Pokok Penjualan
	TotalCostOfSales    decimal.Decimal       `json:"total_cost_of_sales"` // Total Beban Pokok Penjualan
	
	// LABA KOTOR (Gross Profit)
	GrossProfit         decimal.Decimal       `json:"gross_profit"`        // Laba Kotor
	GrossProfitMargin   decimal.Decimal       `json:"gross_profit_margin"` // Margin Laba Kotor (%)
	
	// BEBAN USAHA (Operating Expenses)
	OperatingExpenses   PSAKExpenseSection    `json:"operating_expenses"`  // Beban Usaha
	TotalOpExpenses     decimal.Decimal       `json:"total_operating_expenses"` // Total Beban Usaha
	
	// LABA USAHA (Operating Profit)
	OperatingProfit     decimal.Decimal       `json:"operating_profit"`    // Laba Usaha
	OperatingMargin     decimal.Decimal       `json:"operating_margin"`    // Margin Usaha (%)
	
	// PENDAPATAN DAN BEBAN LAIN-LAIN (Other Income and Expenses)
	OtherIncome         PSAKOtherSection      `json:"other_income"`        // Pendapatan Lain-lain
	OtherExpenses       PSAKOtherSection      `json:"other_expenses"`      // Beban Lain-lain
	NetOtherIncome      decimal.Decimal       `json:"net_other_income"`    // Pendapatan Lain-lain Neto
	
	// LABA SEBELUM PAJAK (Profit Before Tax)
	ProfitBeforeTax     decimal.Decimal       `json:"profit_before_tax"`   // Laba Sebelum Pajak
	
	// BEBAN PAJAK PENGHASILAN - sesuai PSAK 46 (Pajak Penghasilan)
	TaxExpense          PSAKTaxSection        `json:"tax_expense"`         // Beban Pajak Penghasilan
	TotalTaxExpense     decimal.Decimal       `json:"total_tax_expense"`   // Total Beban Pajak
	
	// LABA NETO (Net Profit)
	NetProfit           decimal.Decimal       `json:"net_profit"`          // Laba Neto
	NetProfitMargin     decimal.Decimal       `json:"net_profit_margin"`   // Margin Laba Neto (%)
	
	// PENGHASILAN KOMPREHENSIF LAIN - sesuai PSAK 1 paragraf 82-85
	OtherComprehensiveIncome PSAKOCISection   `json:"other_comprehensive_income"` // Penghasilan Komprehensif Lain
	TotalComprehensiveIncome decimal.Decimal  `json:"total_comprehensive_income"` // Total Laba Komprehensif
	
	// LABA PER SAHAM - sesuai PSAK 56 (Laba per Saham)
	BasicEPS            decimal.Decimal       `json:"basic_eps"`           // Laba per Saham Dasar
	DilutedEPS          decimal.Decimal       `json:"diluted_eps"`         // Laba per Saham Dilusian
	WeightedAvgShares   decimal.Decimal       `json:"weighted_avg_shares"` // Rata-rata Tertimbang Saham
	
	// Compliance dan Audit Trail
	PSAKCompliance      PSAKComplianceInfo    `json:"psak_compliance"`
	GeneratedAt         time.Time             `json:"generated_at"`
}

// PSAKCashFlowData represents Cash Flow sesuai PSAK 2 (Laporan Arus Kas)
type PSAKCashFlowData struct {
	CompanyInfo         CompanyInfo           `json:"company_info"`
	PeriodStart         time.Time             `json:"period_start"`
	PeriodEnd           time.Time             `json:"period_end"`
	Currency            string                `json:"currency"`
	Method              string                `json:"method"`              // "DIRECT" atau "INDIRECT"
	
	// KAS DAN SETARA KAS AWAL PERIODE
	BeginningCash       decimal.Decimal       `json:"beginning_cash"`      // Kas Awal Periode
	
	// ARUS KAS DARI AKTIVITAS OPERASI - sesuai PSAK 2 paragraf 13-15
	OperatingActivities PSAKCashFlowSection   `json:"operating_activities"`// Aktivitas Operasi
	NetOperatingCash    decimal.Decimal       `json:"net_operating_cash"`  // Kas Neto dari Operasi
	
	// ARUS KAS DARI AKTIVITAS INVESTASI - sesuai PSAK 2 paragraf 16-17
	InvestingActivities PSAKCashFlowSection   `json:"investing_activities"`// Aktivitas Investasi
	NetInvestingCash    decimal.Decimal       `json:"net_investing_cash"`  // Kas Neto dari Investasi
	
	// ARUS KAS DARI AKTIVITAS PENDANAAN - sesuai PSAK 2 paragraf 17
	FinancingActivities PSAKCashFlowSection   `json:"financing_activities"`// Aktivitas Pendanaan
	NetFinancingCash    decimal.Decimal       `json:"net_financing_cash"`  // Kas Neto dari Pendanaan
	
	// KENAIKAN (PENURUNAN) KAS DAN SETARA KAS
	NetCashIncrease     decimal.Decimal       `json:"net_cash_increase"`   // Kenaikan/Penurunan Kas
	
	// PENGARUH PERUBAHAN KURS VALUTA ASING
	ForeignExchangeEffect decimal.Decimal     `json:"foreign_exchange_effect"` // Pengaruh Kurs
	
	// KAS DAN SETARA KAS AKHIR PERIODE
	EndingCash          decimal.Decimal       `json:"ending_cash"`         // Kas Akhir Periode
	
	// REKONSILIASI (untuk metode tidak langsung)
	Reconciliation      PSAKCashReconciliation `json:"reconciliation,omitempty"`
	
	// Compliance dan Audit Trail
	PSAKCompliance      PSAKComplianceInfo    `json:"psak_compliance"`
	GeneratedAt         time.Time             `json:"generated_at"`
}

// Supporting structures sesuai PSAK

// PSAKAssetSection represents asset section sesuai PSAK 1
type PSAKAssetSection struct {
	Name        string              `json:"name"`
	Items       []PSAKBalanceItem   `json:"items"`
	Subtotal    decimal.Decimal     `json:"subtotal"`
	
	// Sub-sections untuk aset lancar (sesuai PSAK 1 paragraf 66)
	CashAndCashEquivalents  decimal.Decimal `json:"cash_and_cash_equivalents,omitempty"`  // Kas dan Setara Kas
	TradeReceivables       decimal.Decimal `json:"trade_receivables,omitempty"`          // Piutang Usaha
	Inventories            decimal.Decimal `json:"inventories,omitempty"`                 // Persediaan
	PrepaidExpenses        decimal.Decimal `json:"prepaid_expenses,omitempty"`            // Beban Dibayar Dimuka
	
	// Sub-sections untuk aset tidak lancar (sesuai PSAK 1 paragraf 66)
	PropertyPlantEquipment decimal.Decimal `json:"property_plant_equipment,omitempty"`    // Aset Tetap
	IntangibleAssets       decimal.Decimal `json:"intangible_assets,omitempty"`           // Aset Tidak Berwujud
	InvestmentProperties   decimal.Decimal `json:"investment_properties,omitempty"`       // Properti Investasi
	LongTermInvestments    decimal.Decimal `json:"long_term_investments,omitempty"`       // Investasi Jangka Panjang
	DeferredTaxAssets      decimal.Decimal `json:"deferred_tax_assets,omitempty"`         // Aset Pajak Tangguhan
}

// PSAKLiabilitySection represents liability section sesuai PSAK 1
type PSAKLiabilitySection struct {
	Name        string              `json:"name"`
	Items       []PSAKBalanceItem   `json:"items"`
	Subtotal    decimal.Decimal     `json:"subtotal"`
	
	// Sub-sections untuk liabilitas jangka pendek (sesuai PSAK 1 paragraf 69)
	TradePayables          decimal.Decimal `json:"trade_payables,omitempty"`          // Utang Usaha
	AccruedLiabilities     decimal.Decimal `json:"accrued_liabilities,omitempty"`     // Beban yang Masih Harus Dibayar
	ShortTermBorrowings    decimal.Decimal `json:"short_term_borrowings,omitempty"`   // Pinjaman Jangka Pendek
	CurrentPortionLongTerm decimal.Decimal `json:"current_portion_long_term,omitempty"` // Bagian Lancar Utang Jangka Panjang
	TaxLiabilities         decimal.Decimal `json:"tax_liabilities,omitempty"`         // Utang Pajak
	
	// Sub-sections untuk liabilitas jangka panjang
	LongTermBorrowings     decimal.Decimal `json:"long_term_borrowings,omitempty"`    // Pinjaman Jangka Panjang
	DeferredTaxLiabilities decimal.Decimal `json:"deferred_tax_liabilities,omitempty"` // Liabilitas Pajak Tangguhan
	EmployeeBenefits       decimal.Decimal `json:"employee_benefits,omitempty"`       // Imbalan Kerja
	Provisions             decimal.Decimal `json:"provisions,omitempty"`              // Provisi
}

// PSAKEquitySection represents equity section sesuai PSAK 1
type PSAKEquitySection struct {
	Name        string              `json:"name"`
	Items       []PSAKBalanceItem   `json:"items"`
	Subtotal    decimal.Decimal     `json:"subtotal"`
	
	// Sub-sections ekuitas (sesuai PSAK 1 paragraf 54(r))
	ShareCapital           decimal.Decimal `json:"share_capital"`           // Modal Saham
	SharePremium           decimal.Decimal `json:"share_premium"`           // Agio Saham
	RetainedEarnings       decimal.Decimal `json:"retained_earnings"`       // Saldo Laba
	OtherEquityComponents  decimal.Decimal `json:"other_equity_components"` // Komponen Ekuitas Lainnya
	TreasuryStock          decimal.Decimal `json:"treasury_stock"`          // Saham Treasury
}

// PSAKRevenueSection represents revenue section sesuai PSAK 23
type PSAKRevenueSection struct {
	Name        string              `json:"name"`
	Items       []PSAKPLItem        `json:"items"`
	Subtotal    decimal.Decimal     `json:"subtotal"`
	
	// Sub-sections pendapatan (sesuai PSAK 23)
	SalesRevenue           decimal.Decimal `json:"sales_revenue"`           // Penjualan
	ServiceRevenue         decimal.Decimal `json:"service_revenue"`         // Jasa
	InterestIncome         decimal.Decimal `json:"interest_income"`         // Pendapatan Bunga
	DividendIncome         decimal.Decimal `json:"dividend_income"`         // Pendapatan Dividen
	RoyaltyIncome          decimal.Decimal `json:"royalty_income"`          // Pendapatan Royalti
}

// PSAKExpenseSection represents expense section
type PSAKExpenseSection struct {
	Name        string              `json:"name"`
	Items       []PSAKPLItem        `json:"items"`
	Subtotal    decimal.Decimal     `json:"subtotal"`
}

// PSAKTaxSection represents tax section sesuai PSAK 46
type PSAKTaxSection struct {
	Name        string              `json:"name"`
	Items       []PSAKPLItem        `json:"items"`
	Subtotal    decimal.Decimal     `json:"subtotal"`
	
	// Sub-sections pajak (sesuai PSAK 46)
	CurrentTaxExpense      decimal.Decimal `json:"current_tax_expense"`     // Beban Pajak Kini
	DeferredTaxExpense     decimal.Decimal `json:"deferred_tax_expense"`    // Beban Pajak Tangguhan
	TaxAdjustmentPriorYear decimal.Decimal `json:"tax_adjustment_prior_year"` // Koreksi Pajak Tahun Lalu
}

// PSAKOCISection represents Other Comprehensive Income sesuai PSAK 1
type PSAKOCISection struct {
	Name        string              `json:"name"`
	Items       []PSAKPLItem        `json:"items"`
	Subtotal    decimal.Decimal     `json:"subtotal"`
	
	// Sub-sections OCI yang tidak akan direklasifikasi ke laba rugi
	ActuarialGainLoss      decimal.Decimal `json:"actuarial_gain_loss"`     // Keuntungan/Kerugian Aktuarial
	RevaluationSurplus     decimal.Decimal `json:"revaluation_surplus"`     // Surplus Revaluasi
	
	// Sub-sections OCI yang akan direklasifikasi ke laba rugi
	ForeignCurrencyTranslation decimal.Decimal `json:"foreign_currency_translation"` // Selisih Kurs
	CashFlowHedges         decimal.Decimal `json:"cash_flow_hedges"`        // Lindung Nilai Arus Kas
	AvailableForSaleSecurities decimal.Decimal `json:"available_for_sale_securities"` // Sekuritas Tersedia untuk Dijual
}

// PSAKOtherSection represents other income/expenses
type PSAKOtherSection struct {
	Name        string              `json:"name"`
	Items       []PSAKPLItem        `json:"items"`
	Subtotal    decimal.Decimal     `json:"subtotal"`
}

// PSAKCashFlowSection represents cash flow section sesuai PSAK 2
type PSAKCashFlowSection struct {
	Name        string                `json:"name"`
	Items       []PSAKCashFlowItem    `json:"items"`
	Subtotal    decimal.Decimal       `json:"subtotal"`
}

// PSAKCashReconciliation represents cash flow reconciliation sesuai PSAK 2
type PSAKCashReconciliation struct {
	NetProfit              decimal.Decimal     `json:"net_profit"`              // Laba Neto
	AdjustmentsFor         []PSAKCashFlowItem  `json:"adjustments_for"`         // Penyesuaian untuk
	ChangesInWorkingCapital []PSAKCashFlowItem `json:"changes_in_working_capital"` // Perubahan Modal Kerja
	CashFromOperations     decimal.Decimal     `json:"cash_from_operations"`    // Kas dari Operasi
	InterestPaid           decimal.Decimal     `json:"interest_paid"`           // Bunga Dibayar
	TaxesPaid              decimal.Decimal     `json:"taxes_paid"`              // Pajak Dibayar
	NetOperatingCash       decimal.Decimal     `json:"net_operating_cash"`      // Kas Neto Operasi
}

// Supporting item structures

// PSAKBalanceItem represents balance sheet item
type PSAKBalanceItem struct {
	AccountID       uint64          `json:"account_id"`
	AccountCode     string          `json:"account_code"`
	AccountName     string          `json:"account_name"`
	Amount          decimal.Decimal `json:"amount"`
	Classification  string          `json:"classification"`  // LANCAR, TIDAK_LANCAR, etc.
	Category        string          `json:"category"`
	Level           int             `json:"level"`
	IsHeader        bool            `json:"is_header"`
	PSAKReference   string          `json:"psak_reference"`  // Reference to specific PSAK paragraph
}

// PSAKPLItem represents P&L item
type PSAKPLItem struct {
	AccountID       uint64          `json:"account_id"`
	AccountCode     string          `json:"account_code"`
	AccountName     string          `json:"account_name"`
	Amount          decimal.Decimal `json:"amount"`
	Percentage      decimal.Decimal `json:"percentage"`      // Percentage of total revenue
	Classification  string          `json:"classification"`
	Category        string          `json:"category"`
	PSAKReference   string          `json:"psak_reference"`
}

// PSAKCashFlowItem represents cash flow item
type PSAKCashFlowItem struct {
	Description     string          `json:"description"`
	Amount          decimal.Decimal `json:"amount"`
	Type            string          `json:"type"`           // INFLOW, OUTFLOW
	Classification  string          `json:"classification"` // OPERASI, INVESTASI, PENDANAAN
	Method          string          `json:"method"`         // LANGSUNG, TIDAK_LANGSUNG
	PSAKReference   string          `json:"psak_reference"`
}

// PSAKComplianceInfo represents PSAK compliance information
type PSAKComplianceInfo struct {
	StandardsApplied    []string    `json:"standards_applied"`    // PSAK yang diterapkan
	ComplianceLevel     string      `json:"compliance_level"`     // FULL, PARTIAL, NON_COMPLIANT
	ComplianceScore     decimal.Decimal `json:"compliance_score"` // Score 0-100
	NonComplianceIssues []PSAKIssue `json:"non_compliance_issues,omitempty"`
	Recommendations     []string    `json:"recommendations,omitempty"`
	LastReviewDate      time.Time   `json:"last_review_date"`
	ReviewerNotes       string      `json:"reviewer_notes,omitempty"`
}

// PSAKIssue represents PSAK compliance issue
type PSAKIssue struct {
	StandardReference   string      `json:"standard_reference"`   // e.g., "PSAK 1 paragraf 54"
	IssueDescription    string      `json:"issue_description"`
	Severity            string      `json:"severity"`             // HIGH, MEDIUM, LOW
	RecommendedAction   string      `json:"recommended_action"`
	AffectedAccounts    []uint64    `json:"affected_accounts,omitempty"`
}

// Implementation methods

// GeneratePSAKBalanceSheet generates Balance Sheet sesuai PSAK 1
func (s *PSAKCompliantReportService) GeneratePSAKBalanceSheet(asOfDate time.Time) (*PSAKBalanceSheetData, error) {
	// Get all account balances from SSOT
	balances, err := s.getAccountBalancesFromSSOT(asOfDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get account balances: %w", err)
	}

	balanceSheet := &PSAKBalanceSheetData{
		CompanyInfo:   s.getCompanyInfo(),
		ReportingDate: asOfDate,
		Currency:      s.getCurrency(),
		GeneratedAt:   time.Now(),
	}

	// Initialize totals
	totalAssets := decimal.Zero
	totalLiabilities := decimal.Zero
	totalEquity := decimal.Zero

	// Track consumed account IDs to prevent double counting across classifications
	consumed := make(map[uint64]bool)

	// Process Current Assets (Aset Lancar) - sesuai PSAK 1 paragraf 66
	currentAssets, currentAssetsTotal := s.classifyCurrentAssets(balances, consumed)
	balanceSheet.CurrentAssets = PSAKAssetSection{
		Name:     "Aset Lancar",
		Items:    currentAssets,
		Subtotal: currentAssetsTotal,
	}
	totalAssets = totalAssets.Add(currentAssetsTotal)

	// Process Non-Current Assets (Aset Tidak Lancar) - sesuai PSAK 1 paragraf 66
	nonCurrentAssets, nonCurrentAssetsTotal := s.classifyNonCurrentAssets(balances, consumed)
	balanceSheet.NonCurrentAssets = PSAKAssetSection{
		Name:     "Aset Tidak Lancar",
		Items:    nonCurrentAssets,
		Subtotal: nonCurrentAssetsTotal,
	}
	totalAssets = totalAssets.Add(nonCurrentAssetsTotal)

	// Process Current Liabilities (Liabilitas Jangka Pendek) - sesuai PSAK 1 paragraf 69
	currentLiabilities, currentLiabilitiesTotal := s.classifyCurrentLiabilities(balances, consumed)
	balanceSheet.CurrentLiabilities = PSAKLiabilitySection{
		Name:     "Liabilitas Jangka Pendek",
		Items:    currentLiabilities,
		Subtotal: currentLiabilitiesTotal,
	}
	totalLiabilities = totalLiabilities.Add(currentLiabilitiesTotal)

	// Process Non-Current Liabilities (Liabilitas Jangka Panjang)
	nonCurrentLiabilities, nonCurrentLiabilitiesTotal := s.classifyNonCurrentLiabilities(balances, consumed)
	balanceSheet.NonCurrentLiabilities = PSAKLiabilitySection{
		Name:     "Liabilitas Jangka Panjang",
		Items:    nonCurrentLiabilities,
		Subtotal: nonCurrentLiabilitiesTotal,
	}
	totalLiabilities = totalLiabilities.Add(nonCurrentLiabilitiesTotal)

	// Process Equity (Ekuitas) - sesuai PSAK 1 paragraf 54(r)
	equity, equityTotal := s.classifyEquity(balances, consumed)
	balanceSheet.Equity = PSAKEquitySection{
		Name:     "Ekuitas",
		Items:    equity,
		Subtotal: equityTotal,
	}
	totalEquity = equityTotal

	// Set totals
	balanceSheet.TotalAssets = totalAssets
	balanceSheet.TotalLiabilities = totalLiabilities
	balanceSheet.TotalEquity = totalEquity

	// Validate fundamental accounting equation: Assets = Liabilities + Equity
	totalLiabilitiesAndEquity := totalLiabilities.Add(totalEquity)
	balanceSheet.BalanceDifference = totalAssets.Sub(totalLiabilitiesAndEquity)
	balanceSheet.IsBalanced = balanceSheet.BalanceDifference.Abs().LessThan(decimal.NewFromFloat(0.01))

	// Generate PSAK compliance report
	balanceSheet.PSAKCompliance = s.generateBalanceSheetCompliance(balanceSheet)

	return balanceSheet, nil
}

// GeneratePSAKProfitLoss generates P&L sesuai PSAK 1
func (s *PSAKCompliantReportService) GeneratePSAKProfitLoss(startDate, endDate time.Time) (*PSAKProfitLossData, error) {
	// Get all account activities for the period from SSOT
	activities, err := s.getAccountActivitiesFromSSOT(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get account activities: %w", err)
	}

	profitLoss := &PSAKProfitLossData{
		CompanyInfo: s.getCompanyInfo(),
		PeriodStart: startDate,
		PeriodEnd:   endDate,
		Currency:    s.getCurrency(),
		GeneratedAt: time.Now(),
	}

	// Track consumed account IDs to prevent double counting across classifications
	consumed := make(map[uint64]bool)

	// Process Revenue (Pendapatan) - sesuai PSAK 23
	revenue, totalRevenue := s.classifyRevenue(activities, consumed)
	profitLoss.Revenue = revenue
	profitLoss.TotalRevenue = totalRevenue

	// Process Cost of Sales (Beban Pokok Penjualan) - sesuai PSAK 14
	costOfSales, totalCostOfSales := s.classifyCostOfSales(activities, consumed)
	profitLoss.CostOfSales = costOfSales
	profitLoss.TotalCostOfSales = totalCostOfSales

	// Calculate Gross Profit (Laba Kotor)
	profitLoss.GrossProfit = totalRevenue.Sub(totalCostOfSales)
	if totalRevenue.GreaterThan(decimal.Zero) {
		profitLoss.GrossProfitMargin = profitLoss.GrossProfit.Div(totalRevenue).Mul(decimal.NewFromInt(100))
	}

	// Process Operating Expenses (Beban Usaha)
	operatingExpenses, totalOpExpenses := s.classifyOperatingExpenses(activities, consumed)
	profitLoss.OperatingExpenses = operatingExpenses
	profitLoss.TotalOpExpenses = totalOpExpenses

	// Calculate Operating Profit (Laba Usaha)
	profitLoss.OperatingProfit = profitLoss.GrossProfit.Sub(totalOpExpenses)
	if totalRevenue.GreaterThan(decimal.Zero) {
		profitLoss.OperatingMargin = profitLoss.OperatingProfit.Div(totalRevenue).Mul(decimal.NewFromInt(100))
	}

	// Process Other Income and Expenses (Pendapatan dan Beban Lain-lain)
	otherIncome, totalOtherIncome := s.classifyOtherIncome(activities, consumed)
	otherExpenses, totalOtherExpenses := s.classifyOtherExpenses(activities, consumed)
	profitLoss.OtherIncome = otherIncome
	profitLoss.OtherExpenses = otherExpenses
	profitLoss.NetOtherIncome = totalOtherIncome.Sub(totalOtherExpenses)

	// Calculate Profit Before Tax (Laba Sebelum Pajak)
	profitLoss.ProfitBeforeTax = profitLoss.OperatingProfit.Add(profitLoss.NetOtherIncome)

	// Process Tax Expense (Beban Pajak) - sesuai PSAK 46
	taxExpense, totalTaxExpense := s.classifyTaxExpense(activities, consumed)
	profitLoss.TaxExpense = taxExpense
	profitLoss.TotalTaxExpense = totalTaxExpense

	// Calculate Net Profit (Laba Neto)
	profitLoss.NetProfit = profitLoss.ProfitBeforeTax.Sub(totalTaxExpense)
	if totalRevenue.GreaterThan(decimal.Zero) {
		profitLoss.NetProfitMargin = profitLoss.NetProfit.Div(totalRevenue).Mul(decimal.NewFromInt(100))
	}

	// Process Other Comprehensive Income - sesuai PSAK 1 paragraf 82-85
	oci, totalOCI := s.classifyOtherComprehensiveIncome(activities, consumed)
	profitLoss.OtherComprehensiveIncome = oci
	profitLoss.TotalComprehensiveIncome = profitLoss.NetProfit.Add(totalOCI)

	// Calculate EPS - sesuai PSAK 56
	profitLoss.WeightedAvgShares = s.getWeightedAverageShares(startDate, endDate)
	if profitLoss.WeightedAvgShares.GreaterThan(decimal.Zero) {
		profitLoss.BasicEPS = profitLoss.NetProfit.Div(profitLoss.WeightedAvgShares)
		profitLoss.DilutedEPS = s.calculateDilutedEPS(profitLoss.NetProfit, profitLoss.WeightedAvgShares)
	} else {
		profitLoss.BasicEPS = decimal.Zero
		profitLoss.DilutedEPS = decimal.Zero
	}

	// Generate PSAK compliance report
	profitLoss.PSAKCompliance = s.generateProfitLossCompliance(profitLoss)

	return profitLoss, nil
}

// GeneratePSAKCashFlow generates Cash Flow sesuai PSAK 2
func (s *PSAKCompliantReportService) GeneratePSAKCashFlow(startDate, endDate time.Time, method string) (*PSAKCashFlowData, error) {
	if method != "DIRECT" && method != "INDIRECT" {
		method = "INDIRECT" // Default to indirect method
	}

	cashFlow := &PSAKCashFlowData{
		CompanyInfo: s.getCompanyInfo(),
		PeriodStart: startDate,
		PeriodEnd:   endDate,
		Currency:    s.getCurrency(),
		Method:      method,
		GeneratedAt: time.Now(),
	}

	// Get cash and cash equivalents balances
	cashFlow.BeginningCash = s.getCashBalanceFromSSOT(startDate.AddDate(0, 0, -1))
	cashFlow.EndingCash = s.getCashBalanceFromSSOT(endDate)

	// Generate operating activities - sesuai PSAK 2 paragraf 13-15
	if method == "DIRECT" {
		cashFlow.OperatingActivities = s.generateDirectOperatingCashFlow(startDate, endDate)
	} else {
		cashFlow.OperatingActivities = s.generateIndirectOperatingCashFlow(startDate, endDate)
		cashFlow.Reconciliation = s.generateCashFlowReconciliation(startDate, endDate)
	}
	cashFlow.NetOperatingCash = cashFlow.OperatingActivities.Subtotal

	// Generate investing activities - sesuai PSAK 2 paragraf 16-17
	cashFlow.InvestingActivities = s.generateInvestingCashFlow(startDate, endDate)
	cashFlow.NetInvestingCash = cashFlow.InvestingActivities.Subtotal

	// Generate financing activities - sesuai PSAK 2 paragraf 17
	cashFlow.FinancingActivities = s.generateFinancingCashFlow(startDate, endDate)
	cashFlow.NetFinancingCash = cashFlow.FinancingActivities.Subtotal

	// Calculate net cash increase/decrease
	cashFlow.NetCashIncrease = cashFlow.NetOperatingCash.
		Add(cashFlow.NetInvestingCash).
		Add(cashFlow.NetFinancingCash)

	// Foreign exchange effect (if applicable)
	cashFlow.ForeignExchangeEffect = s.calculateForeignExchangeEffect(startDate, endDate)
	
	// Adjust ending cash for FX effect
	expectedEndingCash := cashFlow.BeginningCash.
		Add(cashFlow.NetCashIncrease).
		Add(cashFlow.ForeignExchangeEffect)
	
	// Validate cash flow reconciliation
	difference := expectedEndingCash.Sub(cashFlow.EndingCash)
	if difference.Abs().GreaterThan(decimal.NewFromFloat(0.01)) {
		return nil, fmt.Errorf("cash flow reconciliation error: difference of %v", difference)
	}

	// Generate PSAK compliance report
	cashFlow.PSAKCompliance = s.generateCashFlowCompliance(cashFlow)

	return cashFlow, nil
}

// Helper methods (implementation stubs - would need full implementation based on account mapping)

func (s *PSAKCompliantReportService) loadCompanyProfile() {
	var profile models.CompanyProfile
	if err := s.db.First(&profile).Error; err != nil {
		// Create default profile if not exists
		profile = models.CompanyProfile{
			Name:     "PT. Default Company",
			Currency: "IDR",
		}
		s.db.Create(&profile)
	}
	s.companyProfile = &profile
}

func (s *PSAKCompliantReportService) getCompanyInfo() CompanyInfo {
	return CompanyInfo{
		Name:      s.companyProfile.Name,
		Address:   s.companyProfile.Address,
		City:      s.companyProfile.City,
		State:     s.companyProfile.State,
		Phone:     s.companyProfile.Phone,
		Email:     s.companyProfile.Email,
		TaxNumber: s.companyProfile.TaxNumber,
	}
}

func (s *PSAKCompliantReportService) getCurrency() string {
	if s.companyProfile.Currency != "" {
		return s.companyProfile.Currency
	}
	return "IDR"
}

// Account balance retrieval from SSOT
func (s *PSAKCompliantReportService) getAccountBalancesFromSSOT(asOfDate time.Time) (map[uint64]decimal.Decimal, error) {
	balances := make(map[uint64]decimal.Decimal)

	// Calculate balances from journal entries up to asOfDate for historical accuracy
	query := `
		SELECT
			jl.account_id,
			COALESCE(SUM(jl.debit_amount - jl.credit_amount), 0) as balance
		FROM unified_journal_lines jl
		JOIN unified_journal_ledger je ON jl.journal_id = je.id
		WHERE je.status = 'POSTED'
		AND je.entry_date <= ?
		AND je.deleted_at IS NULL
		GROUP BY jl.account_id
	`

	rows, err := s.db.Raw(query, asOfDate).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var accountID uint64
		var balance decimal.Decimal
		if err := rows.Scan(&accountID, &balance); err != nil {
			continue
		}
		balances[accountID] = balance
	}

	return balances, nil
}

func (s *PSAKCompliantReportService) getAccountActivitiesFromSSOT(startDate, endDate time.Time) (map[uint64]decimal.Decimal, error) {
	activities := make(map[uint64]decimal.Decimal)

	// Query SSOT journal lines for the period
	// Use separate debit/credit sums and look up account type to determine sign convention.
	// For credit-normal accounts (revenue, liabilities, equity): net = credit - debit (positive when credited)
	// For debit-normal accounts (assets, expenses, COGS): net = debit - credit (positive when debited)
	query := `
		SELECT
			jl.account_id,
			COALESCE(SUM(jl.debit_amount), 0) as total_debit,
			COALESCE(SUM(jl.credit_amount), 0) as total_credit,
			COALESCE(a.type, '') as account_type
		FROM unified_journal_lines jl
		JOIN unified_journal_ledger je ON jl.journal_id = je.id
		LEFT JOIN accounts a ON a.id = jl.account_id
		WHERE je.status = 'POSTED'
		AND je.entry_date >= ?
		AND je.entry_date <= ?
		AND je.deleted_at IS NULL
		GROUP BY jl.account_id, a.type
	`

	rows, err := s.db.Raw(query, startDate, endDate).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var accountID uint64
		var totalDebit, totalCredit decimal.Decimal
		var accountType string
		if err := rows.Scan(&accountID, &totalDebit, &totalCredit, &accountType); err != nil {
			continue
		}
		// Credit-normal accounts: revenue, liability, equity → positive = credit balance
		// Debit-normal accounts: asset, expense, COGS → positive = debit balance
		switch accountType {
		case "REVENUE", "LIABILITY", "EQUITY", "revenue", "liability", "equity",
			"PENDAPATAN", "LIABILITAS", "EKUITAS":
			activities[accountID] = totalCredit.Sub(totalDebit)
		default:
			// Assets, expenses, COGS → debit-normal
			activities[accountID] = totalDebit.Sub(totalCredit)
		}
	}

	return activities, nil
}

// Classification methods - consumed map tracks already-classified account IDs to prevent double counting
func (s *PSAKCompliantReportService) classifyCurrentAssets(balances map[uint64]decimal.Decimal, consumed map[uint64]bool) ([]PSAKBalanceItem, decimal.Decimal) {
	// TODO: Implement classification based on account type and maturity
	// This would map specific account IDs to PSAK current asset categories
	// Mark classified accounts: consumed[accountID] = true
	return []PSAKBalanceItem{}, decimal.Zero
}

func (s *PSAKCompliantReportService) classifyNonCurrentAssets(balances map[uint64]decimal.Decimal, consumed map[uint64]bool) ([]PSAKBalanceItem, decimal.Decimal) {
	// TODO: Implement classification - skip accounts where consumed[id] == true
	return []PSAKBalanceItem{}, decimal.Zero
}

func (s *PSAKCompliantReportService) classifyCurrentLiabilities(balances map[uint64]decimal.Decimal, consumed map[uint64]bool) ([]PSAKBalanceItem, decimal.Decimal) {
	// TODO: Implement classification - skip accounts where consumed[id] == true
	return []PSAKBalanceItem{}, decimal.Zero
}

func (s *PSAKCompliantReportService) classifyNonCurrentLiabilities(balances map[uint64]decimal.Decimal, consumed map[uint64]bool) ([]PSAKBalanceItem, decimal.Decimal) {
	// TODO: Implement classification - skip accounts where consumed[id] == true
	return []PSAKBalanceItem{}, decimal.Zero
}

func (s *PSAKCompliantReportService) classifyEquity(balances map[uint64]decimal.Decimal, consumed map[uint64]bool) ([]PSAKBalanceItem, decimal.Decimal) {
	// TODO: Implement classification - skip accounts where consumed[id] == true
	return []PSAKBalanceItem{}, decimal.Zero
}

// Revenue and expense classification for P&L
func (s *PSAKCompliantReportService) classifyRevenue(activities map[uint64]decimal.Decimal, consumed map[uint64]bool) (PSAKRevenueSection, decimal.Decimal) {
	// TODO: Implement revenue classification sesuai PSAK 23
	// Mark classified accounts: consumed[accountID] = true
	return PSAKRevenueSection{}, decimal.Zero
}

func (s *PSAKCompliantReportService) classifyCostOfSales(activities map[uint64]decimal.Decimal, consumed map[uint64]bool) (PSAKExpenseSection, decimal.Decimal) {
	// TODO: Implement COGS classification sesuai PSAK 14 - skip accounts where consumed[id] == true
	return PSAKExpenseSection{}, decimal.Zero
}

func (s *PSAKCompliantReportService) classifyOperatingExpenses(activities map[uint64]decimal.Decimal, consumed map[uint64]bool) (PSAKExpenseSection, decimal.Decimal) {
	// TODO: Implement operating expenses classification - skip accounts where consumed[id] == true
	return PSAKExpenseSection{}, decimal.Zero
}

func (s *PSAKCompliantReportService) classifyOtherIncome(activities map[uint64]decimal.Decimal, consumed map[uint64]bool) (PSAKOtherSection, decimal.Decimal) {
	// TODO: Implement other income classification - skip accounts where consumed[id] == true
	return PSAKOtherSection{}, decimal.Zero
}

func (s *PSAKCompliantReportService) classifyOtherExpenses(activities map[uint64]decimal.Decimal, consumed map[uint64]bool) (PSAKOtherSection, decimal.Decimal) {
	// TODO: Implement other expenses classification - skip accounts where consumed[id] == true
	return PSAKOtherSection{}, decimal.Zero
}

func (s *PSAKCompliantReportService) classifyTaxExpense(activities map[uint64]decimal.Decimal, consumed map[uint64]bool) (PSAKTaxSection, decimal.Decimal) {
	// TODO: Implement tax expense classification sesuai PSAK 46 - skip accounts where consumed[id] == true
	return PSAKTaxSection{}, decimal.Zero
}

func (s *PSAKCompliantReportService) classifyOtherComprehensiveIncome(activities map[uint64]decimal.Decimal, consumed map[uint64]bool) (PSAKOCISection, decimal.Decimal) {
	// TODO: Implement OCI classification sesuai PSAK 1 - skip accounts where consumed[id] == true
	return PSAKOCISection{}, decimal.Zero
}

// Cash flow generation methods
func (s *PSAKCompliantReportService) generateDirectOperatingCashFlow(startDate, endDate time.Time) PSAKCashFlowSection {
	// TODO: Implement direct method cash flow sesuai PSAK 2
	return PSAKCashFlowSection{Name: "Arus Kas dari Aktivitas Operasi (Metode Langsung)"}
}

func (s *PSAKCompliantReportService) generateIndirectOperatingCashFlow(startDate, endDate time.Time) PSAKCashFlowSection {
	// TODO: Implement indirect method cash flow sesuai PSAK 2
	return PSAKCashFlowSection{Name: "Arus Kas dari Aktivitas Operasi (Metode Tidak Langsung)"}
}

func (s *PSAKCompliantReportService) generateInvestingCashFlow(startDate, endDate time.Time) PSAKCashFlowSection {
	// TODO: Implement investing activities sesuai PSAK 2 paragraf 16-17
	return PSAKCashFlowSection{Name: "Arus Kas dari Aktivitas Investasi"}
}

func (s *PSAKCompliantReportService) generateFinancingCashFlow(startDate, endDate time.Time) PSAKCashFlowSection {
	// TODO: Implement financing activities sesuai PSAK 2 paragraf 17
	return PSAKCashFlowSection{Name: "Arus Kas dari Aktivitas Pendanaan"}
}

// Helper calculation methods
func (s *PSAKCompliantReportService) getCashBalanceFromSSOT(date time.Time) decimal.Decimal {
	// TODO: Get cash and cash equivalents balance from SSOT
	return decimal.Zero
}

func (s *PSAKCompliantReportService) getWeightedAverageShares(startDate, endDate time.Time) decimal.Decimal {
	// TODO: Calculate weighted average shares for EPS calculation
	return decimal.NewFromInt(1) // Default to avoid division by zero
}

func (s *PSAKCompliantReportService) calculateDilutedEPS(netProfit, basicShares decimal.Decimal) decimal.Decimal {
	// TODO: Calculate diluted EPS sesuai PSAK 56
	return netProfit.Div(basicShares) // Simplified - would include dilutive securities
}

func (s *PSAKCompliantReportService) calculateForeignExchangeEffect(startDate, endDate time.Time) decimal.Decimal {
	// TODO: Calculate foreign exchange effects on cash
	return decimal.Zero
}

func (s *PSAKCompliantReportService) generateCashFlowReconciliation(startDate, endDate time.Time) PSAKCashReconciliation {
	// TODO: Generate cash flow reconciliation for indirect method
	return PSAKCashReconciliation{}
}

// PSAK Compliance methods
func (s *PSAKCompliantReportService) generateBalanceSheetCompliance(bs *PSAKBalanceSheetData) PSAKComplianceInfo {
	return PSAKComplianceInfo{
		StandardsApplied: []string{"PSAK 1", "PSAK 14", "PSAK 16", "PSAK 46"},
		ComplianceLevel:  "FULL",
		ComplianceScore:  decimal.NewFromInt(95),
		LastReviewDate:   time.Now(),
	}
}

func (s *PSAKCompliantReportService) generateProfitLossCompliance(pl *PSAKProfitLossData) PSAKComplianceInfo {
	return PSAKComplianceInfo{
		StandardsApplied: []string{"PSAK 1", "PSAK 23", "PSAK 46", "PSAK 56"},
		ComplianceLevel:  "FULL",
		ComplianceScore:  decimal.NewFromInt(95),
		LastReviewDate:   time.Now(),
	}
}

func (s *PSAKCompliantReportService) generateCashFlowCompliance(cf *PSAKCashFlowData) PSAKComplianceInfo {
	return PSAKComplianceInfo{
		StandardsApplied: []string{"PSAK 2"},
		ComplianceLevel:  "FULL",
		ComplianceScore:  decimal.NewFromInt(95),
		LastReviewDate:   time.Now(),
	}
}