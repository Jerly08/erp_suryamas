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
  Divider,
  Accordion,
  AccordionItem,
  AccordionButton,
  AccordionPanel,
  AccordionIcon,
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
  Tooltip,
  useToast
} from '@chakra-ui/react';
import { FiDownload, FiTrendingUp, FiDollarSign, FiPieChart } from 'react-icons/fi';
import { formatCurrency } from '../../utils/formatters';
import JournalDrilldownButton from './JournalDrilldownButton';
import { useTranslation } from '@/hooks/useTranslation';

interface EnhancedProfitLossModalProps {
  isOpen: boolean;
  onClose: () => void;
  data: any;
  onJournalDrilldown?: (itemName: string, accountCode?: string, amount?: number) => void;
  onExport?: (format: 'pdf' | 'excel') => void;
}

const EnhancedProfitLossModal: React.FC<EnhancedProfitLossModalProps> = ({
  isOpen,
  onClose,
  data,
  onJournalDrilldown,
  onExport
}) => {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState<'statement' | 'metrics' | 'analysis'>('statement');
  const toast = useToast();

  // 🔍 DEBUG: Confirm this file is loaded with tooltips
  React.useEffect(() => {
    console.log('🎉 [PL Modal v2.0 WITH TOOLTIPS] Component loaded at:', new Date().toLocaleTimeString());
    console.log('📊 Data sections:', data?.sections?.length || 0);
  }, [data]);

  // Color mode values
  const modalBg = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const sectionBg = useColorModeValue('gray.50', 'gray.700');
  const subsectionBg = useColorModeValue('blue.50', 'blue.900');
  const metricsBg = useColorModeValue('green.50', 'green.900');
  const textColor = useColorModeValue('gray.800', 'white');
  const secondaryTextColor = useColorModeValue('gray.600', 'gray.300');
  const positiveColor = useColorModeValue('green.600', 'green.300');
  const negativeColor = useColorModeValue('red.600', 'red.300');

  if (!data) return null;

  const handleExport = (format: 'pdf' | 'excel') => {
    if (onExport) {
      onExport(format);
    } else {
      toast({
        title: t('reports.profitLoss.exportFeature'),
        description: t('reports.profitLoss.exportComingSoon', { format: format.toUpperCase() }),
        status: 'info',
        duration: 3000,
        isClosable: true,
      });
    }
  };

  const renderFinancialMetrics = () => {
    if (!data.financialMetrics) return null;

    const metrics = data.financialMetrics;

    return (
      <Grid templateColumns="repeat(2, 1fr)" gap={4} mb={6}>
        <GridItem>
          <Card size="sm">
            <CardBody>
              <Stat>
                <HStack spacing={1}>
                  <StatLabel>{t('reports.profitLoss.grossProfit')}</StatLabel>
                  <Tooltip
                    label={t('reports.profitLoss.tooltips.grossProfit')}
                    fontSize="sm"
                    maxW="300px"
                    hasArrow
                    placement="top"
                  >
                    <Box as="span" cursor="help" color="blue.500" fontSize="xs">ℹ️</Box>
                  </Tooltip>
                </HStack>
                <StatNumber color={metrics.grossProfit >= 0 ? positiveColor : negativeColor}>
                  {formatCurrency(metrics.grossProfit)}
                </StatNumber>
                <StatHelpText>
                  <StatArrow type={metrics.grossProfitMargin >= 0 ? 'increase' : 'decrease'} />
                  {metrics.grossProfitMargin.toFixed(1)}%
                </StatHelpText>
              </Stat>
            </CardBody>
          </Card>
        </GridItem>

        <GridItem>
          <Card size="sm">
            <CardBody>
              <Stat>
                <HStack spacing={1}>
                  <StatLabel>{t('reports.profitLoss.operatingIncome')}</StatLabel>
                  <Tooltip
                    label={t('reports.profitLoss.tooltips.operatingIncome')}
                    fontSize="sm"
                    maxW="300px"
                    hasArrow
                    placement="top"
                  >
                    <Box as="span" cursor="help" color="blue.500" fontSize="xs">ℹ️</Box>
                  </Tooltip>
                </HStack>
                <StatNumber color={metrics.operatingIncome >= 0 ? positiveColor : negativeColor}>
                  {formatCurrency(metrics.operatingIncome)}
                </StatNumber>
                <StatHelpText>
                  <StatArrow type={metrics.operatingMargin >= 0 ? 'increase' : 'decrease'} />
                  {metrics.operatingMargin.toFixed(1)}%
                </StatHelpText>
              </Stat>
            </CardBody>
          </Card>
        </GridItem>

        <GridItem>
          <Card size="sm">
            <CardBody>
              <Stat>
                <HStack spacing={1}>
                  <StatLabel>{t('reports.profitLoss.ebitda')}</StatLabel>
                  <Tooltip
                    label={t('reports.profitLoss.tooltips.ebitda')}
                    fontSize="sm"
                    maxW="300px"
                    hasArrow
                    placement="top"
                  >
                    <Box as="span" cursor="help" color="blue.500" fontSize="xs">ℹ️</Box>
                  </Tooltip>
                </HStack>
                <StatNumber color={metrics.ebitda >= 0 ? positiveColor : negativeColor}>
                  {formatCurrency(metrics.ebitda)}
                </StatNumber>
                <StatHelpText>
                  <StatArrow type={metrics.ebitdaMargin >= 0 ? 'increase' : 'decrease'} />
                  {metrics.ebitdaMargin.toFixed(1)}%
                </StatHelpText>
              </Stat>
            </CardBody>
          </Card>
        </GridItem>

        <GridItem>
          <Card size="sm">
            <CardBody>
              <Stat>
                <HStack spacing={1}>
                  <StatLabel>{t('reports.profitLoss.netIncome')}</StatLabel>
                  <Tooltip
                    label={t('reports.profitLoss.tooltips.netIncome')}
                    fontSize="sm"
                    maxW="300px"
                    hasArrow
                    placement="top"
                  >
                    <Box as="span" cursor="help" color="blue.500" fontSize="xs">ℹ️</Box>
                  </Tooltip>
                </HStack>
                <StatNumber color={metrics.netIncome >= 0 ? positiveColor : negativeColor}>
                  {formatCurrency(metrics.netIncome)}
                </StatNumber>
                <StatHelpText>
                  <StatArrow type={metrics.netIncomeMargin >= 0 ? 'increase' : 'decrease'} />
                  {metrics.netIncomeMargin.toFixed(1)}%
                </StatHelpText>
              </Stat>
            </CardBody>
          </Card>
        </GridItem>
      </Grid>
    );
  };

  // ✅ Helper function untuk tooltip penjelasan istilah akuntansi
  const getSectionTooltip = (sectionName: string): string => {
    const normalizedName = sectionName.toUpperCase();
    const tooltipKeys: Record<string, string> = {
      'REVENUE': 'reports.profitLoss.tooltips.revenue',
      'COST OF GOODS SOLD': 'reports.profitLoss.tooltips.cogs',
      'GROSS PROFIT': 'reports.profitLoss.tooltips.grossProfit',
      'OPERATING INCOME': 'reports.profitLoss.tooltips.operatingIncome',
      'NET INCOME': 'reports.profitLoss.tooltips.netIncome',
      'OPERATING EXPENSES': 'reports.profitLoss.tooltips.operatingExpenses',
      'OTHER INCOME': 'reports.profitLoss.tooltips.otherIncome',
      'OTHER EXPENSES': 'reports.profitLoss.tooltips.otherExpenses'
    };

    const key = tooltipKeys[normalizedName];
    return key ? t(key) : '';
  };

  // Helper function to translate section names
  const normalizedName = sectionName.toUpperCase();
  const sectionKeys: Record<string, string> = {
    'REVENUE': 'reports.profitLoss.revenue',
    'COST OF GOODS SOLD': 'reports.profitLoss.costOfGoodsSold',
    'GROSS PROFIT': 'reports.profitLoss.grossProfit',
    'OPERATING INCOME': 'reports.profitLoss.operatingIncome',
    'NET INCOME': 'reports.profitLoss.netIncome',
    'OPERATING EXPENSES': 'reports.profitLoss.operatingExpenses',
    'OTHER INCOME': 'reports.profitLoss.otherIncome',
    'OTHER EXPENSES': 'reports.profitLoss.otherExpenses',
    'NET PROFIT': 'reports.profitLoss.netProfit',
    'NET LOSS': 'reports.profitLoss.netLoss'
  };

  const key = sectionKeys[normalizedName];
  // If no exact match, try uppercase keys directly just in case provided name matches key structure
  return key ? t(key) : (sectionKeys[sectionName] ? t(sectionKeys[sectionName]) : t(sectionName, sectionName));
};

// Helper function to translate item names
const getTranslatedItemName = (itemName: string): string => {
  const itemKeys: Record<string, string> = {
    'Gross Profit': 'reports.profitLoss.grossProfit',
    'Gross Profit Margin (%)': 'reports.profitLoss.items.grossProfitMargin',
    'Operating Income': 'reports.profitLoss.operatingIncome',
    'Operating Margin (%)': 'reports.profitLoss.items.operatingMargin',
    'Net Income': 'reports.profitLoss.netIncome',
    'Net Income Margin (%)': 'reports.profitLoss.items.netIncomeMargin',
    'Income Before Tax': 'reports.profitLoss.items.incomeBeforeTax',
    'Tax Expense (25%)': 'reports.profitLoss.items.taxExpense',
    'EBITDA': 'reports.profitLoss.ebitda',
    'EBITDA Margin (%)': 'reports.profitLoss.items.ebitdaMargin',
    'Total Revenue': 'reports.profitLoss.items.totalRevenue',
    'Total Expenses': 'reports.profitLoss.items.totalExpenses',
    'Total COGS': 'reports.profitLoss.items.totalCogs',
    'Total Operating Expenses': 'reports.profitLoss.items.totalOperatingExpenses'
  };

  const key = itemKeys[itemName];
  return key ? t(key) : itemName;
};

const renderSection = (section: any, index: number) => {
  const isCalculated = section.isCalculated;
  const hasSubsections = section.subsections && section.subsections.length > 0;
  const tooltipText = getSectionTooltip(section.name);
  const translatedName = getTranslatedSectionName(section.name);

  // 🔍 DEBUG: Log section name and tooltip availability
  console.log(`[PL Modal] Section: "${section.name}", Has Tooltip: ${!!tooltipText}`);

  return (
    <Card key={index} mb={4} bg={isCalculated ? metricsBg : 'transparent'}>
      <CardBody>
        <Flex justify="space-between" align="center" mb={3}>
          <HStack spacing={2}>
            <Heading size="md" color={textColor}>
              {translatedName}
            </Heading>
            {/* Always render tooltip icon for testing */}
            <Tooltip
              label={tooltipText || `Info for ${translatedName}`}
              fontSize="sm"
              maxW="400px"
              hasArrow
              placement="top"
              bg="blue.600"
              color="white"
              p={3}
              borderRadius="md"
            >
              <Box
                as="span"
                cursor="help"
                color="blue.500"
                fontSize="lg"
                _hover={{ color: "blue.600" }}
                title={tooltipText ? "Has custom tooltip" : "No custom tooltip"}
              >
                ℹ️
              </Box>
            </Tooltip>
          </HStack>
          <Text fontWeight="bold" fontSize="lg" color={textColor}>
            {formatCurrency(section.total)}
          </Text>
        </Flex>

        {hasSubsections ? (
          <Accordion allowMultiple>
            {section.subsections.map((subsection: any, subIndex: number) => (
              <AccordionItem key={subIndex} border="none">
                <AccordionButton
                  bg={subsectionBg}
                  _hover={{ bg: useColorModeValue('blue.100', 'blue.800') }}
                  borderRadius="md"
                  mb={2}
                >
                  <Box flex="1" textAlign="left">
                    <HStack justify="space-between">
                      <Text fontWeight="semibold">{subsection.name}</Text>
                      <Text fontWeight="bold">{formatCurrency(subsection.total)}</Text>
                    </HStack>
                  </Box>
                  <AccordionIcon />
                </AccordionButton>
                <AccordionPanel pb={4}>
                  <VStack spacing={2} align="stretch">
                    {subsection.items.map((item: any, itemIndex: number) => (
                      <HStack key={itemIndex} justify="space-between" pl={4}>
                        <HStack>
                          <Text fontSize="sm" color={secondaryTextColor}>
                            {item.name}
                          </Text>
                          {onJournalDrilldown && item.accountCode && (
                            <JournalDrilldownButton
                              size="xs"
                              onClick={() => onJournalDrilldown(item.name, item.accountCode, item.amount)}
                            />
                          )}
                        </HStack>
                        <Text fontSize="sm" fontWeight="medium">
                          {item.isPercentage ? `${item.amount.toFixed(1)}%` : formatCurrency(item.amount)}
                        </Text>
                      </HStack>
                    ))}
                  </VStack>
                </AccordionPanel>
              </AccordionItem>
            ))}
          </Accordion>
        ) : (
          <VStack spacing={2} align="stretch">
            {section.items?.map((item: any, itemIndex: number) => (
              <HStack key={itemIndex} justify="space-between">
                <HStack>
                  <Text color={secondaryTextColor}>
                    {getTranslatedItemName(item.name)}
                  </Text>
                  {onJournalDrilldown && (
                    <JournalDrilldownButton
                      size="xs"
                      onClick={() => onJournalDrilldown(item.name, item.accountCode, item.amount)}
                    />
                  )}
                </HStack>
                <Text fontWeight={isCalculated ? "bold" : "medium"}>
                  {item.isPercentage ? `${item.amount.toFixed(1)}%` : formatCurrency(item.amount)}
                </Text>
              </HStack>
            ))}
          </VStack>
        )}
      </CardBody>
    </Card>
  );
};

const renderAnalysisTab = () => {
  if (!data.financialMetrics) {
    return (
      <Box textAlign="center" py={8}>
        <Text color={secondaryTextColor}>
          {t('reports.profitLoss.analysisNotAvailable')}
        </Text>
      </Box>
    );
  }

  const metrics = data.financialMetrics;

  return (
    <VStack spacing={6} align="stretch">
      <Card>
        <CardHeader>
          <Heading size="sm">{t('reports.profitLoss.profitabilityAnalysis')}</Heading>
        </CardHeader>
        <CardBody>
          <Table size="sm">
            <Thead>
              <Tr>
                <Th>{t('reports.profitLoss.metric')}</Th>
                <Th isNumeric>{t('reports.profitLoss.value')}</Th>
                <Th isNumeric>{t('reports.profitLoss.percentage')}</Th>
                <Th>{t('reports.profitLoss.assessment')}</Th>
              </Tr>
            </Thead>
            <Tbody>
              <Tr>
                <Td>{t('reports.profitLoss.grossProfit')}</Td>
                <Td isNumeric>{formatCurrency(metrics.grossProfit)}</Td>
                <Td isNumeric>{metrics.grossProfitMargin.toFixed(1)}%</Td>
                <Td>
                  <Badge colorScheme={metrics.grossProfitMargin > 20 ? 'green' : metrics.grossProfitMargin > 10 ? 'yellow' : 'red'}>
                    {metrics.grossProfitMargin > 20 ? t('reports.profitLoss.assessments.excellent') : metrics.grossProfitMargin > 10 ? t('reports.profitLoss.assessments.good') : t('reports.profitLoss.assessments.needsImprovement')}
                  </Badge>
                </Td>
              </Tr>
              <Tr>
                <Td>{t('reports.profitLoss.operatingIncome')}</Td>
                <Td isNumeric>{formatCurrency(metrics.operatingIncome)}</Td>
                <Td isNumeric>{metrics.operatingMargin.toFixed(1)}%</Td>
                <Td>
                  <Badge colorScheme={metrics.operatingMargin > 15 ? 'green' : metrics.operatingMargin > 5 ? 'yellow' : 'red'}>
                    {metrics.operatingMargin > 15 ? t('reports.profitLoss.assessments.strong') : metrics.operatingMargin > 5 ? t('reports.profitLoss.assessments.moderate') : t('reports.profitLoss.assessments.weak')}
                  </Badge>
                </Td>
              </Tr>
              <Tr>
                <Td>{t('reports.profitLoss.ebitda')}</Td>
                <Td isNumeric>{formatCurrency(metrics.ebitda)}</Td>
                <Td isNumeric>{metrics.ebitdaMargin.toFixed(1)}%</Td>
                <Td>
                  <Badge colorScheme={metrics.ebitdaMargin > 20 ? 'green' : metrics.ebitdaMargin > 10 ? 'yellow' : 'red'}>
                    {metrics.ebitdaMargin > 20 ? t('reports.profitLoss.assessments.excellent') : metrics.ebitdaMargin > 10 ? t('reports.profitLoss.assessments.good') : t('reports.profitLoss.assessments.poor')}
                  </Badge>
                </Td>
              </Tr>
              <Tr>
                <Td>{t('reports.profitLoss.netIncome')}</Td>
                <Td isNumeric>{formatCurrency(metrics.netIncome)}</Td>
                <Td isNumeric>{metrics.netIncomeMargin.toFixed(1)}%</Td>
                <Td>
                  <Badge colorScheme={metrics.netIncomeMargin > 10 ? 'green' : metrics.netIncomeMargin > 3 ? 'yellow' : 'red'}>
                    {metrics.netIncomeMargin > 10 ? t('reports.profitLoss.assessments.profitable') : metrics.netIncomeMargin > 3 ? t('reports.profitLoss.assessments.marginal') : t('reports.profitLoss.assessments.unprofitable')}
                  </Badge>
                </Td>
              </Tr>
            </Tbody>
          </Table>
        </CardBody>
      </Card>

      <Card>
        <CardHeader>
          <Heading size="sm">{t('reports.profitLoss.keyInsights')}</Heading>
        </CardHeader>
        <CardBody>
          <VStack spacing={3} align="stretch">
            <Text fontSize="sm">
              🎯 <strong>{t('reports.profitLoss.insights.profitability')}:</strong> {metrics.netIncomeMargin > 0 ? t('reports.profitLoss.insights.profitabilityPositive') : t('reports.profitLoss.insights.profitabilityNegative')}
            </Text>
            <Text fontSize="sm">
              📊 <strong>{t('reports.profitLoss.insights.operatingEfficiency')}:</strong> {metrics.operatingMargin > 10 ? t('reports.profitLoss.insights.operatingEfficiencyStrong') : t('reports.profitLoss.insights.operatingEfficiencyWeak')}
            </Text>
            <Text fontSize="sm">
              💰 <strong>{t('reports.profitLoss.insights.cashGeneration')}:</strong> {t('reports.profitLoss.insights.ebitdaMarginIndicates', { margin: metrics.ebitdaMargin.toFixed(1), strength: metrics.ebitdaMargin > 15 ? t('reports.profitLoss.insights.cashGenerationStrong') : t('reports.profitLoss.insights.cashGenerationModerate') })}
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
    <ModalContent bg={modalBg} maxH="90vh">
      <ModalHeader borderBottom="1px" borderColor={borderColor}>
        <VStack align="stretch" spacing={2}>
          <HStack justify="space-between">
            <VStack align="start" spacing={0}>
              <Text fontSize="xl" fontWeight="bold">
                {t('reports.profitLoss.enhancedTitle', 'Laporan Laba Rugi Ditingkatkan')}
              </Text>
              <Text fontSize="sm" color={secondaryTextColor}>
                {data.period}
              </Text>
              {data.generatedAt && (
                <Text fontSize="xs" color="gray.500" mt={1}>
                  {t('modal.generated')}: {data.generatedAt}
                </Text>
              )}
              {data.company && (
                <Text fontSize="sm" color={secondaryTextColor}>
                  {data.company.name}
                </Text>
              )}
            </VStack>
            {data.enhanced && (
              <Badge colorScheme="blue" variant="solid">
                {t('reports.profitLoss.enhanced')}
              </Badge>
            )}
          </HStack>

          <HStack spacing={1}>
            <Button
              size="sm"
              variant={activeTab === 'statement' ? 'solid' : 'ghost'}
              onClick={() => setActiveTab('statement')}
              leftIcon={<FiDollarSign />}
            >
              {t('reports.profitLoss.statement')}
            </Button>
            {data.enhanced && (
              <>
                <Button
                  size="sm"
                  variant={activeTab === 'metrics' ? 'solid' : 'ghost'}
                  onClick={() => setActiveTab('metrics')}
                  leftIcon={<FiTrendingUp />}
                >
                  {t('reports.profitLoss.metrics')}
                </Button>
                <Button
                  size="sm"
                  variant={activeTab === 'analysis' ? 'solid' : 'ghost'}
                  onClick={() => setActiveTab('analysis')}
                  leftIcon={<FiPieChart />}
                >
                  {t('reports.profitLoss.analysis')}
                </Button>
              </>
            )}
          </HStack>
        </VStack>
      </ModalHeader>
      <ModalCloseButton />

      <ModalBody py={6}>
        {activeTab === 'statement' && (
          <VStack spacing={4} align="stretch">
            {data.enhanced && renderFinancialMetrics()}
            {data.sections?.map(renderSection)}
          </VStack>
        )}

        {activeTab === 'metrics' && data.enhanced && renderFinancialMetrics()}

        {activeTab === 'analysis' && renderAnalysisTab()}
      </ModalBody>

      <ModalFooter borderTop="1px" borderColor={borderColor}>
        <HStack spacing={3}>
          <Button
            leftIcon={<FiDownload />}
            size="sm"
            variant="outline"
            onClick={() => handleExport('pdf')}
          >
            {t('reports.profitLoss.exportPDF')}
          </Button>
          <Button
            leftIcon={<FiDownload />}
            size="sm"
            variant="outline"
            onClick={() => handleExport('excel')}
          >
            {t('reports.profitLoss.exportExcel')}
          </Button>
          <Button onClick={onClose} size="sm">
            {t('reports.profitLoss.close')}
          </Button>
        </HStack>
      </ModalFooter>
    </ModalContent>
  </Modal>
);
};

export default EnhancedProfitLossModal;
