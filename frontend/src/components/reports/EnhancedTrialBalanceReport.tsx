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
  Tfoot,
  Tr,
  Th,
  Td,
  Tooltip,
  useColorModeValue,
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
  ssotTrialBalanceService,
  SSOTTrialBalanceData
} from '../../services/ssotTrialBalanceService';
import { useTranslation } from '@/hooks/useTranslation';
import {
  exportAndDownloadTrialBalanceCSV,
  exportAndDownloadTrialBalancePDF
} from '../../utils/trialBalanceExportUtils';

interface EnhancedTrialBalanceReportProps {
  onClose?: () => void;
}

export const EnhancedTrialBalanceReport: React.FC<EnhancedTrialBalanceReportProps> = ({
  onClose
}) => {
  // State management
  const [trialBalanceData, setTrialBalanceData] = useState<SSOTTrialBalanceData | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [asOfDate, setAsOfDate] = useState(new Date().toISOString().split('T')[0]);
  const [includeAccountDetails, setIncludeAccountDetails] = useState(true);
  const [companyName, setCompanyName] = useState('Company Name Not Set');

  // Export states
  const [exportingCSV, setExportingCSV] = useState(false);
  const [exportingPDF, setExportingPDF] = useState(false);

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

  // Format currency for display with account type consideration
  const formatCurrency = (amount: number | null | undefined, accountCode?: string, accountType?: string) => {
    if (amount === null || amount === undefined || isNaN(Number(amount))) {
      return 'Rp 0';
    }

    let displayAmount = Number(amount);

    // Convert to positive for revenue and liability accounts for user-friendly display
    if (accountType === 'REVENUE' || accountType === 'LIABILITY' ||
      accountCode?.startsWith('4') || // Revenue accounts
      accountCode?.startsWith('2102') || accountCode?.startsWith('2203') ||
      accountCode === '1105-005') { // PPN/tax accounts
      displayAmount = Math.abs(displayAmount);
      console.log(`[EnhancedTrialBalanceReport] Converted ${accountCode} (${accountType}) balance ${amount} to positive ${displayAmount}`);
    }

    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0
    }).format(displayAmount);
  };

  // Generate Trial Balance Report
  const generateTrialBalanceReport = async () => {
    setLoading(true);
    setError(null);

    try {
      const data = await ssotTrialBalanceService.generateSSOTTrialBalance({
        as_of_date: asOfDate,
        format: 'json'
      });

      setTrialBalanceData(data);

      toast({
        title: t('reports.modal.status.success'),
        description: t('reports.messages.trialBalanceGenerated'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });

    } catch (error: any) {
      const errorMessage = error.message || t('reports.messages.failedToGenerateTrialBalance');
      setError(errorMessage);

      toast({
        title: t('reports.modal.status.error'),
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
    if (!trialBalanceData) {
      toast({
        title: 'No Data',
        description: 'Please generate the Trial Balance first',
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    setExportingCSV(true);

    try {
      exportAndDownloadTrialBalanceCSV(trialBalanceData, {
        includeAccountDetails,
        companyName,
        filename: `trial_balance_${asOfDate}.csv`
      });

      toast({
        title: t('reports.csvExportSuccess'),
        description: t('reports.messages.balanceSheetDownloadedCSV'), // Reusing similar message or add specific if needed
        status: 'success',
        duration: 3000,
        isClosable: true,
      });

    } catch (error: any) {
      toast({
        title: t('reports.csvExportFailedTitle'),
        description: error.message || t('reports.csvExportError'),
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
    if (!trialBalanceData) {
      toast({
        title: 'No Data',
        description: 'Please generate the Trial Balance first',
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    setExportingPDF(true);

    try {
      exportAndDownloadTrialBalancePDF(trialBalanceData, {
        companyName,
        includeAccountDetails,
        filename: `trial_balance_${asOfDate}.pdf`
      });

      toast({
        title: t('reports.pdfExportSuccess'),
        description: t('reports.messages.balanceSheetDownloadedPDF'), // Reusing similar message or add specific if needed
        status: 'success',
        duration: 3000,
        isClosable: true,
      });

    } catch (error: any) {
      toast({
        title: t('reports.pdfExportFailedTitle'),
        description: error.message || t('reports.pdfExportError'),
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
                <Icon as={FiBarChart} color="purple.500" boxSize={6} />
                <VStack align="start" spacing={0}>
                  <Text fontSize="xl" fontWeight="bold" color={headingColor}>
                    {t('reports.trialBalance.title')}
                  </Text>
                  <Text fontSize="sm" color={descriptionColor}>
                    {t('reports.trialBalance.ssotIntegrationDesc') || 'Real-time SSOT integration with advanced export features'}
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
                  {t('reports.common.configuration')}
                </Text>
                <SimpleGrid columns={[1, 2, 3]} spacing={4}>
                  <FormControl>
                    <FormLabel fontSize="sm">{t('reports.common.asOfDate')}</FormLabel>
                    <Input
                      type="date"
                      value={asOfDate}
                      onChange={(e) => setAsOfDate(e.target.value)}
                      size="sm"
                    />
                  </FormControl>
                  <FormControl>
                    <FormLabel fontSize="sm">{t('reports.common.companyName')}</FormLabel>
                    <Input
                      value={companyName}
                      onChange={(e) => setCompanyName(e.target.value)}
                      placeholder="Company Name"
                      size="sm"
                    />
                  </FormControl>
                  <FormControl>
                    <FormLabel fontSize="sm">{t('reports.common.includeAccountDetails')}</FormLabel>
                    <Switch
                      isChecked={includeAccountDetails}
                      onChange={(e) => setIncludeAccountDetails(e.target.checked)}
                      colorScheme="purple"
                    />
                  </FormControl>
                </SimpleGrid>
              </Box>

              {/* Action Buttons */}
              <HStack spacing={3} justify="space-between">
                <HStack spacing={3}>
                  <Button
                    colorScheme="purple"
                    leftIcon={<FiRefreshCw />}
                    onClick={generateTrialBalanceReport}
                    isLoading={loading}
                    loadingText={t('common.generating') || 'Generating...'}
                    size="md"
                  >
                    {t('reports.generateReport') || 'Generate Report'}
                  </Button>

                  {trialBalanceData && (
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

                {trialBalanceData && (
                  <HStack spacing={2}>
                    <Tooltip label="Export as CSV file">
                      <Button
                        colorScheme="green"
                        variant="outline"
                        leftIcon={<FiFileText />}
                        onClick={handleExportCSV}
                        isLoading={exportingCSV}
                        loadingText={t('reports.exporting') || "Exporting..."}
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
                        loadingText={t('reports.exporting') || "Exporting..."}
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
                <Spinner size="xl" thickness="4px" speed="0.65s" color="purple.500" />
                <VStack spacing={2}>
                  <Text fontSize="lg" fontWeight="medium" color={headingColor}>
                    {t('reports.generatingReport')}
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
              <AlertTitle>{t('reports.errorGeneratingReport')}</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </VStack>
            <Button
              size="sm"
              colorScheme="red"
              variant="outline"
              onClick={generateTrialBalanceReport}
              ml={4}
            >
              Retry
            </Button>
          </Alert>
        )}

        {/* Trial Balance Data Display */}
        {trialBalanceData && !loading && (
          <VStack spacing={6} align="stretch">
            {/* Report Header */}
            <Card bg={cardBg} border="1px" borderColor={borderColor}>
              <CardBody textAlign="center">
                <VStack spacing={2}>
                  <Text fontSize="xl" fontWeight="bold" color={headingColor}>
                    {trialBalanceData.company?.name || companyName}
                  </Text>
                  <Text fontSize="lg" fontWeight="semibold">
                    {t('reports.trialBalance.title')}
                  </Text>
                  <Text fontSize="md" color={descriptionColor}>
                    {t('reports.common.asOfDate')}: {new Date(trialBalanceData.as_of_date).toLocaleDateString('id-ID')}
                  </Text>
                  <Badge
                    colorScheme={trialBalanceData.is_balanced ? 'green' : 'red'}
                    px={3}
                    py={1}
                    borderRadius="full"
                  >
                    {trialBalanceData.is_balanced ? (
                      <>
                        <Icon as={FiCheckCircle} mr={1} />
                        {t('reports.trialBalance.balanced')} ✓
                      </>
                    ) : (
                      <>
                        <Icon as={FiAlertTriangle} mr={1} />
                        {t('reports.trialBalance.notBalanced')} (Diff: {formatCurrency(trialBalanceData.difference)})
                      </>
                    )}
                  </Badge>
                </VStack>
              </CardBody>
            </Card>

            {/* Financial Summary */}
            <SimpleGrid columns={[1, 2]} spacing={4}>
              <Card bg="green.50" border="1px" borderColor="green.200">
                <CardBody textAlign="center">
                  <VStack spacing={1}>
                    <Text fontSize="sm" color="green.600">{t('reports.trialBalance.totalDebits')}</Text>
                    <Text fontSize="2xl" fontWeight="bold" color="green.700">
                      {formatCurrency(trialBalanceData.total_debits || 0)}
                    </Text>
                  </VStack>
                </CardBody>
              </Card>

              <Card bg="orange.50" border="1px" borderColor="orange.200">
                <CardBody textAlign="center">
                  <VStack spacing={1}>
                    <Text fontSize="sm" color="orange.600">{t('reports.trialBalance.totalCredits')}</Text>
                    <Text fontSize="2xl" fontWeight="bold" color="orange.700">
                      {formatCurrency(trialBalanceData.total_credits || 0)}
                    </Text>
                  </VStack>
                </CardBody>
              </Card>
            </SimpleGrid>

            {/* Detailed Account Breakdown */}
            {includeAccountDetails && trialBalanceData.accounts && (
              <VStack spacing={4} align="stretch">
                <Divider />
                <Text fontSize="lg" fontWeight="bold" color={headingColor}>
                  {t('reports.trialBalance.accountDetails') || t('reports.accountDetails')}
                </Text>

                <Card bg={cardBg} border="1px" borderColor={borderColor}>
                  <CardHeader pb={2}>
                    <Text fontSize="md" fontWeight="bold" color="purple.600">
                      {t('reports.trialBalance.accounts').toUpperCase()}
                    </Text>
                  </CardHeader>
                  <CardBody pt={0}>
                    <Table size="sm" variant="simple">
                      <Thead>
                        <Tr>
                          <Th>{t('reports.trialBalance.accountCode')}</Th>
                          <Th>{t('reports.trialBalance.accountName')}</Th>
                          <Th>{t('reports.trialBalance.accountType')}</Th>
                          <Th isNumeric>{t('reports.trialBalance.debit')}</Th>
                          <Th isNumeric>{t('reports.trialBalance.credit')}</Th>
                        </Tr>
                      </Thead>
                      <Tbody>
                        {trialBalanceData.accounts.map((account, index) => (
                          <Tr key={index}>
                            <Td>{account.account_code}</Td>
                            <Td>{account.account_name}</Td>
                            <Td>
                              <Badge
                                size="sm"
                                colorScheme={
                                  account.account_type === 'Asset' ? 'green' :
                                    account.account_type === 'Liability' ? 'orange' :
                                      account.account_type === 'Equity' ? 'blue' :
                                        account.account_type === 'Revenue' ? 'purple' :
                                          account.account_type === 'Expense' ? 'red' : 'gray'
                                }
                              >
                                {account.account_type}
                              </Badge>
                            </Td>
                            <Td isNumeric>
                              {account.debit_balance > 0 ? formatCurrency(account.debit_balance, account.account_code, account.account_type) : '-'}
                            </Td>
                            <Td isNumeric>
                              {account.credit_balance > 0 ? formatCurrency(account.credit_balance, account.account_code, account.account_type) : '-'}
                            </Td>
                          </Tr>
                        ))}

                      </Tbody>
                      <Tfoot>
                        {/* Total Row */}
                        <Tr bg="gray.100" fontWeight="bold">
                          <Td colSpan={3}>{t('reports.modal.total').toUpperCase()}</Td>
                          <Td isNumeric fontWeight="bold">
                            {formatCurrency(trialBalanceData.total_debits || 0)}
                          </Td>
                          <Td isNumeric fontWeight="bold">
                            {formatCurrency(trialBalanceData.total_credits || 0)}
                          </Td>
                        </Tr>
                      </Tfoot>
                    </Table>
                  </CardBody>
                </Card>
              </VStack>
            )}

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
          <ModalHeader>Export Options</ModalHeader>
          <ModalCloseButton />
          <ModalBody>
            <VStack spacing={4} align="stretch">
              <FormControl>
                <FormLabel>{t('reports.common.companyName')}</FormLabel>
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
                    colorScheme="purple"
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
                    {t('reports.exportCSV')}
                  </Button>
                  <Button
                    flex={1}
                    colorScheme="red"
                    leftIcon={<FiFilePlus />}
                    onClick={handleExportPDF}
                    isLoading={exportingPDF}
                    loadingText="Exporting..."
                  >
                    {t('reports.exportPDF')}
                  </Button>
                </HStack>
                <Text fontSize="xs" color={descriptionColor} textAlign="center">
                  Files will be downloaded with filename: trial_balance_{asOfDate}.{'{format}'}
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

export default EnhancedTrialBalanceReport;
