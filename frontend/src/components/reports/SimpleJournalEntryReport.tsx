'use client';

import React, { useState, useEffect, useRef, useCallback } from 'react';
import {
  Box,
  VStack,
  HStack,
  Text,
  Button,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  Card,
  CardBody,
  CardHeader,
  FormControl,
  FormLabel,
  Input,
  InputGroup,
  InputLeftElement,
  Select,
  Spinner,
  Alert,
  AlertIcon,
  Badge,
  useToast,
  useColorModeValue,
  SimpleGrid,
  Stat,
  StatLabel,
  StatNumber,
  StatHelpText,
  IconButton,
  Tooltip,
} from '@chakra-ui/react';
import {
  FiBook,
  FiSearch,
  FiRefreshCw,
  FiDownload,
  FiFileText,
  FiFilter,
} from 'react-icons/fi';
import { ssotJournalService, SSOTJournalEntry } from '@/services/ssotJournalService';
import { formatCurrency } from '@/utils/formatters';

interface SimpleJournalEntryReportProps {
  onClose?: () => void;
}

const SimpleJournalEntryReport: React.FC<SimpleJournalEntryReportProps> = ({
  onClose
}) => {
  const toast = useToast();
  const [loading, setLoading] = useState(false);
  const [entries, setEntries] = useState<SSOTJournalEntry[]>([]);
  const [filteredEntries, setFilteredEntries] = useState<SSOTJournalEntry[]>([]);
  // Filters
  const [startDate, setStartDate] = useState(() => {
    const today = new Date();
    const firstDayOfMonth = new Date(today.getFullYear(), today.getMonth(), 1);
    return firstDayOfMonth.toISOString().split('T')[0];
  });
  const [endDate, setEndDate] = useState(() => {
    return new Date().toISOString().split('T')[0];
  });
  const [statusFilter, setStatusFilter] = useState('ALL');
  const [sourceTypeFilter, setSourceTypeFilter] = useState('ALL');
  const [searchTerm, setSearchTerm] = useState('');

  // Pagination
  const [currentPage, setCurrentPage] = useState(1);
  const [itemsPerPage] = useState(10);

  // Color mode values
  const cardBg = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const headerBg = useColorModeValue('gray.50', 'gray.700');
  const textColor = useColorModeValue('gray.800', 'white');
  const mutedColor = useColorModeValue('gray.600', 'gray.400');

  // Load journal entries
  const loadJournalEntries = useCallback(async () => {
    setLoading(true);
    try {
      const response = await ssotJournalService.getJournalEntries({
        start_date: startDate,
        end_date: endDate,
        status: statusFilter !== 'ALL' ? statusFilter : undefined,
        source_type: sourceTypeFilter !== 'ALL' ? sourceTypeFilter : undefined,
        page: 1,
        limit: 100, // Get more entries for local filtering
      });

      setEntries(response.data || []);
      
      const successToastId = 'simple-journal-load-success';
      if (!toast.isActive(successToastId)) {
        toast({
          id: successToastId,
          title: 'Success',
          description: `Loaded ${response.data?.length || 0} journal entries`,
          status: 'success',
          duration: 2000,
          isClosable: true,
        });
      }
    } catch (error) {
      console.error('Error loading journal entries:', error);
      toast({
        title: 'Error',
        description: error instanceof Error ? error.message : 'Failed to load journal entries',
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
      setEntries([]);
    } finally {
      setLoading(false);
    }
  }, [startDate, endDate, statusFilter, sourceTypeFilter, toast]);

  // Filter entries based on search term
  useEffect(() => {
    if (!searchTerm) {
      setFilteredEntries(entries);
    } else {
      const filtered = entries.filter(entry =>
        entry.entry_number?.toLowerCase().includes(searchTerm.toLowerCase()) ||
        entry.description?.toLowerCase().includes(searchTerm.toLowerCase()) ||
        entry.reference?.toLowerCase().includes(searchTerm.toLowerCase())
      );
      setFilteredEntries(filtered);
    }
    setCurrentPage(1);
  }, [entries, searchTerm]);

  // Load initial data (guarded to avoid duplicate calls in React Strict Mode)
  const hasLoadedRef = useRef(false);
  useEffect(() => {
    if (hasLoadedRef.current) return;
    hasLoadedRef.current = true;
    loadJournalEntries();
  }, [loadJournalEntries]);

  // Get status color
  const getStatusColor = (status: string) => {
    switch (status?.toUpperCase()) {
      case 'POSTED':
        return 'green';
      case 'DRAFT':
        return 'yellow';
      case 'REVERSED':
        return 'red';
      default:
        return 'gray';
    }
  };

  // Get source type color
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
        return 'teal';
    }
  };

  // Calculate summary statistics
  const totalDebit = filteredEntries.reduce((sum, entry) => sum + (entry.total_debit || 0), 0);
  const totalCredit = filteredEntries.reduce((sum, entry) => sum + (entry.total_credit || 0), 0);
  const postedEntries = filteredEntries.filter(entry => entry.status === 'POSTED').length;
  const balancedEntries = filteredEntries.filter(entry => entry.is_balanced).length;

  // Pagination
  const totalPages = Math.ceil(filteredEntries.length / itemsPerPage);
  const startIndex = (currentPage - 1) * itemsPerPage;
  const endIndex = startIndex + itemsPerPage;
  const currentEntries = filteredEntries.slice(startIndex, endIndex);

  // Export to CSV
  const exportToCSV = () => {
    const headers = [
      'Entry Number',
      'Date',
      'Description',
      'Reference',
      'Source Type',
      'Status',
      'Total Debit',
      'Total Credit',
      'Balanced'
    ];

    const csvContent = [
      headers.join(','),
      ...filteredEntries.map(entry => [
        entry.entry_number || '',
        entry.entry_date || '',
        `"${entry.description || ''}"`,
        entry.reference || '',
        entry.source_type || '',
        entry.status || '',
        entry.total_debit || 0,
        entry.total_credit || 0,
        entry.is_balanced ? 'Yes' : 'No'
      ].join(','))
    ].join('\n');

    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const link = document.createElement('a');
    const url = URL.createObjectURL(blob);
    link.setAttribute('href', url);
    link.setAttribute('download', `journal-entries-${startDate}-${endDate}.csv`);
    link.style.visibility = 'hidden';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);

    toast({
      title: 'Export Successful',
      description: 'Journal entries have been exported to CSV',
      status: 'success',
      duration: 3000,
      isClosable: true,
    });
  };

  return (
    <VStack spacing={6} align="stretch" p={4}>
      {/* Header */}
      <Card>
        <CardHeader>
          <HStack justify="space-between" align="center">
            <HStack spacing={3}>
              <Box p={2} bg="blue.100" borderRadius="md">
                <FiBook size="20px" color="blue.600" />
              </Box>
              <VStack align="start" spacing={0}>
                <Text fontSize="xl" fontWeight="bold" color={textColor}>
                  Journal Entry Report
                </Text>
                <Text fontSize="sm" color={mutedColor}>
                  Simple view of all journal entries with filtering options
                </Text>
              </VStack>
            </HStack>
            {onClose && (
              <Button variant="ghost" onClick={onClose} size="sm">
                Close
              </Button>
            )}
          </HStack>
        </CardHeader>
      </Card>

      {/* Summary Statistics */}
      <SimpleGrid columns={[2, 2, 4]} spacing={4}>
        <Stat bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
          <StatLabel>Total Entries</StatLabel>
          <StatNumber>{filteredEntries.length}</StatNumber>
          <StatHelpText>{postedEntries} posted</StatHelpText>
        </Stat>
        <Stat bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
          <StatLabel>Total Debit</StatLabel>
          <StatNumber color="blue.600">{formatCurrency(totalDebit)}</StatNumber>
        </Stat>
        <Stat bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
          <StatLabel>Total Credit</StatLabel>
          <StatNumber color="green.600">{formatCurrency(totalCredit)}</StatNumber>
        </Stat>
        <Stat bg={cardBg} p={4} borderRadius="md" border="1px" borderColor={borderColor}>
          <StatLabel>Balance Status</StatLabel>
          <StatNumber>{balancedEntries}</StatNumber>
          <StatHelpText>entries balanced</StatHelpText>
        </Stat>
      </SimpleGrid>

      {/* Filters */}
      <Card>
        <CardHeader>
          <HStack spacing={4} align="center">
            <FiFilter />
            <Text fontWeight="semibold">Filters</Text>
          </HStack>
        </CardHeader>
        <CardBody pt={0}>
          <VStack spacing={4}>
            <SimpleGrid columns={[1, 2, 4]} spacing={4} w="full">
              <FormControl>
                <FormLabel fontSize="sm">Start Date</FormLabel>
                <Input
                  type="date"
                  size="sm"
                  value={startDate}
                  onChange={(e) => setStartDate(e.target.value)}
                />
              </FormControl>
              <FormControl>
                <FormLabel fontSize="sm">End Date</FormLabel>
                <Input
                  type="date"
                  size="sm"
                  value={endDate}
                  onChange={(e) => setEndDate(e.target.value)}
                />
              </FormControl>
              <FormControl>
                <FormLabel fontSize="sm">Status</FormLabel>
                <Select
                  size="sm"
                  value={statusFilter}
                  onChange={(e) => setStatusFilter(e.target.value)}
                >
                  <option value="ALL">All Status</option>
                  <option value="DRAFT">Draft</option>
                  <option value="POSTED">Posted</option>
                  <option value="REVERSED">Reversed</option>
                </Select>
              </FormControl>
              <FormControl>
                <FormLabel fontSize="sm">Source Type</FormLabel>
                <Select
                  size="sm"
                  value={sourceTypeFilter}
                  onChange={(e) => setSourceTypeFilter(e.target.value)}
                >
                  <option value="ALL">All Types</option>
                  <option value="MANUAL">Manual</option>
                  <option value="SALE">Sale</option>
                  <option value="PURCHASE">Purchase</option>
                  <option value="PAYMENT">Payment</option>
                  <option value="TRANSFER">Transfer</option>
                  <option value="CASH_BANK">Cash/Bank</option>
                  <option value="OPENING">Opening</option>
                  <option value="ADJUSTMENT">Adjustment</option>
                </Select>
              </FormControl>
            </SimpleGrid>
            
            <HStack spacing={4} w="full">
              <FormControl flex="1">
                <FormLabel fontSize="sm">Search</FormLabel>
                <InputGroup size="sm">
                  <InputLeftElement pointerEvents="none">
                    <FiSearch />
                  </InputLeftElement>
                  <Input
                    placeholder="Search by entry number, description, or reference..."
                    value={searchTerm}
                    onChange={(e) => setSearchTerm(e.target.value)}
                  />
                </InputGroup>
              </FormControl>
              <HStack spacing={2} mt={6}>
                <Button
                  size="sm"
                  colorScheme="blue"
                  leftIcon={<FiSearch />}
                  onClick={loadJournalEntries}
                  isLoading={loading}
                >
                  Apply Filters
                </Button>
                <Tooltip label="Refresh data">
                  <IconButton
                    aria-label="Refresh"
                    icon={<FiRefreshCw />}
                    size="sm"
                    variant="outline"
                    onClick={loadJournalEntries}
                    isLoading={loading}
                  />
                </Tooltip>
                <Tooltip label="Export to CSV">
                  <IconButton
                    aria-label="Export CSV"
                    icon={<FiDownload />}
                    size="sm"
                    variant="outline"
                    colorScheme="green"
                    onClick={exportToCSV}
                    isDisabled={filteredEntries.length === 0}
                  />
                </Tooltip>
              </HStack>
            </HStack>
          </VStack>
        </CardBody>
      </Card>

      {/* Journal Entries Table */}
      <Card>
        <CardHeader>
          <HStack justify="space-between">
            <Text fontWeight="semibold">
              Journal Entries ({filteredEntries.length} entries)
            </Text>
            {totalPages > 1 && (
              <HStack spacing={2}>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setCurrentPage(prev => Math.max(1, prev - 1))}
                  isDisabled={currentPage === 1}
                >
                  Previous
                </Button>
                <Text fontSize="sm" color={mutedColor}>
                  {currentPage} of {totalPages}
                </Text>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setCurrentPage(prev => Math.min(totalPages, prev + 1))}
                  isDisabled={currentPage === totalPages}
                >
                  Next
                </Button>
              </HStack>
            )}
          </HStack>
        </CardHeader>
        <CardBody pt={0}>
          {loading ? (
            <Box textAlign="center" py={8}>
              <Spinner size="lg" color="blue.500" />
              <Text mt={4} color={mutedColor}>Loading journal entries...</Text>
            </Box>
          ) : filteredEntries.length === 0 ? (
            <Alert status="info">
              <AlertIcon />
              No journal entries found for the selected criteria.
            </Alert>
          ) : (
            <Box overflowX="auto">
              <Table variant="simple" size="sm">
                <Thead bg={headerBg}>
                  <Tr>
                    <Th>Entry #</Th>
                    <Th>Date</Th>
                    <Th>Description</Th>
                    <Th>Reference</Th>
                    <Th>Source</Th>
                    <Th>Status</Th>
                    <Th isNumeric>Debit</Th>
                    <Th isNumeric>Credit</Th>
                    <Th textAlign="center">Balanced</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {currentEntries.map((entry) => (
                    <Tr key={entry.id} _hover={{ bg: headerBg }}>
                      <Td>
                        <Text fontWeight="medium" fontSize="sm">
                          {entry.entry_number}
                        </Text>
                      </Td>
                      <Td>
                        <Text fontSize="sm">
                          {new Date(entry.entry_date).toLocaleDateString('id-ID')}
                        </Text>
                      </Td>
                      <Td maxW="200px">
                        <Text fontSize="sm" noOfLines={2}>
                          {entry.description}
                        </Text>
                      </Td>
                      <Td>
                        <Text fontSize="sm" color={mutedColor}>
                          {entry.reference || '-'}
                        </Text>
                      </Td>
                      <Td>
                        <Badge 
                          colorScheme={getSourceTypeColor(entry.source_type)}
                          variant="subtle"
                          fontSize="xs"
                        >
                          {entry.source_type}
                        </Badge>
                      </Td>
                      <Td>
                        <Badge 
                          colorScheme={getStatusColor(entry.status)}
                          variant="solid"
                          fontSize="xs"
                        >
                          {entry.status}
                        </Badge>
                      </Td>
                      <Td isNumeric>
                        <Text fontSize="sm" color="blue.600" fontWeight="medium">
                          {formatCurrency(entry.total_debit)}
                        </Text>
                      </Td>
                      <Td isNumeric>
                        <Text fontSize="sm" color="green.600" fontWeight="medium">
                          {formatCurrency(entry.total_credit)}
                        </Text>
                      </Td>
                      <Td textAlign="center">
                        <Badge 
                          colorScheme={entry.is_balanced ? 'green' : 'red'} 
                          variant="subtle"
                          fontSize="xs"
                        >
                          {entry.is_balanced ? '✓' : '✗'}
                        </Badge>
                      </Td>
                    </Tr>
                  ))}
                </Tbody>
              </Table>
            </Box>
          )}
        </CardBody>
      </Card>

      {/* Action Buttons */}
      <HStack justify="center" spacing={4}>
        <Button
          colorScheme="green"
          leftIcon={<FiFileText />}
          onClick={exportToCSV}
          isDisabled={filteredEntries.length === 0}
        >
          Export CSV
        </Button>
        <Button
          variant="outline"
          leftIcon={<FiRefreshCw />}
          onClick={loadJournalEntries}
          isLoading={loading}
        >
          Refresh Data
        </Button>
      </HStack>
    </VStack>
  );
};

export default SimpleJournalEntryReport;
