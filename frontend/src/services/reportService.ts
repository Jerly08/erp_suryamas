import { API_V1_BASE } from '../config/api';
import { getAuthHeaders } from '../utils/authTokenUtils';

export interface Report {
  id: string;
  name: string;
  description: string;
  type: 'Financial' | 'Operational' | 'Analytical';
  category: string;
  parameters: string[];
}

export interface ReportData {
  id: string;
  title: string;
  type: string;
  period: string;
  generated_at: string;
  data: any;
  summary: { [key: string]: number };
  parameters: { [key: string]: any };
}

export interface ReportParameters {
  start_date?: string;
  end_date?: string;
  previous_start_date?: string;
  previous_end_date?: string;
  as_of_date?: string;
  group_by?: 'month' | 'quarter' | 'year';
  customer_id?: string;
  vendor_id?: string;
  account_id?: string; // Updated to match backend parameter
  include_valuation?: boolean;
  period?: 'current' | 'ytd' | 'comparative';
  format?: 'json' | 'pdf' | 'csv' | 'excel';
  status?: string; // For journal entry filtering
  reference_type?: string; // For journal entry filtering
}

class ReportService {
  private getAuthHeaders() {
    // Use centralized token utility for consistency across the application
    return getAuthHeaders();
  }

  private buildQueryString(params: ReportParameters): string {
    const searchParams = new URLSearchParams();

    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== null && value !== '' && value !== 'ALL') {
        searchParams.append(key, value.toString());
      }
    });

    return searchParams.toString();
  }

  private formatErrorMessage(textResponse: string, status: number): string {
    if (!textResponse || textResponse.trim() === '') {
      return this.getStatusMessage(status);
    }
    
    // Clean up common backend error formats
    const cleanMessage = textResponse
      .replace(/ERROR:\s*/gi, '')
      .replace(/SQLSTATE\s+\d+/gi, '')
      .replace(/\(.*?\)$/, '') // Remove parenthetical codes at end
      .trim();
    
    return cleanMessage || this.getStatusMessage(status);
  }

  private getStatusMessage(status: number): string {
    switch (status) {
      case 400: return 'Invalid request parameters. Please check your input.';
      case 401: return 'Authentication required. Please log in again.';
      case 403: return 'You do not have permission to access this report.';
      case 404: return 'Report endpoint not found. Please contact support.';
      case 500: return 'Server error occurred. Please try again later.';
      case 503: return 'Service temporarily unavailable. Please try again later.';
      default: return `HTTP ${status}: Request failed`;
    }
  }

  private async handleUnifiedResponse<T>(response: Response): Promise<T> {
    if (!response.ok) {
      let errorData: any;
      try {
        const textResponse = await response.text();
        // Try to parse as JSON first
        try {
          errorData = JSON.parse(textResponse);
        } catch {
          // If not JSON, use text as error message with better formatting
          errorData = { message: this.formatErrorMessage(textResponse, response.status) };
        }
      } catch {
        errorData = {
          error: { 
            code: 'NETWORK_ERROR',
            message: this.getStatusMessage(response.status)
          }
        };
      }

      // Extract error message from various possible structures
      const errorMessage = 
        errorData.error?.message || 
        errorData.message || 
        errorData.errors?.[0] ||
        `HTTP ${response.status}: ${response.statusText}`;
      
      throw new Error(errorMessage);
    }
    
    const contentType = response.headers.get('content-type') || '';
    
    // Check for PDF, Excel, or other binary content
    if (contentType.includes('application/pdf') || 
        contentType.includes('application/vnd.openxmlformats') ||
        contentType.includes('application/vnd.ms-excel') ||
        contentType.includes('text/csv') ||
        contentType.includes('application/octet-stream')) {
      const blob = await response.blob();
      if (blob.size === 0) {
        throw new Error('Received empty file from server');
      }
      return blob as any;
    } else if (contentType.includes('application/json')) {
      try {
        const result = await response.json();
        // Handle unified response structure
        if (result.status === 'error') {
          throw new Error(result.message || 'Request failed');
        }
        if (result.status === 'success' && result.data) {
          return result.data;
        }
        if (result.success !== undefined) {
          if (!result.success) {
            throw new Error(result.error?.message || result.message || 'Request failed');
          }
          return result.data;
        }
        // Handle direct data response without wrapper
        if (result.report_header || result.revenue || result.assets) {
          return result;
        }
        return result.data || result;
      } catch (jsonError) {
        if (jsonError instanceof Error && jsonError.message.includes('Request failed')) {
          throw jsonError;
        }
        throw new Error('Invalid JSON response from server');
      }
    } else {
      // Fallback: try to get as text first
      try {
        const text = await response.text();
        if (text && text.trim()) {
          // Try parsing as JSON one more time
          try {
            const jsonData = JSON.parse(text);
            if (jsonData.status === 'success') {
              return jsonData.data;
            }
            return jsonData;
          } catch {
            // Not JSON, treat as error
            throw new Error(`Unexpected response format: ${text.substring(0, 200)}`);
          }
        } else {
          throw new Error('Received empty response from server');
        }
      } catch (textError) {
        if (textError instanceof Error) {
          throw textError;
        }
        throw new Error('Failed to process server response');
      }
    }
  }

  // Get list of available reports
  async getAvailableReports(): Promise<Report[]> {
    const response = await fetch(`${API_V1_BASE}/reports`, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      throw new Error('Failed to fetch available reports');
    }

    const result = await response.json();
    return result.data || [];
  }

  // Generate Balance Sheet
  async generateBalanceSheet(params: ReportParameters): Promise<ReportData | Blob> {
    const queryString = this.buildQueryString(params);
    const url = `${API_V1_BASE}/reports/balance-sheet${queryString ? '?' + queryString : ''}`;

    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    return this.handleUnifiedResponse(response);
  }

  // Generate Profit & Loss Statement (Enhanced Version)
  async generateProfitLoss(params: ReportParameters): Promise<ReportData | Blob> {
    if (!params.start_date || !params.end_date) {
      throw new Error('Start date and end date are required for profit & loss statement');
    }

    const queryString = this.buildQueryString(params);
    const url = `${API_V1_BASE}/reports/profit-loss${queryString ? '?' + queryString : ''}`;
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    return this.handleUnifiedResponse(response);
  }

  // Generate Enhanced Financial Metrics
  async generateFinancialMetrics(params: ReportParameters): Promise<any> {
    if (!params.start_date || !params.end_date) {
      throw new Error('Start date and end date are required for financial metrics');
    }

    const queryString = this.buildQueryString(params);
    const url = `${API_V1_BASE}/reports/profit-loss${queryString ? '?' + queryString : ''}`;
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    return this.handleUnifiedResponse(response);
  }

  // Generate P&L Period Comparison
  async generateProfitLossComparison(currentPeriod: {start: string, end: string}, previousPeriod: {start: string, end: string}): Promise<any> {
    const params: ReportParameters = {
      start_date: currentPeriod.start,
      end_date: currentPeriod.end,
      previous_start_date: previousPeriod.start,
      previous_end_date: previousPeriod.end,
      format: 'json'
    };

    const queryString = this.buildQueryString(params);
    const url = `${API_V1_BASE}/reports/profit-loss${queryString ? '?' + queryString : ''}`;
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    return this.handleUnifiedResponse(response);
  }

  // Generate Cash Flow Statement (SSOT)
  async generateCashFlow(params: ReportParameters): Promise<ReportData | Blob> {
    if (!params.start_date || !params.end_date) {
      throw new Error('Start date and end date are required for cash flow statement');
    }

    const queryString = this.buildQueryString(params);
    // Use SSOT Cash Flow endpoint for real-time data from journal system
    const url = `${API_V1_BASE}/ssot-reports/cash-flow${queryString ? '?' + queryString : ''}`;
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    return this.handleUnifiedResponse(response);
  }

  // Generate Trial Balance
  async generateTrialBalance(params: ReportParameters): Promise<ReportData | Blob> {
    const queryString = this.buildQueryString(params);
    const url = `${API_V1_BASE}/reports/trial-balance${queryString ? '?' + queryString : ''}`;
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    return this.handleUnifiedResponse(response);
  }

  // Generate General Ledger
  async generateGeneralLedger(params: ReportParameters): Promise<ReportData | Blob> {
    if (!params.start_date || !params.end_date) {
      throw new Error('Start date and end date are required for general ledger');
    }

    const queryString = this.buildQueryString(params);
    const url = `${API_V1_BASE}/reports/general-ledger${queryString ? '?' + queryString : ''}`;
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    return this.handleUnifiedResponse(response);
  }

  // Generate Accounts Receivable Report
  async generateAccountsReceivable(params: ReportParameters): Promise<ReportData | Blob> {
    const queryString = this.buildQueryString(params);
    const url = `${API_V1_BASE}/reports/accounts-receivable${queryString ? '?' + queryString : ''}`;
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      throw new Error('Failed to generate accounts receivable report');
    }

    if (params.format === 'pdf') {
      return await response.blob();
    }

    const result = await response.json();
    return result.data;
  }

  // Generate Accounts Payable Report
  async generateAccountsPayable(params: ReportParameters): Promise<ReportData | Blob> {
    const queryString = this.buildQueryString(params);
    const url = `${API_V1_BASE}/reports/accounts-payable${queryString ? '?' + queryString : ''}`;
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      throw new Error('Failed to generate accounts payable report');
    }

    if (params.format === 'pdf') {
      return await response.blob();
    }

    const result = await response.json();
    return result.data;
  }

  // Generate Sales Summary Report
  async generateSalesSummary(params: ReportParameters): Promise<ReportData | Blob> {
    if (!params.start_date || !params.end_date) {
      throw new Error('Start date and end date are required for sales summary');
    }

    const queryString = this.buildQueryString(params);
    const url = `${API_V1_BASE}/reports/sales-summary${queryString ? '?' + queryString : ''}`;
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    return this.handleUnifiedResponse(response);
  }

  // Generate Purchase Summary Report
  async generatePurchaseSummary(params: ReportParameters): Promise<ReportData | Blob> {
    if (!params.start_date || !params.end_date) {
      throw new Error('Start date and end date are required for purchase summary');
    }

    const queryString = this.buildQueryString(params);
    const url = `${API_V1_BASE}/reports/purchase-summary${queryString ? '?' + queryString : ''}`;
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    return this.handleUnifiedResponse(response);
  }

  // Generate Vendor Analysis Report
  async generateVendorAnalysis(params: ReportParameters): Promise<ReportData | Blob> {
    if (!params.start_date || !params.end_date) {
      throw new Error('Start date and end date are required for vendor analysis');
    }

    const queryString = this.buildQueryString(params);
    const url = `${API_V1_BASE}/reports/vendor-analysis${queryString ? '?' + queryString : ''}`;
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    return this.handleUnifiedResponse(response);
  }

  // Generate Inventory Report
  async generateInventoryReport(params: ReportParameters): Promise<ReportData | Blob> {
    const queryString = this.buildQueryString(params);
    const url = `${API_V1_BASE}/reports/inventory-report${queryString ? '?' + queryString : ''}`;
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      throw new Error('Failed to generate inventory report');
    }

    if (params.format === 'pdf') {
      return await response.blob();
    }

    const result = await response.json();
    return result.data;
  }

  // Generate Financial Ratios Analysis
  async generateFinancialRatios(params: ReportParameters): Promise<ReportData | Blob> {
    const queryString = this.buildQueryString(params);
    const url = `${API_V1_BASE}/reports/financial-ratios${queryString ? '?' + queryString : ''}`;
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      throw new Error('Failed to generate financial ratios analysis');
    }

    if (params.format === 'pdf') {
      return await response.blob();
    }

    const result = await response.json();
    return result.data;
  }

  // Generate Journal Entry Analysis (SSOT endpoint)
  async generateJournalEntryAnalysis(params: ReportParameters): Promise<ReportData | Blob> {
    if (!params.start_date || !params.end_date) {
      throw new Error('Start date and end date are required for journal entry analysis');
    }

    const queryString = this.buildQueryString(params);
    // Route to SSOT endpoint that supports PDF/CSV
    const url = `${API_V1_BASE}/ssot-reports/journal-analysis${queryString ? '?' + queryString : ''}`;
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    return this.handleUnifiedResponse(response);
  }

  // Generic report generator
  async generateReport(reportId: string, params: ReportParameters): Promise<ReportData | Blob> {
    // Handle journal entry analysis separately as it uses different endpoint
    if (reportId === 'journal-entry-analysis') {
      return this.generateJournalEntryAnalysis(params);
    }
    
    
    // Handle special cases that use different endpoints
    if (reportId === 'cash-flow') {
      return this.generateCashFlow(params);
    }
    
    // Map report IDs to endpoints. Values that start with '/'
    // are treated as absolute under API_V1_BASE (e.g. '/ssot-reports/...').
    // Otherwise they are appended under '/reports/'.
    const endpointMap: { [key: string]: string } = {
      // All SSOT reports now live under /api/v1/ssot-reports/* for consistency
      'balance-sheet': '/ssot-reports/balance-sheet',
      // SSOT P&L under /api/v1/reports/ssot-profit-loss (separate endpoint)
      'profit-loss': 'ssot-profit-loss',
      // Other SSOT reports under /api/v1/ssot-reports/*
      'trial-balance': '/ssot-reports/trial-balance',
      'general-ledger': '/ssot-reports/general-ledger',
      'purchase-report': '/ssot-reports/purchase-report',
      // Legacy/other reports remain under /api/v1/reports/*
      'accounts-receivable': 'accounts-receivable',
      'accounts-payable': 'accounts-payable',
      'sales-summary': 'sales-summary',
      'purchase-summary': 'purchase-summary',
      'vendor-analysis': 'vendor-analysis',
      'inventory-report': 'inventory-report',
      'financial-ratios': 'financial-ratios',
      'customer-history': 'customer-history',
      'vendor-history': 'vendor-history'
    };

    const endpoint = endpointMap[reportId];
    if (!endpoint) {
      throw new Error(`Unknown report type: ${reportId}`);
    }

    const queryString = this.buildQueryString(params);
    // Build URL based on whether the endpoint is absolute (starts with '/')
    const url = endpoint.startsWith('/')
      ? `${API_V1_BASE}${endpoint}${queryString ? '?' + queryString : ''}`
      : `${API_V1_BASE}/reports/${endpoint}${queryString ? '?' + queryString : ''}`;

    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    return this.handleUnifiedResponse(response);
  }

  // Download report as file
  async downloadReport(reportData: any, fileName: string): Promise<void> {
    try {
      // Check if reportData is actually a Blob
      if (!(reportData instanceof Blob)) {
        console.error('Invalid data type for download:', typeof reportData, reportData);
        throw new Error('Download failed: Invalid file data received from server');
      }

      // Check if it's a valid blob with size > 0
      if (reportData.size === 0) {
        throw new Error('Download failed: Empty file received from server');
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
      throw new Error(`Failed to download report: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  // Get report templates (if implemented)
  async getReportTemplates(): Promise<any[]> {
    const response = await fetch(`${API_V1_BASE}/reports/templates`, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      throw new Error('Failed to fetch report templates');
    }

    const result = await response.json();
    return result.data || [];
  }

  // Save report template (if implemented)
  async saveReportTemplate(template: {
    name: string;
    type: string;
    description: string;
    template: string;
    is_default: boolean;
  }): Promise<any> {
    const response = await fetch(`${API_V1_BASE}/reports/templates`, {
      method: 'POST',
      headers: this.getAuthHeaders(),
      body: JSON.stringify(template),
    });

    if (!response.ok) {
      throw new Error('Failed to save report template');
    }

    const result = await response.json();
    return result.data;
  }

  // Professional Reports - Now handled by unified generateReport method
  // These methods are deprecated, use generateReport or generateUnifiedReport instead

  // Generic professional report generator - Updated with unified handler
  async generateProfessionalReport(reportType: string, params: ReportParameters): Promise<any | Blob> {
    // Use the same endpoint as generateReport - they are unified now
    return this.generateReport(reportType, params);
  }

  // Generate preview data (JSON format only) - Updated with unified handler
  async generateReportPreview(reportType: string, params: ReportParameters): Promise<ReportData> {
    // Force JSON format for preview and use the unified generateReport method
    const previewParams: ReportParameters = { ...params, format: 'json' };
    const result = await this.generateReport(reportType, previewParams);
    return result as ReportData;
  }

}

export const reportService = new ReportService();
