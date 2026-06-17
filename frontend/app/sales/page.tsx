"use client";

import React, { useState, useEffect } from "react";
import SimpleLayout from "@/components/layout/SimpleLayout";
import ProtectedModule from "@/components/common/ProtectedModule";
import { useAuth } from "@/contexts/AuthContext";
import CompanyTypeGuard from "@/components/common/CompanyTypeGuard";
import { useModulePermissions } from "@/hooks/usePermissions";
import { useTranslation } from "@/hooks/useTranslation";
import EnhancedSalesTable from "@/components/sales/EnhancedSalesTable";
import EnhancedStatsCards from "@/components/sales/EnhancedStatsCards";
import SalesForm from "@/components/sales/SalesForm";
import PaymentForm from "@/components/sales/PaymentForm";
import PPh23SettlementForm from "@/components/sales/PPh23SettlementForm";
import InvoiceTypeManagement from "@/components/invoicetype/InvoiceTypeManagement";
import salesService, { Sale, SalesFilter } from "@/services/salesService";
import {
  Box,
  Heading,
  Text,
  Button,
  Flex,
  HStack,
  Input,
  InputGroup,
  InputLeftElement,
  Card,
  CardHeader,
  CardBody,
  useToast,
  useDisclosure,
  Alert,
  AlertIcon,
  Select,
  VStack,
  useColorModeValue,
  Tooltip,
  IconButton,
  Icon,
  Menu,
  MenuButton,
  MenuList,
  MenuItem,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalCloseButton,
} from "@chakra-ui/react";
import {
  FiPlus,
  FiSearch,
  FiDownload,
  FiRefreshCw,
  FiFilter,
  FiBarChart,
  FiSettings,
  FiChevronLeft,
  FiChevronRight,
} from "react-icons/fi";
import {
  handleLoadingError,
  handleDeleteError,
  handleSuccess,
} from "@/utils/errorHandler";

interface SalesPageState {
  sales: Sale[];
  loading: boolean;
  error: string | null;
  filter: SalesFilter;
  selectedSale: Sale | null;
  summary: any;
  pagination: {
    total: number;
    page: number;
    limit: number;
    totalPages: number;
  };
}

export default function SalesPage() {
  return (
    <ProtectedModule module="sales">
      <CompanyTypeGuard>
        <SimpleLayout>
          <SalesPageContent />
        </SimpleLayout>
      </CompanyTypeGuard>
    </ProtectedModule>
  );
}

const SalesPageContent: React.FC = () => {
  const { user } = useAuth();
  const toast = useToast();
  const { t } = useTranslation();
  const { canCreate, canEdit, canDelete, canExport } =
    useModulePermissions("sales");

  const {
    isOpen: isFormOpen,
    onOpen: onFormOpen,
    onClose: onFormClose,
  } = useDisclosure();
  const {
    isOpen: isPaymentOpen,
    onOpen: onPaymentOpen,
    onClose: onPaymentClose,
  } = useDisclosure();
  const {
    isOpen: isPPh23Open,
    onOpen: onPPh23Open,
    onClose: onPPh23Close,
  } = useDisclosure();
  const {
    isOpen: isInvoiceTypesOpen,
    onOpen: onInvoiceTypesOpen,
    onClose: onInvoiceTypesClose,
  } = useDisclosure();

  // Tooltip descriptions for sales page
  const tooltips = {
    search:
      "Cari penjualan berdasarkan nomor invoice, nama customer, atau referensi",
    status:
      "Filter penjualan berdasarkan status workflow aktif: Draft, Invoiced, Paid, atau Cancelled.",
    dateRange:
      "Filter penjualan berdasarkan tanggal transaksi untuk periode tertentu",
    export: "Unduh data penjualan dalam format PDF atau Excel",
    refresh: "Muat ulang data penjualan terbaru dari server",
  };

  // Theme-aware colors
  const headingColor = useColorModeValue("gray.800", "var(--text-primary)");
  const subheadingColor = useColorModeValue(
    "gray.600",
    "var(--text-secondary)",
  );
  const tableBg = useColorModeValue("white", "var(--bg-secondary)");
  const textColor = useColorModeValue("gray.600", "var(--text-secondary)");
  const paginationBorderColor = useColorModeValue("gray.200", "gray.700");

  const [state, setState] = useState<SalesPageState>({
    sales: [],
    loading: true,
    error: null,
    filter: {
      page: 1,
      limit: 10,
      search: "",
      status: "",
      start_date: "",
      end_date: "",
    },
    selectedSale: null,
    summary: null,
    pagination: {
      total: 0,
      page: 1,
      limit: 10,
      totalPages: 1,
    },
  });

  // Load sales data
  const loadSales = async (newFilter?: Partial<SalesFilter>) => {
    try {
      setState((prev) => ({ ...prev, loading: true, error: null }));

      const filter = newFilter
        ? { ...state.filter, ...newFilter }
        : state.filter;
      const result = await salesService.getSales(filter);
      const page = result.page || filter.page || 1;
      const limit = result.limit || filter.limit || 10;
      const total = result.total || 0;
      const totalPages = Math.max(
        1,
        result.total_pages || Math.ceil(total / limit) || 1,
      );

      setState((prev) => ({
        ...prev,
        sales: result.data || [],
        filter: { ...filter, page, limit },
        pagination: {
          total,
          page,
          limit,
          totalPages,
        },
        loading: false,
      }));
    } catch (error: any) {
      // Don't show error for 401 (authentication) - modal handles it
      if (error.response?.status === 401) {
        setState((prev) => ({
          ...prev,
          loading: false,
          error: null, // Clear error for auth issues
        }));
        return; // Exit early, modal will handle auth error
      }

      // Handle other errors normally
      setState((prev) => ({
        ...prev,
        error: error.response?.data?.message || "Failed to load sales data",
        loading: false,
      }));

      toast({
        title: "Error loading sales",
        description:
          error.response?.data?.message || "Failed to load sales data",
        status: "error",
        duration: 3000,
      });
    }
  };

  // Load sales summary
  const loadSalesSummary = async (filterOverrides?: Partial<SalesFilter>) => {
    try {
      const mergedFilter = filterOverrides
        ? { ...state.filter, ...filterOverrides }
        : state.filter;
      const summary = await salesService.getSalesSummary(mergedFilter);
      setState((prev) => ({ ...prev, summary }));
    } catch (error) {
      console.error("Failed to load sales summary:", error);
    }
  };

  // Initial load
  useEffect(() => {
    loadSales();
    loadSalesSummary();
  }, []);

  // Handle search
  const handleSearch = (searchTerm: string) => {
    setState((prev) => ({
      ...prev,
      filter: { ...prev.filter, search: searchTerm },
    }));
    loadSales({ search: searchTerm, page: 1 });
    loadSalesSummary({ search: searchTerm, page: 1 });
  };

  // Handle filter change
  const handleFilterChange = (key: keyof SalesFilter, value: string) => {
    setState((prev) => ({ ...prev, filter: { ...prev.filter, [key]: value } }));
    loadSales({ [key]: value, page: 1 });
    loadSalesSummary({ [key]: value, page: 1 });
  };

  const handlePageChange = (page: number) => {
    const totalPages = Math.max(state.pagination.totalPages, 1);
    const nextPage = Math.min(Math.max(page, 1), totalPages);
    loadSales({ page: nextPage });
  };

  const handlePageSizeChange = (limit: number) => {
    loadSales({ limit, page: 1 });
  };

  // Handle create/edit sale
  const handleSaleAction = (sale?: Sale) => {
    setState((prev) => ({ ...prev, selectedSale: sale || null }));
    onFormOpen();
  };

  // Handle payment
  const handlePayment = (sale: Sale) => {
    setState((prev) => ({ ...prev, selectedSale: sale }));
    onPaymentOpen();
  };

  const handlePPh23Settlement = (sale: Sale) => {
    setState((prev) => ({ ...prev, selectedSale: sale }));
    onPPh23Open();
  };

  // Handle delete sale
  const handleDelete = async (sale: Sale) => {
    if (!window.confirm("Are you sure you want to delete this sale?")) return;

    try {
      await salesService.deleteSale(sale.id);
      handleSuccess("Sale has been deleted successfully", toast, "delete sale");
      loadSales();
      loadSalesSummary();
    } catch (error: any) {
      handleDeleteError("sale", error, toast);
    }
  };

  // Handle sale status actions
  const handleSaleStatusAction = async (
    sale: Sale,
    action: "confirm" | "cancel",
  ) => {
    try {
      let message = "";

      switch (action) {
        case "confirm":
          // FIXED: Use createInvoiceFromSale to directly create invoice (DRAFT -> INVOICED)
          // This will create journal entries and set proper accounting impact
          await salesService.createInvoiceFromSale(sale.id);
          message =
            "Sale has been invoiced successfully! Journal entries have been created.";
          break;
        case "cancel":
          const reason = window.prompt(
            "Please provide a reason for cancellation:",
          );
          if (!reason) return;
          await salesService.cancelSale(sale.id, reason);
          message = "Sale has been cancelled";
          break;
      }

      handleSuccess(message, toast, action + " sale");
      loadSales();
      loadSalesSummary();
    } catch (error: any) {
      toast({
        title: `Error ${action}ing sale`,
        description:
          error.response?.data?.message || `Failed to ${action} sale`,
        status: "error",
        duration: 3000,
      });
    }
  };

  // Export handlers
  const handleExportPDF = async () => {
    try {
      await salesService.downloadSalesReportPDF(
        state.filter.start_date || undefined,
        state.filter.end_date || undefined,
        state.filter.status || undefined,
        state.filter.search || undefined,
      );
      handleSuccess(
        "Sales report PDF has been downloaded",
        toast,
        "export report",
      );
    } catch (error: any) {
      toast({
        title: "Export failed",
        description: "Failed to export sales report PDF",
        status: "error",
        duration: 3000,
      });
    }
  };

  const handleExportCSV = async () => {
    try {
      await salesService.downloadSalesReportCSV(
        state.filter.start_date || undefined,
        state.filter.end_date || undefined,
        state.filter.status || undefined,
        state.filter.search || undefined,
      );
      handleSuccess(
        "Sales report CSV has been downloaded",
        toast,
        "export report",
      );
    } catch (error: any) {
      toast({
        title: "Export failed",
        description: "Failed to export sales report CSV",
        status: "error",
        duration: 3000,
      });
    }
  };

  // Handle duplicate sale
  const handleDuplicate = async (sale: Sale) => {
    try {
      // Fetch the latest data to ensure we duplicate the most recent version
      setState((prev) => ({ ...prev, loading: true }));
      const freshSale = await salesService.getSale(sale.id);

      // Create a copy without ID and reset status-related fields
      const clonedSale = {
        ...freshSale,
        id: 0, // Reset ID to indicate new record
        invoice_number: "", // Clear invoice number
        code: "", // Clear code
        status: "DRAFT", // Reset status
        date: new Date().toISOString(), // Set to today
        due_date: new Date().toISOString(), // Set to today (will be recalculated)
        paid_amount: 0,
        outstanding_amount: 0,
        approval_status: "NOT_STARTED",
        approval_request: undefined,
        logs: [],
        // Ensure items ID are also reset if necessary, or handled by backend as new items
        sale_items: freshSale.sale_items?.map((item) => ({
          ...item,
          id: 0, // Reset item ID
          sale_id: 0,
        })),
      } as Sale;

      setState((prev) => ({
        ...prev,
        selectedSale: clonedSale,
        loading: false,
      }));
      onFormOpen();
    } catch (error) {
      console.error("Error fetching sale details for duplication:", error);
      toast({
        title: "Error duplicating sale",
        description: "Failed to fetch latest sale data.",
        status: "error",
        duration: 3000,
      });
      setState((prev) => ({ ...prev, loading: false }));
    }
  };

  // Handle form save
  const handleFormSave = () => {
    onFormClose();
    setState((prev) => ({ ...prev, selectedSale: null })); // Reset selected sale
    loadSales();
    loadSalesSummary();
  };

  // Handle form close
  const handleFormClose = () => {
    onFormClose();
    setState((prev) => ({ ...prev, selectedSale: null })); // Reset selected sale
  };

  // Handle payment save
  const handlePaymentSave = () => {
    onPaymentClose();
    loadSales();
    loadSalesSummary();
  };

  const handlePPh23Save = () => {
    onPPh23Close();
    loadSales();
    loadSalesSummary();
  };

  // Handle view details
  const handleViewDetails = (sale: Sale) => {
    window.open(`/sales/${sale.id}`, "_blank");
  };

  // Handle download invoice
  const handleDownloadInvoice = (sale: Sale) => {
    salesService.downloadInvoicePDF(sale.id, sale.invoice_number || sale.code);
  };

  // Handle create receipt (PDF) when fully paid
  const handleCreateReceipt = (sale: Sale) => {
    if (sale.status !== "PAID" && sale.outstanding_amount > 0) {
      toast({
        title: "Not fully paid",
        description: "Receipt can only be created when the sale is fully paid.",
        status: "warning",
        duration: 3000,
      });
      return;
    }
    salesService
      .downloadReceiptPDF(sale.id, sale.invoice_number || sale.code)
      .then(() => {
        handleSuccess(
          "Receipt has been generated and downloaded",
          toast,
          "download receipt",
        );
      })
      .catch((err) => {
        toast({
          title: "Failed to generate receipt",
          description: err?.message || "Please try again later",
          status: "error",
          duration: 4000,
        });
      });
  };

  return (
    <>
      <Box>
        {/* Header */}
        <Flex justify="space-between" align="center" mb={6} wrap="wrap" gap={4}>
          <VStack align="start" spacing={1}>
            <Heading size="xl" color={headingColor} fontWeight="600">
              {t("sales.management")}
            </Heading>
            <Text color={subheadingColor} fontSize="md">
              {t("sales.manageTransactions")}
            </Text>
          </VStack>

          <HStack spacing={3}>
            <Tooltip label={t("sales.refreshData")}>
              <IconButton
                aria-label="Refresh"
                icon={<FiRefreshCw />}
                variant="ghost"
                onClick={() => {
                  loadSales();
                  loadSalesSummary();
                }}
                isLoading={state.loading}
              />
            </Tooltip>

            <Button
              leftIcon={<FiSettings />}
              colorScheme="purple"
              variant="outline"
              size="md"
              onClick={onInvoiceTypesOpen}
              title="Manage Invoice Types"
            >
              {t("sales.invoiceTypes")}
            </Button>

            {canExport && (
              <Menu>
                <MenuButton
                  as={Button}
                  leftIcon={<FiDownload />}
                  colorScheme="green"
                  variant="outline"
                  size="md"
                >
                  {t("sales.exportReport")}
                </MenuButton>
                <MenuList>
                  <MenuItem onClick={handleExportPDF}>
                    {t("sales.exportPDF")}
                  </MenuItem>
                  <MenuItem onClick={handleExportCSV}>
                    {t("sales.exportCSV")}
                  </MenuItem>
                </MenuList>
              </Menu>
            )}

            {(canCreate || !user) && (
              <Button
                leftIcon={<FiPlus />}
                colorScheme="blue"
                size="md"
                px={6}
                fontWeight="medium"
                onClick={() => handleSaleAction()}
                _hover={{
                  transform: "translateY(-1px)",
                  boxShadow: "lg",
                }}
              >
                {t("sales.createSale")}
              </Button>
            )}
          </HStack>
        </Flex>

        {/* Summary Cards */}
        {state.summary && (
          <Box mb={6}>
            <EnhancedStatsCards
              stats={state.summary}
              formatCurrency={salesService.formatCurrency}
            />
          </Box>
        )}

        {/* Search and Filters */}
        <Card mb={6}>
          <CardBody>
            <Flex gap={4} align="end" wrap="wrap">
              <Box flex="1" minW="300px">
                <Text
                  fontSize="sm"
                  fontWeight="medium"
                  mb={2}
                  color={textColor}
                >
                  {t("sales.searchTransactions")}
                </Text>
                <InputGroup>
                  <InputLeftElement pointerEvents="none">
                    <FiSearch color={textColor} />
                  </InputLeftElement>
                  <Input
                    placeholder={t("sales.searchPlaceholder")}
                    value={state.filter.search}
                    onChange={(e) => handleSearch(e.target.value)}
                    bg={tableBg}
                  />
                </InputGroup>
              </Box>

              <Box minW="180px">
                <Text
                  fontSize="sm"
                  fontWeight="medium"
                  mb={2}
                  color={textColor}
                >
                  {t("sales.filterByStatus")}
                </Text>
                <Select
                  placeholder={t("sales.allStatus")}
                  value={state.filter.status}
                  onChange={(e) => handleFilterChange("status", e.target.value)}
                  bg={tableBg}
                >
                  <option value="DRAFT">{t("sales.draft")}</option>
                  <option value="INVOICED">{t("sales.invoiced")}</option>
                  <option value="PAID">{t("sales.paid")}</option>
                  <option value="CANCELLED">{t("sales.cancelled")}</option>
                </Select>
              </Box>

              <Box minW="160px">
                <Text
                  fontSize="sm"
                  fontWeight="medium"
                  mb={2}
                  color={textColor}
                >
                  {t("sales.startDate")}
                </Text>
                <Input
                  type="date"
                  value={state.filter.start_date}
                  onChange={(e) =>
                    handleFilterChange("start_date", e.target.value)
                  }
                  bg={tableBg}
                />
              </Box>

              <Box minW="160px">
                <Text
                  fontSize="sm"
                  fontWeight="medium"
                  mb={2}
                  color={textColor}
                >
                  {t("sales.endDate")}
                </Text>
                <Input
                  type="date"
                  value={state.filter.end_date}
                  onChange={(e) =>
                    handleFilterChange("end_date", e.target.value)
                  }
                  bg={tableBg}
                />
              </Box>

              <Button
                leftIcon={<FiFilter />}
                variant="outline"
                onClick={() => {
                  setState((prev) => ({
                    ...prev,
                    filter: {
                      ...prev.filter,
                      search: "",
                      status: "",
                      start_date: "",
                      end_date: "",
                      page: 1,
                    },
                  }));
                  loadSales({
                    search: "",
                    status: "",
                    start_date: "",
                    end_date: "",
                    page: 1,
                    limit: state.pagination.limit,
                  });
                  loadSalesSummary({
                    search: "",
                    status: "",
                    start_date: "",
                    end_date: "",
                    page: 1,
                  });
                }}
              >
                {t("sales.clearFilters")}
              </Button>
            </Flex>
          </CardBody>
        </Card>

        {/* Error Alert */}
        {state.error && (
          <Alert status="error" mb={6}>
            <AlertIcon />
            {state.error}
          </Alert>
        )}

        {/* Enhanced Sales Table */}
        <EnhancedSalesTable
          sales={state.sales || []}
          loading={state.loading}
          onViewDetails={handleViewDetails}
          onEdit={canEdit ? (sale) => handleSaleAction(sale) : undefined}
          onConfirm={
            canEdit
              ? (sale) => handleSaleStatusAction(sale, "confirm")
              : undefined
          }
          onCancel={
            canEdit
              ? (sale) => handleSaleStatusAction(sale, "cancel")
              : undefined
          }
          onPayment={canEdit ? handlePayment : undefined}
          onPPh23Settlement={canEdit ? handlePPh23Settlement : undefined}
          onDelete={canDelete ? handleDelete : undefined}
          onDuplicate={canCreate ? handleDuplicate : undefined}
          onDownloadInvoice={handleDownloadInvoice}
          onCreateReceipt={handleCreateReceipt}
          formatCurrency={salesService.formatCurrency}
          formatDate={salesService.formatDate}
          canEdit={canEdit}
          canDelete={canDelete}
          totalCount={state.pagination.total}
        />

        {state.pagination.total > 0 && (
          <Flex
            mt={4}
            px={4}
            py={3}
            borderWidth="1px"
            borderColor={paginationBorderColor}
            borderRadius="md"
            bg={tableBg}
            align={{ base: "stretch", md: "center" }}
            justify="space-between"
            direction={{ base: "column", md: "row" }}
            gap={3}
          >
            <Text fontSize="sm" color={textColor}>
              Menampilkan{" "}
              {(state.pagination.page - 1) * state.pagination.limit + 1}-
              {Math.min(
                state.pagination.page * state.pagination.limit,
                state.pagination.total,
              )}{" "}
              dari {state.pagination.total} sales
            </Text>
            <Flex align="center" gap={2} wrap="wrap">
              <Text fontSize="sm" color={textColor}>
                Rows
              </Text>
              <Select
                size="sm"
                maxW="90px"
                value={state.pagination.limit}
                onChange={(e) => handlePageSizeChange(Number(e.target.value))}
                isDisabled={state.loading}
              >
                <option value={10}>10</option>
                <option value={25}>25</option>
                <option value={50}>50</option>
              </Select>
              <Button
                size="sm"
                variant="outline"
                leftIcon={<FiChevronLeft />}
                onClick={() => handlePageChange(state.pagination.page - 1)}
                isDisabled={state.loading || state.pagination.page <= 1}
              >
                Prev
              </Button>
              <Text fontSize="sm" color={textColor}>
                Halaman {state.pagination.page} / {state.pagination.totalPages}
              </Text>
              <Button
                size="sm"
                variant="outline"
                rightIcon={<FiChevronRight />}
                onClick={() => handlePageChange(state.pagination.page + 1)}
                isDisabled={
                  state.loading ||
                  state.pagination.page >= state.pagination.totalPages
                }
              >
                Next
              </Button>
            </Flex>
          </Flex>
        )}
      </Box>

      {/* Sales Form Modal */}
      <SalesForm
        isOpen={isFormOpen}
        onClose={handleFormClose}
        onSave={handleFormSave}
        sale={
          state.selectedSale && state.selectedSale.id > 0
            ? state.selectedSale
            : undefined
        }
        initialData={
          state.selectedSale && state.selectedSale.id === 0
            ? state.selectedSale
            : undefined
        }
      />

      {/* Payment Form Modal */}
      <PaymentForm
        isOpen={isPaymentOpen}
        onClose={onPaymentClose}
        onSave={handlePaymentSave}
        sale={state.selectedSale}
      />

      <PPh23SettlementForm
        isOpen={isPPh23Open}
        onClose={onPPh23Close}
        onSave={handlePPh23Save}
        sale={state.selectedSale}
      />

      {/* Invoice Types Management Modal */}
      <Modal
        isOpen={isInvoiceTypesOpen}
        onClose={onInvoiceTypesClose}
        size="6xl"
        scrollBehavior="inside"
      >
        <ModalOverlay />
        <ModalContent maxH="90vh">
          <ModalHeader>
            <HStack>
              <FiSettings />
              <Text>
                {t("sales.invoiceTypes")} {t("common.management")}
              </Text>
            </HStack>
          </ModalHeader>
          <ModalCloseButton />
          <ModalBody p={0}>
            <InvoiceTypeManagement />
          </ModalBody>
        </ModalContent>
      </Modal>
    </>
  );
};
