'use client';

import React, { useCallback, useEffect, useState } from 'react';
import ProtectedModule from '@/components/common/ProtectedModule';
import SimpleLayout from '@/components/layout/SimpleLayout';
import { useAuth } from '@/contexts/AuthContext';
import { useModulePermissions } from '@/hooks/usePermissions';
import cashBankService, { CashBank } from '@/services/cashbankService';
import payrollService, {
  PayrollImportPreview,
  PayrollItem,
  PayrollPPhPaymentStatus,
  PayrollRun,
  PayrollStatus,
} from '@/services/payrollService';
import {
  isAccountingRole,
  isFinanceManagerRole,
  isFinanceStaffRole,
  isHRRole,
  normalizeRole,
} from '@/utils/roles';
import {
  Alert,
  AlertDescription,
  AlertIcon,
  Badge,
  Box,
  Button,
  Card,
  CardBody,
  Flex,
  FormControl,
  FormLabel,
  HStack,
  Heading,
  Icon,
  IconButton,
  Input,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  Select,
  SimpleGrid,
  Spinner,
  Stat,
  StatHelpText,
  StatLabel,
  StatNumber,
  Table,
  TableContainer,
  Tbody,
  Td,
  Text,
  Textarea,
  Th,
  Thead,
  Tr,
  VStack,
  useColorModeValue,
  useDisclosure,
  useToast,
} from '@chakra-ui/react';
import {
  FiCheckCircle,
  FiDollarSign,
  FiDownload,
  FiEye,
  FiFileText,
  FiLock,
  FiPlus,
  FiRefreshCw,
  FiShield,
  FiTrash2,
  FiUpload,
  FiUsers,
  FiXCircle,
} from 'react-icons/fi';

const formatCurrency = (amount: number) =>
  new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(Number(amount || 0));

const formatDate = (date?: string | null) => {
  if (!date) return '-';
  const parsed = new Date(date);
  if (Number.isNaN(parsed.getTime())) return '-';
  return parsed.toLocaleDateString('id-ID');
};

const toDateInputValue = (date: Date) => {
  const year = date.getFullYear();
  const month = `${date.getMonth() + 1}`.padStart(2, '0');
  const day = `${date.getDate()}`.padStart(2, '0');
  return `${year}-${month}-${day}`;
};

const currentMonthRange = () => {
  const now = new Date();
  const start = new Date(now.getFullYear(), now.getMonth(), 1);
  const end = new Date(now.getFullYear(), now.getMonth() + 1, 0);
  return {
    period_start: toDateInputValue(start),
    period_end: toDateInputValue(end),
    payment_date: toDateInputValue(now),
  };
};

const blankPayrollItem = (): PayrollItem => ({
  employee_name: '',
  employee_code: '',
  department: '',
  position: '',
  base_salary: 0,
  bonus: 0,
  pph: 0,
  bpjs: 0,
  notes: '',
});

const statusLabel = (status: PayrollStatus | string) => {
  switch (status) {
    case 'APPROVED':
      return 'APPROVED';
    case 'REJECTED':
      return 'REJECTED';
    default:
      return 'MENUNGGU FINANCE MANAGER';
  }
};

const statusColor = (status: PayrollStatus | string) => {
  switch (status) {
    case 'APPROVED':
      return 'green';
    case 'REJECTED':
      return 'red';
    default:
      return 'orange';
  }
};

const getPPhPaymentStatus = (payroll: PayrollRun): PayrollPPhPaymentStatus => {
  const totalPPh = Number(payroll.total_pph || 0);
  const paidPPh = Number(payroll.pph_paid_amount || 0);
  if (totalPPh <= 0 || paidPPh >= totalPPh - 1) return 'PAID';
  if (paidPPh > 0) return 'PARTIAL_PAID';
  return (payroll.pph_payment_status || 'UNPAID') as PayrollPPhPaymentStatus;
};

const pphPaymentStatusLabel = (status: PayrollPPhPaymentStatus) => {
  switch (status) {
    case 'PAID':
      return 'PPh PAID';
    case 'PARTIAL_PAID':
      return 'PPh PARTIAL';
    default:
      return 'PPh UNPAID';
  }
};

const pphPaymentStatusColor = (status: PayrollPPhPaymentStatus) => {
  switch (status) {
    case 'PAID':
      return 'green';
    case 'PARTIAL_PAID':
      return 'orange';
    default:
      return 'red';
  }
};

const getPPhRemainingAmount = (payroll?: PayrollRun | null) => {
  if (!payroll) return 0;
  return Math.max(0, Number(payroll.total_pph || 0) - Number(payroll.pph_paid_amount || 0));
};

const getApiErrorMessage = (error: unknown, fallback: string) => {
  const maybeError = error as { response?: { data?: { error?: string } } };
  return maybeError.response?.data?.error || fallback;
};

const payrollImportHeaders = [
  'WORKING DAY',
  'EMPLOYEE ID',
  'WORK UNIT',
  'PAYMENT VIA',
  'ACC NO',
  'DIV',
  'EMPLOYEE NAME',
  'POSITION',
  'BASIC SALARY',
  'COMPENSATION PKWT',
  'OVERTIME',
  'BPJS TK JHT 2%',
  'BPJS TK JP 1%',
  'BPJS KESEHATAN 1%',
  'PROPORTIONAL',
  'OTHER DEDUCTION',
  'NET PAY',
];

export default function PayrollPage() {
  const { user } = useAuth();
  const payrollPermissions = useModulePermissions('payroll');
  const toast = useToast();
  const role = normalizeRole(user?.role);
  const isAdmin = role === 'admin';
  const isHR = isHRRole(role);
  const isFinanceManager = isFinanceManagerRole(role);
  const isSummaryOnly = isFinanceStaffRole(role) || isAccountingRole(role);
  const canCreatePayroll = payrollPermissions.canCreate && (isAdmin || isHR);
  const canApprovePayroll = payrollPermissions.canApprove && isFinanceManager;
  const canSeeEmployeeDetail = isAdmin || isHR || isFinanceManager;

  const [payrollRuns, setPayrollRuns] = useState<PayrollRun[]>([]);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [statusFilter, setStatusFilter] = useState('ALL');
  const [selectedPayroll, setSelectedPayroll] = useState<PayrollRun | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [rejectReason, setRejectReason] = useState('');
  const [importFile, setImportFile] = useState<File | null>(null);
  const [importPreview, setImportPreview] = useState<PayrollImportPreview | null>(null);
  const [importLoading, setImportLoading] = useState(false);
  const [cashBanks, setCashBanks] = useState<CashBank[]>([]);
  const [cashBankLoading, setCashBankLoading] = useState(false);
  const [pphPaymentDraft, setPPhPaymentDraft] = useState({
    payment_date: toDateInputValue(new Date()),
    amount: 0,
    cash_bank_id: '',
    reference: '',
    notes: '',
  });
  const [importDraft, setImportDraft] = useState({
    ...currentMonthRange(),
    notes: 'Imported from Excel payroll template',
  });
  const [draft, setDraft] = useState({
    ...currentMonthRange(),
    notes: '',
    items: [blankPayrollItem()],
  });

  const createModal = useDisclosure();
  const detailModal = useDisclosure();
  const importModal = useDisclosure();
  const pphPaymentModal = useDisclosure();

  const pageBg = useColorModeValue('gray.50', 'gray.900');
  const cardBg = useColorModeValue('white', 'gray.800');
  const mutedText = useColorModeValue('gray.600', 'gray.400');
  const borderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const tableHeaderBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const softPanelBg = useColorModeValue('blue.50', 'whiteAlpha.100');
  const inputBg = useColorModeValue('white', 'whiteAlpha.50');

  const loadPayroll = useCallback(async () => {
    setLoading(true);
    try {
      const data = await payrollService.getPayrollRuns(statusFilter);
      setPayrollRuns(data);
    } catch (error: unknown) {
      toast({
        title: 'Gagal memuat payroll',
        description: getApiErrorMessage(error, 'Data payroll belum bisa dimuat.'),
        status: 'error',
        duration: 4000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  }, [statusFilter, toast]);

  useEffect(() => {
    loadPayroll();
  }, [loadPayroll]);

  const loadCashBanks = useCallback(async () => {
    setCashBankLoading(true);
    try {
      const accounts = await cashBankService.getCashBankAccounts();
      const usableAccounts = accounts.filter(
        (account) =>
          account.is_active &&
          account.account_id &&
          (account.currency || 'IDR').toUpperCase() === 'IDR' &&
          account.allow_direct_transactions !== false,
      );
      setCashBanks(usableAccounts);
    } catch (error: unknown) {
      toast({
        title: 'Gagal memuat kas/bank',
        description: getApiErrorMessage(error, 'Akun kas/bank untuk pembayaran PPh belum bisa dimuat.'),
        status: 'error',
        duration: 4000,
        isClosable: true,
      });
    } finally {
      setCashBankLoading(false);
    }
  }, [toast]);

  const totals = payrollRuns.reduce(
    (acc, payroll) => ({
      salary: acc.salary + Number(payroll.total_salary || 0),
      bonus: acc.bonus + Number(payroll.total_bonus || 0),
      pph: acc.pph + Number(payroll.total_pph || 0),
      bpjs: acc.bpjs + Number(payroll.total_bpjs || 0),
      overall: acc.overall + Number(payroll.total_overall || 0),
    }),
    { salary: 0, bonus: 0, pph: 0, bpjs: 0, overall: 0 },
  );

  const updateDraftItem = (index: number, field: keyof PayrollItem, value: string | number) => {
    setDraft((prev) => ({
      ...prev,
      items: prev.items.map((item, itemIndex) =>
        itemIndex === index ? { ...item, [field]: value } : item,
      ),
    }));
  };

  const addDraftItem = () => {
    setDraft((prev) => ({
      ...prev,
      items: [...prev.items, blankPayrollItem()],
    }));
  };

  const removeDraftItem = (index: number) => {
    setDraft((prev) => ({
      ...prev,
      items: prev.items.length === 1 ? prev.items : prev.items.filter((_, itemIndex) => itemIndex !== index),
    }));
  };

  const resetDraft = () => {
    setDraft({
      ...currentMonthRange(),
      notes: '',
      items: [blankPayrollItem()],
    });
  };

  const resetImportState = () => {
    setImportFile(null);
    setImportPreview(null);
    setImportDraft({
      ...currentMonthRange(),
      notes: 'Imported from Excel payroll template',
    });
  };

  const closeImportModal = () => {
    importModal.onClose();
    resetImportState();
  };

  const downloadImportTemplate = async () => {
    try {
      const blob = await payrollService.downloadPayrollImportTemplate();
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = 'payroll_import_template.xlsx';
      link.click();
      URL.revokeObjectURL(url);
    } catch (error: unknown) {
      toast({
        title: 'Gagal download template',
        description: getApiErrorMessage(error, 'Template payroll belum bisa diunduh.'),
        status: 'error',
        duration: 4000,
        isClosable: true,
      });
    }
  };

  const handlePreviewImport = async () => {
    if (!importFile) {
      toast({
        title: 'File Excel belum dipilih',
        description: 'Upload file .xlsx dengan header payroll sesuai template.',
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    setImportLoading(true);
    try {
      const preview = await payrollService.previewPayrollImport(importFile);
      setImportPreview(preview);
      toast({
        title: 'Preview import berhasil',
        description: `${preview.summary.imported_rows} baris payroll siap direview.`,
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    } catch (error: unknown) {
      toast({
        title: 'Gagal preview import',
        description: getApiErrorMessage(error, 'Pastikan file memakai header template payroll.'),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setImportLoading(false);
    }
  };

  const handleSubmitImportedPayroll = async () => {
    if (!importPreview?.items.length) {
      toast({
        title: 'Belum ada data import',
        description: 'Preview file Excel terlebih dahulu sebelum submit payroll.',
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    setSubmitting(true);
    try {
      await payrollService.createPayrollRun({
        period_start: importDraft.period_start,
        period_end: importDraft.period_end,
        payment_date: importDraft.payment_date,
        notes: importDraft.notes,
        items: importPreview.items,
      });
      toast({
        title: 'Payroll import berhasil disubmit',
        description: 'Data import masuk ke antrian approval Finance Manager.',
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
      closeImportModal();
      await loadPayroll();
    } catch (error: unknown) {
      toast({
        title: 'Gagal submit payroll import',
        description: getApiErrorMessage(error, 'Payroll import belum bisa disubmit.'),
        status: 'error',
        duration: 4000,
        isClosable: true,
      });
    } finally {
      setSubmitting(false);
    }
  };

  const handleCreatePayroll = async () => {
    const cleanItems = draft.items
      .map((item) => ({
        employee_name: item.employee_name.trim(),
        employee_code: item.employee_code?.trim() || '',
        department: item.department?.trim() || '',
        position: item.position?.trim() || '',
        base_salary: Number(item.base_salary || 0),
        bonus: Number(item.bonus || 0),
        pph: Number(item.pph || 0),
        bpjs: Number(item.bpjs || 0),
        notes: item.notes?.trim() || '',
      }))
      .filter((item) => item.employee_name);

    if (!cleanItems.length) {
      toast({
        title: 'Detail payroll belum lengkap',
        description: 'Minimal isi 1 nama karyawan sebelum submit payroll.',
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    setSubmitting(true);
    try {
      await payrollService.createPayrollRun({
        period_start: draft.period_start,
        period_end: draft.period_end,
        payment_date: draft.payment_date,
        notes: draft.notes,
        items: cleanItems,
      });
      toast({
        title: 'Payroll berhasil dibuat',
        description: 'Payroll masuk ke antrian approval Finance Manager.',
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
      createModal.onClose();
      resetDraft();
      await loadPayroll();
    } catch (error: unknown) {
      toast({
        title: 'Gagal membuat payroll',
        description: getApiErrorMessage(error, 'Periksa kembali detail payroll.'),
        status: 'error',
        duration: 4000,
        isClosable: true,
      });
    } finally {
      setSubmitting(false);
    }
  };

  const openDetail = async (payroll: PayrollRun) => {
    setSelectedPayroll(payroll);
    setRejectReason('');
    detailModal.onOpen();

    setDetailLoading(true);
    try {
      const detail = await payrollService.getPayrollRun(payroll.id);
      setSelectedPayroll(detail);
    } catch (error: unknown) {
      toast({
        title: 'Gagal memuat detail payroll',
        description: getApiErrorMessage(error, 'Detail payroll belum bisa dimuat.'),
        status: 'error',
        duration: 4000,
        isClosable: true,
      });
    } finally {
      setDetailLoading(false);
    }
  };

  const openPayPPh = async (payroll: PayrollRun) => {
    const remainingPPh = getPPhRemainingAmount(payroll);
    setSelectedPayroll(payroll);
    setPPhPaymentDraft({
      payment_date: toDateInputValue(new Date()),
      amount: remainingPPh,
      cash_bank_id: '',
      reference: `PAY-PPH-${payroll.payroll_number}`,
      notes: `Pembayaran PPh payroll ${payroll.payroll_number}`,
    });
    pphPaymentModal.onOpen();
    await loadCashBanks();
  };

  const handlePayPPh = async () => {
    if (!selectedPayroll) return;

    const amount = Number(pphPaymentDraft.amount || 0);
    const remainingPPh = getPPhRemainingAmount(selectedPayroll);
    const cashBankId = Number(pphPaymentDraft.cash_bank_id || 0);
    if (!cashBankId) {
      toast({
        title: 'Akun kas/bank belum dipilih',
        description: 'Pilih akun kas/bank IDR untuk settlement hutang PPh.',
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }
    if (amount <= 0 || amount > remainingPPh + 1) {
      toast({
        title: 'Nominal PPh tidak valid',
        description: `Nominal pembayaran harus lebih besar dari 0 dan maksimal ${formatCurrency(remainingPPh)}.`,
        status: 'warning',
        duration: 3500,
        isClosable: true,
      });
      return;
    }

    setSubmitting(true);
    try {
      const updated = await payrollService.payPayrollPPh(selectedPayroll.id, {
        payment_date: pphPaymentDraft.payment_date,
        amount,
        cash_bank_id: cashBankId,
        reference: pphPaymentDraft.reference.trim(),
        notes: pphPaymentDraft.notes.trim(),
      });
      setSelectedPayroll(updated);
      pphPaymentModal.onClose();
      toast({
        title: 'PPh payroll berhasil dibayar',
        description: 'Jurnal settlement PPh dibuat pada tanggal pembayaran, sementara hutang PPh tetap diakui pada periode payroll.',
        status: 'success',
        duration: 4500,
        isClosable: true,
      });
      await loadPayroll();
    } catch (error: unknown) {
      toast({
        title: 'Gagal bayar PPh payroll',
        description: getApiErrorMessage(error, 'Pembayaran PPh payroll belum bisa diproses.'),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setSubmitting(false);
    }
  };

  const approveSelected = async () => {
    if (!selectedPayroll) return;
    setSubmitting(true);
    try {
      const updated = await payrollService.approvePayrollRun(selectedPayroll.id);
      setSelectedPayroll(updated);
      toast({
        title: 'Payroll approved',
        description: 'Payroll sudah disetujui Finance Manager.',
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
      await loadPayroll();
    } catch (error: unknown) {
      toast({
        title: 'Gagal approve payroll',
        description: getApiErrorMessage(error, 'Payroll belum bisa di-approve.'),
        status: 'error',
        duration: 4000,
        isClosable: true,
      });
    } finally {
      setSubmitting(false);
    }
  };

  const rejectSelected = async () => {
    if (!selectedPayroll) return;
    if (!rejectReason.trim()) {
      toast({
        title: 'Catatan reject wajib diisi',
        description: 'Masukkan alasan reject agar HR bisa melakukan revisi.',
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    setSubmitting(true);
    try {
      const updated = await payrollService.rejectPayrollRun(selectedPayroll.id, rejectReason);
      setSelectedPayroll(updated);
      toast({
        title: 'Payroll rejected',
        description: 'Payroll dikembalikan ke HR untuk revisi.',
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
      await loadPayroll();
    } catch (error: unknown) {
      toast({
        title: 'Gagal reject payroll',
        description: getApiErrorMessage(error, 'Payroll belum bisa di-reject.'),
        status: 'error',
        duration: 4000,
        isClosable: true,
      });
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <ProtectedModule module="payroll">
      <SimpleLayout allowedRoles={['admin', 'hr', 'finance', 'finance_manager', 'finance_staff', 'accounting']}>
        <Box bg={pageBg} minH="100vh" p={6}>
          <VStack align="stretch" spacing={6}>
            <Flex justify="space-between" align={{ base: 'stretch', md: 'center' }} gap={4} direction={{ base: 'column', md: 'row' }}>
              <Box>
                <HStack spacing={3} mb={2}>
                  <Icon as={FiUsers} boxSize={7} color="blue.500" />
                  <Heading size="xl">Payroll</Heading>
                </HStack>
                <Text color={mutedText}>
                  HR membuat payroll, Finance Manager approval, Accounting dan Finance Staff hanya melihat summary.
                </Text>
              </Box>
              <HStack>
                <Button leftIcon={<FiRefreshCw />} variant="outline" onClick={loadPayroll} isLoading={loading}>
                  Refresh
                </Button>
                {canCreatePayroll && (
                  <>
                    <Button leftIcon={<FiDownload />} variant="outline" onClick={downloadImportTemplate}>
                      Download Template
                    </Button>
                    <Button leftIcon={<FiUpload />} colorScheme="teal" onClick={importModal.onOpen}>
                      Import Excel
                    </Button>
                    <Button leftIcon={<FiPlus />} colorScheme="blue" onClick={createModal.onOpen}>
                      Create Payroll
                    </Button>
                  </>
                )}
              </HStack>
            </Flex>

            {isSummaryOnly && (
              <Alert status="info" borderRadius="xl" bg={softPanelBg}>
                <AlertIcon />
                <AlertDescription>
                  Akses role {isAccountingRole(role) ? 'Accounting' : 'Finance Staff'} dibatasi ke summary payroll saja. Detail per karyawan disembunyikan dari API dan UI.
                </AlertDescription>
              </Alert>
            )}

            <SimpleGrid columns={{ base: 1, md: 2, xl: 5 }} spacing={4}>
              <Card bg={cardBg} borderWidth="1px" borderColor={borderColor}>
                <CardBody>
                  <Stat>
                    <StatLabel>Total Salary</StatLabel>
                    <StatNumber>{formatCurrency(totals.salary)}</StatNumber>
                    <StatHelpText>Gaji pokok payroll terfilter</StatHelpText>
                  </Stat>
                </CardBody>
              </Card>
              <Card bg={cardBg} borderWidth="1px" borderColor={borderColor}>
                <CardBody>
                  <Stat>
                    <StatLabel>Total Bonus</StatLabel>
                    <StatNumber color="green.500">{formatCurrency(totals.bonus)}</StatNumber>
                    <StatHelpText>Bonus/allowance</StatHelpText>
                  </Stat>
                </CardBody>
              </Card>
              <Card bg={cardBg} borderWidth="1px" borderColor={borderColor}>
                <CardBody>
                  <Stat>
                    <StatLabel>Total PPh</StatLabel>
                    <StatNumber color="orange.500">{formatCurrency(totals.pph)}</StatNumber>
                    <StatHelpText>Potongan pajak</StatHelpText>
                  </Stat>
                </CardBody>
              </Card>
              <Card bg={cardBg} borderWidth="1px" borderColor={borderColor}>
                <CardBody>
                  <Stat>
                    <StatLabel>Total BPJS</StatLabel>
                    <StatNumber color="purple.500">{formatCurrency(totals.bpjs)}</StatNumber>
                    <StatHelpText>Potongan BPJS</StatHelpText>
                  </Stat>
                </CardBody>
              </Card>
              <Card bg={cardBg} borderWidth="1px" borderColor={borderColor}>
                <CardBody>
                  <Stat>
                    <StatLabel>Total Overall Payroll</StatLabel>
                    <StatNumber color="blue.600">{formatCurrency(totals.overall)}</StatNumber>
                    <StatHelpText>Salary + bonus - PPh - BPJS</StatHelpText>
                  </Stat>
                </CardBody>
              </Card>
            </SimpleGrid>

            <Card bg={cardBg} borderWidth="1px" borderColor={borderColor} overflow="hidden">
              <CardBody p={0}>
                <Flex p={4} justify="space-between" align={{ base: 'stretch', md: 'center' }} gap={3} direction={{ base: 'column', md: 'row' }}>
                  <Box>
                    <Heading size="md">Payroll Runs</Heading>
                    <Text color={mutedText} fontSize="sm">
                      {payrollRuns.length} payroll ditampilkan
                    </Text>
                  </Box>
                  <Select maxW={{ base: '100%', md: '240px' }} value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)} bg={inputBg}>
                    <option value="ALL">Semua Status</option>
                    <option value="PENDING_APPROVAL">Menunggu Approval</option>
                    <option value="APPROVED">Approved</option>
                    <option value="REJECTED">Rejected</option>
                  </Select>
                </Flex>

                {loading ? (
                  <Flex justify="center" align="center" minH="260px">
                    <Spinner size="xl" thickness="4px" color="blue.500" />
                  </Flex>
                ) : (
                  <TableContainer>
                    <Table variant="simple">
                      <Thead bg={tableHeaderBg}>
                        <Tr>
                          <Th>No Payroll</Th>
                          <Th>Periode</Th>
                          <Th>Tgl Bayar</Th>
                          <Th isNumeric>Total Salary</Th>
                          <Th isNumeric>Total Bonus</Th>
                          <Th isNumeric>Total PPh</Th>
                          <Th>Status PPh</Th>
                          <Th isNumeric>Total BPJS</Th>
                          <Th isNumeric>Total Overall</Th>
                          <Th>Status</Th>
                          {canSeeEmployeeDetail && <Th>Aksi</Th>}
                        </Tr>
                      </Thead>
                      <Tbody>
                        {payrollRuns.map((payroll) => (
                          <Tr key={payroll.id}>
                            <Td fontWeight="semibold">{payroll.payroll_number}</Td>
                            <Td>{formatDate(payroll.period_start)} - {formatDate(payroll.period_end)}</Td>
                            <Td>{formatDate(payroll.payment_date)}</Td>
                            <Td isNumeric>{formatCurrency(payroll.total_salary)}</Td>
                            <Td isNumeric>{formatCurrency(payroll.total_bonus)}</Td>
                            <Td isNumeric>{formatCurrency(payroll.total_pph)}</Td>
                            <Td>
                              <VStack align="start" spacing={1}>
                                <Badge colorScheme={pphPaymentStatusColor(getPPhPaymentStatus(payroll))}>
                                  {pphPaymentStatusLabel(getPPhPaymentStatus(payroll))}
                                </Badge>
                                {Number(payroll.total_pph || 0) > 0 && (
                                  <Text fontSize="xs" color={mutedText}>
                                    Dibayar {formatCurrency(Number(payroll.pph_paid_amount || 0))}
                                  </Text>
                                )}
                              </VStack>
                            </Td>
                            <Td isNumeric>{formatCurrency(payroll.total_bpjs)}</Td>
                            <Td isNumeric fontWeight="bold" color="blue.600">
                              {formatCurrency(payroll.total_overall)}
                            </Td>
                            <Td>
                              <Badge colorScheme={statusColor(payroll.status)}>{statusLabel(payroll.status)}</Badge>
                            </Td>
                            {canSeeEmployeeDetail && (
                              <Td>
                                <HStack spacing={2}>
                                  <IconButton
                                    aria-label="Lihat detail payroll"
                                    icon={<FiEye />}
                                    size="sm"
                                    variant="ghost"
                                    colorScheme="blue"
                                    onClick={() => openDetail(payroll)}
                                  />
                                  {canApprovePayroll && payroll.status === 'APPROVED' && getPPhRemainingAmount(payroll) > 0 && (
                                    <Button
                                      leftIcon={<FiDollarSign />}
                                      size="sm"
                                      colorScheme="orange"
                                      variant="outline"
                                      onClick={() => openPayPPh(payroll)}
                                    >
                                      Pay PPh
                                    </Button>
                                  )}
                                </HStack>
                              </Td>
                            )}
                          </Tr>
                        ))}
                        {payrollRuns.length === 0 && (
                          <Tr>
                            <Td colSpan={canSeeEmployeeDetail ? 11 : 10}>
                              <Flex py={10} align="center" justify="center" direction="column" gap={2} color={mutedText}>
                                <Icon as={FiFileText} boxSize={8} />
                                <Text>Belum ada data payroll untuk filter ini.</Text>
                              </Flex>
                            </Td>
                          </Tr>
                        )}
                      </Tbody>
                    </Table>
                  </TableContainer>
                )}
              </CardBody>
            </Card>
          </VStack>

          <Modal isOpen={importModal.isOpen} onClose={closeImportModal} size="full" scrollBehavior="inside">
            <ModalOverlay />
            <ModalContent>
              <ModalHeader bgGradient="linear(to-r, teal.50, blue.50)" borderBottomWidth="1px" borderColor={borderColor}>
                <HStack>
                  <Icon as={FiUpload} color="teal.500" />
                  <Box>
                    <Text>Import Payroll via Excel</Text>
                    <Text fontSize="sm" color={mutedText} fontWeight="normal">
                      Upload file dengan header template, review grid, lalu submit ke Finance Manager.
                    </Text>
                  </Box>
                </HStack>
              </ModalHeader>
              <ModalCloseButton />
              <ModalBody py={6}>
                <VStack align="stretch" spacing={5}>
                  <Alert status="info" borderRadius="xl">
                    <AlertIcon />
                    <AlertDescription>
                      Header wajib: {payrollImportHeaders.join(', ')}. Gunakan sheet bernama <b>Salary</b> jika ada; jika tidak, sistem membaca sheet pertama.
                    </AlertDescription>
                  </Alert>

                  <SimpleGrid columns={{ base: 1, md: 3 }} spacing={4}>
                    <FormControl isRequired>
                      <FormLabel>Period Start</FormLabel>
                      <Input type="date" value={importDraft.period_start} onChange={(event) => setImportDraft((prev) => ({ ...prev, period_start: event.target.value }))} bg={inputBg} />
                    </FormControl>
                    <FormControl isRequired>
                      <FormLabel>Period End</FormLabel>
                      <Input type="date" value={importDraft.period_end} onChange={(event) => setImportDraft((prev) => ({ ...prev, period_end: event.target.value }))} bg={inputBg} />
                    </FormControl>
                    <FormControl>
                      <FormLabel>Payment Date</FormLabel>
                      <Input type="date" value={importDraft.payment_date} onChange={(event) => setImportDraft((prev) => ({ ...prev, payment_date: event.target.value }))} bg={inputBg} />
                    </FormControl>
                  </SimpleGrid>

                  <SimpleGrid columns={{ base: 1, lg: 2 }} spacing={4}>
                    <FormControl isRequired>
                      <FormLabel>File Excel Payroll</FormLabel>
                      <Input
                        type="file"
                        accept=".xlsx,.xlsm"
                        p={1}
                        bg={inputBg}
                        onChange={(event) => {
                          setImportFile(event.target.files?.[0] || null);
                          setImportPreview(null);
                        }}
                      />
                      <Text fontSize="sm" color={mutedText} mt={1}>
                        Format didukung: .xlsx atau .xlsm
                      </Text>
                    </FormControl>
                    <FormControl>
                      <FormLabel>Catatan Payroll</FormLabel>
                      <Textarea value={importDraft.notes} onChange={(event) => setImportDraft((prev) => ({ ...prev, notes: event.target.value }))} bg={inputBg} />
                    </FormControl>
                  </SimpleGrid>

                  <HStack>
                    <Button leftIcon={<FiDownload />} variant="outline" onClick={downloadImportTemplate}>
                      Download Template Excel
                    </Button>
                    <Button leftIcon={<FiEye />} colorScheme="teal" onClick={handlePreviewImport} isLoading={importLoading}>
                      Preview Import
                    </Button>
                  </HStack>

                  {importPreview && (
                    <>
                      <SimpleGrid columns={{ base: 1, md: 2, xl: 5 }} spacing={4}>
                        <Card variant="outline"><CardBody><Stat><StatLabel>Imported Rows</StatLabel><StatNumber>{importPreview.summary.imported_rows}</StatNumber><StatHelpText>dari {importPreview.summary.total_rows} baris</StatHelpText></Stat></CardBody></Card>
                        <Card variant="outline"><CardBody><Stat><StatLabel>Total Salary</StatLabel><StatNumber>{formatCurrency(importPreview.summary.total_salary)}</StatNumber></Stat></CardBody></Card>
                        <Card variant="outline"><CardBody><Stat><StatLabel>Total Bonus</StatLabel><StatNumber color="green.500">{formatCurrency(importPreview.summary.total_bonus)}</StatNumber></Stat></CardBody></Card>
                        <Card variant="outline"><CardBody><Stat><StatLabel>Total BPJS</StatLabel><StatNumber color="purple.500">{formatCurrency(importPreview.summary.total_bpjs)}</StatNumber></Stat></CardBody></Card>
                        <Card variant="outline"><CardBody><Stat><StatLabel>Total Overall</StatLabel><StatNumber color="blue.600">{formatCurrency(importPreview.summary.total_overall)}</StatNumber></Stat></CardBody></Card>
                      </SimpleGrid>

                      {importPreview.warnings.length > 0 && (
                        <Alert status="warning" borderRadius="xl">
                          <AlertIcon />
                          <AlertDescription>{importPreview.warnings.join(' | ')}</AlertDescription>
                        </Alert>
                      )}

                      <Box>
                        <Heading size="sm" mb={3}>Review Grid Payroll Import</Heading>
                        <TableContainer borderWidth="1px" borderColor={borderColor} borderRadius="xl" maxH="460px" overflowY="auto">
                          <Table size="sm">
                            <Thead bg={tableHeaderBg} position="sticky" top={0} zIndex={1}>
                              <Tr>
                                <Th>Working Day</Th>
                                <Th>Employee ID</Th>
                                <Th>Work Unit</Th>
                                <Th>Payment Via</Th>
                                <Th>Acc No</Th>
                                <Th>Div</Th>
                                <Th>Employee Name</Th>
                                <Th>Position</Th>
                                <Th isNumeric>Basic Salary</Th>
                                <Th isNumeric>Comp. PKWT</Th>
                                <Th isNumeric>Overtime</Th>
                                <Th isNumeric>BPJS JHT</Th>
                                <Th isNumeric>BPJS JP</Th>
                                <Th isNumeric>BPJS Kes.</Th>
                                <Th isNumeric>Proportional</Th>
                                <Th isNumeric>Other Deduction</Th>
                                <Th isNumeric>Net Pay</Th>
                              </Tr>
                            </Thead>
                            <Tbody>
                              {importPreview.items.map((item, index) => (
                                <Tr key={`${item.employee_code || item.employee_name}-${index}`}>
                                  <Td>{item.working_day || '-'}</Td>
                                  <Td>{item.employee_code || '-'}</Td>
                                  <Td>{item.work_unit || '-'}</Td>
                                  <Td>{item.payment_via || '-'}</Td>
                                  <Td>{item.account_no || '-'}</Td>
                                  <Td>{item.division || '-'}</Td>
                                  <Td fontWeight="semibold">{item.employee_name}</Td>
                                  <Td>{item.position || '-'}</Td>
                                  <Td isNumeric>{formatCurrency(item.base_salary)}</Td>
                                  <Td isNumeric>{formatCurrency(item.compensation_pkwt || 0)}</Td>
                                  <Td isNumeric>{formatCurrency(item.overtime || 0)}</Td>
                                  <Td isNumeric>{formatCurrency(item.bpjs_tk_jht || 0)}</Td>
                                  <Td isNumeric>{formatCurrency(item.bpjs_tk_jp || 0)}</Td>
                                  <Td isNumeric>{formatCurrency(item.bpjs_kesehatan || 0)}</Td>
                                  <Td isNumeric>{formatCurrency(item.proportional || 0)}</Td>
                                  <Td isNumeric>{formatCurrency(item.other_deduction || 0)}</Td>
                                  <Td isNumeric fontWeight="bold" color="blue.600">{formatCurrency(item.net_pay || 0)}</Td>
                                </Tr>
                              ))}
                            </Tbody>
                          </Table>
                        </TableContainer>
                      </Box>
                    </>
                  )}
                </VStack>
              </ModalBody>
              <ModalFooter borderTopWidth="1px" borderColor={borderColor}>
                <Button variant="ghost" mr={3} onClick={closeImportModal}>
                  Batal
                </Button>
                <Button colorScheme="blue" onClick={handleSubmitImportedPayroll} isLoading={submitting} isDisabled={!importPreview?.items.length}>
                  Submit Import ke Finance Manager
                </Button>
              </ModalFooter>
            </ModalContent>
          </Modal>

          <Modal isOpen={createModal.isOpen} onClose={createModal.onClose} size="full" scrollBehavior="inside">
            <ModalOverlay />
            <ModalContent>
              <ModalHeader bgGradient="linear(to-r, blue.50, cyan.50)" borderBottomWidth="1px" borderColor={borderColor}>
                <HStack>
                  <Icon as={FiShield} color="blue.500" />
                  <Box>
                    <Text>Create Payroll</Text>
                    <Text fontSize="sm" color={mutedText} fontWeight="normal">
                      Input detail payroll oleh HR, lalu submit ke Finance Manager.
                    </Text>
                  </Box>
                </HStack>
              </ModalHeader>
              <ModalCloseButton />
              <ModalBody py={6}>
                <VStack align="stretch" spacing={5}>
                  <SimpleGrid columns={{ base: 1, md: 3 }} spacing={4}>
                    <FormControl isRequired>
                      <FormLabel>Period Start</FormLabel>
                      <Input type="date" value={draft.period_start} onChange={(event) => setDraft((prev) => ({ ...prev, period_start: event.target.value }))} bg={inputBg} />
                    </FormControl>
                    <FormControl isRequired>
                      <FormLabel>Period End</FormLabel>
                      <Input type="date" value={draft.period_end} onChange={(event) => setDraft((prev) => ({ ...prev, period_end: event.target.value }))} bg={inputBg} />
                    </FormControl>
                    <FormControl>
                      <FormLabel>Payment Date</FormLabel>
                      <Input type="date" value={draft.payment_date} onChange={(event) => setDraft((prev) => ({ ...prev, payment_date: event.target.value }))} bg={inputBg} />
                    </FormControl>
                  </SimpleGrid>

                  <FormControl>
                    <FormLabel>Catatan Payroll</FormLabel>
                    <Textarea value={draft.notes} onChange={(event) => setDraft((prev) => ({ ...prev, notes: event.target.value }))} placeholder="Catatan internal HR untuk payroll ini" bg={inputBg} />
                  </FormControl>

                  <Flex justify="space-between" align="center">
                    <Box>
                      <Heading size="sm">Detail Payroll Karyawan</Heading>
                      <Text color={mutedText} fontSize="sm">Detail ini hanya terlihat oleh HR, Admin, dan Finance Manager.</Text>
                    </Box>
                    <Button leftIcon={<FiPlus />} size="sm" onClick={addDraftItem}>
                      Tambah Karyawan
                    </Button>
                  </Flex>

                  <TableContainer borderWidth="1px" borderColor={borderColor} borderRadius="xl">
                    <Table size="sm">
                      <Thead bg={tableHeaderBg}>
                        <Tr>
                          <Th minW="180px">Nama</Th>
                          <Th minW="110px">Kode</Th>
                          <Th minW="140px">Departemen</Th>
                          <Th minW="140px">Posisi</Th>
                          <Th minW="140px" isNumeric>Salary</Th>
                          <Th minW="130px" isNumeric>Bonus</Th>
                          <Th minW="130px" isNumeric>PPh</Th>
                          <Th minW="130px" isNumeric>BPJS</Th>
                          <Th minW="140px" isNumeric>Net Pay</Th>
                          <Th w="50px"></Th>
                        </Tr>
                      </Thead>
                      <Tbody>
                        {draft.items.map((item, index) => {
                          const netPay = Number(item.base_salary || 0) + Number(item.bonus || 0) - Number(item.pph || 0) - Number(item.bpjs || 0);
                          return (
                            <Tr key={index}>
                              <Td>
                                <Input size="sm" value={item.employee_name} onChange={(event) => updateDraftItem(index, 'employee_name', event.target.value)} placeholder="Nama karyawan" bg={inputBg} />
                              </Td>
                              <Td>
                                <Input size="sm" value={item.employee_code || ''} onChange={(event) => updateDraftItem(index, 'employee_code', event.target.value)} placeholder="EMP001" bg={inputBg} />
                              </Td>
                              <Td>
                                <Input size="sm" value={item.department || ''} onChange={(event) => updateDraftItem(index, 'department', event.target.value)} placeholder="HR" bg={inputBg} />
                              </Td>
                              <Td>
                                <Input size="sm" value={item.position || ''} onChange={(event) => updateDraftItem(index, 'position', event.target.value)} placeholder="Staff" bg={inputBg} />
                              </Td>
                              <Td>
                                <Input size="sm" type="number" value={item.base_salary} onChange={(event) => updateDraftItem(index, 'base_salary', Number(event.target.value))} textAlign="right" bg={inputBg} />
                              </Td>
                              <Td>
                                <Input size="sm" type="number" value={item.bonus} onChange={(event) => updateDraftItem(index, 'bonus', Number(event.target.value))} textAlign="right" bg={inputBg} />
                              </Td>
                              <Td>
                                <Input size="sm" type="number" value={item.pph} onChange={(event) => updateDraftItem(index, 'pph', Number(event.target.value))} textAlign="right" bg={inputBg} />
                              </Td>
                              <Td>
                                <Input size="sm" type="number" value={item.bpjs} onChange={(event) => updateDraftItem(index, 'bpjs', Number(event.target.value))} textAlign="right" bg={inputBg} />
                              </Td>
                              <Td isNumeric fontWeight="bold">{formatCurrency(netPay)}</Td>
                              <Td>
                                <IconButton aria-label="Hapus detail" icon={<FiTrash2 />} size="sm" variant="ghost" colorScheme="red" onClick={() => removeDraftItem(index)} isDisabled={draft.items.length === 1} />
                              </Td>
                            </Tr>
                          );
                        })}
                      </Tbody>
                    </Table>
                  </TableContainer>
                </VStack>
              </ModalBody>
              <ModalFooter borderTopWidth="1px" borderColor={borderColor}>
                <Button variant="ghost" mr={3} onClick={createModal.onClose}>
                  Batal
                </Button>
                <Button colorScheme="blue" onClick={handleCreatePayroll} isLoading={submitting}>
                  Submit ke Finance Manager
                </Button>
              </ModalFooter>
            </ModalContent>
          </Modal>

          <Modal isOpen={detailModal.isOpen} onClose={detailModal.onClose} size="6xl" scrollBehavior="inside">
            <ModalOverlay />
            <ModalContent maxW={{ base: '96vw', xl: '1180px' }}>
              <ModalHeader borderBottomWidth="1px" borderColor={borderColor}>
                <Flex justify="space-between" align="center" pr={8}>
                  <Box>
                    <Text>Detail Payroll {selectedPayroll?.payroll_number}</Text>
                    <Text fontSize="sm" color={mutedText} fontWeight="normal">
                      {selectedPayroll ? `${formatDate(selectedPayroll.period_start)} - ${formatDate(selectedPayroll.period_end)}` : ''}
                    </Text>
                  </Box>
                  {selectedPayroll && <Badge colorScheme={statusColor(selectedPayroll.status)}>{statusLabel(selectedPayroll.status)}</Badge>}
                </Flex>
              </ModalHeader>
              <ModalCloseButton />
              <ModalBody py={6}>
                {detailLoading ? (
                  <Flex minH="260px" justify="center" align="center">
                    <Spinner size="xl" color="blue.500" />
                  </Flex>
                ) : selectedPayroll ? (
                  <VStack align="stretch" spacing={5}>
                    <SimpleGrid columns={{ base: 1, md: 2, xl: 6 }} spacing={4}>
                      <Card variant="outline"><CardBody><Stat><StatLabel>Total Salary</StatLabel><StatNumber>{formatCurrency(selectedPayroll.total_salary)}</StatNumber></Stat></CardBody></Card>
                      <Card variant="outline"><CardBody><Stat><StatLabel>Total Bonus</StatLabel><StatNumber color="green.500">{formatCurrency(selectedPayroll.total_bonus)}</StatNumber></Stat></CardBody></Card>
                      <Card variant="outline"><CardBody><Stat><StatLabel>Total PPh</StatLabel><StatNumber color="orange.500">{formatCurrency(selectedPayroll.total_pph)}</StatNumber></Stat></CardBody></Card>
                      <Card variant="outline">
                        <CardBody>
                          <Stat>
                            <StatLabel>Status PPh</StatLabel>
                            <StatNumber fontSize="lg">
                              <Badge colorScheme={pphPaymentStatusColor(getPPhPaymentStatus(selectedPayroll))}>
                                {pphPaymentStatusLabel(getPPhPaymentStatus(selectedPayroll))}
                              </Badge>
                            </StatNumber>
                            <StatHelpText>
                              Paid {formatCurrency(Number(selectedPayroll.pph_paid_amount || 0))} | Sisa {formatCurrency(getPPhRemainingAmount(selectedPayroll))}
                            </StatHelpText>
                          </Stat>
                        </CardBody>
                      </Card>
                      <Card variant="outline"><CardBody><Stat><StatLabel>Total BPJS</StatLabel><StatNumber color="purple.500">{formatCurrency(selectedPayroll.total_bpjs)}</StatNumber></Stat></CardBody></Card>
                      <Card variant="outline"><CardBody><Stat><StatLabel>Total Overall</StatLabel><StatNumber color="blue.600">{formatCurrency(selectedPayroll.total_overall)}</StatNumber></Stat></CardBody></Card>
                    </SimpleGrid>

                    {Number(selectedPayroll.total_pph || 0) > 0 && (
                      <Alert status="info" borderRadius="xl">
                        <AlertIcon />
                        <AlertDescription>
                          Hutang PPh payroll diakui pada periode payroll ({formatDate(selectedPayroll.pph_recognition_date || selectedPayroll.period_end)}). Tombol Pay PPh hanya mencatat settlement pada tanggal pajak benar-benar dibayar.
                        </AlertDescription>
                      </Alert>
                    )}

                    {!selectedPayroll.detail_allowed && (
                      <Alert status="info" borderRadius="xl">
                        <AlertIcon />
                        <AlertDescription>
                          Anda hanya memiliki akses summary payroll. Detail per karyawan tidak ditampilkan untuk role ini.
                        </AlertDescription>
                      </Alert>
                    )}

                    {selectedPayroll.detail_allowed && (
                      <TableContainer borderWidth="1px" borderColor={borderColor} borderRadius="xl">
                        <Table size="sm">
                          <Thead bg={tableHeaderBg}>
                            <Tr>
                              <Th>Working Day</Th>
                              <Th>Employee ID</Th>
                              <Th>Work Unit</Th>
                              <Th>Payment Via</Th>
                              <Th>Acc No</Th>
                              <Th>Div</Th>
                              <Th>Karyawan</Th>
                              <Th>Posisi</Th>
                              <Th isNumeric>Basic Salary</Th>
                              <Th isNumeric>Comp. PKWT</Th>
                              <Th isNumeric>Overtime</Th>
                              <Th isNumeric>BPJS JHT</Th>
                              <Th isNumeric>BPJS JP</Th>
                              <Th isNumeric>BPJS Kes.</Th>
                              <Th isNumeric>Proportional</Th>
                              <Th isNumeric>Other Deduction</Th>
                              <Th isNumeric>Net Pay</Th>
                            </Tr>
                          </Thead>
                          <Tbody>
                            {(selectedPayroll.items || []).map((item) => (
                              <Tr key={item.id}>
                                <Td>{item.working_day || '-'}</Td>
                                <Td>{item.employee_code || '-'}</Td>
                                <Td>{item.work_unit || item.department || '-'}</Td>
                                <Td>{item.payment_via || '-'}</Td>
                                <Td>{item.account_no || '-'}</Td>
                                <Td>{item.division || '-'}</Td>
                                <Td>
                                  <Text fontWeight="semibold">{item.employee_name}</Text>
                                  <Text fontSize="xs" color={mutedText}>{item.department || '-'}</Text>
                                </Td>
                                <Td>{item.position || '-'}</Td>
                                <Td isNumeric>{formatCurrency(item.base_salary)}</Td>
                                <Td isNumeric>{formatCurrency(item.compensation_pkwt || 0)}</Td>
                                <Td isNumeric>{formatCurrency(item.overtime || 0)}</Td>
                                <Td isNumeric>{formatCurrency(item.bpjs_tk_jht || 0)}</Td>
                                <Td isNumeric>{formatCurrency(item.bpjs_tk_jp || 0)}</Td>
                                <Td isNumeric>{formatCurrency(item.bpjs_kesehatan || 0)}</Td>
                                <Td isNumeric>{formatCurrency(item.proportional || 0)}</Td>
                                <Td isNumeric>{formatCurrency(item.other_deduction || 0)}</Td>
                                <Td isNumeric fontWeight="bold">{formatCurrency(item.net_pay || 0)}</Td>
                              </Tr>
                            ))}
                          </Tbody>
                        </Table>
                      </TableContainer>
                    )}

                    {selectedPayroll.rejection_reason && (
                      <Alert status="warning" borderRadius="xl">
                        <AlertIcon />
                        <AlertDescription>Alasan reject: {selectedPayroll.rejection_reason}</AlertDescription>
                      </Alert>
                    )}

                    {canApprovePayroll && selectedPayroll.status === 'PENDING_APPROVAL' && (
                      <Box borderWidth="1px" borderColor={borderColor} borderRadius="xl" p={4}>
                        <HStack mb={3} color={mutedText}>
                          <Icon as={FiLock} />
                          <Text fontWeight="semibold">Approval Finance Manager</Text>
                        </HStack>
                        <FormControl>
                          <FormLabel>Catatan Reject</FormLabel>
                          <Textarea value={rejectReason} onChange={(event) => setRejectReason(event.target.value)} placeholder="Wajib diisi jika payroll ditolak." bg={inputBg} />
                        </FormControl>
                      </Box>
                    )}
                  </VStack>
                ) : null}
              </ModalBody>
              <ModalFooter borderTopWidth="1px" borderColor={borderColor}>
                <Button variant="ghost" mr={3} onClick={detailModal.onClose}>
                  Tutup
                </Button>
                {selectedPayroll && canApprovePayroll && selectedPayroll.status === 'PENDING_APPROVAL' && (
                  <>
                    <Button leftIcon={<FiXCircle />} colorScheme="red" variant="outline" mr={3} onClick={rejectSelected} isLoading={submitting}>
                      Reject
                    </Button>
                    <Button leftIcon={<FiCheckCircle />} colorScheme="green" onClick={approveSelected} isLoading={submitting}>
                      Approve Payroll
                    </Button>
                  </>
                )}
                {selectedPayroll && canApprovePayroll && selectedPayroll.status === 'APPROVED' && getPPhRemainingAmount(selectedPayroll) > 0 && (
                  <Button leftIcon={<FiDollarSign />} colorScheme="orange" onClick={() => openPayPPh(selectedPayroll)} isLoading={submitting}>
                    Pay PPh
                  </Button>
                )}
              </ModalFooter>
            </ModalContent>
          </Modal>

          <Modal isOpen={pphPaymentModal.isOpen} onClose={pphPaymentModal.onClose} size="2xl" scrollBehavior="inside">
            <ModalOverlay />
            <ModalContent>
              <ModalHeader borderBottomWidth="1px" borderColor={borderColor}>
                <HStack>
                  <Icon as={FiDollarSign} color="orange.500" />
                  <Box>
                    <Text>Pay PPh Payroll</Text>
                    <Text fontSize="sm" color={mutedText} fontWeight="normal">
                      Settlement hutang PPh dapat berbeda tanggal dari periode payroll.
                    </Text>
                  </Box>
                </HStack>
              </ModalHeader>
              <ModalCloseButton />
              <ModalBody py={6}>
                {selectedPayroll && (
                  <VStack align="stretch" spacing={5}>
                    <Alert status="info" borderRadius="xl">
                      <AlertIcon />
                      <AlertDescription>
                        PPh tetap diakui sebagai hutang pajak pada periode payroll {formatDate(selectedPayroll.pph_recognition_date || selectedPayroll.period_end)}. Form ini hanya mencatat pembayaran/settlement pada tanggal bayar PPh.
                      </AlertDescription>
                    </Alert>

                    <SimpleGrid columns={{ base: 1, md: 3 }} spacing={4}>
                      <Card variant="outline">
                        <CardBody>
                          <Stat>
                            <StatLabel>Total PPh</StatLabel>
                            <StatNumber color="orange.500">{formatCurrency(selectedPayroll.total_pph)}</StatNumber>
                          </Stat>
                        </CardBody>
                      </Card>
                      <Card variant="outline">
                        <CardBody>
                          <Stat>
                            <StatLabel>Sudah Dibayar</StatLabel>
                            <StatNumber color="green.500">{formatCurrency(Number(selectedPayroll.pph_paid_amount || 0))}</StatNumber>
                          </Stat>
                        </CardBody>
                      </Card>
                      <Card variant="outline">
                        <CardBody>
                          <Stat>
                            <StatLabel>Sisa PPh</StatLabel>
                            <StatNumber color="red.500">{formatCurrency(getPPhRemainingAmount(selectedPayroll))}</StatNumber>
                          </Stat>
                        </CardBody>
                      </Card>
                    </SimpleGrid>

                    <SimpleGrid columns={{ base: 1, md: 2 }} spacing={4}>
                      <FormControl isRequired>
                        <FormLabel>Tanggal Bayar PPh</FormLabel>
                        <Input
                          type="date"
                          value={pphPaymentDraft.payment_date}
                          onChange={(event) => setPPhPaymentDraft((prev) => ({ ...prev, payment_date: event.target.value }))}
                          bg={inputBg}
                        />
                      </FormControl>
                      <FormControl isRequired>
                        <FormLabel>Nominal Bayar</FormLabel>
                        <Input
                          type="number"
                          min={0}
                          max={getPPhRemainingAmount(selectedPayroll)}
                          value={pphPaymentDraft.amount}
                          onChange={(event) => setPPhPaymentDraft((prev) => ({ ...prev, amount: Number(event.target.value) }))}
                          bg={inputBg}
                        />
                      </FormControl>
                    </SimpleGrid>

                    <FormControl isRequired>
                      <FormLabel>Akun Kas/Bank IDR</FormLabel>
                      <Select
                        placeholder={cashBankLoading ? 'Memuat kas/bank...' : 'Pilih kas/bank pembayaran PPh'}
                        value={pphPaymentDraft.cash_bank_id}
                        onChange={(event) => setPPhPaymentDraft((prev) => ({ ...prev, cash_bank_id: event.target.value }))}
                        bg={inputBg}
                        isDisabled={cashBankLoading}
                      >
                        {cashBanks.map((account) => (
                          <option key={account.id} value={account.id}>
                            {account.code} - {account.name} ({formatCurrency(account.balance)})
                          </option>
                        ))}
                      </Select>
                      <Text fontSize="sm" color={mutedText} mt={1}>
                        Hanya kas/bank IDR yang aktif dan punya GL/COA link yang ditampilkan.
                      </Text>
                    </FormControl>

                    <FormControl>
                      <FormLabel>Reference</FormLabel>
                      <Input
                        value={pphPaymentDraft.reference}
                        onChange={(event) => setPPhPaymentDraft((prev) => ({ ...prev, reference: event.target.value }))}
                        bg={inputBg}
                      />
                    </FormControl>

                    <FormControl>
                      <FormLabel>Catatan</FormLabel>
                      <Textarea
                        value={pphPaymentDraft.notes}
                        onChange={(event) => setPPhPaymentDraft((prev) => ({ ...prev, notes: event.target.value }))}
                        bg={inputBg}
                      />
                    </FormControl>
                  </VStack>
                )}
              </ModalBody>
              <ModalFooter borderTopWidth="1px" borderColor={borderColor}>
                <Button variant="ghost" mr={3} onClick={pphPaymentModal.onClose}>
                  Batal
                </Button>
                <Button
                  leftIcon={<FiDollarSign />}
                  colorScheme="orange"
                  onClick={handlePayPPh}
                  isLoading={submitting}
                  isDisabled={!selectedPayroll || cashBankLoading || cashBanks.length === 0}
                >
                  Bayar PPh
                </Button>
              </ModalFooter>
            </ModalContent>
          </Modal>
        </Box>
      </SimpleLayout>
    </ProtectedModule>
  );
}
