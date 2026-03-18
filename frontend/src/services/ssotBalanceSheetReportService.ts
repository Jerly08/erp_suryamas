import { API_ENDPOINTS } from '@/config/api';
import { getAuthHeaders } from '@/utils/authTokenUtils';

// SSOT Balance Sheet data structures (aligned with backend)
export interface SSOTBalanceSheetData {
  company: {
    name: string;
  };
  as_of_date: string;
  currency: string;
  
  assets: {
    current_assets: {
      cash: number;
      receivables: number;
      inventory: number;
      prepaid_expenses: number;
      other_current_assets: number;
      total_current_assets: number;
      items: BSAccountItem[];
    };
    non_current_assets: {
      fixed_assets: number;
      intangible_assets: number;
      investments: number;
      other_non_current_assets: number;
      total_non_current_assets: number;
      items: BSAccountItem[];
    };
    total_assets: number;
  };
  
  liabilities: {
    current_liabilities: {
      accounts_payable: number;
      short_term_debt: number;
      accrued_liabilities: number;
      tax_payable: number;
      other_current_liabilities: number;
      total_current_liabilities: number;
      items: BSAccountItem[];
    };
    non_current_liabilities: {
      long_term_debt: number;
      deferred_tax: number;
      other_non_current_liabilities: number;
      total_non_current_liabilities: number;
      items: BSAccountItem[];
    };
    total_liabilities: number;
  };
  
  equity: {
    share_capital: number;
    retained_earnings: number;
    other_equity: number;
    total_equity: number;
    items: BSAccountItem[];
  };
  
  total_liabilities_and_equity: number;
  is_balanced: boolean;
  balance_difference: number;
  
  generated_at: string;
  enhanced: boolean;
  account_details?: SSOTAccountBalance[];
}

export interface BSAccountItem {
  account_code: string;
  account_name: string;
  amount: number;
  account_id?: number;
}

export interface SSOTAccountBalance {
  account_id: number;
  account_code: string;
  account_name: string;
  account_type: string;
  debit_total: number;
  credit_total: number;
  net_balance: number;
}

export interface SSOTBalanceSheetValidation {
  as_of_date: string;
  is_balanced: boolean;
  total_assets: number;
  total_liabilities_and_equity: number;
  balance_difference: number;
  tolerance: number;
  validation_status: 'PASS' | 'FAIL';
  generated_at: string;
  issue?: string;
}

export interface SSOTBalanceSheetArchiveMetadata {
  period_id: number;
  start_date: string;
  end_date: string;
  description: string;
  period_type?: string;
  is_closed: boolean;
  closed_at?: string | null;
  snapshot_available: boolean;
  snapshot_generated?: string | null;
  calculation_basis: string;
  report_display_mode: string;
}

export interface SSOTBalanceSheetComparison {
  from_date: string;
  to_date: string;
  comparison: {
    total_assets: {
      from: number;
      to: number;
      change: number;
      change_percent: number;
    };
    total_liabilities: {
      from: number;
      to: number;
      change: number;
      change_percent: number;
    };
    total_equity: {
      from: number;
      to: number;
      change: number;
      change_percent: number;
    };
  };
  balance_sheet_from: SSOTBalanceSheetData;
  balance_sheet_to: SSOTBalanceSheetData;
}

interface ReportServiceError extends Error {
  status?: number;
}

// Legacy Balance Sheet data structure for compatibility
export interface LegacyBalanceSheetData {
  report_title: string;
  company: string;
  as_of_date: string;
  currency: string;
  
  // Assets section
  assets: {
    current_assets: {
      items: BSAccountItem[];
      subtotal: number;
    };
    non_current_assets: {
      items: BSAccountItem[];
      subtotal: number;
    };
    total_assets: number;
  };
  
  // Liabilities section
  liabilities: {
    current_liabilities: {
      items: BSAccountItem[];
      subtotal: number;
    };
    non_current_liabilities: {
      items: BSAccountItem[];
      subtotal: number;
    };
    total_liabilities: number;
  };
  
  // Equity section
  equity: {
    items: BSAccountItem[];
    total_equity: number;
  };
  
  // Balance validation
  total_liabilities_and_equity: number;
  is_balanced: boolean;
  balance_difference: number;
  
  // Metadata
  generated_at: string;
  enhanced: boolean;
  ssot_source: boolean;
}

class SSOTBalanceSheetReportService {
  private getAuthHeaders() {
    return getAuthHeaders();
  }

  private getLocalDateString(): string {
    const now = new Date();
    const localTime = new Date(now.getTime() - now.getTimezoneOffset() * 60000);
    return localTime.toISOString().split('T')[0];
  }

  private buildQueryString(params: Record<string, string | number | boolean | undefined>): string {
    const searchParams = new URLSearchParams();
    
    Object.entries(params).forEach(([key, value]) => {
      // Add additional check to prevent undefined values from being converted to "undefined" string
      if (value !== undefined && value !== null && value !== '' && value !== 'ALL') {
        searchParams.append(key, value.toString());
      }
    });

    return searchParams.toString();
  }

  private async buildResponseError(response: Response, fallbackMessage: string): Promise<ReportServiceError> {
    let message = fallbackMessage;

    try {
      const error = await response.json();
      message =
        error?.message ||
        error?.error ||
        fallbackMessage;
    } catch {
      message = `${fallbackMessage}. Server returned: ${response.status} ${response.statusText}`;
    }

    const serviceError = new Error(message) as ReportServiceError;
    serviceError.status = response.status;
    return serviceError;
  }

  // Generate SSOT Balance Sheet
  async generateSSOTBalanceSheet(params: {
    as_of_date?: string;
    format?: 'json' | 'summary';
  } = {}): Promise<SSOTBalanceSheetData> {
    const queryParams = {
      as_of_date: params.as_of_date || this.getLocalDateString(),
      format: params.format || 'json'
    };
    
    const queryString = this.buildQueryString(queryParams);
    const baseUrl = API_ENDPOINTS.SSOT_REPORTS.BALANCE_SHEET;
    const url = baseUrl + (queryString ? '?' + queryString : '');
    
    // Log the request for debugging
    console.log('Making SSOT Balance Sheet request to:', url);
    console.log('Base URL:', baseUrl);
    console.log('Query params:', queryParams);
    console.log('Query string:', queryString);
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      throw await this.buildResponseError(
        response,
        `Failed to generate SSOT Balance Sheet`
      );
    }

    const result = await response.json();
    return result.data;
  }

  async getArchivedSSOTBalanceSheet(params: {
    period_id?: number;
    end_date?: string;
  }): Promise<{ data: SSOTBalanceSheetData; archive: SSOTBalanceSheetArchiveMetadata }> {
    const queryString = this.buildQueryString({
      period_id: params.period_id,
      end_date: params.end_date,
    });
    const baseUrl = API_ENDPOINTS.SSOT_REPORTS.BALANCE_SHEET_ARCHIVE;
    const url = baseUrl + (queryString ? '?' + queryString : '');

    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      throw await this.buildResponseError(
        response,
        `Failed to generate archived SSOT Balance Sheet`
      );
    }

    const result = await response.json();
    return {
      data: result.data,
      archive: result.archive,
    };
  }

  // Generate SSOT Balance Sheet as PDF
  async generateSSOTBalanceSheetPDF(params: {
    as_of_date?: string;
  }): Promise<Blob> {
    const queryParams = {
      as_of_date: params.as_of_date || this.getLocalDateString(),
      format: 'pdf'
    };
    
    // Use the correct endpoint from the SSOT_REPORTS nested object
    const baseUrl = API_ENDPOINTS.SSOT_REPORTS.BALANCE_SHEET;
    const queryString = this.buildQueryString(queryParams);
    const url = baseUrl + (queryString ? '?' + queryString : '');
    
    // Log the request for debugging
    console.log('Making PDF export request to:', url);
    console.log('Base URL:', baseUrl);
    console.log('Query params:', queryParams);
    console.log('Query string:', queryString);
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      // Try to parse error as JSON first, fallback to text if it's HTML
      try {
        const error = await response.json();
        throw new Error(error.message || `Failed to generate SSOT Balance Sheet PDF: ${response.statusText}`);
      } catch {
        // If JSON parsing fails, it's likely HTML error page
        throw new Error(`Failed to generate SSOT Balance Sheet PDF. Server returned: ${response.status} ${response.statusText}`);
      }
    }

    return await response.blob();
  }

  // Generate SSOT Balance Sheet as CSV
  async generateSSOTBalanceSheetCSV(params: {
    as_of_date?: string;
  }): Promise<string> {
    const queryParams = {
      as_of_date: params.as_of_date || this.getLocalDateString(),
      format: 'csv'
    };
    
    // Use the correct endpoint from the SSOT_REPORTS nested object
    const baseUrl = API_ENDPOINTS.SSOT_REPORTS.BALANCE_SHEET;
    const queryString = this.buildQueryString(queryParams);
    const url = baseUrl + (queryString ? '?' + queryString : '');
    
    // Log the request for debugging
    console.log('Making CSV export request to:', url);
    console.log('Base URL:', baseUrl);
    console.log('Query params:', queryParams);
    console.log('Query string:', queryString);
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    console.log('CSV export response status:', response.status);
    console.log('CSV export response headers:', [...response.headers.entries()]);

    // Check if the response is HTML (error page) before processing
    const contentType = response.headers.get('content-type') || '';
    console.log('Response content type:', contentType);
    
    // If it's an HTML error page or error status, handle it appropriately
    if (contentType.includes('text/html') || response.status >= 400) {
      const errorText = await response.text();
      console.error('CSV export error response:', errorText);
      
      // Try to parse as JSON first in case it's a JSON error response
      try {
        const errorJson = JSON.parse(errorText);
        throw new Error(errorJson.message || errorJson.error || `Failed to generate SSOT Balance Sheet CSV: ${response.statusText}`);
      } catch {
        // If not JSON, check if it's HTML
        if (errorText.trim().startsWith('<')) {
          throw new Error(`Failed to generate SSOT Balance Sheet CSV. Server returned an HTML error page.`);
        }
        // Otherwise use the text content as the error message
        throw new Error(`Failed to generate SSOT Balance Sheet CSV. Server returned: ${errorText || response.statusText}`);
      }
    }

    // For CSV format, the backend now returns actual CSV content
    const csvContent = await response.text();
    console.log('CSV export content preview:', csvContent.substring(0, 200));
    
    // Additional check if the response is actually HTML instead of CSV
    if (csvContent.trim().startsWith('<')) {
      throw new Error('Failed to generate SSOT Balance Sheet CSV. Server returned an HTML error page instead of CSV content.');
    }
    
    return csvContent;
  }

  // Get SSOT Balance Sheet account details for drilldown
  async getBalanceSheetAccountDetails(params: {
    as_of_date?: string;
    account_type?: 'ASSET' | 'LIABILITY' | 'EQUITY';
  } = {}): Promise<{
    as_of_date: string;
    account_type?: string;
    account_details: SSOTAccountBalance[];
    total_accounts: number;
  }> {
    const queryString = this.buildQueryString({
      as_of_date: params.as_of_date || this.getLocalDateString(),
      account_type: params.account_type
    });
    
    const url = API_ENDPOINTS.SSOT_REPORTS.BALANCE_SHEET_DETAILS + (queryString ? '?' + queryString : '');
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      throw new Error(error.message || `Failed to get Balance Sheet account details: ${response.statusText}`);
    }

    const result = await response.json();
    return result.data;
  }

  // Validate SSOT Balance Sheet
  async validateBalanceSheet(params: {
    as_of_date?: string;
  } = {}): Promise<SSOTBalanceSheetValidation> {
    const queryString = this.buildQueryString({
      as_of_date: params.as_of_date || this.getLocalDateString()
    });
    
    const url = API_ENDPOINTS.SSOT_REPORTS.BALANCE_SHEET_VALIDATE + (queryString ? '?' + queryString : '');
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      throw new Error(error.message || `Failed to validate Balance Sheet: ${response.statusText}`);
    }

    const result = await response.json();
    return result.data;
  }

  // Compare Balance Sheets between two dates
  async compareBalanceSheets(params: {
    from_date?: string;
    to_date?: string;
  } = {}): Promise<SSOTBalanceSheetComparison> {
    const queryString = this.buildQueryString({
      from_date: params.from_date,
      to_date: params.to_date || new Date().toISOString().split('T')[0]
    });
    
    const url = API_ENDPOINTS.SSOT_REPORTS.BALANCE_SHEET_COMPARISON + (queryString ? '?' + queryString : '');
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      throw new Error(error.message || `Failed to compare Balance Sheets: ${response.statusText}`);
    }

    const result = await response.json();
    return result.data;
  }

  // Convert SSOT Balance Sheet data to legacy format for compatibility
  convertToLegacyFormat(ssotData: SSOTBalanceSheetData): LegacyBalanceSheetData {
    return {
      report_title: 'Balance Sheet',
      company: ssotData.company.name,
      as_of_date: ssotData.as_of_date,
      currency: ssotData.currency,
      
      // Assets section
      assets: {
        current_assets: {
          items: ssotData.assets.current_assets.items,
          subtotal: ssotData.assets.current_assets.total_current_assets
        },
        non_current_assets: {
          items: ssotData.assets.non_current_assets.items,
          subtotal: ssotData.assets.non_current_assets.total_non_current_assets
        },
        total_assets: ssotData.assets.total_assets
      },
      
      // Liabilities section
      liabilities: {
        current_liabilities: {
          items: ssotData.liabilities.current_liabilities.items,
          subtotal: ssotData.liabilities.current_liabilities.total_current_liabilities
        },
        non_current_liabilities: {
          items: ssotData.liabilities.non_current_liabilities.items,
          subtotal: ssotData.liabilities.non_current_liabilities.total_non_current_liabilities
        },
        total_liabilities: ssotData.liabilities.total_liabilities
      },
      
      // Equity section
      equity: {
        items: ssotData.equity.items,
        total_equity: ssotData.equity.total_equity
      },
      
      // Balance validation
      total_liabilities_and_equity: ssotData.total_liabilities_and_equity,
      is_balanced: ssotData.is_balanced,
      balance_difference: ssotData.balance_difference,
      
      // Metadata
      generated_at: ssotData.generated_at,
      enhanced: ssotData.enhanced,
      ssot_source: true
    };
  }

  // Format currency amounts
  formatCurrency(amount: number, currency: string = 'IDR'): string {
    if (currency === 'IDR') {
      return new Intl.NumberFormat('id-ID', {
        style: 'currency',
        currency: 'IDR',
        minimumFractionDigits: 0
      }).format(amount);
    } else {
      return new Intl.NumberFormat('en-US', {
        style: 'currency',
        currency: 'USD'
      }).format(amount);
    }
  }

  // Format percentage
  formatPercentage(value: number): string {
    return new Intl.NumberFormat('id-ID', {
      style: 'percent',
      minimumFractionDigits: 1,
      maximumFractionDigits: 2
    }).format(value / 100);
  }

  // Get Balance Sheet summary for dashboard
  async getBalanceSheetSummary(asOfDate?: string): Promise<{
    total_assets: number;
    total_liabilities: number;
    total_equity: number;
    is_balanced: boolean;
    as_of_date: string;
  }> {
    try {
      const balanceSheet = await this.generateSSOTBalanceSheet({
        as_of_date: asOfDate,
        format: 'summary'
      });

      return {
        total_assets: balanceSheet.assets.total_assets,
        total_liabilities: balanceSheet.liabilities.total_liabilities,
        total_equity: balanceSheet.equity.total_equity,
        is_balanced: balanceSheet.is_balanced,
        as_of_date: balanceSheet.as_of_date
      };
    } catch (error) {
      console.error('Error fetching Balance Sheet summary:', error);
      throw error;
    }
  }
}

export const ssotBalanceSheetReportService = new SSOTBalanceSheetReportService();
export default ssotBalanceSheetReportService;
