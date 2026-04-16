package models

import "testing"

func TestGetDefaultPermissionsJournalAdjustmentWorkflow(t *testing.T) {
	t.Run("finance staff prepares journal adjustments", func(t *testing.T) {
		journalPerm := GetDefaultPermissions(RoleFinanceStaff)["journal"]
		if journalPerm == nil {
			t.Fatal("expected finance_staff journal permission to exist")
		}

		if !journalPerm.CanView || !journalPerm.CanCreate || !journalPerm.CanEdit || !journalPerm.CanMenu {
			t.Fatalf("expected finance_staff to be able to access and prepare journal adjustments, got %+v", *journalPerm)
		}

		if journalPerm.CanApprove {
			t.Fatalf("expected finance_staff journal permission to block approval, got %+v", *journalPerm)
		}
	})

	t.Run("finance manager approves journal adjustments", func(t *testing.T) {
		journalPerm := GetDefaultPermissions(RoleFinanceManager)["journal"]
		if journalPerm == nil {
			t.Fatal("expected finance_manager journal permission to exist")
		}

		if !journalPerm.CanView || !journalPerm.CanApprove || !journalPerm.CanMenu {
			t.Fatalf("expected finance_manager to review and approve journal adjustments, got %+v", *journalPerm)
		}

		if journalPerm.CanCreate || journalPerm.CanEdit || journalPerm.CanDelete {
			t.Fatalf("expected finance_manager journal permission to avoid draft creation/edit/delete, got %+v", *journalPerm)
		}
	})

	t.Run("finance alias follows finance manager journal workflow", func(t *testing.T) {
		journalPerm := GetDefaultPermissions(RoleFinance)["journal"]
		if journalPerm == nil {
			t.Fatal("expected finance journal permission to exist")
		}

		if !journalPerm.CanApprove || journalPerm.CanCreate || journalPerm.CanEdit {
			t.Fatalf("expected finance role to match finance manager journal approval workflow, got %+v", *journalPerm)
		}
	})
}
