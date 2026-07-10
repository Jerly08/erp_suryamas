package services

import (
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSalesPPh23TestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Account{},
		&models.CashBank{},
		&models.CashBankTransaction{},
		&models.Sale{},
		&models.SaleItem{},
		&models.SalePayment{},
		&models.SalePaymentSchedule{},
		&models.AccountingPeriod{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
	); err != nil {
		t.Fatalf("auto migrate test db: %v", err)
	}

	return db
}

func seedSalesPPh23Fixture(t *testing.T, db *gorm.DB, outstandingAmount float64) models.Sale {
	t.Helper()

	user := models.User{
		ID:        1,
		Username:  "tester",
		Email:     "tester@example.com",
		Password:  "secret",
		Role:      "admin",
		FirstName: "Test",
		IsActive:  true,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	customer := models.Contact{
		ID:       1,
		Code:     "CUST-001",
		Name:     "PT Pelanggan",
		Type:     "CUSTOMER",
		IsActive: true,
	}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("seed contact: %v", err)
	}

	parentAccount := models.Account{
		Code:     "1105",
		Name:     "PAJAK DIBAYAR DIMUKA",
		Type:     models.AccountTypeAsset,
		Category: models.CategoryCurrentAsset,
		Level:    3,
		IsHeader: true,
		IsActive: true,
	}
	if err := db.Create(&parentAccount).Error; err != nil {
		t.Fatalf("seed parent account: %v", err)
	}

	accounts := []models.Account{
		{
			Code:     "1102-001",
			Name:     "PIUTANG USAHA",
			Type:     models.AccountTypeAsset,
			Category: models.CategoryCurrentAsset,
			Level:    4,
			IsActive: true,
		},
		{
			Code:     "1105-002",
			Name:     "PPH PASAL 23 DIBAYAR DIMUKA",
			Type:     models.AccountTypeAsset,
			Category: models.CategoryCurrentAsset,
			Level:    4,
			ParentID: &parentAccount.ID,
			IsActive: true,
		},
		{
			Code:     "1105-004",
			Name:     "KREDIT PAJAK PPH 23",
			Type:     models.AccountTypeAsset,
			Category: models.CategoryCurrentAsset,
			Level:    4,
			ParentID: &parentAccount.ID,
			IsActive: true,
		},
	}
	for _, account := range accounts {
		if err := db.Create(&account).Error; err != nil {
			t.Fatalf("seed account %s: %v", account.Code, err)
		}
	}

	sale := models.Sale{
		Code:              "SAL-001",
		CustomerID:        customer.ID,
		UserID:            user.ID,
		Type:              models.SaleTypeInvoice,
		Status:            models.SaleStatusPaid,
		Date:              time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		DueDate:           time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC),
		InvoiceNumber:     "INV-001",
		Subtotal:          1000000,
		NetBeforeTax:      1000000,
		TaxableAmount:     1000000,
		TotalAmount:       980000,
		PaidAmount:        980000 - outstandingAmount,
		OutstandingAmount: outstandingAmount,
		PPh23Rate:         2,
		PPh23Amount:       20000,
	}
	if err := db.Create(&sale).Error; err != nil {
		t.Fatalf("seed sale: %v", err)
	}

	return sale
}

func seedSalesPPh23CashBankFixture(t *testing.T, db *gorm.DB) models.CashBank {
	t.Helper()

	cashAccount := models.Account{
		Code:     "1101-003",
		Name:     "BANK GIRO",
		Type:     models.AccountTypeAsset,
		Category: models.CategoryCurrentAsset,
		Level:    4,
		IsActive: true,
		Balance:  1000000,
	}
	if err := db.Create(&cashAccount).Error; err != nil {
		t.Fatalf("seed cash account: %v", err)
	}

	cashBank := models.CashBank{
		Code:             "BANK-001",
		Name:             "Bank Operasional",
		Type:             models.CashBankTypeBank,
		AccountID:        cashAccount.ID,
		Currency:         "IDR",
		Balance:          1000000,
		BookBalanceIDR:   1000000,
		LastExchangeRate: 1,
		IsActive:         true,
	}
	if err := db.Create(&cashBank).Error; err != nil {
		t.Fatalf("seed cash bank: %v", err)
	}
	if err := db.Create(&models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   "OPENING_BALANCE",
		Amount:          1000000,
		Currency:        "IDR",
		ExchangeRate:    1,
		BookAmountIDR:   1000000,
		BalanceAfter:    1000000,
		TransactionDate: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
	}).Error; err != nil {
		t.Fatalf("seed cash bank opening movement: %v", err)
	}

	return cashBank
}

func TestSettlePPh23CreatesJournalAndUpdatesSale(t *testing.T) {
	db := newSalesPPh23TestDB(t)
	sale := seedSalesPPh23Fixture(t, db, 0)

	service := &SalesServiceV2{db: db}
	settlementDate := time.Date(2026, time.April, 16, 0, 0, 0, 0, time.UTC)

	updatedSale, err := service.SettlePPh23(sale.ID, models.SalePPh23SettlementRequest{
		Date:      settlementDate,
		Reference: "BUPOT-001",
		Notes:     "Konfirmasi dari konsultan pajak",
	}, 1)
	if err != nil {
		t.Fatalf("expected settle PPh23 success, got %v", err)
	}

	if updatedSale.PPh23SettlementJournalEntryID == nil {
		t.Fatalf("expected PPh23 settlement journal entry id to be saved")
	}
	if updatedSale.PPh23SettlementDate == nil || !updatedSale.PPh23SettlementDate.Equal(settlementDate) {
		t.Fatalf("expected settlement date %s, got %+v", settlementDate.Format("2006-01-02"), updatedSale.PPh23SettlementDate)
	}
	if updatedSale.PPh23SettlementReference != "BUPOT-001" {
		t.Fatalf("expected settlement reference to be stored, got %s", updatedSale.PPh23SettlementReference)
	}

	var journal models.SSOTJournalEntry
	if err := db.Preload("Lines").First(&journal, *updatedSale.PPh23SettlementJournalEntryID).Error; err != nil {
		t.Fatalf("load journal: %v", err)
	}
	if journal.Status != models.SSOTStatusPosted {
		t.Fatalf("expected journal to auto post, got %s", journal.Status)
	}
	if len(journal.Lines) != 2 {
		t.Fatalf("expected 2 journal lines, got %d", len(journal.Lines))
	}
}

func TestSettlePPh23AllowsUnpaidInvoiceWithoutReducingOutstanding(t *testing.T) {
	db := newSalesPPh23TestDB(t)
	sale := seedSalesPPh23Fixture(t, db, 150000)

	service := &SalesServiceV2{db: db}
	updatedSale, err := service.SettlePPh23(sale.ID, models.SalePPh23SettlementRequest{
		Date:      time.Date(2026, time.April, 16, 0, 0, 0, 0, time.UTC),
		Reference: "BUPOT-PARTIAL-001",
	}, 1)
	if err != nil {
		t.Fatalf("expected settle PPh23 to succeed for unpaid invoice, got %v", err)
	}
	if updatedSale.OutstandingAmount != 150000 {
		t.Fatalf("expected outstanding to stay 150000 without reduction option, got %.2f", updatedSale.OutstandingAmount)
	}
	if updatedSale.Status != models.SaleStatusPaid {
		t.Fatalf("expected workflow status to stay PAID from fixture, got %s", updatedSale.Status)
	}
	if updatedSale.PPh23SettlementJournalEntryID == nil {
		t.Fatalf("expected PPh23 settlement journal entry id to be saved")
	}
}

func TestSettlePPh23CanReduceOutstandingForLegacyGrossInvoice(t *testing.T) {
	db := newSalesPPh23TestDB(t)
	baseSale := seedSalesPPh23Fixture(t, db, 0)

	sale := models.Sale{
		Code:              "SAL-GROSS-PPH",
		CustomerID:        baseSale.CustomerID,
		UserID:            baseSale.UserID,
		Type:              models.SaleTypeInvoice,
		Status:            models.SaleStatusInvoiced,
		Date:              time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		DueDate:           time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC),
		InvoiceNumber:     "INV-GROSS-PPH",
		TaxableAmount:     1000000,
		Subtotal:          1000000,
		NetBeforeTax:      1000000,
		TotalAmount:       1000000,
		PaidAmount:        500000,
		OutstandingAmount: 500000,
		PPh23Rate:         2,
		PPh23Amount:       20000,
	}
	if err := db.Create(&sale).Error; err != nil {
		t.Fatalf("seed legacy gross sale: %v", err)
	}

	service := &SalesServiceV2{db: db}
	updatedSale, err := service.SettlePPh23(sale.ID, models.SalePPh23SettlementRequest{
		Date:              time.Date(2026, time.April, 16, 0, 0, 0, 0, time.UTC),
		Reference:         "BUPOT-GROSS-001",
		ReduceOutstanding: true,
	}, 1)
	if err != nil {
		t.Fatalf("expected settle PPh23 with outstanding reduction success, got %v", err)
	}

	if updatedSale.TotalAmount != 980000 {
		t.Fatalf("expected total amount 980000 after PPh23 reduction, got %.2f", updatedSale.TotalAmount)
	}
	if updatedSale.OutstandingAmount != 480000 {
		t.Fatalf("expected outstanding amount 480000 after PPh23 reduction, got %.2f", updatedSale.OutstandingAmount)
	}

	var journal models.SSOTJournalEntry
	if err := db.Preload("Lines").First(&journal, *updatedSale.PPh23SettlementJournalEntryID).Error; err != nil {
		t.Fatalf("load reduced PPh23 journal: %v", err)
	}
	if len(journal.Lines) != 2 {
		t.Fatalf("expected 2 journal lines for reduced PPh23, got %d", len(journal.Lines))
	}
}

func TestAddLatePPh23ClosesOutstandingInvoice(t *testing.T) {
	db := newSalesPPh23TestDB(t)
	baseSale := seedSalesPPh23Fixture(t, db, 0)

	sale := models.Sale{
		Code:              "SAL-LATE-001",
		CustomerID:        baseSale.CustomerID,
		UserID:            baseSale.UserID,
		Type:              models.SaleTypeInvoice,
		Status:            models.SaleStatusInvoiced,
		Date:              time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		DueDate:           time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC),
		InvoiceNumber:     "INV-LATE-001",
		TaxableAmount:     1000000,
		Subtotal:          1000000,
		TotalAmount:       1000000,
		PaidAmount:        980000,
		OutstandingAmount: 20000,
	}
	if err := db.Create(&sale).Error; err != nil {
		t.Fatalf("seed late sale: %v", err)
	}

	service := &SalesServiceV2{db: db}
	updatedSale, err := service.AddLatePPh23(sale.ID, models.SaleLatePPh23Request{
		Date:      time.Date(2026, time.April, 16, 0, 0, 0, 0, time.UTC),
		Amount:    20000,
		Reference: "BUPOT-LATE-001",
		Notes:     "Info konsultan pajak datang belakangan",
	}, 1)
	if err != nil {
		t.Fatalf("expected add late PPh23 success, got %v", err)
	}

	if updatedSale.PPh23Amount != 20000 {
		t.Fatalf("expected late PPh23 amount 20000, got %.2f", updatedSale.PPh23Amount)
	}
	if updatedSale.TotalAmount != 980000 {
		t.Fatalf("expected updated total amount 980000, got %.2f", updatedSale.TotalAmount)
	}
	if updatedSale.OutstandingAmount != 0 {
		t.Fatalf("expected outstanding amount 0, got %.2f", updatedSale.OutstandingAmount)
	}
	if updatedSale.Status != models.SaleStatusPaid {
		t.Fatalf("expected status PAID, got %s", updatedSale.Status)
	}
	if updatedSale.PPh23SettlementJournalEntryID == nil {
		t.Fatalf("expected late PPh23 journal entry to be saved")
	}

	var journal models.SSOTJournalEntry
	if err := db.Preload("Lines").First(&journal, *updatedSale.PPh23SettlementJournalEntryID).Error; err != nil {
		t.Fatalf("load late PPh23 journal: %v", err)
	}
	if len(journal.Lines) != 2 {
		t.Fatalf("expected 2 journal lines for late PPh23, got %d", len(journal.Lines))
	}
}

func TestAddLatePPh23CanReducePartialOutstandingInvoice(t *testing.T) {
	db := newSalesPPh23TestDB(t)
	baseSale := seedSalesPPh23Fixture(t, db, 0)

	sale := models.Sale{
		Code:              "SAL-LATE-PARTIAL",
		CustomerID:        baseSale.CustomerID,
		UserID:            baseSale.UserID,
		Type:              models.SaleTypeInvoice,
		Status:            models.SaleStatusInvoiced,
		Date:              time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		DueDate:           time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC),
		InvoiceNumber:     "INV-LATE-PARTIAL",
		TaxableAmount:     1000000,
		Subtotal:          1000000,
		TotalAmount:       1000000,
		PaidAmount:        500000,
		OutstandingAmount: 500000,
	}
	if err := db.Create(&sale).Error; err != nil {
		t.Fatalf("seed partial late sale: %v", err)
	}

	service := &SalesServiceV2{db: db}
	updatedSale, err := service.AddLatePPh23(sale.ID, models.SaleLatePPh23Request{
		Date:              time.Date(2026, time.April, 16, 0, 0, 0, 0, time.UTC),
		Amount:            20000,
		Reference:         "BUPOT-LATE-PARTIAL",
		ReduceOutstanding: true,
	}, 1)
	if err != nil {
		t.Fatalf("expected add late PPh23 partial outstanding success, got %v", err)
	}

	if updatedSale.TotalAmount != 980000 {
		t.Fatalf("expected updated total amount 980000, got %.2f", updatedSale.TotalAmount)
	}
	if updatedSale.OutstandingAmount != 480000 {
		t.Fatalf("expected outstanding amount 480000, got %.2f", updatedSale.OutstandingAmount)
	}
	if updatedSale.Status != models.SaleStatusInvoiced {
		t.Fatalf("expected status INVOICED, got %s", updatedSale.Status)
	}
	if updatedSale.PPh23SettlementJournalEntryID == nil {
		t.Fatalf("expected late PPh23 journal entry to be saved")
	}
}

func TestAddLatePPh23HandlesExistingUnderpaidTax(t *testing.T) {
	db := newSalesPPh23TestDB(t)
	sale := seedSalesPPh23Fixture(t, db, 10000)

	service := &SalesServiceV2{db: db}
	updatedSale, err := service.AddLatePPh23(sale.ID, models.SaleLatePPh23Request{
		Date:      time.Date(2026, time.April, 16, 0, 0, 0, 0, time.UTC),
		Amount:    30000,
		Reference: "BUPOT-UNDERPAID-001",
		Notes:     "PPh23 final lebih besar dari estimasi awal",
	}, 1)
	if err != nil {
		t.Fatalf("expected existing PPh23 underpayment adjustment success, got %v", err)
	}

	if updatedSale.PPh23Amount != 30000 {
		t.Fatalf("expected adjusted PPh23 amount 30000, got %.2f", updatedSale.PPh23Amount)
	}
	if updatedSale.TotalAmount != 970000 {
		t.Fatalf("expected updated total amount 970000, got %.2f", updatedSale.TotalAmount)
	}
	if updatedSale.OutstandingAmount != 0 {
		t.Fatalf("expected outstanding amount 0, got %.2f", updatedSale.OutstandingAmount)
	}
	if updatedSale.Status != models.SaleStatusPaid {
		t.Fatalf("expected status PAID, got %s", updatedSale.Status)
	}

	var journal models.SSOTJournalEntry
	if err := db.Preload("Lines").First(&journal, *updatedSale.PPh23SettlementJournalEntryID).Error; err != nil {
		t.Fatalf("load underpaid PPh23 journal: %v", err)
	}
	if len(journal.Lines) != 3 {
		t.Fatalf("expected 3 journal lines for underpaid PPh23, got %d", len(journal.Lines))
	}

	var debitTotal float64
	var creditTotal float64
	for _, line := range journal.Lines {
		debitTotal += line.DebitAmount.InexactFloat64()
		creditTotal += line.CreditAmount.InexactFloat64()
	}
	if debitTotal != 30000 || creditTotal != 30000 {
		t.Fatalf("expected balanced PPh23 journal 30000/30000, got debit %.2f credit %.2f", debitTotal, creditTotal)
	}
}

func TestAddLatePPh23OnPaidInvoiceCreatesPaymentCorrection(t *testing.T) {
	db := newSalesPPh23TestDB(t)
	baseSale := seedSalesPPh23Fixture(t, db, 0)
	cashBank := seedSalesPPh23CashBankFixture(t, db)

	sale := models.Sale{
		Code:              "SAL-LATE-PAID",
		CustomerID:        baseSale.CustomerID,
		UserID:            baseSale.UserID,
		Type:              models.SaleTypeInvoice,
		Status:            models.SaleStatusPaid,
		Date:              time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		DueDate:           time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC),
		InvoiceNumber:     "INV-LATE-PAID",
		TaxableAmount:     1000000,
		Subtotal:          1000000,
		TotalAmount:       1000000,
		PaidAmount:        1000000,
		OutstandingAmount: 0,
		CashBankID:        &cashBank.ID,
		PaymentMethod:     "BANK",
		PaymentMethodType: "BANK",
	}
	if err := db.Create(&sale).Error; err != nil {
		t.Fatalf("seed paid late sale: %v", err)
	}

	payment := models.SalePayment{
		SaleID:                sale.ID,
		Amount:                1000000,
		NativeAmount:          1000000,
		Currency:              "IDR",
		ExchangeRate:          1,
		BookAmountIDR:         1000000,
		PaymentDate:           time.Date(2026, time.April, 10, 0, 0, 0, 0, time.UTC),
		PaymentMethod:         "BANK",
		Reference:             "PAY-001",
		CashBankID:            &cashBank.ID,
		CashBankCode:          cashBank.Code,
		CashBankName:          cashBank.Name,
		CashBankCurrency:      "IDR",
		CashBankGLAccountID:   &cashBank.AccountID,
		CashBankGLAccountCode: "1101-003",
		CashBankGLAccountName: "BANK GIRO",
		UserID:                sale.UserID,
		Status:                models.SalePaymentStatusCompleted,
	}
	if err := db.Create(&payment).Error; err != nil {
		t.Fatalf("seed sale payment: %v", err)
	}

	service := &SalesServiceV2{db: db}
	updatedSale, err := service.AddLatePPh23(sale.ID, models.SaleLatePPh23Request{
		Date:      time.Date(2026, time.April, 16, 0, 0, 0, 0, time.UTC),
		Amount:    20000,
		Reference: "BUPOT-LATE-PAID",
	}, 1)
	if err != nil {
		t.Fatalf("expected add late PPh23 on paid invoice success, got %v", err)
	}

	if updatedSale.TotalAmount != 980000 {
		t.Fatalf("expected updated total amount 980000, got %.2f", updatedSale.TotalAmount)
	}
	if updatedSale.PaidAmount != 980000 {
		t.Fatalf("expected updated paid amount 980000, got %.2f", updatedSale.PaidAmount)
	}
	if updatedSale.OutstandingAmount != 0 {
		t.Fatalf("expected outstanding amount 0, got %.2f", updatedSale.OutstandingAmount)
	}

	var correctionPayments []models.SalePayment
	if err := db.Where("sale_id = ? AND amount < 0", sale.ID).Find(&correctionPayments).Error; err != nil {
		t.Fatalf("load correction payments: %v", err)
	}
	if len(correctionPayments) != 1 {
		t.Fatalf("expected 1 correction payment, got %d", len(correctionPayments))
	}
	if correctionPayments[0].Amount != -20000 {
		t.Fatalf("expected correction payment amount -20000, got %.2f", correctionPayments[0].Amount)
	}

	var cashBankAfter models.CashBank
	if err := db.First(&cashBankAfter, cashBank.ID).Error; err != nil {
		t.Fatalf("reload cash bank: %v", err)
	}
	if cashBankAfter.Balance != 980000 {
		t.Fatalf("expected cash bank balance 980000, got %.2f", cashBankAfter.Balance)
	}

	var cashBankTransactions []models.CashBankTransaction
	if err := db.Where("reference_type = ? AND reference_id = ?", "SALES_PAYMENT", correctionPayments[0].ID).Find(&cashBankTransactions).Error; err != nil {
		t.Fatalf("load cash/bank correction transactions: %v", err)
	}
	if len(cashBankTransactions) != 1 {
		t.Fatalf("expected 1 cash/bank correction transaction, got %d", len(cashBankTransactions))
	}
	if cashBankTransactions[0].Amount != -20000 {
		t.Fatalf("expected cash/bank correction amount -20000, got %.2f", cashBankTransactions[0].Amount)
	}
}
