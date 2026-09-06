'use client';

import React from 'react';
import { useRouter } from 'next/navigation';
import {
  Box,
  SimpleGrid,
  Card,
  CardHeader,
  CardBody,
  Heading,
  Text,
  Stat,
  StatLabel,
  StatNumber,
  StatHelpText,
  StatArrow,
  Flex,
  Icon,
  Button,
  HStack,
  useColorMode,
  useColorModeValue,
} from '@chakra-ui/react';
import {
  FiTrendingUp,
  FiTrendingDown,
  FiDollarSign,
  FiShoppingCart,
  FiActivity,
  FiBarChart2,
  FiPieChart,
  FiUsers,
  FiPlus,
} from 'react-icons/fi';
import AutoFitText from '@/components/common/AutoFitText';
import { useTranslation } from '@/hooks/useTranslation';
import {
  LineChart,
  Line,
  BarChart,
  Bar,
  PieChart as RechartsPieChart,
  Pie,
  Cell,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer
} from 'recharts';

// This would be passed as a prop from the main dashboard page
interface DashboardAnalytics {
  totalSales: number;
  totalPurchases: number;
  accountsReceivable: number;
  accountsPayable: number;
  
  // Growth percentages
  salesGrowth: number;
  purchasesGrowth: number;
  receivablesGrowth: number;
  payablesGrowth: number;
  
  monthlySales: { month: string; value: number }[];
  monthlyPurchases: { month: string; value: number }[];
  cashFlow: { month: string; inflow: number; outflow: number; balance: number }[];
  topAccounts: { name: string; balance: number; type: string }[];
  recentTransactions: any[]; // Define a proper type later
}

interface AdminDashboardProps {
  analytics: DashboardAnalytics | null;
}

interface StatCardProps {
  icon: React.ElementType;
  title: string;
  stat: string;
  change: string;
  changeType: 'increase' | 'decrease';
}

const StatCard: React.FC<StatCardProps> = ({ icon, title, stat, change, changeType }) => {
  const labelColor = useColorModeValue('gray.500', 'var(--text-secondary)');
  const numberColor = useColorModeValue('gray.800', 'var(--text-primary)');
  const iconBgColor = useColorModeValue(
    `${changeType === 'increase' ? 'green' : 'red'}.100`,
    `${changeType === 'increase' ? 'green' : 'red'}.900`
  );
  const iconColor = useColorModeValue(
    `${changeType === 'increase' ? 'green' : 'red'}.500`,
    `${changeType === 'increase' ? 'green' : 'red'}.300`
  );

  return (
    <Card className="card" p={5} minH="130px">
      <CardHeader display="flex" flexDirection="row" alignItems="center" justifyContent="space-between" pb={2}>
        {/* Stat content fills remaining space and truncates long text safely */}
        <Stat flex="1" minW={0} overflow="hidden">
          <StatLabel color={labelColor} noOfLines={1} title={title}>{title}</StatLabel>
          <Box mt={1}>
            <AutoFitText 
              value={stat as string}
              maxFontSize={24}
              minFontSize={14}
              fontWeight={700}
              color={numberColor as string}
              title={typeof stat === 'string' ? stat : ''}
              style={{ lineHeight: 1.1 }}
            />
          </Box>
          <StatHelpText noOfLines={1}>
            <StatArrow type={changeType === 'increase' ? 'increase' : 'decrease'} />
            {change}
          </StatHelpText>
        </Stat>
        <Flex
          w={10}
          h={10}
          align="center"
          justify="center"
          borderRadius="full"
          bg={iconBgColor}
          transition="all 0.3s ease"
          ml={4}
          flexShrink={0}
        >
          <Icon as={icon} color={iconColor} w={5} h={5} />
        </Flex>
      </CardHeader>
    </Card>
  );
};

export const AdminDashboard: React.FC<AdminDashboardProps> = ({ analytics }) => {
  const router = useRouter();
  const { colorMode } = useColorMode();
  const { t } = useTranslation();
  
  // Dynamic colors for charts based on theme
  const chartColors = {
    primary: colorMode === 'dark' ? '#4dabf7' : '#2196F3',
    secondary: colorMode === 'dark' ? '#51cf66' : '#28a745',
    tertiary: colorMode === 'dark' ? '#ffd43b' : '#ffc107',
    quaternary: colorMode === 'dark' ? '#ff6b6b' : '#dc3545',
    background: colorMode === 'dark' ? 'var(--bg-secondary)' : 'white',
    gridColor: colorMode === 'dark' ? '#495057' : '#e0e0e0',
    textColor: colorMode === 'dark' ? 'var(--text-primary)' : '#333333',
  };

  // Thousands separator formatter for axes (e.g., 1000000 -> 1.000.000)
  const formatThousands = (value: number) => new Intl.NumberFormat('id-ID').format(Number(value) || 0);
  
  if (!analytics) {
    return <Box color={colorMode === 'dark' ? 'var(--text-primary)' : 'gray.800'}>{t('dashboard.loading')}</Box>;
  }

  const formatCurrency = (value: number) =>
    new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR' }).format(value);
  
  // Format growth percentage for display
  const formatGrowthPercentage = (growth: number) => {
    const absGrowth = Math.abs(growth);
    const formatted = absGrowth.toFixed(1);
    return growth >= 0 ? `+${formatted}%` : `-${formatted}%`;
  };
  
  // Get growth type for styling
  const getGrowthType = (growth: number) => growth >= 0 ? 'increase' : 'decrease';
  
  // Dynamic chart colors based on theme
  const COLORS = colorMode === 'dark' 
    ? ['#4dabf7', '#51cf66', '#ffd43b', '#ff6b6b', '#9775fa']
    : ['#0088FE', '#00C49F', '#FFBB28', '#FF8042', '#A28bF4'];

  // Format data for charts
  const salesPurchaseData = analytics.monthlySales?.map((sale, index) => ({
    month: sale.month,
    sales: sale.value,
    purchases: analytics.monthlyPurchases?.[index]?.value || 0,
  })) || [];

  const topAccountsData = analytics.topAccounts?.map((account, index) => ({
    name: account.name,
    value: account.balance,
    fill: COLORS[index % COLORS.length],
  })) || [];

  return (
    <Box>
      <SimpleGrid columns={{ base: 1, md: 2, lg: 4 }} spacing={6} mb={6}>
        <StatCard
          icon={FiDollarSign}
          title={t('dashboard.stats.totalRevenue')}
          stat={formatCurrency(analytics.totalSales || 0)}
          change={formatGrowthPercentage(analytics.salesGrowth || 0)}
          changeType={getGrowthType(analytics.salesGrowth || 0)}
        />
        <StatCard
          icon={FiShoppingCart}
          title={t('dashboard.stats.totalPurchases')}
          stat={formatCurrency(analytics.totalPurchases || 0)}
          change={formatGrowthPercentage(analytics.purchasesGrowth || 0)}
          changeType={getGrowthType(analytics.purchasesGrowth || 0)}
        />
        <StatCard
          icon={FiTrendingUp}
          title={t('dashboard.stats.accountsReceivable')}
          stat={formatCurrency(analytics.accountsReceivable || 0)}
          change={formatGrowthPercentage(analytics.receivablesGrowth || 0)}
          changeType={getGrowthType(analytics.receivablesGrowth || 0)}
        />
        <StatCard
          icon={FiTrendingDown}
          title={t('dashboard.stats.accountsPayable')}
          stat={formatCurrency(analytics.accountsPayable || 0)}
          change={formatGrowthPercentage(analytics.payablesGrowth || 0)}
          changeType={getGrowthType(analytics.payablesGrowth || 0)}
        />
      </SimpleGrid>

      {/* Quick Access Section */}
      <Card mb={6}>
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
              leftIcon={<FiTrendingUp />}
              colorScheme="blue"
              variant="outline"
              onClick={() => router.push('/cash-bank')}
              size="md"
            >
              {t('dashboard.quickAccess.manageCashBank')}
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

      <SimpleGrid columns={{ base: 1, lg: 2 }} spacing={6}>
        <Card>
          <CardHeader>
            <Heading size="md" display="flex" alignItems="center">
              <Icon as={FiActivity} mr={2} color="blue.500" />
              {t('dashboard.charts.salesPurchaseOverview')}
            </Heading>
          </CardHeader>
          <CardBody>
            <ResponsiveContainer width="100%" height={300}>
              <LineChart data={salesPurchaseData} margin={{ top: 5, right: 30, left: 20, bottom: 5 }}>
                <CartesianGrid strokeDasharray="3 3" stroke={chartColors.gridColor} />
                <XAxis 
                  dataKey="month" 
                  tick={{ fill: chartColors.textColor, fontSize: 12 }}
                  axisLine={{ stroke: chartColors.gridColor }}
                />
                <YAxis 
                  width="auto"
                  tickMargin={8}
                  tick={{ fill: chartColors.textColor, fontSize: 12 }}
                  axisLine={{ stroke: chartColors.gridColor }}
                  tickFormatter={formatThousands}
                />
                <Tooltip 
                  contentStyle={{
                    backgroundColor: chartColors.background,
                    border: `1px solid ${chartColors.gridColor}`,
                    borderRadius: '8px',
                    color: chartColors.textColor,
                  }}
                  formatter={(value: number, name: string) => [
                    formatCurrency(value),
                    name === 'sales' ? t('dashboard.charts.sales') : t('dashboard.charts.purchases')
                  ]}
                />
                <Legend wrapperStyle={{ color: chartColors.textColor }} />
                <Line type="monotone" dataKey="sales" stroke={chartColors.primary} activeDot={{ r: 8 }} strokeWidth={2} name={t('dashboard.charts.sales')} />
                <Line type="monotone" dataKey="purchases" stroke={chartColors.secondary} strokeWidth={2} name={t('dashboard.charts.purchases')} />
              </LineChart>
            </ResponsiveContainer>
          </CardBody>
        </Card>

        <Card>
          <CardHeader>
            <Heading size="md" display="flex" alignItems="center">
              <Icon as={FiPieChart} mr={2} color="green.500" />
              {t('dashboard.charts.topAccounts')}
            </Heading>
          </CardHeader>
          <CardBody>
            <ResponsiveContainer width="100%" height={300}>
              <RechartsPieChart>
                <Pie 
                  data={topAccountsData} 
                  innerRadius={60} 
                  outerRadius={80} 
                  fill={chartColors.primary} 
                  dataKey="value" 
                  label={({ value }) => formatThousands(value as number)}
                  labelLine={false}
                >
                  {
                    topAccountsData.map((entry, index) => (
                      <Cell 
                        key={`cell-${index}`} 
                        fill={colorMode === 'dark' ? COLORS[index % COLORS.length] : entry.fill} 
                      />
                    ))
                  }
                </Pie>
                <Tooltip 
                  contentStyle={{
                    backgroundColor: chartColors.background,
                    border: `1px solid ${chartColors.gridColor}`,
                    borderRadius: '8px',
                    color: chartColors.textColor,
                  }}
                  formatter={(value: number) => [formatCurrency(value), t('common.labels.balance')]}
                />
                <Legend wrapperStyle={{ color: chartColors.textColor }} />
              </RechartsPieChart>
            </ResponsiveContainer>
          </CardBody>
        </Card>
      </SimpleGrid>

      {/* Cash Flow Chart */}
      <Card mt={6}>
        <CardHeader>
          <Heading size="md" display="flex" alignItems="center">
            <Icon as={FiBarChart2} mr={2} color="purple.500" />
            {t('dashboard.charts.monthlyCashFlow')}
          </Heading>
        </CardHeader>
        <CardBody>
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={analytics.cashFlow || []} margin={{ top: 5, right: 30, left: 20, bottom: 5 }}>
              <CartesianGrid strokeDasharray="3 3" stroke={chartColors.gridColor} />
              <XAxis 
                dataKey="month" 
                tick={{ fill: chartColors.textColor, fontSize: 12 }}
                axisLine={{ stroke: chartColors.gridColor }}
              />
              <YAxis 
                width="auto"
                tickMargin={8}
                tick={{ fill: chartColors.textColor, fontSize: 12 }}
                axisLine={{ stroke: chartColors.gridColor }}
                tickFormatter={formatThousands}
              />
              <Tooltip 
                contentStyle={{
                  backgroundColor: chartColors.background,
                  border: `1px solid ${chartColors.gridColor}`,
                  borderRadius: '8px',
                  color: chartColors.textColor,
                }}
                formatter={(value: number, name: string) => [
                  formatCurrency(value),
                  name === 'inflow' ? t('dashboard.charts.inflow') : 
                  name === 'outflow' ? t('dashboard.charts.outflow') : t('dashboard.charts.balance')
                ]}
              />
              <Legend wrapperStyle={{ color: chartColors.textColor }} />
              <Bar dataKey="inflow" fill={chartColors.secondary} name={t('dashboard.charts.inflow')} />
              <Bar dataKey="outflow" fill={chartColors.quaternary} name={t('dashboard.charts.outflow')} />
              <Bar dataKey="balance" fill={chartColors.primary} name={t('dashboard.charts.balance')} />
            </BarChart>
          </ResponsiveContainer>
        </CardBody>
      </Card>
    </Box>
  );
};
