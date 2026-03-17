'use client';

import React, { useState, useEffect, useRef } from 'react';
import FormField from '../common/FormField';
import CurrencyInput from '../common/CurrencyInput';
import {
  Button,
  Text,
  Badge,
  Tooltip,
  Icon,
  HStack,
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
  Box,
  AlertDialog,
  AlertDialogBody,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogContent,
  AlertDialogOverlay,
  Checkbox,
  useColorModeValue,
  useDisclosure,
  useToast
} from '@chakra-ui/react';
import { FiInfo, FiLock, FiUnlock } from 'react-icons/fi';

import { Account, AccountCreateRequest, AccountUpdateRequest } from '@/types/account';
import { useAuth } from '@/contexts/AuthContext';
import { useTranslation } from '@/hooks/useTranslation';
import { useCompanyType } from '@/hooks/useCompanyType';

interface AccountFormProps {
  account?: Account;
  parentAccounts?: Account[];
  onSubmit: (data: AccountCreateRequest | AccountUpdateRequest) => void;
  onCancel: () => void;
  isSubmitting?: boolean;
  isHeaderMode?: boolean; // Pre-configure form for header account
}

// Helper function to determine smart category based on account type, account code, and parent
const getSmartCategory = (type: string, parentAccounts: Account[], parentId: number | null, accountCode?: string, accountName?: string): string => {
  // If no parent, use smart defaults based on account type
  if (!parentId) {
    const smartDefaults = {
      ASSET: 'FIXED_ASSET',        // Assets without parent are typically fixed assets
      LIABILITY: 'CURRENT_LIABILITY',
      EQUITY: 'EQUITY',
      REVENUE: 'OPERATING_REVENUE',
      EXPENSE: 'OPERATING_EXPENSE',
    };
    return smartDefaults[type as keyof typeof smartDefaults] || '';
  }

  // Find parent account
  const parent = parentAccounts.find(p => p.id === parentId);
  if (!parent) return '';

  // Smart categorization based on parent account code/name and account code pattern
  if (type === 'ASSET') {
    // Use account code pattern for better accuracy
    if (accountCode) {
      const codeNum = parseInt(accountCode);
      if (codeNum >= 1100 && codeNum < 1500) {
        return 'CURRENT_ASSET';
      }
      if (codeNum >= 1500) {
        return 'FIXED_ASSET';
      }
    }

    // Fallback to parent-based logic
    if (parent.code === '1100' || parent.name.includes('CURRENT')) {
      return 'CURRENT_ASSET';
    }
    if (parent.code === '1500' || parent.name.includes('FIXED')) {
      return 'FIXED_ASSET';
    }

    // Account name semantic analysis for better categorization
    if (accountName) {
      const nameLower = accountName.toLowerCase();
      // Current asset indicators
      if (nameLower.includes('kas') || nameLower.includes('bank') || nameLower.includes('piutang') ||
        nameLower.includes('persediaan') || nameLower.includes('inventory')) {
        return 'CURRENT_ASSET';
      }
      // Fixed asset indicators
      if (nameLower.includes('tanah') || nameLower.includes('bangunan') || nameLower.includes('peralatan') ||
        nameLower.includes('kendaraan') || nameLower.includes('mesin') || nameLower.includes('gedung')) {
        return 'FIXED_ASSET';
      }
    }

    // If parent is main ASSETS (1000), default to CURRENT_ASSET for codes < 1500
    // This is more logical as most sub-accounts under main assets are current assets
    if (parent.code === '1000') {
      return accountCode && parseInt(accountCode) >= 1500 ? 'FIXED_ASSET' : 'CURRENT_ASSET';
    }

    return 'CURRENT_ASSET'; // Default to current asset
  }

  if (type === 'LIABILITY') {
    if (parent.code === '2100' || parent.name.includes('CURRENT') || parent.name.includes('JANGKA PENDEK')) {
      return 'CURRENT_LIABILITY';
    }
    if (parent.name.includes('LONG') || parent.name.includes('TERM') || parent.name.includes('JANGKA PANJANG')) {
      return 'LONG_TERM_LIABILITY';
    }
    return 'CURRENT_LIABILITY';
  }

  if (type === 'REVENUE') {
    // Code-based: 7xxx = Other Income
    if (accountCode) {
      const codeNum = parseInt(accountCode);
      if (codeNum >= 7000 && codeNum <= 7999) {
        return 'OTHER_INCOME';
      }
    }
    if (parent.name.includes('OTHER') || parent.name.includes('NON') || parent.name.includes('LAIN')) {
      return 'OTHER_INCOME';
    }
    return 'OPERATING_REVENUE';
  }

  if (type === 'EXPENSE') {
    // Code-based: 5xxx = COGS, 6xxx = Operating, 8xxx = Other Expense, 9xxx = Tax
    if (accountCode) {
      const codeNum = parseInt(accountCode);
      if (codeNum >= 5000 && codeNum <= 5999) {
        return 'COST_OF_GOODS_SOLD';
      }
      if (codeNum >= 8000 && codeNum <= 8999) {
        return 'NON_OPERATING_EXPENSE';
      }
      if (codeNum >= 9000 && codeNum <= 9999) {
        return 'TAX_EXPENSE';
      }
    }
    if (parent.name.includes('PAJAK') || parent.name.includes('TAX')) {
      return 'TAX_EXPENSE';
    }
    if (parent.name.includes('OTHER') || parent.name.includes('NON') || parent.name.includes('LAIN')) {
      return 'OTHER_EXPENSE';
    }
    if (parent.name.includes('POKOK') || parent.name.includes('HPP') || parent.name.includes('COGS')) {
      return 'COST_OF_GOODS_SOLD';
    }
    return 'OPERATING_EXPENSE';
  }

  return 'EQUITY';
};

const AccountForm: React.FC<AccountFormProps> = ({
  account,
  parentAccounts = [],
  onSubmit,
  onCancel,
  isSubmitting = false,
  isHeaderMode = false,
}) => {
  const { user } = useAuth();
  const { t } = useTranslation();
  const { companyType, isService, isTrading } = useCompanyType();
  const isAdmin = user?.role?.toLowerCase() === 'admin';
  const toast = useToast();
  const panelBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const panelBorderColor = useColorModeValue('gray.200', 'var(--border-color)');
  const labelColor = useColorModeValue('gray.700', 'var(--text-primary)');
  const mutedTextColor = useColorModeValue('gray.600', 'var(--text-secondary)');
  const subtleIconColor = useColorModeValue('gray.500', 'gray.400');
  const infoTextColor = useColorModeValue('blue.600', 'blue.200');
  const warningTextColor = useColorModeValue('orange.600', 'orange.200');
  const infoAlertBg = useColorModeValue('blue.50', 'whiteAlpha.100');
  const infoAlertBorderColor = useColorModeValue('blue.200', 'blue.300');
  const checkboxBorderColor = useColorModeValue('gray.300', 'var(--border-color)');

  // Admin can override opening balance restrictions
  const canEditOpeningBalance = !account?.id || isAdmin;

  // Track original balance to detect changes
  const [originalBalance, setOriginalBalance] = useState<number>(0);
  const [balanceChangeConfirmed, setBalanceChangeConfirmed] = useState(false);

  // Balance change confirmation dialog
  const { isOpen: isBalanceAlertOpen, onOpen: onBalanceAlertOpen, onClose: onBalanceAlertClose } = useDisclosure();
  const cancelRef = useRef<HTMLButtonElement>(null);
  const [pendingBalanceValue, setPendingBalanceValue] = useState<number>(0);

  const [formData, setFormData] = useState<any>({
    code: '',
    name: '',
    description: '',
    type: 'ASSET',
    category: '',
    parent_id: null,
    opening_balance: 0,
    is_active: true,
    is_header: isHeaderMode,
    ...(account && {
      code: account.code,
      name: account.name,
      description: account.description,
      type: account.type,
      category: account.category,
      parent_id: account.parent_id,
      is_active: account.is_active,
      is_header: account.is_header || false,
      opening_balance: account.balance || 0,
    }),
  });

  // Debug log for form data changes
  useEffect(() => {
    console.log('Form data updated:', formData);
  }, [formData]);

  const [errors, setErrors] = useState<Record<string, string>>({});

  // Reset form when account prop changes
  useEffect(() => {
    const currentBalance = account?.balance || 0;
    setOriginalBalance(currentBalance);
    setBalanceChangeConfirmed(false);
    setFormData({
      code: '',
      name: '',
      description: '',
      type: 'ASSET',
      category: '',
      parent_id: null,
      opening_balance: 0,
      is_active: true,
      is_header: isHeaderMode,
      ...(account && {
        code: account.code,
        name: account.name,
        description: account.description,
        type: account.type,
        category: account.category,
        parent_id: account.parent_id,
        is_active: account.is_active,
        is_header: account.is_header || false,
        opening_balance: currentBalance,
      }),
    });
  }, [account, isHeaderMode]);

  const handleChange = (
    e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>
  ) => {
    const { name, value, type } = e.target;

    console.log('Form change:', { name, value, type }); // Debug log

    // Handle checkbox inputs
    if (type === 'checkbox') {
      const checked = (e.target as HTMLInputElement).checked;

      // If is_header is checked, reset opening balance to 0
      if (name === 'is_header' && checked) {
        setFormData((prev) => ({ ...prev, [name]: checked, opening_balance: 0 }));
      } else {
        setFormData((prev) => ({ ...prev, [name]: checked }));
      }
    } else {
      // Handle special field mappings
      let fieldValue: any = value;
      if (name === 'parent_id' && value === '') {
        fieldValue = null;
      } else if (name === 'parent_id' && value !== '') {
        fieldValue = parseInt(value);
      } else if (name === 'opening_balance') {
        fieldValue = parseFloat(value) || 0;
      } else if (name === 'name') {
        // Force account names to uppercase for uniformity
        fieldValue = value.toUpperCase();
      }
      setFormData((prev) => ({ ...prev, [name]: fieldValue }));
    }

    // Clear error when field is edited
    if (errors[name]) {
      setErrors((prev) => {
        const newErrors = { ...prev };
        delete newErrors[name];
        return newErrors;
      });
    }
  };

  const validateForm = (): boolean => {
    const newErrors: Record<string, string> = {};

    // Validate required fields
    if (!formData.code) newErrors.code = t('accounts.validation.codeRequired');
    if (!formData.name) newErrors.name = t('accounts.validation.nameRequired');
    if (!formData.type) newErrors.type = t('accounts.validation.typeRequired');

    // Validate code format: allow XXXX (main) or XXXX-XXX (child)
    if (formData.code) {
      const codePattern = /^\d{4}(-\d{3})?$/; // PSAK-style: 4 digits or 4 digits + dash + 3 digits
      if (!codePattern.test(formData.code)) {
        newErrors.code = t('accounts.validation.invalidCodeFormat');
      } else if (formData.parent_id) {
        // If a parent is selected, enforce different rules for header vs non-header
        const parent = parentAccounts.find(p => p.id === formData.parent_id);
        if (parent) {
          const isChildFormat = formData.code.includes('-');
          const isHeader = !!formData.is_header;

          if (isHeader) {
            // Header under a parent may use 4-digit code (e.g., 1106 under 1100)
            // If user chooses child format, still require correct prefix
            if (isChildFormat && !formData.code.startsWith(`${parent.code}-`)) {
              newErrors.code = `${t('accounts.validation.childCodePrefix')} ${parent.code}-XXX`;
            }
            // Otherwise 4-digit is acceptable; no error
          } else {
            // Non-header child can use either format - 4-digit or dashed child format
            // Only validate that if using dashed format, it must start with parent code
            if (isChildFormat && !formData.code.startsWith(`${parent.code}-`)) {
              newErrors.code = `${t('accounts.validation.childCodePrefix')} ${parent.code}-XXX`;
            }
            // For 4-digit format, we allow more flexibility as long as the account type matches
            // The main constraint is that child accounts should logically relate to parent
          }
        }
      }
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  // Handle balance change with confirmation
  const handleBalanceChange = (newValue: number) => {
    if (account?.id && newValue !== originalBalance && !balanceChangeConfirmed) {
      // Show confirmation dialog
      setPendingBalanceValue(newValue);
      onBalanceAlertOpen();
    } else {
      setFormData((prev: any) => ({ ...prev, opening_balance: newValue }));
    }
  };

  const confirmBalanceChange = () => {
    setBalanceChangeConfirmed(true);
    setFormData((prev: any) => ({ ...prev, opening_balance: pendingBalanceValue }));
    onBalanceAlertClose();
    toast({
      title: t('accounts.form.balanceChangeWarning') || 'Saldo diubah',
      description: t('accounts.form.balanceChangeConfirmed') || 'Pastikan perubahan saldo sesuai dengan data transaksi yang ada.',
      status: 'warning',
      duration: 5000,
      isClosable: true,
    });
  };

  const cancelBalanceChange = () => {
    // Revert to original balance
    setFormData((prev: any) => ({ ...prev, opening_balance: originalBalance }));
    onBalanceAlertClose();
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    if (validateForm()) {
      const smartCategory = getSmartCategory(formData.type, parentAccounts, formData.parent_id, formData.code, formData.name);
      const dataWithSmartCategory = {
        ...formData,
        category: smartCategory
      };

      // If editing and balance was NOT changed, remove opening_balance from payload
      // to prevent backend from overwriting the current balance
      if (account?.id && !balanceChangeConfirmed && formData.opening_balance === originalBalance) {
        delete dataWithSmartCategory.opening_balance;
      }

      console.log('Smart category assigned:', smartCategory, 'for type:', formData.type, 'parent:', formData.parent_id, 'code:', formData.code, 'name:', formData.name);

      onSubmit(dataWithSmartCategory);
    }
  };

  return (
    <>
      <form onSubmit={handleSubmit}>
        {isHeaderMode && !account && (
          <Alert status="info" mb={4} borderRadius="md">
            <AlertIcon />
            <Box>
              <AlertTitle fontSize="sm">{t('accounts.form.creatingHeader')}</AlertTitle>
              <AlertDescription fontSize="xs">
                {t('accounts.form.creatingHeaderDesc')}
              </AlertDescription>
            </Box>
          </Alert>
        )}
        {/* Company Type Badge — always visible */}
        <HStack mb={4} p={2} bg={panelBg} borderRadius="md" borderWidth="1px" borderColor={panelBorderColor}>
          <Icon as={FiInfo} color="purple.500" boxSize={4} />
          <Text fontSize="sm" color={mutedTextColor}>
            {t('accounts.form.currentCompanyType')}:
          </Text>
          <Badge colorScheme="purple" fontSize="sm" px={2} py={0.5} borderRadius="md">
            {companyType === 'SERVICE' ? 'Jasa' : companyType === 'TRADING' ? 'Dagang' : 'Manufaktur'}
          </Badge>
        </HStack>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormField
            id="code"
            label={t('accounts.accountCode')}
            value={formData.code || ''}
            onChange={handleChange}
            placeholder={t('accounts.form.enterAccountCode')}
            required
            error={errors.code}
            name="code"
          />

          <FormField
            id="name"
            label={t('accounts.accountName')}
            value={formData.name || ''}
            onChange={handleChange}
            placeholder={t('accounts.form.enterAccountName')}
            required
            error={errors.name}
            name="name"
          />

          <FormField
            id="type"
            label={t('accounts.accountType')}
            type="select"
            value={formData.type || ''}
            onChange={handleChange}
            required
            error={errors.type}
            options={[
              { value: 'ASSET', label: t('accounts.types.asset') },
              { value: 'LIABILITY', label: t('accounts.types.liability') },
              { value: 'EQUITY', label: t('accounts.types.equity') },
              { value: 'REVENUE', label: t('accounts.types.revenue') },
              { value: 'EXPENSE', label: t('accounts.types.expense') },
            ]}
            name="type"
          />


          <FormField
            id="parent_id"
            label={t('accounts.parentAccount')}
            type="select"
            value={formData.parent_id || ''}
            onChange={handleChange}
            options={[
              { value: '', label: t('accounts.form.noParent') },
              ...parentAccounts
                .filter(parent => {
                  // Don't show the current account itself as a parent option
                  if (account && parent.id === account.id) return false;

                  // STRICT: Only allow parent accounts of the SAME type
                  // This prevents illogical hierarchies (e.g., ASSET under EQUITY parent)
                  if (formData.type && parent.type !== formData.type) {
                    return false;
                  }

                  // Prefer header accounts as parents
                  return true;
                })
                .map((parent) => ({
                  value: parent.id.toString(),
                  label: `${parent.code} - ${parent.name}${parent.is_header ? ` (${t('accounts.form.header')})` : ''}`,
                })),
            ]}
            name="parent_id"
          />

          <Box>
            <HStack mb={2}>
              <Text fontSize="sm" fontWeight="medium" color={labelColor}>{t('accounts.form.openingBalance')}</Text>
              <Tooltip
                label={t('accounts.form.openingBalanceTooltip')}
                hasArrow
              >
                <span>
                  <Icon as={FiInfo} color={subtleIconColor} boxSize={3} />
                </span>
              </Tooltip>
              {account && (
                <Badge
                  colorScheme={canEditOpeningBalance ? 'green' : 'orange'}
                  size="sm"
                  variant="subtle"
                >
                  <Icon as={canEditOpeningBalance ? FiUnlock : FiLock} mr={1} />
                  {canEditOpeningBalance ? t('accounts.form.editable') : t('accounts.form.editRestricted')}
                  {isAdmin && account?.id && (
                    <Text as="span" ml={1} fontSize="xs">({t('accounts.form.adminOverride')})</Text>
                  )}
                </Badge>
              )}
            </HStack>
            <CurrencyInput
              value={formData.opening_balance || 0}
              onChange={(value) => handleBalanceChange(value)}
              placeholder={t('accounts.form.openingBalancePlaceholder')}
              size="md"
              min={0}
              isDisabled={!canEditOpeningBalance || formData.is_header}
            />
            {account?.id && formData.opening_balance !== originalBalance && balanceChangeConfirmed && (
              <Alert status="warning" size="sm" mt={2}>
                <AlertIcon boxSize={3} />
                <Box fontSize="xs">
                  <Text fontWeight="medium" color={warningTextColor}>
                    {t('accounts.form.balanceChanged') || 'Saldo telah diubah!'}
                  </Text>
                  <Text color={mutedTextColor}>
                    {t('accounts.form.originalBalance') || 'Saldo sebelumnya'}: Rp {originalBalance.toLocaleString('id-ID')}
                  </Text>
                </Box>
              </Alert>
            )}
            {account?.id && !canEditOpeningBalance && (
              <Alert status="warning" size="sm" mt={2}>
                <AlertIcon boxSize={3} />
                <Box fontSize="xs">
                  <Text fontWeight="medium">{t('accounts.form.openingBalanceLocked')}</Text>
                  <Text color={mutedTextColor}>
                    {t('accounts.form.openingBalanceLockedDesc')}
                  </Text>
                </Box>
              </Alert>
            )}
            {account?.id && isAdmin && (
              <Alert status="info" size="sm" mt={2}>
                <AlertIcon boxSize={3} />
                <Box fontSize="xs">
                  <Text fontWeight="medium" color={infoTextColor}>{t('accounts.form.adminOverrideActive')}</Text>
                  <Text color={infoTextColor}>
                    {t('accounts.form.adminOverrideDesc')}
                  </Text>
                </Box>
              </Alert>
            )}
            {formData.is_header && (
              <Alert status="warning" size="sm" mt={2}>
                <AlertIcon boxSize={3} />
                <Box fontSize="xs">
                  <Text fontWeight="medium" color={warningTextColor}>{t('accounts.form.headerRestriction')}</Text>
                  <Text color={warningTextColor}>
                    {t('accounts.form.headerRestrictionDesc')}
                  </Text>
                </Box>
              </Alert>
            )}
          </Box>

          <div className="md:col-span-2">
            <FormField
              id="description"
              label={t('accounts.description')}
              type="textarea"
              value={formData.description || ''}
              onChange={handleChange}
              placeholder={t('accounts.form.enterDescription')}
              name="description"
            />
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="flex items-center">
              <Checkbox
                id="is_active"
                isChecked={formData.is_active}
                onChange={handleChange}
                name="is_active"
                colorScheme="blue"
                borderColor={checkboxBorderColor}
              >
                <Text fontSize="sm" color={labelColor}>
                  {t('accounts.isActive')}
                </Text>
              </Checkbox>
            </div>

            <div className="flex items-center gap-2">
              <Checkbox
                id="is_header"
                isChecked={formData.is_header}
                onChange={handleChange}
                name="is_header"
                isDisabled={isHeaderMode && !account}
                colorScheme="blue"
                borderColor={checkboxBorderColor}
              >
                <Text fontSize="sm" color={labelColor}>
                  {t('accounts.form.headerAccount')}
                </Text>
              </Checkbox>
              {isHeaderMode && !account && (
                <Badge colorScheme="blue" size="sm">
                  {t('accounts.form.forcedOn')}
                </Badge>
              )}
              <Tooltip
                label={t('accounts.form.headerAccountTooltip')}
                hasArrow
              >
                <span>
                  <Icon as={FiInfo} color={subtleIconColor} boxSize={3} />
                </span>
              </Tooltip>
            </div>
          </div>
        </div>

        {/* Company Type Awareness Warning */}
        {formData.code && (() => {
          const codeNum = parseInt(formData.code);
          const warnings: string[] = [];

          // Check if account code conflicts with company type
          if (isService && !isNaN(codeNum)) {
            if ((codeNum >= 1103 && codeNum <= 1103) || formData.name?.includes('PERSEDIAAN')) {
              warnings.push(t('accounts.form.companyTypeWarning.inventoryForService'));
            }
            if (codeNum >= 5100 && codeNum <= 5103 && (formData.name?.includes('PEMBELIAN') || formData.name?.includes('BARANG DAGANG'))) {
              warnings.push(t('accounts.form.companyTypeWarning.cogsForService'));
            }
          }
          if (isTrading && !isNaN(codeNum)) {
            if (formData.name?.includes('JASA KONSULTASI') || formData.name?.includes('JASA PROFESIONAL')) {
              warnings.push(t('accounts.form.companyTypeWarning.serviceForTrading'));
            }
          }

          return warnings.length > 0 ? (
            <Alert status="warning" variant="left-accent" mt={4}>
              <AlertIcon />
              <Box>
                <AlertTitle fontSize="sm">{t('accounts.form.companyTypeMismatch')}</AlertTitle>
                <AlertDescription fontSize="xs">
                  <Text>Tipe perusahaan saat ini: <Badge colorScheme="purple">{companyType}</Badge></Text>
                  {warnings.map((w, i) => (
                    <Text key={i} mt={1} color={warningTextColor}>⚠️ {w}</Text>
                  ))}
                </AlertDescription>
              </Box>
            </Alert>
          ) : null;
        })()}

        {/* Smart Category Indicator with Tooltip */}
        {formData.type && (
          <Alert
            status="info"
            variant="left-accent"
            mt={4}
            bg={infoAlertBg}
            border="1px solid"
            borderColor={infoAlertBorderColor}
            borderRadius="md"
          >
            <AlertIcon />
            <Box>
              <AlertTitle fontSize="sm">{t('accounts.form.smartCategory')}</AlertTitle>
              <AlertDescription fontSize="xs">
                <HStack spacing={2} mt={2}>
                  <Text>{t('accounts.form.categoryWillBe')}</Text>
                  <Badge colorScheme="blue" variant="solid">
                    {getSmartCategory(formData.type, parentAccounts, formData.parent_id, formData.code, formData.name).replace(/_/g, ' ')}
                  </Badge>
                </HStack>
                <Tooltip
                  label={t('accounts.form.categoryTooltip')}
                  hasArrow
                  placement="top"
                >
                  <HStack spacing={1} mt={2} cursor="help">
                    <Icon as={FiInfo} color={infoTextColor} boxSize={3} />
                    <Text color={infoTextColor}>{t('accounts.form.whatIsCategory')}</Text>
                  </HStack>
                </Tooltip>
              </AlertDescription>
            </Box>
          </Alert>
        )}

        <div className="mt-6 flex justify-end space-x-3">
          <Button
            type="button"
            variant="outline"
            onClick={onCancel}
            isDisabled={isSubmitting}
          >
            {t('common.cancel')}
          </Button>
          <Button
            type="submit"
            colorScheme="brand"
            isLoading={isSubmitting}
          >
            {account?.id ? t('accounts.updateAccount') : t('accounts.createAccount')}
          </Button>
        </div>
      </form>

      {/* Balance Change Confirmation Dialog */}
      <AlertDialog
        isOpen={isBalanceAlertOpen}
        leastDestructiveRef={cancelRef as any}
        onClose={cancelBalanceChange}
        isCentered
      >
        <AlertDialogOverlay>
          <AlertDialogContent>
            <AlertDialogHeader fontSize="lg" fontWeight="bold" color={warningTextColor}>
              ⚠️ {t('accounts.form.confirmBalanceChange') || 'Konfirmasi Perubahan Saldo'}
            </AlertDialogHeader>

            <AlertDialogBody>
              <Text mb={3}>
                {t('accounts.form.balanceChangeWarningMsg') || 'Anda akan mengubah saldo akun ini. Perubahan saldo secara manual dapat menyebabkan ketidaksesuaian dengan data transaksi yang ada.'}
              </Text>
              <Alert status="warning" borderRadius="md" mb={3}>
                <AlertIcon />
                <Box fontSize="sm">
                  <Text fontWeight="bold">
                    {t('accounts.form.currentBalance') || 'Saldo Saat Ini'}: Rp {originalBalance.toLocaleString('id-ID')}
                  </Text>
                  <Text>
                    {t('accounts.form.newBalance') || 'Saldo Baru'}: Rp {pendingBalanceValue.toLocaleString('id-ID')}
                  </Text>
                </Box>
              </Alert>
              <Text fontSize="sm" color={mutedTextColor}>
                {t('accounts.form.balanceChangeAdvice') || 'Disarankan untuk menggunakan jurnal penyesuaian jika ingin mengubah saldo akun.'}
              </Text>
            </AlertDialogBody>

            <AlertDialogFooter>
              <Button ref={cancelRef as any} onClick={cancelBalanceChange}>
                {t('common.cancel') || 'Batal'}
              </Button>
              <Button colorScheme="orange" onClick={confirmBalanceChange} ml={3}>
                {t('accounts.form.confirmChange') || 'Ya, Ubah Saldo'}
              </Button>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialogOverlay>
      </AlertDialog>
    </>
  );
};

export default AccountForm; 
