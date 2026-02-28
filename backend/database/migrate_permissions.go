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

	// Ensure proper UNIQUE CONSTRAINT on (user_id, module) for UPSERT support.
	// 1) Clean up soft-deleted records that would conflict
	db.Exec("DELETE FROM module_permission_records WHERE deleted_at IS NOT NULL")

	// 2) Drop partial indexes that can't be used by ON CONFLICT
	db.Exec("DROP INDEX IF EXISTS idx_module_permission_user_module_active")

	// 3) Create unique constraint if it doesn't exist
	var constraintExists bool
	db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.table_constraints
			WHERE table_name = 'module_permission_records'
			AND constraint_type = 'UNIQUE'
			AND constraint_name = 'uq_module_permission_user_module'
		)
	`).Scan(&constraintExists)

	if !constraintExists {
		if err := db.Exec("ALTER TABLE module_permission_records ADD CONSTRAINT uq_module_permission_user_module UNIQUE (user_id, module)").Error; err != nil {
			log.Printf("Warning: Failed to add unique constraint on module_permission_records: %v", err)
			// Try to handle duplicate records first, then retry
			db.Exec(`
				DELETE FROM module_permission_records a USING module_permission_records b
				WHERE a.id < b.id AND a.user_id = b.user_id AND a.module = b.module
			`)
			if err2 := db.Exec("ALTER TABLE module_permission_records ADD CONSTRAINT uq_module_permission_user_module UNIQUE (user_id, module)").Error; err2 != nil {
				log.Printf("Error: Still failed to add unique constraint: %v", err2)
			} else {
				log.Println("✅ Created unique constraint uq_module_permission_user_module (after dedup)")
			}
		} else {
			log.Println("✅ Created unique constraint uq_module_permission_user_module")
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
