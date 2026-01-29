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
	
	modules := []string{"accounts", "products", "contacts", "assets", "sales", "purchases", "payments", "cash_bank", "journal", "reports", "users", "settings"}
	
	for _, user := range users {
		// Check if user already has permissions
		var count int64
		db.Model(&models.ModulePermissionRecord{}).Where("user_id = ?", user.ID).Count(&count)
		
		if count == 0 {
			// Get default permissions based on role
			defaultPerms := models.GetDefaultPermissions(user.Role)
			
			// Create permission records for each module
			for _, module := range modules {
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
					}
				}
			}
			
			log.Printf("Default permissions created for user: %s (role: %s)", user.Username, user.Role)
		}
	}
	
	return nil
}
