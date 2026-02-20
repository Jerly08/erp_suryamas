'use client';

import React, { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useTranslation } from '@/hooks/useTranslation';
import UnifiedLayout from '@/components/layout/UnifiedLayout';
import { ssotSalesSummaryService, SSOTSalesSummaryData, SalesSummaryItem, SaleItemDetail } from '@/services/ssotSalesSummaryService';
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
    FiArrowLeft, FiShoppingCart, FiFileText, FiFilePlus, FiCalendar, FiClock,
    FiInfo, FiUsers, FiPackage, FiTrendingUp, FiExternalLink, FiHash,
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

const SalesSummaryPage: React.FC = () => {
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
    const [data, setData] = useState<SSOTSalesSummaryData | null>(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [closedPeriods, setClosedPeriods] = useState<any[]>([]);
    const [loadingClosedPeriods, setLoadingClosedPeriods] = useState(false);
    const [companyInfo, setCompanyInfo] = useState<{ name?: string; address?: string; city?: string } | null>(null);
    const [selectedItem, setSelectedItem] = useState<SalesSummaryItem | null>(null);
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
            const result = await ssotSalesSummaryService.generateSSOTSalesSummary({
                start_date: startDate, end_date: endDate, format: 'json'
            });
            setData(result);
            if (result.company) {
                const cmp = result.company as any;
                setCompanyInfo(prev => ({ name: cmp.name || prev?.name, address: cmp.address || prev?.address, city: cmp.city || prev?.city }));
            }
            toast({ title: t('reports.messages.success') || 'Berhasil', description: 'Laporan penjualan berhasil dimuat', status: 'success', duration: 3000, isClosable: true });
        } catch (err: any) {
            setError(err.message);
            toast({ title: t('reports.messages.error') || 'Gagal', description: err.message, status: 'error', duration: 5000, isClosable: true });
        } finally { setLoading(false); }
    };

    const handleExport = async (format: 'pdf' | 'csv') => {
        if (!data) {
            toast({ title: 'Error', description: 'Tidak ada data untuk diekspor', status: 'warning', duration: 3000, isClosable: true });
            return;
        }
        try {
            if (format === 'pdf') {
                // PDF: use server-side generation via GET endpoint
                const queryParams = new URLSearchParams({
                    start_date: startDate,
                    end_date: endDate,
                    format: 'pdf'
                });
                const url = `${API_ENDPOINTS.SSOT_REPORTS.SALES_SUMMARY}?${queryParams.toString()}`;

                const response = await fetch(url, {
                    method: 'GET',
                    headers: {
                        'Authorization': `Bearer ${localStorage.getItem('token')}`,
                    },
                });

                if (!response.ok) {
                    let errorMessage = `HTTP ${response.status}: ${response.statusText}`;
                    try {
                        const errorData = await response.json();
                        errorMessage = errorData.message || errorMessage;
                    } catch { /* ignore */ }
                    throw new Error(errorMessage);
                }

                const blob = await response.blob();
                if (blob.size === 0) throw new Error('Empty file received from server');

                const downloadUrl = URL.createObjectURL(blob);
                const link = document.createElement('a');
                link.href = downloadUrl;
                link.download = `Ringkasan_Penjualan_${startDate}_to_${endDate}.pdf`;
                document.body.appendChild(link);
                link.click();
                document.body.removeChild(link);
                URL.revokeObjectURL(downloadUrl);
            } else {
                // CSV: generate client-side from loaded data — enhanced with BOM & company header
                const bom = '\uFEFF';
                const rows: string[][] = [];

                // Company header
                rows.push([companyInfo?.name || 'Perusahaan']);
                if (companyInfo?.address) rows.push([companyInfo.address]);
                rows.push([]);
                rows.push(['RINGKASAN PENJUALAN']);
                rows.push([`Periode: ${startDate} s/d ${endDate}`]);
                rows.push([`Dicetak: ${new Date().toLocaleDateString('id-ID', { day: '2-digit', month: 'long', year: 'numeric' })}`]);
                rows.push([]);

                // Summary
                rows.push(['RINGKASAN']);
                rows.push(['Total Transaksi', String(data.total_transactions || 0)]);
                rows.push(['Total Penjualan', String(data.total_sales || 0)]);
                rows.push(['Total Diskon', String(data.total_discounts || 0)]);
                rows.push(['Total Retur', String(data.total_returns || 0)]);
                rows.push(['Pendapatan Bersih', String(data.net_revenue || 0)]);
                rows.push([]);
                // Data table
                rows.push(['Tanggal', 'Kode', 'Pelanggan', 'Jumlah', 'Dibayar', 'Sisa', 'Status']);
                if (data.items?.length) {
                    data.items.forEach(i => rows.push([
                        i.date, i.code, i.customer_name,
                        String(i.total_amount), String((i as any).paid_amount || 0),
                        String((i as any).outstanding || 0), i.status
                    ]));
                }

                const csv = bom + rows.map(r => r.map(c => `"${String(c).replace(/"/g, '""')}"`).join(',')).join('\n');
                const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
                const downloadUrl = URL.createObjectURL(blob);
                const link = document.createElement('a');
                link.href = downloadUrl;
                link.download = `Ringkasan_Penjualan_${startDate}_to_${endDate}.csv`;
                document.body.appendChild(link);
                link.click();
                document.body.removeChild(link);
                URL.revokeObjectURL(downloadUrl);
            }
            toast({ title: 'Ekspor Berhasil', status: 'success', duration: 3000, isClosable: true });
        } catch (err: any) {
            console.error('Export error:', err);
            toast({ title: 'Gagal Ekspor', description: err.message, status: 'error', duration: 5000, isClosable: true });
        }
    };

    const formatCurrency = (amount: number | null | undefined) => {
        if (amount === null || amount === undefined || isNaN(Number(amount))) return 'Rp 0';
        return new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', minimumFractionDigits: 0, maximumFractionDigits: 0 }).format(Number(amount));
    };

    const totalSales = data ? (data.total_sales || data.total_revenue || 0) : 0;

    return (
        <UnifiedLayout>
            <VStack spacing={6} align="stretch">
                {/* Header */}
                <HStack spacing={4}>
                    <IconButton icon={<FiArrowLeft />} variant="outline" onClick={() => router.push('/reports')} aria-label="Kembali" size="md" />
                    <VStack align="start" spacing={0}>
                        <Heading as="h1" size="lg">
                            <HStack>
                                <Icon as={FiShoppingCart} color="purple.500" />
                                <Text>{t('reports.salesSummaryReport') || 'Ringkasan Laporan Penjualan'}</Text>
                            </HStack>
                        </Heading>
                        <Text fontSize="sm" color={descriptionColor}>
                            {t('reports.description.salesSummary') || 'Menyediakan ringkasan transaksi penjualan selama periode tertentu.'}
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
                                <Button colorScheme="blue" onClick={fetchReport} isLoading={loading} leftIcon={<FiShoppingCart />} size="md" mt={8}>
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
                            <Spinner size="xl" thickness="4px" speed="0.65s" color="purple.500" />
                            <Text fontSize="lg" fontWeight="medium">Menghasilkan Laporan Penjualan...</Text>
                            <Text fontSize="sm" color={descriptionColor}>Menganalisis data transaksi penjualan</Text>
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
                        <Box bg="purple.50" p={4} borderRadius="md">
                            <HStack justify="space-between" align="start">
                                <VStack align="start" spacing={1}>
                                    <Text fontSize="lg" fontWeight="bold" color="purple.800">
                                        {companyInfo?.name || data.company?.name || 'Perusahaan'}
                                    </Text>
                                    <Text fontSize="sm" color="purple.600">
                                        {companyInfo?.address || data.company?.address || ''}{companyInfo?.city ? `, ${companyInfo.city}` : data.company?.city ? `, ${data.company.city}` : ''}
                                    </Text>
                                </VStack>
                                <Text fontSize="sm" color="purple.600">
                                    {t('reports.modal.currency') || 'Mata Uang'}: {data.currency || 'IDR'}
                                </Text>
                            </HStack>
                        </Box>

                        {/* Report Title */}
                        <Box textAlign="center" bg={summaryBg} p={4} borderRadius="md">
                            <Heading size="md" color={headingColor}>{t('reports.salesSummaryReport') || 'Ringkasan Laporan Penjualan'}</Heading>
                            <Text fontSize="sm" color={descriptionColor}>Periode: {startDate} — {endDate}</Text>
                        </Box>

                        {/* Summary Statistics */}
                        <SimpleGrid columns={[1, 2, 4]} spacing={4}>
                            <Box bg="green.50" p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="green.600">{formatCurrency(totalSales)}</Text>
                                <Text fontSize="sm" color="green.800">{t('reports.salesSummary.totalSales') || 'Total Penjualan'}</Text>
                            </Box>
                            <Box bg="blue.50" p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="blue.600">{data.total_transactions || 0}</Text>
                                <Text fontSize="sm" color="blue.800">{t('reports.salesSummary.totalTransactions') || 'Total Transaksi'}</Text>
                            </Box>
                            <Box bg="purple.50" p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="purple.600">{formatCurrency(data.total_tax)}</Text>
                                <Text fontSize="sm" color="purple.800">{t('reports.salesSummary.totalTax') || 'Total Pajak'}</Text>
                            </Box>
                            <Box bg="orange.50" p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="orange.600">{formatCurrency(data.net_revenue || totalSales)}</Text>
                                <Text fontSize="sm" color="orange.800">Pendapatan Bersih</Text>
                            </Box>
                        </SimpleGrid>

                        {/* ═══ SALES BY CUSTOMER ═══ */}
                        {data.sales_by_customer && data.sales_by_customer.length > 0 && (
                            <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                <HStack justify="space-between" mb={3}>
                                    <HStack>
                                        <Badge colorScheme="teal" px={2} py={1} fontSize="sm">A</Badge>
                                        <Icon as={FiUsers} color="teal.500" />
                                        <Text fontSize="md" fontWeight="bold" color={headingColor}>Penjualan per Pelanggan</Text>
                                    </HStack>
                                    <Text fontSize="sm" color={descriptionColor}>{data.sales_by_customer.length} pelanggan</Text>
                                </HStack>
                                <VStack spacing={0} align="stretch">
                                    {data.sales_by_customer.map((cust, i) => (
                                        <Box key={`cust-${i}`} py={2} px={4} _hover={{ bg: 'gray.50' }} borderRadius="md">
                                            <HStack justify="space-between" mb={1}>
                                                <HStack>
                                                    <Text fontSize="sm" fontWeight="medium" color={textColor}>{cust.customer_name || 'Pelanggan Umum'}</Text>
                                                    <Badge colorScheme="blue" size="sm">{cust.transaction_count}x</Badge>
                                                </HStack>
                                                <Text fontSize="sm" fontWeight="bold" color="green.600">{formatCurrency(cust.total_sales)}</Text>
                                            </HStack>
                                            <HStack justify="space-between">
                                                <Progress
                                                    value={cust.percentage_of_total || 0}
                                                    size="xs"
                                                    colorScheme="teal"
                                                    flex="1"
                                                    mr={4}
                                                    borderRadius="full"
                                                />
                                                <Text fontSize="xs" color={descriptionColor} minW="50px" textAlign="right">
                                                    {(cust.percentage_of_total || 0).toFixed(1)}%
                                                </Text>
                                            </HStack>
                                            {/* Detail items per customer */}
                                            {cust.items && cust.items.length > 0 && (
                                                <VStack spacing={0} align="stretch" mt={2} pl={4} borderLeftWidth="2px" borderColor="teal.200">
                                                    {cust.items.map((item, j) => (
                                                        <HStack key={`ci-${j}`} justify="space-between" py={1}>
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
                                    ))}
                                </VStack>
                            </Box>
                        )}

                        {/* ═══ SALES BY PRODUCT ═══ */}
                        {data.sales_by_product && data.sales_by_product.length > 0 && (
                            <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                <HStack justify="space-between" mb={3}>
                                    <HStack>
                                        <Badge colorScheme="purple" px={2} py={1} fontSize="sm">B</Badge>
                                        <Icon as={FiPackage} color="purple.500" />
                                        <Text fontSize="md" fontWeight="bold" color={headingColor}>Penjualan per Produk/Jasa</Text>
                                    </HStack>
                                    <Text fontSize="sm" color={descriptionColor}>{data.sales_by_product.length} produk</Text>
                                </HStack>
                                <TableContainer>
                                    <Table size="sm" variant="simple">
                                        <Thead bg="purple.50">
                                            <Tr>
                                                <Th>Produk/Jasa</Th>
                                                <Th isNumeric>Qty Terjual</Th>
                                                <Th isNumeric>Rata-rata Harga</Th>
                                                <Th isNumeric>Total Pendapatan</Th>
                                                <Th isNumeric>% Total</Th>
                                            </Tr>
                                        </Thead>
                                        <Tbody>
                                            {data.sales_by_product.map((prod, i) => (
                                                <Tr key={`prod-${i}`} _hover={{ bg: 'gray.50' }}>
                                                    <Td><Text fontSize="sm" fontWeight="medium">{prod.product_name}</Text></Td>
                                                    <Td isNumeric><Badge colorScheme="blue">{prod.quantity_sold}</Badge></Td>
                                                    <Td isNumeric fontSize="sm">{formatCurrency(prod.average_price)}</Td>
                                                    <Td isNumeric fontWeight="bold" color="green.600" fontSize="sm">{formatCurrency(prod.total_revenue)}</Td>
                                                    <Td isNumeric fontSize="sm">{(prod.percentage_of_total || 0).toFixed(1)}%</Td>
                                                </Tr>
                                            ))}
                                        </Tbody>
                                    </Table>
                                </TableContainer>
                            </Box>
                        )}

                        {/* ═══ SALES BY PERIOD ═══ */}
                        {data.sales_by_period && data.sales_by_period.length > 0 && (
                            <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                <HStack justify="space-between" mb={3}>
                                    <HStack>
                                        <Badge colorScheme="green" px={2} py={1} fontSize="sm">C</Badge>
                                        <Icon as={FiTrendingUp} color="green.500" />
                                        <Text fontSize="md" fontWeight="bold" color={headingColor}>Penjualan per Periode</Text>
                                    </HStack>
                                </HStack>
                                <VStack spacing={0} align="stretch">
                                    {data.sales_by_period.map((period, i) => (
                                        <HStack key={`period-${i}`} justify="space-between" py={2} px={4} _hover={{ bg: 'gray.50' }} borderRadius="md">
                                            <HStack>
                                                <Text fontSize="sm" fontWeight="medium" color={textColor}>{period.period}</Text>
                                                <Badge colorScheme="blue" size="sm">{period.transaction_count} transaksi</Badge>
                                            </HStack>
                                            <HStack spacing={4}>
                                                {period.growth_rate !== 0 && (
                                                    <Badge colorScheme={period.growth_rate > 0 ? 'green' : 'red'}>
                                                        {period.growth_rate > 0 ? '+' : ''}{period.growth_rate.toFixed(1)}%
                                                    </Badge>
                                                )}
                                                <Text fontSize="sm" fontWeight="bold" color="green.600" minW="120px" textAlign="right">
                                                    {formatCurrency(period.total_sales)}
                                                </Text>
                                            </HStack>
                                        </HStack>
                                    ))}
                                </VStack>
                            </Box>
                        )}

                        {/* ═══ TRANSACTION LIST ═══ */}
                        {data.items && data.items.length > 0 && (
                            <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                <HStack justify="space-between" mb={3}>
                                    <HStack>
                                        <Badge colorScheme="orange" px={2} py={1} fontSize="sm">D</Badge>
                                        <Text fontSize="md" fontWeight="bold" color={headingColor}>Daftar Transaksi Penjualan</Text>
                                        <Tooltip label="Daftar lengkap semua transaksi penjualan pada periode">
                                            <Box><Icon as={FiInfo} color="gray.400" /></Box>
                                        </Tooltip>
                                    </HStack>
                                    <Badge colorScheme="blue">{data.items.length} transaksi</Badge>
                                </HStack>
                                <TableContainer>
                                    <Table size="sm" variant="simple">
                                        <Thead bg="orange.50">
                                            <Tr>
                                                <Th>{t('reports.salesSummary.date') || 'Tanggal'}</Th>
                                                <Th>{t('reports.salesSummary.code') || 'No. Faktur'}</Th>
                                                <Th>{t('reports.salesSummary.customer') || 'Pelanggan'}</Th>
                                                <Th isNumeric>{t('reports.salesSummary.amount') || 'Jumlah'}</Th>
                                                <Th>{t('reports.salesSummary.status') || 'Status'}</Th>
                                                <Th width="40px"></Th>
                                            </Tr>
                                        </Thead>
                                        <Tbody>
                                            {data.items.map((item: any, idx: number) => (
                                                <Tr key={idx} _hover={{ bg: 'orange.50', cursor: 'pointer' }} onClick={() => { setSelectedItem(item); onDetailOpen(); }}>
                                                    <Td fontSize="sm">{item.date ? new Date(item.date).toLocaleDateString('id-ID') : '-'}</Td>
                                                    <Td><Badge colorScheme="blue" fontSize="xs" cursor="pointer">{item.code || '-'}</Badge></Td>
                                                    <Td fontSize="sm">{item.customer_name || '-'}</Td>
                                                    <Td isNumeric fontWeight="bold" color="green.600" fontSize="sm">{formatCurrency(item.total_amount)}</Td>
                                                    <Td>
                                                        <Badge colorScheme={
                                                            item.status === 'PAID' ? 'green' :
                                                                item.status === 'UNPAID' ? 'red' :
                                                                    item.status === 'PARTIAL' ? 'yellow' : 'gray'
                                                        }>
                                                            {item.status || '-'}
                                                        </Badge>
                                                    </Td>
                                                    <Td>
                                                        <IconButton
                                                            icon={<FiExternalLink />}
                                                            aria-label="Detail"
                                                            size="xs"
                                                            variant="ghost"
                                                            colorScheme="blue"
                                                            onClick={(e) => { e.stopPropagation(); setSelectedItem(item); onDetailOpen(); }}
                                                        />
                                                    </Td>
                                                </Tr>
                                            ))}
                                        </Tbody>
                                    </Table>
                                </TableContainer>
                            </Box>
                        )}

                        {/* ═══ SUMMARY BOX ═══ */}
                        <Box bg="green.50" p={4} borderRadius="md" border="1px" borderColor="green.200">
                            <Text fontSize="md" fontWeight="bold" color="green.800" mb={3}>
                                Ringkasan Penjualan
                            </Text>
                            <SimpleGrid columns={[1, 2]} spacing={3}>
                                <VStack spacing={2}>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color="green.700">Total Penjualan (Bruto):</Text>
                                        <Text fontSize="sm" fontWeight="semibold">{formatCurrency(totalSales)}</Text>
                                    </HStack>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color="green.700">Total Diskon:</Text>
                                        <Text fontSize="sm" fontWeight="semibold" color="red.500">-{formatCurrency(data.total_discount || data.total_discounts || 0)}</Text>
                                    </HStack>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color="green.700">Total Retur:</Text>
                                        <Text fontSize="sm" fontWeight="semibold" color="red.500">-{formatCurrency(data.total_returns || 0)}</Text>
                                    </HStack>
                                </VStack>
                                <VStack spacing={2}>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color="green.700">Total Pajak (PPN):</Text>
                                        <Text fontSize="sm" fontWeight="semibold">{formatCurrency(data.total_tax || 0)}</Text>
                                    </HStack>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color="green.700">Jumlah Transaksi:</Text>
                                        <Text fontSize="sm" fontWeight="semibold">{data.total_transactions || 0} transaksi</Text>
                                    </HStack>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color="green.700" fontWeight="bold">Pendapatan Bersih:</Text>
                                        <Text fontSize="md" fontWeight="bold" color="green.700">{formatCurrency(data.net_revenue || totalSales)}</Text>
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

            {/* ═══ DETAIL MODAL ═══ */}
            <Modal isOpen={isDetailOpen} onClose={onDetailClose} size="4xl" scrollBehavior="inside">
                <ModalOverlay bg="blackAlpha.600" backdropFilter="blur(4px)" />
                <ModalContent borderRadius="xl" overflow="hidden">
                    <ModalHeader
                        bg="linear-gradient(135deg, #667eea 0%, #764ba2 100%)"
                        color="white"
                        py={4}
                    >
                        <HStack spacing={3}>
                            <Icon as={FiShoppingCart} boxSize={5} />
                            <Box>
                                <Text fontSize="lg" fontWeight="bold">Detail Transaksi Penjualan</Text>
                                {selectedItem && (
                                    <Text fontSize="sm" fontWeight="normal" opacity={0.9}>
                                        {selectedItem.code || 'No Invoice'}
                                    </Text>
                                )}
                            </Box>
                        </HStack>
                    </ModalHeader>
                    <ModalCloseButton color="white" />
                    <ModalBody py={6}>
                        {selectedItem && (
                            <VStack spacing={5} align="stretch">
                                {/* Header Info Card */}
                                <Box bg="gray.50" p={4} borderRadius="lg" borderWidth="1px" borderColor="gray.200">
                                    <Grid templateColumns="repeat(2, 1fr)" gap={4}>
                                        <GridItem>
                                            <Text fontSize="xs" color="gray.500" mb={0.5}>Tanggal</Text>
                                            <Text fontWeight="bold">
                                                {selectedItem.date
                                                    ? new Date(selectedItem.date).toLocaleDateString('id-ID', {
                                                        weekday: 'long', year: 'numeric', month: 'long', day: 'numeric'
                                                    })
                                                    : '-'}
                                            </Text>
                                        </GridItem>
                                        <GridItem>
                                            <Text fontSize="xs" color="gray.500" mb={0.5}>No. Faktur</Text>
                                            <HStack>
                                                <Icon as={FiHash} color="blue.400" />
                                                <Badge colorScheme="blue" fontSize="sm" px={2} py={0.5}>{selectedItem.code || '-'}</Badge>
                                            </HStack>
                                        </GridItem>
                                        <GridItem>
                                            <Text fontSize="xs" color="gray.500" mb={0.5}>Pelanggan</Text>
                                            <HStack>
                                                <Icon as={FiUsers} color="purple.400" />
                                                <Text fontWeight="medium">{selectedItem.customer_name || '-'}</Text>
                                            </HStack>
                                        </GridItem>
                                        <GridItem>
                                            <Text fontSize="xs" color="gray.500" mb={0.5}>Status</Text>
                                            <Badge
                                                colorScheme={
                                                    selectedItem.status === 'PAID' ? 'green' :
                                                        selectedItem.status === 'UNPAID' ? 'red' :
                                                            selectedItem.status === 'PARTIAL' ? 'yellow' : 'gray'
                                                }
                                                fontSize="sm" px={3} py={1}
                                            >
                                                {selectedItem.status || '-'}
                                            </Badge>
                                        </GridItem>
                                        <GridItem colSpan={2}>
                                            <Divider my={1} />
                                            <HStack justify="space-between" mt={2}>
                                                <Text fontSize="sm" color="gray.600" fontWeight="medium">Total Transaksi</Text>
                                                <Text fontSize="xl" fontWeight="bold" color="green.600">
                                                    {formatCurrency(selectedItem.total_amount)}
                                                </Text>
                                            </HStack>
                                        </GridItem>
                                    </Grid>
                                </Box>

                                {/* Product Detail Table */}
                                {(() => {
                                    // Find matching products from customer data
                                    const matchedItems: SaleItemDetail[] = [];
                                    if (data?.sales_by_customer) {
                                        for (const cust of data.sales_by_customer) {
                                            if (cust.customer_name === selectedItem.customer_name && cust.items) {
                                                for (const item of cust.items) {
                                                    if (item.invoice_number === selectedItem.code) {
                                                        matchedItems.push(item);
                                                    }
                                                }
                                            }
                                        }
                                    }

                                    if (matchedItems.length > 0) {
                                        return (
                                            <Box>
                                                <HStack mb={3}>
                                                    <Icon as={FiPackage} color="orange.500" />
                                                    <Text fontWeight="bold" color="gray.700">Detail Produk</Text>
                                                    <Badge colorScheme="orange">{matchedItems.length} item</Badge>
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
                                                            {matchedItems.map((prod, i) => (
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
                                                                    <Td isNumeric fontWeight="bold" fontSize="sm" color="green.600">{formatCurrency(prod.total_price)}</Td>
                                                                </Tr>
                                                            ))}
                                                            {/* Total Row */}
                                                            <Tr bg="green.50" fontWeight="bold">
                                                                <Td colSpan={4} textAlign="right" fontSize="sm">Total</Td>
                                                                <Td isNumeric color="green.700" fontSize="sm">
                                                                    {formatCurrency(matchedItems.reduce((sum, p) => sum + p.total_price, 0))}
                                                                </Td>
                                                            </Tr>
                                                        </Tbody>
                                                    </Table>
                                                </Box>
                                            </Box>
                                        );
                                    }

                                    return (
                                        <Box bg="gray.50" p={6} borderRadius="lg" textAlign="center">
                                            <Icon as={FiPackage} boxSize={8} color="gray.300" mb={2} />
                                            <Text color="gray.500" fontSize="sm">Detail produk tidak tersedia untuk transaksi ini</Text>
                                        </Box>
                                    );
                                })()}
                            </VStack>
                        )}
                    </ModalBody>
                    <ModalFooter borderTopWidth="1px">
                        <Button onClick={onDetailClose} colorScheme="purple">Tutup</Button>
                    </ModalFooter>
                </ModalContent>
            </Modal>

        </UnifiedLayout>
    );
};

export default SalesSummaryPage;
