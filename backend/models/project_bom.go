package models

import (
	"time"

	"gorm.io/gorm"
)

// ProjectBOM represents the attachment of a Master BOM template to a specific Project.
// It acts as a snapshot. If the Master BOM price changes, the Project Budget remains intact.
type ProjectBOM struct {
	ID        uint `json:"id" gorm:"primaryKey"`
	ProjectID uint `json:"project_id" gorm:"not null;index"`
	BOMID     uint `json:"bom_id" gorm:"not null;index"` // Reference to the Master Template used

	// Snapshot Data (copied from Master BOM at the time of attachment)
	BOMName         string  `json:"bom_name" gorm:"not null;size:150"`
	TargetProductID uint    `json:"target_product_id" gorm:"index"`
	Quantity        int     `json:"quantity" gorm:"default:1"`                      // How many units of this BOM are needed for the Project
	UnitCost        float64 `json:"unit_cost" gorm:"type:decimal(15,2);default:0"`  // Total Cost per 1 Unit of the BOM
	TotalCost       float64 `json:"total_cost" gorm:"type:decimal(15,2);default:0"` // Quantity * UnitCost

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relationships
	Project       Project          `json:"project" gorm:"foreignKey:ProjectID"`
	BOMTemplate   BOM              `json:"bom_template" gorm:"foreignKey:BOMID"`
	TargetProduct Product          `json:"target_product" gorm:"foreignKey:TargetProductID"`
	Items         []ProjectBOMItem `json:"items" gorm:"foreignKey:ProjectBOMID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// ProjectBOMItem is the snapshot of the BOM components, allowing PR Auto-Procurement to know exactly what is needed.
type ProjectBOMItem struct {
	ID                 uint `json:"id" gorm:"primaryKey"`
	ProjectBOMID       uint `json:"project_bom_id" gorm:"not null;index"`
	ComponentProductID uint `json:"component_product_id" gorm:"not null;index"`

	CategoryStr  string  `json:"category_str" gorm:"size:100"`
	BaseQuantity float64 `json:"base_quantity" gorm:"type:decimal(10,2)"`
	BaseUnit     string  `json:"base_unit" gorm:"size:50"`
	Notes        string  `json:"notes" gorm:"type:text"`

	// Multiplication of (Master RequirementQuantity * ProjectBOM Quantity)
	TotalRequirement float64 `json:"total_requirement" gorm:"type:decimal(10,2)"`
	UnitPrice        float64 `json:"unit_price" gorm:"type:decimal(15,2);default:0"`
	TotalCost        float64 `json:"total_cost" gorm:"type:decimal(15,2);default:0"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relationships
	ProjectBOM       ProjectBOM `json:"project_bom" gorm:"foreignKey:ProjectBOMID"`
	ComponentProduct Product    `json:"component_product" gorm:"foreignKey:ComponentProductID"`
}

// DTO
type ProjectBOMAddPayload struct {
	BOMID    uint `json:"bom_id" binding:"required"`
	Quantity int  `json:"quantity" binding:"required,min=1"`
}
