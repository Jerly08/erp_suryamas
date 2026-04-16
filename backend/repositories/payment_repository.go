package repositories

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"gorm.io/gorm"
	"math"
	"time"
)

type PaymentRepository struct {
	db *gorm.DB
}

func NewPaymentRepository(db *gorm.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

// FindAll retrieves all payments
func (r *PaymentRepository) FindAll() ([]models.Payment, error) {
	var payments []models.Payment
	err := r.db.
		Preload("Contact").
		Preload("User").
		Preload("Approver").
		Preload("Rejector").
		Find(&payments).Error
	return payments, err
}

// FindByID retrieves payment by ID
func (r *PaymentRepository) FindByID(id uint) (*models.Payment, error) {
	var payment models.Payment
	err := r.db.
		Preload("Contact").
		Preload("User").
		Preload("Approver").
		Preload("Rejector").
		First(&payment, id).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

// FindWithFilter retrieves payments with filters
func (r *PaymentRepository) FindWithFilter(filter PaymentFilter) (*PaymentResult, error) {
	query := r.db.
		Model(&models.Payment{}).
		Preload("Contact").
		Preload("User").
		Preload("Approver").
		Preload("Rejector")

	// Apply filters
	if filter.ContactID > 0 {
		query = query.Where("contact_id = ?", filter.ContactID)
	}
	if filter.UserID > 0 {
		query = query.Where("payments.user_id = ?", filter.UserID)
	}

	if !filter.StartDate.IsZero() {
		query = query.Where("date >= ?", filter.StartDate)
	}

	if !filter.EndDate.IsZero() {
		query = query.Where("date <= ?", filter.EndDate)
	}

	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}

	if filter.Method != "" {
		query = query.Where("method = ?", filter.Method)
	}

	if filter.Type != "" {
		// Type would be determined by reference type or other logic
		query = query.Where("reference_type = ?", filter.Type)
	}

	// Apply search filter - search in code, contact name, reference, notes
	if filter.Search != "" {
		searchTerm := "%" + filter.Search + "%"
		query = query.Joins("LEFT JOIN contacts ON contacts.id = payments.contact_id").
			Where("payments.code ILIKE ? OR contacts.name ILIKE ? OR payments.reference ILIKE ? OR payments.notes ILIKE ?",
				searchTerm, searchTerm, searchTerm, searchTerm)
	}

	// Count total records
	var total int64
	query.Count(&total)

	// Apply pagination
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.Limit <= 0 {
		filter.Limit = 10
	}

	offset := (filter.Page - 1) * filter.Limit

	// Get paginated results
	var payments []models.Payment
	err := query.Order("date DESC, id DESC").
		Limit(filter.Limit).
		Offset(offset).
		Find(&payments).Error

	if err != nil {
		return nil, err
	}

	totalPages := int(math.Ceil(float64(total) / float64(filter.Limit)))

	return &PaymentResult{
		Data:       payments,
		Total:      total,
		Page:       filter.Page,
		Limit:      filter.Limit,
		TotalPages: totalPages,
	}, nil
}

// Create creates a new payment
func (r *PaymentRepository) Create(payment *models.Payment) (*models.Payment, error) {
	if err := r.db.Create(payment).Error; err != nil {
		return nil, err
	}

	// Reload with associations
	return r.FindByID(payment.ID)
}

// Update updates a payment
func (r *PaymentRepository) Update(payment *models.Payment) (*models.Payment, error) {
	if err := r.db.Save(payment).Error; err != nil {
		return nil, err
	}

	return r.FindByID(payment.ID)
}

// Delete soft deletes a payment
func (r *PaymentRepository) Delete(id uint) error {
	return r.db.Delete(&models.Payment{}, id).Error
}

// GetPaymentsByContactID retrieves payments for a specific contact
func (r *PaymentRepository) GetPaymentsByContactID(contactID uint, page, limit int) ([]models.Payment, error) {
	offset := (page - 1) * limit

	var payments []models.Payment
	err := r.db.Where("contact_id = ?", contactID).
		Order("date DESC").
		Limit(limit).
		Offset(offset).
		Find(&payments).Error

	return payments, err
}

// GetPaymentsByDateRange retrieves payments within date range
func (r *PaymentRepository) GetPaymentsByDateRange(startDate, endDate time.Time, userID uint) ([]models.Payment, error) {
	var payments []models.Payment
	query := r.db.Model(&models.Payment{})
	if userID > 0 {
		query = query.Where("payments.user_id = ?", userID)
	}
	err := query.Where("date BETWEEN ? AND ?", startDate, endDate).
		Order("date DESC").
		Find(&payments).Error

	return payments, err
}

// GetPaymentSummary gets payment summary statistics
func (r *PaymentRepository) GetPaymentSummary(startDate, endDate string, userID uint) (*PaymentSummary, error) {
	summary := &PaymentSummary{}

	// Parse dates
	start, _ := time.Parse("2006-01-02", startDate)
	end, _ := time.Parse("2006-01-02", endDate)

	baseQuery := r.db.Model(&models.Payment{})
	if userID > 0 {
		baseQuery = baseQuery.Where("payments.user_id = ?", userID)
	}

	// Total received (from customers)
	baseQuery.Session(&gorm.Session{}).
		Joins("JOIN contacts ON payments.contact_id = contacts.id").
		Where("contacts.type = ? AND payments.date BETWEEN ? AND ? AND payments.status = ?",
			"CUSTOMER", start, end, models.PaymentStatusCompleted).
		Select("COALESCE(SUM(amount), 0) as total").
		Scan(&summary.TotalReceived)

	// Total paid (to vendors)
	baseQuery.Session(&gorm.Session{}).
		Joins("JOIN contacts ON payments.contact_id = contacts.id").
		Where("contacts.type = ? AND payments.date BETWEEN ? AND ? AND payments.status = ?",
			"VENDOR", start, end, models.PaymentStatusCompleted).
		Select("COALESCE(SUM(amount), 0) as total").
		Scan(&summary.TotalPaid)

	// Payment method breakdown
	var methodBreakdown []struct {
		Method string
		Total  float64
	}

	baseQuery.Session(&gorm.Session{}).
		Where("date BETWEEN ? AND ? AND status = ?", start, end, models.PaymentStatusCompleted).
		Select("method, SUM(amount) as total").
		Group("method").
		Scan(&methodBreakdown)

	summary.ByMethod = make(map[string]float64)
	for _, item := range methodBreakdown {
		summary.ByMethod[item.Method] = item.Total
	}

	// Status counts
	var statusCounts []struct {
		Status string
		Count  int64
	}

	baseQuery.Session(&gorm.Session{}).
		Where("date BETWEEN ? AND ?", start, end).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&statusCounts)

	summary.StatusCounts = make(map[string]int64)
	for _, item := range statusCounts {
		summary.StatusCounts[item.Status] = item.Count
	}

	// Calculate net flow (received - paid)
	summary.NetFlow = summary.TotalReceived - summary.TotalPaid

	return summary, nil
}

// CountByMonth counts payments by month for code generation
func (r *PaymentRepository) CountByMonth(year, month int) (int64, error) {
	var count int64
	startDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, 0).Add(-time.Second)

	err := r.db.Model(&models.Payment{}).
		Where("created_at BETWEEN ? AND ? AND deleted_at IS NULL", startDate, endDate).
		Count(&count).Error

	return count, err
}

// CountByMonthAndPrefix counts payments by month and code prefix for code generation
func (r *PaymentRepository) CountByMonthAndPrefix(year, month int, prefix string) (int64, error) {
	var count int64
	startDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, 0).Add(-time.Second)

	pattern := fmt.Sprintf("%s/%04d/%02d/%%", prefix, year, month)
	err := r.db.Model(&models.Payment{}).
		Where("created_at BETWEEN ? AND ? AND code LIKE ? AND deleted_at IS NULL", startDate, endDate, pattern).
		Count(&count).Error

	return count, err
}

// CountJournalsByMonth counts payment journals by month for code generation
func (r *PaymentRepository) CountJournalsByMonth(year, month int) (int64, error) {
	var count int64
	err := r.db.Model(&models.JournalEntry{}).
		Where("reference_type = ? AND EXTRACT(year FROM entry_date) = ? AND EXTRACT(month FROM entry_date) = ?",
			models.JournalRefPayment, year, month).
		Count(&count).Error
	return count, err
}

// CreatePaymentAllocation creates payment allocation record
func (r *PaymentRepository) CreatePaymentAllocation(allocation *models.PaymentAllocation) error {
	return r.db.Create(allocation).Error
}

// GetPaymentAllocations retrieves allocations for a payment
func (r *PaymentRepository) GetPaymentAllocations(paymentID uint) ([]models.PaymentAllocation, error) {
	var allocations []models.PaymentAllocation
	err := r.db.Where("payment_id = ?", paymentID).Find(&allocations).Error
	return allocations, err
}

// DTOs
type PaymentFilter struct {
	ContactID uint      `json:"contact_id"`
	UserID    uint      `json:"user_id"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Status    string    `json:"status"`
	Method    string    `json:"method"`
	Type      string    `json:"type"`
	Search    string    `json:"search"` // Search query for code, contact name, reference, notes
	Page      int       `json:"page"`
	Limit     int       `json:"limit"`
}

type PaymentResult struct {
	Data       []models.Payment `json:"data"`
	Total      int64            `json:"total"`
	Page       int              `json:"page"`
	Limit      int              `json:"limit"`
	TotalPages int              `json:"total_pages"`
}

type PaymentSummary struct {
	TotalReceived float64            `json:"total_received"`
	TotalPaid     float64            `json:"total_paid"`
	NetFlow       float64            `json:"net_flow"`
	ByMethod      map[string]float64 `json:"by_method"`
	StatusCounts  map[string]int64   `json:"status_counts"`
}
