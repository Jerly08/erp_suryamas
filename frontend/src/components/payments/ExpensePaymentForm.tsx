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
  VStack,
  HStack,
  FormControl,
  FormLabel,
  FormErrorMessage,
  FormHelperText,
  Input,
  Select,
  Textarea,
  NumberInput,
  NumberInputField,
  Box,
  Text,
  Divider,
  useToast,
  Alert,
  AlertIcon,
  AlertDescription,
  useColorModeValue,
  Tooltip,
  Icon,
  Spinner,
  Badge,
} from '@chakra-ui/react';
import { useForm, Controller } from 'react-hook-form';
import {
  FiCreditCard,
  FiDollarSign,
  FiCalendar,
  FiFileText,
  FiInfo,
  FiBookOpen,
  FiBriefcase,
  FiPercent
} from 'react-icons/fi';
import paymentService, { stripInternalPaymentReference } from '@/services/paymentService';
import accountService from '@/services/accountService';
import cashbankService, {
  CashBank,
  ReferenceExchangeRate,
  getBookBalanceInIDR,
} from '@/services/cashbankService';
import searchableSelectService, { Contact as SelectContact } from '@/services/searchableSelectService';
import closingHistoryService from '@/services/closingHistoryService';
import { AccountCatalogItem } from '@/types/account';
import CurrencyInput from '@/components/common/CurrencyInput';
import { useAuth } from '@/contexts/AuthContext';
import { isFinanceManagerRole, normalizeRole } from '@/utils/roles';

const formatCashBankBalance = (amount: number, currency?: string) => {
  const normalizedCurrency = (currency || 'IDR').toUpperCase();

  if (normalizedCurrency === 'USD') {
    return `USD ${(amount || 0).toLocaleString('en-US', {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    })}`;
  }

  return new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(amount || 0);
};

const formatIDR = (amount: number) =>
  new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(amount || 0);

const formatUsd = (amount: number) =>
  `USD ${(amount || 0).toLocaleString('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })}`;

const formatExchangeRate = (amount?: number) => {
  if (!amount || amount <= 0) {
    return '-';
  }

  return `Rp ${amount.toLocaleString('id-ID', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 6,
  })}`;
};

interface ExpensePaymentFormProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess?: () => void;
  initialData?: any; // Payment object when editing
}

interface ExpensePaymentFormData {
  contact_id: number;
  expense_account_id: number;
  cash_bank_id: number;
  date: string;
  amount: number;
  exchange_rate?: number;
  method: string;
  reference: string;
  notes: string;
  description: string;
  enable_pph: boolean;
  pph_type: string;
  pph_rate: number;
}

// PPh types with default rates
const PPH_TYPES = [
  { value: 'PPH_21', label: 'PPh 21 - Penghasilan Karyawan', defaultRate: 5 },
  { value: 'PPH_23', label: 'PPh 23 - Jasa, Sewa, Royalti', defaultRate: 2 },
  { value: 'PPH_4_2', label: 'PPh 4(2) Final - Sewa Tanah/Bangunan', defaultRate: 10 },
  { value: 'PPH_22', label: 'PPh 22 - Pembelian Barang', defaultRate: 1.5 },
];

// For expense payments we only support two methods: Cash and Bank Transfer
const PAYMENT_METHODS = [
  { value: 'CASH', label: 'Cash', icon: FiDollarSign },
  { value: 'BANK_TRANSFER', label: 'Bank Transfer', icon: FiCreditCard },
];

const ExpensePaymentForm: React.FC<ExpensePaymentFormProps> = ({
  isOpen,
  onClose,
  onSuccess,
  initialData,
}) => {
  const { token, user } = useAuth();
  const toast = useToast();
  const { t } = useTranslation();

  // Check if user is finance or admin (will submit directly as COMPLETED)
  const userRole = normalizeRole(user?.role);
  const isFinanceOrAdmin = userRole === 'admin' || isFinanceManagerRole(userRole);

  // Color mode values
  const bgColor = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const textColor = useColorModeValue('gray.600', 'gray.300');
  const labelColor = useColorModeValue('gray.700', 'gray.200');
  const selectedAccountBg = useColorModeValue('gray.50', 'gray.700');
  const pphBg = useColorModeValue('blue.50', 'blue.900');
  const pphBorderColor = useColorModeValue('blue.200', 'blue.700');
  const pphPreviewBg = useColorModeValue('white', 'gray.800');
  const greenColor = useColorModeValue('green.600', 'green.300');

  const [loading, setLoading] = useState(false);
  const [contacts, setContacts] = useState<SelectContact[]>([]);
  const [expenseAccounts, setExpenseAccounts] = useState<AccountCatalogItem[]>([]);
  const [liabilityAccounts, setLiabilityAccounts] = useState<AccountCatalogItem[]>([]);
  const [cashBanks, setCashBanks] = useState<CashBank[]>([]);
  const [loadingAccounts, setLoadingAccounts] = useState(true);
  const [checkingClosedPeriod, setCheckingClosedPeriod] = useState(false);
  const [isDateClosed, setIsDateClosed] = useState(false);
  const [referenceRate, setReferenceRate] = useState<ReferenceExchangeRate | null>(null);
  const [loadingReferenceRate, setLoadingReferenceRate] = useState(false);
  const [manualRateOverride, setManualRateOverride] = useState(false);
  const {
    control,
    register,
    handleSubmit,
    formState: { errors },
    reset,
    watch,
    setValue,
  } = useForm<ExpensePaymentFormData>({
    defaultValues: {
      date: new Date().toISOString().split('T')[0],
      amount: 0,
      exchange_rate: 1,
      method: 'CASH',
      reference: '',
      notes: '',
      description: '',
      enable_pph: false,
      pph_type: '',
      pph_rate: 0,
    },
  });

  const selectedAccountId = watch('expense_account_id');
  const selectedMethod = watch('method');
  const paymentDate = watch('date');
  const enablePPH = watch('enable_pph');
  const pphType = watch('pph_type');
  const pphRate = watch('pph_rate');
  const amount = watch('amount');
  const selectedCashBankId = watch('cash_bank_id');
  const exchangeRate = watch('exchange_rate');

  // Calculate PPh amounts
  const pphAmount = enablePPH && pphRate > 0 && amount > 0
    ? Math.round(amount * (pphRate / 100))
    : 0;
  const netPayment = amount - pphAmount;

  const selectedAccount = [...expenseAccounts, ...liabilityAccounts].find(
    acc => acc.id === Number(selectedAccountId)
  );
  const selectedCashBank = cashBanks.find(
    cashBank => cashBank.id === Number(selectedCashBankId)
  );
  const selectedCashBankCurrency = (selectedCashBank?.currency || 'IDR').toUpperCase();
  const isUsdCashBank = selectedCashBankCurrency === 'USD';
  const requiresExchangeRate = isUsdCashBank;
  const availableNativeBalance = selectedCashBank
    ? Math.max(0, (selectedCashBank.balance || 0) - (selectedCashBank.frozen_amount || 0))
    : 0;
  const currentBookBalanceIDR = selectedCashBank ? getBookBalanceInIDR(selectedCashBank) : 0;
  const availableBookBalance = selectedCashBank
    ? isUsdCashBank && (selectedCashBank.balance || 0) > 0
      ? currentBookBalanceIDR * (availableNativeBalance / selectedCashBank.balance)
      : availableNativeBalance
    : 0;
  const appliedExchangeRate = requiresExchangeRate ? exchangeRate || 0 : 1;
  const requiredNativeOut = requiresExchangeRate && appliedExchangeRate > 0
    ? netPayment / appliedExchangeRate
    : netPayment;
  const currentCarryRate = isUsdCashBank && availableNativeBalance > 0
    ? availableBookBalance / availableNativeBalance
    : 1;
  const bankBookOutAmount = isUsdCashBank && availableNativeBalance > 0
    ? availableBookBalance * (requiredNativeOut / availableNativeBalance)
    : netPayment;
  const estimatedForexDiff = bankBookOutAmount - netPayment;
  const estimatedForexAmount = Math.abs(estimatedForexDiff);
  const estimatedForexType = estimatedForexDiff > 0.005
    ? 'RUGI KURS'
    : estimatedForexDiff < -0.005
      ? 'LABA KURS'
      : 'NETRAL';
  const endingNativeBalance = Math.max(0, availableNativeBalance - requiredNativeOut);
  const endingBookBalanceIDR = Math.max(0, availableBookBalance - bankBookOutAmount);
  // Show all cash/bank accounts (no filtering by payment method)
  const filteredCashBanks = cashBanks;

  const parseDraftReference = (reference?: string) => {
    if (!reference) {
      return {
        userReference: '',
        expenseAccountId: undefined as number | undefined,
        cashBankId: undefined as number | undefined,
      };
    }

    const expenseMatch = reference.match(/^__EXP_ACC:(\d+),CB:(\d+)__(.*)$/);
    if (expenseMatch) {
      return {
        userReference: expenseMatch[3] || '',
        expenseAccountId: Number(expenseMatch[1]),
        cashBankId: Number(expenseMatch[2]),
      };
    }

    const cashBankMatch = reference.match(/^__CB:(\d+)__(.*)$/);
    if (cashBankMatch) {
      return {
        userReference: cashBankMatch[2] || '',
        expenseAccountId: undefined as number | undefined,
        cashBankId: Number(cashBankMatch[1]),
      };
    }

    return {
      userReference: stripInternalPaymentReference(reference),
      expenseAccountId: undefined as number | undefined,
      cashBankId: undefined as number | undefined,
    };
  };

  // Load accounts and cash banks
  useEffect(() => {
    if (isOpen && token) {
      loadData();
    }
  }, [isOpen, token]);

  // Populate form when editing an existing payment
  useEffect(() => {
    if (isOpen && initialData) {
      const parsedReference = parseDraftReference(initialData.reference);

      // Set form values from the payment record
      if (initialData.contact_id) setValue('contact_id', initialData.contact_id);
      if (initialData.expense_account_id || parsedReference.expenseAccountId) {
        setValue('expense_account_id', Number(initialData.expense_account_id || parsedReference.expenseAccountId));
      }
      if (initialData.cash_bank_id || parsedReference.cashBankId) {
        setValue('cash_bank_id', Number(initialData.cash_bank_id || parsedReference.cashBankId));
      }
      if (initialData.date) setValue('date', initialData.date?.split('T')[0] || initialData.date);
      if (initialData.amount) setValue('amount', initialData.amount);
      if (initialData.exchange_rate) setValue('exchange_rate', initialData.exchange_rate);
      if (initialData.method) setValue('method', initialData.method);
      setValue('reference', stripInternalPaymentReference(parsedReference.userReference));
      if (initialData.notes) setValue('notes', initialData.notes);
      if (initialData.description) setValue('description', initialData.description);
      if (initialData.pph_type) setValue('pph_type', initialData.pph_type);
      if (typeof initialData.pph_rate === 'number') setValue('pph_rate', initialData.pph_rate);
      setValue('enable_pph', Boolean(initialData.pph_type || initialData.pph_amount));
      setManualRateOverride(Boolean(initialData.exchange_rate && initialData.exchange_rate > 0));
    } else if (isOpen && !initialData) {
      // New payment: reset form
      reset({
        date: new Date().toISOString().split('T')[0],
        amount: 0,
        exchange_rate: 1,
        method: 'CASH',
        reference: '',
        notes: '',
        description: '',
        enable_pph: false,
        pph_type: '',
        pph_rate: 0,
      });
      setManualRateOverride(false);
    }
  }, [isOpen, initialData, setValue, reset]);

  useEffect(() => {
    if (!isOpen || !requiresExchangeRate) {
      setReferenceRate(null);
      setLoadingReferenceRate(false);
      setManualRateOverride(false);
      setValue('exchange_rate', 1);
      return;
    }

    let cancelled = false;

    const loadReferenceRate = async () => {
      setLoadingReferenceRate(true);
      try {
        const response = await cashbankService.getReferenceExchangeRate('USD', 'IDR');
        if (cancelled) {
          return;
        }

        setReferenceRate(response);

        if (!manualRateOverride) {
          const suggestedRate =
            response?.exchange_rate ||
            (selectedCashBank?.last_exchange_rate && selectedCashBank.last_exchange_rate > 0
              ? selectedCashBank.last_exchange_rate
              : 0);

          if (suggestedRate > 0) {
            setValue('exchange_rate', Number(suggestedRate.toFixed(6)));
          }
        }
      } finally {
        if (!cancelled) {
          setLoadingReferenceRate(false);
        }
      }
    };

    const fallbackRate =
      selectedCashBank?.last_exchange_rate && selectedCashBank.last_exchange_rate > 0
        ? selectedCashBank.last_exchange_rate
        : 0;
    if (!manualRateOverride && fallbackRate > 0 && !(exchangeRate && exchangeRate > 0)) {
      setValue('exchange_rate', Number(fallbackRate.toFixed(6)));
    }

    loadReferenceRate();

    return () => {
      cancelled = true;
    };
  }, [isOpen, requiresExchangeRate, manualRateOverride, selectedCashBank, exchangeRate, setValue]);

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    if (
      initialData?.id &&
      Number(initialData.cash_bank_id) === Number(selectedCashBankId) &&
      initialData.exchange_rate &&
      initialData.exchange_rate > 0
    ) {
      setManualRateOverride(true);
      return;
    }

    setManualRateOverride(false);
  }, [isOpen, selectedCashBankId, initialData]);

  // Validate payment date against closed accounting periods
  useEffect(() => {
    let cancelled = false;

    const checkClosedPeriod = async () => {
      if (!paymentDate) {
        setIsDateClosed(false);
        return;
      }

      setCheckingClosedPeriod(true);
      try {
        const isClosed = await closingHistoryService.isDateInClosedPeriod(paymentDate);
        if (!cancelled) {
          setIsDateClosed(!!isClosed);
        }
      } catch (err) {
        console.error('Error checking closed period for payment date:', err);
        if (!cancelled) {
          setIsDateClosed(false);
        }
      } finally {
        if (!cancelled) {
          setCheckingClosedPeriod(false);
        }
      }
    };

    checkClosedPeriod();

    return () => {
      cancelled = true;
    };
  }, [paymentDate]);
  const loadData = async () => {
    if (!token) {
      return;
    }

    setLoadingAccounts(true);
    try {
      // Load expense accounts
      const expenseData = await accountService.getAccountCatalog(token, 'EXPENSE');
      setExpenseAccounts(expenseData.filter(acc => acc.active));

      // Load liability accounts
      const liabilityData = await accountService.getAccountCatalog(token, 'LIABILITY');
      setLiabilityAccounts(liabilityData.filter(acc => acc.active));

      // Load cash/bank accounts that can be used for payments
      const cashBankData = await cashbankService.getPaymentAccounts();
      setCashBanks(cashBankData || []);

      // Load contacts (all types for expense payee — vendors, employees, etc.)
      const contactData = await searchableSelectService.getContacts({ is_active: true });
      setContacts(contactData || []);
    } catch (error) {
      console.error('Error loading data:', error);
      toast({
        title: 'Error loading accounts',
        description: 'Failed to load account data. Please try again.',
        status: 'error',
        duration: 3000,
      });
    } finally {
      setLoadingAccounts(false);
    }
  };

  const onSubmit = async (data: ExpensePaymentFormData) => {
    setLoading(true);
    try {
      if (requiresExchangeRate && (!data.exchange_rate || data.exchange_rate <= 0)) {
        throw new Error('Kurs transaksi USD ke IDR wajib diisi sebelum menyimpan pembayaran beban.');
      }

      if (requiresExchangeRate && requiredNativeOut > availableNativeBalance + 0.000001) {
        throw new Error(`Saldo ${selectedCashBank?.name || 'rekening USD'} tidak cukup. Tersedia ${formatUsd(availableNativeBalance)}, dibutuhkan ${formatUsd(requiredNativeOut)}.`);
      }

      const payload: any = {
        contact_id: data.contact_id ? Number(data.contact_id) : 0,
        expense_account_id: Number(data.expense_account_id),
        cash_bank_id: Number(data.cash_bank_id),
        date: data.date,
        amount: data.amount,
        exchange_rate: requiresExchangeRate ? data.exchange_rate : 1,
        method: data.method,
        reference: data.reference || '',
        notes:
          data.notes ||
          data.description ||
          `Payment for ${selectedAccount?.code || ''} ${selectedAccount?.name || 'Expense'}`.trim(),
        description: data.description || selectedAccount?.name || 'Expense Payment',
        auto_create_journal: true,
        pph_type: data.enable_pph ? data.pph_type : '',
        pph_rate: data.enable_pph ? data.pph_rate : 0,
        pph_amount: data.enable_pph ? pphAmount : 0,
      };

      if (initialData?.id) {
        // Update existing draft
        await paymentService.updateDraftPayment(initialData.id, payload);
        toast({
          title: 'Success',
          description: 'Expense payment draft has been updated successfully',
          status: 'success',
          duration: 3000,
        });
      } else {
        // Create new expense payment via unified PaymentService
        const createdPayment = await paymentService.createExpensePayment(payload);
        toast({
          title: 'Success',
          description: createdPayment?.status === 'DRAFT'
            ? 'Expense payment submitted for finance manager approval'
            : 'Expense payment has been created successfully',
          status: 'success',
          duration: 3000,
        });
      }

      handleClose();
      if (onSuccess) onSuccess();
    } catch (error: any) {
      console.error('Error creating expense payment:', error);
      toast({
        title: 'Error creating payment',
        description: error.message || 'Failed to create expense payment',
        status: 'error',
        duration: 5000,
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
    <Modal isOpen={isOpen} onClose={handleClose} size="xl" closeOnOverlayClick={false}>
      <ModalOverlay />
      <ModalContent bg={bgColor}>
        <ModalHeader>
          <HStack>
            <Icon as={FiBriefcase} />
            <Text>{t('payments.form.createExpensePayment')}</Text>
          </HStack>
        </ModalHeader>
        <ModalCloseButton />

        <form onSubmit={handleSubmit(onSubmit)}>
          <ModalBody>
            <VStack spacing={4}>
              {/* Info Alert */}
              <Alert status="info" borderRadius="md">
                <AlertIcon />
                <AlertDescription fontSize="sm">
                  {t('payments.form.expensePaymentDescription')}
                </AlertDescription>
              </Alert>

              {/* Draft warning for non-finance roles */}
              {!isFinanceOrAdmin && (
                <Alert status="warning" borderRadius="md">
                  <AlertIcon />
                  <AlertDescription fontSize="sm">
                    Pembayaran ini akan disimpan sebagai <strong>DRAFT</strong> dan perlu disetujui oleh <strong>Finance Manager</strong> sebelum jurnal dibuat.
                  </AlertDescription>
                </Alert>
              )}

              {loadingAccounts ? (
                <Box py={8} textAlign="center" width="full">
                  <Spinner size="lg" />
                  <Text mt={2} color={textColor}>{t('common.loading')}</Text>
                </Box>
              ) : (
                <>
                  {/* Contact (Optional) */}
                  <FormControl>
                    <FormLabel color={labelColor}>
                      <HStack spacing={1}>
                        <Icon as={FiCreditCard} />
                        <Text>{t('payments.form.contact')} (Opsional)</Text>
                        <Tooltip label={t('payments.form.contactTooltip')}>
                          <Icon as={FiInfo} boxSize={3} color={textColor} />
                        </Tooltip>
                      </HStack>
                    </FormLabel>
                    <Controller
                      name="contact_id"
                      control={control}
                      render={({ field }) => (
                        <Select
                          {...field}
                          placeholder="Tanpa kontak (opsional)"
                          borderColor={borderColor}
                        >
                          {contacts.map(contact => (
                            <option key={contact.id} value={contact.id}>
                              {contact.name} ({contact.type || 'Contact'})
                            </option>
                          ))}
                        </Select>
                      )}
                    />
                    <FormHelperText color={textColor}>
                      {t('payments.form.contactHelper')}
                    </FormHelperText>
                  </FormControl>

                  {/* Expense/Liability Account Selection */}
                  <FormControl isInvalid={!!errors.expense_account_id} isRequired>
                    <FormLabel color={labelColor}>
                      <HStack spacing={1}>
                        <Icon as={FiBookOpen} />
                        <Text>Expense / Liability Account</Text>
                        <Tooltip label={t('payments.form.expenseAccountTooltip')}>
                          <Icon as={FiInfo} boxSize={3} color={textColor} />
                        </Tooltip>
                      </HStack>
                    </FormLabel>
                    <Controller
                      name="expense_account_id"
                      control={control}
                      rules={{ required: t('payments.form.accountRequired') }}
                      render={({ field }) => (
                        <Select
                          {...field}
                          placeholder={t('payments.form.selectAccount')}
                          borderColor={borderColor}
                        >
                          {expenseAccounts.length > 0 && (
                            <>
                              <option disabled style={{ fontWeight: 'bold', color: '#666' }}>
                                ── {t('payments.form.expenseAccounts')} ──
                              </option>
                              {expenseAccounts.map(account => (
                                <option key={account.id} value={account.id}>
                                  {account.code} - {account.name}
                                </option>
                              ))}
                            </>
                          )}
                          {liabilityAccounts.length > 0 && (
                            <>
                              <option disabled style={{ fontWeight: 'bold', color: '#666' }}>
                                ── {t('payments.form.liabilityAccounts')} ──
                              </option>
                              {liabilityAccounts.map(account => (
                                <option key={account.id} value={account.id}>
                                  {account.code} - {account.name}
                                </option>
                              ))}
                            </>
                          )}
                        </Select>
                      )}
                    />
                    <FormErrorMessage>{errors.expense_account_id?.message}</FormErrorMessage>
                    <FormHelperText color={textColor}>
                      Gunakan akun beban untuk pembayaran langsung, atau akun kewajiban bila pelunasan perlu mengurangi liability tertentu.
                    </FormHelperText>
                  </FormControl>

                  {/* Cash/Bank Account */}
                  <FormControl isInvalid={!!errors.cash_bank_id} isRequired>
                    <FormLabel color={labelColor}>
                      <HStack spacing={1}>
                        <Icon as={FiCreditCard} />
                        <Text>{t('payments.form.paymentFrom')}</Text>
                        <Tooltip label={t('payments.form.paymentFromTooltip')}>
                          <Icon as={FiInfo} boxSize={3} color={textColor} />
                        </Tooltip>
                      </HStack>
                    </FormLabel>
                    <Controller
                      name="cash_bank_id"
                      control={control}
                      rules={{ required: t('payments.form.paymentFromRequired') }}
                      render={({ field }) => (
                        <Select {...field} placeholder={t('payments.form.selectAccount')} borderColor={borderColor}>
                          {filteredCashBanks.map(cb => {
                            const frozen = cb.frozen_amount || 0;
                            const available = cb.balance - frozen;
                            return (
                            <option key={cb.id} value={cb.id}>
                              {cb.name} ({cb.type}) - Tersedia: {formatCashBankBalance(available > 0 ? available : 0, cb.currency)}{frozen > 0 ? ` [Terblokir: ${formatCashBankBalance(frozen, cb.currency)}]` : ''}
                            </option>
                            );
                          })}
                        </Select>
                      )}
                    />
                    <FormErrorMessage>{errors.cash_bank_id?.message}</FormErrorMessage>
                    {selectedCashBank && (
                      <FormHelperText color={textColor}>
                        Saldo tersedia: {formatCashBankBalance(availableNativeBalance, selectedCashBank.currency)}
                        {isUsdCashBank ? ` | Nilai buku tersedia: ${formatIDR(availableBookBalance)}` : ''}
                      </FormHelperText>
                    )}
                  </FormControl>

                  <HStack width="full" spacing={4} alignItems="flex-start">
                    {/* Payment Date */}
                    <FormControl isInvalid={!!errors.date || isDateClosed} isRequired flex={1}>
                      <FormLabel color={labelColor}>
                        <HStack spacing={1}>
                          <Icon as={FiCalendar} />
                          <Text>{t('payments.form.paymentDate')}</Text>
                        </HStack>
                      </FormLabel>
                      <Input
                        type="date"
                        {...register('date', { required: t('payments.form.dateRequired') })}
                        borderColor={borderColor}
                        isDisabled={checkingClosedPeriod}
                      />
                      <FormErrorMessage>
                        {errors.date?.message ||
                          (isDateClosed
                            ? t('common.dateClosed')
                            : '')}
                      </FormErrorMessage>
                    </FormControl>

                    {/* Payment Method */}
                    <FormControl isInvalid={!!errors.method} isRequired flex={1}>
                      <FormLabel color={labelColor}>{t('payments.form.paymentMethod')}</FormLabel>
                      <Controller
                        name="method"
                        control={control}
                        rules={{ required: t('payments.form.methodRequired') }}
                        render={({ field }) => (
                          <Select
                            {...field}
                            borderColor={borderColor}
                            onChange={(e) => {
                              const newMethod = e.target.value;
                              field.onChange(newMethod);
                              // No need to reset cash_bank_id — we show all accounts regardless of method
                            }}
                          >
                            {PAYMENT_METHODS.map(method => (
                              <option key={method.value} value={method.value}>
                                {method.label}
                              </option>
                            ))}
                          </Select>
                        )}
                      />
                      <FormErrorMessage>{errors.method?.message}</FormErrorMessage>
                    </FormControl>
                  </HStack>

                  {/* Amount */}
                  <FormControl isInvalid={!!errors.amount} isRequired>
                    <FormLabel color={labelColor}>
                      <HStack spacing={1}>
                        <Icon as={FiDollarSign} />
                        <Text>{isUsdCashBank ? 'Jumlah Beban Bruto (IDR)' : 'Amount'}</Text>
                      </HStack>
                    </FormLabel>
                    <Controller
                      name="amount"
                      control={control}
                      rules={{
                        required: 'Amount is required',
                        min: { value: 0.01, message: 'Amount must be greater than 0' },
                      }}
                      render={({ field }) => (
                        <CurrencyInput
                          value={field.value}
                          onChange={field.onChange}
                          placeholder="Enter payment amount"
                        />
                      )}
                    />
                    <FormErrorMessage>{errors.amount?.message}</FormErrorMessage>
                    <FormHelperText color={textColor}>
                      Nilai yang diinput tetap dalam IDR sebagai beban bruto. Jika ada PPh, kas/bank yang keluar mengikuti nilai neto.
                    </FormHelperText>
                  </FormControl>

                  {requiresExchangeRate && (
                    <Box width="full" p={4} borderRadius="md" bg={pphBg} borderWidth={1} borderColor={pphBorderColor}>
                      <VStack spacing={3} align="stretch">
                        <Input type="hidden" {...register('exchange_rate', { valueAsNumber: true })} />
                        <FormControl isRequired>
                          <FormLabel color={labelColor}>
                            Kurs transaksi (IDR per USD)
                          </FormLabel>
                          <Input
                            type="number"
                            step="0.000001"
                            min="0"
                            value={exchangeRate || ''}
                            onChange={(event) => {
                              setManualRateOverride(true);
                              setValue('exchange_rate', parseFloat(event.target.value) || undefined, {
                                shouldValidate: true,
                                shouldDirty: true,
                              });
                            }}
                            placeholder="Contoh: 16880.270000"
                            borderColor={borderColor}
                          />
                          <FormHelperText color={textColor}>
                            Kurs ini dipakai untuk menghitung USD yang keluar dari rekening dan realized FX saat nilai buku bank USD berbeda dari kurs pembayaran.
                          </FormHelperText>
                        </FormControl>

                        {referenceRate?.exchange_rate ? (
                          <Alert status="info" borderRadius="md">
                            <AlertIcon />
                            <AlertDescription fontSize="sm">
                              Kurs referensi API: 1 USD = {formatExchangeRate(referenceRate.exchange_rate)} dari {referenceRate.provider || referenceRate.source}. Anda tetap bisa override manual.
                            </AlertDescription>
                          </Alert>
                        ) : (
                          <Alert status="warning" borderRadius="md">
                            <AlertIcon />
                            <AlertDescription fontSize="sm">
                              Kurs referensi USD/IDR belum tersedia. Sistem akan tetap memakai kurs manual yang Anda input.
                            </AlertDescription>
                          </Alert>
                        )}

                        {loadingReferenceRate && (
                          <Text fontSize="sm" color={textColor}>
                            Mengambil kurs referensi USD/IDR...
                          </Text>
                        )}

                        {exchangeRate && exchangeRate > 0 && (
                          <Box p={3} borderRadius="md" bg={pphPreviewBg} borderWidth={1} borderColor={borderColor}>
                            <VStack spacing={1} align="stretch" fontSize="sm">
                              <HStack justify="space-between">
                                <Text color={textColor}>Kas keluar (neto)</Text>
                                <Text fontWeight="bold">{formatIDR(netPayment)}</Text>
                              </HStack>
                              <HStack justify="space-between">
                                <Text color={textColor}>USD keluar dari bank</Text>
                                <Text fontWeight="bold">{formatUsd(requiredNativeOut)}</Text>
                              </HStack>
                              <HStack justify="space-between">
                                <Text color={textColor}>Carry rate bank saat ini</Text>
                                <Text fontWeight="bold">{formatExchangeRate(currentCarryRate)}</Text>
                              </HStack>
                              <HStack justify="space-between">
                                <Text color={textColor}>Nilai buku bank yang dikredit</Text>
                                <Text fontWeight="bold">{formatIDR(bankBookOutAmount)}</Text>
                              </HStack>
                              <Divider />
                              <HStack justify="space-between">
                                <Text color={textColor}>Estimasi saldo akhir USD</Text>
                                <Text fontWeight="bold">{formatUsd(endingNativeBalance)}</Text>
                              </HStack>
                              <HStack justify="space-between">
                                <Text color={textColor}>Estimasi nilai buku akhir</Text>
                                <Text fontWeight="bold">{formatIDR(endingBookBalanceIDR)}</Text>
                              </HStack>
                              <HStack justify="space-between">
                                <Text color={estimatedForexType === 'LABA KURS' ? 'green.500' : estimatedForexType === 'RUGI KURS' ? 'red.500' : textColor}>
                                  Estimasi {estimatedForexType}
                                </Text>
                                <Text fontWeight="bold" color={estimatedForexType === 'LABA KURS' ? 'green.500' : estimatedForexType === 'RUGI KURS' ? 'red.500' : textColor}>
                                  {formatIDR(estimatedForexAmount)}
                                </Text>
                              </HStack>
                            </VStack>
                          </Box>
                        )}
                      </VStack>
                    </Box>
                  )}

                  {/* Reference Number */}
                  <FormControl>
                    <FormLabel color={labelColor}>
                      <HStack spacing={1}>
                        <Icon as={FiFileText} />
                        <Text>Reference Number</Text>
                      </HStack>
                    </FormLabel>
                    <Input
                      {...register('reference')}
                      placeholder="e.g., Invoice number, receipt number"
                      borderColor={borderColor}
                    />
                    <FormHelperText color={textColor}>
                      Optional: External reference number for this payment
                    </FormHelperText>
                  </FormControl>

                  {/* === PPh Tax Configuration === */}
                  <Divider />
                  <FormControl display="flex" alignItems="center">
                    <FormLabel color={labelColor} mb={0} htmlFor="enable-pph">
                      <HStack spacing={1}>
                        <Icon as={FiPercent} />
                        <Text>Kenakan PPh (Pemotongan Pajak)</Text>
                      </HStack>
                    </FormLabel>
                    <Controller
                      name="enable_pph"
                      control={control}
                      render={({ field }) => (
                        <input
                          id="enable-pph"
                          type="checkbox"
                          checked={field.value}
                          onChange={(e) => {
                            field.onChange(e.target.checked);
                            if (!e.target.checked) {
                              setValue('pph_type', '');
                              setValue('pph_rate', 0);
                            }
                          }}
                          style={{ width: 20, height: 20, cursor: 'pointer' }}
                        />
                      )}
                    />
                  </FormControl>

                  {enablePPH && (
                    <Box
                      width="full"
                      p={4}
                      borderRadius="md"
                      bg={pphBg}
                      borderWidth={1}
                      borderColor={pphBorderColor}
                    >
                      <VStack spacing={3} align="stretch">
                        <HStack spacing={4} alignItems="flex-start">
                          {/* PPh Type */}
                          <FormControl isRequired flex={2}>
                            <FormLabel color={labelColor} fontSize="sm">Jenis PPh</FormLabel>
                            <Controller
                              name="pph_type"
                              control={control}
                              rules={{ required: enablePPH ? 'Pilih jenis PPh' : false }}
                              render={({ field }) => (
                                <Select
                                  {...field}
                                  placeholder="Pilih jenis PPh"
                                  borderColor={borderColor}
                                  size="sm"
                                  onChange={(e) => {
                                    field.onChange(e.target.value);
                                    const selected = PPH_TYPES.find(t => t.value === e.target.value);
                                    if (selected) {
                                      setValue('pph_rate', selected.defaultRate);
                                    }
                                  }}
                                >
                                  {PPH_TYPES.map(type => (
                                    <option key={type.value} value={type.value}>
                                      {type.label} ({type.defaultRate}%)
                                    </option>
                                  ))}
                                </Select>
                              )}
                            />
                          </FormControl>

                          {/* PPh Rate */}
                          <FormControl isRequired flex={1}>
                            <FormLabel color={labelColor} fontSize="sm">Tarif (%)</FormLabel>
                            <Controller
                              name="pph_rate"
                              control={control}
                              render={({ field }) => (
                                <NumberInput
                                  value={field.value}
                                  onChange={(_, val) => field.onChange(val || 0)}
                                  min={0}
                                  max={100}
                                  precision={1}
                                  size="sm"
                                >
                                  <NumberInputField borderColor={borderColor} />
                                </NumberInput>
                              )}
                            />
                          </FormControl>
                        </HStack>

                        {/* Tax Calculation Preview */}
                        {pphAmount > 0 && (
                          <Box
                            p={3}
                            borderRadius="md"
                            bg={pphPreviewBg}
                            borderWidth={1}
                            borderColor={borderColor}
                          >
                            <VStack spacing={1} align="stretch" fontSize="sm">
                              <HStack justify="space-between">
                                <Text color={textColor}>Jumlah Bruto (Beban)</Text>
                                <Text fontWeight="bold">
                                  {formatIDR(amount)}
                                </Text>
                              </HStack>
                              <HStack justify="space-between">
                                <Text color="red.500">
                                  PPh Dipotong ({pphRate}%)
                                </Text>
                                <Text fontWeight="bold" color="red.500">
                                  - {formatIDR(pphAmount)}
                                </Text>
                              </HStack>
                              <Divider />
                              <HStack justify="space-between">
                                <Text fontWeight="bold" color={greenColor}>
                                  Dibayarkan (Neto)
                                </Text>
                                <Text fontWeight="bold" color={greenColor}>
                                  {formatIDR(netPayment)}
                                </Text>
                              </HStack>
                              <Alert status="info" borderRadius="md" mt={1} py={1}>
                                <AlertIcon boxSize={3} />
                                <AlertDescription fontSize="xs">
                                  Jurnal: Debit Beban {formatIDR(amount)},
                                  Credit Bank {formatIDR(bankBookOutAmount)},
                                  Credit Hutang PPh {formatIDR(pphAmount)}
                                  {isUsdCashBank && estimatedForexAmount > 0 ? `, serta ${estimatedForexType} ${formatIDR(estimatedForexAmount)}` : ''}
                                </AlertDescription>
                              </Alert>
                            </VStack>
                          </Box>
                        )}
                      </VStack>
                    </Box>
                  )}

                  {/* Description */}
                  <FormControl>
                    <FormLabel color={labelColor}>Description</FormLabel>
                    <Input
                      {...register('description')}
                      placeholder="Brief description of the expense"
                      borderColor={borderColor}
                    />
                  </FormControl>

                  {/* Notes */}
                  <FormControl>
                    <FormLabel color={labelColor}>Notes</FormLabel>
                    <Textarea
                      {...register('notes')}
                      placeholder="Additional notes or details"
                      borderColor={borderColor}
                      rows={3}
                    />
                  </FormControl>
                </>
              )}

              {/* Display selected account info */}
              {selectedAccount && (
                <Box
                  width="full"
                  p={3}
                  borderRadius="md"
                  bg={selectedAccountBg}
                  borderWidth={1}
                  borderColor={borderColor}
                >
                  <HStack justify="space-between">
                    <Text fontSize="sm" color={textColor}>
                      Selected Expense / Liability Account:
                    </Text>
                    <Badge colorScheme="blue">
                      {selectedAccount.code} - {selectedAccount.name}
                    </Badge>
                  </HStack>
                </Box>
              )}
            </VStack>
          </ModalBody>

          <ModalFooter>
            <Button variant="ghost" mr={3} onClick={handleClose} isDisabled={loading}>
              {t('common.cancel')}
            </Button>
            <Button
              type="submit"
              colorScheme={isFinanceOrAdmin ? 'blue' : 'orange'}
              isLoading={loading}
              isDisabled={loadingAccounts || isDateClosed || checkingClosedPeriod}
              leftIcon={<FiDollarSign />}
            >
              {isFinanceOrAdmin ? t('payments.form.createExpensePayment') : 'Submit for Approval'}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  );
};

export default ExpensePaymentForm;
