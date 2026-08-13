import api from "./api";
import { API_ENDPOINTS } from "@/config/api";
import { getDownloadFilename } from "@/utils/downloadFilename";

export interface PurchaseItemRequest {
  product_id?: number;
  purchase_request_item_id?: number;
  quantity: number;
  unit_price: number;
  discount?: number;
  tax?: number;
  expense_account_id?: number;
  description?: string;
  item_type?: string; // "INVENTORY" | "ASSET" | "PREPAID" | "EXPENSE"
  is_fixed_asset?: boolean;
  asset_category_id?: number;
  amortization_month?: number;
}

export interface PurchaseCreateRequest {
  vendor_id: number;
  purchase_request_id?: number;
  project_id?: number;
  date: string; // ISO string
  due_date?: string; // ISO string
  discount?: number;
  tax?: number;
  freight_cost?: number;
  other_cost_amount?: number;
  other_cost_description?: string;
  other_cost_account_id?: number;
  notes?: string;
  items: PurchaseItemRequest[]; // backend expects `items`
  payment_terms?: PurchasePaymentTermRequest[];
  request_priority?: "LOW" | "NORMAL" | "HIGH" | "URGENT";
}

export interface PurchaseAccountingCOARequest {
  credit_account_id?: number;
  items: Array<{
    purchase_item_id: number;
    expense_account_id: number;
  }>;
}

export interface PurchaseFinalReviewConfigRequest {
  ppn_rate?: number;
  other_tax_additions?: number;
  freight_cost?: number;
  other_cost_amount?: number;
  other_cost_description?: string;
  other_cost_account_id?: number;
  pph21_rate?: number;
  pph23_rate?: number;
  other_tax_deductions?: number;
  payment_method?: string;
  bank_account_id?: number;
  credit_account_id?: number;
  payment_reference?: string;
  notes?: string;
}

export type PurchaseCOARevisionStatus =
  | "PENDING_FINANCE_MANAGER"
  | "APPROVED"
  | "REJECTED";

export interface PurchaseCOARevisionActor {
  id: number;
  username?: string;
  first_name?: string;
  last_name?: string;
  email?: string;
}

export interface PurchaseCOARevision {
  id: number;
  purchase_id: number;
  status: PurchaseCOARevisionStatus;
  reason: string;
  coa_snapshot: string;
  requested_by: number;
  requested_at: string;
  finance_manager_decision_notes?: string;
  finance_manager_decision_by?: number | null;
  finance_manager_decision_at?: string | null;
  created_at: string;
  updated_at: string;
  requester?: PurchaseCOARevisionActor;
  finance_manager?: PurchaseCOARevisionActor;
}

export type PurchasePrePostingRevisionStatus =
  | "PENDING_FINANCE_MANAGER"
  | "OPEN_FOR_FINANCE"
  | "REJECTED"
  | "RESUBMITTED";

export type PurchasePrePostingRevisionType =
  | "TRANSACTION_DATE"
  | "VENDOR_OR_PROJECT"
  | "ITEMS_AND_AMOUNT"
  | "TAX_AND_COST"
  | "PAYMENT_CONFIGURATION"
  | "OTHER";

export interface PurchasePrePostingRevision {
  id: number;
  purchase_id: number;
  status: PurchasePrePostingRevisionStatus;
  revision_types: string;
  reason: string;
  original_approval_request_id?: number | null;
  resubmitted_approval_request_id?: number | null;
  requested_by: number;
  requested_at: string;
  finance_manager_decision_notes?: string;
  finance_manager_decision_by?: number | null;
  finance_manager_decision_at?: string | null;
  opened_for_revision_by?: number | null;
  opened_for_revision_at?: string | null;
  resubmitted_by?: number | null;
  resubmitted_at?: string | null;
  created_at: string;
  updated_at: string;
  requester?: PurchaseCOARevisionActor;
  finance_manager?: PurchaseCOARevisionActor;
  opened_by?: PurchaseCOARevisionActor;
  resubmitted_by_user?: PurchaseCOARevisionActor;
}

export interface PurchaseFilterParams {
  status?: string;
  vendor_id?: string;
  project_id?: string;
  start_date?: string;
  end_date?: string;
  search?: string;
  approval_status?: string;
  accounting_status?: string;
  requires_approval?: boolean;
  outstanding?: boolean;
  page?: number;
  limit?: number;
}

export interface PurchaseItem {
  id: number;
  product_id?: number | null;
  purchase_request_item_id?: number | null;
  quantity: number;
  unit_price: number;
  discount: number;
  tax: number;
  total_price: number;
  expense_account_id?: number;
  item_type?: string;
  is_fixed_asset?: boolean;
  description?: string;
  asset_category_id?: number | null;
  amortization_month?: number;
  product?: {
    id: number;
    name: string;
    code: string;
  };
  purchase_request_item?: {
    id: number;
    purchase_request_id: number;
    approval_status: string;
    item_name?: string;
    notes?: string;
  };
}

export interface Vendor {
  id: number;
  name: string;
  code: string;
  email?: string;
  phone?: string;
}

export interface PurchaseRequestDocumentSummary {
  id: number;
  purchase_request_id: number;
  document_type: string;
  file_name: string;
  file_path: string;
  file_size: number;
  mime_type?: string;
  uploaded_by?: number;
  approval_revision?: number;
  created_at?: string;
  updated_at?: string;
  uploader?: {
    id: number;
    username?: string;
    first_name?: string;
    last_name?: string;
  };
}

export interface PurchaseRequestSummary {
  id: number;
  pr_number?: string;
  request_number?: string;
  title?: string;
  department?: string;
  vendor_invoice_received_date?: string | null;
  payment_terms_days?: number | null;
  due_date?: string | null;
  project?: {
    id: number;
    code: string;
    name: string;
  };
  documents?: PurchaseRequestDocumentSummary[];
}

export interface ProjectSummary {
  id: number;
  code: string;
  name: string;
}

export interface Purchase {
  id: number;
  code: string;
  vendor_id: number;
  user_id: number;
  date: string;
  due_date?: string;
  subtotal_before_discount: number;
  item_discount_amount: number;
  discount: number; // order-level percent
  order_discount_amount: number;
  net_before_tax: number;
  ppn_rate?: number;
  ppn_amount?: number;
  other_tax_additions?: number;
  total_tax_additions?: number;
  pph21_rate?: number;
  pph21_amount?: number;
  pph23_rate?: number;
  pph23_amount?: number;
  other_tax_deductions?: number;
  total_tax_deductions?: number;
  pph_payment_status?: "NONE" | "PAID" | "UNPAID" | "PARTIAL_PAID";
  pph_paid_amount?: number;
  pph_payment_date?: string | null;
  pph_payment_reference?: string;
  pph_payment_notes?: string;
  pph_payment_journal_entry_id?: number | null;
  tax?: number;
  freight_cost?: number;
  other_cost_amount?: number;
  other_cost_description?: string;
  other_cost_account_id?: number | null;
  other_cost_account?: {
    id: number;
    code: string;
    name: string;
    type?: string;
  } | null;
  tax_amount: number;
  total_amount: number; // grand total
  // Payment tracking fields
  paid_amount: number;
  outstanding_amount: number;
  payment_method: string; // CREDIT, CASH, TRANSFER
  bank_account_id?: number;
  credit_account_id?: number;
  payment_reference?: string;
  matching_status: string;
  status: string;
  notes?: string;
  approval_status: string;
  approval_amount_basis?:
    | "SUBTOTAL_BEFORE_DISCOUNT"
    | "NET_AFTER_DISCOUNT_BEFORE_TAX"
    | "GRAND_TOTAL_AFTER_TAX";
  approval_base_amount?: number;
  requires_approval: boolean;
  approval_request_id?: number;
  approved_at?: string;
  approved_by?: number;
  purchase_request_id?: number;
  project_id?: number;
  vendor?: Vendor;
  purchase_request?: PurchaseRequestSummary | null;
  project?: ProjectSummary | null;
  purchase_items?: PurchaseItem[];
  purchase_payments?: PurchasePayment[];
  payment_terms?: PurchasePaymentTerm[];
  approval_request?: {
    id: number;
    status: string;
    approval_steps: Array<{
      id: number;
      step_id: number;
      status: string;
      is_active: boolean;
      step: {
        id: number;
        step_order: number;
        step_name: string;
        approver_role: string;
      };
    }>;
  };
  created_at: string;
  updated_at: string;
}

export interface PurchaseListResponse {
  data: Purchase[];
  total: number;
  page: number;
  limit: number;
  total_pages: number;
}

export interface PurchaseSummary {
  total_purchases: number;
  total_amount: number;
  total_approved_amount: number;
  total_paid: number;
  total_outstanding: number;
  avg_order_value: number;
  status_counts: { [key: string]: number };
  approval_status_counts: { [key: string]: number };
}

// Purchase Payment Integration Interfaces
export interface PurchasePaymentTermRequest {
  sequence: number;
  percentage?: number;
  amount: number;
  due_date: string;
  description?: string;
}

export interface PurchasePaymentTerm {
  id: number;
  purchase_id: number;
  sequence: number;
  percentage: number;
  amount: number;
  paid_amount?: number;
  outstanding_amount?: number;
  due_date?: string;
  description?: string;
  status: "PENDING" | "PARTIAL" | "PAID" | string;
  created_at?: string;
  updated_at?: string;
}

export interface PurchasePaymentRequest {
  amount: number;
  exchange_rate?: number;
  payment_date: string; // ISO string
  payment_method: string;
  cash_bank_id?: number;
  payment_term_id?: number;
  reference?: string;
  notes?: string;
}

export interface PurchasePPhPaymentRequest {
  payment_date: string;
  amount: number;
  cash_bank_id: number;
  reference?: string;
  notes?: string;
}

export interface PurchasePayment {
  id: number;
  purchase_id: number;
  payment_number: string;
  date: string;
  amount: number;
  method: string;
  reference?: string;
  notes?: string;
  cash_bank_id?: number;
  user_id: number;
  payment_id?: number;
  purchase_payment_term_id?: number | null;
  payment_proof_file_name?: string;
  payment_proof_file_path?: string;
  payment_proof_file_size?: number;
  payment_proof_mime_type?: string;
  payment_proof_uploaded_by?: number;
  payment_proof_uploaded_at?: string;
  payment_proof_review_status?: "PENDING" | "APPROVED" | "REJECTED" | string;
  payment_proof_reviewed_by?: number;
  payment_proof_reviewed_at?: string;
  payment_proof_review_notes?: string;
  created_at: string;
  updated_at: string;

  // Relations
  cash_bank?: any;
  user?: any;
  payment?: any;
  purchase_payment_term?: PurchasePaymentTerm | null;
  payment_proof_uploader?: any;
  payment_proof_reviewer?: any;
}

export interface PurchaseForPayment {
  purchase_id: number;
  bill_number: string;
  vendor: {
    id: number;
    name: string;
    type: string;
  };
  total_amount: number;
  paid_amount: number;
  outstanding_amount: number;
  status: string;
  payment_method: string;
  date: string;
  due_date?: string;
  can_receive_payment: boolean;
  payment_url_suggestion?: string;
}

class PurchaseService {
  async list(params: PurchaseFilterParams): Promise<PurchaseListResponse> {
    const toUpper = (v?: string) => (v ? v.toUpperCase() : undefined);
    const response = await api.get(API_ENDPOINTS.PURCHASES, {
      params: {
        status: toUpper(params.status),
        vendor_id: params.vendor_id,
        project_id: params.project_id,
        start_date: params.start_date,
        end_date: params.end_date,
        search: params.search,
        approval_status: toUpper(params.approval_status),
        accounting_status: toUpper(params.accounting_status),
        requires_approval: params.requires_approval,
        outstanding: params.outstanding,
        page: params.page ?? 1,
        limit: params.limit ?? 10,
      },
    });
    return response.data;
  }

  async create(payload: PurchaseCreateRequest): Promise<Purchase> {
    const response = await api.post(API_ENDPOINTS.PURCHASES, payload);
    return response.data;
  }

  async submitForApproval(id: number): Promise<{ message: string }> {
    const response = await api.post(
      API_ENDPOINTS.PURCHASES_SUBMIT_APPROVAL(id),
    );
    return response.data;
  }

  async getById(id: number): Promise<Purchase> {
    const response = await api.get(API_ENDPOINTS.PURCHASES_BY_ID(id));
    return response.data;
  }

  async update(
    id: number,
    payload: Partial<PurchaseCreateRequest>,
  ): Promise<Purchase> {
    const response = await api.put(API_ENDPOINTS.PURCHASES_BY_ID(id), payload);
    return response.data;
  }

  async completeAccountingCOA(
    id: number,
    payload: PurchaseAccountingCOARequest,
  ): Promise<Purchase> {
    const response = await api.post(
      API_ENDPOINTS.PURCHASES_ACCOUNTING_COA(id),
      payload,
    );
    return response.data?.purchase ?? response.data;
  }

  async listCOARevisions(id: number): Promise<PurchaseCOARevision[]> {
    const response = await api.get(API_ENDPOINTS.PURCHASES_COA_REVISIONS(id));
    return response.data?.data ?? [];
  }

  async requestCOARevision(
    id: number,
    payload: { reason: string },
  ): Promise<PurchaseCOARevision> {
    const response = await api.post(
      API_ENDPOINTS.PURCHASES_COA_REVISIONS(id),
      payload,
    );
    return response.data?.revision ?? response.data;
  }

  async decideCOARevision(
    id: number,
    revisionId: number,
    payload: { approve: boolean; notes?: string },
  ): Promise<PurchaseCOARevision> {
    const response = await api.post(
      API_ENDPOINTS.PURCHASES_COA_REVISION_DECISION(id, revisionId),
      payload,
    );
    return response.data?.revision ?? response.data;
  }

  async listPrePostingRevisions(
    id: number,
  ): Promise<PurchasePrePostingRevision[]> {
    const response = await api.get(
      API_ENDPOINTS.PURCHASES_PRE_POSTING_REVISIONS(id),
    );
    return response.data?.data ?? [];
  }

  async requestPrePostingRevision(
    id: number,
    payload: {
      revision_types: PurchasePrePostingRevisionType[];
      reason: string;
    },
  ): Promise<PurchasePrePostingRevision> {
    const response = await api.post(
      API_ENDPOINTS.PURCHASES_PRE_POSTING_REVISIONS(id),
      payload,
    );
    return response.data?.revision ?? response.data;
  }

  async decidePrePostingRevision(
    id: number,
    revisionId: number,
    payload: { approve: boolean; notes?: string },
  ): Promise<PurchasePrePostingRevision> {
    const response = await api.post(
      API_ENDPOINTS.PURCHASES_PRE_POSTING_REVISION_DECISION(id, revisionId),
      payload,
    );
    return response.data?.revision ?? response.data;
  }

  async updateFinalReviewConfig(
    id: number,
    payload: PurchaseFinalReviewConfigRequest,
  ): Promise<Purchase> {
    const response = await api.post(
      API_ENDPOINTS.PURCHASES_FINAL_REVIEW_CONFIG(id),
      payload,
    );
    return response.data?.purchase ?? response.data;
  }

  async finalApproveAfterCOA(
    id: number,
    payload: { comments?: string } = {},
  ): Promise<Purchase> {
    const response = await api.post(
      API_ENDPOINTS.PURCHASES_FINAL_APPROVAL(id),
      payload,
    );
    return response.data?.purchase ?? response.data;
  }

  async delete(id: number): Promise<{ message: string }> {
    const response = await api.delete(API_ENDPOINTS.PURCHASES_BY_ID(id));
    return response.data;
  }

  async getPendingApproval(
    page = 1,
    limit = 10,
  ): Promise<PurchaseListResponse> {
    return this.list({
      approval_status: "PENDING",
      page,
      limit,
    });
  }

  async getSummary(
    filters: Partial<PurchaseFilterParams> = {},
  ): Promise<PurchaseSummary> {
    const toUpper = (v?: string) => (v ? v.toUpperCase() : undefined);
    const response = await api.get(API_ENDPOINTS.PURCHASES_SUMMARY, {
      params: {
        status: toUpper(filters.status),
        vendor_id: filters.vendor_id,
        project_id: filters.project_id,
        start_date: filters.start_date,
        end_date: filters.end_date,
        search: filters.search,
        approval_status: toUpper(filters.approval_status),
        accounting_status: toUpper(filters.accounting_status),
        requires_approval: filters.requires_approval,
        outstanding: filters.outstanding,
      },
    });
    return response.data;
  }

  // Purchase Payment Integration Methods

  async getPurchaseForPayment(id: number): Promise<PurchaseForPayment> {
    const response = await api.get(API_ENDPOINTS.PURCHASES_FOR_PAYMENT(id));
    return response.data;
  }

  async createIntegratedPayment(
    purchaseId: number,
    data: PurchasePaymentRequest,
  ): Promise<any> {
    // Map frontend field names to backend expected field names
    const backendData = {
      amount: data.amount, // ✅ Correct
      date: data.payment_date, // ✅ Fixed: backend expects "date" not "payment_date"
      method: data.payment_method, // ✅ Fixed: backend expects "method" not "payment_method"
      cash_bank_id: data.cash_bank_id, // ✅ Correct
      payment_term_id: data.payment_term_id,
      reference: data.reference || "", // ✅ Correct
      notes: data.notes || "", // ✅ Correct
    };
    const response = await api.post(
      API_ENDPOINTS.PURCHASES_INTEGRATED_PAYMENT(purchaseId),
      backendData,
    );
    return response.data;
  }

  // NEW: Direct Payment Management Integration
  async createPurchasePayment(
    purchaseId: number,
    data: PurchasePaymentRequest,
  ): Promise<any> {
    // This method uses the new /purchases/:id/payments endpoint that integrates with Payment Management

    // Ensure date is properly formatted
    let formattedDate = data.payment_date;
    if (formattedDate && !formattedDate.includes("T")) {
      // If date is just YYYY-MM-DD, add time component for consistency
      formattedDate = `${formattedDate}T00:00:00.000Z`;
    }

    const backendData = {
      amount: data.amount,
      exchange_rate: data.exchange_rate,
      date: formattedDate, // Send as properly formatted date string
      method: data.payment_method,
      cash_bank_id: data.cash_bank_id,
      payment_term_id: data.payment_term_id,
      reference: data.reference || "",
      notes: data.notes || "",
    };

    console.log("Sending payment data to backend:", backendData);
    // Use extended timeout for payment operations (30 seconds)
    const response = await api.post(
      API_ENDPOINTS.PURCHASES_PAYMENTS(purchaseId),
      backendData,
      {
        timeout: 30000, // 30 seconds timeout for payment operations
      },
    );
    return response.data;
  }

  async payPurchasePPh(
    purchaseId: number,
    data: PurchasePPhPaymentRequest,
  ): Promise<Purchase> {
    const response = await api.post(
      API_ENDPOINTS.PURCHASES_PAY_PPH(purchaseId),
      data,
    );
    return response.data?.purchase ?? response.data?.data ?? response.data;
  }

  async getPurchasePayments(purchaseId: number): Promise<PurchasePayment[]> {
    const response = await api.get(
      API_ENDPOINTS.PURCHASES_PAYMENTS(purchaseId),
    );
    return response.data;
  }

  async uploadPaymentProof(
    purchaseId: number,
    file: File,
  ): Promise<PurchasePayment> {
    const formData = new FormData();
    formData.append("payment_proof", file);

    const response = await api.post(
      API_ENDPOINTS.PURCHASES_PAYMENT_PROOF(purchaseId),
      formData,
      {
        headers: { "Content-Type": "multipart/form-data" },
        timeout: 30000,
      },
    );
    return response.data?.payment ?? response.data;
  }

  async uploadCreditPurchasePaymentProof(
    purchaseId: number,
    paymentId: number,
    file: File,
  ): Promise<PurchasePayment> {
    const formData = new FormData();
    formData.append("payment_proof", file);

    const response = await api.post(
      API_ENDPOINTS.PURCHASES_PAYMENT_PROOF_FOR_PAYMENT(
        purchaseId,
        paymentId,
      ),
      formData,
      {
        headers: { "Content-Type": "multipart/form-data" },
        timeout: 30000,
      },
    );
    return response.data?.payment ?? response.data;
  }

  async reviewPaymentProof(
    purchasePaymentId: number,
    payload: { status: "APPROVED" | "REJECTED"; notes?: string },
  ): Promise<PurchasePayment> {
    const response = await api.post(
      API_ENDPOINTS.PURCHASES_PAYMENT_PROOF_REVIEW(purchasePaymentId),
      payload,
    );
    return response.data?.payment ?? response.data;
  }

  async downloadPaymentProof(
    purchasePaymentId: number,
    fallbackName = "bukti-pembayaran",
    fromPurchaseRequest = false,
  ): Promise<void> {
    const endpoint = fromPurchaseRequest
      ? API_ENDPOINTS.PURCHASE_REQUEST_PAYMENT_PROOF_DOWNLOAD(purchasePaymentId)
      : API_ENDPOINTS.PURCHASES_PAYMENT_PROOF_DOWNLOAD(purchasePaymentId);
    const response = await api.get(endpoint, { responseType: "blob" });
    const filename = getDownloadFilename(
      response.headers?.["content-disposition"],
      fallbackName,
    );

    const url = window.URL.createObjectURL(response.data);
    const link = document.createElement("a");
    link.href = url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    window.URL.revokeObjectURL(url);
  }

  async downloadDocument(
    documentId: number,
    fallbackName = "purchase-document",
  ): Promise<void> {
    const response = await api.get(API_ENDPOINTS.PURCHASES_DOCUMENT_DOWNLOAD(documentId), {
      responseType: "blob",
    });
    const filename = getDownloadFilename(
      response.headers?.["content-disposition"],
      fallbackName,
    );

    const url = window.URL.createObjectURL(response.data);
    const link = document.createElement("a");
    link.href = url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    window.URL.revokeObjectURL(url);
  }

  // Helper method to check if purchase can receive payment
  canReceivePayment(purchase: Purchase): boolean {
    const status = (purchase.status || "").toUpperCase();
    const eligibleStatus =
      status === "APPROVED" || status === "COMPLETED" || status === "PAID";
    return (
      eligibleStatus &&
      purchase.payment_method === "CREDIT" &&
      (purchase.outstanding_amount || 0) > 0
    );
  }

  // Helper method to format currency for payment display
  formatPaymentAmount(amount: number): string {
    const hasFraction = Math.abs((amount || 0) % 1) > Number.EPSILON;

    return new Intl.NumberFormat("id-ID", {
      style: "currency",
      currency: "IDR",
      minimumFractionDigits: hasFraction ? 2 : 0,
      maximumFractionDigits: 2,
    }).format(amount);
  }

  // Export functionality

  async exportPurchasesPDF(filters?: PurchaseFilterParams): Promise<Blob> {
    try {
      const params = new URLSearchParams();
      const toUpper = (v?: string) => (v ? v.toUpperCase() : undefined);

      if (filters?.status) params.append("status", toUpper(filters.status)!);
      if (filters?.vendor_id) params.append("vendor_id", filters.vendor_id);
      if (filters?.project_id) params.append("project_id", filters.project_id);
      if (filters?.start_date) params.append("start_date", filters.start_date);
      if (filters?.end_date) params.append("end_date", filters.end_date);
      if (filters?.search) params.append("search", filters.search);
      if (filters?.approval_status)
        params.append("approval_status", toUpper(filters.approval_status)!);
      if (filters?.accounting_status)
        params.append("accounting_status", toUpper(filters.accounting_status)!);
      if (filters?.requires_approval !== undefined)
        params.append(
          "requires_approval",
          filters.requires_approval.toString(),
        );
      if (filters?.outstanding !== undefined)
        params.append("outstanding", filters.outstanding.toString());

      const response = await api.get(
        `${API_ENDPOINTS.PURCHASES_EXPORT_PDF}?${params}`,
        {
          responseType: "blob",
          headers: { Accept: "application/pdf" },
        },
      );

      return response.data;
    } catch (error: any) {
      console.error("Error exporting purchases PDF:", error);
      throw new Error(
        error.response?.data?.error || "Failed to export purchases PDF",
      );
    }
  }

  async exportPurchasesCSV(filters?: PurchaseFilterParams): Promise<Blob> {
    try {
      const params = new URLSearchParams();
      const toUpper = (v?: string) => (v ? v.toUpperCase() : undefined);

      if (filters?.status) params.append("status", toUpper(filters.status)!);
      if (filters?.vendor_id) params.append("vendor_id", filters.vendor_id);
      if (filters?.project_id) params.append("project_id", filters.project_id);
      if (filters?.start_date) params.append("start_date", filters.start_date);
      if (filters?.end_date) params.append("end_date", filters.end_date);
      if (filters?.search) params.append("search", filters.search);
      if (filters?.approval_status)
        params.append("approval_status", toUpper(filters.approval_status)!);
      if (filters?.accounting_status)
        params.append("accounting_status", toUpper(filters.accounting_status)!);
      if (filters?.requires_approval !== undefined)
        params.append(
          "requires_approval",
          filters.requires_approval.toString(),
        );
      if (filters?.outstanding !== undefined)
        params.append("outstanding", filters.outstanding.toString());

      const response = await api.get(
        `${API_ENDPOINTS.PURCHASES_EXPORT_CSV}?${params}`,
        {
          responseType: "blob",
          headers: { Accept: "text/csv" },
        },
      );

      return response.data;
    } catch (error: any) {
      console.error("Error exporting purchases CSV:", error);
      throw new Error(
        error.response?.data?.error || "Failed to export purchases CSV",
      );
    }
  }

  // Download helper methods

  async downloadPurchasesPDF(filters?: PurchaseFilterParams): Promise<void> {
    try {
      const blob = await this.exportPurchasesPDF(filters);
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;

      // Create filename based on filters
      let filename = "purchases-report";
      if (filters?.start_date && filters?.end_date) {
        filename += `_${filters.start_date}_to_${filters.end_date}`;
      } else if (filters?.start_date) {
        filename += `_from_${filters.start_date}`;
      } else if (filters?.end_date) {
        filename += `_until_${filters.end_date}`;
      }
      filename += ".pdf";

      link.download = filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
    } catch (error: any) {
      console.error("Error downloading purchases PDF:", error);
      throw error;
    }
  }

  async downloadPurchasesCSV(filters?: PurchaseFilterParams): Promise<void> {
    try {
      const blob = await this.exportPurchasesCSV(filters);
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;

      // Create filename based on filters
      let filename = "purchases-report";
      if (filters?.start_date && filters?.end_date) {
        filename += `_${filters.start_date}_to_${filters.end_date}`;
      } else if (filters?.start_date) {
        filename += `_from_${filters.start_date}`;
      } else if (filters?.end_date) {
        filename += `_until_${filters.end_date}`;
      }
      filename += ".csv";

      link.download = filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
    } catch (error: any) {
      console.error("Error downloading purchases CSV:", error);
      throw error;
    }
  }
}

const purchaseService = new PurchaseService();
export default purchaseService;
