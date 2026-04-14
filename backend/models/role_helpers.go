package models

import "strings"

// NormalizeRole standardizes role comparisons across the app.
func NormalizeRole(role string) string {
	return strings.ToLower(strings.TrimSpace(role))
}

// CanonicalRole maps legacy aliases into their persisted internal role key.
func CanonicalRole(role string) string {
	switch NormalizeRole(role) {
	case RoleFinanceManager:
		return RoleFinance
	default:
		return NormalizeRole(role)
	}
}

// NormalizeRoleForStorage returns the normalized role that should be persisted.
func NormalizeRoleForStorage(role string) string {
	canonicalRole := CanonicalRole(role)
	if canonicalRole == "" {
		return RoleEmployee
	}

	return canonicalRole
}

func IsValidUserRole(role string) bool {
	switch NormalizeRole(role) {
	case RoleAdmin, RoleFinance, RoleFinanceManager, RoleFinanceStaff, RoleDirector, RoleInventoryManager, RoleEmployee, RoleAuditor, RoleOperationalUser:
		return true
	default:
		return false
	}
}

func CanManageRequiredApprovals(role string) bool {
	return NormalizeRole(role) == RoleAdmin || IsFinanceManagerRole(role)
}

func IsFinanceManagerRole(role string) bool {
	switch NormalizeRole(role) {
	case RoleFinance, RoleFinanceManager:
		return true
	default:
		return false
	}
}

func IsFinanceStaffRole(role string) bool {
	return NormalizeRole(role) == RoleFinanceStaff
}

func IsFinanceTeamRole(role string) bool {
	return IsFinanceManagerRole(role) || IsFinanceStaffRole(role)
}

func IsStaffRole(role string) bool {
	return NormalizeRole(role) == RoleEmployee
}

func CanReviewPurchaseRequests(role string) bool {
	return NormalizeRole(role) == RoleAdmin || IsFinanceTeamRole(role)
}

// CanViewAllTransactionData controls whether a role can browse requests from all users.
func CanViewAllTransactionData(role string) bool {
	switch NormalizeRole(role) {
	case RoleAdmin, RoleDirector, RoleInventoryManager, RoleFinanceStaff:
		return true
	default:
		return IsFinanceManagerRole(role)
	}
}

// ApprovalRoleForUser maps a user role into the workflow approver bucket.
func ApprovalRoleForUser(role string) string {
	if IsFinanceManagerRole(role) {
		return RoleFinance
	}

	return NormalizeRole(role)
}

// WorkflowRoleMatchesUserRole checks whether a user role can satisfy a workflow step.
func WorkflowRoleMatchesUserRole(workflowRole string, userRole string) bool {
	if NormalizeRole(userRole) == RoleAdmin {
		return true
	}

	requiredRole := NormalizeRole(workflowRole)
	switch requiredRole {
	case RoleFinance:
		return IsFinanceManagerRole(userRole)
	default:
		return requiredRole != "" && requiredRole == NormalizeRole(userRole)
	}
}

// WorkflowStepAssignableRoles returns the concrete user roles that can receive a step notification.
func WorkflowStepAssignableRoles(workflowRole string) []string {
	switch NormalizeRole(workflowRole) {
	case RoleFinance:
		return []string{RoleFinance, RoleFinanceManager}
	case "":
		return nil
	default:
		return []string{NormalizeRole(workflowRole)}
	}
}
