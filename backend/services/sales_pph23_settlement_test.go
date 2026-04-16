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
		&models.Sale{},
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

func TestSettlePPh23RequiresPaidInvoice(t *testing.T) {
	db := newSalesPPh23TestDB(t)
	sale := seedSalesPPh23Fixture(t, db, 150000)

	service := &SalesServiceV2{db: db}
	_, err := service.SettlePPh23(sale.ID, models.SalePPh23SettlementRequest{
		Date: time.Date(2026, time.April, 16, 0, 0, 0, 0, time.UTC),
	}, 1)
	if err == nil {
		t.Fatalf("expected settle PPh23 to fail for unpaid invoice")
	}
}
