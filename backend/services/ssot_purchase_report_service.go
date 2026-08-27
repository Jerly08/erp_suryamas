package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

// SSOTPurchaseReportService generates purchase reports from SSOT journal data
type SSOTPurchaseReportService struct {
	db *gorm.DB
}

// NewSSOTPurchaseReportService creates a new SSOT purchase report service
func NewSSOTPurchaseReportService(db *gorm.DB) *SSOTPurchaseReportService {
	return &SSOTPurchaseReportService{
		db: db,
	}
}

// PurchaseReportData represents comprehensive purchase analysis
type PurchaseReportData struct {
	Company             CompanyInfo               `json:"company"`
	StartDate           time.Time                 `json:"start_date"`
	EndDate             time.Time                 `json:"end_date"`
	Currency            string                    `json:"currency"`
	TotalPurchases      int64                     `json:"total_purchases"`
	CompletedPurchases  int64                     `json:"completed_purchases"`
	TotalAmount         float64                   `json:"total_amount"`
	TotalPaid           float64                   `json:"total_paid"`
	OutstandingPayables float64                   `json:"outstanding_payables"`
	PurchasesByVendor   []VendorPurchaseSummary   `json:"purchases_by_vendor"`
	PurchasesByMonth    []MonthlyPurchaseSummary  `json:"purchases_by_month"`
	PurchasesByCategory []CategoryPurchaseSummary `json:"purchases_by_category"`
	PaymentAnalysis     PurchasePaymentAnalysis   `json:"payment_analysis"`
	TaxAnalysis         PurchaseTaxAnalysis       `json:"tax_analysis"`
	GeneratedAt         time.Time                 `json:"generated_at"`
}

// VendorPurchaseSummary represents purchase summary by vendor
type VendorPurchaseSummary struct {
	VendorID         uint64               `json:"vendor_id"`
	VendorName       string               `json:"vendor_name"`
	TotalPurchases   int64                `json:"total_purchases"`
	TotalAmount      float64              `json:"total_amount"`
	TotalPaid        float64              `json:"total_paid"`
	Outstanding      float64              `json:"outstanding"`
	LastPurchaseDate time.Time            `json:"last_purchase_date"`
	PaymentMethod    string               `json:"payment_method"`
	Status           string               `json:"status"`
	Items            []PurchaseItemDetail `json:"items,omitempty"`
}

// PurchaseItemDetail represents individual item purchased
type PurchaseItemDetail struct {
	ProductID     uint64    `json:"product_id"`
	ProductCode   string    `json:"product_code"`
	ProductName   string    `json:"product_name"`
	Quantity      float64   `json:"quantity"`
	UnitPrice     float64   `json:"unit_price"`
	TotalPrice    float64   `json:"total_price"`
	Unit          string    `json:"unit"`
	PurchaseDate  time.Time `json:"purchase_date"`
	InvoiceNumber string    `json:"invoice_number,omitempty"`
}

// MonthlyPurchaseSummary represents purchase summary by month
type MonthlyPurchaseSummary struct {
	Year           int     `json:"year"`
	Month          int     `json:"month"`
	MonthName      string  `json:"month_name"`
	TotalPurchases int64   `json:"total_purchases"`
	TotalAmount    float64 `json:"total_amount"`
	TotalPaid      float64 `json:"total_paid"`
	AverageAmount  float64 `json:"average_amount"`
}

// CategoryPurchaseSummary represents purchase summary by category
type CategoryPurchaseSummary struct {
	CategoryName   string  `json:"category_name"`
	AccountCode    string  `json:"account_code"`
	AccountName    string  `json:"account_name"`
	TotalPurchases int64   `json:"total_purchases"`
	TotalAmount    float64 `json:"total_amount"`
	Percentage     float64 `json:"percentage"`
}

// PurchasePaymentAnalysis represents payment pattern analysis
type PurchasePaymentAnalysis struct {
	CashPurchases     int64   `json:"cash_purchases"`
	CreditPurchases   int64   `json:"credit_purchases"`
	CashAmount        float64 `json:"cash_amount"`
	CreditAmount      float64 `json:"credit_amount"`
	CashPercentage    float64 `json:"cash_percentage"`
	CreditPercentage  float64 `json:"credit_percentage"`
	AverageOrderValue float64 `json:"average_order_value"`
}

// PurchaseTaxAnalysis represents tax analysis for purchases
type PurchaseTaxAnalysis struct {
	TotalTaxableAmount   float64             `json:"total_taxable_amount"`
	TotalTaxAmount       float64             `json:"total_tax_amount"`
	AverageTaxRate       float64             `json:"average_tax_rate"`
	TaxReclaimableAmount float64             `json:"tax_reclaimable_amount"`
	TaxByMonth           []MonthlyTaxSummary `json:"tax_by_month"`
}

// MonthlyTaxSummary represents tax summary by month
type MonthlyTaxSummary struct {
	Year      int     `json:"year"`
	Month     int     `json:"month"`
	MonthName string  `json:"month_name"`
	TaxAmount float64 `json:"tax_amount"`
}

// GeneratePurchaseReport generates comprehensive purchase report from SSOT data
func (s *SSOTPurchaseReportService) GeneratePurchaseReport(ctx context.Context, startDate, endDate time.Time, projectID *uint) (*PurchaseReportData, error) {
	result := &PurchaseReportData{
		Company:     s.getCompanyInfo(),
		StartDate:   startDate,
		EndDate:     endDate,
		Currency:    s.getCurrencyFromSettings(),
		GeneratedAt: time.Now(),
	}

	// Get purchase transactions from SSOT journal
	purchaseSummary, err := s.getPurchaseSummary(ctx, startDate, endDate, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get purchase summary: %w", err)
	}

	// Populate basic statistics
	result.TotalPurchases = purchaseSummary.TotalCount
	result.CompletedPurchases = purchaseSummary.CompletedCount
	result.TotalAmount = purchaseSummary.TotalAmount
	result.TotalPaid = purchaseSummary.TotalPaid
	result.OutstandingPayables = purchaseSummary.TotalAmount - purchaseSummary.TotalPaid

	// Get detailed analyses
	result.PurchasesByVendor, err = s.getPurchasesByVendor(ctx, startDate, endDate, projectID)
	if err != nil {
		log.Printf("Error getting purchases by vendor: %v", err)
		result.PurchasesByVendor = []VendorPurchaseSummary{}
	}

	result.PurchasesByMonth, err = s.getPurchasesByMonth(ctx, startDate, endDate, projectID)
	if err != nil {
		log.Printf("Error getting purchases by month: %v", err)
		result.PurchasesByMonth = []MonthlyPurchaseSummary{}
	}

	result.PurchasesByCategory, err = s.getPurchasesByCategory(ctx, startDate, endDate, projectID)
	if err != nil {
		log.Printf("Error getting purchases by category: %v", err)
		result.PurchasesByCategory = []CategoryPurchaseSummary{}
	}

	result.PaymentAnalysis, err = s.getPaymentAnalysis(ctx, startDate, endDate, projectID)
	if err != nil {
		log.Printf("Error getting payment analysis: %v", err)
		result.PaymentAnalysis = PurchasePaymentAnalysis{}
	}

	result.TaxAnalysis, err = s.getTaxAnalysis(ctx, startDate, endDate, projectID)
	if err != nil {
		log.Printf("Error getting tax analysis: %v", err)
		result.TaxAnalysis = PurchaseTaxAnalysis{}
	}

	return result, nil
}

// Helper struct for purchase summary
type purchaseBaseSummary struct {
	TotalCount     int64
	CompletedCount int64
	TotalAmount    float64
	TotalPaid      float64
}

// getPurchaseSummary gets basic purchase statistics from purchases table (primary source)
func (s *SSOTPurchaseReportService) getPurchaseSummary(ctx context.Context, startDate, endDate time.Time, projectID *uint) (*purchaseBaseSummary, error) {
	// Use purchases table paid_amount field which is accurately maintained by payment system
	query := fmt.Sprintf(`
		SELECT 
			COUNT(p.id) as total_count,
			COUNT(CASE WHEN COALESCE(p.outstanding_amount, 0) <= 0.01 THEN 1 END) as completed_count,
			COALESCE(SUM(p.total_amount), 0) as total_amount,
			COALESCE(SUM(p.paid_amount), 0) as total_paid
		FROM purchases p
		WHERE p.date BETWEEN ? AND ?
		  AND p.deleted_at IS NULL
		  AND %s
	`, models.PurchaseOperationalReportWhereClause("p"))

	args := []interface{}{startDate, endDate}
	if projectID != nil {
		query += " AND p.project_id = ?"
		args = append(args, *projectID)
	}

	var summary purchaseBaseSummary
	err := s.db.WithContext(ctx).Raw(query, args...).Scan(&summary).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query purchase summary: %w", err)
	}

	return &summary, nil
}

// getPurchasesByVendor gets purchase summary grouped by vendor - using purchases table as source
func (s *SSOTPurchaseReportService) getPurchasesByVendor(ctx context.Context, startDate, endDate time.Time, projectID *uint) ([]VendorPurchaseSummary, error) {
	// Use purchases table paid_amount field for accurate payment tracking
	query := fmt.Sprintf(`
		SELECT 
			p.vendor_id as vendor_id,
			COALESCE(c.name, 'Unknown Vendor') as vendor_name,
			COUNT(DISTINCT p.id) as total_purchases,
			COALESCE(SUM(p.total_amount), 0) as total_amount,
			COALESCE(SUM(p.paid_amount), 0) as total_paid,
			MAX(p.date) as last_purchase_date,
			CASE 
				WHEN bool_or(p.payment_method IN ('CASH', 'BANK_TRANSFER'))
				THEN 'CASH'
				ELSE 'CREDIT'
			END as payment_method,
			CASE 
				WHEN COALESCE(SUM(p.outstanding_amount), 0) <= 0.01
				THEN 'PAID'
				WHEN COALESCE(SUM(p.paid_amount), 0) > 0
				THEN 'PARTIAL'
				ELSE 'UNPAID'
			END as status,
			string_agg(DISTINCT p.code, ', ') as descriptions
		FROM purchases p
		LEFT JOIN contacts c ON c.id = p.vendor_id
		WHERE p.date BETWEEN ? AND ?
		  AND p.deleted_at IS NULL
		  AND %s
	`, models.PurchaseOperationalReportWhereClause("p"))

	args := []interface{}{startDate, endDate}
	if projectID != nil {
		query += " AND p.project_id = ?"
		args = append(args, *projectID)
	}
	query += `
		GROUP BY p.vendor_id, c.name
		ORDER BY total_amount DESC
	`

	var vendors []struct {
		VendorID         uint64    `json:"vendor_id"`
		VendorName       string    `json:"vendor_name"`
		TotalPurchases   int64     `json:"total_purchases"`
		TotalAmount      float64   `json:"total_amount"`
		TotalPaid        float64   `json:"total_paid"`
		LastPurchaseDate time.Time `json:"last_purchase_date"`
		PaymentMethod    string    `json:"payment_method"`
		Status           string    `json:"status"`
		Descriptions     string    `json:"descriptions"`
	}

	err := s.db.WithContext(ctx).Raw(query, args...).Scan(&vendors).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query purchases by vendor: %w", err)
	}

	// Add logging for debugging
	log.Printf("Found %d vendor groups from SSOT journal", len(vendors))
	for i, v := range vendors {
		log.Printf("Vendor %d: ID=%v, Name=%s, Purchases=%d, Amount=%.2f, Descriptions=%s",
			i+1, v.VendorID, v.VendorName, v.TotalPurchases, v.TotalAmount, v.Descriptions)
	}

	// Convert to result format
	var result []VendorPurchaseSummary
	validVendorCount := 0

	for _, v := range vendors {
		// Skip vendors with no purchases
		if v.TotalAmount <= 0 {
			continue
		}

		if v.VendorName != "Unknown Vendor" {
			validVendorCount++
		}

		summary := VendorPurchaseSummary{
			VendorID:         v.VendorID,
			VendorName:       v.VendorName,
			TotalPurchases:   v.TotalPurchases,
			TotalAmount:      v.TotalAmount,
			TotalPaid:        v.TotalPaid,
			Outstanding:      v.TotalAmount - v.TotalPaid,
			LastPurchaseDate: v.LastPurchaseDate,
			PaymentMethod:    v.PaymentMethod,
			Status:           v.Status,
		}

		result = append(result, summary)
	}

	log.Printf("Valid vendors found: %d out of %d total groups", validVendorCount, len(vendors))

	// Fetch items for each vendor using vendor_id to avoid mixing contacts with the same name
	for i := range result {
		if result[i].VendorID == 0 {
			continue
		}

		items, err := s.getPurchaseItemsFromSSOT(ctx, startDate, endDate, result[i].VendorID, projectID)
		if err != nil {
			log.Printf("Warning: Failed to get items for vendor %s: %v", result[i].VendorName, err)
			continue
		}
		result[i].Items = items
		if len(items) > 0 {
			log.Printf("✅ Loaded %d items for vendor: %s", len(items), result[i].VendorName)
		}
	}

	return result, nil
}

// getPurchaseItemsFromSSOT gets detailed items purchased - using purchases table directly.
func (s *SSOTPurchaseReportService) getPurchaseItemsFromSSOT(ctx context.Context, startDate, endDate time.Time, vendorID uint64, projectID *uint) ([]PurchaseItemDetail, error) {
	query := fmt.Sprintf(`
		SELECT 
			COALESCE(pi.product_id, 0) as product_id,
			COALESCE(prod.code, 'N/A') as product_code,
			COALESCE(prod.name, 'Unknown Product') as product_name,
			pi.quantity,
			pi.unit_price,
			pi.total_price,
			COALESCE(prod.unit, 'pcs') as unit,
			pur.date as purchase_date,
			COALESCE(pur.code, '') as invoice_number
		FROM purchases pur
		INNER JOIN purchase_items pi ON pi.purchase_id = pur.id
		INNER JOIN contacts v ON v.id = pur.vendor_id
		LEFT JOIN products prod ON prod.id = pi.product_id
		WHERE pur.date BETWEEN ? AND ?
		  AND pur.deleted_at IS NULL
		  AND pi.deleted_at IS NULL
		  AND pur.vendor_id = ?
		  AND %s
	`, models.PurchaseOperationalReportWhereClause("pur"))

	args := []interface{}{startDate, endDate, vendorID}
	if projectID != nil {
		query += " AND pur.project_id = ?"
		args = append(args, *projectID)
	}
	query += `
		ORDER BY pur.date DESC, pi.id
	`

	var items []PurchaseItemDetail
	err := s.db.WithContext(ctx).Raw(query, args...).Scan(&items).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query purchase items from SSOT: %w", err)
	}

	log.Printf("Loaded %d purchase report items for vendor_id=%d", len(items), vendorID)
	return items, nil
}

// getPurchasesByMonth gets purchase summary grouped by month - using purchases table
func (s *SSOTPurchaseReportService) getPurchasesByMonth(ctx context.Context, startDate, endDate time.Time, projectID *uint) ([]MonthlyPurchaseSummary, error) {
	// Use purchases table paid_amount field for accurate payment tracking
	query := fmt.Sprintf(`
		SELECT 
			EXTRACT(YEAR FROM p.date) as year,
			EXTRACT(MONTH FROM p.date) as month,
			COUNT(DISTINCT p.id) as total_purchases,
			COALESCE(SUM(p.total_amount), 0) as total_amount,
			COALESCE(SUM(p.paid_amount), 0) as total_paid
		FROM purchases p
		WHERE p.date BETWEEN ? AND ?
		  AND p.deleted_at IS NULL
		  AND %s
	`, models.PurchaseOperationalReportWhereClause("p"))

	args := []interface{}{startDate, endDate}
	if projectID != nil {
		query += " AND p.project_id = ?"
		args = append(args, *projectID)
	}
	query += `
		GROUP BY EXTRACT(YEAR FROM p.date), EXTRACT(MONTH FROM p.date)
		ORDER BY year, month
	`

	var months []struct {
		Year           int     `json:"year"`
		Month          int     `json:"month"`
		TotalPurchases int64   `json:"total_purchases"`
		TotalAmount    float64 `json:"total_amount"`
		TotalPaid      float64 `json:"total_paid"`
	}

	err := s.db.WithContext(ctx).Raw(query, args...).Scan(&months).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query purchases by month: %w", err)
	}

	// Convert to result format with month names
	monthNames := []string{
		"", "January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December",
	}

	var result []MonthlyPurchaseSummary
	for _, m := range months {
		avgAmount := float64(0)
		if m.TotalPurchases > 0 {
			avgAmount = m.TotalAmount / float64(m.TotalPurchases)
		}

		summary := MonthlyPurchaseSummary{
			Year:           m.Year,
			Month:          m.Month,
			MonthName:      monthNames[m.Month],
			TotalPurchases: m.TotalPurchases,
			TotalAmount:    m.TotalAmount,
			TotalPaid:      m.TotalPaid,
			AverageAmount:  avgAmount,
		}

		result = append(result, summary)
	}

	return result, nil
}

// getPurchasesByCategory gets purchase summary grouped by posted purchase journal debits.
// This keeps mixed-category purchases split by their actual journal allocation instead of
// forcing the whole purchase into a single debit account bucket.
func (s *SSOTPurchaseReportService) getPurchasesByCategory(ctx context.Context, startDate, endDate time.Time, projectID *uint) ([]CategoryPurchaseSummary, error) {
	projectFilter := ""
	args := []interface{}{models.SSOTStatusPosted, startDate, endDate}
	if projectID != nil {
		projectFilter = " AND p.project_id = ?"
		args = append(args, *projectID)
	}
	args = append(args, startDate, endDate)
	if projectID != nil {
		args = append(args, *projectID)
	}

	query := fmt.Sprintf(`
		WITH categorized_lines AS (
			SELECT
				p.id as purchase_id,
				COALESCE(a.code, '1301') as account_code,
				COALESCE(a.name, 'Uncategorized Purchase') as account_name,
				CASE
					WHEN UPPER(TRIM(COALESCE(a.category, ''))) = 'CURRENT_ASSET' THEN 'Current Assets'
					WHEN UPPER(TRIM(COALESCE(a.category, ''))) = 'FIXED_ASSET' THEN 'Fixed Assets'
					WHEN UPPER(TRIM(COALESCE(a.category, ''))) = 'INTANGIBLE_ASSET' THEN 'Intangible Assets'
					WHEN UPPER(TRIM(COALESCE(a.category, ''))) = 'INVESTMENT_ASSET' THEN 'Investment Assets'
					WHEN UPPER(TRIM(COALESCE(a.type, ''))) = 'EXPENSE' THEN 'Expenses'
					WHEN UPPER(TRIM(COALESCE(a.type, ''))) = 'ASSET' THEN 'Assets'
					WHEN UPPER(TRIM(COALESCE(a.type, ''))) = 'LIABILITY' THEN 'Liabilities'
					ELSE 'Other'
				END as category_name,
				sjl.debit_amount as amount
			FROM purchases p
			JOIN unified_journal_ledger sje
				ON sje.source_id = p.id
				AND sje.source_type = 'PURCHASE'
				AND sje.status = ?
				AND sje.deleted_at IS NULL
			JOIN unified_journal_lines sjl
				ON sjl.journal_id = sje.id
				AND sjl.debit_amount > 0
			JOIN accounts a
				ON a.id = sjl.account_id
				AND a.deleted_at IS NULL
				AND a.code NOT LIKE '21%%'
			WHERE p.date BETWEEN ? AND ?
			  AND p.deleted_at IS NULL
			  AND %s
			  %s

			UNION ALL

			SELECT
				p.id as purchase_id,
				COALESCE(tax_account.code, '1105-005') as account_code,
				COALESCE(tax_account.name, 'PPN Masukan') as account_name,
				'Input Tax' as category_name,
				p.tax_amount as amount
			FROM purchases p
			LEFT JOIN accounts tax_account
				ON tax_account.code = '1105-005'
				AND tax_account.deleted_at IS NULL
			WHERE p.date BETWEEN ? AND ?
			  AND p.deleted_at IS NULL
			  AND COALESCE(p.tax_amount, 0) > 0.01
			  AND %s
			  %s
		)
		SELECT
			account_code,
			account_name,
			category_name,
			COUNT(DISTINCT purchase_id) as total_purchases,
			COALESCE(SUM(amount), 0) as total_amount
		FROM categorized_lines
		GROUP BY account_code, account_name, category_name
		ORDER BY total_amount DESC
	`, models.PurchaseOperationalReportWhereClause("p"), projectFilter, models.PurchaseOperationalReportWhereClause("p"), projectFilter)

	var categories []struct {
		AccountCode    string  `json:"account_code"`
		AccountName    string  `json:"account_name"`
		CategoryName   string  `json:"category_name"`
		TotalPurchases int64   `json:"total_purchases"`
		TotalAmount    float64 `json:"total_amount"`
	}

	err := s.db.WithContext(ctx).Raw(query, args...).Scan(&categories).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query purchases by category: %w", err)
	}

	// Calculate total for percentage calculation
	var totalAmount float64
	for _, cat := range categories {
		totalAmount += cat.TotalAmount
	}

	// Convert to result format with percentages
	var result []CategoryPurchaseSummary
	for _, cat := range categories {
		percentage := float64(0)
		if totalAmount > 0 {
			percentage = (cat.TotalAmount / totalAmount) * 100
		}

		summary := CategoryPurchaseSummary{
			CategoryName:   cat.CategoryName,
			AccountCode:    cat.AccountCode,
			AccountName:    cat.AccountName,
			TotalPurchases: cat.TotalPurchases,
			TotalAmount:    cat.TotalAmount,
			Percentage:     percentage,
		}

		result = append(result, summary)
	}

	return result, nil
}

// getPaymentAnalysis analyzes purchase mix by transaction method.
// Counts and amounts are exclusive so the report cannot show more cash+credit
// purchases than the total number of purchases.
func (s *SSOTPurchaseReportService) getPaymentAnalysis(ctx context.Context, startDate, endDate time.Time, projectID *uint) (PurchasePaymentAnalysis, error) {
	query := fmt.Sprintf(`
		SELECT 
			COUNT(CASE 
				WHEN UPPER(COALESCE(p.payment_method, '')) IN ('CASH', 'BANK_TRANSFER', 'TRANSFER', 'CHECK')
				THEN 1 END) as cash_purchases,
			COUNT(CASE 
				WHEN UPPER(COALESCE(p.payment_method, 'CREDIT')) NOT IN ('CASH', 'BANK_TRANSFER', 'TRANSFER', 'CHECK')
				THEN 1 END) as credit_purchases,
			COALESCE(SUM(CASE 
				WHEN UPPER(COALESCE(p.payment_method, '')) IN ('CASH', 'BANK_TRANSFER', 'TRANSFER', 'CHECK')
				THEN p.total_amount
				ELSE 0
			END), 0) as cash_amount,
			COALESCE(SUM(CASE 
				WHEN UPPER(COALESCE(p.payment_method, 'CREDIT')) NOT IN ('CASH', 'BANK_TRANSFER', 'TRANSFER', 'CHECK')
				THEN p.total_amount
				ELSE 0
			END), 0) as credit_amount,
			COALESCE(AVG(p.total_amount), 0) as average_order_value
		FROM purchases p
		WHERE p.date BETWEEN ? AND ?
		  AND p.deleted_at IS NULL
		  AND %s
	`, models.PurchaseOperationalReportWhereClause("p"))

	args := []interface{}{startDate, endDate}
	if projectID != nil {
		query += " AND p.project_id = ?"
		args = append(args, *projectID)
	}

	var analysis struct {
		CashPurchases     int64   `json:"cash_purchases"`
		CreditPurchases   int64   `json:"credit_purchases"`
		CashAmount        float64 `json:"cash_amount"`
		CreditAmount      float64 `json:"credit_amount"`
		AverageOrderValue float64 `json:"average_order_value"`
	}

	err := s.db.WithContext(ctx).Raw(query, args...).Scan(&analysis).Error
	if err != nil {
		return PurchasePaymentAnalysis{}, fmt.Errorf("failed to analyze payment patterns: %w", err)
	}

	totalAmount := analysis.CashAmount + analysis.CreditAmount
	cashPercentage := float64(0)
	creditPercentage := float64(0)

	if totalAmount > 0 {
		cashPercentage = (analysis.CashAmount / totalAmount) * 100
		creditPercentage = (analysis.CreditAmount / totalAmount) * 100
	}

	return PurchasePaymentAnalysis{
		CashPurchases:     analysis.CashPurchases,
		CreditPurchases:   analysis.CreditPurchases,
		CashAmount:        analysis.CashAmount,
		CreditAmount:      analysis.CreditAmount,
		CashPercentage:    cashPercentage,
		CreditPercentage:  creditPercentage,
		AverageOrderValue: analysis.AverageOrderValue,
	}, nil
}

// getTaxAnalysis analyzes tax information in purchases - using purchases table with tax fields
func (s *SSOTPurchaseReportService) getTaxAnalysis(ctx context.Context, startDate, endDate time.Time, projectID *uint) (PurchaseTaxAnalysis, error) {
	// Historical purchases retain the header-level PPN calculation. Component
	// VAT purchases use the invoice allocations because their taxable DPP can be
	// a subset of the purchase total and only some PPN may be creditable.
	//
	// Deployments can briefly run against a database before the additive
	// allocation migration is present. Keep the legacy query in that case so the
	// report stays available rather than referencing a missing table or column.
	componentVATAvailable := s.db.Migrator().HasTable(&models.PurchaseTaxAllocation{}) &&
		s.db.Migrator().HasColumn(&models.Purchase{}, "vat_treatment")

	legacyTaxQuery := fmt.Sprintf(`
		SELECT
			COALESCE(SUM(p.net_before_tax), 0) as total_taxable_amount,
			COALESCE(SUM(p.tax_amount), 0) as total_tax_amount,
			COALESCE(SUM(p.tax_amount), 0) as tax_reclaimable_amount
		FROM purchases p
		WHERE p.date BETWEEN ? AND ?
		  AND p.deleted_at IS NULL
		  AND %s
	`, models.PurchaseOperationalReportWhereClause("p"))

	taxQuery := legacyTaxQuery
	if componentVATAvailable {
		taxQuery = fmt.Sprintf(`
			SELECT
				COALESCE(SUM(CASE
					WHEN COALESCE(p.vat_treatment, 'STANDARD') = '%s' THEN COALESCE(component_tax.dpp_amount, 0)
					ELSE COALESCE(p.net_before_tax, 0)
				END), 0) as total_taxable_amount,
				COALESCE(SUM(CASE
					WHEN COALESCE(p.vat_treatment, 'STANDARD') = '%s' THEN COALESCE(component_tax.ppn_amount, 0)
					ELSE COALESCE(p.tax_amount, 0)
				END), 0) as total_tax_amount,
				COALESCE(SUM(CASE
					WHEN COALESCE(p.vat_treatment, 'STANDARD') = '%s' THEN COALESCE(component_tax.creditable_ppn_amount, 0)
					ELSE COALESCE(p.tax_amount, 0)
				END), 0) as tax_reclaimable_amount
			FROM purchases p
			LEFT JOIN (
				SELECT
					purchase_id,
					COALESCE(SUM(dpp_amount), 0) as dpp_amount,
					COALESCE(SUM(ppn_amount), 0) as ppn_amount,
					COALESCE(SUM(CASE WHEN COALESCE(is_creditable, FALSE) THEN ppn_amount ELSE 0 END), 0) as creditable_ppn_amount
				FROM purchase_tax_allocations
				WHERE deleted_at IS NULL
				GROUP BY purchase_id
			) component_tax ON component_tax.purchase_id = p.id
			WHERE p.date BETWEEN ? AND ?
			  AND p.deleted_at IS NULL
			  AND %s
		`,
			models.PurchaseVATTreatmentComponentDPPNilaiLain,
			models.PurchaseVATTreatmentComponentDPPNilaiLain,
			models.PurchaseVATTreatmentComponentDPPNilaiLain,
			models.PurchaseOperationalReportWhereClause("p"),
		)
	}

	taxArgs := []interface{}{startDate, endDate}
	if projectID != nil {
		taxQuery += " AND p.project_id = ?"
		taxArgs = append(taxArgs, *projectID)
	}

	var taxSummary struct {
		TotalTaxableAmount   float64 `json:"total_taxable_amount"`
		TotalTaxAmount       float64 `json:"total_tax_amount"`
		TaxReclaimableAmount float64 `json:"tax_reclaimable_amount"`
	}

	err := s.db.WithContext(ctx).Raw(taxQuery, taxArgs...).Scan(&taxSummary).Error
	if err != nil {
		return PurchaseTaxAnalysis{}, fmt.Errorf("failed to analyze tax: %w", err)
	}

	averageTaxRate := float64(0)
	if taxSummary.TotalTaxableAmount > 0 {
		averageTaxRate = (taxSummary.TotalTaxAmount / taxSummary.TotalTaxableAmount) * 100
	}

	// Get monthly tax breakdown. The production PostgreSQL expressions remain
	// unchanged; SQLite support keeps this report testable in the local suite.
	yearExpression := "EXTRACT(YEAR FROM p.date)"
	monthExpression := "EXTRACT(MONTH FROM p.date)"
	if s.db.Dialector.Name() == "sqlite" {
		yearExpression = "CAST(strftime('%Y', p.date) AS INTEGER)"
		monthExpression = "CAST(strftime('%m', p.date) AS INTEGER)"
	}

	legacyMonthlyTaxQuery := fmt.Sprintf(`
		SELECT
			%s as year,
			%s as month,
			COALESCE(SUM(p.tax_amount), 0) as tax_amount
		FROM purchases p
		WHERE p.date BETWEEN ? AND ?
		  AND p.deleted_at IS NULL
		  AND %s
	`, yearExpression, monthExpression, models.PurchaseOperationalReportWhereClause("p"))

	monthlyTaxQuery := legacyMonthlyTaxQuery
	if componentVATAvailable {
		monthlyTaxQuery = fmt.Sprintf(`
			SELECT
				%s as year,
				%s as month,
				COALESCE(SUM(CASE
					WHEN COALESCE(p.vat_treatment, 'STANDARD') = '%s' THEN COALESCE(component_tax.ppn_amount, 0)
					ELSE COALESCE(p.tax_amount, 0)
				END), 0) as tax_amount
			FROM purchases p
			LEFT JOIN (
				SELECT
					purchase_id,
					COALESCE(SUM(ppn_amount), 0) as ppn_amount
				FROM purchase_tax_allocations
				WHERE deleted_at IS NULL
				GROUP BY purchase_id
			) component_tax ON component_tax.purchase_id = p.id
			WHERE p.date BETWEEN ? AND ?
			  AND p.deleted_at IS NULL
			  AND %s
		`,
			yearExpression,
			monthExpression,
			models.PurchaseVATTreatmentComponentDPPNilaiLain,
			models.PurchaseOperationalReportWhereClause("p"),
		)
	}

	monthlyTaxArgs := []interface{}{startDate, endDate}
	if projectID != nil {
		monthlyTaxQuery += " AND p.project_id = ?"
		monthlyTaxArgs = append(monthlyTaxArgs, *projectID)
	}
	monthlyTaxQuery += `
		GROUP BY ` + yearExpression + `, ` + monthExpression + `
		ORDER BY year, month
	`

	var monthlyTax []struct {
		Year      int     `json:"year"`
		Month     int     `json:"month"`
		TaxAmount float64 `json:"tax_amount"`
	}

	err = s.db.WithContext(ctx).Raw(monthlyTaxQuery, monthlyTaxArgs...).Scan(&monthlyTax).Error
	if err != nil {
		log.Printf("Error getting monthly tax data: %v", err)
		monthlyTax = []struct {
			Year      int     `json:"year"`
			Month     int     `json:"month"`
			TaxAmount float64 `json:"tax_amount"`
		}{}
	}

	// Convert monthly tax to result format
	monthNames := []string{
		"", "January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December",
	}

	var taxByMonth []MonthlyTaxSummary
	for _, mt := range monthlyTax {
		summary := MonthlyTaxSummary{
			Year:      mt.Year,
			Month:     mt.Month,
			MonthName: monthNames[mt.Month],
			TaxAmount: mt.TaxAmount,
		}
		taxByMonth = append(taxByMonth, summary)
	}

	return PurchaseTaxAnalysis{
		TotalTaxableAmount:   taxSummary.TotalTaxableAmount,
		TotalTaxAmount:       taxSummary.TotalTaxAmount,
		AverageTaxRate:       averageTaxRate,
		TaxReclaimableAmount: taxSummary.TaxReclaimableAmount,
		TaxByMonth:           taxByMonth,
	}, nil
}

// getCompanyInfo returns company information for reports
func (s *SSOTPurchaseReportService) getCompanyInfo() CompanyInfo {
	// Prefer Settings table (admin-configured company information)
	var settings models.Settings
	if err := s.db.First(&settings).Error; err == nil {
		return CompanyInfo{
			Name:      settings.CompanyName,
			Address:   settings.CompanyAddress,
			City:      "", // City may be embedded in the address field
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

// getCurrencyFromSettings returns the configured currency or IDR as fallback
func (s *SSOTPurchaseReportService) getCurrencyFromSettings() string {
	var settings models.Settings
	if err := s.db.First(&settings).Error; err == nil && settings.Currency != "" {
		return settings.Currency
	}
	return "IDR"
}
