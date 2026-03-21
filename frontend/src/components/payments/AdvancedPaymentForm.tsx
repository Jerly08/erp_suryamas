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
  VStack,
  HStack,
  FormControl,
  FormLabel,
  FormErrorMessage,
  Input,
  Select,
  Textarea,
  NumberInput,
  NumberInputField,
  NumberInputStepper,
  NumberIncrementStepper,
  NumberDecrementStepper,
  Box,
  Text,
  Divider,
  Badge,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  TableContainer,
  useToast,
  Spinner,
  Alert,
  AlertIcon,
  AlertDescription,
  Switch,
  Card,
  CardBody,
  SimpleGrid,
  Tab,
  Tabs,
  TabList,
  TabPanels,
  TabPanel,
  useColorModeValue,
  Tooltip,
  Icon,
} from '@chakra-ui/react';
import { useTranslation } from '@/hooks/useTranslation';
import { useForm, Controller, useWatch } from 'react-hook-form';

import { FiCreditCard, FiDollarSign, FiCalendar, FiUser, FiFileText, FiInfo } from 'react-icons/fi';
import paymentService, {
  PaymentCreateRequest,
  PaymentAllocation,
  BillAllocation,
  stripInternalPaymentReference,
} from '@/services/paymentService';
import { searchableSelectService } from '@/services/searchableSelectService';
import cashbankService, { CashBank, ReferenceExchangeRate, getBookBalanceInIDR } from '@/services/cashbankService';
import periodClosingService from '@/services/periodClosingService';
import AuthExpiredModal from '@/components/auth/AuthExpiredModal';
import CurrencyInput from '@/components/common/CurrencyInput';
import { useAuth } from '@/contexts/AuthContext';
import { normalizeRole } from '@/utils/roles';
import { roundUsdIdrRate } from '@/utils/exchangeRate';

interface AdvancedPaymentFormProps {
  isOpen: boolean;
  onClose: () => void;
  type: 'receivable' | 'payable';
  onSuccess?: () => void;
  preSelectedContact?: any;
  initialData?: any; // Payment object when editing
}

interface PaymentFormData {
  contact_id: number;
  cash_bank_id: number;
  date: string;
  amount: number;
  exchange_rate?: number;
  method: string;
  reference: string;
  notes: string;
  auto_allocate: boolean;
}

interface OutstandingItem {
  id: number;
  code: string;
  date: string;
  total_amount: number;
  outstanding_amount: number;
  currency?: string;
  exchange_rate?: number;
  due_date?: string;
}

const PAYMENT_METHODS = [
  { value: 'CASH', label: 'Cash', icon: FiDollarSign },
  { value: 'BANK_TRANSFER', label: 'Bank Transfer', icon: FiCreditCard },
  { value: 'CHECK', label: 'Check', icon: FiFileText },
  { value: 'CREDIT_CARD', label: 'Credit Card', icon: FiCreditCard },
  { value: 'DEBIT_CARD', label: 'Debit Card', icon: FiCreditCard },
  { value: 'OTHER', label: 'Other', icon: FiFileText },
];

// Tooltip descriptions for payment form
const PAYMENT_TOOLTIPS = {
  contact: 'Pilih customer (untuk pembayaran receivable) atau vendor (untuk pembayaran payable)',
  cashBank: 'Akun kas/bank yang akan digunakan untuk transaksi pembayaran',
  date: 'Tanggal transaksi pembayaran dilakukan',
  amount: 'Total jumlah pembayaran yang diterima atau dibayarkan',
  method: 'Metode pembayaran: Cash (tunai), Bank Transfer (transfer), Check (cek), Credit/Debit Card (kartu)',
  reference: 'Nomor referensi eksternal seperti nomor transfer, nomor cek, atau receipt number',
  notes: 'Catatan tambahan untuk pembayaran ini',
  autoAllocate: 'Aktifkan untuk mengalokasikan pembayaran secara otomatis ke invoice/bill tertua',
  allocation: 'Alokasi pembayaran ke invoice atau bill yang terkait. Pastikan total alokasi sesuai dengan jumlah pembayaran',
};

const formatCashBankBalance = (amount: number, currency?: string) => {
  const normalizedCurrency = (currency || 'IDR').toUpperCase();

  if (normalizedCurrency === 'USD') {
    return `USD ${(amount || 0).toLocaleString('en-US', {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    })}`;
  }

  return paymentService.formatCurrency(amount || 0, normalizedCurrency);
};

const getAvailableBookBalance = (account: CashBank) => {
  const nativeAvailable = Math.max(0, (account.balance || 0) - (account.frozen_amount || 0));
  const normalizedCurrency = (account.currency || 'IDR').toUpperCase();

  if (normalizedCurrency === 'IDR') {
    return nativeAvailable;
  }

  if ((account.balance || 0) <= 0) {
    return 0;
  }

  return getBookBalanceInIDR(account) * (nativeAvailable / account.balance);
};

const formatCashBankAvailability = (account: CashBank, nativeAmount: number) => {
  const nativeLabel = formatCashBankBalance(nativeAmount, account.currency);

  if ((account.currency || 'IDR').toUpperCase() === 'IDR') {
    return nativeLabel;
  }

  const proportionalBookAmount = (account.balance || 0) > 0
    ? getBookBalanceInIDR(account) * (nativeAmount / account.balance)
    : 0;

  return `${nativeLabel} | Nilai buku: ${paymentService.formatCurrency(proportionalBookAmount)}`;
};

const PaymentAllocationTable: React.FC<{
  items: OutstandingItem[];
  allocations: (PaymentAllocation | BillAllocation)[];
  onAllocationsChange: (allocations: any[]) => void;
  maxAmount: number;
  type: 'receivable' | 'payable';
}> = ({ items, allocations, onAllocationsChange, maxAmount, type }) => {
  const { t } = useTranslation();
  const [tempAllocations, setTempAllocations] = useState<Record<number, number>>({});

  // Color mode values
  const textColor = useColorModeValue('gray.600', 'gray.300');
  const mutedTextColor = useColorModeValue('gray.500', 'gray.400');
  const scrollbarTrackColor = useColorModeValue('#f1f1f1', '#2d3748');
  const scrollbarThumbColor = useColorModeValue('#c1c1c1', '#4a5568');
  const scrollbarThumbHoverColor = useColorModeValue('#a8a8a8', '#718096');

  useEffect(() => {
    // Initialize temp allocations from props
    const temp: Record<number, number> = {};
    allocations.forEach(allocation => {
      const id = type === 'receivable'
        ? (allocation as PaymentAllocation).invoice_id
        : (allocation as BillAllocation).bill_id;
      temp[id] = allocation.amount;
    });
    setTempAllocations(temp);
  }, [allocations, type]);

  const updateAllocation = (itemId: number, amount: number) => {
    const newTemp = { ...tempAllocations };
    if (amount > 0) {
      newTemp[itemId] = amount;
    } else {
      delete newTemp[itemId];
    }
    setTempAllocations(newTemp);

    // Convert to proper allocation format
    const newAllocations = Object.entries(newTemp).map(([id, amt]) => {
      const numId = parseInt(id);
      return type === 'receivable'
        ? { invoice_id: numId, amount: amt } as PaymentAllocation
        : { bill_id: numId, amount: amt } as BillAllocation;
    });

    onAllocationsChange(newAllocations);
  };

  const getTotalAllocated = () => {
    return Object.values(tempAllocations).reduce((sum, amount) => sum + amount, 0);
  };

  const getItemAllocation = (itemId: number) => {
    return tempAllocations[itemId] || 0;
  };

  const getItemRemaining = (item: OutstandingItem) => {
    const allocated = getItemAllocation(item.id);
    return item.outstanding_amount - allocated;
  };

  const autoAllocatePayment = () => {
    const remainingAmount = maxAmount - getTotalAllocated();
    if (remainingAmount <= 0) return;

    let amountToAllocate = remainingAmount;
    const newTemp = { ...tempAllocations };

    // Sort items by due date (oldest first) or by amount (smallest first)
    const sortedItems = [...items].sort((a, b) => {
      if (a.due_date && b.due_date) {
        return new Date(a.due_date).getTime() - new Date(b.due_date).getTime();
      }
      return a.outstanding_amount - b.outstanding_amount;
    });

    for (const item of sortedItems) {
      if (amountToAllocate <= 0) break;

      const currentAllocation = newTemp[item.id] || 0;
      const maxItemAllocation = item.outstanding_amount - currentAllocation;
      const allocationAmount = Math.min(amountToAllocate, maxItemAllocation);

      if (allocationAmount > 0) {
        newTemp[item.id] = currentAllocation + allocationAmount;
        amountToAllocate -= allocationAmount;
      }
    }

    setTempAllocations(newTemp);

    // Update parent
    const newAllocations = Object.entries(newTemp).map(([id, amt]) => {
      const numId = parseInt(id);
      return type === 'receivable'
        ? { invoice_id: numId, amount: amt } as PaymentAllocation
        : { bill_id: numId, amount: amt } as BillAllocation;
    });

    onAllocationsChange(newAllocations);
  };

  return (
    <Box>
      <HStack justify="space-between" mb={4}>
        <Text fontWeight="bold">{t('payments.form.allocation')}</Text>
        <HStack>
          <Text fontSize="sm" color={textColor}>
            {t('payments.form.allocationTable.total')}: {paymentService.formatCurrency(getTotalAllocated())} / {paymentService.formatCurrency(maxAmount)}
          </Text>
          <Button size="xs" onClick={autoAllocatePayment} variant="outline">
            {t('payments.form.autoAllocate')}
          </Button>
        </HStack>
      </HStack>

      <TableContainer
        overflowY="auto"
        maxH="400px"
        css={{
          // Enhanced mouse wheel scroll for table
          scrollBehavior: 'smooth',
          willChange: 'scroll-position',
          transform: 'translateZ(0)',
          '&::-webkit-scrollbar': {
            width: '6px',
          },
          '&::-webkit-scrollbar-track': {
            background: scrollbarTrackColor,
            borderRadius: '3px',
          },
          '&::-webkit-scrollbar-thumb': {
            background: scrollbarThumbColor,
            borderRadius: '3px',
            '&:hover': {
              background: scrollbarThumbHoverColor,
            },
          },
        }}
      >
        <Table size="sm">
          <Thead>
            <Tr>
              <Th>{t('payments.form.allocationTable.document')}</Th>
              <Th>{t('payments.form.allocationTable.date')}</Th>
              <Th>{t('payments.form.allocationTable.total')}</Th>
              <Th>{t('payments.form.allocationTable.outstanding')}</Th>
              <Th>{t('payments.form.allocationTable.allocate')}</Th>
              <Th>{t('payments.form.allocationTable.remaining')}</Th>
            </Tr>
          </Thead>
          <Tbody>
            {items.map((item) => (
              <Tr key={item.id}>
                <Td>
                  <Text fontWeight="medium">{item.code}</Text>
                  {item.due_date && (
                    <Text fontSize="xs" color={mutedTextColor}>
                      Due: {paymentService.formatDate(item.due_date)}
                    </Text>
                  )}
                </Td>
                <Td>{paymentService.formatDate(item.date)}</Td>
                <Td>{paymentService.formatCurrency(item.total_amount)}</Td>
                <Td>{paymentService.formatCurrency(item.outstanding_amount)}</Td>
                <Td>
                  <Box width="140px">
                    <CurrencyInput
                      value={getItemAllocation(item.id)}
                      onChange={(value) => updateAllocation(item.id, value)}
                      placeholder="0"
                      size="sm"
                      min={0}
                      max={item.outstanding_amount}
                      showLabel={false}
                    />
                  </Box>
                </Td>
                <Td>
                  <Text color={getItemRemaining(item) === 0 ? 'green.600' : 'orange.600'}>
                    {paymentService.formatCurrency(getItemRemaining(item))}
                  </Text>
                </Td>
              </Tr>
            ))}
          </Tbody>
        </Table>
      </TableContainer>

      {items.length === 0 && (
        <Text textAlign="center" py={4} color={mutedTextColor}>
          No outstanding {type === 'receivable' ? 'invoices' : 'bills'} found for this contact
        </Text>
      )}
    </Box>
  );
};

const AdvancedPaymentForm: React.FC<AdvancedPaymentFormProps> = ({
  isOpen,
  onClose,
  type,
  onSuccess,
  preSelectedContact,
  initialData,
}) => {
  const [contacts, setContacts] = useState<any[]>([]);
  const [cashBankAccounts, setCashBankAccounts] = useState<CashBank[]>([]);
  const [outstandingItems, setOutstandingItems] = useState<OutstandingItem[]>([]);
  const [allocations, setAllocations] = useState<(PaymentAllocation | BillAllocation)[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadingItems, setLoadingItems] = useState(false);
  const [activeTab, setActiveTab] = useState(0);
  const [showAuthExpired, setShowAuthExpired] = useState(false);
  const [referenceRate, setReferenceRate] = useState<ReferenceExchangeRate | null>(null);
  const [exchangeRateEdited, setExchangeRateEdited] = useState(false);
  const { user } = useAuth();

  // Color mode values
  const modalContentBg = useColorModeValue('white', 'gray.800');
  const modalHeaderBg = useColorModeValue('blue.500', 'blue.600');
  const modalFooterBg = useColorModeValue('gray.50', 'gray.700');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const textColor = useColorModeValue('gray.600', 'gray.300');
  const scrollbarTrackColor = useColorModeValue('#f1f1f1', '#2d3748');
  const scrollbarThumbColor = useColorModeValue('#c1c1c1', '#4a5568');
  const scrollbarThumbHoverColor = useColorModeValue('#a8a8a8', '#718096');

  // Debug effect to track modal state changes
  useEffect(() => {
    console.log('AdvancedPaymentForm - showAuthExpired state changed:', showAuthExpired);
  }, [showAuthExpired]);

  const toast = useToast();

  // Check if user has permission to create payments directly vs draft
  const userRole = normalizeRole(user?.role);
  const isFinanceOrAdmin = userRole === 'admin' || userRole === 'finance' || userRole === 'director';

  // If modal is opened but user doesn't have at least 'employee' role, close it
  useEffect(() => {
    if (isOpen && user && !userRole) {
      toast({
        title: 'Access Denied',
        description: 'You do not have permission to access this module.',
        status: 'error',
        duration: 5000,
      });
      onClose();
    }
  }, [isOpen, user, userRole, toast, onClose]);

  const { t } = useTranslation();

  const {
    control,
    register,
    handleSubmit,
    reset,
    setValue,
    formState: { errors, isSubmitting },
    watch,
  } = useForm<PaymentFormData>({
    defaultValues: {
      date: new Date().toISOString().split('T')[0],
      method: 'BANK_TRANSFER',
      auto_allocate: false,
      exchange_rate: undefined,
    }
  });

  const watchedContactId = useWatch({ control, name: 'contact_id' });
  const watchedAmount = useWatch({ control, name: 'amount' });
  const watchedMethod = useWatch({ control, name: 'method' });
  const watchedAutoAllocate = useWatch({ control, name: 'auto_allocate' });
  const watchedCashBankId = useWatch({ control, name: 'cash_bank_id' });
  const watchedExchangeRate = useWatch({ control, name: 'exchange_rate' });
  const selectedCashBank = cashBankAccounts.find(account => account.id === watchedCashBankId);
  const selectedCashBankCurrency = (selectedCashBank?.currency || 'IDR').toUpperCase();
  const isUsdPayableAccount = type === 'payable' && selectedCashBankCurrency === 'USD';
  const selectedAllocationItems = outstandingItems.filter(item => allocations.some(allocation => {
    if (type === 'receivable') {
      return (allocation as PaymentAllocation).invoice_id === item.id;
    }
    return (allocation as BillAllocation).bill_id === item.id;
  }));
  const hasUsdReceivableSelection = type === 'receivable' && selectedAllocationItems.some(item => (item.currency || 'IDR').toUpperCase() === 'USD');
  const requiresFxRate = type === 'payable'
    ? selectedCashBankCurrency === 'USD'
    : selectedCashBankCurrency === 'USD' || hasUsdReceivableSelection;
  const shouldHideForm = !userRole && !!user;

  // Load initial data and force refresh on modal open
  useEffect(() => {
    if (isOpen) {
      console.log('AdvancedPaymentForm - Modal opened');
      loadContacts();
      loadCashBankAccounts();

      // Clear previous data first
      setOutstandingItems([]);
      setAllocations([]);

      if (initialData) {
        // Editing an existing payment: populate form fields
        console.log('AdvancedPaymentForm - Populating form with initialData:', initialData.id);
        reset({
          contact_id: initialData.contact_id || 0,
          cash_bank_id: initialData.cash_bank_id || 0,
          date: initialData.date?.split('T')[0] || new Date().toISOString().split('T')[0],
          amount: initialData.amount || 0,
          exchange_rate: initialData.exchange_rate ? roundUsdIdrRate(initialData.exchange_rate) : undefined,
          method: initialData.method || 'BANK_TRANSFER',
          reference: stripInternalPaymentReference(initialData.reference),
          notes: initialData.notes || '',
          auto_allocate: false,
        });
        // Load outstanding items for this contact
        if (initialData.contact_id) {
          setTimeout(() => loadOutstandingItems(initialData.contact_id), 0);
        }
      } else if (preSelectedContact) {
        console.log('AdvancedPaymentForm - Setting preselected contact:', preSelectedContact.id, preSelectedContact.name);
        setValue('contact_id', preSelectedContact.id);
        setTimeout(() => {
          console.log('AdvancedPaymentForm - Loading outstanding items for preselected contact:', preSelectedContact.id);
          loadOutstandingItems(preSelectedContact.id);
        }, 0);
      }
    } else {
      // Reset state when modal closes
      console.log('AdvancedPaymentForm - Modal closed, clearing state');
      setOutstandingItems([]);
      setAllocations([]);
      reset(); // Reset form values
    }
  }, [isOpen, preSelectedContact?.id, initialData?.id, setValue, reset]);

  // Load outstanding items when contact changes (user selection)
  useEffect(() => {
    if (watchedContactId && watchedContactId > 0 && isOpen) {
      console.log('AdvancedPaymentForm - Contact changed, loading outstanding items:', watchedContactId);
      loadOutstandingItems(watchedContactId);
    }
  }, [watchedContactId, isOpen]);

  useEffect(() => {
    if (!isOpen || !requiresFxRate) {
      setReferenceRate(null);
      setExchangeRateEdited(false);
      setValue('exchange_rate', undefined);
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
        setValue('exchange_rate', roundUsdIdrRate(rate.exchange_rate), { shouldValidate: true });
      }
    };

    loadReferenceRate();
    return () => {
      cancelled = true;
    };
  }, [isOpen, requiresFxRate, watchedCashBankId, exchangeRateEdited, setValue]);

  // Auto allocate when amount changes (always enabled)
  useEffect(() => {
    if (watchedAmount > 0 && outstandingItems.length > 0) {
      autoAllocatePayment();
    } else if (!watchedAmount || watchedAmount === 0) {
      // Clear allocations when amount is 0
      setAllocations([]);
    }
  }, [watchedAmount, outstandingItems]);

  const loadContacts = async () => {
    try {
      const contactData = await searchableSelectService.getContacts({
        type: type === 'receivable' ? 'CUSTOMER' : 'VENDOR'
      });
      setContacts(contactData);
    } catch (error) {
      console.error('Error loading contacts:', error);
    }
  };

  const loadCashBankAccounts = async () => {
    try {
      const accounts = await cashbankService.getPaymentAccounts();
      setCashBankAccounts(accounts);
    } catch (error) {
      console.error('Error loading cash bank accounts:', error);
    }
  };

  const loadOutstandingItems = async (contactId: number) => {
    console.log(`AdvancedPaymentForm - loadOutstandingItems called for contactId: ${contactId}, type: ${type}`);
    try {
      setLoadingItems(true);

      let items: any[] = [];
      if (type === 'receivable') {
        console.log('AdvancedPaymentForm - Fetching unpaid invoices...');
        items = await paymentService.getUnpaidInvoices(contactId);
        console.log('AdvancedPaymentForm - Received invoices:', items.length, items);
        setOutstandingItems(items);
      } else {
        console.log('AdvancedPaymentForm - Fetching unpaid bills...');
        items = await paymentService.getUnpaidBills(contactId);
        console.log('AdvancedPaymentForm - Received bills:', items.length, items);
        setOutstandingItems(items);
      }

      // Log the outstanding amounts
      items.forEach((item: any) => {
        console.log(`  - ${item.code}: Outstanding = ${item.outstanding_amount}`);
      });
    } catch (error) {
      console.error('AdvancedPaymentForm - Error loading outstanding items:', error);
      toast({
        title: 'Error',
        description: 'Failed to load outstanding items',
        status: 'error',
        duration: 3000,
      });
    } finally {
      setLoadingItems(false);
    }
  };

  const formatExchangeRate = (value?: number) => {
    if (!value || value <= 0) {
      return '-';
    }

    return `Rp ${value.toLocaleString('id-ID', {
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    })}`;
  };

  const getUsdNativeOutAmount = () => {
    if (!isUsdPayableAccount || !watchedAmount || !watchedExchangeRate || watchedExchangeRate <= 0) {
      return 0;
    }

    return watchedAmount / watchedExchangeRate;
  };

  const getReceivableHistoricalSettlementBookAmount = () => {
    if (type !== 'receivable' || !watchedAmount) {
      return watchedAmount || 0;
    }
    if (!hasUsdReceivableSelection || !watchedExchangeRate || watchedExchangeRate <= 0) {
      return watchedAmount || 0;
    }

    let totalSettlementBook = 0;
    let totalAllocated = 0;

    allocations.forEach((allocation) => {
      const invoiceId = (allocation as PaymentAllocation).invoice_id;
      const allocatedAmount = allocation.amount || 0;
      const item = outstandingItems.find(outstanding => outstanding.id === invoiceId);
      totalAllocated += allocatedAmount;

      if (!item || (item.currency || 'IDR').toUpperCase() !== 'USD') {
        totalSettlementBook += allocatedAmount;
        return;
      }

      const historicalRate = item.exchange_rate && item.exchange_rate > 0 ? item.exchange_rate : 1;
      totalSettlementBook += (allocatedAmount / watchedExchangeRate) * historicalRate;
    });

    const unappliedAmount = Math.max(0, (watchedAmount || 0) - totalAllocated);
    return totalSettlementBook + unappliedAmount;
  };

  const usdNativeOutAmount = getUsdNativeOutAmount();
  const receivableNativeAmount = requiresFxRate && watchedAmount && watchedExchangeRate && watchedExchangeRate > 0
    ? watchedAmount / watchedExchangeRate
    : 0;
  const availableNativeBalance = selectedCashBank
    ? Math.max(0, (selectedCashBank.balance || 0) - (selectedCashBank.frozen_amount || 0))
    : 0;
  const availableBookBalance = selectedCashBank ? getAvailableBookBalance(selectedCashBank) : 0;
  const currentCarryRate = isUsdPayableAccount && availableNativeBalance > 0
    ? availableBookBalance / availableNativeBalance
    : 0;
  const bankBookOutAmount = isUsdPayableAccount && availableNativeBalance > 0
    ? availableBookBalance * (usdNativeOutAmount / availableNativeBalance)
    : watchedAmount || 0;
  const receivableSettlementBookAmount = getReceivableHistoricalSettlementBookAmount();
  const endingNativeBalance = Math.max(0, availableNativeBalance - usdNativeOutAmount);
  const endingBookBalance = Math.max(0, availableBookBalance - bankBookOutAmount);
  const estimatedForexDiff = type === 'payable'
    ? bankBookOutAmount - (watchedAmount || 0)
    : (watchedAmount || 0) - receivableSettlementBookAmount;
  const estimatedForexType =
    estimatedForexDiff > 0.005
      ? type === 'payable'
        ? 'RUGI KURS'
        : 'LABA KURS'
      : estimatedForexDiff < -0.005
        ? type === 'payable'
          ? 'LABA KURS'
          : 'RUGI KURS'
        : 'NETRAL';
  const estimatedForexAmount = Math.abs(estimatedForexDiff);

  const autoAllocatePayment = () => {
    if (!watchedAmount || outstandingItems.length === 0) return;

    let remainingAmount = watchedAmount;
    const newAllocations: (PaymentAllocation | BillAllocation)[] = [];

    // Sort by due date or amount
    const sortedItems = [...outstandingItems].sort((a, b) => {
      if (a.due_date && b.due_date) {
        return new Date(a.due_date).getTime() - new Date(b.due_date).getTime();
      }
      return a.outstanding_amount - b.outstanding_amount;
    });

    for (const item of sortedItems) {
      if (remainingAmount <= 0) break;

      const allocationAmount = Math.min(remainingAmount, item.outstanding_amount);

      if (type === 'receivable') {
        newAllocations.push({
          invoice_id: item.id,
          amount: allocationAmount,
        } as PaymentAllocation);
      } else {
        newAllocations.push({
          bill_id: item.id,
          amount: allocationAmount,
        } as BillAllocation);
      }

      remainingAmount -= allocationAmount;
    }

    setAllocations(newAllocations);
  };

  // Keep amount in sync with allocations for receivable payments
  useEffect(() => {
    if (type !== 'receivable') return;
    const total = allocations.reduce((sum, a) => sum + a.amount, 0);
    // If user hasn't set amount or amount is less than allocated, auto-adjust
    if ((watchedAmount || 0) < total) {
      setValue('amount', total);
    }
  }, [allocations]);

  const onSubmit = async (data: PaymentFormData) => {
    try {
      // ===== Period closing validation =====
      // Cegah pembuatan payment pada tanggal yang sudah ditutup buku.
      if (data.date) {
        const isClosed = await periodClosingService.isDateInClosedPeriod(data.date);
        if (isClosed) {
          toast({
            title: 'Periode Akuntansi Sudah Ditutup',
            description:
              'Tidak dapat membuat pembayaran pada tanggal ini karena periode akuntansi sudah ditutup. Gunakan tanggal di periode yang masih terbuka.',
            status: 'error',
            duration: 8000,
            isClosable: true,
          });
          return;
        }
      }

      setLoading(true);

      if (requiresFxRate && (!data.exchange_rate || data.exchange_rate <= 0)) {
        toast({
          title: 'Kurs transaksi wajib diisi',
          description: type === 'receivable'
            ? 'Untuk pelunasan piutang multi-currency, isi kurs settlement IDR per USD agar realized FX dihitung dari kurs historis invoice ke kurs pembayaran saat ini.'
            : 'Untuk pembayaran dari rekening USD, isi kurs IDR per USD agar native balance dan jurnal buku dihitung dengan benar.',
          status: 'error',
          duration: 6000,
          isClosable: true,
        });
        return;
      }

      const paymentData: PaymentCreateRequest = {
        contact_id: data.contact_id,
        cash_bank_id: data.cash_bank_id,
        date: data.date,
        amount: data.amount,
        exchange_rate: requiresFxRate ? roundUsdIdrRate(data.exchange_rate) || undefined : undefined,
        method: data.method,
        reference: data.reference || '',
        notes: data.notes || '',
      };

      // Add allocations
      if (type === 'receivable') {
        paymentData.allocations = allocations as PaymentAllocation[];
      } else {
        paymentData.bill_allocations = allocations as BillAllocation[];
      }

      // Balance validation
      // For receivable (incoming) payments, we do NOT block by current account balance
      // For payable (outgoing) payments, ensure sufficient balance
      if (type === 'payable') {
        const selectedAccount = cashBankAccounts.find(account => account.id === data.cash_bank_id);
        if (selectedAccount) {
          const availableBookBalance = getAvailableBookBalance(selectedAccount);
          const availableNativeBalance = Math.max(0, selectedAccount.balance - (selectedAccount.frozen_amount || 0));
          const isUsdAccount = (selectedAccount.currency || 'IDR').toUpperCase() === 'USD';

          if (isUsdAccount && (!data.exchange_rate || data.exchange_rate <= 0)) {
            toast({
              title: 'Kurs transaksi wajib diisi',
              description: 'Untuk pembayaran dari rekening USD, isi kurs IDR per USD agar native balance dan jurnal buku dihitung dengan benar.',
              status: 'error',
              duration: 6000,
              isClosable: true,
            });
            return;
          }

          if (availableBookBalance <= 0) {
            toast({
              title: 'Insufficient Balance ⚠️',
              description: `Cannot process payment. The selected account "${selectedAccount.name}" has zero or negative balance.`,
              status: 'error',
              duration: 8000,
              isClosable: true,
            });
            return;
          }

          if (data.amount > availableBookBalance) {
            toast({
              title: 'Insufficient Balance ⚠️',
              description: (
                `Payment amount ${paymentService.formatCurrency(data.amount)} exceeds available balance ` +
                `${formatCashBankAvailability(selectedAccount, availableNativeBalance)} in account "${selectedAccount.name}". ` +
                `Please reduce the payment amount or select a different account.`
              ),
              status: 'error',
              duration: 10000,
              isClosable: true,
            });
            return;
          }
        }
      }

      // Validate allocations don't exceed payment amount
      const totalAllocated = allocations.reduce((sum, allocation) => sum + allocation.amount, 0);
      if (totalAllocated > data.amount) {
        toast({
          title: 'Validation Error',
          description: 'Allocated amount cannot exceed payment amount',
          status: 'error',
          duration: 5000,
        });
        return;
      }

      // Create or update payment
      let result;
      if (initialData?.id) {
        // Edit mode - update existing draft
        result = await paymentService.updateDraftPayment(initialData.id, paymentData);
      } else {
        // Create mode
        if (type === 'receivable') {
          result = await paymentService.createReceivablePayment(paymentData);
        } else {
          result = await paymentService.createPayablePayment(paymentData);
        }
      }

      toast({
        title: 'Success',
        description: `${type === 'receivable' ? 'Receivable' : 'Payable'} payment created successfully`,
        status: 'success',
        duration: 3000,
      });

      reset();
      setAllocations([]);
      setOutstandingItems([]);
      onClose();
      onSuccess?.();

    } catch (error: any) {
      console.error('AdvancedPaymentForm - Error creating payment:', error);
      console.log('AdvancedPaymentForm - Error details:', {
        isAuthError: error.isAuthError,
        code: error.code,
        message: error.message,
        name: error.name,
        stack: error.stack?.substring(0, 200)
      });

      let errorMessage = error.message || 'Failed to create payment';

      // Handle specific error types - Check both error object and message
      const isAuthError = error.isAuthError ||
        error.code === 'AUTH_SESSION_EXPIRED' ||
        errorMessage.includes('Session expired') ||
        errorMessage.includes('User not authenticated');

      console.log('AdvancedPaymentForm - Auth error check:', { isAuthError, errorMessage });

      if (isAuthError) {
        console.log('AdvancedPaymentForm - Detected auth error, showing auth expired modal');

        // Show auth expired modal first (before closing payment modal)
        setShowAuthExpired(true);

        // Close payment modal after a small delay to ensure auth modal renders
        setTimeout(() => {
          handleClose();
        }, 100);

        return;
      }

      toast({
        title: 'Error',
        description: errorMessage,
        status: 'error',
        duration: 5000,
      });
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    reset();
    setAllocations([]);
    setOutstandingItems([]);
    onClose();
  };

  const getTotalAllocated = () => {
    return allocations.reduce((sum, allocation) => sum + allocation.amount, 0);
  };

  const getRemainingAmount = () => {
    return (watchedAmount || 0) - getTotalAllocated();
  };

  const handleAuthExpiredLoginRedirect = () => {
    // Clear all auth data
    if (typeof window !== 'undefined') {
      localStorage.removeItem('token');
      localStorage.removeItem('refreshToken');
      localStorage.removeItem('user');
    }

    // Redirect to login
    window.location.href = '/login';
  };

  if (shouldHideForm) {
    return null;
  }

  return (
    <>
      {/* Auth Expired Modal */}
      <AuthExpiredModal
        isOpen={showAuthExpired}
        onLoginRedirect={handleAuthExpiredLoginRedirect}
      />

      <Modal
        isOpen={isOpen}
        onClose={handleClose}
        size="6xl"
        scrollBehavior="inside"
      >
        <ModalOverlay bg="blackAlpha.600" backdropFilter="blur(10px)" />
        <ModalContent
          maxH="95vh"
          maxW={{ base: '95vw', md: '90vw', lg: '80vw' }}
          mx={4}
          bg={modalContentBg}
        >
          <ModalHeader
            bg={modalHeaderBg}
            color="white"
            borderTopRadius="md"
            py={4}
          >
            {type === 'receivable' ? t('payments.form.createReceivablePayment') : t('payments.form.createPayablePayment')}
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
                // Enhanced mouse wheel scroll
                scrollBehavior: 'smooth',
                // Better scroll performance for GPU acceleration
                willChange: 'scroll-position',
                transform: 'translateZ(0)',
                // Better touch scrolling on mobile
                WebkitOverflowScrolling: 'touch',
              }}
            >
              <Tabs index={activeTab} onChange={setActiveTab}>
                <TabList>
                  <Tab>{t('payments.form.paymentDetails')}</Tab>
                  <Tab isDisabled={!watchedContactId}>
                    {t('payments.form.allocation')} ({outstandingItems.length})
                  </Tab>
                </TabList>

                <TabPanels>
                  {/* Payment Details Tab */}
                  <TabPanel px={0}>
                    <VStack spacing={6} align="stretch">
                      {/* Draft warning for non-finance roles */}
                      {!isFinanceOrAdmin && (
                        <Alert status="warning" borderRadius="md">
                          <AlertIcon />
                          <AlertDescription fontSize="sm">
                            Pembayaran ini akan disimpan sebagai <strong>DRAFT</strong> dan perlu disetujui oleh Finance sebelum jurnal dibuat.
                          </AlertDescription>
                        </Alert>
                      )}

                      {/* Contact Selection */}
                      <Card>
                        <CardBody>
                          <FormControl isRequired isInvalid={!!errors.contact_id}>
                            <FormLabel>
                              <HStack>
                                <FiUser />
                                <Text>{t('payments.form.contact')}</Text>
                              </HStack>
                            </FormLabel>
                            <Select
                              {...register('contact_id', {
                                required: t('payments.form.contactRequired'),
                                setValueAs: (value: string) => parseInt(value) || 0,
                              })}
                              placeholder={t('payments.form.selectContact')}
                            >
                              {contacts.map((contact) => (
                                <option key={contact.id} value={contact.id}>
                                  {contact.name}
                                </option>
                              ))}
                            </Select>
                            <FormErrorMessage>{errors.contact_id?.message}</FormErrorMessage>
                          </FormControl>
                        </CardBody>
                      </Card>

                      {/* Payment Information */}
                      <Card>
                        <CardBody>
                          <SimpleGrid columns={{ base: 1, md: 2 }} spacing={4}>
                            <FormControl isRequired isInvalid={!!errors.date}>
                              <FormLabel>
                                <HStack>
                                  <FiCalendar />
                                  <Text>{t('payments.form.paymentDate')}</Text>
                                </HStack>
                              </FormLabel>
                              <Input
                                type="date"
                                {...register('date', {
                                  required: t('payments.form.dateRequired'),
                                  validate: {
                                    notFuture: (value: string) => {
                                      const today = new Date();
                                      const inputDate = new Date(value);
                                      return inputDate <= today || t('payments.form.dateNotFuture');
                                    },
                                  },
                                })}
                              />
                              <FormErrorMessage>{errors.date?.message}</FormErrorMessage>
                            </FormControl>

                            <FormControl isRequired isInvalid={!!errors.amount}>
                              <Controller
                                name="amount"
                                control={control}
                                rules={{
                                  required: 'Amount is required',
                                  min: { value: 1, message: 'Amount must be greater than 0' },
                                }}
                                render={({ field }) => (
                                  <CurrencyInput
                                    value={field.value || 0}
                                    onChange={field.onChange}
                                    label="Amount"
                                    placeholder="Masukkan jumlah pembayaran"
                                    isRequired={true}
                                    isInvalid={!!errors.amount}
                                    errorMessage={errors.amount?.message}
                                    min={1}
                                    showLabel={true}
                                  />
                                )}
                              />
                            </FormControl>

                            <FormControl isRequired isInvalid={!!errors.method}>
                              <FormLabel>{t('payments.form.paymentMethod')}</FormLabel>
                              <Select
                                {...register('method', {
                                  required: t('payments.form.methodRequired'),
                                })}
                              >
                                {PAYMENT_METHODS.map((method) => (
                                  <option key={method.value} value={method.value}>
                                    {method.label}
                                  </option>
                                ))}
                              </Select>
                              <FormErrorMessage>{errors.method?.message}</FormErrorMessage>
                            </FormControl>

                            <FormControl isRequired isInvalid={!!errors.cash_bank_id}>
                              <FormLabel>
                                {watchedMethod === 'CASH' ? t('payments.form.cashAccount') : t('payments.form.paymentAccount')}
                              </FormLabel>
                              <Select
                                {...register('cash_bank_id', {
                                  required: t('payments.form.accountRequired'),
                                  setValueAs: (value: string) => parseInt(value) || 0,
                                })}
                                placeholder={t('payments.form.selectAccount')}
                              >
                                {cashBankAccounts
                                  .filter((account) => {
                                    // Filter based on selected payment method
                                    if (watchedMethod === 'CASH') {
                                      return account.type === 'CASH';
                                    } else {
                                      // For other methods, show BANK accounts or all if no CASH type
                                      return account.type === 'BANK' || account.type !== 'CASH';
                                    }
                                  })
                                  .map((account) => {
                                    const currentAmount = watchedAmount || 0;
                                    const frozen = account.frozen_amount || 0;
                                    const availableBalance = Math.max(0, account.balance - frozen);
                                    const availableBookBalance = getAvailableBookBalance(account);
                                    const isInsufficientBalance = availableBookBalance < currentAmount;
                                    const isZeroBalance = availableBalance <= 0;
                                    const balanceStatus = isZeroBalance ? ' ❌ NO BALANCE' :
                                      isInsufficientBalance ? ' ⚠️ INSUFFICIENT' :
                                        ' ✅';

                                    return (
                                      <option key={account.id} value={account.id}
                                        style={{
                                          color: isZeroBalance ? '#e53e3e' :
                                            isInsufficientBalance ? '#dd6b20' :
                                              '#38a169',
                                          backgroundColor: isZeroBalance ? '#fed7d7' :
                                            isInsufficientBalance ? '#feebc8' :
                                              '#f0fff4'
                                        }}
                                      >
                                        {account.type === 'BANK' && account.bank_name
                                          ? `${account.code} - ${account.name} (${account.bank_name}) - Tersedia: ${formatCashBankAvailability(account, availableBalance)}`
                                          : `${account.code} - ${account.name} (${account.type}) - Tersedia: ${formatCashBankAvailability(account, availableBalance)}`
                                        }{frozen > 0 ? ` [Terblokir: ${formatCashBankBalance(frozen, account.currency)}]` : ''}{balanceStatus}
                                      </option>
                                    );
                                  })}
                                {/* Fallback: if no filtered accounts, show all */}
                                {cashBankAccounts.filter((account) => {
                                  if (watchedMethod === 'CASH') {
                                    return account.type === 'CASH';
                                  } else {
                                    return account.type === 'BANK' || account.type !== 'CASH';
                                  }
                                }).length === 0 && cashBankAccounts.map((account) => (
                                  <option key={account.id} value={account.id}>
                                    {account.type === 'BANK' && account.bank_name
                                      ? `${account.code} - ${account.name} (${account.bank_name})`
                                      : `${account.code} - ${account.name} (${account.type})`
                                    }
                                  </option>
                                ))}
                              </Select>
                              <FormErrorMessage>{errors.cash_bank_id?.message}</FormErrorMessage>
                              {/* Real-time Balance Warning */}
                              {watchedCashBankId && (() => {
                                const selectedAccount = cashBankAccounts.find(account => account.id === watchedCashBankId);
                                if (!selectedAccount) return null;

                                const frozen = selectedAccount.frozen_amount || 0;
                                const availableBalance = Math.max(0, selectedAccount.balance - frozen);
                                const availableBookBalance = getAvailableBookBalance(selectedAccount);
                                const isZeroBalance = availableBalance <= 0;
                                const isInsufficientBalance = watchedAmount > availableBookBalance;
                                const remainingBalance = availableBookBalance - (watchedAmount || 0);
                                const isForeignCurrency = (selectedAccount.currency || 'IDR').toUpperCase() !== 'IDR';

                                if (isZeroBalance) {
                                  return (
                                    <Text fontSize="sm" color="red.500" mt={2} fontWeight="medium">
                                      <strong>No Balance Available</strong><br />
                                      Account {selectedAccount.name} has no available balance.{frozen > 0 ? ` (Total: ${formatCashBankBalance(selectedAccount.balance, selectedAccount.currency)}, Terblokir: ${formatCashBankBalance(frozen, selectedAccount.currency)})` : ' Cannot process any payments.'}
                                    </Text>
                                  );
                                }

                                if (isInsufficientBalance && watchedAmount > 0) {
                                  return (
                                    <Text fontSize="sm" color="orange.500" mt={2} fontWeight="medium">
                                      <strong>Insufficient Balance</strong><br />
                                      Tersedia: {formatCashBankAvailability(selectedAccount, availableBalance)} | Dibutuhkan: {paymentService.formatCurrency(watchedAmount)} |
                                      Kurang: {paymentService.formatCurrency(watchedAmount - availableBookBalance)}{frozen > 0 ? ` | Terblokir: ${formatCashBankBalance(frozen, selectedAccount.currency)}` : ''}
                                    </Text>
                                  );
                                }

                                if (watchedAmount > 0 && remainingBalance >= 0) {
                                  return (
                                    <Text fontSize="sm" color="green.600" mt={2}>
                                      ✅ <strong>Sufficient Balance</strong><br />
                                      {isForeignCurrency
                                        ? `Saldo native: ${formatCashBankBalance(selectedAccount.balance, selectedAccount.currency)} | Sisa nilai buku: ${paymentService.formatCurrency(remainingBalance)}`
                                        : `Available: ${paymentService.formatCurrency(selectedAccount.balance)} | After payment: ${paymentService.formatCurrency(remainingBalance)}`}
                                    </Text>
                                  );
                                }

                                return (
                                  <Text fontSize="sm" color="blue.600" mt={2}>
                                    <strong>Saldo Tersedia:</strong> {formatCashBankAvailability(selectedAccount, availableBalance)}{frozen > 0 ? ` (Total: ${formatCashBankBalance(selectedAccount.balance, selectedAccount.currency)}, Terblokir: ${formatCashBankBalance(frozen, selectedAccount.currency)})` : ''}
                                  </Text>
                                );
                              })()}

                              {/* Show helpful message about filtered accounts */}
                              {watchedMethod === 'CASH' &&
                                cashBankAccounts.filter(acc => acc.type === 'CASH').length === 0 && (
                                  <Text fontSize="xs" color="orange.500" mt={1}>
                                    ⚠️ No cash accounts found. Showing all available accounts.
                                  </Text>
                                )}
                            </FormControl>
                          </SimpleGrid>

                          {requiresFxRate && (
                            <Box mt={4} borderWidth="1px" borderRadius="md" p={4} bg="blue.50">
                              <Input type="hidden" {...register('exchange_rate', { valueAsNumber: true })} />
                              <FormControl isRequired>
                                <FormLabel>Kurs transaksi (IDR per USD)</FormLabel>
                                <Input
                                  type="number"
                                  step="1"
                                  min="0"
                                  value={watchedExchangeRate || ''}
                                  onChange={(event) => {
                                    setExchangeRateEdited(true);
                                    setValue('exchange_rate', roundUsdIdrRate(parseFloat(event.target.value), 0) || undefined, { shouldValidate: true });
                                  }}
                                  placeholder="Contoh: 16880"
                                />
                              </FormControl>

                              <Text fontSize="sm" color={textColor} mt={2}>
                                {type === 'payable'
                                  ? 'Pembayaran hutang tetap melunasi AP dalam buku IDR, sedangkan rekening USD akan berkurang sebesar amount / kurs dan nilai buku bank dilepas memakai carry rate historis rekening.'
                                  : 'Pelunasan piutang multi-currency menutup AR pada nilai buku historis invoice, lalu selisih antara nilai buku kas/bank yang diterima dan nilai buku AR tersebut diakui sebagai realized FX.'}
                              </Text>

                              {referenceRate?.exchange_rate ? (
                                <Alert status="info" mt={3} borderRadius="md">
                                  <AlertIcon />
                                  <AlertDescription fontSize="sm">
                                    Kurs referensi API: 1 USD = {formatExchangeRate(referenceRate.exchange_rate)} dari {referenceRate.provider || referenceRate.source}. Anda tetap bisa override manual.
                                  </AlertDescription>
                                </Alert>
                              ) : (
                                <Alert status="warning" mt={3} borderRadius="md">
                                  <AlertIcon />
                                  <AlertDescription fontSize="sm">
                                    Kurs referensi API tidak tersedia. Isi kurs manual sesuai kurs deal/bank yang dipakai pembayaran.
                                  </AlertDescription>
                                </Alert>
                              )}

                              {watchedExchangeRate && watchedExchangeRate > 0 && selectedCashBank && (
                                <Box mt={3}>
                                  <Text fontSize="sm" fontWeight="bold">Preview pembayaran valas</Text>
                                  {type === 'payable' ? (
                                    <>
                                      <Text fontSize="sm" mt={1}>Hutang dilunasi: {paymentService.formatCurrency(watchedAmount || 0)}</Text>
                                      <Text fontSize="sm">USD keluar dari rekening: {formatCashBankBalance(usdNativeOutAmount, 'USD')}</Text>
                                      <Text fontSize="sm">Carry rate buku saat ini: 1 USD = {formatExchangeRate(currentCarryRate)}</Text>
                                      <Text fontSize="sm">Nilai buku bank yang dikredit jurnal: {paymentService.formatCurrency(bankBookOutAmount)}</Text>
                                      <Text fontSize="sm">Saldo native tersedia setelah pembayaran: {formatCashBankBalance(endingNativeBalance, 'USD')}</Text>
                                      <Text fontSize="sm">Nilai buku tersedia setelah pembayaran: {paymentService.formatCurrency(endingBookBalance)}</Text>
                                    </>
                                  ) : (
                                    <>
                                      <Text fontSize="sm" mt={1}>Piutang dilunasi: {paymentService.formatCurrency(watchedAmount || 0)}</Text>
                                      <Text fontSize="sm">USD settlement ekuivalen: {formatCashBankBalance(receivableNativeAmount, 'USD')}</Text>
                                      <Text fontSize="sm">Nilai buku kas/bank yang didebit jurnal: {paymentService.formatCurrency(watchedAmount || 0)}</Text>
                                      <Text fontSize="sm">Nilai buku piutang historis yang ditutup: {paymentService.formatCurrency(receivableSettlementBookAmount)}</Text>
                                      {hasUsdReceivableSelection && (
                                        <Text fontSize="sm">Kurs historis invoice USD diambil dari masing-masing invoice yang dialokasikan.</Text>
                                      )}
                                    </>
                                  )}
                                  <Text fontSize="sm" color={estimatedForexType === 'RUGI KURS' ? 'red.600' : estimatedForexType === 'LABA KURS' ? 'green.600' : textColor}>
                                    Estimasi {estimatedForexType}: {paymentService.formatCurrency(estimatedForexAmount)}
                                  </Text>
                                </Box>
                              )}
                            </Box>
                          )}


                          <FormControl mt={4}>
                            <FormLabel>Reference Number</FormLabel>
                            <Input
                              {...register('reference')}
                              placeholder="Transaction reference number"
                            />
                          </FormControl>

                          <FormControl mt={4}>
                            <FormLabel>Notes</FormLabel>
                            <Textarea
                              {...register('notes')}
                              placeholder="Additional notes about this payment"
                              rows={3}
                            />
                          </FormControl>

                          {outstandingItems.length > 0 && (
                            <FormControl mt={4}>
                              <HStack>
                                <Switch
                                  {...register('auto_allocate')}
                                  colorScheme="blue"
                                />
                                <FormLabel mb={0}>Auto-allocate to outstanding items</FormLabel>
                              </HStack>
                            </FormControl>
                          )}
                        </CardBody>
                      </Card>

                      {/* Payment Summary */}
                      {watchedAmount > 0 && (
                        <Card>
                          <CardBody>
                            <Text fontWeight="bold" mb={3}>Payment Summary</Text>
                            <SimpleGrid columns={{ base: 1, md: 3 }} spacing={4}>
                              <Box>
                                <Text fontSize="sm" color={textColor}>Payment Amount</Text>
                                <Text fontSize="lg" fontWeight="bold">
                                  {paymentService.formatCurrency(watchedAmount || 0)}
                                </Text>
                              </Box>
                              <Box>
                                <Text fontSize="sm" color={textColor}>Allocated</Text>
                                <Text fontSize="lg" fontWeight="bold" color="blue.600">
                                  {paymentService.formatCurrency(getTotalAllocated())}
                                </Text>
                              </Box>
                              <Box>
                                <Text fontSize="sm" color={textColor}>Remaining</Text>
                                <Text
                                  fontSize="lg"
                                  fontWeight="bold"
                                  color={getRemainingAmount() === 0 ? "green.600" : "orange.600"}
                                >
                                  {paymentService.formatCurrency(getRemainingAmount())}
                                </Text>
                              </Box>
                            </SimpleGrid>
                          </CardBody>
                        </Card>
                      )}

                      {outstandingItems.length > 0 && (
                        <HStack justify="center">
                          <Button
                            onClick={() => setActiveTab(1)}
                            colorScheme="blue"
                            variant="outline"
                          >
                            Configure Allocation ({outstandingItems.length} items)
                          </Button>
                        </HStack>
                      )}
                    </VStack>
                  </TabPanel>

                  {/* Allocation Tab */}
                  <TabPanel px={0}>
                    {loadingItems ? (
                      <Box textAlign="center" py={8}>
                        <Spinner size="lg" />
                        <Text mt={2}>Loading outstanding items...</Text>
                      </Box>
                    ) : (
                      <PaymentAllocationTable
                        items={outstandingItems}
                        allocations={allocations}
                        onAllocationsChange={setAllocations}
                        maxAmount={watchedAmount || 0}
                        type={type}
                      />
                    )}
                  </TabPanel>
                </TabPanels>
              </Tabs>
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
                {/* Payment Summary Info in Footer */}
                {watchedAmount > 0 && (
                  <Box flex="1" display={{ base: 'none', md: 'block' }}>
                    <HStack spacing={4}>
                      <Text fontSize="sm" color={textColor}>
                        Amount: <Text as="span" fontWeight="bold" color="blue.600">
                          {paymentService.formatCurrency(watchedAmount || 0)}
                        </Text>
                      </Text>
                      {getTotalAllocated() > 0 && (
                        <Text fontSize="sm" color={textColor}>
                          Remaining: <Text as="span" fontWeight="bold" color={getRemainingAmount() === 0 ? "green.600" : "orange.600"}>
                            {paymentService.formatCurrency(getRemainingAmount())}
                          </Text>
                        </Text>
                      )}
                    </HStack>
                  </Box>
                )}

                {/* Action Buttons */}
                <Button
                  variant="outline"
                  onClick={handleClose}
                  size={{ base: 'sm', md: 'md' }}
                  minW="80px"
                >
                  Cancel
                </Button>
                <Button
                  type="submit"
                  colorScheme="blue"
                  isLoading={isSubmitting || loading}
                  loadingText="Creating..."
                  isDisabled={!watchedContactId || !watchedAmount || (() => {
                    // Only enforce balance checks for payable (outgoing) payments
                    if (type !== 'payable') return false;
                    if (!watchedCashBankId || !watchedAmount) return false;

                    const selectedAccount = cashBankAccounts.find(account => account.id === watchedCashBankId);
                    if (selectedAccount) {
                      // Disable if zero balance or insufficient balance
                      if (getAvailableBookBalance(selectedAccount) <= 0 || watchedAmount > getAvailableBookBalance(selectedAccount)) {
                        return true;
                      }
                    }

                    return false;
                  })()}
                  size={{ base: 'sm', md: 'md' }}
                  minW="140px"
                  leftIcon={<FiDollarSign />}
                >
                  {(() => {
                    if (isSubmitting || loading) return 'Creating...';

                    // Only show balance warnings for payable payments
                    if (type === 'payable') {
                      const selectedAccount = cashBankAccounts.find(account => account.id === watchedCashBankId);
                      if (selectedAccount && watchedCashBankId && watchedAmount > 0) {
                        if (getAvailableBookBalance(selectedAccount) <= 0) {
                          return 'No Balance Available';
                        }
                        if (watchedAmount > getAvailableBookBalance(selectedAccount)) {
                          return 'Insufficient Balance';
                        }
                      }
                    }

                    return 'Create Payment';
                  })()}
                </Button>
              </HStack>
            </ModalFooter>
          </form>
        </ModalContent>
      </Modal>
    </>
  );
};

export default AdvancedPaymentForm;







