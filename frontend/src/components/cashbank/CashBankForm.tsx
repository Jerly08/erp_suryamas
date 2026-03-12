'use client';

import React, { useState, useEffect } from 'react';
import { useTranslation } from '@/hooks/useTranslation';
import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  ModalCloseButton,
  Button,
  FormControl,
  FormLabel,
  FormErrorMessage,
  Input,
  InputGroup,
  InputLeftAddon,
  Select,
  Textarea,
  RadioGroup,
  Radio,
  HStack,
  NumberInput,
  NumberInputField,
  NumberInputStepper,
  NumberIncrementStepper,
  NumberDecrementStepper,
  Alert,
  AlertIcon,
  AlertDescription,
  AlertTitle,
  useToast,
  Box,
  Checkbox,
  Text,
  VStack,
  Divider,
  Card,
  CardBody,
  SimpleGrid,

  Badge,
  useColorModeValue,
} from '@chakra-ui/react';
import { useForm, Controller } from 'react-hook-form';
import { FiUser, FiDollarSign, FiCalendar, FiCreditCard, FiFileText, FiSettings } from 'react-icons/fi';
import { CashBank, CashBankCreateRequest, CashBankUpdateRequest } from '@/services/cashbankService';
import cashbankService from '@/services/cashbankService';
import accountService from '@/services/accountService';
import { useAuth } from '@/contexts/AuthContext';
import { Account } from '@/types/account';

interface CashBankFormProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  account?: CashBank | null;
  mode: 'create' | 'edit';
}

interface AccountOption {
  id: number;
  code: string;
  name: string;
  balance: number;
}

interface CashBankFormData {
  name: string;
  type: 'CASH' | 'BANK';
  bank_name?: string;
  account_no?: string;
  account_holder_name?: string;
  branch?: string;
  currency: string;
  opening_balance?: number;
  opening_exchange_rate?: number;
  opening_date?: string;
  description?: string;
  account_id?: number;
  source_account_id?: number;
  is_holding_account?: boolean;
  allow_direct_transactions?: boolean;
}

const normalizeCurrencyCode = (value?: string): 'IDR' | 'USD' => {
  return (value || 'IDR').toUpperCase() === 'USD' ? 'USD' : 'IDR';
};

const parseOpeningBalanceInput = (value: string, currency: 'IDR' | 'USD'): number | undefined => {
  if (!value.trim()) {
    return undefined;
  }

  if (currency === 'IDR') {
    const digitsOnly = value.replace(/\D/g, '');
    return digitsOnly ? parseInt(digitsOnly, 10) : undefined;
  }

  const sanitized = value.replace(/[^0-9.,]/g, '');
  if (!sanitized) {
    return undefined;
  }

  const lastComma = sanitized.lastIndexOf(',');
  const lastDot = sanitized.lastIndexOf('.');
  const decimalIndex = Math.max(lastComma, lastDot);

  if (decimalIndex < 0) {
    const integerOnly = sanitized.replace(/[.,]/g, '');
    return integerOnly ? parseFloat(integerOnly) : undefined;
  }

  const integerPart = sanitized.slice(0, decimalIndex).replace(/[.,]/g, '');
  const decimalPart = sanitized.slice(decimalIndex + 1).replace(/[^\d]/g, '').slice(0, 2);
  const normalized = decimalPart ? `${integerPart || '0'}.${decimalPart}` : (integerPart || '0');
  const parsed = parseFloat(normalized);

  return Number.isFinite(parsed) ? parsed : undefined;
};

const formatOpeningBalanceDisplay = (value: number | undefined, currency: 'IDR' | 'USD'): string => {
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) {
    return '';
  }

  return new Intl.NumberFormat(currency === 'USD' ? 'en-US' : 'id-ID', {
    minimumFractionDigits: currency === 'USD' ? 2 : 0,
    maximumFractionDigits: currency === 'USD' ? 2 : 0,
  }).format(value);
};

const getOpeningBalancePlaceholder = (currency: 'IDR' | 'USD'): string => {
  return currency === 'USD' ? 'Contoh: 1,250.00' : 'Contoh: 1.000.000';
};

const getOpeningBalanceHelpText = (currency: 'IDR' | 'USD'): string => {
  return currency === 'USD'
    ? 'Masukkan saldo awal dalam USD. Jika ada sen, gunakan format seperti 1,250.50.'
    : 'Masukkan saldo awal dalam IDR. Tidak perlu menulis simbol Rp.';
};

const getOpeningBalanceAlertConfig = (currency: 'IDR' | 'USD') => {
  if (currency === 'USD') {
    return {
      status: 'success' as const,
      title: 'Input saldo awal dalam USD',
      description: 'Nominal yang diketik di field ini adalah nilai asli rekening USD. Nilai buku IDR akan dihitung dari kurs saldo awal di bawah.',
      badgeColorScheme: 'green' as const,
    };
  }

  return {
    status: 'info' as const,
    title: 'Input saldo awal dalam IDR',
    description: 'Nominal yang diketik di field ini langsung dalam Rupiah. Untuk rekening IDR, kurs tidak perlu diisi.',
    badgeColorScheme: 'blue' as const,
  };
};

const CashBankForm: React.FC<CashBankFormProps> = ({
  isOpen,
  onClose,
  onSuccess,
  account,
  mode
}) => {
  const { token } = useAuth();
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [equityAccounts, setEquityAccounts] = useState<AccountOption[]>([]);


  const [openingBalanceDisplay, setOpeningBalanceDisplay] = useState<string>('');
  const toast = useToast();

  // Color mode values
  const modalContentBg = useColorModeValue('white', 'gray.800');
  const modalHeaderBg = useColorModeValue('blue.500', 'blue.600');
  const modalFooterBg = useColorModeValue('gray.50', 'gray.700');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const textColor = useColorModeValue('gray.700', 'gray.300');
  const mutedTextColor = useColorModeValue('gray.500', 'gray.400');
  const readOnlyBg = useColorModeValue('gray.50', 'gray.700');
  const inputAddonBg = useColorModeValue('gray.50', 'gray.700');
  const cardInfoBg = useColorModeValue('blue.50', 'blue.900');
  const cardInfoTextColor = useColorModeValue('blue.700', 'blue.200');
  const cardInfoBorderColor = useColorModeValue('blue.200', 'blue.700');
  const cardSuccessBg = useColorModeValue('green.50', 'green.900');
  const cardSuccessTextColor = useColorModeValue('green.700', 'green.200');
  const cardSuccessBorderColor = useColorModeValue('green.200', 'green.700');
  const warningTextColor = useColorModeValue('orange.500', 'orange.300');
  const scrollbarTrackColor = useColorModeValue('#f1f1f1', '#2d3748');
  const scrollbarThumbColor = useColorModeValue('#c1c1c1', '#4a5568');
  const scrollbarThumbHoverColor = useColorModeValue('#a8a8a8', '#718096');

  const {
    control,
    register,
    handleSubmit,
    reset,
    setValue,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<CashBankFormData>({
    defaultValues: {
      name: '',
      type: 'CASH',
      bank_name: '',
      account_no: '',
      account_holder_name: '',
      branch: '',
      currency: 'IDR',
      opening_balance: 0,
      opening_exchange_rate: 1,
      opening_date: new Date().toISOString().split('T')[0],
      description: '',
      account_id: undefined,
      source_account_id: undefined,
      is_holding_account: false,
      allow_direct_transactions: true
    }
  });

  const watchedOpeningBalance = watch('opening_balance');
  const watchedType = watch('type');
  const watchedCurrency = watch('currency');
  const watchedHoldingAccount = watch('is_holding_account');
  const normalizedWatchedCurrency = normalizeCurrencyCode(watchedCurrency);
  const openingBalanceAlert = getOpeningBalanceAlertConfig(normalizedWatchedCurrency);

  // Load available GL accounts based on selected type (CASH or BANK)
  // This endpoint returns only GL accounts that:
  // 1. Are not already linked to any cash/bank account
  // 2. Match the type filter (KAS for CASH, BANK for BANK)
  useEffect(() => {
    const loadAccounts = async () => {
      if (token && isOpen && mode === 'create' && watchedType) {
        try {
          const availableAccounts = await cashbankService.getAvailableGLAccounts(watchedType);
          // Ensure we have an array before mapping
          if (!Array.isArray(availableAccounts)) {
            console.warn('getAvailableGLAccounts did not return an array:', availableAccounts);
            setAccounts([]);
            return;
          }
          // Map to Account type for compatibility
          const mappedAccounts = availableAccounts.map(acc => ({
            id: acc.id,
            code: acc.code,
            name: acc.name,
            balance: acc.balance,
            type: 'ASSET' as const,
            category: acc.category,
            is_header: false,
            is_active: true,
          }));
          setAccounts(mappedAccounts as Account[]);

          // Clear selected account when type changes (to prevent invalid selection)
          setValue('account_id', undefined);
        } catch (error) {
          console.error('Error loading available GL accounts:', error);
          setAccounts([]);
        }
      }
    };

    loadAccounts();
  }, [token, isOpen, mode, watchedType, setValue]);

  // Load Equity Accounts
  useEffect(() => {
    const loadEquityAccounts = async () => {
      if (token && isOpen && mode === 'create') {
        try {
          const result = await accountService.getAccounts(token, 'EQUITY');
          if (Array.isArray(result)) {
            setEquityAccounts(result.map(acc => ({
              id: acc.id,
              code: acc.code,
              name: acc.name,
              balance: acc.balance
            })));
          }
        } catch (error) {
          console.error('Error loading equity accounts:', error);
        }
      }
    };
    loadEquityAccounts();
  }, [token, isOpen, mode]);

  // Initialize form data when editing or opening modal
  useEffect(() => {
    if (!isOpen) {
      return;
    }

    if (mode === 'edit' && account) {
      reset({
        name: account.name || '',
        type: account.type || 'CASH',
        bank_name: account.bank_name || '',
        account_no: account.account_no || '',
        account_holder_name: account.account_holder_name || '',
        branch: account.branch || '',
        currency: (account.currency || 'IDR').toUpperCase(),
        opening_balance: 0,
        opening_exchange_rate: account.last_exchange_rate && account.last_exchange_rate > 0 ? account.last_exchange_rate : 1,
        opening_date: new Date().toISOString().split('T')[0],
        description: account.description || '',
        account_id: account.account_id,
        source_account_id: undefined,
        is_holding_account: Boolean(account.is_holding_account),
        allow_direct_transactions: account.allow_direct_transactions ?? !account.is_holding_account
      });
      setOpeningBalanceDisplay('');
      return;
    }

    reset({
      name: '',
      type: 'CASH',
      bank_name: '',
      account_no: '',
      account_holder_name: '',
      branch: '',
      currency: 'IDR',
      opening_balance: 0,
      opening_exchange_rate: 1,
      opening_date: new Date().toISOString().split('T')[0],
      description: '',
      account_id: undefined,
      source_account_id: undefined,
      is_holding_account: false,
      allow_direct_transactions: true
    });
    setOpeningBalanceDisplay('');
  }, [isOpen, mode, account, reset]);

  const onSubmit = async (data: CashBankFormData) => {
    try {
      setLoading(true);

      // Basic Validation
      if (!data.name.trim()) {
        throw new Error(t('cashBank.form.errorAccountName'));
      }

      if (data.type === 'BANK' && !data.bank_name?.trim()) {
        throw new Error(t('cashBank.form.errorBankName'));
      }

      // COA Integration Validation (only for create mode)
      if (mode === 'create' && !data.account_id) {
        throw new Error(t('cashBank.form.errorGLAccount'));
      }

      // Equity Account Validation
      if (mode === 'create' && (data.opening_balance || 0) > 0 && !data.source_account_id) {
        throw new Error(t('cashBank.form.errorEquityAccount') || 'Harap pilih akun Modal untuk Saldo Awal');
      }

      // Prepare data for submission
      const normalizedCurrency = (data.currency || 'IDR').toUpperCase();
      if (normalizedCurrency !== 'IDR' && (data.opening_balance || 0) > 0 && !(data.opening_exchange_rate && data.opening_exchange_rate > 0)) {
        throw new Error('Kurs saldo awal wajib diisi untuk rekening mata uang asing');
      }

      const submitData: CashBankCreateRequest = {
        ...data,
        currency: normalizedCurrency,
        opening_exchange_rate: normalizedCurrency === 'IDR' ? 1 : data.opening_exchange_rate,
        is_holding_account: Boolean(data.is_holding_account),
        allow_direct_transactions: data.is_holding_account ? Boolean(data.allow_direct_transactions) : true,
      };

      if (mode === 'create') {
        await cashbankService.createCashBankAccount(submitData);
        toast({
          title: t('common.create'),
          description: `${data.type === 'CASH' ? t('cashBank.form.cashAccount') : t('cashBank.form.bankAccount')} ${t('cashBank.form.successCreate')}`,
          status: 'success',
          duration: 3000,
          isClosable: true,
        });
      } else if (mode === 'edit' && account) {
        const updateData: CashBankUpdateRequest = {
          name: data.name,
          bank_name: data.bank_name,
          account_no: data.account_no,
          account_holder_name: data.account_holder_name,
          branch: data.branch,
          description: data.description,
          is_holding_account: Boolean(data.is_holding_account),
          allow_direct_transactions: data.is_holding_account ? Boolean(data.allow_direct_transactions) : true,
        };
        await cashbankService.updateCashBankAccount(account.id, updateData);
        toast({
          title: t('common.update'),
          description: t('cashBank.form.successUpdate'),
          status: 'success',
          duration: 3000,
          isClosable: true,
        });
      }

      onSuccess();
      onClose();
    } catch (err: unknown) {
      const error = err as { response?: { data?: unknown; status?: number }; message?: string };
      // Enhanced error logging for debugging
      console.error('Error saving account:', error);
      console.error('Error response:', error?.response);
      console.error('Error status:', error?.response?.status);
      if (error?.response?.data) {
        console.error('Backend error payload:', error.response.data);
      }

      const resp = error?.response?.data;
      const statusCode = error?.response?.status;

      // Extract error message with fallback handling
      let message: string;

      // Check if response data exists and has content
      if (resp && typeof resp === 'object' && Object.keys(resp).length > 0) {
        // Try to extract the most specific error message
        message = resp?.details || resp?.message || resp?.error || '';

        // If message is still empty, try to stringify the response
        if (!message && resp) {
          message = JSON.stringify(resp);
        }
      } else if (error?.message) {
        // Use error message from the exception
        message = error.message;
      } else {
        // Fallback for empty error payload
        message = '';
      }

      // Special handling for empty error payload or 500 errors
      if (!message || message === '{}' || message === 'null') {
        if (statusCode === 500) {
          message = 'An unexpected server error occurred. Please check the server logs or contact support.';
        } else {
          message = t('cashBank.form.errorSave');
        }
      }

      toast({
        title: statusCode ? `${t('common.error')} (${statusCode})` : t('common.error'),
        description: typeof message === 'string' ? message : JSON.stringify(message),
        status: 'error',
        duration: 7000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    reset();

    onClose();
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={handleClose}
      size="4xl"
      scrollBehavior="inside"
    >
      <ModalOverlay bg="blackAlpha.600" backdropFilter="blur(10px)" />
      <ModalContent
        maxH="95vh"
        maxW={{ base: '95vw', md: '90vw', lg: '70vw' }}
        mx={4}
        bg={modalContentBg}
      >
        <ModalHeader
          bg={modalHeaderBg}
          color="white"
          borderTopRadius="md"
          py={4}
        >
          {mode === 'create' ? t('cashBank.form.createTitle') : t('cashBank.form.editTitle')}
        </ModalHeader>
        <ModalCloseButton color="white" />

        <form onSubmit={handleSubmit(onSubmit)}>
          <ModalBody
            overflowY="auto"
            maxH="70vh"
            css={{
              '&::-webkit-scrollbar': {
                width: '8px',
              },
              '&::-webkit-scrollbar-track': {
                background: scrollbarTrackColor,
                borderRadius: '4px',
              },
              '&::-webkit-scrollbar-thumb': {
                background: scrollbarThumbColor,
                borderRadius: '4px',
                '&:hover': {
                  background: scrollbarThumbHoverColor,
                },
              },
              scrollBehavior: 'smooth',
              willChange: 'scroll-position',
              transform: 'translateZ(0)',
              WebkitOverflowScrolling: 'touch',
            }}
          >
            <VStack spacing={6} align="stretch">
              {/* Account Type Selection */}
              <Card>
                <CardBody>
                  <FormControl isRequired>
                    <FormLabel>
                      <HStack>
                        <FiSettings />
                        <Text>{t('cashBank.form.accountType')}</Text>
                      </HStack>
                    </FormLabel>
                    <Controller
                      name="type"
                      control={control}
                      rules={{ required: t('cashBank.form.accountTypeRequired') }}
                      render={({ field }) => (
                        <RadioGroup
                          {...field}
                          isDisabled={mode === 'edit'}
                        >
                          <HStack spacing={8}>
                            <Radio value="CASH">
                              <HStack>
                                <Text fontSize="2xl">💵</Text>
                                <Box>
                                  <Text>{t('cashBank.form.cashAccount')}</Text>
                                  <Text fontSize="xs" color={mutedTextColor}>{t('cashBank.form.physicalMoneyManagement')}</Text>
                                </Box>
                              </HStack>
                            </Radio>
                            <Radio value="BANK">
                              <HStack>
                                <Text fontSize="2xl">🏦</Text>
                                <Box>
                                  <Text>{t('cashBank.form.bankAccount')}</Text>
                                  <Text fontSize="xs" color={mutedTextColor}>{t('cashBank.form.electronicBanking')}</Text>
                                </Box>
                              </HStack>
                            </Radio>
                          </HStack>
                        </RadioGroup>
                      )}
                    />
                    {mode === 'edit' && (
                      <Text fontSize="xs" color={warningTextColor} mt={2}>
                        ⚠️ {t('cashBank.form.accountTypeCannotChange')}
                      </Text>
                    )}
                  </FormControl>
                </CardBody>
              </Card>

              {/* Basic Account Information */}
              <Card>
                <CardBody>
                  <Text fontWeight="bold" mb={4} color={textColor}>
                    📋 {t('cashBank.form.basicInformation')}
                  </Text>

                  <SimpleGrid columns={{ base: 1, md: 2 }} spacing={4}>
                    <FormControl isRequired isInvalid={!!errors.name}>
                      <FormLabel>
                        <HStack>
                          <FiFileText />
                          <Text>{t('cashBank.form.accountName')}</Text>
                        </HStack>
                      </FormLabel>
                      <Input
                        {...register('name', {
                          required: t('cashBank.form.accountNameRequired'),
                          minLength: { value: 2, message: t('cashBank.form.accountNameMinLength') }
                        })}
                        placeholder={watchedType === 'CASH' ? t('cashBank.form.accountNamePlaceholder') : t('cashBank.form.bankAccountNamePlaceholder')}
                      />
                      <FormErrorMessage>{errors.name?.message}</FormErrorMessage>
                    </FormControl>

                      <FormControl>
                        <FormLabel>
                          <HStack>
                            <FiDollarSign />
                            <Text>{t('cashBank.form.currency')}</Text>
                          </HStack>
                        </FormLabel>
                        <Controller
                          name="currency"
                          control={control}
                          render={({ field }) => (
                            <Select
                              value={normalizeCurrencyCode(field.value)}
                              onChange={(e) => {
                                const nextCurrency = normalizeCurrencyCode(e.target.value);
                                field.onChange(nextCurrency);

                                if ((watchedOpeningBalance || 0) > 0) {
                                  setOpeningBalanceDisplay(
                                    formatOpeningBalanceDisplay(
                                      typeof watchedOpeningBalance === 'number' ? watchedOpeningBalance : undefined,
                                      nextCurrency,
                                    ),
                                  );
                                }
                              }}
                              isDisabled={mode === 'edit'}
                              bg={mode === 'edit' ? readOnlyBg : undefined}
                            >
                              <option value="IDR">IDR - Indonesian Rupiah</option>
                              <option value="USD">USD - US Dollar</option>
                            </Select>
                          )}
                        />
                       <Text fontSize="xs" color={mutedTextColor} mt={1}>
                         {mode === 'edit' ? 'Mata uang rekening tidak dapat diubah setelah dibuat.' : 'Pilih mata uang utama rekening kas/bank ini.'}
                       </Text>
                     </FormControl>
                  </SimpleGrid>

                  <FormControl mt={4}>
                    <FormLabel>{t('cashBank.form.description')}</FormLabel>
                    <Textarea
                      {...register('description')}
                      placeholder={t('cashBank.form.descriptionPlaceholder')}
                      rows={3}
                      resize="vertical"
                    />
                  </FormControl>

                  <SimpleGrid columns={{ base: 1, md: 2 }} spacing={4} mt={4}>
                    <FormControl>
                      <Controller
                        name="is_holding_account"
                        control={control}
                        render={({ field }) => (
                          <Checkbox
                            isChecked={Boolean(field.value)}
                            onChange={(e) => {
                              const checked = e.target.checked;
                              field.onChange(checked);
                              setValue('allow_direct_transactions', checked ? false : true);
                            }}
                            colorScheme="orange"
                          >
                            Rekening simpanan / holding
                          </Checkbox>
                        )}
                      />
                      <Text fontSize="xs" color={mutedTextColor} mt={1}>
                        Simpan rekening valas seperti USD di Cash & Bank, tetapi jangan dipakai otomatis untuk transaksi operasional harian.
                      </Text>
                    </FormControl>

                    <FormControl>
                      <Checkbox {...register('allow_direct_transactions')} isDisabled={!watchedHoldingAccount} colorScheme="blue">
                        Izinkan transaksi langsung di modul lain
                      </Checkbox>
                      <Text fontSize="xs" color={mutedTextColor} mt={1}>
                        {watchedHoldingAccount
                          ? 'Biarkan nonaktif jika rekening ini hanya dipakai sebagai holding account dan transfer ke bank operasional.'
                          : 'Untuk rekening non-holding, opsi ini selalu aktif.'}
                      </Text>
                    </FormControl>
                  </SimpleGrid>
                </CardBody>
              </Card>

              {/* Bank Details (only for BANK type) */}
              {watchedType === 'BANK' && (
                <Card>
                  <CardBody>
                    <Text fontWeight="bold" mb={4} color={textColor}>
                      🏦 {t('cashBank.form.bankDetails')}
                    </Text>

                    <Alert status={openingBalanceAlert.status} borderRadius="md" mb={4} variant="left-accent">
                      <AlertIcon />
                      <Box flex="1">
                        <HStack spacing={2} mb={1} flexWrap="wrap">
                          <AlertTitle fontSize="sm" fontWeight="bold" mb={0}>
                            {openingBalanceAlert.title}
                          </AlertTitle>
                          <Badge colorScheme={openingBalanceAlert.badgeColorScheme} variant="subtle">
                            Mata uang aktif: {normalizedWatchedCurrency}
                          </Badge>
                        </HStack>
                        <AlertDescription fontSize="xs">
                          {openingBalanceAlert.description}
                        </AlertDescription>
                      </Box>
                    </Alert>

                    <SimpleGrid columns={{ base: 1, md: 2 }} spacing={4}>
                      <FormControl isRequired isInvalid={!!errors.bank_name}>
                        <FormLabel>
                          <HStack>
                            <FiCreditCard />
                            <Text>{t('cashBank.form.bankName')}</Text>
                          </HStack>
                        </FormLabel>
                        <Input
                          {...register('bank_name', {
                            required: watchedType === 'BANK' ? t('cashBank.form.bankNameRequired') : false
                          })}
                          placeholder={t('cashBank.form.bankNamePlaceholder')}
                        />
                        <FormErrorMessage>{errors.bank_name?.message}</FormErrorMessage>
                      </FormControl>

                      <FormControl>
                        <FormLabel>{t('cashBank.form.accountNumber')}</FormLabel>
                        <Input
                          {...register('account_no')}
                          placeholder={t('cashBank.form.accountNumberPlaceholder')}
                        />
                        <Text fontSize="xs" color={mutedTextColor} mt={1}>
                          {t('cashBank.form.accountNumberOptional')}
                        </Text>
                      </FormControl>

                      <FormControl>
                        <FormLabel>{t('cashBank.form.atasNama')}</FormLabel>
                        <Input
                          {...register('account_holder_name')}
                          placeholder={t('cashBank.form.atasNamaPlaceholder')}
                        />
                        <Text fontSize="xs" color={mutedTextColor} mt={1}>
                          {t('cashBank.form.atasNamaOptional')}
                        </Text>
                      </FormControl>

                      <FormControl>
                        <FormLabel>{t('cashBank.form.cabang')}</FormLabel>
                        <Input
                          {...register('branch')}
                          placeholder={t('cashBank.form.cabangPlaceholder')}
                        />
                        <Text fontSize="xs" color={mutedTextColor} mt={1}>
                          {t('cashBank.form.cabangOptional')}
                        </Text>
                      </FormControl>
                    </SimpleGrid>
                  </CardBody>
                </Card>
              )}

              {/* Opening Balance (only for create mode) */}
              {mode === 'create' && (
                <Card>
                  <CardBody>
                    <Text fontWeight="bold" mb={4} color={textColor}>
                      💰 {t('cashBank.form.initialSetup')}
                    </Text>

                    <SimpleGrid columns={{ base: 1, md: 2 }} spacing={4}>
                      <FormControl>
                        <FormLabel>
                          <HStack>
                            <FiDollarSign />
                            <Text>{t('cashBank.form.openingBalance')}</Text>
                            <Badge colorScheme={openingBalanceAlert.badgeColorScheme} variant="outline">
                              {normalizedWatchedCurrency}
                            </Badge>
                          </HStack>
                        </FormLabel>
                        <Controller
                          name="opening_balance"
                          control={control}
                          render={({ field }) => (
                            <InputGroup>
                              <InputLeftAddon bg={inputAddonBg} color={textColor} fontWeight="medium">
                                {normalizedWatchedCurrency}
                              </InputLeftAddon>
                              <Input
                                placeholder={getOpeningBalancePlaceholder(normalizedWatchedCurrency)}
                                value={openingBalanceDisplay}
                                inputMode={normalizedWatchedCurrency === 'USD' ? 'decimal' : 'numeric'}
                                onChange={(e) => {
                                  const input = e.target.value;
                                  setOpeningBalanceDisplay(input);

                                  const parsedValue = parseOpeningBalanceInput(input, normalizedWatchedCurrency);
                                  field.onChange(parsedValue ?? 0);
                                }}
                                onFocus={(e) => {
                                  // Select all for easy replacement
                                  e.target.select();
                                }}
                                onBlur={() => {
                                  setOpeningBalanceDisplay(
                                    formatOpeningBalanceDisplay(
                                      typeof field.value === 'number' ? field.value : undefined,
                                      normalizedWatchedCurrency,
                                    ),
                                  );
                                }}
                              />
                            </InputGroup>
                          )}
                        />
                        <Text fontSize="xs" color={mutedTextColor} mt={1}>
                          {getOpeningBalanceHelpText(normalizedWatchedCurrency)}
                        </Text>
                      </FormControl>

                      <FormControl isRequired isInvalid={!!errors.opening_date}>
                        <FormLabel>
                          <HStack>
                            <FiCalendar />
                            <Text>{t('cashBank.form.openingDate')}</Text>
                          </HStack>
                        </FormLabel>
                        <Input
                          type="date"
                          {...register('opening_date', {
                            required: t('cashBank.form.openingDateRequired'),
                            validate: {
                              notFuture: (value) => {
                                if (!value) {
                                  return t('cashBank.form.openingDateRequired');
                                }
                                const today = new Date();
                                const inputDate = new Date(value);
                                return inputDate <= today || t('cashBank.form.openingDateNotFuture');
                              },
                            },
                          })}
                        />
                        <FormErrorMessage>{errors.opening_date?.message}</FormErrorMessage>
                      </FormControl>
                    </SimpleGrid>

                    {watchedCurrency && watchedCurrency.toUpperCase() !== 'IDR' && (
                      <FormControl mt={4} isRequired={(watchedOpeningBalance || 0) > 0} isInvalid={!!errors.opening_exchange_rate}>
                        <FormLabel>
                          <HStack>
                            <FiDollarSign />
                            <Text>Kurs Saldo Awal ke IDR</Text>
                          </HStack>
                        </FormLabel>
                        <Controller
                          name="opening_exchange_rate"
                          control={control}
                          rules={{
                            validate: (value) => {
                              if ((watchedOpeningBalance || 0) <= 0) return true;
                              return (value || 0) > 0 || 'Kurs wajib diisi untuk saldo awal valas';
                            },
                          }}
                          render={({ field }) => (
                            <NumberInput
                              min={0}
                              precision={4}
                              value={typeof field.value === 'number' ? field.value : undefined}
                              onChange={(_, valueAsNumber) => field.onChange(Number.isFinite(valueAsNumber) ? valueAsNumber : undefined)}
                            >
                              <NumberInputField placeholder="Contoh: 16250" />
                            </NumberInput>
                          )}
                        />
                        <FormErrorMessage>{errors.opening_exchange_rate?.message}</FormErrorMessage>
                        <Text fontSize="xs" color={mutedTextColor} mt={1}>
                          Isi kurs 1 USD ke IDR. Kurs ini dipakai untuk menghitung nilai buku Rupiah yang masuk ke neraca saat saldo awal dibuat.
                        </Text>
                      </FormControl>
                    )}

                    {/* Source of Funds (Equity) Selection - Double Entry */}
                    {/* Always show in Create Mode for transparency, but only required > 0 */}
                    {mode === 'create' && (
                      <FormControl mt={4} isRequired={(watchedOpeningBalance || 0) > 0} isInvalid={!!errors.source_account_id}>
                        <FormLabel>
                          <HStack>
                            <Text>🏦 {t('cashBank.form.sourceAccount') || 'Sumber Dana (Akun Modal)'}</Text>
                          </HStack>
                        </FormLabel>
                        <Select
                          placeholder={t('cashBank.form.selectEquityPlaceholder') || 'Pilih akun Modal (contoh: 3101 - Modal Disetor)'}
                          {...register('source_account_id', {
                            setValueAs: value => value ? parseInt(value) : undefined,
                            required: (watchedOpeningBalance || 0) > 0 ? (t('cashBank.form.equityAccountRequired') || 'Akun Modal wajib dipilih untuk Saldo Awal') : false
                          })}
                        >
                          {equityAccounts.map((acc) => (
                            <option key={acc.id} value={acc.id}>
                              [{acc.code}] {acc.name}
                            </option>
                          ))}
                        </Select>
                        <FormErrorMessage>{errors.source_account_id?.message}</FormErrorMessage>
                        <Text fontSize="xs" color={mutedTextColor} mt={1}>
                          {t('cashBank.form.equityAccountHelp') || 'Pilih akun Ekuitas/Modal sebagai penyeimbang (Kredit) untuk Saldo Awal Kas/Bank (Debit).'}
                        </Text>
                      </FormControl>
                    )}
                  </CardBody>
                </Card>
              )}

              {/* COA Integration Section (Create Mode Only) */}
              {mode === 'create' && (
                <>
                  <Card bg={cardInfoBg} borderColor={cardInfoBorderColor}>
                    <CardBody>
                      <Box>
                        <Text fontWeight="bold" color={cardInfoTextColor} mb={1}>
                          📊 {t('cashBank.form.coaIntegrationTitle')}
                        </Text>
                        <Text fontSize="sm" color={cardInfoTextColor}>
                          {t('cashBank.form.coaIntegrationDesc')}
                        </Text>
                      </Box>
                    </CardBody>
                  </Card>

                  {/* Manual COA Selection Required - Warning */}
                  <Alert status="warning" borderRadius="md">
                    <AlertIcon />
                    <Box>
                      <AlertTitle fontSize="sm" fontWeight="bold">
                        ⚠️ {t('cashBank.form.manualCOARequired')}
                      </AlertTitle>
                      <AlertDescription fontSize="xs">
                        {t('cashBank.form.manualCOADesc')}
                        <br />
                        <br />
                        <strong>{t('common.placeholders.manualEntry')}:</strong> {t('cashBank.form.manualCOAExample')}
                      </AlertDescription>
                    </Box>
                  </Alert>

                  {/* GL Account Selection */}
                  <Card>
                    <CardBody>
                      <VStack align="stretch" spacing={4}>
                        <FormControl isRequired>
                          <FormLabel fontSize="sm">
                            🎯 {watchedType === 'CASH' ? t('cashBank.form.selectCashGLAccount') || 'Pilih Akun GL Kas' : t('cashBank.form.selectBankGLAccount') || 'Pilih Akun GL Bank'}
                          </FormLabel>
                          {accounts.length === 0 ? (
                            <Alert status="warning" size="sm">
                              <AlertIcon />
                              <Text fontSize="xs">
                                {watchedType === 'CASH'
                                  ? (t('cashBank.form.noCashGLAvailable') || 'Tidak ada akun GL Kas yang tersedia. Semua akun GL Kas sudah digunakan atau belum dibuat.')
                                  : (t('cashBank.form.noBankGLAvailable') || 'Tidak ada akun GL Bank yang tersedia. Semua akun GL Bank sudah digunakan atau belum dibuat.')
                                }
                              </Text>
                            </Alert>
                          ) : (
                            <Select
                              placeholder={watchedType === 'CASH'
                                ? (t('cashBank.form.selectCashGLPlaceholder') || 'Pilih akun GL Kas...')
                                : (t('cashBank.form.selectBankGLPlaceholder') || 'Pilih akun GL Bank...')
                              }
                              {...register('account_id', {
                                setValueAs: value => value ? parseInt(value) : undefined,
                                required: t('cashBank.form.glAccountRequired')
                              })}
                              size="sm"
                            >
                              {accounts.map((acc) => (
                                <option key={acc.id} value={acc.id}>
                                  [{acc.code}] {acc.name} - Balance: {accountService.formatBalance(acc.balance)}
                                </option>
                              ))}
                            </Select>
                          )}
                          <Text fontSize="xs" color={mutedTextColor} mt={1}>
                            {watchedType === 'CASH'
                              ? (t('cashBank.form.cashGLHelp') || 'Hanya menampilkan akun GL dengan nama mengandung "KAS" yang belum digunakan')
                              : (t('cashBank.form.bankGLHelp') || 'Hanya menampilkan akun GL dengan nama mengandung "BANK" yang belum digunakan')
                            }
                          </Text>
                        </FormControl>
                      </VStack>
                    </CardBody>
                  </Card>

                  {/* COA Benefits */}
                  <Card bg={cardSuccessBg} borderColor={cardSuccessBorderColor}>
                    <CardBody>
                      <Text fontWeight="bold" color={cardSuccessTextColor} mb={3}>
                        ✅ {t('cashBank.form.coaBenefits')}
                      </Text>
                      <VStack align="start" spacing={2}>
                        <HStack>
                          <Text fontSize="sm" color={cardSuccessTextColor}>
                            📈 {t('cashBank.form.coaBenefit1')}
                          </Text>
                        </HStack>
                        <HStack>
                          <Text fontSize="sm" color={cardSuccessTextColor}>
                            📊 {t('cashBank.form.coaBenefit2')}
                          </Text>
                        </HStack>
                        <HStack>
                          <Text fontSize="sm" color={cardSuccessTextColor}>
                            🔍 {t('cashBank.form.coaBenefit3')}
                          </Text>
                        </HStack>
                      </VStack>
                    </CardBody>
                  </Card>
                </>
              )}
            </VStack>
          </ModalBody>

          <ModalFooter
            bg={modalFooterBg}
            borderBottomRadius="md"
            py={4}
            px={6}
            borderTop="1px"
            borderColor={borderColor}
          >
            <HStack spacing={3} width="full" justify="flex-end">
              <Button
                variant="outline"
                onClick={handleClose}
                size={{ base: 'sm', md: 'md' }}
                minW="80px"
              >
                {t('cashBank.form.cancel')}
              </Button>
              <Button
                type="submit"
                colorScheme="blue"
                isLoading={isSubmitting || loading}
                loadingText={mode === 'create' ? t('cashBank.form.creating') : t('cashBank.form.updating')}
                size={{ base: 'sm', md: 'md' }}
                minW="140px"
                leftIcon={watchedType === 'CASH' ? <FiDollarSign /> : <FiCreditCard />}
              >
                {mode === 'create' ? t('cashBank.form.createAccount') : t('cashBank.form.updateAccount')}
              </Button>
            </HStack>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  );
};

export default CashBankForm;



