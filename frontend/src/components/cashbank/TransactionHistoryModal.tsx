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
      case 'DEPOSIT': return { label: t('common.transactionTypes.deposit'), color: 'green' };
      case 'WITHDRAWAL': return { label: t('common.transactionTypes.withdrawal'), color: 'red' };
      case 'TRANSFER': return { label: t('common.transactionTypes.transfer'), color: 'blue' };
      case 'OPENING_BALANCE': return { label: t('common.transactionTypes.openingBalance'), color: 'purple' };
      case 'ADJUSTMENT': return { label: t('common.transactionTypes.adjustment'), color: 'orange' };
      case 'SALE': return { label: t('common.transactionTypes.sale'), color: 'teal' };
      case 'PURCHASE': return { label: t('common.transactionTypes.purchase'), color: 'orange' };
      case 'SALES_PAYMENT': return { label: t('common.transactionTypes.salesPayment'), color: 'green' };
      case 'PURCHASE_PAYMENT': return { label: t('common.transactionTypes.purchasePayment'), color: 'red' };
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
                {t('common.transactionHistory')}
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
                    <StatLabel>{t('accounts.accountType')}</StatLabel>
                    <StatNumber fontSize="md">
                      <Badge colorScheme={account.type === 'CASH' ? 'green' : 'blue'}>
                        {account.type}
                      </Badge>
                    </StatNumber>
                  </Stat>
                  <Stat size="sm">
                    <StatLabel>{t('settings.currency')}</StatLabel>
                    <StatNumber fontSize="md">{account.currency}</StatNumber>
                  </Stat>
                  <Stat size="sm">
                    <StatLabel>{t('common.currentBalance')}</StatLabel>
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
                    <StatLabel>{t('dashboard.stats.totalTransactions')}</StatLabel>
                    <StatNumber fontSize="md">{total?.toLocaleString() || '0'}</StatNumber>
                  </Stat>
                </SimpleGrid>
              </CardBody>
            </Card>

            {/* Filters */}
            <Card>
              <CardBody>
                <Text fontSize="md" fontWeight="semibold" mb={4} color="gray.700">
                  {t('common.filter')}
                </Text>
                <SimpleGrid columns={{ base: 1, md: 4 }} spacing={4}>
                  <FormControl>
                    <FormLabel fontSize="sm">{t('common.filters.startDate')}</FormLabel>
                    <Input
                      type="date"
                      value={filters.start_date}
                      onChange={(e) => handleFilterChange('start_date', e.target.value)}
                      size="sm"
                    />
                  </FormControl>
                  <FormControl>
                    <FormLabel fontSize="sm">{t('common.filters.endDate')}</FormLabel>
                    <Input
                      type="date"
                      value={filters.end_date}
                      onChange={(e) => handleFilterChange('end_date', e.target.value)}
                      size="sm"
                    />
                  </FormControl>
                  <FormControl>
                    <FormLabel fontSize="sm">{t('common.itemsPerPage')}</FormLabel>
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
                    <FormLabel fontSize="sm">{t('common.actions')}</FormLabel>
                    <HStack>
                      <Tooltip label={t('common.refreshTransactions')} fontSize="xs">
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
                        {t('common.clear')}
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
                {t('common.transactionHistory')} ({total || 0} {t('common.pagination.entries')})
              </Text>

              {loading ? (
                <Box textAlign="center" py={8}>
                  <Spinner size="lg" />
                  <Text mt={2} color="gray.500">{t('common.loadingTransactions')}</Text>
                </Box>
              ) : !Array.isArray(transactions) || transactions.length === 0 ? (
                <Box textAlign="center" py={8}>
                  <Text color="gray.500">{t('common.noTransactionsFound')}</Text>
                  <Text fontSize="sm" color="gray.400" mt={1}>
                    {t('common.noTransactionsHint')}
                  </Text>
                </Box>
              ) : (
                <TableContainer>
                  <Table size="sm" variant="striped">
                    <Thead>
                      <Tr>
                        <Th>{t('common.labels.date')}</Th>
                        <Th>{t('common.labels.type')}</Th>
                        <Th>{t('common.placeholders.referenceNumber')}</Th>
                        <Th isNumeric>{t('common.labels.amount')}</Th>
                        <Th isNumeric>{t('common.labels.balance')}</Th>
                        <Th>{t('common.labels.description')}</Th>
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
                                {new Date(transaction.created_at).toLocaleTimeString('id-ID')}
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
                    {t('common.previous')}
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
                    {t('common.next')}
                  </Button>

                  <Text fontSize="sm" color="gray.500">
                    {t('common.pageOf', { current: currentPage, total: totalPages })}
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
