package services

import (
	"testing"

	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

type requiredTaxAccountFixture struct {
	receivable models.Account
	cash       models.Account
	bank       models.Account
	revenue    models.Account
	outputVAT  models.Account
	payable    models.Account
	inputVAT   models.Account
	expense    models.Account
	badRevenue models.Account
}

func seedRequiredTaxAccountFixture(t *testing.T) (*gorm.DB, requiredTaxAccountFixture) {
	t.Helper()

	db := newTaxAccountHelperTestDB(t)
	fixture := requiredTaxAccountFixture{
		receivable: createTaxTestAccount(t, db, "1102-001", "PIUTANG USAHA", models.AccountTypeAsset, models.CategoryCurrentAsset),
		cash:       createTaxTestAccount(t, db, "1101-002", "KAS BESAR", models.AccountTypeAsset, models.CategoryCurrentAsset),
		bank:       createTaxTestAccount(t, db, "1101-004", "BANK OPERASIONAL", models.AccountTypeAsset, models.CategoryCurrentAsset),
		revenue:    createTaxTestAccount(t, db, "4101-001", "PENJUALAN", models.AccountTypeRevenue, models.CategoryOperatingRevenue),
		outputVAT:  createTaxTestAccount(t, db, "2102-001", "PPN KELUARAN", models.AccountTypeLiability, models.CategoryCurrentLiability),
		payable:    createTaxTestAccount(t, db, "2101-001", "UTANG USAHA", models.AccountTypeLiability, models.CategoryCurrentLiability),
		inputVAT:   createTaxTestAccount(t, db, "2102-002", "PPN MASUKAN", models.AccountTypeLiability, models.CategoryCurrentLiability),
		expense:    createTaxTestAccount(t, db, "6102", "BEBAN ADMINISTRASI & UMUM", models.AccountTypeExpense, models.CategoryAdministrativeExp),
		badRevenue: createTaxTestAccount(t, db, "4101", "PENDAPATAN PENJUALAN HEADER", models.AccountTypeRevenue, models.CategoryOperatingRevenue),
	}

	return db, fixture
}

func createTaxSettingsFromFixture(t *testing.T, db *gorm.DB, fixture requiredTaxAccountFixture, purchaseExpenseAccountID uint) models.TaxAccountSettings {
	t.Helper()

	settings := models.TaxAccountSettings{
		SalesReceivableAccountID:  fixture.receivable.ID,
		SalesCashAccountID:        fixture.cash.ID,
		SalesBankAccountID:        fixture.bank.ID,
		SalesRevenueAccountID:     fixture.revenue.ID,
		SalesOutputVATAccountID:   fixture.outputVAT.ID,
		PurchasePayableAccountID:  fixture.payable.ID,
		PurchaseCashAccountID:     fixture.cash.ID,
		PurchaseBankAccountID:     fixture.bank.ID,
		PurchaseInputVATAccountID: fixture.inputVAT.ID,
		PurchaseExpenseAccountID:  purchaseExpenseAccountID,
		IsActive:                  true,
		ApplyToAllCompanies:       true,
		UpdatedBy:                 1,
	}
	if err := db.Create(&settings).Error; err != nil {
		t.Fatalf("seed tax settings: %v", err)
	}

	return settings
}

func TestTaxAccountServiceRepairsInvalidPurchaseExpenseAccount(t *testing.T) {
	db, fixture := seedRequiredTaxAccountFixture(t)
	createTaxSettingsFromFixture(t, db, fixture, fixture.badRevenue.ID)

	service := NewTaxAccountService(db)
	settings, err := service.GetSettings()
	if err != nil {
		t.Fatalf("get repaired settings: %v", err)
	}

	if settings.PurchaseExpenseAccountID != fixture.expense.ID {
		t.Fatalf("expected purchase expense repaired to %s, got account id %d", fixture.expense.Code, settings.PurchaseExpenseAccountID)
	}
	if settings.PurchaseExpenseAccount.Type != models.AccountTypeExpense {
		t.Fatalf("expected repaired purchase expense type EXPENSE, got %s", settings.PurchaseExpenseAccount.Type)
	}
}

func TestTaxAccountServiceRepairsProductionLikeInvalidRequiredMappings(t *testing.T) {
	db, fixture := seedRequiredTaxAccountFixture(t)
	settings := models.TaxAccountSettings{
		SalesReceivableAccountID:  fixture.badRevenue.ID,
		SalesCashAccountID:        fixture.badRevenue.ID,
		SalesBankAccountID:        fixture.badRevenue.ID,
		SalesRevenueAccountID:     fixture.badRevenue.ID,
		SalesOutputVATAccountID:   fixture.outputVAT.ID,
		PurchasePayableAccountID:  fixture.badRevenue.ID,
		PurchaseCashAccountID:     fixture.badRevenue.ID,
		PurchaseBankAccountID:     fixture.badRevenue.ID,
		PurchaseInputVATAccountID: fixture.inputVAT.ID,
		PurchaseExpenseAccountID:  fixture.badRevenue.ID,
		IsActive:                  true,
		ApplyToAllCompanies:       true,
		UpdatedBy:                 1,
	}
	if err := db.Create(&settings).Error; err != nil {
		t.Fatalf("seed invalid production-like settings: %v", err)
	}

	service := NewTaxAccountService(db)
	repaired, err := service.GetSettings()
	if err != nil {
		t.Fatalf("get repaired production-like settings: %v", err)
	}

	expectedTypes := map[string]struct {
		id          uint
		accountType string
	}{
		"sales_receivable":   {repaired.SalesReceivableAccountID, models.AccountTypeAsset},
		"sales_cash":         {repaired.SalesCashAccountID, models.AccountTypeAsset},
		"sales_bank":         {repaired.SalesBankAccountID, models.AccountTypeAsset},
		"sales_revenue":      {repaired.SalesRevenueAccountID, models.AccountTypeRevenue},
		"sales_output_vat":   {repaired.SalesOutputVATAccountID, models.AccountTypeLiability},
		"purchase_payable":   {repaired.PurchasePayableAccountID, models.AccountTypeLiability},
		"purchase_cash":      {repaired.PurchaseCashAccountID, models.AccountTypeAsset},
		"purchase_bank":      {repaired.PurchaseBankAccountID, models.AccountTypeAsset},
		"purchase_input_vat": {repaired.PurchaseInputVATAccountID, models.AccountTypeLiability},
		"purchase_expense":   {repaired.PurchaseExpenseAccountID, models.AccountTypeExpense},
	}
	for role, expected := range expectedTypes {
		account, err := service.loadAccount(expected.id)
		if err != nil {
			t.Fatalf("load repaired %s account: %v", role, err)
		}
		if account.Type != expected.accountType {
			t.Fatalf("expected %s to be %s, got %s (%s)", role, expected.accountType, account.Type, account.Code)
		}
	}
}

func TestTaxAccountServiceRejectsRevenueAsPurchaseExpenseAccount(t *testing.T) {
	db, fixture := seedRequiredTaxAccountFixture(t)
	settings := createTaxSettingsFromFixture(t, db, fixture, fixture.expense.ID)

	service := NewTaxAccountService(db)
	_, err := service.UpdateSettings(settings.ID, &models.TaxAccountSettingsUpdateRequest{
		PurchaseExpenseAccountID: &fixture.badRevenue.ID,
	}, 1)
	if err == nil {
		t.Fatal("expected update with revenue purchase expense account to fail")
	}
}
