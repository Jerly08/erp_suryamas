'use client';

import React, { useEffect, useState } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useTranslation } from '@/hooks/useTranslation';
import SimpleLayout from '@/components/layout/SimpleLayout';
import api from '@/services/api';
import {
  Box,
  VStack,
  HStack,
  Heading,
  Text,
  Card,
  CardBody,
  CardHeader,
  SimpleGrid,
  Icon,
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
  Spinner,
  useColorModeValue,
  Divider,
  Select,
  FormControl,
  FormLabel,
  FormErrorMessage,
  FormHelperText,
  useToast,
  Button,
  ButtonGroup,
  Badge,
  Tooltip,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalFooter,
  ModalBody,
  ModalCloseButton,
  useDisclosure,
  Tabs,
  TabList,
  TabPanels,
  Tab,
  TabPanel,
  Stack,
  IconButton
} from '@chakra-ui/react';
import {
  FiSave,
  FiX,
  FiSettings,
  FiDollarSign,
  FiShoppingCart,
  FiCreditCard,
  FiInfo,
  FiRefreshCw,
  FiCheck,
  FiActivity,
  FiLock,
  FiAlertTriangle,
  FiPackage
} from 'react-icons/fi';
import { API_ENDPOINTS } from '@/config/api';

interface AccountOption {
  id: number;
  code: string;
  name: string;
  type: string;
  category: string;
  is_active: boolean;
  is_system_critical?: boolean; // Lock critical accounts from modification
}

interface TaxAccountSettings {
  id?: number;
  // Sales accounts
  sales_receivable_account: AccountOption;
  sales_cash_account: AccountOption;
  sales_bank_account: AccountOption;
  sales_revenue_account: AccountOption;
  sales_output_vat_account: AccountOption;

  // Purchase accounts
  purchase_payable_account: AccountOption;
  purchase_cash_account: AccountOption;
  purchase_bank_account: AccountOption;
  purchase_input_vat_account: AccountOption;
  purchase_expense_account: AccountOption;

  // Optional accounts
  withholding_tax21_account?: AccountOption;
  withholding_tax23_account?: AccountOption;
  withholding_tax25_account?: AccountOption;
  tax_payable_account?: AccountOption;
  inventory_account?: AccountOption;
  cogs_account?: AccountOption;

  is_active: boolean;
  apply_to_all_companies: boolean;
  notes: string;
  updated_by_user: {
    id: number;
    name: string;
    username: string;
  };
  created_at: string;
  updated_at: string;
}

interface TaxConfig {
  id?: number;
  config_name: string;
  description: string;

  // Sales tax rates
  sales_ppn_rate: number;
  sales_pph21_rate: number;
  sales_pph23_rate: number;
  sales_other_tax_rate: number;

  // Purchase tax rates
  purchase_ppn_rate: number;
  purchase_pph21_rate: number;
  purchase_pph23_rate: number;
  purchase_pph25_rate: number;
  purchase_other_tax_rate: number;

  // Additional settings
  shipping_taxable: boolean;
  discount_before_tax: boolean;
  rounding_method: string;

  is_active: boolean;
  is_default: boolean;
  notes: string;
  updated_by_user?: {
    id: number;
    name: string;
    username: string;
  };
  created_at?: string;
  updated_at?: string;
}

interface AccountSuggestions {
  sales: {
    [key: string]: {
      recommended_types: string[];
      recommended_categories: string[];
      suggested_codes: string[];
      description: string;
    }
  };
  purchase: {
    [key: string]: {
      recommended_types: string[];
      recommended_categories: string[];
      suggested_codes: string[];
      description: string;
    }
  };
  tax: {
    [key: string]: {
      recommended_types: string[];
      recommended_categories: string[];
      suggested_codes: string[];
      description: string;
    }
  };
  inventory: {
    [key: string]: {
      recommended_types: string[];
      recommended_categories: string[];
      suggested_codes: string[];
      description: string;
    }
  };
}

const TaxAccountSettingsPage: React.FC = () => {
  const { user } = useAuth();
  const { t } = useTranslation();
  const toast = useToast();

  const [settings, setSettings] = useState<TaxAccountSettings | null>(null);
  const [availableAccounts, setAvailableAccounts] = useState<AccountOption[]>([]);
  const [suggestions, setSuggestions] = useState<AccountSuggestions | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [hasChanges, setHasChanges] = useState(false);
  const [validationErrors, setValidationErrors] = useState<{ [key: string]: string }>({});

  // Form state
  const [formData, setFormData] = useState<any>({});

  // Company type from main settings
  const [companyType, setCompanyType] = useState<string | null>(null);

  // Modal for suggestions
  const { isOpen: isSuggestionsOpen, onOpen: onSuggestionsOpen, onClose: onSuggestionsClose } = useDisclosure();

  const blueColor = useColorModeValue('blue.500', 'blue.300');
  const greenColor = useColorModeValue('green.500', 'green.300');
  const purpleColor = useColorModeValue('purple.500', 'purple.300');
  const orangeColor = useColorModeValue('orange.500', 'orange.300');
  const redColor = useColorModeValue('red.500', 'red.300');
  const headingColor = useColorModeValue('gray.800', 'whiteAlpha.900');
  const textColor = useColorModeValue('gray.700', 'whiteAlpha.900');
  const mutedTextColor = useColorModeValue('gray.600', 'whiteAlpha.700');
  const cardBg = useColorModeValue('white', 'gray.800');
  const cardBorderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const statusCardBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const modalBg = useColorModeValue('white', 'gray.800');
  const modalBorderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const modalFooterBg = useColorModeValue('gray.50', 'gray.750');
  const dividerColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const inputBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const inputHoverBg = useColorModeValue('white', 'whiteAlpha.160');
  const inputFocusBg = useColorModeValue('white', 'gray.700');
  const inputBorderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const inputHoverBorderColor = useColorModeValue('gray.300', 'whiteAlpha.300');
  const inputFocusBorderColor = useColorModeValue('blue.400', 'blue.300');
  const inputPlaceholderColor = useColorModeValue('gray.500', 'whiteAlpha.500');
  const lockedInputBg = useColorModeValue('gray.100', 'whiteAlpha.120');
  const lockedInputBorderColor = useColorModeValue('gray.300', 'whiteAlpha.260');
  const selectedHelperColor = useColorModeValue('blue.600', 'blue.200');
  const suggestionCardBg = useColorModeValue('white', 'gray.750');
  const salesSuggestionBorder = useColorModeValue('blue.100', 'blue.700');
  const purchaseSuggestionBorder = useColorModeValue('orange.100', 'orange.700');
  const taxSuggestionBorder = useColorModeValue('green.100', 'green.700');
  const inventorySuggestionBorder = useColorModeValue('purple.100', 'purple.700');
  const fieldStyles = {
    bg: inputBg,
    borderColor: inputBorderColor,
    color: textColor,
    _placeholder: { color: inputPlaceholderColor },
    _hover: {
      bg: inputHoverBg,
      borderColor: inputHoverBorderColor,
    },
    _focus: {
      bg: inputFocusBg,
      borderColor: inputFocusBorderColor,
    },
  };

  // Critical accounts that should not be changed
  const isCriticalAccount = (fieldName: string): boolean => {
    const criticalAccounts = [
      'sales_receivable',     // 1201 - Used in ALL credit sales
      'sales_revenue',        // 4101 - Used in ALL sales
      'purchase_payable',     // 2001 - Used in ALL credit purchases
    ];
    // PPN, Withholding tax, dan inventory accounts TIDAK critical, bisa diubah
    // PPN Masukan, PPN Keluaran, PPh 21, PPh 23, PPh 25, Tax Payable, Inventory, COGS
    return criticalAccounts.includes(fieldName);
  };

  // Fetch current settings
  const fetchSettings = async () => {
    setLoading(true);
    try {
      const [settingsRes, accountsRes, suggestionsRes] = await Promise.all([
        api.get('/api/v1/tax-accounts/current'),
        api.get('/api/v1/tax-accounts/accounts'),
        api.get('/api/v1/tax-accounts/suggestions')
      ]);

      if (settingsRes.data.success) {
        const settingsData = settingsRes.data.data;
        setSettings(settingsData);

        // Helper function to parse account JSON strings
        const parseAccount = (accountData: any) => {
          if (!accountData) return null;
          if (typeof accountData === 'string') {
            try {
              return JSON.parse(accountData);
            } catch (e) {
              console.error('Failed to parse account data:', e);
              return null;
            }
          }
          return accountData; // Already an object
        };

        // Transform the settings data to match form field names
        const formDataTransformed = {
          ...settingsData,
          // Parse account objects from JSON strings
          sales_receivable_account: parseAccount(settingsData.sales_receivable_account),
          sales_cash_account: parseAccount(settingsData.sales_cash_account),
          sales_bank_account: parseAccount(settingsData.sales_bank_account),
          sales_revenue_account: parseAccount(settingsData.sales_revenue_account),
          sales_output_vat_account: parseAccount(settingsData.sales_output_vat_account),
          purchase_payable_account: parseAccount(settingsData.purchase_payable_account),
          purchase_cash_account: parseAccount(settingsData.purchase_cash_account),
          purchase_bank_account: parseAccount(settingsData.purchase_bank_account),
          purchase_input_vat_account: parseAccount(settingsData.purchase_input_vat_account),
          purchase_expense_account: parseAccount(settingsData.purchase_expense_account),
          withholding_tax21_account: parseAccount(settingsData.withholding_tax21_account),
          withholding_tax23_account: parseAccount(settingsData.withholding_tax23_account),
          withholding_tax25_account: parseAccount(settingsData.withholding_tax25_account),
          tax_payable_account: parseAccount(settingsData.tax_payable_account),
          inventory_account: parseAccount(settingsData.inventory_account),
          cogs_account: parseAccount(settingsData.cogs_account),

          // Sales account IDs
          sales_receivable_account_id: parseAccount(settingsData.sales_receivable_account)?.id,
          sales_cash_account_id: parseAccount(settingsData.sales_cash_account)?.id,
          sales_bank_account_id: parseAccount(settingsData.sales_bank_account)?.id,
          sales_revenue_account_id: parseAccount(settingsData.sales_revenue_account)?.id,
          sales_output_vat_account_id: parseAccount(settingsData.sales_output_vat_account)?.id,

          // Purchase account IDs
          purchase_payable_account_id: parseAccount(settingsData.purchase_payable_account)?.id,
          purchase_cash_account_id: parseAccount(settingsData.purchase_cash_account)?.id,
          purchase_bank_account_id: parseAccount(settingsData.purchase_bank_account)?.id,
          purchase_input_vat_account_id: parseAccount(settingsData.purchase_input_vat_account)?.id,
          purchase_expense_account_id: parseAccount(settingsData.purchase_expense_account)?.id,

          // Optional account IDs
          withholding_tax21_account_id: parseAccount(settingsData.withholding_tax21_account)?.id,
          withholding_tax23_account_id: parseAccount(settingsData.withholding_tax23_account)?.id,
          withholding_tax25_account_id: parseAccount(settingsData.withholding_tax25_account)?.id,
          tax_payable_account_id: parseAccount(settingsData.tax_payable_account)?.id,
          inventory_account_id: parseAccount(settingsData.inventory_account)?.id,
          cogs_account_id: parseAccount(settingsData.cogs_account)?.id
        };

        setFormData(formDataTransformed);
      }

      if (accountsRes.data.success) {
        setAvailableAccounts(accountsRes.data.data);
      }

      if (suggestionsRes.data.success) {
        setSuggestions(suggestionsRes.data.data);
      }

      setHasChanges(false);
      setError(null);
    } catch (err: any) {
      console.error('Error fetching tax account settings:', err);
      setError(err.response?.data?.error || err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSettings();
    // Fetch company type from main settings
    const fetchCompanyType = async () => {
      try {
        const res = await api.get(API_ENDPOINTS.SETTINGS);
        if (res.data.success && res.data.data?.company_type) {
          setCompanyType(res.data.data.company_type);
        }
      } catch (err) {
        console.error('Failed to fetch company type:', err);
      }
    };
    fetchCompanyType();
  }, []);

  const handleAccountChange = (fieldName: string, accountId: string) => {
    const accountIdNum = parseInt(accountId);
    const selectedAccount = availableAccounts.find(acc => acc.id === accountIdNum);

    if (selectedAccount) {
      setFormData((prev: any) => ({
        ...prev,
        [`${fieldName}_account_id`]: accountIdNum,
        [`${fieldName}_account`]: selectedAccount
      }));
      setHasChanges(true);

      // Clear validation error for this field
      setValidationErrors(prev => ({
        ...prev,
        [fieldName]: ''
      }));
    }
  };

  const validateForm = () => {
    const errors: { [key: string]: string } = {};

    // NOTE: This page only shows OPTIONAL withholding tax and inventory accounts
    // Sales and purchase accounts are configured elsewhere and already exist in settings
    // So we don't validate them here - they should already be set

    // Required fields based on company type
    if (companyType === 'TRADING' || companyType === 'MANUFACTURING') {
      if (!formData.inventory_account_id) {
        errors['inventory'] = 'Inventory account wajib untuk perusahaan ' + (companyType === 'TRADING' ? 'dagang' : 'manufaktur');
      }
      if (!formData.cogs_account_id) {
        errors['cogs'] = 'COGS account wajib untuk perusahaan ' + (companyType === 'TRADING' ? 'dagang' : 'manufaktur');
      }
    }

    setValidationErrors(errors);
    return Object.keys(errors).length === 0;
  };

  const handleSave = async () => {
    // Validate form (currently always returns true since all fields are optional)
    if (!validateForm()) {
      toast({
        title: t('settings.taxAccounts.messages.validationError'),
        description: t('settings.taxAccounts.messages.checkInput'),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
      return;
    }

    setSaving(true);
    try {
      const payload = {
        // Sales accounts (preserve existing values from settings)
        sales_receivable_account_id: formData.sales_receivable_account_id || settings?.sales_receivable_account?.id,
        sales_cash_account_id: formData.sales_cash_account_id || settings?.sales_cash_account?.id,
        sales_bank_account_id: formData.sales_bank_account_id || settings?.sales_bank_account?.id,
        sales_revenue_account_id: formData.sales_revenue_account_id || settings?.sales_revenue_account?.id,
        sales_output_vat_account_id: formData.sales_output_vat_account_id || settings?.sales_output_vat_account?.id,

        // Purchase accounts (preserve existing values from settings)
        purchase_payable_account_id: formData.purchase_payable_account_id || settings?.purchase_payable_account?.id,
        purchase_cash_account_id: formData.purchase_cash_account_id || settings?.purchase_cash_account?.id,
        purchase_bank_account_id: formData.purchase_bank_account_id || settings?.purchase_bank_account?.id,
        purchase_input_vat_account_id: formData.purchase_input_vat_account_id || settings?.purchase_input_vat_account?.id,
        purchase_expense_account_id: formData.purchase_expense_account_id || settings?.purchase_expense_account?.id,

        // Optional accounts
        withholding_tax21_account_id: formData.withholding_tax21_account_id || null,
        withholding_tax23_account_id: formData.withholding_tax23_account_id || null,
        withholding_tax25_account_id: formData.withholding_tax25_account_id || null,
        tax_payable_account_id: formData.tax_payable_account_id || null,
        inventory_account_id: formData.inventory_account_id || null,
        cogs_account_id: formData.cogs_account_id || null,

        apply_to_all_companies: true,
        notes: formData.notes || ''
      };

      let response;
      if (settings?.id) {
        // Update existing
        response = await api.put(`/api/v1/tax-accounts/${settings.id}`, payload);
      } else {
        // Create new
        response = await api.post('/api/v1/tax-accounts', payload);
      }

      if (response.data.success) {
        // Reload all data to show current account names correctly
        await fetchSettings();

        toast({
          title: t('common.success'),
          description: t('settings.taxAccounts.messages.saveSuccess'),
          status: 'success',
          duration: 3000,
          isClosable: true,
        });
      }
    } catch (error: any) {
      toast({
        title: t('settings.taxAccounts.messages.saveError'),
        description: error.response?.data?.details || error.message,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setSaving(false);
    }
  };

  const handleCancel = () => {
    if (settings) {
      // Helper function to parse account JSON strings
      const parseAccount = (accountData: any) => {
        if (!accountData) return null;
        if (typeof accountData === 'string') {
          try {
            return JSON.parse(accountData);
          } catch (e) {
            console.error('Failed to parse account data:', e);
            return null;
          }
        }
        return accountData; // Already an object
      };

      // Transform the settings data to match form field names
      const formDataTransformed = {
        ...settings,
        // Parse account objects from JSON strings
        sales_receivable_account: parseAccount(settings.sales_receivable_account),
        sales_cash_account: parseAccount(settings.sales_cash_account),
        sales_bank_account: parseAccount(settings.sales_bank_account),
        sales_revenue_account: parseAccount(settings.sales_revenue_account),
        sales_output_vat_account: parseAccount(settings.sales_output_vat_account),
        purchase_payable_account: parseAccount(settings.purchase_payable_account),
        purchase_cash_account: parseAccount(settings.purchase_cash_account),
        purchase_bank_account: parseAccount(settings.purchase_bank_account),
        purchase_input_vat_account: parseAccount(settings.purchase_input_vat_account),
        purchase_expense_account: parseAccount(settings.purchase_expense_account),
        withholding_tax21_account: parseAccount(settings.withholding_tax21_account),
        withholding_tax23_account: parseAccount(settings.withholding_tax23_account),
        withholding_tax25_account: parseAccount(settings.withholding_tax25_account),
        tax_payable_account: parseAccount(settings.tax_payable_account),
        inventory_account: parseAccount(settings.inventory_account),
        cogs_account: parseAccount(settings.cogs_account),

        // Sales account IDs
        sales_receivable_account_id: parseAccount(settings.sales_receivable_account)?.id,
        sales_cash_account_id: parseAccount(settings.sales_cash_account)?.id,
        sales_bank_account_id: parseAccount(settings.sales_bank_account)?.id,
        sales_revenue_account_id: parseAccount(settings.sales_revenue_account)?.id,
        sales_output_vat_account_id: parseAccount(settings.sales_output_vat_account)?.id,

        // Purchase account IDs
        purchase_payable_account_id: parseAccount(settings.purchase_payable_account)?.id,
        purchase_cash_account_id: parseAccount(settings.purchase_cash_account)?.id,
        purchase_bank_account_id: parseAccount(settings.purchase_bank_account)?.id,
        purchase_input_vat_account_id: parseAccount(settings.purchase_input_vat_account)?.id,
        purchase_expense_account_id: parseAccount(settings.purchase_expense_account)?.id,

        // Optional account IDs
        withholding_tax21_account_id: parseAccount(settings.withholding_tax21_account)?.id,
        withholding_tax23_account_id: parseAccount(settings.withholding_tax23_account)?.id,
        withholding_tax25_account_id: parseAccount(settings.withholding_tax25_account)?.id,
        tax_payable_account_id: parseAccount(settings.tax_payable_account)?.id,
        inventory_account_id: parseAccount(settings.inventory_account)?.id,
        cogs_account_id: parseAccount(settings.cogs_account)?.id
      };

      setFormData(formDataTransformed);
    } else {
      setFormData({});
    }
    setHasChanges(false);
    setValidationErrors({});
  };

  const handleRefresh = async () => {
    await fetchSettings();
    toast({
      title: t('settings.taxAccounts.messages.refreshed'),
      description: t('settings.taxAccounts.messages.refreshedDesc'),
      status: 'success',
      duration: 2000,
      isClosable: true,
    });
  };

  const getAccountOptions = (accountTypes: string[] = [], categories: string[] = [], suggestedCodes: string[] = []) => {
    let filtered = availableAccounts;

    if (accountTypes.length > 0) {
      filtered = filtered.filter(acc => accountTypes.includes(acc.type));
    }

    if (categories.length > 0) {
      filtered = filtered.filter(acc => categories.includes(acc.category));
    }

    const sorted = filtered.sort((a, b) => a.code.localeCompare(b.code));

    if (suggestedCodes.length > 0) {
      // Only show accounts that match the suggested codes — no irrelevant accounts
      const recommended = sorted.filter(acc =>
        suggestedCodes.some(code => acc.code.startsWith(code))
      );
      return { recommended, others: [] };
    }

    return { recommended: [], others: sorted };
  };

  const renderAccountSelect = (
    fieldName: string,
    label: string,
    isRequired = true,
    accountTypes: string[] = [],
    categories: string[] = [],
    description?: string,
    tooltipInfo?: string,
    suggestedCodes: string[] = []
  ) => {
    const currentValue = formData[`${fieldName}_account_id`] || '';
    const currentAccount = formData[`${fieldName}_account`];
    const hasError = validationErrors[fieldName];
    const { recommended, others } = getAccountOptions(accountTypes, categories, suggestedCodes);

    // Check if critical from field name OR from selected account's database flag
    const isCritical = isCriticalAccount(fieldName);
    const selectedAccount = availableAccounts.find(acc => acc.id === currentValue);
    const isSelectedCritical = selectedAccount?.is_system_critical === true;

    return (
      <FormControl isRequired={isRequired} isInvalid={!!hasError}>
        <FormLabel fontWeight="semibold" color={mutedTextColor} fontSize="sm">
          <HStack>
            <Text>{label}</Text>
            {tooltipInfo && (
              <Tooltip
                label={tooltipInfo}
                fontSize="xs"
                placement="top"
                hasArrow
                bg="blue.600"
              >
                <span>
                  <Icon as={FiInfo} color="blue.500" boxSize={4} cursor="help" />
                </span>
              </Tooltip>
            )}
            {isRequired && <Badge colorScheme="red" size="sm">Required</Badge>}
            {(isCritical || isSelectedCritical) && (
              <HStack spacing={1}>
                <Icon as={FiLock} color="red.500" boxSize={3} />
                <Badge colorScheme="red" size="sm">LOCKED</Badge>
              </HStack>
            )}
          </HStack>
        </FormLabel>
        <Select
          value={currentValue}
          onChange={(e) => handleAccountChange(fieldName, e.target.value)}
          placeholder={`${t('settings.taxAccounts.renderSelect.selectPlaceholder', { label })}`}
          variant="filled"
          isDisabled={isCritical || isSelectedCritical}
          {...fieldStyles}
          bg={(isCritical || isSelectedCritical) ? lockedInputBg : inputBg}
          borderColor={(isCritical || isSelectedCritical) ? lockedInputBorderColor : inputBorderColor}
          cursor={(isCritical || isSelectedCritical) ? 'not-allowed' : undefined}
          opacity={(isCritical || isSelectedCritical) ? 0.7 : 1}
          _hover={{
            bg: (isCritical || isSelectedCritical) ? lockedInputBg : inputHoverBg,
            borderColor: (isCritical || isSelectedCritical) ? lockedInputBorderColor : inputHoverBorderColor,
          }}
          _focus={{
            bg: (isCritical || isSelectedCritical) ? lockedInputBg : inputFocusBg,
            borderColor: (isCritical || isSelectedCritical) ? lockedInputBorderColor : inputFocusBorderColor,
          }}
        >
          {recommended.length > 0 ? (
            recommended.map((account) => (
              <option key={account.id} value={account.id}>
                {account.code} - {account.name}
              </option>
            ))
          ) : (
            others.map((account) => (
              <option key={account.id} value={account.id}>
                {account.code} - {account.name} ({account.type})
              </option>
            ))
          )}
        </Select>
        {hasError && <FormErrorMessage>{hasError}</FormErrorMessage>}
        {currentAccount && currentValue && (
          <FormHelperText fontSize="xs" color={selectedHelperColor} fontWeight="medium">
            <HStack spacing={1}>
              <Icon as={FiCheck} color="green.500" boxSize={3} />
              <Text>
                {t('settings.taxAccounts.renderSelect.current')}: {currentAccount.code} - {currentAccount.name}
              </Text>
            </HStack>
          </FormHelperText>
        )}
        {(isCritical || isSelectedCritical) && !hasError && (
          <FormHelperText fontSize="xs" color={redColor} fontWeight="medium">
            <HStack spacing={1}>
              <Icon as={FiInfo} />
              <Text>
                {t('settings.taxAccounts.renderSelect.lockedInfo')}
                {isSelectedCritical && ` ${t('settings.taxAccounts.renderSelect.lockedInDb')}`}
              </Text>
            </HStack>
          </FormHelperText>
        )}
        {description && !hasError && !isCritical && !isSelectedCritical && !currentAccount && (
          <FormHelperText fontSize="xs" color={mutedTextColor}>
            {description}
          </FormHelperText>
        )}
      </FormControl>
    );
  };

  // Loading state
  if (loading) {
    return (
      <SimpleLayout allowedRoles={['admin']}>
        <Box display="flex" alignItems="center" justifyContent="center" minH="400px">
          <VStack spacing={4}>
            <Spinner size="xl" thickness="4px" speed="0.65s" color="blue.500" />
            <Text>Loading tax account settings...</Text>
          </VStack>
        </Box>
      </SimpleLayout>
    );
  }

  return (
    <SimpleLayout allowedRoles={['admin']}>
      <Box>
        <VStack spacing={6} alignItems="start">
          {/* Header */}
          <HStack justify="space-between" width="full">
            <VStack alignItems="start" spacing={2}>
              <HStack spacing={3}>
                <Heading as="h1" size="xl" color={headingColor}>{t('settings.taxAccounts.title')}</Heading>
                {companyType && (
                  <Badge
                    colorScheme={
                      companyType === 'SERVICE' ? 'purple' :
                        companyType === 'TRADING' ? 'teal' : 'blue'
                    }
                    fontSize="sm"
                    px={3}
                    py={1}
                    borderRadius="full"
                  >
                    {companyType === 'SERVICE' ? '🏢 Jasa' :
                      companyType === 'TRADING' ? '🏪 Dagang' : '🏭 Manufaktur'}
                  </Badge>
                )}
              </HStack>
              <Text color={mutedTextColor} fontSize="sm">
                {t('settings.taxAccounts.subtitle')}
              </Text>
            </VStack>

            <ButtonGroup spacing={2}>
              {hasChanges && (
                <>
                  <Button
                    colorScheme="green"
                    leftIcon={<FiSave />}
                    onClick={handleSave}
                    isLoading={saving}
                    loadingText={t('common.saving')}
                    size="sm"
                  >
                    {t('common.saveChanges')}
                  </Button>
                  <Button
                    variant="outline"
                    leftIcon={<FiX />}
                    onClick={handleCancel}
                    isDisabled={saving}
                    size="sm"
                  >
                    {t('common.cancel')}
                  </Button>
                </>
              )}
              <Button
                variant="outline"
                leftIcon={<FiRefreshCw />}
                onClick={handleRefresh}
                size="sm"
              >
                {t('common.refresh')}
              </Button>
              <Button
                variant="outline"
                leftIcon={<FiInfo />}
                onClick={onSuggestionsOpen}
                size="sm"
              >
                {t('common.suggestions') || "Suggestions"}
              </Button>
            </ButtonGroup>
          </HStack>

          {/* Status Alert */}
          {settings && (
            <Alert status="success" variant="left-accent">
              <AlertIcon />
              <Box>
                <AlertTitle>{t('settings.taxAccounts.currentConfig')}</AlertTitle>
                <AlertDescription>
                  {t('settings.lastUpdated')}: {new Date(settings.updated_at).toLocaleString()} {t('common.by')} {settings.updated_by_user.name}
                </AlertDescription>
              </Box>
            </Alert>
          )}

          {error && (
            <Alert status="error" width="full">
              <AlertIcon />
              <AlertTitle>Error:</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          {/* Main Content - Tax Accounts Configuration */}
          <Box width="full">
            {/* VAT/PPN Accounts - Critical for Sales & Purchase */}
            <Card mb={6} bg={cardBg} border="1px" borderColor={cardBorderColor} boxShadow="md">
              <CardHeader>
                <HStack spacing={3} justify="space-between">
                  <HStack spacing={3}>
                    <Icon as={FiCreditCard} boxSize={5} color={purpleColor} />
                    <Heading size="md">{t('settings.taxAccounts.vatAccounts')}</Heading>
                    <Badge colorScheme="green" size="sm">{t('settings.taxAccounts.status.configurable')}</Badge>
                  </HStack>
                  <Tooltip
                    label={t('settings.taxAccounts.tooltips.vat')}
                    placement="left"
                    hasArrow
                  >
                    <IconButton
                      aria-label="Info"
                      icon={<FiInfo />}
                      size="sm"
                      variant="ghost"
                      colorScheme="purple"
                    />
                  </Tooltip>
                </HStack>
              </CardHeader>
              <CardBody>
                <Alert status="info" mb={4} variant="left-accent">
                  <AlertIcon />
                  <Box>
                    <AlertTitle fontSize="sm">{t('settings.taxAccounts.taxRemittance')}</AlertTitle>
                    <AlertDescription fontSize="xs">
                      {t('settings.taxAccounts.descriptions.taxRemittance')}
                    </AlertDescription>
                  </Box>
                </Alert>
                <SimpleGrid columns={[1, 1, 2]} spacing={6}>
                  {renderAccountSelect(
                    'purchase_input_vat',
                    t('settings.taxAccounts.vatInput'),
                    false,
                    ['LIABILITY'],
                    ['CURRENT_LIABILITY'],
                    t('settings.taxAccounts.descriptions.vatInput'),
                    t('settings.taxAccounts.tooltips.vat'),
                    ['2102-002', '2102']
                  )}

                  {renderAccountSelect(
                    'sales_output_vat',
                    t('settings.taxAccounts.vatOutput'),
                    false,
                    ['LIABILITY'],
                    ['CURRENT_LIABILITY'],
                    t('settings.taxAccounts.descriptions.vatOutput'),
                    t('settings.taxAccounts.tooltips.vat'),
                    ['2102-001', '2102']
                  )}
                </SimpleGrid>
              </CardBody>
            </Card>

            {/* Company-type-specific alert for withholding taxes */}
            {companyType === 'SERVICE' && (
              <Alert status="info" variant="subtle" borderRadius="md" mb={6}>
                <AlertIcon />
                <Box>
                  <AlertTitle fontSize="sm">{t('settings.taxAccounts.companyAlerts.serviceTitle')}</AlertTitle>
                  <AlertDescription fontSize="xs">
                    {t('settings.taxAccounts.companyAlerts.serviceDesc')}
                  </AlertDescription>
                </Box>
              </Alert>
            )}
            {companyType === 'TRADING' && (
              <Alert status="warning" variant="subtle" borderRadius="md" mb={6}>
                <AlertIcon />
                <Box>
                  <AlertTitle fontSize="sm">{t('settings.taxAccounts.companyAlerts.tradingTitle')}</AlertTitle>
                  <AlertDescription fontSize="xs" dangerouslySetInnerHTML={{ __html: t('settings.taxAccounts.companyAlerts.tradingDesc') }} />
                </Box>
              </Alert>
            )}
            {companyType === 'MANUFACTURING' && (
              <Alert status="warning" variant="subtle" borderRadius="md" mb={6}>
                <AlertIcon />
                <Box>
                  <AlertTitle fontSize="sm">{t('settings.taxAccounts.companyAlerts.manufacturingTitle')}</AlertTitle>
                  <AlertDescription fontSize="xs" dangerouslySetInnerHTML={{ __html: t('settings.taxAccounts.companyAlerts.manufacturingDesc') }} />
                </Box>
              </Alert>
            )}

            {/* Withholding Tax Accounts */}
            <Card mb={6} bg={cardBg} border="1px" borderColor={cardBorderColor} boxShadow="md">
              <CardHeader>
                <HStack spacing={3} justify="space-between">
                  <HStack spacing={3}>
                    <Icon as={FiDollarSign} boxSize={5} color={greenColor} />
                    <Heading size="md">{t('settings.taxAccounts.withholdingTax')}</Heading>
                    <Badge colorScheme="green" size="sm">{t('settings.taxAccounts.status.configurable')}</Badge>
                  </HStack>
                  <Tooltip
                    label={t('settings.taxAccounts.descriptions.withholdingTax')}
                    placement="left"
                    hasArrow
                  >
                    <IconButton
                      aria-label="Info"
                      icon={<FiInfo />}
                      size="sm"
                      variant="ghost"
                      colorScheme="blue"
                    />
                  </Tooltip>
                </HStack>
              </CardHeader>
              <CardBody>
                <SimpleGrid columns={[1, 1, 2]} spacing={6}>
                  {renderAccountSelect(
                    'withholding_tax21',
                    t('settings.taxAccounts.pph21.label'),
                    false,
                    ['ASSET'],
                    ['CURRENT_ASSET'],
                    t('settings.taxAccounts.pph21.desc'),
                    t('settings.taxAccounts.pph21.tooltip'),
                    ['1105-001', '1105']
                  )}

                  {renderAccountSelect(
                    'withholding_tax23',
                    t('settings.taxAccounts.pph23.label'),
                    false,
                    ['ASSET'],
                    ['CURRENT_ASSET'],
                    t('settings.taxAccounts.pph23.desc'),
                    t('settings.taxAccounts.pph23.tooltip'),
                    ['1105-002', '1105']
                  )}

                  {renderAccountSelect(
                    'withholding_tax25',
                    t('settings.taxAccounts.pph25.label'),
                    false,
                    ['ASSET'],
                    ['CURRENT_ASSET'],
                    t('settings.taxAccounts.pph25.desc'),
                    t('settings.taxAccounts.pph25.tooltip'),
                    ['1105-003', '1105']
                  )}

                  {renderAccountSelect(
                    'tax_payable',
                    t('settings.taxAccounts.taxPayable.label'),
                    false,
                    ['LIABILITY'],
                    ['CURRENT_LIABILITY'],
                    t('settings.taxAccounts.taxPayable.desc'),
                    t('settings.taxAccounts.taxPayable.tooltip'),
                    ['2102', '2102-003', '2102-004']
                  )}
                </SimpleGrid>
              </CardBody>
            </Card>

            {/* Inventory & COGS Accounts - Hidden for SERVICE companies */}
            {companyType !== 'SERVICE' && (
              <Card bg={cardBg} border="1px" borderColor={cardBorderColor} boxShadow="md">
                <CardHeader>
                  <HStack spacing={3} justify="space-between">
                    <HStack spacing={3}>
                      <Icon as={FiPackage} boxSize={5} color={blueColor} />
                      <Heading size="md">{t('settings.taxAccounts.inventoryAccounts')}</Heading>
                      {(companyType === 'TRADING' || companyType === 'MANUFACTURING') ? (
                        <Badge colorScheme="red" size="sm">{t('settings.taxAccounts.status.required')}</Badge>
                      ) : (
                        <Badge colorScheme="blue" size="sm">{t('settings.taxAccounts.status.configurable')}</Badge>
                      )}
                    </HStack>
                    <Tooltip
                      label={t('settings.taxAccounts.descriptions.inventory')}
                      placement="left"
                      hasArrow
                    >
                      <IconButton
                        aria-label="Info"
                        icon={<FiInfo />}
                        size="sm"
                        variant="ghost"
                        colorScheme="blue"
                      />
                    </Tooltip>
                  </HStack>
                </CardHeader>
                <CardBody>
                  {(companyType === 'TRADING' || companyType === 'MANUFACTURING') && (
                    <Alert status="warning" mb={4} variant="left-accent" borderRadius="md">
                      <AlertIcon />
                      <AlertDescription fontSize="xs">
                        ⚠️ {t('settings.taxAccounts.companyAlerts.inventoryRequired')} {companyType === 'TRADING' ? t('settings.taxAccounts.companyAlerts.tradingType') : t('settings.taxAccounts.companyAlerts.manufacturingType')}.
                      </AlertDescription>
                    </Alert>
                  )}
                  <SimpleGrid columns={[1, 1, 2]} spacing={6}>
                    {renderAccountSelect(
                      'inventory',
                      t('settings.taxAccounts.inventory.label'),
                      companyType === 'TRADING' || companyType === 'MANUFACTURING',
                      ['ASSET'],
                      ['CURRENT_ASSET'],
                      t('settings.taxAccounts.inventory.desc'),
                      t('settings.taxAccounts.inventory.tooltip'),
                      ['1103', '1103-001', '1103-002', '1103-003']
                    )}

                    {renderAccountSelect(
                      'cogs',
                      t('settings.taxAccounts.cogs.label'),
                      companyType === 'TRADING' || companyType === 'MANUFACTURING',
                      ['EXPENSE'],
                      ['COST_OF_GOODS_SOLD', 'OPERATING_EXPENSE'],
                      t('settings.taxAccounts.cogs.desc'),
                      t('settings.taxAccounts.cogs.tooltip'),
                      ['5000', '5101', '5102']
                    )}
                  </SimpleGrid>
                </CardBody>
              </Card>
            )}
          </Box>

          {/* Current Status */}
          {settings && (
            <Card width="full" bg={statusCardBg} border="1px" borderColor={cardBorderColor} boxShadow="md">
              <CardHeader>
                <HStack spacing={2}>
                  <Icon as={FiCheck} color="green.500" />
                  <Heading size="sm">{t('settings.taxAccounts.statusSection.title')}</Heading>
                </HStack>
              </CardHeader>
              <CardBody>
                <VStack spacing={6} alignItems="start" width="full">
                  {/* VAT/PPN Status */}
                  <Box width="full">
                    <Text fontSize="sm" fontWeight="bold" mb={3} color={purpleColor}>{t('settings.taxAccounts.statusSection.vatAccounts')}</Text>
                    <SimpleGrid columns={[1, 2]} spacing={4}>
                      <VStack spacing={2} alignItems="start">
                        <Text fontSize="xs" color={mutedTextColor} fontWeight="bold">{t('settings.taxAccounts.statusSection.ppnMasukan')}</Text>
                        {settings.purchase_input_vat_account ? (
                          <HStack spacing={2}>
                            <Badge colorScheme="green">{t('settings.taxAccounts.status.configured')}</Badge>
                            <Text fontSize="sm">
                              {settings.purchase_input_vat_account?.code} - {settings.purchase_input_vat_account?.name}
                            </Text>
                          </HStack>
                        ) : (
                          <HStack spacing={2}>
                            <Badge colorScheme="red">{t('settings.taxAccounts.status.notSet')}</Badge>
                            <Text fontSize="xs" color={redColor}>{t('settings.taxAccounts.status.requiredForSetorPPN')}</Text>
                          </HStack>
                        )}
                      </VStack>

                      <VStack spacing={2} alignItems="start">
                        <Text fontSize="xs" color={mutedTextColor} fontWeight="bold">{t('settings.taxAccounts.statusSection.ppnKeluaran')}</Text>
                        {settings.sales_output_vat_account ? (
                          <HStack spacing={2}>
                            <Badge colorScheme="green">{t('settings.taxAccounts.status.configured')}</Badge>
                            <Text fontSize="sm">
                              {settings.sales_output_vat_account?.code} - {settings.sales_output_vat_account?.name}
                            </Text>
                          </HStack>
                        ) : (
                          <HStack spacing={2}>
                            <Badge colorScheme="red">{t('settings.taxAccounts.status.notSet')}</Badge>
                            <Text fontSize="xs" color={redColor}>{t('settings.taxAccounts.status.requiredForSetorPPN')}</Text>
                          </HStack>
                        )}
                      </VStack>
                    </SimpleGrid>
                  </Box>

                  <Divider borderColor={dividerColor} />

                  {/* Withholding Tax Status */}
                  <Box width="full">
                    <Text fontSize="sm" fontWeight="bold" mb={3} color={greenColor}>{t('settings.taxAccounts.statusSection.withholdingTitle')}</Text>
                    <SimpleGrid columns={[1, 2, 3]} spacing={6}>
                      <VStack spacing={2} alignItems="start">
                        <Text fontSize="xs" color={mutedTextColor} fontWeight="bold">{t('settings.taxAccounts.statusSection.pph21Label')}</Text>
                        {settings.withholding_tax21_account ? (
                          <HStack spacing={2}>
                            <Badge colorScheme="green">{t('settings.taxAccounts.status.configured')}</Badge>
                            <Text fontSize="sm">
                              {settings.withholding_tax21_account?.code} - {settings.withholding_tax21_account?.name}
                            </Text>
                          </HStack>
                        ) : (
                          <HStack spacing={2}>
                            <Badge colorScheme="gray">{t('settings.taxAccounts.status.notSet')}</Badge>
                            <Text fontSize="xs" color={mutedTextColor}>{t('settings.taxAccounts.status.optional')}</Text>
                          </HStack>
                        )}
                      </VStack>

                      <VStack spacing={2} alignItems="start">
                        <Text fontSize="xs" color={mutedTextColor} fontWeight="bold">{t('settings.taxAccounts.statusSection.pph23Label')}</Text>
                        {settings.withholding_tax23_account ? (
                          <HStack spacing={2}>
                            <Badge colorScheme="green">{t('settings.taxAccounts.status.configured')}</Badge>
                            <Text fontSize="sm">
                              {settings.withholding_tax23_account?.code} - {settings.withholding_tax23_account?.name}
                            </Text>
                          </HStack>
                        ) : (
                          <HStack spacing={2}>
                            <Badge colorScheme="gray">{t('settings.taxAccounts.status.notSet')}</Badge>
                            <Text fontSize="xs" color={mutedTextColor}>{t('settings.taxAccounts.status.optional')}</Text>
                          </HStack>
                        )}
                      </VStack>

                      <VStack spacing={2} alignItems="start">
                        <Text fontSize="xs" color={mutedTextColor} fontWeight="bold">{t('settings.taxAccounts.statusSection.pph25Label')}</Text>
                        {settings.withholding_tax25_account ? (
                          <HStack spacing={2}>
                            <Badge colorScheme="green">{t('settings.taxAccounts.status.configured')}</Badge>
                            <Text fontSize="sm">
                              {settings.withholding_tax25_account?.code} - {settings.withholding_tax25_account?.name}
                            </Text>
                          </HStack>
                        ) : (
                          <HStack spacing={2}>
                            <Badge colorScheme="gray">{t('settings.taxAccounts.status.notSet')}</Badge>
                            <Text fontSize="xs" color={mutedTextColor}>{t('settings.taxAccounts.status.optional')}</Text>
                          </HStack>
                        )}
                      </VStack>

                      <VStack spacing={2} alignItems="start">
                        <Text fontSize="xs" color={mutedTextColor} fontWeight="bold">{t('settings.taxAccounts.statusSection.taxPayable')}</Text>
                        {settings.tax_payable_account ? (
                          <HStack spacing={2}>
                            <Badge colorScheme="green">{t('settings.taxAccounts.status.configured')}</Badge>
                            <Text fontSize="sm">
                              {settings.tax_payable_account?.code} - {settings.tax_payable_account?.name}
                            </Text>
                          </HStack>
                        ) : (
                          <HStack spacing={2}>
                            <Badge colorScheme="gray">{t('settings.taxAccounts.status.notSet')}</Badge>
                            <Text fontSize="xs" color={mutedTextColor}>{t('settings.taxAccounts.status.optional')}</Text>
                          </HStack>
                        )}
                      </VStack>

                      <VStack spacing={2} alignItems="start">
                        <Text fontSize="xs" color={mutedTextColor} fontWeight="bold">{t('settings.taxAccounts.statusSection.configStatus')}</Text>
                        <Badge colorScheme={settings.is_active ? 'green' : 'gray'} fontSize="sm">
                          {settings.is_active ? t('settings.taxAccounts.status.active') : t('settings.taxAccounts.status.inactive')}
                        </Badge>
                        <Text fontSize="xs" color={mutedTextColor}>
                          {t('settings.taxAccounts.statusSection.lastUpdated')}: {new Date(settings.updated_at).toLocaleDateString()}
                        </Text>
                      </VStack>

                      <VStack spacing={2} alignItems="start">
                        <Text fontSize="xs" color={mutedTextColor} fontWeight="bold">{t('settings.taxAccounts.statusSection.updatedBy')}</Text>
                        <Text fontSize="sm">{settings.updated_by_user?.name || 'System'}</Text>
                        <Text fontSize="xs" color={mutedTextColor}>
                          {new Date(settings.updated_at).toLocaleTimeString()}
                        </Text>
                      </VStack>
                    </SimpleGrid>
                  </Box>
                </VStack>
              </CardBody>
            </Card>
          )}
        </VStack>

        {/* Suggestions Modal */}
        <Modal isOpen={isSuggestionsOpen} onClose={onSuggestionsClose} size="4xl">
          <ModalOverlay />
          <ModalContent bg={modalBg} borderWidth="1px" borderColor={modalBorderColor}>
            <ModalHeader>
              <HStack spacing={3}>
                <Icon as={FiInfo} color="blue.500" />
                <Text>{t('settings.taxAccounts.suggestions.title')}</Text>
              </HStack>
            </ModalHeader>
            <ModalCloseButton />
            <ModalBody>
              {suggestions && (
                <VStack spacing={6} alignItems="start" width="full">
                  <Alert status="info" variant="left-accent">
                    <AlertIcon />
                    <Box>
                      <AlertTitle fontSize="sm">{t('settings.taxAccounts.suggestions.guide')}</AlertTitle>
                      <AlertDescription fontSize="xs">
                        {t('settings.taxAccounts.suggestions.guideDesc')}
                      </AlertDescription>
                    </Box>
                  </Alert>

                  {/* Sales Section */}
                  {suggestions.sales && (
                    <Box width="full">
                      <Text fontSize="md" fontWeight="bold" mb={3} color={blueColor}>💰 Akun Penjualan</Text>
                      <SimpleGrid columns={[1, 1, 2]} spacing={4} width="full">
                        {Object.entries(suggestions.sales || {}).map(([key, suggestion]) => (
                          <Card key={key} size="sm" variant="outline" bg={suggestionCardBg} borderWidth="2px" borderColor={salesSuggestionBorder}>
                            <CardHeader pb={2}>
                              <HStack spacing={2}>
                                <Icon as={FiDollarSign} color="blue.500" />
                                <Text fontWeight="bold" fontSize="md">
                                  {key.replace(/_/g, ' ').toUpperCase()}
                                </Text>
                              </HStack>
                            </CardHeader>
                            <CardBody pt={0}>
                              <VStack spacing={3} alignItems="start">
                                <Text fontSize="sm" color={mutedTextColor}>{suggestion.description}</Text>
                                <Divider borderColor={dividerColor} />
                                <Box width="full">
                                  <Text fontSize="xs" fontWeight="semibold" mb={1}>{t('settings.taxAccounts.suggestions.suggestedCodes')}</Text>
                                  <HStack spacing={2} wrap="wrap">
                                    {suggestion.suggested_codes.map((code) => (
                                      <Badge key={code} colorScheme="blue" fontSize="xs" px={2} py={1}>{code}</Badge>
                                    ))}
                                  </HStack>
                                </Box>
                                <Box width="full">
                                  <Text fontSize="xs" fontWeight="semibold" mb={1}>{t('settings.taxAccounts.suggestions.accountTypes')}</Text>
                                  <HStack spacing={2} wrap="wrap">
                                    {suggestion.recommended_types.map((type) => (
                                      <Badge key={type} colorScheme="purple" fontSize="xs" px={2} py={1}>{type}</Badge>
                                    ))}
                                  </HStack>
                                </Box>
                              </VStack>
                            </CardBody>
                          </Card>
                        ))}
                      </SimpleGrid>
                    </Box>
                  )}

                  {/* Purchase Section */}
                  {suggestions.purchase && (
                    <Box width="full">
                      <Text fontSize="md" fontWeight="bold" mb={3} color={orangeColor}>🛒 Akun Pembelian</Text>
                      <SimpleGrid columns={[1, 1, 2]} spacing={4} width="full">
                        {Object.entries(suggestions.purchase || {}).map(([key, suggestion]) => (
                          <Card key={key} size="sm" variant="outline" bg={suggestionCardBg} borderWidth="2px" borderColor={purchaseSuggestionBorder}>
                            <CardHeader pb={2}>
                              <HStack spacing={2}>
                                <Icon as={FiDollarSign} color="orange.500" />
                                <Text fontWeight="bold" fontSize="md">
                                  {key.replace(/_/g, ' ').toUpperCase()}
                                </Text>
                              </HStack>
                            </CardHeader>
                            <CardBody pt={0}>
                              <VStack spacing={3} alignItems="start">
                                <Text fontSize="sm" color={mutedTextColor}>{suggestion.description}</Text>
                                <Divider borderColor={dividerColor} />
                                <Box width="full">
                                  <Text fontSize="xs" fontWeight="semibold" mb={1}>{t('settings.taxAccounts.suggestions.suggestedCodes')}</Text>
                                  <HStack spacing={2} wrap="wrap">
                                    {suggestion.suggested_codes.map((code) => (
                                      <Badge key={code} colorScheme="orange" fontSize="xs" px={2} py={1}>{code}</Badge>
                                    ))}
                                  </HStack>
                                </Box>
                                <Box width="full">
                                  <Text fontSize="xs" fontWeight="semibold" mb={1}>{t('settings.taxAccounts.suggestions.accountTypes')}</Text>
                                  <HStack spacing={2} wrap="wrap">
                                    {suggestion.recommended_types.map((type) => (
                                      <Badge key={type} colorScheme="purple" fontSize="xs" px={2} py={1}>{type}</Badge>
                                    ))}
                                  </HStack>
                                </Box>
                              </VStack>
                            </CardBody>
                          </Card>
                        ))}
                      </SimpleGrid>
                    </Box>
                  )}

                  {/* Tax Section */}
                  {suggestions.tax && (
                    <Box width="full">
                      <Text fontSize="md" fontWeight="bold" mb={3} color={greenColor}>🏛️ Akun Pajak Penghasilan</Text>
                      <SimpleGrid columns={[1, 1, 2]} spacing={4} width="full">
                        {Object.entries(suggestions.tax || {}).map(([key, suggestion]) => (
                          <Card key={key} size="sm" variant="outline" bg={suggestionCardBg} borderWidth="2px" borderColor={taxSuggestionBorder}>
                            <CardHeader pb={2}>
                              <HStack spacing={2}>
                                <Icon as={FiDollarSign} color="green.500" />
                                <Text fontWeight="bold" fontSize="md">
                                  {key.replace(/_/g, ' ').toUpperCase()}
                                </Text>
                              </HStack>
                            </CardHeader>
                            <CardBody pt={0}>
                              <VStack spacing={3} alignItems="start">
                                <Text fontSize="sm" color={mutedTextColor}>{suggestion.description}</Text>
                                <Divider borderColor={dividerColor} />
                                <Box width="full">
                                  <Text fontSize="xs" fontWeight="semibold" mb={1}>{t('settings.taxAccounts.suggestions.suggestedCodes')}</Text>
                                  <HStack spacing={2} wrap="wrap">
                                    {suggestion.suggested_codes.map((code) => (
                                      <Badge key={code} colorScheme="blue" fontSize="xs" px={2} py={1}>{code}</Badge>
                                    ))}
                                  </HStack>
                                </Box>
                                <Box width="full">
                                  <Text fontSize="xs" fontWeight="semibold" mb={1}>{t('settings.taxAccounts.suggestions.accountTypes')}</Text>
                                  <HStack spacing={2} wrap="wrap">
                                    {suggestion.recommended_types.map((type) => (
                                      <Badge key={type} colorScheme="purple" fontSize="xs" px={2} py={1}>{type}</Badge>
                                    ))}
                                  </HStack>
                                </Box>
                                <Box width="full">
                                  <Text fontSize="xs" fontWeight="semibold" mb={1}>{t('settings.taxAccounts.suggestions.categories')}</Text>
                                  <HStack spacing={2} wrap="wrap">
                                    {suggestion.recommended_categories.map((cat) => (
                                      <Badge key={cat} colorScheme="gray" fontSize="xs" px={2} py={1}>{cat}</Badge>
                                    ))}
                                  </HStack>
                                </Box>
                              </VStack>
                            </CardBody>
                          </Card>
                        ))}
                      </SimpleGrid>
                    </Box>
                  )}

                  {/* Inventory Section */}
                  {suggestions.inventory && (
                    <Box width="full">
                      <Text fontSize="md" fontWeight="bold" mb={3} color={purpleColor}>📦 Persediaan & HPP</Text>
                      <SimpleGrid columns={[1, 1, 2]} spacing={4} width="full">
                        {Object.entries(suggestions.inventory || {}).map(([key, suggestion]) => (
                          <Card key={key} size="sm" variant="outline" bg={suggestionCardBg} borderWidth="2px" borderColor={inventorySuggestionBorder}>
                            <CardHeader pb={2}>
                              <HStack spacing={2}>
                                <Icon as={FiDollarSign} color="purple.500" />
                                <Text fontWeight="bold" fontSize="md">
                                  {key.replace(/_/g, ' ').toUpperCase()}
                                </Text>
                              </HStack>
                            </CardHeader>
                            <CardBody pt={0}>
                              <VStack spacing={3} alignItems="start">
                                <Text fontSize="sm" color={mutedTextColor}>{suggestion.description}</Text>
                                <Divider borderColor={dividerColor} />
                                <Box width="full">
                                  <Text fontSize="xs" fontWeight="semibold" mb={1}>{t('settings.taxAccounts.suggestions.suggestedCodes')}</Text>
                                  <HStack spacing={2} wrap="wrap">
                                    {suggestion.suggested_codes.map((code) => (
                                      <Badge key={code} colorScheme="purple" fontSize="xs" px={2} py={1}>{code}</Badge>
                                    ))}
                                  </HStack>
                                </Box>
                                <Box width="full">
                                  <Text fontSize="xs" fontWeight="semibold" mb={1}>{t('settings.taxAccounts.suggestions.accountTypes')}</Text>
                                  <HStack spacing={2} wrap="wrap">
                                    {suggestion.recommended_types.map((type) => (
                                      <Badge key={type} colorScheme="purple" fontSize="xs" px={2} py={1}>{type}</Badge>
                                    ))}
                                  </HStack>
                                </Box>
                                <Box width="full">
                                  <Text fontSize="xs" fontWeight="semibold" mb={1}>{t('settings.taxAccounts.suggestions.categories')}</Text>
                                  <HStack spacing={2} wrap="wrap">
                                    {suggestion.recommended_categories.map((cat) => (
                                      <Badge key={cat} colorScheme="gray" fontSize="xs" px={2} py={1}>{cat}</Badge>
                                    ))}
                                  </HStack>
                                </Box>
                              </VStack>
                            </CardBody>
                          </Card>
                        ))}
                      </SimpleGrid>
                    </Box>
                  )}

                  <Alert status="warning" variant="left-accent">
                    <AlertIcon />
                    <Box>
                      <AlertDescription fontSize="xs">
                        💡 <strong>{t('settings.taxAccounts.suggestions.note')}</strong> {t('settings.taxAccounts.suggestions.noteDesc')}
                      </AlertDescription>
                    </Box>
                  </Alert>
                </VStack>
              )}
            </ModalBody>
            <ModalFooter bg={modalFooterBg} borderTopWidth="1px" borderColor={modalBorderColor}>
              <Button colorScheme="blue" onClick={onSuggestionsClose}>{t('common.close')}</Button>
            </ModalFooter>
          </ModalContent>
        </Modal>
      </Box>
    </SimpleLayout>
  );
};

export default TaxAccountSettingsPage;
