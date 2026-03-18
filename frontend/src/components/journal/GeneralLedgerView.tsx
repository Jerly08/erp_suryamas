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
    Select,
    Spinner,
    useToast,
    VStack,
    Card,
    CardBody,
    Heading,
    Badge,
    IconButton,
    Tooltip,
    useDisclosure,
    useColorModeValue,
} from '@chakra-ui/react';
import { useTranslation } from '@/hooks/useTranslation';
import { ssotGeneralLedgerService, SSOTGeneralLedgerData, GeneralLedgerEntry } from '@/services/ssotGeneralLedgerService';
import { ssotJournalService, SSOTJournalEntry } from '@/services/ssotJournalService';
import { accountService } from '@/services/accountService';
import { Account } from '@/types/account';
import { FiRefreshCw, FiSearch } from 'react-icons/fi';
import JournalDetailModal from './JournalDetailModal';
import ExportButton from '@/components/common/ExportButton';
import { exportGeneralLedgerToPDF, exportGeneralLedgerToCSV } from '@/utils/journalExport';

export default function GeneralLedgerView() {
    const { t } = useTranslation();
    const toast = useToast();
    const { isOpen: isDetailOpen, onOpen: onDetailOpen, onClose: onDetailClose } = useDisclosure();
    const textColor = useColorModeValue('gray.800', 'gray.100');
    const mutedTextColor = useColorModeValue('gray.500', 'gray.400');
    const cardBg = useColorModeValue('white', 'gray.800');
    const borderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
    const summaryBg = useColorModeValue('gray.50', 'whiteAlpha.100');
    const tableHeaderBg = useColorModeValue('gray.50', 'whiteAlpha.100');
    const tableHoverBg = useColorModeValue('gray.50', 'whiteAlpha.100');
    const inputBg = useColorModeValue('white', 'whiteAlpha.50');
    const inputBorderColor = useColorModeValue('gray.300', 'whiteAlpha.300');
    const inputHoverBorderColor = useColorModeValue('gray.400', 'whiteAlpha.400');
    const inputFocusBorderColor = useColorModeValue('blue.500', 'blue.300');
    const warningTextColor = useColorModeValue('orange.500', 'orange.300');
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
    const [data, setData] = useState<SSOTGeneralLedgerData | null>(null);
    const [selectedEntry, setSelectedEntry] = useState<SSOTJournalEntry | undefined>(undefined);
    const [itemLoading, setItemLoading] = useState<number | null>(null);

    // Filters
    const [accountId, setAccountId] = useState('');
    const [accounts, setAccounts] = useState<Account[]>([]);
    // Default to beginning of the current year for start date
    const [startDate, setStartDate] = useState(
        new Date(new Date().getFullYear(), 0, 1).toISOString().split('T')[0]
    );
    const [endDate, setEndDate] = useState(
        new Date().toISOString().split('T')[0]
    );
    const [exporting, setExporting] = useState(false);
    const isAllAccountsMode = !accountId;

    const handleExportPDF = () => {
        if (!data) return;
        try {
            setExporting(true);
            exportGeneralLedgerToPDF(data, startDate, endDate);
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
            exportGeneralLedgerToCSV(data, startDate, endDate);
            toast({ title: t('common.success'), description: 'CSV berhasil diunduh', status: 'success', duration: 3000 });
        } catch (error) {
            toast({ title: t('common.error'), description: 'Gagal mengexport CSV', status: 'error', duration: 3000 });
        } finally {
            setExporting(false);
        }
    };

    // Load accounts
    useEffect(() => {
        const loadAccounts = async () => {
            try {
                const token = localStorage.getItem('token') || '';
                if (!token) return;
                const loadedAccounts = await accountService.getAccounts(token);
                loadedAccounts.sort((a, b) => a.code.localeCompare(b.code));
                setAccounts(loadedAccounts);
            } catch (error) {
                console.error('Failed to load accounts', error);
            }
        };
        loadAccounts();
    }, []);

    const fetchData = useCallback(async () => {
        setLoading(true);
        try {
            const result = await ssotGeneralLedgerService.generateSSOTGeneralLedger({
                start_date: startDate,
                end_date: endDate,
                account_id: accountId || undefined,
            });
            setData(result);
        } catch (error) {
            console.error('Failed to fetch general ledger', error);
            toast({
                title: t('common.error'),
                description: 'Failed to fetch general ledger',
                status: 'error',
                duration: 3000,
            });
        } finally {
            setLoading(false);
        }
    }, [accountId, startDate, endDate, t, toast]);

    const handleViewDetail = async (journalId?: number) => {
        if (!journalId) return;

        setItemLoading(journalId);
        try {
            const entry = await ssotJournalService.getJournalEntry(journalId);
            setSelectedEntry(entry);
            onDetailOpen();
        } catch (error) {
            console.error('Failed to fetch journal detail', error);
            toast({
                title: t('common.error'),
                description: 'Failed to fetch journal details',
                status: 'error',
                duration: 3000,
            });
        } finally {
            setItemLoading(null);
        }
    };

    // Initial fetch
    useEffect(() => {
        fetchData();
    }, []);

    const formatCurrency = (val: number | undefined) => {
        if (val === undefined) return '-';
        return new Intl.NumberFormat('id-ID', {
            style: 'currency',
            currency: 'IDR',
            minimumFractionDigits: 0
        }).format(val);
    };

    const formatDate = (dateStr: string) => {
        if (!dateStr) return '-';
        return new Date(dateStr).toLocaleDateString('id-ID', {
            day: '2-digit',
            month: 'short',
            year: 'numeric'
        });
    };

    return (
        <Box>
            <VStack spacing={4} align="stretch">
                <Card variant="outline" bg={cardBg} borderColor={borderColor}>
                    <CardBody py={4}>
                        <HStack justify="space-between" wrap="wrap" spacing={4}>
                            <HStack wrap="wrap" spacing={4} align="flex-end">
                                <Box minW="250px">
                                    <Text fontSize="xs" color={mutedTextColor} mb={1}>{t('journal.filters.account')}</Text>
                                    <Select
                                        value={accountId}
                                        onChange={(e) => setAccountId(e.target.value)}
                                        size="sm"
                                        placeholder={t('journal.generalLedgerAccountPlaceholder') || 'Pilih akun untuk Buku Besar'}
                                        {...fieldStyles}
                                    >
                                        {accounts.map(acc => (
                                            <option key={acc.id} value={acc.id}>
                                                {acc.code} - {acc.name}
                                            </option>
                                        ))}
                                    </Select>
                                </Box>
                                {!accountId && (
                                    <Box maxW="360px">
                                        <Text fontSize="xs" color={mutedTextColor}>
                                            {t('journal.generalLedgerAllAccountsHelper') || 'Tanpa memilih akun, tampilan ini menjadi aktivitas jurnal periode lintas akun. Untuk buku besar yang menampilkan saldo, pilih satu akun spesifik.'}
                                        </Text>
                                    </Box>
                                )}
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

                {data && (
                    <Card variant="outline" bg={cardBg} borderColor={borderColor}>
                        <CardBody>
                            <VStack spacing={4} align="stretch">
                                <HStack justify="space-between">
                                    <Box>
                                        {accountId ? (
                                            <>
                                                <Heading size="sm" color={textColor}>{data.account?.account_name}</Heading>
                                                <Text fontSize="sm" color={mutedTextColor}>{data.account?.account_code}</Text>
                                            </>
                                        ) : (
                                            <VStack align="start" spacing={0}>
                                                <Heading size="sm" color={textColor}>
                                                    {t('journal.generalLedgerActivity') || 'Aktivitas Jurnal Periode'}
                                                </Heading>
                                                <Text fontSize="sm" color={mutedTextColor}>
                                                    {t('journal.generalLedgerActivitySubtitle') || 'Ringkasan mutasi debit dan kredit lintas akun untuk periode terpilih.'}
                                                </Text>
                                            </VStack>
                                        )}
                                    </Box>
                                    {accountId && (
                                        <VStack align="end" spacing={0}>
                                            <Text fontSize="xs" color={mutedTextColor}>{t('journal.table.closingBalance')}</Text>
                                            <Text fontWeight="bold" fontSize="lg" color="blue.600">
                                                {formatCurrency(data.closing_balance)}
                                            </Text>
                                        </VStack>
                                    )}
                                </HStack>

                                <HStack spacing={8} bg={summaryBg} p={2} borderRadius="md" border="1px solid" borderColor={borderColor}>
                                    {accountId && (
                                        <Box>
                                            <Text fontSize="xs" color={mutedTextColor}>{t('journal.table.openingBalance')}</Text>
                                            <Text fontWeight="medium" color={textColor}>{formatCurrency(data.opening_balance)}</Text>
                                        </Box>
                                    )}
                                    <Box>
                                        <Text fontSize="xs" color={mutedTextColor}>
                                            {isAllAccountsMode
                                                ? (t('journal.table.periodDebit') || 'Mutasi Debit Periode')
                                                : t('journal.table.totalDebit')}
                                        </Text>
                                        <Text fontWeight="medium" color="green.600">{formatCurrency(data.total_debits)}</Text>
                                    </Box>
                                    <Box>
                                        <Text fontSize="xs" color={mutedTextColor}>
                                            {isAllAccountsMode
                                                ? (t('journal.table.periodCredit') || 'Mutasi Kredit Periode')
                                                : t('journal.table.totalCredit')}
                                        </Text>
                                        <Text fontWeight="medium" color="red.600">{formatCurrency(data.total_credits)}</Text>
                                    </Box>
                                    {!accountId && (
                                        <VStack align="start" spacing={0}>
                                            <Text fontSize="xs" color={warningTextColor} fontStyle="italic">
                                                {t('journal.selectAccountForBalance') || 'Pilih akun spesifik untuk melihat saldo awal & akhir'}
                                            </Text>
                                            <Text fontSize="xs" color={mutedTextColor}>
                                                {t('journal.allAccountsTurnoverNote') || 'Mode Semua Akun menampilkan total mutasi jurnal periode, bukan total saldo akhir neraca.'}
                                            </Text>
                                        </VStack>
                                    )}
                                </HStack>

                                <Box overflowX="auto" borderWidth="1px" borderRadius="lg" borderColor={borderColor} bg={cardBg}>
                                    <Table size="sm" variant="simple" color={textColor}>
                                        <Thead bg={tableHeaderBg}>
                                            <Tr>
                                                <Th color={mutedTextColor}>{t('journal.table.date')}</Th>
                                                <Th color={mutedTextColor}>{t('journal.table.ref')}</Th>
                                                <Th color={mutedTextColor}>{t('journal.table.description')}</Th>
                                                <Th isNumeric color={mutedTextColor}>{t('journal.table.debit')}</Th>
                                                <Th isNumeric color={mutedTextColor}>{t('journal.table.credit')}</Th>
                                                {!isAllAccountsMode && (
                                                    <Th isNumeric color={mutedTextColor}>{t('journal.table.balance')}</Th>
                                                )}
                                                <Th width="50px" color={mutedTextColor}>{t('journal.table.action')}</Th>
                                            </Tr>
                                        </Thead>
                                        <Tbody>
                                            {(data.transactions || []).length === 0 ? (
                                                <Tr>
                                                    <Td colSpan={isAllAccountsMode ? 6 : 7} textAlign="center" py={4} color={mutedTextColor}>No transactions found</Td>
                                                </Tr>
                                            ) : (
                                                (data.transactions || []).map((entry, idx) => (
                                                    <Tr key={idx} _hover={{ bg: tableHoverBg }}>
                                                        <Td>{formatDate(entry.date)}</Td>
                                                        <Td>{entry.journal_code}</Td>
                                                        <Td maxW="300px" whiteSpace="normal">{entry.description}</Td>
                                                        <Td isNumeric>{entry.debit_amount > 0 ? formatCurrency(entry.debit_amount) : '-'}</Td>
                                                        <Td isNumeric>{entry.credit_amount > 0 ? formatCurrency(entry.credit_amount) : '-'}</Td>
                                                        {!isAllAccountsMode && (
                                                            <Td isNumeric fontWeight="semibold">{formatCurrency(entry.balance)}</Td>
                                                        )}
                                                        <Td>
                                                            {entry.journal_id && (
                                                                <Tooltip label="View Detail">
                                                                    <IconButton
                                                                        aria-label="View Detail"
                                                                        icon={<FiSearch />}
                                                                        size="xs"
                                                                        variant="ghost"
                                                                        colorScheme="blue"
                                                                        onClick={() => handleViewDetail(entry.journal_id)}
                                                                        isLoading={itemLoading === entry.journal_id}
                                                                    />
                                                                </Tooltip>
                                                            )}
                                                        </Td>
                                                    </Tr>
                                                ))
                                            )}
                                        </Tbody>
                                    </Table>
                                </Box>
                            </VStack>
                        </CardBody>
                    </Card>
                )}
            </VStack>

            <JournalDetailModal
                isOpen={isDetailOpen}
                onClose={onDetailClose}
                entry={selectedEntry}
            />
        </Box>
    );
}
