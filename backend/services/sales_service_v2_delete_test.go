package services

import (
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDeleteSalePaidWithRecordedPaymentRestoresCashBankAndVAT(t *testing.T) {
	db := openSalesDeleteTestDB(t)

	user := models.User{
		Username:  "sales-finance",
		Email:     "sales-finance@test.local",
		Password:  "secret",
		Role:      "finance",
		FirstName: "Sales Finance",
	}
	require.NoError(t, db.Create(&user).Error)

	customer := models.Contact{
		Code: "CUST-001",
		Name: "PT Maju Jaya",
		Type: "CUSTOMER",
	}
	require.NoError(t, db.Create(&customer).Error)

	bankAccount := createTestAccount(t, db, "1101-001", "BANK TEST", models.AccountTypeAsset, 1110)
	arAccount := createTestAccount(t, db, "1102-001", "PIUTANG USAHA", models.AccountTypeAsset, 0)
	revenueAccount := createTestAccount(t, db, "4101-001", "PENDAPATAN PENJUALAN", models.AccountTypeRevenue, 100)
	ppnOutputAccount := createTestAccount(t, db, "2102-001", "PPN KELUARAN", models.AccountTypeLiability, 10)

	cashBank := models.CashBank{
		Code:             "BNK-SALES-001",
		Name:             "BANK TEST",
		Type:             models.CashBankTypeBank,
		AccountID:        bankAccount.ID,
		Currency:         "IDR",
		Balance:          1110,
		BookBalanceIDR:   1110,
		LastExchangeRate: 1,
		IsActive:         true,
		UserID:           user.ID,
	}
	require.NoError(t, db.Create(&cashBank).Error)

	sale := models.Sale{
		Code:              "SOA-DELETE-0001",
		InvoiceNumber:     "INV-DELETE-0001",
		CustomerID:        customer.ID,
		UserID:            user.ID,
		Date:              time.Date(2026, 3, 24, 10, 0, 0, 0, time.UTC),
		Status:            models.SaleStatusPaid,
		PaymentMethodType: "CREDIT",
		TotalAmount:       110,
		PaidAmount:        110,
		OutstandingAmount: 0,
	}
	require.NoError(t, db.Create(&sale).Error)

	salePayment := models.SalePayment{
		SaleID:        sale.ID,
		Amount:        110,
		NativeAmount:  110,
		Currency:      "IDR",
		ExchangeRate:  1,
		BookAmountIDR: 110,
		PaymentDate:   sale.Date,
		PaymentMethod: "BANK_TRANSFER",
		CashBankID:    &cashBank.ID,
		UserID:        user.ID,
		Status:        "COMPLETED",
	}
	require.NoError(t, db.Create(&salePayment).Error)

	saleSourceID := uint64(sale.ID)
	salePaymentSourceID := uint64(salePayment.ID)
	createPostedSSOTJournal(t, db, "SALES", &saleSourceID, sale.Code, []models.SSOTJournalLine{
		makeSSOTLine(1, uint64(arAccount.ID), 110, 0, "Piutang penjualan"),
		makeSSOTLine(2, uint64(revenueAccount.ID), 0, 100, "Pendapatan penjualan"),
		makeSSOTLine(3, uint64(ppnOutputAccount.ID), 0, 10, "PPN keluaran"),
	})
	createPostedSSOTJournal(t, db, "SALES_PAYMENT", &salePaymentSourceID, "PAY-DELETE-0001", []models.SSOTJournalLine{
		makeSSOTLine(1, uint64(bankAccount.ID), 110, 0, "Penerimaan bank"),
		makeSSOTLine(2, uint64(arAccount.ID), 0, 110, "Pelunasan piutang"),
	})

	require.NoError(t, db.Create(&models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   "OPENING",
		ReferenceID:     1,
		Amount:          1000,
		Currency:        "IDR",
		ExchangeRate:    1,
		BookAmountIDR:   1000,
		BalanceAfter:    1000,
		TransactionDate: sale.Date.Add(-time.Hour),
		Notes:           "Opening balance",
	}).Error)
	require.NoError(t, db.Create(&models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   "SALES_PAYMENT",
		ReferenceID:     salePayment.ID,
		Amount:          110,
		Currency:        "IDR",
		ExchangeRate:    1,
		BookAmountIDR:   110,
		BalanceAfter:    1110,
		TransactionDate: sale.Date,
		Notes:           "Customer payment",
	}).Error)

	svc := newSalesDeleteTestService(db)
	require.NoError(t, svc.DeleteSale(sale.ID))

	var refreshedSale models.Sale
	require.NoError(t, db.First(&refreshedSale, sale.ID).Error)
	require.Equal(t, models.SaleStatusCancelled, refreshedSale.Status)
	require.InDelta(t, 0, refreshedSale.PaidAmount, 0.01)
	require.InDelta(t, 0, refreshedSale.OutstandingAmount, 0.01)

	var refreshedPayment models.SalePayment
	require.NoError(t, db.First(&refreshedPayment, salePayment.ID).Error)
	require.Equal(t, "CANCELLED", refreshedPayment.Status)

	var refreshedCashBank models.CashBank
	require.NoError(t, db.First(&refreshedCashBank, cashBank.ID).Error)
	require.InDelta(t, 1000, refreshedCashBank.Balance, 0.01)
	require.InDelta(t, 1000, refreshedCashBank.BookBalanceIDR, 0.01)

	var reversalCount int64
	require.NoError(t, db.Model(&models.CashBankTransaction{}).
		Where("cash_bank_id = ? AND reference_type = ? AND reference_id = ?", cashBank.ID, "SALES_PAYMENT_REVERSAL", salePayment.ID).
		Count(&reversalCount).Error)
	require.Equal(t, int64(1), reversalCount)

	requireAccountBalance(t, db, bankAccount.ID, 1000)
	requireAccountBalance(t, db, arAccount.ID, 0)
	requireAccountBalance(t, db, revenueAccount.ID, 0)
	requireAccountBalance(t, db, ppnOutputAccount.ID, 0)
}

func TestDeleteSalePaidImmediatePaymentCreatesSaleReversalTransaction(t *testing.T) {
	db := openSalesDeleteTestDB(t)

	user := models.User{
		Username:  "sales-cashier",
		Email:     "sales-cashier@test.local",
		Password:  "secret",
		Role:      "finance",
		FirstName: "Sales Cashier",
	}
	require.NoError(t, db.Create(&user).Error)

	customer := models.Contact{
		Code: "CUST-002",
		Name: "PT Cepat Bayar",
		Type: "CUSTOMER",
	}
	require.NoError(t, db.Create(&customer).Error)

	bankAccount := createTestAccount(t, db, "1101-002", "BANK IMMEDIATE", models.AccountTypeAsset, 1110)
	revenueAccount := createTestAccount(t, db, "4101-002", "PENDAPATAN TUNAI", models.AccountTypeRevenue, 100)
	ppnOutputAccount := createTestAccount(t, db, "2102-002", "PPN KELUARAN TUNAI", models.AccountTypeLiability, 10)

	cashBank := models.CashBank{
		Code:             "BNK-SALES-002",
		Name:             "BANK IMMEDIATE",
		Type:             models.CashBankTypeBank,
		AccountID:        bankAccount.ID,
		Currency:         "IDR",
		Balance:          1110,
		BookBalanceIDR:   1110,
		LastExchangeRate: 1,
		IsActive:         true,
		UserID:           user.ID,
	}
	require.NoError(t, db.Create(&cashBank).Error)

	sale := models.Sale{
		Code:              "SOA-DELETE-0002",
		InvoiceNumber:     "INV-DELETE-0002",
		CustomerID:        customer.ID,
		UserID:            user.ID,
		Date:              time.Date(2026, 3, 24, 11, 0, 0, 0, time.UTC),
		Status:            models.SaleStatusPaid,
		PaymentMethodType: "BANK",
		CashBankID:        &cashBank.ID,
		TotalAmount:       110,
		PaidAmount:        110,
		OutstandingAmount: 0,
	}
	require.NoError(t, db.Create(&sale).Error)

	saleSourceID := uint64(sale.ID)
	createPostedSSOTJournal(t, db, "SALES", &saleSourceID, sale.Code, []models.SSOTJournalLine{
		makeSSOTLine(1, uint64(bankAccount.ID), 110, 0, "Penerimaan langsung bank"),
		makeSSOTLine(2, uint64(revenueAccount.ID), 0, 100, "Pendapatan penjualan"),
		makeSSOTLine(3, uint64(ppnOutputAccount.ID), 0, 10, "PPN keluaran"),
	})

	require.NoError(t, db.Create(&models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   "OPENING",
		ReferenceID:     1,
		Amount:          1000,
		Currency:        "IDR",
		ExchangeRate:    1,
		BookAmountIDR:   1000,
		BalanceAfter:    1000,
		TransactionDate: sale.Date.Add(-time.Hour),
		Notes:           "Opening balance",
	}).Error)
	require.NoError(t, db.Create(&models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   "SALE",
		ReferenceID:     sale.ID,
		Amount:          110,
		Currency:        "IDR",
		ExchangeRate:    1,
		BookAmountIDR:   110,
		BalanceAfter:    1110,
		TransactionDate: sale.Date,
		Notes:           "Immediate sale payment",
	}).Error)

	svc := newSalesDeleteTestService(db)
	require.NoError(t, svc.DeleteSale(sale.ID))

	var refreshedSale models.Sale
	require.NoError(t, db.First(&refreshedSale, sale.ID).Error)
	require.Equal(t, models.SaleStatusCancelled, refreshedSale.Status)

	var refreshedCashBank models.CashBank
	require.NoError(t, db.First(&refreshedCashBank, cashBank.ID).Error)
	require.InDelta(t, 1000, refreshedCashBank.Balance, 0.01)
	require.InDelta(t, 1000, refreshedCashBank.BookBalanceIDR, 0.01)

	var reversalCount int64
	require.NoError(t, db.Model(&models.CashBankTransaction{}).
		Where("cash_bank_id = ? AND reference_type = ? AND reference_id = ?", cashBank.ID, "SALE_REVERSAL", sale.ID).
		Count(&reversalCount).Error)
	require.Equal(t, int64(1), reversalCount)

	requireAccountBalance(t, db, bankAccount.ID, 1000)
	requireAccountBalance(t, db, revenueAccount.ID, 0)
	requireAccountBalance(t, db, ppnOutputAccount.ID, 0)
}

func openSalesDeleteTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Account{},
		&models.CashBank{},
		&models.CashBankTransaction{},
		&models.Sale{},
		&models.SaleItem{},
		&models.SalePayment{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
	))

	return db
}

func newSalesDeleteTestService(db *gorm.DB) *SalesServiceV2 {
	return &SalesServiceV2{
		db:                  db,
		salesJournalService: &SalesJournalServiceV2{db: db, coaService: NewCOAService(db)},
	}
}
