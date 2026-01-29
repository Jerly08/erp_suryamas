'use client';

import React, { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import SimpleLayout from '@/components/layout/SimpleLayout';
import { useTranslation } from '@/hooks/useTranslation';
import SalesSummaryModal from '@/components/reports/SalesSummaryModal';
import PurchaseReportModal from '@/components/reports/PurchaseReportModal';
import {
  Box,
  Heading,
  Text,
  SimpleGrid,
  Grid,
  Button,
  VStack,
  HStack,
  useToast,
  Card,
  CardBody,
  Icon,
  Flex,
  Badge,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalFooter,
  ModalBody,
  ModalCloseButton,
  FormControl,
  FormLabel,
  Input,
  Select,
  Spinner,
  useColorModeValue,
  useDisclosure,
  Menu,
  MenuButton,
  MenuList,
  MenuItem,
  MenuDivider,
  Table,
  Thead,
  Tbody,
  Tfoot,
  Tr,
  Th,
  Td,
  TableContainer,
  Divider,
  Tooltip,
} from '@chakra-ui/react';
import {
  FiFileText,
  FiBarChart,
  FiTrendingUp,
  FiShoppingCart,
  FiActivity,
  FiDownload,
  FiEye,
  FiList,
  FiBook,
  FiDatabase,
  FiFilePlus,
  FiChevronDown,
  FiDollarSign,
  FiUsers,
  FiTruck,
  FiCalendar,
  FiClock,
  FiInfo
} from 'react-icons/fi';
// Legacy reportService removed - now using SSOT services only
import { ssotBalanceSheetReportService, SSOTBalanceSheetData } from '../../src/services/ssotBalanceSheetReportService';
import { ssotCashFlowReportService, SSOTCashFlowData } from '../../src/services/ssotCashFlowReportService';
import { ssotProfitLossService, SSOTProfitLossData } from '../../src/services/ssotProfitLossService';
import { ssotSalesSummaryService, SSOTSalesSummaryData } from '../../src/services/ssotSalesSummaryService';
// Vendor Analysis removed - replaced with Purchase Report
import { ssotTrialBalanceService, SSOTTrialBalanceData } from '../../src/services/ssotTrialBalanceService';
import { ssotGeneralLedgerService, SSOTGeneralLedgerData } from '../../src/services/ssotGeneralLedgerService';
import { reportService, ReportParameters } from '../../src/services/reportService';
import api from '@/services/api';
import { API_ENDPOINTS } from '@/config/api';
import { ssotPurchaseReportService, SSOTPurchaseReportData } from '../../src/services/ssotPurchaseReportService';
// Import Cash Flow Export Service
import cashFlowExportService from '../../src/services/cashFlowExportService';
// Import Closing History Modal
import ClosingHistoryModal from '../../src/components/reports/ClosingHistoryModal';

// Define reports data matching the UI design
const getAvailableReports = (t: any) => [
  {
    id: 'profit-loss',
    name: t('reports.profitLossStatement'),
    description: t('reports.description.profitLoss'),
    type: 'FINANCIAL',
    icon: FiTrendingUp
  },
  /*{
    id: 'balance-sheet',
    name: t('reports.balanceSheet.title'),
    description: t('reports.description.balanceSheet'),
    type: 'FINANCIAL',
    icon: FiBarChart
  },*/
  {
    id: 'cash-flow',
    name: t('reports.cashFlowStatement'),
    description: t('reports.description.cashFlow'),
    type: 'FINANCIAL',
    icon: FiActivity
  },
  {
    id: 'sales-summary',
    name: t('reports.salesSummaryReport'),
    description: t('reports.description.salesSummary'),
    type: 'OPERATIONAL',
    icon: FiShoppingCart
  },
  {
    id: 'purchase-report',
    name: t('reports.purchaseReport'),
    description: t('reports.description.purchaseReport'),
    type: 'OPERATIONAL',
    icon: FiShoppingCart
  },
  {
    id: 'trial-balance',
    name: t('reports.trialBalance.title'),
    description: t('reports.description.trialBalance'),
    type: 'FINANCIAL',
    icon: FiList
  },
  /*{
    id: 'general-ledger',
    name: t('reports.generalLedger'),
    description: t('reports.description.generalLedger'),
    type: 'FINANCIAL',
    icon: FiBook
  },*/
  {
    id: 'customer-history',
    name: t('reports.customerHistory'),
    description: t('reports.description.customerHistory'),
    type: 'OPERATIONAL',
    icon: FiUsers
  },
  {
    id: 'vendor-history',
    name: t('reports.vendorHistory'),
    description: t('reports.description.vendorHistory'),
    type: 'OPERATIONAL',
    icon: FiTruck
  }
];

// Helpers to compute fiscal year range from Settings.fiscal_year_start
const MONTHS = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];
const toISO = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
function computeFiscalRangeFromString(fyStart: string): { startISO: string, endISO: string } {
  const lower = (fyStart || 'January 1').toLowerCase();
  let m = 1, d = 1;
  const idx = MONTHS.findIndex(mm => lower.startsWith(mm.toLowerCase()));
  if (idx >= 0) {
    const match = lower.match(/(\d{1,2})/);
    d = Math.min(Math.max(parseInt(match?.[1] || '1', 10), 1), 31);
    m = idx + 1;
  } else {
    const parts = lower.split(/[-/\s]/).filter(Boolean);
    if (parts.length >= 2) {
      const mm = parseInt(parts[0], 10); const dd = parseInt(parts[1], 10);
      if (mm >= 1 && mm <= 12 && dd >= 1 && dd <= 31) { m = mm; d = dd; }
    }
  }
  const today = new Date();
  const thisYearStart = new Date(Date.UTC(today.getFullYear(), m - 1, d));
  let start = thisYearStart;
  if (today.getTime() < thisYearStart.getTime()) start = new Date(Date.UTC(today.getFullYear() - 1, m - 1, d));
  const nextStart = new Date(Date.UTC(start.getUTCFullYear() + 1, m - 1, d));
  const end = new Date(nextStart.getTime() - 24 * 60 * 60 * 1000);
  return { startISO: toISO(new Date(start.getUTCFullYear(), start.getUTCMonth(), start.getUTCDate())), endISO: toISO(new Date(end.getUTCFullYear(), end.getUTCMonth(), end.getUTCDate())) };
}

const ReportsPage: React.FC = () => {
  const { t } = useTranslation();
  const router = useRouter();
  const [loading, setLoading] = useState(false);
  // Legacy states removed - now using SSOT system only
  const toast = useToast();

  // Color mode values
  const cardBg = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const headingColor = useColorModeValue('gray.700', 'white');
  const textColor = useColorModeValue('gray.800', 'white');
  const descriptionColor = useColorModeValue('gray.600', 'gray.300');
  const modalContentBg = useColorModeValue('white', 'gray.800');
  const summaryBg = useColorModeValue('gray.50', 'gray.700');
  const loadingTextColor = useColorModeValue('gray.700', 'gray.300');
  const previewPeriodTextColor = useColorModeValue('gray.500', 'gray.400');

  const availableReports = getAvailableReports(t);

  // State untuk SSOT Profit Loss
  const [ssotPLOpen, setSSOTPLOpen] = useState(false);
  const [ssotPLData, setSSOTPLData] = useState<SSOTProfitLossData | null>(null);
  const [ssotPLLoading, setSSOTPLLoading] = useState(false);
  const [ssotPLError, setSSOTPLError] = useState<string | null>(null);
  const [ssotStartDate, setSSOTStartDate] = useState('');
  const [ssotEndDate, setSSOTEndDate] = useState('');
  const [ssotPLClosedPeriods, setSSOTPLClosedPeriods] = useState<any[]>([]);
  const [loadingSSOTPLClosedPeriods, setLoadingSSOTPLClosedPeriods] = useState(false);

  // State untuk SSOT Balance Sheet
  const [ssotBSOpen, setSSOTBSOpen] = useState(false);
  const [ssotBSData, setSSOTBSData] = useState<SSOTBalanceSheetData | null>(null);
  const [ssotBSLoading, setSSOTBSLoading] = useState(false);
  const [ssotBSError, setSSOTBSError] = useState<string | null>(null);
  const [ssotAsOfDate, setSSOTAsOfDate] = useState('');
  const [closedPeriods, setClosedPeriods] = useState<any[]>([]);
  const [loadingClosedPeriods, setLoadingClosedPeriods] = useState(false);

  // State untuk SSOT Cash Flow
  const [ssotCFOpen, setSSOTCFOpen] = useState(false);
  const [ssotCFData, setSSOTCFData] = useState<SSOTCashFlowData | null>(null);
  const [ssotCFLoading, setSSOTCFLoading] = useState(false);
  const [ssotCFError, setSSOTCFError] = useState<string | null>(null);
  const [ssotCFStartDate, setSSOTCFStartDate] = useState('');
  const [ssotCFEndDate, setSSOTCFEndDate] = useState('');
  const [ssotCFClosedPeriods, setSSOTCFClosedPeriods] = useState<any[]>([]);
  const [loadingSSOTCFClosedPeriods, setLoadingSSOTCFClosedPeriods] = useState(false);

  // State untuk SSOT Sales Summary
  const [ssotSSOpen, setSSOTSSOpen] = useState(false);
  const [ssotSSData, setSSOTSSData] = useState<SSOTSalesSummaryData | null>(null);
  const [ssotSSLoading, setSSOTSSLoading] = useState(false);
  const [ssotSSError, setSSOTSSError] = useState<string | null>(null);
  const [ssotSSStartDate, setSSOTSSStartDate] = useState('');
  const [ssotSSEndDate, setSSOTSSEndDate] = useState('');

  // State untuk SSOT Purchase Report
  const [ssotPROpen, setSSOTPROpen] = useState(false);
  const [ssotPRData, setSSOTPRData] = useState<SSOTPurchaseReportData | null>(null);
  const [ssotPRLoading, setSSOTPRLoading] = useState(false);
  const [ssotPRError, setSSOTPRError] = useState<string | null>(null);
  const [ssotPRStartDate, setSSOTPRStartDate] = useState('');
  const [ssotPREndDate, setSSOTPREndDate] = useState('');

  // State untuk SSOT Trial Balance
  const [ssotTBOpen, setSSOTTBOpen] = useState(false);
  const [ssotTBData, setSSOTTBData] = useState<SSOTTrialBalanceData | null>(null);
  const [ssotTBLoading, setSSOTTBLoading] = useState(false);
  const [ssotTBError, setSSOTTBError] = useState<string | null>(null);
  const [ssotTBAsOfDate, setSSOTTBAsOfDate] = useState('');
  const [ssotTBClosedPeriods, setSSOTTBClosedPeriods] = useState<any[]>([]);
  const [loadingSSOTTBClosedPeriods, setLoadingSSOTTBClosedPeriods] = useState(false);

  // State untuk SSOT General Ledger
  const [ssotGLOpen, setSSOTGLOpen] = useState(false);
  const [ssotGLData, setSSOTGLData] = useState<SSOTGeneralLedgerData | null>(null);
  const [ssotGLLoading, setSSOTGLLoading] = useState(false);
  const [ssotGLError, setSSOTGLError] = useState<string | null>(null);
  const [ssotGLStartDate, setSSOTGLStartDate] = useState('');
  const [ssotGLEndDate, setSSOTGLEndDate] = useState('');
  const [ssotGLAccountId, setSSOTGLAccountId] = useState<string>('');
  const [ssotGLClosedPeriods, setSSOTGLClosedPeriods] = useState<any[]>([]);
  const [loadingSSOTGLClosedPeriods, setLoadingSSOTGLClosedPeriods] = useState(false);

  // State untuk Customer History
  const [customerHistoryOpen, setCustomerHistoryOpen] = useState(false);
  const [customerHistoryData, setCustomerHistoryData] = useState<any>(null);
  const [customerHistoryLoading, setCustomerHistoryLoading] = useState(false);
  const [customerHistoryError, setCustomerHistoryError] = useState<string | null>(null);
  const [customerHistoryStartDate, setCustomerHistoryStartDate] = useState('');
  const [customerHistoryEndDate, setCustomerHistoryEndDate] = useState('');
  const [customerHistoryCustomerId, setCustomerHistoryCustomerId] = useState<string>('');
  const [customers, setCustomers] = useState<any[]>([]);

  // State untuk Vendor History
  const [vendorHistoryOpen, setVendorHistoryOpen] = useState(false);
  const [vendorHistoryData, setVendorHistoryData] = useState<any>(null);
  const [vendorHistoryLoading, setVendorHistoryLoading] = useState(false);
  const [vendorHistoryError, setVendorHistoryError] = useState<string | null>(null);
  const [vendorHistoryStartDate, setVendorHistoryStartDate] = useState('');
  const [vendorHistoryEndDate, setVendorHistoryEndDate] = useState('');
  const [vendorHistoryVendorId, setVendorHistoryVendorId] = useState<string>('');
  const [vendors, setVendors] = useState<any[]>([]);

  // State untuk Closing History Modal
  const [closingHistoryOpen, setClosingHistoryOpen] = useState(false);
  const [closingHistoryReportType, setClosingHistoryReportType] = useState<string>('');

  // Modal and report generation states
  const { isOpen, onOpen, onClose } = useDisclosure();
  const [selectedReport, setSelectedReport] = useState<any>(null);
  const [reportParams, setReportParams] = useState<ReportParameters>({});
  const [previewReport, setPreviewReport] = useState<any>(null);

  const resetParams = () => {
    setReportParams({});
  };

  // Load settings to default all report ranges to current fiscal year
  useEffect(() => {
    const load = async () => {
      try {
        const resp = await api.get(API_ENDPOINTS.SETTINGS);
        const fyStartStr: string = resp.data?.data?.fiscal_year_start || 'January 1';
        const range = computeFiscalRangeFromString(fyStartStr);
        // Set for PL
        setSSOTStartDate(range.startISO); setSSOTEndDate(range.endISO);
        // Balance Sheet as-of -> fiscal year end
        setSSOTAsOfDate(range.endISO);
        // Cash Flow
        setSSOTCFStartDate(range.startISO); setSSOTCFEndDate(range.endISO);
        // Sales Summary
        setSSOTSSStartDate(range.startISO); setSSOTSSEndDate(range.endISO);
        // Purchase Report
        setSSOTPRStartDate(range.startISO); setSSOTPREndDate(range.endISO);
        // Trial Balance as-of
        setSSOTTBAsOfDate(range.endISO);
        // General Ledger
        setSSOTGLStartDate(range.startISO); setSSOTGLEndDate(range.endISO);
        // Customer History
        setCustomerHistoryStartDate(range.startISO); setCustomerHistoryEndDate(range.endISO);
        // Vendor History
        setVendorHistoryStartDate(range.startISO); setVendorHistoryEndDate(range.endISO);
      } catch (e) {
        // Fallback to current calendar year
        const today = new Date();
        const start = new Date(today.getFullYear(), 0, 1);
        const end = new Date(today.getFullYear(), 11, 31);
        const s = toISO(start), eiso = toISO(end);
        setSSOTStartDate(s); setSSOTEndDate(eiso);
        setSSOTAsOfDate(eiso);
        setSSOTCFStartDate(s); setSSOTCFEndDate(eiso);
        setSSOTSSStartDate(s); setSSOTSSEndDate(eiso);
        setSSOTPRStartDate(s); setSSOTPREndDate(eiso);
        setSSOTTBAsOfDate(eiso);
        setSSOTGLStartDate(s); setSSOTGLEndDate(eiso);
        setCustomerHistoryStartDate(s); setCustomerHistoryEndDate(eiso);
        setVendorHistoryStartDate(s); setVendorHistoryEndDate(eiso);
      }
    };
    load();
  }, []);

  // Load customers and vendors for filter dropdowns
  useEffect(() => {
    const loadContacts = async () => {
      try {
        console.log('Loading customers and vendors...');

        // Load customers
        const customersResp = await api.get('/api/v1/contacts', {
          params: { type: 'CUSTOMER', limit: 1000 }
        });
        console.log('Customers response:', customersResp.data);

        if (customersResp.data?.data) {
          console.log('Setting customers:', customersResp.data.data.length, 'items');
          setCustomers(customersResp.data.data);
        } else if (Array.isArray(customersResp.data)) {
          // Fallback if data is directly in response
          console.log('Setting customers (direct array):', customersResp.data.length, 'items');
          setCustomers(customersResp.data);
        }

        // Load vendors
        const vendorsResp = await api.get('/api/v1/contacts', {
          params: { type: 'VENDOR', limit: 1000 }
        });
        console.log('Vendors response:', vendorsResp.data);

        if (vendorsResp.data?.data) {
          console.log('Setting vendors:', vendorsResp.data.data.length, 'items');
          setVendors(vendorsResp.data.data);
        } else if (Array.isArray(vendorsResp.data)) {
          // Fallback if data is directly in response
          console.log('Setting vendors (direct array):', vendorsResp.data.length, 'items');
          setVendors(vendorsResp.data);
        }
      } catch (error: any) {
        console.error('Failed to load contacts:', error);
        console.error('Error details:', error.response?.data || error.message);
        toast({
          title: t('reports.messages.errorLoadingContacts'),
          description: t('reports.messages.failedToLoadContacts'),
          status: 'error',
          duration: 5000,
          isClosable: true,
        });
      }
    };
    loadContacts();
  }, []);

  // Function untuk fetch SSOT Sales Summary Report
  const fetchSSOTSalesSummaryReport = async () => {
    setSSOTSSLoading(true);
    setSSOTSSError(null);

    try {
      const salesSummaryData = await ssotSalesSummaryService.generateSSOTSalesSummary({
        start_date: ssotSSStartDate,
        end_date: ssotSSEndDate,
        format: 'json'
      });

      console.log('SSOT Sales Summary Data received:', salesSummaryData);
      setSSOTSSData(salesSummaryData);

      toast({
        title: t('reports.messages.success'),
        description: t('reports.messages.salesSummaryGenerated'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    } catch (error: any) {
      setSSOTSSError(error.message || t('reports.messages.failedToGenerateSalesSummary'));
      toast({
        title: t('reports.messages.error'),
        description: error.message || t('reports.messages.failedToGenerateSalesSummary'),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setSSOTSSLoading(false);
    }
  };

  // Function untuk fetch SSOT Purchase Report
  const fetchSSOTPurchaseReport = async () => {
    setSSOTPRLoading(true);
    setSSOTPRError(null);

    try {
      const data: SSOTPurchaseReportData = await ssotPurchaseReportService.generateSSOTPurchaseReport({
        start_date: ssotPRStartDate,
        end_date: ssotPREndDate,
        format: 'json'
      });

      console.log('SSOT Purchase Report Data received:', data);
      setSSOTPRData(data);

      toast({
        title: t('reports.messages.success'),
        description: t('reports.messages.purchaseReportGenerated'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    } catch (error: any) {
      setSSOTPRError(error.message || t('reports.messages.failedToGeneratePurchaseReport'));
      toast({
        title: t('reports.messages.error'),
        description: error.message || t('reports.messages.failedToGeneratePurchaseReport'),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setSSOTPRLoading(false);
    }
  };

  // Function untuk fetch SSOT Trial Balance Report
  const fetchSSOTTrialBalanceReport = async () => {
    setSSOTTBLoading(true);
    setSSOTTBError(null);

    try {
      const trialBalanceData = await ssotTrialBalanceService.generateSSOTTrialBalance({
        as_of_date: ssotTBAsOfDate,
        format: 'json'
      });

      console.log('SSOT Trial Balance Data received:', trialBalanceData);
      setSSOTTBData(trialBalanceData);

      toast({
        title: t('reports.messages.success'),
        description: t('reports.messages.trialBalanceGenerated'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    } catch (error: any) {
      setSSOTTBError(error.message || t('reports.messages.failedToGenerateTrialBalance'));
      toast({
        title: t('reports.messages.error'),
        description: error.message || t('reports.messages.failedToGenerateTrialBalance'),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setSSOTTBLoading(false);
    }
  };

  // Function untuk fetch Customer History Report
  const fetchCustomerHistory = async () => {
    // Validasi input
    if (!customerHistoryCustomerId) {
      toast({
        title: t('reports.messages.validationError'),
        description: t('reports.messages.pleaseSelectCustomer'),
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    if (!customerHistoryStartDate || !customerHistoryEndDate) {
      toast({
        title: t('reports.messages.validationError'),
        description: t('reports.messages.startEndDateRequired'),
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    // Validasi start date tidak boleh lebih besar dari end date
    if (new Date(customerHistoryStartDate) > new Date(customerHistoryEndDate)) {
      toast({
        title: t('reports.messages.validationError'),
        description: t('reports.messages.startDateCannotBeGreater'),
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    setCustomerHistoryLoading(true);
    setCustomerHistoryError(null);

    try {
      const response = await api.get('/api/v1/reports/customer-history', {
        params: {
          customer_id: customerHistoryCustomerId,
          start_date: customerHistoryStartDate,
          end_date: customerHistoryEndDate,
          format: 'json'
        }
      });

      console.log('Customer History Data received:', response.data);
      setCustomerHistoryData(response.data.data || response.data);

      toast({
        title: t('reports.messages.success'),
        description: t('reports.messages.customerHistoryGenerated'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    } catch (error: any) {
      const errorMessage = error.response?.data?.message || error.message || t('reports.messages.failedToGenerateCustomerHistory');
      setCustomerHistoryError(errorMessage);
      toast({
        title: t('reports.messages.error'),
        description: errorMessage,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setCustomerHistoryLoading(false);
    }
  };

  // Function untuk fetch Vendor History Report
  const fetchVendorHistory = async () => {
    // Validasi input
    if (!vendorHistoryVendorId) {
      toast({
        title: t('reports.messages.validationError'),
        description: t('reports.messages.pleaseSelectVendor'),
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    if (!vendorHistoryStartDate || !vendorHistoryEndDate) {
      toast({
        title: t('reports.messages.validationError'),
        description: t('reports.messages.startEndDateRequired'),
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    // Validasi start date tidak boleh lebih besar dari end date
    if (new Date(vendorHistoryStartDate) > new Date(vendorHistoryEndDate)) {
      toast({
        title: t('reports.messages.validationError'),
        description: t('reports.messages.startDateCannotBeGreater'),
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    setVendorHistoryLoading(true);
    setVendorHistoryError(null);

    try {
      const response = await api.get('/api/v1/reports/vendor-history', {
        params: {
          vendor_id: vendorHistoryVendorId,
          start_date: vendorHistoryStartDate,
          end_date: vendorHistoryEndDate,
          format: 'json'
        }
      });

      console.log('Vendor History Data received:', response.data);
      setVendorHistoryData(response.data.data || response.data);

      toast({
        title: t('reports.messages.success'),
        description: t('reports.messages.vendorHistoryGenerated'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    } catch (error: any) {
      const errorMessage = error.response?.data?.message || error.message || t('reports.messages.failedToGenerateVendorHistory');
      setVendorHistoryError(errorMessage);
      toast({
        title: t('reports.messages.error'),
        description: errorMessage,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setVendorHistoryLoading(false);
    }
  };

  // Function untuk fetch SSOT General Ledger Report
  const fetchSSOTGeneralLedgerReport = async () => {
    // Validasi input
    if (!ssotGLStartDate || !ssotGLEndDate) {
      toast({
        title: t('reports.messages.validationError'),
        description: t('reports.messages.startEndDateRequiredForGL'),
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    // Validasi start date tidak boleh lebih besar dari end date
    if (new Date(ssotGLStartDate) > new Date(ssotGLEndDate)) {
      toast({
        title: t('reports.messages.validationError'),
        description: t('reports.messages.startDateCannotBeGreater'),
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    setSSOTGLLoading(true);
    setSSOTGLError(null);

    try {
      const generalLedgerData = await ssotGeneralLedgerService.generateSSOTGeneralLedger({
        start_date: ssotGLStartDate,
        end_date: ssotGLEndDate,
        account_id: ssotGLAccountId || undefined,
        format: 'json'
      });

      console.log('SSOT General Ledger Data received:', generalLedgerData);
      setSSOTGLData(generalLedgerData);

      toast({
        title: t('reports.messages.success'),
        description: t('reports.messages.generalLedgerGenerated'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    } catch (error: any) {
      const errorMessage = error.response?.data?.message || error.message || t('reports.messages.failedToGenerateGeneralLedger');
      setSSOTGLError(errorMessage);
      toast({
        title: t('reports.messages.error'),
        description: errorMessage,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setSSOTGLLoading(false);
    }
  };

  // Helper: normalisasi & deduplikasi closed periods berdasarkan tanggal kalender (YYYY-MM-DD)
  // Sekarang juga menyimpan start_date dari accounting_periods (jika tersedia)
  const buildUniqueClosedPeriods = (entries: any[]): any[] => {
    if (!Array.isArray(entries)) return [];

    // Backend returns (unified):
    // - accounting_periods: { id, code, description, entry_date, start_date, created_at, total_debit, net_income, source }
    // - ssot_journal:       { id, code, description, entry_date, created_at, total_debit, source }
    const mapped = entries
      .filter((entry: any) => entry && (entry.entry_date || entry.end_date))
      .map((entry: any) => {
        const rawEnd = String(entry.entry_date || entry.end_date);
        // Ambil hanya bagian tanggal 'YYYY-MM-DD' untuk menghindari perbedaan timezone
        const endDateKey = rawEnd.slice(0, 10);
        const endDateObj = new Date(endDateKey);
        const formattedEndDate = endDateObj.toLocaleDateString('id-ID', {
          day: '2-digit',
          month: 'short',
          year: 'numeric'
        });

        // Normalisasi start_date jika ada (accounting_periods)
        const rawStart = entry.start_date ? String(entry.start_date).slice(0, 10) : undefined;

        return {
          value: endDateKey,
          label: `${formattedEndDate} - ${entry.description || 'Period Closing'}`,
          // date tetap disimpan sebagai end_date agar kompatibel dengan existing usage
          date: endDateKey,
          start_date: rawStart,
          fiscal_year: endDateObj.getFullYear(),
          entry_id: entry.id,
          code: entry.code,
          source: entry.source,
          created_at: entry.created_at
        };
      });

    // Deduplikasi berdasarkan tanggal kalender (end_date), prioritaskan source accounting_periods
    const uniqueMap = new Map<string, any>();
    mapped.forEach((period: any) => {
      const key = period.date;
      const existing = uniqueMap.get(key);
      if (!existing) {
        uniqueMap.set(key, period);
      } else {
        if (period.source === 'accounting_periods' && existing.source !== 'accounting_periods') {
          uniqueMap.set(key, period);
        } else if (period.source === existing.source) {
          const periodCreated = period.created_at ? new Date(period.created_at).getTime() : 0;
          const existingCreated = existing.created_at ? new Date(existing.created_at).getTime() : 0;
          if (periodCreated > existingCreated) {
            uniqueMap.set(key, period);
          }
        }
      }
    });

    return Array.from(uniqueMap.values()).sort(
      (a, b) => new Date(b.date).getTime() - new Date(a.date).getTime()
    );
  };

  // Function untuk fetch closed periods
  const fetchClosedPeriods = async () => {
    setLoadingClosedPeriods(true);
    try {
      const response = await api.get(API_ENDPOINTS.FISCAL_CLOSING.HISTORY);
      console.log('[Reports] Fetching closed periods:', response.data);
      console.log('[Reports] Raw data:', JSON.stringify(response.data, null, 2));

      if (response.data.success && response.data.data && Array.isArray(response.data.data)) {
        const periods = buildUniqueClosedPeriods(response.data.data);

        setClosedPeriods(periods);
        console.log(
          `[Reports] Loaded ${periods.length} unique closed periods (from ${response.data.data.length} total entries):`,
          periods
        );

        if (response.data.data.length > periods.length) {
          console.log(
            `[Reports] Deduplicated ${response.data.data.length - periods.length} duplicate entries`
          );
        }

        if (periods.length === 0) {
          console.warn('[Reports] No valid closed periods found - entries may not have entry_date');
        }
      } else {
        console.log('[Reports] No closed periods found in response or invalid data structure');
        console.log('[Reports] Response structure:', {
          hasSuccess: !!response.data.success,
          hasData: !!response.data.data,
          dataType: typeof response.data.data,
          isArray: Array.isArray(response.data.data)
        });
        setClosedPeriods([]);
      }
    } catch (error: any) {
      console.error('[Reports] Error fetching closed periods:', error);
      console.error('[Reports] Error details:', error.response?.data || error.message);
      // Silent fail - tidak menampilkan error untuk ini
      setClosedPeriods([]);
    } finally {
      setLoadingClosedPeriods(false);
    }
  };

  // Function untuk fetch SSOT Balance Sheet Report
  const fetchSSOTBalanceSheetReport = async () => {
    setSSOTBSLoading(true);
    setSSOTBSError(null);

    try {
      const balanceSheetData = await ssotBalanceSheetReportService.generateSSOTBalanceSheet({
        as_of_date: ssotAsOfDate,
        format: 'json'
      });

      console.log('SSOT Balance Sheet Data received:', balanceSheetData);
      setSSOTBSData(balanceSheetData);

      toast({
        title: t('reports.messages.success'),
        description: t('reports.messages.balanceSheetGenerated'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });

    } catch (error) {
      console.error('Error fetching SSOT Balance Sheet report:', error);
      const errorMessage = error instanceof Error ? error.message : t('reports.messages.unknownError');
      setSSOTBSError(errorMessage);

      toast({
        title: t('reports.messages.error'),
        description: errorMessage,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setSSOTBSLoading(false);
    }
  };

  // Function untuk fetch SSOT Cash Flow Report
  const fetchSSOTCashFlowReport = async () => {
    setSSOTCFLoading(true);
    setSSOTCFError(null);

    try {
      const cashFlowData = await ssotCashFlowReportService.generateSSOTCashFlow({
        start_date: ssotCFStartDate,
        end_date: ssotCFEndDate,
        format: 'json'
      });

      // Debug: Log the data structure
      console.log('Cash Flow Data Structure:', JSON.stringify(cashFlowData, null, 2));
      console.log('Operating Activities:', cashFlowData?.operating_activities);
      console.log('Investing Activities:', cashFlowData?.investing_activities);
      console.log('Financing Activities:', cashFlowData?.financing_activities);

      setSSOTCFData(cashFlowData as SSOTCashFlowData);

      toast({
        title: t('reports.messages.success'),
        description: t('reports.messages.cashFlowGenerated'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });

    } catch (error) {
      console.error('Error fetching SSOT Cash Flow report:', error);
      const errorMessage = error instanceof Error ? error.message : t('reports.messages.unknownError');
      setSSOTCFError(errorMessage);

      toast({
        title: t('reports.messages.error'),
        description: errorMessage,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setSSOTCFLoading(false);
    }
  };

  // Function untuk fetch SSOT P&L Report
  const fetchSSOTPLReport = async () => {
    setSSOTPLLoading(true);
    setSSOTPLError(null);

    try {
      const result = await ssotProfitLossService.generateSSOTProfitLoss({
        start_date: ssotStartDate,
        end_date: ssotEndDate,
        format: 'json'
      });

      console.log('SSOT P&L Data received:', result);
      setSSOTPLData(result);

      toast({
        title: t('reports.messages.success'),
        description: t('reports.messages.profitLossGenerated'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });

    } catch (error) {
      console.error('Error fetching SSOT P&L report:', error);
      const errorMessage = error instanceof Error ? error.message : t('reports.messages.unknownError');
      setSSOTPLError(errorMessage);

      toast({
        title: t('reports.messages.error'),
        description: errorMessage,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setSSOTPLLoading(false);
    }
  };

  // Enhanced export handlers for SSOT Profit Loss
  const handleSSOTProfitLossExport = async (format: 'pdf' | 'csv') => {
    if (!ssotPLData) {
      toast({
        title: t('reports.messages.error'),
        description: t('reports.messages.noDataToExport'),
        status: 'error',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    try {
      let blob: Blob;
      let filename: string;

      if (format === 'pdf') {
        blob = await ssotProfitLossService.generateSSOTProfitLossPDF({
          start_date: ssotStartDate,
          end_date: ssotEndDate,
          format: 'pdf'
        });
        filename = `SSOT_Profit_Loss_${ssotStartDate}_to_${ssotEndDate}.pdf`;
      } else {
        blob = await ssotProfitLossService.generateSSOTProfitLossCSV({
          start_date: ssotStartDate,
          end_date: ssotEndDate,
          format: 'csv'
        });
        filename = `SSOT_Profit_Loss_${ssotStartDate}_to_${ssotEndDate}.csv`;
      }

      // Create download link
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);

      toast({
        title: t('reports.messages.success'),
        description: t('reports.messages.exportCompleted', { format: format.toUpperCase() }),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    } catch (error) {
      console.error(`Error exporting SSOT P&L as ${format}:`, error);
      toast({
        title: t('reports.messages.error'),
        description: t('reports.messages.exportFailed', { format: format.toUpperCase() }),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    }
  };

  // Enhanced export handlers for Balance Sheet
  const handleEnhancedCSVExport = async (balanceSheetData: SSOTBalanceSheetData) => {
    try {
      // Create simple CSV content directly from the balance sheet data
      const csvLines: string[] = [];

      // Helper function to escape CSV values
      const escapeCSV = (value: string | number | null | undefined): string => {
        if (value === null || value === undefined) return '';
        const str = String(value);
        if (str.includes(',') || str.includes('"') || str.includes('\n')) {
          return `"${str.replace(/"/g, '""')}"`;
        }
        return str;
      };

      // Helper function to format currency
      const formatCurrency = (amount: number | null | undefined): string => {
        if (amount === null || amount === undefined || isNaN(Number(amount))) {
          return '0';
        }
        return Number(amount).toLocaleString('id-ID');
      };

      // Header
      csvLines.push(escapeCSV(balanceSheetData.company?.name || 'PT. Sistem Akuntansi'));
      csvLines.push('BALANCE SHEET');
      csvLines.push(`As of: ${balanceSheetData.as_of_date || new Date().toISOString().split('T')[0]}`);
      csvLines.push(`Generated on: ${new Date().toLocaleString('id-ID')}`);
      csvLines.push(''); // Empty line

      // Summary
      csvLines.push('FINANCIAL SUMMARY');
      csvLines.push('Category,Amount');
      csvLines.push(`Total Assets,${formatCurrency(balanceSheetData.assets?.total_assets || balanceSheetData.total_assets || 0)}`);
      csvLines.push(`Total Liabilities,${formatCurrency(balanceSheetData.liabilities?.total_liabilities || balanceSheetData.total_liabilities || 0)}`);
      csvLines.push(`Total Equity,${formatCurrency(balanceSheetData.equity?.total_equity || balanceSheetData.total_equity || 0)}`);
      csvLines.push(`Total Liabilities + Equity,${formatCurrency(balanceSheetData.total_liabilities_and_equity || 0)}`);
      csvLines.push(`Balanced,${balanceSheetData.is_balanced ? 'Yes' : 'No'}`);
      if (!balanceSheetData.is_balanced && balanceSheetData.balance_difference) {
        csvLines.push(`Balance Difference,${formatCurrency(balanceSheetData.balance_difference)}`);
      }
      csvLines.push(''); // Empty line

      // Detailed breakdown
      csvLines.push('DETAILED BREAKDOWN');
      csvLines.push('Account Code,Account Name,Category,Amount');

      // Assets
      csvLines.push('ASSETS,,,');

      // Current Assets
      if (balanceSheetData.assets?.current_assets?.items && balanceSheetData.assets.current_assets.items.length > 0) {
        csvLines.push('Current Assets,,,');
        balanceSheetData.assets.current_assets.items.forEach(item => {
          csvLines.push([
            escapeCSV(item.account_code || ''),
            escapeCSV(item.account_name || ''),
            'Current Asset',
            formatCurrency(item.amount || 0)
          ].join(','));
        });
        csvLines.push(`Subtotal Current Assets,,,${formatCurrency(balanceSheetData.assets.current_assets.total_current_assets || 0)}`);
        csvLines.push('');
      }

      // Non-Current Assets
      if (balanceSheetData.assets?.non_current_assets?.items && balanceSheetData.assets.non_current_assets.items.length > 0) {
        csvLines.push('Non-Current Assets,,,');
        balanceSheetData.assets.non_current_assets.items.forEach(item => {
          csvLines.push([
            escapeCSV(item.account_code || ''),
            escapeCSV(item.account_name || ''),
            'Non-Current Asset',
            formatCurrency(item.amount || 0)
          ].join(','));
        });
        csvLines.push(`Subtotal Non-Current Assets,,,${formatCurrency(balanceSheetData.assets.non_current_assets.total_non_current_assets || 0)}`);
        csvLines.push('');
      }

      csvLines.push(`TOTAL ASSETS,,,${formatCurrency(balanceSheetData.assets?.total_assets || 0)}`);
      csvLines.push(''); // Empty line

      // Liabilities
      csvLines.push('LIABILITIES,,,');

      // Current Liabilities
      if (balanceSheetData.liabilities?.current_liabilities?.items && balanceSheetData.liabilities.current_liabilities.items.length > 0) {
        csvLines.push('Current Liabilities,,,');
        balanceSheetData.liabilities.current_liabilities.items.forEach(item => {
          csvLines.push([
            escapeCSV(item.account_code || ''),
            escapeCSV(item.account_name || ''),
            'Current Liability',
            formatCurrency(item.amount || 0)
          ].join(','));
        });
        csvLines.push(`Subtotal Current Liabilities,,,${formatCurrency(balanceSheetData.liabilities.current_liabilities.total_current_liabilities || 0)}`);
        csvLines.push('');
      }

      // Non-Current Liabilities
      if (balanceSheetData.liabilities?.non_current_liabilities?.items && balanceSheetData.liabilities.non_current_liabilities.items.length > 0) {
        csvLines.push('Non-Current Liabilities,,,');
        balanceSheetData.liabilities.non_current_liabilities.items.forEach(item => {
          csvLines.push([
            escapeCSV(item.account_code || ''),
            escapeCSV(item.account_name || ''),
            'Non-Current Liability',
            formatCurrency(item.amount || 0)
          ].join(','));
        });
        csvLines.push(`Subtotal Non-Current Liabilities,,,${formatCurrency(balanceSheetData.liabilities.non_current_liabilities.total_non_current_liabilities || 0)}`);
        csvLines.push('');
      }

      csvLines.push(`TOTAL LIABILITIES,,,${formatCurrency(balanceSheetData.liabilities?.total_liabilities || 0)}`);
      csvLines.push(''); // Empty line

      // Equity
      if (balanceSheetData.equity?.items && balanceSheetData.equity.items.length > 0) {
        csvLines.push('EQUITY,,,');
        balanceSheetData.equity.items.forEach(item => {
          csvLines.push([
            escapeCSV(item.account_code || ''),
            escapeCSV(item.account_name || ''),
            'Equity',
            formatCurrency(item.amount || 0)
          ].join(','));
        });
        csvLines.push('');
      }

      csvLines.push(`TOTAL EQUITY,,,${formatCurrency(balanceSheetData.equity?.total_equity || 0)}`);
      csvLines.push(''); // Empty line
      csvLines.push(`TOTAL LIABILITIES + EQUITY,,,${formatCurrency(balanceSheetData.total_liabilities_and_equity || 0)}`);

      // Footer
      csvLines.push('');
      csvLines.push('Generated by Sistem Akuntansi');
      csvLines.push(`Report Date: ${new Date().toLocaleString('id-ID')}`);
      csvLines.push(`Data Source: ${balanceSheetData.enhanced ? 'SSOT Enhanced' : 'SSOT Standard'}`);

      const csvContent = csvLines.join('\n');

      // Create and trigger download in the browser
      const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');

      link.setAttribute('href', url);
      link.setAttribute('download', `balance_sheet_${ssotAsOfDate}.csv`);
      link.style.visibility = 'hidden';
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);

      toast({
        title: t('reports.messages.csvExportSuccessful'),
        description: t('reports.messages.balanceSheetDownloadedCSV'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    } catch (error: any) {
      toast({
        title: t('reports.messages.csvExportFailed'),
        description: error.message || t('reports.messages.csvExportError'),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    }
  };

  const handleEnhancedPDFExport = async (balanceSheetData: SSOTBalanceSheetData) => {
    try {
      // Use the new PDF generation method from the service
      const { ssotBalanceSheetReportService } = await import('../../src/services/ssotBalanceSheetReportService');
      const pdfBlob = await ssotBalanceSheetReportService.generateSSOTBalanceSheetPDF({
        as_of_date: ssotAsOfDate
      });

      // Create download link
      const url = window.URL.createObjectURL(pdfBlob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `SSOT_BalanceSheet_${ssotAsOfDate}.pdf`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);

      toast({
        title: t('reports.messages.pdfExportSuccessful'),
        description: t('reports.messages.balanceSheetDownloadedPDF'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    } catch (error: any) {
      toast({
        title: t('reports.messages.pdfExportFailed'),
        description: error.message || t('reports.messages.pdfExportError'),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    }
  };

  // Cash Flow Export Handlers
  const handleCashFlowCSVExport = async () => {
    if (!ssotCFData || !ssotCFStartDate || !ssotCFEndDate) {
      toast({
        title: 'Export Failed',
        description: 'No Cash Flow data available or missing date parameters',
        status: 'error',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    setLoading(true);
    try {
      await cashFlowExportService.exportToCSV({
        start_date: ssotCFStartDate,
        end_date: ssotCFEndDate
      });

      toast({
        title: t('reports.messages.csvExportSuccessful'),
        description: t('reports.messages.cashFlowDownloadedCSV'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    } catch (error: any) {
      toast({
        title: t('reports.messages.csvExportFailed'),
        description: error.message || t('reports.messages.csvExportError'),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  const handleCashFlowPDFExport = async () => {
    if (!ssotCFData || !ssotCFStartDate || !ssotCFEndDate) {
      toast({
        title: t('reports.messages.exportFailed'),
        description: t('reports.messages.noCashFlowData'),
        status: 'error',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    setLoading(true);
    try {
      await cashFlowExportService.exportToPDF({
        start_date: ssotCFStartDate,
        end_date: ssotCFEndDate
      });

      toast({
        title: t('reports.messages.pdfExportSuccessful'),
        description: t('reports.messages.cashFlowDownloadedPDF'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    } catch (error: any) {
      toast({
        title: t('reports.messages.pdfExportFailed'),
        description: error.message || t('reports.messages.pdfExportError'),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  // Trial Balance Export Handlers
  const handleTrialBalanceExport = async (format: 'pdf' | 'csv') => {
    if (!ssotTBData || !ssotTBAsOfDate) {
      toast({
        title: t('reports.messages.exportFailed'),
        description: t('reports.messages.noTrialBalanceData'),
        status: 'error',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    setLoading(true);
    try {
      if (format === 'pdf') {
        await ssotTrialBalanceService.exportToPDF({
          as_of_date: ssotTBAsOfDate
        });
      } else {
        await ssotTrialBalanceService.exportToCSV({
          as_of_date: ssotTBAsOfDate
        });
      }

      toast({
        title: t('reports.messages.exportSuccessful'),
        description: t('reports.messages.reportDownloaded', { reportName: 'Trial Balance', format: format.toUpperCase() }),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    } catch (error: any) {
      toast({
        title: t('reports.messages.exportFailed'),
        description: error.message || t('reports.messages.exportError', { format: format.toUpperCase() }),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  // General Ledger Export Handlers
  const handleGeneralLedgerExport = async (format: 'pdf' | 'csv') => {
    if (!ssotGLData || !ssotGLStartDate || !ssotGLEndDate) {
      toast({
        title: t('reports.messages.exportFailed'),
        description: t('reports.messages.noGeneralLedgerData'),
        status: 'error',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    setLoading(true);
    try {
      if (format === 'pdf') {
        await ssotGeneralLedgerService.exportToPDF({
          start_date: ssotGLStartDate,
          end_date: ssotGLEndDate,
          account_id: ssotGLAccountId || undefined
        });
      } else {
        await ssotGeneralLedgerService.exportToCSV({
          start_date: ssotGLStartDate,
          end_date: ssotGLEndDate,
          account_id: ssotGLAccountId || undefined
        });
      }

      toast({
        title: t('reports.messages.exportSuccessful'),
        description: t('reports.messages.reportDownloaded', { reportName: 'General Ledger', format: format.toUpperCase() }),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    } catch (error: any) {
      toast({
        title: t('reports.messages.exportFailed'),
        description: error.message || t('reports.messages.exportError', { format: format.toUpperCase() }),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  // Customer History Export Handler
  const handleCustomerHistoryExport = async (format: 'pdf' | 'csv') => {
    if (!customerHistoryData || !customerHistoryCustomerId || !customerHistoryStartDate || !customerHistoryEndDate) {
      toast({
        title: t('reports.messages.exportFailed'),
        description: t('reports.messages.noCustomerHistoryData'),
        status: 'error',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    setLoading(true);
    try {
      const response = await api.get('/api/v1/reports/customer-history', {
        params: {
          customer_id: customerHistoryCustomerId,
          start_date: customerHistoryStartDate,
          end_date: customerHistoryEndDate,
          format: format
        },
        responseType: format === 'pdf' ? 'blob' : 'json'
      });

      if (format === 'pdf') {
        const blob = new Blob([response.data], { type: 'application/pdf' });
        const url = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = `Customer_History_${customerHistoryStartDate}_to_${customerHistoryEndDate}.pdf`;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        window.URL.revokeObjectURL(url);
      } else {
        // CSV export - convert JSON to CSV
        const data = response.data.data || response.data;
        const csvLines: string[] = [];

        // Header
        csvLines.push(t('reports.csv.customerHistoryReport'));
        csvLines.push(`${t('reports.csv.period')}: ${customerHistoryStartDate} to ${customerHistoryEndDate}`);
        csvLines.push(`${t('reports.csv.customer')}: ${data.customer?.name || 'N/A'}`);
        csvLines.push('');

        // Transactions
        csvLines.push(`${t('reports.csv.headers.date')},${t('reports.csv.headers.type')},${t('reports.csv.headers.code')},${t('reports.csv.headers.description')},${t('reports.csv.headers.reference')},${t('reports.csv.headers.amount')},${t('reports.csv.headers.paid')},${t('reports.csv.headers.outstanding')},${t('reports.csv.headers.status')}`);
        if (data.transactions && Array.isArray(data.transactions)) {
          data.transactions.forEach((tx: any) => {
            csvLines.push([
              tx.date || '',
              tx.transaction_type || '',
              tx.transaction_code || '',
              `"${tx.description || ''}"`,
              tx.reference || '',
              tx.amount || 0,
              tx.paid_amount || 0,
              tx.outstanding || 0,
              tx.status || ''
            ].join(','));
          });
        }

        // Summary
        csvLines.push('');
        csvLines.push(t('reports.csv.summary'));
        csvLines.push(`${t('reports.csv.totalTransactions')},${data.summary?.total_transactions || 0}`);
        csvLines.push(`${t('reports.csv.totalAmount')},${data.summary?.total_amount || 0}`);
        csvLines.push(`${t('reports.csv.totalPaid')},${data.summary?.total_paid || 0}`);
        csvLines.push(`${t('reports.csv.totalOutstanding')},${data.summary?.total_outstanding || 0}`);

        const csvContent = csvLines.join('\n');
        const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
        const url = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = `Customer_History_${customerHistoryStartDate}_to_${customerHistoryEndDate}.csv`;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        window.URL.revokeObjectURL(url);
      }

      toast({
        title: t('reports.messages.exportSuccessful'),
        description: t('reports.messages.reportDownloaded', { reportName: 'Customer History', format: format.toUpperCase() }),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    } catch (error: any) {
      toast({
        title: t('reports.messages.exportFailed'),
        description: error.message || t('reports.messages.exportError', { format: format.toUpperCase() }),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  // Vendor History Export Handler
  const handleVendorHistoryExport = async (format: 'pdf' | 'csv') => {
    if (!vendorHistoryData || !vendorHistoryVendorId || !vendorHistoryStartDate || !vendorHistoryEndDate) {
      toast({
        title: t('reports.messages.exportFailed'),
        description: t('reports.messages.noVendorHistoryData'),
        status: 'error',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    setLoading(true);
    try {
      const response = await api.get('/api/v1/reports/vendor-history', {
        params: {
          vendor_id: vendorHistoryVendorId,
          start_date: vendorHistoryStartDate,
          end_date: vendorHistoryEndDate,
          format: format
        },
        responseType: format === 'pdf' ? 'blob' : 'json'
      });

      if (format === 'pdf') {
        const blob = new Blob([response.data], { type: 'application/pdf' });
        const url = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = `Vendor_History_${vendorHistoryStartDate}_to_${vendorHistoryEndDate}.pdf`;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        window.URL.revokeObjectURL(url);
      } else {
        // CSV export - convert JSON to CSV
        const data = response.data.data || response.data;
        const csvLines: string[] = [];

        // Header
        csvLines.push(t('reports.csv.vendorHistoryReport'));
        csvLines.push(`${t('reports.csv.period')}: ${vendorHistoryStartDate} to ${vendorHistoryEndDate}`);
        csvLines.push(`${t('reports.csv.vendor')}: ${data.vendor?.name || 'N/A'}`);
        csvLines.push('');

        // Transactions
        csvLines.push(`${t('reports.csv.headers.date')},${t('reports.csv.headers.type')},${t('reports.csv.headers.code')},${t('reports.csv.headers.description')},${t('reports.csv.headers.reference')},${t('reports.csv.headers.amount')},${t('reports.csv.headers.paid')},${t('reports.csv.headers.outstanding')},${t('reports.csv.headers.status')}`);
        if (data.transactions && Array.isArray(data.transactions)) {
          data.transactions.forEach((tx: any) => {
            csvLines.push([
              tx.date || '',
              tx.transaction_type || '',
              tx.transaction_code || '',
              `"${tx.description || ''}"`,
              tx.reference || '',
              tx.amount || 0,
              tx.paid_amount || 0,
              tx.outstanding || 0,
              tx.status || ''
            ].join(','));
          });
        }

        // Summary
        csvLines.push('');
        csvLines.push(t('reports.csv.summary'));
        csvLines.push(`${t('reports.csv.totalTransactions')},${data.summary?.total_transactions || 0}`);
        csvLines.push(`${t('reports.csv.totalAmount')},${data.summary?.total_amount || 0}`);
        csvLines.push(`${t('reports.csv.totalPaid')},${data.summary?.total_paid || 0}`);
        csvLines.push(`${t('reports.csv.totalOutstanding')},${data.summary?.total_outstanding || 0}`);

        const csvContent = csvLines.join('\n');
        const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
        const url = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = `Vendor_History_${vendorHistoryStartDate}_to_${vendorHistoryEndDate}.csv`;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        window.URL.revokeObjectURL(url);
      }

      toast({
        title: t('reports.messages.exportSuccessful'),
        description: t('reports.messages.reportDownloaded', { reportName: 'Vendor History', format: format.toUpperCase() }),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    } catch (error: any) {
      toast({
        title: t('reports.messages.exportFailed'),
        description: error.message || t('reports.messages.exportError', { format: format.toUpperCase() }),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  // Purchase Report Export Handler
  const handlePurchaseReportExport = async (format: 'pdf' | 'csv') => {
    if (!ssotPRData || !ssotPRStartDate || !ssotPREndDate) {
      toast({
        title: t('reports.messages.exportFailed'),
        description: t('reports.messages.noPurchaseReportData'),
        status: 'error',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    setLoading(true);
    try {
      // Import the SSOT Purchase Report Service
      const { ssotPurchaseReportService } = await import('../../src/services/ssotPurchaseReportService');

      const params = {
        start_date: ssotPRStartDate,
        end_date: ssotPREndDate,
        format: format
      };

      console.log('Exporting Purchase Report with params:', params);

      let result: Blob;
      if (format === 'pdf') {
        result = await ssotPurchaseReportService.exportToPDF(params);
      } else {
        result = await ssotPurchaseReportService.exportToCSV(params);
      }

      if (result && result.size > 0) {
        const fileName = `SSOT_Purchase_Report_${ssotPRStartDate}_to_${ssotPREndDate}.${format}`;

        // Create download link
        const url = window.URL.createObjectURL(result);
        const link = document.createElement('a');
        link.href = url;
        link.download = fileName;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        window.URL.revokeObjectURL(url);

        toast({
          title: t('reports.messages.exportSuccessful'),
          description: t('reports.messages.reportDownloaded', { reportName: 'Purchase Report', format: format.toUpperCase() }),
          status: 'success',
          duration: 3000,
          isClosable: true,
        });
      } else {
        throw new Error(t('reports.messages.emptyFileReceived'));
      }

    } catch (error: any) {
      console.error('Purchase Report export error:', error);

      toast({
        title: t('reports.messages.exportFailed'),
        description: error.message || t('reports.messages.exportError', { format: format.toUpperCase() }),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  const handleViewReport = async (report: any) => {
    setLoading(true);

    try {
      if (report.id === 'balance-sheet') {
        setSSOTBSOpen(true);
        await fetchSSOTBalanceSheetReport();
      } else if (report.id === 'profit-loss') {
        setSSOTPLOpen(true);
        await fetchSSOTPLReport();
      } else if (report.id === 'cash-flow') {
        setSSOTCFOpen(true);
        await fetchSSOTCashFlowReport();
      } else if (report.id === 'sales-summary') {
        setSSOTSSOpen(true);
        await fetchSSOTSalesSummaryReport();
      } else if (report.id === 'purchase-report') {
        setSSOTPROpen(true);
        await fetchSSOTPurchaseReport();
      } else if (report.id === 'trial-balance') {
        setSSOTTBOpen(true);
        await fetchSSOTTrialBalanceReport();
      } else if (report.id === 'general-ledger') {
        setSSOTGLOpen(true);
        await fetchSSOTGeneralLedgerReport();
      }

    } catch (error) {
      console.error('Failed to load SSOT report:', error);

      toast({
        title: t('reports.messages.reportLoadError'),
        description: error instanceof Error ? error.message : t('reports.messages.failedToLoadReport'),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  // Quick download function for PDF and CSV
  const handleQuickDownload = async (report: any, format: 'pdf' | 'csv') => {
    setLoading(true);

    try {
      // For customer-history and vendor-history, show message to use View Report first
      if (report.id === 'customer-history' || report.id === 'vendor-history') {
        toast({
          title: t('reports.messages.actionRequired'),
          description: t('reports.messages.useViewReportFirst', { entity: report.id === 'customer-history' ? t('reports.modal.customer') : t('reports.modal.vendor') }),
          status: 'info',
          duration: 6000,
          isClosable: true,
        });
        setLoading(false);
        return;
      }

      // Set default parameters based on report type
      let params: any = { format };

      if (report.id === 'balance-sheet' || report.id === 'trial-balance') {
        params.as_of_date = new Date().toISOString().split('T')[0];
      } else if (['profit-loss', 'cash-flow', 'sales-summary', 'purchase-report', 'general-ledger'].includes(report.id)) {
        const today = new Date();
        const firstDayOfMonth = new Date(today.getFullYear(), today.getMonth(), 1);
        params.start_date = firstDayOfMonth.toISOString().split('T')[0];
        params.end_date = today.toISOString().split('T')[0];

        if (report.id === 'general-ledger') {
          params.account_id = 'all';
        }
      }

      console.log('Downloading report:', report.id, 'with params:', params);

      // Generate and download the report
      const result = await reportService.generateReport(report.id, params);

      console.log('Report result type:', typeof result, 'isBlob:', result instanceof Blob);
      console.log('Report result:', result);

      if (result instanceof Blob) {
        if (result.size === 0) {
          throw new Error('Empty file received from server');
        }

        const fileName = `${report.id}_${format}_${new Date().toISOString().split('T')[0]}.${format}`;
        await reportService.downloadReport(result, fileName);

        toast({
          title: t('reports.messages.downloadSuccessful'),
          description: t('reports.messages.reportDownloaded', { reportName: report.name, format: format.toUpperCase() }),
          status: 'success',
          duration: 3000,
          isClosable: true,
        });
      } else if (typeof result === 'object' && result !== null) {
        // If it's JSON data, create a manual download
        if (format === 'csv') {
          // Convert JSON to CSV
          const csvContent = convertJSONToCSV(result, report.id);
          const blob = new Blob([csvContent], { type: 'text/csv' });
          const fileName = `${report.id}_${format}_${new Date().toISOString().split('T')[0]}.${format}`;
          await reportService.downloadReport(blob, fileName);

          toast({
            title: 'Download Successful',
            description: `${report.name} has been downloaded as CSV`,
            status: 'success',
            duration: 3000,
            isClosable: true,
          });
        } else {
          // For PDF requests, provide specific guidance
          if (format === 'pdf') {
            throw new Error(`PDF export for ${report.name} is not yet implemented. Please use the "View Report" button to access the SSOT version with export options.`);
          } else {
            throw new Error(`${format.toUpperCase()} export is not yet supported for ${report.name}. Please use the "View Report" button for available export options.`);
          }
        }
      } else {
        throw new Error(`Invalid response format from server: ${typeof result}`);
      }

    } catch (error) {
      console.error('Quick download failed:', error);

      let errorMessage = 'Failed to download report';
      if (error instanceof Error) {
        errorMessage = error.message;
      }

      // Provide user-friendly error messages
      if (errorMessage.includes('Unknown report type') || errorMessage.includes('endpoint not found')) {
        errorMessage = `${format.toUpperCase()} export is not yet supported for ${report.name}. Please use the "More..." option for available formats.`;
      }

      toast({
        title: 'Download Failed',
        description: errorMessage,
        status: 'error',
        duration: 8000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  // Helper function to convert JSON to CSV with professional formatting
  const convertJSONToCSV = (data: any, reportType: string): string => {
    try {
      if (!data) return 'No data available';

      // Unwrap common backend CSV wrappers
      const root = (data && (data as any).data) ? (data as any).data : data;
      const payload = (root && (root as any).data) ? (root as any).data : root;

      // Helpers
      const formatCurrencyForCSV = (amount: number | null | undefined): string => {
        if (amount === null || amount === undefined || isNaN(Number(amount))) return '0';
        return Number(amount).toLocaleString('id-ID');
      };
      const escapeCSV = (value: string): string => {
        if (!value) return '';
        if (value.includes(',') || value.includes('"') || value.includes('\n')) {
          return `"${value.replace(/"/g, '""')}"`;
        }
        return value;
      };

      const csvLines: string[] = [];

      // Profit & Loss
      if (reportType === 'profit-loss') {
        const companyName = payload.company?.name || 'PT. Sistem Akuntansi';
        const period = payload.period || `${payload.start_date || ''} to ${payload.end_date || ''}`;
        csvLines.push(companyName);
        csvLines.push('PROFIT & LOSS STATEMENT');
        csvLines.push(`Period: ${period}`);
        csvLines.push('');
        csvLines.push('Account,Amount');
        if (payload.sections && Array.isArray(payload.sections)) {
          payload.sections.forEach((section: any) => {
            csvLines.push(`${escapeCSV(section.name || 'Unknown Section')},`);
            if (section.items && Array.isArray(section.items)) {
              section.items.forEach((item: any) => {
                const accountName = item.account_code ? `${item.account_code} - ${item.name}` : item.name;
                const amount = item.is_percentage ? `${item.amount}%` : formatCurrencyForCSV(item.amount);
                csvLines.push(`  ${escapeCSV(accountName)},${amount}`);
              });
            }
            const totalAmount = formatCurrencyForCSV(section.total);
            csvLines.push(`Total ${escapeCSV(section.name)},${totalAmount}`);
            csvLines.push('');
          });
        }
        if ((payload as any).financialMetrics) {
          csvLines.push('FINANCIAL METRICS,');
          csvLines.push(`Gross Profit,${formatCurrencyForCSV(payload.financialMetrics.grossProfit)}`);
          csvLines.push(`Gross Margin,${(payload.financialMetrics.grossProfitMargin || 0).toFixed(1)}%`);
          csvLines.push(`Operating Income,${formatCurrencyForCSV(payload.financialMetrics.operatingIncome)}`);
          csvLines.push(`Operating Margin,${(payload.financialMetrics.operatingMargin || 0).toFixed(1)}%`);
          csvLines.push(`Net Income,${formatCurrencyForCSV(payload.financialMetrics.netIncome)}`);
          csvLines.push(`Net Margin,${(payload.financialMetrics.netIncomeMargin || 0).toFixed(1)}%`);
        }
      }
      // Balance Sheet
      else if (reportType === 'balance-sheet') {
        const companyName = payload.company?.name || 'PT. Sistem Akuntansi';
        const asOfDate = payload.as_of_date || new Date().toISOString().split('T')[0];
        csvLines.push(companyName);
        csvLines.push('BALANCE SHEET');
        csvLines.push(`As of: ${asOfDate}`);
        csvLines.push('');
        csvLines.push('Account,Amount');
        csvLines.push('SUMMARY,');
        const totalAssets = (payload.assets && payload.assets.total_assets !== undefined) ? payload.assets.total_assets : (payload.total_assets || 0);
        const totalLiabilities = (payload.liabilities && payload.liabilities.total_liabilities !== undefined) ? payload.liabilities.total_liabilities : (payload.total_liabilities || 0);
        const totalEquity = (payload.equity && payload.equity.total_equity !== undefined) ? payload.equity.total_equity : (payload.total_equity || 0);
        csvLines.push(`Total Assets,${formatCurrencyForCSV(totalAssets)}`);
        csvLines.push(`Total Liabilities,${formatCurrencyForCSV(totalLiabilities)}`);
        csvLines.push(`Total Equity,${formatCurrencyForCSV(totalEquity)}`);
        csvLines.push(`Balanced,${payload.is_balanced ? 'Yes' : 'No'}`);
        if (payload.assets) {
          csvLines.push('');
          csvLines.push('ASSETS,');
          if (payload.assets.current_assets?.items) {
            csvLines.push('Current Assets,');
            payload.assets.current_assets.items.forEach((item: any) => {
              csvLines.push(`  ${item.account_code} - ${escapeCSV(item.account_name)},${formatCurrencyForCSV(item.amount)}`);
            });
          }
          if (payload.assets.non_current_assets?.items) {
            csvLines.push('Non-Current Assets,');
            payload.assets.non_current_assets.items.forEach((item: any) => {
              csvLines.push(`  ${item.account_code} - ${escapeCSV(item.account_name)},${formatCurrencyForCSV(item.amount)}`);
            });
          }
        }
        if (payload.liabilities?.current_liabilities?.items) {
          csvLines.push('');
          csvLines.push('LIABILITIES,');
          payload.liabilities.current_liabilities.items.forEach((item: any) => {
            csvLines.push(`  ${item.account_code} - ${escapeCSV(item.account_name)},${formatCurrencyForCSV(item.amount)}`);
          });
        }
        if (payload.equity?.items) {
          csvLines.push('');
          csvLines.push('EQUITY,');
          payload.equity.items.forEach((item: any) => {
            csvLines.push(`  ${item.account_code} - ${escapeCSV(item.account_name)},${formatCurrencyForCSV(item.amount)}`);
          });
        }
      }
      // Trial Balance
      else if (reportType === 'trial-balance') {
        const companyName = payload.company?.name || 'PT. Sistem Akuntansi';
        const reportDate = payload.report_date || new Date().toISOString().split('T')[0];
        csvLines.push(companyName);
        csvLines.push('TRIAL BALANCE');
        csvLines.push(`As of: ${reportDate}`);
        csvLines.push('');
        csvLines.push('Account Code,Account Name,Account Type,Debit Balance,Credit Balance');
        if (payload.accounts && Array.isArray(payload.accounts)) {
          payload.accounts.forEach((account: any) => {
            csvLines.push([
              escapeCSV(account.account_code || ''),
              escapeCSV(account.account_name || account.name || ''),
              escapeCSV(account.account_type || ''),
              formatCurrencyForCSV(account.debit_balance),
              formatCurrencyForCSV(account.credit_balance)
            ].join(','));
          });
          csvLines.push('');
          csvLines.push('TOTALS,,,' + formatCurrencyForCSV(payload.total_debits) + ',' + formatCurrencyForCSV(payload.total_credits));
          csvLines.push(`Balanced: ${payload.is_balanced ? 'Yes' : 'No'}`);
        }
      }
      // Generic fallback
      else {
        csvLines.push('FINANCIAL REPORT');
        csvLines.push(`Report Type: ${reportType}`);
        csvLines.push('');
        csvLines.push('Property,Value');
        if (typeof payload === 'object') {
          Object.entries(payload).forEach(([key, value]) => {
            let displayValue = '';
            if (typeof value === 'object' && value !== null) {
              displayValue = JSON.stringify(value);
            } else {
              displayValue = String(value || '');
            }
            csvLines.push(`${escapeCSV(key)},${escapeCSV(displayValue)}`);
          });
        }
      }

      // Footer
      csvLines.push('');
      csvLines.push(`Generated on: ${new Date().toLocaleString('id-ID')}`);
      return csvLines.join('\n');
    } catch (error) {
      console.error('Error converting to CSV:', error);
      return `Error converting data to CSV format: ${error instanceof Error ? error.message : 'Unknown error'}`;
    }
  };

  // Format currency for display
  const formatCurrency = (amount: number | null | undefined) => {
    if (amount === null || amount === undefined || isNaN(Number(amount))) {
      return 'Rp 0';
    }

    const numericAmount = Number(amount);
    if (!isFinite(numericAmount)) {
      return 'Rp 0';
    }

    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0
    }).format(numericAmount);
  };

  // Get badge color for entry type
  const getEntryTypeBadgeColor = (entryType: string) => {
    switch (entryType?.toUpperCase()) {
      case 'SALE':
        return 'green';
      case 'PURCHASE':
        return 'orange';
      case 'PAYMENT':
        return 'blue';
      case 'CASH_BANK':
        return 'teal';
      case 'JOURNAL':
        return 'purple';
      default:
        return 'gray';
    }
  };

  const handleGenerateReport = (report: any) => {
    setSelectedReport(report);
    resetParams();

    // Set default parameters based on report type
    if (report.id === 'balance-sheet') {
      setReportParams({ as_of_date: new Date().toISOString().split('T')[0], format: 'pdf' });
    } else if (report.id === 'profit-loss' || report.id === 'cash-flow') {
      const today = new Date();
      const firstDayOfMonth = new Date(today.getFullYear(), today.getMonth(), 1);
      setReportParams({
        start_date: firstDayOfMonth.toISOString().split('T')[0],
        end_date: today.toISOString().split('T')[0],
        format: 'pdf'
      });
    } else if (report.id === 'sales-summary' || report.id === 'purchase-summary' || report.id === 'purchase-report') {
      const today = new Date();
      const thirtyDaysAgo = new Date(today);
      thirtyDaysAgo.setDate(today.getDate() - 30);
      setReportParams({
        start_date: thirtyDaysAgo.toISOString().split('T')[0],
        end_date: today.toISOString().split('T')[0],
        group_by: 'month',
        format: 'pdf'
      });
    } else if (report.id === 'trial-balance') {
      setReportParams({ as_of_date: new Date().toISOString().split('T')[0], format: 'pdf' });
    } else if (report.id === 'general-ledger') {
      const today = new Date();
      const firstDayOfMonth = new Date(today.getFullYear(), today.getMonth(), 1);
      setReportParams({
        account_id: 'all',
        start_date: firstDayOfMonth.toISOString().split('T')[0],
        end_date: today.toISOString().split('T')[0],
        format: 'pdf'
      });
    }

    onOpen();
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value } = e.target;
    setReportParams(prev => ({ ...prev, [name]: value }));
  };

  const executeReport = async () => {
    if (!selectedReport) return;

    setLoading(true);
    try {
      // Validate required parameters
      if (['profit-loss', 'cash-flow', 'sales-summary', 'purchase-summary', 'purchase-report', 'general-ledger'].includes(selectedReport.id)) {
        if (!reportParams.start_date || !reportParams.end_date) {
          throw new Error(t('reports.messages.startDateEndDateRequired'));
        }
      }

      if (['balance-sheet', 'trial-balance'].includes(selectedReport.id)) {
        if (!reportParams.as_of_date) {
          throw new Error(t('reports.messages.asOfDateRequired'));
        }
      }

      if (selectedReport.id === 'general-ledger') {
        if (!reportParams.account_id) {
          throw new Error(t('reports.messages.accountIdRequired'));
        }
      }

      // Use unified report generation for all report types
      const result = await reportService.generateReport(selectedReport.id, reportParams);

      if (!result) {
        throw new Error(t('reports.messages.noDataReceived'));
      }

      // Check if result is a Blob (for file downloads)
      if (result instanceof Blob) {
        if (result.size === 0) {
          throw new Error(t('reports.messages.emptyFileReceived'));
        }

        // Handle the result - Download file
        const fileName = `${selectedReport.id}_report_${new Date().toISOString().split('T')[0]}.${reportParams.format}`;
        await reportService.downloadReport(result, fileName);
        toast({
          title: t('reports.messages.downloadSuccessful'),
          description: t('reports.messages.reportDownloadedSuccess', { reportName: selectedReport.name }),
          status: 'success',
          duration: 5000,
          isClosable: true,
        });
      } else {
        console.error('Unexpected result format:', typeof result, result);
        throw new Error(t('reports.messages.invalidResponseFormat'));
      }

      onClose();
    } catch (error) {
      console.error('Failed to generate report:', error);

      let errorMessage = t('reports.messages.unknownError');
      if (error instanceof Error) {
        errorMessage = error.message;
      } else if (typeof error === 'string') {
        errorMessage = error;
      }

      toast({
        title: t('reports.messages.reportGenerationFailed'),
        description: errorMessage,
        status: 'error',
        duration: 8000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <SimpleLayout allowedRoles={['admin', 'finance', 'director', 'inventory_manager']}>
      <Box p={8}>
        <VStack spacing={8} align="stretch">
          <VStack align="start" spacing={4}>
            <Flex justify="space-between" align="center" w="full">
              <Heading as="h1" size="xl" color={headingColor} fontWeight="medium">
                {t('reports.financialReports')}
              </Heading>
            </Flex>
          </VStack>

          {/* Financial Reports Grid */}
          <SimpleGrid columns={[1, 2, 3]} spacing={6} position="relative">
            {availableReports.map((report) => (
              <Card
                key={report.id}
                bg={cardBg}
                border="1px"
                borderColor={borderColor}
                borderRadius="md"
                overflow="visible"
                _hover={{ shadow: 'md' }}
                transition="all 0.2s"
                position="relative"
              >
                <CardBody p={0} position="relative">
                  <VStack spacing={0} align="stretch">
                    {/* Icon and Badge Header */}
                    <Flex p={4} align="center" justify="space-between">
                      <Icon as={report.icon} size="24px" color="blue.500" />
                      <Badge
                        colorScheme={report.type === 'FINANCIAL' ? 'green' : 'blue'}
                        variant="solid"
                        fontSize="xs"
                        px={2}
                        py={1}
                        borderRadius="md"
                      >
                        {report.type}
                      </Badge>
                    </Flex>

                    {/* Content */}
                    <VStack spacing={3} align="stretch" px={4} pb={4}>
                      <Heading size="md" color={textColor} fontWeight="medium">
                        {report.name}
                      </Heading>
                      <Text
                        fontSize="sm"
                        color={descriptionColor}
                        lineHeight="1.4"
                        noOfLines={3}
                      >
                        {report.description}
                      </Text>

                      {/* Action Buttons */}
                      <HStack spacing={2} width="full" mt={2} align="flex-start">
                        <Button
                          colorScheme="blue"
                          size="md"
                          flex="1"
                          onClick={() => {
                            // Navigate to dedicated report pages
                            router.push(`/reports/${report.id}`);
                          }}
                          isLoading={loading}
                          leftIcon={<FiEye />}
                        >
                          {t('reports.viewReport')}
                        </Button>
                        <VStack spacing={1} flex="1">
                          <Button
                            colorScheme="red"
                            variant="outline"
                            size="sm"
                            width="full"
                            isLoading={loading}
                            leftIcon={<FiFilePlus />}
                            onClick={() => handleQuickDownload(report, 'pdf')}
                          >
                            {t('reports.pdf')}
                          </Button>
                          <Button
                            colorScheme="green"
                            variant="outline"
                            size="sm"
                            width="full"
                            isLoading={loading}
                            leftIcon={<FiFileText />}
                            onClick={() => handleQuickDownload(report, 'csv')}
                          >
                            {t('reports.csvLabel')}
                          </Button>
                          <Button
                            colorScheme="gray"
                            variant="outline"
                            size="sm"
                            width="full"
                            isLoading={loading}
                            leftIcon={<FiDownload />}
                            onClick={() => handleGenerateReport(report)}
                          >
                            {t('reports.more')}
                          </Button>
                        </VStack>
                      </HStack>
                    </VStack>
                  </VStack>
                </CardBody>
              </Card>
            ))}
          </SimpleGrid>
        </VStack>
      </Box>

      {/* Report Parameters Modal */}
      <Modal isOpen={isOpen} onClose={onClose} size="md">
        <ModalOverlay />
        <ModalContent bg={modalContentBg}>
          <ModalHeader>{selectedReport?.name}</ModalHeader>
          <ModalCloseButton />
          <ModalBody>
            {selectedReport && (
              <VStack spacing={4} align="stretch">
                {/* Balance Sheet Parameters */}
                {selectedReport.id === 'balance-sheet' && (
                  <>
                    <FormControl isRequired>
                      <FormLabel>{t('reports.modal.asOfDate')}</FormLabel>
                      <Input
                        type="date"
                        name="as_of_date"
                        value={reportParams.as_of_date || ''}
                        onChange={handleInputChange}
                      />
                    </FormControl>
                  </>
                )}

                {/* Profit & Loss and Cash Flow Parameters */}
                {(selectedReport.id === 'profit-loss' || selectedReport.id === 'cash-flow') && (
                  <>
                    <FormControl isRequired>
                      <FormLabel>{t('reports.modal.startDate')}</FormLabel>
                      <Input
                        type="date"
                        name="start_date"
                        value={reportParams.start_date || ''}
                        onChange={handleInputChange}
                      />
                    </FormControl>
                    <FormControl isRequired>
                      <FormLabel>{t('reports.modal.endDate')}</FormLabel>
                      <Input
                        type="date"
                        name="end_date"
                        value={reportParams.end_date || ''}
                        onChange={handleInputChange}
                      />
                    </FormControl>
                  </>
                )}

                {/* Sales Summary Parameters */}
                {selectedReport.id === 'sales-summary' && (
                  <>
                    <FormControl isRequired>
                      <FormLabel>Start Date</FormLabel>
                      <Input
                        type="date"
                        name="start_date"
                        value={reportParams.start_date || ''}
                        onChange={handleInputChange}
                      />
                    </FormControl>
                    <FormControl isRequired>
                      <FormLabel>End Date</FormLabel>
                      <Input
                        type="date"
                        name="end_date"
                        value={reportParams.end_date || ''}
                        onChange={handleInputChange}
                      />
                    </FormControl>
                  </>
                )}

                {/* Purchase Summary and Purchase Report Parameters */}
                {(selectedReport.id === 'purchase-summary' || selectedReport.id === 'purchase-report') && (
                  <>
                    <FormControl isRequired>
                      <FormLabel>Start Date</FormLabel>
                      <Input
                        type="date"
                        name="start_date"
                        value={reportParams.start_date || ''}
                        onChange={handleInputChange}
                      />
                    </FormControl>
                    <FormControl isRequired>
                      <FormLabel>End Date</FormLabel>
                      <Input
                        type="date"
                        name="end_date"
                        value={reportParams.end_date || ''}
                        onChange={handleInputChange}
                      />
                    </FormControl>
                    <FormControl>
                      <FormLabel>Group By</FormLabel>
                      <Select
                        name="group_by"
                        value={reportParams.group_by || 'month'}
                        onChange={handleInputChange}
                      >
                        <option value="month">Month</option>
                        <option value="quarter">Quarter</option>
                        <option value="year">Year</option>
                      </Select>
                    </FormControl>
                  </>
                )}

                {/* Trial Balance Parameters */}
                {selectedReport.id === 'trial-balance' && (
                  <>
                    <FormControl isRequired>
                      <FormLabel>As of Date</FormLabel>
                      <Input
                        type="date"
                        name="as_of_date"
                        value={reportParams.as_of_date || ''}
                        onChange={handleInputChange}
                      />
                    </FormControl>
                  </>
                )}

                {/* General Ledger Parameters */}
                {selectedReport.id === 'general-ledger' && (
                  <>
                    <FormControl isRequired>
                      <FormLabel>Account Selection</FormLabel>
                      <Select
                        name="account_id"
                        value={reportParams.account_id || 'all'}
                        onChange={handleInputChange}
                      >
                        <option value="all">All Accounts</option>
                        <option value="1101">Cash Account</option>
                        <option value="1102">Bank BCA</option>
                        <option value="1104">Bank Mandiri</option>
                        <option value="1201">Accounts Receivable</option>
                        <option value="2101">Accounts Payable</option>
                        <option value="4101">Sales Revenue</option>
                        <option value="5101">Cost of Goods Sold</option>
                        <option value="6101">Administrative Expenses</option>
                      </Select>
                    </FormControl>
                    <FormControl isRequired>
                      <FormLabel>Start Date</FormLabel>
                      <Input
                        type="date"
                        name="start_date"
                        value={reportParams.start_date || ''}
                        onChange={handleInputChange}
                      />
                    </FormControl>
                    <FormControl isRequired>
                      <FormLabel>End Date</FormLabel>
                      <Input
                        type="date"
                        name="end_date"
                        value={reportParams.end_date || ''}
                        onChange={handleInputChange}
                      />
                    </FormControl>
                  </>
                )}

                {/* Format Selection - Common for all reports */}
                <FormControl>
                  <FormLabel>Output Format</FormLabel>
                  <Select
                    name="format"
                    value={reportParams.format || 'pdf'}
                    onChange={handleInputChange}
                  >
                    {/* For SSOT P&L, only allow PDF and CSV as per requirements */}
                    {selectedReport?.id === 'profit-loss' ? (
                      <>
                        <option value="pdf">PDF</option>
                        <option value="csv">CSV</option>
                      </>
                    ) : (
                      <>
                        <option value="pdf">PDF</option>
                        <option value="csv">CSV</option>
                        <option value="excel">Excel</option>
                        <option value="json">JSON</option>
                      </>
                    )}
                  </Select>
                </FormControl>
              </VStack>
            )}
          </ModalBody>
          <ModalFooter>
            <Button variant="ghost" mr={3} onClick={onClose}>
              Cancel
            </Button>
            <Button
              colorScheme="blue"
              onClick={() => {
                if (selectedReport?.id === 'sales-summary') {
                  // For sales summary, open SSOT modal with date parameters
                  setSSOTSSStartDate(reportParams.start_date || ssotSSStartDate);
                  setSSOTSSEndDate(reportParams.end_date || ssotSSEndDate);
                  setSSOTSSOpen(true);
                  // Auto-fetch the report
                  if (reportParams.start_date && reportParams.end_date) {
                    fetchSSOTSalesSummaryReport();
                  }
                  onClose();
                } else {
                  executeReport();
                }
              }}
              isLoading={loading}
              leftIcon={<FiDownload />}
            >
              Generate Report
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      {/* SSOT P&L Modal */}
      <Modal isOpen={ssotPLOpen} onClose={() => setSSOTPLOpen(false)} size="6xl">
        <ModalOverlay />
        <ModalContent bg={modalContentBg}>
          <ModalHeader>
            <HStack>
              <Icon as={FiTrendingUp} color="purple.500" />
              <VStack align="start" spacing={0}>
                <Text fontSize="lg" fontWeight="bold">
                  {t('reports.modal.profitLossTitle')}
                </Text>
                <Text fontSize="sm" color={previewPeriodTextColor}>
                  {t('reports.vendorHistoryReport.helperText')}
                </Text>
              </VStack>
            </HStack>
          </ModalHeader>
          <ModalCloseButton />
          <ModalBody pb={6}>
            {/* Date Range Controls */}
            <Box mb={4}>
              {/* Closed Period History Selector */}
              <HStack spacing={4} mb={4} alignItems="flex-start">
                <FormControl flex="1">
                  <FormLabel fontSize="sm">
                    <HStack spacing={2}>
                      <Icon as={FiCalendar} />
                      <Text>{t('reports.modal.periodClosedHistory')}</Text>
                      <Tooltip label={t('reports.modal.selectClosedPeriodTooltip')} placement="top">
                        <Box><Icon as={FiInfo} color="gray.500" /></Box>
                      </Tooltip>
                    </HStack>
                  </FormLabel>
                  <Select
                    placeholder={t('reports.modal.selectClosedPeriodOptional')}
                    size="md"
                    onChange={(e) => {
                      if (e.target.value) {
                        const selectedPeriod = ssotPLClosedPeriods.find(p => p.date === e.target.value);
                        if (selectedPeriod) {
                          // Set date range untuk periode yang dipilih
                          // Prioritas: gunakan start_date dari accounting_periods jika tersedia
                          const rawStart = selectedPeriod.start_date as string | undefined;

                          if (rawStart) {
                            const normalizedStart = String(rawStart).slice(0, 10);
                            setSSOTStartDate(normalizedStart);
                          } else {
                            // Fallback: gunakan fiscal year start (behaviour lama)
                            const periodEndDate = new Date(selectedPeriod.date);
                            const fiscalYearStart = new Date(periodEndDate.getFullYear(), 0, 1); // Default Jan 1
                            setSSOTStartDate(fiscalYearStart.toISOString().split('T')[0]);
                          }

                          // End date selalu pakai end_date / entry_date dari history
                          setSSOTEndDate(selectedPeriod.date);
                        }
                      }
                    }}
                    isDisabled={loadingSSOTPLClosedPeriods}
                  >
                    {ssotPLClosedPeriods.length > 0 ? (
                      ssotPLClosedPeriods.map((period, index) => (
                        <option key={`${period.date}-${index}`} value={period.date}>
                          {period.label}
                        </option>
                      ))
                    ) : (
                      <option disabled>{t('reports.modal.noClosedPeriodsAvailable')}</option>
                    )}
                  </Select>
                </FormControl>
                <Button
                  size="md"
                  variant="outline"
                  colorScheme="blue"
                  onClick={async () => {
                    setLoadingSSOTPLClosedPeriods(true);
                    try {
                      const response = await api.get(API_ENDPOINTS.FISCAL_CLOSING.HISTORY);
                      if (response.data.success && response.data.data && Array.isArray(response.data.data)) {
                        const periods = buildUniqueClosedPeriods(response.data.data);
                        setSSOTPLClosedPeriods(periods);
                        toast({
                          title: t('messages.toast.success'),
                          description: t('reports.modal.loadedClosedPeriods', { count: periods.length }),
                          status: 'success',
                          duration: 2000
                        });
                      }
                    } catch (error) {
                      console.error('Failed to load closed periods:', error);
                    } finally {
                      setLoadingSSOTPLClosedPeriods(false);
                    }
                  }}
                  isLoading={loadingSSOTPLClosedPeriods}
                  leftIcon={<FiClock />}
                  mt={7}
                >
                  {t('reports.modal.loadHistory')}
                </Button>
              </HStack>

              <Divider my={4} />

              <HStack spacing={4} mb={4} flexWrap="wrap">
                <FormControl>
                  <FormLabel>{t('reports.modal.startDate')}</FormLabel>
                  <Input
                    type="date"
                    value={ssotStartDate}
                    onChange={(e) => setSSOTStartDate(e.target.value)}
                  />
                </FormControl>
                <FormControl>
                  <FormLabel>{t('reports.modal.endDate')}</FormLabel>
                  <Input
                    type="date"
                    value={ssotEndDate}
                    onChange={(e) => setSSOTEndDate(e.target.value)}
                  />
                </FormControl>
                <Button
                  colorScheme="blue"
                  onClick={fetchSSOTPLReport}
                  isLoading={ssotPLLoading}
                  leftIcon={<FiTrendingUp />}
                  size="md"
                  mt={8}
                  whiteSpace="nowrap"
                >
                  {t('reports.modal.generateReport')}
                </Button>
              </HStack>
            </Box>

            {ssotPLLoading && (
              <Box textAlign="center" py={8}>
                <VStack spacing={4}>
                  <Spinner size="xl" thickness="4px" speed="0.65s" color="green.500" />
                  <VStack spacing={2}>
                    <Text fontSize="lg" fontWeight="medium" color={loadingTextColor}>
                      {t('reports.modal.generatingProfitLoss')}
                    </Text>
                    <Text fontSize="sm" color={descriptionColor}>
                      {t('reports.modal.analyzingRevenueExpenses')}
                    </Text>
                  </VStack>
                </VStack>
              </Box>
            )}

            {ssotPLError && (
              <Box bg="red.50" p={4} borderRadius="md" mb={4}>
                <Text color="red.600" fontWeight="medium">Error: {ssotPLError}</Text>
                <Button
                  mt={2}
                  size="sm"
                  colorScheme="red"
                  variant="outline"
                  onClick={fetchSSOTPLReport}
                >
                  {t('reports.modal.retry')}
                </Button>
              </Box>
            )}

            {ssotPLData && !ssotPLLoading && (
              <VStack spacing={6} align="stretch">
                {/* Company Header */}
                {ssotPLData.company && (
                  <Box bg="purple.50" p={4} borderRadius="md">
                    <HStack justify="space-between" align="start">
                      <VStack align="start" spacing={1}>
                        <Text fontSize="lg" fontWeight="bold" color="purple.800">
                          {ssotPLData.company.name || 'Company Name Not Available'}
                        </Text>
                        <Text fontSize="sm" color="purple.600">
                          {ssotPLData.company.address ? (
                            ssotPLData.company.city ? `${ssotPLData.company.address}, ${ssotPLData.company.city}` : ssotPLData.company.address
                          ) : 'Address not available'}
                        </Text>
                        {ssotPLData.company.phone && (
                          <Text fontSize="sm" color="purple.600">
                            {ssotPLData.company.phone} | {ssotPLData.company.email}
                          </Text>
                        )}
                      </VStack>
                      <VStack align="end" spacing={1}>
                        <Text fontSize="sm" color="purple.600">
                          {t('reports.modal.currency')}: {ssotPLData.currency || 'IDR'}
                        </Text>
                        <Text fontSize="xs" color="purple.500">
                          {t('reports.modal.generated')}: {ssotPLData.generated_at ? new Date(ssotPLData.generated_at).toLocaleString('id-ID') : 'N/A'}
                        </Text>
                      </VStack>
                    </HStack>
                  </Box>
                )}

                {/* Report Header */}
                <Box textAlign="center" bg={summaryBg} p={4} borderRadius="md">
                  <Heading size="md" color={headingColor}>
                    {ssotPLData.title || t('reports.modal.profitLossTitle')}
                  </Heading>
                  <Text fontSize="sm" color={descriptionColor}>
                    {t('reports.modal.period')}: {ssotPLData.period || `${ssotStartDate} - ${ssotEndDate}`}
                  </Text>
                  <Text fontSize="xs" color={descriptionColor} mt={1}>
                    {t('reports.modal.generated')}: {new Date().toLocaleDateString('id-ID')} at {new Date().toLocaleTimeString('id-ID')}
                  </Text>
                </Box>

                {/* Analysis Message */}
                {ssotPLData.message && (
                  <Box bg="blue.50" p={4} borderRadius="md" border="1px" borderColor="blue.200">
                    <Text fontSize="sm" color="blue.800">
                      <strong>{t('reports.modal.analysis')}:</strong> {ssotPLData.message}
                    </Text>
                  </Box>
                )}

                {/* Summary Statistics */}
                <SimpleGrid columns={[1, 2, 4]} spacing={4}>
                  <Box bg="purple.50" p={4} borderRadius="md" textAlign="center">
                    <Text fontSize="2xl" fontWeight="bold" color="purple.600">
                      {formatCurrency(ssotPLData.total_revenue || 0)}
                    </Text>
                    <Text fontSize="sm" color="purple.800">{t('reports.modal.totalRevenue')}</Text>
                  </Box>
                  <Box bg="blue.50" p={4} borderRadius="md" textAlign="center">
                    <Text fontSize="2xl" fontWeight="bold" color="blue.600">
                      {formatCurrency(ssotPLData.total_expenses || 0)}
                    </Text>
                    <Text fontSize="sm" color="blue.800">{t('reports.modal.totalExpenses')}</Text>
                  </Box>
                  <Box bg="orange.50" p={4} borderRadius="md" textAlign="center">
                    <Text fontSize="2xl" fontWeight="bold" color="orange.600">
                      {formatCurrency(ssotPLData.net_profit || 0)}
                    </Text>
                    <Text fontSize="sm" color="orange.800">{t('reports.modal.netProfit')}</Text>
                  </Box>
                  <Box bg="green.50" p={4} borderRadius="md" textAlign="center">
                    <Text fontSize="2xl" fontWeight="bold" color="green.600">
                      {formatCurrency(ssotPLData.net_loss || 0)}
                    </Text>
                    <Text fontSize="sm" color="green.800">{t('reports.modal.netLoss')}</Text>
                  </Box>
                </SimpleGrid>

                {/* Revenue Details */}
                {ssotPLData.revenue_details && ssotPLData.revenue_details.length > 0 && (
                  <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                    <Text fontSize="md" fontWeight="bold" color={headingColor} mb={3}>
                      {t('reports.modal.revenueDetails')}
                    </Text>
                    <SimpleGrid columns={[1, 2, 3]} spacing={4}>
                      {ssotPLData.revenue_details.map((revenue: any) => (
                        <Box key={revenue.account_id} bg="white" p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                          <VStack align="start" spacing={0}>
                            <Text fontSize="sm" color="gray.700">
                              {t('reports.modal.accountId')}: {revenue.account_id}
                            </Text>
                            <Text fontSize="sm" color="gray.700">
                              {t('reports.modal.accountName')}: {revenue.account_name || 'Unnamed Account'}
                            </Text>
                            <Text fontSize="sm" color="gray.700">
                              {t('reports.modal.amount')}: {formatCurrency(revenue.amount || 0)}
                            </Text>
                          </VStack>
                        </Box>
                      ))}
                    </SimpleGrid>
                  </Box>
                )}

                {/* Expense Details */}
                {ssotPLData.expense_details && ssotPLData.expense_details.length > 0 && (
                  <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                    <Text fontSize="md" fontWeight="bold" color={headingColor} mb={3}>
                      {t('reports.modal.expenseDetails')}
                    </Text>
                    <SimpleGrid columns={[1, 2, 3]} spacing={4}>
                      {ssotPLData.expense_details.map((expense: any) => (
                        <Box key={expense.account_id} bg="white" p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                          <VStack align="start" spacing={0}>
                            <Text fontSize="sm" color="gray.700">
                              {t('reports.modal.accountId')}: {expense.account_id}
                            </Text>
                            <Text fontSize="sm" color="gray.700">
                              {t('reports.modal.accountName')}: {expense.account_name || 'Unnamed Account'}
                            </Text>
                            <Text fontSize="sm" color="gray.700">
                              {t('reports.modal.amount')}: {formatCurrency(expense.amount || 0)}
                            </Text>
                          </VStack>
                        </Box>
                      ))}
                    </SimpleGrid>
                  </Box>
                )}

                {/* Financial Metrics Summary */}
                {ssotPLData.financialMetrics && (
                  <Box bg="green.50" p={4} borderRadius="md" border="1px" borderColor="green.200">
                    <Text fontSize="md" fontWeight="bold" color="green.800" mb={3}>
                      {t('reports.modal.keyFinancialMetrics')}
                    </Text>
                    <SimpleGrid columns={[1, 2]} spacing={3}>
                      <VStack spacing={2}>
                        <HStack justify="space-between" w="full">
                          <Text fontSize="sm" color="green.700">{t('reports.modal.grossProfit')}:</Text>
                          <Text fontSize="sm" fontWeight="semibold">
                            {formatCurrency(ssotPLData.financialMetrics.grossProfit || 0)}
                          </Text>
                        </HStack>
                        <HStack justify="space-between" w="full">
                          <Text fontSize="sm" color="green.700">{t('reports.modal.grossMargin')}:</Text>
                          <Text fontSize="sm" fontWeight="semibold">
                            {(ssotPLData.financialMetrics.grossProfitMargin || 0).toFixed(1)}%
                          </Text>
                        </HStack>
                        <HStack justify="space-between" w="full">
                          <Text fontSize="sm" color="green.700">{t('reports.modal.operatingIncome')}:</Text>
                          <Text fontSize="sm" fontWeight="semibold">
                            {formatCurrency(ssotPLData.financialMetrics.operatingIncome || 0)}
                          </Text>
                        </HStack>
                      </VStack>
                      <VStack spacing={2}>
                        <HStack justify="space-between" w="full">
                          <Text fontSize="sm" color="green.700">{t('reports.modal.operatingMargin')}:</Text>
                          <Text fontSize="sm" fontWeight="semibold">
                            {(ssotPLData.financialMetrics.operatingMargin || 0).toFixed(1)}%
                          </Text>
                        </HStack>
                        <HStack justify="space-between" w="full">
                          <Text fontSize="sm" color="green.700">{t('reports.modal.netIncome')}:</Text>
                          <Text fontSize="sm" fontWeight="semibold">
                            {formatCurrency(ssotPLData.financialMetrics.netIncome || 0)}
                          </Text>
                        </HStack>
                        <HStack justify="space-between" w="full">
                          <Text fontSize="sm" color="green.700">{t('reports.modal.netMargin')}:</Text>
                          <Text fontSize="sm" fontWeight="semibold">
                            {(ssotPLData.financialMetrics.netIncomeMargin || 0).toFixed(1)}%
                          </Text>
                        </HStack>
                      </VStack>
                    </SimpleGrid>
                  </Box>
                )}

                {/* Sections Data (if available) */}
                {ssotPLData.sections && ssotPLData.sections.length > 0 && (
                  <VStack spacing={4} align="stretch">
                    <Text fontSize="lg" fontWeight="bold" color={headingColor}>
                      {t('reports.modal.detailedBreakdown')}
                    </Text>
                    {ssotPLData.sections.map((section: any, index: number) => (
                      <Box key={index} bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                        <VStack spacing={3} align="stretch">
                          <HStack justify="space-between">
                            <Text fontSize="md" fontWeight="bold" color={headingColor}>
                              {section.name}
                            </Text>
                            <Text fontSize="md" fontWeight="bold" color={section.total >= 0 ? 'green.600' : 'red.600'}>
                              {formatCurrency(section.total || 0)}
                            </Text>
                          </HStack>

                          {section.items && section.items.length > 0 && (
                            <VStack spacing={1} align="stretch">
                              {section.items.map((item: any, itemIndex: number) => (
                                <HStack key={itemIndex} justify="space-between" pl={4}>
                                  <Text fontSize="sm" color={descriptionColor}>
                                    {item.account_code ? `${item.account_code} - ${item.name}` : item.name}
                                    {item.is_percentage && ' (%)'}
                                  </Text>
                                  <Text fontSize="sm" color={textColor}>
                                    {item.is_percentage ?
                                      `${typeof item.amount === 'number' ? item.amount.toFixed(1) : item.amount}%` :
                                      formatCurrency(item.amount || 0)
                                    }
                                  </Text>
                                </HStack>
                              ))}
                            </VStack>
                          )}
                        </VStack>
                      </Box>
                    ))}
                  </VStack>
                )}

                {/* No Data State */}
                {!ssotPLData.hasData && (
                  <Box bg="yellow.50" p={6} borderRadius="md" textAlign="center">
                    <Icon as={FiTrendingUp} boxSize={12} color="yellow.400" mb={4} />
                    <Text fontSize="lg" fontWeight="semibold" color="yellow.800" mb={2}>
                      {t('reports.modal.noDataAvailable')}
                    </Text>
                    <Text fontSize="sm" color="yellow.700">
                      {t('reports.modal.noPLDataMessage')}
                    </Text>
                    <Text fontSize="sm" color="yellow.700" mt={2}>
                      {t('reports.modal.noPLDataHint')}
                    </Text>
                  </Box>
                )}
              </VStack>
            )}
          </ModalBody>
          <ModalFooter>
            <HStack spacing={3}>
              {ssotPLData && !ssotPLLoading && (
                <>
                  <Button
                    colorScheme="red"
                    variant="outline"
                    size="sm"
                    leftIcon={<FiFilePlus />}
                    onClick={() => handleSSOTProfitLossExport('pdf')}
                  >
                    {t('reports.exportPDF')}
                  </Button>
                  <Button
                    colorScheme="green"
                    variant="outline"
                    size="sm"
                    leftIcon={<FiFileText />}
                    onClick={() => handleSSOTProfitLossExport('csv')}
                  >
                    {t('reports.exportCSV')}
                  </Button>
                </>
              )}
            </HStack>
            <Button variant="ghost" onClick={() => setSSOTPLOpen(false)}>
              {t('reports.close')}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      {/* SSOT Balance Sheet Modal */}
      <Modal
        isOpen={ssotBSOpen}
        onClose={() => setSSOTBSOpen(false)}
        size="6xl"
        onCloseComplete={() => {
          // Reset closed periods when modal closes
          setClosedPeriods([]);
        }}
      >
        <ModalOverlay />
        <ModalContent bg={modalContentBg}>
          <ModalHeader>
            <HStack>
              <Icon as={FiBarChart} color="blue.500" />
              <VStack align="start" spacing={0}>
                <Text fontSize="lg" fontWeight="bold">
                  {t('reports.balanceSheet.enhancedTitle')}
                </Text>
                <Text fontSize="sm" color={previewPeriodTextColor}>
                  {t('reports.balanceSheet.realTimeSsotIntegration')}
                </Text>
              </VStack>
            </HStack>
          </ModalHeader>
          <ModalCloseButton />
          <ModalBody pb={6}>
            <Box mb={4}>
              <HStack spacing={4} mb={4} flexWrap="wrap">
                <FormControl flex="1">
                  <FormLabel>{t('reports.modal.asOfDate')}</FormLabel>
                  <Input
                    type="date"
                    value={ssotAsOfDate}
                    onChange={(e) => setSSOTAsOfDate(e.target.value)}
                  />
                </FormControl>
                <FormControl flex="1">
                  <FormLabel display="flex" alignItems="center">
                    {t('reports.balanceSheet.closedPeriod')}
                    <Badge ml={2} colorScheme="blue" fontSize="xs">
                      {t('reports.balanceSheet.quickSelect')}
                    </Badge>
                    <Tooltip label={t('reports.balanceSheet.selectClosedPeriodTooltip')}>
                      <span>
                        <Icon as={FiInfo} ml={1} color="gray.500" boxSize={3} />
                      </span>
                    </Tooltip>
                  </FormLabel>
                  <Select
                    placeholder={t('reports.balanceSheet.selectClosedPeriodOptional')}
                    value=""
                    onChange={(e) => {
                      if (e.target.value) {
                        // Auto-fill As Of Date dengan entry_date dari closed period
                        const selectedDate = e.target.value;
                        setSSOTAsOfDate(selectedDate);

                        // Find selected period details untuk logging
                        const selectedPeriod = closedPeriods.find(p => p.date === selectedDate);

                        console.log(`[Reports] Closed period selected: ${selectedDate}`);
                        console.log(`[Reports] Auto-filled As Of Date: ${selectedDate}`);

                        if (selectedPeriod) {
                          console.log(`[Reports] Period details: ${selectedPeriod.label}`);
                        }

                        // Show success toast
                        toast({
                          title: t('reports.periodSelected'),
                          description: t('reports.generatingReportFor', {
                            date: new Date(selectedDate).toLocaleDateString('id-ID', {
                              day: '2-digit',
                              month: 'long',
                              year: 'numeric'
                            })
                          }),
                          status: 'info',
                          duration: 2000,
                          isClosable: true,
                        });

                        // Auto-generate report dengan tanggal yang dipilih
                        // Gunakan setTimeout untuk memastikan state sudah update
                        setTimeout(() => {
                          console.log(`[Reports] Auto-generating Balance Sheet for date: ${selectedDate}`);
                          // Call generate function langsung dengan parameter date yang sudah diset
                          setSSOTBSLoading(true);
                          setSSOTBSError(null);

                          ssotBalanceSheetReportService.generateSSOTBalanceSheet({
                            as_of_date: selectedDate,
                            format: 'json'
                          })
                            .then((balanceSheetData) => {
                              console.log('SSOT Balance Sheet Data received (auto-generated):', balanceSheetData);
                              setSSOTBSData(balanceSheetData);

                              toast({
                                title: t('reports.modal.status.success'),
                                description: t('reports.ssotBalanceSheetGenerated'),
                                status: 'success',
                                duration: 3000,
                                isClosable: true,
                              });
                            })
                            .catch((error) => {
                              console.error('Error auto-generating SSOT Balance Sheet:', error);
                              const errorMessage = error instanceof Error ? error.message : t('reports.modal.unknownError');
                              setSSOTBSError(errorMessage);

                              toast({
                                title: t('common.error'),
                                description: errorMessage,
                                status: 'error',
                                duration: 5000,
                                isClosable: true,
                              });
                            })
                            .finally(() => {
                              setSSOTBSLoading(false);
                            });
                        }, 100);
                      }
                    }}
                    onFocus={() => {
                      // Fetch closed periods saat dropdown diklik (lazy loading)
                      if (closedPeriods.length === 0 && !loadingClosedPeriods) {
                        console.log('[Reports] Fetching closed periods on dropdown focus...');
                        fetchClosedPeriods();
                      }
                    }}
                    isDisabled={loadingClosedPeriods}
                    icon={loadingClosedPeriods ? <Spinner size="xs" /> : undefined}
                  >
                    {loadingClosedPeriods && (
                      <option disabled>{t('reports.balanceSheet.loadingClosedPeriods')}</option>
                    )}

                    {!loadingClosedPeriods && closedPeriods.length === 0 && (
                      <option disabled>{t('reports.balanceSheet.noClosedPeriodsFound')}</option>
                    )}

                    {!loadingClosedPeriods && closedPeriods.length > 0 && (
                      <>
                        {/* Group by year */}
                        {Array.from(new Set(closedPeriods.map(p => {
                          const year = new Date(p.date).getFullYear();
                          const currentYear = new Date().getFullYear();
                          if (year === currentYear) return t('reports.balanceSheet.currentYear');
                          if (year === currentYear - 1) return t('reports.balanceSheet.lastYear');
                          return t('reports.balanceSheet.year', { year });
                        }))).map(group => (
                          <optgroup label={group} key={group}>
                            {closedPeriods
                              .filter(p => {
                                const year = new Date(p.date).getFullYear();
                                const currentYear = new Date().getFullYear();
                                if (year === currentYear) return group === t('reports.currentYear');
                                if (year === currentYear - 1) return group === t('reports.lastYear');
                                return group === t('reports.year', { year });
                              })
                              .map((period, index) => (
                                <option key={`${period.date}-${index}`} value={period.date}>
                                  {period.label}
                                </option>
                              ))}
                          </optgroup>
                        ))}
                      </>
                    )}
                  </Select>
                  <Text fontSize="xs" color="gray.500" mt={1}>
                    {t('reports.balanceSheet.selectClosedPeriodTooltip')}
                  </Text>
                </FormControl>
                <Button
                  colorScheme="blue"
                  onClick={fetchSSOTBalanceSheetReport}
                  isLoading={ssotBSLoading}
                  leftIcon={<FiBarChart />}
                  size="md"
                  mt={8}
                  whiteSpace="nowrap"
                >
                  {t('reports.modal.generateReport')}
                </Button>
              </HStack>
            </Box>

            {ssotBSLoading && (
              <Box textAlign="center" py={8}>
                <VStack spacing={4}>
                  <Spinner size="xl" thickness="4px" speed="0.65s" color="blue.500" />
                  <VStack spacing={2}>
                    <Text fontSize="lg" fontWeight="medium" color={loadingTextColor}>
                      {t('reports.generatingBalanceSheet')}
                    </Text>
                    <Text fontSize="sm" color={descriptionColor}>
                      {t('reports.modal.analyzingFinancialPosition')}
                    </Text>
                  </VStack>
                </VStack>
              </Box>
            )}

            {ssotBSError && (
              <Box bg="red.50" p={4} borderRadius="md" mb={4}>
                <Text color="red.600" fontWeight="medium">{t('common.error')}: {ssotBSError}</Text>
                <Button
                  mt={2}
                  size="sm"
                  colorScheme="red"
                  variant="outline"
                  onClick={fetchSSOTBalanceSheetReport}
                >
                  {t('reports.modal.retry')}
                </Button>
              </Box>
            )}

            {ssotBSData && !ssotBSLoading && (
              <VStack spacing={6} align="stretch">
                <Box textAlign="center" bg={summaryBg} p={4} borderRadius="md">
                  <Heading size="md" color={headingColor}>
                    {ssotBSData.company?.name || 'PT. Sistem Akuntansi'}
                  </Heading>
                  <Text fontSize="lg" fontWeight="semibold" mt={1}>
                    {t('reports.balanceSheet.enhancedTitle')}
                  </Text>
                  <Text fontSize="sm" color={descriptionColor}>
                    {t('reports.balanceSheet.asOf')}: {ssotBSData.as_of_date ? new Date(ssotBSData.as_of_date).toLocaleDateString('id-ID') : ssotAsOfDate}
                  </Text>
                  {ssotBSData.is_balanced ? (
                    <Badge colorScheme="green" mt={2}>{t('reports.balanceSheet.balancedCheck')}</Badge>
                  ) : (
                    <Badge colorScheme="red" mt={2}>{t('reports.balanceSheet.notBalancedDiff', { amount: formatCurrency(ssotBSData.balance_difference || 0) })}</Badge>
                  )}
                </Box>

                <SimpleGrid columns={[1, 2, 3]} spacing={4}>
                  <Box bg="green.50" p={4} borderRadius="md" textAlign="center">
                    <Text fontSize="sm" color="green.600">{t('reports.balanceSheet.totalAssets')}</Text>
                    <Text fontSize="xl" fontWeight="bold" color="green.700">
                      {formatCurrency(ssotBSData.assets?.total_assets || ssotBSData.total_assets || 0)}
                    </Text>
                  </Box>

                  <Box bg="orange.50" p={4} borderRadius="md" textAlign="center">
                    <Text fontSize="sm" color="orange.600">{t('reports.balanceSheet.totalLiabilities')}</Text>
                    <Text fontSize="xl" fontWeight="bold" color="orange.700">
                      {formatCurrency(ssotBSData.liabilities?.total_liabilities || ssotBSData.total_liabilities || 0)}
                    </Text>
                  </Box>

                  <Box bg="blue.50" p={4} borderRadius="md" textAlign="center">
                    <Text fontSize="sm" color="blue.600">{t('reports.balanceSheet.totalEquity')}</Text>
                    <Text fontSize="xl" fontWeight="bold" color="blue.700">
                      {formatCurrency(ssotBSData.equity?.total_equity || ssotBSData.total_equity || 0)}
                    </Text>
                  </Box>
                </SimpleGrid>

                {/* Display detailed sections if available */}
                {(ssotBSData.assets?.current_assets || ssotBSData.assets?.non_current_assets) && (
                  <VStack spacing={4} align="stretch">
                    <Text fontSize="lg" fontWeight="bold" color={headingColor}>
                      {t('reports.balanceSheet.detailedBreakdown')}
                    </Text>

                    {/* Assets Section */}
                    <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                      <VStack spacing={3} align="stretch">
                        <HStack justify="space-between">
                          <Text fontSize="md" fontWeight="bold" color={headingColor}>{t('reports.balanceSheet.assets').toUpperCase()}</Text>
                          <Text fontSize="md" fontWeight="bold" color="green.600">
                            {formatCurrency(ssotBSData.assets?.total_assets || 0)}
                          </Text>
                        </HStack>

                        {ssotBSData.assets?.current_assets?.items && (
                          <VStack spacing={1} align="stretch" pl={4}>
                            <Text fontSize="sm" fontWeight="semibold" color={descriptionColor}>{t('reports.balanceSheet.currentAssets')}</Text>
                            {ssotBSData.assets.current_assets.items.map((item: any, index: number) => (
                              <HStack key={index} justify="space-between" pl={4}>
                                <Text fontSize="sm" color={descriptionColor}>
                                  {item.account_code} - {item.account_name}
                                </Text>
                                <Text fontSize="sm" color={textColor}>
                                  {formatCurrency(item.amount || 0)}
                                </Text>
                              </HStack>
                            ))}
                          </VStack>
                        )}

                        {ssotBSData.assets?.non_current_assets?.items && (
                          <VStack spacing={1} align="stretch" pl={4}>
                            <Text fontSize="sm" fontWeight="semibold" color={descriptionColor}>{t('reports.balanceSheet.nonCurrentAssets')}</Text>
                            {ssotBSData.assets.non_current_assets.items.map((item: any, index: number) => (
                              <HStack key={index} justify="space-between" pl={4}>
                                <Text fontSize="sm" color={descriptionColor}>
                                  {item.account_code} - {item.account_name}
                                </Text>
                                <Text fontSize="sm" color={textColor}>
                                  {formatCurrency(item.amount || 0)}
                                </Text>
                              </HStack>
                            ))}
                          </VStack>
                        )}
                      </VStack>
                    </Box>

                    {/* Liabilities Section */}
                    {ssotBSData.liabilities && (
                      <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                        <VStack spacing={3} align="stretch">
                          <HStack justify="space-between">
                            <Text fontSize="md" fontWeight="bold" color={headingColor}>{t('reports.balanceSheet.liabilities').toUpperCase()}</Text>
                            <Text fontSize="md" fontWeight="bold" color="orange.600">
                              {formatCurrency(ssotBSData.liabilities?.total_liabilities || 0)}
                            </Text>
                          </HStack>

                          {ssotBSData.liabilities?.current_liabilities?.items && (
                            <VStack spacing={1} align="stretch" pl={4}>
                              <Text fontSize="sm" fontWeight="semibold" color={descriptionColor}>{t('reports.balanceSheet.currentLiabilities')}</Text>
                              {ssotBSData.liabilities.current_liabilities.items.map((item: any, index: number) => (
                                <HStack key={index} justify="space-between" pl={4}>
                                  <Text fontSize="sm" color={descriptionColor}>
                                    {item.account_code} - {item.account_name}
                                  </Text>
                                  <Text fontSize="sm" color={textColor}>
                                    {formatCurrency(item.amount || 0)}
                                  </Text>
                                </HStack>
                              ))}
                            </VStack>
                          )}
                        </VStack>
                      </Box>
                    )}

                    {/* Equity Section */}
                    {ssotBSData.equity?.items && (
                      <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                        <VStack spacing={3} align="stretch">
                          <HStack justify="space-between">
                            <Text fontSize="md" fontWeight="bold" color={headingColor}>{t('reports.balanceSheet.equity').toUpperCase()}</Text>
                            <Text fontSize="md" fontWeight="bold" color="blue.600">
                              {formatCurrency(ssotBSData.equity?.total_equity || 0)}
                            </Text>
                          </HStack>

                          <VStack spacing={1} align="stretch" pl={4}>
                            {ssotBSData.equity.items.map((item: any, index: number) => (
                              <HStack key={index} justify="space-between" pl={4}>
                                <Text fontSize="sm" color={descriptionColor}>
                                  {item.account_code} - {item.account_name}
                                </Text>
                                <Text fontSize="sm" color={textColor}>
                                  {formatCurrency(item.amount || 0)}
                                </Text>
                              </HStack>
                            ))}
                          </VStack>
                        </VStack>
                      </Box>
                    )}
                  </VStack>
                )}
              </VStack>
            )}
          </ModalBody>
          <ModalFooter>
            <HStack spacing={3}>
              {ssotBSData && !ssotBSLoading && (
                <>
                  <Button
                    colorScheme="red"
                    variant="outline"
                    size="sm"
                    leftIcon={<FiFilePlus />}
                    onClick={() => handleEnhancedPDFExport(ssotBSData)}
                  >
                    {t('reports.exportPDF')}
                  </Button>
                  <Button
                    colorScheme="green"
                    variant="outline"
                    size="sm"
                    leftIcon={<FiFileText />}
                    onClick={() => handleEnhancedCSVExport(ssotBSData)}
                  >
                    {t('reports.exportCSV')}
                  </Button>
                </>
              )}
            </HStack>
            <Button variant="ghost" onClick={() => setSSOTBSOpen(false)}>
              {t('reports.close')}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      {/* SSOT Cash Flow Modal */}
      <Modal isOpen={ssotCFOpen} onClose={() => setSSOTCFOpen(false)} size="6xl">
        <ModalOverlay />
        <ModalContent bg={modalContentBg}>
          <ModalHeader>
            <HStack>
              <Icon as={FiActivity} color="blue.500" />
              <VStack align="start" spacing={0}>
                <Text fontSize="lg" fontWeight="bold">
                  {t('reports.cashFlow.ssotTitle')}
                </Text>
                <Text fontSize="sm" color={previewPeriodTextColor}>
                  {ssotCFStartDate} - {ssotCFEndDate} | SSOT Journal Integration
                </Text>
              </VStack>
            </HStack>
          </ModalHeader>
          <ModalCloseButton />
          <ModalBody pb={6}>
            <Box mb={4}>
              {/* Closed Period History Selector */}
              <HStack spacing={4} mb={4} alignItems="flex-start">
                <FormControl flex="1">
                  <FormLabel fontSize="sm">
                    <HStack spacing={2}>
                      <Icon as={FiCalendar} />
                      <Text>{t('reports.cashFlow.periodClosedHistory')}</Text>
                      <Tooltip label={t('reports.cashFlow.tooltips.periodClosedHistory')} placement="top">
                        <Box><Icon as={FiInfo} color="gray.500" /></Box>
                      </Tooltip>
                    </HStack>
                  </FormLabel>
                  <Select
                    placeholder={t('reports.cashFlow.selectClosedPeriod')}
                    size="md"
                    onChange={(e) => {
                      if (e.target.value) {
                        const selectedPeriod = ssotCFClosedPeriods.find(p => p.date === e.target.value);
                        if (selectedPeriod) {
                          // Set date range untuk periode yang dipilih
                          // Prioritas: gunakan start_date dari accounting_periods jika tersedia
                          const rawStart = selectedPeriod.start_date as string | undefined;

                          if (rawStart) {
                            const normalizedStart = String(rawStart).slice(0, 10);
                            setSSOTCFStartDate(normalizedStart);
                          } else {
                            // Fallback: gunakan fiscal year start (behaviour lama)
                            const periodEndDate = new Date(selectedPeriod.date);
                            const fiscalYearStart = new Date(periodEndDate.getFullYear(), 0, 1); // Default Jan 1
                            setSSOTCFStartDate(fiscalYearStart.toISOString().split('T')[0]);
                          }

                          // End date selalu pakai end_date / entry_date dari history
                          setSSOTCFEndDate(selectedPeriod.date);
                        }
                      }
                    }}
                    isDisabled={loadingSSOTCFClosedPeriods}
                  >
                    {ssotCFClosedPeriods.length > 0 ? (
                      ssotCFClosedPeriods.map((period, index) => (
                        <option key={`${period.date}-${index}`} value={period.date}>
                          {period.label}
                        </option>
                      ))
                    ) : (
                      <option disabled>{t('reports.cashFlow.noClosedPeriodsAvailable')}</option>
                    )}
                  </Select>
                </FormControl>
                <Button
                  size="md"
                  variant="outline"
                  colorScheme="blue"
                  onClick={async () => {
                    setLoadingSSOTCFClosedPeriods(true);
                    try {
                      const response = await api.get(API_ENDPOINTS.FISCAL_CLOSING.HISTORY);
                      if (response.data.success && response.data.data && Array.isArray(response.data.data)) {
                        const periods = buildUniqueClosedPeriods(response.data.data);
                        setSSOTCFClosedPeriods(periods);
                        toast({
                          title: t('common.labels.loading'),
                          description: t('reports.cashFlow.loadedClosedPeriods', { count: periods.length }),
                          status: 'success',
                          duration: 2000
                        });
                      }
                    } catch (error) {
                      console.error('Failed to load closed periods:', error);
                    } finally {
                      setLoadingSSOTCFClosedPeriods(false);
                    }
                  }}
                  isLoading={loadingSSOTCFClosedPeriods}
                  leftIcon={<FiClock />}
                  mt={7}
                >
                  {t('reports.loadHistory')}
                </Button>
              </HStack>

              <Divider my={4} />

              <HStack spacing={4} mb={4} flexWrap="wrap">
                <FormControl>
                  <FormLabel>{t('reports.cashFlow.startDate')}</FormLabel>
                  <Input
                    type="date"
                    value={ssotCFStartDate}
                    onChange={(e) => setSSOTCFStartDate(e.target.value)}
                  />
                </FormControl>
                <FormControl>
                  <FormLabel>{t('reports.cashFlow.endDate')}</FormLabel>
                  <Input
                    type="date"
                    value={ssotCFEndDate}
                    onChange={(e) => setSSOTCFEndDate(e.target.value)}
                  />
                </FormControl>
                <Button
                  colorScheme="blue"
                  onClick={fetchSSOTCashFlowReport}
                  isLoading={ssotCFLoading}
                  leftIcon={<FiActivity />}
                  size="md"
                  mt={8}
                  whiteSpace="nowrap"
                >
                  {t('reports.generateReport')}
                </Button>
              </HStack>
            </Box>

            {ssotCFLoading && (
              <Box textAlign="center" py={8}>
                <VStack spacing={4}>
                  <Spinner size="xl" thickness="4px" speed="0.65s" color="blue.500" />
                  <VStack spacing={2}>
                    <Text fontSize="lg" fontWeight="medium" color={loadingTextColor}>
                      {t('reports.cashFlow.generating')}
                    </Text>
                    <Text fontSize="sm" color={descriptionColor}>
                      {t('reports.cashFlow.analyzingJournalEntries')}
                    </Text>
                  </VStack>
                </VStack>
              </Box>
            )}

            {ssotCFError && (
              <Box bg="red.50" p={4} borderRadius="md" mb={4}>
                <Text color="red.600" fontWeight="medium">{t('reports.cashFlow.error')}: {ssotCFError}</Text>
                <Button
                  mt={2}
                  size="sm"
                  colorScheme="red"
                  variant="outline"
                  onClick={fetchSSOTCashFlowReport}
                >
                  {t('reports.cashFlow.retry')}
                </Button>
              </Box>
            )}

            {ssotCFData && !ssotCFLoading && (
              <VStack spacing={6} align="stretch">
                {/* Cash Position Summary */}
                <Box bg={summaryBg} p={4} borderRadius="md" borderWidth="2px" borderColor={borderColor}>
                  <HStack justify="space-between">
                    <VStack align="start" spacing={1}>
                      <Text fontSize="lg" fontWeight="bold" color={headingColor}>
                        {t('reports.cashFlow.cashPositionSummary')}
                      </Text>
                      <Text fontSize="sm" color={descriptionColor}>
                        {ssotCFData.start_date} to {ssotCFData.end_date}
                      </Text>
                    </VStack>
                    <VStack align="end" spacing={1}>
                      <Text fontSize="sm" color={descriptionColor}>{t('reports.cashFlow.netCashFlow')}</Text>
                      <Text fontSize="xl" fontWeight="bold" color={(ssotCFData.net_cash_flow || 0) >= 0 ? 'green.600' : 'red.600'}>
                        {formatCurrency(ssotCFData.net_cash_flow || 0)}
                      </Text>
                      <Text fontSize="xs" color={descriptionColor} fontStyle="italic">
                        {t('reports.cashFlow.formula.netCashFlow')}
                      </Text>
                    </VStack>
                  </HStack>
                </Box>

                {/* Cash Balance Details */}
                <SimpleGrid columns={{ base: 1, md: 3 }} spacing={4}>
                  <Box bg="blue.50" p={4} borderRadius="md" borderWidth="1px" borderColor="blue.200">
                    <Text fontSize="sm" color="gray.600" mb={2}>{t('reports.cashFlow.beginningCash')}</Text>
                    <Text fontSize="2xl" fontWeight="bold" color="blue.600">
                      {formatCurrency(ssotCFData.cash_at_beginning || 0)}
                    </Text>
                  </Box>
                  <Box bg={(ssotCFData.net_cash_flow || 0) >= 0 ? 'green.50' : 'red.50'} p={4} borderRadius="md" borderWidth="1px" borderColor={(ssotCFData.net_cash_flow || 0) >= 0 ? 'green.200' : 'red.200'}>
                    <Text fontSize="sm" color="gray.600" mb={2}>{t('reports.cashFlow.netChange')}</Text>
                    <Text fontSize="2xl" fontWeight="bold" color={(ssotCFData.net_cash_flow || 0) >= 0 ? 'green.600' : 'red.600'}>
                      {(ssotCFData.net_cash_flow || 0) >= 0 ? '+' : ''}{formatCurrency(ssotCFData.net_cash_flow || 0)}
                    </Text>
                  </Box>
                  <Box bg="purple.50" p={4} borderRadius="md" borderWidth="1px" borderColor="purple.200">
                    <Text fontSize="sm" color="gray.600" mb={2}>{t('reports.cashFlow.endingCash')}</Text>
                    <Text fontSize="2xl" fontWeight="bold" color="purple.600">
                      {formatCurrency(ssotCFData.cash_at_end || 0)}
                    </Text>
                  </Box>
                </SimpleGrid>

                {/* Operating Activities */}
                {ssotCFData.operating_activities && (
                  <Box bg={cardBg} p={4} borderRadius="md" borderWidth="1px" borderColor={borderColor}>
                    <Text fontSize="lg" fontWeight="bold" color="green.600" mb={3}>
                      💼 {t('reports.cashFlow.operatingActivities')}
                    </Text>
                    <VStack align="stretch" spacing={2}>
                      <HStack justify="space-between" bg="gray.50" p={2} borderRadius="md">
                        <Tooltip
                          label={t('reports.cashFlow.tooltips.netIncome')}
                          fontSize="sm"
                          placement="top"
                          hasArrow
                        >
                          <Text fontSize="sm" fontWeight="medium" cursor="help" borderBottom="1px dotted" borderColor="gray.400">
                            {t('reports.cashFlow.netIncome')} ℹ️
                          </Text>
                        </Tooltip>
                        <Text fontSize="sm" fontWeight="bold" color={(ssotCFData.operating_activities?.net_income || 0) >= 0 ? 'green.600' : 'red.600'}>
                          {formatCurrency(ssotCFData.operating_activities?.net_income || 0)}
                        </Text>
                      </HStack>
                      {(ssotCFData.operating_activities?.adjustments?.total_adjustments || 0) !== 0 && (
                        <Box mt={2}>
                          <Tooltip
                            label={t('reports.cashFlow.tooltips.adjustments')}
                            fontSize="sm"
                            placement="top"
                            hasArrow
                          >
                            <Text fontSize="sm" fontWeight="medium" color="gray.600" mb={2} cursor="help" borderBottom="1px dotted" borderColor="gray.400" display="inline-block">
                              {t('reports.cashFlow.adjustments')} ℹ️:
                            </Text>
                          </Tooltip>
                          <HStack justify="space-between" pl={4} py={1} bg="orange.50" borderRadius="md">
                            <Text fontSize="sm" color="gray.700">{t('reports.cashFlow.totalAdjustments')}</Text>
                            <Text fontSize="sm" fontWeight="medium" color={(ssotCFData.operating_activities?.adjustments?.total_adjustments || 0) >= 0 ? 'green.600' : 'red.600'}>
                              {formatCurrency(ssotCFData.operating_activities?.adjustments?.total_adjustments || 0)}
                            </Text>
                          </HStack>
                        </Box>
                      )}
                      {ssotCFData.operating_activities?.working_capital_changes?.items?.length > 0 && (
                        <Box mt={2}>
                          <Tooltip
                            label={t('reports.cashFlow.tooltips.workingCapitalChanges')}
                            fontSize="sm"
                            placement="top"
                            hasArrow
                          >
                            <Text fontSize="sm" fontWeight="medium" color="gray.600" mb={2} cursor="help" borderBottom="1px dotted" borderColor="gray.400" display="inline-block">
                              {t('reports.cashFlow.workingCapitalChanges')} ℹ️:
                            </Text>
                          </Tooltip>
                          {ssotCFData.operating_activities.working_capital_changes.items.map((item: any, idx: number) => (
                            <HStack key={idx} justify="space-between" pl={4} py={1}>
                              <Text fontSize="sm" color="gray.700">{item.account_name} ({item.account_code})</Text>
                              <Text fontSize="sm" fontWeight="medium" color={(item.amount || 0) >= 0 ? 'green.600' : 'red.600'}>
                                {formatCurrency(item.amount || 0)}
                              </Text>
                            </HStack>
                          ))}
                        </Box>
                      )}
                      <Divider />
                      <HStack justify="space-between" bg="green.50" p={2} borderRadius="md">
                        <VStack align="start" spacing={0}>
                          <Tooltip
                            label={t('reports.cashFlow.tooltips.totalOperatingCashFlow')}
                            fontSize="sm"
                            placement="top"
                            hasArrow
                          >
                            <Text fontWeight="bold" color="green.700" cursor="help" borderBottom="1px dotted" borderColor="green.600">
                              {t('reports.cashFlow.totalOperatingCashFlow')} ℹ️
                            </Text>
                          </Tooltip>
                          <Text fontSize="xs" color="green.600" fontStyle="italic">
                            {t('reports.cashFlow.formula.operatingCashFlow')}
                          </Text>
                        </VStack>
                        <VStack align="end" spacing={0}>
                          <Text fontWeight="bold" color="green.700">
                            {formatCurrency(ssotCFData.operating_activities?.total_operating_cash_flow || 0)}
                          </Text>
                          <Text fontSize="xs" color="green.600">
                            ({formatCurrency(ssotCFData.operating_activities?.net_income || 0)} + {formatCurrency(
                              ssotCFData.operating_activities?.adjustments?.total_adjustments || 0
                            )} + {formatCurrency(
                              ssotCFData.operating_activities?.working_capital_changes?.total_working_capital_changes || 0
                            )})
                          </Text>
                        </VStack>
                      </HStack>
                    </VStack>
                  </Box>
                )}

                {/* Investing Activities */}
                {ssotCFData.investing_activities?.items?.length > 0 && (
                  <Box bg={cardBg} p={4} borderRadius="md" borderWidth="1px" borderColor={borderColor}>
                    <Text fontSize="lg" fontWeight="bold" color="blue.600" mb={3}>
                      📊 {t('reports.cashFlow.investingActivities')}
                    </Text>
                    <VStack align="stretch" spacing={2}>
                      {ssotCFData.investing_activities.items.map((item: any, idx: number) => (
                        <HStack key={idx} justify="space-between">
                          <Text fontSize="sm" color="gray.700">{item.account_name} ({item.account_code})</Text>
                          <Text fontSize="sm" fontWeight="medium" color={(item.amount || 0) >= 0 ? 'green.600' : 'red.600'}>
                            {formatCurrency(item.amount || 0)}
                          </Text>
                        </HStack>
                      ))}
                      <Divider />
                      <HStack justify="space-between" bg="blue.50" p={2} borderRadius="md">
                        <VStack align="start" spacing={0}>
                          <Text fontWeight="bold" color="blue.700">{t('reports.cashFlow.totalInvestingCashFlow')}</Text>
                          <Text fontSize="xs" color="blue.600" fontStyle="italic">
                            {t('reports.cashFlow.formula.investingCashFlow')}
                          </Text>
                        </VStack>
                        <Text fontWeight="bold" color="blue.700">
                          {formatCurrency(ssotCFData.investing_activities?.total_investing_cash_flow || 0)}
                        </Text>
                      </HStack>
                    </VStack>
                  </Box>
                )}

                {/* Financing Activities */}
                {ssotCFData.financing_activities?.items?.length > 0 && (
                  <Box bg={cardBg} p={4} borderRadius="md" borderWidth="1px" borderColor={borderColor}>
                    <Text fontSize="lg" fontWeight="bold" color="purple.600" mb={3}>
                      🏦 {t('reports.cashFlow.financingActivities')}
                    </Text>
                    <VStack align="stretch" spacing={2}>
                      {ssotCFData.financing_activities.items.map((item: any, idx: number) => (
                        <HStack key={idx} justify="space-between">
                          <Text fontSize="sm" color="gray.700">{item.account_name} ({item.account_code})</Text>
                          <Text fontSize="sm" fontWeight="medium" color={(item.amount || 0) >= 0 ? 'green.600' : 'red.600'}>
                            {formatCurrency(item.amount || 0)}
                          </Text>
                        </HStack>
                      ))}
                      <Divider />
                      <HStack justify="space-between" bg="purple.50" p={2} borderRadius="md">
                        <VStack align="start" spacing={0}>
                          <Text fontWeight="bold" color="purple.700">{t('reports.cashFlow.totalFinancingCashFlow')}</Text>
                          <Text fontSize="xs" color="purple.600" fontStyle="italic">
                            {t('reports.cashFlow.formula.financingCashFlow')}
                          </Text>
                        </VStack>
                        <Text fontWeight="bold" color="purple.700">
                          {formatCurrency(ssotCFData.financing_activities?.total_financing_cash_flow || 0)}
                        </Text>
                      </HStack>
                    </VStack>
                  </Box>
                )}

                {/* Account Transaction History */}
                {ssotCFData.account_details && ssotCFData.account_details.length > 0 && (
                  <Box bg={cardBg} p={4} borderRadius="md" borderWidth="1px" borderColor={borderColor}>
                    <Text fontSize="lg" fontWeight="bold" color={headingColor} mb={3}>
                      📋 Transaction History by Account
                    </Text>
                    <TableContainer>
                      <Table size="sm" variant="simple">
                        <Thead>
                          <Tr bg="gray.50">
                            <Th>Account Code</Th>
                            <Th>Account Name</Th>
                            <Th isNumeric>Debit</Th>
                            <Th isNumeric>Credit</Th>
                            <Th isNumeric>Net Change</Th>
                          </Tr>
                        </Thead>
                        <Tbody>
                          {ssotCFData.account_details
                            .filter((acc: any) => acc?.account_code?.startsWith('1101') || acc?.account_code?.startsWith('1102'))
                            .map((account: any, idx: number) => (
                              <Tr key={idx} _hover={{ bg: 'gray.50' }}>
                                <Td fontFamily="mono" fontWeight="medium">{account.account_code || 'N/A'}</Td>
                                <Td>{account.account_name || 'Unknown Account'}</Td>
                                <Td isNumeric color="green.600">{formatCurrency(account.debit_total || 0)}</Td>
                                <Td isNumeric color="red.600">{formatCurrency(account.credit_total || 0)}</Td>
                                <Td isNumeric fontWeight="bold" color={(account.net_balance || 0) >= 0 ? 'green.600' : 'red.600'}>
                                  {formatCurrency(account.net_balance || 0)}
                                </Td>
                              </Tr>
                            ))}
                        </Tbody>
                      </Table>
                    </TableContainer>
                  </Box>
                )}
              </VStack>
            )}
          </ModalBody>
          <ModalFooter>
            <HStack spacing={3}>
              {ssotCFData && !ssotCFLoading && (
                <>
                  <Button
                    colorScheme="red"
                    variant="outline"
                    size="sm"
                    leftIcon={<FiFilePlus />}
                    onClick={() => handleCashFlowPDFExport()}
                    isLoading={loading}
                  >
                    {t('reports.exportPDF')}
                  </Button>
                  <Button
                    colorScheme="green"
                    variant="outline"
                    size="sm"
                    leftIcon={<FiFileText />}
                    onClick={() => handleCashFlowCSVExport()}
                    isLoading={loading}
                  >
                    {t('reports.exportCSV')}
                  </Button>
                </>
              )}
            </HStack>
            <Button variant="ghost" onClick={() => setSSOTCFOpen(false)}>
              {t('reports.close')}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      {/* SSOT Sales Summary Modal - Using enhanced component */}
      <SalesSummaryModal
        isOpen={ssotSSOpen}
        onClose={() => setSSOTSSOpen(false)}
        data={ssotSSData}
        isLoading={ssotSSLoading}
        error={ssotSSError}
        startDate={ssotSSStartDate}
        endDate={ssotSSEndDate}
        onDateChange={(newStartDate, newEndDate) => {
          setSSOTSSStartDate(newStartDate);
          setSSOTSSEndDate(newEndDate);
        }}
        onFetch={fetchSSOTSalesSummaryReport}
        onExport={async (format) => {
          try {
            toast({
              title: 'Export ' + format.toUpperCase(),
              description: `Exporting Sales Summary Report as ${format.toUpperCase()}...`,
              status: 'info',
              duration: 2000,
              isClosable: true,
            });

            if (format === 'pdf' || format === 'excel') {
              // Try to use the report service for professional export
              try {
                const result = await reportService.generateReport('sales-summary', {
                  start_date: ssotSSStartDate,
                  end_date: ssotSSEndDate,
                  format: format === 'excel' ? 'csv' : 'pdf'
                });

                if (result instanceof Blob) {
                  const fileName = `sales-summary-${ssotSSStartDate}-to-${ssotSSEndDate}.${format === 'excel' ? 'csv' : 'pdf'}`;
                  await reportService.downloadReport(result, fileName);

                  toast({
                    title: 'Export Successful',
                    description: `Sales Summary Report exported as ${format.toUpperCase()}`,
                    status: 'success',
                    duration: 3000,
                    isClosable: true,
                  });
                  return;
                }
              } catch (exportError) {
                console.warn('Professional export failed, falling back to JSON:', exportError);
              }
            }

            // Fallback: export as JSON/CSV
            if (ssotSSData) {
              let content: string;
              let mimeType: string;
              let extension: string;

              if (format === 'excel') {
                // Generate CSV content
                const customers = ssotSSData.sales_by_customer || [];
                const csvHeaders = 'Customer Name,Total Sales,Transaction Count,Average Transaction\n';
                const csvRows = customers.map(customer =>
                  `"${customer.customer_name || 'Unnamed Customer'}",` +
                  `${customer.total_sales || 0},` +
                  `${customer.transaction_count || 0},` +
                  `${customer.average_transaction || (customer.total_sales / (customer.transaction_count || 1))}`
                ).join('\n');
                content = csvHeaders + csvRows;
                mimeType = 'text/csv';
                extension = 'csv';
              } else {
                // Generate JSON content
                const reportData = {
                  reportType: 'Sales Summary Report',
                  period: `${ssotSSStartDate} to ${ssotSSEndDate}`,
                  generatedOn: new Date().toISOString(),
                  totalRevenue: ssotSSData.total_revenue || ssotSSData.total_sales || 0,
                  customers: ssotSSData.sales_by_customer || [],
                  products: ssotSSData.sales_by_product || [],
                  company: ssotSSData.company || {}
                };
                content = JSON.stringify(reportData, null, 2);
                mimeType = 'application/json';
                extension = 'json';
              }

              const dataBlob = new Blob([content], { type: mimeType });
              const url = URL.createObjectURL(dataBlob);
              const link = document.createElement('a');
              link.href = url;
              link.download = `sales-summary-${ssotSSStartDate}-to-${ssotSSEndDate}.${extension}`;
              link.click();
              URL.revokeObjectURL(url);

              toast({
                title: 'Export Successful',
                description: `Sales Summary Report exported as ${extension.toUpperCase()}`,
                status: 'success',
                duration: 3000,
                isClosable: true,
              });
            }
          } catch (error) {
            console.error('Export failed:', error);
            toast({
              title: t('reports.messages.exportFailed'),
              description: error instanceof Error ? error.message : t('reports.messages.exportReportError'),
              status: 'error',
              duration: 5000,
              isClosable: true,
            });
          }
        }}
      />

      {/* SSOT Purchase Report Modal - Using enhanced component */}
      <PurchaseReportModal
        isOpen={ssotPROpen}
        onClose={() => setSSOTPROpen(false)}
        data={ssotPRData}
        isLoading={ssotPRLoading}
        error={ssotPRError}
        startDate={ssotPRStartDate}
        endDate={ssotPREndDate}
        onDateChange={(newStartDate, newEndDate) => {
          setSSOTPRStartDate(newStartDate);
          setSSOTPREndDate(newEndDate);
        }}
        onFetch={fetchSSOTPurchaseReport}
        onExport={async (format) => {
          try {
            toast({
              title: 'Export ' + format.toUpperCase(),
              description: `Exporting Purchase Report as ${format.toUpperCase()}...`,
              status: 'info',
              duration: 2000,
              isClosable: true,
            });

            // Try to use the report service for professional export
            try {
              const result = await reportService.generateReport('purchase-report', {
                start_date: ssotPRStartDate,
                end_date: ssotPREndDate,
                format: format === 'excel' ? 'csv' : 'pdf'
              });

              if (result instanceof Blob) {
                const fileName = `purchase-report-${ssotPRStartDate}-to-${ssotPREndDate}.${format === 'excel' ? 'csv' : 'pdf'}`;
                await reportService.downloadReport(result, fileName);

                toast({
                  title: 'Export Successful',
                  description: `Purchase Report exported as ${format.toUpperCase()}`,
                  status: 'success',
                  duration: 3000,
                  isClosable: true,
                });
                return;
              }
            } catch (exportError) {
              console.warn('Professional export failed, falling back to JSON:', exportError);
            }

            // Fallback: export as JSON/CSV
            if (ssotPRData) {
              let content: string;
              let mimeType: string;
              let extension: string;

              if (format === 'excel') {
                // Generate CSV content
                const vendors = ssotPRData.purchases_by_vendor || [];
                const csvHeaders = 'Vendor Name,Total Amount,Total Purchases,Outstanding\n';
                const csvRows = vendors.map((vendor: any) =>
                  `"${vendor.vendor_name || 'Unnamed Vendor'}",` +
                  `${vendor.total_amount || 0},` +
                  `${vendor.total_purchases || 0},` +
                  `${vendor.outstanding || 0}`
                ).join('\n');
                content = csvHeaders + csvRows;
                mimeType = 'text/csv';
                extension = 'csv';
              } else {
                // Generate JSON content
                const reportData = {
                  reportType: 'Purchase Report',
                  period: `${ssotPRStartDate} to ${ssotPREndDate}`,
                  generatedOn: new Date().toISOString(),
                  totalAmount: ssotPRData.total_amount || 0,
                  totalPurchases: ssotPRData.total_purchases || 0,
                  vendors: ssotPRData.purchases_by_vendor || [],
                  company: ssotPRData.company || {}
                };
                content = JSON.stringify(reportData, null, 2);
                mimeType = 'application/json';
                extension = 'json';
              }

              const dataBlob = new Blob([content], { type: mimeType });
              const url = URL.createObjectURL(dataBlob);
              const link = document.createElement('a');
              link.href = url;
              link.download = `purchase-report-${ssotPRStartDate}-to-${ssotPREndDate}.${extension}`;
              link.click();
              URL.revokeObjectURL(url);

              toast({
                title: 'Export Successful',
                description: `Purchase Report exported as ${extension.toUpperCase()}`,
                status: 'success',
                duration: 3000,
                isClosable: true,
              });
            }
          } catch (error) {
            console.error('Export failed:', error);
            toast({
              title: t('reports.messages.exportFailed'),
              description: error instanceof Error ? error.message : t('reports.messages.exportReportError'),
              status: 'error',
              duration: 5000,
              isClosable: true,
            });
          }
        }}
      />

      {/* SSOT Trial Balance Modal */}
      <Modal isOpen={ssotTBOpen} onClose={() => setSSOTTBOpen(false)} size="6xl">
        <ModalOverlay />
        <ModalContent bg={modalContentBg}>
          <ModalHeader>
            <HStack>
              <Icon as={FiBook} color="blue.500" />
              <VStack align="start" spacing={0}>
                <Text fontSize="lg" fontWeight="bold">
                  {t('reports.trialBalance.title')}
                </Text>
                <Text fontSize="sm" color={previewPeriodTextColor}>
                  As of {ssotTBAsOfDate} | SSOT Journal Integration
                </Text>
              </VStack>
            </HStack>
          </ModalHeader>
          <ModalCloseButton />
          <ModalBody pb={6}>
            <Box mb={4}>
              {/* Closed Period History Selector */}
              <HStack spacing={4} mb={4} alignItems="flex-start">
                <FormControl flex="1">
                  <FormLabel fontSize="sm">
                    <HStack spacing={2}>
                      <Icon as={FiCalendar} />
                      <Text>{t('reports.closedPeriod')}</Text>
                      <Tooltip label="Select a closed period to view historical Trial Balance" placement="top">
                        <Box><Icon as={FiInfo} color="gray.500" /></Box>
                      </Tooltip>
                    </HStack>
                  </FormLabel>
                  <Select
                    placeholder={t('reports.selectClosedPeriod')}
                    size="md"
                    onChange={(e) => {
                      if (e.target.value) {
                        const selectedPeriod = ssotTBClosedPeriods.find(p => p.date === e.target.value);
                        if (selectedPeriod) {
                          // Set as-of date ke periode yang dipilih
                          setSSOTTBAsOfDate(selectedPeriod.date);
                        }
                      }
                    }}
                    isDisabled={loadingSSOTTBClosedPeriods}
                  >
                    {ssotTBClosedPeriods.length > 0 ? (
                      ssotTBClosedPeriods.map((period, index) => (
                        <option key={`${period.date}-${index}`} value={period.date}>
                          {period.label}
                        </option>
                      ))
                    ) : (
                      <option disabled>{t('reports.noClosedPeriodsAvailable')}</option>
                    )}
                  </Select>
                </FormControl>
                <Button
                  size="md"
                  variant="outline"
                  colorScheme="blue"
                  onClick={async () => {
                    setLoadingSSOTTBClosedPeriods(true);
                    try {
                      const response = await api.get(API_ENDPOINTS.FISCAL_CLOSING.HISTORY);
                      if (response.data.success && response.data.data && Array.isArray(response.data.data)) {
                        const periods = buildUniqueClosedPeriods(response.data.data);
                        setSSOTTBClosedPeriods(periods);
                        toast({
                          title: t('reports.modal.status.success'),
                          description: t('reports.loadedClosedPeriods', { count: periods.length }),
                          status: 'success',
                          duration: 2000
                        });
                      }
                    } catch (error) {
                      console.error('Failed to load closed periods:', error);
                    } finally {
                      setLoadingSSOTTBClosedPeriods(false);
                    }
                  }}
                  isLoading={loadingSSOTTBClosedPeriods}
                  leftIcon={<FiClock />}
                  mt={7}
                >
                  {t('reports.loadHistory')}
                </Button>
              </HStack>

              <Divider my={4} />

              <HStack spacing={4} mb={4} flexWrap="wrap">
                <FormControl>
                  <FormLabel>{t('reports.asOf')}</FormLabel>
                  <Input
                    type="date"
                    value={ssotTBAsOfDate}
                    onChange={(e) => setSSOTTBAsOfDate(e.target.value)}
                  />
                </FormControl>
                <Button
                  colorScheme="blue"
                  onClick={fetchSSOTTrialBalanceReport}
                  isLoading={ssotTBLoading}
                  leftIcon={<FiBook />}
                  size="md"
                  mt={8}
                  whiteSpace="nowrap"
                >
                  {t('reports.generateReport')}
                </Button>
              </HStack>
            </Box>

            {ssotTBLoading && (
              <Box textAlign="center" py={8}>
                <VStack spacing={4}>
                  <Spinner size="xl" thickness="4px" speed="0.65s" color="blue.500" />
                  <VStack spacing={2}>
                    <Text fontSize="lg" fontWeight="medium" color={loadingTextColor}>
                      Generating Trial Balance
                    </Text>
                    <Text fontSize="sm" color={descriptionColor}>
                      Analyzing journal entries from SSOT journal system...
                    </Text>
                  </VStack>
                </VStack>
              </Box>
            )}

            {ssotTBError && (
              <Box bg="red.50" p={4} borderRadius="md" mb={4}>
                <Text color="red.600" fontWeight="medium">Error: {ssotTBError}</Text>
                <Button
                  mt={2}
                  size="sm"
                  colorScheme="red"
                  variant="outline"
                  onClick={fetchSSOTTrialBalanceReport}
                >
                  Retry
                </Button>
              </Box>
            )}

            {ssotTBData && !ssotTBLoading && (
              <VStack spacing={6} align="stretch">
                {/* Company Header */}
                {ssotTBData.company && (
                  <Box bg="blue.50" p={4} borderRadius="md">
                    <HStack justify="space-between" align="start">
                      <VStack align="start" spacing={1}>
                        <Text fontSize="lg" fontWeight="bold" color="blue.800">
                          {ssotTBData.company.name || 'Company Name Not Available'}
                        </Text>
                        <Text fontSize="sm" color="blue.600">
                          {ssotTBData.company.address ? (
                            ssotTBData.company.city ? `${ssotTBData.company.address}, ${ssotTBData.company.city}` : ssotTBData.company.address
                          ) : 'Address not available'}
                        </Text>
                        {ssotTBData.company.phone && (
                          <Text fontSize="sm" color="blue.600">
                            {ssotTBData.company.phone} | {ssotTBData.company.email}
                          </Text>
                        )}
                      </VStack>
                      <VStack align="end" spacing={1}>
                        <Text fontSize="sm" color="blue.600">
                          {t('reports.currency')}: {ssotTBData.currency || 'IDR'}
                        </Text>
                        <Text fontSize="xs" color="blue.500">
                          {t('reports.trialBalance.generatedOn')} {ssotTBData.generated_at ? new Date(ssotTBData.generated_at).toLocaleString('id-ID') : 'N/A'}
                        </Text>
                      </VStack>
                    </HStack>
                  </Box>
                )}

                {/* Report Header */}
                <Box textAlign="center" bg={summaryBg} p={4} borderRadius="md">
                  <Heading size="md" color={headingColor}>
                    {t('reports.trialBalance.title')}
                  </Heading>
                  <Text fontSize="sm" color={descriptionColor}>
                    {t('reports.asOf')}: {ssotTBAsOfDate}
                  </Text>
                  <Text fontSize="xs" color={descriptionColor} mt={1}>
                    {t('reports.trialBalance.generatedOn')} {new Date().toLocaleDateString('id-ID')} at {new Date().toLocaleTimeString('id-ID')}
                  </Text>
                </Box>

                {/* Summary Statistics */}
                <SimpleGrid columns={[1, 2, 3]} spacing={4}>
                  <Box bg="blue.50" p={4} borderRadius="md" textAlign="center">
                    <Text fontSize="2xl" fontWeight="bold" color="blue.600">
                      {ssotTBData.accounts?.length || 0}
                    </Text>
                    <Text fontSize="sm" color="blue.800">{t('reports.trialBalance.totalAccounts')}</Text>
                  </Box>
                  <Box bg="orange.50" p={4} borderRadius="md" textAlign="center">
                    <Text fontSize="2xl" fontWeight="bold" color="orange.600">
                      {formatCurrency(ssotTBData.total_debits || 0)}
                    </Text>
                    <Text fontSize="sm" color="orange.800">{t('reports.trialBalance.totalDebits')}</Text>
                  </Box>
                  <Box bg="green.50" p={4} borderRadius="md" textAlign="center">
                    <Text fontSize="2xl" fontWeight="bold" color="green.600">
                      {formatCurrency(ssotTBData.total_credits || 0)}
                    </Text>
                    <Text fontSize="sm" color="green.800">{t('reports.trialBalance.totalCredits')}</Text>
                  </Box>
                </SimpleGrid>

                {/* Balance Status */}
                <Box bg={ssotTBData.is_balanced ? 'green.50' : 'red.50'} p={4} borderRadius="md" border="2px" borderColor={ssotTBData.is_balanced ? 'green.500' : 'red.500'}>
                  <HStack justify="space-between">
                    <VStack align="start" spacing={0}>
                      <Text fontSize="lg" fontWeight="bold" color={ssotTBData.is_balanced ? 'green.700' : 'red.700'}>
                        {ssotTBData.is_balanced ? `✓ ${t('reports.trialBalance.balanced')}` : `⚠ ${t('reports.trialBalance.notBalanced')}`}
                      </Text>
                      <Text fontSize="sm" color={ssotTBData.is_balanced ? 'green.600' : 'red.600'}>
                        {ssotTBData.is_balanced ? t('reports.trialBalance.balancedDesc') : `${t('reports.trialBalance.difference')}: ${formatCurrency(ssotTBData.difference || 0)}`}
                      </Text>
                    </VStack>
                    {!ssotTBData.is_balanced && (
                      <Badge colorScheme="red" fontSize="md" p={2}>
                        {t('reports.trialBalance.investigationRequired')}
                      </Badge>
                    )}
                  </HStack>
                </Box>

                {/* Account Details Table */}
                <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                  <Text fontSize="md" fontWeight="bold" color={headingColor} mb={3}>
                    {t('reports.accountDetails')}
                  </Text>
                  <TableContainer>
                    <Table size="sm" variant="striped">
                      <Thead bg="gray.100">
                        <Tr>
                          <Th>{t('reports.trialBalance.accountCode')}</Th>
                          <Th>{t('reports.trialBalance.accountName')}</Th>
                          <Th>{t('reports.trialBalance.accountType')}</Th>
                          <Th isNumeric>{t('reports.trialBalance.debit')}</Th>
                          <Th isNumeric>{t('reports.trialBalance.credit')}</Th>
                        </Tr>
                      </Thead>
                      <Tbody>
                        {ssotTBData.accounts && ssotTBData.accounts.length > 0 ? (
                          ssotTBData.accounts.map((account: any, index: number) => (
                            <Tr key={account.account_id || index} _hover={{ bg: 'gray.50' }}>
                              <Td fontFamily="mono" fontWeight="medium">{account.account_code || 'N/A'}</Td>
                              <Td>{account.account_name || 'Unnamed Account'}</Td>
                              <Td>
                                <Badge colorScheme={
                                  account.account_type === 'Asset' ? 'green' :
                                    account.account_type === 'Liability' ? 'orange' :
                                      account.account_type === 'Equity' ? 'purple' :
                                        account.account_type === 'Revenue' ? 'blue' :
                                          account.account_type === 'Expense' ? 'red' : 'gray'
                                }>
                                  {account.account_type || 'N/A'}
                                </Badge>
                              </Td>
                              <Td isNumeric color="orange.600" fontWeight="medium">
                                {(account.debit_balance && account.debit_balance > 0) ? formatCurrency(account.debit_balance) : '-'}
                              </Td>
                              <Td isNumeric color="green.600" fontWeight="medium">
                                {(account.credit_balance && account.credit_balance > 0) ? formatCurrency(account.credit_balance) : '-'}
                              </Td>
                            </Tr>
                          ))
                        ) : (
                          <Tr>
                            <Td colSpan={5} textAlign="center" py={8}>
                              <Text color="gray.500">No accounts found</Text>
                            </Td>
                          </Tr>
                        )}
                      </Tbody>
                      <Thead bg="gray.200">
                        <Tr>
                          <Th colSpan={3} textAlign="right" fontSize="md">{t('reports.modal.total').toUpperCase()}</Th>
                          <Th isNumeric fontSize="md" fontWeight="bold" color="orange.700">
                            {formatCurrency(ssotTBData.total_debits || 0)}
                          </Th>
                          <Th isNumeric fontSize="md" fontWeight="bold" color="green.700">
                            {formatCurrency(ssotTBData.total_credits || 0)}
                          </Th>
                        </Tr>
                      </Thead>
                    </Table>
                  </TableContainer>
                </Box>
              </VStack>
            )}
          </ModalBody>
          <ModalFooter>
            <HStack spacing={3}>
              {ssotTBData && !ssotTBLoading && (
                <>
                  <Button
                    colorScheme="red"
                    variant="outline"
                    size="sm"
                    leftIcon={<FiFilePlus />}
                    onClick={() => handleTrialBalanceExport('pdf')}
                    isLoading={loading}
                  >
                    {t('reports.exportPDF')}
                  </Button>
                  <Button
                    colorScheme="green"
                    variant="outline"
                    size="sm"
                    leftIcon={<FiFileText />}
                    onClick={() => handleTrialBalanceExport('csv')}
                    isLoading={loading}
                  >
                    {t('reports.exportCSV')}
                  </Button>
                </>
              )}
            </HStack>
            <Button variant="ghost" onClick={() => setSSOTTBOpen(false)}>
              {t('reports.close')}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      {/* Customer History Modal */}
      <Modal isOpen={customerHistoryOpen} onClose={() => setCustomerHistoryOpen(false)} size="6xl">
        <ModalOverlay />
        <ModalContent bg={modalContentBg}>
          <ModalHeader>
            <HStack>
              <Icon as={FiUsers} color="blue.500" />
              <VStack align="start" spacing={0}>
                <Text fontSize="lg" fontWeight="bold">
                  {t('reports.customerHistoryReport.title')}
                </Text>
                <Text fontSize="sm" color={previewPeriodTextColor}>
                  {t('reports.customerHistoryReport.description')}
                </Text>
              </VStack>
            </HStack>
          </ModalHeader>
          <ModalCloseButton />
          <ModalBody pb={6}>
            <Box mb={4}>
              <VStack spacing={4} align="stretch">
                <HStack spacing={4} flexWrap="wrap">
                  <FormControl flex="1" minW="250px" isRequired>
                    <FormLabel>{t('reports.customerHistoryReport.selectCustomer')}</FormLabel>
                    <Select
                      placeholder={t('reports.customerHistoryReport.chooseCustomer')}
                      value={customerHistoryCustomerId}
                      onChange={(e) => setCustomerHistoryCustomerId(e.target.value)}
                      borderColor={!customerHistoryCustomerId ? 'red.300' : 'gray.200'}
                    >
                      {customers.map((customer) => (
                        <option key={customer.id} value={customer.id}>
                          {customer.name} ({customer.code})
                        </option>
                      ))}
                    </Select>
                    <Text fontSize="xs" color="gray.500" mt={1}>
                      {t('reports.customerHistoryReport.available', { count: customers.length })}
                    </Text>
                  </FormControl>
                  <FormControl flex="1" minW="180px" isRequired>
                    <FormLabel>{t('reports.generalLedgerReport.startDate')}</FormLabel>
                    <Input
                      type="date"
                      value={customerHistoryStartDate}
                      onChange={(e) => setCustomerHistoryStartDate(e.target.value)}
                      borderColor={!customerHistoryStartDate ? 'red.300' : 'gray.200'}
                    />
                  </FormControl>
                  <FormControl flex="1" minW="180px" isRequired>
                    <FormLabel>{t('reports.generalLedgerReport.endDate')}</FormLabel>
                    <Input
                      type="date"
                      value={customerHistoryEndDate}
                      onChange={(e) => setCustomerHistoryEndDate(e.target.value)}
                      borderColor={!customerHistoryEndDate ? 'red.300' : 'gray.200'}
                    />
                  </FormControl>
                  <Button
                    colorScheme="blue"
                    onClick={fetchCustomerHistory}
                    isLoading={customerHistoryLoading}
                    isDisabled={!customerHistoryCustomerId || !customerHistoryStartDate || !customerHistoryEndDate}
                    leftIcon={<FiUsers />}
                    size="md"
                    mt={8}
                    whiteSpace="nowrap"
                  >
                    {t('reports.generalLedgerReport.generateReport')}
                  </Button>
                </HStack>
                <Text fontSize="xs" color={descriptionColor}>
                  {t('reports.customerHistoryReport.helperText')}
                </Text>
              </VStack>
            </Box>

            {customerHistoryLoading && (
              <Box textAlign="center" py={8}>
                <VStack spacing={4}>
                  <Spinner size="xl" thickness="4px" speed="0.65s" color="blue.500" />
                  <VStack spacing={2}>
                    <Text fontSize="lg" fontWeight="medium" color={loadingTextColor}>
                      Loading Customer History
                    </Text>
                    <Text fontSize="sm" color={descriptionColor}>
                      Fetching transactions and payment records...
                    </Text>
                  </VStack>
                </VStack>
              </Box>
            )}

            {customerHistoryError && (
              <Box bg="red.50" p={4} borderRadius="md" mb={4}>
                <Text color="red.600" fontWeight="medium">Error: {customerHistoryError}</Text>
                <Button
                  mt={2}
                  size="sm"
                  colorScheme="red"
                  variant="outline"
                  onClick={fetchCustomerHistory}
                >
                  Retry
                </Button>
              </Box>
            )}

            {customerHistoryData && !customerHistoryLoading && (
              <VStack spacing={6} align="stretch">
                {/* Customer Info */}
                {customerHistoryData.customer && (
                  <Box bg="blue.50" p={4} borderRadius="md" borderWidth="1px" borderColor="blue.200">
                    <VStack align="start" spacing={2}>
                      <HStack>
                        <Text fontSize="lg" fontWeight="bold" color="blue.800">
                          {customerHistoryData.customer.name}
                        </Text>
                        <Badge colorScheme="blue">{customerHistoryData.customer.code}</Badge>
                      </HStack>
                      <Text fontSize="sm" color="blue.600">
                        {customerHistoryData.customer.email} | {customerHistoryData.customer.phone}
                      </Text>
                      {customerHistoryData.customer.address && (
                        <Text fontSize="sm" color="blue.600">
                          {customerHistoryData.customer.address}
                        </Text>
                      )}
                    </VStack>
                  </Box>
                )}

                {/* Summary */}
                {customerHistoryData.summary && (
                  <SimpleGrid columns={[1, 2, 4]} spacing={4}>
                    <Box bg="green.50" p={4} borderRadius="md" borderWidth="1px" borderColor="green.200">
                      <Text fontSize="sm" color="gray.600" mb={1}>{t('reports.customerHistoryReport.totalTransactions')}</Text>
                      <Text fontSize="2xl" fontWeight="bold" color="green.600">
                        {customerHistoryData.summary.total_transactions || 0}
                      </Text>
                    </Box>
                    <Box bg="blue.50" p={4} borderRadius="md" borderWidth="1px" borderColor="blue.200">
                      <Text fontSize="sm" color="gray.600" mb={1}>{t('reports.customerHistoryReport.totalAmount')}</Text>
                      <Text fontSize="2xl" fontWeight="bold" color="blue.600">
                        {formatCurrency(customerHistoryData.summary.total_amount || 0)}
                      </Text>
                    </Box>
                    <Box bg="purple.50" p={4} borderRadius="md" borderWidth="1px" borderColor="purple.200">
                      <Text fontSize="sm" color="gray.600" mb={1}>{t('reports.customerHistoryReport.totalPaid')}</Text>
                      <Text fontSize="2xl" fontWeight="bold" color="purple.600">
                        {formatCurrency(customerHistoryData.summary.total_paid || 0)}
                      </Text>
                    </Box>
                    <Box bg="orange.50" p={4} borderRadius="md" borderWidth="1px" borderColor="orange.200">
                      <Text fontSize="sm" color="gray.600" mb={1}>{t('reports.customerHistoryReport.outstanding')}</Text>
                      <Text fontSize="2xl" fontWeight="bold" color="orange.600">
                        {formatCurrency(customerHistoryData.summary.total_outstanding || 0)}
                      </Text>
                    </Box>
                  </SimpleGrid>
                )}

                {/* Transactions Table */}
                <Box bg={cardBg} p={4} borderRadius="md" borderWidth="1px" borderColor={borderColor}>
                  <Text fontSize="lg" fontWeight="bold" color={headingColor} mb={3}>
                    Transaction Details
                  </Text>
                  <Box overflowX="auto" maxH="500px" overflowY="auto">
                    <Table size="sm" variant="striped" colorScheme="gray">
                      <Thead bg="blue.600" position="sticky" top={0} zIndex={1}>
                        <Tr>
                          <Th color="white">{t('reports.customerHistoryReport.date')}</Th>
                          <Th color="white">{t('reports.customerHistoryReport.type')}</Th>
                          <Th color="white">{t('reports.customerHistoryReport.code')}</Th>
                          <Th color="white">{t('reports.customerHistoryReport.desc')}</Th>
                          <Th color="white" isNumeric>{t('reports.customerHistoryReport.amount')}</Th>
                          <Th color="white" isNumeric>{t('reports.customerHistoryReport.paid')}</Th>
                          <Th color="white" isNumeric>{t('reports.customerHistoryReport.outstanding')}</Th>
                          <Th color="white">{t('reports.customerHistoryReport.status')}</Th>
                        </Tr>
                      </Thead>
                      <Tbody>
                        {customerHistoryData.transactions && customerHistoryData.transactions.length > 0 ? (
                          customerHistoryData.transactions.map((tx: any, index: number) => (
                            <Tr key={index} _hover={{ bg: 'blue.50' }}>
                              <Td fontSize="sm">
                                {tx.date ? new Date(tx.date).toLocaleDateString('id-ID') : 'N/A'}
                              </Td>
                              <Td>
                                <Badge colorScheme={
                                  tx.transaction_type === 'SALE' ? 'green' :
                                    tx.transaction_type === 'INVOICE' ? 'blue' :
                                      tx.transaction_type === 'PAYMENT' ? 'purple' : 'gray'
                                } size="sm">
                                  {tx.transaction_type}
                                </Badge>
                              </Td>
                              <Td fontSize="sm" fontFamily="mono" fontWeight="medium">{tx.transaction_code}</Td>
                              <Td fontSize="sm" maxW="250px">{tx.description}</Td>
                              <Td isNumeric fontSize="sm" fontWeight="bold" color="blue.700">
                                {formatCurrency(tx.amount || 0)}
                              </Td>
                              <Td isNumeric fontSize="sm" color="green.600">
                                {formatCurrency(tx.paid_amount || 0)}
                              </Td>
                              <Td isNumeric fontSize="sm" color="orange.600" fontWeight="medium">
                                {formatCurrency(tx.outstanding || 0)}
                              </Td>
                              <Td>
                                <Badge colorScheme={
                                  tx.status === 'PAID' ? 'green' :
                                    tx.status === 'UNPAID' ? 'red' :
                                      tx.status === 'PARTIAL' ? 'yellow' : 'gray'
                                } size="sm">
                                  {tx.status}
                                </Badge>
                              </Td>
                            </Tr>
                          ))
                        ) : (
                          <Tr>
                            <Td colSpan={8} textAlign="center" py={8}>
                              <Text color="gray.500">{t('reports.customerHistoryReport.noTransactions')}</Text>
                            </Td>
                          </Tr>
                        )}
                      </Tbody>
                    </Table>
                  </Box>
                </Box>
              </VStack>
            )}
          </ModalBody>
          <ModalFooter>
            <HStack spacing={3}>
              {customerHistoryData && !customerHistoryLoading && (
                <>
                  <Button
                    colorScheme="red"
                    variant="outline"
                    size="sm"
                    leftIcon={<FiFilePlus />}
                    onClick={() => handleCustomerHistoryExport('pdf')}
                    isLoading={loading}
                  >
                    {t('reports.exportPDF')}
                  </Button>
                  <Button
                    colorScheme="green"
                    variant="outline"
                    size="sm"
                    leftIcon={<FiFileText />}
                    onClick={() => handleCustomerHistoryExport('csv')}
                    isLoading={loading}
                  >
                    {t('reports.exportCSV')}
                  </Button>
                </>
              )}
            </HStack>
            <Button variant="ghost" onClick={() => setCustomerHistoryOpen(false)}>
              {t('reports.close')}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      {/* Vendor History Modal */}
      <Modal isOpen={vendorHistoryOpen} onClose={() => setVendorHistoryOpen(false)} size="6xl">
        <ModalOverlay />
        <ModalContent bg={modalContentBg}>
          <ModalHeader>
            <HStack>
              <Icon as={FiTruck} color="orange.500" />
              <VStack align="start" spacing={0}>
                <Text fontSize="lg" fontWeight="bold">
                  {t('reports.vendorHistoryReport.title')}
                </Text>
                <Text fontSize="sm" color={previewPeriodTextColor}>
                  {t('reports.vendorHistoryReport.description')}
                </Text>
              </VStack>
            </HStack>
          </ModalHeader>
          <ModalCloseButton />
          <ModalBody pb={6}>
            <Box mb={4}>
              <VStack spacing={4} align="stretch">
                <HStack spacing={4} flexWrap="wrap">
                  <FormControl flex="1" minW="250px" isRequired>
                    <FormLabel>{t('reports.vendorHistoryReport.selectVendor')}</FormLabel>
                    <Select
                      placeholder={t('reports.vendorHistoryReport.chooseVendor')}
                      value={vendorHistoryVendorId}
                      onChange={(e) => setVendorHistoryVendorId(e.target.value)}
                      borderColor={!vendorHistoryVendorId ? 'red.300' : 'gray.200'}
                    >
                      {vendors.map((vendor) => (
                        <option key={vendor.id} value={vendor.id}>
                          {vendor.name} ({vendor.code})
                        </option>
                      ))}
                    </Select>
                  </FormControl>
                  <FormControl flex="1" minW="180px" isRequired>
                    <FormLabel>{t('reports.generalLedgerReport.startDate')}</FormLabel>
                    <Input
                      type="date"
                      value={vendorHistoryStartDate}
                      onChange={(e) => setVendorHistoryStartDate(e.target.value)}
                      borderColor={!vendorHistoryStartDate ? 'red.300' : 'gray.200'}
                    />
                  </FormControl>
                  <FormControl flex="1" minW="180px" isRequired>
                    <FormLabel>{t('reports.generalLedgerReport.endDate')}</FormLabel>
                    <Input
                      type="date"
                      value={vendorHistoryEndDate}
                      onChange={(e) => setVendorHistoryEndDate(e.target.value)}
                      borderColor={!vendorHistoryEndDate ? 'red.300' : 'gray.200'}
                    />
                  </FormControl>
                  <Button
                    colorScheme="orange"
                    onClick={fetchVendorHistory}
                    isLoading={vendorHistoryLoading}
                    isDisabled={!vendorHistoryVendorId || !vendorHistoryStartDate || !vendorHistoryEndDate}
                    leftIcon={<FiTruck />}
                    size="md"
                    mt={8}
                    whiteSpace="nowrap"
                  >
                    {t('reports.generalLedgerReport.generateReport')}
                  </Button>
                </HStack>
                <Text fontSize="xs" color={descriptionColor}>
                  {t('reports.vendorHistoryReport.helperText')}
                </Text>
              </VStack>
            </Box>

            {vendorHistoryLoading && (
              <Box textAlign="center" py={8}>
                <VStack spacing={4}>
                  <Spinner size="xl" thickness="4px" speed="0.65s" color="orange.500" />
                  <VStack spacing={2}>
                    <Text fontSize="lg" fontWeight="medium" color={loadingTextColor}>
                      onClose={() => setVendorHistoryOpen(false)}
                      title={t('reports.vendorHistoryReport.loading')}
                      message={t('reports.vendorHistoryReport.fetching')}
                    </Text>
                  </VStack>
                </VStack>
              </Box>
            )}

            {vendorHistoryError && (
              <Box bg="red.50" p={4} borderRadius="md" mb={4}>
                <Text color="red.600" fontWeight="medium">Error: {vendorHistoryError}</Text>
                <Button
                  mt={2}
                  size="sm"
                  colorScheme="red"
                  variant="outline"
                  onClick={fetchVendorHistory}
                >
                  Retry
                </Button>
              </Box>
            )}

            {vendorHistoryData && !vendorHistoryLoading && (
              <VStack spacing={6} align="stretch">
                {/* Vendor Info */}
                {vendorHistoryData.vendor && (
                  <Box bg="orange.50" p={4} borderRadius="md" borderWidth="1px" borderColor="orange.200">
                    <VStack align="start" spacing={2}>
                      <HStack>
                        <Text fontSize="lg" fontWeight="bold" color="orange.800">
                          {vendorHistoryData.vendor.name}
                        </Text>
                        <Badge colorScheme="orange">{vendorHistoryData.vendor.code}</Badge>
                      </HStack>
                      <Text fontSize="sm" color="orange.600">
                        {vendorHistoryData.vendor.email} | {vendorHistoryData.vendor.phone}
                      </Text>
                      {vendorHistoryData.vendor.address && (
                        <Text fontSize="sm" color="orange.600">
                          {vendorHistoryData.vendor.address}
                        </Text>
                      )}
                    </VStack>
                  </Box>
                )}

                {/* Summary */}
                {vendorHistoryData.summary && (
                  <SimpleGrid columns={[1, 2, 4]} spacing={4}>
                    <Box bg="green.50" p={4} borderRadius="md" borderWidth="1px" borderColor="green.200">
                      <Text fontSize="sm" color="gray.600" mb={1}>Total Transactions</Text>
                      <Text fontSize="2xl" fontWeight="bold" color="green.600">
                        {vendorHistoryData.summary.total_transactions || 0}
                      </Text>
                    </Box>
                    <Box bg="blue.50" p={4} borderRadius="md" borderWidth="1px" borderColor="blue.200">
                      <Text fontSize="sm" color="gray.600" mb={1}>Total Amount</Text>
                      <Text fontSize="2xl" fontWeight="bold" color="blue.600">
                        {formatCurrency(vendorHistoryData.summary.total_amount || 0)}
                      </Text>
                    </Box>
                    <Box bg="purple.50" p={4} borderRadius="md" borderWidth="1px" borderColor="purple.200">
                      <Text fontSize="sm" color="gray.600" mb={1}>Total Paid</Text>
                      <Text fontSize="2xl" fontWeight="bold" color="purple.600">
                        {formatCurrency(vendorHistoryData.summary.total_paid || 0)}
                      </Text>
                    </Box>
                    <Box bg="orange.50" p={4} borderRadius="md" borderWidth="1px" borderColor="orange.200">
                      <Text fontSize="sm" color="gray.600" mb={1}>Outstanding</Text>
                      <Text fontSize="2xl" fontWeight="bold" color="orange.600">
                        {formatCurrency(vendorHistoryData.summary.total_outstanding || 0)}
                      </Text>
                    </Box>
                  </SimpleGrid>
                )}

                {/* Transactions Table */}
                <Box bg={cardBg} p={4} borderRadius="md" borderWidth="1px" borderColor={borderColor}>
                  <Text fontSize="lg" fontWeight="bold" color={headingColor} mb={3}>
                    Transaction Details
                  </Text>
                  <Box overflowX="auto" maxH="500px" overflowY="auto">
                    <Table size="sm" variant="striped" colorScheme="gray">
                      <Thead bg="orange.600" position="sticky" top={0} zIndex={1}>
                        <Tr>
                          <Th color="white">{t('reports.customerHistoryReport.date')}</Th>
                          <Th color="white">{t('reports.customerHistoryReport.type')}</Th>
                          <Th color="white">{t('reports.customerHistoryReport.code')}</Th>
                          <Th color="white">{t('reports.customerHistoryReport.desc')}</Th>
                          <Th color="white" isNumeric>{t('reports.customerHistoryReport.amount')}</Th>
                          <Th color="white" isNumeric>{t('reports.customerHistoryReport.paid')}</Th>
                          <Th color="white" isNumeric>{t('reports.customerHistoryReport.outstanding')}</Th>
                          <Th color="white">{t('reports.customerHistoryReport.status')}</Th>
                        </Tr>
                      </Thead>
                      <Tbody>
                        {vendorHistoryData.transactions && vendorHistoryData.transactions.length > 0 ? (
                          vendorHistoryData.transactions.map((tx: any, index: number) => (
                            <Tr key={index} _hover={{ bg: 'orange.50' }}>
                              <Td fontSize="sm">
                                {tx.date ? new Date(tx.date).toLocaleDateString('id-ID') : 'N/A'}
                              </Td>
                              <Td>
                                <Badge colorScheme={
                                  tx.transaction_type === 'PURCHASE' ? 'orange' :
                                    tx.transaction_type === 'BILL' ? 'red' :
                                      tx.transaction_type === 'PAYMENT' ? 'purple' : 'gray'
                                } size="sm">
                                  {tx.transaction_type}
                                </Badge>
                              </Td>
                              <Td fontSize="sm" fontFamily="mono" fontWeight="medium">{tx.transaction_code}</Td>
                              <Td fontSize="sm" maxW="250px">{tx.description}</Td>
                              <Td isNumeric fontSize="sm" fontWeight="bold" color="orange.700">
                                {formatCurrency(tx.amount || 0)}
                              </Td>
                              <Td isNumeric fontSize="sm" color="green.600">
                                {formatCurrency(tx.paid_amount || 0)}
                              </Td>
                              <Td isNumeric fontSize="sm" color="red.600" fontWeight="medium">
                                {formatCurrency(tx.outstanding || 0)}
                              </Td>
                              <Td>
                                <Badge colorScheme={
                                  tx.status === 'PAID' ? 'green' :
                                    tx.status === 'UNPAID' ? 'red' :
                                      tx.status === 'PARTIAL' ? 'yellow' : 'gray'
                                } size="sm">
                                  {tx.status}
                                </Badge>
                              </Td>
                            </Tr>
                          ))
                        ) : (
                          <Tr>
                            <Td colSpan={8} textAlign="center" py={8}>
                              <Text color="gray.500">No transactions found for this period</Text>
                            </Td>
                          </Tr>
                        )}
                      </Tbody>
                    </Table>
                  </Box>
                </Box>
              </VStack>
            )}
          </ModalBody>
          <ModalFooter>
            <HStack spacing={3}>
              {vendorHistoryData && !vendorHistoryLoading && (
                <>
                  <Button
                    colorScheme="red"
                    variant="outline"
                    size="sm"
                    leftIcon={<FiFilePlus />}
                    onClick={() => handleVendorHistoryExport('pdf')}
                    isLoading={loading}
                  >
                    {t('reports.exportPDF')}
                  </Button>
                  <Button
                    colorScheme="green"
                    variant="outline"
                    size="sm"
                    leftIcon={<FiFileText />}
                    onClick={() => handleVendorHistoryExport('csv')}
                    isLoading={loading}
                  >
                    {t('reports.exportCSV')}
                  </Button>
                </>
              )}
            </HStack>
            <Button variant="ghost" onClick={() => setVendorHistoryOpen(false)}>
              {t('reports.close')}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      {/* SSOT General Ledger Modal */}
      <Modal isOpen={ssotGLOpen} onClose={() => setSSOTGLOpen(false)} size="full">
        <ModalOverlay />
        <ModalContent bg={modalContentBg} maxW="95vw" m={4}>
          <ModalHeader>
            <HStack>
              <Icon as={FiBook} color="green.500" />
              <VStack align="start" spacing={0}>
                <Text fontSize="lg" fontWeight="bold">
                  General Ledger (SSOT)
                </Text>
                <Text fontSize="sm" color={previewPeriodTextColor}>
                  {ssotGLStartDate} - {ssotGLEndDate} | SSOT Journal Integration
                </Text>
              </VStack>
            </HStack>
          </ModalHeader>
          <ModalCloseButton />
          <ModalBody pb={6}>
            <Box mb={4}>
              <VStack spacing={4} align="stretch">
                {/* Closed Period History Selector */}
                <HStack spacing={4} mb={2} alignItems="flex-start">
                  <FormControl flex="1">
                    <FormLabel fontSize="sm">
                      <HStack spacing={2}>
                        <Icon as={FiCalendar} />
                        <Text>{t('reports.closedPeriod')}</Text>
                        <Tooltip label="Select a closed period to view historical General Ledger" placement="top">
                          <Box><Icon as={FiInfo} color="gray.500" /></Box>
                        </Tooltip>
                      </HStack>
                    </FormLabel>
                    <Select
                      placeholder={t('reports.selectClosedPeriod')}
                      size="md"
                      onChange={(e) => {
                        if (e.target.value) {
                          const selectedPeriod = ssotGLClosedPeriods.find(p => p.date === e.target.value);
                          if (selectedPeriod) {
                            // Set date range untuk periode yang dipilih
                            // Prioritas: gunakan start_date dari accounting_periods jika tersedia
                            const rawStart = selectedPeriod.start_date as string | undefined;

                            if (rawStart) {
                              const normalizedStart = String(rawStart).slice(0, 10);
                              setSSOTGLStartDate(normalizedStart);
                            } else {
                              // Fallback: gunakan fiscal year start (behaviour lama)
                              const periodEndDate = new Date(selectedPeriod.date);
                              const fiscalYearStart = new Date(periodEndDate.getFullYear(), 0, 1); // Default Jan 1
                              setSSOTGLStartDate(fiscalYearStart.toISOString().split('T')[0]);
                            }

                            // End date selalu pakai end_date / entry_date dari history
                            setSSOTGLEndDate(selectedPeriod.date);
                          }
                        }
                      }}
                      isDisabled={loadingSSOTGLClosedPeriods}
                    >
                      {ssotGLClosedPeriods.length > 0 ? (
                        ssotGLClosedPeriods.map((period, index) => (
                          <option key={`${period.date}-${index}`} value={period.date}>
                            {period.label}
                          </option>
                        ))
                      ) : (
                        <option disabled>{t('reports.noClosedPeriodsAvailable')}</option>
                      )}
                    </Select>
                  </FormControl>
                  <Button
                    size="md"
                    variant="outline"
                    colorScheme="blue"
                    onClick={async () => {
                      setLoadingSSOTGLClosedPeriods(true);
                      try {
                        const response = await api.get(API_ENDPOINTS.FISCAL_CLOSING.HISTORY);
                        if (response.data.success && response.data.data && Array.isArray(response.data.data)) {
                          const periods = buildUniqueClosedPeriods(response.data.data);
                          setSSOTGLClosedPeriods(periods);
                          toast({
                            title: 'Success',
                            description: `Loaded ${periods.length} closed periods`,
                            status: 'success',
                            duration: 2000
                          });
                        }
                      } catch (error) {
                        console.error('Failed to load closed periods:', error);
                      } finally {
                        setLoadingSSOTGLClosedPeriods(false);
                      }
                    }}
                    isLoading={loadingSSOTGLClosedPeriods}
                    leftIcon={<FiClock />}
                    mt={7}
                  >
                    {t('reports.generalLedgerReport.loadHistory')}
                  </Button>
                </HStack>

                <Divider my={2} />

                <HStack spacing={4} flexWrap="wrap">
                  <FormControl flex="1" minW="200px" isRequired>
                    <FormLabel>{t('reports.generalLedgerReport.startDate')}</FormLabel>
                    <Input
                      type="date"
                      value={ssotGLStartDate}
                      onChange={(e) => setSSOTGLStartDate(e.target.value)}
                      isRequired
                      borderColor={!ssotGLStartDate ? 'red.300' : 'gray.200'}
                    />
                  </FormControl>
                  <FormControl flex="1" minW="200px" isRequired>
                    <FormLabel>{t('reports.generalLedgerReport.endDate')}</FormLabel>
                    <Input
                      type="date"
                      value={ssotGLEndDate}
                      onChange={(e) => setSSOTGLEndDate(e.target.value)}
                      isRequired
                      borderColor={!ssotGLEndDate ? 'red.300' : 'gray.200'}
                    />
                  </FormControl>
                  <FormControl flex="1" minW="200px">
                    <FormLabel>{t('reports.generalLedgerReport.accountId')}</FormLabel>
                    <Input
                      type="text"
                      placeholder={t('reports.generalLedgerReport.accountIdPlaceholder')}
                      value={ssotGLAccountId}
                      onChange={(e) => setSSOTGLAccountId(e.target.value)}
                    />
                  </FormControl>
                  <Button
                    colorScheme="blue"
                    onClick={fetchSSOTGeneralLedgerReport}
                    isLoading={ssotGLLoading}
                    isDisabled={!ssotGLStartDate || !ssotGLEndDate}
                    leftIcon={<FiBook />}
                    size="md"
                    mt={8}
                    whiteSpace="nowrap"
                  >
                    {t('reports.generalLedgerReport.generateReport')}
                  </Button>
                </HStack>
                <Text fontSize="xs" color={descriptionColor}>
                  {t('reports.generalLedgerReport.accountIdHelper')}
                </Text>
              </VStack>
            </Box>

            {ssotGLLoading && (
              <Box textAlign="center" py={8}>
                <VStack spacing={4}>
                  <Spinner size="xl" thickness="4px" speed="0.65s" color="green.500" />
                  <VStack spacing={2}>
                    <Text fontSize="lg" fontWeight="medium" color={loadingTextColor}>
                      {t('reports.generalLedgerReport.generating')}
                    </Text>
                    <Text fontSize="sm" color={descriptionColor}>
                      {t('reports.generalLedgerReport.analyzing')}
                    </Text>
                  </VStack>
                </VStack>
              </Box>
            )}

            {ssotGLError && (
              <Box bg="red.50" p={4} borderRadius="md" mb={4}>
                <Text color="red.600" fontWeight="medium">Error: {ssotGLError}</Text>
                <Button
                  mt={2}
                  size="sm"
                  colorScheme="red"
                  variant="outline"
                  onClick={fetchSSOTGeneralLedgerReport}
                >
                  Retry
                </Button>
              </Box>
            )}

            {ssotGLData && !ssotGLLoading && (
              <VStack spacing={6} align="stretch">
                {/* Company Header */}
                {ssotGLData.company && (
                  <Box bg="green.50" p={4} borderRadius="md">
                    <HStack justify="space-between" align="start">
                      <VStack align="start" spacing={1}>
                        <Text fontSize="lg" fontWeight="bold" color="green.800">
                          {ssotGLData.company.name || 'Company Name Not Available'}
                        </Text>
                        <Text fontSize="sm" color="green.600">
                          {ssotGLData.company.address ? (
                            ssotGLData.company.city ? `${ssotGLData.company.address}, ${ssotGLData.company.city}` : ssotGLData.company.address
                          ) : 'Address not available'}
                        </Text>
                        {ssotGLData.company.phone && (
                          <Text fontSize="sm" color="green.600">
                            {ssotGLData.company.phone} | {ssotGLData.company.email}
                          </Text>
                        )}
                      </VStack>
                      <VStack align="end" spacing={1}>
                        <Text fontSize="sm" color="green.600">
                          Currency: {ssotGLData.currency || 'IDR'}
                        </Text>
                        <Text fontSize="xs" color="green.500">
                          Generated: {ssotGLData.generated_at ? new Date(ssotGLData.generated_at).toLocaleString('id-ID') : 'N/A'}
                        </Text>
                      </VStack>
                    </HStack>
                  </Box>
                )}

                {/* Report Header */}
                <Box textAlign="center" bg={summaryBg} p={4} borderRadius="md">
                  <Heading size="md" color={headingColor}>
                    {t('reports.generalLedgerReport.title')}
                  </Heading>
                  <Text fontSize="sm" color={descriptionColor}>
                    {t('reports.generalLedgerReport.period')} {ssotGLStartDate} - {ssotGLEndDate}
                  </Text>
                  <Text fontSize="xs" color={descriptionColor} mt={1}>
                    {t('reports.generalLedgerReport.generatedOn')} {new Date().toLocaleDateString('id-ID')} at {new Date().toLocaleTimeString('id-ID')}
                  </Text>
                </Box>

                {/* Summary Statistics */}
                <Box bg="gray.50" p={6} borderRadius="lg" border="2px" borderColor="gray.300">
                  <SimpleGrid columns={[1, 2, 5]} spacing={6}>
                    <Box textAlign="center">
                      <Text fontSize="xs" color="gray.600" mb={1} fontWeight="semibold" textTransform="uppercase">{t('reports.generalLedgerReport.totalEntries')}</Text>
                      <Text fontSize="3xl" fontWeight="bold" color="green.600">
                        {ssotGLData.transactions?.length || 0}
                      </Text>
                    </Box>
                    <Box textAlign="center">
                      <Text fontSize="xs" color="gray.600" mb={1} fontWeight="semibold" textTransform="uppercase">{t('reports.generalLedgerReport.openingBalance')}</Text>
                      <Text fontSize="2xl" fontWeight="bold" color="blue.600">
                        {formatCurrency(ssotGLData.opening_balance || 0)}
                      </Text>
                    </Box>
                    <Box textAlign="center">
                      <Text fontSize="xs" color="gray.600" mb={1} fontWeight="semibold" textTransform="uppercase">{t('reports.generalLedgerReport.totalDebits')}</Text>
                      <Text fontSize="2xl" fontWeight="bold" color="cyan.600">
                        {formatCurrency(ssotGLData.total_debits || 0)}
                      </Text>
                    </Box>
                    <Box textAlign="center">
                      <Text fontSize="xs" color="gray.600" mb={1} fontWeight="semibold" textTransform="uppercase">{t('reports.generalLedgerReport.totalCredits')}</Text>
                      <Text fontSize="2xl" fontWeight="bold" color="pink.600">
                        {formatCurrency(ssotGLData.total_credits || 0)}
                      </Text>
                    </Box>
                    <Box textAlign="center">
                      <Text fontSize="xs" color="gray.600" mb={1} fontWeight="semibold" textTransform="uppercase">{t('reports.generalLedgerReport.closingBalance')}</Text>
                      <Text fontSize="2xl" fontWeight="bold" color="purple.600">
                        {formatCurrency(ssotGLData.closing_balance || 0)}
                      </Text>
                    </Box>
                  </SimpleGrid>

                  <Divider my={4} />

                  <SimpleGrid columns={[1, 2]} spacing={4}>
                    <Box textAlign="center" py={2}>
                      <Text fontSize="xs" color="gray.600" mb={1} fontWeight="semibold" textTransform="uppercase">{t('reports.generalLedgerReport.netChange')}</Text>
                      <Text fontSize="xl" fontWeight="bold" color={
                        ((ssotGLData.total_debits || 0) - (ssotGLData.total_credits || 0)) > 0 ? "green.600" :
                          ((ssotGLData.total_debits || 0) - (ssotGLData.total_credits || 0)) < 0 ? "red.600" : "gray.600"
                      }>
                        {formatCurrency((ssotGLData.total_debits || 0) - (ssotGLData.total_credits || 0))}
                      </Text>
                      <Badge colorScheme={
                        (ssotGLData.total_debits || 0) === (ssotGLData.total_credits || 0) ? "green" :
                          (ssotGLData.total_debits || 0) > (ssotGLData.total_credits || 0) ? "blue" : "orange"
                      } size="sm" mt={1}>
                        {(ssotGLData.total_debits || 0) === (ssotGLData.total_credits || 0) ? "Balanced" :
                          (ssotGLData.total_debits || 0) > (ssotGLData.total_credits || 0) ? "Net Debit" : "Net Credit"}
                      </Badge>
                    </Box>
                    <Box textAlign="center" py={2}>
                      <Text fontSize="xs" color="gray.600" mb={1} fontWeight="semibold" textTransform="uppercase">{t('reports.generalLedgerReport.status')}</Text>
                      <Badge colorScheme={
                        ssotGLData.is_balanced ? "green" : "yellow"
                      } size="lg" fontSize="md" px={3} py={1}>
                        {ssotGLData.is_balanced ? `✓ ${t('reports.generalLedgerReport.balanced')}` : `⚠ ${t('reports.generalLedgerReport.unbalanced')}`}
                      </Badge>
                    </Box>
                  </SimpleGrid>
                </Box>

                {/* Account Information */}
                {ssotGLData.account && (
                  <Box bg="blue.50" p={4} borderRadius="md" border="1px" borderColor="blue.200">
                    <HStack justify="space-between">
                      <VStack align="start" spacing={0}>
                        <Text fontSize="lg" fontWeight="bold" color="blue.800">
                          {ssotGLData.account.account_code} - {ssotGLData.account.account_name}
                        </Text>
                        <Text fontSize="sm" color="blue.600">
                          Account Type: {ssotGLData.account.account_type || 'N/A'}
                        </Text>
                      </VStack>
                    </HStack>
                  </Box>
                )}

                {/* Transaction Details Table */}
                <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor} overflowX="auto">
                  <HStack justify="space-between" mb={3}>
                    <Text fontSize="md" fontWeight="bold" color={headingColor}>
                      {t('reports.generalLedgerReport.transactionDetails')}
                    </Text>
                    <Badge colorScheme="blue" fontSize="sm" px={2} py={1}>
                      {ssotGLData.transactions?.length || 0} {t('reports.generalLedgerReport.entries').toLowerCase()}
                    </Badge>
                  </HStack>
                  <Box overflowX="auto" maxH="600px" overflowY="auto">
                    <Table size="sm" variant="striped" colorScheme="gray">
                      <Thead bg="blue.600" position="sticky" top={0} zIndex={1}>
                        <Tr>
                          <Th color="white" fontSize="xs" py={3}>{t('reports.generalLedgerReport.date')}</Th>
                          <Th color="white" fontSize="xs" py={3}>{t('reports.generalLedgerReport.journalCode')}</Th>
                          <Th color="white" fontSize="xs" py={3} minW="250px">{t('reports.generalLedgerReport.description')}</Th>
                          <Th color="white" fontSize="xs" py={3}>{t('reports.generalLedgerReport.reference')}</Th>
                          <Th color="white" fontSize="xs" py={3} minW="150px">{t('reports.generalLedgerReport.account')}</Th>
                          <Th color="white" fontSize="xs" py={3} isNumeric>{t('reports.generalLedgerReport.debit')}</Th>
                          <Th color="white" fontSize="xs" py={3} isNumeric>{t('reports.generalLedgerReport.credit')}</Th>
                          <Th color="white" fontSize="xs" py={3} isNumeric>{t('reports.generalLedgerReport.balance')}</Th>
                          <Th color="white" fontSize="xs" py={3}>{t('reports.generalLedgerReport.type')}</Th>
                        </Tr>
                      </Thead>
                      <Tbody>
                        {ssotGLData.transactions && ssotGLData.transactions.length > 0 ? (
                          ssotGLData.transactions.map((entry: any, index: number) => (
                            <Tr key={index} _hover={{ bg: 'blue.50' }}>
                              <Td fontSize="sm" whiteSpace="nowrap">
                                {entry.date ? new Date(entry.date).toLocaleDateString('id-ID') : 'N/A'}
                              </Td>
                              <Td fontSize="sm" fontFamily="mono" fontWeight="medium">
                                {entry.journal_code || 'N/A'}
                              </Td>
                              <Td fontSize="sm" maxW="350px">
                                {entry.description || 'No description'}
                              </Td>
                              <Td fontSize="sm">{entry.reference || '-'}</Td>
                              <Td fontSize="sm">
                                {entry.account_code ? (
                                  <VStack align="start" spacing={0}>
                                    <Text fontWeight="semibold" color="gray.700">{entry.account_code}</Text>
                                    <Text fontSize="xs" color="gray.500">{entry.account_name || ''}</Text>
                                  </VStack>
                                ) : (
                                  <Text color="gray.400">-</Text>
                                )}
                              </Td>
                              <Td isNumeric fontSize="sm" fontWeight="semibold" color="blue.700">
                                {(entry.debit_amount && entry.debit_amount > 0) ? formatCurrency(entry.debit_amount) : '-'}
                              </Td>
                              <Td isNumeric fontSize="sm" fontWeight="semibold" color="red.600">
                                {(entry.credit_amount && entry.credit_amount > 0) ? formatCurrency(entry.credit_amount) : '-'}
                              </Td>
                              <Td isNumeric fontSize="sm" fontWeight="bold" color="purple.700" bg="purple.50">
                                {entry.balance != null ? formatCurrency(entry.balance) : '-'}
                              </Td>
                              <Td>
                                <Badge colorScheme={
                                  entry.entry_type === 'SALE' ? 'green' :
                                    entry.entry_type === 'PURCHASE' ? 'blue' :
                                      entry.entry_type === 'PAYMENT' ? 'orange' : 'gray'
                                } size="sm" fontSize="xs">
                                  {entry.entry_type || 'N/A'}
                                </Badge>
                              </Td>
                            </Tr>
                          ))
                        ) : (
                          <Tr>
                            <Td colSpan={9} textAlign="center" py={8}>
                              <VStack spacing={2}>
                                <Text color="gray.500" fontSize="md">No transactions found for this period</Text>
                                <Text color="gray.400" fontSize="sm">Try adjusting your date range or filters</Text>
                              </VStack>
                            </Td>
                          </Tr>
                        )}
                      </Tbody>
                      {ssotGLData.transactions && ssotGLData.transactions.length > 0 && (
                        <Tfoot bg="blue.700" position="sticky" bottom={0}>
                          <Tr>
                            <Th color="white" colSpan={5} textAlign="right" fontSize="md" py={4}>TOTAL</Th>
                            <Th color="white" isNumeric fontSize="md" fontWeight="bold" py={4}>
                              {formatCurrency(ssotGLData.total_debits || 0)}
                            </Th>
                            <Th color="white" isNumeric fontSize="md" fontWeight="bold" py={4}>
                              {formatCurrency(ssotGLData.total_credits || 0)}
                            </Th>
                            <Th color="white" isNumeric fontSize="md" fontWeight="bold" py={4}>
                              {formatCurrency((ssotGLData.total_debits || 0) - (ssotGLData.total_credits || 0))}
                            </Th>
                            <Th></Th>
                          </Tr>
                        </Tfoot>
                      )}
                    </Table>
                  </Box>
                </Box>
              </VStack>
            )}
          </ModalBody>
          <ModalFooter>
            <HStack spacing={3}>
              {ssotGLData && !ssotGLLoading && (
                <>
                  <Button
                    colorScheme="red"
                    variant="outline"
                    size="sm"
                    leftIcon={<FiFilePlus />}
                    onClick={() => handleGeneralLedgerExport('pdf')}
                    isLoading={loading}
                  >
                    {t('reports.exportPDF')}
                  </Button>
                  <Button
                    colorScheme="green"
                    variant="outline"
                    size="sm"
                    leftIcon={<FiFileText />}
                    onClick={() => handleGeneralLedgerExport('csv')}
                    isLoading={loading}
                  >
                    {t('reports.exportCSV')}
                  </Button>
                </>
              )}
            </HStack>
            <Button variant="ghost" onClick={() => setSSOTGLOpen(false)}>
              {t('reports.close')}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      {/* Closing History Modal */}
      <ClosingHistoryModal
        isOpen={closingHistoryOpen}
        onClose={() => setClosingHistoryOpen(false)}
        reportType={closingHistoryReportType}
        currentPeriod={{
          start: ssotStartDate || ssotCFStartDate || ssotSSStartDate,
          end: ssotEndDate || ssotCFEndDate || ssotSSEndDate
        }}
      />

    </SimpleLayout >
  );
};

export default ReportsPage;
