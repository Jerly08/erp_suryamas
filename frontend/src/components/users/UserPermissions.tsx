import React, { useState, useEffect } from 'react';
import api from '@/services/api';
import { handleApiError } from '@/utils/authErrorHandler';
import { API_ENDPOINTS } from '@/config/api';
import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalFooter,
  ModalBody,
  ModalCloseButton,
  Button,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  Checkbox,
  Box,
  Text,
  VStack,
  HStack,
  Badge,
  useToast,
  Spinner,
  Alert,
  AlertIcon,
  Tooltip,
  IconButton,
  Divider,
} from '@chakra-ui/react';
import { FiRefreshCw, FiSave } from 'react-icons/fi';
import { formatRoleForApproval } from '@/utils/roles';

interface Permission {
  can_view: boolean;
  can_create: boolean;
  can_edit: boolean;
  can_delete: boolean;
  can_approve: boolean;
  can_export: boolean;
  can_menu: boolean;
}

interface UserPermission {
  user_id: number;
  username: string;
  email: string;
  role: string;
  permissions: {
    [key: string]: Permission;
  };
}

interface UserPermissionsProps {
  isOpen: boolean;
  onClose: () => void;
  user: {
    id: number;
    username: string;
    email: string;
    full_name: string;
    role: string;
  } | null;
}

const modules = [
  { key: 'accounts', label: 'Accounts', description: 'Chart of Accounts & Accounting', actions: ['view', 'create', 'edit', 'delete', 'export', 'menu'] },
  { key: 'products', label: 'Products', description: 'Product & Inventory Management', actions: ['view', 'create', 'edit', 'delete', 'export', 'menu'] },
  { key: 'boms', label: 'Master BOMs', description: 'Bill of Materials Templates', actions: ['view', 'create', 'edit', 'delete', 'menu'] },
  { key: 'projects', label: 'Projects', description: 'Project Management & Budgets', actions: ['view', 'create', 'edit', 'delete', 'menu'] },
  { key: 'contacts', label: 'Contacts', description: 'Customer, Vendor & Employee', actions: ['view', 'create', 'edit', 'delete', 'export', 'menu'] },
  { key: 'assets', label: 'Assets', description: 'Fixed Assets Management', actions: ['view', 'create', 'edit', 'delete', 'export', 'menu'] },
  { key: 'prepaid_expenses', label: 'Prepaid Expenses', description: 'Amortization & Prepaid Tracking', actions: ['view', 'create', 'edit', 'delete', 'export', 'menu'] },
  { key: 'purchase_requests', label: 'Purchase Requests', description: 'Purchase Requests & Approvals', actions: ['view', 'create', 'edit', 'delete', 'approve', 'export', 'menu'] },
  { key: 'sales', label: 'Sales', description: 'Sales & Invoicing', actions: ['view', 'create', 'edit', 'delete', 'approve', 'export', 'menu'] },
  { key: 'purchases', label: 'Purchases', description: 'Purchase Orders & Receipts', actions: ['view', 'create', 'edit', 'delete', 'approve', 'export', 'menu'] },
  { key: 'payments', label: 'Payments', description: 'Payment Processing', actions: ['view', 'create', 'edit', 'delete', 'approve', 'export', 'menu'] },
  { key: 'cash_bank', label: 'Cash & Bank', description: 'Cash & Bank Management', actions: ['view', 'create', 'edit', 'delete', 'approve', 'export', 'menu'] },
  { key: 'journal', label: 'Journal', description: 'General Journal & Journal Entries', actions: ['view', 'create', 'edit', 'delete', 'export', 'menu'] },
  { key: 'reports', label: 'Reports', description: 'Financial & Business Reports', actions: ['view', 'create', 'edit', 'delete', 'export', 'menu'] },
  { key: 'users', label: 'Users', description: 'User Management (Admin Only)', actions: ['view', 'create', 'edit', 'delete', 'menu'] },
  { key: 'settings', label: 'Settings', description: 'System Settings & Invoice Types Management', actions: ['view', 'create', 'edit', 'delete', 'menu'] },
];

const UserPermissions: React.FC<UserPermissionsProps> = ({ isOpen, onClose, user }) => {
  const [permissions, setPermissions] = useState<UserPermission | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [originalPermissions, setOriginalPermissions] = useState<UserPermission | null>(null);
  const toast = useToast();

  useEffect(() => {
    if (user && isOpen) {
      fetchUserPermissions();
    }
  }, [user, isOpen]);

  const fetchUserPermissions = async () => {
    if (!user) return;

    setLoading(true);
    try {
      const response = await api.get(API_ENDPOINTS.PERMISSIONS_USER_BY_ID(user.id));
      const data = response.data;

      setPermissions(data);
      setOriginalPermissions(JSON.parse(JSON.stringify(data))); // Deep copy
    } catch (error: any) {
      console.error('Error fetching user permissions:', error);

      const errorResult = handleApiError(error, 'UserPermissions.fetchUserPermissions');

      toast({
        title: 'Error',
        description: errorResult.message,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  const handlePermissionChange = (module: string, permissionType: string, value: boolean) => {
    if (!permissions) return;

    setPermissions({
      ...permissions,
      permissions: {
        ...permissions.permissions,
        [module]: {
          ...permissions.permissions[module],
          [permissionType]: value,
        },
      },
    });
  };

  const handleToggleAll = (moduleKey: string, value: boolean, moduleActions: string[]) => {
    if (!permissions) return;

    const newPerms = { ...permissions.permissions[moduleKey] };
    if (moduleActions.includes('view')) newPerms.can_view = value;
    if (moduleActions.includes('create')) newPerms.can_create = value;
    if (moduleActions.includes('edit')) newPerms.can_edit = value;
    if (moduleActions.includes('delete')) newPerms.can_delete = value;
    if (moduleActions.includes('approve')) newPerms.can_approve = value;
    if (moduleActions.includes('export')) newPerms.can_export = value;
    if (moduleActions.includes('menu')) newPerms.can_menu = value;

    setPermissions({
      ...permissions,
      permissions: {
        ...permissions.permissions,
        [moduleKey]: {
          ...permissions.permissions[moduleKey],
          ...newPerms,
        },
      },
    });
  };

  const handleSavePermissions = async () => {
    if (!user || !permissions) return;

    setSaving(true);
    try {
      await api.put(API_ENDPOINTS.PERMISSIONS_USER_BY_ID(user.id), {
        permissions: permissions.permissions,
      });

      toast({
        title: 'Success',
        description: 'Permissions updated successfully',
        status: 'success',
        duration: 3000,
        isClosable: true,
      });

      setOriginalPermissions(JSON.parse(JSON.stringify(permissions)));
    } catch (error: any) {
      console.error('Error updating user permissions:', error);

      const errorResult = handleApiError(error, 'UserPermissions.handleSavePermissions');

      toast({
        title: 'Error',
        description: errorResult.message,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setSaving(false);
    }
  };

  const handleResetToDefault = async () => {
    if (!user || !window.confirm('Reset permissions to default based on role? This cannot be undone.')) return;

    setSaving(true);
    try {
      await api.post(API_ENDPOINTS.PERMISSIONS_USER_RESET(user.id));

      toast({
        title: 'Success',
        description: 'Permissions reset to default',
        status: 'success',
        duration: 3000,
        isClosable: true,
      });

      // Refresh permissions
      fetchUserPermissions();
    } catch (error: any) {
      console.error('Error resetting user permissions:', error);

      const errorResult = handleApiError(error, 'UserPermissions.handleResetToDefault');

      toast({
        title: 'Error',
        description: errorResult.message,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setSaving(false);
    }
  };

  const hasChanges = () => {
    if (!permissions || !originalPermissions) return false;
    return JSON.stringify(permissions.permissions) !== JSON.stringify(originalPermissions.permissions);
  };

  const getRoleBadgeColor = (role: string) => {
    switch (role.toLowerCase()) {
      case 'admin': return 'red';
      case 'finance': return 'purple';
      case 'finance_manager': return 'purple';
      case 'finance_staff': return 'orange';
      case 'inventory_manager': return 'blue';
      case 'director': return 'green';
      case 'employee': return 'gray';
      default: return 'gray';
    }
  };

  if (!user) return null;

  return (
    <Modal isOpen={isOpen} onClose={onClose} size="6xl">
      <ModalOverlay />
      <ModalContent maxW="90vw">
        <ModalHeader>
          <VStack align="start" spacing={2}>
            <HStack>
              <Text>Manage Permissions for</Text>
              <Text fontWeight="bold">{user.full_name}</Text>
              <Badge colorScheme={getRoleBadgeColor(user.role)}>{formatRoleForApproval(user.role)}</Badge>
            </HStack>
            <Text fontSize="sm" color="gray.600">
              Configure module-specific permissions for create, edit, delete, and other actions
            </Text>
          </VStack>
        </ModalHeader>
        <ModalCloseButton />

        <ModalBody>
          {loading ? (
            <Box textAlign="center" py={10}>
              <Spinner size="xl" />
              <Text mt={4}>Loading permissions...</Text>
            </Box>
          ) : permissions ? (
            <Box>
              <Alert status="info" mb={4}>
                <AlertIcon />
                <Text fontSize="sm">
                  Permissions control what actions users can perform in each module.
                  View permission is required for all other permissions to work.
                  Menu permission controls whether users can access the module through navigation menus.
                </Text>
              </Alert>

              <Box overflowX="auto">
                <Table variant="simple" size="sm">
                  <Thead>
                    <Tr bg="gray.50">
                      <Th width="250px">Module</Th>
                      <Th textAlign="center">View</Th>
                      <Th textAlign="center">Create</Th>
                      <Th textAlign="center">Edit</Th>
                      <Th textAlign="center">Delete</Th>
                      <Th textAlign="center">Approve</Th>
                      <Th textAlign="center">Export</Th>
                      <Th textAlign="center">Menu</Th>
                      <Th textAlign="center">Actions</Th>
                    </Tr>
                  </Thead>
                  <Tbody>
                    {modules.map((module) => {
                      const perm = permissions.permissions[module.key] || {
                        can_view: false,
                        can_create: false,
                        can_edit: false,
                        can_delete: false,
                        can_approve: false,
                        can_export: false,
                        can_menu: false,
                      };

                      const activePermsCount = module.actions.reduce((count, action) => {
                        const val = action === 'menu' ? perm.can_menu
                          : action === 'approve' ? perm.can_approve
                            : action === 'export' ? perm.can_export
                              : perm[`can_${action}` as keyof typeof perm] as boolean;
                        return count + (val ? 1 : 0);
                      }, 0);

                      const allChecked = activePermsCount === module.actions.length;
                      const someChecked = activePermsCount > 0 && activePermsCount < module.actions.length;

                      return (
                        <Tr key={module.key} _hover={{ bg: 'gray.50' }}>
                          <Td>
                            <VStack align="start" spacing={0}>
                              <Text fontWeight="medium">{module.label}</Text>
                              <Text fontSize="xs" color="gray.500">{module.description}</Text>
                            </VStack>
                          </Td>
                          <Td textAlign="center">
                            {module.actions.includes('view') && (
                              <Checkbox
                                isChecked={perm.can_view}
                                onChange={(e) => handlePermissionChange(module.key, 'can_view', e.target.checked)}
                              />
                            )}
                          </Td>
                          <Td textAlign="center">
                            {module.actions.includes('create') && (
                              <Checkbox
                                isChecked={perm.can_create}
                                onChange={(e) => handlePermissionChange(module.key, 'can_create', e.target.checked)}
                                isDisabled={!perm.can_view}
                              />
                            )}
                          </Td>
                          <Td textAlign="center">
                            {module.actions.includes('edit') && (
                              <Checkbox
                                isChecked={perm.can_edit}
                                onChange={(e) => handlePermissionChange(module.key, 'can_edit', e.target.checked)}
                                isDisabled={!perm.can_view}
                              />
                            )}
                          </Td>
                          <Td textAlign="center">
                            {module.actions.includes('delete') && (
                              <Checkbox
                                isChecked={perm.can_delete}
                                onChange={(e) => handlePermissionChange(module.key, 'can_delete', e.target.checked)}
                                isDisabled={!perm.can_view}
                              />
                            )}
                          </Td>
                          <Td textAlign="center">
                            {module.actions.includes('approve') && (
                              <Checkbox
                                isChecked={perm.can_approve}
                                onChange={(e) => handlePermissionChange(module.key, 'can_approve', e.target.checked)}
                                isDisabled={!perm.can_view}
                              />
                            )}
                          </Td>
                          <Td textAlign="center">
                            {module.actions.includes('export') && (
                              <Checkbox
                                isChecked={perm.can_export}
                                onChange={(e) => handlePermissionChange(module.key, 'can_export', e.target.checked)}
                                isDisabled={!perm.can_view}
                              />
                            )}
                          </Td>
                          <Td textAlign="center">
                            {module.actions.includes('menu') && (
                              <Checkbox
                                isChecked={perm.can_menu}
                                onChange={(e) => handlePermissionChange(module.key, 'can_menu', e.target.checked)}
                                isDisabled={!perm.can_view}
                              />
                            )}
                          </Td>
                          <Td textAlign="center">
                            <HStack justify="center">
                              <Button
                                size="xs"
                                variant={allChecked ? "solid" : someChecked ? "outline" : "ghost"}
                                colorScheme={allChecked ? "green" : "gray"}
                                onClick={() => handleToggleAll(module.key, !allChecked, module.actions)}
                              >
                                {allChecked ? 'All' : someChecked ? 'Some' : 'None'}
                              </Button>
                            </HStack>
                          </Td>
                        </Tr>
                      );
                    })}
                  </Tbody>
                </Table>
              </Box>

              <Divider my={4} />

              <HStack justify="space-between">
                <Text fontSize="sm" color="gray.600">
                  * Admin role typically has full access to all modules
                </Text>
                <HStack>
                  <Tooltip label="Reset to default permissions based on role">
                    <IconButton
                      aria-label="Reset to default"
                      icon={<FiRefreshCw />}
                      size="sm"
                      variant="outline"
                      onClick={handleResetToDefault}
                      isLoading={saving}
                    />
                  </Tooltip>
                </HStack>
              </HStack>
            </Box>
          ) : (
            <Alert status="error">
              <AlertIcon />
              Failed to load permissions
            </Alert>
          )}
        </ModalBody>

        <ModalFooter>
          <Button variant="ghost" mr={3} onClick={onClose}>
            Cancel
          </Button>
          <Button
            colorScheme="blue"
            onClick={handleSavePermissions}
            isLoading={saving}
            loadingText="Saving..."
            leftIcon={<FiSave />}
            isDisabled={!hasChanges()}
          >
            Save Permissions
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
};

export default UserPermissions;
