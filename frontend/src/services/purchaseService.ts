import api from './api';
import { API_ENDPOINTS } from '@/config/api';

export interface PurchaseItemRequest {
  product_id?: number;
  quantity: number;
  unit_price: number;
  discount?: number;
  tax?: number;
  expense_account_id?: number;
  description?: string;
  item_type?: string;        // "INVENTORY" | "ASSET"
  is_fixed_asset?: boolean;
  asset_category_id?: number;
}

export interface PurchaseCreateRequest {
  vendor_id: number;
  purchase_request_id?: number;
  project_id?: number;
  date: string; // ISO string
  due_date?: string; // ISO string
  discount?: number;
  tax?: number;
  notes?: string;
  items: PurchaseItemRequest[]; // backend expects `items`
  request_priority?: 'LOW' | 'NORMAL' | 'HIGH' | 'URGENT';
}

export interface PurchaseFilterParams {
  status?: string;
  vendor_id?: string;
  project_id?: string;
  start_date?: string;
  end_date?: string;
  search?: string;
  approval_status?: string;
  requires_approval?: boolean;
  page?: number;
  limit?: number;
}

export interface PurchaseItem {
  id: number;
  product_id: number;
  quantity: number;
  unit_price: number;
  discount: number;
  tax: number;
  total_price: number;
  product?: {
    id: number;
    name: string;
    code: string;
  };
}

export interface Vendor {
  id: number;
  name: string;
  code: string;
  email?: string;
  phone?: string;
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
  approval_amount_basis?: 'SUBTOTAL_BEFORE_DISCOUNT' | 'NET_AFTER_DISCOUNT_BEFORE_TAX' | 'GRAND_TOTAL_AFTER_TAX';
  approval_base_amount?: number;
  requires_approval: boolean;
  approval_request_id?: number;
  approved_at?: string;
  approved_by?: number;
  vendor?: Vendor;
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
  payment_date: string; // ISO string
  payment_method: string;
  cash_bank_id?: number;
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
        requires_approval: params.requires_approval,
        page: params.page ?? 1,
        limit: params.limit ?? 10,
      }
    });
    return response.data;
  }

  async create(payload: PurchaseCreateRequest): Promise<Purchase> {
    const response = await api.post(API_ENDPOINTS.PURCHASES, payload);
    return response.data;
  }

  async submitForApproval(id: number): Promise<{ message: string }> {
    const response = await api.post(API_ENDPOINTS.PURCHASES_SUBMIT_APPROVAL(id));
    return response.data;
  }

  async getById(id: number): Promise<Purchase> {
    const response = await api.get(API_ENDPOINTS.PURCHASES_BY_ID(id));
    return response.data;
  }

  async update(id: number, payload: Partial<PurchaseCreateRequest>): Promise<Purchase> {
    const response = await api.put(API_ENDPOINTS.PURCHASES_BY_ID(id), payload);
    return response.data;
  }

  async delete(id: number): Promise<{ message: string }> {
    const response = await api.delete(API_ENDPOINTS.PURCHASES_BY_ID(id));
    return response.data;
  }

  async getPendingApproval(page = 1, limit = 10): Promise<PurchaseListResponse> {
    return this.list({
      approval_status: 'PENDING',
      page,
      limit,
    });
  }

  async getSummary(startDate?: string, endDate?: string): Promise<PurchaseSummary> {
    const response = await api.get(API_ENDPOINTS.PURCHASES_SUMMARY, {
      params: {
        start_date: startDate,
        end_date: endDate,
      },
    });
    return response.data;
  }

  // Purchase Payment Integration Methods

  async getPurchaseForPayment(id: number): Promise<PurchaseForPayment> {
    const response = await api.get(API_ENDPOINTS.PURCHASES_FOR_PAYMENT(id));
    return response.data;
  }

  async createIntegratedPayment(purchaseId: number, data: PurchasePaymentRequest): Promise<any> {
    // Map frontend field names to backend expected field names
    const backendData = {
      amount: data.amount,                    // ✅ Correct
      date: data.payment_date,               // ✅ Fixed: backend expects "date" not "payment_date"
      method: data.payment_method,           // ✅ Fixed: backend expects "method" not "payment_method"
      cash_bank_id: data.cash_bank_id,      // ✅ Correct
      reference: data.reference || '',       // ✅ Correct
      notes: data.notes || ''               // ✅ Correct
    };
    const response = await api.post(API_ENDPOINTS.PURCHASES_INTEGRATED_PAYMENT(purchaseId), backendData);
    return response.data;
  }

  // NEW: Direct Payment Management Integration
  async createPurchasePayment(purchaseId: number, data: PurchasePaymentRequest): Promise<any> {
    // This method uses the new /purchases/:id/payments endpoint that integrates with Payment Management

    // Ensure date is properly formatted
    let formattedDate = data.payment_date;
    if (formattedDate && !formattedDate.includes('T')) {
      // If date is just YYYY-MM-DD, add time component for consistency
      formattedDate = `${formattedDate}T00:00:00.000Z`;
    }

    const backendData = {
      amount: data.amount,
      date: formattedDate,  // Send as properly formatted date string
      method: data.payment_method,
      cash_bank_id: data.cash_bank_id,
      reference: data.reference || '',
      notes: data.notes || ''
    };

    console.log('Sending payment data to backend:', backendData);
    // Use extended timeout for payment operations (30 seconds)
    const response = await api.post(API_ENDPOINTS.PURCHASES_PAYMENTS(purchaseId), backendData, {
      timeout: 30000 // 30 seconds timeout for payment operations
    });
    return response.data;
  }

  async getPurchasePayments(purchaseId: number): Promise<PurchasePayment[]> {
    const response = await api.get(API_ENDPOINTS.PURCHASES_PAYMENTS(purchaseId));
    return response.data;
  }

  // Helper method to check if purchase can receive payment
  canReceivePayment(purchase: Purchase): boolean {
    const status = (purchase.status || '').toUpperCase();
    const eligibleStatus = status === 'APPROVED' || status === 'COMPLETED' || status === 'PAID';
    return eligibleStatus &&
      purchase.payment_method === 'CREDIT' &&
      (purchase.outstanding_amount || 0) > 0;
  }

  // Helper method to format currency for payment display
  formatPaymentAmount(amount: number): string {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0
    }).format(amount);
  }

  // Export functionality

  async exportPurchasesPDF(filters?: PurchaseFilterParams): Promise<Blob> {
    try {
      const params = new URLSearchParams();

      if (filters?.status) params.append('status', filters.status);
      if (filters?.vendor_id) params.append('vendor_id', filters.vendor_id);
      if (filters?.project_id) params.append('project_id', filters.project_id);
      if (filters?.start_date) params.append('start_date', filters.start_date);
      if (filters?.end_date) params.append('end_date', filters.end_date);
      if (filters?.search) params.append('search', filters.search);
      if (filters?.approval_status) params.append('approval_status', filters.approval_status);
      if (filters?.requires_approval !== undefined) params.append('requires_approval', filters.requires_approval.toString());

      const response = await api.get(`${API_ENDPOINTS.PURCHASES_EXPORT_PDF}?${params}`, {
        responseType: 'blob',
        headers: { Accept: 'application/pdf' }
      });

      return response.data;
    } catch (error: any) {
      console.error('Error exporting purchases PDF:', error);
      throw new Error(error.response?.data?.error || 'Failed to export purchases PDF');
    }
  }

  async exportPurchasesCSV(filters?: PurchaseFilterParams): Promise<Blob> {
    try {
      const params = new URLSearchParams();

      if (filters?.status) params.append('status', filters.status);
      if (filters?.vendor_id) params.append('vendor_id', filters.vendor_id);
      if (filters?.project_id) params.append('project_id', filters.project_id);
      if (filters?.start_date) params.append('start_date', filters.start_date);
      if (filters?.end_date) params.append('end_date', filters.end_date);
      if (filters?.search) params.append('search', filters.search);
      if (filters?.approval_status) params.append('approval_status', filters.approval_status);
      if (filters?.requires_approval !== undefined) params.append('requires_approval', filters.requires_approval.toString());

      const response = await api.get(`${API_ENDPOINTS.PURCHASES_EXPORT_CSV}?${params}`, {
        responseType: 'blob',
        headers: { Accept: 'text/csv' }
      });

      return response.data;
    } catch (error: any) {
      console.error('Error exporting purchases CSV:', error);
      throw new Error(error.response?.data?.error || 'Failed to export purchases CSV');
    }
  }

  // Download helper methods

  async downloadPurchasesPDF(filters?: PurchaseFilterParams): Promise<void> {
    try {
      const blob = await this.exportPurchasesPDF(filters);
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;

      // Create filename based on filters
      let filename = 'purchases-report';
      if (filters?.start_date && filters?.end_date) {
        filename += `_${filters.start_date}_to_${filters.end_date}`;
      } else if (filters?.start_date) {
        filename += `_from_${filters.start_date}`;
      } else if (filters?.end_date) {
        filename += `_until_${filters.end_date}`;
      }
      filename += '.pdf';

      link.download = filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
    } catch (error: any) {
      console.error('Error downloading purchases PDF:', error);
      throw error;
    }
  }

  async downloadPurchasesCSV(filters?: PurchaseFilterParams): Promise<void> {
    try {
      const blob = await this.exportPurchasesCSV(filters);
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;

      // Create filename based on filters
      let filename = 'purchases-report';
      if (filters?.start_date && filters?.end_date) {
        filename += `_${filters.start_date}_to_${filters.end_date}`;
      } else if (filters?.start_date) {
        filename += `_from_${filters.start_date}`;
      } else if (filters?.end_date) {
        filename += `_until_${filters.end_date}`;
      }
      filename += '.csv';

      link.download = filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
    } catch (error: any) {
      console.error('Error downloading purchases CSV:', error);
      throw error;
    }
  }
}

const purchaseService = new PurchaseService();
export default purchaseService;
