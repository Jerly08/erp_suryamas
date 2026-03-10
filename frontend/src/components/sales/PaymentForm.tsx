'use client';

import React, { useState, useEffect } from 'react';
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
  Select,
  Textarea,
  VStack,
  HStack,
  Text,
  Box,
  Divider,
  useToast
} from '@chakra-ui/react';
import { useForm } from 'react-hook-form';
import { useAuth } from '@/contexts/AuthContext';
import salesService, { Sale, SalePaymentRequest } from '@/services/salesService';
import cashbankService from '@/services/cashbankService';
import accountService from '@/services/accountService';
import { Account as GLAccount } from '@/types/account';
import { useTranslation } from '@/hooks/useTranslation';

interface PaymentFormProps {
  isOpen: boolean;
  onClose: () => void;
  sale: Sale | null;
  onSave: () => void;
}

interface PaymentFormData {
  date: string;
  amount: number;
  method: string;
  reference: string;
  account_id: number;
  cash_bank_id?: number;
  notes: string;
}

const PaymentForm: React.FC<PaymentFormProps> = ({
  isOpen,
  onClose,
  sale,
  onSave
}) => {
  const { t } = useTranslation();
  const { token, user } = useAuth();
  const [loading, setLoading] = useState(false);
  const [accounts, setAccounts] = useState<any[]>([]);
  const [creditAccounts, setCreditAccounts] = useState<GLAccount[]>([]);
  const [paymentHistory, setPaymentHistory] = useState<any[]>([]);
  const [accountsLoading, setAccountsLoading] = useState(false);
  const [loadingCreditAccounts, setLoadingCreditAccounts] = useState(false);
  const toast = useToast();

  const {
    register,
    handleSubmit,
    reset,
    setValue,
    watch,
    formState: { errors }
  } = useForm<PaymentFormData>();

  const watchAmount = watch('amount');
  const [displayAmount, setDisplayAmount] = useState('0');

  // Format number to Rupiah display
  const formatRupiah = (value: number | string): string => {
    const numValue = typeof value === 'string' ? parseFloat(value) || 0 : value;
    return new Intl.NumberFormat('id-ID').format(numValue);
  };

  // Parse Rupiah string to number
  const parseRupiah = (value: string): number => {
    // Remove Rp prefix, spaces, and convert Indonesian decimal format
    const cleanValue = value
      .replace(/^Rp\s*/, '') // Remove "Rp " prefix
      .replace(/\./g, '') // Remove thousand separators (dots)
      .replace(/,/, '.'); // Convert comma to decimal point
    return parseFloat(cleanValue) || 0;
  };

  // Handle amount input change
  const handleAmountChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const inputValue = e.target.value;
    
    // Only allow numbers, dots, commas, spaces, and "Rp"
    const allowedCharsRegex = /^[Rp\d.,\s]*$/;
    if (!allowedCharsRegex.test(inputValue)) {
      return; // Ignore invalid characters
    }
    
    const numericValue = parseRupiah(inputValue);
    
    // Validate max amount
    if (sale && numericValue > sale.outstanding_amount) {
      return; // Don't allow amount greater than outstanding
    }
    
    // Update form value
    setValue('amount', numericValue);
    
    // Update display value
    setDisplayAmount(formatRupiah(numericValue));
  };

  useEffect(() => {
    if (sale && isOpen) {
      // Set default values
      setValue('date', new Date().toISOString().split('T')[0]);
      const defaultAmount = sale.outstanding_amount || 0;
      setValue('amount', defaultAmount);
      setDisplayAmount(formatRupiah(defaultAmount));
      setValue('method', 'BANK_TRANSFER');
      setValue('account_id', 0);
      setValue('reference', '');
      setValue('notes', '');
      
      // Load accounts for payment
      loadAccounts();
    }
  }, [sale, isOpen, setValue]);

  const loadAccounts = async () => {
    // Check if user is authenticated and has required permissions
    if (!token || !user) {
      toast({
        title: t('sales.messages.authRequired'),
        description: t('sales.messages.pleaseLogin'),
        status: 'error',
        duration: 5000
      });
      return;
    }

    // Check if user has permission to view accounts (based on RBAC)
    const allowedRoles = ['admin', 'finance', 'director', 'employee'];
    if (!allowedRoles.includes(user.role.toLowerCase())) {
      toast({
        title: t('sales.messages.accessDenied'),
        description: t('messages.toast.accessDeniedDesc'),
        status: 'error',
        duration: 5000
      });
      return;
    }

    try {
      setAccountsLoading(true);
      
      // Use cashbank service to get payment accounts (cash and bank accounts)
      const paymentAccounts = await cashbankService.getPaymentAccounts();
      setAccounts(paymentAccounts || []);

      if (paymentAccounts.length === 0) {
        toast({
          title: t('common.noData'),
          description: t('common.noData'),
          status: 'warning',
          duration: 5000
        });
      }
      
    } catch (error: any) {
      console.error('Error loading payment accounts:', error);
      
      // Set empty accounts if service fails
      setAccounts([]);
      
      // Provide more specific error messages based on the error
      let errorMessage = t('messages.toast.serverErrorDesc');
      
      if (error.message?.includes('403') || error.message?.includes('Forbidden')) {
        errorMessage = t('messages.toast.accessDeniedDesc');
      } else if (error.message?.includes('401') || error.message?.includes('Unauthorized')) {
        errorMessage = t('messages.toast.sessionExpiredDesc');
      } else if (error.message?.includes('Network')) {
        errorMessage = t('messages.toast.networkErrorDesc');
      }
      
      toast({
        title: t('messages.toast.error'),
        description: errorMessage,
        status: 'error',
        duration: 5000
      });
    } finally {
      setAccountsLoading(false);
    }
  };

  // Fetch credit accounts (liability) for credit card payment method selection
  const loadCreditAccounts = async () => {
    if (!token) return;
    try {
      setLoadingCreditAccounts(true);
      
      // Try catalog endpoint first for EMPLOYEE role, fallback to regular endpoint
      if (user?.role === 'employee') {
        try {
          const catalogData = await accountService.getAccountCatalog(token, 'LIABILITY');
          const formattedAccounts: GLAccount[] = catalogData.map(item => ({
            id: item.id,
            code: item.code,
            name: item.name,
            type: 'LIABILITY' as const,
            is_active: item.active,
            level: 1,
            is_header: false,
            balance: 0,
            created_at: '',
            updated_at: '',
            description: '',
          }));
          console.log('Formatted credit accounts from catalog:', formattedAccounts);
          setCreditAccounts(formattedAccounts);
          return; // Success, exit early
        } catch (catalogError: any) {
          console.log('Catalog endpoint not available, trying regular endpoint:', catalogError.message);
          // Fall through to try regular endpoint
        }
      }
      
      // Use full account data for other roles or as fallback for EMPLOYEE
      try {
        const data = await accountService.getAccounts(token, 'LIABILITY');
        const list: GLAccount[] = Array.isArray(data) ? data : [];
        console.log('Formatted credit accounts from regular endpoint:', list);
        setCreditAccounts(list);
      } catch (regularError: any) {
        console.error('Regular accounts endpoint also failed:', regularError);
        throw regularError; // Re-throw to be caught by outer catch
      }
    } catch (err: any) {
      console.error('Error fetching credit accounts:', err);
      // If both endpoints fail, fall back to manual entry mode
      setCreditAccounts([]);
      
      // Only show warning for non-EMPLOYEE users or if it's not a permission error
      if (user?.role !== 'employee' || !err.message?.includes('Insufficient permissions')) {
        toast({
          title: 'Limited Access',
          description: 'Unable to load credit accounts list. Credit card payment will use default liability account.',
          status: 'warning',
          duration: 5000,
          isClosable: true,
        });
      }
    } finally {
      setLoadingCreditAccounts(false);
    }
  };

  const onSubmit = async (data: PaymentFormData) => {
    if (!sale) return;

    try {
      setLoading(true);

      // Validate required fields
      if (!data.account_id || data.account_id === 0) {
        toast({
          title: t('messages.toast.validationError'),
          description: t('purchases.payment.selectAccountRequired'),
          status: 'error',
          duration: 3000
        });
        return;
      }

      // For sales payments (receivables), we don't need balance validation
      // because we're receiving money from customers, not paying out
      // Balance validation only applies to payable payments (to vendors)

      // Convert date to proper ISO datetime format for backend
      const paymentDateTime = new Date(data.date).toISOString();
      
      const paymentData: SalePaymentRequest = {
        payment_date: paymentDateTime, // Send full datetime in ISO format
        amount: data.amount,
        payment_method: data.method, // Use correct field name
        reference: data.reference || '', // Ensure it's not undefined
        cash_bank_id: data.account_id, // Use the selected account ID as cash_bank_id
        notes: data.notes || '' // Ensure it's not undefined
      };

      await salesService.createIntegratedPayment(sale.id, paymentData);

      toast({
        title: t('messages.toast.saveSuccess'),
        description: t('messages.toast.saveSuccessDesc'),
        status: 'success',
        duration: 3000
      });

      reset();
      onSave();
      onClose();
    } catch (error: any) {
      toast({
        title: t('messages.toast.error'),
        description: error.response?.data?.message || t('messages.toast.serverErrorDesc'),
        status: 'error',
        duration: 5000
      });
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const paymentMethods = [
    { value: 'CASH', label: t('purchases.payment.cash') },
    { value: 'BANK_TRANSFER', label: t('purchases.payment.bankTransfer') },
    { value: 'CHECK', label: t('purchases.payment.check') },
    { value: 'CREDIT_CARD', label: t('purchases.payment.creditCard') },
    { value: 'DEBIT_CARD', label: t('purchases.payment.debitCard') },
    { value: 'OTHER', label: t('common.other') }
  ];

  return (
    <Modal isOpen={isOpen} onClose={handleClose} size="lg">
      <ModalOverlay />
      <ModalContent>
        <ModalHeader>{t('purchases.payment.title')}</ModalHeader>
        <ModalCloseButton />
        
        <form onSubmit={handleSubmit(onSubmit)}>
          <ModalBody>
            {sale && (
              <Box mb={6} p={4} bg="gray.50" borderRadius="md">
                <VStack align="stretch" spacing={2}>
                  <HStack justify="space-between">
                    <Text fontSize="sm" color="gray.600">{t('sales.form.code')}:</Text>
                    <Text fontWeight="medium">{sale.code}</Text>
                  </HStack>
                  <HStack justify="space-between">
                    <Text fontSize="sm" color="gray.600">{t('sales.invoice')}:</Text>
                    <Text fontWeight="medium">{sale.invoice_number || 'N/A'}</Text>
                  </HStack>
                  <HStack justify="space-between">
                    <Text fontSize="sm" color="gray.600">{t('sales.customer')}:</Text>
                    <Text fontWeight="medium">{sale.customer?.name || 'N/A'}</Text>
                  </HStack>
                  <Divider />
                  <HStack justify="space-between">
                    <Text fontSize="sm" color="gray.600">{t('purchases.payment.totalAmount')}:</Text>
                    <Text fontWeight="bold">
                      {salesService.formatCurrency(sale.total_amount)}
                    </Text>
                  </HStack>
                  <HStack justify="space-between">
                    <Text fontSize="sm" color="gray.600">{t('purchases.payment.paidAmount')}:</Text>
                    <Text>{salesService.formatCurrency(sale.paid_amount)}</Text>
                  </HStack>
                  <HStack justify="space-between">
                    <Text fontSize="sm" color="gray.600">{t('purchases.payment.outstanding')}:</Text>
                    <Text fontWeight="bold" color="orange.600">
                      {salesService.formatCurrency(sale.outstanding_amount)}
                    </Text>
                  </HStack>
                </VStack>
              </Box>
            )}

            {/* Payment History Section */}
            {sale?.sale_payments && sale.sale_payments.length > 0 && (
              <Box mb={4} p={4} bg="blue.50" borderRadius="md" borderLeft="4px" borderColor="blue.400">
                <Text fontSize="sm" fontWeight="bold" color="blue.700" mb={2}>
                  📋 {t('purchases.payment.previousPayments')}
                </Text>
                <VStack spacing={2} align="stretch">
                  {sale.sale_payments.slice(-3).map((payment, index) => (
                    <HStack key={index} justify="space-between" fontSize="sm">
                      <Text color="gray.600">
                        {salesService.formatDate(payment.date)} • {payment.method}
                        {payment.reference && ` • Ref: ${payment.reference}`}
                      </Text>
                      <Text fontWeight="medium" color="green.600">
                        +{salesService.formatCurrency(payment.amount)}
                      </Text>
                    </HStack>
                  ))}
                  {sale.sale_payments.length > 3 && (
                    <Text fontSize="xs" color="gray.500" textAlign="center">
                      ... {t('purchases.payment.andMorePayments', { count: sale.sale_payments.length - 3 })}
                    </Text>
                  )}
                </VStack>
              </Box>
            )}

            <VStack spacing={4} align="stretch">
              <HStack spacing={4}>
                <FormControl isRequired isInvalid={!!errors.date}>
                  <FormLabel>{t('purchases.payment.paymentDate')} *</FormLabel>
                  <Input
                    type="date"
                    max={new Date().toISOString().split('T')[0]} // Prevent future dates
                    {...register('date', {
                      required: t('validation.required'),
                      validate: {
                        notFuture: (value) => {
                          const today = new Date();
                          const inputDate = new Date(value);
                          return inputDate <= today || t('purchases.payment.dateCannotBeFuture');
                        }
                      }
                    })}
                  />
                  <FormErrorMessage>{errors.date?.message}</FormErrorMessage>
                </FormControl>

                <FormControl isRequired isInvalid={!!errors.amount}>
                  <FormLabel>{t('purchases.payment.amount')} *</FormLabel>
                  <Box position="relative">
                    <Input
                      placeholder="Rp 0"
                      value={`Rp ${displayAmount}`}
                      textAlign="right"
                      fontWeight="medium"
                      fontSize="md"
                      pl={8}
                      {...register('amount', {
                        required: t('validation.required'),
                        min: { value: 0.01, message: t('purchases.payment.amountMustBeGreaterThanZero') },
                        max: {
                          value: sale?.outstanding_amount || 0,
                          message: t('purchases.payment.amountExceedsOutstanding')
                        },
                        validate: {
                          notZero: (value) => value > 0 || t('purchases.payment.amountMustBeGreaterThanZero')
                        },
                        onChange: handleAmountChange
                      })}
                    />
                  </Box>
                  
                  {/* Amount Info Display */}
                  {watchAmount > 0 && (
                    <Text fontSize="sm" color="gray.600" mt={1}>
                      💰 {t('purchases.payment.payment')}: <Text as="span" fontWeight="bold" color="green.600">
                        Rp {formatRupiah(watchAmount)}
                      </Text>
                      {sale && watchAmount < sale.outstanding_amount && (
                        <Text as="span" color="orange.500">
                          {' • '} {t('purchases.payment.remaining')}: Rp {formatRupiah(sale.outstanding_amount - watchAmount)}
                        </Text>
                      )}
                      {sale && watchAmount === sale.outstanding_amount && (
                        <Text as="span" color="green.500">
                          {' • '} ✅ {t('purchases.payment.fullPayment')}
                        </Text>
                      )}
                    </Text>
                  )}
                  
                  {/* Quick Amount Selection Buttons */}
                  <HStack spacing={2} mt={2}>
                    <Button
                      size="xs"
                      variant="outline"
                      onClick={() => {
                        const amount = (sale?.outstanding_amount || 0) * 0.25;
                        setValue('amount', amount);
                        setDisplayAmount(formatRupiah(amount));
                      }}
                      disabled={!sale?.outstanding_amount}
                    >
                      25%
                    </Button>
                    <Button
                      size="xs"
                      variant="outline"
                      onClick={() => {
                        const amount = (sale?.outstanding_amount || 0) * 0.5;
                        setValue('amount', amount);
                        setDisplayAmount(formatRupiah(amount));
                      }}
                      disabled={!sale?.outstanding_amount}
                    >
                      50%
                    </Button>
                    <Button
                      size="xs"
                      variant="outline"
                      onClick={() => {
                        const amount = (sale?.outstanding_amount || 0) * 0.75;
                        setValue('amount', amount);
                        setDisplayAmount(formatRupiah(amount));
                      }}
                      disabled={!sale?.outstanding_amount}
                    >
                      75%
                    </Button>
                    <Button
                      size="xs"
                      variant="solid"
                      colorScheme="blue"
                      onClick={() => {
                        const amount = sale?.outstanding_amount || 0;
                        setValue('amount', amount);
                        setDisplayAmount(formatRupiah(amount));
                      }}
                      disabled={!sale?.outstanding_amount}
                    >
                      {t('purchases.payment.fullPayment')}
                    </Button>
                  </HStack>
                  
                  <FormErrorMessage>{errors.amount?.message}</FormErrorMessage>
                  {watchAmount > (sale?.outstanding_amount || 0) && (
                    <Text fontSize="sm" color="red.500" mt={1}>
                      ⚠️ {t('purchases.payment.amountExceedsBalance', { amount: salesService.formatCurrency(sale?.outstanding_amount || 0) })}
                    </Text>
                  )}
                  {watchAmount > 0 && watchAmount <= (sale?.outstanding_amount || 0) && (
                    <Text fontSize="sm" color="green.600" mt={1}>
                      ✓ {t('purchases.payment.remainingBalance')}: {salesService.formatCurrency((sale?.outstanding_amount || 0) - watchAmount)}
                    </Text>
                  )}
                  {watchAmount === (sale?.outstanding_amount || 0) && watchAmount > 0 && (
                    <Text fontSize="sm" color="blue.600" mt={1} fontWeight="medium">
                      🎉 {t('purchases.payment.willFullyPay')}
                    </Text>
                  )}
                </FormControl>
              </HStack>

              <HStack spacing={4}>
                <FormControl isRequired isInvalid={!!errors.method}>
                  <FormLabel>{t('purchases.payment.paymentMethod')}</FormLabel>
                  <Select
                    {...register('method', {
                      required: t('validation.required')
                    })}
                  >
                    <option value="">{t('purchases.payment.selectPaymentMethod')}</option>
                    {paymentMethods.map(method => (
                      <option key={method.value} value={method.value}>
                        {method.label}
                      </option>
                    ))}
                  </Select>
                  <FormErrorMessage>{errors.method?.message}</FormErrorMessage>
                </FormControl>

                {/* Cash/Bank Account dropdown for all payment methods */}
                {watch('method') && (
                  <FormControl isRequired isInvalid={!!errors.account_id}>
                    <FormLabel>
                      {watch('method') === 'CASH' ? t('purchases.payment.cashAccount') : 
                       watch('method') === 'CREDIT_CARD' ? t('purchases.payment.creditCardAccount') : t('purchases.payment.bankAccount')} *
                    </FormLabel>
                    <Select
                      {...register('account_id', {
                        required: watch('method') === 'CASH' ? t('purchases.payment.cashAccountRequired') :
                                 watch('method') === 'CREDIT_CARD' ? t('purchases.payment.creditCardAccountRequired') : t('purchases.payment.bankAccountRequired'),
                        setValueAs: value => parseInt(value) || 0
                      })}
                      disabled={accountsLoading || accounts.length === 0}
                    >
                      {accountsLoading ? (
                        <option value="">{t('common.loading')}</option>
                      ) : accounts.length === 0 ? (
                        <option value="">{t('purchases.payment.noAccountsAvailable')}</option>
                      ) : (
                        <>
                          <option value="">
                            {watch('method') === 'CASH' ? t('purchases.payment.selectCashAccount') :
                             watch('method') === 'CREDIT_CARD' ? t('purchases.payment.selectCreditCardAccount') : t('purchases.payment.selectBankAccount')}
                          </option>
                          {accounts
                            .filter(account => {
                              const method = watch('method');
                              if (method === 'CASH') {
                                return account.type === 'CASH';
                              } else if (method === 'CREDIT_CARD') {
                                // Show all accounts for credit card (could be liability or bank accounts)
                                return true;
                              } else {
                                // For other methods, show BANK accounts
                                return account.type === 'BANK' || account.type !== 'CASH';
                              }
                            })
                            .map(account => {
                              // For sales payments, we don't need balance validation warnings
                              // since we're receiving money, not paying out
                              return (
                                <option key={account.id} value={account.id}>
                                  {account.type === 'BANK' && account.bank_name
                                    ? `${account.code} - ${account.name} (${account.bank_name}) - ${salesService.formatCurrency(account.balance)}${(account.frozen_amount || 0) > 0 ? ` [Terblokir: ${salesService.formatCurrency(account.frozen_amount)}]` : ''}`
                                    : `${account.code} - ${account.name} (${account.type}) - ${salesService.formatCurrency(account.balance)}${(account.frozen_amount || 0) > 0 ? ` [Terblokir: ${salesService.formatCurrency(account.frozen_amount)}]` : ''}`
                                  }
                                </option>
                              );
                            })}
                        </>
                      )}
                    </Select>
                    <FormErrorMessage>{errors.account_id?.message}</FormErrorMessage>
                    
                    {/* For sales payments, show positive account balance info */}
                    {watch('account_id') && (() => {
                      const selectedAccountId = watch('account_id');
                      const currentAmount = watch('amount') || 0;
                      const selectedAccount = accounts.find(account => account.id === selectedAccountId);
                      if (!selectedAccount) return null;
                      
                      const newBalance = selectedAccount.balance + currentAmount;
                      
                      if (currentAmount > 0) {
                        return (
                          <Text fontSize="sm" color="green.600" mt={2}>
                            💰 <strong>{t('purchases.payment.receivingPayment')}</strong><br/>
                            {t('purchases.payment.currentBalance')}: {salesService.formatCurrency(selectedAccount.balance)} |
                            {t('purchases.payment.afterPayment')}: {salesService.formatCurrency(newBalance)}
                          </Text>
                        );
                      }
                      
                      return (
                        <Text fontSize="sm" color="blue.600" mt={2}>
                          💰 <strong>{t('purchases.payment.accountBalance')}:</strong> {salesService.formatCurrency(selectedAccount.balance)}
                        </Text>
                      );
                    })()}
                    
                    {accounts.length === 0 && !accountsLoading && (
                      <Text fontSize="xs" color="orange.500" mt={1}>
                        ⚠️ {t('purchases.payment.noAccountsLoaded', { 
                          type: watch('method') === 'CASH' ? t('purchases.payment.cash').toLowerCase() : 
                                watch('method') === 'CREDIT_CARD' ? t('purchases.payment.creditCard').toLowerCase() : t('purchases.payment.bank').toLowerCase() 
                        })}
                      </Text>
                    )}
                  </FormControl>
                )}
              </HStack>

              <FormControl>
                <FormLabel>{t('purchases.payment.reference')}</FormLabel>
                <Input
                  placeholder={t('common.placeholders.referenceNumber')}
                  {...register('reference')}
                />
              </FormControl>

              <FormControl>
                <FormLabel>{t('purchases.payment.notes')}</FormLabel>
                <Textarea
                  placeholder={t('purchases.payment.additionalNotes')}
                  {...register('notes')}
                />
              </FormControl>
            </VStack>
          </ModalBody>

          <ModalFooter>
            <Button variant="ghost" mr={3} onClick={handleClose}>
              {t('common.cancel')}
            </Button>
            <Button
              type="submit"
              colorScheme="blue"
              isLoading={loading}
              loadingText={t('purchases.payment.processing')}
              isDisabled={loading}
            >
              {loading ? t('purchases.payment.processing') : t('purchases.payment.recordPayment')}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  );
};

export default PaymentForm;
