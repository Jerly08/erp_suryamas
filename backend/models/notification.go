package models

import (
	"time"

	"gorm.io/gorm"
)

type Notification struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	UserID    uint           `json:"user_id" gorm:"not null;index"`
	Type      string         `json:"type" gorm:"not null;size:50"` // LOW_STOCK, STOCK_OUT, REORDER_ALERT
	Title     string         `json:"title" gorm:"not null;size:200"`
	Message   string         `json:"message" gorm:"type:text"`
	Data      string         `json:"data" gorm:"type:jsonb"` // JSON data for additional context
	IsRead    bool           `json:"is_read" gorm:"default:false"`
	ReadAt    *time.Time     `json:"read_at"`
	Priority  string         `json:"priority" gorm:"size:20;default:'NORMAL'"` // LOW, NORMAL, HIGH, URGENT
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	User User `json:"user" gorm:"foreignKey:UserID"`
}

type StockAlert struct {
	ID             uint           `json:"id" gorm:"primaryKey"`
	ProductID      uint           `json:"product_id" gorm:"not null;index"`
	AlertType      string         `json:"alert_type" gorm:"not null;size:50"` // LOW_STOCK, OUT_OF_STOCK, OVERSTOCK
	CurrentStock   int            `json:"current_stock"`
	ThresholdStock int            `json:"threshold_stock"`
	Status         string         `json:"status" gorm:"size:20;default:'ACTIVE'"` // ACTIVE, RESOLVED, DISMISSED
	LastAlertAt    time.Time      `json:"last_alert_at"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	Product Product `json:"product" gorm:"foreignKey:ProductID"`
}

// Notification Types Constants
const (
	NotificationTypeLowStock         = "LOW_STOCK"
	NotificationTypeStockOut         = "STOCK_OUT"
	NotificationTypeReorderAlert     = "REORDER_ALERT"
	NotificationTypeOverstock        = "OVERSTOCK"
	NotificationTypeMinStock         = "MIN_STOCK"
	NotificationTypeApprovalPending  = "APPROVAL_PENDING"
	NotificationTypeApprovalApproved = "APPROVAL_APPROVED"
	NotificationTypeApprovalRejected = "APPROVAL_REJECTED"
)

// Notification Priority Constants
const (
	NotificationPriorityLow    = "LOW"
	NotificationPriorityNormal = "NORMAL"
	NotificationPriorityMedium = "MEDIUM"
	NotificationPriorityHigh   = "HIGH"
	NotificationPriorityUrgent = "URGENT"
)

// Stock Alert Types Constants
const (
	StockAlertTypeLowStock     = "LOW_STOCK"
	StockAlertTypeOutOfStock   = "OUT_OF_STOCK"
	StockAlertTypeOverstock    = "OVERSTOCK"
	StockAlertTypeReorderAlert = "REORDER_ALERT"
)

// Stock Alert Status Constants
const (
	StockAlertStatusActive    = "ACTIVE"
	StockAlertStatusResolved  = "RESOLVED"
	StockAlertStatusDismissed = "DISMISSED"
)
