'use client';

import React, { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useTranslation } from '@/hooks/useTranslation';
import UnifiedLayout from '@/components/layout/UnifiedLayout';
import { ssotPurchaseReportService, SSOTPurchaseReportData } from '@/services/ssotPurchaseReportService';
import {
    Box, Heading, Text, Button, VStack, HStack, Card, CardBody, Icon, SimpleGrid,
    FormControl, FormLabel, Input, Spinner, useToast, useColorModeValue,
    IconButton, Badge, Table, Thead, Tbody, Tr, Th, Td, TableContainer,
} from '@chakra-ui/react';
import { FiArrowLeft, FiShoppingCart, FiFileText, FiFilePlus } from 'react-icons/fi';

const toISO = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;

const PurchaseReportPage: React.FC = () => {
    const router = useRouter();
    const { t } = useTranslation();
    const toast = useToast();

    const cardBg = useColorModeValue('white', 'gray.800');
    const borderColor = useColorModeValue('gray.200', 'gray.600');
    const headingColor = useColorModeValue('gray.700', 'white');
    const descriptionColor = useColorModeValue('gray.600', 'gray.300');
    const summaryBg = useColorModeValue('gray.50', 'gray.700');

    const [startDate, setStartDate] = useState('');
    const [endDate, setEndDate] = useState('');
    const [data, setData] = useState<SSOTPurchaseReportData | null>(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        const today = new Date();
        setStartDate(toISO(new Date(today.getFullYear(), 0, 1)));
        setEndDate(toISO(new Date(today.getFullYear(), 11, 31)));
    }, []);

    const fetchReport = async () => {
        setLoading(true);
        setError(null);
        try {
            const result = await ssotPurchaseReportService.generateSSOTPurchaseReport({
                start_date: startDate, end_date: endDate, format: 'json'
            });
            setData(result);
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
                        <Heading as="h1" size="lg"><HStack><Icon as={FiShoppingCart} color="orange.500" /><Text>{t('reports.purchaseReport')}</Text></HStack></Heading>
                        <Text fontSize="sm" color={descriptionColor}>{t('reports.description.purchaseReport')}</Text>
                    </VStack>
                </HStack>

                <Card bg={cardBg} borderColor={borderColor} borderWidth="1px">
                    <CardBody>
                        <HStack spacing={4} flexWrap="wrap">
                            <FormControl flex="1" minW="200px">
                                <FormLabel>{t('reports.modal.startDate')}</FormLabel>
                                <Input type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} />
                            </FormControl>
                            <FormControl flex="1" minW="200px">
                                <FormLabel>{t('reports.modal.endDate')}</FormLabel>
                                <Input type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)} />
                            </FormControl>
                            <Button colorScheme="orange" onClick={fetchReport} isLoading={loading} leftIcon={<FiShoppingCart />} mt={8}>
                                {t('reports.modal.generateReport')}
                            </Button>
                        </HStack>
                    </CardBody>
                </Card>

                {loading && <Box textAlign="center" py={8}><Spinner size="xl" color="orange.500" /></Box>}
                {error && <Box bg="red.50" p={4} borderRadius="md"><Text color="red.600">Error: {error}</Text></Box>}

                {data && !loading && (
                    <VStack spacing={6} align="stretch">
                        <Box textAlign="center" bg={summaryBg} p={4} borderRadius="md">
                            <Heading size="md" color={headingColor}>{t('reports.purchaseReport')}</Heading>
                            <Text fontSize="sm" color={descriptionColor}>{startDate} - {endDate}</Text>
                        </Box>

                        <SimpleGrid columns={[1, 2, 4]} spacing={4}>
                            <Box bg="orange.50" p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="orange.600">{formatCurrency(data.total_amount)}</Text>
                                <Text fontSize="sm">{t('reports.purchaseReport.totalPurchases')}</Text>
                            </Box>
                            <Box bg="blue.50" p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="blue.600">{data.total_purchases || 0}</Text>
                                <Text fontSize="sm">{t('reports.purchaseReport.totalTransactions')}</Text>
                            </Box>
                            <Box bg="purple.50" p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="purple.600">{formatCurrency(data.total_paid)}</Text>
                                <Text fontSize="sm">{t('reports.purchaseReport.totalPaid')}</Text>
                            </Box>
                            <Box bg="green.50" p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="green.600">{formatCurrency(data.outstanding_payables)}</Text>
                                <Text fontSize="sm">{t('reports.purchaseReport.outstanding')}</Text>
                            </Box>
                        </SimpleGrid>

                        {data.purchases_by_vendor && data.purchases_by_vendor.length > 0 && (
                            <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                <Text fontSize="md" fontWeight="bold" color={headingColor} mb={3}>{t('reports.purchaseReport.purchaseDetails')}</Text>
                                <TableContainer>
                                    <Table size="sm" variant="striped">
                                        <Thead bg="orange.600">
                                            <Tr>
                                                <Th color="white">{t('reports.purchaseReport.vendor')}</Th>
                                                <Th color="white" isNumeric>{t('reports.purchaseReport.totalPurchases')}</Th>
                                                <Th color="white" isNumeric>{t('reports.purchaseReport.amount')}</Th>
                                                <Th color="white" isNumeric>{t('reports.purchaseReport.paid')}</Th>
                                                <Th color="white" isNumeric>{t('reports.purchaseReport.outstanding')}</Th>
                                                <Th color="white">{t('reports.purchaseReport.status')}</Th>
                                            </Tr>
                                        </Thead>
                                        <Tbody>
                                            {data.purchases_by_vendor.map((vendor, idx) => (
                                                <Tr key={idx}>
                                                    <Td>{vendor.vendor_name || '-'}</Td>
                                                    <Td isNumeric>{vendor.total_purchases}</Td>
                                                    <Td isNumeric fontWeight="bold" color="orange.600">{formatCurrency(vendor.total_amount)}</Td>
                                                    <Td isNumeric color="green.600">{formatCurrency(vendor.total_paid)}</Td>
                                                    <Td isNumeric color="red.600">{formatCurrency(vendor.outstanding)}</Td>
                                                    <Td><Badge colorScheme={vendor.status === 'COMPLETED' ? 'green' : vendor.status === 'PENDING' ? 'yellow' : 'gray'}>{vendor.status}</Badge></Td>
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

export default PurchaseReportPage;
