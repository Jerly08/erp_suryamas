'use client';

import React, { useEffect, useState } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useTranslation } from '@/hooks/useTranslation';
import { useModulePermissions } from '@/hooks/usePermissions';
import ProtectedModule from '@/components/common/ProtectedModule';
import SimpleLayout from '@/components/layout/SimpleLayout';
import { DataTable } from '@/components/common/DataTable';
import {
  Box,
  Heading,
  Text,
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
  Spinner,
  Button,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalFooter,
  ModalBody,
  ModalCloseButton,
  FormControl,
  FormLabel,
  Input,
  Select,
  useDisclosure,
  useToast,
  VStack,
  HStack,
  IconButton,
  Tooltip,
  useColorModeValue
} from '@chakra-ui/react';
import { FiEye, FiEdit, FiTrash2, FiShield } from 'react-icons/fi';
import UserPermissions from '@/components/users/UserPermissions';

interface User {
  id: number;
  username: string;
  email: string;
  full_name: string;
  first_name?: string;
  last_name?: string;
  role: string;
  is_active: boolean;
  phone?: string;
  department?: string;
  position?: string;
  created_at: string;
  updated_at: string;
  created_at_display?: string;
  status?: string;
}

interface CreateUserForm {
  username: string;
  email: string;
  password: string;
  confirmPassword: string;
  first_name: string;
  last_name: string;
  role: string;
  department: string;
  position: string;
  phone: string;
}

// Columns will be defined dynamically in the component

const UsersPage: React.FC = () => {
  const { user: authUser } = useAuth();
  const { t } = useTranslation();
  const userPermissions = useModulePermissions('users');
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const { isOpen, onOpen, onClose } = useDisclosure();
  const toast = useToast();

  // State for view modal
  const [isViewModalOpen, setIsViewModalOpen] = useState(false);
  const [viewUser, setViewUser] = useState<User | null>(null);

  // State for edit modal
  const [isEditMode, setIsEditMode] = useState(false);
  const [selectedUser, setSelectedUser] = useState<User | null>(null);

  // State for permissions modal
  const [isPermissionsModalOpen, setIsPermissionsModalOpen] = useState(false);
  const [permissionsUser, setPermissionsUser] = useState<User | null>(null);

  const [formData, setFormData] = useState<CreateUserForm>({
    username: '',
    email: '',
    password: '',
    confirmPassword: '',
    first_name: '',
    last_name: '',
    role: 'employee',
    department: '',
    position: '',
    phone: '',
  });

  const headingColor = useColorModeValue('gray.800', 'whiteAlpha.900');
  const textColor = useColorModeValue('gray.700', 'whiteAlpha.900');
  const mutedTextColor = useColorModeValue('gray.500', 'whiteAlpha.700');
  const cardBg = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const modalBg = useColorModeValue('white', 'gray.800');
  const modalHeaderBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const modalFooterBg = useColorModeValue('gray.50', 'whiteAlpha.50');
  const inputBg = useColorModeValue('white', 'whiteAlpha.50');
  const inputBorderColor = useColorModeValue('gray.300', 'whiteAlpha.300');
  const detailCardBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const fieldFocusStyles = {
    borderColor: 'blue.400',
    boxShadow: '0 0 0 1px var(--chakra-colors-blue-400)',
  };

  const mapUserForTable = (rawUser: any): User => ({
    ...rawUser,
    status: rawUser.is_active ? t('common.active') : t('common.inactive'),
    created_at_display: rawUser.created_at ? new Date(rawUser.created_at).toLocaleDateString() : '-',
  });

  const loadUsers = async () => {
    const res = await fetch('/api/v1/users', {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('token')}`,
      },
    });

    if (!res.ok) {
      throw new Error('Failed to fetch users data');
    }

    const response = await res.json();
    const usersData = response.data || [];
    setUsers(usersData.map(mapUserForTable));
  };

  useEffect(() => {
    const fetchUsers = async () => {
      try {
        await loadUsers();
      } catch (err: any) {
        setError(err.message || 'An error occurred');
      } finally {
        setLoading(false);
      }
    };

    fetchUsers();
  }, []);

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value } = e.target;
    setFormData(prev => ({
      ...prev,
      [name]: value
    }));
  };

  const resetForm = () => {
    setFormData({
      username: '',
      email: '',
      password: '',
      confirmPassword: '',
      first_name: '',
      last_name: '',
      role: 'employee',
      department: '',
      position: '',
      phone: '',
    });
  };

  // handleCreateUser is now replaced by handleSaveUser above

  const handleModalClose = () => {
    resetForm();
    setIsEditMode(false);
    setSelectedUser(null);
    onClose();
  };

  // Handle view user
  const handleView = (user: User) => {
    setViewUser(user);
    setIsViewModalOpen(true);
  };

  // Handle permissions
  const handlePermissions = (user: User) => {
    setPermissionsUser(user);
    setIsPermissionsModalOpen(true);
  };

  // Handle edit user
  const handleEdit = (user: User) => {
    setSelectedUser(user);
    setFormData({
      username: user.username,
      email: user.email,
      password: '', // Leave empty for editing
      confirmPassword: '',
      first_name: user.first_name || user.full_name.split(' ')[0] || '',
      last_name: user.last_name || user.full_name.split(' ').slice(1).join(' ') || '',
      role: user.role,
      department: user.department || '',
      position: user.position || '',
      phone: user.phone || '',
    });
    setIsEditMode(true);
    onOpen();
  };

  // Handle delete user
  const handleDelete = async (userId: number) => {
    if (String(authUser?.id) === String(userId)) {
      toast({
        title: t('common.error'),
        description: 'You cannot delete your own account',
        status: 'error',
        duration: 4000,
        isClosable: true,
      });
      return;
    }

    if (!window.confirm(t('messages.confirmDelete'))) {
      return;
    }

    try {
      const response = await fetch(`/api/v1/users/${userId}`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => null);
        throw new Error(errorData?.error || 'Failed to delete user');
      }

      await loadUsers();

      toast({
        title: t('messages.success'),
        description: t('messages.dataDeleted'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    } catch (error: any) {
      toast({
        title: t('common.error'),
        description: error.message,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    }
  };

  // Update handleCreateUser to handle both create and edit
  const handleSaveUser = async () => {
    // Validation
    if (!formData.username.trim()) {
      toast({
        title: 'Error',
        description: 'Username is required',
        status: 'error',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    if (!formData.email.trim()) {
      toast({
        title: 'Error',
        description: 'Email is required',
        status: 'error',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    // Only validate password for new users
    if (!isEditMode) {
      if (!formData.password.trim() || formData.password.length < 6) {
        toast({
          title: 'Error',
          description: 'Password must be at least 6 characters',
          status: 'error',
          duration: 3000,
          isClosable: true,
        });
        return;
      }

      if (formData.password !== formData.confirmPassword) {
        toast({
          title: 'Error',
          description: 'Passwords do not match',
          status: 'error',
          duration: 3000,
          isClosable: true,
        });
        return;
      }
    }

    setCreating(true);

    try {
      const { confirmPassword, ...createData } = formData;

      // Remove password fields if editing and password is empty
      if (isEditMode && !createData.password) {
        delete (createData as any).password;
      }

      const url = isEditMode
        ? `/api/v1/users/${selectedUser?.id}`
        : '/api/v1/users';

      const response = await fetch(url, {
        method: isEditMode ? 'PUT' : 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
        body: JSON.stringify(createData),
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || `Failed to ${isEditMode ? 'update' : 'create'} user`);
      }

      await response.json();

      await loadUsers();

      toast({
        title: t('messages.success'),
        description: isEditMode ? t('messages.dataUpdated') : t('messages.dataSaved'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });

      handleModalClose();
    } catch (error: any) {
      toast({
        title: 'Error',
        description: error.message,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setCreating(false);
    }
  };

  if (loading) {
    return (
      <ProtectedModule module="users">
        <SimpleLayout>
          <Box display="flex" alignItems="center" color={textColor}>
            <Spinner size="xl" thickness="4px" speed="0.65s" color="blue.500" />
            <Text ml={4}>{t('common.loading')}</Text>
          </Box>
        </SimpleLayout>
      </ProtectedModule>
    );
  }

  // Define columns with action buttons
  const columns: any[] = [
    { header: t('users.name'), accessor: 'full_name' },
    { header: t('users.email'), accessor: 'email' },
    { header: t('users.role'), accessor: 'role' },
    { header: t('users.status'), accessor: 'status' },
    { header: t('users.created'), accessor: 'created_at_display' },
    {
      header: t('users.actions'),
      accessor: (user: User) => (
        <HStack spacing={2}>
          {userPermissions.canView && (
          <Tooltip label="View Details" placement="top">
            <IconButton
              aria-label="View user"
              icon={<FiEye />}
              size="sm"
              variant="outline"
              colorScheme="blue"
              onClick={() => handleView(user)}
            />
          </Tooltip>
          )}
          {userPermissions.canEdit && (
          <Tooltip label="Edit User" placement="top">
            <IconButton
              aria-label="Edit user"
              icon={<FiEdit />}
              size="sm"
              variant="outline"
              colorScheme="green"
              onClick={() => handleEdit(user)}
            />
          </Tooltip>
          )}
          {userPermissions.canEdit && (
          <Tooltip label="Manage Permissions" placement="top">
            <IconButton
              aria-label="Manage permissions"
              icon={<FiShield />}
              size="sm"
              variant="outline"
              colorScheme="purple"
              onClick={() => handlePermissions(user)}
            />
          </Tooltip>
          )}
          {userPermissions.canDelete && (
          <Tooltip label={String(authUser?.id) === String(user.id) ? 'You cannot delete your own account' : 'Delete User'} placement="top">
            <IconButton
              aria-label="Delete user"
              icon={<FiTrash2 />}
              size="sm"
              variant="outline"
              colorScheme="red"
              isDisabled={String(authUser?.id) === String(user.id)}
              onClick={() => handleDelete(user.id)}
            />
          </Tooltip>
          )}
        </HStack>
      ),
    },
  ];

  return (
    <ProtectedModule module="users">
      <SimpleLayout>
        <Box>
          <HStack justify="space-between" align="center" mb={6}>
            <Heading as="h1" size="xl" color={headingColor}>{t('users.management')}</Heading>
            {userPermissions.canCreate && (
              <Button
                colorScheme="blue"
                onClick={() => {
                  setIsEditMode(false);
                  resetForm();
                  onOpen();
                }}
                size="md"
              >
                {t('common.createAccount') || "Create Account"}
              </Button>
            )}
          </HStack>

          {error && (
            <Alert status="error" mb={4}>
              <AlertIcon />
              <AlertTitle>Error:</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          <Box bg={cardBg} borderRadius="lg" overflow="hidden" boxShadow="sm" borderWidth="1px" borderColor={borderColor}>
            <DataTable
              columns={columns}
              data={users}
              keyField="id"
              title="Users List"
              searchable={true}
              pagination={true}
              pageSize={10}
            />
          </Box>

          {/* Create/Edit User Modal */}
          <Modal isOpen={isOpen} onClose={handleModalClose} size="xl">
            <ModalOverlay />
            <ModalContent bg={modalBg} borderWidth="1px" borderColor={borderColor}>
              <ModalHeader bg={modalHeaderBg} borderBottomWidth="1px" borderColor={borderColor}>{isEditMode ? t('users.editAccount') : t('users.createAccount')}</ModalHeader>
              <ModalCloseButton color={textColor} />
              <ModalBody>
                <VStack spacing={4}>
                  <HStack spacing={4} width="100%">
                    <FormControl isRequired>
                      <FormLabel color={textColor}>{t('users.firstName')}</FormLabel>
                      <Input
                        name="first_name"
                        value={formData.first_name}
                        onChange={handleInputChange}
                        placeholder={t('users.placeholders.enterFirstName') || "Enter first name"}
                        bg={inputBg}
                        borderColor={inputBorderColor}
                        color={textColor}
                        _hover={{ borderColor: inputBorderColor }}
                        _focusVisible={fieldFocusStyles}
                        _placeholder={{ color: mutedTextColor }}
                      />
                    </FormControl>
                    <FormControl isRequired>
                      <FormLabel color={textColor}>{t('users.lastName')}</FormLabel>
                      <Input
                        name="last_name"
                        value={formData.last_name}
                        onChange={handleInputChange}
                        placeholder={t('users.placeholders.enterLastName') || "Enter last name"}
                        bg={inputBg}
                        borderColor={inputBorderColor}
                        color={textColor}
                        _hover={{ borderColor: inputBorderColor }}
                        _focusVisible={fieldFocusStyles}
                        _placeholder={{ color: mutedTextColor }}
                      />
                    </FormControl>
                  </HStack>

                  <HStack spacing={4} width="100%">
                    <FormControl isRequired>
                      <FormLabel color={textColor}>{t('users.username')}</FormLabel>
                      <Input
                        name="username"
                        value={formData.username}
                        onChange={handleInputChange}
                        placeholder={t('users.placeholders.enterUsername') || "Enter username"}
                        bg={inputBg}
                        borderColor={inputBorderColor}
                        color={textColor}
                        _hover={{ borderColor: inputBorderColor }}
                        _focusVisible={fieldFocusStyles}
                        _placeholder={{ color: mutedTextColor }}
                      />
                    </FormControl>
                    <FormControl isRequired>
                      <FormLabel color={textColor}>Email</FormLabel>
                      <Input
                        type="email"
                        name="email"
                        value={formData.email}
                        onChange={handleInputChange}
                        placeholder="Enter email address"
                        bg={inputBg}
                        borderColor={inputBorderColor}
                        color={textColor}
                        _hover={{ borderColor: inputBorderColor }}
                        _focusVisible={fieldFocusStyles}
                        _placeholder={{ color: mutedTextColor }}
                      />
                    </FormControl>
                  </HStack>

                  <HStack spacing={4} width="100%">
                    <FormControl isRequired={!isEditMode}>
                      <FormLabel color={textColor}>{t('users.password')} {isEditMode && t('users.leaveEmpty')}</FormLabel>
                      <Input
                        type="password"
                        name="password"
                        value={formData.password}
                        onChange={handleInputChange}
                        placeholder={isEditMode ? t('users.placeholders.leaveEmpty') : t('users.placeholders.enterPassword')}
                        bg={inputBg}
                        borderColor={inputBorderColor}
                        color={textColor}
                        _hover={{ borderColor: inputBorderColor }}
                        _focusVisible={fieldFocusStyles}
                        _placeholder={{ color: mutedTextColor }}
                      />
                    </FormControl>
                    <FormControl isRequired={!isEditMode}>
                      <FormLabel color={textColor}>{t('users.confirmPassword')}</FormLabel>
                      <Input
                        type="password"
                        name="confirmPassword"
                        value={formData.confirmPassword}
                        onChange={handleInputChange}
                        placeholder={t('users.placeholders.confirmPassword') || "Confirm password"}
                        bg={inputBg}
                        borderColor={inputBorderColor}
                        color={textColor}
                        _hover={{ borderColor: inputBorderColor }}
                        _focusVisible={fieldFocusStyles}
                        _placeholder={{ color: mutedTextColor }}
                      />
                    </FormControl>
                  </HStack>

                  <HStack spacing={4} width="100%">
                    <FormControl isRequired>
                      <FormLabel color={textColor}>Role</FormLabel>
                      <Select
                        name="role"
                        value={formData.role}
                        onChange={handleInputChange}
                        bg={inputBg}
                        borderColor={inputBorderColor}
                        color={textColor}
                        _hover={{ borderColor: inputBorderColor }}
                        _focusVisible={fieldFocusStyles}
                      >
                        <option value="employee">{t('users.roles.employee') || "Employee"}</option>
                        <option value="admin">{t('users.roles.admin') || "Administrator"}</option>
                        <option value="finance">{t('users.roles.finance') || "Finance"}</option>
                        <option value="director">{t('users.roles.director') || "Director"}</option>
                        <option value="inventory_manager">{t('users.roles.inventoryManager') || "Inventory Manager"}</option>
                      </Select>
                    </FormControl>
                    <FormControl>
                      <FormLabel color={textColor}>Phone</FormLabel>
                      <Input
                        name="phone"
                        value={formData.phone}
                        onChange={handleInputChange}
                        placeholder="Enter phone number"
                        bg={inputBg}
                        borderColor={inputBorderColor}
                        color={textColor}
                        _hover={{ borderColor: inputBorderColor }}
                        _focusVisible={fieldFocusStyles}
                        _placeholder={{ color: mutedTextColor }}
                      />
                    </FormControl>
                  </HStack>

                  <HStack spacing={4} width="100%">
                    <FormControl>
                      <FormLabel color={textColor}>Department</FormLabel>
                      <Input
                        name="department"
                        value={formData.department}
                        onChange={handleInputChange}
                        placeholder="Enter department"
                        bg={inputBg}
                        borderColor={inputBorderColor}
                        color={textColor}
                        _hover={{ borderColor: inputBorderColor }}
                        _focusVisible={fieldFocusStyles}
                        _placeholder={{ color: mutedTextColor }}
                      />
                    </FormControl>
                    <FormControl>
                      <FormLabel color={textColor}>Position</FormLabel>
                      <Input
                        name="position"
                        value={formData.position}
                        onChange={handleInputChange}
                        placeholder="Enter position"
                        bg={inputBg}
                        borderColor={inputBorderColor}
                        color={textColor}
                        _hover={{ borderColor: inputBorderColor }}
                        _focusVisible={fieldFocusStyles}
                        _placeholder={{ color: mutedTextColor }}
                      />
                    </FormControl>
                  </HStack>
                </VStack>
              </ModalBody>

              <ModalFooter bg={modalFooterBg} borderTopWidth="1px" borderColor={borderColor}>
                <Button variant="ghost" mr={3} onClick={handleModalClose}>
                  {t('common.cancel')}
                </Button>
                <Button
                  colorScheme="blue"
                  onClick={handleSaveUser}
                  isLoading={creating}
                  loadingText={isEditMode ? t('common.updating') : t('common.creating')}
                >
                  {isEditMode ? t('users.updateAccount') : t('users.createAccount')}
                </Button>
              </ModalFooter>
            </ModalContent>
          </Modal>

          {/* View User Modal */}
          <Modal isOpen={isViewModalOpen} onClose={() => setIsViewModalOpen(false)} size="lg">
            <ModalOverlay />
            <ModalContent bg={modalBg} borderWidth="1px" borderColor={borderColor}>
              <ModalHeader bg={modalHeaderBg} borderBottomWidth="1px" borderColor={borderColor}>{t('users.userDetails')}</ModalHeader>
              <ModalCloseButton color={textColor} />
              <ModalBody>
                {viewUser && (
                  <VStack spacing={3} align="stretch">
                    <Box bg={detailCardBg} borderWidth="1px" borderColor={borderColor} borderRadius="md" p={4}>
                      <Text fontSize="sm" color={mutedTextColor} mb={1}>{t('users.fullName')}</Text>
                      <Text color={textColor}>{viewUser.full_name}</Text>
                    </Box>
                    <Box bg={detailCardBg} borderWidth="1px" borderColor={borderColor} borderRadius="md" p={4}>
                      <Text fontSize="sm" color={mutedTextColor} mb={1}>{t('users.username')}</Text>
                      <Text color={textColor}>{viewUser.username}</Text>
                    </Box>
                    <Box bg={detailCardBg} borderWidth="1px" borderColor={borderColor} borderRadius="md" p={4}>
                      <Text fontSize="sm" color={mutedTextColor} mb={1}>{t('users.email')}</Text>
                      <Text color={textColor}>{viewUser.email}</Text>
                    </Box>
                    <Box bg={detailCardBg} borderWidth="1px" borderColor={borderColor} borderRadius="md" p={4}>
                      <Text fontSize="sm" color={mutedTextColor} mb={1}>{t('users.role')}</Text>
                      <Text color={textColor}>{viewUser.role}</Text>
                    </Box>
                    <Box bg={detailCardBg} borderWidth="1px" borderColor={borderColor} borderRadius="md" p={4}>
                      <Text fontSize="sm" color={mutedTextColor} mb={1}>{t('users.status')}</Text>
                      <Text color={textColor}>{viewUser.is_active ? t('common.active') : t('common.inactive')}</Text>
                    </Box>
                    <Box bg={detailCardBg} borderWidth="1px" borderColor={borderColor} borderRadius="md" p={4}>
                      <Text fontSize="sm" color={mutedTextColor} mb={1}>{t('users.createdDate')}</Text>
                      <Text color={textColor}>{new Date(viewUser.created_at).toLocaleString()}</Text>
                    </Box>
                    <Box bg={detailCardBg} borderWidth="1px" borderColor={borderColor} borderRadius="md" p={4}>
                      <Text fontSize="sm" color={mutedTextColor} mb={1}>{t('users.lastUpdated')}</Text>
                      <Text color={textColor}>{new Date(viewUser.updated_at).toLocaleString()}</Text>
                    </Box>
                  </VStack>
                )}
              </ModalBody>
              <ModalFooter bg={modalFooterBg} borderTopWidth="1px" borderColor={borderColor}>
                <Button onClick={() => setIsViewModalOpen(false)}>{t('common.close')}</Button>
              </ModalFooter>
            </ModalContent>
          </Modal>

          {/* Permissions Modal */}
          <UserPermissions
            isOpen={isPermissionsModalOpen}
            onClose={() => {
              setIsPermissionsModalOpen(false);
              setPermissionsUser(null);
            }}
            user={permissionsUser}
          />
        </Box>
      </SimpleLayout>
    </ProtectedModule>
  );
};

export default UsersPage;
