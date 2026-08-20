'use client';

import React, { useCallback, useEffect, useState } from 'react';
import {
  Alert,
  AlertIcon,
  Badge,
  Box,
  Button,
  Card,
  CardBody,
  Flex,
  FormControl,
  FormLabel,
  HStack,
  IconButton,
  Input,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalHeader,
  ModalOverlay,
  Select,
  SimpleGrid,
  Spinner,
  Stat,
  StatHelpText,
  StatLabel,
  StatNumber,
  Table,
  TableContainer,
  Tbody,
  Td,
  Text,
  Th,
  Thead,
  Tooltip,
  Tr,
  useColorModeValue,
  useToast,
  VStack,
} from '@chakra-ui/react';
import { FiFilter, FiRefreshCw } from 'react-icons/fi';
import { useTranslation } from '@/hooks/useTranslation';
import cashbankService, {
  CashBank,
  CashBankTransaction,
  getBookBalanceInIDR,
  TransactionFilter,
  TransactionResult,
} from '@/services/cashbankService';
import { getCashBankAmountPrecision, normalizeCashBankCurrency } from '@/utils/cashBankCurrency';

interface TransactionHistoryModalProps {
  isOpen: boolean;
  onClose: () => void;
  account: CashBank | null;
}

interface ApiErrorShape {
  response?: {
    data?: {
      details?: string;
      message?: string;
    };
  };
  message?: string;
}

const getCurrencyCode = (currency?: string): string => normalizeCashBankCurrency(currency);

const formatNativeAmount = (currency: string, amount: number): string => `${currency} ${Math.abs(amount || 0).toLocaleString('id-ID', {
  minimumFractionDigits: getCashBankAmountPrecision(currency),
  maximumFractionDigits: getCashBankAmountPrecision(currency),
})}`;

const formatIDRAmount = (amount?: number): string => `IDR ${(amount || 0).toLocaleString('id-ID', {
  minimumFractionDigits: 0,
  maximumFractionDigits: 2,
})}`;

const getTransactionEffectiveTimestamp = (transaction: CashBankTransaction): number => {
  const accountingDate = new Date(transaction.transaction_date);
  const createdAt = new Date(transaction.created_at);

  if (!Number.isNaN(accountingDate.getTime()) && !Number.isNaN(createdAt.getTime())) {
    return new Date(
      accountingDate.getFullYear(),
      accountingDate.getMonth(),
      accountingDate.getDate(),
      createdAt.getHours(),
      createdAt.getMinutes(),
      createdAt.getSeconds(),
      createdAt.getMilliseconds(),
    ).getTime();
  }

  if (!Number.isNaN(createdAt.getTime())) {
    return createdAt.getTime();
  }

  return accountingDate.getTime();
};

const getTransactionDisplayDate = (transaction: CashBankTransaction): Date => {
  const transactionDate = new Date(transaction.transaction_date);
  if (!Number.isNaN(transactionDate.getTime())) {
    return transactionDate;
  }

  return new Date(transaction.created_at);
};

const getTransactionDisplayTime = (transaction: CashBankTransaction): Date => {
  const createdAt = new Date(transaction.created_at);
  return !Number.isNaN(createdAt.getTime()) ? createdAt : new Date(transaction.transaction_date);
};

const TransactionHistoryModal: React.FC<TransactionHistoryModalProps> = ({
  isOpen,
  onClose,
  account,
}) => {
  const [transactions, setTransactions] = useState<CashBankTransaction[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [currentPage, setCurrentPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [total, setTotal] = useState(0);
  const [filters, setFilters] = useState<TransactionFilter>({
    page: 1,
    limit: 20,
    start_date: '',
    end_date: '',
    sort_order: 'asc',
  });

  const toast = useToast();
  const { t } = useTranslation();
  const modalBg = useColorModeValue('white', 'gray.800');
  const modalHeaderBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const borderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const cardBg = useColorModeValue('white', 'gray.800');
  const sectionTitleColor = useColorModeValue('gray.700', 'gray.100');
  const textColor = useColorModeValue('gray.800', 'whiteAlpha.900');
  const mutedTextColor = useColorModeValue('gray.500', 'whiteAlpha.700');
  const helperTextColor = useColorModeValue('gray.600', 'whiteAlpha.800');
  const fieldBg = useColorModeValue('white', 'whiteAlpha.50');
  const fieldBorderColor = useColorModeValue('gray.300', 'whiteAlpha.300');
  const fieldHoverBorderColor = useColorModeValue('gray.400', 'whiteAlpha.400');
  const fieldFocusBorderColor = useColorModeValue('blue.500', 'blue.300');
  const placeholderColor = useColorModeValue('gray.500', 'whiteAlpha.500');
  const alertInfoBg = useColorModeValue('blue.50', 'rgba(66, 153, 225, 0.12)');
  const alertInfoBorderColor = useColorModeValue('blue.200', 'blue.700');
  const alertInfoTextColor = useColorModeValue('blue.800', 'blue.100');
  const tableContainerBg = useColorModeValue('white', 'gray.800');
  const tableHeaderBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const tableRowStripeBg = useColorModeValue('gray.50', 'whiteAlpha.50');
  const positiveTextColor = useColorModeValue('green.600', 'green.300');
  const negativeTextColor = useColorModeValue('red.600', 'red.300');
  const neutralTextColor = useColorModeValue('gray.700', 'gray.200');
  const subtleTextColor = useColorModeValue('gray.400', 'whiteAlpha.500');
  const fieldStyles = {
    bg: fieldBg,
    color: textColor,
    borderColor: fieldBorderColor,
    _hover: { borderColor: fieldHoverBorderColor },
    _focusVisible: {
      borderColor: fieldFocusBorderColor,
      boxShadow: `0 0 0 1px ${fieldFocusBorderColor}`,
    },
    _placeholder: { color: placeholderColor },
  };

  const fetchTransactions = useCallback(async () => {
    if (!account) {
      return;
    }

    try {
      setLoading(true);
      setError(null);

      const result: TransactionResult = await cashbankService.getTransactionHistory(account.id, filters);
      const transactionData = Array.isArray(result.data) ? [...result.data] : [];
      const sortOrder = filters.sort_order === 'desc' ? 'desc' : 'asc';
      transactionData.sort((left, right) => {
        const leftTime = getTransactionEffectiveTimestamp(left);
        const rightTime = getTransactionEffectiveTimestamp(right);

        if (leftTime === rightTime) {
          return sortOrder === 'desc' ? right.id - left.id : left.id - right.id;
        }

        return sortOrder === 'desc' ? rightTime - leftTime : leftTime - rightTime;
      });

      setTransactions(transactionData);
      setCurrentPage(result.page || 1);
      setTotalPages(result.total_pages || 1);
      setTotal(result.total || 0);
    } catch (requestError: unknown) {
      console.error('Error fetching transaction history:', requestError);
      const normalizedError = requestError as ApiErrorShape;
      const errorMessage = normalizedError.response?.data?.details || normalizedError.message || 'Failed to fetch transaction history';
      setError(errorMessage);
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
  }, [account, filters, toast]);

  useEffect(() => {
    if (isOpen && account) {
      void fetchTransactions();
    }
  }, [isOpen, account, fetchTransactions]);

  const handleFilterChange = (field: keyof TransactionFilter, value: string | number) => {
    setFilters(prev => ({
      ...prev,
      [field]: value,
      page: 1,
    }));
  };

  const handlePageChange = (newPage: number) => {
    setFilters(prev => ({
      ...prev,
      page: newPage,
    }));
  };

  const clearFilters = () => {
    setFilters({
      page: 1,
      limit: 20,
      start_date: '',
      end_date: '',
      sort_order: 'asc',
    });
  };

  const formatTransactionType = (type: string) => {
    switch (type) {
      case 'DEPOSIT':
        return { label: t('common.transactionTypes.deposit'), color: 'green' };
      case 'WITHDRAWAL':
        return { label: t('common.transactionTypes.withdrawal'), color: 'red' };
      case 'OWNER_DRAWING':
        return { label: 'Pengambilan Pemilik / Prive', color: 'orange' };
      case 'TRANSFER':
        return { label: t('common.transactionTypes.transfer'), color: 'blue' };
      case 'OPENING_BALANCE':
        return { label: t('common.transactionTypes.openingBalance'), color: 'purple' };
      case 'ADJUSTMENT':
        return { label: t('common.transactionTypes.adjustment'), color: 'orange' };
      default:
        return { label: type, color: 'gray' };
    }
  };

  if (!account) {
    return null;
  }

  const accountCurrency = getCurrencyCode(account.currency);
  const currentBookBalanceIDR = getBookBalanceInIDR(account);

  return (
    <Modal isOpen={isOpen} onClose={onClose} size="7xl">
      <ModalOverlay />
      <ModalContent bg={modalBg} border="1px solid" borderColor={borderColor}>
        <ModalHeader bg={modalHeaderBg} borderBottom="1px solid" borderColor={borderColor}>
          <Flex alignItems="center" gap={3}>
            <Text fontSize="lg">Transaction History</Text>
            <Box>
              <Text fontSize="lg" fontWeight="bold" color={sectionTitleColor}>
                {t('common.transactionHistory')}
              </Text>
              <Text fontSize="sm" color={mutedTextColor} fontFamily="mono">
                {account.name} ({account.code})
              </Text>
            </Box>
          </Flex>
        </ModalHeader>
        <ModalCloseButton />

        <ModalBody pb={6}>
          <VStack spacing={6} align="stretch">
            <Card bg={cardBg} border="1px solid" borderColor={borderColor}>
              <CardBody>
                <SimpleGrid columns={{ base: 1, md: 4 }} spacing={4}>
                  <Stat size="sm">
                    <StatLabel color={mutedTextColor}>{t('accounts.accountType')}</StatLabel>
                    <StatNumber fontSize="md" color={textColor}>
                      <Badge colorScheme={account.type === 'CASH' ? 'green' : 'blue'}>
                        {account.type}
                      </Badge>
                    </StatNumber>
                  </Stat>
                  <Stat size="sm">
                    <StatLabel color={mutedTextColor}>{t('settings.currency')}</StatLabel>
                    <StatNumber fontSize="md" color={textColor}>{accountCurrency}</StatNumber>
                  </Stat>
                  <Stat size="sm">
                    <StatLabel color={mutedTextColor}>{t('common.currentBalance')}</StatLabel>
                    <StatNumber fontSize="md" color={account.balance < 0 ? negativeTextColor : positiveTextColor} fontFamily="mono">
                      {formatNativeAmount(accountCurrency, account.balance)}
                      {account.balance < 0 && ' (Dr)'}
                    </StatNumber>
                    <StatHelpText color={helperTextColor}>Nilai buku: {formatIDRAmount(currentBookBalanceIDR)}</StatHelpText>
                  </Stat>
                  <Stat size="sm">
                    <StatLabel color={mutedTextColor}>{t('dashboard.stats.totalTransactions')}</StatLabel>
                    <StatNumber fontSize="md" color={textColor}>{total.toLocaleString()}</StatNumber>
                  </Stat>
                </SimpleGrid>
              </CardBody>
            </Card>

            <Card bg={cardBg} border="1px solid" borderColor={borderColor}>
              <CardBody>
                <Text fontSize="md" fontWeight="semibold" mb={4} color={sectionTitleColor}>
                  {t('common.filter')}
                </Text>
                <SimpleGrid columns={{ base: 1, md: 5 }} spacing={4}>
                  <FormControl>
                    <FormLabel fontSize="sm">{t('common.filters.startDate')}</FormLabel>
                    <Input
                      type="date"
                      value={filters.start_date}
                      onChange={(e) => handleFilterChange('start_date', e.target.value)}
                      size="sm"
                      {...fieldStyles}
                    />
                  </FormControl>
                  <FormControl>
                    <FormLabel fontSize="sm">{t('common.filters.endDate')}</FormLabel>
                    <Input
                      type="date"
                      value={filters.end_date}
                      onChange={(e) => handleFilterChange('end_date', e.target.value)}
                      size="sm"
                      {...fieldStyles}
                    />
                  </FormControl>
                  <FormControl>
                    <FormLabel fontSize="sm">{t('common.itemsPerPage')}</FormLabel>
                    <Select
                      value={filters.limit}
                      onChange={(e) => handleFilterChange('limit', parseInt(e.target.value, 10))}
                      size="sm"
                      {...fieldStyles}
                    >
                      <option value={10}>10</option>
                      <option value={20}>20</option>
                      <option value={50}>50</option>
                      <option value={100}>100</option>
                    </Select>
                  </FormControl>
                  <FormControl>
                    <FormLabel fontSize="sm">{t('common.sort.timeOrder')}</FormLabel>
                    <Select
                      value={filters.sort_order || 'asc'}
                      onChange={(e) => handleFilterChange('sort_order', e.target.value as 'asc' | 'desc')}
                      size="sm"
                      {...fieldStyles}
                    >
                      <option value="asc">{t('common.sort.timeOldestFirst')}</option>
                      <option value="desc">{t('common.sort.timeNewestFirst')}</option>
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
                          onClick={() => void fetchTransactions()}
                          isLoading={loading}
                        />
                      </Tooltip>
                      <Button leftIcon={<FiFilter />} size="sm" variant="outline" onClick={clearFilters}>
                        {t('common.clear')}
                      </Button>
                    </HStack>
                  </FormControl>
                </SimpleGrid>
              </CardBody>
            </Card>

            {error && (
              <Alert status="error">
                <AlertIcon />
                {error}
              </Alert>
            )}

            <Box>
              <Text fontSize="md" fontWeight="semibold" mb={4} color={sectionTitleColor}>
                {t('common.transactionHistory')} ({total} {t('common.pagination.entries')})
              </Text>

              <Alert status="info" mb={4} borderRadius="md" alignItems="flex-start" bg={alertInfoBg} border="1px solid" borderColor={alertInfoBorderColor}>
                <AlertIcon mt={1} />
                <Box>
                  <Flex gap={2} mb={2} wrap="wrap">
                    <Badge colorScheme="blue" variant="subtle">NATIVE AMOUNT</Badge>
                    <Badge colorScheme="green" variant="subtle">BOOK AMOUNT IDR</Badge>
                  </Flex>
                  <Text fontSize="sm" fontWeight="medium" color={alertInfoTextColor}>
                    Riwayat ini memisahkan nominal rekening dan nilai buku ledger.
                  </Text>
                  <Text fontSize="xs" color={helperTextColor} mt={1}>
                    `Native Amount` mengikuti mata uang rekening. `Book Amount` adalah nilai yang diposting ke ledger dalam IDR. `Rate` akan terisi untuk transaksi yang memakai kurs.
                  </Text>
                </Box>
              </Alert>

              {loading ? (
                <Box textAlign="center" py={8}>
                  <Spinner size="lg" />
                  <Text mt={2} color={mutedTextColor}>{t('common.loadingTransactions')}</Text>
                </Box>
              ) : transactions.length === 0 ? (
                <Box textAlign="center" py={8}>
                  <Text color={mutedTextColor}>{t('common.noTransactionsFound')}</Text>
                  <Text fontSize="sm" color={subtleTextColor} mt={1}>
                    {t('common.noTransactionsHint')}
                  </Text>
                </Box>
              ) : (
                <TableContainer bg={tableContainerBg} border="1px solid" borderColor={borderColor} borderRadius="md">
                  <Table size="sm" variant="striped">
                    <Thead bg={tableHeaderBg}>
                      <Tr>
                        <Th color={mutedTextColor}>{t('common.labels.date')}</Th>
                        <Th color={mutedTextColor}>{t('common.labels.type')}</Th>
                        <Th color={mutedTextColor}>{t('common.placeholders.referenceNumber')}</Th>
                        <Th isNumeric color={mutedTextColor}>Native Amount</Th>
                        <Th isNumeric color={mutedTextColor}>Rate</Th>
                        <Th isNumeric color={mutedTextColor}>Book Amount</Th>
                        <Th isNumeric color={mutedTextColor}>Ending Native Balance</Th>
                        <Th color={mutedTextColor}>{t('common.labels.description')}</Th>
                      </Tr>
                    </Thead>
                    <Tbody>
                      {transactions.map(transaction => {
                        const typeInfo = formatTransactionType(transaction.reference_type);
                        const transactionCurrency = getCurrencyCode(transaction.currency || accountCurrency);
                        const isPositive = transaction.amount >= 0;
                        const displayDate = getTransactionDisplayDate(transaction);
                        const displayTime = getTransactionDisplayTime(transaction);

                        return (
                          <Tr key={transaction.id} _even={{ bg: tableRowStripeBg }}>
                            <Td>
                              <Text fontSize="sm" fontFamily="mono" color={textColor}>
                                {displayDate.toLocaleDateString('id-ID')}
                              </Text>
                              <Text fontSize="xs" color={mutedTextColor}>
                                {displayTime.toLocaleTimeString('id-ID')}
                              </Text>
                            </Td>
                            <Td>
                              <Badge colorScheme={typeInfo.color} size="sm" variant="solid">
                                {typeInfo.label}
                              </Badge>
                            </Td>
                            <Td>
                              <Text fontSize="sm" color={textColor}>
                                {transaction.reference_type === 'TRANSFER' && transaction.reference_id
                                  ? `TRF-${transaction.reference_id}`
                                  : transaction.reference_id
                                    ? `REF-${transaction.reference_id}`
                                    : 'Manual Entry'}
                              </Text>
                            </Td>
                            <Td isNumeric>
                              <Text
                                fontSize="sm"
                                fontFamily="mono"
                                fontWeight="medium"
                                color={isPositive ? positiveTextColor : negativeTextColor}
                              >
                                {isPositive ? '+' : '-'}{formatNativeAmount(transactionCurrency, transaction.amount)}
                              </Text>
                            </Td>
                            <Td isNumeric>
                              {(transaction.exchange_rate || 0) > 0 ? (
                                <Text fontSize="sm" fontFamily="mono" color={helperTextColor}>
                                  {(transaction.exchange_rate || 0).toLocaleString('id-ID', {
                                    minimumFractionDigits: getCashBankAmountPrecision(transactionCurrency),
                                    maximumFractionDigits: 6,
                                  })}
                                </Text>
                              ) : (
                                <Text fontSize="xs" color={subtleTextColor}>
                                  Base / carry
                                </Text>
                              )}
                            </Td>
                            <Td isNumeric>
                              <Text fontSize="sm" fontFamily="mono" color={transaction.book_amount_idr && transaction.book_amount_idr >= 0 ? positiveTextColor : negativeTextColor}>
                                {formatIDRAmount(transaction.book_amount_idr)}
                              </Text>
                              <Text fontSize="xs" color={subtleTextColor}>
                                Posted to ledger
                              </Text>
                            </Td>
                            <Td isNumeric>
                              <Text fontSize="sm" fontFamily="mono" color={transaction.balance_after < 0 ? negativeTextColor : neutralTextColor}>
                                {formatNativeAmount(transactionCurrency, transaction.balance_after)}
                                {transaction.balance_after < 0 && ' (Dr)'}
                              </Text>
                            </Td>
                            <Td>
                              <Text fontSize="sm" color={helperTextColor} noOfLines={2}>
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

              {totalPages > 1 && (
                <Flex justify="center" align="center" gap={4} mt={6}>
                  <Button size="sm" onClick={() => handlePageChange(currentPage - 1)} isDisabled={currentPage === 1 || loading}>
                    {t('common.previous')}
                  </Button>

                  <HStack spacing={1}>
                    {Array.from({ length: Math.min(5, totalPages) }, (_, index) => {
                      const pageNum = Math.max(1, Math.min(totalPages - 4, currentPage - 2)) + index;
                      if (pageNum > totalPages) {
                        return null;
                      }

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

                  <Button size="sm" onClick={() => handlePageChange(currentPage + 1)} isDisabled={currentPage === totalPages || loading}>
                    {t('common.next')}
                  </Button>

                  <Text fontSize="sm" color={mutedTextColor}>
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
