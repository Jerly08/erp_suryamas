import React, { useCallback, useEffect, useState } from "react";
import {
  Alert,
  AlertIcon,
  Badge,
  Box,
  Button,
  Card,
  CardBody,
  FormControl,
  FormLabel,
  Heading,
  HStack,
  Input,
  Select,
  SimpleGrid,
  Spinner,
  Table,
  Tbody,
  Td,
  Text,
  Tr,
  VStack,
  useColorModeValue,
  useToast,
} from "@chakra-ui/react";
import { FiRefreshCw } from "react-icons/fi";
import ExportButton from "@/components/common/ExportButton";
import { useTranslation } from "@/hooks/useTranslation";
import {
  BSAccountItem,
  SSOTBalanceSheetArchiveMetadata,
  SSOTBalanceSheetData,
  ssotBalanceSheetReportService,
} from "@/services/ssotBalanceSheetReportService";
import {
  PeriodFilterOption,
  formatDateOnly,
  periodClosingService,
} from "@/services/periodClosingService";
import {
  exportBalanceSheetToCSV,
  exportBalanceSheetToPDF,
  loadReportLetterhead,
} from "@/utils/journalExport";

type ReportMode = "live" | "archive";

interface FetchError extends Error {
  status?: number;
}

const getLocalDateInputValue = () => {
  const now = new Date();
  const localTime = new Date(now.getTime() - now.getTimezoneOffset() * 60000);
  return localTime.toISOString().split("T")[0];
};

const downloadBlob = (blob: Blob, filename: string) => {
  const downloadUrl = window.URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = downloadUrl;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  window.URL.revokeObjectURL(downloadUrl);
};

export default function BalanceSheetView() {
  const { t } = useTranslation();
  const toast = useToast();
  const cardBg = useColorModeValue("white", "gray.800");
  const textColor = useColorModeValue("gray.800", "gray.100");
  const mutedTextColor = useColorModeValue("gray.500", "gray.400");
  const borderColor = useColorModeValue("gray.200", "whiteAlpha.200");
  const inputBg = useColorModeValue("white", "whiteAlpha.50");
  const inputBorderColor = useColorModeValue("gray.300", "whiteAlpha.300");
  const inputHoverBorderColor = useColorModeValue("gray.400", "whiteAlpha.400");
  const inputFocusBorderColor = useColorModeValue("blue.500", "blue.300");
  const assetHeadingColor = useColorModeValue("green.600", "green.300");
  const assetHeadingBorderColor = useColorModeValue("green.200", "green.700");
  const totalAssetsBg = useColorModeValue("green.100", "green.900");
  const liabilitiesHeadingColor = useColorModeValue("red.600", "red.300");
  const liabilitiesHeadingBorderColor = useColorModeValue("red.200", "red.700");
  const totalLiabilitiesBg = useColorModeValue("red.50", "red.900");
  const equityHeadingColor = useColorModeValue("blue.600", "blue.300");
  const totalEquityBg = useColorModeValue("blue.50", "blue.900");
  const totalLiabilitiesEquityBg = useColorModeValue(
    "purple.100",
    "purple.900",
  );
  const unbalancedBg = useColorModeValue("red.100", "red.900");
  const unbalancedTextColor = useColorModeValue("red.700", "red.200");
  const infoBg = useColorModeValue("blue.50", "blue.900");
  const fieldStyles = {
    bg: inputBg,
    color: textColor,
    borderColor: inputBorderColor,
    _hover: { borderColor: inputHoverBorderColor },
    _focusVisible: {
      borderColor: inputFocusBorderColor,
      boxShadow: `0 0 0 1px ${inputFocusBorderColor}`,
    },
  };

  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<SSOTBalanceSheetData | null>(null);
  const [reportMode, setReportMode] = useState<ReportMode>("live");
  const [asOfDate, setAsOfDate] = useState(getLocalDateInputValue());
  const [archiveMetadata, setArchiveMetadata] =
    useState<SSOTBalanceSheetArchiveMetadata | null>(null);
  const [closedPeriods, setClosedPeriods] = useState<PeriodFilterOption[]>([]);
  const [closedPeriodsLoading, setClosedPeriodsLoading] = useState(false);
  const [selectedClosedPeriodId, setSelectedClosedPeriodId] = useState("");
  const [exporting, setExporting] = useState(false);

  const selectedClosedPeriod =
    closedPeriods.find(
      (periodOption) =>
        String(periodOption.period.id) === selectedClosedPeriodId,
    ) ?? null;
  const effectiveAsOfDate =
    reportMode === "archive"
      ? archiveMetadata?.end_date ||
        selectedClosedPeriod?.period.end_date ||
        asOfDate
      : asOfDate;

  const loadLiveBalanceSheet = useCallback(async (date: string) => {
    const result = await ssotBalanceSheetReportService.generateSSOTBalanceSheet(
      {
        as_of_date: date,
      },
    );
    setData(result);
    setArchiveMetadata(null);
  }, []);

  const formatCurrency = (val: number | undefined) => {
    if (val === undefined) return "-";
    return new Intl.NumberFormat("id-ID", {
      style: "currency",
      currency: "IDR",
      minimumFractionDigits: 0,
    }).format(val);
  };

  const formatLongDate = (value?: string) => {
    return formatDateOnly(value, "id-ID", { dateStyle: "long" });
  };

  const loadClosedPeriods = useCallback(async () => {
    setClosedPeriodsLoading(true);
    try {
      const periods = await periodClosingService.getClosedPeriodsForFilter();
      setClosedPeriods(periods);
      setSelectedClosedPeriodId(
        (current) =>
          current || (periods[0] ? String(periods[0].period.id) : ""),
      );
    } catch (error) {
      console.error("Failed to load closed periods", error);
      setClosedPeriods([]);
    } finally {
      setClosedPeriodsLoading(false);
    }
  }, []);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      if (reportMode === "archive") {
        if (!selectedClosedPeriod) {
          toast({
            title: t("common.error"),
            description: t(
              "journal.balanceSheetReport.archiveSelectionRequired",
            ),
            status: "warning",
            duration: 3000,
          });
          return;
        }

        const result =
          await ssotBalanceSheetReportService.getArchivedSSOTBalanceSheet({
            period_id: selectedClosedPeriod.period.id,
          });

        setData(result.data);
        setArchiveMetadata(result.archive);
        return;
      }

      await loadLiveBalanceSheet(asOfDate);
    } catch (error) {
      console.error("Failed to fetch balance sheet", error);
      const fetchError = error as FetchError;

      if (reportMode === "archive" && fetchError.status === 404) {
        const fallbackDate = selectedClosedPeriod?.period.end_date || asOfDate;
        setReportMode("live");
        setArchiveMetadata(null);
        setAsOfDate(fallbackDate);
        setSelectedClosedPeriodId("");
        await loadClosedPeriods();

        try {
          await loadLiveBalanceSheet(fallbackDate);
        } catch (fallbackError) {
          console.error(
            "Failed to load live balance sheet fallback",
            fallbackError,
          );
        }

        toast({
          title: t("common.warning"),
          description:
            "Periode archive sudah tidak tersedia karena tutup buku telah dibuka kembali. Laporan dialihkan ke mode live.",
          status: "warning",
          duration: 5000,
        });
        return;
      }

      toast({
        title: t("common.error"),
        description: fetchError.message || "Failed to fetch balance sheet",
        status: "error",
        duration: 3000,
      });
    } finally {
      setLoading(false);
    }
  }, [
    asOfDate,
    loadClosedPeriods,
    loadLiveBalanceSheet,
    reportMode,
    selectedClosedPeriod,
    t,
    toast,
  ]);

  useEffect(() => {
    void loadClosedPeriods();
  }, [loadClosedPeriods]);

  useEffect(() => {
    void (async () => {
      setLoading(true);
      try {
        await loadLiveBalanceSheet(getLocalDateInputValue());
      } catch (error) {
        console.error("Failed to fetch initial balance sheet", error);
      } finally {
        setLoading(false);
      }
    })();
  }, [loadLiveBalanceSheet]);

  const handleExportPDF = async () => {
    if (!data) return;
    try {
      setExporting(true);
      if (reportMode === "archive" && archiveMetadata) {
        const letterhead = await loadReportLetterhead({
          companyName: data.company?.name,
        });
        exportBalanceSheetToPDF(data, effectiveAsOfDate, { letterhead });
      } else {
        const blob =
          await ssotBalanceSheetReportService.generateSSOTBalanceSheetPDF({
            as_of_date: effectiveAsOfDate,
          });
        downloadBlob(blob, `Neraca_${effectiveAsOfDate}.pdf`);
      }
      toast({
        title: t("common.success"),
        description: "PDF berhasil diunduh",
        status: "success",
        duration: 3000,
      });
    } catch {
      toast({
        title: t("common.error"),
        description: "Gagal mengexport PDF",
        status: "error",
        duration: 3000,
      });
    } finally {
      setExporting(false);
    }
  };

  const handleExportCSV = async () => {
    if (!data) return;
    try {
      setExporting(true);
      if (reportMode === "archive" && archiveMetadata) {
        exportBalanceSheetToCSV(data, effectiveAsOfDate);
      } else {
        const csvContent =
          await ssotBalanceSheetReportService.generateSSOTBalanceSheetCSV({
            as_of_date: effectiveAsOfDate,
          });
        const csvBlob = new Blob([csvContent], {
          type: "text/csv;charset=utf-8;",
        });
        downloadBlob(csvBlob, `Neraca_${effectiveAsOfDate}.csv`);
      }
      toast({
        title: t("common.success"),
        description: "CSV berhasil diunduh",
        status: "success",
        duration: 3000,
      });
    } catch {
      toast({
        title: t("common.error"),
        description: "Gagal mengexport CSV",
        status: "error",
        duration: 3000,
      });
    } finally {
      setExporting(false);
    }
  };

  const AccountSection = ({
    title,
    items,
    total,
    color = "gray",
  }: {
    title: string;
    items: BSAccountItem[];
    total: number;
    color?: string;
  }) => {
    const sectionTitleColor = useColorModeValue(`${color}.700`, `${color}.200`);
    const sectionTotalBg = useColorModeValue(`${color}.50`, `${color}.900`);

    return (
      <Box mb={6}>
        <Text fontWeight="bold" fontSize="md" mb={2} color={sectionTitleColor}>
          {title}
        </Text>
        <Table size="sm" variant="simple" color={textColor}>
          <Tbody>
            {(items || []).map((item, idx) => (
              <Tr key={idx}>
                <Td pl={4}>
                  {item.account_code} - {item.account_name}
                </Td>
                <Td isNumeric>{formatCurrency(item.amount)}</Td>
              </Tr>
            ))}
            <Tr bg={sectionTotalBg}>
              <Td fontWeight="semibold">Total {title}</Td>
              <Td isNumeric fontWeight="bold">
                {formatCurrency(total)}
              </Td>
            </Tr>
          </Tbody>
        </Table>
      </Box>
    );
  };

  return (
    <Box>
      <Card variant="outline" mb={4} bg={cardBg} borderColor={borderColor}>
        <CardBody py={4}>
          <VStack spacing={4} align="stretch">
            <SimpleGrid columns={{ base: 1, md: 3 }} spacing={4}>
              <FormControl>
                <FormLabel fontSize="sm" color={mutedTextColor}>
                  {t("journal.balanceSheetReport.reportMode")}
                </FormLabel>
                <Select
                  size="sm"
                  value={reportMode}
                  onChange={(e) => setReportMode(e.target.value as ReportMode)}
                  {...fieldStyles}
                >
                  <option value="live">
                    {t("journal.balanceSheetReport.liveMode")}
                  </option>
                  <option value="archive">
                    {t("journal.balanceSheetReport.archiveMode")}
                  </option>
                </Select>
              </FormControl>

              {reportMode === "live" ? (
                <FormControl>
                  <FormLabel fontSize="sm" color={mutedTextColor}>
                    {t("journal.filters.asOfDate")}
                  </FormLabel>
                  <Input
                    type="date"
                    value={asOfDate}
                    onChange={(e) => setAsOfDate(e.target.value)}
                    size="sm"
                    {...fieldStyles}
                  />
                </FormControl>
              ) : (
                <FormControl>
                  <FormLabel fontSize="sm" color={mutedTextColor}>
                    {t("journal.balanceSheetReport.closedPeriod")}
                  </FormLabel>
                  <Select
                    size="sm"
                    value={selectedClosedPeriodId}
                    onChange={(e) => setSelectedClosedPeriodId(e.target.value)}
                    isDisabled={
                      closedPeriodsLoading || closedPeriods.length === 0
                    }
                    {...fieldStyles}
                  >
                    {closedPeriods.length === 0 ? (
                      <option value="">
                        {t("journal.balanceSheetReport.noClosedPeriods")}
                      </option>
                    ) : null}
                    {closedPeriods.map((periodOption) => (
                      <option
                        key={periodOption.period.id}
                        value={String(periodOption.period.id)}
                      >
                        {periodOption.label}
                      </option>
                    ))}
                  </Select>
                </FormControl>
              )}

              <HStack align="end" spacing={3}>
                <Button
                  leftIcon={<FiRefreshCw />}
                  size="sm"
                  colorScheme="blue"
                  onClick={() => void fetchData()}
                  isLoading={loading}
                  isDisabled={reportMode === "archive" && !selectedClosedPeriod}
                >
                  {t("journal.filters.generate")}
                </Button>
                <ExportButton
                  onExportPDF={handleExportPDF}
                  onExportCSV={handleExportCSV}
                  isLoading={exporting}
                  disabled={!data}
                  showExcel={false}
                  showPDF={true}
                  showCSV={true}
                  size="sm"
                />
              </HStack>
            </SimpleGrid>

            <Alert status="info" borderRadius="md" bg={infoBg}>
              <AlertIcon />
              <Box>
                <Text fontWeight="semibold" color={textColor}>
                  {reportMode === "live"
                    ? t("journal.balanceSheetReport.liveModeTitle")
                    : t("journal.balanceSheetReport.archiveModeTitle")}
                </Text>
                <Text fontSize="sm" color={mutedTextColor}>
                  {reportMode === "live"
                    ? t("journal.balanceSheetReport.liveModeHelper")
                    : t("journal.balanceSheetReport.archiveModeHelper")}
                </Text>
              </Box>
            </Alert>
          </VStack>
        </CardBody>
      </Card>

      {loading && !data && (
        <Box textAlign="center" py={10}>
          <Spinner />
        </Box>
      )}

      {data && (
        <Card variant="outline" bg={cardBg} borderColor={borderColor}>
          <CardBody>
            <VStack spacing={6} align="stretch">
              <Box textAlign="center" mb={4}>
                <Heading size="lg" color={textColor}>
                  {data.company?.name}
                </Heading>
                <Text fontSize="xl" fontWeight="medium" color={textColor}>
                  {t("journal.balanceSheet")}
                </Text>
                {reportMode === "archive" && archiveMetadata ? (
                  <VStack spacing={1} mt={2}>
                    <HStack justify="center" spacing={2}>
                      <Text color={mutedTextColor}>
                        {t("journal.balanceSheetReport.archivePeriod")}:
                      </Text>
                      <Text color={textColor}>
                        {formatLongDate(archiveMetadata.start_date)} -{" "}
                        {formatLongDate(archiveMetadata.end_date)}
                      </Text>
                    </HStack>
                    <HStack justify="center" spacing={2}>
                      <Text color={mutedTextColor}>
                        {t("journal.balanceSheetReport.asOf")}:
                      </Text>
                      <Text color={textColor}>
                        {formatLongDate(archiveMetadata.end_date)}
                      </Text>
                      <Badge
                        colorScheme={
                          archiveMetadata.snapshot_available
                            ? "green"
                            : "orange"
                        }
                      >
                        {archiveMetadata.snapshot_available
                          ? t("journal.balanceSheetReport.snapshotReady")
                          : t("journal.balanceSheetReport.snapshotFallback")}
                      </Badge>
                    </HStack>
                  </VStack>
                ) : (
                  <Text color={mutedTextColor}>
                    {t("journal.balanceSheetReport.asOf", {
                      date: formatLongDate(effectiveAsOfDate),
                    })}
                  </Text>
                )}
              </Box>

              <SimpleGrid columns={{ base: 1, md: 2 }} spacing={8}>
                <Box>
                  <Heading
                    size="md"
                    mb={4}
                    color={assetHeadingColor}
                    borderBottomWidth="2px"
                    borderColor={assetHeadingBorderColor}
                    pb={2}
                  >
                    {t("journal.balanceSheetReport.assets")}
                  </Heading>
                  <AccountSection
                    title={t("journal.balanceSheetReport.currentAssets")}
                    items={data.assets.current_assets.items}
                    total={data.assets.current_assets.total_current_assets}
                    color="green"
                  />
                  <AccountSection
                    title={t("journal.balanceSheetReport.nonCurrentAssets")}
                    items={data.assets.non_current_assets.items}
                    total={
                      data.assets.non_current_assets.total_non_current_assets
                    }
                    color="teal"
                  />
                  <Box bg={totalAssetsBg} p={2} borderRadius="md">
                    <HStack justify="space-between">
                      <Text fontWeight="bold" fontSize="lg">
                        {t("journal.balanceSheetReport.totalAssets")}
                      </Text>
                      <Text fontWeight="bold" fontSize="lg">
                        {formatCurrency(data.assets.total_assets)}
                      </Text>
                    </HStack>
                  </Box>
                </Box>

                <Box>
                  <Heading
                    size="md"
                    mb={4}
                    color={liabilitiesHeadingColor}
                    borderBottomWidth="2px"
                    borderColor={liabilitiesHeadingBorderColor}
                    pb={2}
                  >
                    {t("journal.balanceSheetReport.liabilitiesEquity")}
                  </Heading>
                  <AccountSection
                    title={t("journal.balanceSheetReport.currentLiabilities")}
                    items={data.liabilities.current_liabilities.items}
                    total={
                      data.liabilities.current_liabilities
                        .total_current_liabilities
                    }
                    color="red"
                  />
                  <AccountSection
                    title={t(
                      "journal.balanceSheetReport.nonCurrentLiabilities",
                    )}
                    items={data.liabilities.non_current_liabilities.items}
                    total={
                      data.liabilities.non_current_liabilities
                        .total_non_current_liabilities
                    }
                    color="orange"
                  />
                  <Box bg={totalLiabilitiesBg} p={2} borderRadius="md" mb={4}>
                    <HStack justify="space-between">
                      <Text fontWeight="bold">
                        {t("journal.balanceSheetReport.totalLiabilities")}
                      </Text>
                      <Text fontWeight="bold">
                        {formatCurrency(data.liabilities.total_liabilities)}
                      </Text>
                    </HStack>
                  </Box>

                  <Heading size="sm" mb={4} color={equityHeadingColor}>
                    {t("journal.balanceSheetReport.equity")}
                  </Heading>
                  <AccountSection
                    title={t("journal.balanceSheetReport.equitySection")}
                    items={data.equity.items}
                    total={data.equity.total_equity}
                    color="blue"
                  />
                  <Box bg={totalEquityBg} p={2} borderRadius="md" mb={4}>
                    <HStack justify="space-between">
                      <Text fontWeight="bold">
                        {t("journal.balanceSheetReport.totalEquity")}
                      </Text>
                      <Text fontWeight="bold">
                        {formatCurrency(data.equity.total_equity)}
                      </Text>
                    </HStack>
                  </Box>

                  <Box bg={totalLiabilitiesEquityBg} p={2} borderRadius="md">
                    <HStack justify="space-between">
                      <Text fontWeight="bold" fontSize="lg">
                        {t("journal.balanceSheetReport.totalLiabilitiesEquity")}
                      </Text>
                      <Text fontWeight="bold" fontSize="lg">
                        {formatCurrency(data.total_liabilities_and_equity)}
                      </Text>
                    </HStack>
                  </Box>
                </Box>
              </SimpleGrid>

              {!data.is_balanced && (
                <Box
                  bg={unbalancedBg}
                  p={4}
                  borderRadius="md"
                  textAlign="center"
                >
                  <Text color={unbalancedTextColor} fontWeight="bold">
                    {t("journal.balanceSheetReport.unbalanced", {
                      amount: formatCurrency(data.balance_difference),
                    })}
                  </Text>
                </Box>
              )}
            </VStack>
          </CardBody>
        </Card>
      )}
    </Box>
  );
}
