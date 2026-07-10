package services

import (
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSalesShippingTaxFlowsToPostedJournal(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{}, &models.Contact{}, &models.Account{}, &models.Settings{},
		&models.TaxAccountSettings{}, &models.Product{}, &models.Sale{}, &models.SaleItem{},
		&models.CashBank{}, &models.SSOTJournalEntry{}, &models.SSOTJournalLine{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	user := models.User{ID: 1, Username: "tester", Email: "tester@example.com", Password: "secret", Role: "admin", FirstName: "Test", IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&models.Settings{CompanyName: "Test Company", CompanyType: models.CompanyTypeService}).Error; err != nil {
		t.Fatalf("create settings: %v", err)
	}

	createAccount := func(code, name, accountType, category string) models.Account {
		account := models.Account{Code: code, Name: name, Type: accountType, Category: category, IsActive: true}
		if err := db.Create(&account).Error; err != nil {
			t.Fatalf("create account %s: %v", code, err)
		}
		return account
	}
	receivable := createAccount("1102-001", "PIUTANG USAHA", models.AccountTypeAsset, models.CategoryCurrentAsset)
	cash := createAccount("1101-002", "KAS", models.AccountTypeAsset, models.CategoryCurrentAsset)
	bank := createAccount("1101-004", "BANK", models.AccountTypeAsset, models.CategoryCurrentAsset)
	revenue := createAccount("4101-001", "PENDAPATAN PENJUALAN", models.AccountTypeRevenue, models.CategoryOperatingRevenue)
	shippingRevenue := createAccount("4101-010", "PENDAPATAN ONGKIR", models.AccountTypeRevenue, models.CategoryOperatingRevenue)
	outputVAT := createAccount("2102-001", "PPN KELUARAN", models.AccountTypeLiability, models.CategoryCurrentLiability)
	payable := createAccount("2101-001", "UTANG USAHA", models.AccountTypeLiability, models.CategoryCurrentLiability)
	expense := createAccount("6102", "BEBAN OPERASIONAL", models.AccountTypeExpense, models.CategoryOperatingExpense)

	shippingRevenueID := shippingRevenue.ID
	taxSettings := models.TaxAccountSettings{
		SalesReceivableAccountID: receivable.ID, SalesCashAccountID: cash.ID, SalesBankAccountID: bank.ID,
		SalesRevenueAccountID: revenue.ID, SalesShippingRevenueAccountID: &shippingRevenueID, SalesOutputVATAccountID: outputVAT.ID,
		PurchasePayableAccountID: payable.ID, PurchaseCashAccountID: cash.ID, PurchaseBankAccountID: bank.ID,
		PurchaseInputVATAccountID: outputVAT.ID, PurchaseExpenseAccountID: expense.ID,
		IsActive: true, ApplyToAllCompanies: true, UpdatedBy: user.ID,
	}
	if err := db.Create(&taxSettings).Error; err != nil {
		t.Fatalf("create tax settings: %v", err)
	}

	customer := models.Contact{Code: "CUST-001", Name: "PT Pelanggan", Type: "CUSTOMER", IsActive: true}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	product := models.Product{Code: "SVC-001", Name: "Jasa Test", IsService: true, IsActive: true}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}

	receivableID := receivable.ID
	sale := models.Sale{
		Code: "SOA-TEST", InvoiceNumber: "INV-TEST", CustomerID: customer.ID, Customer: customer,
		UserID: user.ID, Type: models.SaleTypeInvoice, Status: models.SaleStatusInvoiced,
		Date: time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC), PaymentMethodType: "CREDIT",
		ReceivableAccountID: &receivableID, PPNRate: 11, PPNPercent: 11,
		ShippingCost: 32500, ShippingTaxable: true,
	}
	if err := db.Create(&sale).Error; err != nil {
		t.Fatalf("create sale: %v", err)
	}
	item := models.SaleItem{SaleID: sale.ID, ProductID: product.ID, Quantity: 1, UnitPrice: 120000, LineTotal: 120000, FinalAmount: 120000, Taxable: true, RevenueAccountID: revenue.ID}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create sale item: %v", err)
	}
	sale.SaleItems = []models.SaleItem{item}
	applyCalculatedSalesAmounts(&sale, sale.SaleItems)
	if sale.TaxableAmount != 152500 || sale.PPN != 16775 || sale.TotalAmount != 169275 {
		t.Fatalf("unexpected persisted totals: DPP=%.2f PPN=%.2f total=%.2f", sale.TaxableAmount, sale.PPN, sale.TotalAmount)
	}
	if err := db.Save(&sale).Error; err != nil {
		t.Fatalf("save sale totals: %v", err)
	}

	service := NewSalesJournalServiceSSOT(db, NewCOAService(db), NewSettingsService(db))
	if err := service.CreateSalesJournal(&sale, nil); err != nil {
		t.Fatalf("create sales journal: %v", err)
	}

	var journal models.SSOTJournalEntry
	if err := db.Preload("Lines").Where("source_type = ? AND source_id = ?", "SALE", sale.ID).First(&journal).Error; err != nil {
		t.Fatalf("load journal: %v", err)
	}
	if journal.Status != models.SSOTStatusPosted {
		t.Fatalf("expected POSTED journal, got %s", journal.Status)
	}
	if !journal.TotalDebit.Equal(journal.TotalCredit) || journal.TotalDebit.InexactFloat64() != 169275 {
		t.Fatalf("expected balanced total 169275, debit=%s credit=%s", journal.TotalDebit, journal.TotalCredit)
	}

	expected := map[uint]struct{ debit, credit float64 }{
		receivable.ID:      {debit: 169275},
		revenue.ID:         {credit: 120000},
		shippingRevenue.ID: {credit: 32500},
		outputVAT.ID:       {credit: 16775},
	}
	for _, line := range journal.Lines {
		want, ok := expected[uint(line.AccountID)]
		if !ok {
			continue
		}
		if line.DebitAmount.InexactFloat64() != want.debit || line.CreditAmount.InexactFloat64() != want.credit {
			t.Fatalf("unexpected line for account %d: debit=%s credit=%s", line.AccountID, line.DebitAmount, line.CreditAmount)
		}
		delete(expected, uint(line.AccountID))
	}
	if len(expected) != 0 {
		t.Fatalf("missing expected journal accounts: %v", expected)
	}
}
