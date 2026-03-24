package services

import (
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetPPNBalanceUsesFreshSettingsAndSSOTBalance(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Account{},
		&models.TaxAccountSettings{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
	))

	user := models.User{
		Username:  "finance-tax",
		Email:     "finance-tax@test.local",
		Password:  "secret",
		Role:      "finance",
		FirstName: "Finance Tax",
	}
	require.NoError(t, db.Create(&user).Error)

	oldInputVAT := createTestAccount(t, db, "1240", "PPN MASUKAN LAMA", models.AccountTypeAsset, 0)
	currentInputVAT := createTestAccount(t, db, "2102-002", "PPN MASUKAN", models.AccountTypeLiability, 0)
	outputVAT := createTestAccount(t, db, "2102-001", "PPN KELUARAN", models.AccountTypeLiability, 0)

	oldSettings := models.TaxAccountSettings{
		SalesOutputVATAccountID:   outputVAT.ID,
		PurchaseInputVATAccountID: oldInputVAT.ID,
		IsActive:                  true,
		ApplyToAllCompanies:       true,
		UpdatedBy:                 user.ID,
	}
	require.NoError(t, db.Create(&oldSettings).Error)

	service := NewTaxPaymentService(db)

	require.NoError(t, db.Model(&models.TaxAccountSettings{}).
		Where("id = ?", oldSettings.ID).
		Update("is_active", false).Error)

	newSettings := models.TaxAccountSettings{
		SalesOutputVATAccountID:   outputVAT.ID,
		PurchaseInputVATAccountID: currentInputVAT.ID,
		IsActive:                  true,
		ApplyToAllCompanies:       true,
		UpdatedBy:                 user.ID,
	}
	require.NoError(t, db.Create(&newSettings).Error)

	createPostedSSOTJournal(t, db, models.SSOTSourceTypePurchase, nil, "PO-PPN-BALANCE", []models.SSOTJournalLine{
		makeSSOTLine(1, uint64(currentInputVAT.ID), 11000, 0, "PPN Masukan"),
		makeSSOTLine(2, uint64(outputVAT.ID), 0, 0, "Counter"),
	})

	var refreshedInputVAT models.Account
	require.NoError(t, db.First(&refreshedInputVAT, currentInputVAT.ID).Error)
	require.InDelta(t, 0, refreshedInputVAT.Balance, 0.01)

	balance, err := service.GetPPNBalance("INPUT")
	require.NoError(t, err)
	require.InDelta(t, 11000, balance, 0.01)
}

func TestCreatePPNPaymentUsesSSOTBalancesWhenAccountBalanceStale(t *testing.T) {
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
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
		&models.TaxAccountSettings{},
		&models.AccountingPeriod{},
	))

	user := models.User{
		Username:  "finance-create-tax",
		Email:     "finance-create-tax@test.local",
		Password:  "secret",
		Role:      "finance",
		FirstName: "Finance Create Tax",
	}
	require.NoError(t, db.Create(&user).Error)

	require.NoError(t, db.Create(&models.Contact{
		ID:   1,
		Code: "TAX-001",
		Name: "Negara",
		Type: "VENDOR",
	}).Error)

	bankAccount := createTestAccount(t, db, "1101-001", "BANK TEST", models.AccountTypeAsset, 1000)
	ppnOutputAccount := createTestAccount(t, db, "2102-001", "PPN KELUARAN", models.AccountTypeLiability, 0)
	ppnInputAccount := createTestAccount(t, db, "2102-002", "PPN MASUKAN", models.AccountTypeLiability, 0)

	require.NoError(t, db.Create(&models.TaxAccountSettings{
		SalesOutputVATAccountID:   ppnOutputAccount.ID,
		PurchaseInputVATAccountID: ppnInputAccount.ID,
		SalesBankAccountID:        bankAccount.ID,
		PurchaseBankAccountID:     bankAccount.ID,
		IsActive:                  true,
		ApplyToAllCompanies:       true,
		UpdatedBy:                 user.ID,
	}).Error)

	cashBank := models.CashBank{
		Code:             "BNK-TEST-PPN",
		Name:             "BANK TEST",
		Type:             models.CashBankTypeBank,
		AccountID:        bankAccount.ID,
		Currency:         "IDR",
		Balance:          1000,
		BookBalanceIDR:   1000,
		LastExchangeRate: 1,
		IsActive:         true,
		UserID:           user.ID,
	}
	require.NoError(t, db.Create(&cashBank).Error)

	createPostedSSOTJournal(t, db, models.SSOTSourceTypeSale, nil, "SALE-PPN", []models.SSOTJournalLine{
		makeSSOTLine(1, uint64(ppnOutputAccount.ID), 0, 110, "PPN Keluaran"),
		makeSSOTLine(2, uint64(bankAccount.ID), 110, 0, "Counter"),
	})
	createPostedSSOTJournal(t, db, models.SSOTSourceTypePurchase, nil, "PURCHASE-PPN", []models.SSOTJournalLine{
		makeSSOTLine(1, uint64(ppnInputAccount.ID), 50, 0, "PPN Masukan"),
		makeSSOTLine(2, uint64(bankAccount.ID), 0, 50, "Counter"),
	})

	service := NewTaxPaymentService(db)
	payment, err := service.CreatePPNPayment(CreatePPNPaymentRequest{
		PPNType:    "INPUT",
		Date:       time.Date(2026, 3, 24, 11, 0, 0, 0, time.UTC),
		CashBankID: cashBank.ID,
		Notes:      "test setor ppn",
	}, user.ID)
	require.NoError(t, err)
	require.NotNil(t, payment)
	require.InDelta(t, 60, payment.Amount, 0.01)

	var refreshedCashBank models.CashBank
	require.NoError(t, db.First(&refreshedCashBank, cashBank.ID).Error)
	require.InDelta(t, 940, refreshedCashBank.Balance, 0.01)
}
