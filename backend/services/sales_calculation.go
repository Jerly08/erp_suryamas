package services

import (
	"app-sistem-akuntansi/models"

	"github.com/shopspring/decimal"
)

// salesAmountResult is the single source of truth for persisted sales totals.
// TaxableAmount is the PPN tax base, while NetBeforeTax is the goods/services
// amount after the sale-level discount and before shipping or taxes.
type salesAmountResult struct {
	Subtotal        float64
	DiscountAmount  float64
	NetBeforeTax    float64
	TaxableAmount   float64
	PPNAmount       float64
	PPhAmount       float64
	PPh21Amount     float64
	PPh23Amount     float64
	TotalDeductions float64
	TotalTax        float64
	TotalAmount     float64
}

func salesMoney(value decimal.Decimal) decimal.Decimal {
	return value.Round(2)
}

func calculateSalesAmounts(sale *models.Sale, items []models.SaleItem) salesAmountResult {
	if sale == nil {
		return salesAmountResult{}
	}

	taxableSubtotal := decimal.Zero
	nonTaxableSubtotal := decimal.Zero
	for _, item := range items {
		amount := decimal.NewFromFloat(item.LineTotal)
		if item.Taxable {
			taxableSubtotal = taxableSubtotal.Add(amount)
		} else {
			nonTaxableSubtotal = nonTaxableSubtotal.Add(amount)
		}
	}

	subtotal := salesMoney(taxableSubtotal.Add(nonTaxableSubtotal))
	discountRate := decimal.NewFromFloat(sale.DiscountPercent).Div(decimal.NewFromInt(100))
	if discountRate.IsNegative() {
		discountRate = decimal.Zero
	}
	if discountRate.GreaterThan(decimal.NewFromInt(1)) {
		discountRate = decimal.NewFromInt(1)
	}

	discountAmount := salesMoney(subtotal.Mul(discountRate))
	taxableGoods := salesMoney(taxableSubtotal.Mul(decimal.NewFromInt(1).Sub(discountRate)))
	netBeforeTax := salesMoney(subtotal.Sub(discountAmount))

	shipping := salesMoney(decimal.NewFromFloat(sale.ShippingCost))
	taxableAmount := taxableGoods
	if sale.ShippingTaxable {
		taxableAmount = salesMoney(taxableAmount.Add(shipping))
	}

	ppnRate := sale.PPNRate
	if ppnRate == 0 {
		ppnRate = sale.PPNPercent
	}
	ppnAmount := salesMoney(taxableAmount.Mul(decimal.NewFromFloat(ppnRate).Div(decimal.NewFromInt(100))))

	// Withholding tax remains based on the taxable goods/services. Reimbursed
	// shipping is part of DPP PPN, but is not automatically treated as a PPh base.
	pphAmount := salesMoney(taxableGoods.Mul(decimal.NewFromFloat(sale.PPhPercent).Div(decimal.NewFromInt(100))))
	pph21Amount := salesMoney(taxableGoods.Mul(decimal.NewFromFloat(sale.PPh21Rate).Div(decimal.NewFromInt(100))))
	pph23Amount := salesMoney(taxableGoods.Mul(decimal.NewFromFloat(sale.PPh23Rate).Div(decimal.NewFromInt(100))))
	otherDeductions := salesMoney(decimal.NewFromFloat(sale.OtherTaxDeductions))
	totalDeductions := salesMoney(pphAmount.Add(pph21Amount).Add(pph23Amount).Add(otherDeductions))
	otherAdditions := salesMoney(decimal.NewFromFloat(sale.OtherTaxAdditions))
	totalTax := salesMoney(ppnAmount.Add(otherAdditions).Sub(totalDeductions))
	totalAmount := salesMoney(netBeforeTax.Add(shipping).Add(ppnAmount).Add(otherAdditions).Sub(totalDeductions))

	return salesAmountResult{
		Subtotal:        subtotal.InexactFloat64(),
		DiscountAmount:  discountAmount.InexactFloat64(),
		NetBeforeTax:    netBeforeTax.InexactFloat64(),
		TaxableAmount:   taxableAmount.InexactFloat64(),
		PPNAmount:       ppnAmount.InexactFloat64(),
		PPhAmount:       pphAmount.InexactFloat64(),
		PPh21Amount:     pph21Amount.InexactFloat64(),
		PPh23Amount:     pph23Amount.InexactFloat64(),
		TotalDeductions: totalDeductions.InexactFloat64(),
		TotalTax:        totalTax.InexactFloat64(),
		TotalAmount:     totalAmount.InexactFloat64(),
	}
}

func applyCalculatedSalesAmounts(sale *models.Sale, items []models.SaleItem) {
	result := calculateSalesAmounts(sale, items)
	sale.Subtotal = result.Subtotal
	sale.SubTotal = result.Subtotal
	sale.DiscountAmount = result.DiscountAmount
	sale.NetBeforeTax = result.NetBeforeTax
	sale.TaxableAmount = result.TaxableAmount
	sale.PPN = result.PPNAmount
	sale.PPNAmount = result.PPNAmount
	sale.PPh = result.PPhAmount
	sale.PPh21Amount = result.PPh21Amount
	sale.PPh23Amount = result.PPh23Amount
	sale.TotalTaxAdditions = sale.OtherTaxAdditions
	sale.TotalTaxDeductions = result.TotalDeductions
	sale.TotalTax = result.TotalTax
	sale.Tax = result.TotalTax
	sale.TotalAmount = result.TotalAmount
	sale.OutstandingAmount = salesMoney(decimal.NewFromFloat(result.TotalAmount).Sub(decimal.NewFromFloat(sale.PaidAmount))).InexactFloat64()
}
