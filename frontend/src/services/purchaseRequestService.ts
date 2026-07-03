import { API_ENDPOINTS } from "@/config/api";
import api from "./api";

const PURCHASE_REQUEST_UPLOAD_TIMEOUT_MS = 120000;

const downloadBlobResponse = (blob: Blob, disposition: string, fallbackName: string) => {
  const filenameMatch =
    disposition.match(/filename\*=UTF-8''([^;]+)/i) ||
    disposition.match(/filename="?([^"]+)"?/i);
  const filename = filenameMatch?.[1]
    ? decodeURIComponent(filenameMatch[1])
    : fallbackName;

  const url = window.URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  window.URL.revokeObjectURL(url);
};

export interface User {
  id: number;
  username: string;
  first_name: string;
  last_name: string;
}

export interface Product {
  id: number;
  code: string;
  name: string;
  unit: string;
  purchase_price: number;
}

export interface Contact {
  id: number;
  code?: string;
  name: string;
  type: "CUSTOMER" | "VENDOR" | "EMPLOYEE";
  payment_terms?: number;
  is_active?: boolean;
}

export interface PurchaseRequestPaymentSummary {
  id: number;
  purchase_id: number;
  payment_number?: string;
  date?: string;
  amount: number;
  method?: string;
  reference?: string;
  payment_id?: number | null;
  payment_proof_file_name?: string;
  payment_proof_file_path?: string;
  payment_proof_file_size?: number;
  payment_proof_mime_type?: string;
  payment_proof_uploaded_by?: number | null;
  payment_proof_uploaded_at?: string | null;
  payment_proof_review_status?: string;
  payment_proof_reviewed_by?: number | null;
  payment_proof_reviewed_at?: string | null;
  payment_proof_review_notes?: string;
  payment?: {
    id: number;
    code: string;
    status: string;
    amount: number;
    date: string;
    method: string;
    reference?: string;
    finance_manager_approved_at?: string | null;
    approved_at?: string | null;
  } | null;
}

export interface PurchaseRequestItem {
  id: number;
  purchase_request_id: number;
  product_id?: number | null;
  item_name: string;
  quantity: number;
  estimated_price: number;
  total_estimated_price: number;
  approval_status: "PENDING" | "APPROVED" | "REJECTED";
  finance_staff_decision: "PENDING" | "APPROVED" | "REJECTED";
  finance_manager_decision: "PENDING" | "APPROVED" | "REJECTED";
  director_decision: "PENDING" | "APPROVED" | "REJECTED";
  purchase_id?: number | null;
  purchased_at?: string | null;
  notes: string;
  product?: Product;
  purchase?: {
    id: number;
    code: string;
    status: string;
    payment_method?: string;
    due_date?: string;
    paid_amount: number;
    outstanding_amount: number;
    total_amount: number;
    project_id?: number | null;
    purchase_payments?: PurchaseRequestPaymentSummary[];
  };
}

export interface PurchaseRequestDocument {
  id: number;
  purchase_request_id: number;
  document_type: string;
  file_name: string;
  file_path: string;
  file_size: number;
  mime_type: string;
  uploaded_by: number;
  approval_revision: number;
  created_at: string;
  updated_at: string;
  uploader?: User;
}

export interface PurchaseRequestRevisionSnapshot {
  id: number;
  purchase_request_id: number;
  revision_number: number;
  reapproval_reason: string;
  previous_status: string;
  previous_approval_stage: string;
  previous_total_estimated_amount: number;
  new_total_estimated_amount?: number | null;
  item_snapshot?: string;
  opened_by: number;
  opened_at: string;
  open_reason?: string;
  submitted_by?: number | null;
  submitted_at?: string | null;
  submitted_notes?: string;
  approved_by?: number | null;
  approved_at?: string | null;
  finance_staff_approved_by?: number | null;
  finance_staff_approved_at?: string | null;
  finance_manager_approved_by?: number | null;
  finance_manager_approved_at?: string | null;
  director_approved_by?: number | null;
  director_approved_at?: string | null;
  opener?: User;
  submitter?: User;
}

export interface PurchaseRequest {
  id: number;
  pr_number: string;
  requester_id: number;
  department: string;
  title?: string;
  project_id: number | null;
  vendor_id: number | null;
  vendor_invoice_received_date?: string | null;
  payment_terms_days?: number | null;
  due_date?: string | null;
  requires_vendor_invoice?: boolean;
  vendor_invoice_status?:
    | "NOT_REQUIRED"
    | "AWAITING_INVOICE"
    | "PENDING_APPROVAL"
    | "APPROVED"
    | string;
  vendor_invoice_requested_by?: number | null;
  vendor_invoice_requested_at?: string | null;
  vendor_invoice_submitted_by?: number | null;
  vendor_invoice_submitted_at?: string | null;
  vendor_invoice_revision_notes?: string;
  revision_opened_by?: number | null;
  revision_opened_at?: string | null;
  revision_reason?: string;
  revision_submitted_by?: number | null;
  revision_submitted_at?: string | null;
  total_estimated_amount: number;
  status:
    | "DRAFT"
    | "PENDING_APPROVAL"
    | "APPROVED"
    | "REJECTED"
    | "COMPLETED"
    | "CANCELLED"
    | "AWAITING_REVISION";
  current_approval_stage?:
    | "FINANCE_STAFF"
    | "FINANCE_MANAGER"
    | "DIRECTOR"
    | "FINALIZED";
  approval_revision?: number;
  reapproval_reason?:
    | "ADDITIONAL_DOCUMENTS"
    | "INVOICE_REVISION"
    | "REQUESTER_REVISION"
    | "";
  notes: string;
  approved_by: number | null;
  approved_at: string | null;
  finance_staff_approved_by?: number | null;
  finance_staff_approved_at?: string | null;
  finance_staff_review_notes?: string;
  finance_manager_approved_by?: number | null;
  finance_manager_approved_at?: string | null;
  finance_manager_review_notes?: string;
  director_approved_by?: number | null;
  director_approved_at?: string | null;
  rejection_reason: string;
  cancelled_by?: number | null;
  cancelled_at?: string | null;
  cancelled_reason?: string;
  created_at: string;
  updated_at: string;
  requester?: User;
  approver?: User;
  finance_staff_approver?: User;
  finance_manager_approver?: User;
  director_approver?: User;
  canceller?: User;
  revision_opener?: User;
  revision_submitter?: User;
  vendor_invoice_requester?: User;
  vendor_invoice_submitter?: User;
  project?: {
    id: number;
    code: string;
    name: string;
  };
  vendor?: Contact;
  items?: PurchaseRequestItem[];
  documents?: PurchaseRequestDocument[];
  revision_snapshots?: PurchaseRequestRevisionSnapshot[];
}

export interface PurchaseRequestItemPayload {
  product_id?: number;
  item_name?: string;
  quantity: number;
  estimated_price: number;
  notes?: string;
}

export interface PurchaseRequestCreatePayload {
  department?: string;
  title?: string;
  project_id?: number | null;
  vendor_id?: number | null;
  vendor_invoice_received_date?: string | null;
  payment_terms_days?: number | null;
  due_date?: string | null;
  notes?: string;
  items: PurchaseRequestItemPayload[];
}

export interface RejectPayload {
  reason: string;
}

export interface PurchaseRequestReviewItemPayload {
  item_id: number;
  decision: "APPROVED" | "REJECTED";
}

export interface PurchaseRequestReviewPayload {
  items: PurchaseRequestReviewItemPayload[];
  reason?: string;
  escalate_to_director?: boolean;
  require_vendor_invoice?: boolean;
}

export interface PurchaseRequestProjectUpdatePayload {
  project_id?: number | null;
}

export interface PurchaseRequestInvoiceTermsUpdatePayload {
  vendor_invoice_received_date?: string | null;
  payment_terms_days?: number | null;
  due_date?: string | null;
}

export interface PurchaseRequestInvoiceRevisionItemPayload {
  item_id: number;
  quantity: number;
  estimated_price: number;
  notes?: string;
}

export interface PurchaseRequestInvoiceRevisionPayload {
  notes?: string;
  items: PurchaseRequestInvoiceRevisionItemPayload[];
}

export interface PurchaseRequestOpenRevisionPayload {
  reason: string;
}

export interface PurchaseRequestRequesterRevisionItemPayload {
  item_id: number;
  quantity: number;
  estimated_price: number;
  notes?: string;
}

export interface PurchaseRequestRequesterRevisionPayload {
  notes?: string;
  items: PurchaseRequestRequesterRevisionItemPayload[];
}

const BASE_URL = "/api/v1/purchase-requests";

export const purchaseRequestService = {
  getAll: async (
    department?: string,
    status?: string,
    availableForPurchase?: boolean,
    currentPurchaseId?: number,
  ): Promise<PurchaseRequest[]> => {
    let query = "";
    const params = new URLSearchParams();
    if (department) params.append("department", department);
    if (status) params.append("status", status);
    if (availableForPurchase) params.append("available_for_purchase", "true");
    if (currentPurchaseId) {
      params.append("current_purchase_id", currentPurchaseId.toString());
    }

    if (params.toString()) query = `?${params.toString()}`;

    const response = await api.get(`${BASE_URL}${query}`);
    return response.data;
  },

  getById: async (id: number): Promise<PurchaseRequest> => {
    const response = await api.get(`${BASE_URL}/${id}`);
    return response.data;
  },

  create: async (
    data: PurchaseRequestCreatePayload,
    documents: File[] = [],
  ): Promise<PurchaseRequest> => {
    if (documents.length > 0) {
      const formData = new FormData();
      formData.append("payload", JSON.stringify(data));
      documents.forEach((file) => {
        formData.append("documents", file);
      });

      const response = await api.post(BASE_URL, formData, {
        headers: { "Content-Type": "multipart/form-data" },
        timeout: PURCHASE_REQUEST_UPLOAD_TIMEOUT_MS,
      });
      return response.data;
    }

    const response = await api.post(BASE_URL, data);
    return response.data;
  },

  uploadDocuments: async (
    id: number,
    documents: File[],
  ): Promise<{
    message: string;
    documents: PurchaseRequestDocument[];
    reapproval_started: boolean;
  }> => {
    const formData = new FormData();
    documents.forEach((file) => {
      formData.append("documents", file);
    });

    const response = await api.post(`${BASE_URL}/${id}/documents`, formData, {
      headers: { "Content-Type": "multipart/form-data" },
      timeout: PURCHASE_REQUEST_UPLOAD_TIMEOUT_MS,
    });
    return response.data;
  },

  uploadRealizationDocuments: async (
    id: number,
    documents: File[],
  ): Promise<{
    message: string;
    documents: PurchaseRequestDocument[];
  }> => {
    const formData = new FormData();
    documents.forEach((file) => {
      formData.append("realization_documents", file);
    });

    const response = await api.post(
      `${BASE_URL}/${id}/realization-documents`,
      formData,
      {
        headers: { "Content-Type": "multipart/form-data" },
        timeout: PURCHASE_REQUEST_UPLOAD_TIMEOUT_MS,
      },
    );
    return response.data;
  },

  getDocuments: async (id: number): Promise<PurchaseRequestDocument[]> => {
    const response = await api.get(`${BASE_URL}/${id}/documents`);
    return response.data;
  },

  deleteDocument: async (documentId: number): Promise<{ message: string }> => {
    const response = await api.delete(`${BASE_URL}/documents/${documentId}`);
    return response.data;
  },

  downloadDocument: async (
    documentId: number,
    fallbackName = "purchase-request-document",
  ): Promise<void> => {
    const response = await api.get(
      API_ENDPOINTS.PURCHASE_REQUEST_DOCUMENT_DOWNLOAD(documentId),
      { responseType: "blob" },
    );
    downloadBlobResponse(
      response.data,
      response.headers?.["content-disposition"] || "",
      fallbackName,
    );
  },

  review: async (
    id: number,
    payload: PurchaseRequestReviewPayload,
  ): Promise<{ message: string }> => {
    const response = await api.put(`${BASE_URL}/${id}/review`, payload);
    return response.data;
  },

  updateProject: async (
    id: number,
    payload: PurchaseRequestProjectUpdatePayload,
  ): Promise<PurchaseRequest> => {
    const response = await api.put(`${BASE_URL}/${id}/project`, payload);
    return response.data;
  },

  updateInvoiceTerms: async (
    id: number,
    payload: PurchaseRequestInvoiceTermsUpdatePayload,
  ): Promise<PurchaseRequest> => {
    const response = await api.put(`${BASE_URL}/${id}/invoice-terms`, payload);
    return response.data;
  },

  openRequesterRevision: async (
    id: number,
    payload: PurchaseRequestOpenRevisionPayload,
  ): Promise<PurchaseRequest> => {
    const response = await api.put(
      `${BASE_URL}/${id}/open-requester-revision`,
      payload,
    );
    return response.data.data;
  },

  submitRequesterRevision: async (
    id: number,
    payload: PurchaseRequestRequesterRevisionPayload,
    documents: File[],
  ): Promise<PurchaseRequest> => {
    const formData = new FormData();
    formData.append("payload", JSON.stringify(payload));
    documents.forEach((file) => {
      formData.append("documents", file);
    });

    const response = await api.post(`${BASE_URL}/${id}/requester-revision`, formData, {
      headers: { "Content-Type": "multipart/form-data" },
      timeout: PURCHASE_REQUEST_UPLOAD_TIMEOUT_MS,
    });
    return response.data.data;
  },

  submitInvoiceRevision: async (
    id: number,
    payload: PurchaseRequestInvoiceRevisionPayload,
    documents: File[],
  ): Promise<PurchaseRequest> => {
    const formData = new FormData();
    formData.append("payload", JSON.stringify(payload));
    documents.forEach((file) => {
      formData.append("documents", file);
    });

    const response = await api.post(`${BASE_URL}/${id}/invoice-revision`, formData, {
      headers: { "Content-Type": "multipart/form-data" },
      timeout: PURCHASE_REQUEST_UPLOAD_TIMEOUT_MS,
    });
    return response.data.data;
  },

  approve: async (id: number): Promise<{ message: string }> => {
    const response = await api.put(`${BASE_URL}/${id}/approve`);
    return response.data;
  },

  reject: async (id: number, reason: string): Promise<{ message: string }> => {
    const response = await api.put(`${BASE_URL}/${id}/reject`, { reason });
    return response.data;
  },

  cancel: async (id: number, reason: string): Promise<PurchaseRequest> => {
    const response = await api.put(`${BASE_URL}/${id}/cancel`, { reason });
    return response.data.data;
  },
};
