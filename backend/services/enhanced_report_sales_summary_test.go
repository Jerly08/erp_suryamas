package services

import (
	"testing"

	"app-sistem-akuntansi/models"
	"github.com/stretchr/testify/require"
)

func TestGetSalesSummarySaleAmountsSeparatesShippingRevenueAndTax(t *testing.T) {
	sale := models.Sale{
		ShippingCost:    32500,
		ShippingTaxable: true,
		PPNRate:         11,
		SaleItems: []models.SaleItem{
			{
				Quantity:  1,
				LineTotal: 120000,
				Taxable:   true,
			},
		},
	}

	amounts := getSalesSummarySaleAmounts(&sale)

	require.InDelta(t, 169275, amounts.InvoiceTotal, 0.001)
	require.InDelta(t, 152500, amounts.Revenue, 0.001)
	require.InDelta(t, 32500, amounts.ShippingRevenue, 0.001)
	require.InDelta(t, 152500, amounts.TaxableBase, 0.001)
	require.InDelta(t, 16775, amounts.PPN, 0.001)
	require.Zero(t, amounts.Discount)
}

func TestGetSalesSummarySaleAmountsExcludesNonTaxableShippingFromDPP(t *testing.T) {
	sale := models.Sale{
		ShippingCost:    32500,
		ShippingTaxable: false,
		PPNRate:         11,
		SaleItems: []models.SaleItem{
			{
				Quantity:  1,
				LineTotal: 120000,
				Taxable:   true,
			},
		},
	}

	amounts := getSalesSummarySaleAmounts(&sale)

	require.InDelta(t, 165700, amounts.InvoiceTotal, 0.001)
	require.InDelta(t, 152500, amounts.Revenue, 0.001)
	require.InDelta(t, 120000, amounts.TaxableBase, 0.001)
	require.InDelta(t, 13200, amounts.PPN, 0.001)
}
