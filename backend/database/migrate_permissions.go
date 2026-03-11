package database

import (
	"app-sistem-akuntansi/models"
	"log"

	"gorm.io/gorm"
)

// MigratePermissions creates permissions table and initializes default permissions
func MigratePermissions(db *gorm.DB) error {
	// Auto migrate ModulePermissionRecord model
	if err := db.AutoMigrate(&models.ModulePermissionRecord{}); err != nil {
		return err
	}

	// Ensure proper UNIQUE INDEX on (user_id, module) for UPSERT support.
	// 1) Clean up soft-deleted records that would conflict
	db.Unscoped().Where("deleted_at IS NOT NULL").Delete(&models.ModulePermissionRecord{})

	// 2) Drop partial indexes that can't be used by ON CONFLICT
	db.Exec("DROP INDEX IF EXISTS idx_module_permission_user_module_active")

	// 3) Create unique index if it doesn't exist
	var indexExists bool
	db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE tablename = 'module_permissions'
			AND indexname = 'idx_module_permissions_user_module'
		)
	`).Scan(&indexExists)

	if !indexExists {
		if err := db.Exec("CREATE UNIQUE INDEX idx_module_permissions_user_module ON module_permissions (user_id, module)").Error; err != nil {
			log.Printf("Warning: Failed to create unique index on module_permissions: %v", err)
		} else {
			log.Println("✅ Created unique index idx_module_permissions_user_module")
		}
	}

	log.Println("Permissions table migrated successfully")

	// Initialize default permissions for existing users
	var users []models.User
	if err := db.Find(&users).Error; err != nil {
		return err
	}

	modules := []string{"accounts", "products", "contacts", "assets", "sales", "purchases", "payments", "cash_bank", "journal", "reports", "users", "settings", "purchase_requests", "projects", "boms"}

	for _, user := range users {
		// Get default permissions based on role
		defaultPerms := models.GetDefaultPermissions(user.Role)

		// Create permission records for each module if they don't exist
		for _, module := range modules {
			var count int64
			db.Model(&models.ModulePermissionRecord{}).Where("user_id = ? AND module = ?", user.ID, module).Count(&count)

			if count == 0 {
				perm := defaultPerms[module]
				if perm != nil {
					permission := models.ModulePermissionRecord{
						UserID:     user.ID,
						Module:     module,
						CanView:    perm.CanView,
						CanCreate:  perm.CanCreate,
						CanEdit:    perm.CanEdit,
						CanDelete:  perm.CanDelete,
						CanApprove: perm.CanApprove,
						CanExport:  perm.CanExport,
						CanMenu:    perm.CanMenu,
					}

					if err := db.Create(&permission).Error; err != nil {
						log.Printf("Error creating permission for user %d module %s: %v", user.ID, module, err)
					} else {
						log.Printf("Added missing permission for user: %s (role: %s, module: %s)", user.Username, user.Role, module)
					}
				}
			}
		}
	}

	return nil
}
