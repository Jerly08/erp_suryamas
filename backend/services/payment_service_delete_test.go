package services

import (
	"testing"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCancelTaxPaymentRestoresPPNAndCashBankBalances(t *testing.T) {
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
		&models.Payment{},
		&models.PaymentAllocation{},
		&models.Journal{},
		&models.JournalEntry{},
		&models.JournalLine{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
		&models.TaxAccountSettings{},
	))
	require.NoError(t, db.Exec(`ALTER TABLE unified_journal_lines ADD COLUMN deleted_at DATETIME`).Error)

	user := models.User{
		Username:  "finance-test",
		Email:     "finance@test.local",
		Password:  "secret",
		Role:      "finance",
		FirstName: "Finance",
	}
	require.NoError(t, db.Create(&user).Error)

	contact := models.Contact{
		Code: "TAX-001",
		Name: "Negara",
		Type: "VENDOR",
	}
	require.NoError(t, db.Create(&contact).Error)

	bankAccount := createTestAccount(t, db, "1101-001", "BANK TEST", models.AccountTypeAsset, 940)
	equityAccount := createTestAccount(t, db, "3101-001", "MODAL", models.AccountTypeEquity, 1000)
	arAccount := createTestAccount(t, db, "1102-001", "PIUTANG", models.AccountTypeAsset, 110)
	apAccount := createTestAccount(t, db, "2101-001", "UTANG", models.AccountTypeLiability, 50)
	revenueAccount := createTestAccount(t, db, "4101-001", "PENJUALAN", models.AccountTypeRevenue, 0)
	expenseAccount := createTestAccount(t, db, "5101-001", "BEBAN", models.AccountTypeExpense, 0)
	ppnOutputAccount := createTestAccount(t, db, "2102-001", "PPN KELUARAN", models.AccountTypeLiability, 0)
	ppnInputAccount := createTestAccount(t, db, "2102-002", "PPN MASUKAN", models.AccountTypeLiability, 0)

	settings := models.TaxAccountSettings{
		SalesReceivableAccountID:  arAccount.ID,
		SalesCashAccountID:        bankAccount.ID,
		SalesBankAccountID:        bankAccount.ID,
		SalesRevenueAccountID:     revenueAccount.ID,
		SalesOutputVATAccountID:   ppnOutputAccount.ID,
		PurchasePayableAccountID:  apAccount.ID,
		PurchaseCashAccountID:     bankAccount.ID,
		PurchaseBankAccountID:     bankAccount.ID,
		PurchaseInputVATAccountID: ppnInputAccount.ID,
		PurchaseExpenseAccountID:  expenseAccount.ID,
		IsActive:                  true,
		ApplyToAllCompanies:       true,
		UpdatedBy:                 user.ID,
	}
	require.NoError(t, db.Create(&settings).Error)

	cashBank := models.CashBank{
		Code:             "BNK-TEST-001",
		Name:             "BANK TEST",
		Type:             models.CashBankTypeBank,
		AccountID:        bankAccount.ID,
		Currency:         "IDR",
		Balance:          940,
		BookBalanceIDR:   940,
		LastExchangeRate: 1,
		IsActive:         true,
		UserID:           user.ID,
	}
	require.NoError(t, db.Create(&cashBank).Error)

	payment := models.Payment{
		Code:        "SETOR-PPN-TEST-0001",
		ContactID:   contact.ID,
		UserID:      user.ID,
		Date:        time.Date(2026, 3, 24, 10, 0, 0, 0, time.UTC),
		Amount:      60,
		Method:      models.PaymentMethodBankTransfer,
		Status:      models.PaymentStatusCompleted,
		PaymentType: models.PaymentTypeTaxPPN,
		CashBankID:  &cashBank.ID,
	}
	require.NoError(t, db.Create(&payment).Error)

	paymentSourceID := uint64(payment.ID)
	baseOpeningJournal := createPostedSSOTJournal(t, db, "OPENING", nil, "OPENING-BANK", []models.SSOTJournalLine{
		makeSSOTLine(1, uint64(bankAccount.ID), 1000, 0, "Opening bank"),
		makeSSOTLine(2, uint64(equityAccount.ID), 0, 1000, "Opening equity"),
	})
	createPostedSSOTJournal(t, db, "SALE", nil, "SALE-PPN", []models.SSOTJournalLine{
		makeSSOTLine(1, uint64(arAccount.ID), 110, 0, "Piutang"),
		makeSSOTLine(2, uint64(ppnOutputAccount.ID), 0, 110, "PPN Keluaran"),
	})
	createPostedSSOTJournal(t, db, "PURCHASE", nil, "PURCHASE-PPN", []models.SSOTJournalLine{
		makeSSOTLine(1, uint64(ppnInputAccount.ID), 50, 0, "PPN Masukan"),
		makeSSOTLine(2, uint64(apAccount.ID), 0, 50, "Utang"),
	})
	paymentJournal := createPostedSSOTJournal(t, db, models.SSOTSourceTypePayment, &paymentSourceID, payment.Code, []models.SSOTJournalLine{
		makeSSOTLine(1, uint64(ppnOutputAccount.ID), 110, 0, "Setor PPN keluaran"),
		makeSSOTLine(2, uint64(ppnInputAccount.ID), 0, 50, "Kompensasi PPN masukan"),
		makeSSOTLine(3, uint64(bankAccount.ID), 0, 60, "Pembayaran bank"),
	})

	payment.JournalEntryID = paymentTestUintPtr(uint(paymentJournal.ID))
	require.NoError(t, db.Save(&payment).Error)

	require.NoError(t, db.Create(&models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   "OPENING",
		ReferenceID:     uint(baseOpeningJournal.ID),
		Amount:          1000,
		Currency:        "IDR",
		ExchangeRate:    1,
		BookAmountIDR:   1000,
		BalanceAfter:    1000,
		TransactionDate: payment.Date.Add(-time.Hour),
		Notes:           "Opening balance",
	}).Error)
	require.NoError(t, db.Create(&models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   "PAYMENT",
		ReferenceID:     payment.ID,
		Amount:          -60,
		Currency:        "IDR",
		ExchangeRate:    1,
		BookAmountIDR:   -60,
		BalanceAfter:    940,
		TransactionDate: payment.Date,
		Notes:           "Setor PPN",
	}).Error)

	svc := &PaymentService{db: db}
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		var currentPayment models.Payment
		if err := tx.First(&currentPayment, payment.ID).Error; err != nil {
			return err
		}
		return svc.cancelPaymentTransaction(tx, &currentPayment, "hapus setor PPN test", user.ID)
	}))

	var refreshedPayment models.Payment
	require.NoError(t, db.First(&refreshedPayment, payment.ID).Error)
	require.Equal(t, models.PaymentStatusFailed, refreshedPayment.Status)

	var refreshedCashBank models.CashBank
	require.NoError(t, db.First(&refreshedCashBank, cashBank.ID).Error)
	require.InDelta(t, 1000, refreshedCashBank.Balance, 0.01)
	require.InDelta(t, 1000, refreshedCashBank.BookBalanceIDR, 0.01)

	var reversalCount int64
	require.NoError(t, db.Model(&models.CashBankTransaction{}).
		Where("cash_bank_id = ? AND reference_type = ? AND reference_id = ?", cashBank.ID, "PAYMENT_REVERSAL", payment.ID).
		Count(&reversalCount).Error)
	require.Equal(t, int64(1), reversalCount)

	requireAccountBalance(t, db, ppnOutputAccount.ID, 110)
	requireAccountBalance(t, db, ppnInputAccount.ID, -50)
	requireAccountBalance(t, db, bankAccount.ID, 1000)

	var paymentEntry models.SSOTJournalEntry
	require.NoError(t, db.First(&paymentEntry, paymentJournal.ID).Error)
	require.NotNil(t, paymentEntry.DeletedAt)

	taxPaymentService := NewTaxPaymentService(db)
	ppnInputBalance, err := taxPaymentService.GetPPNBalance("INPUT")
	require.NoError(t, err)
	ppnOutputBalance, err := taxPaymentService.GetPPNBalance("OUTPUT")
	require.NoError(t, err)
	require.InDelta(t, 50, ppnInputBalance, 0.01)
	require.InDelta(t, 110, ppnOutputBalance, 0.01)
}

func TestDeleteTaxPaymentKeepsAuditTrailForPPNHistory(t *testing.T) {
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
		&models.Payment{},
		&models.PaymentAllocation{},
		&models.Journal{},
		&models.JournalEntry{},
		&models.JournalLine{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
		&models.TaxAccountSettings{},
	))
	require.NoError(t, db.Exec(`ALTER TABLE unified_journal_lines ADD COLUMN deleted_at DATETIME`).Error)

	user := models.User{
		Username:  "finance-delete",
		Email:     "finance-delete@test.local",
		Password:  "secret",
		Role:      "finance",
		FirstName: "Finance Delete",
	}
	require.NoError(t, db.Create(&user).Error)

	contact := models.Contact{
		Code: "TAX-002",
		Name: "Negara",
		Type: "VENDOR",
	}
	require.NoError(t, db.Create(&contact).Error)

	bankAccount := createTestAccount(t, db, "1101-101", "BANK DELETE TEST", models.AccountTypeAsset, 940)
	equityAccount := createTestAccount(t, db, "3101-101", "MODAL DELETE", models.AccountTypeEquity, 1000)
	ppnOutputAccount := createTestAccount(t, db, "2102-101", "PPN KELUARAN DELETE", models.AccountTypeLiability, 0)
	ppnInputAccount := createTestAccount(t, db, "2102-102", "PPN MASUKAN DELETE", models.AccountTypeLiability, 0)

	settings := models.TaxAccountSettings{
		SalesCashAccountID:        bankAccount.ID,
		SalesBankAccountID:        bankAccount.ID,
		SalesOutputVATAccountID:   ppnOutputAccount.ID,
		PurchaseCashAccountID:     bankAccount.ID,
		PurchaseBankAccountID:     bankAccount.ID,
		PurchaseInputVATAccountID: ppnInputAccount.ID,
		IsActive:                  true,
		ApplyToAllCompanies:       true,
		UpdatedBy:                 user.ID,
	}
	require.NoError(t, db.Create(&settings).Error)

	cashBank := models.CashBank{
		Code:             "BNK-DELETE-001",
		Name:             "BANK DELETE TEST",
		Type:             models.CashBankTypeBank,
		AccountID:        bankAccount.ID,
		Currency:         "IDR",
		Balance:          940,
		BookBalanceIDR:   940,
		LastExchangeRate: 1,
		IsActive:         true,
		UserID:           user.ID,
	}
	require.NoError(t, db.Create(&cashBank).Error)

	payment := models.Payment{
		Code:        "SETOR-PPN-DELETE-0001",
		ContactID:   contact.ID,
		UserID:      user.ID,
		Date:        time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC),
		Amount:      60,
		Method:      models.PaymentMethodBankTransfer,
		Status:      models.PaymentStatusCompleted,
		PaymentType: models.PaymentTypeTaxPPNOutput,
		CashBankID:  &cashBank.ID,
	}
	require.NoError(t, db.Create(&payment).Error)

	paymentSourceID := uint64(payment.ID)
	baseOpeningJournal := createPostedSSOTJournal(t, db, "OPENING", nil, "OPENING-BANK-DELETE", []models.SSOTJournalLine{
		makeSSOTLine(1, uint64(bankAccount.ID), 1000, 0, "Opening bank"),
		makeSSOTLine(2, uint64(equityAccount.ID), 0, 1000, "Opening equity"),
	})
	paymentJournal := createPostedSSOTJournal(t, db, models.SSOTSourceTypePayment, &paymentSourceID, payment.Code, []models.SSOTJournalLine{
		makeSSOTLine(1, uint64(ppnOutputAccount.ID), 60, 0, "Setor PPN keluaran"),
		makeSSOTLine(2, uint64(bankAccount.ID), 0, 60, "Pembayaran bank"),
	})
	payment.JournalEntryID = paymentTestUintPtr(uint(paymentJournal.ID))
	require.NoError(t, db.Save(&payment).Error)

	require.NoError(t, db.Create(&models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   "OPENING",
		ReferenceID:     uint(baseOpeningJournal.ID),
		Amount:          1000,
		Currency:        "IDR",
		ExchangeRate:    1,
		BookAmountIDR:   1000,
		BalanceAfter:    1000,
		TransactionDate: payment.Date.Add(-time.Hour),
		Notes:           "Opening balance",
	}).Error)
	require.NoError(t, db.Create(&models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   "PAYMENT",
		ReferenceID:     payment.ID,
		Amount:          -60,
		Currency:        "IDR",
		ExchangeRate:    1,
		BookAmountIDR:   -60,
		BalanceAfter:    940,
		TransactionDate: payment.Date,
		Notes:           "Setor PPN",
	}).Error)

	paymentService := &PaymentService{db: db}
	require.NoError(t, paymentService.DeletePayment(payment.ID, "hapus setor PPN untuk audit", user.ID))

	var keptPayment models.Payment
	require.NoError(t, db.First(&keptPayment, payment.ID).Error)
	require.Equal(t, models.PaymentStatusFailed, keptPayment.Status)

	var visiblePPNPayments []models.Payment
	taxPaymentService := NewTaxPaymentService(db)
	visiblePPNPayments, err = taxPaymentService.GetPPNPaymentsByType(models.PaymentTypeTaxPPNOutput, time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, visiblePPNPayments, 1)
	require.Equal(t, models.PaymentStatusFailed, visiblePPNPayments[0].Status)

	summary, err := taxPaymentService.GetPPNPaymentSummary(time.Time{}, time.Time{})
	require.NoError(t, err)
	ppnKeluaran := summary["ppn_keluaran"].(map[string]interface{})
	require.InDelta(t, 0, ppnKeluaran["total_amount"].(float64), 0.01)
}

func TestDeleteExpensePaymentKeepsAuditTrailAndRestoresCashBank(t *testing.T) {
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
		&models.Payment{},
		&models.PaymentAllocation{},
		&models.Journal{},
		&models.JournalEntry{},
		&models.JournalLine{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
		&models.TaxAccountSettings{},
	))
	require.NoError(t, db.Exec(`ALTER TABLE unified_journal_lines ADD COLUMN deleted_at DATETIME`).Error)

	user := models.User{
		Username:  "finance-expense-delete",
		Email:     "finance-expense-delete@test.local",
		Password:  "secret",
		Role:      "finance",
		FirstName: "Finance Expense",
	}
	require.NoError(t, db.Create(&user).Error)

	contact := models.Contact{
		Code: "EXP-001",
		Name: "Vendor Expense",
		Type: "VENDOR",
	}
	require.NoError(t, db.Create(&contact).Error)

	bankAccount := createTestAccount(t, db, "1101-201", "BANK EXPENSE TEST", models.AccountTypeAsset, 750)
	equityAccount := createTestAccount(t, db, "3101-201", "MODAL EXPENSE", models.AccountTypeEquity, 1000)
	expenseAccount := createTestAccount(t, db, "6105-201", "BEBAN GAJI TEST", models.AccountTypeExpense, 0)

	cashBank := models.CashBank{
		Code:             "BNK-EXP-001",
		Name:             "BANK EXPENSE TEST",
		Type:             models.CashBankTypeBank,
		AccountID:        bankAccount.ID,
		Currency:         "IDR",
		Balance:          750,
		BookBalanceIDR:   750,
		LastExchangeRate: 1,
		IsActive:         true,
		UserID:           user.ID,
	}
	require.NoError(t, db.Create(&cashBank).Error)

	payment := models.Payment{
		Code:             "PAY-EXP-DELETE-0001",
		ContactID:        contact.ID,
		UserID:           user.ID,
		Date:             time.Date(2026, 3, 24, 14, 0, 0, 0, time.UTC),
		Amount:           250,
		NativeAmount:     250,
		Currency:         "IDR",
		ExchangeRate:     1,
		BookAmountIDR:    250,
		Method:           models.PaymentMethodBankTransfer,
		Status:           models.PaymentStatusCompleted,
		PaymentType:      models.PaymentTypeExpense,
		ExpenseAccountID: &expenseAccount.ID,
		CashBankID:       &cashBank.ID,
		Notes:            "Expense payment test",
	}
	require.NoError(t, db.Create(&payment).Error)

	paymentSourceID := uint64(payment.ID)
	baseOpeningJournal := createPostedSSOTJournal(t, db, "OPENING", nil, "OPENING-EXPENSE-BANK", []models.SSOTJournalLine{
		makeSSOTLine(1, uint64(bankAccount.ID), 1000, 0, "Opening bank"),
		makeSSOTLine(2, uint64(equityAccount.ID), 0, 1000, "Opening equity"),
	})
	expenseJournal := createPostedSSOTJournal(t, db, models.SSOTSourceTypePayment, &paymentSourceID, payment.Code, []models.SSOTJournalLine{
		makeSSOTLine(1, uint64(expenseAccount.ID), 250, 0, "Expense payment"),
		makeSSOTLine(2, uint64(bankAccount.ID), 0, 250, "Cash/bank outflow"),
	})
	payment.JournalEntryID = paymentTestUintPtr(uint(expenseJournal.ID))
	require.NoError(t, db.Save(&payment).Error)

	require.NoError(t, db.Create(&models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   "OPENING",
		ReferenceID:     uint(baseOpeningJournal.ID),
		Amount:          1000,
		Currency:        "IDR",
		ExchangeRate:    1,
		BookAmountIDR:   1000,
		BalanceAfter:    1000,
		TransactionDate: payment.Date.Add(-time.Hour),
		Notes:           "Opening balance",
	}).Error)
	require.NoError(t, db.Create(&models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   "PAYMENT",
		ReferenceID:     payment.ID,
		Amount:          -250,
		Currency:        "IDR",
		ExchangeRate:    1,
		BookAmountIDR:   -250,
		BalanceAfter:    750,
		TransactionDate: payment.Date,
		Notes:           "Expense Payment",
	}).Error)

	paymentService := &PaymentService{db: db}
	require.NoError(t, paymentService.DeletePayment(payment.ID, "hapus expense payment untuk audit", user.ID))

	var keptPayment models.Payment
	require.NoError(t, db.First(&keptPayment, payment.ID).Error)
	require.Equal(t, models.PaymentStatusFailed, keptPayment.Status)

	var refreshedCashBank models.CashBank
	require.NoError(t, db.First(&refreshedCashBank, cashBank.ID).Error)
	require.InDelta(t, 1000, refreshedCashBank.Balance, 0.01)
	require.InDelta(t, 1000, refreshedCashBank.BookBalanceIDR, 0.01)

	requireAccountBalance(t, db, expenseAccount.ID, 0)
	requireAccountBalance(t, db, bankAccount.ID, 1000)

	var reversalCount int64
	require.NoError(t, db.Model(&models.CashBankTransaction{}).
		Where("cash_bank_id = ? AND reference_type = ? AND reference_id = ?", cashBank.ID, "PAYMENT_REVERSAL", payment.ID).
		Count(&reversalCount).Error)
	require.Equal(t, int64(1), reversalCount)

	var paymentEntry models.SSOTJournalEntry
	require.NoError(t, db.First(&paymentEntry, expenseJournal.ID).Error)
	require.NotNil(t, paymentEntry.DeletedAt)

	paymentRepo := repositories.NewPaymentRepository(db)
	result, err := paymentRepo.FindWithFilter(repositories.PaymentFilter{Page: 1, Limit: 10})
	require.NoError(t, err)
	require.Len(t, result.Data, 1)
	require.Equal(t, payment.ID, result.Data[0].ID)
	require.Equal(t, models.PaymentStatusFailed, result.Data[0].Status)
}

func TestDeleteAlreadyFailedPaymentPreservesAuditTrail(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Payment{},
	))

	user := models.User{
		Username:  "finance-failed-keep",
		Email:     "finance-failed-keep@test.local",
		Password:  "secret",
		Role:      "finance",
		FirstName: "Finance Failed Keep",
	}
	require.NoError(t, db.Create(&user).Error)

	contact := models.Contact{
		Code: "CNT-FAILED-001",
		Name: "Audit Trail Contact",
		Type: "VENDOR",
	}
	require.NoError(t, db.Create(&contact).Error)

	payment := models.Payment{
		Code:        "PAY-FAILED-KEEP-0001",
		ContactID:   contact.ID,
		UserID:      user.ID,
		Date:        time.Date(2026, 3, 24, 16, 0, 0, 0, time.UTC),
		Amount:      100,
		Method:      models.PaymentMethodBankTransfer,
		Status:      models.PaymentStatusFailed,
		PaymentType: models.PaymentTypeExpense,
	}
	require.NoError(t, db.Create(&payment).Error)

	paymentService := &PaymentService{db: db}
	require.NoError(t, paymentService.DeletePayment(payment.ID, "hapus lagi payment gagal", user.ID))

	var keptPayment models.Payment
	require.NoError(t, db.First(&keptPayment, payment.ID).Error)
	require.Equal(t, models.PaymentStatusFailed, keptPayment.Status)
	require.False(t, keptPayment.DeletedAt.Valid)
}

func createTestAccount(t *testing.T, db *gorm.DB, code, name, accountType string, balance float64) models.Account {
	t.Helper()

	account := models.Account{
		Code:     code,
		Name:     name,
		Type:     accountType,
		IsActive: true,
		IsHeader: false,
		Balance:  balance,
	}
	require.NoError(t, db.Create(&account).Error)
	return account
}

func makeSSOTLine(lineNumber int, accountID uint64, debit, credit float64, description string) models.SSOTJournalLine {
	return models.SSOTJournalLine{
		AccountID:    accountID,
		LineNumber:   lineNumber,
		Description:  description,
		DebitAmount:  decimal.NewFromFloat(debit),
		CreditAmount: decimal.NewFromFloat(credit),
	}
}

func createPostedSSOTJournal(t *testing.T, db *gorm.DB, sourceType string, sourceID *uint64, sourceCode string, lines []models.SSOTJournalLine) models.SSOTJournalEntry {
	t.Helper()

	var totalDebit float64
	var totalCredit float64
	for _, line := range lines {
		totalDebit += line.DebitAmount.InexactFloat64()
		totalCredit += line.CreditAmount.InexactFloat64()
	}

	entry := models.SSOTJournalEntry{
		EntryNumber:     sourceCode + "-JE",
		SourceType:      sourceType,
		SourceID:        sourceID,
		SourceCode:      sourceCode,
		EntryDate:       time.Date(2026, 3, 24, 10, 0, 0, 0, time.UTC),
		Description:     sourceCode,
		Reference:       sourceCode,
		Status:          models.SSOTStatusPosted,
		IsBalanced:      true,
		IsAutoGenerated: true,
		TotalDebit:      decimal.NewFromFloat(totalDebit),
		TotalCredit:     decimal.NewFromFloat(totalCredit),
		CreatedBy:       1,
		PostedBy:        paymentTestUint64Ptr(1),
	}
	require.NoError(t, db.Create(&entry).Error)

	for _, line := range lines {
		line.JournalID = entry.ID
		require.NoError(t, db.Create(&line).Error)
	}

	return entry
}

func requireAccountBalance(t *testing.T, db *gorm.DB, accountID uint, expected float64) {
	t.Helper()

	var account models.Account
	require.NoError(t, db.First(&account, accountID).Error)
	require.InDelta(t, expected, account.Balance, 0.01)
}

func paymentTestUintPtr(value uint) *uint {
	return &value
}

func paymentTestUint64Ptr(value uint64) *uint64 {
	return &value
}
