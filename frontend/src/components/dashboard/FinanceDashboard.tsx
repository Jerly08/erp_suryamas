'use client';

import React, { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import {
  Alert,
  AlertIcon,
  Badge,
  Box,
  Button,
  Card,
  CardBody,
  CardHeader,
  Flex,
  Grid,
  GridItem,
  Heading,
  HStack,
  Icon,
  Progress,
  SimpleGrid,
  Spinner,
  Stat,
  StatHelpText,
  StatLabel,
  StatNumber,
  Text,
  VStack,
  useColorMode,
  useColorModeValue,
} from '@chakra-ui/react';
import {
  FiActivity,
  FiAlertCircle,
  FiBarChart2,
  FiDollarSign,
  FiPlus,
  FiShoppingCart,
  FiTrendingDown,
  FiTrendingUp,
} from 'react-icons/fi';
import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
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
  cash_bank_balance: number;
}

interface DashboardAnalytics {
  totalSales: number;
  totalPurchases: number;
  accountsReceivable: number;
  accountsPayable: number;
  salesGrowth: number;
  purchasesGrowth: number;
  receivablesGrowth: number;
  payablesGrowth: number;
  monthlySales: { month: string; value: number }[];
  monthlyPurchases: { month: string; value: number }[];
  cashFlow: { month: string; inflow: number; outflow: number; balance: number }[];
  topAccounts: { name: string; balance: number; type: string }[];
  recentTransactions: any[];
}

interface FinanceDashboardProps {
  analytics: DashboardAnalytics | null;
}

interface FinanceStatCardProps {
  title: string;
  value: string;
  helper: string;
  icon: React.ElementType;
  accentColor: string;
}

const FinanceStatCard: React.FC<FinanceStatCardProps> = ({
  title,
  value,
  helper,
  icon,
  accentColor,
}) => {
  const cardBg = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const iconBg = useColorModeValue(`${accentColor}.50`, `${accentColor}.900`);
  const mutedTextColor = useColorModeValue('gray.500', 'whiteAlpha.700');

  return (
    <Card bg={cardBg} border="1px" borderColor={borderColor} boxShadow="sm" h="100%">
      <CardBody>
        <Flex justify="space-between" align="start" gap={4}>
          <Stat>
            <StatLabel color={mutedTextColor} fontSize="sm">
              {title}
            </StatLabel>
            <StatNumber fontSize="2xl">{value}</StatNumber>
            <StatHelpText mb={0}>{helper}</StatHelpText>
          </Stat>
          <Flex
            align="center"
            justify="center"
            w={11}
            h={11}
            borderRadius="xl"
            bg={iconBg}
            color={`${accentColor}.500`}
            flexShrink={0}
          >
            <Icon as={icon} boxSize={5} />
          </Flex>
        </Flex>
      </CardBody>
    </Card>
  );
};

export const FinanceDashboard: React.FC<FinanceDashboardProps> = ({ analytics }) => {
  const router = useRouter();
  const { t } = useTranslation();
  const { colorMode } = useColorMode();
  const [data, setData] = useState<FinanceDashboardData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const headingColor = useColorModeValue('gray.800', 'whiteAlpha.900');
  const mutedTextColor = useColorModeValue('gray.500', 'whiteAlpha.700');
  const cardBg = useColorModeValue('white', 'gray.800');
  const cardBorderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const panelBg = useColorModeValue('gray.50', 'whiteAlpha.50');

  const chartColors = {
    receivable: colorMode === 'dark' ? '#f6ad55' : '#dd6b20',
    payable: colorMode === 'dark' ? '#fc8181' : '#e53e3e',
    cash: colorMode === 'dark' ? '#68d391' : '#38a169',
    sales: colorMode === 'dark' ? '#63b3ed' : '#3182ce',
    purchase: colorMode === 'dark' ? '#f6ad55' : '#dd6b20',
    balance: colorMode === 'dark' ? '#9f7aea' : '#6b46c1',
    inflow: colorMode === 'dark' ? '#4fd1c5' : '#319795',
    outflow: colorMode === 'dark' ? '#fc8181' : '#e53e3e',
    grid: colorMode === 'dark' ? '#2d3748' : '#e2e8f0',
    text: colorMode === 'dark' ? '#e2e8f0' : '#4a5568',
  };

  useEffect(() => {
    const fetchFinanceDashboardData = async () => {
      try {
        setLoading(true);
        const response = await api.get(API_ENDPOINTS.DASHBOARD_FINANCE);
        const d = response.data.data || {};
        setData({
          invoices_pending_payment: d.invoices_pending_payment ?? 0,
          invoices_not_paid: d.invoices_not_paid ?? 0,
          journals_need_posting: d.journals_need_posting ?? 0,
          outstanding_receivables: d.outstanding_receivables ?? 0,
          outstanding_payables: d.outstanding_payables ?? 0,
          purchases_pending_approval: d.purchases_pending_approval ?? 0,
          sales_pending_approval: d.sales_pending_approval ?? 0,
          cash_bank_balance: d.cash_bank_balance ?? 0,
        });
      } catch (fetchError: any) {
        console.error('Error fetching finance dashboard data:', fetchError);
        setError(fetchError.response?.data?.error || t('messages.operationFailed'));
      } finally {
        setLoading(false);
      }
    };

    fetchFinanceDashboardData();
  }, [t]);

  const formatCurrency = (amount: number) =>
    new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(amount || 0);

  const formatCompactCurrency = (amount: number) => {
    const value = Math.abs(amount || 0);
    if (value >= 1_000_000_000) return `Rp ${(amount / 1_000_000_000).toFixed(1)}B`;
    if (value >= 1_000_000) return `Rp ${(amount / 1_000_000).toFixed(1)}M`;
    if (value >= 1_000) return `Rp ${(amount / 1_000).toFixed(1)}K`;
    return `Rp ${Math.round(amount || 0)}`;
  };

  const queueTotal = data
    ? data.journals_need_posting +
      data.purchases_pending_approval +
      data.sales_pending_approval +
      data.invoices_pending_payment +
      data.invoices_not_paid
    : 0;

  const liquidityCoverage = data && data.outstanding_payables > 0
    ? (data.cash_bank_balance / data.outstanding_payables) * 100
    : data && data.cash_bank_balance > 0
      ? 100
      : 0;

  const monthlySeriesByMonth = new Map<string, { month: string; sales: number; purchases: number }>();

  analytics?.monthlySales?.forEach((sale) => {
    monthlySeriesByMonth.set(sale.month, {
      month: sale.month,
      sales: sale.value,
      purchases: monthlySeriesByMonth.get(sale.month)?.purchases || 0,
    });
  });

  analytics?.monthlyPurchases?.forEach((purchase) => {
    monthlySeriesByMonth.set(purchase.month, {
      month: purchase.month,
      sales: monthlySeriesByMonth.get(purchase.month)?.sales || 0,
      purchases: purchase.value,
    });
  });

  const monthlyPerformanceData = Array.from(monthlySeriesByMonth.values());

  const cashFlowTrendData = analytics?.cashFlow?.map((item) => ({
    month: item.month,
    inflow: item.inflow,
    outflow: item.outflow,
    balance: item.balance,
  })) || [];

  if (loading) {
    return (
      <Box>
        <Heading as="h2" size="xl" mb={6} color={headingColor}>
          {t('navigation.dashboard')} - {t('users.finance')}
        </Heading>
        <Flex justify="center" align="center" h="220px">
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
      <Heading as="h2" size="xl" mb={2} color={headingColor}>
        {t('navigation.dashboard')} - {t('users.finance')}
      </Heading>
      <Text color={mutedTextColor} mb={6}>
        Finance manager view for liquidity, approved AR/AP exposure, approval backlog, and monthly cash movement.
      </Text>

      <SimpleGrid columns={{ base: 1, md: 2, xl: 4 }} spacing={4} mb={6}>
        <FinanceStatCard
          title="Cash & Bank Position"
          value={formatCurrency(data.cash_bank_balance)}
          helper="Available liquidity across cash and bank accounts"
          icon={FiDollarSign}
          accentColor="green"
        />
        <FinanceStatCard
          title="Outstanding Receivables"
          value={formatCurrency(data.outstanding_receivables)}
          helper={`${data.invoices_pending_payment} invoices require collection follow-up`}
          icon={FiTrendingUp}
          accentColor="orange"
        />
        <FinanceStatCard
          title="Outstanding Payables"
          value={formatCurrency(data.outstanding_payables)}
          helper={`${data.invoices_not_paid} vendor bills still unpaid`}
          icon={FiTrendingDown}
          accentColor="red"
        />
        <FinanceStatCard
          title="Operational Backlog"
          value={`${queueTotal}`}
          helper={`${data.journals_need_posting} journals and ${data.purchases_pending_approval + data.sales_pending_approval} approvals waiting`}
          icon={FiActivity}
          accentColor="blue"
        />
      </SimpleGrid>

      <Grid templateColumns={{ base: '1fr', xl: '1fr 1fr' }} gap={6} mb={6}>
        <GridItem>
          <Card bg={cardBg} border="1px" borderColor={cardBorderColor} boxShadow="sm" h="100%">
            <CardHeader pb={2}>
              <HStack justify="space-between" align="start">
                <Box>
                  <Heading size="md" color={headingColor}>Monthly Sales vs Purchases</Heading>
                  <Text fontSize="sm" color={mutedTextColor}>
                    Track recognized sales and approved purchases month by month.
                  </Text>
                </Box>
                <Icon as={FiBarChart2} color="blue.500" boxSize={5} />
              </HStack>
            </CardHeader>
            <CardBody>
              <Box h="320px">
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={monthlyPerformanceData}>
                    <CartesianGrid strokeDasharray="3 3" stroke={chartColors.grid} />
                    <XAxis dataKey="month" stroke={chartColors.text} />
                    <YAxis stroke={chartColors.text} tickFormatter={formatCompactCurrency} />
                    <Tooltip formatter={(value: number) => formatCurrency(value)} labelStyle={{ color: '#1a202c' }} />
                    <Legend />
                    <Line type="monotone" dataKey="sales" name="Sales" stroke={chartColors.sales} strokeWidth={3} dot={false} />
                    <Line type="monotone" dataKey="purchases" name="Purchases" stroke={chartColors.purchase} strokeWidth={3} dot={false} />
                  </LineChart>
                </ResponsiveContainer>
              </Box>
            </CardBody>
          </Card>
        </GridItem>

        <GridItem>
          <Card bg={cardBg} border="1px" borderColor={cardBorderColor} boxShadow="sm" h="100%">
            <CardHeader pb={2}>
              <HStack justify="space-between" align="start">
                <Box>
                  <Heading size="md" color={headingColor}>Cash Movement Trend</Heading>
                  <Text fontSize="sm" color={mutedTextColor}>
                    Inflow, outflow, and net movement from recorded cash and bank transactions.
                  </Text>
                </Box>
                <Icon as={FiTrendingUp} color="green.500" boxSize={5} />
              </HStack>
            </CardHeader>
            <CardBody>
              <Box h="320px">
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={cashFlowTrendData}>
                    <CartesianGrid strokeDasharray="3 3" stroke={chartColors.grid} />
                    <XAxis dataKey="month" stroke={chartColors.text} />
                    <YAxis stroke={chartColors.text} tickFormatter={formatCompactCurrency} />
                    <Tooltip formatter={(value: number) => formatCurrency(value)} labelStyle={{ color: '#1a202c' }} />
                    <Legend />
                    <Line type="monotone" dataKey="inflow" name="Cash In" stroke={chartColors.inflow} strokeWidth={3} dot={false} />
                    <Line type="monotone" dataKey="outflow" name="Cash Out" stroke={chartColors.outflow} strokeWidth={3} dot={false} />
                    <Line type="monotone" dataKey="balance" name="Net Movement" stroke={chartColors.balance} strokeWidth={3} dot={false} />
                  </LineChart>
                </ResponsiveContainer>
              </Box>
            </CardBody>
          </Card>
        </GridItem>
      </Grid>

      <Grid templateColumns={{ base: '1fr', xl: '1fr 1fr' }} gap={6} mb={6}>
        <GridItem>
          <Card bg={cardBg} border="1px" borderColor={cardBorderColor} boxShadow="sm" h="100%">
            <CardHeader pb={2}>
              <HStack justify="space-between" align="start">
                <Box>
                  <Heading size="md" color={headingColor}>Liquidity Snapshot</Heading>
                  <Text fontSize="sm" color={mutedTextColor}>
                    Quick read on cash coverage and working capital.
                  </Text>
                </Box>
              </HStack>
            </CardHeader>
            <CardBody>
              <VStack align="stretch" spacing={4}>
                <Box bg={panelBg} borderRadius="lg" p={4}>
                  <HStack justify="space-between" mb={2}>
                    <Text fontSize="sm" fontWeight="medium" color={headingColor}>
                      Cash coverage to payables
                    </Text>
                    <Text fontSize="sm" color={mutedTextColor}>
                      {Math.min(liquidityCoverage, 999).toFixed(0)}%
                    </Text>
                  </HStack>
                  <Progress
                    value={Math.min(liquidityCoverage, 100)}
                    colorScheme={liquidityCoverage >= 100 ? 'green' : liquidityCoverage >= 70 ? 'orange' : 'red'}
                    borderRadius="full"
                    size="sm"
                  />
                </Box>
                <SimpleGrid columns={3} spacing={3}>
                  <Box bg={panelBg} borderRadius="lg" p={3}>
                    <Text fontSize="xs" color={mutedTextColor}>Cash</Text>
                    <Text fontWeight="bold" color={`${chartColors.cash}`}>{formatCompactCurrency(data.cash_bank_balance)}</Text>
                  </Box>
                  <Box bg={panelBg} borderRadius="lg" p={3}>
                    <Text fontSize="xs" color={mutedTextColor}>AR</Text>
                    <Text fontWeight="bold" color={`${chartColors.receivable}`}>{formatCompactCurrency(data.outstanding_receivables)}</Text>
                  </Box>
                  <Box bg={panelBg} borderRadius="lg" p={3}>
                    <Text fontSize="xs" color={mutedTextColor}>AP</Text>
                    <Text fontWeight="bold" color={`${chartColors.payable}`}>{formatCompactCurrency(data.outstanding_payables)}</Text>
                  </Box>
                </SimpleGrid>
                <Text fontSize="sm" color={mutedTextColor}>
                  Working capital gap: {formatCurrency(data.outstanding_receivables - data.outstanding_payables)}
                </Text>
              </VStack>
            </CardBody>
          </Card>
        </GridItem>

        <GridItem>
          <Card bg={cardBg} border="1px" borderColor={cardBorderColor} boxShadow="sm" h="100%">
            <CardHeader pb={2}>
              <HStack justify="space-between" align="start">
                <Box>
                  <Heading size="md" color={headingColor}>Finance Action Queue</Heading>
                  <Text fontSize="sm" color={mutedTextColor}>
                    Focus list for month-end and daily finance operations.
                  </Text>
                </Box>
                <Icon as={FiAlertCircle} color="orange.500" boxSize={5} />
              </HStack>
            </CardHeader>
            <CardBody>
              <VStack align="stretch" spacing={3}>
                {[
                  { label: 'Receivable Follow-up', value: data.invoices_pending_payment, color: 'orange.500' },
                  { label: 'Payable Settlement', value: data.invoices_not_paid, color: 'red.500' },
                  { label: 'Journal Posting', value: data.journals_need_posting, color: 'blue.500' },
                  { label: 'Purchase Approval', value: data.purchases_pending_approval, color: 'purple.500' },
                  { label: 'Sales Approval', value: data.sales_pending_approval, color: 'teal.500' },
                ].map((item) => (
                  <Flex
                    key={item.label}
                    justify="space-between"
                    align="center"
                    bg={panelBg}
                    borderRadius="lg"
                    px={4}
                    py={3}
                  >
                    <Text fontSize="sm" color={headingColor}>{item.label}</Text>
                    <Badge colorScheme="gray" px={2.5} py={1} borderRadius="full" color={item.color}>
                      {item.value} items
                    </Badge>
                  </Flex>
                ))}
              </VStack>
            </CardBody>
          </Card>
        </GridItem>
      </Grid>

      <Card mb={6} bg={cardBg} border="1px" borderColor={cardBorderColor} boxShadow="sm">
        <CardHeader>
          <Heading size="md" display="flex" alignItems="center" color={headingColor}>
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
              leftIcon={<FiActivity />}
              colorScheme="blue"
              variant="outline"
              onClick={() => router.push('/journal')}
              size="md"
            >
              Review Journals
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
