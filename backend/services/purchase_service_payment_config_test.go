package services

import (
	"testing"

	"app-sistem-akuntansi/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNormalizePurchasePaymentConfigurationUsesDefaultCreditAccount(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	require.NoError(t, db.AutoMigrate(&models.Account{}))
	require.NoError(t, db.Create(&models.Account{
		ID:       1,
		Code:     "2101-001",
		Name:     "UTANG USAHA - LOKAL",
		Type:     models.AccountTypeLiability,
		IsActive: true,
		IsHeader: false,
	}).Error)

	service := &PurchaseService{db: db}

	bankAccountID, creditAccountID, err := service.normalizePurchasePaymentConfiguration(
		models.PurchasePaymentCredit,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.Nil(t, bankAccountID)
	require.NotNil(t, creditAccountID)
	require.EqualValues(t, 1, *creditAccountID)
}

func TestNormalizePurchasePaymentConfigurationMapsHeaderCreditAccountToDetail(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	require.NoError(t, db.AutoMigrate(&models.Account{}))
	require.NoError(t, db.Create(&models.Account{
		ID:       10,
		Code:     "2101",
		Name:     "UTANG USAHA",
		Type:     models.AccountTypeLiability,
		IsActive: true,
		IsHeader: true,
	}).Error)
	require.NoError(t, db.Create(&models.Account{
		ID:       11,
		Code:     "2101-001",
		Name:     "UTANG USAHA - LOKAL",
		Type:     models.AccountTypeLiability,
		ParentID: uintPtr(10),
		IsActive: true,
		IsHeader: false,
	}).Error)

	service := &PurchaseService{db: db}
	selectedID := uint(10)

	_, creditAccountID, err := service.normalizePurchasePaymentConfiguration(
		models.PurchasePaymentCredit,
		nil,
		&selectedID,
	)
	require.NoError(t, err)
	require.NotNil(t, creditAccountID)
	require.EqualValues(t, 11, *creditAccountID)
}

func uintPtr(value uint) *uint {
	return &value
}
