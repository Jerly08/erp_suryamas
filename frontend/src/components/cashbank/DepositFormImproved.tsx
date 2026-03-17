'use client';

import React, { useEffect, useState } from 'react';
import {
  Alert,
  AlertIcon,
  AlertDescription,
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
import { useAuth } from '@/contexts/AuthContext';
import cashbankService, { CashBank, DepositRequest, getBookBalanceInIDR, ReferenceExchangeRate } from '@/services/cashbankService';
import { ErrorHandler } from '@/utils/errorHandler';
import { roundUsdIdrRate } from '@/utils/exchangeRate';

interface Account {
  id: number;
  code: string;
  name: string;
  type: string;
  category: string;
  is_active: boolean;
  balance: number;
}

interface DepositSourceAccounts {
  revenue: Account[];
  equity: Account[];
}

interface DepositFormImprovedProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  account: CashBank | null;
}

interface ApiErrorShape {
  response?: {
    status?: number;
    data?: {
      details?: string;
      message?: string;
    };
  };
  status?: number;
  message?: string;
}

const getCurrencyCode = (account?: CashBank | null): string => (account?.currency || 'IDR').toUpperCase();

const formatNativeAmount = (currency: string, amount: number): string => `${currency} ${Math.abs(amount || 0).toLocaleString('id-ID', {
  minimumFractionDigits: currency === 'USD' ? 2 : 0,
  maximumFractionDigits: currency === 'USD' ? 2 : 0,
})}`;

const formatIDRAmount = (amount: number): string => `IDR ${(amount || 0).toLocaleString('id-ID', {
  minimumFractionDigits: 0,
  maximumFractionDigits: 2,
})}`;

const formatReferenceTimestamp = (value?: string): string => {
  if (!value) {
    return '-';
  }

  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }

  return parsed.toLocaleString('id-ID');
};

const getDepositRateSourceMeta = (options: {
  hasReferenceRate: boolean;
  hasLastRate: boolean;
  isManualOverride: boolean;
}) => {
  if (options.isManualOverride) {
    return {
      label: 'MANUAL',
      colorScheme: 'orange',
      description: 'Kurs aktif sudah Anda ubah manual dan angka ini yang akan diposting.',
    };
  }

  if (options.hasReferenceRate) {
    return {
      label: 'API',
      colorScheme: 'blue',
      description: 'Kurs aktif mengikuti referensi API untuk membantu input awal user.',
    };
  }

  if (options.hasLastRate) {
    return {
      label: 'LAST RATE',
      colorScheme: 'purple',
      description: 'Kurs aktif memakai kurs terakhir yang tersimpan pada rekening ini.',
    };
  }

  return {
    label: 'MANUAL',
    colorScheme: 'gray',
    description: 'Kurs aktif menunggu input manual karena belum ada referensi lain.',
  };
};

const DepositFormImproved: React.FC<DepositFormImprovedProps> = ({
  isOpen,
  onClose,
  onSuccess,
  account,
}) => {
  const { token } = useAuth();
  const [formData, setFormData] = useState({
    date: new Date().toISOString().split('T')[0],
    amount: 0,
    exchange_rate: 1,
    reference: '',
    notes: '',
    source_account_id: '',
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sourceAccounts, setSourceAccounts] = useState<DepositSourceAccounts>({ revenue: [], equity: [] });
  const [loadingAccounts, setLoadingAccounts] = useState(false);
  const [referenceRateInfo, setReferenceRateInfo] = useState<ReferenceExchangeRate | null>(null);
  const [referenceRateError, setReferenceRateError] = useState<string | null>(null);
  const [loadingReferenceRate, setLoadingReferenceRate] = useState(false);

  const toast = useToast();
  const currencyCode = getCurrencyCode(account);
  const isUsdAccount = currencyCode === 'USD';
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
  const previewBg = useColorModeValue('blue.50', 'rgba(66, 153, 225, 0.12)');
  const previewBorderColor = useColorModeValue('blue.200', 'blue.700');
  const fieldBg = useColorModeValue('white', 'whiteAlpha.50');
  const fieldBorderColor = useColorModeValue('gray.300', 'whiteAlpha.300');
  const fieldHoverBorderColor = useColorModeValue('gray.400', 'whiteAlpha.400');
  const fieldFocusBorderColor = useColorModeValue('blue.500', 'blue.300');
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
    if (isOpen && token) {
      void loadDepositSourceAccounts();
    }
  }, [isOpen, token]);

  useEffect(() => {
    if (!isOpen || !account) {
      return;
    }

    setFormData(prev => ({
      ...prev,
      exchange_rate: currencyCode === 'IDR'
        ? 1
        : roundUsdIdrRate(account.last_exchange_rate, 1),
    }));
  }, [isOpen, account, currencyCode]);

  useEffect(() => {
    if (!isOpen || !account || !isUsdAccount) {
      setReferenceRateInfo(null);
      setReferenceRateError(null);
      setLoadingReferenceRate(false);
      return;
    }

    let cancelled = false;

    const loadReferenceRate = async () => {
      try {
        setLoadingReferenceRate(true);
        setReferenceRateError(null);
        const reference = await cashbankService.getReferenceExchangeRate('USD', 'IDR');
        if (cancelled) {
          return;
        }
        if (!reference) {
          setReferenceRateInfo(null);
          setReferenceRateError('Kurs referensi API belum tersedia. Form memakai kurs terakhir rekening atau input manual.');
          return;
        }

        setReferenceRateInfo(reference);
        setFormData(prev => ({
          ...prev,
          exchange_rate: roundUsdIdrRate(reference.exchange_rate, 1),
        }));
      } catch (fetchError) {
        if (cancelled) {
          return;
        }

        console.error('Error loading USD/IDR reference rate for deposit:', fetchError);
        setReferenceRateInfo(null);
        setReferenceRateError('Kurs referensi API belum tersedia. Form memakai kurs terakhir rekening atau input manual.');
      } finally {
        if (!cancelled) {
          setLoadingReferenceRate(false);
        }
      }
    };

    void loadReferenceRate();

    return () => {
      cancelled = true;
    };
  }, [isOpen, account, isUsdAccount]);

  const loadDepositSourceAccounts = async () => {
    try {
      setLoadingAccounts(true);
      const accounts = await cashbankService.getDepositSourceAccounts();
      setSourceAccounts(accounts);
    } catch (loadError) {
      console.error('Error loading deposit source accounts:', loadError);
      setSourceAccounts({
        revenue: [],
        equity: [{ id: 0, code: '3101', name: 'Modal Pemilik (Default)', type: 'EQUITY', category: 'OWNER_EQUITY', is_active: true, balance: 0 }],
      });
    } finally {
      setLoadingAccounts(false);
    }
  };

  const handleInputChange = (field: string, value: string | number) => {
    setFormData(prev => ({
      ...prev,
      [field]: value,
    }));
    setError(null);
  };

  const handleDepositError = (requestError: unknown) => {
    const normalizedError = requestError as ApiErrorShape;
    const status = normalizedError?.response?.status || normalizedError?.status;

    if (status === 403) {
      const permissionMessage = (
        <Box>
          <Text fontSize="sm" fontWeight="medium" mb={2}>
            You do not have permission to make deposits.
          </Text>
          <Text fontSize="sm" color="gray.600" mb={3}>
            This action requires Cash & Bank access permissions.
          </Text>
          <Text fontSize="sm" color="gray.700">
            Contact your administrator or manager to request access.
          </Text>
        </Box>
      );

      setError('Access denied - insufficient permissions');
      toast({
        title: 'Access Denied',
        description: permissionMessage,
        status: 'error',
        duration: 8000,
        isClosable: true,
      });
      return;
    }

    if (status === 400) {
      const validationMessage = normalizedError?.response?.data?.details || normalizedError?.response?.data?.message || 'Invalid data';
      setError(`Validation error: ${validationMessage}`);
      toast({
        title: 'Invalid Request',
        description: validationMessage,
        status: 'warning',
        duration: 8000,
        isClosable: true,
      });
      return;
    }

    const errorMessage = ErrorHandler.handleAPIError(normalizedError, toast, {
      operation: 'process deposit',
      fallbackMessage: 'Failed to process deposit. Please try again.',
      duration: 6000,
    });

    setError(errorMessage);
  };

  const handleSubmit = async () => {
    try {
      setLoading(true);
      setError(null);

      if (!account) {
        throw new Error('No account selected');
      }

      if (formData.amount <= 0) {
        throw new Error('Amount must be greater than zero');
      }

      if (isUsdAccount && formData.exchange_rate <= 0) {
        throw new Error('Exchange rate is required for USD deposits');
      }

      const requestData: DepositRequest = {
        account_id: Number(account.id),
        date: formData.date,
        amount: Number(formData.amount),
        exchange_rate: isUsdAccount ? roundUsdIdrRate(formData.exchange_rate, 1) : 1,
        reference: formData.reference || '',
        notes: formData.notes || '',
      };

      if (formData.source_account_id && parseInt(formData.source_account_id, 10) > 0) {
        requestData.source_account_id = parseInt(formData.source_account_id, 10);
      }

      const dateRegex = /^\d{4}-\d{2}-\d{2}$/;
      if (!dateRegex.test(requestData.date)) {
        throw new Error('Invalid date format. Please use YYYY-MM-DD format.');
      }

      await cashbankService.processDeposit(requestData);

      const selectedSource = sourceAccounts.equity.find(entry => entry.id === parseInt(formData.source_account_id || '0', 10));
      const sourceAccountName = selectedSource ? selectedSource.name : 'Modal Disetor (Default)';
      const depositBookAmountIDR = isUsdAccount ? formData.amount * formData.exchange_rate : formData.amount;

      toast({
        title: 'Deposit Successful',
        description: (
          <Box>
            <Text fontSize="sm" fontWeight="bold">
              {formatNativeAmount(currencyCode, formData.amount)} deposited to {account.name}
            </Text>
            <Text fontSize="xs" color={mutedTextColor} mt={1}>
              Debit: {account.name} (+{formatNativeAmount(currencyCode, formData.amount)})
            </Text>
            <Text fontSize="xs" color={mutedTextColor}>
              Credit: {sourceAccountName} ({formatIDRAmount(depositBookAmountIDR)})
            </Text>
            <Text fontSize="xs" color="green.200" mt={1} fontWeight="bold">
              Double-entry balanced automatically.
            </Text>
          </Box>
        ),
        status: 'success',
        duration: 5000,
        isClosable: true,
      });

      onSuccess();
      onClose();
    } catch (submitError: unknown) {
      console.error('Error processing deposit:', submitError);
      handleDepositError(submitError);
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    setError(null);
    setFormData({
      date: new Date().toISOString().split('T')[0],
      amount: 0,
      exchange_rate: isUsdAccount ? roundUsdIdrRate(account?.last_exchange_rate, 1) : 1,
      reference: '',
      notes: '',
      source_account_id: '',
    });
    onClose();
  };

  if (!account) {
    return null;
  }

  const newBalance = account.balance + formData.amount;
  const currentBookBalanceIDR = getBookBalanceInIDR(account);
  const depositBookAmountIDR = isUsdAccount ? formData.amount * formData.exchange_rate : formData.amount;
  const newBookBalanceIDR = currentBookBalanceIDR + depositBookAmountIDR;
  const selectedAccount = sourceAccounts.equity.find(entry => entry.id === parseInt(formData.source_account_id || '0', 10));
  const sourceAccountName = selectedAccount ? selectedAccount.name : 'Modal Disetor (Default)';
  const roundedReferenceRate = roundUsdIdrRate(referenceRateInfo?.exchange_rate, 0);
  const exchangeRateOverridden = Boolean(referenceRateInfo && Math.abs((formData.exchange_rate || 0) - roundedReferenceRate) > 0.0001);
  const depositRateSource = getDepositRateSourceMeta({
    hasReferenceRate: Boolean(referenceRateInfo),
    hasLastRate: Boolean(account.last_exchange_rate && account.last_exchange_rate > 0),
    isManualOverride: exchangeRateOverridden,
  });

  return (
    <Modal isOpen={isOpen} onClose={handleClose} size="2xl" scrollBehavior="inside">
      <ModalOverlay bg="blackAlpha.600" />
      <ModalContent bg={modalBg} border="1px solid" borderColor={borderColor}>
        <ModalHeader bg={modalHeaderBg} borderBottom="1px solid" borderColor={borderColor}>
          <Flex alignItems="center" gap={3}>
            <Text fontSize="lg">Deposit</Text>
            <Box>
              <Text fontSize="lg" fontWeight="bold" color={sectionTitleColor}>
                Make Deposit
              </Text>
              <Text fontSize="sm" color={helperTextColor} fontFamily="mono">
                {account.name} ({account.code})
              </Text>
            </Box>
            <Badge colorScheme="green" variant="solid" fontSize="xs">
              AUTOMATIC MODE
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
                Account Information
              </Text>
              <Box bg={panelBg} p={4} borderRadius="md" border="1px solid" borderColor={panelBorderColor}>
                <Flex justify="space-between" align="center" mb={2}>
                  <Box>
                    <Text fontSize="sm" color={mutedTextColor} mb={1}>Account Name</Text>
                    <Text fontWeight="medium" color={bodyTextColor}>{account.name}</Text>
                  </Box>
                  <Badge colorScheme={account.type === 'CASH' ? 'green' : 'blue'}>
                    {account.type}
                  </Badge>
                </Flex>

                <Stat>
                  <StatLabel color={mutedTextColor}>Current Native Balance</StatLabel>
                  <StatNumber color={account.balance < 0 ? 'red.500' : 'green.600'} fontFamily="mono">
                    {formatNativeAmount(currencyCode, account.balance)}
                    {account.balance < 0 && ' (Dr)'}
                  </StatNumber>
                  <StatHelpText color={helperTextColor}>
                    {account.balance < 0 ? 'Overdraft' : account.balance > 0 ? 'Available' : 'Zero Balance'}
                  </StatHelpText>
                  <StatHelpText color={helperTextColor}>
                    Book balance for ledger: {formatIDRAmount(currentBookBalanceIDR)}
                  </StatHelpText>
                </Stat>
              </Box>
            </Box>

            <Alert status={isUsdAccount ? 'info' : 'success'} borderRadius="md" alignItems="flex-start">
              <AlertIcon mt={1} />
              <Box>
                <Flex gap={2} mb={2} wrap="wrap">
                  <Badge colorScheme="blue" variant="subtle">NATIVE {currencyCode}</Badge>
                  <Badge colorScheme="green" variant="subtle">BOOK IDR</Badge>
                </Flex>
                <Text fontSize="sm" fontWeight="medium">
                  {isUsdAccount
                    ? 'Akun USD menyimpan 2 angka: saldo rekening dalam USD dan nilai buku neraca dalam IDR.'
                    : 'Untuk akun IDR, saldo rekening dan nilai buku neraca selalu sama.'}
                </Text>
                <Text fontSize="xs" color={mutedTextColor} mt={1}>
                  {isUsdAccount
                    ? 'Kurs transaksi di bawah akan menentukan berapa nilai buku IDR yang diposting ke ledger.'
                    : 'Deposit IDR akan menambah saldo native dan nilai buku dengan angka yang sama.'}
                </Text>
              </Box>
            </Alert>

            <Box>
              <Text fontSize="md" fontWeight="semibold" color={sectionTitleColor} mb={3}>
                Transaction Details
              </Text>

              <VStack spacing={4} align="stretch">
                <FormControl isRequired>
                  <FormLabel>Transaction Date</FormLabel>
                  <Input
                    type="date"
                    value={formData.date}
                    onChange={(e) => handleInputChange('date', e.target.value)}
                    {...fieldStyles}
                  />
                </FormControl>

                <FormControl isRequired>
                  <FormLabel>Amount ({currencyCode})</FormLabel>
                  <NumberInput
                    value={formData.amount}
                    onChange={(_, value) => handleInputChange('amount', value || 0)}
                    min={0}
                    precision={isUsdAccount ? 2 : 0}
                  >
                    <NumberInputField
                      placeholder={isUsdAccount ? 'USD 0.00' : 'IDR 0'}
                      fontFamily="mono"
                      {...fieldStyles}
                    />
                    <NumberInputStepper>
                      <NumberIncrementStepper />
                      <NumberDecrementStepper />
                    </NumberInputStepper>
                  </NumberInput>
                  <Text fontSize="xs" color={helperTextColor} mt={1}>
                    Ini adalah nominal yang masuk ke rekening. Untuk akun USD, angka ini tetap dalam USD.
                  </Text>
                </FormControl>

                {isUsdAccount && (
                  <FormControl isRequired>
                    <FormLabel>Exchange Rate (IDR per USD)</FormLabel>
                    <NumberInput
                      value={formData.exchange_rate}
                      onChange={(_, value) => handleInputChange('exchange_rate', roundUsdIdrRate(value, 0))}
                      min={0}
                      precision={0}
                      step={1}
                    >
                      <NumberInputField
                        placeholder="Contoh: 16935"
                        fontFamily="mono"
                        {...fieldStyles}
                      />
                      <NumberInputStepper>
                        <NumberIncrementStepper />
                        <NumberDecrementStepper />
                      </NumberInputStepper>
                    </NumberInput>
                    <Text fontSize="xs" color={helperTextColor} mt={1}>
                      Masukkan berapa Rupiah untuk 1 USD. Nilai ini dipakai untuk menghitung posting book value ke ledger.
                    </Text>
                    <Text fontSize="xs" color={helperTextColor} mt={1}>
                      Contoh: jika setor `USD 100` dengan kurs `16,250`, maka nilai buku yang masuk adalah `IDR 1,625,000`.
                    </Text>
                    {referenceRateInfo && (
                      <Alert status="info" borderRadius="md" mt={3}>
                        <AlertIcon />
                        <Box>
                          <AlertTitle fontSize="sm" mb={1}>
                            Kurs referensi API
                          </AlertTitle>
                          <AlertDescription fontSize="sm">
                            Disarankan dari {referenceRateInfo.provider}: 1 USD = {formatIDRAmount(roundUsdIdrRate(referenceRateInfo.exchange_rate, 0))}.
                            Diambil {formatReferenceTimestamp(referenceRateInfo.fetched_at)} dan tetap bisa diubah sebelum posting.
                          </AlertDescription>
                        </Box>
                      </Alert>
                    )}
                    {loadingReferenceRate && (
                      <Text fontSize="xs" color={helperTextColor} mt={2}>
                        Mengambil kurs referensi USD/IDR dari API...
                      </Text>
                    )}
                    {referenceRateError && (
                      <Alert status="warning" borderRadius="md" mt={3}>
                        <AlertIcon />
                        <AlertDescription fontSize="sm">
                          {referenceRateError}
                        </AlertDescription>
                      </Alert>
                    )}
                    {exchangeRateOverridden && (
                      <Text fontSize="xs" color="orange.500" mt={2}>
                        Anda sedang memakai kurs manual yang berbeda dari kurs referensi API.
                      </Text>
                    )}
                    <HStack spacing={2} mt={2} align="center">
                      <Badge colorScheme={depositRateSource.colorScheme} variant="subtle">
                        {depositRateSource.label}
                      </Badge>
                      <Text fontSize="xs" color={helperTextColor}>
                        {depositRateSource.description}
                      </Text>
                    </HStack>
                  </FormControl>
                )}

                <FormControl>
                  <FormLabel>Reference</FormLabel>
                  <Input
                    value={formData.reference}
                    onChange={(e) => handleInputChange('reference', e.target.value)}
                    placeholder="e.g., receipt number or bank slip"
                    {...fieldStyles}
                  />
                </FormControl>

                <FormControl>
                  <FormLabel>Notes</FormLabel>
                  <Textarea
                    value={formData.notes}
                    onChange={(e) => handleInputChange('notes', e.target.value)}
                    placeholder="Optional transaction notes"
                    rows={3}
                    {...fieldStyles}
                  />
                </FormControl>
              </VStack>
            </Box>

            <Box>
              <Text fontSize="md" fontWeight="semibold" color={sectionTitleColor} mb={3}>
                Capital / Equity Account Selection
              </Text>
              <Text fontSize="sm" color={mutedTextColor} mb={3}>
                Select the equity account to be credited. Leave blank to use the default Modal Disetor account.
              </Text>

              <FormControl>
                <FormLabel>Credit Account (Equity Source)</FormLabel>
                <Select
                  value={formData.source_account_id}
                  onChange={(e) => handleInputChange('source_account_id', e.target.value)}
                  placeholder="Use default Modal Disetor account"
                  isDisabled={loadingAccounts}
                  {...fieldStyles}
                >
                  {sourceAccounts.equity.map(entry => (
                    <option key={`equity-${entry.id}`} value={entry.id}>
                      {entry.code} - {entry.name}
                      {entry.balance !== 0 && ` (Balance: ${entry.balance.toLocaleString('id-ID')})`}
                    </option>
                  ))}
                </Select>
                {loadingAccounts && (
                  <Text fontSize="xs" color={helperTextColor} mt={1}>
                    Loading equity accounts...
                  </Text>
                )}
              </FormControl>
            </Box>

            {formData.amount > 0 && (
              <Box>
                <Text fontSize="md" fontWeight="semibold" color={sectionTitleColor} mb={3}>
                  Double-Entry Preview
                </Text>
                <Box bg={previewBg} p={4} borderRadius="md" border="1px solid" borderColor={previewBorderColor}>
                  <VStack spacing={3} align="stretch">
                    <HStack justify="space-between">
                      <Text fontSize="sm" fontWeight="bold" color="blue.600">
                        Journal Entry (Auto-Generated)
                      </Text>
                      <Badge colorScheme="blue" variant="solid" fontSize="xs">
                        BALANCED
                      </Badge>
                    </HStack>

                    <Divider />

                    <HStack justify="space-between" align="center">
                      <HStack spacing={2}>
                        <Badge colorScheme="green" variant="outline" fontSize="xs">DR</Badge>
                        <Box>
                          <Text fontSize="sm" fontWeight="medium" color={bodyTextColor}>{account.name}</Text>
                          <Text fontSize="xs" color={mutedTextColor} fontFamily="mono">{account.code}</Text>
                          <Text fontSize="xs" color={helperTextColor}>Saldo rekening bertambah</Text>
                        </Box>
                      </HStack>
                      <Text fontSize="sm" fontWeight="bold" fontFamily="mono" color="green.600">
                        +{formatNativeAmount(currencyCode, formData.amount)}
                      </Text>
                    </HStack>

                    <HStack justify="space-between" align="center">
                      <HStack spacing={2}>
                        <Badge colorScheme="orange" variant="outline" fontSize="xs">CR</Badge>
                        <Box>
                          <Text fontSize="sm" fontWeight="medium" color={bodyTextColor}>{sourceAccountName}</Text>
                          <Text fontSize="xs" color={mutedTextColor} fontFamily="mono">
                            {selectedAccount?.code || '3101'}
                          </Text>
                          <Text fontSize="xs" color={helperTextColor}>Posting lawan dalam nilai buku IDR</Text>
                        </Box>
                      </HStack>
                      <Text fontSize="sm" fontWeight="bold" fontFamily="mono" color="orange.600">
                        +{formatIDRAmount(depositBookAmountIDR)}
                      </Text>
                    </HStack>

                    <Divider />

                    <HStack justify="space-between">
                      <Text fontSize="sm" fontWeight="bold" color={bodyTextColor}>Total Balance</Text>
                      <Text fontSize="sm" fontWeight="bold" color="green.600">
                        DR {formatIDRAmount(depositBookAmountIDR)} = CR {formatIDRAmount(depositBookAmountIDR)}
                      </Text>
                    </HStack>
                  </VStack>
                </Box>
              </Box>
            )}

            {formData.amount > 0 && (
              <Box>
                <Text fontSize="md" fontWeight="semibold" color={sectionTitleColor} mb={3}>
                  Deposit Preview
                </Text>
                <Alert status={isUsdAccount ? 'info' : 'success'} borderRadius="md">
                  <AlertIcon />
                  <Box w="full">
                    <SimpleGrid columns={{ base: 1, md: 3 }} spacing={3}>
                      <Box>
                        <Text fontSize="xs" color={mutedTextColor}>Native amount in</Text>
                        <Text fontSize="sm" fontWeight="bold" color="green.600" fontFamily="mono">
                          +{formatNativeAmount(currencyCode, formData.amount)}
                        </Text>
                      </Box>
                      <Box>
                        <Text fontSize="xs" color={mutedTextColor}>Book amount posted</Text>
                        <Text fontSize="sm" fontWeight="bold" color="blue.600" fontFamily="mono">
                          {formatIDRAmount(depositBookAmountIDR)}
                        </Text>
                      </Box>
                      <Box>
                        <Text fontSize="xs" color={mutedTextColor}>Ending native balance</Text>
                        <Text fontSize="sm" fontWeight="bold" color="green.600" fontFamily="mono">
                          {formatNativeAmount(currencyCode, newBalance)}
                        </Text>
                      </Box>
                    </SimpleGrid>
                    <Text fontSize="sm" mt={3} color={bodyTextColor}>
                      Ending book balance: {formatIDRAmount(newBookBalanceIDR)}
                    </Text>
                    {isUsdAccount && (
                      <Text fontSize="xs" color={mutedTextColor} mt={1}>
                        Perhitungan: {formatNativeAmount('USD', formData.amount)} x {formData.exchange_rate.toLocaleString('id-ID', {
                          minimumFractionDigits: 2,
                          maximumFractionDigits: 4,
                        })} = {formatIDRAmount(depositBookAmountIDR)}
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
            Cancel
          </Button>
          <Button
            colorScheme="green"
            onClick={handleSubmit}
            isLoading={loading}
            loadingText="Processing deposit..."
            isDisabled={formData.amount <= 0 || (isUsdAccount && formData.exchange_rate <= 0)}
          >
            Process Deposit
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
};

export default DepositFormImproved;
