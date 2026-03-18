package controllers

import (
	"app-sistem-akuntansi/models"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"net/http"
	"strconv"
)

type UserController struct {
	db *gorm.DB
}

func getCurrentUserID(c *gin.Context) (uint, bool) {
	rawUserID, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}

	switch v := rawUserID.(type) {
	case uint:
		return v, true
	case int:
		if v < 0 {
			return 0, false
		}
		return uint(v), true
	case int64:
		if v < 0 {
			return 0, false
		}
		return uint(v), true
	case float64:
		if v < 0 {
			return 0, false
		}
		return uint(v), true
	default:
		return 0, false
	}
}

func (uc *UserController) activeAdminCount() (int64, error) {
	var count int64
	err := uc.db.Model(&models.User{}).
		Where("role = ? AND is_active = ?", models.RoleAdmin, true).
		Count(&count).Error
	return count, err
}

func NewUserController(db *gorm.DB) *UserController {
	return &UserController{db: db}
}

// GetUsers retrieves users with optional filters
func (uc *UserController) GetUsers(c *gin.Context) {
	var users []models.User
	query := uc.db.Model(&models.User{})

	// Apply filters
	if status := c.Query("status"); status != "" {
		if status == "active" {
			query = query.Where("is_active = ?", true)
		} else if status == "inactive" {
			query = query.Where("is_active = ?", false)
		}
	}

	if role := c.Query("role"); role != "" {
		query = query.Where("role = ?", role)
	}

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset := (page - 1) * limit

	// Get total count
	var total int64
	query.Count(&total)

	// Apply pagination
	query = query.Offset(offset).Limit(limit)

	// Execute query (exclude sensitive fields)
	if err := query.Select("id, username, email, role, first_name, last_name, phone, address, department, position, hire_date, is_active, last_login_at, created_at, updated_at").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}

	// Calculate full name for each user
	for i := range users {
		fullName := ""
		if users[i].FirstName != "" {
			fullName = users[i].FirstName
		}
		if users[i].LastName != "" {
			if fullName != "" {
				fullName += " " + users[i].LastName
			} else {
				fullName = users[i].LastName
			}
		}
		if fullName == "" {
			fullName = users[i].Username
		}
		// Add full_name to the response (we'll use a map)
	}

	// Convert to response format with full_name
	response := make([]map[string]interface{}, len(users))
	for i, user := range users {
		fullName := ""
		if user.FirstName != "" {
			fullName = user.FirstName
		}
		if user.LastName != "" {
			if fullName != "" {
				fullName += " " + user.LastName
			} else {
				fullName = user.LastName
			}
		}
		if fullName == "" {
			fullName = user.Username
		}

		response[i] = map[string]interface{}{
			"id":            user.ID,
			"username":      user.Username,
			"email":         user.Email,
			"role":          user.Role,
			"first_name":    user.FirstName,
			"last_name":     user.LastName,
			"full_name":     fullName,
			"phone":         user.Phone,
			"address":       user.Address,
			"department":    user.Department,
			"position":      user.Position,
			"hire_date":     user.HireDate,
			"is_active":     user.IsActive,
			"last_login_at": user.LastLoginAt,
			"created_at":    user.CreatedAt,
			"updated_at":    user.UpdatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data": response,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": total,
			"pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// GetUser retrieves a single user by ID
func (uc *UserController) GetUser(c *gin.Context) {
	id := c.Param("id")
	userID, err := strconv.Atoi(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var user models.User
	if err := uc.db.Select("id, username, email, role, first_name, last_name, phone, address, department, position, hire_date, is_active, last_login_at, created_at, updated_at").First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user"})
		return
	}

	// Calculate full name
	fullName := ""
	if user.FirstName != "" {
		fullName = user.FirstName
	}
	if user.LastName != "" {
		if fullName != "" {
			fullName += " " + user.LastName
		} else {
			fullName = user.LastName
		}
	}
	if fullName == "" {
		fullName = user.Username
	}

	response := map[string]interface{}{
		"id":            user.ID,
		"username":      user.Username,
		"email":         user.Email,
		"role":          user.Role,
		"first_name":    user.FirstName,
		"last_name":     user.LastName,
		"full_name":     fullName,
		"phone":         user.Phone,
		"address":       user.Address,
		"department":    user.Department,
		"position":      user.Position,
		"hire_date":     user.HireDate,
		"is_active":     user.IsActive,
		"last_login_at": user.LastLoginAt,
		"created_at":    user.CreatedAt,
		"updated_at":    user.UpdatedAt,
	}

	c.JSON(http.StatusOK, response)
}

// CreateUser creates a new user
func (uc *UserController) CreateUser(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user already exists
	var existingUser models.User
	if err := uc.db.Where("username = ? OR email = ?", req.Username, req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User with this username or email already exists"})
		return
	}

	// Create new user
	user := models.User{
		Username:   req.Username,
		Email:      req.Email,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		Role:       req.Role,
		Phone:      req.Phone,
		Department: req.Department,
		Position:   req.Position,
		IsActive:   true,
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}
	user.Password = string(hashedPassword)

	if err := uc.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Return user without sensitive information
	response := map[string]interface{}{
		"id":         user.ID,
		"username":   user.Username,
		"email":      user.Email,
		"role":       user.Role,
		"first_name": user.FirstName,
		"last_name":  user.LastName,
		"full_name":  user.GetDisplayName(),
		"phone":      user.Phone,
		"department": user.Department,
		"position":   user.Position,
		"is_active":  user.IsActive,
		"created_at": user.CreatedAt,
		"updated_at": user.UpdatedAt,
	}

	c.JSON(http.StatusCreated, response)
}

// UpdateUser updates an existing user
func (uc *UserController) UpdateUser(c *gin.Context) {
	id := c.Param("id")
	userID, err := strconv.Atoi(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var user models.User
	if err := uc.db.First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user"})
		return
	}

	var updateData map[string]interface{}
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Remove sensitive fields that shouldn't be updated directly
	delete(updateData, "id")
	delete(updateData, "created_at")
	delete(updateData, "updated_at")

	if user.Role == models.RoleAdmin && user.IsActive {
		nextRole := user.Role
		if role, exists := updateData["role"]; exists {
			if roleStr, ok := role.(string); ok && roleStr != "" {
				nextRole = roleStr
			}
		}

		nextIsActive := user.IsActive
		if isActive, exists := updateData["is_active"]; exists {
			if isActiveBool, ok := isActive.(bool); ok {
				nextIsActive = isActiveBool
			}
		}

		if nextRole != models.RoleAdmin || !nextIsActive {
			adminCount, countErr := uc.activeAdminCount()
			if countErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate admin safeguards"})
				return
			}

			if adminCount <= 1 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot remove or deactivate the last active admin"})
				return
			}
		}
	}

	// Handle password update separately if provided
	if password, exists := updateData["password"]; exists {
		if passwordStr, ok := password.(string); ok && passwordStr != "" {
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(passwordStr), bcrypt.DefaultCost)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
				return
			}
			updateData["password"] = string(hashedPassword)
		} else {
			delete(updateData, "password")
		}
	}

	if err := uc.db.Model(&user).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	// Fetch updated user
	if err := uc.db.Select("id, username, email, role, first_name, last_name, phone, address, department, position, hire_date, is_active, last_login_at, created_at, updated_at").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch updated user"})
		return
	}

	// Calculate full name
	fullName := ""
	if user.FirstName != "" {
		fullName = user.FirstName
	}
	if user.LastName != "" {
		if fullName != "" {
			fullName += " " + user.LastName
		} else {
			fullName = user.LastName
		}
	}
	if fullName == "" {
		fullName = user.Username
	}

	response := map[string]interface{}{
		"id":            user.ID,
		"username":      user.Username,
		"email":         user.Email,
		"role":          user.Role,
		"first_name":    user.FirstName,
		"last_name":     user.LastName,
		"full_name":     fullName,
		"phone":         user.Phone,
		"address":       user.Address,
		"department":    user.Department,
		"position":      user.Position,
		"hire_date":     user.HireDate,
		"is_active":     user.IsActive,
		"last_login_at": user.LastLoginAt,
		"created_at":    user.CreatedAt,
		"updated_at":    user.UpdatedAt,
	}

	c.JSON(http.StatusOK, response)
}

// DeleteUser soft deletes a user
func (uc *UserController) DeleteUser(c *gin.Context) {
	id := c.Param("id")
	userID, err := strconv.Atoi(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var user models.User
	if err := uc.db.First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user"})
		return
	}

	if currentUserID, ok := getCurrentUserID(c); ok && currentUserID == user.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You cannot delete your own account"})
		return
	}

	if user.Role == models.RoleAdmin && user.IsActive {
		adminCount, countErr := uc.activeAdminCount()
		if countErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate admin safeguards"})
			return
		}

		if adminCount <= 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete the last active admin"})
			return
		}
	}

	// Soft delete the user
	if err := uc.db.Delete(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}
