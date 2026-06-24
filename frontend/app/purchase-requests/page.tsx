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
  FormControl,
  FormLabel,
  useToast,
  IconButton,
  Tooltip,
  Badge,
  Textarea,
  Select,
  Grid,
  Link,
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
import Layout from "@/components/layout/Layout";
import ProtectedModule from "@/components/common/ProtectedModule";
import { usePermissions } from "@/hooks/usePermissions";
import CurrencyInput from "@/components/common/CurrencyInput";
import {
  purchaseRequestService,
  PurchaseRequest,
  PurchaseRequestCreatePayload,
  PurchaseRequestItem,
} from "@/services/purchaseRequestService";
import { projectService, Project } from "@/services/projectService";
import searchableSelectService, {
  Contact as SelectContact,
} from "@/services/searchableSelectService";
import { useAuth } from "@/contexts/AuthContext";
import api from "@/services/api";
import {
  isAccountingRole,
  isFinanceManagerRole,
  isFinanceStaffRole,
  isFinanceTeamRole,
  normalizeRole,
} from "@/utils/roles";
import { getImageUrl } from "@/utils/imageUrl";

const ACCEPTED_DOCUMENT_TYPES =
  ".pdf,.doc,.docx,.xls,.xlsx,.csv,.txt,.jpg,.jpeg,.png,.webp";
const MAX_DOCUMENT_SIZE = 20 * 1024 * 1024;
const PURCHASE_REQUEST_PAGE_SIZE_OPTIONS = [10, 25, 50];
const PURCHASE_REQUEST_PROGRESS_FILTERS = [
  { value: "", label: "Semua Progress" },
  { value: "WAITING_FINANCE_STAFF", label: "Belum Review Finance Staff" },
  { value: "FINANCE_STAFF_APPROVED", label: "Sudah Review Finance Staff" },
  { value: "WAITING_FINANCE_MANAGER", label: "Belum Review Finance Manager" },
  { value: "FINANCE_MANAGER_APPROVED", label: "Sudah Review Finance Manager" },
  { value: "WAITING_DIRECTOR", label: "Menunggu Director" },
  { value: "APPROVED", label: "PR Approved" },
  { value: "REJECTED", label: "PR Rejected" },
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

const createEmptyPurchaseRequestItem = (): PurchaseRequestFormItem => ({
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
  const [progressFilter, setProgressFilter] =
    useState<PurchaseRequestProgressFilter>("");
  const [startDateFilter, setStartDateFilter] = useState("");
  const [endDateFilter, setEndDateFilter] = useState("");
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isUploadingDocuments, setIsUploadingDocuments] = useState(false);
  const [isUpdatingProject, setIsUpdatingProject] = useState(false);
  const [pendingAdditionalDocuments, setPendingAdditionalDocuments] = useState<
    File[]
  >([]);
  const { canCreate, canEdit } = usePermissions();
  const formatCurrency = (value: number) => {
    return new Intl.NumberFormat("id-ID", {
      style: "currency",
      currency: "IDR",
    }).format(value);
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

  const [selectedRequest, setSelectedRequest] =
    useState<PurchaseRequest | null>(null);
  const [reviewReason, setReviewReason] = useState("");
  const [projectUpdateId, setProjectUpdateId] = useState("");
  const [reviewNeedsDirectorApproval, setReviewNeedsDirectorApproval] =
    useState(false);
  const [reviewDecisions, setReviewDecisions] = useState<
    Record<number, "APPROVED" | "REJECTED" | "">
  >({});
  const handledNotificationRequestId = useRef<string | null>(null);

  // Form state
  const [title, setTitle] = useState("");
  const [department, setDepartment] = useState("");
  const [projectId, setProjectId] = useState("");
  const [vendorId, setVendorId] = useState("");
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

  const handleOpenCreate = () => {
    setTitle("");
    setDepartment(userDepartment);
    setProjectId("");
    setVendorId("");
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
      return "Ukuran dokumen maksimal 20MB per file.";
    }

    return null;
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
        description: err.response?.data?.error || err.message,
        status: "error",
        duration: 4000,
      });
    } finally {
      setIsUploadingDocuments(false);
    }
  };

  const formatFileSize = (size?: number) => {
    const bytes = Number(size || 0);
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  const getDocumentUrl = (path?: string) => getImageUrl(path || "") || "#";

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

    try {
      setIsSubmitting(true);
      const payload: PurchaseRequestCreatePayload = {
        title: title.trim() || undefined,
        department,
        project_id: projectId ? parseInt(projectId) : undefined,
        vendor_id: vendorId ? parseInt(vendorId) : undefined,
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
        description: err.response?.data?.error || err.message,
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

    return {
      hasPurchase: purchaseIDs.size > 0,
      hasPayment:
        totalPaid > 0 ||
        purchases.some((purchase) => ["PAID", "COMPLETED"].includes(purchase.status)),
      purchaseCount: purchaseIDs.size,
      totalPaid,
      totalOutstanding,
      totalAmount,
      purchaseCodes: purchases.map((purchase) => purchase.code).filter(Boolean),
    };
  };

  const getRealizationBadge = (pr?: PurchaseRequest | null) => {
    const summary = getPurchaseRealizationSummary(pr);
    if (!summary.hasPurchase) {
      return <Badge colorScheme="gray">Belum Pembelian</Badge>;
    }

    if (summary.hasPayment) {
      return <Badge colorScheme="green">Sudah Ada Pembayaran</Badge>;
    }

    return <Badge colorScheme="blue">Sudah Dibuat Pembelian</Badge>;
  };

  const matchesProgressFilter = (pr: PurchaseRequest) => {
    if (!progressFilter) return true;

    const stage = getCurrentReviewStage(pr);
    const realization = getPurchaseRealizationSummary(pr);
    const hasStaffReview =
      Boolean(pr.finance_staff_approved_by || pr.finance_staff_approved_at) ||
      (pr.items || []).some(
        (item) => normalizeDecision(item.finance_staff_decision) !== "PENDING",
      );
    const hasManagerReview =
      Boolean(pr.finance_manager_approved_by || pr.finance_manager_approved_at) ||
      (pr.items || []).some(
        (item) => normalizeDecision(item.finance_manager_decision) !== "PENDING",
      );

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
      case "WAITING_DIRECTOR":
        return pr.status === "PENDING_APPROVAL" && stage === "DIRECTOR";
      case "APPROVED":
        return pr.status === "APPROVED";
      case "REJECTED":
        return pr.status === "REJECTED";
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

  const handleView = async (pr: PurchaseRequest) => {
    try {
      setLoading(true);
      const detail = await purchaseRequestService.getById(pr.id);
      setSelectedRequest(detail);
      setProjectUpdateId(detail.project_id?.toString() || "");
      setPendingAdditionalDocuments([]);
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
    searchQuery || progressFilter || startDateFilter || endDateFilter,
  );

  const handleClearFilters = () => {
    setSearchQuery("");
    setProgressFilter("");
    setStartDateFilter("");
    setEndDateFilter("");
    setCurrentPage(1);
  };

  const filteredRequests = requests.filter((r) => {
    const normalizedSearch = searchQuery.trim().toLowerCase();
	    const matchesSearch =
	      !normalizedSearch ||
	      r.pr_number.toLowerCase().includes(normalizedSearch) ||
	      getPurchaseRequestTitle(r).toLowerCase().includes(normalizedSearch) ||
	      (r.vendor?.name || "").toLowerCase().includes(normalizedSearch) ||
	      (r.project?.name || "").toLowerCase().includes(normalizedSearch);

    const requestDate = getDateKey(r.created_at);
    const matchesStartDate =
      !startDateFilter || Boolean(requestDate && requestDate >= startDateFilter);
    const matchesEndDate =
      !endDateFilter || Boolean(requestDate && requestDate <= endDateFilter);

    return (
	      matchesSearch &&
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
  }, [searchQuery, startDateFilter, endDateFilter, pageSize]);

  useEffect(() => {
    setCurrentPage((page) => Math.min(page, totalPages));
  }, [totalPages]);

  const getStatusBadge = (request: PurchaseRequest) => {
    switch (request.status) {
      case "DRAFT":
        return <Badge colorScheme="gray">Draft</Badge>;
      case "PENDING_APPROVAL":
        return (
          <Badge colorScheme="orange">
            {getStageLabel(request) || "Pending Review"}
          </Badge>
        );
      case "APPROVED":
        return <Badge colorScheme="green">Approved</Badge>;
      case "REJECTED":
        return <Badge colorScheme="red">Rejected</Badge>;
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
                    placeholder="Cari by Nomor PR, Vendor, atau Proyek..."
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
	                    <Th>Status</Th>
	                    <Th textAlign="center">Aksi</Th>
	                  </Tr>
                </Thead>
                <Tbody>
	                  {loading ? (
	                    <Tr>
	                      <Td colSpan={11} textAlign="center" py={4}>
	                        Memuat...
	                      </Td>
	                    </Tr>
	                  ) : filteredRequests.length === 0 ? (
	                    <Tr>
	                      <Td colSpan={11} textAlign="center" py={4}>
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
	                        </Td>
	                        <Td minW="170px">
	                          {getRealizationBadge(pr)}
	                          {getPurchaseRealizationSummary(pr).hasPayment && (
	                            <Text fontSize="xs" color={mutedTextColor} mt={1}>
	                              Terbayar{" "}
	                              {formatCurrency(
	                                getPurchaseRealizationSummary(pr).totalPaid,
	                              )}
	                            </Text>
	                          )}
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
                  <Select
                    placeholder="-- Pilih Vendor (Opsional) --"
                    value={vendorId}
                    onChange={(e) => setVendorId(e.target.value)}
                    bg={inputBg}
                    borderColor={inputBorderColor}
                    color={textColor}
                    _hover={{ borderColor: inputBorderColor }}
                    _focusVisible={fieldFocusStyles}
                  >
                    {vendors.map((vendor) => (
                      <option key={vendor.id} value={vendor.id}>
                        {vendor.name}
                        {vendor.code ? ` - ${vendor.code}` : ""}
                      </option>
                    ))}
                  </Select>
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
                      CSV, TXT, JPG, PNG, atau WebP. Maksimal 20MB per file.
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
	                    <Flex mt={1} gap={2} align="center" wrap="wrap">
	                      {getRealizationBadge(selectedRequest)}
	                      {getPurchaseRealizationSummary(selectedRequest)
	                        .purchaseCount > 0 && (
	                        <Text fontSize="xs" color={mutedTextColor}>
	                          {
	                            getPurchaseRealizationSummary(selectedRequest)
	                              .purchaseCount
	                          }{" "}
	                          pembelian
	                        </Text>
	                      )}
	                    </Flex>
	                    {getPurchaseRealizationSummary(selectedRequest).hasPayment && (
	                      <Text fontSize="xs" color={mutedTextColor} mt={1}>
	                        Terbayar{" "}
	                        {formatCurrency(
	                          getPurchaseRealizationSummary(selectedRequest)
	                            .totalPaid,
	                        )}
	                        {getPurchaseRealizationSummary(selectedRequest)
	                          .totalOutstanding > 0 &&
	                          `, sisa ${formatCurrency(
	                            getPurchaseRealizationSummary(selectedRequest)
	                              .totalOutstanding,
	                          )}`}
	                      </Text>
	                    )}
	                  </Box>
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
	                  {(selectedRequest.project || canUpdatePurchaseRequestProject) && (
	                    <Box>
	                      <Text fontSize="sm" color={mutedTextColor}>
	                        Dialokasikan ke Proyek
	                      </Text>
	                      {canUpdatePurchaseRequestProject ? (
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
                              <Badge colorScheme="blue">Sudah Dipilih</Badge>
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
                      {selectedRequest?.documents?.length || 0} file
                    </Badge>
                  </Flex>
                  {selectedRequest &&
                    canEdit("purchase_requests") &&
                    selectedRequest.status !== "COMPLETED" &&
                    selectedRequest.status !== "REJECTED" &&
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

                {selectedRequest?.documents?.length ? (
                  <Box>
                    {selectedRequest.documents.map((document, index) => (
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
                              {formatFileSize(document.file_size)}
                              {document.uploader &&
                                ` - diunggah oleh ${document.uploader.first_name} ${document.uploader.last_name}`}
                            </Text>
                            {document.approval_revision > 1 && (
                              <Badge mt={1} colorScheme="orange">
                                Dokumen tambahan - Approval ulang
                              </Badge>
                            )}
                          </Box>
                        </Flex>
                        <Button
                          as={Link}
                          href={getDocumentUrl(document.file_path)}
                          target="_blank"
                          rel="noopener noreferrer"
                          leftIcon={<FiDownload />}
                          size="sm"
                          variant="outline"
                          colorScheme="blue"
                          textDecoration="none"
                          _hover={{ textDecoration: "none" }}
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
                <Button variant="ghost" onClick={onViewClose}>
                  Tutup
                </Button>
              )}
            </ModalFooter>
          </ModalContent>
        </Modal>
      </Layout>
    </ProtectedModule>
  );
}
