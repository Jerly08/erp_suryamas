'use client';

import React, { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useTranslation } from '@/hooks/useTranslation';
import UnifiedLayout from '@/components/layout/UnifiedLayout';
import { ssotCashFlowReportService, SSOTCashFlowData } from '@/services/ssotCashFlowReportService';
import cashFlowExportService from '@/services/cashFlowExportService';
import api from '@/services/api';
import { API_ENDPOINTS } from '@/config/api';
import {
    Box, Heading, Text, Button, VStack, HStack, Card, CardBody, Icon, SimpleGrid,
    FormControl, FormLabel, Input, Select, Spinner, useToast, useColorModeValue,
    Divider, IconButton, Badge,
} from '@chakra-ui/react';
import { FiArrowLeft, FiActivity, FiFileText, FiFilePlus, FiCalendar, FiClock } from 'react-icons/fi';

const toISO = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;

const CashFlowPage: React.FC = () => {
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
    const [data, setData] = useState<SSOTCashFlowData | null>(null);
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
            const result = await ssotCashFlowReportService.generateSSOTCashFlow({
                start_date: startDate, end_date: endDate, format: 'json'
            });
            setData(result as SSOTCashFlowData);
            toast({ title: t('reports.messages.success'), status: 'success', duration: 3000 });
        } catch (err: any) {
            setError(err.message);
            toast({ title: t('reports.messages.error'), description: err.message, status: 'error', duration: 5000 });
        } finally {
            setLoading(false);
        }
    };

    const handleExportPDF = async () => {
        if (!data) return;
        try {
            await cashFlowExportService.exportToPDF({ start_date: startDate, end_date: endDate });
            toast({ title: t('reports.messages.pdfExportSuccessful'), status: 'success', duration: 3000 });
        } catch (err: any) {
            toast({ title: t('reports.messages.pdfExportFailed'), status: 'error', duration: 5000 });
        }
    };

    const handleExportCSV = async () => {
        if (!data) return;
        try {
            await cashFlowExportService.exportToCSV({ start_date: startDate, end_date: endDate });
            toast({ title: t('reports.messages.csvExportSuccessful'), status: 'success', duration: 3000 });
        } catch (err: any) {
            toast({ title: t('reports.messages.csvExportFailed'), status: 'error', duration: 5000 });
        }
    };

    const formatCurrency = (amount: number | null | undefined) => {
        if (amount === null || amount === undefined) return 'Rp 0';
        return new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', minimumFractionDigits: 0 }).format(Number(amount));
    };

    const renderActivitySection = (title: string, items: any[] | undefined, color: string) => {
        if (!items || items.length === 0) return null;
        const total = items.reduce((sum, item) => sum + (item.amount || 0), 0);
        return (
            <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                <HStack justify="space-between" mb={3}>
                    <Text fontSize="md" fontWeight="bold" color={headingColor}>{title}</Text>
                    <Badge colorScheme={color} fontSize="md" px={3} py={1}>{formatCurrency(total)}</Badge>
                </HStack>
                <VStack spacing={2} align="stretch">
                    {items.map((item: any, idx: number) => (
                        <HStack key={idx} justify="space-between" pl={4}>
                            <Text fontSize="sm" color={descriptionColor}>{item.description || item.account_name || 'Item'}</Text>
                            <Text fontSize="sm" fontWeight="medium" color={item.amount >= 0 ? 'green.600' : 'red.600'}>{formatCurrency(item.amount)}</Text>
                        </HStack>
                    ))}
                </VStack>
            </Box>
        );
    };

    return (
        <UnifiedLayout>
            <VStack spacing={6} align="stretch">
                <HStack spacing={4}>
                    <IconButton icon={<FiArrowLeft />} variant="outline" onClick={() => router.push('/reports')} aria-label="Back" />
                    <VStack align="start" spacing={0}>
                        <Heading as="h1" size="lg"><HStack><Icon as={FiActivity} color="teal.500" /><Text>{t('reports.cashFlowStatement')}</Text></HStack></Heading>
                        <Text fontSize="sm" color={descriptionColor}>{t('reports.description.cashFlow')}</Text>
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
                            <Button colorScheme="teal" onClick={fetchReport} isLoading={loading} leftIcon={<FiActivity />} mt={8}>
                                {t('reports.modal.generateReport')}
                            </Button>
                        </HStack>
                    </CardBody>
                </Card>

                {loading && <Box textAlign="center" py={8}><Spinner size="xl" color="teal.500" /></Box>}
                {error && <Box bg="red.50" p={4} borderRadius="md"><Text color="red.600">Error: {error}</Text></Box>}

                {data && !loading && (
                    <VStack spacing={6} align="stretch">
                        <Box textAlign="center" bg={summaryBg} p={4} borderRadius="md">
                            <Heading size="md" color={headingColor}>{t('reports.cashFlowStatement')}</Heading>
                            <Text fontSize="sm" color={descriptionColor}>{startDate} - {endDate}</Text>
                        </Box>

                        <SimpleGrid columns={[1, 2, 4]} spacing={4}>
                            <Box bg="blue.50" p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="blue.600">{formatCurrency(data.beginning_cash)}</Text>
                                <Text fontSize="sm">{t('reports.cashFlow.beginningCash')}</Text>
                            </Box>
                            <Box bg="green.50" p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="green.600">{formatCurrency(data.net_cash_change)}</Text>
                                <Text fontSize="sm">{t('reports.cashFlow.netChange')}</Text>
                            </Box>
                            <Box bg="purple.50" p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="purple.600">{formatCurrency(data.ending_cash)}</Text>
                                <Text fontSize="sm">{t('reports.cashFlow.endingCash')}</Text>
                            </Box>
                        </SimpleGrid>

                        {renderActivitySection(t('reports.cashFlow.operatingActivities'), data.operating_activities?.items, 'green')}
                        {renderActivitySection(t('reports.cashFlow.investingActivities'), data.investing_activities?.items, 'blue')}
                        {renderActivitySection(t('reports.cashFlow.financingActivities'), data.financing_activities?.items, 'purple')}

                        <HStack spacing={3} justify="flex-end">
                            <Button colorScheme="red" variant="outline" leftIcon={<FiFilePlus />} onClick={handleExportPDF}>{t('reports.exportPDF')}</Button>
                            <Button colorScheme="green" variant="outline" leftIcon={<FiFileText />} onClick={handleExportCSV}>{t('reports.exportCSV')}</Button>
                        </HStack>
                    </VStack>
                )}
            </VStack>
        </UnifiedLayout>
    );
};

export default CashFlowPage;
