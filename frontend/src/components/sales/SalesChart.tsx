'use client';

import React, { useState } from 'react';
import {
  Box,
  Card,
  CardHeader,
  CardBody,
  Heading,
  Text,
  VStack,
  HStack,
  Button,
  ButtonGroup,
  SimpleGrid,
  Flex,
  Badge,
  Select,
  useColorModeValue
} from '@chakra-ui/react';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
  BarChart,
  Bar,
  Area,
  AreaChart,
  PieChart,
  Pie,
  Cell
} from 'recharts';
import { SalesAnalytics } from '@/services/salesService';
import { formatIDR } from '@/utils/currency';

interface SalesChartProps {
  analytics: SalesAnalytics | null;
}

const SalesChart: React.FC<SalesChartProps> = ({ analytics }) => {
  const [chartType, setChartType] = useState<'line' | 'bar' | 'area'>('line');
  const [metric, setMetric] = useState<'amount' | 'sales' | 'growth'>('amount');

  // Chart colors
  const primaryColor = useColorModeValue('#3182ce', '#63b3ed');
  const secondaryColor = useColorModeValue('#38a169', '#68d391');
  const gridColor = useColorModeValue('#e2e8f0', '#4a5568');
  const textColor = useColorModeValue('#2d3748', '#e2e8f0');
  const tooltipBg = useColorModeValue('white', 'gray.700');
  const trendItemBg = useColorModeValue('gray.50', 'gray.700');

  const pieColors = ['#3182ce', '#38a169', '#ed8936', '#e53e3e', '#805ad5', '#d69e2e'];

  if (!analytics || !analytics.data || analytics.data.length === 0) {
    return (
      <Card>
        <CardBody>
          <Box textAlign="center" py={10}>
            <Text color="gray.500" fontSize="lg">
              No analytics data available
            </Text>
            <Text color="gray.400" fontSize="sm" mt={2}>
              Sales data will appear here once transactions are recorded
            </Text>
          </Box>
        </CardBody>
      </Card>
    );
  }

  const formatCurrency = (value: number) => {
    return formatIDR(value, { minimumFractionDigits: 0, maximumFractionDigits: 0, showSymbol: true });
  };

  const formatNumber = (value: number) => {
    return formatIDR(value, { showSymbol: false, minimumFractionDigits: 0, maximumFractionDigits: 0 });
  };

  // Prepare chart data
  const chartData = analytics.data.map(item => ({
    period: item.period,
    sales: item.total_sales,
    amount: item.total_amount,
    growth: item.growth_rate,
    formattedAmount: formatCurrency(item.total_amount),
    formattedSales: formatNumber(item.total_sales),
    formattedGrowth: `${item.growth_rate.toFixed(1)}%`
  }));

  // Calculate summary statistics
  const totalSales = analytics.data.reduce((sum, item) => sum + item.total_sales, 0);
  const totalAmount = analytics.data.reduce((sum, item) => sum + item.total_amount, 0);
  const avgGrowth = analytics.data.length > 0 
    ? analytics.data.reduce((sum, item) => sum + item.growth_rate, 0) / analytics.data.length 
    : 0;

  // Data for pie chart (last 6 periods)
  const pieData = analytics.data.slice(-6).map((item, index) => ({
    name: item.period,
    value: item.total_amount,
    fill: pieColors[index % pieColors.length]
  }));

  const yAxisTickFormatter = (value: number) => {
    if (metric === 'amount') return formatNumber(value);
    if (metric === 'sales') return formatNumber(value);
    // growth is already in percentage units (e.g., 5 means 5%)
    return `${Number(value).toFixed(0)}%`;
  };

  const renderChart = () => {
    const dataKey = metric === 'amount' ? 'amount' : metric === 'sales' ? 'sales' : 'growth';
    const color = metric === 'growth' ? secondaryColor : primaryColor;

    switch (chartType) {
      case 'bar':
        return (
          <ResponsiveContainer width="100%" height={400}>
            <BarChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" stroke={gridColor} />
              <XAxis dataKey="period" tick={{ fill: textColor, fontSize: 12 }} />
              <YAxis tick={{ fill: textColor, fontSize: 12 }} tickFormatter={yAxisTickFormatter} />
              <Tooltip 
                contentStyle={{ 
                  backgroundColor: tooltipBg,
                  border: `1px solid ${gridColor}`,
                  borderRadius: '8px'
                }}
                formatter={(value: any) => {
                  if (metric === 'amount') return [formatCurrency(value), 'Revenue'];
                  if (metric === 'sales') return [formatNumber(value), 'Sales Count'];
                  return [`${Number(value).toFixed(1)}%`, 'Growth Rate'];
                }}
              />
              <Legend />
              <Bar 
                dataKey={dataKey} 
                fill={color} 
                radius={[4, 4, 0, 0]}
                name={metric === 'amount' ? 'Revenue' : metric === 'sales' ? 'Sales' : 'Growth Rate'}
              />
            </BarChart>
          </ResponsiveContainer>
        );
      case 'area':
        return (
          <ResponsiveContainer width="100%" height={400}>
            <AreaChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" stroke={gridColor} />
              <XAxis dataKey="period" tick={{ fill: textColor, fontSize: 12 }} />
              <YAxis tick={{ fill: textColor, fontSize: 12 }} tickFormatter={yAxisTickFormatter} />
              <Tooltip 
                contentStyle={{ 
                  backgroundColor: tooltipBg,
                  border: `1px solid ${gridColor}`,
                  borderRadius: '8px'
                }}
                formatter={(value: any) => {
                  if (metric === 'amount') return [formatCurrency(value), 'Revenue'];
                  if (metric === 'sales') return [formatNumber(value), 'Sales Count'];
                  return [`${Number(value).toFixed(1)}%`, 'Growth Rate'];
                }}
              />
              <Legend />
              <Area 
                type="monotone" 
                dataKey={dataKey} 
                stroke={color} 
                fill={`${color}33`}
                name={metric === 'amount' ? 'Revenue' : metric === 'sales' ? 'Sales' : 'Growth Rate'}
              />
            </AreaChart>
          </ResponsiveContainer>
        );
      default: // line
        return (
          <ResponsiveContainer width="100%" height={400}>
            <LineChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" stroke={gridColor} />
              <XAxis dataKey="period" tick={{ fill: textColor, fontSize: 12 }} />
              <YAxis tick={{ fill: textColor, fontSize: 12 }} tickFormatter={yAxisTickFormatter} />
              <Tooltip 
                contentStyle={{ 
                  backgroundColor: tooltipBg,
                  border: `1px solid ${gridColor}`,
                  borderRadius: '8px'
                }}
                formatter={(value: any) => {
                  if (metric === 'amount') return [formatCurrency(value), 'Revenue'];
                  if (metric === 'sales') return [formatNumber(value), 'Sales Count'];
                  return [`${Number(value).toFixed(1)}%`, 'Growth Rate'];
                }}
              />
              <Legend />
              <Line 
                type="monotone" 
                dataKey={dataKey} 
                stroke={color} 
                strokeWidth={3}
                dot={{ fill: color, strokeWidth: 2, r: 6 }}
                activeDot={{ r: 8, fill: color }}
                name={metric === 'amount' ? 'Revenue' : metric === 'sales' ? 'Sales' : 'Growth Rate'}
              />
            </LineChart>
          </ResponsiveContainer>
        );
    }
  };

  return (
    <VStack spacing={6} align="stretch">
      {/* Summary Cards */}
      <SimpleGrid columns={{ base: 1, md: 3 }} spacing={4}>
        <Card>
          <CardBody>
            <VStack align="start" spacing={2}>
              <Text fontSize="sm" color="gray.500" fontWeight="medium">
                Total Sales ({analytics.period})
              </Text>
              <Text fontSize="2xl" fontWeight="bold" color="blue.500">
                {formatNumber(totalSales)}
              </Text>
              <Badge colorScheme="blue" variant="subtle">
                Transactions
              </Badge>
            </VStack>
          </CardBody>
        </Card>

        <Card>
          <CardBody>
            <VStack align="start" spacing={2}>
              <Text fontSize="sm" color="gray.500" fontWeight="medium">
                Total Revenue ({analytics.period})
              </Text>
              <Text fontSize="2xl" fontWeight="bold" color="green.500">
                {formatCurrency(totalAmount)}
              </Text>
              <Badge colorScheme="green" variant="subtle">
                Revenue
              </Badge>
            </VStack>
          </CardBody>
        </Card>

        <Card>
          <CardBody>
            <VStack align="start" spacing={2}>
              <Text fontSize="sm" color="gray.500" fontWeight="medium">
                Average Growth Rate
              </Text>
              <Text fontSize="2xl" fontWeight="bold" color={avgGrowth >= 0 ? "green.500" : "red.500"}>
                {avgGrowth.toFixed(1)}%
              </Text>
              <Badge colorScheme={avgGrowth >= 0 ? "green" : "red"} variant="subtle">
                {avgGrowth >= 0 ? "Growth" : "Decline"}
              </Badge>
            </VStack>
          </CardBody>
        </Card>
      </SimpleGrid>

      {/* Main Chart */}
      <Card>
        <CardHeader>
          <Flex justify="space-between" align="center" wrap="wrap" gap={4}>
            <Heading size="md">
              Sales Analytics - {analytics.period.charAt(0).toUpperCase() + analytics.period.slice(1)}
            </Heading>
            
            <HStack spacing={4} wrap="wrap">
              <Select
                value={metric}
                onChange={(e) => setMetric(e.target.value as any)}
                width="auto"
                size="sm"
              >
                <option value="amount">Revenue</option>
                <option value="sales">Sales Count</option>
                <option value="growth">Growth Rate</option>
              </Select>

              <ButtonGroup size="sm" isAttached>
                <Button
                  variant={chartType === 'line' ? 'solid' : 'outline'}
                  onClick={() => setChartType('line')}
                  colorScheme="blue"
                >
                  Line
                </Button>
                <Button
                  variant={chartType === 'bar' ? 'solid' : 'outline'}
                  onClick={() => setChartType('bar')}
                  colorScheme="blue"
                >
                  Bar
                </Button>
                <Button
                  variant={chartType === 'area' ? 'solid' : 'outline'}
                  onClick={() => setChartType('area')}
                  colorScheme="blue"
                >
                  Area
                </Button>
              </ButtonGroup>
            </HStack>
          </Flex>
        </CardHeader>
        <CardBody>
          {renderChart()}
        </CardBody>
      </Card>

      {/* Additional Charts */}
      <SimpleGrid columns={{ base: 1, lg: 2 }} spacing={6}>
        {/* Revenue Distribution */}
        <Card>
          <CardHeader>
            <Heading size="md">Revenue Distribution (Last 6 Periods)</Heading>
          </CardHeader>
          <CardBody>
            <ResponsiveContainer width="100%" height={300}>
              <PieChart>
                <Pie
                  data={pieData}
                  cx="50%"
                  cy="50%"
                  innerRadius={60}
                  outerRadius={100}
                  paddingAngle={5}
                  dataKey="value"
                >
                  {pieData.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.fill} />
                  ))}
                </Pie>
                <Tooltip formatter={(value: any) => [formatCurrency(value), 'Revenue']} />
                <Legend />
              </PieChart>
            </ResponsiveContainer>
          </CardBody>
        </Card>

        {/* Trend Analysis */}
        <Card>
          <CardHeader>
            <Heading size="md">Trend Analysis</Heading>
          </CardHeader>
          <CardBody>
            <VStack align="stretch" spacing={4}>
              {analytics.data.slice(-5).map((item, index) => {
                const isPositive = item.growth_rate >= 0;
                return (
                  <Box key={item.period} p={4} borderRadius="md" bg={trendItemBg}>
                    <Flex justify="space-between" align="center">
                      <VStack align="start" spacing={1}>
                        <Text fontWeight="medium">{item.period}</Text>
                        <Text fontSize="sm" color="gray.500">
                          {formatNumber(item.total_sales)} sales
                        </Text>
                      </VStack>
                      <VStack align="end" spacing={1}>
                        <Text fontWeight="bold">{formatCurrency(item.total_amount)}</Text>
                        <Badge colorScheme={isPositive ? "green" : "red"} variant="subtle">
                          {isPositive ? "+" : ""}{item.growth_rate.toFixed(1)}%
                        </Badge>
                      </VStack>
                    </Flex>
                  </Box>
                );
              })}
            </VStack>
          </CardBody>
        </Card>
      </SimpleGrid>
    </VStack>
  );
};

export default SalesChart;
