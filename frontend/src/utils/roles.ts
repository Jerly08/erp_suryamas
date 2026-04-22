export const normalizeRole = (role?: string | null): string => {
  return (role || '').toString().trim().toLowerCase();
};

export const canonicalRole = (role?: string | null): string => {
  const normalized = normalizeRole(role);
  return normalized === 'finance_manager' ? 'finance' : normalized;
};

// Public alias for readability in UI code
export const toRoleKey = (role?: string | null): string => normalizeRole(role);

export const normalizeRoles = (roles?: (string | null | undefined)[]): string[] => {
  return (roles || []).map(r => normalizeRole(r)).filter(Boolean);
};

export const isFinanceManagerRole = (role?: string | null): boolean => {
  const normalized = normalizeRole(role);
  return normalized === 'finance' || normalized === 'finance_manager';
};

export const isFinanceStaffRole = (role?: string | null): boolean => {
  return normalizeRole(role) === 'finance_staff';
};

export const isFinanceTeamRole = (role?: string | null): boolean => {
  return isFinanceManagerRole(role) || isFinanceStaffRole(role);
};

export const isStaffRole = (role?: string | null): boolean => {
  return normalizeRole(role) === 'employee';
};

export const approvalRoleForUser = (role?: string | null): string => {
  return isFinanceManagerRole(role) ? 'finance' : normalizeRole(role);
};

export const workflowRoleMatchesUserRole = (workflowRole?: string | null, userRole?: string | null): boolean => {
  const normalizedUserRole = normalizeRole(userRole);
  if (!normalizedUserRole) return false;

  const normalizedWorkflowRole = normalizeRole(workflowRole);
  if (normalizedWorkflowRole === 'finance') {
    return isFinanceManagerRole(normalizedUserRole);
  }

  return normalizedWorkflowRole !== '' && normalizedWorkflowRole === normalizedUserRole;
};

export const canManageRequiredApprovals = (role?: string | null): boolean => {
  return isFinanceManagerRole(role);
};

export const isRoleAllowed = (allowedRoles: (string | null | undefined)[] = [], role?: string | null): boolean => {
  const roleNorm = normalizeRole(role);
  const roleCanonical = canonicalRole(role);
  const allowed = normalizeRoles(allowedRoles);
  return roleNorm !== '' && allowed.some(allowedRole =>
    allowedRole === roleNorm || canonicalRole(allowedRole) === roleCanonical
  );
};

export const humanizeRole = (role?: string | null): string => {
  const r = normalizeRole(role);
  if (!r) return 'Unknown';
  return r.charAt(0).toUpperCase() + r.slice(1);
};

// Format role for approval trail display
export const formatRoleForApproval = (role?: string | null): string => {
  const r = normalizeRole(role);
  switch (r) {
    case 'admin':
      return 'Admin';
    case 'director':
      return 'Director';
    case 'finance':
    case 'finance_manager':
      return 'Finance Manager';
    case 'finance_staff':
      return 'Finance Staff';
    case 'employee':
      return 'Staff';
    case 'inventory_manager':
      return 'Inventory Manager';
    default:
      return humanizeRole(role);
  }
};

// Get name mapping for specific roles in Indonesian context
export const getRoleDisplayName = (role?: string | null): { role: string; example?: string } => {
  const r = normalizeRole(role);
  switch (r) {
    case 'employee':
      return { role: 'Staff', example: 'John' };
    case 'finance':
    case 'finance_manager':
      return { role: 'Finance Manager', example: 'Jack' };
    case 'finance_staff':
      return { role: 'Finance Staff', example: 'Jill' };
    case 'director':
      return { role: 'Director', example: 'Josh' };
    case 'admin':
      return { role: 'Admin' };
    case 'inventory_manager':
      return { role: 'Inventory Manager' };
    default:
      return { role: humanizeRole(role) };
  }
};

