import { API_ENDPOINTS } from '../config/api';
import { getAuthHeaders } from '@/utils/authTokenUtils';

export interface SSOTCashFlowData {
  company: {
    name: string;
  };
  start_date: string;
  end_date: string;
  currency: string;
  
  // Operating Activities
  operating_activities: {
    net_income: number;
    adjustments: {
      depreciation: number;
      amortization: number;
      bad_debt_expense: number;
      gain_loss_on_asset_disposal: number;
      other_non_cash_items: number;
      total_adjustments: number;
      items: Array<{
        account_code: string;
        account_name: string;
        amount: number;
        type: string;
      }>;
    };
    working_capital_changes: {
      accounts_receivable_change: number;
      inventory_change: number;
      prepaid_expenses_change: number;
      accounts_payable_change: number;
      accrued_liabilities_change: number;
      other_working_capital_change: number;
      total_working_capital_changes: number;
      items: Array<{
        account_code: string;
        account_name: string;
        amount: number;
        type: string;
      }>;
    };
    total_operating_cash_flow: number;
  };
  
  // Investing Activities
  investing_activities: {
    purchase_of_fixed_assets: number;
    sale_of_fixed_assets: number;
    purchase_of_investments: number;
    sale_of_investments: number;
    intangible_asset_purchases: number;
    other_investing_activities: number;
    total_investing_cash_flow: number;
    items: Array<{
      account_code: string;
      account_name: string;
      amount: number;
      type: string;
    }>;
  };
  
  // Financing Activities
  financing_activities: {
    share_capital_increase: number;
    share_capital_decrease: number;
    long_term_debt_increase: number;
    long_term_debt_decrease: number;
    short_term_debt_increase: number;
    short_term_debt_decrease: number;
    dividends_paid: number;
    other_financing_activities: number;
    total_financing_cash_flow: number;
    items: Array<{
      account_code: string;
      account_name: string;
      amount: number;
      type: string;
    }>;
  };
  
  // Summary
  net_cash_flow: number;
  cash_at_beginning: number;
  cash_at_end: number;
  
  // Ratios
  cash_flow_ratios: {
    operating_cash_flow_ratio: number;
    cash_flow_to_debt_ratio: number;
    free_cash_flow: number;
    cash_flow_per_share?: number;
  };
  
  generated_at: string;
  enhanced: boolean;
  has_reconciliation_warning?: boolean;
  reconciliation_difference?: number;
  account_details: Array<{
    account_id: number;
    account_code: string;
    account_name: string;
    account_type: string;
    debit_total: number;
    credit_total: number;
    net_balance: number;
  }>;
  data_source: string;
  message?: string;
}

export interface SSOTCashFlowParams {
  start_date: string;
  end_date: string;
  format?: 'json' | 'pdf' | 'excel' | 'csv';
}

class SSOTCashFlowReportService {
  private getAuthHeaders() {
    return getAuthHeaders();
  }

  private buildQueryString(params: SSOTCashFlowParams): string {
    const searchParams = new URLSearchParams();
    
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== null && value !== '') {
        searchParams.append(key, value.toString());
      }
    });

    return searchParams.toString();
  }

  private async handleResponse<T>(response: Response): Promise<T> {
    if (!response.ok) {
      let errorMessage = `HTTP ${response.status}: ${response.statusText}`;
      
      try {
        const errorData = await response.json();
        if (errorData.message) {
          errorMessage = errorData.message;
        } else if (errorData.error) {
          errorMessage = errorData.error;
        }
      } catch {
        // Use default error message if JSON parsing fails
      }
      
      throw new Error(errorMessage);
    }
    
    const contentType = response.headers.get('content-type') || '';
    
    // Handle PDF/Excel/CSV downloads
    if (contentType.includes('application/pdf') || 
        contentType.includes('application/vnd.openxmlformats') ||
        contentType.includes('text/csv')) {
      const blob = await response.blob();
      if (blob.size === 0) {
        throw new Error('Received empty file from server');
      }
      return blob as any;
    }
    
    // Handle JSON response
    const result = await response.json();
    
    if (result.status === 'error') {
      throw new Error(result.message || 'Request failed');
    }
    
    let data = result.data || result;
    
    if (result.status === 'success' && result.data) {
      data = result.data;
    }
    
    // Normalize shape and ensure numeric fields are parsed
    if (data && typeof data === 'object') {
      data = this.normalizeCashFlowResponse(data);
    }
    
    return data;
  }

  /**
   * Generate SSOT Cash Flow Statement
   * @param params Cash Flow parameters
   * @returns Cash Flow data or Blob for file downloads
   */
  async generateSSOTCashFlow(params: SSOTCashFlowParams): Promise<SSOTCashFlowData | Blob> {
    if (!params.start_date || !params.end_date) {
      throw new Error('Start date and end date are required for cash flow statement');
    }

    const queryString = this.buildQueryString(params);
    const url = API_ENDPOINTS.SSOT_REPORTS.CASH_FLOW + (queryString ? '?' + queryString : '');
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    return this.handleResponse(response);
  }

  /**
   * Get SSOT Cash Flow Summary
   * @param params Cash Flow parameters
   * @returns Simplified cash flow summary
   */
  async getSSOTCashFlowSummary(params: SSOTCashFlowParams): Promise<any> {
    if (!params.start_date || !params.end_date) {
      throw new Error('Start date and end date are required for cash flow summary');
    }

    const queryString = this.buildQueryString(params);
    const url = API_ENDPOINTS.SSOT_REPORTS.CASH_FLOW_SUMMARY + (queryString ? '?' + queryString : '');
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    return this.handleResponse(response);
  }

  /**
   * Validate SSOT Cash Flow Statement
   * @param params Cash Flow parameters
   * @returns Validation results
   */
  async validateSSOTCashFlow(params: SSOTCashFlowParams): Promise<any> {
    if (!params.start_date || !params.end_date) {
      throw new Error('Start date and end date are required for cash flow validation');
    }

    const queryString = this.buildQueryString(params);
    const url = API_ENDPOINTS.SSOT_REPORTS.CASH_FLOW_VALIDATE + (queryString ? '?' + queryString : '');
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    return this.handleResponse(response);
  }

  /**
   * Download cash flow report as file
   * @param reportData Blob data from API
   * @param fileName File name for download
   */
  async downloadCashFlowReport(reportData: Blob, fileName: string): Promise<void> {
    try {
      if (!(reportData instanceof Blob)) {
        throw new Error('Invalid file data received from server');
      }

      if (reportData.size === 0) {
        throw new Error('Empty file received from server');
      }

      const url = window.URL.createObjectURL(reportData);
      const link = document.createElement('a');
      link.href = url;
      link.download = fileName;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
    } catch (error) {
      console.error('Download error:', error);
      throw new Error(`Failed to download cash flow report: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  /**
   * Format currency for display
   * @param amount Amount to format
   * @returns Formatted currency string
   */
  formatCurrency(amount: number): string {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0
    }).format(amount);
  }

  /**
   * Normalize API response shape and process numeric fields.
   * - Ensures top-level fields (net_cash_flow, cash_at_beginning, cash_at_end) exist
   *   even if the backend returns them only under `summary`.
   */
  public normalizeCashFlowResponse(data: any): any {
    if (!data || typeof data !== 'object') return data;
    
    const processValue = (value: any): number => {
      if (value === null || value === undefined || value === '') return 0;
      const num = Number(value);
      return isNaN(num) ? 0 : num;
    };
    
    const processed = { ...data };
    
    // Process top-level numeric fields
    if ('net_cash_flow' in processed) processed.net_cash_flow = processValue(processed.net_cash_flow);
    if ('cash_at_beginning' in processed) processed.cash_at_beginning = processValue(processed.cash_at_beginning);
    if ('cash_at_end' in processed) processed.cash_at_end = processValue(processed.cash_at_end);
    if ('reconciliation_difference' in processed) processed.reconciliation_difference = processValue(processed.reconciliation_difference);
    if ('has_reconciliation_warning' in processed) processed.has_reconciliation_warning = Boolean(processed.has_reconciliation_warning);
    
    // Process operating activities
    if (processed.operating_activities) {
      processed.operating_activities.net_income = processValue(processed.operating_activities.net_income);
      processed.operating_activities.total_operating_cash_flow = processValue(processed.operating_activities.total_operating_cash_flow);
      
      if (processed.operating_activities.adjustments) {
        const adj = processed.operating_activities.adjustments;
        adj.depreciation = processValue(adj.depreciation);
        adj.amortization = processValue(adj.amortization);
        adj.bad_debt_expense = processValue(adj.bad_debt_expense);
        adj.gain_loss_on_asset_disposal = processValue(adj.gain_loss_on_asset_disposal);
        adj.other_non_cash_items = processValue(adj.other_non_cash_items);
        adj.total_adjustments = processValue(adj.total_adjustments);
        
        if (adj.items && Array.isArray(adj.items)) {
          adj.items.forEach((item: any) => {
            if (item && typeof item === 'object') {
              item.amount = processValue(item.amount);
            }
          });
        }
      }
      
      if (processed.operating_activities.working_capital_changes) {
        const wcc = processed.operating_activities.working_capital_changes;
        wcc.accounts_receivable_change = processValue(wcc.accounts_receivable_change);
        wcc.inventory_change = processValue(wcc.inventory_change);
        wcc.prepaid_expenses_change = processValue(wcc.prepaid_expenses_change);
        wcc.accounts_payable_change = processValue(wcc.accounts_payable_change);
        wcc.accrued_liabilities_change = processValue(wcc.accrued_liabilities_change);
        wcc.other_working_capital_change = processValue(wcc.other_working_capital_change);
        wcc.total_working_capital_changes = processValue(wcc.total_working_capital_changes);
        
        if (wcc.items && Array.isArray(wcc.items)) {
          wcc.items.forEach((item: any) => {
            if (item && typeof item === 'object') {
              item.amount = processValue(item.amount);
            }
          });
        }
      }
    }
    
    // Process investing activities
    if (processed.investing_activities) {
      const inv = processed.investing_activities;
      inv.purchase_of_fixed_assets = processValue(inv.purchase_of_fixed_assets);
      inv.sale_of_fixed_assets = processValue(inv.sale_of_fixed_assets);
      inv.purchase_of_investments = processValue(inv.purchase_of_investments);
      inv.sale_of_investments = processValue(inv.sale_of_investments);
      inv.intangible_asset_purchases = processValue(inv.intangible_asset_purchases);
      inv.other_investing_activities = processValue(inv.other_investing_activities);
      inv.total_investing_cash_flow = processValue(inv.total_investing_cash_flow);
      
      if (inv.items && Array.isArray(inv.items)) {
        inv.items.forEach((item: any) => {
          if (item && typeof item === 'object') {
            item.amount = processValue(item.amount);
          }
        });
      }
    }
    
    // Process financing activities
    if (processed.financing_activities) {
      const fin = processed.financing_activities;
      fin.share_capital_increase = processValue(fin.share_capital_increase);
      fin.share_capital_decrease = processValue(fin.share_capital_decrease);
      fin.long_term_debt_increase = processValue(fin.long_term_debt_increase);
      fin.long_term_debt_decrease = processValue(fin.long_term_debt_decrease);
      fin.short_term_debt_increase = processValue(fin.short_term_debt_increase);
      fin.short_term_debt_decrease = processValue(fin.short_term_debt_decrease);
      fin.dividends_paid = processValue(fin.dividends_paid);
      fin.other_financing_activities = processValue(fin.other_financing_activities);
      fin.total_financing_cash_flow = processValue(fin.total_financing_cash_flow);
      
      if (fin.items && Array.isArray(fin.items)) {
        fin.items.forEach((item: any) => {
          if (item && typeof item === 'object') {
            item.amount = processValue(item.amount);
          }
        });
      }
    }
    
    // Process cash flow ratios
    if (processed.cash_flow_ratios) {
      const ratios = processed.cash_flow_ratios;
      ratios.operating_cash_flow_ratio = processValue(ratios.operating_cash_flow_ratio);
      ratios.cash_flow_to_debt_ratio = processValue(ratios.cash_flow_to_debt_ratio);
      ratios.free_cash_flow = processValue(ratios.free_cash_flow);
      if ('cash_flow_per_share' in ratios) {
        ratios.cash_flow_per_share = processValue(ratios.cash_flow_per_share);
      }
    }
    
    // Fallback from summary if top-level values are missing
    if (processed.summary && typeof processed.summary === 'object') {
      const s = processed.summary;
      if (processed.net_cash_flow === undefined || processed.net_cash_flow === 0) {
        processed.net_cash_flow = processValue(s.net_cash_flow);
      }
      if (processed.cash_at_beginning === undefined || processed.cash_at_beginning === 0) {
        processed.cash_at_beginning = processValue(s.cash_at_beginning);
      }
      if (processed.cash_at_end === undefined || processed.cash_at_end === 0) {
        processed.cash_at_end = processValue(s.cash_at_end);
      }
      // Also expose section totals if helpful
      if (!processed.operating_cash_flow && s.operating_cash_flow !== undefined) {
        processed.operating_cash_flow = processValue(s.operating_cash_flow);
      }
      if (!processed.investing_cash_flow && s.investing_cash_flow !== undefined) {
        processed.investing_cash_flow = processValue(s.investing_cash_flow);
      }
      if (!processed.financing_cash_flow && s.financing_cash_flow !== undefined) {
        processed.financing_cash_flow = processValue(s.financing_cash_flow);
      }
    }

    // Process account details
    if (processed.account_details && Array.isArray(processed.account_details)) {
      processed.account_details.forEach((account: any) => {
        if (account && typeof account === 'object') {
          account.debit_total = processValue(account.debit_total);
          account.credit_total = processValue(account.credit_total);
          account.net_balance = processValue(account.net_balance);
        }
      });
    }
    
    return processed;
  }

  /**
   * Get default date range (current month)
   * @returns Object with start_date and end_date
   */
  getDefaultDateRange(): { start_date: string; end_date: string } {
    const today = new Date();
    const firstDayOfMonth = new Date(today.getFullYear(), today.getMonth(), 1);
    
    return {
      start_date: firstDayOfMonth.toISOString().split('T')[0],
      end_date: today.toISOString().split('T')[0]
    };
  }
}

// Export singleton instance
export const ssotCashFlowReportService = new SSOTCashFlowReportService();
