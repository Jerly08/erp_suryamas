import api from "./api";
import { API_ENDPOINTS } from "@/config/api";

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
  notes?: string;
  items: PurchaseItemRequest[]; // backend expects `items`
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
  pph21_rate?: number;
  pph23_rate?: number;
  other_tax_deductions?: number;
  payment_method?: string;
  bank_account_id?: number;
  credit_account_id?: number;
  payment_reference?: string;
  notes?: string;
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

export interface PurchaseRequestSummary {
  id: number;
  pr_number?: string;
  request_number?: string;
  title?: string;
  department?: string;
  project?: {
    id: number;
    code: string;
    name: string;
  };
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
export interface PurchasePaymentRequest {
  amount: number;
  exchange_rate?: number;
  payment_date: string; // ISO string
  payment_method: string;
  cash_bank_id?: number;
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
  created_at: string;
  updated_at: string;

  // Relations
  cash_bank?: any;
  user?: any;
  payment?: any;
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
