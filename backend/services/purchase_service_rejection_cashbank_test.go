package services

import (
	"testing"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestOnPurchaseRejectedKeepsOriginalAndReversalCashBankHistory(t *testing.T) {
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
		&models.Purchase{},
		&models.PurchaseItem{},
		&models.ApprovalWorkflow{},
		&models.ApprovalStep{},
		&models.ApprovalRequest{},
		&models.ApprovalAction{},
		&models.PurchaseRequest{},
		&models.Project{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
	))
	require.NoError(t, db.Exec(`ALTER TABLE unified_journal_lines ADD COLUMN deleted_at DATETIME`).Error)

	user := models.User{
		Username:  "purchase-finance",
		Email:     "purchase-finance@test.local",
		Password:  "secret",
		Role:      "finance",
		FirstName: "Purchase Finance",
	}
	require.NoError(t, db.Create(&user).Error)

	vendor := models.Contact{
		Code: "VEND-001",
		Name: "CV Supplier",
		Type: "VENDOR",
	}
	require.NoError(t, db.Create(&vendor).Error)

	bankAccount := createTestAccount(t, db, "1101-201", "BANK PURCHASE", models.AccountTypeAsset, 389)
	cashBank := models.CashBank{
		Code:             "BNK-PURCHASE-001",
		Name:             "BANK PURCHASE",
		Type:             models.CashBankTypeBank,
		AccountID:        bankAccount.ID,
		Currency:         "IDR",
		Balance:          389,
		BookBalanceIDR:   389,
		LastExchangeRate: 1,
		IsActive:         true,
		UserID:           user.ID,
	}
	require.NoError(t, db.Create(&cashBank).Error)

	purchase := models.Purchase{
		Code:              "PO-REJECT-0001",
		VendorID:          vendor.ID,
		UserID:            user.ID,
		Date:              time.Date(2026, 3, 24, 13, 0, 0, 0, time.UTC),
		PaymentMethod:     models.PurchasePaymentTransfer,
		BankAccountID:     &cashBank.ID,
		Status:            models.PurchaseStatusApproved,
		ApprovalStatus:    models.PurchaseApprovalRejected,
		NetBeforeTax:      100,
		PPNAmount:         11,
		TotalAmount:       111,
		PaidAmount:        111,
		OutstandingAmount: 0,
	}
	require.NoError(t, db.Create(&purchase).Error)

	require.NoError(t, db.Create(&models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   "OPENING",
		ReferenceID:     1,
		Amount:          500,
		Currency:        "IDR",
		ExchangeRate:    1,
		BookAmountIDR:   500,
		BalanceAfter:    500,
		TransactionDate: purchase.Date.Add(-time.Hour),
		Notes:           "Opening balance",
	}).Error)
	require.NoError(t, db.Create(&models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   "PURCHASE",
		ReferenceID:     purchase.ID,
		Amount:          -111,
		Currency:        "IDR",
		ExchangeRate:    1,
		BookAmountIDR:   -111,
		BalanceAfter:    389,
		TransactionDate: purchase.Date,
		Notes:           "Immediate purchase payment",
	}).Error)

	purchaseRepo := repositories.NewPurchaseRepository(db)
	service := &PurchaseService{
		db:           db,
		purchaseRepo: purchaseRepo,
	}

	require.NoError(t, service.OnPurchaseRejected(purchase.ID))

	var refreshedCashBank models.CashBank
	require.NoError(t, db.First(&refreshedCashBank, cashBank.ID).Error)
	require.InDelta(t, 500, refreshedCashBank.Balance, 0.01)
	require.InDelta(t, 500, refreshedCashBank.BookBalanceIDR, 0.01)

	var originalCount int64
	require.NoError(t, db.Model(&models.CashBankTransaction{}).
		Where("cash_bank_id = ? AND reference_type = ? AND reference_id = ?", cashBank.ID, "PURCHASE", purchase.ID).
		Count(&originalCount).Error)
	require.Equal(t, int64(1), originalCount)

	var reversalCount int64
	require.NoError(t, db.Model(&models.CashBankTransaction{}).
		Where("cash_bank_id = ? AND reference_type = ? AND reference_id = ?", cashBank.ID, "PURCHASE_REVERSAL", purchase.ID).
		Count(&reversalCount).Error)
	require.Equal(t, int64(1), reversalCount)
}
