package database

import (
	"app-sistem-akuntansi/models"
	"log"
	"strings"

	"gorm.io/gorm"
)

// JournalManualApprovalPermissionMigration aligns journal access with the
// latest workflow:
// - employees can still submit manual journal drafts
// - finance staff prepares draft journal adjustments
// - finance manager/finance approves and posts journal drafts/adjustments
func JournalManualApprovalPermissionMigration(db *gorm.DB) {
	if db == nil {
		return
	}

	roleConfigs := map[string]*models.ModulePermission{
		"employee":        models.GetDefaultPermissions("employee")["journal"],
		"finance":         models.GetDefaultPermissions("finance")["journal"],
		"finance_manager": models.GetDefaultPermissions("finance_manager")["journal"],
		"finance_staff":   models.GetDefaultPermissions("finance_staff")["journal"],
		"admin":           models.GetDefaultPermissions("admin")["journal"],
		"director":        models.GetDefaultPermissions("director")["journal"],
	}

	for role, permission := range roleConfigs {
		if permission == nil {
			continue
		}

		var users []models.User
		if err := db.Where("LOWER(role) = ?", strings.ToLower(role)).Find(&users).Error; err != nil {
			log.Printf("Warning: failed to load %s users for journal permission migration: %v", role, err)
			continue
		}

		for _, user := range users {
			err := db.Exec(`
				INSERT INTO module_permissions
					(user_id, module, can_view, can_create, can_edit, can_delete, can_approve, can_export, can_menu, created_at, updated_at, deleted_at)
				VALUES
					(?, 'journal', ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), NULL)
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
				permission.CanView,
				permission.CanCreate,
				permission.CanEdit,
				permission.CanDelete,
				permission.CanApprove,
				permission.CanExport,
				permission.CanMenu,
			).Error
			if err != nil {
				log.Printf("Warning: failed to upsert journal permission for user %s (%s): %v", user.Username, role, err)
				continue
			}
		}
	}

	log.Println("Journal manual approval permission migration completed")
}
