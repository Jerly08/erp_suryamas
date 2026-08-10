package services

import (
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type confirmedSaleCreditConversionFixture struct {
	db         *gorm.DB
	service    *SalesServiceV2
	sale       models.Sale
	receivable models.Account
}

func newConfirmedSaleCreditConversionFixture(t *testing.T) confirmedSaleCreditConversionFixture {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Account{},
		&models.AccountingPeriod{},
		&models.CashBank{},
		&models.CashBankTransaction{},
		&models.Project{},
		&models.InvoiceType{},
		&models.Product{},
		&models.Sale{},
		&models.SaleItem{},
		&models.SalePayment{},
		&models.SalePaymentSchedule{},
		&models.Payment{},
		&models.PaymentAllocation{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
	))

	user := models.User{
		Username: "credit-conversion-user-" + t.Name(),
		Email:    "credit-conversion-" + t.Name() + "@test.local",
		Password: "secret",
		Role:     models.RoleFinance,
		IsActive: true,
	}
	require.NoError(t, db.Create(&user).Error)

	customer := models.Contact{
		Code:     "CUST-" + t.Name(),
		Name:     "Pelanggan Uji Konversi Kredit",
		Type:     models.ContactTypeCustomer,
		IsActive: true,
	}
	require.NoError(t, db.Create(&customer).Error)

	receivable := models.Account{
		Code:     "1102-001-" + t.Name(),
		Name:     "PIUTANG USAHA - PELANGGAN UJI",
		Type:     models.AccountTypeAsset,
		IsActive: true,
		IsHeader: false,
	}
	require.NoError(t, db.Create(&receivable).Error)

	saleDate := time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)
	sale := models.Sale{
		Code:              "SOA-CREDIT-" + t.Name(),
		CustomerID:        customer.ID,
		UserID:            user.ID,
		Type:              models.SaleTypeInvoice,
		Date:              saleDate,
		Currency:          "IDR",
		ExchangeRate:      1,
		TotalAmount:       100_000,
		OutstandingAmount: 100_000,
		PaymentTerms:      "CASH",
		PaymentMethod:     "BANK TRANSFER",
		PaymentMethodType: "BANK",
		Status:            models.SaleStatusConfirmed,
	}
	require.NoError(t, db.Create(&sale).Error)

	return confirmedSaleCreditConversionFixture{
		db:         db,
		service:    &SalesServiceV2{db: db},
		sale:       sale,
		receivable: receivable,
	}
}

func TestConvertConfirmedSaleToCreditConvertsOnlyNonFinancialSale(t *testing.T) {
	fixture := newConfirmedSaleCreditConversionFixture(t)
	dueDate := fixture.sale.Date.AddDate(0, 0, 30)

	converted, err := fixture.service.ConvertConfirmedSaleToCredit(
		fixture.sale.ID,
		models.ConfirmedSaleCreditConversionRequest{
			PaymentTerms:        "NET_30",
			DueDate:             dueDate,
			ReceivableAccountID: fixture.receivable.ID,
		},
		1,
	)

	require.NoError(t, err)
	require.Equal(t, models.SaleStatusConfirmed, converted.Status)
	require.Equal(t, "CREDIT", converted.PaymentMethod)
	require.Equal(t, "CREDIT", converted.PaymentMethodType)
	require.Equal(t, "NET_30", converted.PaymentTerms)
	require.Equal(t, dueDate, converted.DueDate)
	require.NotNil(t, converted.ReceivableAccountID)
	require.Equal(t, fixture.receivable.ID, *converted.ReceivableAccountID)
	require.Nil(t, converted.CashBankID)

	var journalCount, paymentCount, allocationCount, cashBankTransactionCount int64
	require.NoError(t, fixture.db.Model(&models.SSOTJournalEntry{}).Count(&journalCount).Error)
	require.NoError(t, fixture.db.Model(&models.SalePayment{}).Count(&paymentCount).Error)
	require.NoError(t, fixture.db.Model(&models.PaymentAllocation{}).Count(&allocationCount).Error)
	require.NoError(t, fixture.db.Model(&models.CashBankTransaction{}).Count(&cashBankTransactionCount).Error)
	require.Zero(t, journalCount)
	require.Zero(t, paymentCount)
	require.Zero(t, allocationCount)
	require.Zero(t, cashBankTransactionCount)
}

func TestConvertConfirmedSaleToCreditRejectsSaleWithJournal(t *testing.T) {
	fixture := newConfirmedSaleCreditConversionFixture(t)
	sourceID := uint64(fixture.sale.ID)
	journal := models.SSOTJournalEntry{
		EntryNumber: "SALE-CREDIT-CONVERSION-TEST",
		SourceType:  models.SSOTSourceTypeSale,
		SourceID:    &sourceID,
		SourceCode:  fixture.sale.Code,
		EntryDate:   fixture.sale.Date,
		Description: "Jurnal penjualan yang sudah ada",
		TotalDebit:  decimal.NewFromInt(100_000),
		TotalCredit: decimal.NewFromInt(100_000),
		Status:      models.SSOTStatusPosted,
		IsBalanced:  true,
		CreatedBy:   1,
	}
	require.NoError(t, fixture.db.Create(&journal).Error)

	_, err := fixture.service.ConvertConfirmedSaleToCredit(
		fixture.sale.ID,
		models.ConfirmedSaleCreditConversionRequest{
			PaymentTerms:        "NET_30",
			DueDate:             fixture.sale.Date.AddDate(0, 0, 30),
			ReceivableAccountID: fixture.receivable.ID,
		},
		1,
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "sudah memiliki jurnal")

	var sale models.Sale
	require.NoError(t, fixture.db.First(&sale, fixture.sale.ID).Error)
	require.Equal(t, "BANK", sale.PaymentMethodType)
	require.Equal(t, "CASH", sale.PaymentTerms)
}
