import React, { useEffect, useState } from 'react';
import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalCloseButton,
  VStack,
  HStack,
  Text,
  Badge,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  TableContainer,
  Box,
  Flex,
  Spinner,
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
  Button,
  Icon,
  useDisclosure,
  useToast,
  Stat,
  StatLabel,
  StatNumber,
  StatHelpText,
  SimpleGrid,
  Card,
  CardBody,
  Divider,
  IconButton,
  Tooltip,
} from '@chakra-ui/react';
import {
  FiChevronDown,
  FiChevronRight,
  FiExternalLink,
  FiRefreshCw,
  FiInfo,
  FiTrendingUp,
  FiDollarSign,
  FiCalendar,
  FiHash,
  FiFileText,
} from 'react-icons/fi';
import purchaseJournalService, {
  PurchaseJournalData,
  JournalEntryWithDetails,
} from '@/services/purchaseJournalService';

interface PurchaseJournalEntriesModalProps {
  isOpen: boolean;
  onClose: () => void;
  purchase: {
    id: number;
    code: string;
    total_amount: number;
  } | null;
}

interface JournalEntryItemProps {
  entry: JournalEntryWithDetails;
  onViewDetails: (entry: JournalEntryWithDetails) => void;
}

const JournalEntryItem: React.FC<JournalEntryItemProps> = ({ entry, onViewDetails }) => {
  const { isOpen, onToggle } = useDisclosure();
  const [details, setDetails] = useState<JournalEntryWithDetails | null>(null);
  const [loadingDetails, setLoadingDetails] = useState(false);

  const formatCurrency = (amount: number) => {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0
    }).format(amount);
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString('id-ID', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  };

  const getStatusColor = (status: string) => {
    switch (status?.toUpperCase()) {
      case 'POSTED': return 'green';
      case 'DRAFT': return 'yellow';
      case 'REVERSED': return 'red';
      default: return 'gray';
    }
  };

  const handleToggleDetails = async () => {
    if (!isOpen && !details) {
      setLoadingDetails(true);
      try {
        const entryDetails = await purchaseJournalService.getJournalEntryDetails(entry.id);
        setDetails(entryDetails);
      } catch (error) {
        console.error('Error loading journal entry details:', error);
      } finally {
        setLoadingDetails(false);
      }
    }
    onToggle();
  };

  return (
    <Card mb={4} shadow="sm" borderWidth="1px">
      <CardBody>
        {/* Header */}
        <Flex justify="space-between" align="center" mb={3}>
          <VStack align="start" spacing={1}>
            <HStack>
              <Text fontWeight="semibold" fontSize="md">
                {entry.entry_number || `JE-${entry.id}`}
              </Text>
              <Badge colorScheme={getStatusColor(entry.status)} variant="subtle" fontSize="xs">
                {entry.status || 'UNKNOWN'}
              </Badge>
            </HStack>
            <Text fontSize="sm" color="gray.600">
              {entry.description}
            </Text>
          </VStack>
          
          <VStack align="end" spacing={1}>
            <HStack>
              <Text fontWeight="bold" color="blue.600">
                {formatCurrency(entry.total_debit)}
              </Text>
              <IconButton
                aria-label="Toggle details"
                icon={isOpen ? <FiChevronDown /> : <FiChevronRight />}
                size="sm"
                variant="ghost"
                onClick={handleToggleDetails}
              />
            </HStack>
            <Text fontSize="xs" color="gray.500">
              {formatDate(entry.entry_date)}
            </Text>
          </VStack>
        </Flex>

        {/* Expandable Details */}
        {isOpen ? (
          <Box pt={3} borderTop="1px solid" borderColor="gray.200">
            {loadingDetails ? (
              <Flex justify="center" py={4}>
                <Spinner size="sm" />
                <Text ml={2} fontSize="sm" color="gray.600">Loading details...</Text>
              </Flex>
            ) : details?.lines ? (
              <TableContainer>
                <Table size="sm">
                  <Thead>
                    <Tr>
                      <Th fontSize="xs">Account</Th>
                      <Th fontSize="xs">Description</Th>
                      <Th fontSize="xs" textAlign="right">Debit</Th>
                      <Th fontSize="xs" textAlign="right">Credit</Th>
                    </Tr>
                  </Thead>
                  <Tbody>
                    {details.lines.map((line, index) => (
                      <Tr key={index}>
                        <Td fontSize="sm">
                          <VStack align="start" spacing={0}>
                            <Text fontWeight="medium">
                              {line.account?.code || `ACC-${line.account_id}`}
                            </Text>
                            <Text fontSize="xs" color="gray.600">
                              {line.account?.name || 'Unknown Account'}
                            </Text>
                          </VStack>
                        </Td>
                        <Td fontSize="sm" maxW="200px">
                          <Text noOfLines={2}>{line.description}</Text>
                        </Td>
                        <Td fontSize="sm" textAlign="right">
                          {line.debit_amount > 0 ? formatCurrency(line.debit_amount) : '-'}
                        </Td>
                        <Td fontSize="sm" textAlign="right">
                          {line.credit_amount > 0 ? formatCurrency(line.credit_amount) : '-'}
                        </Td>
                      </Tr>
                    ))}
                  </Tbody>
                </Table>
              </TableContainer>
            ) : (
              <Text fontSize="sm" color="gray.600" textAlign="center" py={4}>
                No detailed information available
              </Text>
            )}
            
            {/* Additional Entry Info */}
            {details && (
              <SimpleGrid columns={2} spacing={4} mt={4} pt={3} borderTop="1px solid" borderColor="gray.100">
                <Stat size="sm">
                  <StatLabel fontSize="xs">Source Type</StatLabel>
                  <StatNumber fontSize="sm">{entry.source_type || 'Unknown'}</StatNumber>
                </Stat>
                <Stat size="sm">
                  <StatLabel fontSize="xs">Auto Generated</StatLabel>
                  <StatNumber fontSize="sm">{entry.is_auto_generated ? 'Yes' : 'No'}</StatNumber>
                </Stat>
              </SimpleGrid>
            )}
          </Box>
        ) : null}
      </CardBody>
    </Card>
  );
};

const PurchaseJournalEntriesModal: React.FC<PurchaseJournalEntriesModalProps> = ({
  isOpen,
  onClose,
  purchase,
}) => {
  const purchaseId = purchase?.id || 0;
  const purchaseCode = purchase?.code || 'Unknown';
  const purchaseAmount = purchase?.total_amount || 0;
  const [journalData, setJournalData] = useState<PurchaseJournalData | null>(null);
  const [loading, setLoading] = useState(true);
  const [backfilling, setBackfilling] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const toast = useToast();

  const formatCurrency = (amount: number) => {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0
    }).format(amount);
  };

  const fetchJournalEntries = async () => {
    if (!purchaseId || purchaseId === 0) return;
    
    setLoading(true);
    setError(null);
    
    try {
      const data = await purchaseJournalService.getPurchaseJournalEntries(purchaseId);
      setJournalData(data);
    } catch (err) {
      console.error('Error fetching journal entries:', err);
      setError(err instanceof Error ? err.message : 'Failed to fetch journal entries');
      toast({
        title: 'Error',
        description: 'Failed to fetch journal entries',
        status: 'error',
        duration: 3000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isOpen && purchaseId && purchaseId > 0) {
      fetchJournalEntries();
    }
  }, [isOpen, purchaseId]);

  const handleRefresh = () => {
    fetchJournalEntries();
  };

  const handleBackfill = async () => {
    if (!purchaseId || purchaseId === 0) return;

    setBackfilling(true);
    setError(null);
    try {
      const data = await purchaseJournalService.backfillPurchaseJournalEntries(purchaseId);
      setJournalData(data);
      toast({
        title: 'Journal created',
        description: 'Purchase journal entries have been backfilled.',
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to backfill journal entries';
      setError(message);
      toast({
        title: 'Backfill failed',
        description: message,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setBackfilling(false);
    }
  };

  const handleViewDetails = (entry: JournalEntryWithDetails) => {
    // Navigate to journal entry details or open detailed modal
    console.log('View details for journal entry:', entry.id);
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} size="4xl">
      <ModalOverlay />
      <ModalContent maxH="90vh">
        <ModalHeader>
          <VStack align="start" spacing={2}>
            <HStack>
              <Icon as={FiFileText} color="blue.500" />
              <Text>Journal Entries</Text>
              <Badge colorScheme="blue" variant="subtle">
                {purchaseCode}
              </Badge>
            </HStack>
            <Text fontSize="sm" color="gray.600" fontWeight="normal">
              Accounting journal entries for purchase order
            </Text>
          </VStack>
        </ModalHeader>
        <ModalCloseButton />
        
        <ModalBody pb={6}>
          {/* Summary Cards */}
          <SimpleGrid columns={3} spacing={4} mb={6}>
            <Card>
              <CardBody>
                <Stat size="sm">
                  <StatLabel>
                    <HStack>
                      <Icon as={FiHash} />
                      <Text>Total Entries</Text>
                    </HStack>
                  </StatLabel>
                  <StatNumber color="blue.600">
                    {journalData?.count || 0}
                  </StatNumber>
                </Stat>
              </CardBody>
            </Card>
            
            <Card>
              <CardBody>
                <Stat size="sm">
                  <StatLabel>
                    <HStack>
                      <Icon as={FiDollarSign} />
                      <Text>Purchase Amount</Text>
                    </HStack>
                  </StatLabel>
                  <StatNumber color="green.600">
                    {formatCurrency(purchaseAmount)}
                  </StatNumber>
                </Stat>
              </CardBody>
            </Card>
            
            <Card>
              <CardBody>
                <Stat size="sm">
                  <StatLabel>
                    <HStack>
                      <Icon as={FiTrendingUp} />
                      <Text>Journal Total</Text>
                    </HStack>
                  </StatLabel>
                  <StatNumber color="purple.600">
                    {journalData?.journal_entries 
                      ? formatCurrency(journalData.journal_entries.reduce((sum, entry) => sum + entry.total_debit, 0))
                      : formatCurrency(0)
                    }
                  </StatNumber>
                </Stat>
              </CardBody>
            </Card>
          </SimpleGrid>

          <Flex justify="space-between" align="center" mb={4}>
            <Text fontWeight="semibold" fontSize="lg">
              Journal Entries
            </Text>
            <HStack>
              {journalData?.count === 0 && !loading ? (
                <Button
                  leftIcon={<FiFileText />}
                  size="sm"
                  colorScheme="blue"
                  variant="solid"
                  onClick={handleBackfill}
                  isLoading={backfilling}
                >
                  Backfill Journal
                </Button>
              ) : null}
              <Button
                leftIcon={<FiRefreshCw />}
                size="sm"
                variant="outline"
                onClick={handleRefresh}
                isLoading={loading}
              >
                Refresh
              </Button>
            </HStack>
          </Flex>

          {loading ? (
            <Flex justify="center" align="center" py={12}>
              <VStack>
                <Spinner size="lg" color="blue.500" />
                <Text color="gray.600">Loading journal entries...</Text>
              </VStack>
            </Flex>
          ) : error ? (
            <Alert status="error" borderRadius="md">
              <AlertIcon />
              <Box>
                <AlertTitle>Error loading journal entries</AlertTitle>
                <AlertDescription>{error}</AlertDescription>
              </Box>
            </Alert>
          ) : journalData?.count === 0 ? (
            <Alert status="info" borderRadius="md">
              <AlertIcon />
              <Box flex="1">
                <AlertTitle>No Journal Entries Found</AlertTitle>
                <AlertDescription>
                  This purchase doesn't have any associated journal entries yet. 
                  Journal entries are typically created when the purchase is approved.
                </AlertDescription>
              </Box>
              <Button
                leftIcon={<FiFileText />}
                size="sm"
                colorScheme="blue"
                onClick={handleBackfill}
                isLoading={backfilling}
              >
                Backfill
              </Button>
            </Alert>
          ) : (
            <VStack spacing={0} align="stretch">
              {journalData?.journal_entries.map((entry) => (
                <JournalEntryItem
                  key={entry.id}
                  entry={entry}
                  onViewDetails={handleViewDetails}
                />
              ))}
            </VStack>
          )}

          {(journalData?.count ?? 0) > 0 && (
            <Box mt={6} pt={4} borderTop="1px solid" borderColor="gray.200">
              <Text fontSize="sm" color="gray.600">
                <Icon as={FiInfo} mr={2} />
                Journal entries are automatically created when purchases are approved and represent 
                the accounting impact of the transaction. Each entry shows the debits and credits 
                that affect your chart of accounts.
              </Text>
            </Box>
          )}
        </ModalBody>
      </ModalContent>
    </Modal>
  );
};

export default PurchaseJournalEntriesModal;
