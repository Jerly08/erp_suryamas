package services

import (
	"testing"

	"app-sistem-akuntansi/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetAssetCategoriesSeedsDefaultsWhenTableIsEmpty(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	require.NoError(t, db.AutoMigrate(&models.AssetCategory{}))

	service := &AssetService{db: db}

	categories, err := service.GetAssetCategories()
	require.NoError(t, err)
	require.Len(t, categories, len(defaultAssetCategories()))

	var count int64
	require.NoError(t, db.Model(&models.AssetCategory{}).Where("is_active = ?", true).Count(&count).Error)
	require.EqualValues(t, len(defaultAssetCategories()), count)
}
