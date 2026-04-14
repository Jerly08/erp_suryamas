package database

import (
	"errors"
	"log"

	"app-sistem-akuntansi/models"
	"gorm.io/gorm"
)

func financeStaffPermissionContains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}

	return false
}

func legacyFinanceStaffDefaultPermissions() map[string]*models.ModulePermission {
	modules := []string{"accounts", "products", "contacts", "assets", "prepaid_expenses", "sales", "purchases", "payments", "cash_bank", "journal", "reports", "users", "settings", "purchase_requests", "projects", "boms"}
	menuModules := []string{"accounts", "sales", "purchases", "payments", "cash_bank", "journal", "reports", "purchase_requests", "projects", "boms", "prepaid_expenses"}

	permissions := make(map[string]*models.ModulePermission, len(modules))
	for _, module := range modules {
		switch {
		case financeStaffPermissionContains(menuModules, module):
			canApprove := module == "purchase_requests"
			permissions[module] = &models.ModulePermission{
				CanView:    true,
				CanCreate:  false,
				CanEdit:    false,
				CanDelete:  false,
				CanApprove: canApprove,
				CanExport:  true,
				CanMenu:    true,
			}
		case module == "products" || module == "contacts" || module == "assets":
			permissions[module] = &models.ModulePermission{
				CanView:    true,
				CanCreate:  false,
				CanEdit:    false,
				CanDelete:  false,
				CanApprove: false,
				CanExport:  false,
				CanMenu:    false,
			}
		case module == "users" || module == "settings":
			permissions[module] = &models.ModulePermission{
				CanView:    false,
				CanCreate:  false,
				CanEdit:    false,
				CanDelete:  false,
				CanApprove: false,
				CanExport:  false,
				CanMenu:    false,
			}
		default:
			permissions[module] = &models.ModulePermission{
				CanView:    true,
				CanCreate:  false,
				CanEdit:    false,
				CanDelete:  false,
				CanApprove: false,
				CanExport:  false,
				CanMenu:    false,
			}
		}
	}

	return permissions
}

func modulePermissionMatchesRecord(record models.ModulePermissionRecord, permission *models.ModulePermission) bool {
	if permission == nil {
		return false
	}

	return record.CanView == permission.CanView &&
		record.CanCreate == permission.CanCreate &&
		record.CanEdit == permission.CanEdit &&
		record.CanDelete == permission.CanDelete &&
		record.CanApprove == permission.CanApprove &&
		record.CanExport == permission.CanExport &&
		record.CanMenu == permission.CanMenu
}

func applyModulePermissionToRecord(db *gorm.DB, record *models.ModulePermissionRecord, permission *models.ModulePermission) error {
	if record == nil || permission == nil {
		return nil
	}

	return db.Model(record).Updates(map[string]interface{}{
		"can_view":    permission.CanView,
		"can_create":  permission.CanCreate,
		"can_edit":    permission.CanEdit,
		"can_delete":  permission.CanDelete,
		"can_approve": permission.CanApprove,
		"can_export":  permission.CanExport,
		"can_menu":    permission.CanMenu,
	}).Error
}

// FinanceStaffDefaultPermissionMigration aligns legacy finance staff module
// permissions with the newer operational default while preserving custom
// overrides that no longer match the old baseline.
func FinanceStaffDefaultPermissionMigration(db *gorm.DB) {
	var users []models.User
	if err := db.Where("LOWER(role) = LOWER(?)", models.RoleFinanceStaff).Find(&users).Error; err != nil {
		log.Printf("Warning: failed to load finance staff users for permission migration: %v", err)
		return
	}

	if len(users) == 0 {
		return
	}

	legacyDefaults := legacyFinanceStaffDefaultPermissions()
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
			default:
				if modulePermissionMatchesRecord(record, legacyDefaults[module]) {
					if updateErr := applyModulePermissionToRecord(db, &record, newPermission); updateErr != nil {
						log.Printf("Warning: failed to update finance staff permission for user %s module %s: %v", user.Username, module, updateErr)
					}
				}
			}
		}
	}

	log.Println("Finance staff default permission migration completed")
}
