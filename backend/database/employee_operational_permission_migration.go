package database

import (
	"app-sistem-akuntansi/models"
	"log"

	"gorm.io/gorm"
)

// EmployeeOperationalPermissionMigration aligns employee access with the
// current workflow:
//   - employees/staff can add product master data from the Products page
//   - employees/staff can create purchase requests
//   - employees/staff can submit payment requests from the payments page
//   - approvals still stay with finance manager/director via module workflow
func EmployeeOperationalPermissionMigration(db *gorm.DB) {
	if db == nil {
		return
	}

	var users []models.User
	if err := db.Where("LOWER(role) = LOWER(?)", models.RoleEmployee).Find(&users).Error; err != nil {
		log.Printf("Warning: failed to load employee users for operational permission migration: %v", err)
		return
	}

	if len(users) == 0 {
		return
	}

	targetModules := []string{"products", "purchase_requests", "payments"}

	for _, user := range users {
		defaultPerms := models.GetDefaultPermissions(user.Role)

		for _, module := range targetModules {
			permission := defaultPerms[module]
			if permission == nil {
				continue
			}

			err := db.Exec(`
				INSERT INTO module_permissions
					(user_id, module, can_view, can_create, can_edit, can_delete, can_approve, can_export, can_menu, created_at, updated_at, deleted_at)
				VALUES
					(?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), NULL)
				ON CONFLICT (user_id, module)
				DO UPDATE SET
					can_view = EXCLUDED.can_view,
					can_create = EXCLUDED.can_create,
					can_edit = EXCLUDED.can_edit,
					can_delete = EXCLUDED.can_delete,
					can_approve = EXCLUDED.can_approve,
					can_export = EXCLUDED.can_export,
					can_menu = EXCLUDED.can_menu,
					updated_at = NOW(),
					deleted_at = NULL
			`,
				user.ID,
				module,
				permission.CanView,
				permission.CanCreate,
				permission.CanEdit,
				permission.CanDelete,
				permission.CanApprove,
				permission.CanExport,
				permission.CanMenu,
			).Error
			if err != nil {
				log.Printf("Warning: failed to upsert employee permission for user %s module %s: %v", user.Username, module, err)
			}
		}
	}

	log.Println("Employee operational permission migration completed")
}
