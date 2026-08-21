'use client';

import React, { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import {
  Alert,
  AlertDescription,
  AlertIcon,
  AlertTitle,
  Badge,
  Box,
  Button,
  Divider,
  Flex,
  FormControl,
  FormLabel,
  HStack,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  NumberDecrementStepper,
  NumberIncrementStepper,
  NumberInput,
  NumberInputField,
  NumberInputStepper,
  Select,
  SimpleGrid,
  Stat,
  StatHelpText,
  StatLabel,
  StatNumber,
  Text,
  Textarea,
  useColorModeValue,
  useToast,
  VStack,
} from '@chakra-ui/react';
import cashbankService, {
  CashBank,
  CashBankCounterpartyAccount,
  WithdrawalRequest,
  getAvailableBalance,
  getBookBalanceInIDR,
} from '@/services/cashbankService';
import { useAuth } from '@/contexts/AuthContext';
import { isFinanceManagerRole, normalizeRole } from '@/utils/roles';
import { useTranslation } from '@/hooks/useTranslation';
import {
  getCashBankAmountPrecision,
  isCashBankForeignCurrency,
  normalizeCashBankCurrency,
} from '@/utils/cashBankCurrency';

interface DepositWithdrawalFormProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  account: CashBank | null;
  mode: 'withdrawal';
}

interface ApiErrorShape {
  response?: {
    data?: {
      details?: string;
      message?: string;
      error?: string;
    };
  };
  code?: string;
  message?: string;
}

const getCurrencyCode = (account?: CashBank | null): string => normalizeCashBankCurrency(account?.currency);

const formatNativeAmount = (currency: string, amount: number, locale: string): string => `${currency} ${Math.abs(amount || 0).toLocaleString(locale, {
  minimumFractionDigits: getCashBankAmountPrecision(currency),
  maximumFractionDigits: getCashBankAmountPrecision(currency),
})}`;

const formatIDRAmount = (amount: number, locale: string): string => `IDR ${(amount || 0).toLocaleString(locale, {
  minimumFractionDigits: 0,
  maximumFractionDigits: 2,
})}`;

const DepositWithdrawalForm: React.FC<DepositWithdrawalFormProps> = ({
  isOpen,
  onClose,
  onSuccess,
  account,
}) => {
  const router = useRouter();
  const { user } = useAuth();
  const [formData, setFormData] = useState({
    date: new Date().toISOString().split('T')[0],
    amount: 0,
    reference: '',
    notes: '',
    target_account_id: '',
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loadingAccounts, setLoadingAccounts] = useState(false);
  const [targetAccountsLoadError, setTargetAccountsLoadError] = useState(false);
  const [targetAccountsLoadAttempt, setTargetAccountsLoadAttempt] = useState(0);
  const [targetAccounts, setTargetAccounts] = useState<CashBankCounterpartyAccount[]>([]);

  const toast = useToast();
  const { t, language } = useTranslation();
  const numberLocale = language === 'en' ? 'en-US' : 'id-ID';
  const formatNative = (currency: string, amount: number) => formatNativeAmount(currency, amount, numberLocale);
  const formatIDR = (amount: number) => formatIDRAmount(amount, numberLocale);
  const modalBg = useColorModeValue('white', 'gray.800');
  const modalHeaderBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const modalFooterBg = useColorModeValue('gray.50', 'gray.900');
  const borderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const sectionTitleColor = useColorModeValue('gray.700', 'gray.100');
  const bodyTextColor = useColorModeValue('gray.800', 'whiteAlpha.900');
  const mutedTextColor = useColorModeValue('gray.600', 'whiteAlpha.800');
  const helperTextColor = useColorModeValue('gray.500', 'whiteAlpha.700');
  const panelBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const panelBorderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const previewBg = useColorModeValue('orange.50', 'rgba(237, 137, 54, 0.12)');
  const previewBorderColor = useColorModeValue('orange.200', 'orange.700');
  const fieldBg = useColorModeValue('white', 'whiteAlpha.50');
  const fieldBorderColor = useColorModeValue('gray.300', 'whiteAlpha.300');
  const fieldHoverBorderColor = useColorModeValue('gray.400', 'whiteAlpha.400');
  const fieldFocusBorderColor = useColorModeValue('orange.500', 'orange.300');
  const fieldFocusShadow = `0 0 0 1px ${fieldFocusBorderColor}`;
  const placeholderColor = useColorModeValue('gray.500', 'whiteAlpha.500');
  const fieldStyles = {
    bg: fieldBg,
    color: bodyTextColor,
    borderColor: fieldBorderColor,
    _hover: { borderColor: fieldHoverBorderColor },
    _focusVisible: {
      borderColor: fieldFocusBorderColor,
      boxShadow: fieldFocusShadow,
    },
    _placeholder: { color: placeholderColor },
  };

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    let cancelled = false;

    const loadWithdrawalTargets = async () => {
      try {
        setLoadingAccounts(true);
        setTargetAccountsLoadError(false);
        const accounts = await cashbankService.getOwnerDrawingAccounts();
        if (!cancelled) {
          setTargetAccounts(accounts);
        }
      } catch (loadError) {
        console.error('Error loading withdrawal target accounts:', loadError);
        if (!cancelled) {
          setTargetAccounts([]);
          setTargetAccountsLoadError(true);
        }
      } finally {
        if (!cancelled) {
          setLoadingAccounts(false);
        }
      }
    };

    void loadWithdrawalTargets();

    return () => {
      cancelled = true;
    };
  }, [isOpen, targetAccountsLoadAttempt]);

  const retryLoadOwnerDrawingAccounts = () => {
    setTargetAccountsLoadAttempt(previous => previous + 1);
  };

  const handleInputChange = (field: string, value: string | number) => {
    setFormData(prev => ({
      ...prev,
      [field]: value,
    }));
    setError(null);
  };

  const handleSubmit = async () => {
    try {
      setLoading(true);
      setError(null);

      if (!account) {
        throw new Error(t('cashBank.withdrawalForm.errors.noAccount'));
      }

      if (formData.amount <= 0) {
        throw new Error(t('cashBank.withdrawalForm.errors.amountMustBeGreaterThanZero'));
      }

      const availableBalance = getAvailableBalance(account);
      if (formData.amount > availableBalance) {
        throw new Error(t('cashBank.withdrawalForm.errors.insufficientBalance', {
          available: formatNative(getCurrencyCode(account), availableBalance),
        }));
      }

      if (!formData.target_account_id) {
        throw new Error(t('cashBank.withdrawalForm.errors.targetRequired'));
      }

      const normalizedRole = normalizeRole(user?.role);
      const canPostOwnerDrawing = isFinanceManagerRole(normalizedRole) || normalizedRole === 'director';
      if (!canPostOwnerDrawing) {
        throw new Error(t('cashBank.withdrawalForm.errors.ownerDrawingRole'));
      }
      if (!formData.reference.trim()) {
        throw new Error(t('cashBank.withdrawalForm.errors.ownerDrawingReference'));
      }
      if (!formData.notes.trim()) {
        throw new Error(t('cashBank.withdrawalForm.errors.ownerDrawingNotes'));
      }

      const requestData: WithdrawalRequest = {
        account_id: account.id,
        date: formData.date,
        amount: formData.amount,
        reference: formData.reference,
        notes: formData.notes,
        target_account_id: parseInt(formData.target_account_id, 10),
        purpose: 'OWNER_DRAWING',
      };

      await cashbankService.processWithdrawal(requestData);

      const selectedTargetAccount = targetAccounts.find(
        entry => entry.id === parseInt(formData.target_account_id, 10),
      );
      const carryingRate = getCurrencyCode(account) === 'IDR'
        ? 1
        : (account.balance > 0 && getBookBalanceInIDR(account) > 0
          ? getBookBalanceInIDR(account) / account.balance
          : (account.last_exchange_rate || 1));
      const bookImpactIDR = getCurrencyCode(account) === 'IDR'
        ? formData.amount
        : formData.amount * carryingRate;

      toast({
        title: t('cashBank.withdrawalForm.success.title'),
        description: (
          <Box>
            <Text fontSize="sm" fontWeight="bold">
              {t('cashBank.withdrawalForm.success.description', {
                amount: formatNative(getCurrencyCode(account), formData.amount),
                accountName: account.name,
              })}
            </Text>
            <Text fontSize="xs" color={mutedTextColor} mt={1}>
              {t('cashBank.withdrawalForm.success.debit', {
                accountName: selectedTargetAccount?.name || t('cashBank.withdrawalForm.journalPreview.counterAccount'),
                amount: formatIDR(bookImpactIDR),
              })}
            </Text>
            <Text fontSize="xs" color={mutedTextColor}>
              {t('cashBank.withdrawalForm.success.credit', {
                accountName: account.name,
                amount: formatNative(getCurrencyCode(account), formData.amount),
              })}
            </Text>
            <Text fontSize="xs" color="orange.500" mt={1} fontWeight="bold">
              {t('cashBank.withdrawalForm.success.balanced')}
            </Text>
          </Box>
        ),
        status: 'success',
        duration: 5000,
        isClosable: true,
      });

      onSuccess();
      handleClose();
    } catch (submitError: unknown) {
      console.error('Error processing withdrawal:', submitError);
      const normalizedError = submitError as ApiErrorShape;

      let errorMessage =
        normalizedError.response?.data?.details ||
        normalizedError.response?.data?.error ||
        normalizedError.response?.data?.message ||
        normalizedError.message ||
        t('cashBank.withdrawalForm.errors.failedToProcess');
      let errorTitle = t('cashBank.transactionFailed');

      if (normalizedError.code === 'ECONNABORTED' || normalizedError.message?.includes('timeout') || normalizedError.message?.includes('exceeded')) {
        errorTitle = t('cashBank.transactionTimeout');
        errorMessage = t('cashBank.withdrawalForm.errors.timeout');
      }

      setError(errorMessage);
      toast({
        title: errorTitle,
        description: errorMessage,
        status: 'error',
        duration: 8000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    setError(null);
    setFormData({
      date: new Date().toISOString().split('T')[0],
      amount: 0,
      reference: '',
      notes: '',
      target_account_id: '',
    });
    onClose();
  };

  const handleGuidedNavigation = (path: string) => {
    handleClose();
    router.push(path);
  };

  if (!account) {
    return null;
  }

  const currencyCode = getCurrencyCode(account);
  const availableBalance = getAvailableBalance(account);
  const currentBookBalanceIDR = getBookBalanceInIDR(account);
  const currentBookRate = currencyCode === 'IDR'
    ? 1
    : (account.balance > 0 && currentBookBalanceIDR > 0 ? currentBookBalanceIDR / account.balance : (account.last_exchange_rate || 1));
  const bookImpactIDR = currencyCode === 'IDR' ? formData.amount : formData.amount * currentBookRate;
  const newBalance = account.balance - formData.amount;
  const newBookBalanceIDR = currentBookBalanceIDR - bookImpactIDR;
  const isForeignCurrencyAccount = isCashBankForeignCurrency(currencyCode);
  const normalizedRole = normalizeRole(user?.role);
  const canPostOwnerDrawing = isFinanceManagerRole(normalizedRole) || normalizedRole === 'director';
  const selectedTargetAccount = targetAccounts.find(
    entry => entry.id === parseInt(formData.target_account_id || '0', 10),
  );

  return (
    <Modal isOpen={isOpen} onClose={handleClose} size="2xl" scrollBehavior="inside">
      <ModalOverlay bg="blackAlpha.600" />
      <ModalContent bg={modalBg} border="1px solid" borderColor={borderColor}>
        <ModalHeader bg={modalHeaderBg} borderBottom="1px solid" borderColor={borderColor}>
          <Flex alignItems="center" gap={3}>
            <Box>
              <Text fontSize="lg" fontWeight="bold" color={sectionTitleColor}>
                {t('cashBank.withdrawalForm.ownerDrawingTitle')}
              </Text>
              <Text fontSize="sm" color={helperTextColor} fontFamily="mono">
                {account.name} ({account.code})
              </Text>
            </Box>
            <Badge colorScheme="orange" variant="solid" fontSize="xs">
              {t('cashBank.withdrawalForm.autoJournal')}
            </Badge>
          </Flex>
        </ModalHeader>

        <ModalBody>
          {error && (
            <Alert status="error" mb={4} variant="subtle" borderRadius="md">
              <AlertIcon />
              <Box>
                <Text fontSize="sm" fontWeight="medium">
                  {error}
                </Text>
              </Box>
            </Alert>
          )}

          <VStack spacing={6} align="stretch">
            <Box>
              <Text fontSize="md" fontWeight="semibold" color={sectionTitleColor} mb={3}>
                {t('cashBank.accountInformation')}
              </Text>
              <Box bg={panelBg} p={4} borderRadius="md" border="1px solid" borderColor={panelBorderColor}>
                <Flex justify="space-between" align="center" mb={2}>
                  <Box>
                    <Text fontSize="sm" color={mutedTextColor} mb={1}>
                      {t('cashBank.accountName')}
                    </Text>
                    <Text fontWeight="medium" color={bodyTextColor}>{account.name}</Text>
                  </Box>
                  <Badge colorScheme={account.type === 'CASH' ? 'green' : 'blue'}>
                    {account.type === 'CASH'
                      ? t('cashBank.withdrawalForm.accountTypeCash')
                      : t('cashBank.withdrawalForm.accountTypeBank')}
                  </Badge>
                </Flex>

                <Stat>
                  <StatLabel color={mutedTextColor}>
                    {t('cashBank.withdrawalForm.currentNativeBalance')}
                  </StatLabel>
                  <StatNumber color={account.balance < 0 ? 'red.500' : 'green.600'} fontFamily="mono">
                    {formatNative(currencyCode, account.balance)}
                    {account.balance < 0 && ' (Dr)'}
                  </StatNumber>
                  <StatHelpText color={helperTextColor}>
                    {account.balance < 0
                      ? t('cashBank.overdraft')
                      : account.balance > 0
                        ? t('cashBank.withdrawalForm.available')
                        : t('cashBank.zeroBalance')}
                  </StatHelpText>
                  <StatHelpText color={helperTextColor}>
                    {t('cashBank.withdrawalForm.ledgerBookBalance', {
                      amount: formatIDR(currentBookBalanceIDR),
                    })}
                  </StatHelpText>
                  <StatHelpText color="orange.500">
                    {t('cashBank.withdrawalForm.availableToUse', {
                      amount: formatNative(currencyCode, availableBalance),
                    })}
                  </StatHelpText>
                </Stat>
              </Box>
            </Box>

            <Alert status={isForeignCurrencyAccount ? 'info' : 'success'} borderRadius="md" alignItems="flex-start">
              <AlertIcon mt={1} />
              <Box>
                <Flex gap={2} mb={2} wrap="wrap">
                  <Badge colorScheme="blue" variant="subtle">
                    {t('cashBank.withdrawalForm.nativeBadge', { currency: currencyCode })}
                  </Badge>
                  <Badge colorScheme="green" variant="subtle">
                    {t('cashBank.withdrawalForm.bookIdrBadge')}
                  </Badge>
                </Flex>
                <Text fontSize="sm" fontWeight="medium">
                  {isForeignCurrencyAccount
                    ? t('cashBank.withdrawalForm.foreignCurrencyImpact', { currency: currencyCode })
                    : t('cashBank.withdrawalForm.idrImpact')}
                </Text>
                <Text fontSize="xs" color={mutedTextColor} mt={1}>
                  {isForeignCurrencyAccount
                    ? t('cashBank.withdrawalForm.foreignCurrencyRateHelp')
                    : t('cashBank.withdrawalForm.idrBalanceHelp')}
                </Text>
              </Box>
            </Alert>

            <Alert status="warning" borderRadius="md" alignItems="flex-start">
              <AlertIcon mt={1} />
              <Box>
                <AlertTitle fontSize="sm" mb={1}>
                  {t('cashBank.withdrawalForm.ownerDrawingOnlyTitle')}
                </AlertTitle>
                <AlertDescription fontSize="sm">
                  {t('cashBank.withdrawalForm.ownerDrawingOnlyDescription')}
                </AlertDescription>
              </Box>
            </Alert>

            <Box>
              <Text fontSize="md" fontWeight="semibold" color={sectionTitleColor} mb={1}>
                {t('cashBank.withdrawalForm.guidedWorkflows.title')}
              </Text>
              <Text fontSize="sm" color={mutedTextColor} mb={3}>
                {t('cashBank.withdrawalForm.guidedWorkflows.description')}
              </Text>
              <SimpleGrid columns={{ base: 1, md: 2 }} spacing={3}>
                <Box bg={panelBg} p={4} borderRadius="md" border="1px solid" borderColor={panelBorderColor}>
                  <Text fontSize="sm" fontWeight="semibold" color={bodyTextColor}>
                    {t('cashBank.withdrawalForm.guidedWorkflows.expense.title')}
                  </Text>
                  <Text fontSize="xs" color={mutedTextColor} mt={1}>
                    {t('cashBank.withdrawalForm.guidedWorkflows.expense.description')}
                  </Text>
                  <Button
                    size="sm"
                    variant="outline"
                    colorScheme="blue"
                    mt={3}
                    onClick={() => handleGuidedNavigation('/payments')}
                  >
                    {t('cashBank.withdrawalForm.guidedWorkflows.expense.action')}
                  </Button>
                </Box>

                <Box bg={panelBg} p={4} borderRadius="md" border="1px solid" borderColor={panelBorderColor}>
                  <Text fontSize="sm" fontWeight="semibold" color={bodyTextColor}>
                    {t('cashBank.withdrawalForm.guidedWorkflows.vendor.title')}
                  </Text>
                  <Text fontSize="xs" color={mutedTextColor} mt={1}>
                    {t('cashBank.withdrawalForm.guidedWorkflows.vendor.description')}
                  </Text>
                  <Button
                    size="sm"
                    variant="outline"
                    colorScheme="blue"
                    mt={3}
                    onClick={() => handleGuidedNavigation('/payments')}
                  >
                    {t('cashBank.withdrawalForm.guidedWorkflows.vendor.action')}
                  </Button>
                </Box>

                <Box bg={panelBg} p={4} borderRadius="md" border="1px solid" borderColor={panelBorderColor}>
                  <Text fontSize="sm" fontWeight="semibold" color={bodyTextColor}>
                    {t('cashBank.withdrawalForm.guidedWorkflows.asset.title')}
                  </Text>
                  <Text fontSize="xs" color={mutedTextColor} mt={1}>
                    {t('cashBank.withdrawalForm.guidedWorkflows.asset.description')}
                  </Text>
                  <Button
                    size="sm"
                    variant="outline"
                    colorScheme="purple"
                    mt={3}
                    onClick={() => handleGuidedNavigation('/purchases')}
                  >
                    {t('cashBank.withdrawalForm.guidedWorkflows.asset.action')}
                  </Button>
                </Box>

                <Box bg={panelBg} p={4} borderRadius="md" border="1px solid" borderColor={panelBorderColor}>
                  <Text fontSize="sm" fontWeight="semibold" color={bodyTextColor}>
                    {t('cashBank.withdrawalForm.guidedWorkflows.payroll.title')}
                  </Text>
                  <Text fontSize="xs" color={mutedTextColor} mt={1}>
                    {t('cashBank.withdrawalForm.guidedWorkflows.payroll.description')}
                  </Text>
                  <Button
                    size="sm"
                    variant="outline"
                    colorScheme="teal"
                    mt={3}
                    onClick={() => handleGuidedNavigation('/payroll')}
                  >
                    {t('cashBank.withdrawalForm.guidedWorkflows.payroll.action')}
                  </Button>
                </Box>

                <Box
                  bg={panelBg}
                  p={4}
                  borderRadius="md"
                  border="1px solid"
                  borderColor={panelBorderColor}
                  gridColumn={{ md: '1 / -1' }}
                >
                  <Text fontSize="sm" fontWeight="semibold" color={bodyTextColor}>
                    {t('cashBank.withdrawalForm.guidedWorkflows.transfer.title')}
                  </Text>
                  <Text fontSize="xs" color={mutedTextColor} mt={1}>
                    {t('cashBank.withdrawalForm.guidedWorkflows.transfer.description', {
                      accountName: account.name,
                    })}
                  </Text>
                  <Button size="sm" variant="outline" colorScheme="orange" mt={3} onClick={handleClose}>
                    {t('cashBank.withdrawalForm.guidedWorkflows.transfer.action')}
                  </Button>
                </Box>
              </SimpleGrid>
            </Box>

            <Box>
              <Text fontSize="md" fontWeight="semibold" color={sectionTitleColor} mb={3}>
                {t('cashBank.withdrawalForm.ownerDrawingDetails')}
              </Text>

              <VStack spacing={4} align="stretch">
                <FormControl isRequired>
                  <FormLabel>{t('payments.form.paymentDate')}</FormLabel>
                  <Input
                    type="date"
                    value={formData.date}
                    onChange={(e) => handleInputChange('date', e.target.value)}
                    {...fieldStyles}
                  />
                </FormControl>

                <FormControl isRequired>
                  <FormLabel>{t('payments.form.amount')} ({currencyCode})</FormLabel>
                  <NumberInput
                    value={formData.amount}
                    onChange={(_, value) => handleInputChange('amount', value || 0)}
                    min={0}
                    precision={getCashBankAmountPrecision(currencyCode)}
                  >
                    <NumberInputField fontFamily="mono" {...fieldStyles} />
                    <NumberInputStepper>
                      <NumberIncrementStepper />
                      <NumberDecrementStepper />
                    </NumberInputStepper>
                  </NumberInput>
                  {formData.amount > availableBalance && (
                    <Text fontSize="xs" color="red.500" mt={1}>
                      {t('cashBank.withdrawalForm.amountExceedsAvailable')}
                    </Text>
                  )}
                </FormControl>

                <FormControl isRequired>
                  <FormLabel>{t('payments.form.referenceNumber')}</FormLabel>
                  <Input
                    value={formData.reference}
                    onChange={(e) => handleInputChange('reference', e.target.value)}
                    placeholder={t('common.placeholders.referenceNumber')}
                    {...fieldStyles}
                  />
                  <Text fontSize="xs" color={helperTextColor} mt={1}>
                    {t('cashBank.withdrawalForm.ownerReferenceHelp')}
                  </Text>
                </FormControl>

                <FormControl isRequired>
                  <FormLabel>{t('payments.form.notes')}</FormLabel>
                  <Textarea
                    value={formData.notes}
                    onChange={(e) => handleInputChange('notes', e.target.value)}
                    placeholder={t('common.placeholders.notes')}
                    rows={3}
                    {...fieldStyles}
                  />
                  <Text fontSize="xs" color={helperTextColor} mt={1}>
                    {t('cashBank.withdrawalForm.ownerNotesHelp')}
                  </Text>
                </FormControl>
              </VStack>
            </Box>

            <Box>
              <Text fontSize="md" fontWeight="semibold" color={sectionTitleColor} mb={3}>
                {t('cashBank.withdrawalForm.ownerDrawingJournalMapping')}
              </Text>
              <Text fontSize="sm" color={mutedTextColor} mb={3}>
                {t('cashBank.withdrawalForm.ownerDrawingMappingHelp')}
              </Text>

              <VStack spacing={4} align="stretch">
                <Alert status="info" borderRadius="md" alignItems="flex-start">
                  <AlertIcon mt={1} />
                  <Box>
                    <AlertTitle fontSize="sm" mb={1}>
                      {t('cashBank.withdrawalForm.mappingTitle')}
                    </AlertTitle>
                    <AlertDescription fontSize="sm">
                      {t('cashBank.withdrawalForm.ownerDrawingMappingDescription', {
                        posting: t('cashBank.withdrawalForm.purposes.ownerDrawing.posting'),
                      })}
                    </AlertDescription>
                  </Box>
                </Alert>

                <Alert status="warning" borderRadius="md" alignItems="flex-start">
                  <AlertIcon mt={1} />
                  <Box>
                    <AlertTitle fontSize="sm" mb={1}>
                      {t('cashBank.withdrawalForm.ownerControlTitle')}
                    </AlertTitle>
                    <AlertDescription fontSize="sm">
                      {t('cashBank.withdrawalForm.ownerControlDescription')}
                    </AlertDescription>
                    {!canPostOwnerDrawing && (
                      <Text fontSize="xs" mt={2} fontWeight="semibold">
                        {t('cashBank.withdrawalForm.ownerControlRoleDenied')}
                      </Text>
                    )}
                  </Box>
                </Alert>

                <FormControl isRequired>
                  <FormLabel>{t('cashBank.withdrawalForm.targets.ownerDrawing.label')}</FormLabel>
                  <Select
                    value={formData.target_account_id}
                    onChange={(e) => handleInputChange('target_account_id', e.target.value)}
                    placeholder={t('cashBank.withdrawalForm.targets.ownerDrawing.placeholder')}
                    isDisabled={loadingAccounts || targetAccountsLoadError}
                    {...fieldStyles}
                  >
                    {targetAccounts.map(entry => (
                      <option key={entry.id} value={entry.id}>
                        {entry.code} - {entry.name}
                        {typeof entry.balance === 'number'
                          && entry.balance !== 0
                          && ` (${t('cashBank.withdrawalForm.targetBalance', {
                            amount: entry.balance.toLocaleString(numberLocale),
                          })})`}
                      </option>
                    ))}
                  </Select>
                  <Text fontSize="xs" color={helperTextColor} mt={1}>
                    {t('cashBank.withdrawalForm.targets.ownerDrawing.help')}
                  </Text>
                  {loadingAccounts && (
                    <Text fontSize="xs" color={helperTextColor} mt={1}>
                      {t('cashBank.withdrawalForm.targets.loading')}
                    </Text>
                  )}
                </FormControl>

                {!loadingAccounts && targetAccountsLoadError && (
                  <Alert status="error" borderRadius="md" alignItems="center">
                    <AlertIcon />
                    <AlertDescription fontSize="sm" flex="1">
                      {t('cashBank.withdrawalForm.targets.ownerDrawing.loadError')}
                    </AlertDescription>
                    <Button size="xs" variant="outline" colorScheme="red" onClick={retryLoadOwnerDrawingAccounts}>
                      {t('cashBank.withdrawalForm.targets.ownerDrawing.retry')}
                    </Button>
                  </Alert>
                )}

                {!loadingAccounts && !targetAccountsLoadError && targetAccounts.length === 0 && (
                  <Alert status="warning" borderRadius="md">
                    <AlertIcon />
                    <AlertDescription fontSize="sm">
                      {t('cashBank.withdrawalForm.targets.ownerDrawing.empty')}
                    </AlertDescription>
                  </Alert>
                )}
              </VStack>
            </Box>

            {formData.amount > 0 && (
              <Box>
                <Text fontSize="md" fontWeight="semibold" color={sectionTitleColor} mb={3}>
                  {t('cashBank.withdrawalForm.journalPreview.title')}
                </Text>
                <Box bg={previewBg} p={4} borderRadius="md" border="1px solid" borderColor={previewBorderColor}>
                  <VStack spacing={3} align="stretch">
                    <HStack justify="space-between">
                      <Text fontSize="sm" fontWeight="bold" color="orange.600">
                        {t('cashBank.withdrawalForm.journalPreview.entry')}
                      </Text>
                      <Badge colorScheme="orange" variant="solid" fontSize="xs">
                        {t('cashBank.withdrawalForm.journalPreview.balanced')}
                      </Badge>
                    </HStack>

                    <Divider />

                    <HStack justify="space-between" align="center">
                      <HStack spacing={2}>
                        <Badge colorScheme="green" variant="outline" fontSize="xs">DR</Badge>
                        <Box>
                          <Text fontSize="sm" fontWeight="medium" color={bodyTextColor}>
                            {selectedTargetAccount?.name || t('cashBank.withdrawalForm.journalPreview.counterAccount')}
                          </Text>
                          <Text fontSize="xs" color={mutedTextColor} fontFamily="mono">
                            {selectedTargetAccount?.code || '-'}
                          </Text>
                          <Text fontSize="xs" color={helperTextColor}>
                            {t('cashBank.withdrawalForm.purposes.ownerDrawing.posting')}
                          </Text>
                        </Box>
                      </HStack>
                      <Text fontSize="sm" fontWeight="bold" fontFamily="mono" color="green.600">
                        +{formatIDR(bookImpactIDR)}
                      </Text>
                    </HStack>

                    <HStack justify="space-between" align="center">
                      <HStack spacing={2}>
                        <Badge colorScheme="orange" variant="outline" fontSize="xs">CR</Badge>
                        <Box>
                          <Text fontSize="sm" fontWeight="medium" color={bodyTextColor}>{account.name}</Text>
                          <Text fontSize="xs" color={mutedTextColor} fontFamily="mono">{account.code}</Text>
                          <Text fontSize="xs" color={helperTextColor}>
                            {t('cashBank.withdrawalForm.journalPreview.balanceDecrease')}
                          </Text>
                        </Box>
                      </HStack>
                      <Text fontSize="sm" fontWeight="bold" fontFamily="mono" color="orange.600">
                        -{formatNative(currencyCode, formData.amount)}
                      </Text>
                    </HStack>

                    <Divider />

                    <HStack justify="space-between">
                      <Text fontSize="sm" fontWeight="bold" color={bodyTextColor}>
                        {t('cashBank.withdrawalForm.journalPreview.totalBalance')}
                      </Text>
                      <Text fontSize="sm" fontWeight="bold" color="green.600">
                        DR {formatIDR(bookImpactIDR)} = CR {formatIDR(bookImpactIDR)}
                      </Text>
                    </HStack>
                  </VStack>
                </Box>
              </Box>
            )}

            {formData.amount > 0 && (
              <Box>
                <Text fontSize="md" fontWeight="semibold" color={sectionTitleColor} mb={3}>
                  {t('cashBank.balancePreview')}
                </Text>
                <Alert status={newBalance < 0 ? 'warning' : 'info'} borderRadius="md">
                  <AlertIcon />
                  <Box w="full">
                    <SimpleGrid columns={{ base: 1, md: 3 }} spacing={3}>
                      <Box>
                        <Text fontSize="xs" color={mutedTextColor}>
                          {t('cashBank.withdrawalForm.balancePreview.nativeAmountOut')}
                        </Text>
                        <Text fontSize="sm" fontWeight="bold" color="orange.600" fontFamily="mono">
                          -{formatNative(currencyCode, formData.amount)}
                        </Text>
                      </Box>
                      <Box>
                        <Text fontSize="xs" color={mutedTextColor}>
                          {t('cashBank.withdrawalForm.balancePreview.bookAmountReleased')}
                        </Text>
                        <Text fontSize="sm" fontWeight="bold" color="blue.600" fontFamily="mono">
                          {formatIDR(bookImpactIDR)}
                        </Text>
                      </Box>
                      <Box>
                        <Text fontSize="xs" color={mutedTextColor}>
                          {t('cashBank.withdrawalForm.balancePreview.endingNativeBalance')}
                        </Text>
                        <Text fontSize="sm" fontWeight="bold" color={newBalance < 0 ? 'red.500' : 'green.600'} fontFamily="mono">
                          {formatNative(currencyCode, newBalance)}
                        </Text>
                      </Box>
                    </SimpleGrid>
                    <Text fontSize="sm" mt={3} color={bodyTextColor}>
                      {t('cashBank.withdrawalForm.balancePreview.endingBookBalance', {
                        amount: formatIDR(newBookBalanceIDR),
                      })}
                    </Text>
                    {isForeignCurrencyAccount && (
                      <Text fontSize="xs" color={mutedTextColor} mt={1}>
                        {t('cashBank.withdrawalForm.balancePreview.carryingRate', {
                          currency: currencyCode,
                          rate: currentBookRate.toLocaleString(numberLocale, {
                            minimumFractionDigits: 2,
                            maximumFractionDigits: 4,
                          }),
                        })}
                      </Text>
                    )}
                  </Box>
                </Alert>
              </Box>
            )}
          </VStack>
        </ModalBody>

        <ModalFooter bg={modalFooterBg} borderTop="1px solid" borderColor={borderColor}>
          <Button variant="ghost" mr={3} onClick={handleClose} isDisabled={loading}>
            {t('common.cancel')}
          </Button>
          <Button
            colorScheme="orange"
            onClick={handleSubmit}
            isLoading={loading}
            loadingText={t('cashBank.withdrawalForm.processing')}
            isDisabled={
              formData.amount <= 0 ||
              formData.amount > availableBalance ||
              !formData.target_account_id ||
              !canPostOwnerDrawing ||
              !formData.reference.trim() ||
              !formData.notes.trim()
            }
          >
            {t('cashBank.withdrawalForm.submitOwnerDrawing')}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
};

export default DepositWithdrawalForm;
