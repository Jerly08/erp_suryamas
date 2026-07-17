package services

import (
	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"log"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type SettingsService struct {
	db *gorm.DB
}

// InitializeCOA sets the company type and triggers account seeding
func (s *SettingsService) InitializeCOA(companyType string, userID uint) error {
	var settings models.Settings
	if err := s.db.First(&settings).Error; err != nil {
		return fmt.Errorf("failed to get settings: %v", err)
	}

	// Double check if already set
	if settings.CompanyType != "" {
		return errors.New("company type is already set and cannot be changed")
	}

	// Update CompanyType
	updates := map[string]interface{}{
		"company_type": companyType,
		"updated_by":   userID,
	}

	if err := s.db.Model(&settings).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update company type: %v", err)
	}

	// Invalidate cache
	settingsCacheMutex.Lock()
	settingsCache = nil
	settingsCacheMutex.Unlock()

	// Trigger seeding
	log.Printf("🚀 Initializing COA used type: %s", companyType)
	if err := database.SeedAccountsImproved(s.db); err != nil {
		return fmt.Errorf("failed to seed accounts: %v", err)
	}

	return nil
}

// Simple in-memory cache for settings (process-local)
var (
	settingsCache      *models.Settings
	settingsCacheAt    time.Time
	settingsCacheMutex sync.RWMutex
	settingsCacheTTL   = 5 * time.Minute
)

// ClearSettingsCache invalidates the process-local settings cache.
func ClearSettingsCache() {
	settingsCacheMutex.Lock()
	settingsCache = nil
	settingsCacheAt = time.Time{}
	settingsCacheMutex.Unlock()
}

// NewSettingsService creates a new instance of SettingsService
func NewSettingsService(db *gorm.DB) *SettingsService {
	return &SettingsService{db: db}
}

// GetSettings retrieves the system settings (with caching)
func (s *SettingsService) GetSettings() (*models.Settings, error) {
	// Try cache first
	settingsCacheMutex.RLock()
	if settingsCache != nil && time.Since(settingsCacheAt) < settingsCacheTTL {
		defer settingsCacheMutex.RUnlock()
		return settingsCache, nil
	}
	settingsCacheMutex.RUnlock()

	var settings models.Settings
	// First, try to get existing settings
	err := s.db.First(&settings).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// If no settings exist, create default settings (from accounting_config)
			settings = s.createDefaultSettings()
			if err := s.db.Create(&settings).Error; err != nil {
				log.Printf("Error creating default settings: %v", err)
				return nil, err
			}
		} else {
			log.Printf("Error fetching settings: %v", err)
			return nil, err
		}
	}

	// Update cache
	settingsCacheMutex.Lock()
	settingsCache = &settings
	settingsCacheAt = time.Now()
	settingsCacheMutex.Unlock()

	return &settings, nil
}

// UpdateSettings updates the system settings
func (s *SettingsService) UpdateSettings(updates map[string]interface{}, userID uint) error {
	var settings models.Settings
	// Get existing settings or create if not exists
	err := s.db.First(&settings).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			settings = s.createDefaultSettings()
			if err := s.db.Create(&settings).Error; err != nil {
				log.Printf("Error creating settings: %v", err)
				return err
			}
		} else {
			log.Printf("Error fetching settings: %v", err)
			return err
		}
	}

	// Normalize fields (frontend-backend sync)
	if v, ok := updates["fiscal_year_start"].(string); ok && v != "" {
		updates["fiscal_year_start"] = s.normalizeFiscalYearStart(v)
	}

	if err := s.validateSalesNextNumberUpdate(&settings, updates); err != nil {
		return err
	}

	// Capture old values for audit logging
	oldValues := s.captureCurrentValues(&settings, updates)
	// Add the user who is updating
	updates["updated_by"] = userID
	// Validate certain fields before updating
	if err := s.validateSettings(updates); err != nil {
		return err
	}
	// Update settings
	if err := s.db.Model(&settings).Updates(updates).Error; err != nil {
		log.Printf("Error updating settings: %v", err)
		return err
	}
	// Invalidate cache after update
	settingsCacheMutex.Lock()
	settingsCache = nil
	settingsCacheMutex.Unlock()
	// Log changes to history
	go s.logSettingsChanges(settings.ID, oldValues, updates, userID, "UPDATE")
	return nil
}

// validateSettings validates the settings before saving
func (s *SettingsService) validateSettings(updates map[string]interface{}) error {
	// Validate email if present
	if email, ok := updates["company_email"].(string); ok {
		if email != "" && !isValidEmail(email) {
			return errors.New("invalid email format")
		}
	}
	// Validate tax rate if present
	if taxRate, ok := updates["default_tax_rate"].(float64); ok {
		if taxRate < 0 || taxRate > 100 {
			return errors.New("tax rate must be between 0 and 100")
		}
	}
	// Validate decimal places
	if decimalPlaces, ok := updates["decimal_places"].(int); ok {
		if decimalPlaces < 0 || decimalPlaces > 4 {
			return errors.New("decimal places must be between 0 and 4")
		}
	}
	// Validate and normalize logo display scale from JSON numbers.
	if value, exists := updates["company_logo_scale"]; exists {
		var logoScale float64
		switch number := value.(type) {
		case float64:
			logoScale = number
		case float32:
			logoScale = float64(number)
		case int:
			logoScale = float64(number)
		default:
			return errors.New("company logo scale must be a number")
		}
		if math.Trunc(logoScale) != logoScale || logoScale < 50 || logoScale > 180 {
			return errors.New("company logo scale must be a whole number between 50 and 180")
		}
		updates["company_logo_scale"] = int(logoScale)
	}
	// The footer branding is rendered on public screens, so keep it concise and non-empty.
	if value, exists := updates["powered_by_text"]; exists {
		poweredByText, ok := value.(string)
		if !ok {
			return errors.New("teks branding footer harus berupa teks")
		}

		poweredByText = strings.TrimSpace(poweredByText)
		if poweredByText == "" {
			return errors.New("teks branding footer tidak boleh kosong")
		}
		if utf8.RuneCountInString(poweredByText) > 160 {
			return errors.New("teks branding footer maksimal 160 karakter")
		}

		updates["powered_by_text"] = poweredByText
	}
	// Validate language
	if language, ok := updates["language"].(string); ok {
		if language != "id" && language != "en" {
			return errors.New("language must be 'id' or 'en'")
		}
	}
	// Validate date format
	if dateFormat, ok := updates["date_format"].(string); ok {
		validFormats := []string{"DD/MM/YYYY", "MM/DD/YYYY", "YYYY-MM-DD", "DD-MM-YYYY"}
		isValid := false
		for _, format := range validFormats {
			if dateFormat == format {
				isValid = true
				break
			}
		}
		if !isValid {
			return errors.New("invalid date format")
		}
	}
	// Validate currency
	if currency, ok := updates["currency"].(string); ok {
		validCurrencies := []string{"IDR", "USD", "EUR", "SGD", "MYR"}
		isValid := false
		for _, curr := range validCurrencies {
			if currency == curr {
				isValid = true
				break
			}
		}
		if !isValid {
			return errors.New("invalid currency code")
		}
	}
	// Validate prefixes (non-empty and length)
	prefixes := []string{"invoice_prefix", "sales_prefix", "quote_prefix", "purchase_prefix", "journal_prefix", "payment_receivable_prefix", "payment_payable_prefix"}
	for _, prefix := range prefixes {
		if value, ok := updates[prefix].(string); ok {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("%s cannot be empty", prefix)
			}
			if len(value) > 10 {
				return fmt.Errorf("%s cannot be longer than 10 characters", prefix)
			}
		}
	}
	// Other next-number counters are validated at their domain-specific update paths.
	return nil
}

func (s *SettingsService) validateSalesNextNumberUpdate(settings *models.Settings, updates map[string]interface{}) error {
	value, exists := updates["sales_next_number"]
	if !exists {
		return nil
	}

	nextNumber, err := normalizePositiveCounter(value)
	if err != nil {
		return errors.New("Nomor counter penjualan harus berupa angka bulat minimal 1.")
	}

	prefix := settings.SalesPrefix
	if updatedPrefix, ok := updates["sales_prefix"].(string); ok {
		prefix = strings.TrimSpace(updatedPrefix)
	}
	if prefix == "" {
		return errors.New("Prefix kode penjualan tidak boleh kosong.")
	}

	usedCode := formatInvoiceNumber(prefix, nextNumber)
	isUsed, err := s.isSalesCodeUsed(usedCode)
	if err != nil {
		log.Printf("Error validating sales_next_number for prefix %s: %v", prefix, err)
		return errors.New("Gagal memeriksa kode penjualan yang sudah dipakai. Silakan coba lagi.")
	}

	if isUsed {
		nextAvailableNumber, findErr := s.findNextAvailableSalesNumber(prefix, nextNumber+1)
		if findErr != nil {
			log.Printf("Error finding next available sales number for prefix %s: %v", prefix, findErr)
			return fmt.Errorf("Kode penjualan %s sudah pernah dipakai. Gunakan nomor lain yang belum dipakai.", usedCode)
		}
		nextAvailableCode := formatInvoiceNumber(prefix, nextAvailableNumber)
		return fmt.Errorf(
			"Kode penjualan %s sudah pernah dipakai. Gunakan nomor %d atau nomor lain yang belum dipakai agar kode berikutnya menjadi %s.",
			usedCode,
			nextAvailableNumber,
			nextAvailableCode,
		)
	}

	updates["sales_next_number"] = nextNumber
	return nil
}

func normalizePositiveCounter(value interface{}) (int, error) {
	var number float64

	switch typedValue := value.(type) {
	case int:
		number = float64(typedValue)
	case int32:
		number = float64(typedValue)
	case int64:
		number = float64(typedValue)
	case float32:
		number = float64(typedValue)
	case float64:
		number = typedValue
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typedValue))
		if err != nil {
			return 0, err
		}
		number = float64(parsed)
	default:
		return 0, fmt.Errorf("unsupported counter type %T", value)
	}

	if math.Trunc(number) != number || number < 1 {
		return 0, errors.New("counter must be a whole number greater than zero")
	}

	return int(number), nil
}

func (s *SettingsService) isSalesCodeUsed(code string) (bool, error) {
	var count int64
	err := s.db.Table("sales").Where("code = ?", code).Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (s *SettingsService) findNextAvailableSalesNumber(prefix string, startNumber int) (int, error) {
	if startNumber < 1 {
		startNumber = 1
	}

	for candidate := startNumber; candidate < startNumber+100000; candidate++ {
		code := formatInvoiceNumber(prefix, candidate)
		isUsed, err := s.isSalesCodeUsed(code)
		if err != nil {
			return 0, err
		}
		if !isUsed {
			return candidate, nil
		}
	}

	return 0, fmt.Errorf("unable to find available sales number for prefix %s", prefix)
}

// createDefaultSettings creates default system settings
func (s *SettingsService) createDefaultSettings() models.Settings {
	// Load unified config for defaults
	cfg := config.GetAccountingConfig()
	journalPrefix := "JE"
	requireApproval := false
	if cfg != nil {
		if cfg.JournalSettings.CodePrefix != "" {
			journalPrefix = cfg.JournalSettings.CodePrefix
		}
		requireApproval = cfg.JournalSettings.RequireApproval
	}
	return models.Settings{
		CompanyName:             "PT. Sistem Akuntansi Indonesia",
		CompanyAddress:          "Jl. Sudirman Kav. 45-46, Jakarta Pusat 10210, Indonesia",
		CompanyPhone:            "+62-21-5551234",
		CompanyEmail:            "info@sistemakuntansi.co.id",
		CompanyLogoScale:        100,
		Currency:                ifThen(cfg != nil && cfg.CurrencySettings.BaseCurrency != "", cfg.CurrencySettings.BaseCurrency, "IDR"),
		DateFormat:              "DD/MM/YYYY",
		FiscalYearStart:         "January 1",
		Language:                "id",
		Timezone:                "Asia/Jakarta",
		ThousandSeparator:       ".",
		DecimalSeparator:        ",",
		DecimalPlaces:           ifThenInt(cfg != nil && cfg.CurrencySettings.DecimalPlaces != 0, cfg.CurrencySettings.DecimalPlaces, 2),
		DefaultTaxRate:          ifThenFloat(cfg != nil && cfg.TaxRates.DefaultPPN != 0, cfg.TaxRates.DefaultPPN, 11.0),
		InvoicePrefix:           "INV",
		SalesPrefix:             "SOA",
		QuotePrefix:             "QT",
		PurchasePrefix:          "PO",
		PaymentReceivablePrefix: "RCV",
		PaymentPayablePrefix:    "PAY",
		JournalPrefix:           journalPrefix,
		JournalNextNumber:       1,
		RequireJournalApproval:  requireApproval,
	}
}

// Utility inline helpers for default selection
func ifThen(cond bool, a string, b string) string {
	if cond {
		return a
	}
	return b
}
func ifThenInt(cond bool, a int, b int) int {
	if cond {
		return a
	}
	return b
}
func ifThenFloat(cond bool, a float64, b float64) float64 {
	if cond {
		return a
	}
	return b
}

// GetNextInvoiceNumber gets and increments the next invoice number
func (s *SettingsService) GetNextInvoiceNumber() (string, error) {
	var settings models.Settings

	// Start transaction
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Lock the settings row for update
	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&settings).Error; err != nil {
		tx.Rollback()
		return "", err
	}

	// Get current number and format
	invoiceNumber := formatInvoiceNumber(settings.InvoicePrefix, settings.InvoiceNextNumber)

	// Increment the number
	settings.InvoiceNextNumber++
	if err := tx.Save(&settings).Error; err != nil {
		tx.Rollback()
		return "", err
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return "", err
	}

	return invoiceNumber, nil
}

// GetNextQuoteNumber gets and increments the next quote number
func (s *SettingsService) GetNextQuoteNumber() (string, error) {
	var settings models.Settings

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&settings).Error; err != nil {
		tx.Rollback()
		return "", err
	}

	quoteNumber := formatInvoiceNumber(settings.QuotePrefix, settings.QuoteNextNumber)
	settings.QuoteNextNumber++

	if err := tx.Save(&settings).Error; err != nil {
		tx.Rollback()
		return "", err
	}

	if err := tx.Commit().Error; err != nil {
		return "", err
	}

	return quoteNumber, nil
}

// GetNextSalesNumber gets and increments the next sales number
func (s *SettingsService) GetNextSalesNumber() (string, error) {
	var settings models.Settings

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&settings).Error; err != nil {
		tx.Rollback()
		return "", err
	}

	nextNumber, err := s.findNextAvailableSalesNumber(settings.SalesPrefix, settings.SalesNextNumber)
	if err != nil {
		tx.Rollback()
		return "", err
	}

	salesNumber := formatInvoiceNumber(settings.SalesPrefix, nextNumber)
	settings.SalesNextNumber = nextNumber + 1

	if err := tx.Save(&settings).Error; err != nil {
		tx.Rollback()
		return "", err
	}

	if err := tx.Commit().Error; err != nil {
		return "", err
	}

	return salesNumber, nil
}

// GetNextPurchaseNumber gets and increments the next purchase order number
func (s *SettingsService) GetNextPurchaseNumber() (string, error) {
	var settings models.Settings

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&settings).Error; err != nil {
		tx.Rollback()
		return "", err
	}

	purchaseNumber := formatInvoiceNumber(settings.PurchasePrefix, settings.PurchaseNextNumber)
	settings.PurchaseNextNumber++

	if err := tx.Save(&settings).Error; err != nil {
		tx.Rollback()
		return "", err
	}

	if err := tx.Commit().Error; err != nil {
		return "", err
	}

	return purchaseNumber, nil
}

// Helper functions

func isValidEmail(email string) bool {
	// Simple email validation
	if len(email) < 3 || len(email) > 254 {
		return false
	}

	atIndex := -1
	for i, char := range email {
		if char == '@' {
			if atIndex != -1 {
				return false // Multiple @ symbols
			}
			atIndex = i
		}
	}

	if atIndex <= 0 || atIndex >= len(email)-1 {
		return false
	}

	return true
}

func formatInvoiceNumber(prefix string, number int) string {
	// Format: PREFIX-YYYY-MM-00001
	// You can customize this format as needed
	return prefix + "-" + formatNumberWithPadding(number, 5)
}

func formatNumberWithPadding(number int, padding int) string {
	format := "%0" + string(rune(padding+'0')) + "d"
	return fmt.Sprintf(format, number)
}

// ResetToDefaults resets all settings to default values
func (s *SettingsService) ResetToDefaults(userID uint) error {
	var settings models.Settings

	// Get existing settings or create if not exists
	err := s.db.First(&settings).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			settings = s.createDefaultSettings()
			if err := s.db.Create(&settings).Error; err != nil {
				log.Printf("Error creating default settings: %v", err)
				return err
			}
			return nil
		}
		log.Printf("Error fetching settings: %v", err)
		return err
	}

	// Reset to default values
	defaultSettings := s.createDefaultSettings()

	// Keep the ID and preserve existing next numbers to avoid conflicts
	defaultSettings.ID = settings.ID
	defaultSettings.CreatedAt = settings.CreatedAt
	defaultSettings.InvoiceNextNumber = settings.InvoiceNextNumber
	defaultSettings.SalesNextNumber = settings.SalesNextNumber
	defaultSettings.QuoteNextNumber = settings.QuoteNextNumber
	defaultSettings.PurchaseNextNumber = settings.PurchaseNextNumber
	defaultSettings.UpdatedBy = userID

	// Save the reset settings
	if err := s.db.Save(&defaultSettings).Error; err != nil {
		log.Printf("Error resetting settings to defaults: %v", err)
		return err
	}

	log.Printf("Settings reset to defaults by user %d", userID)
	return nil
}

// GetNextJournalNumberTx generates next journal number within a transaction (safe sequence)
func (s *SettingsService) GetNextJournalNumberTx(tx *gorm.DB) (string, error) {
	var settingsForUpdate models.Settings
	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&settingsForUpdate).Error; err != nil {
		return "", err
	}
	code := fmt.Sprintf("%s-%05d", settingsForUpdate.JournalPrefix, settingsForUpdate.JournalNextNumber)
	settingsForUpdate.JournalNextNumber++
	if err := tx.Save(&settingsForUpdate).Error; err != nil {
		return "", err
	}
	// Invalidate cache since sequence changed
	settingsCacheMutex.Lock()
	settingsCache = nil
	settingsCacheMutex.Unlock()
	return code, nil
}

// normalizeFiscalYearStart normalizes various inputs to a standard format like "January 1"
func (s *SettingsService) normalizeFiscalYearStart(input string) string {
	in := strings.TrimSpace(strings.ToLower(input))
	// Accept formats: "mm/dd/yyyy", "mm/dd", "dd/mm/yyyy", month name + day
	months := []string{"january", "february", "march", "april", "may", "june", "july", "august", "september", "october", "november", "december"}
	for _, m := range months {
		if strings.Contains(in, m) {
			// e.g., "january 1" -> capitalize
			return strings.Title(m) + " " + extractDay(in)
		}
	}
	// Try MM/DD or DD/MM
	parts := strings.FieldsFunc(in, func(r rune) bool { return r == '/' || r == '-' })
	if len(parts) >= 2 {
		mm := parts[0]
		dd := parts[1]
		// If year exists, ignore
		monthIdx := toInt(mm)
		day := toInt(dd)
		if monthIdx >= 1 && monthIdx <= 12 && day >= 1 && day <= 31 {
			return strings.Title(months[monthIdx-1]) + fmt.Sprintf(" %d", day)
		}
	}
	// Fallback
	return "January 1"
}

// GetCurrentFiscalYearRange returns ISO start and end date for the current fiscal year based on settings
func (s *SettingsService) GetCurrentFiscalYearRange() (string, string, error) {
	settings, err := s.GetSettings()
	if err != nil {
		return "", "", err
	}
	startISO, endISO := computeFiscalYearRangeISO(time.Now(), settings.FiscalYearStart)
	return startISO, endISO, nil
}

// computeFiscalYearRangeISO computes fiscal year start/end ISO strings using a reference date and a Month Day string (e.g., "January 1")
func computeFiscalYearRangeISO(ref time.Time, fiscalStart string) (string, string) {
	months := map[string]int{
		"january": 1, "february": 2, "march": 3, "april": 4, "may": 5, "june": 6,
		"july": 7, "august": 8, "september": 9, "october": 10, "november": 11, "december": 12,
	}
	fs := strings.TrimSpace(strings.ToLower(fiscalStart))
	// Normalize common formats like "January 1" or "01/01"
	month := 1
	day := 1
	// Month name
	for name, idx := range months {
		if strings.HasPrefix(fs, name) {
			month = idx
			day = toFirstNumber(fs)
			break
		}
	}
	// If not matched, try numeric
	if month == 1 && day == 1 && !strings.HasPrefix(fs, "january") {
		parts := strings.FieldsFunc(fs, func(r rune) bool { return r == '/' || r == '-' || r == ' ' })
		if len(parts) >= 2 {
			m := toInt(parts[0])
			d := toInt(parts[1])
			if m >= 1 && m <= 12 && d >= 1 && d <= 31 {
				month = m
				day = d
			}
		}
	}
	// Determine year boundaries relative to reference date
	y := ref.Year()
	fyStartThisYear := time.Date(y, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	var start time.Time
	if ref.Before(fyStartThisYear) {
		start = time.Date(y-1, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	} else {
		start = fyStartThisYear
	}
	nextStart := start.AddDate(1, 0, 0)
	end := nextStart.AddDate(0, 0, -1)
	startISO := start.Format("2006-01-02")
	endISO := end.Format("2006-01-02")
	return startISO, endISO
}

// toFirstNumber extracts the first number from string or returns 1
func toFirstNumber(s string) int {
	for i := 1; i <= 31; i++ {
		if strings.Contains(s, fmt.Sprintf("%d", i)) {
			return i
		}
	}
	return 1
}

func extractDay(s string) string {
	for i := 1; i <= 31; i++ {
		if strings.Contains(s, fmt.Sprintf("%d", i)) {
			return fmt.Sprintf("%d", i)
		}
	}
	return "1"
}

func toInt(s string) int {
	var n int
	for _, r := range s {
		if r < '0' || r > '9' {
			return -1
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// GetValidationRules returns validation rules for settings
func (s *SettingsService) GetValidationRules() map[string]interface{} {
	return map[string]interface{}{
		"date_formats":         []string{"DD/MM/YYYY", "MM/DD/YYYY", "YYYY-MM-DD", "DD-MM-YYYY"},
		"currencies":           []string{"IDR", "USD", "EUR", "SGD", "MYR"},
		"languages":            []string{"id", "en"},
		"tax_rate_range":       map[string]float64{"min": 0, "max": 100},
		"decimal_places_range": map[string]int{"min": 0, "max": 4},
		"prefix_max_length":    10,
		"min_next_number":      1,
	}
}

type SettingsHistoryResult struct {
	Data       []models.SettingsHistoryResponse `json:"data"`
	Total      int64                            `json:"total"`
	Page       int                              `json:"page"`
	Limit      int                              `json:"limit"`
	TotalPages int                              `json:"total_pages"`
}

// GetSettingsHistory retrieves paginated settings history
func (s *SettingsService) GetSettingsHistory(filter models.SettingsHistoryFilter) (*SettingsHistoryResult, error) {
	var history []models.SettingsHistory
	var total int64

	query := s.db.Model(&models.SettingsHistory{}).Preload("User")

	// Apply filters
	if filter.Field != "" {
		query = query.Where("field = ?", filter.Field)
	}
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.ChangedBy != "" {
		query = query.Where("changed_by = ?", filter.ChangedBy)
	}
	if filter.StartDate != "" {
		query = query.Where("created_at >= ?", filter.StartDate)
	}
	if filter.EndDate != "" {
		query = query.Where("created_at <= ?", filter.EndDate)
	}

	// Count total records
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count settings history: %v", err)
	}

	// Apply pagination
	offset := (filter.Page - 1) * filter.Limit
	if err := query.Offset(offset).Limit(filter.Limit).Order("created_at DESC").Find(&history).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve settings history: %v", err)
	}

	totalPages := int(math.Ceil(float64(total) / float64(filter.Limit)))

	// Convert to response format
	responses := make([]models.SettingsHistoryResponse, len(history))
	for i, h := range history {
		responses[i] = h.ToResponse()
	}

	return &SettingsHistoryResult{
		Data:       responses,
		Total:      total,
		Page:       filter.Page,
		Limit:      filter.Limit,
		TotalPages: totalPages,
	}, nil
}

// Helper methods for audit logging

// captureCurrentValues captures current values of fields that will be updated
func (s *SettingsService) captureCurrentValues(settings *models.Settings, updates map[string]interface{}) map[string]interface{} {
	oldValues := make(map[string]interface{})

	v := reflect.ValueOf(*settings)
	t := reflect.TypeOf(*settings)

	for key := range updates {
		// Skip meta fields
		if key == "updated_by" || key == "updated_at" {
			continue
		}

		// Find field in struct
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			jsonTag := field.Tag.Get("json")

			// Parse json tag to get field name
			fieldName := jsonTag
			if commaIdx := len(jsonTag); commaIdx > 0 {
				if idx := len(jsonTag); idx > 0 {
					for j, char := range jsonTag {
						if char == ',' {
							fieldName = jsonTag[:j]
							break
						}
					}
				}
			}

			if fieldName == key {
				oldValues[key] = v.Field(i).Interface()
				break
			}
		}
	}

	return oldValues
}

// logSettingsChanges logs changes to settings_history table
func (s *SettingsService) logSettingsChanges(settingsID uint, oldValues, newValues map[string]interface{}, userID uint, action string) {
	for field, newValue := range newValues {
		// Skip meta fields
		if field == "updated_by" || field == "updated_at" {
			continue
		}

		oldValue := oldValues[field]

		// Skip if values are the same
		if reflect.DeepEqual(oldValue, newValue) {
			continue
		}

		// Convert values to JSON strings
		oldValueJSON, _ := json.Marshal(oldValue)
		newValueJSON, _ := json.Marshal(newValue)

		// Create history record
		history := models.SettingsHistory{
			SettingsID: settingsID,
			Field:      field,
			OldValue:   string(oldValueJSON),
			NewValue:   string(newValueJSON),
			Action:     action,
			ChangedBy:  userID,
		}

		// Save history record
		if err := s.db.Create(&history).Error; err != nil {
			log.Printf("Error logging settings change for field %s: %v", field, err)
		}
	}
}
