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
    Spinner,
    useToast,
    VStack,
    Heading,
    Menu,
    MenuButton,
    MenuList,
    MenuItem,
    IconButton,
    useColorModeValue,
} from '@chakra-ui/react';
import { useTranslation } from '@/hooks/useTranslation';
import { ssotJournalService, SSOTJournalEntry } from '@/services/ssotJournalService';
import { FiRefreshCw, FiPlus, FiMoreVertical, FiEdit2, FiTrash2, FiCheckCircle, FiEye } from 'react-icons/fi';
import ManualEntryForm from './ManualEntryForm';
import JournalDetailModal from './JournalDetailModal';
import ExportButton from '@/components/common/ExportButton';
import { exportGeneralJournalToPDF, exportGeneralJournalToCSV } from '@/utils/journalExport';

export default function GeneralJournalView() {
    const { t } = useTranslation();
    const toast = useToast();
    const textColor = useColorModeValue('gray.800', 'gray.100');
    const mutedTextColor = useColorModeValue('gray.500', 'gray.400');
    const subtleTextColor = useColorModeValue('gray.400', 'gray.500');
    const inputBg = useColorModeValue('white', 'whiteAlpha.50');
    const inputBorderColor = useColorModeValue('gray.300', 'whiteAlpha.300');
    const inputHoverBorderColor = useColorModeValue('gray.400', 'whiteAlpha.400');
    const inputFocusBorderColor = useColorModeValue('blue.500', 'blue.300');
    const tableBg = useColorModeValue('white', 'gray.800');
    const tableHeaderBg = useColorModeValue('gray.50', 'whiteAlpha.100');
    const tableHoverBg = useColorModeValue('gray.50', 'whiteAlpha.100');
    const tableSeparatorBg = useColorModeValue('gray.100', 'whiteAlpha.50');
    const borderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
    const menuBg = useColorModeValue('white', 'gray.800');
    const menuBorderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
    const menuHoverBg = useColorModeValue('gray.50', 'whiteAlpha.100');
    const fieldStyles = {
        bg: inputBg,
        color: textColor,
        borderColor: inputBorderColor,
        _hover: { borderColor: inputHoverBorderColor },
        _focusVisible: {
            borderColor: inputFocusBorderColor,
            boxShadow: `0 0 0 1px ${inputFocusBorderColor}`,
        },
        _placeholder: { color: mutedTextColor },
    };

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
    const [exporting, setExporting] = useState(false);

    const handleExportPDF = () => {
        try {
            setExporting(true);
            exportGeneralJournalToPDF(entries, startDate, endDate);
            toast({ title: t('common.success'), description: 'PDF berhasil diunduh', status: 'success', duration: 3000 });
        } catch {
            toast({ title: t('common.error'), description: 'Gagal mengexport PDF', status: 'error', duration: 3000 });
        } finally {
            setExporting(false);
        }
    };

    const handleExportCSV = () => {
        try {
            setExporting(true);
            exportGeneralJournalToCSV(entries, startDate, endDate);
            toast({ title: t('common.success'), description: 'CSV berhasil diunduh', status: 'success', duration: 3000 });
        } catch {
            toast({ title: t('common.error'), description: 'Gagal mengexport CSV', status: 'error', duration: 3000 });
        } finally {
            setExporting(false);
        }
    };

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
        } catch (error: unknown) {
            const message = error instanceof Error ? error.message : t('journal.post_error');
            toast({
                title: t('common.error'),
                description: message,
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
        } catch (error: unknown) {
            const message = error instanceof Error ? error.message : t('journal.delete_error');
            toast({
                title: t('common.error'),
                description: message,
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

    const getSourceTypeColor = (sourceType: string) => {
        switch (sourceType?.toUpperCase()) {
            case 'SALE':
                return 'blue';
            case 'PURCHASE':
                return 'orange';
            case 'PAYMENT':
                return 'purple';
            case 'TRANSFER':
                return 'teal';
            case 'CASH_BANK':
                return 'cyan';
            case 'OPENING':
                return 'green';
            case 'ADJUSTMENT':
                return 'pink';
            case 'MANUAL':
                return 'gray';
            default:
                return 'gray';
        }
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
                    <Heading size="md" color={textColor}>{selectedEntry ? t('journal.editJournalEntry') : t('journal.createManualEntry')}</Heading>
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
                            <Text fontSize="xs" color={mutedTextColor} mb={1}>{t('journal.filters.startDate')}</Text>
                            <Input
                                type="date"
                                value={startDate}
                                onChange={(e) => setStartDate(e.target.value)}
                                size="sm"
                                width="150px"
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
                                width="150px"
                                {...fieldStyles}
                            />
                        </Box>
                        <Box>
                            <Text fontSize="xs" color={mutedTextColor} mb={1}>{t('common.search')}</Text>
                            <Input
                                value={searchTerm}
                                onChange={(e) => setSearchTerm(e.target.value)}
                                size="sm"
                                width="220px"
                                placeholder={t('journal.table.ref')}
                                {...fieldStyles}
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

                    <HStack spacing={2}>
                        <ExportButton
                            onExportPDF={handleExportPDF}
                            onExportCSV={handleExportCSV}
                            isLoading={exporting}
                            disabled={entries.length === 0}
                            showExcel={false}
                            showPDF={true}
                            showCSV={true}
                            size="sm"
                        />
                        <Button
                            colorScheme="blue"
                            leftIcon={<FiPlus />}
                            onClick={() => { setSelectedEntry(undefined); setShowCreateForm(true); }}
                            size="sm"
                        >
                            {t('journal.createManualEntry')}
                        </Button>
                    </HStack>
                </HStack>

                <Box overflowX="auto" borderWidth="1px" borderRadius="lg" bg={tableBg} borderColor={borderColor}>
                    <Table size="sm" variant="simple" color={textColor}>
                        <Thead bg={tableHeaderBg}>
                            <Tr>
                                <Th color={mutedTextColor}>{t('journal.table.date')}</Th>
                                <Th color={mutedTextColor}>{t('journal.table.number')}</Th>
                                <Th color={mutedTextColor}>{t('journal.table.account')}</Th>
                                <Th color={mutedTextColor}>{t('journal.table.description')}</Th>
                                <Th color={mutedTextColor}>{t('journal.table.ref')}</Th>
                                <Th isNumeric color={mutedTextColor}>{t('journal.table.debit')}</Th>
                                <Th isNumeric color={mutedTextColor}>{t('journal.table.credit')}</Th>
                                <Th color={mutedTextColor}>{t('journal.table.status')}</Th>
                                <Th color={mutedTextColor}>{t('journal.table.action')}</Th>
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
                                        <Text color={mutedTextColor}>{t('journal.noJournalEntries')}</Text>
                                    </Td>
                                </Tr>
                            ) : (
                                entries.map((entry) => (
                                    <React.Fragment key={entry.id}>
                                        {/* Entry Header / First Line */}
                                        {(entry.lines || []).map((line, index) => (
                                            <Tr key={`${entry.id}-${line.id}`} _hover={{ bg: tableHoverBg }}>
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
                                                            <Badge size="sm" colorScheme={getSourceTypeColor(entry.source_type)} fontSize="xx-small">{entry.source_type}</Badge>
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
                                                        <Text fontSize="sm" color={subtleTextColor}>{t('journal.unknownAccount')}</Text>
                                                    )}
                                                </Td>

                                                {/* Line Description + Entry Description (tooltip) */}
                                                <Td borderBottomWidth={index === (entry.lines?.length || 0) - 1 ? "1px" : "0px"}>
                                                    <VStack align="start" spacing={0}>
                                                        <Text fontSize="sm" noOfLines={1} title={line.description}>{line.description}</Text>
                                                        {index === 0 && entry.description !== line.description && (
                                                            <Text fontSize="xs" color={mutedTextColor} noOfLines={1} title={entry.description}>
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
                                                            <MenuList bg={menuBg} borderColor={menuBorderColor}>
                                                                <MenuItem icon={<FiEye />} onClick={() => handleView(entry)} _hover={{ bg: menuHoverBg }}>
                                                                    {t('journal.viewDetails')}
                                                                </MenuItem>
                                                                {entry.status === 'DRAFT' && (
                                                                    <>
                                                                        <MenuItem icon={<FiCheckCircle />} onClick={() => handlePost(entry.id)} _hover={{ bg: menuHoverBg }}>
                                                                            {t('journal.postVerify')}
                                                                        </MenuItem>
                                                                        <MenuItem icon={<FiEdit2 />} onClick={() => handleEdit(entry)} _hover={{ bg: menuHoverBg }}>
                                                                            {t('journal.edit')}
                                                                        </MenuItem>
                                                                        <MenuItem icon={<FiTrash2 />} onClick={() => handleDelete(entry.id)} color="red.500" _hover={{ bg: menuHoverBg }}>
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
                                            <Td colSpan={9} height="10px" bg={tableSeparatorBg} p={0}></Td>
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
