package services

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
)

// SSOTBalanceSheetService generates Balance Sheet reports from SSOT Journal System
type SSOTBalanceSheetService struct {
	db *gorm.DB
}

// NewSSOTBalanceSheetService creates a new SSOT Balance Sheet service
func NewSSOTBalanceSheetService(db *gorm.DB) *SSOTBalanceSheetService {
	return &SSOTBalanceSheetService{
		db: db,
	}
}

func (s *SSOTBalanceSheetService) loadConfiguredVATAccountIDs() (uint, uint) {
	settings, err := NewTaxAccountService(s.db).GetSettings()
	if err != nil || settings == nil {
		if err != nil {
			log.Printf("⚠️ Failed to load tax settings for balance sheet VAT netting: %v", err)
		}
		return 0, 0
	}

	return settings.PurchaseInputVATAccountID, settings.SalesOutputVATAccountID
}

func isPurchaseInputVATAccount(accountID uint, code, name string, configuredID uint) bool {
	lowerName := strings.ToLower(name)
	return (configuredID != 0 && accountID == configuredID) ||
		strings.Contains(lowerName, "ppn masukan") ||
		code == "1105-005" ||
		code == "1240" ||
		strings.HasPrefix(code, "2102-002")
}

func isSalesOutputVATAccount(accountID uint, code, name string, configuredID uint) bool {
	lowerName := strings.ToLower(name)
	return (configuredID != 0 && accountID == configuredID) ||
		strings.Contains(lowerName, "ppn keluaran") ||
		code == "2103" ||
		strings.HasPrefix(code, "2102-001")
}

func (s *SSOTBalanceSheetService) currentEarningsStartDate(asOf time.Time) *time.Time {
	var lastClosed models.AccountingPeriod
	err := s.db.
		Where("deleted_at IS NULL AND is_closed = ? AND end_date <= ?", true, normalizeClosingDayEnd(asOf)).
		Order("end_date DESC").
		First(&lastClosed).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			log.Printf("failed to load last closed period for current earnings window: %v", err)
		}
		return nil
	}

	start := nextClosingStartDate(lastClosed.EndDate)
	return &start
}

// SSOTBalanceSheetData represents the comprehensive Balance Sheet structure for SSOT
type SSOTBalanceSheetData struct {
	Company  CompanyInfo `json:"company"`
	AsOfDate time.Time   `json:"as_of_date"`
	Currency string      `json:"currency"`

	// Assets Section
	Assets struct {
		CurrentAssets struct {
			Cash               float64         `json:"cash"`
			Receivables        float64         `json:"receivables"`
			Inventory          float64         `json:"inventory"`
			PrepaidExpenses    float64         `json:"prepaid_expenses"`
			OtherCurrentAssets float64         `json:"other_current_assets"`
			TotalCurrentAssets float64         `json:"total_current_assets"`
			Items              []BSAccountItem `json:"items"`
		} `json:"current_assets"`

		NonCurrentAssets struct {
			FixedAssets           float64         `json:"fixed_assets"`
			IntangibleAssets      float64         `json:"intangible_assets"`
			Investments           float64         `json:"investments"`
			OtherNonCurrentAssets float64         `json:"other_non_current_assets"`
			TotalNonCurrentAssets float64         `json:"total_non_current_assets"`
			Items                 []BSAccountItem `json:"items"`
		} `json:"non_current_assets"`

		TotalAssets float64 `json:"total_assets"`
	} `json:"assets"`

	// Liabilities Section
	Liabilities struct {
		CurrentLiabilities struct {
			AccountsPayable         float64         `json:"accounts_payable"`
			ShortTermDebt           float64         `json:"short_term_debt"`
			AccruedLiabilities      float64         `json:"accrued_liabilities"`
			TaxPayable              float64         `json:"tax_payable"`
			OtherCurrentLiabilities float64         `json:"other_current_liabilities"`
			TotalCurrentLiabilities float64         `json:"total_current_liabilities"`
			Items                   []BSAccountItem `json:"items"`
		} `json:"current_liabilities"`

		NonCurrentLiabilities struct {
			LongTermDebt               float64         `json:"long_term_debt"`
			DeferredTax                float64         `json:"deferred_tax"`
			OtherNonCurrentLiabilities float64         `json:"other_non_current_liabilities"`
			TotalNonCurrentLiabilities float64         `json:"total_non_current_liabilities"`
			Items                      []BSAccountItem `json:"items"`
		} `json:"non_current_liabilities"`

		TotalLiabilities float64 `json:"total_liabilities"`
	} `json:"liabilities"`

	// Equity Section
	Equity struct {
		ShareCapital     float64         `json:"share_capital"`
		RetainedEarnings float64         `json:"retained_earnings"`
		OtherEquity      float64         `json:"other_equity"`
		TotalEquity      float64         `json:"total_equity"`
		Items            []BSAccountItem `json:"items"`
	} `json:"equity"`

	// Balance Check
	TotalLiabilitiesAndEquity float64 `json:"total_liabilities_and_equity"`
	IsBalanced                bool    `json:"is_balanced"`
	BalanceDifference         float64 `json:"balance_difference"`

	GeneratedAt time.Time `json:"generated_at"`
	Enhanced    bool      `json:"enhanced"`

	// Account Details for Drilldown
	AccountDetails []SSOTAccountBalance `json:"account_details,omitempty"`
}

// BSAccountItem represents an account item within a Balance Sheet section
type BSAccountItem struct {
	AccountCode string  `json:"account_code"`
	AccountName string  `json:"account_name"`
	Amount      float64 `json:"amount"`
	AccountID   uint    `json:"account_id,omitempty"`
}

// GenerateSSOTBalanceSheet generates Balance Sheet statement from SSOT journal system
func (s *SSOTBalanceSheetService) GenerateSSOTBalanceSheet(asOfDate string) (*SSOTBalanceSheetData, error) {
	// Default to end of current fiscal year if parameter is empty
	if strings.TrimSpace(asOfDate) == "" {
		settingsSvc := NewSettingsService(s.db)
		_, fyEnd, err := settingsSvc.GetCurrentFiscalYearRange()
		if err == nil {
			asOfDate = fyEnd
		}
	}
	// Parse date
	asOf, err := time.Parse("2006-01-02", asOfDate)
	if err != nil {
		return nil, fmt.Errorf("invalid as of date format: %v", err)
	}

	// Get account balances from SSOT journal entries up to the specified date
	accountBalances, err := s.getAccountBalancesFromSSOT(asOfDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get account balances: %v", err)
	}

	// Generate Balance Sheet data structure
	bsData := s.generateBalanceSheetFromBalances(accountBalances, asOf)

	return bsData, nil
}

// getAccountBalancesFromSSOT retrieves account balances from SSOT journal system up to a specific date
// This version deduplicates accounts by code and aggregates their balances
// IMPORTANT: Always calculate from journal lines with date filter to ensure accuracy for historical dates
func (s *SSOTBalanceSheetService) getAccountBalancesFromSSOT(asOfDate string) ([]SSOTAccountBalance, error) {
	var rawBalances []SSOTAccountBalance

	// ALWAYS use journal-based calculation with date filter
	// This ensures we get accurate balances for any point in time, not just current balance
	log.Printf("Calculating balances from SSOT journal lines up to %s", asOfDate)

	journalQuery := `
		SELECT 
			MIN(a.id) as account_id,
			a.code as account_code,
			MAX(a.name) as account_name,
			UPPER(a.type) as account_type,
			COALESCE(SUM(ujl.debit_amount), 0) as debit_total,
			COALESCE(SUM(ujl.credit_amount), 0) as credit_total,
			CASE 
				WHEN UPPER(a.type) IN ('ASSET', 'EXPENSE') THEN 
					COALESCE(SUM(ujl.debit_amount), 0) - COALESCE(SUM(ujl.credit_amount), 0)
				ELSE 
					COALESCE(SUM(ujl.credit_amount), 0) - COALESCE(SUM(ujl.debit_amount), 0)
			END as net_balance
		FROM accounts a
		INNER JOIN unified_journal_lines ujl ON ujl.account_id = a.id
		INNER JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
	WHERE COALESCE(a.is_header, false) = false
	  AND UPPER(a.type) IN ('ASSET', 'LIABILITY', 'EQUITY')
	  AND a.deleted_at IS NULL
	  AND uje.status = 'POSTED'
	  AND uje.deleted_at IS NULL
	  AND DATE(uje.entry_date) <= ?
	GROUP BY a.code, UPPER(a.type)
		HAVING COALESCE(SUM(ujl.debit_amount), 0) <> 0 
		    OR COALESCE(SUM(ujl.credit_amount), 0) <> 0
		ORDER BY a.code
	`

	if err := s.db.Raw(journalQuery, asOfDate).Scan(&rawBalances).Error; err != nil {
		return nil, fmt.Errorf("error executing journal balances query: %v", err)
	}

	log.Printf("Retrieved %d accounts from journal lines up to %s", len(rawBalances), asOfDate)

	// If still no data, try legacy journals as last resort
	if len(rawBalances) == 0 {
		legacy, lerr := s.getAccountBalancesFromLegacy(asOfDate)
		if lerr == nil && len(legacy) > 0 {
			log.Printf("Using legacy journals: %d accounts", len(legacy))
			return legacy, nil
		}
	}

	// Log liability accounts
	log.Printf("=== DEDUPLICATED LIABILITY ACCOUNTS ===")
	for _, balance := range rawBalances {
		if strings.ToUpper(balance.AccountType) == "LIABILITY" {
			log.Printf("Liability: %s - %s | Balance: %.2f | Debit: %.2f | Credit: %.2f",
				balance.AccountCode, balance.AccountName, balance.NetBalance,
				balance.DebitTotal, balance.CreditTotal)
		}
	}

	// Log sample accounts for verification
	log.Printf("=== SAMPLE ACCOUNTS (first 10) ===")
	for i, balance := range rawBalances {
		if i < 10 {
			log.Printf("%s - %s | Type: %s | Balance: %.2f",
				balance.AccountCode, balance.AccountName, balance.AccountType, balance.NetBalance)
		}
	}

	return rawBalances, nil
}

// getAccountBalancesFromAccountTable gets account balances directly from accounts.balance when SSOT data is not available
// This version deduplicates accounts by code and aggregates their balances
func (s *SSOTBalanceSheetService) getAccountBalancesFromAccountTable() ([]SSOTAccountBalance, error) {
	var balances []SSOTAccountBalance

	query := `
		SELECT 
			MIN(a.id) as account_id,
			a.code as account_code,
			MAX(a.name) as account_name,
			UPPER(a.type) as account_type,
			0 as debit_total,
			0 as credit_total,
			SUM(COALESCE(a.balance, 0)) as net_balance
	FROM accounts a
	WHERE UPPER(a.type) IN ('ASSET', 'LIABILITY', 'EQUITY')
	  AND COALESCE(a.is_header, false) = false
	  AND a.deleted_at IS NULL
	GROUP BY a.code, UPPER(a.type)
		HAVING SUM(COALESCE(a.balance, 0)) <> 0
		ORDER BY a.code
	`

	if err := s.db.Raw(query).Scan(&balances).Error; err != nil {
		return nil, fmt.Errorf("error executing fallback account balances query: %v", err)
	}

	log.Printf("Fallback method: Found %d deduplicated accounts with direct balances", len(balances))
	for i, balance := range balances {
		if i < 10 && balance.NetBalance != 0 { // Log first 10 non-zero accounts
			log.Printf("Fallback Account %s (%s): Type=%s, Balance=%.2f",
				balance.AccountCode, balance.AccountName, balance.AccountType, balance.NetBalance)
		}
	}

	return balances, nil
}

// getAccountBalancesFromLegacy retrieves account balances using legacy journal tables up to an as-of date
// This version also deduplicates accounts by code
func (s *SSOTBalanceSheetService) getAccountBalancesFromLegacy(asOfDate string) ([]SSOTAccountBalance, error) {
	var balances []SSOTAccountBalance

	// Legacy query with deduplication by account code
	legacyQuery := `
		SELECT 
			MIN(a.id) as account_id,
			a.code as account_code,
			MAX(a.name) as account_name,
			UPPER(a.type) as account_type,
			COALESCE(SUM(jl.debit_amount), 0) as debit_total,
			COALESCE(SUM(jl.credit_amount), 0) as credit_total,
			CASE 
				WHEN UPPER(a.type) IN ('ASSET', 'EXPENSE') THEN 
					COALESCE(SUM(jl.debit_amount), 0) - COALESCE(SUM(jl.credit_amount), 0)
				ELSE 
					COALESCE(SUM(jl.credit_amount), 0) - COALESCE(SUM(jl.debit_amount), 0)
			END as net_balance
		FROM accounts a
		LEFT JOIN journal_lines jl ON jl.account_id = a.id
		LEFT JOIN journal_entries je ON je.id = jl.journal_entry_id 
			AND je.status = 'POSTED' 
			AND je.deleted_at IS NULL
			AND je.entry_date <= ?
	WHERE COALESCE(a.is_header, false) = false
	  AND UPPER(a.type) IN ('ASSET', 'LIABILITY', 'EQUITY')
	  AND a.deleted_at IS NULL
	GROUP BY a.code, UPPER(a.type)
	HAVING COALESCE(SUM(jl.debit_amount), 0) <> 0
		    OR COALESCE(SUM(jl.credit_amount), 0) <> 0
		ORDER BY a.code
	`

	if err := s.db.Raw(legacyQuery, asOfDate).Scan(&balances).Error; err != nil {
		return nil, fmt.Errorf("legacy account balances query failed: %v", err)
	}

	// If no balances found, try to get all accounts with their direct balances (deduplicated)
	if len(balances) == 0 {
		log.Printf("No legacy balances found, trying direct account balances")
		allAccountsQuery := `
			SELECT 
				MIN(a.id) as account_id,
				a.code as account_code,
				MAX(a.name) as account_name,
				UPPER(a.type) as account_type,
				0 as debit_total,
				0 as credit_total,
				SUM(COALESCE(a.balance, 0)) as net_balance
		FROM accounts a
		WHERE COALESCE(a.is_header, false) = false
		  AND UPPER(a.type) IN ('ASSET', 'LIABILITY', 'EQUITY')
		  AND a.deleted_at IS NULL
		GROUP BY a.code, UPPER(a.type)
		HAVING SUM(COALESCE(a.balance, 0)) <> 0
			ORDER BY a.code
		`
		if err := s.db.Raw(allAccountsQuery).Scan(&balances).Error; err != nil {
			return nil, fmt.Errorf("error executing all accounts query: %v", err)
		}
		log.Printf("Retrieved %d accounts with direct balances (legacy fallback, deduplicated)", len(balances))
	}

	return balances, nil
}

// calculateNetIncome calculates net income (Revenue - Expenses) from SSOT journal system
func (s *SSOTBalanceSheetService) calculateNetIncome(asOfDate string) float64 {
	// NOTE: Scanning a single aggregate value into a basic type with GORM Scan can be unreliable.
	// To be robust, scan into a struct with a named column and then read the field.
	type niRow struct {
		NetIncome float64 `gorm:"column:net_income"`
	}
	var row niRow
	var netIncome float64

	log.Printf("calculateNetIncome START - AsOfDate: %s", asOfDate)

	asOf, err := time.Parse("2006-01-02", asOfDate)
	if err != nil {
		log.Printf("Failed to parse asOfDate for net income window: %v", err)
		return 0
	}

	currentWindowStart := s.currentEarningsStartDate(asOf)
	queryArgs := []interface{}{asOfDate}
	windowFilter := ""
	if currentWindowStart != nil {
		windowFilter = "AND DATE(uje.entry_date) >= ?"
		queryArgs = append(queryArgs, currentWindowStart.Format("2006-01-02"))
		log.Printf("calculateNetIncome current earnings window: %s to %s", currentWindowStart.Format("2006-01-02"), asOfDate)
	} else {
		log.Printf("calculateNetIncome current earnings window: beginning to %s", asOfDate)
	}

	// Helper to check for account types including localized names
	// Revenue: REVENUE, PENDAPATAN%
	// Expense: EXPENSE, BEBAN%

	// Use ILIKE for case-insensitive matching.
	// NOTE: is_header filter is kept to exclude header accounts from direct balance calculation.
	// Header accounts should have their balances rolled up into child accounts.
	// If header accounts have orphan balances, those need to be fixed in the chart of accounts.
	// Net Income = Revenue - Expenses
	// Revenue (credit normal): credit_amount - debit_amount = positive income
	// Expense (debit normal): SUBTRACT from income, so use: -(debit_amount - credit_amount) = credit_amount - debit_amount
	// Both use (credit - debit), so we can simplify:
	//   Revenue accounts contribute positive (credit > debit)
	//   Expense accounts contribute negative (debit > credit, so credit - debit is negative)
	query := strings.Replace(`
		SELECT 
			COALESCE(SUM(
				CASE 
					WHEN TRIM(a.type) ILIKE 'REVENUE' OR TRIM(a.type) ILIKE 'PENDAPATAN%' THEN 
						COALESCE(ujl.credit_amount, 0) - COALESCE(ujl.debit_amount, 0)
					WHEN TRIM(a.type) ILIKE 'EXPENSE' OR TRIM(a.type) ILIKE 'BEBAN%' THEN 
						COALESCE(ujl.credit_amount, 0) - COALESCE(ujl.debit_amount, 0)
					ELSE 0
				END
			), 0) as net_income
		FROM accounts a
		INNER JOIN unified_journal_lines ujl ON ujl.account_id = a.id
		INNER JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
		WHERE uje.status = 'POSTED' AND uje.deleted_at IS NULL
		AND DATE(uje.entry_date) <= ?
		__WINDOW_FILTER__
		AND UPPER(COALESCE(uje.source_type, '')) NOT IN ('CLOSING', 'REVERSAL')
	AND COALESCE(a.is_header, false) = false
	AND (
		TRIM(a.type) ILIKE 'REVENUE' OR TRIM(a.type) ILIKE 'PENDAPATAN%' OR 
		TRIM(a.type) ILIKE 'EXPENSE' OR TRIM(a.type) ILIKE 'BEBAN%'
	)
	AND a.deleted_at IS NULL
	`, "__WINDOW_FILTER__", windowFilter, 1)

	if err := s.db.Raw(query, queryArgs...).Scan(&row).Error; err == nil {
		netIncome = row.NetIncome
		log.Printf("Net Income calculated from SSOT: %.2f", netIncome)
	} else {
		log.Printf("Failed to get net income from SSOT, checking fallbacks: %v", err)
	}

	// If SSOT returned zero and tables might be empty, try legacy journals fallback
	if netIncome == 0 {
		var legacy niRow
		legacyQuery := strings.Replace(`
			SELECT 
				COALESCE(SUM(
					CASE 
						WHEN TRIM(a.type) ILIKE 'REVENUE' OR TRIM(a.type) ILIKE 'PENDAPATAN%' THEN 
							COALESCE(jl.credit_amount, 0) - COALESCE(jl.debit_amount, 0)
						WHEN TRIM(a.type) ILIKE 'EXPENSE' OR TRIM(a.type) ILIKE 'BEBAN%' THEN 
							COALESCE(jl.credit_amount, 0) - COALESCE(jl.debit_amount, 0)
						ELSE 0
					END
				), 0) as net_income
			FROM accounts a
			INNER JOIN journal_lines jl ON jl.account_id = a.id
			INNER JOIN journal_entries je ON je.id = jl.journal_entry_id
			WHERE je.status = 'POSTED' AND je.deleted_at IS NULL
			AND je.entry_date <= ?
			__WINDOW_FILTER__
			AND COALESCE(a.is_header, false) = false
			AND (
				TRIM(a.type) ILIKE 'REVENUE' OR TRIM(a.type) ILIKE 'PENDAPATAN%' OR 
				TRIM(a.type) ILIKE 'EXPENSE' OR TRIM(a.type) ILIKE 'BEBAN%'
			)
			AND a.deleted_at IS NULL`, "__WINDOW_FILTER__", strings.ReplaceAll(windowFilter, "DATE(uje.entry_date)", "DATE(je.entry_date)"), 1)
		if err := s.db.Raw(legacyQuery, queryArgs...).Scan(&legacy).Error; err == nil {
			netIncome = legacy.NetIncome
			log.Printf("Net Income from legacy journals: %.2f", netIncome)
		} else {
			// Fallback to accounts table balances as last resort
			var revenue, expense float64
			if err := s.db.Raw(`SELECT COALESCE(SUM(balance), 0) FROM accounts WHERE (TRIM(type) ILIKE 'REVENUE' OR TRIM(type) ILIKE 'PENDAPATAN%') AND deleted_at IS NULL`).Scan(&revenue).Error; err != nil {
				log.Printf("Failed to scan fallback revenue: %v", err)
			}
			if err := s.db.Raw(`SELECT COALESCE(SUM(balance), 0) FROM accounts WHERE (TRIM(type) ILIKE 'EXPENSE' OR TRIM(type) ILIKE 'BEBAN%') AND deleted_at IS NULL`).Scan(&expense).Error; err != nil {
				log.Printf("Failed to scan fallback expense: %v", err)
			}
			netIncome = revenue - expense
			log.Printf("Net Income from fallback (Accounts Table) - Revenue: %.2f, Expense: %.2f, Net: %.2f", revenue, expense, netIncome)
		}
	}

	log.Printf("calculateNetIncome END - Final Result: %.2f", netIncome)
	return netIncome
}

// generateBalanceSheetFromBalances creates the Balance Sheet structure from account balances
func (s *SSOTBalanceSheetService) generateBalanceSheetFromBalances(balances []SSOTAccountBalance, asOf time.Time) *SSOTBalanceSheetData {
	// Get company information from settings
	settingsSvc := NewSettingsService(s.db)
	settings, err := settingsSvc.GetSettings()

	var companyInfo CompanyInfo
	if err == nil && settings != nil {
		// Use actual settings from database
		companyInfo = CompanyInfo{
			Name:    settings.CompanyName,
			Address: settings.CompanyAddress,
			Phone:   settings.CompanyPhone,
			Email:   settings.CompanyEmail,
		}
	} else {
		// Fallback to default values if settings cannot be retrieved
		companyInfo = CompanyInfo{
			Name:    "PT. Sistem Akuntansi Indonesia",
			Address: "Jl. Sudirman Kav. 45-46, Jakarta Pusat 10210, Indonesia",
			Phone:   "+62-21-5551234",
			Email:   "info@sistemakuntansi.co.id",
		}
	}

	bsData := &SSOTBalanceSheetData{
		Company:     companyInfo,
		AsOfDate:    asOf,
		Currency:    "IDR",
		Enhanced:    true,
		GeneratedAt: time.Now(),
	}

	// Initialize sections
	bsData.Assets.CurrentAssets.Items = []BSAccountItem{}
	bsData.Assets.NonCurrentAssets.Items = []BSAccountItem{}
	bsData.Liabilities.CurrentLiabilities.Items = []BSAccountItem{}
	bsData.Liabilities.NonCurrentLiabilities.Items = []BSAccountItem{}
	bsData.Equity.Items = []BSAccountItem{}
	bsData.AccountDetails = balances

	// Current year earnings must always be derived from journal activity up to the
	// requested as-of date. Posted closing entries already zero-out closed periods,
	// so historical closings should not force net income to zero for later dates.
	netIncome := s.calculateNetIncome(asOf.Format("2006-01-02"))
	log.Printf("Net income calculated up to %s: %.2f", asOf.Format("2006-01-02"), netIncome)

	// Process each account balance
	purchaseInputVATAccountID, salesOutputVATAccountID := s.loadConfiguredVATAccountIDs()

	for _, balance := range balances {
		code := balance.AccountCode
		amount := balance.NetBalance

		// For liability accounts, ensure the amount is positive for display
		// In accounting, liability accounts normally have credit balances (positive values)
		if strings.ToUpper(balance.AccountType) == "LIABILITY" {
			// Log liability accounts for debugging
			if amount != 0 || strings.Contains(strings.ToUpper(balance.AccountName), "UTANG") {
				log.Printf("Liability Account Found: %s - %s (Type: %s, Balance: %.2f)",
					code, balance.AccountName, balance.AccountType, amount)
			}

			// Log specific liability account for debugging
			if code == "2101" {
				log.Printf("Processing UTANG USAHA (2101): Amount=%.2f, Type=%s", amount, balance.AccountType)
			}

			// Removed negative liability inversion: negative liabilities represent valid overpaid debts (Debit > Credit)
			// and MUST remain negative so they properly reduce Total Liabilities, preserving the accounting equation.
		}

		item := BSAccountItem{
			AccountCode: balance.AccountCode,
			AccountName: balance.AccountName,
			Amount:      amount,
			AccountID:   balance.AccountID,
		}

		// Check if this is a PPN account — skip from categorization entirely
		// PPN accounts will be handled by the PPN netting logic below
		isPPNMasukan := isPurchaseInputVATAccount(balance.AccountID, code, item.AccountName, purchaseInputVATAccountID)
		isPPNKeluaran := isSalesOutputVATAccount(balance.AccountID, code, item.AccountName, salesOutputVATAccountID)
		if isPPNMasukan || isPPNKeluaran {
			log.Printf("Skipping PPN account from categorization: %s - %s (%.2f)", code, item.AccountName, amount)
			continue // Don't categorize PPN accounts — handled below
		}

		// Convert account type to uppercase for comparison
		accountType := strings.ToUpper(balance.AccountType)
		switch accountType {
		case "ASSET":
			s.categorizeAssetAccount(bsData, item, code)
		case "LIABILITY":
			s.categorizeLiabilityAccount(bsData, item, code)
		case "EQUITY":
			s.categorizeEquityAccount(bsData, item, code)
		default:
			// Try to categorize based on code prefixes as fallback
			if strings.HasPrefix(code, "1") {
				s.categorizeAssetAccount(bsData, item, code)
			} else if strings.HasPrefix(code, "2") {
				s.categorizeLiabilityAccount(bsData, item, code)
			} else if strings.HasPrefix(code, "3") {
				s.categorizeEquityAccount(bsData, item, code)
			}
		}
	}

	// === PPN NETTING ===
	// Calculate net PPN directly from raw balance data (NOT from categorized items)
	var rawPPNKeluaran, rawPPNMasukan float64
	for _, balance := range balances {
		code := balance.AccountCode
		if isSalesOutputVATAccount(balance.AccountID, code, balance.AccountName, salesOutputVATAccountID) {
			// PPN Keluaran: LIABILITY type, net_balance = credit - debit (positive means credit balance)
			rawPPNKeluaran += balance.NetBalance
		} else if isPurchaseInputVATAccount(balance.AccountID, code, balance.AccountName, purchaseInputVATAccountID) {
			// PPN Masukan may be configured as ASSET or legacy contra-liability; use its signed net balance as-is.
			rawPPNMasukan += balance.NetBalance
		}
	}

	// Net PPN = PPN Keluaran - PPN Masukan
	// PPN Keluaran normally has credit balance (+), while PPN Masukan can be:
	// - ASSET: positive debit balance
	// - legacy contra-liability: negative debit balance
	ppnMasukanForNetting := rawPPNMasukan
	if rawPPNMasukan < 0 {
		ppnMasukanForNetting = -rawPPNMasukan
	}
	netPPNValue := rawPPNKeluaran - ppnMasukanForNetting

	log.Printf("PPN Netting - Keluaran: %.2f, Masukan: %.2f, Net: %.2f",
		rawPPNKeluaran, ppnMasukanForNetting, netPPNValue)

	if netPPNValue > 0 {
		// PPN Kurang Bayar (owe more) → show as Liability
		ppnItem := BSAccountItem{
			AccountCode: "PPN_NET",
			AccountName: fmt.Sprintf("PPN Terutang (Keluaran %.0f - Masukan %.0f)",
				rawPPNKeluaran, ppnMasukanForNetting),
			Amount: netPPNValue,
		}
		bsData.Liabilities.CurrentLiabilities.Items = append(bsData.Liabilities.CurrentLiabilities.Items, ppnItem)
		bsData.Liabilities.CurrentLiabilities.TaxPayable += netPPNValue
		log.Printf("Added PPN_NET as LIABILITY (Kurang Bayar): %.2f", netPPNValue)
	} else if netPPNValue < 0 {
		// PPN Lebih Bayar (overpaid) → show as Asset (receivable)
		ppnAmount := -netPPNValue // Make positive for display
		ppnItem := BSAccountItem{
			AccountCode: "PPN_NET",
			AccountName: fmt.Sprintf("PPN Lebih Bayar (Masukan %.0f - Keluaran %.0f)",
				ppnMasukanForNetting, rawPPNKeluaran),
			Amount: ppnAmount,
		}
		bsData.Assets.CurrentAssets.Items = append(bsData.Assets.CurrentAssets.Items, ppnItem)
		bsData.Assets.CurrentAssets.OtherCurrentAssets += ppnAmount
		log.Printf("Added PPN_NET as ASSET (Lebih Bayar): %.2f", ppnAmount)
	}

	// Add Net Income to Equity ONLY if period has NOT been closed
	// Add Net Income to Equity if it exists
	// Ideally we should track "IsPeriodClosed" status, but "Current Year Earnings" is standard
	if netIncome != 0 {
		// Show Net Income / Current Year Earnings as separate line in Equity
		// Check if "Laba/Rugi Berjalan" or similar already exists in items to avoid dups
		exists := false
		for _, item := range bsData.Equity.Items {
			if item.AccountCode == "NET_INCOME" || strings.Contains(strings.ToLower(item.AccountName), "laba/rugi berjalan") {
				exists = true
				break
			}
		}

		if !exists {
			bsData.Equity.RetainedEarnings += netIncome
			netIncomeItem := BSAccountItem{
				AccountCode: "NET_INCOME",
				AccountName: "Laba/Rugi Berjalan",
				Amount:      netIncome,
			}
			bsData.Equity.Items = append(bsData.Equity.Items, netIncomeItem)
			log.Printf("Added Net Income to Equity: %.2f", netIncome)
		}
	}

	// Calculate totals and check balance
	s.calculateBalanceSheetTotals(bsData)

	// Log totals for debugging
	log.Printf("Balance Sheet Totals - Assets: %.2f, Liabilities: %.2f, Equity: %.2f",
		bsData.Assets.TotalAssets, bsData.Liabilities.TotalLiabilities, bsData.Equity.TotalEquity)

	return bsData
}

// netPPNAccounts is now a no-op since PPN accounts are handled directly in generateBalanceSheetFromBalances
func (s *SSOTBalanceSheetService) netPPNAccounts(bsData *SSOTBalanceSheetData) {
	// No-op: PPN netting is now done inline in generateBalanceSheetFromBalances
}

// addNetPPNToBalanceSheet is now a no-op since PPN_NET is added directly in generateBalanceSheetFromBalances
func (s *SSOTBalanceSheetService) addNetPPNToBalanceSheet(bsData *SSOTBalanceSheetData) {
	// No-op: PPN_NET is now added inline in generateBalanceSheetFromBalances
}

// categorizeAssetAccount categorizes asset accounts into current and non-current assets
// Only adds items with non-zero amounts to improve report clarity
func (s *SSOTBalanceSheetService) categorizeAssetAccount(bsData *SSOTBalanceSheetData, item BSAccountItem, code string) {
	// Skip zero-balance items for cleaner display
	if item.Amount == 0 {
		return
	}

	switch {
	// Current Assets (11xx)
	case strings.HasPrefix(code, "1101"): // Cash accounts (use 4-char prefix to avoid matching 1102, 1103, etc.)
		bsData.Assets.CurrentAssets.Cash += item.Amount
		bsData.Assets.CurrentAssets.Items = append(bsData.Assets.CurrentAssets.Items, item)

	case strings.HasPrefix(code, "1102"), strings.HasPrefix(code, "112"): // Accounts Receivable (1102-xxx)
		bsData.Assets.CurrentAssets.Receivables += item.Amount
		bsData.Assets.CurrentAssets.Items = append(bsData.Assets.CurrentAssets.Items, item)

	case strings.HasPrefix(code, "1103"), strings.HasPrefix(code, "113"), strings.HasPrefix(code, "130"): // Inventory (1103-xxx)
		bsData.Assets.CurrentAssets.Inventory += item.Amount
		bsData.Assets.CurrentAssets.Items = append(bsData.Assets.CurrentAssets.Items, item)

	case strings.HasPrefix(code, "1104"), strings.HasPrefix(code, "1105"), strings.HasPrefix(code, "114"), strings.HasPrefix(code, "115"): // Prepaid expenses
		bsData.Assets.CurrentAssets.PrepaidExpenses += item.Amount
		bsData.Assets.CurrentAssets.Items = append(bsData.Assets.CurrentAssets.Items, item)

	case strings.HasPrefix(code, "11"): // Other current assets
		bsData.Assets.CurrentAssets.OtherCurrentAssets += item.Amount
		bsData.Assets.CurrentAssets.Items = append(bsData.Assets.CurrentAssets.Items, item)

	// Special handling for PPN accounts using code-based matching
	case strings.HasPrefix(code, "2102-002"): // PPN Masukan — stays in assets as contra-liability
		bsData.Assets.CurrentAssets.OtherCurrentAssets += item.Amount
		bsData.Assets.CurrentAssets.Items = append(bsData.Assets.CurrentAssets.Items, item)

	// Non-Current Assets (12xx, 13xx, 14xx, 15xx)
	case strings.HasPrefix(code, "12"), strings.HasPrefix(code, "16"), strings.HasPrefix(code, "17"): // Fixed Assets
		bsData.Assets.NonCurrentAssets.FixedAssets += item.Amount
		bsData.Assets.NonCurrentAssets.Items = append(bsData.Assets.NonCurrentAssets.Items, item)

	case strings.HasPrefix(code, "14"): // Intangible Assets
		bsData.Assets.NonCurrentAssets.IntangibleAssets += item.Amount
		bsData.Assets.NonCurrentAssets.Items = append(bsData.Assets.NonCurrentAssets.Items, item)

	case strings.HasPrefix(code, "15"): // Investments
		bsData.Assets.NonCurrentAssets.Investments += item.Amount
		bsData.Assets.NonCurrentAssets.Items = append(bsData.Assets.NonCurrentAssets.Items, item)

	default: // Other non-current assets
		bsData.Assets.NonCurrentAssets.OtherNonCurrentAssets += item.Amount
		bsData.Assets.NonCurrentAssets.Items = append(bsData.Assets.NonCurrentAssets.Items, item)
	}
}

// categorizeLiabilityAccount categorizes liability accounts into current and non-current liabilities
// Only adds items with non-zero amounts to improve report clarity
func (s *SSOTBalanceSheetService) categorizeLiabilityAccount(bsData *SSOTBalanceSheetData, item BSAccountItem, code string) {
	// IMPORTANT: Don't convert PPN accounts here - they need original values for netting
	// Use code-based matching instead of name-based

	amount := item.Amount

	// Skip zero-balance items for cleaner display
	if amount == 0 {
		return
	}

	// Liability accounts that carry a debit balance should not be presented as
	// negative liabilities on the face of the balance sheet. Reclass them to the
	// asset side so the statement remains readable and aligned with accounting
	// presentation rules for advances/overpayments.
	if amount < 0 {
		reclassItem := item
		reclassItem.Amount = -amount
		reclassItem.AccountName = fmt.Sprintf("%s (Saldo Debit Direklasifikasi)", item.AccountName)

		switch {
		case strings.HasPrefix(code, "22"), strings.HasPrefix(code, "23"):
			bsData.Assets.NonCurrentAssets.OtherNonCurrentAssets += reclassItem.Amount
			bsData.Assets.NonCurrentAssets.Items = append(bsData.Assets.NonCurrentAssets.Items, reclassItem)
		default:
			bsData.Assets.CurrentAssets.OtherCurrentAssets += reclassItem.Amount
			bsData.Assets.CurrentAssets.Items = append(bsData.Assets.CurrentAssets.Items, reclassItem)
		}

		log.Printf("Reclassified negative liability to asset: %s - %s (Amount: %.2f)",
			reclassItem.AccountCode, reclassItem.AccountName, reclassItem.Amount)
		return
	}

	// Log liability categorization for debugging
	log.Printf("Categorizing Liability: %s - %s (Amount: %.2f)",
		item.AccountCode, item.AccountName, amount)

	// Update the item amount (preserve original for PPN)
	item.Amount = amount

	switch {
	// Current Liabilities (21xx)
	case strings.HasPrefix(code, "210"): // Accounts Payable
		bsData.Liabilities.CurrentLiabilities.AccountsPayable += amount
		bsData.Liabilities.CurrentLiabilities.Items = append(bsData.Liabilities.CurrentLiabilities.Items, item)
		log.Printf("Added to AccountsPayable: %.2f", amount)

	case strings.HasPrefix(code, "211"): // Short-term debt
		bsData.Liabilities.CurrentLiabilities.ShortTermDebt += amount
		bsData.Liabilities.CurrentLiabilities.Items = append(bsData.Liabilities.CurrentLiabilities.Items, item)
		log.Printf("Added to ShortTermDebt: %.2f", amount)

	case strings.HasPrefix(code, "212"), strings.HasPrefix(code, "213"): // Accrued liabilities and taxes
		if strings.Contains(strings.ToLower(item.AccountName), "tax") || strings.Contains(strings.ToLower(item.AccountName), "pajak") || strings.Contains(strings.ToLower(item.AccountName), "pph") {
			bsData.Liabilities.CurrentLiabilities.TaxPayable += amount
			log.Printf("Added to TaxPayable: %.2f", amount)
		} else {
			bsData.Liabilities.CurrentLiabilities.AccruedLiabilities += amount
			log.Printf("Added to AccruedLiabilities: %.2f", amount)
		}
		bsData.Liabilities.CurrentLiabilities.Items = append(bsData.Liabilities.CurrentLiabilities.Items, item)

	case strings.HasPrefix(code, "21"): // Other current liabilities
		bsData.Liabilities.CurrentLiabilities.OtherCurrentLiabilities += amount
		bsData.Liabilities.CurrentLiabilities.Items = append(bsData.Liabilities.CurrentLiabilities.Items, item)
		log.Printf("Added to OtherCurrentLiabilities: %.2f", amount)

	// Non-Current Liabilities (22xx, 23xx)
	case strings.HasPrefix(code, "22"): // Long-term debt
		bsData.Liabilities.NonCurrentLiabilities.LongTermDebt += amount
		bsData.Liabilities.NonCurrentLiabilities.Items = append(bsData.Liabilities.NonCurrentLiabilities.Items, item)
		log.Printf("Added to LongTermDebt: %.2f", amount)

	case strings.HasPrefix(code, "23"): // Other non-current liabilities
		if strings.Contains(strings.ToLower(item.AccountName), "tax") || strings.Contains(strings.ToLower(item.AccountName), "pajak") {
			bsData.Liabilities.NonCurrentLiabilities.DeferredTax += amount
			log.Printf("Added to DeferredTax: %.2f", amount)
		} else {
			bsData.Liabilities.NonCurrentLiabilities.OtherNonCurrentLiabilities += amount
			log.Printf("Added to OtherNonCurrentLiabilities: %.2f", amount)
		}
		bsData.Liabilities.NonCurrentLiabilities.Items = append(bsData.Liabilities.NonCurrentLiabilities.Items, item)

	default: // Other liabilities
		bsData.Liabilities.NonCurrentLiabilities.OtherNonCurrentLiabilities += amount
		bsData.Liabilities.NonCurrentLiabilities.Items = append(bsData.Liabilities.NonCurrentLiabilities.Items, item)
		log.Printf("Added to OtherNonCurrentLiabilities (default): %.2f", amount)
	}
}

// categorizeEquityAccount categorizes equity accounts
// Only adds items with non-zero amounts to improve report clarity
func (s *SSOTBalanceSheetService) categorizeEquityAccount(bsData *SSOTBalanceSheetData, item BSAccountItem, code string) {
	// Skip zero-balance items for cleaner display
	if item.Amount == 0 {
		return
	}

	switch {
	case strings.HasPrefix(code, "31"): // Share Capital
		bsData.Equity.ShareCapital += item.Amount
		bsData.Equity.Items = append(bsData.Equity.Items, item)

	case strings.HasPrefix(code, "32"): // Retained Earnings
		bsData.Equity.RetainedEarnings += item.Amount
		bsData.Equity.Items = append(bsData.Equity.Items, item)

	default: // Other Equity
		bsData.Equity.OtherEquity += item.Amount
		bsData.Equity.Items = append(bsData.Equity.Items, item)
	}
}

// calculateBalanceSheetTotals calculates all totals and checks if the balance sheet is balanced
func (s *SSOTBalanceSheetService) calculateBalanceSheetTotals(bsData *SSOTBalanceSheetData) {
	// Calculate current assets total
	bsData.Assets.CurrentAssets.TotalCurrentAssets =
		bsData.Assets.CurrentAssets.Cash +
			bsData.Assets.CurrentAssets.Receivables +
			bsData.Assets.CurrentAssets.Inventory +
			bsData.Assets.CurrentAssets.PrepaidExpenses +
			bsData.Assets.CurrentAssets.OtherCurrentAssets

	// Calculate non-current assets total
	bsData.Assets.NonCurrentAssets.TotalNonCurrentAssets =
		bsData.Assets.NonCurrentAssets.FixedAssets +
			bsData.Assets.NonCurrentAssets.IntangibleAssets +
			bsData.Assets.NonCurrentAssets.Investments +
			bsData.Assets.NonCurrentAssets.OtherNonCurrentAssets

	// Calculate total assets
	bsData.Assets.TotalAssets =
		bsData.Assets.CurrentAssets.TotalCurrentAssets +
			bsData.Assets.NonCurrentAssets.TotalNonCurrentAssets

	// Log assets calculation
	log.Printf("Assets Calculation - Current: %.2f, Non-Current: %.2f, Total: %.2f",
		bsData.Assets.CurrentAssets.TotalCurrentAssets,
		bsData.Assets.NonCurrentAssets.TotalNonCurrentAssets,
		bsData.Assets.TotalAssets)

	// Calculate current liabilities total
	bsData.Liabilities.CurrentLiabilities.TotalCurrentLiabilities =
		bsData.Liabilities.CurrentLiabilities.AccountsPayable +
			bsData.Liabilities.CurrentLiabilities.ShortTermDebt +
			bsData.Liabilities.CurrentLiabilities.AccruedLiabilities +
			bsData.Liabilities.CurrentLiabilities.TaxPayable +
			bsData.Liabilities.CurrentLiabilities.OtherCurrentLiabilities

	// Log liabilities calculation
	log.Printf("Liabilities Calculation - AccountsPayable: %.2f, Other: %.2f, Total: %.2f",
		bsData.Liabilities.CurrentLiabilities.AccountsPayable,
		bsData.Liabilities.CurrentLiabilities.ShortTermDebt+
			bsData.Liabilities.CurrentLiabilities.AccruedLiabilities+
			bsData.Liabilities.CurrentLiabilities.TaxPayable+
			bsData.Liabilities.CurrentLiabilities.OtherCurrentLiabilities,
		bsData.Liabilities.CurrentLiabilities.TotalCurrentLiabilities)

	// Calculate total liabilities
	bsData.Liabilities.TotalLiabilities =
		bsData.Liabilities.CurrentLiabilities.TotalCurrentLiabilities +
			bsData.Liabilities.NonCurrentLiabilities.TotalNonCurrentLiabilities

	// Log liabilities calculation
	log.Printf("Liabilities Calculation - Current: %.2f, Non-Current: %.2f, Total: %.2f",
		bsData.Liabilities.CurrentLiabilities.TotalCurrentLiabilities,
		bsData.Liabilities.NonCurrentLiabilities.TotalNonCurrentLiabilities,
		bsData.Liabilities.TotalLiabilities)

	// Calculate total equity
	bsData.Equity.TotalEquity =
		bsData.Equity.ShareCapital +
			bsData.Equity.RetainedEarnings +
			bsData.Equity.OtherEquity

	// Sync Total Equity with sum of Equity.Items
	var actualTotalEquity float64
	for _, item := range bsData.Equity.Items {
		actualTotalEquity += item.Amount
	}
	bsData.Equity.TotalEquity = actualTotalEquity

	// Log equity calculation
	log.Printf("Equity Calculation - ShareCapital: %.2f, RetainedEarnings: %.2f, OtherEquity: %.2f, Total: %.2f",
		bsData.Equity.ShareCapital,
		bsData.Equity.RetainedEarnings,
		bsData.Equity.OtherEquity,
		bsData.Equity.TotalEquity)

	// Calculate total liabilities and equity
	bsData.TotalLiabilitiesAndEquity = bsData.Liabilities.TotalLiabilities + bsData.Equity.TotalEquity

	// Check if balance sheet is balanced
	tolerance := 0.01 // 1 cent tolerance
	bsData.BalanceDifference = bsData.Assets.TotalAssets - bsData.TotalLiabilitiesAndEquity
	bsData.IsBalanced = (bsData.BalanceDifference >= -tolerance && bsData.BalanceDifference <= tolerance)

	// Log final balance check
	log.Printf("Balance Check - Assets: %.2f, Liabilities+Equity: %.2f, Difference: %.2f, Balanced: %t",
		bsData.Assets.TotalAssets,
		bsData.TotalLiabilitiesAndEquity,
		bsData.BalanceDifference,
		bsData.IsBalanced)
}

// calculateAccountBalanceFromTransactions calculates an account's balance from its transaction data
func (s *SSOTBalanceSheetService) calculateAccountBalanceFromTransactions(accountID uint, accountType string) (float64, error) {
	query := `
		SELECT
			CASE
				WHEN UPPER(?) IN ('ASSET', 'EXPENSE') THEN
					COALESCE(SUM(ujl.debit_amount), 0) - COALESCE(SUM(ujl.credit_amount), 0)
				ELSE
					COALESCE(SUM(ujl.credit_amount), 0) - COALESCE(SUM(ujl.debit_amount), 0)
			END as net_balance
		FROM accounts a
		INNER JOIN unified_journal_lines ujl ON ujl.account_id = a.id
		INNER JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id AND uje.status = 'POSTED' AND uje.deleted_at IS NULL
		WHERE a.id = ?
		  AND COALESCE(a.is_header, false) = false
		GROUP BY a.type
	`

	type result struct {
		NetBalance float64 `gorm:"column:net_balance"`
	}

	var row result
	if err := s.db.Raw(query, accountType, accountID).Scan(&row).Error; err != nil {
		return 0, fmt.Errorf("error calculating account balance from transactions: %v", err)
	}

	return row.NetBalance, nil
}

// calculateAccountBalanceFromChildren calculates an account's balance from its child accounts
func (s *SSOTBalanceSheetService) calculateAccountBalanceFromChildren(accountID uint, accountType string) (float64, error) {
	// First, get direct children
	query := `
		SELECT COALESCE(SUM(
			CASE 
				WHEN UPPER(?) IN ('ASSET', 'EXPENSE') THEN 
					-- For assets and expenses, debit increases balance
					COALESCE(balance, 0)
				ELSE 
					-- For liabilities, equity, and revenue, credit increases balance
					COALESCE(balance, 0)
			END
		), 0) as total_balance
		FROM accounts 
		WHERE parent_id = ?
		  AND COALESCE(is_header, false) = false
	`

	type result struct {
		TotalBalance float64 `gorm:"column:total_balance"`
	}

	var row result
	if err := s.db.Raw(query, accountType, accountID).Scan(&row).Error; err != nil {
		return 0, fmt.Errorf("error calculating account balance from children: %v", err)
	}

	return row.TotalBalance, nil
}

// removePPNAccountsFromDisplay is now a no-op since PPN accounts are never added individually
func (s *SSOTBalanceSheetService) removePPNAccountsFromDisplay(bsData *SSOTBalanceSheetData) {
	// No-op: PPN accounts are skipped during categorization
	// Only PPN_NET is added, so there's nothing to remove
}
