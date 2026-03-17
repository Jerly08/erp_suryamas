'use client';

import React, { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useTranslation } from '@/hooks/useTranslation';
import UnifiedLayout from '@/components/layout/UnifiedLayout';
import { ssotTrialBalanceService, SSOTTrialBalanceData } from '@/services/ssotTrialBalanceService';
import api from '@/services/api';
import { API_ENDPOINTS } from '@/config/api';
import {
    Box, Heading, Text, Button, VStack, HStack, Card, CardBody, Icon, SimpleGrid,
    FormControl, FormLabel, Input, Select, Spinner, useToast, useColorModeValue,
    Divider, Tooltip, IconButton, Badge, Table, Thead, Tbody, Tr, Th, Td, TableContainer, Tfoot,
    Modal, ModalOverlay, ModalContent, ModalHeader, ModalBody, ModalFooter,
    ModalCloseButton, Grid, GridItem, useDisclosure,
} from '@chakra-ui/react';
import {
    FiArrowLeft, FiList, FiFileText, FiFilePlus, FiCalendar, FiClock,
    FiInfo, FiCheckCircle, FiAlertTriangle, FiExternalLink, FiHash,
} from 'react-icons/fi';

const MONTHS = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];
const toISO = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;

const buildUniqueClosedPeriods = (entries: any[]): any[] => {
    if (!Array.isArray(entries)) return [];
    const mapped = entries
        .filter((entry: any) => entry && (entry.entry_date || entry.end_date))
        .map((entry: any) => {
            const rawEnd = String(entry.entry_date || entry.end_date);
            const endDateKey = rawEnd.slice(0, 10);
            const endDateObj = new Date(endDateKey);
            const formattedEndDate = endDateObj.toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric' });
            const rawStart = entry.start_date ? String(entry.start_date).slice(0, 10) : undefined;
            return { value: endDateKey, label: `${formattedEndDate} - ${entry.description || 'Period Closing'}`, date: endDateKey, start_date: rawStart };
        });
    const uniqueMap = new Map<string, any>();
    mapped.forEach((p: any) => { if (!uniqueMap.has(p.date)) uniqueMap.set(p.date, p); });
    return Array.from(uniqueMap.values()).sort((a, b) => new Date(b.date).getTime() - new Date(a.date).getTime());
};

// Helper to get account type badge color
const getAccountTypeBadgeColor = (type: string): string => {
    switch (type) {
        case 'Asset': return 'blue';
        case 'Liability': return 'red';
        case 'Equity': return 'purple';
        case 'Revenue': return 'green';
        case 'Expense': return 'orange';
        default: return 'gray';
    }
};
const accountTypeLabel = (type: string): string => {
    switch (type) {
        case 'Asset': return 'Aset';
        case 'Liability': return 'Kewajiban';
        case 'Equity': return 'Ekuitas';
        case 'Revenue': return 'Pendapatan';
        case 'Expense': return 'Beban';
        default: return type;
    }
};

const TrialBalancePage: React.FC = () => {
    const router = useRouter();
    const { t } = useTranslation();
    const toast = useToast();

    const cardBg = useColorModeValue('white', 'gray.800');
    const isDarkMode = useColorModeValue(false, true);
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
    const companyHeaderBg = useColorModeValue('cyan.50', 'rgba(0, 180, 216, 0.14)');
    const companyHeaderTextColor = useColorModeValue('cyan.800', 'cyan.100');
    const companyHeaderSubtextColor = useColorModeValue('cyan.600', 'cyan.200');
    const statBlueBg = useColorModeValue('blue.50', 'rgba(66, 153, 225, 0.14)');
    const statBlueText = useColorModeValue('blue.800', 'blue.100');
    const statRedBg = useColorModeValue('red.50', 'rgba(229, 62, 62, 0.14)');
    const statRedText = useColorModeValue('red.800', 'red.100');
    const balancedBg = useColorModeValue('green.50', 'rgba(72, 187, 120, 0.14)');
    const unbalancedBg = useColorModeValue('yellow.50', 'rgba(214, 158, 46, 0.14)');
    const summaryPanelBg = useColorModeValue('cyan.50', 'rgba(0, 180, 216, 0.14)');
    const summaryPanelBorder = useColorModeValue('cyan.200', 'cyan.700');
    const summaryPanelText = useColorModeValue('cyan.800', 'cyan.100');
    const hoverBg = useColorModeValue('gray.50', 'whiteAlpha.100');
    const subtleTextColor = useColorModeValue('gray.500', 'gray.400');
    const detailSummaryBg = useColorModeValue('gray.50', 'whiteAlpha.100');
    const detailSummaryBorder = useColorModeValue('gray.200', 'whiteAlpha.200');
    const modalBg = useColorModeValue('white', 'gray.800');
    const modalBorderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
    const tableHeaderBg = useColorModeValue('cyan.600', 'cyan.800');
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
    const getToneBg = (color: string) => (isDarkMode ? `${color}.900` : `${color}.50`);
    const warningTextColor = isDarkMode ? 'yellow.100' : 'yellow.700';

    const [asOfDate, setAsOfDate] = useState('');
    const [data, setData] = useState<SSOTTrialBalanceData | null>(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [closedPeriods, setClosedPeriods] = useState<any[]>([]);
    const [loadingClosedPeriods, setLoadingClosedPeriods] = useState(false);
    const [companyInfo, setCompanyInfo] = useState<{ name?: string; address?: string; city?: string } | null>(null);
    const [selectedType, setSelectedType] = useState<string | null>(null);
    const { isOpen: isDetailOpen, onOpen: onDetailOpen, onClose: onDetailClose } = useDisclosure();

    useEffect(() => {
        const loadSettings = async () => {
            try {
                const resp = await api.get(API_ENDPOINTS.SETTINGS);
                const s = resp.data?.data;
                if (s?.company_name || s?.company_address) {
                    setCompanyInfo({ name: s.company_name, address: s.company_address, city: s.company_city });
                }
            } catch { /* ignore */ }
        };
        setAsOfDate(toISO(new Date()));
        loadSettings();
        loadClosedPeriods();
    }, []);

    const loadClosedPeriods = async () => {
        setLoadingClosedPeriods(true);
        try {
            const response = await api.get(API_ENDPOINTS.FISCAL_CLOSING.HISTORY);
            if (response.data.success && Array.isArray(response.data.data)) {
                setClosedPeriods(buildUniqueClosedPeriods(response.data.data));
            }
        } catch { /* ignore */ } finally { setLoadingClosedPeriods(false); }
    };

    const fetchReport = async () => {
        setLoading(true);
        setError(null);
        try {
            const result = await ssotTrialBalanceService.generateSSOTTrialBalance({ as_of_date: asOfDate, format: 'json' });
            setData(result);
            if (result.company) {
                setCompanyInfo(prev => ({ name: result.company?.name || prev?.name, address: result.company?.address || prev?.address, city: result.company?.city || prev?.city }));
            }
            toast({ title: 'Berhasil', description: 'Neraca saldo berhasil dimuat', status: 'success', duration: 3000, isClosable: true });
        } catch (err: any) {
            setError(err.message);
            toast({ title: 'Gagal', description: err.message, status: 'error', duration: 5000, isClosable: true });
        } finally { setLoading(false); }
    };

    const handleExport = async (format: 'pdf' | 'csv') => {
        if (!data) {
            toast({ title: 'Error', description: 'Tidak ada data untuk diekspor', status: 'warning', duration: 3000, isClosable: true });
            return;
        }
        try {
            if (format === 'pdf') {
                await ssotTrialBalanceService.exportToPDF({ as_of_date: asOfDate });
                toast({ title: 'Ekspor Berhasil', status: 'success', duration: 3000, isClosable: true });
            } else {
                // CSV: generate client-side (backend only returns JSON metadata for csv)
                if (!data.accounts?.length) {
                    toast({ title: 'Error', description: 'Tidak ada data akun untuk diekspor', status: 'warning', duration: 3000, isClosable: true });
                    return;
                }
                const rows: string[][] = [];
                const COLS = 7; // consistent column count
                const emptyRow = () => new Array(COLS).fill('');
                const padRow = (arr: string[]) => {
                    while (arr.length < COLS) arr.push('');
                    return arr;
                };

                // ── Company Header (all rows padded to 7 columns) ──
                rows.push(padRow([companyInfo?.name || data.company?.name || 'Perusahaan']));
                rows.push(padRow([companyInfo?.address || data.company?.address || '']));
                rows.push(emptyRow());
                rows.push(padRow(['NERACA SALDO (TRIAL BALANCE)']));
                rows.push(padRow([`Per Tanggal: ${asOfDate}`]));
                rows.push(padRow([`Mata Uang: ${data.currency || 'IDR'}`]));
                rows.push(padRow([`Dicetak: ${new Date().toLocaleDateString('id-ID', { day: '2-digit', month: 'long', year: 'numeric' })}`]));
                rows.push(emptyRow());

                // ── Table Header ──
                rows.push(['Kode Akun', 'Nama Akun', 'Tipe Akun', 'Saldo Debit', 'Saldo Kredit', 'Saldo Bersih', 'D/K']);

                // ── Accounts Grouped by Type ──
                const typeLabels: Record<string, string> = { Asset: 'Aset', Liability: 'Kewajiban', Equity: 'Ekuitas', Revenue: 'Pendapatan', Expense: 'Beban', Other: 'Lainnya' };
                const groups = getAccountsByType();
                const orderedTypes = ['Asset', 'Liability', 'Equity', 'Revenue', 'Expense', 'Other'];

                orderedTypes.filter(t => groups[t]).forEach(type => {
                    const group = groups[type];
                    const label = typeLabels[type] || type;

                    // Group Header
                    rows.push(padRow([`── ${label} (${group.accounts.length} akun) ──`]));

                    // Accounts sorted by code
                    const sorted = [...group.accounts].sort((a: any, b: any) => (a.account_code || '').localeCompare(b.account_code || ''));
                    sorted.forEach((acc: any) => {
                        const debit = acc.debit_balance || 0;
                        const credit = acc.credit_balance || 0;
                        const net = debit - credit;
                        rows.push([
                            acc.account_code || '',
                            acc.account_name || '',
                            label,
                            String(debit),
                            String(credit),
                            String(Math.abs(net)),
                            net > 0 ? 'D' : net < 0 ? 'K' : '-'
                        ]);
                    });

                    // Subtotal
                    const netSub = group.totalDebit - group.totalCredit;
                    rows.push(['', `Sub-total ${label}`, '', String(group.totalDebit), String(group.totalCredit), String(Math.abs(netSub)), netSub >= 0 ? 'D' : 'K']);
                    rows.push(emptyRow());
                });

                // ── Grand Total ──
                rows.push(['', 'TOTAL', '', String(data.total_debits), String(data.total_credits), String(Math.abs(data.total_debits - data.total_credits)), data.total_debits >= data.total_credits ? 'D' : 'K']);
                rows.push(emptyRow());

                // ── Summary ──
                rows.push(padRow(['RINGKASAN']));
                rows.push(padRow(['Jumlah Akun', String(data.accounts.length)]));
                rows.push(padRow(['Total Debit', String(data.total_debits)]));
                rows.push(padRow(['Total Kredit', String(data.total_credits)]));
                rows.push(padRow(['Selisih', String(Math.abs(data.difference || 0))]));
                rows.push(padRow(['Status', data.is_balanced ? 'SEIMBANG' : 'TIDAK SEIMBANG']));

                // BOM + CSV content with semicolon separator (Excel-friendly for Indonesian locale)
                const bom = '\uFEFF';
                const sep = 'sep=;\n'; // tells Excel to use semicolon as delimiter
                const csv = bom + sep + rows.map(r => r.map(c => `"${String(c).replace(/"/g, '""')}"`).join(';')).join('\n');
                const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
                const url = URL.createObjectURL(blob);
                const link = document.createElement('a');
                link.href = url;
                link.download = `Neraca_Saldo_${asOfDate}.csv`;
                document.body.appendChild(link);
                link.click();
                document.body.removeChild(link);
                URL.revokeObjectURL(url);
                toast({ title: 'Ekspor CSV Berhasil', status: 'success', duration: 3000, isClosable: true });
            }
        } catch (err: any) {
            console.error('Export error:', err);
            toast({ title: 'Ekspor Gagal', description: err.message, status: 'error', duration: 5000, isClosable: true });
        }
    };

    const formatCurrency = (amount: number | null | undefined) => {
        if (amount === null || amount === undefined || isNaN(Number(amount))) return 'Rp 0';
        return new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', minimumFractionDigits: 0, maximumFractionDigits: 0 }).format(Number(amount));
    };

    // Group accounts by type for summary
    const getAccountsByType = () => {
        if (!data?.accounts) return {};
        const groups: { [key: string]: { accounts: any[]; totalDebit: number; totalCredit: number } } = {};
        data.accounts.forEach((acc: any) => {
            const type = acc.account_type || 'Other';
            if (!groups[type]) groups[type] = { accounts: [], totalDebit: 0, totalCredit: 0 };
            groups[type].accounts.push(acc);
            groups[type].totalDebit += acc.debit_balance || 0;
            groups[type].totalCredit += acc.credit_balance || 0;
        });
        return groups;
    };

    const accountGroups = data ? getAccountsByType() : {};
    const typeOrder = ['Asset', 'Liability', 'Equity', 'Revenue', 'Expense', 'Other'];

    return (
        <UnifiedLayout>
            <VStack spacing={6} align="stretch">
                {/* Header */}
                <HStack spacing={4}>
                    <IconButton icon={<FiArrowLeft />} variant="outline" onClick={() => router.push('/reports')} aria-label="Kembali" size="md" />
                    <VStack align="start" spacing={0}>
                        <Heading as="h1" size="lg">
                            <HStack>
                                <Icon as={FiList} color="cyan.500" />
                                <Text>{t('reports.trialBalance.title') || 'Neraca Saldo'}</Text>
                            </HStack>
                        </Heading>
                        <Text fontSize="sm" color={descriptionColor}>
                            {t('reports.description.trialBalance') || 'Menyajikan saldo debit dan kredit seluruh akun pada tanggal tertentu.'}
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
                                            <Tooltip label="Pilih periode tutup untuk memuat saldo pada tanggal penutupan" placement="top">
                                                <Box><Icon as={FiInfo} color={infoIconColor} /></Box>
                                            </Tooltip>
                                        </HStack>
                                    </FormLabel>
                                    <Select
                                        placeholder={t('reports.selectClosedPeriod') || 'Pilih periode tutup'}
                                        size="md"
                                        onChange={(e) => {
                                            if (e.target.value) {
                                                setAsOfDate(e.target.value);
                                            }
                                        }}
                                        isDisabled={loadingClosedPeriods}
                                        {...fieldStyles}
                                    >
                                        {closedPeriods.map((p, i) => (
                                            <option key={`${p.date}-${i}`} value={p.date}>{p.label}</option>
                                        ))}
                                    </Select>
                                </FormControl>
                                <Button size="md" variant="outline" colorScheme="blue" onClick={loadClosedPeriods} isLoading={loadingClosedPeriods} leftIcon={<FiClock />}>
                                    {t('reports.modal.loadHistory') || 'Muat Riwayat'}
                                </Button>
                            </HStack>

                            <Divider />

                            {/* Date picker */}
                            <HStack spacing={4} flexWrap="wrap">
                                <FormControl flex="1" minW="200px">
                                    <FormLabel>{t('reports.modal.asOfDate') || 'Per Tanggal'}</FormLabel>
                                    <Input type="date" value={asOfDate} onChange={(e) => setAsOfDate(e.target.value)} {...fieldStyles} />
                                </FormControl>
                                <Button colorScheme="blue" onClick={fetchReport} isLoading={loading} leftIcon={<FiList />} size="md" mt={8}>
                                    {t('reports.modal.generateReport') || 'Buat Laporan'}
                                </Button>
                            </HStack>
                        </VStack>
                    </CardBody>
                </Card>

                {/* Loading */}
                {loading && (
                    <Box textAlign="center" py={8}>
                        <VStack spacing={4}>
                            <Spinner size="xl" thickness="4px" speed="0.65s" color="cyan.500" />
                            <Text fontSize="lg" fontWeight="medium">Menghasilkan Neraca Saldo...</Text>
                            <Text fontSize="sm" color={descriptionColor}>Menghitung saldo dari jurnal SSOT</Text>
                        </VStack>
                    </Box>
                )}

                {/* Error */}
                {error && (
                    <Box bg={errorBg} p={4} borderRadius="md" border="1px solid" borderColor={errorBorderColor}>
                        <Text color="red.600" fontWeight="medium">Error: {error}</Text>
                        <Button mt={2} size="sm" colorScheme="red" variant="outline" onClick={fetchReport}>Coba Lagi</Button>
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
                                        {companyInfo?.name || data.company?.name || 'Perusahaan'}
                                    </Text>
                                    <Text fontSize="sm" color={companyHeaderSubtextColor}>
                                        {companyInfo?.address || data.company?.address || ''}{companyInfo?.city ? `, ${companyInfo.city}` : data.company?.city ? `, ${data.company.city}` : ''}
                                    </Text>
                                </VStack>
                                <Text fontSize="sm" color={companyHeaderSubtextColor}>
                                    Mata Uang: {data.currency || 'IDR'}
                                </Text>
                            </HStack>
                        </Box>

                        {/* Report Title */}
                        <Box textAlign="center" bg={summaryBg} p={4} borderRadius="md">
                            <Heading size="md" color={headingColor}>{t('reports.trialBalance.title') || 'Neraca Saldo'}</Heading>
                            <Text fontSize="sm" color={descriptionColor}>Per Tanggal: {asOfDate}</Text>
                        </Box>

                        {/* Summary Statistics */}
                        <SimpleGrid columns={[1, 3]} spacing={4}>
                            <Box bg={statBlueBg} p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="blue.600">{formatCurrency(data.total_debits)}</Text>
                                <Text fontSize="sm" color={statBlueText}>{t('reports.trialBalance.totalDebits') || 'Total Debit'}</Text>
                            </Box>
                            <Box bg={statRedBg} p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="red.600">{formatCurrency(data.total_credits)}</Text>
                                <Text fontSize="sm" color={statRedText}>{t('reports.trialBalance.totalCredits') || 'Total Kredit'}</Text>
                            </Box>
                            <Box bg={data.is_balanced ? balancedBg : unbalancedBg} p={4} borderRadius="md" textAlign="center">
                                <HStack justify="center" spacing={2} mb={1}>
                                    <Icon as={data.is_balanced ? FiCheckCircle : FiAlertTriangle} color={data.is_balanced ? "green.500" : "yellow.500"} boxSize={6} />
                                    <Badge colorScheme={data.is_balanced ? "green" : "yellow"} fontSize="lg" px={4} py={2}>
                                        {data.is_balanced ? (t('reports.trialBalance.balanced') || 'SEIMBANG') : (t('reports.trialBalance.notBalanced') || 'TIDAK SEIMBANG')}
                                    </Badge>
                                </HStack>
                                {!data.is_balanced && data.difference !== 0 && (
                                    <Text fontSize="sm" color={warningTextColor}>Selisih: {formatCurrency(Math.abs(data.difference))}</Text>
                                )}
                            </Box>
                        </SimpleGrid>

                        {/* Account Type Summary Cards */}
                        {Object.keys(accountGroups).length > 0 && (
                            <SimpleGrid columns={[2, 3, 5]} spacing={3}>
                                {typeOrder.filter(t => accountGroups[t]).map(type => {
                                    const group = accountGroups[type];
                                    const netBalance = group.totalDebit - group.totalCredit;
                                    const isDebitNormal = type === 'Asset' || type === 'Expense';
                                    return (
                                        <Box key={type} bg={getToneBg(getAccountTypeBadgeColor(type))} p={3} borderRadius="md" textAlign="center"
                                            _hover={{ shadow: 'md', cursor: 'pointer', transform: 'translateY(-2px)' }}
                                            transition="all 0.2s"
                                            onClick={() => { setSelectedType(type); onDetailOpen(); }}
                                        >
                                            <HStack justify="center" spacing={1}>
                                                <Badge colorScheme={getAccountTypeBadgeColor(type)} mb={1}>{accountTypeLabel(type)}</Badge>
                                                <Icon as={FiExternalLink} boxSize={3} color={`${getAccountTypeBadgeColor(type)}.400`} />
                                            </HStack>
                                            <Text fontSize="xs" color={subtleTextColor}>{group.accounts.length} akun</Text>
                                            <Text fontSize="sm" fontWeight="bold" color={`${getAccountTypeBadgeColor(type)}.700`}>
                                                {formatCurrency(isDebitNormal ? group.totalDebit : group.totalCredit)}
                                            </Text>
                                        </Box>
                                    );
                                })}
                            </SimpleGrid>
                        )}

                        {/* Accounts Table - Grouped by Type */}
                        {data.accounts && data.accounts.length > 0 && (
                            <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                <HStack justify="space-between" mb={3}>
                                    <HStack>
                                        <Icon as={FiList} color="cyan.500" />
                                        <Text fontSize="md" fontWeight="bold" color={headingColor}>Daftar Akun</Text>
                                    </HStack>
                                    <Text fontSize="sm" color={descriptionColor}>{data.accounts.length} akun</Text>
                                </HStack>
                                <TableContainer maxH="600px" overflowY="auto">
                                    <Table size="sm" variant="simple">
                                        <Thead bg={tableHeaderBg} position="sticky" top={0} zIndex={1}>
                                            <Tr>
                                                <Th color="white" width="120px">{t('reports.trialBalance.accountCode') || 'Kode Akun'}</Th>
                                                <Th color="white">{t('reports.trialBalance.accountName') || 'Nama Akun'}</Th>
                                                <Th color="white" width="100px">{t('reports.trialBalance.accountType') || 'Tipe'}</Th>
                                                <Th color="white" isNumeric width="180px">{t('reports.trialBalance.debit') || 'Debit'}</Th>
                                                <Th color="white" isNumeric width="180px">{t('reports.trialBalance.credit') || 'Kredit'}</Th>
                                                <Th color="white" width="60px" textAlign="center">Aksi</Th>
                                            </Tr>
                                        </Thead>
                                        <Tbody>
                                            {typeOrder.filter(type => accountGroups[type]).map(type => {
                                                const group = accountGroups[type];
                                                return (
                                                    <React.Fragment key={`group-${type}`}>
                                                        {/* Group Header */}
                                                        <Tr bg={getToneBg(getAccountTypeBadgeColor(type))}>
                                                            <Td colSpan={5} py={2}>
                                                                <Text fontSize="sm" fontWeight="bold" color={`${getAccountTypeBadgeColor(type)}.700`}>
                                                                    {accountTypeLabel(type)} ({group.accounts.length} akun)
                                                                </Text>
                                                            </Td>
                                                            <Td py={2} textAlign="center">
                                                                <Tooltip label={`Lihat Detail ${accountTypeLabel(type)}`} placement="top">
                                                                    <IconButton
                                                                        icon={<FiExternalLink />}
                                                                        aria-label="Detail"
                                                                        size="xs"
                                                                        variant="ghost"
                                                                        colorScheme={getAccountTypeBadgeColor(type)}
                                                                        onClick={() => { setSelectedType(type); onDetailOpen(); }}
                                                                    />
                                                                </Tooltip>
                                                            </Td>
                                                        </Tr>
                                                        {/* Accounts in this group */}
                                                        {group.accounts.map((acc: any, idx: number) => (
                                                            <Tr key={`${type}-${idx}`} _hover={{ bg: hoverBg }}>
                                                                <Td fontFamily="mono" fontSize="sm">{acc.account_code}</Td>
                                                                <Td fontSize="sm">{acc.account_name}</Td>
                                                                <Td>
                                                                    <Badge colorScheme={getAccountTypeBadgeColor(acc.account_type)} size="sm" fontSize="xs">
                                                                        {accountTypeLabel(acc.account_type)}
                                                                    </Badge>
                                                                </Td>
                                                                <Td isNumeric color="blue.600" fontSize="sm" fontWeight={acc.debit_balance > 0 ? 'semibold' : 'normal'}>
                                                                    {acc.debit_balance > 0 ? formatCurrency(acc.debit_balance) : '-'}
                                                                </Td>
                                                                <Td isNumeric color="red.600" fontSize="sm" fontWeight={acc.credit_balance > 0 ? 'semibold' : 'normal'}>
                                                                    {acc.credit_balance > 0 ? formatCurrency(acc.credit_balance) : '-'}
                                                                </Td>
                                                                <Td></Td>
                                                            </Tr>
                                                        ))}
                                                        {/* Group Subtotal */}
                                                        <Tr bg={getToneBg(getAccountTypeBadgeColor(type))}>
                                                            <Td colSpan={3} py={1}>
                                                                <Text fontSize="xs" fontWeight="bold" color={`${getAccountTypeBadgeColor(type)}.700`} textAlign="right">
                                                                    Sub-total {accountTypeLabel(type)}
                                                                </Text>
                                                            </Td>
                                                            <Td isNumeric py={1}>
                                                                <Text fontSize="xs" fontWeight="bold" color="blue.700">
                                                                    {group.totalDebit > 0 ? formatCurrency(group.totalDebit) : '-'}
                                                                </Text>
                                                            </Td>
                                                            <Td isNumeric py={1}>
                                                                <Text fontSize="xs" fontWeight="bold" color="red.700">
                                                                    {group.totalCredit > 0 ? formatCurrency(group.totalCredit) : '-'}
                                                                </Text>
                                                            </Td>
                                                            <Td></Td>
                                                        </Tr>
                                                    </React.Fragment>
                                                );
                                            })}
                                        </Tbody>
                                        <Tfoot bg="cyan.700" position="sticky" bottom={0}>
                                            <Tr>
                                                <Th color="white" colSpan={3} fontSize="sm">TOTAL</Th>
                                                <Th color="white" isNumeric fontSize="sm">{formatCurrency(data.total_debits)}</Th>
                                                <Th color="white" isNumeric fontSize="sm">{formatCurrency(data.total_credits)}</Th>
                                                <Th></Th>
                                            </Tr>
                                        </Tfoot>
                                    </Table>
                                </TableContainer>
                            </Box>
                        )}

                        {/* Summary Box */}
                        <Box bg={summaryPanelBg} p={4} borderRadius="md" border="1px" borderColor={summaryPanelBorder}>
                            <Text fontSize="md" fontWeight="bold" color={summaryPanelText} mb={3}>
                                Ringkasan Neraca Saldo
                            </Text>
                            <SimpleGrid columns={[1, 2]} spacing={3}>
                                <VStack spacing={2}>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color={summaryPanelText}>Total Debit:</Text>
                                        <Text fontSize="sm" fontWeight="semibold" color="blue.600">{formatCurrency(data.total_debits)}</Text>
                                    </HStack>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color={summaryPanelText}>Total Kredit:</Text>
                                        <Text fontSize="sm" fontWeight="semibold" color="red.600">{formatCurrency(data.total_credits)}</Text>
                                    </HStack>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color={summaryPanelText}>Selisih:</Text>
                                        <Text fontSize="sm" fontWeight="semibold" color={data.difference === 0 ? 'green.600' : 'yellow.600'}>
                                            {formatCurrency(Math.abs(data.difference || 0))}
                                        </Text>
                                    </HStack>
                                </VStack>
                                <VStack spacing={2}>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color={summaryPanelText}>Jumlah Akun:</Text>
                                        <Text fontSize="sm" fontWeight="semibold">{data.accounts?.length || 0} akun</Text>
                                    </HStack>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color={summaryPanelText}>Status:</Text>
                                        <Badge colorScheme={data.is_balanced ? "green" : "yellow"}>
                                            {data.is_balanced ? 'SEIMBANG' : 'TIDAK SEIMBANG'}
                                        </Badge>
                                    </HStack>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color={summaryPanelText}>Per Tanggal:</Text>
                                        <Text fontSize="sm" fontWeight="semibold">{asOfDate}</Text>
                                    </HStack>
                                </VStack>
                            </SimpleGrid>
                        </Box>

                        {/* Export Buttons */}
                        <HStack spacing={3} justify="flex-end">
                            <Button colorScheme="red" variant="outline" leftIcon={<FiFilePlus />} onClick={() => handleExport('pdf')}>
                                {t('reports.exportPDF') || 'Ekspor PDF'}
                            </Button>
                            <Button colorScheme="green" variant="outline" leftIcon={<FiFileText />} onClick={() => handleExport('csv')}>
                                {t('reports.exportCSV') || 'Ekspor CSV'}
                            </Button>
                        </HStack>
                    </VStack>
                )}
            </VStack>

            {/* ═══ ACCOUNT TYPE DETAIL MODAL ═══ */}
            <Modal isOpen={isDetailOpen} onClose={onDetailClose} size="5xl" scrollBehavior="inside">
                <ModalOverlay bg="blackAlpha.600" backdropFilter="blur(4px)" />
                <ModalContent borderRadius="xl" overflow="hidden" bg={modalBg} border="1px solid" borderColor={modalBorderColor}>
                    <ModalHeader
                        bg="linear-gradient(135deg, #00b4d8, #0077b6)"
                        color="white"
                        py={4}
                    >
                        <HStack spacing={3}>
                            <Icon as={FiHash} boxSize={5} />
                            <Box>
                                <Text fontSize="lg" fontWeight="bold">Detail Akun — {selectedType ? accountTypeLabel(selectedType) : ''}</Text>
                                {selectedType && accountGroups[selectedType] && (
                                    <Text fontSize="sm" fontWeight="normal" opacity={0.9}>
                                        {accountGroups[selectedType].accounts.length} akun terdaftar
                                    </Text>
                                )}
                            </Box>
                        </HStack>
                    </ModalHeader>
                    <ModalCloseButton color="white" />
                    <ModalBody py={6}>
                        {selectedType && accountGroups[selectedType] && (() => {
                            const group = accountGroups[selectedType];
                            const color = getAccountTypeBadgeColor(selectedType);
                            return (
                                <VStack spacing={5} align="stretch">
                                    {/* Summary Card */}
                                    <Box bg={detailSummaryBg} p={4} borderRadius="lg" borderWidth="1px" borderColor={detailSummaryBorder}>
                                        <SimpleGrid columns={4} spacing={4}>
                                            <Box textAlign="center">
                                                <Text fontSize="xs" color={subtleTextColor}>Jumlah Akun</Text>
                                                <Text fontSize="xl" fontWeight="bold" color={`${color}.600`}>{group.accounts.length}</Text>
                                            </Box>
                                            <Box textAlign="center">
                                                <Text fontSize="xs" color={subtleTextColor}>Total Debit</Text>
                                                <Text fontSize="md" fontWeight="bold" color="blue.600">{formatCurrency(group.totalDebit)}</Text>
                                            </Box>
                                            <Box textAlign="center">
                                                <Text fontSize="xs" color={subtleTextColor}>Total Kredit</Text>
                                                <Text fontSize="md" fontWeight="bold" color="red.600">{formatCurrency(group.totalCredit)}</Text>
                                            </Box>
                                            <Box textAlign="center">
                                                <Text fontSize="xs" color={subtleTextColor}>Saldo Bersih</Text>
                                                <Text fontSize="md" fontWeight="bold" color={group.totalDebit >= group.totalCredit ? 'blue.600' : 'red.600'}>
                                                    {formatCurrency(Math.abs(group.totalDebit - group.totalCredit))}
                                                </Text>
                                            </Box>
                                        </SimpleGrid>
                                    </Box>

                                    {/* Accounts Detail Table */}
                                    <Box>
                                        <HStack mb={3}>
                                            <Icon as={FiList} color={`${color}.500`} />
                                            <Text fontWeight="bold" color={headingColor}>Daftar Kode Akun</Text>
                                        </HStack>
                                        <Box borderWidth="1px" borderRadius="lg" overflow="hidden">
                                            <Table size="sm" variant="simple">
                                                <Thead bg={getToneBg(color)}>
                                                    <Tr>
                                                        <Th width="50px">#</Th>
                                                        <Th width="120px">Kode Akun</Th>
                                                        <Th>Nama Akun</Th>
                                                        <Th isNumeric width="160px">Debit</Th>
                                                        <Th isNumeric width="160px">Kredit</Th>
                                                        <Th isNumeric width="160px">Saldo Bersih</Th>
                                                    </Tr>
                                                </Thead>
                                                <Tbody>
                                                    {group.accounts
                                                        .sort((a: any, b: any) => (a.account_code || '').localeCompare(b.account_code || ''))
                                                        .map((acc: any, i: number) => {
                                                            const net = (acc.debit_balance || 0) - (acc.credit_balance || 0);
                                                            return (
                                                                <Tr key={i} _hover={{ bg: hoverBg }}>
                                                                    <Td color={subtleTextColor} fontSize="xs">{i + 1}</Td>
                                                                    <Td>
                                                                        <Badge colorScheme={color} variant="subtle" fontFamily="mono" fontSize="xs">
                                                                            {acc.account_code}
                                                                        </Badge>
                                                                    </Td>
                                                                    <Td fontWeight="medium" fontSize="sm">{acc.account_name}</Td>
                                                                    <Td isNumeric fontSize="sm" color="blue.600" fontWeight={acc.debit_balance > 0 ? 'semibold' : 'normal'}>
                                                                        {acc.debit_balance > 0 ? formatCurrency(acc.debit_balance) : '-'}
                                                                    </Td>
                                                                    <Td isNumeric fontSize="sm" color="red.600" fontWeight={acc.credit_balance > 0 ? 'semibold' : 'normal'}>
                                                                        {acc.credit_balance > 0 ? formatCurrency(acc.credit_balance) : '-'}
                                                                    </Td>
                                                                    <Td isNumeric fontSize="sm" fontWeight="bold" color={net >= 0 ? 'blue.700' : 'red.700'}>
                                                                        {net !== 0 ? formatCurrency(Math.abs(net)) : '-'}
                                                                        {net !== 0 && <Text as="span" fontSize="xs" color={subtleTextColor} ml={1}>{net > 0 ? 'D' : 'K'}</Text>}
                                                                    </Td>
                                                                </Tr>
                                                            );
                                                        })}
                                                    {/* Totals Row */}
                                                    <Tr bg={`${color}.50`} fontWeight="bold">
                                                        <Td colSpan={3} textAlign="right" fontSize="sm">Total {accountTypeLabel(selectedType)}</Td>
                                                        <Td isNumeric color="blue.700" fontSize="sm">
                                                            {group.totalDebit > 0 ? formatCurrency(group.totalDebit) : '-'}
                                                        </Td>
                                                        <Td isNumeric color="red.700" fontSize="sm">
                                                            {group.totalCredit > 0 ? formatCurrency(group.totalCredit) : '-'}
                                                        </Td>
                                                        <Td isNumeric color={group.totalDebit >= group.totalCredit ? 'blue.700' : 'red.700'} fontSize="sm">
                                                            {formatCurrency(Math.abs(group.totalDebit - group.totalCredit))}
                                                            <Text as="span" fontSize="xs" color={subtleTextColor} ml={1}>
                                                                {group.totalDebit >= group.totalCredit ? 'D' : 'K'}
                                                            </Text>
                                                        </Td>
                                                    </Tr>
                                                </Tbody>
                                            </Table>
                                        </Box>
                                    </Box>
                                </VStack>
                            );
                        })()}
                    </ModalBody>
                    <ModalFooter borderTopWidth="1px">
                        <Button onClick={onDetailClose} colorScheme="cyan">Tutup</Button>
                    </ModalFooter>
                </ModalContent>
            </Modal>

        </UnifiedLayout>
    );
};

export default TrialBalancePage;
