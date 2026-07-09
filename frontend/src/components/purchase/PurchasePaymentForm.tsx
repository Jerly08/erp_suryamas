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
  useColorModeValue,
} from '@chakra-ui/react';
import { Purchase, PurchasePaymentRequest } from '@/services/purchaseService';
import purchaseService from '@/services/purchaseService';
import cashbankService, { ReferenceExchangeRate } from '@/services/cashbankService';
import { useTranslation } from '@/hooks/useTranslation';
import { roundUsdIdrRate } from '@/utils/exchangeRate';

interface CashBank {
  id: number;
  name: string;
  account_code: string;
  balance: number;
  currency?: string;
  book_balance_idr?: number;
  last_exchange_rate?: number;
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
  const { t, language } = useTranslation();
  const toast = useToast();
  const [loading, setLoading] = useState(false);
  const [loadingPurchase, setLoadingPurchase] = useState(false);
  const [freshPurchase, setFreshPurchase] = useState<Purchase | null>(null);
  const [referenceRate, setReferenceRate] = useState<ReferenceExchangeRate | null>(null);
  const [exchangeRateEdited, setExchangeRateEdited] = useState(false);
  const [formData, setFormData] = useState<PurchasePaymentRequest>({
    amount: 0,
    exchange_rate: undefined,
    payment_date: new Date().toISOString().split('T')[0],
    payment_method: 'Bank Transfer',
    cash_bank_id: undefined,
    reference: '',
    notes: '',
  });
  const [displayAmount, setDisplayAmount] = useState('0');
  const modalBg = useColorModeValue('white', 'gray.800');
  const modalHeaderBg = useColorModeValue('gray.50', 'gray.700');
  const modalBorderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const cardBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const infoPanelBg = useColorModeValue('blue.50', 'whiteAlpha.100');
  const previewBg = useColorModeValue('white', 'whiteAlpha.50');
  const headingColor = useColorModeValue('gray.800', 'whiteAlpha.900');
  const textColor = useColorModeValue('gray.700', 'whiteAlpha.900');
  const mutedTextColor = useColorModeValue('gray.600', 'whiteAlpha.700');
  const inputBg = useColorModeValue('white', 'whiteAlpha.50');
  const inputBorderColor = useColorModeValue('gray.300', 'whiteAlpha.300');
  const inputHoverBorder = useColorModeValue('gray.400', 'whiteAlpha.500');
  const inputPlaceholderColor = useColorModeValue('gray.400', 'whiteAlpha.500');
  const dividerColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const infoAlertBg = useColorModeValue('blue.50', 'rgba(66, 153, 225, 0.16)');
  const infoAlertBorder = useColorModeValue('blue.200', 'blue.800');

  // Format number to Rupiah display
  const formatRupiah = (value: number | string): string => {
    const numValue = typeof value === 'string' ? parseFloat(value) || 0 : value;
    return new Intl.NumberFormat('id-ID').format(numValue);
  };

  const roundPaymentAmount = (value: number): number => {
    if (!Number.isFinite(value)) return 0;
    return Math.round(value * 100) / 100;
  };

  const amountsMatch = (left: number, right: number): boolean =>
    Math.abs(roundPaymentAmount(left) - roundPaymentAmount(right)) <= 0.005;

  const exceedsAmount = (left: number, right: number): boolean =>
    roundPaymentAmount(left) - roundPaymentAmount(right) > 0.005;

  // Parse Rupiah string to number
  const parseRupiah = (value: string): number => {
    // Remove Rp prefix, spaces, and convert Indonesian decimal format
    const cleanValue = value
      .replace(/^Rp\s*/, '') // Remove "Rp " prefix
      .replace(/\./g, '') // Remove thousand separators (dots)
      .replace(/,/, '.'); // Convert comma to decimal point
    return parseFloat(cleanValue) || 0;
  };

  const getPaymentTermOutstanding = (term: NonNullable<Purchase['payment_terms']>[number]): number =>
    roundPaymentAmount(
      typeof term.outstanding_amount === 'number'
        ? term.outstanding_amount
        : Math.max((term.amount || 0) - (term.paid_amount || 0), 0)
    );

  const getOpenPaymentTerms = (value?: Purchase | null) =>
    (value?.payment_terms || []).filter(term => getPaymentTermOutstanding(term) > 0.005);

  const getSelectedTerm = (value?: Purchase | null) =>
    (value?.payment_terms || []).find(term => term.id === formData.payment_term_id);

  const getMaxPaymentAmount = (value?: Purchase | null): number => {
    const selectedTerm = getSelectedTerm(value);
    if (selectedTerm) {
      return getPaymentTermOutstanding(selectedTerm);
    }
    return roundPaymentAmount(value?.outstanding_amount || 0);
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

          const defaultTerm = getOpenPaymentTerms(latestPurchase)[0];
          const defaultAmount = defaultTerm
            ? getPaymentTermOutstanding(defaultTerm)
            : roundPaymentAmount(latestPurchase.outstanding_amount || 0);
          setFormData({
            amount: defaultAmount,
            exchange_rate: undefined,
            payment_date: new Date().toISOString().split('T')[0],
            payment_method: 'Bank Transfer',
            cash_bank_id: cashBanks.length > 0 ? cashBanks[0].id : undefined,
            payment_term_id: defaultTerm?.id,
            reference: '',
            notes: `${t('purchases.payment.defaultNotesPrefix')} ${latestPurchase.code}`,
          });
          setDisplayAmount(formatRupiah(defaultAmount));
        } catch (error) {
          console.error('Error fetching fresh purchase data:', error);
          toast({
            title: t('messages.warning'),
            description: t('purchases.payment.latestPurchaseLoadWarning'),
            status: 'warning',
            duration: 4000,
            isClosable: true,
          });
          // Fallback to use the provided purchase prop
          setFreshPurchase(purchase);
          const defaultTerm = getOpenPaymentTerms(purchase)[0];
          const defaultAmount = defaultTerm
            ? getPaymentTermOutstanding(defaultTerm)
            : roundPaymentAmount(purchase.outstanding_amount || 0);
          setFormData({
            amount: defaultAmount,
            exchange_rate: undefined,
            payment_date: new Date().toISOString().split('T')[0],
            payment_method: 'Bank Transfer',
            cash_bank_id: cashBanks.length > 0 ? cashBanks[0].id : undefined,
            payment_term_id: defaultTerm?.id,
            reference: '',
            notes: `${t('purchases.payment.defaultNotesPrefix')} ${purchase.code}`,
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
  }, [isOpen, purchase?.id, cashBanks, t]);

  const selectedAccount = cashBanks.find(account => account.id === formData.cash_bank_id);
  const selectedAccountCurrency = (selectedAccount?.currency || 'IDR').toUpperCase();
  const isUsdAccount = selectedAccountCurrency === 'USD';

  useEffect(() => {
    if (!isOpen || !isUsdAccount) {
      setReferenceRate(null);
      setExchangeRateEdited(false);
      return;
    }

    let cancelled = false;
    const loadReferenceRate = async () => {
      const rate = await cashbankService.getReferenceExchangeRate('USD', 'IDR');
      if (cancelled) {
        return;
      }

      setReferenceRate(rate);
      if (!exchangeRateEdited && rate?.exchange_rate) {
        setFormData(prev => ({
          ...prev,
          exchange_rate: prev.exchange_rate && prev.exchange_rate > 0 ? prev.exchange_rate : roundUsdIdrRate(rate.exchange_rate, 1),
        }));
      }
    };

    loadReferenceRate();
    return () => {
      cancelled = true;
    };
  }, [isOpen, isUsdAccount, selectedAccount?.id, exchangeRateEdited]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const currentPurchase = freshPurchase || purchase;
    if (!currentPurchase) return;

    const maxPaymentAmount = getMaxPaymentAmount(currentPurchase);
    const roundedAmount = roundPaymentAmount(formData.amount);
    const amountToSubmit = amountsMatch(roundedAmount, maxPaymentAmount)
      ? maxPaymentAmount
      : roundedAmount;
    const dataToSubmit = {
      ...formData,
      amount: amountToSubmit,
      exchange_rate: formData.exchange_rate ? roundUsdIdrRate(formData.exchange_rate, 1) : undefined,
    };

    // Enhanced Validation
    if (!amountToSubmit || amountToSubmit <= 0) {
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
    if (exceedsAmount(amountToSubmit, maxPaymentAmount)) {
      const maxAmount = maxPaymentAmount;
      toast({
        title: t('purchases.payment.amountTooHigh') + ' ⚠️',
        description: `Nominal melebihi sisa ${formData.payment_term_id ? 'termin' : 'outstanding'}: ${formatCurrency(maxAmount)}`,
        status: 'error',
        duration: 6000,
        isClosable: true,
      });
      return;
    }

    // Additional validation for minimum payment amount (optional: can be removed if not needed)
    const minPayment = 1000; // Rp 1,000 minimum
    if (amountToSubmit < minPayment) {
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
    const isUsdAccount = (selectedAccount?.currency || 'IDR').toUpperCase() === 'USD';
    if (isUsdAccount && (!dataToSubmit.exchange_rate || dataToSubmit.exchange_rate <= 0)) {
      toast({
        title: t('purchases.payment.exchangeRateRequiredTitle'),
        description: t('purchases.payment.exchangeRateRequiredDesc'),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
      return;
    }

    if (selectedAccount) {
      if (getAccountBookBalance(selectedAccount) <= 0) {
        toast({
          title: t('purchases.payment.insufficientBalance') + ' ⚠️',
          description: t('purchases.payment.insufficientBalanceDesc'),
          status: 'error',
          duration: 8000,
          isClosable: true,
        });
        return;
      }

      if (exceedsAmount(amountToSubmit, getAccountBookBalance(selectedAccount))) {
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
          title: errorData.error || t('purchases.payment.closedPeriodTitle'),
          description: errorData.message || t('purchases.payment.closedPeriodDesc'),
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
          title: t('purchases.payment.requestTimeoutTitle'),
          description: t('purchases.payment.requestTimeoutDesc'),
          status: 'warning',
          duration: 10000,
          isClosable: true,
        });
      } else if (isInsufficientBalance) {
        const availableBalance = error.response?.data?.details?.match(/Available: ([\d\.]+)/);
        const requestedAmount = error.response?.data?.requested_amount || formData.amount;
        const account = cashBanks.find(item => item.id === formData.cash_bank_id);
        const isUsdAccount = (account?.currency || 'IDR').toUpperCase() === 'USD';

        let errorMessage = t('purchases.payment.insufficientBalanceDetailed');
        if (availableBalance) {
          errorMessage += isUsdAccount
            ? ` ${t('purchases.payment.nativeAvailableBalance')}: ${formatAccountBalance(parseFloat(availableBalance[1]), 'USD')}. ${t('purchases.payment.bookPaymentAmount')}: ${formatCurrency(requestedAmount)}.`
            : ` ${t('purchases.payment.availableBalanceLabel')}: ${formatCurrency(parseFloat(availableBalance[1]))}, ${t('purchases.payment.requiredBalanceLabel')}: ${formatCurrency(requestedAmount)}.`;
        }

        toast({
          title: `${t('purchases.payment.insufficientBalance')} ${t('purchases.payment.warningIndicator')}`,
          description: errorMessage,
          status: 'error',
          duration: 8000,
          isClosable: true,
        });
      } else {
        // Generic error handling
        let errorTitle = t('purchases.payment.paymentFailed');
        let errorDescription = t('purchases.payment.paymentFailedDesc');

        if (error.response?.data?.message) {
          errorDescription = error.response.data.message;
        } else if (error.response?.data?.error) {
          errorDescription = error.response.data.error;
        } else if (error.message) {
          errorDescription = error.message;
        }

        // Add specific guidance based on error type
        if (error.response?.status === 401) {
          errorTitle = t('purchases.payment.authenticationErrorTitle');
          errorDescription = t('purchases.payment.authenticationErrorDesc');
        } else if (error.response?.status === 403) {
          errorTitle = t('purchases.payment.permissionErrorTitle');
          errorDescription = t('purchases.payment.permissionErrorDesc');
        } else if (error.response?.status === 500) {
          errorTitle = t('purchases.payment.serverErrorTitle');
          errorDescription = t('purchases.payment.serverErrorDesc');
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

    const numericValue = roundPaymentAmount(parseRupiah(inputValue));

    // Validate max amount using fresh purchase data
    const currentPurchase = freshPurchase || purchase;
    const maxAmount = getMaxPaymentAmount(currentPurchase);
    if (exceedsAmount(numericValue, maxAmount)) {
      toast({
        title: t('purchases.payment.amountTooHigh'),
        description: t('purchases.payment.maxPaymentAmount', { amount: formatCurrency(maxAmount) }),
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
      setExchangeRateEdited(false);
    }
    if (field === 'exchange_rate') {
      value = roundUsdIdrRate(parseFloat(e.target.value), 0);
      setExchangeRateEdited(true);
    }

    setFormData(prev => {
      const next = {
        ...prev,
        [field]: value,
      };

      if (field === 'cash_bank_id') {
        const nextAccount = cashBanks.find(account => account.id === Number(value));
        if ((nextAccount?.currency || 'IDR').toUpperCase() !== 'USD') {
          next.exchange_rate = undefined;
        }
      }

      return next;
    });
  };

  const formatCurrency = (amount: number): string => {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(amount);
  };

  const formatAccountBalance = (amount: number, currency?: string): string => {
    const normalizedCurrency = (currency || 'IDR').toUpperCase();

    if (normalizedCurrency === 'USD') {
      return `USD ${(amount || 0).toLocaleString('en-US', {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
      })}`;
    }

    return formatCurrency(amount || 0);
  };

  const getAccountBookBalance = (account?: CashBank): number => {
    if (!account) return 0;

    if ((account.currency || 'IDR').toUpperCase() === 'IDR') {
      return account.balance || 0;
    }

    if (typeof account.book_balance_idr === 'number') {
      return account.book_balance_idr;
    }

    const fallbackRate = account.last_exchange_rate && account.last_exchange_rate > 0
      ? roundUsdIdrRate(account.last_exchange_rate, 1)
      : 1;

    return (account.balance || 0) * fallbackRate;
  };

  const formatAccountSummary = (account?: CashBank): string => {
    if (!account) {
      return formatCurrency(0);
    }

    const nativeLabel = formatAccountBalance(account.balance || 0, account.currency);
    if ((account.currency || 'IDR').toUpperCase() === 'IDR') {
      return nativeLabel;
    }

    return `${nativeLabel} | Nilai buku: ${formatCurrency(getAccountBookBalance(account))}`;
  };

  const formatExchangeRate = (value?: number): string => {
    if (!value || value <= 0) {
      return '-';
    }

    return `Rp ${value.toLocaleString('id-ID', {
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    })}`;
  };

  const getUsdNativeOutAmount = (): number => {
    if (!isUsdAccount || !formData.amount || !formData.exchange_rate || formData.exchange_rate <= 0) {
      return 0;
    }

    return formData.amount / formData.exchange_rate;
  };

  const formatDate = (dateString: string): string => {
    return new Date(dateString).toLocaleDateString(language === 'en' ? 'en-US' : 'id-ID', {
      day: '2-digit',
      month: 'long',
      year: 'numeric',
    });
  };

  // Use fresh purchase data if available, fallback to prop
  const currentPurchase = freshPurchase || purchase;
  const openPaymentTerms = getOpenPaymentTerms(currentPurchase);
  const selectedPaymentTerm = getSelectedTerm(currentPurchase);
  const maxPaymentAmount = getMaxPaymentAmount(currentPurchase);
  const usdNativeOutAmount = getUsdNativeOutAmount();
  const currentAccountBookBalance = getAccountBookBalance(selectedAccount);
  const currentCarryRate = isUsdAccount && selectedAccount && (selectedAccount.balance || 0) > 0
    ? currentAccountBookBalance / (selectedAccount.balance || 0)
    : 0;
  const bankBookOutAmount = isUsdAccount && selectedAccount && usdNativeOutAmount > 0 && (selectedAccount.balance || 0) > 0
    ? currentAccountBookBalance * (usdNativeOutAmount / selectedAccount.balance)
    : formData.amount;
  const endingBookBalance = Math.max(0, currentAccountBookBalance - bankBookOutAmount);
  const endingNativeBalance = isUsdAccount && selectedAccount
    ? Math.max(0, (selectedAccount.balance || 0) - usdNativeOutAmount)
    : 0;
  const estimatedForexDiff = bankBookOutAmount - formData.amount;
  const estimatedForexType = estimatedForexDiff > 0.005
    ? t('purchases.payment.forexLoss')
    : estimatedForexDiff < -0.005
      ? t('purchases.payment.forexGain')
      : t('purchases.payment.forexNeutral');
  const estimatedForexAmount = Math.abs(estimatedForexDiff);

  if (!purchase) return null;

  return (
    <Modal isOpen={isOpen} onClose={onClose} size="lg">
      <ModalOverlay />
      <ModalContent bg={modalBg} border="1px solid" borderColor={modalBorderColor}>
        <ModalHeader bg={modalHeaderBg} borderBottomWidth="1px" borderColor={modalBorderColor} color={headingColor}>
          {t('purchases.payment.title')}
        </ModalHeader>
        <ModalCloseButton />
        <form onSubmit={handleSubmit}>
          <ModalBody>
            {loadingPurchase ? (
              <VStack spacing={4} py={8}>
                <Spinner size="lg" />
                <Text color={textColor}>{t('common.loading')}</Text>
              </VStack>
            ) : (
              <VStack spacing={4} align="stretch">
                {/* Purchase Information */}
                <Box p={4} bg={cardBg} borderRadius="md" border="1px solid" borderColor={modalBorderColor}>
                  <Text fontSize="sm" fontWeight="bold" color={mutedTextColor} mb={2}>
                    {t('purchases.payment.purchaseInfo').toUpperCase()}
                  </Text>
                  <HStack justify="space-between">
                    <Box>
                      <Text fontSize="sm" color={mutedTextColor}>{t('purchases.purchaseNumber')}</Text>
                      <Text fontWeight="bold" color={textColor}>{currentPurchase?.code}</Text>
                    </Box>
                    <Box>
                      <Text fontSize="sm" color={mutedTextColor}>{t('purchases.vendor')}</Text>
                      <Text fontWeight="bold" color={textColor}>{currentPurchase?.vendor?.name}</Text>
                    </Box>
                    <Box>
                      <Text fontSize="sm" color={mutedTextColor}>{t('purchases.date')}</Text>
                      <Text fontWeight="bold" color={textColor}>{formatDate(currentPurchase?.date || '')}</Text>
                    </Box>
                  </HStack>
                  <Divider my={2} borderColor={dividerColor} />
                  <HStack justify="space-between">
                    <Box>
                      <Text fontSize="sm" color={mutedTextColor}>{t('purchases.payment.totalAmount')}</Text>
                      <Text fontWeight="bold" color={textColor}>{formatCurrency(currentPurchase?.total_amount || 0)}</Text>
                    </Box>
                    <Box>
                      <Text fontSize="sm" color={mutedTextColor}>{t('purchases.payment.paidAmount')}</Text>
                      <Text fontWeight="bold" color={textColor}>{formatCurrency(currentPurchase?.paid_amount || 0)}</Text>
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

                <Alert status="info" bg={infoAlertBg} borderWidth="1px" borderColor={infoAlertBorder}>
                  <AlertIcon />
                  <Text fontSize="sm" color={textColor}>
                    {t('purchases.payment.paymentSuccessDesc')}
                    {loading && (
                      <><br />{t('purchases.payment.processingIndicator')} <strong>{t('purchases.payment.processing')}</strong></>
                    )}
                  </Text>
                </Alert>

                {/* Payment Form */}
                {openPaymentTerms.length > 0 && (
                  <FormControl>
                    <FormLabel color={headingColor}>Termin yang Dibayar</FormLabel>
                    <Select
                      value={formData.payment_term_id || ''}
                      onChange={(e) => {
                        const termID = parseInt(e.target.value, 10) || undefined;
                        const term = openPaymentTerms.find(item => item.id === termID);
                        const nextAmount = term
                          ? getPaymentTermOutstanding(term)
                          : roundPaymentAmount(currentPurchase?.outstanding_amount || 0);
                        setFormData(prev => ({
                          ...prev,
                          payment_term_id: termID,
                          amount: nextAmount,
                        }));
                        setDisplayAmount(formatRupiah(nextAmount));
                      }}
                      bg={inputBg}
                      color={textColor}
                      borderColor={inputBorderColor}
                      _hover={{ borderColor: inputHoverBorder }}
                    >
                      {openPaymentTerms.map(term => (
                        <option key={term.id} value={term.id}>
                          {term.description || `Termin ${term.sequence}`} - {formatCurrency(getPaymentTermOutstanding(term))} jatuh tempo {term.due_date ? new Date(term.due_date).toLocaleDateString('id-ID') : '-'}
                        </option>
                      ))}
                    </Select>
                    {selectedPaymentTerm && (
                      <Text fontSize="xs" mt={2} color={mutedTextColor}>
                        Sisa termin terpilih: {formatCurrency(getPaymentTermOutstanding(selectedPaymentTerm))}
                      </Text>
                    )}
                  </FormControl>
                )}

                <FormControl isRequired>
                  <FormLabel color={headingColor}>{t('purchases.payment.amount')} *</FormLabel>
                  <Input
                    placeholder={t('purchases.payment.amountPlaceholder')}
                    value={`Rp ${displayAmount}`}
                    onChange={handleAmountChange}
                    textAlign="right"
                    fontWeight="medium"
                    fontSize="md"
                    pl={8}
                    bg={inputBg}
                    color={textColor}
                    borderColor={inputBorderColor}
                    _hover={{ borderColor: inputHoverBorder }}
                    _placeholder={{ color: inputPlaceholderColor }}
                  />

                  {/* Quick Payment Amount Buttons */}
                  <HStack spacing={2} mt={3} flexWrap="wrap">
                    <Text fontSize="sm" color={mutedTextColor} minW="fit-content">{t('purchases.payment.quickSelect')}</Text>
                    <Button
                      size="xs"
                      variant="outline"
                      colorScheme="blue"
                      onClick={() => {
                        const amount = roundPaymentAmount(maxPaymentAmount * 0.25);
                        setFormData(prev => ({ ...prev, amount }));
                        setDisplayAmount(formatRupiah(amount));
                      }}
                      disabled={!maxPaymentAmount}
                    >
                      25%
                    </Button>
                    <Button
                      size="xs"
                      variant="outline"
                      colorScheme="blue"
                      onClick={() => {
                        const amount = roundPaymentAmount(maxPaymentAmount * 0.5);
                        setFormData(prev => ({ ...prev, amount }));
                        setDisplayAmount(formatRupiah(amount));
                      }}
                      disabled={!maxPaymentAmount}
                    >
                      50%
                    </Button>
                    <Button
                      size="xs"
                      variant="outline"
                      colorScheme="orange"
                      onClick={() => {
                        const amount = roundPaymentAmount(maxPaymentAmount * 0.8);
                        setFormData(prev => ({ ...prev, amount }));
                        setDisplayAmount(formatRupiah(amount));
                      }}
                      disabled={!maxPaymentAmount}
                    >
                      80%
                    </Button>
                    <Button
                      size="xs"
                      variant="solid"
                      colorScheme="green"
                      onClick={() => {
                        const amount = roundPaymentAmount(maxPaymentAmount);
                        setFormData(prev => ({ ...prev, amount }));
                        setDisplayAmount(formatRupiah(amount));
                      }}
                      disabled={!maxPaymentAmount}
                    >
                      {t('purchases.payment.fullPayButton')}
                    </Button>
                  </HStack>

                  {/* Payment Amount Info */}
                  {formData.amount > 0 && (
                    <Text fontSize="sm" color={mutedTextColor} mt={2}>
                      {t('purchases.payment.paymentIndicator')} <Text as="span" fontWeight="bold" color="green.600">
                        {formatCurrency(formData.amount)}
                      </Text>
                      {!amountsMatch(formData.amount, maxPaymentAmount) &&
                        !exceedsAmount(formData.amount, maxPaymentAmount) && (
                        <Text as="span" color="orange.500">
                          {` ${t('purchases.payment.separator')} `}{t('purchases.payment.remaining')}: {formatCurrency(maxPaymentAmount - formData.amount)}
                        </Text>
                      )}
                      {amountsMatch(formData.amount, maxPaymentAmount) && (
                        <Text as="span" color="green.500">
                          {` ${t('purchases.payment.separator')} `}{t('purchases.payment.fullPaymentStatus')}
                        </Text>
                      )}
                    </Text>
                  )}

                  {/* Validation Messages */}
                  {exceedsAmount(formData.amount, maxPaymentAmount) && (
                    <Text fontSize="sm" color="red.500" mt={1}>
                      {t('purchases.payment.warningIndicator')} {t('purchases.payment.amountExceedsBalance', { amount: formatCurrency(maxPaymentAmount) })}
                    </Text>
                  )}
                  {formData.amount > 0 &&
                    !exceedsAmount(formData.amount, maxPaymentAmount) &&
                    !amountsMatch(formData.amount, maxPaymentAmount) && (
                    <Text fontSize="sm" color="blue.600" mt={1}>
                      {t('purchases.payment.partialPayment')} - {t('purchases.payment.remainingBalance')}: {formatCurrency(maxPaymentAmount - formData.amount)}
                    </Text>
                  )}
                  {amountsMatch(formData.amount, maxPaymentAmount) && formData.amount > 0 && (
                    <Text fontSize="sm" color="green.600" mt={1} fontWeight="medium">
                      {t('purchases.payment.willFullyPay')}
                    </Text>
                  )}
                </FormControl>

                <FormControl isRequired>
                  <FormLabel color={headingColor}>{t('purchases.payment.paymentDate')}</FormLabel>
                  <Input
                    type="date"
                    value={formData.payment_date}
                    onChange={handleChange('payment_date')}
                    bg={inputBg}
                    color={textColor}
                    borderColor={inputBorderColor}
                    _hover={{ borderColor: inputHoverBorder }}
                  />
                </FormControl>

                <FormControl isRequired>
                  <FormLabel color={headingColor}>{t('purchases.payment.paymentMethod')}</FormLabel>
                  <Select
                    value={formData.payment_method}
                    onChange={handleChange('payment_method')}
                    bg={inputBg}
                    color={textColor}
                    borderColor={inputBorderColor}
                    _hover={{ borderColor: inputHoverBorder }}
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
                    bg={inputBg}
                    color={textColor}
                    borderColor={inputBorderColor}
                    _hover={{ borderColor: inputHoverBorder }}
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
                        const accountBookBalance = getAccountBookBalance(account);
                        const isInsufficientBalance = accountBookBalance < formData.amount;
                        const isZeroBalance = accountBookBalance <= 0;
                        const balanceStatus = isZeroBalance
                          ? ` ${t('purchases.payment.noBalanceOption')}`
                          : isInsufficientBalance
                            ? ` ${t('purchases.payment.insufficientOption')}`
                            : ` ${t('purchases.payment.availableOption')}`;

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
                            {account.name} ({account.account_code}) - {formatAccountSummary(account)}{balanceStatus}
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
                        {account.name} ({account.account_code}) - {formatAccountSummary(account)}
                      </option>
                    ))}
                  </Select>

                  {/* Real-time Balance Warning */}
                  {formData.cash_bank_id && (() => {
                    const selectedAccount = cashBanks.find(account => account.id === formData.cash_bank_id);
                    if (!selectedAccount) return null;

                    const accountBookBalance = getAccountBookBalance(selectedAccount);
                    const isZeroBalance = accountBookBalance <= 0;
                    const isInsufficientBalance = formData.amount > accountBookBalance;
                    const remainingBalance = accountBookBalance - formData.amount;

                    if (isZeroBalance) {
                      return (
                        <Text fontSize="sm" color="red.500" mt={2} fontWeight="medium">
                          {t('purchases.payment.noBalanceAvailable')}<br />
                          {t('purchases.payment.accountHasBalance', {
                            account: selectedAccount.name,
                            balance: formatAccountSummary(selectedAccount),
                          })}
                        </Text>
                      );
                    }

                    if (isInsufficientBalance && formData.amount > 0) {
                      return (
                        <Text fontSize="sm" color="orange.500" mt={2} fontWeight="medium">
                          {t('purchases.payment.insufficientBalance')}<br />
                          {t('purchases.payment.availableBalanceLabel')}: {formatAccountSummary(selectedAccount)} | {t('purchases.payment.requiredBalanceLabel')}: {formatCurrency(formData.amount)} |
                          {t('purchases.payment.shortByLabel')}: {formatCurrency(formData.amount - accountBookBalance)}
                        </Text>
                      );
                    }

                    if (formData.amount > 0 && remainingBalance >= 0) {
                      return (
                        <Text fontSize="sm" color="green.600" mt={2}>
                          {t('purchases.payment.sufficientBalance')}<br />
                          {t('purchases.payment.availableBalanceLabel')}: {formatAccountSummary(selectedAccount)} | {t('purchases.payment.afterPayment')}: {formatCurrency(remainingBalance)}
                        </Text>
                      );
                    }

                    return (
                        <Text fontSize="sm" color="blue.600" mt={2}>
                          {t('purchases.payment.accountBalanceLabel')}: {formatAccountSummary(selectedAccount)}
                        </Text>
                      );
                  })()}
                </FormControl>

                {isUsdAccount && (
                  <Box borderWidth="1px" borderRadius="md" p={4} bg={infoPanelBg} borderColor={infoAlertBorder}>
                    <FormControl isRequired>
                      <FormLabel color={headingColor}>{t('purchases.payment.transactionExchangeRate')}</FormLabel>
                      <Input
                        type="number"
                        step="1"
                        min="0"
                        value={formData.exchange_rate ?? ''}
                        onChange={handleChange('exchange_rate')}
                        placeholder={t('purchases.payment.transactionExchangeRatePlaceholder')}
                        bg={inputBg}
                        color={textColor}
                        borderColor={inputBorderColor}
                        _hover={{ borderColor: inputHoverBorder }}
                        _placeholder={{ color: inputPlaceholderColor }}
                      />
                    </FormControl>

                    <Text fontSize="sm" color={mutedTextColor} mt={2}>
                      {t('purchases.payment.transactionExchangeRateHelp')}
                    </Text>

                    {referenceRate?.exchange_rate ? (
                      <Alert status="info" mt={3} borderRadius="md">
                        <AlertIcon />
                        <Text fontSize="sm">
                          {t('purchases.payment.apiReferenceRate', {
                            rate: formatExchangeRate(referenceRate.exchange_rate),
                            provider: referenceRate.provider || referenceRate.source || '-',
                          })} {t('purchases.payment.apiReferenceRateDesc')}
                        </Text>
                      </Alert>
                    ) : (
                      <Alert status="warning" mt={3} borderRadius="md">
                        <AlertIcon />
                        <Text fontSize="sm">
                          {t('purchases.payment.apiReferenceRateUnavailable')}
                        </Text>
                      </Alert>
                    )}

                    {formData.exchange_rate && formData.exchange_rate > 0 && (
                      <Box mt={3} p={3} bg={previewBg} borderRadius="md" border="1px solid" borderColor={modalBorderColor}>
                        <Text fontSize="sm" fontWeight="bold" color={headingColor}>
                          {t('purchases.payment.foreignPaymentPreview')}
                        </Text>
                        <Text fontSize="sm" color={textColor} mt={1}>
                          {t('purchases.payment.purchaseSettled')}: {formatCurrency(formData.amount)}
                        </Text>
                        <Text fontSize="sm" color={textColor}>
                          {t('purchases.payment.usdOutflow')}: {formatAccountBalance(usdNativeOutAmount, 'USD')}
                        </Text>
                        <Text fontSize="sm" color={textColor}>
                          {t('purchases.payment.currentCarryRate')}: 1 USD = {formatExchangeRate(currentCarryRate)}
                        </Text>
                        {selectedAccount && (
                          <>
                            <Text fontSize="sm" color={textColor}>
                              {t('purchases.payment.bankBookCredit')}: {formatCurrency(bankBookOutAmount)}
                            </Text>
                            <Text fontSize="sm" color={textColor}>
                              {t('purchases.payment.nativeBalanceAfterPayment')}: {formatAccountBalance(endingNativeBalance, 'USD')}
                            </Text>
                            <Text fontSize="sm" color={textColor}>
                              {t('purchases.payment.bookValueAfterPayment')}: {formatCurrency(endingBookBalance)}
                            </Text>
                            <Text
                              fontSize="sm"
                              color={
                                estimatedForexType === t('purchases.payment.forexLoss')
                                  ? 'red.600'
                                  : estimatedForexType === t('purchases.payment.forexGain')
                                    ? 'green.600'
                                    : 'gray.700'
                              }
                            >
                              {t('purchases.payment.estimatedForex')}: {formatCurrency(estimatedForexAmount)} ({estimatedForexType})
                            </Text>
                          </>
                        )}
                      </Box>
                    )}
                  </Box>
                )}

                <FormControl>
                  <FormLabel color={headingColor}>{t('purchases.payment.reference')}</FormLabel>
                  <Input
                    value={formData.reference}
                    onChange={handleChange('reference')}
                    placeholder={t('common.placeholders.referenceNumber')}
                    bg={inputBg}
                    color={textColor}
                    borderColor={inputBorderColor}
                    _hover={{ borderColor: inputHoverBorder }}
                    _placeholder={{ color: inputPlaceholderColor }}
                  />
                </FormControl>

                <FormControl>
                  <FormLabel color={headingColor}>{t('purchases.payment.notes')}</FormLabel>
                  <Textarea
                    value={formData.notes}
                    onChange={handleChange('notes')}
                    placeholder={t('common.placeholders.notes')}
                    rows={3}
                    bg={inputBg}
                    color={textColor}
                    borderColor={inputBorderColor}
                    _hover={{ borderColor: inputHoverBorder }}
                    _placeholder={{ color: inputPlaceholderColor }}
                  />
                </FormControl>
              </VStack>
            )}
          </ModalBody>

          <ModalFooter borderTopWidth="1px" borderColor={modalBorderColor}>
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
                  if ((selectedAccount.currency || 'IDR').toUpperCase() === 'USD' && (!formData.exchange_rate || formData.exchange_rate <= 0)) {
                    return true;
                  }
                  // Disable if zero balance or insufficient balance
                  if (getAccountBookBalance(selectedAccount) <= 0 || formData.amount > getAccountBookBalance(selectedAccount)) {
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
                  if ((selectedAccount.currency || 'IDR').toUpperCase() === 'USD' && (!formData.exchange_rate || formData.exchange_rate <= 0)) {
                    return t('purchases.payment.fillUsdRate');
                  }
                  if (getAccountBookBalance(selectedAccount) <= 0) {
                    return t('purchases.payment.insufficientBalance');
                  }
                  if (formData.amount > getAccountBookBalance(selectedAccount)) {
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
