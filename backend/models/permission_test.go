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

		if !journalPerm.CanView || !journalPerm.CanCreate || !journalPerm.CanApprove || !journalPerm.CanMenu {
			t.Fatalf("expected finance_manager to submit and approve journal workflow, got %+v", *journalPerm)
		}

		if journalPerm.CanEdit || journalPerm.CanDelete {
			t.Fatalf("expected finance_manager journal permission to avoid edit/delete bypass, got %+v", *journalPerm)
		}
	})

	t.Run("finance alias follows finance manager journal workflow", func(t *testing.T) {
		journalPerm := GetDefaultPermissions(RoleFinance)["journal"]
		if journalPerm == nil {
			t.Fatal("expected finance journal permission to exist")
		}

		if !journalPerm.CanApprove || !journalPerm.CanCreate || journalPerm.CanEdit {
			t.Fatalf("expected finance role to match finance manager journal workflow, got %+v", *journalPerm)
		}
	})

	t.Run("admin is read only on journal workflow", func(t *testing.T) {
		journalPerm := GetDefaultPermissions(RoleAdmin)["journal"]
		if journalPerm == nil {
			t.Fatal("expected admin journal permission to exist")
		}

		if !journalPerm.CanView || !journalPerm.CanExport || !journalPerm.CanMenu {
			t.Fatalf("expected admin to keep journal visibility, got %+v", *journalPerm)
		}

		if journalPerm.CanCreate || journalPerm.CanEdit || journalPerm.CanDelete || journalPerm.CanApprove {
			t.Fatalf("expected admin to stay outside journal operational workflow, got %+v", *journalPerm)
		}
	})

	t.Run("workflow managed journal permission ignores legacy overrides", func(t *testing.T) {
		legacyFullAccess := &ModulePermission{
			CanView:    true,
			CanCreate:  true,
			CanEdit:    true,
			CanDelete:  true,
			CanApprove: true,
			CanExport:  true,
			CanMenu:    true,
		}

		effective := EnforceWorkflowManagedModulePermission(RoleAdmin, "journal", legacyFullAccess)
		if effective == nil {
			t.Fatal("expected effective journal permission for admin")
		}

		if effective.CanCreate || effective.CanEdit || effective.CanDelete || effective.CanApprove {
			t.Fatalf("expected workflow enforcement to remove admin journal operational access, got %+v", *effective)
		}
	})
}

func TestGetDefaultPermissionsCashBankTransferApprovalWorkflow(t *testing.T) {
	t.Run("finance manager keeps cash bank approval access", func(t *testing.T) {
		perm := GetDefaultPermissions(RoleFinanceManager)["cash_bank"]
		if perm == nil {
			t.Fatal("expected finance_manager cash_bank permission to exist")
		}

		if !perm.CanApprove || !perm.CanCreate || !perm.CanView {
			t.Fatalf("expected finance_manager to manage cash bank transfer approvals, got %+v", *perm)
		}
	})

	t.Run("admin stays outside bank transfer approval lane", func(t *testing.T) {
		perm := GetDefaultPermissions(RoleAdmin)["cash_bank"]
		if perm == nil {
			t.Fatal("expected admin cash_bank permission to exist")
		}

		if !perm.CanView || !perm.CanCreate {
			t.Fatalf("expected admin to keep cash bank access, got %+v", *perm)
		}

		if perm.CanApprove {
			t.Fatalf("expected admin cash_bank approval to be disabled, got %+v", *perm)
		}
	})

	t.Run("director stays outside bank transfer approval lane", func(t *testing.T) {
		perm := GetDefaultPermissions(RoleDirector)["cash_bank"]
		if perm == nil {
			t.Fatal("expected director cash_bank permission to exist")
		}

		if perm.CanApprove {
			t.Fatalf("expected director cash_bank approval to be disabled, got %+v", *perm)
		}
	})
}

func TestGetDefaultPermissionsFinanceStaffProjectAndPurchaseRequestCreate(t *testing.T) {
	permissions := GetDefaultPermissions(RoleFinanceStaff)

	projectPermission := permissions["projects"]
	if projectPermission == nil {
		t.Fatal("expected finance_staff projects permission to exist")
	}

	if !projectPermission.CanView || !projectPermission.CanCreate || !projectPermission.CanEdit || !projectPermission.CanMenu {
		t.Fatalf("expected finance_staff to manage project drafts, got %+v", *projectPermission)
	}

	if projectPermission.CanApprove || projectPermission.CanDelete {
		t.Fatalf("expected finance_staff project permission to avoid approve/delete bypass, got %+v", *projectPermission)
	}

	purchaseRequestPermission := permissions["purchase_requests"]
	if purchaseRequestPermission == nil {
		t.Fatal("expected finance_staff purchase_requests permission to exist")
	}

	if !purchaseRequestPermission.CanView || !purchaseRequestPermission.CanCreate || !purchaseRequestPermission.CanEdit || !purchaseRequestPermission.CanApprove || !purchaseRequestPermission.CanMenu {
		t.Fatalf("expected finance_staff to create and review purchase requests, got %+v", *purchaseRequestPermission)
	}

	if purchaseRequestPermission.CanDelete {
		t.Fatalf("expected finance_staff purchase request permission to avoid delete bypass, got %+v", *purchaseRequestPermission)
	}
}

func TestGetDefaultPermissionsEmployeePurchaseRequestAndPaymentCreate(t *testing.T) {
	permissions := GetDefaultPermissions(RoleEmployee)

	purchaseRequestPermission := permissions["purchase_requests"]
	if purchaseRequestPermission == nil {
		t.Fatal("expected employee purchase_requests permission to exist")
	}

	if !purchaseRequestPermission.CanView || !purchaseRequestPermission.CanCreate || !purchaseRequestPermission.CanEdit || !purchaseRequestPermission.CanMenu {
		t.Fatalf("expected employee to create and manage own purchase requests, got %+v", *purchaseRequestPermission)
	}

	if purchaseRequestPermission.CanApprove || purchaseRequestPermission.CanDelete {
		t.Fatalf("expected employee purchase request permission to avoid approve/delete bypass, got %+v", *purchaseRequestPermission)
	}

	paymentPermission := permissions["payments"]
	if paymentPermission == nil {
		t.Fatal("expected employee payments permission to exist")
	}

	if !paymentPermission.CanView || !paymentPermission.CanCreate || !paymentPermission.CanEdit || !paymentPermission.CanMenu {
		t.Fatalf("expected employee to create payment drafts from payments page, got %+v", *paymentPermission)
	}

	if paymentPermission.CanApprove || paymentPermission.CanDelete {
		t.Fatalf("expected employee payments permission to avoid approve/delete bypass, got %+v", *paymentPermission)
	}

	cashBankPermission := permissions["cash_bank"]
	if cashBankPermission == nil {
		t.Fatal("expected employee cash_bank permission to exist")
	}

	if !cashBankPermission.CanView {
		t.Fatalf("expected employee to load payment cash/bank accounts, got %+v", *cashBankPermission)
	}

	if cashBankPermission.CanCreate || cashBankPermission.CanEdit || cashBankPermission.CanApprove || cashBankPermission.CanMenu {
		t.Fatalf("expected employee cash_bank access to stay read-only without menu exposure, got %+v", *cashBankPermission)
	}
}
