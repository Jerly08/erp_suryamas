"use client";

import React, { useState, useEffect, useCallback } from "react";
import SimpleLayout from "@/components/layout/SimpleLayout";
import ProtectedModule from "@/components/common/ProtectedModule";
import SearchableSelect from "@/components/common/SearchableSelect";
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
import salesService, {
  Sale,
  SalesCodeCounter,
  SalesFilter,
} from "@/services/salesService";
import contactService from "@/services/contactService";
import { projectService, type Project } from "@/services/projectService";
import type { Contact } from "@/types/contact";
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
  CardBody,
  useToast,
  useDisclosure,
  Alert,
  AlertIcon,
  AlertDescription,
  Select,
  VStack,
  useColorModeValue,
  Tooltip,
  IconButton,
  Icon,
  Badge,
  FormControl,
  FormLabel,
  FormHelperText,
  NumberInput,
  NumberInputField,
  NumberInputStepper,
  NumberIncrementStepper,
  NumberDecrementStepper,
  Menu,
  MenuButton,
  MenuList,
  MenuItem,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  ModalCloseButton,
} from "@chakra-ui/react";
import {
  FiPlus,
  FiSearch,
  FiDownload,
  FiRefreshCw,
  FiFilter,
  FiSettings,
  FiChevronLeft,
  FiChevronRight,
  FiChevronsLeft,
  FiChevronsRight,
  FiHash,
  FiSave,
  FiX,
} from "react-icons/fi";
import { handleDeleteError, handleSuccess } from "@/utils/errorHandler";
import {
  isAccountingRole,
  isFinanceManagerRole,
  isFinanceStaffRole,
  normalizeRole,
} from "@/utils/roles";

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

interface SalesCodeCounterState extends SalesCodeCounter {
  loading: boolean;
  saving: boolean;
  error: string | null;
}

function getSalesCounterErrorMessage(error: unknown, fallback: string): string {
  const responseData = (
    error as {
      response?: {
        data?: {
          details?: string;
          message?: string;
          error?: string;
        };
      };
    }
  ).response?.data;

  return (
    responseData?.details ||
    responseData?.message ||
    responseData?.error ||
    fallback
  );
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
  const { user, token } = useAuth();
  const toast = useToast();
  const { t } = useTranslation();
  const { canCreate, canEdit, canDelete, canExport } =
    useModulePermissions("sales");
  const role = normalizeRole(user?.role);
  const isAdmin = role === "admin";
  const canUseCreditCorrection =
    isAdmin ||
    isFinanceStaffRole(role) ||
    isAccountingRole(role) ||
    isFinanceManagerRole(role);

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
  const {
    isOpen: isSalesCounterOpen,
    onOpen: onSalesCounterOpen,
    onClose: onSalesCounterClose,
  } = useDisclosure();

  // Theme-aware colors
  const headingColor = useColorModeValue("gray.800", "var(--text-primary)");
  const subheadingColor = useColorModeValue(
    "gray.600",
    "var(--text-secondary)",
  );
  const tableBg = useColorModeValue("white", "var(--bg-secondary)");
  const textColor = useColorModeValue("gray.600", "var(--text-secondary)");
  const paginationBorderColor = useColorModeValue("gray.200", "gray.700");
  const counterIconBg = useColorModeValue("blue.50", "whiteAlpha.100");
  const counterIconColor = useColorModeValue("blue.600", "blue.200");
  const modalBorderColor = useColorModeValue("gray.200", "gray.700");

  const [state, setState] = useState<SalesPageState>({
    sales: [],
    loading: true,
    error: null,
    filter: {
      page: 1,
      limit: 10,
      search: "",
      status: "",
      customer_id: "",
      project_id: "",
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
  const [salesCodeCounter, setSalesCodeCounter] =
    useState<SalesCodeCounterState>({
      prefix: "SOA",
      nextNumber: 1,
      nextCode: "SOA-00001",
      loading: false,
      saving: false,
      error: null,
    });
  const [filterCustomers, setFilterCustomers] = useState<Contact[]>([]);
  const [filterProjects, setFilterProjects] = useState<Project[]>([]);
  const [loadingFilterOptions, setLoadingFilterOptions] = useState(false);

  const loadFilterOptions = useCallback(async () => {
    if (!token) {
      setFilterCustomers([]);
      setFilterProjects([]);
      return;
    }

    setLoadingFilterOptions(true);
    const [customersResult, projectsResult] = await Promise.allSettled([
      contactService.getContacts(token, "CUSTOMER"),
      projectService.getAll(),
    ]);

    if (customersResult.status === "fulfilled") {
      setFilterCustomers(customersResult.value);
    } else {
      console.error("Failed to load sales customer filters:", customersResult.reason);
      setFilterCustomers([]);
    }

    if (projectsResult.status === "fulfilled") {
      setFilterProjects(projectsResult.value);
    } else {
      console.error("Failed to load sales project filters:", projectsResult.reason);
      setFilterProjects([]);
    }

    setLoadingFilterOptions(false);
  }, [token]);

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

  const loadSalesCodeCounter = useCallback(async () => {
    if (!isAdmin) return;

    setSalesCodeCounter((prev) => ({
      ...prev,
      loading: true,
      error: null,
    }));

    try {
      const counter = await salesService.getSalesCodeCounter();
      setSalesCodeCounter({
        ...counter,
        loading: false,
        saving: false,
        error: null,
      });
    } catch (error: unknown) {
      const message = getSalesCounterErrorMessage(
        error,
        "Gagal memuat counter kode penjualan",
      );

      setSalesCodeCounter((prev) => ({
        ...prev,
        loading: false,
        error: message,
      }));
    }
  }, [isAdmin]);

  // Initial load
  useEffect(() => {
    loadSales();
    loadSalesSummary();
  }, []);

  useEffect(() => {
    loadFilterOptions();
  }, [loadFilterOptions]);

  useEffect(() => {
    if (isAdmin) {
      loadSalesCodeCounter();
    }
  }, [isAdmin, loadSalesCodeCounter]);

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

  const getVisiblePageNumbers = () => {
    const totalPages = Math.max(state.pagination.totalPages, 1);
    const currentPage = Math.min(Math.max(state.pagination.page, 1), totalPages);
    const maxButtons = 5;
    const startPage = Math.max(
      1,
      Math.min(currentPage - Math.floor(maxButtons / 2), totalPages - maxButtons + 1),
    );
    const endPage = Math.min(totalPages, startPage + maxButtons - 1);

    return Array.from(
      { length: endPage - startPage + 1 },
      (_, index) => startPage + index,
    );
  };

  const handleOpenSalesCounter = () => {
    onSalesCounterOpen();
    loadSalesCodeCounter();
  };

  const handleSalesCodeCounterChange = (nextNumberValue: number) => {
    const nextNumber =
      Number.isFinite(nextNumberValue) && nextNumberValue > 0
        ? Math.floor(nextNumberValue)
        : 1;

    setSalesCodeCounter((prev) => ({
      ...prev,
      nextNumber,
      nextCode: salesService.formatSalesCode(prev.prefix, nextNumber),
      error: null,
    }));
  };

  const handleSaveSalesCodeCounter = async () => {
    if (!isAdmin) return;

    setSalesCodeCounter((prev) => ({
      ...prev,
      saving: true,
      error: null,
    }));

    try {
      const updatedCounter = await salesService.updateSalesCodeCounter(
        salesCodeCounter.nextNumber,
      );

      setSalesCodeCounter({
        ...updatedCounter,
        loading: false,
        saving: false,
        error: null,
      });

      toast({
        title: "Counter kode penjualan diperbarui",
        description: `Kode berikutnya: ${updatedCounter.nextCode}`,
        status: "success",
        duration: 3000,
        isClosable: true,
      });
      onSalesCounterClose();
    } catch (error: unknown) {
      const message = getSalesCounterErrorMessage(
        error,
        "Counter tidak bisa disimpan. Pastikan nomor berikutnya belum pernah dipakai.",
      );

      setSalesCodeCounter((prev) => ({
        ...prev,
        saving: false,
        error: message,
      }));

      toast({
        title: "Gagal menyimpan counter kode penjualan",
        description: message,
        status: "error",
        duration: 5000,
        isClosable: true,
      });
    }
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
      await salesService.downloadSalesReportPDF({
        start_date: state.filter.start_date || undefined,
        end_date: state.filter.end_date || undefined,
        status: state.filter.status || undefined,
        customer_id: state.filter.customer_id || undefined,
        project_id: state.filter.project_id || undefined,
        search: state.filter.search || undefined,
      });
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
      await salesService.downloadSalesReportCSV({
        start_date: state.filter.start_date || undefined,
        end_date: state.filter.end_date || undefined,
        status: state.filter.status || undefined,
        customer_id: state.filter.customer_id || undefined,
        project_id: state.filter.project_id || undefined,
        search: state.filter.search || undefined,
      });
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
    loadSalesCodeCounter();
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

  const handleCreditCorrection = (sale: Sale) => {
    window.open(`/sales/${sale.id}?focus=credit-correction`, "_blank");
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
                  loadSalesCodeCounter();
                }}
                isLoading={state.loading}
              />
            </Tooltip>

            {isAdmin && (
              <Button
                leftIcon={<FiHash />}
                colorScheme="blue"
                variant="outline"
                size="md"
                onClick={handleOpenSalesCounter}
                isLoading={salesCodeCounter.loading && isSalesCounterOpen}
              >
                Counter Penjualan
              </Button>
            )}

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

              <Box flex="1" minW="240px">
                <Text
                  fontSize="sm"
                  fontWeight="medium"
                  mb={2}
                  color={textColor}
                >
                  {t("sales.filterByCustomer")}
                </Text>
                <SearchableSelect
                  options={filterCustomers.map((customer) => ({
                    id: customer.id,
                    code: customer.code,
                    name: customer.name,
                    active: customer.is_active,
                  }))}
                  placeholder={t("sales.allCustomers")}
                  value={state.filter.customer_id || ""}
                  onChange={(value) =>
                    handleFilterChange(
                      "customer_id",
                      value ? value.toString() : "",
                    )
                  }
                  isLoading={loadingFilterOptions}
                  displayFormat={(customer) =>
                    customer.code
                      ? `${customer.code} - ${customer.name}`
                      : customer.name
                  }
                />
              </Box>

              <Box flex="1" minW="240px">
                <Text
                  fontSize="sm"
                  fontWeight="medium"
                  mb={2}
                  color={textColor}
                >
                  {t("sales.filterByProject")}
                </Text>
                <SearchableSelect
                  options={filterProjects.map((project) => ({
                    id: project.id,
                    code: project.code,
                    name: project.name,
                    active: project.status === "ACTIVE",
                  }))}
                  placeholder={t("sales.allProjects")}
                  value={state.filter.project_id || ""}
                  onChange={(value) =>
                    handleFilterChange(
                      "project_id",
                      value ? value.toString() : "",
                    )
                  }
                  isLoading={loadingFilterOptions}
                  displayFormat={(project) =>
                    project.code
                      ? `${project.code} - ${project.name}`
                      : project.name
                  }
                />
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
                      customer_id: "",
                      project_id: "",
                      start_date: "",
                      end_date: "",
                      page: 1,
                    },
                  }));
                  loadSales({
                    search: "",
                    status: "",
                    customer_id: "",
                    project_id: "",
                    start_date: "",
                    end_date: "",
                    page: 1,
                    limit: state.pagination.limit,
                  });
                  loadSalesSummary({
                    search: "",
                    status: "",
                    customer_id: "",
                    project_id: "",
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
          onCreditCorrection={
            canUseCreditCorrection ? handleCreditCorrection : undefined
          }
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
            <VStack align={{ base: "stretch", md: "start" }} spacing={0}>
              <Text fontSize="sm" color={textColor}>
                Menampilkan{" "}
                {(state.pagination.page - 1) * state.pagination.limit + 1}-
                {Math.min(
                  state.pagination.page * state.pagination.limit,
                  state.pagination.total,
                )}{" "}
                dari {state.pagination.total} sales
              </Text>
              {state.pagination.totalPages > 1 && (
                <Text fontSize="xs" color={textColor}>
                  Total {state.pagination.totalPages} halaman penjualan.
                </Text>
              )}
            </VStack>
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
                leftIcon={<FiChevronsLeft />}
                onClick={() => handlePageChange(1)}
                isDisabled={state.loading || state.pagination.page <= 1}
              >
                Awal
              </Button>
              <Button
                size="sm"
                variant="outline"
                leftIcon={<FiChevronLeft />}
                onClick={() => handlePageChange(state.pagination.page - 1)}
                isDisabled={state.loading || state.pagination.page <= 1}
              >
                Prev
              </Button>
              {getVisiblePageNumbers().map((pageNumber) => (
                <Button
                  key={pageNumber}
                  size="sm"
                  variant={
                    pageNumber === state.pagination.page ? "solid" : "outline"
                  }
                  colorScheme={
                    pageNumber === state.pagination.page ? "blue" : undefined
                  }
                  onClick={() => handlePageChange(pageNumber)}
                  isDisabled={state.loading}
                  minW="36px"
                >
                  {pageNumber}
                </Button>
              ))}
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
              <Button
                size="sm"
                variant="outline"
                rightIcon={<FiChevronsRight />}
                onClick={() => handlePageChange(state.pagination.totalPages)}
                isDisabled={
                  state.loading ||
                  state.pagination.page >= state.pagination.totalPages
                }
              >
                Akhir
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

      <Modal
        isOpen={isSalesCounterOpen}
        onClose={onSalesCounterClose}
        size="lg"
        isCentered
      >
        <ModalOverlay />
        <ModalContent>
          <ModalHeader borderBottomWidth="1px" borderColor={modalBorderColor}>
            <HStack spacing={3}>
              <Flex
                w="36px"
                h="36px"
                align="center"
                justify="center"
                borderRadius="md"
                bg={counterIconBg}
                color={counterIconColor}
                flexShrink={0}
              >
                <Icon as={FiHash} boxSize={5} />
              </Flex>
              <Box>
                <HStack spacing={2}>
                  <Text>Counter Kode Penjualan</Text>
                  <Badge colorScheme="blue" variant="subtle">
                    Admin
                  </Badge>
                </HStack>
                <Text fontSize="sm" fontWeight="normal" color={subheadingColor}>
                  Atur nomor yang akan dipakai pada penjualan berikutnya.
                </Text>
              </Box>
            </HStack>
          </ModalHeader>
          <ModalCloseButton />
          <ModalBody py={6}>
            <VStack spacing={5} align="stretch">
              <Box>
                <Text fontSize="sm" color={textColor} mb={1}>
                  Preview Kode Berikutnya
                </Text>
                <Heading size="md" color={headingColor}>
                  Kode Penjualan:{" "}
                  {salesCodeCounter.loading
                    ? "Memuat..."
                    : salesCodeCounter.nextCode}
                </Heading>
              </Box>

              {salesCodeCounter.error && (
                <Alert status="error" borderRadius="md">
                  <AlertIcon />
                  <AlertDescription>{salesCodeCounter.error}</AlertDescription>
                </Alert>
              )}

              <FormControl>
                <FormLabel fontSize="sm" color={textColor}>
                  Nomor Berikutnya
                </FormLabel>
                <NumberInput
                  min={1}
                  precision={0}
                  value={salesCodeCounter.nextNumber}
                  onChange={(_, valueAsNumber) =>
                    handleSalesCodeCounterChange(valueAsNumber)
                  }
                  isDisabled={
                    salesCodeCounter.loading || salesCodeCounter.saving
                  }
                >
                  <NumberInputField bg={tableBg} />
                  <NumberInputStepper>
                    <NumberIncrementStepper />
                    <NumberDecrementStepper />
                  </NumberInputStepper>
                </NumberInput>
                <FormHelperText color={subheadingColor}>
                  Jangan isi nomor yang sudah pernah menjadi kode penjualan.
                  Sistem akan menolak nomor yang sudah dipakai.
                </FormHelperText>
              </FormControl>
            </VStack>
          </ModalBody>
          <ModalFooter borderTopWidth="1px" borderColor={modalBorderColor}>
            <HStack spacing={3}>
              <Button
                leftIcon={<FiX />}
                variant="ghost"
                onClick={onSalesCounterClose}
                isDisabled={salesCodeCounter.saving}
              >
                Batal
              </Button>
              <Button
                leftIcon={<FiSave />}
                colorScheme="blue"
                onClick={handleSaveSalesCodeCounter}
                isLoading={salesCodeCounter.saving}
                isDisabled={
                  salesCodeCounter.loading || salesCodeCounter.nextNumber < 1
                }
              >
                Simpan Counter
              </Button>
            </HStack>
          </ModalFooter>
        </ModalContent>
      </Modal>

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
