import api from './api';
import { API_ENDPOINTS } from '../config/api';

export interface SSOTPurchaseReportData {
  company: CompanyInfo;
  start_date: string;
  end_date: string;
  currency: string;
  total_purchases: number;
  completed_purchases: number;
  total_amount: number;
  total_paid: number;
  outstanding_payables: number;
  purchases_by_vendor: VendorPurchaseSummary[];
  purchases_by_month: MonthlyPurchaseSummary[];
  purchases_by_category: CategoryPurchaseSummary[];
  payment_analysis: PurchasePaymentAnalysis;
  tax_analysis: PurchaseTaxAnalysis;
  generated_at: string;
}

export interface CompanyInfo {
  name: string;
  address: string;
  city: string;
  state: string;
  phone: string;
  email: string;
  tax_number: string;
}

export interface VendorPurchaseSummary {
  vendor_id: number;
  vendor_name: string;
  total_purchases: number;
  total_amount: number;
  total_paid: number;
  outstanding: number;
  last_purchase_date: string;
  payment_method: string;
  status: string;
  items?: PurchaseItemDetail[]; // Items purchased from this vendor
}

export interface PurchaseItemDetail {
  product_id: number;
  product_code: string;
  product_name: string;
  quantity: number;
  unit_price: number;
  total_price: number;
  unit: string;
  purchase_date: string;
  invoice_number?: string;
}

export interface MonthlyPurchaseSummary {
  year: number;
  month: number;
  month_name: string;
  total_purchases: number;
  total_amount: number;
  total_paid: number;
  average_amount: number;
}

export interface CategoryPurchaseSummary {
  category_name: string;
  account_code: string;
  account_name: string;
  total_purchases: number;
  total_amount: number;
  percentage: number;
}

export interface PurchasePaymentAnalysis {
  cash_purchases: number;
  credit_purchases: number;
  cash_amount: number;
  credit_amount: number;
  cash_percentage: number;
  credit_percentage: number;
  average_order_value: number;
}

export interface PurchaseTaxAnalysis {
  total_taxable_amount: number;
  total_tax_amount: number;
  average_tax_rate: number;
  tax_reclaimable_amount: number;
  tax_by_month: MonthlyTaxSummary[];
}

export interface MonthlyTaxSummary {
  year: number;
  month: number;
  month_name: string;
  tax_amount: number;
}

export interface SSOTPurchaseReportParams {
  start_date: string;
  end_date: string;
  format?: 'json' | 'pdf' | 'csv';
  project_id?: number;
}

class SSOTPurchaseReportService {

  async generateSSOTPurchaseReport(params: SSOTPurchaseReportParams): Promise<SSOTPurchaseReportData> {
    try {
      const queryParams = new URLSearchParams({
        start_date: params.start_date,
        end_date: params.end_date,
        format: params.format || 'json'
      });

      if (params.project_id) {
        queryParams.set('project_id', params.project_id.toString());
      }

      const response = await api.get(API_ENDPOINTS.SSOT_REPORTS.PURCHASE_REPORT + `?${queryParams}`);

      const payload = response?.data ?? {};
      const statusStr = typeof payload.status === 'string' ? payload.status.toLowerCase() : '';
      const isSuccess = statusStr === 'success' || payload.success === true;

      // Accept common success shapes
      if (isSuccess) {
        return (payload.data ?? payload) as SSOTPurchaseReportData;
      }

      // If data exists (even without explicit success flag), treat as success
      if (payload.data && typeof payload.data === 'object') {
        return payload.data as SSOTPurchaseReportData;
      }

      // Heuristic: HTTP 200 with a positive message and no error field
      if (response.status === 200 && !payload.error) {
        const msg: string | undefined = payload.message;
        if (msg && /generated successfully/i.test(msg)) {
          return (payload.data ?? payload) as SSOTPurchaseReportData;
        }
      }

      throw new Error(payload.message || payload.error?.message || 'Failed to generate purchase report');
    } catch (error: any) {
      console.error('Error generating SSOT purchase report:', error);
      if (error.response?.data?.message) {
        throw new Error(error.response.data.message);
      }
      throw error;
    }
  }

  // Deprecated: Use generateSSOTPurchaseReport instead
  async generateSSOTVendorAnalysis(params: any): Promise<any> {
    console.warn('generateSSOTVendorAnalysis is deprecated. Use generateSSOTPurchaseReport instead.');
    
    try {
      // Redirect to new purchase report endpoint
      return await this.generateSSOTPurchaseReport({
        start_date: params.start_date,
        end_date: params.end_date,
        format: params.format
      });
    } catch (error) {
      console.error('Deprecated vendor analysis redirected to purchase report failed:', error);
      throw error;
    }
  }

  formatCurrency(value: number): string {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR'
    }).format(value);
  }

  formatPercentage(value: number): string {
    return new Intl.NumberFormat('id-ID', {
      style: 'percent',
      minimumFractionDigits: 1,
      maximumFractionDigits: 1
    }).format(value / 100);
  }

  calculateGrowthRate(current: number, previous: number): number {
    if (previous === 0) return 0;
    return ((current - previous) / previous) * 100;
  }

  getPaymentMethodColor(method: string): string {
    switch (method.toUpperCase()) {
      case 'CASH':
        return 'green';
      case 'CREDIT':
        return 'orange';
      default:
        return 'gray';
    }
  }

  getStatusColor(status: string): string {
    switch (status.toUpperCase()) {
      case 'COMPLETED':
        return 'green';
      case 'PENDING':
        return 'orange';
      case 'APPROVED':
        return 'blue';
      default:
        return 'gray';
    }
  }

  getCategoryColor(category: string): string {
    switch (category.toLowerCase()) {
      case 'inventory':
        return 'blue';
      case 'fixed assets':
        return 'purple';
      case 'expenses':
        return 'red';
      default:
        return 'gray';
    }
  }

  // Helper method to get current period data
  getCurrentPeriodSummary(data: SSOTPurchaseReportData) {
    const now = new Date();
    const currentYear = now.getFullYear();
    const currentMonth = now.getMonth() + 1;

    const currentMonthData = data.purchases_by_month.find(
      m => m.year === currentYear && m.month === currentMonth
    );

    const previousMonthData = data.purchases_by_month.find(
      m => m.year === currentYear && m.month === currentMonth - 1
    ) || data.purchases_by_month.find(
      m => m.year === currentYear - 1 && m.month === 12
    );

    return {
      current: currentMonthData || {
        year: currentYear,
        month: currentMonth,
        month_name: new Date(currentYear, currentMonth - 1).toLocaleString('default', { month: 'long' }),
        total_purchases: 0,
        total_amount: 0,
        total_paid: 0,
        average_amount: 0
      },
      previous: previousMonthData,
      growth: previousMonthData ? this.calculateGrowthRate(
        currentMonthData?.total_amount || 0,
        previousMonthData.total_amount
      ) : 0
    };
  }

  // Helper method to get top vendors by amount
  getTopVendors(data: SSOTPurchaseReportData, limit: number = 5): VendorPurchaseSummary[] {
    return data.purchases_by_vendor
      .sort((a, b) => b.total_amount - a.total_amount)
      .slice(0, limit);
  }

  // Helper method to get payment efficiency
  getPaymentEfficiency(data: SSOTPurchaseReportData): {
    total_purchases: number;
    paid_purchases: number;
    efficiency_percentage: number;
    outstanding_count: number;
  } {
    const totalPurchases = data.total_purchases;
    const totalVendors = data.purchases_by_vendor.length;
    const paidVendors = data.purchases_by_vendor.filter(v => v.outstanding <= 0).length;
    const outstandingVendors = data.purchases_by_vendor.filter(v => v.outstanding > 0).length;

    // Calculate efficiency based on amount paid vs total amount (not vendor count vs purchase count)
    const totalAmount = data.total_amount || 0;
    const totalPaid = data.total_paid || 0;
    const efficiencyPercentage = totalAmount > 0 ? (totalPaid / totalAmount) * 100 : 0;

    return {
      total_purchases: totalPurchases,
      paid_purchases: totalVendors - outstandingVendors,
      efficiency_percentage: efficiencyPercentage,
      outstanding_count: outstandingVendors
    };
  }

  // Export helper methods
  async exportToPDF(params: SSOTPurchaseReportParams): Promise<Blob> {
    const pdfParams = { ...params, format: 'pdf' as const };
    
    try {
      const queryParams = new URLSearchParams({
        start_date: pdfParams.start_date,
        end_date: pdfParams.end_date,
        format: pdfParams.format
      });

      if (pdfParams.project_id) {
        queryParams.set('project_id', pdfParams.project_id.toString());
      }

      const response = await api.get(
        API_ENDPOINTS.SSOT_REPORTS.PURCHASE_REPORT + `?${queryParams}`,
        {
          responseType: 'blob'
        }
      );

      return response.data;
    } catch (error: any) {
      console.error('Error exporting purchase report to PDF:', error);
      if (error.response?.status === 501) {
        throw new Error('PDF export is not yet implemented');
      }
      throw new Error('Failed to export purchase report to PDF');
    }
  }

  async exportToCSV(params: SSOTPurchaseReportParams): Promise<Blob> {
    const csvParams = { ...params, format: 'csv' as const };
    
    try {
      const queryParams = new URLSearchParams({
        start_date: csvParams.start_date,
        end_date: csvParams.end_date,
        format: csvParams.format
      });

      if (csvParams.project_id) {
        queryParams.set('project_id', csvParams.project_id.toString());
      }

      const response = await api.get(
        API_ENDPOINTS.SSOT_REPORTS.PURCHASE_REPORT + `?${queryParams}`,
        {
          responseType: 'blob'
        }
      );

      return response.data;
    } catch (error: any) {
      console.error('Error exporting purchase report to CSV:', error);
      if (error.response?.status === 501) {
        throw new Error('CSV export is not yet implemented');
      }
      throw new Error('Failed to export purchase report to CSV');
    }
  }

  // Validation helpers
  validateDateRange(startDate: string, endDate: string): { valid: boolean; error?: string } {
    const start = new Date(startDate);
    const end = new Date(endDate);
    const now = new Date();

    if (isNaN(start.getTime())) {
      return { valid: false, error: 'Invalid start date format' };
    }

    if (isNaN(end.getTime())) {
      return { valid: false, error: 'Invalid end date format' };
    }

    if (start > end) {
      return { valid: false, error: 'Start date must be before end date' };
    }

    if (start > now) {
      return { valid: false, error: 'Start date cannot be in the future' };
    }

    // Check if date range is too large (more than 2 years)
    const diffTime = Math.abs(end.getTime() - start.getTime());
    const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));
    
    if (diffDays > 730) {
      return { valid: false, error: 'Date range cannot exceed 2 years' };
    }

    return { valid: true };
  }
}

export const ssotPurchaseReportService = new SSOTPurchaseReportService();
export default ssotPurchaseReportService;
