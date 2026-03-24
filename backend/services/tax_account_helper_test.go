package services

import (
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTaxAccountHelperTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	if err := db.AutoMigrate(&models.User{}, &models.Account{}, &models.TaxAccountSettings{}); err != nil {
		t.Fatalf("auto migrate test db: %v", err)
	}

	if err := db.Create(&models.User{
		ID:        1,
		Username:  "tester",
		Email:     "tester@example.com",
		Password:  "secret",
		Role:      "admin",
		FirstName: "Test",
		IsActive:  true,
	}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	return db
}

func createTaxTestAccount(t *testing.T, db *gorm.DB, code, name, accountType, category string) models.Account {
	t.Helper()

	account := models.Account{
		Code:     code,
		Name:     name,
		Type:     accountType,
		Category: category,
		IsActive: true,
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create account %s: %v", code, err)
	}
	return account
}

func seedTaxSettings(t *testing.T, db *gorm.DB, purchaseInputVATAccountID uint) {
	t.Helper()

	receivable := createTaxTestAccount(t, db, "1201-001", "PIUTANG USAHA", models.AccountTypeAsset, models.CategoryCurrentAsset)
	cash := createTaxTestAccount(t, db, "1101-001", "KAS", models.AccountTypeAsset, models.CategoryCurrentAsset)
	bank := createTaxTestAccount(t, db, "1101-003", "BANK - GIRO", models.AccountTypeAsset, models.CategoryCurrentAsset)
	salesRevenue := createTaxTestAccount(t, db, "4101-001", "PENDAPATAN PENJUALAN", models.AccountTypeRevenue, models.CategorySalesRevenue)
	outputVAT := createTaxTestAccount(t, db, "2102-001", "PPN KELUARAN", models.AccountTypeLiability, models.CategoryCurrentLiability)
	payable := createTaxTestAccount(t, db, "2101-001", "UTANG USAHA", models.AccountTypeLiability, models.CategoryCurrentLiability)
	expense := createTaxTestAccount(t, db, "6101-001", "BEBAN OPERASIONAL", models.AccountTypeExpense, models.CategoryOperatingExpense)

	settings := models.TaxAccountSettings{
		SalesReceivableAccountID:  receivable.ID,
		SalesCashAccountID:        cash.ID,
		SalesBankAccountID:        bank.ID,
		SalesRevenueAccountID:     salesRevenue.ID,
		SalesOutputVATAccountID:   outputVAT.ID,
		PurchasePayableAccountID:  payable.ID,
		PurchaseCashAccountID:     cash.ID,
		PurchaseBankAccountID:     bank.ID,
		PurchaseInputVATAccountID: purchaseInputVATAccountID,
		PurchaseExpenseAccountID:  expense.ID,
		IsActive:                  true,
		ApplyToAllCompanies:       true,
		UpdatedBy:                 1,
	}

	if err := db.Create(&settings).Error; err != nil {
		t.Fatalf("seed tax settings: %v", err)
	}
}

func TestGetPurchaseInputVATAccountUsesLatestSettingsWithoutRestart(t *testing.T) {
	db := newTaxAccountHelperTestDB(t)

	oldVAT := createTaxTestAccount(t, db, "1240", "PPN MASUKAN LAMA", models.AccountTypeAsset, models.CategoryCurrentAsset)
	newVAT := createTaxTestAccount(t, db, "2102-002", "PPN MASUKAN BARU", models.AccountTypeLiability, models.CategoryCurrentLiability)
	seedTaxSettings(t, db, oldVAT.ID)

	helper := NewTaxAccountHelper(db)

	firstAccount, err := helper.GetPurchaseInputVATAccount(db)
	if err != nil {
		t.Fatalf("first vat lookup: %v", err)
	}
	if firstAccount.ID != oldVAT.ID {
		t.Fatalf("expected first VAT account %s, got %s", oldVAT.Code, firstAccount.Code)
	}

	if err := db.Model(&models.TaxAccountSettings{}).
		Where("is_active = ?", true).
		Updates(map[string]interface{}{
			"purchase_input_vat_account_id": newVAT.ID,
			"updated_at":                    time.Now().Add(time.Minute),
		}).Error; err != nil {
		t.Fatalf("update tax settings: %v", err)
	}

	secondAccount, err := helper.GetPurchaseInputVATAccount(db)
	if err != nil {
		t.Fatalf("second vat lookup: %v", err)
	}
	if secondAccount.ID != newVAT.ID {
		t.Fatalf("expected updated VAT account %s, got %s", newVAT.Code, secondAccount.Code)
	}
}

func TestGetPurchaseInputVATAccountFallsBackFromInvalidRevenueAccount(t *testing.T) {
	db := newTaxAccountHelperTestDB(t)

	fallbackVAT := createTaxTestAccount(t, db, "2102-002", "PPN MASUKAN", models.AccountTypeLiability, models.CategoryCurrentLiability)
	invalidRevenue := createTaxTestAccount(t, db, "4101", "PENDAPATAN PENJUALAN", models.AccountTypeRevenue, models.CategorySalesRevenue)
	seedTaxSettings(t, db, invalidRevenue.ID)

	helper := NewTaxAccountHelper(db)

	account, err := helper.GetPurchaseInputVATAccount(db)
	if err != nil {
		t.Fatalf("vat lookup with invalid config: %v", err)
	}
	if account.ID != fallbackVAT.ID {
		t.Fatalf("expected fallback VAT account %s, got %s", fallbackVAT.Code, account.Code)
	}
}
