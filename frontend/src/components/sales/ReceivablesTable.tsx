'use client';

import React, { useState, useEffect } from 'react';
import {
  Box,
  Card,
  CardHeader,
  CardBody,
  Heading,
  Text,
  VStack,
  HStack,
  Badge,
  Button,
  SimpleGrid,
  Stat,
  StatLabel,
  StatNumber,
  StatHelpText,
  Flex,
  useToast,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  TableContainer,
  Progress,
  Divider,
  Select,
  Input,
  InputGroup,
  InputLeftElement
} from '@chakra-ui/react';
import {
  FiRefreshCw,
  FiDownload,
  FiSearch,
  FiAlertCircle,
  FiDollarSign,
  FiTrendingUp,
  FiUsers,
  FiFileText
} from 'react-icons/fi';
import salesService, { ReceivablesReport, ReceivableItem } from '@/services/salesService';
import { useTranslation } from '@/hooks/useTranslation';

interface ReceivablesTableProps {
  // Optional props for filtering or customization
  customerId?: number;
  showExportButton?: boolean;
}

interface AgingSummary {
  current: number;
  days1to30: number;
  days31to60: number;
  days61to90: number;
  days90plus: number;
}

const ReceivablesTable: React.FC<ReceivablesTableProps> = ({
  customerId,
  showExportButton = true
}) => {
  const { t } = useTranslation();
  const [receivablesData, setReceivablesData] = useState<ReceivablesReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [overdueFilter, setOverdueFilter] = useState('');
  const toast = useToast();

  useEffect(() => {
    loadReceivablesData();
  }, [customerId]);

  const loadReceivablesData = async () => {
    try {
      setLoading(true);
      const data = await salesService.getReceivablesReport();
      setReceivablesData(data);
    } catch (error) {
      toast({
        title: 'Error loading receivables',
        description: 'Failed to load receivables data',
        status: 'error',
        duration: 3000
      });
    } finally {
      setLoading(false);
    }
  };

  const handleExportReport = async () => {
    try {
      // Implementation would depend on backend PDF export for receivables
      toast({
        title: 'Export functionality',
        description: 'Export feature coming soon',
        status: 'info',
        duration: 3000
      });
    } catch (error) {
      toast({
        title: 'Export failed',
        description: 'Failed to export receivables report',
        status: 'error',
        duration: 3000
      });
    }
  };

  const getStatusBadgeColor = (status: string, daysOverdue: number) => {
    if (daysOverdue > 0) return 'red';
    if (status === 'PAID') return 'green';
    if (status === 'INVOICED') return 'blue';
    return 'gray';
  };

  const getStatusLabel = (status: string, daysOverdue: number) => {
    if (daysOverdue > 0) return `Overdue (${daysOverdue} days)`;
    return salesService.getStatusLabel(status);
  };

  const getOverdueCategory = (daysOverdue: number) => {
    if (daysOverdue <= 0) return 'Current';
    if (daysOverdue <= 30) return '1-30 Days';
    if (daysOverdue <= 60) return '31-60 Days';
    if (daysOverdue <= 90) return '61-90 Days';
    return '90+ Days';
  };

  const filteredReceivables = receivablesData?.receivables?.filter(item => {
    const matchesSearch = !searchTerm || 
      item.customer_name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      item.invoice_number?.toLowerCase().includes(searchTerm.toLowerCase());
    
    const matchesStatus = !statusFilter || item.status === statusFilter;
    
    const matchesOverdue = !overdueFilter || 
      (overdueFilter === 'current' && item.days_overdue <= 0) ||
      (overdueFilter === '1-30' && item.days_overdue > 0 && item.days_overdue <= 30) ||
      (overdueFilter === '31-60' && item.days_overdue > 30 && item.days_overdue <= 60) ||
      (overdueFilter === '61-90' && item.days_overdue > 60 && item.days_overdue <= 90) ||
      (overdueFilter === '90+' && item.days_overdue > 90);
    
    return matchesSearch && matchesStatus && matchesOverdue;
  }) || [];

  const calculateAgingSummary = (): AgingSummary => {
    if (!receivablesData?.receivables) {
      return {
        current: 0,
        days1to30: 0,
        days31to60: 0,
        days61to90: 0,
        days90plus: 0,
      };
    }
    
    const aging = {
      current: 0,
      days1to30: 0,
      days31to60: 0,
      days61to90: 0,
      days90plus: 0
    };

    receivablesData.receivables.forEach(item => {
      if (item.days_overdue <= 0) {
        aging.current += item.outstanding_amount;
      } else if (item.days_overdue <= 30) {
        aging.days1to30 += item.outstanding_amount;
      } else if (item.days_overdue <= 60) {
        aging.days31to60 += item.outstanding_amount;
      } else if (item.days_overdue <= 90) {
        aging.days61to90 += item.outstanding_amount;
      } else {
        aging.days90plus += item.outstanding_amount;
      }
    });

    return aging;
  };

  const agingSummary = calculateAgingSummary();

  if (loading) {
    return (
      <Box>
        <Progress size="sm" isIndeterminate />
        <Text mt={4} textAlign="center">{t('common.loading')}</Text>
      </Box>
    );
  }

  if (!receivablesData) {
    return (
      <Card>
        <CardBody>
          <Text textAlign="center" color="gray.500">
            {t('common.noData')}
          </Text>
        </CardBody>
      </Card>
    );
  }

  return (
    <VStack spacing={6} align="stretch">
      {/* Summary Cards */}
      <SimpleGrid columns={{ base: 1, md: 2, lg: 4 }} spacing={4}>
        <Card>
          <CardBody>
            <Flex justify="space-between" align="start">
              <Box>
                <Text fontSize="sm" color="gray.600" mb={1}>
                  Total Outstanding
                </Text>
                <Text fontSize="2xl" fontWeight="bold">
                  {salesService.formatCurrency(receivablesData.total_outstanding)}
                </Text>
              </Box>
              <Box p={3} bg="blue.50" borderRadius="md" color="blue.500">
                <FiDollarSign size={20} />
              </Box>
            </Flex>
          </CardBody>
        </Card>

        <Card>
          <CardBody>
            <Flex justify="space-between" align="start">
              <Box>
                <Text fontSize="sm" color="gray.600" mb={1}>
                  Overdue Amount
                </Text>
                <Text fontSize="2xl" fontWeight="bold" color="red.500">
                  {salesService.formatCurrency(receivablesData.overdue_amount)}
                </Text>
              </Box>
              <Box p={3} bg="red.50" borderRadius="md" color="red.500">
                <FiAlertCircle size={20} />
              </Box>
            </Flex>
          </CardBody>
        </Card>

        <Card>
          <CardBody>
            <Flex justify="space-between" align="start">
              <Box>
                <Text fontSize="sm" color="gray.600" mb={1}>
                  Total Invoices
                </Text>
                <Text fontSize="2xl" fontWeight="bold">
                  {receivablesData.receivables?.length || 0}
                </Text>
              </Box>
              <Box p={3} bg="purple.50" borderRadius="md" color="purple.500">
                <FiFileText size={20} />
              </Box>
            </Flex>
          </CardBody>
        </Card>

        <Card>
          <CardBody>
            <Flex justify="space-between" align="start">
              <Box>
                <Text fontSize="sm" color="gray.600" mb={1}>
                  Collection Rate
                </Text>
                <Text fontSize="2xl" fontWeight="bold" color="green.500">
                  {receivablesData.total_outstanding > 0 
                    ? Math.round(((receivablesData.total_outstanding - receivablesData.overdue_amount) / receivablesData.total_outstanding) * 100)
                    : 100}%
                </Text>
              </Box>
              <Box p={3} bg="green.50" borderRadius="md" color="green.500">
                <FiTrendingUp size={20} />
              </Box>
            </Flex>
          </CardBody>
        </Card>
      </SimpleGrid>

      {/* Aging Analysis */}
      <Card>
        <CardHeader>
          <Heading size="md">Aging Analysis</Heading>
        </CardHeader>
        <CardBody>
          <SimpleGrid columns={{ base: 1, md: 5 }} spacing={4}>
            <Stat>
              <StatLabel>Current</StatLabel>
              <StatNumber fontSize="lg">{salesService.formatCurrency(agingSummary.current || 0)}</StatNumber>
              <StatHelpText color="green.500">0 days</StatHelpText>
            </Stat>
            <Stat>
              <StatLabel>1-30 Days</StatLabel>
              <StatNumber fontSize="lg">{salesService.formatCurrency(agingSummary.days1to30 || 0)}</StatNumber>
              <StatHelpText color="yellow.500">Recent overdue</StatHelpText>
            </Stat>
            <Stat>
              <StatLabel>31-60 Days</StatLabel>
              <StatNumber fontSize="lg">{salesService.formatCurrency(agingSummary.days31to60 || 0)}</StatNumber>
              <StatHelpText color="orange.500">Moderate risk</StatHelpText>
            </Stat>
            <Stat>
              <StatLabel>61-90 Days</StatLabel>
              <StatNumber fontSize="lg">{salesService.formatCurrency(agingSummary.days61to90 || 0)}</StatNumber>
              <StatHelpText color="red.400">High risk</StatHelpText>
            </Stat>
            <Stat>
              <StatLabel>90+ Days</StatLabel>
              <StatNumber fontSize="lg">{salesService.formatCurrency(agingSummary.days90plus || 0)}</StatNumber>
              <StatHelpText color="red.600">Critical</StatHelpText>
            </Stat>
          </SimpleGrid>
        </CardBody>
      </Card>

      {/* Receivables Table */}
      <Card>
        <CardHeader>
          <Flex justify="space-between" align="center">
            <Heading size="md">Outstanding Receivables</Heading>
            <HStack spacing={3}>
              <Button
                size="sm"
                variant="ghost"
                leftIcon={<FiRefreshCw />}
                onClick={loadReceivablesData}
                isLoading={loading}
              >
                {t('common.refresh')}
              </Button>
              {showExportButton && (
                <Button
                  size="sm"
                  variant="outline"
                  leftIcon={<FiDownload />}
                  onClick={handleExportReport}
                >
                  {t('common.export')}
                </Button>
              )}
            </HStack>
          </Flex>
          
          {/* Filters */}
          <HStack spacing={4} mt={4}>
            <InputGroup maxW="300px">
              <InputLeftElement pointerEvents="none">
                <FiSearch color="gray.300" />
              </InputLeftElement>
              <Input
                placeholder={t('common.placeholders.search')}
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
              />
            </InputGroup>
            
            <Select
              placeholder={t('common.filters.allStatus')}
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              maxW="200px"
            >
              <option value="INVOICED">{t('sales.invoiced')}</option>
              <option value="OVERDUE">{t('sales.overdue')}</option>
            </Select>
            
            <Select
              placeholder={t('common.filters.allAges')}
              value={overdueFilter}
              onChange={(e) => setOverdueFilter(e.target.value)}
              maxW="200px"
            >
              <option value="current">Current</option>
              <option value="1-30">1-30 Days</option>
              <option value="31-60">31-60 Days</option>
              <option value="61-90">61-90 Days</option>
              <option value="90+">90+ Days</option>
            </Select>
          </HStack>
        </CardHeader>
        
        <CardBody>
          <TableContainer>
            <Table variant="simple">
              <Thead>
                <Tr>
                  <Th>Customer</Th>
                  <Th>Invoice</Th>
                  <Th>Date</Th>
                  <Th>Due Date</Th>
                  <Th isNumeric>Total Amount</Th>
                  <Th isNumeric>Paid Amount</Th>
                  <Th isNumeric>Outstanding</Th>
                  <Th>Status</Th>
                  <Th>Age</Th>
                </Tr>
              </Thead>
              <Tbody>
                {filteredReceivables.map((item) => (
                  <Tr key={item.sale_id}>
                    <Td fontWeight="medium">{item.customer_name}</Td>
                    <Td>
                      <Text color="blue.500" fontWeight="medium">
                        {item.invoice_number}
                      </Text>
                    </Td>
                    <Td>{salesService.formatDate(item.date)}</Td>
                    <Td>
                      <Text color={item.days_overdue > 0 ? 'red.500' : 'gray.700'}>
                        {salesService.formatDate(item.due_date)}
                      </Text>
                    </Td>
                    <Td isNumeric>{salesService.formatCurrency(item.total_amount)}</Td>
                    <Td isNumeric>{salesService.formatCurrency(item.paid_amount)}</Td>
                    <Td isNumeric>
                      <Text fontWeight="bold" color={item.days_overdue > 0 ? 'red.500' : 'gray.700'}>
                        {salesService.formatCurrency(item.outstanding_amount)}
                      </Text>
                    </Td>
                    <Td>
                      <Badge
                        colorScheme={getStatusBadgeColor(item.status, item.days_overdue)}
                        variant="subtle"
                      >
                        {getStatusLabel(item.status, item.days_overdue)}
                      </Badge>
                    </Td>
                    <Td>
                      <Badge
                        colorScheme={item.days_overdue > 60 ? 'red' : item.days_overdue > 30 ? 'orange' : item.days_overdue > 0 ? 'yellow' : 'green'}
                        variant="outline"
                      >
                        {getOverdueCategory(item.days_overdue)}
                      </Badge>
                    </Td>
                  </Tr>
                ))}
              </Tbody>
            </Table>
          </TableContainer>
          
          {filteredReceivables.length === 0 && (
            <Box py={8} textAlign="center">
              <Text color="gray.500">{t('common.table.emptyState')}</Text>
            </Box>
          )}
        </CardBody>
      </Card>
    </VStack>
  );
};

export default ReceivablesTable;
