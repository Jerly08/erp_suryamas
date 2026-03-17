import React, { useState } from 'react';
import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalFooter,
  ModalBody,
  ModalCloseButton,
  Box,
  Text,
  VStack,
  HStack,
  Button,
  SimpleGrid,
  Badge,
  Flex,
  useColorModeValue,
  Grid,
  GridItem,
  Stat,
  StatLabel,
  StatNumber,
  StatHelpText,
  StatArrow,
  Card,
  CardBody,
  CardHeader,
  Heading,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  useToast,
  Spinner,
  Icon
} from '@chakra-ui/react';
import { FiDownload, FiShoppingCart, FiDollarSign, FiPieChart, FiUsers, FiTrendingUp, FiFilePlus, FiFileText } from 'react-icons/fi';
import { 
  FormControl,
  FormLabel,
  Input
} from '@chakra-ui/react';
import { formatCurrency } from '../../utils/formatters';
import { SSOTSalesSummaryData, CustomerSalesData, ProductSalesData, SaleItemDetail } from '../../services/ssotSalesSummaryService';
import { useTranslation } from '@/hooks/useTranslation';

interface SalesSummaryModalProps {
  isOpen: boolean;
  onClose: () => void;
  data: SSOTSalesSummaryData | null;
  isLoading: boolean;
  error: string | null;
  startDate: string;
  endDate: string;
  onDateChange?: (startDate: string, endDate: string) => void;
  onFetch?: () => void;
  onExport?: (format: 'pdf' | 'excel') => void;
}

const SalesSummaryModal: React.FC<SalesSummaryModalProps> = ({
  isOpen,
  onClose,
  data,
  isLoading,
  error,
  startDate,
  endDate,
  onDateChange,
  onFetch,
  onExport
}) => {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState<'summary'>('summary');
  const toast = useToast();
  
  // Color mode values
  const modalBg = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const sectionBg = useColorModeValue('gray.50', 'gray.700');
  const textColor = useColorModeValue('gray.800', 'white');
  const secondaryTextColor = useColorModeValue('gray.600', 'gray.300');
  const loadingTextColor = useColorModeValue('gray.700', 'gray.300');
  const previewPeriodTextColor = useColorModeValue('gray.500', 'gray.400');
  const inputBg = useColorModeValue('white', 'whiteAlpha.100');
  const inputBorderColor = useColorModeValue('gray.300', 'whiteAlpha.300');
  const inputHoverBorderColor = useColorModeValue('blue.400', 'blue.300');
  const inputFocusBorderColor = useColorModeValue('blue.500', 'blue.200');
  const errorBg = useColorModeValue('red.50', 'rgba(127, 29, 29, 0.32)');
  const errorBorderColor = useColorModeValue('red.200', 'rgba(252, 129, 129, 0.32)');
  const companyHeaderBg = useColorModeValue('blue.50', 'rgba(66, 153, 225, 0.16)');
  const companyHeaderBorderColor = useColorModeValue('blue.200', 'rgba(144, 205, 244, 0.24)');
  const companyHeaderTextColor = useColorModeValue('blue.800', 'blue.100');
  const companyHeaderSubtextColor = useColorModeValue('blue.600', 'blue.200');
  const positiveStatBg = useColorModeValue('green.50', 'rgba(72, 187, 120, 0.16)');
  const primaryStatBg = useColorModeValue('blue.50', 'rgba(66, 153, 225, 0.16)');
  const accentStatBg = useColorModeValue('purple.50', 'rgba(159, 122, 234, 0.18)');
  const tealStatBg = useColorModeValue('teal.50', 'rgba(56, 178, 172, 0.16)');
  const rowCardBg = useColorModeValue('white', 'whiteAlpha.100');
  const rowHoverBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const tableHeaderBg = useColorModeValue('blue.50', 'rgba(66, 153, 225, 0.16)');
  const tableHeaderBorderColor = useColorModeValue('blue.200', 'rgba(144, 205, 244, 0.24)');
  const subtableHeaderBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const subtableHeaderTextColor = useColorModeValue('gray.700', 'gray.200');
  const itemDividerColor = useColorModeValue('gray.100', 'whiteAlpha.200');
  const fieldStyles = {
    bg: inputBg,
    borderColor: inputBorderColor,
    _hover: { borderColor: inputHoverBorderColor },
    _focus: {
      borderColor: inputFocusBorderColor,
      boxShadow: `0 0 0 1px ${inputFocusBorderColor}`,
    },
  };
  
  const handleExport = (format: 'pdf' | 'excel') => {
    if (onExport) {
      onExport(format);
    } else {
      if (data) {
        // Fallback: download as JSON
        const reportData = {
          reportType: 'Sales Summary',
          period: `${startDate} to ${endDate}`,
          generatedOn: new Date().toISOString(),
          data: data
        };
        const dataStr = JSON.stringify(reportData, null, 2);
        const dataBlob = new Blob([dataStr], { type: 'application/json' });
        const url = URL.createObjectURL(dataBlob);
        const link = document.createElement('a');
        link.href = url;
        link.download = `sales-summary-${startDate}-to-${endDate}.json`;
        link.click();
        URL.revokeObjectURL(url);
      }
      
      toast({
        title: t('reports.modal.exportFeature'),
        description: t('reports.modal.exportComingSoon', { format: format.toUpperCase() }),
        status: 'info',
        duration: 3000,
        isClosable: true,
      });
    }
  };

  const renderSummaryMetrics = () => {
    if (!data) return null;
    
    // Calculate total customers from sales_by_customer array
    const totalCustomers = data.sales_by_customer ? data.sales_by_customer.length : 0;
    
    // Calculate total orders from transaction counts
    const totalOrders = data.sales_by_customer 
      ? data.sales_by_customer.reduce((sum, customer) => sum + (customer.transaction_count || 0), 0)
      : 0;
    
    // Calculate average order value
    const averageOrderValue = totalOrders > 0 ? (data.total_revenue || data.total_sales || 0) / totalOrders : 0;
    
    return (
      <Grid templateColumns="repeat(auto-fit, minmax(240px, 1fr))" gap={4} mb={6}>
        <GridItem>
          <Card size="sm">
            <CardBody>
              <Stat>
                <StatLabel>{t('reports.modal.totalRevenue')}</StatLabel>
                <StatNumber color="green.600">
                  {formatCurrency(data.total_revenue || data.total_sales || 0)}
                </StatNumber>
                <StatHelpText>
                  <StatArrow type="increase" />
                  {t('reports.modal.salesForPeriod')}
                </StatHelpText>
              </Stat>
            </CardBody>
          </Card>
        </GridItem>
        
        <GridItem>
          <Card size="sm">
            <CardBody>
              <Stat>
                <StatLabel>{t('reports.modal.totalCustomers')}</StatLabel>
                <StatNumber color="blue.600">
                  {totalCustomers}
                </StatNumber>
                <StatHelpText>
                  <Icon as={FiUsers} />
                  {t('reports.modal.activeCustomers')}
                </StatHelpText>
              </Stat>
            </CardBody>
          </Card>
        </GridItem>
        
        {totalOrders > 0 && (
          <GridItem>
            <Card size="sm">
              <CardBody>
                <Stat>
                  <StatLabel>{t('reports.modal.totalOrders')}</StatLabel>
                  <StatNumber color="purple.600">
                    {totalOrders}
                  </StatNumber>
                  <StatHelpText>
                    <Icon as={FiShoppingCart} />
                    {t('reports.modal.ordersProcessed')}
                  </StatHelpText>
                </Stat>
              </CardBody>
            </Card>
          </GridItem>
        )}
        
        {averageOrderValue > 0 && (
          <GridItem>
            <Card size="sm">
              <CardBody>
                <Stat>
                  <StatLabel>{t('reports.modal.averageOrderValue')}</StatLabel>
                  <StatNumber color="orange.600">
                    {formatCurrency(averageOrderValue)}
                  </StatNumber>
                  <StatHelpText>
                    <Icon as={FiTrendingUp} />
                    {t('reports.modal.perOrderAverage')}
                  </StatHelpText>
                </Stat>
              </CardBody>
            </Card>
          </GridItem>
        )}
      </Grid>
    );
  };

  // Note: This function is currently unused as the Customers tab has been removed
  const renderCustomersTab = () => {
    if (!data) return null;

    const customers = data.sales_by_customer || [];
    // For top customers, we'll sort the sales_by_customer by total_sales
    const topCustomers = [...customers]
      .sort((a, b) => (b.total_sales || 0) - (a.total_sales || 0))
      .slice(0, 6);

    return (
      <VStack spacing={6} align="stretch">
        {customers.length > 0 && (
          <Card>
            <CardHeader>
              <Heading size="sm">Sales by Customer</Heading>
            </CardHeader>
            <CardBody>
              <Table size="sm">
                <Thead>
                  <Tr>
                    <Th>Customer</Th>
                    <Th isNumeric>Total Sales</Th>
                    <Th isNumeric>Orders</Th>
                    <Th isNumeric>Avg Order</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {customers.map((customer: CustomerSalesData, index: number) => (
                    <Tr key={index}>
                      <Td>
                        <VStack align="start" spacing={1}>
                          <Text fontWeight="medium">
                            {customer.customer_name || 'Unnamed Customer'}
                          </Text>
                        </VStack>
                      </Td>
                      <Td isNumeric>
                        <Text fontWeight="bold" color="green.600">
                          {formatCurrency(customer.total_sales || 0)}
                        </Text>
                      </Td>
                      <Td isNumeric>
                        <Text color="purple.600">
                          {customer.transaction_count || 0}
                        </Text>
                      </Td>
                      <Td isNumeric>
                        <Text color="orange.600">
                          {formatCurrency(
                            customer.average_transaction ||
                            (customer.total_sales && customer.transaction_count > 0 
                              ? customer.total_sales / customer.transaction_count 
                              : 0)
                          )}
                        </Text>
                      </Td>
                    </Tr>
                  ))}
                </Tbody>
              </Table>
            </CardBody>
          </Card>
        )}

        {topCustomers.length > 0 && (
          <Card>
            <CardHeader>
              <Heading size="sm">Top Performing Customers</Heading>
            </CardHeader>
            <CardBody>
              <SimpleGrid columns={[1, 2, 3]} spacing={4}>
                {topCustomers.map((customer: CustomerSalesData, index: number) => (
                  <Box key={index} border="1px" borderColor={borderColor} borderRadius="md" p={4}>
                    <VStack spacing={3}>
                      <Badge colorScheme="gold" size="lg" variant="solid">
                        #{index + 1}
                      </Badge>
                      <Text fontWeight="bold" fontSize="md" textAlign="center">
                        {customer.customer_name}
                      </Text>
                      <Text fontSize="lg" fontWeight="bold" color="green.600">
                        {formatCurrency(customer.total_sales)}
                      </Text>
            <Text fontSize="sm" color={previewPeriodTextColor}>
                        {((customer.total_sales / (data.total_revenue || data.total_sales || 1)) * 100).toFixed(1)}% of total
                      </Text>
                      {customer.transaction_count && (
                        <Text fontSize="xs" color="purple.500">
                          {customer.transaction_count} orders
                        </Text>
                      )}
                    </VStack>
                  </Box>
                ))}
              </SimpleGrid>
            </CardBody>
          </Card>
        )}

        {customers.length === 0 && topCustomers.length === 0 && (
          <Box textAlign="center" py={8}>
            <Text color={secondaryTextColor}>
              No customer data available for this period
            </Text>
          </Box>
        )}
      </VStack>
    );
  };

  // Note: This function is currently unused as the Analysis tab has been removed
  const renderAnalysisTab = () => {
    if (!data) return null;

    // Calculate some basic trends from the data
    const totalCustomers = data.sales_by_customer ? data.sales_by_customer.length : 0;
    const totalOrders = data.sales_by_customer 
      ? data.sales_by_customer.reduce((sum, customer) => sum + (customer.transaction_count || 0), 0)
      : 0;
    
    // Calculate growth rate (simplified)
    const growthRate = totalOrders > 0 ? ((data.total_revenue || data.total_sales || 0) / totalOrders) : 0;
    
    return (
      <VStack spacing={6} align="stretch">
        <Card>
          <CardHeader>
            <Heading size="sm">Sales Performance Analysis</Heading>
          </CardHeader>
          <CardBody>
            <SimpleGrid columns={[1, 2, 4]} spacing={4}>
              <Box p={4} bg={positiveStatBg} borderRadius="md" textAlign="center">
                <Text fontSize="2xl" fontWeight="bold" color="green.600">
                  {growthRate.toFixed(1)}%
                </Text>
                <Text fontSize="sm" color="green.800">
                  Growth Rate
                </Text>
              </Box>
              
              <Box p={4} bg={primaryStatBg} borderRadius="md" textAlign="center">
                <Text fontSize="2xl" fontWeight="bold" color="blue.600">
                  {totalCustomers}
                </Text>
                <Text fontSize="sm" color="blue.800">
                  Total Customers
                </Text>
              </Box>
              
              <Box p={4} bg={accentStatBg} borderRadius="md" textAlign="center">
                <Text fontSize="2xl" fontWeight="bold" color="purple.600">
                  {totalOrders}
                </Text>
                <Text fontSize="sm" color="purple.800">
                  Total Orders
                </Text>
              </Box>
              
              <Box p={4} bg={tealStatBg} borderRadius="md" textAlign="center">
                <Text fontSize="2xl" fontWeight="bold" color="teal.600">
                  {data.sales_by_customer && data.sales_by_customer.length > 0 
                    ? Math.round(data.sales_by_customer.reduce((sum, customer) => sum + (customer.transaction_count || 0), 0) / data.sales_by_customer.length)
                    : 0}
                </Text>
                <Text fontSize="sm" color="teal.800">
                  Avg Orders/Customer
                </Text>
              </Box>
            </SimpleGrid>
          </CardBody>
        </Card>
        
        <Card>
          <CardHeader>
            <Heading size="sm">Key Insights</Heading>
          </CardHeader>
          <CardBody>
            <VStack spacing={3} align="stretch">
              <Text fontSize="sm">
                📈 <strong>Sales Performance:</strong> {growthRate > 0 ? 'Your sales are growing positively' : 'Sales performance shows room for improvement'}
              </Text>
              <Text fontSize="sm">
                👥 <strong>Customer Base:</strong> {totalCustomers} active customers indicate {totalCustomers > 10 ? 'strong' : 'developing'} customer loyalty
              </Text>
              <Text fontSize="sm">
                🎯 <strong>Market Expansion:</strong> {totalOrders} total orders shows business activity level
              </Text>
            </VStack>
          </CardBody>
        </Card>
      </VStack>
    );
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} size="6xl" scrollBehavior="inside">
      <ModalOverlay />
      <ModalContent bg={modalBg} maxW="90vw">
        <ModalHeader>
          <HStack>
            <Icon as={FiShoppingCart} color="blue.500" />
            <VStack align="start" spacing={0}>
              <Text fontSize="lg" fontWeight="bold">
                {t('reports.modal.salesSummaryTitle')}
              </Text>
              <Text fontSize="sm" color={previewPeriodTextColor}>
                {startDate} - {endDate} | {t('reports.modal.ssotJournalIntegration')}
              </Text>
            </VStack>
          </HStack>
        </ModalHeader>
        <ModalCloseButton />
        
        <ModalBody pb={6} px={8}>
          {/* Date Range Controls - Moved to top like other modals */}
          <Box mb={4}>
            <HStack spacing={4} mb={4} flexWrap="wrap">
              <FormControl>
                <FormLabel>{t('reports.modal.startDate')}</FormLabel>
                <Input 
                  type="date" 
                  value={startDate} 
                  onChange={(e) => onDateChange && onDateChange(e.target.value, endDate)} 
                  {...fieldStyles}
                />
              </FormControl>
              <FormControl>
                <FormLabel>{t('reports.modal.endDate')}</FormLabel>
                <Input 
                  type="date" 
                  value={endDate} 
                  onChange={(e) => onDateChange && onDateChange(startDate, e.target.value)} 
                  {...fieldStyles}
                />
              </FormControl>
              <Button
                colorScheme="blue"
                onClick={onFetch}
                isLoading={isLoading}
                leftIcon={<FiShoppingCart />}
                size="md"
                mt={8}
                whiteSpace="nowrap"
              >
                {t('reports.modal.generateReport')}
              </Button>
            </HStack>
          </Box>

          {isLoading && (
            <Box textAlign="center" py={8}>
              <VStack spacing={4}>
                <Spinner size="xl" thickness="4px" speed="0.65s" color="blue.500" />
                <VStack spacing={2}>
                  <Text fontSize="lg" fontWeight="medium" color={loadingTextColor}>
                    {t('reports.modal.generatingSalesSummary')}
                  </Text>
                  <Text fontSize="sm" color={secondaryTextColor}>
                    {t('reports.modal.analyzingSalesTransactions')}
                  </Text>
                </VStack>
              </VStack>
            </Box>
          )}

          {error && (
            <Box bg={errorBg} p={4} borderRadius="md" mb={4} border="1px" borderColor={errorBorderColor}>
              <Text color="red.300" fontWeight="medium">Error: {error}</Text>
              <Button
                mt={2}
                size="sm"
                colorScheme="red"
                variant="outline"
                onClick={onFetch}
              >
                {t('reports.modal.retry')}
              </Button>
            </Box>
          )}

          {data && !isLoading && (
            <VStack spacing={6} align="stretch">
              {/* Company Header */}
              {data.company && (
                <Box bg={companyHeaderBg} p={4} borderRadius="md" border="1px" borderColor={companyHeaderBorderColor}>
                  <HStack justify="space-between" align="start">
                    <VStack align="start" spacing={1}>
                      <Text fontSize="lg" fontWeight="bold" color={companyHeaderTextColor}>
                        {data.company.name || 'Company Name Not Set'}
                      </Text>
                      <Text fontSize="sm" color={companyHeaderSubtextColor}>
                        {data.company.address ? (
                          data.company.city ? `${data.company.address}, ${data.company.city}` : data.company.address
                        ) : 'Address not available'}
                      </Text>
                      {data.company.phone && (
                        <Text fontSize="sm" color={companyHeaderSubtextColor}>
                          {data.company.phone} | {data.company.email}
                        </Text>
                      )}
                    </VStack>
                    <VStack align="end" spacing={1}>
                      <Text fontSize="sm" color={companyHeaderSubtextColor}>
                        {t('reports.modal.currency')}: {data.currency || 'IDR'}
                      </Text>
                      <Text fontSize="xs" color={companyHeaderSubtextColor}>
                        {t('reports.modal.generated')}: {data.generated_at ? new Date(data.generated_at).toLocaleString('id-ID') : new Date().toLocaleString('id-ID')}
                      </Text>
                    </VStack>
                  </HStack>
                </Box>
              )}

              {/* Report Header */}
              <Box textAlign="center" bg={sectionBg} p={4} borderRadius="md">
                <Heading size="md" color={textColor}>
                  {t('reports.modal.salesSummaryReport')}
                </Heading>
                <Text fontSize="sm" color={secondaryTextColor}>
                  {t('reports.modal.period')}: {startDate} - {endDate}
                </Text>
                <Text fontSize="xs" color={secondaryTextColor} mt={1}>
                  {t('reports.modal.generated')}: {new Date().toLocaleDateString('id-ID')} at {new Date().toLocaleTimeString('id-ID')}
                </Text>
              </Box>

              {renderSummaryMetrics()}
              
              {/* Period Summary */}
              <Card>
                <CardBody>
                  <Flex justify="space-between" align="center" mb={3}>
                    <Heading size="md" color={textColor}>
                      {t('reports.modal.salesPerformance')}
                    </Heading>
                    <Text fontWeight="bold" fontSize="lg" color="green.600">
                      {formatCurrency(data.total_revenue || data.total_sales || 0)}
                    </Text>
                  </Flex>
                  
                  <SimpleGrid columns={[1, 3]} spacing={4}>
                    <Box textAlign="center" p={3} bg={sectionBg} borderRadius="md">
                      <Text fontSize="sm" color={secondaryTextColor}>{t('reports.modal.period')}</Text>
                      <Text fontWeight="medium">{startDate} to {endDate}</Text>
                    </Box>
                    <Box textAlign="center" p={3} bg={sectionBg} borderRadius="md">
                      <Text fontSize="sm" color={secondaryTextColor}>{t('reports.modal.reportType')}</Text>
                      <Text fontWeight="medium">{t('reports.modal.ssotIntegration')}</Text>
                    </Box>
                    <Box textAlign="center" p={3} bg={sectionBg} borderRadius="md">
                      <Text fontSize="sm" color={secondaryTextColor}>{t('reports.modal.status')}</Text>
                      <Badge colorScheme="green">{t('reports.modal.active')}</Badge>
                    </Box>
                  </SimpleGrid>
                </CardBody>
              </Card>

              {/* Top Customers Section */}
              {data.sales_by_customer && data.sales_by_customer.length > 0 && (
                <Box>
                  <Heading size="sm" mb={4} color={textColor}>
                    {t('reports.modal.topPerformingCustomers')} ({Math.min(data.sales_by_customer.length, 6)} {t('reports.modal.customers')})
                  </Heading>
                  <SimpleGrid columns={[1, 2, 3]} spacing={4}>
                    {[...data.sales_by_customer]
                      .sort((a, b) => (b.total_sales || 0) - (a.total_sales || 0))
                      .slice(0, 6)
                      .map((customer: CustomerSalesData, index: number) => (
                        <Box key={index} border="1px" borderColor={borderColor} borderRadius="md" p={4} bg={rowCardBg}>
                          <VStack spacing={3}>
                            <Badge colorScheme="blue" size="lg" variant="solid">
                              #{index + 1}
                            </Badge>
                            <Text fontWeight="bold" fontSize="md" textAlign="center" color={textColor}>
                              {customer.customer_name}
                            </Text>
                            <Text fontSize="lg" fontWeight="bold" color="green.600">
                              {formatCurrency(customer.total_sales)}
                            </Text>
                            <Text fontSize="sm" color={secondaryTextColor}>
                              {((customer.total_sales / (data.total_revenue || data.total_sales || 1)) * 100).toFixed(1)}% {t('reports.modal.ofTotal')}
                            </Text>
                            {customer.transaction_count && (
                              <Text fontSize="xs" color="purple.600">
                                {customer.transaction_count} {t('reports.modal.orders')}
                              </Text>
                            )}
                          </VStack>
                        </Box>
                      ))}
                  </SimpleGrid>
                </Box>
              )}

              {/* Sales by Customer Table */}
              {data.sales_by_customer && data.sales_by_customer.length > 0 && (
                <Box>
                  <Heading size="sm" mb={4} color={textColor}>
                    {t('reports.modal.salesByCustomer')} ({data.sales_by_customer.length} {t('reports.modal.customers')})
                  </Heading>
                  
                  {/* Customer Table Header */}
                  <Box bg={tableHeaderBg} p={3} borderRadius="md" mb={2} border="1px solid" borderColor={tableHeaderBorderColor}>
                    <SimpleGrid columns={[1, 3]} spacing={2} fontSize="sm" fontWeight="bold" color={companyHeaderTextColor}>
                      <Text>{t('reports.modal.customer')}</Text>
                      <Text textAlign="right">{t('reports.modal.totalSales')}</Text>
                      <Text textAlign="right">{t('reports.modal.orders')}</Text>
                    </SimpleGrid>
                  </Box>
                  
                  {/* Customer Rows */}
                  <VStack spacing={2} align="stretch" maxH="400px" overflow="auto">
                    {data.sales_by_customer.map((customer: CustomerSalesData, index: number) => (
                      <Box key={index} border="1px solid" borderColor={borderColor} borderRadius="md" p={4} bg={rowCardBg} _hover={{ bg: rowHoverBg }}>
                        <SimpleGrid columns={[1, 3]} spacing={2} fontSize="sm">
                          <VStack align="start" spacing={1}>
                            <Text fontWeight="bold" fontSize="md" color={textColor}>
                              {customer.customer_name || 'Unnamed Customer'}
                            </Text>
                          </VStack>
                          <Text textAlign="right" fontSize="sm" fontWeight="bold" color="green.600">
                            {formatCurrency(customer.total_sales || 0)}
                          </Text>
                          <Text textAlign="right" fontSize="sm" fontWeight="medium" color="purple.600">
                            {customer.transaction_count || 0}
                          </Text>
                        </SimpleGrid>
                      </Box>
                    ))}
                  </VStack>
                </Box>
              )}

              {/* Sale Items Detail */}
              {data.sales_by_customer && data.sales_by_customer.some(c => c.items && c.items.length > 0) && (
                <Box>
                  <Heading size="sm" mb={4} color={textColor}>
                    {t('reports.modal.itemsSales')}
                  </Heading>
                  
                  <VStack spacing={4} align="stretch">
                    {data.sales_by_customer
                      .filter(customer => customer.items && customer.items.length > 0)
                      .map((customer: CustomerSalesData, customerIndex: number) => (
                        <Card key={customerIndex}>
                          <CardHeader bg={tableHeaderBg} py={3}>
                            <HStack justify="space-between">
                              <Heading size="xs" color={companyHeaderTextColor}>
                                {customer.customer_name}
                              </Heading>
                              <Badge colorScheme="blue">
                                {customer.items?.length || 0} {t('reports.modal.items')}
                              </Badge>
                            </HStack>
                          </CardHeader>
                          <CardBody>
                            {/* Items Table Header */}
                            <Box bg={subtableHeaderBg} p={2} borderRadius="md" mb={2}>
                              <SimpleGrid columns={[1, 5]} spacing={2} fontSize="xs" fontWeight="bold" color={subtableHeaderTextColor}>
                                <Text>{t('reports.modal.product')}</Text>
                                <Text textAlign="right">{t('reports.modal.qty')}</Text>
                                <Text textAlign="right">{t('reports.modal.unitPrice')}</Text>
                                <Text textAlign="right">{t('reports.modal.total')}</Text>
                                <Text textAlign="center">{t('reports.modal.date')}</Text>
                              </SimpleGrid>
                            </Box>
                            
                            {/* Items Rows */}
                            <VStack spacing={1} align="stretch">
                              {customer.items?.map((item: SaleItemDetail, itemIndex: number) => (
                                <Box 
                                  key={itemIndex} 
                                  borderBottom="1px solid" 
                                  borderColor={itemDividerColor} 
                                  py={2}
                                  _hover={{ bg: rowHoverBg }}
                                >
                                  <SimpleGrid columns={[1, 5]} spacing={2} fontSize="sm">
                                    <VStack align="start" spacing={0}>
                                      <Text fontWeight="medium" color={textColor}>
                                        {item.product_name}
                                      </Text>
                                      <Text fontSize="xs" color={previewPeriodTextColor}>
                                        {item.product_code}
                                      </Text>
                                    </VStack>
                                    <Text textAlign="right" color="purple.600">
                                      {item.quantity} {item.unit}
                                    </Text>
                                    <Text textAlign="right" color={textColor}>
                                      {formatCurrency(item.unit_price)}
                                    </Text>
                                    <Text textAlign="right" fontWeight="bold" color="green.600">
                                      {formatCurrency(item.total_price)}
                                    </Text>
                                    <Text textAlign="center" fontSize="xs" color={secondaryTextColor}>
                                      {new Date(item.sale_date).toLocaleDateString('id-ID')}
                                    </Text>
                                  </SimpleGrid>
                                </Box>
                              ))}
                            </VStack>
                            
                            {/* Customer Total */}
                            <Box mt={3} pt={3} borderTop="2px solid" borderColor={tableHeaderBorderColor}>
                              <Flex justify="space-between" align="center">
                                <Text fontWeight="bold" color={textColor}>
                                  {t('reports.modal.subtotal')} ({customer.customer_name})
                                </Text>
                                <Text fontWeight="bold" fontSize="lg" color="blue.600">
                                  {formatCurrency(customer.total_sales)}
                                </Text>
                              </Flex>
                            </Box>
                          </CardBody>
                        </Card>
                      ))}
                  </VStack>
                </Box>
              )}
            </VStack>
          )}
        </ModalBody>

        <ModalFooter>
          <HStack spacing={3}>
            {data && !isLoading && (
              <>
                <Button
                  colorScheme="red"
                  variant="outline"
                  size="sm"
                  leftIcon={<FiFilePlus />}
                  onClick={() => handleExport('pdf')}
                >
                  {t('reports.modal.exportPDF')}
                </Button>
                <Button
                  colorScheme="green"
                  variant="outline"
                  size="sm"
                  leftIcon={<FiFileText />}
                  onClick={() => handleExport('excel')}
                >
                  {t('reports.modal.exportCSV')}
                </Button>
              </>
            )}
          </HStack>
          <Button variant="ghost" onClick={onClose}>
            {t('reports.modal.close')}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
};

export default SalesSummaryModal;
