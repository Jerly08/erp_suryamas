package models

import (
	"time"

	"gorm.io/gorm"
)

// Project represents a cost center or tracking entity for budgeting vs actuals
type Project struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	Code        string `json:"code" gorm:"unique;not null;size:50"`
	Name        string `json:"name" gorm:"not null;size:150"`
	Description string `json:"description" gorm:"type:text"`
	Status      string `json:"status" gorm:"size:20;default:'ACTIVE'"` // ACTIVE, COMPLETED, ON_HOLD, CANCELLED

	StartDate *time.Time `json:"start_date"`
	EndDate   *time.Time `json:"end_date"`

	// Financial Tracking
	TotalBudget         float64 `json:"total_budget" gorm:"type:decimal(15,2);default:0"`
	TotalActualCost     float64 `json:"total_actual_cost" gorm:"type:decimal(15,2);default:0"`
	TotalSalesRevenue   float64 `json:"total_sales_revenue" gorm:"type:decimal(15,2);default:0"`
	SalesPipelineAmount float64 `json:"sales_pipeline_amount" gorm:"-"`

	// Relations (Manager, etc.)
	ManagerID *uint `json:"manager_id" gorm:"index"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Foreign Keys
	Manager     *User        `json:"manager,omitempty" gorm:"foreignKey:ManagerID"`
	ProjectBOMs []ProjectBOM `json:"project_boms" gorm:"foreignKey:ProjectID"`
}

// Project Status Constants
const (
	ProjectStatusActive    = "ACTIVE"
	ProjectStatusCompleted = "COMPLETED"
	ProjectStatusOnHold    = "ON_HOLD"
	ProjectStatusCancelled = "CANCELLED"
)

// DTOs for Create/Update API
type ProjectCreateRequest struct {
	Code        string     `json:"code" binding:"required"`
	Name        string     `json:"name" binding:"required"`
	Description string     `json:"description"`
	StartDate   *time.Time `json:"start_date"`
	EndDate     *time.Time `json:"end_date"`
	TotalBudget float64    `json:"total_budget"`
	ManagerID   *uint      `json:"manager_id"`
}

type ProjectUpdateRequest struct {
	Name           *string    `json:"name"`
	Description    *string    `json:"description"`
	Status         *string    `json:"status"`
	StartDate      *time.Time `json:"start_date"`
	EndDate        *time.Time `json:"end_date"`
	ClearStartDate bool       `json:"clear_start_date"`
	ClearEndDate   bool       `json:"clear_end_date"`
	TotalBudget    *float64   `json:"total_budget"`
	ManagerID      *uint      `json:"manager_id"`
}

type ProjectFilter struct {
	Page   int
	Limit  int
	Search string
}
