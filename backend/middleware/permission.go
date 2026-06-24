package middleware

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"log"
	"net/http"
	"strconv"
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
			// Manage Permissions is authoritative whenever a module record exists.
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

		hasPermission = modulePermissionAllowsAction(basePermission, action)
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

func (pm *PermissionMiddleware) CanEditPurchaseOrFinanceStaffDraft() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleInterface, _ := c.Get("role")
		role := ""
		if roleStr, ok := roleInterface.(string); ok {
			role = roleStr
		}

		if models.IsFinanceStaffRole(role) {
			purchaseID, err := strconv.ParseUint(c.Param("id"), 10, 32)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase ID"})
				c.Abort()
				return
			}

			var purchase models.Purchase
			err = pm.db.
				Select("id", "status").
				Where("id = ?", uint(purchaseID)).
				First(&purchase).
				Error
			if err == nil && purchase.Status == models.PurchaseStatusDraft {
				c.Next()
				return
			}
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Purchase not found"})
				c.Abort()
				return
			}
			if err != nil {
				log.Printf("[PERMISSION DEBUG] Failed to check purchase draft access for finance staff: %v", err)
			}
		}

		pm.CanEdit("purchases")(c)
	}
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
		roleInterface, _ := c.Get("role")
		role := ""
		if roleStr, ok := roleInterface.(string); ok {
			role = roleStr
		}

		if models.IsFinanceStaffRole(role) || models.IsAccountingRole(role) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":               "Purchase approval must be handled by Finance Manager or Director workflow step",
				"required_permission": "approve",
				"module":              "purchases",
				"user_role":           role,
			})
			c.Abort()
			return
		}

		pm.CanApprove("purchases")(c)
	}
}

func (pm *PermissionMiddleware) CanReviewPurchaseRequests() gin.HandlerFunc {
	return pm.CanApprove("purchase_requests")
}
