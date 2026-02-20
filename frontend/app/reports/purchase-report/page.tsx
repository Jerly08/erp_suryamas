'use client';

import React, { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useTranslation } from '@/hooks/useTranslation';
import UnifiedLayout from '@/components/layout/UnifiedLayout';
import { ssotPurchaseReportService, SSOTPurchaseReportData, VendorPurchaseSummary, PurchaseItemDetail } from '@/services/ssotPurchaseReportService';
import api from '@/services/api';
import { API_ENDPOINTS } from '@/config/api';
import {
    Box, Heading, Text, Button, VStack, HStack, Card, CardBody, Icon, SimpleGrid,
    FormControl, FormLabel, Input, Select, Spinner, useToast, useColorModeValue,
    Divider, Tooltip, IconButton, Badge, Table, Thead, Tbody, Tr, Th, Td, TableContainer,
    Progress, Modal, ModalOverlay, ModalContent, ModalHeader, ModalBody, ModalFooter,
    ModalCloseButton, Grid, GridItem, useDisclosure,
} from '@chakra-ui/react';
import {
    FiArrowLeft, FiShoppingBag, FiFileText, FiFilePlus, FiCalendar, FiClock,
    FiInfo, FiUsers, FiPackage, FiTrendingUp, FiDollarSign, FiCreditCard, FiExternalLink, FiHash,
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
            const formattedEndDate = endDateObj.toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric' });
            const rawStart = entry.start_date ? String(entry.start_date).slice(0, 10) : undefined;
            return {
                value: endDateKey,
                label: `${formattedEndDate} - ${entry.description || 'Period Closing'}`,
                date: endDateKey,
                start_date: rawStart,
                fiscal_year: endDateObj.getFullYear(),
            };
        });
    const uniqueMap = new Map<string, any>();
    mapped.forEach((p: any) => { if (!uniqueMap.has(p.date)) uniqueMap.set(p.date, p); });
    return Array.from(uniqueMap.values()).sort((a, b) => new Date(b.date).getTime() - new Date(a.date).getTime());
};

const PurchaseReportPage: React.FC = () => {
    const router = useRouter();
    const { t } = useTranslation();
    const toast = useToast();

    const cardBg = useColorModeValue('white', 'gray.800');
    const borderColor = useColorModeValue('gray.200', 'gray.600');
    const headingColor = useColorModeValue('gray.700', 'white');
    const textColor = useColorModeValue('gray.800', 'white');
    const descriptionColor = useColorModeValue('gray.600', 'gray.300');
    const summaryBg = useColorModeValue('gray.50', 'gray.700');

    const [startDate, setStartDate] = useState('');
    const [endDate, setEndDate] = useState('');
    const [data, setData] = useState<SSOTPurchaseReportData | null>(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [closedPeriods, setClosedPeriods] = useState<any[]>([]);
    const [loadingClosedPeriods, setLoadingClosedPeriods] = useState(false);
    const [companyInfo, setCompanyInfo] = useState<{ name?: string; address?: string; city?: string } | null>(null);
    const [selectedVendor, setSelectedVendor] = useState<VendorPurchaseSummary | null>(null);
    const { isOpen: isDetailOpen, onOpen: onDetailOpen, onClose: onDetailClose } = useDisclosure();

    useEffect(() => {
        const loadSettings = async () => {
            try {
                const resp = await api.get(API_ENDPOINTS.SETTINGS);
                const s = resp.data?.data;
                const range = computeFiscalRangeFromString(s?.fiscal_year_start || 'January 1');
                setStartDate(range.startISO);
                setEndDate(range.endISO);
                if (s?.company_name || s?.company_address) {
                    setCompanyInfo({ name: s.company_name, address: s.company_address, city: s.company_city });
                }
            } catch {
                const today = new Date();
                setStartDate(toISO(new Date(today.getFullYear(), 0, 1)));
                setEndDate(toISO(new Date(today.getFullYear(), 11, 31)));
            }
        };
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
            const result = await ssotPurchaseReportService.generateSSOTPurchaseReport({
                start_date: startDate, end_date: endDate, format: 'json'
            });
            setData(result);
            if (result.company) {
                setCompanyInfo(prev => ({ name: result.company?.name || prev?.name, address: result.company?.address || prev?.address, city: result.company?.city || prev?.city }));
            }
            toast({ title: 'Berhasil', description: 'Laporan pembelian berhasil dimuat', status: 'success', duration: 3000, isClosable: true });
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
            const blob = format === 'pdf'
                ? await ssotPurchaseReportService.exportToPDF({ start_date: startDate, end_date: endDate })
                : await ssotPurchaseReportService.exportToCSV({ start_date: startDate, end_date: endDate });
            const url = URL.createObjectURL(blob);
            const link = document.createElement('a');
            link.href = url;
            link.download = `Laporan_Pembelian_${startDate}_to_${endDate}.${format === 'pdf' ? 'pdf' : 'csv'}`;
            link.click();
            URL.revokeObjectURL(url);
            toast({ title: 'Ekspor Berhasil', status: 'success', duration: 3000, isClosable: true });
        } catch {
            // Fallback CSV — enhanced with BOM & company header
            if (!data.purchases_by_vendor?.length) return;
            const bom = '\uFEFF';
            const rows: string[][] = [];

            // Company header
            rows.push([companyInfo?.name || 'Perusahaan']);
            if (companyInfo?.address) rows.push([companyInfo.address]);
            rows.push([]);
            rows.push(['LAPORAN PEMBELIAN']);
            rows.push([`Periode: ${startDate} s/d ${endDate}`]);
            rows.push([`Dicetak: ${new Date().toLocaleDateString('id-ID', { day: '2-digit', month: 'long', year: 'numeric' })}`]);
            rows.push([]);

            // Summary
            rows.push(['RINGKASAN']);
            rows.push(['Total Transaksi', String(data.total_purchases || 0)]);
            rows.push(['Total Pembelian', String(data.total_amount || 0)]);
            rows.push(['Total Dibayar', String(data.total_paid || 0)]);
            rows.push(['Total Sisa', String(data.outstanding_payables || 0)]);
            rows.push([]);

            // Data table
            rows.push(['Vendor', 'Jumlah Transaksi', 'Total Pembelian', 'Dibayar', 'Sisa', 'Status']);
            data.purchases_by_vendor.forEach(v => rows.push([
                v.vendor_name, String(v.total_purchases), String(v.total_amount),
                String(v.total_paid), String(v.outstanding), v.status
            ]));

            // Totals
            rows.push([]);
            const totalAmount = data.purchases_by_vendor.reduce((s, v) => s + (v.total_amount || 0), 0);
            const totalPaid = data.purchases_by_vendor.reduce((s, v) => s + (v.total_paid || 0), 0);
            const totalOutstanding = data.purchases_by_vendor.reduce((s, v) => s + (v.outstanding || 0), 0);
            rows.push(['TOTAL', String(data.purchases_by_vendor.length), String(totalAmount), String(totalPaid), String(totalOutstanding), '']);

            const csv = bom + rows.map(r => r.map(c => `"${String(c).replace(/"/g, '""')}"`).join(',')).join('\n');
            const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
            const url = URL.createObjectURL(blob);
            const link = document.createElement('a');
            link.href = url;
            link.download = `Laporan_Pembelian_${startDate}_to_${endDate}.csv`;
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
            URL.revokeObjectURL(url);
            toast({ title: 'Ekspor CSV Berhasil', status: 'success', duration: 3000, isClosable: true });
        }
    };

    const formatCurrency = (amount: number | null | undefined) => {
        if (amount === null || amount === undefined || isNaN(Number(amount))) return 'Rp 0';
        return new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', minimumFractionDigits: 0, maximumFractionDigits: 0 }).format(Number(amount));
    };

    return (
        <UnifiedLayout>
            <VStack spacing={6} align="stretch">
                {/* Header */}
                <HStack spacing={4}>
                    <IconButton icon={<FiArrowLeft />} variant="outline" onClick={() => router.push('/reports')} aria-label="Kembali" size="md" />
                    <VStack align="start" spacing={0}>
                        <Heading as="h1" size="lg">
                            <HStack>
                                <Icon as={FiShoppingBag} color="orange.500" />
                                <Text>{t('reports.purchaseReport') || 'Laporan Pembelian'}</Text>
                            </HStack>
                        </Heading>
                        <Text fontSize="sm" color={descriptionColor}>
                            {t('reports.description.purchaseReport') || 'Menyediakan ringkasan transaksi pembelian selama periode tertentu.'}
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
                                            <Tooltip label="Pilih periode tutup untuk memuat laporan berdasarkan periode fiskal yang telah ditutup" placement="top">
                                                <Box><Icon as={FiInfo} color="gray.500" /></Box>
                                            </Tooltip>
                                        </HStack>
                                    </FormLabel>
                                    <Select
                                        placeholder={t('reports.selectClosedPeriod') || 'Pilih periode tutup'}
                                        size="md"
                                        onChange={(e) => {
                                            if (e.target.value) {
                                                const sel = closedPeriods.find(p => p.date === e.target.value);
                                                if (sel) {
                                                    setStartDate(sel.start_date || new Date(new Date(sel.date).getFullYear(), 0, 1).toISOString().split('T')[0]);
                                                    setEndDate(sel.date);
                                                }
                                            }
                                        }}
                                        isDisabled={loadingClosedPeriods}
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

                            {/* Date Range */}
                            <HStack spacing={4} flexWrap="wrap">
                                <FormControl flex="1" minW="200px">
                                    <FormLabel>{t('reports.modal.startDate') || 'Tanggal Mulai'}</FormLabel>
                                    <Input type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} />
                                </FormControl>
                                <FormControl flex="1" minW="200px">
                                    <FormLabel>{t('reports.modal.endDate') || 'Tanggal Akhir'}</FormLabel>
                                    <Input type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)} />
                                </FormControl>
                                <Button colorScheme="blue" onClick={fetchReport} isLoading={loading} leftIcon={<FiShoppingBag />} size="md" mt={8}>
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
                            <Spinner size="xl" thickness="4px" speed="0.65s" color="orange.500" />
                            <Text fontSize="lg" fontWeight="medium">Menghasilkan Laporan Pembelian...</Text>
                            <Text fontSize="sm" color={descriptionColor}>Menganalisis data transaksi pembelian</Text>
                        </VStack>
                    </Box>
                )}

                {/* Error */}
                {error && (
                    <Box bg="red.50" p={4} borderRadius="md">
                        <Text color="red.600" fontWeight="medium">Error: {error}</Text>
                        <Button mt={2} size="sm" colorScheme="red" variant="outline" onClick={fetchReport}>Coba Lagi</Button>
                    </Box>
                )}

                {/* Report Data */}
                {data && !loading && (
                    <VStack spacing={6} align="stretch">
                        {/* Company Header */}
                        <Box bg="orange.50" p={4} borderRadius="md">
                            <HStack justify="space-between" align="start">
                                <VStack align="start" spacing={1}>
                                    <Text fontSize="lg" fontWeight="bold" color="orange.800">
                                        {companyInfo?.name || data.company?.name || 'Perusahaan'}
                                    </Text>
                                    <Text fontSize="sm" color="orange.600">
                                        {companyInfo?.address || data.company?.address || ''}{companyInfo?.city ? `, ${companyInfo.city}` : data.company?.city ? `, ${data.company.city}` : ''}
                                    </Text>
                                </VStack>
                                <Text fontSize="sm" color="orange.600">
                                    Mata Uang: {data.currency || 'IDR'}
                                </Text>
                            </HStack>
                        </Box>

                        {/* Report Title */}
                        <Box textAlign="center" bg={summaryBg} p={4} borderRadius="md">
                            <Heading size="md" color={headingColor}>{t('reports.purchaseReport') || 'Laporan Pembelian'}</Heading>
                            <Text fontSize="sm" color={descriptionColor}>Periode: {startDate} — {endDate}</Text>
                        </Box>

                        {/* Summary Statistics */}
                        <SimpleGrid columns={[1, 2, 4]} spacing={4}>
                            <Box bg="orange.50" p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="orange.600">{formatCurrency(data.total_amount)}</Text>
                                <Text fontSize="sm" color="orange.800">Total Pembelian</Text>
                            </Box>
                            <Box bg="blue.50" p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="blue.600">{data.total_purchases || 0}</Text>
                                <Text fontSize="sm" color="blue.800">Total Transaksi</Text>
                            </Box>
                            <Box bg="green.50" p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="green.600">{formatCurrency(data.total_paid)}</Text>
                                <Text fontSize="sm" color="green.800">Total Dibayar</Text>
                            </Box>
                            <Box bg="red.50" p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="red.600">{formatCurrency(data.outstanding_payables)}</Text>
                                <Text fontSize="sm" color="red.800">Hutang Belum Dibayar</Text>
                            </Box>
                        </SimpleGrid>

                        {/* ═══ PURCHASES BY VENDOR ═══ */}
                        {data.purchases_by_vendor && data.purchases_by_vendor.length > 0 && (
                            <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                <HStack justify="space-between" mb={3}>
                                    <HStack>
                                        <Badge colorScheme="orange" px={2} py={1} fontSize="sm">A</Badge>
                                        <Icon as={FiUsers} color="orange.500" />
                                        <Text fontSize="md" fontWeight="bold" color={headingColor}>Pembelian per Vendor/Pemasok</Text>
                                    </HStack>
                                    <Text fontSize="sm" color={descriptionColor}>{data.purchases_by_vendor.length} vendor</Text>
                                </HStack>
                                <VStack spacing={0} align="stretch">
                                    {data.purchases_by_vendor.map((vendor, i) => {
                                        const pctOfTotal = data.total_amount > 0 ? (vendor.total_amount / data.total_amount) * 100 : 0;
                                        return (
                                            <Box key={`vendor-${i}`} py={2} px={4} _hover={{ bg: 'orange.50', cursor: 'pointer' }} borderRadius="md" onClick={() => { setSelectedVendor(vendor); onDetailOpen(); }}>
                                                <HStack justify="space-between" mb={1}>
                                                    <HStack>
                                                        <Text fontSize="sm" fontWeight="medium" color={textColor}>{vendor.vendor_name || 'Vendor Umum'}</Text>
                                                        <Badge colorScheme="blue" size="sm">{vendor.total_purchases}x</Badge>
                                                        <Badge colorScheme={vendor.status === 'COMPLETED' ? 'green' : vendor.status === 'PENDING' ? 'yellow' : 'gray'}>
                                                            {vendor.status || '-'}
                                                        </Badge>
                                                    </HStack>
                                                    <HStack spacing={2}>
                                                        <Text fontSize="sm" fontWeight="bold" color="orange.600">{formatCurrency(vendor.total_amount)}</Text>
                                                        <Tooltip label="Lihat Detail" placement="top">
                                                            <IconButton
                                                                icon={<FiExternalLink />}
                                                                aria-label="Detail"
                                                                size="xs"
                                                                variant="ghost"
                                                                colorScheme="orange"
                                                                onClick={(e) => { e.stopPropagation(); setSelectedVendor(vendor); onDetailOpen(); }}
                                                            />
                                                        </Tooltip>
                                                    </HStack>
                                                </HStack>
                                                <HStack justify="space-between" mb={1}>
                                                    <HStack spacing={4}>
                                                        <Text fontSize="xs" color="green.600">Dibayar: {formatCurrency(vendor.total_paid)}</Text>
                                                        {vendor.outstanding > 0 && (
                                                            <Text fontSize="xs" color="red.600">Sisa: {formatCurrency(vendor.outstanding)}</Text>
                                                        )}
                                                    </HStack>
                                                </HStack>
                                                <HStack justify="space-between">
                                                    <Progress
                                                        value={pctOfTotal}
                                                        size="xs"
                                                        colorScheme="orange"
                                                        flex="1"
                                                        mr={4}
                                                        borderRadius="full"
                                                    />
                                                    <Text fontSize="xs" color={descriptionColor} minW="50px" textAlign="right">
                                                        {pctOfTotal.toFixed(1)}%
                                                    </Text>
                                                </HStack>
                                                {/* Detail items per vendor */}
                                                {vendor.items && vendor.items.length > 0 && (
                                                    <VStack spacing={0} align="stretch" mt={2} pl={4} borderLeftWidth="2px" borderColor="orange.200">
                                                        {vendor.items.map((item, j) => (
                                                            <HStack key={`vi-${j}`} justify="space-between" py={1}>
                                                                <HStack>
                                                                    <Text fontSize="xs" color="gray.500" fontFamily="mono">{item.product_code || '-'}</Text>
                                                                    <Text fontSize="xs" color={descriptionColor}>{item.product_name}</Text>
                                                                    <Text fontSize="xs" color="gray.400">x{item.quantity} {item.unit || ''}</Text>
                                                                </HStack>
                                                                <Text fontSize="xs" fontWeight="medium">{formatCurrency(item.total_price)}</Text>
                                                            </HStack>
                                                        ))}
                                                    </VStack>
                                                )}
                                            </Box>
                                        );
                                    })}
                                </VStack>
                            </Box>
                        )}

                        {/* ═══ PURCHASES BY CATEGORY ═══ */}
                        {data.purchases_by_category && data.purchases_by_category.length > 0 && (
                            <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                <HStack justify="space-between" mb={3}>
                                    <HStack>
                                        <Badge colorScheme="purple" px={2} py={1} fontSize="sm">B</Badge>
                                        <Icon as={FiPackage} color="purple.500" />
                                        <Text fontSize="md" fontWeight="bold" color={headingColor}>Pembelian per Kategori</Text>
                                    </HStack>
                                    <Text fontSize="sm" color={descriptionColor}>{data.purchases_by_category.length} kategori</Text>
                                </HStack>
                                <TableContainer>
                                    <Table size="sm" variant="simple">
                                        <Thead bg="purple.50">
                                            <Tr>
                                                <Th>Kategori</Th>
                                                <Th>Kode Akun</Th>
                                                <Th isNumeric>Jumlah Transaksi</Th>
                                                <Th isNumeric>Total Pembelian</Th>
                                                <Th isNumeric>% Total</Th>
                                            </Tr>
                                        </Thead>
                                        <Tbody>
                                            {data.purchases_by_category.map((cat, i) => (
                                                <Tr key={`cat-${i}`} _hover={{ bg: 'gray.50' }}>
                                                    <Td><Text fontSize="sm" fontWeight="medium">{cat.category_name || cat.account_name}</Text></Td>
                                                    <Td><Badge colorScheme="gray" fontSize="xs">{cat.account_code}</Badge></Td>
                                                    <Td isNumeric><Badge colorScheme="blue">{cat.total_purchases}</Badge></Td>
                                                    <Td isNumeric fontWeight="bold" color="orange.600" fontSize="sm">{formatCurrency(cat.total_amount)}</Td>
                                                    <Td isNumeric fontSize="sm">{(cat.percentage || 0).toFixed(1)}%</Td>
                                                </Tr>
                                            ))}
                                        </Tbody>
                                    </Table>
                                </TableContainer>
                            </Box>
                        )}

                        {/* ═══ PURCHASES BY MONTH ═══ */}
                        {data.purchases_by_month && data.purchases_by_month.length > 0 && (
                            <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                <HStack justify="space-between" mb={3}>
                                    <HStack>
                                        <Badge colorScheme="green" px={2} py={1} fontSize="sm">C</Badge>
                                        <Icon as={FiTrendingUp} color="green.500" />
                                        <Text fontSize="md" fontWeight="bold" color={headingColor}>Pembelian per Bulan</Text>
                                    </HStack>
                                </HStack>
                                <VStack spacing={0} align="stretch">
                                    {data.purchases_by_month.map((month, i) => {
                                        const prev = i > 0 ? data.purchases_by_month[i - 1].total_amount : 0;
                                        const growthRate = prev > 0 ? ((month.total_amount - prev) / prev) * 100 : 0;
                                        return (
                                            <HStack key={`month-${i}`} justify="space-between" py={2} px={4} _hover={{ bg: 'gray.50' }} borderRadius="md">
                                                <HStack>
                                                    <Text fontSize="sm" fontWeight="medium" color={textColor}>{month.month_name} {month.year}</Text>
                                                    <Badge colorScheme="blue" size="sm">{month.total_purchases} transaksi</Badge>
                                                </HStack>
                                                <HStack spacing={4}>
                                                    {i > 0 && growthRate !== 0 && (
                                                        <Badge colorScheme={growthRate > 0 ? 'red' : 'green'}>
                                                            {growthRate > 0 ? '+' : ''}{growthRate.toFixed(1)}%
                                                        </Badge>
                                                    )}
                                                    <Text fontSize="sm" fontWeight="bold" color="orange.600" minW="120px" textAlign="right">
                                                        {formatCurrency(month.total_amount)}
                                                    </Text>
                                                </HStack>
                                            </HStack>
                                        );
                                    })}
                                </VStack>
                            </Box>
                        )}

                        {/* ═══ PAYMENT ANALYSIS ═══ */}
                        {data.payment_analysis && (
                            <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                <HStack mb={3}>
                                    <Badge colorScheme="blue" px={2} py={1} fontSize="sm">D</Badge>
                                    <Icon as={FiCreditCard} color="blue.500" />
                                    <Text fontSize="md" fontWeight="bold" color={headingColor}>Analisis Pembayaran</Text>
                                </HStack>
                                <SimpleGrid columns={[1, 2, 4]} spacing={4}>
                                    <Box bg="green.50" p={3} borderRadius="md" textAlign="center">
                                        <Text fontSize="xl" fontWeight="bold" color="green.600">{data.payment_analysis.cash_purchases || 0}</Text>
                                        <Text fontSize="xs" color="green.800">Pembelian Tunai</Text>
                                        <Text fontSize="xs" color="green.600">{formatCurrency(data.payment_analysis.cash_amount)}</Text>
                                        <Text fontSize="xs" color="gray.500">({(data.payment_analysis.cash_percentage || 0).toFixed(1)}%)</Text>
                                    </Box>
                                    <Box bg="orange.50" p={3} borderRadius="md" textAlign="center">
                                        <Text fontSize="xl" fontWeight="bold" color="orange.600">{data.payment_analysis.credit_purchases || 0}</Text>
                                        <Text fontSize="xs" color="orange.800">Pembelian Kredit</Text>
                                        <Text fontSize="xs" color="orange.600">{formatCurrency(data.payment_analysis.credit_amount)}</Text>
                                        <Text fontSize="xs" color="gray.500">({(data.payment_analysis.credit_percentage || 0).toFixed(1)}%)</Text>
                                    </Box>
                                    <Box bg="blue.50" p={3} borderRadius="md" textAlign="center">
                                        <Text fontSize="xl" fontWeight="bold" color="blue.600">{formatCurrency(data.payment_analysis.average_order_value)}</Text>
                                        <Text fontSize="xs" color="blue.800">Rata-rata Pembelian</Text>
                                    </Box>
                                    <Box bg="purple.50" p={3} borderRadius="md" textAlign="center">
                                        <Text fontSize="xl" fontWeight="bold" color="purple.600">
                                            {data.total_amount > 0 ? ((data.total_paid / data.total_amount) * 100).toFixed(1) : '0'}%
                                        </Text>
                                        <Text fontSize="xs" color="purple.800">Efisiensi Pembayaran</Text>
                                    </Box>
                                </SimpleGrid>
                            </Box>
                        )}

                        {/* ═══ TAX ANALYSIS ═══ */}
                        {data.tax_analysis && (
                            <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                <HStack mb={3}>
                                    <Badge colorScheme="teal" px={2} py={1} fontSize="sm">E</Badge>
                                    <Icon as={FiDollarSign} color="teal.500" />
                                    <Text fontSize="md" fontWeight="bold" color={headingColor}>Analisis Pajak Pembelian</Text>
                                </HStack>
                                <SimpleGrid columns={[1, 2, 4]} spacing={4}>
                                    <Box bg="teal.50" p={3} borderRadius="md" textAlign="center">
                                        <Text fontSize="xl" fontWeight="bold" color="teal.600">{formatCurrency(data.tax_analysis.total_taxable_amount)}</Text>
                                        <Text fontSize="xs" color="teal.800">Nilai Kena Pajak</Text>
                                    </Box>
                                    <Box bg="red.50" p={3} borderRadius="md" textAlign="center">
                                        <Text fontSize="xl" fontWeight="bold" color="red.600">{formatCurrency(data.tax_analysis.total_tax_amount)}</Text>
                                        <Text fontSize="xs" color="red.800">Total PPN Masukan</Text>
                                    </Box>
                                    <Box bg="blue.50" p={3} borderRadius="md" textAlign="center">
                                        <Text fontSize="xl" fontWeight="bold" color="blue.600">{(data.tax_analysis.average_tax_rate || 0).toFixed(1)}%</Text>
                                        <Text fontSize="xs" color="blue.800">Rata-rata Tarif Pajak</Text>
                                    </Box>
                                    <Box bg="green.50" p={3} borderRadius="md" textAlign="center">
                                        <Text fontSize="xl" fontWeight="bold" color="green.600">{formatCurrency(data.tax_analysis.tax_reclaimable_amount)}</Text>
                                        <Text fontSize="xs" color="green.800">PPN Dapat Dikreditkan</Text>
                                    </Box>
                                </SimpleGrid>

                                {/* Monthly Tax Breakdown */}
                                {data.tax_analysis.tax_by_month && data.tax_analysis.tax_by_month.length > 0 && (
                                    <Box mt={3}>
                                        <Text fontSize="sm" fontWeight="medium" mb={2} color={descriptionColor}>PPN Masukan per Bulan:</Text>
                                        <HStack spacing={2} flexWrap="wrap">
                                            {data.tax_analysis.tax_by_month.map((tm, i) => (
                                                <Box key={`tm-${i}`} bg="teal.50" px={3} py={1} borderRadius="md" fontSize="xs">
                                                    <Text fontWeight="medium">{tm.month_name}</Text>
                                                    <Text color="teal.600">{formatCurrency(tm.tax_amount)}</Text>
                                                </Box>
                                            ))}
                                        </HStack>
                                    </Box>
                                )}
                            </Box>
                        )}

                        {/* ═══ SUMMARY BOX ═══ */}
                        <Box bg="orange.50" p={4} borderRadius="md" border="1px" borderColor="orange.200">
                            <Text fontSize="md" fontWeight="bold" color="orange.800" mb={3}>
                                Ringkasan Pembelian
                            </Text>
                            <SimpleGrid columns={[1, 2]} spacing={3}>
                                <VStack spacing={2}>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color="orange.700">Total Pembelian (Bruto):</Text>
                                        <Text fontSize="sm" fontWeight="semibold">{formatCurrency(data.total_amount)}</Text>
                                    </HStack>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color="orange.700">Total Dibayar:</Text>
                                        <Text fontSize="sm" fontWeight="semibold" color="green.600">{formatCurrency(data.total_paid)}</Text>
                                    </HStack>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color="orange.700">Hutang Belum Dibayar:</Text>
                                        <Text fontSize="sm" fontWeight="semibold" color="red.500">{formatCurrency(data.outstanding_payables)}</Text>
                                    </HStack>
                                </VStack>
                                <VStack spacing={2}>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color="orange.700">Total PPN Masukan:</Text>
                                        <Text fontSize="sm" fontWeight="semibold">{formatCurrency(data.tax_analysis?.total_tax_amount || 0)}</Text>
                                    </HStack>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color="orange.700">Jumlah Transaksi:</Text>
                                        <Text fontSize="sm" fontWeight="semibold">{data.total_purchases || 0} transaksi</Text>
                                    </HStack>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color="orange.700">Jumlah Vendor:</Text>
                                        <Text fontSize="sm" fontWeight="semibold">{data.purchases_by_vendor?.length || 0} vendor</Text>
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

            {/* ═══ VENDOR DETAIL MODAL ═══ */}
            <Modal isOpen={isDetailOpen} onClose={onDetailClose} size="4xl" scrollBehavior="inside">
                <ModalOverlay bg="blackAlpha.600" backdropFilter="blur(4px)" />
                <ModalContent borderRadius="xl" overflow="hidden">
                    <ModalHeader
                        bg="linear-gradient(135deg, #f093fb 0%, #f5576c 100%)"
                        color="white"
                        py={4}
                    >
                        <HStack spacing={3}>
                            <Icon as={FiShoppingBag} boxSize={5} />
                            <Box>
                                <Text fontSize="lg" fontWeight="bold">Detail Pembelian Vendor</Text>
                                {selectedVendor && (
                                    <Text fontSize="sm" fontWeight="normal" opacity={0.9}>
                                        {selectedVendor.vendor_name || 'Vendor'}
                                    </Text>
                                )}
                            </Box>
                        </HStack>
                    </ModalHeader>
                    <ModalCloseButton color="white" />
                    <ModalBody py={6}>
                        {selectedVendor && (
                            <VStack spacing={5} align="stretch">
                                {/* Header Info Card */}
                                <Box bg="gray.50" p={4} borderRadius="lg" borderWidth="1px" borderColor="gray.200">
                                    <Grid templateColumns="repeat(2, 1fr)" gap={4}>
                                        <GridItem>
                                            <Text fontSize="xs" color="gray.500" mb={0.5}>Vendor</Text>
                                            <HStack>
                                                <Icon as={FiUsers} color="orange.400" />
                                                <Text fontWeight="bold">{selectedVendor.vendor_name || '-'}</Text>
                                            </HStack>
                                        </GridItem>
                                        <GridItem>
                                            <Text fontSize="xs" color="gray.500" mb={0.5}>Total Transaksi</Text>
                                            <Badge colorScheme="blue" fontSize="sm" px={2} py={0.5}>{selectedVendor.total_purchases}x</Badge>
                                        </GridItem>
                                        <GridItem>
                                            <Text fontSize="xs" color="gray.500" mb={0.5}>Metode Pembayaran</Text>
                                            <Text fontWeight="medium">{selectedVendor.payment_method || '-'}</Text>
                                        </GridItem>
                                        <GridItem>
                                            <Text fontSize="xs" color="gray.500" mb={0.5}>Status</Text>
                                            <Badge
                                                colorScheme={
                                                    selectedVendor.status === 'COMPLETED' ? 'green' :
                                                        selectedVendor.status === 'PENDING' ? 'yellow' : 'gray'
                                                }
                                                fontSize="sm" px={3} py={1}
                                            >
                                                {selectedVendor.status || '-'}
                                            </Badge>
                                        </GridItem>
                                        <GridItem colSpan={2}>
                                            <Divider my={1} />
                                            <SimpleGrid columns={3} spacing={4} mt={2}>
                                                <Box>
                                                    <Text fontSize="xs" color="gray.500">Total Pembelian</Text>
                                                    <Text fontSize="lg" fontWeight="bold" color="orange.600">
                                                        {formatCurrency(selectedVendor.total_amount)}
                                                    </Text>
                                                </Box>
                                                <Box>
                                                    <Text fontSize="xs" color="gray.500">Dibayar</Text>
                                                    <Text fontSize="lg" fontWeight="bold" color="green.600">
                                                        {formatCurrency(selectedVendor.total_paid)}
                                                    </Text>
                                                </Box>
                                                <Box>
                                                    <Text fontSize="xs" color="gray.500">Sisa Hutang</Text>
                                                    <Text fontSize="lg" fontWeight="bold" color={selectedVendor.outstanding > 0 ? 'red.600' : 'gray.400'}>
                                                        {formatCurrency(selectedVendor.outstanding)}
                                                    </Text>
                                                </Box>
                                            </SimpleGrid>
                                        </GridItem>
                                    </Grid>
                                </Box>

                                {/* Product Detail Table */}
                                {selectedVendor.items && selectedVendor.items.length > 0 ? (
                                    <Box>
                                        <HStack mb={3}>
                                            <Icon as={FiPackage} color="orange.500" />
                                            <Text fontWeight="bold" color="gray.700">Detail Produk</Text>
                                            <Badge colorScheme="orange">{selectedVendor.items.length} item</Badge>
                                        </HStack>
                                        <Box borderWidth="1px" borderRadius="lg" overflow="hidden">
                                            <Table size="sm" variant="simple">
                                                <Thead bg="orange.50">
                                                    <Tr>
                                                        <Th>#</Th>
                                                        <Th>Produk</Th>
                                                        <Th isNumeric>Qty</Th>
                                                        <Th isNumeric>Harga Satuan</Th>
                                                        <Th isNumeric>Total</Th>
                                                    </Tr>
                                                </Thead>
                                                <Tbody>
                                                    {selectedVendor.items.map((prod, i) => (
                                                        <Tr key={i} _hover={{ bg: 'gray.50' }}>
                                                            <Td color="gray.500" fontSize="xs">{i + 1}</Td>
                                                            <Td>
                                                                <VStack align="start" spacing={0}>
                                                                    <Text fontWeight="medium" fontSize="sm">{prod.product_name}</Text>
                                                                    {prod.product_code && (
                                                                        <Text fontSize="xs" color="gray.400">{prod.product_code}</Text>
                                                                    )}
                                                                </VStack>
                                                            </Td>
                                                            <Td isNumeric fontSize="sm">{prod.quantity} {prod.unit}</Td>
                                                            <Td isNumeric fontSize="sm">{formatCurrency(prod.unit_price)}</Td>
                                                            <Td isNumeric fontWeight="bold" fontSize="sm" color="orange.600">{formatCurrency(prod.total_price)}</Td>
                                                        </Tr>
                                                    ))}
                                                    {/* Total Row */}
                                                    <Tr bg="orange.50" fontWeight="bold">
                                                        <Td colSpan={4} textAlign="right" fontSize="sm">Total</Td>
                                                        <Td isNumeric color="orange.700" fontSize="sm">
                                                            {formatCurrency(selectedVendor.items.reduce((sum, p) => sum + p.total_price, 0))}
                                                        </Td>
                                                    </Tr>
                                                </Tbody>
                                            </Table>
                                        </Box>
                                    </Box>
                                ) : (
                                    <Box bg="gray.50" p={6} borderRadius="lg" textAlign="center">
                                        <Icon as={FiPackage} boxSize={8} color="gray.300" mb={2} />
                                        <Text color="gray.500" fontSize="sm">Detail produk tidak tersedia untuk vendor ini</Text>
                                    </Box>
                                )}
                            </VStack>
                        )}
                    </ModalBody>
                    <ModalFooter borderTopWidth="1px">
                        <Button onClick={onDetailClose} colorScheme="orange">Tutup</Button>
                    </ModalFooter>
                </ModalContent>
            </Modal>

        </UnifiedLayout>
    );
};

export default PurchaseReportPage;
