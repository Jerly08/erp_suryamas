package models

import (
	"time"

	"gorm.io/gorm"
)

// ComponentTemplate stores a reusable set of components (materials) that can be
// loaded into a BOM form to avoid adding items one-by-one.
type ComponentTemplate struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	Name        string `json:"name" gorm:"not null;size:150;uniqueIndex"`
	Description string `json:"description" gorm:"type:text"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	Items []ComponentTemplateItem `json:"items" gorm:"foreignKey:TemplateID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// ComponentTemplateItem is a single component row inside a ComponentTemplate.
// It stores product reference, category, requirement quantity, unit and notes.
// unit_price and stock are NOT stored — they are fetched live from the Product table when loaded.
type ComponentTemplateItem struct {
	ID                 uint    `json:"id" gorm:"primaryKey"`
	TemplateID         uint    `json:"template_id" gorm:"not null;index"`
	ComponentProductID uint    `json:"component_product_id" gorm:"not null;index"`
	CategoryStr        string  `json:"category_str" gorm:"size:100"`
	RequirementQty     float64 `json:"requirement_quantity" gorm:"type:decimal(10,2)"`
	BaseUnit           string  `json:"base_unit" gorm:"size:50"`
	Notes              string  `json:"notes" gorm:"type:text"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relationships
	ComponentProduct Product `json:"component_product" gorm:"foreignKey:ComponentProductID"`
}

// --- DTOs ---

type ComponentTemplateCreateRequest struct {
	Name        string                         `json:"name" binding:"required"`
	Description string                         `json:"description"`
	Items       []ComponentTemplateItemPayload `json:"items" binding:"required,min=1"`
}

type ComponentTemplateItemPayload struct {
	ComponentProductID uint    `json:"component_product_id" binding:"required"`
	CategoryStr        string  `json:"category_str"`
	RequirementQty     float64 `json:"requirement_quantity" binding:"required,gt=0"`
	BaseUnit           string  `json:"base_unit"`
	Notes              string  `json:"notes"`
}
