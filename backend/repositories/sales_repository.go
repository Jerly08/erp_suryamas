package repositories

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type SalesRepository struct {
	db *gorm.DB
}

func NewSalesRepository(db *gorm.DB) *SalesRepository {
	return &SalesRepository{
		db: db,
	}
}

// DB returns the database instance for direct access
func (r *SalesRepository) DB() *gorm.DB {
	return r.db
}

// Basic CRUD Operations

func (r *SalesRepository) Create(sale *models.Sale) (*models.Sale, error) {
	err := r.db.Create(sale).Error
	if err != nil {
		return nil, err
	}
	return r.FindByID(sale.ID)
}

func (r *SalesRepository) FindByID(id uint) (*models.Sale, error) {
	var sale models.Sale
	err := r.db.Preload("Customer").
		Preload("Customer.Addresses").
		Preload("User").
		Preload("SalesPerson").
		Preload("SaleItems").
		Preload("SaleItems.Product").
		Preload("SaleItems.RevenueAccount").
		Preload("SaleItems.TaxAccount").
		Preload("SalePayments").
		Preload("SalePayments.CashBank").
		Preload("SalePayments.Account").
		Preload("SalePayments.User").
		Preload("PaymentSchedules", func(db *gorm.DB) *gorm.DB {
			return db.Order("sequence ASC")
		}).
		Preload("SaleReturns").
		Preload("SaleReturns.ReturnItems").
		Preload("SaleReturns.User").
		First(&sale, id).Error

	if err != nil {
		return nil, err
	}
	return &sale, nil
}

func (r *SalesRepository) Update(sale *models.Sale) (*models.Sale, error) {
	err := r.db.Session(&gorm.Session{FullSaveAssociations: true}).Save(sale).Error
	if err != nil {
		return nil, err
	}
	return r.FindByID(sale.ID)
}

func (r *SalesRepository) Delete(id uint) error {
	return r.db.Delete(&models.Sale{}, id).Error
}

// Query Operations with Filters

func (r *SalesRepository) FindWithFilter(filter models.SalesFilter) ([]models.Sale, int64, error) {
	var sales []models.Sale
	var total int64

	// Base query for counting
	countQuery := r.db.Model(&models.Sale{})

	// Base query for fetching data
	dataQuery := r.db.Model(&models.Sale{}).
		Preload("Customer").
		Preload("User").
		Preload("SalesPerson").
		Preload("SaleItems").
		Preload("SalePayments")

	// Apply filters to both queries
	if filter.Status != "" {
		switch filter.Status {
		case models.SaleStatusDraft:
			countQuery = countQuery.Where("status IN ?", []string{models.SaleStatusDraft, models.SaleStatusPending, models.SaleStatusConfirmed})
			dataQuery = dataQuery.Where("status IN ?", []string{models.SaleStatusDraft, models.SaleStatusPending, models.SaleStatusConfirmed})
		case models.SaleStatusInvoiced:
			countQuery = countQuery.Where("status IN ?", []string{models.SaleStatusInvoiced, models.SaleStatusOverdue})
			dataQuery = dataQuery.Where("status IN ?", []string{models.SaleStatusInvoiced, models.SaleStatusOverdue})
		default:
			countQuery = countQuery.Where("status = ?", filter.Status)
			dataQuery = dataQuery.Where("status = ?", filter.Status)
		}
	}

	if filter.CustomerID != "" {
		countQuery = countQuery.Where("customer_id = ?", filter.CustomerID)
		dataQuery = dataQuery.Where("customer_id = ?", filter.CustomerID)
	}

	if filter.StartDate != "" {
		countQuery = countQuery.Where("date >= ?", filter.StartDate)
		dataQuery = dataQuery.Where("date >= ?", filter.StartDate)
	}

	if filter.EndDate != "" {
		countQuery = countQuery.Where("date <= ?", filter.EndDate)
		dataQuery = dataQuery.Where("date <= ?", filter.EndDate)
	}

	if filter.Search != "" {
		searchPattern := "%" + strings.ToLower(filter.Search) + "%"
		// Apply search to count query
		countQuery = countQuery.Joins("JOIN contacts ON contacts.id = sales.customer_id").
			Where("LOWER(sales.code) LIKE ? OR LOWER(sales.invoice_number) LIKE ? OR LOWER(contacts.name) LIKE ?",
				searchPattern, searchPattern, searchPattern)
		// Apply search to data query
		dataQuery = dataQuery.Joins("JOIN contacts ON contacts.id = sales.customer_id").
			Where("LOWER(sales.code) LIKE ? OR LOWER(sales.invoice_number) LIKE ? OR LOWER(contacts.name) LIKE ?",
				searchPattern, searchPattern, searchPattern)
	}

	// Count total records
	err := countQuery.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Apply pagination and fetch data
	offset := (filter.Page - 1) * filter.Limit
	err = dataQuery.Offset(offset).Limit(filter.Limit).
		Order("sales.date DESC, sales.created_at DESC").
		Find(&sales).Error

	return sales, total, err
}

func (r *SalesRepository) FindByCustomerID(customerID uint, page, limit int) ([]models.Sale, error) {
	var sales []models.Sale
	offset := (page - 1) * limit

	err := r.db.Where("customer_id = ?", customerID).
		Preload("SaleItems").
		Preload("SaleItems.Product").
		Preload("SalePayments").
		Offset(offset).Limit(limit).
		Order("date DESC").
		Find(&sales).Error

	return sales, err
}

func (r *SalesRepository) FindInvoicesByCustomerID(customerID uint) ([]models.Sale, error) {
	var sales []models.Sale

	err := r.db.Where("customer_id = ? AND invoice_number IS NOT NULL AND invoice_number != ''", customerID).
		Preload("SaleItems").
		Preload("SaleItems.Product").
		Preload("SalePayments").
		Order("date DESC").
		Find(&sales).Error

	return sales, err
}

func (r *SalesRepository) FindByDateRange(startDate, endDate string) ([]models.Sale, error) {
	var sales []models.Sale

	query := r.db.Preload("Customer").
		Preload("SaleItems").
		Preload("SaleItems.Product")

	if startDate != "" {
		query = query.Where("date >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("date <= ?", endDate)
	}

	err := query.Order("date DESC").Find(&sales).Error
	return sales, err
}

// Payment Operations

func (r *SalesRepository) CreatePayment(payment *models.SalePayment) (*models.SalePayment, error) {
	err := r.db.Create(payment).Error
	if err != nil {
		return nil, err
	}

	var createdPayment models.SalePayment
	err = r.db.Preload("Sale").
		Preload("CashBank").
		Preload("Account").
		Preload("User").
		First(&createdPayment, payment.ID).Error

	return &createdPayment, err
}

func (r *SalesRepository) FindPaymentsBySaleID(saleID uint) ([]models.SalePayment, error) {
	var payments []models.SalePayment
	err := r.db.Where("sale_id = ?", saleID).
		Preload("CashBank").
		Preload("Account").
		Preload("User").
		Order("date DESC").
		Find(&payments).Error

	return payments, err
}

// Return Operations

func (r *SalesRepository) CreateReturn(saleReturn *models.SaleReturn) (*models.SaleReturn, error) {
	err := r.db.Create(saleReturn).Error
	if err != nil {
		return nil, err
	}

	var createdReturn models.SaleReturn
	err = r.db.Preload("Sale").
		Preload("User").
		Preload("Approver").
		First(&createdReturn, saleReturn.ID).Error

	return &createdReturn, err
}

func (r *SalesRepository) CreateReturnItem(returnItem *models.SaleReturnItem) error {
	return r.db.Create(returnItem).Error
}

func (r *SalesRepository) FindReturns(page, limit int) ([]models.SaleReturn, error) {
	var returns []models.SaleReturn
	offset := (page - 1) * limit

	err := r.db.Preload("Sale").
		Preload("Sale.Customer").
		Preload("User").
		Preload("ReturnItems").
		Preload("ReturnItems.SaleItem").
		Preload("ReturnItems.SaleItem.Product").
		Offset(offset).Limit(limit).
		Order("date DESC").
		Find(&returns).Error

	return returns, err
}

// Sale Item Operations

func (r *SalesRepository) FindSaleItemByID(id uint) (*models.SaleItem, error) {
	var item models.SaleItem
	err := r.db.Preload("Product").
		Preload("Sale").
		First(&item, id).Error

	return &item, err
}

// Counting Operations for Number Generation

func (r *SalesRepository) CountByTypeAndYear(saleType string, year int) (int64, error) {
	var count int64
	startDate := fmt.Sprintf("%d-01-01", year)
	endDate := fmt.Sprintf("%d-12-31", year)

	err := r.db.Model(&models.Sale{}).
		Where("type = ? AND date BETWEEN ? AND ?", saleType, startDate, endDate).
		Count(&count).Error

	return count, err
}

// FindByCode checks if a sale with the given code exists
func (r *SalesRepository) FindByCode(code string) (*models.Sale, error) {
	var sale models.Sale
	err := r.db.Where("code = ?", code).First(&sale).Error
	if err != nil {
		return nil, err
	}
	return &sale, nil
}

// ExistsByCode checks if a sale with the given code exists
func (r *SalesRepository) ExistsByCode(code string) (bool, error) {
	var count int64
	err := r.db.Model(&models.Sale{}).Where("code = ?", code).Count(&count).Error
	return count > 0, err
}

func (r *SalesRepository) CountInvoicesByMonth(year, month int) (int64, error) {
	var count int64
	startDate := fmt.Sprintf("%04d-%02d-01", year, month)
	// Use the last day of the month dynamically to avoid invalid dates like 2025-09-31
	nextMonth := time.Date(year, time.Month(month+1), 1, 0, 0, 0, 0, time.UTC)
	lastDay := nextMonth.AddDate(0, 0, -1)
	endDate := lastDay.Format("2006-01-02")

	err := r.db.Model(&models.Sale{}).
		Where("invoice_number IS NOT NULL AND invoice_number != '' AND date BETWEEN ? AND ?",
			startDate, endDate).
		Count(&count).Error

	return count, err
}

func (r *SalesRepository) CountQuotationsByMonth(year, month int) (int64, error) {
	var count int64
	startDate := fmt.Sprintf("%04d-%02d-01", year, month)
	// Use the last day of the month dynamically to avoid invalid dates
	nextMonth := time.Date(year, time.Month(month+1), 1, 0, 0, 0, 0, time.UTC)
	lastDay := nextMonth.AddDate(0, 0, -1)
	endDate := lastDay.Format("2006-01-02")

	err := r.db.Model(&models.Sale{}).
		Where("quotation_number IS NOT NULL AND quotation_number != '' AND date BETWEEN ? AND ?",
			startDate, endDate).
		Count(&count).Error

	return count, err
}

func (r *SalesRepository) CountPaymentsByMonth(year, month int) (int64, error) {
	var count int64
	startDate := fmt.Sprintf("%04d-%02d-01", year, month)
	// Use the last day of the month dynamically to avoid invalid dates
	nextMonth := time.Date(year, time.Month(month+1), 1, 0, 0, 0, 0, time.UTC)
	lastDay := nextMonth.AddDate(0, 0, -1)
	endDate := lastDay.Format("2006-01-02")

	err := r.db.Model(&models.SalePayment{}).
		Where("date BETWEEN ? AND ?", startDate, endDate).
		Count(&count).Error

	return count, err
}

func (r *SalesRepository) CountReturnsByMonth(year, month int) (int64, error) {
	var count int64
	startDate := fmt.Sprintf("%04d-%02d-01", year, month)
	// Use the last day of the month dynamically to avoid invalid dates
	nextMonth := time.Date(year, time.Month(month+1), 1, 0, 0, 0, 0, time.UTC)
	lastDay := nextMonth.AddDate(0, 0, -1)
	endDate := lastDay.Format("2006-01-02")

	err := r.db.Model(&models.SaleReturn{}).
		Where("date BETWEEN ? AND ?", startDate, endDate).
		Count(&count).Error

	return count, err
}

func (r *SalesRepository) CountCreditNotesByMonth(year, month int) (int64, error) {
	var count int64
	startDate := fmt.Sprintf("%04d-%02d-01", year, month)
	// Use the last day of the month dynamically to avoid invalid dates
	nextMonth := time.Date(year, time.Month(month+1), 1, 0, 0, 0, 0, time.UTC)
	lastDay := nextMonth.AddDate(0, 0, -1)
	endDate := lastDay.Format("2006-01-02")

	err := r.db.Model(&models.SaleReturn{}).
		Where("credit_note_number IS NOT NULL AND credit_note_number != '' AND date BETWEEN ? AND ?",
			startDate, endDate).
		Count(&count).Error

	return count, err
}

// Analytics and Reporting

func (r *SalesRepository) GetSalesSummary(startDate, endDate string) (*models.SalesSummaryResponse, error) {
	var result struct {
		TotalSales       int64   `json:"total_sales"`
		TotalAmount      float64 `json:"total_amount"`
		TotalPaid        float64 `json:"total_paid"`
		TotalOutstanding float64 `json:"total_outstanding"`
	}

	query := r.db.Model(&models.Sale{}).
		Where("status IN ?", []string{models.SaleStatusInvoiced, models.SaleStatusOverdue, models.SaleStatusPaid})

	if startDate != "" {
		query = query.Where("date >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("date <= ?", endDate)
	}

	err := query.Select(`
		COUNT(*) as total_sales,
		COALESCE(SUM(total_amount), 0) as total_amount,
		COALESCE(SUM(paid_amount), 0) as total_paid,
		COALESCE(SUM(outstanding_amount), 0) as total_outstanding
	`).Scan(&result).Error

	if err != nil {
		return nil, err
	}

	// Calculate average order value
	avgOrderValue := 0.0
	if result.TotalSales > 0 {
		avgOrderValue = result.TotalAmount / float64(result.TotalSales)
	}

	// Get top customers
	var topCustomers []models.CustomerSales
	customerQuery := r.db.Model(&models.Sale{}).
		Select(`
			customer_id,
			contacts.name as customer_name,
			COALESCE(SUM(total_amount), 0) as total_amount,
			COUNT(*) as total_orders
		`).
		Joins("JOIN contacts ON contacts.id = sales.customer_id").
		Where("sales.status IN ?", []string{models.SaleStatusInvoiced, models.SaleStatusOverdue, models.SaleStatusPaid})

	if startDate != "" {
		customerQuery = customerQuery.Where("sales.date >= ?", startDate)
	}
	if endDate != "" {
		customerQuery = customerQuery.Where("sales.date <= ?", endDate)
	}

	err = customerQuery.
		Group("customer_id, contacts.name").
		Order("total_amount DESC").
		Limit(5).
		Scan(&topCustomers).Error

	if err != nil {
		return nil, err
	}

	return &models.SalesSummaryResponse{
		TotalSales:       result.TotalSales,
		TotalAmount:      result.TotalAmount,
		TotalPaid:        result.TotalPaid,
		TotalOutstanding: result.TotalOutstanding,
		AvgOrderValue:    avgOrderValue,
		TopCustomers:     topCustomers,
	}, nil
}

func (r *SalesRepository) GetSalesAnalytics(period, year string) (*models.SalesAnalyticsResponse, error) {
	var data []models.SalesAnalyticsData

	query := r.db.Model(&models.Sale{}).
		Where("status != ? AND EXTRACT(YEAR FROM date) = ?", models.SaleStatusCancelled, year)

	var groupBy, selectClause string
	switch period {
	case "daily":
		selectClause = `
			TO_CHAR(date, 'YYYY-MM-DD') as period,
			COUNT(*) as total_sales,
			COALESCE(SUM(total_amount), 0) as total_amount
		`
		groupBy = "TO_CHAR(date, 'YYYY-MM-DD')"
	case "weekly":
		selectClause = `
			TO_CHAR(date, 'YYYY-"W"WW') as period,
			COUNT(*) as total_sales,
			COALESCE(SUM(total_amount), 0) as total_amount
		`
		groupBy = "TO_CHAR(date, 'YYYY-\"W\"WW')"
	default: // monthly
		selectClause = `
			TO_CHAR(date, 'YYYY-MM') as period,
			COUNT(*) as total_sales,
			COALESCE(SUM(total_amount), 0) as total_amount
		`
		groupBy = "TO_CHAR(date, 'YYYY-MM')"
	}

	err := query.Select(selectClause).
		Group(groupBy).
		Order("period ASC").
		Scan(&data).Error

	if err != nil {
		return nil, err
	}

	// Calculate growth rates
	for i := range data {
		if i > 0 {
			prevAmount := data[i-1].TotalAmount
			currentAmount := data[i].TotalAmount
			if prevAmount > 0 {
				data[i].GrowthRate = ((currentAmount - prevAmount) / prevAmount) * 100
			}
		}
	}

	return &models.SalesAnalyticsResponse{
		Period: period,
		Data:   data,
	}, nil
}

func (r *SalesRepository) GetReceivablesReport() (*models.ReceivablesReportResponse, error) {
	var receivables []models.ReceivableItem
	now := time.Now()

	err := r.db.Model(&models.Sale{}).
		Select(`
			sales.id as sale_id,
			sales.invoice_number,
			contacts.name as customer_name,
			sales.date,
			sales.due_date,
			sales.total_amount,
			sales.paid_amount,
			sales.outstanding_amount,
			CASE 
				WHEN sales.due_date < ? THEN EXTRACT(DAY FROM ? - sales.due_date)::int
				ELSE 0
			END as days_overdue,
			sales.status
		`, now, now).
		Joins("JOIN contacts ON contacts.id = sales.customer_id").
		Where("sales.outstanding_amount > 0 AND sales.status IN (?)",
			[]string{models.SaleStatusInvoiced, models.SaleStatusOverdue}).
		Order("sales.due_date ASC").
		Scan(&receivables).Error

	if err != nil {
		return nil, err
	}

	// Calculate totals
	var totalOutstanding, overdueAmount float64
	for _, item := range receivables {
		totalOutstanding += item.OutstandingAmount
		if item.DaysOverdue > 0 {
			overdueAmount += item.OutstandingAmount
		}
	}

	return &models.ReceivablesReportResponse{
		TotalOutstanding: totalOutstanding,
		OverdueAmount:    overdueAmount,
		Receivables:      receivables,
	}, nil
}

// GetCustomerOutstandingAmount gets customer outstanding amount
func (r *SalesRepository) GetCustomerOutstandingAmount(customerID uint) (float64, error) {
	var totalOutstanding float64
	err := r.db.Model(&models.Sale{}).
		Where("customer_id = ? AND outstanding_amount > 0", customerID).
		Select("COALESCE(SUM(outstanding_amount), 0)").
		Scan(&totalOutstanding).Error
	return totalOutstanding, err
}

// CreateJournal creates journal entry
func (r *SalesRepository) CreateJournal(journal *models.Journal) error {
	return r.db.Create(journal).Error
}

// CountOrdersByMonth counts orders by month
func (r *SalesRepository) CountOrdersByMonth(year, month int) (int64, error) {
	var count int64
	err := r.db.Model(&models.Sale{}).
		Where("type = ? AND EXTRACT(year FROM created_at) = ? AND EXTRACT(month FROM created_at) = ?",
			"ORDER", year, month).
		Count(&count).Error
	return count, err
}

// CountJournalsByMonth counts journals by month
func (r *SalesRepository) CountJournalsByMonth(year, month int) (int64, error) {
	var count int64
	err := r.db.Model(&models.Journal{}).
		Where("reference_type = ? AND EXTRACT(year FROM date) = ? AND EXTRACT(month FROM date) = ?",
			models.JournalRefTypeSale, year, month).
		Count(&count).Error
	return count, err
}

// FindInvoicesDueOn finds invoices that are due on a specific date
func (r *SalesRepository) FindInvoicesDueOn(dueDate time.Time) ([]models.Sale, error) {
	var sales []models.Sale
	err := r.db.Where("due_date = ? AND outstanding_amount > 0 AND status IN (?)",
		dueDate.Format("2006-01-02"),
		[]string{models.SaleStatusInvoiced}).
		Preload("Customer").
		Preload("SaleItems").
		Preload("SalePayments").
		Find(&sales).Error
	return sales, err
}

// FindInvoicesOverdueAsOf finds invoices that are overdue as of a specific date
func (r *SalesRepository) FindInvoicesOverdueAsOf(asOfDate time.Time) ([]models.Sale, error) {
	var sales []models.Sale
	err := r.db.Where("due_date < ? AND outstanding_amount > 0 AND status IN (?)",
		asOfDate.Format("2006-01-02"),
		[]string{models.SaleStatusInvoiced, models.SaleStatusOverdue}).
		Preload("Customer").
		Preload("SaleItems").
		Preload("SalePayments").
		Find(&sales).Error
	return sales, err
}

// FindOverdueInvoicesForInterest finds overdue invoices that need interest calculation
func (r *SalesRepository) FindOverdueInvoicesForInterest() ([]models.Sale, error) {
	var sales []models.Sale
	err := r.db.Where("status = ? AND outstanding_amount > 0 AND due_date < ?",
		models.SaleStatusOverdue, time.Now().Format("2006-01-02")).
		Preload("Customer").
		Preload("SaleItems").
		Preload("SalePayments").
		Find(&sales).Error
	return sales, err
}

// FindOverdueInvoicesWithDays finds all overdue invoices with calculated overdue days
func (r *SalesRepository) FindOverdueInvoicesWithDays() ([]models.Sale, error) {
	var sales []models.Sale
	err := r.db.Where("status = ? AND outstanding_amount > 0",
		models.SaleStatusOverdue).
		Preload("Customer").
		Preload("SaleItems").
		Preload("SalePayments").
		Find(&sales).Error
	return sales, err
}
