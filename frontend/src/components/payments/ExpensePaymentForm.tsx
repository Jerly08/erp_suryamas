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
  FiBriefcase
} from 'react-icons/fi';
import paymentService from '@/services/paymentService';
import accountService from '@/services/accountService';
import cashbankService, { CashBank, WithdrawalRequest } from '@/services/cashbankService';
import searchableSelectService, { Contact as SelectContact } from '@/services/searchableSelectService';
import closingHistoryService from '@/services/closingHistoryService';
import { AccountCatalogItem } from '@/types/account';
import CurrencyInput from '@/components/common/CurrencyInput';
import { useAuth } from '@/contexts/AuthContext';
interface ExpensePaymentFormProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess?: () => void;
}

interface ExpensePaymentFormData {
  contact_id: number;
  expense_account_id: number;
  cash_bank_id: number;
  date: string;
  amount: number;
  method: string;
  reference: string;
  notes: string;
  description: string;
}

// For expense payments we only support two methods: Cash and Bank Transfer
const PAYMENT_METHODS = [
  { value: 'CASH', label: 'Cash', icon: FiDollarSign },
  { value: 'BANK_TRANSFER', label: 'Bank Transfer', icon: FiCreditCard },
];

const ExpensePaymentForm: React.FC<ExpensePaymentFormProps> = ({
  isOpen,
  onClose,
  onSuccess,
}) => {
  const { token } = useAuth();
  const toast = useToast();
  const { t } = useTranslation();

  // Color mode values
  const bgColor = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const textColor = useColorModeValue('gray.600', 'gray.300');
  const labelColor = useColorModeValue('gray.700', 'gray.200');
  const selectedAccountBg = useColorModeValue('gray.50', 'gray.700');

  const [loading, setLoading] = useState(false);
  const [contacts, setContacts] = useState<SelectContact[]>([]);
  const [expenseAccounts, setExpenseAccounts] = useState<AccountCatalogItem[]>([]);
  const [liabilityAccounts, setLiabilityAccounts] = useState<AccountCatalogItem[]>([]);
  const [cashBanks, setCashBanks] = useState<CashBank[]>([]);
  const [loadingAccounts, setLoadingAccounts] = useState(true);
  const [checkingClosedPeriod, setCheckingClosedPeriod] = useState(false);
  const [isDateClosed, setIsDateClosed] = useState(false);
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
      method: 'CASH',
      reference: '',
      notes: '',
      description: '',
    },
  });

  const selectedAccountId = watch('expense_account_id');
  const selectedMethod = watch('method');
  const paymentDate = watch('date');
  const selectedAccount = [...expenseAccounts, ...liabilityAccounts].find(
    acc => acc.id === Number(selectedAccountId)
  );
  // Filter cash/bank accounts based on selected payment method
  const filteredCashBanks = cashBanks.filter(cb => {
    if (selectedMethod === 'CASH') return cb.type === 'CASH';
    if (selectedMethod === 'BANK_TRANSFER') return cb.type === 'BANK';
    return true;
  });

  // Load accounts and cash banks
  useEffect(() => {
    if (isOpen && token) {
      loadData();
    }
  }, [isOpen, token]);

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

      // Load contacts (vendors) for tagging expense payee
      const contactData = await searchableSelectService.getContacts({ type: 'VENDOR', is_active: true });
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
    // Validate contact selection
    if (!data.contact_id || data.contact_id === 0) {
      toast({
        title: 'Contact Required',
        description: 'Please select a contact for expense payment',
        status: 'error',
        duration: 3000,
      });
      return;
    }

    setLoading(true);
    try {
      // Create expense payment via unified PaymentService so it appears in Payment Management
      await paymentService.createExpensePayment({
        contact_id: Number(data.contact_id),
        expense_account_id: Number(data.expense_account_id),
        cash_bank_id: Number(data.cash_bank_id),
        date: data.date,
        amount: data.amount,
        method: data.method,
        reference: data.reference || '',
        notes:
          data.notes ||
          data.description ||
          `Payment for ${selectedAccount?.code || ''} ${selectedAccount?.name || 'Expense'}`.trim(),
        description: data.description || selectedAccount?.name || 'Expense Payment',
        auto_create_journal: true,
      });

      toast({
        title: 'Success',
        description: 'Expense payment has been created successfully',
        status: 'success',
        duration: 3000,
      });

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

              {loadingAccounts ? (
                <Box py={8} textAlign="center" width="full">
                  <Spinner size="lg" />
                  <Text mt={2} color={textColor}>{t('common.loading')}</Text>
                </Box>
              ) : (
                <>
                  {/* Contact */}
                  <FormControl isInvalid={!!errors.contact_id} isRequired>
                    <FormLabel color={labelColor}>
                      <HStack spacing={1}>
                        <Icon as={FiCreditCard} />
                        <Text>{t('payments.form.contact')}</Text>
                        <Tooltip label={t('payments.form.contactTooltip')}>
                          <Icon as={FiInfo} boxSize={3} color={textColor} />
                        </Tooltip>
                      </HStack>
                    </FormLabel>
                    <Controller
                      name="contact_id"
                      control={control}
                      rules={{ required: t('payments.form.contactRequired') }}
                      render={({ field }) => (
                        <Select
                          {...field}
                          placeholder={t('payments.form.selectContact')}
                          borderColor={borderColor}
                        >
                          {contacts.map(contact => (
                            <option key={contact.id} value={contact.id}>
                              {contact.name} (Vendor)
                            </option>
                          ))}
                        </Select>
                      )}
                    />
                    <FormHelperText color={textColor}>
                      {t('payments.form.contactHelper')}
                    </FormHelperText>
                    <FormErrorMessage>{errors.contact_id?.message}</FormErrorMessage>
                  </FormControl>

                  {/* Expense/Liability Account Selection */}
                  <FormControl isInvalid={!!errors.expense_account_id} isRequired>
                    <FormLabel color={labelColor}>
                      <HStack spacing={1}>
                        <Icon as={FiBookOpen} />
                        <Text>{t('payments.form.expenseAccount')}</Text>
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
                          {filteredCashBanks.map(cb => (
                            <option key={cb.id} value={cb.id}>
                              {cb.name} ({cb.type}) - Balance: {new Intl.NumberFormat('id-ID', {
                                style: 'currency',
                                currency: 'IDR',
                                minimumFractionDigits: 0,
                              }).format(cb.balance || 0)}
                            </option>
                          ))}
                        </Select>
                      )}
                    />
                    <FormErrorMessage>{errors.cash_bank_id?.message}</FormErrorMessage>
                  </FormControl>

                  <HStack width="full" spacing={4}>
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
                              // Reset selected cash/bank account when payment method changes
                              setValue('cash_bank_id', undefined as any);
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
                        <Text>Amount</Text>
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
                  </FormControl>

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
                      Selected Account:
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
              colorScheme="blue"
              isLoading={loading}
              isDisabled={loadingAccounts || isDateClosed || checkingClosedPeriod}
              leftIcon={<FiDollarSign />}
            >
              {t('payments.form.createExpensePayment')}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  );
};

export default ExpensePaymentForm;