import React, { useState, useEffect, useCallback } from "react";
import {
  Alert,
  AlertIcon,
  Box,
  Button,
  HStack,
  Select,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  Badge,
  Text,
  Input,
  Spinner,
  useToast,
  VStack,
  Heading,
  Menu,
  MenuButton,
  MenuList,
  MenuItem,
  IconButton,
  Tooltip,
  useColorModeValue,
} from "@chakra-ui/react";
import { useTranslation } from "@/hooks/useTranslation";
import { useModulePermissions } from "@/hooks/usePermissions";
import {
  ssotJournalService,
  SSOTJournalEntry,
} from "@/services/ssotJournalService";
import {
  FiRefreshCw,
  FiPlus,
  FiMoreVertical,
  FiEdit2,
  FiTrash2,
  FiCheckCircle,
  FiEye,
  FiChevronLeft,
  FiChevronRight,
  FiRotateCcw,
} from "react-icons/fi";
import ManualEntryForm from "./ManualEntryForm";
import JournalDetailModal from "./JournalDetailModal";
import ExportButton from "@/components/common/ExportButton";
import { formatAccountingDate } from "@/utils/accountingDate";
import { useAuth } from "@/contexts/AuthContext";
import {
  exportGeneralJournalToPDF,
  exportGeneralJournalToCSV,
} from "@/utils/journalExport";

type JournalEntryMode = "MANUAL" | "ADJUSTMENT";

export default function GeneralJournalView() {
  const { t } = useTranslation();
  const { canCreate, canEdit, canDelete, canExport, canApprove } =
    useModulePermissions("journal");
  const { user } = useAuth();
  const toast = useToast();
  const textColor = useColorModeValue("gray.800", "gray.100");
  const mutedTextColor = useColorModeValue("gray.500", "gray.400");
  const subtleTextColor = useColorModeValue("gray.400", "gray.500");
  const inputBg = useColorModeValue("white", "whiteAlpha.50");
  const inputBorderColor = useColorModeValue("gray.300", "whiteAlpha.300");
  const inputHoverBorderColor = useColorModeValue("gray.400", "whiteAlpha.400");
  const inputFocusBorderColor = useColorModeValue("blue.500", "blue.300");
  const tableBg = useColorModeValue("white", "gray.800");
  const tableHeaderBg = useColorModeValue("gray.50", "whiteAlpha.100");
  const tableHoverBg = useColorModeValue("gray.50", "whiteAlpha.100");
  const tableSeparatorBg = useColorModeValue("gray.100", "whiteAlpha.50");
  const borderColor = useColorModeValue("gray.200", "whiteAlpha.200");
  const menuBg = useColorModeValue("white", "gray.800");
  const menuBorderColor = useColorModeValue("gray.200", "whiteAlpha.200");
  const menuHoverBg = useColorModeValue("gray.50", "whiteAlpha.100");
  const fieldStyles = {
    bg: inputBg,
    color: textColor,
    borderColor: inputBorderColor,
    _hover: { borderColor: inputHoverBorderColor },
    _focusVisible: {
      borderColor: inputFocusBorderColor,
      boxShadow: `0 0 0 1px ${inputFocusBorderColor}`,
    },
    _placeholder: { color: mutedTextColor },
  };

  const [entries, setEntries] = useState<SSOTJournalEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [entryMode, setEntryMode] = useState<JournalEntryMode>("MANUAL");
  const [selectedEntry, setSelectedEntry] = useState<
    SSOTJournalEntry | undefined
  >(undefined);
  const [adjustmentBaseEntry, setAdjustmentBaseEntry] = useState<
    SSOTJournalEntry | undefined
  >(undefined);

  // View Modal State
  const [viewEntry, setViewEntry] = useState<SSOTJournalEntry | undefined>(
    undefined,
  );
  const [isViewModalOpen, setIsViewModalOpen] = useState(false);

  // Filters
  const [startDate, setStartDate] = useState(
    new Date(new Date().getFullYear(), 0, 1).toISOString().split("T")[0],
  );
  const [endDate, setEndDate] = useState(
    new Date().toISOString().split("T")[0],
  );
  const [searchTerm, setSearchTerm] = useState("");
  const [exporting, setExporting] = useState(false);
  const [currentPage, setCurrentPage] = useState(1);
  const [totalEntries, setTotalEntries] = useState(0);
  const [totalPages, setTotalPages] = useState(0);
  const [statusFilter, setStatusFilter] = useState("ALL");
  const [sourceTypeFilter, setSourceTypeFilter] = useState("ALL");
  const [adjustmentRequirementFilter, setAdjustmentRequirementFilter] =
    useState("ALL");
  const [pendingAdjustmentCount, setPendingAdjustmentCount] = useState(0);
  const [pendingAdjustmentPreview, setPendingAdjustmentPreview] = useState<
    SSOTJournalEntry[]
  >([]);
  const pageSize = 100;
  const normalizedRole = String(user?.role || "").toLowerCase();
  const isAdmin = normalizedRole === "admin";
  const isFinanceStaff =
    normalizedRole === "finance_staff" || normalizedRole === "accounting";
  const isFinanceManagerApprover =
    normalizedRole === "finance_manager" || normalizedRole === "finance";
  const isDirectorApprover = normalizedRole === "director";
  const canManageAdjustmentDraft = isFinanceStaff || isFinanceManagerApprover;
  const canCreateAdjustment = canCreate && canManageAdjustmentDraft;
  const canCreateManualEntry = canCreate && !isFinanceStaff && !isAdmin;
  const canPostDraftEntry =
    canApprove && (isFinanceManagerApprover || isDirectorApprover);
  const canManageGeneralDraft =
    canEdit && !isFinanceManagerApprover && !isDirectorApprover && !isAdmin;
  const canDeleteGeneralDraft =
    canDelete && !isFinanceManagerApprover && !isDirectorApprover && !isAdmin;
  const canReverseJournalEntry = canApprove && isFinanceManagerApprover;
  const canShowCreateAdjustmentAction = useCallback(
    (entry: SSOTJournalEntry) =>
      entry.status === "POSTED" &&
      canCreateAdjustment &&
      !entry.reversed_by &&
      entry.source_type !== "REVERSAL" &&
      entry.source_type !== "ADJUSTMENT" &&
      entry.adjustment_required === true &&
      entry.adjustment_completed !== true,
    [canCreateAdjustment],
  );

  const isDirectorApprovalPending = useCallback((entry: SSOTJournalEntry) => {
    return (
      entry.status === "DRAFT" &&
      !!entry.require_director_approval &&
      String(entry.approval_stage || "").toUpperCase() === "DIRECTOR"
    );
  }, []);

  const getDraftApprovalLabel = useCallback(
    (entry: SSOTJournalEntry) => {
      if (entry.status !== "DRAFT") {
        return "";
      }
      if (isDirectorApprovalPending(entry)) {
        return "Menunggu approval director";
      }
      return "Menunggu approval finance manager";
    },
    [isDirectorApprovalPending],
  );

  const getExportEntries = useCallback(async () => {
    return ssotJournalService.getAllJournalEntries({
      start_date: startDate,
      end_date: endDate,
      status: statusFilter,
      source_type: sourceTypeFilter,
      adjustment_requirement: adjustmentRequirementFilter,
      search: searchTerm,
    });
  }, [
    adjustmentRequirementFilter,
    endDate,
    searchTerm,
    sourceTypeFilter,
    startDate,
    statusFilter,
  ]);

  const handleExportPDF = async () => {
    try {
      setExporting(true);
      const exportEntries = await getExportEntries();
      exportGeneralJournalToPDF(exportEntries, startDate, endDate);
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
    try {
      setExporting(true);
      const exportEntries = await getExportEntries();
      exportGeneralJournalToCSV(exportEntries, startDate, endDate);
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

  const fetchEntries = useCallback(async () => {
    setLoading(true);
    try {
      const response = await ssotJournalService.getJournalEntries({
        start_date: startDate,
        end_date: endDate,
        status: statusFilter,
        source_type: sourceTypeFilter,
        adjustment_requirement: adjustmentRequirementFilter,
        search: searchTerm,
        page: currentPage,
        limit: pageSize,
      });
      setEntries(response.data || []);
      setTotalEntries(response.total || 0);
      setTotalPages(response.totalPages || 0);
    } catch (error) {
      console.error("Failed to fetch journal entries", error);
      toast({
        title: t("common.error"),
        description: "Failed to fetch journal entries",
        status: "error",
        duration: 3000,
      });
    } finally {
      setLoading(false);
    }
  }, [
    adjustmentRequirementFilter,
    currentPage,
    endDate,
    pageSize,
    searchTerm,
    sourceTypeFilter,
    startDate,
    statusFilter,
    t,
    toast,
  ]);

  useEffect(() => {
    fetchEntries();
  }, [fetchEntries]);

  useEffect(() => {
    const loadPendingAdjustmentQueue = async () => {
      if (
        !isDirectorApprover &&
        !isFinanceManagerApprover &&
        !canManageAdjustmentDraft
      ) {
        setPendingAdjustmentCount(0);
        setPendingAdjustmentPreview([]);
        return;
      }

      try {
        const queueEntries = await ssotJournalService.getAllJournalEntries({
          status: "DRAFT",
          source_type: "ADJUSTMENT",
        });
        const filteredQueue = queueEntries.filter((entry) => {
          const directorStage = isDirectorApprovalPending(entry);
          if (isDirectorApprover) {
            return directorStage;
          }
          if (isFinanceManagerApprover) {
            return !directorStage;
          }
          return true;
        });
        setPendingAdjustmentCount(filteredQueue.length);
        setPendingAdjustmentPreview(filteredQueue.slice(0, 5));
      } catch (error) {
        console.error("Failed to load pending adjustment queue", error);
        setPendingAdjustmentCount(0);
        setPendingAdjustmentPreview([]);
      }
    };

    loadPendingAdjustmentQueue();
  }, [
    canManageAdjustmentDraft,
    entries,
    isDirectorApprovalPending,
    isDirectorApprover,
    isFinanceManagerApprover,
  ]);

  const focusPendingAdjustments = useCallback(
    (reference?: string) => {
      setCurrentPage(1);
      setStartDate("");
      setEndDate("");
      setStatusFilter("DRAFT");
      setSourceTypeFilter("ADJUSTMENT");
      setAdjustmentRequirementFilter("ALL");
      setSearchTerm(reference || "");
    },
    [
      setCurrentPage,
      setEndDate,
      setSearchTerm,
      setSourceTypeFilter,
      setStartDate,
      setStatusFilter,
    ],
  );

  useEffect(() => {
    if (totalPages > 0 && currentPage > totalPages) {
      setCurrentPage(totalPages);
    }
  }, [currentPage, totalPages]);

  const handlePost = async (
    entry: SSOTJournalEntry,
    options: { require_director_approval?: boolean } = {},
  ) => {
    try {
      const result = await ssotJournalService.postJournalEntry(
        entry.id,
        options,
      );
      const sentToDirector =
        result.status === "DRAFT" && isDirectorApprovalPending(result);
      toast({
        title: t("common.success"),
        description: sentToDirector
          ? "Draft jurnal sudah disetujui finance manager dan dikirim ke director."
          : t("journal.post_success"),
        status: "success",
        duration: 3000,
      });
      if (sentToDirector) {
        focusPendingAdjustments(result.reference || "");
      } else {
        fetchEntries();
      }
    } catch (error: unknown) {
      const message =
        error instanceof Error ? error.message : t("journal.post_error");
      toast({
        title: t("common.error"),
        description: message,
        status: "error",
        duration: 3000,
      });
    }
  };

  const handleDelete = async (id: number) => {
    if (!window.confirm(t("journal.delete_confirm"))) return;
    try {
      await ssotJournalService.deleteJournalEntry(id);
      toast({
        title: t("common.success"),
        description: t("journal.delete_success"),
        status: "success",
        duration: 3000,
      });
      fetchEntries();
    } catch (error: unknown) {
      const message =
        error instanceof Error ? error.message : t("journal.delete_error");
      toast({
        title: t("common.error"),
        description: message,
        status: "error",
        duration: 3000,
      });
    }
  };

  const handleReverse = async (entry: SSOTJournalEntry) => {
    const reason = window.prompt(
      "Masukkan alasan reversal jurnal",
      "Reversed via general journal view",
    );
    if (reason === null) return;

    try {
      await ssotJournalService.reverseJournalEntry(
        entry.id,
        reason.trim() || "Reversed via general journal view",
      );
      toast({
        title: t("common.success"),
        description: "Jurnal berhasil direverse",
        status: "success",
        duration: 3000,
      });
      fetchEntries();
    } catch (error: unknown) {
      const message =
        error instanceof Error ? error.message : "Gagal mereverse jurnal";
      toast({
        title: t("common.error"),
        description: message,
        status: "error",
        duration: 3000,
      });
    }
  };

  const handleEdit = (entry: SSOTJournalEntry) => {
    setEntryMode(entry.source_type === "ADJUSTMENT" ? "ADJUSTMENT" : "MANUAL");
    setAdjustmentBaseEntry(undefined);
    setSelectedEntry(entry);
    setShowCreateForm(true);
  };

  const handleCreateAdjustment = (entry: SSOTJournalEntry) => {
    setEntryMode("ADJUSTMENT");
    setSelectedEntry(undefined);
    setAdjustmentBaseEntry(entry);
    setShowCreateForm(true);
  };

  const handleView = async (entry: SSOTJournalEntry) => {
    setViewEntry(entry);
    setIsViewModalOpen(true);

    try {
      const fullEntry = await ssotJournalService.getJournalEntry(entry.id);
      setViewEntry(fullEntry);
    } catch (error) {
      console.error("Failed to fetch journal detail", error);
    }
  };

  const formatCurrency = (val: number) => {
    return new Intl.NumberFormat("id-ID", {
      style: "currency",
      currency: "IDR",
      minimumFractionDigits: 0,
    }).format(val);
  };

  const getSourceTypeColor = (sourceType: string) => {
    switch (sourceType?.toUpperCase()) {
      case "SALE":
        return "blue";
      case "PURCHASE":
        return "orange";
      case "PAYMENT":
        return "purple";
      case "TRANSFER":
        return "teal";
      case "CASH_BANK":
        return "cyan";
      case "OPENING":
        return "green";
      case "ADJUSTMENT":
        return "pink";
      case "MANUAL":
        return "gray";
      default:
        return "gray";
    }
  };

  if (showCreateForm) {
    // Fix for undefined reference type mismatch
    const initialWithSafeRef = selectedEntry
      ? {
          ...selectedEntry,
          reference: selectedEntry.reference || "",
        }
      : undefined;

    return (
      <Box>
        <HStack mb={4} justify="space-between">
          <Heading size="md" color={textColor}>
            {selectedEntry
              ? entryMode === "ADJUSTMENT"
                ? t("journal.editAdjustmentEntry")
                : t("journal.editJournalEntry")
              : entryMode === "ADJUSTMENT"
                ? t("journal.createAdjustment")
                : t("journal.createManualEntry")}
          </Heading>
          <Button
            variant="ghost"
            onClick={() => {
              setShowCreateForm(false);
              setSelectedEntry(undefined);
              setAdjustmentBaseEntry(undefined);
              setEntryMode("MANUAL");
            }}
          >
            {t("journal.backToList")}
          </Button>
        </HStack>
        <ManualEntryForm
          initialData={initialWithSafeRef}
          baseEntry={adjustmentBaseEntry}
          entryType={entryMode}
          onSuccess={(savedEntry) => {
            setShowCreateForm(false);
            setSelectedEntry(undefined);
            setAdjustmentBaseEntry(undefined);
            setEntryMode("MANUAL");
            if (
              savedEntry?.source_type === "ADJUSTMENT" &&
              savedEntry.status === "DRAFT"
            ) {
              focusPendingAdjustments(savedEntry.reference || "");
              return;
            }
            fetchEntries();
          }}
          onCancel={() => {
            setShowCreateForm(false);
            setSelectedEntry(undefined);
            setAdjustmentBaseEntry(undefined);
            setEntryMode("MANUAL");
          }}
        />
      </Box>
    );
  }

  return (
    <Box>
      <JournalDetailModal
        isOpen={isViewModalOpen}
        onClose={() => setIsViewModalOpen(false)}
        entry={viewEntry}
      />

      <VStack spacing={4} align="stretch">
        {pendingAdjustmentCount > 0 &&
          (isDirectorApprover ||
            isFinanceManagerApprover ||
            canManageAdjustmentDraft) && (
            <Alert
              status={
                isDirectorApprover || isFinanceManagerApprover
                  ? "warning"
                  : "info"
              }
              borderRadius="md"
              variant="left-accent"
            >
              <AlertIcon />
              <Box flex="1">
                <Text fontWeight="semibold">
                  {isDirectorApprover
                    ? `${pendingAdjustmentCount} jurnal adjustment menunggu approval director`
                    : isFinanceManagerApprover
                      ? `${pendingAdjustmentCount} jurnal adjustment menunggu approval finance manager`
                      : `${pendingAdjustmentCount} jurnal adjustment masih menunggu approval finance manager`}
                </Text>
                <Text fontSize="sm" mt={1}>
                  Queue approval memakai tanggal pembuatan draft, jadi jurnal
                  yang dibackdate ke tanggal jurnal asal tetap akan muncul di
                  sini sesuai tahap approval-nya.
                </Text>
                {pendingAdjustmentPreview.length > 0 && (
                  <Text fontSize="sm" mt={2} color={mutedTextColor}>
                    {pendingAdjustmentPreview
                      .map((entry) => {
                        const label =
                          entry.entry_number ||
                          entry.reference ||
                          `Draft #${entry.id}`;
                        return `${label} (${formatAccountingDate(
                          entry.entry_date,
                        )})`;
                      })
                      .join(", ")}
                  </Text>
                )}
              </Box>
              <HStack spacing={2} alignSelf="flex-start">
                <Button
                  size="sm"
                  colorScheme={
                    isDirectorApprover || isFinanceManagerApprover
                      ? "orange"
                      : "blue"
                  }
                  onClick={() => focusPendingAdjustments()}
                >
                  {isDirectorApprover
                    ? "Lihat Approval Director"
                    : isFinanceManagerApprover
                      ? "Lihat Approval Adjustment"
                      : "Lihat Draft Adjustment"}
                </Button>
                {(statusFilter !== "ALL" ||
                  sourceTypeFilter !== "ALL" ||
                  adjustmentRequirementFilter !== "ALL" ||
                  searchTerm !== "" ||
                  startDate !==
                    new Date(new Date().getFullYear(), 0, 1)
                      .toISOString()
                      .split("T")[0] ||
                  endDate !== new Date().toISOString().split("T")[0]) && (
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => {
                      setCurrentPage(1);
                      setStartDate(
                        new Date(new Date().getFullYear(), 0, 1)
                          .toISOString()
                          .split("T")[0],
                      );
                      setEndDate(new Date().toISOString().split("T")[0]);
                      setStatusFilter("ALL");
                      setSourceTypeFilter("ALL");
                      setAdjustmentRequirementFilter("ALL");
                      setSearchTerm("");
                    }}
                  >
                    Reset Filter
                  </Button>
                )}
              </HStack>
            </Alert>
          )}

        <Alert status="info" borderRadius="md" variant="left-accent">
          <AlertIcon />
          <Box>
            <Text fontWeight="semibold">
              Jurnal adjustment hanya dibuka untuk 5 kategori transaksi client.
            </Text>
            <Text fontSize="sm">
              Gunakan filter Adjustment untuk melihat jurnal yang butuh
              adjustment, sudah dibuat adjustment, atau tidak masuk mapping.
              Tombol adjustment otomatis disembunyikan jika jurnal di luar
              kategori ini atau sudah punya adjustment.
            </Text>
          </Box>
        </Alert>

        {canCreateAdjustment && (
          <Alert status="info" borderRadius="md">
            <AlertIcon />
            {t("journal.adjustmentActionInfo")}
          </Alert>
        )}

        {canCreate && !canApprove && !canCreateAdjustment && (
          <Alert status="info" borderRadius="md">
            <AlertIcon />
            Jurnal manual yang Anda buat akan disimpan sebagai draft dan perlu
            verifikasi finance sebelum diposting.
          </Alert>
        )}

        <HStack justify="space-between" wrap="wrap" spacing={4}>
          <HStack spacing={4}>
            <Box>
              <Text fontSize="xs" color={mutedTextColor} mb={1}>
                {t("journal.filters.startDate")}
              </Text>
              <Input
                type="date"
                value={startDate}
                onChange={(e) => {
                  setCurrentPage(1);
                  setStartDate(e.target.value);
                }}
                size="sm"
                width="150px"
                {...fieldStyles}
              />
            </Box>
            <Box>
              <Text fontSize="xs" color={mutedTextColor} mb={1}>
                {t("journal.filters.endDate")}
              </Text>
              <Input
                type="date"
                value={endDate}
                onChange={(e) => {
                  setCurrentPage(1);
                  setEndDate(e.target.value);
                }}
                size="sm"
                width="150px"
                {...fieldStyles}
              />
            </Box>
            <Box>
              <Text fontSize="xs" color={mutedTextColor} mb={1}>
                {t("common.search")}
              </Text>
              <Input
                value={searchTerm}
                onChange={(e) => {
                  setCurrentPage(1);
                  setSearchTerm(e.target.value);
                }}
                size="sm"
                width="220px"
                placeholder={t("journal.table.ref")}
                {...fieldStyles}
              />
            </Box>
            <Box>
              <Text fontSize="xs" color={mutedTextColor} mb={1}>
                Status
              </Text>
              <Select
                value={statusFilter}
                onChange={(e) => {
                  setCurrentPage(1);
                  setStatusFilter(e.target.value);
                }}
                size="sm"
                width="140px"
                {...fieldStyles}
              >
                <option value="ALL">Semua</option>
                <option value="DRAFT">Draft</option>
                <option value="POSTED">Posted</option>
                <option value="REVERSED">Reversed</option>
              </Select>
            </Box>
            <Box>
              <Text fontSize="xs" color={mutedTextColor} mb={1}>
                Source
              </Text>
              <Select
                value={sourceTypeFilter}
                onChange={(e) => {
                  setCurrentPage(1);
                  setSourceTypeFilter(e.target.value);
                }}
                size="sm"
                width="170px"
                {...fieldStyles}
              >
                <option value="ALL">Semua</option>
                <option value="ADJUSTMENT">Adjustment</option>
                <option value="MANUAL">Manual</option>
                <option value="SALE">Sale</option>
                <option value="PURCHASE">Purchase</option>
                <option value="PAYMENT">Payment</option>
                <option value="REVERSAL">Reversal</option>
              </Select>
            </Box>
            <Box>
              <Text fontSize="xs" color={mutedTextColor} mb={1}>
                Adjustment
              </Text>
              <Select
                value={adjustmentRequirementFilter}
                onChange={(e) => {
                  setCurrentPage(1);
                  setAdjustmentRequirementFilter(e.target.value);
                }}
                size="sm"
                width="210px"
                {...fieldStyles}
              >
                <option value="ALL">Semua</option>
                <option value="NEEDS_ADJUSTMENT">Butuh Adjustment</option>
                <option value="ADJUSTED">Sudah Adjustment</option>
                <option value="NOT_REQUIRED">Tidak Perlu Adjustment</option>
              </Select>
            </Box>
            <Box alignSelf="flex-end">
              <Button
                leftIcon={<FiRefreshCw />}
                size="sm"
                onClick={fetchEntries}
                isLoading={loading}
              >
                {t("journal.filters.apply")}
              </Button>
            </Box>
          </HStack>

          <HStack spacing={2}>
            {canExport && (
              <ExportButton
                onExportPDF={handleExportPDF}
                onExportCSV={handleExportCSV}
                isLoading={exporting}
                disabled={entries.length === 0}
                showExcel={false}
                showPDF={true}
                showCSV={true}
                size="sm"
              />
            )}
            {canCreateManualEntry && (
              <Button
                colorScheme="blue"
                leftIcon={<FiPlus />}
                onClick={() => {
                  setEntryMode("MANUAL");
                  setSelectedEntry(undefined);
                  setShowCreateForm(true);
                }}
                size="sm"
              >
                {t("journal.createManualEntry")}
              </Button>
            )}
          </HStack>
        </HStack>

        <Box
          overflowX="auto"
          borderWidth="1px"
          borderRadius="lg"
          bg={tableBg}
          borderColor={borderColor}
        >
          <Table size="sm" variant="simple" color={textColor}>
            <Thead bg={tableHeaderBg}>
              <Tr>
                <Th color={mutedTextColor}>{t("journal.table.date")}</Th>
                <Th color={mutedTextColor}>{t("journal.table.number")}</Th>
                <Th color={mutedTextColor}>{t("journal.table.account")}</Th>
                <Th color={mutedTextColor}>{t("journal.table.description")}</Th>
                <Th color={mutedTextColor}>{t("journal.table.ref")}</Th>
                <Th isNumeric color={mutedTextColor}>
                  {t("journal.table.debit")}
                </Th>
                <Th isNumeric color={mutedTextColor}>
                  {t("journal.table.credit")}
                </Th>
                <Th color={mutedTextColor}>{t("journal.table.status")}</Th>
                <Th color={mutedTextColor}>{t("journal.table.action")}</Th>
              </Tr>
            </Thead>
            <Tbody>
              {loading ? (
                <Tr>
                  <Td colSpan={9} textAlign="center" py={10}>
                    <Spinner />
                  </Td>
                </Tr>
              ) : entries.length === 0 ? (
                <Tr>
                  <Td colSpan={9} textAlign="center" py={10}>
                    <Text color={mutedTextColor}>
                      {t("journal.noJournalEntries")}
                    </Text>
                  </Td>
                </Tr>
              ) : (
                entries.map((entry) => (
                  <React.Fragment key={entry.id}>
                    {/* Entry Header / First Line */}
                    {(entry.lines || []).map((line, index) => (
                      <Tr
                        key={`${entry.id}-${line.id}`}
                        _hover={{ bg: tableHoverBg }}
                      >
                        {/* Date and Number only on first line */}
                        <Td
                          borderBottomWidth={
                            index === (entry.lines?.length || 0) - 1
                              ? "1px"
                              : "0px"
                          }
                        >
                          {index === 0 && (
                            <Text fontWeight="medium">
                              {formatAccountingDate(entry.entry_date)}
                            </Text>
                          )}
                        </Td>
                        <Td
                          borderBottomWidth={
                            index === (entry.lines?.length || 0) - 1
                              ? "1px"
                              : "0px"
                          }
                        >
                          {index === 0 && (
                            <VStack align="start" spacing={0}>
                              <Text fontWeight="medium" fontSize="sm">
                                {entry.entry_number}
                              </Text>
                              <Badge
                                size="sm"
                                colorScheme={getSourceTypeColor(
                                  entry.source_type || "MANUAL",
                                )}
                                fontSize="xx-small"
                              >
                                {entry.source_type || "MANUAL"}
                              </Badge>
                              {entry.source_type === "ADJUSTMENT" &&
                                (entry.adjustment_category_label ||
                                  entry.adjustment_category_code) && (
                                  <Text
                                    fontSize="xs"
                                    color={mutedTextColor}
                                    noOfLines={1}
                                    title={
                                      entry.adjustment_category_label ||
                                      entry.adjustment_category_code
                                    }
                                  >
                                    {entry.adjustment_category_label ||
                                      entry.adjustment_category_code}
                                  </Text>
                                )}
                              {entry.adjustment_required && (
                                <>
                                  <Badge
                                    mt={1}
                                    colorScheme={
                                      entry.adjustment_completed
                                        ? "green"
                                        : "orange"
                                    }
                                    fontSize="xx-small"
                                  >
                                    {entry.adjustment_completed
                                      ? "Sudah Adjustment"
                                      : "Butuh Adjustment"}
                                  </Badge>
                                  <Text
                                    fontSize="xs"
                                    color={mutedTextColor}
                                    noOfLines={1}
                                    title={
                                      entry.adjustment_requirement_reason ||
                                      entry.adjustment_requirement_label
                                    }
                                  >
                                    {entry.adjustment_requirement_label}
                                  </Text>
                                </>
                              )}
                            </VStack>
                          )}
                        </Td>

                        {/* Account Info */}
                        <Td
                          borderBottomWidth={
                            index === (entry.lines?.length || 0) - 1
                              ? "1px"
                              : "0px"
                          }
                        >
                          {line.account ? (
                            <VStack align="start" spacing={0}>
                              <Text
                                fontSize="sm"
                                fontWeight={
                                  line.credit_amount > 0 ? "normal" : "medium"
                                }
                                pl={line.credit_amount > 0 ? 4 : 0}
                              >
                                {line.account.code} - {line.account.name}
                              </Text>
                            </VStack>
                          ) : (
                            <Text fontSize="sm" color={subtleTextColor}>
                              {t("journal.unknownAccount")}
                            </Text>
                          )}
                        </Td>

                        {/* Line Description + Entry Description (tooltip) */}
                        <Td
                          borderBottomWidth={
                            index === (entry.lines?.length || 0) - 1
                              ? "1px"
                              : "0px"
                          }
                        >
                          <VStack align="start" spacing={0}>
                            <Text
                              fontSize="sm"
                              noOfLines={1}
                              title={line.description}
                            >
                              {line.description}
                            </Text>
                            {index === 0 &&
                              entry.description !== line.description && (
                                <Text
                                  fontSize="xs"
                                  color={mutedTextColor}
                                  noOfLines={1}
                                  title={entry.description}
                                >
                                  {entry.description}
                                </Text>
                              )}
                          </VStack>
                        </Td>

                        <Td
                          borderBottomWidth={
                            index === (entry.lines?.length || 0) - 1
                              ? "1px"
                              : "0px"
                          }
                        >
                          {index === 0 ? entry.reference || "-" : ""}
                        </Td>

                        <Td
                          isNumeric
                          borderBottomWidth={
                            index === (entry.lines?.length || 0) - 1
                              ? "1px"
                              : "0px"
                          }
                        >
                          {line.debit_amount > 0
                            ? formatCurrency(line.debit_amount)
                            : "-"}
                        </Td>
                        <Td
                          isNumeric
                          borderBottomWidth={
                            index === (entry.lines?.length || 0) - 1
                              ? "1px"
                              : "0px"
                          }
                        >
                          {line.credit_amount > 0
                            ? formatCurrency(line.credit_amount)
                            : "-"}
                        </Td>

                        <Td
                          borderBottomWidth={
                            index === (entry.lines?.length || 0) - 1
                              ? "1px"
                              : "0px"
                          }
                        >
                          {index === 0 && (
                            <VStack align="start" spacing={1}>
                              <Badge
                                colorScheme={
                                  entry.status === "POSTED"
                                    ? "green"
                                    : entry.status === "DRAFT"
                                      ? "yellow"
                                      : "red"
                                }
                              >
                                {entry.status}
                              </Badge>
                              {entry.status === "DRAFT" && (
                                <Text fontSize="xs" color={mutedTextColor}>
                                  {getDraftApprovalLabel(entry)}
                                </Text>
                              )}
                              {entry.adjustment_required && (
                                <Text
                                  fontSize="xs"
                                  color={
                                    entry.adjustment_completed
                                      ? "green.600"
                                      : "orange.600"
                                  }
                                  noOfLines={2}
                                  title={
                                    entry.adjustment_requirement_tagline ||
                                    entry.adjustment_requirement_reason
                                  }
                                >
                                  {entry.adjustment_completed
                                    ? "Adjustment sudah dibuat"
                                    : "Masuk mapping khusus adjustment"}
                                </Text>
                              )}
                            </VStack>
                          )}
                        </Td>

                        <Td
                          borderBottomWidth={
                            index === (entry.lines?.length || 0) - 1
                              ? "1px"
                              : "0px"
                          }
                        >
                          {index === 0 && (
                            <Menu>
                              <MenuButton
                                as={IconButton}
                                icon={<FiMoreVertical />}
                                variant="ghost"
                                size="xs"
                                aria-label="Actions"
                              />
                              <MenuList
                                bg={menuBg}
                                borderColor={menuBorderColor}
                              >
                                <MenuItem
                                  icon={<FiEye />}
                                  onClick={() => handleView(entry)}
                                  _hover={{ bg: menuHoverBg }}
                                >
                                  {t("journal.viewDetails")}
                                </MenuItem>
                                {entry.status === "DRAFT" &&
                                  canPostDraftEntry &&
                                  entry.source_type === "ADJUSTMENT" &&
                                  !isDirectorApprovalPending(entry) &&
                                  isFinanceManagerApprover && (
                                    <MenuItem
                                      icon={<FiCheckCircle />}
                                      onClick={() => handlePost(entry)}
                                      _hover={{ bg: menuHoverBg }}
                                    >
                                      {t("journal.approveAdjustment")}
                                    </MenuItem>
                                  )}
                                {entry.status === "DRAFT" &&
                                  canPostDraftEntry &&
                                  entry.source_type !== "ADJUSTMENT" &&
                                  !isDirectorApprovalPending(entry) &&
                                  isFinanceManagerApprover && (
                                    <MenuItem
                                      icon={<FiCheckCircle />}
                                      onClick={() => handlePost(entry)}
                                      _hover={{ bg: menuHoverBg }}
                                    >
                                      {t("journal.postVerify")}
                                    </MenuItem>
                                  )}
                                {entry.status === "DRAFT" &&
                                  canPostDraftEntry &&
                                  !isDirectorApprovalPending(entry) &&
                                  isFinanceManagerApprover &&
                                  (entry.source_type === "ADJUSTMENT" ||
                                    entry.source_type === "MANUAL" ||
                                    !entry.source_type) && (
                                    <MenuItem
                                      icon={<FiPlus />}
                                      onClick={() =>
                                        handlePost(entry, {
                                          require_director_approval: true,
                                        })
                                      }
                                      _hover={{ bg: menuHoverBg }}
                                    >
                                      Kirim ke Director
                                    </MenuItem>
                                  )}
                                {entry.status === "DRAFT" &&
                                  canPostDraftEntry &&
                                  isDirectorApprovalPending(entry) &&
                                  isDirectorApprover && (
                                    <MenuItem
                                      icon={<FiCheckCircle />}
                                      onClick={() => handlePost(entry)}
                                      _hover={{ bg: menuHoverBg }}
                                    >
                                      Approve Director & Posting Jurnal
                                    </MenuItem>
                                  )}
                                {entry.status === "DRAFT" &&
                                  canManageGeneralDraft &&
                                  !isDirectorApprovalPending(entry) &&
                                  (entry.source_type !== "ADJUSTMENT" ||
                                    canManageAdjustmentDraft) && (
                                    <MenuItem
                                      icon={<FiEdit2 />}
                                      onClick={() => handleEdit(entry)}
                                      _hover={{ bg: menuHoverBg }}
                                    >
                                      {t("journal.edit")}
                                    </MenuItem>
                                  )}
                                {entry.status === "DRAFT" &&
                                  canDeleteGeneralDraft &&
                                  !isDirectorApprovalPending(entry) &&
                                  (entry.source_type !== "ADJUSTMENT" ||
                                    canManageAdjustmentDraft) && (
                                    <MenuItem
                                      icon={<FiTrash2 />}
                                      onClick={() => handleDelete(entry.id)}
                                      color="red.500"
                                      _hover={{ bg: menuHoverBg }}
                                    >
                                      {t("journal.delete")}
                                    </MenuItem>
                                  )}
                                {canShowCreateAdjustmentAction(entry) && (
                                  <MenuItem
                                    icon={<FiPlus />}
                                    onClick={() =>
                                      handleCreateAdjustment(entry)
                                    }
                                    _hover={{ bg: menuHoverBg }}
                                  >
                                    {t("journal.createAdjustment")}
                                  </MenuItem>
                                )}
                                {entry.status === "POSTED" &&
                                  canReverseJournalEntry && (
                                    <Tooltip
                                      label="Membuat jurnal pembalik untuk membatalkan efek saldo dari jurnal ini. Histori jurnal asli tetap disimpan."
                                      hasArrow
                                      placement="left"
                                      openDelay={200}
                                    >
                                      <MenuItem
                                        icon={<FiRotateCcw />}
                                        onClick={() => handleReverse(entry)}
                                        _hover={{ bg: menuHoverBg }}
                                      >
                                        Reverse
                                      </MenuItem>
                                    </Tooltip>
                                  )}
                              </MenuList>
                            </Menu>
                          )}
                        </Td>
                      </Tr>
                    ))}
                    {/* Spacer Row for visual separation */}
                    <Tr>
                      <Td
                        colSpan={9}
                        height="10px"
                        bg={tableSeparatorBg}
                        p={0}
                      ></Td>
                    </Tr>
                  </React.Fragment>
                ))
              )}
            </Tbody>
          </Table>
        </Box>

        <HStack justify="space-between" wrap="wrap" spacing={3}>
          <Text fontSize="sm" color={mutedTextColor}>
            {totalEntries > 0
              ? `Menampilkan ${(currentPage - 1) * pageSize + 1}-${Math.min(currentPage * pageSize, totalEntries)} dari ${totalEntries} jurnal`
              : "Belum ada jurnal untuk filter ini"}
          </Text>
          <HStack spacing={2}>
            <Button
              size="sm"
              variant="outline"
              leftIcon={<FiChevronLeft />}
              onClick={() => setCurrentPage((prev) => Math.max(prev - 1, 1))}
              isDisabled={currentPage <= 1 || loading}
            >
              Prev
            </Button>
            <Text fontSize="sm" color={mutedTextColor}>
              Halaman {Math.min(currentPage, Math.max(totalPages, 1))} /{" "}
              {Math.max(totalPages, 1)}
            </Text>
            <Button
              size="sm"
              variant="outline"
              rightIcon={<FiChevronRight />}
              onClick={() => setCurrentPage((prev) => prev + 1)}
              isDisabled={loading || currentPage >= Math.max(totalPages, 1)}
            >
              Next
            </Button>
          </HStack>
        </HStack>
      </VStack>
    </Box>
  );
}
