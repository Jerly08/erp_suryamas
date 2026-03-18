import React, { useEffect, useState } from 'react';
import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalFooter,
  ModalBody,
  ModalCloseButton,
  Button,
  VStack,
  HStack,
  Text,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  Badge,
  Box,
  Spinner,
  Alert,
  AlertIcon,
  Input,
  FormControl,
  FormLabel,
  useColorModeValue,
  Divider,
  Icon,
  Tooltip,
  IconButton,
  Select,
} from '@chakra-ui/react';
import { FiCalendar, FiRefreshCw, FiFilter } from 'react-icons/fi';
import closingHistoryService, { ClosingHistoryItem } from '../../services/closingHistoryService';
import { formatDateOnly, formatPeriodRange, parseDateOnly } from '../../services/periodClosingService';

interface ClosingHistoryModalProps {
  isOpen: boolean;
  onClose: () => void;
  reportType?: string;
  currentPeriod?: { start: string; end: string };
}

const ClosingHistoryModal: React.FC<ClosingHistoryModalProps> = ({
  isOpen,
  onClose,
  reportType,
  currentPeriod,
}) => {
  const [history, setHistory] = useState<ClosingHistoryItem[]>([]);
  const [filteredHistory, setFilteredHistory] = useState<ClosingHistoryItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [filterStartDate, setFilterStartDate] = useState('');
  const [filterEndDate, setFilterEndDate] = useState('');
  const [filterYear, setFilterYear] = useState<string>('all');

  const bgColor = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const headerBg = useColorModeValue('gray.50', 'gray.700');
  const hoverBg = useColorModeValue('gray.50', 'gray.700');
  const inputBg = useColorModeValue('white', 'whiteAlpha.100');
  const inputBorderColor = useColorModeValue('gray.300', 'whiteAlpha.300');
  const inputHoverBorderColor = useColorModeValue('blue.400', 'blue.300');
  const inputFocusBorderColor = useColorModeValue('blue.500', 'blue.200');
  const mutedTextColor = useColorModeValue('gray.600', 'gray.300');
  const fieldStyles = {
    bg: inputBg,
    borderColor: inputBorderColor,
    _hover: { borderColor: inputHoverBorderColor },
    _focus: {
      borderColor: inputFocusBorderColor,
      boxShadow: `0 0 0 1px ${inputFocusBorderColor}`,
    },
  };

  useEffect(() => {
    if (isOpen) {
      void fetchHistory();
    }
  }, [isOpen]);

  useEffect(() => {
    let filtered = [...history];

    if (filterYear !== 'all') {
      filtered = filtered.filter((item) => {
        const year = getEntryYear(item.entry_date);
        return year === filterYear;
      });
    }

    if (filterStartDate || filterEndDate) {
      filtered = closingHistoryService.filterHistoryByDateRange(
        filtered,
        filterStartDate,
        filterEndDate,
      );
    }

    setFilteredHistory(filtered);
  }, [history, filterStartDate, filterEndDate, filterYear]);

  const fetchHistory = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await closingHistoryService.getAllClosingHistory();
      setHistory(data);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to load closing history');
    } finally {
      setLoading(false);
    }
  };

  const getUniqueYears = (): string[] => {
    const years = new Set<string>();
    history.forEach((item) => {
      const year = getEntryYear(item.entry_date);
      if (year) {
        years.add(year);
      }
    });
    return Array.from(years).sort((a, b) => b.localeCompare(a));
  };

  const clearFilters = () => {
    setFilterStartDate('');
    setFilterEndDate('');
    setFilterYear('all');
  };

  const getEntryYear = (value: string) => {
    const dateOnly = parseDateOnly(value);
    if (dateOnly) {
      return dateOnly.getFullYear().toString();
    }

    const parsed = new Date(value);
    if (Number.isNaN(parsed.getTime())) {
      return '';
    }

    return parsed.getFullYear().toString();
  };

  const formatCurrency = (amount: number) =>
    new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(amount);

  const getStatusColor = (status?: string) => {
    if (!status) return 'green';
    const normalized = status.toLowerCase();
    if (normalized === 'reopened') return 'orange';
    return 'green';
  };

  const getStatusLabel = (status?: string) => {
    if (!status) return 'Closed';
    return status.toLowerCase() === 'reopened' ? 'Reopened' : 'Closed';
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} size="5xl">
      <ModalOverlay />
      <ModalContent bg={bgColor} borderWidth="1px" borderColor={borderColor}>
        <ModalHeader>
          <HStack spacing={3}>
            <Icon as={FiCalendar} />
            <Text>Closing History</Text>
            {reportType ? (
              <Badge colorScheme="blue" ml={2}>
                {reportType}
              </Badge>
            ) : null}
          </HStack>
        </ModalHeader>
        <ModalCloseButton />

        <ModalBody>
          <VStack spacing={4} align="stretch">
            <Box p={4} borderWidth={1} borderColor={borderColor} borderRadius="md">
              <HStack spacing={4} mb={3}>
                <Icon as={FiFilter} />
                <Text fontWeight="bold">Filters</Text>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={clearFilters}
                  leftIcon={<FiRefreshCw />}
                >
                  Clear
                </Button>
              </HStack>

              <HStack spacing={4}>
                <FormControl flex="1">
                  <FormLabel fontSize="sm">Year</FormLabel>
                  <Select
                    value={filterYear}
                    onChange={(e) => setFilterYear(e.target.value)}
                    size="sm"
                    {...fieldStyles}
                  >
                    <option value="all">All Years</option>
                    {getUniqueYears().map((year) => (
                      <option key={year} value={year}>
                        {year}
                      </option>
                    ))}
                  </Select>
                </FormControl>

                <FormControl flex="1">
                  <FormLabel fontSize="sm">Start Date</FormLabel>
                  <Input
                    type="date"
                    value={filterStartDate}
                    onChange={(e) => setFilterStartDate(e.target.value)}
                    size="sm"
                    {...fieldStyles}
                  />
                </FormControl>

                <FormControl flex="1">
                  <FormLabel fontSize="sm">End Date</FormLabel>
                  <Input
                    type="date"
                    value={filterEndDate}
                    onChange={(e) => setFilterEndDate(e.target.value)}
                    size="sm"
                    {...fieldStyles}
                  />
                </FormControl>
              </HStack>
            </Box>

            <Divider />

            {currentPeriod ? (
              <Alert status="info" borderRadius="md">
                <AlertIcon />
                <Box>
                  <Text fontWeight="bold">Current Report Period</Text>
                  <Text fontSize="sm">
                    {formatPeriodRange(currentPeriod.start, currentPeriod.end)}
                  </Text>
                </Box>
              </Alert>
            ) : null}

            <Alert status="info" borderRadius="md" variant="left-accent">
              <AlertIcon />
              <Box>
                <Text fontSize="sm">
                  Riwayat diambil dari period closing canonical. Jika sebuah periode pernah dibuka kembali,
                  status dan jejak auditnya tetap ditampilkan agar angka archive tidak menyesatkan.
                </Text>
              </Box>
            </Alert>

            {loading ? (
              <Box textAlign="center" py={8}>
                <Spinner size="lg" />
                <Text mt={3}>Loading closing history...</Text>
              </Box>
            ) : error ? (
              <Alert status="error" borderRadius="md">
                <AlertIcon />
                <Text>{error}</Text>
                <Button size="sm" ml="auto" onClick={() => void fetchHistory()}>
                  Retry
                </Button>
              </Alert>
            ) : filteredHistory.length === 0 ? (
              <Alert status="warning" borderRadius="md">
                <AlertIcon />
                <Text>
                  {history.length === 0
                    ? 'No closing history found.'
                    : 'No results match your filters.'}
                </Text>
              </Alert>
            ) : (
              <Box overflowX="auto">
                <Table variant="simple">
                  <Thead bg={headerBg}>
                    <Tr>
                      <Th>Closing Date</Th>
                      <Th>Period</Th>
                      <Th>Source</Th>
                      <Th>Description</Th>
                      <Th isNumeric>Net Income</Th>
                      <Th>Status</Th>
                    </Tr>
                  </Thead>
                  <Tbody>
                    {filteredHistory.map((item) => {
                      const formatted = closingHistoryService.formatClosingHistory(item);
                      const description = item.history_events?.[0]?.description || formatted.description;

                      return (
                        <Tr key={item.id} _hover={{ bg: hoverBg }}>
                          <Td>{formatDateOnly(item.entry_date)}</Td>
                          <Td>
                            <Text fontSize="sm" fontWeight="medium">
                              {formatted.period}
                            </Text>
                          </Td>
                          <Td>
                            <VStack spacing={1} align="start">
                              <Badge colorScheme={item.source === 'period' ? 'blue' : 'purple'}>
                                {formatted.sourceLabel}
                              </Badge>
                              <Text fontSize="xs" color={mutedTextColor} fontFamily="mono">
                                {item.code}
                              </Text>
                            </VStack>
                          </Td>
                          <Td>
                            <Tooltip label={description} placement="top">
                              <Text fontSize="sm" noOfLines={2} maxW="280px">
                                {description}
                              </Text>
                            </Tooltip>
                          </Td>
                          <Td isNumeric>
                            <Text fontWeight="medium">
                              {formatCurrency(item.net_income || 0)}
                            </Text>
                          </Td>
                          <Td>
                            <Badge colorScheme={getStatusColor(item.status)}>
                              {getStatusLabel(item.status)}
                            </Badge>
                          </Td>
                        </Tr>
                      );
                    })}
                  </Tbody>
                </Table>
              </Box>
            )}

            {filteredHistory.length > 0 ? (
              <Box p={3} bg={headerBg} borderRadius="md">
                <HStack justify="space-between">
                  <Text fontSize="sm" color={mutedTextColor}>
                    Showing {filteredHistory.length} of {history.length} closing periods
                  </Text>
                  <HStack>
                    <Text fontSize="sm" fontWeight="bold">
                      Latest Closing:
                    </Text>
                    <Text fontSize="sm">
                      {formatDateOnly(filteredHistory[0].entry_date)}
                    </Text>
                  </HStack>
                </HStack>
              </Box>
            ) : null}
          </VStack>
        </ModalBody>

        <ModalFooter>
          <HStack spacing={3}>
            <IconButton
              aria-label="Refresh"
              icon={<FiRefreshCw />}
              onClick={() => void fetchHistory()}
              variant="ghost"
              size="sm"
            />
            <Button onClick={onClose}>Close</Button>
          </HStack>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
};

export default ClosingHistoryModal;
