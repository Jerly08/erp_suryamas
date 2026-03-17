package models

import (
	"gorm.io/gorm"
	"time"
)

type AssetCategory struct {
	ID                                 uint           `json:"id" gorm:"primaryKey"`
	Code                               string         `json:"code" gorm:"unique;not null;size:20"`
	Name                               string         `json:"name" gorm:"not null;size:100"`
	Description                        string         `json:"description" gorm:"type:text"`
	ParentID                           *uint          `json:"parent_id" gorm:"index"`
	IsActive                           bool           `json:"is_active" gorm:"default:true"`
	FixedAssetAccountCode              string         `json:"fixed_asset_account_code" gorm:"size:20"`
	AccumulatedDepreciationAccountCode string         `json:"accumulated_depreciation_account_code" gorm:"size:20"`
	CreatedAt                          time.Time      `json:"created_at"`
	UpdatedAt                          time.Time      `json:"updated_at"`
	DeletedAt                          gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	Parent   *AssetCategory  `json:"parent,omitempty" gorm:"foreignKey:ParentID"`
	Children []AssetCategory `json:"children,omitempty" gorm:"foreignKey:ParentID"`
	Assets   []Asset         `json:"-" gorm:"foreignKey:CategoryID"`
}

type Asset struct {
	ID                 uint           `json:"id" gorm:"primaryKey"`
	Code               string         `json:"code" gorm:"unique;not null;size:20"`
	Name               string         `json:"name" gorm:"not null;size:100"`
	CategoryID         *uint          `json:"category_id" gorm:"index"`
	Category           string         `json:"category" gorm:"size:50"`        // Keep for backward compatibility
	Status             string         `json:"status" gorm:"not null;size:20"` // ACTIVE, INACTIVE, SOLD
	PurchaseDate       time.Time      `json:"purchase_date"`
	PurchasePrice      float64        `json:"purchase_price" gorm:"type:decimal(15,2);default:0"`
	SalvageValue       float64        `json:"salvage_value" gorm:"type:decimal(15,2);default:0"`
	UsefulLife         int            `json:"useful_life" gorm:"default:0"`       // Years
	DepreciationMethod string         `json:"depreciation_method" gorm:"size:20"` // STRAIGHT_LINE, DECLINING_BALANCE
	IsActive           bool           `json:"is_active" gorm:"default:true"`
	Notes              string         `json:"notes" gorm:"type:text"`
	Location           string         `json:"location" gorm:"size:100"`
	Coordinates        string         `json:"coordinates" gorm:"size:50"` // "lat,lng" format
	MapsURL            string         `json:"maps_url" gorm:"size:500"`   // Generated Google Maps URL
	SerialNumber       string         `json:"serial_number" gorm:"size:50"`
	Condition          string         `json:"condition" gorm:"size:20;default:'Good'"`
	ImagePath          string         `json:"image_path" gorm:"size:255"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	AssetAccountID                   *uint          `json:"asset_account_id" gorm:"index"`
	DepreciationAccountID            *uint          `json:"depreciation_account_id" gorm:"index"`
	AccumulatedDepreciationAccountID *uint          `json:"accumulated_depreciation_account_id" gorm:"index"`
	AccumulatedDepreciation          float64        `json:"accumulated_depreciation" gorm:"type:decimal(15,2);default:0"`
	OpeningAccumulatedDepreciation   float64        `json:"opening_accumulated_depreciation" gorm:"type:decimal(15,2);default:0"`
	AssetCategory                    *AssetCategory `json:"asset_category,omitempty" gorm:"foreignKey:CategoryID"`
	AssetAccount                     *Account       `json:"asset_account" gorm:"foreignKey:AssetAccountID"`
	DepreciationAccount              *Account       `json:"depreciation_account" gorm:"foreignKey:DepreciationAccountID"`
	AccumulatedDepreciationAccount   *Account       `json:"accumulated_depreciation_account" gorm:"foreignKey:AccumulatedDepreciationAccountID"`
}

// Asset Status Constants
const (
	AssetStatusActive       = "ACTIVE"
	AssetStatusInactive     = "INACTIVE"
	AssetStatusSold         = "SOLD"
	AssetStatusPendingSetup = "PENDING" // Auto-created from purchase, awaiting user to set useful life/salvage/method
)

// Depreciation Methods Constants
const (
	DepreciationMethodStraightLine     = "STRAIGHT_LINE"
	DepreciationMethodDecliningBalance = "DECLINING_BALANCE"
)
