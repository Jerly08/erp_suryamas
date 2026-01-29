'use client';

import React, { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useTranslation } from '@/hooks/useTranslation';
import UnifiedLayout from '@/components/layout/UnifiedLayout';
import { ssotProfitLossService, SSOTProfitLossData } from '@/services/ssotProfitLossService';
import { ssotJournalService } from '@/services/ssotJournalService';
import api from '@/services/api';
import { API_ENDPOINTS } from '@/config/api';
import {
    Box,
    Heading,
    Text,
    Button,
    VStack,
    HStack,
    Card,
    CardBody,
    Icon,
    SimpleGrid,
    FormControl,
    FormLabel,
    Input,
    Select,
    Spinner,
    useToast,
    useColorModeValue,
    Divider,
    Tooltip,
    IconButton,
    Modal,
    ModalOverlay,
    ModalContent,
    ModalHeader,
    ModalCloseButton,
    ModalBody,
    ModalFooter,
    Table,
    Thead,
    Tbody,
    Tr,
    Th,
    Td,
    TableContainer,
    Badge,
    useDisclosure,
} from '@chakra-ui/react';
import {
    FiArrowLeft,
    FiTrendingUp,
    FiDownload,
    FiFileText,
    FiFilePlus,
    FiCalendar,
    FiClock,
    FiInfo,
    FiEye,
} from 'react-icons/fi';

// Helpers
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
    }
    const today = new Date();
    const thisYearStart = new Date(Date.UTC(today.getFullYear(), m - 1, d));
    let start = thisYearStart;
    if (today.getTime() < thisYearStart.getTime()) start = new Date(Date.UTC(today.getFullYear() - 1, m - 1, d));
    const nextStart = new Date(Date.UTC(start.getUTCFullYear() + 1, m - 1, d));
    const end = new Date(nextStart.getTime() - 24 * 60 * 60 * 1000);
    return { startISO: toISO(new Date(start.getUTCFullYear(), start.getUTCMonth(), start.getUTCDate())), endISO: toISO(new Date(end.getUTCFullYear(), end.getUTCMonth(), end.getUTCDate())) };
}

const buildUniqueClosedPeriods = (entries: any[]): any[] => {
    if (!Array.isArray(entries)) return [];
    const mapped = entries
        .filter((entry: any) => entry && (entry.entry_date || entry.end_date))
        .map((entry: any) => {
            const rawEnd = String(entry.entry_date || entry.end_date);
            const endDateKey = rawEnd.slice(0, 10);
            const endDateObj = new Date(endDateKey);
            const formattedEndDate = endDateObj.toLocaleDateString('id-ID', {
                day: '2-digit',
                month: 'short',
                year: 'numeric'
            });
            const rawStart = entry.start_date ? String(entry.start_date).slice(0, 10) : undefined;
            return {
                value: endDateKey,
                label: `${formattedEndDate} - ${entry.description || 'Period Closing'}`,
                date: endDateKey,
                start_date: rawStart,
                fiscal_year: endDateObj.getFullYear(),
                entry_id: entry.id,
                source: entry.source,
            };
        });
    const uniqueMap = new Map<string, any>();
    mapped.forEach((period: any) => {
        const key = period.date;
        if (!uniqueMap.has(key)) {
            uniqueMap.set(key, period);
        }
    });
    return Array.from(uniqueMap.values()).sort((a, b) => new Date(b.date).getTime() - new Date(a.date).getTime());
};

const ProfitLossPage: React.FC = () => {
    const router = useRouter();
    const { t } = useTranslation();
    const toast = useToast();

    // Color mode values
    const cardBg = useColorModeValue('white', 'gray.800');
    const borderColor = useColorModeValue('gray.200', 'gray.600');
    const headingColor = useColorModeValue('gray.700', 'white');
    const textColor = useColorModeValue('gray.800', 'white');
    const descriptionColor = useColorModeValue('gray.600', 'gray.300');
    const summaryBg = useColorModeValue('gray.50', 'gray.700');

    // State
    const [startDate, setStartDate] = useState('');
    const [endDate, setEndDate] = useState('');
    const [data, setData] = useState<SSOTProfitLossData | null>(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [closedPeriods, setClosedPeriods] = useState<any[]>([]);
    const [loadingClosedPeriods, setLoadingClosedPeriods] = useState(false);

    // Transaction detail modal state
    const { isOpen: isDetailOpen, onOpen: onDetailOpen, onClose: onDetailClose } = useDisclosure();
    const [selectedAccount, setSelectedAccount] = useState<{ code: string; name: string } | null>(null);
    const [transactionDetails, setTransactionDetails] = useState<any[]>([]);
    const [loadingDetails, setLoadingDetails] = useState(false);

    // Calculation breakdown modal state
    const { isOpen: isBreakdownOpen, onOpen: onBreakdownOpen, onClose: onBreakdownClose } = useDisclosure();
    const [selectedCalcItem, setSelectedCalcItem] = useState<{ name: string; formula: string; components: any[] } | null>(null);

    // Load fiscal year settings on mount
    useEffect(() => {
        const loadSettings = async () => {
            try {
                const resp = await api.get(API_ENDPOINTS.SETTINGS);
                const fyStartStr: string = resp.data?.data?.fiscal_year_start || 'January 1';
                const range = computeFiscalRangeFromString(fyStartStr);
                setStartDate(range.startISO);
                setEndDate(range.endISO);
            } catch (e) {
                const today = new Date();
                const start = new Date(today.getFullYear(), 0, 1);
                const end = new Date(today.getFullYear(), 11, 31);
                setStartDate(toISO(start));
                setEndDate(toISO(end));
            }
        };
        loadSettings();
        loadClosedPeriods();
    }, []);

    const loadClosedPeriods = async () => {
        setLoadingClosedPeriods(true);
        try {
            const response = await api.get(API_ENDPOINTS.FISCAL_CLOSING.HISTORY);
            if (response.data.success && response.data.data && Array.isArray(response.data.data)) {
                const periods = buildUniqueClosedPeriods(response.data.data);
                setClosedPeriods(periods);
            }
        } catch (error) {
            console.error('Failed to load closed periods:', error);
        } finally {
            setLoadingClosedPeriods(false);
        }
    };

    const fetchReport = async () => {
        setLoading(true);
        setError(null);

        try {
            const result = await ssotProfitLossService.generateSSOTProfitLoss({
                start_date: startDate,
                end_date: endDate,
                format: 'json'
            });
            setData(result);
            toast({
                title: t('reports.messages.success'),
                description: t('reports.messages.profitLossGenerated'),
                status: 'success',
                duration: 3000,
                isClosable: true,
            });
        } catch (err: any) {
            const errorMessage = err.message || t('reports.messages.unknownError');
            setError(errorMessage);
            toast({
                title: t('reports.messages.error'),
                description: errorMessage,
                status: 'error',
                duration: 5000,
                isClosable: true,
            });
        } finally {
            setLoading(false);
        }
    };

    const handleExport = async (format: 'pdf' | 'csv') => {
        if (!data) {
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
                    start_date: startDate,
                    end_date: endDate,
                    format: 'pdf'
                });
                filename = `Profit_Loss_${startDate}_to_${endDate}.pdf`;
            } else {
                blob = await ssotProfitLossService.generateSSOTProfitLossCSV({
                    start_date: startDate,
                    end_date: endDate,
                    format: 'csv'
                });
                filename = `Profit_Loss_${startDate}_to_${endDate}.csv`;
            }

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
        } catch (err) {
            console.error(`Error exporting as ${format}:`, err);
            toast({
                title: t('reports.messages.error'),
                description: t('reports.messages.exportFailed', { format: format.toUpperCase() }),
                status: 'error',
                duration: 5000,
                isClosable: true,
            });
        }
    };

    // Handler for viewing transaction details
    const handleViewDetail = async (accountCode: string, accountName: string) => {
        if (!accountCode) {
            toast({
                title: t('common.info'),
                description: t('reports.modal.noAccountCode') || 'No account code available for this item',
                status: 'info',
                duration: 3000,
                isClosable: true,
            });
            return;
        }

        setSelectedAccount({ code: accountCode, name: accountName });
        setLoadingDetails(true);
        onDetailOpen();

        try {
            // Fetch journal entries for the selected date range
            const response = await ssotJournalService.getJournalEntries({
                start_date: startDate,
                end_date: endDate,
                status: 'posted',
                limit: 100,
            });

            // Filter entries that include the selected account code
            const filteredEntries: any[] = [];
            response.data.forEach((entry: any) => {
                if (entry.lines && Array.isArray(entry.lines)) {
                    entry.lines.forEach((line: any) => {
                        if (line.account?.code === accountCode || line.account_code === accountCode) {
                            filteredEntries.push({
                                entry_number: entry.entry_number,
                                entry_date: entry.entry_date,
                                description: line.description || entry.description,
                                debit_amount: line.debit_amount || 0,
                                credit_amount: line.credit_amount || 0,
                                source_type: entry.source_type,
                            });
                        }
                    });
                }
            });

            setTransactionDetails(filteredEntries);
        } catch (err) {
            console.error('Error fetching transaction details:', err);
            toast({
                title: t('reports.messages.error'),
                description: t('reports.modal.failedToLoadDetails') || 'Failed to load transaction details',
                status: 'error',
                duration: 5000,
                isClosable: true,
            });
            setTransactionDetails([]);
        } finally {
            setLoadingDetails(false);
        }
    };

    // Handler for showing calculation breakdown
    const handleShowBreakdown = (itemName: string) => {
        if (!data || !data.sections) return;

        let formula = '';
        let components: any[] = [];

        const findSectionByKeyword = (keyword: string) => {
            return data.sections?.find((s: any) =>
                s.name.toLowerCase().includes(keyword.toLowerCase())
            );
        };

        const getSectionItems = (section: any, detailsFallback?: any[]) => {
            // First try section items
            if (section && section.items && section.items.length > 0) {
                const items = section.items.filter((item: any) => !item.is_percentage);
                // If items have account_code, use them
                if (items.some((item: any) => item.account_code)) {
                    return items;
                }
            }
            // Fallback to revenue_details or expense_details
            if (detailsFallback && detailsFallback.length > 0) {
                return detailsFallback.map((item: any) => ({
                    account_code: item.account_id ? String(item.account_id) : '-',
                    name: item.account_name || item.name,
                    amount: item.amount || 0
                }));
            }
            // Return section items without account codes if that's all we have
            if (section && section.items) {
                return section.items.filter((item: any) => !item.is_percentage);
            }
            return [];
        };

        // Build formula and components based on item name
        if (itemName.toLowerCase().includes('laba kotor') || itemName.toLowerCase().includes('gross profit')) {
            const revenueSection = findSectionByKeyword('pendapatan') || findSectionByKeyword('revenue');
            const cogsSection = findSectionByKeyword('hpp') || findSectionByKeyword('cogs') || findSectionByKeyword('harga pokok');
            formula = 'Total Pendapatan - Harga Pokok Penjualan (HPP)';
            components = [
                { type: 'add', label: 'Total Pendapatan', items: getSectionItems(revenueSection, data.revenue_details), total: revenueSection?.total || data.total_revenue || 0 },
                { type: 'subtract', label: 'Harga Pokok Penjualan', items: getSectionItems(cogsSection), total: cogsSection?.total || 0 },
            ];
        } else if (itemName.toLowerCase().includes('laba operasional') || itemName.toLowerCase().includes('operating')) {
            const grossProfitSection = findSectionByKeyword('laba kotor') || findSectionByKeyword('gross profit');
            const opexSection = findSectionByKeyword('operasional') || findSectionByKeyword('operating') || findSectionByKeyword('beban usaha');
            formula = 'Laba Kotor - Beban Operasional';
            components = [
                { type: 'add', label: 'Laba Kotor', items: getSectionItems(grossProfitSection, data.revenue_details), total: grossProfitSection?.total || data.financialMetrics?.grossProfit || 0 },
                { type: 'subtract', label: 'Beban Operasional', items: getSectionItems(opexSection, data.expense_details), total: opexSection?.total || data.total_expenses || 0 },
            ];
        } else if (itemName.toLowerCase().includes('laba bersih') || itemName.toLowerCase().includes('net income')) {
            const netIncomeSection = findSectionByKeyword('laba bersih') || findSectionByKeyword('net income');
            formula = 'Laba Sebelum Pajak - Beban Pajak';
            components = [
                { type: 'result', label: 'Laba Bersih', items: getSectionItems(netIncomeSection), total: data.financialMetrics?.netIncome || 0 },
            ];
        } else if (itemName.toLowerCase().includes('laba sebelum pajak') || itemName.toLowerCase().includes('before tax')) {
            formula = 'Laba Operasional + Pendapatan Lainnya - Beban Lainnya';
            components = [
                { type: 'result', label: 'Laba Sebelum Pajak', items: [], total: (data as any)?.summary?.incomeBeforeTax || data.financialMetrics?.operatingIncome || 0 },
            ];
        }

        setSelectedCalcItem({ name: itemName, formula, components });
        onBreakdownOpen();
    };

    const formatCurrency = (amount: number | null | undefined) => {
        if (amount === null || amount === undefined || isNaN(Number(amount))) return 'Rp 0';
        return new Intl.NumberFormat('id-ID', {
            style: 'currency',
            currency: 'IDR',
            minimumFractionDigits: 0,
            maximumFractionDigits: 0
        }).format(Number(amount));
    };

    return (
        <UnifiedLayout>
            <VStack spacing={6} align="stretch">
                {/* Header */}
                <HStack spacing={4}>
                    <IconButton
                        icon={<FiArrowLeft />}
                        variant="outline"
                        onClick={() => router.push('/reports')}
                        aria-label={t('common.back')}
                        size="md"
                    />
                    <VStack align="start" spacing={0}>
                        <Heading as="h1" size="lg">
                            <HStack>
                                <Icon as={FiTrendingUp} color="purple.500" />
                                <Text>{t('reports.profitLossStatement')}</Text>
                            </HStack>
                        </Heading>
                        <Text fontSize="sm" color={descriptionColor}>
                            {t('reports.description.profitLoss')}
                        </Text>
                    </VStack>
                </HStack>

                {/* Filter Controls */}
                <Card bg={cardBg} borderColor={borderColor} borderWidth="1px">
                    <CardBody>
                        <VStack spacing={4} align="stretch">
                            {/* Closed Period Selector */}
                            <HStack spacing={4} alignItems="flex-end" flexWrap="wrap">
                                <FormControl flex="1" minW="250px">
                                    <FormLabel fontSize="sm">
                                        <HStack spacing={2}>
                                            <Icon as={FiCalendar} />
                                            <Text>{t('reports.closedPeriod')}</Text>
                                            <Tooltip label={t('reports.modal.selectClosedPeriodTooltip')} placement="top">
                                                <Box><Icon as={FiInfo} color="gray.500" /></Box>
                                            </Tooltip>
                                        </HStack>
                                    </FormLabel>
                                    <Select
                                        placeholder={t('reports.selectClosedPeriod')}
                                        size="md"
                                        onChange={(e) => {
                                            if (e.target.value) {
                                                const selectedPeriod = closedPeriods.find(p => p.date === e.target.value);
                                                if (selectedPeriod) {
                                                    if (selectedPeriod.start_date) {
                                                        setStartDate(selectedPeriod.start_date);
                                                    } else {
                                                        const periodEndDate = new Date(selectedPeriod.date);
                                                        const fiscalYearStart = new Date(periodEndDate.getFullYear(), 0, 1);
                                                        setStartDate(fiscalYearStart.toISOString().split('T')[0]);
                                                    }
                                                    setEndDate(selectedPeriod.date);
                                                }
                                            }
                                        }}
                                        isDisabled={loadingClosedPeriods}
                                    >
                                        {closedPeriods.map((period, index) => (
                                            <option key={`${period.date}-${index}`} value={period.date}>
                                                {period.label}
                                            </option>
                                        ))}
                                    </Select>
                                </FormControl>
                                <Button
                                    size="md"
                                    variant="outline"
                                    colorScheme="blue"
                                    onClick={loadClosedPeriods}
                                    isLoading={loadingClosedPeriods}
                                    leftIcon={<FiClock />}
                                >
                                    {t('reports.modal.loadHistory')}
                                </Button>
                            </HStack>

                            <Divider />

                            {/* Date Range */}
                            <HStack spacing={4} flexWrap="wrap">
                                <FormControl flex="1" minW="200px">
                                    <FormLabel>{t('reports.modal.startDate')}</FormLabel>
                                    <Input
                                        type="date"
                                        value={startDate}
                                        onChange={(e) => setStartDate(e.target.value)}
                                    />
                                </FormControl>
                                <FormControl flex="1" minW="200px">
                                    <FormLabel>{t('reports.modal.endDate')}</FormLabel>
                                    <Input
                                        type="date"
                                        value={endDate}
                                        onChange={(e) => setEndDate(e.target.value)}
                                    />
                                </FormControl>
                                <Button
                                    colorScheme="blue"
                                    onClick={fetchReport}
                                    isLoading={loading}
                                    leftIcon={<FiTrendingUp />}
                                    size="md"
                                    mt={8}
                                >
                                    {t('reports.modal.generateReport')}
                                </Button>
                            </HStack>
                        </VStack>
                    </CardBody>
                </Card>

                {/* Loading State */}
                {loading && (
                    <Box textAlign="center" py={8}>
                        <VStack spacing={4}>
                            <Spinner size="xl" thickness="4px" speed="0.65s" color="purple.500" />
                            <Text fontSize="lg" fontWeight="medium">{t('reports.modal.generatingProfitLoss')}</Text>
                            <Text fontSize="sm" color={descriptionColor}>{t('reports.modal.analyzingRevenueExpenses')}</Text>
                        </VStack>
                    </Box>
                )}

                {/* Error State */}
                {error && (
                    <Box bg="red.50" p={4} borderRadius="md">
                        <Text color="red.600" fontWeight="medium">Error: {error}</Text>
                        <Button mt={2} size="sm" colorScheme="red" variant="outline" onClick={fetchReport}>
                            {t('reports.modal.retry')}
                        </Button>
                    </Box>
                )}

                {/* Report Data */}
                {data && !loading && (
                    <VStack spacing={6} align="stretch">
                        {/* Company Header */}
                        {data.company && (
                            <Box bg="purple.50" p={4} borderRadius="md">
                                <HStack justify="space-between" align="start">
                                    <VStack align="start" spacing={1}>
                                        <Text fontSize="lg" fontWeight="bold" color="purple.800">
                                            {data.company.name || t('common.name')}
                                        </Text>
                                        <Text fontSize="sm" color="purple.600">
                                            {data.company.address || ''} {data.company.city ? `, ${data.company.city}` : ''}
                                        </Text>
                                    </VStack>
                                    <VStack align="end" spacing={1}>
                                        <Text fontSize="sm" color="purple.600">
                                            {t('reports.modal.currency')}: {data.currency || 'IDR'}
                                        </Text>
                                    </VStack>
                                </HStack>
                            </Box>
                        )}

                        {/* Report Title */}
                        <Box textAlign="center" bg={summaryBg} p={4} borderRadius="md">
                            <Heading size="md" color={headingColor}>
                                {data.title || t('reports.profitLossStatement')}
                            </Heading>
                            <Text fontSize="sm" color={descriptionColor}>
                                {t('reports.modal.period')}: {data.period || `${startDate} - ${endDate}`}
                            </Text>
                        </Box>

                        {/* Summary Statistics */}
                        <SimpleGrid columns={[1, 2, 4]} spacing={4}>
                            <Box bg="purple.50" p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="purple.600">
                                    {formatCurrency(data.total_revenue || 0)}
                                </Text>
                                <Text fontSize="sm" color="purple.800">{t('reports.modal.totalRevenue')}</Text>
                            </Box>
                            <Box bg="blue.50" p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="blue.600">
                                    {formatCurrency(data.total_expenses || 0)}
                                </Text>
                                <Text fontSize="sm" color="blue.800">{t('reports.modal.totalExpenses')}</Text>
                            </Box>
                            <Box bg="orange.50" p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="orange.600">
                                    {formatCurrency(data.net_profit || 0)}
                                </Text>
                                <Text fontSize="sm" color="orange.800">{t('reports.modal.netProfit')}</Text>
                            </Box>
                            <Box bg="green.50" p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="green.600">
                                    {formatCurrency(data.net_loss || 0)}
                                </Text>
                                <Text fontSize="sm" color="green.800">{t('reports.modal.netLoss')}</Text>
                            </Box>
                        </SimpleGrid>

                        {/* Revenue Details */}
                        {data.revenue_details && data.revenue_details.length > 0 && (
                            <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                <Text fontSize="md" fontWeight="bold" color={headingColor} mb={3}>
                                    {t('reports.modal.revenueDetails')}
                                </Text>
                                <SimpleGrid columns={[1, 2, 3]} spacing={4}>
                                    {data.revenue_details.map((revenue: any) => (
                                        <Box key={revenue.account_id} bg="white" p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                            <VStack align="start" spacing={0}>
                                                <Text fontSize="sm" color="gray.700">{revenue.account_name || t('accounts.title')}</Text>
                                                <Text fontSize="md" fontWeight="bold" color="purple.600">{formatCurrency(revenue.amount || 0)}</Text>
                                            </VStack>
                                        </Box>
                                    ))}
                                </SimpleGrid>
                            </Box>
                        )}

                        {/* Expense Details */}
                        {data.expense_details && data.expense_details.length > 0 && (
                            <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                <Text fontSize="md" fontWeight="bold" color={headingColor} mb={3}>
                                    {t('reports.modal.expenseDetails')}
                                </Text>
                                <SimpleGrid columns={[1, 2, 3]} spacing={4}>
                                    {data.expense_details.map((expense: any) => (
                                        <Box key={expense.account_id} bg="white" p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                            <VStack align="start" spacing={0}>
                                                <Text fontSize="sm" color="gray.700">{expense.account_name || t('accounts.title')}</Text>
                                                <Text fontSize="md" fontWeight="bold" color="blue.600">{formatCurrency(expense.amount || 0)}</Text>
                                            </VStack>
                                        </Box>
                                    ))}
                                </SimpleGrid>
                            </Box>
                        )}

                        {/* Financial Metrics */}
                        {data.financialMetrics && (
                            <Box bg="green.50" p={4} borderRadius="md" border="1px" borderColor="green.200">
                                <Text fontSize="md" fontWeight="bold" color="green.800" mb={3}>
                                    {t('reports.modal.keyFinancialMetrics')}
                                </Text>
                                <SimpleGrid columns={[1, 2]} spacing={3}>
                                    <VStack spacing={2}>
                                        <HStack justify="space-between" w="full">
                                            <Text fontSize="sm" color="green.700">{t('reports.modal.grossProfit')}:</Text>
                                            <Text fontSize="sm" fontWeight="semibold">{formatCurrency(data.financialMetrics.grossProfit || 0)}</Text>
                                        </HStack>
                                        <HStack justify="space-between" w="full">
                                            <Text fontSize="sm" color="green.700">{t('reports.modal.grossMargin')}:</Text>
                                            <Text fontSize="sm" fontWeight="semibold">{(data.financialMetrics.grossProfitMargin || 0).toFixed(1)}%</Text>
                                        </HStack>
                                        <HStack justify="space-between" w="full">
                                            <Text fontSize="sm" color="green.700">{t('reports.modal.operatingIncome')}:</Text>
                                            <Text fontSize="sm" fontWeight="semibold">{formatCurrency(data.financialMetrics.operatingIncome || 0)}</Text>
                                        </HStack>
                                    </VStack>
                                    <VStack spacing={2}>
                                        <HStack justify="space-between" w="full">
                                            <Text fontSize="sm" color="green.700">{t('reports.modal.operatingMargin')}:</Text>
                                            <Text fontSize="sm" fontWeight="semibold">{(data.financialMetrics.operatingMargin || 0).toFixed(1)}%</Text>
                                        </HStack>
                                        <HStack justify="space-between" w="full">
                                            <Text fontSize="sm" color="green.700">{t('reports.modal.netIncome')}:</Text>
                                            <Text fontSize="sm" fontWeight="semibold">{formatCurrency(data.financialMetrics.netIncome || 0)}</Text>
                                        </HStack>
                                        <HStack justify="space-between" w="full">
                                            <Text fontSize="sm" color="green.700">{t('reports.modal.netMargin')}:</Text>
                                            <Text fontSize="sm" fontWeight="semibold">{(data.financialMetrics.netIncomeMargin || 0).toFixed(1)}%</Text>
                                        </HStack>
                                    </VStack>
                                </SimpleGrid>
                            </Box>
                        )}

                        {/* Sections Details */}
                        {data.sections && data.sections.length > 0 && (
                            <VStack spacing={4} align="stretch">
                                <Text fontSize="lg" fontWeight="bold" color={headingColor}>
                                    {t('reports.modal.detailedBreakdown')}
                                </Text>
                                {data.sections.map((section: any, index: number) => (
                                    <Box key={index} bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                        <HStack justify="space-between" mb={2}>
                                            <Text fontSize="md" fontWeight="bold" color={headingColor}>{section.name}</Text>
                                            <Text fontSize="md" fontWeight="bold" color={section.total >= 0 ? 'green.600' : 'red.600'}>
                                                {formatCurrency(section.total || 0)}
                                            </Text>
                                        </HStack>
                                        {section.items && section.items.length > 0 && (
                                            <VStack spacing={1} align="stretch">
                                                {section.items.map((item: any, itemIndex: number) => (
                                                    <HStack key={itemIndex} justify="space-between" pl={4} py={1} _hover={{ bg: 'gray.50' }} borderRadius="md">
                                                        <Text fontSize="sm" color={descriptionColor} flex={1}>
                                                            {item.name}
                                                        </Text>
                                                        <Text fontSize="sm" fontWeight="medium" color={textColor} minW="120px" textAlign="right">
                                                            {item.is_percentage ? `${item.amount?.toFixed(1)}%` : formatCurrency(item.amount || 0)}
                                                        </Text>
                                                        <Box minW="60px" textAlign="center">
                                                            {!item.is_percentage && (item.name.toLowerCase().includes('laba') || item.name.toLowerCase().includes('profit')) && (
                                                                <Tooltip label="Lihat Rincian Perhitungan">
                                                                    <IconButton
                                                                        aria-label="View Calculation"
                                                                        icon={<FiEye />}
                                                                        size="xs"
                                                                        colorScheme="blue"
                                                                        variant="ghost"
                                                                        onClick={() => handleShowBreakdown(item.name)}
                                                                    />
                                                                </Tooltip>
                                                            )}
                                                        </Box>
                                                    </HStack>
                                                ))}
                                            </VStack>
                                        )}
                                    </Box>
                                ))}
                            </VStack>
                        )}

                        {/* Export Buttons */}
                        <HStack spacing={3} justify="flex-end">
                            <Button
                                colorScheme="red"
                                variant="outline"
                                leftIcon={<FiFilePlus />}
                                onClick={() => handleExport('pdf')}
                            >
                                {t('reports.exportPDF')}
                            </Button>
                            <Button
                                colorScheme="green"
                                variant="outline"
                                leftIcon={<FiFileText />}
                                onClick={() => handleExport('csv')}
                            >
                                {t('reports.exportCSV')}
                            </Button>
                        </HStack>
                    </VStack>
                )}
            </VStack>

            {/* Transaction Detail Modal */}
            <Modal isOpen={isDetailOpen} onClose={onDetailClose} size="xl" scrollBehavior="inside">
                <ModalOverlay />
                <ModalContent maxW="800px">
                    <ModalHeader>
                        <VStack align="start" spacing={1}>
                            <Text>{t('reports.modal.transactionDetails') || 'Transaction Details'}</Text>
                            {selectedAccount && (
                                <HStack>
                                    <Badge colorScheme="purple">{selectedAccount.code}</Badge>
                                    <Text fontSize="sm" color="gray.600">{selectedAccount.name}</Text>
                                </HStack>
                            )}
                        </VStack>
                    </ModalHeader>
                    <ModalCloseButton />
                    <ModalBody>
                        {loadingDetails ? (
                            <VStack py={8}>
                                <Spinner size="lg" color="blue.500" />
                                <Text color="gray.500">{t('common.loading') || 'Loading...'}</Text>
                            </VStack>
                        ) : transactionDetails.length > 0 ? (
                            <TableContainer>
                                <Table size="sm" variant="striped">
                                    <Thead>
                                        <Tr>
                                            <Th>{t('journal.entryNumber') || 'No. Jurnal'}</Th>
                                            <Th>{t('common.date') || 'Tanggal'}</Th>
                                            <Th>{t('common.description') || 'Keterangan'}</Th>
                                            <Th isNumeric>{t('journal.debit') || 'Debit'}</Th>
                                            <Th isNumeric>{t('journal.credit') || 'Kredit'}</Th>
                                        </Tr>
                                    </Thead>
                                    <Tbody>
                                        {transactionDetails.map((tx: any, idx: number) => (
                                            <Tr key={idx}>
                                                <Td>
                                                    <Badge colorScheme="blue" fontSize="xs">
                                                        {tx.entry_number}
                                                    </Badge>
                                                </Td>
                                                <Td fontSize="sm">
                                                    {new Date(tx.entry_date).toLocaleDateString('id-ID')}
                                                </Td>
                                                <Td fontSize="sm" maxW="200px" isTruncated>
                                                    {tx.description}
                                                </Td>
                                                <Td isNumeric fontSize="sm" color={tx.debit_amount > 0 ? 'green.600' : 'gray.400'}>
                                                    {tx.debit_amount > 0 ? formatCurrency(tx.debit_amount) : '-'}
                                                </Td>
                                                <Td isNumeric fontSize="sm" color={tx.credit_amount > 0 ? 'red.600' : 'gray.400'}>
                                                    {tx.credit_amount > 0 ? formatCurrency(tx.credit_amount) : '-'}
                                                </Td>
                                            </Tr>
                                        ))}
                                    </Tbody>
                                </Table>
                            </TableContainer>
                        ) : (
                            <VStack py={8}>
                                <Text color="gray.500">{t('reports.modal.noTransactionsFound') || 'No transactions found for this account in the selected period'}</Text>
                            </VStack>
                        )}
                    </ModalBody>
                    <ModalFooter>
                        <Button colorScheme="blue" onClick={onDetailClose}>
                            {t('common.close') || 'Close'}
                        </Button>
                    </ModalFooter>
                </ModalContent>
            </Modal>

            {/* Calculation Breakdown Modal */}
            <Modal isOpen={isBreakdownOpen} onClose={onBreakdownClose} size="xl" scrollBehavior="inside">
                <ModalOverlay />
                <ModalContent maxW="700px">
                    <ModalHeader>
                        <VStack align="start" spacing={1}>
                            <Text>Rincian Perhitungan</Text>
                            {selectedCalcItem && (
                                <Badge colorScheme="purple" fontSize="sm">{selectedCalcItem.name}</Badge>
                            )}
                        </VStack>
                    </ModalHeader>
                    <ModalCloseButton />
                    <ModalBody>
                        {selectedCalcItem && (
                            <VStack spacing={4} align="stretch">
                                {/* Formula */}
                                <Box bg="blue.50" p={3} borderRadius="md" border="1px" borderColor="blue.200">
                                    <Text fontSize="sm" color="blue.700" fontWeight="semibold">
                                        Formula: {selectedCalcItem.formula}
                                    </Text>
                                </Box>

                                {/* Components */}
                                {selectedCalcItem.components.map((component: any, idx: number) => (
                                    <Box key={idx} p={3} borderRadius="md" border="1px" borderColor={borderColor}>
                                        <HStack justify="space-between" mb={2}>
                                            <HStack>
                                                <Badge colorScheme={component.type === 'add' ? 'green' : component.type === 'subtract' ? 'red' : 'blue'}>
                                                    {component.type === 'add' ? '+' : component.type === 'subtract' ? '-' : '='}
                                                </Badge>
                                                <Text fontWeight="semibold">{component.label}</Text>
                                            </HStack>
                                            <Text fontWeight="bold" color={component.total >= 0 ? 'green.600' : 'red.600'}>
                                                {formatCurrency(component.total)}
                                            </Text>
                                        </HStack>
                                        {component.items && component.items.length > 0 && (
                                            <TableContainer mt={2}>
                                                <Table size="sm" variant="simple">
                                                    <Thead bg="gray.50">
                                                        <Tr>
                                                            <Th width="120px">Kode Akun</Th>
                                                            <Th>Nama Akun</Th>
                                                            <Th isNumeric width="150px">Jumlah</Th>
                                                        </Tr>
                                                    </Thead>
                                                    <Tbody>
                                                        {component.items.map((item: any, itemIdx: number) => (
                                                            <Tr key={itemIdx} _hover={{ bg: 'gray.50' }}>
                                                                <Td>
                                                                    <Badge colorScheme="purple" fontSize="xs">
                                                                        {item.account_code || '-'}
                                                                    </Badge>
                                                                </Td>
                                                                <Td>
                                                                    <Text fontSize="sm" color={descriptionColor}>
                                                                        {item.name}
                                                                    </Text>
                                                                </Td>
                                                                <Td isNumeric>
                                                                    <Text fontSize="sm" fontWeight="medium" color={textColor}>
                                                                        {formatCurrency(item.amount || 0)}
                                                                    </Text>
                                                                </Td>
                                                            </Tr>
                                                        ))}
                                                    </Tbody>
                                                </Table>
                                            </TableContainer>
                                        )}
                                        {(!component.items || component.items.length === 0) && (
                                            <Text fontSize="sm" color="gray.500" pl={4} fontStyle="italic">
                                                Tidak ada akun detail tersedia
                                            </Text>
                                        )}
                                    </Box>
                                ))}

                                {selectedCalcItem.components.length === 0 && (
                                    <Text color="gray.500" textAlign="center">
                                        Tidak ada rincian tersedia untuk item ini
                                    </Text>
                                )}
                            </VStack>
                        )}
                    </ModalBody>
                    <ModalFooter>
                        <Button colorScheme="blue" onClick={onBreakdownClose}>
                            Tutup
                        </Button>
                    </ModalFooter>
                </ModalContent>
            </Modal>
        </UnifiedLayout>
    );
};

export default ProfitLossPage;
