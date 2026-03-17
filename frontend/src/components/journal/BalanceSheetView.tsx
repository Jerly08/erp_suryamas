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
    SimpleGrid,
    useColorModeValue,
} from '@chakra-ui/react';
import { useTranslation } from '@/hooks/useTranslation';
import { ssotBalanceSheetReportService, SSOTBalanceSheetData, BSAccountItem } from '@/services/ssotBalanceSheetReportService';
import { FiRefreshCw } from 'react-icons/fi';
import ExportButton from '@/components/common/ExportButton';
import { exportBalanceSheetToPDF, exportBalanceSheetToCSV } from '@/utils/journalExport';

export default function BalanceSheetView() {
    const { t } = useTranslation();
    const toast = useToast();
    const cardBg = useColorModeValue('white', 'gray.800');
    const textColor = useColorModeValue('gray.800', 'gray.100');
    const mutedTextColor = useColorModeValue('gray.500', 'gray.400');
    const borderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
    const inputBg = useColorModeValue('white', 'whiteAlpha.50');
    const inputBorderColor = useColorModeValue('gray.300', 'whiteAlpha.300');
    const inputHoverBorderColor = useColorModeValue('gray.400', 'whiteAlpha.400');
    const inputFocusBorderColor = useColorModeValue('blue.500', 'blue.300');
    const assetHeadingColor = useColorModeValue('green.600', 'green.300');
    const assetHeadingBorderColor = useColorModeValue('green.200', 'green.700');
    const totalAssetsBg = useColorModeValue('green.100', 'green.900');
    const liabilitiesHeadingColor = useColorModeValue('red.600', 'red.300');
    const liabilitiesHeadingBorderColor = useColorModeValue('red.200', 'red.700');
    const totalLiabilitiesBg = useColorModeValue('red.50', 'red.900');
    const equityHeadingColor = useColorModeValue('blue.600', 'blue.300');
    const totalEquityBg = useColorModeValue('blue.50', 'blue.900');
    const totalLiabilitiesEquityBg = useColorModeValue('purple.100', 'purple.900');
    const unbalancedBg = useColorModeValue('red.100', 'red.900');
    const unbalancedTextColor = useColorModeValue('red.700', 'red.200');
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

    const [loading, setLoading] = useState(false);
    const [data, setData] = useState<SSOTBalanceSheetData | null>(null);

    const [startDate, setStartDate] = useState(
        new Date(new Date().getFullYear(), 0, 1).toISOString().split('T')[0]
    );
    const [endDate, setEndDate] = useState(
        new Date().toISOString().split('T')[0]
    );
    const [exporting, setExporting] = useState(false);

    const handleExportPDF = () => {
        if (!data) return;
        try {
            setExporting(true);
            exportBalanceSheetToPDF(data, endDate);
            toast({ title: t('common.success'), description: 'PDF berhasil diunduh', status: 'success', duration: 3000 });
        } catch (error) {
            toast({ title: t('common.error'), description: 'Gagal mengexport PDF', status: 'error', duration: 3000 });
        } finally {
            setExporting(false);
        }
    };

    const handleExportCSV = () => {
        if (!data) return;
        try {
            setExporting(true);
            exportBalanceSheetToCSV(data, endDate);
            toast({ title: t('common.success'), description: 'CSV berhasil diunduh', status: 'success', duration: 3000 });
        } catch (error) {
            toast({ title: t('common.error'), description: 'Gagal mengexport CSV', status: 'error', duration: 3000 });
        } finally {
            setExporting(false);
        }
    };

    const fetchData = useCallback(async () => {
        setLoading(true);
        try {
            const result = await ssotBalanceSheetReportService.generateSSOTBalanceSheet({
                as_of_date: endDate,
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
    }, [endDate, t, toast]);

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

    const AccountSection = ({ title, items, total, color = 'gray' }: { title: string, items: BSAccountItem[], total: number, color?: string }) => {
        const sectionTitleColor = useColorModeValue(`${color}.700`, `${color}.200`);
        const sectionTotalBg = useColorModeValue(`${color}.50`, `${color}.900`);

        return (
        <Box mb={6}>
            <Text fontWeight="bold" fontSize="md" mb={2} color={sectionTitleColor}>{title}</Text>
            <Table size="sm" variant="simple" color={textColor}>
                <Tbody>
                    {(items || []).map((item, idx) => (
                        <Tr key={idx}>
                            <Td pl={4}>{item.account_code} - {item.account_name}</Td>
                            <Td isNumeric>{formatCurrency(item.amount)}</Td>
                        </Tr>
                    ))}
                    <Tr bg={sectionTotalBg}>
                        <Td fontWeight="semibold">Total {title}</Td>
                        <Td isNumeric fontWeight="bold">{formatCurrency(total)}</Td>
                    </Tr>
                </Tbody>
            </Table>
        </Box>
    );
    };

    return (
        <Box>
            <Card variant="outline" mb={4} bg={cardBg} borderColor={borderColor}>
                <CardBody py={4}>
                    <HStack justify="space-between">
                        <HStack spacing={4} align="flex-end">
                            <Box>
                                <Text fontSize="xs" color={mutedTextColor} mb={1}>{t('journal.filters.startDate')}</Text>
                                <Input
                                    type="date"
                                    value={startDate}
                                    onChange={(e) => setStartDate(e.target.value)}
                                    size="sm"
                                    width="140px"
                                    {...fieldStyles}
                                />
                            </Box>
                            <Box>
                                <Text fontSize="xs" color={mutedTextColor} mb={1}>{t('journal.filters.endDate')}</Text>
                                <Input
                                    type="date"
                                    value={endDate}
                                    onChange={(e) => setEndDate(e.target.value)}
                                    size="sm"
                                    width="140px"
                                    {...fieldStyles}
                                />
                            </Box>
                            <Button
                                leftIcon={<FiRefreshCw />}
                                size="sm"
                                colorScheme="blue"
                                onClick={fetchData}
                                isLoading={loading}
                            >
                                {t('journal.filters.generate')}
                            </Button>
                        </HStack>
                        <ExportButton
                            onExportPDF={handleExportPDF}
                            onExportCSV={handleExportCSV}
                            isLoading={exporting}
                            disabled={!data}
                            showExcel={false}
                            showPDF={true}
                            showCSV={true}
                            size="sm"
                        />
                    </HStack>
                </CardBody>
            </Card>

            {loading && !data && (
                <Box textAlign="center" py={10}><Spinner /></Box>
            )}

            {data && (
                <Card variant="outline" bg={cardBg} borderColor={borderColor}>
                    <CardBody>
                        <VStack spacing={6} align="stretch">
                            <Box textAlign="center" mb={4}>
                                <Heading size="lg" color={textColor}>{data.company?.name}</Heading>
                                <Text fontSize="xl" fontWeight="medium" color={textColor}>{t('journal.balanceSheet')}</Text>
                                <Text color={mutedTextColor}>
                                    {t('journal.balanceSheetReport.period') || 'Periode'}: {new Date(startDate).toLocaleDateString('id-ID', { dateStyle: 'long' })} - {new Date(endDate).toLocaleDateString('id-ID', { dateStyle: 'long' })}
                                </Text>
                            </Box>

                            <SimpleGrid columns={{ base: 1, md: 2 }} spacing={8}>
                                {/* ASSETS */}
                                <Box>
                                    <Heading size="md" mb={4} color={assetHeadingColor} borderBottomWidth="2px" borderColor={assetHeadingBorderColor} pb={2}>{t('journal.balanceSheetReport.assets')}</Heading>
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
                                    <Box bg={totalAssetsBg} p={2} borderRadius="md">
                                        <HStack justify="space-between">
                                            <Text fontWeight="bold" fontSize="lg">{t('journal.balanceSheetReport.totalAssets')}</Text>
                                            <Text fontWeight="bold" fontSize="lg">{formatCurrency(data.assets.total_assets)}</Text>
                                        </HStack>
                                    </Box>
                                </Box>

                                {/* LIABILITIES & EQUITY */}
                                <Box>
                                    <Heading size="md" mb={4} color={liabilitiesHeadingColor} borderBottomWidth="2px" borderColor={liabilitiesHeadingBorderColor} pb={2}>{t('journal.balanceSheetReport.liabilitiesEquity')}</Heading>
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
                                    <Box bg={totalLiabilitiesBg} p={2} borderRadius="md" mb={4}>
                                        <HStack justify="space-between">
                                            <Text fontWeight="bold">{t('journal.balanceSheetReport.totalLiabilities')}</Text>
                                            <Text fontWeight="bold">{formatCurrency(data.liabilities.total_liabilities)}</Text>
                                        </HStack>
                                    </Box>

                                    <Heading size="sm" mb={4} color={equityHeadingColor}>{t('journal.balanceSheetReport.equity')}</Heading>
                                    <AccountSection
                                        title={t('journal.balanceSheetReport.equitySection')}
                                        items={data.equity.items}
                                        total={data.equity.total_equity}
                                        color="blue"
                                    />
                                    <Box bg={totalEquityBg} p={2} borderRadius="md" mb={4}>
                                        <HStack justify="space-between">
                                            <Text fontWeight="bold">{t('journal.balanceSheetReport.totalEquity')}</Text>
                                            <Text fontWeight="bold">{formatCurrency(data.equity.total_equity)}</Text>
                                        </HStack>
                                    </Box>

                                    <Box bg={totalLiabilitiesEquityBg} p={2} borderRadius="md">
                                        <HStack justify="space-between">
                                            <Text fontWeight="bold" fontSize="lg">{t('journal.balanceSheetReport.totalLiabilitiesEquity')}</Text>
                                            <Text fontWeight="bold" fontSize="lg">{formatCurrency(data.total_liabilities_and_equity)}</Text>
                                        </HStack>
                                    </Box>
                                </Box>
                            </SimpleGrid>

                            {!data.is_balanced && (
                                <Box bg={unbalancedBg} p={4} borderRadius="md" textAlign="center">
                                    <Text color={unbalancedTextColor} fontWeight="bold">
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
