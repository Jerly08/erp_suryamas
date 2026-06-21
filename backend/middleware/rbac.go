package middleware

import (
	"net/http"
	"strings"

	"app-sistem-akuntansi/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RBACManager struct {
	DB *gorm.DB
}

func NewRBACManager(db *gorm.DB) *RBACManager {
	return &RBACManager{DB: db}
}

// RequireRole checks if user has one of the required roles
func (rm *RBACManager) RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User role not found",
				"code":  "ROLE_NOT_FOUND",
			})
			c.Abort()
			return
		}

		roleStr := userRole.(string)
		for _, role := range roles {
			if roleStr == role {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"error":          "Insufficient role permissions",
			"code":           "INSUFFICIENT_ROLE",
			"required_roles": roles,
			"user_role":      roleStr,
		})
		c.Abort()
	}
}

// RequirePermission checks if user has specific permission for a resource.
// Uses per-user ModulePermissionRecord first, then falls back to role-based defaults.
// Does NOT depend on the legacy role_permissions/permissions tables.
func (rm *RBACManager) RequirePermission(resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User role not found",
				"code":  "ROLE_NOT_FOUND",
			})
			c.Abort()
			return
		}

		// Get user_id for per-user permission override
		var userID uint
		if v, ok := c.Get("user_id"); ok {
			switch t := v.(type) {
			case float64:
				userID = uint(t)
			case int:
				userID = uint(t)
			case uint:
				userID = t
			}
		}

		roleStr := userRole.(string)
		permissionName := resource + ":" + action

		// userHasModulePermission checks per-user module_permission_records first,
		// then falls back to GetDefaultPermissions(role) — covers all cases.
		hasPermission, err := rm.userHasModulePermission(userID, roleStr, resource, action)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Error checking user permissions",
				"code":  "PERMISSION_CHECK_ERROR",
			})
			c.Abort()
			return
		}

		if !hasPermission {
			c.JSON(http.StatusForbidden, gin.H{
				"error":               "Insufficient permissions",
				"code":                "INSUFFICIENT_PERMISSION",
				"required_permission": permissionName,
				"user_role":           roleStr,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// AutoPermission automatically determines the required action based on the HTTP method
func (rm *RBACManager) AutoPermission(resource string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var action string
		switch c.Request.Method {
		case http.MethodGet:
			action = "view"
		case http.MethodPost:
			action = "create"
		case http.MethodPut, http.MethodPatch:
			action = "edit"
		case http.MethodDelete:
			action = "delete"
		default:
			action = "view" // fallback
		}

		// Delegate to RequirePermission
		rm.RequirePermission(resource, action)(c)
	}
}

// RequirePermissions checks if user has all specified permissions
func (rm *RBACManager) RequirePermissions(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User role not found",
				"code":  "ROLE_NOT_FOUND",
			})
			c.Abort()
			return
		}

		// Get userID for per-user Manage Permission override
		var userID uint
		if v, ok := c.Get("user_id"); ok {
			switch t := v.(type) {
			case float64:
				userID = uint(t)
			case int:
				userID = uint(t)
			case uint:
				userID = t
			}
		}

		roleStr := userRole.(string)

		for _, permission := range permissions {
			// 1) If user has module permission record, use it
			if userID != 0 {
				parts := strings.Split(permission, ":")
				if len(parts) == 2 {
					if exists, err := rm.userModuleRecordExists(userID, parts[0]); err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Error checking user permissions", "code": "PERMISSION_CHECK_ERROR"})
						c.Abort()
						return
					} else if exists {
						ok, err := rm.userHasModulePermission(userID, roleStr, parts[0], parts[1])
						if err != nil {
							c.JSON(http.StatusInternalServerError, gin.H{"error": "Error checking user permissions", "code": "PERMISSION_CHECK_ERROR"})
							c.Abort()
							return
						}
						if !ok {
							c.JSON(http.StatusForbidden, gin.H{
								"error":                "Insufficient permissions",
								"code":                 "INSUFFICIENT_PERMISSION",
								"required_permissions": permissions,
								"missing_permission":   permission,
								"user_role":            roleStr,
							})
							c.Abort()
							return
						}
						// managed permission allowed this one, continue to next required perm
						continue
					}
				}
			}

			// 2) Role permission table check
			hasPermission, err := rm.hasPermission(roleStr, permission)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Error checking permissions",
					"code":  "PERMISSION_CHECK_ERROR",
				})
				c.Abort()
				return
			}

			// 3) If role table denies, fallback to role-based module defaults
			if !hasPermission {
				parts := strings.Split(permission, ":")
				if len(parts) == 2 {
					ok, err := rm.userHasModulePermission(userID, roleStr, parts[0], parts[1])
					if err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{
							"error": "Error checking user permissions",
							"code":  "PERMISSION_CHECK_ERROR",
						})
						c.Abort()
						return
					}
					hasPermission = ok
				}
			}

			if !hasPermission {
				c.JSON(http.StatusForbidden, gin.H{
					"error":                "Insufficient permissions",
					"code":                 "INSUFFICIENT_PERMISSION",
					"required_permissions": permissions,
					"missing_permission":   permission,
					"user_role":            roleStr,
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// RequireAnyPermission checks if user has at least one of the specified permissions
func (rm *RBACManager) RequireAnyPermission(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User role not found",
				"code":  "ROLE_NOT_FOUND",
			})
			c.Abort()
			return
		}

		// Get userID for per-user Manage Permission override
		var userID uint
		if v, ok := c.Get("user_id"); ok {
			switch t := v.(type) {
			case float64:
				userID = uint(t)
			case int:
				userID = uint(t)
			case uint:
				userID = t
			}
		}

		roleStr := userRole.(string)

		for _, permission := range permissions {
			parts := strings.Split(permission, ":")

			// A Manage Permissions record is authoritative, including an explicit
			// denial. Do not let the role table grant the same permission again.
			if userID != 0 && len(parts) == 2 {
				exists, err := rm.userModuleRecordExists(userID, parts[0])
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Error checking user permissions", "code": "PERMISSION_CHECK_ERROR"})
					c.Abort()
					return
				}
				if exists {
					allowed, err := rm.userHasModulePermission(userID, roleStr, parts[0], parts[1])
					if err != nil {
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Error checking user permissions", "code": "PERMISSION_CHECK_ERROR"})
						c.Abort()
						return
					}
					if allowed {
						c.Next()
						return
					}
					continue
				}
			}

			// No per-user record: check the legacy role table, then role defaults.
			hasPermission, err := rm.hasPermission(roleStr, permission)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error checking permissions", "code": "PERMISSION_CHECK_ERROR"})
				c.Abort()
				return
			}

			if hasPermission {
				c.Next()
				return
			}

			// Fallback to role-based module defaults.
			if len(parts) == 2 {
				ok, err := rm.userHasModulePermission(userID, roleStr, parts[0], parts[1])
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Error checking user permissions", "code": "PERMISSION_CHECK_ERROR"})
					c.Abort()
					return
				}
				if ok {
					c.Next()
					return
				}
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"error":           "Insufficient permissions",
			"code":            "INSUFFICIENT_PERMISSION",
			"required_any_of": permissions,
			"user_role":       roleStr,
		})
		c.Abort()
	}
}

// RequireOwnershipOrRole checks if user owns the resource or has required role
func (rm *RBACManager) RequireOwnershipOrRole(userIDField string, roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User ID not found",
				"code":  "USER_ID_NOT_FOUND",
			})
			c.Abort()
			return
		}

		userRole, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User role not found",
				"code":  "ROLE_NOT_FOUND",
			})
			c.Abort()
			return
		}

		currentUserID := userID.(uint)
		roleStr := userRole.(string)

		// Check if user has one of the required roles
		for _, role := range roles {
			if roleStr == role {
				c.Next()
				return
			}
		}

		// Check ownership - get resource user ID from request
		resourceUserIDStr := c.Param(userIDField)
		if resourceUserIDStr == "" {
			resourceUserIDStr = c.Query(userIDField)
		}

		if resourceUserIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Resource user ID not provided",
				"code":  "RESOURCE_USER_ID_MISSING",
			})
			c.Abort()
			return
		}

		// Convert string to uint (simple conversion for demo)
		var resourceUserID uint
		if resourceUserIDStr != "" {
			// In production, use proper conversion with error handling
			resourceUserID = uint(parseUint(resourceUserIDStr))
		}

		if currentUserID == resourceUserID {
			c.Next()
			return
		}

		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied: not owner and insufficient role",
			"code":  "ACCESS_DENIED",
		})
		c.Abort()
	}
}

// AdminOnly restricts access to admin users only
func (rm *RBACManager) AdminOnly() gin.HandlerFunc {
	return rm.RequireRole(models.RoleAdmin)
}

// FinanceOrAdmin restricts access to finance users and admins
func (rm *RBACManager) FinanceOrAdmin() gin.HandlerFunc {
	return rm.RequireRole(models.RoleAdmin, models.RoleFinance)
}

// DirectorOrAdmin restricts access to director and admin users
func (rm *RBACManager) DirectorOrAdmin() gin.HandlerFunc {
	return rm.RequireRole(models.RoleAdmin, models.RoleDirector)
}

// InventoryManagerOrAdmin restricts access to inventory manager and admin users
func (rm *RBACManager) InventoryManagerOrAdmin() gin.HandlerFunc {
	return rm.RequireRole(models.RoleAdmin, models.RoleInventoryManager)
}

// Custom permission checkers for specific resources

// CanReadFinancialData checks if user can read financial data
func (rm *RBACManager) CanReadFinancialData() gin.HandlerFunc {
	return rm.RequireAnyPermission(
		"accounts:read",
		"transactions:read",
		"reports:read",
	)
}

// CanModifyFinancialData checks if user can modify financial data
func (rm *RBACManager) CanModifyFinancialData() gin.HandlerFunc {
	return rm.RequireAnyPermission(
		"accounts:update",
		"transactions:create",
		"transactions:update",
	)
}

// CanAccessReports checks if user can access reports
func (rm *RBACManager) CanAccessReports() gin.HandlerFunc {
	return rm.RequireRole(
		models.RoleAdmin,
		models.RoleDirector,
		models.RoleFinance,
	)
}

// CanManageUsers checks if user can manage other users
func (rm *RBACManager) CanManageUsers() gin.HandlerFunc {
	return rm.RequirePermission("users", "manage")
}

// CanManageInventory checks if user can manage inventory
func (rm *RBACManager) CanManageInventory() gin.HandlerFunc {
	return rm.RequireRole(
		models.RoleAdmin,
		models.RoleInventoryManager,
	)
}

// Helper functions

func (rm *RBACManager) hasPermission(role, permissionName string) (bool, error) {
	var count int64

	// Split permission name to get resource and action
	parts := strings.Split(permissionName, ":")
	if len(parts) != 2 {
		return false, nil
	}

	resource := parts[0]
	action := parts[1]

	// Check if role has the specific permission in role_permissions table.
	// If the table doesn't exist yet, gracefully return false (no permission)
	// so the caller can fall back to userHasModulePermission() defaults.
	err := rm.DB.Table("role_permissions rp").
		Joins("JOIN permissions p ON rp.permission_id = p.id").
		Where("rp.role = ? AND p.resource = ? AND p.action = ?", role, resource, action).
		Count(&count).Error

	if err != nil {
		// Table may not exist — treat as "no permission found via role table",
		// let the caller fall back to module-level defaults.
		return false, nil
	}

	return count > 0, nil
}

// userModuleRecordExists returns whether a per-user module permission record exists
func (rm *RBACManager) userModuleRecordExists(userID uint, resource string) (bool, error) {
	var count int64
	if err := rm.DB.Model(&models.ModulePermissionRecord{}).Where("user_id = ? AND module = ?", userID, resource).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// userHasModulePermission checks per-user module permissions (Settings -> Manage Permission),
// falling back to role-based defaults when no custom record exists.
func (rm *RBACManager) userHasModulePermission(userID uint, role string, resource string, action string) (bool, error) {
	var rec models.ModulePermissionRecord
	if err := rm.DB.Where("user_id = ? AND module = ?", userID, resource).First(&rec).Error; err == nil {
		switch action {
		case "read", "view":
			return rec.CanView, nil
		case "create":
			return rec.CanCreate, nil
		case "update", "edit":
			return rec.CanEdit, nil
		case "delete":
			return rec.CanDelete, nil
		case "approve":
			return rec.CanApprove, nil
		case "export":
			return rec.CanExport, nil
		}
		return false, nil
	} else if err == gorm.ErrRecordNotFound {
		// Fallback to role defaults if no custom record
		perms := models.GetDefaultPermissions(role)
		if modPerm, ok := perms[resource]; ok {
			switch action {
			case "read", "view":
				return modPerm.CanView, nil
			case "create":
				return modPerm.CanCreate, nil
			case "update", "edit":
				return modPerm.CanEdit, nil
			case "delete":
				return modPerm.CanDelete, nil
			case "approve":
				return modPerm.CanApprove, nil
			case "export":
				return modPerm.CanExport, nil
			}
		}
		return false, nil
	} else {
		return false, err
	}
}

// Simplified uint parsing (replace with proper implementation)
func parseUint(s string) int {
	// Simple conversion - in production use strconv.ParseUint with error handling
	result := 0
	for _, char := range s {
		if char >= '0' && char <= '9' {
			result = result*10 + int(char-'0')
		}
	}
	return result
}

// Permission constants for easier reference
const (
	// User permissions
	PermissionUsersRead   = "users:read"
	PermissionUsersCreate = "users:create"
	PermissionUsersUpdate = "users:update"
	PermissionUsersDelete = "users:delete"
	PermissionUsersManage = "users:manage"

	// Account permissions
	PermissionAccountsRead   = "accounts:read"
	PermissionAccountsCreate = "accounts:create"
	PermissionAccountsUpdate = "accounts:update"
	PermissionAccountsDelete = "accounts:delete"

	// Transaction permissions
	PermissionTransactionsRead   = "transactions:read"
	PermissionTransactionsCreate = "transactions:create"
	PermissionTransactionsUpdate = "transactions:update"
	PermissionTransactionsDelete = "transactions:delete"

	// Product permissions
	PermissionProductsRead   = "products:read"
	PermissionProductsCreate = "products:create"
	PermissionProductsUpdate = "products:update"
	PermissionProductsDelete = "products:delete"

	// Sales permissions
	PermissionSalesRead   = "sales:read"
	PermissionSalesCreate = "sales:create"
	PermissionSalesUpdate = "sales:update"
	PermissionSalesDelete = "sales:delete"

	// Purchase permissions
	PermissionPurchasesRead   = "purchases:read"
	PermissionPurchasesCreate = "purchases:create"
	PermissionPurchasesUpdate = "purchases:update"
	PermissionPurchasesDelete = "purchases:delete"

	// Report permissions
	PermissionReportsRead   = "reports:read"
	PermissionReportsCreate = "reports:create"
	PermissionReportsUpdate = "reports:update"
	PermissionReportsDelete = "reports:delete"

	// Budget permissions
	PermissionBudgetsRead   = "budgets:read"
	PermissionBudgetsCreate = "budgets:create"
	PermissionBudgetsUpdate = "budgets:update"
	PermissionBudgetsDelete = "budgets:delete"

	// Asset permissions
	PermissionAssetsRead   = "assets:read"
	PermissionAssetsCreate = "assets:create"
	PermissionAssetsUpdate = "assets:update"
	PermissionAssetsDelete = "assets:delete"

	// Contact permissions
	PermissionContactsRead   = "contacts:read"
	PermissionContactsCreate = "contacts:create"
	PermissionContactsUpdate = "contacts:update"
	PermissionContactsDelete = "contacts:delete"
)
