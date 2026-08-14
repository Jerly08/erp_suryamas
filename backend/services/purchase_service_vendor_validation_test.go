package services

import (
	"testing"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreatePurchaseRejectsInactiveVendor(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}
	require.NoError(t, db.AutoMigrate(
		&models.Contact{},
		&models.ContactAddress{},
	))

	inactiveVendor := models.Contact{
		Code:     "VEND-INACTIVE",
		Name:     "Inactive Vendor",
		Type:     models.ContactTypeVendor,
		IsActive: true,
	}
	require.NoError(t, db.Create(&inactiveVendor).Error)
	require.NoError(t, db.Model(&inactiveVendor).Update("is_active", false).Error)

	service := &PurchaseService{
		db:          db,
		contactRepo: repositories.NewContactRepository(db),
	}

	_, err = service.CreatePurchase(models.PurchaseCreateRequest{VendorID: inactiveVendor.ID}, 0)
	require.EqualError(t, err, "selected vendor is inactive and cannot be used for a new purchase")
}
