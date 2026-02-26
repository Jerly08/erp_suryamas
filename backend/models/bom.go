package models

import (
	"time"

	"gorm.io/gorm"
)

// BOM represents the header of a Bill of Materials master template "Recipe".
// It defines the final assembly outcome.
type BOM struct {
	ID              uint    `json:"id" gorm:"primaryKey"`
	Code            string  `json:"code" gorm:"unique;not null;size:50"`
	TargetProductID uint    `json:"target_product_id" gorm:"index"`   // The final assembled product
	TargetQuantity  int     `json:"target_quantity" gorm:"default:1"` // How many units are produced from this BOM
	Name            string  `json:"name" gorm:"not null;size:150"`
	Description     string  `json:"description" gorm:"type:text"`
	TotalCost       float64 `json:"total_cost" gorm:"type:decimal(15,2);default:0"` // Total Cost per Unit (Sum of all item total costs)

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relationships
	TargetProduct Product   `json:"target_product" gorm:"foreignKey:TargetProductID"`
	Items         []BOMItem `json:"items" gorm:"foreignKey:BOMID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// BOMItem represents a single component/material row matching the "Base Cost" calculation side.
type BOMItem struct {
	ID                 uint `json:"id" gorm:"primaryKey"`
	BOMID              uint `json:"bom_id" gorm:"not null;index"`
	ComponentProductID uint `json:"component_product_id" gorm:"not null;index"` // The raw material

	CategoryStr  string  `json:"category_str" gorm:"size:100"`            // Grouping category (e.g., A. Kelengkapan, B. Name Plate)
	BaseQuantity float64 `json:"base_quantity" gorm:"type:decimal(10,2)"` // Qty Input in Persiapan Unit
	BaseUnit     string  `json:"base_unit" gorm:"size:50"`                // Unit Input in Persiapan Unit (e.g., L, Pcs, Unit)
	Notes        string  `json:"notes" gorm:"type:text"`                  // Keterangan

	UnitPrice           float64 `json:"unit_price" gorm:"type:decimal(15,2);default:0"`     // Pulled from Product's Purchase Price
	RequirementQuantity float64 `json:"requirement_quantity" gorm:"type:decimal(10,2)"`     // The strict needed quantity (Kebutuhan)
	TotalCost           float64 `json:"total_cost" gorm:"type:decimal(15,2);default:0"`     // RequirementQuantity * UnitPrice
	CostPercentage      float64 `json:"cost_percentage" gorm:"type:decimal(5,2);default:0"` // (TotalCost / BOM.TotalCost) * 100

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relationships
	BOM              BOM     `json:"bom,omitempty" gorm:"foreignKey:BOMID"`
	ComponentProduct Product `json:"component_product" gorm:"foreignKey:ComponentProductID"`
}

// DTOs for Create/Update API

type BOMCreateRequest struct {
	Code            string                 `json:"code" binding:"required"`
	TargetProductID uint                   `json:"target_product_id" binding:"required"`
	TargetQuantity  int                    `json:"target_quantity" binding:"required,min=1"`
	Name            string                 `json:"name" binding:"required"`
	Description     string                 `json:"description"`
	Items           []BOMItemCreatePayload `json:"items" binding:"required,min=1"`
}

type BOMItemCreatePayload struct {
	ComponentProductID  uint    `json:"component_product_id" binding:"required"`
	CategoryStr         string  `json:"category_str"`
	BaseQuantity        float64 `json:"base_quantity"`
	BaseUnit            string  `json:"base_unit"`
	Notes               string  `json:"notes"`
	UnitPrice           float64 `json:"unit_price"` // Can be passed from frontend snapshot
	RequirementQuantity float64 `json:"requirement_quantity" binding:"required,gt=0"`
}

type BOMUpdateRequest struct {
	TargetProductID *uint                   `json:"target_product_id"`
	TargetQuantity  *int                    `json:"target_quantity"`
	Name            *string                 `json:"name"`
	Description     *string                 `json:"description"`
	Items           *[]BOMItemCreatePayload `json:"items"` // Complete replacement array if provided
}
