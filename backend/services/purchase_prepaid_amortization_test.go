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

func TestPurchaseItemsRejectDuplicatePurchaseRequestItem(t *testing.T) {
	purchaseRequestItemID := uint(42)
	items := []models.PurchaseItemRequest{
		{
			ItemType:              models.PurchaseItemTypeExpense,
			Description:           "Biaya pertama",
			PurchaseRequestItemID: &purchaseRequestItemID,
			Quantity:              1,
			UnitPrice:             100000,
		},
		{
			ItemType:              models.PurchaseItemTypeExpense,
			Description:           "Biaya kedua",
			PurchaseRequestItemID: &purchaseRequestItemID,
			Quantity:              1,
			UnitPrice:             100000,
		},
	}

	for _, testCase := range []struct {
		name string
		run  func(*PurchaseService, *models.Purchase, []models.PurchaseItemRequest) error
	}{
		{name: "create", run: func(service *PurchaseService, purchase *models.Purchase, requests []models.PurchaseItemRequest) error {
			return service.calculatePurchaseTotals(purchase, requests)
		}},
		{name: "update", run: func(service *PurchaseService, purchase *models.Purchase, requests []models.PurchaseItemRequest) error {
			return service.updatePurchaseItems(purchase, requests)
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.run(&PurchaseService{}, &models.Purchase{}, items)
			require.ErrorContains(t, err, "purchase request item 42 is selected more than once")
		})
	}
}
