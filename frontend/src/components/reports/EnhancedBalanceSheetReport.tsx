'use client';

import React, { useState } from 'react';
import {
  Box,
  VStack,
  HStack,
  Text,
  Button,
  Card,
  CardBody,
  CardHeader,
  Icon,
  Badge,
  SimpleGrid,
  useToast,
  Spinner,
  FormControl,
  FormLabel,
  Input,
  Switch,
  Divider,
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalFooter,
  ModalBody,
  ModalCloseButton,
  useDisclosure,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  Tooltip,
  useColorModeValue,
  Select,
} from '@chakra-ui/react';
import {
  FiBarChart,
  FiDownload,
  FiFileText,
  FiFilePlus,
  FiRefreshCw,
  FiSettings,
  FiCheckCircle,
  FiAlertTriangle,
  FiInfo,
} from 'react-icons/fi';

// Import services and utilities
import {
  ssotBalanceSheetReportService,
  SSOTBalanceSheetData
} from '../../services/ssotBalanceSheetReportService';
import {
  exportAndDownloadCSV,
  exportAndDownloadPDF
} from '../../utils/balanceSheetExportClient';
import {
  periodClosingService,
  PeriodFilterOption
} from '../../services/periodClosingService';
import { useTranslation } from '@/hooks/useTranslation';

interface EnhancedBalanceSheetReportProps {
  onClose?: () => void;
}

export const EnhancedBalanceSheetReport: React.FC<EnhancedBalanceSheetReportProps> = ({
  onClose
}) => {
  // State management
  const [balanceSheetData, setBalanceSheetData] = useState<SSOTBalanceSheetData | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [asOfDate, setAsOfDate] = useState(new Date().toISOString().split('T')[0]);
  const [includeAccountDetails, setIncludeAccountDetails] = useState(true);
  const [companyName, setCompanyName] = useState('Company Name Not Set');

  // Export states
  const [exportingCSV, setExportingCSV] = useState(false);
  const [exportingPDF, setExportingPDF] = useState(false);

  // Closed Period states
  const [closedPeriods, setClosedPeriods] = useState<PeriodFilterOption[]>([]);
  const [loadingPeriods, setLoadingPeriods] = useState(false);
  const [periodOptionsLoaded, setPeriodOptionsLoaded] = useState(false);

  // Modal controls
  const {
    isOpen: isExportModalOpen,
    onOpen: onExportModalOpen,
    onClose: onExportModalClose


  } = useDisclosure();

  const toast = useToast();
  const { t } = useTranslation();

  // Color mode values
  const cardBg = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const headingColor = useColorModeValue('gray.700', 'white');
  const textColor = useColorModeValue('gray.800', 'white');
  const descriptionColor = useColorModeValue('gray.600', 'gray.300');

  // Fetch closed periods for dropdown (lazy loading)
  const fetchClosedPeriods = async () => {
    if (periodOptionsLoaded) return; // Already loaded

    setLoadingPeriods(true);
    try {
      const options = await periodClosingService.getClosedPeriodsForFilter();
      setClosedPeriods(options);
      setPeriodOptionsLoaded(true);

      if (options.length === 0) {
        console.log('[EnhancedBalanceSheetReport] No closed periods found');
      } else {
        console.log(`[EnhancedBalanceSheetReport] Loaded ${options.length} closed periods`);
      }
    } catch (error) {
      console.error('Error loading closed periods:', error);
      // Silent fail - user can still enter date manually
    } finally {
      setLoadingPeriods(false);
    }
  };

  // Format currency for display with account type consideration
  const formatCurrency = (amount: number | null | undefined, accountCode?: string, accountType?: string) => {
    if (amount === null || amount === undefined || isNaN(Number(amount))) {
      return 'Rp 0';
    }

    let displayAmount = Number(amount);

    // Revenue accounts (4xxx) should NOT appear on a Balance Sheet — they belong on P&L.
    // If they somehow appear, display as-is without masking sign.
    // PPN accounts are handled via PPN_NET netting in the backend, so no special conversion needed here.

    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0
    }).format(displayAmount);
  };

  // Generate Balance Sheet Report
  const generateBalanceSheetReport = async () => {
    setLoading(true);
    setError(null);

    try {
      const data = await ssotBalanceSheetReportService.generateSSOTBalanceSheet({
        as_of_date: asOfDate,
        format: 'json'
      });

      setBalanceSheetData(data);

      toast({
        title: t('reports.modal.status.success'),
        description: t('reports.ssotBalanceSheetGenerated'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });

    } catch (error: unknown) {
      const errorMessage = error instanceof Error ? error.message : 'Failed to generate Balance Sheet';
      setError(errorMessage);

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
  };

  // Export CSV function
  const handleExportCSV = async () => {
    if (!asOfDate) {
      toast({
        title: t('reports.noData'),
        description: t('reports.selectDateFirst'),
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    setExportingCSV(true);

    try {
      console.log('Exporting CSV with asOfDate:', asOfDate);
      // Use the new CSV generation method from the service
      const csvContent = await ssotBalanceSheetReportService.generateSSOTBalanceSheetCSV({
        as_of_date: asOfDate
      });

      console.log('Received CSV content, creating download link');
      // Create download link
      const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `SSOT_BalanceSheet_${asOfDate}.csv`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);

      toast({
        title: t('reports.balanceSheet.csvExportSuccess'),
        description: t('reports.balanceSheet.balanceSheetDownloadedCSV'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });

    } catch (error: unknown) {
      const errorMessage = error instanceof Error ? error.message : 'Failed to export CSV';
      console.error('CSV Export Error:', error); // Log the error for debugging

      toast({
        title: t('reports.balanceSheet.csvExportFailedTitle'),
        description: errorMessage,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setExportingCSV(false);
    }
  };

  // Export PDF function
  const handleExportPDF = async () => {
    if (!asOfDate) {
      toast({
        title: t('reports.noData'),
        description: t('reports.selectDateFirst'),
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    setExportingPDF(true);

    try {
      // Use the new PDF generation method from the service
      const pdfBlob = await ssotBalanceSheetReportService.generateSSOTBalanceSheetPDF({
        as_of_date: asOfDate
      });

      // Create download link
      const url = window.URL.createObjectURL(pdfBlob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `SSOT_BalanceSheet_${asOfDate}.pdf`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);

      toast({
        title: t('reports.balanceSheet.pdfExportSuccess'),
        description: t('reports.balanceSheet.balanceSheetDownloadedPDF'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });

    } catch (error: unknown) {
      const errorMessage = error instanceof Error ? error.message : 'Failed to export PDF';
      toast({
        title: t('reports.balanceSheet.pdfExportFailedTitle'),
        description: errorMessage,
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setExportingPDF(false);
    }
  };

  return (
    <Box p={6}>
      <VStack spacing={6} align="stretch">
        {/* Header */}
        <Card bg={cardBg} border="1px" borderColor={borderColor}>
          <CardHeader pb={3}>
            <HStack justify="space-between">
              <HStack>
                <Icon as={FiBarChart} color="blue.500" boxSize={6} />
                <VStack align="start" spacing={0}>
                  <Text fontSize="xl" fontWeight="bold" color={headingColor}>
                    {t('reports.balanceSheet.ssotBalanceSheet')}
                  </Text>
                  <Text fontSize="sm" color={descriptionColor}>
                    {t('reports.balanceSheet.ssotIntegrationDesc')}
                  </Text>
                </VStack>
              </HStack>
              {onClose && (
                <Button variant="ghost" onClick={onClose}>
                  Close
                </Button>
              )}
            </HStack>
          </CardHeader>

          <CardBody pt={0}>
            <VStack spacing={4} align="stretch">
              {/* Configuration Section */}
              <Box bg={useColorModeValue('gray.50', 'gray.700')} p={4} borderRadius="md">
                <Text fontSize="md" fontWeight="semibold" mb={3} color={headingColor}>
                  {t('reports.balanceSheet.configuration')}
                </Text>
                <SimpleGrid columns={[1, 2]} spacing={4}>
                  <FormControl>
                    <FormLabel fontSize="sm" color="blue.600" fontWeight="semibold">
                      {t('reports.balanceSheet.asOf')}
                    </FormLabel>
                    <Input
                      type="date"
                      value={asOfDate}
                      onChange={(e) => setAsOfDate(e.target.value)}
                      size="sm"
                    />
                  </FormControl>

                  <FormControl>
                    <FormLabel fontSize="sm" display="flex" alignItems="center">
                      {t('reports.balanceSheet.closedPeriod')}
                      <Badge ml={2} colorScheme="blue" fontSize="xs">
                        {t('reports.balanceSheet.quickSelect')}
                      </Badge>
                      <Tooltip label={t('reports.balanceSheet.selectClosedPeriodTooltip')}>
                        <span>
                          <Icon as={FiInfo} ml={1} color="gray.500" boxSize={3} />
                        </span>
                      </Tooltip>
                    </FormLabel>
                    <Select
                      placeholder={t('reports.balanceSheet.selectClosedPeriodOptional')}
                      value=""
                      onChange={(e) => {
                        if (e.target.value) {
                          setAsOfDate(e.target.value);
                          console.log(`[EnhancedBalanceSheetReport] Selected period: ${e.target.value}`);
                        }
                      }}
                      onFocus={fetchClosedPeriods}
                      isDisabled={loadingPeriods}
                      size="sm"
                      icon={loadingPeriods ? <Spinner size="xs" /> : undefined}
                    >
                      {loadingPeriods && (
                        <option disabled>{t('reports.balanceSheet.loadingClosedPeriods')}</option>
                      )}

                      {!loadingPeriods && closedPeriods.length === 0 && (
                        <option disabled>{t('reports.balanceSheet.noClosedPeriodsFound')}</option>
                      )}

                      {!loadingPeriods && closedPeriods.length > 0 && (
                        <>
                          {/* Group by category */}
                          {Array.from(new Set(closedPeriods.map(p => p.group))).map(group => (
                            <optgroup label={group} key={group}>
                              {closedPeriods
                                .filter(p => p.group === group)
                                .map(option => (
                                  <option key={option.value} value={option.value}>
                                    {option.label}
                                  </option>
                                ))}
                            </optgroup>
                          ))}
                        </>
                      )}
                    </Select>
                    <Text fontSize="xs" color={descriptionColor} mt={1}>
                      {t('reports.balanceSheet.selectClosedPeriodTooltip')}
                    </Text>
                  </FormControl>
                </SimpleGrid>
              </Box>

              {/* Action Buttons */}
              <HStack spacing={3} justify="space-between">
                <HStack spacing={3}>
                  <Button
                    colorScheme="blue"
                    leftIcon={<FiRefreshCw />}
                    onClick={generateBalanceSheetReport}
                    isLoading={loading}
                    loadingText={t('common.generating') || 'Generating...'}
                    size="md"
                  >
                    {t('reports.generateReport') || 'Generate Report'}
                  </Button>

                  {balanceSheetData && (
                    <Button
                      variant="outline"
                      leftIcon={<FiSettings />}
                      onClick={onExportModalOpen}
                      size="md"
                    >
                      {t('reports.exportOptions')}
                    </Button>
                  )}
                </HStack>

                {balanceSheetData && (
                  <HStack spacing={2}>
                    <Tooltip label="Export as CSV file">
                      <Button
                        colorScheme="green"
                        variant="outline"
                        leftIcon={<FiFileText />}
                        onClick={handleExportCSV}
                        isLoading={exportingCSV}
                        loadingText={t('reports.exporting')}
                        size="sm"
                      >
                        CSV
                      </Button>
                    </Tooltip>

                    <Tooltip label="Export as PDF file">
                      <Button
                        colorScheme="red"
                        variant="outline"
                        leftIcon={<FiFilePlus />}
                        onClick={handleExportPDF}
                        isLoading={exportingPDF}
                        loadingText={t('reports.exporting')}
                        size="sm"
                      >
                        PDF
                      </Button>
                    </Tooltip>
                  </HStack>
                )}
              </HStack>
            </VStack>
          </CardBody>
        </Card>

        {/* Loading State */}
        {loading && (
          <Card bg={cardBg} border="1px" borderColor={borderColor}>
            <CardBody>
              <VStack spacing={4} py={8}>
                <Spinner size="xl" thickness="4px" speed="0.65s" color="blue.500" />
                <VStack spacing={2}>
                  <Text fontSize="lg" fontWeight="medium" color={headingColor}>
                    {t('reports.balanceSheet.generatingBalanceSheet')}
                  </Text>
                  <Text fontSize="sm" color={descriptionColor}>
                    {t('reports.modal.analyzingFinancialPosition')}
                  </Text>
                </VStack>
              </VStack>
            </CardBody>
          </Card>
        )}

        {/* Error State */}
        {error && (
          <Alert status="error" borderRadius="md">
            <AlertIcon />
            <VStack align="start" spacing={2} flex="1">
              <AlertTitle>Error generating Balance Sheet</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </VStack>
            <Button
              size="sm"
              colorScheme="red"
              variant="outline"
              onClick={generateBalanceSheetReport}
              ml={4}
            >
              Retry
            </Button>
          </Alert>
        )}

        {/* Balance Sheet Data Display */}
        {balanceSheetData && !loading && (
          <VStack spacing={6} align="stretch">
            {/* Report Header */}
            <Card bg={cardBg} border="1px" borderColor={borderColor}>
              <CardBody textAlign="center">
                <VStack spacing={2}>
                  <Text fontSize="xl" fontWeight="bold" color={headingColor}>
                    {balanceSheetData.company?.name || t('reports.balanceSheet.companyName')}
                  </Text>
                  <Text fontSize="lg" fontWeight="semibold">
                    {t('reports.balanceSheet.enhancedTitle')}
                  </Text>
                  <Text fontSize="md" color={descriptionColor}>
                    {t('reports.balanceSheet.asOf')}: {new Date(balanceSheetData.as_of_date).toLocaleDateString('id-ID')}
                  </Text>
                  <Badge
                    colorScheme={balanceSheetData.is_balanced ? 'green' : 'red'}
                    px={3}
                    py={1}
                    borderRadius="full"
                  >
                    {balanceSheetData.is_balanced ? (
                      <>
                        <Icon as={FiCheckCircle} mr={1} />
                        {t('reports.balanceSheet.balancedCheck')}
                      </>
                    ) : (
                      <>
                        <Icon as={FiAlertTriangle} mr={1} />
                        {t('reports.balanceSheet.notBalancedDiff', { amount: formatCurrency(balanceSheetData.balance_difference) })}
                      </>
                    )}
                  </Badge>
                </VStack>
              </CardBody>
            </Card>

            {/* Financial Summary */}
            <SimpleGrid columns={[1, 2, 3]} spacing={4}>
              <Card bg="green.50" border="1px" borderColor="green.200">
                <CardBody textAlign="center">
                  <VStack spacing={1}>
                    <Text fontSize="sm" color="green.600">{t('reports.balanceSheet.totalAssets')}</Text>
                    <Text fontSize="2xl" fontWeight="bold" color="green.700">
                      {formatCurrency(balanceSheetData.assets?.total_assets || 0)}
                    </Text>
                  </VStack>
                </CardBody>
              </Card>

              <Card bg="orange.50" border="1px" borderColor="orange.200">
                <CardBody textAlign="center">
                  <VStack spacing={1}>
                    <Text fontSize="sm" color="orange.600">{t('reports.balanceSheet.totalLiabilities')}</Text>
                    <Text fontSize="2xl" fontWeight="bold" color="orange.700">
                      {formatCurrency(balanceSheetData.liabilities?.total_liabilities || 0)}
                    </Text>
                  </VStack>
                </CardBody>
              </Card>

              <Card bg="blue.50" border="1px" borderColor="blue.200">
                <CardBody textAlign="center">
                  <VStack spacing={1}>
                    <Text fontSize="sm" color="blue.600">{t('reports.balanceSheet.totalEquity')}</Text>
                    <Text fontSize="2xl" fontWeight="bold" color="blue.700">
                      {formatCurrency(balanceSheetData.equity?.total_equity || 0)}
                    </Text>
                  </VStack>
                </CardBody>
              </Card>
            </SimpleGrid>

            {/* Detailed Breakdown */}
            {includeAccountDetails && (
              <VStack spacing={4} align="stretch">
                <Divider />
                <Text fontSize="lg" fontWeight="bold" color={headingColor}>
                  {t('reports.balanceSheet.accountDetails')}
                </Text>

                {/* Assets Table */}
                {balanceSheetData.assets?.current_assets?.items && (
                  <Card bg={cardBg} border="1px" borderColor={borderColor}>
                    <CardHeader pb={2}>
                      <Text fontSize="md" fontWeight="bold" color="green.600">
                        {t('reports.assets').toUpperCase()}
                      </Text>
                    </CardHeader>
                    <CardBody pt={0}>
                      <Table size="sm" variant="simple">
                        <Thead>
                          <Tr>
                            <Th>{t('reports.accountCode')}</Th>
                            <Th>{t('reports.accountName')}</Th>
                            <Th isNumeric>{t('reports.amount')}</Th>
                          </Tr>
                        </Thead>
                        <Tbody>
                          {/* Current Assets */}
                          <Tr>
                            <Td colSpan={3}>
                              <Text fontSize="sm" fontWeight="semibold" color={descriptionColor}>
                                {t('reports.currentAssets')}
                              </Text>
                            </Td>
                          </Tr>
                          {balanceSheetData.assets.current_assets.items.map((item, index) => (
                            <Tr key={index}>
                              <Td pl={6}>{item.account_code}</Td>
                              <Td pl={6}>{item.account_name}</Td>
                              <Td isNumeric>{formatCurrency(item.amount, item.account_code, 'ASSET')}</Td>
                            </Tr>
                          ))}

                          {/* Non-Current Assets */}
                          {balanceSheetData.assets.non_current_assets?.items && (
                            <>
                              <Tr>
                                <Td colSpan={3}>
                                  <Text fontSize="sm" fontWeight="semibold" color={descriptionColor}>
                                    {t('reports.balanceSheet.nonCurrentAssets')}
                                  </Text>
                                </Td>
                              </Tr>
                              {balanceSheetData.assets.non_current_assets.items.map((item, index) => (
                                <Tr key={index}>
                                  <Td pl={6}>{item.account_code}</Td>
                                  <Td pl={6}>{item.account_name}</Td>
                                  <Td isNumeric>{formatCurrency(item.amount, item.account_code, 'ASSET')}</Td>
                                </Tr>
                              ))}
                            </>
                          )}

                          {/* Total Assets */}
                          <Tr bg="green.50">
                            <Td fontWeight="bold">{t('reports.totalAssets').toUpperCase()}</Td>
                            <Td></Td>
                            <Td isNumeric fontWeight="bold">
                              {formatCurrency(balanceSheetData.assets?.total_assets || 0)}
                            </Td>
                          </Tr>
                        </Tbody>
                      </Table>
                    </CardBody>
                  </Card>
                )}

                {/* Liabilities & Equity Table */}
                <Card bg={cardBg} border="1px" borderColor={borderColor}>
                  <CardHeader pb={2}>
                    <Text fontSize="md" fontWeight="bold" color="orange.600">
                      {t('reports.liabilitiesAndEquity').toUpperCase()}
                    </Text>
                  </CardHeader>
                  <CardBody pt={0}>
                    <Table size="sm" variant="simple">
                      <Thead>
                        <Tr>
                          <Th>Account Code</Th>
                          <Th>Account Name</Th>
                          <Th isNumeric>Amount</Th>
                        </Tr>
                      </Thead>
                      <Tbody>
                        {/* Liabilities */}
                        {balanceSheetData.liabilities?.current_liabilities?.items && (
                          <>
                            <Tr>
                              <Td colSpan={3}>
                                <Text fontSize="sm" fontWeight="semibold" color={descriptionColor}>
                                  {t('reports.balanceSheet.currentLiabilities')}
                                </Text>
                              </Td>
                            </Tr>
                            {balanceSheetData.liabilities.current_liabilities.items.map((item, index) => (
                              <Tr key={index}>
                                <Td pl={6}>{item.account_code}</Td>
                                <Td pl={6}>{item.account_name}</Td>
                                <Td isNumeric>{formatCurrency(item.amount, item.account_code, 'LIABILITY')}</Td>
                              </Tr>
                            ))}
                          </>
                        )}

                        {/* Equity */}
                        {balanceSheetData.equity?.items && (
                          <>
                            <Tr>
                              <Td colSpan={3}>
                                <Text fontSize="sm" fontWeight="semibold" color={descriptionColor}>
                                  {t('reports.equity')}
                                </Text>
                              </Td>
                            </Tr>
                            {balanceSheetData.equity.items.map((item, index) => (
                              <Tr key={index}>
                                <Td pl={6}>{item.account_code}</Td>
                                <Td pl={6}>{item.account_name}</Td>
                                <Td isNumeric>{formatCurrency(item.amount, item.account_code, 'EQUITY')}</Td>
                              </Tr>
                            ))}
                          </>
                        )}

                        {/* Totals */}
                        <Tr bg="orange.50">
                          <Td fontWeight="bold">{t('reports.totalLiabilities').toUpperCase()}</Td>
                          <Td></Td>
                          <Td isNumeric fontWeight="bold">
                            {formatCurrency(balanceSheetData.liabilities?.total_liabilities || 0)}
                          </Td>
                        </Tr>
                        <Tr bg="blue.50">
                          <Td fontWeight="bold">{t('reports.totalEquity').toUpperCase()}</Td>
                          <Td></Td>
                          <Td isNumeric fontWeight="bold">
                            {formatCurrency(balanceSheetData.equity?.total_equity || 0)}
                          </Td>
                        </Tr>
                        <Tr bg="gray.100">
                          <Td fontWeight="bold">{t('reports.totalLiabilitiesAndEquity').toUpperCase()}</Td>
                          <Td></Td>
                          <Td isNumeric fontWeight="bold">
                            {formatCurrency(balanceSheetData.total_liabilities_and_equity || 0)}
                          </Td>
                        </Tr>
                      </Tbody>
                    </Table>
                  </CardBody>
                </Card>
              </VStack>
            )}

            {/* Export Information */}
            {/* Export Information */}
            <Alert status="info" borderRadius="md">
              <AlertIcon />
              <Box>
                <AlertTitle>{t('reports.exportReady')}</AlertTitle>
                <AlertDescription>
                  {t('reports.exportReadyDesc')}
                </AlertDescription>
              </Box>
            </Alert>
          </VStack>
        )}
      </VStack>

      {/* Export Options Modal */}
      <Modal isOpen={isExportModalOpen} onClose={onExportModalClose} size="md">
        <ModalOverlay />
        <ModalContent>
          <ModalHeader>{t('reports.exportOptions')}</ModalHeader>
          <ModalCloseButton />
          <ModalBody>
            <VStack spacing={4} align="stretch">
              <FormControl>
                <FormLabel>{t('reports.companyName')}</FormLabel>
                <Input
                  value={companyName}
                  onChange={(e) => setCompanyName(e.target.value)}
                  placeholder={t('reports.companyNamePlaceholder')}
                />
              </FormControl>

              <FormControl>
                <HStack justify="space-between">
                  <FormLabel mb={0}>{t('reports.includeAccountDetails')}</FormLabel>
                  <Switch
                    isChecked={includeAccountDetails}
                    onChange={(e) => setIncludeAccountDetails(e.target.checked)}
                    colorScheme="blue"
                  />
                </HStack>
                <Text fontSize="sm" color={descriptionColor}>
                  {t('reports.includeAccountDetailsDesc')}
                </Text>
              </FormControl>

              <VStack spacing={2} align="stretch">
                <Text fontSize="sm" fontWeight="semibold">{t('reports.exportFormats')}</Text>
                <HStack spacing={3}>
                  <Button
                    flex={1}
                    colorScheme="green"
                    leftIcon={<FiFileText />}
                    onClick={handleExportCSV}
                    isLoading={exportingCSV}
                    loadingText="Exporting..."
                  >
                    Export CSV
                  </Button>
                  <Button
                    flex={1}
                    colorScheme="red"
                    leftIcon={<FiFilePlus />}
                    onClick={handleExportPDF}
                    isLoading={exportingPDF}
                    loadingText="Exporting..."
                  >
                    Export PDF
                  </Button>
                </HStack>
                <Text fontSize="xs" color={descriptionColor} textAlign="center">
                  Files will be downloaded with filename: balance_sheet_{asOfDate}.{'{format}'}
                </Text>
              </VStack>
            </VStack>
          </ModalBody>
          <ModalFooter>
            <Button variant="ghost" onClick={onExportModalClose}>
              Close
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </Box>
  );
};

export default EnhancedBalanceSheetReport;