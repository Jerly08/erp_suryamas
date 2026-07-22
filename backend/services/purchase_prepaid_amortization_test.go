package services

import (
	"testing"

	"app-sistem-akuntansi/models"

	"github.com/stretchr/testify/require"
)

func TestPurchaseItemsPreservePrepaidAmortizationMonth(t *testing.T) {
	itemRequest := models.PurchaseItemRequest{
		ItemType:          models.PurchaseItemTypePrepaid,
		Description:       "Langganan satu bulan",
		AmortizationMonth: 1,
		Quantity:          1,
		UnitPrice:         3499000,
	}

	t.Run("create", func(t *testing.T) {
		service := &PurchaseService{}
		purchase := &models.Purchase{}

		require.NoError(t, service.calculatePurchaseTotals(purchase, []models.PurchaseItemRequest{itemRequest}))
		require.Len(t, purchase.PurchaseItems, 1)
		require.Equal(t, 1, purchase.PurchaseItems[0].AmortizationMonth)
	})

	t.Run("update", func(t *testing.T) {
		service := &PurchaseService{}
		purchase := &models.Purchase{}

		require.NoError(t, service.updatePurchaseItems(purchase, []models.PurchaseItemRequest{itemRequest}))
		require.Len(t, purchase.PurchaseItems, 1)
		require.Equal(t, 1, purchase.PurchaseItems[0].AmortizationMonth)
	})
}

func TestPurchaseItemsRejectPrepaidWithoutAmortizationMonth(t *testing.T) {
	itemRequest := models.PurchaseItemRequest{
		ItemType:    models.PurchaseItemTypePrepaid,
		Description: "Durasi belum ditentukan",
		Quantity:    1,
		UnitPrice:   100000,
	}

	t.Run("create", func(t *testing.T) {
		service := &PurchaseService{}
		purchase := &models.Purchase{}

		err := service.calculatePurchaseTotals(purchase, []models.PurchaseItemRequest{itemRequest})
		require.ErrorContains(t, err, "durasi amortisasi prepaid harus antara 1 dan 120 bulan")
	})

	t.Run("update", func(t *testing.T) {
		service := &PurchaseService{}
		purchase := &models.Purchase{}

		err := service.updatePurchaseItems(purchase, []models.PurchaseItemRequest{itemRequest})
		require.ErrorContains(t, err, "durasi amortisasi prepaid harus antara 1 dan 120 bulan")
	})
}
