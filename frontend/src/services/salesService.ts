import api from "./api";
import { formatDateWithIndonesianMonth } from "../utils/dataFormatters";
import { API_ENDPOINTS } from "@/config/api";

export interface Sale {
  id: number;
  code: string;
  invoice_number?: string;
  quotation_number?: string;
  customer_id: number;
  user_id: number;
  sales_person_id?: number;
  invoice_type_id?: number; // Selected invoice type (affects numbering)
  project_id?: number;
  project?: {
    id: number;
    name: string;
    code?: string;
  };
  type: "QUOTATION" | "ORDER" | "INVOICE" | "SALE";
  status:
    | "DRAFT"
    | "PENDING"
    | "CONFIRMED"
    | "INVOICED"
    | "PAID"
    | "OVERDUE"
    | "CANCELLED";
  settlement_status?:
    | "DRAFT"
    | "PENDING"
    | "CONFIRMED"
    | "INVOICED"
    | "PARTIALLY_PAID"
    | "PAID"
    | "OVERDUE"
    | "CANCELLED";
  date: string;
  due_date?: string;
  valid_until?: string;
  currency: string;
  exchange_rate: number;
  subtotal: number;
  sub_total: number; // Read-only alias for backend compatibility
  discount_percent: number;
  discount_amount: number;
  discount: number; // Legacy field for compatibility
  taxable_amount: number;
  net_before_tax?: number;
  ppn: number;
  ppn_percent: number;
  pph: number;
  pph_percent: number;
  pph_type?: string;
  // Enhanced tax configuration
  ppn_rate?: number;
  ppn_amount?: number;
  other_tax_additions?: number;
  pph21_rate?: number;
  pph21_amount?: number;
  pph23_rate?: number;
  pph23_amount?: number;
  pph23_settlement_date?: string;
  pph23_settlement_reference?: string;
  pph23_settlement_notes?: string;
  pph23_settlement_journal_entry_id?: number;
  pph_paid_amount?: number;
  pph_payment_status?: "PAID" | "UNPAID" | "PARTIAL_PAID" | "NONE";
  other_tax_deductions?: number;
  total_tax: number;
  tax: number; // Backend legacy field
  total_amount: number;
  paid_amount: number;
  outstanding_amount: number;
  payment_terms: string;
  payment_schedules?: SalePaymentSchedule[];
  payment_method?: string;
  payment_method_type?: string; // CASH, BANK, CREDIT
  cash_bank_id?: number;
  receivable_account_id?: number;
  shipping_method?: string;
  shipping_cost: number;
  shipping_taxable: boolean;
  billing_address?: string;
  shipping_address?: string;
  notes?: string;
  internal_notes?: string;
  reference?: string;
  created_at: string;
  updated_at: string;

  // Relations
  customer?: any;
  user?: any;
  sales_person?: any;
  sale_items?: SaleItem[];
  sale_payments?: SalePayment[];
  sale_returns?: SaleReturn[];
  receivable_account?: {
    id: number;
    code: string;
    name: string;
  };

  // Backward compatibility
  items?: SaleItem[];
}

export interface SaleItem {
  id?: number;
  sale_id?: number;
  product_id: number;
  product_name?: string;
  description?: string;
  location?: string;
  quantity: number;
  unit_price: number;
  discount_percent: number;
  discount_amount: number;
  line_total: number;
  taxable: boolean;
  ppn_amount: number;
  pph_amount: number;
  total_tax: number;
  final_amount: number;
  available_quantity?: number;
  fulfilled_quantity?: number;
  backorder_quantity?: number;
  estimated_unit_cost?: number;
  estimated_cogs_amount?: number;
  actual_cogs_amount?: number;
  cogs_variance_amount?: number;
  fulfillment_status?:
    | "SERVICE"
    | "IN_STOCK"
    | "PARTIAL_BACKORDER"
    | "BACKORDERED"
    | "FULFILLED";
  revenue_account_id?: number;
  tax_account_id?: number;

  // Legacy fields for backward compatibility
  total_price: number; // Same as line_total
  discount: number; // Legacy discount field
  tax: number; // Legacy tax field

  // Relations
  product?: any;
}

export interface SalePaymentSchedule {
  id?: number;
  sale_id?: number;
  sequence: number;
  percentage: number;
  amount: number;
  paid_amount?: number;
  outstanding_amount?: number;
  due_date: string;
  description?: string;
  status?: string;
}

export interface SalePayment {
  id: number;
  sale_id: number;
  payment_number: string;
  date: string;
  amount: number;
  native_amount?: number;
  currency?: string;
  exchange_rate?: number;
  book_amount_idr?: number;
  forex_gain_loss_amount?: number;
  forex_gain_loss_type?: string;
  method: string;
  reference?: string;
  cash_bank_id?: number;
  account_id?: number; // Make optional to match backend
  cash_bank_code?: string;
  cash_bank_name?: string;
  cash_bank_currency?: string;
  cash_bank_gl_account_id?: number;
  cash_bank_gl_account_code?: string;
  cash_bank_gl_account_name?: string;
  notes?: string;
  user_id: number;

  // Relations
  cash_bank?: any;
  account?: any;
  user?: any;
}

export interface SaleReturn {
  id: number;
  sale_id: number;
  return_number: string;
  credit_note_number?: string;
  date: string;
  type: "RETURN" | "CREDIT_NOTE";
  reason: string;
  total_amount: number;
  status: "PENDING" | "APPROVED" | "REJECTED";
  notes?: string;
  user_id: number;
  approved_by?: number;
  approved_at?: string;

  // Relations
  return_items?: SaleReturnItem[];
}

export interface SaleReturnItem {
  id: number;
  sale_return_id: number;
  sale_item_id: number;
  quantity: number;
  unit_price: number;
  total_amount: number;
  reason?: string;
}

export interface SaleCreateRequest {
  customer_id: number;
  sales_person_id?: number;
  invoice_type_id?: number; // ✅ ensure invoice type is sent on create
  project_id?: number;
  type: string;
  date: string; // ISO datetime string format for Go backend (e.g., '2025-08-15T00:00:00Z')
  due_date?: string;
  valid_until?: string;
  currency?: string;
  exchange_rate?: number;
  discount_percent?: number;
  ppn_percent?: number;
  pph_percent?: number;
  pph_type?: string;
  // Enhanced tax configuration
  ppn_rate?: number;
  other_tax_additions?: number;
  pph21_rate?: number;
  pph23_rate?: number;
  other_tax_deductions?: number;
  payment_terms?: string;
  payment_method?: string;
  payment_method_type?: string; // CASH, BANK, CREDIT
  cash_bank_id?: number;
  receivable_account_id?: number;
  shipping_method?: string;
  shipping_cost?: number;
  shipping_taxable?: boolean;
  billing_address?: string;
  shipping_address?: string;
  notes?: string;
  internal_notes?: string;
  reference?: string;
  payment_schedules?: SalePaymentScheduleRequest[];
  items: SaleItemRequest[]; // Use consistent interface
}

export interface SaleItemRequest {
  product_id: number;
  product_name?: string;
  description?: string;
  location?: string;
  quantity: number;
  unit_price: number;
  estimated_unit_cost?: number;
  discount?: number; // Legacy field - will be mapped to discount_percent
  discount_percent?: number; // New field to match backend
  tax?: number; // Legacy field
  taxable?: boolean; // New field to match backend
  revenue_account_id?: number;
}

export interface SaleUpdateRequest {
  customer_id?: number;
  sales_person_id?: number;
  invoice_type_id?: number; // allow updating invoice type before invoicing
  project_id?: number;
  date?: string; // ISO datetime string format for Go backend (e.g., '2025-08-15T00:00:00Z')
  due_date?: string;
  valid_until?: string;
  discount_percent?: number;
  ppn_percent?: number;
  pph_percent?: number;
  pph_type?: string;
  // Enhanced tax configuration
  ppn_rate?: number;
  other_tax_additions?: number;
  pph21_rate?: number;
  pph23_rate?: number;
  other_tax_deductions?: number;
  payment_terms?: string;
  payment_method?: string;
  payment_method_type?: string; // CASH, BANK, CREDIT
  cash_bank_id?: number;
  receivable_account_id?: number;
  shipping_method?: string;
  shipping_cost?: number;
  shipping_taxable?: boolean;
  billing_address?: string;
  shipping_address?: string;
  notes?: string;
  internal_notes?: string;
  reference?: string;
  payment_schedules?: SalePaymentScheduleRequest[];
  items?: SaleItemUpdateRequest[];
}

export interface SaleItemUpdateRequest {
  id?: number;
  product_id: number;
  product_name?: string;
  description?: string;
  location?: string;
  quantity: number;
  unit_price: number;
  estimated_unit_cost?: number;
  discount?: number; // Legacy field - will be mapped to discount_percent
  discount_percent?: number; // New field to match backend
  tax?: number; // Legacy field
  taxable?: boolean;
  revenue_account_id?: number;
  delete?: boolean;
}

export interface SalePaymentScheduleRequest {
  sequence?: number;
  percentage: number;
  due_date: string;
  description?: string;
}

export interface SalePaymentRequest {
  payment_date: string; // Align with backend field name
  amount: number;
  payment_method: string; // Align with backend field name
  exchange_rate?: number;
  reference?: string;
  cash_bank_id?: number;
  account_id?: number; // Make optional to match backend
  notes?: string;
}

export interface SalePPh23SettlementRequest {
  date: string;
  reference?: string;
  notes?: string;
  reduce_outstanding?: boolean;
}

export interface SaleLatePPh23Request {
  date: string;
  amount?: number;
  rate?: number;
  reference?: string;
  notes?: string;
  reduce_outstanding?: boolean;
}

export interface IntegratedSalePaymentResponse {
  success: boolean;
  status: string;
  message: string;
  payment: SalePayment;
  updated_sale?: {
    id: number;
    status: Sale["status"];
    settlement_status?: NonNullable<Sale["settlement_status"]>;
    paid_amount: number;
    outstanding_amount: number;
  };
}

export interface SaleReturnRequest {
  return_date: string; // Match backend field name
  date?: string; // Deprecated, use return_date instead
  type?: string; // Optional in backend
  reason: string;
  notes?: string;
  return_items: SaleReturnItemRequest[]; // Match backend field name
  items?: SaleReturnItemRequest[]; // Deprecated, use return_items instead
}

export interface SaleReturnItemRequest {
  sale_item_id: number;
  quantity: number;
  reason?: string;
}

export interface SalesFilter {
  page?: number;
  limit?: number;
  status?: string;
  customer_id?: string;
  start_date?: string;
  end_date?: string;
  search?: string;
}

export interface SalesResult {
  data: Sale[];
  total: number;
  page: number;
  limit: number;
  total_pages: number;
}

export interface SalesSummary {
  total_sales: number;
  total_amount: number;
  total_paid: number;
  total_outstanding: number;
  avg_order_value: number;
  top_customers: CustomerSales[];
}

export interface CustomerSales {
  customer_id: number;
  customer_name: string;
  total_amount: number;
  total_orders: number;
}

export interface SalesAnalytics {
  period: string;
  data: SalesAnalyticsData[];
}

export interface SalesAnalyticsData {
  period: string;
  total_sales: number;
  total_amount: number;
  growth_rate: number;
}

export interface ReceivablesReport {
  total_outstanding: number;
  overdue_amount: number;
  receivables: ReceivableItem[];
}

export interface ReceivableItem {
  sale_id: number;
  invoice_number: string;
  customer_name: string;
  date: string;
  due_date: string;
  total_amount: number;
  paid_amount: number;
  outstanding_amount: number;
  days_overdue: number;
  status: string;
}

class SalesService {
  // Basic CRUD Operations

  async getSales(filter: SalesFilter = {}): Promise<SalesResult> {
    const params = new URLSearchParams();
    Object.entries(filter).forEach(([key, value]) => {
      if (value !== undefined && value !== null && value !== "") {
        params.append(key, value.toString());
      }
    });

    const response = await api.get(`${API_ENDPOINTS.SALES}?${params}`);
    return response.data;
  }

  async getSale(id: number): Promise<Sale> {
    const response = await api.get(API_ENDPOINTS.SALES_BY_ID(id));
    // Backend commonly wraps responses as { status, message, data }
    return response.data && response.data.data
      ? response.data.data
      : response.data;
  }

  async createSale(data: SaleCreateRequest): Promise<Sale> {
    const response = await api.post(API_ENDPOINTS.SALES, data);
    return response.data;
  }

  async updateSale(id: number, data: SaleUpdateRequest): Promise<Sale> {
    const response = await api.put(API_ENDPOINTS.SALES_BY_ID(id), data);
    return response.data;
  }

  async deleteSale(id: number): Promise<void> {
    await api.delete(API_ENDPOINTS.SALES_BY_ID(id));
  }

  // Status Management

  async confirmSale(id: number): Promise<void> {
    await api.post(API_ENDPOINTS.SALES_CONFIRM(id));
  }

  async createInvoiceFromSale(id: number): Promise<Sale> {
    const response = await api.post(API_ENDPOINTS.SALES_INVOICE(id));
    return response.data;
  }

  async cancelSale(id: number, reason: string): Promise<void> {
    await api.post(API_ENDPOINTS.SALES_CANCEL(id), { reason });
  }

  // Payment Management

  async getSalePayments(saleId: number): Promise<SalePayment[]> {
    const response = await api.get(API_ENDPOINTS.SALES_PAYMENTS(saleId));
    return response.data;
  }

  async createSalePayment(
    saleId: number,
    data: SalePaymentRequest,
  ): Promise<SalePayment> {
    // Transform frontend format to backend format
    const backendData = {
      sale_id: saleId,
      amount: data.amount,
      payment_date: data.payment_date,
      payment_method: data.payment_method,
      exchange_rate: data.exchange_rate,
      reference: data.reference,
      notes: data.notes,
      cash_bank_id: data.cash_bank_id,
      account_id: data.account_id,
    };
    const response = await api.post(
      API_ENDPOINTS.SALES_PAYMENTS(saleId),
      backendData,
    );
    return response.data;
  }

  async settleSalePPh23(
    saleId: number,
    data: SalePPh23SettlementRequest,
  ): Promise<Sale> {
    const response = await api.post(
      API_ENDPOINTS.SALES_PPH23_SETTLEMENT(saleId),
      data,
    );
    return response.data && response.data.data
      ? response.data.data
      : response.data;
  }

  async addLateSalePPh23(
    saleId: number,
    data: SaleLatePPh23Request,
  ): Promise<Sale> {
    const response = await api.post(API_ENDPOINTS.SALES_PPH23_ADD(saleId), data);
    return response.data && response.data.data
      ? response.data.data
      : response.data;
  }

  // NEW: Integrated Payment Management endpoint
  async createIntegratedPayment(
    saleId: number,
    data: SalePaymentRequest,
  ): Promise<IntegratedSalePaymentResponse> {
    // This endpoint creates payment records in both Sales and Payment Management
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
      API_ENDPOINTS.SALES_INTEGRATED_PAYMENT(saleId),
      {
        ...backendData,
        exchange_rate: data.exchange_rate || 1,
      },
    );
    return response.data;
  }

  // Returns and Credit Notes

  async createSaleReturn(
    saleId: number,
    data: SaleReturnRequest,
  ): Promise<SaleReturn> {
    // Transform frontend format to backend format
    const backendData = {
      sale_id: saleId,
      return_date: data.return_date || data.date || new Date().toISOString(),
      reason: data.reason,
      notes: data.notes,
      return_items: data.return_items || data.items || [],
    };
    const response = await api.post(
      API_ENDPOINTS.SALES_RETURNS(saleId),
      backendData,
    );
    return response.data;
  }

  async getSaleReturns(
    page: number = 1,
    limit: number = 10,
  ): Promise<SaleReturn[]> {
    const response = await api.get(
      `${API_ENDPOINTS.SALES_ALL_RETURNS}?page=${page}&limit=${limit}`,
    );
    return response.data;
  }

  // Reporting and Analytics

  async getSalesSummary(
    filters: Partial<SalesFilter> = {},
  ): Promise<SalesSummary> {
    const params = new URLSearchParams();
    if (filters.start_date) params.append("start_date", filters.start_date);
    if (filters.end_date) params.append("end_date", filters.end_date);
    if (filters.status) params.append("status", filters.status);
    if (filters.customer_id) params.append("customer_id", filters.customer_id);
    if (filters.search) params.append("search", filters.search);

    const response = await api.get(`${API_ENDPOINTS.SALES_SUMMARY}?${params}`);
    // Backend wraps responses as { status, message, data }
    return response.data && response.data.data
      ? response.data.data
      : response.data;
  }

  async getSalesAnalytics(
    period: string = "monthly",
    year: string = "2024",
  ): Promise<SalesAnalytics> {
    const response = await api.get(
      `${API_ENDPOINTS.SALES_ANALYTICS}?period=${period}&year=${year}`,
    );
    return response.data;
  }

  async getReceivablesReport(): Promise<ReceivablesReport> {
    const response = await api.get(API_ENDPOINTS.SALES_RECEIVABLES);
    return response.data;
  }

  // Customer Portal

  async getCustomerSales(
    customerId: number,
    page: number = 1,
    limit: number = 10,
  ): Promise<Sale[]> {
    const response = await api.get(
      `${API_ENDPOINTS.SALES_CUSTOMER(customerId)}?page=${page}&limit=${limit}`,
    );
    return response.data;
  }

  async getCustomerInvoices(customerId: number): Promise<Sale[]> {
    const response = await api.get(
      API_ENDPOINTS.SALES_CUSTOMER_INVOICES(customerId),
    );
    return response.data;
  }

  // PDF Export

  async exportInvoicePDF(saleId: number): Promise<Blob> {
    try {
      const response = await api.get(API_ENDPOINTS.SALES_INVOICE_PDF(saleId), {
        responseType: "blob",
        headers: {
          Accept: "application/pdf",
        },
        // Add timestamp to avoid any caching issues in dev
        params: { t: Date.now() },
      });
      return response.data;
    } catch (err: any) {
      // If server returned JSON error with blob, try to decode for better message
      const res = err?.response;
      if (res && res.data instanceof Blob) {
        try {
          const text = await res.data.text();
          // Try parse JSON
          try {
            const json = JSON.parse(text);
            err.message = json.error || json.message || err.message;
          } catch {
            err.message = text || err.message;
          }
        } catch {}
      }
      throw err;
    }
  }

  async exportSalesReportPDF(
    startDate?: string,
    endDate?: string,
    status?: string,
    search?: string,
  ): Promise<Blob> {
    const params = new URLSearchParams();
    if (startDate) params.append("start_date", startDate);
    if (endDate) params.append("end_date", endDate);
    if (status) params.append("status", status);
    if (search) params.append("search", search);

    const response = await api.get(
      `${API_ENDPOINTS.SALES_REPORT_PDF}?${params}`,
      {
        responseType: "blob",
      },
    );
    return response.data;
  }

  async exportSalesReportCSV(
    startDate?: string,
    endDate?: string,
    status?: string,
    search?: string,
  ): Promise<Blob> {
    const params = new URLSearchParams();
    if (startDate) params.append("start_date", startDate);
    if (endDate) params.append("end_date", endDate);
    if (status) params.append("status", status);
    if (search) params.append("search", search);

    const response = await api.get(
      `${API_ENDPOINTS.SALES_REPORT_CSV}?${params}`,
      {
        responseType: "blob",
        headers: { Accept: "text/csv" },
      },
    );
    return response.data;
  }

  async exportReceiptPDF(saleId: number): Promise<Blob> {
    try {
      const response = await api.get(API_ENDPOINTS.SALES_RECEIPT_PDF(saleId), {
        responseType: "blob",
        headers: { Accept: "application/pdf" },
        params: { t: Date.now() },
      });
      return response.data;
    } catch (err: any) {
      // Try to decode JSON error from blob
      const res = err?.response;
      if (res && res.data instanceof Blob) {
        try {
          const text = await res.data.text();
          try {
            const json = JSON.parse(text);
            err.message = json.error || json.message || err.message;
          } catch {
            err.message = text || err.message;
          }
        } catch {}
      }
      throw err;
    }
  }

  // Helper methods for frontend

  getStatusColor(status: string): string {
    const colors: { [key: string]: string } = {
      DRAFT: "gray",
      PENDING: "yellow",
      CONFIRMED: "blue",
      INVOICED: "purple",
      PARTIALLY_PAID: "orange",
      PAID: "green",
      OVERDUE: "red",
      CANCELLED: "red",
    };
    return colors[status] || "gray";
  }

  getStatusLabel(status: string): string {
    const labels: { [key: string]: string } = {
      DRAFT: "Draft",
      PENDING: "Pending",
      CONFIRMED: "Confirmed",
      INVOICED: "Invoiced",
      PARTIALLY_PAID: "Partially Paid",
      PAID: "Paid",
      OVERDUE: "Overdue",
      CANCELLED: "Cancelled",
    };
    return labels[status] || status;
  }

  getTypeLabel(type: string): string {
    const labels: { [key: string]: string } = {
      QUOTATION: "Quotation",
      ORDER: "Order",
      INVOICE: "Invoice",
      SALE: "Sale",
    };
    return labels[type] || type;
  }

  getDisplayStatus(sale: Pick<Sale, "status" | "settlement_status">): string {
    return sale.settlement_status || sale.status;
  }

  formatCashBankSnapshotSummary(
    payment?: Partial<
      Pick<
        SalePayment,
        | "cash_bank_code"
        | "cash_bank_name"
        | "cash_bank_currency"
        | "cash_bank_gl_account_code"
        | "cash_bank_gl_account_name"
      >
    >,
  ): string {
    if (!payment) return "";

    const accountLabel = [payment.cash_bank_code, payment.cash_bank_name]
      .filter(Boolean)
      .join(" - ");
    const currencyLabel = payment.cash_bank_currency
      ? `[${payment.cash_bank_currency.toUpperCase()}]`
      : "";
    const glLabel = [
      payment.cash_bank_gl_account_code,
      payment.cash_bank_gl_account_name,
    ]
      .filter(Boolean)
      .join(" - ");

    return [accountLabel, currencyLabel, glLabel ? `GL ${glLabel}` : ""]
      .filter(Boolean)
      .join(" | ");
  }

  calculateLineTotal(
    quantity: number,
    unitPrice: number,
    discountPercent: number = 0,
  ): number {
    const subtotal = quantity * unitPrice;
    const discountAmount = subtotal * (discountPercent / 100);
    return subtotal - discountAmount;
  }

  calculateTaxAmount(amount: number, taxPercent: number): number {
    return amount * (taxPercent / 100);
  }

  formatCurrency(amount: number, currency: string = "IDR"): string {
    // Handle null, undefined, NaN or invalid values
    if (
      amount === null ||
      amount === undefined ||
      isNaN(amount) ||
      !isFinite(amount)
    ) {
      amount = 0;
    }

    const hasFraction = Math.abs((amount || 0) % 1) > Number.EPSILON;

    return new Intl.NumberFormat("id-ID", {
      style: "currency",
      currency: currency,
      minimumFractionDigits: hasFraction ? 2 : 0,
      maximumFractionDigits: 2,
    }).format(amount);
  }

  formatDate(date: string): string {
    // Check for empty date or default Go zero date
    if (!date || date === "0001-01-01T00:00:00Z" || date === "0001-01-01") {
      return "-";
    }
    // Use Indonesian month names for better clarity (e.g., "9 Juni 2025" instead of "9/6/2025")
    return formatDateWithIndonesianMonth(date);
  }

  // Validation helpers

  validateSaleData(data: SaleCreateRequest): string[] {
    const errors: string[] = [];

    if (!data.customer_id) {
      errors.push("Customer is required");
    }

    if (!data.type) {
      errors.push("Sale type is required");
    }

    if (!data.date) {
      errors.push("Date is required");
    }

    if (
      (data.payment_method_type || "").toUpperCase() === "CREDIT" &&
      (!data.receivable_account_id || data.receivable_account_id <= 0)
    ) {
      errors.push("Receivable account / COA is required for credit sales");
    }

    if (!data.items || data.items.length === 0) {
      errors.push("At least one item is required");
    }

    data.items?.forEach((item, index) => {
      const hasManualName =
        !!item.product_name && item.product_name.trim() !== "";
      if (
        !item.product_id &&
        !hasManualName &&
        (!item.description || item.description.trim() === "")
      ) {
        errors.push(
          `Product, manual product name, or description is required for item ${index + 1}`,
        );
      }
      if (!item.quantity || item.quantity <= 0) {
        errors.push(`Valid quantity is required for item ${index + 1}`);
      }
      if (!item.unit_price || item.unit_price < 0) {
        errors.push(`Valid unit price is required for item ${index + 1}`);
      }
      if (!item.revenue_account_id || item.revenue_account_id <= 0) {
        errors.push(`Revenue account / COA is required for item ${index + 1}`);
      }
    });

    return errors;
  }

  // Generate PDF download

  async downloadInvoicePDF(
    saleId: number,
    invoiceNumber: string,
  ): Promise<void> {
    try {
      const blob = await this.exportInvoicePDF(saleId);
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `Invoice_${invoiceNumber}.pdf`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
    } catch (error) {
      console.error("Error downloading invoice PDF:", error);
      throw error;
    }
  }

  async downloadSalesReportPDF(
    startDate?: string,
    endDate?: string,
    status?: string,
    search?: string,
  ): Promise<void> {
    try {
      const blob = await this.exportSalesReportPDF(
        startDate,
        endDate,
        status,
        search,
      );
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      const filename = `Sales_Report_${startDate || "all"}_to_${endDate || "all"}.pdf`;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
    } catch (error) {
      console.error("Error downloading sales report PDF:", error);
      throw error;
    }
  }

  async downloadSalesReportCSV(
    startDate?: string,
    endDate?: string,
    status?: string,
    search?: string,
  ): Promise<void> {
    try {
      const blob = await this.exportSalesReportCSV(
        startDate,
        endDate,
        status,
        search,
      );
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      const filename = `Sales_Report_${startDate || "all"}_to_${endDate || "all"}.csv`;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
    } catch (error) {
      console.error("Error downloading sales report CSV:", error);
      throw error;
    }
  }

  async downloadReceiptPDF(saleId: number, receiptName: string): Promise<void> {
    try {
      const blob = await this.exportReceiptPDF(saleId);
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      const safeName = receiptName || `SALE_${saleId}`;
      link.download = `Receipt_${safeName}.pdf`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
    } catch (error) {
      console.error("Error downloading receipt PDF:", error);
      throw error;
    }
  }
}

export default new SalesService();
