package models

import (
	"time"

	"gorm.io/gorm"
)

const (
	SaleBackorderAllocationStatusApplied  = "APPLIED"
	SaleBackorderAllocationStatusReversed = "REVERSED"
)

type SaleBackorderAllocation struct {
	ID                uint           `json:"id" gorm:"primaryKey"`
	SaleID            uint           `json:"sale_id" gorm:"not null;index"`
	SaleItemID        uint           `json:"sale_item_id" gorm:"not null;index"`
	ProductID         uint           `json:"product_id" gorm:"not null;index"`
	PurchaseID        *uint          `json:"purchase_id" gorm:"index"`
	PurchaseItemID    *uint          `json:"purchase_item_id" gorm:"index"`
	Quantity          int            `json:"quantity" gorm:"not null"`
	EstimatedUnitCost float64        `json:"estimated_unit_cost" gorm:"type:decimal(15,2);default:0"`
	ActualUnitCost    float64        `json:"actual_unit_cost" gorm:"type:decimal(15,2);default:0"`
	EstimatedAmount   float64        `json:"estimated_amount" gorm:"type:decimal(15,2);default:0"`
	ActualAmount      float64        `json:"actual_amount" gorm:"type:decimal(15,2);default:0"`
	VarianceAmount    float64        `json:"variance_amount" gorm:"type:decimal(15,2);default:0"`
	Status            string         `json:"status" gorm:"size:20;default:'APPLIED'"`
	Notes             string         `json:"notes" gorm:"type:text"`
	AppliedAt         *time.Time     `json:"applied_at"`
	ReversedAt        *time.Time     `json:"reversed_at"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `json:"-" gorm:"index"`

	Sale         Sale          `json:"sale,omitempty" gorm:"foreignKey:SaleID"`
	SaleItem     SaleItem      `json:"sale_item,omitempty" gorm:"foreignKey:SaleItemID"`
	Product      Product       `json:"product,omitempty" gorm:"foreignKey:ProductID"`
	Purchase     *Purchase     `json:"purchase,omitempty" gorm:"foreignKey:PurchaseID"`
	PurchaseItem *PurchaseItem `json:"purchase_item,omitempty" gorm:"foreignKey:PurchaseItemID"`
}
