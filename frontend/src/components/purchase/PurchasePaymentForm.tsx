'use client';

import React, { useState, useEffect } from 'react';
import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalFooter,
  ModalBody,
  ModalCloseButton,
  Button,
  FormControl,
  FormLabel,
  Input,
  Select,
  Textarea,
  VStack,
  HStack,
  Alert,
  AlertIcon,
  Text,
  Divider,
  Box,
  useToast,
  Spinner,
  Tooltip,
  Icon,
} from '@chakra-ui/react';
import { FiInfo } from 'react-icons/fi';
import { Purchase, PurchasePaymentRequest } from '@/services/purchaseService';
import purchaseService from '@/services/purchaseService';
import { useTranslation } from '@/hooks/useTranslation';

interface CashBank {
  id: number;
  name: string;
  account_code: string;
  balance: number;
}

interface PurchasePaymentFormProps {
  isOpen: boolean;
  onClose: () => void;
  purchase: Purchase | null;
  onSuccess?: (result: any) => void;
  cashBanks: CashBank[];
}

const PurchasePaymentForm: React.FC<PurchasePaymentFormProps> = ({
  isOpen,
  onClose,
  purchase,
  onSuccess,
  cashBanks = [],
}) => {
  const { t } = useTranslation();
  const toast = useToast();
  const [loading, setLoading] = useState(false);
  const [loadingPurchase, setLoadingPurchase] = useState(false);
  const [freshPurchase, setFreshPurchase] = useState<Purchase | null>(null);
  const [formData, setFormData] = useState<PurchasePaymentRequest>({
    amount: 0,
    payment_date: new Date().toISOString().split('T')[0],
    payment_method: 'Bank Transfer',
    cash_bank_id: undefined,
    reference: '',
    notes: '',
  });
  const [displayAmount, setDisplayAmount] = useState('0');

  // Tooltip descriptions for payment form
  const tooltips = {
    amount: 'Jumlah pembayaran yang akan dibayarkan. Maksimal sesuai sisa outstanding purchase',
    paymentDate: 'Tanggal pembayaran dilakukan',
    paymentMethod: 'Metode pembayaran: Bank Transfer (transfer bank), Cash (tunai), Check (cek)',
    cashBank: 'Pilih akun kas/bank yang akan digunakan untuk pembayaran',
    reference: 'Nomor referensi pembayaran (contoh: nomor transfer, nomor cek, receipt number)',
    notes: 'Catatan tambahan untuk pembayaran ini',
  };

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

  // Fetch fresh purchase data from server when modal opens
  useEffect(() => {
    const fetchFreshPurchase = async () => {
      if (isOpen && purchase) {
        setLoadingPurchase(true);
        try {
          // Fetch latest purchase data from server to get accurate outstanding amount
          const latestPurchase = await purchaseService.getById(purchase.id);
          setFreshPurchase(latestPurchase);

          const defaultAmount = latestPurchase.outstanding_amount || 0;
          setFormData({
            amount: defaultAmount,
            payment_date: new Date().toISOString().split('T')[0],
            payment_method: 'Bank Transfer',
            cash_bank_id: cashBanks.length > 0 ? cashBanks[0].id : undefined,
            reference: '',
            notes: `Payment for purchase ${latestPurchase.code}`,
          });
          setDisplayAmount(formatRupiah(defaultAmount));
        } catch (error) {
          console.error('Error fetching fresh purchase data:', error);
          toast({
            title: 'Warning',
            description: 'Could not load latest purchase data. Please close and try again.',
            status: 'warning',
            duration: 4000,
            isClosable: true,
          });
          // Fallback to use the provided purchase prop
          setFreshPurchase(purchase);
          const defaultAmount = purchase.outstanding_amount || 0;
          setFormData({
            amount: defaultAmount,
            payment_date: new Date().toISOString().split('T')[0],
            payment_method: 'Bank Transfer',
            cash_bank_id: cashBanks.length > 0 ? cashBanks[0].id : undefined,
            reference: '',
            notes: `Payment for purchase ${purchase.code}`,
          });
          setDisplayAmount(formatRupiah(defaultAmount));
        } finally {
          setLoadingPurchase(false);
        }
      } else {
        // Reset when modal closes
        setFreshPurchase(null);
      }
    };

    fetchFreshPurchase();
  }, [isOpen, purchase?.id, cashBanks]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const currentPurchase = freshPurchase || purchase;
    if (!currentPurchase) return;

    // Round amount to ensure it's an integer (no decimals for IDR)
    const roundedAmount = Math.round(formData.amount);
    const dataToSubmit = { ...formData, amount: roundedAmount };

    // Enhanced Validation
    if (!roundedAmount || roundedAmount <= 0) {
      toast({
        title: t('purchases.payment.validationError'),
        description: t('purchases.payment.amountRequired'),
        status: 'error',
        duration: 4000,
        isClosable: true,
      });
      return;
    }

    // Strict validation: prevent exceeding outstanding amount using fresh data
    if (roundedAmount > (currentPurchase.outstanding_amount || 0)) {
      const maxAmount = currentPurchase.outstanding_amount || 0;
      toast({
        title: t('purchases.payment.amountTooHigh') + ' ⚠️',
        description: t('purchases.payment.amountExceedsOutstanding'),
        status: 'error',
        duration: 6000,
        isClosable: true,
      });
      return;
    }

    // Additional validation for minimum payment amount (optional: can be removed if not needed)
    const minPayment = 1000; // Rp 1,000 minimum
    if (roundedAmount < minPayment) {
      toast({
        title: t('purchases.payment.minimumPayment'),
        description: `${t('purchases.payment.amountRequired')} ${formatCurrency(minPayment)}`,
        status: 'error',
        duration: 4000,
        isClosable: true,
      });
      return;
    }

    if (!dataToSubmit.cash_bank_id) {
      toast({
        title: t('purchases.payment.validationError'),
        description: t('purchases.payment.selectAccountRequired'),
        status: 'error',
        duration: 4000,
        isClosable: true,
      });
      return;
    }

    // Balance validation - prevent payments when insufficient balance
    const selectedAccount = cashBanks.find(account => account.id === dataToSubmit.cash_bank_id);
    if (selectedAccount) {
      if (selectedAccount.balance <= 0) {
        toast({
          title: t('purchases.payment.insufficientBalance') + ' ⚠️',
          description: t('purchases.payment.insufficientBalanceDesc'),
          status: 'error',
          duration: 8000,
          isClosable: true,
        });
        return;
      }

      if (roundedAmount > selectedAccount.balance) {
        toast({
          title: t('purchases.payment.insufficientBalance') + ' ⚠️',
          description: t('purchases.payment.insufficientBalanceDesc'),
          status: 'error',
          duration: 10000,
          isClosable: true,
        });
        return;
      }
    }

    setLoading(true);
    try {
      // Use the new Payment Management integration endpoint
      const result = await purchaseService.createPurchasePayment(currentPurchase.id, dataToSubmit);

      // Avoid duplicate success toasts.
      // If a parent onSuccess handler is provided, let the parent show the toast.
      if (onSuccess) {
        onSuccess(result);
      } else {
        toast({
          title: t('purchases.payment.paymentSuccess') + ' 🎉',
          description: t('purchases.payment.paymentSuccessDesc'),
          status: 'success',
          duration: 5000,
          isClosable: true,
        });
      }

      onClose();
    } catch (error: any) {
      console.error('Error recording payment:', error);

      // Check if it's a period closed error (400 - business validation)
      if (error?.response?.status === 400 && error?.response?.data?.code === 'PERIOD_CLOSED') {
        const errorData = error.response.data;
        toast({
          title: errorData.error || 'Periode Sudah Ditutup',
          description: errorData.message || 'Tidak dapat membuat transaksi pada periode yang sudah ditutup.',
          status: 'error',
          duration: 8000,
          isClosable: true,
        });
        setLoading(false);
        return;
      }

      // Check if this is a timeout error
      const isTimeoutError = error.code === 'ECONNABORTED' || error.message?.includes('timeout');

      // Check if this is an insufficient balance error
      const isInsufficientBalance = error.response?.data?.error_type === 'INSUFFICIENT_BALANCE' ||
        error.response?.data?.error?.includes('Saldo rekening tidak mencukupi') ||
        error.message?.includes('insufficient balance');

      if (isTimeoutError) {
        toast({
          title: 'Request Timeout ⏱️',
          description: 'The payment is taking longer than expected to process. This might be due to system load. Please check the payment status in a few moments or try again.',
          status: 'warning',
          duration: 10000,
          isClosable: true,
        });
      } else if (isInsufficientBalance) {
        const availableBalance = error.response?.data?.details?.match(/Available: ([\d\.]+)/);
        const requestedAmount = error.response?.data?.requested_amount || formData.amount;

        let errorMessage = 'Saldo rekening bank yang dipilih tidak mencukupi untuk melakukan pembayaran ini.';
        if (availableBalance) {
          errorMessage += ` Saldo tersedia: ${formatCurrency(parseFloat(availableBalance[1]))}, yang dibutuhkan: ${formatCurrency(requestedAmount)}.`;
        }

        toast({
          title: 'Saldo Tidak Mencukupi ⚠️',
          description: errorMessage,
          status: 'error',
          duration: 8000,
          isClosable: true,
        });
      } else {
        // Generic error handling
        let errorTitle = 'Payment Failed';
        let errorDescription = 'Failed to record payment. Please try again.';

        if (error.response?.data?.message) {
          errorDescription = error.response.data.message;
        } else if (error.response?.data?.error) {
          errorDescription = error.response.data.error;
        } else if (error.message) {
          errorDescription = error.message;
        }

        // Add specific guidance based on error type
        if (error.response?.status === 401) {
          errorTitle = 'Authentication Error';
          errorDescription = 'Your session has expired. Please login again.';
        } else if (error.response?.status === 403) {
          errorTitle = 'Permission Error';
          errorDescription = 'You do not have permission to record payments.';
        } else if (error.response?.status === 500) {
          errorTitle = 'Server Error';
          errorDescription = 'A server error occurred. Please try again later or contact support.';
        }

        toast({
          title: errorTitle,
          description: errorDescription,
          status: 'error',
          duration: 8000,
          isClosable: true,
        });
      }
    } finally {
      setLoading(false);
    }
  };

  // Handle amount input change with Rupiah formatting
  const handleAmountChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const inputValue = e.target.value;

    // Only allow numbers, dots, commas, spaces, and "Rp"
    const allowedCharsRegex = /^[Rp\d.,\s]*$/;
    if (!allowedCharsRegex.test(inputValue)) {
      return; // Ignore invalid characters
    }

    const numericValue = parseRupiah(inputValue);

    // Validate max amount using fresh purchase data
    const currentPurchase = freshPurchase || purchase;
    const maxAmount = currentPurchase?.outstanding_amount || 0;
    if (numericValue > maxAmount) {
      toast({
        title: 'Amount Exceeds Outstanding Balance',
        description: `Maximum payment amount is ${formatCurrency(maxAmount)}`,
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      // Still allow the input but show validation message
    }

    // Update form value
    setFormData(prev => ({ ...prev, amount: numericValue }));

    // Update display value
    setDisplayAmount(formatRupiah(numericValue));
  };

  const handleChange = (field: keyof PurchasePaymentRequest) => (
    e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>
  ) => {
    let value: string | number = e.target.value;

    // Special handling for cash_bank_id field
    if (field === 'cash_bank_id') {
      value = parseFloat(e.target.value) || 0;
    }

    setFormData(prev => ({
      ...prev,
      [field]: value,
    }));
  };

  const formatCurrency = (amount: number): string => {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(amount);
  };

  const formatDate = (dateString: string): string => {
    return new Date(dateString).toLocaleDateString('id-ID', {
      day: '2-digit',
      month: 'long',
      year: 'numeric',
    });
  };

  // Use fresh purchase data if available, fallback to prop
  const currentPurchase = freshPurchase || purchase;

  if (!purchase) return null;

  return (
    <Modal isOpen={isOpen} onClose={onClose} size="lg">
      <ModalOverlay />
      <ModalContent>
        <ModalHeader>{t('purchases.payment.title')}</ModalHeader>
        <ModalCloseButton />
        <form onSubmit={handleSubmit}>
          <ModalBody>
            {loadingPurchase ? (
              <VStack spacing={4} py={8}>
                <Spinner size="lg" />
                <Text>{t('common.loading')}</Text>
              </VStack>
            ) : (
              <VStack spacing={4} align="stretch">
                {/* Purchase Information */}
                <Box p={4} bg="gray.50" borderRadius="md">
                  <Text fontSize="sm" fontWeight="bold" color="gray.600" mb={2}>
                    {t('purchases.payment.purchaseInfo').toUpperCase()}
                  </Text>
                  <HStack justify="space-between">
                    <Box>
                      <Text fontSize="sm" color="gray.600">{t('purchases.purchaseNumber')}</Text>
                      <Text fontWeight="bold">{currentPurchase?.code}</Text>
                    </Box>
                    <Box>
                      <Text fontSize="sm" color="gray.600">{t('purchases.vendor')}</Text>
                      <Text fontWeight="bold">{currentPurchase?.vendor?.name}</Text>
                    </Box>
                    <Box>
                      <Text fontSize="sm" color="gray.600">{t('purchases.date')}</Text>
                      <Text fontWeight="bold">{formatDate(currentPurchase?.date || '')}</Text>
                    </Box>
                  </HStack>
                  <Divider my={2} />
                  <HStack justify="space-between">
                    <Box>
                      <Text fontSize="sm" color="gray.600">{t('purchases.payment.totalAmount')}</Text>
                      <Text fontWeight="bold">{formatCurrency(currentPurchase?.total_amount || 0)}</Text>
                    </Box>
                    <Box>
                      <Text fontSize="sm" color="gray.600">{t('purchases.payment.paidAmount')}</Text>
                      <Text fontWeight="bold">{formatCurrency(currentPurchase?.paid_amount || 0)}</Text>
                    </Box>
                    <Box>
                      <Text fontSize="sm" color={currentPurchase?.outstanding_amount === 0 ? "green.600" : "red.600"}>
                        {t('purchases.payment.outstanding')}
                      </Text>
                      <Text fontWeight="bold" color={currentPurchase?.outstanding_amount === 0 ? "green.600" : "red.600"}>
                        {formatCurrency(currentPurchase?.outstanding_amount || 0)}
                      </Text>
                    </Box>
                  </HStack>
                  {currentPurchase?.outstanding_amount === 0 && (
                    <Alert status="success" mt={3}>
                      <AlertIcon />
                      <Text fontSize="sm">{t('purchases.payment.paymentSuccess')}</Text>
                    </Alert>
                  )}
                </Box>

                <Alert status="info">
                  <AlertIcon />
                  <Text fontSize="sm">
                    {t('purchases.payment.paymentSuccessDesc')}
                    {loading && (
                      <><br />⏳ <strong>{t('purchases.payment.processing')}</strong></>
                    )}
                  </Text>
                </Alert>

                {/* Payment Form */}
                <FormControl isRequired>
                  <FormLabel>{t('purchases.payment.amount')} *</FormLabel>
                  <Input
                    placeholder="Rp 0"
                    value={`Rp ${displayAmount}`}
                    onChange={handleAmountChange}
                    textAlign="right"
                    fontWeight="medium"
                    fontSize="md"
                    pl={8}
                  />

                  {/* Quick Payment Amount Buttons */}
                  <HStack spacing={2} mt={3} flexWrap="wrap">
                    <Text fontSize="sm" color="gray.600" minW="fit-content">Quick Select:</Text>
                    <Button
                      size="xs"
                      variant="outline"
                      colorScheme="blue"
                      onClick={() => {
                        // Use floor to ensure consistent integer amounts
                        const amount = Math.floor((currentPurchase?.outstanding_amount || 0) * 0.25);
                        setFormData(prev => ({ ...prev, amount }));
                        setDisplayAmount(formatRupiah(amount));
                      }}
                      disabled={!currentPurchase?.outstanding_amount}
                    >
                      25%
                    </Button>
                    <Button
                      size="xs"
                      variant="outline"
                      colorScheme="blue"
                      onClick={() => {
                        // Use floor to ensure total doesn't exceed outstanding when split 50/50
                        const amount = Math.floor((currentPurchase?.outstanding_amount || 0) * 0.5);
                        setFormData(prev => ({ ...prev, amount }));
                        setDisplayAmount(formatRupiah(amount));
                      }}
                      disabled={!currentPurchase?.outstanding_amount}
                    >
                      50%
                    </Button>
                    <Button
                      size="xs"
                      variant="outline"
                      colorScheme="orange"
                      onClick={() => {
                        // Use floor to ensure consistent integer amounts
                        const amount = Math.floor((currentPurchase?.outstanding_amount || 0) * 0.8);
                        setFormData(prev => ({ ...prev, amount }));
                        setDisplayAmount(formatRupiah(amount));
                      }}
                      disabled={!currentPurchase?.outstanding_amount}
                    >
                      80%
                    </Button>
                    <Button
                      size="xs"
                      variant="solid"
                      colorScheme="green"
                      onClick={() => {
                        const amount = currentPurchase?.outstanding_amount || 0;
                        setFormData(prev => ({ ...prev, amount }));
                        setDisplayAmount(formatRupiah(amount));
                      }}
                      disabled={!currentPurchase?.outstanding_amount}
                    >
                      100% Full Pay
                    </Button>
                  </HStack>

                  {/* Payment Amount Info */}
                  {formData.amount > 0 && (
                    <Text fontSize="sm" color="gray.600" mt={2}>
                      💰 Payment: <Text as="span" fontWeight="bold" color="green.600">
                        {formatCurrency(formData.amount)}
                      </Text>
                      {formData.amount < (currentPurchase?.outstanding_amount || 0) && (
                        <Text as="span" color="orange.500">
                          {' • '} Remaining: {formatCurrency((currentPurchase?.outstanding_amount || 0) - formData.amount)}
                        </Text>
                      )}
                      {formData.amount === (currentPurchase?.outstanding_amount || 0) && (
                        <Text as="span" color="green.500">
                          {' • '} ✅ Full Payment
                        </Text>
                      )}
                    </Text>
                  )}

                  {/* Validation Messages */}
                  {formData.amount > (currentPurchase?.outstanding_amount || 0) && (
                    <Text fontSize="sm" color="red.500" mt={1}>
                      ⚠️ Amount exceeds outstanding balance of {formatCurrency(currentPurchase?.outstanding_amount || 0)}
                    </Text>
                  )}
                  {formData.amount > 0 && formData.amount <= (currentPurchase?.outstanding_amount || 0) && formData.amount < (currentPurchase?.outstanding_amount || 0) && (
                    <Text fontSize="sm" color="blue.600" mt={1}>
                      ✓ Partial payment - Remaining balance: {formatCurrency((currentPurchase?.outstanding_amount || 0) - formData.amount)}
                    </Text>
                  )}
                  {formData.amount === (currentPurchase?.outstanding_amount || 0) && formData.amount > 0 && (
                    <Text fontSize="sm" color="green.600" mt={1} fontWeight="medium">
                      🎉 This will fully pay the purchase!
                    </Text>
                  )}
                </FormControl>

                <FormControl isRequired>
                  <FormLabel>{t('purchases.payment.paymentDate')}</FormLabel>
                  <Input
                    type="date"
                    value={formData.payment_date}
                    onChange={handleChange('payment_date')}
                  />
                </FormControl>

                <FormControl isRequired>
                  <FormLabel>{t('purchases.payment.paymentMethod')}</FormLabel>
                  <Select
                    value={formData.payment_method}
                    onChange={handleChange('payment_method')}
                  >
                    <option value="Bank Transfer">{t('purchases.payment.bankTransfer')}</option>
                    <option value="Cash">{t('purchases.payment.cash')}</option>
                    <option value="Check">{t('purchases.payment.check')}</option>
                    <option value="Other">{t('common.other')}</option>
                  </Select>
                </FormControl>

                <FormControl isRequired>
                  <FormLabel>
                    {t('purchases.payment.cashBankAccount')}
                  </FormLabel>
                  <Select
                    value={formData.cash_bank_id || ''}
                    onChange={handleChange('cash_bank_id')}
                    placeholder={t('purchases.payment.selectAccount')}
                  >
                    {cashBanks
                      .filter(account => {
                        // Filter based on payment method
                        if (formData.payment_method === 'Cash') {
                          return account.account_code?.toUpperCase().includes('CASH') ||
                            account.name?.toUpperCase().includes('CASH') ||
                            (account as any).type === 'CASH';
                        } else {
                          return account.account_code?.toUpperCase().includes('BANK') ||
                            account.name?.toUpperCase().includes('BANK') ||
                            (account as any).type === 'BANK' ||
                            (!account.account_code?.toUpperCase().includes('CASH') &&
                              !account.name?.toUpperCase().includes('CASH'));
                        }
                      })
                      .map((account) => {
                        const isInsufficientBalance = account.balance < formData.amount;
                        const isZeroBalance = account.balance <= 0;
                        const balanceStatus = isZeroBalance ? ' ❌ NO BALANCE' :
                          isInsufficientBalance ? ' ⚠️ INSUFFICIENT' :
                            ' ✅';

                        return (
                          <option
                            key={account.id}
                            value={account.id}
                            style={{
                              color: isZeroBalance ? '#e53e3e' :
                                isInsufficientBalance ? '#dd6b20' :
                                  '#38a169',
                              backgroundColor: isZeroBalance ? '#fed7d7' :
                                isInsufficientBalance ? '#feebc8' :
                                  '#f0fff4'
                            }}
                          >
                            {account.name} ({account.account_code}) - {formatCurrency(account.balance)}{balanceStatus}
                          </option>
                        );
                      })}
                    {/* If no filtered accounts, show all accounts */}
                    {cashBanks.filter(account => {
                      if (formData.payment_method === 'Cash') {
                        return account.account_code?.toUpperCase().includes('CASH') ||
                          account.name?.toUpperCase().includes('CASH') ||
                          (account as any).type === 'CASH';
                      } else {
                        return account.account_code?.toUpperCase().includes('BANK') ||
                          account.name?.toUpperCase().includes('BANK') ||
                          (account as any).type === 'BANK' ||
                          (!account.account_code?.toUpperCase().includes('CASH') &&
                            !account.name?.toUpperCase().includes('CASH'));
                      }
                    }).length === 0 && cashBanks.map((account) => (
                      <option key={account.id} value={account.id}>
                        {account.name} ({account.account_code}) - {formatCurrency(account.balance)}
                      </option>
                    ))}
                  </Select>

                  {/* Real-time Balance Warning */}
                  {formData.cash_bank_id && (() => {
                    const selectedAccount = cashBanks.find(account => account.id === formData.cash_bank_id);
                    if (!selectedAccount) return null;

                    const isZeroBalance = selectedAccount.balance <= 0;
                    const isInsufficientBalance = formData.amount > selectedAccount.balance;
                    const remainingBalance = selectedAccount.balance - formData.amount;

                    if (isZeroBalance) {
                      return (
                        <Text fontSize="sm" color="red.500" mt={2} fontWeight="medium">
                          ❌ <strong>No Balance Available</strong><br />
                          Account "{selectedAccount.name}" has {formatCurrency(selectedAccount.balance)} balance. Cannot process any payments.
                        </Text>
                      );
                    }

                    if (isInsufficientBalance && formData.amount > 0) {
                      return (
                        <Text fontSize="sm" color="orange.500" mt={2} fontWeight="medium">
                          ⚠️ <strong>Insufficient Balance</strong><br />
                          Available: {formatCurrency(selectedAccount.balance)} | Required: {formatCurrency(formData.amount)} |
                          Short by: {formatCurrency(formData.amount - selectedAccount.balance)}
                        </Text>
                      );
                    }

                    if (formData.amount > 0 && remainingBalance >= 0) {
                      return (
                        <Text fontSize="sm" color="green.600" mt={2}>
                          ✅ <strong>Sufficient Balance</strong><br />
                          Available: {formatCurrency(selectedAccount.balance)} | After payment: {formatCurrency(remainingBalance)}
                        </Text>
                      );
                    }

                    return (
                      <Text fontSize="sm" color="blue.600" mt={2}>
                        💰 <strong>Account Balance:</strong> {formatCurrency(selectedAccount.balance)}
                      </Text>
                    );
                  })()}
                </FormControl>

                <FormControl>
                  <FormLabel>{t('purchases.payment.reference')}</FormLabel>
                  <Input
                    value={formData.reference}
                    onChange={handleChange('reference')}
                    placeholder={t('common.placeholders.referenceNumber')}
                  />
                </FormControl>

                <FormControl>
                  <FormLabel>{t('purchases.payment.notes')}</FormLabel>
                  <Textarea
                    value={formData.notes}
                    onChange={handleChange('notes')}
                    placeholder={t('common.placeholders.notes')}
                    rows={3}
                  />
                </FormControl>
              </VStack>
            )}
          </ModalBody>

          <ModalFooter>
            <Button variant="ghost" mr={3} onClick={onClose} disabled={loading}>
              {t('common.cancel')}
            </Button>
            <Button
              colorScheme="green"
              type="submit"
              disabled={loading || loadingPurchase || (() => {
                // Disable if no account selected
                if (!formData.cash_bank_id) return true;

                // Disable if amount is invalid
                if (!formData.amount || formData.amount <= 0) return true;

                // Check balance availability
                const selectedAccount = cashBanks.find(account => account.id === formData.cash_bank_id);
                if (selectedAccount) {
                  // Disable if zero balance or insufficient balance
                  if (selectedAccount.balance <= 0 || formData.amount > selectedAccount.balance) {
                    return true;
                  }
                }

                return false;
              })()}
              leftIcon={loading ? <Spinner size="sm" /> : undefined}
              loadingText={t('purchases.payment.processing')}
              isLoading={loading}
            >
              {(() => {
                if (loading) return t('purchases.payment.processing');

                // Check for balance issues
                const selectedAccount = cashBanks.find(account => account.id === formData.cash_bank_id);
                if (selectedAccount && formData.cash_bank_id) {
                  if (selectedAccount.balance <= 0) {
                    return t('purchases.payment.insufficientBalance');
                  }
                  if (formData.amount > selectedAccount.balance) {
                    return t('purchases.payment.insufficientBalance');
                  }
                }

                return t('purchases.payment.recordPayment');
              })()}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  );
};

export default PurchasePaymentForm;