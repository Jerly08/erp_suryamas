package models

import (
	"gorm.io/gorm"
	"time"
)

type User struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Username    string         `json:"username" gorm:"unique;not null;size:50"`
	Email       string         `json:"email" gorm:"unique;not null;size:100"`
	Password    string         `json:"-" gorm:"not null;size:255"`
	Role        string         `json:"role" gorm:"not null;default:'employee';size:20"` // admin, director, finance, employee, inventory_manager
	FirstName   string         `json:"first_name" gorm:"size:50"`
	LastName    string         `json:"last_name" gorm:"size:50"`
	Phone       string         `json:"phone" gorm:"size:20"`
	Address     string         `json:"address" gorm:"type:text"`
	Department  string         `json:"department" gorm:"size:50"`
	Position    string         `json:"position" gorm:"size:50"`
	HireDate    *time.Time     `json:"hire_date"`
	Salary      float64        `json:"-" gorm:"type:decimal(15,2)"`
	IsActive    bool           `json:"is_active" gorm:"default:true"`
	LastLoginAt *time.Time     `json:"last_login_at"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	CreatedAuditLogs []AuditLog `json:"-" gorm:"foreignKey:UserID"`
	Sales            []Sale     `json:"-" gorm:"foreignKey:UserID"`
	Purchases        []Purchase `json:"-" gorm:"foreignKey:UserID"`
}

// GetDisplayName returns a formatted display name for the user
func (u *User) GetDisplayName() string {
	if u.FirstName != "" && u.LastName != "" {
		return u.FirstName + " " + u.LastName
	} else if u.FirstName != "" {
		return u.FirstName
	} else if u.LastName != "" {
		return u.LastName
	}
	return u.Username
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Username   string `json:"username" binding:"required"`
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"required,min=6"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Role       string `json:"role"`
	Phone      string `json:"phone"`
	Department string `json:"department"`
	Position   string `json:"position"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
