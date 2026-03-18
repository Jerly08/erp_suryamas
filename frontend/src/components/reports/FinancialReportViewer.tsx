import React from 'react';
import {
  Box,
  VStack,
  HStack,
  Text,
  Heading,
  Card,
  CardBody,
  CardHeader,
  Table,
  Thead,
  Tbody,
  Tfoot,
  Tr,
  Th,
  Td,
  TableContainer,
  Stat,
  StatLabel,
  StatNumber,
  StatHelpText,
  SimpleGrid,
  Badge,
  Alert,
  AlertIcon,
  Divider,
  Icon,
  Progress,
} from '@chakra-ui/react';
import {
  FiTrendingUp,
  FiTrendingDown,
  FiDollarSign,
  FiPieChart,
  FiBarChart,
  FiActivity,
} from 'react-icons/fi';
import financialReportService, {
  ProfitLossStatement,
  BalanceSheet,
  CashFlowStatement,
  TrialBalance,
  GeneralLedger,
  FinancialRatios,
  FinancialHealthScore,
} from '../../services/financialReportService';

interface FinancialReportViewerProps {
  reportType: string;
  reportData: any;
}

const FinancialReportViewer: React.FC<FinancialReportViewerProps> = ({
  reportType,
  reportData,
}) => {
  if (!reportData) {
    return (
      <Alert status="info">
        <AlertIcon />
        No report data to display. Please generate a report first.
      </Alert>
    );
  }

  const renderReportContent = () => {
    switch (reportType) {
      case 'PROFIT_LOSS':
        return <ProfitLossView data={reportData as ProfitLossStatement} />;
      case 'BALANCE_SHEET':
        return <BalanceSheetView data={reportData as BalanceSheet} />;
      case 'CASH_FLOW':
        return <CashFlowView data={reportData as CashFlowStatement} />;
      case 'TRIAL_BALANCE':
        return <TrialBalanceView data={reportData as TrialBalance} />;
      case 'GENERAL_LEDGER':
        return <GeneralLedgerView data={reportData as GeneralLedger} />;
      default:
        return (
          <Alert status="warning">
            <AlertIcon />
            Report type "{reportType}" is not supported for viewing.
          </Alert>
        );
    }
  };

  return (
    <Box>
      {renderReportContent()}
    </Box>
  );
};

// Profit & Loss Statement Viewer
const ProfitLossView: React.FC<{ data: ProfitLossStatement }> = ({ data }) => {
  const getVarianceColor = (variance: number) => {
    if (variance > 0) return 'green.500';
    if (variance < 0) return 'red.500';
    return 'gray.500';
  };

  const renderAccountLineItems = (items: any[], title: string) => (
    <TableContainer>
      <Table size="sm">
        <Thead>
          <Tr>
            <Th>{title}</Th>
            <Th isNumeric>Amount</Th>
            {data.comparative && <Th isNumeric>Previous</Th>}
            {data.comparative && <Th isNumeric>Variance</Th>}
          </Tr>
        </Thead>
        <Tbody>
          {items.map((item, index) => (
            <Tr key={index}>
              <Td fontSize="sm">
                {item.accountCode} - {item.accountName}
              </Td>
              <Td isNumeric fontSize="sm">
                {financialReportService.formatCurrency(item.balance)}
              </Td>
              {data.comparative && (
                <>
                  <Td isNumeric fontSize="sm">
                    {financialReportService.formatCurrency(item.previousBalance ?? 0)}
                  </Td>
                  <Td isNumeric fontSize="sm">
                    <Text color={getVarianceColor((item.balance ?? 0) - (item.previousBalance ?? 0))}>
                      {financialReportService.formatCurrency((item.balance ?? 0) - (item.previousBalance ?? 0))}
                    </Text>
                  </Td>
                </>
              )}
            </Tr>
          ))}
        </Tbody>
      </Table>
    </TableContainer>
  );

  return (
    <VStack spacing={6} align="stretch">
      {/* Header */}
      <Card>
        <CardHeader>
          <VStack align="start" spacing={2}>
            <Heading size="lg" display="flex" alignItems="center">
              <Icon as={FiTrendingUp} mr={2} />
              {data.reportHeader.reportTitle}
            </Heading>
            <Text color="gray.600">
              {financialReportService.formatDateRange(data.reportHeader.startDate, data.reportHeader.endDate)}
            </Text>
            <Text fontSize="sm" color="gray.500">
              Generated on {financialReportService.formatDate(data.reportHeader.generatedAt)}
            </Text>
          </VStack>
        </CardHeader>
      </Card>

      {/* Key Metrics */}
      <SimpleGrid columns={{ base: 2, md: 4 }} spacing={4}>
        <Stat>
          <StatLabel>Total Revenue</StatLabel>
          <StatNumber color="green.500">
            {financialReportService.formatCurrency(data.totalRevenue)}
          </StatNumber>
          {data.comparative && (
            <StatHelpText>
              <Icon
                as={(data.comparative.variance.revenueVariance || 0) >= 0 ? FiTrendingUp : FiTrendingDown}
              />
              {financialReportService.formatPercentage(data.comparative.variance.revenueVariance || 0)} vs previous
            </StatHelpText>
          )}
        </Stat>
        
        <Stat>
          <StatLabel>Gross Profit</StatLabel>
          <StatNumber color="blue.500">
            {financialReportService.formatCurrency(data.grossProfit)}
          </StatNumber>
          <StatHelpText>
            {financialReportService.formatPercentage(data.totalRevenue ? (data.grossProfit / data.totalRevenue) * 100 : 0, 1)} margin
          </StatHelpText>
        </Stat>

        <Stat>
          <StatLabel>Total Expenses</StatLabel>
          <StatNumber color="red.500">
            {financialReportService.formatCurrency(data.totalExpenses)}
          </StatNumber>
        </Stat>

        <Stat>
          <StatLabel>Net Income</StatLabel>
          <StatNumber color={data.netIncome >= 0 ? 'green.500' : 'red.500'}>
            {financialReportService.formatCurrency(data.netIncome)}
          </StatNumber>
          <StatHelpText>
            {financialReportService.formatPercentage(data.totalRevenue ? (data.netIncome / data.totalRevenue) * 100 : 0, 1)} margin
          </StatHelpText>
        </Stat>
      </SimpleGrid>

      {/* Revenue Section */}
      <Card>
        <CardHeader>
          <Heading size="md" color="green.600">Revenue</Heading>
        </CardHeader>
        <CardBody>
          {renderAccountLineItems(data.revenue, 'Revenue Accounts')}
          <HStack justify="space-between" mt={4} p={3} bg="green.50" rounded="md">
            <Text fontWeight="bold">Total Revenue</Text>
            <Text fontWeight="bold" color="green.600">
              {financialReportService.formatCurrency(data.totalRevenue)}
            </Text>
          </HStack>
        </CardBody>
      </Card>

      {/* COGS Section */}
      {data.cogs.length > 0 && (
        <Card>
          <CardHeader>
            <Heading size="md" color="orange.600">Cost of Goods Sold</Heading>
          </CardHeader>
          <CardBody>
            {renderAccountLineItems(data.cogs, 'COGS Accounts')}
            <HStack justify="space-between" mt={4} p={3} bg="orange.50" rounded="md">
              <Text fontWeight="bold">Total COGS</Text>
              <Text fontWeight="bold" color="orange.600">
                {financialReportService.formatCurrency(data.totalCOGS)}
              </Text>
            </HStack>
          </CardBody>
        </Card>
      )}

      {/* Gross Profit */}
      <Card bg="blue.50" borderColor="blue.200">
        <CardBody>
          <HStack justify="space-between">
            <Heading size="md">Gross Profit</Heading>
            <Heading size="md" color="blue.600">
              {financialReportService.formatCurrency(data.grossProfit)}
            </Heading>
          </HStack>
        </CardBody>
      </Card>

      {/* Expenses Section */}
      <Card>
        <CardHeader>
          <Heading size="md" color="red.600">Operating Expenses</Heading>
        </CardHeader>
        <CardBody>
          {renderAccountLineItems(data.expenses, 'Expense Accounts')}
          <HStack justify="space-between" mt={4} p={3} bg="red.50" rounded="md">
            <Text fontWeight="bold">Total Expenses</Text>
            <Text fontWeight="bold" color="red.600">
              {financialReportService.formatCurrency(data.totalExpenses)}
            </Text>
          </HStack>
        </CardBody>
      </Card>

      {/* Net Income */}
      <Card bg={data.netIncome >= 0 ? 'green.50' : 'red.50'} borderColor={data.netIncome >= 0 ? 'green.200' : 'red.200'}>
        <CardBody>
          <HStack justify="space-between">
            <Heading size="lg">Net Income</Heading>
            <Heading size="lg" color={data.netIncome >= 0 ? 'green.600' : 'red.600'}>
              {financialReportService.formatCurrency(data.netIncome)}
            </Heading>
          </HStack>
        </CardBody>
      </Card>
    </VStack>
  );
};

// Balance Sheet Viewer
const BalanceSheetView: React.FC<{ data: BalanceSheet }> = ({ data }) => {
  const renderBalanceSheetSection = (section: any, title: string, color: string) => (
    <Card>
      <CardHeader>
        <Heading size="md" color={`${color}.600`}>{title}</Heading>
      </CardHeader>
      <CardBody>
        {section.categories.map((category: any, categoryIndex: number) => (
          <Box key={categoryIndex} mb={4}>
            <Text fontWeight="semibold" mb={2}>{category.name}</Text>
            <TableContainer>
              <Table size="sm">
                <Tbody>
                  {category.accounts.map((account: any, accountIndex: number) => (
                    <Tr key={accountIndex}>
                      <Td fontSize="sm">
                        {account.accountCode} - {account.accountName}
                      </Td>
                      <Td isNumeric fontSize="sm">
                        {financialReportService.formatCurrency(account.balance)}
                      </Td>
                    </Tr>
                  ))}
                  <Tr bg={`${color}.50`}>
                    <Td fontWeight="bold">Total {category.name}</Td>
                    <Td isNumeric fontWeight="bold">
                      {financialReportService.formatCurrency(category.total)}
                    </Td>
                  </Tr>
                </Tbody>
              </Table>
            </TableContainer>
          </Box>
        ))}
        <HStack justify="space-between" mt={4} p={3} bg={`${color}.100`} rounded="md">
          <Text fontWeight="bold" fontSize="lg">Total {title}</Text>
          <Text fontWeight="bold" fontSize="lg" color={`${color}.700`}>
            {financialReportService.formatCurrency(section.total)}
          </Text>
        </HStack>
      </CardBody>
    </Card>
  );

  return (
    <VStack spacing={6} align="stretch">
      {/* Header */}
      <Card>
        <CardHeader>
          <VStack align="start" spacing={2}>
            <Heading size="lg" display="flex" alignItems="center">
              <Icon as={FiPieChart} mr={2} />
              {data.reportHeader.reportTitle}
            </Heading>
            <Text color="gray.600">
              As of {financialReportService.formatDate(data.reportHeader.endDate)}
            </Text>
            {!data.isBalanced && (
              <Alert status="error" size="sm">
                <AlertIcon />
                Balance Sheet is not balanced! Please check your accounts.
              </Alert>
            )}
          </VStack>
        </CardHeader>
      </Card>

      {/* Key Totals */}
      <SimpleGrid columns={{ base: 1, md: 3 }} spacing={4}>
        <Stat>
          <StatLabel>Total Assets</StatLabel>
          <StatNumber color="blue.500">
            {financialReportService.formatCurrency(data.totalAssets)}
          </StatNumber>
        </Stat>
        
        <Stat>
          <StatLabel>Total Liabilities</StatLabel>
          <StatNumber color="red.500">
            {financialReportService.formatCurrency(data.totalLiabilities)}
          </StatNumber>
        </Stat>

        <Stat>
          <StatLabel>Total Equity</StatLabel>
          <StatNumber color="green.500">
            {financialReportService.formatCurrency(data.totalEquity)}
          </StatNumber>
        </Stat>
      </SimpleGrid>

      {/* Assets */}
      {renderBalanceSheetSection(data.assets, 'Assets', 'blue')}

      {/* Liabilities */}
      {renderBalanceSheetSection(data.liabilities, 'Liabilities', 'red')}

      {/* Equity */}
      {renderBalanceSheetSection(data.equity, 'Equity', 'green')}

      {/* Balance Verification */}
      <Card bg={data.isBalanced ? 'green.50' : 'red.50'} borderColor={data.isBalanced ? 'green.200' : 'red.200'}>
        <CardBody>
          <VStack spacing={3}>
            <Heading size="md">Balance Verification</Heading>
            <SimpleGrid columns={2} spacing={4} width="100%">
              <Box textAlign="center">
                <Text fontSize="sm" color="gray.600">Assets</Text>
                <Text fontSize="lg" fontWeight="bold">
                  {financialReportService.formatCurrency(data.totalAssets)}
                </Text>
              </Box>
              <Box textAlign="center">
                <Text fontSize="sm" color="gray.600">Liabilities + Equity</Text>
                <Text fontSize="lg" fontWeight="bold">
                  {financialReportService.formatCurrency(data.totalLiabilities + data.totalEquity)}
                </Text>
              </Box>
            </SimpleGrid>
            <Badge colorScheme={data.isBalanced ? 'green' : 'red'} size="lg">
              {data.isBalanced ? 'BALANCED' : 'NOT BALANCED'}
            </Badge>
          </VStack>
        </CardBody>
      </Card>
    </VStack>
  );
};

// Cash Flow Statement Viewer
const CashFlowView: React.FC<{ data: CashFlowStatement }> = ({ data }) => {
  const renderCashFlowSection = (section: any, title: string, color: string) => (
    <Card>
      <CardHeader>
        <Heading size="md" color={`${color}.600`}>{title}</Heading>
      </CardHeader>
      <CardBody>
        <TableContainer>
          <Table size="sm">
            <Tbody>
              {section.items.map((item: any, index: number) => (
                <Tr key={index}>
                  <Td fontSize="sm">{item.description}</Td>
                  <Td isNumeric fontSize="sm">
                    {financialReportService.formatCurrency(item.amount)}
                  </Td>
                </Tr>
              ))}
            </Tbody>
          </Table>
        </TableContainer>
        <HStack justify="space-between" mt={4} p={3} bg={`${color}.50`} rounded="md">
          <Text fontWeight="bold">Net Cash from {title}</Text>
          <Text fontWeight="bold" color={`${color}.600`}>
            {financialReportService.formatCurrency(section.total)}
          </Text>
        </HStack>
      </CardBody>
    </Card>
  );

  return (
    <VStack spacing={6} align="stretch">
      {/* Header */}
      <Card>
        <CardHeader>
          <VStack align="start" spacing={2}>
            <Heading size="lg" display="flex" alignItems="center">
              <Icon as={FiActivity} mr={2} />
              {data.reportHeader.reportTitle}
            </Heading>
            <Text color="gray.600">
              {financialReportService.formatDateRange(data.reportHeader.startDate, data.reportHeader.endDate)}
            </Text>
          </VStack>
        </CardHeader>
      </Card>

      {/* Cash Summary */}
      <SimpleGrid columns={{ base: 1, md: 3 }} spacing={4}>
        <Stat>
          <StatLabel>Beginning Cash</StatLabel>
          <StatNumber>{financialReportService.formatCurrency(data.beginningCash)}</StatNumber>
        </Stat>
        
        <Stat>
          <StatLabel>Net Cash Flow</StatLabel>
          <StatNumber color={data.netCashFlow >= 0 ? 'green.500' : 'red.500'}>
            {financialReportService.formatCurrency(data.netCashFlow)}
          </StatNumber>
        </Stat>

        <Stat>
          <StatLabel>Ending Cash</StatLabel>
          <StatNumber color="blue.500">
            {financialReportService.formatCurrency(data.endingCash)}
          </StatNumber>
        </Stat>
      </SimpleGrid>

      {/* Operating Activities */}
      {renderCashFlowSection(data.operatingActivities, 'Operating Activities', 'green')}

      {/* Investing Activities */}
      {renderCashFlowSection(data.investingActivities, 'Investing Activities', 'blue')}

      {/* Financing Activities */}
      {renderCashFlowSection(data.financingActivities, 'Financing Activities', 'purple')}

      {/* Net Cash Flow Summary */}
      <Card bg={data.netCashFlow >= 0 ? 'green.50' : 'red.50'} borderColor={data.netCashFlow >= 0 ? 'green.200' : 'red.200'}>
        <CardBody>
          <VStack spacing={3}>
            <Heading size="md">Cash Flow Summary</Heading>
            <SimpleGrid columns={2} spacing={4} width="100%">
              <Box textAlign="center">
                <Text fontSize="sm" color="gray.600">Beginning Cash</Text>
                <Text fontSize="lg" fontWeight="bold">
                  {financialReportService.formatCurrency(data.beginningCash)}
                </Text>
              </Box>
              <Box textAlign="center">
                <Text fontSize="sm" color="gray.600">Net Change</Text>
                <Text fontSize="lg" fontWeight="bold" color={data.netCashFlow >= 0 ? 'green.600' : 'red.600'}>
                  {data.netCashFlow >= 0 ? '+' : ''}{financialReportService.formatCurrency(data.netCashFlow)}
                </Text>
              </Box>
            </SimpleGrid>
            <Divider />
            <Text fontSize="xl" fontWeight="bold">
              Ending Cash: {financialReportService.formatCurrency(data.endingCash)}
            </Text>
          </VStack>
        </CardBody>
      </Card>
    </VStack>
  );
};

// Trial Balance Viewer
const TrialBalanceView: React.FC<{ data: TrialBalance }> = ({ data }) => {
  return (
    <VStack spacing={6} align="stretch">
      {/* Header */}
      <Card>
        <CardHeader>
          <VStack align="start" spacing={2}>
            <Heading size="lg" display="flex" alignItems="center">
              <Icon as={FiBarChart} mr={2} />
              {data.reportHeader.reportTitle}
            </Heading>
            <Text color="gray.600">
              As of {financialReportService.formatDate(data.reportHeader.endDate)}
            </Text>
            {!data.isBalanced && (
              <Alert status="error">
                <AlertIcon />
                Trial Balance is not balanced! Debits do not equal Credits.
              </Alert>
            )}
          </VStack>
        </CardHeader>
      </Card>

      {/* Balance Summary */}
      <SimpleGrid columns={{ base: 1, md: 3 }} spacing={4}>
        <Stat>
          <StatLabel>Total Debits</StatLabel>
          <StatNumber color="blue.500">
            {financialReportService.formatCurrency(data.totalDebits)}
          </StatNumber>
        </Stat>
        
        <Stat>
          <StatLabel>Total Credits</StatLabel>
          <StatNumber color="green.500">
            {financialReportService.formatCurrency(data.totalCredits)}
          </StatNumber>
        </Stat>

        <Stat>
          <StatLabel>Balance Status</StatLabel>
          <Badge colorScheme={data.isBalanced ? 'green' : 'red'} fontSize="md" p={2}>
            {data.isBalanced ? 'BALANCED' : 'NOT BALANCED'}
          </Badge>
        </Stat>
      </SimpleGrid>

      {/* Accounts Table */}
      <Card>
        <CardHeader>
          <Heading size="md">Account Balances</Heading>
        </CardHeader>
        <CardBody>
          <TableContainer>
            <Table size="sm">
              <Thead>
                <Tr>
                  <Th>Account Code</Th>
                  <Th>Account Name</Th>
                  <Th>Type</Th>
                  <Th isNumeric>Debit Balance</Th>
                  <Th isNumeric>Credit Balance</Th>
                </Tr>
              </Thead>
              <Tbody>
                {data.accounts.map((account, index) => (
                  <Tr key={index}>
                    <Td fontSize="sm">{account.accountCode}</Td>
                    <Td fontSize="sm">{account.accountName}</Td>
                    <Td fontSize="sm">
                      <Badge size="sm" colorScheme="blue">
                        {account.accountType}
                      </Badge>
                    </Td>
                    <Td isNumeric fontSize="sm">
                      {account.debitBalance > 0 ? 
                        financialReportService.formatCurrency(account.debitBalance) : 
                        '-'
                      }
                    </Td>
                    <Td isNumeric fontSize="sm">
                      {account.creditBalance > 0 ? 
                        financialReportService.formatCurrency(account.creditBalance) : 
                        '-'
                      }
                    </Td>
                  </Tr>
                ))}
              </Tbody>
              <Tfoot>
                <Tr bg="gray.100">
                  <Th colSpan={3} textAlign="right">TOTAL</Th>
                  <Th isNumeric fontWeight="bold" color="blue.600">
                    {financialReportService.formatCurrency(data.totalDebits)}
                  </Th>
                  <Th isNumeric fontWeight="bold" color="green.600">
                    {financialReportService.formatCurrency(data.totalCredits)}
                  </Th>
                </Tr>
              </Tfoot>
            </Table>
          </TableContainer>
        </CardBody>
      </Card>
    </VStack>
  );
};

// General Ledger Viewer
const GeneralLedgerView: React.FC<{ data: GeneralLedger }> = ({ data }) => {
  return (
    <VStack spacing={6} align="stretch">
      {/* Header */}
      <Card>
        <CardHeader>
          <VStack align="start" spacing={2}>
            <Heading size="lg" display="flex" alignItems="center">
              <Icon as={FiDollarSign} mr={2} />
              {data.reportHeader.reportTitle}
            </Heading>
            <Text color="gray.600">
              {data.account.accountCode} - {data.account.accountName}
            </Text>
            <Text color="gray.500">
              {financialReportService.formatDateRange(data.reportHeader.startDate, data.reportHeader.endDate)}
            </Text>
          </VStack>
        </CardHeader>
      </Card>

      {/* Account Summary */}
      <SimpleGrid columns={{ base: 2, md: 4 }} spacing={4}>
        <Stat>
          <StatLabel>Beginning Balance</StatLabel>
          <StatNumber>{financialReportService.formatCurrency(data.beginningBalance)}</StatNumber>
        </Stat>
        
        <Stat>
          <StatLabel>Total Debits</StatLabel>
          <StatNumber color="blue.500">
            {financialReportService.formatCurrency(data.totalDebits)}
          </StatNumber>
        </Stat>

        <Stat>
          <StatLabel>Total Credits</StatLabel>
          <StatNumber color="green.500">
            {financialReportService.formatCurrency(data.totalCredits)}
          </StatNumber>
        </Stat>

        <Stat>
          <StatLabel>Ending Balance</StatLabel>
          <StatNumber color="purple.500">
            {financialReportService.formatCurrency(data.endingBalance)}
          </StatNumber>
        </Stat>
      </SimpleGrid>

      {/* Transactions */}
      <Card>
        <CardHeader>
          <Heading size="md">Transaction History</Heading>
        </CardHeader>
        <CardBody>
          <TableContainer>
            <Table size="sm">
              <Thead>
                <Tr>
                  <Th>Date</Th>
                  <Th>Journal</Th>
                  <Th>Description</Th>
                  <Th>Reference</Th>
                  <Th isNumeric>Debit</Th>
                  <Th isNumeric>Credit</Th>
                  <Th isNumeric>Balance</Th>
                </Tr>
              </Thead>
              <Tbody>
                <Tr bg="gray.50">
                  <Td colSpan={6} fontWeight="bold">Beginning Balance</Td>
                  <Td isNumeric fontWeight="bold">
                    {financialReportService.formatCurrency(data.beginningBalance)}
                  </Td>
                </Tr>
                {data.transactions.map((transaction, index) => (
                  <Tr key={index}>
                    <Td fontSize="sm">
                      {financialReportService.formatDate(transaction.date)}
                    </Td>
                    <Td fontSize="sm">{transaction.journalCode}</Td>
                    <Td fontSize="sm">{transaction.description}</Td>
                    <Td fontSize="sm">{transaction.reference}</Td>
                    <Td isNumeric fontSize="sm">
                      {transaction.debitAmount > 0 ? 
                        financialReportService.formatCurrency(transaction.debitAmount) : 
                        '-'
                      }
                    </Td>
                    <Td isNumeric fontSize="sm">
                      {transaction.creditAmount > 0 ? 
                        financialReportService.formatCurrency(transaction.creditAmount) : 
                        '-'
                      }
                    </Td>
                    <Td isNumeric fontSize="sm" fontWeight="semibold">
                      {financialReportService.formatCurrency(transaction.balance)}
                    </Td>
                  </Tr>
                ))}
              </Tbody>
            </Table>
          </TableContainer>
        </CardBody>
      </Card>
    </VStack>
  );
};

export default FinancialReportViewer;
