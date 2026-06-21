package database

import (
	"app-sistem-akuntansi/models"
	"log"

	"gorm.io/gorm"
)

// CashBankTransferApprovalPermissionMigration aligns cash & bank permissions
// with the bank transfer workflow:
//   - finance manager/finance can approve transfer requests
//   - non finance-manager roles can create based on their module access, but
//     approval stays with finance manager
func CashBankTransferApprovalPermissionMigration(db *gorm.DB) {
	if db == nil {
		return
	}

	var users []models.User
	if err := db.Find(&users).Error; err != nil {
		log.Printf("Warning: failed to load users for cash bank permission migration: %v", err)
		return
	}

	for _, user := range users {
		permission := models.GetDefaultPermissions(user.Role)["cash_bank"]
		if permission == nil {
			continue
		}

		err := db.Exec(`
			INSERT INTO module_permissions
				(user_id, module, can_view, can_create, can_edit, can_delete, can_approve, can_export, can_menu, created_at, updated_at, deleted_at)
			VALUES
				(?, 'cash_bank', ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), NULL)
			ON CONFLICT (user_id, module) DO NOTHING
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
			log.Printf("Warning: failed to upsert cash_bank permission for user %s (%s): %v", user.Username, user.Role, err)
		}
	}

	log.Println("Cash bank transfer approval permission migration completed")
}
