package services

import (
	"testing"

	"app-sistem-akuntansi/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBuildSaleItemFulfillmentSnapshotWithRemainingStockTracksCumulativeDemand(t *testing.T) {
	product := &models.Product{
		ID:    1,
		Code:  "PRD-001",
		Name:  "Produk A",
		Unit:  "pcs",
		Stock: 10,
	}
	remainingProductStock := make(map[uint]int)

	firstSnapshot := buildSaleItemFulfillmentSnapshotWithRemainingStock(product, 8, remainingProductStock)
	require.Equal(t, 8, firstSnapshot.availableQuantity)
	require.Equal(t, 0, firstSnapshot.backorderQuantity)
	require.Equal(t, models.SaleFulfillmentStatusInStock, firstSnapshot.fulfillmentStatus)
	require.Equal(t, 2, remainingProductStock[product.ID])

	secondSnapshot := buildSaleItemFulfillmentSnapshotWithRemainingStock(product, 5, remainingProductStock)
	require.Equal(t, 2, secondSnapshot.availableQuantity)
	require.Equal(t, 3, secondSnapshot.backorderQuantity)
	require.Equal(t, models.SaleFulfillmentStatusPartialBackorder, secondSnapshot.fulfillmentStatus)
	require.Equal(t, 0, remainingProductStock[product.ID])
}

func TestValidateStockForCreateUsesCumulativeStockPerProduct(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	require.NoError(t, db.AutoMigrate(&models.Product{}))
	require.NoError(t, db.Create(&models.Product{
		Code:     "PRD-001",
		Name:     "Produk A",
		Unit:     "pcs",
		Stock:    10,
		IsActive: true,
	}).Error)

	service := &SalesServiceV2{db: db}

	result, err := service.ValidateStockForCreate(models.StockValidationRequest{
		Items: []models.SaleItemRequest{
			{ProductID: 1, Quantity: 8},
			{ProductID: 1, Quantity: 5},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Items, 2)

	require.True(t, result.Items[0].IsSufficient)
	require.Equal(t, 10, result.Items[0].AvailableQty)

	require.False(t, result.Items[1].IsSufficient)
	require.Equal(t, 2, result.Items[1].AvailableQty)
	require.True(t, result.HasInsufficient)
}
