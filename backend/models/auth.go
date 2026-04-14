package models

import (
	"gorm.io/gorm"
	"time"
)

// RefreshToken represents a refresh token in the database
type RefreshToken struct {
	ID         uint           `json:"id" gorm:"primaryKey"`
	Token      string         `json:"token" gorm:"unique;not null;type:text"`
	UserID     uint           `json:"user_id" gorm:"not null;index"`
	ExpiresAt  time.Time      `json:"expires_at"`
	IsRevoked  bool           `json:"is_revoked" gorm:"default:false"`
	DeviceInfo string         `json:"device_info" gorm:"size:500"`
	IPAddress  string         `json:"ip_address" gorm:"size:45"`
	UserAgent  string         `json:"user_agent" gorm:"type:text"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	User User `json:"user" gorm:"foreignKey:UserID"`
}

// UserSession represents an active user session
type UserSession struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	SessionID    string         `json:"session_id" gorm:"unique;not null;size:255"`
	UserID       uint           `json:"user_id" gorm:"not null;index"`
	IPAddress    string         `json:"ip_address" gorm:"size:45"`
	UserAgent    string         `json:"user_agent" gorm:"type:text"`
	DeviceInfo   string         `json:"device_info" gorm:"size:500"`
	LastActivity time.Time      `json:"last_activity"`
	ExpiresAt    time.Time      `json:"expires_at"`
	IsActive     bool           `json:"is_active" gorm:"default:true"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	User User `json:"user" gorm:"foreignKey:UserID"`
}

// BlacklistedToken represents a blacklisted JWT token
type BlacklistedToken struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Token     string         `json:"token" gorm:"unique;not null;size:1000"`
	UserID    uint           `json:"user_id" gorm:"not null;index"`
	ExpiresAt time.Time      `json:"expires_at"`
	Reason    string         `json:"reason" gorm:"size:100"` // LOGOUT, REVOKED, COMPROMISED
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	User User `json:"user" gorm:"foreignKey:UserID"`
}

// AuthAttempt represents login attempts for security monitoring
type AuthAttempt struct {
	ID            uint           `json:"id" gorm:"primaryKey"`
	Email         string         `json:"email" gorm:"size:100;index"`
	Username      string         `json:"username" gorm:"size:50;index"`
	IPAddress     string         `json:"ip_address" gorm:"size:45"`
	UserAgent     string         `json:"user_agent" gorm:"type:text"`
	Success       bool           `json:"success" gorm:"default:false"`
	FailureReason string         `json:"failure_reason" gorm:"size:100"`
	AttemptedAt   time.Time      `json:"attempted_at"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
}

// RateLimitRecord represents rate limiting data
type RateLimitRecord struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	IPAddress    string         `json:"ip_address" gorm:"unique;not null;size:45"`
	Endpoint     string         `json:"endpoint" gorm:"size:100"`
	Attempts     int            `json:"attempts" gorm:"default:0"`
	WindowStart  time.Time      `json:"window_start"`
	BlockedUntil *time.Time     `json:"blocked_until"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

// Enhanced login/register request structures
type EnhancedLoginRequest struct {
	EmailOrUsername string `json:"email_or_username" binding:"required" validate:"required"`
	Password        string `json:"password" binding:"required" validate:"required,min=6"`
	RememberMe      bool   `json:"remember_me"`
	DeviceInfo      string `json:"device_info"`
}

type EnhancedRegisterRequest struct {
	Username        string `json:"username" binding:"required" validate:"required,min=3,max=50,alphanum"`
	Email           string `json:"email" binding:"required" validate:"required,email"`
	Password        string `json:"password" binding:"required" validate:"required,min=8,containsany=!@#$%^&*,containsany=0123456789,containsany=ABCDEFGHIJKLMNOPQRSTUVWXYZ"`
	ConfirmPassword string `json:"confirm_password" binding:"required" validate:"required,eqfield=Password"`
	FirstName       string `json:"first_name" binding:"required" validate:"required,min=2,max=50"`
	LastName        string `json:"last_name" binding:"required" validate:"required,min=2,max=50"`
	Role            string `json:"role" validate:"omitempty,oneof=admin finance director inventory_manager employee auditor"`
	Phone           string `json:"phone" validate:"omitempty,e164"`
	Department      string `json:"department" validate:"omitempty,max=50"`
	Position        string `json:"position" validate:"omitempty,max=50"`
}

type TokenResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int64     `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         User      `json:"user"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required" validate:"required,min=8,containsany=!@#$%^&*,containsany=0123456789,containsany=ABCDEFGHIJKLMNOPQRSTUVWXYZ"`
	ConfirmPassword string `json:"confirm_password" binding:"required" validate:"required,eqfield=NewPassword"`
}

type ResetPasswordRequest struct {
	Email string `json:"email" binding:"required" validate:"required,email"`
}

// Permission and Role definitions
type Permission struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Name        string         `json:"name" gorm:"unique;not null;size:100"`
	Resource    string         `json:"resource" gorm:"not null;size:50"` // users, accounts, products, etc.
	Action      string         `json:"action" gorm:"not null;size:20"`   // create, read, update, delete
	Description string         `json:"description" gorm:"type:text"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

type RolePermission struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	Role         string         `json:"role" gorm:"not null;size:20;index"`
	PermissionID uint           `json:"permission_id" gorm:"not null;index"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	Permission Permission `json:"permission" gorm:"foreignKey:PermissionID"`
}

// Blacklist reasons constants
const (
	BlacklistReasonLogout      = "LOGOUT"
	BlacklistReasonRevoked     = "REVOKED"
	BlacklistReasonCompromised = "COMPROMISED"
	BlacklistReasonExpired     = "EXPIRED"
)

// Auth failure reasons constants
const (
	FailureReasonInvalidCredentials = "INVALID_CREDENTIALS"
	FailureReasonAccountDisabled    = "ACCOUNT_DISABLED"
	FailureReasonAccountLocked      = "ACCOUNT_LOCKED"
	FailureReasonTooManyAttempts    = "TOO_MANY_ATTEMPTS"
	FailureReasonInvalidToken       = "INVALID_TOKEN"
	FailureReasonExpiredToken       = "EXPIRED_TOKEN"
)

// User roles constants (moved from user.go for consistency)
const (
	RoleAdmin            = "admin"
	RoleFinance          = "finance"
	RoleFinanceManager   = "finance_manager"
	RoleFinanceStaff     = "finance_staff"
	RoleDirector         = "director"
	RoleInventoryManager = "inventory_manager"
	RoleEmployee         = "employee"
	RoleAuditor          = "auditor"
	RoleOperationalUser  = "operational_user"
)
