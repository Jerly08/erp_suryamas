package services

import (
	"strings"
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEnsurePurchasePaymentLinkAssociatesUniquePaymentTerm(t *testing.T) {
	db := newPurchasePaymentLinkTestDB(t)
	purchaseID := uint(91)
	termID := uint(17)
	payment := createPurchasePaymentLinkTestPayment(t, db, "PAY-LINK-TERM")

	allocation := models.PaymentAllocation{
		PaymentID:             uint64(payment.ID),
		BillID:                purchasePaymentLinkUintPtr(purchaseID),
		PurchasePaymentTermID: purchasePaymentLinkUintPtr(termID),
		AllocatedAmount:       12000000,
	}
	if err := db.Create(&allocation).Error; err != nil {
		t.Fatalf("failed to create payment allocation: %v", err)
	}

	var link *models.PurchasePayment
	if err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		link, err = ensurePurchasePaymentLink(tx, payment, purchaseID, allocation.AllocatedAmount)
		return err
	}); err != nil {
		t.Fatalf("ensurePurchasePaymentLink returned error: %v", err)
	}

	if link == nil {
		t.Fatal("expected a purchase payment link")
	}
	if link.PurchasePaymentTermID == nil || *link.PurchasePaymentTermID != termID {
		t.Fatalf("expected link to term %d, got %#v", termID, link.PurchasePaymentTermID)
	}
	if link.PaymentID == nil || *link.PaymentID != payment.ID {
		t.Fatalf("expected link to payment %d, got %#v", payment.ID, link.PaymentID)
	}
	if link.Amount != allocation.AllocatedAmount {
		t.Fatalf("expected link amount %.2f, got %.2f", allocation.AllocatedAmount, link.Amount)
	}

	var storedAllocation models.PaymentAllocation
	if err := db.First(&storedAllocation, allocation.ID).Error; err != nil {
		t.Fatalf("failed to reload allocation: %v", err)
	}
	if storedAllocation.PurchasePaymentTermID == nil || *storedAllocation.PurchasePaymentTermID != termID {
		t.Fatalf("linking unexpectedly changed the allocation term: %#v", storedAllocation.PurchasePaymentTermID)
	}
}

func TestEnsurePurchasePaymentLinkDoesNotGuessAcrossMultipleTerms(t *testing.T) {
	db := newPurchasePaymentLinkTestDB(t)
	purchaseID := uint(92)
	payment := createPurchasePaymentLinkTestPayment(t, db, "PAY-LINK-MULTI")
	allocations := []models.PaymentAllocation{
		{
			PaymentID:             uint64(payment.ID),
			BillID:                purchasePaymentLinkUintPtr(purchaseID),
			PurchasePaymentTermID: purchasePaymentLinkUintPtr(21),
			AllocatedAmount:       5000000,
		},
		{
			PaymentID:             uint64(payment.ID),
			BillID:                purchasePaymentLinkUintPtr(purchaseID),
			PurchasePaymentTermID: purchasePaymentLinkUintPtr(22),
			AllocatedAmount:       7000000,
		},
	}
	if err := db.Create(&allocations).Error; err != nil {
		t.Fatalf("failed to create payment allocations: %v", err)
	}

	var link *models.PurchasePayment
	if err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		link, err = ensurePurchasePaymentLink(tx, payment, purchaseID, 12000000)
		return err
	}); err != nil {
		t.Fatalf("ensurePurchasePaymentLink returned error: %v", err)
	}

	if link == nil {
		t.Fatal("expected a purchase payment link")
	}
	if link.PurchasePaymentTermID != nil {
		t.Fatalf("expected no term link for multi-term allocation, got %d", *link.PurchasePaymentTermID)
	}
	if link.Amount != 12000000 {
		t.Fatalf("expected total allocation amount 12000000, got %.2f", link.Amount)
	}
}

func newPurchasePaymentLinkTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"),
		&gorm.Config{DisableForeignKeyConstraintWhenMigrating: true},
	)
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Payment{},
		&models.PaymentAllocation{},
		&models.PurchasePayment{},
	); err != nil {
		t.Fatalf("failed to migrate test models: %v", err)
	}
	return db
}

func createPurchasePaymentLinkTestPayment(t *testing.T, db *gorm.DB, code string) *models.Payment {
	t.Helper()
	payment := &models.Payment{
		Code:      code,
		ContactID: 1,
		UserID:    1,
		Date:      time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC),
		Amount:    12000000,
		Method:    models.PaymentMethodBankTransfer,
		Status:    models.PaymentStatusCompleted,
	}
	if err := db.Create(payment).Error; err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}
	return payment
}

func purchasePaymentLinkUintPtr(value uint) *uint {
	return &value
}
