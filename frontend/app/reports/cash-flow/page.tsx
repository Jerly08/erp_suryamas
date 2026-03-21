'use client';

import React, { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useTranslation } from '@/hooks/useTranslation';
import UnifiedLayout from '@/components/layout/UnifiedLayout';
import { ssotCashFlowReportService, SSOTCashFlowData } from '@/services/ssotCashFlowReportService';
import api from '@/services/api';
import { API_ENDPOINTS } from '@/config/api';
import periodClosingService from '@/services/periodClosingService';
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
    Badge,
    Modal,
    ModalOverlay,
    ModalContent,
    ModalHeader,
    ModalBody,
    ModalCloseButton,
    ModalFooter,
    useDisclosure,
    Table,
    Thead,
    Tbody,
    Tr,
    Th,
    Td,
} from '@chakra-ui/react';
import {
    FiArrowLeft,
    FiTrendingUp,
    FiFilePlus,
    FiFileText,
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

const CashFlowPage: React.FC = () => {
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
    const inputBg = useColorModeValue('white', 'whiteAlpha.50');
    const inputBorderColor = useColorModeValue('gray.300', 'whiteAlpha.300');
    const inputHoverBorderColor = useColorModeValue('gray.400', 'whiteAlpha.400');
    const inputFocusBorderColor = useColorModeValue('blue.500', 'blue.300');
    const infoIconColor = useColorModeValue('gray.500', 'gray.400');
    const errorBg = useColorModeValue('red.50', 'rgba(229, 62, 62, 0.14)');
    const errorBorderColor = useColorModeValue('red.200', 'red.700');
    const companyHeaderBg = useColorModeValue('purple.50', 'rgba(128, 90, 213, 0.14)');
    const companyHeaderTextColor = useColorModeValue('purple.800', 'purple.100');
    const companyHeaderSubtextColor = useColorModeValue('purple.600', 'purple.200');
    const statPurpleBg = useColorModeValue('purple.50', 'rgba(128, 90, 213, 0.14)');
    const statBlueBg = useColorModeValue('blue.50', 'rgba(66, 153, 225, 0.14)');
    const statOrangeBg = useColorModeValue('orange.50', 'rgba(221, 107, 32, 0.14)');
    const statGreenBg = useColorModeValue('green.50', 'rgba(72, 187, 120, 0.14)');
    const hoverBg = useColorModeValue('gray.50', 'whiteAlpha.100');
    const dividerBorderColor = useColorModeValue('gray.100', 'whiteAlpha.100');
    const summaryCashBg = useColorModeValue('green.50', 'rgba(72, 187, 120, 0.14)');
    const summaryCashBorder = useColorModeValue('green.200', 'green.700');
    const summaryCashText = useColorModeValue('green.800', 'green.100');
    const ratioBg = useColorModeValue('blue.50', 'rgba(66, 153, 225, 0.14)');
    const ratioBorder = useColorModeValue('blue.200', 'blue.700');
    const ratioText = useColorModeValue('blue.800', 'blue.100');
    const modalBg = useColorModeValue('white', 'gray.800');
    const modalHeaderBg = useColorModeValue('gray.50', 'whiteAlpha.100');
    const modalBorderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
    const infoPanelBg = useColorModeValue('blue.50', 'rgba(66, 153, 225, 0.14)');
    const infoPanelBorderColor = useColorModeValue('blue.200', 'blue.700');
    const infoPanelTextColor = useColorModeValue('blue.700', 'blue.100');
    const sectionHeaderBg = useColorModeValue('gray.50', 'whiteAlpha.100');
    const subtleTextColor = useColorModeValue('gray.500', 'gray.400');
    const notePanelBg = useColorModeValue('yellow.50', 'rgba(214, 158, 46, 0.14)');
    const notePanelBorderColor = useColorModeValue('yellow.200', 'yellow.700');
    const notePanelTextColor = useColorModeValue('yellow.800', 'yellow.100');
    const fieldStyles = {
        bg: inputBg,
        color: textColor,
        borderColor: inputBorderColor,
        _hover: { borderColor: inputHoverBorderColor },
        _focusVisible: {
            borderColor: inputFocusBorderColor,
            boxShadow: `0 0 0 1px ${inputFocusBorderColor}`,
        },
    };

    // State
    const [startDate, setStartDate] = useState('');
    const [endDate, setEndDate] = useState('');
    const [data, setData] = useState<SSOTCashFlowData | null>(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [closedPeriods, setClosedPeriods] = useState<any[]>([]);
    const [loadingClosedPeriods, setLoadingClosedPeriods] = useState(false);
    const [companyInfo, setCompanyInfo] = useState<{ name?: string; address?: string; phone?: string; email?: string; website?: string; taxNumber?: string; logoPath?: string } | null>(null);

    // Breakdown modal state
    const { isOpen: isBreakdownOpen, onOpen: onBreakdownOpen, onClose: onBreakdownClose } = useDisclosure();
    const [breakdownData, setBreakdownData] = useState<{
        title: string;
        formula?: string;
        components: Array<{
            label: string;
            description?: string;
            items: Array<{ account_code: string; account_name: string; amount: number }>;
            total: number;
        }>;
    } | null>(null);

    // Load fiscal year settings & company info on mount
    useEffect(() => {
        const loadSettings = async () => {
            try {
                const resp = await api.get(API_ENDPOINTS.SETTINGS);
                const settingsData = resp.data?.data;
                const fyStartStr: string = settingsData?.fiscal_year_start || 'January 1';
                const range = computeFiscalRangeFromString(fyStartStr);
                setStartDate(range.startISO);
                setEndDate(range.endISO);

                // Extract company info from settings
                if (settingsData?.company_name || settingsData?.company_address) {
                    setCompanyInfo({
                        name: settingsData.company_name,
                        address: [settingsData.company_address, settingsData.company_city].filter(Boolean).join(', '),
                        phone: settingsData.company_phone,
                        email: settingsData.company_email,
                        website: settingsData.company_website,
                        taxNumber: settingsData.tax_number,
                        logoPath: settingsData.company_logo,
                    });
                }
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
            const periods = await periodClosingService.getClosedPeriodsForFilter();
            setClosedPeriods(periods.map((period) => ({
                value: period.value,
                label: period.label,
                date: period.value,
                start_date: period.period.start_date?.slice(0, 10),
            })));
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
            const result = await ssotCashFlowReportService.generateSSOTCashFlow({
                start_date: startDate,
                end_date: endDate,
            });

            if (result instanceof Blob) {
                toast({ title: 'Report downloaded', status: 'success', duration: 3000, isClosable: true });
                return;
            }

            setData(result);

            // Update company info from API response if available
            if (result.company) {
                const cmp = result.company as any;
                setCompanyInfo(prev => ({
                    name: cmp.name || prev?.name,
                    address: cmp.address || prev?.address,
                    phone: prev?.phone,
                    email: prev?.email,
                    website: prev?.website,
                    taxNumber: prev?.taxNumber,
                    logoPath: prev?.logoPath,
                }));
            }

            toast({
                title: t('reports.messages.success') || 'Laporan Berhasil',
                description: 'Laporan arus kas berhasil dimuat',
                status: 'success',
                duration: 3000,
                isClosable: true,
            });
        } catch (err: any) {
            const errorMessage = err.message || t('reports.messages.unknownError') || 'Terjadi kesalahan';
            setError(errorMessage);
            toast({
                title: t('reports.messages.error') || 'Gagal memuat laporan',
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
        if (!startDate || !endDate) {
            toast({
                title: t('reports.messages.error') || 'Error',
                description: 'Pilih periode laporan terlebih dahulu.',
                status: 'error',
                duration: 3000,
                isClosable: true,
            });
            return;
        }

        try {
            const result = await ssotCashFlowReportService.generateSSOTCashFlow({
                start_date: startDate,
                end_date: endDate,
                format,
            });

            if (!(result instanceof Blob)) {
                throw new Error(`Server did not return a ${format.toUpperCase()} file`);
            }

            const extension = format === 'pdf' ? 'pdf' : 'csv';
            await ssotCashFlowReportService.downloadCashFlowReport(
                result,
                `Arus_Kas_${startDate}_to_${endDate}.${extension}`
            );

            toast({
                title: t('reports.messages.success') || 'Ekspor Berhasil',
                description: t('reports.messages.exportCompleted', { format: format.toUpperCase() }) || `File ${format.toUpperCase()} berhasil diunduh`,
                status: 'success',
                duration: 3000,
                isClosable: true,
            });
        } catch (err: any) {
            console.error(`Error exporting as ${format}:`, err);
            toast({
                title: t('reports.messages.error') || 'Gagal ekspor',
                description: t('reports.messages.exportFailed', { format: format.toUpperCase() }) || `Gagal mengekspor sebagai ${format.toUpperCase()}`,
                status: 'error',
                duration: 5000,
                isClosable: true,
            });
        }
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

    type AmountVariant = 'cashflow' | 'performance';

    const getAmountMeta = (amount: number | null | undefined, variant: AmountVariant = 'cashflow') => {
        const value = Number(amount || 0);
        const isPositive = value > 0;
        const isNegative = value < 0;

        if (variant === 'performance') {
            return {
                value,
                absoluteValue: Math.abs(value),
                badgeLabel: isPositive ? 'Laba' : isNegative ? 'Rugi' : 'Impas',
                badgeScheme: isPositive ? 'green' : isNegative ? 'red' : 'gray',
                textColor: isPositive ? 'green.600' : isNegative ? 'red.500' : subtleTextColor,
            };
        }

        return {
            value,
            absoluteValue: Math.abs(value),
            badgeLabel: isPositive ? 'Kas Masuk' : isNegative ? 'Kas Keluar' : 'Nihil',
            badgeScheme: isPositive ? 'green' : isNegative ? 'red' : 'gray',
            textColor: isPositive ? 'green.600' : isNegative ? 'red.500' : subtleTextColor,
        };
    };

    const renderAmountInline = (
        amount: number | null | undefined,
        options?: {
            variant?: AmountVariant;
            showBadge?: boolean;
            fontSize?: string;
            fontWeight?: string;
            justify?: string;
            minW?: string;
            textAlign?: 'left' | 'center' | 'right';
            badgeSize?: string;
        }
    ) => {
        const meta = getAmountMeta(amount, options?.variant || 'cashflow');
        return (
            <HStack
                spacing={2}
                justify={options?.justify || 'flex-end'}
                flexWrap="wrap"
                minW={options?.minW}
            >
                {options?.showBadge !== false && (
                    <Badge colorScheme={meta.badgeScheme} fontSize={options?.badgeSize || '0.65rem'} px={2} py={0.5} borderRadius="full">
                        {meta.badgeLabel}
                    </Badge>
                )}
                <Text
                    fontSize={options?.fontSize || 'sm'}
                    fontWeight={options?.fontWeight || 'semibold'}
                    color={meta.textColor}
                    textAlign={options?.textAlign || 'right'}
                >
                    {formatCurrency(meta.absoluteValue)}
                </Text>
            </HStack>
        );
    };

    const renderAmountStack = (
        amount: number | null | undefined,
        options?: {
            variant?: AmountVariant;
            showBadge?: boolean;
            amountFontSize?: string;
            amountFontWeight?: string;
        }
    ) => {
        const meta = getAmountMeta(amount, options?.variant || 'cashflow');
        return (
            <VStack spacing={1} align="center">
                {options?.showBadge !== false && (
                    <Badge colorScheme={meta.badgeScheme} fontSize="0.65rem" px={2} py={0.5} borderRadius="full">
                        {meta.badgeLabel}
                    </Badge>
                )}
                <Text
                    fontSize={options?.amountFontSize || '2xl'}
                    fontWeight={options?.amountFontWeight || 'bold'}
                    color={meta.textColor}
                >
                    {formatCurrency(meta.absoluteValue)}
                </Text>
            </VStack>
        );
    };

    const formatAmountNarrative = (amount: number | null | undefined, variant: AmountVariant = 'cashflow') => {
        const meta = getAmountMeta(amount, variant);
        return `${meta.badgeLabel} ${formatCurrency(meta.absoluteValue)}`;
    };

    const op = data?.operating_activities;
    const inv = data?.investing_activities;
    const fin = data?.financing_activities;

    // Handler for showing section breakdown
    const handleShowBreakdown = (section: 'operating' | 'investing' | 'financing' | 'summary') => {
        if (!data) return;

        if (section === 'operating') {
            const components: any[] = [];
            if (op?.net_income) {
                components.push({
                    label: 'Laba Bersih',
                    description: 'Nilai ini merupakan kalkulasi dari Laporan Laba Rugi (Total Pendapatan − Total Beban). Lihat laporan Laba Rugi untuk rincian akun.',
                    items: [],
                    total: op.net_income,
                });
            }
            if ((op?.adjustments?.items?.length || 0) > 0) {
                components.push({
                    label: 'Penyesuaian Non-Kas',
                    items: op!.adjustments.items,
                    total: op!.adjustments.total_adjustments,
                });
            }
            if ((op?.working_capital_changes?.items?.length || 0) > 0) {
                components.push({
                    label: 'Perubahan Modal Kerja',
                    items: op!.working_capital_changes.items,
                    total: op!.working_capital_changes.total_working_capital_changes,
                });
            }
            setBreakdownData({
                title: 'Aktivitas Operasi',
                formula: 'Laba Bersih + Penyesuaian + Perubahan Modal Kerja',
                components,
            });
        } else if (section === 'investing') {
            setBreakdownData({
                title: 'Aktivitas Investasi',
                formula: 'Pembelian / Penjualan Aset & Investasi',
                components: [{
                    label: 'Item Investasi',
                    items: inv?.items || [],
                    total: inv?.total_investing_cash_flow || 0,
                }],
            });
        } else if (section === 'financing') {
            setBreakdownData({
                title: 'Aktivitas Pendanaan',
                formula: 'Modal, Pinjaman & Dividen',
                components: [{
                    label: 'Item Pendanaan',
                    items: fin?.items || [],
                    total: fin?.total_financing_cash_flow || 0,
                }],
            });
        } else if (section === 'summary') {
            const allItems: Array<{ account_code: string; account_name: string; amount: number }> = [];
            // Combine all items from all sections
            if ((op?.adjustments?.items?.length || 0) > 0) {
                op!.adjustments.items.forEach(i => allItems.push(i));
            }
            if ((op?.working_capital_changes?.items?.length || 0) > 0) {
                op!.working_capital_changes.items.forEach(i => allItems.push(i));
            }
            if ((inv?.items?.length || 0) > 0) {
                inv!.items.forEach(i => allItems.push(i));
            }
            if ((fin?.items?.length || 0) > 0) {
                fin!.items.forEach(i => allItems.push(i));
            }
            setBreakdownData({
                title: 'Ringkasan Seluruh Arus Kas',
                formula: `Kas Awal (${formatCurrency(data.cash_at_beginning || 0)}) + Perubahan Bersih = Kas Akhir`,
                components: [
                    { label: 'Arus Kas Operasi (A)', items: [...(op?.adjustments?.items || []), ...(op?.working_capital_changes?.items || [])], total: op?.total_operating_cash_flow || 0 },
                    { label: 'Arus Kas Investasi (B)', items: inv?.items || [], total: inv?.total_investing_cash_flow || 0 },
                    { label: 'Arus Kas Pendanaan (C)', items: fin?.items || [], total: fin?.total_financing_cash_flow || 0 },
                ],
            });
        }

        onBreakdownOpen();
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
                        aria-label={t('common.back') || 'Kembali'}
                        size="md"
                    />
                    <VStack align="start" spacing={0}>
                        <Heading as="h1" size="lg">
                            <HStack>
                                <Icon as={FiTrendingUp} color="purple.500" />
                                <Text>Laporan Arus Kas</Text>
                            </HStack>
                        </Heading>
                        <Text fontSize="sm" color={descriptionColor}>
                            Mengukur seberapa baik perusahaan mengelola kas untuk membayar kewajiban hutang dan mendanai biaya operasional
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
                                            <Text>{t('reports.closedPeriod') || 'Periode tutup'}</Text>
                                            <Tooltip label={t('reports.modal.selectClosedPeriodTooltip') || 'Pilih periode tutup untuk memuat laporan berdasarkan periode fiskal yang telah ditutup'} placement="top">
                                                <Box><Icon as={FiInfo} color={infoIconColor} /></Box>
                                            </Tooltip>
                                        </HStack>
                                    </FormLabel>
                                    <Select
                                        placeholder={t('reports.selectClosedPeriod') || 'Pilih periode tutup'}
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
                                        {...fieldStyles}
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
                                    {t('reports.modal.loadHistory') || 'Muat Riwayat'}
                                </Button>
                            </HStack>
                            {!loadingClosedPeriods && closedPeriods.length === 0 && (
                                <Text fontSize="xs" color={descriptionColor}>
                                    Belum ada periode tutup final yang bisa dipakai untuk mode archive. Periode yang masih future-pending atau sudah reopen tidak ditampilkan.
                                </Text>
                            )}

                            <Divider />

                            {/* Date Range */}
                            <HStack spacing={4} flexWrap="wrap">
                                <FormControl flex="1" minW="200px">
                                    <FormLabel>{t('reports.modal.startDate') || 'Tanggal Mulai'}</FormLabel>
                                    <Input
                                        type="date"
                                        value={startDate}
                                        onChange={(e) => setStartDate(e.target.value)}
                                        {...fieldStyles}
                                    />
                                </FormControl>
                                <FormControl flex="1" minW="200px">
                                    <FormLabel>{t('reports.modal.endDate') || 'Tanggal Akhir'}</FormLabel>
                                    <Input
                                        type="date"
                                        value={endDate}
                                        onChange={(e) => setEndDate(e.target.value)}
                                        {...fieldStyles}
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
                                    {t('reports.modal.generateReport') || 'Buat Laporan'}
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
                            <Text fontSize="lg" fontWeight="medium">{t('reports.modal.generatingProfitLoss') || 'Menghasilkan Laporan Arus Kas...'}</Text>
                            <Text fontSize="sm" color={descriptionColor}>Menganalisis arus kas dari aktivitas operasi, investasi, dan pendanaan</Text>
                        </VStack>
                    </Box>
                )}

                {/* Error State */}
                {error && (
                    <Box bg={errorBg} p={4} borderRadius="md" border="1px solid" borderColor={errorBorderColor}>
                        <Text color="red.600" fontWeight="medium">Error: {error}</Text>
                        <Button mt={2} size="sm" colorScheme="red" variant="outline" onClick={fetchReport}>
                            {t('reports.modal.retry') || 'Coba Lagi'}
                        </Button>
                    </Box>
                )}

                {/* Report Data */}
                {data && !loading && (
                    <VStack spacing={6} align="stretch">
                        {/* Company Header */}
                        <Box bg={companyHeaderBg} p={4} borderRadius="md">
                            <HStack justify="space-between" align="start">
                                <VStack align="start" spacing={1}>
                                    <Text fontSize="lg" fontWeight="bold" color={companyHeaderTextColor}>
                                        {companyInfo?.name || data.company?.name || t('common.name') || 'Perusahaan'}
                                    </Text>
                                    <Text fontSize="sm" color={companyHeaderSubtextColor}>
                                        {companyInfo?.address || (data.company as any)?.address || ''}
                                    </Text>
                                </VStack>
                                <VStack align="end" spacing={1}>
                                    <Text fontSize="sm" color={companyHeaderSubtextColor}>
                                        {t('reports.modal.currency') || 'Mata Uang'}: {data.currency || 'IDR'}
                                    </Text>
                                </VStack>
                            </HStack>
                        </Box>

                        {/* Report Title */}
                        <Box textAlign="center" bg={summaryBg} p={4} borderRadius="md">
                            <Heading size="md" color={headingColor}>
                                Laporan Arus Kas (Metode Tidak Langsung)
                            </Heading>
                            <Text fontSize="sm" color={descriptionColor}>
                                {t('reports.modal.period') || 'Periode'}: {startDate} - {endDate}
                            </Text>
                        </Box>

                        <Box bg={notePanelBg} p={4} borderRadius="md" border="1px solid" borderColor={notePanelBorderColor}>
                            <Text fontSize="sm" fontWeight="semibold" color={notePanelTextColor} mb={1}>
                                Cara membaca angka di laporan ini
                            </Text>
                            <Text fontSize="sm" color={notePanelTextColor}>
                                Nilai negatif sekarang ditampilkan sebagai <b>Kas Keluar</b> dan nilai positif sebagai <b>Kas Masuk</b>, jadi lebih mudah dibaca tanpa harus menebak arti tanda minus.
                            </Text>
                        </Box>

                        {data.has_reconciliation_warning && (
                            <Box bg={errorBg} p={4} borderRadius="md" border="1px solid" borderColor={errorBorderColor}>
                                <Text fontSize="sm" fontWeight="semibold" color="red.600" mb={1}>
                                    Perlu pengecekan rekonsiliasi arus kas
                                </Text>
                                <Text fontSize="sm" color={descriptionColor}>
                                    Laporan ini menemukan selisih rekonsiliasi sebesar {formatCurrency(Math.abs(data.reconciliation_difference || 0))}.
                                    Angka tidak disamarkan otomatis, jadi mohon cek jurnal dan akun kas/bank pada periode ini.
                                </Text>
                            </Box>
                        )}

                        {/* Summary Statistics */}
                        <SimpleGrid columns={[1, 2, 4]} spacing={4}>
                            <Box bg={statPurpleBg} p={4} borderRadius="md" textAlign="center">
                                {renderAmountStack(data.cash_at_beginning || 0, { showBadge: false })}
                                <Text fontSize="sm" color={companyHeaderTextColor}>Kas Awal</Text>
                                <Text fontSize="xs" color={subtleTextColor}>Saldo awal periode</Text>
                            </Box>
                            <Box bg={statBlueBg} p={4} borderRadius="md" textAlign="center">
                                {renderAmountStack(op?.total_operating_cash_flow || 0)}
                                <Text fontSize="sm" color={ratioText}>Arus Kas Operasi</Text>
                            </Box>
                            <Box bg={statOrangeBg} p={4} borderRadius="md" textAlign="center">
                                {renderAmountStack(data.net_cash_flow || 0)}
                                <Text fontSize="sm" color="orange.800">Perubahan Bersih</Text>
                            </Box>
                            <Box bg={statGreenBg} p={4} borderRadius="md" textAlign="center">
                                {renderAmountStack(data.cash_at_end || 0, { showBadge: false })}
                                <Text fontSize="sm" color={summaryCashText}>Kas Akhir</Text>
                                <Text fontSize="xs" color={subtleTextColor}>Saldo akhir periode</Text>
                            </Box>
                        </SimpleGrid>

                        {/* ═══ OPERATING ACTIVITIES ═══ */}
                        <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                            <HStack justify="space-between" mb={3}>
                                <HStack>
                                    <Badge colorScheme="teal" px={2} py={1} fontSize="sm">A</Badge>
                                    <Text fontSize="md" fontWeight="bold" color={headingColor}>Aktivitas Operasi</Text>
                                    <Tooltip label="Arus kas dari kegiatan operasional perusahaan (metode tidak langsung)">
                                        <Box><Icon as={FiInfo} color={infoIconColor} /></Box>
                                    </Tooltip>
                                </HStack>
                                <HStack spacing={2}>
                                    {renderAmountInline(op?.total_operating_cash_flow || 0, { fontSize: 'md', fontWeight: 'bold' })}
                                    <Tooltip label="Lihat rincian akun">
                                        <IconButton
                                            icon={<FiEye />}
                                            aria-label="Detail Operasi"
                                            size="xs"
                                            variant="ghost"
                                            colorScheme="teal"
                                            onClick={() => handleShowBreakdown('operating')}
                                        />
                                    </Tooltip>
                                </HStack>
                            </HStack>

                            {/* Net Income */}
                            <HStack justify="space-between" pl={4} py={1} bg={sectionHeaderBg} borderRadius="md" mb={2} px={3}>
                                <Text fontSize="sm" fontWeight="semibold" color="teal.700">Laba Bersih</Text>
                                {renderAmountInline(op?.net_income || 0, { variant: 'performance', fontSize: 'sm', fontWeight: 'semibold' })}
                            </HStack>

                            {/* Adjustments */}
                            {((op?.adjustments?.items?.length || 0) > 0 || (op?.adjustments?.total_adjustments || 0) !== 0) && (
                                <Box mb={2}>
                                    <Text fontSize="xs" fontWeight="semibold" color={subtleTextColor} pl={4} mb={1}>Penyesuaian Non-Kas:</Text>
                                    <VStack spacing={0} align="stretch">
                                        {(op?.adjustments?.items || []).map((item, i) => (
                                            <HStack key={`adj-${i}`} justify="space-between" pl={8} py={1} _hover={{ bg: hoverBg }} borderRadius="md">
                                                <Text fontSize="sm" color={descriptionColor}>{item.account_code} {item.account_name}</Text>
                                                {renderAmountInline(item.amount, { fontSize: 'sm', fontWeight: 'medium', minW: '200px' })}
                                            </HStack>
                                        ))}
                                        <HStack justify="space-between" pl={4} py={1} borderTopWidth="1px" borderColor={dividerBorderColor} mt={1}>
                                            <Text fontSize="sm" fontWeight="semibold">Total Penyesuaian</Text>
                                            {renderAmountInline(op?.adjustments?.total_adjustments || 0, { fontSize: 'sm', fontWeight: 'semibold', minW: '200px' })}
                                        </HStack>
                                    </VStack>
                                </Box>
                            )}

                            {/* Working Capital Changes */}
                            {((op?.working_capital_changes?.items?.length || 0) > 0 || (op?.working_capital_changes?.total_working_capital_changes || 0) !== 0) && (
                                <Box mb={2}>
                                    <Text fontSize="xs" fontWeight="semibold" color={subtleTextColor} pl={4} mb={1}>Perubahan Modal Kerja:</Text>
                                    <VStack spacing={0} align="stretch">
                                        {(op?.working_capital_changes?.items || []).map((item, i) => (
                                            <HStack key={`wc-${i}`} justify="space-between" pl={8} py={1} _hover={{ bg: hoverBg }} borderRadius="md">
                                                <Text fontSize="sm" color={descriptionColor}>{item.account_code} {item.account_name}</Text>
                                                {renderAmountInline(item.amount, { fontSize: 'sm', fontWeight: 'medium', minW: '200px' })}
                                            </HStack>
                                        ))}
                                        <HStack justify="space-between" pl={4} py={1} borderTopWidth="1px" borderColor={dividerBorderColor} mt={1}>
                                            <Text fontSize="sm" fontWeight="semibold">Total Perubahan Modal Kerja</Text>
                                            {renderAmountInline(op?.working_capital_changes?.total_working_capital_changes || 0, { fontSize: 'sm', fontWeight: 'semibold', minW: '200px' })}
                                        </HStack>
                                    </VStack>
                                </Box>
                            )}

                            <Divider my={2} />
                            <HStack justify="space-between" pl={4} py={1}>
                                <Text fontSize="sm" fontWeight="bold" color={headingColor}>Total Arus Kas dari Aktivitas Operasi</Text>
                                {renderAmountInline(op?.total_operating_cash_flow || 0, { fontSize: 'sm', fontWeight: 'bold', minW: '220px' })}
                            </HStack>
                        </Box>

                        {/* ═══ INVESTING ACTIVITIES ═══ */}
                        <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                            <HStack justify="space-between" mb={3}>
                                <HStack>
                                    <Badge colorScheme="purple" px={2} py={1} fontSize="sm">B</Badge>
                                    <Text fontSize="md" fontWeight="bold" color={headingColor}>Aktivitas Investasi</Text>
                                    <Tooltip label="Arus kas dari pembelian/penjualan aset tetap dan investasi">
                                        <Box><Icon as={FiInfo} color={infoIconColor} /></Box>
                                    </Tooltip>
                                </HStack>
                                <HStack spacing={2}>
                                    {renderAmountInline(inv?.total_investing_cash_flow || 0, { fontSize: 'md', fontWeight: 'bold' })}
                                    <Tooltip label="Lihat rincian akun">
                                        <IconButton
                                            icon={<FiEye />}
                                            aria-label="Detail Investasi"
                                            size="xs"
                                            variant="ghost"
                                            colorScheme="purple"
                                            onClick={() => handleShowBreakdown('investing')}
                                        />
                                    </Tooltip>
                                </HStack>
                            </HStack>

                            {(inv?.items?.length || 0) > 0 ? (
                                <VStack spacing={0} align="stretch">
                                    {inv!.items.map((item, i) => (
                                        <HStack key={`inv-${i}`} justify="space-between" pl={4} py={1} _hover={{ bg: hoverBg }} borderRadius="md">
                                            <Text fontSize="sm" color={descriptionColor}>{item.account_code} {item.account_name}</Text>
                                            {renderAmountInline(item.amount, { fontSize: 'sm', fontWeight: 'medium', minW: '200px' })}
                                        </HStack>
                                    ))}
                                </VStack>
                            ) : (
                                <Text fontSize="sm" color={subtleTextColor} pl={4} fontStyle="italic">Tidak ada aktivitas investasi pada periode ini</Text>
                            )}

                            <Divider my={2} />
                            <HStack justify="space-between" pl={4} py={1}>
                                <Text fontSize="sm" fontWeight="bold" color={headingColor}>Total Arus Kas dari Aktivitas Investasi</Text>
                                {renderAmountInline(inv?.total_investing_cash_flow || 0, { fontSize: 'sm', fontWeight: 'bold', minW: '220px' })}
                            </HStack>
                        </Box>

                        {/* ═══ FINANCING ACTIVITIES ═══ */}
                        <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                            <HStack justify="space-between" mb={3}>
                                <HStack>
                                    <Badge colorScheme="green" px={2} py={1} fontSize="sm">C</Badge>
                                    <Text fontSize="md" fontWeight="bold" color={headingColor}>Aktivitas Pendanaan</Text>
                                    <Tooltip label="Arus kas dari modal, pinjaman, dan dividen">
                                        <Box><Icon as={FiInfo} color={infoIconColor} /></Box>
                                    </Tooltip>
                                </HStack>
                                <HStack spacing={2}>
                                    {renderAmountInline(fin?.total_financing_cash_flow || 0, { fontSize: 'md', fontWeight: 'bold' })}
                                    <Tooltip label="Lihat rincian akun">
                                        <IconButton
                                            icon={<FiEye />}
                                            aria-label="Detail Pendanaan"
                                            size="xs"
                                            variant="ghost"
                                            colorScheme="green"
                                            onClick={() => handleShowBreakdown('financing')}
                                        />
                                    </Tooltip>
                                </HStack>
                            </HStack>

                            {(fin?.items?.length || 0) > 0 ? (
                                <VStack spacing={0} align="stretch">
                                    {fin!.items.map((item, i) => (
                                        <HStack key={`fin-${i}`} justify="space-between" pl={4} py={1} _hover={{ bg: hoverBg }} borderRadius="md">
                                            <Text fontSize="sm" color={descriptionColor}>{item.account_code} {item.account_name}</Text>
                                            {renderAmountInline(item.amount, { fontSize: 'sm', fontWeight: 'medium', minW: '200px' })}
                                        </HStack>
                                    ))}
                                </VStack>
                            ) : (
                                <Text fontSize="sm" color={subtleTextColor} pl={4} fontStyle="italic">Tidak ada aktivitas pendanaan pada periode ini</Text>
                            )}

                            <Divider my={2} />
                            <HStack justify="space-between" pl={4} py={1}>
                                <Text fontSize="sm" fontWeight="bold" color={headingColor}>Total Arus Kas dari Aktivitas Pendanaan</Text>
                                {renderAmountInline(fin?.total_financing_cash_flow || 0, { fontSize: 'sm', fontWeight: 'bold', minW: '220px' })}
                            </HStack>
                        </Box>

                        {/* ═══ CASH FLOW SUMMARY ═══ */}
                        <Box bg={summaryCashBg} p={4} borderRadius="md" border="1px" borderColor={summaryCashBorder}>
                            <HStack justify="space-between" mb={3}>
                                <Text fontSize="md" fontWeight="bold" color={summaryCashText}>
                                    Ringkasan Arus Kas
                                </Text>
                                <Tooltip label="Lihat rincian semua akun">
                                    <IconButton
                                        icon={<FiEye />}
                                        aria-label="Detail Ringkasan"
                                        size="xs"
                                        variant="ghost"
                                        colorScheme="green"
                                        onClick={() => handleShowBreakdown('summary')}
                                    />
                                </Tooltip>
                            </HStack>
                            <SimpleGrid columns={[1, 2]} spacing={3}>
                                <VStack spacing={2}>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color={summaryCashText}>Kas Awal Periode:</Text>
                                        <Text fontSize="sm" fontWeight="semibold">{formatCurrency(data.cash_at_beginning || 0)}</Text>
                                    </HStack>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color={summaryCashText}>Arus Kas Operasi (A):</Text>
                                        {renderAmountInline(op?.total_operating_cash_flow || 0, { fontSize: 'sm', fontWeight: 'semibold', minW: '180px' })}
                                    </HStack>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color={summaryCashText}>Arus Kas Investasi (B):</Text>
                                        {renderAmountInline(inv?.total_investing_cash_flow || 0, { fontSize: 'sm', fontWeight: 'semibold', minW: '180px' })}
                                    </HStack>
                                </VStack>
                                <VStack spacing={2}>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color={summaryCashText}>Arus Kas Pendanaan (C):</Text>
                                        {renderAmountInline(fin?.total_financing_cash_flow || 0, { fontSize: 'sm', fontWeight: 'semibold', minW: '180px' })}
                                    </HStack>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color={summaryCashText}>Kenaikan (Penurunan) Bersih:</Text>
                                        {renderAmountInline(data.net_cash_flow || 0, { fontSize: 'sm', fontWeight: 'semibold', minW: '180px' })}
                                    </HStack>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color={summaryCashText} fontWeight="bold">Kas Akhir Periode:</Text>
                                        <Text fontSize="md" fontWeight="bold" color="green.700">
                                            {formatCurrency(data.cash_at_end || 0)}
                                        </Text>
                                    </HStack>
                                </VStack>
                            </SimpleGrid>
                        </Box>

                        {/* Cash Flow Ratios */}
                        {data.cash_flow_ratios && (
                            <Box bg={ratioBg} p={4} borderRadius="md" border="1px" borderColor={ratioBorder}>
                                <Text fontSize="md" fontWeight="bold" color={ratioText} mb={3}>
                                    Rasio Arus Kas
                                </Text>
                                <SimpleGrid columns={[1, 3]} spacing={3}>
                                    <HStack justify="space-between">
                                        <Text fontSize="sm" color={ratioText}>Operating CF Ratio:</Text>
                                        <Text fontSize="sm" fontWeight="semibold">{(data.cash_flow_ratios.operating_cash_flow_ratio || 0).toFixed(2)}x</Text>
                                    </HStack>
                                    <HStack justify="space-between">
                                        <Text fontSize="sm" color={ratioText}>Free Cash Flow:</Text>
                                        {renderAmountInline(data.cash_flow_ratios.free_cash_flow || 0, { fontSize: 'sm', fontWeight: 'semibold', minW: '180px' })}
                                    </HStack>
                                    <HStack justify="space-between">
                                        <Text fontSize="sm" color={ratioText}>CF to Debt Ratio:</Text>
                                        <Text fontSize="sm" fontWeight="semibold">{(data.cash_flow_ratios.cash_flow_to_debt_ratio || 0).toFixed(2)}x</Text>
                                    </HStack>
                                </SimpleGrid>
                            </Box>
                        )}

                        {/* Export Buttons */}
                        <HStack spacing={3} justify="flex-end">
                            <Button
                                colorScheme="red"
                                variant="outline"
                                leftIcon={<FiFilePlus />}
                                onClick={() => handleExport('pdf')}
                            >
                                {t('reports.exportPDF') || 'Ekspor PDF'}
                            </Button>
                            <Button
                                colorScheme="green"
                                variant="outline"
                                leftIcon={<FiFileText />}
                                onClick={() => handleExport('csv')}
                            >
                                {t('reports.exportCSV') || 'Ekspor CSV'}
                            </Button>
                        </HStack>
                    </VStack>
                )}
            </VStack>

            {/* ═══ BREAKDOWN MODAL ═══ */}
            <Modal isOpen={isBreakdownOpen} onClose={onBreakdownClose} size="xl" scrollBehavior="inside">
                <ModalOverlay />
                <ModalContent maxW="650px" bg={modalBg} border="1px solid" borderColor={modalBorderColor}>
                    <ModalHeader pb={2} bg={modalHeaderBg} borderBottom="1px solid" borderColor={modalBorderColor}>
                        <VStack align="start" spacing={1}>
                            <Text fontSize="lg" fontWeight="bold" color={headingColor}>
                                Rincian Perhitungan
                            </Text>
                            {breakdownData && (
                                <Badge colorScheme="purple" fontSize="sm">{breakdownData.title}</Badge>
                            )}
                        </VStack>
                    </ModalHeader>
                    <ModalCloseButton />
                    <ModalBody pb={6}>
                        {breakdownData?.formula && (
                            <Box bg={infoPanelBg} p={3} borderRadius="md" mb={4} borderLeft="4px solid" borderColor={infoPanelBorderColor}>
                                <Text fontSize="sm" color={infoPanelTextColor}>
                                    Formula: {breakdownData.formula}
                                </Text>
                            </Box>
                        )}

                        {breakdownData?.components.map((comp, ci) => (
                            <Box key={ci} mb={4}>
                                <HStack justify="space-between" mb={2} bg={sectionHeaderBg} p={2} borderRadius="md">
                                    <HStack>
                                        <Badge colorScheme={getAmountMeta(comp.total).badgeScheme} fontSize="xs">
                                            {getAmountMeta(comp.total).badgeLabel}
                                        </Badge>
                                        <Text fontSize="sm" fontWeight="bold">{comp.label}</Text>
                                    </HStack>
                                    <Text fontSize="sm" fontWeight="bold" color={getAmountMeta(comp.total).textColor}>
                                        {formatCurrency(getAmountMeta(comp.total).absoluteValue)}
                                    </Text>
                                </HStack>

                                {comp.items.length > 0 ? (
                                    <Table size="sm" variant="simple">
                                        <Thead>
                                            <Tr>
                                                <Th fontSize="xs" py={1}>Kode Akun</Th>
                                                <Th fontSize="xs" py={1}>Nama Akun</Th>
                                                <Th fontSize="xs" py={1} isNumeric>Jumlah</Th>
                                            </Tr>
                                        </Thead>
                                        <Tbody>
                                            {comp.items.map((item: any, ii: number) => (
                                                <Tr key={ii} _hover={{ bg: hoverBg }}>
                                                    <Td fontSize="xs" py={1} color={subtleTextColor} w="90px">
                                                        {item.account_code || '-'}
                                                    </Td>
                                                    <Td fontSize="xs" py={1}>
                                                        {item.account_name || item.name || '-'}
                                                    </Td>
                                                    <Td fontSize="xs" py={1} isNumeric color={getAmountMeta(item.amount).textColor}>
                                                        {formatAmountNarrative(item.amount)}
                                                    </Td>
                                                </Tr>
                                            ))}
                                        </Tbody>
                                    </Table>
                                ) : comp.description ? (
                                    <Box bg={notePanelBg} p={3} borderRadius="md" borderLeft="3px solid" borderColor={notePanelBorderColor}>
                                        <Text fontSize="xs" color={notePanelTextColor} fontStyle="italic">
                                            {comp.description}
                                        </Text>
                                    </Box>
                                ) : (
                                    <Text fontSize="xs" color={subtleTextColor} fontStyle="italic" pl={2}>
                                        Tidak ada item pada seksi ini
                                    </Text>
                                )}
                            </Box>
                        ))}
                    </ModalBody>
                    <ModalFooter>
                        <Button colorScheme="purple" onClick={onBreakdownClose}>Tutup</Button>
                    </ModalFooter>
                </ModalContent>
            </Modal>
        </UnifiedLayout>
    );
};

export default CashFlowPage;
