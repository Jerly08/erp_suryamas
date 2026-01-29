import React, { useState, useEffect, useCallback } from 'react';
import {
    Box,
    Button,
    HStack,
    Table,
    Thead,
    Tbody,
    Tr,
    Th,
    Td,
    Text,
    Input,
    Spinner,
    useToast,
    VStack,
    Card,
    CardBody,
    Heading,
    Divider,
    SimpleGrid,
    Badge,
} from '@chakra-ui/react';
import { useTranslation } from '@/hooks/useTranslation';
import { ssotBalanceSheetReportService, SSOTBalanceSheetData, BSAccountItem } from '@/services/ssotBalanceSheetReportService';
import { FiRefreshCw } from 'react-icons/fi';

export default function BalanceSheetView() {
    const { t } = useTranslation();
    const toast = useToast();

    const [loading, setLoading] = useState(false);
    const [data, setData] = useState<SSOTBalanceSheetData | null>(null);

    const [asOfDate, setAsOfDate] = useState(
        new Date().toISOString().split('T')[0]
    );

    const fetchData = useCallback(async () => {
        setLoading(true);
        try {
            const result = await ssotBalanceSheetReportService.generateSSOTBalanceSheet({
                as_of_date: asOfDate,
            });
            setData(result);
        } catch (error) {
            console.error('Failed to fetch balance sheet', error);
            toast({
                title: t('common.error'),
                description: 'Failed to fetch balance sheet',
                status: 'error',
                duration: 3000,
            });
        } finally {
            setLoading(false);
        }
    }, [asOfDate, t, toast]);

    useEffect(() => {
        fetchData();
    }, []); // Initial load

    const formatCurrency = (val: number | undefined) => {
        if (val === undefined) return '-';
        return new Intl.NumberFormat('id-ID', {
            style: 'currency',
            currency: 'IDR',
            minimumFractionDigits: 0
        }).format(val);
    };

    const AccountSection = ({ title, items, total, color = 'gray' }: { title: string, items: BSAccountItem[], total: number, color?: string }) => (
        <Box mb={6}>
            <Text fontWeight="bold" fontSize="md" mb={2} color={`${color}.700`}>{title}</Text>
            <Table size="sm" variant="simple">
                <Tbody>
                    {(items || []).map((item, idx) => (
                        <Tr key={idx}>
                            <Td pl={4}>{item.account_code} - {item.account_name}</Td>
                            <Td isNumeric>{formatCurrency(item.amount)}</Td>
                        </Tr>
                    ))}
                    <Tr bg={`${color}.50`}>
                        <Td fontWeight="semibold">Total {title}</Td>
                        <Td isNumeric fontWeight="bold">{formatCurrency(total)}</Td>
                    </Tr>
                </Tbody>
            </Table>
        </Box>
    );

    return (
        <Box>
            <Card variant="outline" mb={4}>
                <CardBody py={4}>
                    <HStack justify="space-between">
                        <HStack spacing={4} align="flex-end">
                            <Box>
                                <Text fontSize="xs" color="gray.500" mb={1}>{t('journal.filters.asOfDate') || 'As of Date'}</Text>
                                <Input
                                    type="date"
                                    value={asOfDate}
                                    onChange={(e) => setAsOfDate(e.target.value)}
                                    size="sm"
                                    width="140px"
                                />
                            </Box>
                            <Button
                                leftIcon={<FiRefreshCw />}
                                size="sm"
                                colorScheme="blue"
                                onClick={fetchData}
                                isLoading={loading}
                            >
                                {t('common.refresh')}
                            </Button>
                        </HStack>
                    </HStack>
                </CardBody>
            </Card>

            {loading && !data && (
                <Box textAlign="center" py={10}><Spinner /></Box>
            )}

            {data && (
                <Card variant="outline">
                    <CardBody>
                        <VStack spacing={6} align="stretch">
                            <Box textAlign="center" mb={4}>
                                <Heading size="lg">{data.company?.name}</Heading>
                                <Text fontSize="xl" fontWeight="medium">{t('journal.balanceSheet')}</Text>
                                <Text color="gray.500">{t('journal.balanceSheetReport.asOf', { date: new Date(data.as_of_date).toLocaleDateString('id-ID', { dateStyle: 'long' }) })}</Text>
                            </Box>

                            <SimpleGrid columns={{ base: 1, md: 2 }} spacing={8}>
                                {/* ASSETS */}
                                <Box>
                                    <Heading size="md" mb={4} color="green.600" borderBottomWidth="2px" borderColor="green.200" pb={2}>{t('journal.balanceSheetReport.assets')}</Heading>
                                    <AccountSection
                                        title={t('journal.balanceSheetReport.currentAssets')}
                                        items={data.assets.current_assets.items}
                                        total={data.assets.current_assets.total_current_assets}
                                        color="green"
                                    />
                                    <AccountSection
                                        title={t('journal.balanceSheetReport.nonCurrentAssets')}
                                        items={data.assets.non_current_assets.items}
                                        total={data.assets.non_current_assets.total_non_current_assets}
                                        color="teal"
                                    />
                                    <Box bg="green.100" p={2} borderRadius="md">
                                        <HStack justify="space-between">
                                            <Text fontWeight="bold" fontSize="lg">{t('journal.balanceSheetReport.totalAssets')}</Text>
                                            <Text fontWeight="bold" fontSize="lg">{formatCurrency(data.assets.total_assets)}</Text>
                                        </HStack>
                                    </Box>
                                </Box>

                                {/* LIABILITIES & EQUITY */}
                                <Box>
                                    <Heading size="md" mb={4} color="red.600" borderBottomWidth="2px" borderColor="red.200" pb={2}>{t('journal.balanceSheetReport.liabilitiesEquity')}</Heading>
                                    <AccountSection
                                        title={t('journal.balanceSheetReport.currentLiabilities')}
                                        items={data.liabilities.current_liabilities.items}
                                        total={data.liabilities.current_liabilities.total_current_liabilities}
                                        color="red"
                                    />
                                    <AccountSection
                                        title={t('journal.balanceSheetReport.nonCurrentLiabilities')}
                                        items={data.liabilities.non_current_liabilities.items}
                                        total={data.liabilities.non_current_liabilities.total_non_current_liabilities}
                                        color="orange"
                                    />
                                    <Box bg="red.50" p={2} borderRadius="md" mb={4}>
                                        <HStack justify="space-between">
                                            <Text fontWeight="bold">{t('journal.balanceSheetReport.totalLiabilities')}</Text>
                                            <Text fontWeight="bold">{formatCurrency(data.liabilities.total_liabilities)}</Text>
                                        </HStack>
                                    </Box>

                                    <Heading size="sm" mb={4} color="blue.600">{t('journal.balanceSheetReport.equity')}</Heading>
                                    <AccountSection
                                        title={t('journal.balanceSheetReport.equitySection')}
                                        items={data.equity.items}
                                        total={data.equity.total_equity}
                                        color="blue"
                                    />
                                    <Box bg="blue.50" p={2} borderRadius="md" mb={4}>
                                        <HStack justify="space-between">
                                            <Text fontWeight="bold">{t('journal.balanceSheetReport.totalEquity')}</Text>
                                            <Text fontWeight="bold">{formatCurrency(data.equity.total_equity)}</Text>
                                        </HStack>
                                    </Box>

                                    <Box bg="purple.100" p={2} borderRadius="md">
                                        <HStack justify="space-between">
                                            <Text fontWeight="bold" fontSize="lg">{t('journal.balanceSheetReport.totalLiabilitiesEquity')}</Text>
                                            <Text fontWeight="bold" fontSize="lg">{formatCurrency(data.total_liabilities_and_equity)}</Text>
                                        </HStack>
                                    </Box>
                                </Box>
                            </SimpleGrid>

                            {!data.is_balanced && (
                                <Box bg="red.100" p={4} borderRadius="md" textAlign="center">
                                    <Text color="red.700" fontWeight="bold">
                                        {t('journal.balanceSheetReport.unbalanced', { amount: formatCurrency(data.balance_difference) })}
                                    </Text>
                                </Box>
                            )}
                        </VStack>
                    </CardBody>
                </Card>
            )}
        </Box>
    );
}
