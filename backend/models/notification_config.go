package models

import (
	"gorm.io/gorm"
	"time"
)

// NotificationRule defines rule-based notification targeting
type NotificationRule struct {
	ID                uint           `json:"id" gorm:"primaryKey"`
	Name              string         `json:"name" gorm:"not null;size:100"`
	Description       string         `json:"description" gorm:"type:text"`
	Role              string         `json:"role" gorm:"size:50;index"` // employee, finance, director
	Department        string         `json:"department" gorm:"size:50"`
	MinAmount         float64        `json:"min_amount" gorm:"type:decimal(15,2);default:0"`
	MaxAmount         float64        `json:"max_amount" gorm:"type:decimal(15,2);default:0"` // 0 means no limit
	NotificationTypes []string       `json:"notification_types" gorm:"serializer:json"`      // Types of notifications to receive
	Priority          string         `json:"priority" gorm:"size:20;default:'NORMAL'"`
	IsActive          bool           `json:"is_active" gorm:"default:true"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `json:"-" gorm:"index"`
}

// NotificationPreference stores user-specific notification preferences
type NotificationPreference struct {
	ID                    uint   `json:"id" gorm:"primaryKey"`
	UserID                uint   `json:"user_id" gorm:"uniqueIndex;not null"`
	EmailEnabled          bool   `json:"email_enabled" gorm:"default:true"`
	InAppEnabled          bool   `json:"in_app_enabled" gorm:"default:true"`
	PushEnabled           bool   `json:"push_enabled" gorm:"default:false"`
	BatchNotifications    bool   `json:"batch_notifications" gorm:"default:false"`
	MaxDailyNotifications int    `json:"max_daily_notifications" gorm:"default:100"`
	QuietHoursStart       string `json:"quiet_hours_start" gorm:"size:5"` // Format: "HH:MM"
	QuietHoursEnd         string `json:"quiet_hours_end" gorm:"size:5"`   // Format: "HH:MM"

	// Notification type preferences
	ApprovalPending   bool `json:"approval_pending" gorm:"default:true"`
	ApprovalApproved  bool `json:"approval_approved" gorm:"default:true"`
	ApprovalRejected  bool `json:"approval_rejected" gorm:"default:true"`
	ApprovalEscalated bool `json:"approval_escalated" gorm:"default:true"`
	StockAlerts       bool `json:"stock_alerts" gorm:"default:true"`
	PaymentReminders  bool `json:"payment_reminders" gorm:"default:true"`
	SystemAlerts      bool `json:"system_alerts" gorm:"default:true"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	User User `json:"user" gorm:"foreignKey:UserID"`
}

// NotificationBatch groups similar notifications to reduce spam
type NotificationBatch struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	UserID       uint           `json:"user_id" gorm:"not null;index"`
	BatchType    string         `json:"batch_type" gorm:"not null;size:50"` // approval_pending, stock_alerts, etc
	ItemCount    int            `json:"item_count" gorm:"default:0"`
	TotalAmount  float64        `json:"total_amount" gorm:"type:decimal(15,2);default:0"`
	Summary      string         `json:"summary" gorm:"type:text"`
	DetailedData string         `json:"detailed_data" gorm:"type:jsonb"`
	IsProcessed  bool           `json:"is_processed" gorm:"default:false"`
	ProcessedAt  *time.Time     `json:"processed_at"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	User User `json:"user" gorm:"foreignKey:UserID"`
}

// NotificationQueue manages notification delivery scheduling
type NotificationQueue struct {
	ID             uint           `json:"id" gorm:"primaryKey"`
	NotificationID uint           `json:"notification_id" gorm:"not null;index"`
	UserID         uint           `json:"user_id" gorm:"not null;index"`
	ScheduledFor   time.Time      `json:"scheduled_for" gorm:"not null;index"`
	DeliveryMethod string         `json:"delivery_method" gorm:"size:20"` // IMMEDIATE, BATCH, SCHEDULED
	RetryCount     int            `json:"retry_count" gorm:"default:0"`
	Status         string         `json:"status" gorm:"size:20;default:'PENDING'"` // PENDING, SENT, FAILED
	SentAt         *time.Time     `json:"sent_at"`
	Error          string         `json:"error" gorm:"type:text"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	Notification Notification `json:"notification" gorm:"foreignKey:NotificationID"`
	User         User         `json:"user" gorm:"foreignKey:UserID"`
}

// Role-based notification matrix constants
const (
	// Notification scopes
	NotificationScopeOwn        = "OWN"        // Only own transactions
	NotificationScopeDepartment = "DEPARTMENT" // Department level
	NotificationScopeAll        = "ALL"        // All notifications

	// Delivery methods
	DeliveryImmediate = "IMMEDIATE"
	DeliveryBatch     = "BATCH"
	DeliveryScheduled = "SCHEDULED"

	// Queue status
	QueueStatusPending = "PENDING"
	QueueStatusSent    = "SENT"
	QueueStatusFailed  = "FAILED"
)

// NotificationMatrix defines what notifications each role should receive
type NotificationMatrix struct {
	Role                    string   `json:"role"`
	AllowedTypes            []string `json:"allowed_types"`
	AmountThreshold         float64  `json:"amount_threshold"`
	NotificationScope       string   `json:"notification_scope"`
	RequiresDepartmentMatch bool     `json:"requires_department_match"`
}

// GetNotificationMatrix returns the notification rules for each role
func GetNotificationMatrix() map[string]NotificationMatrix {
	return map[string]NotificationMatrix{
		"employee": {
			Role: "employee",
			AllowedTypes: []string{
				NotificationTypeApprovalApproved,
				NotificationTypeApprovalRejected,
				"OWN_PURCHASE_STATUS",
			},
			AmountThreshold:         0,
			NotificationScope:       NotificationScopeOwn,
			RequiresDepartmentMatch: false,
		},
		"finance_staff": {
			Role: "finance_staff",
			AllowedTypes: []string{
				NotificationTypeApprovalPending,
				NotificationTypeApprovalEscalated,
				"BUDGET_ALERT",
			},
			AmountThreshold:         0,
			NotificationScope:       NotificationScopeAll,
			RequiresDepartmentMatch: false,
		},
		"finance": {
			Role: "finance",
			AllowedTypes: []string{
				NotificationTypeApprovalPending,
				NotificationTypeApprovalEscalated,
				NotificationTypeLowStock,
				NotificationTypeStockOut,
				"PAYMENT_DUE",
				"BUDGET_ALERT",
			},
			AmountThreshold:         25000000, // Max approval amount for finance
			NotificationScope:       NotificationScopeAll,
			RequiresDepartmentMatch: false,
		},
		"director": {
			Role: "director",
			AllowedTypes: []string{
				NotificationTypeApprovalPending,
				NotificationTypeApprovalEscalated,
				"HIGH_VALUE_PURCHASE",
				"CRITICAL_ALERT",
				"MONTHLY_SUMMARY",
				"BUDGET_EXCEEDED",
			},
			AmountThreshold:         25000001, // Min amount for director approval
			NotificationScope:       NotificationScopeAll,
			RequiresDepartmentMatch: false,
		},
		"admin": {
			Role: "admin",
			AllowedTypes: []string{
				"ALL", // Admin receives all notification types
			},
			AmountThreshold:         0,
			NotificationScope:       NotificationScopeAll,
			RequiresDepartmentMatch: false,
		},
	}
}

// Additional notification types for smart system
const (
	NotificationTypeApprovalEscalated = "APPROVAL_ESCALATED"
	NotificationTypePaymentDue        = "PAYMENT_DUE"
	NotificationTypeBudgetAlert       = "BUDGET_ALERT"
	NotificationTypeBudgetExceeded    = "BUDGET_EXCEEDED"
	NotificationTypeCriticalAlert     = "CRITICAL_ALERT"
	NotificationTypeMonthlySummary    = "MONTHLY_SUMMARY"
	NotificationTypeHighValuePurchase = "HIGH_VALUE_PURCHASE"
)
