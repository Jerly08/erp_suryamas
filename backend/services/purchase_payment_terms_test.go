package services

import (
	"strings"
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBuildPurchasePaymentTermsDefaultFullTerm(t *testing.T) {
	dueDate := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	purchase := &models.Purchase{
		ID:          7,
		Date:        time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC),
		DueDate:     dueDate,
		TotalAmount: 25000000,
	}

	terms, err := buildPurchasePaymentTerms(purchase, nil)
	if err != nil {
		t.Fatalf("buildPurchasePaymentTerms returned error: %v", err)
	}
	if len(terms) != 1 {
		t.Fatalf("expected 1 default term, got %d", len(terms))
	}
	if terms[0].PurchaseID != purchase.ID {
		t.Fatalf("expected purchase id %d, got %d", purchase.ID, terms[0].PurchaseID)
	}
	if terms[0].Amount != 25000000 {
		t.Fatalf("expected default amount 25000000, got %.2f", terms[0].Amount)
	}
	if !terms[0].DueDate.Equal(dueDate) {
		t.Fatalf("expected due date %s, got %s", dueDate, terms[0].DueDate)
	}
	if terms[0].Status != models.PurchasePaymentTermPending {
		t.Fatalf("expected status %s, got %s", models.PurchasePaymentTermPending, terms[0].Status)
	}
}

func TestBuildPurchasePaymentTermsRejectsUnbalancedTotal(t *testing.T) {
	purchase := &models.Purchase{
		ID:          9,
		Date:        time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC),
		TotalAmount: 25000000,
	}

	_, err := buildPurchasePaymentTerms(purchase, []models.PurchasePaymentTermRequest{
		{Sequence: 1, Amount: 10000000, DueDate: purchase.Date, Description: "Termin 1"},
		{Sequence: 2, Amount: 10000000, DueDate: purchase.Date, Description: "Termin 2"},
	})
	if err == nil {
		t.Fatal("expected unbalanced total error")
	}
	if !strings.Contains(err.Error(), "must equal purchase total") {
		t.Fatalf("expected total mismatch error, got %v", err)
	}
}

func TestBuildPurchasePaymentTermsAcceptsBalancedTerms(t *testing.T) {
	purchase := &models.Purchase{
		ID:          10,
		Date:        time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC),
		TotalAmount: 25000000,
	}

	terms, err := buildPurchasePaymentTerms(purchase, []models.PurchasePaymentTermRequest{
		{Sequence: 1, Amount: 10000000, DueDate: purchase.Date, Description: "DP"},
		{Sequence: 2, Amount: 15000000, DueDate: purchase.Date.AddDate(0, 1, 0), Description: "Pelunasan"},
	})
	if err != nil {
		t.Fatalf("buildPurchasePaymentTerms returned error: %v", err)
	}
	if len(terms) != 2 {
		t.Fatalf("expected 2 terms, got %d", len(terms))
	}
	if terms[0].Amount != 10000000 || terms[1].Amount != 15000000 {
		t.Fatalf("unexpected term amounts: %.2f %.2f", terms[0].Amount, terms[1].Amount)
	}
}

func TestHydratePurchasePaymentTermsWithAllocations(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err := db.AutoMigrate(&models.PaymentAllocation{}); err != nil {
		t.Fatalf("failed to migrate payment allocations: %v", err)
	}

	purchase := &models.Purchase{
		ID: 11,
		PaymentTerms: []models.PurchasePaymentTerm{
			{ID: 1, PurchaseID: 11, Sequence: 1, Amount: 10000000, Status: models.PurchasePaymentTermPending},
			{ID: 2, PurchaseID: 11, Sequence: 2, Amount: 15000000, Status: models.PurchasePaymentTermPending},
		},
	}
	allocations := []models.PaymentAllocation{
		{PaymentID: 1, BillID: purchasePaymentTermUintPtr(11), PurchasePaymentTermID: purchasePaymentTermUintPtr(1), AllocatedAmount: 10000000},
		{PaymentID: 2, BillID: purchasePaymentTermUintPtr(11), PurchasePaymentTermID: purchasePaymentTermUintPtr(2), AllocatedAmount: 5000000},
	}
	if err := db.Create(&allocations).Error; err != nil {
		t.Fatalf("failed to seed allocations: %v", err)
	}

	if err := hydratePurchasePaymentTermsWithAllocations(db, purchase); err != nil {
		t.Fatalf("hydratePurchasePaymentTermsWithAllocations returned error: %v", err)
	}

	if purchase.PaymentTerms[0].PaidAmount != 10000000 || purchase.PaymentTerms[0].OutstandingAmount != 0 {
		t.Fatalf("unexpected first term progress: paid %.2f outstanding %.2f", purchase.PaymentTerms[0].PaidAmount, purchase.PaymentTerms[0].OutstandingAmount)
	}
	if purchase.PaymentTerms[0].Status != models.PurchasePaymentTermPaid {
		t.Fatalf("expected first term paid, got %s", purchase.PaymentTerms[0].Status)
	}
	if purchase.PaymentTerms[1].PaidAmount != 5000000 || purchase.PaymentTerms[1].OutstandingAmount != 10000000 {
		t.Fatalf("unexpected second term progress: paid %.2f outstanding %.2f", purchase.PaymentTerms[1].PaidAmount, purchase.PaymentTerms[1].OutstandingAmount)
	}
	if purchase.PaymentTerms[1].Status != models.PurchasePaymentTermPartial {
		t.Fatalf("expected second term partial, got %s", purchase.PaymentTerms[1].Status)
	}
}

func purchasePaymentTermUintPtr(value uint) *uint {
	return &value
}
