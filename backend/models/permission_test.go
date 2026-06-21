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

	t.Run("accounting follows finance staff journal adjustment workflow", func(t *testing.T) {
		journalPerm := GetDefaultPermissions(RoleAccounting)["journal"]
		if journalPerm == nil {
			t.Fatal("expected accounting journal permission to exist")
		}

		if !journalPerm.CanView || !journalPerm.CanCreate || !journalPerm.CanEdit || !journalPerm.CanMenu {
			t.Fatalf("expected accounting to prepare journal adjustments, got %+v", *journalPerm)
		}

		if journalPerm.CanApprove {
			t.Fatalf("expected accounting journal permission to block approval, got %+v", *journalPerm)
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

func TestGetDefaultPermissionsPayrollWorkflow(t *testing.T) {
	t.Run("hr creates payroll but cannot approve", func(t *testing.T) {
		perm := GetDefaultPermissions(RoleHR)["payroll"]
		if perm == nil {
			t.Fatal("expected HR payroll permission to exist")
		}

		if !perm.CanView || !perm.CanCreate || !perm.CanEdit || !perm.CanMenu {
			t.Fatalf("expected HR to input payroll details, got %+v", *perm)
		}

		if perm.CanApprove || perm.CanDelete {
			t.Fatalf("expected HR payroll permission to avoid approve/delete bypass, got %+v", *perm)
		}
	})

	t.Run("finance manager approves payroll without creating it", func(t *testing.T) {
		perm := GetDefaultPermissions(RoleFinanceManager)["payroll"]
		if perm == nil {
			t.Fatal("expected finance_manager payroll permission to exist")
		}

		if !perm.CanView || !perm.CanApprove || !perm.CanMenu {
			t.Fatalf("expected finance_manager to view and approve payroll, got %+v", *perm)
		}

		if perm.CanCreate || perm.CanEdit || perm.CanDelete {
			t.Fatalf("expected finance_manager payroll permission to avoid HR input lane, got %+v", *perm)
		}
	})

	t.Run("finance staff and accounting get summary access only", func(t *testing.T) {
		for _, role := range []string{RoleFinanceStaff, RoleAccounting} {
			perm := GetDefaultPermissions(role)["payroll"]
			if perm == nil {
				t.Fatalf("expected %s payroll permission to exist", role)
			}

			if !perm.CanView || !perm.CanMenu || !perm.CanExport {
				t.Fatalf("expected %s to see payroll summary, got %+v", role, *perm)
			}

			if perm.CanCreate || perm.CanEdit || perm.CanDelete || perm.CanApprove {
				t.Fatalf("expected %s payroll permission to be summary-only, got %+v", role, *perm)
			}
		}
	})

	t.Run("admin cannot bypass finance manager payroll approval", func(t *testing.T) {
		perm := GetDefaultPermissions(RoleAdmin)["payroll"]
		if perm == nil {
			t.Fatal("expected admin payroll permission to exist")
		}

		if !perm.CanView || !perm.CanCreate || !perm.CanEdit || !perm.CanMenu {
			t.Fatalf("expected admin to support payroll setup and input, got %+v", *perm)
		}

		if perm.CanApprove {
			t.Fatalf("expected payroll approval to stay with finance manager, got %+v", *perm)
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

func TestGetDefaultPermissionsAccountingMirrorsFinanceStaffOperations(t *testing.T) {
	permissions := GetDefaultPermissions(RoleAccounting)

	accountPermission := permissions["accounts"]
	if accountPermission == nil {
		t.Fatal("expected accounting accounts permission to exist")
	}
	if !accountPermission.CanView || accountPermission.CanCreate || accountPermission.CanEdit || accountPermission.CanDelete {
		t.Fatalf("expected accounting to view COA without managing COA records, got %+v", *accountPermission)
	}

	purchasePermission := permissions["purchases"]
	if purchasePermission == nil {
		t.Fatal("expected accounting purchases permission to exist")
	}
	if !purchasePermission.CanView || !purchasePermission.CanCreate || !purchasePermission.CanEdit || !purchasePermission.CanMenu {
		t.Fatalf("expected accounting to create and edit purchase drafts, got %+v", *purchasePermission)
	}
	if !purchasePermission.CanApprove || purchasePermission.CanDelete {
		t.Fatalf("expected accounting to review purchases without delete access, got %+v", *purchasePermission)
	}
}

func TestGetDefaultPermissionsEmployeePurchaseRequestAndPaymentCreate(t *testing.T) {
	permissions := GetDefaultPermissions(RoleEmployee)

	productPermission := permissions["products"]
	if productPermission == nil {
		t.Fatal("expected employee products permission to exist")
	}

	if !productPermission.CanView || !productPermission.CanCreate || !productPermission.CanMenu {
		t.Fatalf("expected employee to view and add products from products page, got %+v", *productPermission)
	}

	if productPermission.CanEdit || productPermission.CanDelete || productPermission.CanApprove {
		t.Fatalf("expected employee product permission to avoid edit/delete/approve bypass, got %+v", *productPermission)
	}

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
