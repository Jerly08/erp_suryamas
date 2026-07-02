import api from './api';
import { API_ENDPOINTS } from '../config/api';

export interface SSOTTrialBalanceData {
  company?: CompanyInfo;
  as_of_date: string;
  currency: string;
  accounts: TrialBalanceAccount[];
  total_debits: number;
  total_credits: number;
  is_balanced: boolean;
  difference: number;
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

export interface TrialBalanceAccount {
  account_id: number;
  account_code: string;
  account_name: string;
  account_type: string;
  debit_balance: number;
  credit_balance: number;
  normal_balance?: string;
  ssot_balance?: number;
}

export interface SSOTTrialBalanceParams {
  as_of_date?: string;
  format?: 'json' | 'pdf' | 'excel';
}

class SSOTTrialBalanceService {

  async generateSSOTTrialBalance(params: SSOTTrialBalanceParams = {}): Promise<SSOTTrialBalanceData> {
    try {
      const queryParams = new URLSearchParams();
      if (params.as_of_date) {
        queryParams.append('as_of_date', params.as_of_date);
      }
      queryParams.append('format', params.format || 'json');

      const response = await api.get(API_ENDPOINTS.SSOT_REPORTS.TRIAL_BALANCE + `?${queryParams}`);

      if (response.data.status === 'success') {
        return response.data.data;
      }

      throw new Error(response.data.message || 'Failed to generate trial balance');
    } catch (error: any) {
      console.error('Error generating SSOT trial balance:', error);
      if (error.response?.data?.message) {
        throw new Error(error.response.data.message);
      }
      throw error;
    }
  }

  formatCurrency(value: number): string {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR'
    }).format(value);
  }

  getAccountTypeIcon(accountType: string): string {
    const icons: { [key: string]: string } = {
      'Asset': '💰',
      'Liability': '📋',
      'Equity': '🏦',
      'Revenue': '📈',
      'Expense': '💸',
      'Other': '📊'
    };
    return icons[accountType] || '📊';
  }

  validateBalance(totalDebits: number, totalCredits: number, tolerance: number = 0.01): boolean {
    return Math.abs(totalDebits - totalCredits) < tolerance;
  }

  /**
   * Export Trial Balance as PDF
   */
  async exportToPDF(params: { as_of_date?: string }): Promise<void> {
    try {
      const queryParams = new URLSearchParams({
        format: 'pdf'
      });

      if (params.as_of_date) {
        queryParams.append('as_of_date', params.as_of_date);
      }

      const url = `${API_ENDPOINTS.SSOT_REPORTS.TRIAL_BALANCE}?${queryParams.toString()}`;
      
      const response = await fetch(url, {
        method: 'GET',
        credentials: 'include',
      });

      if (!response.ok) {
        let errorMessage = `HTTP ${response.status}: ${response.statusText}`;
        try {
          const errorData = await response.json();
          errorMessage = errorData.message || errorMessage;
        } catch {
          // Use default error message if JSON parsing fails
        }
        throw new Error(errorMessage);
      }

      const blob = await response.blob();
      if (blob.size === 0) {
        throw new Error('Empty file received from server');
      }

      const filename = `Trial_Balance_${params.as_of_date || new Date().toISOString().split('T')[0]}.pdf`;
      
      // Trigger download
      const downloadUrl = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = downloadUrl;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(downloadUrl);

    } catch (error) {
      console.error('PDF export error:', error);
      throw new Error(`Failed to export Trial Balance as PDF: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  /**
   * Export Trial Balance as CSV
   */
  async exportToCSV(params: { as_of_date?: string }): Promise<void> {
    try {
      const queryParams = new URLSearchParams({
        format: 'csv'
      });

      if (params.as_of_date) {
        queryParams.append('as_of_date', params.as_of_date);
      }

      const url = `${API_ENDPOINTS.SSOT_REPORTS.TRIAL_BALANCE}?${queryParams.toString()}`;
      
      const response = await fetch(url, {
        method: 'GET',
        credentials: 'include',
      });

      if (!response.ok) {
        let errorMessage = `HTTP ${response.status}: ${response.statusText}`;
        try {
          const errorData = await response.json();
          errorMessage = errorData.message || errorMessage;
        } catch {
          // Use default error message if JSON parsing fails
        }
        throw new Error(errorMessage);
      }

      const blob = await response.blob();
      if (blob.size === 0) {
        throw new Error('Empty file received from server');
      }

      const filename = `Trial_Balance_${params.as_of_date || new Date().toISOString().split('T')[0]}.csv`;
      
      // Trigger download
      const downloadUrl = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = downloadUrl;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(downloadUrl);

    } catch (error) {
      console.error('CSV export error:', error);
      throw new Error(`Failed to export Trial Balance as CSV: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }
}

export const ssotTrialBalanceService = new SSOTTrialBalanceService();
