'use client';

import React, { useMemo, useState } from 'react';
import {
  Box,
  VStack,
  HStack,
  Text,
  Card,
  CardBody,
  CardHeader,
  Grid,
  GridItem,
  Stat,
  StatLabel,
  StatNumber,
  StatHelpText,
  StatArrow,
  Progress,
  Badge,
  Tabs,
  TabList,
  TabPanels,
  Tab,
  TabPanel,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  Select,
  useColorModeValue,
  Flex,
  Divider,
  IconButton,
  Tooltip,
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
} from '@chakra-ui/react';
import { 
  FiTrendingUp, 
  FiTrendingDown, 
  FiBarChart2, 
  FiPieChart, 
  FiActivity,
  FiCalendar,
  FiDollarSign,
  FiUsers,
  FiCheckCircle,
  FiAlertCircle,
} from 'react-icons/fi';
import { formatCurrency } from '@/utils/formatters';

interface JournalEntry {
  id: number;
  code: string;
  description: string;
  reference: string;
  reference_type: string;
  entry_date: string;
  status: string;
  total_debit: number;
  total_credit: number;
  is_balanced: boolean;
  creator: {
    id: number;
    name: string;
  };
}

interface JournalVisualizationData {
  journal_entries: JournalEntry[];
  summary: {
    total_debit: number;
    total_credit: number;
    net_amount: number;
    entry_count: number;
    date_range_start: string;
    date_range_end: string;
    accounts_involved: string[];
  };
  metadata: {
    report_type: string;
    line_item_name: string;
    filter_criteria: string;
    generated_at: string;
  };
}

interface JournalDataVisualizationProps {
  data: JournalVisualizationData;
  title?: string;
}

interface CreatorStat {
  name: string;
  entries: number;
  total_amount: number;
  balanced_entries: number;
  accuracy: number;
}

export const JournalDataVisualization: React.FC<JournalDataVisualizationProps> = ({
  data,
  title = 'Journal Entry Analysis',
}) => {
  const [selectedPeriod, setSelectedPeriod] = useState<'daily' | 'weekly' | 'monthly'>('daily');
  
  // Color mode values
  const cardBg = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const textColor = useColorModeValue('gray.800', 'white');
  const mutedColor = useColorModeValue('gray.600', 'gray.400');
  const successColor = useColorModeValue('green.500', 'green.300');
  const errorColor = useColorModeValue('red.500', 'red.300');
  const warningColor = useColorModeValue('orange.500', 'orange.300');

  // Calculate analytics from journal entries
  const analytics = useMemo(() => {
    if (!data?.journal_entries?.length) {
      return {
        dailyTrends: [],
        typeDistribution: [],
        statusBreakdown: [],
        creatorStats: [],
        balanceAccuracy: 0,
        averageAmount: 0,
        largestEntry: null,
        unbalancedEntries: [],
        recentActivity: [],
      };
    }

    const entries = data.journal_entries;
    
    // Daily trends analysis
    const dailyTrends = entries.reduce((acc: any, entry) => {
      const date = new Date(entry.entry_date).toISOString().split('T')[0];
      if (!acc[date]) {
        acc[date] = {
          date,
          debit: 0,
          credit: 0,
          count: 0,
          balanced: 0,
        };
      }
      acc[date].debit += entry.total_debit;
      acc[date].credit += entry.total_credit;
      acc[date].count += 1;
      if (entry.is_balanced) acc[date].balanced += 1;
      return acc;
    }, {});

    const sortedDailyTrends = Object.values(dailyTrends).sort((a: any, b: any) => 
      new Date(a.date).getTime() - new Date(b.date).getTime()
    );

    // Transaction type distribution
    const typeDistribution = entries.reduce((acc: any, entry) => {
      const type = entry.reference_type || 'UNKNOWN';
      if (!acc[type]) {
        acc[type] = { type, count: 0, total_amount: 0, color: getTypeColor(type) };
      }
      acc[type].count += 1;
      acc[type].total_amount += Math.max(entry.total_debit, entry.total_credit);
      return acc;
    }, {});

    // Status breakdown
    const statusBreakdown = entries.reduce((acc: any, entry) => {
      const status = entry.status || 'UNKNOWN';
      if (!acc[status]) {
        acc[status] = { status, count: 0, percentage: 0, color: getStatusColor(status) };
      }
      acc[status].count += 1;
      return acc;
    }, {});

    Object.values(statusBreakdown).forEach((status: any) => {
      status.percentage = (status.count / entries.length) * 100;
    });

    // Creator statistics
    const creatorStats = entries.reduce<Record<string, CreatorStat>>((acc, entry) => {
      const creatorName = entry.creator?.name || 'Unknown';
      if (!acc[creatorName]) {
        acc[creatorName] = {
          name: creatorName,
          entries: 0,
          total_amount: 0,
          balanced_entries: 0,
          accuracy: 0,
        };
      }
      acc[creatorName].entries += 1;
      acc[creatorName].total_amount += Math.max(entry.total_debit, entry.total_credit);
      if (entry.is_balanced) acc[creatorName].balanced_entries += 1;
      return acc;
    }, {});

    Object.values(creatorStats).forEach((creator) => {
      creator.accuracy = creator.entries > 0 ? (creator.balanced_entries / creator.entries) * 100 : 0;
    });

    // Calculate overall balance accuracy
    const balancedCount = entries.filter(e => e.is_balanced).length;
    const balanceAccuracy = entries.length > 0 ? (balancedCount / entries.length) * 100 : 0;

    // Calculate average amount
    const totalAmount = entries.reduce((sum, entry) => 
      sum + Math.max(entry.total_debit, entry.total_credit), 0
    );
    const averageAmount = entries.length > 0 ? totalAmount / entries.length : 0;

    // Find largest entry
    const largestEntry = entries.reduce<JournalEntry | null>((largest, entry) => {
      const entryAmount = Math.max(entry.total_debit, entry.total_credit);
      const largestAmount = largest ? Math.max(largest.total_debit, largest.total_credit) : 0;
      return entryAmount > largestAmount ? entry : largest;
    }, null);

    // Unbalanced entries
    const unbalancedEntries = entries.filter(entry => !entry.is_balanced);

    // Recent activity (last 7 entries)
    const recentActivity = entries
      .sort((a, b) => new Date(b.entry_date).getTime() - new Date(a.entry_date).getTime())
      .slice(0, 7);

    return {
      dailyTrends: sortedDailyTrends,
      typeDistribution: Object.values(typeDistribution),
      statusBreakdown: Object.values(statusBreakdown),
      creatorStats: Object.values(creatorStats).sort((a, b) => b.entries - a.entries),
      balanceAccuracy,
      averageAmount,
      largestEntry,
      unbalancedEntries,
      recentActivity,
    };
  }, [data]);

  const getTypeColor = (type: string) => {
    switch (type) {
      case 'SALE': return 'blue';
      case 'PURCHASE': return 'orange';
      case 'PAYMENT': return 'purple';
      case 'CASH_BANK': return 'teal';
      case 'MANUAL': return 'gray';
      default: return 'gray';
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'POSTED': return 'green';
      case 'DRAFT': return 'yellow';
      case 'REVERSED': return 'red';
      default: return 'gray';
    }
  };

  if (!data?.journal_entries?.length) {
    return (
      <Card>
        <CardBody>
          <Alert status="info">
            <AlertIcon />
            <AlertTitle>No Data Available</AlertTitle>
            <AlertDescription>
              No journal entries found for the selected criteria. Please adjust your filters or date range.
            </AlertDescription>
          </Alert>
        </CardBody>
      </Card>
    );
  }

  return (
    <VStack spacing={6} align="stretch">
      {/* Header */}
      <Card>
        <CardHeader>
          <HStack justify="space-between">
            <VStack align="start" spacing={1}>
              <Text fontSize="xl" fontWeight="bold" color={textColor}>
                {title}
              </Text>
              <Text fontSize="sm" color={mutedColor}>
                {data.metadata?.line_item_name} • {data.summary?.entry_count} entries
              </Text>
            </VStack>
            <Badge colorScheme="blue" variant="solid" px={3} py={1}>
              Enhanced Analysis
            </Badge>
          </HStack>
        </CardHeader>
      </Card>

      {/* Key Metrics Overview */}
      <Grid templateColumns="repeat(auto-fit, minmax(200px, 1fr))" gap={4}>
        <Card>
          <CardBody>
            <Stat>
              <HStack>
                <Box p={2} bg="blue.100" borderRadius="md">
                  <FiDollarSign color="blue.500" />
                </Box>
                <VStack align="start" spacing={0}>
                  <StatLabel fontSize="xs">Total Volume</StatLabel>
                  <StatNumber fontSize="lg">
                    {formatCurrency(Math.max(data.summary.total_debit, data.summary.total_credit))}
                  </StatNumber>
                  <StatHelpText fontSize="xs">
                    Avg: {formatCurrency(analytics.averageAmount)}
                  </StatHelpText>
                </VStack>
              </HStack>
            </Stat>
          </CardBody>
        </Card>

        <Card>
          <CardBody>
            <Stat>
              <HStack>
                <Box p={2} bg="green.100" borderRadius="md">
                  <FiCheckCircle color="green.500" />
                </Box>
                <VStack align="start" spacing={0}>
                  <StatLabel fontSize="xs">Balance Accuracy</StatLabel>
                  <StatNumber fontSize="lg">{analytics.balanceAccuracy.toFixed(1)}%</StatNumber>
                  <StatHelpText fontSize="xs">
                    <StatArrow type={analytics.balanceAccuracy > 95 ? 'increase' : 'decrease'} />
                    {analytics.balanceAccuracy > 95 ? 'Excellent' : analytics.balanceAccuracy > 80 ? 'Good' : 'Needs Review'}
                  </StatHelpText>
                </VStack>
              </HStack>
            </Stat>
          </CardBody>
        </Card>

        <Card>
          <CardBody>
            <Stat>
              <HStack>
                <Box p={2} bg="purple.100" borderRadius="md">
                  <FiActivity color="purple.500" />
                </Box>
                <VStack align="start" spacing={0}>
                  <StatLabel fontSize="xs">Active Days</StatLabel>
                  <StatNumber fontSize="lg">{analytics.dailyTrends.length}</StatNumber>
                  <StatHelpText fontSize="xs">
                    {analytics.dailyTrends.length > 0 ? 
                      `${(data.summary.entry_count / analytics.dailyTrends.length).toFixed(1)} avg/day` : 'No activity'
                    }
                  </StatHelpText>
                </VStack>
              </HStack>
            </Stat>
          </CardBody>
        </Card>

        <Card>
          <CardBody>
            <Stat>
              <HStack>
                <Box p={2} bg="orange.100" borderRadius="md">
                  <FiUsers color="orange.500" />
                </Box>
                <VStack align="start" spacing={0}>
                  <StatLabel fontSize="xs">Contributors</StatLabel>
                  <StatNumber fontSize="lg">{analytics.creatorStats.length}</StatNumber>
                  <StatHelpText fontSize="xs">
                    Most active: {analytics.creatorStats[0]?.name || 'N/A'}
                  </StatHelpText>
                </VStack>
              </HStack>
            </Stat>
          </CardBody>
        </Card>
      </Grid>

      <Tabs variant="enclosed">
        <TabList>
          <Tab>
            <HStack>
              <FiTrendingUp />
              <Text>Trends</Text>
            </HStack>
          </Tab>
          <Tab>
            <HStack>
              <FiPieChart />
              <Text>Distribution</Text>
            </HStack>
          </Tab>
          <Tab>
            <HStack>
              <FiUsers />
              <Text>Contributors</Text>
            </HStack>
          </Tab>
          <Tab>
            <HStack>
              <FiAlertCircle />
              <Text>Issues</Text>
            </HStack>
          </Tab>
        </TabList>

        <TabPanels>
          {/* Trends Tab */}
          <TabPanel>
            <VStack spacing={6} align="stretch">
              <Card>
                <CardHeader>
                  <HStack justify="space-between">
                    <Text fontSize="lg" fontWeight="semibold">Daily Activity Trends</Text>
                    <Select size="sm" w="auto" value={selectedPeriod} onChange={(e) => setSelectedPeriod(e.target.value as any)}>
                      <option value="daily">Daily View</option>
                      <option value="weekly">Weekly View</option>
                      <option value="monthly">Monthly View</option>
                    </Select>
                  </HStack>
                </CardHeader>
                <CardBody>
                  <VStack spacing={4} align="stretch">
                    {analytics.dailyTrends.slice(-10).map((trend: any, index: number) => (
                      <Box key={trend.date}>
                        <HStack justify="space-between" mb={2}>
                          <Text fontSize="sm" fontWeight="medium">
                            {new Date(trend.date).toLocaleDateString()}
                          </Text>
                          <HStack spacing={4}>
                            <Text fontSize="sm" color={successColor}>
                              {trend.count} entries
                            </Text>
                            <Text fontSize="sm" color={mutedColor}>
                              {((trend.balanced / trend.count) * 100).toFixed(0)}% balanced
                            </Text>
                          </HStack>
                        </HStack>
                        <HStack spacing={2}>
                          <Text fontSize="xs" minW="60px">Debit:</Text>
                          <Progress value={(trend.debit / data.summary.total_debit) * 100} size="sm" colorScheme="green" flex="1" />
                          <Text fontSize="xs" minW="100px" textAlign="right">{formatCurrency(trend.debit)}</Text>
                        </HStack>
                        <HStack spacing={2}>
                          <Text fontSize="xs" minW="60px">Credit:</Text>
                          <Progress value={(trend.credit / data.summary.total_credit) * 100} size="sm" colorScheme="red" flex="1" />
                          <Text fontSize="xs" minW="100px" textAlign="right">{formatCurrency(trend.credit)}</Text>
                        </HStack>
                        {index < analytics.dailyTrends.slice(-10).length - 1 && <Divider mt={3} />}
                      </Box>
                    ))}
                  </VStack>
                </CardBody>
              </Card>
            </VStack>
          </TabPanel>

          {/* Distribution Tab */}
          <TabPanel>
            <Grid templateColumns="repeat(auto-fit, minmax(300px, 1fr))" gap={6}>
              {/* Transaction Types */}
              <Card>
                <CardHeader>
                  <Text fontSize="lg" fontWeight="semibold">Transaction Types</Text>
                </CardHeader>
                <CardBody>
                  <VStack spacing={4} align="stretch">
                    {analytics.typeDistribution.map((type: any) => (
                      <Box key={type.type}>
                        <HStack justify="space-between" mb={2}>
                          <Badge colorScheme={type.color} variant="solid">{type.type}</Badge>
                          <Text fontSize="sm" color={mutedColor}>{type.count} entries</Text>
                        </HStack>
                        <Progress 
                          value={(type.count / data.summary.entry_count) * 100} 
                          size="md" 
                          colorScheme={type.color}
                        />
                        <HStack justify="space-between" mt={1}>
                          <Text fontSize="xs" color={mutedColor}>
                            {((type.count / data.summary.entry_count) * 100).toFixed(1)}%
                          </Text>
                          <Text fontSize="xs" color={mutedColor}>
                            {formatCurrency(type.total_amount)}
                          </Text>
                        </HStack>
                      </Box>
                    ))}
                  </VStack>
                </CardBody>
              </Card>

              {/* Status Distribution */}
              <Card>
                <CardHeader>
                  <Text fontSize="lg" fontWeight="semibold">Entry Status</Text>
                </CardHeader>
                <CardBody>
                  <VStack spacing={4} align="stretch">
                    {analytics.statusBreakdown.map((status: any) => (
                      <Box key={status.status}>
                        <HStack justify="space-between" mb={2}>
                          <Badge colorScheme={status.color} variant="solid">{status.status}</Badge>
                          <Text fontSize="sm" color={mutedColor}>{status.count} entries</Text>
                        </HStack>
                        <Progress 
                          value={status.percentage} 
                          size="md" 
                          colorScheme={status.color}
                        />
                        <Text fontSize="xs" color={mutedColor} mt={1}>
                          {status.percentage.toFixed(1)}%
                        </Text>
                      </Box>
                    ))}
                  </VStack>
                </CardBody>
              </Card>
            </Grid>
          </TabPanel>

          {/* Contributors Tab */}
          <TabPanel>
            <Card>
              <CardHeader>
                <Text fontSize="lg" fontWeight="semibold">Contributor Performance</Text>
              </CardHeader>
              <CardBody>
                <Table size="sm">
                  <Thead>
                    <Tr>
                      <Th>Creator</Th>
                      <Th isNumeric>Entries</Th>
                      <Th isNumeric>Total Amount</Th>
                      <Th isNumeric>Accuracy</Th>
                      <Th>Performance</Th>
                    </Tr>
                  </Thead>
                  <Tbody>
                    {analytics.creatorStats.map((creator: any) => (
                      <Tr key={creator.name}>
                        <Td>
                          <Text fontWeight="medium">{creator.name}</Text>
                        </Td>
                        <Td isNumeric>{creator.entries}</Td>
                        <Td isNumeric>{formatCurrency(creator.total_amount)}</Td>
                        <Td isNumeric>
                          <Text color={creator.accuracy > 95 ? successColor : creator.accuracy > 80 ? warningColor : errorColor}>
                            {creator.accuracy.toFixed(1)}%
                          </Text>
                        </Td>
                        <Td>
                          <HStack>
                            <Progress 
                              value={creator.accuracy} 
                              size="sm" 
                              colorScheme={creator.accuracy > 95 ? 'green' : creator.accuracy > 80 ? 'yellow' : 'red'}
                              w="100px"
                            />
                            <Badge 
                              size="sm"
                              colorScheme={creator.accuracy > 95 ? 'green' : creator.accuracy > 80 ? 'yellow' : 'red'}
                            >
                              {creator.accuracy > 95 ? 'Excellent' : creator.accuracy > 80 ? 'Good' : 'Review'}
                            </Badge>
                          </HStack>
                        </Td>
                      </Tr>
                    ))}
                  </Tbody>
                </Table>
              </CardBody>
            </Card>
          </TabPanel>

          {/* Issues Tab */}
          <TabPanel>
            <VStack spacing={6} align="stretch">
              {/* Unbalanced Entries */}
              {analytics.unbalancedEntries.length > 0 ? (
                <Card>
                  <CardHeader>
                    <HStack>
                      <FiAlertCircle color={errorColor} />
                      <Text fontSize="lg" fontWeight="semibold" color={errorColor}>
                        Unbalanced Entries ({analytics.unbalancedEntries.length})
                      </Text>
                    </HStack>
                  </CardHeader>
                  <CardBody>
                    <Table size="sm">
                      <Thead>
                        <Tr>
                          <Th>Code</Th>
                          <Th>Date</Th>
                          <Th>Description</Th>
                          <Th isNumeric>Debit</Th>
                          <Th isNumeric>Credit</Th>
                          <Th isNumeric>Difference</Th>
                        </Tr>
                      </Thead>
                      <Tbody>
                        {analytics.unbalancedEntries.slice(0, 10).map((entry: JournalEntry) => (
                          <Tr key={entry.id}>
                            <Td>
                              <Text fontSize="sm" fontFamily="mono">{entry.code}</Text>
                            </Td>
                            <Td>
                              <Text fontSize="sm">{new Date(entry.entry_date).toLocaleDateString()}</Text>
                            </Td>
                            <Td maxW="200px">
                              <Text fontSize="sm" noOfLines={2}>{entry.description}</Text>
                            </Td>
                            <Td isNumeric>
                              <Text fontSize="sm" color="green.600">
                                {formatCurrency(entry.total_debit)}
                              </Text>
                            </Td>
                            <Td isNumeric>
                              <Text fontSize="sm" color="red.600">
                                {formatCurrency(entry.total_credit)}
                              </Text>
                            </Td>
                            <Td isNumeric>
                              <Text fontSize="sm" color={errorColor} fontWeight="bold">
                                {formatCurrency(Math.abs(entry.total_debit - entry.total_credit))}
                              </Text>
                            </Td>
                          </Tr>
                        ))}
                      </Tbody>
                    </Table>
                  </CardBody>
                </Card>
              ) : (
                <Card>
                  <CardBody>
                    <HStack justify="center" py={4}>
                      <FiCheckCircle color={successColor} size="24" />
                      <Text color={successColor} fontSize="lg" fontWeight="semibold">
                        All entries are balanced! Great work.
                      </Text>
                    </HStack>
                  </CardBody>
                </Card>
              )}

              {/* Summary Card */}
              <Card>
                <CardHeader>
                  <Text fontSize="lg" fontWeight="semibold">Quality Summary</Text>
                </CardHeader>
                <CardBody>
                  <Grid templateColumns="repeat(auto-fit, minmax(200px, 1fr))" gap={4}>
                    <Box>
                      <Text fontSize="sm" color={mutedColor} mb={1}>Overall Health Score</Text>
                      <HStack>
                        <Progress 
                          value={analytics.balanceAccuracy} 
                          size="lg" 
                          colorScheme={analytics.balanceAccuracy > 95 ? 'green' : analytics.balanceAccuracy > 80 ? 'yellow' : 'red'}
                          flex="1"
                        />
                        <Text fontSize="lg" fontWeight="bold">
                          {analytics.balanceAccuracy.toFixed(0)}%
                        </Text>
                      </HStack>
                    </Box>
                    <Box>
                      <Text fontSize="sm" color={mutedColor}>Largest Entry</Text>
                      <Text fontSize="md" fontWeight="semibold">
                        {analytics.largestEntry ? formatCurrency(Math.max(
                          analytics.largestEntry.total_debit, 
                          analytics.largestEntry.total_credit
                        )) : 'N/A'}
                      </Text>
                    </Box>
                    <Box>
                      <Text fontSize="sm" color={mutedColor}>Issues Found</Text>
                      <Text fontSize="md" fontWeight="semibold" color={analytics.unbalancedEntries.length > 0 ? errorColor : successColor}>
                        {analytics.unbalancedEntries.length} unbalanced entries
                      </Text>
                    </Box>
                  </Grid>
                </CardBody>
              </Card>
            </VStack>
          </TabPanel>
        </TabPanels>
      </Tabs>
    </VStack>
  );
};

export default JournalDataVisualization;
