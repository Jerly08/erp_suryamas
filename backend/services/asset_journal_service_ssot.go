package services

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// AssetJournalServiceSSOT handles ALL asset-related journal entries via unified_journal_ledger (SSOT).
// This is the single source of truth for:
// - Asset acquisition journals (standalone asset creation)
// - Depreciation journals (monthly)
// - Capitalization journals (from purchase receipt)
// - Reversal journals (asset deletion/disposal)
type AssetJournalServiceSSOT struct {
	db *gorm.DB
}

// NewAssetJournalServiceSSOT creates a new instance
func NewAssetJournalServiceSSOT(db *gorm.DB) *AssetJournalServiceSSOT {
	return &AssetJournalServiceSSOT{db: db}
}

// ── Account Resolution Helpers ─────────────────────────────────────────────

// assetCategoryToAccounts maps asset category names to COA account codes.
// Based on the standard Indonesian COA template in account_templates.go:
//
//	1201-xxx = Aset Tetap, 1202-xxx = Akumulasi Penyusutan
var assetCategoryAccountMap = map[string]struct {
	FixedAssetCode        string
	AccumulatedDeprecCode string
}{
	"bangunan":           {"1201-002", "1202-002"},
	"building":           {"1201-002", "1202-002"},
	"real estate":        {"1201-002", "1202-002"},
	"kendaraan":          {"1201-003", "1202-003"},
	"vehicle":            {"1201-003", "1202-003"},
	"peralatan":          {"1201-004", "1202-004"},
	"equipment":          {"1201-004", "1202-004"},
	"office equipment":   {"1201-004", "1202-004"},
	"computer equipment": {"1201-004", "1202-004"},
	"it infrastructure":  {"1201-004", "1202-004"},
	"furniture":          {"1201-004", "1202-004"},
	"mesin":              {"1201-005", "1202-005"},
	"machinery":          {"1201-005", "1202-005"},
	"tanah":              {"1201-001", ""}, // Tanah tidak disusutkan
	"land":               {"1201-001", ""},
}

// resolveAccountByCode finds an account by its code. Returns error if not found.
func (s *AssetJournalServiceSSOT) resolveAccountByCode(tx *gorm.DB, code string) (*models.Account, error) {
	var acc models.Account
	if err := tx.Where("code = ? AND deleted_at IS NULL", code).First(&acc).Error; err != nil {
		return nil, fmt.Errorf("account with code %s not found: %v", code, err)
	}
	return &acc, nil
}

// resolveAssetCategoryFromDB looks up the AssetCategory record from the database
// using CategoryID first, then falling back to name match.
func (s *AssetJournalServiceSSOT) resolveAssetCategoryFromDB(tx *gorm.DB, asset *models.Asset) *models.AssetCategory {
	var cat models.AssetCategory

	// Try by CategoryID first
	if asset.CategoryID != nil && *asset.CategoryID != 0 {
		if err := tx.First(&cat, *asset.CategoryID).Error; err == nil {
			return &cat
		}
	}

	// Fallback: match by name (case-insensitive)
	categoryName := strings.TrimSpace(asset.Category)
	if categoryName != "" {
		if err := tx.Where("LOWER(name) = LOWER(?) AND is_active = ? AND deleted_at IS NULL", categoryName, true).First(&cat).Error; err == nil {
			return &cat
		}
	}

	return nil
}

// resolveFixedAssetAccount resolves the fixed asset account for a given asset.
// Priority: asset.AssetAccountID > DB category mapping > hardcoded category mapping > default 1201-004 (Peralatan)
func (s *AssetJournalServiceSSOT) resolveFixedAssetAccount(tx *gorm.DB, asset *models.Asset) (*models.Account, error) {
	// 1. User-specified account on the asset record
	if asset.AssetAccountID != nil && *asset.AssetAccountID != 0 {
		var acc models.Account
		if err := tx.First(&acc, *asset.AssetAccountID).Error; err == nil {
			return &acc, nil
		}
		log.Printf("WARNING [ASSET-SSOT] Specified AssetAccountID %d not found, falling back to category mapping", *asset.AssetAccountID)
	}

	// 1.5. DB-based category mapping
	if dbCat := s.resolveAssetCategoryFromDB(tx, asset); dbCat != nil && dbCat.FixedAssetAccountCode != "" {
		acc, err := s.resolveAccountByCode(tx, dbCat.FixedAssetAccountCode)
		if err == nil {
			return acc, nil
		}
		log.Printf("WARNING [ASSET-SSOT] DB category fixed_asset_account_code '%s' not found for category '%s': %v", dbCat.FixedAssetAccountCode, dbCat.Name, err)
	}

	// 2. Hardcoded category-based mapping
	categoryLower := strings.ToLower(strings.TrimSpace(asset.Category))
	if mapping, exists := assetCategoryAccountMap[categoryLower]; exists && mapping.FixedAssetCode != "" {
		acc, err := s.resolveAccountByCode(tx, mapping.FixedAssetCode)
		if err == nil {
			return acc, nil
		}
		log.Printf("WARNING [ASSET-SSOT] Category account %s not found for category '%s': %v", mapping.FixedAssetCode, asset.Category, err)
	}

	// 3. Default: 1201-004 Peralatan
	acc, err := s.resolveAccountByCode(tx, "1201-004")
	if err != nil {
		// Last resort: any FIXED_ASSET leaf account
		var fallback models.Account
		if err2 := tx.Where("category = ? AND is_header = false AND is_active = true AND deleted_at IS NULL",
			models.CategoryFixedAsset).First(&fallback).Error; err2 == nil {
			return &fallback, nil
		}
		return nil, fmt.Errorf("no fixed asset account found in COA: %v. Please configure Fixed Asset Account Code on the Asset Category, or set asset_account_id on the asset record", err)
	}
	return acc, nil
}

// resolveAccumulatedDepreciationAccount resolves the accumulated depreciation account.
// Priority: asset.AccumulatedDepreciationAccountID > DB category mapping > hardcoded category mapping > default 1202-004
func (s *AssetJournalServiceSSOT) resolveAccumulatedDepreciationAccount(tx *gorm.DB, asset *models.Asset) (*models.Account, error) {
	categoryLower := strings.ToLower(strings.TrimSpace(asset.Category))

	// Tanah (land) should NOT have depreciation
	if categoryLower == "tanah" || categoryLower == "land" {
		return nil, fmt.Errorf("land assets (tanah) are not subject to depreciation per PSAK 16")
	}

	// 1. User-specified account on the asset record
	if asset.AccumulatedDepreciationAccountID != nil && *asset.AccumulatedDepreciationAccountID != 0 {
		var acc models.Account
		if err := tx.First(&acc, *asset.AccumulatedDepreciationAccountID).Error; err == nil {
			return &acc, nil
		}
		log.Printf("WARNING [ASSET-SSOT] Specified AccumulatedDepreciationAccountID %d not found, falling back to category mapping", *asset.AccumulatedDepreciationAccountID)
	}

	// 1.5. DB-based category mapping
	if dbCat := s.resolveAssetCategoryFromDB(tx, asset); dbCat != nil && dbCat.AccumulatedDepreciationAccountCode != "" {
		acc, err := s.resolveAccountByCode(tx, dbCat.AccumulatedDepreciationAccountCode)
		if err == nil {
			return acc, nil
		}
		log.Printf("WARNING [ASSET-SSOT] DB category accumulated_depreciation_account_code '%s' not found for category '%s': %v", dbCat.AccumulatedDepreciationAccountCode, dbCat.Name, err)
	}

	// 2. Hardcoded category-based mapping
	if mapping, exists := assetCategoryAccountMap[categoryLower]; exists && mapping.AccumulatedDeprecCode != "" {
		acc, err := s.resolveAccountByCode(tx, mapping.AccumulatedDeprecCode)
		if err == nil {
			return acc, nil
		}
		log.Printf("WARNING [ASSET-SSOT] Category account %s not found for accumulated depreciation, category '%s': %v", mapping.AccumulatedDeprecCode, asset.Category, err)
	}

	// 3. Default: 1202-004 Akumulasi Penyusutan Peralatan
	acc, err := s.resolveAccountByCode(tx, "1202-004")
	if err != nil {
		return nil, fmt.Errorf("no accumulated depreciation account found in COA: %v. Please configure accumulated_depreciation_account_code on the Asset Category, or ensure account 1202-004 exists", err)
	}
	return acc, nil
}

// resolveDepreciationExpenseAccount resolves the depreciation expense account.
// Priority: asset.DepreciationAccountID > 6104 (Service) or 6103 (Trading) > any DEPRECIATION_EXPENSE
func (s *AssetJournalServiceSSOT) resolveDepreciationExpenseAccount(tx *gorm.DB, asset *models.Asset) (*models.Account, error) {
	// 1. User-specified
	if asset.DepreciationAccountID != nil && *asset.DepreciationAccountID != 0 {
		var acc models.Account
		if err := tx.First(&acc, *asset.DepreciationAccountID).Error; err == nil {
			return &acc, nil
		}
		log.Printf("WARNING [ASSET-SSOT] Specified DepreciationAccountID %d not found for asset %s, falling back to default codes", *asset.DepreciationAccountID, asset.Code)
	}

	// 2. Try 6104 (Service company) first, then 6103 (Trading)
	for _, code := range []string{"6104", "6103"} {
		acc, err := s.resolveAccountByCode(tx, code)
		if err == nil {
			return acc, nil
		}
	}

	// 3. Any depreciation expense account (must be EXPENSE type)
	var acc models.Account
	if err := tx.Where("category = ? AND type = 'EXPENSE' AND is_header = false AND is_active = true AND deleted_at IS NULL",
		models.CategoryDepreciationExp).First(&acc).Error; err == nil {
		return &acc, nil
	}

	return nil, fmt.Errorf("depreciation expense account not found (tried 6104, 6103, category=%s). Please create a depreciation expense account in the Chart of Accounts", models.CategoryDepreciationExp)
}

// resolveCreditAccountForPayment resolves the credit account based on payment method.
func (s *AssetJournalServiceSSOT) resolveCreditAccountForPayment(tx *gorm.DB, paymentMethod string, paymentAccountID *uint, creditAccountID *uint) (*models.Account, error) {
	method := strings.ToUpper(strings.TrimSpace(paymentMethod))

	switch method {
	case "CASH", "TUNAI":
		if paymentAccountID != nil && *paymentAccountID != 0 {
			var acc models.Account
			if err := tx.First(&acc, *paymentAccountID).Error; err == nil {
				return &acc, nil
			}
		}
		return s.resolveAccountByCode(tx, "1101-002") // Kas Besar

	case "BANK", "TRANSFER":
		if paymentAccountID != nil && *paymentAccountID != 0 {
			var acc models.Account
			if err := tx.First(&acc, *paymentAccountID).Error; err == nil {
				return &acc, nil
			}
		}
		return s.resolveAccountByCode(tx, "1101-004") // Bank Tabungan

	case "CREDIT", "KREDIT", "HUTANG":
		if creditAccountID != nil && *creditAccountID != 0 {
			var acc models.Account
			if err := tx.First(&acc, *creditAccountID).Error; err == nil {
				return &acc, nil
			}
		}
		return s.resolveAccountByCode(tx, "2101-001") // Utang Usaha Lokal

	case "SALDO_AWAL", "OPENING":
		// Opening balance: credit to equity account (Modal Disetor / Laba Ditahan)
		if creditAccountID != nil && *creditAccountID != 0 {
			var acc models.Account
			if err := tx.First(&acc, *creditAccountID).Error; err == nil {
				return &acc, nil
			}
		}
		// Default: try 3101 Modal Disetor, then 3102 Laba Ditahan
		for _, code := range []string{"3101", "3102"} {
			acc, err := s.resolveAccountByCode(tx, code)
			if err == nil {
				return acc, nil
			}
		}
		// Last resort: any EQUITY leaf account
		var fallback models.Account
		if err := tx.Where("type = 'EQUITY' AND is_header = false AND is_active = true AND deleted_at IS NULL").
			First(&fallback).Error; err == nil {
			return &fallback, nil
		}
		return nil, fmt.Errorf("equity account not found for opening balance (tried 3101, 3102). Please create an equity account in the Chart of Accounts")

	default:
		// Default to AP (credit)
		log.Printf("⚠️ [ASSET-SSOT] Unknown payment method '%s', defaulting to Hutang Usaha", paymentMethod)
		return s.resolveAccountByCode(tx, "2101-001")
	}
}

// ── Journal Creation Methods ───────────────────────────────────────────────

// CreateAssetWithAcquisitionJournal atomically creates an asset record AND its acquisition journal
// in a single database transaction. If either fails, both are rolled back.
func (s *AssetJournalServiceSSOT) CreateAssetWithAcquisitionJournal(
	asset *models.Asset,
	userID uint,
	paymentMethod string,
	paymentAccountID *uint,
	creditAccountID *uint,
	assetService AssetServiceInterface,
) error {
	if asset.PurchasePrice <= 0 {
		return fmt.Errorf("asset purchase price must be greater than 0")
	}
	if err := CheckClosedPeriod(s.db, asset.PurchaseDate); err != nil {
		return err
	}

	methodUpper := strings.ToUpper(strings.TrimSpace(paymentMethod))
	isOpeningEntry := methodUpper == "SALDO_AWAL" || methodUpper == "OPENING"
	openingAccumulated := asset.OpeningAccumulatedDepreciation
	if openingAccumulated < 0 {
		return fmt.Errorf("opening accumulated depreciation cannot be negative")
	}
	maxOpeningAccumulated := asset.PurchasePrice - asset.SalvageValue
	if maxOpeningAccumulated < 0 {
		maxOpeningAccumulated = 0
	}
	if openingAccumulated > maxOpeningAccumulated {
		return fmt.Errorf("opening accumulated depreciation cannot exceed depreciable amount")
	}
	categoryLower := strings.ToLower(strings.TrimSpace(asset.Category))
	if (categoryLower == "tanah" || categoryLower == "land") && openingAccumulated > 0 {
		return fmt.Errorf("land assets cannot have accumulated depreciation")
	}
	if !isOpeningEntry && openingAccumulated > 0 {
		return fmt.Errorf("opening accumulated depreciation can only be used for opening balance assets")
	}
	if asset.AccumulatedDepreciation < openingAccumulated {
		asset.AccumulatedDepreciation = openingAccumulated
	}

	// Generate asset code OUTSIDE transaction to avoid transaction abort issues
	if asset.Code == "" {
		code, err := assetService.GenerateAssetCode(asset.Category)
		if err != nil {
			return fmt.Errorf("failed to generate asset code: %v", err)
		}
		asset.Code = code
	}

	intendedIsActive := asset.IsActive

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Step 1: Create asset record
		if err := tx.Select("*").Create(asset).Error; err != nil {
			return fmt.Errorf("failed to create asset: %v", err)
		}
		if err := tx.Exec("UPDATE assets SET is_active = ? WHERE id = ?", intendedIsActive, asset.ID).Error; err != nil {
			return fmt.Errorf("failed to persist asset active flag: %v", err)
		}
		asset.IsActive = intendedIsActive

		// Step 2: Resolve accounts for journal
		fixedAssetAccount, err := s.resolveFixedAssetAccount(tx, asset)
		if err != nil {
			return fmt.Errorf("failed to resolve fixed asset account: %v", err)
		}

		creditAccount, err := s.resolveCreditAccountForPayment(tx, paymentMethod, paymentAccountID, creditAccountID)
		if err != nil {
			return fmt.Errorf("failed to resolve credit account: %v", err)
		}

		amount := decimal.NewFromFloat(asset.PurchasePrice)
		now := time.Now()
		sourceID := uint64(asset.ID)
		postedBy := uint64(userID)

		// Step 3: Determine source type and description based on payment method
		sourceType := models.SSOTSourceTypeAsset
		description := fmt.Sprintf("Pembelian Aset Tetap - %s (%s)", asset.Name, asset.Code)
		entryPrefix := "ASSET-ACQ"

		if isOpeningEntry {
			sourceType = models.SSOTSourceTypeOpening
			description = fmt.Sprintf("Saldo Awal Aset Tetap - %s (%s)", asset.Name, asset.Code)
			entryPrefix = "ASSET-OB"
		}

		// Create journal entry
		entry := &models.SSOTJournalEntry{
			EntryNumber:     fmt.Sprintf("%s-%d-%d", entryPrefix, asset.ID, now.Unix()),
			SourceType:      sourceType,
			SourceID:        &sourceID,
			SourceCode:      asset.Code,
			EntryDate:       asset.PurchaseDate,
			Description:     description,
			Reference:       asset.Code,
			TotalDebit:      amount,
			TotalCredit:     amount,
			Status:          "DRAFT",
			IsBalanced:      true,
			IsAutoGenerated: true,
			CreatedBy:       uint64(userID),
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		if err := tx.Create(entry).Error; err != nil {
			return fmt.Errorf("failed to create asset acquisition journal entry: %v", err)
		}

		// Step 4: Create journal lines
		lines := []models.SSOTJournalLine{
			{
				JournalID:    entry.ID,
				AccountID:    uint64(fixedAssetAccount.ID),
				LineNumber:   1,
				Description:  fmt.Sprintf("Aset Tetap - %s", asset.Name),
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		}

		equityOrPaymentCredit := amount
		if isOpeningEntry && openingAccumulated > 0 {
			accumDepreciationAccount, err := s.resolveAccumulatedDepreciationAccount(tx, asset)
			if err != nil {
				return fmt.Errorf("failed to resolve accumulated depreciation account for opening balance: %v", err)
			}

			openingAccumulatedAmount := decimal.NewFromFloat(openingAccumulated)
			equityOrPaymentCredit = equityOrPaymentCredit.Sub(openingAccumulatedAmount)
			lines = append(lines, models.SSOTJournalLine{
				JournalID:    entry.ID,
				AccountID:    uint64(accumDepreciationAccount.ID),
				LineNumber:   len(lines) + 1,
				Description:  fmt.Sprintf("Akumulasi Penyusutan Awal - %s", asset.Name),
				DebitAmount:  decimal.Zero,
				CreditAmount: openingAccumulatedAmount,
				CreatedAt:    now,
				UpdatedAt:    now,
			})
		}

		if !equityOrPaymentCredit.IsZero() || !isOpeningEntry {
			lines = append(lines, models.SSOTJournalLine{
				JournalID:  entry.ID,
				AccountID:  uint64(creditAccount.ID),
				LineNumber: len(lines) + 1,
				Description: func() string {
					if isOpeningEntry {
						return fmt.Sprintf("Saldo Awal Ekuitas - %s", asset.Name)
					}
					return fmt.Sprintf("Pembayaran Aset - %s (%s)", asset.Name, paymentMethod)
				}(),
				DebitAmount:  decimal.Zero,
				CreditAmount: equityOrPaymentCredit,
				CreatedAt:    now,
				UpdatedAt:    now,
			})
		}

		for _, line := range lines {
			if err := tx.Create(&line).Error; err != nil {
				return fmt.Errorf("failed to create journal line: %v", err)
			}
			if err := s.updateAccountBalance(tx, line.AccountID, line.DebitAmount, line.CreditAmount); err != nil {
				log.Printf("WARNING [ASSET-SSOT] Failed to update balance for account %d: %v", line.AccountID, err)
			}
		}

		// Step 5: Post the journal entry
		if err := tx.Model(entry).Updates(map[string]interface{}{
			"status":    "POSTED",
			"posted_at": &now,
			"posted_by": &postedBy,
		}).Error; err != nil {
			return fmt.Errorf("failed to post asset acquisition journal: %v", err)
		}

		log.Printf("✅ [ASSET-SSOT] Atomically created Asset %s and acquisition journal #%d: Dr %s (%.2f) / Cr %s (%.2f)",
			asset.Code, entry.ID, fixedAssetAccount.Code, asset.PurchasePrice, creditAccount.Code, asset.PurchasePrice)

		return nil
	})

	if err != nil {
		return err
	}

	if err := s.db.Exec("UPDATE assets SET is_active = ? WHERE id = ?", intendedIsActive, asset.ID).Error; err != nil {
		return fmt.Errorf("failed to finalize asset active flag: %v", err)
	}
	asset.IsActive = intendedIsActive
	return nil
}

// CreateAssetAcquisitionJournal creates a SSOT journal entry for a standalone asset purchase.
// Journal: Dr Fixed Asset (1201-xxx) / Cr Kas-Bank-Hutang
func (s *AssetJournalServiceSSOT) CreateAssetAcquisitionJournal(asset *models.Asset, userID uint, paymentMethod string, paymentAccountID *uint, creditAccountID *uint) error {
	if asset.PurchasePrice <= 0 {
		return fmt.Errorf("asset purchase price must be greater than 0")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		// Check for existing journal
		var existingCount int64
		tx.Model(&models.SSOTJournalEntry{}).
			Where("source_type = ? AND source_id = ? AND deleted_at IS NULL", models.SSOTSourceTypeAsset, asset.ID).
			Count(&existingCount)
		if existingCount > 0 {
			log.Printf("⚠️ [ASSET-SSOT] Acquisition journal already exists for Asset #%d, skipping", asset.ID)
			return nil
		}

		// Resolve accounts
		fixedAssetAccount, err := s.resolveFixedAssetAccount(tx, asset)
		if err != nil {
			return fmt.Errorf("failed to resolve fixed asset account: %v", err)
		}

		creditAccount, err := s.resolveCreditAccountForPayment(tx, paymentMethod, paymentAccountID, creditAccountID)
		if err != nil {
			return fmt.Errorf("failed to resolve credit account: %v", err)
		}

		amount := decimal.NewFromFloat(asset.PurchasePrice)
		now := time.Now()
		sourceID := uint64(asset.ID)
		postedBy := uint64(userID)

		// Create journal entry as DRAFT first
		entry := &models.SSOTJournalEntry{
			EntryNumber:     fmt.Sprintf("ASSET-ACQ-%d-%d", asset.ID, now.Unix()),
			SourceType:      models.SSOTSourceTypeAsset,
			SourceID:        &sourceID,
			SourceCode:      asset.Code,
			EntryDate:       asset.PurchaseDate,
			Description:     fmt.Sprintf("Pembelian Aset Tetap - %s (%s)", asset.Name, asset.Code),
			Reference:       asset.Code,
			TotalDebit:      amount,
			TotalCredit:     amount,
			Status:          "DRAFT",
			IsBalanced:      true,
			IsAutoGenerated: true,
			CreatedBy:       uint64(userID),
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		if err := tx.Create(entry).Error; err != nil {
			return fmt.Errorf("failed to create asset acquisition journal entry: %v", err)
		}

		// Create journal lines
		lines := []models.SSOTJournalLine{
			{
				JournalID:    entry.ID,
				AccountID:    uint64(fixedAssetAccount.ID),
				LineNumber:   1,
				Description:  fmt.Sprintf("Aset Tetap - %s", asset.Name),
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			{
				JournalID:    entry.ID,
				AccountID:    uint64(creditAccount.ID),
				LineNumber:   2,
				Description:  fmt.Sprintf("Pembayaran Aset - %s (%s)", asset.Name, paymentMethod),
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		}

		for _, line := range lines {
			if err := tx.Create(&line).Error; err != nil {
				return fmt.Errorf("failed to create journal line: %v", err)
			}
			// Update account balance
			if err := s.updateAccountBalance(tx, line.AccountID, line.DebitAmount, line.CreditAmount); err != nil {
				log.Printf("⚠️ [ASSET-SSOT] Failed to update balance for account %d: %v", line.AccountID, err)
			}
		}

		// Post the journal entry
		if err := tx.Model(entry).Updates(map[string]interface{}{
			"status":    "POSTED",
			"posted_at": &now,
			"posted_by": &postedBy,
		}).Error; err != nil {
			return fmt.Errorf("failed to post asset acquisition journal: %v", err)
		}

		log.Printf("✅ [ASSET-SSOT] Created acquisition journal #%d for Asset %s: Dr %s (%.2f) / Cr %s (%.2f)",
			entry.ID, asset.Code, fixedAssetAccount.Code, asset.PurchasePrice, creditAccount.Code, asset.PurchasePrice)

		return nil
	})
}

// CreateDepreciationJournal creates a SSOT journal entry for monthly depreciation.
// Journal: Dr Beban Penyusutan (6104/6103) / Cr Akumulasi Penyusutan (1202-xxx)
func (s *AssetJournalServiceSSOT) CreateDepreciationJournal(asset *models.Asset, depreciationAmount float64, userID uint, entryDate time.Time) error {
	if depreciationAmount <= 0 {
		return fmt.Errorf("depreciation amount must be greater than 0")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		// Generate unique reference for this depreciation period
		periodRef := fmt.Sprintf("%s-DEP-%s", asset.Code, entryDate.Format("2006-01"))

		// Check for existing depreciation journal for this period
		var existingCount int64
		tx.Model(&models.SSOTJournalEntry{}).
			Where("source_type = ? AND reference = ? AND deleted_at IS NULL", models.SSOTSourceTypeDepreciation, periodRef).
			Count(&existingCount)
		if existingCount > 0 {
			log.Printf("⚠️ [ASSET-SSOT] Depreciation journal already exists for %s period %s, skipping",
				asset.Code, entryDate.Format("2006-01"))
			return nil
		}

		// Resolve accounts
		depreciationExpenseAccount, err := s.resolveDepreciationExpenseAccount(tx, asset)
		if err != nil {
			return fmt.Errorf("failed to resolve depreciation expense account: %v", err)
		}

		accumDepreciationAccount, err := s.resolveAccumulatedDepreciationAccount(tx, asset)
		if err != nil {
			return fmt.Errorf("failed to resolve accumulated depreciation account: %v", err)
		}

		amount := decimal.NewFromFloat(depreciationAmount)
		now := time.Now()
		sourceID := uint64(asset.ID)
		postedBy := uint64(userID)

		entry := &models.SSOTJournalEntry{
			EntryNumber:     fmt.Sprintf("DEP-%d-%s-%d", asset.ID, entryDate.Format("200601"), now.Unix()),
			SourceType:      models.SSOTSourceTypeDepreciation,
			SourceID:        &sourceID,
			SourceCode:      asset.Code,
			EntryDate:       entryDate,
			Description:     fmt.Sprintf("Penyusutan Bulanan - %s (%s) Periode %s", asset.Name, asset.Code, entryDate.Format("Jan 2006")),
			Reference:       periodRef,
			TotalDebit:      amount,
			TotalCredit:     amount,
			Status:          "DRAFT",
			IsBalanced:      true,
			IsAutoGenerated: true,
			CreatedBy:       uint64(userID),
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		if err := tx.Create(entry).Error; err != nil {
			return fmt.Errorf("failed to create depreciation journal entry: %v", err)
		}

		lines := []models.SSOTJournalLine{
			{
				JournalID:    entry.ID,
				AccountID:    uint64(depreciationExpenseAccount.ID),
				LineNumber:   1,
				Description:  fmt.Sprintf("Beban Penyusutan - %s", asset.Name),
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			{
				JournalID:    entry.ID,
				AccountID:    uint64(accumDepreciationAccount.ID),
				LineNumber:   2,
				Description:  fmt.Sprintf("Akumulasi Penyusutan - %s", asset.Name),
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		}

		for _, line := range lines {
			if err := tx.Create(&line).Error; err != nil {
				return fmt.Errorf("failed to create depreciation journal line: %v", err)
			}
			if err := s.updateAccountBalance(tx, line.AccountID, line.DebitAmount, line.CreditAmount); err != nil {
				log.Printf("⚠️ [ASSET-SSOT] Failed to update balance for account %d: %v", line.AccountID, err)
			}
		}

		// Post
		if err := tx.Model(entry).Updates(map[string]interface{}{
			"status":    "POSTED",
			"posted_at": &now,
			"posted_by": &postedBy,
		}).Error; err != nil {
			return fmt.Errorf("failed to post depreciation journal: %v", err)
		}

		// Update accumulated depreciation on the asset record
		if err := tx.Model(asset).UpdateColumn("accumulated_depreciation",
			gorm.Expr("accumulated_depreciation + ?", depreciationAmount)).Error; err != nil {
			log.Printf("⚠️ [ASSET-SSOT] Failed to update asset accumulated_depreciation: %v", err)
		}

		log.Printf("✅ [ASSET-SSOT] Created depreciation journal #%d for Asset %s: Dr %s / Cr %s (%.2f) Period %s",
			entry.ID, asset.Code, depreciationExpenseAccount.Code, accumDepreciationAccount.Code,
			depreciationAmount, entryDate.Format("2006-01"))

		return nil
	})
}

// CreateCapitalizationJournal creates a SSOT journal entry for capitalizing a purchase item into fixed asset.
// Journal: Dr Fixed Asset (1201-xxx) / Cr Source (Persediaan/Beban)
func (s *AssetJournalServiceSSOT) CreateCapitalizationJournal(input CapitalizationInput) error {
	if input.Amount <= 0 {
		return fmt.Errorf("capitalization amount must be greater than 0")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		amount := decimal.NewFromFloat(input.Amount)
		now := time.Now()
		sourceID := uint64(input.ReferenceID)
		postedBy := uint64(input.UserID)

		// Resolve fixed asset account
		fixedAssetAccountID := input.FixedAssetAccountID
		if fixedAssetAccountID == 0 {
			acc, err := s.resolveAccountByCode(tx, "1201-004")
			if err != nil {
				return fmt.Errorf("failed to resolve default fixed asset account: %v", err)
			}
			fixedAssetAccountID = acc.ID
		}

		entry := &models.SSOTJournalEntry{
			EntryNumber:     fmt.Sprintf("CAP-%d-%d", input.ReferenceID, now.Unix()),
			SourceType:      models.SSOTSourceTypeAsset,
			SourceID:        &sourceID,
			EntryDate:       input.Date,
			Description:     input.Description,
			Reference:       input.Reference,
			TotalDebit:      amount,
			TotalCredit:     amount,
			Status:          "DRAFT",
			IsBalanced:      true,
			IsAutoGenerated: true,
			CreatedBy:       uint64(input.UserID),
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		if err := tx.Create(entry).Error; err != nil {
			return fmt.Errorf("failed to create capitalization journal entry: %v", err)
		}

		lines := []models.SSOTJournalLine{
			{
				JournalID:    entry.ID,
				AccountID:    uint64(fixedAssetAccountID),
				LineNumber:   1,
				Description:  "Kapitalisasi ke Aset Tetap",
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			{
				JournalID:    entry.ID,
				AccountID:    uint64(input.SourceAccountID),
				LineNumber:   2,
				Description:  "Kapitalisasi dari Sumber",
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		}

		for _, line := range lines {
			if err := tx.Create(&line).Error; err != nil {
				return fmt.Errorf("failed to create capitalization journal line: %v", err)
			}
			if err := s.updateAccountBalance(tx, line.AccountID, line.DebitAmount, line.CreditAmount); err != nil {
				log.Printf("⚠️ [ASSET-SSOT] Failed to update balance for account %d: %v", line.AccountID, err)
			}
		}

		// Post
		if err := tx.Model(entry).Updates(map[string]interface{}{
			"status":    "POSTED",
			"posted_at": &now,
			"posted_by": &postedBy,
		}).Error; err != nil {
			return fmt.Errorf("failed to post capitalization journal: %v", err)
		}

		log.Printf("✅ [ASSET-SSOT] Created capitalization journal #%d: Dr %d / Cr %d (%.2f)",
			entry.ID, fixedAssetAccountID, input.SourceAccountID, input.Amount)

		return nil
	})
}

// ReverseAssetJournals reverses all SSOT journal entries for a given asset.
// Used when an asset is deleted.
func (s *AssetJournalServiceSSOT) ReverseAssetJournals(assetID uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Find all journal entries for this asset
		var entries []models.SSOTJournalEntry
		if err := tx.Where("source_type IN ? AND source_id = ? AND deleted_at IS NULL",
			[]string{models.SSOTSourceTypeAsset, models.SSOTSourceTypeOpening, models.SSOTSourceTypeDepreciation}, assetID).
			Find(&entries).Error; err != nil {
			return fmt.Errorf("failed to find asset journals: %v", err)
		}

		if len(entries) == 0 {
			log.Printf("ℹ️ [ASSET-SSOT] No journals found for Asset #%d, nothing to reverse", assetID)
			return nil
		}

		for _, entry := range entries {
			// Load lines
			var lines []models.SSOTJournalLine
			if err := tx.Where("journal_id = ?", entry.ID).Find(&lines).Error; err != nil {
				log.Printf("⚠️ [ASSET-SSOT] Failed to load lines for journal #%d: %v", entry.ID, err)
				continue
			}

			// Reverse balances
			for _, line := range lines {
				negDebit := line.DebitAmount.Neg()
				negCredit := line.CreditAmount.Neg()
				if err := s.updateAccountBalance(tx, line.AccountID, negDebit, negCredit); err != nil {
					log.Printf("⚠️ [ASSET-SSOT] Failed to reverse balance for account %d: %v", line.AccountID, err)
				}
			}

			// Delete lines
			if err := tx.Where("journal_id = ?", entry.ID).Delete(&models.SSOTJournalLine{}).Error; err != nil {
				log.Printf("⚠️ [ASSET-SSOT] Failed to delete lines for journal #%d: %v", entry.ID, err)
			}

			// Delete entry
			if err := tx.Delete(&entry).Error; err != nil {
				log.Printf("⚠️ [ASSET-SSOT] Failed to delete journal #%d: %v", entry.ID, err)
			}
		}

		log.Printf("✅ [ASSET-SSOT] Reversed %d journal entries for Asset #%d", len(entries), assetID)
		return nil
	})
}

// ── Batch Depreciation ─────────────────────────────────────────────────────

// DepreciationRunResult holds the result of a monthly depreciation run
type DepreciationRunResult struct {
	Period            string   `json:"period"`
	TotalAssets       int      `json:"total_assets"`
	ProcessedAssets   int      `json:"processed_assets"`
	SkippedAssets     int      `json:"skipped_assets"`
	TotalDepreciation float64  `json:"total_depreciation"`
	Errors            []string `json:"errors,omitempty"`
}

// RunMonthlyDepreciation runs depreciation for all active assets for a given month.
// Each asset creates its own individual SSOT journal entry.
func (s *AssetJournalServiceSSOT) RunMonthlyDepreciation(year int, month int, userID uint) (*DepreciationRunResult, error) {
	result := &DepreciationRunResult{
		Period: fmt.Sprintf("%04d-%02d", year, month),
	}

	// Entry date = last day of the month
	entryDate := time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC)

	// Guard: check if this period is closed
	if err := CheckClosedPeriod(s.db, entryDate); err != nil {
		return nil, err
	}

	// Get all active, depreciable assets
	var assets []models.Asset
	if err := s.db.Where("is_active = true AND status = ? AND purchase_date <= ? AND depreciation_method != '' AND depreciation_method IS NOT NULL",
		models.AssetStatusActive, entryDate).
		Find(&assets).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch active assets: %v", err)
	}

	result.TotalAssets = len(assets)

	for _, asset := range assets {
		// Skip land (not depreciable)
		categoryLower := strings.ToLower(strings.TrimSpace(asset.Category))
		if categoryLower == "tanah" || categoryLower == "land" {
			result.SkippedAssets++
			continue
		}

		// Calculate monthly depreciation
		monthlyDeprec := s.calculateMonthlyDepreciation(&asset, entryDate)
		if monthlyDeprec <= 0 {
			result.SkippedAssets++
			continue
		}

		// Check if already fully depreciated
		currentBookValue := asset.PurchasePrice - asset.AccumulatedDepreciation
		if currentBookValue <= asset.SalvageValue {
			result.SkippedAssets++
			log.Printf("ℹ️ [ASSET-SSOT] Asset %s already fully depreciated (BV=%.2f, SV=%.2f)", asset.Code, currentBookValue, asset.SalvageValue)
			continue
		}

		// Cap depreciation so book value doesn't go below salvage value
		maxDeprec := currentBookValue - asset.SalvageValue
		if monthlyDeprec > maxDeprec {
			monthlyDeprec = maxDeprec
		}

		// Create the journal
		if err := s.CreateDepreciationJournal(&asset, monthlyDeprec, userID, entryDate); err != nil {
			errMsg := fmt.Sprintf("Asset %s: %v", asset.Code, err)
			result.Errors = append(result.Errors, errMsg)
			log.Printf("⚠️ [ASSET-SSOT] %s", errMsg)
			continue
		}

		result.ProcessedAssets++
		result.TotalDepreciation += monthlyDeprec
	}

	log.Printf("✅ [ASSET-SSOT] Monthly depreciation run completed for %s: %d/%d assets processed, total=%.2f",
		result.Period, result.ProcessedAssets, result.TotalAssets, result.TotalDepreciation)

	return result, nil
}

// calculateMonthlyDepreciation calculates the monthly depreciation for an asset.
func (s *AssetJournalServiceSSOT) calculateMonthlyDepreciation(asset *models.Asset, asOfDate time.Time) float64 {
	if asset.UsefulLife <= 0 || asset.PurchasePrice <= 0 {
		return 0
	}

	depreciableAmount := asset.PurchasePrice - asset.SalvageValue
	if depreciableAmount <= 0 {
		return 0
	}

	switch strings.ToUpper(asset.DepreciationMethod) {
	case "STRAIGHT_LINE":
		// Monthly = (Cost - Salvage) / (Life × 12)
		return depreciableAmount / (float64(asset.UsefulLife) * 12)

	case "DECLINING_BALANCE":
		// Double declining balance
		rate := 2.0 / float64(asset.UsefulLife)
		currentBookValue := asset.PurchasePrice - asset.AccumulatedDepreciation
		annualDeprec := currentBookValue * rate
		monthlyDeprec := annualDeprec / 12

		// Don't depreciate below salvage value
		if currentBookValue-monthlyDeprec < asset.SalvageValue {
			monthlyDeprec = currentBookValue - asset.SalvageValue
		}
		if monthlyDeprec < 0 {
			return 0
		}
		return monthlyDeprec

	default:
		return 0
	}
}

// ── COA Balance Helper ─────────────────────────────────────────────────────

// updateAccountBalance updates account.balance field for COA tree view display.
// Same pattern as PurchaseJournalServiceSSOT.updateAccountBalance.
func (s *AssetJournalServiceSSOT) updateAccountBalance(db *gorm.DB, accountID uint64, debitAmount, creditAmount decimal.Decimal) error {
	var account models.Account
	if err := db.First(&account, accountID).Error; err != nil {
		return fmt.Errorf("account %d not found: %v", accountID, err)
	}

	debit := debitAmount.InexactFloat64()
	credit := creditAmount.InexactFloat64()
	netChange := debit - credit

	switch strings.ToUpper(account.Type) {
	case "ASSET", "EXPENSE":
		account.Balance += netChange
	case "LIABILITY", "EQUITY", "REVENUE":
		account.Balance -= netChange
	}

	if err := db.Model(&account).Select("Balance").Updates(models.Account{Balance: account.Balance}).Error; err != nil {
		return fmt.Errorf("failed to save account balance: %v", err)
	}

	log.Printf("💰 [ASSET-SSOT] Updated account %s (%s) balance: Dr=%.2f, Cr=%.2f, Change=%.2f, New=%.2f",
		account.Code, account.Name, debit, credit, netChange, account.Balance)

	return nil
}
