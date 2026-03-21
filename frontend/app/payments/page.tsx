'use client';

import React, { useEffect, useRef, useState } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useTranslation } from '@/hooks/useTranslation';
import { useModulePermissions } from '@/hooks/usePermissions';
import { parseClosedPeriodError, formatClosedPeriodDate } from '@/utils/errorHandler';
import ProtectedModule from '@/components/common/ProtectedModule';
import SimpleLayout from '@/components/layout/SimpleLayout';
import CompanyTypeGuard from '@/components/common/CompanyTypeGuard';
import { DataTable } from '@/components/common/DataTable';
import {
  Box,
  Flex,
  Heading,
  Button,
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
  Badge,
  Text,
  HStack,
  VStack,
  Select,
  Input,
  InputGroup,
  InputLeftElement,
  FormControl,
  FormLabel,
  IconButton,
  Tooltip,
  Spinner,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalFooter,
  ModalBody,
  ModalCloseButton,
  useDisclosure,
  AlertDialog,
  AlertDialogBody,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogContent,
  AlertDialogOverlay,
  Card,
  CardHeader,
  CardBody,
  Stat,
  StatLabel,
  StatNumber,
  StatHelpText,
  SimpleGrid,
  Menu,
  MenuButton,
  MenuList,
  MenuItem,
  MenuDivider,
  useToast,
  useColorModeValue,
} from '@chakra-ui/react';
import {
  FiPlus,
  FiEye,
  FiEdit,
  FiTrash2,
  FiFilter,
  FiRefreshCw,
  FiDownload,
  FiDollarSign,
  FiFilePlus,
  FiSearch,
  FiMoreVertical,
  FiFileText,
  FiChevronDown,
  FiArrowDown,
  FiArrowRight,
  FiBriefcase
} from 'react-icons/fi';
import paymentService, { Payment, PaymentFilters, PaymentResult, PaymentCreateRequest } from '@/services/paymentService';
import AdvancedPaymentForm from '@/components/payments/AdvancedPaymentForm';
import PaymentDetailModal from '@/components/payments/PaymentDetailModal';
import PPNPaymentModal from '@/components/payments/PPNPaymentModal';
import ExpensePaymentForm from '@/components/payments/ExpensePaymentForm';
import { exportPaymentsToPDF, exportPaymentDetailToPDF, PDFExportOptions } from '@/utils/pdfExport';
import ExportButton from '@/components/common/ExportButton';

// Status type for filtering
type PaymentStatusType = 'ALL' | 'PENDING' | 'COMPLETED' | 'FAILED' | 'DRAFT';

// Payment method type for filtering
type PaymentMethodType = 'ALL' | 'CASH' | 'BANK_TRANSFER' | 'CHECK' | 'CREDIT_CARD' | 'DEBIT_CARD' | 'OTHER';

// Pagination settings
const ITEMS_PER_PAGE = 10;

// Date formatter for display
const formatDate = (dateString: string) => {
  return new Date(dateString).toLocaleDateString('id-ID');
};

// Currency formatter - fixed to match sales format
const formatCurrency = (amount: number) => {
  return new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0
  }).format(amount);
};

const PaymentsPage: React.FC = () => {
  const { token, user } = useAuth();
  const { t, language } = useTranslation();
  const toast = useToast();

  // Theme colors for dark mode support
  const bgColor = useColorModeValue('gray.50', 'gray.900');
  const cardBg = useColorModeValue('white', 'gray.800');
  const headingColor = useColorModeValue('gray.800', 'gray.100');
  const textSecondary = useColorModeValue('gray.600', 'gray.400');
  const textPrimary = useColorModeValue('gray.700', 'gray.200');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const hoverBg = useColorModeValue('gray.50', 'gray.700');
  const inputBg = useColorModeValue('white', 'gray.700');
  const searchIconColor = useColorModeValue('gray.300', 'gray.500');
  const alertBg = useColorModeValue('red.50', 'red.900');
  const alertTextColor = useColorModeValue('red.600', 'red.300');
  const statColors = {
    green: useColorModeValue('green.500', 'green.400'),
    blue: useColorModeValue('blue.600', 'blue.400'),
    purple: useColorModeValue('purple.600', 'purple.400'),
    orange: useColorModeValue('orange.600', 'orange.400')
  };

  // Tooltip descriptions for payment page
  const tooltips = {
    search: 'Cari pembayaran berdasarkan kode, nama contact, atau nomor referensi',
    contact: 'Pilih customer atau vendor yang terkait dengan pembayaran',
    paymentMethod: 'Metode pembayaran: Cash (tunai), Bank Transfer (transfer bank), Check (cek), Credit Card (kartu kredit), dll',
    amount: 'Jumlah nominal pembayaran yang dilakukan',
    date: 'Tanggal pembayaran dilakukan',
    reference: 'Nomor referensi pembayaran (contoh: nomor transfer, nomor cek)',
    status: 'Status pembayaran: Pending (menunggu), Completed (selesai), Failed (gagal)',
    bankAccount: 'Akun kas/bank yang digunakan untuk pembayaran',
    notes: 'Catatan atau keterangan tambahan untuk pembayaran ini',
    allocations: 'Alokasi pembayaran ke invoice atau purchase yang terkait',
    attachments: 'Lampiran bukti pembayaran (foto transfer, receipt, dll)',
  };

  const [payments, setPayments] = useState<Payment[]>([]);
  const [allPayments, setAllPayments] = useState<Payment[]>([]); // Store all payments for client-side filtering
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchInput, setSearchInput] = useState(''); // Local search state for client-side filtering
  const [showFilters, setShowFilters] = useState(false);
  const [pagination, setPagination] = useState({
    current: 1,
    total: 1,
    totalItems: 0
  });
  const [summary, setSummary] = useState<any>(null);

  // Filter states
  const [filters, setFilters] = useState<PaymentFilters>({
    page: 1,
    limit: ITEMS_PER_PAGE
  });
  const [statusFilter, setStatusFilter] = useState<PaymentStatusType>('ALL');
  const [methodFilter, setMethodFilter] = useState<PaymentMethodType>('ALL');
  const [startDate, setStartDate] = useState<string>('');
  const [endDate, setEndDate] = useState<string>('');

  // State for modals
  const [showPaymentForm, setShowPaymentForm] = useState(false);
  const [paymentType, setPaymentType] = useState<'receivable' | 'payable'>('receivable');
  const [selectedPayment, setSelectedPayment] = useState<Payment | null>(null);
  const [showConfirmDelete, setShowConfirmDelete] = useState(false);
  const [showPaymentDetail, setShowPaymentDetail] = useState(false);
  const [formLoading, setFormLoading] = useState(false);
  const [showPPNPayment, setShowPPNPayment] = useState(false);
  const [ppnPaymentType, setPPNPaymentType] = useState<'INPUT' | 'OUTPUT'>('OUTPUT');
  const [showExpensePayment, setShowExpensePayment] = useState(false);
  const [approvingPaymentId, setApprovingPaymentId] = useState<number | null>(null);
  const cancelDeleteRef = useRef<HTMLButtonElement>(null);

  // Permission-based access control
  const { canCreate, canEdit, canDelete, canExport } = useModulePermissions('payments');

  // Check if current user is finance or admin
  const isFinanceOrAdmin = user?.role === 'finance' || user?.role === 'admin';

  // Approve DRAFT expense payment (finance only)
  const handleApproveExpensePayment = async (payment: Payment) => {
    if (!isFinanceOrAdmin) return;
    setApprovingPaymentId(payment.id);
    try {
      await paymentService.approveExpensePayment(payment.id);
      toast({
        title: 'Pembayaran Disetujui',
        description: `Pembayaran ${payment.code} telah disetujui dan jurnal telah dibuat.`,
        status: 'success',
        duration: 4000,
      });
      loadPayments();
    } catch (err: any) {
      toast({
        title: 'Gagal Menyetujui',
        description: err.message || 'Terjadi kesalahan saat menyetujui pembayaran',
        status: 'error',
        duration: 5000,
      });
    } finally {
      setApprovingPaymentId(null);
    }
  };

  // Approve DRAFT standard payment (finance only)
  const handleApproveStandardPayment = async (payment: Payment) => {
    if (!isFinanceOrAdmin) return;
    setApprovingPaymentId(payment.id);
    try {
      await paymentService.approveStandardPayment(payment.id);
      toast({
        title: 'Pembayaran Disetujui',
        description: `Pembayaran ${payment.code} telah disetujui dan jurnal telah diposting.`,
        status: 'success',
        duration: 4000,
      });
      loadPayments();
    } catch (err: any) {
      toast({
        title: 'Gagal Menyetujui',
        description: err.message || 'Terjadi kesalahan saat menyetujui pembayaran',
        status: 'error',
        duration: 5000,
      });
    } finally {
      setApprovingPaymentId(null);
    }
  };

  // New Payment handler
  const handleNewPayment = (type: 'receivable' | 'payable' = 'receivable') => {
    setPaymentType(type);
    setSelectedPayment(null);
    setShowPaymentForm(true);
  };

  // PPN Payment handler
  const handlePPNPayment = () => {
    setPPNPaymentType('OUTPUT');
    setShowPPNPayment(true);
  };

  // Edit Payment handler
  const handleEditPayment = (payment: Payment) => {
    // Check if PPN payment - cannot be edited
    if (payment.payment_type?.startsWith('TAX_PPN') || payment.code?.startsWith('SETOR-PPN')) {
      toast({
        title: t('payments.messages.cannotEditPPN'),
        description: t('payments.messages.cannotEditPPNDesc'),
        status: 'warning',
        duration: 5000,
      });
      return;
    }

    if (payment.payment_type === 'EXPENSE') {
      setSelectedPayment(payment);
      setShowExpensePayment(true);
      return;
    }

    // Detect payment type from contact type or code
    let type: 'receivable' | 'payable' = 'receivable';

    // Check contact type first (most reliable)
    if (payment.contact?.type === 'VENDOR') {
      type = 'payable';
    } else if (payment.contact?.type === 'CUSTOMER') {
      type = 'receivable';
    }
    // Fallback to code prefix
    else if (payment.code?.startsWith('PAY')) {
      type = 'payable';
    } else if (payment.code?.startsWith('RCV')) {
      type = 'receivable';
    }

    setPaymentType(type);
    setSelectedPayment(payment);
    setShowPaymentForm(true);
  };

  // View Payment handler
  const handleViewPayment = (payment: Payment) => {
    setSelectedPayment(payment);
    setShowPaymentDetail(true);
  };

  // Delete Payment handler
  const handleDeletePayment = (payment: Payment) => {
    setSelectedPayment(payment);
    setShowConfirmDelete(true);
  };

  const columns = [
    {
      header: t('payments.code'),
      accessor: (row: Payment) => (
        <Text fontWeight="medium" color={statColors.blue}>{row.code}</Text>
      )
    },
    {
      header: t('payments.contact'),
      accessor: (row: Payment) => {
        // For PPN tax payments, show "Negara" instead of contact name
        if (row.payment_type === 'TAX_PPN' ||
          row.payment_type === 'TAX_PPN_INPUT' ||
          row.payment_type === 'TAX_PPN_OUTPUT' ||
          row.code?.startsWith('SETOR-PPN')) {
          return t('payments.negara');
        }
        return row.contact?.name || '-';
      }
    },
    {
      header: t('payments.date'),
      accessor: (row: Payment) => formatDate(row.date)
    },
    {
      header: t('payments.amount'),
      accessor: (row: Payment) => (
        <Text fontWeight="medium">{formatCurrency(row.amount)}</Text>
      )
    },
    {
      header: t('payments.method'),
      accessor: (row: Payment) => paymentService.getMethodDisplayName(row.method)
    },
    {
      header: t('payments.status'),
      accessor: (row: Payment) => {
        const statusKey = row.status.toLowerCase();
        const translationKey = `payments.statuses.${statusKey}`;
        const statusLabel = t(translationKey);
        return (
          <Badge colorScheme={paymentService.getStatusColorScheme(row.status)} variant="subtle">
            {statusLabel}
          </Badge>
        );
      }
    },
    {
      header: t('payments.actions'),
      accessor: (row: Payment) => (
        <Menu strategy="fixed" placement="bottom-end">
          <MenuButton
            as={IconButton}
            icon={<FiMoreVertical />}
            variant="ghost"
            size="sm"
            aria-label={t('payments.actions')}
          />
          <MenuList zIndex={9999}>
            <MenuItem icon={<FiEye />} onClick={() => handleViewPayment(row)}>
              {t('payments.viewDetails')}
            </MenuItem>
            {/* Approve button: only for DRAFT payments and finance/admin */}
            {isFinanceOrAdmin && row.status === 'DRAFT' && (
              <>
                <MenuDivider />
                <MenuItem
                  icon={<FiArrowRight />}
                  color="green.500"
                  isDisabled={approvingPaymentId === row.id}
                  onClick={() => {
                    // Route approval based on payment type
                    if (row.payment_type === 'EXPENSE') {
                      handleApproveExpensePayment(row);
                    } else {
                      handleApproveStandardPayment(row);
                    }
                  }}
                >
                  {approvingPaymentId === row.id ? 'Menyetujui...' : 'Setujui & Posting Jurnal'}
                </MenuItem>
                <MenuDivider />
              </>
            )}
            {canEdit && row.status !== 'COMPLETED' && !(row.payment_type?.startsWith('TAX_PPN') || row.code?.startsWith('SETOR-PPN')) && (
              <MenuItem icon={<FiEdit />} onClick={() => handleEditPayment(row)}>
                {t('payments.edit')}
              </MenuItem>
            )}
            <MenuItem
              icon={<FiFilePlus />}
              onClick={() => paymentService.downloadPaymentDetailPDF(row.id, row.code)}
            >
              {t('payments.exportPDF')}
            </MenuItem>
            {canDelete && (
              <>
                <MenuDivider />
                <MenuItem icon={<FiTrash2 />} color="red.500" onClick={() => handleDeletePayment(row)}>
                  {t('payments.delete')}
                </MenuItem>
              </>
            )}
          </MenuList>
        </Menu>
      )
    }
  ];

  // Load payments data
  const loadPayments = async (newFilters?: Partial<PaymentFilters>) => {
    try {
      setLoading(true);
      setError(null);

      const currentFilters = newFilters ? { ...filters, ...newFilters } : filters;

      // Prepare filters for API request
      const apiFilters: PaymentFilters = {
        page: currentFilters.page,
        limit: currentFilters.limit
      };

      // NOTE: Search is handled client-side, so we don't send search to API
      // Other filters (status, method, date) still use server-side filtering

      // Add status filter if selected
      if (statusFilter !== 'ALL') {
        apiFilters.status = statusFilter;
      }

      // Add method filter if selected
      if (methodFilter !== 'ALL') {
        apiFilters.method = methodFilter;
      }

      // Add date filters if selected
      if (startDate) {
        apiFilters.start_date = startDate;
      }

      if (endDate) {
        apiFilters.end_date = endDate;
      }

      // Make API call
      const result = await paymentService.getPayments(apiFilters);

      // Store all payments for client-side filtering
      const paymentData = result?.data || [];
      setAllPayments(paymentData);

      // Apply client-side search filter if search input exists
      if (searchInput && searchInput.trim()) {
        const filtered = filterPaymentsBySearch(paymentData, searchInput);
        setPayments(filtered);
      } else {
        setPayments(paymentData);
      }

      setFilters({ ...currentFilters, page: result?.page || currentFilters.page });
      setPagination({
        current: result?.page || 1,
        total: result?.total_pages || 1,
        totalItems: result?.total || 0
      });

    } catch (err: any) {
      console.error('Error fetching payments:', err);
      setError(err.message || 'An error occurred while fetching payments');
      setPayments([]);

      toast({
        title: 'Error loading payments',
        description: err.message || 'Failed to load payments data',
        status: 'error',
        duration: 3000
      });
    } finally {
      setLoading(false);
    }
  };

  // Load payment summary
  const loadPaymentSummary = async () => {
    try {
      // Use allPayments (full page data) to calculate summary
      const dataSource = allPayments.length > 0 ? allPayments : payments;
      const totalAmount = dataSource.reduce((sum, payment) => sum + payment.amount, 0);
      const completedPayments = dataSource.filter(p => p.status === 'COMPLETED');
      const pendingPayments = dataSource.filter(p => p.status === 'PENDING');
      const completedAmount = completedPayments.reduce((sum, payment) => sum + payment.amount, 0);
      const pendingAmount = pendingPayments.reduce((sum, payment) => sum + payment.amount, 0);

      setSummary({
        total_payments: dataSource.length,
        total_amount: totalAmount,
        completed_amount: completedAmount,
        pending_amount: pendingAmount,
        completed_count: completedPayments.length,
        pending_count: pendingPayments.length,
        avg_payment_value: dataSource.length > 0 ? totalAmount / dataSource.length : 0
      });
    } catch (error) {
      console.error('Failed to calculate payment summary:', error);
    }
  };

  // Initial load
  useEffect(() => {
    if (token) {
      loadPayments();
    }
  }, [token]);

  // Update summary when payments change
  useEffect(() => {
    if (payments.length > 0) {
      loadPaymentSummary();
    } else {
      // Reset summary when no payments match
      setSummary({
        total_payments: 0,
        total_amount: 0,
        completed_amount: 0,
        pending_amount: 0,
        completed_count: 0,
        pending_count: 0,
        avg_payment_value: 0
      });
    }
  }, [payments, allPayments, pagination.totalItems]);

  // Handle filters change - trigger API call
  useEffect(() => {
    if (token) {
      loadPayments({ page: 1 });
    }
  }, [statusFilter, methodFilter, startDate, endDate]);

  // Apply client-side search when allPayments changes
  useEffect(() => {
    if (searchInput) {
      handleSearch(searchInput);
    }
  }, [allPayments]);

  // Helper function to filter payments by search term (client-side)
  const filterPaymentsBySearch = (paymentsToFilter: Payment[], searchTerm: string) => {
    if (!searchTerm.trim()) return paymentsToFilter;

    const term = searchTerm.toLowerCase();
    return paymentsToFilter.filter(payment => {
      // Search in payment code
      if (payment.code?.toLowerCase().includes(term)) return true;

      // Search in contact name (handle PPN tax payments - show "Negara")
      if (payment.payment_type === 'TAX_PPN' ||
        payment.payment_type === 'TAX_PPN_INPUT' ||
        payment.payment_type === 'TAX_PPN_OUTPUT' ||
        payment.code?.startsWith('SETOR-PPN')) {
        if ('negara'.includes(term)) return true;
      } else if (payment.contact?.name?.toLowerCase().includes(term)) {
        return true;
      }

      // Search in payment reference
      if (payment.reference?.toLowerCase().includes(term)) return true;

      // Search in notes
      if (payment.notes?.toLowerCase().includes(term)) return true;

      return false;
    });
  };

  // Handle page change
  const handlePageChange = (page: number) => {
    setFilters(prev => ({
      ...prev,
      page: page
    }));
    loadPayments({ page });
  };

  // Client-side search handler (instant, no API call)
  const handleSearch = (value: string) => {
    setSearchInput(value);

    // Client-side filtering - no API call
    if (!value.trim()) {
      // If search is empty, show all payments
      setPayments(allPayments);
      return;
    }

    // Filter payments based on search term
    const filtered = filterPaymentsBySearch(allPayments, value);
    setPayments(filtered);
  };

  // Handle filter change
  const handleFilterChange = (key: string, value: string) => {
    switch (key) {
      case 'status':
        setStatusFilter(value as PaymentStatusType);
        break;
      case 'method':
        setMethodFilter(value as PaymentMethodType);
        break;
      case 'start_date':
        setStartDate(value);
        break;
      case 'end_date':
        setEndDate(value);
        break;
    }
  };

  // Apply filters
  const applyFilters = () => {
    loadPayments({ page: 1 });
  };

  // Reset filters
  const resetFilters = () => {
    setStatusFilter('ALL');
    setMethodFilter('ALL');
    setStartDate('');
    setEndDate('');
    setSearchInput(''); // Clear search input
    setPayments(allPayments); // Reset to show all payments
    setFilters({
      page: 1,
      limit: ITEMS_PER_PAGE
    });
    // Reload data from server with reset filters
    loadPayments({ page: 1 });
  };

  // Note: handleDelete (window.confirm) removed — delete uses AlertDialog flow via handleDeletePayment + confirmDeletePayment

  // Confirm delete
  const confirmDeletePayment = async () => {
    if (!selectedPayment) return;

    try {
      await paymentService.deletePayment(selectedPayment.id);
      toast({
        title: t('common.messages.toast.success'),
        description: t('payments.messages.deleteSuccess'),
        status: 'success',
        duration: 3000
      });
      loadPayments();
      setShowConfirmDelete(false);
      setSelectedPayment(null);
    } catch (error: any) {
      console.error('Error deleting payment:', error);
      const errMsg = error.message || '';
      const closedPeriod = parseClosedPeriodError(errMsg);
      if (closedPeriod) {
        toast({
          title: t('common.messages.toast.closedPeriodError'),
          description: t('common.messages.toast.closedPeriodErrorDesc', { date: formatClosedPeriodDate(closedPeriod.date, language) }),
          status: 'error',
          duration: 5000,
          isClosable: true,
        });
      } else {
        toast({
          title: t('common.messages.toast.error'),
          description: errMsg || t('payments.messages.deleteError'),
          status: 'error',
          duration: 5000,
          isClosable: true,
        });
      }
    }
  };

  // Export payments to Excel handler
  const handleExportPayments = async () => {
    try {
      // Prepare current filters for export
      const exportFilters: PaymentFilters = { ...filters };
      if (statusFilter !== 'ALL') exportFilters.status = statusFilter;
      if (methodFilter !== 'ALL') exportFilters.method = methodFilter;
      if (startDate) exportFilters.start_date = startDate;
      if (endDate) exportFilters.end_date = endDate;

      // Call export API
      const blob = await paymentService.exportPayments(exportFilters);

      // Create download link
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `payments-export-${new Date().toISOString().split('T')[0]}.xlsx`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
    } catch (error: any) {
      console.error('Error exporting payments:', error);
      setError(error.message || 'Failed to export payments');
    }
  };

  // Export payments to PDF handler
  const handleExportPaymentsPDF = async () => {
    try {
      setError(null);

      // Prepare PDF export options
      const pdfOptions: PDFExportOptions = {
        title: 'Laporan Pembayaran',
        subtitle: 'Daftar Transaksi Pembayaran',
        companyName: 'PT. Sistem Akuntansi',
        companyAddress: 'Jakarta, Indonesia',
        includeFilters: true,
        statusFilter: statusFilter,
        methodFilter: methodFilter,
        startDate: startDate,
        endDate: endDate
      };

      // Export PDF with current data
      exportPaymentsToPDF(payments, pdfOptions);

    } catch (error: any) {
      console.error('Error exporting payments to PDF:', error);
      setError(error.message || 'Failed to export payments to PDF');
    }
  };

  // Handle bulk export (like sales page)
  const handleBulkExport = async () => {
    try {
      // Prepare filters for export
      const exportStatus = statusFilter !== 'ALL' ? statusFilter : undefined;
      const exportMethod = methodFilter !== 'ALL' ? methodFilter : undefined;

      // Use backend PDF export
      await paymentService.downloadPaymentReportPDF(
        startDate || undefined,
        endDate || undefined,
        exportStatus,
        exportMethod
      );

      toast({
        title: t('common.messages.toast.success'),
        description: t('payments.messages.exportSuccess'),
        status: 'success',
        duration: 3000
      });
    } catch (error: any) {
      console.error('Error exporting payments to PDF:', error);
      toast({
        title: t('common.messages.toast.error'),
        description: t('payments.messages.exportError'),
        status: 'error',
        duration: 3000
      });
    }
  };

  // Refresh data handler
  const handleRefreshData = () => {
    loadPayments();
    loadPaymentSummary();
  };

  // Handle form save
  const handleFormSave = () => {
    setShowPaymentForm(false);
    setSelectedPayment(null);
    loadPayments();
  };

  // Handle form cancel
  const handleFormCancel = () => {
    setShowPaymentForm(false);
    setSelectedPayment(null);
    setError(null);
  };

  if (loading && (!payments || payments.length === 0)) {
    return (
      <CompanyTypeGuard>
        <SimpleLayout allowedRoles={['admin', 'finance', 'director', 'employee', 'inventory_manager']}>
          <Box display="flex" justifyContent="center" alignItems="center" height="400px">
            <Spinner size="xl" thickness="4px" speed="0.65s" color="brand.500" />
            <Text ml={4} fontSize="lg">{t('payments.loading')}</Text>
          </Box>
        </SimpleLayout>
      </CompanyTypeGuard>
    );
  }

  return (
    <ProtectedModule module="payments">
      <CompanyTypeGuard>
        <SimpleLayout allowedRoles={['admin', 'finance', 'director', 'employee', 'inventory_manager']}>
          <Box bg={bgColor} minH="100vh" p={6}>
            <VStack spacing={6} align="stretch">
              {/* Summary Cards */}
              {summary && (
                <SimpleGrid columns={{ base: 1, md: 2, lg: 4 }} spacing={4}>
                  <Card bg={cardBg} borderWidth="1px" borderColor={borderColor}>
                    <CardBody>
                      <Stat>
                        <StatLabel>{t('payments.summary.totalPayments')}</StatLabel>
                        <StatNumber>{summary.total_payments}</StatNumber>
                        <StatHelpText>{t('payments.summary.currentPage')}</StatHelpText>
                      </Stat>
                    </CardBody>
                  </Card>
                  <Card bg={cardBg} borderWidth="1px" borderColor={borderColor}>
                    <CardBody>
                      <Stat>
                        <StatLabel>{t('payments.summary.totalAmount')}</StatLabel>
                        <StatNumber>{formatCurrency(summary.total_amount)}</StatNumber>
                        <StatHelpText>{t('payments.summary.loadedRowsOnly')}</StatHelpText>
                      </Stat>
                    </CardBody>
                  </Card>
                  <Card bg={cardBg} borderWidth="1px" borderColor={borderColor}>
                    <CardBody>
                      <Stat>
                        <StatLabel>{t('payments.summary.completed')}</StatLabel>
                        <StatNumber color={statColors.green}>
                          {formatCurrency(summary.completed_amount)}
                        </StatNumber>
                        <StatHelpText>{summary.completed_count} {t('payments.summary.payments')} {t('payments.summary.onCurrentPage')}</StatHelpText>
                      </Stat>
                    </CardBody>
                  </Card>
                  <Card bg={cardBg} borderWidth="1px" borderColor={borderColor}>
                    <CardBody>
                      <Stat>
                        <StatLabel>{t('payments.summary.avgPaymentValue')}</StatLabel>
                        <StatNumber>{formatCurrency(summary.avg_payment_value)}</StatNumber>
                        <StatHelpText>{t('payments.summary.perLoadedTransaction')}</StatHelpText>
                      </Stat>
                    </CardBody>
                  </Card>
                </SimpleGrid>
              )}

              {/* Header */}
              <Flex justify="space-between" align="center">
                <Box>
                  <Heading as="h1" size="xl" mb={2} color={headingColor}>{t('payments.title')}</Heading>
                  <Text color={textSecondary}>{t('payments.managePayments')}</Text>
                </Box>
                <HStack spacing={3}>
                  <Button
                    leftIcon={<FiRefreshCw />}
                    variant="ghost"
                    size="md"
                    onClick={handleRefreshData}
                    isLoading={loading}
                  >
                    {t('payments.refresh')}
                  </Button>
                  {canExport && (
                    <Menu>
                      <MenuButton
                        as={Button}
                        leftIcon={<FiDownload />}
                        colorScheme="green"
                        variant="outline"
                        size="md"
                      >
                        {t('payments.exportReport')}
                      </MenuButton>
                      <MenuList>
                        <MenuItem icon={<FiFileText />} onClick={handleBulkExport}>
                          {t('payments.exportPDFReport')}
                        </MenuItem>
                        <MenuItem icon={<FiDownload />} onClick={async () => {
                          try {
                            // Prepare current filters for export
                            const exportFilters: any = {};
                            if (statusFilter !== 'ALL') exportFilters.status = statusFilter;
                            if (methodFilter !== 'ALL') exportFilters.method = methodFilter;
                            if (startDate) exportFilters.start_date = startDate;
                            if (endDate) exportFilters.end_date = endDate;

                            // Call Excel export API
                            await paymentService.downloadPaymentReportExcel(
                              startDate || undefined,
                              endDate || undefined,
                              exportFilters.status,
                              exportFilters.method
                            );

                            toast({
                              title: t('common.messages.toast.success'),
                              description: t('payments.messages.exportSuccess'),
                              status: 'success',
                              duration: 3000
                            });
                          } catch (error: any) {
                            console.error('Error exporting payments to Excel:', error);
                            toast({
                              title: t('common.messages.toast.error'),
                              description: t('payments.messages.exportError'),
                              status: 'error',
                              duration: 3000
                            });
                          }
                        }}>
                          {t('payments.exportExcelReport')}
                        </MenuItem>
                      </MenuList>
                    </Menu>
                  )}
                  {canCreate && (
                    <Menu>
                      <MenuButton
                        as={Button}
                        leftIcon={<FiPlus />}
                        rightIcon={<FiChevronDown />}
                        colorScheme="blue"
                        size="md"
                      >
                        {t('payments.createPayment')}
                      </MenuButton>
                      <MenuList>
                        <MenuItem
                          icon={<FiArrowDown />}
                          onClick={() => handleNewPayment('receivable')}
                        >
                          {t('payments.receivablePayment')}
                        </MenuItem>
                        <MenuItem
                          icon={<FiArrowRight />}
                          onClick={() => handleNewPayment('payable')}
                        >
                          {t('payments.payablePayment')}
                        </MenuItem>
                        <MenuDivider />
                        <MenuItem
                          icon={<FiDollarSign />}
                          onClick={() => handlePPNPayment()}
                        >
                          {t('payments.setorPPN')}
                        </MenuItem>
                        <MenuItem
                          icon={<FiBriefcase />}
                          onClick={() => {
                            setSelectedPayment(null);
                            setShowExpensePayment(true);
                          }}
                        >
                          {t('payments.expensePayment')}
                        </MenuItem>
                      </MenuList>
                    </Menu>
                  )}
                </HStack>
              </Flex>

              {/* Search and Filters */}
              <Card bg={cardBg} borderWidth="1px" borderColor={borderColor}>
                <CardBody>
                  <HStack spacing={4} wrap="wrap">
                    <InputGroup maxW="400px">
                      <InputLeftElement pointerEvents="none">
                        <FiSearch color={searchIconColor} />
                      </InputLeftElement>
                      <Input
                        placeholder={t('payments.searchPlaceholder')}
                        value={searchInput}
                        onChange={(e) => handleSearch(e.target.value)}
                        bg={inputBg}
                        borderColor={borderColor}
                      />
                    </InputGroup>

                    <Select
                      maxW="200px"
                      placeholder={t('payments.filters.allStatus')}
                      value={statusFilter}
                      onChange={(e) => handleFilterChange('status', e.target.value)}
                      bg={inputBg}
                      borderColor={borderColor}
                    >
                      <option value="PENDING">{t('payments.filters.pending')}</option>
                      <option value="COMPLETED">{t('payments.filters.completed')}</option>
                      <option value="DRAFT">{t('payments.filters.draft')}</option>
                      <option value="FAILED">{t('payments.filters.failed')}</option>
                    </Select>

                    <Select
                      maxW="200px"
                      placeholder={t('payments.filters.allMethods')}
                      value={methodFilter}
                      onChange={(e) => handleFilterChange('method', e.target.value)}
                      bg={inputBg}
                      borderColor={borderColor}
                    >
                      <option value="CASH">{t('payments.filters.cash')}</option>
                      <option value="BANK_TRANSFER">{t('payments.filters.bankTransfer')}</option>
                      <option value="CHECK">{t('payments.filters.check')}</option>
                      <option value="CREDIT_CARD">{t('payments.filters.creditCard')}</option>
                      <option value="DEBIT_CARD">{t('payments.filters.debitCard')}</option>
                    </Select>

                    <Input
                      type="date"
                      maxW="200px"
                      placeholder={t('payments.filters.startDate')}
                      value={startDate}
                      onChange={(e) => handleFilterChange('start_date', e.target.value)}
                      bg={inputBg}
                      borderColor={borderColor}
                    />

                    <Input
                      type="date"
                      maxW="200px"
                      placeholder={t('payments.filters.endDate')}
                      value={endDate}
                      onChange={(e) => handleFilterChange('end_date', e.target.value)}
                      bg={inputBg}
                      borderColor={borderColor}
                    />

                    <Button
                      leftIcon={<FiFilter />}
                      variant="outline"
                      onClick={resetFilters}
                      colorScheme="gray"
                    >
                      {t('payments.clearFilters')}
                    </Button>
                  </HStack>
                </CardBody>
              </Card>

              {/* Error Alert */}
              {error && (
                <Alert status="error">
                  <AlertIcon />
                  {error}
                </Alert>
              )}

              {/* Payments Table */}
              <Card bg={cardBg} borderWidth="1px" borderColor={borderColor}>
                <CardHeader>
                  <Flex justify="space-between" align="center">
                    <Heading size="md" color={headingColor}>{t('payments.paymentTransactions')} ({payments?.length || 0})</Heading>
                  </Flex>
                </CardHeader>
                <CardBody>
                  {loading ? (
                    <Flex justify="center" py={10}>
                      <Spinner size="lg" />
                    </Flex>
                  ) : (
                    <DataTable
                      columns={columns}
                      data={payments || []}
                      keyField="id"
                      searchable={false}
                      pagination={true}
                      pageSize={ITEMS_PER_PAGE}
                    />
                  )}
                </CardBody>
              </Card>
            </VStack>

            {/* Payment Form Modal */}
            <AdvancedPaymentForm
              isOpen={showPaymentForm}
              onClose={handleFormCancel}
              type={paymentType}
              onSuccess={handleFormSave}
              initialData={selectedPayment}
              preSelectedContact={selectedPayment ? {
                id: selectedPayment.contact_id,
                name: selectedPayment.contact?.name || 'Unknown'
              } : undefined}
            />

            {/* Delete Confirmation Dialog */}
            <AlertDialog
              isOpen={showConfirmDelete}
              leastDestructiveRef={cancelDeleteRef}
              onClose={() => setShowConfirmDelete(false)}
            >
              <AlertDialogOverlay>
                <AlertDialogContent bg={cardBg}>
                  <AlertDialogHeader fontSize="lg" fontWeight="bold" color={headingColor}>
                    {t('payments.deletePayment')}
                  </AlertDialogHeader>

                  <AlertDialogBody color={textPrimary}>
                    {t('payments.deleteConfirmation')}
                    {selectedPayment && (
                      <Box mt={3} p={3} bg={alertBg} borderRadius="md">
                        <Text fontSize="sm" fontWeight="bold" color={textPrimary}>{t('payments.code')}: {selectedPayment.code}</Text>
                        <Text fontSize="sm" color={textPrimary}>{t('payments.amount')}: {formatCurrency(selectedPayment.amount)}</Text>
                        <Text fontSize="sm" color={alertTextColor}>{t('payments.deleteWarning')}</Text>
                      </Box>
                    )}
                  </AlertDialogBody>

                  <AlertDialogFooter>
                    <Button ref={cancelDeleteRef} onClick={() => setShowConfirmDelete(false)}>
                      {t('payments.cancel')}
                    </Button>
                    <Button colorScheme="red" onClick={confirmDeletePayment} ml={3}>
                      {t('payments.delete')}
                    </Button>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialogOverlay>
            </AlertDialog>
            {/* Payment Detail Modal */}
            <PaymentDetailModal
              payment={selectedPayment}
              isOpen={showPaymentDetail}
              onClose={() => {
                setShowPaymentDetail(false);
                setSelectedPayment(null);
              }}
            />

            {/* PPN Payment Modal */}
            <PPNPaymentModal
              isOpen={showPPNPayment}
              onClose={() => setShowPPNPayment(false)}
              ppnType={ppnPaymentType}
              onSuccess={() => {
                setShowPPNPayment(false);
                loadPayments();
              }}
            />

            {/* Expense Payment Modal */}
            <ExpensePaymentForm
              isOpen={showExpensePayment}
              onClose={() => {
                setShowExpensePayment(false);
                setSelectedPayment(null);
              }}
              initialData={selectedPayment}
              onSuccess={() => {
                setShowExpensePayment(false);
                setSelectedPayment(null);
                loadPayments();
              }}
            />
          </Box>
        </SimpleLayout>
      </CompanyTypeGuard>
    </ProtectedModule>
  );
};

export default PaymentsPage;
