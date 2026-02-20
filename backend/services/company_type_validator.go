package services

import (
	"log"
	"sync"
	"time"

	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

// CompanyTypeValidator provides company-type-aware validation
type CompanyTypeValidator struct {
	db *gorm.DB
}

// Cached company type (process-local, TTL-based)
var (
	cachedCompanyType   string
	companyTypeCacheAt  time.Time
	companyTypeCacheMu  sync.RWMutex
	companyTypeCacheTTL = 10 * time.Minute
)

// NewCompanyTypeValidator creates a new validator
func NewCompanyTypeValidator(db *gorm.DB) *CompanyTypeValidator {
	return &CompanyTypeValidator{db: db}
}

// GetCompanyType returns the current company type from settings (cached)
func (v *CompanyTypeValidator) GetCompanyType() string {
	companyTypeCacheMu.RLock()
	if cachedCompanyType != "" && time.Since(companyTypeCacheAt) < companyTypeCacheTTL {
		ct := cachedCompanyType
		companyTypeCacheMu.RUnlock()
		return ct
	}
	companyTypeCacheMu.RUnlock()

	// Cache miss - fetch from DB
	companyTypeCacheMu.Lock()
	defer companyTypeCacheMu.Unlock()

	// Double-check after acquiring write lock
	if cachedCompanyType != "" && time.Since(companyTypeCacheAt) < companyTypeCacheTTL {
		return cachedCompanyType
	}

	var settings models.Settings
	if err := v.db.First(&settings).Error; err != nil {
		log.Printf("⚠️ Failed to get company type from settings: %v", err)
		return ""
	}

	cachedCompanyType = settings.CompanyType
	companyTypeCacheAt = time.Now()
	return cachedCompanyType
}

// IsServiceCompany returns true if the company is a service company
func (v *CompanyTypeValidator) IsServiceCompany() bool {
	return v.GetCompanyType() == models.CompanyTypeService
}

// IsTradingCompany returns true if the company is a trading company
func (v *CompanyTypeValidator) IsTradingCompany() bool {
	return v.GetCompanyType() == models.CompanyTypeTrading
}

// IsManufacturingCompany returns true if the company is a manufacturing company
func (v *CompanyTypeValidator) IsManufacturingCompany() bool {
	return v.GetCompanyType() == models.CompanyTypeManufacturing
}

// IsInventoryAllowed returns true if the company type supports inventory tracking
func (v *CompanyTypeValidator) IsInventoryAllowed() bool {
	ct := v.GetCompanyType()
	return ct == models.CompanyTypeTrading || ct == models.CompanyTypeManufacturing
}

// GetPurchaseDebitAccountCode returns the appropriate debit account code for purchases
// SERVICE: uses expense account (5102-001 Biaya Proyek or 6102 Beban Administrasi)
// TRADING/MANUFACTURING: uses inventory account (1103-001 Persediaan)
func (v *CompanyTypeValidator) GetPurchaseDebitAccountCode() string {
	if v.IsServiceCompany() {
		return "5102-001" // Biaya Proyek (direct service cost)
	}
	return "1103-001" // Persediaan Barang Dagang
}

// InvalidateCache clears the cached company type (call after settings update)
func InvalidateCompanyTypeCache() {
	companyTypeCacheMu.Lock()
	defer companyTypeCacheMu.Unlock()
	cachedCompanyType = ""
}
