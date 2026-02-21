package services

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// SSOTCashFlowService generates Cash Flow reports from SSOT Journal System
type SSOTCashFlowService struct {
	db *gorm.DB
}

// NewSSOTCashFlowService creates a new SSOT Cash Flow service
func NewSSOTCashFlowService(db *gorm.DB) *SSOTCashFlowService {
	return &SSOTCashFlowService{
		db: db,
	}
}

// SSOTCashFlowData represents the comprehensive Cash Flow statement structure for SSOT
type SSOTCashFlowData struct {
	Company   CompanyInfo `json:"company"`
	StartDate time.Time   `json:"start_date"`
	EndDate   time.Time   `json:"end_date"`
	Currency  string      `json:"currency"`

	// Operating Activities
	OperatingActivities struct {
		NetIncome   float64 `json:"net_income"`
		Adjustments struct {
			Depreciation            float64         `json:"depreciation"`
			Amortization            float64         `json:"amortization"`
			BadDebtExpense          float64         `json:"bad_debt_expense"`
			GainLossOnAssetDisposal float64         `json:"gain_loss_on_asset_disposal"`
			OtherNonCashItems       float64         `json:"other_non_cash_items"`
			TotalAdjustments        float64         `json:"total_adjustments"`
			Items                   []CFSectionItem `json:"items"`
		} `json:"adjustments"`

		WorkingCapitalChanges struct {
			AccountsReceivableChange   float64         `json:"accounts_receivable_change"`
			InventoryChange            float64         `json:"inventory_change"`
			PrepaidExpensesChange      float64         `json:"prepaid_expenses_change"`
			AccountsPayableChange      float64         `json:"accounts_payable_change"`
			AccruedLiabilitiesChange   float64         `json:"accrued_liabilities_change"`
			OtherWorkingCapitalChange  float64         `json:"other_working_capital_change"`
			TotalWorkingCapitalChanges float64         `json:"total_working_capital_changes"`
			Items                      []CFSectionItem `json:"items"`
		} `json:"working_capital_changes"`

		TotalOperatingCashFlow float64 `json:"total_operating_cash_flow"`
	} `json:"operating_activities"`

	// Investing Activities
	InvestingActivities struct {
		PurchaseOfFixedAssets    float64         `json:"purchase_of_fixed_assets"`
		SaleOfFixedAssets        float64         `json:"sale_of_fixed_assets"`
		PurchaseOfInvestments    float64         `json:"purchase_of_investments"`
		SaleOfInvestments        float64         `json:"sale_of_investments"`
		IntangibleAssetPurchases float64         `json:"intangible_asset_purchases"`
		OtherInvestingActivities float64         `json:"other_investing_activities"`
		TotalInvestingCashFlow   float64         `json:"total_investing_cash_flow"`
		Items                    []CFSectionItem `json:"items"`
	} `json:"investing_activities"`

	// Financing Activities
	FinancingActivities struct {
		ShareCapitalIncrease     float64         `json:"share_capital_increase"`
		ShareCapitalDecrease     float64         `json:"share_capital_decrease"`
		LongTermDebtIncrease     float64         `json:"long_term_debt_increase"`
		LongTermDebtDecrease     float64         `json:"long_term_debt_decrease"`
		ShortTermDebtIncrease    float64         `json:"short_term_debt_increase"`
		ShortTermDebtDecrease    float64         `json:"short_term_debt_decrease"`
		DividendsPaid            float64         `json:"dividends_paid"`
		OtherFinancingActivities float64         `json:"other_financing_activities"`
		TotalFinancingCashFlow   float64         `json:"total_financing_cash_flow"`
		Items                    []CFSectionItem `json:"items"`
	} `json:"financing_activities"`

	// Summary
	NetCashFlow     float64 `json:"net_cash_flow"`
	CashAtBeginning float64 `json:"cash_at_beginning"`
	CashAtEnd       float64 `json:"cash_at_end"`

	// Analysis
	CashFlowRatios struct {
		OperatingCashFlowRatio float64 `json:"operating_cash_flow_ratio"`
		CashFlowToDebtRatio    float64 `json:"cash_flow_to_debt_ratio"`
		FreeCashFlow           float64 `json:"free_cash_flow"`
		CashFlowPerShare       float64 `json:"cash_flow_per_share,omitempty"`
	} `json:"cash_flow_ratios"`

	GeneratedAt time.Time `json:"generated_at"`
	Enhanced    bool      `json:"enhanced"`

	// Account Details for Drilldown
	AccountDetails []SSOTAccountBalance `json:"account_details,omitempty"`
}

// CFSectionItem represents an item within a Cash Flow section
type CFSectionItem struct {
	AccountCode string  `json:"account_code"`
	AccountName string  `json:"account_name"`
	Amount      float64 `json:"amount"`
	AccountID   uint    `json:"account_id,omitempty"`
	Type        string  `json:"type"` // increase, decrease, inflow, outflow
}

// CashFlowTransaction extends SSOTAccountBalance with Category for cash flow classification
type CashFlowTransaction struct {
	AccountID       uint    `json:"account_id"`
	AccountCode     string  `json:"account_code"`
	AccountName     string  `json:"account_name"`
	AccountType     string  `json:"account_type"`
	AccountCategory string  `json:"account_category"`
	DebitTotal      float64 `json:"debit_total"`
	CreditTotal     float64 `json:"credit_total"`
	NetBalance      float64 `json:"net_balance"`
}

// GenerateSSOTCashFlow generates Cash Flow statement from SSOT journal system
func (s *SSOTCashFlowService) GenerateSSOTCashFlow(startDate, endDate string) (*SSOTCashFlowData, error) {
	// Default to current fiscal year when parameters are empty
	if strings.TrimSpace(startDate) == "" || strings.TrimSpace(endDate) == "" {
		settingsSvc := NewSettingsService(s.db)
		fyStart, fyEnd, err := settingsSvc.GetCurrentFiscalYearRange()
		if err == nil {
			if strings.TrimSpace(startDate) == "" {
				startDate = fyStart
			}
			if strings.TrimSpace(endDate) == "" {
				endDate = fyEnd
			}
		}
	}
	// Parse dates
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start date format: %v", err)
	}

	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, fmt.Errorf("invalid end date format: %v", err)
	}

	// Make end date inclusive by using exclusive upper bound (end + 1 day)
	endExclusive := end.AddDate(0, 0, 1).Format("2006-01-02")

	// Get non-cash/non-PL transactions for balance sheet movement analysis
	transactions, err := s.getCashFlowTransactions(startDate, endExclusive)
	if err != nil {
		return nil, fmt.Errorf("failed to get cash flow data: %v", err)
	}

	// Get net income for the period (from P&L accounts)
	netIncome, err := s.getNetIncomeFromSSOT(startDate, endExclusive)
	if err != nil {
		return nil, fmt.Errorf("failed to get net income: %v", err)
	}

	// Get beginning and ending cash balances
	beginningCash, endingCash, err := s.getCashBalances(startDate, endExclusive)
	if err != nil {
		return nil, fmt.Errorf("failed to get cash balances: %v", err)
	}

	fmt.Printf("[CashFlow DEBUG] Net Income: %.2f, Beginning Cash: %.2f, Ending Cash: %.2f, Transactions: %d\n",
		netIncome, beginningCash, endingCash, len(transactions))

	// Generate Cash Flow data structure
	cfData := s.generateCashFlowFromTransactions(transactions, netIncome, beginningCash, endingCash, start, end)

	return cfData, nil
}

// getCashFlowTransactions retrieves balance sheet account movements (excluding cash & P&L accounts)
// for the indirect method cash flow statement
func (s *SSOTCashFlowService) getCashFlowTransactions(startDate, endDate string) ([]CashFlowTransaction, error) {
	var transactions []CashFlowTransaction

	// Indirect method: We need changes in non-cash balance sheet accounts
	// EXCLUDE: Cash accounts (1101-xxx) — they ARE the result
	// EXCLUDE: Revenue/COGS/Expense accounts (4xxx-8xxx) — already captured in Net Income
	query := `
		SELECT 
			a.id as account_id,
			a.code as account_code,
			a.name as account_name,
			a.type as account_type,
			COALESCE(a.category, '') as account_category,
			COALESCE(SUM(ujl.debit_amount), 0) as debit_total,
			COALESCE(SUM(ujl.credit_amount), 0) as credit_total,
			CASE 
				WHEN UPPER(a.type) IN ('ASSET', 'EXPENSE') THEN 
					COALESCE(SUM(ujl.debit_amount), 0) - COALESCE(SUM(ujl.credit_amount), 0)
				ELSE 
					COALESCE(SUM(ujl.credit_amount), 0) - COALESCE(SUM(ujl.debit_amount), 0)
			END as net_balance
		FROM accounts a
		LEFT JOIN unified_journal_lines ujl ON ujl.account_id = a.id
		LEFT JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id AND uje.deleted_at IS NULL
		WHERE uje.status = 'POSTED' 
			AND uje.entry_date >= ? 
			AND uje.entry_date < ?
			AND UPPER(COALESCE(uje.source_type, '')) != 'CLOSING'
			AND a.code NOT LIKE '1101%'
			AND a.code NOT LIKE '4%'
			AND a.code NOT LIKE '5%'
			AND a.code NOT LIKE '6%'
			AND a.code NOT LIKE '7%'
			AND a.code NOT LIKE '8%'
			AND a.deleted_at IS NULL
		GROUP BY a.id, a.code, a.name, a.type, a.category
		HAVING COALESCE(SUM(ujl.debit_amount), 0) > 0 OR COALESCE(SUM(ujl.credit_amount), 0) > 0
		ORDER BY a.code
	`

	if err := s.db.Raw(query, startDate, endDate).Scan(&transactions).Error; err != nil {
		return nil, fmt.Errorf("error executing cash flow transactions query: %v", err)
	}

	// Debug log
	for _, t := range transactions {
		fmt.Printf("[CashFlow DEBUG] Transaction: %s %s | Type: %s | Cat: %s | Debit: %.2f | Credit: %.2f | Net: %.2f\n",
			t.AccountCode, t.AccountName, t.AccountType, t.AccountCategory,
			t.DebitTotal, t.CreditTotal, t.NetBalance)
	}

	return transactions, nil
}

// getNetIncomeFromSSOT calculates net income from revenue and expense accounts
func (s *SSOTCashFlowService) getNetIncomeFromSSOT(startDate, endDate string) (float64, error) {
	var result struct {
		NetIncome float64 `json:"net_income"`
	}

	// Revenue: credit normal → credit - debit = positive when revenue exists
	// Expense: debit normal → debit - credit = positive when expense exists
	// Net Income = Revenue - Expense
	query := `
		SELECT 
			COALESCE(
			SUM(CASE 
				WHEN a.code LIKE '4%' OR a.code LIKE '7%' THEN 
					COALESCE(ujl.credit_amount, 0) - COALESCE(ujl.debit_amount, 0)
				WHEN a.code LIKE '5%' OR a.code LIKE '6%' OR a.code LIKE '8%' OR a.code LIKE '9%' THEN 
					-(COALESCE(ujl.debit_amount, 0) - COALESCE(ujl.credit_amount, 0))
				ELSE 0
			END), 0
		) as net_income
		FROM accounts a
		LEFT JOIN unified_journal_lines ujl ON ujl.account_id = a.id
		LEFT JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id AND uje.deleted_at IS NULL
		WHERE uje.status = 'POSTED' 
			AND uje.entry_date >= ? 
			AND uje.entry_date < ?
			AND UPPER(COALESCE(uje.source_type, '')) != 'CLOSING'
			AND a.deleted_at IS NULL
			AND (a.code LIKE '4%' OR a.code LIKE '5%' OR a.code LIKE '6%' OR a.code LIKE '7%' OR a.code LIKE '8%' OR a.code LIKE '9%')
	`

	if err := s.db.Raw(query, startDate, endDate).Scan(&result).Error; err != nil {
		return 0, fmt.Errorf("error calculating net income: %v", err)
	}

	fmt.Printf("[CashFlow DEBUG] Net Income calculated: %.2f\n", result.NetIncome)
	return result.NetIncome, nil
}

// getCashBalances gets beginning and ending cash balances
// Cash accounts: 1101-xxx (KAS KECIL, KAS BESAR, BANK-GIRO, BANK-TABUNGAN)
func (s *SSOTCashFlowService) getCashBalances(startDate, endDate string) (float64, float64, error) {
	var beginningCash, endingCash float64

	// Beginning cash = all cash account movements BEFORE the start date
	beginningQuery := `
		SELECT 
			COALESCE(SUM(
				COALESCE(ujl.debit_amount, 0) - COALESCE(ujl.credit_amount, 0)
			), 0) as cash_balance
		FROM accounts a
		LEFT JOIN unified_journal_lines ujl ON ujl.account_id = a.id
		LEFT JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
		WHERE uje.status = 'POSTED' 
			AND uje.deleted_at IS NULL
			AND a.deleted_at IS NULL
			AND uje.entry_date < ?
			AND a.code LIKE '1101%'
	`

	// Ending cash = all cash account movements up to (but not including) the end date
	// Since endDate is already exclusive (end + 1 day), this covers the full period
	endingQuery := `
		SELECT 
			COALESCE(SUM(
				COALESCE(ujl.debit_amount, 0) - COALESCE(ujl.credit_amount, 0)
			), 0) as cash_balance
		FROM accounts a
		LEFT JOIN unified_journal_lines ujl ON ujl.account_id = a.id
		LEFT JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
		WHERE uje.status = 'POSTED' 
			AND uje.deleted_at IS NULL
			AND a.deleted_at IS NULL
			AND uje.entry_date < ?
			AND a.code LIKE '1101%'
	`

	var beginResult struct {
		CashBalance float64 `json:"cash_balance"`
	}
	var endResult struct {
		CashBalance float64 `json:"cash_balance"`
	}

	if err := s.db.Raw(beginningQuery, startDate).Scan(&beginResult).Error; err != nil {
		return 0, 0, fmt.Errorf("error getting beginning cash balance: %v", err)
	}

	if err := s.db.Raw(endingQuery, endDate).Scan(&endResult).Error; err != nil {
		return 0, 0, fmt.Errorf("error getting ending cash balance: %v", err)
	}

	beginningCash = beginResult.CashBalance
	endingCash = endResult.CashBalance

	fmt.Printf("[CashFlow DEBUG] Beginning Cash (before %s): %.2f, Ending Cash (before %s): %.2f\n",
		startDate, beginningCash, endDate, endingCash)

	return beginningCash, endingCash, nil
}

// generateCashFlowFromTransactions creates the Cash Flow structure from transactions
func (s *SSOTCashFlowService) generateCashFlowFromTransactions(transactions []CashFlowTransaction, netIncome, beginningCash, endingCash float64, start, end time.Time) *SSOTCashFlowData {
	// Get company information from settings
	settingsSvc := NewSettingsService(s.db)
	settings, err := settingsSvc.GetSettings()

	var companyInfo CompanyInfo
	if err == nil && settings != nil {
		companyInfo = CompanyInfo{
			Name:    settings.CompanyName,
			Address: settings.CompanyAddress,
			Phone:   settings.CompanyPhone,
			Email:   settings.CompanyEmail,
		}
	} else {
		companyInfo = CompanyInfo{
			Name:    "PT. Sistem Akuntansi Indonesia",
			Address: "Jl. Sudirman Kav. 45-46, Jakarta Pusat 10210, Indonesia",
			Phone:   "+62-21-5551234",
			Email:   "info@sistemakuntansi.co.id",
		}
	}

	cfData := &SSOTCashFlowData{
		Company:         companyInfo,
		StartDate:       start,
		EndDate:         end,
		Currency:        "IDR",
		Enhanced:        true,
		GeneratedAt:     time.Now(),
		CashAtBeginning: beginningCash,
		CashAtEnd:       endingCash,
	}

	// Initialize sections
	cfData.OperatingActivities.NetIncome = netIncome
	cfData.OperatingActivities.Adjustments.Items = []CFSectionItem{}
	cfData.OperatingActivities.WorkingCapitalChanges.Items = []CFSectionItem{}
	cfData.InvestingActivities.Items = []CFSectionItem{}
	cfData.FinancingActivities.Items = []CFSectionItem{}

	// Process each transaction — categorize based on account code + category
	for _, transaction := range transactions {
		code := transaction.AccountCode
		amount := transaction.NetBalance

		// Skip if amount is zero
		if amount == 0 {
			continue
		}

		item := CFSectionItem{
			AccountCode: transaction.AccountCode,
			AccountName: transaction.AccountName,
			Amount:      amount,
			AccountID:   transaction.AccountID,
		}

		// Categorize based on account codes matching actual COA structure
		s.categorizeCashFlowTransaction(cfData, item, code, amount, transaction.AccountCategory)
	}

	// Calculate totals and ratios
	s.calculateCashFlowTotals(cfData)

	return cfData
}

// categorizeCashFlowTransaction categorizes transactions into cash flow activities
// Uses the actual COA structure from account_templates.go:
//
//	Cash:          1101-xxx  → SKIP (cash is the result, not the cause)
//	AR:            1102-xxx  → Operating (Working Capital)
//	Inventory:     1103-xxx  → Operating (Working Capital) — Trading/Manufacturing only
//	Prepaid:       1104-xxx, 1105-xxx → Operating (Working Capital)
//	Fixed Assets:  1201-xxx  → Investing
//	Acc Depr:      1202-xxx  → Operating (Non-cash Adjustment)
//	AP:            2101-xxx  → Operating (Working Capital)
//	Tax Payable:   2102-xxx  → Operating (Working Capital)
//	Accrued:       2103-xxx  → Operating (Working Capital)
//	LT Debt:       22xx      → Financing
//	Equity:        31xx      → Financing
//	Revenue:       4xxx      → SKIP (in Net Income)
//	COGS/Expense:  5xxx-8xxx → SKIP (in Net Income)
func (s *SSOTCashFlowService) categorizeCashFlowTransaction(cfData *SSOTCashFlowData, item CFSectionItem, code string, amount float64, category string) {
	switch {

	// ============ OPERATING ACTIVITIES — Non-cash Adjustments ============

	// Accumulated Depreciation (1202-xxx) — non-cash item, add back to operating
	case strings.HasPrefix(code, "1202"):
		// Increase in acc depreciation = non-cash expense already in Net Income
		// For indirect method: add back depreciation (it reduced net income but isn't cash)
		// AccDepr is a contra-asset: credit increases it. NetBalance = debit - credit (negative = increase)
		// So negative amount means depreciation was charged → flip sign to add back
		depAmount := amount * -1
		cfData.OperatingActivities.Adjustments.Depreciation += depAmount
		item.Amount = depAmount
		item.Type = "increase"
		cfData.OperatingActivities.Adjustments.Items = append(cfData.OperatingActivities.Adjustments.Items, item)

	// ============ OPERATING ACTIVITIES — Working Capital Changes ============
	// For assets: increase = cash outflow (flip sign), decrease = cash inflow
	// For liabilities: increase = cash inflow (keep sign), decrease = cash outflow

	// Accounts Receivable (1102-xxx)
	case strings.HasPrefix(code, "1102"):
		// AR is an asset. Increase in AR = cash NOT received yet = cash outflow
		change := amount * -1
		cfData.OperatingActivities.WorkingCapitalChanges.AccountsReceivableChange += change
		item.Amount = change
		if amount > 0 {
			item.Type = "decrease" // Asset increase = cash decrease
		} else {
			item.Type = "increase" // Asset decrease = cash increase (collected)
		}
		cfData.OperatingActivities.WorkingCapitalChanges.Items = append(cfData.OperatingActivities.WorkingCapitalChanges.Items, item)

	// Inventory (1103-xxx) — Trading & Manufacturing companies
	case strings.HasPrefix(code, "1103"):
		// Inventory is an asset. Increase in inventory = cash outflow
		change := amount * -1
		cfData.OperatingActivities.WorkingCapitalChanges.InventoryChange += change
		item.Amount = change
		if amount > 0 {
			item.Type = "decrease"
		} else {
			item.Type = "increase"
		}
		cfData.OperatingActivities.WorkingCapitalChanges.Items = append(cfData.OperatingActivities.WorkingCapitalChanges.Items, item)

	// Prepaid Expenses & Advances (1104-xxx)
	case strings.HasPrefix(code, "1104"):
		change := amount * -1
		cfData.OperatingActivities.WorkingCapitalChanges.PrepaidExpensesChange += change
		item.Amount = change
		if amount > 0 {
			item.Type = "decrease"
		} else {
			item.Type = "increase"
		}
		cfData.OperatingActivities.WorkingCapitalChanges.Items = append(cfData.OperatingActivities.WorkingCapitalChanges.Items, item)

	// Prepaid Taxes (1105-xxx — PPH dibayar dimuka)
	case strings.HasPrefix(code, "1105"):
		change := amount * -1
		cfData.OperatingActivities.WorkingCapitalChanges.PrepaidExpensesChange += change
		item.Amount = change
		item.AccountName = item.AccountName + " (Pajak Dibayar Dimuka)"
		if amount > 0 {
			item.Type = "decrease"
		} else {
			item.Type = "increase"
		}
		cfData.OperatingActivities.WorkingCapitalChanges.Items = append(cfData.OperatingActivities.WorkingCapitalChanges.Items, item)

	// Accounts Payable (2101-xxx)
	case strings.HasPrefix(code, "2101"):
		// AP is a liability. Increase in AP = cash NOT paid yet = cash inflow
		cfData.OperatingActivities.WorkingCapitalChanges.AccountsPayableChange += amount
		if amount > 0 {
			item.Type = "increase" // Liability increase = cash inflow (deferred payment)
		} else {
			item.Type = "decrease" // Liability decrease = cash outflow (paid)
		}
		cfData.OperatingActivities.WorkingCapitalChanges.Items = append(cfData.OperatingActivities.WorkingCapitalChanges.Items, item)

	// Tax Payable — PPN (2102-xxx)
	case strings.HasPrefix(code, "2102"):
		// Tax payable is a liability. Same treatment as AP
		cfData.OperatingActivities.WorkingCapitalChanges.OtherWorkingCapitalChange += amount
		if amount > 0 {
			item.Type = "increase"
		} else {
			item.Type = "decrease"
		}
		cfData.OperatingActivities.WorkingCapitalChanges.Items = append(cfData.OperatingActivities.WorkingCapitalChanges.Items, item)

	// Accrued Liabilities (2103-xxx — Biaya yang masih harus dibayar)
	case strings.HasPrefix(code, "2103"):
		cfData.OperatingActivities.WorkingCapitalChanges.AccruedLiabilitiesChange += amount
		if amount > 0 {
			item.Type = "increase"
		} else {
			item.Type = "decrease"
		}
		cfData.OperatingActivities.WorkingCapitalChanges.Items = append(cfData.OperatingActivities.WorkingCapitalChanges.Items, item)

	// Other current liabilities (210x except 2101, 2102, 2103 — already handled above)
	case strings.HasPrefix(code, "210") && !strings.HasPrefix(code, "2101") && !strings.HasPrefix(code, "2102") && !strings.HasPrefix(code, "2103"):
		cfData.OperatingActivities.WorkingCapitalChanges.OtherWorkingCapitalChange += amount
		if amount > 0 {
			item.Type = "increase"
		} else {
			item.Type = "decrease"
		}
		cfData.OperatingActivities.WorkingCapitalChanges.Items = append(cfData.OperatingActivities.WorkingCapitalChanges.Items, item)

	// ============ INVESTING ACTIVITIES ============

	// Fixed Assets (1201-xxx — Tanah, Bangunan, Kendaraan, Peralatan, Mesin)
	case strings.HasPrefix(code, "1201"):
		if amount > 0 {
			// Asset increase = purchase = cash outflow
			cfData.InvestingActivities.PurchaseOfFixedAssets += amount * -1
			item.Amount = amount * -1
			item.Type = "outflow"
		} else {
			// Asset decrease = sale/disposal = cash inflow
			cfData.InvestingActivities.SaleOfFixedAssets += amount * -1
			item.Amount = amount * -1
			item.Type = "inflow"
		}
		cfData.InvestingActivities.Items = append(cfData.InvestingActivities.Items, item)

	// Other non-current assets (12xx except 1201, 1202 already handled)
	case strings.HasPrefix(code, "12") && !strings.HasPrefix(code, "1201") && !strings.HasPrefix(code, "1202"):
		if amount > 0 {
			cfData.InvestingActivities.OtherInvestingActivities += amount * -1
			item.Amount = amount * -1
			item.Type = "outflow"
		} else {
			cfData.InvestingActivities.OtherInvestingActivities += amount * -1
			item.Amount = amount * -1
			item.Type = "inflow"
		}
		cfData.InvestingActivities.Items = append(cfData.InvestingActivities.Items, item)

	// ============ FINANCING ACTIVITIES ============

	// Share Capital / Equity (31xx — Modal Disetor, Laba Ditahan, etc.)
	case strings.HasPrefix(code, "31"):
		if amount > 0 {
			cfData.FinancingActivities.ShareCapitalIncrease += amount
			item.Type = "inflow"
		} else {
			cfData.FinancingActivities.ShareCapitalDecrease += amount * -1
			item.Amount = amount
			item.Type = "outflow"
		}
		cfData.FinancingActivities.Items = append(cfData.FinancingActivities.Items, item)

	// Long-term Debt (22xx — Pinjaman Bank Jangka Panjang)
	case strings.HasPrefix(code, "22"):
		if amount > 0 {
			cfData.FinancingActivities.LongTermDebtIncrease += amount
			item.Type = "inflow"
		} else {
			cfData.FinancingActivities.LongTermDebtDecrease += amount * -1
			item.Amount = amount
			item.Type = "outflow"
		}
		cfData.FinancingActivities.Items = append(cfData.FinancingActivities.Items, item)

	// Short-term Debt (211x — if any short-term borrowings beyond AP)
	case strings.HasPrefix(code, "211"):
		if amount > 0 {
			cfData.FinancingActivities.ShortTermDebtIncrease += amount
			item.Type = "inflow"
		} else {
			cfData.FinancingActivities.ShortTermDebtDecrease += amount * -1
			item.Amount = amount
			item.Type = "outflow"
		}
		cfData.FinancingActivities.Items = append(cfData.FinancingActivities.Items, item)

	// Dividends (check by account name)
	case strings.Contains(strings.ToLower(item.AccountName), "dividend") ||
		strings.Contains(strings.ToLower(item.AccountName), "dividen"):
		cfData.FinancingActivities.DividendsPaid += amount * -1
		item.Amount = amount * -1
		item.Type = "outflow"
		cfData.FinancingActivities.Items = append(cfData.FinancingActivities.Items, item)

	// ============ CATCH-ALL ============
	default:
		// Any uncategorized balance sheet movement goes to Other Working Capital
		fmt.Printf("[CashFlow DEBUG] Uncategorized account: %s %s (amount: %.2f, category: %s)\n",
			code, item.AccountName, amount, category)
		cfData.OperatingActivities.WorkingCapitalChanges.OtherWorkingCapitalChange += amount
		item.Type = "other"
		cfData.OperatingActivities.WorkingCapitalChanges.Items = append(cfData.OperatingActivities.WorkingCapitalChanges.Items, item)
	}
}

// calculateCashFlowTotals calculates all totals and ratios
func (s *SSOTCashFlowService) calculateCashFlowTotals(cfData *SSOTCashFlowData) {
	// Calculate operating activities adjustments total
	cfData.OperatingActivities.Adjustments.TotalAdjustments =
		cfData.OperatingActivities.Adjustments.Depreciation +
			cfData.OperatingActivities.Adjustments.Amortization +
			cfData.OperatingActivities.Adjustments.BadDebtExpense +
			cfData.OperatingActivities.Adjustments.GainLossOnAssetDisposal +
			cfData.OperatingActivities.Adjustments.OtherNonCashItems

	// Calculate working capital changes total
	cfData.OperatingActivities.WorkingCapitalChanges.TotalWorkingCapitalChanges =
		cfData.OperatingActivities.WorkingCapitalChanges.AccountsReceivableChange +
			cfData.OperatingActivities.WorkingCapitalChanges.InventoryChange +
			cfData.OperatingActivities.WorkingCapitalChanges.PrepaidExpensesChange +
			cfData.OperatingActivities.WorkingCapitalChanges.AccountsPayableChange +
			cfData.OperatingActivities.WorkingCapitalChanges.AccruedLiabilitiesChange +
			cfData.OperatingActivities.WorkingCapitalChanges.OtherWorkingCapitalChange

	// Calculate total operating cash flow
	cfData.OperatingActivities.TotalOperatingCashFlow =
		cfData.OperatingActivities.NetIncome +
			cfData.OperatingActivities.Adjustments.TotalAdjustments +
			cfData.OperatingActivities.WorkingCapitalChanges.TotalWorkingCapitalChanges

	// Calculate investing activities total
	cfData.InvestingActivities.TotalInvestingCashFlow =
		cfData.InvestingActivities.PurchaseOfFixedAssets +
			cfData.InvestingActivities.SaleOfFixedAssets +
			cfData.InvestingActivities.PurchaseOfInvestments +
			cfData.InvestingActivities.SaleOfInvestments +
			cfData.InvestingActivities.IntangibleAssetPurchases +
			cfData.InvestingActivities.OtherInvestingActivities

	// Calculate financing activities total
	cfData.FinancingActivities.TotalFinancingCashFlow =
		cfData.FinancingActivities.ShareCapitalIncrease -
			cfData.FinancingActivities.ShareCapitalDecrease +
			cfData.FinancingActivities.LongTermDebtIncrease -
			cfData.FinancingActivities.LongTermDebtDecrease +
			cfData.FinancingActivities.ShortTermDebtIncrease -
			cfData.FinancingActivities.ShortTermDebtDecrease -
			cfData.FinancingActivities.DividendsPaid +
			cfData.FinancingActivities.OtherFinancingActivities

	// Calculate net cash flow
	cfData.NetCashFlow =
		cfData.OperatingActivities.TotalOperatingCashFlow +
			cfData.InvestingActivities.TotalInvestingCashFlow +
			cfData.FinancingActivities.TotalFinancingCashFlow

	// Calculate ratios
	if cfData.OperatingActivities.NetIncome != 0 {
		cfData.CashFlowRatios.OperatingCashFlowRatio = cfData.OperatingActivities.TotalOperatingCashFlow / cfData.OperatingActivities.NetIncome
	}

	// Free Cash Flow = Operating CF + Capital Expenditures (which is already negative)
	cfData.CashFlowRatios.FreeCashFlow = cfData.OperatingActivities.TotalOperatingCashFlow + cfData.InvestingActivities.PurchaseOfFixedAssets

	// Validate cash balance reconciliation
	expectedEndingCash := cfData.CashAtBeginning + cfData.NetCashFlow
	diff := cfData.CashAtEnd - expectedEndingCash
	if diff != 0 {
		// Log the discrepancy for debugging
		fmt.Printf("[CashFlow DEBUG] Reconciliation diff: %.2f (Expected: %.2f, Actual: %.2f)\n",
			diff, expectedEndingCash, cfData.CashAtEnd)
		// Adjust via OtherNonCashItems to balance
		cfData.OperatingActivities.Adjustments.OtherNonCashItems += diff
		cfData.OperatingActivities.Adjustments.TotalAdjustments += diff
		cfData.OperatingActivities.TotalOperatingCashFlow += diff
		cfData.NetCashFlow += diff
	}

	fmt.Printf("[CashFlow DEBUG] TOTALS — Operating: %.2f, Investing: %.2f, Financing: %.2f, Net: %.2f\n",
		cfData.OperatingActivities.TotalOperatingCashFlow,
		cfData.InvestingActivities.TotalInvestingCashFlow,
		cfData.FinancingActivities.TotalFinancingCashFlow,
		cfData.NetCashFlow)
}
