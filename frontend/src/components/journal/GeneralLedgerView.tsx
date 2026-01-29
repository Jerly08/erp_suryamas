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
} from '@chakra-ui/react';
import { useTranslation } from '@/hooks/useTranslation';
import { ssotGeneralLedgerService, SSOTGeneralLedgerData, GeneralLedgerEntry } from '@/services/ssotGeneralLedgerService';
import { ssotJournalService, SSOTJournalEntry } from '@/services/ssotJournalService';
import { accountService } from '@/services/accountService';
import { Account } from '@/types/account';
import { FiRefreshCw, FiSearch } from 'react-icons/fi';
import JournalDetailModal from './JournalDetailModal';

export default function GeneralLedgerView() {
    const { t } = useTranslation();
    const toast = useToast();
    const { isOpen: isDetailOpen, onOpen: onDetailOpen, onClose: onDetailClose } = useDisclosure();

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
                <Card variant="outline">
                    <CardBody py={4}>
                        <HStack wrap="wrap" spacing={4} align="flex-end">
                            <Box minW="250px">
                                <Text fontSize="xs" color="gray.500" mb={1}>{t('journal.filters.account')}</Text>
                                <Select
                                    value={accountId}
                                    onChange={(e) => setAccountId(e.target.value)}
                                    size="sm"
                                    placeholder="Select Account"
                                >
                                    {accounts.map(acc => (
                                        <option key={acc.id} value={acc.id}>
                                            {acc.code} - {acc.name}
                                        </option>
                                    ))}
                                </Select>
                            </Box>
                            <Box>
                                <Text fontSize="xs" color="gray.500" mb={1}>{t('journal.filters.startDate')}</Text>
                                <Input
                                    type="date"
                                    value={startDate}
                                    onChange={(e) => setStartDate(e.target.value)}
                                    size="sm"
                                    width="140px"
                                />
                            </Box>
                            <Box>
                                <Text fontSize="xs" color="gray.500" mb={1}>{t('journal.filters.endDate')}</Text>
                                <Input
                                    type="date"
                                    value={endDate}
                                    onChange={(e) => setEndDate(e.target.value)}
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
                                {t('journal.filters.generate')}
                            </Button>
                        </HStack>
                    </CardBody>
                </Card>

                {data && (
                    <Card variant="outline">
                        <CardBody>
                            <VStack spacing={4} align="stretch">
                                <HStack justify="space-between">
                                    <Box>
                                        <Heading size="sm">{data.account?.account_name}</Heading>
                                        <Text fontSize="sm" color="gray.500">{data.account?.account_code}</Text>
                                    </Box>
                                    <VStack align="end" spacing={0}>
                                        <Text fontSize="xs" color="gray.500">{t('journal.table.closingBalance')}</Text>
                                        <Text fontWeight="bold" fontSize="lg" color="blue.600">
                                            {formatCurrency(data.closing_balance)}
                                        </Text>
                                    </VStack>
                                </HStack>

                                <HStack spacing={8} bg="gray.50" p={2} borderRadius="md">
                                    <Box>
                                        <Text fontSize="xs" color="gray.500">{t('journal.table.openingBalance')}</Text>
                                        <Text fontWeight="medium">{formatCurrency(data.opening_balance)}</Text>
                                    </Box>
                                    <Box>
                                        <Text fontSize="xs" color="gray.500">{t('journal.table.totalDebit')}</Text>
                                        <Text fontWeight="medium" color="green.600">{formatCurrency(data.total_debits)}</Text>
                                    </Box>
                                    <Box>
                                        <Text fontSize="xs" color="gray.500">{t('journal.table.totalCredit')}</Text>
                                        <Text fontWeight="medium" color="red.600">{formatCurrency(data.total_credits)}</Text>
                                    </Box>
                                </HStack>

                                <Box overflowX="auto">
                                    <Table size="sm" variant="simple">
                                        <Thead>
                                            <Tr>
                                                <Th>{t('journal.table.date')}</Th>
                                                <Th>{t('journal.table.ref')}</Th>
                                                <Th>{t('journal.table.description')}</Th>
                                                <Th isNumeric>{t('journal.table.debit')}</Th>
                                                <Th isNumeric>{t('journal.table.credit')}</Th>
                                                <Th isNumeric>{t('journal.table.balance')}</Th>
                                                <Th width="50px">{t('journal.table.action')}</Th>
                                            </Tr>
                                        </Thead>
                                        <Tbody>
                                            {(data.transactions || []).length === 0 ? (
                                                <Tr>
                                                    <Td colSpan={7} textAlign="center" py={4}>No transactions found</Td>
                                                </Tr>
                                            ) : (
                                                (data.transactions || []).map((entry, idx) => (
                                                    <Tr key={idx} _hover={{ bg: "gray.50" }}>
                                                        <Td>{formatDate(entry.date)}</Td>
                                                        <Td>{entry.journal_code}</Td>
                                                        <Td maxW="300px" whiteSpace="normal">{entry.description}</Td>
                                                        <Td isNumeric>{entry.debit_amount > 0 ? formatCurrency(entry.debit_amount) : '-'}</Td>
                                                        <Td isNumeric>{entry.credit_amount > 0 ? formatCurrency(entry.credit_amount) : '-'}</Td>
                                                        <Td isNumeric fontWeight="semibold">{formatCurrency(entry.balance)}</Td>
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
