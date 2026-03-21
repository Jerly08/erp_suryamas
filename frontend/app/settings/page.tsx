"use client";

import React, { useCallback, useEffect, useState } from "react";
import { useTranslation } from "@/hooks/useTranslation";
import ProtectedModule from "@/components/common/ProtectedModule";
import SimpleLayout from "@/components/layout/SimpleLayout";
import api from "@/services/api";
import { ReopenPeriodDialog } from "@/components/periods/ReopenPeriodDialog";
import {
  ClosedPeriod,
  formatDateOnly,
  formatDateTime,
  formatPeriodRange,
  PeriodHistoryEvent,
  periodClosingService,
} from "@/services/periodClosingService";
import { API_ENDPOINTS } from "@/config/api";
import { getImageUrl } from "@/utils/imageUrl";
import {
  Box,
  VStack,
  HStack,
  Heading,
  Text,
  Card,
  CardBody,
  CardHeader,
  SimpleGrid,
  Icon,
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
  Spinner,
  useColorModeValue,
  Divider,
  Select,
  Input,
  Textarea,
  NumberInput,
  NumberInputField,
  NumberInputStepper,
  NumberIncrementStepper,
  NumberDecrementStepper,
  FormControl,
  FormLabel,
  useToast,
  Button,
  ButtonGroup,
  Image,
  Stack,
  FormHelperText,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  ModalCloseButton,
  Tooltip,
  Badge,
} from "@chakra-ui/react";
import {
  FiHome,
  FiSettings,
  FiGlobe,
  FiDollarSign,
  FiSave,
  FiX,
  FiCreditCard,
  FiCheckCircle,
  FiInfo,
  FiAlertTriangle,
  FiPackage,
  FiUnlock,
  FiClock,
} from "react-icons/fi";
import Link from "next/link";

function normalizeApiDate(value?: string | null): string {
  if (!value) return "";
  return value.split("T")[0];
}

function getLatestClosableDateString(): string {
  const localNow = new Date();
  const allowSameDayClosing =
    process.env.NEXT_PUBLIC_ALLOW_SAME_DAY_PERIOD_CLOSING === "true" ||
    process.env.NODE_ENV !== "production";
  localNow.setHours(0, 0, 0, 0);
  if (!allowSameDayClosing) {
    localNow.setDate(localNow.getDate() - 1);
  }

  const year = localNow.getFullYear();
  const month = String(localNow.getMonth() + 1).padStart(2, "0");
  const day = String(localNow.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

interface TaxAccountSettings {
  sales_receivable_account?: { id: number; code: string; name: string };
  sales_revenue_account?: { id: number; code: string; name: string };
  purchase_payable_account?: { id: number; code: string; name: string };
  withholding_tax21_account?: { id: number; code: string; name: string };
  withholding_tax23_account?: { id: number; code: string; name: string };
  withholding_tax25_account?: { id: number; code: string; name: string };
  inventory_account?: { id: number; code: string; name: string };
  cogs_account?: { id: number; code: string; name: string };
  is_active?: boolean;
  updated_at?: string;
}

interface PeriodClosingAccount {
  id: number;
  code: string;
  name: string;
  balance: number;
}

interface PeriodClosingEntry {
  description: string;
  debit_account: string;
  credit_account: string;
  amount: number;
}

interface PeriodClosingPreview {
  start_date: string;
  end_date: string;
  total_revenue: number;
  total_expense: number;
  net_income: number;
  retained_earnings_id: number;
  can_close: boolean;
  has_blocking_issues?: boolean;
  blocking_messages?: string[];
  validation_messages: string[];
  revenue_accounts: PeriodClosingAccount[];
  expense_accounts: PeriodClosingAccount[];
  closing_entries: PeriodClosingEntry[];
  transaction_count: number;
  period_days: number;
}

interface PeriodClosingLastInfo {
  has_previous_closing: boolean;
  last_closing_date?: string | null;
  next_start_date?: string | null;
  period_start_date?: string | null;
  last_period_start_date?: string | null;
  last_period_end_date?: string | null;
  requires_reclose?: boolean;
  reopened_period_start?: string | null;
  reopened_period_end?: string | null;
  future_closed_pending?: boolean;
}

interface ApiErrorLike {
  message?: string;
  response?: {
    data?: {
      error?: string;
      details?: string;
    };
  };
}

const getApiErrorMessage = (error: unknown, fallback: string): string => {
  const apiError = error as ApiErrorLike;
  return (
    apiError.response?.data?.details ||
    apiError.response?.data?.error ||
    apiError.message ||
    fallback
  );
};

interface SystemSettings {
  id?: number;
  company_name: string;
  company_address: string;
  company_phone: string;
  company_email: string;
  company_type?: string; // Added field
  company_website?: string;
  company_logo?: string;
  tax_number?: string;
  currency: string;
  date_format: string;
  fiscal_year_start: string;
  default_tax_rate: number;
  language: string;
  timezone: string;
  thousand_separator: string;
  decimal_separator: string;
  decimal_places: number;
  invoice_prefix: string;
  invoice_next_number?: number; // removed from API - keep optional for backward compatibility
  quote_prefix: string;
  quote_next_number?: number; // removed from API - keep optional for backward compatibility
  purchase_prefix: string;
  purchase_next_number?: number; // removed from API - keep optional for backward compatibility
  // New: Payment prefixes
  payment_receivable_prefix?: string;
  payment_payable_prefix?: string;
  email_notifications: boolean;
  smtp_host?: string;
  smtp_port?: number;
  smtp_username?: string;
  smtp_from?: string;
}

const SettingsPage: React.FC = () => {
  const { t, language, setLanguage } = useTranslation();
  const toast = useToast();
  const [settings, setSettings] = useState<SystemSettings | null>(null);
  const [formData, setFormData] = useState<SystemSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [hasChanges, setHasChanges] = useState(false);
  const [uploadingLogo, setUploadingLogo] = useState(false);
  // Tax account settings
  const [taxAccountSettings, setTaxAccountSettings] =
    useState<TaxAccountSettings | null>(null);
  const [loadingTaxAccounts, setLoadingTaxAccounts] = useState(false);

  // Period closing (flexible period closing)
  const [periodStartDate, setPeriodStartDate] = useState("");
  const [periodEndDate, setPeriodEndDate] = useState("");
  const [periodClosingPreview, setPeriodClosingPreview] =
    useState<PeriodClosingPreview | null>(null);
  const [loadingPeriodPreview, setLoadingPeriodPreview] = useState(false);
  const [showPeriodClosingModal, setShowPeriodClosingModal] = useState(false);
  const [executingPeriodClosing, setExecutingPeriodClosing] = useState(false);
  const [latestClosableDate, setLatestClosableDate] = useState("");
  const [lastClosingInfo, setLastClosingInfo] =
    useState<PeriodClosingLastInfo | null>(null);
  const [showClosingHistoryModal, setShowClosingHistoryModal] = useState(false);

  // Reopen period
  const [showReopenDialog, setShowReopenDialog] = useState(false);
  const [reopenStartDate, setReopenStartDate] = useState("");
  const [reopenEndDate, setReopenEndDate] = useState("");

  // Closing history
  const [closingHistory, setClosingHistory] = useState<ClosedPeriod[]>([]);
  const [loadingHistory, setLoadingHistory] = useState(false);

  // Move useColorModeValue to top level to fix hooks order
  const blueColor = useColorModeValue("blue.500", "blue.300");
  const greenColor = useColorModeValue("green.500", "green.300");
  const redColor = useColorModeValue("red.500", "red.300");
  const orangeColor = useColorModeValue("orange.500", "orange.300");
  const headingColor = useColorModeValue("gray.800", "whiteAlpha.900");
  const textColor = useColorModeValue("gray.700", "whiteAlpha.900");
  const mutedTextColor = useColorModeValue("gray.600", "whiteAlpha.700");
  const cardBg = useColorModeValue("white", "gray.800");
  const cardBorderColor = useColorModeValue("gray.200", "whiteAlpha.200");
  const modalBg = useColorModeValue("white", "gray.800");
  const modalHeaderBg = useColorModeValue("red.500", "red.400");
  const modalFooterBg = useColorModeValue("gray.50", "gray.750");
  const panelBg = useColorModeValue("gray.50", "whiteAlpha.100");
  const itemBg = useColorModeValue("white", "whiteAlpha.50");
  const inputBg = useColorModeValue("gray.50", "whiteAlpha.100");
  const inputHoverBg = useColorModeValue("white", "whiteAlpha.160");
  const inputFocusBg = useColorModeValue("white", "gray.700");
  const inputBorderColor = useColorModeValue("gray.200", "whiteAlpha.200");
  const inputHoverBorderColor = useColorModeValue("gray.300", "whiteAlpha.300");
  const inputFocusBorderColor = useColorModeValue("blue.400", "blue.300");
  const inputPlaceholderColor = useColorModeValue("gray.500", "whiteAlpha.500");
  const readOnlyInputBg = useColorModeValue("gray.100", "whiteAlpha.120");
  const lockedFieldBg = useColorModeValue("blue.50", "blue.900");
  const lockedFieldBorderColor = useColorModeValue("blue.200", "blue.700");
  const lockedHelperColor = useColorModeValue("blue.600", "blue.200");
  const successPanelBg = useColorModeValue("green.50", "green.900");
  const successPanelBorderColor = useColorModeValue("green.200", "green.700");
  const warningPanelBg = useColorModeValue("orange.50", "orange.900");
  const warningPanelBorderColor = useColorModeValue("orange.200", "orange.700");
  const subtleHoverBg = useColorModeValue("orange.50", "orange.900");
  const dividerColor = useColorModeValue("gray.200", "whiteAlpha.200");
  const fieldStyles = {
    bg: inputBg,
    borderColor: inputBorderColor,
    color: textColor,
    _placeholder: { color: inputPlaceholderColor },
    _hover: {
      bg: inputHoverBg,
      borderColor: inputHoverBorderColor,
    },
    _focus: {
      bg: inputFocusBg,
      borderColor: inputFocusBorderColor,
    },
  };
  const locale = language === "id" ? "id-ID" : "en-US";
  const formatPeriodDate = (value?: string | null) =>
    formatDateOnly(value, locale, {
      day: "numeric",
      month: "short",
      year: "numeric",
    });
  const formatPeriodDateTime = (value?: string | null) =>
    formatDateTime(value, locale);
  const formatClosingRange = (
    startDate?: string | null,
    endDate?: string | null,
  ) => formatPeriodRange(startDate, endDate, locale);
  const formatPeriodHistoryDescription = (
    item: ClosedPeriod,
    event: PeriodHistoryEvent,
  ) => {
    let description = event.description;

    if (event.type === "CLOSED" && item.closed_by_name) {
      description = description.replace(
        /oleh user #\d+/i,
        `oleh ${item.closed_by_name}`,
      );
    }

    if (event.type === "REOPENED" && item.reopened_by_name) {
      description = description.replace(
        /oleh user #\d+/i,
        `oleh ${item.reopened_by_name}`,
      );
    }

    return description;
  };
  const periodClosingInfoMessages =
    periodClosingPreview?.validation_messages.filter(
      (msg) => !periodClosingPreview.blocking_messages?.includes(msg),
    ) ?? [];
  const latestClosingHistory = closingHistory[0] ?? null;

  const renderClosingHistoryItem = (
    item: ClosedPeriod,
    eventLimit?: number,
  ) => (
    <Box
      key={item.id}
      p={3}
      borderRadius="md"
      border="1px"
      borderColor={
        item.is_closed ? successPanelBorderColor : warningPanelBorderColor
      }
      bg={item.is_closed ? successPanelBg : warningPanelBg}
    >
      <HStack justify="space-between" mb={1} align="start" spacing={3}>
        <HStack spacing={2} flexWrap="wrap" align="center">
          <Badge
            colorScheme={item.is_closed ? "green" : "orange"}
            fontSize="2xs"
          >
            {item.is_closed ? "Tertutup" : "Terbuka"}
          </Badge>
          {item.can_reopen ? (
            <Badge colorScheme="blue" variant="subtle" fontSize="2xs">
              Periode Terbaru
            </Badge>
          ) : null}
          <Text fontSize="xs" fontWeight="semibold">
            {item.start_date && !item.start_date.startsWith("0001")
              ? formatPeriodDate(item.start_date)
              : "-"}
            {" - "}
            {item.end_date && !item.end_date.startsWith("0001")
              ? formatPeriodDate(item.end_date)
              : "-"}
          </Text>
          <Text fontSize="xs" color={mutedTextColor}>
            {item.description}
          </Text>
        </HStack>
        {item.can_reopen ? (
          <Button
            size="xs"
            variant="outline"
            colorScheme="orange"
            leftIcon={<FiUnlock />}
            onClick={() => {
              setShowClosingHistoryModal(false);
              setReopenStartDate(normalizeApiDate(item.start_date));
              setReopenEndDate(normalizeApiDate(item.end_date));
              setShowReopenDialog(true);
            }}
          >
            Buka Kembali
          </Button>
        ) : null}
      </HStack>
      <HStack spacing={4} fontSize="xs" color={mutedTextColor} flexWrap="wrap">
        <Text>
          Revenue: Rp {(item.total_revenue || 0).toLocaleString("id-ID")}
        </Text>
        <Text>
          Expense: Rp {(item.total_expense || 0).toLocaleString("id-ID")}
        </Text>
        <Text
          fontWeight="semibold"
          color={item.net_income >= 0 ? "green.600" : "red.600"}
        >
          Net: Rp {(item.net_income || 0).toLocaleString("id-ID")}
        </Text>
        <Text>
          Transaksi: {(item.transaction_count || 0).toLocaleString("id-ID")}
        </Text>
        <Text>Reopen: {(item.reopen_count || 0).toLocaleString("id-ID")}x</Text>
      </HStack>
      {item.closed_at || item.snapshot_generated_at ? (
        <HStack
          spacing={4}
          fontSize="xs"
          color={mutedTextColor}
          mt={2}
          flexWrap="wrap"
        >
          {item.closed_at ? (
            <Text>Ditutup: {formatPeriodDateTime(item.closed_at)}</Text>
          ) : null}
          {item.closed_by_name ? (
            <Text>
              Oleh: <strong>{item.closed_by_name}</strong>
            </Text>
          ) : null}
          {item.snapshot_generated_at ? (
            <Text>
              Snapshot: {formatPeriodDateTime(item.snapshot_generated_at)}
            </Text>
          ) : null}
        </HStack>
      ) : null}
      {item.history_events && item.history_events.length > 0 ? (
        <VStack spacing={1} alignItems="stretch" mt={2}>
          {item.history_events
            .slice(0, eventLimit)
            .map((event, index) => (
              <Box
                key={`${item.id}-${event.type}-${index}`}
                p={2}
                bg={itemBg}
                borderRadius="md"
              >
                <HStack justify="space-between" align="start">
                  <Text fontSize="xs" fontWeight="semibold" color={textColor}>
                    {event.title}
                  </Text>
                  {event.at ? (
                    <Text fontSize="2xs" color={mutedTextColor}>
                      {formatPeriodDateTime(event.at)}
                    </Text>
                  ) : null}
                </HStack>
                <Text fontSize="xs" color={mutedTextColor}>
                  {formatPeriodHistoryDescription(item, event)}
                </Text>
              </Box>
            ))}
        </VStack>
      ) : item.notes ? (
        <Text fontSize="xs" color={mutedTextColor} mt={2} fontStyle="italic">
          {item.notes}
        </Text>
      ) : null}
    </Box>
  );

  const fetchTaxAccountSettings = useCallback(async () => {
    setLoadingTaxAccounts(true);
    try {
      const response = await api.get("/api/v1/tax-accounts/current");
      if (response.data.success) {
        setTaxAccountSettings(response.data.data);
      }
    } catch (err: unknown) {
      console.error("Error fetching tax account settings:", err);
      // Don't show error if tax accounts are not configured yet
    } finally {
      setLoadingTaxAccounts(false);
    }
  }, []);

  // Fetch last closing info to auto-populate start date
  const fetchLastClosingInfo = useCallback(async () => {
    try {
      const response = await api.get("/api/v1/period-closing/last-info");
      if (response.data.success && response.data.data) {
        const info = response.data.data as PeriodClosingLastInfo;
        setLastClosingInfo(info);

        // Auto-populate start date based on last closing
        if (
          info.requires_reclose &&
          info.reopened_period_start &&
          info.reopened_period_end
        ) {
          const reopenedStart = info.reopened_period_start.split("T")[0];
          const reopenedEnd = info.reopened_period_end.split("T")[0];
          setPeriodStartDate(reopenedStart);
          setPeriodEndDate(reopenedEnd);
          console.log(
            "Period dates set from reopened period:",
            reopenedStart,
            reopenedEnd,
          );
        } else if (info.has_previous_closing && info.next_start_date) {
          // Has previous closing - MUST use next start date (day after last closing)
          const nextStart = info.next_start_date.split("T")[0];
          setPeriodStartDate(nextStart);
          console.log("Period start date set from last closing:", nextStart);
        } else if (!info.has_previous_closing && info.period_start_date) {
          // First closing - use earliest transaction date
          const firstDate = info.period_start_date.split("T")[0];
          setPeriodStartDate(firstDate);
          console.log(
            "Period start date set from earliest transaction:",
            firstDate,
          );
        } else {
          // No data available - let user input manually
          console.log(
            "No closing info available, user must input start date manually",
          );
        }
      }
    } catch (err) {
      console.error("Failed to fetch last closing info:", err);
      // On error, just let user input dates manually
    }
  }, []);

  // Fetch closing history
  const fetchClosingHistory = useCallback(async () => {
    setLoadingHistory(true);
    try {
      const history = await periodClosingService.getClosingHistory(10);
      setClosingHistory(history);
    } catch (err) {
      console.error("Failed to fetch closing history:", err);
    } finally {
      setLoadingHistory(false);
    }
  }, []);

  const handlePreviewPeriodClosing = async () => {
    if (!periodStartDate || !periodEndDate) {
      toast({
        title: "Dates required",
        description: "Please select both start and end dates",
        status: "warning",
        duration: 3000,
      });
      return;
    }

    if (
      latestClosableDate &&
      lastClosingInfo?.requires_reclose !== true &&
      periodEndDate > latestClosableDate
    ) {
      toast({
        title: "Periode belum bisa ditutup",
        description: `Tanggal akhir maksimal untuk close period saat ini adalah ${latestClosableDate}. Tanggal masa depan belum bisa diproses.`,
        status: "warning",
        duration: 5000,
        isClosable: true,
      });
      return;
    }

    setLoadingPeriodPreview(true);
    try {
      const response = await api.get(
        `/api/v1/period-closing/preview?start_date=${periodStartDate}&end_date=${periodEndDate}`,
      );
      if (response.data.success) {
        setPeriodClosingPreview(response.data.data);
        setShowPeriodClosingModal(true);
      }
    } catch (err: unknown) {
      toast({
        title: "Failed to preview closing",
        description: getApiErrorMessage(err, "Failed to preview closing"),
        status: "error",
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setLoadingPeriodPreview(false);
    }
  };

  const handleExecutePeriodClosing = async () => {
    if (!periodClosingPreview) return;

    setExecutingPeriodClosing(true);
    try {
      const response = await api.post("/api/v1/period-closing/execute", {
        start_date: periodStartDate,
        end_date: periodEndDate,
        description: `Period closing from ${periodStartDate} to ${periodEndDate}`,
        notes: "Period closing executed from settings",
      });
      if (response.data.success) {
        toast({
          title: "Period Closed Successfully!",
          description:
            "All revenue and expense accounts have been reset and transferred to retained earnings",
          status: "success",
          duration: 5000,
          isClosable: true,
        });
        setShowPeriodClosingModal(false);
        setPeriodClosingPreview(null);

        // Refresh settings to update fiscal year start (auto-updated by backend)
        console.log("Refreshing settings to get updated fiscal_year_start...");
        await fetchSettings();

        // Refresh last closing info
        await fetchLastClosingInfo();
        await fetchClosingHistory();

        console.log("Settings and closing info refreshed");
      }
    } catch (err: unknown) {
      toast({
        title: "Failed to close period",
        description: getApiErrorMessage(err, "Failed to close period"),
        status: "error",
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setExecutingPeriodClosing(false);
    }
  };

  const fetchSettings = useCallback(async () => {
    setLoading(true);
    try {
      const response = await api.get(API_ENDPOINTS.SETTINGS);
      if (response.data.success) {
        const fiscalYearStart = response.data.data?.fiscal_year_start;

        console.log("Settings loaded:");
        console.log("  - fiscal_year_start (DB):", fiscalYearStart);

        setSettings(response.data.data);
        setFormData(response.data.data);
        setHasChanges(false);
        // Sync language from settings
        if (
          response.data.data.language &&
          response.data.data.language !== language
        ) {
          setLanguage(response.data.data.language);
        }
      }
    } catch (err: unknown) {
      console.error("Error fetching settings:", err);
      setError(getApiErrorMessage(err, "Failed to fetch settings"));
    } finally {
      setLoading(false);
    }
  }, [language, setLanguage]);

  useEffect(() => {
    setLatestClosableDate(getLatestClosableDateString());
  }, []);

  useEffect(() => {
    void fetchSettings();
    void fetchTaxAccountSettings();
    void fetchLastClosingInfo();
    void fetchClosingHistory();
  }, [
    fetchSettings,
    fetchTaxAccountSettings,
    fetchLastClosingInfo,
    fetchClosingHistory,
  ]);

  const handleFormChange = (
    field: keyof SystemSettings,
    value: SystemSettings[keyof SystemSettings],
  ) => {
    setFormData((prev) => {
      const baseSettings = prev ?? settings;
      if (!baseSettings) return prev;

      return {
        ...baseSettings,
        [field]: value,
      };
    });
    setHasChanges(true);
  };

  const handleSave = async () => {
    if (!formData || !hasChanges) return;

    setSaving(true);
    try {
      const response = await api.put(API_ENDPOINTS.SETTINGS_UPDATE, formData);
      if (response.data.success) {
        setSettings(response.data.data);
        setFormData(response.data.data);
        setHasChanges(false);

        // Update language if changed
        if (response.data.data.language !== language) {
          setLanguage(response.data.data.language);
        }

        toast({
          title: t("settings.updateSuccess") || "Settings updated successfully",
          status: "success",
          duration: 3000,
          isClosable: true,
        });
      }
    } catch (error: unknown) {
      toast({
        title: t("settings.updateError") || "Failed to update settings",
        description: getApiErrorMessage(error, "Failed to update settings"),
        status: "error",
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setSaving(false);
    }
  };

  const handleCancel = () => {
    setFormData(settings);
    setHasChanges(false);
  };

  const handleLogoUpload = async (file: File) => {
    if (!file) return;
    setUploadingLogo(true);
    try {
      const fd = new FormData();
      fd.append("image", file);
      const resp = await api.post("/api/v1/settings/company/logo", fd, {
        headers: { "Content-Type": "multipart/form-data" },
      });
      const newPath: string | undefined = resp.data?.path;
      if (newPath) {
        setSettings((prev) =>
          prev ? { ...prev, company_logo: newPath } : prev,
        );
        setFormData((prev) =>
          prev ? { ...prev, company_logo: newPath } : prev,
        );
        toast({
          title:
            t("settings.taxAccounts.messages.logoUpdated") || "Logo updated",
          status: "success",
          duration: 2500,
          isClosable: true,
        });
      } else {
        toast({
          title:
            t("settings.taxAccounts.messages.uploadSuccessNoPath") ||
            "Upload succeeded but no path returned",
          status: "warning",
          duration: 3000,
          isClosable: true,
        });
      }
    } catch (err: unknown) {
      toast({
        title:
          t("settings.taxAccounts.messages.logoUploadFailed") ||
          "Failed to upload logo",
        description: getApiErrorMessage(err, "Failed to upload logo"),
        status: "error",
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setUploadingLogo(false);
    }
  };

  // Add safety check for formData initialization
  useEffect(() => {
    if (settings && !formData) {
      setFormData(settings);
    }
  }, [settings, formData]);

  // Loading state - moved after all hooks
  if (loading) {
    return (
      <ProtectedModule module="settings">
        <SimpleLayout>
          <Box>
            <Spinner size="xl" thickness="4px" speed="0.65s" color="blue.500" />
            <Text ml={4}>{t("common.loading")}</Text>
          </Box>
        </SimpleLayout>
      </ProtectedModule>
    );
  }

  // Don't render form until settings are loaded
  if (!settings) {
    return (
      <ProtectedModule module="settings">
        <SimpleLayout>
          <Box>
            <Text>No settings data available</Text>
          </Box>
        </SimpleLayout>
      </ProtectedModule>
    );
  }

  return (
    <ProtectedModule module="settings">
      <SimpleLayout>
        <Box>
          <VStack spacing={6} alignItems="start">
            <HStack justify="space-between" width="full">
              <Heading as="h1" size="xl" color={headingColor}>
                {t("settings.title")}
              </Heading>
              {hasChanges && (
                <ButtonGroup>
                  <Button
                    colorScheme="green"
                    leftIcon={<FiSave />}
                    onClick={handleSave}
                    isLoading={saving}
                    loadingText={t("common.saving") || "Saving..."}
                    size="sm"
                  >
                    {t("common.buttons.saveChanges") || "Save Changes"}
                  </Button>
                  <Button
                    variant="outline"
                    leftIcon={<FiX />}
                    onClick={handleCancel}
                    isDisabled={saving}
                    size="sm"
                  >
                    {t("common.cancel") || "Cancel"}
                  </Button>
                </ButtonGroup>
              )}
            </HStack>

            {error && (
              <Alert status="error" width="full">
                <AlertIcon />
                <AlertTitle>{t("common.error")}:</AlertTitle>
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}

            <SimpleGrid columns={[1, 1, 2, 2]} spacing={6} width="full">
              {/* Company Information Card */}
              <Card
                bg={cardBg}
                border="1px"
                borderColor={cardBorderColor}
                boxShadow="md"
              >
                <CardHeader>
                  <HStack spacing={3}>
                    <Icon as={FiHome} boxSize={6} color={blueColor} />
                    <Heading size="md">{t("settings.companyInfo")}</Heading>
                  </HStack>
                </CardHeader>
                <CardBody>
                  <VStack spacing={4} alignItems="start">
                    {/* Company Logo */}
                    <FormControl>
                      <FormLabel
                        fontWeight="semibold"
                        color={mutedTextColor}
                        fontSize="sm"
                      >
                        {t("settings.companyLogo") || "Company Logo"}
                      </FormLabel>
                      <Stack
                        direction={{ base: "column", md: "row" }}
                        spacing={4}
                        align="center"
                      >
                        <Image
                          src={
                            getImageUrl(
                              formData?.company_logo ||
                                settings?.company_logo ||
                                "",
                            ) || undefined
                          }
                          alt="Company Logo"
                          boxSize="80px"
                          objectFit="contain"
                          borderRadius="md"
                          fallbackStrategy="onError"
                        />
                        <Input
                          type="file"
                          accept="image/*"
                          variant="filled"
                          {...fieldStyles}
                          onChange={(e) => {
                            const f = e.target.files?.[0];
                            if (f) handleLogoUpload(f);
                          }}
                          isDisabled={uploadingLogo}
                        />
                        {uploadingLogo && <Spinner size="sm" />}
                      </Stack>
                    </FormControl>
                    <Divider borderColor={dividerColor} />

                    {/* Company Type (Read Only) */}
                    <FormControl>
                      <FormLabel
                        fontWeight="semibold"
                        color={mutedTextColor}
                        fontSize="sm"
                      >
                        {t("settings.companyType") || "Company Type"}
                      </FormLabel>
                      <HStack>
                        <Input
                          value={
                            formData?.company_type
                              ? t(
                                  `settings.companyTypes.${formData.company_type.toLowerCase()}`,
                                )
                              : t("settings.notSet")
                          }
                          isReadOnly
                          variant="filled"
                          bg={readOnlyInputBg}
                          borderColor={inputBorderColor}
                          color={mutedTextColor}
                          cursor="not-allowed"
                          _hover={{
                            bg: readOnlyInputBg,
                            borderColor: inputBorderColor,
                          }}
                          _focus={{
                            bg: readOnlyInputBg,
                            borderColor: inputBorderColor,
                          }}
                        />
                        {formData?.company_type && (
                          <Tooltip label={t("settings.companyTypeLocked")}>
                            <Box color={mutedTextColor} cursor="help">
                              <Icon as={FiInfo} />
                            </Box>
                          </Tooltip>
                        )}
                      </HStack>
                      {!formData?.company_type && (
                        <FormHelperText fontSize="xs" color={orangeColor}>
                          {t("settings.companyTypeHelperText")}
                        </FormHelperText>
                      )}
                    </FormControl>
                    <Divider borderColor={dividerColor} />

                    <FormControl>
                      <FormLabel
                        fontWeight="semibold"
                        color={mutedTextColor}
                        fontSize="sm"
                      >
                        {t("settings.companyName")}
                      </FormLabel>
                      <Input
                        value={
                          formData?.company_name ?? settings?.company_name ?? ""
                        }
                        onChange={(e) =>
                          handleFormChange("company_name", e.target.value)
                        }
                        placeholder={
                          t("settings.companyNamePlaceholder") ||
                          "Enter company name"
                        }
                        variant="filled"
                        {...fieldStyles}
                      />
                    </FormControl>
                    <Divider borderColor={dividerColor} />

                    <FormControl>
                      <FormLabel
                        fontWeight="semibold"
                        color={mutedTextColor}
                        fontSize="sm"
                      >
                        {t("settings.address")}
                      </FormLabel>
                      <Textarea
                        value={
                          formData?.company_address ??
                          settings?.company_address ??
                          ""
                        }
                        onChange={(e) =>
                          handleFormChange("company_address", e.target.value)
                        }
                        placeholder={
                          t("settings.addressPlaceholder") ||
                          "Enter company address"
                        }
                        rows={3}
                        variant="filled"
                        {...fieldStyles}
                      />
                    </FormControl>
                    <Divider borderColor={dividerColor} />

                    <FormControl>
                      <FormLabel
                        fontWeight="semibold"
                        color={mutedTextColor}
                        fontSize="sm"
                      >
                        {t("settings.phone")}
                      </FormLabel>
                      <Input
                        value={
                          formData?.company_phone ??
                          settings?.company_phone ??
                          ""
                        }
                        onChange={(e) =>
                          handleFormChange("company_phone", e.target.value)
                        }
                        placeholder={
                          t("settings.phonePlaceholder") || "Enter phone number"
                        }
                        type="tel"
                        variant="filled"
                        {...fieldStyles}
                      />
                    </FormControl>
                    <Divider borderColor={dividerColor} />

                    <FormControl>
                      <FormLabel
                        fontWeight="semibold"
                        color={mutedTextColor}
                        fontSize="sm"
                      >
                        {t("settings.email")}
                      </FormLabel>
                      <Input
                        value={
                          formData?.company_email ??
                          settings?.company_email ??
                          ""
                        }
                        onChange={(e) =>
                          handleFormChange("company_email", e.target.value)
                        }
                        placeholder={
                          t("settings.emailPlaceholder") ||
                          "Enter email address"
                        }
                        type="email"
                        variant="filled"
                        {...fieldStyles}
                      />
                    </FormControl>
                    <Divider borderColor={dividerColor} />

                    <FormControl>
                      <FormLabel
                        fontWeight="semibold"
                        color={mutedTextColor}
                        fontSize="sm"
                      >
                        {t("settings.taxNumber")}
                      </FormLabel>
                      <Input
                        value={
                          formData?.tax_number ?? settings?.tax_number ?? ""
                        }
                        onChange={(e) =>
                          handleFormChange("tax_number", e.target.value)
                        }
                        placeholder={
                          t("settings.taxNumberPlaceholder") ||
                          "Enter tax number (optional)"
                        }
                        variant="filled"
                        {...fieldStyles}
                      />
                    </FormControl>
                  </VStack>
                </CardBody>
              </Card>

              {/* System Configuration Card */}
              <Card
                bg={cardBg}
                border="1px"
                borderColor={cardBorderColor}
                boxShadow="md"
              >
                <CardHeader>
                  <HStack spacing={3}>
                    <Icon as={FiDollarSign} boxSize={6} color={greenColor} />
                    <Heading size="md">{t("settings.systemConfig")}</Heading>
                  </HStack>
                </CardHeader>
                <CardBody>
                  <VStack spacing={4} alignItems="start">
                    <FormControl>
                      <FormLabel
                        fontWeight="semibold"
                        color={mutedTextColor}
                        fontSize="sm"
                      >
                        {t("settings.dateFormat")}
                      </FormLabel>
                      <Select
                        value={
                          formData?.date_format ||
                          settings?.date_format ||
                          "YYYY-MM-DD"
                        }
                        onChange={(e) =>
                          handleFormChange("date_format", e.target.value)
                        }
                        variant="filled"
                        {...fieldStyles}
                      >
                        <option value="YYYY-MM-DD">YYYY-MM-DD</option>
                        <option value="DD/MM/YYYY">DD/MM/YYYY</option>
                        <option value="MM/DD/YYYY">MM/DD/YYYY</option>
                        <option value="DD-MM-YYYY">DD-MM-YYYY</option>
                      </Select>
                    </FormControl>
                    <Divider borderColor={dividerColor} />

                    <FormControl>
                      <HStack
                        mb={2}
                        justify="space-between"
                        width="full"
                        align="center"
                      >
                        <FormLabel
                          fontWeight="semibold"
                          color={mutedTextColor}
                          fontSize="sm"
                          mb={0}
                        >
                          {t("settings.defaultTaxRate")} (%)
                        </FormLabel>
                        <Badge
                          colorScheme="orange"
                          borderRadius="md"
                          px={2}
                          py={1}
                        >
                          {(
                            formData?.default_tax_rate ??
                            settings?.default_tax_rate ??
                            0
                          ).toFixed(2)}
                          %
                        </Badge>
                      </HStack>
                      <NumberInput
                        value={
                          formData?.default_tax_rate ??
                          settings?.default_tax_rate ??
                          0
                        }
                        onChange={(_, valueNumber) =>
                          handleFormChange(
                            "default_tax_rate",
                            Number.isFinite(valueNumber)
                              ? Math.min(Math.max(valueNumber, 0), 100)
                              : 0,
                          )
                        }
                        min={0}
                        max={100}
                        precision={2}
                        step={0.5}
                      >
                        <NumberInputField
                          placeholder="11.00"
                          {...fieldStyles}
                        />
                        <NumberInputStepper>
                          <NumberIncrementStepper />
                          <NumberDecrementStepper />
                        </NumberInputStepper>
                      </NumberInput>
                      <FormHelperText fontSize="xs" color={mutedTextColor}>
                        Dipakai sebagai PPN default untuk transaksi penjualan
                        dan pembelian baru. Nilai ini hanya menjadi nilai awal
                        dan masih bisa diubah per transaksi.
                      </FormHelperText>
                      <FormHelperText fontSize="xs" color={mutedTextColor}>
                        Contoh umum: 11 untuk PPN 11%.
                      </FormHelperText>
                    </FormControl>
                    <Divider borderColor={dividerColor} />

                    <FormControl>
                      <HStack mb={2}>
                        <Icon as={FiGlobe} boxSize={4} color={mutedTextColor} />
                        <FormLabel
                          fontWeight="semibold"
                          color={mutedTextColor}
                          fontSize="sm"
                          mb={0}
                        >
                          {t("settings.language")}
                        </FormLabel>
                      </HStack>
                      <Select
                        value={
                          formData?.language || settings?.language || language
                        }
                        onChange={(e) => {
                          handleFormChange("language", e.target.value);
                        }}
                        variant="filled"
                        {...fieldStyles}
                      >
                        <option value="id">{t("settings.indonesian")}</option>
                        <option value="en">{t("settings.english")}</option>
                      </Select>
                    </FormControl>
                  </VStack>
                </CardBody>
              </Card>

              {/* Flexible Period Closing Card */}
              <Card
                bg={cardBg}
                border="1px"
                borderColor={cardBorderColor}
                boxShadow="md"
              >
                <CardHeader>
                  <HStack spacing={3} justify="space-between" width="full">
                    <HStack spacing={3}>
                      <Icon as={FiCheckCircle} boxSize={6} color={redColor} />
                      <Heading size="md">
                        {t("settings.periodClosing") || "Period Closing"}
                      </Heading>
                    </HStack>
                    <Tooltip
                      label={
                        <Box p={2}>
                          <Text fontSize="xs" fontWeight="bold" mb={1}>
                            {t("settings.periodClosingDesc")}
                          </Text>
                          <Text fontSize="xs" fontWeight="semibold" mb={1}>
                            {t("settings.periodClosingFeatures")}
                          </Text>
                          <Text fontSize="xs" color="green.200">
                            - {t("settings.periodClosingResetRev")}
                          </Text>
                          <Text fontSize="xs" color="red.200">
                            - {t("settings.periodClosingResetExp")}
                          </Text>
                          <Text fontSize="xs" color="blue.200">
                            - {t("settings.periodClosingRetained")}
                          </Text>
                          <Text fontSize="xs">
                            - {t("settings.periodClosingJournal")}
                          </Text>
                          <Text fontSize="xs">
                            - {t("settings.periodClosingLock")}
                          </Text>
                          <Text
                            fontSize="xs"
                            mt={2}
                            fontWeight="semibold"
                            color="orange.200"
                          >
                            {t("settings.permanentAction")}
                          </Text>
                        </Box>
                      }
                      placement="left"
                      hasArrow
                      bg="red.600"
                    >
                      <Box cursor="help">
                        <Icon
                          as={FiInfo}
                          boxSize={4}
                          color={mutedTextColor}
                          _hover={{ color: redColor }}
                        />
                      </Box>
                    </Tooltip>
                  </HStack>
                </CardHeader>
                <CardBody>
                  <VStack spacing={4} alignItems="start">
                    <Alert status="error" borderRadius="md" size="sm">
                      <AlertIcon />
                      <Box>
                        <AlertTitle fontSize="sm">
                          {t("settings.permanentAction")}
                        </AlertTitle>
                        <AlertDescription fontSize="xs">
                          {t(
                            "settings.periodClosingWarnings.permanentActionDesc",
                          )}
                        </AlertDescription>
                      </Box>
                    </Alert>

                    {lastClosingInfo?.has_previous_closing &&
                      lastClosingInfo?.last_closing_date && (
                        <Alert status="info" borderRadius="md" size="sm">
                          <AlertIcon />
                          <Box flex="1">
                            <AlertTitle fontSize="sm">
                              {t(
                                "settings.periodClosingLabels.lastClosingTitle",
                              )}
                            </AlertTitle>
                            <AlertDescription fontSize="xs">
                              {t("settings.periodClosingLabels.lastClosed")}:{" "}
                              {formatPeriodDate(
                                normalizeApiDate(
                                  lastClosingInfo.last_closing_date,
                                ),
                              )}
                            </AlertDescription>
                          </Box>
                          <Button
                            size="xs"
                            colorScheme="orange"
                            variant="solid"
                            leftIcon={<FiUnlock />}
                            onClick={() => {
                              setReopenStartDate(
                                normalizeApiDate(
                                  lastClosingInfo.last_period_start_date ||
                                    lastClosingInfo.last_closing_date,
                                ),
                              );
                              setReopenEndDate(
                                normalizeApiDate(
                                  lastClosingInfo.last_period_end_date ||
                                    lastClosingInfo.last_closing_date,
                                ),
                              );
                              setShowReopenDialog(true);
                            }}
                            ml={2}
                          >
                            Buka Kembali
                          </Button>
                        </Alert>
                      )}

                    {lastClosingInfo?.requires_reclose && (
                      <Alert status="warning" borderRadius="md" size="sm">
                        <AlertIcon />
                        <Box>
                          <AlertTitle fontSize="sm">
                            Periode Dibuka Kembali
                          </AlertTitle>
                          <AlertDescription fontSize="xs">
                            Periode yang terakhir dibuka kembali harus ditutup
                            lagi dengan rentang tanggal yang sama:{" "}
                            {formatClosingRange(
                              normalizeApiDate(
                                lastClosingInfo.reopened_period_start,
                              ),
                              normalizeApiDate(
                                lastClosingInfo.reopened_period_end,
                              ),
                            )}
                          </AlertDescription>
                        </Box>
                      </Alert>
                    )}

                    {lastClosingInfo?.future_closed_pending &&
                      !lastClosingInfo?.requires_reclose && (
                        <Alert status="warning" borderRadius="md" size="sm">
                          <AlertIcon />
                          <Box>
                            <AlertTitle fontSize="sm">
                              Periode Tertutup Belum Efektif
                            </AlertTitle>
                            <AlertDescription fontSize="xs">
                              Ada period closing dengan tanggal akhir di masa
                              depan. Buka kembali periode tersebut terlebih
                              dahulu sebelum melakukan closing baru.
                            </AlertDescription>
                          </Box>
                        </Alert>
                      )}

                    {lastClosingInfo &&
                      !lastClosingInfo.has_previous_closing && (
                        <Alert status="warning" borderRadius="md" size="sm">
                          <AlertIcon />
                          <Box>
                            <AlertTitle fontSize="sm">
                              {t(
                                "settings.periodClosingWarnings.firstTimeTitle",
                              )}
                            </AlertTitle>
                            <AlertDescription fontSize="xs">
                              {t(
                                "settings.periodClosingWarnings.firstTimeDesc",
                              )}
                            </AlertDescription>
                          </Box>
                        </Alert>
                      )}

                    <Alert
                      status="success"
                      borderRadius="md"
                      size="sm"
                      variant="left-accent"
                    >
                      <AlertIcon />
                      <Box>
                        <AlertTitle fontSize="sm">
                          {t("settings.periodClosingWarnings.autoUpdateTitle")}
                        </AlertTitle>
                        <AlertDescription fontSize="xs">
                          <Box
                            dangerouslySetInnerHTML={{
                              __html: t(
                                "settings.periodClosingWarnings.autoUpdateDesc",
                              ),
                            }}
                          />
                        </AlertDescription>
                      </Box>
                    </Alert>

                    <Text fontSize="sm" color={mutedTextColor}>
                      {t("settings.periodClosingLabels.description")}
                    </Text>

                    <FormControl>
                      <FormLabel fontSize="sm" fontWeight="semibold">
                        {t("common.filters.startDate") || "Start Date"}
                      </FormLabel>
                      <Input
                        type="date"
                        value={periodStartDate}
                        onChange={(e) => setPeriodStartDate(e.target.value)}
                        variant="filled"
                        {...fieldStyles}
                        bg={
                          lastClosingInfo?.requires_reclose ||
                          !!lastClosingInfo?.next_start_date
                            ? lockedFieldBg
                            : inputBg
                        }
                        borderColor={
                          lastClosingInfo?.requires_reclose ||
                          !!lastClosingInfo?.next_start_date
                            ? lockedFieldBorderColor
                            : inputBorderColor
                        }
                        _hover={{
                          bg:
                            lastClosingInfo?.requires_reclose ||
                            !!lastClosingInfo?.next_start_date
                              ? lockedFieldBg
                              : inputHoverBg,
                          borderColor:
                            lastClosingInfo?.requires_reclose ||
                            !!lastClosingInfo?.next_start_date
                              ? lockedFieldBorderColor
                              : inputHoverBorderColor,
                        }}
                        _focus={{
                          bg:
                            lastClosingInfo?.requires_reclose ||
                            !!lastClosingInfo?.next_start_date
                              ? lockedFieldBg
                              : inputFocusBg,
                          borderColor:
                            lastClosingInfo?.requires_reclose ||
                            !!lastClosingInfo?.next_start_date
                              ? lockedFieldBorderColor
                              : inputFocusBorderColor,
                        }}
                        isReadOnly={
                          lastClosingInfo?.requires_reclose === true ||
                          !!lastClosingInfo?.next_start_date
                        }
                        cursor={
                          lastClosingInfo?.requires_reclose ||
                          !!lastClosingInfo?.next_start_date
                            ? "not-allowed"
                            : "text"
                        }
                        placeholder="Select start date"
                      />
                      <FormHelperText
                        fontSize="xs"
                        color={
                          lastClosingInfo?.requires_reclose ||
                          !!lastClosingInfo?.next_start_date
                            ? lockedHelperColor
                            : mutedTextColor
                        }
                      >
                        {lastClosingInfo?.requires_reclose
                          ? "Tanggal mulai dikunci mengikuti periode yang sebelumnya dibuka kembali."
                          : lastClosingInfo?.next_start_date
                            ? `${t("settings.periodClosingHelper.autoStart")} (${lastClosingInfo.last_closing_date ? formatPeriodDate(normalizeApiDate(lastClosingInfo.last_closing_date)) : "N/A"})`
                            : t("settings.periodClosingHelper.manualStart")}
                      </FormHelperText>
                    </FormControl>

                    <FormControl>
                      <FormLabel fontSize="sm" fontWeight="semibold">
                        {t("common.filters.endDate") || "End Date"}
                      </FormLabel>
                      <Input
                        type="date"
                        value={periodEndDate}
                        onChange={(e) => setPeriodEndDate(e.target.value)}
                        variant="filled"
                        {...fieldStyles}
                        isReadOnly={lastClosingInfo?.requires_reclose === true}
                        max={
                          lastClosingInfo?.requires_reclose === true ||
                          !latestClosableDate
                            ? undefined
                            : latestClosableDate
                        }
                        cursor={
                          lastClosingInfo?.requires_reclose
                            ? "not-allowed"
                            : "text"
                        }
                      />
                      <FormHelperText fontSize="xs" color={mutedTextColor}>
                        {lastClosingInfo?.requires_reclose
                          ? "Tanggal akhir dikunci mengikuti periode yang sebelumnya dibuka kembali."
                          : latestClosableDate
                            ? `${t("settings.periodClosingLabels.endDateHelper")} Batas maksimal saat ini: ${latestClosableDate}.`
                            : t("settings.periodClosingLabels.endDateHelper")}
                      </FormHelperText>
                    </FormControl>

                    <Button
                      colorScheme="red"
                      variant="solid"
                      width="full"
                      onClick={handlePreviewPeriodClosing}
                      isLoading={loadingPeriodPreview}
                      loadingText={t(
                        "settings.periodClosingLabels.loadingPreview",
                      )}
                      isDisabled={!periodStartDate || !periodEndDate}
                    >
                      {t("settings.periodClosingLabels.previewButton")}
                    </Button>

                    {/* Closing History Section */}
                    <Divider borderColor={dividerColor} />
                    <HStack justify="space-between" width="full">
                      <HStack spacing={2}>
                        <Icon as={FiClock} color={mutedTextColor} />
                        <Text fontWeight="semibold" fontSize="sm">
                          Riwayat Tutup Buku
                        </Text>
                        {closingHistory.length > 0 ? (
                          <Badge colorScheme="blue" variant="subtle">
                            {closingHistory.length} periode
                          </Badge>
                        ) : null}
                      </HStack>
                      <HStack spacing={2}>
                        <Button
                          size="xs"
                          variant="outline"
                          onClick={() => setShowClosingHistoryModal(true)}
                          isDisabled={closingHistory.length === 0}
                        >
                          Lihat Riwayat
                        </Button>
                        <Button
                          size="xs"
                          variant="ghost"
                          onClick={fetchClosingHistory}
                          isLoading={loadingHistory}
                        >
                          Refresh
                        </Button>
                      </HStack>
                    </HStack>

                    {loadingHistory ? (
                      <HStack justify="center" py={3}>
                        <Spinner size="sm" />
                        <Text fontSize="xs" color={mutedTextColor}>
                          Loading...
                        </Text>
                      </HStack>
                    ) : closingHistory.length === 0 ? (
                      <Text
                        fontSize="xs"
                        color={mutedTextColor}
                        textAlign="center"
                        py={2}
                      >
                        Belum ada riwayat tutup buku
                      </Text>
                    ) : (
                      <VStack spacing={2} width="full" alignItems="stretch">
                        <Alert status="info" borderRadius="md">
                          <AlertIcon />
                          <Box>
                            <AlertTitle fontSize="sm">
                              Riwayat lengkap dipindah ke modal
                            </AlertTitle>
                            <AlertDescription fontSize="xs">
                              Halaman settings tetap ringkas. Gunakan tombol
                              {" "}
                              <strong>Lihat Riwayat</strong> untuk membuka
                              daftar semua periode yang sudah pernah ditutup.
                            </AlertDescription>
                          </Box>
                        </Alert>
                        {latestClosingHistory ? (
                          <>
                            <Text fontSize="xs" color={mutedTextColor}>
                              Ringkasan periode terbaru
                            </Text>
                            {renderClosingHistoryItem(latestClosingHistory, 2)}
                          </>
                        ) : null}
                      </VStack>
                    )}
                  </VStack>
                </CardBody>
              </Card>

              {/* Tax Account Configuration Card */}
              <Card
                bg={cardBg}
                border="1px"
                borderColor={cardBorderColor}
                boxShadow="md"
              >
                <CardHeader>
                  <HStack spacing={3} justify="space-between">
                    <HStack spacing={3}>
                      <Icon as={FiCreditCard} boxSize={6} color={orangeColor} />
                      <Heading size="md">
                        {t("settings.taxAccountConfig")}
                      </Heading>
                    </HStack>
                    {settings?.company_type && (
                      <Badge
                        colorScheme={
                          settings.company_type === "SERVICE"
                            ? "purple"
                            : settings.company_type === "TRADING"
                              ? "teal"
                              : "blue"
                        }
                        fontSize="xs"
                        px={3}
                        py={1}
                        borderRadius="full"
                        textTransform="uppercase"
                      >
                        {settings.company_type === "SERVICE"
                          ? t("settings.companyTypeBadges.service")
                          : settings.company_type === "TRADING"
                            ? t("settings.companyTypeBadges.trading")
                            : t("settings.companyTypeBadges.manufacturing")}
                      </Badge>
                    )}
                  </HStack>
                </CardHeader>
                <CardBody>
                  <VStack spacing={4} alignItems="start">
                    <Text fontSize="sm" color={mutedTextColor} mb={2}>
                      {t("settings.taxAccountConfigDesc")}
                    </Text>

                    {/* PPN Rate Display */}
                    <Box
                      width="full"
                      p={3}
                      bg={warningPanelBg}
                      borderRadius="md"
                      border="1px"
                      borderColor={warningPanelBorderColor}
                    >
                      <HStack justify="space-between">
                        <HStack spacing={2}>
                          <Icon
                            as={FiDollarSign}
                            color={orangeColor}
                            boxSize={4}
                          />
                          <Text
                            fontSize="sm"
                            fontWeight="semibold"
                            color={textColor}
                          >
                            {t("settings.taxRecommendations.ppnLabel")}
                          </Text>
                        </HStack>
                        <Badge
                          colorScheme="orange"
                          fontSize="sm"
                          px={2}
                          py={1}
                          borderRadius="md"
                        >
                          {settings?.default_tax_rate ?? 11}%
                        </Badge>
                      </HStack>
                    </Box>

                    {/* Company-type-specific recommendation */}
                    {settings?.company_type === "SERVICE" && (
                      <Alert
                        status="info"
                        variant="left-accent"
                        size="sm"
                        borderRadius="md"
                      >
                        <AlertIcon />
                        <AlertDescription fontSize="xs">
                          {t("settings.taxRecommendations.service")}
                        </AlertDescription>
                      </Alert>
                    )}
                    {settings?.company_type === "TRADING" && (
                      <Alert
                        status="warning"
                        variant="left-accent"
                        size="sm"
                        borderRadius="md"
                      >
                        <AlertIcon />
                        <AlertDescription fontSize="xs">
                          <Box
                            dangerouslySetInnerHTML={{
                              __html: t("settings.taxRecommendations.trading"),
                            }}
                          />
                        </AlertDescription>
                      </Alert>
                    )}
                    {settings?.company_type === "MANUFACTURING" && (
                      <Alert
                        status="warning"
                        variant="left-accent"
                        size="sm"
                        borderRadius="md"
                      >
                        <AlertIcon />
                        <AlertDescription fontSize="xs">
                          <Box
                            dangerouslySetInnerHTML={{
                              __html: t(
                                "settings.taxRecommendations.manufacturing",
                              ),
                            }}
                          />
                        </AlertDescription>
                      </Alert>
                    )}
                    {!settings?.company_type && (
                      <Alert
                        status="error"
                        variant="left-accent"
                        size="sm"
                        borderRadius="md"
                      >
                        <AlertIcon />
                        <AlertDescription fontSize="xs">
                          {t("settings.taxRecommendations.noType")}
                        </AlertDescription>
                      </Alert>
                    )}

                    {loadingTaxAccounts ? (
                      <HStack width="full" justify="center" py={4}>
                        <Spinner size="sm" color="blue.500" />
                        <Text fontSize="xs" color={mutedTextColor}>
                          {t("settings.taxAccountConfigLoading")}
                        </Text>
                      </HStack>
                    ) : taxAccountSettings ? (
                      <VStack spacing={3} width="full" alignItems="start">
                        {/* Withholding Tax Section */}
                        <HStack justify="space-between" width="full">
                          <VStack alignItems="start" spacing={1}>
                            <HStack spacing={2}>
                              <Text
                                fontSize="sm"
                                fontWeight="semibold"
                                color={textColor}
                              >
                                {t("settings.taxAccountSections.withholding")}
                              </Text>
                              {settings?.company_type === "SERVICE" && (
                                <Badge
                                  colorScheme="red"
                                  fontSize="2xs"
                                  variant="subtle"
                                >
                                  {t("settings.taxRecommendations.primary")}
                                </Badge>
                              )}
                            </HStack>
                            <VStack spacing={1} alignItems="start" mt={2}>
                              {taxAccountSettings.withholding_tax21_account ? (
                                <HStack spacing={1}>
                                  <Icon
                                    as={FiCheckCircle}
                                    color="green.500"
                                    boxSize={3}
                                  />
                                  <Text
                                    fontSize="xs"
                                    color="green.600"
                                    fontWeight="medium"
                                  >
                                    PPh 21:{" "}
                                    {
                                      taxAccountSettings
                                        .withholding_tax21_account.code
                                    }{" "}
                                    -{" "}
                                    {
                                      taxAccountSettings
                                        .withholding_tax21_account.name
                                    }
                                  </Text>
                                </HStack>
                              ) : (
                                <HStack spacing={1}>
                                  <Icon
                                    as={
                                      settings?.company_type === "SERVICE"
                                        ? FiAlertTriangle
                                        : FiInfo
                                    }
                                    color={
                                      settings?.company_type === "SERVICE"
                                        ? "orange.400"
                                        : mutedTextColor
                                    }
                                    boxSize={3}
                                  />
                                  <Text
                                    fontSize="xs"
                                    color={
                                      settings?.company_type === "SERVICE"
                                        ? orangeColor
                                        : mutedTextColor
                                    }
                                  >
                                    PPh 21:{" "}
                                    {t(
                                      "settings.taxAccountStatus.notConfigured",
                                    )}
                                  </Text>
                                </HStack>
                              )}
                              {taxAccountSettings.withholding_tax23_account ? (
                                <HStack spacing={1}>
                                  <Icon
                                    as={FiCheckCircle}
                                    color="green.500"
                                    boxSize={3}
                                  />
                                  <Text
                                    fontSize="xs"
                                    color="green.600"
                                    fontWeight="medium"
                                  >
                                    PPh 23:{" "}
                                    {
                                      taxAccountSettings
                                        .withholding_tax23_account.code
                                    }{" "}
                                    -{" "}
                                    {
                                      taxAccountSettings
                                        .withholding_tax23_account.name
                                    }
                                  </Text>
                                </HStack>
                              ) : (
                                <HStack spacing={1}>
                                  <Icon
                                    as={
                                      settings?.company_type === "SERVICE" ||
                                      settings?.company_type === "MANUFACTURING"
                                        ? FiAlertTriangle
                                        : FiInfo
                                    }
                                    color={
                                      settings?.company_type === "SERVICE" ||
                                      settings?.company_type === "MANUFACTURING"
                                        ? "orange.400"
                                        : mutedTextColor
                                    }
                                    boxSize={3}
                                  />
                                  <Text
                                    fontSize="xs"
                                    color={
                                      settings?.company_type === "SERVICE" ||
                                      settings?.company_type === "MANUFACTURING"
                                        ? orangeColor
                                        : mutedTextColor
                                    }
                                  >
                                    PPh 23:{" "}
                                    {t(
                                      "settings.taxAccountStatus.notConfigured",
                                    )}
                                  </Text>
                                </HStack>
                              )}
                              {taxAccountSettings.withholding_tax25_account ? (
                                <HStack spacing={1}>
                                  <Icon
                                    as={FiCheckCircle}
                                    color="green.500"
                                    boxSize={3}
                                  />
                                  <Text
                                    fontSize="xs"
                                    color="green.600"
                                    fontWeight="medium"
                                  >
                                    PPh 25:{" "}
                                    {
                                      taxAccountSettings
                                        .withholding_tax25_account.code
                                    }{" "}
                                    -{" "}
                                    {
                                      taxAccountSettings
                                        .withholding_tax25_account.name
                                    }
                                  </Text>
                                </HStack>
                              ) : (
                                <Text fontSize="xs" color={mutedTextColor}>
                                  PPh 25:{" "}
                                  {t("settings.taxAccountStatus.notConfigured")}{" "}
                                  ({t("settings.taxRecommendations.optional")})
                                </Text>
                              )}
                            </VStack>
                          </VStack>
                          <Icon
                            as={FiDollarSign}
                            color={orangeColor}
                            boxSize={4}
                          />
                        </HStack>

                        {/* Inventory & COGS Section - conditional based on company type */}
                        {settings?.company_type !== "SERVICE" && (
                          <>
                            <Divider borderColor={dividerColor} />
                            <HStack justify="space-between" width="full">
                              <VStack alignItems="start" spacing={1}>
                                <HStack spacing={2}>
                                  <Text
                                    fontSize="sm"
                                    fontWeight="semibold"
                                    color={textColor}
                                  >
                                    {t(
                                      "settings.taxAccountSections.inventoryCogs",
                                    )}
                                  </Text>
                                  {(settings?.company_type === "TRADING" ||
                                    settings?.company_type ===
                                      "MANUFACTURING") && (
                                    <Badge
                                      colorScheme="red"
                                      fontSize="2xs"
                                      variant="subtle"
                                    >
                                      {t(
                                        "settings.taxRecommendations.required",
                                      )}
                                    </Badge>
                                  )}
                                </HStack>
                                <VStack spacing={1} alignItems="start" mt={2}>
                                  {taxAccountSettings.inventory_account ? (
                                    <HStack spacing={1}>
                                      <Icon
                                        as={FiCheckCircle}
                                        color="green.500"
                                        boxSize={3}
                                      />
                                      <Text
                                        fontSize="xs"
                                        color="green.600"
                                        fontWeight="medium"
                                      >
                                        Inventory:{" "}
                                        {
                                          taxAccountSettings.inventory_account
                                            .code
                                        }{" "}
                                        -{" "}
                                        {
                                          taxAccountSettings.inventory_account
                                            .name
                                        }
                                      </Text>
                                    </HStack>
                                  ) : (
                                    <HStack spacing={1}>
                                      <Icon
                                        as={FiAlertTriangle}
                                        color="red.400"
                                        boxSize={3}
                                      />
                                      <Text
                                        fontSize="xs"
                                        color={redColor}
                                        fontWeight="medium"
                                      >
                                        Inventory:{" "}
                                        {t(
                                          "settings.taxAccountStatus.notConfigured",
                                        )}
                                      </Text>
                                    </HStack>
                                  )}
                                  {taxAccountSettings.cogs_account ? (
                                    <HStack spacing={1}>
                                      <Icon
                                        as={FiCheckCircle}
                                        color="green.500"
                                        boxSize={3}
                                      />
                                      <Text
                                        fontSize="xs"
                                        color="green.600"
                                        fontWeight="medium"
                                      >
                                        COGS:{" "}
                                        {taxAccountSettings.cogs_account.code} -{" "}
                                        {taxAccountSettings.cogs_account.name}
                                      </Text>
                                    </HStack>
                                  ) : (
                                    <HStack spacing={1}>
                                      <Icon
                                        as={FiAlertTriangle}
                                        color="red.400"
                                        boxSize={3}
                                      />
                                      <Text
                                        fontSize="xs"
                                        color={redColor}
                                        fontWeight="medium"
                                      >
                                        COGS:{" "}
                                        {t(
                                          "settings.taxAccountStatus.notConfigured",
                                        )}
                                      </Text>
                                    </HStack>
                                  )}
                                </VStack>
                              </VStack>
                              <Icon
                                as={FiPackage}
                                color={blueColor}
                                boxSize={4}
                              />
                            </HStack>
                          </>
                        )}
                      </VStack>
                    ) : (
                      <VStack spacing={3} width="full" alignItems="start">
                        <Text fontSize="xs" color={mutedTextColor}>
                          {t("settings.taxAccountStatus.noConfigFound")}
                        </Text>
                      </VStack>
                    )}

                    <Link href="/settings/tax-accounts">
                      <Button
                        colorScheme="orange"
                        variant="outline"
                        size="sm"
                        leftIcon={<FiSettings />}
                        width="full"
                        _hover={{ bg: subtleHoverBg }}
                      >
                        {t("settings.taxAccounts.title") ||
                          "Configure Tax Accounts"}
                      </Button>
                    </Link>
                  </VStack>
                </CardBody>
              </Card>
            </SimpleGrid>
          </VStack>
        </Box>

        {/* Period Closing Preview Modal */}
        <Modal
          isOpen={showPeriodClosingModal}
          onClose={() => setShowPeriodClosingModal(false)}
          size="xl"
        >
          <ModalOverlay />
          <ModalContent
            maxW="800px"
            bg={modalBg}
            borderWidth="1px"
            borderColor={cardBorderColor}
          >
            <ModalHeader bg={modalHeaderBg} color="white">
              {t("settings.periodClosing") || "Period Closing Preview"}
            </ModalHeader>
            <ModalCloseButton color="white" />
            <ModalBody py={6}>
              {periodClosingPreview && (
                <VStack spacing={4} alignItems="start">
                  {/* Period Info */}
                  <Box
                    width="full"
                    p={3}
                    bg={panelBg}
                    borderRadius="md"
                    borderWidth="1px"
                    borderColor={cardBorderColor}
                  >
                    <Text fontSize="sm" fontWeight="bold" mb={1}>
                      {t("settings.periodClosingLabels.period")}:{" "}
                      {formatClosingRange(
                        normalizeApiDate(periodClosingPreview.start_date),
                        normalizeApiDate(periodClosingPreview.end_date),
                      )}
                    </Text>
                    <Text fontSize="xs" color={mutedTextColor}>
                      {periodClosingPreview.period_days}{" "}
                      {t("settings.periodClosingLabels.days")} |{" "}
                      {periodClosingPreview.transaction_count}{" "}
                      {t("settings.periodClosingLabels.transactions")}
                    </Text>
                  </Box>

                  {/* Summary */}
                  <Box
                    width="full"
                    p={4}
                    bg={panelBg}
                    borderRadius="md"
                    borderWidth="1px"
                    borderColor={cardBorderColor}
                  >
                    <Text fontSize="lg" fontWeight="bold" mb={2}>
                      {t("settings.periodClosingLabels.financialSummary")}
                    </Text>
                    <SimpleGrid columns={3} spacing={4}>
                      <Box>
                        <Text fontSize="xs" color={mutedTextColor}>
                          {t("settings.periodClosingLabels.totalRevenue")}
                        </Text>
                        <Text
                          fontSize="lg"
                          fontWeight="bold"
                          color={greenColor}
                        >
                          Rp{" "}
                          {periodClosingPreview.total_revenue.toLocaleString(
                            "id-ID",
                          )}
                        </Text>
                      </Box>
                      <Box>
                        <Text fontSize="xs" color={mutedTextColor}>
                          {t("settings.periodClosingLabels.totalExpense")}
                        </Text>
                        <Text fontSize="lg" fontWeight="bold" color={redColor}>
                          Rp{" "}
                          {periodClosingPreview.total_expense.toLocaleString(
                            "id-ID",
                          )}
                        </Text>
                      </Box>
                      <Box>
                        <Text fontSize="xs" color={mutedTextColor}>
                          {t("settings.periodClosingLabels.netIncome")}
                        </Text>
                        <Text fontSize="lg" fontWeight="bold" color={blueColor}>
                          Rp{" "}
                          {periodClosingPreview.net_income.toLocaleString(
                            "id-ID",
                          )}
                        </Text>
                      </Box>
                    </SimpleGrid>
                  </Box>

                  {/* Validation Messages */}
                  {periodClosingInfoMessages.length > 0 && (
                      <Alert
                        status={
                          periodClosingPreview.can_close ? "info" : "error"
                        }
                        borderRadius="md"
                      >
                        <AlertIcon />
                        <Box>
                          <AlertTitle fontSize="sm">
                            {periodClosingPreview.can_close
                              ? t("settings.periodClosingLabels.validation")
                              : t(
                                  "settings.periodClosingLabels.validationIssues",
                                )}
                          </AlertTitle>
                          <AlertDescription fontSize="xs">
                            {periodClosingInfoMessages.map((msg, i: number) => (
                              <Text key={i}>{msg}</Text>
                            ))}
                          </AlertDescription>
                        </Box>
                      </Alert>
                    )}

                  {periodClosingPreview.blocking_messages &&
                    periodClosingPreview.blocking_messages.length > 0 && (
                      <Alert status="error" borderRadius="md">
                        <AlertIcon />
                        <Box>
                          <AlertTitle fontSize="sm">
                            Alasan Tombol Close Dinonaktifkan
                          </AlertTitle>
                          <AlertDescription fontSize="xs">
                            {periodClosingPreview.blocking_messages.map(
                              (msg: string, i: number) => (
                                <Text key={`blocking-${i}`}>{msg}</Text>
                              ),
                            )}
                          </AlertDescription>
                        </Box>
                      </Alert>
                    )}

                  {/* What Will Happen */}
                  {periodClosingPreview.can_close && (
                    <Alert status="warning" borderRadius="md">
                      <AlertIcon />
                      <Box>
                        <AlertTitle fontSize="sm">
                          {t("settings.periodClosingFeatures") ||
                            "What will happen:"}
                        </AlertTitle>
                        <AlertDescription fontSize="xs">
                          - {periodClosingPreview.revenue_accounts?.length || 0}{" "}
                          {t("settings.periodClosingLabels.featureRevenue")}
                          <br />-{" "}
                          {periodClosingPreview.expense_accounts?.length ||
                            0}{" "}
                          {t("settings.periodClosingLabels.featureExpense")}
                          <br />-{" "}
                          {t(
                            "settings.periodClosingLabels.featureRetained",
                          )} Rp{" "}
                          {periodClosingPreview.net_income.toLocaleString(
                            "id-ID",
                          )}
                          <br />-{" "}
                          {t("settings.periodClosingLabels.featureJournal")}
                          <br />-{" "}
                          {t("settings.periodClosingLabels.featureLocked")}
                        </AlertDescription>
                      </Box>
                    </Alert>
                  )}

                  {/* Closing Entries Preview */}
                  {periodClosingPreview.closing_entries &&
                    periodClosingPreview.closing_entries.length > 0 && (
                      <Box width="full">
                        <Text fontSize="sm" fontWeight="bold" mb={2}>
                          {t("settings.periodClosingLabels.closingEntries")}
                        </Text>
                        <VStack spacing={2} width="full" alignItems="start">
                          {periodClosingPreview.closing_entries.map(
                            (entry, i: number) => (
                              <Box
                                key={i}
                                width="full"
                                p={3}
                                bg={itemBg}
                                border="1px"
                                borderColor={cardBorderColor}
                                borderRadius="md"
                              >
                                <Text
                                  fontSize="xs"
                                  fontWeight="semibold"
                                  mb={1}
                                >
                                  {entry.description}
                                </Text>
                                <HStack spacing={2} fontSize="xs">
                                  <Text>Dr: {entry.debit_account}</Text>
                                  <Text>{"->"}</Text>
                                  <Text>Cr: {entry.credit_account}</Text>
                                  <Text fontWeight="bold">
                                    Rp {entry.amount.toLocaleString("id-ID")}
                                  </Text>
                                </HStack>
                              </Box>
                            ),
                          )}
                        </VStack>
                      </Box>
                    )}
                </VStack>
              )}
            </ModalBody>
            <ModalFooter
              bg={modalFooterBg}
              borderTopWidth="1px"
              borderColor={cardBorderColor}
            >
              <Button
                variant="ghost"
                mr={3}
                onClick={() => setShowPeriodClosingModal(false)}
                isDisabled={executingPeriodClosing}
              >
                {t("common.cancel") || "Cancel"}
              </Button>
              <Button
                colorScheme="red"
                onClick={handleExecutePeriodClosing}
                isLoading={executingPeriodClosing}
                loadingText={t("common.processing") || "Closing..."}
                isDisabled={
                  !periodClosingPreview?.can_close || executingPeriodClosing
                }
              >
                {t("settings.periodClosing") || "Execute Period Closing"}
              </Button>
            </ModalFooter>
          </ModalContent>
        </Modal>

        <Modal
          isOpen={showClosingHistoryModal}
          onClose={() => setShowClosingHistoryModal(false)}
          size="5xl"
          scrollBehavior="inside"
        >
          <ModalOverlay />
          <ModalContent
            bg={modalBg}
            borderWidth="1px"
            borderColor={cardBorderColor}
            maxH="88vh"
          >
            <ModalHeader bg={blueColor} color="white">
              Riwayat Tutup Buku
            </ModalHeader>
            <ModalCloseButton color="white" />
            <ModalBody py={6} overflow="hidden">
              <VStack spacing={4} align="stretch" h="full">
                <HStack justify="space-between" flexWrap="wrap">
                  <Text fontSize="sm" color={mutedTextColor}>
                    {closingHistory.length > 0
                      ? `${closingHistory.length} periode tercatat. Scroll di dalam modal untuk menelusuri histori lama.`
                      : "Belum ada riwayat tutup buku."}
                  </Text>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={fetchClosingHistory}
                    isLoading={loadingHistory}
                  >
                    Refresh
                  </Button>
                </HStack>

                {loadingHistory ? (
                  <HStack justify="center" py={6}>
                    <Spinner size="sm" />
                    <Text fontSize="sm" color={mutedTextColor}>
                      Loading history...
                    </Text>
                  </HStack>
                ) : closingHistory.length === 0 ? (
                  <Text fontSize="sm" color={mutedTextColor} textAlign="center" py={6}>
                    Belum ada riwayat tutup buku
                  </Text>
                ) : (
                  <Box
                    maxH={{ base: "52vh", md: "58vh", lg: "62vh" }}
                    overflowY="auto"
                    pr={2}
                    pb={1}
                    overscrollBehavior="contain"
                  >
                    <VStack spacing={3} align="stretch">
                      {closingHistory.map((item) => renderClosingHistoryItem(item))}
                    </VStack>
                  </Box>
                )}
              </VStack>
            </ModalBody>
            <ModalFooter bg={modalFooterBg} borderTopWidth="1px" borderColor={cardBorderColor}>
              <Button variant="ghost" onClick={() => setShowClosingHistoryModal(false)}>
                Tutup
              </Button>
            </ModalFooter>
          </ModalContent>
        </Modal>

        {/* Reopen Period Dialog */}
        <ReopenPeriodDialog
          isOpen={showReopenDialog}
          onClose={() => setShowReopenDialog(false)}
          startDate={reopenStartDate}
          endDate={reopenEndDate}
          onSuccess={() => {
            fetchLastClosingInfo();
            fetchClosingHistory();
          }}
        />
      </SimpleLayout>
    </ProtectedModule>
  );
};

export default SettingsPage;
