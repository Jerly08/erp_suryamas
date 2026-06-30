'use client';

import React, { useCallback, useEffect, useState } from 'react';
import ProtectedModule from '@/components/common/ProtectedModule';
import SimpleLayout from '@/components/layout/SimpleLayout';
import { useAuth } from '@/contexts/AuthContext';
import { useModulePermissions } from '@/hooks/usePermissions';
import cashBankService, { CashBank } from '@/services/cashbankService';
import payrollService, { PayrollBPJSPaymentStatus, PayrollImportPreview, PayrollItem, PayrollPPhPaymentStatus, PayrollRun, PayrollSalaryPaymentStatus, PayrollStatus } from '@/services/payrollService';
import { isAccountingRole, isFinanceManagerRole, isFinanceStaffRole, isHRRole, normalizeRole } from '@/utils/roles';
import { filterOperationalCashBankAccounts, filterActiveLeafAccounts } from '@/utils/accountFilters';
import accountService from '@/services/accountService';
import type { AccountCatalogItem } from '@/types/account';
import CurrencyInput from '@/components/common/CurrencyInput';
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
import { FiCheckCircle, FiDollarSign, FiDownload, FiEye, FiFileText, FiLock, FiRefreshCw, FiUpload, FiUsers, FiXCircle } from 'react-icons/fi';

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

const getPayrollItemBonus = (item: PayrollItem) => {
  const componentTotal =
    Number(item.attendance_allowance || 0) +
    Number(item.compensation_pkwt || 0) +
    Number(item.overtime || 0) +
    Number(item.proportional || 0);
  if (componentTotal > 0) return componentTotal;

  const bonus = Number(item.bonus || 0);
  if (bonus > 0) return bonus;
  return 0;
};

const getPayrollItemBPJS = (item: PayrollItem) => {
  const componentTotal = Number(item.bpjs_tk_jht || 0) + Number(item.bpjs_tk_jp || 0) + Number(item.bpjs_kesehatan || 0);
  if (componentTotal > 0) return componentTotal;
  return Number(item.bpjs || 0);
};

const getPayrollItemEmployeeLoanDeduction = (item: PayrollItem) => Number(item.employee_loan_deduction || 0);

const getPayrollItemNetPay = (item: PayrollItem) => Math.max(0, Number(item.base_salary || 0) + getPayrollItemBonus(item) - getPayrollItemBPJS(item) - Number(item.other_deduction || 0) - getPayrollItemEmployeeLoanDeduction(item));

const recalculatePayrollItem = (item: PayrollItem): PayrollItem => {
  const bpjs = getPayrollItemBPJS(item);
  return {
    ...item,
    bpjs,
    net_pay: getPayrollItemNetPay({ ...item, bpjs }),
  };
};

const payrollBPJSComponentFields: Array<keyof PayrollItem> = ['bpjs_tk_jht', 'bpjs_tk_jp', 'bpjs_kesehatan'];

const sumPayrollBPJSComponents = (item: PayrollItem) => Number(item.bpjs_tk_jht || 0) + Number(item.bpjs_tk_jp || 0) + Number(item.bpjs_kesehatan || 0);

const recalculatePayrollImportPreview = (preview: PayrollImportPreview, items: PayrollItem[]): PayrollImportPreview => ({
  ...preview,
  items,
  summary: {
    ...preview.summary,
    imported_rows: items.length,
    total_salary: items.reduce((sum, item) => sum + Number(item.base_salary || 0), 0),
    total_bonus: items.reduce((sum, item) => sum + getPayrollItemBonus(item), 0),
    total_pph: items.reduce((sum, item) => sum + Number(item.pph || 0), 0),
    total_bpjs: items.reduce((sum, item) => sum + getPayrollItemBPJS(item), 0),
    total_employee_loan_deduction: items.reduce((sum, item) => sum + getPayrollItemEmployeeLoanDeduction(item), 0),
    total_overall: items.reduce((sum, item) => sum + getPayrollItemNetPay(item), 0),
  },
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

const getSalaryPaymentStatus = (payroll: PayrollRun): PayrollSalaryPaymentStatus => {
  const total = Number(payroll.total_overall || 0);
  const paid = Number(payroll.salary_paid_amount || 0);
  if (total <= 0 || paid >= total - 1) return 'PAID';
  if (paid > 0) return 'PARTIAL_PAID';
  return (payroll.salary_payment_status || 'UNPAID') as PayrollSalaryPaymentStatus;
};

const salaryPaymentStatusLabel = (status: PayrollSalaryPaymentStatus) => {
  if (status === 'PAID') return 'GAJI PAID';
  if (status === 'PARTIAL_PAID') return 'GAJI PARTIAL';
  return 'GAJI UNPAID';
};

const salaryPaymentStatusColor = (status: PayrollSalaryPaymentStatus) => {
  if (status === 'PAID') return 'green';
  if (status === 'PARTIAL_PAID') return 'yellow';
  return 'red';
};

const getSalaryRemainingAmount = (payroll?: PayrollRun | null) => {
  if (!payroll) return 0;
  return Math.max(0, Number(payroll.total_overall || 0) - Number(payroll.salary_paid_amount || 0));
};

const getBPJSPaymentStatus = (payroll: PayrollRun): PayrollBPJSPaymentStatus => {
  const total = Number(payroll.total_bpjs || 0);
  const paid = Number(payroll.bpjs_paid_amount || 0);
  if (total <= 0 || paid >= total - 1) return 'PAID';
  if (paid > 0) return 'PARTIAL_PAID';
  return (payroll.bpjs_payment_status || 'UNPAID') as PayrollBPJSPaymentStatus;
};

const bpjsPaymentStatusLabel = (status: PayrollBPJSPaymentStatus) => {
  if (status === 'PAID') return 'BPJS PAID';
  if (status === 'PARTIAL_PAID') return 'BPJS PARTIAL';
  return 'BPJS UNPAID';
};

const bpjsPaymentStatusColor = (status: PayrollBPJSPaymentStatus) => {
  if (status === 'PAID') return 'green';
  if (status === 'PARTIAL_PAID') return 'orange';
  return 'red';
};

const getBPJSRemainingAmount = (payroll?: PayrollRun | null) => {
  if (!payroll) return 0;
  return Math.max(0, Number(payroll.total_bpjs || 0) - Number(payroll.bpjs_paid_amount || 0));
};

const getApiErrorMessage = (error: unknown, fallback: string) => {
  const maybeError = error as {
    response?: {
      data?: { code?: string; error?: string; message?: string; details?: string };
    };
  };
  const apiError = maybeError.response?.data;
  if (apiError?.code === 'PERIOD_CLOSED') {
    const closedDate = apiError.error?.replace('CLOSED_PERIOD:', '');
    return closedDate
      ? `Periode payroll untuk tanggal ${formatDate(closedDate)} sudah ditutup. Gunakan periode atau tanggal pembayaran yang masih terbuka.`
      : 'Periode payroll sudah ditutup. Gunakan periode atau tanggal pembayaran yang masih terbuka.';
  }

  const message = apiError?.error || apiError?.message || fallback;
  return apiError?.details ? `${message}: ${apiError.details}` : message;
};

const payrollImportHeaders = [
  'PAYMENT DATE',
  'PAYMENT PERIOD',
  'WORKING DAY',
  'EMPLOYEE ID',
  'WORK UNIT',
  'DIV',
  'PAYMENT VIA',
  'ACC NO',
  'EMPLOYEE NAME',
  'POSITION',
  'BASIC SALARY',
  'ATTENDANCE ALLOWANCE',
  'COMPENSATION PKWT',
  'OVERTIME',
  'PPH21 TERUTANG',
  'BPJS TK JHT 2%',
  'BPJS TK JP 1%',
  'BPJS KESEHATAN 1%',
  'PROPORTIONAL',
  'OTHER DEDUCTION',
  'DEDUCTION (PINJAMAN KARYAWAN)',
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
  const canCreatePayroll = payrollPermissions.canCreate;
  const canApprovePayroll = payrollPermissions.canApprove;
  const canSeeEmployeeDetail = isAdmin || isHR || isFinanceManager;

  const [payrollRuns, setPayrollRuns] = useState<PayrollRun[]>([]);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [statusFilter, setStatusFilter] = useState('ALL');
  const [selectedPayroll, setSelectedPayroll] = useState<PayrollRun | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [expenseAccounts, setExpenseAccounts] = useState<AccountCatalogItem[]>([]);
  const [employeeLoanAccounts, setEmployeeLoanAccounts] = useState<AccountCatalogItem[]>([]);
  const [accountsLoading, setAccountsLoading] = useState(false);
  const [selectedExpenseAccount, setSelectedExpenseAccount] = useState<string>('');
  const [selectedEmployeeLoanAccount, setSelectedEmployeeLoanAccount] = useState<string>('');
  const [postingDate, setPostingDate] = useState(currentMonthRange().payment_date);
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
  const [salaryPaymentDraft, setSalaryPaymentDraft] = useState({
    payment_date: toDateInputValue(new Date()),
    amount: 0,
    cash_bank_id: '',
    reference: '',
    notes: '',
  });
  const [bpjsPaymentDraft, setBPJSPaymentDraft] = useState({
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
  const detailModal = useDisclosure();
  const importModal = useDisclosure();
  const pphPaymentModal = useDisclosure();
  const salaryPaymentModal = useDisclosure();
  const bpjsPaymentModal = useDisclosure();

  const pageBg = useColorModeValue('gray.50', 'gray.900');
  const cardBg = useColorModeValue('white', 'gray.800');
  const mutedText = useColorModeValue('gray.600', 'gray.400');
  const borderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const tableHeaderBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const softPanelBg = useColorModeValue('blue.50', 'whiteAlpha.100');
  const inputBg = useColorModeValue('white', 'whiteAlpha.50');
  const modalHeaderGradient = useColorModeValue('linear(to-r, teal.50, blue.50)', 'linear(to-r, whiteAlpha.100, whiteAlpha.50)');

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
      const usableAccounts = filterOperationalCashBankAccounts(accounts).filter((account) => account.is_active && account.account_id && (account.currency || 'IDR').toUpperCase() === 'IDR' && account.allow_direct_transactions !== false);
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

  const loadApprovalAccounts = useCallback(async () => {
    setAccountsLoading(true);
    try {
      const token = window.localStorage.getItem('token') || '';

      const expenseData = await accountService.getAccountCatalog(token, 'EXPENSE');
      const assetData = await accountService.getAccountCatalog(token, 'ASSET');

      const filteredExpenses = filterActiveLeafAccounts(expenseData);
      const filteredEmployeeLoanAccounts = filterActiveLeafAccounts(assetData).filter((acc) => {
        const label = `${acc.code} ${acc.name}`.toLowerCase();
        return acc.code === '1107-001' || (label.includes('pinjaman') && label.includes('karyawan'));
      });

      setExpenseAccounts(filteredExpenses);
      setEmployeeLoanAccounts(filteredEmployeeLoanAccounts);

      const defaultExpense = filteredExpenses.find((acc) => acc.name.toLowerCase().includes('gaji') || acc.name.toLowerCase().includes('salary'))
        || filteredExpenses.find((acc) => acc.code === '6105' || acc.code === '5101-001' || acc.code === '5201' || acc.code === '6101');
      if (defaultExpense) {
        setSelectedExpenseAccount(defaultExpense.id.toString());
      } else if (filteredExpenses.length > 0) {
        setSelectedExpenseAccount(filteredExpenses[0].id.toString());
      }

      const defaultEmployeeLoanAccount = filteredEmployeeLoanAccounts.find((acc) => acc.code === '1107-001') || filteredEmployeeLoanAccounts[0];
      if (defaultEmployeeLoanAccount) {
        setSelectedEmployeeLoanAccount(defaultEmployeeLoanAccount.id.toString());
      }
    } catch (error) {
      console.error('Failed to load accounts for approval:', error);
    } finally {
      setAccountsLoading(false);
    }
  }, []);

  const totals = payrollRuns.reduce(
    (acc, payroll) => ({
      salary: acc.salary + Number(payroll.total_salary || 0),
      bonus: acc.bonus + Number(payroll.total_bonus || 0),
      pph: acc.pph + Number(payroll.total_pph || 0),
      bpjs: acc.bpjs + Number(payroll.total_bpjs || 0),
      employeeLoan: acc.employeeLoan + Number(payroll.total_employee_loan_deduction || 0),
      overall: acc.overall + Number(payroll.total_overall || 0),
    }),
    { salary: 0, bonus: 0, pph: 0, bpjs: 0, employeeLoan: 0, overall: 0 },
  );

  const updateImportPreviewItem = (index: number, field: keyof PayrollItem, value: string | number) => {
    setImportPreview((prev) => {
      if (!prev) return prev;
      const items = prev.items.map((item, itemIndex) => {
        if (itemIndex !== index) return item;
        const updated = { ...item, [field]: value } as PayrollItem;
        if (payrollBPJSComponentFields.includes(field)) {
          updated.bpjs = sumPayrollBPJSComponents(updated);
        }
        return recalculatePayrollItem(updated);
      });
      return recalculatePayrollImportPreview(prev, items);
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
      const normalizedItems = preview.items.map(recalculatePayrollItem);
      const normalizedPreview = recalculatePayrollImportPreview(preview, normalizedItems);
      setImportPreview(normalizedPreview);
      setImportDraft((prev) => ({
        ...prev,
        period_start: preview.suggested_period_start || prev.period_start,
        period_end: preview.suggested_period_end || prev.period_end,
        payment_date: preview.suggested_payment_date || prev.payment_date,
      }));
      toast({
        title: 'Preview import berhasil',
        description: `${normalizedPreview.summary.imported_rows} baris payroll siap direview.`,
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
    if (!importDraft.period_start || !importDraft.period_end) {
      toast({
        title: 'Periode wajib diisi',
        description: 'Period Start dan Period End harus diisi sebelum submit payroll.',
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

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
        items: importPreview.items.map(recalculatePayrollItem),
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

  const openDetail = async (payroll: PayrollRun) => {
    setSelectedPayroll(payroll);
    setRejectReason('');
    setSelectedExpenseAccount('');
    setSelectedEmployeeLoanAccount('');
    setPostingDate((payroll.payment_date || payroll.period_end || '').slice(0, 10));
    detailModal.onOpen();

    if (canApprovePayroll && payroll.status === 'PENDING_APPROVAL') {
      loadApprovalAccounts();
    }

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

  const openPaySalary = async (payroll: PayrollRun) => {
    const remainingSalary = getSalaryRemainingAmount(payroll);
    setSelectedPayroll(payroll);
    setSalaryPaymentDraft({
      payment_date: (payroll.payment_date || toDateInputValue(new Date())).slice(0, 10),
      amount: remainingSalary,
      cash_bank_id: '',
      reference: `PAY-SALARY-${payroll.payroll_number}`,
      notes: `Pembayaran net salary payroll ${payroll.payroll_number}`,
    });
    salaryPaymentModal.onOpen();
    await loadCashBanks();
  };

  const openPayBPJS = async (payroll: PayrollRun) => {
    const remainingBPJS = getBPJSRemainingAmount(payroll);
    setSelectedPayroll(payroll);
    setBPJSPaymentDraft({
      payment_date: toDateInputValue(new Date()),
      amount: remainingBPJS,
      cash_bank_id: '',
      reference: `PAY-BPJS-${payroll.payroll_number}`,
      notes: `Pembayaran BPJS payroll ${payroll.payroll_number}`,
    });
    bpjsPaymentModal.onOpen();
    await loadCashBanks();
  };

  const handlePaySalary = async () => {
    if (!selectedPayroll) return;
    const amount = Number(salaryPaymentDraft.amount || 0);
    const remainingSalary = getSalaryRemainingAmount(selectedPayroll);
    const cashBankId = Number(salaryPaymentDraft.cash_bank_id || 0);
    if (!cashBankId || amount <= 0 || amount > remainingSalary + 1) {
      toast({
        title: 'Pembayaran gaji tidak valid',
        description: `Pilih kas/bank dan isi nominal maksimal ${formatCurrency(remainingSalary)}.`,
        status: 'warning', duration: 3500, isClosable: true,
      });
      return;
    }

    setSubmitting(true);
    try {
      const updated = await payrollService.payPayrollSalary(selectedPayroll.id, {
        payment_date: salaryPaymentDraft.payment_date,
        amount,
        cash_bank_id: cashBankId,
        reference: salaryPaymentDraft.reference.trim(),
        notes: salaryPaymentDraft.notes.trim(),
      });
      setSelectedPayroll(updated);
      salaryPaymentModal.onClose();
      toast({
        title: 'Gaji payroll berhasil dibayar',
        description: 'Utang gaji didebit dan kas/bank dikredit pada tanggal pembayaran aktual.',
        status: 'success', duration: 4500, isClosable: true,
      });
      await loadPayroll();
    } catch (error: unknown) {
      toast({
        title: 'Gagal bayar gaji',
        description: getApiErrorMessage(error, 'Pembayaran gaji payroll belum bisa diproses.'),
        status: 'error', duration: 5000, isClosable: true,
      });
    } finally {
      setSubmitting(false);
    }
  };

  const handlePayBPJS = async () => {
    if (!selectedPayroll) return;

    const amount = Number(bpjsPaymentDraft.amount || 0);
    const remainingBPJS = getBPJSRemainingAmount(selectedPayroll);
    const cashBankId = Number(bpjsPaymentDraft.cash_bank_id || 0);
    if (!cashBankId) {
      toast({
        title: 'Akun kas/bank belum dipilih',
        description: 'Pilih akun kas/bank IDR untuk settlement hutang BPJS.',
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }
    if (amount <= 0 || amount > remainingBPJS + 1) {
      toast({
        title: 'Nominal BPJS tidak valid',
        description: `Nominal pembayaran harus lebih besar dari 0 dan maksimal ${formatCurrency(remainingBPJS)}.`,
        status: 'warning',
        duration: 3500,
        isClosable: true,
      });
      return;
    }

    setSubmitting(true);
    try {
      const updated = await payrollService.payPayrollBPJS(selectedPayroll.id, {
        payment_date: bpjsPaymentDraft.payment_date,
        amount,
        cash_bank_id: cashBankId,
        reference: bpjsPaymentDraft.reference.trim(),
        notes: bpjsPaymentDraft.notes.trim(),
      });
      setSelectedPayroll(updated);
      bpjsPaymentModal.onClose();
      toast({
        title: 'BPJS payroll berhasil dibayar',
        description: 'Jurnal settlement BPJS dibuat pada tanggal pembayaran aktual.',
        status: 'success',
        duration: 4500,
        isClosable: true,
      });
      await loadPayroll();
    } catch (error: unknown) {
      toast({
        title: 'Gagal bayar BPJS payroll',
        description: getApiErrorMessage(error, 'Pembayaran BPJS payroll belum bisa diproses.'),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setSubmitting(false);
    }
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
      const payload: {
        expense_account_id?: number;
        employee_loan_account_id?: number;
        posting_date?: string;
      } = {};
      if (selectedExpenseAccount) {
        payload.expense_account_id = Number(selectedExpenseAccount);
      }
      if (Number(selectedPayroll.total_employee_loan_deduction || 0) > 0 && selectedEmployeeLoanAccount) {
        payload.employee_loan_account_id = Number(selectedEmployeeLoanAccount);
      }
      payload.posting_date = postingDate;

      const updated = await payrollService.approvePayrollRun(selectedPayroll.id, payload);
      setSelectedPayroll(updated);
      toast({
        title: 'Payroll approved',
        description: 'Beban dan kewajiban payroll sudah diposting. Kas/bank belum berkurang sampai pembayaran aktual.',
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
      <SimpleLayout>
        <Box bg={pageBg} minH="100vh" p={6}>
          <VStack align="stretch" spacing={6}>
            <Flex justify="space-between" align={{ base: 'stretch', md: 'center' }} gap={4} direction={{ base: 'column', md: 'row' }}>
              <Box>
                <HStack spacing={3} mb={2}>
                  <Icon as={FiUsers} boxSize={7} color="blue.500" />
                  <Heading size="xl">Payroll</Heading>
                </HStack>
                <Text color={mutedText}>Payroll ditambahkan melalui Import Excel, lalu dikirim ke Finance Manager untuk approval.</Text>
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
                  </>
                )}
              </HStack>
            </Flex>

            {isSummaryOnly && (
              <Alert status="info" borderRadius="xl" bg={softPanelBg}>
                <AlertIcon />
                <AlertDescription>Akses role {isAccountingRole(role) ? 'Accounting' : 'Finance Staff'} dibatasi ke summary payroll saja. Detail per karyawan disembunyikan dari API dan UI.</AlertDescription>
              </Alert>
            )}

            <SimpleGrid columns={{ base: 1, md: 2, xl: 6 }} spacing={4}>
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
                    <StatHelpText>Attendance allowance + tunjangan lain</StatHelpText>
                  </Stat>
                </CardBody>
              </Card>
              <Card bg={cardBg} borderWidth="1px" borderColor={borderColor}>
                <CardBody>
                  <Stat>
                    <StatLabel>PPh21 Terutang</StatLabel>
                    <StatNumber color="orange.500">{formatCurrency(totals.pph)}</StatNumber>
                    <StatHelpText>Hutang pajak, tidak mengurangi salary</StatHelpText>
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
                    <StatLabel>Pinjaman Karyawan</StatLabel>
                    <StatNumber color="cyan.600">{formatCurrency(totals.employeeLoan)}</StatNumber>
                    <StatHelpText>Deduction payroll terfilter</StatHelpText>
                  </Stat>
                </CardBody>
              </Card>
              <Card bg={cardBg} borderWidth="1px" borderColor={borderColor}>
                <CardBody>
                  <Stat>
                    <StatLabel>Total Net Salary</StatLabel>
                    <StatNumber color="blue.600">{formatCurrency(totals.overall)}</StatNumber>
                    <StatHelpText>Salary + bonus - BPJS/potongan lain/pinjaman</StatHelpText>
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
                          <Th isNumeric>PPh21 Terutang</Th>
                          <Th>Status PPh21</Th>
                          <Th isNumeric>Total BPJS</Th>
                          <Th>Status BPJS</Th>
                          <Th isNumeric>Pinjaman Karyawan</Th>
                          <Th isNumeric>Net Salary</Th>
                          <Th>Status Gaji</Th>
                          <Th>Status</Th>
                          {canSeeEmployeeDetail && <Th>Aksi</Th>}
                        </Tr>
                      </Thead>
                      <Tbody>
                        {payrollRuns.map((payroll) => (
                          <Tr key={payroll.id}>
                            <Td fontWeight="semibold">{payroll.payroll_number}</Td>
                            <Td>
                              {formatDate(payroll.period_start)} - {formatDate(payroll.period_end)}
                            </Td>
                            <Td>{formatDate(payroll.payment_date)}</Td>
                            <Td isNumeric>{formatCurrency(payroll.total_salary)}</Td>
                            <Td isNumeric>{formatCurrency(payroll.total_bonus)}</Td>
                            <Td isNumeric>{formatCurrency(payroll.total_pph)}</Td>
                            <Td>
                              <VStack align="start" spacing={1}>
                                <Badge colorScheme={pphPaymentStatusColor(getPPhPaymentStatus(payroll))}>{pphPaymentStatusLabel(getPPhPaymentStatus(payroll))}</Badge>
                                {Number(payroll.total_pph || 0) > 0 && (
                                  <Text fontSize="xs" color={mutedText}>
                                    Dibayar {formatCurrency(Number(payroll.pph_paid_amount || 0))}
                                  </Text>
                                )}
                              </VStack>
                            </Td>
                            <Td isNumeric>{formatCurrency(payroll.total_bpjs)}</Td>
                            <Td>
                              <VStack align="start" spacing={1}>
                                <Badge colorScheme={bpjsPaymentStatusColor(getBPJSPaymentStatus(payroll))}>{bpjsPaymentStatusLabel(getBPJSPaymentStatus(payroll))}</Badge>
                                {Number(payroll.total_bpjs || 0) > 0 && (
                                  <Text fontSize="xs" color={mutedText}>
                                    Dibayar {formatCurrency(Number(payroll.bpjs_paid_amount || 0))}
                                  </Text>
                                )}
                              </VStack>
                            </Td>
                            <Td isNumeric>{formatCurrency(Number(payroll.total_employee_loan_deduction || 0))}</Td>
                            <Td isNumeric fontWeight="bold" color="blue.600">
                              {formatCurrency(payroll.total_overall)}
                            </Td>
                            <Td>
                              <VStack align="start" spacing={1}>
                                <Badge colorScheme={salaryPaymentStatusColor(getSalaryPaymentStatus(payroll))}>{salaryPaymentStatusLabel(getSalaryPaymentStatus(payroll))}</Badge>
                                <Text fontSize="xs" color={mutedText}>Dibayar {formatCurrency(Number(payroll.salary_paid_amount || 0))}</Text>
                              </VStack>
                            </Td>
                            <Td>
                              <Badge colorScheme={statusColor(payroll.status)}>{statusLabel(payroll.status)}</Badge>
                            </Td>
                            {canSeeEmployeeDetail && (
                              <Td>
                                <HStack spacing={2}>
                                  <IconButton aria-label="Lihat detail payroll" icon={<FiEye />} size="sm" variant="ghost" colorScheme="blue" onClick={() => openDetail(payroll)} />
                                  {canApprovePayroll && payroll.status === 'APPROVED' && getSalaryRemainingAmount(payroll) > 0 && (
                                    <Button leftIcon={<FiDollarSign />} size="sm" colorScheme="blue" variant="outline" onClick={() => openPaySalary(payroll)}>
                                      Pay Salary
                                    </Button>
                                  )}
                                  {canApprovePayroll && payroll.status === 'APPROVED' && getBPJSRemainingAmount(payroll) > 0 && (
                                    <Button leftIcon={<FiDollarSign />} size="sm" colorScheme="purple" variant="outline" onClick={() => openPayBPJS(payroll)}>
                                      Pay BPJS
                                    </Button>
                                  )}
                                  {canApprovePayroll && payroll.status === 'APPROVED' && getPPhRemainingAmount(payroll) > 0 && (
                                    <Button leftIcon={<FiDollarSign />} size="sm" colorScheme="orange" variant="outline" onClick={() => openPayPPh(payroll)}>
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
                            <Td colSpan={canSeeEmployeeDetail ? 14 : 13}>
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
            <ModalContent bg={cardBg} borderWidth="1px" borderColor={borderColor}>
              <ModalHeader bgGradient={modalHeaderGradient} borderBottomWidth="1px" borderColor={borderColor}>
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
                      <Input
                        type="date"
                        value={importDraft.period_start}
                        onChange={(event) =>
                          setImportDraft((prev) => ({
                            ...prev,
                            period_start: event.target.value,
                          }))
                        }
                        bg={inputBg}
                      />
                    </FormControl>
                    <FormControl isRequired>
                      <FormLabel>Period End</FormLabel>
                      <Input
                        type="date"
                        value={importDraft.period_end}
                        onChange={(event) =>
                          setImportDraft((prev) => ({
                            ...prev,
                            period_end: event.target.value,
                          }))
                        }
                        bg={inputBg}
                      />
                    </FormControl>
                    <FormControl>
                      <FormLabel>Payment Date</FormLabel>
                      <Input
                        type="date"
                        value={importDraft.payment_date}
                        onChange={(event) =>
                          setImportDraft((prev) => ({
                            ...prev,
                            payment_date: event.target.value,
                          }))
                        }
                        bg={inputBg}
                      />
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
                      <Textarea
                        value={importDraft.notes}
                        onChange={(event) =>
                          setImportDraft((prev) => ({
                            ...prev,
                            notes: event.target.value,
                          }))
                        }
                        bg={inputBg}
                      />
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
                      <SimpleGrid columns={{ base: 1, md: 2, xl: 7 }} spacing={4}>
                        <Card variant="outline">
                          <CardBody>
                            <Stat>
                              <StatLabel>Imported Rows</StatLabel>
                              <StatNumber>{importPreview.summary.imported_rows}</StatNumber>
                              <StatHelpText>dari {importPreview.summary.total_rows} baris</StatHelpText>
                            </Stat>
                          </CardBody>
                        </Card>
                        <Card variant="outline">
                          <CardBody>
                            <Stat>
                              <StatLabel>Total Salary</StatLabel>
                              <StatNumber>{formatCurrency(importPreview.summary.total_salary)}</StatNumber>
                            </Stat>
                          </CardBody>
                        </Card>
                        <Card variant="outline">
                          <CardBody>
                            <Stat>
                              <StatLabel>Total Bonus</StatLabel>
                              <StatNumber color="green.500">{formatCurrency(importPreview.summary.total_bonus)}</StatNumber>
                            </Stat>
                          </CardBody>
                        </Card>
                        <Card variant="outline">
                          <CardBody>
                            <Stat>
                              <StatLabel>PPh21 Terutang</StatLabel>
                              <StatNumber color="orange.500">{formatCurrency(importPreview.summary.total_pph)}</StatNumber>
                            </Stat>
                          </CardBody>
                        </Card>
                        <Card variant="outline">
                          <CardBody>
                            <Stat>
                              <StatLabel>Total BPJS</StatLabel>
                              <StatNumber color="purple.500">{formatCurrency(importPreview.summary.total_bpjs)}</StatNumber>
                            </Stat>
                          </CardBody>
                        </Card>
                        <Card variant="outline">
                          <CardBody>
                            <Stat>
                              <StatLabel>Pinjaman Karyawan</StatLabel>
                              <StatNumber color="cyan.600">{formatCurrency(Number(importPreview.summary.total_employee_loan_deduction || 0))}</StatNumber>
                            </Stat>
                          </CardBody>
                        </Card>
                        <Card variant="outline">
                          <CardBody>
                            <Stat>
                              <StatLabel>Net Salary</StatLabel>
                              <StatNumber color="blue.600">{formatCurrency(importPreview.summary.total_overall)}</StatNumber>
                            </Stat>
                          </CardBody>
                        </Card>
                      </SimpleGrid>

                      {importPreview.warnings.length > 0 && (
                        <Alert status="warning" borderRadius="xl">
                          <AlertIcon />
                          <AlertDescription>{importPreview.warnings.join(' | ')}</AlertDescription>
                        </Alert>
                      )}

                      <Box>
                        <Heading size="sm" mb={3}>
                          Review Grid Payroll Import
                        </Heading>
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
                                <Th isNumeric>Attendance Allowance</Th>
                                <Th isNumeric>Comp. PKWT</Th>
                                <Th isNumeric>Overtime</Th>
                                <Th isNumeric>PPh21 Terutang</Th>
                                <Th isNumeric>BPJS JHT</Th>
                                <Th isNumeric>BPJS JP</Th>
                                <Th isNumeric>BPJS Kes.</Th>
                                <Th isNumeric>Proportional</Th>
                                <Th isNumeric>Other Deduction</Th>
                                <Th isNumeric>Deduction (Pinjaman Karyawan)</Th>
                                <Th isNumeric>Net Salary</Th>
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
                                  <Td isNumeric>{formatCurrency(item.attendance_allowance || 0)}</Td>
                                  <Td isNumeric>{formatCurrency(item.compensation_pkwt || 0)}</Td>
                                  <Td isNumeric>{formatCurrency(item.overtime || 0)}</Td>
                                  <Td isNumeric minW="140px">
                                    <CurrencyInput size="sm" value={item.pph || 0} onChange={(value) => updateImportPreviewItem(index, 'pph', value)} showLabel={false} min={0} />
                                  </Td>
                                  <Td isNumeric minW="130px">
                                    <CurrencyInput size="sm" value={item.bpjs_tk_jht || 0} onChange={(value) => updateImportPreviewItem(index, 'bpjs_tk_jht', value)} showLabel={false} min={0} />
                                  </Td>
                                  <Td isNumeric minW="130px">
                                    <CurrencyInput size="sm" value={item.bpjs_tk_jp || 0} onChange={(value) => updateImportPreviewItem(index, 'bpjs_tk_jp', value)} showLabel={false} min={0} />
                                  </Td>
                                  <Td isNumeric minW="130px">
                                    <CurrencyInput size="sm" value={item.bpjs_kesehatan || 0} onChange={(value) => updateImportPreviewItem(index, 'bpjs_kesehatan', value)} showLabel={false} min={0} />
                                  </Td>
                                  <Td isNumeric>{formatCurrency(item.proportional || 0)}</Td>
                                  <Td isNumeric>{formatCurrency(item.other_deduction || 0)}</Td>
                                  <Td isNumeric minW="150px">
                                    <CurrencyInput size="sm" value={item.employee_loan_deduction || 0} onChange={(value) => updateImportPreviewItem(index, 'employee_loan_deduction', value)} showLabel={false} min={0} />
                                  </Td>
                                  <Td isNumeric fontWeight="bold" color="blue.600">
                                    {formatCurrency(getPayrollItemNetPay(item))}
                                  </Td>
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

          <Modal isOpen={detailModal.isOpen} onClose={detailModal.onClose} size="full" scrollBehavior="inside">
            <ModalOverlay />
            <ModalContent maxW={{ base: '98vw', xl: '95vw' }} my={4} bg={cardBg} borderWidth="1px" borderColor={borderColor}>
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
                    <SimpleGrid columns={{ base: 1, md: 2, xl: 7 }} spacing={4}>
                      <Card variant="outline">
                        <CardBody>
                          <Stat>
                            <StatLabel>Total Salary</StatLabel>
                            <StatNumber>{formatCurrency(selectedPayroll.total_salary)}</StatNumber>
                          </Stat>
                        </CardBody>
                      </Card>
                      <Card variant="outline">
                        <CardBody>
                          <Stat>
                            <StatLabel>Total Bonus</StatLabel>
                            <StatNumber color="green.500">{formatCurrency(selectedPayroll.total_bonus)}</StatNumber>
                          </Stat>
                        </CardBody>
                      </Card>
                      <Card variant="outline">
                        <CardBody>
                          <Stat>
                            <StatLabel>PPh21 Terutang</StatLabel>
                            <StatNumber fontSize="lg" whiteSpace="nowrap" color="orange.500">{formatCurrency(selectedPayroll.total_pph)}</StatNumber>
                          </Stat>
                        </CardBody>
                      </Card>
                      <Card variant="outline">
                        <CardBody>
                          <Stat>
                            <StatLabel>Status PPh21</StatLabel>
                            <StatNumber fontSize="lg">
                              <Badge colorScheme={pphPaymentStatusColor(getPPhPaymentStatus(selectedPayroll))}>{pphPaymentStatusLabel(getPPhPaymentStatus(selectedPayroll))}</Badge>
                            </StatNumber>
                            <StatHelpText>
                              Paid {formatCurrency(Number(selectedPayroll.pph_paid_amount || 0))} | Sisa {formatCurrency(getPPhRemainingAmount(selectedPayroll))}
                            </StatHelpText>
                          </Stat>
                        </CardBody>
                      </Card>
                      <Card variant="outline">
                        <CardBody>
                          <Stat>
                            <StatLabel>Total BPJS</StatLabel>
                            <StatNumber color="purple.500">{formatCurrency(selectedPayroll.total_bpjs)}</StatNumber>
                            <StatHelpText>
                              <Badge colorScheme={bpjsPaymentStatusColor(getBPJSPaymentStatus(selectedPayroll))}>{bpjsPaymentStatusLabel(getBPJSPaymentStatus(selectedPayroll))}</Badge>
                              {' '}Paid {formatCurrency(Number(selectedPayroll.bpjs_paid_amount || 0))} | Sisa {formatCurrency(getBPJSRemainingAmount(selectedPayroll))}
                            </StatHelpText>
                          </Stat>
                        </CardBody>
                      </Card>
                      <Card variant="outline">
                        <CardBody>
                          <Stat>
                            <StatLabel>Pinjaman Karyawan</StatLabel>
                            <StatNumber color="cyan.600">{formatCurrency(Number(selectedPayroll.total_employee_loan_deduction || 0))}</StatNumber>
                            <StatHelpText>Kredit ke COA pinjaman saat approval</StatHelpText>
                          </Stat>
                        </CardBody>
                      </Card>
                      <Card variant="outline">
                        <CardBody>
                          <Stat>
                            <StatLabel>Net Salary</StatLabel>
                            <StatNumber color="blue.600">{formatCurrency(selectedPayroll.total_overall)}</StatNumber>
                            <StatHelpText>
                              <Badge colorScheme={salaryPaymentStatusColor(getSalaryPaymentStatus(selectedPayroll))}>{salaryPaymentStatusLabel(getSalaryPaymentStatus(selectedPayroll))}</Badge>
                              {' '}Paid {formatCurrency(Number(selectedPayroll.salary_paid_amount || 0))}
                            </StatHelpText>
                          </Stat>
                        </CardBody>
                      </Card>
                    </SimpleGrid>

                    {(Number(selectedPayroll.total_pph || 0) > 0 || Number(selectedPayroll.total_bpjs || 0) > 0) && (
                      <Alert status="info" borderRadius="xl">
                        <AlertIcon />
                        <AlertDescription>
                          Hutang PPh payroll diakui pada periode payroll ({formatDate(selectedPayroll.pph_recognition_date || selectedPayroll.period_end)}
                          ). Tombol Pay BPJS dan Pay PPh hanya mencatat settlement pada tanggal kewajiban benar-benar dibayar.
                        </AlertDescription>
                      </Alert>
                    )}

                    {!selectedPayroll.detail_allowed && (
                      <Alert status="info" borderRadius="xl">
                        <AlertIcon />
                        <AlertDescription>Anda hanya memiliki akses summary payroll. Detail per karyawan tidak ditampilkan untuk role ini.</AlertDescription>
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
                              <Th isNumeric>Attendance Allowance</Th>
                              <Th isNumeric>Comp. PKWT</Th>
                              <Th isNumeric>Overtime</Th>
                              <Th isNumeric>PPh21 Terutang</Th>
                              <Th isNumeric>BPJS JHT</Th>
                              <Th isNumeric>BPJS JP</Th>
                              <Th isNumeric>BPJS Kes.</Th>
                              <Th isNumeric>Proportional</Th>
                              <Th isNumeric>Other Deduction</Th>
                              <Th isNumeric>Deduction (Pinjaman Karyawan)</Th>
                              <Th isNumeric>Net Salary</Th>
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
                                  <Text fontSize="xs" color={mutedText}>
                                    {item.department || '-'}
                                  </Text>
                                </Td>
                                <Td>{item.position || '-'}</Td>
                                <Td isNumeric>{formatCurrency(item.base_salary)}</Td>
                                <Td isNumeric>{formatCurrency(item.attendance_allowance || 0)}</Td>
                                <Td isNumeric>{formatCurrency(item.compensation_pkwt || 0)}</Td>
                                <Td isNumeric>{formatCurrency(item.overtime || 0)}</Td>
                                <Td isNumeric color="orange.600">
                                  {formatCurrency(item.pph || 0)}
                                </Td>
                                <Td isNumeric>{formatCurrency(item.bpjs_tk_jht || 0)}</Td>
                                <Td isNumeric>{formatCurrency(item.bpjs_tk_jp || 0)}</Td>
                                <Td isNumeric>{formatCurrency(item.bpjs_kesehatan || 0)}</Td>
                                <Td isNumeric>{formatCurrency(item.proportional || 0)}</Td>
                                <Td isNumeric>{formatCurrency(item.other_deduction || 0)}</Td>
                                <Td isNumeric>{formatCurrency(item.employee_loan_deduction || 0)}</Td>
                                <Td isNumeric fontWeight="bold">
                                  {formatCurrency(getPayrollItemNetPay(item))}
                                </Td>
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

                        <Alert status="info" borderRadius="lg" mb={4}>
                          <AlertIcon />
                          <AlertDescription>Approval hanya memposting beban, kewajiban payroll, dan kredit Pinjaman Karyawan bila ada. Kas/bank baru berkurang saat tombol Pay Salary, Pay BPJS, atau Pay PPh dijalankan.</AlertDescription>
                        </Alert>
                        <SimpleGrid columns={{ base: 1, md: Number(selectedPayroll.total_employee_loan_deduction || 0) > 0 ? 3 : 2 }} spacing={4} mb={4}>
                          <FormControl isRequired>
                            <FormLabel fontSize="sm" fontWeight="medium">
                              Akun Beban Gaji (Debit)
                            </FormLabel>
                            {accountsLoading ? (
                              <Spinner size="sm" />
                            ) : (
                              <Select value={selectedExpenseAccount} onChange={(e) => setSelectedExpenseAccount(e.target.value)} bg={inputBg}>
                                {expenseAccounts.map((acc) => (
                                  <option key={acc.id} value={acc.id}>
                                    {acc.code} - {acc.name}
                                  </option>
                                ))}
                              </Select>
                            )}
                          </FormControl>

                          {Number(selectedPayroll.total_employee_loan_deduction || 0) > 0 && (
                            <FormControl isRequired>
                              <FormLabel fontSize="sm" fontWeight="medium">
                                Akun Pinjaman Karyawan (Credit)
                              </FormLabel>
                              {accountsLoading ? (
                                <Spinner size="sm" />
                              ) : (
                                <Select value={selectedEmployeeLoanAccount} onChange={(e) => setSelectedEmployeeLoanAccount(e.target.value)} bg={inputBg} placeholder="Pilih COA pinjaman karyawan">
                                  {employeeLoanAccounts.map((acc) => (
                                    <option key={acc.id} value={acc.id}>
                                      {acc.code} - {acc.name}
                                    </option>
                                  ))}
                                </Select>
                              )}
                              <Text fontSize="xs" color={mutedText} mt={1}>
                                Total kredit: {formatCurrency(Number(selectedPayroll.total_employee_loan_deduction || 0))}
                              </Text>
                            </FormControl>
                          )}

                          <FormControl isRequired>
                            <FormLabel fontSize="sm" fontWeight="medium">
                              Tanggal Posting Payroll
                            </FormLabel>
                            <Input type="date" min={selectedPayroll.period_start.slice(0, 10)} max={selectedPayroll.period_end.slice(0, 10)} value={postingDate} onChange={(e) => setPostingDate(e.target.value)} bg={inputBg} />
                          </FormControl>
                        </SimpleGrid>

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
                {selectedPayroll && canApprovePayroll && selectedPayroll.status === 'APPROVED' && getSalaryRemainingAmount(selectedPayroll) > 0 && (
                  <Button leftIcon={<FiDollarSign />} colorScheme="blue" mr={3} onClick={() => openPaySalary(selectedPayroll)} isLoading={submitting}>
                    Pay Salary
                  </Button>
                )}
                {selectedPayroll && canApprovePayroll && selectedPayroll.status === 'APPROVED' && getBPJSRemainingAmount(selectedPayroll) > 0 && (
                  <Button leftIcon={<FiDollarSign />} colorScheme="purple" mr={3} onClick={() => openPayBPJS(selectedPayroll)} isLoading={submitting}>
                    Pay BPJS
                  </Button>
                )}
                {selectedPayroll && canApprovePayroll && selectedPayroll.status === 'APPROVED' && getPPhRemainingAmount(selectedPayroll) > 0 && (
                  <Button leftIcon={<FiDollarSign />} colorScheme="orange" onClick={() => openPayPPh(selectedPayroll)} isLoading={submitting}>
                    Pay PPh
                  </Button>
                )}
              </ModalFooter>
            </ModalContent>
          </Modal>

          <Modal isOpen={salaryPaymentModal.isOpen} onClose={salaryPaymentModal.onClose} size="2xl" scrollBehavior="inside">
            <ModalOverlay />
            <ModalContent bg={cardBg} borderWidth="1px" borderColor={borderColor}>
              <ModalHeader borderBottomWidth="1px" borderColor={borderColor}>
                <HStack>
                  <Icon as={FiDollarSign} color="blue.500" />
                  <Box>
                    <Text>Pay Salary Payroll</Text>
                    <Text fontSize="sm" color={mutedText} fontWeight="normal">Settlement utang gaji pada tanggal pembayaran aktual.</Text>
                  </Box>
                </HStack>
              </ModalHeader>
              <ModalCloseButton />
              <ModalBody py={6}>
                {selectedPayroll && (
                  <VStack align="stretch" spacing={5}>
                    <Alert status="info" borderRadius="xl">
                      <AlertIcon />
                      <AlertDescription>Jurnal: Debit Utang Gaji dan Kredit Kas/Bank. Pembayaran tidak mengubah beban gaji yang sudah diakui saat approval.</AlertDescription>
                    </Alert>
                    <SimpleGrid columns={{ base: 1, md: 3 }} spacing={4}>
                      <Card variant="outline"><CardBody><Stat><StatLabel>Net Salary</StatLabel><StatNumber fontSize="lg" whiteSpace="nowrap" color="blue.600">{formatCurrency(selectedPayroll.total_overall)}</StatNumber></Stat></CardBody></Card>
                      <Card variant="outline"><CardBody><Stat><StatLabel>Sudah Dibayar</StatLabel><StatNumber fontSize="lg" whiteSpace="nowrap" color="green.500">{formatCurrency(Number(selectedPayroll.salary_paid_amount || 0))}</StatNumber></Stat></CardBody></Card>
                      <Card variant="outline"><CardBody><Stat><StatLabel>Sisa Gaji</StatLabel><StatNumber fontSize="lg" whiteSpace="nowrap" color="red.500">{formatCurrency(getSalaryRemainingAmount(selectedPayroll))}</StatNumber></Stat></CardBody></Card>
                    </SimpleGrid>
                    <SimpleGrid columns={{ base: 1, md: 2 }} spacing={4}>
                      <FormControl isRequired>
                        <FormLabel>Tanggal Bayar Gaji</FormLabel>
                        <Input type="date" value={salaryPaymentDraft.payment_date} onChange={(event) => setSalaryPaymentDraft((prev) => ({ ...prev, payment_date: event.target.value }))} bg={inputBg} />
                      </FormControl>
                      <FormControl isRequired>
                        <FormLabel>Nominal Pembayaran</FormLabel>
                        <CurrencyInput
                          value={salaryPaymentDraft.amount}
                          onChange={(amount) => setSalaryPaymentDraft((prev) => ({ ...prev, amount }))}
                          min={1}
                          max={getSalaryRemainingAmount(selectedPayroll)}
                          showLabel={false}
                          keepFormattedOnFocus
                        />
                      </FormControl>
                    </SimpleGrid>
                    <FormControl isRequired>
                      <FormLabel>Kas/Bank Pembayaran</FormLabel>
                      <Select placeholder="Pilih Kas/Bank IDR" value={salaryPaymentDraft.cash_bank_id} onChange={(event) => setSalaryPaymentDraft((prev) => ({ ...prev, cash_bank_id: event.target.value }))} bg={inputBg} isDisabled={cashBankLoading}>
                        {cashBanks.map((account) => <option key={account.id} value={account.id}>{account.code} - {account.name} ({formatCurrency(account.balance)})</option>)}
                      </Select>
                    </FormControl>
                    <FormControl>
                      <FormLabel>Reference</FormLabel>
                      <Input value={salaryPaymentDraft.reference} onChange={(event) => setSalaryPaymentDraft((prev) => ({ ...prev, reference: event.target.value }))} bg={inputBg} />
                    </FormControl>
                    <FormControl>
                      <FormLabel>Catatan</FormLabel>
                      <Textarea value={salaryPaymentDraft.notes} onChange={(event) => setSalaryPaymentDraft((prev) => ({ ...prev, notes: event.target.value }))} bg={inputBg} />
                    </FormControl>
                  </VStack>
                )}
              </ModalBody>
              <ModalFooter borderTopWidth="1px" borderColor={borderColor}>
                <Button variant="ghost" mr={3} onClick={salaryPaymentModal.onClose}>Batal</Button>
                <Button leftIcon={<FiDollarSign />} colorScheme="blue" onClick={handlePaySalary} isLoading={submitting} isDisabled={!selectedPayroll || cashBankLoading || cashBanks.length === 0}>Bayar Gaji</Button>
              </ModalFooter>
            </ModalContent>
          </Modal>

          <Modal isOpen={bpjsPaymentModal.isOpen} onClose={bpjsPaymentModal.onClose} size="2xl" scrollBehavior="inside">
            <ModalOverlay />
            <ModalContent bg={cardBg} borderWidth="1px" borderColor={borderColor}>
              <ModalHeader borderBottomWidth="1px" borderColor={borderColor}>
                <HStack>
                  <Icon as={FiDollarSign} color="purple.500" />
                  <Box>
                    <Text>Pay BPJS Payroll</Text>
                    <Text fontSize="sm" color={mutedText} fontWeight="normal">
                      Settlement hutang BPJS dapat berbeda tanggal dari periode payroll.
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
                        Jurnal: Debit 2103-002 - Utang BPJS & Potongan Payroll dan Kredit Kas/Bank. Nominal default hanya Total BPJS, tidak termasuk Other Deduction.
                      </AlertDescription>
                    </Alert>

                    <SimpleGrid columns={{ base: 1, md: 3 }} spacing={4}>
                      <Card variant="outline">
                        <CardBody>
                          <Stat>
                            <StatLabel>Total BPJS</StatLabel>
                            <StatNumber color="purple.500">{formatCurrency(selectedPayroll.total_bpjs)}</StatNumber>
                          </Stat>
                        </CardBody>
                      </Card>
                      <Card variant="outline">
                        <CardBody>
                          <Stat>
                            <StatLabel>Sudah Dibayar</StatLabel>
                            <StatNumber fontSize="lg" whiteSpace="nowrap" color="green.500">{formatCurrency(Number(selectedPayroll.bpjs_paid_amount || 0))}</StatNumber>
                          </Stat>
                        </CardBody>
                      </Card>
                      <Card variant="outline">
                        <CardBody>
                          <Stat>
                            <StatLabel>Sisa BPJS</StatLabel>
                            <StatNumber fontSize="lg" whiteSpace="nowrap" color="red.500">{formatCurrency(getBPJSRemainingAmount(selectedPayroll))}</StatNumber>
                          </Stat>
                        </CardBody>
                      </Card>
                    </SimpleGrid>

                    <SimpleGrid columns={{ base: 1, md: 2 }} spacing={4}>
                      <FormControl isRequired>
                        <FormLabel>Tanggal Bayar BPJS</FormLabel>
                        <Input
                          type="date"
                          value={bpjsPaymentDraft.payment_date}
                          onChange={(event) => setBPJSPaymentDraft((prev) => ({ ...prev, payment_date: event.target.value }))}
                          bg={inputBg}
                        />
                      </FormControl>
                      <FormControl isRequired>
                        <FormLabel>Nominal Bayar</FormLabel>
                        <CurrencyInput
                          value={bpjsPaymentDraft.amount}
                          onChange={(amount) => setBPJSPaymentDraft((prev) => ({ ...prev, amount }))}
                          min={1}
                          max={getBPJSRemainingAmount(selectedPayroll)}
                          showLabel={false}
                          keepFormattedOnFocus
                        />
                      </FormControl>
                    </SimpleGrid>

                    <FormControl isRequired>
                      <FormLabel>Akun Kas/Bank IDR</FormLabel>
                      <Select
                        placeholder={cashBankLoading ? 'Memuat kas/bank...' : 'Pilih kas/bank pembayaran BPJS'}
                        value={bpjsPaymentDraft.cash_bank_id}
                        onChange={(event) => setBPJSPaymentDraft((prev) => ({ ...prev, cash_bank_id: event.target.value }))}
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
                      <Input value={bpjsPaymentDraft.reference} onChange={(event) => setBPJSPaymentDraft((prev) => ({ ...prev, reference: event.target.value }))} bg={inputBg} />
                    </FormControl>

                    <FormControl>
                      <FormLabel>Catatan</FormLabel>
                      <Textarea value={bpjsPaymentDraft.notes} onChange={(event) => setBPJSPaymentDraft((prev) => ({ ...prev, notes: event.target.value }))} bg={inputBg} />
                    </FormControl>
                  </VStack>
                )}
              </ModalBody>
              <ModalFooter borderTopWidth="1px" borderColor={borderColor}>
                <Button variant="ghost" mr={3} onClick={bpjsPaymentModal.onClose}>
                  Batal
                </Button>
                <Button leftIcon={<FiDollarSign />} colorScheme="purple" onClick={handlePayBPJS} isLoading={submitting} isDisabled={!selectedPayroll || cashBankLoading || cashBanks.length === 0}>
                  Bayar BPJS
                </Button>
              </ModalFooter>
            </ModalContent>
          </Modal>

          <Modal isOpen={pphPaymentModal.isOpen} onClose={pphPaymentModal.onClose} size="2xl" scrollBehavior="inside">
            <ModalOverlay />
            <ModalContent bg={cardBg} borderWidth="1px" borderColor={borderColor}>
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
                            <StatLabel>PPh21 Terutang</StatLabel>
                            <StatNumber color="orange.500">{formatCurrency(selectedPayroll.total_pph)}</StatNumber>
                          </Stat>
                        </CardBody>
                      </Card>
                      <Card variant="outline">
                        <CardBody>
                          <Stat>
                            <StatLabel>Sudah Dibayar</StatLabel>
                            <StatNumber fontSize="lg" whiteSpace="nowrap" color="green.500">{formatCurrency(Number(selectedPayroll.pph_paid_amount || 0))}</StatNumber>
                          </Stat>
                        </CardBody>
                      </Card>
                      <Card variant="outline">
                        <CardBody>
                          <Stat>
                            <StatLabel>Sisa PPh</StatLabel>
                            <StatNumber fontSize="lg" whiteSpace="nowrap" color="red.500">{formatCurrency(getPPhRemainingAmount(selectedPayroll))}</StatNumber>
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
                          onChange={(event) =>
                            setPPhPaymentDraft((prev) => ({
                              ...prev,
                              payment_date: event.target.value,
                            }))
                          }
                          bg={inputBg}
                        />
                      </FormControl>
                      <FormControl isRequired>
                        <FormLabel>Nominal Bayar</FormLabel>
                        <CurrencyInput
                          value={pphPaymentDraft.amount}
                          onChange={(amount) => setPPhPaymentDraft((prev) => ({ ...prev, amount }))}
                          min={1}
                          max={getPPhRemainingAmount(selectedPayroll)}
                          showLabel={false}
                          keepFormattedOnFocus
                        />
                      </FormControl>
                    </SimpleGrid>

                    <FormControl isRequired>
                      <FormLabel>Akun Kas/Bank IDR</FormLabel>
                      <Select
                        placeholder={cashBankLoading ? 'Memuat kas/bank...' : 'Pilih kas/bank pembayaran PPh'}
                        value={pphPaymentDraft.cash_bank_id}
                        onChange={(event) =>
                          setPPhPaymentDraft((prev) => ({
                            ...prev,
                            cash_bank_id: event.target.value,
                          }))
                        }
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
                        onChange={(event) =>
                          setPPhPaymentDraft((prev) => ({
                            ...prev,
                            reference: event.target.value,
                          }))
                        }
                        bg={inputBg}
                      />
                    </FormControl>

                    <FormControl>
                      <FormLabel>Catatan</FormLabel>
                      <Textarea
                        value={pphPaymentDraft.notes}
                        onChange={(event) =>
                          setPPhPaymentDraft((prev) => ({
                            ...prev,
                            notes: event.target.value,
                          }))
                        }
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
                <Button leftIcon={<FiDollarSign />} colorScheme="orange" onClick={handlePayPPh} isLoading={submitting} isDisabled={!selectedPayroll || cashBankLoading || cashBanks.length === 0}>
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
