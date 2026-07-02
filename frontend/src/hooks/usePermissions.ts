import { useState, useEffect } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import api from '@/services/api';
import { handleApiError } from '@/utils/authErrorHandler';
import { API_ENDPOINTS } from '@/config/api';

interface ModulePermission {
  can_view: boolean;
  can_create: boolean;
  can_edit: boolean;
  can_delete: boolean;
  can_approve: boolean;
  can_export: boolean;
  can_menu: boolean;
}

interface UserPermissions {
  [module: string]: ModulePermission;
}

interface UsePermissionsReturn {
  permissions: UserPermissions | null;
  loading: boolean;
  error: string | null;
  canView: (module: string) => boolean;
  canCreate: (module: string) => boolean;
  canEdit: (module: string) => boolean;
  canDelete: (module: string) => boolean;
  canApprove: (module: string) => boolean;
  canExport: (module: string) => boolean;
  canMenu: (module: string) => boolean;
  hasAnyPermission: (module: string) => boolean;
  refetchPermissions: () => Promise<void>;
}

const DEFAULT_PERMISSION: ModulePermission = {
  can_view: false,
  can_create: false,
  can_edit: false,
  can_delete: false,
  can_approve: false,
  can_export: false,
  can_menu: false,
};

export const usePermissions = (): UsePermissionsReturn => {
  const { user } = useAuth();
  const [permissions, setPermissions] = useState<UserPermissions | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchPermissions = async () => {
    if (!user) {
      setLoading(false);
      return;
    }

    try {
      setLoading(true);
      setError(null);

      // Use the configured API instance which has auth interceptors
      const response = await api.get(API_ENDPOINTS.PERMISSIONS_ME);
      
      setPermissions(response.data.permissions || {});
    } catch (err: any) {
      console.error('Error fetching permissions:', err);
      
      // Use the centralized error handler
      const errorResult = handleApiError(err, 'usePermissions.fetchPermissions');
      
      // Set the error message for the UI
      setError(errorResult.message);
      
      // Always clear permissions on any error for security
      setPermissions({});
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchPermissions();

    const handlePermissionsUpdated = (event: Event) => {
      const updatedUserId = (event as CustomEvent<{ userId?: number | string }>).detail?.userId;
      if (updatedUserId !== undefined && String(updatedUserId) === String(user?.id)) {
        fetchPermissions();
      }
    };

    window.addEventListener('permissions-updated', handlePermissionsUpdated);
    return () => window.removeEventListener('permissions-updated', handlePermissionsUpdated);
  }, [user]);

  const getModulePermission = (module: string): ModulePermission => {
    if (!permissions || !permissions[module]) {
      return DEFAULT_PERMISSION;
    }
    return permissions[module];
  };

  const canView = (module: string): boolean => {
    const perm = getModulePermission(module);
    return perm.can_view;
  };

  const canCreate = (module: string): boolean => {
    const perm = getModulePermission(module);
    return perm.can_create;
  };

  const canEdit = (module: string): boolean => {
    const perm = getModulePermission(module);
    return perm.can_edit;
  };

  const canDelete = (module: string): boolean => {
    const perm = getModulePermission(module);
    return perm.can_delete;
  };

  const canApprove = (module: string): boolean => {
    const perm = getModulePermission(module);
    return perm.can_approve;
  };

  const canExport = (module: string): boolean => {
    const perm = getModulePermission(module);
    return perm.can_export;
  };

  const canMenu = (module: string): boolean => {
    const perm = getModulePermission(module);
    return perm.can_menu;
  };

  const hasAnyPermission = (module: string): boolean => {
    const perm = getModulePermission(module);
    return perm.can_view || perm.can_create || perm.can_edit || 
           perm.can_delete || perm.can_approve || perm.can_export;
  };

  return {
    permissions,
    loading,
    error,
    canView,
    canCreate,
    canEdit,
    canDelete,
    canApprove,
    canExport,
    canMenu,
    hasAnyPermission,
    refetchPermissions: fetchPermissions,
  };
};

// Helper hook untuk specific module
export const useModulePermissions = (module: string) => {
  const {
    permissions,
    loading,
    error,
    canView,
    canCreate,
    canEdit,
    canDelete,
    canApprove,
    canExport,
    canMenu,
    hasAnyPermission,
    refetchPermissions,
  } = usePermissions();

  return {
    permissions: permissions?.[module] || DEFAULT_PERMISSION,
    loading,
    error,
    canView: canView(module),
    canCreate: canCreate(module),
    canEdit: canEdit(module),
    canDelete: canDelete(module),
    canApprove: canApprove(module),
    canExport: canExport(module),
    canMenu: canMenu(module),
    hasAnyPermission: hasAnyPermission(module),
    refetchPermissions,
  };
};
