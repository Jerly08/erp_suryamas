package services

import (
	"testing"

	"app-sistem-akuntansi/models"
)

func TestCalculateSalesAmountsIncludesTaxableShippingInDPP(t *testing.T) {
	sale := &models.Sale{
		PPNRate:         11,
		ShippingCost:    32500,
		ShippingTaxable: true,
	}
	items := []models.SaleItem{
		{LineTotal: 110000, Taxable: true},
		{LineTotal: 10000, Taxable: true},
	}

	result := calculateSalesAmounts(sale, items)
	if result.TaxableAmount != 152500 {
		t.Fatalf("expected DPP 152500, got %.2f", result.TaxableAmount)
	}
	if result.PPNAmount != 16775 {
		t.Fatalf("expected PPN 16775, got %.2f", result.PPNAmount)
	}
	if result.TotalAmount != 169275 {
		t.Fatalf("expected total 169275, got %.2f", result.TotalAmount)
	}
}

func TestCalculateSalesAmountsExcludesNonTaxableShippingFromDPP(t *testing.T) {
	sale := &models.Sale{PPNRate: 11, ShippingCost: 32500}
	items := []models.SaleItem{{LineTotal: 120000, Taxable: true}}

	result := calculateSalesAmounts(sale, items)
	if result.TaxableAmount != 120000 || result.PPNAmount != 13200 || result.TotalAmount != 165700 {
		t.Fatalf("unexpected non-taxable shipping result: %+v", result)
	}
}

func TestCalculateSalesAmountsAllocatesDiscountAndKeepsShippingOutOfPPhBase(t *testing.T) {
	sale := &models.Sale{
		DiscountPercent: 10,
		PPNRate:         11,
		PPh23Rate:       2,
		ShippingCost:    10000,
		ShippingTaxable: true,
	}
	items := []models.SaleItem{
		{LineTotal: 100000, Taxable: true},
		{LineTotal: 50000, Taxable: false},
	}

	result := calculateSalesAmounts(sale, items)
	if result.DiscountAmount != 15000 || result.NetBeforeTax != 135000 {
		t.Fatalf("unexpected discount allocation: %+v", result)
	}
	if result.TaxableAmount != 100000 || result.PPNAmount != 11000 {
		t.Fatalf("unexpected PPN base: %+v", result)
	}
	if result.PPh23Amount != 1800 {
		t.Fatalf("expected PPh23 on taxable goods only (1800), got %.2f", result.PPh23Amount)
	}
	if result.TotalAmount != 154200 {
		t.Fatalf("expected total 154200, got %.2f", result.TotalAmount)
	}
}

func TestSalePPhAndNetTotalHelpersDoNotDoubleCountTaxableShipping(t *testing.T) {
	sale := &models.Sale{
		NetBeforeTax:    120000,
		TaxableAmount:   152500,
		PPN:             16775,
		ShippingCost:    32500,
		ShippingTaxable: true,
	}

	if base := calculateSalePPhTaxBase(sale); base != 120000 {
		t.Fatalf("expected PPh base 120000, got %.2f", base)
	}
	if total := calculateSaleNetTotalWithCurrentTaxes(sale); total != 169275 {
		t.Fatalf("expected net invoice total 169275, got %.2f", total)
	}
}
