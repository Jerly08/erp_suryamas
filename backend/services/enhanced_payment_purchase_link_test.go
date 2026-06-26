package services

import (
	"app-sistem-akuntansi/models"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreatePaymentAllocationsCreatesPurchasePaymentLink(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Purchase{},
		&models.Payment{},
		&models.PaymentAllocation{},
		&models.PurchasePayment{},
	))

	user := models.User{
		Username: "payable-link-user",
		Email:    "payable-link-user@example.com",
		Password: "hashed",
		Role:     models.RoleFinanceManager,
		IsActive: true,
	}
	require.NoError(t, db.Create(&user).Error)

	vendor := models.Contact{
		Code:     "VEN-PAY-LINK",
		Name:     "Vendor Payment Link",
		Type:     models.ContactTypeVendor,
		IsActive: true,
	}
	require.NoError(t, db.Create(&vendor).Error)

	paymentDate := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	purchase := models.Purchase{
		Code:              "PO-PAY-LINK",
		VendorID:          vendor.ID,
		UserID:            user.ID,
		Date:              paymentDate,
		DueDate:           paymentDate,
		TotalAmount:       1200000,
		OutstandingAmount: 1200000,
		PaymentMethod:     models.PurchasePaymentCredit,
		Status:            models.PurchaseStatusApproved,
	}
	require.NoError(t, db.Create(&purchase).Error)

	payment := models.Payment{
		Code:      "PAY-PAY-LINK",
		ContactID: vendor.ID,
		UserID:    user.ID,
		Date:      paymentDate,
		Amount:    1200000,
		Method:    models.PaymentMethodBankTransfer,
		Status:    models.PaymentStatusCompleted,
	}
	require.NoError(t, db.Create(&payment).Error)

	service := &EnhancedPaymentServiceWithJournal{}
	_, err = service.createPaymentAllocations(db, &payment, &PaymentWithJournalRequest{
		ContactID: vendor.ID,
		Amount:    payment.Amount,
		BillAllocations: []AllocationItem{
			{BillID: &purchase.ID, Amount: payment.Amount},
		},
	}, models.ContactTypeVendor, true)
	require.NoError(t, err)

	var link models.PurchasePayment
	require.NoError(t, db.Preload("Payment").Where("purchase_id = ?", purchase.ID).First(&link).Error)
	require.Equal(t, payment.ID, *link.PaymentID)
	require.Equal(t, payment.Code, link.PaymentNumber)
	require.Equal(t, payment.Amount, link.Amount)
	require.Equal(t, models.PaymentStatusCompleted, link.Payment.Status)

	var updatedPurchase models.Purchase
	require.NoError(t, db.First(&updatedPurchase, purchase.ID).Error)
	require.Equal(t, payment.Amount, updatedPurchase.PaidAmount)
	require.Equal(t, 0.0, updatedPurchase.OutstandingAmount)
}
