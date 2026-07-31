package repositories

import (
	"strings"
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPurchaseRepositoryUpdateWithItemReplacementSoftDeletesOldItems(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Project{},
		&models.PurchaseRequest{},
		&models.PurchaseRequestItem{},
		&models.Account{},
		&models.Product{},
		&models.Purchase{},
		&models.PurchaseItem{},
		&models.ApprovalWorkflow{},
		&models.ApprovalRequest{},
		&models.ApprovalStep{},
		&models.ApprovalAction{},
	))

	user := models.User{
		Username:  "replace-items-user",
		Email:     "replace-items-user@test.local",
		Password:  "secret",
		Role:      models.RoleFinanceStaff,
		FirstName: "Finance",
	}
	require.NoError(t, db.Create(&user).Error)

	vendor := models.Contact{Code: "VEND-REPLACE", Name: "Vendor Replacement", Type: models.ContactTypeVendor}
	require.NoError(t, db.Create(&vendor).Error)

	purchase := models.Purchase{
		Code:     "PO-REPLACE-ITEMS",
		VendorID: vendor.ID,
		UserID:   user.ID,
		Date:     time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
		Status:   models.PurchaseStatusDraft,
	}
	require.NoError(t, db.Create(&purchase).Error)

	purchaseRequestItemID := uint(9001)
	oldItem := models.PurchaseItem{
		PurchaseID:            purchase.ID,
		PurchaseRequestItemID: &purchaseRequestItemID,
		ItemType:              models.PurchaseItemTypeExpense,
		Description:           "Item sebelum diperbarui",
		Quantity:              1,
		UnitPrice:             100000,
		TotalPrice:            100000,
	}
	require.NoError(t, db.Create(&oldItem).Error)

	purchase.Notes = "Item diperbarui"
	purchase.PurchaseItems = []models.PurchaseItem{
		{
			PurchaseRequestItemID: &purchaseRequestItemID,
			ItemType:              models.PurchaseItemTypeExpense,
			Description:           "Item setelah diperbarui",
			Quantity:              1,
			UnitPrice:             17700000,
			TotalPrice:            17700000,
		},
	}

	repo := NewPurchaseRepository(db)
	updated, err := repo.UpdateWithItemReplacement(&purchase)
	require.NoError(t, err)
	require.Len(t, updated.PurchaseItems, 1)
	require.Equal(t, "Item setelah diperbarui", updated.PurchaseItems[0].Description)

	var activeItems []models.PurchaseItem
	require.NoError(t, db.Where("purchase_id = ?", purchase.ID).Find(&activeItems).Error)
	require.Len(t, activeItems, 1)
	require.Equal(t, "Item setelah diperbarui", activeItems[0].Description)
	require.Equal(t, purchaseRequestItemID, *activeItems[0].PurchaseRequestItemID)

	var allItems []models.PurchaseItem
	require.NoError(t, db.Unscoped().Where("purchase_id = ?", purchase.ID).Order("id ASC").Find(&allItems).Error)
	require.Len(t, allItems, 2)
	require.True(t, allItems[0].DeletedAt.Valid)
	require.False(t, allItems[1].DeletedAt.Valid)
}
