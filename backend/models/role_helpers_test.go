package models

import "testing"

func TestNormalizeRoleAcceptsDisplayRoleLabels(t *testing.T) {
	cases := map[string]string{
		"Finance Manager":   RoleFinanceManager,
		"FINANCE-MANAGER":   RoleFinanceManager,
		" finance  staff ":  RoleFinanceStaff,
		"Accounting":        RoleAccounting,
		"Inventory Manager": RoleInventoryManager,
	}

	for input, expected := range cases {
		if actual := NormalizeRole(input); actual != expected {
			t.Fatalf("expected %q to normalize to %q, got %q", input, expected, actual)
		}
	}
}

func TestCanManageRequiredApprovalsFinanceOnly(t *testing.T) {
	t.Run("finance manager can approve required payment drafts", func(t *testing.T) {
		if !CanManageRequiredApprovals(RoleFinanceManager) {
			t.Fatalf("expected finance_manager to manage required approvals")
		}

		if !CanManageRequiredApprovals(RoleFinance) {
			t.Fatalf("expected finance alias to manage required approvals")
		}
	})

	t.Run("admin cannot bypass finance approval lane", func(t *testing.T) {
		if CanManageRequiredApprovals(RoleAdmin) {
			t.Fatalf("expected admin to be blocked from required payment approvals")
		}
	})
}

func TestCanReviewPurchaseRequestsByStageRolesOnly(t *testing.T) {
	t.Run("finance team and director can review purchase requests", func(t *testing.T) {
		if !CanReviewPurchaseRequests(RoleFinanceStaff) {
			t.Fatalf("expected finance_staff to review purchase requests")
		}
		if !CanReviewPurchaseRequests(RoleAccounting) {
			t.Fatalf("expected accounting to review purchase requests")
		}
		if !CanReviewPurchaseRequests(RoleFinanceManager) {
			t.Fatalf("expected finance_manager to review purchase requests")
		}
		if !CanReviewPurchaseRequests(RoleDirector) {
			t.Fatalf("expected director to review purchase requests")
		}
	})

	t.Run("admin cannot bypass purchase request approval stages", func(t *testing.T) {
		if CanReviewPurchaseRequests(RoleAdmin) {
			t.Fatalf("expected admin to be blocked from purchase request review workflow")
		}
	})
}

func TestWorkflowRoleMatchesUserRoleDoesNotBypassAdmin(t *testing.T) {
	if WorkflowRoleMatchesUserRole(RoleDirector, RoleAdmin) {
		t.Fatalf("expected admin to stop bypassing workflow roles")
	}

	if !WorkflowRoleMatchesUserRole(RoleFinance, RoleFinanceManager) {
		t.Fatalf("expected finance manager to match finance workflow role")
	}

	if !WorkflowRoleMatchesUserRole(RoleFinanceStaff, RoleAccounting) {
		t.Fatalf("expected accounting to match finance staff workflow role")
	}
}
