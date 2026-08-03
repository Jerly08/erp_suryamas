import api from './api';
import { API_ENDPOINTS } from '../config/api';

// Types - Updated to match backend Go struct
export interface PaymentActor {
  id: number;
  username?: string;
  first_name?: string;
  last_name?: string;
  email?: string;
  role?: string;
}

export interface PaymentAccountSummary {
  id: number;
  code?: string;
  name: string;
  type?: string;
  currency?: string;
  account?: {
    id: number;
    code?: string;
    name: string;
  };
}

export interface Payment {
  id: number;
  code: string;
  contact_id: number;
  expense_account_id?: number;
  cash_bank_id?: number;
  cash_bank_code?: string;
  cash_bank_name?: string;
  cash_bank_currency?: string;
  cash_bank_gl_account_id?: number;
  cash_bank_gl_account_code?: string;
  cash_bank_gl_account_name?: string;
  expense_account?: PaymentAccountSummary | null;
  cash_bank?: PaymentAccountSummary | null;
  contact?: {
    id: number;
    name: string;
    type: 'CUSTOMER' | 'VENDOR';
  };
  user_id: number;
  date: string;
  amount: number;
  native_amount?: number;
  currency?: string;
  exchange_rate?: number;
  book_amount_idr?: number;
  forex_gain_loss_amount?: number;
  forex_gain_loss_type?: string;
  legacy_non_invoice_eligible?: boolean;
  method: string;
  reference: string;
  status: 'PENDING' | 'PENDING_ACCOUNTING' | 'PENDING_FINAL_REVIEW' | 'COMPLETED' | 'FAILED' | 'DRAFT' | 'REJECTED' | 'REVERSED';
  payment_type?: string; // REGULAR, TAX_PPN, TAX_PPN_INPUT, TAX_PPN_OUTPUT
  pph_type?: string;
  pph_rate?: number;
  pph_amount?: number;
  pph_paid_amount?: number;
  pph_payment_status?: 'PAID' | 'UNPAID' | 'PARTIAL_PAID' | 'NONE';
  approval_stage?: 'FINANCE_MANAGER' | 'ACCOUNTING' | 'FINANCE_MANAGER_FINAL' | 'DIRECTOR' | '';
  require_director_approval?: boolean;
  finance_manager_approved_by?: number | null;
  finance_manager_approved_at?: string | null;
  director_approved_by?: number | null;
  director_approved_at?: string | null;
  accounting_reviewed_by?: number | null;
  accounting_reviewed_at?: string | null;
  approved_by?: number | null;
  approved_at?: string | null;
  rejected_by?: number | null;
  rejected_at?: string | null;
  rejection_reason?: string;
  user?: PaymentActor;
  finance_manager_approver?: PaymentActor | null;
  director_approver?: PaymentActor | null;
  accounting_reviewer?: PaymentActor | null;
  approver?: PaymentActor | null;
  rejector?: PaymentActor | null;
  notes: string;
  created_at: string;
  updated_at: string;
}

export const stripInternalPaymentReference = (reference?: string): string => {
  if (!reference) {
    return '';
  }

  return reference
    .replace(/^__ALLOC_PENDING__\s*/, '')
    .replace(/^__EXP_LIAB_OVERPAY__\s*/, '')
    .replace(/^__ALLOC_PENDING__\s*/, '')
    .replace(/^__EXP_ACC:\d+,CB:\d+__/, '')
    .replace(/^__CB:\d+__/, '')
    .trim();
};

export interface PaymentFilters {
  page?: number;
  limit?: number;
  contact_id?: number;
  status?: string;
  method?: string;
  search?: string;
  start_date?: string;
  end_date?: string;
}

export interface PaymentResult {
  data: Payment[];
  total: number;
  page: number;
  limit: number;
  total_pages: number;
}

export type PaymentTransactionKind =
  | 'SALES_RECEIPT'
  | 'CUSTOMER_RECEIPT'
  | 'OUTGOING_PAYMENT'
  | 'NON_INVOICE';

export interface PaymentTransaction extends Payment {
  row_id: string;
  record_type: 'PAYMENT' | 'SALE_PAYMENT';
  record_id: number;
  transaction_kind: PaymentTransactionKind;
  source_label: string;
  source_reference?: string;
  sale_id?: number;
  is_invoice_receipt: boolean;
  is_read_only: boolean;
}

export interface PaymentTransactionFilters extends PaymentFilters {
  kind?: 'ALL' | 'RECEIPT' | 'OUTGOING' | 'NON_INVOICE';
}

export interface PaymentTransactionResult {
  data: PaymentTransaction[];
  total: number;
  page: number;
  limit: number;
  total_pages: number;
}

export interface PaymentCreateRequest {
  contact_id: number;
  cash_bank_id?: number;
  date: string;
  amount: number;
  exchange_rate?: number;
  method: string;
  reference?: string;
  notes?: string;
  require_director_approval?: boolean;
  allocations?: PaymentAllocation[];
  bill_allocations?: BillAllocation[];
}

export interface PaymentAllocation {
  invoice_id: number;
  amount: number;
}

export interface BillAllocation {
  bill_id: number;
  amount: number;
}

export interface PaymentSummary {
  total_received: number;
  total_paid: number;
  net_flow: number;
  by_method: Record<string, number>;
  status_counts: Record<string, number>;
}

export interface PaymentAnalytics {
  total_received: number;
  total_paid: number;
  net_flow: number;
  received_growth: number;
  paid_growth: number;
  flow_growth: number;
  total_outstanding: number;
  by_method: Record<string, number>;
  daily_trend: Array<{
    date: string;
    received: number;
    paid: number;
  }>;
  recent_payments: Payment[];
  avg_payment_time: number;
  success_rate: number;
}

class PaymentService {
  // Use API_ENDPOINTS instead of hardcoded baseUrl

  private getSSOTPaymentBase(): string {
    return API_ENDPOINTS.PAYMENTS.SSOT.LIST;
  }

  // Get all payments with filters
  async getPayments(filters: PaymentFilters = {}): Promise<PaymentResult> {
    try {
      const params = new URLSearchParams();

      if (filters.page) params.append('page', filters.page.toString());
      if (filters.limit) params.append('limit', filters.limit.toString());
      if (filters.contact_id) params.append('contact_id', filters.contact_id.toString());
      if (filters.status) params.append('status', filters.status);
      if (filters.method) params.append('method', filters.method);
      if (filters.search) params.append('search', filters.search);
      if (filters.start_date) params.append('start_date', filters.start_date);
      if (filters.end_date) params.append('end_date', filters.end_date);

      // Prefer SSOT list route if available; fall back to legacy when not present
      const listCandidates = [
        (API_ENDPOINTS as any).PAYMENTS?.SSOT?.LIST,
        API_ENDPOINTS.PAYMENTS.LIST,
      ].filter(Boolean) as string[];

      let lastError: any = null;
      for (const base of listCandidates) {
        try {
          const response = await api.get(`${base}?${params.toString()}`);
          return response.data;
        } catch (err: any) {
          // If it's not a 404, rethrow immediately; otherwise, try next candidate
          if (err?.response?.status !== 404) {
            throw err;
          }
          lastError = err;
        }
      }

      if (lastError) throw lastError;
      throw new Error('No valid payments endpoint available');
    } catch (error) {
      console.error('Error fetching payments:', error);
      throw error;
    }
  }

  // Lists payments and sales receipts together without changing either source
  // workflow. Invoice receipts remain created from the Sales module.
  async getPaymentTransactions(
    filters: PaymentTransactionFilters = {}
  ): Promise<PaymentTransactionResult> {
    const params = new URLSearchParams();

    if (filters.page) params.append('page', filters.page.toString());
    if (filters.limit) params.append('limit', filters.limit.toString());
    if (filters.contact_id) params.append('contact_id', filters.contact_id.toString());
    if (filters.status) params.append('status', filters.status);
    if (filters.method) params.append('method', filters.method);
    if (filters.search) params.append('search', filters.search);
    if (filters.start_date) params.append('start_date', filters.start_date);
    if (filters.end_date) params.append('end_date', filters.end_date);
    if (filters.kind && filters.kind !== 'ALL') params.append('kind', filters.kind);

    try {
      const endpoint = API_ENDPOINTS.PAYMENTS.SSOT.TRANSACTIONS;
      const response = await api.get(`${endpoint}?${params.toString()}`);
      return response.data;
    } catch (error) {
      console.error('Error fetching payment transactions:', error);
      throw error;
    }
  }

  // Get payment by ID
  async getPaymentById(id: number): Promise<Payment> {
    try {
      const response = await api.get(API_ENDPOINTS.PAYMENTS.GET_BY_ID(id));
      return response.data;
    } catch (error) {
      console.error('Error fetching payment:', error);
      throw error;
    }
  }

  // Create receivable payment (from customer)
  async createReceivablePayment(data: PaymentCreateRequest): Promise<Payment> {
    try {
      // Convert to SSOT format with proper array-based allocations
      const ssotData = {
        contact_id: data.contact_id,
        cash_bank_id: data.cash_bank_id,
        date: this.formatDateForAPI(data.date),
        amount: data.amount,
        exchange_rate: data.exchange_rate,
        method: data.method,
        reference: data.reference || '',
        notes: data.notes || '',
        require_director_approval: !!data.require_director_approval,
        auto_create_journal: true,
        preview_journal: false,
        // 🔥 FIX: Send full invoice_allocations array, not just single target_invoice_id
        ...(data.allocations && data.allocations.length > 0 && {
          invoice_allocations: data.allocations
        })
      };

      // Use a more generous timeout specifically for payment creation,
      // because backend payment + journal processing can legitimately take longer.
      const response = await api.post(API_ENDPOINTS.PAYMENTS.SSOT.RECEIVABLE, ssotData, {
        timeout: 60000 // 60 seconds timeout for receivable payment operations
      });
      // SSOT returns data in response.data.data format
      return response.data.data?.payment || response.data;
    } catch (error: any) {
      console.error('PaymentService - Error creating receivable payment:', error);
      console.log('PaymentService - Error details:', {
        isAuthError: error.isAuthError,
        code: error.code,
        message: error.message,
        responseStatus: error.response?.status,
        responseData: error.response?.data
      });

      // Explicit handling for network timeouts so the UI can show a clearer message
      if (error.code === 'ECONNABORTED' || (typeof error.message === 'string' && error.message.toLowerCase().includes('timeout'))) {
        throw new Error('Permintaan pembuatan pembayaran terlalu lama (timeout). Cek koneksi / status server, lalu pastikan pembayaran belum tercatat sebelum mencoba lagi.');
      }

      // Check if it's an authentication error from API interceptor
      if (error.isAuthError || error.code === 'AUTH_SESSION_EXPIRED' || error.message?.includes('Session expired')) {
        console.log('PaymentService - Detected auth error, throwing auth error');
        const authError = new Error('Session expired. Please login again.');
        (authError as any).isAuthError = true;
        (authError as any).code = 'AUTH_SESSION_EXPIRED';
        throw authError;
      } else if (error.response?.status === 401) {
        const authError = new Error('Session expired. Please login again.');
        (authError as any).isAuthError = true;
        (authError as any).code = 'AUTH_SESSION_EXPIRED';
        throw authError;
      } else if (error.response?.status === 403) {
        throw new Error('You do not have permission to create payments.');
      } else if (error.response?.data?.error) {
        throw new Error(error.response.data.error);
      } else if (error.response?.data?.details) {
        throw new Error(error.response.data.details);
      } else if (error.message) {
        throw new Error(error.message);
      }

      throw error;
    }
  }

  // Create payable payment (to vendor)
  async createPayablePayment(data: PaymentCreateRequest): Promise<Payment> {
    try {
      // Convert to SSOT format with proper array-based allocations
      const ssotData = {
        contact_id: data.contact_id,
        cash_bank_id: data.cash_bank_id,
        date: this.formatDateForAPI(data.date),
        amount: data.amount,
        exchange_rate: data.exchange_rate,
        method: data.method,
        reference: data.reference || '',
        notes: data.notes || '',
        require_director_approval: !!data.require_director_approval,
        auto_create_journal: true,
        preview_journal: false,
        // 🔥 FIX: Send full bill_allocations array, not just single target_bill_id
        ...(data.bill_allocations && data.bill_allocations.length > 0 && {
          bill_allocations: data.bill_allocations
        })
      };

      const response = await api.post(API_ENDPOINTS.PAYMENTS.SSOT.PAYABLE, ssotData, {
        timeout: 60000 // 60 seconds timeout for payable payment operations
      });
      // SSOT returns data in response.data.data format
      return response.data.data?.payment || response.data;
    } catch (error: any) {
      console.error('PaymentService - Error creating payable payment:', error);
      console.log('PaymentService - Error details:', {
        isAuthError: error.isAuthError,
        code: error.code,
        message: error.message,
        responseStatus: error.response?.status,
        responseData: error.response?.data
      });

      // Explicit handling for network timeouts so the UI can show a clearer message
      if (error.code === 'ECONNABORTED' || (typeof error.message === 'string' && error.message.toLowerCase().includes('timeout'))) {
        throw new Error('Permintaan pembayaran hutang terlalu lama (timeout). Cek koneksi / status server, lalu pastikan pembayaran belum tercatat sebelum mencoba lagi.');
      }

      // Check if it's an authentication error from API interceptor
      if (error.isAuthError || error.code === 'AUTH_SESSION_EXPIRED' || error.message?.includes('Session expired')) {
        console.log('PaymentService - Detected auth error, throwing auth error');
        const authError = new Error('Session expired. Please login again.');
        (authError as any).isAuthError = true;
        (authError as any).code = 'AUTH_SESSION_EXPIRED';
        throw authError;
      } else if (error.response?.status === 401) {
        const authError = new Error('Session expired. Please login again.');
        (authError as any).isAuthError = true;
        (authError as any).code = 'AUTH_SESSION_EXPIRED';
        throw authError;
      } else if (error.response?.status === 403) {
        throw new Error('You do not have permission to create payments.');
      } else if (error.response?.data?.error) {
        throw new Error(error.response.data.error);
      } else if (error.response?.data?.details) {
        throw new Error(error.response.data.details);
      } else if (error.message) {
        throw new Error(error.message);
      }

      throw error;
    }
  }

  // Update Draft Payment
  async updateDraftPayment(id: number, data: any): Promise<Payment> {
    try {
      const ssotData = {
        contact_id: data.contact_id,
        expense_account_id: data.expense_account_id,
        cash_bank_id: data.cash_bank_id,
        date: this.formatDateForAPI(data.date),
        amount: data.amount,
        exchange_rate: data.exchange_rate,
        method: data.method,
        reference: data.reference || '',
        notes: data.notes || '',
        description: data.description || '',
        require_director_approval: !!data.require_director_approval,
        allow_liability_overpayment: !!data.allow_liability_overpayment,
        pph_type: data.pph_type || '',
        pph_rate: data.pph_rate || 0,
        pph_amount: data.pph_amount || 0,
        ...(data.allocations !== undefined && {
          invoice_allocations: data.allocations,
        }),
        ...(data.bill_allocations !== undefined && {
          bill_allocations: data.bill_allocations,
        }),
      };

      const response = await api.put(`${this.getSSOTPaymentBase()}/${id}/draft`, ssotData);
      return response.data.data;
    } catch (error: any) {
      console.error('PaymentService - Error updating draft payment:', error);

      // Accounting COA completion returns a useful validation detail from the
      // backend. Preserve it so the form can explain what must be corrected.
      if (error.response?.data?.details) {
        throw new Error(error.response.data.details);
      }
      if (error.response?.data?.error) {
        throw new Error(error.response.data.error);
      }
      throw error;
    }
  }

  // Cancel payment
  async cancelPayment(id: number, reason: string): Promise<void> {
    try {
      await api.post(API_ENDPOINTS.PAYMENTS.CANCEL(id), { reason });
    } catch (error) {
      console.error('Error cancelling payment:', error);
      throw error;
    }
  }

  // Delete payment
  async deletePayment(id: number | string): Promise<void> {
    try {
      await api.delete(API_ENDPOINTS.PAYMENTS.DELETE(id));
    } catch (error: any) {
      console.error('Error deleting payment:', error);
      throw new Error(error.response?.data?.error || 'Failed to delete payment');
    }
  }

  // Export payments to Excel/CSV
  async exportPayments(filters?: PaymentFilters): Promise<Blob> {
    try {
      const params = new URLSearchParams();

      if (filters?.page) params.append('page', filters.page.toString());
      if (filters?.limit) params.append('limit', filters.limit.toString());
      if (filters?.contact_id) params.append('contact_id', filters.contact_id.toString());
      if (filters?.status) params.append('status', filters.status);
      if (filters?.method) params.append('method', filters.method);
      if (filters?.start_date) params.append('start_date', filters.start_date);
      if (filters?.end_date) params.append('end_date', filters.end_date);

      const response = await api.get(API_ENDPOINTS.PAYMENTS.EXPORT_EXCEL + `?${params}`, {
        responseType: 'blob',
      });

      return response.data;
    } catch (error: any) {
      console.error('Error exporting payments:', error);
      throw new Error(error.response?.data?.error || 'Failed to export payments');
    }
  }

  // Export payment report to PDF
  async exportPaymentReportPDF(startDate?: string, endDate?: string, status?: string, method?: string): Promise<Blob> {
    try {
      const params = new URLSearchParams();

      if (startDate) params.append('start_date', startDate);
      if (endDate) params.append('end_date', endDate);
      if (status) params.append('status', status);
      if (method) params.append('method', method);

      const response = await api.get(API_ENDPOINTS.PAYMENTS.REPORT.PDF + `?${params}`, {
        responseType: 'blob',
      });

      return response.data;
    } catch (error: any) {
      console.error('Error exporting payment report PDF:', error);
      throw new Error(error.response?.data?.error || 'Failed to export payment report PDF');
    }
  }

  // Export payment report to Excel
  async exportPaymentReportExcel(startDate?: string, endDate?: string, status?: string, method?: string): Promise<Blob> {
    try {
      const params = new URLSearchParams();

      if (startDate) params.append('start_date', startDate);
      if (endDate) params.append('end_date', endDate);
      if (status) params.append('status', status);
      if (method) params.append('method', method);

      const response = await api.get(API_ENDPOINTS.PAYMENTS.EXPORT_EXCEL + `?${params}`, {
        responseType: 'blob',
      });

      return response.data;
    } catch (error: any) {
      console.error('Error exporting payment report Excel:', error);
      throw new Error(error.response?.data?.error || 'Failed to export payment report Excel');
    }
  }

  // Export payment detail to PDF
  async exportPaymentDetailPDF(paymentId: number): Promise<Blob> {
    try {
      const response = await api.get(API_ENDPOINTS.PAYMENTS.PDF(paymentId), {
        responseType: 'blob',
      });

      return response.data;
    } catch (error: any) {
      console.error('Error exporting payment detail PDF:', error);
      throw new Error(error.response?.data?.error || 'Failed to export payment detail PDF');
    }
  }

  // Download payment report PDF
  async downloadPaymentReportPDF(startDate?: string, endDate?: string, status?: string, method?: string): Promise<void> {
    try {
      const blob = await this.exportPaymentReportPDF(startDate, endDate, status, method);
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;

      const filename = `Payment_Report_${startDate || 'all'}_to_${endDate || 'all'}.pdf`;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
    } catch (error: any) {
      console.error('Error downloading payment report PDF:', error);
      throw error;
    }
  }

  // Download payment detail PDF
  async downloadPaymentDetailPDF(paymentId: number, paymentCode: string): Promise<void> {
    try {
      const blob = await this.exportPaymentDetailPDF(paymentId);
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `Payment_${paymentCode}.pdf`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
    } catch (error: any) {
      console.error('Error downloading payment detail PDF:', error);
      throw error;
    }
  }

  // Download payment report Excel
  async downloadPaymentReportExcel(startDate?: string, endDate?: string, status?: string, method?: string): Promise<void> {
    try {
      const blob = await this.exportPaymentReportExcel(startDate, endDate, status, method);
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;

      const filename = `Payment_Report_${startDate || 'all'}_to_${endDate || 'all'}.xlsx`;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
    } catch (error: any) {
      console.error('Error downloading payment report Excel:', error);
      throw error;
    }
  }

  // Get unpaid invoices for customer
  async getUnpaidInvoices(customerId: number): Promise<any[]> {
    try {
      // Add cache busting parameter to ensure fresh data
      const timestamp = new Date().getTime();
      const response = await api.get(`${API_ENDPOINTS.PAYMENTS.UNPAID_INVOICES(customerId)}?_t=${timestamp}`);
      const d = response.data;
      if (Array.isArray(d)) return d;
      if (Array.isArray(d?.invoices)) return d.invoices;
      if (Array.isArray(d?.data)) return d.data;
      return [];
    } catch (error) {
      console.error('Error fetching unpaid invoices:', error);
      return [];
    }
  }

  // Get unpaid bills for vendor
  async getUnpaidBills(vendorId: number): Promise<any[]> {
    try {
      // Add cache busting parameter to ensure fresh data
      const timestamp = new Date().getTime();
      const response = await api.get(`${API_ENDPOINTS.PAYMENTS.UNPAID_BILLS(vendorId)}?_t=${timestamp}`);
      const d = response.data;
      if (Array.isArray(d)) return d;
      if (Array.isArray(d?.bills)) return d.bills;
      if (Array.isArray(d?.data)) return d.data;
      return [];
    } catch (error) {
      console.error('Error fetching unpaid bills:', error);
      return [];
    }
  }

  // Get payment summary
  async getPaymentSummary(startDate: string, endDate: string): Promise<PaymentSummary> {
    try {
      const response = await api.get(API_ENDPOINTS.PAYMENTS.SUMMARY + `?start_date=${startDate}&end_date=${endDate}`);
      return response.data;
    } catch (error) {
      console.error('Error fetching payment summary:', error);
      throw error;
    }
  }

  // Get payment analytics
  async getPaymentAnalytics(startDate: string, endDate: string): Promise<PaymentAnalytics> {
    try {
      const response = await api.get(API_ENDPOINTS.PAYMENTS.ANALYTICS + `?start_date=${startDate}&end_date=${endDate}`);
      return response.data;
    } catch (error) {
      console.error('Error fetching payment analytics:', error);
      throw error;
    }
  }

  // Generate payment report
  async generateReport(reportType: 'cash_flow' | 'aging' | 'method_analysis', params: any): Promise<any> {
    try {
      const response = await api.post(API_ENDPOINTS.PAYMENTS.GENERATE_REPORT(reportType), params);
      return response.data;
    } catch (error) {
      console.error('Error generating report:', error);
      throw error;
    }
  }

  // Bulk payment processing
  async processBulkPayments(payments: PaymentCreateRequest[]): Promise<any> {
    try {
      const response = await api.post(API_ENDPOINTS.PAYMENTS.BULK, { payments });
      return response.data;
    } catch (error) {
      console.error('Error processing bulk payments:', error);
      throw error;
    }
  }

  // Format currency for display
  formatCurrency(amount: number, currency: string = 'IDR'): string {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: currency,
      minimumFractionDigits: 0,
      maximumFractionDigits: 2,
    }).format(amount);
  }

  // Format date for display
  formatDate(dateString: string): string {
    return new Date(dateString).toLocaleDateString('id-ID', {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric'
    });
  }

  // Format date time for display
  formatDateTime(dateString: string): string {
    return new Date(dateString).toLocaleString('id-ID', {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  }

  // Get status color scheme for badges
  getStatusColorScheme(status: string): string {
    switch (status.toUpperCase()) {
      case 'COMPLETED':
        return 'green';
      case 'PENDING':
        return 'yellow';
      case 'PENDING_ACCOUNTING':
        return 'blue';
      case 'PENDING_FINAL_REVIEW':
        return 'purple';
      case 'DRAFT':
        return 'orange';
      case 'REJECTED':
      case 'FAILED':
      case 'CANCELLED':
      case 'REVERSED':
        return 'red';
      default:
        return 'gray';
    }
  }

  // Get method display name
  getMethodDisplayName(method: string): string {
    const methodMap: Record<string, string> = {
      'CASH': 'Tunai',
      'BANK_TRANSFER': 'Transfer Bank',
      'CHECK': 'Cek',
      'CREDIT_CARD': 'Kartu Kredit',
      'DEBIT_CARD': 'Kartu Debit',
      'OTHER': 'Lainnya'
    };

    return methodMap[method] || method;
  }

  // Format date for API (convert to RFC3339/ISO 8601 with timezone)
  formatDateForAPI(dateString: string): string {
    // If date is already in ISO format, return as is
    if (dateString.includes('T')) {
      return dateString;
    }

    let date: Date;

    // Try to parse DD/MM/YYYY or MM/DD/YYYY format first (from date picker)
    if (dateString.includes('/')) {
      const parts = dateString.split('/');
      if (parts.length === 3) {
        // Assume DD/MM/YYYY format (common in Indonesia)
        const day = parseInt(parts[0], 10);
        const month = parseInt(parts[1], 10) - 1; // Month is 0-indexed
        const year = parseInt(parts[2], 10);
        date = new Date(year, month, day, 0, 0, 0);
      } else {
        throw new Error('Invalid date format');
      }
    } else {
      // Convert YYYY-MM-DD to YYYY-MM-DDTHH:mm:ssZ (assume local timezone)
      date = new Date(dateString + 'T00:00:00');
    }

    // Check if date is valid
    if (isNaN(date.getTime())) {
      throw new Error('Invalid date format');
    }

    // Return in ISO format with local timezone
    return date.toISOString();
  }

  // Validate payment data
  validatePaymentData(data: PaymentCreateRequest): string[] {
    const errors: string[] = [];

    if (!data.contact_id) {
      errors.push('Contact is required');
    }

    if (!data.amount || data.amount <= 0) {
      errors.push('Amount must be greater than zero');
    }

    if (!data.date) {
      errors.push('Payment date is required');
    }

    if (!data.method) {
      errors.push('Payment method is required');
    }

    // Check if date is not in the future
    if (data.date && new Date(data.date) > new Date()) {
      errors.push('Payment date cannot be in the future');
    }

    return errors;
  }

  // Calculate total allocation amount
  calculateAllocationTotal(allocations: PaymentAllocation[]): number {
    return allocations.reduce((total, allocation) => total + allocation.amount, 0);
  }

  // ============ SSOT Journal Integration Methods ============

  // Create payment with automatic journal entry creation
  async createPaymentWithJournal(data: PaymentWithJournalRequest): Promise<PaymentWithJournalResponse> {
    try {
      const formattedData = {
        ...data,
        date: this.formatDateForAPI(data.date),
        auto_create_journal: data.auto_create_journal ?? true
      };

      const response = await api.post(API_ENDPOINTS.PAYMENTS_ENHANCED, formattedData, {
        timeout: 60000 // 60 seconds timeout for journal-heavy payment operations
      });
      return response.data.data;
    } catch (error: any) {
      console.error('PaymentService - Error creating payment with journal:', error);

      // Explicit handling for network timeouts so the UI can show a clearer message
      if (error.code === 'ECONNABORTED' || (typeof error.message === 'string' && error.message.toLowerCase().includes('timeout'))) {
        throw new Error('Proses pembayaran + jurnal terlalu lama (timeout). Sistem mungkin sedang sibuk, silakan cek kembali status pembayaran sebelum mengulang.');
      }

      if (error.response?.data?.error) {
        throw new Error(error.response.data.error);
      } else if (error.response?.data?.details) {
        throw new Error(error.response.data.details);
      }

      throw error;
    }
  }

  // Preview journal entry that would be created for a payment
  async previewPaymentJournal(data: PaymentWithJournalRequest): Promise<PaymentJournalResult> {
    try {
      const formattedData = {
        ...data,
        date: this.formatDateForAPI(data.date)
      };

      const response = await api.post(API_ENDPOINTS.PAYMENTS_PREVIEW_JOURNAL, formattedData);
      return response.data.data;
    } catch (error: any) {
      console.error('PaymentService - Error previewing payment journal:', error);
      throw new Error(error.response?.data?.error || 'Failed to preview payment journal');
    }
  }

  // Get payment with journal entry details
  async getPaymentWithJournal(paymentId: number): Promise<PaymentWithJournalResponse> {
    try {
      const response = await api.get(API_ENDPOINTS.PAYMENTS_WITH_JOURNAL(paymentId));
      return response.data.data;
    } catch (error: any) {
      console.error('PaymentService - Error getting payment with journal:', error);
      throw new Error(error.response?.data?.error || 'Failed to get payment with journal details');
    }
  }

  // Reverse payment and its journal entry
  async reversePayment(paymentId: number, reason: string): Promise<PaymentWithJournalResponse> {
    try {
      const response = await api.post(API_ENDPOINTS.PAYMENTS_REVERSE(paymentId), { reason });
      return response.data.data;
    } catch (error: any) {
      console.error('PaymentService - Error reversing payment:', error);
      throw new Error(error.response?.data?.error || 'Failed to reverse payment');
    }
  }

  // Get account balance updates from payment
  async getAccountBalanceUpdates(paymentId: number): Promise<AccountBalanceUpdate[]> {
    try {
      const response = await api.get(API_ENDPOINTS.PAYMENTS_ACCOUNT_UPDATES(paymentId));
      return response.data.data;
    } catch (error: any) {
      console.error('PaymentService - Error getting account updates:', error);
      throw new Error(error.response?.data?.error || 'Failed to get account balance updates');
    }
  }

  // Get real-time account balances from SSOT
  async getRealTimeAccountBalances(): Promise<SSOTAccountBalance[]> {
    try {
      const response = await api.get(API_ENDPOINTS.PAYMENTS_ACCOUNT_BALANCES);
      return response.data.data;
    } catch (error: any) {
      console.error('PaymentService - Error getting real-time balances:', error);
      throw new Error(error.response?.data?.error || 'Failed to get real-time account balances');
    }
  }

  // Refresh account balances materialized view
  async refreshAccountBalances(): Promise<void> {
    try {
      await api.post(API_ENDPOINTS.PAYMENTS_REFRESH_BALANCES);
    } catch (error: any) {
      console.error('PaymentService - Error refreshing account balances:', error);
      throw new Error(error.response?.data?.error || 'Failed to refresh account balances');
    }
  }

  // Get journal entries for a payment
  async getPaymentJournalEntries(paymentId: number): Promise<JournalEntry[]> {
    try {
      const response = await api.get(API_ENDPOINTS.PAYMENTS_JOURNAL_ENTRIES + `?payment_id=${paymentId}`);
      return response.data.data;
    } catch (error: any) {
      console.error('PaymentService - Error getting payment journal entries:', error);
      throw new Error(error.response?.data?.error || 'Failed to get payment journal entries');
    }
  }

  // Get payment-journal integration metrics
  async getIntegrationMetrics(): Promise<PaymentIntegrationMetrics> {
    try {
      const response = await api.get(API_ENDPOINTS.PAYMENTS_INTEGRATION_METRICS);
      return response.data.data;
    } catch (error: any) {
      console.error('PaymentService - Error getting integration metrics:', error);
      throw new Error(error.response?.data?.error || 'Failed to get integration metrics');
    }
  }

  // Create expense payment (for expense/liability accounts from COA)
  async createExpensePayment(data: ExpensePaymentRequest): Promise<Payment> {
    try {
      // Format the payment request for expense payment
      const formattedData = {
        contact_id: data.contact_id,
        expense_account_id: data.expense_account_id,
        cash_bank_id: data.cash_bank_id,
        date: this.formatDateForAPI(data.date),
        amount: data.amount,
        exchange_rate: data.exchange_rate,
        method: data.method,
        reference: data.reference || '',
        notes: data.notes || '',
        description: data.description || 'COA / Non-Invoice Payment',
        payment_type: 'EXPENSE',
        auto_create_journal: data.auto_create_journal ?? true,
        require_director_approval: !!data.require_director_approval,
        allow_liability_overpayment: !!data.allow_liability_overpayment,
        pph_type: data.pph_type || '',
        pph_rate: data.pph_rate || 0,
        pph_amount: data.pph_amount || 0,
      };

      // Check if SSOT expense payment endpoint exists, otherwise use regular endpoint
      const endpoint = (API_ENDPOINTS as any).PAYMENTS?.SSOT?.EXPENSE ||
        API_ENDPOINTS.PAYMENTS.CREATE;

      const response = await api.post(endpoint, formattedData, {
        timeout: 60000 // 60 seconds timeout for expense payment operations
      });

      // Handle response format variations
      return response.data?.data?.payment || response.data?.data || response.data?.payment || response.data;
    } catch (error: any) {
      console.error('PaymentService - Error creating expense payment:', error);

      // Explicit handling for network timeouts so the UI can show a clearer message
      if (error.code === 'ECONNABORTED' || (typeof error.message === 'string' && error.message.toLowerCase().includes('timeout'))) {
        throw new Error('Permintaan pembayaran beban terlalu lama (timeout). Cek koneksi / status server, lalu pastikan transaksi belum tercatat sebelum mencoba lagi.');
      }

      // Check for authentication errors
      if (error.isAuthError || error.code === 'AUTH_SESSION_EXPIRED' || error.message?.includes('Session expired')) {
        const authError = new Error('Session expired. Please login again.');
        (authError as any).isAuthError = true;
        (authError as any).code = 'AUTH_SESSION_EXPIRED';
        throw authError;
      } else if (error.response?.status === 401) {
        const authError = new Error('Session expired. Please login again.');
        (authError as any).isAuthError = true;
        (authError as any).code = 'AUTH_SESSION_EXPIRED';
        throw authError;
      } else if (error.response?.status === 403) {
        throw new Error('You do not have permission to create expense payments.');
      } else if (error.response?.data?.error) {
        throw new Error(error.response.data.error);
      } else if (error.response?.data?.details) {
        throw new Error(error.response.data.details);
      } else if (error.message) {
        throw new Error(error.message);
      }

      throw new Error('An unexpected error occurred while creating expense payment');
    }
  }

  // Approve a DRAFT expense payment (finance/admin only)
  async approveExpensePayment(paymentId: number, requireDirectorApproval: boolean = false): Promise<Payment> {
    try {
      const response = await api.post(`${this.getSSOTPaymentBase()}/${paymentId}/approve-expense`, {
        require_director_approval: requireDirectorApproval,
      });
      return response.data?.data || response.data;
    } catch (error: any) {
      throw new Error(error.response?.data?.details || error.response?.data?.error || 'Failed to approve expense payment');
    }
  }

  // Approve a DRAFT standard payment (receivable/payable) (finance/admin only)
  async approveStandardPayment(paymentId: number, requireDirectorApproval: boolean = false): Promise<Payment> {
    try {
      const response = await api.post(`${this.getSSOTPaymentBase()}/${paymentId}/approve-standard`, {
        require_director_approval: requireDirectorApproval,
      });
      return response.data?.data?.payment || response.data?.data || response.data;
    } catch (error: any) {
      throw new Error(error.response?.data?.details || error.response?.data?.error || 'Failed to approve payment');
    }
  }

  async rejectPayment(paymentId: number, reason: string): Promise<Payment> {
    try {
      const response = await api.post(API_ENDPOINTS.PAYMENTS.SSOT.REJECT(paymentId), {
        reason,
      });
      return response.data?.data || response.data;
    } catch (error: any) {
      throw new Error(error.response?.data?.details || error.response?.data?.error || 'Failed to reject payment');
    }
  }

  getActorDisplayName(actor?: PaymentActor | null): string {
    if (!actor) {
      return '-';
    }

    const fullName = [actor.first_name, actor.last_name].filter(Boolean).join(' ').trim();
    return fullName || actor.username || actor.email || `User #${actor.id}`;
  }

  getStatusLabel(status: string): string {
    switch (status.toUpperCase()) {
      case 'COMPLETED':
        return 'Approved';
      case 'PENDING':
        return 'Menunggu Director';
      case 'PENDING_ACCOUNTING':
        return 'Menunggu Accounting';
      case 'PENDING_FINAL_REVIEW':
        return 'Menunggu Review FM';
      case 'DRAFT':
        return 'Menunggu Finance Manager';
      case 'REJECTED':
        return 'Rejected';
      case 'FAILED':
        return 'Failed';
      case 'REVERSED':
        return 'Reversed';
      default:
        return status;
    }
  }

}

// ============ SSOT Journal Integration Types ============

export interface PaymentWithJournalRequest {
  contact_id: number;
  cash_bank_id: number;
  date: string;
  amount: number;
  exchange_rate?: number;
  method: string;
  reference?: string;
  notes?: string;
  auto_create_journal?: boolean;
  preview_journal?: boolean;
  target_invoice_id?: number;
  target_bill_id?: number;
}

export interface PaymentWithJournalResponse {
  payment: Payment;
  journal_result?: PaymentJournalResult;
  contact: {
    id: number;
    name: string;
    type: 'CUSTOMER' | 'VENDOR';
  };
  cash_bank?: {
    id: number;
    name: string;
    account_code: string;
  };
  allocations?: PaymentAllocation[];
  summary: PaymentProcessingSummary;
  success: boolean;
  message: string;
  warnings?: string[];
}

export interface PaymentJournalResult {
  journal_entry: JournalEntry;
  account_updates: AccountBalanceUpdate[];
  success: boolean;
  message: string;
}

export interface JournalEntry {
  id: number;
  entry_number: string;
  status: string;
  total_debit: number;
  total_credit: number;
  is_balanced: boolean;
  lines: JournalLine[];
  created_at: string;
  updated_at: string;
}

export interface JournalLine {
  id: number;
  line_number: number;
  account_id: number;
  description: string;
  debit_amount: number;
  credit_amount: number;
}

export interface AccountBalanceUpdate {
  account_id: number;
  account_code: string;
  account_name: string;
  old_balance: number;
  new_balance: number;
  change: number;
  change_type: 'INCREASE' | 'DECREASE';
}

export interface SSOTAccountBalance {
  account_id: number;
  account_code: string;
  account_name: string;
  account_type: string;
  account_category: string;
  normal_balance: string;
  total_debits: number;
  total_credits: number;
  transaction_count: number;
  current_balance: number;
  last_transaction_date?: string;
  last_updated: string;
  is_active: boolean;
  is_header: boolean;
}

export interface PaymentProcessingSummary {
  total_amount: number;
  processing_time: string;
  journal_entry_created: boolean;
  account_balances_updated: boolean;
  allocations_created: number;
  transaction_id: string;
}

export interface PaymentJournalMetrics {
  journal_coverage_rate: number;
  journal_success_rate: number;
  balance_accuracy_score: string;
  total_payments: number;
  payments_with_journal: number;
  payments_without_journal: number;
  last_refresh_time: string;
}

export interface PaymentIntegrationMetrics {
  total_payments: number;
  payments_with_journal: number;
  total_amount: number;
  success_rate: number;
}

// Updated types to match backend
export interface PaymentWithJournalInfo {
  payment: Payment;
  journal_info?: {
    journal_entry_id: number;
    journal_entry_number: string;
    status: string;
    total_debit?: number;
    total_credit?: number;
  };
}

export interface JournalPreviewRequest {
  payment_type: 'RECEIVE' | 'SEND';
  amount: number;
  currency: string;
  payment_method: string;
  description: string;
  customer_id?: string;
  vendor_id?: string;
  invoice_id?: string;
  journal_options: {
    auto_post: boolean;
    generate_reference: boolean;
    validate_balance: boolean;
    update_account_balances: boolean;
  };
}

export interface CreatePaymentWithJournalRequest {
  payment_type: 'RECEIVE' | 'SEND';
  amount: number;
  currency: string;
  payment_method: string;
  description: string;
  reference_id?: string;
  customer_id?: string;
  vendor_id?: string;
  invoice_id?: string;
  metadata?: Record<string, any>;
  journal_options: {
    auto_post: boolean;
    generate_reference: boolean;
    validate_balance: boolean;
    update_account_balances: boolean;
  };
}

export interface ExpensePaymentRequest {
  contact_id?: number;
  expense_account_id: number;
  cash_bank_id: number;
  date: string;
  amount: number;
  exchange_rate?: number;
  method: string;
  reference?: string;
  notes?: string;
  description?: string;
  auto_create_journal?: boolean;
  require_director_approval?: boolean;
  allow_liability_overpayment?: boolean;
  // PPh withholding tax (optional)
  pph_type?: string;   // PPH_21, PPH_23, PPH_4_2, PPH_22
  pph_rate?: number;   // e.g. 2 for 2%
  pph_amount?: number; // calculated tax amount
}

export default new PaymentService();
