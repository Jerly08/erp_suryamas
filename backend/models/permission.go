package models

import (
	"time"

	"gorm.io/gorm"
)

// ModulePermissionRecord represents a specific permission for modules
type ModulePermissionRecord struct {
	ID         uint           `json:"id" gorm:"primaryKey"`
	UserID     uint           `json:"user_id" gorm:"not null;uniqueIndex:idx_module_permissions_user_module"`
	Module     string         `json:"module" gorm:"not null;size:50;uniqueIndex:idx_module_permissions_user_module"` // accounts, products, contacts, assets, sales, purchases, payments, cash_bank, settings
	CanView    bool           `json:"can_view" gorm:"default:false"`
	CanCreate  bool           `json:"can_create" gorm:"default:false"`
	CanEdit    bool           `json:"can_edit" gorm:"default:false"`
	CanDelete  bool           `json:"can_delete" gorm:"default:false"`
	CanApprove bool           `json:"can_approve" gorm:"default:false"`
	CanExport  bool           `json:"can_export" gorm:"default:false"`
	CanMenu    bool           `json:"can_menu" gorm:"default:false"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`

	// Relations
	User User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

func (ModulePermissionRecord) TableName() string {
	return "module_permissions"
}

// UserPermission is a simplified structure for API responses
type UserPermission struct {
	UserID      uint                         `json:"user_id"`
	Username    string                       `json:"username"`
	Email       string                       `json:"email"`
	Role        string                       `json:"role"`
	Permissions map[string]*ModulePermission `json:"permissions"`
}

// ModulePermission represents permissions for a specific module
type ModulePermission struct {
	CanView    bool `json:"can_view"`
	CanCreate  bool `json:"can_create"`
	CanEdit    bool `json:"can_edit"`
	CanDelete  bool `json:"can_delete"`
	CanApprove bool `json:"can_approve"`
	CanExport  bool `json:"can_export"`
	CanMenu    bool `json:"can_menu"`
}

func cloneModulePermission(permission *ModulePermission) *ModulePermission {
	if permission == nil {
		return nil
	}

	cloned := *permission
	return &cloned
}

// GetJournalWorkflowPermission locks the journal module to the approved
// business workflow regardless of legacy permission rows:
// - finance staff prepares draft journal adjustments
// - finance manager/finance can submit their own journal directly or route it to director
// - finance manager/finance approves and posts staff drafts
// - admin stays read only for journal monitoring
func GetJournalWorkflowPermission(role string) *ModulePermission {
	switch NormalizeRole(role) {
	case RoleAdmin:
		return &ModulePermission{
			CanView:    true,
			CanCreate:  false,
			CanEdit:    false,
			CanDelete:  false,
			CanApprove: false,
			CanExport:  true,
			CanMenu:    true,
		}
	case RoleFinance, RoleFinanceManager:
		return &ModulePermission{
			CanView:    true,
			CanCreate:  true,
			CanEdit:    false,
			CanDelete:  false,
			CanApprove: true,
			CanExport:  true,
			CanMenu:    true,
		}
	case RoleFinanceStaff:
		return &ModulePermission{
			CanView:    true,
			CanCreate:  true,
			CanEdit:    true,
			CanDelete:  false,
			CanApprove: false,
			CanExport:  true,
			CanMenu:    true,
		}
	default:
		return nil
	}
}

func GetCashBankWorkflowPermission(role string, current *ModulePermission) *ModulePermission {
	permission := cloneModulePermission(current)
	if permission == nil {
		defaultPermissions := GetDefaultPermissions(role)
		permission = cloneModulePermission(defaultPermissions["cash_bank"])
	}
	if permission == nil {
		return nil
	}

	permission.CanApprove = IsFinanceManagerRole(role)
	return permission
}

func EnforceWorkflowManagedModulePermission(role string, module string, permission *ModulePermission) *ModulePermission {
	switch module {
	case "journal":
		if workflowPermission := GetJournalWorkflowPermission(role); workflowPermission != nil {
			return workflowPermission
		}
		return cloneModulePermission(permission)
	case "cash_bank":
		if workflowPermission := GetCashBankWorkflowPermission(role, permission); workflowPermission != nil {
			return workflowPermission
		}
		return cloneModulePermission(permission)
	default:
		return cloneModulePermission(permission)
	}
}

// GetDefaultPermissions returns default permissions based on role
func GetDefaultPermissions(role string) map[string]*ModulePermission {
	permissions := make(map[string]*ModulePermission)
	modules := []string{"accounts", "products", "contacts", "assets", "prepaid_expenses", "sales", "purchases", "payments", "cash_bank", "journal", "reports", "users", "settings", "purchase_requests", "projects", "boms"}

	switch NormalizeRole(role) {
	case RoleAdmin:
		for _, module := range modules {
			if journalPermission := GetJournalWorkflowPermission(role); module == "journal" && journalPermission != nil {
				permissions[module] = journalPermission
				continue
			}

			canApprove := true
			if module == "cash_bank" {
				canApprove = false
			}

			permissions[module] = &ModulePermission{
				CanView:    true,
				CanCreate:  true,
				CanEdit:    true,
				CanDelete:  true,
				CanApprove: canApprove,
				CanExport:  true,
				CanMenu:    true,
			}
		}
	case RoleFinance, RoleFinanceManager:
		// Finance and Finance Manager have full access to financial modules
		financialModules := []string{"accounts", "payments", "cash_bank", "sales", "purchases", "journal", "purchase_requests", "projects", "boms", "prepaid_expenses"}
		for _, module := range modules {
			if journalPermission := GetJournalWorkflowPermission(role); module == "journal" && journalPermission != nil {
				permissions[module] = journalPermission
			} else if contains(financialModules, module) {
				permissions[module] = &ModulePermission{
					CanView:    true,
					CanCreate:  true,
					CanEdit:    true,
					CanDelete:  false,
					CanApprove: true,
					CanExport:  true,
					CanMenu:    true,
				}
			} else if module == "settings" {
				// Finance roles need settings access for invoice types and financial configuration
				permissions[module] = &ModulePermission{
					CanView:    true,
					CanCreate:  true,  // Can create invoice types
					CanEdit:    true,  // Can edit invoice types
					CanDelete:  false, // Cannot delete settings for safety
					CanApprove: true,
					CanExport:  true,
					CanMenu:    true,
				}
			} else if module == "users" {
				// Users module is admin-only - no access for other roles
				permissions[module] = &ModulePermission{
					CanView:    false,
					CanCreate:  false,
					CanEdit:    false,
					CanDelete:  false,
					CanApprove: false,
					CanExport:  false,
					CanMenu:    false,
				}
			} else {
				permissions[module] = &ModulePermission{
					CanView:    true,
					CanCreate:  false,
					CanEdit:    false,
					CanDelete:  false,
					CanApprove: false,
					CanExport:  false,
					CanMenu:    false, // No menu access for non-financial modules
				}
			}
		}
	case RoleFinanceStaff:
		operationalModules := []string{"sales", "purchases", "payments", "journal", "prepaid_expenses", "projects"}
		viewOnlyMenuModules := []string{"accounts", "reports", "boms"}
		for _, module := range modules {
			switch {
			case module == "journal":
				permissions[module] = GetJournalWorkflowPermission(role)
			case contains(operationalModules, module):
				permissions[module] = &ModulePermission{
					CanView:    true,
					CanCreate:  true,
					CanEdit:    true,
					CanDelete:  false,
					CanApprove: false,
					CanExport:  true,
					CanMenu:    true,
				}
			case module == "cash_bank":
				permissions[module] = &ModulePermission{
					CanView:    true,
					CanCreate:  true,
					CanEdit:    false,
					CanDelete:  false,
					CanApprove: false,
					CanExport:  true,
					CanMenu:    true,
				}
			case module == "purchase_requests":
				permissions[module] = &ModulePermission{
					CanView:    true,
					CanCreate:  true,
					CanEdit:    true,
					CanDelete:  false,
					CanApprove: true,
					CanExport:  true,
					CanMenu:    true,
				}
			case contains(viewOnlyMenuModules, module):
				permissions[module] = &ModulePermission{
					CanView:    true,
					CanCreate:  false,
					CanEdit:    false,
					CanDelete:  false,
					CanApprove: false,
					CanExport:  true,
					CanMenu:    true,
				}
			case module == "products" || module == "contacts" || module == "assets":
				permissions[module] = &ModulePermission{
					CanView:    true,
					CanCreate:  false,
					CanEdit:    false,
					CanDelete:  false,
					CanApprove: false,
					CanExport:  false,
					CanMenu:    false,
				}
			case module == "users" || module == "settings":
				permissions[module] = &ModulePermission{
					CanView:    false,
					CanCreate:  false,
					CanEdit:    false,
					CanDelete:  false,
					CanApprove: false,
					CanExport:  false,
					CanMenu:    false,
				}
			default:
				permissions[module] = &ModulePermission{
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
	case RoleInventoryManager:
		// Inventory manager has comprehensive access to inventory and related operations
		coreInventoryModules := []string{"products", "purchases", "sales", "boms"}
		supportingModules := []string{"contacts", "assets", "reports"}
		financialSupportModules := []string{"accounts", "payments", "cash_bank", "journal"}

		for _, module := range modules {
			if contains(coreInventoryModules, module) {
				// Full access to core inventory modules
				permissions[module] = &ModulePermission{
					CanView:    true,
					CanCreate:  true,
					CanEdit:    true,
					CanDelete:  false, // Safety: no delete permission
					CanApprove: false, // Purchase approvals handled by finance/director
					CanExport:  true,
					CanMenu:    true,
				}
			} else if contains(supportingModules, module) {
				// Good access to supporting modules (contacts for vendors/customers, assets for inventory items, reports for analytics)
				permissions[module] = &ModulePermission{
					CanView:    true,
					CanCreate:  true,
					CanEdit:    true,
					CanDelete:  false,
					CanApprove: false,
					CanExport:  true, // Can export reports and asset lists
					CanMenu:    true,
				}
			} else if contains(financialSupportModules, module) {
				// Limited financial access - can create entries for inventory operations but cannot approve
				permissions[module] = &ModulePermission{
					CanView:    true,
					CanCreate:  true,  // Can create expense accounts, payments for purchases
					CanEdit:    false, // Cannot edit financial records (safety)
					CanDelete:  false,
					CanApprove: false, // Financial approvals remain with finance team
					CanExport:  true,  // Can export for reporting
					CanMenu:    false, // No menu access to financial modules
				}
			} else if module == "users" {
				// Users module is admin-only - no access for other roles
				permissions[module] = &ModulePermission{
					CanView:    false,
					CanCreate:  false,
					CanEdit:    false,
					CanDelete:  false,
					CanApprove: false,
					CanExport:  false,
					CanMenu:    false,
				}
			} else {
				// View-only access to other modules
				permissions[module] = &ModulePermission{
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
	case RoleEmployee:
		// Employee has limited access
		for _, module := range modules {
			if module == "contacts" {
				// Employee needs view access to contacts for vendor/customer data loading
				// but should NOT have menu access to browse contacts directly
				permissions[module] = &ModulePermission{
					CanView:    true, // Essential for loading vendor/customer lists in purchases
					CanCreate:  true, // Can create vendors/customers when needed
					CanEdit:    false,
					CanDelete:  false,
					CanApprove: false,
					CanExport:  false,
					CanMenu:    false, // KEY: No menu access to prevent browsing other employees
				}
			} else if module == "products" || module == "boms" {
				permissions[module] = &ModulePermission{
					CanView:    true,
					CanCreate:  true,
					CanEdit:    false,
					CanDelete:  false,
					CanApprove: false,
					CanExport:  false,
					CanMenu:    true, // Can access products menu
				}
			} else if module == "accounts" {
				// Employee needs view access to accounts for purchase form dropdowns
				permissions[module] = &ModulePermission{
					CanView:    true, // Essential for purchase forms
					CanCreate:  false,
					CanEdit:    false,
					CanDelete:  false,
					CanApprove: false,
					CanExport:  false,
					CanMenu:    false, // No menu access to accounts
				}
			} else if module == "cash_bank" {
				// Employee needs read access to payment accounts for payment form dropdowns,
				// but should not operate cash/bank directly from that module.
				permissions[module] = &ModulePermission{
					CanView:    true,
					CanCreate:  false,
					CanEdit:    false,
					CanDelete:  false,
					CanApprove: false,
					CanExport:  false,
					CanMenu:    false,
				}
			} else if module == "purchases" || module == "purchase_requests" {
				// Employee should be able to create purchases and purchase requests
				permissions[module] = &ModulePermission{
					CanView:    true,
					CanCreate:  true,  // Employees can create purchase requests
					CanEdit:    true,  // Can edit their own requests/purchases
					CanDelete:  false, // Cannot delete purchases
					CanApprove: false, // Cannot approve purchases
					CanExport:  false,
					CanMenu:    true, // Can access purchases and requests menu
				}
			} else if module == "payments" {
				// Employee can submit payment requests and revise their own drafts,
				// while finance manager remains the approver.
				permissions[module] = &ModulePermission{
					CanView:    true,
					CanCreate:  true,
					CanEdit:    true,
					CanDelete:  false,
					CanApprove: false,
					CanExport:  false,
					CanMenu:    true,
				}
			} else if module == "journal" {
				// Employees may submit manual journals as drafts, but finance verifies/posts them.
				permissions[module] = &ModulePermission{
					CanView:    true,
					CanCreate:  true,
					CanEdit:    false,
					CanDelete:  false,
					CanApprove: false,
					CanExport:  false,
					CanMenu:    true,
				}
			} else if module == "users" {
				// Users module is admin-only - no access for employees
				permissions[module] = &ModulePermission{
					CanView:    false,
					CanCreate:  false,
					CanEdit:    false,
					CanDelete:  false,
					CanApprove: false,
					CanExport:  false,
					CanMenu:    false,
				}
			} else {
				permissions[module] = &ModulePermission{
					CanView:    true,
					CanCreate:  false,
					CanEdit:    false,
					CanDelete:  false,
					CanApprove: false,
					CanExport:  false,
					CanMenu:    false, // No menu access to other modules
				}
			}
		}
	case RoleDirector:
		// Director has view, approve, and limited create/edit access
		for _, module := range modules {
			if module == "purchases" || module == "purchase_requests" || module == "sales" || module == "payments" || module == "cash_bank" || module == "journal" || module == "projects" || module == "boms" {
				// Directors need create/edit access for purchases to create receipts,
				// and for sales/payments for operational oversight
				permissions[module] = &ModulePermission{
					CanView:    true,
					CanCreate:  true,  // ✅ Allow creating for operational modules
					CanEdit:    true,  // ✅ Allow editing for operational modules (needed for receipts)
					CanDelete:  false, // Still no delete access for safety
					CanApprove: module != "cash_bank",
					CanExport:  true,
					CanMenu:    true,
				}
			} else if module == "settings" {
				// Directors need settings access for system configuration and invoice types
				permissions[module] = &ModulePermission{
					CanView:    true,
					CanCreate:  true,  // Can create invoice types and settings
					CanEdit:    true,  // Can edit system settings
					CanDelete:  false, // Cannot delete settings for safety
					CanApprove: true,
					CanExport:  true,
					CanMenu:    false, // Directors don't need settings menu access
				}
			} else if module == "users" {
				// Users module is admin-only - no access for directors
				permissions[module] = &ModulePermission{
					CanView:    false,
					CanCreate:  false,
					CanEdit:    false,
					CanDelete:  false,
					CanApprove: false,
					CanExport:  false,
					CanMenu:    false,
				}
			} else {
				// For other modules, keep view/approve only access
				permissions[module] = &ModulePermission{
					CanView:    true,
					CanCreate:  false,
					CanEdit:    false,
					CanDelete:  false,
					CanApprove: true,
					CanExport:  true,
					CanMenu:    false, // No menu access to other modules
				}
			}
		}
	default:
		// Default no permissions
		for _, module := range modules {
			permissions[module] = &ModulePermission{
				CanView:    false,
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

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
