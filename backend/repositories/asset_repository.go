package repositories

import (
	"app-sistem-akuntansi/models"
	"errors"
	"strings"

	"gorm.io/gorm"
)

type AssetRepositoryInterface interface {
	FindAll() ([]models.Asset, error)
	FindByID(id uint) (*models.Asset, error)
	FindByCode(code string) (*models.Asset, error)
	CodeExists(code string) (bool, error)
	Create(asset *models.Asset) error
	Update(asset *models.Asset) error
	Delete(id uint) error
	GetActiveAssets() ([]models.Asset, error)
	GetAssetsByCategory(category string) ([]models.Asset, error)
	GetAssetsForDepreciation() ([]models.Asset, error)
	Count() (int64, error)
	GetTotalValue() (float64, error)
}

type AssetRepository struct {
	db *gorm.DB
}

func NewAssetRepository(db *gorm.DB) AssetRepositoryInterface {
	return &AssetRepository{db: db}
}

// FindAll retrieves all assets with preloaded relations
func (r *AssetRepository) FindAll() ([]models.Asset, error) {
	var assets []models.Asset
	err := r.db.Preload("AssetAccount").Preload("DepreciationAccount").Preload("AccumulatedDepreciationAccount").Find(&assets).Error
	return assets, err
}

// FindByID retrieves asset by ID with relations
func (r *AssetRepository) FindByID(id uint) (*models.Asset, error) {
	var asset models.Asset
	err := r.db.Preload("AssetAccount").Preload("DepreciationAccount").Preload("AccumulatedDepreciationAccount").First(&asset, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("asset not found")
		}
		return nil, err
	}
	return &asset, nil
}

// FindByCode retrieves asset by code
func (r *AssetRepository) FindByCode(code string) (*models.Asset, error) {
	var asset models.Asset
	err := r.db.Where("code = ?", code).First(&asset).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("asset not found")
		}
		return nil, err
	}
	return &asset, nil
}

// CodeExists checks if an asset code already exists, including soft-deleted ones
func (r *AssetRepository) CodeExists(code string) (bool, error) {
	var count int64
	err := r.db.Unscoped().Model(&models.Asset{}).Where("code = ?", code).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Create creates a new asset
func (r *AssetRepository) Create(asset *models.Asset) error {
	// Let the database handle unique constraint checking to prevent race conditions
	// The service layer handles retries if there's a unique constraint violation
	intendedIsActive := asset.IsActive
	if err := r.db.Select("*").Create(asset).Error; err != nil {
		if strings.Contains(err.Error(), "assets_pkey") {
			if syncErr := r.db.Exec(`
				SELECT setval(
					pg_get_serial_sequence('assets', 'id'),
					COALESCE((SELECT MAX(id) FROM assets), 1),
					true
				)
			`).Error; syncErr == nil {
				if retryErr := r.db.Select("*").Create(asset).Error; retryErr == nil {
					goto applyActiveFlag
				} else {
					return retryErr
				}
			}
		}
		return err
	}

	// GORM can still fall back to the DB default for bool fields with `default:true`.
	// Re-apply the intended flag explicitly so PENDING/INACTIVE assets persist as false.
applyActiveFlag:
	if err := r.db.Exec("UPDATE assets SET is_active = ? WHERE id = ?", intendedIsActive, asset.ID).Error; err != nil {
		return err
	}
	asset.IsActive = intendedIsActive
	return nil
}

// Update updates an existing asset
func (r *AssetRepository) Update(asset *models.Asset) error {
	// Check if asset exists
	var existingAsset models.Asset
	err := r.db.First(&existingAsset, asset.ID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("asset not found")
		}
		return err
	}

	// Check if code is being changed and if new code already exists
	if existingAsset.Code != asset.Code {
		var codeCheck models.Asset
		err := r.db.Where("code = ? AND id != ?", asset.Code, asset.ID).First(&codeCheck).Error
		if err == nil {
			return errors.New("asset code already exists")
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}

	return r.db.Save(asset).Error
}

// Delete soft deletes an asset
func (r *AssetRepository) Delete(id uint) error {
	// Check if asset exists
	var asset models.Asset
	err := r.db.First(&asset, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("asset not found")
		}
		return err
	}

	return r.db.Delete(&asset).Error
}

// GetActiveAssets retrieves all active assets
func (r *AssetRepository) GetActiveAssets() ([]models.Asset, error) {
	var assets []models.Asset
	err := r.db.Where("is_active = ? AND status = ?", true, models.AssetStatusActive).
		Preload("AssetAccount").Preload("DepreciationAccount").Preload("AccumulatedDepreciationAccount").
		Find(&assets).Error
	return assets, err
}

// GetAssetsByCategory retrieves assets by category
func (r *AssetRepository) GetAssetsByCategory(category string) ([]models.Asset, error) {
	var assets []models.Asset
	err := r.db.Where("category = ? AND is_active = ?", category, true).
		Preload("AssetAccount").Preload("DepreciationAccount").Preload("AccumulatedDepreciationAccount").
		Find(&assets).Error
	return assets, err
}

// GetAssetsForDepreciation retrieves assets that need depreciation calculation
func (r *AssetRepository) GetAssetsForDepreciation() ([]models.Asset, error) {
	var assets []models.Asset
	err := r.db.Where("is_active = ? AND status = ? AND useful_life > 0",
		true, models.AssetStatusActive).
		Preload("AssetAccount").Preload("DepreciationAccount").Preload("AccumulatedDepreciationAccount").
		Find(&assets).Error
	return assets, err
}

// Count returns total number of assets
func (r *AssetRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&models.Asset{}).Count(&count).Error
	return count, err
}

// GetTotalValue calculates total value of all active assets
func (r *AssetRepository) GetTotalValue() (float64, error) {
	var totalValue float64
	err := r.db.Model(&models.Asset{}).
		Where("is_active = ? AND status = ?", true, models.AssetStatusActive).
		Select("COALESCE(SUM(purchase_price - accumulated_depreciation), 0)").
		Scan(&totalValue).Error
	return totalValue, err
}
