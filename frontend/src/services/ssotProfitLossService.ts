import API_BASE_URL, { API_ENDPOINTS, API_V1_BASE } from '@/config/api';
import { getAuthHeaders } from '@/utils/authTokenUtils';

// Types for SSOT Profit Loss Report
export interface SSOTProfitLossData {
  company?: {
    name: string;
    address?: string;
    city?: string;
    phone?: string;
    email?: string;
  };
  title?: string;
  period?: string;
  currency?: string;
  generated_at?: string;
  data_source?: string;
  data_source_label?: string;
  message?: string;
  hasData: boolean;
  enhanced?: boolean;

  // Summary fields for display
  total_revenue: number;
  total_expenses: number;
  net_profit: number;
  net_loss: number;

  // Detailed breakdown
  sections?: Array<{
    name: string;
    total: number;
    items?: Array<{
      account_code?: string;
      name: string;
      amount: number;
      is_percentage?: boolean;
    }>;
    subsections?: Array<{
      name: string;
      total: number;
      items?: Array<{
        account_code?: string;
        name: string;
        amount: number;
        is_percentage?: boolean;
      }>;
    }>;
  }>;

  // Financial metrics
  financialMetrics?: {
    grossProfit: number;
    grossProfitMargin: number;
    operatingIncome: number;
    operatingMargin: number;
    netIncome: number;
    netIncomeMargin: number;
  };

  // Revenue and expense details
  revenue_details?: Array<{
    account_id: number;
    account_name: string;
    amount: number;
  }>;

  expense_details?: Array<{
    account_id: number;
    account_name: string;
    amount: number;
  }>;

  other_income_items?: Array<{
    account_code?: string;
    account_name: string;
    amount: number;
  }>;

  other_expense_items?: Array<{
    account_code?: string;
    account_name: string;
    amount: number;
  }>;
}

export interface SSOTProfitLossParams {
  start_date: string;
  end_date: string;
  format?: 'json' | 'pdf' | 'csv';
}

class SSOTProfitLossService {
  private getAuthHeaders() {
    return getAuthHeaders();
  }

  private buildQueryString(params: Record<string, any>): string {
    const searchParams = new URLSearchParams();

    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== null && value !== '') {
        searchParams.append(key, String(value));
      }
    });

    return searchParams.toString();
  }

  /**
   * Generate SSOT Profit & Loss Statement
   * @param params Profit Loss parameters
   * @returns Profit Loss data or Blob for file downloads
   */
  async generateSSOTProfitLoss(params: SSOTProfitLossParams): Promise<SSOTProfitLossData | Blob> {
    if (!params.start_date || !params.end_date) {
      throw new Error('Start date and end date are required for profit & loss statement');
    }

    const queryString = this.buildQueryString(params);
    // Remove any trailing slash from API_BASE_URL to prevent double slashes
    const baseUrl = API_BASE_URL.endsWith('/') ? API_BASE_URL.slice(0, -1) : API_BASE_URL;
    const url = `${baseUrl}${API_V1_BASE}/reports/ssot-profit-loss${queryString ? '?' + queryString : ''}`;

    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      try {
        const error = await response.json();
        throw new Error(error.message || `Failed to generate SSOT Profit Loss: ${response.statusText}`);
      } catch (e) {
        throw new Error(`Failed to generate SSOT Profit Loss. Server returned: ${response.status} ${response.statusText}`);
      }
    }

    // Check if response is a file (PDF/CSV) based on content-type or format param
    const contentType = response.headers.get('content-type') || '';
    const isFileDownload = params.format === 'pdf' || params.format === 'csv' ||
      contentType.includes('application/pdf') ||
      contentType.includes('text/csv') ||
      contentType.includes('application/octet-stream');

    if (isFileDownload) {
      // Return as Blob for file downloads
      return await response.blob();
    }

    // Parse as JSON for regular data
    const result = await response.json();

    // Handle different response formats
    if (result.data) {
      return result.data;
    }

    if (result.status === 'success' && result.data) {
      return result.data;
    }

    return result;
  }

  /**
   * Generate SSOT Profit & Loss as PDF
   * @param params Profit Loss parameters
   * @returns PDF Blob
   */
  async generateSSOTProfitLossPDF(params: SSOTProfitLossParams): Promise<Blob> {
    const result = await this.generateSSOTProfitLoss({
      ...params,
      format: 'pdf'
    });

    if (result instanceof Blob) {
      return result;
    }

    throw new Error('Expected PDF blob but received other data type');
  }

  /**
   * Generate SSOT Profit & Loss as CSV
   * @param params Profit Loss parameters
   * @returns CSV Blob
   */
  async generateSSOTProfitLossCSV(params: SSOTProfitLossParams): Promise<Blob> {
    const result = await this.generateSSOTProfitLoss({
      ...params,
      format: 'csv'
    });

    if (result instanceof Blob) {
      return result;
    }

    throw new Error('Expected CSV blob but received other data type');
  }

  formatCurrency(value: number): string {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR'
    }).format(value);
  }

  getAccountTypeColor(accountType: string): string {
    const colors: { [key: string]: string } = {
      'REVENUE': 'green',
      'EXPENSE': 'red',
      'ASSET': 'blue',
      'LIABILITY': 'orange',
      'EQUITY': 'purple'
    };
    return colors[accountType.toUpperCase()] || 'gray';
  }

  calculateMargin(numerator: number, denominator: number): number {
    if (denominator === 0) return 0;
    return Math.round((numerator / denominator) * 100 * 100) / 100; // Round to 2 decimal places
  }
}

export const ssotProfitLossService = new SSOTProfitLossService();
