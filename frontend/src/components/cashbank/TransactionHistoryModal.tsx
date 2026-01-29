'use client';

import React, { useState, useEffect } from 'react';
import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalCloseButton,
  Box,
  Text,
  VStack,
  HStack,
  Flex,
  Badge,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  TableContainer,
  Button,
  FormControl,
  FormLabel,
  Input,
  Select,
  Spinner,
  Alert,
  AlertIcon,
  useToast,
  IconButton,
  Tooltip,
  SimpleGrid,
  Stat,
  StatLabel,
  StatNumber,
  StatHelpText,
  Card,
  CardBody,
} from '@chakra-ui/react';
import { FiRefreshCw, FiDownload, FiCalendar, FiFilter } from 'react-icons/fi';
import { useTranslation } from '@/hooks/useTranslation';
import cashbankService, { CashBank, TransactionResult, CashBankTransaction, TransactionFilter } from '@/services/cashbankService';

interface TransactionHistoryModalProps {
  isOpen: boolean;
  onClose: () => void;
  account: CashBank | null;
}

const TransactionHistoryModal: React.FC<TransactionHistoryModalProps> = ({
  isOpen,
  onClose,
  account
}) => {
  const [transactions, setTransactions] = useState<CashBankTransaction[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [currentPage, setCurrentPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [total, setTotal] = useState(0);
  const toast = useToast();
  const { t } = useTranslation();

  // Filter states
  const [filters, setFilters] = useState<TransactionFilter>({
    page: 1,
    limit: 20,
    start_date: '',
    end_date: ''
  });

  const fetchTransactions = async () => {
    if (!account) return;

    try {
      setLoading(true);
      setError(null);

      const result: TransactionResult = await cashbankService.getTransactionHistory(account.id, filters);

      // Ensure we always have an array for transactions
      const transactionData = Array.isArray(result.data) ? result.data : [];

      setTransactions(transactionData);
      setCurrentPage(result.page || 1);
      setTotalPages(result.total_pages || 1);
      setTotal(result.total || 0);
    } catch (err: any) {
      console.error('Error fetching transaction history:', err);
      const errorMessage = err.response?.data?.details || err.message || 'Failed to fetch transaction history';
      setError(errorMessage);

      // Reset transactions to empty array on error to prevent runtime errors
      setTransactions([]);
      setTotal(0);
      setCurrentPage(1);
      setTotalPages(1);

      toast({
        title: 'Error',
        description: errorMessage,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isOpen && account) {
      fetchTransactions();
    }
  }, [isOpen, account, filters]);

  const handleFilterChange = (field: keyof TransactionFilter, value: any) => {
    setFilters(prev => ({
      ...prev,
      [field]: value,
      page: 1 // Reset to first page when filtering
    }));
  };

  const handlePageChange = (newPage: number) => {
    setFilters(prev => ({
      ...prev,
      page: newPage
    }));
  };

  const clearFilters = () => {
    setFilters({
      page: 1,
      limit: 20,
      start_date: '',
      end_date: ''
    });
  };

  const formatTransactionType = (type: string) => {
    switch (type) {
      case 'DEPOSIT': return { label: 'Deposit', color: 'green' };
      case 'WITHDRAWAL': return { label: 'Withdrawal', color: 'red' };
      case 'TRANSFER': return { label: 'Transfer', color: 'blue' };
      case 'OPENING_BALANCE': return { label: 'Opening Balance', color: 'purple' };
      case 'ADJUSTMENT': return { label: 'Adjustment', color: 'orange' };
      default: return { label: type, color: 'gray' };
    }
  };

  if (!account) return null;

  return (
    <Modal isOpen={isOpen} onClose={onClose} size="6xl">
      <ModalOverlay />
      <ModalContent>
        <ModalHeader>
          <Flex alignItems="center" gap={3}>
            <Text fontSize="lg">{account.type === 'CASH' ? '💵' : '🏦'}</Text>
            <Box>
              <Text fontSize="lg" fontWeight="bold">
                {t('common.transactionHistory') || 'Transaction History'}
              </Text>
              <Text fontSize="sm" color="gray.500" fontFamily="mono">
                {account.name} ({account.code})
              </Text>
            </Box>
          </Flex>
        </ModalHeader>
        <ModalCloseButton />

        <ModalBody pb={6}>
          <VStack spacing={6} align="stretch">
            {/* Account Summary */}
            <Card>
              <CardBody>
                <SimpleGrid columns={4} spacing={4}>
                  <Stat size="sm">
                    <StatLabel>{t('accounts.accountType') || 'Account Type'}</StatLabel>
                    <StatNumber fontSize="md">
                      <Badge colorScheme={account.type === 'CASH' ? 'green' : 'blue'}>
                        {account.type}
                      </Badge>
                    </StatNumber>
                  </Stat>
                  <Stat size="sm">
                    <StatLabel>{t('settings.currency') || 'Currency'}</StatLabel>
                    <StatNumber fontSize="md">{account.currency}</StatNumber>
                  </Stat>
                  <Stat size="sm">
                    <StatLabel>{t('products.currentStock').replace('Stock', 'Balance') || 'Current Balance'}</StatLabel>
                    <StatNumber
                      fontSize="md"
                      color={account.balance < 0 ? 'red.500' : 'green.600'}
                      fontFamily="mono"
                    >
                      {account.currency} {Math.abs(account.balance).toLocaleString('id-ID', { maximumFractionDigits: 0 })}
                      {account.balance < 0 && ' (Dr)'}
                    </StatNumber>
                  </Stat>
                  <Stat size="sm">
                    <StatLabel>{t('dashboard.stats.totalTransactions') || 'Total Transactions'}</StatLabel>
                    <StatNumber fontSize="md">{total?.toLocaleString() || '0'}</StatNumber>
                  </Stat>
                </SimpleGrid>
              </CardBody>
            </Card>

            {/* Filters */}
            <Card>
              <CardBody>
                <Text fontSize="md" fontWeight="semibold" mb={4} color="gray.700">
                  {t('common.filter') || 'Filter Transactions'}
                </Text>
                <SimpleGrid columns={{ base: 1, md: 4 }} spacing={4}>
                  <FormControl>
                    <FormLabel fontSize="sm">{t('common.filters.startDate') || 'Start Date'}</FormLabel>
                    <Input
                      type="date"
                      value={filters.start_date}
                      onChange={(e) => handleFilterChange('start_date', e.target.value)}
                      size="sm"
                    />
                  </FormControl>
                  <FormControl>
                    <FormLabel fontSize="sm">{t('common.filters.endDate') || 'End Date'}</FormLabel>
                    <Input
                      type="date"
                      value={filters.end_date}
                      onChange={(e) => handleFilterChange('end_date', e.target.value)}
                      size="sm"
                    />
                  </FormControl>
                  <FormControl>
                    <FormLabel fontSize="sm">Items per page</FormLabel>
                    <Select
                      value={filters.limit}
                      onChange={(e) => handleFilterChange('limit', parseInt(e.target.value))}
                      size="sm"
                    >
                      <option value={10}>10</option>
                      <option value={20}>20</option>
                      <option value={50}>50</option>
                      <option value={100}>100</option>
                    </Select>
                  </FormControl>
                  <FormControl>
                    <FormLabel fontSize="sm">Actions</FormLabel>
                    <HStack>
                      <Tooltip label="Refresh transactions" fontSize="xs">
                        <IconButton
                          aria-label="Refresh"
                          icon={<FiRefreshCw />}
                          size="sm"
                          onClick={fetchTransactions}
                          isLoading={loading}
                        />
                      </Tooltip>
                      <Button
                        leftIcon={<FiFilter />}
                        size="sm"
                        variant="outline"
                        onClick={clearFilters}
                      >
                        Clear
                      </Button>
                    </HStack>
                  </FormControl>
                </SimpleGrid>
              </CardBody>
            </Card>

            {/* Error Alert */}
            {error && (
              <Alert status="error">
                <AlertIcon />
                {error}
              </Alert>
            )}

            {/* Transaction Table */}
            <Box>
              <Text fontSize="md" fontWeight="semibold" mb={4} color="gray.700">
                {t('common.transactionHistory') || 'Transaction History'} ({total || 0} {t('common.pagination.entries') || 'records'})
              </Text>

              {loading ? (
                <Box textAlign="center" py={8}>
                  <Spinner size="lg" />
                  <Text mt={2} color="gray.500">Loading transactions...</Text>
                </Box>
              ) : !Array.isArray(transactions) || transactions.length === 0 ? (
                <Box textAlign="center" py={8}>
                  <Text color="gray.500">No transactions found for this account</Text>
                  <Text fontSize="sm" color="gray.400" mt={1}>
                    Try adjusting your filter criteria or refresh the data
                  </Text>
                </Box>
              ) : (
                <TableContainer>
                  <Table size="sm" variant="striped">
                    <Thead>
                      <Tr>
                        <Th>{t('common.labels.date') || 'Date'}</Th>
                        <Th>{t('common.labels.type') || 'Type'}</Th>
                        <Th>{t('common.placeholders.referenceNumber') || 'Reference'}</Th>
                        <Th isNumeric>{t('common.labels.amount') || 'Amount'}</Th>
                        <Th isNumeric>{t('common.labels.balance') || 'Balance After'}</Th>
                        <Th>{t('common.labels.description') || 'Notes'}</Th>
                      </Tr>
                    </Thead>
                    <Tbody>
                      {Array.isArray(transactions) && transactions.map((transaction) => {
                        const typeInfo = formatTransactionType(transaction.reference_type);
                        const isPositive = transaction.amount >= 0;

                        return (
                          <Tr key={transaction.id}>
                            <Td>
                              <Text fontSize="sm" fontFamily="mono">
                                {new Date(transaction.transaction_date).toLocaleDateString('id-ID')}
                              </Text>
                              <Text fontSize="xs" color="gray.500">
                                {new Date(transaction.transaction_date).toLocaleTimeString('id-ID')}
                              </Text>
                            </Td>
                            <Td>
                              <Badge
                                colorScheme={typeInfo.color}
                                size="sm"
                                variant="solid"
                              >
                                {typeInfo.label}
                              </Badge>
                            </Td>
                            <Td>
                              <Text fontSize="sm">
                                {transaction.reference_type === 'TRANSFER' && transaction.reference_id
                                  ? `TRF-${transaction.reference_id}`
                                  : transaction.reference_id
                                    ? `REF-${transaction.reference_id}`
                                    : 'Manual Entry'
                                }
                              </Text>
                            </Td>
                            <Td isNumeric>
                              <Text
                                fontSize="sm"
                                fontFamily="mono"
                                fontWeight="medium"
                                color={isPositive ? 'green.600' : 'red.600'}
                              >
                                {isPositive ? '+' : ''}{account.currency} {Math.abs(transaction.amount).toLocaleString('id-ID', { maximumFractionDigits: 0 })}
                              </Text>
                            </Td>
                            <Td isNumeric>
                              <Text
                                fontSize="sm"
                                fontFamily="mono"
                                color={transaction.balance_after < 0 ? 'red.500' : 'gray.700'}
                              >
                                {account.currency} {Math.abs(transaction.balance_after).toLocaleString('id-ID', { maximumFractionDigits: 0 })}
                                {transaction.balance_after < 0 && ' (Dr)'}
                              </Text>
                            </Td>
                            <Td>
                              <Text fontSize="sm" color="gray.600" noOfLines={2}>
                                {transaction.notes || '-'}
                              </Text>
                            </Td>
                          </Tr>
                        );
                      })}
                    </Tbody>
                  </Table>
                </TableContainer>
              )}

              {/* Pagination */}
              {totalPages > 1 && (
                <Flex justify="center" align="center" gap={4} mt={6}>
                  <Button
                    size="sm"
                    onClick={() => handlePageChange(currentPage - 1)}
                    isDisabled={currentPage === 1 || loading}
                  >
                    Previous
                  </Button>

                  <HStack spacing={1}>
                    {Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
                      const pageNum = Math.max(1, Math.min(totalPages - 4, currentPage - 2)) + i;
                      if (pageNum > totalPages) return null;

                      return (
                        <Button
                          key={pageNum}
                          size="sm"
                          variant={currentPage === pageNum ? 'solid' : 'ghost'}
                          colorScheme={currentPage === pageNum ? 'blue' : 'gray'}
                          onClick={() => handlePageChange(pageNum)}
                          isDisabled={loading}
                        >
                          {pageNum}
                        </Button>
                      );
                    })}
                  </HStack>

                  <Button
                    size="sm"
                    onClick={() => handlePageChange(currentPage + 1)}
                    isDisabled={currentPage === totalPages || loading}
                  >
                    Next
                  </Button>

                  <Text fontSize="sm" color="gray.500">
                    Page {currentPage} of {totalPages}
                  </Text>
                </Flex>
              )}
            </Box>
          </VStack>
        </ModalBody>
      </ModalContent>
    </Modal>
  );
};

export default TransactionHistoryModal;
