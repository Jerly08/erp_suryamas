package database

import (
	"strings"
	"testing"

	"app-sistem-akuntansi/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRequireExplicitTemplateSyncApprovalRequiresEnv(t *testing.T) {
	t.Setenv(destructiveCOATemplateSyncEnv, "")

	err := requireExplicitTemplateSyncApproval()
	if err == nil {
		t.Fatal("expected destructive template sync approval to be required")
	}
}

func TestRequireExplicitTemplateSyncApprovalAllowsApprovedEnv(t *testing.T) {
	t.Setenv(destructiveCOATemplateSyncEnv, destructiveCOATemplateSyncApproval)

	if err := requireExplicitTemplateSyncApproval(); err != nil {
		t.Fatalf("expected approval guard to pass, got %v", err)
	}
}

func TestBuildExistingAccountUpdatesSkipsSafeMode(t *testing.T) {
	existing := models.Account{
		Code:        "1101",
		Name:        "Kas Existing",
		IsActive:    false,
		Balance:     12345.67,
		Description: "existing account",
	}
	seed := models.Account{
		Code:        "1101",
		Name:        "Kas Template Baru",
		IsActive:    true,
		Balance:     0,
		Description: "template account",
	}

	updates, shouldUpdate := buildExistingAccountUpdates(existing, seed, "KAS TEMPLATE BARU", AccountSeedOptions{})
	if shouldUpdate {
		t.Fatal("expected safe seed mode to skip updates for existing accounts")
	}
	if updates != nil {
		t.Fatalf("expected no updates in safe seed mode, got %#v", updates)
	}
}

func TestBuildExistingAccountUpdatesRespectsSystemCriticalProtection(t *testing.T) {
	existing := models.Account{
		Code:             "1101",
		Name:             "Kas Existing",
		IsActive:         false,
		IsSystemCritical: true,
	}
	seed := models.Account{
		Code:        "1101",
		Name:        "Kas Template Baru",
		Type:        models.AccountTypeAsset,
		Category:    models.CategoryCurrentAsset,
		IsActive:    true,
		IsHeader:    true,
		Level:       1,
		Description: "template account",
	}

	updates, shouldUpdate := buildExistingAccountUpdates(existing, seed, "KAS TEMPLATE BARU", AccountSeedOptions{
		UpdateExistingTemplateAccounts: true,
	})
	if !shouldUpdate {
		t.Fatal("expected template sync mode to build update payload")
	}
	if _, exists := updates["is_active"]; exists {
		t.Fatal("expected system critical account to keep existing is_active")
	}
	if _, exists := updates["type"]; exists {
		t.Fatal("expected system critical account to keep existing type")
	}
	if _, exists := updates["category"]; exists {
		t.Fatal("expected system critical account to keep existing category")
	}
	if updates["name"] != "KAS TEMPLATE BARU" {
		t.Fatalf("expected normalized name update, got %#v", updates["name"])
	}
}

func TestGetStandardCOAIncludesDedicatedNonOperatingAccounts(t *testing.T) {
	companyTypes := []string{
		models.CompanyTypeService,
		models.CompanyTypeTrading,
		models.CompanyTypeManufacturing,
	}

	expectedCategories := map[string]string{
		"7104": models.CategoryDividendIncome,
		"7105": models.CategoryGainOnSale,
		"8104": models.CategoryLossOnSale,
		"8105": models.CategoryFinancialExpense,
	}

	for _, companyType := range companyTypes {
		coa := GetStandardCOA(companyType)
		indexed := make(map[string]models.Account, len(coa))
		for _, account := range coa {
			indexed[account.Code] = account
		}

		for code, category := range expectedCategories {
			account, exists := indexed[code]
			if !exists {
				t.Fatalf("expected company type %s to include account %s", companyType, code)
			}
			if account.Category != category {
				t.Fatalf("expected company type %s account %s to use category %s, got %s", companyType, code, category, account.Category)
			}
		}

		if account, exists := indexed["4101"]; !exists || account.Category != models.CategoryOperatingRevenue {
			t.Fatalf("expected company type %s account 4101 to map to %s", companyType, models.CategoryOperatingRevenue)
		}
	}
}

func TestProfitLossCOAAlignmentMigrationBackfills4101AndAddsDedicatedAccounts(t *testing.T) {
	db := setupDatabaseTestDB(t)

	require.NoError(t, db.Create(&models.Settings{
		CompanyName: "PT Test ERP",
		CompanyType: models.CompanyTypeTrading,
	}).Error)

	parents := []models.Account{
		{Code: "4000", Name: "PENDAPATAN", Type: models.AccountTypeRevenue, Category: models.CategoryOperatingRevenue, Level: 1, IsHeader: true, IsActive: true, IsSystemCritical: true},
		{Code: "7000", Name: "PENDAPATAN LAIN-LAIN", Type: models.AccountTypeRevenue, Category: models.CategoryOtherIncome, Level: 1, IsHeader: true, IsActive: true},
		{Code: "8000", Name: "BEBAN LAIN-LAIN", Type: models.AccountTypeExpense, Category: models.CategoryNonOperatingExpense, Level: 1, IsHeader: true, IsActive: true},
	}
	for _, parent := range parents {
		require.NoError(t, db.Create(&parent).Error)
	}

	require.NoError(t, db.Create(&models.Account{
		Code:             "4101",
		Name:             "PENJUALAN BARANG",
		Type:             models.AccountTypeRevenue,
		Category:         "",
		Level:            2,
		IsHeader:         true,
		IsActive:         true,
		IsSystemCritical: true,
	}).Error)

	ProfitLossCOAAlignmentMigration(db)

	var account4101 models.Account
	require.NoError(t, db.Where("code = ? AND deleted_at IS NULL", "4101").First(&account4101).Error)
	require.Equal(t, models.CategoryOperatingRevenue, account4101.Category)

	expected := map[string]struct {
		category   string
		parentCode string
	}{
		"7104": {category: models.CategoryDividendIncome, parentCode: "7000"},
		"7105": {category: models.CategoryGainOnSale, parentCode: "7000"},
		"8104": {category: models.CategoryLossOnSale, parentCode: "8000"},
		"8105": {category: models.CategoryFinancialExpense, parentCode: "8000"},
	}

	for code, expectation := range expected {
		var account models.Account
		require.NoError(t, db.Where("code = ? AND deleted_at IS NULL", code).First(&account).Error)
		require.Equal(t, expectation.category, account.Category)
		require.NotNil(t, account.ParentID)

		var parent models.Account
		require.NoError(t, db.First(&parent, *account.ParentID).Error)
		require.Equal(t, expectation.parentCode, parent.Code)
	}

	var migrationRecord models.MigrationRecord
	require.NoError(t, db.Where("migration_id = ?", profitLossCOAAlignmentMigrationID).First(&migrationRecord).Error)
}

func setupDatabaseTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		if strings.Contains(err.Error(), "requires cgo") {
			t.Skip("sqlite test skipped because this environment runs with CGO disabled")
		}
		require.NoError(t, err)
	}

	require.NoError(t, db.AutoMigrate(&models.Settings{}, &models.Account{}, &models.MigrationRecord{}))
	return db
}
