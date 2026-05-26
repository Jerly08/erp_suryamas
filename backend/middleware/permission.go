package middleware

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"log"
	"net/http"
)

type PermissionMiddleware struct {
	db *gorm.DB
}

func NewPermissionMiddleware(db *gorm.DB) *PermissionMiddleware {
	return &PermissionMiddleware{
		db: db,
	}
}

func modulePermissionAllowsAction(permission *models.ModulePermission, action string) bool {
	if permission == nil {
		return false
	}

	switch action {
	case "view":
		return permission.CanView
	case "create":
		return permission.CanCreate
	case "edit":
		return permission.CanEdit
	case "delete":
		return permission.CanDelete
	case "approve":
		return permission.CanApprove
	case "export":
		return permission.CanExport
	default:
		return false
	}
}

// CheckModulePermission checks if user has specific permission for a module
func (pm *PermissionMiddleware) CheckModulePermission(module string, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context (set by JWT middleware)
		userIDInterface, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		// Convert user_id to uint
		var userID uint
		switch v := userIDInterface.(type) {
		case float64:
			userID = uint(v)
		case int:
			userID = uint(v)
		case uint:
			userID = v
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
			c.Abort()
			return
		}

		// Get user role from context
		roleInterface, _ := c.Get("role")
		role := ""
		if roleStr, ok := roleInterface.(string); ok {
			role = roleStr
		}

		// Debug logging
		log.Printf("[PERMISSION DEBUG] UserID: %d, Role: %s, Module: %s, Action: %s", userID, role, module, action)

		// Check if user has specific permission in database
		var permission models.ModulePermissionRecord
		err := pm.db.Where("user_id = ? AND module = ?", userID, module).First(&permission).Error

		hasPermission := false
		var basePermission *models.ModulePermission

		if err == nil {
			// Permission record found, use it as the base before workflow overrides.
			log.Printf("[PERMISSION DEBUG] Found custom permission record for user %d, module %s", userID, module)
			basePermission = &models.ModulePermission{
				CanView:    permission.CanView,
				CanCreate:  permission.CanCreate,
				CanEdit:    permission.CanEdit,
				CanDelete:  permission.CanDelete,
				CanApprove: permission.CanApprove,
				CanExport:  permission.CanExport,
				CanMenu:    permission.CanMenu,
			}
		} else if err == gorm.ErrRecordNotFound {
			// No custom permission, use default based on role
			log.Printf("[PERMISSION DEBUG] No custom permission found, using default for role: %s", role)
			defaultPerms := models.GetDefaultPermissions(role)
			if modPerm, ok := defaultPerms[module]; ok {
				log.Printf("[PERMISSION DEBUG] Found default permissions for module %s", module)
				basePermission = modPerm
			} else {
				log.Printf("[PERMISSION DEBUG] No default permissions found for module %s", module)
			}
		} else {
			log.Printf("[PERMISSION DEBUG] Database error: %v", err)
		}

		effectivePermission := models.EnforceWorkflowManagedModulePermission(role, module, basePermission)
		hasPermission = modulePermissionAllowsAction(effectivePermission, action)
		log.Printf("[PERMISSION DEBUG] Final result - hasPermission: %v", hasPermission)

		if !hasPermission {
			log.Printf("[PERMISSION DEBUG] Access denied for user %d, role %s, module %s, action %s", userID, role, module, action)
			c.JSON(http.StatusForbidden, gin.H{
				"error":               "You don't have permission to " + action + " " + module,
				"required_permission": action,
				"module":              module,
				"user_role":           role,
				"debug_info":          fmt.Sprintf("UserID: %d, Role: %s", userID, role),
			})
			c.Abort()
			return
		} else {
			log.Printf("[PERMISSION DEBUG] Access granted for user %d, role %s, module %s, action %s", userID, role, module, action)
		}

		c.Next()
	}
}

// Convenience methods for common permissions
func (pm *PermissionMiddleware) CanView(module string) gin.HandlerFunc {
	return pm.CheckModulePermission(module, "view")
}

func (pm *PermissionMiddleware) CanCreate(module string) gin.HandlerFunc {
	return pm.CheckModulePermission(module, "create")
}

func (pm *PermissionMiddleware) CanEdit(module string) gin.HandlerFunc {
	return pm.CheckModulePermission(module, "edit")
}

func (pm *PermissionMiddleware) CanDelete(module string) gin.HandlerFunc {
	return pm.CheckModulePermission(module, "delete")
}

func (pm *PermissionMiddleware) CanApprove(module string) gin.HandlerFunc {
	return pm.CheckModulePermission(module, "approve")
}

func (pm *PermissionMiddleware) CanExport(module string) gin.HandlerFunc {
	return pm.CheckModulePermission(module, "export")
}

func (pm *PermissionMiddleware) CanUsePurchaseApprovalWorkflow() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")
		if role == "" {
			role = c.GetString("user_role")
		}

		roleNorm := models.NormalizeRole(role)
		if models.IsFinanceStaffRole(roleNorm) || models.IsFinanceManagerRole(roleNorm) || roleNorm == models.RoleDirector {
			c.Next()
			return
		}

		c.JSON(http.StatusForbidden, gin.H{
			"error":     "Only Finance Staff, Finance Manager, or Director can act on purchase approval workflow steps",
			"user_role": role,
		})
		c.Abort()
	}
}

func (pm *PermissionMiddleware) CanReviewPurchaseRequests() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")
		if models.CanReviewPurchaseRequests(role) {
			c.Next()
			return
		}

		c.JSON(403, gin.H{
			"error": "Only finance staff, finance manager, or director can review purchase requests",
		})
		c.Abort()
	}
}
