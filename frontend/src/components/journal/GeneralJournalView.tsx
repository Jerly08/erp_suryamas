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
    Badge,
    Text,
    Input,
    Select,
    Spinner,
    useToast,
    VStack,
    Heading,
    Menu,
    MenuButton,
    MenuList,
    MenuItem,
    IconButton,
} from '@chakra-ui/react';
import { useTranslation } from '@/hooks/useTranslation';
import { ssotJournalService, SSOTJournalEntry } from '@/services/ssotJournalService';
import { FiRefreshCw, FiCalendar, FiPlus, FiMoreVertical, FiEdit2, FiTrash2, FiCheckCircle, FiEye } from 'react-icons/fi';
import ManualEntryForm from './ManualEntryForm';
import JournalDetailModal from './JournalDetailModal';

export default function GeneralJournalView() {
    const { t } = useTranslation();
    const toast = useToast();

    const [entries, setEntries] = useState<SSOTJournalEntry[]>([]);
    const [loading, setLoading] = useState(false);
    const [showCreateForm, setShowCreateForm] = useState(false);
    const [selectedEntry, setSelectedEntry] = useState<SSOTJournalEntry | undefined>(undefined);

    // View Modal State
    const [viewEntry, setViewEntry] = useState<SSOTJournalEntry | undefined>(undefined);
    const [isViewModalOpen, setIsViewModalOpen] = useState(false);

    // Filters
    const [startDate, setStartDate] = useState(
        new Date(new Date().getFullYear(), 0, 1).toISOString().split('T')[0]
    );
    const [endDate, setEndDate] = useState(
        new Date().toISOString().split('T')[0]
    );
    const [searchTerm, setSearchTerm] = useState('');

    const fetchEntries = useCallback(async () => {
        setLoading(true);
        try {
            const response = await ssotJournalService.getJournalEntries({
                start_date: startDate,
                end_date: endDate,
                search: searchTerm,
                limit: 100, // Reasonable limit
            });
            setEntries(response.data || []);
        } catch (error) {
            console.error('Failed to fetch journal entries', error);
            toast({
                title: t('common.error'),
                description: 'Failed to fetch journal entries',
                status: 'error',
                duration: 3000,
            });
        } finally {
            setLoading(false);
        }
    }, [startDate, endDate, searchTerm, t, toast]);

    useEffect(() => {
        fetchEntries();
    }, [fetchEntries]);

    const handlePost = async (id: number) => {
        try {
            await ssotJournalService.postJournalEntry(id);
            toast({
                title: t('common.success'),
                description: t('journal.post_success'),
                status: 'success',
                duration: 3000,
            });
            fetchEntries();
        } catch (error: any) {
            toast({
                title: t('common.error'),
                description: error.message || t('journal.post_error'),
                status: 'error',
                duration: 3000,
            });
        }
    };

    const handleDelete = async (id: number) => {
        if (!window.confirm(t('journal.delete_confirm'))) return;
        try {
            await ssotJournalService.deleteJournalEntry(id);
            toast({
                title: t('common.success'),
                description: t('journal.delete_success'),
                status: 'success',
                duration: 3000,
            });
            fetchEntries();
        } catch (error: any) {
            toast({
                title: t('common.error'),
                description: error.message || t('journal.delete_error'),
                status: 'error',
                duration: 3000,
            });
        }
    };

    const handleEdit = (entry: SSOTJournalEntry) => {
        setSelectedEntry(entry);
        setShowCreateForm(true);
    };

    const handleView = (entry: SSOTJournalEntry) => {
        setViewEntry(entry);
        setIsViewModalOpen(true);
    };

    const formatCurrency = (val: number) => {
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

    if (showCreateForm) {
        // Fix for undefined reference type mismatch
        const initialWithSafeRef = selectedEntry ? {
            ...selectedEntry,
            reference: selectedEntry.reference || ''
        } : undefined;

        return (
            <Box>
                <HStack mb={4} justify="space-between">
                    <Heading size="md">{selectedEntry ? t('journal.editJournalEntry') : t('journal.createManualEntry')}</Heading>
                    <Button variant="ghost" onClick={() => { setShowCreateForm(false); setSelectedEntry(undefined); }}>{t('journal.backToList')}</Button>
                </HStack>
                <ManualEntryForm
                    initialData={initialWithSafeRef}
                    onSuccess={() => {
                        setShowCreateForm(false);
                        setSelectedEntry(undefined);
                        fetchEntries();
                    }}
                    onCancel={() => { setShowCreateForm(false); setSelectedEntry(undefined); }}
                />
            </Box>
        );
    }

    return (
        <Box>
            <JournalDetailModal
                isOpen={isViewModalOpen}
                onClose={() => setIsViewModalOpen(false)}
                entry={viewEntry}
            />

            <VStack spacing={4} align="stretch">
                <HStack justify="space-between" wrap="wrap" spacing={4}>
                    <HStack spacing={4}>
                        <Box>
                            <Text fontSize="xs" color="gray.500" mb={1}>{t('journal.filters.startDate')}</Text>
                            <Input
                                type="date"
                                value={startDate}
                                onChange={(e) => setStartDate(e.target.value)}
                                size="sm"
                                width="150px"
                            />
                        </Box>
                        <Box>
                            <Text fontSize="xs" color="gray.500" mb={1}>{t('journal.filters.endDate')}</Text>
                            <Input
                                type="date"
                                value={endDate}
                                onChange={(e) => setEndDate(e.target.value)}
                                size="sm"
                                width="150px"
                            />
                        </Box>
                        <Box alignSelf="flex-end">
                            <Button
                                leftIcon={<FiRefreshCw />}
                                size="sm"
                                onClick={fetchEntries}
                                isLoading={loading}
                            >
                                {t('journal.filters.apply')}
                            </Button>
                        </Box>
                    </HStack>

                    <Button
                        colorScheme="blue"
                        leftIcon={<FiPlus />}
                        onClick={() => { setSelectedEntry(undefined); setShowCreateForm(true); }}
                        size="sm"
                    >
                        {t('journal.createManualEntry')}
                    </Button>
                </HStack>

                <Box overflowX="auto" borderWidth="1px" borderRadius="lg" bg="white">
                    <Table size="sm" variant="simple">
                        <Thead bg="gray.50">
                            <Tr>
                                <Th>{t('journal.table.date')}</Th>
                                <Th>{t('journal.table.number')}</Th>
                                <Th>{t('journal.table.account')}</Th>
                                <Th>{t('journal.table.description')}</Th>
                                <Th>{t('journal.table.ref')}</Th>
                                <Th isNumeric>{t('journal.table.debit')}</Th>
                                <Th isNumeric>{t('journal.table.credit')}</Th>
                                <Th>{t('journal.table.status')}</Th>
                                <Th>{t('journal.table.action')}</Th>
                            </Tr>
                        </Thead>
                        <Tbody>
                            {loading ? (
                                <Tr>
                                    <Td colSpan={9} textAlign="center" py={10}>
                                        <Spinner />
                                    </Td>
                                </Tr>
                            ) : entries.length === 0 ? (
                                <Tr>
                                    <Td colSpan={9} textAlign="center" py={10}>
                                        <Text color="gray.500">{t('journal.noJournalEntries')}</Text>
                                    </Td>
                                </Tr>
                            ) : (
                                entries.map((entry) => (
                                    <React.Fragment key={entry.id}>
                                        {/* Entry Header / First Line */}
                                        {(entry.lines || []).map((line, index) => (
                                            <Tr key={`${entry.id}-${line.id}`} _hover={{ bg: "gray.50" }}>
                                                {/* Date and Number only on first line */}
                                                <Td borderBottomWidth={index === (entry.lines?.length || 0) - 1 ? "1px" : "0px"}>
                                                    {index === 0 && (
                                                        <Text fontWeight="medium">{formatDate(entry.entry_date)}</Text>
                                                    )}
                                                </Td>
                                                <Td borderBottomWidth={index === (entry.lines?.length || 0) - 1 ? "1px" : "0px"}>
                                                    {index === 0 && (
                                                        <VStack align="start" spacing={0}>
                                                            <Text fontWeight="medium" fontSize="sm">{entry.entry_number}</Text>
                                                            <Badge size="sm" colorScheme="gray" fontSize="xx-small">{entry.source_type}</Badge>
                                                        </VStack>
                                                    )}
                                                </Td>

                                                {/* Account Info */}
                                                <Td borderBottomWidth={index === (entry.lines?.length || 0) - 1 ? "1px" : "0px"}>
                                                    {line.account ? (
                                                        <VStack align="start" spacing={0}>
                                                            <Text fontSize="sm" fontWeight={line.credit_amount > 0 ? "normal" : "medium"} pl={line.credit_amount > 0 ? 4 : 0}>
                                                                {line.account.code} - {line.account.name}
                                                            </Text>
                                                        </VStack>
                                                    ) : (
                                                        <Text fontSize="sm" color="gray.400">{t('journal.unknownAccount')}</Text>
                                                    )}
                                                </Td>

                                                {/* Line Description + Entry Description (tooltip) */}
                                                <Td borderBottomWidth={index === (entry.lines?.length || 0) - 1 ? "1px" : "0px"}>
                                                    <VStack align="start" spacing={0}>
                                                        <Text fontSize="sm" noOfLines={1} title={line.description}>{line.description}</Text>
                                                        {index === 0 && entry.description !== line.description && (
                                                            <Text fontSize="xs" color="gray.500" noOfLines={1} title={entry.description}>
                                                                {entry.description}
                                                            </Text>
                                                        )}
                                                    </VStack>
                                                </Td>

                                                <Td borderBottomWidth={index === (entry.lines?.length || 0) - 1 ? "1px" : "0px"}>
                                                    {index === 0 ? (entry.reference || '-') : ''}
                                                </Td>

                                                <Td isNumeric borderBottomWidth={index === (entry.lines?.length || 0) - 1 ? "1px" : "0px"}>
                                                    {line.debit_amount > 0 ? formatCurrency(line.debit_amount) : '-'}
                                                </Td>
                                                <Td isNumeric borderBottomWidth={index === (entry.lines?.length || 0) - 1 ? "1px" : "0px"}>
                                                    {line.credit_amount > 0 ? formatCurrency(line.credit_amount) : '-'}
                                                </Td>

                                                <Td borderBottomWidth={index === (entry.lines?.length || 0) - 1 ? "1px" : "0px"}>
                                                    {index === 0 && (
                                                        <Badge
                                                            colorScheme={
                                                                entry.status === 'POSTED' ? 'green' :
                                                                    entry.status === 'DRAFT' ? 'yellow' : 'red'
                                                            }
                                                        >
                                                            {entry.status}
                                                        </Badge>
                                                    )}
                                                </Td>

                                                <Td borderBottomWidth={index === (entry.lines?.length || 0) - 1 ? "1px" : "0px"}>
                                                    {index === 0 && (
                                                        <Menu>
                                                            <MenuButton
                                                                as={IconButton}
                                                                icon={<FiMoreVertical />}
                                                                variant="ghost"
                                                                size="xs"
                                                                aria-label="Actions"
                                                            />
                                                            <MenuList>
                                                                <MenuItem icon={<FiEye />} onClick={() => handleView(entry)}>
                                                                    {t('journal.viewDetails')}
                                                                </MenuItem>
                                                                {entry.status === 'DRAFT' && (
                                                                    <>
                                                                        <MenuItem icon={<FiCheckCircle />} onClick={() => handlePost(entry.id)}>
                                                                            {t('journal.postVerify')}
                                                                        </MenuItem>
                                                                        <MenuItem icon={<FiEdit2 />} onClick={() => handleEdit(entry)}>
                                                                            {t('journal.edit')}
                                                                        </MenuItem>
                                                                        <MenuItem icon={<FiTrash2 />} onClick={() => handleDelete(entry.id)} color="red.500">
                                                                            {t('journal.delete')}
                                                                        </MenuItem>
                                                                    </>
                                                                )}
                                                            </MenuList>
                                                        </Menu>
                                                    )}
                                                </Td>
                                            </Tr>
                                        ))}
                                        {/* Spacer Row for visual separation */}
                                        <Tr>
                                            <Td colSpan={9} height="10px" bg="gray.100" p={0}></Td>
                                        </Tr>
                                    </React.Fragment>
                                ))
                            )}
                        </Tbody>
                    </Table>
                </Box>
            </VStack>
        </Box>
    );
}
