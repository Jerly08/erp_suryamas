"use client";

import React, { useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import {
  Box,
  ButtonGroup,
  Checkbox,
  Button,
  Flex,
  Heading,
  Icon,
  Input,
  InputGroup,
  InputLeftElement,
  Table,
  TableContainer,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  Text,
  useColorModeValue,
  useDisclosure,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalCloseButton,
  ModalBody,
  ModalFooter,
  AlertDialog,
  AlertDialogOverlay,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogBody,
  AlertDialogFooter,
  FormControl,
  FormLabel,
  useToast,
  IconButton,
  Tooltip,
  Badge,
  Textarea,
  Select,
  Grid,
} from "@chakra-ui/react";
import {
  FiPlus,
  FiSearch,
  FiTrash2,
  FiCheck,
  FiEye,
  FiPaperclip,
  FiDownload,
  FiX,
  FiFileText,
  FiChevronLeft,
  FiChevronRight,
  FiEdit3,
  FiPackage,
} from "react-icons/fi";
import paymentService from "@/services/paymentService";
import Layout from "@/components/layout/Layout";
import ProtectedModule from "@/components/common/ProtectedModule";
import { usePermissions } from "@/hooks/usePermissions";
import CurrencyInput from "@/components/common/CurrencyInput";
import SearchableSelect from "@/components/common/SearchableSelect";
import {
  purchaseRequestService,
  PurchaseRequest,
  PurchaseRequestCreatePayload,
  PurchaseRequestDocument,
  PurchaseRequestItem,
} from "@/services/purchaseRequestService";
import { projectService, Project } from "@/services/projectService";
import searchableSelectService, {
  Contact as SelectContact,
} from "@/services/searchableSelectService";
import { useAuth } from "@/contexts/AuthContext";
import api from "@/services/api";
import purchaseService from "@/services/purchaseService";
import {
  humanizeRole,
  isAccountingRole,
  isFinanceManagerRole,
  isFinanceStaffRole,
  isFinanceTeamRole,
  normalizeRole,
} from "@/utils/roles";

const ACCEPTED_DOCUMENT_TYPES =
  ".pdf,.doc,.docx,.xls,.xlsx,.csv,.txt,.jpg,.jpeg,.png,.webp";
const MAX_DOCUMENT_SIZE_MB = 50;
const MAX_DOCUMENT_SIZE = MAX_DOCUMENT_SIZE_MB * 1024 * 1024;
const DOCUMENT_UPLOAD_TIMEOUT_MESSAGE =
  "Upload dokumen membutuhkan waktu lebih lama dari biasanya. Cek koneksi internet, pastikan pengajuan belum tersimpan, lalu coba lagi.";
const PURCHASE_REQUEST_PAGE_SIZE_OPTIONS = [10, 25, 50];
const PURCHASE_REQUEST_PROGRESS_FILTERS = [
  { value: "", label: "Semua Progress" },
  { value: "WAITING_FINANCE_STAFF", label: "Belum Review Finance Staff" },
  { value: "FINANCE_STAFF_APPROVED", label: "Sudah Review Finance Staff" },
  { value: "WAITING_FINANCE_MANAGER", label: "Belum Review Finance Manager" },
  { value: "FINANCE_MANAGER_APPROVED", label: "Sudah Review Finance Manager" },
  {
    value: "FINANCE_MANAGER_APPROVED_WITHOUT_PURCHASE",
    label: "FM Approved - Belum Pembelian",
  },
  { value: "AWAITING_VENDOR_INVOICE", label: "Menunggu Invoice Aktual" },
  { value: "VENDOR_INVOICE_REVIEW", label: "Invoice Aktual Dalam Review" },
  { value: "AWAITING_REQUESTER_REVISION", label: "Menunggu Revisi PIC" },
  { value: "REQUESTER_REVISION_REVIEW", label: "Revisi PIC Dalam Review" },
  { value: "WAITING_DIRECTOR", label: "Menunggu Director" },
  { value: "APPROVED", label: "PR Approved" },
  { value: "REJECTED", label: "PR Rejected" },
  { value: "CANCELLED", label: "PR Cancelled" },
  { value: "PURCHASE_CREATED", label: "Sudah Dibuat Pembelian" },
  { value: "PAYMENT_CREATED", label: "Sudah Ada Pembayaran" },
] as const;

type PurchaseRequestProgressFilter =
  (typeof PURCHASE_REQUEST_PROGRESS_FILTERS)[number]["value"];

type PurchaseRequestFormItem = {
  source: "catalog" | "manual";
  product_id: string;
  item_name: string;
  quantity: number | string;
  estimated_price: number | string;
  notes: string;
};

type InvoiceRevisionFormItem = {
  item_id: number;
  item_name: string;
  quantity: number | string;
  estimated_price: number | string;
  notes: string;
};

type RequesterRevisionFormItem = {
  client_id: string;
  item_id?: number;
  source: "catalog" | "manual";
  product_id: string;
  item_name: string;
  quantity: number | string;
  estimated_price: number | string;
  notes: string;
};

type FinalizedPaymentReceipt = {
  purchaseId: number;
  purchaseCode: string;
  purchasePaymentId: number;
  paymentId?: number | null;
  paymentCode: string;
  hasPaymentProof?: boolean;
  hasUploadedPaymentProof?: boolean;
  paymentProofFileName?: string;
  paymentProofReviewStatus?: string;
  amount: number;
  date?: string;
  method?: string;
  reference?: string;
};

const createEmptyPurchaseRequestItem = (): PurchaseRequestFormItem => ({
  source: "catalog",
  product_id: "",
  item_name: "",
  quantity: 1,
  estimated_price: 0,
  notes: "",
});

const createRequesterRevisionItemKey = () =>
  `requester-revision-${Date.now()}-${Math.random().toString(36).slice(2)}`;

const createEmptyRequesterRevisionItem = (): RequesterRevisionFormItem => ({
  client_id: createRequesterRevisionItemKey(),
  source: "catalog",
  product_id: "",
  item_name: "",
  quantity: 1,
  estimated_price: 0,
  notes: "",
});

export default function PurchaseRequestsPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [requests, setRequests] = useState<PurchaseRequest[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [vendors, setVendors] = useState<SelectContact[]>([]);
  const [products, setProducts] = useState<any[]>([]); // simplified type for catalogue items
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [vendorFilter, setVendorFilter] = useState("");
  const [progressFilter, setProgressFilter] =
    useState<PurchaseRequestProgressFilter>("");
  const [startDateFilter, setStartDateFilter] = useState("");
  const [endDateFilter, setEndDateFilter] = useState("");
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isUploadingDocuments, setIsUploadingDocuments] = useState(false);
  const [isUpdatingProject, setIsUpdatingProject] = useState(false);
  const [isUpdatingInvoiceTerms, setIsUpdatingInvoiceTerms] = useState(false);
  const [isSubmittingInvoiceRevision, setIsSubmittingInvoiceRevision] =
    useState(false);
  const [isOpeningRequesterRevision, setIsOpeningRequesterRevision] =
    useState(false);
  const [isSubmittingRequesterRevision, setIsSubmittingRequesterRevision] =
    useState(false);
  const [isCancellingRequest, setIsCancellingRequest] = useState(false);
  const [isUploadingRealizationDocuments, setIsUploadingRealizationDocuments] =
    useState(false);
  const [pendingAdditionalDocuments, setPendingAdditionalDocuments] = useState<
    File[]
  >([]);
  const [pendingRealizationDocuments, setPendingRealizationDocuments] =
    useState<File[]>([]);
  const { canCreate, canEdit, canApprove } = usePermissions();
  const formatCurrency = (value: number) => {
    const numericValue = Number(value) || 0;
    const hasFraction = Math.abs(numericValue % 1) > Number.EPSILON;

    return new Intl.NumberFormat("id-ID", {
      style: "currency",
      currency: "IDR",
      minimumFractionDigits: hasFraction ? 2 : 0,
      maximumFractionDigits: 2,
    }).format(numericValue);
  };
  const formatDocumentUploadTime = (value?: string | null) => {
    if (!value) return "";
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return "";

    return date.toLocaleString("id-ID", {
      day: "2-digit",
      month: "short",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
      timeZone: "Asia/Jakarta",
      timeZoneName: "short",
    });
  };
  const getDocumentUploaderName = (document: PurchaseRequestDocument) => {
    const uploader = document.uploader;
    if (!uploader) return "";
    const fullName = `${uploader.first_name || ""} ${uploader.last_name || ""}`.trim();
    return fullName || uploader.username || uploader.email || "";
  };
  const getDocumentUploadMeta = (document: PurchaseRequestDocument) => {
    const pieces = [formatFileSize(document.file_size)];
    const uploaderName = getDocumentUploaderName(document);
    const uploadedAt = formatDocumentUploadTime(document.created_at);
    if (uploaderName && uploadedAt) {
      pieces.push(`diunggah oleh ${uploaderName} pada ${uploadedAt}`);
    } else if (uploaderName) {
      pieces.push(`diunggah oleh ${uploaderName}`);
    } else if (uploadedAt) {
      pieces.push(`diunggah pada ${uploadedAt}`);
    }
    return pieces.join(" - ");
  };
  const toast = useToast();
  const { user } = useAuth();
  const userDepartment =
    (user as { department?: string } | null)?.department || "";
  const roleNorm = normalizeRole(user?.role);
  const canApprovePurchaseRequest =
    roleNorm === "director" || isFinanceTeamRole(roleNorm);
  const canUpdatePurchaseRequestProject =
    roleNorm === "admin" || isFinanceTeamRole(roleNorm);
  const canCancelPurchaseRequests =
    canEdit("purchase_requests") &&
    (roleNorm === "admin" || isFinanceManagerRole(roleNorm));

  const {
    isOpen: isCreateOpen,
    onOpen: onCreateOpen,
    onClose: onCreateClose,
  } = useDisclosure();
  const {
    isOpen: isViewOpen,
    onOpen: onViewOpen,
    onClose: onViewClose,
  } = useDisclosure();
  const {
    isOpen: isCancelOpen,
    onOpen: onCancelOpen,
    onClose: onCancelClose,
  } = useDisclosure();
  const {
    isOpen: isRequesterRevisionOpen,
    onOpen: onRequesterRevisionOpen,
    onClose: onRequesterRevisionClose,
  } = useDisclosure();

  const [selectedRequest, setSelectedRequest] =
    useState<PurchaseRequest | null>(null);
  const [reviewReason, setReviewReason] = useState("");
  const [projectUpdateId, setProjectUpdateId] = useState("");
  const [invoiceTermsUpdateReceivedDate, setInvoiceTermsUpdateReceivedDate] =
    useState("");
  const [invoiceTermsUpdateDays, setInvoiceTermsUpdateDays] = useState("");
  const [reviewNeedsDirectorApproval, setReviewNeedsDirectorApproval] =
    useState(false);
  const [reviewRequiresVendorInvoice, setReviewRequiresVendorInvoice] =
    useState(false);
  const [reviewDecisions, setReviewDecisions] = useState<
    Record<number, "APPROVED" | "REJECTED" | "">
  >({});
  const [invoiceRevisionItems, setInvoiceRevisionItems] = useState<
    InvoiceRevisionFormItem[]
  >([]);
  const [invoiceRevisionDocuments, setInvoiceRevisionDocuments] = useState<
    File[]
  >([]);
  const [invoiceRevisionNotes, setInvoiceRevisionNotes] = useState("");
  const [requesterRevisionOpenReason, setRequesterRevisionOpenReason] =
    useState("");
  const [requesterRevisionItems, setRequesterRevisionItems] = useState<
    RequesterRevisionFormItem[]
  >([]);
  const [requesterRevisionDocuments, setRequesterRevisionDocuments] = useState<
    File[]
  >([]);
  const [requesterRevisionNotes, setRequesterRevisionNotes] = useState("");
  const [cancelReason, setCancelReason] = useState("");
  const handledNotificationRequestId = useRef<string | null>(null);
  const cancelLeastDestructiveRef = useRef<HTMLButtonElement>(null);
  const requesterRevisionLeastDestructiveRef = useRef<HTMLButtonElement>(null);

  // Form state
  const [title, setTitle] = useState("");
  const [department, setDepartment] = useState("");
  const [projectId, setProjectId] = useState("");
  const [vendorId, setVendorId] = useState("");
  const [vendorInvoiceReceivedDate, setVendorInvoiceReceivedDate] =
    useState("");
  const [paymentTermsDays, setPaymentTermsDays] = useState("30");
  const [notes, setNotes] = useState("");
  const [documents, setDocuments] = useState<File[]>([]);
  const [items, setItems] = useState<PurchaseRequestFormItem[]>([
    createEmptyPurchaseRequestItem(),
  ]);

  // Theme colors
  const bgCard = useColorModeValue("white", "gray.800");
  const textColor = useColorModeValue("gray.700", "white");
  const mutedTextColor = useColorModeValue("gray.500", "gray.400");
  const borderColor = useColorModeValue("gray.200", "gray.700");
  const tableHeaderBg = useColorModeValue("gray.50", "gray.700");
  const inputBg = useColorModeValue("white", "gray.700");
  const inputBorderColor = useColorModeValue("gray.300", "gray.600");
  const modalBg = useColorModeValue("white", "gray.800");
  const modalHeaderBg = useColorModeValue("gray.50", "whiteAlpha.100");
  const modalFooterBg = useColorModeValue("gray.50", "whiteAlpha.50");
  const infoPanelBg = useColorModeValue("gray.50", "whiteAlpha.100");
  const notePanelBg = useColorModeValue("blue.50", "blue.900");
  const notePanelColor = useColorModeValue("blue.800", "blue.100");
  const rowHoverBg = useColorModeValue("gray.50", "whiteAlpha.100");
  const fieldFocusStyles = {
    borderColor: "blue.400",
    boxShadow: "0 0 0 1px var(--chakra-colors-blue-400)",
  };

  useEffect(() => {
    fetchData();
  }, []);

  const fetchData = async () => {
    try {
      setLoading(true);
      const [prsRes, projectsRes, vendorsRes, productsRes] = await Promise.all([
        purchaseRequestService.getAll(),
        projectService.getAll().catch(() => []),
        searchableSelectService
          .getVendors({ is_active: true, limit: 1000 })
          .catch(() => []),
        api
          .get("/api/v1/products")
          .then((res) => res.data.data)
          .catch(() => []),
      ]);
      setRequests(prsRes || []);
      setProjects(projectsRes || []);
      setVendors(vendorsRes || []);
      setProducts(productsRes || []);
    } catch (error) {
      toast({ title: "Gagal memuat data", status: "error", duration: 3000 });
    } finally {
      setLoading(false);
    }
  };

  const formatDateInputValue = (date: Date) => {
    const year = date.getFullYear();
    const month = `${date.getMonth() + 1}`.padStart(2, "0");
    const day = `${date.getDate()}`.padStart(2, "0");
    return `${year}-${month}-${day}`;
  };

  const normalizeDateInputValue = (value?: string | null) => {
    if (!value || value.startsWith("0001-01-01")) return "";

    const trimmedValue = value
      .trim()
      .replace(/[\u200e\u200f\u202a-\u202e]/g, "");
    const isoMatch = trimmedValue.match(/^(\d{4})-(\d{2})-(\d{2})/);
    if (isoMatch) {
      return `${isoMatch[1]}-${isoMatch[2]}-${isoMatch[3]}`;
    }

    const compactMatch = trimmedValue.match(/^(\d{8})$/);
    if (compactMatch) {
      const compactValue = compactMatch[1];
      const yearFirst = Number(compactValue.slice(0, 4));
      const isYearFirst = yearFirst >= 1900 && yearFirst <= 9999;
      const year = isYearFirst
        ? yearFirst
        : Number(compactValue.slice(4, 8));
      const month = Number(
        isYearFirst ? compactValue.slice(4, 6) : compactValue.slice(2, 4),
      );
      const day = Number(
        isYearFirst ? compactValue.slice(6, 8) : compactValue.slice(0, 2),
      );
      const parsedDate = new Date(year, month - 1, day);
      if (
        parsedDate.getFullYear() === year &&
        parsedDate.getMonth() === month - 1 &&
        parsedDate.getDate() === day
      ) {
        return formatDateInputValue(parsedDate);
      }

      return "";
    }

    const localizedMatch = trimmedValue.match(
      /^(\d{1,2})[/-](\d{1,2})[/-](\d{4})$/,
    );
    const numericParts = trimmedValue.match(/\d+/g) || [];
    if (!localizedMatch && numericParts.length < 3) return "";

    const [firstPart, secondPart, thirdPart] = localizedMatch
      ? [localizedMatch[1], localizedMatch[2], localizedMatch[3]]
      : numericParts;
    if (!firstPart || !secondPart || !thirdPart) return "";

    const firstNumber = Number(firstPart);
    const secondNumber = Number(secondPart);
    const thirdNumber = Number(thirdPart);
    const isYearFirst = firstPart.length === 4;
    const day = isYearFirst ? thirdNumber : firstNumber;
    const month = secondNumber;
    const year = isYearFirst ? firstNumber : thirdNumber;
    const parsedDate = new Date(year, month - 1, day);
    if (
      parsedDate.getFullYear() !== year ||
      parsedDate.getMonth() !== month - 1 ||
      parsedDate.getDate() !== day
    ) {
      return "";
    }

    return formatDateInputValue(parsedDate);
  };

  const formatDateInputDisplayValue = (value?: string | null) => {
    const normalizedValue = normalizeDateInputValue(value);
    if (!normalizedValue) return "";

    const [year, month, day] = normalizedValue.split("-").map(Number);
    return new Date(year, month - 1, day).toLocaleDateString("id-ID", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
    });
  };

  const addDaysToDateInput = (dateValue: string, dayCount: number) => {
    if (!dateValue || !Number.isFinite(dayCount) || dayCount < 0) return "";

    const normalizedDateValue = normalizeDateInputValue(dateValue);
    if (!normalizedDateValue) return "";

    const [year, month, day] = normalizedDateValue.split("-").map(Number);
    const date = new Date(year, month - 1, day);
    date.setDate(date.getDate() + dayCount);
    return formatDateInputValue(date);
  };

  const getCalculatedDueDateInput = (
    invoiceReceivedDate: string,
    termsDaysValue: string,
  ) => {
    if (!invoiceReceivedDate || termsDaysValue.trim() === "") return "";

    const termsDays = Number(termsDaysValue);
    if (!Number.isInteger(termsDays) || termsDays < 0) return "";

    return addDaysToDateInput(invoiceReceivedDate, termsDays);
  };

  const buildInvoiceTermsPayload = (
    invoiceReceivedDate: string,
    termsDaysValue: string,
    includeNulls = false,
    includeDueDate = true,
  ) => {
    const normalizedInvoiceReceivedDate =
      normalizeDateInputValue(invoiceReceivedDate);
    const dueDate = includeDueDate
      ? getCalculatedDueDateInput(
          normalizedInvoiceReceivedDate,
          termsDaysValue,
        )
      : "";
    const termsDays =
      termsDaysValue.trim() === "" ? null : Number(termsDaysValue);

    return {
      vendor_invoice_received_date: normalizedInvoiceReceivedDate
        ? `${normalizedInvoiceReceivedDate}T00:00:00Z`
        : includeNulls
          ? null
          : undefined,
      payment_terms_days:
        termsDays !== null && Number.isFinite(termsDays)
          ? termsDays
          : includeNulls
            ? null
            : undefined,
      due_date: dueDate
        ? `${dueDate}T00:00:00Z`
        : includeNulls
          ? null
          : undefined,
    };
  };

  const validateInvoiceTermsInput = (
    termsDaysValue: string,
    invoiceReceivedDate: string,
    requireDueDate = false,
  ) => {
    const normalizedInvoiceReceivedDate =
      normalizeDateInputValue(invoiceReceivedDate);

    if (
      requireDueDate &&
      (!normalizedInvoiceReceivedDate || termsDaysValue.trim() === "")
    ) {
      return "Isi tanggal terima invoice vendor dan termin pembayaran agar due date bisa dihitung.";
    }

    if (termsDaysValue.trim() === "") return null;

    const termsDays = Number(termsDaysValue);
    if (!Number.isInteger(termsDays) || termsDays < 0) {
      return "Termin pembayaran harus berupa angka hari 0 atau lebih.";
    }
    if (termsDays > 3650) {
      return "Termin pembayaran tidak boleh lebih dari 3650 hari.";
    }
    if (invoiceReceivedDate && !normalizedInvoiceReceivedDate) {
      return "Tanggal terima invoice vendor belum valid.";
    }
    if (
      requireDueDate &&
      !getCalculatedDueDateInput(normalizedInvoiceReceivedDate, termsDaysValue)
    ) {
      return "Due date belum bisa dihitung dari tanggal terima invoice dan termin.";
    }
    if (
      normalizedInvoiceReceivedDate &&
      !getCalculatedDueDateInput(normalizedInvoiceReceivedDate, termsDaysValue)
    ) {
      return "Due date belum bisa dihitung dari tanggal terima invoice dan termin.";
    }

    return null;
  };

  const handleVendorChange = (nextVendorId: string) => {
    setVendorId(nextVendorId);

    const selectedVendor = vendors.find(
      (vendor) => vendor.id.toString() === nextVendorId,
    );
    if (selectedVendor?.payment_terms !== undefined) {
      setPaymentTermsDays(selectedVendor.payment_terms.toString());
    }
  };

  const handleOpenCreate = () => {
    setTitle("");
    setDepartment(userDepartment);
    setProjectId("");
    setVendorId("");
    setVendorInvoiceReceivedDate("");
    setPaymentTermsDays("30");
    setNotes("");
    setDocuments([]);
    setItems([createEmptyPurchaseRequestItem()]);
    onCreateOpen();
  };

  const handleAddItem = () => {
    setItems([...items, createEmptyPurchaseRequestItem()]);
  };

  const handleRemoveItem = (index: number) => {
    const newItems = items.filter((_, i) => i !== index);
    setItems(
      newItems.length
        ? newItems
        : [createEmptyPurchaseRequestItem()],
    );
  };

  const handleItemSourceChange = (
    index: number,
    source: PurchaseRequestFormItem["source"],
  ) => {
    setItems((currentItems) =>
      currentItems.map((item, itemIndex) =>
        itemIndex === index
          ? {
              ...item,
              source,
              product_id: "",
              item_name: "",
              estimated_price: 0,
            }
          : item,
      ),
    );
  };

  const handleItemChange = (
    index: number,
    field: string,
    value: string | number,
  ) => {
    const newItems = [...items];
    if (field === "product_id") {
      const selectedProd = products.find(
        (p) => p.id.toString() === value.toString(),
      );
      if (selectedProd) {
        newItems[index].estimated_price = selectedProd.purchase_price || 0;
      }
    }
    newItems[index] = { ...newItems[index], [field]: value } as any;
    setItems(newItems);
  };

  const validateDocument = (file: File) => {
    const allowedExtensions = ACCEPTED_DOCUMENT_TYPES.split(",");
    const extension = `.${file.name.split(".").pop()?.toLowerCase() || ""}`;

    if (!allowedExtensions.includes(extension)) {
      return "Format dokumen belum didukung. Gunakan PDF, Word, Excel, CSV, TXT, JPG, PNG, atau WebP.";
    }

    if (file.size > MAX_DOCUMENT_SIZE) {
      return `Ukuran dokumen terlalu besar. Maksimal ${MAX_DOCUMENT_SIZE_MB}MB per file.`;
    }

    return null;
  };

  const getDocumentUploadErrorMessage = (err: unknown) => {
    const uploadError = err as {
      code?: string;
      message?: string;
      response?: {
        status?: number;
        data?: { error?: string; message?: string; details?: string };
      };
    };
    const rawMessage =
      uploadError.response?.data?.error ||
      uploadError.response?.data?.message ||
      uploadError.response?.data?.details ||
      uploadError.message ||
      "";
    const normalizedMessage = String(rawMessage).toLowerCase();
    const status = uploadError.response?.status;

    if (
      uploadError.code === "ECONNABORTED" ||
      normalizedMessage.includes("timeout") ||
      normalizedMessage.includes("exceeded")
    ) {
      return DOCUMENT_UPLOAD_TIMEOUT_MESSAGE;
    }

    if (
      status === 413 ||
      normalizedMessage.includes("too large") ||
      normalizedMessage.includes("maksimal") ||
      normalizedMessage.includes("maximum") ||
      normalizedMessage.includes("request entity too large")
    ) {
      return `Ukuran dokumen terlalu besar. Maksimal ${MAX_DOCUMENT_SIZE_MB}MB per file.`;
    }

    if (
      normalizedMessage.includes("invalid file type") ||
      normalizedMessage.includes("format dokumen")
    ) {
      return "Format dokumen belum didukung. Gunakan PDF, Word, Excel, CSV, TXT, JPG, PNG, atau WebP.";
    }

    if (
      normalizedMessage.includes("file content does not match") ||
      normalizedMessage.includes("isi dokumen")
    ) {
      return "Isi dokumen tidak sesuai dengan format file. Pastikan file tidak rusak dan ekstensi file benar.";
    }

    if (
      normalizedMessage.includes("permission denied") ||
      normalizedMessage.includes("failed to save") ||
      normalizedMessage.includes("folder upload")
    ) {
      return "Dokumen belum bisa disimpan di server. Hubungi admin untuk mengecek folder upload.";
    }

    return rawMessage || "Dokumen belum bisa diproses. Coba beberapa saat lagi.";
  };

  const handleDocumentChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const selectedFiles = Array.from(event.target.files || []);
    if (selectedFiles.length === 0) return;

    const invalidFile = selectedFiles.find((file) => validateDocument(file));
    if (invalidFile) {
      toast({
        title: "Dokumen tidak bisa diunggah",
        description: validateDocument(invalidFile) || undefined,
        status: "warning",
        duration: 4000,
      });
      event.target.value = "";
      return;
    }

    setDocuments((prev) => {
      const existing = new Set(prev.map((file) => `${file.name}-${file.size}`));
      const nextFiles = selectedFiles.filter(
        (file) => !existing.has(`${file.name}-${file.size}`),
      );
      return [...prev, ...nextFiles];
    });
    event.target.value = "";
  };

  const handleRemoveDocument = (index: number) => {
    setDocuments((prev) => prev.filter((_, itemIndex) => itemIndex !== index));
  };

  const handleExistingDocumentChange = (
    event: React.ChangeEvent<HTMLInputElement>,
  ) => {
    if (!selectedRequest) return;

    const selectedFiles = Array.from(event.target.files || []);
    event.target.value = "";
    if (selectedFiles.length === 0) return;

    const invalidFile = selectedFiles.find((file) => validateDocument(file));
    if (invalidFile) {
      toast({
        title: "Dokumen tidak bisa diunggah",
        description: validateDocument(invalidFile) || undefined,
        status: "warning",
        duration: 4000,
      });
      return;
    }

    setPendingAdditionalDocuments((currentFiles) => {
      const existing = new Set(
        currentFiles.map((file) => `${file.name}-${file.size}`),
      );
      const uniqueFiles = selectedFiles.filter(
        (file) => !existing.has(`${file.name}-${file.size}`),
      );
      return [...currentFiles, ...uniqueFiles];
    });
  };

  const handleRemovePendingAdditionalDocument = (index: number) => {
    setPendingAdditionalDocuments((files) =>
      files.filter((_, fileIndex) => fileIndex !== index),
    );
  };

  const handleRealizationDocumentChange = (
    event: React.ChangeEvent<HTMLInputElement>,
  ) => {
    if (!selectedRequest) return;

    const selectedFiles = Array.from(event.target.files || []);
    event.target.value = "";
    if (selectedFiles.length === 0) return;

    const invalidFile = selectedFiles.find((file) => validateDocument(file));
    if (invalidFile) {
      toast({
        title: "Dokumen realisasi tidak bisa diunggah",
        description: validateDocument(invalidFile) || undefined,
        status: "warning",
        duration: 4000,
      });
      return;
    }

    setPendingRealizationDocuments((currentFiles) => {
      const existing = new Set(
        currentFiles.map((file) => `${file.name}-${file.size}`),
      );
      const uniqueFiles = selectedFiles.filter(
        (file) => !existing.has(`${file.name}-${file.size}`),
      );
      return [...currentFiles, ...uniqueFiles];
    });
  };

  const handleRemovePendingRealizationDocument = (index: number) => {
    setPendingRealizationDocuments((files) =>
      files.filter((_, fileIndex) => fileIndex !== index),
    );
  };

  const buildInvoiceRevisionItems = (
    pr?: PurchaseRequest | null,
  ): InvoiceRevisionFormItem[] =>
    (pr?.items || []).map((item) => ({
      item_id: item.id,
      item_name: item.product?.name || item.item_name || `Item #${item.id}`,
      quantity: item.quantity,
      estimated_price: item.estimated_price,
      notes: item.notes || "",
    }));

  const buildRequesterRevisionItems = (
    pr?: PurchaseRequest | null,
  ): RequesterRevisionFormItem[] =>
    (pr?.items || []).map((item) => ({
      client_id: `existing-${item.id}`,
      item_id: item.id,
      source: item.product_id ? "catalog" : "manual",
      product_id: item.product_id?.toString() || "",
      item_name: item.product?.name || item.item_name || `Item #${item.id}`,
      quantity: item.quantity,
      estimated_price: item.estimated_price,
      notes: item.notes || "",
    }));

  const handleInvoiceRevisionDocumentChange = (
    event: React.ChangeEvent<HTMLInputElement>,
  ) => {
    const selectedFiles = Array.from(event.target.files || []);
    event.target.value = "";
    if (selectedFiles.length === 0) return;

    const invalidFile = selectedFiles.find((file) => validateDocument(file));
    if (invalidFile) {
      toast({
        title: "Invoice tidak bisa diunggah",
        description: validateDocument(invalidFile) || undefined,
        status: "warning",
        duration: 4000,
      });
      return;
    }

    setInvoiceRevisionDocuments((currentFiles) => {
      const existing = new Set(
        currentFiles.map((file) => `${file.name}-${file.size}`),
      );
      const uniqueFiles = selectedFiles.filter(
        (file) => !existing.has(`${file.name}-${file.size}`),
      );
      return [...currentFiles, ...uniqueFiles];
    });
  };

  const handleRemoveInvoiceRevisionDocument = (index: number) => {
    setInvoiceRevisionDocuments((files) =>
      files.filter((_, fileIndex) => fileIndex !== index),
    );
  };

  const handleInvoiceRevisionItemChange = (
    itemId: number,
    field: "quantity" | "estimated_price" | "notes",
    value: string,
  ) => {
    setInvoiceRevisionItems((currentItems) =>
      currentItems.map((item) =>
        item.item_id === itemId ? { ...item, [field]: value } : item,
      ),
    );
  };

  const handleRequesterRevisionDocumentChange = (
    event: React.ChangeEvent<HTMLInputElement>,
  ) => {
    const selectedFiles = Array.from(event.target.files || []);
    event.target.value = "";
    if (selectedFiles.length === 0) return;

    const invalidFile = selectedFiles.find((file) => validateDocument(file));
    if (invalidFile) {
      toast({
        title: "Dokumen revisi tidak bisa diunggah",
        description: validateDocument(invalidFile) || undefined,
        status: "warning",
        duration: 4000,
      });
      return;
    }

    setRequesterRevisionDocuments((currentFiles) => {
      const existing = new Set(
        currentFiles.map((file) => `${file.name}-${file.size}`),
      );
      const uniqueFiles = selectedFiles.filter(
        (file) => !existing.has(`${file.name}-${file.size}`),
      );
      return [...currentFiles, ...uniqueFiles];
    });
  };

  const handleRemoveRequesterRevisionDocument = (index: number) => {
    setRequesterRevisionDocuments((files) =>
      files.filter((_, fileIndex) => fileIndex !== index),
    );
  };

  const handleAddRequesterRevisionItem = () => {
    setRequesterRevisionItems((currentItems) => [
      ...currentItems,
      createEmptyRequesterRevisionItem(),
    ]);
  };

  const handleRemoveRequesterRevisionItem = (clientID: string) => {
    if (requesterRevisionItems.length <= 1) {
      toast({
        title: "PR harus memiliki minimal satu item",
        description: "Tambahkan item lain sebelum menghapus baris terakhir.",
        status: "warning",
        duration: 4000,
      });
      return;
    }
    setRequesterRevisionItems((currentItems) =>
      currentItems.filter((item) => item.client_id !== clientID),
    );
  };

  const handleRequesterRevisionItemSourceChange = (
    clientID: string,
    source: RequesterRevisionFormItem["source"],
  ) => {
    setRequesterRevisionItems((currentItems) =>
      currentItems.map((item) =>
        item.client_id === clientID && !item.item_id
          ? {
              ...item,
              source,
              product_id: "",
              item_name: "",
              estimated_price: 0,
            }
          : item,
      ),
    );
  };

  const handleRequesterRevisionItemChange = (
    clientID: string,
    field:
      | "product_id"
      | "item_name"
      | "quantity"
      | "estimated_price"
      | "notes",
    value: string | number,
  ) => {
    setRequesterRevisionItems((currentItems) =>
      currentItems.map((item) => {
        if (item.client_id !== clientID) return item;

        if (field === "product_id") {
          const selectedProduct = products.find(
            (product) => product.id?.toString() === value.toString(),
          );
          return {
            ...item,
            product_id: value.toString(),
            estimated_price: selectedProduct?.purchase_price || 0,
          };
        }

        return { ...item, [field]: value };
      }),
    );
  };

  const handleSubmitAdditionalDocuments = async () => {
    if (!selectedRequest || pendingAdditionalDocuments.length === 0) return;

    try {
      setIsUploadingDocuments(true);
      const result = await purchaseRequestService.uploadDocuments(
        selectedRequest.id,
        pendingAdditionalDocuments,
      );
      const refreshed = await purchaseRequestService.getById(selectedRequest.id);
      setSelectedRequest(refreshed);
      setProjectUpdateId(refreshed.project_id?.toString() || "");
      initializeReviewForm(refreshed);
      setPendingAdditionalDocuments([]);
      await fetchData();
      toast({
        title: result.reapproval_started
          ? "Dokumen dikirim untuk approval ulang"
          : "Dokumen berhasil dikirim",
        description: result.reapproval_started
          ? "Approval PR dimulai kembali dari Finance Staff. Finance Manager akan menerima notifikasi setelah review staff selesai."
          : "Dokumen baru sudah dikirim dan reviewer terkait telah diberi notifikasi.",
        status: "success",
        duration: 5000,
      });
    } catch (err: any) {
      toast({
        title: "Gagal mengunggah dokumen",
        description: getDocumentUploadErrorMessage(err),
        status: "error",
        duration: 4000,
      });
    } finally {
      setIsUploadingDocuments(false);
    }
  };

  const handleUploadRealizationDocuments = async () => {
    if (!selectedRequest || pendingRealizationDocuments.length === 0) return;

    try {
      setIsUploadingRealizationDocuments(true);
      await purchaseRequestService.uploadRealizationDocuments(
        selectedRequest.id,
        pendingRealizationDocuments,
      );
      const refreshed = await purchaseRequestService.getById(selectedRequest.id);
      setSelectedRequest(refreshed);
      setProjectUpdateId(refreshed.project_id?.toString() || "");
      initializeReviewForm(refreshed);
      setPendingRealizationDocuments([]);
      await fetchData();
      toast({
        title: "Dokumen realisasi berhasil diunggah",
        description:
          "Dokumen tersimpan sebagai bukti billing/realisasi PIC tanpa memulai approval ulang.",
        status: "success",
        duration: 4500,
      });
    } catch (err: any) {
      toast({
        title: "Gagal upload dokumen realisasi",
        description: getDocumentUploadErrorMessage(err),
        status: "error",
        duration: 5000,
      });
    } finally {
      setIsUploadingRealizationDocuments(false);
    }
  };

  const handleSubmitInvoiceRevision = async () => {
    if (!selectedRequest) return;

    if (invoiceRevisionDocuments.length === 0) {
      return toast({
        title: "Upload invoice aktual dulu",
        description:
          "Lampirkan invoice vendor sebelum mengirim revisi nominal PR.",
        status: "warning",
        duration: 4000,
      });
    }

    const invalidItemIndex = invoiceRevisionItems.findIndex(
      (item) => Number(item.quantity) < 1 || Number(item.estimated_price) < 0,
    );
    if (invalidItemIndex >= 0) {
      return toast({
        title: `Nominal invoice baris ${invalidItemIndex + 1} belum valid`,
        description: "Pastikan qty minimal 1 dan harga invoice tidak negatif.",
        status: "warning",
        duration: 4000,
      });
    }

    try {
      setIsSubmittingInvoiceRevision(true);
      const updated = await purchaseRequestService.submitInvoiceRevision(
        selectedRequest.id,
        {
          notes: invoiceRevisionNotes.trim() || undefined,
          items: invoiceRevisionItems.map((item) => ({
            item_id: item.item_id,
            quantity: Number(item.quantity),
            estimated_price: Number(item.estimated_price),
            notes: item.notes,
          })),
        },
        invoiceRevisionDocuments,
      );
      setSelectedRequest(updated);
      setInvoiceRevisionItems(buildInvoiceRevisionItems(updated));
      setInvoiceRevisionDocuments([]);
      setInvoiceRevisionNotes("");
      initializeReviewForm(updated);
      await fetchData();
      toast({
        title: "Invoice aktual dikirim untuk approval ulang",
        description:
          "PR kembali ke Finance Staff untuk review nominal invoice.",
        status: "success",
        duration: 5000,
      });
    } catch (err: any) {
      toast({
        title: "Gagal mengirim invoice aktual",
        description: getDocumentUploadErrorMessage(err),
        status: "error",
        duration: 5000,
      });
    } finally {
      setIsSubmittingInvoiceRevision(false);
    }
  };

  const handleOpenRequesterRevisionRequest = () => {
    setRequesterRevisionOpenReason("");
    onRequesterRevisionOpen();
  };

  const handleOpenRequesterRevision = async () => {
    if (!selectedRequest) return;

    const reason = requesterRevisionOpenReason.trim();
    if (!reason) {
      return toast({
        title: "Alasan buka revisi wajib diisi",
        status: "warning",
        duration: 3000,
      });
    }

    try {
      setIsOpeningRequesterRevision(true);
      const updated = await purchaseRequestService.openRequesterRevision(
        selectedRequest.id,
        { reason },
      );
      setSelectedRequest(updated);
      setRequesterRevisionItems(buildRequesterRevisionItems(updated));
      setRequesterRevisionDocuments([]);
      setRequesterRevisionNotes("");
      setRequesterRevisionOpenReason("");
      await fetchData();
      onRequesterRevisionClose();
      const isInitialRevision = isInitialRequesterRevisionRequest(selectedRequest);
      toast({
        title: isInitialRevision
          ? "Perbaikan PR diminta ke PIC"
          : "PR dibuka untuk revisi PIC",
        description: isInitialRevision
          ? "Belum ada approval yang dikirim. PIC dapat memperbaiki daftar item dan nominal; dokumen hanya ditambahkan bila diperlukan."
          : "PIC dapat mengubah daftar item dan nominal; dokumen hanya ditambahkan bila diperlukan.",
        status: "success",
        duration: 4000,
      });
    } catch (err: any) {
      toast({
        title: "Gagal membuka revisi PR",
        description: err.response?.data?.error || err.message,
        status: "error",
        duration: 5000,
      });
    } finally {
      setIsOpeningRequesterRevision(false);
    }
  };

  const handleSubmitRequesterRevision = async () => {
    if (!selectedRequest) return;

    if (!requesterRevisionNotes.trim()) {
      return toast({
        title: "Catatan revisi wajib diisi",
        description:
          "Jelaskan alasan perubahan agar riwayat revisi tetap dapat diaudit.",
        status: "warning",
        duration: 4000,
      });
    }

    const invalidItemIndex = requesterRevisionItems.findIndex(
      (item) =>
        Number(item.quantity) < 1 ||
        Number(item.estimated_price) < 0 ||
        (!item.item_id &&
          ((item.source === "catalog" && !item.product_id) ||
            (item.source === "manual" && !item.item_name.trim()))),
    );
    if (invalidItemIndex >= 0) {
      return toast({
        title: `Nominal revisi baris ${invalidItemIndex + 1} belum valid`,
        description:
          "Pastikan item baru sudah dipilih/diisi, qty minimal 1, dan harga tidak negatif.",
        status: "warning",
        duration: 4000,
      });
    }

    try {
      setIsSubmittingRequesterRevision(true);
      const updated = await purchaseRequestService.submitRequesterRevision(
        selectedRequest.id,
        {
          notes: requesterRevisionNotes.trim() || undefined,
          items: requesterRevisionItems.map((item) => ({
            ...(item.item_id ? { item_id: item.item_id } : {}),
            ...(!item.item_id && item.source === "catalog" && item.product_id
              ? { product_id: Number(item.product_id) }
              : {}),
            ...(!item.item_id && item.source === "manual"
              ? { item_name: item.item_name.trim() }
              : {}),
            quantity: Number(item.quantity),
            estimated_price: Number(item.estimated_price),
            notes: item.notes,
          })),
        },
        requesterRevisionDocuments,
      );
      setSelectedRequest(updated);
      setRequesterRevisionItems(buildRequesterRevisionItems(updated));
      setRequesterRevisionDocuments([]);
      setRequesterRevisionNotes("");
      initializeReviewForm(updated);
      await fetchData();
      toast({
        title: "Revisi PR dikirim untuk approval ulang",
        description: "PR kembali ke Finance Staff dan Finance Manager.",
        status: "success",
        duration: 5000,
      });
    } catch (err: any) {
      toast({
        title: "Gagal mengirim revisi PR",
        description: getDocumentUploadErrorMessage(err),
        status: "error",
        duration: 5000,
      });
    } finally {
      setIsSubmittingRequesterRevision(false);
    }
  };

  const formatFileSize = (size?: number) => {
    const bytes = Number(size || 0);
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  const handleDownloadDocument = async (document: PurchaseRequestDocument) => {
    try {
      await purchaseRequestService.downloadDocument(document.id, document.file_name);
    } catch (err: unknown) {
      const error = err as {
        response?: { data?: { error?: string } };
        message?: string;
      };
      toast({
        title: "Gagal membuka dokumen",
        description:
          error.response?.data?.error ||
          error.message ||
          "Dokumen belum bisa diunduh.",
        status: "error",
        duration: 4000,
        isClosable: true,
      });
    }
  };

  const handleDownloadPaymentReceipt = async (
    receipt: FinalizedPaymentReceipt,
  ) => {
    if (receipt.hasPaymentProof) {
      try {
        await purchaseService.downloadPaymentProof(
          receipt.purchasePaymentId,
          receipt.paymentProofFileName || `${receipt.paymentCode}-bukti-bank`,
          true,
        );
      } catch (err: unknown) {
        const error = err as {
          response?: { data?: { error?: string } };
          message?: string;
        };
        toast({
          title: "Gagal membuka bukti pembayaran bank",
          description:
            error.response?.data?.error ||
            error.message ||
            "Bukti pembayaran bank belum bisa diunduh.",
          status: "error",
          duration: 4000,
        });
      }
      return;
    }

    if (!receipt.paymentId) {
      toast({
        title: "Bukti pembayaran belum diunggah",
        description:
          "Pembayaran sudah tercatat, tetapi file bukti pembayaran bank belum tersedia.",
        status: "warning",
        duration: 3500,
      });
      return;
    }

    try {
      await paymentService.downloadPaymentDetailPDF(
        receipt.paymentId,
        receipt.paymentCode,
      );
    } catch (err: unknown) {
      const error = err as {
        response?: { data?: { error?: string } };
        message?: string;
      };
      toast({
        title: "Gagal membuka bukti pembayaran",
        description:
          error.response?.data?.error ||
          error.message ||
          "Receipt pembayaran belum bisa diunduh.",
        status: "error",
        duration: 4000,
      });
    }
  };

  const handleCreateSubmit = async () => {
    const invalidItemIndex = items.findIndex((item) => {
      const hasItemIdentity =
        item.source === "catalog"
          ? Boolean(item.product_id)
          : Boolean(item.item_name.trim());
      return (
        !hasItemIdentity ||
        Number(item.quantity) < 1 ||
        Number(item.estimated_price) < 0
      );
    });

    if (invalidItemIndex >= 0) {
      return toast({
        title: `Lengkapi kebutuhan baris ${invalidItemIndex + 1}`,
        description:
          "Pilih produk atau isi nama kebutuhan manual, lalu pastikan qty dan estimasi harganya valid.",
        status: "warning",
        duration: 4000,
      });
    }

    const invoiceTermsError = validateInvoiceTermsInput(
      paymentTermsDays,
      vendorInvoiceReceivedDate,
    );
    if (invoiceTermsError) {
      return toast({
        title: "Tanggal invoice vendor belum valid",
        description: invoiceTermsError,
        status: "warning",
        duration: 4000,
      });
    }

    try {
      setIsSubmitting(true);
      const invoiceTermsPayload = buildInvoiceTermsPayload(
        vendorInvoiceReceivedDate,
        paymentTermsDays,
      );
      const payload: PurchaseRequestCreatePayload = {
        title: title.trim() || undefined,
        department,
        project_id: projectId ? parseInt(projectId) : undefined,
        vendor_id: vendorId ? parseInt(vendorId) : undefined,
        ...invoiceTermsPayload,
        notes,
        items: items.map((item) => ({
          product_id:
            item.source === "catalog"
              ? parseInt(item.product_id, 10)
              : undefined,
          item_name:
            item.source === "manual" ? item.item_name.trim() : undefined,
          quantity: Number(item.quantity),
          estimated_price: Number(item.estimated_price),
          notes: item.notes,
        })),
      };

      await purchaseRequestService.create(payload, documents);
      toast({
        title: "Pengajuan berhasil dikirim",
        status: "success",
        duration: 3000,
      });
      onCreateClose();
      fetchData();
    } catch (err: any) {
      toast({
        title: "Gagal membuat pengajuan",
        description: getDocumentUploadErrorMessage(err),
        status: "error",
        duration: 4000,
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  const getCurrentReviewStage = (pr?: PurchaseRequest | null) => {
    if (!pr || pr.status !== "PENDING_APPROVAL") return null;
    return pr.current_approval_stage || "FINANCE_STAFF";
  };

  const isInitialRequesterRevisionRequest = (
    pr?: PurchaseRequest | null,
  ) =>
    pr?.status === "PENDING_APPROVAL" &&
    getCurrentReviewStage(pr) === "FINANCE_STAFF";

  const getStageLabel = (pr?: PurchaseRequest | null) => {
    const stage = getCurrentReviewStage(pr);
    if (stage === "DIRECTOR") return "Menunggu Director";
    if (stage === "FINANCE_MANAGER") return "Menunggu Finance Manager";
    if (stage === "FINANCE_STAFF") return "Menunggu Finance Staff/Accounting";
    return null;
  };

  const canCurrentUserReviewRequest = (pr?: PurchaseRequest | null) => {
    const stage = getCurrentReviewStage(pr);
    if (!stage || !canApprovePurchaseRequest) return false;
    if (stage === "FINANCE_STAFF")
      return isFinanceStaffRole(roleNorm) || isAccountingRole(roleNorm);
    if (stage === "FINANCE_MANAGER") return isFinanceManagerRole(roleNorm);
    if (stage === "DIRECTOR") return roleNorm === "director";
    return false;
  };

  const canRequestDirectorApproval = (pr?: PurchaseRequest | null) => {
    const stage = getCurrentReviewStage(pr);
    if (stage !== "FINANCE_MANAGER") return false;
    return isFinanceManagerRole(roleNorm);
  };

  const getPurchaseRequestTitle = (pr?: PurchaseRequest | null) => {
    const explicitTitle = pr?.title?.trim();
    if (explicitTitle) return explicitTitle;

    const firstItem = pr?.items?.[0];
    const firstItemName = firstItem?.product?.name || firstItem?.item_name;
    if (firstItemName?.trim()) return firstItemName.trim();

    return pr?.notes?.trim() || "-";
  };

  const getInvoiceTermsDaysInputValue = (pr?: PurchaseRequest | null) => {
    if (pr?.payment_terms_days != null) {
      return pr.payment_terms_days.toString();
    }
    if (pr?.vendor?.payment_terms != null) {
      return pr.vendor.payment_terms.toString();
    }
    return "";
  };

  const normalizePurchasePaymentMethod = (paymentMethod?: string | null) =>
    (paymentMethod || "CREDIT").trim().toUpperCase();

  const purchaseRequiresDueDate = (
    purchase?: PurchaseRequestItem["purchase"] | null,
  ) => normalizePurchasePaymentMethod(purchase?.payment_method) === "CREDIT";

  const formatDisplayDate = (value?: string | null) => {
    if (!value || value.startsWith("0001-01-01")) return "";

    const parsedDate = new Date(value);
    if (Number.isNaN(parsedDate.getTime())) return "";

    return parsedDate.toLocaleDateString("id-ID");
  };

  const getPurchaseDueDateSummary = (pr?: PurchaseRequest | null) => {
    const purchaseRequestDueDate = formatDisplayDate(pr?.due_date);
    const purchaseByID = new Map<
      number,
      NonNullable<PurchaseRequestItem["purchase"]>
    >();
    const purchaseIDs = new Set<number>();

    (pr?.items || []).forEach((item) => {
      if (item.purchase?.id) {
        purchaseByID.set(item.purchase.id, item.purchase);
        purchaseIDs.add(item.purchase.id);
      } else if (item.purchase_id) {
        purchaseIDs.add(item.purchase_id);
      }
    });

    const dueDateEntries = Array.from(purchaseByID.values())
      .map((purchase) => ({
        purchaseId: purchase.id,
        purchaseCode: purchase.code,
        dueDate: purchase.due_date,
        displayDate: formatDisplayDate(purchase.due_date),
      }))
      .filter((entry) => Boolean(entry.displayDate))
      .sort(
        (a, b) =>
          new Date(a.dueDate || "").getTime() -
          new Date(b.dueDate || "").getTime(),
      );
    const linkedPurchases = Array.from(purchaseByID.values());
    const hasUnknownPurchaseMethod =
      purchaseIDs.size > 0 && linkedPurchases.length === 0;
    const requiresDueDate =
      hasUnknownPurchaseMethod || linkedPurchases.some(purchaseRequiresDueDate);

    return {
      purchaseRequestDueDate,
      hasPurchase: purchaseIDs.size > 0,
      requiresDueDate,
      dueDateEntries,
      missingDueDateCount: Math.max(0, purchaseIDs.size - dueDateEntries.length),
    };
  };

  const renderPurchaseDueDate = (
    pr?: PurchaseRequest | null,
    compact = false,
  ) => {
    const summary = getPurchaseDueDateSummary(pr);

    if (summary.hasPurchase && !summary.requiresDueDate) {
      return (
        <Badge colorScheme="gray" variant="subtle">
          Tidak diperlukan
        </Badge>
      );
    }

    if (summary.purchaseRequestDueDate) {
      return (
        <Flex direction="column" gap={1} align="flex-start">
          <Text fontSize="sm" fontWeight="medium" whiteSpace="nowrap">
            {summary.purchaseRequestDueDate}
          </Text>
          {!compact && (
            <Text fontSize="xs" color={mutedTextColor}>
              Dari PR
            </Text>
          )}
        </Flex>
      );
    }

    if (!summary.hasPurchase) {
      return <Badge colorScheme="gray">Belum ada due date</Badge>;
    }

    if (summary.dueDateEntries.length === 0) {
      if (
        summary.requiresDueDate &&
        pr?.payment_terms_days != null &&
        !pr.vendor_invoice_received_date
      ) {
        return (
          <Badge colorScheme="yellow" variant="subtle">
            {compact ? "Butuh tgl invoice" : "Belum ada tanggal invoice"}
          </Badge>
        );
      }

      return (
        <Badge colorScheme="yellow" variant="subtle">
          Belum ada due date
        </Badge>
      );
    }

    const firstDueDate = summary.dueDateEntries[0];
    const tooltipLabel = summary.dueDateEntries
      .map(
        (entry) =>
          `${entry.purchaseCode || `Purchase #${entry.purchaseId}`}: ${
            entry.displayDate
          }`,
      )
      .join(", ");

    return (
      <Flex direction="column" gap={1} align="flex-start">
        <Tooltip label={tooltipLabel} placement="top" hasArrow>
          <Text fontSize="sm" fontWeight="medium" whiteSpace="nowrap">
            {firstDueDate.displayDate}
          </Text>
        </Tooltip>
        {!compact && firstDueDate.purchaseCode && (
          <Text fontSize="xs" color={mutedTextColor}>
            {firstDueDate.purchaseCode}
          </Text>
        )}
        {summary.dueDateEntries.length > 1 && (
          <Badge colorScheme="blue" variant="subtle">
            +{summary.dueDateEntries.length - 1} due date
          </Badge>
        )}
        {!compact && summary.missingDueDateCount > 0 && (
          <Badge colorScheme="yellow" variant="subtle">
            {summary.missingDueDateCount} belum ada due date
          </Badge>
        )}
      </Flex>
    );
  };

  const getPurchaseRealizationSummary = (pr?: PurchaseRequest | null) => {
    const purchaseByID = new Map<
      number,
      NonNullable<PurchaseRequestItem["purchase"]>
    >();
    const purchaseIDs = new Set<number>();

    (pr?.items || []).forEach((item) => {
      if (item.purchase?.id) {
        purchaseByID.set(item.purchase.id, item.purchase);
        purchaseIDs.add(item.purchase.id);
      } else if (item.purchase_id) {
        purchaseIDs.add(item.purchase_id);
      }
    });

    const purchases = Array.from(purchaseByID.values());
    const totalPaid = purchases.reduce(
      (sum, purchase) => sum + Number(purchase.paid_amount || 0),
      0,
    );
    const totalOutstanding = purchases.reduce(
      (sum, purchase) => sum + Number(purchase.outstanding_amount || 0),
      0,
    );
    const totalAmount = purchases.reduce(
      (sum, purchase) => sum + Number(purchase.total_amount || 0),
      0,
    );
    const finalizedReceiptByKey = new Map<string, FinalizedPaymentReceipt>();

    purchases.forEach((purchase) => {
      (purchase.purchase_payments || []).forEach((purchasePayment) => {
        const payment = purchasePayment.payment;
        const proofReviewStatus = (
          purchasePayment.payment_proof_review_status || "PENDING"
        ).toUpperCase();
        const hasUploadedPaymentProof = Boolean(
          purchasePayment.payment_proof_file_path,
        );
        const hasApprovedPaymentProof =
          hasUploadedPaymentProof && proofReviewStatus === "APPROVED";
        const hasFinalPaymentRecord =
          Boolean(payment?.id) && payment?.status?.toUpperCase() === "COMPLETED";
        const hasDirectPurchasePayment =
          !payment?.id && Number(purchasePayment.amount || 0) > 0;
        if (!hasFinalPaymentRecord && !hasDirectPurchasePayment) return;

        const key = payment?.id
          ? `payment-${payment.id}`
          : `purchase-payment-${purchasePayment.id}`;
        if (finalizedReceiptByKey.has(key)) return;

        finalizedReceiptByKey.set(key, {
          purchaseId: purchase.id,
          purchaseCode: purchase.code,
          purchasePaymentId: purchasePayment.id,
          paymentId: payment?.id,
          paymentCode:
            payment?.code ||
            purchasePayment.payment_number ||
            `${purchase.code}-PAY`,
          hasPaymentProof: hasApprovedPaymentProof,
          hasUploadedPaymentProof,
          paymentProofFileName: purchasePayment.payment_proof_file_name,
          paymentProofReviewStatus: proofReviewStatus,
          amount: Number(payment?.amount || purchasePayment.amount || 0),
          date: payment?.date || purchasePayment.date,
          method: payment?.method || purchasePayment.method,
          reference: payment?.reference || purchasePayment.reference,
        });
      });
    });
    const finalizedReceipts = Array.from(finalizedReceiptByKey.values());
    const paymentProofReceipts = finalizedReceipts.filter((receipt) =>
      Boolean(receipt.hasPaymentProof),
    );
    const pendingPaymentProofCount = finalizedReceipts.filter(
      (receipt) =>
        receipt.hasUploadedPaymentProof &&
        receipt.paymentProofReviewStatus === "PENDING",
    ).length;
    const rejectedPaymentProofCount = finalizedReceipts.filter(
      (receipt) =>
        receipt.hasUploadedPaymentProof &&
        receipt.paymentProofReviewStatus === "REJECTED",
    ).length;
    const paymentWithoutProofCount =
      finalizedReceipts.filter((receipt) => !receipt.hasUploadedPaymentProof)
        .length;

    return {
      hasPurchase: purchaseIDs.size > 0,
      hasPayment:
        totalPaid > 0 ||
        purchases.some((purchase) => ["PAID", "COMPLETED"].includes(purchase.status)),
      hasPaymentProof: paymentProofReceipts.length > 0,
      paymentProofReceipts,
      paymentWithoutProofCount,
      pendingPaymentProofCount,
      rejectedPaymentProofCount,
      purchaseCount: purchaseIDs.size,
      totalPaid,
      totalOutstanding,
      totalAmount,
      purchaseCodes: purchases.map((purchase) => purchase.code).filter(Boolean),
    };
  };

  const canCancelPurchaseRequest = (pr?: PurchaseRequest | null) => {
    if (!pr || !canCancelPurchaseRequests) return false;
    if (pr.status !== "APPROVED") return false;
    return !getPurchaseRealizationSummary(pr).hasPurchase;
  };

  const financeManagerReviewStarted = (pr?: PurchaseRequest | null) => {
    if (!pr) return false;
    if (
      pr.finance_manager_approved_by ||
      pr.finance_manager_approved_at ||
      pr.finance_manager_review_notes?.trim()
    ) {
      return true;
    }

    return (pr.items || []).some((item) => {
      const decision = (item.finance_manager_decision || "")
        .trim()
        .toUpperCase();
      return decision !== "" && decision !== "PENDING";
    });
  };

  const canOpenRequesterRevision = (pr?: PurchaseRequest | null) => {
    if (!pr || !canApprove("purchase_requests")) return false;
    if (pr.status !== "APPROVED" && pr.status !== "PENDING_APPROVAL") {
      return false;
    }
    if (getPurchaseRealizationSummary(pr).hasPurchase) return false;
    if (roleNorm === "admin") return true;
    if (!isFinanceTeamRole(roleNorm)) return false;
    if (pr.status === "APPROVED") return true;

    const stage = getCurrentReviewStage(pr);
    if (stage === "FINANCE_STAFF") {
      return isFinanceStaffRole(roleNorm) || isAccountingRole(roleNorm);
    }
    if (stage === "FINANCE_MANAGER") {
      return (
        isFinanceManagerRole(roleNorm) ||
        (isFinanceStaffRole(roleNorm) && !financeManagerReviewStarted(pr))
      );
    }
    return false;
  };

  const renderPaymentProofRealization = (
    pr?: PurchaseRequest | null,
    compact = false,
  ) => {
    const summary = getPurchaseRealizationSummary(pr);

    if (summary.hasPaymentProof) {
      const visibleReceipts = compact
        ? summary.paymentProofReceipts.slice(0, 2)
        : summary.paymentProofReceipts;
      const hiddenCount = summary.paymentProofReceipts.length - visibleReceipts.length;

      return (
        <Flex
          direction="column"
          gap={1.5}
          align="flex-start"
          maxW={compact ? "180px" : "100%"}
          minW={0}
        >
          <Badge colorScheme="green" variant="subtle">
            Bukti Pembayaran Bank
          </Badge>
          <Flex gap={1.5} wrap="wrap" maxW="100%" minW={0}>
            {visibleReceipts.map((receipt) => (
              <Tooltip
                key={`${receipt.paymentId || "direct"}-${receipt.purchasePaymentId}`}
                label={receipt.paymentProofFileName || receipt.paymentCode}
                placement="top"
              >
                <Button
                  leftIcon={<FiDownload />}
                  size="xs"
                  colorScheme="green"
                  variant="outline"
                  onClick={() => handleDownloadPaymentReceipt(receipt)}
                  maxW={compact ? "170px" : "240px"}
                  minW={0}
                  justifyContent="flex-start"
                >
                  <Text as="span" noOfLines={1} minW={0}>
                    {compact
                      ? receipt.paymentCode
                      : receipt.paymentProofFileName || receipt.paymentCode}
                  </Text>
                </Button>
              </Tooltip>
            ))}
            {hiddenCount > 0 && (
              <Badge colorScheme="green" alignSelf="center">
                +{hiddenCount} bukti
              </Badge>
            )}
          </Flex>
          {summary.paymentWithoutProofCount > 0 && (
            <Badge colorScheme="yellow" variant="subtle">
              {summary.paymentWithoutProofCount} belum ada bukti
            </Badge>
          )}
          {summary.pendingPaymentProofCount > 0 && (
            <Badge colorScheme="yellow" variant="subtle">
              {summary.pendingPaymentProofCount} menunggu review FM
            </Badge>
          )}
          {summary.rejectedPaymentProofCount > 0 && (
            <Badge colorScheme="red" variant="subtle">
              {summary.rejectedPaymentProofCount} ditolak FM
            </Badge>
          )}
        </Flex>
      );
    }

    if (summary.hasPayment) {
      return (
        <Flex direction="column" gap={1} align="flex-start">
          {summary.pendingPaymentProofCount > 0 ? (
            <Badge colorScheme="yellow" variant="subtle">
              Menunggu review FM
            </Badge>
          ) : summary.rejectedPaymentProofCount > 0 ? (
            <Badge colorScheme="red" variant="subtle">
              Bukti ditolak FM
            </Badge>
          ) : (
            <Badge colorScheme="yellow" variant="subtle">
              Belum upload bukti
            </Badge>
          )}
        </Flex>
      );
    }

    if (summary.hasPurchase) {
      return (
        <Flex direction="column" gap={1} align="flex-start">
          <Badge colorScheme="blue" variant="subtle">
            Belum ada bukti
          </Badge>
          <Text fontSize="xs" color={mutedTextColor}>
            {summary.purchaseCount} pembelian sudah dibuat
          </Text>
        </Flex>
      );
    }

    return <Badge colorScheme="gray">Belum Pembelian</Badge>;
  };

  const hasFinanceStaffReview = (pr?: PurchaseRequest | null) => {
    return (
      Boolean(pr?.finance_staff_approved_by || pr?.finance_staff_approved_at) ||
      (pr?.items || []).some(
        (item) => normalizeDecision(item.finance_staff_decision) !== "PENDING",
      )
    );
  };

  const hasFinanceManagerReview = (pr?: PurchaseRequest | null) => {
    return (
      Boolean(
        pr?.finance_manager_approved_by || pr?.finance_manager_approved_at,
      ) ||
      (pr?.items || []).some(
        (item) => normalizeDecision(item.finance_manager_decision) !== "PENDING",
      )
    );
  };

  const matchesProgressFilter = (pr: PurchaseRequest) => {
    if (!progressFilter) return true;

    const stage = getCurrentReviewStage(pr);
    const realization = getPurchaseRealizationSummary(pr);
    const hasStaffReview = hasFinanceStaffReview(pr);
    const hasManagerReview = hasFinanceManagerReview(pr);

    switch (progressFilter) {
      case "WAITING_FINANCE_STAFF":
        return pr.status === "PENDING_APPROVAL" && stage === "FINANCE_STAFF";
      case "FINANCE_STAFF_APPROVED":
        return hasStaffReview;
      case "WAITING_FINANCE_MANAGER":
        return (
          pr.status === "PENDING_APPROVAL" &&
          stage === "FINANCE_MANAGER" &&
          !hasManagerReview
        );
      case "FINANCE_MANAGER_APPROVED":
        return hasManagerReview;
      case "FINANCE_MANAGER_APPROVED_WITHOUT_PURCHASE":
        return hasManagerReview && !realization.hasPurchase;
      case "AWAITING_VENDOR_INVOICE":
        return (
          pr.requires_vendor_invoice &&
          pr.status === "APPROVED" &&
          (pr.vendor_invoice_status || "").toUpperCase() ===
            "AWAITING_INVOICE"
        );
      case "VENDOR_INVOICE_REVIEW":
        return (
          pr.requires_vendor_invoice &&
          pr.status === "PENDING_APPROVAL" &&
          pr.reapproval_reason === "INVOICE_REVISION"
        );
      case "AWAITING_REQUESTER_REVISION":
        return (
          pr.status === "AWAITING_REVISION" &&
          pr.reapproval_reason === "REQUESTER_REVISION"
        );
      case "REQUESTER_REVISION_REVIEW":
        return (
          pr.status === "PENDING_APPROVAL" &&
          pr.reapproval_reason === "REQUESTER_REVISION"
        );
      case "WAITING_DIRECTOR":
        return pr.status === "PENDING_APPROVAL" && stage === "DIRECTOR";
      case "APPROVED":
        return pr.status === "APPROVED";
      case "REJECTED":
        return pr.status === "REJECTED";
      case "CANCELLED":
        return pr.status === "CANCELLED";
      case "PURCHASE_CREATED":
        return realization.hasPurchase;
      case "PAYMENT_CREATED":
        return realization.hasPayment;
      default:
        return true;
    }
  };

  const getInitialReviewDecision = (
    pr: PurchaseRequest,
    item: PurchaseRequestItem,
  ) => {
    const stage = getCurrentReviewStage(pr);
    if (stage === "DIRECTOR") {
      if (item.director_decision && item.director_decision !== "PENDING") {
        return item.director_decision;
      }
      if (
        item.finance_manager_decision &&
        item.finance_manager_decision !== "PENDING"
      ) {
        return item.finance_manager_decision;
      }
      if (
        item.finance_staff_decision &&
        item.finance_staff_decision !== "PENDING"
      ) {
        return item.finance_staff_decision;
      }
      return "";
    }

    if (stage === "FINANCE_MANAGER") {
      if (
        item.finance_manager_decision &&
        item.finance_manager_decision !== "PENDING"
      ) {
        return item.finance_manager_decision;
      }
      if (
        item.finance_staff_decision &&
        item.finance_staff_decision !== "PENDING"
      ) {
        return item.finance_staff_decision;
      }
      return "";
    }

    if (
      item.finance_staff_decision &&
      item.finance_staff_decision !== "PENDING"
    ) {
      return item.finance_staff_decision;
    }

    return "";
  };

  const initializeReviewForm = (pr: PurchaseRequest) => {
    const nextDecisions: Record<number, "APPROVED" | "REJECTED" | ""> = {};
    (pr.items || []).forEach((item) => {
      nextDecisions[item.id] = getInitialReviewDecision(pr, item);
    });
    setReviewDecisions(nextDecisions);
    setReviewReason(pr.rejection_reason || "");
    setReviewNeedsDirectorApproval(false);
    setReviewRequiresVendorInvoice(Boolean(pr.requires_vendor_invoice));
  };

  const setAllReviewDecisions = (decision: "APPROVED" | "REJECTED") => {
    if (!selectedRequest?.items?.length) return;
    const nextDecisions: Record<number, "APPROVED" | "REJECTED" | ""> = {};
    selectedRequest.items.forEach((item) => {
      nextDecisions[item.id] = decision;
    });
    setReviewDecisions(nextDecisions);
  };

  const handleReviewSubmit = async () => {
    if (!selectedRequest?.items?.length) return;

    const undecidedItem = selectedRequest.items.find(
      (item) => !reviewDecisions[item.id],
    );
    if (undecidedItem) {
      toast({
        title: "Semua item harus dipilih",
        description:
          "Tentukan approve atau reject untuk setiap item sebelum submit review.",
        status: "warning",
        duration: 4000,
      });
      return;
    }

    try {
      setIsSubmitting(true);
      await purchaseRequestService.review(selectedRequest.id, {
        items: selectedRequest.items.map((item) => ({
          item_id: item.id,
          decision: reviewDecisions[item.id] as "APPROVED" | "REJECTED",
        })),
        reason: reviewReason.trim() || undefined,
        escalate_to_director:
          canRequestDirectorApproval(selectedRequest) &&
          reviewNeedsDirectorApproval,
        require_vendor_invoice:
          getCurrentReviewStage(selectedRequest) === "FINANCE_STAFF" &&
          reviewRequiresVendorInvoice,
      });
      toast({
        title: "Review PR berhasil dikirim",
        status: "success",
        duration: 3000,
      });
      onViewClose();
      fetchData();
    } catch (err: any) {
      toast({
        title: "Gagal submit review PR",
        description: err.response?.data?.error || err.message,
        status: "error",
        duration: 4000,
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleUpdateProject = async () => {
    if (!selectedRequest) return;

    try {
      setIsUpdatingProject(true);
      const updated = await purchaseRequestService.updateProject(
        selectedRequest.id,
        {
          project_id: projectUpdateId ? parseInt(projectUpdateId, 10) : null,
        },
      );
      setSelectedRequest(updated);
      setProjectUpdateId(updated.project_id?.toString() || "");
      await fetchData();
      toast({
        title: "Project PR berhasil diperbarui",
        status: "success",
        duration: 3000,
      });
    } catch (err: any) {
      toast({
        title: "Gagal memperbarui project PR",
        description: err.response?.data?.error || err.message,
        status: "error",
        duration: 4000,
      });
    } finally {
      setIsUpdatingProject(false);
    }
  };

  const handleUpdateInvoiceTerms = async () => {
    if (!selectedRequest) return;

    const dueDateSummary = getPurchaseDueDateSummary(selectedRequest);
    const invoiceTermsError = validateInvoiceTermsInput(
      invoiceTermsUpdateDays,
      invoiceTermsUpdateReceivedDate,
      dueDateSummary.requiresDueDate,
    );
    if (invoiceTermsError) {
      return toast({
        title: "Tanggal invoice vendor belum valid",
        description: invoiceTermsError,
        status: "warning",
        duration: 4000,
      });
    }

    try {
      setIsUpdatingInvoiceTerms(true);
      const updated = await purchaseRequestService.updateInvoiceTerms(
        selectedRequest.id,
        buildInvoiceTermsPayload(
          invoiceTermsUpdateReceivedDate,
          invoiceTermsUpdateDays,
          true,
          dueDateSummary.requiresDueDate,
        ),
      );
      setSelectedRequest(updated);
      setInvoiceTermsUpdateReceivedDate(
        normalizeDateInputValue(updated.vendor_invoice_received_date),
      );
      setInvoiceTermsUpdateDays(getInvoiceTermsDaysInputValue(updated));
      await fetchData();
      toast({
        title: "Due date PR berhasil diperbarui",
        status: "success",
        duration: 3000,
      });
    } catch (err: any) {
      toast({
        title: "Gagal memperbarui due date PR",
        description: err.response?.data?.error || err.message,
        status: "error",
        duration: 5000,
      });
    } finally {
      setIsUpdatingInvoiceTerms(false);
    }
  };

  const handleOpenCancelRequest = () => {
    setCancelReason("");
    onCancelOpen();
  };

  const handleCancelPurchaseRequest = async () => {
    if (!selectedRequest) return;

    const reason = cancelReason.trim();
    if (!reason) {
      toast({
        title: "Alasan pembatalan wajib diisi",
        status: "warning",
        duration: 3000,
      });
      return;
    }

    try {
      setIsCancellingRequest(true);
      const updated = await purchaseRequestService.cancel(
        selectedRequest.id,
        reason,
      );
      setSelectedRequest(updated);
      await fetchData();
      onCancelClose();
      toast({
        title: "Purchase request berhasil dibatalkan",
        status: "success",
        duration: 3000,
      });
    } catch (err: any) {
      toast({
        title: "Gagal membatalkan PR",
        description: err.response?.data?.error || err.message,
        status: "error",
        duration: 5000,
      });
    } finally {
      setIsCancellingRequest(false);
    }
  };

  const handleView = async (pr: PurchaseRequest) => {
    try {
      setLoading(true);
      const detail = await purchaseRequestService.getById(pr.id);
      setSelectedRequest(detail);
      setProjectUpdateId(detail.project_id?.toString() || "");
      setInvoiceTermsUpdateReceivedDate(
        normalizeDateInputValue(detail.vendor_invoice_received_date),
      );
      setInvoiceTermsUpdateDays(
        getInvoiceTermsDaysInputValue(detail),
      );
      setPendingAdditionalDocuments([]);
      setPendingRealizationDocuments([]);
      setInvoiceRevisionItems(buildInvoiceRevisionItems(detail));
      setInvoiceRevisionDocuments([]);
      setInvoiceRevisionNotes(detail.vendor_invoice_revision_notes || "");
      setRequesterRevisionItems(buildRequesterRevisionItems(detail));
      setRequesterRevisionDocuments([]);
      setRequesterRevisionNotes("");
      setRequesterRevisionOpenReason("");
      setCancelReason("");
      initializeReviewForm(detail);
      onViewOpen();
    } catch {
      toast({
        title: "Gagal mengambil detail PR",
        status: "error",
        duration: 3000,
      });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    const notificationRequestId = searchParams?.get("notificationRequestId");

    if (
      !notificationRequestId ||
      handledNotificationRequestId.current === notificationRequestId
    ) {
      return;
    }

    handledNotificationRequestId.current = notificationRequestId;

    const clearNotificationRequestParam = () => {
      const nextParams = new URLSearchParams(searchParams?.toString() || "");
      nextParams.delete("notificationRequestId");
      const nextUrl = nextParams.toString()
        ? `/purchase-requests?${nextParams.toString()}`
        : "/purchase-requests";
      router.replace(nextUrl);
    };

    const requestId = Number(notificationRequestId);
    if (!Number.isFinite(requestId)) {
      clearNotificationRequestParam();
      return;
    }

    const openRequestFromNotification = async () => {
      try {
        setLoading(true);
        const detail = await purchaseRequestService.getById(requestId);
        setSelectedRequest(detail);
        setProjectUpdateId(detail.project_id?.toString() || "");
        setInvoiceTermsUpdateReceivedDate(
          normalizeDateInputValue(detail.vendor_invoice_received_date),
        );
        setInvoiceTermsUpdateDays(
          getInvoiceTermsDaysInputValue(detail),
        );
        setPendingAdditionalDocuments([]);
        setPendingRealizationDocuments([]);
        setInvoiceRevisionItems(buildInvoiceRevisionItems(detail));
        setInvoiceRevisionDocuments([]);
        setInvoiceRevisionNotes(detail.vendor_invoice_revision_notes || "");
        setRequesterRevisionItems(buildRequesterRevisionItems(detail));
        setRequesterRevisionDocuments([]);
        setRequesterRevisionNotes("");
        setRequesterRevisionOpenReason("");
        setCancelReason("");
        initializeReviewForm(detail);
        onViewOpen();
      } catch {
        toast({
          title: "Gagal membuka detail PR dari notifikasi",
          status: "error",
          duration: 3000,
        });
      } finally {
        setLoading(false);
        clearNotificationRequestParam();
      }
    };

    void openRequestFromNotification();
  }, [onViewOpen, router, searchParams, toast]);

  const getDateKey = (value?: string | null) => {
    if (!value) return "";

    const parsedDate = new Date(value);
    if (Number.isNaN(parsedDate.getTime())) return "";

    const year = parsedDate.getFullYear();
    const month = String(parsedDate.getMonth() + 1).padStart(2, "0");
    const day = String(parsedDate.getDate()).padStart(2, "0");

    return `${year}-${month}-${day}`;
  };

  const isDateRangeInvalid =
    Boolean(startDateFilter && endDateFilter) && startDateFilter > endDateFilter;

  const hasActiveFilters = Boolean(
    searchQuery ||
      vendorFilter ||
      progressFilter ||
      startDateFilter ||
      endDateFilter,
  );

  const handleClearFilters = () => {
    setSearchQuery("");
    setVendorFilter("");
    setProgressFilter("");
    setStartDateFilter("");
    setEndDateFilter("");
    setCurrentPage(1);
  };

  const filteredRequests = requests.filter((r) => {
    const normalizedSearch = searchQuery.trim().toLowerCase();
    const requesterSearchText = [
      r.requester?.first_name,
      r.requester?.last_name,
      r.requester?.username,
      r.requester?.email,
      r.requester?.department,
      r.requester?.position,
      r.requester?.role,
      humanizeRole(r.requester?.role),
      r.department,
    ]
      .filter(Boolean)
      .join(" ")
      .toLowerCase();
    const matchesSearch =
      !normalizedSearch ||
      r.pr_number.toLowerCase().includes(normalizedSearch) ||
      getPurchaseRequestTitle(r).toLowerCase().includes(normalizedSearch) ||
      (r.vendor?.name || "").toLowerCase().includes(normalizedSearch) ||
      (r.vendor?.code || "").toLowerCase().includes(normalizedSearch) ||
      (r.project?.name || "").toLowerCase().includes(normalizedSearch) ||
      (r.project?.code || "").toLowerCase().includes(normalizedSearch) ||
      requesterSearchText.includes(normalizedSearch);
    const matchesVendor =
      !vendorFilter ||
      r.vendor_id?.toString() === vendorFilter ||
      r.vendor?.id?.toString() === vendorFilter;

    const requestDate = getDateKey(r.created_at);
    const matchesStartDate =
      !startDateFilter || Boolean(requestDate && requestDate >= startDateFilter);
    const matchesEndDate =
      !endDateFilter || Boolean(requestDate && requestDate <= endDateFilter);

    return (
      matchesSearch &&
      matchesVendor &&
      matchesProgressFilter(r) &&
      !isDateRangeInvalid &&
      matchesStartDate &&
      matchesEndDate
    );
  });

  const totalPages = Math.max(1, Math.ceil(filteredRequests.length / pageSize));
  const pageStartIndex = (currentPage - 1) * pageSize;
  const paginatedRequests = filteredRequests.slice(
    pageStartIndex,
    pageStartIndex + pageSize,
  );

  useEffect(() => {
    setCurrentPage(1);
  }, [
    searchQuery,
    vendorFilter,
    progressFilter,
    startDateFilter,
    endDateFilter,
    pageSize,
  ]);

  useEffect(() => {
    setCurrentPage((page) => Math.min(page, totalPages));
  }, [totalPages]);

  const getStatusBadge = (request: PurchaseRequest) => {
    switch (request.status) {
      case "DRAFT":
        return <Badge colorScheme="gray">Draft</Badge>;
      case "PENDING_APPROVAL":
        if (request.reapproval_reason === "REQUESTER_REVISION") {
          return (
            <Badge colorScheme="purple">
              Revisi PIC - {getStageLabel(request) || "Review"}
            </Badge>
          );
        }
        return (
          <Badge colorScheme="orange">
            {getStageLabel(request) || "Pending Review"}
          </Badge>
        );
      case "APPROVED":
        if (
          request.requires_vendor_invoice &&
          (request.vendor_invoice_status || "").toUpperCase() ===
            "AWAITING_INVOICE"
        ) {
          return <Badge colorScheme="yellow">Approved - Butuh Invoice</Badge>;
        }
        return <Badge colorScheme="green">Approved</Badge>;
      case "REJECTED":
        return <Badge colorScheme="red">Rejected</Badge>;
      case "CANCELLED":
        return (
          <Badge colorScheme="orange">
            {request.finance_manager_approved_by || request.finance_manager_approved_at
              ? "FM Approved - Cancelled"
              : "Cancelled"}
          </Badge>
        );
      case "AWAITING_REVISION":
        return <Badge colorScheme="purple">Menunggu Revisi PIC</Badge>;
      case "COMPLETED":
        return <Badge colorScheme="blue">PO Processed</Badge>;
      default:
        return <Badge>{request.status}</Badge>;
    }
  };

  const getDecisionBadge = (decision?: string | null) => {
    switch (decision) {
      case "APPROVED":
        return <Badge colorScheme="green">Approved</Badge>;
      case "REJECTED":
        return <Badge colorScheme="red">Rejected</Badge>;
      default:
        return <Badge colorScheme="gray">Pending</Badge>;
    }
  };

  function normalizeDecision(
    decision?: string | null,
  ): PurchaseRequestItem["approval_status"] {
    if (decision === "APPROVED" || decision === "REJECTED") {
      return decision;
    }
    return "PENDING";
  }

  const getStageDecisionSummary = (
    pr: PurchaseRequest | null,
    stageKey: "finance_staff" | "finance_manager",
  ) => {
    const summary = {
      approved: 0,
      rejected: 0,
      pending: 0,
      total: 0,
      status: "PENDING" as "APPROVED" | "REJECTED" | "PARTIAL" | "PENDING",
    };

    (pr?.items || []).forEach((item) => {
      const decision = normalizeDecision(
        stageKey === "finance_staff"
          ? item.finance_staff_decision
          : item.finance_manager_decision,
      );
      summary.total += 1;
      if (decision === "APPROVED") summary.approved += 1;
      if (decision === "REJECTED") summary.rejected += 1;
      if (decision === "PENDING") summary.pending += 1;
    });

    if (summary.approved > 0 && summary.rejected > 0) {
      summary.status = "PARTIAL";
    } else if (summary.approved > 0 && summary.rejected === 0) {
      summary.status = "APPROVED";
    } else if (summary.rejected > 0 && summary.approved === 0) {
      summary.status = "REJECTED";
    }

    return summary;
  };

  const getStageSummaryBadge = (
    summary: ReturnType<typeof getStageDecisionSummary>,
  ) => {
    switch (summary.status) {
      case "APPROVED":
        return <Badge colorScheme="green">Approved</Badge>;
      case "REJECTED":
        return <Badge colorScheme="red">Rejected</Badge>;
      case "PARTIAL":
        return <Badge colorScheme="orange">Partial</Badge>;
      default:
        return <Badge colorScheme="gray">Pending</Badge>;
    }
  };

  const getStageSummaryText = (
    summary: ReturnType<typeof getStageDecisionSummary>,
  ) =>
    `${summary.approved} approved, ${summary.rejected} rejected, ${summary.pending} pending`;

  const getReviewNotes = (
    pr: PurchaseRequest | null,
    stageKey: "finance_staff" | "finance_manager",
  ) => {
    if (!pr) return "";
    if (stageKey === "finance_staff") {
      return pr.finance_staff_review_notes?.trim() || "";
    }

    const currentNotes = pr.finance_manager_review_notes?.trim();
    if (currentNotes) return currentNotes;

    if (!pr.director_approver && pr.finance_manager_approver) {
      return pr.rejection_reason?.trim() || "";
    }

    return "";
  };

  const getEffectiveItemDecision = (
    pr: PurchaseRequest | null,
    item: PurchaseRequestItem,
    includeDraftReview = false,
  ): PurchaseRequestItem["approval_status"] => {
    if (
      includeDraftReview &&
      canCurrentUserReviewRequest(pr) &&
      Object.prototype.hasOwnProperty.call(reviewDecisions, item.id)
    ) {
      return normalizeDecision(reviewDecisions[item.id]);
    }

    const stage = getCurrentReviewStage(pr);
    if (stage === "DIRECTOR") {
      const directorDecision = normalizeDecision(item.director_decision);
      if (directorDecision !== "PENDING") return directorDecision;

      const managerDecision = normalizeDecision(item.finance_manager_decision);
      if (managerDecision !== "PENDING") return managerDecision;

      return normalizeDecision(item.finance_staff_decision);
    }

    if (stage === "FINANCE_MANAGER") {
      const managerDecision = normalizeDecision(item.finance_manager_decision);
      if (managerDecision !== "PENDING") return managerDecision;

      return normalizeDecision(item.finance_staff_decision);
    }

    if (stage === "FINANCE_STAFF") {
      return normalizeDecision(item.finance_staff_decision);
    }

    return normalizeDecision(item.approval_status);
  };

  const getReviewSummary = (
    pr: PurchaseRequest | null,
    includeDraftReview = false,
  ) => {
    const summary = {
      approved: 0,
      rejected: 0,
      pending: 0,
      activeTotal: 0,
    };

    (pr?.items || []).forEach((item) => {
      const decision = getEffectiveItemDecision(pr, item, includeDraftReview);

      if (decision === "APPROVED") summary.approved += 1;
      if (decision === "REJECTED") summary.rejected += 1;
      if (decision === "PENDING") summary.pending += 1;

      if (decision !== "REJECTED") {
        summary.activeTotal += Number(item.total_estimated_price || 0);
      }
    });

    return summary;
  };

  const getDisplayBudgetTotal = (pr: PurchaseRequest) => {
    if (!pr.items?.length) {
      return Number(pr.total_estimated_amount || 0);
    }

    return getReviewSummary(pr).activeTotal;
  };

  const selectedRequestSummary = getReviewSummary(selectedRequest, true);
  const selectedRequestDueDateSummary =
    getPurchaseDueDateSummary(selectedRequest);
  const selectedRequestRealizationSummary =
    getPurchaseRealizationSummary(selectedRequest);
  const selectedRequestDocuments = selectedRequest?.documents || [];
  const selectedRequestRealizationDocuments = selectedRequestDocuments.filter(
    (document) => (document.document_type || "").toUpperCase() === "REALIZATION_DOCUMENT",
  );
  const selectedRequestSupportingDocuments = selectedRequestDocuments.filter(
    (document) => (document.document_type || "").toUpperCase() !== "REALIZATION_DOCUMENT",
  );
  const createDueDatePreview = getCalculatedDueDateInput(
    vendorInvoiceReceivedDate,
    paymentTermsDays,
  );
  const invoiceTermsUpdateDueDatePreview =
    selectedRequestDueDateSummary.requiresDueDate
      ? getCalculatedDueDateInput(
          invoiceTermsUpdateReceivedDate,
          invoiceTermsUpdateDays,
        )
      : "";
  const canEditSelectedRequestInvoiceTerms = Boolean(
    selectedRequest &&
      canEdit("purchase_requests") &&
      selectedRequest.status !== "COMPLETED" &&
      selectedRequest.status !== "REJECTED" &&
      selectedRequest.status !== "CANCELLED" &&
      selectedRequest.status !== "AWAITING_REVISION",
  );
  const canEditSelectedRequestProject = Boolean(
    selectedRequest &&
      canUpdatePurchaseRequestProject &&
      selectedRequest.status !== "CANCELLED",
  );
  const canSubmitSelectedRequestInvoiceRevision = Boolean(
    selectedRequest &&
      canEdit("purchase_requests") &&
      selectedRequest.requires_vendor_invoice &&
      selectedRequest.status === "APPROVED" &&
      !selectedRequestRealizationSummary.hasPurchase &&
      (roleNorm === "admin" || selectedRequest.requester_id === Number(user?.id)),
  );
  const canSubmitSelectedRequesterRevision = Boolean(
    selectedRequest &&
      selectedRequest.status === "AWAITING_REVISION" &&
      selectedRequest.reapproval_reason === "REQUESTER_REVISION" &&
      !selectedRequestRealizationSummary.hasPurchase &&
      (roleNorm === "admin" || selectedRequest.requester_id === Number(user?.id)),
  );
  const canUploadSelectedRequestRealizationDocuments = Boolean(
    selectedRequest &&
      canEdit("purchase_requests") &&
      (selectedRequest.status === "APPROVED" ||
        selectedRequest.status === "COMPLETED") &&
      (roleNorm === "admin" ||
        isFinanceTeamRole(roleNorm) ||
        selectedRequest.requester_id === Number(user?.id)),
  );

  const getVendorInvoiceStatusBadge = (pr?: PurchaseRequest | null) => {
    if (!pr?.requires_vendor_invoice) {
      return <Badge colorScheme="gray">Tidak Perlu Invoice Aktual</Badge>;
    }

    switch ((pr.vendor_invoice_status || "").toUpperCase()) {
      case "PENDING_APPROVAL":
        return <Badge colorScheme="orange">Invoice Menunggu Review</Badge>;
      case "APPROVED":
        return <Badge colorScheme="green">Invoice Aktual Approved</Badge>;
      case "AWAITING_INVOICE":
      default:
        return <Badge colorScheme="yellow">Menunggu Invoice Aktual</Badge>;
    }
  };

  const getDocumentTypeBadge = (documentType?: string) => {
    if ((documentType || "").toUpperCase() === "VENDOR_INVOICE") {
      return (
        <Badge mt={1} colorScheme="purple">
          Invoice Aktual
        </Badge>
      );
    }
    if ((documentType || "").toUpperCase() === "REQUESTER_REVISION_DOCUMENT") {
      return (
        <Badge mt={1} colorScheme="orange">
          Revisi PIC
        </Badge>
      );
    }
    if ((documentType || "").toUpperCase() === "REALIZATION_DOCUMENT") {
      return (
        <Badge mt={1} colorScheme="green">
          Realisasi PIC
        </Badge>
      );
    }
    return null;
  };

  return (
    <ProtectedModule module="purchase_requests">
      <Layout>
        <Box p={6}>
          <Flex justify="space-between" align="center" mb={6}>
            <Box>
              <Heading size="lg" color={textColor}>
                Purchase Requests
              </Heading>
              <Text color={mutedTextColor} mt={1}>
                Kelola Pengajuan Pembelian Barang/Jasa
              </Text>
            </Box>
            {canCreate("purchase_requests") && (
              <Button
                leftIcon={<FiPlus />}
                colorScheme="blue"
                onClick={handleOpenCreate}
              >
                Buat Pengajuan
              </Button>
            )}
          </Flex>

          <Box
            bg={bgCard}
            rounded="lg"
            shadow="sm"
            borderWidth="1px"
            borderColor={borderColor}
          >
            <Flex
              p={4}
              borderBottomWidth="1px"
              borderColor={borderColor}
              gap={3}
              align="flex-end"
              wrap="wrap"
            >
              <FormControl maxW={{ base: "100%", md: "320px" }}>
                <FormLabel fontSize="sm" color={mutedTextColor}>
                  Pencarian
                </FormLabel>
                <InputGroup>
                  <InputLeftElement pointerEvents="none">
                    <Icon as={FiSearch} color={mutedTextColor} />
                  </InputLeftElement>
                  <Input
                    placeholder="Cari Nomor PR, Submitter, Departemen, Vendor, atau Proyek..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    bg={inputBg}
                    borderColor={inputBorderColor}
                    color={textColor}
                    _hover={{ borderColor: inputBorderColor }}
                    _focusVisible={fieldFocusStyles}
                    _placeholder={{ color: mutedTextColor }}
	                  />
		                </InputGroup>
		              </FormControl>

              <FormControl maxW={{ base: "100%", md: "260px" }}>
                <FormLabel fontSize="sm" color={mutedTextColor}>
                  Vendor
                </FormLabel>
                <SearchableSelect
                  options={vendors.map((vendor) => ({
                    id: vendor.id,
                    code: vendor.code,
                    name: vendor.name,
                    active: vendor.is_active,
                  }))}
                  placeholder="Semua Vendor"
                  value={vendorFilter}
                  onChange={(value) =>
                    setVendorFilter(value ? value.toString() : "")
                  }
                  displayFormat={(vendor) =>
                    vendor.code
                      ? `${vendor.code} - ${vendor.name}`
                      : vendor.name
                  }
                  size="md"
                />
              </FormControl>

		              <FormControl maxW={{ base: "100%", md: "260px" }}>
		                <FormLabel fontSize="sm" color={mutedTextColor}>
		                  Progress
	                </FormLabel>
	                <Select
	                  value={progressFilter}
	                  onChange={(e) =>
	                    setProgressFilter(
	                      e.target.value as PurchaseRequestProgressFilter,
	                    )
	                  }
	                  bg={inputBg}
	                  borderColor={inputBorderColor}
	                  color={textColor}
	                  _hover={{ borderColor: inputBorderColor }}
	                  _focusVisible={fieldFocusStyles}
	                >
	                  {PURCHASE_REQUEST_PROGRESS_FILTERS.map((option) => (
	                    <option key={option.value || "all"} value={option.value}>
	                      {option.label}
	                    </option>
	                  ))}
	                </Select>
	              </FormControl>

	              <FormControl maxW={{ base: "100%", md: "180px" }}>
                <FormLabel fontSize="sm" color={mutedTextColor}>
                  Dari Tanggal
                </FormLabel>
                <Input
                  type="date"
                  value={startDateFilter}
                  onChange={(e) => setStartDateFilter(e.target.value)}
                  bg={inputBg}
                  borderColor={inputBorderColor}
                  color={textColor}
                  _hover={{ borderColor: inputBorderColor }}
                  _focusVisible={fieldFocusStyles}
                />
              </FormControl>

              <FormControl maxW={{ base: "100%", md: "180px" }}>
                <FormLabel fontSize="sm" color={mutedTextColor}>
                  Sampai Tanggal
                </FormLabel>
                <Input
                  type="date"
                  value={endDateFilter}
                  onChange={(e) => setEndDateFilter(e.target.value)}
                  bg={inputBg}
                  borderColor={inputBorderColor}
                  color={textColor}
                  _hover={{ borderColor: inputBorderColor }}
                  _focusVisible={fieldFocusStyles}
                />
              </FormControl>

              <Button
                leftIcon={<FiX />}
                variant="outline"
                onClick={handleClearFilters}
                isDisabled={!hasActiveFilters}
              >
                Reset Filter
              </Button>

              <Text
                fontSize="sm"
                color={isDateRangeInvalid ? "red.500" : mutedTextColor}
                ml={{ base: 0, md: "auto" }}
              >
                {isDateRangeInvalid
                  ? "Tanggal mulai tidak boleh lebih besar dari tanggal akhir."
                  : `Menampilkan ${filteredRequests.length} dari ${requests.length} PR`}
              </Text>
            </Flex>

            <Box overflowX="auto">
              <Table variant="simple">
                <Thead bg={tableHeaderBg}>
	                  <Tr>
	                    <Th>PR Number</Th>
	                    <Th>Judul Pengajuan</Th>
	                    <Th>Requester</Th>
	                    <Th>Vendor Name</Th>
	                    <Th>Departemen/Proyek</Th>
	                    <Th>Tgl Pengajuan</Th>
		                    <Th isNumeric>Estimasi Anggaran</Th>
		                    <Th>Progress</Th>
		                    <Th>Realisasi</Th>
		                    <Th>Due Date</Th>
		                    <Th>Status</Th>
		                    <Th textAlign="center">Aksi</Th>
		                  </Tr>
                </Thead>
                <Tbody>
	                  {loading ? (
	                    <Tr>
	                      <Td colSpan={12} textAlign="center" py={4}>
	                        Memuat...
	                      </Td>
	                    </Tr>
	                  ) : filteredRequests.length === 0 ? (
	                    <Tr>
	                      <Td colSpan={12} textAlign="center" py={4}>
	                        Tidak ada pengajuan ditemukan
	                      </Td>
	                    </Tr>
                  ) : (
                    paginatedRequests.map((pr) => (
                      <Tr key={pr.id} _hover={{ bg: rowHoverBg }}>
                        <Td
                          fontWeight="bold"
                          color="blue.500"
                          cursor="pointer"
                          onClick={() => handleView(pr)}
	                        >
	                          {pr.pr_number}
	                        </Td>
	                        <Td minW="220px">
	                          <Text fontWeight="medium" noOfLines={2}>
	                            {getPurchaseRequestTitle(pr)}
	                          </Text>
	                        </Td>
	                        <Td>
	                          {pr.requester?.first_name} {pr.requester?.last_name}
	                        </Td>
                        <Td>
                          <Text fontWeight="medium">
                            {pr.vendor?.name || "-"}
                          </Text>
                          {pr.vendor?.code && (
                            <Text fontSize="xs" color={mutedTextColor}>
                              {pr.vendor.code}
                            </Text>
                          )}
                        </Td>
                        <Td>
                          {pr.department && (
                            <Text fontSize="xs" color={mutedTextColor}>
                              {pr.department}
                            </Text>
                          )}
                          {pr.project && (
                            <Badge colorScheme="purple" fontSize="xs" mt={1}>
                              {pr.project.code} - {pr.project.name}
                            </Badge>
                          )}
                        </Td>
                        <Td>{new Date(pr.created_at).toLocaleDateString()}</Td>
	                        <Td isNumeric>
	                          {formatCurrency(getDisplayBudgetTotal(pr))}
	                        </Td>
	                        <Td>
	                          <Text fontSize="sm">
	                            {getStageLabel(pr) || "Selesai"}
	                          </Text>
	                          {pr.finance_manager_approved_at && (
	                            <Badge colorScheme="green" mt={1}>
	                              FM Approved
	                            </Badge>
	                          )}
                            {pr.requires_vendor_invoice && (
                              <Box mt={1}>
                                {getVendorInvoiceStatusBadge(pr)}
                              </Box>
                            )}
	                        </Td>
		                        <Td minW="260px">
		                          {renderPaymentProofRealization(pr, true)}
		                        </Td>
		                        <Td minW="150px">
		                          {renderPurchaseDueDate(pr, true)}
		                        </Td>
		                        <Td>{getStatusBadge(pr)}</Td>
                        <Td>
                          <Flex justify="center" gap={2}>
                            <Tooltip label="Lihat Detail">
                              <IconButton
                                aria-label="View"
                                icon={<FiEye />}
                                size="sm"
                                variant="ghost"
                                onClick={() => handleView(pr)}
                              />
                            </Tooltip>

                            {canCurrentUserReviewRequest(pr) && (
                              <Tooltip label="Review Item PR">
                                <IconButton
                                  aria-label="Review"
                                  icon={<FiCheck />}
                                  size="sm"
                                  colorScheme="blue"
                                  variant="ghost"
                                  onClick={() => handleView(pr)}
                                />
                              </Tooltip>
                            )}
                          </Flex>
                        </Td>
                      </Tr>
                    ))
                  )}
                </Tbody>
              </Table>
            </Box>

            {filteredRequests.length > 0 && (
              <Flex
                mt={4}
                pt={4}
                borderTopWidth="1px"
                borderColor={borderColor}
                align={{ base: "stretch", md: "center" }}
                justify="space-between"
                direction={{ base: "column", md: "row" }}
                gap={3}
              >
                <Text fontSize="sm" color={mutedTextColor}>
                  Menampilkan {pageStartIndex + 1}-
                  {Math.min(pageStartIndex + pageSize, filteredRequests.length)}{" "}
                  dari {filteredRequests.length} PR
                </Text>
                <Flex align="center" gap={2} wrap="wrap">
                  <Text fontSize="sm" color={mutedTextColor}>
                    Rows
                  </Text>
                  <Select
                    size="sm"
                    maxW="90px"
                    value={pageSize}
                    onChange={(e) => setPageSize(Number(e.target.value))}
                    bg={inputBg}
                    borderColor={inputBorderColor}
                    color={textColor}
                    _hover={{ borderColor: inputBorderColor }}
                    _focusVisible={fieldFocusStyles}
                  >
                    {PURCHASE_REQUEST_PAGE_SIZE_OPTIONS.map((option) => (
                      <option key={option} value={option}>
                        {option}
                      </option>
                    ))}
                  </Select>
                  <Button
                    size="sm"
                    variant="outline"
                    leftIcon={<FiChevronLeft />}
                    onClick={() =>
                      setCurrentPage((page) => Math.max(page - 1, 1))
                    }
                    isDisabled={currentPage <= 1}
                  >
                    Prev
                  </Button>
                  <Text fontSize="sm" color={mutedTextColor}>
                    Halaman {currentPage} / {totalPages}
                  </Text>
                  <Button
                    size="sm"
                    variant="outline"
                    rightIcon={<FiChevronRight />}
                    onClick={() =>
                      setCurrentPage((page) => Math.min(page + 1, totalPages))
                    }
                    isDisabled={currentPage >= totalPages}
                  >
                    Next
                  </Button>
                </Flex>
              </Flex>
            )}
          </Box>
        </Box>

        {/* MODAL CREATE PR */}
        <Modal
          isOpen={isCreateOpen}
          onClose={onCreateClose}
          size="4xl"
          scrollBehavior="inside"
        >
          <ModalOverlay />
          <ModalContent
            bg={modalBg}
            borderWidth="1px"
            borderColor={borderColor}
            w={{ base: "calc(100vw - 1rem)", md: "calc(100vw - 2rem)" }}
            maxW="1200px"
            maxH={{ base: "calc(100dvh - 0.75rem)", md: "calc(100dvh - 2rem)" }}
            display="flex"
            flexDirection="column"
            overflow="hidden"
          >
            <ModalHeader
              bg={modalHeaderBg}
              borderBottomWidth="1px"
              borderColor={borderColor}
              flexShrink={0}
            >
              Buat Purchase Request Baru
            </ModalHeader>
            <ModalCloseButton color={textColor} />
            <ModalBody flex="1" minH={0} overflowY="auto" px={{ base: 4, md: 6 }}>
              <Grid
                templateColumns={{ base: "1fr", md: "repeat(2, 1fr)" }}
	                gap={4}
	                mb={6}
	              >
	                <FormControl gridColumn={{ base: "auto", md: "1 / -1" }}>
	                  <FormLabel color={textColor}>Judul Pengajuan</FormLabel>
	                  <Input
	                    value={title}
	                    onChange={(e) => setTitle(e.target.value)}
	                    placeholder="Contoh: Jasa konsultan perizinan"
	                    maxLength={255}
	                    bg={inputBg}
	                    borderColor={inputBorderColor}
	                    color={textColor}
	                    _hover={{ borderColor: inputBorderColor }}
	                    _focusVisible={fieldFocusStyles}
	                    _placeholder={{ color: mutedTextColor }}
	                  />
	                </FormControl>
	                <FormControl>
	                  <FormLabel color={textColor}>Departemen</FormLabel>
                  <Input
                    value={department}
                    onChange={(e) => setDepartment(e.target.value)}
                    placeholder="Opsional"
                    bg={inputBg}
                    borderColor={inputBorderColor}
                    color={textColor}
                    _hover={{ borderColor: inputBorderColor }}
                    _focusVisible={fieldFocusStyles}
                    _placeholder={{ color: mutedTextColor }}
                  />
                </FormControl>
                <FormControl>
                  <FormLabel color={textColor}>Vendor Name</FormLabel>
                  <SearchableSelect
                    options={vendors.map((vendor) => ({
                      id: vendor.id,
                      code: vendor.code,
                      name: vendor.name,
                      active: vendor.is_active,
                    }))}
                    placeholder="-- Pilih Vendor (Opsional) --"
                    value={vendorId}
                    onChange={(value) =>
                      handleVendorChange(value ? value.toString() : "")
                    }
                    displayFormat={(vendor) =>
                      vendor.code
                        ? `${vendor.code} - ${vendor.name}`
                        : vendor.name
                    }
                    size="md"
                  />
		                </FormControl>
	                <FormControl>
	                  <FormLabel color={textColor}>
	                    Tanggal Terima Invoice Vendor
	                  </FormLabel>
	                  <Input
	                    type="date"
	                    value={normalizeDateInputValue(vendorInvoiceReceivedDate)}
	                    onChange={(e) =>
	                      setVendorInvoiceReceivedDate(
	                        normalizeDateInputValue(e.currentTarget.value),
	                      )
	                    }
	                    bg={inputBg}
	                    borderColor={inputBorderColor}
	                    color={textColor}
	                    _hover={{ borderColor: inputBorderColor }}
	                    _focusVisible={fieldFocusStyles}
	                  />
	                  <Text fontSize="xs" color={mutedTextColor} mt={1}>
	                    Ketuk field untuk membuka kalender.
	                  </Text>
	                </FormControl>
	                <FormControl>
	                  <FormLabel color={textColor}>Termin Pembayaran</FormLabel>
	                  <Input
	                    type="number"
	                    min={0}
	                    value={paymentTermsDays}
	                    onChange={(e) =>
	                      setPaymentTermsDays(
	                        e.target.value.replace(/[^0-9]/g, ""),
	                      )
	                    }
	                    placeholder="Contoh: 30"
	                    bg={inputBg}
	                    borderColor={inputBorderColor}
	                    color={textColor}
	                    _hover={{ borderColor: inputBorderColor }}
	                    _focusVisible={fieldFocusStyles}
	                    _placeholder={{ color: mutedTextColor }}
	                  />
	                  <Text fontSize="xs" color={mutedTextColor} mt={1}>
	                    Hari sejak invoice vendor diterima.
	                  </Text>
	                </FormControl>
	                <FormControl>
	                  <FormLabel color={textColor}>Due Date</FormLabel>
	                  <Input
	                    value={formatDateInputDisplayValue(createDueDatePreview) || "-"}
	                    isReadOnly
	                    bg={inputBg}
	                    borderColor={inputBorderColor}
	                    color={textColor}
	                  />
	                </FormControl>
	                <FormControl>
	                  <FormLabel color={textColor}>Tautkan Ke Proyek</FormLabel>
                  <Select
                    placeholder="-- Pilih Proyek (Opsional) --"
                    value={projectId}
                    onChange={(e) => setProjectId(e.target.value)}
                    bg={inputBg}
                    borderColor={inputBorderColor}
                    color={textColor}
                    _hover={{ borderColor: inputBorderColor }}
                    _focusVisible={fieldFocusStyles}
                  >
                    {projects.map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.code} - {p.name}
                      </option>
                    ))}
                  </Select>
                </FormControl>
              </Grid>

              <Box
                border="1px"
                borderColor={borderColor}
                rounded="md"
                p={4}
                mb={4}
                bg={infoPanelBg}
              >
                <Text fontWeight="bold" mb={4} color={textColor}>
                  Rincian Kebutuhan (Items)
                </Text>
                <Table size="sm" variant="unstyled">
                  <Thead borderBottom="1px" borderColor={borderColor}>
                    <Tr>
                      <Th color={mutedTextColor} px={1}>
                        Item / Benchmark
                      </Th>
                      <Th color={mutedTextColor} w="100px" px={1}>
                        Qty
                      </Th>
                      <Th color={mutedTextColor} w="180px" px={1}>
                        Estimasi Harga Satuan
                      </Th>
                      <Th color={mutedTextColor} px={1}>
                        Catatan
                      </Th>
                      <Th w="50px" px={1}></Th>
                    </Tr>
                  </Thead>
                  <Tbody>
                    {items.map((item, index) => (
                      <Tr key={index}>
                        <Td p={1} verticalAlign="bottom">
                          <ButtonGroup
                            isAttached
                            size="xs"
                            variant="outline"
                            mb={2}
                          >
                            <Button
                              leftIcon={<FiPackage />}
                              colorScheme={
                                item.source === "catalog" ? "blue" : "gray"
                              }
                              variant={
                                item.source === "catalog" ? "solid" : "outline"
                              }
                              onClick={() =>
                                handleItemSourceChange(index, "catalog")
                              }
                            >
                              Katalog
                            </Button>
                            <Button
                              leftIcon={<FiEdit3 />}
                              colorScheme={
                                item.source === "manual" ? "blue" : "gray"
                              }
                              variant={
                                item.source === "manual" ? "solid" : "outline"
                              }
                              onClick={() =>
                                handleItemSourceChange(index, "manual")
                              }
                            >
                              Input Manual
                            </Button>
                          </ButtonGroup>
                          {item.source === "catalog" ? (
                            <Select
                              placeholder="Pilih Item/Barang"
                              value={item.product_id}
                              onChange={(e) =>
                                handleItemChange(
                                  index,
                                  "product_id",
                                  e.target.value,
                                )
                              }
                              bg={inputBg}
                              borderColor={inputBorderColor}
                              color={textColor}
                              _hover={{ borderColor: inputBorderColor }}
                              _focusVisible={fieldFocusStyles}
                            >
                              {products.map((p) => (
                                <option key={p.id} value={p.id}>
                                  {p.code} - {p.name}
                                </option>
                              ))}
                            </Select>
                          ) : (
                            <Input
                              placeholder="Nama kebutuhan, contoh: Jasa servis AC"
                              value={item.item_name}
                              onChange={(e) =>
                                handleItemChange(
                                  index,
                                  "item_name",
                                  e.target.value,
                                )
                              }
                              maxLength={255}
                              bg={inputBg}
                              borderColor={inputBorderColor}
                              color={textColor}
                              _hover={{ borderColor: inputBorderColor }}
                              _focusVisible={fieldFocusStyles}
                              _placeholder={{ color: mutedTextColor }}
                            />
                          )}
                        </Td>
                        <Td p={1} verticalAlign="bottom">
                          <Input
                            type="number"
                            min={1}
                            value={item.quantity}
                            onKeyDown={(e) => {
                              // Cegah input huruf 'e', tanda plus/minus, dan titik/koma
                              if (
                                ["e", "E", "+", "-", ".", ","].includes(e.key)
                              ) {
                                e.preventDefault();
                              }
                            }}
                            onChange={(e) => {
                              // Pastikan tidak ada huruf yang lolos (misal dr paste)
                              const cleanVal = e.target.value.replace(
                                /[^0-9]/g,
                                "",
                              );
                              handleItemChange(index, "quantity", cleanVal);
                            }}
                            bg={inputBg}
                            borderColor={inputBorderColor}
                            color={textColor}
                            _hover={{ borderColor: inputBorderColor }}
                            _focusVisible={fieldFocusStyles}
                          />
                        </Td>
                        <Td p={1} verticalAlign="bottom">
                          <CurrencyInput
                            value={Number(item.estimated_price) || 0}
                            onChange={(val) =>
                              handleItemChange(
                                index,
                                "estimated_price",
                                val.toString(),
                              )
                            }
                            allowDecimals
                            decimalScale={2}
                          />
                        </Td>
                        <Td p={1} verticalAlign="bottom">
                          <Input
                            placeholder="Cth: Mendesak"
                            value={item.notes}
                            onChange={(e) =>
                              handleItemChange(index, "notes", e.target.value)
                            }
                            bg={inputBg}
                            borderColor={inputBorderColor}
                            color={textColor}
                            _hover={{ borderColor: inputBorderColor }}
                            _focusVisible={fieldFocusStyles}
                            _placeholder={{ color: mutedTextColor }}
                          />
                        </Td>
                        <Td p={1} textAlign="right" verticalAlign="bottom">
                          <IconButton
                            aria-label="Remove"
                            icon={<FiTrash2 />}
                            size="sm"
                            colorScheme="red"
                            variant="ghost"
                            onClick={() => handleRemoveItem(index)}
                          />
                        </Td>
                      </Tr>
                    ))}
                  </Tbody>
                </Table>
                <Button
                  leftIcon={<FiPlus />}
                  size="sm"
                  mt={3}
                  onClick={handleAddItem}
                >
                  Tambah Item
                </Button>
              </Box>

              <FormControl>
                <FormLabel color={textColor}>Catatan Umum</FormLabel>
                <Textarea
                  value={notes}
                  onChange={(e) => setNotes(e.target.value)}
                  rows={3}
                  bg={inputBg}
                  borderColor={inputBorderColor}
                  color={textColor}
                  _hover={{ borderColor: inputBorderColor }}
                  _focusVisible={fieldFocusStyles}
                />
              </FormControl>

              <Box
                mt={4}
                borderWidth="1px"
                borderColor={borderColor}
                rounded="md"
                p={4}
                bg={infoPanelBg}
              >
                <Flex
                  justify="space-between"
                  align={{ base: "flex-start", md: "center" }}
                  direction={{ base: "column", md: "row" }}
                  gap={3}
                >
                  <Box>
                    <Text fontWeight="bold" color={textColor}>
                      Dokumen Pendukung
                    </Text>
                    <Text fontSize="sm" color={mutedTextColor}>
                      Bisa unggah beberapa file dalam satu PR: PDF, Word, Excel,
                      CSV, TXT, JPG, PNG, atau WebP. Maksimal 50MB per file.
                    </Text>
                  </Box>
                  <Button
                    as="label"
                    leftIcon={<FiPaperclip />}
                    variant="outline"
                    colorScheme="blue"
                    cursor="pointer"
                    flexShrink={0}
                  >
                    Pilih Dokumen
                    <Input
                      type="file"
                      multiple
                      accept={ACCEPTED_DOCUMENT_TYPES}
                      display="none"
                      onChange={handleDocumentChange}
                    />
                  </Button>
                </Flex>

                {documents.length > 0 && (
                  <Box mt={3}>
                    {documents.map((file, index) => (
                      <Flex
                        key={`${file.name}-${file.size}-${index}`}
                        align="center"
                        justify="space-between"
                        gap={3}
                        py={2}
                        borderTopWidth={index === 0 ? "0" : "1px"}
                        borderColor={borderColor}
                      >
                        <Flex align="center" gap={2} minW={0}>
                          <Icon as={FiFileText} color="blue.500" />
                          <Box minW={0}>
                            <Text fontSize="sm" fontWeight="medium" noOfLines={1}>
                              {file.name}
                            </Text>
                            <Text fontSize="xs" color={mutedTextColor}>
                              {formatFileSize(file.size)}
                            </Text>
                          </Box>
                        </Flex>
                        <IconButton
                          aria-label="Hapus dokumen"
                          icon={<FiX />}
                          size="sm"
                          variant="ghost"
                          colorScheme="red"
                          onClick={() => handleRemoveDocument(index)}
                        />
                      </Flex>
                    ))}
                  </Box>
                )}
              </Box>
            </ModalBody>
            <ModalFooter
              bg={modalFooterBg}
              borderTop="1px"
              borderColor={borderColor}
              borderBottomRadius="md"
              flexShrink={0}
              gap={3}
              flexWrap="wrap"
              position="sticky"
              bottom={0}
              zIndex={2}
            >
              <Button
                variant="ghost"
                onClick={onCreateClose}
                w={{ base: "full", sm: "auto" }}
              >
                Batal
              </Button>
              <Button
                colorScheme="blue"
                isLoading={isSubmitting}
                onClick={handleCreateSubmit}
                w={{ base: "full", sm: "auto" }}
              >
                Kirim Pengajuan
              </Button>
            </ModalFooter>
          </ModalContent>
        </Modal>

        {/* MODAL VIEW / DETAILS */}
        <Modal
          isOpen={isViewOpen}
          onClose={onViewClose}
          size="6xl"
          scrollBehavior="inside"
        >
          <ModalOverlay />
          <ModalContent
            bg={modalBg}
            borderWidth="1px"
            borderColor={borderColor}
            w={{ base: "calc(100vw - 1rem)", lg: "calc(100vw - 4rem)" }}
            maxW="1800px"
            maxH={{ base: "calc(100dvh - 0.75rem)", md: "calc(100dvh - 2rem)" }}
            mx="auto"
            display="flex"
            flexDirection="column"
            overflow="hidden"
          >
            <ModalHeader
              display="flex"
              justifyContent="space-between"
              alignItems={{ base: "flex-start", md: "center" }}
              flexDirection={{ base: "column", md: "row" }}
              gap={2}
              pr={14}
              bg={modalHeaderBg}
              borderBottomWidth="1px"
              borderColor={borderColor}
              flexShrink={0}
            >
              <Text pr={2}>
                Detail Purchase Request: {selectedRequest?.pr_number}
              </Text>
              <Box>
                {selectedRequest && getStatusBadge(selectedRequest)}
              </Box>
            </ModalHeader>
            <ModalCloseButton color={textColor} />
            <ModalBody
              flex="1"
              minH={0}
              pb={6}
              overflowY="auto"
              overflowX="hidden"
              px={{ base: 4, md: 6 }}
              overscrollBehavior="contain"
            >
              {selectedRequest && (
                <Grid
                  templateColumns={{ base: "1fr", md: "repeat(2, 1fr)" }}
                  gap={4}
                  mb={6}
                  bg={infoPanelBg}
                  p={4}
                  rounded="md"
                >
	                  <Box>
	                    <Text fontSize="sm" color={mutedTextColor}>
	                      Pemohon
	                    </Text>
	                    <Text fontWeight="medium">
	                      {selectedRequest.requester?.first_name}{" "}
	                      {selectedRequest.requester?.last_name}
	                    </Text>
	                  </Box>
	                  <Box>
	                    <Text fontSize="sm" color={mutedTextColor}>
	                      Judul Pengajuan
	                    </Text>
	                    <Text fontWeight="medium">
	                      {getPurchaseRequestTitle(selectedRequest)}
	                    </Text>
	                  </Box>
	                  <Box>
	                    <Text fontSize="sm" color={mutedTextColor}>
	                      Departemen
                    </Text>
                    <Text fontWeight="medium">
                      {selectedRequest.department || "-"}
                    </Text>
                  </Box>
                  <Box>
                    <Text fontSize="sm" color={mutedTextColor}>
                      Vendor Name
                    </Text>
                    <Text fontWeight="medium">
                      {selectedRequest.vendor?.name || "-"}
                    </Text>
                    {selectedRequest.vendor?.code && (
                      <Text fontSize="xs" color={mutedTextColor}>
                        {selectedRequest.vendor.code}
                      </Text>
                    )}
                  </Box>
	                  <Box>
	                    <Text fontSize="sm" color={mutedTextColor}>
	                      Tanggal Pengajuan
	                    </Text>
	                    <Text fontWeight="medium">
	                      {new Date(selectedRequest.created_at).toLocaleString()}
	                    </Text>
	                  </Box>
		                  <Box>
		                    <Text fontSize="sm" color={mutedTextColor}>
		                      Realisasi Pembelian/Pembayaran
		                    </Text>
		                    <Box mt={1}>
		                      {renderPaymentProofRealization(selectedRequest)}
		                    </Box>
			                  </Box>
		                  <Box>
		                    <Text fontSize="sm" color={mutedTextColor}>
		                      Due Date Pembayaran
		                    </Text>
			                    <Box mt={1}>
			                      {renderPurchaseDueDate(selectedRequest)}
			                    </Box>
			                    {(selectedRequest.vendor_invoice_received_date ||
			                      selectedRequest.payment_terms_days != null) && (
			                      <Text fontSize="xs" color={mutedTextColor} mt={1}>
			                        Invoice diterima{" "}
			                        {formatDisplayDate(
			                          selectedRequest.vendor_invoice_received_date,
			                        ) || "-"}
			                        {selectedRequest.payment_terms_days != null
			                          ? `, termin ${selectedRequest.payment_terms_days} hari`
			                          : ""}
			                      </Text>
			                    )}
			                  </Box>
                        <Box>
                          <Text fontSize="sm" color={mutedTextColor}>
                            Invoice Aktual Vendor
                          </Text>
                          <Flex mt={1} direction="column" gap={1} align="flex-start">
                            {getVendorInvoiceStatusBadge(selectedRequest)}
                            {selectedRequest.vendor_invoice_requested_at && (
                              <Text fontSize="xs" color={mutedTextColor}>
                                Ditandai Finance Staff pada{" "}
                                {new Date(
                                  selectedRequest.vendor_invoice_requested_at,
                                ).toLocaleString()}
                              </Text>
                            )}
                            {selectedRequest.vendor_invoice_submitted_at && (
                              <Text fontSize="xs" color={mutedTextColor}>
                                Invoice dikirim pada{" "}
                                {new Date(
                                  selectedRequest.vendor_invoice_submitted_at,
                                ).toLocaleString()}
                              </Text>
                            )}
                          </Flex>
                        </Box>
                        {(selectedRequest.revision_opened_at ||
                          selectedRequest.reapproval_reason ===
                            "REQUESTER_REVISION") && (
                          <Box>
                            <Text fontSize="sm" color={mutedTextColor}>
                              Revisi PIC
                            </Text>
                            <Flex
                              mt={1}
                              direction="column"
                              gap={1}
                              align="flex-start"
                            >
                              <Badge colorScheme="purple">
                                {selectedRequest.status === "AWAITING_REVISION"
                                  ? "Menunggu PIC Submit"
                                  : selectedRequest.status === "PENDING_APPROVAL"
                                    ? "Dalam Review Ulang"
                                    : "Riwayat Revisi"}
                              </Badge>
                              {selectedRequest.revision_opener && (
                                <Text fontSize="xs" color={mutedTextColor}>
                                  Dibuka oleh{" "}
                                  {selectedRequest.revision_opener.first_name}{" "}
                                  {selectedRequest.revision_opener.last_name}
                                  {selectedRequest.revision_opened_at &&
                                    ` pada ${new Date(selectedRequest.revision_opened_at).toLocaleString()}`}
                                </Text>
                              )}
                              {selectedRequest.revision_reason && (
                                <Text fontSize="xs" color={mutedTextColor}>
                                  Alasan: {selectedRequest.revision_reason}
                                </Text>
                              )}
                            </Flex>
                          </Box>
                        )}
			                  {canEditSelectedRequestInvoiceTerms && (
			                    <Box gridColumn={{ base: "auto", md: "1 / -1" }}>
			                      <Text fontSize="sm" color={mutedTextColor} mb={2}>
			                        Koreksi Due Date PR
			                      </Text>
			                      <Grid
			                        templateColumns={{
			                          base: "1fr",
			                          md: "1.2fr 1fr 1fr auto",
			                        }}
			                        gap={2}
			                        alignItems="end"
			                      >
			                        <FormControl>
			                          <FormLabel fontSize="xs" color={mutedTextColor}>
			                            Tgl Terima Invoice
			                          </FormLabel>
			                          <Input
			                            type="date"
			                            size="sm"
			                            value={normalizeDateInputValue(
			                              invoiceTermsUpdateReceivedDate,
			                            )}
			                            onChange={(e) =>
			                              setInvoiceTermsUpdateReceivedDate(
			                                normalizeDateInputValue(
			                                  e.currentTarget.value,
			                                ),
			                              )
			                            }
			                            bg={inputBg}
			                            borderColor={inputBorderColor}
			                            color={textColor}
			                            _hover={{ borderColor: inputBorderColor }}
			                            _focusVisible={fieldFocusStyles}
			                          />
			                          <Text fontSize="xs" color={mutedTextColor} mt={1}>
			                            Ketuk field untuk membuka kalender.
			                          </Text>
			                        </FormControl>
			                        <FormControl>
			                          <FormLabel fontSize="xs" color={mutedTextColor}>
			                            Termin (hari)
			                          </FormLabel>
			                          <Input
			                            type="number"
			                            min={0}
			                            size="sm"
			                            value={invoiceTermsUpdateDays}
			                            onChange={(e) =>
			                              setInvoiceTermsUpdateDays(
			                                e.target.value.replace(/[^0-9]/g, ""),
			                              )
			                            }
			                            bg={inputBg}
			                            borderColor={inputBorderColor}
			                            color={textColor}
			                            _hover={{ borderColor: inputBorderColor }}
			                            _focusVisible={fieldFocusStyles}
			                          />
			                        </FormControl>
			                        <FormControl>
			                          <FormLabel fontSize="xs" color={mutedTextColor}>
			                            Due Date
			                          </FormLabel>
			                          <Input
			                            size="sm"
			                            value={
			                              selectedRequestDueDateSummary.requiresDueDate
			                                ? formatDateInputDisplayValue(
			                                    invoiceTermsUpdateDueDatePreview,
			                                  ) || "-"
			                                : "Tidak diperlukan"
			                            }
			                            isReadOnly
			                            bg={inputBg}
			                            borderColor={inputBorderColor}
			                            color={textColor}
			                          />
			                        </FormControl>
			                        <Button
			                          leftIcon={<FiCheck />}
			                          size="sm"
			                          colorScheme="blue"
			                          onClick={handleUpdateInvoiceTerms}
			                          isLoading={isUpdatingInvoiceTerms}
			                        >
			                          Simpan
			                        </Button>
			                      </Grid>
			                    </Box>
			                  )}
			                  {getStageLabel(selectedRequest) && (
		                    <Box>
	                      <Text fontSize="sm" color={mutedTextColor}>
                        Tahap Review
                      </Text>
                      <Text fontWeight="medium">
                        {getStageLabel(selectedRequest)}
                      </Text>
                    </Box>
                  )}
	                  {(selectedRequest.project || canEditSelectedRequestProject) && (
	                    <Box>
	                      <Text fontSize="sm" color={mutedTextColor}>
	                        Dialokasikan ke Proyek
	                      </Text>
	                      {canEditSelectedRequestProject ? (
	                        <Flex
	                          mt={1}
	                          gap={2}
	                          align={{ base: "stretch", sm: "center" }}
	                          direction={{ base: "column", sm: "row" }}
	                        >
	                          <Select
	                            size="sm"
	                            value={projectUpdateId}
	                            onChange={(e) => setProjectUpdateId(e.target.value)}
	                            placeholder="Belum dipilih"
	                            bg={inputBg}
	                            borderColor={inputBorderColor}
	                            color={textColor}
	                            _hover={{ borderColor: inputBorderColor }}
	                            _focusVisible={fieldFocusStyles}
	                          >
	                            {projects.map((p) => (
	                              <option key={p.id} value={p.id}>
	                                {p.code} - {p.name}
	                              </option>
	                            ))}
	                          </Select>
	                          <Button
	                            leftIcon={<FiCheck />}
	                            size="sm"
	                            colorScheme="purple"
	                            variant="outline"
	                            onClick={handleUpdateProject}
	                            isLoading={isUpdatingProject}
	                            isDisabled={
	                              projectUpdateId ===
	                              (selectedRequest.project_id?.toString() || "")
	                            }
	                          >
	                            Simpan
	                          </Button>
	                        </Flex>
	                      ) : (
	                        <Text fontWeight="medium" color="purple.600">
	                          {selectedRequest.project?.code} -{" "}
	                          {selectedRequest.project?.name}
	                        </Text>
	                      )}
	                    </Box>
	                  )}
                  {selectedRequest.finance_staff_approver && (
                    <Box>
                      <Text fontSize="sm" color={mutedTextColor}>
                        Review Finance Staff/Accounting
                      </Text>
	                      <Text fontWeight="medium" color="orange.600">
	                        {selectedRequest.finance_staff_approver.first_name}{" "}
	                        {selectedRequest.finance_staff_approver.last_name}
	                        {selectedRequest.finance_staff_approved_at &&
	                          ` pada ${new Date(selectedRequest.finance_staff_approved_at).toLocaleString()}`}
	                      </Text>
	                      <Flex mt={1} gap={2} align="center" wrap="wrap">
	                        {getStageSummaryBadge(
	                          getStageDecisionSummary(
	                            selectedRequest,
	                            "finance_staff",
	                          ),
	                        )}
	                        <Text fontSize="xs" color={mutedTextColor}>
	                          {getStageSummaryText(
	                            getStageDecisionSummary(
	                              selectedRequest,
	                              "finance_staff",
	                            ),
	                          )}
	                        </Text>
	                      </Flex>
	                      {getReviewNotes(selectedRequest, "finance_staff") && (
	                        <Text fontSize="xs" color={mutedTextColor} mt={1}>
	                          Catatan:{" "}
	                          {getReviewNotes(selectedRequest, "finance_staff")}
	                        </Text>
	                      )}
	                    </Box>
	                  )}
	                  {selectedRequest.finance_manager_approver && (
                    <Box>
                      <Text fontSize="sm" color={mutedTextColor}>
                        Review Finance Manager
                      </Text>
	                      <Text fontWeight="medium" color="green.600">
	                        {selectedRequest.finance_manager_approver.first_name}{" "}
	                        {selectedRequest.finance_manager_approver.last_name}
	                        {selectedRequest.finance_manager_approved_at &&
	                          ` pada ${new Date(selectedRequest.finance_manager_approved_at).toLocaleString()}`}
	                      </Text>
	                      <Flex mt={1} gap={2} align="center" wrap="wrap">
	                        {getStageSummaryBadge(
	                          getStageDecisionSummary(
	                            selectedRequest,
	                            "finance_manager",
	                          ),
	                        )}
	                        <Text fontSize="xs" color={mutedTextColor}>
	                          {getStageSummaryText(
	                            getStageDecisionSummary(
	                              selectedRequest,
	                              "finance_manager",
	                            ),
	                          )}
	                        </Text>
	                      </Flex>
	                      {getReviewNotes(selectedRequest, "finance_manager") && (
	                        <Text fontSize="xs" color={mutedTextColor} mt={1}>
	                          Catatan:{" "}
	                          {getReviewNotes(selectedRequest, "finance_manager")}
	                        </Text>
	                      )}
	                    </Box>
	                  )}
                  {selectedRequest.director_approver && (
                    <Box>
                      <Text fontSize="sm" color={mutedTextColor}>
                        Review Director
                      </Text>
                      <Text fontWeight="medium" color="purple.600">
                        {selectedRequest.director_approver.first_name}{" "}
                        {selectedRequest.director_approver.last_name}
                        {selectedRequest.director_approved_at &&
                          ` pada ${new Date(selectedRequest.director_approved_at).toLocaleString()}`}
                      </Text>
                    </Box>
                  )}
                  {selectedRequest.status === "APPROVED" &&
                    selectedRequest.approver && (
                      <Box>
                        <Text fontSize="sm" color={mutedTextColor}>
                          Disetujui Oleh
                        </Text>
                        <Text fontWeight="medium" color="green.600">
                          {selectedRequest.approver.first_name}{" "}
                          {selectedRequest.approver.last_name}
                          {selectedRequest.approved_at &&
                            ` pada ${new Date(selectedRequest.approved_at).toLocaleString()}`}
                        </Text>
                      </Box>
                    )}
                  {selectedRequest.status === "CANCELLED" && (
                    <Box gridColumn={{ base: "auto", md: "1 / -1" }}>
                      <Text fontSize="sm" color={mutedTextColor}>
                        Dibatalkan
                      </Text>
                      <Text fontWeight="medium" color="orange.600">
                        {selectedRequest.canceller
                          ? `${selectedRequest.canceller.first_name} ${selectedRequest.canceller.last_name}`.trim()
                          : selectedRequest.cancelled_by
                            ? `User ID ${selectedRequest.cancelled_by}`
                            : "-"}
                        {selectedRequest.cancelled_at &&
                          ` pada ${new Date(selectedRequest.cancelled_at).toLocaleString()}`}
                      </Text>
                      {selectedRequest.cancelled_reason && (
                        <Text fontSize="sm" color={mutedTextColor} mt={1}>
                          Alasan: {selectedRequest.cancelled_reason}
                        </Text>
                      )}
                    </Box>
                  )}
                </Grid>
              )}

              <Flex
                justify="space-between"
                align="center"
                mb={3}
                wrap="wrap"
                gap={2}
              >
                <Text fontWeight="bold">Rincian Item</Text>
                <Flex gap={2} wrap="wrap">
                  <Badge colorScheme="green">
                    {selectedRequestSummary.approved} Approved
                  </Badge>
                  <Badge colorScheme="red">
                    {selectedRequestSummary.rejected} Rejected
                  </Badge>
                  <Badge colorScheme="gray">
                    {selectedRequestSummary.pending} Pending
                  </Badge>
                </Flex>
              </Flex>
              <Box
                borderWidth="1px"
                borderColor={borderColor}
                rounded="md"
                overflow="hidden"
                mb={4}
              >
                <TableContainer w="full" maxW="100%" overflowX="auto" pb={1}>
                  <Table
                    variant="striped"
                    size="sm"
                    minW={
                      canCurrentUserReviewRequest(selectedRequest)
                        ? "1240px"
                        : "1040px"
                    }
                    mb={0}
                  >
                    <Thead>
                      <Tr>
                        <Th minW="220px">Item (Benchmark)</Th>
                        <Th isNumeric minW="70px">Qty</Th>
                        <Th isNumeric minW="150px">Est. Harga Satuan</Th>
                        <Th isNumeric minW="150px">Total Est.</Th>
                        <Th minW="95px">Staff</Th>
                        <Th minW="95px">Manager</Th>
                        <Th minW="95px">Director</Th>
                        <Th minW="95px">Final</Th>
                        <Th minW="125px">Purchase</Th>
                        {canCurrentUserReviewRequest(selectedRequest) && (
                          <Th minW="220px">Keputusan</Th>
                        )}
                      </Tr>
                    </Thead>
                    <Tbody>
                      {selectedRequest?.items?.map((it) => (
                        <Tr key={it.id}>
                          <Td verticalAlign="top">
                            <Text fontWeight="medium">
                              {it.product?.name || it.item_name}
                            </Text>
                            {!it.product_id && (
                              <Badge colorScheme="gray" mt={1}>
                                Input Manual
                              </Badge>
                            )}
                            {it.notes && (
                              <Text fontSize="xs" color={mutedTextColor}>
                                {it.notes}
                              </Text>
                            )}
                          </Td>
                          <Td isNumeric verticalAlign="top" whiteSpace="nowrap">
                            {it.quantity}
                          </Td>
                          <Td isNumeric verticalAlign="top" whiteSpace="nowrap">
                            {formatCurrency(it.estimated_price)}
                          </Td>
                          <Td
                            isNumeric
                            fontWeight="semibold"
                            verticalAlign="top"
                            whiteSpace="nowrap"
                          >
                            {formatCurrency(it.total_estimated_price)}
                          </Td>
                          <Td verticalAlign="top">
                            {getDecisionBadge(it.finance_staff_decision)}
                          </Td>
                          <Td verticalAlign="top">
                            {getDecisionBadge(it.finance_manager_decision)}
                          </Td>
                          <Td verticalAlign="top">
                            {getDecisionBadge(it.director_decision)}
                          </Td>
                          <Td verticalAlign="top">
                            {getDecisionBadge(it.approval_status)}
                          </Td>
	                          <Td verticalAlign="top" whiteSpace="nowrap">
	                            {it.purchase_id ? (
	                              <Flex direction="column" gap={1} align="flex-start">
	                                <Badge colorScheme="blue">Sudah Dipilih</Badge>
	                                {formatDisplayDate(it.purchase?.due_date) && (
	                                  <Text fontSize="xs" color={mutedTextColor}>
	                                    Due {formatDisplayDate(it.purchase?.due_date)}
	                                  </Text>
	                                )}
	                              </Flex>
	                            ) : (
	                              <Badge colorScheme="gray">Belum Dipakai</Badge>
	                            )}
                          </Td>
                          {canCurrentUserReviewRequest(selectedRequest) && (
                            <Td verticalAlign="top">
                              <Box minW="190px">
                                <Select
                                  size="sm"
                                  value={reviewDecisions[it.id] || ""}
                                  onChange={(e) =>
                                    setReviewDecisions((prev) => ({
                                      ...prev,
                                      [it.id]: e.target.value as
                                        | "APPROVED"
                                        | "REJECTED"
                                        | "",
                                    }))
                                  }
                                  bg={inputBg}
                                  borderColor={inputBorderColor}
                                  color={textColor}
                                  _hover={{ borderColor: inputBorderColor }}
                                  _focusVisible={fieldFocusStyles}
                                >
                                  <option value="">Pilih keputusan</option>
                                  <option value="APPROVED">Approve</option>
                                  <option value="REJECTED">Reject</option>
                                </Select>
                              </Box>
                            </Td>
                          )}
                        </Tr>
                      ))}
                      <Tr>
                        <Td
                          colSpan={
                            canCurrentUserReviewRequest(selectedRequest) ? 9 : 8
                          }
                          textAlign="right"
                          fontWeight="bold"
                        >
                          Total Pagu Request:
                        </Td>
                        <Td
                          isNumeric
                          fontWeight="bold"
                          fontSize="md"
                          color="blue.600"
                          minW="170px"
                          whiteSpace="nowrap"
                        >
                          {formatCurrency(selectedRequestSummary.activeTotal)}
                        </Td>
                      </Tr>
                    </Tbody>
                  </Table>
                </TableContainer>
              </Box>

              {selectedRequest?.notes && (
                <Box mt={4}>
                  <Text fontSize="sm" fontWeight="bold">
                    Catatan:
                  </Text>
                  <Text
                    mt={1}
                    p={3}
                    bg={notePanelBg}
                    color={notePanelColor}
                    rounded="md"
                    fontSize="sm"
                  >
                    {selectedRequest.notes}
                  </Text>
                </Box>
              )}

              {selectedRequest?.reapproval_reason === "REQUESTER_REVISION" && (
                <Box
                  mt={4}
                  borderWidth="1px"
                  borderColor={borderColor}
                  rounded="md"
                  p={4}
                  bg={infoPanelBg}
                >
                  <Flex
                    justify="space-between"
                    align={{ base: "flex-start", md: "center" }}
                    direction={{ base: "column", md: "row" }}
                    gap={2}
                    mb={3}
                  >
                    <Flex align="center" gap={2} wrap="wrap">
                      <Icon as={FiEdit3} color="purple.500" />
                      <Text fontWeight="bold">Revisi PR oleh PIC</Text>
                      <Badge colorScheme="purple">
                        {selectedRequest.status === "AWAITING_REVISION"
                          ? "Menunggu PIC"
                          : "Review Ulang"}
                      </Badge>
                    </Flex>
                    {selectedRequest.revision_submitted_at && (
                      <Text fontSize="xs" color={mutedTextColor}>
                        Dikirim pada{" "}
                        {new Date(
                          selectedRequest.revision_submitted_at,
                        ).toLocaleString()}
                      </Text>
                    )}
                  </Flex>

                  {selectedRequest.status === "AWAITING_REVISION" &&
                  canSubmitSelectedRequesterRevision ? (
                    <Box>
                      <Text fontSize="sm" color={mutedTextColor} mb={3}>
                        Perbarui daftar item dan nominal sesuai informasi
                        terbaru. Dokumen baru dapat ditambahkan bila diperlukan;
                        catatan revisi wajib diisi untuk jejak audit.
                      </Text>
                      <TableContainer
                        overflowX="auto"
                        borderWidth="1px"
                        borderColor={borderColor}
                        rounded="md"
                      >
                        <Table size="sm" minW="900px">
                          <Thead bg={tableHeaderBg}>
                            <Tr>
                              <Th>Item</Th>
                              <Th isNumeric w="120px">
                                Qty
                              </Th>
                              <Th isNumeric w="190px">
                                Harga Revisi
                              </Th>
                              <Th isNumeric w="160px">
                                Total
                              </Th>
                              <Th minW="180px">Catatan</Th>
                              <Th w="56px" />
                            </Tr>
                          </Thead>
                          <Tbody>
                            {requesterRevisionItems.map((item) => (
                              <Tr key={item.client_id}>
                                <Td verticalAlign="top">
                                  {item.item_id ? (
                                    <Box>
                                      <Text fontWeight="medium" noOfLines={2}>
                                        {item.item_name}
                                      </Text>
                                      <Text fontSize="xs" color={mutedTextColor}>
                                        Item sebelumnya
                                      </Text>
                                    </Box>
                                  ) : (
                                    <Box>
                                      <Flex align="center" gap={2} mb={2}>
                                        <Badge colorScheme="blue">Item Baru</Badge>
                                        <ButtonGroup
                                          isAttached
                                          size="xs"
                                          variant="outline"
                                        >
                                          <Button
                                            leftIcon={<FiPackage />}
                                            colorScheme={
                                              item.source === "catalog"
                                                ? "blue"
                                                : "gray"
                                            }
                                            variant={
                                              item.source === "catalog"
                                                ? "solid"
                                                : "outline"
                                            }
                                            onClick={() =>
                                              handleRequesterRevisionItemSourceChange(
                                                item.client_id,
                                                "catalog",
                                              )
                                            }
                                          >
                                            Katalog
                                          </Button>
                                          <Button
                                            leftIcon={<FiEdit3 />}
                                            colorScheme={
                                              item.source === "manual"
                                                ? "blue"
                                                : "gray"
                                            }
                                            variant={
                                              item.source === "manual"
                                                ? "solid"
                                                : "outline"
                                            }
                                            onClick={() =>
                                              handleRequesterRevisionItemSourceChange(
                                                item.client_id,
                                                "manual",
                                              )
                                            }
                                          >
                                            Manual
                                          </Button>
                                        </ButtonGroup>
                                      </Flex>
                                      {item.source === "catalog" ? (
                                        <Select
                                          placeholder="Pilih item/barang"
                                          value={item.product_id}
                                          onChange={(event) =>
                                            handleRequesterRevisionItemChange(
                                              item.client_id,
                                              "product_id",
                                              event.target.value,
                                            )
                                          }
                                          bg={inputBg}
                                          borderColor={inputBorderColor}
                                          color={textColor}
                                          _hover={{ borderColor: inputBorderColor }}
                                          _focusVisible={fieldFocusStyles}
                                        >
                                          {products.map((product) => (
                                            <option key={product.id} value={product.id}>
                                              {product.code} - {product.name}
                                            </option>
                                          ))}
                                        </Select>
                                      ) : (
                                        <Input
                                          placeholder="Nama kebutuhan"
                                          value={item.item_name}
                                          onChange={(event) =>
                                            handleRequesterRevisionItemChange(
                                              item.client_id,
                                              "item_name",
                                              event.target.value,
                                            )
                                          }
                                          maxLength={255}
                                          bg={inputBg}
                                          borderColor={inputBorderColor}
                                          color={textColor}
                                          _hover={{ borderColor: inputBorderColor }}
                                          _focusVisible={fieldFocusStyles}
                                          _placeholder={{ color: mutedTextColor }}
                                        />
                                      )}
                                    </Box>
                                  )}
                                </Td>
                                <Td verticalAlign="top">
                                  <Input
                                    type="number"
                                    min={1}
                                    size="sm"
                                    textAlign="right"
                                    value={item.quantity}
                                    onChange={(e) =>
                                      handleRequesterRevisionItemChange(
                                        item.client_id,
                                        "quantity",
                                        e.currentTarget.value,
                                      )
                                    }
                                    bg={inputBg}
                                    borderColor={inputBorderColor}
                                    color={textColor}
                                    _hover={{ borderColor: inputBorderColor }}
                                    _focusVisible={fieldFocusStyles}
                                  />
                                </Td>
                                <Td verticalAlign="top">
                                  <CurrencyInput
                                    value={Number(item.estimated_price) || 0}
                                    onChange={(value) =>
                                      handleRequesterRevisionItemChange(
                                        item.client_id,
                                        "estimated_price",
                                        value.toString(),
                                      )
                                    }
                                    size="sm"
                                    showLabel={false}
                                    allowDecimals
                                    decimalScale={2}
                                  />
                                </Td>
                                <Td
                                  isNumeric
                                  verticalAlign="top"
                                  whiteSpace="nowrap"
                                >
                                  {formatCurrency(
                                    (Number(item.quantity) || 0) *
                                      (Number(item.estimated_price) || 0),
                                  )}
                                </Td>
                                <Td verticalAlign="top">
                                  <Input
                                    size="sm"
                                    value={item.notes}
                                    onChange={(e) =>
                                      handleRequesterRevisionItemChange(
                                        item.client_id,
                                        "notes",
                                        e.currentTarget.value,
                                      )
                                    }
                                    placeholder="Catatan revisi"
                                    bg={inputBg}
                                    borderColor={inputBorderColor}
                                    color={textColor}
                                    _hover={{ borderColor: inputBorderColor }}
                                    _focusVisible={fieldFocusStyles}
                                  />
                                </Td>
                                <Td verticalAlign="top" textAlign="right">
                                  <IconButton
                                    aria-label={`Hapus item ${item.item_name || "baru"}`}
                                    icon={<FiTrash2 />}
                                    size="sm"
                                    colorScheme="red"
                                    variant="ghost"
                                    onClick={() =>
                                      handleRemoveRequesterRevisionItem(
                                        item.client_id,
                                      )
                                    }
                                  />
                                </Td>
                              </Tr>
                            ))}
                          </Tbody>
                        </Table>
                      </TableContainer>

                      <Button
                        leftIcon={<FiPlus />}
                        size="sm"
                        mt={3}
                        onClick={handleAddRequesterRevisionItem}
                      >
                        Tambah Item
                      </Button>

                      <FormControl mt={3} isRequired>
                        <FormLabel color={textColor}>Catatan Revisi</FormLabel>
                        <Textarea
                          value={requesterRevisionNotes}
                          onChange={(e) =>
                            setRequesterRevisionNotes(e.target.value)
                          }
                          placeholder="Jelaskan alasan perubahan item atau nominal PR."
                          rows={3}
                          bg={inputBg}
                          borderColor={inputBorderColor}
                          color={textColor}
                          _hover={{ borderColor: inputBorderColor }}
                          _focusVisible={fieldFocusStyles}
                          _placeholder={{ color: mutedTextColor }}
                        />
                      </FormControl>

                      <Box mt={3}>
                        <Button
                          as="label"
                          leftIcon={<FiPaperclip />}
                          size="sm"
                          variant="outline"
                          colorScheme="purple"
                          cursor="pointer"
                        >
                          Tambah Dokumen (Opsional)
                          <Input
                            type="file"
                            multiple
                            accept={ACCEPTED_DOCUMENT_TYPES}
                            display="none"
                            onChange={handleRequesterRevisionDocumentChange}
                          />
                        </Button>
                        <Text fontSize="xs" color={mutedTextColor} mt={1}>
                          Opsional. Dokumen yang sudah ada tetap tersimpan.
                          Format PDF, Word, Excel, CSV, TXT, JPG, PNG, atau
                          WebP. Maksimal 50MB per file.
                        </Text>
                      </Box>

                      {requesterRevisionDocuments.length > 0 && (
                        <Box mt={3}>
                          {requesterRevisionDocuments.map((file, index) => (
                            <Flex
                              key={`${file.name}-${file.size}-${index}`}
                              align="center"
                              justify="space-between"
                              gap={3}
                              py={2}
                              borderTopWidth={index === 0 ? "0" : "1px"}
                              borderColor={borderColor}
                            >
                              <Flex align="center" gap={2} minW={0}>
                                <Icon as={FiFileText} color="purple.500" />
                                <Box minW={0}>
                                  <Text
                                    fontSize="sm"
                                    fontWeight="medium"
                                    noOfLines={1}
                                  >
                                    {file.name}
                                  </Text>
                                  <Text fontSize="xs" color={mutedTextColor}>
                                    {formatFileSize(file.size)}
                                  </Text>
                                </Box>
                              </Flex>
                              <IconButton
                                aria-label={`Hapus dokumen revisi ${file.name}`}
                                icon={<FiX />}
                                size="sm"
                                variant="ghost"
                                colorScheme="red"
                                onClick={() =>
                                  handleRemoveRequesterRevisionDocument(index)
                                }
                              />
                            </Flex>
                          ))}
                        </Box>
                      )}

                      <Flex justify="flex-end" mt={4}>
                        <Button
                          colorScheme="purple"
                          leftIcon={<FiCheck />}
                          onClick={handleSubmitRequesterRevision}
                          isLoading={isSubmittingRequesterRevision}
                        >
                          Submit Revisi untuk Approval
                        </Button>
                      </Flex>
                    </Box>
                  ) : (
                    <Box p={3} rounded="md" bg={notePanelBg} color={notePanelColor}>
                      <Text fontSize="sm" fontWeight="semibold">
                        {selectedRequest.status === "AWAITING_REVISION"
                          ? "PR sedang menunggu PIC/Requester mengirim revisi."
                          : `Revisi PR sedang menunggu approval ulang: ${
                              getStageLabel(selectedRequest) || "Finance"
                            }.`}
                      </Text>
                      {selectedRequest.revision_reason && (
                        <Text fontSize="xs" mt={1}>
                          Alasan buka revisi: {selectedRequest.revision_reason}
                        </Text>
                      )}
                    </Box>
                  )}
                </Box>
              )}

              {(selectedRequest?.revision_snapshots || []).length > 0 && (
                <Box
                  mt={4}
                  borderWidth="1px"
                  borderColor={borderColor}
                  rounded="md"
                  p={4}
                  bg={infoPanelBg}
                >
                  <Flex align="center" gap={2} mb={3} wrap="wrap">
                    <Icon as={FiFileText} color="purple.500" />
                    <Text fontWeight="bold">Riwayat Revisi PR</Text>
                    <Badge colorScheme="purple">
                      {selectedRequest?.revision_snapshots?.length || 0} revisi
                    </Badge>
                  </Flex>
                  <Flex direction="column" gap={3}>
                    {(selectedRequest?.revision_snapshots || []).map(
                      (snapshot) => (
                        <Box
                          key={snapshot.id}
                          borderWidth="1px"
                          borderColor={borderColor}
                          rounded="md"
                          p={3}
                        >
                          <Flex
                            justify="space-between"
                            align={{ base: "flex-start", md: "center" }}
                            direction={{ base: "column", md: "row" }}
                            gap={2}
                          >
                            <Box>
                              <Text fontWeight="semibold">
                                Revision {snapshot.revision_number}
                              </Text>
                              <Text fontSize="xs" color={mutedTextColor}>
                                Dibuka{" "}
                                {new Date(snapshot.opened_at).toLocaleString()}
                                {snapshot.opener
                                  ? ` oleh ${snapshot.opener.first_name} ${snapshot.opener.last_name}`
                                  : ""}
                              </Text>
                            </Box>
                            <Flex gap={2} wrap="wrap">
                              <Badge colorScheme="gray">
                                Lama{" "}
                                {formatCurrency(
                                  snapshot.previous_total_estimated_amount,
                                )}
                              </Badge>
                              {snapshot.new_total_estimated_amount != null && (
                                <Badge colorScheme="purple">
                                  Baru{" "}
                                  {formatCurrency(
                                    snapshot.new_total_estimated_amount,
                                  )}
                                </Badge>
                              )}
                            </Flex>
                          </Flex>
                          {snapshot.open_reason && (
                            <Text fontSize="sm" color={mutedTextColor} mt={2}>
                              Alasan: {snapshot.open_reason}
                            </Text>
                          )}
                          {snapshot.submitted_notes && (
                            <Text fontSize="sm" color={mutedTextColor} mt={1}>
                              Catatan PIC: {snapshot.submitted_notes}
                            </Text>
                          )}
                        </Box>
                      ),
                    )}
                  </Flex>
                </Box>
              )}

              {selectedRequest?.requires_vendor_invoice && (
                <Box
                  mt={4}
                  borderWidth="1px"
                  borderColor={borderColor}
                  rounded="md"
                  p={4}
                  bg={infoPanelBg}
                >
                  <Flex
                    justify="space-between"
                    align={{ base: "flex-start", md: "center" }}
                    direction={{ base: "column", md: "row" }}
                    gap={2}
                    mb={3}
                  >
                    <Flex align="center" gap={2} wrap="wrap">
                      <Icon as={FiEdit3} color="purple.500" />
                      <Text fontWeight="bold">Revisi Invoice Aktual</Text>
                      {getVendorInvoiceStatusBadge(selectedRequest)}
                    </Flex>
                    {selectedRequest.vendor_invoice_submitter && (
                      <Text fontSize="xs" color={mutedTextColor}>
                        Terakhir dikirim oleh{" "}
                        {selectedRequest.vendor_invoice_submitter.first_name}{" "}
                        {selectedRequest.vendor_invoice_submitter.last_name}
                      </Text>
                    )}
                  </Flex>

                  {selectedRequest.status === "PENDING_APPROVAL" &&
                  selectedRequest.reapproval_reason === "INVOICE_REVISION" ? (
                    <Box p={3} rounded="md" bg={notePanelBg} color={notePanelColor}>
                      <Text fontSize="sm" fontWeight="semibold">
                        Invoice aktual sedang menunggu approval ulang:{" "}
                        {getStageLabel(selectedRequest) || "Menunggu Finance"}.
                      </Text>
                      <Text fontSize="xs" mt={1}>
                        Nominal PR sudah dikirim ulang dan tidak bisa diubah
                        sampai review selesai.
                      </Text>
                    </Box>
                  ) : canSubmitSelectedRequestInvoiceRevision ? (
                    <Box>
                      <Text fontSize="sm" color={mutedTextColor} mb={3}>
                        Ubah nominal sesuai invoice asli vendor, lampirkan invoice,
                        lalu submit untuk approval ulang Finance Staff dan Finance
                        Manager.
                      </Text>
                      <TableContainer overflowX="auto" borderWidth="1px" borderColor={borderColor} rounded="md">
                        <Table size="sm" minW="760px">
                          <Thead bg={tableHeaderBg}>
                            <Tr>
                              <Th>Item</Th>
                              <Th isNumeric w="120px">Qty</Th>
                              <Th isNumeric w="190px">Harga Invoice</Th>
                              <Th isNumeric w="160px">Total</Th>
                              <Th minW="180px">Catatan</Th>
                            </Tr>
                          </Thead>
                          <Tbody>
                            {invoiceRevisionItems.map((item) => (
                              <Tr key={item.item_id}>
                                <Td verticalAlign="top">
                                  <Text fontWeight="medium" noOfLines={2}>
                                    {item.item_name}
                                  </Text>
                                </Td>
                                <Td verticalAlign="top">
                                  <Input
                                    type="number"
                                    min={1}
                                    size="sm"
                                    textAlign="right"
                                    value={item.quantity}
                                    onChange={(e) =>
                                      handleInvoiceRevisionItemChange(
                                        item.item_id,
                                        "quantity",
                                        e.currentTarget.value,
                                      )
                                    }
                                    bg={inputBg}
                                    borderColor={inputBorderColor}
                                    color={textColor}
                                    _hover={{ borderColor: inputBorderColor }}
                                    _focusVisible={fieldFocusStyles}
                                  />
                                </Td>
                                <Td verticalAlign="top">
                                  <CurrencyInput
                                    value={Number(item.estimated_price) || 0}
                                    onChange={(value) =>
                                      handleInvoiceRevisionItemChange(
                                        item.item_id,
                                        "estimated_price",
                                        value.toString(),
                                      )
                                    }
                                    size="sm"
                                    showLabel={false}
                                    allowDecimals
                                    decimalScale={2}
                                  />
                                </Td>
                                <Td isNumeric verticalAlign="top" whiteSpace="nowrap">
                                  {formatCurrency(
                                    (Number(item.quantity) || 0) *
                                      (Number(item.estimated_price) || 0),
                                  )}
                                </Td>
                                <Td verticalAlign="top">
                                  <Input
                                    size="sm"
                                    value={item.notes}
                                    onChange={(e) =>
                                      handleInvoiceRevisionItemChange(
                                        item.item_id,
                                        "notes",
                                        e.currentTarget.value,
                                      )
                                    }
                                    placeholder="Catatan invoice"
                                    bg={inputBg}
                                    borderColor={inputBorderColor}
                                    color={textColor}
                                    _hover={{ borderColor: inputBorderColor }}
                                    _focusVisible={fieldFocusStyles}
                                  />
                                </Td>
                              </Tr>
                            ))}
                          </Tbody>
                        </Table>
                      </TableContainer>

                      <FormControl mt={3}>
                        <FormLabel color={textColor}>Catatan Revisi</FormLabel>
                        <Textarea
                          value={invoiceRevisionNotes}
                          onChange={(e) => setInvoiceRevisionNotes(e.target.value)}
                          placeholder="Opsional, jelaskan perbedaan invoice aktual dengan estimasi awal."
                          rows={3}
                          bg={inputBg}
                          borderColor={inputBorderColor}
                          color={textColor}
                          _hover={{ borderColor: inputBorderColor }}
                          _focusVisible={fieldFocusStyles}
                          _placeholder={{ color: mutedTextColor }}
                        />
                      </FormControl>

                      <Box mt={3}>
                        <Button
                          as="label"
                          leftIcon={<FiPaperclip />}
                          size="sm"
                          variant="outline"
                          colorScheme="purple"
                          cursor="pointer"
                        >
                          Pilih Invoice Aktual
                          <Input
                            type="file"
                            multiple
                            accept={ACCEPTED_DOCUMENT_TYPES}
                            display="none"
                            onChange={handleInvoiceRevisionDocumentChange}
                          />
                        </Button>
                        <Text fontSize="xs" color={mutedTextColor} mt={1}>
                          Wajib lampirkan invoice vendor. Format PDF, Word,
                          Excel, CSV, TXT, JPG, PNG, atau WebP. Maksimal 50MB
                          per file.
                        </Text>
                      </Box>

                      {invoiceRevisionDocuments.length > 0 && (
                        <Box mt={3}>
                          {invoiceRevisionDocuments.map((file, index) => (
                            <Flex
                              key={`${file.name}-${file.size}-${index}`}
                              align="center"
                              justify="space-between"
                              gap={3}
                              py={2}
                              borderTopWidth={index === 0 ? "0" : "1px"}
                              borderColor={borderColor}
                            >
                              <Flex align="center" gap={2} minW={0}>
                                <Icon as={FiFileText} color="purple.500" />
                                <Box minW={0}>
                                  <Text fontSize="sm" fontWeight="medium" noOfLines={1}>
                                    {file.name}
                                  </Text>
                                  <Text fontSize="xs" color={mutedTextColor}>
                                    {formatFileSize(file.size)}
                                  </Text>
                                </Box>
                              </Flex>
                              <IconButton
                                aria-label={`Hapus invoice ${file.name}`}
                                icon={<FiX />}
                                size="sm"
                                variant="ghost"
                                colorScheme="red"
                                onClick={() =>
                                  handleRemoveInvoiceRevisionDocument(index)
                                }
                              />
                            </Flex>
                          ))}
                        </Box>
                      )}

                      <Flex justify="flex-end" mt={4}>
                        <Button
                          leftIcon={<FiCheck />}
                          colorScheme="purple"
                          onClick={handleSubmitInvoiceRevision}
                          isLoading={isSubmittingInvoiceRevision}
                          loadingText="Mengirim..."
                        >
                          Submit Invoice Aktual
                        </Button>
                      </Flex>
                    </Box>
                  ) : (
                    <Box p={3} rounded="md" bg={notePanelBg} color={notePanelColor}>
                      <Text fontSize="sm" fontWeight="semibold">
                        Revisi invoice aktual tidak tersedia.
                      </Text>
                      <Text fontSize="xs" mt={1}>
                        Revisi hanya bisa dilakukan oleh pemohon/admin setelah PR
                        approved dan sebelum PR dibuat pembelian.
                      </Text>
                    </Box>
                  )}
                </Box>
              )}

              {(selectedRequestRealizationDocuments.length > 0 ||
                canUploadSelectedRequestRealizationDocuments) && (
                <Box
                  mt={4}
                  borderWidth="1px"
                  borderColor={borderColor}
                  rounded="md"
                  p={4}
                  bg={infoPanelBg}
                >
                  <Flex
                    justify="space-between"
                    align={{ base: "flex-start", md: "center" }}
                    direction={{ base: "column", md: "row" }}
                    gap={3}
                    mb={3}
                  >
                    <Flex align="center" gap={2}>
                      <Icon as={FiFileText} color="green.500" />
                      <Text fontWeight="bold">Dokumen Realisasi PIC</Text>
                      <Badge colorScheme="green">
                        {selectedRequestRealizationDocuments.length} file
                      </Badge>
                    </Flex>
                    {canUploadSelectedRequestRealizationDocuments && (
                      <Button
                        as="label"
                        leftIcon={<FiPaperclip />}
                        size="sm"
                        variant="outline"
                        colorScheme="green"
                        cursor="pointer"
                        isLoading={isUploadingRealizationDocuments}
                      >
                        Pilih Billing/Realisasi
                        <Input
                          type="file"
                          multiple
                          accept={ACCEPTED_DOCUMENT_TYPES}
                          display="none"
                          onChange={handleRealizationDocumentChange}
                        />
                      </Button>
                    )}
                  </Flex>

                  {selectedRequestRealizationDocuments.length > 0 ? (
                    <Box>
                      {selectedRequestRealizationDocuments.map(
                        (document, index) => (
                          <Flex
                            key={document.id}
                            align={{ base: "flex-start", md: "center" }}
                            justify="space-between"
                            direction={{ base: "column", md: "row" }}
                            gap={3}
                            py={3}
                            borderTopWidth={index === 0 ? "0" : "1px"}
                            borderColor={borderColor}
                          >
                            <Flex align="center" gap={3} minW={0}>
                              <Icon as={FiFileText} color="green.500" />
	                              <Box minW={0}>
	                                <Text fontWeight="medium" noOfLines={1}>
	                                  {document.file_name}
	                                </Text>
	                                <Text fontSize="xs" color={mutedTextColor}>
	                                  {getDocumentUploadMeta(document)}
	                                </Text>
	                                {getDocumentTypeBadge(document.document_type)}
	                              </Box>
                            </Flex>
                            <Button
                              onClick={() => handleDownloadDocument(document)}
                              leftIcon={<FiDownload />}
                              size="sm"
                              variant="outline"
                              colorScheme="green"
                            >
                              Buka
                            </Button>
                          </Flex>
                        ),
                      )}
                    </Box>
                  ) : (
                    <Text fontSize="sm" color={mutedTextColor}>
                      Belum ada dokumen billing/realisasi yang diunggah.
                    </Text>
                  )}

                  {pendingRealizationDocuments.length > 0 && (
                    <Box
                      mt={4}
                      pt={4}
                      borderTopWidth="1px"
                      borderColor={borderColor}
                    >
                      <Flex justify="space-between" align="center" gap={3} mb={2}>
                        <Box minW={0}>
                          <Text fontSize="sm" fontWeight="bold">
                            Draft Dokumen Realisasi
                          </Text>
                          <Text fontSize="xs" color={mutedTextColor}>
                            File akan disimpan sebagai bukti realisasi tanpa
                            approval ulang.
                          </Text>
                        </Box>
                        <Badge colorScheme="green">
                          {pendingRealizationDocuments.length} draft
                        </Badge>
                      </Flex>

                      {pendingRealizationDocuments.map((file, index) => (
                        <Flex
                          key={`${file.name}-${file.size}-${index}`}
                          align="center"
                          justify="space-between"
                          gap={3}
                          py={2}
                          borderTopWidth={index === 0 ? "0" : "1px"}
                          borderColor={borderColor}
                        >
                          <Flex align="center" gap={2} minW={0}>
                            <Icon as={FiFileText} color="green.500" />
                            <Box minW={0}>
                              <Text fontSize="sm" fontWeight="medium" noOfLines={1}>
                                {file.name}
                              </Text>
                              <Text fontSize="xs" color={mutedTextColor}>
                                {formatFileSize(file.size)}
                              </Text>
                            </Box>
                          </Flex>
                          <IconButton
                            aria-label={`Hapus realisasi ${file.name}`}
                            icon={<FiX />}
                            size="sm"
                            variant="ghost"
                            colorScheme="red"
                            onClick={() =>
                              handleRemovePendingRealizationDocument(index)
                            }
                          />
                        </Flex>
                      ))}

                      <Flex justify="flex-end" mt={3}>
                        <Button
                          leftIcon={<FiCheck />}
                          size="sm"
                          colorScheme="green"
                          onClick={handleUploadRealizationDocuments}
                          isLoading={isUploadingRealizationDocuments}
                          loadingText="Mengunggah..."
                        >
                          Upload Dokumen Realisasi
                        </Button>
                      </Flex>
                    </Box>
                  )}
                </Box>
              )}

              <Box
                mt={4}
                borderWidth="1px"
                borderColor={borderColor}
                rounded="md"
                p={4}
                bg={infoPanelBg}
              >
                <Flex
                  justify="space-between"
                  align={{ base: "flex-start", md: "center" }}
                  direction={{ base: "column", md: "row" }}
                  gap={3}
                  mb={3}
                >
                  <Flex align="center" gap={2}>
                    <Icon as={FiPaperclip} color="blue.500" />
                    <Text fontWeight="bold">Dokumen Pendukung</Text>
                    <Badge colorScheme="blue">
                      {selectedRequestSupportingDocuments.length} file
                    </Badge>
                  </Flex>
                  {selectedRequest &&
                    canEdit("purchase_requests") &&
                    selectedRequest.status !== "COMPLETED" &&
                    selectedRequest.status !== "REJECTED" &&
                    selectedRequest.status !== "CANCELLED" &&
                    selectedRequest.status !== "AWAITING_REVISION" &&
                    !(
                      selectedRequest.status === "PENDING_APPROVAL" &&
                      selectedRequest.reapproval_reason ===
                        "ADDITIONAL_DOCUMENTS"
                    ) && (
                    <Button
                      as="label"
                      leftIcon={<FiPaperclip />}
                      size="sm"
                      variant="outline"
                      colorScheme="blue"
                      cursor="pointer"
                      isLoading={isUploadingDocuments}
                    >
                      Pilih Dokumen Tambahan
                      <Input
                        type="file"
                        multiple
                        accept={ACCEPTED_DOCUMENT_TYPES}
                        display="none"
                        onChange={handleExistingDocumentChange}
                      />
                    </Button>
                  )}
                </Flex>

                {selectedRequest?.status === "PENDING_APPROVAL" &&
                  selectedRequest.reapproval_reason ===
                    "ADDITIONAL_DOCUMENTS" && (
                    <Box
                      mb={3}
                      p={3}
                      rounded="md"
                      bg={notePanelBg}
                      color={notePanelColor}
                    >
                      <Text fontSize="sm" fontWeight="semibold">
                        Dokumen tambahan sedang dalam approval ulang:{" "}
                        {getStageLabel(selectedRequest) || "Menunggu Finance"}.
                      </Text>
                      <Text fontSize="xs" mt={1}>
                        Dokumen baru tidak dapat ditambah atau dihapus sampai
                        proses approval ini selesai.
                      </Text>
                    </Box>
                  )}

                {selectedRequestSupportingDocuments.length ? (
                  <Box>
                    {selectedRequestSupportingDocuments.map((document, index) => (
                      <Flex
                        key={document.id}
                        align={{ base: "flex-start", md: "center" }}
                        justify="space-between"
                        direction={{ base: "column", md: "row" }}
                        gap={3}
                        py={3}
                        borderTopWidth={index === 0 ? "0" : "1px"}
                        borderColor={borderColor}
                      >
                        <Flex align="center" gap={3} minW={0}>
                          <Icon as={FiFileText} color="blue.500" />
                          <Box minW={0}>
	                            <Text fontWeight="medium" noOfLines={1}>
	                              {document.file_name}
	                            </Text>
	                            <Text fontSize="xs" color={mutedTextColor}>
	                              {getDocumentUploadMeta(document)}
	                            </Text>
                            {document.approval_revision > 1 && (
                              <Badge mt={1} colorScheme="orange">
                                Dokumen tambahan - Approval ulang
                              </Badge>
                            )}
                            {getDocumentTypeBadge(document.document_type)}
                          </Box>
                        </Flex>
                        <Button
                          onClick={() => handleDownloadDocument(document)}
                          leftIcon={<FiDownload />}
                          size="sm"
                          variant="outline"
                          colorScheme="blue"
                        >
                          Buka
                        </Button>
                      </Flex>
                    ))}
                  </Box>
                ) : (
                  <Text fontSize="sm" color={mutedTextColor}>
                    Belum ada dokumen pendukung untuk PR ini.
                  </Text>
                )}

                {pendingAdditionalDocuments.length > 0 && (
                  <Box
                    mt={4}
                    pt={4}
                    borderTopWidth="1px"
                    borderColor={borderColor}
                  >
                    <Flex justify="space-between" align="center" gap={3} mb={2}>
                      <Box>
                        <Text fontSize="sm" fontWeight="bold">
                          Draft Dokumen Tambahan
                        </Text>
                        <Text fontSize="xs" color={mutedTextColor}>
                          File belum dikirim. Hapus dengan tombol X bila perlu,
                          lalu submit untuk memulai approval ulang.
                        </Text>
                      </Box>
                      <Badge colorScheme="orange">
                        {pendingAdditionalDocuments.length} draft
                      </Badge>
                    </Flex>

                    {pendingAdditionalDocuments.map((file, index) => (
                      <Flex
                        key={`${file.name}-${file.size}-${index}`}
                        align="center"
                        justify="space-between"
                        gap={3}
                        py={2}
                        borderTopWidth={index === 0 ? "0" : "1px"}
                        borderColor={borderColor}
                      >
                        <Flex align="center" gap={2} minW={0}>
                          <Icon as={FiFileText} color="orange.500" />
                          <Box minW={0}>
                            <Text fontSize="sm" fontWeight="medium" noOfLines={1}>
                              {file.name}
                            </Text>
                            <Text fontSize="xs" color={mutedTextColor}>
                              {formatFileSize(file.size)}
                            </Text>
                          </Box>
                        </Flex>
                        <IconButton
                          aria-label={`Hapus draft ${file.name}`}
                          icon={<FiX />}
                          size="sm"
                          variant="ghost"
                          colorScheme="red"
                          onClick={() =>
                            handleRemovePendingAdditionalDocument(index)
                          }
                        />
                      </Flex>
                    ))}

                    <Flex justify="flex-end" mt={3}>
                      <Button
                        leftIcon={<FiCheck />}
                        size="sm"
                        colorScheme="blue"
                        onClick={handleSubmitAdditionalDocuments}
                        isLoading={isUploadingDocuments}
                        loadingText="Mengirim..."
                      >
                        Submit Dokumen Tambahan
                      </Button>
                    </Flex>
                  </Box>
                )}
              </Box>

              {canCurrentUserReviewRequest(selectedRequest) && (
                <Box
                  mt={4}
                  borderWidth="1px"
                  borderColor={borderColor}
                  rounded="md"
                  p={4}
                  bg={infoPanelBg}
                >
                  <Flex
                    gap={2}
                    mb={3}
                    wrap="wrap"
                    direction={{ base: "column", sm: "row" }}
                  >
                    <Button
                      size="sm"
                      variant="outline"
                      colorScheme="green"
                      w={{ base: "full", sm: "auto" }}
                      onClick={() => setAllReviewDecisions("APPROVED")}
                    >
                      Approve Semua
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      colorScheme="red"
                      w={{ base: "full", sm: "auto" }}
                      onClick={() => setAllReviewDecisions("REJECTED")}
                    >
                      Reject Semua
                    </Button>
                  </Flex>
                  {canOpenRequesterRevision(selectedRequest) && (
                    <Box
                      mb={3}
                      pt={3}
                      borderTopWidth="1px"
                      borderColor={borderColor}
                    >
                      <Button
                        leftIcon={<FiEdit3 />}
                        size="sm"
                        colorScheme="purple"
                        variant="outline"
                        w={{ base: "full", sm: "auto" }}
                        onClick={handleOpenRequesterRevisionRequest}
                      >
                        {isInitialRequesterRevisionRequest(selectedRequest)
                          ? "Minta Perbaikan PIC"
                          : "Buka Revisi PIC"}
                      </Button>
                      <Text fontSize="sm" color={mutedTextColor} mt={2}>
                        {isInitialRequesterRevisionRequest(selectedRequest)
                          ? "Kembalikan PR ke PIC untuk diperbaiki sebelum review dikirim. Alasan perbaikan wajib diisi."
                          : "Buka akses revisi PIC. Setelah disubmit, PR akan kembali ke Finance Staff untuk approval ulang."}
                      </Text>
                    </Box>
                  )}
                  <FormControl>
                    <FormLabel color={textColor}>Catatan Review</FormLabel>
                    <Textarea
                      placeholder="Opsional. Isi bila ada item yang ditolak atau ada catatan untuk tahap berikutnya."
                      value={reviewReason}
                      onChange={(e) => setReviewReason(e.target.value)}
                      rows={3}
                      bg={inputBg}
                      borderColor={inputBorderColor}
                      color={textColor}
                      _hover={{ borderColor: inputBorderColor }}
                      _focusVisible={fieldFocusStyles}
                      _placeholder={{ color: mutedTextColor }}
                    />
                  </FormControl>
                  {getCurrentReviewStage(selectedRequest) ===
                    "FINANCE_STAFF" && (
                    <Box mt={3}>
                      <Checkbox
                        isChecked={reviewRequiresVendorInvoice}
                        isDisabled={
                          Boolean(selectedRequest?.requires_vendor_invoice) ||
                          selectedRequest?.reapproval_reason ===
                            "INVOICE_REVISION"
                        }
                        onChange={(e) =>
                          setReviewRequiresVendorInvoice(e.target.checked)
                        }
                        colorScheme="purple"
                      >
                        Butuh invoice aktual dari PIC/Staff
                      </Checkbox>
                      <Text fontSize="sm" color={mutedTextColor} mt={1}>
                        Jika dicentang, setelah Finance Manager approve PIC/Staff
                        boleh upload invoice vendor dan revisi nominal PR untuk
                        approval ulang. Jika tidak dicentang, nominal PR terkunci
                        setelah approval final.
                      </Text>
                    </Box>
                  )}
                  {canRequestDirectorApproval(selectedRequest) && (
                    <Box mt={3}>
                      <Checkbox
                        isChecked={reviewNeedsDirectorApproval}
                        onChange={(e) =>
                          setReviewNeedsDirectorApproval(e.target.checked)
                        }
                        colorScheme="purple"
                      >
                        Minta approval director setelah review finance manager
                      </Checkbox>
                      <Text fontSize="sm" color={mutedTextColor} mt={1}>
                        Jika dicentang, keputusan finance manager menjadi
                        rekomendasi dan PR lanjut ke tahap director.
                      </Text>
                    </Box>
                  )}
                </Box>
              )}
            </ModalBody>
            <ModalFooter
              bg={modalFooterBg}
              borderTopWidth="1px"
              borderColor={borderColor}
              flexShrink={0}
              position="sticky"
              bottom={0}
              zIndex={2}
              px={{ base: 4, md: 6 }}
            >
              {canCurrentUserReviewRequest(selectedRequest) ? (
                <Flex
                  w="full"
                  justify="space-between"
                  align={{ base: "stretch", md: "center" }}
                  direction={{ base: "column-reverse", md: "row" }}
                  gap={2}
                >
                  <Button
                    variant="ghost"
                    onClick={onViewClose}
                    w={{ base: "full", md: "auto" }}
                  >
                    Tutup
                  </Button>
                  <Button
                    colorScheme="blue"
                    onClick={handleReviewSubmit}
                    isLoading={isSubmitting}
                    w={{ base: "full", md: "auto" }}
                  >
                    {getCurrentReviewStage(selectedRequest) === "FINANCE_STAFF"
                      ? "Kirim Review Staff"
                      : getCurrentReviewStage(selectedRequest) === "DIRECTOR"
                        ? "Kirim Review Director"
                        : reviewNeedsDirectorApproval
                          ? "Kirim ke Director"
                          : "Kirim Final Review"}
                  </Button>
                </Flex>
              ) : (
                <Flex
                  w="full"
                  justify="space-between"
                  align={{ base: "stretch", md: "center" }}
                  direction={{ base: "column-reverse", md: "row" }}
                  gap={2}
                >
                  <Button
                    variant="ghost"
                    onClick={onViewClose}
                    w={{ base: "full", md: "auto" }}
                  >
                    Tutup
                  </Button>
                  <Flex
                    gap={2}
                    direction={{ base: "column", md: "row" }}
                    w={{ base: "full", md: "auto" }}
                  >
                    {canOpenRequesterRevision(selectedRequest) && (
                      <Button
                        leftIcon={<FiEdit3 />}
                        colorScheme="purple"
                        variant="outline"
                        onClick={handleOpenRequesterRevisionRequest}
                        w={{ base: "full", md: "auto" }}
                      >
                        {isInitialRequesterRevisionRequest(selectedRequest)
                          ? "Minta Perbaikan PIC"
                          : "Buka Revisi PIC"}
                      </Button>
                    )}
                    {canCancelPurchaseRequest(selectedRequest) && (
                      <Button
                        leftIcon={<FiX />}
                        colorScheme="orange"
                        variant="outline"
                        onClick={handleOpenCancelRequest}
                        w={{ base: "full", md: "auto" }}
                      >
                        Cancel PR
                      </Button>
                    )}
                  </Flex>
                </Flex>
              )}
            </ModalFooter>
          </ModalContent>
        </Modal>
        <AlertDialog
          isOpen={isRequesterRevisionOpen}
          leastDestructiveRef={requesterRevisionLeastDestructiveRef}
          onClose={onRequesterRevisionClose}
          isCentered
        >
          <AlertDialogOverlay>
            <AlertDialogContent bg={modalBg} color={textColor}>
              <AlertDialogHeader fontSize="lg" fontWeight="bold">
                {isInitialRequesterRevisionRequest(selectedRequest)
                  ? "Minta Perbaikan PR dari PIC"
                  : "Buka Revisi PR untuk PIC"}
              </AlertDialogHeader>
              <AlertDialogBody>
                <Text mb={3}>
                  {isInitialRequesterRevisionRequest(selectedRequest)
                    ? `PR ${selectedRequest?.pr_number} akan dikembalikan ke PIC untuk diperbaiki sebelum review Finance Staff dikirim. Tidak ada approval yang dicatat. Setelah PIC submit, PR masuk kembali ke Finance Staff lalu Finance Manager.`
                    : `PR ${selectedRequest?.pr_number} akan dibuka agar PIC dapat memperbarui nominal dan menambahkan dokumen pendukung. Setelah PIC submit, PR akan masuk approval ulang Finance Staff dan Finance Manager.`}
                </Text>
                <FormControl isRequired>
                  <FormLabel>
                    {isInitialRequesterRevisionRequest(selectedRequest)
                      ? "Alasan perbaikan"
                      : "Alasan buka revisi"}
                  </FormLabel>
                  <Textarea
                    value={requesterRevisionOpenReason}
                    onChange={(e) =>
                      setRequesterRevisionOpenReason(e.target.value)
                    }
                    placeholder="Contoh: PIC perlu menyesuaikan nominal berdasarkan dokumen vendor terbaru."
                    bg={inputBg}
                    borderColor={inputBorderColor}
                    color={textColor}
                    _hover={{ borderColor: inputBorderColor }}
                    _focusVisible={fieldFocusStyles}
                    minH="110px"
                  />
                </FormControl>
              </AlertDialogBody>
              <AlertDialogFooter>
                <Button
                  ref={requesterRevisionLeastDestructiveRef}
                  onClick={onRequesterRevisionClose}
                  isDisabled={isOpeningRequesterRevision}
                >
                  Batal
                </Button>
                <Button
                  colorScheme="purple"
                  onClick={handleOpenRequesterRevision}
                  isLoading={isOpeningRequesterRevision}
                  isDisabled={!requesterRevisionOpenReason.trim()}
                  ml={3}
                >
                  {isInitialRequesterRevisionRequest(selectedRequest)
                    ? "Kirim Permintaan"
                    : "Buka Revisi"}
                </Button>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialogOverlay>
        </AlertDialog>
        <AlertDialog
          isOpen={isCancelOpen}
          leastDestructiveRef={cancelLeastDestructiveRef}
          onClose={onCancelClose}
          isCentered
        >
          <AlertDialogOverlay>
            <AlertDialogContent bg={modalBg} color={textColor}>
              <AlertDialogHeader fontSize="lg" fontWeight="bold">
                Cancel Purchase Request
              </AlertDialogHeader>
              <AlertDialogBody>
                <Text mb={3}>
                  PR {selectedRequest?.pr_number} akan dibatalkan tanpa
                  menghapus histori approval. Tindakan ini hanya bisa dilakukan
                  jika PR belum dibuat pembelian.
                </Text>
                <FormControl isRequired>
                  <FormLabel>Alasan pembatalan</FormLabel>
                  <Textarea
                    value={cancelReason}
                    onChange={(e) => setCancelReason(e.target.value)}
                    placeholder="Contoh: PR sudah tidak dilanjutkan ke pembelian karena kebutuhan dibatalkan."
                    bg={inputBg}
                    borderColor={inputBorderColor}
                    color={textColor}
                    _hover={{ borderColor: inputBorderColor }}
                    _focusVisible={fieldFocusStyles}
                    minH="110px"
                  />
                </FormControl>
              </AlertDialogBody>
              <AlertDialogFooter>
                <Button
                  ref={cancelLeastDestructiveRef}
                  onClick={onCancelClose}
                  isDisabled={isCancellingRequest}
                >
                  Batal
                </Button>
                <Button
                  colorScheme="orange"
                  onClick={handleCancelPurchaseRequest}
                  isLoading={isCancellingRequest}
                  isDisabled={!cancelReason.trim()}
                  ml={3}
                >
                  Batalkan PR
                </Button>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialogOverlay>
        </AlertDialog>
      </Layout>
    </ProtectedModule>
  );
}
