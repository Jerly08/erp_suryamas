'use client';

import React, { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useTranslation } from '@/hooks/useTranslation';
import UnifiedLayout from '@/components/layout/UnifiedLayout';
import api from '@/services/api';
import {
    Box, Heading, Text, Button, VStack, HStack, Card, CardBody, Icon, SimpleGrid,
    FormControl, FormLabel, Input, Select, Spinner, useToast, useColorModeValue,
    IconButton, Badge, Table, Thead, Tbody, Tr, Th, Td, TableContainer,
} from '@chakra-ui/react';
import { FiArrowLeft, FiTruck, FiFileText, FiFilePlus } from 'react-icons/fi';

const toISO = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;

interface Vendor { id: number; name: string; code: string; }

const VendorHistoryPage: React.FC = () => {
    const router = useRouter();
    const { t } = useTranslation();
    const toast = useToast();

    const cardBg = useColorModeValue('white', 'gray.800');
    const borderColor = useColorModeValue('gray.200', 'gray.600');
    const headingColor = useColorModeValue('gray.700', 'white');
    const descriptionColor = useColorModeValue('gray.600', 'gray.300');

    const [startDate, setStartDate] = useState('');
    const [endDate, setEndDate] = useState('');
    const [vendorId, setVendorId] = useState('');
    const [vendors, setVendors] = useState<Vendor[]>([]);
    const [data, setData] = useState<any>(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        const today = new Date();
        setStartDate(toISO(new Date(today.getFullYear(), 0, 1)));
        setEndDate(toISO(new Date(today.getFullYear(), 11, 31)));
        loadVendors();
    }, []);

    const loadVendors = async () => {
        try {
            const resp = await api.get('/api/v1/vendors');
            if (resp.data.success) setVendors(resp.data.data || []);
        } catch (err) { console.error('Failed to load vendors'); }
    };

    const fetchReport = async () => {
        if (!vendorId) {
            toast({ title: 'Error', description: t('reports.vendorHistoryReport.selectVendorFirst'), status: 'error', duration: 3000 });
            return;
        }
        setLoading(true);
        setError(null);
        try {
            const resp = await api.get('/api/v1/reports/vendor-history', {
                params: { vendor_id: vendorId, start_date: startDate, end_date: endDate }
            });
            setData(resp.data.data || resp.data);
            toast({ title: t('reports.messages.success'), status: 'success', duration: 3000 });
        } catch (err: any) {
            setError(err.message);
            toast({ title: t('reports.messages.error'), description: err.message, status: 'error', duration: 5000 });
        } finally {
            setLoading(false);
        }
    };

    const formatCurrency = (amount: number | null | undefined) => {
        if (amount === null || amount === undefined) return 'Rp 0';
        return new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', minimumFractionDigits: 0 }).format(Number(amount));
    };

    return (
        <UnifiedLayout>
            <VStack spacing={6} align="stretch">
                <HStack spacing={4}>
                    <IconButton icon={<FiArrowLeft />} variant="outline" onClick={() => router.push('/reports')} aria-label="Back" />
                    <VStack align="start" spacing={0}>
                        <Heading as="h1" size="lg"><HStack><Icon as={FiTruck} color="orange.500" /><Text>{t('reports.vendorHistoryReport.title')}</Text></HStack></Heading>
                        <Text fontSize="sm" color={descriptionColor}>{t('reports.vendorHistoryReport.description')}</Text>
                    </VStack>
                </HStack>

                <Card bg={cardBg} borderColor={borderColor} borderWidth="1px">
                    <CardBody>
                        <VStack spacing={4} align="stretch">
                            <HStack spacing={4} flexWrap="wrap">
                                <FormControl flex="1" minW="250px" isRequired>
                                    <FormLabel>{t('reports.vendorHistoryReport.selectVendor')}</FormLabel>
                                    <Select placeholder={t('reports.vendorHistoryReport.chooseVendor')} value={vendorId} onChange={(e) => setVendorId(e.target.value)}>
                                        {vendors.map(v => <option key={v.id} value={v.id}>{v.name} ({v.code})</option>)}
                                    </Select>
                                </FormControl>
                                <FormControl flex="1" minW="180px">
                                    <FormLabel>{t('reports.modal.startDate')}</FormLabel>
                                    <Input type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} />
                                </FormControl>
                                <FormControl flex="1" minW="180px">
                                    <FormLabel>{t('reports.modal.endDate')}</FormLabel>
                                    <Input type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)} />
                                </FormControl>
                                <Button colorScheme="orange" onClick={fetchReport} isLoading={loading} leftIcon={<FiTruck />} mt={8}>
                                    {t('reports.modal.generateReport')}
                                </Button>
                            </HStack>
                        </VStack>
                    </CardBody>
                </Card>

                {loading && <Box textAlign="center" py={8}><Spinner size="xl" color="orange.500" /></Box>}
                {error && <Box bg="red.50" p={4} borderRadius="md"><Text color="red.600">Error: {error}</Text></Box>}

                {data && !loading && (
                    <VStack spacing={6} align="stretch">
                        {data.vendor && (
                            <Box bg="orange.50" p={4} borderRadius="md">
                                <HStack><Text fontSize="lg" fontWeight="bold" color="orange.800">{data.vendor.name}</Text><Badge colorScheme="orange">{data.vendor.code}</Badge></HStack>
                                <Text fontSize="sm" color="orange.600">{data.vendor.email} | {data.vendor.phone}</Text>
                            </Box>
                        )}

                        {data.summary && (
                            <SimpleGrid columns={[1, 2, 4]} spacing={4}>
                                <Box bg="green.50" p={4} borderRadius="md" textAlign="center">
                                    <Text fontSize="2xl" fontWeight="bold" color="green.600">{data.summary.total_transactions || 0}</Text>
                                    <Text fontSize="sm">Total Transactions</Text>
                                </Box>
                                <Box bg="blue.50" p={4} borderRadius="md" textAlign="center">
                                    <Text fontSize="2xl" fontWeight="bold" color="blue.600">{formatCurrency(data.summary.total_amount)}</Text>
                                    <Text fontSize="sm">Total Amount</Text>
                                </Box>
                                <Box bg="purple.50" p={4} borderRadius="md" textAlign="center">
                                    <Text fontSize="2xl" fontWeight="bold" color="purple.600">{formatCurrency(data.summary.total_paid)}</Text>
                                    <Text fontSize="sm">Total Paid</Text>
                                </Box>
                                <Box bg="orange.50" p={4} borderRadius="md" textAlign="center">
                                    <Text fontSize="2xl" fontWeight="bold" color="orange.600">{formatCurrency(data.summary.total_outstanding)}</Text>
                                    <Text fontSize="sm">Outstanding</Text>
                                </Box>
                            </SimpleGrid>
                        )}

                        {data.transactions && data.transactions.length > 0 && (
                            <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                <TableContainer maxH="500px" overflowY="auto">
                                    <Table size="sm" variant="striped">
                                        <Thead bg="orange.600" position="sticky" top={0} zIndex={1}>
                                            <Tr>
                                                <Th color="white">{t('reports.customerHistoryReport.date')}</Th>
                                                <Th color="white">{t('reports.customerHistoryReport.type')}</Th>
                                                <Th color="white">{t('reports.customerHistoryReport.code')}</Th>
                                                <Th color="white" isNumeric>{t('reports.customerHistoryReport.amount')}</Th>
                                                <Th color="white" isNumeric>{t('reports.customerHistoryReport.paid')}</Th>
                                                <Th color="white" isNumeric>{t('reports.customerHistoryReport.outstanding')}</Th>
                                                <Th color="white">{t('reports.customerHistoryReport.status')}</Th>
                                            </Tr>
                                        </Thead>
                                        <Tbody>
                                            {data.transactions.map((tx: any, idx: number) => (
                                                <Tr key={idx}>
                                                    <Td>{tx.date ? new Date(tx.date).toLocaleDateString('id-ID') : '-'}</Td>
                                                    <Td><Badge colorScheme={tx.transaction_type === 'PURCHASE' ? 'orange' : 'red'}>{tx.transaction_type}</Badge></Td>
                                                    <Td fontFamily="mono">{tx.transaction_code}</Td>
                                                    <Td isNumeric fontWeight="bold" color="orange.700">{formatCurrency(tx.amount)}</Td>
                                                    <Td isNumeric color="green.600">{formatCurrency(tx.paid_amount)}</Td>
                                                    <Td isNumeric color="red.600">{formatCurrency(tx.outstanding)}</Td>
                                                    <Td><Badge colorScheme={tx.status === 'PAID' ? 'green' : tx.status === 'UNPAID' ? 'red' : 'yellow'}>{tx.status}</Badge></Td>
                                                </Tr>
                                            ))}
                                        </Tbody>
                                    </Table>
                                </TableContainer>
                            </Box>
                        )}

                        <HStack spacing={3} justify="flex-end">
                            <Button colorScheme="red" variant="outline" leftIcon={<FiFilePlus />}>{t('reports.exportPDF')}</Button>
                            <Button colorScheme="green" variant="outline" leftIcon={<FiFileText />}>{t('reports.exportCSV')}</Button>
                        </HStack>
                    </VStack>
                )}
            </VStack>
        </UnifiedLayout>
    );
};

export default VendorHistoryPage;
