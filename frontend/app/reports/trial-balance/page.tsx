'use client';

import React, { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useTranslation } from '@/hooks/useTranslation';
import UnifiedLayout from '@/components/layout/UnifiedLayout';
import { ssotTrialBalanceService, SSOTTrialBalanceData } from '@/services/ssotTrialBalanceService';
import {
    Box, Heading, Text, Button, VStack, HStack, Card, CardBody, Icon, SimpleGrid,
    FormControl, FormLabel, Input, Spinner, useToast, useColorModeValue,
    IconButton, Badge, Table, Thead, Tbody, Tr, Th, Td, TableContainer, Tfoot,
} from '@chakra-ui/react';
import { FiArrowLeft, FiList, FiFileText, FiFilePlus } from 'react-icons/fi';

const toISO = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;

const TrialBalancePage: React.FC = () => {
    const router = useRouter();
    const { t } = useTranslation();
    const toast = useToast();

    const cardBg = useColorModeValue('white', 'gray.800');
    const borderColor = useColorModeValue('gray.200', 'gray.600');
    const headingColor = useColorModeValue('gray.700', 'white');
    const descriptionColor = useColorModeValue('gray.600', 'gray.300');
    const summaryBg = useColorModeValue('gray.50', 'gray.700');

    const [asOfDate, setAsOfDate] = useState('');
    const [data, setData] = useState<SSOTTrialBalanceData | null>(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        setAsOfDate(toISO(new Date()));
    }, []);

    const fetchReport = async () => {
        setLoading(true);
        setError(null);
        try {
            const result = await ssotTrialBalanceService.generateSSOTTrialBalance({ as_of_date: asOfDate, format: 'json' });
            setData(result);
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
            await ssotTrialBalanceService.exportToPDF({ as_of_date: asOfDate });
            toast({ title: t('reports.messages.pdfExportSuccessful'), status: 'success', duration: 3000 });
        } catch (err: any) {
            toast({ title: t('reports.messages.pdfExportFailed'), status: 'error', duration: 5000 });
        }
    };

    const handleExportCSV = async () => {
        if (!data) return;
        try {
            await ssotTrialBalanceService.exportToCSV({ as_of_date: asOfDate });
            toast({ title: t('reports.messages.csvExportSuccessful'), status: 'success', duration: 3000 });
        } catch (err: any) {
            toast({ title: t('reports.messages.csvExportFailed'), status: 'error', duration: 5000 });
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
                        <Heading as="h1" size="lg"><HStack><Icon as={FiList} color="cyan.500" /><Text>{t('reports.trialBalance')}</Text></HStack></Heading>
                        <Text fontSize="sm" color={descriptionColor}>{t('reports.description.trialBalance')}</Text>
                    </VStack>
                </HStack>

                <Card bg={cardBg} borderColor={borderColor} borderWidth="1px">
                    <CardBody>
                        <HStack spacing={4} flexWrap="wrap">
                            <FormControl flex="1" minW="200px">
                                <FormLabel>{t('reports.modal.asOfDate')}</FormLabel>
                                <Input type="date" value={asOfDate} onChange={(e) => setAsOfDate(e.target.value)} />
                            </FormControl>
                            <Button colorScheme="cyan" onClick={fetchReport} isLoading={loading} leftIcon={<FiList />} mt={8}>
                                {t('reports.modal.generateReport')}
                            </Button>
                        </HStack>
                    </CardBody>
                </Card>

                {loading && <Box textAlign="center" py={8}><Spinner size="xl" color="cyan.500" /></Box>}
                {error && <Box bg="red.50" p={4} borderRadius="md"><Text color="red.600">Error: {error}</Text></Box>}

                {data && !loading && (
                    <VStack spacing={6} align="stretch">
                        <Box textAlign="center" bg={summaryBg} p={4} borderRadius="md">
                            <Heading size="md" color={headingColor}>{t('reports.trialBalance')}</Heading>
                            <Text fontSize="sm" color={descriptionColor}>{t('reports.modal.asOfDate')}: {asOfDate}</Text>
                        </Box>

                        <SimpleGrid columns={[1, 3]} spacing={4}>
                            <Box bg="blue.50" p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="blue.600">{formatCurrency(data.total_debits)}</Text>
                                <Text fontSize="sm">{t('reports.trialBalance.totalDebits')}</Text>
                            </Box>
                            <Box bg="red.50" p={4} borderRadius="md" textAlign="center">
                                <Text fontSize="2xl" fontWeight="bold" color="red.600">{formatCurrency(data.total_credits)}</Text>
                                <Text fontSize="sm">{t('reports.trialBalance.totalCredits')}</Text>
                            </Box>
                            <Box bg={data.is_balanced ? "green.50" : "yellow.50"} p={4} borderRadius="md" textAlign="center">
                                <Badge colorScheme={data.is_balanced ? "green" : "yellow"} fontSize="lg" px={4} py={2}>
                                    {data.is_balanced ? t('reports.trialBalance.balanced') : t('reports.trialBalance.unbalanced')}
                                </Badge>
                            </Box>
                        </SimpleGrid>

                        {data.accounts && data.accounts.length > 0 && (
                            <Box bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
                                <TableContainer maxH="500px" overflowY="auto">
                                    <Table size="sm" variant="striped">
                                        <Thead bg="cyan.600" position="sticky" top={0} zIndex={1}>
                                            <Tr>
                                                <Th color="white">{t('reports.trialBalance.accountCode')}</Th>
                                                <Th color="white">{t('reports.trialBalance.accountName')}</Th>
                                                <Th color="white">{t('reports.trialBalance.accountType')}</Th>
                                                <Th color="white" isNumeric>{t('reports.trialBalance.debit')}</Th>
                                                <Th color="white" isNumeric>{t('reports.trialBalance.credit')}</Th>
                                            </Tr>
                                        </Thead>
                                        <Tbody>
                                            {data.accounts.map((acc: any, idx: number) => (
                                                <Tr key={idx}>
                                                    <Td fontFamily="mono">{acc.account_code}</Td>
                                                    <Td>{acc.account_name}</Td>
                                                    <Td><Badge colorScheme="gray">{acc.account_type}</Badge></Td>
                                                    <Td isNumeric color="blue.600">{acc.debit_balance > 0 ? formatCurrency(acc.debit_balance) : '-'}</Td>
                                                    <Td isNumeric color="red.600">{acc.credit_balance > 0 ? formatCurrency(acc.credit_balance) : '-'}</Td>
                                                </Tr>
                                            ))}
                                        </Tbody>
                                        <Tfoot bg="cyan.700" position="sticky" bottom={0}>
                                            <Tr>
                                                <Th color="white" colSpan={3}>TOTAL</Th>
                                                <Th color="white" isNumeric>{formatCurrency(data.total_debits)}</Th>
                                                <Th color="white" isNumeric>{formatCurrency(data.total_credits)}</Th>
                                            </Tr>
                                        </Tfoot>
                                    </Table>
                                </TableContainer>
                            </Box>
                        )}

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

export default TrialBalancePage;
