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

	log.Println("Permissions table migrated successfully")

	// Initialize default permissions for existing users
	var users []models.User
	if err := db.Find(&users).Error; err != nil {
		return err
	}

	modules := []string{"accounts", "products", "contacts", "assets", "sales", "purchases", "payments", "cash_bank", "journal", "reports", "users", "settings", "purchase_requests", "projects"}

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
