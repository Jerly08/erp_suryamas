package controllers

import (
	"app-sistem-akuntansi/models"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PermissionController struct {
	db *gorm.DB
}

func NewPermissionController(db *gorm.DB) *PermissionController {
	return &PermissionController{db: db}
}

// GetUserPermissions retrieves permissions for a specific user
func (pc *PermissionController) GetUserPermissions(c *gin.Context) {
	userID := c.Param("userId")

	id, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get user details
	var user models.User
	if err := pc.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Get user permissions from database
	var permissions []models.ModulePermissionRecord
	if err := pc.db.Where("user_id = ?", id).Find(&permissions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch permissions"})
		return
	}

	// Start with default permissions based on role
	permMap := models.GetDefaultPermissions(user.Role)

	// Override with database permissions if they exist
	for _, perm := range permissions {
		permMap[perm.Module] = &models.ModulePermission{
			CanView:    perm.CanView,
			CanCreate:  perm.CanCreate,
			CanEdit:    perm.CanEdit,
			CanDelete:  perm.CanDelete,
			CanApprove: perm.CanApprove,
			CanExport:  perm.CanExport,
			CanMenu:    perm.CanMenu,
		}
	}

	response := models.UserPermission{
		UserID:      uint(id),
		Username:    user.Username,
		Email:       user.Email,
		Role:        user.Role,
		Permissions: permMap,
	}

	c.JSON(http.StatusOK, response)
}

// UpdateUserPermissions updates permissions for a specific user
func (pc *PermissionController) UpdateUserPermissions(c *gin.Context) {
	userID := c.Param("userId")

	id, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Parse request body
	var request struct {
		Permissions map[string]*models.ModulePermission `json:"permissions"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify user exists
	var user models.User
	if err := pc.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Use raw SQL UPSERT to handle all constraint cases atomically.
	// This avoids any soft-delete / unique-index conflicts entirely.
	for module, perm := range request.Permissions {
		err := pc.db.Exec(`
			INSERT INTO module_permission_records
				(user_id, module, can_view, can_create, can_edit, can_delete, can_approve, can_export, can_menu, created_at, updated_at, deleted_at)
			VALUES
				(?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), NULL)
			ON CONFLICT (user_id, module)
			DO UPDATE SET
				can_view    = EXCLUDED.can_view,
				can_create  = EXCLUDED.can_create,
				can_edit    = EXCLUDED.can_edit,
				can_delete  = EXCLUDED.can_delete,
				can_approve = EXCLUDED.can_approve,
				can_export  = EXCLUDED.can_export,
				can_menu    = EXCLUDED.can_menu,
				updated_at  = NOW(),
				deleted_at  = NULL
		`, id, module,
			perm.CanView, perm.CanCreate, perm.CanEdit, perm.CanDelete,
			perm.CanApprove, perm.CanExport, perm.CanMenu,
		).Error

		if err != nil {
			log.Printf("[PERMISSION ERROR] Failed to upsert permission for user %d, module %s: %v", id, module, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update permissions"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Permissions updated successfully"})
}

// GetAllUsersPermissions retrieves permissions for all users
func (pc *PermissionController) GetAllUsersPermissions(c *gin.Context) {
	// Get all users
	var users []models.User
	if err := pc.db.Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}

	var response []models.UserPermission

	for _, user := range users {
		// Get user permissions from database
		var permissions []models.ModulePermissionRecord
		pc.db.Where("user_id = ?", user.ID).Find(&permissions)

		// Start with default permissions based on role
		permMap := models.GetDefaultPermissions(user.Role)

		// Override with database permissions if they exist
		for _, perm := range permissions {
			permMap[perm.Module] = &models.ModulePermission{
				CanView:    perm.CanView,
				CanCreate:  perm.CanCreate,
				CanEdit:    perm.CanEdit,
				CanDelete:  perm.CanDelete,
				CanApprove: perm.CanApprove,
				CanExport:  perm.CanExport,
				CanMenu:    perm.CanMenu,
			}
		}

		userPerm := models.UserPermission{
			UserID:      user.ID,
			Username:    user.Username,
			Email:       user.Email,
			Role:        user.Role,
			Permissions: permMap,
		}

		response = append(response, userPerm)
	}

	c.JSON(http.StatusOK, response)
}

// ResetToDefaultPermissions resets user permissions to default based on their role
func (pc *PermissionController) ResetToDefaultPermissions(c *gin.Context) {
	userID := c.Param("userId")

	id, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get user details
	var user models.User
	if err := pc.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Get default permissions based on role
	defaultPerms := models.GetDefaultPermissions(user.Role)

	// Start transaction
	tx := pc.db.Begin()

	// Hard delete existing permissions (Unscoped) to avoid unique constraint violation with soft-deleted records
	if err := tx.Unscoped().Where("user_id = ?", id).Delete(&models.ModulePermissionRecord{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset permissions"})
		return
	}

	// Create default permissions
	for module, perm := range defaultPerms {
		permission := models.ModulePermissionRecord{
			UserID:     uint(id),
			Module:     module,
			CanView:    perm.CanView,
			CanCreate:  perm.CanCreate,
			CanEdit:    perm.CanEdit,
			CanDelete:  perm.CanDelete,
			CanApprove: perm.CanApprove,
			CanExport:  perm.CanExport,
			CanMenu:    perm.CanMenu,
		}

		if err := tx.Create(&permission).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create default permissions"})
			return
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save permissions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Permissions reset to default successfully"})
}

// CheckUserPermission checks if a user has a specific permission for a module
// GetMyPermissions retrieves permissions for the current authenticated user
func (pc *PermissionController) GetMyPermissions(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Convert to uint
	var id uint
	switch v := userID.(type) {
	case float64:
		id = uint(v)
	case int:
		id = uint(v)
	case uint:
		id = v
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get user details
	var user models.User
	if err := pc.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Get user permissions from database
	var permissions []models.ModulePermissionRecord
	if err := pc.db.Where("user_id = ?", id).Find(&permissions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch permissions"})
		return
	}

	// Start with default permissions based on role
	permMap := models.GetDefaultPermissions(user.Role)

	// Override with database permissions if they exist
	for _, perm := range permissions {
		permMap[perm.Module] = &models.ModulePermission{
			CanView:    perm.CanView,
			CanCreate:  perm.CanCreate,
			CanEdit:    perm.CanEdit,
			CanDelete:  perm.CanDelete,
			CanApprove: perm.CanApprove,
			CanExport:  perm.CanExport,
			CanMenu:    perm.CanMenu,
		}
	}

	response := models.UserPermission{
		UserID:      uint(id),
		Username:    user.Username,
		Email:       user.Email,
		Role:        user.Role,
		Permissions: permMap,
	}

	c.JSON(http.StatusOK, response)
}

func (pc *PermissionController) CheckUserPermission(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	module := c.Query("module")
	action := c.Query("action") // view, create, edit, delete, approve, export

	if module == "" || action == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Module and action are required"})
		return
	}

	// Get permission
	var permission models.ModulePermissionRecord
	err := pc.db.Where("user_id = ? AND module = ?", userID, module).First(&permission).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// No permission record, check default based on role
			var user models.User
			if err := pc.db.First(&user, userID).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user"})
				return
			}

			defaultPerms := models.GetDefaultPermissions(user.Role)
			if modPerm, ok := defaultPerms[module]; ok {
				permission = models.ModulePermissionRecord{
					Module:     module,
					CanView:    modPerm.CanView,
					CanCreate:  modPerm.CanCreate,
					CanEdit:    modPerm.CanEdit,
					CanDelete:  modPerm.CanDelete,
					CanApprove: modPerm.CanApprove,
					CanExport:  modPerm.CanExport,
					CanMenu:    modPerm.CanMenu,
				}
			} else {
				c.JSON(http.StatusOK, gin.H{"has_permission": false})
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permission"})
			return
		}
	}

	// Check specific action
	hasPermission := false
	switch action {
	case "view":
		hasPermission = permission.CanView
	case "create":
		hasPermission = permission.CanCreate
	case "edit":
		hasPermission = permission.CanEdit
	case "delete":
		hasPermission = permission.CanDelete
	case "approve":
		hasPermission = permission.CanApprove
	case "export":
		hasPermission = permission.CanExport
	case "menu":
		hasPermission = permission.CanMenu
	}

	c.JSON(http.StatusOK, gin.H{
		"has_permission": hasPermission,
		"module":         module,
		"action":         action,
	})
}
