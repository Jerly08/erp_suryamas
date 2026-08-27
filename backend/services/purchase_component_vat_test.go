package services

import (
	"math"
	"testing"
	"time"

	"app-sistem-akuntansi/models"
)

func purchaseComponentVATFloat64Pointer(value float64) *float64 {
	return &value
}

func purchaseComponentVATBoolPointer(value bool) *bool {
	return &value
}

func purchaseComponentVATIntPointer(value int) *int {
	return &value
}

// PLN-style invoices often show the commercial charge and surcharge together,
// while the tax invoice uses only the surcharge as DPP Nilai Lain. This test
// ensures the server derives the source amount from that one item and retains
// the invoice-authoritative DPP/PPN figures.
func TestBuildPurchaseTaxAllocationsForComponentDPPNilaiLain(t *testing.T) {
	fakturDate := time.Date(2026, time.May, 25, 0, 0, 0, 0, time.UTC)
	purchase := &models.Purchase{
		Discount: 0,
		PurchaseItems: []models.PurchaseItem{
			{ID: 11, Quantity: 1, UnitPrice: 607837, TotalPrice: 607837, Description: "Biaya listrik"},
			{ID: 12, Quantity: 1, UnitPrice: 18235, TotalPrice: 18235, Description: "BPJU"},
			{ID: 13, Quantity: 1, UnitPrice: 31304, TotalPrice: 31304, Description: "Surcharge"},
		},
	}

	allocations, err := buildPurchaseTaxAllocations(purchase, []models.PurchaseTaxAllocationRequest{{
		PurchaseItemIndex: purchaseComponentVATIntPointer(2),
		DPPAmount:         purchaseComponentVATFloat64Pointer(28695),
		PPNRate:           purchaseComponentVATFloat64Pointer(12),
		PPNAmount:         purchaseComponentVATFloat64Pointer(3443),
		IsCreditable:      purchaseComponentVATBoolPointer(true),
		TaxInvoiceNumber:  "010.001-26.00000001",
		TaxInvoiceDate:    &fakturDate,
	}})
	if err != nil {
		t.Fatalf("build component VAT allocation: %v", err)
	}
	if len(allocations) != 1 {
		t.Fatalf("expected one allocation, got %d", len(allocations))
	}

	allocation := allocations[0]
	if allocation.PurchaseItemID != 13 {
		t.Fatalf("expected surcharge item ID 13, got %d", allocation.PurchaseItemID)
	}
	if allocation.SourceAmount != 31304 || allocation.DPPAmount != 28695 || allocation.PPNAmount != 3443 {
		t.Fatalf("unexpected allocation values: %+v", allocation)
	}

	purchase.VATTreatment = models.PurchaseVATTreatmentComponentDPPNilaiLain
	purchase.TaxAllocations = allocations
	if err := applyPurchaseVATTreatment(purchase); err != nil {
		t.Fatalf("apply component VAT treatment: %v", err)
	}
	if purchase.PPNRate != 0 || purchase.PPNAmount != 3443 {
		t.Fatalf("expected header to retain only invoice PPN (rate=0 amount=3443), got rate=%.2f amount=%.2f", purchase.PPNRate, purchase.PPNAmount)
	}
}

func TestPurchaseComponentVATNonCreditableIsAssignedToSourceItem(t *testing.T) {
	fakturDate := time.Date(2026, time.May, 25, 0, 0, 0, 0, time.UTC)
	notCreditable := false
	purchase := &models.Purchase{
		VATTreatment: models.PurchaseVATTreatmentComponentDPPNilaiLain,
		TaxAllocations: []models.PurchaseTaxAllocation{{
			PurchaseItemID:   99,
			DPPAmount:        28695,
			PPNRate:          12,
			PPNAmount:        3443,
			IsCreditable:     notCreditable,
			TaxInvoiceNumber: "010.001-26.00000002",
			TaxInvoiceDate:   &fakturDate,
		}},
		PurchaseItems: []models.PurchaseItem{{
			ID:         99,
			Quantity:   1,
			UnitPrice:  31304,
			TotalPrice: 31304,
		}},
	}

	if err := applyPurchaseVATTreatment(purchase); err != nil {
		t.Fatalf("apply component VAT treatment: %v", err)
	}
	byItem := purchaseNonCreditableVATByItem(purchase)
	if math.Abs(byItem[99]-3443) > 0.001 {
		t.Fatalf("expected non-creditable VAT 3443 on item 99, got %.2f", byItem[99])
	}
	if got := purchaseCreditableVATAmount(purchase); got != 0 {
		t.Fatalf("expected no creditable VAT, got %.2f", got)
	}
}

func TestApplyPurchaseVATTreatmentRejectsOverallocatedDPP(t *testing.T) {
	fakturDate := time.Date(2026, time.May, 25, 0, 0, 0, 0, time.UTC)
	purchase := &models.Purchase{
		VATTreatment: models.PurchaseVATTreatmentComponentDPPNilaiLain,
		PurchaseItems: []models.PurchaseItem{{
			ID:         7,
			Quantity:   1,
			UnitPrice:  31304,
			TotalPrice: 31304,
		}},
		TaxAllocations: []models.PurchaseTaxAllocation{
			{PurchaseItemID: 7, DPPAmount: 20000, PPNRate: 12, PPNAmount: 2400, TaxInvoiceNumber: "A", TaxInvoiceDate: &fakturDate},
			{PurchaseItemID: 7, DPPAmount: 20000, PPNRate: 12, PPNAmount: 2400, TaxInvoiceNumber: "B", TaxInvoiceDate: &fakturDate},
		},
	}

	if err := applyPurchaseVATTreatment(purchase); err == nil {
		t.Fatal("expected component VAT treatment to reject DPP above its source item")
	}
}
