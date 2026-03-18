'use client';

import React, { useState, useCallback, useRef, useEffect } from 'react';
import {
  Box,
  VStack,
  HStack,
  Text,
  Button,
  Card,
  CardBody,
  useDisclosure,
  useToast,
  FormControl,
  FormLabel,
  Input,
  Grid,
  GridItem,
  Stat,
  StatLabel,
  StatNumber,
  StatHelpText,
  StatArrow,
  Badge,
  Divider,
  Tooltip,
  IconButton,
  Switch,
  Flex,
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
  Progress,
  Spinner,
} from '@chakra-ui/react';
import { FiTrendingUp, FiDownload, FiEye, FiDatabase, FiActivity, FiRefreshCw, FiAlertTriangle, FiCheckCircle } from 'react-icons/fi';
import { formatCurrency } from '@/utils/formatters';
import { reportService } from '@/services/reportService';
import { enhancedPLService } from '@/services/enhancedPLService';
import { ssotProfitLossService } from '@/services/ssotProfitLossService';
import { ssotJournalService } from '@/services/ssotJournalService';
import { cogsService } from '@/services/cogsService';
import { useAuth } from '@/contexts/AuthContext';
import { useTranslation } from '@/hooks/useTranslation';
import EnhancedProfitLossModal from './EnhancedProfitLossModal';
import { JournalDrilldownModal } from './JournalDrilldownModal';

interface EnhancedPLData {
  title: string;
  period: string;
  company: any;
  enhanced: boolean;
  sections: any[];
  financialMetrics: {
    grossProfit: number;
    grossProfitMargin: number;
    operatingIncome: number;
    operatingMargin: number;
    ebitda: number;
    ebitdaMargin: number;
    netIncome: number;
    netIncomeMargin: number;
  };
  generatedAt?: string;
}

interface JournalDrilldownRequest {
  account_codes?: string[];
  account_ids?: number[];
  start_date: string;
  end_date: string;
  report_type?: string;
  line_item_name?: string;
  min_amount?: number;
  max_amount?: number;
  transaction_types?: string[];
  page: number;
  limit: number;
}

// Helper: Convert SSOT data to EnhancedPLData format
const transformSSOTToEnhancedPL = (ssotData: any, startDate: string, endDate: string): EnhancedPLData => {
  return {
    title: '', // Will be translated in modal using t('reports.profitLoss.enhancedTitle')
    period: ssotData.period || `${new Date(startDate).toLocaleDateString('id-ID', { day: 'numeric', month: 'long', year: 'numeric' })} - ${new Date(endDate).toLocaleDateString('id-ID', { day: 'numeric', month: 'long', year: 'numeric' })}`,
    company: ssotData.company || { name: 'Company Name Not Set' },
    enhanced: ssotData.enhanced ?? true,
    sections: ssotData.sections || [],
    financialMetrics: ssotData.financialMetrics || {
      grossProfit: 0,
      grossProfitMargin: 0,
      operatingIncome: 0,
      operatingMargin: 0,
      ebitda: 0,
      ebitdaMargin: 0,
      netIncome: 0,
      netIncomeMargin: 0,
    },
    generatedAt: new Date().toLocaleString('id-ID', {
      dateStyle: 'full',
      timeStyle: 'medium'
    })
  };
};

const EnhancedPLReportPage: React.FC = () => {
  const { token } = useAuth();
  const { t } = useTranslation();
  const toast = useToast();
  const [loading, setLoading] = useState(false);
  const [plData, setPLData] = useState<EnhancedPLData | null>(null);
  const [reportParams, setReportParams] = useState({
    start_date: '',
    end_date: '',
  });
  const [drilldownRequest, setDrilldownRequest] = useState<JournalDrilldownRequest | null>(null);
  const [realTimeUpdates, setRealTimeUpdates] = useState(false);
  const [isConnectedToBalanceService, setIsConnectedToBalanceService] = useState(false);
  const [lastUpdateTime, setLastUpdateTime] = useState<Date | null>(null);

  // COGS Health Check States
  const [cogsHealth, setCogsHealth] = useState<any>(null);
  const [checkingCOGS, setCheckingCOGS] = useState(false);
  const [backfillingCOGS, setBackfillingCOGS] = useState(false);
  const [showCOGSWarning, setShowCOGSWarning] = useState(false);

  const { isOpen: isPLModalOpen, onOpen: onPLModalOpen, onClose: onPLModalClose } = useDisclosure();
  const { isOpen: isDrilldownModalOpen, onOpen: onDrilldownModalOpen, onClose: onDrilldownModalClose } = useDisclosure();
  const { isOpen: isCOGSModalOpen, onOpen: onCOGSModalOpen, onClose: onCOGSModalClose } = useDisclosure();

  // Set default date range (current month)
  React.useEffect(() => {
    const today = new Date();
    const firstDayOfMonth = new Date(today.getFullYear(), today.getMonth(), 1);
    setReportParams({
      start_date: firstDayOfMonth.toISOString().split('T')[0],
      end_date: today.toISOString().split('T')[0],
    });
  }, []);

  // Real-time WebSocket connection effect
  useEffect(() => {
    if (realTimeUpdates && token) {
      setIsConnectedToBalanceService(false);
    } else {
      setIsConnectedToBalanceService(false);
    }
  }, [realTimeUpdates, token]);

  // Check COGS Health before generating P&L
  const checkCOGSHealth = async () => {
    if (!reportParams.start_date || !reportParams.end_date) {
      return null;
    }

    try {
      setCheckingCOGS(true);
      const health = await cogsService.getCOGSHealthStatus(
        reportParams.start_date,
        reportParams.end_date
      );
      setCogsHealth(health);
      return health;
    } catch (error) {
      console.error('Error checking COGS health:', error);
      return null;
    } finally {
      setCheckingCOGS(false);
    }
  };

  // Auto-backfill COGS if needed
  const handleBackfillCOGS = async () => {
    if (!reportParams.start_date || !reportParams.end_date) {
      return;
    }

    try {
      setBackfillingCOGS(true);

      const result = await cogsService.backfillCOGS(
        reportParams.start_date,
        reportParams.end_date,
        false // execute, not dry run
      );

      toast({
        title: t('reports.profitLoss.page.cogsWarning.backfillComplete'),
        description: t('reports.profitLoss.page.cogsWarning.backfillCompleteDesc', { count: result.sales_processed ?? 0 }),
        status: 'success',
        duration: 5000,
        isClosable: true,
      });

      // Close modal and re-check health
      onCOGSModalClose();
      await checkCOGSHealth();

    } catch (error) {
      console.error('Error backfilling COGS:', error);
      toast({
        title: t('reports.profitLoss.page.cogsWarning.backfillFailed'),
        description: error instanceof Error ? error.message : t('reports.profitLoss.page.cogsWarning.backfillFailed'),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setBackfillingCOGS(false);
    }
  };

  const generateEnhancedPL = async () => {
    if (!reportParams.start_date || !reportParams.end_date) {
      toast({
        title: t('reports.profitLoss.page.missingParameters'),
        description: t('reports.profitLoss.page.missingParametersDesc'),
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    setLoading(true);
    try {
      console.log('Generating Enhanced P&L with params:', reportParams);

      // 🔍 STEP 1: Check COGS Health First
      const health = await checkCOGSHealth();

      // 🚨 STEP 2: Show warning if COGS is missing
      if (health && !health.healthy && health.sales_without_cogs > 0) {
        setShowCOGSWarning(true);
        onCOGSModalOpen();
        setLoading(false);
        return; // Stop here, let user decide
      }

      // ✅ STEP 3: Generate SSOT P&L (includes COGS automatically)
      const ssotData = await ssotProfitLossService.generateSSOTProfitLoss({
        start_date: reportParams.start_date,
        end_date: reportParams.end_date,
        format: 'json'
      }) as unknown as any; // Cast to any to avoid type issues with Blob union, assuming json returned

      console.log('SSOT P&L data received:', ssotData);

      const formattedData = transformSSOTToEnhancedPL(ssotData, reportParams.start_date, reportParams.end_date);

      setPLData(formattedData);
      setShowCOGSWarning(false); // Clear warning
      onPLModalOpen();

    } catch (error) {
      console.error('Error generating enhanced P&L:', error);
      toast({
        title: t('reports.profitLoss.page.generationFailed'),
        description: error instanceof Error ? error.message : t('reports.profitLoss.page.generationFailedDesc'),
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  // Handle journal drilldown from P&L modal
  const handleJournalDrilldown = useCallback((itemName: string, accountCode?: string, amount?: number) => {
    console.log('Journal drilldown requested:', { itemName, accountCode, amount });

    if (!reportParams.start_date || !reportParams.end_date) {
      toast({
        title: 'Invalid Date Range',
        description: 'Please ensure the P&L report has valid start and end dates',
        status: 'warning',
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    const drilldownReq: JournalDrilldownRequest = {
      start_date: reportParams.start_date,
      end_date: reportParams.end_date,
      report_type: 'PROFIT_LOSS',
      line_item_name: itemName,
      page: 1,
      limit: 50,
    };

    // Add account filter if available
    if (accountCode) {
      drilldownReq.account_codes = [accountCode];
    }

    // Add amount filter if available
    if (amount !== undefined && amount > 0) {
      drilldownReq.min_amount = Math.max(0, amount * 0.01); // Allow 1% variance
      drilldownReq.max_amount = amount * 1.01;
    }

    setDrilldownRequest(drilldownReq);
    onDrilldownModalOpen();
  }, [reportParams, toast, onDrilldownModalOpen]);

  // Handle P&L export
  const handlePLExport = useCallback(async (format: 'pdf' | 'excel') => {
    if (!reportParams.start_date || !reportParams.end_date) return;

    try {
      const result = await reportService.generateProfessionalReport('profit-loss', {
        ...reportParams,
        format: format === 'excel' ? 'csv' : 'pdf'
      });

      if (result instanceof Blob) {
        const fileName = `profit-loss-${new Date().toISOString().split('T')[0]}.${format === 'excel' ? 'csv' : 'pdf'}`;
        await reportService.downloadReport(result, fileName);

        toast({
          title: 'Export Successful',
          description: `P&L report exported as ${format.toUpperCase()}`,
          status: 'success',
          duration: 3000,
          isClosable: true,
        });
      }
    } catch (error) {
      toast({
        title: 'Export Failed',
        description: error instanceof Error ? error.message : 'Failed to export report',
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    }
  }, [reportParams, toast]);

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setReportParams(prev => ({ ...prev, [name]: value }));
  };

  return (
    <Box p={8}>
      <VStack spacing={8} align="stretch">
        {/* Header */}
        <Box>
          <HStack spacing={4} align="center">
            <Box p={3} bg="blue.100" borderRadius="md">
              <FiTrendingUp size="24px" color="blue.600" />
            </Box>
            <VStack align="start" spacing={1}>
              <Text fontSize="2xl" fontWeight="bold" color="gray.700">
                {t('reports.profitLoss.page.title')}
              </Text>
              <Text fontSize="md" color="gray.600">
                {t('reports.profitLoss.page.description')}
              </Text>
            </VStack>
          </HStack>
        </Box>

        {/* Parameters Card */}
        <Card>
          <CardBody>
            <VStack spacing={4} align="stretch">
              <Flex justify="space-between" align="center">
                <Text fontSize="lg" fontWeight="semibold">{t('reports.profitLoss.page.reportParameters')}</Text>

                {/* Real-time Updates Control */}
                <HStack spacing={3}>
                  <Text fontSize="sm" color="gray.600">{t('reports.profitLoss.page.realTimeUpdates')}</Text>
                  <Switch
                    isChecked={realTimeUpdates}
                    onChange={(e) => setRealTimeUpdates(e.target.checked)}
                    colorScheme="green"
                  />
                  {isConnectedToBalanceService && (
                    <Tooltip label={t('reports.profitLoss.page.connected')} fontSize="xs">
                      <Badge
                        colorScheme="green"
                        variant="subtle"
                        fontSize="xs"
                        display="flex"
                        alignItems="center"
                        gap={1}
                      >
                        <FiActivity size={10} />
                        {t('reports.profitLoss.page.connected')}
                      </Badge>
                    </Tooltip>
                  )}
                  {lastUpdateTime && (
                    <Text fontSize="xs" color="gray.500">
                      {t('reports.profitLoss.page.lastUpdate')}: {lastUpdateTime.toLocaleTimeString()}
                    </Text>
                  )}
                </HStack>
              </Flex>

              <Grid templateColumns="repeat(2, 1fr)" gap={4}>
                <GridItem>
                  <FormControl isRequired>
                    <FormLabel>{t('reports.profitLoss.page.startDate')}</FormLabel>
                    <Input
                      type="date"
                      name="start_date"
                      value={reportParams.start_date}
                      onChange={handleInputChange}
                    />
                  </FormControl>
                </GridItem>
                <GridItem>
                  <FormControl isRequired>
                    <FormLabel>{t('reports.profitLoss.page.endDate')}</FormLabel>
                    <Input
                      type="date"
                      name="end_date"
                      value={reportParams.end_date}
                      onChange={handleInputChange}
                    />
                  </FormControl>
                </GridItem>
              </Grid>
            </VStack>
          </CardBody>
        </Card>

        {/* Action Buttons */}
        <HStack spacing={4}>
          <Button
            colorScheme="blue"
            size="lg"
            leftIcon={<FiEye />}
            onClick={generateEnhancedPL}
            isLoading={loading}
            loadingText={t('reports.profitLoss.page.generating')}
          >
            {t('reports.profitLoss.page.generateButton')}
          </Button>

          {/* Manual Refresh Button - visible when we have existing data */}
          {plData && (
            <Tooltip label={t('reports.profitLoss.page.refreshTooltip')} fontSize="xs">
              <IconButton
                aria-label={t('reports.profitLoss.page.refreshTooltip')}
                icon={<FiRefreshCw />}
                size="lg"
                variant="outline"
                colorScheme="gray"
                onClick={generateEnhancedPL}
                isLoading={loading}
                isDisabled={loading}
              />
            </Tooltip>
          )}

          <Button
            variant="outline"
            size="lg"
            leftIcon={<FiDownload />}
            onClick={() => handlePLExport('pdf')}
            isDisabled={loading}
          >
            {t('reports.profitLoss.page.exportPDF')}
          </Button>
          <Button
            variant="outline"
            size="lg"
            leftIcon={<FiDatabase />}
            onClick={() => handlePLExport('excel')}
            isDisabled={loading}
          >
            {t('reports.profitLoss.page.exportCSV')}
          </Button>
        </HStack>

        {/* Quick Stats Preview */}
        {plData?.financialMetrics && (
          <Grid templateColumns="repeat(4, 1fr)" gap={6}>
            <GridItem>
              <Card>
                <CardBody>
                  <Stat>
                    <StatLabel>{t('reports.profitLoss.grossProfit')}</StatLabel>
                    <StatNumber color={plData.financialMetrics.grossProfit >= 0 ? 'green.600' : 'red.600'}>
                      {formatCurrency(plData.financialMetrics.grossProfit)}
                    </StatNumber>
                    <StatHelpText>
                      <StatArrow type={plData.financialMetrics.grossProfitMargin >= 0 ? 'increase' : 'decrease'} />
                      {(plData.financialMetrics.grossProfitMargin ?? 0).toFixed(1)}%
                    </StatHelpText>
                  </Stat>
                </CardBody>
              </Card>
            </GridItem>
            <GridItem>
              <Card>
                <CardBody>
                  <Stat>
                    <StatLabel>{t('reports.profitLoss.operatingIncome')}</StatLabel>
                    <StatNumber color={plData.financialMetrics.operatingIncome >= 0 ? 'green.600' : 'red.600'}>
                      {formatCurrency(plData.financialMetrics.operatingIncome)}
                    </StatNumber>
                    <StatHelpText>
                      <StatArrow type={plData.financialMetrics.operatingMargin >= 0 ? 'increase' : 'decrease'} />
                      {(plData.financialMetrics.operatingMargin ?? 0).toFixed(1)}%
                    </StatHelpText>
                  </Stat>
                </CardBody>
              </Card>
            </GridItem>
            <GridItem>
              <Card>
                <CardBody>
                  <Stat>
                    <StatLabel>{t('reports.profitLoss.ebitda')}</StatLabel>
                    <StatNumber color={plData.financialMetrics.ebitda >= 0 ? 'green.600' : 'red.600'}>
                      {formatCurrency(plData.financialMetrics.ebitda)}
                    </StatNumber>
                    <StatHelpText>
                      <StatArrow type={plData.financialMetrics.ebitdaMargin >= 0 ? 'increase' : 'decrease'} />
                      {(plData.financialMetrics.ebitdaMargin ?? 0).toFixed(1)}%
                    </StatHelpText>
                  </Stat>
                </CardBody>
              </Card>
            </GridItem>
            <GridItem>
              <Card>
                <CardBody>
                  <Stat>
                    <StatLabel>{t('reports.profitLoss.netIncome')}</StatLabel>
                    <StatNumber color={plData.financialMetrics.netIncome >= 0 ? 'green.600' : 'red.600'}>
                      {formatCurrency(plData.financialMetrics.netIncome)}
                    </StatNumber>
                    <StatHelpText>
                      <StatArrow type={plData.financialMetrics.netIncomeMargin >= 0 ? 'increase' : 'decrease'} />
                      {(plData.financialMetrics.netIncomeMargin ?? 0).toFixed(1)}%
                    </StatHelpText>
                  </Stat>
                </CardBody>
              </Card>
            </GridItem>
          </Grid>
        )}

        {/* Features */}
        <Card>
          <CardBody>
            <VStack spacing={4} align="stretch">
              <Text fontSize="lg" fontWeight="semibold">{t('reports.profitLoss.page.features')}</Text>
              <VStack spacing={3} align="start">
                <HStack>
                  <Badge colorScheme="green">{t('reports.profitLoss.enhanced')}</Badge>
                  <Text fontSize="sm">{t('reports.profitLoss.page.featureEnhanced')}</Text>
                </HStack>
                <HStack>
                  <Badge colorScheme="blue">{t('common.interactive')}</Badge>
                  <Text fontSize="sm">{t('reports.profitLoss.page.featureInteractive')}</Text>
                </HStack>
                <HStack>
                  <Badge colorScheme="purple">{t('common.comprehensive')}</Badge>
                  <Text fontSize="sm">{t('reports.profitLoss.page.featureComprehensive')}</Text>
                </HStack>
                <HStack>
                  <Badge colorScheme="orange">{t('common.exportReady')}</Badge>
                  <Text fontSize="sm">{t('reports.profitLoss.page.featureExportReady')}</Text>
                </HStack>
                <HStack>
                  <Badge colorScheme="teal">{t('common.realTime')}</Badge>
                  <Text fontSize="sm">{t('reports.profitLoss.page.featureRealTime')}</Text>
                </HStack>
                <HStack>
                  <Badge colorScheme="yellow">{t('common.liveData')}</Badge>
                  <Text fontSize="sm">{t('reports.profitLoss.page.featureLiveData')}</Text>
                </HStack>
              </VStack>
            </VStack>
          </CardBody>
        </Card>
      </VStack>

      {/* Enhanced P&L Modal */}
      {plData && (
        <EnhancedProfitLossModal
          isOpen={isPLModalOpen}
          onClose={onPLModalClose}
          data={plData}
          onJournalDrilldown={handleJournalDrilldown}
          onExport={handlePLExport}
        />
      )}

      {/* Journal Drilldown Modal */}
      {drilldownRequest && (
        <JournalDrilldownModal
          isOpen={isDrilldownModalOpen}
          onClose={onDrilldownModalClose}
          drilldownRequest={drilldownRequest}
          title={`Journal Entries: ${drilldownRequest.line_item_name || 'Selected Item'}`}
        />
      )}

      {/* COGS Warning Modal */}
      <Modal isOpen={isCOGSModalOpen} onClose={onCOGSModalClose} size="xl">
        <ModalOverlay />
        <ModalContent>
          <ModalHeader>
            <HStack spacing={2}>
              <FiAlertTriangle color="orange" />
              <Text>{t('reports.profitLoss.page.cogsWarning.title')}</Text>
            </HStack>
          </ModalHeader>
          <ModalCloseButton />
          <ModalBody>
            <VStack spacing={4} align="stretch">
              {cogsHealth && (
                <>
                  <Alert status="warning">
                    <AlertIcon />
                    <VStack align="start" spacing={1}>
                      <AlertTitle>{t('reports.profitLoss.page.cogsWarning.description')}</AlertTitle>
                      <AlertDescription>
                        {cogsHealth.message}
                      </AlertDescription>
                    </VStack>
                  </Alert>

                  <Card>
                    <CardBody>
                      <VStack spacing={3} align="stretch">
                        <Text fontWeight="semibold">{t('reports.profitLoss.page.cogsWarning.status')}</Text>
                        <HStack justify="space-between">
                          <Text fontSize="sm">{t('reports.profitLoss.page.cogsWarning.totalSales')}</Text>
                          <Badge colorScheme="blue">{cogsHealth.total_sales}</Badge>
                        </HStack>
                        <HStack justify="space-between">
                          <Text fontSize="sm">{t('reports.profitLoss.page.cogsWarning.salesWithCOGS')}</Text>
                          <Badge colorScheme="green">{cogsHealth.sales_with_cogs}</Badge>
                        </HStack>
                        <HStack justify="space-between">
                          <Text fontSize="sm">{t('reports.profitLoss.page.cogsWarning.salesWithoutCOGS')}</Text>
                          <Badge colorScheme="red">{cogsHealth.sales_without_cogs}</Badge>
                        </HStack>
                        <HStack justify="space-between">
                          <Text fontSize="sm">{t('reports.profitLoss.page.cogsWarning.completeness')}</Text>
                          <Badge colorScheme={cogsHealth.completeness_percentage >= 95 ? 'green' : 'red'}>
                            {cogsHealth.completeness_percentage.toFixed(1)}%
                          </Badge>
                        </HStack>
                      </VStack>
                    </CardBody>
                  </Card>

                  <Alert status="info">
                    <AlertIcon />
                    <VStack align="start" spacing={1} fontSize="sm">
                      <Text fontWeight="semibold">{t('reports.profitLoss.page.cogsWarning.whatIsCOGS')}</Text>
                      <Text>
                        {t('reports.profitLoss.page.cogsWarning.cogsExplanation')}
                      </Text>
                    </VStack>
                  </Alert>

                  <Divider />

                  <VStack spacing={2} align="stretch">
                    <Text fontWeight="semibold" fontSize="sm">{t('reports.profitLoss.page.cogsWarning.options')}</Text>
                    <Button
                      colorScheme="green"
                      leftIcon={backfillingCOGS ? <Spinner size="sm" /> : <FiCheckCircle />}
                      onClick={async () => {
                        await handleBackfillCOGS();
                        // After backfill, generate P&L again
                        await generateEnhancedPL();
                      }}
                      isLoading={backfillingCOGS}
                      loadingText={t('reports.profitLoss.page.cogsWarning.creatingCOGS')}
                    >
                      {t('reports.profitLoss.page.cogsWarning.autoCreate')}
                    </Button>
                    <Text fontSize="xs" color="gray.600" px={2}>
                      {t('reports.profitLoss.page.cogsWarning.autoCreateDesc')}
                    </Text>

                    <Divider />

                    <Button
                      variant="outline"
                      onClick={async () => {
                        onCOGSModalClose();
                        setShowCOGSWarning(false);
                        // Continue without COGS check (using SSOT endpoint)
                        setLoading(true);
                        try {
                          const ssotData = await ssotProfitLossService.generateSSOTProfitLoss({
                            start_date: reportParams.start_date,
                            end_date: reportParams.end_date,
                            format: 'json'
                          }) as unknown as any;
                          const formattedData = transformSSOTToEnhancedPL(ssotData, reportParams.start_date, reportParams.end_date);
                          setPLData(formattedData);
                          onPLModalOpen();
                        } catch (error) {
                          console.error('Error:', error);
                          toast({
                            title: t('reports.profitLoss.page.generationFailed'),
                            description: t('reports.profitLoss.page.generationFailedDesc'),
                            status: 'error',
                            duration: 5000,
                            isClosable: true,
                          });
                        } finally {
                          setLoading(false);
                        }
                      }}
                    >
                      {t('reports.profitLoss.page.cogsWarning.continueAnyway')}
                    </Button>
                    <Text fontSize="xs" color="gray.600" px={2}>
                      {t('reports.profitLoss.page.cogsWarning.continueAnywayDesc')}
                    </Text>
                  </VStack>
                </>
              )}
            </VStack>
          </ModalBody>
          <ModalFooter>
            <Button variant="ghost" onClick={onCOGSModalClose}>
              {t('reports.profitLoss.page.cogsWarning.cancel')}
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </Box>
  );
};

export default EnhancedPLReportPage;
