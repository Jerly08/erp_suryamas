import api from './api';
import { API_ENDPOINTS } from '../config/api';

export interface SSOTSalesSummaryData {
  company?: CompanyInfo;
  start_date: string;
  end_date: string;
  currency: string;
  total_revenue: number;
  total_sales: number;
  total_transactions?: number;
  total_tax?: number;
  total_discount?: number;
  total_discounts: number;
  total_returns: number;
  net_revenue: number;
  sales_by_customer: CustomerSalesData[];
  sales_by_product: ProductSalesData[];
  sales_by_period: PeriodSalesData[];
  items?: SalesSummaryItem[];
  generated_at: string;
}

export interface SalesSummaryItem {
  date: string;
  code: string;
  customer_name: string;
  total_amount: number;
  status: string;
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

export interface CustomerSalesData {
  customer_id: number;
  customer_name: string;
  total_sales: number;
  transaction_count: number;
  average_transaction: number;
  percentage_of_total: number;
  items?: SaleItemDetail[];
}

export interface SaleItemDetail {
  product_id: number;
  product_code: string;
  product_name: string;
  quantity: number;
  unit_price: number;
  total_price: number;
  unit: string;
  sale_date: string;
  invoice_number?: string;
}

export interface ProductSalesData {
  product_id: number;
  product_name: string;
  quantity_sold: number;
  total_revenue: number;
  average_price: number;
  percentage_of_total: number;
}

export interface PeriodSalesData {
  period: string;
  start_date: string;
  end_date: string;
  total_sales: number;
  transaction_count: number;
  growth_rate: number;
}

export interface SSOTSalesSummaryParams {
  start_date: string;
  end_date: string;
  format?: 'json' | 'pdf' | 'excel';
}

class SSOTSalesSummaryService {

  async generateSSOTSalesSummary(params: SSOTSalesSummaryParams): Promise<SSOTSalesSummaryData> {
    try {
      const queryParams = new URLSearchParams({
        start_date: params.start_date,
        end_date: params.end_date,
        format: params.format || 'json'
      });

      const response = await api.get(API_ENDPOINTS.SSOT_REPORTS.SALES_SUMMARY + `?${queryParams}`);

      if (response.data.status === 'success') {
        return response.data.data;
      }

      throw new Error(response.data.message || 'Failed to generate sales summary');
    } catch (error: any) {
      console.error('Error generating SSOT sales summary:', error);
      if (error.response?.data?.message) {
        throw new Error(error.response.data.message);
      }
      throw error;
    }
  }

  async exportSalesSummary(data: SSOTSalesSummaryData, format: 'pdf' | 'excel'): Promise<Blob> {
    try {
      const response = await api.post(
        API_ENDPOINTS.SSOT_REPORTS.SALES_SUMMARY_EXPORT,
        {
          data,
          format
        },
        {
          responseType: 'blob'
        }
      );

      return response.data;
    } catch (error: any) {
      console.error('Error exporting sales summary:', error);
      throw error;
    }
  }

  formatCurrency(value: number): string {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR'
    }).format(value);
  }

  calculateGrowthRate(current: number, previous: number): number {
    if (previous === 0) return 0;
    return ((current - previous) / previous) * 100;
  }
}

export const ssotSalesSummaryService = new SSOTSalesSummaryService();