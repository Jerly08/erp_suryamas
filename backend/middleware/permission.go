package middleware

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"log"
	"net/http"
	"time"
)

type PermissionMiddleware struct {
	db    *gorm.DB
	cache *PermissionCache
}

func NewPermissionMiddleware(db *gorm.DB) *PermissionMiddleware {
	return &PermissionMiddleware{
		db:    db,
		cache: NewPermissionCache(5 * time.Minute), // Cache for 5 minutes
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

		// Check cache first
		if cachedResult, found := pm.cache.Get(userID, module, action); found {
			log.Printf("[PERMISSION CACHE] Cache hit for user %d, module %s, action %s: %v", userID, module, action, cachedResult)
			if !cachedResult {
				c.JSON(http.StatusForbidden, gin.H{
					"error":               "You don't have permission to " + action + " " + module,
					"required_permission": action,
					"module":              module,
					"user_role":           role,
				})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		// Check if user has specific permission in database
		var permission models.ModulePermissionRecord
		err := pm.db.Where("user_id = ? AND module = ?", userID, module).First(&permission).Error

		hasPermission := false

		if err == nil {
			// Permission record found, check specific action
			log.Printf("[PERMISSION DEBUG] Found custom permission record for user %d, module %s", userID, module)
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
			default:
				hasPermission = false
			}
			log.Printf("[PERMISSION DEBUG] Custom permission result: %v", hasPermission)
		} else if err == gorm.ErrRecordNotFound {
			// No custom permission, use default based on role
			log.Printf("[PERMISSION DEBUG] No custom permission found, using default for role: %s", role)
			defaultPerms := models.GetDefaultPermissions(role)
			if modPerm, ok := defaultPerms[module]; ok {
				log.Printf("[PERMISSION DEBUG] Found default permissions for module %s", module)
				switch action {
				case "view":
					hasPermission = modPerm.CanView
				case "create":
					hasPermission = modPerm.CanCreate
				case "edit":
					hasPermission = modPerm.CanEdit
				case "delete":
					hasPermission = modPerm.CanDelete
				case "approve":
					hasPermission = modPerm.CanApprove
				case "export":
					hasPermission = modPerm.CanExport
				}
				log.Printf("[PERMISSION DEBUG] Default permission result: %v", hasPermission)
			} else {
				log.Printf("[PERMISSION DEBUG] No default permissions found for module %s", module)
			}
		} else {
			log.Printf("[PERMISSION DEBUG] Database error: %v", err)
		}

		log.Printf("[PERMISSION DEBUG] Final result - hasPermission: %v", hasPermission)

		// Store result in cache
		pm.cache.Set(userID, module, action, hasPermission)

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

func (pm *PermissionMiddleware) CanReviewPurchaseRequests() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")
		if models.CanReviewPurchaseRequests(role) {
			c.Next()
			return
		}

		pm.CheckModulePermission("purchase_requests", "approve")(c)
	}
}
