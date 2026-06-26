package database

import (
	"app-sistem-akuntansi/models"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBackfillPurchasePaymentLinks(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Purchase{},
		&models.CashBank{},
		&models.CashBankTransaction{},
		&models.Payment{},
		&models.PaymentAllocation{},
		&models.PurchasePayment{},
	))

	user := models.User{
		Username: "purchase-payment-backfill",
		Email:    "purchase-payment-backfill@example.com",
		Password: "hashed",
		Role:     models.RoleFinanceStaff,
		IsActive: true,
	}
	require.NoError(t, db.Create(&user).Error)

	vendor := models.Contact{
		Code:     "VEN-BACKFILL",
		Name:     "Vendor Backfill",
		Type:     models.ContactTypeVendor,
		IsActive: true,
	}
	require.NoError(t, db.Create(&vendor).Error)

	paymentDate := time.Date(2026, 6, 26, 10, 30, 0, 0, time.UTC)
	purchase := models.Purchase{
		Code:              "PO-BACKFILL",
		VendorID:          vendor.ID,
		UserID:            user.ID,
		Date:              paymentDate,
		DueDate:           paymentDate,
		TotalAmount:       600000,
		PaidAmount:        600000,
		OutstandingAmount: 0,
		PaymentMethod:     models.PurchasePaymentCredit,
		Status:            models.PurchaseStatusApproved,
	}
	require.NoError(t, db.Create(&purchase).Error)

	payment := models.Payment{
		Code:      "PAY-BACKFILL",
		ContactID: vendor.ID,
		UserID:    user.ID,
		Date:      paymentDate,
		Amount:    600000,
		Method:    models.PaymentMethodBankTransfer,
		Status:    models.PaymentStatusCompleted,
	}
	require.NoError(t, db.Create(&payment).Error)

	allocation := models.PaymentAllocation{
		PaymentID:       uint64(payment.ID),
		BillID:          &purchase.ID,
		AllocatedAmount: 600000,
	}
	require.NoError(t, db.Create(&allocation).Error)

	BackfillPurchasePaymentLinks(db)
	BackfillPurchasePaymentLinks(db)

	var links []models.PurchasePayment
	require.NoError(t, db.Find(&links).Error)
	require.Len(t, links, 1)
	require.Equal(t, purchase.ID, links[0].PurchaseID)
	require.NotNil(t, links[0].PaymentID)
	require.Equal(t, payment.ID, *links[0].PaymentID)
	require.Equal(t, 600000.0, links[0].Amount)
}

func TestBackfillPurchasePaymentLinksForImmediatePurchase(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Purchase{},
		&models.CashBank{},
		&models.CashBankTransaction{},
		&models.PurchasePayment{},
	))

	user := models.User{
		Username: "immediate-purchase-backfill",
		Email:    "immediate-purchase-backfill@example.com",
		Password: "hashed",
		Role:     models.RoleFinanceStaff,
		IsActive: true,
	}
	require.NoError(t, db.Create(&user).Error)

	vendor := models.Contact{
		Code:     "VEN-IMMEDIATE",
		Name:     "Vendor Immediate",
		Type:     models.ContactTypeVendor,
		IsActive: true,
	}
	require.NoError(t, db.Create(&vendor).Error)

	cashBank := models.CashBank{
		Code:     "BANK-IMMEDIATE",
		Name:     "Bank Immediate",
		Type:     models.CashBankTypeBank,
		Currency: "IDR",
		IsActive: true,
		UserID:   user.ID,
	}
	require.NoError(t, db.Create(&cashBank).Error)

	paymentDate := time.Date(2026, 6, 26, 14, 0, 0, 0, time.UTC)
	purchase := models.Purchase{
		Code:                "PO/2026/06/0002",
		VendorID:            vendor.ID,
		UserID:              user.ID,
		Date:                paymentDate,
		DueDate:             paymentDate,
		TotalAmount:         1200000,
		PaidAmount:          1200000,
		OutstandingAmount:   0,
		PaymentMethod:       models.PurchasePaymentTransfer,
		PaymentReference:    "TRF-IMMEDIATE",
		BankAccountID:       &cashBank.ID,
		Status:              models.PurchaseStatusApproved,
		ApprovalStatus:      models.ApprovalStatusApproved,
		ApprovalAmountBasis: "TOTAL_AMOUNT",
	}
	require.NoError(t, db.Create(&purchase).Error)

	transaction := models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   "PURCHASE",
		ReferenceID:     purchase.ID,
		Amount:          -1200000,
		Currency:        "IDR",
		ExchangeRate:    1,
		BookAmountIDR:   -1200000,
		TransactionDate: paymentDate,
		Notes:           "Payment for purchase PO/2026/06/0002 - BANK_TRANSFER",
	}
	require.NoError(t, db.Create(&transaction).Error)

	BackfillPurchasePaymentLinks(db)
	BackfillPurchasePaymentLinks(db)

	var links []models.PurchasePayment
	require.NoError(t, db.Find(&links).Error)
	require.Len(t, links, 1)
	require.Equal(t, purchase.ID, links[0].PurchaseID)
	require.Nil(t, links[0].PaymentID)
	require.Equal(t, "PAY-PO-2026-06-0002", links[0].PaymentNumber)
	require.Equal(t, 1200000.0, links[0].Amount)
	require.Equal(t, models.PurchasePaymentTransfer, links[0].Method)
	require.Equal(t, "TRF-IMMEDIATE", links[0].Reference)
	require.Equal(t, cashBank.ID, *links[0].CashBankID)
}
