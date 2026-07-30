'use client';

import React, { useEffect, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
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
  Spinner,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalFooter,
  ModalBody,
  ModalCloseButton,
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
  Tabs,
  TabList,
  Tab,
  useToast,
  useColorModeValue,
  Textarea,
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
  FiChevronLeft,
  FiChevronsLeft,
  FiChevronRight,
  FiChevronsRight,
  FiArrowRight,
  FiBriefcase
} from 'react-icons/fi';
import paymentService, {
  Payment,
  PaymentFilters,
  PaymentTransaction,
  PaymentTransactionFilters,
  PaymentTransactionResult,
} from '@/services/paymentService';
import AdvancedPaymentForm from '@/components/payments/AdvancedPaymentForm';
import PurchaseTermPaymentPicker from '@/components/payments/PurchaseTermPaymentPicker';
import PaymentDetailModal from '@/components/payments/PaymentDetailModal';
import PPNPaymentModal from '@/components/payments/PPNPaymentModal';
import ExpensePaymentForm from '@/components/payments/ExpensePaymentForm';
import { PostedTransactionCorrectionsQueueModal } from '@/components/corrections/PostedTransactionCorrections';
import { exportPaymentsToPDF, PDFExportOptions } from '@/utils/pdfExport';
import { isAccountingRole, isFinanceManagerRole, normalizeRole } from '@/utils/roles';
import { getPaymentPPhStatus } from '@/utils/taxPaymentStatus';

// Status type for filtering
type PaymentStatusType = 'ALL' | 'PENDING' | 'PENDING_ACCOUNTING' | 'PENDING_FINAL_REVIEW' | 'COMPLETED' | 'FAILED' | 'DRAFT' | 'REJECTED';

// Payment method type for filtering
type PaymentMethodType = 'ALL' | 'CASH' | 'BANK_TRANSFER' | 'CHECK' | 'CREDIT_CARD' | 'DEBIT_CARD' | 'OTHER';
type PaymentTransactionFilter = 'ALL' | 'RECEIPT' | 'OUTGOING' | 'NON_INVOICE';

// Pagination settings
const ITEMS_PER_PAGE = 10;

// Date formatter for display
const formatDate = (dateString: string) => {
  return new Date(dateString).toLocaleDateString('id-ID');
};

// Currency formatter - fixed to match sales format
const formatCurrency = (amount: number) => {
  const hasFraction = Math.abs((amount || 0) % 1) > Number.EPSILON;

  return new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    minimumFractionDigits: hasFraction ? 2 : 0,
    maximumFractionDigits: 2
  }).format(amount);
};

const isCancelledPaymentStatus = (status: string) => {
  const normalized = status.toUpperCase();
  return normalized === 'FAILED' || normalized === 'CANCELLED' || normalized === 'REJECTED' || normalized === 'REVERSED';
};

const PaymentsPage: React.FC = () => {
  const { token, user } = useAuth();
  const router = useRouter();
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
    status: 'Status pembayaran: Pending (menunggu), Completed (selesai), Cancelled (dibatalkan)',
    bankAccount: 'Akun kas/bank yang digunakan untuk pembayaran',
    notes: 'Catatan atau keterangan tambahan untuk pembayaran ini',
    allocations: 'Alokasi pembayaran ke invoice atau purchase yang terkait',
    attachments: 'Lampiran bukti pembayaran (foto transfer, receipt, dll)',
  };

  const [payments, setPayments] = useState<PaymentTransaction[]>([]);
  const [allPayments, setAllPayments] = useState<PaymentTransaction[]>([]);
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
  const [filters, setFilters] = useState<PaymentTransactionFilters>({
    page: 1,
    limit: ITEMS_PER_PAGE
  });
  const [transactionFilter, setTransactionFilter] = useState<PaymentTransactionFilter>('ALL');
  const [statusFilter, setStatusFilter] = useState<PaymentStatusType>('ALL');
  const [methodFilter, setMethodFilter] = useState<PaymentMethodType>('ALL');
  const [startDate, setStartDate] = useState<string>('');
  const [endDate, setEndDate] = useState<string>('');

  // State for modals
  const [showPaymentForm, setShowPaymentForm] = useState(false);
  const [showPurchaseTermPayment, setShowPurchaseTermPayment] = useState(false);
  const [paymentType, setPaymentType] = useState<'receivable' | 'payable'>('receivable');
  const [selectedPayment, setSelectedPayment] = useState<Payment | null>(null);
  const [showConfirmDelete, setShowConfirmDelete] = useState(false);
  const [showPaymentDetail, setShowPaymentDetail] = useState(false);
  const [formLoading, setFormLoading] = useState(false);
  const [showPPNPayment, setShowPPNPayment] = useState(false);
  const [ppnPaymentType, setPPNPaymentType] = useState<'INPUT' | 'OUTPUT'>('OUTPUT');
  const [showExpensePayment, setShowExpensePayment] = useState(false);
  const [showCorrectionQueue, setShowCorrectionQueue] = useState(false);
  const [paymentDecisionLoadingId, setPaymentDecisionLoadingId] = useState<number | null>(null);
  const [selectedPaymentForRejection, setSelectedPaymentForRejection] = useState<Payment | null>(null);
  const [rejectReason, setRejectReason] = useState('');
  const cancelDeleteRef = useRef<HTMLButtonElement>(null);
  const hasMountedSearchEffect = useRef(false);

  // Permission-based access control
  const { canCreate, canEdit, canDelete, canExport } = useModulePermissions('payments');

  // Approval actors
  const roleNorm = normalizeRole(user?.role);
  const isFinanceManagerApprover = isFinanceManagerRole(roleNorm);
  const isDirectorApprover = roleNorm === 'director';
  const isAccountingReviewer = isAccountingRole(roleNorm) || roleNorm === 'admin';

  const getStatusBadgeLabel = (payment: Payment) => paymentService.getStatusLabel(payment.status);

  const getStatusMetaText = (payment: Payment) => {
    if (payment.status === 'DRAFT') {
      return 'Menunggu approval finance manager';
    }
    if (payment.status === 'PENDING' && payment.require_director_approval) {
      const financeManagerName = paymentService.getActorDisplayName(
        payment.finance_manager_approver || payment.approver,
      );
      if (payment.finance_manager_approved_at) {
        return `Menunggu approval director. Finance Manager ${financeManagerName} sudah approve pada ${paymentService.formatDateTime(payment.finance_manager_approved_at)}.`;
      }
      return 'Menunggu approval director';
    }
    if (payment.status === 'PENDING_ACCOUNTING') {
      const financeManagerName = paymentService.getActorDisplayName(
        payment.finance_manager_approver || payment.approver,
      );
      if (payment.finance_manager_approved_at) {
        return `FM ${financeManagerName} sudah approve pada ${paymentService.formatDateTime(payment.finance_manager_approved_at)}. Menunggu Accounting pilih COA.`;
      }
      return 'Menunggu Accounting pilih COA';
    }
    if (payment.status === 'PENDING_FINAL_REVIEW') {
      return 'COA sudah disiapkan Accounting. Menunggu FM review dan posting jurnal.';
    }
    if (payment.status === 'COMPLETED' && payment.approved_at) {
      return `Approved by ${paymentService.getActorDisplayName(payment.approver)} pada ${paymentService.formatDateTime(payment.approved_at)}`;
    }
    if (payment.status === 'REJECTED' && payment.rejected_at) {
      return `Rejected by ${paymentService.getActorDisplayName(payment.rejector)} pada ${paymentService.formatDateTime(payment.rejected_at)}`;
    }
    if (payment.status === 'PENDING') {
      return 'Pending processing';
    }
    return '';
  };

  const getApprovalSuccessMessage = (payment: Payment, result?: Payment) => {
    switch (result?.status) {
      case 'PENDING':
        return {
          title: 'Dikirim ke Director',
          description: `Pembayaran ${payment.code} sudah disetujui finance manager dan dikirim ke director.`,
        };
      case 'PENDING_ACCOUNTING':
        return {
          title: 'Disetujui FM',
          description: `Pembayaran ${payment.code} masuk ke Accounting untuk set/review COA.`,
        };
      case 'COMPLETED':
        return {
          title: 'Jurnal Diposting',
          description: `Pembayaran ${payment.code} sudah direview FM dan jurnal telah diposting.`,
        };
      default:
        return {
          title: 'Pembayaran Disetujui',
          description: `Status pembayaran ${payment.code} berhasil diperbarui.`,
        };
    }
  };

  // Advance expense payment approval flow.
  const handleApproveExpensePayment = async (payment: Payment, requireDirectorApproval = false) => {
    if (!isFinanceManagerApprover && !isDirectorApprover) return;
    setPaymentDecisionLoadingId(payment.id);
    try {
      const result = await paymentService.approveExpensePayment(payment.id, requireDirectorApproval);
      const successMessage = getApprovalSuccessMessage(payment, result);
      toast({
        title: successMessage.title,
        description: successMessage.description,
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
      setPaymentDecisionLoadingId(null);
    }
  };

  // Advance receivable/payable payment approval flow.
  const handleApproveStandardPayment = async (payment: Payment, requireDirectorApproval = false) => {
    if (!isFinanceManagerApprover && !isDirectorApprover) return;
    setPaymentDecisionLoadingId(payment.id);
    try {
      const result = await paymentService.approveStandardPayment(payment.id, requireDirectorApproval);
      const successMessage = getApprovalSuccessMessage(payment, result);
      toast({
        title: successMessage.title,
        description: successMessage.description,
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
      setPaymentDecisionLoadingId(null);
    }
  };

  const handleOpenRejectPayment = (payment: Payment) => {
    setSelectedPaymentForRejection(payment);
    setRejectReason('');
  };

  const handleCloseRejectPayment = () => {
    setSelectedPaymentForRejection(null);
    setRejectReason('');
  };

  const handleRejectPayment = async () => {
    if (!selectedPaymentForRejection) return;
    if (!rejectReason.trim()) {
      toast({
        title: 'Alasan rejection wajib diisi',
        description: 'Masukkan alasan agar requester tahu apa yang perlu diperbaiki.',
        status: 'warning',
        duration: 4000,
      });
      return;
    }

    setPaymentDecisionLoadingId(selectedPaymentForRejection.id);
    try {
      await paymentService.rejectPayment(selectedPaymentForRejection.id, rejectReason.trim());
      const rejectedByDirector =
        selectedPaymentForRejection.status === 'PENDING' &&
        selectedPaymentForRejection.require_director_approval;
      toast({
        title: 'Payment ditolak',
        description: rejectedByDirector
          ? `Payment ${selectedPaymentForRejection.code} sudah direject oleh director.`
          : `Payment ${selectedPaymentForRejection.code} sudah direject oleh finance manager.`,
        status: 'success',
        duration: 4000,
      });
      handleCloseRejectPayment();
      loadPayments();
    } catch (err: any) {
      toast({
        title: 'Gagal reject payment',
        description: err.message || 'Terjadi kesalahan saat reject payment',
        status: 'error',
        duration: 5000,
      });
    } finally {
      setPaymentDecisionLoadingId(null);
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

    if (payment.payment_type === 'EXPENSE' || payment.legacy_non_invoice_eligible) {
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
  const handleViewPayment = (payment: PaymentTransaction) => {
    if (payment.is_invoice_receipt && payment.sale_id) {
      router.push(`/sales/${payment.sale_id}`);
      return;
    }
    setSelectedPayment(payment);
    setShowPaymentDetail(true);
  };

  // Delete Payment handler
  const handleDeletePayment = (payment: Payment) => {
    setSelectedPayment(payment);
    setShowConfirmDelete(true);
  };

  const canEditPaymentBeforePosting = (payment: Payment) => {
    if (!canEdit || payment.payment_type?.startsWith('TAX_PPN') || payment.code?.startsWith('SETOR-PPN')) {
      return false;
    }
    if (payment.status === 'DRAFT') {
      return true;
    }
    if (payment.status === 'PENDING_ACCOUNTING') {
      return isAccountingReviewer;
    }
    if (payment.status === 'PENDING_FINAL_REVIEW') {
      return isFinanceManagerApprover || roleNorm === 'admin';
    }
    return false;
  };

  const getTransactionKindMeta = (payment: PaymentTransaction) => {
    switch (payment.transaction_kind) {
      case 'SALES_RECEIPT':
        return { label: 'Penerimaan Penjualan', colorScheme: 'green' };
      case 'CUSTOMER_RECEIPT':
        return { label: 'Penerimaan', colorScheme: 'teal' };
      case 'NON_INVOICE':
        return { label: 'Non-Invoice', colorScheme: 'purple' };
      default:
        return { label: 'Pembayaran Keluar', colorScheme: 'orange' };
    }
  };

  const columns = [
    {
      header: t('payments.code'),
      accessor: (row: PaymentTransaction) => (
        <Button
          variant="link"
          size="sm"
          color={statColors.blue}
          fontWeight="medium"
          onClick={() => handleViewPayment(row)}
          whiteSpace="normal"
          textAlign="left"
          height="auto"
          minW={0}
        >
          {row.code}
        </Button>
      )
    },
    {
      header: t('payments.contact'),
      accessor: (row: PaymentTransaction) => {
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
      header: 'Jenis',
      accessor: (row: PaymentTransaction) => {
        const kind = getTransactionKindMeta(row);
        return <Badge colorScheme={kind.colorScheme}>{kind.label}</Badge>;
      }
    },
    {
      header: 'Sumber',
      accessor: (row: PaymentTransaction) => (
        <VStack align="start" spacing={0} minW="180px">
          <Text fontSize="sm" fontWeight="medium" whiteSpace="normal">
            {row.source_label || '-'}
          </Text>
          {row.source_reference ? (
            <Text fontSize="xs" color={textSecondary}>
              {row.source_reference}
            </Text>
          ) : null}
        </VStack>
      )
    },
    {
      header: t('payments.date'),
      accessor: (row: PaymentTransaction) => formatDate(row.date)
    },
    {
      header: t('payments.amount'),
      accessor: (row: PaymentTransaction) => (
        <Text fontWeight="medium">{formatCurrency(row.amount)}</Text>
      )
    },
    {
      header: t('payments.method'),
      accessor: (row: PaymentTransaction) => paymentService.getMethodDisplayName(row.method)
    },
    {
      header: t('payments.status'),
      accessor: (row: PaymentTransaction) => {
        const statusMeta = getStatusMetaText(row);
        return (
          <VStack align="start" spacing={1}>
            <Badge colorScheme={paymentService.getStatusColorScheme(row.status)} variant="subtle">
              {getStatusBadgeLabel(row)}
            </Badge>
            {statusMeta ? (
              <Text fontSize="xs" color={textSecondary}>
                {statusMeta}
              </Text>
            ) : null}
            {row.rejection_reason ? (
              <Text fontSize="xs" color="red.500">
                {row.rejection_reason}
              </Text>
            ) : null}
          </VStack>
        );
      }
    },
    {
      header: 'Status PPh',
      accessor: (row: PaymentTransaction) => {
        const pphStatus = getPaymentPPhStatus(row);
        return (
          <VStack align="start" spacing={1}>
            <Badge colorScheme={pphStatus.colorScheme} variant="subtle">
              {pphStatus.label}
            </Badge>
            {pphStatus.totalAmount > 0 ? (
              <>
                <Text fontSize="xs" color={textSecondary}>
                  PPh: {formatCurrency(pphStatus.totalAmount)}
                </Text>
                {pphStatus.outstandingAmount > 0 ? (
                  <Text fontSize="xs" color="orange.500">
                    Sisa: {formatCurrency(pphStatus.outstandingAmount)}
                  </Text>
                ) : null}
              </>
            ) : null}
          </VStack>
        );
      }
    },
    {
      header: t('payments.actions'),
      accessor: (row: PaymentTransaction) => (
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
              {row.is_invoice_receipt ? 'Buka Penjualan' : t('payments.viewDetails')}
            </MenuItem>
            {!row.is_invoice_receipt && (
              <>
            {/* Finance manager approval lane */}
            {isFinanceManagerApprover && row.status === 'DRAFT' && (
              <>
                <MenuDivider />
                <MenuItem
                  icon={<FiArrowRight />}
                  color="green.500"
                  isDisabled={paymentDecisionLoadingId === row.id}
                  onClick={() => {
                    // Route approval based on payment type
                    if (row.payment_type === 'EXPENSE') {
                      handleApproveExpensePayment(row);
                    } else {
                      handleApproveStandardPayment(row);
                    }
                  }}
                >
                  {paymentDecisionLoadingId === row.id ? 'Menyetujui...' : 'Setujui'}
                </MenuItem>
                <MenuItem
                  icon={<FiArrowRight />}
                  color="blue.500"
                  isDisabled={paymentDecisionLoadingId === row.id}
                  onClick={() => {
                    if (row.payment_type === 'EXPENSE') {
                      handleApproveExpensePayment(row, true);
                    } else {
                      handleApproveStandardPayment(row, true);
                    }
                  }}
                >
                  {paymentDecisionLoadingId === row.id ? 'Mengirim...' : 'Setujui & Kirim ke Director'}
                </MenuItem>
                <MenuItem
                  icon={<FiTrash2 />}
                  color="red.500"
                  isDisabled={paymentDecisionLoadingId === row.id}
                  onClick={() => handleOpenRejectPayment(row)}
                >
                  Reject Payment
                </MenuItem>
                <MenuDivider />
              </>
            )}
            {/* Final finance manager posting lane */}
            {isFinanceManagerApprover && row.status === 'PENDING_FINAL_REVIEW' && (
              <>
                <MenuDivider />
                <MenuItem
                  icon={<FiArrowRight />}
                  color="green.500"
                  isDisabled={paymentDecisionLoadingId === row.id}
                  onClick={() => {
                    if (row.payment_type === 'EXPENSE') {
                      handleApproveExpensePayment(row);
                    } else {
                      handleApproveStandardPayment(row);
                    }
                  }}
                >
                  {paymentDecisionLoadingId === row.id ? 'Posting...' : 'Review & Posting Jurnal'}
                </MenuItem>
                <MenuItem
                  icon={<FiTrash2 />}
                  color="red.500"
                  isDisabled={paymentDecisionLoadingId === row.id}
                  onClick={() => handleOpenRejectPayment(row)}
                >
                  Reject Payment
                </MenuItem>
                <MenuDivider />
              </>
            )}
            {/* Director approval lane */}
            {isDirectorApprover && row.status === 'PENDING' && row.require_director_approval && (
              <>
                <MenuDivider />
                <MenuItem
                  icon={<FiArrowRight />}
                  color="green.500"
                  isDisabled={paymentDecisionLoadingId === row.id}
                  onClick={() => {
                    if (row.payment_type === 'EXPENSE') {
                      handleApproveExpensePayment(row);
                    } else {
                      handleApproveStandardPayment(row);
                    }
                  }}
                >
                  {paymentDecisionLoadingId === row.id ? 'Menyetujui...' : 'Approve Director'}
                </MenuItem>
                <MenuItem
                  icon={<FiTrash2 />}
                  color="red.500"
                  isDisabled={paymentDecisionLoadingId === row.id}
                  onClick={() => handleOpenRejectPayment(row)}
                >
                  Reject Payment
                </MenuItem>
                <MenuDivider />
              </>
            )}
            {canEditPaymentBeforePosting(row) && (
              <MenuItem icon={<FiEdit />} onClick={() => handleEditPayment(row)}>
                {row.status === 'PENDING_ACCOUNTING'
                  ? row.payment_type === 'EXPENSE' && !row.expense_account_id
                    ? 'Set COA Kontra'
                    : row.legacy_non_invoice_eligible
                      ? 'Lengkapi COA Kontra (Legacy)'
                    : 'Set COA / Pembayaran'
                  : row.status === 'PENDING_FINAL_REVIEW'
                    ? 'Ubah / Review COA'
                    : t('payments.edit')}
              </MenuItem>
            )}
            <MenuItem
              icon={<FiFilePlus />}
              onClick={() => paymentService.downloadPaymentDetailPDF(row.id, row.code)}
            >
              {t('payments.exportPDF')}
            </MenuItem>
            {canDelete && !isCancelledPaymentStatus(row.status) && (
              <>
                <MenuDivider />
                <MenuItem icon={<FiTrash2 />} color="red.500" onClick={() => handleDeletePayment(row)}>
                  {t('payments.delete')}
                </MenuItem>
              </>
            )}
              </>
            )}
          </MenuList>
        </Menu>
      )
    }
  ];

  // Load payments data
  const loadPayments = async (newFilters?: Partial<PaymentTransactionFilters>) => {
    try {
      setLoading(true);
      setError(null);

      const currentFilters = newFilters ? { ...filters, ...newFilters } : filters;
      const page = Math.max(1, Number(currentFilters.page) || 1);
      const limit = Math.max(1, Number(currentFilters.limit) || ITEMS_PER_PAGE);
      const searchTerm =
        typeof currentFilters.search === 'string'
          ? currentFilters.search.trim()
          : searchInput.trim();

      // Prepare filters for API request
      const apiFilters: PaymentTransactionFilters = {
        page,
        limit
      };

      if (transactionFilter !== 'ALL') {
        apiFilters.kind = transactionFilter;
      }

      if (searchTerm) {
        apiFilters.search = searchTerm;
      }

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
      const result = await paymentService.getPaymentTransactions(apiFilters);
      const normalizedResult = result as PaymentTransactionResult & { totalPages?: number };

      const totalItems = Number(result?.total || 0);
      const totalPages = Math.max(
        1,
        Number(
          normalizedResult?.total_pages ||
          normalizedResult?.totalPages ||
          Math.ceil(totalItems / limit) ||
          1
        )
      );
      const resultPage = Math.min(
        Math.max(1, Number(result?.page || page)),
        totalPages
      );
      const resultLimit = Math.max(1, Number(result?.limit || limit));

      // Store current server page for display and page-level summary.
      const paymentData = result?.data || [];
      setAllPayments(paymentData);
      setPayments(paymentData);
      setFilters({
        ...currentFilters,
        page: resultPage,
        limit: resultLimit,
        search: searchTerm || undefined
      });
      setPagination({
        current: resultPage,
        total: totalPages,
        totalItems
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
      const pendingPayments = dataSource.filter(p => p.status === 'PENDING' || p.status === 'DRAFT');
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
  }, [statusFilter, methodFilter, startDate, endDate, transactionFilter]);

  // Server-side search keeps transactions from other pages searchable.
  useEffect(() => {
    if (!token) return;

    if (!hasMountedSearchEffect.current) {
      hasMountedSearchEffect.current = true;
      return;
    }

    const timer = window.setTimeout(() => {
      loadPayments({
        page: 1,
        search: searchInput.trim() || undefined
      });
    }, 350);

    return () => window.clearTimeout(timer);
    // loadPayments intentionally reads the latest filter state from this render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchInput, token]);

  // Handle page change
  const handlePageChange = (page: number) => {
    const totalPages = Math.max(1, pagination.total);
    const nextPage = Math.min(Math.max(1, page), totalPages);
    if (nextPage === pagination.current || loading) return;
    loadPayments({ page: nextPage });
  };

  const handlePageSizeChange = (limit: number) => {
    loadPayments({
      page: 1,
      limit
    });
  };

  const getVisiblePaymentPageNumbers = () => {
    const totalPages = Math.max(1, pagination.total);
    const currentPage = Math.min(Math.max(1, pagination.current), totalPages);
    const maxButtons = 5;
    const startPage = Math.max(
      1,
      Math.min(currentPage - Math.floor(maxButtons / 2), totalPages - maxButtons + 1)
    );
    const endPage = Math.min(totalPages, startPage + maxButtons - 1);
    return Array.from({ length: endPage - startPage + 1 }, (_, index) => startPage + index);
  };

  const handleSearch = (value: string) => {
    setSearchInput(value);
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
      case 'kind':
        setTransactionFilter(value as PaymentTransactionFilter);
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
    const hadActiveFilters =
      statusFilter !== 'ALL' ||
      methodFilter !== 'ALL' ||
      transactionFilter !== 'ALL' ||
      !!startDate ||
      !!endDate ||
      !!searchInput.trim();

    setStatusFilter('ALL');
    setMethodFilter('ALL');
    setTransactionFilter('ALL');
    setStartDate('');
    setEndDate('');
    setSearchInput('');
    setFilters({
      page: 1,
      limit: ITEMS_PER_PAGE
    });

    if (!hadActiveFilters) {
      loadPayments({ page: 1, limit: ITEMS_PER_PAGE, search: undefined });
    }
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
        <SimpleLayout>
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
        <SimpleLayout>
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
                  <Heading as="h1" size="xl" mb={2} color={headingColor}>Transaksi Pembayaran & Penerimaan</Heading>
                  <Text color={textSecondary}>Pembayaran keluar, transaksi non-invoice, dan penerimaan dari penjualan</Text>
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
                  {(isFinanceManagerApprover || isAccountingReviewer || roleNorm === 'finance_staff' || roleNorm === 'admin') && (
                    <Button
                      leftIcon={<FiEdit />}
                      variant="outline"
                      colorScheme="orange"
                      size="md"
                      onClick={() => setShowCorrectionQueue(true)}
                    >
                      Koreksi Transaksi
                    </Button>
                  )}
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
                          icon={<FiArrowRight />}
                          onClick={() => handleNewPayment('payable')}
                        >
                          {t('payments.payablePayment')}
                        </MenuItem>
                        <MenuItem
                          icon={<FiBriefcase />}
                          onClick={() => setShowPurchaseTermPayment(true)}
                        >
                          {t('payments.purchaseTermPayment')}
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
                  <Tabs
                    variant="soft-rounded"
                    colorScheme="blue"
                    index={['ALL', 'RECEIPT', 'OUTGOING', 'NON_INVOICE'].indexOf(transactionFilter)}
                    onChange={(index) => handleFilterChange('kind', ['ALL', 'RECEIPT', 'OUTGOING', 'NON_INVOICE'][index])}
                    mb={4}
                  >
                    <TabList gap={2} flexWrap="wrap">
                      <Tab>Semua</Tab>
                      <Tab>Penerimaan</Tab>
                      <Tab>Pembayaran Keluar</Tab>
                      <Tab>Non-Invoice</Tab>
                    </TabList>
                  </Tabs>
                  <HStack spacing={4} wrap="wrap">
                    <InputGroup maxW="400px">
                      <InputLeftElement pointerEvents="none">
                        <FiSearch color={searchIconColor} />
                      </InputLeftElement>
                      <Input
                        placeholder="Cari kode, kontak, invoice, atau referensi"
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
                      <option value="PENDING_ACCOUNTING">Menunggu Accounting</option>
                      <option value="PENDING_FINAL_REVIEW">Menunggu Review FM</option>
                      <option value="COMPLETED">{t('payments.filters.completed')}</option>
                      <option value="DRAFT">{t('payments.filters.draft')}</option>
                      <option value="REJECTED">Rejected</option>
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
                    <Heading size="md" color={headingColor}>
                      Transaksi Pembayaran & Penerimaan ({pagination.totalItems || payments?.length || 0})
                    </Heading>
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
                      keyField="row_id"
                      searchable={false}
                      pagination={false}
                    />
                  )}
                  {!loading && pagination.totalItems > 0 && (
                    <Flex
                      mt={4}
                      px={4}
                      py={3}
                      borderWidth="1px"
                      borderColor={borderColor}
                      borderRadius="md"
                      bg={cardBg}
                      align={{ base: 'stretch', md: 'center' }}
                      justify="space-between"
                      direction={{ base: 'column', md: 'row' }}
                      gap={3}
                    >
                      <VStack align={{ base: 'stretch', md: 'start' }} spacing={0}>
                        <Text fontSize="sm" color={textSecondary}>
                          Menampilkan {(pagination.current - 1) * (filters.limit || ITEMS_PER_PAGE) + 1}-
                          {Math.min(
                            pagination.current * (filters.limit || ITEMS_PER_PAGE),
                            pagination.totalItems
                          )}{' '}
                          dari {pagination.totalItems} transaksi
                        </Text>
                        {pagination.total > 1 && (
                          <Text fontSize="xs" color={textSecondary}>
                            Halaman {pagination.current} dari {pagination.total}
                          </Text>
                        )}
                      </VStack>

                      <Flex align="center" gap={2} wrap="wrap">
                        <Text fontSize="sm" color={textSecondary}>
                          Rows
                        </Text>
                        <Select
                          size="sm"
                          maxW="90px"
                          value={filters.limit || ITEMS_PER_PAGE}
                          onChange={(e) => handlePageSizeChange(Number(e.target.value))}
                          isDisabled={loading}
                          bg={inputBg}
                          borderColor={borderColor}
                        >
                          <option value={10}>10</option>
                          <option value={25}>25</option>
                          <option value={50}>50</option>
                        </Select>
                        <Button
                          size="sm"
                          variant="outline"
                          leftIcon={<FiChevronsLeft />}
                          onClick={() => handlePageChange(1)}
                          isDisabled={loading || pagination.current <= 1}
                        >
                          Awal
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          leftIcon={<FiChevronLeft />}
                          onClick={() => handlePageChange(pagination.current - 1)}
                          isDisabled={loading || pagination.current <= 1}
                        >
                          Prev
                        </Button>
                        {getVisiblePaymentPageNumbers().map((pageNumber) => (
                          <Button
                            key={pageNumber}
                            size="sm"
                            minW="36px"
                            variant={pageNumber === pagination.current ? 'solid' : 'outline'}
                            colorScheme={pageNumber === pagination.current ? 'blue' : undefined}
                            onClick={() => handlePageChange(pageNumber)}
                            isDisabled={loading}
                          >
                            {pageNumber}
                          </Button>
                        ))}
                        <Button
                          size="sm"
                          variant="outline"
                          rightIcon={<FiChevronRight />}
                          onClick={() => handlePageChange(pagination.current + 1)}
                          isDisabled={loading || pagination.current >= pagination.total}
                        >
                          Next
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          rightIcon={<FiChevronsRight />}
                          onClick={() => handlePageChange(pagination.total)}
                          isDisabled={loading || pagination.current >= pagination.total}
                        >
                          Akhir
                        </Button>
                      </Flex>
                    </Flex>
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

            <PurchaseTermPaymentPicker
              isOpen={showPurchaseTermPayment}
              onClose={() => setShowPurchaseTermPayment(false)}
              onSuccess={() => loadPayments()}
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

            <Modal
              isOpen={!!selectedPaymentForRejection}
              onClose={handleCloseRejectPayment}
              size="lg"
            >
              <ModalOverlay />
              <ModalContent bg={cardBg}>
                <ModalHeader color={headingColor}>Reject Payment</ModalHeader>
                <ModalCloseButton />
                <ModalBody>
                  <VStack spacing={4} align="stretch">
                    <Alert status="warning" borderRadius="md">
                      <AlertIcon />
                      <AlertDescription>
                        Payment akan ditandai rejected dan tidak diposting ke jurnal. Isi alasan rejection agar requester bisa revisi dengan jelas.
                      </AlertDescription>
                    </Alert>
                    {selectedPaymentForRejection && (
                      <Box p={3} borderWidth="1px" borderColor={borderColor} borderRadius="md">
                        <Text fontWeight="bold" color={headingColor}>{selectedPaymentForRejection.code}</Text>
                        <Text fontSize="sm" color={textSecondary}>
                          {formatCurrency(selectedPaymentForRejection.amount)} • {selectedPaymentForRejection.contact?.name || '-'}
                        </Text>
                      </Box>
                    )}
                    <FormControl isRequired>
                      <FormLabel>Alasan Rejection</FormLabel>
                      <Textarea
                        value={rejectReason}
                        onChange={(e) => setRejectReason(e.target.value)}
                        placeholder="Contoh: nominal belum sesuai invoice, rekening pembayaran salah, atau dokumen pendukung belum lengkap."
                        rows={4}
                      />
                    </FormControl>
                  </VStack>
                </ModalBody>
                <ModalFooter>
                  <Button variant="ghost" mr={3} onClick={handleCloseRejectPayment}>
                    Batal
                  </Button>
                  <Button
                    colorScheme="red"
                    onClick={handleRejectPayment}
                    isLoading={
                      selectedPaymentForRejection
                        ? paymentDecisionLoadingId === selectedPaymentForRejection.id
                        : false
                    }
                  >
                    Reject Payment
                  </Button>
                </ModalFooter>
              </ModalContent>
            </Modal>

            {/* Payment Detail Modal */}
            <PaymentDetailModal
              payment={selectedPayment}
              isOpen={showPaymentDetail}
              onClose={() => {
                setShowPaymentDetail(false);
                setSelectedPayment(null);
              }}
            />

            <PostedTransactionCorrectionsQueueModal
              isOpen={showCorrectionQueue}
              onClose={() => setShowCorrectionQueue(false)}
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
