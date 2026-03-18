import React, { useState, useEffect } from 'react';
import {
  Box,
  VStack,
  HStack,
  Text,
  Heading,
  Card,
  CardBody,
  CardHeader,
  SimpleGrid,
  Stat,
  StatLabel,
  StatNumber,
  StatHelpText,
  Badge,
  Alert,
  AlertIcon,
  Progress,
  CircularProgress,
  CircularProgressLabel,
  Icon,
  useToast,
  Spinner,
  Button,
  Divider,
} from '@chakra-ui/react';
import {
  FiTrendingUp,
  FiTrendingDown,
  FiDollarSign,
  FiActivity,
  FiAlertTriangle,
  FiRefreshCw,
  FiPieChart,
  FiBarChart,
} from 'react-icons/fi';
import financialReportService, {
  FinancialDashboard as FinancialDashboardData,
  RealTimeFinancialMetrics,
  FinancialHealthScore,
  FinancialRatios,
} from '../../services/financialReportService';

const FinancialDashboard: React.FC = () => {
  const [dashboardData, setDashboardData] = useState<FinancialDashboardData | null>(null);
  const [realTimeMetrics, setRealTimeMetrics] = useState<RealTimeFinancialMetrics | null>(null);
  const [healthScore, setHealthScore] = useState<FinancialHealthScore | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [lastRefresh, setLastRefresh] = useState<Date>(new Date());
  const toast = useToast();

  useEffect(() => {
    loadDashboardData();
    // Set up auto-refresh every 5 minutes
    const interval = setInterval(loadDashboardData, 5 * 60 * 1000);
    return () => clearInterval(interval);
  }, []);

  const loadDashboardData = async () => {
    setIsLoading(true);
    try {
      const [dashboard, metrics, health] = await Promise.all([
        financialReportService.getFinancialDashboard(),
        financialReportService.getRealTimeMetrics(),
        financialReportService.getFinancialHealthScore(),
      ]);

      setDashboardData(dashboard);
      setRealTimeMetrics(metrics);
      setHealthScore(health);
      setLastRefresh(new Date());
    } catch (error) {
      console.error('Failed to load dashboard data:', error);
      toast({
        title: 'Error',
        description: 'Failed to load dashboard data',
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setIsLoading(false);
    }
  };

  const refreshDashboard = () => {
    loadDashboardData();
  };

  if (isLoading && !dashboardData) {
    return (
      <Box textAlign="center" py={12}>
        <VStack spacing={4}>
          <Spinner size="xl" thickness="4px" speed="0.65s" color="blue.500" />
          <Text fontSize="lg" color="gray.600">Loading Financial Dashboard...</Text>
        </VStack>
      </Box>
    );
  }

  return (
    <VStack spacing={6} align="stretch">
      {/* Header */}
      <Card>
        <CardHeader>
          <HStack justify="space-between" align="center">
            <VStack align="start" spacing={1}>
              <Heading size="lg" display="flex" alignItems="center">
                <Icon as={FiPieChart} mr={2} />
                Financial Dashboard
              </Heading>
              <Text fontSize="sm" color="gray.500">
                Last updated: {lastRefresh.toLocaleString('id-ID')}
              </Text>
            </VStack>
            <Button
              size="sm"
              variant="outline"
              leftIcon={<Icon as={FiRefreshCw} />}
              onClick={refreshDashboard}
              isLoading={isLoading}
            >
              Refresh
            </Button>
          </HStack>
        </CardHeader>
      </Card>

      {/* Real-Time Metrics */}
      {realTimeMetrics && (
        <Card>
          <CardHeader>
            <Heading size="md" display="flex" alignItems="center">
              <Icon as={FiActivity} mr={2} />
              Real-Time Metrics
            </Heading>
          </CardHeader>
          <CardBody>
            <SimpleGrid columns={{ base: 2, md: 4, lg: 6 }} spacing={4}>
              <Stat>
                <StatLabel>Cash Position</StatLabel>
                <StatNumber color="blue.500">
                  {financialReportService.formatCurrency(realTimeMetrics.cashPosition)}
                </StatNumber>
              </Stat>
              
              <Stat>
                <StatLabel>Today Revenue</StatLabel>
                <StatNumber color="green.500">
                  {financialReportService.formatCurrency(realTimeMetrics.dailyRevenue)}
                </StatNumber>
                <StatHelpText>
                  Net: {financialReportService.formatCurrency(realTimeMetrics.dailyNetIncome)}
                </StatHelpText>
              </Stat>

              <Stat>
                <StatLabel>MTD Revenue</StatLabel>
                <StatNumber color="green.600">
                  {financialReportService.formatCurrency(realTimeMetrics.monthlyRevenue)}
                </StatNumber>
                <StatHelpText>
                  Net: {financialReportService.formatCurrency(realTimeMetrics.monthlyNetIncome)}
                </StatHelpText>
              </Stat>

              <Stat>
                <StatLabel>YTD Revenue</StatLabel>
                <StatNumber color="green.700">
                  {financialReportService.formatCurrency(realTimeMetrics.yearlyRevenue)}
                </StatNumber>
                <StatHelpText>
                  Net: {financialReportService.formatCurrency(realTimeMetrics.yearlyNetIncome)}
                </StatHelpText>
              </Stat>

              <Stat>
                <StatLabel>Receivables</StatLabel>
                <StatNumber color="orange.500">
                  {financialReportService.formatCurrency(realTimeMetrics.pendingReceivables)}
                </StatNumber>
              </Stat>

              <Stat>
                <StatLabel>Payables</StatLabel>
                <StatNumber color="red.500">
                  {financialReportService.formatCurrency(realTimeMetrics.pendingPayables)}
                </StatNumber>
              </Stat>
            </SimpleGrid>
          </CardBody>
        </Card>
      )}

      {/* Financial Health Score */}
      {healthScore && (
        <Card>
          <CardHeader>
            <Heading size="md" display="flex" alignItems="center">
              <Icon as={FiBarChart} mr={2} />
              Financial Health Score
            </Heading>
          </CardHeader>
          <CardBody>
            <SimpleGrid columns={{ base: 1, md: 2 }} spacing={6}>
              {/* Overall Score */}
              <VStack spacing={4}>
                <CircularProgress
                  value={healthScore.overallScore}
                  size="120px"
                  thickness="8px"
                  color={financialReportService.getHealthScoreColor(healthScore.overallScore)}
                >
                  <CircularProgressLabel fontSize="xl" fontWeight="bold">
                    {healthScore.overallScore.toFixed(0)}
                  </CircularProgressLabel>
                </CircularProgress>
                <VStack spacing={1}>
                  <Badge 
                    colorScheme={healthScore.scoreGrade.includes('A') ? 'green' : 
                               healthScore.scoreGrade.includes('B') ? 'blue' : 
                               healthScore.scoreGrade.includes('C') ? 'yellow' : 'red'}
                    fontSize="lg"
                    px={3}
                    py={1}
                  >
                    Grade: {healthScore.scoreGrade}
                  </Badge>
                  <Text fontSize="sm" color="gray.500" textAlign="center">
                    Overall Financial Health
                  </Text>
                </VStack>
              </VStack>

              {/* Component Scores */}
              <VStack spacing={3} align="stretch">
                <Text fontWeight="semibold" mb={2}>Component Breakdown</Text>
                
                <Box>
                  <HStack justify="space-between" mb={1}>
                    <Text fontSize="sm">Liquidity</Text>
                    <Text fontSize="sm" fontWeight="bold">
                      {healthScore.components.liquidityScore.toFixed(0)}%
                    </Text>
                  </HStack>
                  <Progress 
                    value={healthScore.components.liquidityScore} 
                    colorScheme="blue" 
                    size="sm" 
                    borderRadius="full" 
                  />
                </Box>

                <Box>
                  <HStack justify="space-between" mb={1}>
                    <Text fontSize="sm">Profitability</Text>
                    <Text fontSize="sm" fontWeight="bold">
                      {healthScore.components.profitabilityScore.toFixed(0)}%
                    </Text>
                  </HStack>
                  <Progress 
                    value={healthScore.components.profitabilityScore} 
                    colorScheme="green" 
                    size="sm" 
                    borderRadius="full" 
                  />
                </Box>

                <Box>
                  <HStack justify="space-between" mb={1}>
                    <Text fontSize="sm">Leverage</Text>
                    <Text fontSize="sm" fontWeight="bold">
                      {healthScore.components.leverageScore.toFixed(0)}%
                    </Text>
                  </HStack>
                  <Progress 
                    value={healthScore.components.leverageScore} 
                    colorScheme="purple" 
                    size="sm" 
                    borderRadius="full" 
                  />
                </Box>

                <Box>
                  <HStack justify="space-between" mb={1}>
                    <Text fontSize="sm">Efficiency</Text>
                    <Text fontSize="sm" fontWeight="bold">
                      {healthScore.components.efficiencyScore.toFixed(0)}%
                    </Text>
                  </HStack>
                  <Progress 
                    value={healthScore.components.efficiencyScore} 
                    colorScheme="orange" 
                    size="sm" 
                    borderRadius="full" 
                  />
                </Box>

                <Box>
                  <HStack justify="space-between" mb={1}>
                    <Text fontSize="sm">Growth</Text>
                    <Text fontSize="sm" fontWeight="bold">
                      {healthScore.components.growthScore.toFixed(0)}%
                    </Text>
                  </HStack>
                  <Progress 
                    value={healthScore.components.growthScore} 
                    colorScheme="teal" 
                    size="sm" 
                    borderRadius="full" 
                  />
                </Box>
              </VStack>
            </SimpleGrid>
          </CardBody>
        </Card>
      )}

      {/* Financial Ratios */}
      {dashboardData?.ratios && (
        <Card>
          <CardHeader>
            <Heading size="md">Key Financial Ratios</Heading>
          </CardHeader>
          <CardBody>
            <SimpleGrid columns={{ base: 2, md: 4 }} spacing={4}>
              <Stat>
                <StatLabel>Current Ratio</StatLabel>
                <StatNumber color={dashboardData.ratios.currentRatio >= 1.5 ? 'green.500' : 'red.500'}>
                  {dashboardData.ratios.currentRatio.toFixed(2)}
                </StatNumber>
                <StatHelpText>
                  {dashboardData.ratios.currentRatio >= 1.5 ? 'Good' : 'Needs Attention'}
                </StatHelpText>
              </Stat>

              <Stat>
                <StatLabel>Gross Profit Margin</StatLabel>
                <StatNumber color="blue.500">
                  {financialReportService.formatPercentage(dashboardData.ratios.grossProfitMargin)}
                </StatNumber>
              </Stat>

              <Stat>
                <StatLabel>Net Profit Margin</StatLabel>
                <StatNumber color={dashboardData.ratios.netProfitMargin >= 10 ? 'green.500' : 'orange.500'}>
                  {financialReportService.formatPercentage(dashboardData.ratios.netProfitMargin)}
                </StatNumber>
              </Stat>

              <Stat>
                <StatLabel>ROA</StatLabel>
                <StatNumber color="purple.500">
                  {financialReportService.formatPercentage(dashboardData.ratios.roa)}
                </StatNumber>
              </Stat>
            </SimpleGrid>
          </CardBody>
        </Card>
      )}

      {/* Financial Alerts */}
      {dashboardData?.alerts && dashboardData.alerts.length > 0 && (
        <Card>
          <CardHeader>
            <Heading size="md" display="flex" alignItems="center">
              <Icon as={FiAlertTriangle} mr={2} />
              Financial Alerts
            </Heading>
          </CardHeader>
          <CardBody>
            <VStack spacing={3} align="stretch">
              {dashboardData.alerts.map((alert, index) => (
                <Alert
                  key={index}
                  status={alert.severity === 'HIGH' ? 'error' : alert.severity === 'MEDIUM' ? 'warning' : 'info'}
                  borderRadius="md"
                >
                  <AlertIcon />
                  <VStack align="start" spacing={1} flex={1}>
                    <HStack justify="space-between" width="100%">
                      <Text fontWeight="bold">{alert.title}</Text>
                      <Badge colorScheme={alert.severity === 'HIGH' ? 'red' : alert.severity === 'MEDIUM' ? 'orange' : 'blue'}>
                        {alert.severity}
                      </Badge>
                    </HStack>
                    <Text fontSize="sm">{alert.description}</Text>
                    <Text fontSize="xs" color="gray.500">
                      Value: {alert.value.toFixed(2)} | Threshold: {alert.threshold}
                    </Text>
                  </VStack>
                </Alert>
              ))}
            </VStack>
          </CardBody>
        </Card>
      )}

      {/* Cash Position Summary */}
      {dashboardData?.cashPosition && (
        <Card>
          <CardHeader>
            <Heading size="md" display="flex" alignItems="center">
              <Icon as={FiDollarSign} mr={2} />
              Cash Position
            </Heading>
          </CardHeader>
          <CardBody>
            <SimpleGrid columns={{ base: 1, md: 2 }} spacing={6}>
              <Stat>
                <StatLabel>Total Cash</StatLabel>
                <StatNumber color="blue.500">
                  {financialReportService.formatCurrency(dashboardData.cashPosition.totalCash)}
                </StatNumber>
                <StatHelpText>
                  30-day flow: {financialReportService.formatCurrency(dashboardData.cashPosition.cashFlow30Day)}
                </StatHelpText>
              </Stat>

              <VStack align="start" spacing={2}>
                <Text fontWeight="semibold" fontSize="sm">Cash Accounts</Text>
                {dashboardData.cashPosition.cashAccounts.length === 0 ? (
                  <Text fontSize="sm" color="gray.500">No cash accounts data</Text>
                ) : (
                  dashboardData.cashPosition.cashAccounts.map((account: any, index: number) => (
                    <HStack key={index} justify="space-between" width="100%">
                      <Text fontSize="sm">{account.name}</Text>
                      <Text fontSize="sm" fontWeight="medium">
                        {financialReportService.formatCurrency(account.balance)}
                      </Text>
                    </HStack>
                  ))
                )}
              </VStack>
            </SimpleGrid>
          </CardBody>
        </Card>
      )}

      {/* Key Metrics Summary */}
      {dashboardData?.keyMetrics && (
        <Card>
          <CardHeader>
            <Heading size="md">Key Financial Metrics</Heading>
          </CardHeader>
          <CardBody>
            <SimpleGrid columns={{ base: 2, md: 4, lg: 5 }} spacing={4}>
              <Stat>
                <StatLabel>Total Assets</StatLabel>
                <StatNumber color="blue.500">
                  {financialReportService.formatCurrency(dashboardData.keyMetrics.totalAssets)}
                </StatNumber>
              </Stat>

              <Stat>
                <StatLabel>Total Liabilities</StatLabel>
                <StatNumber color="red.500">
                  {financialReportService.formatCurrency(dashboardData.keyMetrics.totalLiabilities)}
                </StatNumber>
              </Stat>

              <Stat>
                <StatLabel>Total Equity</StatLabel>
                <StatNumber color="green.500">
                  {financialReportService.formatCurrency(dashboardData.keyMetrics.totalEquity)}
                </StatNumber>
              </Stat>

              <Stat>
                <StatLabel>Receivables</StatLabel>
                <StatNumber color="orange.500">
                  {financialReportService.formatCurrency(dashboardData.keyMetrics.accountsReceivable)}
                </StatNumber>
              </Stat>

              <Stat>
                <StatLabel>Inventory</StatLabel>
                <StatNumber color="purple.500">
                  {financialReportService.formatCurrency(dashboardData.keyMetrics.inventory)}
                </StatNumber>
              </Stat>
            </SimpleGrid>
          </CardBody>
        </Card>
      )}

      {/* Health Recommendations */}
      {healthScore?.recommendations && healthScore.recommendations.length > 0 && (
        <Card>
          <CardHeader>
            <Heading size="md">Financial Health Recommendations</Heading>
          </CardHeader>
          <CardBody>
            <VStack spacing={4} align="stretch">
              {healthScore.recommendations.map((recommendation, index) => (
                <Card key={index} variant="outline" size="sm">
                  <CardBody>
                    <VStack align="start" spacing={2}>
                      <HStack justify="space-between" width="100%">
                        <Text fontWeight="bold">{recommendation.title}</Text>
                        <Badge 
                          colorScheme={recommendation.priority === 'HIGH' ? 'red' : 
                                     recommendation.priority === 'MEDIUM' ? 'orange' : 'blue'}
                        >
                          {recommendation.priority}
                        </Badge>
                      </HStack>
                      <Text fontSize="sm" color="gray.600">
                        {recommendation.description}
                      </Text>
                      <Text fontSize="sm" color="blue.600" fontStyle="italic">
                        Action: {recommendation.action}
                      </Text>
                    </VStack>
                  </CardBody>
                </Card>
              ))}
            </VStack>
          </CardBody>
        </Card>
      )}

      {/* Recent Activity */}
      {dashboardData?.recentActivity && dashboardData.recentActivity.length > 0 && (
        <Card>
          <CardHeader>
            <Heading size="md">Recent Financial Activity</Heading>
          </CardHeader>
          <CardBody>
            <VStack spacing={3} align="stretch">
              {dashboardData.recentActivity.map((activity: any, index: number) => (
                <HStack key={index} justify="space-between" p={3} bg="gray.50" borderRadius="md">
                  <VStack align="start" spacing={1}>
                    <Text fontSize="sm" fontWeight="medium">{activity.description}</Text>
                    <Text fontSize="xs" color="gray.500">{activity.date}</Text>
                  </VStack>
                  <Text fontSize="sm" fontWeight="bold" color={activity.amount >= 0 ? 'green.500' : 'red.500'}>
                    {financialReportService.formatCurrency(activity.amount)}
                  </Text>
                </HStack>
              ))}
            </VStack>
          </CardBody>
        </Card>
      )}
    </VStack>
  );
};

export default FinancialDashboard;
