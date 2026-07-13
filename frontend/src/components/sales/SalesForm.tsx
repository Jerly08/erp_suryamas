"use client";

import React, { useState, useEffect, useRef } from "react";
import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  ModalCloseButton,
  Button,
  FormControl,
  FormLabel,
  FormErrorMessage,
  FormHelperText,
  Input,
  Select,
  Textarea,
  VStack,
  HStack,
  SimpleGrid,
  Box,
  Divider,
  Text,
  IconButton,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  TableContainer,
  useToast,
  NumberInput,
  NumberInputField,
  NumberInputStepper,
  NumberIncrementStepper,
  NumberDecrementStepper,
  Switch,
  Badge,
  Tooltip,
  Flex,
  Card,
  CardHeader,
  CardBody,
  Heading,
  Alert,
  AlertIcon,
  AlertDescription,
  Icon,
  useColorModeValue,
} from "@chakra-ui/react";
import CurrencyInput from "@/components/common/CurrencyInput";
import ErrorAlert, { ErrorDetail } from "@/components/common/ErrorAlert";
import { useForm, useFieldArray, Controller } from "react-hook-form";
import {
  FiPlus,
  FiTrash2,
  FiSave,
  FiX,
  FiDollarSign,
  FiShoppingCart,
  FiFileText,
} from "react-icons/fi";
import salesService, {
  Sale,
  SaleCreateRequest,
  SaleUpdateRequest,
  SaleItemRequest,
  SaleItemUpdateRequest,
  SalePaymentScheduleRequest,
} from "@/services/salesService";
import invoiceTypeService, { InvoiceType } from "@/services/invoiceTypeService";
import ErrorHandler, { ParsedValidationError } from "@/utils/errorHandler";
import { useAuth } from "@/contexts/AuthContext";
import { useTranslation } from "@/hooks/useTranslation";
import { useModulePermissions } from "@/hooks/usePermissions";
import { isFinanceManagerRole, normalizeRole } from "@/utils/roles";
import {
  filterActiveLeafAccounts,
  filterOperationalCashBankAccounts,
} from "@/utils/accountFilters";
import { calculateSalesAmounts } from "@/utils/salesCalculation";

interface SalesFormProps {
  isOpen: boolean;
  onClose: () => void;
  onSave: () => void;
  sale?: Sale | null;
  initialData?: Sale | null; // For duplication/cloning
}

type AccountCatalogRow = {
  id: number;
  code: string;
  name: string;
  active?: boolean;
  parent_id?: number;
  parent_code?: string;
  parent_name?: string;
  is_active?: boolean;
  is_header?: boolean;
};

const isReceivableAccountCandidate = (account: AccountCatalogRow): boolean => {
  const normalizedName = (account.name || "").toLowerCase();
  const normalizedCode = (account.code || "").toLowerCase();
  const normalizedParentName = (account.parent_name || "").toLowerCase();
  const normalizedParentCode = (account.parent_code || "").toLowerCase();
  const isContraReceivable =
    normalizedName.includes("cadangan") ||
    normalizedName.includes("kerugian") ||
    normalizedName.includes("penyisihan") ||
    normalizedName.includes("allowance") ||
    normalizedName.includes("provision") ||
    normalizedName.includes("contra");

  return !isContraReceivable && (
    normalizedName.includes("piutang") ||
    normalizedName.includes("receivable") ||
    normalizedName.includes("intercomp") ||
    normalizedName.includes("intercompany") ||
    normalizedParentName.includes("piutang") ||
    normalizedParentName.includes("receivable") ||
    normalizedParentName.includes("intercomp") ||
    normalizedParentName.includes("intercompany") ||
    normalizedCode.startsWith("1106-") ||
    normalizedParentCode === "1106"
  );
};

interface FormData {
  customer_id: number;
  sales_person_id?: number;
  project_id?: number;
  invoice_type_id?: number;
  type: string;
  date: string;
  due_date?: string;
  valid_until?: string;
  currency: string;
  exchange_rate: number;
  discount_percent: number;
  // Payment method and accounting fields
  payment_method_type: "CASH" | "BANK" | "CREDIT";
  cash_bank_id?: number;
  receivable_account_id?: number;
  // Legacy tax fields (for backward compatibility)
  ppn_percent: number;
  pph_percent: number;
  pph_type?: string;
  // Enhanced tax configuration
  ppn_rate: number;
  other_tax_additions: number;
  pph21_rate: number;
  pph23_rate: number;
  other_tax_deductions: number;
  payment_terms: string;
  payment_schedules: Array<{
    sequence?: number;
    percentage: number;
    due_date: string;
    description?: string;
  }>;
  payment_method?: string;
  shipping_method?: string;
  shipping_cost: number;
  shipping_taxable: boolean;
  billing_address?: string;
  shipping_address?: string;
  notes?: string;
  internal_notes?: string;
  reference?: string;
  items: Array<{
    id?: number;
    product_id: number;
    product_name?: string;
    description: string;
    location?: string;
    quantity: number;
    unit_price: number;
    estimated_unit_cost?: number;
    discount_percent: number;
    taxable: boolean;
    revenue_account_id?: number;
    delete?: boolean;
  }>;
}

type ShippingTreatment = "buyer_taxable" | "buyer_non_taxable" | "buyer_direct";

const resolveShippingTreatment = (
  shippingCost: number,
  shippingTaxable: boolean,
): ShippingTreatment => {
  if ((Number(shippingCost) || 0) <= 0 && !shippingTaxable) {
    return "buyer_direct";
  }

  return shippingTaxable ? "buyer_taxable" : "buyer_non_taxable";
};

const SalesForm: React.FC<SalesFormProps> = ({
  isOpen,
  onClose,
  onSave,
  sale,
  initialData,
}) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [customers, setCustomers] = useState<any[]>([]);
  const [projects, setProjects] = useState<any[]>([]);
  const [products, setProducts] = useState<any[]>([]);
  const [salesPersons, setSalesPersons] = useState<any[]>([]);
  const [accounts, setAccounts] = useState<any[]>([]);
  const [defaultRevenueAccountId, setDefaultRevenueAccountId] = useState<
    number | null
  >(null);
  const [defaultRevenueAccountLabel, setDefaultRevenueAccountLabel] =
    useState("");
  const [receivableAccounts, setReceivableAccounts] = useState<
    AccountCatalogRow[]
  >([]);
  const [cashBankAccounts, setCashBankAccounts] = useState<any[]>([]);
  const [invoiceTypes, setInvoiceTypes] = useState<InvoiceType[]>([]);
  const [loadingData, setLoadingData] = useState(true);
  const [defaultTaxRate, setDefaultTaxRate] = useState<number>(0);
  const [validationError, setValidationError] =
    useState<ParsedValidationError | null>(null);
  const toast = useToast();
  const { user, token } = useAuth();
  const modalBodyRef = useRef<HTMLDivElement>(null);

  // Color mode values for dark mode support
  const modalBg = useColorModeValue("white", "gray.800");
  const headerBg = useColorModeValue("blue.50", "gray.700");
  const headingColor = useColorModeValue("blue.700", "blue.300");
  const subHeadingColor = useColorModeValue("gray.600", "gray.300");
  const textColor = useColorModeValue("gray.600", "gray.400");
  const inputBg = useColorModeValue("gray.50", "gray.600");
  const inputFocusBg = useColorModeValue("white", "gray.500");
  const tableBg = useColorModeValue("white", "gray.700");
  const borderColor = useColorModeValue("gray.200", "gray.600");
  const footerBg = useColorModeValue("white", "gray.800");
  const shadowColor = useColorModeValue(
    "rgba(0, 0, 0, 0.1)",
    "rgba(0, 0, 0, 0.3)",
  );
  const alertBg = useColorModeValue("blue.50", "blue.900");
  const alertBorderColor = useColorModeValue("blue.200", "blue.700");
  const readOnlyBg = useColorModeValue("gray.100", "whiteAlpha.100");
  const successTextColor = useColorModeValue("green.600", "green.200");
  const manualEntryBg = useColorModeValue(
    "orange.50",
    "rgba(221, 107, 32, 0.16)",
  );
  const manualEntryFocusBg = useColorModeValue(
    "orange.100",
    "rgba(221, 107, 32, 0.24)",
  );
  const manualEntryBorderColor = useColorModeValue("orange.200", "orange.400");
  const scrollTrackBg = useColorModeValue("#f7fafc", "#2d3748");
  const scrollThumbBg = useColorModeValue("#cbd5e0", "#4a5568");
  const scrollThumbHoverBg = useColorModeValue("#a0aec0", "#718096");
  const fieldFocusStyles = {
    borderColor: "blue.400",
    boxShadow: "0 0 0 1px var(--chakra-colors-blue-400)",
  };
  const baseFieldStyles = {
    bg: inputBg,
    borderColor,
    color: useColorModeValue("gray.800", "whiteAlpha.900"),
    _hover: { borderColor },
    _focusVisible: fieldFocusStyles,
  };

  const {
    canCreate: canCreateSales,
    canEdit: canEditSales,
    loading: loadingSalesPermissions,
  } = useModulePermissions("sales");
  const { canView: canViewAccounts } = useModulePermissions("accounts");
  const canSelectRevenueAccountFromCOA =
    canViewAccounts || isFinanceManagerRole(normalizeRole(user?.role));

  // For new sales, check create permission; for editing, check edit permission
  const hasPermission = sale ? canEditSales : canCreateSales;

  // Partial Edit: if sale is not DRAFT, lock financial fields (items, customer, tax, discount)
  const isInvoiced = !!(sale && sale.status && sale.status !== "DRAFT");

  // If modal is opened but user doesn't have permission, close it and show error
  useEffect(() => {
    if (isOpen && user && !loadingSalesPermissions && !hasPermission) {
      const action = sale ? "edit" : "create";
      toast({
        title: t("sales.messages.accessDenied"),
        description: sale
          ? t("sales.messages.noPermissionEdit")
          : t("sales.messages.noPermissionCreate"),
        status: "error",
        duration: 5000,
      });
      onClose();
    }
  }, [isOpen, user, loadingSalesPermissions, hasPermission, sale, toast, onClose]);

  const {
    register,
    handleSubmit,
    reset,
    watch,
    setValue,
    setError,
    control,
    clearErrors,
    formState: { errors },
  } = useForm<FormData>({
    defaultValues: {
      type: "INVOICE",
      currency: "IDR",
      exchange_rate: 1,
      project_id: undefined,
      discount_percent: 0,
      // Legacy tax fields — will be overridden by system settings in loadFormData
      ppn_percent: 0,
      pph_percent: 0,
      // Enhanced tax configuration — will be overridden by system settings in loadFormData
      ppn_rate: 0,
      other_tax_additions: 0,
      pph21_rate: 0,
      pph23_rate: 0,
      other_tax_deductions: 0,
      payment_terms: "NET_30",
      payment_schedules: [],
      payment_method_type: "CREDIT" as "CASH" | "BANK" | "CREDIT",
      receivable_account_id: undefined,
      shipping_cost: 0,
      shipping_taxable: true,
      items: [
        {
          product_id: 0,
          product_name: "",
          description: "",
          location: "",
          quantity: 1,
          unit_price: 0,
          estimated_unit_cost: 0,
          discount_percent: 0,
          taxable: true,
        },
      ],
    },
  });

  const { fields, append, remove } = useFieldArray({
    control,
    name: "items",
  });

  const {
    fields: scheduleFields,
    append: appendSchedule,
    remove: removeSchedule,
  } = useFieldArray({
    control,
    name: "payment_schedules",
  });

  const watchItems = watch("items");
  const watchDiscountPercent = watch("discount_percent");
  // Legacy tax fields
  const watchPPNPercent = watch("ppn_percent");
  const watchPPhPercent = watch("pph_percent");
  // Enhanced tax configuration
  const watchPPNRate = watch("ppn_rate");
  const watchOtherTaxAdditions = watch("other_tax_additions");
  const watchPPh21Rate = watch("pph21_rate");
  const watchPPh23Rate = watch("pph23_rate");
  const watchOtherTaxDeductions = watch("other_tax_deductions");
  const watchShippingCost = watch("shipping_cost");
  const watchShippingTaxable = watch("shipping_taxable");
  const watchPaymentTerms = watch("payment_terms");
  const watchPaymentSchedules = watch("payment_schedules");
  const watchPaymentMethodType = watch("payment_method_type");
  const watchCashBankId = watch("cash_bank_id");
  const watchReceivableAccountId = watch("receivable_account_id");
  const watchDate = watch("date");
  const watchType = watch("type");
  const watchDueDate = watch("due_date");
  const [discountType, setDiscountType] = useState<"percentage" | "amount">(
    "percentage",
  );
  const [shippingTreatment, setShippingTreatment] =
    useState<ShippingTreatment>("buyer_taxable");
  const shouldHideForm = !loadingSalesPermissions && !hasPermission && !!user;

  const resolveRevenueAccountId = (item: FormData["items"][number]) =>
    item.revenue_account_id || defaultRevenueAccountId || 0;

  const getRevenueAccountHelperText = () =>
    defaultRevenueAccountLabel
      ? `COA akan memakai default sistem: ${defaultRevenueAccountLabel}`
      : "COA akan memakai default sistem saat sales disimpan.";

  const resolveReceivableAccountId = (formReceivableAccountId?: number) =>
    formReceivableAccountId ||
    watchReceivableAccountId ||
    0;

  const getReceivableAccountHelperText = () =>
    "Akun yang dipilih akan muncul sebagai debit piutang di jurnal invoice kredit.";

  useEffect(() => {
    if (isOpen) {
      // Load form data first, then populate with sale data
      loadFormData().then(() => {
        if (sale) {
          // Small delay to ensure all state updates are complete
          setTimeout(() => {
            populateFormWithSale(sale);
          }, 100);
        } else if (initialData) {
          // Handle duplicated/cloned data
          setTimeout(() => {
            populateFormWithSale(initialData);
            // After population, ensure date is set to today for duplicated items (though handleDuplicate does it, re-confirm here)
            setValue("date", new Date().toISOString().split("T")[0]);
          }, 100);
        } else {
          resetForm();
        }
      });
    }
  }, [isOpen, sale, initialData]);

  // Auto-calculate due date based on payment terms and date
  useEffect(() => {
    if (
      watchDate &&
      watchPaymentTerms &&
      watchPaymentTerms !== "CUSTOM" &&
      !watchDueDate
    ) {
      const calculatedDueDate = calculateDueDateFromPaymentTerms(
        watchDate,
        watchPaymentTerms,
      );
      if (calculatedDueDate) {
        setValue("due_date", calculatedDueDate);
      }
    }
  }, [watchDate, watchPaymentTerms, setValue]);

  // Sync legacy ppn_percent with new ppn_rate
  useEffect(() => {
    if (watchPPNRate !== undefined && watchPPNRate !== null) {
      if (watchPPNPercent !== watchPPNRate) {
        setValue("ppn_percent", watchPPNRate);
      }
    }
  }, [watchPPNRate, watchPPNPercent, setValue]);

  // Reset due date when payment terms change (except for CUSTOM)
  useEffect(() => {
    if (watchPaymentTerms && watchPaymentTerms !== "CUSTOM" && watchDate) {
      const calculatedDueDate = calculateDueDateFromPaymentTerms(
        watchDate,
        watchPaymentTerms,
      );
      if (calculatedDueDate) {
        setValue("due_date", calculatedDueDate);
      }
    }
  }, [watchPaymentTerms, watchDate, setValue]);

  useEffect(() => {
    if (watchPaymentTerms !== "CUSTOM") {
      return;
    }

    if ((watchPaymentSchedules || []).length === 0) {
      appendSchedule({
        sequence: 1,
        percentage: 100,
        due_date: watchDate || "",
        description: "Pelunasan invoice",
      });
      return;
    }

    const lastScheduleDueDate = getLastCustomScheduleDueDate();
    if (lastScheduleDueDate) {
      setValue("due_date", lastScheduleDueDate);
    }
  }, [
    watchPaymentTerms,
    watchPaymentSchedules,
    watchDate,
    appendSchedule,
    setValue,
  ]);

  useEffect(() => {
    if (watchPaymentMethodType !== "CREDIT" && watchPaymentTerms === "CUSTOM") {
      setValue("payment_terms", "COD");
      setValue("payment_schedules", []);
    }
  }, [watchPaymentMethodType, watchPaymentTerms, setValue]);

  useEffect(() => {
    if (watchPaymentMethodType !== "CREDIT") {
      clearErrors("receivable_account_id");
    }
  }, [
    watchPaymentMethodType,
    clearErrors,
  ]);

  useEffect(() => {
    if (!watchCashBankId || cashBankAccounts.length === 0) {
      return;
    }

    const selectedAccount = cashBankAccounts.find(
      (account) => account.id === watchCashBankId,
    );

    if (!selectedAccount) {
      setValue("cash_bank_id", undefined, {
        shouldDirty: true,
        shouldValidate: true,
      });
      return;
    }

    const normalizedAccountType = String(
      selectedAccount.type || "",
    ).toUpperCase();
    const isValidForPaymentMethod =
      (watchPaymentMethodType === "CASH" && normalizedAccountType === "CASH") ||
      ((watchPaymentMethodType === "BANK" ||
        watchPaymentMethodType === "CREDIT") &&
        normalizedAccountType === "BANK");

    if (!isValidForPaymentMethod) {
      setValue("cash_bank_id", undefined, {
        shouldDirty: true,
        shouldValidate: true,
      });
    }
  }, [watchPaymentMethodType, watchCashBankId, cashBankAccounts, setValue]);

  // Ensure modal body is scrollable when content overflows
  useEffect(() => {
    if (isOpen) {
      // Small delay to ensure DOM is ready
      setTimeout(() => {
        const modalBody = modalBodyRef.current;
        if (modalBody) {
          const isOverflowing = modalBody.scrollHeight > modalBody.clientHeight;
          if (isOverflowing) {
            modalBody.style.overflowY = "scroll";
          }
        }
      }, 100);
    }
  }, [isOpen, fields.length]);

  const loadFormData = async () => {
    if (!token) {
      toast({
        title: t("sales.messages.authRequired"),
        description: t("sales.messages.pleaseLogin"),
        status: "error",
        duration: 5000,
        isClosable: true,
      });
      return;
    }

    setLoadingData(true);

    try {
      console.log("SalesForm: Starting to load form data...");

      // Load all data concurrently with proper error handling
      const [
        customersResult,
        productsResult,
        salesPersonsResult,
        accountsResult,
        assetAccountsResult,
        cashBankResult,
        projectsResult,
        invoiceTypesResult,
        settingsResult,
      ] = await Promise.allSettled([
        // Load customers
        (async () => {
          console.log("SalesForm: Loading customers...");
          const contactService = await import("@/services/contactService");
          const result = await contactService.default.getContacts(
            token,
            "CUSTOMER",
          );
          console.log("SalesForm: Customers loaded:", result?.length || 0);
          return result;
        })(),

        // Load products with fallback for permission errors
        (async () => {
          try {
            console.log("SalesForm: Loading products with token...");
            const productService = await import("@/services/productService");
            const result = await productService.default.getProducts({}, token);
            console.log(
              "SalesForm: Products loaded:",
              result?.data?.length || 0,
            );
            return result;
          } catch (error: any) {
            console.warn(
              "SalesForm: Failed to load products, using empty list:",
              error?.message || error,
            );
            // Return empty result for any error
            return { data: [] }; // Empty products array
          }
        })(),

        // Load sales persons from contacts (employees)
        (async () => {
          console.log("SalesForm: Loading sales persons (employees)...");
          const contactService = await import("@/services/contactService");
          const result = await contactService.default.getContacts(
            token,
            "EMPLOYEE",
          );
          console.log("SalesForm: Sales persons loaded:", result?.length || 0);
          return result;
        })(),

        // Load revenue accounts (catalog is enough for selector/default COA)
        (async () => {
          console.log("SalesForm: Loading revenue accounts...");
          const accountService = await import("@/services/accountService");
          const result = await accountService.default.getAccountCatalog(
            token,
            "REVENUE",
          );
          console.log(
            "SalesForm: Revenue accounts loaded:",
            result?.length || 0,
          );
          return result;
        })(),

        // Load asset accounts for credit receivable COA selection
        (async () => {
          console.log("SalesForm: Loading asset accounts for receivables...");
          const accountService = await import("@/services/accountService");
          const result = await accountService.default.getAccountCatalog(
            token,
            "ASSET",
          );
          console.log(
            "SalesForm: Asset accounts loaded:",
            result?.length || 0,
          );
          return result;
        })(),

        // Load cash & bank accounts for payment method selection
        (async () => {
          console.log("SalesForm: Loading cash & bank accounts...");
          const cashBankService = await import("@/services/cashbankService");
          const result = await cashBankService.default.getCashBankAccounts();
          console.log(
            "SalesForm: Cash & bank accounts loaded:",
            result?.length || 0,
          );
          return result;
        })(),

        // Load projects for optional project tagging
        (async () => {
          console.log("SalesForm: Loading projects...");
          const { projectService } = await import("@/services/projectService");
          const result = await projectService.getAll();
          console.log("SalesForm: Projects loaded:", result?.length || 0);
          return result;
        })(),

        // Load invoice types
        (async () => {
          console.log("SalesForm: Loading invoice types...");
          const result = await invoiceTypeService.getActiveInvoiceTypes();
          console.log("SalesForm: Invoice types loaded:", result?.length || 0);
          return result;
        })(),

        // Load system settings for default tax rate
        (async () => {
          console.log("SalesForm: Loading system settings...");
          const api = (await import("@/services/api")).default;
          const response = await api.get("/api/v1/settings/public");
          console.log("SalesForm: System settings loaded");
          return response.data;
        })(),
      ]);

      // Process customers
      if (
        customersResult.status === "fulfilled" &&
        Array.isArray(customersResult.value)
      ) {
        setCustomers(customersResult.value);
      } else {
        console.warn(
          "Failed to load customers:",
          customersResult.status === "rejected"
            ? customersResult.reason
            : "No data",
        );
        setCustomers([]);
      }

      // Process products
      if (
        productsResult.status === "fulfilled" &&
        productsResult.value?.data &&
        Array.isArray(productsResult.value.data)
      ) {
        setProducts(productsResult.value.data);
      } else {
        console.warn(
          "Failed to load products:",
          productsResult.status === "rejected"
            ? productsResult.reason
            : "No data",
        );
        setProducts([]);
      }

      // Process sales persons
      if (
        salesPersonsResult.status === "fulfilled" &&
        Array.isArray(salesPersonsResult.value)
      ) {
        const salesPersonsData = salesPersonsResult.value.map((contact) => ({
          ...contact,
          name: contact.name || contact.company_name || "Unknown Employee",
        }));
        setSalesPersons(salesPersonsData);
      } else {
        console.warn(
          "Failed to load sales persons:",
          salesPersonsResult.status === "rejected"
            ? salesPersonsResult.reason
            : "No data",
        );
        setSalesPersons([]);
      }

      // Process accounts - catalog already returns lightweight detail rows for selector/default COA.
      if (
        accountsResult.status === "fulfilled" &&
        Array.isArray(accountsResult.value)
      ) {
        const revenueAccounts = filterActiveLeafAccounts(accountsResult.value)
          .map((acc: { id: number; code: string; name: string }) => ({
            id: acc.id,
            code: acc.code,
            name: acc.name,
          }));
        console.log(
          "SalesForm: Revenue accounts loaded:",
          revenueAccounts.length,
        );
        setAccounts(revenueAccounts);
        if (revenueAccounts.length > 0) {
          setDefaultRevenueAccountId(revenueAccounts[0].id);
          setDefaultRevenueAccountLabel(
            `${revenueAccounts[0].code} - ${revenueAccounts[0].name}`,
          );
        } else {
          setDefaultRevenueAccountId(null);
          setDefaultRevenueAccountLabel("");
        }
      } else {
        console.warn(
          "Failed to load accounts:",
          accountsResult.status === "rejected"
            ? accountsResult.reason
            : "No data",
        );
        console.log("SalesForm: AccountsResult details:", accountsResult);
        setAccounts([]);
        setDefaultRevenueAccountId(null);
        setDefaultRevenueAccountLabel("");
      }

      if (
        assetAccountsResult.status === "fulfilled" &&
        Array.isArray(assetAccountsResult.value)
      ) {
        const receivableOptions = filterActiveLeafAccounts(
          assetAccountsResult.value as AccountCatalogRow[],
        )
          .filter(isReceivableAccountCandidate)
          .map((acc) => ({
            id: acc.id,
            code: acc.code,
            name: acc.name,
            active: acc.active,
            parent_id: acc.parent_id,
            parent_code: acc.parent_code,
            parent_name: acc.parent_name,
            is_active: acc.is_active,
            is_header: acc.is_header,
          }));
        setReceivableAccounts(receivableOptions);
      } else {
        console.warn(
          "Failed to load asset accounts:",
          assetAccountsResult.status === "rejected"
            ? assetAccountsResult.reason
            : "No data",
        );
        setReceivableAccounts([]);
      }

      // Process cash & bank accounts
      if (
        cashBankResult.status === "fulfilled" &&
        Array.isArray(cashBankResult.value)
      ) {
        setCashBankAccounts(
          filterOperationalCashBankAccounts(cashBankResult.value),
        );
      } else {
        console.warn(
          "Failed to load cash & bank accounts:",
          cashBankResult.status === "rejected"
            ? cashBankResult.reason
            : "No data",
        );
        setCashBankAccounts([]);
      }

      // Process projects
      if (
        projectsResult.status === "fulfilled" &&
        Array.isArray(projectsResult.value)
      ) {
        setProjects(projectsResult.value);
      } else {
        console.warn(
          "Failed to load projects:",
          projectsResult.status === "rejected"
            ? projectsResult.reason
            : "No data",
        );
        setProjects([]);
      }

      // Process invoice types
      if (
        invoiceTypesResult.status === "fulfilled" &&
        Array.isArray(invoiceTypesResult.value)
      ) {
        setInvoiceTypes(invoiceTypesResult.value);
      } else {
        console.warn(
          "Failed to load invoice types:",
          invoiceTypesResult.status === "rejected"
            ? invoiceTypesResult.reason
            : "No data",
        );
        setInvoiceTypes([]);
      }

      // Process system settings — extract default tax rate
      if (settingsResult.status === "fulfilled" && settingsResult.value?.data) {
        const settingsData = settingsResult.value.data;
        const configuredTaxRate = settingsData.default_tax_rate ?? 0;
        console.log(
          "SalesForm: Using default tax rate from settings:",
          configuredTaxRate,
        );
        setDefaultTaxRate(configuredTaxRate);
        // Apply to form if creating new sale (no existing sale/initialData)
        if (!sale && !initialData) {
          setValue("ppn_rate", configuredTaxRate);
          setValue("ppn_percent", configuredTaxRate);
        }
      } else {
        console.warn(
          "Failed to load system settings, using default tax rate: 0",
        );
        setDefaultTaxRate(0);
      }
    } catch (error: any) {
      console.error("Error loading form data:", error);
      toast({
        title: t("sales.messages.loadingError"),
        description: t("sales.messages.failedToLoadData"),
        status: "error",
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setLoadingData(false);
    }
  };

  const populateFormWithSale = (saleData: Sale) => {
    console.log("📝 Populating form with sale data:", saleData);

    const formValues: FormData = {
      customer_id: saleData.customer_id,
      sales_person_id: saleData.sales_person_id || undefined,
      project_id: saleData.project_id || undefined,
      invoice_type_id: saleData.invoice_type_id || undefined,
      type: saleData.type || "INVOICE",
      date: saleData.date
        ? saleData.date.split("T")[0]
        : new Date().toISOString().split("T")[0],
      due_date: saleData.due_date ? saleData.due_date.split("T")[0] : undefined,
      valid_until: saleData.valid_until
        ? saleData.valid_until.split("T")[0]
        : undefined,
      currency: saleData.currency || "IDR",
      exchange_rate: saleData.exchange_rate || 1,
      discount_percent: saleData.discount_percent || 0,
      // Legacy tax fields
      ppn_percent: saleData.ppn_percent || 0,
      pph_percent: saleData.pph_percent || 0,
      pph_type: saleData.pph_type || undefined,
      // Enhanced tax configuration
      ppn_rate: saleData.ppn_rate ?? saleData.ppn_percent ?? 0,
      other_tax_additions: saleData.other_tax_additions || 0,
      pph21_rate: saleData.pph21_rate || 0,
      pph23_rate: saleData.pph23_rate || 0,
      other_tax_deductions: saleData.other_tax_deductions || 0,
      payment_terms: saleData.payment_terms || "NET_30",
      payment_schedules:
        saleData.payment_schedules && saleData.payment_schedules.length > 0
          ? saleData.payment_schedules.map((schedule) => ({
              sequence: schedule.sequence,
              percentage: schedule.percentage || 0,
              due_date: schedule.due_date
                ? schedule.due_date.split("T")[0]
                : "",
              description: schedule.description || "",
            }))
          : [],
      payment_method: saleData.payment_method || undefined,
      payment_method_type:
        (saleData.payment_method_type as FormData["payment_method_type"]) ||
        "CREDIT",
      cash_bank_id: saleData.cash_bank_id || undefined,
      receivable_account_id:
        String(saleData.payment_method_type || "").toUpperCase() === "CREDIT"
          ? saleData.receivable_account_id || undefined
          : undefined,
      shipping_method: saleData.shipping_method || undefined,
      shipping_cost: saleData.shipping_cost || 0,
      shipping_taxable: saleData.shipping_taxable ?? true,
      billing_address: saleData.billing_address || undefined,
      shipping_address: saleData.shipping_address || undefined,
      notes: saleData.notes || undefined,
      internal_notes: saleData.internal_notes || undefined,
      reference: saleData.reference || undefined,
      items:
        saleData.sale_items && saleData.sale_items.length > 0
          ? saleData.sale_items.map((item) => ({
              id: item.id,
              product_id: item.product_id || 0,
              product_name: item.product_name || item.product?.name || "",
              description: item.description || "",
              location: item.location || "",
              quantity: item.quantity || 1,
              unit_price: item.unit_price || 0,
              estimated_unit_cost: item.estimated_unit_cost || 0,
              discount_percent: item.discount_percent || 0,
              taxable: item.taxable !== undefined ? item.taxable : true,
              revenue_account_id: item.revenue_account_id || undefined,
            }))
          : [
              {
                product_id: 0,
                product_name: "",
                description: "",
                location: "",
                quantity: 1,
                unit_price: 0,
                discount_percent: 0,
                taxable: true,
              },
            ],
    };

    setShippingTreatment(
      resolveShippingTreatment(
        formValues.shipping_cost,
        formValues.shipping_taxable,
      ),
    );
    reset(formValues);

    console.log("✅ Form populated successfully");
  };

  const resetForm = () => {
    reset({
      type: "INVOICE",
      date: new Date().toISOString().split("T")[0],
      currency: "IDR",
      exchange_rate: 1,
      discount_percent: 0,
      // Legacy tax fields — use system configured rate
      ppn_percent: defaultTaxRate,
      pph_percent: 0,
      // Enhanced tax configuration — use system configured rate
      ppn_rate: defaultTaxRate,
      other_tax_additions: 0,
      pph21_rate: 0,
      pph23_rate: 0,
      other_tax_deductions: 0,
      payment_terms: "NET_30",
      payment_schedules: [],
      payment_method_type: "CREDIT" as "CASH" | "BANK" | "CREDIT",
      receivable_account_id: undefined,
      shipping_cost: 0,
      shipping_taxable: true,
      items: [
        {
          product_id: 0,
          product_name: "",
          description: "",
          location: "",
          quantity: 1,
          unit_price: 0,
          estimated_unit_cost: 0,
          discount_percent: 0,
          taxable: true,
        },
      ],
    });
    setShippingTreatment("buyer_taxable");
  };

  const handleProductChange = async (index: number, productId: number) => {
    clearErrors([
      `items.${index}.product_id`,
      `items.${index}.product_name`,
      `items.${index}.description`,
      `items.${index}.unit_price`,
      `items.${index}.revenue_account_id`,
    ]);
    const product = products.find(
      (p) => p.id === parseInt(productId.toString()),
    );
    if (product) {
      setValue(`items.${index}.product_id`, product.id);
      setValue(`items.${index}.product_name`, product.name);
      setValue(`items.${index}.description`, product.name);
      setValue(`items.${index}.unit_price`, product.price);
      setValue(
        `items.${index}.estimated_unit_cost`,
        Number(product.cost_price || product.purchase_price || 0),
      );

      // --- Auto-Kitting Validation Logic ---
      if (product.is_assembly) {
        try {
          const { bomService } = await import("@/services/bomService");
          const response = await bomService.getBOMs({
            target_product_id: product.id,
          });

          if (response.success && response.data && response.data.length > 0) {
            const bom = response.data[0];
            const insufficientComponents: string[] = [];

            bom.items?.forEach((item) => {
              const comp = item.component_product;
              // Calculate required quantity for 1 unit of assembly
              const qtyNeeded =
                item.requirement_quantity / (bom.target_quantity || 1);
              if (comp && comp.stock < qtyNeeded) {
                insufficientComponents.push(
                  `- ${comp.name} (Stok: ${comp.stock}, Butuh: ${qtyNeeded.toFixed(2)})`,
                );
              }
            });

            if (insufficientComponents.length > 0) {
              toast({
                title: "Stok Bahan Baku Tidak Cukup (Auto-Kitting)",
                description: `Bahan baku untuk merakit 1 ${product.name} tidak mencukupi:\n${insufficientComponents.join("\n")}`,
                status: "error",
                duration: 8000,
                isClosable: true,
                position: "top-right",
              });
            } else {
              toast({
                title: "Stok Bahan Baku Tersedia",
                description: `Bahan baku untuk merakit ${product.name} mencukupi.`,
                status: "success",
                duration: 3000,
                isClosable: true,
                position: "top-right",
              });
            }
          }
        } catch (error) {
          console.error("Failed to check assembly component stock", error);
        }
      }
      // --- End Validation Logic ---
    } else {
      // When manual entry is selected (productId = 0), clear the fields
      setValue(`items.${index}.product_id`, 0);
      setValue(`items.${index}.product_name`, "");
      setValue(`items.${index}.description`, "");
      setValue(`items.${index}.unit_price`, 0);
      setValue(`items.${index}.estimated_unit_cost`, 0);
    }
  };

  // Helper function to check if item is manual entry
  const isManualEntry = (index: number) => {
    return (
      !watchItems[index]?.product_id || watchItems[index]?.product_id === 0
    );
  };

  const getItemProductName = (index: number) => {
    return watchItems[index]?.product_name?.trim() || "";
  };

  const getItemFulfillmentPreview = (index: number) => {
    const currentItem = watchItems[index];
    if (!currentItem || !currentItem.product_id) {
      return null;
    }

    const product = products.find((p) => p.id === currentItem.product_id);
    if (!product || product.is_service || product.is_assembly) {
      return null;
    }

    const requestedQty = Math.max(
      0,
      Math.floor(Number(currentItem.quantity) || 0),
    );
    if (requestedQty <= 0) {
      return null;
    }

    const requestedBefore = watchItems
      .slice(0, index)
      .reduce((sum, item) => {
        if (item.product_id !== currentItem.product_id) {
          return sum;
        }
        return sum + Math.max(0, Math.floor(Number(item.quantity) || 0));
      }, 0);

    const remainingBefore = Math.max(
      0,
      Number(product.stock || 0) - requestedBefore,
    );
    const availableNow = Math.min(requestedQty, remainingBefore);
    const futureFulfillmentQty = Math.max(0, requestedQty - availableNow);

    return {
      availableNow,
      futureFulfillmentQty,
      estimatedUnitCost:
        Number(currentItem.estimated_unit_cost || 0) ||
        Number(product.cost_price || product.purchase_price || 0),
      unit: product.unit || "qty",
    };
  };

  const calculateLineTotal = (item: any) => {
    const quantity = Number(item?.quantity) || 0;
    const unitPrice = Number(item?.unit_price) || 0;
    const discountPercent = Number(item?.discount_percent) || 0;
    const subtotal = quantity * unitPrice;
    const discountAmount = subtotal * (discountPercent / 100);
    return subtotal - discountAmount;
  };

  const calculateSubtotal = () => {
    return watchItems.reduce((sum, item) => sum + calculateLineTotal(item), 0);
  };

  // Calculate subtotal for taxable items only
  const calculateTaxableSubtotal = () => {
    return watchItems.reduce((sum, item) => {
      if (item.taxable !== false) {
        // Default to true if undefined
        return sum + calculateLineTotal(item);
      }
      return sum;
    }, 0);
  };

  // Calculate subtotal for non-taxable items
  const calculateNonTaxableSubtotal = () => {
    return watchItems.reduce((sum, item) => {
      if (item.taxable === false) {
        return sum + calculateLineTotal(item);
      }
      return sum;
    }, 0);
  };

  const calculateTotal = () => {
    const subtotal = calculateSubtotal();
    const discountPercent =
      discountType === "percentage"
        ? watchDiscountPercent
        : subtotal > 0
          ? (Math.min(watchDiscountPercent || 0, subtotal) / subtotal) * 100
          : 0;
    return calculateSalesAmounts({
      items: watchItems,
      discountPercent,
      ppnRate: watchPPNRate ?? watchPPNPercent ?? defaultTaxRate,
      pph21Rate: watchPPh21Rate,
      pph23Rate: watchPPh23Rate,
      otherTaxAdditions: watchOtherTaxAdditions,
      otherTaxDeductions: watchOtherTaxDeductions,
      shippingCost: watchShippingCost,
      shippingTaxable: watchShippingTaxable,
    }).totalAmount;
  };

  const handleShippingTreatmentChange = (treatment: ShippingTreatment) => {
    setShippingTreatment(treatment);
    setValue("shipping_taxable", treatment === "buyer_taxable", {
      shouldDirty: true,
      shouldValidate: true,
    });

    if (treatment === "buyer_direct") {
      setValue("shipping_cost", 0, {
        shouldDirty: true,
        shouldValidate: true,
      });
    }
  };

  const getPaymentSchedulePercentageTotal = () => {
    return (watchPaymentSchedules || []).reduce((sum, schedule) => {
      return sum + (Number(schedule?.percentage) || 0);
    }, 0);
  };

  const getScheduleAmount = (percentage: number, index: number) => {
    const total = calculateTotal();
    const schedules = watchPaymentSchedules || [];
    const sanitizedPercentage = Number(percentage) || 0;

    if (index === schedules.length - 1) {
      const previousAmount = schedules
        .slice(0, index)
        .reduce((sum, schedule) => {
          return sum + total * ((Number(schedule?.percentage) || 0) / 100);
        }, 0);
      return Math.max(0, total - previousAmount);
    }

    return total * (sanitizedPercentage / 100);
  };

  const getLastCustomScheduleDueDate = () => {
    const schedules = (watchPaymentSchedules || []).filter(
      (schedule) => !!schedule?.due_date,
    );
    if (schedules.length === 0) {
      return null;
    }

    return schedules
      .map((schedule) => schedule.due_date)
      .sort()
      .slice(-1)[0];
  };

  // Helper function to get global discount amount for display
  const getGlobalDiscountAmount = () => {
    const subtotal = calculateSubtotal();
    return discountType === "percentage"
      ? subtotal * (watchDiscountPercent / 100)
      : Math.min(watchDiscountPercent || 0, subtotal);
  };

  // Helper function to calculate due date from payment terms
  const calculateDueDateFromPaymentTerms = (
    date: string,
    terms: string,
  ): string | null => {
    if (!date || terms === "COD" || terms === "CUSTOM") return null;

    const baseDate = new Date(date);
    let days = 0;

    switch (terms) {
      case "NET_15":
        days = 15;
        break;
      case "NET_30":
        days = 30;
        break;
      case "NET_60":
        days = 60;
        break;
      case "NET_90":
        days = 90;
        break;
      default:
        return null;
    }

    const dueDate = new Date(baseDate);
    dueDate.setDate(dueDate.getDate() + days);
    return dueDate.toISOString().split("T")[0];
  };

  // Helper function to get payment terms explanation
  const getPaymentTermsExplanation = (terms: string): string => {
    switch (terms) {
      case "COD":
        return "Customer pays immediately upon delivery";
      case "NET_15":
        return "Customer has 15 days from invoice date to pay";
      case "NET_30":
        return "Customer has 30 days from invoice date to pay";
      case "NET_60":
        return "Customer has 60 days from invoice date to pay";
      case "NET_90":
        return "Customer has 90 days from invoice date to pay";
      case "CUSTOM":
        return "Customer pays based on the custom payment schedule shown below";
      default:
        return "";
    }
  };

  // Helper function to get field labels based on transaction type
  const getDateFieldLabel = (type: string): string => {
    switch (type) {
      case "QUOTE":
      case "QUOTATION":
        return t("sales.form.quoteDate");
      case "INVOICE":
        return t("sales.form.invoiceDate");
      case "SALES_ORDER":
        return t("sales.form.orderDate");
      default:
        return t("sales.form.transactionDate");
    }
  };

  // Check if due date is auto-calculated
  const isDueDateAutoCalculated =
    watchPaymentTerms &&
    watchPaymentTerms !== "CUSTOM" &&
    watchPaymentTerms !== "COD";

  const addItem = () => {
    append({
      product_id: 0,
      product_name: "",
      description: "",
      location: "",
      quantity: 1,
      unit_price: 0,
      estimated_unit_cost: 0,
      discount_percent: 0,
      taxable: true,
    });
  };

  const removeItem = (index: number) => {
    if (fields.length > 1) {
      remove(index);
    }
  };

  const addPaymentSchedule = () => {
    appendSchedule({
      sequence: scheduleFields.length + 1,
      percentage: 0,
      due_date: watchDate || "",
      description: "",
    });
  };

  const removePaymentSchedule = (index: number) => {
    removeSchedule(index);
  };

  const onSubmit = async (data: FormData) => {
    try {
      setLoading(true);

      // For invoiced sales, skip item validation (items are locked)
      let validItems: typeof data.items = [];
      if (!isInvoiced) {
        clearErrors("items");

        let hasItemFieldError = false;
        (data.items || []).forEach((item, index) => {
          const manualName = item.product_name?.trim() || "";
          const description = item.description?.trim() || "";
          const hasSelectedProduct = item.product_id > 0;
          const rowStarted =
            hasSelectedProduct ||
            manualName !== "" ||
            description !== "" ||
            (item.unit_price || 0) > 0 ||
            (item.revenue_account_id || 0) > 0;

          if (!rowStarted) {
            return;
          }

          if (!hasSelectedProduct && !manualName && !description) {
            setError(`items.${index}.product_id`, {
              type: "manual",
              message: "Pilih product atau isi nama/deskripsi item manual",
            });
            hasItemFieldError = true;
          }

          if (!item.quantity || item.quantity <= 0) {
            setError(`items.${index}.quantity`, {
              type: "manual",
              message: "Quantity minimal 1",
            });
            hasItemFieldError = true;
          }

          if (!item.unit_price || item.unit_price <= 0) {
            setError(`items.${index}.unit_price`, {
              type: "manual",
              message: "Unit price harus lebih besar dari 0",
            });
            hasItemFieldError = true;
          }

          if (
            canSelectRevenueAccountFromCOA &&
            (!item.revenue_account_id || item.revenue_account_id <= 0)
          ) {
            setError(`items.${index}.revenue_account_id`, {
              type: "manual",
              message: "Pilih COA / Revenue Account untuk item ini",
            });
            hasItemFieldError = true;
          }
        });

        if (hasItemFieldError) {
          ErrorHandler.handleValidationError(
            ["Lengkapi field item yang masih kosong sebelum menyimpan sales"],
            toast,
            "sales form",
          );
          return;
        }

        // Validate items - allow either catalog products or manual product names
        validItems = data.items.filter((item) => {
          const hasSelectedProduct = item.product_id > 0;
          const hasManualProductName = !!item.product_name?.trim();
          const hasDescription = !!item.description?.trim();
          return (
            (hasSelectedProduct || hasManualProductName || hasDescription) &&
            item.unit_price >= 0
          );
        });

        if (validItems.length === 0) {
          const errorMsg =
            products.length > 0
              ? t("sales.messages.atLeastOneItem")
              : t("sales.messages.atLeastOneItemManual");
          ErrorHandler.handleValidationError([errorMsg], toast, "sales form");
          return;
        }

        // Additional validation
        const validationErrors = salesService.validateSaleData({
          ...data,
          receivable_account_id:
            data.payment_method_type === "CREDIT"
              ? resolveReceivableAccountId(data.receivable_account_id)
              : undefined,
          items: validItems.map((item) => ({
            product_id: item.product_id,
            product_name: item.product_name?.trim() || "",
            description: item.description || "",
            quantity: Math.max(1, Math.floor(item.quantity || 1)),
            unit_price: Math.min(
              999999999999.99,
              Math.max(0, item.unit_price || 0),
            ),
            estimated_unit_cost: Math.min(
              999999999999.99,
              Math.max(0, item.estimated_unit_cost || 0),
            ),
            discount: Math.min(
              999999.99,
              Math.max(0, item.discount_percent || 0),
            ),
            discount_percent: Math.min(
              100,
              Math.max(0, item.discount_percent || 0),
            ),
            tax: 0,
            taxable: item.taxable !== false,
            revenue_account_id: resolveRevenueAccountId(item),
          })),
        });

        if (validationErrors.length > 0) {
          ErrorHandler.handleValidationError(
            validationErrors,
            toast,
            "sales form",
          );
          return;
        }
      }

      const itemSubtotal = validItems.reduce(
        (sum, item) => sum + calculateLineTotal(item),
        0,
      );
      const normalizedDiscountPercent =
        discountType === "percentage"
          ? data.discount_percent
          : itemSubtotal > 0
            ? (Math.min(data.discount_percent || 0, itemSubtotal) / itemSubtotal) *
              100
            : 0;

      const receivableAccountId = resolveReceivableAccountId(
        data.receivable_account_id,
      );
      if (
        data.payment_method_type === "CREDIT" &&
        (!receivableAccountId || receivableAccountId <= 0)
      ) {
        setError("receivable_account_id", {
          type: "manual",
          message: "Pilih COA piutang untuk penjualan kredit",
        });
        ErrorHandler.handleValidationError(
          ["Pilih COA piutang untuk penjualan kredit"],
          toast,
          "sales form",
        );
        return;
      }

      const validPaymentSchedules: SalePaymentScheduleRequest[] | undefined =
        data.payment_terms === "CUSTOM"
          ? (data.payment_schedules || [])
              .filter(
                (schedule) =>
                  (Number(schedule.percentage) || 0) > 0 ||
                  !!schedule.due_date ||
                  !!schedule.description?.trim(),
              )
              .map((schedule, index) => ({
                sequence: index + 1,
                percentage: Number(schedule.percentage) || 0,
                due_date: schedule.due_date
                  ? `${schedule.due_date}T00:00:00Z`
                  : "",
                description: schedule.description?.trim() || "",
              }))
          : [];

      if (data.payment_terms === "CUSTOM") {
        if (data.payment_method_type !== "CREDIT") {
          ErrorHandler.handleValidationError(
            [
              "Termin custom hanya boleh dipakai untuk penjualan kredit/piutang",
            ],
            toast,
            "sales form",
          );
          return;
        }

        if (!validPaymentSchedules || validPaymentSchedules.length === 0) {
          ErrorHandler.handleValidationError(
            [
              "Minimal satu termin pembayaran wajib diisi untuk payment terms custom",
            ],
            toast,
            "sales form",
          );
          return;
        }

        const invalidSchedule = validPaymentSchedules.find(
          (schedule, index) => {
            return schedule.percentage <= 0 || !schedule.due_date;
          },
        );
        if (invalidSchedule) {
          ErrorHandler.handleValidationError(
            [
              "Setiap termin custom wajib memiliki persentase lebih dari 0 dan tanggal jatuh tempo",
            ],
            toast,
            "sales form",
          );
          return;
        }

        const percentageTotal = validPaymentSchedules.reduce(
          (sum, schedule) => sum + schedule.percentage,
          0,
        );
        if (Math.abs(percentageTotal - 100) > 0.01) {
          ErrorHandler.handleValidationError(
            [
              `Total persentase termin harus 100%. Saat ini ${percentageTotal.toFixed(2)}%`,
            ],
            toast,
            "sales form",
          );
          return;
        }
      }

      if (sale) {
        // Update existing sale
        let updateData: SaleUpdateRequest;

        if (isInvoiced) {
          // Partial edit: only send non-financial fields for invoiced/overdue sales
          updateData = {
            sales_person_id: data.sales_person_id,
            project_id: data.project_id || 0,
            due_date: data.due_date ? `${data.due_date}T00:00:00Z` : undefined,
            payment_terms: data.payment_terms,
            payment_schedules: validPaymentSchedules,
            payment_method: data.payment_method,
            payment_method_type: data.payment_method_type,
            cash_bank_id: data.cash_bank_id,
            shipping_method: data.shipping_method,
            billing_address: data.billing_address,
            shipping_address: data.shipping_address,
            notes: data.notes,
            internal_notes: data.internal_notes,
            reference: data.reference,
          };
        } else {
          // Full edit: send all fields for DRAFT sales
          updateData = {
            customer_id: data.customer_id,
            sales_person_id: data.sales_person_id,
            project_id: data.project_id || 0,
            date: data.date ? `${data.date}T00:00:00Z` : undefined,
            due_date: data.due_date ? `${data.due_date}T00:00:00Z` : undefined,
            valid_until: data.valid_until
              ? `${data.valid_until}T00:00:00Z`
              : undefined,
            discount_percent: normalizedDiscountPercent,
            ppn_percent: data.ppn_percent,
            pph_percent: data.pph_percent,
            pph_type: data.pph_type,
            ppn_rate: data.ppn_rate,
            other_tax_additions: data.other_tax_additions,
            pph21_rate: data.pph21_rate,
            pph23_rate: data.pph23_rate,
            other_tax_deductions: data.other_tax_deductions,
            payment_terms: data.payment_terms,
            payment_schedules: validPaymentSchedules,
            payment_method: data.payment_method,
            payment_method_type: data.payment_method_type,
            cash_bank_id: data.cash_bank_id,
            receivable_account_id:
              data.payment_method_type === "CREDIT"
                ? receivableAccountId
                : undefined,
            shipping_method: data.shipping_method,
            shipping_cost: data.shipping_cost,
            shipping_taxable: data.shipping_taxable,
            billing_address: data.billing_address,
            shipping_address: data.shipping_address,
            notes: data.notes,
            internal_notes: data.internal_notes,
            reference: data.reference,
            items: validItems.map((item) => ({
              id: item.id,
              product_id: item.product_id,
              product_name: item.product_name?.trim() || "",
              description: item.description,
              location: item.location || "",
              quantity: item.quantity,
              unit_price: item.unit_price,
              estimated_unit_cost: Math.max(
                0,
                Number(item.estimated_unit_cost || 0),
              ),
              discount: item.discount_percent || 0,
              tax: 0,
              revenue_account_id: resolveRevenueAccountId(item),
              delete: item.delete || false,
            })),
          };
        }

        console.log("Sending updateData:", updateData);
        await salesService.updateSale(sale.id, updateData);
        ErrorHandler.handleSuccess(
          t("sales.messages.saleUpdated"),
          toast,
          "update sale",
        );
      } else {
        // Create new sale
        const createData: SaleCreateRequest = {
          customer_id: data.customer_id,
          sales_person_id: data.sales_person_id,
          project_id: data.project_id,
          invoice_type_id: data.invoice_type_id,
          type: data.type,
          date: `${data.date}T00:00:00Z`, // Convert to ISO datetime format for Go backend
          due_date: data.due_date ? `${data.due_date}T00:00:00Z` : undefined,
          valid_until: data.valid_until
            ? `${data.valid_until}T00:00:00Z`
            : undefined,
          currency: data.currency,
          exchange_rate: data.exchange_rate,
          discount_percent: normalizedDiscountPercent,
          // Legacy tax fields
          ppn_percent: data.ppn_percent,
          pph_percent: data.pph_percent,
          pph_type: data.pph_type,
          // Enhanced tax configuration
          ppn_rate: data.ppn_rate,
          other_tax_additions: data.other_tax_additions,
          pph21_rate: data.pph21_rate,
          pph23_rate: data.pph23_rate,
          other_tax_deductions: data.other_tax_deductions,
          payment_terms: data.payment_terms,
          payment_schedules: validPaymentSchedules,
          payment_method: data.payment_method,
          payment_method_type: data.payment_method_type,
          cash_bank_id: data.cash_bank_id,
          receivable_account_id:
            data.payment_method_type === "CREDIT"
              ? receivableAccountId
              : undefined,
          shipping_method: data.shipping_method,
          shipping_cost: data.shipping_cost,
          shipping_taxable: data.shipping_taxable,
          billing_address: data.billing_address,
          shipping_address: data.shipping_address,
          notes: data.notes,
          internal_notes: data.internal_notes,
          reference: data.reference,
          items: validItems.map((item) => ({
            product_id: item.product_id,
            product_name: item.product_name?.trim() || "",
            description: item.description || "",
            location: item.location || "",
            quantity: Math.max(1, Math.floor(item.quantity || 1)), // Ensure positive integer
            unit_price: Math.min(
              999999999999.99,
              Math.max(0, item.unit_price || 0),
            ), // Cap to prevent overflow
            estimated_unit_cost: Math.min(
              999999999999.99,
              Math.max(0, item.estimated_unit_cost || 0),
            ),
            discount: Math.min(
              999999.99,
              Math.max(0, item.discount_percent || 0),
            ), // Legacy field as flat amount
            discount_percent: Math.min(
              100,
              Math.max(0, item.discount_percent || 0),
            ), // New field as percentage
            tax: 0, // Tax will be calculated by backend based on taxable flag
            taxable: item.taxable !== false, // Default to true if not specified
            revenue_account_id: resolveRevenueAccountId(item),
          })),
        };

        console.log("Sending createData:", createData);
        await salesService.createSale(createData);
        ErrorHandler.handleSuccess(
          t("sales.messages.saleCreated"),
          toast,
          "create sale",
        );
      }

      // Clear any validation errors on success
      setValidationError(null);

      onSave();
      onClose();
    } catch (error: any) {
      console.error("SalesForm submission error:", error);

      // Check if it's a period closed error (400 - business validation)
      if (
        error?.response?.status === 400 &&
        error?.response?.data?.code === "PERIOD_CLOSED"
      ) {
        const errorData = error.response.data;
        toast({
          title: errorData.error || "Periode Sudah Ditutup",
          description:
            errorData.message ||
            "Tidak dapat membuat transaksi pada periode yang sudah ditutup.",
          status: "error",
          duration: 8000,
          isClosable: true,
          position: "top",
        });
        setLoading(false);
        return;
      }

      // Log detailed error information for debugging
      if (error.response) {
        console.error("Error response:", {
          status: error.response.status,
          data: error.response.data,
          headers: error.response.headers,
        });
        console.error("Raw error response data:", error.response.data);
        console.error("Error response status text:", error.response.statusText);
      } else {
        console.error("No response data available:", error.message);
      }

      // Clear any previous validation errors
      setValidationError(null);

      // Handle different error formats
      let errorObj = error;
      if (typeof error === "string") {
        errorObj = { message: error };
      } else if (!error) {
        errorObj = { message: "An unknown error occurred" };
      }

      // Parse the error for display
      let parsedError: ParsedValidationError;
      try {
        parsedError = ErrorHandler.parseValidationError(errorObj);
      } catch (parseError) {
        console.error("Error parsing validation error:", parseError);
        // Fallback error structure
        parsedError = {
          type: "unknown",
          title: "Error",
          message:
            errorObj?.response?.data?.error ||
            errorObj?.message ||
            "An unexpected error occurred",
          errors: [],
          canRetry: true,
          suggestions: ["Please check your input and try again"],
        };
      }

      console.log("Parsed error:", parsedError);

      // If it's a validation error, show the ErrorAlert instead of toast
      if (
        parsedError.type === "validation" ||
        parsedError.type === "business" ||
        error?.response?.status === 400
      ) {
        setValidationError(parsedError);

        // Scroll to the top so user can see the error
        if (modalBodyRef.current) {
          modalBodyRef.current.scrollTop = 0;
        }
      } else {
        // For other errors, continue using toast notifications
        ErrorHandler.handleSaveError("sale", errorObj, toast, !!sale);
      }
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    // Clear validation errors when closing
    setValidationError(null);
    reset();
    onClose();
  };

  if (shouldHideForm) {
    return null;
  }

  return (
    <>
      <Modal
        isOpen={isOpen}
        onClose={handleClose}
        size="full"
        isCentered
        closeOnOverlayClick={false}
        scrollBehavior="inside"
        motionPreset="slideInBottom"
        blockScrollOnMount={false}
      >
        <ModalOverlay
          bg="blackAlpha.700"
          backdropFilter="blur(4px)"
          onWheel={(e) => e.stopPropagation()}
        />
        <ModalContent
          maxH={{ base: "100vh", md: "95vh" }}
          minH={{ base: "100vh", md: "80vh" }}
          mx={{ base: 0, md: 4 }}
          my={{ base: 0, md: 2 }}
          borderRadius="xl"
          bg={modalBg}
          shadow="2xl"
          overflow="hidden"
          display="flex"
          flexDirection="column"
          w={{ base: "100vw", md: "96vw" }}
          maxW={{ base: "100vw", md: "1680px" }}
        >
          <ModalHeader
            bg={headerBg}
            borderBottomWidth={1}
            borderColor={borderColor}
            pb={4}
            pt={6}
          >
            <HStack justify="space-between" align="center">
              <Box>
                <Heading size="lg" color={headingColor}>
                  {sale ? t("sales.editSale") : t("sales.createSale")}
                </Heading>
                <Text color={textColor} fontSize="sm" mt={1}>
                  {sale
                    ? t("sales.manageTransactions")
                    : t("sales.manageTransactions")}
                </Text>
              </Box>
              <Badge
                colorScheme="blue"
                variant="solid"
                px={3}
                py={1}
                borderRadius="md"
              >
                <Icon as={FiShoppingCart} mr={1} />
                {t("sales.title")}
              </Badge>
            </HStack>
          </ModalHeader>
          <ModalCloseButton />

          <form
            onSubmit={handleSubmit(onSubmit)}
            style={{ height: "100%", display: "flex", flexDirection: "column" }}
          >
            {/* Hidden type field with default value */}
            <input type="hidden" {...register("type")} value="INVOICE" />

            <ModalBody
              ref={modalBodyRef}
              flex="1"
              overflowY="auto"
              px={6}
              py={4}
              pb={8}
              maxH="calc(95vh - 200px)"
              minH="400px"
              sx={{
                // Enable mouse wheel scrolling
                overscrollBehavior: "contain",
                WebkitOverflowScrolling: "touch",
                scrollBehavior: "smooth",

                // Force scrollbar to be visible
                overflowY: "scroll !important",

                // Custom scrollbar styles
                "&::-webkit-scrollbar": {
                  width: "8px",
                  display: "block",
                },
                "&::-webkit-scrollbar-track": {
                  background: scrollTrackBg,
                  borderRadius: "4px",
                },
                "&::-webkit-scrollbar-thumb": {
                  background: scrollThumbBg,
                  borderRadius: "4px",
                  "&:hover": {
                    background: scrollThumbHoverBg,
                  },
                },
              }}
              onWheel={(e) => {
                // Allow natural scroll behavior
                const target = e.currentTarget;
                const isScrollable = target.scrollHeight > target.clientHeight;
                if (isScrollable) {
                  // Let the natural scroll happen
                  return;
                }
                e.preventDefault();
                e.stopPropagation();
              }}
            >
              <VStack spacing={6} align="stretch">
                {/* Error Alert */}
                {validationError && (
                  <ErrorAlert
                    title={validationError.title}
                    message={validationError.message}
                    errors={validationError.errors}
                    type={
                      validationError.type === "validation"
                        ? "error"
                        : "warning"
                    }
                    onClose={() => setValidationError(null)}
                    dismissible={true}
                    className="mb-4"
                  />
                )}

                {/* Basic Information */}
                {/* Partial Edit Warning for Invoiced Sales */}
                {isInvoiced && (
                  <Alert status="info" borderRadius="md" mb={4}>
                    <AlertIcon />
                    <AlertDescription fontSize="sm">
                      ⚠️ Penjualan sudah difakturkan. Hanya field non-finansial
                      yang bisa diedit (jatuh tempo, payment terms, catatan,
                      alamat). Item, customer, diskon, dan pajak tidak bisa
                      diubah.
                    </AlertDescription>
                  </Alert>
                )}

                <Box>
                  <Heading size="md" mb={4} color={subHeadingColor}>
                    📋 {t("sales.form.basicInfo")}
                  </Heading>
                  <VStack spacing={4}>
                    <HStack w="full" spacing={4} align="flex-start">
                      <FormControl isRequired isInvalid={!!errors.customer_id}>
                        <FormLabel>{t("sales.customer")}</FormLabel>
                        <Select
                          {...register("customer_id", {
                            required: t("validation.required"),
                            setValueAs: (value) => parseInt(value) || 0,
                          })}
                          bg={inputBg}
                          _focus={{ bg: inputFocusBg }}
                          isDisabled={loadingData || isInvoiced}
                        >
                          <option value="">
                            {loadingData
                              ? t("common.loading")
                              : customers.length === 0
                                ? t("common.noData")
                                : t("sales.form.selectCustomer")}
                          </option>
                          {customers.map((customer) => (
                            <option key={customer.id} value={customer.id}>
                              {customer.code} - {customer.name}
                            </option>
                          ))}
                        </Select>
                        <FormErrorMessage>
                          {errors.customer_id?.message}
                        </FormErrorMessage>
                        <FormHelperText color={textColor}>
                          {t("sales.tooltips.customer")}
                        </FormHelperText>
                      </FormControl>

                      <FormControl>
                        <FormLabel>{t("sales.form.salesPerson")}</FormLabel>
                        <Select
                          {...register("sales_person_id", {
                            setValueAs: (value) =>
                              value ? parseInt(value) : undefined,
                          })}
                          bg={inputBg}
                          _focus={{ bg: inputFocusBg }}
                          isDisabled={loadingData}
                        >
                          <option value="">
                            {loadingData
                              ? t("common.loading")
                              : salesPersons.length === 0
                                ? t("common.noData")
                                : t("sales.form.selectSalesPerson")}
                          </option>
                          {salesPersons.map((person) => (
                            <option key={person.id} value={person.id}>
                              {person.name}
                            </option>
                          ))}
                        </Select>
                        <FormHelperText color={textColor}>
                          {t("sales.tooltips.salesPerson")}
                        </FormHelperText>
                      </FormControl>

                      <FormControl>
                        <FormLabel>Proyek (Opsional)</FormLabel>
                        <Select
                          {...register("project_id", {
                            setValueAs: (value) =>
                              value ? parseInt(value) : undefined,
                          })}
                          bg={inputBg}
                          _focus={{ bg: inputFocusBg }}
                          isDisabled={loadingData}
                        >
                          <option value="">
                            {loadingData
                              ? t("common.loading")
                              : projects.length === 0
                                ? t("common.noData")
                                : "Pilih Proyek"}
                          </option>
                          {projects.map((project) => (
                            <option key={project.id} value={project.id}>
                              {project.code} - {project.name}
                            </option>
                          ))}
                        </Select>
                        <FormHelperText color={textColor}>
                          Pilih proyek untuk mengaitkan penjualan ini
                        </FormHelperText>
                      </FormControl>

                      <FormControl>
                        <FormLabel>{t("sales.invoiceTypes")}</FormLabel>
                        <Select
                          {...register("invoice_type_id", {
                            setValueAs: (value) =>
                              value ? parseInt(value) : undefined,
                          })}
                          bg={inputBg}
                          _focus={{ bg: inputFocusBg }}
                          isDisabled={loadingData || isInvoiced}
                        >
                          <option value="">
                            {loadingData
                              ? t("common.loading")
                              : invoiceTypes.length === 0
                                ? t("common.noData")
                                : t("sales.form.selectInvoiceType")}
                          </option>
                          {invoiceTypes.map((invoiceType) => (
                            <option key={invoiceType.id} value={invoiceType.id}>
                              {invoiceType.name} ({invoiceType.code})
                            </option>
                          ))}
                        </Select>
                        <FormHelperText color={textColor}>
                          {t("sales.tooltips.invoiceType")}
                        </FormHelperText>
                      </FormControl>
                    </HStack>

                    {/* Display Invoice Number when editing */}
                    {sale && (
                      <FormControl isReadOnly>
                        <FormLabel>{t("sales.form.invoiceNumber")}</FormLabel>
                        <Input
                          value={sale.invoice_number || sale.code || "-"}
                          bg={inputBg}
                          isDisabled
                          fontWeight="bold"
                          isReadOnly
                        />
                        <FormHelperText color={textColor}>
                          {t("sales.tooltips.autoGeneratedId")}
                        </FormHelperText>
                      </FormControl>
                    )}

                    <HStack w="full" spacing={4}>
                      <FormControl isRequired isInvalid={!!errors.date}>
                        <FormLabel>{getDateFieldLabel(watchType)}</FormLabel>
                        <Input
                          type="date"
                          {...register("date", {
                            required: t("validation.required"),
                          })}
                          bg={inputBg}
                          _focus={{ bg: inputFocusBg }}
                        />
                        <FormErrorMessage>
                          {errors.date?.message}
                        </FormErrorMessage>
                        <FormHelperText color={textColor}>
                          {t("sales.tooltips.date")}
                        </FormHelperText>
                      </FormControl>

                      <FormControl>
                        <FormLabel>
                          {t("sales.dueDate")}
                          {isDueDateAutoCalculated && (
                            <Badge ml={2} colorScheme="green" size="sm">
                              {t("sales.form.autoCalculated")}
                            </Badge>
                          )}
                        </FormLabel>
                        <Input
                          type="date"
                          {...register("due_date")}
                          bg={isDueDateAutoCalculated ? readOnlyBg : inputBg}
                          color={baseFieldStyles.color}
                          borderColor={borderColor}
                          _hover={{ borderColor }}
                          _focusVisible={fieldFocusStyles}
                          _focus={{
                            bg: isDueDateAutoCalculated
                              ? readOnlyBg
                              : inputFocusBg,
                          }}
                          isReadOnly={Boolean(isDueDateAutoCalculated)}
                        />
                        <FormHelperText
                          color={
                            isDueDateAutoCalculated
                              ? successTextColor
                              : textColor
                          }
                        >
                          {isDueDateAutoCalculated
                            ? t("sales.form.dueDateAutoCalculated")
                            : t("sales.tooltips.dueDate")}
                        </FormHelperText>
                      </FormControl>

                      <FormControl>
                        <FormLabel>{t("sales.validUntil")}</FormLabel>
                        <Input
                          type="date"
                          {...register("valid_until")}
                          bg={inputBg}
                          _focus={{ bg: inputFocusBg }}
                        />
                        <FormHelperText color={textColor}>
                          {t("sales.tooltips.validUntil")}
                        </FormHelperText>
                      </FormControl>
                    </HStack>
                  </VStack>
                </Box>

                <Divider />

                {/* Items Section */}
                <Box>
                  <Flex justify="space-between" align="center" mb={4}>
                    <Heading size="md" color={subHeadingColor}>
                      🛍️ {t("sales.form.items")}
                    </Heading>
                    <Button
                      size="sm"
                      colorScheme="blue"
                      leftIcon={<FiPlus />}
                      onClick={addItem}
                      isDisabled={isInvoiced}
                    >
                      {t("sales.form.addItem")}
                    </Button>
                  </Flex>

                  {/* Products warning if empty */}
                  {!loadingData && products.length === 0 && (
                    <Alert
                      status="warning"
                      mb={4}
                      borderRadius="md"
                      bg={alertBg}
                      borderColor={alertBorderColor}
                    >
                      <AlertIcon />
                      <AlertDescription fontSize="sm">
                        {t("common.noData")}
                      </AlertDescription>
                    </Alert>
                  )}

                  {products.length > 0 && (
                    <Alert
                      status="info"
                      mb={4}
                      borderRadius="md"
                      bg={alertBg}
                      borderColor={alertBorderColor}
                    >
                      <AlertIcon />
                      <AlertDescription fontSize="sm">
                        Jika qty melebihi stok, sales tetap bisa disimpan tetapi
                        sistem akan menandainya sebagai future fulfillment.
                        Qty yang tersedia tetap siap saat invoicing, sisanya
                        menunggu pemenuhan stok/pembelian.
                      </AlertDescription>
                    </Alert>
                  )}

                  <Box
                    overflowX="auto"
                    border="1px"
                    borderColor={borderColor}
                    borderRadius="md"
                    bg={tableBg}
                    shadow="sm"
                    css={{
                      "&::-webkit-scrollbar": {
                        height: "8px",
                      },
                      "&::-webkit-scrollbar-track": {
                        background: scrollTrackBg,
                        borderRadius: "4px",
                      },
                      "&::-webkit-scrollbar-thumb": {
                        background: scrollThumbBg,
                        borderRadius: "4px",
                      },
                      "&::-webkit-scrollbar-thumb:hover": {
                        background: scrollThumbHoverBg,
                      },
                    }}
                  >
                    <Table variant="simple" size="sm" minW="1660px">
                      <Thead>
                        <Tr>
                          <Th minW="150px">{t("sales.form.product")}</Th>
                          <Th minW="170px">{t("sales.form.description")}</Th>
                          <Th minW="130px">{t("sales.form.location")}</Th>
                          <Th minW="120px">{t("sales.form.quantity")}</Th>
                          <Th minW="200px">{t("sales.form.unitPrice")}</Th>
                          <Th minW="180px">Est. HPP</Th>
                          <Th minW="130px">{t("sales.form.itemDiscount")}</Th>
                          <Th minW="95px">{t("sales.form.taxable")}</Th>
                          <Th minW="190px">{t("sales.form.revenueAccount")}</Th>
                          <Th minW="150px">{t("sales.form.lineTotal")}</Th>
                          <Th minW="70px" width="70px">{t("common.actions")}</Th>
                        </Tr>
                      </Thead>
                      <Tbody>
                        {fields.map((field, index) => (
                          <Tr key={field.id}>
                            <Td minW="150px">
                              <FormControl
                                isInvalid={
                                  !!errors.items?.[index]?.product_id ||
                                  !!errors.items?.[index]?.product_name
                                }
                              >
                                <VStack spacing={1} align="stretch">
                                  {!isManualEntry(index) && (
                                    <Input
                                      type="hidden"
                                      {...register(
                                        `items.${index}.product_name`,
                                      )}
                                    />
                                  )}
                                  <Select
                                    size="sm"
                                    {...register(`items.${index}.product_id`, {
                                      setValueAs: (value) =>
                                        parseInt(value) || 0,
                                      validate: (value) => {
                                        const manualName =
                                          getItemProductName(index);
                                        const description =
                                          watchItems?.[
                                            index
                                          ]?.description?.trim() || "";
                                        return (
                                          value > 0 ||
                                          !!manualName ||
                                          !!description ||
                                          "Pilih product atau isi item manual"
                                        );
                                      },
                                    })}
                                    onChange={(e) =>
                                      handleProductChange(
                                        index,
                                        parseInt(e.target.value),
                                      )
                                    }
                                    {...baseFieldStyles}
                                    placeholder={t(
                                      "common.placeholders.selectProduct",
                                    )}
                                    isDisabled={isInvoiced}
                                  >
                                    <option value="">
                                      {t("sales.form.manualEntry")}
                                    </option>
                                    {products.map((product) => (
                                      <option
                                        key={product.id}
                                        value={product.id}
                                      >
                                        {product.code} - {product.name} ({product.stock ?? 0} {product.unit})
                                      </option>
                                    ))}
                                  </Select>
                                  {isManualEntry(index) && (
                                    <>
                                      <Badge colorScheme="orange" size="xs">
                                        {t("sales.form.manualEntry")}
                                      </Badge>
                                      <Input
                                        size="sm"
                                        {...register(
                                          `items.${index}.product_name`,
                                          {
                                            validate: (value) => {
                                              if (!isManualEntry(index))
                                                return true;
                                              return (
                                                !!value?.trim() ||
                                                t("validation.required")
                                              );
                                            },
                                            onChange: () =>
                                              clearErrors([
                                                `items.${index}.product_id`,
                                                `items.${index}.product_name`,
                                              ]),
                                          },
                                        )}
                                        placeholder={t(
                                          "products.form.enterProductName",
                                        )}
                                        bg={manualEntryBg}
                                        color={baseFieldStyles.color}
                                        _hover={{
                                          borderColor: manualEntryBorderColor,
                                        }}
                                        _focus={{ bg: manualEntryFocusBg }}
                                        _focusVisible={fieldFocusStyles}
                                        borderColor={manualEntryBorderColor}
                                        isDisabled={isInvoiced}
                                      />
                                    </>
                                  )}
                                  {(() => {
                                    const itemValue = watchItems?.[index];
                                    if (!itemValue || !itemValue.product_id)
                                      return null;
                                    const prod = products.find(
                                      (p) => p.id === itemValue.product_id,
                                    );
                                    if (prod?.is_assembly) {
                                      return (
                                        <Tooltip label="Stok komponen akan otomatis dipotong saat penjualan disetujui (Auto-Kitting)">
                                          <Badge
                                            colorScheme="purple"
                                            size="xs"
                                            cursor="help"
                                          >
                                            🚀 Auto-Kitting
                                          </Badge>
                                        </Tooltip>
                                      );
                                    }
                                    return null;
                                  })()}
                                </VStack>
                                <FormErrorMessage fontSize="xs">
                                  {errors.items?.[index]?.product_id?.message ||
                                    errors.items?.[index]?.product_name
                                      ?.message}
                                </FormErrorMessage>
                              </FormControl>
                            </Td>
                            <Td minW="170px">
                              <FormControl
                                isInvalid={!!errors.items?.[index]?.description}
                              >
                                <Input
                                  size="sm"
                                  {...register(`items.${index}.description`, {
                                    validate: (value) => {
                                      if (
                                        isManualEntry(index) &&
                                        !getItemProductName(index)
                                      ) {
                                        return true;
                                      }
                                      return (
                                        !!value?.trim() ||
                                        t("validation.required")
                                      );
                                    },
                                    onChange: () =>
                                      clearErrors([
                                        `items.${index}.product_id`,
                                        `items.${index}.description`,
                                      ]),
                                  })}
                                  placeholder={
                                    isManualEntry(index)
                                      ? t("common.placeholders.itemDescription")
                                      : t("common.placeholders.itemDescription")
                                  }
                                  bg={
                                    isManualEntry(index)
                                      ? manualEntryBg
                                      : inputBg
                                  }
                                  color={baseFieldStyles.color}
                                  _hover={{
                                    borderColor: isManualEntry(index)
                                      ? manualEntryBorderColor
                                      : borderColor,
                                  }}
                                  _focus={{
                                    bg: isManualEntry(index)
                                      ? manualEntryFocusBg
                                      : inputFocusBg,
                                  }}
                                  _focusVisible={fieldFocusStyles}
                                  borderColor={
                                    isManualEntry(index)
                                      ? manualEntryBorderColor
                                      : borderColor
                                  }
                                  isDisabled={isInvoiced}
                                />
                                <FormErrorMessage fontSize="xs">
                                  {errors.items?.[index]?.description?.message}
                                </FormErrorMessage>
                              </FormControl>
                            </Td>
                            <Td minW="130px">
                              <Input
                                size="sm"
                                {...register(`items.${index}.location`)}
                                placeholder="Gedung A, dll"
                                {...baseFieldStyles}
                              />
                            </Td>
                            <Td minW="120px">
                              <FormControl
                                isInvalid={!!errors.items?.[index]?.quantity}
                              >
                                <Controller
                                  control={control}
                                  name={`items.${index}.quantity`}
                                  rules={{
                                    required: "Quantity is required",
                                    min: {
                                      value: 1,
                                      message: "Quantity minimal 1",
                                    },
                                  }}
                                  render={({ field }) => (
                                    <NumberInput
                                      size="sm"
                                      min={1}
                                      isDisabled={isInvoiced}
                                      value={field.value ?? 1}
                                      onChange={(
                                        valueString,
                                        valueAsNumber,
                                      ) => {
                                        const parsedValue =
                                          valueString === ""
                                            ? ""
                                            : Number.isFinite(valueAsNumber)
                                              ? Math.max(
                                                  1,
                                                  Math.floor(valueAsNumber),
                                                )
                                              : 1;
                                        field.onChange(parsedValue);
                                        clearErrors(`items.${index}.quantity`);
                                      }}
                                    >
                                      <NumberInputField
                                        {...baseFieldStyles}
                                        name={field.name}
                                        ref={field.ref}
                                        onBlur={field.onBlur}
                                      />
                                      <NumberInputStepper>
                                        <NumberIncrementStepper />
                                        <NumberDecrementStepper />
                                      </NumberInputStepper>
                                    </NumberInput>
                                  )}
                                />
                                <FormErrorMessage fontSize="xs">
                                  {errors.items?.[index]?.quantity?.message}
                                </FormErrorMessage>
                                {(() => {
                                  const fulfillmentPreview =
                                    getItemFulfillmentPreview(index);
                                  if (!fulfillmentPreview) {
                                    return null;
                                  }

                                  if (
                                    fulfillmentPreview.futureFulfillmentQty > 0
                                  ) {
                                    return (
                                      <Text
                                        mt={1}
                                        fontSize="xs"
                                        color="orange.500"
                                      >
                                        Future fulfillment:{" "}
                                        {fulfillmentPreview.availableNow}{" "}
                                        {fulfillmentPreview.unit} ready stock,{" "}
                                        {
                                          fulfillmentPreview.futureFulfillmentQty
                                        }{" "}
                                        {fulfillmentPreview.unit} menunggu
                                        stok. Est. HPP backorder:{" "}
                                        {salesService.formatCurrency(
                                          fulfillmentPreview.futureFulfillmentQty *
                                            fulfillmentPreview.estimatedUnitCost,
                                        )}
                                      </Text>
                                    );
                                  }

                                  return (
                                    <Text
                                      mt={1}
                                      fontSize="xs"
                                      color={successTextColor}
                                    >
                                      Ready stock:{" "}
                                      {fulfillmentPreview.availableNow}{" "}
                                      {fulfillmentPreview.unit} tersedia saat
                                      invoicing.
                                    </Text>
                                  );
                                })()}
                              </FormControl>
                            </Td>
                            <Td minW="200px">
                              <Box minW="180px">
                                <CurrencyInput
                                  value={watchItems[index]?.unit_price || 0}
                                  onChange={(value) => {
                                    setValue(`items.${index}.unit_price`, value);
                                    clearErrors(`items.${index}.unit_price`);
                                  }}
                                  placeholder="Rp 10.000"
                                  size="sm"
                                  min={0}
                                  showLabel={false}
                                  isDisabled={isInvoiced}
                                  isInvalid={!!errors.items?.[index]?.unit_price}
                                  errorMessage={
                                    errors.items?.[index]?.unit_price?.message
                                  }
                                />
                              </Box>
                            </Td>
                            <Td minW="180px">
                              <Box minW="160px">
                                <CurrencyInput
                                  value={
                                    watchItems[index]?.estimated_unit_cost || 0
                                  }
                                  onChange={(value) => {
                                    setValue(
                                      `items.${index}.estimated_unit_cost`,
                                      value,
                                    );
                                  }}
                                  placeholder="Rp 0"
                                  size="sm"
                                  min={0}
                                  showLabel={false}
                                  isDisabled={isInvoiced}
                                />
                                <Text mt={1} fontSize="xs" color={textColor}>
                                  Dipakai untuk HPP estimasi/backorder.
                                </Text>
                              </Box>
                            </Td>
                            <Td minW="130px">
                              <Controller
                                control={control}
                                name={`items.${index}.discount_percent`}
                                render={({ field }) => (
                                  <NumberInput
                                    size="sm"
                                    min={0}
                                    max={100}
                                    isDisabled={isInvoiced}
                                    value={field.value ?? 0}
                                    onChange={(valueString, valueAsNumber) => {
                                      const parsedValue =
                                        valueString === ""
                                          ? ""
                                          : Number.isFinite(valueAsNumber)
                                            ? Math.min(
                                                100,
                                                Math.max(0, valueAsNumber),
                                              )
                                            : 0;
                                      field.onChange(parsedValue);
                                    }}
                                  >
                                    <NumberInputField
                                      {...baseFieldStyles}
                                      name={field.name}
                                      ref={field.ref}
                                      onBlur={field.onBlur}
                                    />
                                  </NumberInput>
                                )}
                              />
                            </Td>
                            <Td minW="95px">
                              <VStack spacing={1} align="center">
                                <Switch
                                  size="sm"
                                  {...register(`items.${index}.taxable`)}
                                  colorScheme="green"
                                  isDisabled={isInvoiced}
                                />
                                <Text
                                  fontSize="xs"
                                  color={
                                    watchItems[index]?.taxable !== false
                                      ? successTextColor
                                      : textColor
                                  }
                                >
                                  {watchItems[index]?.taxable !== false
                                    ? "Tax"
                                    : "No Tax"}
                                </Text>
                              </VStack>
                            </Td>
                            <Td minW="190px">
                              <FormControl
                                isInvalid={
                                  !!errors.items?.[index]?.revenue_account_id
                                }
                              >
                {canSelectRevenueAccountFromCOA ? (
                  <Select
                                    size="sm"
                                    {...register(
                                      `items.${index}.revenue_account_id`,
                                      {
                                        setValueAs: (value) =>
                                          parseInt(value) || 0,
                                        validate: (value) =>
                                          Number(value ?? 0) > 0 ||
                                          "Pilih COA / Revenue Account untuk item ini",
                                        onChange: () =>
                                          clearErrors(
                                            `items.${index}.revenue_account_id`,
                                          ),
                                      },
                                    )}
                                    {...baseFieldStyles}
                                    placeholder={t(
                                      "common.placeholders.selectAccount",
                                    )}
                                    isDisabled={isInvoiced}
                                  >
                                    <option value="">Pilih COA Pendapatan</option>
                                    {accounts.map((account) => (
                                      <option key={account.id} value={account.id}>
                                        {account.code} - {account.name}
                                      </option>
                                    ))}
                                  </Select>
                                ) : (
                                  <Box
                                    px={3}
                                    py={2}
                                    borderWidth="1px"
                                    borderColor={borderColor}
                                    borderRadius="md"
                                    bg={readOnlyBg}
                                  >
                                    <Text fontSize="xs" fontWeight="medium">
                                      Default COA Sistem
                                    </Text>
                                    <Text fontSize="xs" color={textColor}>
                                      {getRevenueAccountHelperText()}
                                    </Text>
                                  </Box>
                                )}
                                <FormErrorMessage fontSize="xs">
                                  {
                                    errors.items?.[index]?.revenue_account_id
                                      ?.message
                                  }
                                </FormErrorMessage>
                              </FormControl>
                            </Td>
                            <Td minW="150px">
                              <Text
                                fontSize="sm"
                                fontWeight="medium"
                                color={successTextColor}
                              >
                                {salesService.formatCurrency(
                                  calculateLineTotal(watchItems[index] || {}),
                                )}
                              </Text>
                            </Td>
                            <Td minW="70px">
                              <IconButton
                                size="sm"
                                colorScheme="red"
                                variant="ghost"
                                icon={<FiTrash2 />}
                                onClick={() => removeItem(index)}
                                isDisabled={fields.length === 1 || isInvoiced}
                                aria-label={t("accessibility.removeItem")}
                              />
                            </Td>
                          </Tr>
                        ))}
                      </Tbody>
                    </Table>
                  </Box>
                </Box>

                <Divider />

                {/* Enhanced Tax Configuration (similar to Purchase Form) */}
                <Card>
                  <CardHeader pb={3}>
                    <Heading size="md" color={subHeadingColor}>
                      💰 {t("sales.form.taxInfo")}
                    </Heading>
                  </CardHeader>
                  <CardBody pt={0}>
                    <VStack spacing={4} align="stretch">
                      {/* Global Discount */}
                      <FormControl>
                        <FormLabel>
                          {t("sales.form.globalDiscount")}
                          <Badge
                            ml={2}
                            colorScheme="blue"
                            size="sm"
                            cursor="pointer"
                            onClick={() =>
                              setDiscountType(
                                discountType === "percentage"
                                  ? "amount"
                                  : "percentage",
                              )
                            }
                          >
                            {discountType === "percentage" ? "%" : "Rp"}
                          </Badge>
                        </FormLabel>
                        {discountType === "percentage" ? (
                          <NumberInput
                            min={0}
                            max={100}
                            size="sm"
                            isDisabled={isInvoiced}
                          >
                            <NumberInputField
                              {...register("discount_percent", {
                                setValueAs: (value) => parseFloat(value) || 0,
                              })}
                              {...baseFieldStyles}
                              placeholder="0"
                            />
                          </NumberInput>
                        ) : (
                          <CurrencyInput
                            value={watchDiscountPercent || 0}
                            onChange={(value) =>
                              setValue("discount_percent", value)
                            }
                            placeholder="Rp 0"
                            min={0}
                            showLabel={false}
                            size="sm"
                            isDisabled={isInvoiced}
                          />
                        )}
                        <FormHelperText fontSize="xs" color={textColor}>
                          {t("sales.tooltips.discount")}
                        </FormHelperText>
                      </FormControl>

                      <Divider />

                      {/* Tax Additions (Penambahan) */}
                      <Box>
                        <Text
                          fontSize="sm"
                          fontWeight="medium"
                          color={successTextColor}
                          mb={3}
                        >
                          ➕ {t("sales.form.otherTaxAdditions")}
                        </Text>
                        <SimpleGrid w="full" columns={{ base: 1, xl: 3 }} spacing={4}>
                          <FormControl>
                            <FormLabel fontSize="sm">
                              {t("sales.form.ppnRate")}
                            </FormLabel>
                            <NumberInput
                              min={0}
                              max={100}
                              step={0.1}
                              size="sm"
                              isDisabled={isInvoiced}
                            >
                              <NumberInputField
                                {...register("ppn_rate", {
                                  setValueAs: (value) => parseFloat(value) || 0,
                                })}
                                {...baseFieldStyles}
                                placeholder="11"
                              />
                              <NumberInputStepper>
                                <NumberIncrementStepper />
                                <NumberDecrementStepper />
                              </NumberInputStepper>
                            </NumberInput>
                            <FormHelperText fontSize="xs">
                              {t("sales.tooltips.ppnRate")}
                            </FormHelperText>
                          </FormControl>

                          <FormControl>
                            <FormLabel fontSize="sm">
                              {t("sales.form.otherTaxAdditions")}
                            </FormLabel>
                            <CurrencyInput
                              value={watchOtherTaxAdditions || 0}
                              onChange={(value) =>
                                setValue("other_tax_additions", value)
                              }
                              placeholder="Rp 0"
                              min={0}
                              showLabel={false}
                              size="sm"
                              isDisabled={isInvoiced}
                            />
                            <FormHelperText fontSize="xs">
                              {t("sales.tooltips.otherTaxAdditions")}
                            </FormHelperText>
                          </FormControl>

                          <FormControl>
                            <FormLabel fontSize="sm">
                              {t("sales.form.shippingCost")}
                            </FormLabel>
                            <CurrencyInput
                              value={watchShippingCost || 0}
                              onChange={(value) =>
                                setValue("shipping_cost", value)
                              }
                              placeholder="Rp 0"
                              min={0}
                              showLabel={false}
                              size="sm"
                              isDisabled={
                                isInvoiced || shippingTreatment === "buyer_direct"
                              }
                            />
                            <FormHelperText fontSize="xs">
                              {shippingTreatment === "buyer_direct"
                                ? t("sales.tooltips.shippingDirectNoInvoice")
                                : t("sales.tooltips.shippingCost")}
                            </FormHelperText>
                            <Box mt={3}>
                              <HStack spacing={2} mb={2} align="center">
                                <Text fontSize="xs" fontWeight="medium" color={textColor}>
                                  {t("sales.form.shippingTreatment")}
                                </Text>
                                <Tooltip
                                  label={t("sales.tooltips.shippingTreatment")}
                                  hasArrow
                                  placement="top"
                                >
                                  <Box
                                    as="span"
                                    display="inline-flex"
                                    alignItems="center"
                                    justifyContent="center"
                                    w="18px"
                                    h="18px"
                                    borderRadius="full"
                                    bg={readOnlyBg}
                                    color={textColor}
                                    fontSize="xs"
                                    cursor="help"
                                  >
                                    ?
                                  </Box>
                                </Tooltip>
                              </HStack>
                              <SimpleGrid columns={{ base: 1, md: 3, xl: 1 }} spacing={2}>
                                {[
                                  {
                                    value: "buyer_taxable" as ShippingTreatment,
                                    label: t("sales.form.shippingTreatmentTaxable"),
                                    helper: t("sales.form.shippingTreatmentTaxableDesc"),
                                    colorScheme: "green",
                                  },
                                  {
                                    value: "buyer_non_taxable" as ShippingTreatment,
                                    label: t("sales.form.shippingTreatmentNonTaxable"),
                                    helper: t("sales.form.shippingTreatmentNonTaxableDesc"),
                                    colorScheme: "orange",
                                  },
                                  {
                                    value: "buyer_direct" as ShippingTreatment,
                                    label: t("sales.form.shippingTreatmentDirect"),
                                    helper: t("sales.form.shippingTreatmentDirectDesc"),
                                    colorScheme: "gray",
                                  },
                                ].map((option) => {
                                  const isActive =
                                    shippingTreatment === option.value;

                                  return (
                                    <Button
                                      key={option.value}
                                      type="button"
                                      size="sm"
                                      h="auto"
                                      minH="58px"
                                      p={2}
                                      justifyContent="flex-start"
                                      textAlign="left"
                                      whiteSpace="normal"
                                      variant={isActive ? "solid" : "outline"}
                                      colorScheme={option.colorScheme}
                                      isDisabled={isInvoiced}
                                      onClick={() =>
                                        handleShippingTreatmentChange(option.value)
                                      }
                                    >
                                      <VStack spacing={0.5} align="stretch">
                                        <Text fontSize="xs" fontWeight="semibold">
                                          {option.label}
                                        </Text>
                                        <Text fontSize="2xs" fontWeight="normal">
                                          {option.helper}
                                        </Text>
                                      </VStack>
                                    </Button>
                                  );
                                })}
                              </SimpleGrid>
                            </Box>
                          </FormControl>
                        </SimpleGrid>
                      </Box>

                      <Divider />

                      {/* Tax Deductions (Pemotongan) */}
                      <Box>
                        <Text
                          fontSize="sm"
                          fontWeight="medium"
                          color="red.600"
                          mb={3}
                        >
                          ➖ {t("sales.form.otherTaxDeductions")}
                        </Text>
                        <HStack w="full" spacing={4}>
                          <FormControl>
                            <FormLabel fontSize="sm">
                              {t("sales.form.pph21Rate")}
                            </FormLabel>
                            <NumberInput
                              min={0}
                              max={100}
                              step={0.1}
                              size="sm"
                              isDisabled={isInvoiced}
                            >
                              <NumberInputField
                                {...register("pph21_rate", {
                                  setValueAs: (value) => parseFloat(value) || 0,
                                })}
                                {...baseFieldStyles}
                                placeholder="2"
                              />
                              <NumberInputStepper>
                                <NumberIncrementStepper />
                                <NumberDecrementStepper />
                              </NumberInputStepper>
                            </NumberInput>
                            <FormHelperText fontSize="xs">
                              {t("sales.tooltips.pph21Rate")}
                            </FormHelperText>
                          </FormControl>

                          <FormControl>
                            <FormLabel fontSize="sm">
                              {t("sales.form.pph23Rate")}
                            </FormLabel>
                            <NumberInput
                              min={0}
                              max={100}
                              step={0.1}
                              size="sm"
                              isDisabled={isInvoiced}
                            >
                              <NumberInputField
                                {...register("pph23_rate", {
                                  setValueAs: (value) => parseFloat(value) || 0,
                                })}
                                {...baseFieldStyles}
                                placeholder="2"
                              />
                              <NumberInputStepper>
                                <NumberIncrementStepper />
                                <NumberDecrementStepper />
                              </NumberInputStepper>
                            </NumberInput>
                            <FormHelperText fontSize="xs">
                              PPh 23: 2% jasa umum, 15% dividen/bunga/royalti
                            </FormHelperText>
                          </FormControl>

                          <FormControl>
                            <FormLabel fontSize="sm">
                              Other Tax Deductions (IDR)
                            </FormLabel>
                            <CurrencyInput
                              value={watchOtherTaxDeductions || 0}
                              onChange={(value) =>
                                setValue("other_tax_deductions", value)
                              }
                              placeholder="Rp 0"
                              min={0}
                              showLabel={false}
                              size="sm"
                              isDisabled={isInvoiced}
                            />
                            <FormHelperText fontSize="xs">
                              Potongan pajak lainnya (flat amount)
                            </FormHelperText>
                          </FormControl>
                        </HStack>
                      </Box>

                      {/* Tax Summary Calculation */}
                      {calculateSubtotal() > 0 && (
                        <Box
                          mt={4}
                          p={4}
                          bg={alertBg}
                          borderRadius="md"
                          border="1px solid"
                          borderColor={alertBorderColor}
                        >
                          <VStack spacing={2} align="stretch">
                            <Text
                              fontSize="sm"
                              fontWeight="semibold"
                              color={subHeadingColor}
                            >
                              Tax Summary:
                            </Text>
                            {(() => {
                              const subtotal = calculateSubtotal();
                              const ppnRate =
                                watchPPNRate ??
                                watchPPNPercent ??
                                defaultTaxRate;
                              const discountPercent =
                                discountType === "percentage"
                                  ? watchDiscountPercent
                                  : subtotal > 0
                                    ? (Math.min(watchDiscountPercent || 0, subtotal) /
                                        subtotal) *
                                      100
                                    : 0;
                              const amounts = calculateSalesAmounts({
                                items: watchItems,
                                discountPercent,
                                ppnRate,
                                pph21Rate: watchPPh21Rate,
                                pph23Rate: watchPPh23Rate,
                                otherTaxAdditions: watchOtherTaxAdditions,
                                otherTaxDeductions: watchOtherTaxDeductions,
                                shippingCost: watchShippingCost,
                                shippingTaxable: watchShippingTaxable,
                              });
                              const discountAmount = amounts.discountAmount;
                              const afterDiscount = amounts.netBeforeTax;
                              const ppnAmount = amounts.ppnAmount;
                              const otherAdditions = watchOtherTaxAdditions || 0;
                              const pph21Amount = amounts.pph21Amount;
                              const pph23Amount = amounts.pph23Amount;
                              const otherDeductions =
                                watchOtherTaxDeductions || 0;
                              const finalTotal = amounts.totalAmount;
                              const shippingStatusLabel =
                                shippingTreatment === "buyer_taxable"
                                  ? t("sales.form.shippingIncludedInDpp")
                                  : shippingTreatment === "buyer_non_taxable"
                                    ? t("sales.form.shippingExcludedFromDpp")
                                    : t("sales.form.shippingDirectInSummary");
                              const shippingStatusColorScheme =
                                shippingTreatment === "buyer_taxable"
                                  ? "green"
                                  : shippingTreatment === "buyer_non_taxable"
                                    ? "orange"
                                    : "gray";

                              return (
                                <VStack spacing={2} fontSize="xs">
                                  <HStack justify="space-between" w="full">
                                    <Text color={textColor}>Subtotal:</Text>
                                    <Text color={textColor}>
                                      {salesService.formatCurrency(subtotal)}
                                    </Text>
                                  </HStack>
                                  {discountAmount > 0 && (
                                    <HStack justify="space-between" w="full">
                                      <Text color={textColor}>
                                        Global Discount:
                                      </Text>
                                      <Text color="red.500">
                                        -
                                        {salesService.formatCurrency(
                                          discountAmount,
                                        )}
                                      </Text>
                                    </HStack>
                                  )}
                                  <HStack justify="space-between" w="full">
                                    <Text color={textColor}>
                                      After Discount:
                                    </Text>
                                    <Text color={textColor}>
                                      {salesService.formatCurrency(
                                        afterDiscount,
                                      )}
                                    </Text>
                                  </HStack>
                                  {watchShippingCost > 0 && (
                                    <HStack justify="space-between" w="full">
                                      <Text color={textColor}>
                                        + {t("sales.form.shippingCost")}:
                                      </Text>
                                      <Text color={textColor}>
                                        {salesService.formatCurrency(
                                          watchShippingCost,
                                        )}
                                      </Text>
                                    </HStack>
                                  )}
                                  {(watchShippingCost > 0 ||
                                    shippingTreatment !== "buyer_taxable") && (
                                    <HStack
                                      justify="space-between"
                                      w="full"
                                      align="flex-start"
                                      gap={3}
                                    >
                                      <Text color={textColor}>
                                        {t("sales.form.shippingDppStatus")}:
                                      </Text>
                                      <Badge
                                        colorScheme={shippingStatusColorScheme}
                                        whiteSpace="normal"
                                        textAlign="right"
                                      >
                                        {shippingStatusLabel}
                                      </Badge>
                                    </HStack>
                                  )}
                                  <HStack justify="space-between" w="full">
                                    <Text color={textColor}>DPP PPN:</Text>
                                    <Text color={textColor}>
                                      {salesService.formatCurrency(
                                        amounts.taxableAmount,
                                      )}
                                    </Text>
                                  </HStack>

                                  <Divider />
                                  <HStack justify="space-between" w="full">
                                    <Text color={successTextColor}>
                                      + PPN ({ppnRate}%):
                                    </Text>
                                    <Text color={successTextColor}>
                                      {salesService.formatCurrency(ppnAmount)}
                                    </Text>
                                  </HStack>
                                  {otherAdditions > 0 && (
                                    <HStack justify="space-between" w="full">
                                      <Text color={successTextColor}>
                                        + Other Additions:
                                      </Text>
                                      <Text color={successTextColor}>
                                        {salesService.formatCurrency(
                                          otherAdditions,
                                        )}
                                      </Text>
                                    </HStack>
                                  )}
                                  {pph21Amount > 0 && (
                                    <HStack justify="space-between" w="full">
                                      <Text color="red.600">
                                        - PPh21 ({watchPPh21Rate}%):
                                      </Text>
                                      <Text color="red.600">
                                        -
                                        {salesService.formatCurrency(
                                          pph21Amount,
                                        )}
                                      </Text>
                                    </HStack>
                                  )}
                                  {pph23Amount > 0 && (
                                    <HStack justify="space-between" w="full">
                                      <Text color="red.600">
                                        - PPh23 ({watchPPh23Rate}%):
                                      </Text>
                                      <Text color="red.600">
                                        -
                                        {salesService.formatCurrency(
                                          pph23Amount,
                                        )}
                                      </Text>
                                    </HStack>
                                  )}
                                  {otherDeductions > 0 && (
                                    <HStack justify="space-between" w="full">
                                      <Text color="red.600">
                                        - Other Deductions:
                                      </Text>
                                      <Text color="red.600">
                                        -
                                        {salesService.formatCurrency(
                                          otherDeductions,
                                        )}
                                      </Text>
                                    </HStack>
                                  )}
                                  <Divider />

                                  <HStack justify="space-between" w="full">
                                    <Text
                                      fontWeight="bold"
                                      color={subHeadingColor}
                                    >
                                      Final Total:
                                    </Text>
                                    <Text fontWeight="bold" color="blue.600">
                                      {salesService.formatCurrency(finalTotal)}
                                    </Text>
                                  </HStack>
                                </VStack>
                              );
                            })()}
                          </VStack>
                        </Box>
                      )}
                    </VStack>
                  </CardBody>
                </Card>

                <Divider />

                {/* Additional Information */}
                <Box>
                  <Heading size="md" mb={4} color={subHeadingColor}>
                    📝 {t("sales.form.additionalInfo")}
                  </Heading>
                  <VStack spacing={4}>
                    <HStack w="full" spacing={4}>
                      <FormControl>
                        <FormLabel>{t("sales.form.paymentTerms")}</FormLabel>
                        <Select
                          {...register("payment_terms")}
                          bg={inputBg}
                          _focus={{ bg: inputFocusBg }}
                        >
                          <option value="COD">
                            {t("sales.form.paymentTermsOptions.cod")}
                          </option>
                          <option value="NET_15">
                            {t("sales.form.paymentTermsOptions.net15")}
                          </option>
                          <option value="NET_30">
                            {t("sales.form.paymentTermsOptions.net30")}
                          </option>
                          <option value="NET_60">
                            {t("sales.form.paymentTermsOptions.net60")}
                          </option>
                          <option value="NET_90">
                            {t("sales.form.paymentTermsOptions.net90")}
                          </option>
                          <option
                            value="CUSTOM"
                            disabled={watchPaymentMethodType !== "CREDIT"}
                          >
                            Custom Termin / Installment Schedule
                          </option>
                        </Select>
                        <FormHelperText color={textColor}>
                          {watchPaymentMethodType === "CREDIT"
                            ? 'Untuk cicilan atau termin 30%-40%-30%, pilih "Custom Termin / Installment Schedule".'
                            : "Termin custom hanya tersedia untuk penjualan kredit/piutang."}
                        </FormHelperText>
                        {watchPaymentMethodType === "CREDIT" &&
                          watchPaymentTerms !== "CUSTOM" && (
                            <HStack mt={2} spacing={3} align="center">
                              <Text fontSize="sm" color="orange.600">
                                Blok jadwal termin baru muncul jika payment
                                terms diubah ke custom.
                              </Text>
                              <Button
                                size="xs"
                                variant="outline"
                                colorScheme="blue"
                                onClick={() =>
                                  setValue("payment_terms", "CUSTOM")
                                }
                              >
                                Gunakan Termin Custom
                              </Button>
                            </HStack>
                          )}
                      </FormControl>

                      <FormControl>
                        <FormLabel>{t("sales.form.reference")}</FormLabel>
                        <Input
                          {...register("reference")}
                          placeholder={t("common.placeholders.referenceNumber")}
                          bg={inputBg}
                          _focus={{ bg: inputFocusBg }}
                        />
                        <FormHelperText color={textColor}>
                          {t("common.placeholders.referenceNumber")}
                        </FormHelperText>
                      </FormControl>
                    </HStack>

                    {/* Double Entry Accounting Section */}
                    <Box
                      p={4}
                      bg={alertBg}
                      borderRadius="md"
                      border="1px solid"
                      borderColor={alertBorderColor}
                    >
                      <Text
                        fontSize="md"
                        fontWeight="semibold"
                        color={subHeadingColor}
                        mb={3}
                      >
                        🏦 {t("sales.form.paymentInfo")}
                      </Text>
                      <VStack spacing={4} align="stretch">
                        <Alert status="info" size="sm" borderRadius="md">
                          <AlertIcon />
                          <AlertDescription fontSize="sm">
                            <Text>
                              <strong>Double Entry Logic:</strong>
                            </Text>
                            <Text mt={1}>
                              •{" "}
                              <strong>
                                {t("sales.form.paymentMethodOptions.cash")}/
                                {t("sales.form.paymentMethodOptions.bank")}:
                              </strong>{" "}
                              Saat invoice dibuat, sistem otomatis Debit
                              Cash/Bank Account dan Credit Revenue Account
                            </Text>
                            <Text>
                              •{" "}
                              <strong>
                                {t("sales.form.paymentMethodOptions.credit")}:
                              </strong>{" "}
                              Saat invoice dibuat, sistem Debit selected
                              Receivable COA dan Credit Revenue Account
                            </Text>
                          </AlertDescription>
                        </Alert>

                        <SimpleGrid
                          w="full"
                          columns={{
                            base: 1,
                            lg: watchPaymentMethodType === "CREDIT" ? 3 : 2,
                          }}
                          spacing={4}
                        >
                          <FormControl isRequired>
                            <FormLabel>
                              {t("sales.form.paymentMethod")}
                            </FormLabel>
                            <Select
                              {...register("payment_method_type", {
                                required: t("validation.required"),
                              })}
                              bg={inputBg}
                              _focus={{ bg: inputFocusBg }}
                            >
                              <option value="CASH">
                                {t("sales.form.paymentMethodOptions.cash")}
                              </option>
                              <option value="BANK">
                                {t("sales.form.paymentMethodOptions.bank")}
                              </option>
                              <option value="CREDIT">
                                {t("sales.form.paymentMethodOptions.credit")}
                              </option>
                            </Select>
                            <FormHelperText color={textColor}>
                              {t("sales.tooltips.paymentMethod")}
                            </FormHelperText>
                          </FormControl>

                          {(watchPaymentMethodType === "CASH" ||
                            watchPaymentMethodType === "BANK") && (
                            <FormControl isRequired>
                              <FormLabel>
                                {watchPaymentMethodType === "CASH"
                                  ? t("sales.form.paymentMethodOptions.cash")
                                  : t("sales.form.paymentMethodOptions.bank")}
                              </FormLabel>
                              <Select
                                {...register("cash_bank_id", {
                                  required: t("validation.required"),
                                  setValueAs: (value) =>
                                    parseInt(value) || undefined,
                                })}
                                bg={inputBg}
                                _focus={{ bg: inputFocusBg }}
                                isDisabled={loadingData}
                              >
                                <option value="">
                                  {loadingData
                                    ? `Loading ${watchPaymentMethodType.toLowerCase()} accounts...`
                                    : cashBankAccounts.length === 0
                                      ? `No ${watchPaymentMethodType.toLowerCase()} accounts available`
                                      : `Select ${watchPaymentMethodType.toLowerCase()} account`}
                                </option>
                                {cashBankAccounts
                                  .filter((account) =>
                                    watchPaymentMethodType === "CASH"
                                      ? account.type === "CASH"
                                      : account.type === "BANK",
                                  )
                                  .map((account) => (
                                    <option key={account.id} value={account.id}>
                                      {account.code} - {account.name}
                                      {account.bank_name &&
                                        ` (${account.bank_name})`}
                                    </option>
                                  ))}
                              </Select>
                              <FormHelperText color={textColor}>
                                Which {watchPaymentMethodType.toLowerCase()}{" "}
                                account will receive the money
                              </FormHelperText>
                            </FormControl>
                          )}

                          {watchPaymentMethodType === "CREDIT" && (
                            <FormControl
                              isRequired
                              isInvalid={!!errors.receivable_account_id}
                            >
                              <FormLabel>Akun Piutang (COA)</FormLabel>
                              <Select
                                {...register("receivable_account_id", {
                                  required:
                                    "Pilih COA piutang untuk penjualan kredit",
                                  setValueAs: (value) =>
                                    parseInt(value) || undefined,
                                })}
                                bg={inputBg}
                                _focus={{ bg: inputFocusBg }}
                                isDisabled={loadingData}
                              >
                                <option value="">
                                  {loadingData
                                    ? "Loading akun piutang..."
                                    : receivableAccounts.length === 0
                                      ? "Tidak ada akun piutang detail aktif"
                                      : "Pilih akun piutang"}
                                </option>
                                {receivableAccounts.map((account) => (
                                  <option key={account.id} value={account.id}>
                                    {account.code} - {account.name}
                                  </option>
                                ))}
                              </Select>
                              <FormHelperText color={textColor}>
                                {getReceivableAccountHelperText()}
                              </FormHelperText>
                              <FormErrorMessage>
                                {errors.receivable_account_id?.message}
                              </FormErrorMessage>
                            </FormControl>
                          )}

                          {watchPaymentMethodType === "CREDIT" && (
                            <FormControl>
                              <FormLabel>
                                Bank Account for Invoice (optional)
                              </FormLabel>
                              <Select
                                {...register("cash_bank_id", {
                                  setValueAs: (value) =>
                                    parseInt(value) || undefined,
                                })}
                                bg={inputBg}
                                _focus={{ bg: inputFocusBg }}
                                isDisabled={loadingData}
                              >
                                <option value="">
                                  {loadingData
                                    ? "Loading bank accounts..."
                                    : cashBankAccounts.length === 0
                                      ? "No bank accounts available"
                                      : "Select bank account"}
                                </option>
                                {cashBankAccounts
                                  .filter((account) => account.type === "BANK")
                                  .map((account) => (
                                    <option key={account.id} value={account.id}>
                                      {account.code} - {account.name}
                                      {account.bank_name &&
                                        ` (${account.bank_name})`}
                                    </option>
                                  ))}
                              </Select>
                              <FormHelperText color={textColor}>
                                Bank tujuan transfer yang akan dicetak pada
                                invoice. Jika kosong, sistem akan menampilkan
                                bank aktif pertama.
                              </FormHelperText>
                            </FormControl>
                          )}
                        </SimpleGrid>

                        {watchPaymentMethodType === "CREDIT" && (
                          <Alert status="warning" size="sm" borderRadius="md">
                            <AlertIcon />
                            <AlertDescription fontSize="sm">
                              <Text>
                                <strong>Credit Sale:</strong> Invoice akan
                                membentuk piutang pada COA yang dipilih.
                                Pembayaran customer akan meng-credit COA yang
                                sama.
                              </Text>
                            </AlertDescription>
                          </Alert>
                        )}

                        {(watchPaymentMethodType === "CASH" ||
                          watchPaymentMethodType === "BANK") && (
                          <Alert status="success" size="sm" borderRadius="md">
                            <AlertIcon />
                            <AlertDescription fontSize="sm">
                              <Text>
                                <strong>Immediate Payment:</strong> Money will
                                be recorded as received in the selected{" "}
                                {watchPaymentMethodType.toLowerCase()} account.
                              </Text>
                            </AlertDescription>
                          </Alert>
                        )}
                      </VStack>
                    </Box>

                    {/* Payment Terms Explanation */}
                    {watchPaymentTerms &&
                      watchPaymentTerms !== "CUSTOM" &&
                      getPaymentTermsExplanation(watchPaymentTerms) && (
                        <Alert
                          status="info"
                          borderRadius="md"
                          bg={alertBg}
                          borderColor={alertBorderColor}
                        >
                          <AlertIcon color="blue.500" />
                          <AlertDescription fontSize="sm">
                            <Text>
                              <strong>Payment Terms:</strong>{" "}
                              {getPaymentTermsExplanation(watchPaymentTerms)}
                            </Text>
                            {watchDate && isDueDateAutoCalculated && (
                              <Text mt={1} color={successTextColor}>
                                <strong>Due Date:</strong>{" "}
                                {salesService.formatDate(
                                  calculateDueDateFromPaymentTerms(
                                    watchDate,
                                    watchPaymentTerms,
                                  ) || "",
                                )}
                              </Text>
                            )}
                          </AlertDescription>
                        </Alert>
                      )}

                    {watchPaymentTerms === "CUSTOM" && (
                      <Box
                        p={4}
                        border="1px solid"
                        borderColor={borderColor}
                        borderRadius="md"
                        bg={tableBg}
                      >
                        <HStack justify="space-between" align="center" mb={4}>
                          <Box>
                            <Text fontWeight="semibold" color={subHeadingColor}>
                              Payment Schedule
                            </Text>
                            <Text fontSize="sm" color={textColor}>
                              Tetap 1 invoice, tetapi customer membayar sesuai
                              termin yang Anda tentukan di bawah.
                            </Text>
                          </Box>
                          <Button
                            size="sm"
                            leftIcon={<FiPlus />}
                            onClick={addPaymentSchedule}
                            colorScheme="blue"
                            variant="outline"
                          >
                            Tambah Termin
                          </Button>
                        </HStack>

                        <VStack spacing={3} align="stretch">
                          {scheduleFields.length === 0 && (
                            <Alert status="warning" borderRadius="md">
                              <AlertIcon />
                              <AlertDescription fontSize="sm">
                                Tambahkan minimal satu termin. Untuk kasus
                                30%-40%-30%, buat 3 termin sampai total
                                persentasenya 100%.
                              </AlertDescription>
                            </Alert>
                          )}

                          {scheduleFields.map((field, index) => {
                            const schedulePercentage =
                              Number(
                                watchPaymentSchedules?.[index]?.percentage,
                              ) || 0;
                            const scheduleDueDate =
                              watchPaymentSchedules?.[index]?.due_date;
                            const scheduleDescription =
                              watchPaymentSchedules?.[index]?.description;
                            const scheduleAmount = getScheduleAmount(
                              schedulePercentage,
                              index,
                            );

                            return (
                              <Box
                                key={field.id}
                                p={3}
                                border="1px solid"
                                borderColor={borderColor}
                                borderRadius="md"
                              >
                                <HStack
                                  justify="space-between"
                                  align="start"
                                  mb={3}
                                >
                                  <Box>
                                    <Text fontWeight="medium">
                                      Termin {index + 1}
                                    </Text>
                                    <Text fontSize="xs" color={textColor}>
                                      Nilai termin otomatis dihitung dari total
                                      invoice saat ini.
                                    </Text>
                                  </Box>
                                  <IconButton
                                    aria-label={`Hapus termin ${index + 1}`}
                                    icon={<FiTrash2 />}
                                    size="sm"
                                    variant="ghost"
                                    colorScheme="red"
                                    onClick={() => removePaymentSchedule(index)}
                                  />
                                </HStack>

                                <HStack align="start" spacing={3}>
                                  <FormControl isRequired>
                                    <FormLabel fontSize="sm">
                                      Persentase (%)
                                    </FormLabel>
                                    <Input
                                      type="number"
                                      step="0.01"
                                      min="0"
                                      max="100"
                                      {...register(
                                        `payment_schedules.${index}.percentage` as const,
                                        {
                                          setValueAs: (value) =>
                                            Number(value) || 0,
                                        },
                                      )}
                                      bg={inputBg}
                                      _focus={{ bg: inputFocusBg }}
                                    />
                                  </FormControl>

                                  <FormControl isRequired>
                                    <FormLabel fontSize="sm">
                                      Jatuh Tempo
                                    </FormLabel>
                                    <Input
                                      type="date"
                                      {...register(
                                        `payment_schedules.${index}.due_date` as const,
                                      )}
                                      bg={inputBg}
                                      _focus={{ bg: inputFocusBg }}
                                    />
                                  </FormControl>

                                  <FormControl>
                                    <FormLabel fontSize="sm">
                                      Deskripsi
                                    </FormLabel>
                                    <Input
                                      placeholder={`Mis. DP ${index + 1}`}
                                      {...register(
                                        `payment_schedules.${index}.description` as const,
                                      )}
                                      bg={inputBg}
                                      _focus={{ bg: inputFocusBg }}
                                    />
                                  </FormControl>
                                </HStack>

                                <HStack
                                  justify="space-between"
                                  mt={3}
                                  p={3}
                                  bg={alertBg}
                                  borderRadius="md"
                                >
                                  <Text fontSize="sm" color={textColor}>
                                    {scheduleDueDate
                                      ? `Termin jatuh tempo ${salesService.formatDate(scheduleDueDate)}`
                                      : "Isi jatuh tempo untuk termin ini"}
                                  </Text>
                                  <Text
                                    fontSize="sm"
                                    fontWeight="semibold"
                                    color="blue.600"
                                  >
                                    {schedulePercentage.toFixed(2)}% ={" "}
                                    {salesService.formatCurrency(
                                      scheduleAmount,
                                    )}
                                  </Text>
                                </HStack>
                              </Box>
                            );
                          })}
                        </VStack>

                        <HStack
                          justify="space-between"
                          mt={4}
                          p={3}
                          bg={alertBg}
                          borderRadius="md"
                        >
                          <Text fontSize="sm" color={textColor}>
                            Total invoice{" "}
                            {salesService.formatCurrency(calculateTotal())} akan
                            tetap menjadi 1 piutang usaha.
                          </Text>
                          <Text
                            fontSize="sm"
                            fontWeight="bold"
                            color={
                              Math.abs(
                                getPaymentSchedulePercentageTotal() - 100,
                              ) < 0.01
                                ? "green.600"
                                : "red.600"
                            }
                          >
                            Total termin:{" "}
                            {getPaymentSchedulePercentageTotal().toFixed(2)}%
                          </Text>
                        </HStack>
                      </Box>
                    )}

                    <FormControl>
                      <FormLabel>{t("sales.form.notes")}</FormLabel>
                      <Textarea
                        {...register("notes")}
                        placeholder={t("common.placeholders.notes")}
                        rows={3}
                        bg={inputBg}
                        _focus={{ bg: inputFocusBg }}
                      />
                      <FormHelperText color={textColor}>
                        {t("sales.tooltips.notes")}
                      </FormHelperText>
                    </FormControl>

                    <FormControl>
                      <FormLabel>{t("sales.form.internalNotes")}</FormLabel>
                      <Textarea
                        {...register("internal_notes")}
                        placeholder={t("common.placeholders.internalNotes")}
                        rows={3}
                        bg={inputBg}
                        _focus={{ bg: inputFocusBg }}
                      />
                      <FormHelperText color={textColor}>
                        {t("sales.tooltips.internalNotes")}
                      </FormHelperText>
                    </FormControl>
                  </VStack>
                </Box>
              </VStack>
            </ModalBody>

            <ModalFooter
              position="sticky"
              bottom={0}
              borderTopWidth={2}
              borderColor={borderColor}
              bg={footerBg}
              boxShadow={`0 -4px 12px ${shadowColor}`}
              px={6}
              py={4}
              mt={6}
              flexShrink={0}
              zIndex={10}
            >
              <HStack justify="space-between" spacing={4} w="full">
                {/* Left side - Form info */}
                <HStack spacing={2}>
                  <Text fontSize="sm" color={textColor}>
                    {loadingData
                      ? t("common.loading")
                      : `${fields.length} ${t("sales.form.items")}`}
                  </Text>
                  {calculateSubtotal() > 0 && (
                    <Text fontSize="sm" color="blue.600" fontWeight="medium">
                      {t("sales.total")}:{" "}
                      {salesService.formatCurrency(calculateTotal())}
                    </Text>
                  )}
                </HStack>

                {/* Right side - Action buttons */}
                <HStack spacing={3}>
                  <Button
                    leftIcon={<FiX />}
                    onClick={handleClose}
                    variant="outline"
                    size="lg"
                    isDisabled={loading}
                    colorScheme="gray"
                    minW="120px"
                  >
                    {t("common.cancel")}
                  </Button>
                  <Button
                    leftIcon={loading ? undefined : <FiSave />}
                    type="submit"
                    colorScheme="blue"
                    size="lg"
                    isLoading={loading}
                    loadingText={
                      sale ? t("common.updating") : t("common.creating")
                    }
                    minW="150px"
                    shadow="md"
                    _hover={{
                      shadow: "lg",
                      transform: "translateY(-1px)",
                    }}
                    _active={{
                      transform: "translateY(0)",
                    }}
                  >
                    {sale ? t("common.update") : t("common.create")}
                  </Button>
                </HStack>
              </HStack>
            </ModalFooter>
          </form>
        </ModalContent>
      </Modal>
    </>
  );
};

export default SalesForm;
