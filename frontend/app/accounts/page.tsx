'use client';

import React, { useState, useEffect } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useModulePermissions } from '@/hooks/usePermissions';
import { useTranslation } from '@/hooks/useTranslation';
import SimpleLayout from '@/components/layout/SimpleLayout';
import AccountsTable from '@/components/accounts/AccountsTable';
import {
  Box,
  Flex,
  Heading,
  Button,
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalCloseButton,
  useToast,
  Tabs,
  TabList,
  TabPanels,
  Tab,
  TabPanel,
  Input,
  InputGroup,
  InputLeftElement,
  HStack,
  VStack,
  Select,
  Text,
  Badge,
  Spinner,
  Menu,
  MenuButton,
  MenuList,
  MenuItem,
  MenuDivider,
  useColorMode,
  useColorModeValue,
  Tooltip,
  IconButton,
  Popover,
  PopoverTrigger,
  PopoverContent,
  PopoverHeader,
  PopoverBody,
  PopoverArrow,
  PopoverCloseButton,
  UnorderedList,
  ListItem,
  Code,
  Divider,
} from '@chakra-ui/react';
import { FiPlus, FiEdit, FiTrash2, FiDownload, FiSearch, FiSettings, FiInfo, FiHelpCircle } from 'react-icons/fi';
import AccountForm from '@/components/accounts/AccountForm';
import AccountTreeView from '@/components/accounts/AccountTreeView';
import AdminDeleteDialog from '@/components/accounts/AdminDeleteDialog';
import { Account, AccountCreateRequest, AccountUpdateRequest } from '@/types/account';
import accountService from '@/services/accountService';
import { API_ENDPOINTS, API_BASE_URL } from '@/config/api'; // Ensure API_BASE_URL is imported or use relative path
import CompanyTypeSelection from '@/components/onboarding/CompanyTypeSelection';
import CompanyTypeGuard from '@/components/common/CompanyTypeGuard';
import axios from 'axios';

const AccountsPage = () => {
  const { token } = useAuth();
  const { t } = useTranslation();
  const {
    canView,
    canCreate,
    canEdit,
    canDelete,
    canExport,
    loading: permissionLoading
  } = useModulePermissions('accounts');

  const [hierarchyAccounts, setHierarchyAccounts] = useState<Account[]>([]);
  const [flatAccounts, setFlatAccounts] = useState<Account[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tabIndex, setTabIndex] = useState(0);
  const [searchTerm, setSearchTerm] = useState('');
  const [typeFilter, setTypeFilter] = useState('');


  // Company Type is now handled by CompanyTypeGuard and AuthContext

  const [isModalOpen, setIsModalOpen] = useState(false);
  const [selectedAccount, setSelectedAccount] = useState<Account | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isHeaderMode, setIsHeaderMode] = useState(false); // Track if creating header account
  const [isAdminDeleteOpen, setIsAdminDeleteOpen] = useState(false);
  const [accountToDelete, setAccountToDelete] = useState<Account | null>(null);
  const toast = useToast();

  // Theme-aware colors (hooks must be called unconditionally and before any early returns)
  const headingColor = useColorModeValue('gray.800', 'var(--text-primary)');
  const cardBg = useColorModeValue('white', 'var(--bg-secondary)');
  const borderColor = useColorModeValue('gray.200', 'var(--border-color)');
  const tabBorderColor = useColorModeValue('gray.200', 'var(--border-color)');
  const emptyStateColor = useColorModeValue('gray.500', 'var(--text-secondary)');
  const modalBg = useColorModeValue('white', 'var(--bg-secondary)');
  const modalHeaderTextColor = useColorModeValue('gray.800', 'var(--text-primary)');
  // Move dynamic shadow hook call here to avoid calling hooks inside conditional render branches
  const cardShadow = useColorModeValue('sm', 'var(--shadow)');

  // Helper function to get balance for display
  const getDisplayBalance = (account: Account): number => {
    // 🔧 ALWAYS use backend balance - no total_balance calculation
    // Now displaying as positive per accounting principles
    return Math.abs(account.balance);
  };

  // Helper function to flatten hierarchy for List View
  const flattenHierarchy = (accounts: Account[]): Account[] => {
    const result: Account[] = [];

    const flatten = (accounts: Account[], level: number = 0) => {
      accounts.sort((a, b) => a.code.localeCompare(b.code));

      for (const account of accounts) {
        const accountWithLevel = {
          ...account,
          hierarchyLevel: level,
          // Clear children to avoid circular references in JSON
          children: undefined
        };
        result.push(accountWithLevel);

        if (account.children && account.children.length > 0) {
          flatten(account.children, level + 1);
        }
      }
    };

    flatten(accounts);
    return result;
  };

  // 🗑️ REMOVED: All balance modification functions
  // Backend data is used directly without any processing

  // Unified fetch function using only hierarchy endpoint
  const fetchAccountData = async () => {
    if (!token) return;

    setIsLoading(true);
    try {
      // Silently recalculate parent balances before fetching
      try {
        await accountService.recalculateParentBalances(token);
        console.log('✅ Parent balances recalculated');
      } catch (recalcErr) {
        console.warn('⚠️ Parent balance recalculation skipped:', recalcErr);
      }

      const hierarchyData = await accountService.getAccountHierarchy(token);
      const safeHierarchy: Account[] = Array.isArray(hierarchyData) ? hierarchyData : [];
      console.log('📊 Raw Backend Data:', safeHierarchy);

      // 🔧 SIMPLE: Use backend data directly without ANY modifications
      console.log('✅ Using backend data directly (no processing)');

      setHierarchyAccounts(safeHierarchy);
      setFlatAccounts(flattenHierarchy(safeHierarchy));
      setError(null);
    } catch (err: any) {
      setError(err.message || 'Failed to load accounts');
      console.error('Error fetching accounts:', err);
    } finally {
      setIsLoading(false);
    }
  };

  // Load accounts on component mount
  useEffect(() => {
    if (token) {
      fetchAccountData();
    }
  }, [token]);

  const handleSetupSuccess = () => {
    // Refresh page to pick up new settings in AuthContext
    window.location.reload();
  };

  // Handle fix account header status
  const handleFixHeaderStatus = async () => {
    if (!token) return;

    try {
      const response = await fetch(`${API_ENDPOINTS.ACCOUNTS.FIX_HEADER_STATUS}`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        }
      });

      if (!response.ok) {
        throw new Error('Failed to fix header status');
      }

      const result = await response.json();
      console.log('Header status fix result:', result);

      toast({
        title: 'Header Status Fixed',
        description: 'Account hierarchy has been corrected successfully.',
        status: 'success',
        duration: 3000,
        isClosable: true,
      });

      // Refresh accounts list
      fetchAccountData();
    } catch (err: any) {
      console.error('Fix header status error:', err);
      toast({
        title: 'Error',
        description: err.message || 'Failed to fix header status',
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    }
  };

  // Handle form submission for create/update
  const handleSubmit = async (accountData: AccountCreateRequest | AccountUpdateRequest) => {
    console.log('handleSubmit called with data:', accountData);
    console.log('selectedAccount:', selectedAccount);

    setIsSubmitting(true);
    setError(null);

    try {
      if (selectedAccount) {
        // Update existing account
        console.log('Updating account with code:', selectedAccount.code);
        console.log('Update data:', accountData);
        const result = await accountService.updateAccount(token!, selectedAccount.code, accountData as AccountUpdateRequest);
        console.log('Update result:', result);
        toast({
          title: 'Account updated',
          description: 'Account has been updated successfully.',
          status: 'success',
          duration: 3000,
          isClosable: true,
        });
      } else {
        // Create new account
        console.log('Creating new account with data:', accountData);
        const result = await accountService.createAccount(token!, accountData as AccountCreateRequest);
        console.log('Create result:', result);
        toast({
          title: 'Account created',
          description: 'New account has been created successfully.',
          status: 'success',
          duration: 3000,
          isClosable: true,
        });
      }

      // Refresh accounts list
      fetchAccountData();

      // Close modal
      setIsModalOpen(false);
      setSelectedAccount(null);
    } catch (err: any) {
      const errorMessage = err.message || `Error ${selectedAccount ? 'updating' : 'creating'} account`;
      console.error('Submit error:', err);
      setError(errorMessage);
      toast({
        title: 'Error',
        description: errorMessage,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  // Handle regular account deletion
  const handleDelete = async (account: Account) => {
    console.log('Delete account:', account); // Debug log
    if (!window.confirm(`Are you sure you want to delete account "${account.name}"?`)) {
      return;
    }

    try {
      await accountService.deleteAccount(token!, account.code);
      toast({
        title: 'Account deleted',
        description: 'Account has been deleted successfully.',
        status: 'success',
        duration: 3000,
        isClosable: true,
      });

      // Refresh accounts list
      fetchAccountData();
    } catch (err: any) {
      const errorMessage = err.message || 'Error deleting account';
      toast({
        title: 'Error',
        description: errorMessage,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    }
  };

  // Handle admin delete (for header accounts)
  const handleAdminDelete = (account: Account) => {
    console.log('Admin delete account:', account);
    setAccountToDelete(account);
    setIsAdminDeleteOpen(true);
  };

  // Get children of an account
  const getAccountChildren = (parentId: number): Account[] => {
    return flatAccounts.filter(account => account.parent_id === parentId);
  };

  // Perform admin delete with options
  const performAdminDelete = async (options: { cascade_delete: boolean; new_parent_id?: number }) => {
    if (!accountToDelete || !token) return;

    setIsSubmitting(true);
    try {
      const result = await accountService.adminDeleteAccount(token, accountToDelete.code, options);

      toast({
        title: 'Account deleted (Admin)',
        description: `Account has been deleted successfully. ${options.cascade_delete ? 'All child accounts were also deleted.' : 'Child accounts were preserved.'}`,
        status: 'success',
        duration: 5000,
        isClosable: true,
      });

      // Close dialog and refresh data
      setIsAdminDeleteOpen(false);
      setAccountToDelete(null);
      fetchAccountData();
    } catch (err: any) {
      const errorMessage = err.message || 'Error deleting account';
      toast({
        title: 'Admin Delete Error',
        description: errorMessage,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  // Handle download template
  const handleDownloadTemplate = async () => {
    try {
      const blob = await accountService.downloadTemplate();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.style.display = 'none';
      a.href = url;
      a.download = 'accounts_import_template.csv';
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
    } catch (err: any) {
      toast({
        title: 'Download failed',
        description: err.message || 'Failed to download template',
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    }
  };

  // Handle download PDF
  const handleDownloadPDF = async () => {
    try {
      const blob = await accountService.exportAccountsPDF(token!);
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.style.display = 'none';
      a.href = url;
      a.download = `chart_of_accounts_${new Date().toISOString().split('T')[0]}.pdf`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      toast({
        title: 'Download successful',
        description: 'Chart of Accounts PDF has been downloaded successfully.',
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    } catch (err: any) {
      toast({
        title: 'Download failed',
        description: err.message || 'Failed to download PDF',
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    }
  };

  // Handle download Excel
  const handleDownloadExcel = async () => {
    try {
      const blob = await accountService.exportAccountsExcel(token!);
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.style.display = 'none';
      a.href = url;
      a.download = `chart_of_accounts_${new Date().toISOString().split('T')[0]}.xlsx`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      toast({
        title: 'Download successful',
        description: 'Chart of Accounts Excel has been downloaded successfully.',
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    } catch (err: any) {
      toast({
        title: 'Download failed',
        description: err.message || 'Failed to download Excel',
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    }
  };

  // Open modal for creating a new account
  const handleCreate = () => {
    setSelectedAccount(null);
    setIsHeaderMode(false);
    setIsModalOpen(true);
  };

  // Open modal for creating a header account
  const handleCreateHeader = () => {
    setSelectedAccount(null);
    setIsHeaderMode(true);
    setIsModalOpen(true);
  };

  // Open modal for editing an existing account
  const handleEdit = (account: Account) => {
    console.log('Edit account:', account); // Debug log
    setSelectedAccount(account);
    setIsHeaderMode(false); // Reset header mode for edits
    setIsModalOpen(true);
  };

  // Filter accounts based on search and type
  const filteredAccounts = flatAccounts.filter(account => {
    const matchesSearch = account.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      account.code.toLowerCase().includes(searchTerm.toLowerCase());
    const matchesType = !typeFilter || account.type === typeFilter;
    return matchesSearch && matchesType;
  });



  // Use filtered accounts directly since they're already flattened
  const hierarchicalAccounts = filteredAccounts;

  // Show loading while checking permissions
  if (permissionLoading) {
    return (
      <CompanyTypeGuard moduleName="Accounts">
        <SimpleLayout>
          <Flex justify="center" align="center" minH="60vh">
            <VStack spacing={4}>
              <Spinner size="xl" color="blue.500" thickness="4px" />
              <Text>Checking permissions...</Text>
            </VStack>
          </Flex>
        </SimpleLayout>
      </CompanyTypeGuard>
    );
  }



  // If user doesn't have view permission, show access denied
  if (!canView) {
    return (
      <CompanyTypeGuard moduleName="Accounts">
        <SimpleLayout>
          <Alert status="error" borderRadius="md">
            <AlertIcon />
            <AlertTitle mr={2}>Access Denied!</AlertTitle>
            <AlertDescription>
              You don't have permission to view Chart of Accounts. Please contact your administrator.
            </AlertDescription>
          </Alert>
        </SimpleLayout>
      </CompanyTypeGuard>
    );
  }

  return (
    <CompanyTypeGuard moduleName="Accounts">
      <SimpleLayout>
        <Box>
          <Flex justify="space-between" align="center" mb={6}>
            <HStack spacing={3}>
              <Heading size="xl" color={headingColor} fontWeight="600">{t('accounts.chartOfAccounts')}</Heading>
              <Popover placement="bottom-start">
                <PopoverTrigger>
                  <IconButton
                    aria-label="COA Information"
                    icon={<FiHelpCircle />}
                    size="sm"
                    variant="ghost"
                    colorScheme="blue"
                    _hover={{ bg: 'blue.50' }}
                  />
                </PopoverTrigger>
                <PopoverContent maxW="500px" boxShadow="xl">
                  <PopoverArrow />
                  <PopoverCloseButton />
                  <PopoverHeader fontWeight="bold" fontSize="lg">
                    📚 {t('accounts.guide.title')}
                  </PopoverHeader>
                  <PopoverBody>
                    <VStack align="stretch" spacing={3}>
                      <Box>
                        <Text fontWeight="semibold" color="blue.600" mb={2}>🏷️ {t('accounts.guide.codeStructure')}</Text>
                        <UnorderedList spacing={1} fontSize="sm">
                          <ListItem><Code>1xxx</Code> - {t('accounts.guide.assets')}</ListItem>
                          <ListItem><Code>2xxx</Code> - {t('accounts.guide.liabilities')}</ListItem>
                          <ListItem><Code>3xxx</Code> - {t('accounts.guide.equity')}</ListItem>
                          <ListItem><Code>4xxx</Code> - {t('accounts.guide.revenue')}</ListItem>
                          <ListItem><Code>5xxx</Code> - {t('accounts.guide.expenses')}</ListItem>
                        </UnorderedList>
                      </Box>

                      <Divider />

                      <Box>
                        <Text fontWeight="semibold" color="green.600" mb={2}>✅ {t('accounts.guide.requiredAccounts')}</Text>
                        <UnorderedList spacing={1} fontSize="sm">
                          <ListItem><Code>1101</Code> - {t('accounts.guide.cash')}</ListItem>
                          <ListItem><Code>1101-003</Code> - BANK GIRO</ListItem>
                          <ListItem><Code>1102</Code> - {t('accounts.guide.accountsReceivable')} (PIUTANG USAHA)</ListItem>
                          <ListItem><Code>2101</Code> - {t('accounts.guide.accountsPayable')}</ListItem>
                          <ListItem><Code>2102-001</Code> - {t('accounts.guide.vatOutput')}</ListItem>
                          <ListItem><Code>1105-005</Code> - {t('accounts.guide.vatInput')}</ListItem>
                          <ListItem><Code>4101</Code> - {t('accounts.guide.salesRevenue')}</ListItem>
                          <ListItem><Code>5101</Code> - {t('accounts.guide.costOfGoodsSold')}</ListItem>
                        </UnorderedList>
                      </Box>

                      <Divider />

                      <Box>
                        <Text fontWeight="semibold" color="orange.600" mb={2}>⚠️ {t('accounts.guide.importantTips')}</Text>
                        <UnorderedList spacing={1} fontSize="sm">
                          <ListItem>{t('accounts.guide.tip1')}</ListItem>
                          <ListItem>{t('accounts.guide.tip2')}</ListItem>
                          <ListItem>{t('accounts.guide.tip3')}</ListItem>
                          <ListItem>{t('accounts.guide.tip4')}</ListItem>
                        </UnorderedList>
                      </Box>

                      <Divider />

                      <Box>
                        <Text fontWeight="semibold" color="purple.600" mb={2}>🔧 {t('accounts.guide.ifDeleted')}</Text>
                        <Text fontSize="sm" mb={2} color="red.600">
                          ⚠️ <strong>{t('accounts.guide.hardcodedWarning')}</strong>
                        </Text>
                        <Text fontSize="sm" fontWeight="semibold" mb={1}>{t('accounts.guide.hardcodedAccounts')}</Text>
                        <UnorderedList spacing={1} fontSize="sm" mb={2}>
                          <ListItem><Code>1101 - {t('accounts.guide.cash')}</Code> (Asset, type: Asset)</ListItem>
                          <ListItem><Code>1101-003 - BANK GIRO</Code> (Asset, type: Asset)</ListItem>
                          <ListItem><Code>1102 - PIUTANG USAHA</Code> (Asset, type: Asset)</ListItem>
                          <ListItem><Code>1105-005 - {t('accounts.guide.vatInput')}</Code> (Asset, type: Asset)</ListItem>
                          <ListItem><Code>2102-001 - {t('accounts.guide.vatOutput')}</Code> (Liability, type: Liability)</ListItem>
                          <ListItem><Code>4101 - {t('accounts.guide.salesRevenue')}</Code> (Revenue, type: Revenue)</ListItem>
                          <ListItem><Code>5101 - {t('accounts.guide.costOfGoodsSold')}</Code> (Expense, type: Expense)</ListItem>
                        </UnorderedList>
                        <Text fontSize="sm" fontWeight="semibold" mb={1}>{t('accounts.guide.howToRecreate')}</Text>
                        <UnorderedList spacing={1} fontSize="sm" mt={1}>
                          <ListItem>{t('accounts.guide.recreate1')}</ListItem>
                          <ListItem>{t('accounts.guide.recreate2')}</ListItem>
                          <ListItem>{t('accounts.guide.recreate3')}</ListItem>
                          <ListItem>{t('accounts.guide.recreate4')}</ListItem>
                          <ListItem>{t('accounts.guide.recreate5')}</ListItem>
                        </UnorderedList>
                      </Box>

                      <Box bg="blue.50" p={3} borderRadius="md">
                        <Text fontSize="xs" color="blue.800">
                          💡 <strong>{t('accounts.guide.proTip')}</strong>
                        </Text>
                      </Box>
                    </VStack>
                  </PopoverBody>
                </PopoverContent>
              </Popover>
            </HStack>
            {canCreate && (
              <HStack spacing={3}>
                <Tooltip
                  label={t('accounts.guide.headerTooltip')}
                  placement="bottom"
                  hasArrow
                >
                  <Button
                    variant="outline"
                    colorScheme="blue"
                    leftIcon={<FiPlus />}
                    onClick={handleCreateHeader}
                    size="md"
                    px={6}
                    py={2}
                    borderRadius="md"
                    fontWeight="medium"
                    _hover={{
                      transform: 'translateY(-1px)',
                      boxShadow: 'md'
                    }}
                  >
                    {t('accounts.addHeaderAccount')}
                  </Button>
                </Tooltip>
                <Tooltip
                  label={t('accounts.guide.accountTooltip')}
                  placement="bottom"
                  hasArrow
                >
                  <Button
                    colorScheme="blue"
                    leftIcon={<FiPlus />}
                    onClick={handleCreate}
                    size="md"
                    px={6}
                    py={2}
                    borderRadius="md"
                    fontWeight="medium"
                    _hover={{
                      transform: 'translateY(-1px)',
                      boxShadow: 'lg'
                    }}
                  >
                    {t('accounts.addAccount')}
                  </Button>
                </Tooltip>
              </HStack>
            )}
          </Flex>

          {/* Info Banner */}
          <Alert status="warning" mb={4} borderRadius="md" variant="left-accent">
            <AlertIcon />
            <Box flex="1">
              <AlertTitle fontSize="sm" mb={1}>⚠️ {t('accounts.guide.warningTitle')}</AlertTitle>
              <AlertDescription fontSize="xs">
                {t('accounts.guide.warningDesc')}
              </AlertDescription>
            </Box>
          </Alert>

          {error && (
            <Alert status="error" mb={4}>
              <AlertIcon />
              <AlertTitle mr={2}>Error!</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          <Box
            bg={cardBg}
            borderRadius="lg"
            boxShadow={cardShadow}
            overflow="hidden"
            border="1px"
            borderColor={borderColor}
            className="table-container"
            transition="all 0.3s ease"
          >
            <Tabs
              index={tabIndex}
              onChange={setTabIndex}
              variant="unstyled"
            >
              <TabList borderBottom="1px" borderColor={tabBorderColor} px={4}>
                <Tab
                  _selected={{
                    color: 'blue.500',
                    borderBottom: '2px solid',
                    borderColor: 'blue.500'
                  }}
                  pb={4}
                  pt={4}
                  fontWeight="medium"
                  fontSize="sm"
                >
                  {t('accounts.listView')}
                </Tab>
                <Tab
                  _selected={{
                    color: 'blue.500',
                    borderBottom: '2px solid',
                    borderColor: 'blue.500'
                  }}
                  pb={4}
                  pt={4}
                  fontWeight="medium"
                  fontSize="sm"
                >
                  {t('accounts.treeView')}
                </Tab>
              </TabList>

              <TabPanels>
                <TabPanel px={0} py={0}>

                  {isLoading ? (
                    <Flex justify="center" py={10}>
                      <Spinner size="lg" color="blue.500" />
                    </Flex>
                  ) : hierarchicalAccounts.length === 0 ? (
                    <Box textAlign="center" py={10}>
                      <Text color={emptyStateColor} mb={4}>
                        {flatAccounts.length === 0 ? 'No accounts found. Try creating one!' : 'No accounts match your search criteria.'}
                      </Text>
                      {flatAccounts.length === 0 && (
                        <Button colorScheme="blue" onClick={handleCreate}>
                          Create First Account
                        </Button>
                      )}
                    </Box>
                  ) : (
                    <AccountsTable
                      accounts={hierarchicalAccounts}
                      onEdit={handleEdit}
                      onDelete={handleDelete}
                      onAdminDelete={handleAdminDelete}
                    />
                  )}
                </TabPanel>

                <TabPanel px={0}>
                  {isLoading ? (
                    <Flex justify="center" py={10}>
                      <Spinner size="lg" />
                    </Flex>
                  ) : (
                    <AccountTreeView
                      accounts={hierarchyAccounts}
                      onEdit={handleEdit}
                      onDelete={handleDelete}
                      onAdminDelete={handleAdminDelete}
                      showActions={true}
                      showBalance={true}
                    />
                  )}
                </TabPanel>

              </TabPanels>
            </Tabs>
          </Box>

          <Modal isOpen={isModalOpen} onClose={() => setIsModalOpen(false)} size="lg">
            <ModalOverlay />
            <ModalContent bg={modalBg} border="1px solid" borderColor={borderColor}>
              <ModalHeader color={modalHeaderTextColor} borderBottom="1px solid" borderColor={borderColor}>
                {selectedAccount ? t('accounts.editAccount') : (isHeaderMode ? t('accounts.createHeaderAccount') : t('accounts.createAccount'))}
              </ModalHeader>
              <ModalCloseButton />
              <ModalBody pb={6}>
                <AccountForm
                  account={selectedAccount || undefined}
                  parentAccounts={flatAccounts.filter(a => a.id !== selectedAccount?.id)}
                  onSubmit={handleSubmit}
                  onCancel={() => setIsModalOpen(false)}
                  isSubmitting={isSubmitting}
                  isHeaderMode={isHeaderMode}
                />
              </ModalBody>
            </ModalContent>
          </Modal>

          {/* Admin Delete Dialog */}
          <AdminDeleteDialog
            isOpen={isAdminDeleteOpen}
            onClose={() => setIsAdminDeleteOpen(false)}
            account={accountToDelete}
            parentAccounts={flatAccounts.filter(a => a.is_header)}
            childAccounts={accountToDelete ? getAccountChildren(accountToDelete.id) : []}
            onConfirm={performAdminDelete}
            isSubmitting={isSubmitting}
          />
        </Box>
      </SimpleLayout>
    </CompanyTypeGuard>
  );
};

export default AccountsPage;
