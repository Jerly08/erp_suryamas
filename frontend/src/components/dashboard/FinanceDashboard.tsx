'use client';
import React, { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import {
  Box,
  Flex,
  Heading,
  Text,
  Card,
  CardHeader,
  CardBody,
  Button,
  HStack,
  Icon,
  Spinner,
  Alert,
  AlertIcon,
  useColorModeValue
} from '@chakra-ui/react';
import {
  FiDollarSign,
  FiShoppingCart,
  FiTrendingUp,
  FiPlus,
  FiBarChart2
} from 'react-icons/fi';
import api from '../../services/api';
import { API_ENDPOINTS } from '@/config/api';
import { useTranslation } from '@/hooks/useTranslation';

interface FinanceDashboardData {
  invoices_pending_payment: number;
  invoices_not_paid: number;
  journals_need_posting: number;
  outstanding_receivables: number;
  outstanding_payables: number;
  purchases_pending_approval: number;
  sales_pending_approval: number;
}

export const FinanceDashboard = () => {
  const router = useRouter();
  const { t } = useTranslation();
  const [data, setData] = useState<FinanceDashboardData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const headingColor = useColorModeValue('gray.800', 'whiteAlpha.900');
  const mutedTextColor = useColorModeValue('gray.500', 'whiteAlpha.700');
  const cardBg = useColorModeValue('white', 'gray.800');
  const cardBorderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const statCardBg = useColorModeValue('white', 'whiteAlpha.100');
  const statCardShadow = useColorModeValue('sm', 'dark-lg');

  useEffect(() => {
    fetchFinanceDashboardData();
  }, []);

  const fetchFinanceDashboardData = async () => {
    try {
      setLoading(true);
      const response = await api.get(API_ENDPOINTS.DASHBOARD_FINANCE);
      // Exclude bank reconciliation fields if present
      const d = response.data.data || {};
      setData({
        invoices_pending_payment: d.invoices_pending_payment ?? 0,
        invoices_not_paid: d.invoices_not_paid ?? 0,
        journals_need_posting: d.journals_need_posting ?? 0,
        outstanding_receivables: d.outstanding_receivables ?? 0,
        outstanding_payables: d.outstanding_payables ?? 0,
        purchases_pending_approval: d.purchases_pending_approval ?? 0,
        sales_pending_approval: d.sales_pending_approval ?? 0,
      });
    } catch (error: any) {
      console.error('Error fetching finance dashboard data:', error);
      setError(error.response?.data?.error || t('messages.operationFailed'));
    } finally {
      setLoading(false);
    }
  };

  const formatCurrency = (amount: number) => {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      minimumFractionDigits: 0,
    }).format(amount || 0);
  };

  if (loading) {
    return (
      <Box>
        <Heading as="h2" size="xl" mb={6} color={headingColor}>
          {t('navigation.dashboard')} - {t('users.finance')}
        </Heading>
        <Flex justify="center" align="center" h="200px">
          <Spinner size="xl" color="blue.500" />
        </Flex>
      </Box>
    );
  }

  if (error) {
    return (
      <Box>
        <Heading as="h2" size="xl" mb={6} color={headingColor}>
          {t('navigation.dashboard')} - {t('users.finance')}
        </Heading>
        <Alert status="error">
          <AlertIcon />
          {error}
        </Alert>
      </Box>
    );
  }

  if (!data) {
    return (
      <Box>
        <Heading as="h2" size="xl" mb={6} color={headingColor}>
          {t('navigation.dashboard')} - {t('users.finance')}
        </Heading>
        <Alert status="info">
          <AlertIcon />
          {t('common.noData')}
        </Alert>
      </Box>
    );
  }

  return (
    <Box>
      <Heading as="h2" size="xl" mb={6} color={headingColor}>
        {t('navigation.dashboard')} - {t('users.finance')}
      </Heading>

      <Flex gap={4} flexWrap="wrap" mt={4}>
        <Box bg={statCardBg} border="1px" borderColor={cardBorderColor} p={4} borderRadius="lg" boxShadow={statCardShadow} flex="1" minW="220px">
          <Heading as="h3" size="sm" mb={2} color="orange.600">{t('dashboard.stats.accountsReceivable')}</Heading>
          <Text fontSize="2xl" fontWeight="bold" color="orange.600">{data.invoices_pending_payment}</Text>
          <Text fontSize="sm" color={mutedTextColor} mt={1}>
            {t('common.labels.total')}: {formatCurrency(data.outstanding_receivables)}
          </Text>
        </Box>

        <Box bg={statCardBg} border="1px" borderColor={cardBorderColor} p={4} borderRadius="lg" boxShadow={statCardShadow} flex="1" minW="220px">
          <Heading as="h3" size="sm" mb={2} color="red.600">{t('dashboard.stats.accountsPayable')}</Heading>
          <Text fontSize="2xl" fontWeight="bold" color="red.600">{data.invoices_not_paid}</Text>
          <Text fontSize="sm" color={mutedTextColor} mt={1}>
            {t('common.labels.total')}: {formatCurrency(data.outstanding_payables)}
          </Text>
        </Box>

        <Box bg={statCardBg} border="1px" borderColor={cardBorderColor} p={4} borderRadius="lg" boxShadow={statCardShadow} flex="1" minW="220px">
          <Heading as="h3" size="sm" mb={2} color="blue.600">{t('dashboard.pendingApprovals')}</Heading>
          <Text fontSize="2xl" fontWeight="bold" color="blue.600">{data.purchases_pending_approval + data.sales_pending_approval}</Text>
          <Text fontSize="sm" color={mutedTextColor} mt={1}>
            {data.purchases_pending_approval > 0 && `${data.purchases_pending_approval} ${t('purchases.title')}`}
            {data.purchases_pending_approval > 0 && data.sales_pending_approval > 0 && ', '}
            {data.sales_pending_approval > 0 && `${data.sales_pending_approval} ${t('sales.title')}`}
            {data.purchases_pending_approval === 0 && data.sales_pending_approval === 0 && t('sales.draft')}
          </Text>
        </Box>
      </Flex>

      <Card mt={6} bg={cardBg} border="1px" borderColor={cardBorderColor} boxShadow="md">
        <CardHeader>
          <Heading size="md" display="flex" alignItems="center">
            <Icon as={FiPlus} mr={2} color="blue.500" />
            {t('dashboard.quickAccess.title')}
          </Heading>
        </CardHeader>
        <CardBody>
          <HStack spacing={4} flexWrap="wrap">
            <Button
              leftIcon={<FiDollarSign />}
              colorScheme="green"
              variant="outline"
              onClick={() => router.push('/sales')}
              size="md"
            >
              {t('dashboard.quickAccess.addSale')}
            </Button>
            <Button
              leftIcon={<FiShoppingCart />}
              colorScheme="orange"
              variant="outline"
              onClick={() => router.push('/purchases')}
              size="md"
            >
              {t('dashboard.quickAccess.addPurchase')}
            </Button>
            <Button
              leftIcon={<FiBarChart2 />}
              colorScheme="purple"
              variant="outline"
              onClick={() => router.push('/reports')}
              size="md"
            >
              {t('dashboard.quickAccess.financialReports')}
            </Button>
          </HStack>
        </CardBody>
      </Card>
    </Box>
  );
};
