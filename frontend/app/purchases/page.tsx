"use client";

import React, { useEffect, useState, useCallback, useRef } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";
import { useTranslation } from "@/hooks/useTranslation";
import SimpleLayout from "@/components/layout/SimpleLayout";
import {
  parseClosedPeriodError,
  formatClosedPeriodDate,
} from "@/utils/errorHandler";
import CompanyTypeGuard from "@/components/common/CompanyTypeGuard";
import { DataTable } from "@/components/common/DataTable";
import EnhancedPurchaseTable from "@/components/purchase/EnhancedPurchaseTable";
import {
  Box,
  Flex,
  Heading,
  Button,
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
  Badge,
  Text,
  HStack,
  VStack,
  Spinner,
  useToast,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  ModalCloseButton,
  useDisclosure,
  Select,
  Input,
  InputGroup,
  InputLeftElement,
  FormControl,
  FormLabel,
  Grid,
  GridItem,
  Card,
  CardBody,
  CardHeader,
  Stat,
  StatLabel,
  StatNumber,
  Textarea,
  NumberInput,
  NumberInputField,
  NumberInputStepper,
  NumberIncrementStepper,
  NumberDecrementStepper,
  IconButton,
  Divider,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  TableContainer,
  SimpleGrid,
  FormHelperText,
  Tabs,
  TabList,
  TabPanels,
  Tab,
  TabPanel,
  Link,
  Icon,
  Menu,
  MenuButton,
  MenuList,
  MenuItem,
  Tooltip,
  Radio,
  RadioGroup,
} from "@chakra-ui/react";
import {
  FiPlus,
  FiEye,
  FiEdit,
  FiTrash2,
  FiFilter,
  FiRefreshCw,
  FiCheckCircle,
  FiClock,
  FiXCircle,
  FiAlertCircle,
  FiPackage,
  FiDownload,
  FiFileText,
  FiSearch,
  FiDollarSign,
  FiChevronLeft,
  FiChevronRight,
} from "react-icons/fi";
import purchaseService, {
  Purchase,
  PurchaseFilterParams,
  PurchaseItem,
  PurchaseSummary,
} from "@/services/purchaseService";
import {
  PurchaseRequest,
  PurchaseRequestItem as PurchaseRequestLineItem,
} from "@/services/purchaseRequestService";
import SubmitApprovalButton from "@/components/purchase/SubmitApprovalButton";
import { ApprovalPanel } from "@/components/approval/ApprovalPanel";
import PurchasePaymentForm from "@/components/purchase/PurchasePaymentForm";
import contactService from "@/services/contactService";
import productService, { Product } from "@/services/productService";
import accountService from "@/services/accountService";
import { Account as GLAccount, AccountCatalogItem } from "@/types/account";
import approvalService from "@/services/approvalService";
import { assetService } from "@/services/assetService";
import {
  isAccountingRole,
  isFinanceManagerRole,
  isFinanceTeamRole,
  normalizeRole,
  workflowRoleMatchesUserRole,
} from "@/utils/roles";
import { useModulePermissions } from "@/hooks/usePermissions";
import ProtectedModule from "@/components/common/ProtectedModule";
import { useColorModeValue } from "@chakra-ui/react";
import SearchableSelect from "@/components/common/SearchableSelect";
import CurrencyInput from "@/components/common/CurrencyInput";
import {
  filterActiveLeafAccounts,
  filterOperationalCashBankAccounts,
} from "@/utils/accountFilters";
import { getPurchasePPhStatus } from "@/utils/taxPaymentStatus";
// SSOT Journal Integration
import PurchaseJournalEntriesModal from "../../src/components/purchase/PurchaseJournalEntriesModal";
import purchaseJournalService from "../../src/services/purchaseJournalService";
import { API_ENDPOINTS } from "@/config/api";

// Types for form data
interface PurchaseFormData {
  vendor_id: string;
  purchase_request_id: string;
  project_id: string;
  date: string;
  due_date: string;
  notes: string;
  discount: string;

  // Legacy tax field (backward compatibility)
  tax: string;
  freight_cost: string;

  // Tax additions (Penambahan)
  ppn_rate: string;
  other_tax_additions: string;

  // Tax deductions (Pemotongan)
  pph21_rate: string;
  pph23_rate: string;
  other_tax_deductions: string;

  // Payment method fields
  payment_method: string;
  bank_account_id: string;
  credit_account_id: string; // New field for liability account
  payment_reference: string;

  items: PurchaseItemFormData[];
}

interface PurchaseItemFormData {
  product_id: string;
  purchase_request_item_id: string;
  quantity: string;
  unit_price: string;
  discount: string;
  tax: string;
  expense_account_id: string;
  item_type: string; // "INVENTORY" | "ASSET" | "PREPAID" | "EXPENSE"
  is_fixed_asset: boolean;
  description: string; // Asset name/description / Prepaid name
  asset_category_id: string; // Asset category ID
  amortization_month: string; // Durasi bulan amortisasi untuk PREPAID
}

interface Vendor {
  id: number;
  name: string;
  code: string;
}

interface BankAccount {
  id: number;
  name: string;
  code: string;
  type: string;
  account_id?: number;
  is_active?: boolean;
  balance?: number;
  currency: string;
  book_balance_idr?: number;
  last_exchange_rate?: number;
  frozen_amount?: number;
  account?: {
    id?: number;
    code?: string;
    name?: string;
    is_active?: boolean;
    is_header?: boolean;
  };
}

type PaymentAccountsErrorPayload = {
  error?: string | { message?: string };
  message?: string;
  required_permission?: string;
  module?: string;
  data?: BankAccount[];
};

class PaymentAccountsLoadError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "PaymentAccountsLoadError";
    this.status = status;
  }
}

const paymentAccountsErrorMessage = (
  status: number,
  payload?: PaymentAccountsErrorPayload,
) => {
  switch (status) {
    case 401:
      return "Sesi login Anda sudah berakhir. Silakan login kembali, lalu coba lagi.";
    case 403:
      return "Akun Anda belum memiliki izin View pada modul Cash & Bank. Minta admin mengaktifkannya melalui Manage Permissions agar daftar rekening dapat digunakan di Purchase.";
    case 404:
      return "Layanan daftar rekening Kas & Bank tidak ditemukan. Hubungi administrator sistem.";
    default:
      if (status >= 500) {
        return "Server belum dapat memuat daftar rekening Kas & Bank. Silakan coba lagi beberapa saat.";
      }

      if (typeof payload?.message === "string" && payload.message.trim()) {
        return `Daftar rekening Kas & Bank belum dapat dimuat: ${payload.message}`;
      }

      return "Daftar rekening Kas & Bank belum dapat dimuat. Silakan coba lagi.";
  }
};

const requestOperationalPaymentAccounts = async (
  token: string,
): Promise<BankAccount[]> => {
  let response: Response;
  try {
    response = await fetch(API_ENDPOINTS.CASH_BANK.PAYMENT_ACCOUNTS, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
    });
  } catch {
    throw new PaymentAccountsLoadError(
      0,
      "Tidak dapat terhubung ke server untuk memuat rekening Kas & Bank. Periksa koneksi Anda lalu coba lagi.",
    );
  }

  let payload: PaymentAccountsErrorPayload | undefined;
  try {
    payload = (await response.json()) as PaymentAccountsErrorPayload;
  } catch {
    payload = undefined;
  }

  if (!response.ok) {
    throw new PaymentAccountsLoadError(
      response.status,
      paymentAccountsErrorMessage(response.status, payload),
    );
  }

  return filterOperationalCashBankAccounts(
    Array.isArray(payload?.data) ? payload.data : [],
  );
};

const normalizePurchasePaymentFields = (
  current: PurchaseFormData,
  nextPaymentMethod: string,
): PurchaseFormData => {
  const normalizedMethod = nextPaymentMethod || "CREDIT";
  const nextState: PurchaseFormData = {
    ...current,
    payment_method: normalizedMethod,
  };

  if (normalizedMethod === "CREDIT") {
    nextState.bank_account_id = "";
    nextState.payment_reference = "";
    return nextState;
  }

  nextState.credit_account_id = "";
  if (normalizedMethod === "CASH") {
    nextState.payment_reference = "";
  }

  return nextState;
};

interface Receipt {
  id: number;
  receipt_number: string;
  received_date: string;
  status: string;
  notes: string;
  received_by: number;
  receiver: {
    id: number;
    name: string;
  };
  receipt_items: ReceiptItem[];
}

interface ReceiptItem {
  id: number;
  purchase_item_id: number;
  quantity_received: number;
  condition: string;
  notes: string;
  purchase_item: {
    id: number;
    quantity: number;
    item_type?: string;
    description?: string;
    product?: {
      id: number;
      name: string;
      code: string;
    };
  };
}

// Status color mapping
const getStatusColor = (status: string) => {
  switch (status.toLowerCase()) {
    case "approved":
    case "completed":
      return "green";
    case "draft":
    case "pending_approval":
      return "yellow";
    case "pending":
      return "blue";
    case "pending_accounting":
      return "orange";
    case "pending_final_approval":
      return "purple";
    case "cancelled":
    case "rejected":
      return "red";
    default:
      return "gray";
  }
};

// Approval status color mapping
const getApprovalStatusColor = (approvalStatus: string) => {
  switch ((approvalStatus || "").toLowerCase()) {
    case "approved":
      return "green";
    case "pending":
      return "yellow";
    case "rejected":
      return "red";
    case "not_required":
    case "not_started":
      return "gray";
    default:
      return "gray";
  }
};

const formatDateForInput = (date: Date) => {
  const year = date.getFullYear();
  const month = `${date.getMonth() + 1}`.padStart(2, "0");
  const day = `${date.getDate()}`.padStart(2, "0");
  return `${year}-${month}-${day}`;
};

const getCurrentMonthPurchaseRange = () => {
  const now = new Date();
  return {
    start_date: formatDateForInput(
      new Date(now.getFullYear(), now.getMonth(), 1),
    ),
    end_date: formatDateForInput(now),
  };
};

const getDefaultPurchaseFilters = (): PurchaseFilterParams => ({
  status: "",
  vendor_id: "",
  project_id: "",
  approval_status: "",
  accounting_status: "",
  search: "",
  start_date: "",
  end_date: "",
  page: 1,
  limit: 10,
});

const getAccountingMonitoringFilters = (): PurchaseFilterParams => ({
  ...getDefaultPurchaseFilters(),
  ...getCurrentMonthPurchaseRange(),
  accounting_status: "READY_FOR_COA",
});

// Format currency in IDR
const formatCurrency = (amount: number) => {
  const hasFraction = Math.abs((amount || 0) % 1) > Number.EPSILON;

  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    minimumFractionDigits: hasFraction ? 2 : 0,
    maximumFractionDigits: 2,
  }).format(amount);
};

const formatAccountBalance = (amount: number, currency?: string) => {
  const normalizedCurrency = (currency || "IDR").toUpperCase();

  if (normalizedCurrency === "USD") {
    return `USD ${amount.toLocaleString("en-US", {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    })}`;
  }

  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    minimumFractionDigits: Math.abs((amount || 0) % 1) > Number.EPSILON ? 2 : 0,
    maximumFractionDigits: 2,
  }).format(amount);
};

const getAccountBookBalance = (account?: Partial<BankAccount>) => {
  if (!account) return 0;

  const normalizedCurrency = (account.currency || "IDR").toUpperCase();
  const nativeBalance = account.balance || 0;

  if (normalizedCurrency === "IDR") {
    return nativeBalance;
  }

  if (typeof account.book_balance_idr === "number") {
    return account.book_balance_idr;
  }

  const fallbackRate =
    account.last_exchange_rate && account.last_exchange_rate > 0
      ? account.last_exchange_rate
      : 1;

  return nativeBalance * fallbackRate;
};

const formatBankAccountSummary = (account?: Partial<BankAccount>) => {
  if (!account) {
    return formatAccountBalance(0, "IDR");
  }

  const nativeLabel = formatAccountBalance(
    account.balance || 0,
    account.currency,
  );
  if ((account.currency || "IDR").toUpperCase() === "IDR") {
    return nativeLabel;
  }

  return `${nativeLabel} | Nilai buku: ${formatCurrency(getAccountBookBalance(account))}`;
};

// Format date
const formatDate = (date: string) => {
  return new Date(date).toLocaleDateString("id-ID");
};

const isReceiptablePurchaseItem = (item?: { item_type?: string | null }) => {
  const itemType = String(item?.item_type || "INVENTORY").toUpperCase();
  return itemType === "INVENTORY" || itemType === "ASSET";
};

const getReceiptablePurchaseItems = <T extends { item_type?: string | null }>(
  items?: T[] | null,
): T[] => {
  if (!Array.isArray(items)) return [];
  return items.filter(isReceiptablePurchaseItem);
};

const purchaseHasReceiptableItems = (purchase: Purchase) => {
  if (
    !Array.isArray(purchase.purchase_items) ||
    purchase.purchase_items.length === 0
  ) {
    // Fallback: if list endpoint doesn't include items, don't hide the action prematurely.
    return true;
  }

  return getReceiptablePurchaseItems(purchase.purchase_items).length > 0;
};

const receiptHasReceiptableItems = (receipt: Receipt) => {
  return (receipt.receipt_items || []).some((item) =>
    isReceiptablePurchaseItem(item.purchase_item),
  );
};

const createPurchaseItemFormData = (
  overrides: Partial<PurchaseItemFormData> = {},
): PurchaseItemFormData => ({
  product_id: "",
  purchase_request_item_id: "",
  quantity: "1",
  unit_price: "0",
  discount: "0",
  tax: "0",
  expense_account_id: "",
  item_type: "INVENTORY",
  is_fixed_asset: false,
  description: "",
  asset_category_id: "",
  amortization_month: "",
  ...overrides,
});

const mapPurchaseItemToFormData = (
  item: PurchaseItem,
): PurchaseItemFormData => {
  const itemType =
    item.item_type || (item.is_fixed_asset ? "ASSET" : "INVENTORY");
  const normalizedItemType = itemType.toUpperCase();
  const isDescriptiveItem =
    normalizedItemType === "EXPENSE" ||
    normalizedItemType === "ASSET" ||
    normalizedItemType === "PREPAID";
  const itemDescription =
    item.description?.trim() ||
    (isDescriptiveItem
      ? item.purchase_request_item?.item_name?.trim() ||
        item.product?.name?.trim()
      : "") ||
    "";

  return createPurchaseItemFormData({
    product_id: item.product_id?.toString() || "",
    purchase_request_item_id: item.purchase_request_item_id?.toString() || "",
    quantity: item.quantity?.toString() || "1",
    unit_price: item.unit_price?.toString() || "0",
    discount: item.discount?.toString() || "0",
    tax: item.tax?.toString() || "0",
    expense_account_id: item.expense_account_id?.toString() || "",
    item_type: itemType,
    is_fixed_asset: item.is_fixed_asset || itemType === "ASSET",
    description: itemDescription,
    asset_category_id: item.asset_category_id?.toString() || "",
    amortization_month:
      itemType === "PREPAID" ? item.amortization_month?.toString() || "12" : "",
  });
};

const getSelectablePurchaseRequestItems = (
  purchaseRequest?: PurchaseRequest | null,
  currentPurchaseId?: number | null,
) => {
  return (purchaseRequest?.items || []).filter((item) => {
    if (item.approval_status !== "APPROVED") return false;
    if (!item.purchase_id) return true;
    return currentPurchaseId != null && item.purchase_id === currentPurchaseId;
  });
};

const isPurchaseRequestSelectableForPurchase = (
  purchaseRequest: PurchaseRequest,
  currentPurchaseId?: number | null,
) =>
  purchaseRequest.status === "APPROVED" &&
  getSelectablePurchaseRequestItems(purchaseRequest, currentPurchaseId).length >
    0;

const getPurchaseRequestTitle = (purchaseRequest?: PurchaseRequest | null) => {
  const explicitTitle = purchaseRequest?.title?.trim();
  if (explicitTitle) return explicitTitle;

  const firstItem = purchaseRequest?.items?.[0];
  const firstItemName = firstItem?.product?.name || firstItem?.item_name;
  if (firstItemName?.trim()) return firstItemName.trim();

  return purchaseRequest?.notes?.trim() || "";
};

const getPurchaseRequestOptionLabel = (purchaseRequest: PurchaseRequest) => {
  const title = getPurchaseRequestTitle(purchaseRequest);
  const titlePart = title ? ` - ${title}` : "";
  const departmentPart = purchaseRequest.department
    ? ` - ${purchaseRequest.department}`
    : "";

  return `${purchaseRequest.pr_number}${titlePart}${departmentPart} (${formatCurrency(
    purchaseRequest.total_estimated_amount,
  )})`;
};

const mapPurchaseRequestItemToFormData = (
  item: PurchaseRequestLineItem,
  defaultExpenseAccountId: number | null,
) => {
  const isManualItem = !item.product_id;
  return createPurchaseItemFormData({
    product_id: item.product_id?.toString() || "",
    purchase_request_item_id: item.id?.toString() || "",
    quantity: item.quantity?.toString() || "1",
    unit_price: item.estimated_price?.toString() || "0",
    item_type: isManualItem ? "EXPENSE" : "INVENTORY",
    description: isManualItem ? item.item_name : "",
    expense_account_id: defaultExpenseAccountId
      ? defaultExpenseAccountId.toString()
      : "",
  });
};

const getPurchaseItemDisplayName = (item: PurchaseItem) => {
  if (item.is_fixed_asset || item.item_type === "ASSET") {
    return `${item.description || "Aset Tanpa Deskripsi"} (Aset Tetap)`;
  }

  if (item.item_type === "PREPAID") {
    return item.description || "Prepaid Tanpa Deskripsi";
  }

  if (item.item_type === "EXPENSE") {
    return item.description || "Beban/Jasa Tanpa Deskripsi";
  }

  return (
    item.product?.name ||
    (item.product_id ? `Product #${item.product_id}` : "Produk Tidak Ditemukan")
  );
};

const columns = [
  { header: "Code", accessor: "code" as keyof Purchase },
  {
    header: "Vendor",
    accessor: ((row: Purchase) => {
      return row.vendor?.name || "N/A";
    }) as (row: Purchase) => React.ReactNode,
  },
  {
    header: "Date",
    accessor: ((row: Purchase) => {
      return new Date(row.date).toLocaleDateString("id-ID");
    }) as (row: Purchase) => React.ReactNode,
  },
  {
    header: "Total",
    accessor: ((row: Purchase) => {
      return formatCurrency(row.total_amount);
    }) as (row: Purchase) => React.ReactNode,
  },
  {
    header: "Paid",
    accessor: ((row: Purchase) => {
      const paidAmount = row.paid_amount || 0;
      return (
        <Text
          color={paidAmount > 0 ? "green.600" : "gray.500"}
          fontWeight={paidAmount > 0 ? "semibold" : "normal"}
        >
          {formatCurrency(paidAmount)}
        </Text>
      );
    }) as (row: Purchase) => React.ReactNode,
  },
  {
    header: "Outstanding",
    accessor: ((row: Purchase) => {
      const outstandingAmount = row.outstanding_amount || 0;
      return (
        <Text
          color={outstandingAmount > 0 ? "orange.600" : "gray.500"}
          fontWeight={outstandingAmount > 0 ? "semibold" : "normal"}
        >
          {formatCurrency(outstandingAmount)}
        </Text>
      );
    }) as (row: Purchase) => React.ReactNode,
  },
  {
    header: "Payment",
    accessor: ((row: Purchase) => {
      const paymentMethod = row.payment_method || "CASH";
      const canReceivePayment = purchaseService.canReceivePayment(row);
      return (
        <VStack spacing={1} align="start">
          <Badge
            colorScheme={paymentMethod === "CREDIT" ? "blue" : "gray"}
            variant="subtle"
            fontSize="xs"
          >
            {paymentMethod}
          </Badge>
          {canReceivePayment && (
            <Badge colorScheme="green" variant="outline" fontSize="xs">
              Can Pay
            </Badge>
          )}
        </VStack>
      );
    }) as (row: Purchase) => React.ReactNode,
  },
  {
    header: "Status",
    accessor: ((row: Purchase) => (
      <Badge colorScheme={getStatusColor(row.status)} variant="subtle">
        {row.status.replace("_", " ").toUpperCase()}
      </Badge>
    )) as (row: Purchase) => React.ReactNode,
  },
  {
    header: "Approval Status",
    accessor: ((row: Purchase) => (
      <Badge
        colorScheme={getApprovalStatusColor(row.approval_status)}
        variant="subtle"
      >
        {row.approval_status.replace("_", " ").toUpperCase()}
      </Badge>
    )) as (row: Purchase) => React.ReactNode,
  },
];

const PurchasesPage: React.FC = () => {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { token, user } = useAuth();
  const isServiceCompany = user?.company_type === "SERVICE";
  const toast = useToast();
  const { t, language } = useTranslation();
  const {
    isOpen: isFilterOpen,
    onOpen: onFilterOpen,
    onClose: onFilterClose,
  } = useDisclosure();

  // Theme colors for enhanced styling - MUST be at top to follow Rules of Hooks
  const bgColor = useColorModeValue("gray.50", "gray.900");
  const cardBg = useColorModeValue("white", "gray.800");
  const headingColor = useColorModeValue("gray.800", "gray.100");
  const borderColor = useColorModeValue("gray.200", "gray.600");

  // Pre-calculate all useColorModeValue calls to avoid conditional hook calls
  const textSecondary = useColorModeValue("gray.600", "gray.400");
  const textPrimary = useColorModeValue("gray.700", "gray.200");
  const hoverBg = useColorModeValue("gray.50", "gray.700");
  const hoverBorder = useColorModeValue("gray.300", "gray.500");
  const buttonBlueBg = useColorModeValue("blue.500", "blue.600");
  const buttonBlueHover = useColorModeValue("blue.600", "blue.500");
  const statColors = {
    orange: useColorModeValue("orange.600", "orange.400"),
    green: useColorModeValue("green.600", "green.400"),
    red: useColorModeValue("red.600", "red.400"),
    purple: useColorModeValue("purple.600", "purple.400"),
    blue: useColorModeValue("blue.600", "blue.300"),
  };
  const statBgColors = {
    orange: useColorModeValue("orange.50", "orange.900"),
    green: useColorModeValue("green.50", "green.900"),
    red: useColorModeValue("red.50", "red.900"),
    purple: useColorModeValue("purple.50", "purple.900"),
    blue: useColorModeValue("blue.50", "blue.900"),
  };
  const modalBg = useColorModeValue("white", "gray.900");
  const modalFilterBg = useColorModeValue("blue.50", "blue.900");
  const modalFilterColor = useColorModeValue("blue.600", "blue.300");
  const modalHoverBg = useColorModeValue("gray.100", "gray.700");
  const inputHoverBorder = useColorModeValue("gray.300", "gray.500");
  const inputFocusBorder = useColorModeValue("blue.500", "blue.400");
  const inputFocusShadow = `0 0 0 1px ${useColorModeValue("blue.500", "blue.400")}`;
  const ghostHoverBg = useColorModeValue("gray.50", "gray.700");

  // Additional colors for view modal
  const modalContentBg = useColorModeValue("white", "gray.800");
  const modalHeaderBg = useColorModeValue("gray.50", "gray.700");
  const notesBoxBg = useColorModeValue("gray.50", "gray.600");
  const tableBg = useColorModeValue("white", "gray.700");
  const tableHeaderBg = useColorModeValue("gray.50", "gray.600");
  const inputBg = useColorModeValue("white", "whiteAlpha.50");
  const inputTextColor = useColorModeValue("gray.800", "whiteAlpha.900");
  const inputPlaceholderColor = useColorModeValue("gray.500", "whiteAlpha.500");
  const inputBorderColor = useColorModeValue("gray.300", "whiteAlpha.300");
  const readOnlyInputBg = useColorModeValue("gray.100", "whiteAlpha.100");
  const helperTextColor = useColorModeValue("gray.500", "whiteAlpha.700");
  const warningTextColor = useColorModeValue("orange.600", "orange.300");
  const employeeInfoBg = useColorModeValue("blue.50", "blue.900");
  const employeeInfoBorder = useColorModeValue("blue.200", "blue.700");
  const employeeInfoText = useColorModeValue("blue.700", "blue.100");
  const greenButtonHoverBg = useColorModeValue(
    "green.50",
    "rgba(56, 161, 105, 0.16)",
  );
  const blueButtonHoverBg = useColorModeValue(
    "blue.50",
    "rgba(66, 153, 225, 0.16)",
  );
  const redButtonHoverBg = useColorModeValue(
    "red.50",
    "rgba(229, 62, 62, 0.16)",
  );
  const inventoryTypeBg = useColorModeValue(
    "blue.50",
    "rgba(49, 130, 206, 0.18)",
  );
  const inventoryTypeHoverBg = useColorModeValue(
    "blue.100",
    "rgba(49, 130, 206, 0.28)",
  );
  const inventoryTypeBorder = useColorModeValue("blue.300", "blue.500");
  const inventoryTypeText = useColorModeValue("blue.700", "blue.200");
  const assetTypeBg = useColorModeValue(
    "orange.50",
    "rgba(221, 107, 32, 0.18)",
  );
  const assetTypeHoverBg = useColorModeValue(
    "orange.100",
    "rgba(221, 107, 32, 0.28)",
  );
  const assetTypeBorder = useColorModeValue("orange.300", "orange.500");
  const assetTypeText = useColorModeValue("orange.700", "orange.200");
  const prepaidTypeBg = useColorModeValue(
    "purple.50",
    "rgba(128, 90, 213, 0.18)",
  );
  const prepaidTypeHoverBg = useColorModeValue(
    "purple.100",
    "rgba(128, 90, 213, 0.28)",
  );
  const prepaidTypeBorder = useColorModeValue("purple.300", "purple.500");
  const prepaidTypeText = useColorModeValue("purple.700", "purple.200");
  const fieldFocusStyles = {
    borderColor: inputFocusBorder,
    boxShadow: inputFocusShadow,
  };

  // Tooltip descriptions for purchase page
  const tooltips = {
    search: t("purchases.tooltips.search"),
    vendor: t("purchases.tooltips.vendor"),
    status: t("purchases.tooltips.status"),
    approvalStatus: t("purchases.tooltips.approvalStatus"),
    date: t("purchases.tooltips.date"),
    dueDate: t("purchases.tooltips.dueDate"),
    paymentMethod: t("purchases.tooltips.paymentMethod"),
    bankAccount: t("purchases.tooltips.bankAccount"),
    creditAccount: t("purchases.tooltips.creditAccount"),
    ppnRate: t("purchases.tooltips.ppnRate"),
    otherTaxAdditions: t("purchases.tooltips.otherTaxAdditions"),
    pph21Rate: t("purchases.tooltips.pph21Rate"),
    pph23Rate: t("purchases.tooltips.pph23Rate"),
    otherTaxDeductions: t("purchases.tooltips.otherTaxDeductions"),
    discount: t("purchases.tooltips.discount"),
    product: t("purchases.tooltips.product"),
    quantity: t("purchases.tooltips.quantity"),
    unitPrice: t("purchases.tooltips.unitPrice"),
    expenseAccount: t("purchases.tooltips.expenseAccount"),
    notes: t("purchases.tooltips.notes"),
    reference: t("purchases.tooltips.reference"),
  };

  // State management
  const [purchases, setPurchases] = useState<Purchase[]>([]);
  const [allPurchases, setAllPurchases] = useState<Purchase[]>([]); // Store all purchases for client-side filtering
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [pagination, setPagination] = useState({
    page: 1,
    limit: 10,
    total: 0,
    totalPages: 0,
  });

  // Filter state
  const [filters, setFilters] = useState<PurchaseFilterParams>(
    getDefaultPurchaseFilters,
  );
  const bootstrappedPurchaseTokenRef = useRef<string | null>(null);

  // Local search state (client-side, no debouncing needed)
  const [searchInput, setSearchInput] = useState("");

  // Statistics state
  const [stats, setStats] = useState({
    total: 0,
    pending: 0,
    approved: 0,
    rejected: 0,
    needingApproval: 0,
    totalValue: 0,
    totalApprovedAmount: 0,
    totalPaid: 0,
    totalOutstanding: 0,
  });

  // Local UI helpers to reflect receipt availability/completion immediately after creation
  const [purchasesWithReceipts, setPurchasesWithReceipts] = useState<
    Set<number>
  >(new Set());
  const [fullyReceivedPurchases, setFullyReceivedPurchases] = useState<
    Set<number>
  >(new Set());
  const [remainingQtyMap, setRemainingQtyMap] = useState<
    Record<number, number>
  >({});

  // View and Edit Modal states
  const {
    isOpen: isViewOpen,
    onOpen: onViewOpen,
    onClose: onViewClose,
  } = useDisclosure();
  const {
    isOpen: isEditOpen,
    onOpen: onEditOpen,
    onClose: onEditClose,
  } = useDisclosure();
  const {
    isOpen: isCreateOpen,
    onOpen: onCreateOpen,
    onClose: onCreateClose,
  } = useDisclosure();

  // Receipt Modal states
  const {
    isOpen: isReceiptOpen,
    onOpen: onReceiptOpen,
    onClose: onReceiptClose,
  } = useDisclosure();

  // Payment Modal states
  const {
    isOpen: isPaymentOpen,
    onOpen: onPaymentOpen,
    onClose: onPaymentClose,
  } = useDisclosure();

  const [selectedPurchase, setSelectedPurchase] = useState<Purchase | null>(
    null,
  );
  const [selectedPurchaseForPayment, setSelectedPurchaseForPayment] =
    useState<Purchase | null>(null);
  const {
    isOpen: isPPhPaymentOpen,
    onOpen: onPPhPaymentOpen,
    onClose: onPPhPaymentClose,
  } = useDisclosure();
  const [selectedPurchaseForPPh, setSelectedPurchaseForPPh] =
    useState<Purchase | null>(null);
  const [pphPaymentForm, setPPhPaymentForm] = useState({
    payment_date: formatDateForInput(new Date()),
    amount: "",
    cash_bank_id: "",
    reference: "",
    notes: "",
  });
  const [savingPPhPayment, setSavingPPhPayment] = useState(false);
  const [purchaseRequests, setPurchaseRequests] = useState<PurchaseRequest[]>(
    [],
  );
  const [projects, setProjects] = useState<any[]>([]);
  const [loadingPRs, setLoadingPRs] = useState(false);
  const [loadingProjects, setLoadingProjects] = useState(false);
  const [vendors, setVendors] = useState<Vendor[]>([]);
  const [products, setProducts] = useState<Product[]>([]);
  const [expenseAccounts, setExpenseAccounts] = useState<GLAccount[]>([]);
  const [fixedAssetAccounts, setFixedAssetAccounts] = useState<GLAccount[]>([]);
  const [loadingFixedAssetAccounts, setLoadingFixedAssetAccounts] =
    useState(false);
  const [bankAccounts, setBankAccounts] = useState<BankAccount[]>([]);
  const [creditAccounts, setCreditAccounts] = useState<GLAccount[]>([]); // New state for liability accounts
  const [cashBanks, setCashBanks] = useState<any[]>([]); // For payment form
  const [loadingExpenseAccounts, setLoadingExpenseAccounts] = useState(false);
  const [defaultExpenseAccountId, setDefaultExpenseAccountId] = useState<
    number | null
  >(null);
  const [canListExpenseAccounts, setCanListExpenseAccounts] = useState(true);
  const [loadingBankAccounts, setLoadingBankAccounts] = useState(false);
  const [loadingCreditAccounts, setLoadingCreditAccounts] = useState(false); // New loading state
  const [defaultTaxRate, setDefaultTaxRate] = useState<number>(0);
  const [formData, setFormData] = useState<PurchaseFormData>({
    vendor_id: "",
    purchase_request_id: "",
    project_id: "",
    date: new Date().toISOString().split("T")[0],
    due_date: "",
    notes: "",
    discount: "0",

    // Legacy tax field
    tax: "0",

    // Tax additions (Penambahan)
    ppn_rate: "0",
    other_tax_additions: "0",
    freight_cost: "0",

    // Tax deductions (Pemotongan)
    pph21_rate: "0",
    pph23_rate: "0",
    other_tax_deductions: "0",

    // Payment method fields
    payment_method: "CREDIT",
    bank_account_id: "",
    credit_account_id: "", // New field for liability account
    payment_reference: "",

    items: [],
  });
  const [loadingVendors, setLoadingVendors] = useState(false);
  const [loadingProducts, setLoadingProducts] = useState(false);
  const [assetCategories, setAssetCategories] = useState<any[]>([]);
  const [loadingAssetCategories, setLoadingAssetCategories] = useState(false);
  const selectedPurchaseRequest =
    purchaseRequests.find(
      (pr) => pr.id.toString() === formData.purchase_request_id,
    ) || null;
  const selectablePurchaseRequestItems = getSelectablePurchaseRequestItems(
    selectedPurchaseRequest,
    selectedPurchase?.id ?? null,
  );

  // Add Vendor Modal states
  const {
    isOpen: isAddVendorOpen,
    onOpen: onAddVendorOpen,
    onClose: onAddVendorClose,
  } = useDisclosure();
  const [newVendorData, setNewVendorData] = useState({
    name: "",
    code: "",
    email: "",
    phone: "",
    mobile: "",
    address: "",
    pic_name: "",
    external_id: "",
    notes: "",
  });
  const [savingVendor, setSavingVendor] = useState(false);

  // Add Product Modal states
  const {
    isOpen: isAddProductOpen,
    onOpen: onAddProductOpen,
    onClose: onAddProductClose,
  } = useDisclosure();
  const [newProductData, setNewProductData] = useState({
    name: "",
    code: "",
    description: "",
    unit: "",
    purchase_price: "0",
    sale_price: "0",
  });
  const [savingProduct, setSavingProduct] = useState(false);

  // Receipt form state
  const [receiptFormData, setReceiptFormData] = useState({
    received_date: new Date().toISOString().split("T")[0],
    notes: "",
    receipt_items: [] as Array<{
      purchase_item_id: number;
      quantity_received: number;
      condition: string;
      notes: string;
      // NEW: Auto Asset Creation Fields
      create_asset?: boolean;
      asset_category?: string;
      asset_useful_life?: number;
      asset_salvage_percentage?: number;
      serial_number?: string;
    }>,
  });
  const [savingReceipt, setSavingReceipt] = useState(false);

  // Journal Modal states
  const {
    isOpen: isJournalOpen,
    onOpen: onJournalOpen,
    onClose: onJournalClose,
  } = useDisclosure();
  const [selectedPurchaseForJournal, setSelectedPurchaseForJournal] =
    useState<Purchase | null>(null);

  // Finance Tax/Payment Config Modal states
  const {
    isOpen: isFinanceTaxOpen,
    onOpen: onFinanceTaxOpen,
    onClose: onFinanceTaxClose,
  } = useDisclosure();
  const [financeTaxPurchase, setFinanceTaxPurchase] = useState<Purchase | null>(
    null,
  );
  const [financeTaxForm, setFinanceTaxForm] = useState({
    ppn_rate: "0",
    other_tax_additions: "0",
    freight_cost: "0",
    pph21_rate: "0",
    pph23_rate: "0",
    other_tax_deductions: "0",
    payment_method: "CREDIT",
    bank_account_id: "",
    credit_account_id: "",
    payment_reference: "",
  });
  const [savingFinanceTax, setSavingFinanceTax] = useState(false);

  // Accounting COA completion modal state
  const {
    isOpen: isAccountingCOAOpen,
    onOpen: onAccountingCOAOpen,
    onClose: onAccountingCOAClose,
  } = useDisclosure();
  const [accountingCOAPurchase, setAccountingCOAPurchase] =
    useState<Purchase | null>(null);
  const [accountingCOAForm, setAccountingCOAForm] = useState({
    ppn_rate: "0",
    other_tax_additions: "0",
    freight_cost: "0",
    pph21_rate: "0",
    pph23_rate: "0",
    other_tax_deductions: "0",
    payment_method: "CREDIT",
    bank_account_id: "",
    credit_account_id: "",
    payment_reference: "",
    items: [] as Array<{
      purchase_item_id: number;
      label: string;
      quantity: number;
      total_price: number;
      expense_account_id: string;
    }>,
  });
  const [savingAccountingCOA, setSavingAccountingCOA] = useState(false);

  // Receipts modal state
  const {
    isOpen: isReceiptsOpen,
    onOpen: onReceiptsOpen,
    onClose: onReceiptsClose,
  } = useDisclosure();
  const [receipts, setReceipts] = useState<Receipt[]>([]);
  const [loadingReceipts, setLoadingReceipts] = useState(false);
  const [selectedPurchaseForReceipts, setSelectedPurchaseForReceipts] =
    useState<Purchase | null>(null);
  const handledNotificationPurchaseId = useRef<string | null>(null);

  // Role-based permissions (kept for approval workflow logic)
  const roleNorm = normalizeRole(user?.role as any);
  const isFinanceOperationalRole = isFinanceTeamRole(roleNorm);
  const isFinanceOrAdmin = roleNorm === "admin" || isFinanceOperationalRole;

  // Permission-based access control
  const { canCreate, canEdit, canDelete, canExport, canApprove } =
    useModulePermissions("purchases");

  const logPaymentAccountsError = (error: unknown) => {
    const loadError =
      error instanceof PaymentAccountsLoadError
        ? error
        : new PaymentAccountsLoadError(
            0,
            "Daftar rekening Kas & Bank belum dapat dimuat. Silakan coba lagi.",
          );
    console.warn("[Purchases] Payment accounts unavailable", {
      status: loadError.status,
      message: loadError.message,
    });
  };

  const isAccounting = isAccountingRole(roleNorm);
  const accountingMonitoringEnabled =
    isAccounting || isFinanceManagerRole(roleNorm) || roleNorm === "admin";
  const canSelectAccountFromCOA =
    roleNorm === "admin" || isAccounting;

  const getDefaultPurchaseItemAccount = useCallback(
    (itemType?: string) => {
      const normalizedType = (itemType || "INVENTORY").toUpperCase();
      const matchAccount = (predicate: (account: GLAccount) => boolean) =>
        expenseAccounts.find((account) => predicate(account));

      if (normalizedType === "ASSET") {
        const fixedAssetAccount = matchAccount((account) => {
          const code = account.code || "";
          const name = (account.name || "").toLowerCase();
          return (
            (code.startsWith("15") && code.length >= 4) ||
            ["1501", "1502", "1503", "1504", "1505", "1509"].includes(code) ||
            name.includes("fixed asset") ||
            name.includes("asset tetap") ||
            name.includes("peralatan") ||
            name.includes("mesin") ||
            name.includes("printer") ||
            name.includes("komputer") ||
            name.includes("equipment") ||
            name.includes("furniture") ||
            name.includes("kendaraan") ||
            name.includes("bangunan") ||
            name.includes("gedung")
          );
        });
        if (fixedAssetAccount) {
          return fixedAssetAccount;
        }
      }

      if (normalizedType === "PREPAID") {
        const prepaidAccount = matchAccount((account) => {
          const code = account.code || "";
          const name = (account.name || "").toLowerCase();
          const isPrepaidAsset =
            code.startsWith("1104") ||
            code.startsWith("1105") ||
            code.startsWith("1106") ||
            code.startsWith("1107") ||
            code.startsWith("1108") ||
            code.startsWith("1109") ||
            name.includes("prepaid") ||
            name.includes("dibayar dimuka") ||
            name.includes("uang muka") ||
            name.includes("bayar dimuka") ||
            name.includes("sewa dibayar") ||
            name.includes("asuransi dibayar");
          const isOtherCurrentAsset =
            code.startsWith("11") &&
            code.length >= 4 &&
            !code.startsWith("1101") &&
            !code.startsWith("1102") &&
            !code.startsWith("1103");
          return isPrepaidAsset || isOtherCurrentAsset;
        });
        if (prepaidAccount) {
          return prepaidAccount;
        }
      }

      if (normalizedType === "EXPENSE") {
        const expenseAccount = matchAccount(
          (account) =>
            String(account.type || "").toUpperCase() === "EXPENSE",
        );
        if (expenseAccount) {
          return expenseAccount;
        }
      }

      const inventoryOrExpenseAccount = matchAccount((account) => {
        const code = account.code || "";
        const name = (account.name || "").toLowerCase();
        const isInventory =
          code.startsWith("1103") || name.includes("persediaan");
        if (isInventory) {
          return true;
        }

        return (
          isServiceCompany &&
          String(account.type || "").toUpperCase() === "EXPENSE"
        );
      });

      if (inventoryOrExpenseAccount) {
        return inventoryOrExpenseAccount;
      }

      if (defaultExpenseAccountId) {
        return (
          expenseAccounts.find(
            (account) => account.id === defaultExpenseAccountId,
          ) || null
        );
      }

      return expenseAccounts[0] || null;
    },
    [defaultExpenseAccountId, expenseAccounts, isServiceCompany],
  );

  const getDefaultPurchaseItemAccountId = useCallback(
    (itemType?: string) => getDefaultPurchaseItemAccount(itemType)?.id || null,
    [getDefaultPurchaseItemAccount],
  );

  const getDefaultPurchaseItemAccountLabel = useCallback(
    (itemType?: string) => {
      const account = getDefaultPurchaseItemAccount(itemType);
      return account ? `${account.code} - ${account.name}` : "default sistem";
    },
    [getDefaultPurchaseItemAccount],
  );

  const resolvePurchaseItemExpenseAccountId = useCallback(
    (item: PurchaseItemFormData) =>
      canSelectAccountFromCOA
        ? item.expense_account_id
          ? parseInt(item.expense_account_id, 10)
          : getDefaultPurchaseItemAccountId(item.item_type) || undefined
        : undefined,
    [canSelectAccountFromCOA, getDefaultPurchaseItemAccountId],
  );

  // Fetch system settings for default tax rate
  useEffect(() => {
    const fetchSettings = async () => {
      try {
        const api = (await import("@/services/api")).default;
        const response = await api.get("/api/v1/settings/public");
        if (response.data?.data?.default_tax_rate !== undefined) {
          const rate = response.data.data.default_tax_rate;
          console.log(
            "PurchasesPage: Using default tax rate from settings:",
            rate,
          );
          setDefaultTaxRate(rate);
          // Set initial form ppn_rate to match settings
          setFormData((prev) => ({ ...prev, ppn_rate: rate.toString() }));
        }
      } catch (err) {
        console.warn(
          "PurchasesPage: Failed to load system settings, using default tax rate: 0",
        );
      }
    };
    if (token) fetchSettings();
  }, [token]);

  // Helper function to notify directors
  const notifyDirectors = async (purchase: Purchase) => {
    try {
      // This would typically call a notification service API
      // For now, we'll just log it as the notification is handled by backend
      console.log("Director notification sent for purchase:", purchase.code);
    } catch (err) {
      console.error("Error sending director notification:", err);
    }
  };

  // Handle add new vendor
  const handleAddVendor = async () => {
    if (!newVendorData.name.trim()) {
      toast({
        title: t("common.error"),
        description: t("contacts.validation.nameRequired"),
        status: "error",
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    if (!newVendorData.email.trim()) {
      toast({
        title: t("common.error"),
        description:
          t("contacts.validation.emailRequired") || "Vendor email is required",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    const normalizedEmail = newVendorData.email.trim();
    const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    if (!emailPattern.test(normalizedEmail)) {
      toast({
        title: t("common.error"),
        description: t("contacts.validation.emailInvalid"),
        status: "error",
        duration: 4000,
        isClosable: true,
      });
      return;
    }

    try {
      setSavingVendor(true);

      // Generate unique vendor code if not provided
      let vendorCode = newVendorData.code.trim();
      if (!vendorCode) {
        // Generate code based on name + timestamp + random to ensure uniqueness
        const namePrefix = newVendorData.name
          .trim()
          .substring(0, 3)
          .toUpperCase()
          .replace(/[^A-Z]/g, "X");
        const timestamp = Date.now().toString().slice(-6); // Last 6 digits of timestamp
        const random = Math.floor(Math.random() * 1000)
          .toString()
          .padStart(3, "0"); // 3-digit random
        vendorCode = `V${namePrefix}${timestamp}${random}`;
      } else {
        // Check if manually entered code already exists in current vendors list
        const existingVendor = vendors.find(
          (v) => v.code.toLowerCase() === vendorCode.toLowerCase(),
        );
        if (existingVendor) {
          toast({
            title: t("common.error"),
            description: t("contacts.validation.vendorCodeExists"),
            status: "error",
            duration: 5000,
            isClosable: true,
          });
          return;
        }
      }

      const vendorPayload = {
        name: newVendorData.name.trim(),
        email: normalizedEmail,
        phone: newVendorData.phone.trim(),
        mobile: newVendorData.mobile.trim(),
        address: newVendorData.address.trim(),
        pic_name: newVendorData.pic_name.trim(),
        external_id: newVendorData.external_id.trim(),
        notes: newVendorData.notes.trim(),
        code: vendorCode,
      };

      console.log("Creating vendor with payload:", vendorPayload);

      let newVendor;
      try {
        newVendor = await contactService.createVendorForPurchase(
          token!,
          vendorPayload,
        );
        console.log("Vendor creation response:", newVendor);

        // Check if the response indicates an error (some APIs return error in success response)
        if (
          newVendor &&
          typeof newVendor === "object" &&
          "error" in newVendor
        ) {
          throw new Error(
            (newVendor.error as string) || "Server returned an error",
          );
        }
      } catch (createError: any) {
        console.error("API Error creating vendor:", createError);
        throw new Error(
          createError.message ||
            createError.response?.data?.error ||
            "Failed to create vendor: Server error",
        );
      }

      // Validate that the new vendor was created successfully
      // Handle different response structures
      const vendorData = newVendor;

      // Additional checks for undefined response
      if (!newVendor) {
        console.error("Vendor creation returned undefined response");
        throw new Error(
          "Failed to create vendor: Server returned no response. Please try again.",
        );
      }

      if (!vendorData || !vendorData.id) {
        console.error("Invalid vendor response:", newVendor);
        console.error("Expected vendor data with id field, got:", vendorData);
        throw new Error(
          "Failed to create vendor: Invalid response structure from server. Please check console for details.",
        );
      }

      // Use the validated vendor data
      const vendorId = vendorData.id;
      const vendorName = vendorData.name;
      const finalVendorCode = vendorData.code || `V${vendorId}`;

      // Add the new vendor to the vendors list
      const formattedVendor = {
        id: vendorId,
        name: vendorName,
        code: finalVendorCode,
      };

      console.log("Adding formatted vendor to list:", formattedVendor);
      setVendors((prev) => [...prev, formattedVendor]);

      // Select the new vendor in the form
      setFormData((prev) => ({ ...prev, vendor_id: vendorId.toString() }));

      // Reset form and close modal
      setNewVendorData({
        name: "",
        code: "",
        email: "",
        phone: "",
        mobile: "",
        address: "",
        pic_name: "",
        external_id: "",
        notes: "",
      });

      onAddVendorClose();

      toast({
        title: t("common.success"),
        description:
          t("contacts.messages.createSuccess", { name: vendorName }) ||
          `Vendor "${vendorName}" created successfully and selected`,
        status: "success",
        duration: 3000,
        isClosable: true,
      });
    } catch (err: any) {
      console.error("Error creating vendor:", err);
      const rawMessage =
        err?.response?.data?.error ||
        (err instanceof Error ? err.message : "");
      const normalizedMessage = rawMessage.toLowerCase();
      let userMessage = rawMessage;

      if (normalizedMessage.includes("name") && normalizedMessage.includes("required")) {
        userMessage = t("contacts.validation.vendorNameRequired");
      } else if (normalizedMessage.includes("email") && normalizedMessage.includes("required")) {
        userMessage = t("contacts.validation.emailRequired");
      } else if (normalizedMessage.includes("email") && normalizedMessage.includes("invalid")) {
        userMessage = t("contacts.validation.emailInvalid");
      } else if (
        normalizedMessage.includes("duplicate") ||
        normalizedMessage.includes("already in use") ||
        normalizedMessage.includes("already exists")
      ) {
        userMessage = t("contacts.validation.vendorCodeExists");
      } else if (
        normalizedMessage.includes("unauthorized") ||
        normalizedMessage.includes("authentication token")
      ) {
        userMessage = t("contacts.messages.sessionExpired");
      } else if (
        normalizedMessage.includes("forbidden") ||
        normalizedMessage.includes("permission")
      ) {
        userMessage = t("contacts.messages.permissionDenied");
      } else if (
        normalizedMessage.includes("too long") ||
        normalizedMessage.includes("maximum length")
      ) {
        userMessage = t("contacts.validation.fieldTooLong");
      } else if (
        !rawMessage ||
        normalizedMessage.includes("server error") ||
        normalizedMessage.includes("unable to save") ||
        normalizedMessage.includes("network error") ||
        normalizedMessage.includes("failed to fetch") ||
        normalizedMessage.includes("invalid response") ||
        normalizedMessage.includes("no response")
      ) {
        userMessage = t("contacts.messages.saveFailedFriendly");
      }

      toast({
        title: t("common.error"),
        description: userMessage || t("contacts.messages.saveFailedFriendly"),
        status: "error",
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setSavingVendor(false);
    }
  };

  // Handle add new product
  const handleAddProduct = async () => {
    if (!newProductData.name.trim()) {
      toast({
        title: "Error",
        description: "Product name is required",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    if (!newProductData.unit.trim()) {
      toast({
        title: "Error",
        description: "Product unit is required",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    try {
      setSavingProduct(true);

      const generatedProductCode =
        newProductData.code.trim() || `PRD-${Date.now()}`;

      const productPayload: Product = {
        name: newProductData.name,
        code: generatedProductCode,
        description: newProductData.description || undefined,
        unit: newProductData.unit,
        purchase_price: parseFloat(newProductData.purchase_price) || 0,
        cost_price: parseFloat(newProductData.purchase_price) || 0,
        sale_price: parseFloat(newProductData.sale_price) || 0,
        stock: 0,
        min_stock: 0,
        max_stock: 0,
        reorder_level: 0,
        is_active: true,
        is_service: false,
        taxable: false,
      };

      const newProduct = await productService.createProduct(productPayload);
      const createdProduct = (
        "data" in newProduct ? newProduct.data : newProduct
      ) as Product;

      // Add the new product to the products list
      setProducts((prev) => [...prev, createdProduct]);

      // Select the new product in the form if we have items
      if (formData.items.length > 0) {
        const items = [...formData.items];
        items[0] = {
          ...items[0],
          product_id: createdProduct.id?.toString() || "",
          unit_price: createdProduct.purchase_price?.toString() || "0",
        };
        setFormData({ ...formData, items });
      } else {
        // Add a new item with the created product
        setFormData({
          ...formData,
          items: [
            createPurchaseItemFormData({
              product_id: createdProduct.id?.toString() || "",
              unit_price: createdProduct.purchase_price?.toString() || "0",
              expense_account_id: defaultExpenseAccountId
                ? defaultExpenseAccountId.toString()
                : "",
            }),
          ],
        });
      }

      // Reset form and close modal
      setNewProductData({
        name: "",
        code: "",
        description: "",
        unit: "",
        purchase_price: "0",
        sale_price: "0",
      });

      onAddProductClose();

      toast({
        title: "Success",
        description: `Product "${newProduct.data.name}" created successfully and selected`,
        status: "success",
        duration: 3000,
        isClosable: true,
      });
    } catch (err: any) {
      console.error("Error creating product:", err);
      toast({
        title: "Error",
        description: err.response?.data?.error || "Failed to create product",
        status: "error",
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setSavingProduct(false);
    }
  };

  // Fetch purchases from API
  const fetchPurchases = async (
    filterParams: PurchaseFilterParams = filters,
  ) => {
    if (!token) return;

    try {
      setLoading(true);
      const response = await purchaseService.list(filterParams);

      // Ensure response data is an array
      const purchaseData = Array.isArray(response?.data) ? response.data : [];

      // Store all purchases for client-side filtering
      setAllPurchases(purchaseData);
      setPurchases(purchaseData);
      setPagination({
        page: response?.page || 1,
        limit: response?.limit || 10,
        total: response?.total || 0,
        totalPages: response?.total_pages || 0,
      });

      // Fetch summary with the SAME filters as the table so cards stay consistent
      let summaryResponse: PurchaseSummary | null = null;
      try {
        summaryResponse = await purchaseService.getSummary({
          status: filterParams.status,
          vendor_id: filterParams.vendor_id,
          project_id: filterParams.project_id,
          start_date: filterParams.start_date,
          end_date: filterParams.end_date,
          search: filterParams.search,
          approval_status: filterParams.approval_status,
          accounting_status: filterParams.accounting_status,
          requires_approval: filterParams.requires_approval,
          outstanding: filterParams.outstanding,
        });
      } catch (summaryErr) {
        console.warn("Failed to fetch purchase summary:", summaryErr);
      }

      const statusCounts = summaryResponse?.status_counts || {};
      const approvalCounts = summaryResponse?.approval_status_counts || {};

      const pendingApproval =
        approvalCounts.PENDING ||
        purchaseData.filter((p) => {
          const approvalStatus = (p?.approval_status || "").toUpperCase();
          const status = (p?.status || "").toUpperCase();
          return (
            approvalStatus === "PENDING" ||
            (!!p?.requires_approval &&
              approvalStatus !== "APPROVED" &&
              approvalStatus !== "REJECTED" &&
              status !== "CANCELLED")
          );
        }).length;

      const approved =
        approvalCounts.APPROVED ||
        purchaseData.filter(
          (p) => (p?.approval_status || "").toUpperCase() === "APPROVED",
        ).length;

      const rejected =
        Math.max(approvalCounts.REJECTED || 0, statusCounts.CANCELLED || 0) ||
        purchaseData.filter((p) => {
          const approvalStatus = (p?.approval_status || "").toUpperCase();
          const status = (p?.status || "").toUpperCase();
          return approvalStatus === "REJECTED" || status === "CANCELLED";
        }).length;

      const totalPurchases =
        summaryResponse?.total_purchases ??
        response?.total ??
        purchaseData.length;
      const totalValue = summaryResponse?.total_amount ?? 0;
      const totalPaid = summaryResponse?.total_paid ?? 0;
      const totalOutstanding = summaryResponse?.total_outstanding ?? 0;
      const totalApprovedAmount = summaryResponse?.total_approved_amount ?? 0;

      if (!summaryResponse) {
        const fallbackTotalValue = purchaseData.reduce((sum, p) => {
          const amount = p?.total_amount || 0;
          return (
            sum +
            (typeof amount === "number" ? amount : parseFloat(amount) || 0)
          );
        }, 0);

        const fallbackTotalPaid = purchaseData.reduce((sum, p) => {
          const amount = p?.paid_amount || 0;
          return (
            sum +
            (typeof amount === "number" ? amount : parseFloat(amount) || 0)
          );
        }, 0);

        const fallbackTotalOutstanding = purchaseData.reduce((sum, p) => {
          const amount = p?.outstanding_amount || 0;
          return (
            sum +
            (typeof amount === "number" ? amount : parseFloat(amount) || 0)
          );
        }, 0);

        setStats({
          total: response?.total || totalPurchases,
          pending: pendingApproval,
          approved: approved,
          rejected: rejected,
          needingApproval: pendingApproval,
          totalValue: fallbackTotalValue,
          totalApprovedAmount: 0,
          totalPaid: fallbackTotalPaid,
          totalOutstanding: fallbackTotalOutstanding,
        });
        return;
      }

      setStats({
        total: totalPurchases,
        pending: pendingApproval,
        approved: approved,
        rejected: rejected,
        needingApproval: pendingApproval,
        totalValue: totalValue,
        totalApprovedAmount: totalApprovedAmount,
        totalPaid: totalPaid,
        totalOutstanding: totalOutstanding,
      });

      setError(null);
    } catch (err: any) {
      console.error("Error fetching purchases:", err);

      // Set empty state on error
      setPurchases([]);
      setPagination({
        page: 1,
        limit: 10,
        total: 0,
        totalPages: 0,
      });

      setStats({
        total: 0,
        pending: 0,
        approved: 0,
        rejected: 0,
        needingApproval: 0,
        totalValue: 0,
        totalApprovedAmount: 0,
        totalPaid: 0,
        totalOutstanding: 0,
      });

      const errorMessage =
        err.response?.data?.message ||
        err.message ||
        "Failed to fetch purchases";
      setError(errorMessage);

      toast({
        title: "Error",
        description:
          "Failed to fetch purchase data. Please check your connection and try again.",
        status: "error",
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!token || !user || bootstrappedPurchaseTokenRef.current === token) {
      return;
    }

    bootstrappedPurchaseTokenRef.current = token;
    const queryAccountingStatus = searchParams
      ?.get("accounting_status")
      ?.toUpperCase();
    const queryStartDate = searchParams?.get("start_date") || "";
    const queryEndDate = searchParams?.get("end_date") || "";
    const querySearch = searchParams?.get("search") || "";
    const hasQueryFilters = Boolean(
      queryAccountingStatus || queryStartDate || queryEndDate || querySearch,
    );
    const initialFilters = hasQueryFilters
      ? {
          ...getDefaultPurchaseFilters(),
          accounting_status: queryAccountingStatus || "",
          start_date: queryStartDate,
          end_date: queryEndDate,
          search: querySearch,
        }
      : isAccounting
        ? getAccountingMonitoringFilters()
        : getDefaultPurchaseFilters();

    setSearchInput(querySearch);
    setFilters(initialFilters);
    fetchPurchases(initialFilters);
    fetchVendors(); // Load vendors for filter
    fetchBankAccounts(); // Load bank/cash accounts for view & forms
  }, [token, user, isAccounting, searchParams]);

  // Handle filter changes (immediate for non-search filters)
  const handleFilterChange = (newFilters: Partial<PurchaseFilterParams>) => {
    const updatedFilters = { ...filters, ...newFilters, page: 1 };
    setFilters(updatedFilters);
    fetchPurchases(updatedFilters);
  };

  const handleApplyAccountingMonitoring = (
    accountingStatus = "READY_FOR_COA",
    useCurrentMonth = true,
  ) => {
    const updatedFilters: PurchaseFilterParams = {
      ...filters,
      ...(useCurrentMonth ? getCurrentMonthPurchaseRange() : {}),
      status: "",
      approval_status: "",
      accounting_status: accountingStatus,
      page: 1,
    };
    setSearchInput("");
    setFilters(updatedFilters);
    fetchPurchases(updatedFilters);
  };

  const handleClearFilters = () => {
    setSearchInput("");
    setPurchases(allPurchases);
    const resetFilters = getDefaultPurchaseFilters();
    setFilters(resetFilters);
    fetchPurchases(resetFilters);
  };

  // Client-side search handler (instant, no API call)
  const handleSearchChange = (value: string) => {
    setSearchInput(value);

    // Client-side filtering - no API call
    if (!value.trim()) {
      // If search is empty, show all purchases
      setPurchases(allPurchases);
      return;
    }

    // Filter purchases based on search term
    const searchTerm = value.toLowerCase();
    const filtered = allPurchases.filter((purchase) => {
      // Search in purchase code
      if (purchase.code?.toLowerCase().includes(searchTerm)) return true;

      // Search in vendor name
      if (purchase.vendor?.name?.toLowerCase().includes(searchTerm))
        return true;

      // Search in notes
      if (purchase.notes?.toLowerCase().includes(searchTerm)) return true;

      // Search in payment reference
      if (purchase.payment_reference?.toLowerCase().includes(searchTerm))
        return true;

      return false;
    });

    setPurchases(filtered);
  };

  // Apply client-side search when allPurchases changes
  useEffect(() => {
    if (searchInput) {
      handleSearchChange(searchInput);
    }
  }, [allPurchases]);

  // Handle page change
  const handlePageChange = (page: number) => {
    const totalRows = pagination.total || purchases.length;
    const totalPages = Math.max(
      pagination.totalPages || Math.ceil(totalRows / pagination.limit) || 1,
      1,
    );
    const nextPage = Math.min(Math.max(page, 1), totalPages);
    const updatedFilters = { ...filters, page: nextPage };
    setFilters(updatedFilters);
    fetchPurchases(updatedFilters);
  };

  const handlePageSizeChange = (limit: number) => {
    const updatedFilters = { ...filters, limit, page: 1 };
    setFilters(updatedFilters);
    fetchPurchases(updatedFilters);
  };

  // Handle refresh
  const handleRefresh = () => {
    fetchPurchases();
    toast({
      title: "Refreshed",
      description: "Purchase data has been refreshed",
      status: "info",
      duration: 2000,
      isClosable: true,
    });
  };

  // Export handlers (aligned with Sales/Payments)
  const handleExportPDF = async () => {
    try {
      await purchaseService.downloadPurchasesPDF({
        status: filters.status || undefined,
        vendor_id: filters.vendor_id || undefined,
        approval_status: filters.approval_status || undefined,
        start_date: filters.start_date || undefined,
        end_date: filters.end_date || undefined,
        search: filters.search || undefined,
        accounting_status: filters.accounting_status || undefined,
      });
      toast({
        title: "Success",
        description: "Purchase report PDF has been downloaded",
        status: "success",
        duration: 3000,
      });
    } catch (error: any) {
      toast({
        title: "Export failed",
        description: error?.message || "Failed to export purchases PDF",
        status: "error",
        duration: 3000,
      });
    }
  };

  const handleExportCSV = async () => {
    try {
      await purchaseService.downloadPurchasesCSV({
        status: filters.status || undefined,
        vendor_id: filters.vendor_id || undefined,
        approval_status: filters.approval_status || undefined,
        start_date: filters.start_date || undefined,
        end_date: filters.end_date || undefined,
        search: filters.search || undefined,
        accounting_status: filters.accounting_status || undefined,
      });
      toast({
        title: "Success",
        description: "Purchase report CSV has been downloaded",
        status: "success",
        duration: 3000,
      });
    } catch (error: any) {
      toast({
        title: "Export failed",
        description: error?.message || "Failed to export purchases CSV",
        status: "error",
        duration: 3000,
      });
    }
  };

  // Handle purchase submission for approval
  const handleSubmitForApproval = async (purchaseId: number) => {
    try {
      await purchaseService.submitForApproval(purchaseId);
      await fetchPurchases(); // Refresh data
      toast({
        title: "Success",
        description: "Purchase submitted for approval",
        status: "success",
        duration: 3000,
        isClosable: true,
      });
    } catch (err: any) {
      toast({
        title: "Error",
        description:
          err.response?.data?.error || "Failed to submit purchase for approval",
        status: "error",
        duration: 5000,
        isClosable: true,
      });
    }
  };

  // Handle delete purchase
  const handleDelete = async (purchaseId: number) => {
    // Find the purchase to check its status
    const purchaseToDelete = purchases.find((p) => p.id === purchaseId);
    const isApproved =
      purchaseToDelete &&
      (purchaseToDelete.status || "").toUpperCase() === "APPROVED";

    const confirmMessage = isApproved
      ? "WARNING: This purchase is APPROVED. Are you sure you want to delete this approved purchase? This action cannot be undone."
      : "Are you sure you want to delete this purchase?";

    if (!confirm(confirmMessage)) return;

    try {
      await purchaseService.delete(purchaseId);
      await fetchPurchases(); // Refresh data
      toast({
        title: "Success",
        description: `Purchase ${isApproved ? "(APPROVED)" : ""} deleted successfully`,
        status: "success",
        duration: 3000,
        isClosable: true,
      });
    } catch (err: any) {
      const errMsg = err.response?.data?.error || err.message || "";
      const closedPeriod = parseClosedPeriodError(errMsg);
      if (closedPeriod) {
        toast({
          title: t("messages.toast.closedPeriodError"),
          description: t("messages.toast.closedPeriodErrorDesc", {
            date: formatClosedPeriodDate(closedPeriod.date, language),
          }),
          status: "error",
          duration: 5000,
          isClosable: true,
        });
      } else {
        toast({
          title: t("messages.toast.error"),
          description:
            errMsg ||
            t("purchases.messages.deleteError") ||
            "Failed to delete purchase",
          status: "error",
          duration: 5000,
          isClosable: true,
        });
      }
    }
  };

  // Handle view purchase
  const handleView = async (purchase: Purchase) => {
    try {
      // Fetch detailed purchase data
      const detailResponse = await purchaseService.getById(purchase.id);
      setSelectedPurchase(detailResponse);
      onViewOpen();
    } catch (err: any) {
      toast({
        title: "Error",
        description: "Failed to fetch purchase details",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
    }
  };

  useEffect(() => {
    if (!searchParams) {
      return;
    }

    const notificationPurchaseId = searchParams.get("notificationPurchaseId");

    if (
      !notificationPurchaseId ||
      handledNotificationPurchaseId.current === notificationPurchaseId
    ) {
      return;
    }

    handledNotificationPurchaseId.current = notificationPurchaseId;

    const clearNotificationPurchaseParam = () => {
      const nextParams = new URLSearchParams(searchParams.toString());
      nextParams.delete("notificationPurchaseId");
      const nextUrl = nextParams.toString()
        ? `/purchases?${nextParams.toString()}`
        : "/purchases";
      router.replace(nextUrl);
    };

    const purchaseId = Number(notificationPurchaseId);
    if (!Number.isFinite(purchaseId)) {
      clearNotificationPurchaseParam();
      return;
    }

    const openPurchaseFromNotification = async () => {
      try {
        const detailResponse = await purchaseService.getById(purchaseId);
        setSelectedPurchase(detailResponse);
        onViewOpen();
      } catch (error) {
        toast({
          title: "Error",
          description: "Failed to open purchase from notification",
          status: "error",
          duration: 3000,
          isClosable: true,
        });
      } finally {
        clearNotificationPurchaseParam();
      }
    };

    void openPurchaseFromNotification();
  }, [onViewOpen, router, searchParams, toast]);

  // Handle record payment for approved credit purchases
  const handleRecordPayment = async (purchase: Purchase) => {
    try {
      // Validate that purchase can receive payment
      if (!purchaseService.canReceivePayment(purchase)) {
        toast({
          title: t("purchases.cannotRecordPaymentTitle"),
          description: t("purchases.canReceivePaymentErrorDesc"),
          status: "warning",
          duration: 4000,
          isClosable: true,
        });
        return;
      }

      // Fetch cash banks for payment form
      if (cashBanks.length === 0) {
        const accountsLoaded = await fetchCashBanksForPayment();
        if (!accountsLoaded) return;
      }

      setSelectedPurchaseForPayment(purchase);
      onPaymentOpen();
    } catch (err: any) {
      toast({
        title: "Error",
        description: "Failed to prepare payment form",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
    }
  };

  // Handle payment success
  const handlePaymentSuccess = async (result: any) => {
    toast({
      title: t("purchases.paymentSuccessTitle"),
      description: t("purchases.paymentSuccessDesc", {
        code: result.payment?.code || "N/A",
      }),
      status: "success",
      duration: 5000,
      isClosable: true,
    });

    // Refresh purchases to show updated amounts
    await fetchPurchases();
  };

  // Fetch cash banks for payment form
  const fetchCashBanksForPayment = async (): Promise<boolean> => {
    if (!token) return false;
    try {
      const accounts = await requestOperationalPaymentAccounts(token);
      setCashBanks(accounts);
      return true;
    } catch (error: unknown) {
      setCashBanks([]);
      logPaymentAccountsError(error);
      return false;
    }
  };

  // Handle create receipt for approved/paid/completed purchases with pending receipt quantity.
  const handleCreateReceipt = async (purchase: Purchase) => {
    try {
      // Fetch detailed purchase data to get items and latest status from backend
      const detailResponse = await purchaseService.getById(purchase.id);
      setSelectedPurchase(detailResponse);

      // Safety guard: backend mengizinkan status ini selama masih ada qty yang belum direceive.
      const serverStatus = (detailResponse.status || "").toUpperCase();
      const allowedStatuses = ["APPROVED", "PENDING", "PAID", "COMPLETED"];
      if (!allowedStatuses.includes(serverStatus)) {
        toast({
          title: t("purchases.createReceiptErrorTitle"),
          description: t("purchases.createReceiptErrorDesc", {
            status: serverStatus || "UNKNOWN",
          }),
          status: "warning",
          duration: 5000,
          isClosable: true,
        });
        return;
      }

      // Fetch existing receipts to compute remaining qty per item
      const receiptsRes = await fetch(
        `${API_ENDPOINTS.PURCHASES_RECEIPTS_BY_ID(purchase.id)}`,
        {
          method: "GET",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
          },
        },
      );
      let existingReceipts: any[] = [];
      if (receiptsRes.ok) {
        const data = await receiptsRes.json();
        existingReceipts = data.data || [];
      }
      const receivedMap: Record<number, number> = {};
      for (const r of existingReceipts) {
        for (const it of r.receipt_items || []) {
          const pid = it.purchase_item_id || it.purchase_item?.id;
          if (!pid) continue;
          receivedMap[pid] =
            (receivedMap[pid] || 0) + (it.quantity_received || 0);
        }
      }

      // Ensure accounts catalog is loaded (to detect fixed asset 150x)
      await fetchExpenseAccounts();

      const receiptableItems = getReceiptablePurchaseItems(
        detailResponse.purchase_items || [],
      );
      if (receiptableItems.length === 0) {
        toast({
          title: "Receipt tidak diperlukan",
          description:
            "Purchase ini hanya berisi jasa/beban/prepaid, jadi tidak perlu goods receipt.",
          status: "info",
          duration: 5000,
          isClosable: true,
        });
        return;
      }

      // Initialize receipt form data with remaining quantity per item
      const remainingMap: Record<number, number> = {};
      receiptableItems.forEach((it: any) => {
        const receivedSoFar = receivedMap[it.id] || 0;
        remainingMap[it.id] = Math.max((it.quantity || 0) - receivedSoFar, 0);
      });
      setRemainingQtyMap(remainingMap);

      // If semua sisa = 0, tidak perlu buka modal dan sembunyikan tombol Create Receipt
      const allZero = Object.values(remainingMap).every((v) => v <= 0);
      if (allZero) {
        setFullyReceivedPurchases((prev) => new Set(prev).add(purchase.id));
        toast({
          title: t("purchases.allItemsReceivedTitle"),
          description: t("purchases.allItemsReceivedDesc"),
          status: "info",
          duration: 4000,
          isClosable: true,
        });
        return; // Jangan buka modal
      }

      const receiptItems = receiptableItems
        .map((item: any) => {
          const remaining = remainingMap[item.id] || 0;
          // Determine if the item's expense account is a Fixed Asset (150x) to default-enable asset creation
          const acc = expenseAccounts.find(
            (a) => a.id === item.expense_account_id,
          );
          const accCode = (acc?.code || "").toString();
          const accName = (acc?.name || "").toLowerCase();
          const isFixedAsset =
            accCode.startsWith("15") ||
            accName.includes("asset tetap") ||
            accName.includes("fixed asset") ||
            accName.includes("bangunan") ||
            accName.includes("gedung") ||
            accName.includes("peralatan") ||
            accName.includes("kendaraan") ||
            accName.includes("mesin") ||
            accName.includes("furniture") ||
            accName.includes("computer") ||
            accName.includes("komputer");
          return {
            purchase_item_id: item.id,
            quantity_received: remaining, // default to remaining qty
            condition: "GOOD",
            notes: "",
            create_asset: isFixedAsset, // auto-enable when account is fixed asset
          };
        })
        .filter((item) => item.quantity_received > 0);

      setReceiptFormData({
        received_date: new Date().toISOString().split("T")[0],
        notes: "",
        receipt_items: receiptItems,
      });

      onReceiptOpen();
    } catch (err: any) {
      toast({
        title: "Error",
        description: "Failed to fetch purchase details for receipt creation",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
    }
  };

  // Handle save receipt
  const handleSaveReceipt = async () => {
    if (!selectedPurchase) {
      toast({
        title: "Error", // Or common.error
        description: t("purchases.noPurchaseSelected"),
        status: "error",
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    if (receiptFormData.receipt_items.length === 0) {
      toast({
        title: "Error",
        description: t("purchases.addReceiptItemError"),
        status: "error",
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    const invalidQtyItem = receiptFormData.receipt_items.find(
      (item) => Number(item.quantity_received || 0) < 1,
    );
    if (invalidQtyItem) {
      toast({
        title: "Qty receipt belum valid",
        description:
          "Jumlah diterima minimal 1. Jika belum ingin membuat receipt, klik Cancel.",
        status: "warning",
        duration: 4000,
        isClosable: true,
      });
      return;
    }

    try {
      setSavingReceipt(true);
      const filteredItems = receiptFormData.receipt_items
        .map((item) => {
          const maxRemaining =
            remainingQtyMap[item.purchase_item_id] ?? item.quantity_received;
          const safeQty = Math.min(
            Number(item.quantity_received || 0),
            maxRemaining,
          );

          return {
            purchase_item_id: item.purchase_item_id,
            quantity_received: safeQty,
            condition: item.condition,
            notes: item.notes,
            // NOTE: Do NOT send asset-specific flags here to keep API payload compatible.
            // Asset creation is handled separately via assetService after receipt is successfully created.
          };
        })
        .filter((item) => item.quantity_received > 0);

      if (filteredItems.length === 0) {
        toast({
          title: "Qty receipt belum valid",
          description:
            "Tidak ada item dengan jumlah diterima yang valid. Minimal isi qty 1.",
          status: "warning",
          duration: 4000,
          isClosable: true,
        });
        return;
      }

      const payload = {
        purchase_id: selectedPurchase.id,
        received_date: receiptFormData.received_date + "T00:00:00Z",
        notes: receiptFormData.notes,
        receipt_items: filteredItems,
      };

      // Call API to create receipt
      const response = await fetch(`/api/v1/purchases/receipts`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(payload),
      });

      // Robust error handling: backend may respond with plain text (e.g. "Internal Server Error")
      if (!response.ok) {
        let errorMessage = "Failed to create receipt";

        try {
          const contentType = response.headers.get("content-type") || "";

          if (contentType.includes("application/json")) {
            const errorData = await response.json();
            // Prefer specific backend fields, but fall back to any useful details
            errorMessage =
              errorData?.error ||
              errorData?.message ||
              errorData?.details ||
              errorMessage;

            console.error("Create receipt failed with JSON error:", {
              status: response.status,
              statusText: response.statusText,
              errorData,
            });
          } else {
            const errorText = await response.text();
            if (errorText) {
              errorMessage = errorText;
            }
            console.error("Create receipt failed with non-JSON response:", {
              status: response.status,
              statusText: response.statusText,
              body: errorText,
            });
          }
        } catch (parseError) {
          console.error("Failed to parse receipt error response:", parseError);
        }

        // Include HTTP status in the thrown error message for easier debugging
        throw new Error(
          `${errorMessage} (HTTP ${response.status} ${response.statusText})`,
        );
      }

      let receiptData: any = {};
      try {
        receiptData = await response.json();
      } catch (parseOkError) {
        console.error(
          "Failed to parse receipt success response as JSON:",
          parseOkError,
        );
      }

      // Normalize receipt number from API response shape
      const receiptNumber: string =
        receiptData?.receipt?.receipt_number ||
        receiptData?.receipt_number ||
        "N/A";

      // Mark this purchase as having receipts in local UI state
      setPurchasesWithReceipts((prev) =>
        new Set(prev).add(selectedPurchase.id),
      );

      // Update remaining map after this receipt
      const newRemaining: Record<number, number> = { ...remainingQtyMap };
      for (const ri of filteredItems) {
        newRemaining[ri.purchase_item_id] = Math.max(
          (newRemaining[ri.purchase_item_id] ?? 0) -
            (ri.quantity_received || 0),
          0,
        );
      }
      setRemainingQtyMap(newRemaining);

      // Determine if this receipt completes all remaining quantities
      const isFullyReceived = Object.values(newRemaining).every((v) => v <= 0);
      if (isFullyReceived) {
        setFullyReceivedPurchases((prev) =>
          new Set(prev).add(selectedPurchase.id),
        );
      }

      // NOTE: Asset creation is now automatic at purchase approval time (PSAK 16 accrual basis)
      // No need to create assets from receipts anymore

      const successMessage = t("purchases.receiptCreatedSuccessDescNoAssets", {
        number: receiptNumber,
      });

      toast({
        title: t("purchases.receiptCreatedSuccessTitle"),
        description: successMessage,
        status: "success",
        duration: 5000,
        isClosable: true,
      });

      // Reset form and close modal
      setReceiptFormData({
        received_date: new Date().toISOString().split("T")[0],
        notes: "",
        receipt_items: [],
      });

      onReceiptClose();

      // Refresh the purchase list
      await fetchPurchases();
    } catch (err: any) {
      console.error("Error creating receipt:", err);
      toast({
        title: "Error",
        description: err.message || "Failed to create receipt",
        status: "error",
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setSavingReceipt(false);
    }
  };

  // Fetch vendors
  const fetchVendors = async () => {
    if (!token) return;

    try {
      setLoadingVendors(true);

      const response = await fetch(API_ENDPOINTS.PURCHASES_VENDORS, {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        setVendors([]);
        console.warn("[Purchases] Vendors unavailable", {
          status: response.status,
        });
        return;
      }

      const vendorsData = await response.json();

      // Handle both array and object responses (e.g., { data: [...] })
      const vendorsArray = Array.isArray(vendorsData)
        ? vendorsData
        : vendorsData.data || [];

      // Transform the data to match our Vendor interface
      const formattedVendors = vendorsArray.map((vendor: any) => ({
        id: vendor.id,
        name: vendor.name,
        code: vendor.code || `V${vendor.id}`,
      }));

      setVendors(formattedVendors);
    } catch (error: unknown) {
      setVendors([]);
      console.warn("[Purchases] Vendors unavailable", error);
    } finally {
      setLoadingVendors(false);
    }
  };

  // Fetch Purchase Requests
  const fetchPurchaseRequests = async (currentPurchaseId?: number | null) => {
    if (!token) return;
    try {
      setLoadingPRs(true);
      const params = new URLSearchParams({
        status: "APPROVED",
        available_for_purchase: "true",
      });
      if (currentPurchaseId) {
        params.set("current_purchase_id", currentPurchaseId.toString());
      }
      const response = await fetch(
        `/api/v1/purchase-requests?${params.toString()}`,
        {
          headers: { Authorization: `Bearer ${token}` },
        },
      );
      if (response.ok) {
        const data = await response.json();
        const prs = Array.isArray(data.data)
          ? data.data
          : Array.isArray(data)
            ? data
            : [];
        setPurchaseRequests(
          prs.filter((pr: PurchaseRequest) =>
            isPurchaseRequestSelectableForPurchase(pr, currentPurchaseId),
          ),
        );
      }
    } catch (err: any) {
      console.error("Error fetching PRs:", err);
    } finally {
      setLoadingPRs(false);
    }
  };

  // Fetch Projects
  const fetchProjects = async () => {
    if (!token) return;
    try {
      setLoadingProjects(true);
      const response = await fetch("/api/v1/projects", {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (response.ok) {
        const data = await response.json();
        setProjects(
          Array.isArray(data.data)
            ? data.data
            : Array.isArray(data)
              ? data
              : [],
        );
      }
    } catch (err: any) {
      console.error("Error fetching projects:", err);
    } finally {
      setLoadingProjects(false);
    }
  };

  // Fetch products for dropdown
  const fetchProductsList = async () => {
    try {
      setLoadingProducts(true);
      const data = await productService.getProducts({ page: 1, limit: 1000 });
      const list: Product[] = Array.isArray(data?.data) ? data.data : [];
      setProducts(list);
    } catch (err: any) {
      console.error("Error fetching products:", err);
      toast({
        title: "Error",
        description: "Failed to fetch products",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
    } finally {
      setLoadingProducts(false);
    }
  };

  // Fetch asset categories for fixed asset purchase items
  const fetchAssetCategories = async () => {
    try {
      setLoadingAssetCategories(true);
      const res = await assetService.getAssetCategories();
      setAssetCategories(res?.data || []);
    } catch (err: any) {
      console.error("Error fetching asset categories:", err);
      setAssetCategories([]);
    } finally {
      setLoadingAssetCategories(false);
    }
  };

  // Fetch bank accounts for payment method selection
  const fetchBankAccounts = async () => {
    if (!token) return;
    try {
      setLoadingBankAccounts(true);
      const accounts = await requestOperationalPaymentAccounts(token);
      setBankAccounts(accounts);
    } catch (error: unknown) {
      setBankAccounts([]);
      logPaymentAccountsError(error);
    } finally {
      setLoadingBankAccounts(false);
    }
  };

  // Fetch credit accounts (liability) for credit payment method selection
  const fetchCreditAccounts = async () => {
    if (!token) return;
    try {
      setLoadingCreditAccounts(true);

      // Use public catalog endpoint for all roles - no authentication required
      try {
        const catalogData = await accountService.getCreditAccounts(); // Use our fixed public method
        const formattedAccounts: GLAccount[] = filterActiveLeafAccounts(
          catalogData,
        ).map((item) => ({
          id: item.id,
          code: item.code,
          name: item.name,
          type: "LIABILITY" as const,
          is_active: item.active,
          level: 1,
          is_header: false,
          balance: 0,
          created_at: "",
          updated_at: "",
          description: "",
        }));
        console.log(
          "Formatted credit accounts from public catalog:",
          formattedAccounts,
        );
        setCreditAccounts(formattedAccounts);
        return; // Success, exit early
      } catch (catalogError: any) {
        console.log(
          "Public catalog endpoint failed, trying regular endpoint with auth:",
          catalogError.message,
        );
        // Fall through to try regular endpoint with authentication
      }

      // Use full account data for other roles or as fallback for EMPLOYEE
      try {
        const data = await accountService.getAccounts(token, "LIABILITY");
        const list: GLAccount[] = Array.isArray(data)
          ? filterActiveLeafAccounts(data)
          : [];
        console.log("Formatted credit accounts from regular endpoint:", list);
        setCreditAccounts(list);
      } catch (regularError: any) {
        console.error("Regular accounts endpoint also failed:", regularError);
        throw regularError; // Re-throw to be caught by outer catch
      }
    } catch (err: any) {
      console.error("Error fetching credit accounts:", err);
      // If both endpoints fail, fall back to manual entry mode
      setCreditAccounts([]);

      // Show friendly message for network errors, but don't show "Limited Access" for public endpoint failures
      console.log(
        "Unable to load credit accounts, falling back to manual entry mode",
      );
      // Only show toast for actual network errors, not permission issues
      if (
        !err.message?.includes("Network error") &&
        !err.message?.includes("Failed to fetch")
      ) {
        toast({
          title: "Network Issue",
          description:
            "Unable to load credit accounts list. Please check your connection.",
          status: "warning",
          duration: 3000,
          isClosable: true,
        });
      }
    } finally {
      setLoadingCreditAccounts(false);
    }
  };

  // Fetch both expense and inventory accounts for purchase items
  const fetchExpenseAccounts = async () => {
    if (!token) return;
    try {
      setLoadingExpenseAccounts(true);

      let allAccounts: GLAccount[] = [];

      // Use public catalog endpoint for all roles - no authentication required
      try {
        // Fetch expense accounts using public endpoint
        const expenseCatalogData = await accountService.getExpenseAccounts(); // Use our fixed public method
        const formattedExpenseAccounts: GLAccount[] = filterActiveLeafAccounts(
          expenseCatalogData,
        ).map((item) => ({
          id: item.id,
          code: item.code,
          name: item.name,
          type: "EXPENSE" as const,
          is_active: item.active,
          level: 1,
          is_header: false,
          balance: 0,
          created_at: "",
          updated_at: "",
          description: "",
        }));

        // For service companies: only show expense accounts (no inventory/fixed assets)
        if (isServiceCompany) {
          allAccounts = formattedExpenseAccounts;
        } else {
          // Fetch asset accounts (inventory + fixed assets) using public endpoint
          try {
            const assetCatalogData = await accountService.getAccountCatalog(
              undefined,
              "ASSET",
            ); // Public endpoint
            // Include both inventory and fixed asset accounts
            const assetAccounts = filterActiveLeafAccounts(assetCatalogData)
              .filter((item) => {
                const code = item.code || "";
                const name = (item.name || "").toLowerCase();

                // Inventory accounts (current assets)
                const isInventory =
                  code.startsWith("1103") || name.includes("persediaan");

                // Fixed asset accounts (codes 1500-1599 and specific account codes)
                const isFixedAsset =
                  (code.startsWith("15") && code.length >= 4) || // Fixed asset codes 1500-1599
                  ["1501", "1502", "1503", "1504", "1505", "1509"].includes(
                    code,
                  ) || // Specific known fixed asset accounts
                  name.includes("fixed asset") ||
                  name.includes("asset tetap") ||
                  name.includes("peralatan") ||
                  name.includes("mesin") ||
                  name.includes("printer") ||
                  name.includes("komputer") ||
                  name.includes("equipment") ||
                  name.includes("furniture") ||
                  name.includes("kendaraan") ||
                  name.includes("bangunan") ||
                  name.includes("gedung");

                // Prepaid / Current Asset accounts (Beban Dibayar Dimuka)
                const isPrepaidAsset =
                  code.startsWith("1104") ||
                  code.startsWith("1105") ||
                  code.startsWith("1106") ||
                  code.startsWith("1107") ||
                  code.startsWith("1108") ||
                  code.startsWith("1109") ||
                  name.includes("prepaid") ||
                  name.includes("dibayar dimuka") ||
                  name.includes("uang muka") ||
                  name.includes("bayar dimuka") ||
                  name.includes("sewa dibayar") ||
                  name.includes("asuransi dibayar");

                return isInventory || isFixedAsset || isPrepaidAsset;
              })
              .map((item) => ({
                id: item.id,
                code: item.code,
                name: item.name,
                type: "ASSET" as const,
                is_active: item.active,
                level: 1,
                is_header: false,
                balance: 0,
                created_at: "",
                updated_at: "",
                description: "",
              }));
            allAccounts = [...assetAccounts, ...formattedExpenseAccounts];
          } catch (assetError) {
            console.log(
              "Could not fetch asset accounts, using expense only:",
              assetError,
            );
            allAccounts = formattedExpenseAccounts;
          }
        }

        // Filter out header/parent accounts — only detail accounts can be selected for transactions
        const detailAccounts = filterActiveLeafAccounts(allAccounts);
        console.log(
          "Formatted accounts from public catalog (inventory + fixed assets + expense), filtered headers:",
          detailAccounts.length,
          "/",
          allAccounts.length,
        );
        setExpenseAccounts(detailAccounts);
        setCanListExpenseAccounts(true);
        if (detailAccounts.length > 0) {
          setDefaultExpenseAccountId(detailAccounts[0].id as number);
        }
        return; // Success, exit early
      } catch (catalogError: any) {
        console.log(
          "Public catalog endpoint failed, trying regular endpoint with auth:",
          catalogError.message,
        );
        // Fall through to try regular endpoint with authentication
      }

      // Use full account data for other roles or as fallback for EMPLOYEE
      try {
        // Fetch expense accounts
        const expenseData = await accountService.getAccounts(token, "EXPENSE");
        const expenseList: GLAccount[] = Array.isArray(expenseData)
          ? expenseData
          : [];

        // Try to fetch asset accounts (inventory + fixed assets)
        let allAccountsList: GLAccount[] = expenseList;
        if (!isServiceCompany) {
          try {
            const assetData = await accountService.getAccounts(token, "ASSET");
            const assetList: GLAccount[] = Array.isArray(assetData)
              ? assetData
              : [];
            const assetAccounts = assetList.filter((acc) => {
              const code = acc.code || "";
              const name = (acc.name || "").toLowerCase();

              // Inventory accounts (current assets)
              const isInventory =
                code.startsWith("1103") || name.includes("persediaan");

              // Fixed asset accounts (codes 1500-1599 and specific account codes)
              const isFixedAsset =
                (code.startsWith("15") && code.length >= 4) || // Fixed asset codes 1500-1599
                ["1501", "1502", "1503", "1504", "1505", "1509"].includes(
                  code,
                ) || // Specific known fixed asset accounts
                name.includes("fixed asset") ||
                name.includes("asset tetap") ||
                name.includes("peralatan") ||
                name.includes("mesin") ||
                name.includes("printer") ||
                name.includes("komputer") ||
                name.includes("equipment") ||
                name.includes("furniture") ||
                name.includes("kendaraan") ||
                name.includes("bangunan") ||
                name.includes("gedung");

              // Prepaid / Current Asset accounts (Beban Dibayar Dimuka)
              const isPrepaidAsset =
                code.startsWith("1104") ||
                code.startsWith("1105") ||
                code.startsWith("1106") ||
                code.startsWith("1107") ||
                code.startsWith("1108") ||
                code.startsWith("1109") ||
                name.includes("prepaid") ||
                name.includes("dibayar dimuka") ||
                name.includes("uang muka") ||
                name.includes("bayar dimuka") ||
                name.includes("sewa dibayar") ||
                name.includes("asuransi dibayar");

              return isInventory || isFixedAsset || isPrepaidAsset;
            });
            allAccountsList = [...assetAccounts, ...expenseList];
          } catch (assetError) {
            console.log(
              "Could not fetch asset accounts from regular endpoint, using expense only:",
              assetError,
            );
          }
        }

        // Filter out header/parent accounts — only detail accounts can be selected for transactions
        const detailAccountsList = filterActiveLeafAccounts(allAccountsList);
        console.log(
          "Formatted accounts from regular endpoint (inventory + fixed assets + expense), filtered headers:",
          detailAccountsList.length,
          "/",
          allAccountsList.length,
        );
        setExpenseAccounts(detailAccountsList);
        setCanListExpenseAccounts(true);
        if (detailAccountsList.length > 0) {
          setDefaultExpenseAccountId(detailAccountsList[0].id as number);
        }
      } catch (regularError: any) {
        console.error("Regular accounts endpoint also failed:", regularError);
        throw regularError; // Re-throw to be caught by outer catch
      }
    } catch (err: any) {
      console.error("Error fetching accounts:", err);
      // If both endpoints fail, fall back to manual entry mode
      setCanListExpenseAccounts(false);
      setExpenseAccounts([]);
      setDefaultExpenseAccountId(null);

      // Show friendly message for network errors, but don't show "Limited Access" for public endpoint failures
      console.log(
        "Unable to load expense accounts, falling back to manual entry mode",
      );
      // Only show toast for actual network errors, not permission issues
      if (
        !err.message?.includes("Network error") &&
        !err.message?.includes("Failed to fetch")
      ) {
        toast({
          title: "Network Issue",
          description:
            "Unable to load accounts list. Please check your connection.",
          status: "warning",
          duration: 3000,
          isClosable: true,
        });
      }
    } finally {
      setLoadingExpenseAccounts(false);
    }
  };

  // Helper function to get receiver name from user object
  const getReceiverName = (receiver: any): string => {
    if (!receiver) return "N/A";

    // Try to build name from FirstName and LastName
    if (receiver.first_name || receiver.last_name) {
      return `${receiver.first_name || ""} ${receiver.last_name || ""}`.trim();
    }

    // Fallback to username
    if (receiver.username) {
      return receiver.username;
    }

    // Final fallback
    return "N/A";
  };

  // Handle view receipts
  const handleViewReceipts = async (purchase: Purchase) => {
    try {
      setLoadingReceipts(true);
      setSelectedPurchaseForReceipts(purchase);

      // Fetch all receipts (include PARTIAL and COMPLETE)
      const response = await fetch(
        `${API_ENDPOINTS.PURCHASES_RECEIPTS_BY_ID(purchase.id)}`,
        {
          method: "GET",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
          },
        },
      );

      if (!response.ok) {
        throw new Error("Failed to fetch receipts");
      }

      const data = await response.json();
      const validReceipts = ((data.data || []) as Receipt[]).filter(
        receiptHasReceiptableItems,
      );
      setReceipts(validReceipts);
      // Mark purchase as having receipts locally
      if (validReceipts.length > 0) {
        setPurchasesWithReceipts((prev) => new Set(prev).add(purchase.id));
      }
      onReceiptsOpen();
    } catch (err: any) {
      console.error("Error fetching receipts:", err);
      toast({
        title: "Error",
        description: "Failed to fetch receipts",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
    } finally {
      setLoadingReceipts(false);
    }
  };

  // Handle download receipt PDF
  const handleDownloadReceiptPDF = async (
    receiptId: number,
    receiptNumber: string,
  ) => {
    try {
      const response = await fetch(
        `${API_ENDPOINTS.PURCHASES_RECEIPT_PDF(receiptId)}`,
        {
          method: "GET",
          headers: {
            Authorization: `Bearer ${token}`,
          },
        },
      );

      if (!response.ok) {
        throw new Error("Failed to generate receipt PDF");
      }

      // Create blob and download
      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `receipt_${receiptNumber}.pdf`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);

      toast({
        title: "Success",
        description: `Receipt ${receiptNumber} PDF downloaded successfully`,
        status: "success",
        duration: 3000,
        isClosable: true,
      });
    } catch (err: any) {
      console.error("Error downloading receipt PDF:", err);
      toast({
        title: "Error",
        description: "Failed to download receipt PDF",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
    }
  };

  // Helper function to get valid account IDs
  const getValidAccountIds = async () => {
    try {
      console.log("🔍 Fetching valid account IDs for asset creation...");

      // Try to get available accounts
      const [fixedAssetRes, liabilityRes, depreciationRes] = await Promise.all([
        assetService.getFixedAssetAccounts().catch(() => ({ data: [] })),
        assetService.getLiabilityAccounts().catch(() => ({ data: [] })),
        assetService
          .getDepreciationExpenseAccounts()
          .catch(() => ({ data: [] })),
      ]);

      const fixedAssets = fixedAssetRes.data || [];
      const liabilities = liabilityRes.data || [];
      const depreciation = depreciationRes.data || [];

      console.log("📋 Available accounts:", {
        fixedAssets: fixedAssets.length,
        liabilities: liabilities.length,
        depreciation: depreciation.length,
      });

      return {
        assetAccountId: fixedAssets.length > 0 ? fixedAssets[0].id : undefined,
        liabilityAccountId:
          liabilities.length > 0 ? liabilities[0].id : undefined,
        depreciationAccountId:
          depreciation.length > 0 ? depreciation[0].id : undefined,
      };
    } catch (err) {
      console.warn(
        "⚠️ Could not fetch account IDs, using undefined (backend will use defaults)",
      );
      return {
        assetAccountId: undefined,
        liabilityAccountId: undefined,
        depreciationAccountId: undefined,
      };
    }
  };

  // NEW: Create assets from receipt items
  const createAssetsFromReceipt = async (
    purchase: Purchase,
    assetItems: any[],
    receiptNumber: string,
  ) => {
    try {
      console.log("📝 Starting asset creation process...", {
        purchase: purchase.code,
        assetItemsCount: assetItems.length,
      });
      const assetsCreated = [] as any[];
      const errors = [] as string[];

      // Load existing assets once for duplicate guard
      let existingAssets: any[] = [];
      try {
        const res = await assetService.getAssets();
        existingAssets = res.data || [];
      } catch (e) {
        console.warn(
          "⚠️ Could not load existing assets for duplicate guard. Proceeding without duplicate check.",
        );
      }
      const existingSerials = new Set<string>(
        existingAssets
          .map((a: any) =>
            (a.serial_number || "").toString().trim().toLowerCase(),
          )
          .filter((s: string) => !!s),
      );

      // Helper to detect duplicate by computed name + receipt reference in notes
      const isDuplicateByNameAndReceipt = (name: string) => {
        return existingAssets.some(
          (a: any) =>
            a.name === name &&
            typeof a.notes === "string" &&
            a.notes.includes(`Receipt ${receiptNumber}`),
        );
      };

      // Get valid account IDs first
      const accountIds = await getValidAccountIds();

      for (const receiptItem of assetItems) {
        console.log("🔧 Processing asset item:", receiptItem);

        // Find the corresponding purchase item
        const purchaseItem = purchase.purchase_items?.find(
          (p) => p.id === receiptItem.purchase_item_id,
        );

        if (!purchaseItem) {
          console.error(
            "❌ Purchase item not found for receipt item:",
            receiptItem.purchase_item_id,
          );
          continue;
        }

        console.log("✅ Found purchase item:", purchaseItem);

        // Calculate asset values
        const purchasePrice =
          purchaseItem.unit_price * receiptItem.quantity_received;
        const salvageValue =
          (purchasePrice * (receiptItem.asset_salvage_percentage || 10)) / 100;

        // Create asset payload aligned with AssetCreateRequest; let backend handle journals
        const assetName = `${purchaseItem.product?.name || "Asset"} (${purchase.code})`;

        // Parse user ID from auth context (backend requires user_id)
        let userIdNum: number | undefined = undefined;
        try {
          if (user?.id) {
            const parsed = parseInt(user.id, 10);
            if (!Number.isNaN(parsed) && parsed > 0) {
              userIdNum = parsed;
            }
          }
        } catch (e) {
          console.warn(
            "Could not parse user.id for asset creation",
            user?.id,
            e,
          );
        }

        const assetData = {
          // Core Asset Info
          name: assetName,
          category: receiptItem.asset_category || "Equipment",
          serial_number: receiptItem.serial_number || undefined,
          condition: receiptItem.condition || "Good",
          is_active: true,

          // Financial Info
          purchase_date: purchase.date,
          purchase_price: purchasePrice,
          salvage_value: salvageValue,
          useful_life: receiptItem.asset_useful_life || 5,
          depreciation_method: "STRAIGHT_LINE",

          // Notes (include references & vendor info here instead of extra fields)
          notes: `Auto-created from Purchase ${purchase.code}, Receipt ${receiptNumber}. Vendor: ${purchase.vendor?.name || "N/A"}. ${receiptItem.notes || ""}`,

          // Accounting links (optional, backend may fall back to defaults)
          asset_account_id: accountIds.assetAccountId,
          depreciation_account_id: accountIds.depreciationAccountId,

          // Backend-required context
          user_id: userIdNum,
          payment_method: "CREDIT",
        };

        console.log("📋 Asset data prepared:", assetData);

        try {
          // Duplicate guard: skip if serial exists OR name+receipt already present
          const serialKey = (assetData.serial_number || "")
            .toString()
            .trim()
            .toLowerCase();
          if (
            (serialKey && existingSerials.has(serialKey)) ||
            isDuplicateByNameAndReceipt(assetName)
          ) {
            console.warn("⏭️ Skipping duplicate asset creation for", {
              assetName,
              serial: serialKey,
            });
            continue;
          }

          // Use assetService to create asset
          console.log(
            "🚀 Calling assetService.createAsset with data:",
            assetData,
          );
          const newAsset = await assetService.createAsset(assetData);
          console.log(
            "✅ Asset created successfully via assetService:",
            newAsset,
          );
          assetsCreated.push(newAsset);
          // Update in-memory sets to prevent duplicates within the same run
          if (serialKey) existingSerials.add(serialKey);
          existingAssets.push({
            name: assetName,
            notes: assetData.notes,
            serial_number: assetData.serial_number,
          });
        } catch (apiError: any) {
          console.error("❌ AssetService Error:", apiError);
          const errorResponse = apiError.response?.data;
          let errorMessage = apiError.message || "Unknown error";

          // Handle specific account-related errors
          if (errorResponse?.details?.includes("foreign key constraint")) {
            errorMessage =
              "Account configuration error. Please check Chart of Accounts setup.";
          } else if (
            errorResponse?.details?.includes("fk_journal_lines_account")
          ) {
            errorMessage =
              "Invalid account IDs. Asset created without journal entries.";
          } else if (errorResponse?.error) {
            errorMessage = errorResponse.error;
          }

          console.error("❌ Detailed error:", {
            status: apiError.response?.status,
            data: errorResponse,
            message: errorMessage,
          });

          errors.push(
            `Asset for ${purchaseItem.product?.name}: ${errorMessage}`,
          );
        }
      }

      console.log("📊 Asset creation summary:", {
        created: assetsCreated.length,
        errors: errors.length,
      });

      if (assetsCreated.length > 0) {
        toast({
          title: "Assets Created Successfully! 🎉",
          description: `${assetsCreated.length} asset(s) automatically created from this receipt.`,
          status: "success",
          duration: 5000,
          isClosable: true,
        });
      }

      if (errors.length > 0) {
        console.error("⚠️ Asset creation errors:", errors);
        toast({
          title: "Partial Asset Creation",
          description: `${assetsCreated.length} assets created, ${errors.length} failed. Check console for details.`,
          status: "warning",
          duration: 7000,
          isClosable: true,
        });
      }
    } catch (err: any) {
      console.error("💥 Critical error in asset creation:", err);
      toast({
        title: "Asset Creation Error",
        description:
          "Receipt created successfully, but assets could not be created. Please create them manually.",
        status: "error",
        duration: 7000,
        isClosable: true,
      });
    }
  };

  // Handle download all receipts PDF
  const handleDownloadAllReceiptsPDF = async (purchase: Purchase) => {
    try {
      const response = await fetch(
        `${API_ENDPOINTS.PURCHASES_ALL_RECEIPTS_PDF(purchase.id)}`,
        {
          method: "GET",
          headers: {
            Authorization: `Bearer ${token}`,
          },
        },
      );

      if (!response.ok) {
        throw new Error("Failed to generate combined receipts PDF");
      }

      // Create blob and download
      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `receipts_${purchase.code}.pdf`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);

      toast({
        title: "Success",
        description: `All receipts for ${purchase.code} downloaded successfully`,
        status: "success",
        duration: 3000,
        isClosable: true,
      });
    } catch (err: any) {
      console.error("Error downloading all receipts PDF:", err);
      toast({
        title: "Error",
        description: "Failed to download receipts PDF",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
    }
  };

  // Handle edit purchase
  const handleEdit = async (purchase: Purchase) => {
    try {
      // Fetch detailed purchase data for editing
      const detailResponse = await purchaseService.getById(purchase.id);
      setSelectedPurchase(detailResponse);

      // Set form data for editing
      setFormData({
        vendor_id: detailResponse.vendor_id?.toString() || "",
        purchase_request_id:
          detailResponse.purchase_request_id?.toString() || "",
        project_id: detailResponse.project_id?.toString() || "",
        date: detailResponse.date.split("T")[0], // Format for date input
        due_date: detailResponse.due_date
          ? detailResponse.due_date.split("T")[0]
          : "",
        notes: detailResponse.notes || "",
        discount: detailResponse.discount?.toString() || "0",
        tax: detailResponse.tax?.toString() || "0",
        // Tax additions (Penambahan)
        ppn_rate:
          detailResponse.ppn_rate?.toString() || defaultTaxRate.toString(),
        other_tax_additions:
          detailResponse.other_tax_additions?.toString() || "0",
        freight_cost: detailResponse.freight_cost?.toString() || "0",
        // Tax deductions (Pemotongan)
        pph21_rate: detailResponse.pph21_rate?.toString() || "0",
        pph23_rate: detailResponse.pph23_rate?.toString() || "0",
        other_tax_deductions:
          detailResponse.other_tax_deductions?.toString() || "0",
        // Payment method fields
        payment_method: detailResponse.payment_method || "CREDIT",
        bank_account_id: detailResponse.bank_account_id?.toString() || "",
        credit_account_id: detailResponse.credit_account_id?.toString() || "",
        payment_reference: detailResponse.payment_reference || "",
        items: detailResponse.purchase_items?.map(
          mapPurchaseItemToFormData,
        ) || [
          createPurchaseItemFormData({
            product_id: "2",
            unit_price: "1000",
            expense_account_id: "1",
          }),
        ],
      });

      await fetchVendors(); // Load vendors for dropdown
      await fetchPurchaseRequests(detailResponse.id); // Load Purchase Requests
      await fetchProjects(); // Load Projects
      await fetchProductsList(); // Load products for dropdown
      await fetchExpenseAccounts(); // Load expense accounts for dropdown
      await fetchBankAccounts(); // Load bank accounts for dropdown
      await fetchCreditAccounts(); // Load credit accounts (liability) for dropdown
      onEditOpen();
    } catch (err: any) {
      toast({
        title: "Error",
        description: "Failed to fetch purchase details for editing",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
    }
  };

  // Handle Finance Tax/Payment Config (read-only items, only tax & payment editable)
  const handleFinanceTaxConfig = async (purchase: Purchase) => {
    try {
      const detailResponse = await purchaseService.getById(purchase.id);
      setFinanceTaxPurchase(detailResponse);
      setFinanceTaxForm({
        ppn_rate:
          detailResponse.ppn_rate?.toString() ?? defaultTaxRate.toString(),
        other_tax_additions:
          detailResponse.other_tax_additions?.toString() ?? "0",
        freight_cost: detailResponse.freight_cost?.toString() ?? "0",
        pph21_rate: detailResponse.pph21_rate?.toString() ?? "0",
        pph23_rate: detailResponse.pph23_rate?.toString() ?? "0",
        other_tax_deductions:
          detailResponse.other_tax_deductions?.toString() ?? "0",
        payment_method: detailResponse.payment_method || "CREDIT",
        bank_account_id: detailResponse.bank_account_id?.toString() || "",
        credit_account_id: detailResponse.credit_account_id?.toString() || "",
        payment_reference: detailResponse.payment_reference || "",
      });
      await fetchBankAccounts();
      await fetchCreditAccounts();
      onFinanceTaxOpen();
    } catch (err: any) {
      toast({
        title: "Error",
        description: "Gagal memuat detail purchase",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
    }
  };

  // Calculate total payment amount including taxes for balance validation
  const calculateFinanceTaxTotal = () => {
    if (!financeTaxPurchase) return 0;
    const netBeforeTax =
      financeTaxPurchase.net_before_tax || financeTaxPurchase.total_amount || 0;
    const ppnRate = parseFloat(financeTaxForm.ppn_rate) || 0;
    const otherTaxAdditions =
      parseFloat(financeTaxForm.other_tax_additions) || 0;
    const freightCost = parseFloat(financeTaxForm.freight_cost) || 0;
    const pph21Rate = parseFloat(financeTaxForm.pph21_rate) || 0;
    const pph23Rate = parseFloat(financeTaxForm.pph23_rate) || 0;
    const otherTaxDeductions =
      parseFloat(financeTaxForm.other_tax_deductions) || 0;

    const ppnAmount = netBeforeTax * (ppnRate / 100);
    const pph21Amount = netBeforeTax * (pph21Rate / 100);
    const pph23Amount = netBeforeTax * (pph23Rate / 100);

    return (
      netBeforeTax +
      freightCost +
      ppnAmount +
      otherTaxAdditions -
      pph21Amount -
      pph23Amount -
      otherTaxDeductions
    );
  };

  // Check if selected bank/cash account has sufficient balance
  const getSelectedBankAccount = (): BankAccount | undefined => {
    if (!financeTaxForm.bank_account_id) return undefined;
    return getEligibleBankAccounts().find(
      (a) => a.id.toString() === financeTaxForm.bank_account_id,
    );
  };

  const getEligibleBankAccounts = (): BankAccount[] => {
    if (financeTaxForm.payment_method === "CASH") {
      return bankAccounts.filter((a) => a.type === "CASH");
    }
    if (
      financeTaxForm.payment_method === "BANK_TRANSFER" ||
      financeTaxForm.payment_method === "CHECK"
    ) {
      return bankAccounts.filter((a) => a.type === "BANK");
    }
    return [];
  };

  const getPPhPaymentBankAccounts = (): BankAccount[] =>
    bankAccounts.filter((account) => {
      const currency = (account.currency || "IDR").toUpperCase();
      return currency === "IDR" && ["CASH", "BANK"].includes(account.type);
    });

  const isBalanceInsufficient = (): boolean => {
    const paymentMethod = financeTaxForm.payment_method;
    if (paymentMethod === "CREDIT") return false;
    const selectedAccount = getSelectedBankAccount();
    if (!selectedAccount) return false;
    const totalPayment = calculateFinanceTaxTotal();
    return getAccountBookBalance(selectedAccount) < totalPayment;
  };

  // Get accounts with sufficient balance for recommendations
  const getSufficientAccounts = (): BankAccount[] => {
    const totalPayment = calculateFinanceTaxTotal();
    return getEligibleBankAccounts().filter(
      (a) => getAccountBookBalance(a) >= totalPayment,
    );
  };

  const getAccountingCOAPaymentMethod = () =>
    isPendingFinalApprovalPurchase(accountingCOAPurchase)
      ? accountingCOAForm.payment_method
      : accountingCOAPurchase?.payment_method || "CREDIT";

  const calculateAccountingFinalReviewTotal = () => {
    if (!accountingCOAPurchase) return 0;
    const netBeforeTax =
      accountingCOAPurchase.net_before_tax ||
      accountingCOAPurchase.total_amount ||
      0;
    const ppnRate = parseFloat(accountingCOAForm.ppn_rate) || 0;
    const otherTaxAdditions =
      parseFloat(accountingCOAForm.other_tax_additions) || 0;
    const freightCost = parseFloat(accountingCOAForm.freight_cost) || 0;
    const pph21Rate = parseFloat(accountingCOAForm.pph21_rate) || 0;
    const pph23Rate = parseFloat(accountingCOAForm.pph23_rate) || 0;
    const otherTaxDeductions =
      parseFloat(accountingCOAForm.other_tax_deductions) || 0;

    return (
      netBeforeTax +
      freightCost +
      netBeforeTax * (ppnRate / 100) +
      otherTaxAdditions -
      netBeforeTax * (pph21Rate / 100) -
      netBeforeTax * (pph23Rate / 100) -
      otherTaxDeductions
    );
  };

  const getAccountingFinalReviewEligibleBankAccounts = (): BankAccount[] => {
    if (accountingCOAForm.payment_method === "CASH") {
      return bankAccounts.filter((a) => a.type === "CASH");
    }
    if (
      accountingCOAForm.payment_method === "BANK_TRANSFER" ||
      accountingCOAForm.payment_method === "CHECK"
    ) {
      return bankAccounts.filter((a) => a.type === "BANK");
    }
    return [];
  };

  const getAccountingFinalReviewSelectedBankAccount = ():
    | BankAccount
    | undefined => {
    if (!accountingCOAForm.bank_account_id) return undefined;
    return getAccountingFinalReviewEligibleBankAccounts().find(
      (a) => a.id.toString() === accountingCOAForm.bank_account_id,
    );
  };

  const isAccountingFinalReviewBalanceInsufficient = (): boolean => {
    const paymentMethod = accountingCOAForm.payment_method;
    if (paymentMethod === "CREDIT") return false;
    const selectedAccount = getAccountingFinalReviewSelectedBankAccount();
    if (!selectedAccount) return false;
    return (
      getAccountBookBalance(selectedAccount) < calculateAccountingFinalReviewTotal()
    );
  };

  const handleOpenPurchasePPhPayment = async (purchase: Purchase) => {
    const pphStatus = getPurchasePPhStatus(purchase);
    if (pphStatus.totalAmount <= 0) {
      toast({
        title: "Tidak ada PPh",
        description: "Purchase ini tidak memiliki PPh 21/23 untuk dibayar.",
        status: "info",
        duration: 3000,
        isClosable: true,
      });
      return;
    }
    if (pphStatus.outstandingAmount <= 0) {
      toast({
        title: "PPh sudah dibayar",
        description: "Tidak ada sisa PPh yang perlu disetorkan.",
        status: "success",
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    if (bankAccounts.length === 0) {
      await fetchBankAccounts();
    }

    setSelectedPurchaseForPPh(purchase);
    setPPhPaymentForm({
      payment_date: formatDateForInput(new Date()),
      amount: pphStatus.outstandingAmount.toString(),
      cash_bank_id: "",
      reference: "",
      notes: `Pembayaran PPh purchase ${purchase.code}`,
    });
    onPPhPaymentOpen();
  };

  const handleSubmitPurchasePPhPayment = async () => {
    if (!selectedPurchaseForPPh) return;

    const amount = Number(pphPaymentForm.amount || 0);
    const pphStatus = getPurchasePPhStatus(selectedPurchaseForPPh);
    const cashBankId = Number(pphPaymentForm.cash_bank_id || 0);
    const selectedCashBank = getPPhPaymentBankAccounts().find(
      (account) => account.id === cashBankId,
    );

    if (!pphPaymentForm.payment_date) {
      toast({
        title: "Tanggal wajib diisi",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
      return;
    }
    if (amount <= 0 || amount > pphStatus.outstandingAmount + 1) {
      toast({
        title: "Nominal PPh tidak valid",
        description: `Nominal harus lebih besar dari 0 dan maksimal ${formatCurrency(pphStatus.outstandingAmount)}.`,
        status: "error",
        duration: 4000,
        isClosable: true,
      });
      return;
    }
    if (!cashBankId || !selectedCashBank) {
      toast({
        title: "Pilih kas/bank",
        description: "Pilih akun kas/bank IDR untuk setoran PPh.",
        status: "error",
        duration: 4000,
        isClosable: true,
      });
      return;
    }
    if (getAccountBookBalance(selectedCashBank) + 1 < amount) {
      toast({
        title: "Saldo kas/bank tidak cukup",
        description: `Saldo tersedia ${formatBankAccountSummary(selectedCashBank)}.`,
        status: "error",
        duration: 5000,
        isClosable: true,
      });
      return;
    }

    try {
      setSavingPPhPayment(true);
      await purchaseService.payPurchasePPh(selectedPurchaseForPPh.id, {
        payment_date: pphPaymentForm.payment_date,
        amount,
        cash_bank_id: cashBankId,
        reference: pphPaymentForm.reference.trim(),
        notes: pphPaymentForm.notes.trim(),
      });
      toast({
        title: "PPh purchase berhasil dibayar",
        description: "Status PPh purchase sudah diperbarui.",
        status: "success",
        duration: 4000,
        isClosable: true,
      });
      onPPhPaymentClose();
      setSelectedPurchaseForPPh(null);
      await fetchPurchases();
    } catch (error: any) {
      toast({
        title: "Gagal bayar PPh purchase",
        description:
          error.response?.data?.details ||
          error.response?.data?.error ||
          error.message ||
          "Pembayaran PPh purchase belum bisa diproses.",
        status: "error",
        duration: 6000,
        isClosable: true,
      });
    } finally {
      setSavingPPhPayment(false);
    }
  };

  // Save Finance Tax/Payment Config (only tax + payment fields, NO items)
  // NOTE: Approval is handled separately via "Action Required" button (supports escalation to Director)
  const handleSaveFinanceTax = async () => {
    if (!financeTaxPurchase) return;

    // Guard: block save if selected bank/cash has insufficient balance
    if (isBalanceInsufficient()) {
      const selectedAccount = getSelectedBankAccount();
      toast({
        title: "Saldo Tidak Mencukupi",
        description: `Saldo ${selectedAccount?.name} (${formatBankAccountSummary(selectedAccount)}) tidak mencukupi untuk pembayaran ${formatCurrency(calculateFinanceTaxTotal())}. Silakan pilih akun lain.`,
        status: "error",
        duration: 5000,
        isClosable: true,
      });
      return;
    }
    try {
      setSavingFinanceTax(true);
      const payload: any = {
        ppn_rate: !isNaN(parseFloat(financeTaxForm.ppn_rate))
          ? parseFloat(financeTaxForm.ppn_rate)
          : defaultTaxRate,
        other_tax_additions:
          parseFloat(financeTaxForm.other_tax_additions) || 0,
        freight_cost: parseFloat(financeTaxForm.freight_cost) || 0,
        pph21_rate: parseFloat(financeTaxForm.pph21_rate) || 0,
        pph23_rate: parseFloat(financeTaxForm.pph23_rate) || 0,
        other_tax_deductions:
          parseFloat(financeTaxForm.other_tax_deductions) || 0,
        payment_method: financeTaxForm.payment_method,
        bank_account_id: financeTaxForm.bank_account_id
          ? parseInt(financeTaxForm.bank_account_id)
          : undefined,
        credit_account_id:
          canSelectAccountFromCOA && financeTaxForm.credit_account_id
            ? parseInt(financeTaxForm.credit_account_id)
            : undefined,
        payment_reference: financeTaxForm.payment_reference,
        // NOTE: No items sent — backend will skip item update
      };

      await purchaseService.update(financeTaxPurchase.id, payload);

      toast({
        title: "Berhasil",
        description: `Konfigurasi pajak & pembayaran untuk ${financeTaxPurchase.code} berhasil disimpan. Gunakan tombol "Action Required" untuk approve.`,
        status: "success",
        duration: 5000,
        isClosable: true,
      });

      onFinanceTaxClose();
      await fetchPurchases();
    } catch (err: any) {
      const errorMessage =
        err.response?.data?.message ||
        err.response?.data?.error ||
        err.message ||
        "An error occurred";
      toast({
        title: "Error",
        description: errorMessage,
        status: "error",
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setSavingFinanceTax(false);
    }
  };

  const handleAccountingCOAConfig = async (purchase: Purchase) => {
    try {
      const detailResponse = await purchaseService.getById(purchase.id);
      setAccountingCOAPurchase(detailResponse);
      setAccountingCOAForm({
        ppn_rate:
          detailResponse.ppn_rate?.toString() ?? defaultTaxRate.toString(),
        other_tax_additions:
          detailResponse.other_tax_additions?.toString() ?? "0",
        freight_cost: detailResponse.freight_cost?.toString() ?? "0",
        pph21_rate: detailResponse.pph21_rate?.toString() ?? "0",
        pph23_rate: detailResponse.pph23_rate?.toString() ?? "0",
        other_tax_deductions:
          detailResponse.other_tax_deductions?.toString() ?? "0",
        payment_method: detailResponse.payment_method || "CREDIT",
        bank_account_id: detailResponse.bank_account_id?.toString() || "",
        credit_account_id: detailResponse.credit_account_id?.toString() || "",
        payment_reference: detailResponse.payment_reference || "",
        items: (detailResponse.purchase_items || []).map((item) => ({
          purchase_item_id: item.id,
          label: getPurchaseItemDisplayName(item),
          quantity: item.quantity,
          total_price: item.total_price || item.quantity * item.unit_price,
          expense_account_id: item.expense_account_id?.toString() || "",
        })),
      });
      await fetchBankAccounts();
      await fetchExpenseAccounts();
      await fetchCreditAccounts();
      onAccountingCOAOpen();
    } catch (err: any) {
      toast({
        title: "Error",
        description:
          err.response?.data?.error ||
          err.message ||
          "Gagal memuat detail purchase untuk input COA",
        status: "error",
        duration: 4000,
        isClosable: true,
      });
    }
  };

  const isPendingFinalApprovalPurchase = (purchase?: Purchase | null) =>
    (purchase?.status || "").toUpperCase() === "PENDING_FINAL_APPROVAL";

  const handleSaveAccountingCOA = async () => {
    if (!accountingCOAPurchase) return;
    const isFinanceManagerFinalReview =
      isPendingFinalApprovalPurchase(accountingCOAPurchase) &&
      isFinanceManagerRole(normalizeRole(user?.role as any));
    const accountingPaymentMethod = getAccountingCOAPaymentMethod();

    const missingItem = accountingCOAForm.items.find(
      (item) => !item.expense_account_id,
    );
    if (missingItem) {
      toast({
        title: "COA Belum Lengkap",
        description: `Pilih COA untuk ${missingItem.label}.`,
        status: "warning",
        duration: 4000,
        isClosable: true,
      });
      return;
    }

    if (
      accountingPaymentMethod === "CREDIT" &&
      !accountingCOAForm.credit_account_id
    ) {
      toast({
        title: "COA Hutang Belum Dipilih",
        description: "Purchase kredit wajib memiliki akun hutang/liability.",
        status: "warning",
        duration: 4000,
        isClosable: true,
      });
      return;
    }

    if (
      isFinanceManagerFinalReview &&
      accountingPaymentMethod !== "CREDIT" &&
      !accountingCOAForm.bank_account_id
    ) {
      toast({
        title: "Akun Bank/Kas Belum Dipilih",
        description: "Pilih akun bank/kas untuk metode pembayaran ini.",
        status: "warning",
        duration: 4000,
        isClosable: true,
      });
      return;
    }

    if (
      isFinanceManagerFinalReview &&
      isAccountingFinalReviewBalanceInsufficient()
    ) {
      const selectedAccount = getAccountingFinalReviewSelectedBankAccount();
      toast({
        title: "Saldo Tidak Mencukupi",
        description: `Saldo ${selectedAccount?.name} (${formatBankAccountSummary(selectedAccount)}) tidak mencukupi untuk pembayaran ${formatCurrency(calculateAccountingFinalReviewTotal())}.`,
        status: "error",
        duration: 5000,
        isClosable: true,
      });
      return;
    }

    try {
      setSavingAccountingCOA(true);
      if (isFinanceManagerFinalReview) {
        await purchaseService.updateFinalReviewConfig(accountingCOAPurchase.id, {
          ppn_rate: !isNaN(parseFloat(accountingCOAForm.ppn_rate))
            ? parseFloat(accountingCOAForm.ppn_rate)
            : defaultTaxRate,
          other_tax_additions:
            parseFloat(accountingCOAForm.other_tax_additions) || 0,
          freight_cost: parseFloat(accountingCOAForm.freight_cost) || 0,
          pph21_rate: parseFloat(accountingCOAForm.pph21_rate) || 0,
          pph23_rate: parseFloat(accountingCOAForm.pph23_rate) || 0,
          other_tax_deductions:
            parseFloat(accountingCOAForm.other_tax_deductions) || 0,
          payment_method: accountingPaymentMethod,
          bank_account_id:
            accountingPaymentMethod !== "CREDIT" &&
            accountingCOAForm.bank_account_id
              ? parseInt(accountingCOAForm.bank_account_id, 10)
              : undefined,
          credit_account_id:
            accountingPaymentMethod === "CREDIT" &&
            accountingCOAForm.credit_account_id
              ? parseInt(accountingCOAForm.credit_account_id, 10)
              : undefined,
          payment_reference: accountingCOAForm.payment_reference,
        });
      }

      const updatedPurchase = await purchaseService.completeAccountingCOA(accountingCOAPurchase.id, {
        credit_account_id: accountingCOAForm.credit_account_id
          ? parseInt(accountingCOAForm.credit_account_id, 10)
          : undefined,
        items: accountingCOAForm.items.map((item) => ({
          purchase_item_id: item.purchase_item_id,
          expense_account_id: parseInt(item.expense_account_id, 10),
        })),
      });

      if (isFinanceManagerFinalReview) {
        const finalizedPurchase = await purchaseService.finalApproveAfterCOA(
          accountingCOAPurchase.id,
          {
            comments:
              "Final approval Finance Manager setelah review/koreksi GL/COA",
          },
        );

        toast({
          title: "Purchase Final Approved",
          description:
            "Koreksi pajak, pembayaran, dan COA tersimpan; jurnal purchase sudah diposting.",
          status: "success",
          duration: 5000,
          isClosable: true,
        });

        if (selectedPurchase?.id === accountingCOAPurchase.id) {
          setSelectedPurchase(finalizedPurchase);
        }
      } else {
        toast({
          title: "COA Berhasil Disimpan",
          description:
            "Purchase sekarang menunggu final approval Finance Manager sebelum jurnal diposting.",
          status: "success",
          duration: 5000,
          isClosable: true,
        });

        if (selectedPurchase?.id === accountingCOAPurchase.id) {
          setSelectedPurchase(updatedPurchase);
        }
      }

      onAccountingCOAClose();
      await fetchPurchases();
    } catch (err: any) {
      toast({
        title: "Gagal Finalisasi COA",
        description:
          err.response?.data?.error ||
          err.response?.data?.message ||
          err.message ||
          "Terjadi kesalahan saat finalisasi COA accounting",
        status: "error",
        duration: 6000,
        isClosable: true,
      });
    } finally {
      setSavingAccountingCOA(false);
    }
  };

  const handleFinalApprovePurchase = async (purchase: Purchase) => {
    try {
      const result = await purchaseService.finalApproveAfterCOA(purchase.id, {
        comments:
          "Final approval Finance Manager setelah Accounting melengkapi GL/COA",
      });

      toast({
        title: "Purchase Final Approved",
        description:
          "Jurnal purchase sudah diposting dan proses stok/payable sudah dijalankan.",
        status: "success",
        duration: 5000,
        isClosable: true,
      });

      if (selectedPurchase?.id === purchase.id) {
        setSelectedPurchase(result);
      }
      await fetchPurchases();
    } catch (err: any) {
      toast({
        title: "Gagal Final Approval",
        description:
          err.response?.data?.error ||
          err.response?.data?.message ||
          err.message ||
          "Final approval purchase belum bisa diproses.",
        status: "error",
        duration: 6000,
        isClosable: true,
      });
    }
  };

  // Handle create new purchase
  const handleCreate = async () => {
    // Reset form data
    setFormData({
      vendor_id: "",
      purchase_request_id: "",
      project_id: "",
      date: new Date().toISOString().split("T")[0], // Today's date
      due_date: "",
      notes: "",
      discount: "0",

      // Legacy tax field
      tax: "0",

      // Tax additions (Penambahan)
      ppn_rate: defaultTaxRate.toString(),
      other_tax_additions: "0",
      freight_cost: "0",

      // Tax deductions (Pemotongan)
      pph21_rate: "0",
      pph23_rate: "0",
      other_tax_deductions: "0",

      // Payment method fields
      payment_method: "CREDIT",
      bank_account_id: "",
      credit_account_id: "",
      payment_reference: "",

      items: [],
    });
    setSelectedPurchase(null);
    await fetchVendors(); // Load vendors for dropdown
    await fetchPurchaseRequests(); // Load Purchase Requests
    await fetchProjects(); // Load Projects
    await fetchProductsList(); // Load products for dropdown
    await fetchExpenseAccounts(); // Load expense accounts for dropdown
    await fetchBankAccounts(); // Load bank accounts for dropdown
    await fetchCreditAccounts(); // Load credit accounts (liability) for dropdown
    await fetchAssetCategories(); // Load asset categories for fixed asset items
    onCreateOpen();
  };

  // Handle save for both create and edit
  const handleSave = async () => {
    if (!formData.vendor_id) {
      toast({
        title: "Error",
        description: "Please select a vendor",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    if (formData.items.length === 0) {
      toast({
        title: "Error",
        description: "Please add at least one item",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
      return;
    }

    const isEditingDraftPurchase =
      !!selectedPurchase &&
      (selectedPurchase.status || "").toUpperCase() === "DRAFT";
    const shouldRequireItemAccount =
      canSelectAccountFromCOA &&
      canListExpenseAccounts &&
      expenseAccounts.length > 0 &&
      !isEditingDraftPurchase;

    // Validate all items based on item_type
    const invalidItems = formData.items.filter((item) => {
      const quantity = parseFloat(item.quantity);
      const unitPrice = parseFloat(item.unit_price);
      const amortizationMonth = parseInt(item.amortization_month || "0", 10);
      const hasValidQuantity = Number.isFinite(quantity) && quantity > 0;
      const hasValidUnitPrice = Number.isFinite(unitPrice) && unitPrice >= 0;

      if (!hasValidQuantity || !hasValidUnitPrice) {
        return true;
      }

      if (item.item_type === "ASSET") {
        // Asset items require: description, asset_category_id, quantity
        return !item.description?.trim() || !item.asset_category_id;
      }
      if (item.item_type === "PREPAID") {
        // Prepaid items require COA only for roles that are allowed to choose COA.
        return (
          !item.description?.trim() ||
          (shouldRequireItemAccount &&
            !resolvePurchaseItemExpenseAccountId(item)) ||
          amortizationMonth <= 0
        );
      }
      if (item.item_type === "EXPENSE") {
        return (
          !item.description?.trim() ||
          (shouldRequireItemAccount &&
            !resolvePurchaseItemExpenseAccountId(item))
        );
      }
      // Inventory items require COA only for roles that are allowed to choose COA.
      return (
        !item.product_id ||
        (shouldRequireItemAccount && !resolvePurchaseItemExpenseAccountId(item))
      );
    });

    if (invalidItems.length > 0) {
      const assetInvalid = invalidItems.some((i) => i.item_type === "ASSET");
      const prepaidInvalid = invalidItems.some(
        (i) => i.item_type === "PREPAID",
      );
      const expenseInvalid = invalidItems.some(
        (i) => i.item_type === "EXPENSE",
      );

      let errorMsg = "Please fill in all required fields for each item";
      if (assetInvalid)
        errorMsg = "Item Aset Tetap: Nama aset dan Kategori Aset harus diisi.";
      else if (prepaidInvalid)
        errorMsg = shouldRequireItemAccount
          ? "Item Prepaid: Deskripsi dan Akun harus diisi."
          : "Item Prepaid: Deskripsi dan durasi amortisasi harus diisi.";
      else if (expenseInvalid)
        errorMsg = shouldRequireItemAccount
          ? "Item Beban/Jasa: Deskripsi dan akun beban harus diisi."
          : "Item Beban/Jasa: Deskripsi harus diisi.";

      toast({
        title: "Error",
        description: errorMsg,
        status: "error",
        duration: 4000,
        isClosable: true,
      });
      return;
    }

    if (
      canSelectAccountFromCOA &&
      isFinanceOrAdmin &&
      formData.payment_method === "CREDIT" &&
      !formData.credit_account_id
    ) {
      toast({
        title: "Error",
        description: "Please select a liability account for credit purchases",
        status: "error",
        duration: 4000,
        isClosable: true,
      });
      return;
    }

    if (
      isFinanceOrAdmin &&
      formData.payment_method !== "CREDIT" &&
      !formData.bank_account_id
    ) {
      toast({
        title: "Error",
        description: `Please select a ${formData.payment_method === "CASH" ? "cash" : "bank"} account for this payment method`,
        status: "error",
        duration: 4000,
        isClosable: true,
      });
      return;
    }

    try {
      setLoading(true);

      const normalizedPayment = normalizePurchasePaymentFields(
        formData,
        formData.payment_method,
      );
      const normalizedPaymentMethod = normalizedPayment.payment_method;

      // Format the payload with proper tax rates
      const payload: any = {
        vendor_id: parseInt(formData.vendor_id),
        date: formData.date
          ? `${formData.date}T00:00:00Z`
          : new Date().toISOString(),
        due_date: formData.due_date
          ? `${formData.due_date}T00:00:00Z`
          : undefined,
        notes: formData.notes,
        discount: parseFloat(formData.discount) || 0,
        // Send proper tax rates (not legacy tax field)
        ppn_rate: !isNaN(parseFloat(formData.ppn_rate))
          ? parseFloat(formData.ppn_rate)
          : defaultTaxRate,
        other_tax_additions: parseFloat(formData.other_tax_additions) || 0,
        freight_cost: parseFloat(formData.freight_cost) || 0,
        pph21_rate: parseFloat(formData.pph21_rate) || 0,
        pph23_rate: parseFloat(formData.pph23_rate) || 0,
        other_tax_deductions: parseFloat(formData.other_tax_deductions) || 0,
        // Payment method fields
        payment_method: normalizedPaymentMethod,
        bank_account_id:
          normalizedPaymentMethod === "CREDIT" ||
          !normalizedPayment.bank_account_id
            ? undefined
            : parseInt(normalizedPayment.bank_account_id),
        credit_account_id:
          normalizedPaymentMethod === "CREDIT" &&
          canSelectAccountFromCOA &&
          normalizedPayment.credit_account_id
            ? parseInt(normalizedPayment.credit_account_id)
            : undefined,
        payment_reference:
          normalizedPaymentMethod === "BANK_TRANSFER" ||
          normalizedPaymentMethod === "CHECK"
            ? normalizedPayment.payment_reference.trim()
            : "",
        items: formData.items.map((item) => ({
          item_type: item.item_type || "INVENTORY",
          is_fixed_asset: item.item_type === "ASSET",
          description: item.description || "",
          asset_category_id: item.asset_category_id
            ? parseInt(item.asset_category_id)
            : undefined,
          product_id: item.product_id ? parseInt(item.product_id) : undefined,
          purchase_request_item_id: item.purchase_request_item_id
            ? parseInt(item.purchase_request_item_id)
            : undefined,
          amortization_month:
            item.item_type === "PREPAID"
              ? parseInt(item.amortization_month) || 0
              : 0,
          quantity: parseFloat(item.quantity),
          unit_price: parseFloat(item.unit_price),
          discount: parseFloat(item.discount) || 0,
          tax: parseFloat(item.tax) || 0,
          expense_account_id: resolvePurchaseItemExpenseAccountId(item),
        })),
      };

      if (formData.purchase_request_id) {
        payload.purchase_request_id = parseInt(formData.purchase_request_id);
      }
      if (formData.project_id) {
        payload.project_id = parseInt(formData.project_id);
      }

      let response;

      if (selectedPurchase) {
        // Update existing purchase
        response = await purchaseService.update(selectedPurchase.id, payload);
        toast({
          title: "Success",
          description: `Purchase ${response.code} updated successfully`,
          status: "success",
          duration: 3000,
          isClosable: true,
        });
        onEditClose();
      } else {
        // Create new purchase
        response = await purchaseService.create(payload);
        toast({
          title: "Success",
          description: `Purchase ${response.code} created successfully. Use "Submit for Approval" button to submit when ready.`,
          status: "success",
          duration: 5000,
          isClosable: true,
        });
        onCreateClose();

        // NOTE: Purchase is now created as DRAFT - Employee must manually submit for approval
        // This allows Employee to review the purchase details before submitting
      }

      // Refresh the list
      await fetchPurchases();
    } catch (err: any) {
      console.error("Error saving purchase:", err);
      const errorMessage =
        err.response?.data?.message ||
        err.response?.data?.error ||
        err.message ||
        "An error occurred";
      toast({
        title: "Error",
        description: errorMessage,
        status: "error",
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  // Handle duplicate purchase
  const handleDuplicate = async (purchase: Purchase) => {
    try {
      setLoading(true);
      // Fetch detailed purchase data for duplication
      const detailResponse = await purchaseService.getById(purchase.id);

      // Set form data for NEW purchase (cleared ID, cleared status-related fields)
      setFormData({
        vendor_id: detailResponse.vendor_id?.toString() || "",
        purchase_request_id: "",
        project_id: detailResponse.project_id?.toString() || "",
        date: new Date().toISOString().split("T")[0], // Use today's date
        due_date: "", // Clear due date
        notes: detailResponse.notes || "",
        discount: detailResponse.discount?.toString() || "0",
        tax: detailResponse.tax?.toString() || "0",
        // Tax additions
        ppn_rate:
          detailResponse.ppn_rate?.toString() || defaultTaxRate.toString(),
        other_tax_additions:
          detailResponse.other_tax_additions?.toString() || "0",
        freight_cost: detailResponse.freight_cost?.toString() || "0",
        // Tax deductions
        pph21_rate: detailResponse.pph21_rate?.toString() || "0",
        pph23_rate: detailResponse.pph23_rate?.toString() || "0",
        other_tax_deductions:
          detailResponse.other_tax_deductions?.toString() || "0",
        // Payment method fields
        payment_method: detailResponse.payment_method || "CREDIT",
        bank_account_id: detailResponse.bank_account_id?.toString() || "",
        credit_account_id: detailResponse.credit_account_id?.toString() || "",
        payment_reference: "", // Clear payment reference
        items:
          detailResponse.purchase_items?.map((item) => ({
            ...mapPurchaseItemToFormData(item),
            purchase_request_item_id: "",
          })) || [],
      });

      // Reset selected purchase so it treats it as NEW
      setSelectedPurchase(null);

      // We can run these in parallel to speed up
      await Promise.all([
        fetchVendors(),
        fetchPurchaseRequests(),
        fetchProjects(),
        fetchProductsList(),
        fetchExpenseAccounts(),
        fetchBankAccounts(),
        fetchCreditAccounts(),
      ]);

      onCreateOpen();

      toast({
        title: t("common.duplicate"),
        description: t("purchases.messages.duplicateSuccess"),
        status: "info",
        duration: 3000,
        isClosable: true,
      });
    } catch (error) {
      console.error("Error duplicating purchase:", error);
      toast({
        title: "Error",
        description: "Failed to duplicate purchase data",
        status: "error",
        duration: 3000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  const getCurrentApprovalStepInfo = (purchase?: Purchase | null) => {
    const steps = purchase?.approval_request?.approval_steps || [];
    const activeStep =
      steps.find(
        (step) =>
          step.is_active && (step.status || "").toUpperCase() === "PENDING",
      ) ||
      steps.find((step) => (step.status || "").toUpperCase() === "PENDING");

    if (!activeStep?.step?.approver_role) {
      return null;
    }

    const stepName = activeStep.step.step_name || "";

    return {
      step_name: stepName,
      approver_role: normalizeRole(activeStep.step.approver_role),
      step_order: activeStep.step.step_order,
      is_escalated: stepName.toLowerCase().includes("escalated"),
    };
  };

  const canCurrentUserActOnPurchase = (purchase?: Purchase | null) => {
    const roleNorm = normalizeRole(user?.role as any);
    if (!purchase || !roleNorm) return false;

    const approvalStatus = (purchase.approval_status || "").toUpperCase();
    const purchaseStatus = (purchase.status || "").toUpperCase();

    if (purchaseStatus === "PENDING_FINAL_APPROVAL") {
      return isFinanceManagerRole(roleNorm);
    }

    if (
      approvalStatus === "APPROVED" ||
      approvalStatus === "REJECTED" ||
      purchaseStatus === "CANCELLED"
    ) {
      return false;
    }

    if (purchaseStatus === "DRAFT") {
      return isFinanceManagerRole(roleNorm);
    }

    const activeStep = getCurrentApprovalStepInfo(purchase);
    if (!activeStep?.approver_role) {
      return false;
    }

    return workflowRoleMatchesUserRole(activeStep.approver_role, roleNorm);
  };

  // Smart Action Button Logic
  const getActionButtonProps = (purchase: Purchase) => {
    const roleNorm = normalizeRole(user?.role as any);
    const status = (purchase.approval_status || "").toUpperCase();
    const purchaseStatus = (purchase.status || "").toUpperCase();
    const activeStep = getCurrentApprovalStepInfo(purchase);
    const canActOnDraftApproval =
      purchaseStatus === "DRAFT" && canCurrentUserActOnPurchase(purchase);
    const isUserTurn =
      purchaseStatus === "DRAFT"
        ? canActOnDraftApproval
        : purchaseStatus === "PENDING_FINAL_APPROVAL"
          ? isFinanceManagerRole(roleNorm)
        : workflowRoleMatchesUserRole(activeStep?.approver_role, roleNorm);

    if (purchaseStatus === "PENDING_FINAL_APPROVAL") {
      return isFinanceManagerRole(roleNorm)
        ? {
            text: "Review & Finalisasi",
            icon: <FiCheckCircle />,
            colorScheme: "purple",
            variant: "solid",
          }
        : {
            text: "Waiting for Finance Manager Finalisasi",
            icon: <FiClock />,
            colorScheme: "purple",
            variant: "outline",
          };
    }

    // Completed states
    if (
      status === "APPROVED" ||
      status === "REJECTED" ||
      purchaseStatus === "CANCELLED"
    ) {
      return {
        text: "View",
        icon: <FiEye />,
        colorScheme: "gray",
        variant: "outline",
      };
    }

    if (purchaseStatus === "DRAFT" && !canActOnDraftApproval) {
      return {
        text: "Submit for Approval",
        icon: <FiAlertCircle />,
        colorScheme: "blue",
        variant: "solid",
      };
    }

    // User's turn to act
    if (isUserTurn) {
      // Show appropriate text based on escalation
      const actionText = activeStep?.is_escalated
        ? "Action Required (Escalated)"
        : "Action Required";
      return {
        text: actionText,
        icon: <FiAlertCircle />,
        colorScheme: "orange",
        variant: "solid",
      };
    }

    // Waiting for others - show who needs to act
    if (status === "PENDING" || purchaseStatus === "PENDING_APPROVAL") {
      if (activeStep) {
        const waitingForRole =
          activeStep.approver_role === "finance"
            ? "Finance Manager"
            : activeStep.approver_role === "purchase_finance_staff"
              ? "Finance Staff"
            : activeStep.approver_role === "director"
              ? "Director"
              : activeStep.approver_role === "admin"
                ? "Admin"
                : "Approval";

        const waitingText = activeStep.is_escalated
          ? `Waiting for ${waitingForRole} (Escalated)`
          : `Waiting for ${waitingForRole}`;
        return {
          text: waitingText,
          icon: <FiClock />,
          colorScheme: "blue",
          variant: "outline",
        };
      }
      return {
        text: "Review Progress",
        icon: <FiClock />,
        colorScheme: "blue",
        variant: "outline",
      };
    }

    return {
      text: "View",
      icon: <FiEye />,
      colorScheme: "gray",
      variant: "outline",
    };
  };

  // Action buttons for each row
  const renderActions = (purchase: Purchase) => {
    const actionProps = getActionButtonProps(purchase);
    const roleNorm = normalizeRole(user?.role as any);
    const purchaseStatus = (purchase.status || "").toUpperCase();
    const pphStatus = getPurchasePPhStatus(purchase);
    const financeCanConfigurePurchase =
      (roleNorm === "admin" || isFinanceOperationalRole) &&
      ((purchaseStatus === "DRAFT" && canCurrentUserActOnPurchase(purchase)) ||
        purchaseStatus === "PENDING" ||
        purchaseStatus === "PENDING_APPROVAL");
    const accountingCanCompleteCOA =
      canEdit &&
      ((purchaseStatus === "PENDING_ACCOUNTING" &&
        (roleNorm === "admin" || isAccounting)) ||
        (purchaseStatus === "PENDING_FINAL_APPROVAL" &&
          isFinanceManagerRole(roleNorm)));
    const financeManagerCanFinalApprove =
      canApprove &&
      purchaseStatus === "PENDING_FINAL_APPROVAL" &&
      isFinanceManagerRole(roleNorm);
    const canEditDraftPurchase =
      purchaseStatus === "DRAFT" &&
      (roleNorm === "admin" || roleNorm === "finance_staff");

    return (
      <HStack spacing={2}>
        {canEditDraftPurchase && (
          <Button
            size="sm"
            colorScheme="blue"
            variant="outline"
            leftIcon={<FiEdit />}
            onClick={() => handleEdit(purchase)}
            fontWeight="medium"
            _hover={{
              transform: "translateY(-1px)",
              boxShadow: "md",
            }}
          >
            {t("purchases.edit")}
          </Button>
        )}

        {/* Smart Single Action Button */}
        <Button
          size="sm"
          variant={actionProps.variant}
          colorScheme={actionProps.colorScheme}
          leftIcon={actionProps.icon}
          onClick={() => {
            // Draft purchases should be submitted first by non-approver roles.
            if (
              purchaseStatus === "DRAFT" &&
              actionProps.text === "Submit for Approval"
            ) {
              handleSubmitForApproval(purchase.id);
            } else if (financeManagerCanFinalApprove) {
              handleAccountingCOAConfig(purchase);
            } else {
              setSelectedPurchase(purchase);
              onViewOpen();
            }
          }}
          fontWeight={actionProps.variant === "solid" ? "semibold" : "medium"}
          _hover={{
            transform: "translateY(-1px)",
            boxShadow: "md",
          }}
        >
          {actionProps.text}
        </Button>

        {/* Finance/Admin: Edit Tax & Payment button for PENDING purchases (read-only items) */}
        {financeCanConfigurePurchase && (
          <Button
            size="sm"
            colorScheme="teal"
            variant="outline"
            leftIcon={<FiEdit />}
            onClick={() => handleFinanceTaxConfig(purchase)}
            fontWeight="medium"
            _hover={{
              transform: "translateY(-1px)",
              boxShadow: "md",
            }}
          >
            Set Pajak & Pembayaran
          </Button>
        )}

        {accountingCanCompleteCOA && purchaseStatus !== "PENDING_FINAL_APPROVAL" && (
          <Button
            size="sm"
            colorScheme="orange"
            variant="solid"
            leftIcon={<FiEdit />}
            onClick={() => handleAccountingCOAConfig(purchase)}
            fontWeight="semibold"
            _hover={{
              transform: "translateY(-1px)",
              boxShadow: "md",
            }}
          >
            Lengkapi COA
          </Button>
        )}

        {/* Record Payment button for APPROVED or PAID CREDIT purchases with outstanding amount */}
        {(purchaseStatus === "APPROVED" ||
          purchaseStatus === "PAID" ||
          purchaseStatus === "COMPLETED") &&
          purchaseService.canReceivePayment(purchase) &&
          (roleNorm === "admin" ||
            isFinanceOperationalRole ||
            roleNorm === "director") && (
            <Button
              size="sm"
              colorScheme="blue"
              variant="solid"
              leftIcon={<Icon as={FiPlus} />}
              onClick={() => handleRecordPayment(purchase)}
              fontWeight="semibold"
              _hover={{
                transform: "translateY(-1px)",
                boxShadow: "md",
              }}
            >
              Record Payment
            </Button>
          )}

        {(purchaseStatus === "APPROVED" ||
          purchaseStatus === "PAID" ||
          purchaseStatus === "COMPLETED") &&
          pphStatus.outstandingAmount > 0 &&
          (roleNorm === "admin" ||
            isFinanceOperationalRole ||
            roleNorm === "director") && (
            <Button
              size="sm"
              colorScheme="orange"
              variant="outline"
              leftIcon={<FiDollarSign />}
              onClick={() => handleOpenPurchasePPhPayment(purchase)}
              fontWeight="semibold"
              _hover={{
                transform: "translateY(-1px)",
                boxShadow: "md",
              }}
            >
              Bayar PPh
            </Button>
          )}

        {/* Create Receipt button for approved/paid/completed purchases with remaining receiptable quantity */}
        {(purchaseStatus === "APPROVED" ||
          purchaseStatus === "PAID" ||
          purchaseStatus === "COMPLETED") &&
          canEdit &&
          (roleNorm === "inventory_manager" ||
            roleNorm === "admin" ||
            roleNorm === "director" ||
            isFinanceOperationalRole) &&
          purchaseHasReceiptableItems(purchase) &&
          !fullyReceivedPurchases.has(purchase.id) && (
            <Button
              size="sm"
              colorScheme="green"
              variant="solid"
              leftIcon={<FiPackage />}
              onClick={() => handleCreateReceipt(purchase)}
              fontWeight="semibold"
              _hover={{
                transform: "translateY(-1px)",
                boxShadow: "md",
              }}
            >
              Create Receipt
            </Button>
          )}

        {/* View Receipts button only for purchases that can have goods receipts */}
        {(roleNorm === "admin" ||
          isFinanceOperationalRole ||
          roleNorm === "director" ||
          roleNorm === "inventory_manager" ||
          roleNorm === "employee") &&
          purchaseHasReceiptableItems(purchase) && (
            <Button
              size="sm"
              colorScheme="blue"
              variant="outline"
              leftIcon={<FiFileText />}
              onClick={() => handleViewReceipts(purchase)}
              fontWeight="medium"
              _hover={{
                transform: "translateY(-1px)",
                boxShadow: "md",
              }}
            >
              Receipts
            </Button>
          )}

        {/* View Journal Entries button for APPROVED or PAID purchases - for users with report view permissions */}
        {(purchaseStatus === "APPROVED" ||
          purchaseStatus === "PAID" ||
          purchaseStatus === "COMPLETED") &&
          (roleNorm === "admin" ||
            isFinanceOperationalRole ||
            roleNorm === "director") && (
            <Button
              size="sm"
              colorScheme="purple"
              variant="outline"
              leftIcon={<FiFileText />}
              onClick={() => {
                setSelectedPurchaseForJournal(purchase);
                onJournalOpen();
              }}
              fontWeight="medium"
              _hover={{
                transform: "translateY(-1px)",
                boxShadow: "md",
              }}
            >
              {t("purchases.viewJournal")}
            </Button>
          )}

        {/* Delete button for ADMIN - can delete any status */}
        {normalizeRole(user?.role as any) === "admin" && (
          <Button
            size="sm"
            colorScheme="red"
            variant="outline"
            leftIcon={<FiTrash2 />}
            onClick={() => handleDelete(purchase.id)}
          >
            {t("purchases.delete")}
          </Button>
        )}

        {/* Delete button removed for DIRECTOR role per requirement */}
      </HStack>
    );
  };

  const purchaseTotalRows = pagination.total || purchases.length;
  const purchaseTotalPages = Math.max(
    pagination.totalPages ||
      Math.ceil(purchaseTotalRows / (pagination.limit || 10)) ||
      1,
    1,
  );
  const purchaseStartRow =
    purchaseTotalRows > 0 ? (pagination.page - 1) * pagination.limit + 1 : 0;
  const purchaseEndRow = Math.min(
    pagination.page * pagination.limit,
    purchaseTotalRows,
  );

  if (loading) {
    return (
      <ProtectedModule module="purchases">
        <CompanyTypeGuard moduleName="Purchases">
          <SimpleLayout>
            <Box>
              <Text>{t("purchases.loadingPurchases")}</Text>
            </Box>
          </SimpleLayout>
        </CompanyTypeGuard>
      </ProtectedModule>
    );
  }

  return (
    <ProtectedModule module="purchases">
      <CompanyTypeGuard moduleName="Purchases">
        <SimpleLayout>
          <Box bg={bgColor} minH="100vh" p={6}>
            <VStack spacing={6} align="stretch">
              {/* Enhanced Header */}
              <Card
                bg={cardBg}
                borderWidth="1px"
                borderColor={borderColor}
                boxShadow="sm"
                borderRadius="lg"
                mb={2}
              >
                <CardBody p={6}>
                  <Flex
                    justify="space-between"
                    align="center"
                    wrap="wrap"
                    gap={4}
                  >
                    <VStack align="start" spacing={1}>
                      <Heading size="xl" color={headingColor} fontWeight="600">
                        {t("purchases.title") || "Purchase Management"}
                      </Heading>
                      <Text fontSize="md" color={textSecondary}>
                        {t("purchases.manageTransactions")}
                      </Text>
                    </VStack>

                    <HStack spacing={3}>
                      <Tooltip label={t("purchases.refreshData")}>
                        <IconButton
                          aria-label={t("purchases.refresh")}
                          icon={<FiRefreshCw />}
                          variant="ghost"
                          onClick={handleRefresh}
                          isLoading={loading}
                          _hover={{
                            bg: hoverBg,
                            transform: "translateY(-1px)",
                            boxShadow: "md",
                          }}
                          transition="all 0.2s ease"
                        />
                      </Tooltip>

                      <Menu strategy="fixed">
                        <MenuButton
                          as={Button}
                          leftIcon={<FiDownload />}
                          colorScheme="green"
                          variant="outline"
                          size="md"
                          _hover={{
                            bg: hoverBg,
                            borderColor: hoverBorder,
                            transform: "translateY(-1px)",
                          }}
                          transition="all 0.2s ease"
                        >
                          {t("purchases.exportReport")}
                        </MenuButton>
                        <MenuList
                          zIndex={10001}
                          boxShadow="lg"
                          border="1px solid"
                          borderColor={borderColor}
                          bg={cardBg}
                          minW="160px"
                          maxW="240px"
                        >
                          <MenuItem
                            onClick={handleExportPDF}
                            icon={<FiFileText />}
                          >
                            {t("purchases.exportPDF")}
                          </MenuItem>
                          <MenuItem
                            onClick={handleExportCSV}
                            icon={<FiDownload />}
                          >
                            {t("purchases.exportCSV")}
                          </MenuItem>
                        </MenuList>
                      </Menu>

                      {canCreate && (
                        <Button
                          leftIcon={<FiPlus />}
                          colorScheme="blue"
                          size="md"
                          px={6}
                          fontWeight="medium"
                          onClick={handleCreate}
                          _hover={{
                            transform: "translateY(-1px)",
                            boxShadow: "lg",
                          }}
                        >
                          {t("purchases.createPurchase")}
                        </Button>
                      )}
                    </HStack>
                  </Flex>
                </CardBody>
              </Card>

              {/* Enhanced Statistics Cards */}
              <Grid
                templateColumns="repeat(auto-fit, minmax(280px, 1fr))"
                gap={6}
              >
                <Card
                  bg={cardBg}
                  borderWidth="1px"
                  borderColor={borderColor}
                  boxShadow="sm"
                  borderRadius="lg"
                  _hover={{
                    boxShadow: "md",
                    transform: "translateY(-2px)",
                    transition: "all 0.2s ease",
                  }}
                  transition="all 0.2s ease"
                >
                  <CardBody p={6}>
                    <Flex align="center" justify="space-between">
                      <Stat>
                        <StatLabel
                          color={textSecondary}
                          fontSize="sm"
                          fontWeight="medium"
                          mb={2}
                        >
                          {t("purchases.totalPurchases")}
                        </StatLabel>
                        <StatNumber
                          color={headingColor}
                          fontSize="2xl"
                          fontWeight="bold"
                        >
                          {stats.total}
                        </StatNumber>
                      </Stat>
                      <Box
                        p={3}
                        borderRadius="lg"
                        bg={statBgColors.blue}
                        color={statColors.blue}
                      >
                        <FiRefreshCw size={20} />
                      </Box>
                    </Flex>
                  </CardBody>
                </Card>

                <Card
                  bg={cardBg}
                  borderWidth="1px"
                  borderColor={borderColor}
                  boxShadow="sm"
                  borderRadius="lg"
                  _hover={{
                    boxShadow: "md",
                    transform: "translateY(-2px)",
                    transition: "all 0.2s ease",
                  }}
                  transition="all 0.2s ease"
                >
                  <CardBody p={6}>
                    <Flex align="center" justify="space-between">
                      <Stat>
                        <StatLabel
                          color={textSecondary}
                          fontSize="sm"
                          fontWeight="medium"
                          mb={2}
                        >
                          {t("purchases.pendingApproval")}
                        </StatLabel>
                        <StatNumber
                          color={statColors.orange}
                          fontSize="2xl"
                          fontWeight="bold"
                        >
                          {stats.needingApproval}
                        </StatNumber>
                      </Stat>
                      <Box
                        p={3}
                        borderRadius="lg"
                        bg={statBgColors.orange}
                        color={statColors.orange}
                      >
                        <FiClock size={20} />
                      </Box>
                    </Flex>
                  </CardBody>
                </Card>

                <Card
                  bg={cardBg}
                  borderWidth="1px"
                  borderColor={borderColor}
                  boxShadow="sm"
                  borderRadius="lg"
                  _hover={{
                    boxShadow: "md",
                    transform: "translateY(-2px)",
                    transition: "all 0.2s ease",
                  }}
                  transition="all 0.2s ease"
                >
                  <CardBody p={6}>
                    <Flex align="center" justify="space-between">
                      <Stat>
                        <StatLabel
                          color={textSecondary}
                          fontSize="sm"
                          fontWeight="medium"
                          mb={2}
                        >
                          {t("purchases.approved")}
                        </StatLabel>
                        <StatNumber
                          color={statColors.green}
                          fontSize="2xl"
                          fontWeight="bold"
                        >
                          {stats.approved}
                        </StatNumber>
                      </Stat>
                      <Box
                        p={3}
                        borderRadius="lg"
                        bg={statBgColors.green}
                        color={statColors.green}
                      >
                        <FiCheckCircle size={20} />
                      </Box>
                    </Flex>
                  </CardBody>
                </Card>

                <Card
                  bg={cardBg}
                  borderWidth="1px"
                  borderColor={borderColor}
                  boxShadow="sm"
                  borderRadius="lg"
                  _hover={{
                    boxShadow: "md",
                    transform: "translateY(-2px)",
                    transition: "all 0.2s ease",
                  }}
                  transition="all 0.2s ease"
                >
                  <CardBody p={6}>
                    <Flex align="center" justify="space-between">
                      <Stat>
                        <StatLabel
                          color={textSecondary}
                          fontSize="sm"
                          fontWeight="medium"
                          mb={2}
                        >
                          {t("purchases.rejected")}
                        </StatLabel>
                        <StatNumber
                          color={statColors.red}
                          fontSize="2xl"
                          fontWeight="bold"
                        >
                          {stats.rejected}
                        </StatNumber>
                      </Stat>
                      <Box
                        p={3}
                        borderRadius="lg"
                        bg={statBgColors.red}
                        color={statColors.red}
                      >
                        <FiXCircle size={20} />
                      </Box>
                    </Flex>
                  </CardBody>
                </Card>

                <Card
                  bg={cardBg}
                  borderWidth="1px"
                  borderColor={borderColor}
                  boxShadow="sm"
                  borderRadius="lg"
                  _hover={{
                    boxShadow: "md",
                    transform: "translateY(-2px)",
                    transition: "all 0.2s ease",
                  }}
                  transition="all 0.2s ease"
                >
                  <CardBody p={6}>
                    <Flex align="center" justify="space-between">
                      <Stat>
                        <StatLabel
                          color={textSecondary}
                          fontSize="sm"
                          fontWeight="medium"
                          mb={2}
                        >
                          {t("purchases.totalValue")}
                        </StatLabel>
                        <StatNumber
                          color={headingColor}
                          fontSize="lg"
                          fontWeight="bold"
                        >
                          {formatCurrency(stats.totalValue || 0)}
                        </StatNumber>
                      </Stat>
                      <Box
                        p={3}
                        borderRadius="lg"
                        bg={statBgColors.purple}
                        color={statColors.purple}
                      >
                        <FiAlertCircle size={20} />
                      </Box>
                    </Flex>
                  </CardBody>
                </Card>

                <Card
                  bg={cardBg}
                  borderWidth="1px"
                  borderColor={borderColor}
                  boxShadow="sm"
                  borderRadius="lg"
                  _hover={{
                    boxShadow: "md",
                    transform: "translateY(-2px)",
                    transition: "all 0.2s ease",
                  }}
                  transition="all 0.2s ease"
                >
                  <CardBody p={6}>
                    <Flex align="center" justify="space-between">
                      <Stat>
                        <StatLabel
                          color={textSecondary}
                          fontSize="sm"
                          fontWeight="medium"
                          mb={2}
                        >
                          {t("purchases.totalApprovedAmount")}
                        </StatLabel>
                        <StatNumber
                          color={headingColor}
                          fontSize="lg"
                          fontWeight="bold"
                        >
                          {formatCurrency(stats.totalApprovedAmount || 0)}
                        </StatNumber>
                      </Stat>
                      <Box
                        p={3}
                        borderRadius="lg"
                        bg={statBgColors.purple}
                        color={statColors.purple}
                      >
                        <FiAlertCircle size={20} />
                      </Box>
                    </Flex>
                  </CardBody>
                </Card>

                <Card
                  bg={cardBg}
                  borderWidth="1px"
                  borderColor={borderColor}
                  boxShadow="sm"
                  borderRadius="lg"
                  _hover={{
                    boxShadow: "md",
                    transform: "translateY(-2px)",
                    transition: "all 0.2s ease",
                  }}
                  transition="all 0.2s ease"
                >
                  <CardBody p={6}>
                    <Flex align="center" justify="space-between">
                      <Stat>
                        <StatLabel
                          color={textSecondary}
                          fontSize="sm"
                          fontWeight="medium"
                          mb={2}
                        >
                          {t("purchases.totalPaid")}
                        </StatLabel>
                        <StatNumber
                          color={statColors.green}
                          fontSize="lg"
                          fontWeight="bold"
                        >
                          {formatCurrency(stats.totalPaid || 0)}
                        </StatNumber>
                      </Stat>
                      <Box
                        p={3}
                        borderRadius="lg"
                        bg={statBgColors.green}
                        color={statColors.green}
                      >
                        <FiCheckCircle size={20} />
                      </Box>
                    </Flex>
                  </CardBody>
                </Card>

                <Card
                  bg={cardBg}
                  borderWidth="1px"
                  borderColor={borderColor}
                  boxShadow="sm"
                  borderRadius="lg"
                  _hover={{
                    boxShadow: "md",
                    transform: "translateY(-2px)",
                    transition: "all 0.2s ease",
                  }}
                  transition="all 0.2s ease"
                >
                  <CardBody p={6}>
                    <Flex align="center" justify="space-between">
                      <Stat>
                        <StatLabel
                          color={textSecondary}
                          fontSize="sm"
                          fontWeight="medium"
                          mb={2}
                        >
                          {t("purchases.outstandingAmount")}
                        </StatLabel>
                        <StatNumber
                          color={statColors.orange}
                          fontSize="lg"
                          fontWeight="bold"
                        >
                          {formatCurrency(stats.totalOutstanding || 0)}
                        </StatNumber>
                      </Stat>
                      <Box
                        p={3}
                        borderRadius="lg"
                        bg={statBgColors.orange}
                        color={statColors.orange}
                      >
                        <FiClock size={20} />
                      </Box>
                    </Flex>
                  </CardBody>
                </Card>
              </Grid>

              {error && (
                <Alert status="error">
                  <AlertIcon />
                  <AlertTitle>Error!</AlertTitle>
                  <AlertDescription>{error}</AlertDescription>
                </Alert>
              )}

              {accountingMonitoringEnabled && (
                <Card
                  mb={6}
                  bg={filters.accounting_status ? statBgColors.orange : cardBg}
                  borderWidth="1px"
                  borderColor={
                    filters.accounting_status ? "orange.200" : borderColor
                  }
                  boxShadow="sm"
                >
                  <CardBody>
                    <Flex
                      gap={4}
                      align={{ base: "stretch", lg: "center" }}
                      justify="space-between"
                      direction={{ base: "column", lg: "row" }}
                    >
                      <Box>
                        <HStack spacing={3} mb={2}>
                          <Box
                            p={2}
                            borderRadius="lg"
                            bg={statBgColors.orange}
                            color={statColors.orange}
                          >
                            <FiFileText size={18} />
                          </Box>
                          <Heading size="sm">Monitoring Accounting</Heading>
                          <Badge colorScheme="orange" variant="subtle">
                            Siap Input GL/COA
                          </Badge>
                        </HStack>
                        <Text fontSize="sm" color={textSecondary}>
                          Gunakan filter ini untuk melihat Purchase yang sudah
                          dibuat dan sudah disetujui Finance Manager, terutama
                          yang masih menunggu Accounting melengkapi GL/COA.
                        </Text>
                      </Box>

                      <HStack spacing={2} wrap="wrap">
                        <Badge colorScheme="blue" px={3} py={1}>
                          {pagination.total} data tampil
                        </Badge>
                        <Button
                          size="sm"
                          colorScheme="orange"
                          leftIcon={<FiClock />}
                          onClick={() =>
                            handleApplyAccountingMonitoring(
                              "READY_FOR_COA",
                              true,
                            )
                          }
                        >
                          Bulan Ini - Siap COA
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          leftIcon={<FiCheckCircle />}
                          onClick={() =>
                            handleApplyAccountingMonitoring(
                              "APPROVED_BY_FINANCE",
                              true,
                            )
                          }
                        >
                          Semua Approved FM
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          colorScheme="purple"
                          leftIcon={<FiCheckCircle />}
                          onClick={() =>
                            handleApplyAccountingMonitoring(
                              "PENDING_FINAL_APPROVAL",
                              true,
                            )
                          }
                        >
                          Menunggu Final Approval
                        </Button>
                      </HStack>
                    </Flex>
                  </CardBody>
                </Card>
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
                        color={textSecondary}
                      >
                        {t("purchases.searchTransactions")}
                      </Text>
                      <InputGroup>
                        <InputLeftElement pointerEvents="none">
                          <FiSearch color={textSecondary} />
                        </InputLeftElement>
                        <Input
                          placeholder={t("purchases.searchPlaceholder")}
                          value={searchInput}
                          onChange={(e) => handleSearchChange(e.target.value)}
                          bg={cardBg}
                        />
                      </InputGroup>
                    </Box>

                    <Box minW="180px">
                      <Text
                        fontSize="sm"
                        fontWeight="medium"
                        mb={2}
                        color={textSecondary}
                      >
                        {t("purchases.filterByVendor")}
                      </Text>
                      <Select
                        placeholder={t("purchases.allVendors")}
                        value={filters.vendor_id || ""}
                        onChange={(e) =>
                          handleFilterChange({ vendor_id: e.target.value })
                        }
                        bg={cardBg}
                      >
                        {vendors.map((vendor) => (
                          <option key={vendor.id} value={vendor.id.toString()}>
                            {vendor.name}
                          </option>
                        ))}
                      </Select>
                    </Box>

                    <Box minW="160px">
                      <Text
                        fontSize="sm"
                        fontWeight="medium"
                        mb={2}
                        color={textSecondary}
                      >
                        Project
                      </Text>
                      <Select
                        placeholder="All Projects"
                        value={filters.project_id || ""}
                        onChange={(e) =>
                          handleFilterChange({ project_id: e.target.value })
                        }
                        bg={cardBg}
                      >
                        {projects.map((proj) => (
                          <option key={proj.id} value={proj.id.toString()}>
                            {proj.name}
                          </option>
                        ))}
                      </Select>
                    </Box>

                    <Box minW="160px">
                      <Text
                        fontSize="sm"
                        fontWeight="medium"
                        mb={2}
                        color={textSecondary}
                      >
                        {t("purchases.filterByStatus")}
                      </Text>
                      <Select
                        placeholder={t("purchases.allStatuses")}
                        value={filters.status || ""}
                        onChange={(e) =>
                          handleFilterChange({
                            status: e.target.value,
                            accounting_status: "",
                          })
                        }
                        bg={cardBg}
                      >
                        <option value="">{t("purchases.allStatuses")}</option>
                        <option value="draft">{t("purchases.draft")}</option>
                        <option value="pending_approval">
                          {t("purchases.pendingApprovalStatus")}
                        </option>
                        <option value="pending_accounting">
                          Menunggu Accounting
                        </option>
                        <option value="pending_final_approval">
                          Menunggu Final Approval
                        </option>
                        <option value="approved">
                          {t("purchases.approved")}
                        </option>
                        <option value="cancelled">
                          {t("purchases.cancelled")}
                        </option>
                      </Select>
                    </Box>

                    <Box minW="160px">
                      <Text
                        fontSize="sm"
                        fontWeight="medium"
                        mb={2}
                        color={textSecondary}
                      >
                        Outstanding
                      </Text>
                      <Select
                        placeholder="Semua"
                        value={
                          filters.outstanding === true
                            ? "true"
                            : filters.outstanding === false
                              ? "false"
                              : ""
                        }
                        onChange={(e) => {
                          const val = e.target.value;
                          handleFilterChange({
                            outstanding:
                              val === "true"
                                ? true
                                : val === "false"
                                  ? false
                                  : undefined,
                          });
                        }}
                        bg={cardBg}
                      >
                        <option value="">Semua</option>
                        <option value="true">Belum Lunas (Outstanding)</option>
                      </Select>
                    </Box>

                    {accountingMonitoringEnabled && (
                      <Box minW="220px">
                        <Text
                          fontSize="sm"
                          fontWeight="medium"
                          mb={2}
                          color={textSecondary}
                        >
                          Monitoring Accounting
                        </Text>
                        <Select
                          placeholder="Semua Purchase"
                          value={filters.accounting_status || ""}
                          onChange={(e) =>
                            handleFilterChange({
                              accounting_status: e.target.value,
                              status: "",
                              approval_status: "",
                            })
                          }
                          bg={cardBg}
                        >
                          <option value="">Semua Purchase</option>
                          <option value="READY_FOR_COA">
                            Siap Input GL/COA
                          </option>
                          <option value="APPROVED_BY_FINANCE">
                            Approved Finance Manager
                          </option>
                          <option value="PENDING_FINAL_APPROVAL">
                            Menunggu Final Approval
                          </option>
                          <option value="COA_COMPLETED">COA Selesai</option>
                        </Select>
                      </Box>
                    )}

                    <Box minW="160px">
                      <Text
                        fontSize="sm"
                        fontWeight="medium"
                        mb={2}
                        color={textSecondary}
                      >
                        {t("purchases.approvalStatus")}
                      </Text>
                      <Select
                        placeholder={t("purchases.allApprovalStatuses")}
                        value={filters.approval_status || ""}
                        onChange={(e) =>
                          handleFilterChange({
                            approval_status: e.target.value,
                            accounting_status: "",
                          })
                        }
                        bg={cardBg}
                      >
                        <option value="">
                          {t("purchases.allApprovalStatuses")}
                        </option>
                        <option value="not_required">
                          {t("purchases.notRequired")}
                        </option>
                        <option value="pending">
                          {t("purchases.pending")}
                        </option>
                        <option value="approved">
                          {t("purchases.approved")}
                        </option>
                        <option value="rejected">
                          {t("purchases.rejected")}
                        </option>
                      </Select>
                    </Box>

                    <Box minW="160px">
                      <Text
                        fontSize="sm"
                        fontWeight="medium"
                        mb={2}
                        color={textSecondary}
                      >
                        {t("purchases.startDate")}
                      </Text>
                      <Input
                        type="date"
                        value={filters.start_date || ""}
                        onChange={(e) =>
                          handleFilterChange({ start_date: e.target.value })
                        }
                        bg={cardBg}
                      />
                    </Box>

                    <Box minW="160px">
                      <Text
                        fontSize="sm"
                        fontWeight="medium"
                        mb={2}
                        color={textSecondary}
                      >
                        {t("purchases.endDate")}
                      </Text>
                      <Input
                        type="date"
                        value={filters.end_date || ""}
                        onChange={(e) =>
                          handleFilterChange({ end_date: e.target.value })
                        }
                        bg={cardBg}
                      />
                    </Box>

                    <Button
                      leftIcon={<FiFilter />}
                      variant="outline"
                      onClick={handleClearFilters}
                    >
                      {t("purchases.clearFilters")}
                    </Button>
                  </Flex>
                </CardBody>
              </Card>

              {/* Main Data Table */}
              <EnhancedPurchaseTable
                purchases={purchases}
                loading={loading}
                onViewDetails={handleView}
                onEdit={canEdit ? handleEdit : undefined}
                onSubmitForApproval={handleSubmitForApproval}
                onDelete={canDelete ? handleDelete : undefined}
                onDuplicate={canEdit ? handleDuplicate : undefined}
                renderActions={renderActions}
                title={t("purchases.purchaseTransactions")}
                formatCurrency={formatCurrency}
                formatDate={formatDate}
                canEdit={canEdit}
                canDelete={canDelete}
                userRole={normalizeRole(user?.role as any)}
              />

              {purchaseTotalRows > 0 && (
                <Flex
                  mt={4}
                  px={4}
                  py={3}
                  borderWidth="1px"
                  borderColor={borderColor}
                  borderRadius="md"
                  bg={cardBg}
                  align={{ base: "stretch", md: "center" }}
                  justify="space-between"
                  direction={{ base: "column", md: "row" }}
                  gap={3}
                >
                  <Text fontSize="sm" color={textSecondary}>
                    Menampilkan {purchaseStartRow}-{purchaseEndRow} dari{" "}
                    {purchaseTotalRows} purchase
                  </Text>
                  <Flex align="center" gap={2} wrap="wrap">
                    <Text fontSize="sm" color={textSecondary}>
                      Rows
                    </Text>
                    <Select
                      size="sm"
                      maxW="90px"
                      value={pagination.limit}
                      onChange={(e) =>
                        handlePageSizeChange(Number(e.target.value))
                      }
                      isDisabled={loading}
                    >
                      <option value={10}>10</option>
                      <option value={25}>25</option>
                      <option value={50}>50</option>
                    </Select>
                    <Button
                      size="sm"
                      variant="outline"
                      leftIcon={<FiChevronLeft />}
                      onClick={() => handlePageChange(pagination.page - 1)}
                      isDisabled={loading || pagination.page <= 1}
                    >
                      Prev
                    </Button>
                    <Text fontSize="sm" color={textSecondary}>
                      Halaman {pagination.page} / {purchaseTotalPages}
                    </Text>
                    <Button
                      size="sm"
                      variant="outline"
                      rightIcon={<FiChevronRight />}
                      onClick={() => handlePageChange(pagination.page + 1)}
                      isDisabled={
                        loading ||
                        pagination.page >= purchaseTotalPages
                      }
                    >
                      Next
                    </Button>
                  </Flex>
                </Flex>
              )}

              {/* View Purchase Modal */}
              <Modal isOpen={isViewOpen} onClose={onViewClose} size="xl">
                <ModalOverlay />
                <ModalContent bg={modalContentBg}>
                  <ModalHeader
                    bg={modalHeaderBg}
                    borderBottomWidth={1}
                    borderColor={borderColor}
                  >
                    {t("purchases.viewPurchase")} - {selectedPurchase?.code}
                  </ModalHeader>
                  <ModalCloseButton />
                  <ModalBody>
                    {selectedPurchase && (
                      <VStack spacing={6} align="stretch">
                        {/* Show rejection alert for cancelled/rejected purchases */}
                        {(selectedPurchase.status === "CANCELLED" ||
                          selectedPurchase.approval_status === "REJECTED") && (
                          <Alert status="error" variant="left-accent">
                            <AlertIcon />
                            <VStack align="start" spacing={1}>
                              <AlertTitle>
                                {selectedPurchase.status === "CANCELLED"
                                  ? t("purchases.purchaseCancelled")
                                  : t("purchases.purchaseRejected")}
                              </AlertTitle>
                              <AlertDescription>
                                {selectedPurchase.status === "CANCELLED"
                                  ? t("purchases.purchaseCancelledDesc")
                                  : t("purchases.purchaseRejectedDesc")}
                              </AlertDescription>
                            </VStack>
                          </Alert>
                        )}

                        {/* Basic Info */}
                        <SimpleGrid columns={2} spacing={4}>
                          <FormControl>
                            <FormLabel>{t("purchases.purchaseCode")}</FormLabel>
                            <Text fontWeight="medium">
                              {selectedPurchase.code}
                            </Text>
                          </FormControl>

                          <FormControl>
                            <FormLabel>{t("purchases.vendor")}</FormLabel>
                            <Text fontWeight="medium">
                              {selectedPurchase.vendor?.name || "N/A"}
                            </Text>
                          </FormControl>

                          {selectedPurchase.purchase_request_id && (
                            <>
                              <FormControl>
                                <FormLabel>Judul PR</FormLabel>
                                <Text fontWeight="medium">
                                  {selectedPurchase.purchase_request?.title ||
                                    "-"}
                                </Text>
                              </FormControl>

                              <FormControl>
                                <FormLabel>Divisi Pengaju</FormLabel>
                                <Text fontWeight="medium">
                                  {selectedPurchase.purchase_request
                                    ?.department || "-"}
                                </Text>
                              </FormControl>
                            </>
                          )}

                          <FormControl>
                            <FormLabel>{t("purchases.date")}</FormLabel>
                            <Text fontWeight="medium">
                              {new Date(
                                selectedPurchase.date,
                              ).toLocaleDateString("id-ID")}
                            </Text>
                          </FormControl>

                          <FormControl>
                            <FormLabel>{t("purchases.totalAmount")}</FormLabel>
                            <Text fontWeight="medium" color="green.500">
                              {formatCurrency(selectedPurchase.total_amount)}
                            </Text>
                          </FormControl>

                          <FormControl>
                            <FormLabel>{t("purchases.status")}</FormLabel>
                            <Badge
                              colorScheme={getStatusColor(
                                selectedPurchase.status,
                              )}
                              variant="subtle"
                              w="fit-content"
                            >
                              {selectedPurchase.status
                                .replace("_", " ")
                                .toUpperCase()}
                            </Badge>
                          </FormControl>

                          <FormControl>
                            <FormLabel>
                              {t("purchases.approvalStatus")}
                            </FormLabel>
                            <Badge
                              colorScheme={getApprovalStatusColor(
                                selectedPurchase.approval_status,
                              )}
                              variant="subtle"
                              w="fit-content"
                            >
                              {selectedPurchase.approval_status
                                .replace("_", " ")
                                .toUpperCase()}
                            </Badge>
                          </FormControl>
                        </SimpleGrid>

                        {/* Payment Information */}
                        {selectedPurchase.payment_method && (
                          <Box>
                            <FormLabel mb={3}>
                              {t("purchases.paymentInformation")}
                            </FormLabel>
                            <SimpleGrid columns={3} spacing={4}>
                              <FormControl>
                                <FormLabel fontSize="sm">
                                  {t("purchases.paymentMethod")}
                                </FormLabel>
                                <Badge
                                  colorScheme={
                                    selectedPurchase.payment_method === "CREDIT"
                                      ? "orange"
                                      : selectedPurchase.payment_method ===
                                          "CASH"
                                        ? "green"
                                        : selectedPurchase.payment_method ===
                                            "BANK_TRANSFER"
                                          ? "blue"
                                          : selectedPurchase.payment_method ===
                                              "CHECK"
                                            ? "purple"
                                            : "gray"
                                  }
                                  variant="subtle"
                                  w="fit-content"
                                >
                                  {selectedPurchase.payment_method.replace(
                                    "_",
                                    " ",
                                  )}
                                </Badge>
                              </FormControl>

                              {selectedPurchase.bank_account_id && (
                                <FormControl>
                                  <FormLabel fontSize="sm">
                                    {t("purchases.bankAccount")}
                                  </FormLabel>
                                  <Text fontWeight="medium">
                                    {(() => {
                                      // Prefer relation from backend if available
                                      const relatedAccount: any = (
                                        selectedPurchase as any
                                      ).bank_account;
                                      const accountFromList = bankAccounts.find(
                                        (account) =>
                                          account.id ===
                                          selectedPurchase.bank_account_id,
                                      );
                                      const name =
                                        relatedAccount?.name ||
                                        accountFromList?.name;
                                      const code =
                                        relatedAccount?.code ||
                                        accountFromList?.code;
                                      if (!name && !code)
                                        return "Unknown Account";
                                      if (name && code)
                                        return `${name} (${code})`;
                                      return name || code || "Unknown Account";
                                    })()}
                                  </Text>
                                </FormControl>
                              )}

                              {selectedPurchase.payment_reference && (
                                <FormControl>
                                  <FormLabel fontSize="sm">
                                    {t("purchases.paymentReference")}
                                  </FormLabel>
                                  <Text fontWeight="medium">
                                    {selectedPurchase.payment_reference}
                                  </Text>
                                </FormControl>
                              )}
                            </SimpleGrid>
                          </Box>
                        )}

                        {/* Notes */}
                        {selectedPurchase.notes && (
                          <FormControl>
                            <FormLabel>{t("purchases.notes")}</FormLabel>
                            <Text
                              p={3}
                              bg={notesBoxBg}
                              borderRadius="md"
                              border="1px solid"
                              borderColor={borderColor}
                              color={textPrimary}
                            >
                              {selectedPurchase.notes}
                            </Text>
                          </FormControl>
                        )}

                        {/* Approval Panel */}
                        <ApprovalPanel
                          purchaseId={selectedPurchase.id}
                          approvalStatus={selectedPurchase.approval_status}
                          purchaseStatus={selectedPurchase.status}
                          approvalSteps={
                            selectedPurchase.approval_request?.approval_steps ||
                            []
                          }
                          purchaseAmount={selectedPurchase.total_amount}
                          canApprove={canCurrentUserActOnPurchase(
                            selectedPurchase,
                          )}
                          onApprove={async (
                            comments?: string,
                            requiresDirector?: boolean,
                          ) => {
                            if (!selectedPurchase) return;
                            try {
                              if (
                                (selectedPurchase.status || "").toUpperCase() ===
                                "PENDING_FINAL_APPROVAL"
                              ) {
                                await handleFinalApprovePurchase(
                                  selectedPurchase,
                                );
                                return;
                              }

                              const roleNorm = normalizeRole(
                                user?.role as unknown as string,
                              );
                              const activeStep =
                                getCurrentApprovalStepInfo(selectedPurchase);
                              const canEscalateToDirector =
                                isFinanceManagerRole(roleNorm) &&
                                activeStep?.approver_role === "finance";
                              // Call API to approve with escalation parameter
                              const result =
                                await approvalService.approvePurchase(
                                  selectedPurchase.id,
                                  {
                                    comments: comments || "",
                                    escalate_to_director:
                                      Boolean(
                                        requiresDirector &&
                                          canEscalateToDirector,
                                      ),
                                  },
                                );

                              // Handle different approval outcomes
                              if (result.escalated) {
                                toast({
                                  title: "Approved & Escalated",
                                  description:
                                    result.message ||
                                    "Purchase approved by Finance and escalated to Director for final approval",
                                  status: "info",
                                  duration: 5000,
                                  isClosable: true,
                                });

                                // Send notification to directors
                                await notifyDirectors(selectedPurchase);
                              } else {
                                toast({
                                  title: "Approved",
                                  description:
                                    result.message ||
                                    "Purchase approved successfully",
                                  status: "success",
                                  duration: 3000,
                                  isClosable: true,
                                });
                              }

                              // Refresh purchase data
                              const detailResponse =
                                await purchaseService.getById(
                                  selectedPurchase.id,
                                );
                              setSelectedPurchase(detailResponse);
                              await fetchPurchases();
                              // Don't close modal - let user see the updated history with comments
                            } catch (err: any) {
                              toast({
                                title: "Error",
                                description:
                                  err.response?.data?.message ||
                                  err.response?.data?.error ||
                                  "Failed to approve",
                                status: "error",
                                duration: 5000,
                                isClosable: true,
                              });
                            }
                          }}
                          onReject={async (comments: string) => {
                            if (!selectedPurchase) return;
                            if (!comments || comments.trim() === "") {
                              toast({
                                title: "Komentar diperlukan",
                                description: "Mohon isi alasan penolakan.",
                                status: "warning",
                                duration: 3000,
                                isClosable: true,
                              });
                              return;
                            }
                            try {
                              await approvalService.rejectPurchase(
                                selectedPurchase.id,
                                { comments },
                              );
                              toast({
                                title: "Rejected",
                                description: "Purchase rejected successfully",
                                status: "warning",
                                duration: 3000,
                                isClosable: true,
                              });
                              const detailResponse =
                                await purchaseService.getById(
                                  selectedPurchase.id,
                                );
                              setSelectedPurchase(detailResponse);
                              await fetchPurchases();
                              // Don't close modal - let user see the updated history with rejection comments
                            } catch (err: any) {
                              toast({
                                title: "Error",
                                description:
                                  err.response?.data?.message ||
                                  "Failed to reject",
                                status: "error",
                                duration: 5000,
                                isClosable: true,
                              });
                            }
                          }}
                        />

                        {/* Items */}
                        {selectedPurchase.purchase_items &&
                          selectedPurchase.purchase_items.length > 0 && (
                            <FormControl>
                              <FormLabel>
                                {t("purchases.purchaseItems")}
                              </FormLabel>
                              <TableContainer>
                                <Table size="sm" bg={tableBg}>
                                  <Thead bg={tableHeaderBg}>
                                    <Tr>
                                      <Th>{t("purchases.product")}</Th>
                                      <Th isNumeric>
                                        {t("purchases.quantity")}
                                      </Th>
                                      <Th isNumeric>
                                        {t("purchases.unitPrice")}
                                      </Th>
                                      <Th isNumeric>{t("purchases.total")}</Th>
                                    </Tr>
                                  </Thead>
                                  <Tbody>
                                    {selectedPurchase.purchase_items.map(
                                      (item: any, index: number) => (
                                        <Tr key={index}>
                                          <Td>
                                            {item.item_type === "ASSET" ||
                                            item.is_fixed_asset
                                              ? `${item.description || "Unknown Asset"} (Aset Tetap)`
                                              : getPurchaseItemDisplayName(item)}
                                          </Td>
                                          <Td isNumeric>{item.quantity}</Td>
                                          <Td isNumeric>
                                            {formatCurrency(item.unit_price)}
                                          </Td>
                                          <Td isNumeric>
                                            {formatCurrency(
                                              item.quantity * item.unit_price,
                                            )}
                                          </Td>
                                        </Tr>
                                      ),
                                    )}
                                  </Tbody>
                                </Table>
                              </TableContainer>
                            </FormControl>
                          )}
                      </VStack>
                    )}
                  </ModalBody>
                  <ModalFooter>
                    <HStack>
                      <Button onClick={onViewClose}>
                        {t("purchases.close")}
                      </Button>
                    </HStack>
                  </ModalFooter>
                </ModalContent>
              </Modal>

              {/* Edit Purchase Modal */}
              <Modal isOpen={isEditOpen} onClose={onEditClose} size="2xl">
                <ModalOverlay />
                <ModalContent bg={modalContentBg}>
                  <ModalHeader
                    bg={modalHeaderBg}
                    borderBottomWidth={1}
                    borderColor={borderColor}
                  >
                    Edit Purchase - {selectedPurchase?.code}
                  </ModalHeader>
                  <ModalCloseButton />
                  <ModalBody>
                    <VStack spacing={4} align="stretch">
                      {/* Tax Configuration Section */}
                      <Card>
                        <CardHeader pb={3}>
                          <Text
                            fontSize="md"
                            fontWeight="semibold"
                            color={textPrimary}
                          >
                            💰 Tax Configuration
                          </Text>
                        </CardHeader>
                        <CardBody pt={0}>
                          <VStack spacing={4} align="stretch">
                            {/* Tax Additions (Penambahan) */}
                            <Box>
                              <Text
                                fontSize="sm"
                                fontWeight="medium"
                                color={statColors.green}
                                mb={3}
                              >
                                ➕ Tax Additions (Penambahan)
                              </Text>
                              <SimpleGrid
                                columns={{ base: 1, md: 3 }}
                                spacing={4}
                              >
                                <FormControl>
                                  <FormLabel fontSize="sm">
                                    PPN Rate (%)
                                  </FormLabel>
                                  <NumberInput
                                    value={formData.ppn_rate}
                                    onChange={(value) =>
                                      setFormData({
                                        ...formData,
                                        ppn_rate: value,
                                      })
                                    }
                                    size="sm"
                                    min={0}
                                    max={100}
                                    step={0.1}
                                  >
                                    <NumberInputField placeholder="11" />
                                    <NumberInputStepper>
                                      <NumberIncrementStepper />
                                      <NumberDecrementStepper />
                                    </NumberInputStepper>
                                  </NumberInput>
                                  <FormHelperText fontSize="xs">
                                    Pajak Pertambahan Nilai (default 11%)
                                  </FormHelperText>
                                </FormControl>

                                <FormControl>
                                  <FormLabel fontSize="sm">
                                    Other Tax Additions (%)
                                  </FormLabel>
                                  <NumberInput
                                    value={formData.other_tax_additions}
                                    onChange={(value) =>
                                      setFormData({
                                        ...formData,
                                        other_tax_additions: value,
                                      })
                                    }
                                    size="sm"
                                    min={0}
                                    max={100}
                                    step={0.1}
                                  >
                                    <NumberInputField placeholder="0" />
                                    <NumberInputStepper>
                                      <NumberIncrementStepper />
                                      <NumberDecrementStepper />
                                    </NumberInputStepper>
                                  </NumberInput>
                                  <FormHelperText fontSize="xs">
                                    Pajak tambahan lainnya (opsional)
                                  </FormHelperText>
                                </FormControl>

                                <FormControl>
                                  <FormLabel fontSize="sm">
                                    Freight / Biaya Pengiriman
                                  </FormLabel>
                                  <CurrencyInput
                                    value={
                                      parseFloat(formData.freight_cost) || 0
                                    }
                                    onChange={(value) =>
                                      setFormData({
                                        ...formData,
                                        freight_cost: value.toString(),
                                      })
                                    }
                                    placeholder="0"
                                    size="sm"
                                    min={0}
                                  />
                                  <FormHelperText fontSize="xs">
                                    Dikapitalisasi ke inventory valuation/HPP.
                                  </FormHelperText>
                                </FormControl>
                              </SimpleGrid>
                            </Box>

                            <Divider />

                            {/* Tax Deductions (Pemotongan) */}
                            <Box>
                              <Text
                                fontSize="sm"
                                fontWeight="medium"
                                color={statColors.red}
                                mb={3}
                              >
                                ➖ Tax Deductions (Pemotongan)
                              </Text>
                              <SimpleGrid columns={3} spacing={4}>
                                <FormControl>
                                  <FormLabel fontSize="sm">
                                    PPh 21 Rate (%)
                                  </FormLabel>
                                  <NumberInput
                                    value={formData.pph21_rate}
                                    onChange={(value) =>
                                      setFormData({
                                        ...formData,
                                        pph21_rate: value,
                                      })
                                    }
                                    size="sm"
                                    min={0}
                                    max={100}
                                    step={0.1}
                                  >
                                    <NumberInputField placeholder="0" />
                                    <NumberInputStepper>
                                      <NumberIncrementStepper />
                                      <NumberDecrementStepper />
                                    </NumberInputStepper>
                                  </NumberInput>
                                  <FormHelperText fontSize="xs">
                                    Pajak Penghasilan Pasal 21
                                  </FormHelperText>
                                </FormControl>

                                <FormControl>
                                  <FormLabel fontSize="sm">
                                    PPh 23 Rate (%)
                                  </FormLabel>
                                  <NumberInput
                                    value={formData.pph23_rate}
                                    onChange={(value) =>
                                      setFormData({
                                        ...formData,
                                        pph23_rate: value,
                                      })
                                    }
                                    size="sm"
                                    min={0}
                                    max={100}
                                    step={0.1}
                                  >
                                    <NumberInputField placeholder="0" />
                                    <NumberInputStepper>
                                      <NumberIncrementStepper />
                                      <NumberDecrementStepper />
                                    </NumberInputStepper>
                                  </NumberInput>
                                  <FormHelperText fontSize="xs">
                                    Pajak Penghasilan Pasal 23
                                  </FormHelperText>
                                </FormControl>

                                <FormControl>
                                  <FormLabel fontSize="sm">
                                    Other Tax Deductions (%)
                                  </FormLabel>
                                  <NumberInput
                                    value={formData.other_tax_deductions}
                                    onChange={(value) =>
                                      setFormData({
                                        ...formData,
                                        other_tax_deductions: value,
                                      })
                                    }
                                    size="sm"
                                    min={0}
                                    max={100}
                                    step={0.1}
                                  >
                                    <NumberInputField placeholder="0" />
                                    <NumberInputStepper>
                                      <NumberIncrementStepper />
                                      <NumberDecrementStepper />
                                    </NumberInputStepper>
                                  </NumberInput>
                                  <FormHelperText fontSize="xs">
                                    Potongan pajak lainnya (opsional)
                                  </FormHelperText>
                                </FormControl>
                              </SimpleGrid>
                            </Box>

                            {/* Tax Summary Calculation */}
                            {formData.items.length > 0 && (
                              <Box
                                mt={4}
                                p={4}
                                bg={notesBoxBg}
                                borderRadius="md"
                                border="1px solid"
                                borderColor={borderColor}
                              >
                                <VStack spacing={2} align="stretch">
                                  <Text
                                    fontSize="sm"
                                    fontWeight="semibold"
                                    color={textPrimary}
                                  >
                                    Tax Summary:
                                  </Text>
                                  {(() => {
                                    const subtotal = formData.items.reduce(
                                      (total, item) => {
                                        const qty = parseFloat(
                                          item.quantity || "0",
                                        );
                                        const price = parseFloat(
                                          item.unit_price || "0",
                                        );
                                        const discount = parseFloat(
                                          item.discount || "0",
                                        );
                                        const itemSubtotal =
                                          (isNaN(qty) ? 0 : qty) *
                                          (isNaN(price) ? 0 : price);
                                        const lineTotal =
                                          itemSubtotal -
                                          (isNaN(discount) ? 0 : discount);
                                        return total + lineTotal;
                                      },
                                      0,
                                    );

                                    const discount =
                                      (parseFloat(formData.discount) || 0) /
                                      100;
                                    const discountedSubtotal =
                                      subtotal * (1 - discount);

                                    const ppnAmount =
                                      (discountedSubtotal *
                                        (parseFloat(formData.ppn_rate) || 0)) /
                                      100;
                                    const otherAdditions =
                                      (discountedSubtotal *
                                        (parseFloat(
                                          formData.other_tax_additions,
                                        ) || 0)) /
                                      100;
                                    const totalAdditions =
                                      ppnAmount + otherAdditions;
                                    const freightCost =
                                      parseFloat(formData.freight_cost) || 0;

                                    const pph21Amount =
                                      (discountedSubtotal *
                                        (parseFloat(formData.pph21_rate) ||
                                          0)) /
                                      100;
                                    const pph23Amount =
                                      (discountedSubtotal *
                                        (parseFloat(formData.pph23_rate) ||
                                          0)) /
                                      100;
                                    const otherDeductions =
                                      (discountedSubtotal *
                                        (parseFloat(
                                          formData.other_tax_deductions,
                                        ) || 0)) /
                                      100;
                                    const totalDeductions =
                                      pph21Amount +
                                      pph23Amount +
                                      otherDeductions;

                                    const grossBeforePPh =
                                      discountedSubtotal +
                                      freightCost +
                                      totalAdditions;
                                    const finalTotal =
                                      grossBeforePPh - totalDeductions;

                                    return (
                                      <SimpleGrid
                                        columns={2}
                                        spacing={4}
                                        fontSize="xs"
                                      >
                                        <VStack align="start" spacing={1}>
                                          <Text color={textSecondary}>
                                            Subtotal: {formatCurrency(subtotal)}
                                          </Text>
                                          <Text color={textSecondary}>
                                            Discount ({formData.discount}%): -
                                            {formatCurrency(
                                              subtotal * discount,
                                            )}
                                          </Text>
                                          <Text color={textSecondary}>
                                            After Discount:{" "}
                                            {formatCurrency(discountedSubtotal)}
                                          </Text>
                                          <Text
                                            color={statColors.blue}
                                            fontWeight="semibold"
                                          >
                                            Harga Item sebelum PPh:{" "}
                                            {formatCurrency(discountedSubtotal)}
                                          </Text>
                                        </VStack>

                                        <VStack align="start" spacing={1}>
                                          <Text color={statColors.green}>
                                            + PPN ({formData.ppn_rate}%):{" "}
                                            {formatCurrency(ppnAmount)}
                                          </Text>
                                          {parseFloat(
                                            formData.other_tax_additions,
                                          ) > 0 && (
                                            <Text color={statColors.green}>
                                              + Other Additions (
                                              {formData.other_tax_additions}%):{" "}
                                              {formatCurrency(otherAdditions)}
                                            </Text>
                                          )}
                                          {freightCost > 0 && (
                                            <Text color={statColors.green}>
                                              + Freight / Pengiriman:{" "}
                                              {formatCurrency(freightCost)}
                                            </Text>
                                          )}
                                          {parseFloat(formData.pph21_rate) >
                                            0 && (
                                            <Text color={statColors.red}>
                                              - PPh 21 ({formData.pph21_rate}%):{" "}
                                              {formatCurrency(pph21Amount)}
                                            </Text>
                                          )}
                                          {parseFloat(formData.pph23_rate) >
                                            0 && (
                                            <Text color={statColors.red}>
                                              - PPh 23 ({formData.pph23_rate}%):{" "}
                                              {formatCurrency(pph23Amount)}
                                            </Text>
                                          )}
                                          {parseFloat(
                                            formData.other_tax_deductions,
                                          ) > 0 && (
                                            <Text color={statColors.red}>
                                              - Other Deductions (
                                              {formData.other_tax_deductions}%):{" "}
                                              {formatCurrency(otherDeductions)}
                                            </Text>
                                          )}
                                          <Text color={textSecondary}>
                                            Tagihan sebelum PPh:{" "}
                                            {formatCurrency(grossBeforePPh)}
                                          </Text>
                                          <Text color={statColors.red}>
                                            Amount PPh dipotong: -
                                            {formatCurrency(totalDeductions)}
                                          </Text>
                                          <Text
                                            fontWeight="bold"
                                            color={statColors.blue}
                                            borderTop="1px solid"
                                            borderColor={borderColor}
                                            pt={1}
                                          >
                                            Net Payment Vendor:{" "}
                                            {formatCurrency(finalTotal)}
                                          </Text>
                                        </VStack>
                                      </SimpleGrid>
                                    );
                                  })()}
                                </VStack>
                              </Box>
                            )}
                          </VStack>
                        </CardBody>
                      </Card>

                      {/* Payment Method Section */}
                      <Card>
                        <CardHeader pb={3}>
                          <Text
                            fontSize="md"
                            fontWeight="semibold"
                            color={textPrimary}
                          >
                            💳 Payment Method
                          </Text>
                        </CardHeader>
                        <CardBody pt={0}>
                          <SimpleGrid columns={2} spacing={4}>
                            <FormControl isRequired>
                              <FormLabel fontSize="sm" fontWeight="medium">
                                Payment Method
                              </FormLabel>
                              <Select
                                value={formData.payment_method}
                                onChange={(e) => {
                                  setFormData(
                                    normalizePurchasePaymentFields(
                                      formData,
                                      e.target.value,
                                    ),
                                  );
                                }}
                                size="sm"
                              >
                                <option value="CREDIT">Credit</option>
                                <option value="CASH">Cash</option>
                                <option value="BANK_TRANSFER">
                                  Bank Transfer
                                </option>
                                <option value="CHECK">Check</option>
                              </Select>
                              <FormHelperText fontSize="xs">
                                {formData.payment_method === "CREDIT" &&
                                  "Purchase on credit - payment due later"}
                                {formData.payment_method === "CASH" &&
                                  "Direct cash payment"}
                                {formData.payment_method === "BANK_TRANSFER" &&
                                  "Electronic bank transfer"}
                                {formData.payment_method === "CHECK" &&
                                  "Payment by check"}
                              </FormHelperText>
                            </FormControl>

                            {/* Cash/Bank Account dropdown for Bank Transfer, Cash, Check */}
                            {formData.payment_method !== "CREDIT" && (
                              <FormControl isRequired>
                                <FormLabel fontSize="sm" fontWeight="medium">
                                  {formData.payment_method === "CASH"
                                    ? "Cash Account"
                                    : "Bank Account"}
                                </FormLabel>
                                <Select
                                  value={formData.bank_account_id}
                                  onChange={(e) =>
                                    setFormData({
                                      ...formData,
                                      bank_account_id: e.target.value,
                                    })
                                  }
                                  size="sm"
                                  disabled={loadingBankAccounts}
                                  placeholder={
                                    loadingBankAccounts
                                      ? "Loading accounts..."
                                      : formData.payment_method === "CASH"
                                        ? "Select cash account"
                                        : "Select bank account"
                                  }
                                >
                                  {bankAccounts
                                    .filter((account) => {
                                      // Filter berdasarkan payment method
                                      if (formData.payment_method === "CASH") {
                                        return account.type === "CASH";
                                      } else {
                                        return account.type === "BANK";
                                      }
                                    })
                                    .map((account) => (
                                      <option
                                        key={account.id}
                                        value={account.id.toString()}
                                      >
                                        {account.name} ({account.code}) -{" "}
                                        {account.currency}{" "}
                                        {account.balance?.toLocaleString() ||
                                          "0"}
                                      </option>
                                    ))}
                                </Select>
                                <FormHelperText fontSize="xs">
                                  Required: Select{" "}
                                  {formData.payment_method === "CASH"
                                    ? "cash"
                                    : "bank"}{" "}
                                  account for payment processing
                                </FormHelperText>
                              </FormControl>
                            )}

                            {/* Credit Account dropdown for Credit payment */}
                            {formData.payment_method === "CREDIT" && (
                              <FormControl isRequired={canSelectAccountFromCOA}>
                                <FormLabel fontSize="sm" fontWeight="medium">
                                  Liability Account
                                </FormLabel>
                                {canSelectAccountFromCOA ? (
                                  <Select
                                    value={formData.credit_account_id}
                                    onChange={(e) =>
                                      setFormData({
                                        ...formData,
                                        credit_account_id: e.target.value,
                                      })
                                    }
                                    size="sm"
                                    disabled={loadingCreditAccounts}
                                    placeholder={
                                      loadingCreditAccounts
                                        ? "Loading accounts..."
                                        : "Select liability account"
                                    }
                                  >
                                    {creditAccounts.map((account) => (
                                      <option
                                        key={account.id}
                                        value={account.id?.toString()}
                                      >
                                        {account.code} - {account.name}
                                      </option>
                                    ))}
                                  </Select>
                                ) : (
                                  <Box
                                    p={3}
                                    borderWidth="1px"
                                    borderColor={inputBorderColor}
                                    borderRadius="md"
                                    bg={inputBg}
                                  >
                                    <Text fontSize="xs" fontWeight="semibold">
                                      COA Liability Belum Dipilih
                                    </Text>
                                    <Text fontSize="xs" color={textSecondary}>
                                      GL/COA akan dilengkapi Accounting setelah
                                      approval Finance Manager.
                                    </Text>
                                  </Box>
                                )}
                                <FormHelperText fontSize="xs">
                                  {canSelectAccountFromCOA
                                    ? "Required: Select liability account for tracking credit purchases"
                                    : "COA liability dikosongkan dan akan dilengkapi oleh Accounting sebelum final approval."}
                                </FormHelperText>
                              </FormControl>
                            )}
                          </SimpleGrid>

                          {/* Payment Reference (for non-credit and non-cash payments) */}
                          {formData.payment_method !== "CREDIT" &&
                            formData.payment_method !== "CASH" && (
                              <FormControl mt={4}>
                                <FormLabel fontSize="sm" fontWeight="medium">
                                  Payment Reference
                                </FormLabel>
                                <Input
                                  value={formData.payment_reference}
                                  onChange={(e) =>
                                    setFormData({
                                      ...formData,
                                      payment_reference: e.target.value,
                                    })
                                  }
                                  placeholder={
                                    formData.payment_method === "CHECK"
                                      ? "Check number"
                                      : formData.payment_method ===
                                          "BANK_TRANSFER"
                                        ? "Transaction ID or reference number"
                                        : "Payment reference number"
                                  }
                                  size="sm"
                                />
                                <FormHelperText fontSize="xs">
                                  Optional reference for tracking this payment
                                </FormHelperText>
                              </FormControl>
                            )}
                        </CardBody>
                      </Card>
                    </VStack>
                  </ModalBody>
                  <ModalFooter>
                    <HStack spacing={3}>
                      <Button variant="ghost" onClick={onEditClose}>
                        Cancel
                      </Button>
                      <Button colorScheme="blue" onClick={handleSave}>
                        Update Purchase
                      </Button>
                    </HStack>
                  </ModalFooter>
                </ModalContent>
              </Modal>

              {/* Create Purchase Modal */}
              <Modal isOpen={isCreateOpen} onClose={onCreateClose} size="6xl">
                <ModalOverlay />
                <ModalContent maxW="95vw" maxH="95vh" bg={modalContentBg}>
                  <ModalHeader
                    bg={modalHeaderBg}
                    borderRadius="md"
                    mx={4}
                    mt={4}
                    mb={2}
                    borderBottomWidth={1}
                    borderColor={borderColor}
                  >
                    <HStack>
                      <Box
                        w={1}
                        h={6}
                        bg={statColors.blue}
                        borderRadius="full"
                      />
                      <Text
                        fontSize="lg"
                        fontWeight="bold"
                        color={statColors.blue}
                      >
                        Create New Purchase
                      </Text>
                    </HStack>
                  </ModalHeader>
                  <ModalCloseButton top={6} right={6} />
                  <ModalBody overflowY="auto" px={6} pb={2}>
                    <VStack spacing={6} align="stretch">
                      {/* Basic Information Section */}
                      <Card>
                        <CardHeader pb={3}>
                          <Text
                            fontSize="md"
                            fontWeight="semibold"
                            color={textPrimary}
                          >
                            📋 Basic Information
                          </Text>
                        </CardHeader>
                        <CardBody pt={0}>
                          <SimpleGrid
                            columns={{ base: 1, md: 2, lg: 3 }}
                            spacing={4}
                          >
                            <FormControl>
                              <FormLabel fontSize="sm" fontWeight="medium">
                                {t("purchases.purchaseCode")}
                              </FormLabel>
                              <Input
                                value={t("purchases.autoGenerated")}
                                isReadOnly
                                bg={readOnlyInputBg}
                                borderColor={inputBorderColor}
                                size="sm"
                                color={helperTextColor}
                              />
                              <FormHelperText
                                fontSize="xs"
                                color={helperTextColor}
                              >
                                {t("purchases.autoGeneratedDesc")}
                              </FormHelperText>
                            </FormControl>

                            <FormControl isRequired>
                              <FormLabel fontSize="sm" fontWeight="medium">
                                Vendor
                              </FormLabel>
                              <HStack spacing={2}>
                                {loadingVendors ? (
                                  <Spinner size="sm" />
                                ) : (
                                  <Select
                                    placeholder="Select vendor"
                                    value={formData.vendor_id}
                                    onChange={(e) =>
                                      setFormData({
                                        ...formData,
                                        vendor_id: e.target.value,
                                      })
                                    }
                                    size="sm"
                                    flex={1}
                                    bg={inputBg}
                                    color={inputTextColor}
                                    borderColor={inputBorderColor}
                                    _hover={{ borderColor: inputHoverBorder }}
                                    _focusVisible={fieldFocusStyles}
                                    _placeholder={{
                                      color: inputPlaceholderColor,
                                    }}
                                  >
                                    {vendors.map((vendor) => (
                                      <option key={vendor.id} value={vendor.id}>
                                        {vendor.name} ({vendor.code})
                                      </option>
                                    ))}
                                  </Select>
                                )}
                                <IconButton
                                  aria-label="Add new vendor"
                                  icon={<FiPlus />}
                                  size="sm"
                                  colorScheme="green"
                                  variant="outline"
                                  onClick={onAddVendorOpen}
                                  title="Add New Vendor"
                                  _hover={{ bg: greenButtonHoverBg }}
                                />
                              </HStack>
                            </FormControl>

                            <FormControl>
                              <FormLabel fontSize="sm" fontWeight="medium">
                                Purchase Request
                              </FormLabel>
                              {loadingPRs ? (
                                <Spinner size="sm" />
                              ) : (
                                <Select
                                  placeholder="Select PR (Optional)"
                                  value={formData.purchase_request_id}
                                  onChange={(e) => {
                                    const selectedPrId = e.target.value;
                                    const selectedPr = purchaseRequests.find(
                                      (pr) => pr.id.toString() === selectedPrId,
                                    );
                                    const approvedItems =
                                      getSelectablePurchaseRequestItems(
                                        selectedPr,
                                        selectedPurchase?.id ?? null,
                                      );
                                    const purchaseItemsFromPr =
                                      approvedItems.map((item) =>
                                        mapPurchaseRequestItemToFormData(
                                          item,
                                          defaultExpenseAccountId,
                                        ),
                                      );
                                    const selectedPrTitle =
                                      getPurchaseRequestTitle(selectedPr);

                                    setFormData((prev) => ({
                                      ...prev,
                                      purchase_request_id: selectedPrId,
                                      project_id: selectedPr?.project_id
                                        ? selectedPr.project_id.toString()
                                        : prev.project_id,
                                      items: selectedPrId
                                        ? purchaseItemsFromPr
                                        : prev.items.filter(
                                            (item) =>
                                              !item.purchase_request_item_id,
                                          ),
                                      notes: selectedPr?.notes
                                        ? `Dari PR: ${selectedPr.pr_number}${
                                            selectedPrTitle
                                              ? ` - ${selectedPrTitle}`
                                              : ""
                                          }. ${selectedPr.notes}`
                                        : prev.notes,
                                    }));
                                  }}
                                  size="sm"
                                  bg={inputBg}
                                  color={inputTextColor}
                                  borderColor={inputBorderColor}
                                  _hover={{ borderColor: inputHoverBorder }}
                                  _focusVisible={fieldFocusStyles}
                                  _placeholder={{
                                    color: inputPlaceholderColor,
                                  }}
                                >
                                  {purchaseRequests.map((pr) => (
                                    <option key={pr.id} value={pr.id}>
                                      {getPurchaseRequestOptionLabel(pr)}
                                    </option>
                                  ))}
                                </Select>
                              )}
                              <FormHelperText
                                fontSize="xs"
                                color={warningTextColor}
                              >
                                {selectedPurchaseRequest
                                  ? selectablePurchaseRequestItems.length > 0
                                    ? `${selectablePurchaseRequestItems.length} item approved dari PR ini otomatis dimuat ke form purchase.`
                                    : "Tidak ada item approved yang belum dipakai dari PR ini."
                                  : "Warning: Amount cannot exceed PR limit"}
                              </FormHelperText>
                            </FormControl>

                            <FormControl>
                              <FormLabel fontSize="sm" fontWeight="medium">
                                Project
                              </FormLabel>
                              {loadingProjects ? (
                                <Spinner size="sm" />
                              ) : (
                                <Select
                                  placeholder="Select Project (Optional)"
                                  value={formData.project_id}
                                  onChange={(e) =>
                                    setFormData({
                                      ...formData,
                                      project_id: e.target.value,
                                    })
                                  }
                                  size="sm"
                                  bg={inputBg}
                                  color={inputTextColor}
                                  borderColor={inputBorderColor}
                                  _hover={{ borderColor: inputHoverBorder }}
                                  _focusVisible={fieldFocusStyles}
                                  _placeholder={{
                                    color: inputPlaceholderColor,
                                  }}
                                >
                                  {projects.map((proj) => (
                                    <option key={proj.id} value={proj.id}>
                                      {proj.name} ({proj.code})
                                    </option>
                                  ))}
                                </Select>
                              )}
                            </FormControl>

                            <FormControl isRequired>
                              <FormLabel fontSize="sm" fontWeight="medium">
                                Purchase Date
                              </FormLabel>
                              <Input
                                type="date"
                                size="sm"
                                value={formData.date}
                                onChange={(e) =>
                                  setFormData({
                                    ...formData,
                                    date: e.target.value,
                                  })
                                }
                                bg={inputBg}
                                color={inputTextColor}
                                borderColor={inputBorderColor}
                                _hover={{ borderColor: inputHoverBorder }}
                                _focusVisible={fieldFocusStyles}
                              />
                            </FormControl>

                            <FormControl>
                              <FormLabel fontSize="sm" fontWeight="medium">
                                Due Date
                              </FormLabel>
                              <Input
                                type="date"
                                size="sm"
                                value={formData.due_date}
                                onChange={(e) =>
                                  setFormData({
                                    ...formData,
                                    due_date: e.target.value,
                                  })
                                }
                                bg={inputBg}
                                color={inputTextColor}
                                borderColor={inputBorderColor}
                                _hover={{ borderColor: inputHoverBorder }}
                                _focusVisible={fieldFocusStyles}
                              />
                            </FormControl>
                          </SimpleGrid>

                          <SimpleGrid columns={2} spacing={4} mt={4}>
                            <FormControl>
                              <FormLabel fontSize="sm" fontWeight="medium">
                                Discount (%)
                              </FormLabel>
                              <NumberInput
                                value={formData.discount}
                                onChange={(value) =>
                                  setFormData({ ...formData, discount: value })
                                }
                                size="sm"
                                min={0}
                                max={100}
                              >
                                <NumberInputField
                                  placeholder="0"
                                  bg={inputBg}
                                  color={inputTextColor}
                                  borderColor={inputBorderColor}
                                  _hover={{ borderColor: inputHoverBorder }}
                                  _focusVisible={fieldFocusStyles}
                                />
                                <NumberInputStepper>
                                  <NumberIncrementStepper />
                                  <NumberDecrementStepper />
                                </NumberInputStepper>
                              </NumberInput>
                              <FormHelperText
                                fontSize="xs"
                                color={helperTextColor}
                              >
                                Masukkan persentase diskon atas subtotal (0-100)
                              </FormHelperText>
                            </FormControl>

                            <FormControl>
                              <FormLabel fontSize="sm" fontWeight="medium">
                                Notes
                              </FormLabel>
                              <Textarea
                                value={formData.notes}
                                onChange={(e) =>
                                  setFormData({
                                    ...formData,
                                    notes: e.target.value,
                                  })
                                }
                                placeholder="Enter any notes or descriptions..."
                                rows={3}
                                size="sm"
                                resize="vertical"
                                bg={inputBg}
                                color={inputTextColor}
                                borderColor={inputBorderColor}
                                _hover={{ borderColor: inputHoverBorder }}
                                _focusVisible={fieldFocusStyles}
                                _placeholder={{ color: inputPlaceholderColor }}
                              />
                            </FormControl>
                          </SimpleGrid>
                        </CardBody>
                      </Card>

                      {/* Purchase Items Section */}
                      <Card>
                        <CardHeader pb={3}>
                          <Flex justify="space-between" align="center">
                            <Text
                              fontSize="md"
                              fontWeight="semibold"
                              color={textPrimary}
                            >
                              🛒 Purchase Items
                            </Text>
                            <Button
                              size="sm"
                              leftIcon={<FiPlus />}
                              colorScheme="blue"
                              variant="outline"
                              onClick={() => {
                                setFormData({
                                  ...formData,
                                  items: [
                                    ...formData.items,
                                    createPurchaseItemFormData({
                                      expense_account_id:
                                        isServiceCompany &&
                                        expenseAccounts.length > 0
                                          ? expenseAccounts[0].id!.toString()
                                          : "",
                                    }),
                                  ],
                                });
                              }}
                            >
                              Add Item
                            </Button>
                          </Flex>
                        </CardHeader>
                        <CardBody pt={0}>
                          <Box overflow="visible">
                            <Table size="sm" variant="simple">
                              <Thead bg={tableHeaderBg}>
                                <Tr>
                                  <Th
                                    fontSize="xs"
                                    fontWeight="semibold"
                                    color={textSecondary}
                                    w="130px"
                                  >
                                    Tipe Item
                                  </Th>
                                  <Th
                                    fontSize="xs"
                                    fontWeight="semibold"
                                    color={textSecondary}
                                  >
                                    Produk / Deskripsi Aset
                                  </Th>
                                  <Th
                                    fontSize="xs"
                                    fontWeight="semibold"
                                    color={textSecondary}
                                    isNumeric
                                  >
                                    Qty
                                  </Th>
                                  <Th
                                    fontSize="xs"
                                    fontWeight="semibold"
                                    color={textSecondary}
                                    isNumeric
                                  >
                                    Unit Price (IDR)
                                  </Th>
                                  <Th
                                    fontSize="xs"
                                    fontWeight="semibold"
                                    color={textSecondary}
                                    isNumeric
                                  >
                                    Discount (IDR)
                                  </Th>
                                  <Th
                                    fontSize="xs"
                                    fontWeight="semibold"
                                    color={textSecondary}
                                  >
                                    {isServiceCompany
                                      ? "Akun Beban *"
                                      : "Account"}
                                  </Th>
                                  <Th
                                    fontSize="xs"
                                    fontWeight="semibold"
                                    color={textSecondary}
                                    isNumeric
                                  >
                                    Line Total (IDR)
                                  </Th>
                                  <Th
                                    fontSize="xs"
                                    fontWeight="semibold"
                                    color={textSecondary}
                                    w="60px"
                                  >
                                    Action
                                  </Th>
                                </Tr>
                              </Thead>
                              <Tbody>
                                {formData.items.length === 0 ? (
                                  <Tr>
                                    <Td colSpan={8} textAlign="center" py={8}>
                                      <VStack spacing={2}>
                                        <Text
                                          fontSize="sm"
                                          color={textSecondary}
                                        >
                                          No items added yet
                                        </Text>
                                        <Text
                                          fontSize="xs"
                                          color={textSecondary}
                                        >
                                          Click &quot;Add Item&quot; button to
                                          start adding purchase items
                                        </Text>
                                      </VStack>
                                    </Td>
                                  </Tr>
                                ) : (
                                  formData.items.map((item, index) => (
                                    <Tr key={index} _hover={{ bg: hoverBg }}>
                                      {/* Kolom Tipe Item - Dropdown (Select) diganti dari Radio */}
                                      <Td verticalAlign="top" pt={4}>
                                        <Select
                                          value={item.item_type || "INVENTORY"}
                                          onChange={(e) => {
                                            const val = e.target.value;
                                            const items = [...formData.items];
                                            items[index] = {
                                              ...items[index],
                                              item_type: val,
                                              is_fixed_asset: val === "ASSET",
                                              // Reset fields saat ganti tipe
                                              product_id:
                                                val !== "INVENTORY"
                                                  ? ""
                                                  : items[index].product_id,
                                              description:
                                                val === "INVENTORY"
                                                  ? ""
                                                  : items[index].description,
                                              asset_category_id:
                                                val === "ASSET"
                                                  ? items[index]
                                                      .asset_category_id
                                                  : "",
                                              amortization_month:
                                                val === "PREPAID" ? "12" : "",
                                            };
                                            setFormData({ ...formData, items });
                                          }}
                                          size="sm"
                                          w="130px"
                                          fontSize="xs"
                                          fontWeight="medium"
                                          cursor="pointer"
                                          bg={
                                            item.item_type === "PREPAID"
                                              ? prepaidTypeBg
                                              : item.item_type === "ASSET"
                                                ? assetTypeBg
                                                : inventoryTypeBg
                                          }
                                          borderColor={
                                            item.item_type === "PREPAID"
                                              ? prepaidTypeBorder
                                              : item.item_type === "ASSET"
                                                ? assetTypeBorder
                                                : inventoryTypeBorder
                                          }
                                          color={
                                            item.item_type === "PREPAID"
                                              ? prepaidTypeText
                                              : item.item_type === "ASSET"
                                                ? assetTypeText
                                                : inventoryTypeText
                                          }
                                          _hover={{
                                            bg:
                                              item.item_type === "PREPAID"
                                                ? prepaidTypeHoverBg
                                                : item.item_type === "ASSET"
                                                  ? assetTypeHoverBg
                                                  : inventoryTypeHoverBg,
                                          }}
                                          _focusVisible={fieldFocusStyles}
                                        >
                                          <option value="INVENTORY">
                                            📦 Persediaan
                                          </option>
                                          <option value="ASSET">
                                            🏢 Aset Tetap
                                          </option>
                                          <option value="PREPAID">
                                            ⏳ Prepaid Expense
                                          </option>
                                          <option value="EXPENSE">
                                            Beban / Jasa Manual
                                          </option>
                                        </Select>
                                      </Td>

                                      {/* Kolom Produk / Deskripsi Aset / Prepaid - Kondisional */}
                                      <Td
                                        minW="220px"
                                        verticalAlign="top"
                                        pt={3}
                                      >
                                        {item.item_type === "EXPENSE" ? (
                                          <Input
                                            placeholder="Nama beban/jasa, contoh: Jasa servis AC"
                                            value={item.description}
                                            onChange={(e) => {
                                              const items = [...formData.items];
                                              items[index] = {
                                                ...items[index],
                                                description: e.target.value,
                                              };
                                              setFormData({ ...formData, items });
                                            }}
                                            size="sm"
                                            bg={inputBg}
                                            color={inputTextColor}
                                            borderColor={inputBorderColor}
                                            _hover={{
                                              borderColor: inputHoverBorder,
                                            }}
                                            _focusVisible={fieldFocusStyles}
                                            _placeholder={{
                                              color: inputPlaceholderColor,
                                            }}
                                          />
                                        ) : item.item_type === "ASSET" ? (
                                          /* === ASSET MODE: Input Deskripsi + Kategori Aset === */
                                          <VStack spacing={2} align="stretch">
                                            <Input
                                              placeholder="Nama aset (cth: Laptop Dell, AC Daikin)"
                                              value={item.description}
                                              onChange={(e) => {
                                                const items = [
                                                  ...formData.items,
                                                ];
                                                items[index] = {
                                                  ...items[index],
                                                  description: e.target.value,
                                                };
                                                setFormData({
                                                  ...formData,
                                                  items,
                                                });
                                              }}
                                              size="sm"
                                              bg={inputBg}
                                              color={inputTextColor}
                                              borderColor={assetTypeBorder}
                                              _hover={{
                                                borderColor: inputHoverBorder,
                                              }}
                                              _focusVisible={fieldFocusStyles}
                                              _placeholder={{
                                                color: inputPlaceholderColor,
                                              }}
                                            />
                                            <Select
                                              placeholder="Pilih Kategori Aset"
                                              value={item.asset_category_id}
                                              onChange={(e) => {
                                                const items = [
                                                  ...formData.items,
                                                ];
                                                items[index] = {
                                                  ...items[index],
                                                  asset_category_id:
                                                    e.target.value,
                                                };
                                                setFormData({
                                                  ...formData,
                                                  items,
                                                });
                                              }}
                                              size="sm"
                                              bg={inputBg}
                                              color={inputTextColor}
                                              borderColor={assetTypeBorder}
                                              _hover={{
                                                borderColor: inputHoverBorder,
                                              }}
                                              _focusVisible={fieldFocusStyles}
                                              _placeholder={{
                                                color: inputPlaceholderColor,
                                              }}
                                              isDisabled={
                                                loadingAssetCategories
                                              }
                                            >
                                              {assetCategories.map((cat) => (
                                                <option
                                                  key={cat.id}
                                                  value={cat.id?.toString()}
                                                >
                                                  {cat.code} - {cat.name}
                                                </option>
                                              ))}
                                            </Select>
                                            <Badge
                                              colorScheme="orange"
                                              fontSize="2xs"
                                              px={2}
                                              py={0.5}
                                              borderRadius="md"
                                              w="fit-content"
                                            >
                                              Aset akan otomatis terdaftar
                                              dengan status PENDING
                                            </Badge>
                                          </VStack>
                                        ) : item.item_type === "PREPAID" ? (
                                          /* === PREPAID MODE: Input Deskripsi + Durasi Amortisasi === */
                                          <VStack spacing={2} align="stretch">
                                            <Input
                                              placeholder="Nama Kontrak/Sewa (cth: Sewa Ruko 1 Tahun)"
                                              value={item.description}
                                              onChange={(e) => {
                                                const items = [
                                                  ...formData.items,
                                                ];
                                                items[index] = {
                                                  ...items[index],
                                                  description: e.target.value,
                                                };
                                                setFormData({
                                                  ...formData,
                                                  items,
                                                });
                                              }}
                                              size="sm"
                                              bg={inputBg}
                                              color={inputTextColor}
                                              borderColor={prepaidTypeBorder}
                                              _hover={{
                                                borderColor: inputHoverBorder,
                                              }}
                                              _focusVisible={fieldFocusStyles}
                                              _placeholder={{
                                                color: inputPlaceholderColor,
                                              }}
                                            />
                                            <InputGroup size="sm">
                                              <NumberInput
                                                min={1}
                                                max={120}
                                                keepWithinRange={false}
                                                clampValueOnBlur={false}
                                                value={item.amortization_month}
                                                onChange={(val) => {
                                                  const items = [
                                                    ...formData.items,
                                                  ];
                                                  items[index] = {
                                                    ...items[index],
                                                    amortization_month: val,
                                                  };
                                                  setFormData({
                                                    ...formData,
                                                    items,
                                                  });
                                                }}
                                                w="full"
                                                borderColor={prepaidTypeBorder}
                                              >
                                                <NumberInputField
                                                  pl="1.5rem"
                                                  pr="3.5rem"
                                                  placeholder="12"
                                                  bg={inputBg}
                                                  color={inputTextColor}
                                                  borderColor={
                                                    prepaidTypeBorder
                                                  }
                                                  _hover={{
                                                    borderColor:
                                                      inputHoverBorder,
                                                  }}
                                                  _focusVisible={
                                                    fieldFocusStyles
                                                  }
                                                  _placeholder={{
                                                    color:
                                                      inputPlaceholderColor,
                                                  }}
                                                />
                                                <NumberInputStepper>
                                                  <NumberIncrementStepper />
                                                  <NumberDecrementStepper />
                                                </NumberInputStepper>
                                              </NumberInput>
                                              <InputLeftElement
                                                width="1.5rem"
                                                justifyContent="center"
                                              >
                                                <Text
                                                  fontSize="xs"
                                                  color={helperTextColor}
                                                >
                                                  ⏱️
                                                </Text>
                                              </InputLeftElement>
                                              <Box
                                                position="absolute"
                                                right="2rem"
                                                top="50%"
                                                transform="translateY(-50%)"
                                                zIndex={1}
                                                pointerEvents="none"
                                              >
                                                <Text
                                                  fontSize="xs"
                                                  color={helperTextColor}
                                                  fontWeight="medium"
                                                >
                                                  Bln
                                                </Text>
                                              </Box>
                                            </InputGroup>
                                            <Badge
                                              colorScheme="purple"
                                              fontSize="2xs"
                                              px={2}
                                              py={0.5}
                                              borderRadius="md"
                                              w="fit-content"
                                              alignSelf="flex-start"
                                            >
                                              Amortisasi berjalan otomatis
                                              setelah bayar
                                            </Badge>
                                          </VStack>
                                        ) : /* === INVENTORY MODE: Select Produk (seperti sebelumnya) === */
                                        loadingProducts ? (
                                          <Flex
                                            align="center"
                                            justify="center"
                                            h="32px"
                                          >
                                            <Spinner size="sm" />
                                          </Flex>
                                        ) : (
                                          <HStack spacing={2}>
                                            <Select
                                              placeholder={
                                                isServiceCompany
                                                  ? "Pilih Produk/Jasa"
                                                  : "Select product"
                                              }
                                              value={item.product_id}
                                              onChange={(e) => {
                                                const items = [
                                                  ...formData.items,
                                                ];
                                                items[index] = {
                                                  ...items[index],
                                                  product_id: e.target.value,
                                                };
                                                // Auto-fill unit price from product purchase_price if available
                                                const selectedProduct =
                                                  products.find(
                                                    (p) =>
                                                      p.id?.toString() ===
                                                      e.target.value,
                                                  );
                                                if (
                                                  selectedProduct &&
                                                  selectedProduct.purchase_price
                                                ) {
                                                  items[index] = {
                                                    ...items[index],
                                                    unit_price:
                                                      selectedProduct.purchase_price.toString(),
                                                  };
                                                }
                                                setFormData({
                                                  ...formData,
                                                  items,
                                                });
                                              }}
                                              size="sm"
                                              maxW="280px"
                                              bg={inputBg}
                                              color={inputTextColor}
                                              borderColor={inputBorderColor}
                                              _hover={{
                                                borderColor: inputHoverBorder,
                                              }}
                                              _focusVisible={fieldFocusStyles}
                                              _placeholder={{
                                                color: inputPlaceholderColor,
                                              }}
                                            >
                                              {products.map((p) => (
                                                <option
                                                  key={p.id}
                                                  value={p.id?.toString()}
                                                >
                                                  {isServiceCompany
                                                    ? `${p?.code} - ${p?.name}`
                                                    : `${p?.id} - ${p?.name || p?.code}`}
                                                </option>
                                              ))}
                                            </Select>
                                            <IconButton
                                              aria-label="Add new product"
                                              icon={<FiPlus />}
                                              size="sm"
                                              colorScheme="blue"
                                              variant="outline"
                                              onClick={onAddProductOpen}
                                              title="Add New Product"
                                              _hover={{ bg: blueButtonHoverBg }}
                                            />
                                          </HStack>
                                        )}
                                      </Td>

                                      {/* Kolom Qty */}
                                      <Td isNumeric verticalAlign="top" pt={3}>
                                        <FormControl>
                                          <NumberInput
                                            size="sm"
                                            min={1}
                                            value={item.quantity}
                                            onChange={(valueString) => {
                                              const items = [...formData.items];
                                              items[index] = {
                                                ...items[index],
                                                quantity: valueString,
                                              };
                                              setFormData({
                                                ...formData,
                                                items,
                                              });
                                            }}
                                            maxW="80px"
                                          >
                                            <NumberInputField
                                              textAlign="right"
                                              fontSize="sm"
                                              bg={inputBg}
                                              color={inputTextColor}
                                              borderColor={inputBorderColor}
                                              _hover={{
                                                borderColor: inputHoverBorder,
                                              }}
                                              _focusVisible={fieldFocusStyles}
                                              _placeholder={{
                                                color: inputPlaceholderColor,
                                              }}
                                            />
                                            <NumberInputStepper>
                                              <NumberIncrementStepper />
                                              <NumberDecrementStepper />
                                            </NumberInputStepper>
                                          </NumberInput>
                                        </FormControl>
                                      </Td>

                                      {/* Kolom Unit Price */}
                                      <Td isNumeric verticalAlign="top" pt={3}>
                                        <Box maxW="160px">
                                          <CurrencyInput
                                            value={
                                              parseFloat(item.unit_price) || 0
                                            }
                                            onChange={(value) => {
                                              const items = [...formData.items];
                                              items[index] = {
                                                ...items[index],
                                                unit_price: value.toString(),
                                              };
                                              setFormData({
                                                ...formData,
                                                items,
                                              });
                                            }}
                                            placeholder="Rp 10.000"
                                            size="sm"
                                            min={0}
                                            showLabel={false}
                                          />
                                        </Box>
                                      </Td>

                                      {/* Kolom Discount */}
                                      <Td isNumeric verticalAlign="top" pt={3}>
                                        <Box maxW="140px">
                                          <CurrencyInput
                                            value={
                                              parseFloat(item.discount) || 0
                                            }
                                            onChange={(value) => {
                                              const items = [...formData.items];
                                              items[index] = {
                                                ...items[index],
                                                discount: value.toString(),
                                              };
                                              setFormData({
                                                ...formData,
                                                items,
                                              });
                                            }}
                                            placeholder="Rp 0"
                                            size="sm"
                                            min={0}
                                            showLabel={false}
                                          />
                                        </Box>
                                      </Td>

                                      {/* Kolom Account - Hidden for ASSET type */}
                                      <Td
                                        minW="240px"
                                        verticalAlign="top"
                                        pt={3}
                                      >
                                        {item.item_type === "ASSET" &&
                                        canSelectAccountFromCOA ? (
                                          <Box maxW="240px">
                                            <SearchableSelect
                                              options={expenseAccounts
                                                .filter((acc) => {
                                                  const code = acc.code || "";
                                                  const name = (
                                                    acc.name || ""
                                                  ).toLowerCase();
                                                  return (
                                                    (code.startsWith("15") &&
                                                      code.length >= 4) ||
                                                    [
                                                      "1501",
                                                      "1502",
                                                      "1503",
                                                      "1504",
                                                      "1505",
                                                      "1509",
                                                    ].includes(code) ||
                                                    name.includes(
                                                      "fixed asset",
                                                    ) ||
                                                    name.includes(
                                                      "asset tetap",
                                                    ) ||
                                                    name.includes(
                                                      "peralatan",
                                                    ) ||
                                                    name.includes("mesin") ||
                                                    name.includes("printer") ||
                                                    name.includes("komputer") ||
                                                    name.includes(
                                                      "equipment",
                                                    ) ||
                                                    name.includes(
                                                      "furniture",
                                                    ) ||
                                                    name.includes(
                                                      "kendaraan",
                                                    ) ||
                                                    name.includes("bangunan") ||
                                                    name.includes("gedung")
                                                  );
                                                })
                                                .map((acc) => ({
                                                  id: acc.id!,
                                                  code: acc.code,
                                                  name: acc.name,
                                                  active: acc.is_active,
                                                }))}
                                              value={item.expense_account_id}
                                              onChange={(val) => {
                                                const items = [
                                                  ...formData.items,
                                                ];
                                                items[index] = {
                                                  ...items[index],
                                                  expense_account_id:
                                                    val.toString(),
                                                };
                                                setFormData({
                                                  ...formData,
                                                  items,
                                                });
                                              }}
                                              placeholder="Pilih Akun Aset Tetap..."
                                              isLoading={loadingExpenseAccounts}
                                              isDisabled={
                                                item.item_type !== "ASSET"
                                              }
                                              displayFormat={(option) =>
                                                `${option.code} - ${option.name}`
                                              }
                                            />
                                          </Box>
                                        ) : item.item_type === "PREPAID" &&
                                          canSelectAccountFromCOA ? (
                                          /* === PREPAID MODE: Show Current Asset / Prepaid accounts only === */
                                          <Box maxW="240px">
                                            <SearchableSelect
                                              options={expenseAccounts
                                                .filter((acc) => {
                                                  const code = acc.code || "";
                                                  const name = (
                                                    acc.name || ""
                                                  ).toLowerCase();
                                                  // Current Asset / Prepaid accounts (Beban Dibayar Dimuka)
                                                  // Typically codes 1104, 1105, 1106, or other current asset codes
                                                  const isPrepaidAsset =
                                                    code.startsWith("1104") ||
                                                    code.startsWith("1105") ||
                                                    code.startsWith("1106") ||
                                                    code.startsWith("1107") ||
                                                    code.startsWith("1108") ||
                                                    code.startsWith("1109") ||
                                                    name.includes("prepaid") ||
                                                    name.includes(
                                                      "dibayar dimuka",
                                                    ) ||
                                                    name.includes(
                                                      "uang muka",
                                                    ) ||
                                                    name.includes(
                                                      "bayar dimuka",
                                                    ) ||
                                                    name.includes(
                                                      "sewa dibayar",
                                                    ) ||
                                                    name.includes(
                                                      "asuransi dibayar",
                                                    );
                                                  // Also include generic current asset accounts (11xx) that are not inventory (1103) or cash (1101/1102)
                                                  const isOtherCurrentAsset =
                                                    code.startsWith("11") &&
                                                    code.length >= 4 &&
                                                    !code.startsWith("1101") &&
                                                    !code.startsWith("1102") &&
                                                    !code.startsWith("1103");
                                                  return (
                                                    isPrepaidAsset ||
                                                    isOtherCurrentAsset
                                                  );
                                                })
                                                .map((acc) => ({
                                                  id: acc.id!,
                                                  code: acc.code,
                                                  name: acc.name,
                                                  active: acc.is_active,
                                                }))}
                                              value={item.expense_account_id}
                                              onChange={(val) => {
                                                const items = [
                                                  ...formData.items,
                                                ];
                                                items[index] = {
                                                  ...items[index],
                                                  expense_account_id:
                                                    val.toString(),
                                                };
                                                setFormData({
                                                  ...formData,
                                                  items,
                                                });
                                              }}
                                              placeholder="Pilih Akun Aset Lancar (Prepaid)..."
                                              isLoading={loadingExpenseAccounts}
                                              displayFormat={(option) =>
                                                `${option.code} - ${option.name}`
                                              }
                                            />
                                          </Box>
                                        ) : canListExpenseAccounts &&
                                          canSelectAccountFromCOA ? (
                                          <Box maxW="240px">
                                            <SearchableSelect
                                              options={expenseAccounts.map(
                                                (acc) => ({
                                                  id: acc.id!,
                                                  code: acc.code,
                                                  name: acc.name,
                                                  active: acc.is_active,
                                                }),
                                              )}
                                              value={item.expense_account_id}
                                              onChange={(value) => {
                                                const items = [
                                                  ...formData.items,
                                                ];
                                                items[index] = {
                                                  ...items[index],
                                                  expense_account_id:
                                                    value.toString(),
                                                };
                                                setFormData({
                                                  ...formData,
                                                  items,
                                                });
                                              }}
                                              placeholder="Pilih akun..."
                                              isLoading={loadingExpenseAccounts}
                                              displayFormat={(option) =>
                                                `${option.code} - ${option.name}`
                                              }
                                            />
                                          </Box>
                                        ) : (
                                          <Box
                                            maxW="240px"
                                            p={3}
                                            borderWidth="1px"
                                            borderColor={inputBorderColor}
                                            borderRadius="md"
                                            bg={inputBg}
                                          >
                                            <Text
                                              fontSize="xs"
                                              fontWeight="semibold"
                                              color={inputTextColor}
                                            >
                                              COA Belum Dipilih
                                            </Text>
                                            <Text
                                              fontSize="xs"
                                              color={textSecondary}
                                              mt={1}
                                            >
                                              COA dikosongkan saat create
                                              purchase dan dapat dilengkapi oleh
                                              role accounting/finance manager.
                                            </Text>
                                          </Box>
                                        )}
                                      </Td>

                                      {/* Kolom Line Total */}
                                      <Td isNumeric verticalAlign="top" pt={3}>
                                        <Text
                                          fontSize="sm"
                                          fontWeight="medium"
                                          color={statColors.green}
                                        >
                                          {(() => {
                                            const qty = parseFloat(
                                              item.quantity || "0",
                                            );
                                            const price = parseFloat(
                                              item.unit_price || "0",
                                            );
                                            const discount = parseFloat(
                                              item.discount || "0",
                                            );
                                            const subtotal =
                                              (isNaN(qty) ? 0 : qty) *
                                              (isNaN(price) ? 0 : price);
                                            const lineTotal =
                                              subtotal -
                                              (isNaN(discount) ? 0 : discount);
                                            return formatCurrency(lineTotal);
                                          })()}
                                        </Text>
                                      </Td>

                                      {/* Kolom Action */}
                                      <Td verticalAlign="top" pt={3}>
                                        <IconButton
                                          aria-label="Remove item"
                                          size="sm"
                                          colorScheme="red"
                                          variant="ghost"
                                          icon={<FiTrash2 />}
                                          onClick={() => {
                                            const items = [...formData.items];
                                            items.splice(index, 1);
                                            setFormData({ ...formData, items });
                                          }}
                                          _hover={{ bg: redButtonHoverBg }}
                                        />
                                      </Td>
                                    </Tr>
                                  ))
                                )}
                              </Tbody>
                            </Table>
                          </Box>

                          {/* Summary Row */}
                          {formData.items.length > 0 && (
                            <Box
                              mt={4}
                              p={4}
                              bg={statBgColors.blue}
                              borderRadius="md"
                              borderLeft="4px solid"
                              borderLeftColor={statColors.blue}
                            >
                              <Flex justify="space-between" align="center">
                                <Text
                                  fontSize="sm"
                                  fontWeight="medium"
                                  color={textPrimary}
                                >
                                  Total Items: {formData.items.length}
                                </Text>
                                <Text
                                  fontSize="lg"
                                  fontWeight="bold"
                                  color={statColors.blue}
                                >
                                  Subtotal:{" "}
                                  {formatCurrency(
                                    formData.items.reduce((total, item) => {
                                      const qty = parseFloat(
                                        item.quantity || "0",
                                      );
                                      const price = parseFloat(
                                        item.unit_price || "0",
                                      );
                                      const discount = parseFloat(
                                        item.discount || "0",
                                      );
                                      const subtotal =
                                        (isNaN(qty) ? 0 : qty) *
                                        (isNaN(price) ? 0 : price);
                                      const lineTotal =
                                        subtotal -
                                        (isNaN(discount) ? 0 : discount);
                                      return total + lineTotal;
                                    }, 0),
                                  )}
                                </Text>
                              </Flex>
                            </Box>
                          )}

                          <FormControl>
                            <FormHelperText
                              mt={3}
                              fontSize="xs"
                              color={helperTextColor}
                            >
                              📌 Tambahkan minimal 1 item pembelian. Semua field
                              harus diisi dengan benar.
                            </FormHelperText>
                          </FormControl>
                        </CardBody>
                      </Card>

                      {/* Employee Info Banner */}
                      {!isFinanceOrAdmin && (
                        <Box
                          p={4}
                          bg={employeeInfoBg}
                          borderRadius="md"
                          border="1px solid"
                          borderColor={employeeInfoBorder}
                        >
                          <Text
                            fontSize="sm"
                            color={employeeInfoText}
                            fontWeight="medium"
                          >
                            ℹ️ Konfigurasi Pajak dan Metode Pembayaran akan
                            diatur oleh Finance saat proses approval. Default:
                            PPN 11%, Metode Pembayaran Credit.
                          </Text>
                        </Box>
                      )}

                      {/* Tax Configuration Section — Finance/Admin only */}
                      {isFinanceOrAdmin && (
                        <Card>
                          <CardHeader pb={3}>
                            <Text
                              fontSize="md"
                              fontWeight="semibold"
                              color={textPrimary}
                            >
                              💰 Tax Configuration
                            </Text>
                          </CardHeader>
                          <CardBody pt={0}>
                            <VStack spacing={4} align="stretch">
                              {/* Tax Additions (Penambahan) */}
                              <Box>
                                <Text
                                  fontSize="sm"
                                  fontWeight="medium"
                                  color={statColors.green}
                                  mb={3}
                                >
                                  ➕ Tax Additions (Penambahan)
                                </Text>
                                <SimpleGrid
                                  columns={{ base: 1, md: 3 }}
                                  spacing={4}
                                >
                                  <FormControl>
                                    <FormLabel fontSize="sm">
                                      PPN Rate (%)
                                    </FormLabel>
                                    <NumberInput
                                      value={formData.ppn_rate}
                                      onChange={(value) =>
                                        setFormData({
                                          ...formData,
                                          ppn_rate: value,
                                        })
                                      }
                                      size="sm"
                                      min={0}
                                      max={100}
                                      step={0.1}
                                    >
                                      <NumberInputField
                                        placeholder="11"
                                        bg={inputBg}
                                        color={inputTextColor}
                                        borderColor={inputBorderColor}
                                        _hover={{
                                          borderColor: inputHoverBorder,
                                        }}
                                        _focusVisible={fieldFocusStyles}
                                        _placeholder={{
                                          color: inputPlaceholderColor,
                                        }}
                                      />
                                      <NumberInputStepper>
                                        <NumberIncrementStepper />
                                        <NumberDecrementStepper />
                                      </NumberInputStepper>
                                    </NumberInput>
                                    <FormHelperText
                                      fontSize="xs"
                                      color={helperTextColor}
                                    >
                                      Pajak Pertambahan Nilai (default 11%)
                                    </FormHelperText>
                                  </FormControl>

                                  <FormControl>
                                    <FormLabel fontSize="sm">
                                      Other Tax Additions (%)
                                    </FormLabel>
                                    <NumberInput
                                      value={formData.other_tax_additions}
                                      onChange={(value) =>
                                        setFormData({
                                          ...formData,
                                          other_tax_additions: value,
                                        })
                                      }
                                      size="sm"
                                      min={0}
                                      max={100}
                                      step={0.1}
                                    >
                                      <NumberInputField
                                        placeholder="0"
                                        bg={inputBg}
                                        color={inputTextColor}
                                        borderColor={inputBorderColor}
                                        _hover={{
                                          borderColor: inputHoverBorder,
                                        }}
                                        _focusVisible={fieldFocusStyles}
                                        _placeholder={{
                                          color: inputPlaceholderColor,
                                        }}
                                      />
                                      <NumberInputStepper>
                                        <NumberIncrementStepper />
                                        <NumberDecrementStepper />
                                      </NumberInputStepper>
                                    </NumberInput>
                                    <FormHelperText
                                      fontSize="xs"
                                      color={helperTextColor}
                                    >
                                      Pajak tambahan lainnya (opsional)
                                    </FormHelperText>
                                  </FormControl>

                                  <FormControl>
                                    <FormLabel fontSize="sm">
                                      Freight / Biaya Pengiriman
                                    </FormLabel>
                                    <CurrencyInput
                                      value={
                                        parseFloat(formData.freight_cost) || 0
                                      }
                                      onChange={(value) =>
                                        setFormData({
                                          ...formData,
                                          freight_cost: value.toString(),
                                        })
                                      }
                                      placeholder="0"
                                      size="sm"
                                      min={0}
                                    />
                                    <FormHelperText
                                      fontSize="xs"
                                      color={helperTextColor}
                                    >
                                      Dikapitalisasi ke inventory valuation/HPP.
                                    </FormHelperText>
                                  </FormControl>
                                </SimpleGrid>
                              </Box>

                              <Divider />

                              {/* Tax Deductions (Pemotongan) */}
                              <Box>
                                <Text
                                  fontSize="sm"
                                  fontWeight="medium"
                                  color={statColors.red}
                                  mb={3}
                                >
                                  ➖ Tax Deductions (Pemotongan)
                                </Text>
                                <SimpleGrid columns={3} spacing={4}>
                                  <FormControl>
                                    <FormLabel fontSize="sm">
                                      PPh 21 Rate (%)
                                    </FormLabel>
                                    <NumberInput
                                      value={formData.pph21_rate}
                                      onChange={(value) =>
                                        setFormData({
                                          ...formData,
                                          pph21_rate: value,
                                        })
                                      }
                                      size="sm"
                                      min={0}
                                      max={100}
                                      step={0.1}
                                    >
                                      <NumberInputField
                                        placeholder="2"
                                        bg={inputBg}
                                        color={inputTextColor}
                                        borderColor={inputBorderColor}
                                        _hover={{
                                          borderColor: inputHoverBorder,
                                        }}
                                        _focusVisible={fieldFocusStyles}
                                        _placeholder={{
                                          color: inputPlaceholderColor,
                                        }}
                                      />
                                      <NumberInputStepper>
                                        <NumberIncrementStepper />
                                        <NumberDecrementStepper />
                                      </NumberInputStepper>
                                    </NumberInput>
                                    <FormHelperText
                                      fontSize="xs"
                                      color={helperTextColor}
                                    >
                                      PPh 21: 2% jasa konstruksi, 15%
                                      dividen/bunga
                                    </FormHelperText>
                                  </FormControl>

                                  <FormControl>
                                    <FormLabel fontSize="sm">
                                      PPh 23 Rate (%)
                                    </FormLabel>
                                    <NumberInput
                                      value={formData.pph23_rate}
                                      onChange={(value) =>
                                        setFormData({
                                          ...formData,
                                          pph23_rate: value,
                                        })
                                      }
                                      size="sm"
                                      min={0}
                                      max={100}
                                      step={0.1}
                                    >
                                      <NumberInputField
                                        placeholder="2"
                                        bg={inputBg}
                                        color={inputTextColor}
                                        borderColor={inputBorderColor}
                                        _hover={{
                                          borderColor: inputHoverBorder,
                                        }}
                                        _focusVisible={fieldFocusStyles}
                                        _placeholder={{
                                          color: inputPlaceholderColor,
                                        }}
                                      />
                                      <NumberInputStepper>
                                        <NumberIncrementStepper />
                                        <NumberDecrementStepper />
                                      </NumberInputStepper>
                                    </NumberInput>
                                    <FormHelperText
                                      fontSize="xs"
                                      color={helperTextColor}
                                    >
                                      PPh 23: 2% jasa umum, 15%
                                      dividen/bunga/royalti
                                    </FormHelperText>
                                  </FormControl>

                                  <FormControl>
                                    <FormLabel fontSize="sm">
                                      Other Tax Deductions (%)
                                    </FormLabel>
                                    <NumberInput
                                      value={formData.other_tax_deductions}
                                      onChange={(value) =>
                                        setFormData({
                                          ...formData,
                                          other_tax_deductions: value,
                                        })
                                      }
                                      size="sm"
                                      min={0}
                                      max={100}
                                      step={0.1}
                                    >
                                      <NumberInputField
                                        placeholder="0"
                                        bg={inputBg}
                                        color={inputTextColor}
                                        borderColor={inputBorderColor}
                                        _hover={{
                                          borderColor: inputHoverBorder,
                                        }}
                                        _focusVisible={fieldFocusStyles}
                                        _placeholder={{
                                          color: inputPlaceholderColor,
                                        }}
                                      />
                                      <NumberInputStepper>
                                        <NumberIncrementStepper />
                                        <NumberDecrementStepper />
                                      </NumberInputStepper>
                                    </NumberInput>
                                    <FormHelperText
                                      fontSize="xs"
                                      color={helperTextColor}
                                    >
                                      Potongan pajak lainnya (opsional)
                                    </FormHelperText>
                                  </FormControl>
                                </SimpleGrid>
                              </Box>

                              {/* Tax Summary Calculation */}
                              {formData.items.length > 0 && (
                                <Box
                                  mt={4}
                                  p={4}
                                  bg={notesBoxBg}
                                  borderRadius="md"
                                  border="1px solid"
                                  borderColor={borderColor}
                                >
                                  <VStack spacing={2} align="stretch">
                                    <Text
                                      fontSize="sm"
                                      fontWeight="semibold"
                                      color={textPrimary}
                                    >
                                      Tax Summary:
                                    </Text>
                                    {(() => {
                                      const subtotal = formData.items.reduce(
                                        (total, item) => {
                                          const qty = parseFloat(
                                            item.quantity || "0",
                                          );
                                          const price = parseFloat(
                                            item.unit_price || "0",
                                          );
                                          const discount = parseFloat(
                                            item.discount || "0",
                                          );
                                          const itemSubtotal =
                                            (isNaN(qty) ? 0 : qty) *
                                            (isNaN(price) ? 0 : price);
                                          const lineTotal =
                                            itemSubtotal -
                                            (isNaN(discount) ? 0 : discount);
                                          return total + lineTotal;
                                        },
                                        0,
                                      );

                                      const discount =
                                        (parseFloat(formData.discount) || 0) /
                                        100;
                                      const discountedSubtotal =
                                        subtotal * (1 - discount);

                                      const ppnAmount =
                                        (discountedSubtotal *
                                          (parseFloat(formData.ppn_rate) ||
                                            0)) /
                                        100;
                                      const otherAdditions =
                                        (discountedSubtotal *
                                          (parseFloat(
                                            formData.other_tax_additions,
                                          ) || 0)) /
                                        100;
                                      const totalAdditions =
                                        ppnAmount + otherAdditions;
                                      const freightCost =
                                        parseFloat(formData.freight_cost) || 0;

                                      const pph21Amount =
                                        (discountedSubtotal *
                                          (parseFloat(formData.pph21_rate) ||
                                            0)) /
                                        100;
                                      const pph23Amount =
                                        (discountedSubtotal *
                                          (parseFloat(formData.pph23_rate) ||
                                            0)) /
                                        100;
                                      const otherDeductions =
                                        (discountedSubtotal *
                                          (parseFloat(
                                            formData.other_tax_deductions,
                                          ) || 0)) /
                                        100;
                                      const totalDeductions =
                                        pph21Amount +
                                        pph23Amount +
                                        otherDeductions;

                                      const grossBeforePPh =
                                        discountedSubtotal +
                                        freightCost +
                                        totalAdditions;
                                      const finalTotal =
                                        grossBeforePPh - totalDeductions;

                                      return (
                                        <SimpleGrid
                                          columns={2}
                                          spacing={4}
                                          fontSize="xs"
                                        >
                                          <VStack align="start" spacing={1}>
                                            <Text color={textSecondary}>
                                              Subtotal:{" "}
                                              {formatCurrency(subtotal)}
                                            </Text>
                                            <Text color={textSecondary}>
                                              Discount ({formData.discount}%): -
                                              {formatCurrency(
                                                subtotal * discount,
                                              )}
                                            </Text>
                                            <Text color={textSecondary}>
                                              After Discount:{" "}
                                              {formatCurrency(
                                                discountedSubtotal,
                                              )}
                                            </Text>
                                            <Text
                                              color={statColors.blue}
                                              fontWeight="semibold"
                                            >
                                              Harga Item sebelum PPh:{" "}
                                              {formatCurrency(
                                                discountedSubtotal,
                                              )}
                                            </Text>
                                          </VStack>

                                          <VStack align="start" spacing={1}>
                                            <Text color={statColors.green}>
                                              + PPN ({formData.ppn_rate}%):{" "}
                                              {formatCurrency(ppnAmount)}
                                            </Text>
                                            {parseFloat(
                                              formData.other_tax_additions,
                                            ) > 0 && (
                                              <Text color={statColors.green}>
                                                + Other Additions (
                                                {formData.other_tax_additions}
                                                %):{" "}
                                                {formatCurrency(otherAdditions)}
                                              </Text>
                                            )}
                                            {freightCost > 0 && (
                                              <Text color={statColors.green}>
                                                + Freight / Pengiriman:{" "}
                                                {formatCurrency(freightCost)}
                                              </Text>
                                            )}
                                            {parseFloat(formData.pph21_rate) >
                                              0 && (
                                              <Text color={statColors.red}>
                                                - PPh 21 ({formData.pph21_rate}
                                                %):{" "}
                                                {formatCurrency(pph21Amount)}
                                              </Text>
                                            )}
                                            {parseFloat(formData.pph23_rate) >
                                              0 && (
                                              <Text color={statColors.red}>
                                                - PPh 23 ({formData.pph23_rate}
                                                %):{" "}
                                                {formatCurrency(pph23Amount)}
                                              </Text>
                                            )}
                                            {parseFloat(
                                              formData.other_tax_deductions,
                                            ) > 0 && (
                                              <Text color={statColors.red}>
                                                - Other Deductions (
                                                {formData.other_tax_deductions}
                                                %):{" "}
                                                {formatCurrency(
                                                  otherDeductions,
                                                )}
                                              </Text>
                                            )}
                                            <Text color={textSecondary}>
                                              Tagihan sebelum PPh:{" "}
                                              {formatCurrency(grossBeforePPh)}
                                            </Text>
                                            <Text color={statColors.red}>
                                              Amount PPh dipotong: -
                                              {formatCurrency(totalDeductions)}
                                            </Text>
                                            <Text
                                              fontWeight="bold"
                                              color={statColors.blue}
                                              borderTop="1px solid"
                                              borderColor={borderColor}
                                              pt={1}
                                            >
                                              Net Payment Vendor:{" "}
                                              {formatCurrency(finalTotal)}
                                            </Text>
                                          </VStack>
                                        </SimpleGrid>
                                      );
                                    })()}
                                  </VStack>
                                </Box>
                              )}
                            </VStack>
                          </CardBody>
                        </Card>
                      )}

                      {/* Payment Method Section — Finance/Admin only */}
                      {isFinanceOrAdmin && (
                        <Card>
                          <CardHeader pb={3}>
                            <Text
                              fontSize="md"
                              fontWeight="semibold"
                              color={textPrimary}
                            >
                              💳 Payment Method
                            </Text>
                          </CardHeader>
                          <CardBody pt={0}>
                            <SimpleGrid columns={2} spacing={4}>
                              <FormControl isRequired>
                                <FormLabel fontSize="sm" fontWeight="medium">
                                  Payment Method
                                </FormLabel>
                                <Select
                                  value={formData.payment_method}
                                  onChange={(e) => {
                                    setFormData(
                                      normalizePurchasePaymentFields(
                                        formData,
                                        e.target.value,
                                      ),
                                    );
                                  }}
                                  size="sm"
                                >
                                  <option value="CREDIT">Credit</option>
                                  <option value="CASH">Cash</option>
                                  <option value="BANK_TRANSFER">
                                    Bank Transfer
                                  </option>
                                  <option value="CHECK">Check</option>
                                </Select>
                                <FormHelperText fontSize="xs">
                                  {formData.payment_method === "CREDIT" &&
                                    "Purchase on credit - payment due later"}
                                  {formData.payment_method === "CASH" &&
                                    "Direct cash payment"}
                                  {formData.payment_method ===
                                    "BANK_TRANSFER" &&
                                    "Electronic bank transfer"}
                                  {formData.payment_method === "CHECK" &&
                                    "Payment by check"}
                                </FormHelperText>
                              </FormControl>

                              {/* Cash/Bank Account dropdown for Bank Transfer, Cash, Check */}
                              {formData.payment_method !== "CREDIT" && (
                                <FormControl isRequired>
                                  <FormLabel fontSize="sm" fontWeight="medium">
                                    {formData.payment_method === "CASH"
                                      ? "Cash Account"
                                      : "Bank Account"}
                                  </FormLabel>
                                  <Select
                                    value={formData.bank_account_id}
                                    onChange={(e) =>
                                      setFormData({
                                        ...formData,
                                        bank_account_id: e.target.value,
                                      })
                                    }
                                    size="sm"
                                    disabled={loadingBankAccounts}
                                    placeholder={
                                      loadingBankAccounts
                                        ? "Loading accounts..."
                                        : formData.payment_method === "CASH"
                                          ? "Select cash account"
                                          : "Select bank account"
                                    }
                                  >
                                    {bankAccounts
                                      .filter((account) => {
                                        // Filter berdasarkan payment method
                                        if (
                                          formData.payment_method === "CASH"
                                        ) {
                                          return account.type === "CASH";
                                        } else {
                                          return account.type === "BANK";
                                        }
                                      })
                                      .map((account) => (
                                        <option
                                          key={account.id}
                                          value={account.id.toString()}
                                        >
                                          {account.name} ({account.code}) -{" "}
                                          {account.currency}{" "}
                                          {account.balance?.toLocaleString() ||
                                            "0"}
                                        </option>
                                      ))}
                                  </Select>
                                  <FormHelperText fontSize="xs">
                                    Required: Select{" "}
                                    {formData.payment_method === "CASH"
                                      ? "cash"
                                      : "bank"}{" "}
                                    account for payment processing
                                  </FormHelperText>
                                </FormControl>
                              )}

                              {/* Credit Account dropdown for Credit payment */}
                              {formData.payment_method === "CREDIT" && (
                                <FormControl
                                  isRequired={canSelectAccountFromCOA}
                                >
                                  <FormLabel fontSize="sm" fontWeight="medium">
                                    Liability Account
                                  </FormLabel>
                                  {canSelectAccountFromCOA ? (
                                    <Select
                                      value={formData.credit_account_id}
                                      onChange={(e) =>
                                        setFormData({
                                          ...formData,
                                          credit_account_id: e.target.value,
                                        })
                                      }
                                      size="sm"
                                      disabled={loadingCreditAccounts}
                                      placeholder={
                                        loadingCreditAccounts
                                          ? "Loading accounts..."
                                          : "Select liability account"
                                      }
                                    >
                                      {creditAccounts.map((account) => (
                                        <option
                                          key={account.id}
                                          value={account.id?.toString()}
                                        >
                                          {account.code} - {account.name}
                                        </option>
                                      ))}
                                    </Select>
                                  ) : (
                                    <Box
                                      p={3}
                                      borderWidth="1px"
                                      borderColor={inputBorderColor}
                                      borderRadius="md"
                                      bg={inputBg}
                                    >
                                      <Text fontSize="xs" fontWeight="semibold">
                                        COA Liability Belum Dipilih
                                      </Text>
                                      <Text fontSize="xs" color={textSecondary}>
                                        GL/COA akan dilengkapi Accounting setelah
                                        approval Finance Manager.
                                      </Text>
                                    </Box>
                                  )}
                                  <FormHelperText fontSize="xs">
                                    {canSelectAccountFromCOA
                                      ? "Required: Select liability account for tracking credit purchases"
                                      : "COA liability dikosongkan dan akan dilengkapi oleh Accounting sebelum final approval."}
                                  </FormHelperText>
                                </FormControl>
                              )}
                            </SimpleGrid>

                            {/* Payment Reference (for non-credit and non-cash payments) */}
                            {formData.payment_method !== "CREDIT" &&
                              formData.payment_method !== "CASH" && (
                                <FormControl mt={4}>
                                  <FormLabel fontSize="sm" fontWeight="medium">
                                    Payment Reference
                                  </FormLabel>
                                  <Input
                                    value={formData.payment_reference}
                                    onChange={(e) =>
                                      setFormData({
                                        ...formData,
                                        payment_reference: e.target.value,
                                      })
                                    }
                                    placeholder={
                                      formData.payment_method === "CHECK"
                                        ? "Check number"
                                        : formData.payment_method ===
                                            "BANK_TRANSFER"
                                          ? "Transaction ID or reference number"
                                          : "Payment reference number"
                                    }
                                    size="sm"
                                  />
                                  <FormHelperText fontSize="xs">
                                    Optional reference for tracking this payment
                                  </FormHelperText>
                                </FormControl>
                              )}
                          </CardBody>
                        </Card>
                      )}
                    </VStack>
                  </ModalBody>
                  <ModalFooter>
                    <HStack spacing={3}>
                      <Button variant="ghost" onClick={onCreateClose}>
                        Cancel
                      </Button>
                      <Button colorScheme="blue" onClick={handleSave}>
                        Create Purchase
                      </Button>
                    </HStack>
                  </ModalFooter>
                </ModalContent>
              </Modal>

              {/* Add Vendor Modal */}
              <Modal
                isOpen={isAddVendorOpen}
                onClose={onAddVendorClose}
                size="lg"
              >
                <ModalOverlay />
                <ModalContent bg={modalContentBg}>
                  <ModalHeader
                    bg={modalHeaderBg}
                    borderBottomWidth={1}
                    borderColor={borderColor}
                  >
                    <HStack>
                      <Box
                        w={1}
                        h={6}
                        bg={statColors.green}
                        borderRadius="full"
                      />
                      <Text
                        fontSize="lg"
                        fontWeight="bold"
                        color={statColors.green}
                      >
                        Add New Vendor
                      </Text>
                    </HStack>
                  </ModalHeader>
                  <ModalCloseButton />
                  <ModalBody>
                    <VStack spacing={4} align="stretch">
                      <SimpleGrid columns={2} spacing={4}>
                        <FormControl isRequired>
                          <FormLabel fontSize="sm">Vendor Name</FormLabel>
                          <Input
                            size="sm"
                            placeholder="Enter vendor name"
                            value={newVendorData.name}
                            maxLength={100}
                            onChange={(e) =>
                              setNewVendorData({
                                ...newVendorData,
                                name: e.target.value,
                              })
                            }
                          />
                        </FormControl>

                        <FormControl>
                          <FormLabel fontSize="sm">Vendor Code</FormLabel>
                          <Input
                            size="sm"
                            placeholder="Auto-generated if empty"
                            value={newVendorData.code}
                            maxLength={20}
                            onChange={(e) =>
                              setNewVendorData({
                                ...newVendorData,
                                code: e.target.value,
                              })
                            }
                          />
                        </FormControl>
                      </SimpleGrid>

                      <SimpleGrid columns={2} spacing={4}>
                        <FormControl isRequired>
                          <FormLabel fontSize="sm">Email</FormLabel>
                          <Input
                            size="sm"
                            type="email"
                            placeholder="vendor@company.com"
                            value={newVendorData.email}
                            maxLength={100}
                            onChange={(e) =>
                              setNewVendorData({
                                ...newVendorData,
                                email: e.target.value,
                              })
                            }
                          />
                        </FormControl>

                        <FormControl>
                          <FormLabel fontSize="sm">Phone</FormLabel>
                          <Input
                            size="sm"
                            placeholder="Enter phone number"
                            value={newVendorData.phone}
                            maxLength={20}
                            onChange={(e) =>
                              setNewVendorData({
                                ...newVendorData,
                                phone: e.target.value,
                              })
                            }
                          />
                        </FormControl>
                      </SimpleGrid>

                      <SimpleGrid columns={2} spacing={4}>
                        <FormControl>
                          <FormLabel fontSize="sm">Mobile</FormLabel>
                          <Input
                            size="sm"
                            placeholder="Enter mobile number"
                            value={newVendorData.mobile}
                            maxLength={20}
                            onChange={(e) =>
                              setNewVendorData({
                                ...newVendorData,
                                mobile: e.target.value,
                              })
                            }
                          />
                        </FormControl>

                        <FormControl>
                          <FormLabel fontSize="sm">PIC Name</FormLabel>
                          <Input
                            size="sm"
                            placeholder="Person in charge"
                            value={newVendorData.pic_name}
                            maxLength={100}
                            onChange={(e) =>
                              setNewVendorData({
                                ...newVendorData,
                                pic_name: e.target.value,
                              })
                            }
                          />
                        </FormControl>
                      </SimpleGrid>

                      <FormControl>
                        <FormLabel fontSize="sm">Vendor ID</FormLabel>
                        <Input
                          size="sm"
                          placeholder="External vendor ID (optional)"
                          value={newVendorData.external_id}
                          maxLength={50}
                          onChange={(e) =>
                            setNewVendorData({
                              ...newVendorData,
                              external_id: e.target.value,
                            })
                          }
                        />
                      </FormControl>

                      <FormControl>
                        <FormLabel fontSize="sm">Address</FormLabel>
                        <Textarea
                          size="sm"
                          placeholder="Enter vendor address"
                          rows={3}
                          value={newVendorData.address}
                          onChange={(e) =>
                            setNewVendorData({
                              ...newVendorData,
                              address: e.target.value,
                            })
                          }
                        />
                      </FormControl>

                      <FormControl>
                        <FormLabel fontSize="sm">Notes</FormLabel>
                        <Textarea
                          size="sm"
                          placeholder="Additional notes (optional)"
                          rows={2}
                          value={newVendorData.notes}
                          onChange={(e) =>
                            setNewVendorData({
                              ...newVendorData,
                              notes: e.target.value,
                            })
                          }
                        />
                      </FormControl>
                    </VStack>
                  </ModalBody>
                  <ModalFooter>
                    <HStack spacing={3}>
                      <Button
                        variant="ghost"
                        onClick={() => {
                          setNewVendorData({
                            name: "",
                            code: "",
                            email: "",
                            phone: "",
                            mobile: "",
                            address: "",
                            pic_name: "",
                            external_id: "",
                            notes: "",
                          });
                          onAddVendorClose();
                        }}
                        disabled={savingVendor}
                      >
                        Cancel
                      </Button>
                      <Button
                        colorScheme="green"
                        onClick={handleAddVendor}
                        isLoading={savingVendor}
                        loadingText="Creating..."
                      >
                        Create Vendor
                      </Button>
                    </HStack>
                  </ModalFooter>
                </ModalContent>
              </Modal>

              {/* Add Product Modal */}
              <Modal
                isOpen={isAddProductOpen}
                onClose={onAddProductClose}
                size="lg"
              >
                <ModalOverlay />
                <ModalContent bg={modalContentBg}>
                  <ModalHeader
                    bg={modalHeaderBg}
                    borderBottomWidth={1}
                    borderColor={borderColor}
                  >
                    <HStack>
                      <Box
                        w={1}
                        h={6}
                        bg={statColors.blue}
                        borderRadius="full"
                      />
                      <Text
                        fontSize="lg"
                        fontWeight="bold"
                        color={statColors.blue}
                      >
                        Add New Product
                      </Text>
                    </HStack>
                  </ModalHeader>
                  <ModalCloseButton />
                  <ModalBody>
                    <VStack spacing={4} align="stretch">
                      <FormControl isRequired>
                        <FormLabel fontSize="sm">Product Name</FormLabel>
                        <Input
                          size="sm"
                          placeholder="Enter product name"
                          value={newProductData.name}
                          onChange={(e) =>
                            setNewProductData({
                              ...newProductData,
                              name: e.target.value,
                            })
                          }
                        />
                      </FormControl>

                      <FormControl>
                        <FormLabel fontSize="sm">Product Code</FormLabel>
                        <Input
                          size="sm"
                          placeholder="Enter product code (optional)"
                          value={newProductData.code}
                          onChange={(e) =>
                            setNewProductData({
                              ...newProductData,
                              code: e.target.value,
                            })
                          }
                        />
                      </FormControl>

                      <FormControl>
                        <FormLabel fontSize="sm">Description</FormLabel>
                        <Textarea
                          size="sm"
                          placeholder="Enter product description"
                          value={newProductData.description}
                          onChange={(e) =>
                            setNewProductData({
                              ...newProductData,
                              description: e.target.value,
                            })
                          }
                        />
                      </FormControl>

                      <SimpleGrid columns={3} spacing={4}>
                        <FormControl isRequired>
                          <FormLabel fontSize="sm">Unit</FormLabel>
                          <Input
                            size="sm"
                            placeholder="e.g., pcs, kg, box"
                            value={newProductData.unit}
                            onChange={(e) =>
                              setNewProductData({
                                ...newProductData,
                                unit: e.target.value,
                              })
                            }
                          />
                        </FormControl>

                        <FormControl>
                          <FormLabel fontSize="sm">
                            Purchase Price (IDR)
                          </FormLabel>
                          <CurrencyInput
                            value={
                              parseFloat(newProductData.purchase_price) || 0
                            }
                            onChange={(value) =>
                              setNewProductData({
                                ...newProductData,
                                purchase_price: value.toString(),
                              })
                            }
                            placeholder="Rp 10.000"
                            size="sm"
                            min={0}
                            showLabel={false}
                          />
                        </FormControl>

                        <FormControl>
                          <FormLabel fontSize="sm">Sale Price (IDR)</FormLabel>
                          <CurrencyInput
                            value={parseFloat(newProductData.sale_price) || 0}
                            onChange={(value) =>
                              setNewProductData({
                                ...newProductData,
                                sale_price: value.toString(),
                              })
                            }
                            placeholder="Rp 15.000"
                            size="sm"
                            min={0}
                            showLabel={false}
                          />
                        </FormControl>
                      </SimpleGrid>
                    </VStack>
                  </ModalBody>
                  <ModalFooter>
                    <HStack spacing={3} w="100%">
                      <Button
                        variant="ghost"
                        onClick={() => {
                          setNewProductData({
                            name: "",
                            code: "",
                            description: "",
                            unit: "",
                            purchase_price: "0",
                            sale_price: "0",
                          });
                          onAddProductClose();
                        }}
                        flex={1}
                      >
                        Cancel
                      </Button>
                      <Button
                        colorScheme="blue"
                        onClick={handleAddProduct}
                        isLoading={savingProduct}
                        loadingText="Creating..."
                        flex={1}
                      >
                        Create Product
                      </Button>
                    </HStack>
                  </ModalFooter>
                </ModalContent>
              </Modal>

              {/* Create Receipt Modal */}
              <Modal isOpen={isReceiptOpen} onClose={onReceiptClose} size="xl">
                <ModalOverlay />
                <ModalContent bg={modalContentBg}>
                  <ModalHeader
                    bg={modalHeaderBg}
                    borderBottomWidth={1}
                    borderColor={borderColor}
                  >
                    <HStack>
                      <Box
                        w={1}
                        h={6}
                        bg={statColors.green}
                        borderRadius="full"
                      />
                      <Text
                        fontSize="lg"
                        fontWeight="bold"
                        color={statColors.green}
                      >
                        Create Goods Receipt - {selectedPurchase?.code}
                      </Text>
                    </HStack>
                  </ModalHeader>
                  <ModalCloseButton />
                  <ModalBody>
                    {selectedPurchase && (
                      <VStack spacing={6} align="stretch">
                        {/* Purchase Info */}
                        <Card variant="outline">
                          <CardBody p={4}>
                            <SimpleGrid columns={3} spacing={4}>
                              <FormControl>
                                <FormLabel fontSize="sm">
                                  Purchase Code
                                </FormLabel>
                                <Text fontWeight="medium">
                                  {selectedPurchase.code}
                                </Text>
                              </FormControl>
                              <FormControl>
                                <FormLabel fontSize="sm">Vendor</FormLabel>
                                <Text fontWeight="medium">
                                  {selectedPurchase.vendor?.name || "N/A"}
                                </Text>
                              </FormControl>
                              <FormControl>
                                <FormLabel fontSize="sm">
                                  Total Amount
                                </FormLabel>
                                <Text fontWeight="medium" color="green.500">
                                  {formatCurrency(
                                    selectedPurchase.total_amount,
                                  )}
                                </Text>
                              </FormControl>
                            </SimpleGrid>
                          </CardBody>
                        </Card>

                        {/* Receipt Details */}
                        <SimpleGrid columns={2} spacing={4}>
                          <FormControl isRequired>
                            <FormLabel fontSize="sm">Received Date</FormLabel>
                            <Input
                              type="date"
                              size="sm"
                              value={receiptFormData.received_date}
                              onChange={(e) =>
                                setReceiptFormData({
                                  ...receiptFormData,
                                  received_date: e.target.value,
                                })
                              }
                            />
                          </FormControl>
                          <FormControl>
                            <FormLabel fontSize="sm">Receipt Notes</FormLabel>
                            <Input
                              size="sm"
                              placeholder="General notes for this receipt"
                              value={receiptFormData.notes}
                              onChange={(e) =>
                                setReceiptFormData({
                                  ...receiptFormData,
                                  notes: e.target.value,
                                })
                              }
                            />
                          </FormControl>
                        </SimpleGrid>

                        {/* Receipt Items */}
                        <FormControl>
                          <FormLabel fontSize="sm">Receipt Items</FormLabel>
                          <TableContainer>
                            <Table size="sm" bg={tableBg}>
                              <Thead bg={tableHeaderBg}>
                                <Tr>
                                  <Th fontSize="xs">Product</Th>
                                  <Th fontSize="xs" isNumeric>
                                    Ordered Qty
                                  </Th>
                                  <Th fontSize="xs" isNumeric>
                                    Received Qty
                                  </Th>
                                  <Th fontSize="xs">Condition</Th>
                                  <Th fontSize="xs">Serial Number</Th>
                                  <Th fontSize="xs">Notes</Th>
                                </Tr>
                              </Thead>
                              <Tbody>
                                {receiptFormData.receipt_items.map(
                                  (receiptItem, index) => {
                                    const purchaseItem =
                                      selectedPurchase.purchase_items?.find(
                                        (item) =>
                                          item.id ===
                                          receiptItem.purchase_item_id,
                                      );
                                    return (
                                      <Tr key={receiptItem.purchase_item_id}>
                                        <Td fontSize="sm">
                                          {purchaseItem
                                            ? getPurchaseItemDisplayName(
                                                purchaseItem,
                                              )
                                            : "Item tidak ditemukan"}
                                        </Td>
                                        <Td fontSize="sm" isNumeric>
                                          {purchaseItem?.quantity || 0}
                                        </Td>
                                        <Td>
                                          <FormControl>
                                            <NumberInput
                                              size="sm"
                                              min={1}
                                              max={
                                                remainingQtyMap[
                                                  purchaseItem?.id as number
                                                ] ??
                                                (purchaseItem?.quantity || 0)
                                              }
                                              value={
                                                receiptItem.quantity_received
                                              }
                                              onChange={(_, value) => {
                                                const newItems = [
                                                  ...receiptFormData.receipt_items,
                                                ];
                                                const maxQty =
                                                  remainingQtyMap[
                                                    purchaseItem?.id as number
                                                  ] ??
                                                  (purchaseItem?.quantity || 1);
                                                const nextQty = Number.isFinite(
                                                  value,
                                                )
                                                  ? Math.min(
                                                      Math.max(value, 1),
                                                      Math.max(maxQty, 1),
                                                    )
                                                  : 1;
                                                newItems[
                                                  index
                                                ].quantity_received = nextQty;
                                                setReceiptFormData({
                                                  ...receiptFormData,
                                                  receipt_items: newItems,
                                                });
                                              }}
                                            >
                                              <NumberInputField />
                                              <NumberInputStepper>
                                                <NumberIncrementStepper />
                                                <NumberDecrementStepper />
                                              </NumberInputStepper>
                                            </NumberInput>
                                          </FormControl>
                                        </Td>
                                        <Td>
                                          <Select
                                            size="sm"
                                            value={receiptItem.condition}
                                            onChange={(e) => {
                                              const newItems = [
                                                ...receiptFormData.receipt_items,
                                              ];
                                              newItems[index].condition =
                                                e.target.value;
                                              setReceiptFormData({
                                                ...receiptFormData,
                                                receipt_items: newItems,
                                              });
                                            }}
                                          >
                                            <option value="GOOD">Good</option>
                                            <option value="DAMAGED">
                                              Damaged
                                            </option>
                                            <option value="DEFECTIVE">
                                              Defective
                                            </option>
                                          </Select>
                                        </Td>
                                        {/* Serial Number */}
                                        <Td>
                                          <Input
                                            size="sm"
                                            placeholder="Serial/Chassis number"
                                            value={
                                              receiptItem.serial_number || ""
                                            }
                                            onChange={(e) => {
                                              const newItems = [
                                                ...receiptFormData.receipt_items,
                                              ];
                                              newItems[index].serial_number =
                                                e.target.value;
                                              setReceiptFormData({
                                                ...receiptFormData,
                                                receipt_items: newItems,
                                              });
                                            }}
                                          />
                                        </Td>

                                        {/* Notes */}
                                        <Td>
                                          <Input
                                            size="sm"
                                            placeholder="Item notes"
                                            value={receiptItem.notes}
                                            onChange={(e) => {
                                              const newItems = [
                                                ...receiptFormData.receipt_items,
                                              ];
                                              newItems[index].notes =
                                                e.target.value;
                                              setReceiptFormData({
                                                ...receiptFormData,
                                                receipt_items: newItems,
                                              });
                                            }}
                                          />
                                        </Td>
                                      </Tr>
                                    );
                                  },
                                )}
                              </Tbody>
                            </Table>
                          </TableContainer>
                        </FormControl>

                        <Alert status="info" variant="left-accent">
                          <AlertIcon />
                          <VStack align="start" spacing={1}>
                            <AlertTitle fontSize="sm">
                              Receipt Information
                            </AlertTitle>
                            <AlertDescription fontSize="xs">
                              • Creating this receipt will mark the purchase as
                              COMPLETED if all items are fully received.
                              <br />
                              • Stock quantities were already updated when the
                              purchase was approved.
                              <br />• Aset tetap (fixed assets) otomatis
                              tercatat saat purchase di-approve. Cek halaman
                              Assets untuk mengatur masa manfaat dan
                              mengaktifkan depresiasi.
                            </AlertDescription>
                          </VStack>
                        </Alert>
                      </VStack>
                    )}
                  </ModalBody>
                  <ModalFooter>
                    <HStack spacing={3} w="100%">
                      <Button
                        variant="ghost"
                        onClick={() => {
                          setReceiptFormData({
                            received_date: new Date()
                              .toISOString()
                              .split("T")[0],
                            notes: "",
                            receipt_items: [],
                          });
                          onReceiptClose();
                        }}
                        disabled={savingReceipt}
                        flex={1}
                      >
                        Cancel
                      </Button>
                      <Button
                        colorScheme="green"
                        onClick={handleSaveReceipt}
                        isLoading={savingReceipt}
                        loadingText="Creating Receipt..."
                        flex={1}
                        leftIcon={<FiPackage />}
                      >
                        Create Receipt
                      </Button>
                    </HStack>
                  </ModalFooter>
                </ModalContent>
              </Modal>

              {/* Receipts Modal */}
              <Modal
                isOpen={isReceiptsOpen}
                onClose={onReceiptsClose}
                size="4xl"
              >
                <ModalOverlay />
                <ModalContent>
                  <ModalHeader>
                    <HStack>
                      <Icon as={FiPackage} />
                      <VStack align="start" spacing={0}>
                        <Text fontWeight="bold">
                          Receipts for {selectedPurchaseForReceipts?.code}
                        </Text>
                        <Text fontSize="sm" color="gray.600">
                          Vendor: {selectedPurchaseForReceipts?.vendor?.name}
                        </Text>
                      </VStack>
                    </HStack>
                  </ModalHeader>
                  <ModalCloseButton />
                  <ModalBody>
                    {loadingReceipts ? (
                      <VStack spacing={4}>
                        <Spinner size="lg" />
                        <Text>Loading receipts...</Text>
                      </VStack>
                    ) : (
                      <Box>
                        {receipts.length === 0 ? (
                          <Alert status="info">
                            <AlertIcon />
                            <Text>
                              No completed receipts found for this purchase.
                            </Text>
                          </Alert>
                        ) : (
                          <TableContainer>
                            <Table size="sm">
                              <Thead>
                                <Tr>
                                  <Th>Receipt #</Th>
                                  <Th>Date</Th>
                                  <Th>Received By</Th>
                                  <Th>Status</Th>
                                  <Th>Items</Th>
                                  <Th>Actions</Th>
                                </Tr>
                              </Thead>
                              <Tbody>
                                {receipts.map((receipt) => (
                                  <Tr key={receipt.id}>
                                    <Td fontWeight="medium">
                                      {receipt.receipt_number}
                                    </Td>
                                    <Td>{formatDate(receipt.received_date)}</Td>
                                    <Td>{getReceiverName(receipt.receiver)}</Td>
                                    <Td>
                                      <Badge
                                        colorScheme={getStatusColor(
                                          receipt.status,
                                        )}
                                      >
                                        {receipt.status}
                                      </Badge>
                                    </Td>
                                    <Td>
                                      {(receipt.receipt_items || []).reduce(
                                        (sum: number, it: any) =>
                                          sum + (it.quantity_received || 0),
                                        0,
                                      )}
                                    </Td>
                                    <Td>
                                      <IconButton
                                        aria-label="Download Receipt PDF"
                                        icon={<FiDownload />}
                                        size="sm"
                                        colorScheme="blue"
                                        variant="ghost"
                                        onClick={() =>
                                          handleDownloadReceiptPDF(
                                            receipt.id,
                                            receipt.receipt_number,
                                          )
                                        }
                                        title="Download this receipt as PDF"
                                      />
                                    </Td>
                                  </Tr>
                                ))}
                              </Tbody>
                            </Table>
                          </TableContainer>
                        )}
                      </Box>
                    )}
                  </ModalBody>
                  <ModalFooter>
                    <HStack spacing={3} w="100%">
                      {receipts.length > 0 && (
                        <Button
                          leftIcon={<FiDownload />}
                          colorScheme="green"
                          onClick={() =>
                            selectedPurchaseForReceipts &&
                            handleDownloadAllReceiptsPDF(
                              selectedPurchaseForReceipts,
                            )
                          }
                          flex={1}
                        >
                          Download Receipts
                        </Button>
                      )}
                      <Button
                        variant="ghost"
                        onClick={onReceiptsClose}
                        flex={receipts.length > 0 ? 0 : 1}
                      >
                        Close
                      </Button>
                    </HStack>
                  </ModalFooter>
                </ModalContent>
              </Modal>

              {/* Journal Entries Modal */}
              <PurchaseJournalEntriesModal
                isOpen={isJournalOpen}
                onClose={onJournalClose}
                purchase={selectedPurchaseForJournal}
              />

              <Modal
                isOpen={isPPhPaymentOpen}
                onClose={() => {
                  if (!savingPPhPayment) {
                    onPPhPaymentClose();
                  }
                }}
                size="2xl"
                scrollBehavior="inside"
              >
                <ModalOverlay />
                <ModalContent>
                  <ModalHeader>
                    Bayar PPh Purchase {selectedPurchaseForPPh?.code || ""}
                  </ModalHeader>
                  <ModalCloseButton isDisabled={savingPPhPayment} />
                  <ModalBody>
                    {selectedPurchaseForPPh && (
                      <VStack spacing={5} align="stretch">
                        {(() => {
                          const pphStatus = getPurchasePPhStatus(
                            selectedPurchaseForPPh,
                          );
                          return (
                            <SimpleGrid
                              columns={{ base: 1, md: 3 }}
                              spacing={4}
                            >
                              <Card variant="outline">
                                <CardBody>
                                  <Stat>
                                    <StatLabel>Total PPh</StatLabel>
                                    <StatNumber color="orange.500">
                                      {formatCurrency(pphStatus.totalAmount)}
                                    </StatNumber>
                                  </Stat>
                                </CardBody>
                              </Card>
                              <Card variant="outline">
                                <CardBody>
                                  <Stat>
                                    <StatLabel>Sudah Dibayar</StatLabel>
                                    <StatNumber color="green.500">
                                      {formatCurrency(pphStatus.paidAmount)}
                                    </StatNumber>
                                  </Stat>
                                </CardBody>
                              </Card>
                              <Card variant="outline">
                                <CardBody>
                                  <Stat>
                                    <StatLabel>Sisa PPh</StatLabel>
                                    <StatNumber color="red.500">
                                      {formatCurrency(
                                        pphStatus.outstandingAmount,
                                      )}
                                    </StatNumber>
                                  </Stat>
                                </CardBody>
                              </Card>
                            </SimpleGrid>
                          );
                        })()}

                        <Alert status="info" borderRadius="md">
                          <AlertIcon />
                          <Box>
                            <AlertTitle fontSize="sm">
                              Setoran PPh memakai kas/bank
                            </AlertTitle>
                            <AlertDescription fontSize="sm">
                              Jurnal yang dibuat: Dr Hutang PPh, Cr Kas/Bank.
                              Status PPh di tabel purchase akan berubah setelah
                              pembayaran tersimpan.
                            </AlertDescription>
                          </Box>
                        </Alert>

                        <SimpleGrid columns={{ base: 1, md: 2 }} spacing={4}>
                          <FormControl isRequired>
                            <FormLabel>Tanggal Bayar PPh</FormLabel>
                            <Input
                              type="date"
                              value={pphPaymentForm.payment_date}
                              onChange={(event) =>
                                setPPhPaymentForm((prev) => ({
                                  ...prev,
                                  payment_date: event.target.value,
                                }))
                              }
                            />
                          </FormControl>

                          <FormControl isRequired>
                            <FormLabel>Nominal PPh</FormLabel>
                            <NumberInput
                              min={0}
                              precision={0}
                              value={pphPaymentForm.amount}
                              onChange={(valueString) =>
                                setPPhPaymentForm((prev) => ({
                                  ...prev,
                                  amount: valueString,
                                }))
                              }
                            >
                              <NumberInputField />
                              <NumberInputStepper>
                                <NumberIncrementStepper />
                                <NumberDecrementStepper />
                              </NumberInputStepper>
                            </NumberInput>
                          </FormControl>

                          <FormControl isRequired>
                            <FormLabel>Akun Kas/Bank</FormLabel>
                            <Select
                              placeholder={
                                loadingBankAccounts
                                  ? "Memuat kas/bank..."
                                  : "Pilih kas/bank IDR"
                              }
                              value={pphPaymentForm.cash_bank_id}
                              onChange={(event) =>
                                setPPhPaymentForm((prev) => ({
                                  ...prev,
                                  cash_bank_id: event.target.value,
                                }))
                              }
                              disabled={loadingBankAccounts}
                            >
                              {getPPhPaymentBankAccounts().map((account) => (
                                <option
                                  key={account.id}
                                  value={account.id.toString()}
                                >
                                  {account.code} - {account.name} (
                                  {formatBankAccountSummary(account)})
                                </option>
                              ))}
                            </Select>
                          </FormControl>

                          <FormControl>
                            <FormLabel>Referensi</FormLabel>
                            <Input
                              placeholder="NTPN / bukti bayar"
                              value={pphPaymentForm.reference}
                              onChange={(event) =>
                                setPPhPaymentForm((prev) => ({
                                  ...prev,
                                  reference: event.target.value,
                                }))
                              }
                            />
                          </FormControl>
                        </SimpleGrid>

                        <FormControl>
                          <FormLabel>Catatan</FormLabel>
                          <Textarea
                            value={pphPaymentForm.notes}
                            onChange={(event) =>
                              setPPhPaymentForm((prev) => ({
                                ...prev,
                                notes: event.target.value,
                              }))
                            }
                          />
                        </FormControl>
                      </VStack>
                    )}
                  </ModalBody>
                  <ModalFooter>
                    <Button
                      variant="ghost"
                      mr={3}
                      onClick={onPPhPaymentClose}
                      isDisabled={savingPPhPayment}
                    >
                      Batal
                    </Button>
                    <Button
                      colorScheme="orange"
                      leftIcon={<FiDollarSign />}
                      onClick={handleSubmitPurchasePPhPayment}
                      isLoading={savingPPhPayment}
                    >
                      Bayar PPh
                    </Button>
                  </ModalFooter>
                </ModalContent>
              </Modal>

              {/* Payment Modal */}
              <PurchasePaymentForm
                isOpen={isPaymentOpen}
                onClose={onPaymentClose}
                purchase={selectedPurchaseForPayment}
                onSuccess={handlePaymentSuccess}
                cashBanks={cashBanks}
              />

              {/* Accounting COA Completion Modal */}
              <Modal
                isOpen={isAccountingCOAOpen}
                onClose={onAccountingCOAClose}
                size="6xl"
                scrollBehavior="inside"
              >
                <ModalOverlay bg="blackAlpha.600" backdropFilter="blur(4px)" />
                <ModalContent bg={modalBg} borderRadius="xl" maxH="90vh">
                  <ModalHeader
                    borderBottom="1px solid"
                    borderColor={borderColor}
                    pb={4}
                  >
                    <VStack align="start" spacing={1}>
                      <Heading size="md" color={headingColor}>
                        {isPendingFinalApprovalPurchase(accountingCOAPurchase)
                          ? "Review & Finalisasi GL/COA"
                          : "Lengkapi GL/COA Accounting"}
                      </Heading>
                      <Text fontSize="sm" color={textSecondary}>
                        {accountingCOAPurchase?.code} -{" "}
                        {accountingCOAPurchase?.vendor?.name || "N/A"}
                      </Text>
                    </VStack>
                  </ModalHeader>
                  <ModalCloseButton />
                  <ModalBody py={6}>
                    <VStack spacing={5} align="stretch">
                      <Alert status="info" borderRadius="lg">
                        <AlertIcon />
                        <Box>
                          <AlertTitle fontSize="sm">
                            {isPendingFinalApprovalPurchase(
                              accountingCOAPurchase,
                            )
                              ? "Review terakhir Finance Manager"
                              : "Purchase sudah disetujui Finance Manager"}
                          </AlertTitle>
                          <AlertDescription fontSize="sm">
                            {isPendingFinalApprovalPurchase(
                              accountingCOAPurchase,
                            )
                              ? "Koreksi akun hutang dan GL/COA debit jika input Finance Staff atau Accounting belum tepat. Saat tombol final ditekan, perubahan disimpan lalu jurnal diposting."
                              : "Posting jurnal, stok, payable/prepaid, dan fulfillment tetap ditahan setelah Accounting menyimpan GL/COA. Finance Manager perlu melakukan final approval sebelum jurnal diposting."}
                          </AlertDescription>
                        </Box>
                      </Alert>

                      <SimpleGrid columns={{ base: 1, md: 4 }} spacing={4}>
                        <Box>
                          <Text fontSize="xs" color={textSecondary}>
                            Tanggal
                          </Text>
                          <Text fontSize="sm" fontWeight="medium">
                            {accountingCOAPurchase?.date
                              ? formatDate(accountingCOAPurchase.date)
                              : "-"}
                          </Text>
                        </Box>
                        <Box>
                          <Text fontSize="xs" color={textSecondary}>
                            Metode Pembayaran
                          </Text>
                          <Text fontSize="sm" fontWeight="medium">
                            {getAccountingCOAPaymentMethod() || "-"}
                          </Text>
                        </Box>
                        <Box>
                          <Text fontSize="xs" color={textSecondary}>
                            Total
                          </Text>
                          <Text
                            fontSize="sm"
                            fontWeight="bold"
                            color="blue.600"
                          >
                            {formatCurrency(
                              accountingCOAPurchase?.total_amount || 0,
                            )}
                          </Text>
                        </Box>
                        <Box>
                          <Text fontSize="xs" color={textSecondary}>
                            Status
                          </Text>
                          <Badge
                            colorScheme={
                              isPendingFinalApprovalPurchase(
                                accountingCOAPurchase,
                              )
                                ? "purple"
                                : "orange"
                            }
                          >
                            {isPendingFinalApprovalPurchase(
                              accountingCOAPurchase,
                            )
                              ? "PENDING FINAL APPROVAL"
                              : "PENDING ACCOUNTING"}
                          </Badge>
                        </Box>
                      </SimpleGrid>

                      {isPendingFinalApprovalPurchase(
                        accountingCOAPurchase,
                      ) && (
                        <Box
                          borderWidth="1px"
                          borderColor={borderColor}
                          borderRadius="lg"
                          p={4}
                        >
                          <Text
                            fontWeight="bold"
                            mb={3}
                            color={headingColor}
                            fontSize="sm"
                          >
                            Review Pajak & Pembayaran
                          </Text>
                          <SimpleGrid columns={{ base: 1, md: 3 }} spacing={4}>
                            <FormControl>
                              <FormLabel fontSize="sm">PPN Rate (%)</FormLabel>
                              <Input
                                type="number"
                                step="0.01"
                                value={accountingCOAForm.ppn_rate}
                                onChange={(e) =>
                                  setAccountingCOAForm((prev) => ({
                                    ...prev,
                                    ppn_rate: e.target.value,
                                  }))
                                }
                              />
                            </FormControl>
                            <FormControl>
                              <FormLabel fontSize="sm">
                                Pajak Tambahan Lainnya
                              </FormLabel>
                              <Input
                                type="number"
                                step="0.01"
                                value={accountingCOAForm.other_tax_additions}
                                onChange={(e) =>
                                  setAccountingCOAForm((prev) => ({
                                    ...prev,
                                    other_tax_additions: e.target.value,
                                  }))
                                }
                              />
                            </FormControl>
                            <FormControl>
                              <FormLabel fontSize="sm">
                                Freight / Biaya Pengiriman
                              </FormLabel>
                              <CurrencyInput
                                value={
                                  parseFloat(accountingCOAForm.freight_cost) ||
                                  0
                                }
                                onChange={(value) =>
                                  setAccountingCOAForm((prev) => ({
                                    ...prev,
                                    freight_cost: value.toString(),
                                  }))
                                }
                                placeholder="0"
                                size="sm"
                                min={0}
                              />
                            </FormControl>
                            <FormControl>
                              <FormLabel fontSize="sm">
                                PPh 21 Rate (%)
                              </FormLabel>
                              <Input
                                type="number"
                                step="0.01"
                                value={accountingCOAForm.pph21_rate}
                                onChange={(e) =>
                                  setAccountingCOAForm((prev) => ({
                                    ...prev,
                                    pph21_rate: e.target.value,
                                  }))
                                }
                              />
                            </FormControl>
                            <FormControl>
                              <FormLabel fontSize="sm">
                                PPh 23 Rate (%)
                              </FormLabel>
                              <Input
                                type="number"
                                step="0.01"
                                value={accountingCOAForm.pph23_rate}
                                onChange={(e) =>
                                  setAccountingCOAForm((prev) => ({
                                    ...prev,
                                    pph23_rate: e.target.value,
                                  }))
                                }
                              />
                            </FormControl>
                            <FormControl>
                              <FormLabel fontSize="sm">
                                Potongan Pajak Lainnya
                              </FormLabel>
                              <Input
                                type="number"
                                step="0.01"
                                value={accountingCOAForm.other_tax_deductions}
                                onChange={(e) =>
                                  setAccountingCOAForm((prev) => ({
                                    ...prev,
                                    other_tax_deductions: e.target.value,
                                  }))
                                }
                              />
                            </FormControl>
                            <FormControl>
                              <FormLabel fontSize="sm">
                                Metode Pembayaran
                              </FormLabel>
                              <Select
                                value={accountingCOAForm.payment_method}
                                onChange={(e) =>
                                  setAccountingCOAForm((prev) => ({
                                    ...prev,
                                    payment_method: e.target.value,
                                    bank_account_id:
                                      e.target.value === "CREDIT"
                                        ? ""
                                        : prev.bank_account_id,
                                  }))
                                }
                              >
                                <option value="CREDIT">
                                  Kredit (Bayar Nanti)
                                </option>
                                <option value="CASH">Tunai</option>
                                <option value="BANK_TRANSFER">
                                  Transfer Bank
                                </option>
                                <option value="CHECK">Cek/Giro</option>
                              </Select>
                            </FormControl>
                            {accountingCOAForm.payment_method !== "CREDIT" && (
                              <FormControl
                                isRequired
                                isInvalid={isAccountingFinalReviewBalanceInsufficient()}
                              >
                                <FormLabel fontSize="sm">
                                  Akun Bank / Kas
                                </FormLabel>
                                <Select
                                  placeholder="Pilih Akun Bank/Kas"
                                  value={accountingCOAForm.bank_account_id}
                                  onChange={(e) =>
                                    setAccountingCOAForm((prev) => ({
                                      ...prev,
                                      bank_account_id: e.target.value,
                                    }))
                                  }
                                >
                                  {getAccountingFinalReviewEligibleBankAccounts().map(
                                    (account) => {
                                      const bookBalance =
                                        getAccountBookBalance(account);
                                      const isSufficient =
                                        bookBalance >=
                                        calculateAccountingFinalReviewTotal();
                                      return (
                                        <option
                                          key={account.id}
                                          value={account.id.toString()}
                                        >
                                          {account.name} ({account.code}) -{" "}
                                          {formatBankAccountSummary(account)}
                                          {!isSufficient ? " [KURANG]" : ""}
                                        </option>
                                      );
                                    },
                                  )}
                                </Select>
                              </FormControl>
                            )}
                            <FormControl>
                              <FormLabel fontSize="sm">
                                Referensi Pembayaran
                              </FormLabel>
                              <Input
                                placeholder="No. Transfer / No. Cek"
                                value={accountingCOAForm.payment_reference}
                                onChange={(e) =>
                                  setAccountingCOAForm((prev) => ({
                                    ...prev,
                                    payment_reference: e.target.value,
                                  }))
                                }
                              />
                            </FormControl>
                          </SimpleGrid>
                          {isAccountingFinalReviewBalanceInsufficient() && (
                            <Alert
                              status="error"
                              borderRadius="lg"
                              variant="left-accent"
                              mt={4}
                            >
                              <AlertIcon />
                              <AlertDescription fontSize="xs">
                                Saldo{" "}
                                {
                                  getAccountingFinalReviewSelectedBankAccount()
                                    ?.name
                                }{" "}
                                tidak mencukupi untuk pembayaran{" "}
                                {formatCurrency(
                                  calculateAccountingFinalReviewTotal(),
                                )}
                                .
                              </AlertDescription>
                            </Alert>
                          )}
                          <Flex justify="flex-end" mt={4}>
                            <Box textAlign="right">
                              <Text fontSize="xs" color={textSecondary}>
                                Total Setelah Review
                              </Text>
                              <Text
                                fontSize="md"
                                fontWeight="bold"
                                color="blue.600"
                              >
                                {formatCurrency(
                                  calculateAccountingFinalReviewTotal(),
                                )}
                              </Text>
                            </Box>
                          </Flex>
                        </Box>
                      )}

                      {getAccountingCOAPaymentMethod() === "CREDIT" && (
                        <FormControl isRequired>
                          <FormLabel fontSize="sm">
                            Akun Hutang / Liability
                          </FormLabel>
                          <Select
                            placeholder="Pilih akun hutang"
                            value={accountingCOAForm.credit_account_id}
                            onChange={(e) =>
                              setAccountingCOAForm((prev) => ({
                                ...prev,
                                credit_account_id: e.target.value,
                              }))
                            }
                          >
                            {creditAccounts.map((account) => (
                              <option
                                key={account.id}
                                value={account.id.toString()}
                              >
                                {account.code} - {account.name}
                              </option>
                            ))}
                          </Select>
                          <FormHelperText>
                            Akun ini menjadi sisi kredit untuk purchase kredit.
                          </FormHelperText>
                        </FormControl>
                      )}

                      <Box
                        borderWidth="1px"
                        borderColor={borderColor}
                        borderRadius="lg"
                        overflow="hidden"
                      >
                        <TableContainer>
                          <Table size="sm">
                            <Thead bg={tableHeaderBg}>
                              <Tr>
                                <Th>Item</Th>
                                <Th isNumeric>Qty</Th>
                                <Th isNumeric>Total</Th>
                                <Th minW="320px">GL/COA Debit</Th>
                              </Tr>
                            </Thead>
                            <Tbody>
                              {accountingCOAForm.items.map((item, index) => (
                                <Tr key={item.purchase_item_id}>
                                  <Td fontWeight="medium">{item.label}</Td>
                                  <Td isNumeric>{item.quantity}</Td>
                                  <Td isNumeric>
                                    {formatCurrency(item.total_price)}
                                  </Td>
                                  <Td>
                                    <Select
                                      placeholder="Pilih GL/COA"
                                      value={item.expense_account_id}
                                      onChange={(e) =>
                                        setAccountingCOAForm((prev) => ({
                                          ...prev,
                                          items: prev.items.map((row, idx) =>
                                            idx === index
                                              ? {
                                                  ...row,
                                                  expense_account_id:
                                                    e.target.value,
                                                }
                                              : row,
                                          ),
                                        }))
                                      }
                                    >
                                      {expenseAccounts.map((account) => (
                                        <option
                                          key={account.id}
                                          value={account.id?.toString()}
                                        >
                                          {account.code} - {account.name} (
                                          {account.type})
                                        </option>
                                      ))}
                                    </Select>
                                  </Td>
                                </Tr>
                              ))}
                            </Tbody>
                          </Table>
                        </TableContainer>
                      </Box>
                    </VStack>
                  </ModalBody>
                  <ModalFooter
                    borderTop="1px solid"
                    borderColor={borderColor}
                    pt={4}
                  >
                    <HStack spacing={3} w="full" justify="flex-end">
                      <Button
                        variant="ghost"
                        onClick={onAccountingCOAClose}
                        isDisabled={savingAccountingCOA}
                      >
                        Batal
                      </Button>
                      <Button
                        colorScheme={
                          isPendingFinalApprovalPurchase(accountingCOAPurchase)
                            ? "purple"
                            : "orange"
                        }
                        onClick={handleSaveAccountingCOA}
                        isLoading={savingAccountingCOA}
                        loadingText={
                          isPendingFinalApprovalPurchase(accountingCOAPurchase)
                            ? "Finalisasi..."
                            : "Menyimpan..."
                        }
                        leftIcon={<FiCheckCircle />}
                      >
                        {isPendingFinalApprovalPurchase(accountingCOAPurchase)
                          ? "Simpan & Finalisasi Jurnal"
                          : "Simpan COA"}
                      </Button>
                    </HStack>
                  </ModalFooter>
                </ModalContent>
              </Modal>

              {/* Finance Tax & Payment Configuration Modal (Read-Only Items) */}
              <Modal
                isOpen={isFinanceTaxOpen}
                onClose={onFinanceTaxClose}
                size="4xl"
                scrollBehavior="inside"
              >
                <ModalOverlay bg="blackAlpha.600" backdropFilter="blur(4px)" />
                <ModalContent bg={modalBg} borderRadius="xl" maxH="90vh">
                  <ModalHeader
                    borderBottom="1px solid"
                    borderColor={borderColor}
                    pb={4}
                  >
                    <VStack align="start" spacing={1}>
                      <Heading size="md" color={headingColor}>
                        Konfigurasi Pajak &amp; Pembayaran
                      </Heading>
                      <Text fontSize="sm" color={textSecondary}>
                        {financeTaxPurchase?.code} —{" "}
                        {financeTaxPurchase?.vendor?.name || "N/A"}
                      </Text>
                    </VStack>
                  </ModalHeader>
                  <ModalCloseButton />
                  <ModalBody py={6}>
                    <VStack spacing={6} align="stretch">
                      {/* Read-Only Purchase Summary */}
                      <Box
                        borderWidth="1px"
                        borderColor={borderColor}
                        borderRadius="lg"
                        p={4}
                        bg={modalHeaderBg}
                      >
                        <Text
                          fontWeight="bold"
                          mb={3}
                          color={headingColor}
                          fontSize="sm"
                        >
                          Ringkasan Purchase (Read-Only)
                        </Text>
                        <SimpleGrid
                          columns={{ base: 1, md: 3 }}
                          spacing={3}
                          mb={4}
                        >
                          <Box>
                            <Text fontSize="xs" color={textSecondary}>
                              Tanggal
                            </Text>
                            <Text fontSize="sm" fontWeight="medium">
                              {financeTaxPurchase?.date
                                ? formatDate(financeTaxPurchase.date)
                                : "-"}
                            </Text>
                          </Box>
                          <Box>
                            <Text fontSize="xs" color={textSecondary}>
                              Subtotal (Sebelum Diskon)
                            </Text>
                            <Text
                              fontSize="sm"
                              fontWeight="bold"
                              color="blue.600"
                            >
                              {formatCurrency(
                                financeTaxPurchase?.subtotal_before_discount ||
                                  0,
                              )}
                            </Text>
                          </Box>
                          <Box>
                            <Text fontSize="xs" color={textSecondary}>
                              Diskon
                            </Text>
                            <Text fontSize="sm" fontWeight="medium">
                              {financeTaxPurchase?.discount || 0}%
                            </Text>
                          </Box>
                        </SimpleGrid>

                        {/* Items Table (Read-Only) */}
                        <Text
                          fontSize="xs"
                          color={textSecondary}
                          mb={2}
                          fontWeight="semibold"
                        >
                          Item Pembelian
                        </Text>
                        <TableContainer>
                          <Table size="sm" variant="simple">
                            <Thead bg={tableHeaderBg}>
                              <Tr>
                                <Th fontSize="xs">Produk</Th>
                                <Th fontSize="xs" isNumeric>
                                  Qty
                                </Th>
                                <Th fontSize="xs" isNumeric>
                                  Harga Satuan
                                </Th>
                                <Th fontSize="xs" isNumeric>
                                  Total
                                </Th>
                              </Tr>
                            </Thead>
                            <Tbody>
                              {financeTaxPurchase?.purchase_items?.map(
                                (item: any, idx) => (
                                  <Tr key={idx}>
                                    <Td fontSize="sm">
                                      {getPurchaseItemDisplayName(item)}
                                    </Td>
                                    <Td fontSize="sm" isNumeric>
                                      {item.quantity}
                                    </Td>
                                    <Td fontSize="sm" isNumeric>
                                      {formatCurrency(item.unit_price)}
                                    </Td>
                                    <Td
                                      fontSize="sm"
                                      isNumeric
                                      fontWeight="medium"
                                    >
                                      {formatCurrency(item.total_price)}
                                    </Td>
                                  </Tr>
                                ),
                              )}
                            </Tbody>
                          </Table>
                        </TableContainer>
                        <Flex justify="flex-end" mt={3}>
                          <Box textAlign="right">
                            <Text fontSize="xs" color={textSecondary}>
                              Net Sebelum Pajak
                            </Text>
                            <Text
                              fontSize="md"
                              fontWeight="bold"
                              color={headingColor}
                            >
                              {formatCurrency(
                                financeTaxPurchase?.net_before_tax || 0,
                              )}
                            </Text>
                          </Box>
                        </Flex>
                      </Box>

                      {/* Editable: Tax Configuration */}
                      <Box
                        borderWidth="1px"
                        borderColor={borderColor}
                        borderRadius="lg"
                        p={4}
                      >
                        <Text
                          fontWeight="bold"
                          mb={3}
                          color={headingColor}
                          fontSize="sm"
                        >
                          Konfigurasi Pajak
                        </Text>
                        <SimpleGrid columns={{ base: 1, md: 2 }} spacing={4}>
                          <FormControl>
                            <FormLabel fontSize="sm">PPN Rate (%)</FormLabel>
                            <Input
                              type="number"
                              step="0.01"
                              value={financeTaxForm.ppn_rate}
                              onChange={(e) =>
                                setFinanceTaxForm((prev) => ({
                                  ...prev,
                                  ppn_rate: e.target.value,
                                }))
                              }
                              _hover={{ borderColor: inputHoverBorder }}
                              _focus={{
                                borderColor: inputFocusBorder,
                                boxShadow: inputFocusShadow,
                              }}
                            />
                            <FormHelperText fontSize="xs">
                              Contoh: 11 untuk PPN 11%
                            </FormHelperText>
                          </FormControl>
                          <FormControl>
                            <FormLabel fontSize="sm">
                              Pajak Tambahan Lainnya (Rp)
                            </FormLabel>
                            <Input
                              type="number"
                              step="0.01"
                              value={financeTaxForm.other_tax_additions}
                              onChange={(e) =>
                                setFinanceTaxForm((prev) => ({
                                  ...prev,
                                  other_tax_additions: e.target.value,
                                }))
                              }
                              _hover={{ borderColor: inputHoverBorder }}
                              _focus={{
                                borderColor: inputFocusBorder,
                                boxShadow: inputFocusShadow,
                              }}
                            />
                          </FormControl>
                          <FormControl>
                            <FormLabel fontSize="sm">
                              Freight / Biaya Pengiriman
                            </FormLabel>
                            <CurrencyInput
                              value={
                                parseFloat(financeTaxForm.freight_cost) || 0
                              }
                              onChange={(value) =>
                                setFinanceTaxForm((prev) => ({
                                  ...prev,
                                  freight_cost: value.toString(),
                                }))
                              }
                              placeholder="0"
                              size="sm"
                              min={0}
                            />
                            <FormHelperText fontSize="xs">
                              Masuk ke nilai inventory dan HPP setelah purchase
                              diposting.
                            </FormHelperText>
                          </FormControl>
                          <FormControl>
                            <FormLabel fontSize="sm">PPh 21 Rate (%)</FormLabel>
                            <Input
                              type="number"
                              step="0.01"
                              value={financeTaxForm.pph21_rate}
                              onChange={(e) =>
                                setFinanceTaxForm((prev) => ({
                                  ...prev,
                                  pph21_rate: e.target.value,
                                }))
                              }
                              _hover={{ borderColor: inputHoverBorder }}
                              _focus={{
                                borderColor: inputFocusBorder,
                                boxShadow: inputFocusShadow,
                              }}
                            />
                          </FormControl>
                          <FormControl>
                            <FormLabel fontSize="sm">PPh 23 Rate (%)</FormLabel>
                            <Input
                              type="number"
                              step="0.01"
                              value={financeTaxForm.pph23_rate}
                              onChange={(e) =>
                                setFinanceTaxForm((prev) => ({
                                  ...prev,
                                  pph23_rate: e.target.value,
                                }))
                              }
                              _hover={{ borderColor: inputHoverBorder }}
                              _focus={{
                                borderColor: inputFocusBorder,
                                boxShadow: inputFocusShadow,
                              }}
                            />
                          </FormControl>
                          <FormControl>
                            <FormLabel fontSize="sm">
                              Potongan Pajak Lainnya (Rp)
                            </FormLabel>
                            <Input
                              type="number"
                              step="0.01"
                              value={financeTaxForm.other_tax_deductions}
                              onChange={(e) =>
                                setFinanceTaxForm((prev) => ({
                                  ...prev,
                                  other_tax_deductions: e.target.value,
                                }))
                              }
                              _hover={{ borderColor: inputHoverBorder }}
                              _focus={{
                                borderColor: inputFocusBorder,
                                boxShadow: inputFocusShadow,
                              }}
                            />
                          </FormControl>
                        </SimpleGrid>
                      </Box>

                      {/* Editable: Payment Configuration */}
                      <Box
                        borderWidth="1px"
                        borderColor={borderColor}
                        borderRadius="lg"
                        p={4}
                      >
                        <Text
                          fontWeight="bold"
                          mb={3}
                          color={headingColor}
                          fontSize="sm"
                        >
                          Metode Pembayaran
                        </Text>
                        <SimpleGrid columns={{ base: 1, md: 2 }} spacing={4}>
                          <FormControl>
                            <FormLabel fontSize="sm">
                              Metode Pembayaran
                            </FormLabel>
                            <Select
                              value={financeTaxForm.payment_method}
                              onChange={(e) =>
                                setFinanceTaxForm((prev) => ({
                                  ...prev,
                                  payment_method: e.target.value,
                                  bank_account_id: "",
                                  credit_account_id: "",
                                }))
                              }
                              _hover={{ borderColor: inputHoverBorder }}
                              _focus={{
                                borderColor: inputFocusBorder,
                                boxShadow: inputFocusShadow,
                              }}
                            >
                              <option value="CREDIT">
                                Kredit (Bayar Nanti)
                              </option>
                              <option value="CASH">Tunai</option>
                              <option value="BANK_TRANSFER">
                                Transfer Bank
                              </option>
                              <option value="CHECK">Cek/Giro</option>
                            </Select>
                          </FormControl>

                          {(financeTaxForm.payment_method === "CASH" ||
                            financeTaxForm.payment_method === "BANK_TRANSFER" ||
                            financeTaxForm.payment_method === "CHECK") && (
                            <FormControl isInvalid={isBalanceInsufficient()}>
                              <FormLabel fontSize="sm">
                                Akun Bank / Kas
                              </FormLabel>
                              <Select
                                placeholder="Pilih Akun Bank/Kas"
                                value={financeTaxForm.bank_account_id}
                                onChange={(e) =>
                                  setFinanceTaxForm((prev) => ({
                                    ...prev,
                                    bank_account_id: e.target.value,
                                  }))
                                }
                                _hover={{ borderColor: inputHoverBorder }}
                                _focus={{
                                  borderColor: isBalanceInsufficient()
                                    ? "red.500"
                                    : inputFocusBorder,
                                  boxShadow: isBalanceInsufficient()
                                    ? "0 0 0 1px red"
                                    : inputFocusShadow,
                                }}
                                borderColor={
                                  isBalanceInsufficient()
                                    ? "red.500"
                                    : undefined
                                }
                              >
                                {getEligibleBankAccounts().map((account) => {
                                  const nativeBalance = account.balance ?? 0;
                                  const bookBalance =
                                    getAccountBookBalance(account);
                                  const totalPayment =
                                    calculateFinanceTaxTotal();
                                  const isSufficient =
                                    bookBalance >= totalPayment;
                                  return (
                                    <option
                                      key={account.id}
                                      value={account.id.toString()}
                                    >
                                      {account.name} ({account.code}) -{" "}
                                      {account.currency} | Saldo:{" "}
                                      {formatAccountBalance(
                                        nativeBalance,
                                        account.currency,
                                      )}
                                      {account.currency?.toUpperCase() !== "IDR"
                                        ? ` | Nilai buku: ${formatCurrency(bookBalance)}`
                                        : ""}
                                      {!isSufficient ? " [KURANG]" : ""}
                                    </option>
                                  );
                                })}
                              </Select>

                              {/* Show selected account balance info */}
                              {financeTaxForm.bank_account_id &&
                                getSelectedBankAccount() &&
                                !isBalanceInsufficient() && (
                                  <Text
                                    fontSize="xs"
                                    color="green.600"
                                    mt={1}
                                    fontWeight="medium"
                                  >
                                    Saldo tersedia:{" "}
                                    {formatAccountBalance(
                                      getSelectedBankAccount()?.balance ?? 0,
                                      getSelectedBankAccount()?.currency,
                                    )}{" "}
                                    (mencukupi)
                                  </Text>
                                )}
                            </FormControl>
                          )}

                          {/* Insufficient Balance Warning */}
                          {isBalanceInsufficient() && (
                            <Box gridColumn={{ md: "span 2" }}>
                              <Alert
                                status="error"
                                borderRadius="lg"
                                variant="left-accent"
                              >
                                <AlertIcon />
                                <Box flex="1">
                                  <AlertTitle fontSize="sm">
                                    Saldo Tidak Mencukupi
                                  </AlertTitle>
                                  <AlertDescription fontSize="xs">
                                    <Text>
                                      Saldo{" "}
                                      <strong>
                                        {getSelectedBankAccount()?.name}
                                      </strong>{" "}
                                      (
                                      {formatBankAccountSummary(
                                        getSelectedBankAccount(),
                                      )}
                                      ) tidak mencukupi untuk pembayaran sebesar{" "}
                                      <strong>
                                        {formatCurrency(
                                          calculateFinanceTaxTotal(),
                                        )}
                                      </strong>
                                      .
                                    </Text>
                                    <Text mt={1}>
                                      Kekurangan:{" "}
                                      <strong>
                                        {formatCurrency(
                                          calculateFinanceTaxTotal() -
                                            getAccountBookBalance(
                                              getSelectedBankAccount(),
                                            ),
                                        )}
                                      </strong>
                                    </Text>
                                  </AlertDescription>
                                </Box>
                              </Alert>

                              {/* Recommended accounts */}
                              {getSufficientAccounts().length > 0 ? (
                                <Box
                                  mt={3}
                                  p={3}
                                  bg="green.50"
                                  borderRadius="md"
                                  borderWidth="1px"
                                  borderColor="green.200"
                                >
                                  <Text
                                    fontSize="xs"
                                    fontWeight="bold"
                                    color="green.700"
                                    mb={2}
                                  >
                                    Rekomendasi akun dengan saldo mencukupi:
                                  </Text>
                                  <VStack align="stretch" spacing={1}>
                                    {getSufficientAccounts().map((account) => (
                                      <Flex
                                        key={account.id}
                                        justify="space-between"
                                        align="center"
                                        p={2}
                                        bg="white"
                                        borderRadius="md"
                                        borderWidth="1px"
                                        borderColor="green.100"
                                        cursor="pointer"
                                        _hover={{
                                          bg: "green.50",
                                          borderColor: "green.300",
                                        }}
                                        onClick={() =>
                                          setFinanceTaxForm((prev) => ({
                                            ...prev,
                                            bank_account_id:
                                              account.id.toString(),
                                          }))
                                        }
                                      >
                                        <Box>
                                          <Text
                                            fontSize="xs"
                                            fontWeight="medium"
                                          >
                                            {account.name} ({account.code})
                                          </Text>
                                          <Text fontSize="xs" color="gray.500">
                                            {account.currency}
                                          </Text>
                                        </Box>
                                        <Text
                                          fontSize="xs"
                                          fontWeight="bold"
                                          color="green.600"
                                        >
                                          {formatBankAccountSummary(account)}
                                        </Text>
                                      </Flex>
                                    ))}
                                  </VStack>
                                </Box>
                              ) : (
                                <Alert
                                  status="warning"
                                  borderRadius="lg"
                                  mt={3}
                                  variant="left-accent"
                                >
                                  <AlertIcon />
                                  <AlertDescription fontSize="xs">
                                    Tidak ada akun Bank/Kas yang memiliki saldo
                                    mencukupi (
                                    {formatCurrency(calculateFinanceTaxTotal())}
                                    ). Pertimbangkan menggunakan metode{" "}
                                    <strong>Kredit (Bayar Nanti)</strong> atau
                                    tambahkan dana ke salah satu akun terlebih
                                    dahulu.
                                  </AlertDescription>
                                </Alert>
                              )}
                            </Box>
                          )}

                          {financeTaxForm.payment_method === "CREDIT" &&
                            (canSelectAccountFromCOA ? (
                              <FormControl>
                                <FormLabel fontSize="sm">
                                  Akun Hutang (Liability)
                                </FormLabel>
                                <Select
                                  placeholder="Pilih Akun Hutang"
                                  value={financeTaxForm.credit_account_id}
                                  onChange={(e) =>
                                    setFinanceTaxForm((prev) => ({
                                      ...prev,
                                      credit_account_id: e.target.value,
                                    }))
                                  }
                                  _hover={{ borderColor: inputHoverBorder }}
                                  _focus={{
                                    borderColor: inputFocusBorder,
                                    boxShadow: inputFocusShadow,
                                  }}
                                >
                                  {creditAccounts.map((account) => (
                                    <option
                                      key={account.id}
                                      value={account.id.toString()}
                                    >
                                      {account.name} ({account.code})
                                    </option>
                                  ))}
                                </Select>
                              </FormControl>
                            ) : (
                              <Box
                                borderWidth="1px"
                                borderColor={borderColor}
                                borderRadius="md"
                                bg={inputBg}
                                p={3}
                              >
                                <Text fontSize="xs" fontWeight="semibold">
                                  COA Liability Menunggu Accounting
                                </Text>
                                <Text fontSize="xs" color={textSecondary}>
                                  Finance Staff dapat mengatur pajak dan metode
                                  pembayaran, tetapi GL/COA akan dilengkapi
                                  Accounting setelah approval Finance Manager.
                                </Text>
                              </Box>
                            ))}

                          <FormControl>
                            <FormLabel fontSize="sm">
                              Referensi Pembayaran
                            </FormLabel>
                            <Input
                              placeholder="No. Transfer / No. Cek"
                              value={financeTaxForm.payment_reference}
                              onChange={(e) =>
                                setFinanceTaxForm((prev) => ({
                                  ...prev,
                                  payment_reference: e.target.value,
                                }))
                              }
                              _hover={{ borderColor: inputHoverBorder }}
                              _focus={{
                                borderColor: inputFocusBorder,
                                boxShadow: inputFocusShadow,
                              }}
                            />
                          </FormControl>
                        </SimpleGrid>
                      </Box>
                    </VStack>
                  </ModalBody>
                  <ModalFooter
                    borderTop="1px solid"
                    borderColor={borderColor}
                    pt={4}
                  >
                    <HStack spacing={3} w="full" justify="flex-end">
                      <Button
                        variant="ghost"
                        onClick={onFinanceTaxClose}
                        isDisabled={savingFinanceTax}
                      >
                        Batal
                      </Button>
                      <Button
                        colorScheme={isBalanceInsufficient() ? "red" : "teal"}
                        onClick={() => handleSaveFinanceTax()}
                        isLoading={savingFinanceTax}
                        isDisabled={isBalanceInsufficient()}
                        loadingText="Menyimpan..."
                        leftIcon={<FiCheckCircle />}
                        title={
                          isBalanceInsufficient()
                            ? "Saldo akun bank/kas tidak mencukupi. Pilih akun lain atau gunakan metode Kredit."
                            : ""
                        }
                      >
                        {isBalanceInsufficient()
                          ? "Saldo Tidak Mencukupi"
                          : "Simpan Konfigurasi"}
                      </Button>
                    </HStack>
                  </ModalFooter>
                </ModalContent>
              </Modal>
            </VStack>
          </Box>
        </SimpleLayout>
      </CompanyTypeGuard>
    </ProtectedModule>
  );
};

export default PurchasesPage;
