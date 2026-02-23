package services

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

type AssetServiceInterface interface {
	GetAllAssets() ([]models.Asset, error)
	GetAssetByID(id uint) (*models.Asset, error)
	CreateAsset(asset *models.Asset) error
	CreateAssetWithJournal(asset *models.Asset, userId uint, paymentMethod string, paymentAccountID *uint, creditAccountID *uint) error
	UpdateAsset(asset *models.Asset) error
	DeleteAsset(id uint) error
	GenerateAssetCode(category string) (string, error)
	CalculateDepreciation(asset *models.Asset, asOfDate time.Time) (float64, error)
	GetDepreciationSchedule(asset *models.Asset) ([]DepreciationEntry, error)
	GetAssetsSummary() (*AssetsSummary, error)
	GetAssetsForDepreciationReport() ([]AssetDepreciationReport, error)
	CreateDepreciationJournalEntry(asset *models.Asset, depreciationAmount float64, userId uint, entryDate time.Time) error
	GetAssetCategories() ([]models.AssetCategory, error)
	CreateAssetCategory(category *models.AssetCategory) error
}

type AssetService struct {
	assetRepo repositories.AssetRepositoryInterface
	db        *gorm.DB
}

type DepreciationEntry struct {
	Year                    int       `json:"year"`
	Date                    time.Time `json:"date"`
	DepreciationCost        float64   `json:"depreciation_cost"`
	AccumulatedDepreciation float64   `json:"accumulated_depreciation"`
	BookValue               float64   `json:"book_value"`
}

type AssetsSummary struct {
	TotalAssets       int64   `json:"total_assets"`
	ActiveAssets      int64   `json:"active_assets"`
	TotalValue        float64 `json:"total_value"`
	TotalDepreciation float64 `json:"total_depreciation"`
	NetBookValue      float64 `json:"net_book_value"`
}

type AssetDepreciationReport struct {
	Asset                 models.Asset `json:"asset"`
	AnnualDepreciation    float64      `json:"annual_depreciation"`
	MonthlyDepreciation   float64      `json:"monthly_depreciation"`
	RemainingDepreciation float64      `json:"remaining_depreciation"`
	RemainingYears        int          `json:"remaining_years"`
	CurrentBookValue      float64      `json:"current_book_value"`
}

func NewAssetService(assetRepo repositories.AssetRepositoryInterface, db *gorm.DB) AssetServiceInterface {
	return &AssetService{
		assetRepo: assetRepo,
		db:        db,
	}
}

// GetAllAssets retrieves all assets
func (s *AssetService) GetAllAssets() ([]models.Asset, error) {
	return s.assetRepo.FindAll()
}

// GetAssetByID retrieves asset by ID
func (s *AssetService) GetAssetByID(id uint) (*models.Asset, error) {
	return s.assetRepo.FindByID(id)
}

// CreateAsset creates a new asset with generated code
func (s *AssetService) CreateAsset(asset *models.Asset) error {
	// Generate asset code OUTSIDE transaction to avoid transaction abort issues
	if asset.Code == "" {
		code, err := s.GenerateAssetCode(asset.Category)
		if err != nil {
			return err
		}
		asset.Code = code
	}

	// Set default status
	if asset.Status == "" {
		asset.Status = models.AssetStatusActive
	}

	// Validate purchase date
	if asset.PurchaseDate.IsZero() {
		return errors.New("purchase date is required")
	}

	// Validate purchase price
	if asset.PurchasePrice <= 0 {
		return errors.New("purchase price must be greater than 0")
	}

	// Validate useful life for depreciable assets
	if asset.UsefulLife <= 0 && asset.DepreciationMethod != "" {
		return errors.New("useful life must be greater than 0 for depreciable assets")
	}

	// Calculate initial depreciation if needed
	if asset.AccumulatedDepreciation == 0 && asset.DepreciationMethod != "" {
		depreciation, err := s.CalculateDepreciation(asset, time.Now())
		if err == nil {
			asset.AccumulatedDepreciation = depreciation
		}
	}

	// Retry logic for handling unique constraint violations
	const maxRetries = 5
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := s.assetRepo.Create(asset)
		if err == nil {
			// Success!
			return nil
		}

		// Check if it's a unique constraint violation on asset code
		if isUniqueCodeError(err) && attempt < maxRetries {
			// Generate new code and retry
			newCode, codeErr := s.GenerateAssetCode(asset.Category)
			if codeErr != nil {
				return errors.New("failed to generate new asset code after collision: " + codeErr.Error())
			}
			asset.Code = newCode
			continue // Retry with new code
		}

		// If it's not a unique constraint error or we've exhausted retries, return the error
		return err
	}

	// Should never reach here, but just in case
	return errors.New("exhausted all retry attempts")
}

// CreateAssetWithJournal creates a new asset and generates corresponding journal entries
func (s *AssetService) CreateAssetWithJournal(asset *models.Asset, userId uint, paymentMethod string, paymentAccountID *uint, creditAccountID *uint) error {
	// Generate asset code OUTSIDE transaction to avoid transaction abort issues
	if asset.Code == "" {
		code, err := s.GenerateAssetCode(asset.Category)
		if err != nil {
			return err
		}
		asset.Code = code
	}

	// Set default status
	if asset.Status == "" {
		asset.Status = models.AssetStatusActive
	}

	// Validate purchase date
	if asset.PurchaseDate.IsZero() {
		return errors.New("purchase date is required")
	}

	// Validate purchase price
	if asset.PurchasePrice <= 0 {
		return errors.New("purchase price must be greater than 0")
	}

	// Validate useful life for depreciable assets
	if asset.UsefulLife <= 0 && asset.DepreciationMethod != "" {
		return errors.New("useful life must be greater than 0 for depreciable assets")
	}

	// Calculate initial depreciation if needed
	if asset.AccumulatedDepreciation == 0 && asset.DepreciationMethod != "" {
		depreciation, err := s.CalculateDepreciation(asset, time.Now())
		if err == nil {
			asset.AccumulatedDepreciation = depreciation
		}
	}

	// Retry logic with fresh transaction for each attempt
	const maxRetries = 5
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Start a fresh transaction for each attempt
		tx := s.db.Begin()
		if tx.Error != nil {
			return tx.Error
		}

		// Try to create the asset
		createErr := tx.Create(asset).Error
		if createErr == nil {
			// Success! Generate and create journal entry
			journalEntry := models.GenerateAssetJournalEntry(*asset, userId, paymentMethod, paymentAccountID, creditAccountID)
			if err := tx.Create(journalEntry).Error; err != nil {
				tx.Rollback()
				return err
			}

			// Update account balances
			if err := s.updateAccountBalances(tx, journalEntry); err != nil {
				tx.Rollback()
				return err
			}

			// Commit transaction and return success
			return tx.Commit().Error
		}

		// Rollback failed transaction immediately
		tx.Rollback()

		// Check if it's a unique constraint violation on asset code
		if isUniqueCodeError(createErr) && attempt < maxRetries {
			// Generate new code and retry with fresh transaction
			newCode, codeErr := s.GenerateAssetCode(asset.Category)
			if codeErr != nil {
				return errors.New("failed to generate new asset code after collision: " + codeErr.Error())
			}
			asset.Code = newCode
			continue // Retry with new code and fresh transaction
		}

		// If it's not a unique constraint error or we've exhausted retries, return the error
		return createErr
	}

	// Should never reach here, but just in case
	return errors.New("exhausted all retry attempts")

}

// UpdateAsset updates an existing asset
func (s *AssetService) UpdateAsset(asset *models.Asset) error {
	// Validate purchase date
	if asset.PurchaseDate.IsZero() {
		return errors.New("purchase date is required")
	}

	// Validate purchase price
	if asset.PurchasePrice <= 0 {
		return errors.New("purchase price must be greater than 0")
	}

	// Recalculate depreciation if depreciation-related fields changed
	if asset.DepreciationMethod != "" && asset.UsefulLife > 0 {
		depreciation, err := s.CalculateDepreciation(asset, time.Now())
		if err == nil {
			asset.AccumulatedDepreciation = depreciation
		}
	}

	return s.assetRepo.Update(asset)
}

// DeleteAsset deletes an asset
func (s *AssetService) DeleteAsset(id uint) error {
	return s.assetRepo.Delete(id)
}

// GenerateAssetCode generates a unique asset code based on category
func (s *AssetService) GenerateAssetCode(category string) (string, error) {
	// Get category prefix with the following priority:
	// 1) If a matching AssetCategory exists in DB, use its Code as prefix
	// 2) Fallback to static mapping for known default categories
	prefix := s.getDynamicCategoryPrefix(category)
	if prefix == "" {
		prefix = getCategoryStaticPrefix(category)
	}
	if prefix == "" {
		prefix = "AS" // ultimate fallback
	}

	year := time.Now().Format("2006")

	// Start from sequence 1 and find the next available number
	for sequence := 1; sequence <= 9999; sequence++ {
		code := prefix + "-" + year + "-" + padLeft(strconv.Itoa(sequence), 3, "0")

		// Check if code exists in database
		_, err := s.assetRepo.FindByCode(code)
		if err != nil {
			// Code doesn't exist, we can use it
			return code, nil
		}
		// Code exists, try next sequence
	}

	// If we reach here, all sequences are exhausted
	return "", errors.New("unable to generate unique asset code: all sequences exhausted for category " + category)
}

// CalculateDepreciation calculates accumulated depreciation up to a specific date
func (s *AssetService) CalculateDepreciation(asset *models.Asset, asOfDate time.Time) (float64, error) {
	if asset.UsefulLife <= 0 || asset.PurchasePrice <= 0 {
		return 0, nil
	}

	// Calculate months since purchase
	monthsSincePurchase := monthsDifference(asset.PurchaseDate, asOfDate)
	if monthsSincePurchase <= 0 {
		return 0, nil
	}

	depreciableAmount := asset.PurchasePrice - asset.SalvageValue

	switch asset.DepreciationMethod {
	case models.DepreciationMethodStraightLine:
		return s.calculateStraightLineDepreciation(depreciableAmount, asset.UsefulLife, monthsSincePurchase), nil
	case models.DepreciationMethodDecliningBalance:
		return s.calculateDecliningBalanceDepreciation(asset.PurchasePrice, asset.SalvageValue, asset.UsefulLife, monthsSincePurchase), nil
	default:
		return s.calculateStraightLineDepreciation(depreciableAmount, asset.UsefulLife, monthsSincePurchase), nil
	}
}

// GetDepreciationSchedule generates depreciation schedule for an asset
func (s *AssetService) GetDepreciationSchedule(asset *models.Asset) ([]DepreciationEntry, error) {
	if asset.UsefulLife <= 0 || asset.PurchasePrice <= 0 {
		return []DepreciationEntry{}, nil
	}

	var schedule []DepreciationEntry

	for year := 1; year <= asset.UsefulLife; year++ {
		date := asset.PurchaseDate.AddDate(year-1, 0, 0)

		depreciation, _ := s.CalculateDepreciation(asset, date.AddDate(1, 0, -1))

		var annualDepreciation float64
		if year == 1 {
			annualDepreciation = depreciation
		} else {
			prevDepreciation, _ := s.CalculateDepreciation(asset, date.AddDate(0, 0, -1))
			annualDepreciation = depreciation - prevDepreciation
		}

		bookValue := asset.PurchasePrice - depreciation
		if bookValue < asset.SalvageValue {
			bookValue = asset.SalvageValue
		}

		entry := DepreciationEntry{
			Year:                    year,
			Date:                    date,
			DepreciationCost:        annualDepreciation,
			AccumulatedDepreciation: depreciation,
			BookValue:               bookValue,
		}

		schedule = append(schedule, entry)

		// Stop if we've reached salvage value
		if bookValue <= asset.SalvageValue {
			break
		}
	}

	return schedule, nil
}

// GetAssetsSummary returns summary statistics for all assets
func (s *AssetService) GetAssetsSummary() (*AssetsSummary, error) {
	totalAssets, err := s.assetRepo.Count()
	if err != nil {
		return nil, err
	}

	activeAssets, err := s.assetRepo.GetActiveAssets()
	if err != nil {
		return nil, err
	}

	totalValue, err := s.assetRepo.GetTotalValue()
	if err != nil {
		return nil, err
	}

	// Calculate total depreciation and net book value
	var totalDepreciation, netBookValue float64
	for _, asset := range activeAssets {
		totalDepreciation += asset.AccumulatedDepreciation
		netBookValue += (asset.PurchasePrice - asset.AccumulatedDepreciation)
	}

	return &AssetsSummary{
		TotalAssets:       totalAssets,
		ActiveAssets:      int64(len(activeAssets)),
		TotalValue:        totalValue,
		TotalDepreciation: totalDepreciation,
		NetBookValue:      netBookValue,
	}, nil
}

// GetAssetsForDepreciationReport returns depreciation report for all assets
func (s *AssetService) GetAssetsForDepreciationReport() ([]AssetDepreciationReport, error) {
	assets, err := s.assetRepo.GetAssetsForDepreciation()
	if err != nil {
		return nil, err
	}

	var reports []AssetDepreciationReport

	for _, asset := range assets {
		// Calculate annual depreciation
		depreciableAmount := asset.PurchasePrice - asset.SalvageValue
		var annualDepreciation float64

		if asset.DepreciationMethod == models.DepreciationMethodStraightLine {
			annualDepreciation = depreciableAmount / float64(asset.UsefulLife)
		} else {
			// For declining balance, use first year depreciation as estimate
			annualDepreciation = depreciableAmount * 0.2 // 20% declining balance
		}

		monthlyDepreciation := annualDepreciation / 12
		currentBookValue := asset.PurchasePrice - asset.AccumulatedDepreciation
		remainingDepreciation := math.Max(0, currentBookValue-asset.SalvageValue)

		var remainingYears int
		if annualDepreciation > 0 {
			remainingYears = int(math.Ceil(remainingDepreciation / annualDepreciation))
		}

		report := AssetDepreciationReport{
			Asset:                 asset,
			AnnualDepreciation:    annualDepreciation,
			MonthlyDepreciation:   monthlyDepreciation,
			RemainingDepreciation: remainingDepreciation,
			RemainingYears:        remainingYears,
			CurrentBookValue:      currentBookValue,
		}

		reports = append(reports, report)
	}

	return reports, nil
}

// Helper functions

// getCategoryStaticPrefix provides default prefixes for built-in categories.
func getCategoryStaticPrefix(category string) string {
	switch category {
	case "Fixed Asset":
		return "FA"
	case "Real Estate":
		return "RE"
	case "Computer", "Computer Equipment": // Support both variations
		return "CE"
	case "Vehicle":
		return "VH"
	case "Office Equipment":
		return "OE"
	case "Furniture":
		return "FR"
	case "IT Infrastructure":
		return "IT"
	case "Machinery":
		return "MC"
	default:
		return ""
	}
}

// getDynamicCategoryPrefix tries to read prefix(code) from asset_categories table matching the given name.
func (s *AssetService) getDynamicCategoryPrefix(category string) string {
	if strings.TrimSpace(category) == "" {
		return ""
	}
	var cat models.AssetCategory
	// Case-insensitive name match; prefer active categories
	err := s.db.Where("LOWER(name) = LOWER(?) AND is_active = ?", category, true).First(&cat).Error
	if err != nil {
		// Category not found in database, return empty string to fallback to static prefix
		return ""
	}
	code := strings.TrimSpace(cat.Code)
	if code == "" {
		return ""
	}
	// Sanitize: keep only letters/numbers and dash, uppercase
	code = strings.ToUpper(code)
	return code
}

func padLeft(str string, length int, pad string) string {
	for len(str) < length {
		str = pad + str
	}
	return str
}

func monthsDifference(startDate, endDate time.Time) int {
	if endDate.Before(startDate) {
		return 0
	}

	months := int(endDate.Month()) - int(startDate.Month())
	months += (endDate.Year() - startDate.Year()) * 12

	// Add partial month if end date day is >= start date day
	if endDate.Day() >= startDate.Day() {
		months++
	}

	return months
}

func (s *AssetService) calculateStraightLineDepreciation(depreciableAmount float64, usefulLifeYears int, monthsSincePurchase int) float64 {
	monthlyDepreciation := depreciableAmount / float64(usefulLifeYears*12)
	return monthlyDepreciation * float64(monthsSincePurchase)
}

func (s *AssetService) calculateDecliningBalanceDepreciation(purchasePrice, salvageValue float64, usefulLifeYears int, monthsSincePurchase int) float64 {
	rate := 2.0 / float64(usefulLifeYears) // Double declining balance
	monthlyRate := rate / 12

	accumulated := 0.0
	bookValue := purchasePrice

	for month := 0; month < monthsSincePurchase; month++ {
		monthlyDepreciation := bookValue * monthlyRate

		// Don't depreciate below salvage value
		if bookValue-monthlyDepreciation < salvageValue {
			monthlyDepreciation = bookValue - salvageValue
		}

		accumulated += monthlyDepreciation
		bookValue -= monthlyDepreciation

		// Stop if we reach salvage value
		if bookValue <= salvageValue {
			break
		}
	}

	return accumulated
}

// CreateDepreciationJournalEntry creates journal entry for asset depreciation
func (s *AssetService) CreateDepreciationJournalEntry(asset *models.Asset, depreciationAmount float64, userId uint, entryDate time.Time) error {
	// Start a database transaction
	tx := s.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()

	// Generate and create depreciation journal entry
	journalEntry := models.GenerateDepreciationJournalEntry(*asset, depreciationAmount, userId, entryDate)
	if err := tx.Create(journalEntry).Error; err != nil {
		return err
	}

	// Update account balances
	if err := s.updateAccountBalances(tx, journalEntry); err != nil {
		return err
	}

	// Update asset's accumulated depreciation
	asset.AccumulatedDepreciation += depreciationAmount
	if err := tx.Save(asset).Error; err != nil {
		return err
	}

	// Commit transaction
	return tx.Commit().Error
}

// updateAccountBalances updates the account balances based on journal entry lines
func (s *AssetService) updateAccountBalances(tx *gorm.DB, journalEntry *models.JournalEntry) error {
	// Load journal lines
	if err := tx.Preload("JournalLines").Find(journalEntry).Error; err != nil {
		return err
	}

	// Update account balances for each journal line
	for _, line := range journalEntry.JournalLines {
		// Get the account
		var account models.Account
		if err := tx.First(&account, line.AccountID).Error; err != nil {
			continue // Skip if account not found
		}

		// Calculate balance change based on normal balance type
		normalBalance := account.GetNormalBalance()
		var balanceChange float64

		if normalBalance == models.NormalBalanceDebit {
			// For debit accounts: debit increases, credit decreases
			balanceChange = line.DebitAmount - line.CreditAmount
		} else {
			// For credit accounts: credit increases, debit decreases
			balanceChange = line.CreditAmount - line.DebitAmount
		}

		// Update account balance
		account.Balance += balanceChange
		if err := tx.Model(&account).Select("Balance").Updates(models.Account{Balance: account.Balance}).Error; err != nil {
			return err
		}
	}

	return nil
}

// GetAssetCategories retrieves all asset categories
func (s *AssetService) GetAssetCategories() ([]models.AssetCategory, error) {
	var categories []models.AssetCategory
	err := s.db.Where("is_active = ?", true).Order("name ASC").Find(&categories).Error
	return categories, err
}

// CreateAssetCategory creates a new asset category
func (s *AssetService) CreateAssetCategory(category *models.AssetCategory) error {
	// Validate required fields
	if category.Name == "" {
		return errors.New("category name is required")
	}
	if category.Code == "" {
		return errors.New("category code is required")
	}

	// Set default values
	if !category.IsActive {
		category.IsActive = true
	}

	// Check for duplicate code or name
	var existingCategory models.AssetCategory
	if err := s.db.Where("code = ? OR name = ?", category.Code, category.Name).First(&existingCategory).Error; err == nil {
		return errors.New("category with this code or name already exists")
	}

	return s.db.Create(category).Error
}

// isUniqueCodeError checks if the error is due to unique constraint violation on asset code
func isUniqueCodeError(err error) bool {
	if err == nil {
		return false
	}
	errorStr := err.Error()
	// PostgreSQL unique violation - check for multiple possible constraint names
	return strings.Contains(errorStr, "SQLSTATE 23505") &&
		(strings.Contains(errorStr, "assets_code_key") ||
			strings.Contains(errorStr, "uni_assets_code") ||
			strings.Contains(errorStr, "assets_code_unique") ||
			strings.Contains(errorStr, "idx_assets_code"))
}
