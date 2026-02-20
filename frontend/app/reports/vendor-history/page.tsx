'use client';

import React, { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useTranslation } from '@/hooks/useTranslation';
import UnifiedLayout from '@/components/layout/UnifiedLayout';
import api from '@/services/api';
import { API_ENDPOINTS } from '@/config/api';
import {
    Box, Heading, Text, Button, VStack, HStack, Card, CardBody, Icon, SimpleGrid,
    FormControl, FormLabel, Input, Select, Spinner, useToast, useColorModeValue,
    Divider, Tooltip, IconButton, Badge, Table, Thead, Tbody, Tr, Th, Td, TableContainer, Tfoot,
    Progress,
    Modal, ModalOverlay, ModalContent, ModalHeader, ModalCloseButton, ModalBody, ModalFooter,
    useDisclosure,
} from '@chakra-ui/react';
import {
    FiArrowLeft, FiTruck, FiFileText, FiFilePlus, FiCalendar, FiClock,
    FiInfo, FiDollarSign, FiUser, FiMail, FiPhone, FiMapPin, FiHash, FiEye,
} from 'react-icons/fi';

const toISO = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;

interface Vendor { id: number; name: string; code: string; }

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

const getStatusColor = (status: string): string => {
    const s = (status || '').toUpperCase();
    if (s === 'PAID' || s === 'COMPLETED' || s === 'POSTED') return 'green';
    if (s === 'UNPAID' || s === 'OVERDUE') return 'red';
    if (s === 'PARTIAL' || s === 'PARTIALLY_PAID') return 'yellow';
    if (s === 'DRAFT') return 'gray';
    return 'blue';
};

const getTypeColor = (type: string): string => {
    switch (type) {
        case 'PURCHASE': return 'orange';
        case 'BILL': return 'blue';
        case 'PAYMENT': return 'purple';
        default: return 'gray';
    }
};
const getTypeLabel = (type: string): string => {
    switch (type) {
        case 'PURCHASE': return 'Pembelian';
        case 'BILL': return 'Tagihan';
        case 'PAYMENT': return 'Pembayaran';
        default: return type;
    }
};

const VendorHistoryPage: React.FC = () => {
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
    const [vendorId, setVendorId] = useState('');
    const [vendors, setVendors] = useState<Vendor[]>([]);
    const [data, setData] = useState<any>(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [closedPeriods, setClosedPeriods] = useState<any[]>([]);
    const [loadingClosedPeriods, setLoadingClosedPeriods] = useState(false);
    const [companyInfo, setCompanyInfo] = useState<{ name?: string; address?: string; city?: string } | null>(null);

    // Detail modal state
    const { isOpen: isDetailOpen, onOpen: onDetailOpen, onClose: onDetailClose } = useDisclosure();
    const [selectedTx, setSelectedTx] = useState<any>(null);
    const [journalDetail, setJournalDetail] = useState<any>(null);
    const [loadingDetail, setLoadingDetail] = useState(false);

    useEffect(() => {
        const today = new Date();
        setStartDate(toISO(new Date(today.getFullYear(), 0, 1)));
        setEndDate(toISO(new Date(today.getFullYear(), 11, 31)));
        loadVendors();
        loadSettings();
        loadClosedPeriods();
    }, []);

    const loadSettings = async () => {
        try {
            const resp = await api.get(API_ENDPOINTS.SETTINGS);
            const s = resp.data?.data;
            if (s?.company_name || s?.company_address) {
                setCompanyInfo({ name: s.company_name, address: s.company_address, city: s.company_city });
            }
        } catch { /* ignore */ }
    };

    const loadVendors = async () => {
        try {
            const resp = await api.get(API_ENDPOINTS.CONTACTS);
            const contactData = resp.data?.data || resp.data || [];
            const list = Array.isArray(contactData) ? contactData : (contactData.contacts || contactData.items || []);
            const vendorList = list
                .filter((c: any) => (c.type || c.Type || '').toUpperCase() === 'VENDOR')
                .map((c: any) => ({ id: c.id || c.ID, name: c.name || c.Name, code: c.code || c.Code || '' }));
            setVendors(vendorList);
        } catch (err) { console.error('Failed to load vendors:', err); }
    };

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
        if (!vendorId) {
            toast({ title: 'Peringatan', description: 'Pilih vendor terlebih dahulu', status: 'warning', duration: 3000, isClosable: true });
            return;
        }
        setLoading(true);
        setError(null);
        try {
            const resp = await api.get('/api/v1/reports/vendor-history', {
                params: { vendor_id: vendorId, start_date: startDate, end_date: endDate }
            });
            const result = resp.data.data || resp.data;
            setData(result);
            if (result.company) {
                setCompanyInfo(prev => ({ name: result.company?.name || prev?.name, address: result.company?.address || prev?.address, city: result.company?.city || prev?.city }));
            }
            toast({ title: 'Berhasil', description: 'Riwayat vendor berhasil dimuat', status: 'success', duration: 3000, isClosable: true });
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
            const resp = await fetch(`/api/v1/reports/vendor-history?vendor_id=${vendorId}&start_date=${startDate}&end_date=${endDate}&format=${format}`, {
                headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` }
            });
            if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
            const blob = await resp.blob();
            const url = URL.createObjectURL(blob);
            const link = document.createElement('a');
            link.href = url;
            link.download = `Riwayat_Vendor_${data.vendor?.name || 'report'}_${startDate}.${format}`;
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
            URL.revokeObjectURL(url);
            toast({ title: 'Ekspor Berhasil', status: 'success', duration: 3000, isClosable: true });
        } catch (err: any) {
            if (format === 'csv' && data.transactions?.length) {
                // Fallback CSV — enhanced with BOM, company header, and vendor info
                const bom = '\uFEFF';
                const rows: string[][] = [];

                // Company header
                rows.push([companyInfo?.name || 'Perusahaan']);
                if (companyInfo?.address) rows.push([companyInfo.address]);
                rows.push([]);
                rows.push(['RIWAYAT VENDOR']);
                rows.push([`Vendor: ${data.vendor?.name || '-'}`]);
                rows.push([`Periode: ${startDate} s/d ${endDate}`]);
                rows.push([`Dicetak: ${new Date().toLocaleDateString('id-ID', { day: '2-digit', month: 'long', year: 'numeric' })}`]);
                rows.push([]);

                // Summary
                if (data.summary) {
                    rows.push(['RINGKASAN']);
                    rows.push(['Total Transaksi', String(data.summary.total_transactions || data.transactions.length)]);
                    rows.push(['Total Jumlah', String(data.summary.total_amount || 0)]);
                    rows.push(['Total Dibayar', String(data.summary.total_paid || 0)]);
                    rows.push(['Total Sisa', String(data.summary.total_outstanding || 0)]);
                    rows.push([]);
                }

                // Data table
                rows.push(['Tanggal', 'Tipe', 'Kode', 'Deskripsi', 'Jumlah', 'Dibayar', 'Sisa', 'Status']);
                data.transactions.forEach((tx: any) => rows.push([
                    tx.date ? new Date(tx.date).toLocaleDateString('id-ID') : '-',
                    tx.transaction_type, tx.transaction_code, tx.description || '',
                    String(tx.amount || 0), String(tx.paid_amount || 0), String(tx.outstanding || 0), tx.status
                ]));

                // Totals row
                const totalAmt = data.transactions.reduce((s: number, tx: any) => s + (tx.amount || 0), 0);
                const totalPaid = data.transactions.reduce((s: number, tx: any) => s + (tx.paid_amount || 0), 0);
                const totalOut = data.transactions.reduce((s: number, tx: any) => s + (tx.outstanding || 0), 0);
                rows.push([]);
                rows.push(['', '', '', 'TOTAL', String(totalAmt), String(totalPaid), String(totalOut), '']);

                const csv = bom + rows.map(r => r.map(c => `"${String(c).replace(/"/g, '""')}"`).join(',')).join('\n');
                const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
                const url = URL.createObjectURL(blob);
                const link = document.createElement('a');
                link.href = url;
                link.download = `Riwayat_Vendor_${data.vendor?.name || 'report'}_${startDate}.csv`;
                document.body.appendChild(link);
                link.click();
                document.body.removeChild(link);
                URL.revokeObjectURL(url);
                toast({ title: 'Ekspor CSV Berhasil', status: 'success', duration: 3000, isClosable: true });
            } else {
                toast({ title: 'Ekspor Gagal', description: err.message, status: 'error', duration: 5000, isClosable: true });
            }
        }
    };

    const formatCurrency = (amount: number | null | undefined) => {
        if (amount === null || amount === undefined || isNaN(Number(amount))) return 'Rp 0';
        return new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', minimumFractionDigits: 0, maximumFractionDigits: 0 }).format(Number(amount));
    };

    const fetchJournalDetail = async (tx: any) => {
        setSelectedTx(tx);
        setJournalDetail(null);
        setLoadingDetail(true);
        onDetailOpen();
        try {
            const resp = await api.get(`/api/v1/reports/transaction-journal-detail?source_type=${tx.transaction_type}&source_id=${tx.transaction_id}`);
            const detail = resp.data?.data;
            setJournalDetail(detail);
        } catch (err) {
            console.error('Failed to load journal detail:', err);
            setJournalDetail({ journals: [] });
        } finally {
            setLoadingDetail(false);
        }
    };

    const getAnalytics = () => {
        if (!data?.transactions) return { purchaseCount: 0, paymentCount: 0, billCount: 0, purchasesByMonth: [] as any[] };
        let purchaseCount = 0, paymentCount = 0, billCount = 0;
        const monthMap: { [key: string]: { month: string; amount: number; count: number } } = {};
        data.transactions.forEach((tx: any) => {
            if (tx.transaction_type === 'PURCHASE') purchaseCount++;
            else if (tx.transaction_type === 'PAYMENT') paymentCount++;
            else if (tx.transaction_type === 'BILL') billCount++;
            if (tx.transaction_type === 'PURCHASE' && tx.date) {
                const d = new Date(tx.date);
                const key = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`;
                const monthName = d.toLocaleDateString('id-ID', { month: 'long', year: 'numeric' });
                if (!monthMap[key]) monthMap[key] = { month: monthName, amount: 0, count: 0 };
                monthMap[key].amount += tx.amount || 0;
                monthMap[key].count++;
            }
        });
        const purchasesByMonth = Object.entries(monthMap).sort(([a], [b]) => a.localeCompare(b)).map(([, v]) => v);
        return { purchaseCount, paymentCount, billCount, purchasesByMonth };
    };

    const analytics = data ? getAnalytics() : null;
    const paymentProgress = data?.summary?.total_amount > 0 ? ((data.summary.total_paid / data.summary.total_amount) * 100) : 0;

    return (
        <UnifiedLayout>
            <VStack spacing={6} align="stretch">
                {/* Header */}
                <HStack spacing={4}>
                    <IconButton icon={<FiArrowLeft />} variant="outline" onClick={() => router.push('/reports')} aria-label="Kembali" size="md" />
                    <VStack align="start" spacing={0}>
                        <Heading as="h1" size="lg">
                            <HStack>
                                <Icon as={FiTruck} color="orange.500" />
                                <Text>{t('reports.vendorHistoryReport.title') || 'Riwayat Vendor'}</Text>
                            </HStack>
                        </Heading>
                        <Text fontSize="sm" color={descriptionColor}>
                            {t('reports.description.vendorHistory') || 'Catatan lengkap aktivitas vendor termasuk transaksi pembelian dan riwayat pembayaran.'}
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
                                            <Text>{t('reports.closedPeriod') || 'Periode Tutup'}</Text>
                                            <Tooltip label="Pilih periode tutup untuk memuat riwayat berdasarkan periode fiskal" placement="top">
                                                <Box><Icon as={FiInfo} color="gray.500" /></Box>
                                            </Tooltip>
                                        </HStack>
                                    </FormLabel>
                                    <Select
                                        placeholder={t('reports.selectClosedPeriod') || 'Pilih periode tutup'}
                                        size="md"
                                        onChange={(e) => {
                                            if (e.target.value) {
                                                const selected = closedPeriods.find(p => p.date === e.target.value);
                                                if (selected) {
                                                    if (selected.start_date) setStartDate(selected.start_date);
                                                    else {
                                                        const d = new Date(selected.date);
                                                        setStartDate(`${d.getFullYear()}-01-01`);
                                                    }
                                                    setEndDate(selected.date);
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

                            {/* Vendor + Date filter */}
                            <HStack spacing={4} flexWrap="wrap">
                                <FormControl flex="1.5" minW="250px" isRequired>
                                    <FormLabel>{t('reports.vendorHistoryReport.selectVendor') || 'Pilih Vendor'}</FormLabel>
                                    <Select placeholder={t('reports.vendorHistoryReport.chooseVendor') || 'Pilih vendor...'} value={vendorId} onChange={(e) => setVendorId(e.target.value)}>
                                        {vendors.map(v => <option key={v.id} value={v.id}>{v.name} ({v.code})</option>)}
                                    </Select>
                                </FormControl>
                                <FormControl flex="1" minW="180px">
                                    <FormLabel>{t('reports.modal.startDate') || 'Tanggal Mulai'}</FormLabel>
                                    <Input type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} />
                                </FormControl>
                                <FormControl flex="1" minW="180px">
                                    <FormLabel>{t('reports.modal.endDate') || 'Tanggal Akhir'}</FormLabel>
                                    <Input type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)} />
                                </FormControl>
                                <Button colorScheme="orange" onClick={fetchReport} isLoading={loading} leftIcon={<FiTruck />} size="md" mt={8}>
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
                            <Text fontSize="lg" fontWeight="medium">Menghasilkan Riwayat Vendor...</Text>
                            <Text fontSize="sm" color={descriptionColor}>Menganalisis transaksi pembelian dan pembayaran</Text>
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
                            <Heading size="md" color={headingColor}>{t('reports.vendorHistoryReport.title') || 'Riwayat Vendor'}</Heading>
                            <Text fontSize="sm" color={descriptionColor}>Periode: {startDate} — {endDate}</Text>
                        </Box>

                        {/* Vendor Info Card */}
                        {data.vendor && (
                            <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                <HStack justify="space-between" mb={3}>
                                    <HStack>
                                        <Badge colorScheme="orange" px={2} py={1} fontSize="sm">🏭</Badge>
                                        <Text fontSize="md" fontWeight="bold" color={headingColor}>Informasi Vendor</Text>
                                    </HStack>
                                </HStack>
                                <SimpleGrid columns={[1, 2, 3]} spacing={3}>
                                    <HStack>
                                        <Icon as={FiUser} color="orange.500" />
                                        <VStack align="start" spacing={0}>
                                            <Text fontSize="xs" color={descriptionColor}>Nama</Text>
                                            <Text fontSize="sm" fontWeight="semibold">{data.vendor.name}</Text>
                                        </VStack>
                                    </HStack>
                                    <HStack>
                                        <Icon as={FiHash} color="orange.500" />
                                        <VStack align="start" spacing={0}>
                                            <Text fontSize="xs" color={descriptionColor}>Kode</Text>
                                            <Text fontSize="sm" fontWeight="semibold">{data.vendor.code}</Text>
                                        </VStack>
                                    </HStack>
                                    {data.vendor.email && (
                                        <HStack>
                                            <Icon as={FiMail} color="orange.500" />
                                            <VStack align="start" spacing={0}>
                                                <Text fontSize="xs" color={descriptionColor}>Email</Text>
                                                <Text fontSize="sm">{data.vendor.email}</Text>
                                            </VStack>
                                        </HStack>
                                    )}
                                    {data.vendor.phone && (
                                        <HStack>
                                            <Icon as={FiPhone} color="orange.500" />
                                            <VStack align="start" spacing={0}>
                                                <Text fontSize="xs" color={descriptionColor}>Telepon</Text>
                                                <Text fontSize="sm">{data.vendor.phone}</Text>
                                            </VStack>
                                        </HStack>
                                    )}
                                    {data.vendor.address && (
                                        <HStack>
                                            <Icon as={FiMapPin} color="orange.500" />
                                            <VStack align="start" spacing={0}>
                                                <Text fontSize="xs" color={descriptionColor}>Alamat</Text>
                                                <Text fontSize="sm">{data.vendor.address}</Text>
                                            </VStack>
                                        </HStack>
                                    )}
                                </SimpleGrid>
                            </Box>
                        )}

                        {/* Summary Statistics */}
                        {data.summary && (
                            <SimpleGrid columns={[1, 2, 4]} spacing={4}>
                                <Box bg="green.50" p={4} borderRadius="md" textAlign="center">
                                    <Text fontSize="2xl" fontWeight="bold" color="green.600">{data.summary.total_transactions || 0}</Text>
                                    <Text fontSize="sm" color="green.800">Total Transaksi</Text>
                                </Box>
                                <Box bg="orange.50" p={4} borderRadius="md" textAlign="center">
                                    <Text fontSize="2xl" fontWeight="bold" color="orange.600">{formatCurrency(data.summary.total_amount)}</Text>
                                    <Text fontSize="sm" color="orange.800">Total Pembelian</Text>
                                </Box>
                                <Box bg="purple.50" p={4} borderRadius="md" textAlign="center">
                                    <Text fontSize="2xl" fontWeight="bold" color="purple.600">{formatCurrency(data.summary.total_paid)}</Text>
                                    <Text fontSize="sm" color="purple.800">Total Dibayar</Text>
                                </Box>
                                <Box bg={(data.summary.total_outstanding || 0) > 0 ? "red.50" : "green.50"} p={4} borderRadius="md" textAlign="center">
                                    <Text fontSize="2xl" fontWeight="bold" color={(data.summary.total_outstanding || 0) > 0 ? "red.600" : "green.600"}>{formatCurrency(data.summary.total_outstanding)}</Text>
                                    <Text fontSize="sm" color={(data.summary.total_outstanding || 0) > 0 ? "red.800" : "green.800"}>Hutang</Text>
                                </Box>
                            </SimpleGrid>
                        )}

                        {/* Payment Progress */}
                        {data.summary && data.summary.total_amount > 0 && (
                            <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                <HStack justify="space-between" mb={2}>
                                    <HStack>
                                        <Badge colorScheme="teal" px={2} py={1} fontSize="sm">A</Badge>
                                        <Text fontSize="md" fontWeight="bold" color={headingColor}>Efisiensi Pembayaran</Text>
                                    </HStack>
                                    <Text fontSize="sm" fontWeight="bold" color={paymentProgress >= 100 ? 'green.600' : 'orange.600'}>
                                        {paymentProgress.toFixed(1)}%
                                    </Text>
                                </HStack>
                                <Progress value={paymentProgress} colorScheme={paymentProgress >= 100 ? 'green' : paymentProgress >= 50 ? 'orange' : 'red'} borderRadius="full" size="lg" hasStripe={paymentProgress < 100} isAnimated={paymentProgress < 100} />
                                <HStack justify="space-between" mt={2}>
                                    <Text fontSize="xs" color={descriptionColor}>Dibayar: {formatCurrency(data.summary.total_paid)}</Text>
                                    <Text fontSize="xs" color={descriptionColor}>Total: {formatCurrency(data.summary.total_amount)}</Text>
                                </HStack>
                            </Box>
                        )}

                        {/* Transaction Breakdown */}
                        {analytics && (analytics.purchaseCount > 0 || analytics.paymentCount > 0) && (
                            <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                <HStack justify="space-between" mb={3}>
                                    <HStack>
                                        <Badge colorScheme="purple" px={2} py={1} fontSize="sm">B</Badge>
                                        <Text fontSize="md" fontWeight="bold" color={headingColor}>Rincian Jenis Transaksi</Text>
                                    </HStack>
                                </HStack>
                                <SimpleGrid columns={[1, 3]} spacing={3}>
                                    <Box bg="orange.50" p={3} borderRadius="md" textAlign="center">
                                        <Text fontSize="xl" fontWeight="bold" color="orange.600">{analytics.purchaseCount}</Text>
                                        <Text fontSize="xs" color="orange.700">Pembelian</Text>
                                    </Box>
                                    <Box bg="purple.50" p={3} borderRadius="md" textAlign="center">
                                        <Text fontSize="xl" fontWeight="bold" color="purple.600">{analytics.paymentCount}</Text>
                                        <Text fontSize="xs" color="purple.700">Pembayaran</Text>
                                    </Box>
                                    <Box bg="blue.50" p={3} borderRadius="md" textAlign="center">
                                        <Text fontSize="xl" fontWeight="bold" color="blue.600">{analytics.billCount}</Text>
                                        <Text fontSize="xs" color="blue.700">Tagihan</Text>
                                    </Box>
                                </SimpleGrid>
                            </Box>
                        )}

                        {/* Purchases by Month */}
                        {analytics && analytics.purchasesByMonth.length > 0 && (
                            <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                <HStack justify="space-between" mb={3}>
                                    <HStack>
                                        <Badge colorScheme="orange" px={2} py={1} fontSize="sm">C</Badge>
                                        <Text fontSize="md" fontWeight="bold" color={headingColor}>Pembelian per Bulan</Text>
                                    </HStack>
                                </HStack>
                                <VStack spacing={2} align="stretch">
                                    {analytics.purchasesByMonth.map((m: any, i: number) => (
                                        <HStack key={i} justify="space-between" py={2} px={3} bg={summaryBg} borderRadius="md">
                                            <HStack>
                                                <Icon as={FiCalendar} color="orange.500" />
                                                <Text fontSize="sm" fontWeight="medium">{m.month}</Text>
                                                <Badge colorScheme="gray" fontSize="xs">{m.count} transaksi</Badge>
                                            </HStack>
                                            <Text fontSize="sm" fontWeight="bold" color="orange.600">{formatCurrency(m.amount)}</Text>
                                        </HStack>
                                    ))}
                                </VStack>
                            </Box>
                        )}

                        {/* Transaction Table */}
                        {data.transactions && data.transactions.length > 0 && (
                            <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                <HStack justify="space-between" mb={3}>
                                    <HStack>
                                        <Badge colorScheme="cyan" px={2} py={1} fontSize="sm">D</Badge>
                                        <Text fontSize="md" fontWeight="bold" color={headingColor}>Daftar Transaksi</Text>
                                    </HStack>
                                    <Text fontSize="sm" color={descriptionColor}>{data.transactions.length} transaksi</Text>
                                </HStack>
                                <TableContainer maxH="500px" overflowY="auto">
                                    <Table size="sm" variant="simple">
                                        <Thead bg="orange.600" position="sticky" top={0} zIndex={1}>
                                            <Tr>
                                                <Th color="white">Tanggal</Th>
                                                <Th color="white">Tipe</Th>
                                                <Th color="white">Kode</Th>
                                                <Th color="white">Deskripsi</Th>
                                                <Th color="white" isNumeric>Jumlah</Th>
                                                <Th color="white" isNumeric>Dibayar</Th>
                                                <Th color="white" isNumeric>Sisa</Th>
                                                <Th color="white">Status</Th>
                                                <Th color="white" textAlign="center">Aksi</Th>
                                            </Tr>
                                        </Thead>
                                        <Tbody>
                                            {data.transactions.map((tx: any, idx: number) => (
                                                <Tr key={idx} _hover={{ bg: 'orange.50', cursor: 'pointer' }} onClick={() => fetchJournalDetail(tx)} title="Klik untuk melihat detail jurnal & kode akun">
                                                    <Td fontSize="sm">{tx.date ? new Date(tx.date).toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric' }) : '-'}</Td>
                                                    <Td><Badge colorScheme={getTypeColor(tx.transaction_type)} fontSize="xs">{getTypeLabel(tx.transaction_type)}</Badge></Td>
                                                    <Td fontFamily="mono" fontSize="sm">{tx.transaction_code}</Td>
                                                    <Td fontSize="sm" maxW="200px" isTruncated>{tx.description || '-'}</Td>
                                                    <Td isNumeric fontWeight="bold" color="orange.700" fontSize="sm">{formatCurrency(tx.amount)}</Td>
                                                    <Td isNumeric color="green.600" fontSize="sm">{formatCurrency(tx.paid_amount)}</Td>
                                                    <Td isNumeric color={(tx.outstanding || 0) > 0 ? "red.600" : "green.600"} fontSize="sm">{formatCurrency(tx.outstanding)}</Td>
                                                    <Td><Badge colorScheme={getStatusColor(tx.status)} fontSize="xs">{tx.status}</Badge></Td>
                                                    <Td textAlign="center">
                                                        <Tooltip label="Lihat detail jurnal & kode akun" fontSize="xs">
                                                            <IconButton
                                                                aria-label="Detail"
                                                                icon={<FiEye />}
                                                                size="xs"
                                                                colorScheme="orange"
                                                                variant="ghost"
                                                                onClick={(e) => { e.stopPropagation(); fetchJournalDetail(tx); }}
                                                            />
                                                        </Tooltip>
                                                    </Td>
                                                </Tr>
                                            ))}
                                        </Tbody>
                                        <Tfoot bg="orange.700" position="sticky" bottom={0}>
                                            <Tr>
                                                <Th color="white" colSpan={4} fontSize="sm">TOTAL</Th>
                                                <Th color="white" isNumeric fontSize="sm">{formatCurrency(data.summary?.total_amount)}</Th>
                                                <Th color="white" isNumeric fontSize="sm">{formatCurrency(data.summary?.total_paid)}</Th>
                                                <Th color="white" isNumeric fontSize="sm">{formatCurrency(data.summary?.total_outstanding)}</Th>
                                                <Th color="white"></Th>
                                                <Th color="white"></Th>
                                            </Tr>
                                        </Tfoot>
                                    </Table>
                                </TableContainer>
                            </Box>
                        )}

                        {/* Summary Box */}
                        <Box bg="orange.50" p={4} borderRadius="md" border="1px" borderColor="orange.200">
                            <Text fontSize="md" fontWeight="bold" color="orange.800" mb={3}>
                                Ringkasan Riwayat Vendor
                            </Text>
                            <SimpleGrid columns={[1, 2]} spacing={3}>
                                <VStack spacing={2}>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color="orange.700">Nama Vendor:</Text>
                                        <Text fontSize="sm" fontWeight="semibold">{data.vendor?.name || '-'}</Text>
                                    </HStack>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color="orange.700">Total Transaksi:</Text>
                                        <Text fontSize="sm" fontWeight="semibold">{data.summary?.total_transactions || 0} transaksi</Text>
                                    </HStack>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color="orange.700">Total Pembelian:</Text>
                                        <Text fontSize="sm" fontWeight="semibold" color="orange.600">{formatCurrency(data.summary?.total_amount)}</Text>
                                    </HStack>
                                </VStack>
                                <VStack spacing={2}>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color="orange.700">Total Dibayar:</Text>
                                        <Text fontSize="sm" fontWeight="semibold" color="green.600">{formatCurrency(data.summary?.total_paid)}</Text>
                                    </HStack>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color="orange.700">Hutang:</Text>
                                        <Text fontSize="sm" fontWeight="semibold" color={(data.summary?.total_outstanding || 0) > 0 ? "red.600" : "green.600"}>{formatCurrency(data.summary?.total_outstanding)}</Text>
                                    </HStack>
                                    <HStack justify="space-between" w="full">
                                        <Text fontSize="sm" color="orange.700">Efisiensi Pembayaran:</Text>
                                        <Badge colorScheme={paymentProgress >= 100 ? "green" : "orange"}>{paymentProgress.toFixed(1)}%</Badge>
                                    </HStack>
                                </VStack>
                            </SimpleGrid>
                            {data.summary?.first_transaction && (
                                <Divider my={2} />
                            )}
                            {data.summary?.first_transaction && (
                                <HStack justify="space-between">
                                    <Text fontSize="xs" color="orange.600">Transaksi Pertama: {new Date(data.summary.first_transaction).toLocaleDateString('id-ID', { day: '2-digit', month: 'long', year: 'numeric' })}</Text>
                                    {data.summary?.last_transaction && (
                                        <Text fontSize="xs" color="orange.600">Transaksi Terakhir: {new Date(data.summary.last_transaction).toLocaleDateString('id-ID', { day: '2-digit', month: 'long', year: 'numeric' })}</Text>
                                    )}
                                </HStack>
                            )}
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

                {/* Transaction Journal Detail Modal */}
                <Modal isOpen={isDetailOpen} onClose={onDetailClose} size="5xl" scrollBehavior="inside">
                    <ModalOverlay bg="blackAlpha.400" backdropFilter="blur(4px)" />
                    <ModalContent borderRadius="xl" overflow="hidden">
                        <ModalHeader bg="orange.600" color="white" py={4}>
                            <VStack align="start" spacing={1}>
                                <HStack>
                                    <Text fontSize="lg" fontWeight="bold">Detail Jurnal Transaksi</Text>
                                    {selectedTx && (
                                        <Badge colorScheme={getTypeColor(selectedTx.transaction_type)} variant="solid" fontSize="xs">
                                            {getTypeLabel(selectedTx.transaction_type)}
                                        </Badge>
                                    )}
                                </HStack>
                                {selectedTx && (
                                    <Text fontSize="sm" opacity={0.9}>
                                        {selectedTx.transaction_code} — {selectedTx.date ? new Date(selectedTx.date).toLocaleDateString('id-ID', { day: '2-digit', month: 'long', year: 'numeric' }) : ''}
                                    </Text>
                                )}
                            </VStack>
                        </ModalHeader>
                        <ModalCloseButton color="white" />
                        <ModalBody py={6}>
                            {loadingDetail && (
                                <Box textAlign="center" py={10}>
                                    <Spinner size="xl" thickness="4px" speed="0.65s" color="orange.500" />
                                    <Text mt={3} color="gray.500">Memuat detail jurnal...</Text>
                                </Box>
                            )}

                            {!loadingDetail && selectedTx && (
                                <VStack spacing={5} align="stretch">
                                    {/* Transaction Summary Card */}
                                    <Box bg="orange.50" p={4} borderRadius="lg" border="1px" borderColor="orange.200">
                                        <SimpleGrid columns={[1, 2, 4]} spacing={3}>
                                            <VStack align="start" spacing={0}>
                                                <Text fontSize="xs" color="orange.600" fontWeight="medium">Jumlah</Text>
                                                <Text fontSize="md" fontWeight="bold" color="orange.800">{formatCurrency(selectedTx.amount)}</Text>
                                            </VStack>
                                            <VStack align="start" spacing={0}>
                                                <Text fontSize="xs" color="green.600" fontWeight="medium">Dibayar</Text>
                                                <Text fontSize="md" fontWeight="bold" color="green.700">{formatCurrency(selectedTx.paid_amount)}</Text>
                                            </VStack>
                                            <VStack align="start" spacing={0}>
                                                <Text fontSize="xs" color="red.600" fontWeight="medium">Hutang</Text>
                                                <Text fontSize="md" fontWeight="bold" color={(selectedTx.outstanding || 0) > 0 ? 'red.700' : 'green.700'}>
                                                    {formatCurrency(selectedTx.outstanding)}
                                                </Text>
                                            </VStack>
                                            <VStack align="start" spacing={0}>
                                                <Text fontSize="xs" color="gray.600" fontWeight="medium">Status</Text>
                                                <Badge colorScheme={getStatusColor(selectedTx.status)} mt={1}>{selectedTx.status}</Badge>
                                            </VStack>
                                        </SimpleGrid>
                                        {selectedTx.description && (
                                            <Text fontSize="sm" color="orange.700" mt={2}>📝 {selectedTx.description}</Text>
                                        )}
                                    </Box>

                                    {/* Journal Entries */}
                                    {journalDetail?.journals?.length > 0 ? (
                                        journalDetail.journals.map((journal: any, jIdx: number) => (
                                            <Box key={jIdx} borderWidth="1px" borderRadius="lg" overflow="hidden" borderColor="gray.200">
                                                {/* Journal Header */}
                                                <Box bg="gray.50" px={4} py={3} borderBottomWidth="1px" borderColor="gray.200">
                                                    <HStack justify="space-between" flexWrap="wrap">
                                                        <HStack spacing={3}>
                                                            <Badge colorScheme="purple" variant="subtle" px={2} py={1}>
                                                                📋 {journal.entry_number}
                                                            </Badge>
                                                            <Badge colorScheme={journal.status === 'POSTED' ? 'green' : journal.status === 'DRAFT' ? 'yellow' : 'red'}>
                                                                {journal.status}
                                                            </Badge>
                                                        </HStack>
                                                        <Text fontSize="xs" color="gray.500">
                                                            Tanggal: {new Date(journal.date).toLocaleDateString('id-ID', { day: '2-digit', month: 'long', year: 'numeric' })}
                                                        </Text>
                                                    </HStack>
                                                    {journal.description && (
                                                        <Text fontSize="sm" color="gray.600" mt={1}>{journal.description}</Text>
                                                    )}
                                                </Box>

                                                {/* Account Lines Table */}
                                                <TableContainer>
                                                    <Table size="sm" variant="simple">
                                                        <Thead bg="orange.50">
                                                            <Tr>
                                                                <Th color="orange.700" w="120px">Kode Akun</Th>
                                                                <Th color="orange.700">Nama Akun</Th>
                                                                <Th color="orange.700">Keterangan</Th>
                                                                <Th color="orange.700" isNumeric>Debit</Th>
                                                                <Th color="orange.700" isNumeric>Kredit</Th>
                                                            </Tr>
                                                        </Thead>
                                                        <Tbody>
                                                            {journal.lines?.map((line: any, lIdx: number) => (
                                                                <Tr key={lIdx} _hover={{ bg: 'gray.50' }}>
                                                                    <Td fontFamily="mono" fontSize="sm" fontWeight="semibold" color="purple.700">
                                                                        {line.account_code || '-'}
                                                                    </Td>
                                                                    <Td fontSize="sm" fontWeight="medium">{line.account_name || '-'}</Td>
                                                                    <Td fontSize="sm" color="gray.600">{line.description || '-'}</Td>
                                                                    <Td isNumeric fontSize="sm" fontWeight={line.debit > 0 ? 'bold' : 'normal'} color={line.debit > 0 ? 'blue.700' : 'gray.400'}>
                                                                        {line.debit > 0 ? formatCurrency(line.debit) : '-'}
                                                                    </Td>
                                                                    <Td isNumeric fontSize="sm" fontWeight={line.credit > 0 ? 'bold' : 'normal'} color={line.credit > 0 ? 'green.700' : 'gray.400'}>
                                                                        {line.credit > 0 ? formatCurrency(line.credit) : '-'}
                                                                    </Td>
                                                                </Tr>
                                                            ))}
                                                        </Tbody>
                                                        <Tfoot bg="gray.100">
                                                            <Tr fontWeight="bold">
                                                                <Td colSpan={3} textAlign="right" fontSize="sm">TOTAL</Td>
                                                                <Td isNumeric fontSize="sm" color="blue.800">{formatCurrency(journal.total_debit)}</Td>
                                                                <Td isNumeric fontSize="sm" color="green.800">{formatCurrency(journal.total_credit)}</Td>
                                                            </Tr>
                                                            <Tr>
                                                                <Td colSpan={3} textAlign="right" fontSize="xs" color="gray.500">Selisih (Balance Check)</Td>
                                                                <Td colSpan={2} textAlign="center" fontSize="xs" color={journal.is_balanced !== false ? 'green.600' : 'red.600'} fontWeight="bold">
                                                                    {journal.is_balanced !== false ? '✅ Seimbang' : `❌ ${formatCurrency(Math.abs(journal.total_debit - journal.total_credit))}`}
                                                                </Td>
                                                            </Tr>
                                                        </Tfoot>
                                                    </Table>
                                                </TableContainer>
                                            </Box>
                                        ))
                                    ) : (
                                        <Box textAlign="center" py={8} bg="gray.50" borderRadius="lg">
                                            <Text fontSize="lg" color="gray.400">📭</Text>
                                            <Text fontSize="sm" color="gray.500" mt={2}>Tidak ada jurnal yang terkait dengan transaksi ini</Text>
                                            <Text fontSize="xs" color="gray.400" mt={1}>Transaksi mungkin belum diposting atau jurnal belum dibuat</Text>
                                        </Box>
                                    )}
                                </VStack>
                            )}
                        </ModalBody>
                        <ModalFooter borderTopWidth="1px">
                            <Button onClick={onDetailClose} colorScheme="orange" variant="outline">Tutup</Button>
                        </ModalFooter>
                    </ModalContent>
                </Modal>
            </VStack>
        </UnifiedLayout>
    );
};

export default VendorHistoryPage;
