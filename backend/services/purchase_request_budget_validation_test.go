package services

import (
	"app-sistem-akuntansi/models"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatePurchaseRequestBudgetUsesNetBeforeTax(t *testing.T) {
	service := &PurchaseService{}

	t.Run("allows item discount below approved budget even when gross exceeds it", func(t *testing.T) {
		purchase := &models.Purchase{PPNRate: 11}
		err := service.calculatePurchaseTotals(purchase, []models.PurchaseItemRequest{
			{
				ItemType:    models.PurchaseItemTypeExpense,
				Description: "Pengajuan Bindex Untuk Arsip File 7 pcs",
				Quantity:    6,
				UnitPrice:   32100,
				Discount:    27927,
			},
		})

		require.NoError(t, err)
		require.InDelta(t, 192600, purchase.SubtotalBeforeDiscount, 0.001)
		require.InDelta(t, 27927, purchase.ItemDiscountAmount, 0.001)
		require.InDelta(t, 164673, purchase.NetBeforeTax, 0.001)
		require.Greater(t, purchase.TotalAmount, 175840.0, "tax is intentionally excluded from the PR budget check")
		require.NoError(t, validatePurchaseRequestBudget(purchase, 175840))
	})

	t.Run("uses the order discount percentage", func(t *testing.T) {
		purchase := &models.Purchase{Discount: 10}
		err := service.calculatePurchaseTotals(purchase, []models.PurchaseItemRequest{
			{
				ItemType:    models.PurchaseItemTypeExpense,
				Description: "Jasa operasional",
				Quantity:    1,
				UnitPrice:   200000,
			},
		})

		require.NoError(t, err)
		require.InDelta(t, 20000, purchase.OrderDiscountAmount, 0.001)
		require.InDelta(t, 180000, purchase.NetBeforeTax, 0.001)
		require.NoError(t, validatePurchaseRequestBudget(purchase, 180000))
	})

	t.Run("blocks net amounts above the approved budget", func(t *testing.T) {
		purchase := &models.Purchase{NetBeforeTax: 175840.01}
		err := validatePurchaseRequestBudget(purchase, 175840)

		require.ErrorContains(t, err, "melebihi pagu PR yang disetujui")
	})
}
