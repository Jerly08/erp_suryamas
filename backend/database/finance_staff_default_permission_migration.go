package database

import (
	"errors"
	"log"

	"app-sistem-akuntansi/models"
	"gorm.io/gorm"
)

// FinanceStaffDefaultPermissionMigration aligns legacy finance staff module
// permissions with the newer operational default. Existing rows are always
// preserved because they may have been configured through Manage Permissions.
func FinanceStaffDefaultPermissionMigration(db *gorm.DB) {
	var users []models.User
	if err := db.Where("LOWER(role) = LOWER(?)", models.RoleFinanceStaff).Find(&users).Error; err != nil {
		log.Printf("Warning: failed to load finance staff users for permission migration: %v", err)
		return
	}

	if len(users) == 0 {
		return
	}

	currentDefaults := models.GetDefaultPermissions(models.RoleFinanceStaff)

	for _, user := range users {
		for module, newPermission := range currentDefaults {
			if newPermission == nil {
				continue
			}

			var record models.ModulePermissionRecord
			err := db.Where("user_id = ? AND module = ?", user.ID, module).First(&record).Error
			switch {
			case errors.Is(err, gorm.ErrRecordNotFound):
				record = models.ModulePermissionRecord{
					UserID:     user.ID,
					Module:     module,
					CanView:    newPermission.CanView,
					CanCreate:  newPermission.CanCreate,
					CanEdit:    newPermission.CanEdit,
					CanDelete:  newPermission.CanDelete,
					CanApprove: newPermission.CanApprove,
					CanExport:  newPermission.CanExport,
					CanMenu:    newPermission.CanMenu,
				}
				if createErr := db.Create(&record).Error; createErr != nil {
					log.Printf("Warning: failed to create finance staff permission for user %s module %s: %v", user.Username, module, createErr)
				}
			case err != nil:
				log.Printf("Warning: failed to load finance staff permission for user %s module %s: %v", user.Username, module, err)
			}
		}
	}

	log.Println("Finance staff default permission migration completed")
}
