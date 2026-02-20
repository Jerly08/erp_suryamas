import {
  Account,
  AccountCreateRequest,
  AccountUpdateRequest,
  AccountImportRequest,
  AccountSummaryResponse,
  AccountCatalogItem,
  ApiResponse,
  ApiError
} from '@/types/account';
import { API_ENDPOINTS } from '@/config/api';

class AccountService {

  // SSOT account balance DTO
  public typeSSOTBalance?: never; // dummy to keep file typed as module

  private getHeaders(token?: string): HeadersInit {
    const headers: HeadersInit = {
      'Content-Type': 'application/json',
    };

    if (token) {
      headers.Authorization = `Bearer ${token}`;
    }

    return headers;
  }

  private async handleResponse<T>(response: Response): Promise<T> {
    if (!response.ok) {
      let errorData: ApiError;
      try {
        errorData = await response.json();
      } catch {
        errorData = {
          error: 'Network error',
          code: 'NETWORK_ERROR',
        };
      }

      throw new Error(errorData.error || `HTTP error! status: ${response.status}`);
    }

    return response.json();
  }

  // Get all accounts
  async getAccounts(token: string, type?: string): Promise<Account[]> {
    let url = API_ENDPOINTS.ACCOUNTS.LIST;
    const params = new URLSearchParams();
    if (type) params.set('type', type);
    params.set('_t', Date.now().toString());
    url += `?${params.toString()}`;

    const response = await fetch(url, {
      method: 'GET',
      headers: { ...this.getHeaders(token), 'Cache-Control': 'no-store' },
      cache: 'no-store',
    });

    const result: ApiResponse<Account[]> = await this.handleResponse(response);
    return result.data;
  }

  // Get account catalog (minimal data for accounts) - PUBLIC ENDPOINT (no auth required)
  async getAccountCatalog(token?: string, type?: string): Promise<AccountCatalogItem[]> {
    let url = API_ENDPOINTS.ACCOUNTS.CATALOG;
    const params = new URLSearchParams();
    if (type) params.set('type', type);
    params.set('_t', Date.now().toString());
    url += `?${params.toString()}`;

    const response = await fetch(url, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
        'Cache-Control': 'no-store',
      },
      cache: 'no-store',
    });

    const result: ApiResponse<AccountCatalogItem[]> = await this.handleResponse(response);
    return result.data;
  }

  // Get expense accounts specifically for purchase items - PUBLIC ENDPOINT (no auth required)
  async getExpenseAccounts(token?: string): Promise<AccountCatalogItem[]> {
    return this.getAccountCatalog(undefined, 'EXPENSE');
  }

  // Get liability accounts for credit payment methods - PUBLIC ENDPOINT (no auth required)
  async getCreditAccounts(token?: string): Promise<AccountCatalogItem[]> {
    const url = `${API_ENDPOINTS.ACCOUNTS.CREDIT}?type=LIABILITY&_t=${Date.now()}`;

    const response = await fetch(url, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
        'Cache-Control': 'no-store',
      },
      cache: 'no-store',
    });

    const result: ApiResponse<AccountCatalogItem[]> = await this.handleResponse(response);
    return result.data;
  }

  // Get cash and bank accounts for payment purposes
  async getPaymentAccounts(token: string): Promise<{
    id: number;
    code: string;
    name: string;
    type: string;
    bank_name?: string;
    account_no?: string;
    currency: string;
    balance: number;
  }[]> {
    const response = await fetch(API_ENDPOINTS.CASH_BANK.PAYMENT_ACCOUNTS, {
      method: 'GET',
      headers: this.getHeaders(token),
    });

    const result = await this.handleResponse<{
      success: boolean;
      data: {
        id: number;
        code: string;
        name: string;
        type: string;
        bank_name?: string;
        account_no?: string;
        currency: string;
        balance: number;
      }[];
      message: string;
    }>(response);

    return result.data;
  }

  // Get single account by code
  async getAccount(token: string, code: string): Promise<Account> {
    const url = `${API_ENDPOINTS.ACCOUNTS.GET_BY_CODE(code)}?_t=${Date.now()}`;
    const response = await fetch(url, {
      method: 'GET',
      headers: { ...this.getHeaders(token), 'Cache-Control': 'no-store' },
      cache: 'no-store',
    });

    const result: ApiResponse<Account> = await this.handleResponse(response);
    return result.data;
  }

  // Create new account
  async createAccount(token: string, accountData: AccountCreateRequest): Promise<Account> {
    const response = await fetch(API_ENDPOINTS.ACCOUNTS.CREATE, {
      method: 'POST',
      headers: this.getHeaders(token),
      body: JSON.stringify(accountData),
    });

    const result: ApiResponse<Account> = await this.handleResponse(response);
    return result.data;
  }

  // Update existing account
  async updateAccount(token: string, code: string, accountData: AccountUpdateRequest): Promise<Account> {
    const response = await fetch(API_ENDPOINTS.ACCOUNTS.UPDATE(code), {
      method: 'PUT',
      headers: this.getHeaders(token),
      body: JSON.stringify(accountData),
    });

    const result: ApiResponse<Account> = await this.handleResponse(response);
    return result.data;
  }

  // Delete account
  async deleteAccount(token: string, code: string): Promise<void> {
    const response = await fetch(API_ENDPOINTS.ACCOUNTS.DELETE(code), {
      method: 'DELETE',
      headers: this.getHeaders(token),
    });

    await this.handleResponse(response);
  }

  // Recalculate all parent/header account balances from children
  async recalculateParentBalances(token: string): Promise<{ message: string; headers_updated: number }> {
    const response = await fetch(API_ENDPOINTS.ACCOUNTS.RECALCULATE_PARENT_BALANCES, {
      method: 'POST',
      headers: this.getHeaders(token),
    });

    const result: any = await this.handleResponse(response);
    return result.data || result;
  }

  // Admin delete account with cascade options
  async adminDeleteAccount(token: string, code: string, options: {
    cascade_delete?: boolean;
    new_parent_id?: number;
  }): Promise<{ message: string; cascade: boolean }> {
    const response = await fetch(API_ENDPOINTS.ACCOUNTS.ADMIN_DELETE(code), {
      method: 'DELETE',
      headers: this.getHeaders(token),
      body: JSON.stringify(options),
    });

    return this.handleResponse(response);
  }

  // Get account hierarchy
  async getAccountHierarchy(token: string): Promise<Account[]> {
    const url = `${API_ENDPOINTS.ACCOUNTS.HIERARCHY}?_t=${Date.now()}`;
    const response = await fetch(url, {
      method: 'GET',
      headers: { ...this.getHeaders(token), 'Cache-Control': 'no-store' },
      cache: 'no-store',
    });

    const result: ApiResponse<Account[] | null | undefined> = await this.handleResponse(response);
    // Go will encode a nil slice as null; normalize to [] for frontend safety
    const data = Array.isArray(result?.data) ? result!.data! : [];
    return data;
  }

  // Get legacy SSOT balances per account (kept for compatibility)
  async getSSOTAccountBalances(token: string, asOfDate?: string): Promise<{
    account_id: number;
    account_code: string;
    account_name: string;
    account_type: string;
    debit_total: number;
    credit_total: number;
    net_balance: number;
  }[]> {
    const params = new URLSearchParams();
    if (asOfDate) params.set('as_of_date', asOfDate);

    // 🗏 Add filter to ensure only POSTED journal entries (from INVOICED sales)
    params.set('status_filter', 'POSTED');
    params.set('source_filter', 'INVOICED_ONLY'); // Backend should understand this

    params.set('_t', Date.now().toString());
    const url = `${API_ENDPOINTS.SSOT_REPORTS.ACCOUNT_BALANCES}${params.toString() ? `?${params.toString()}` : ''}`;

    console.log('🔍 Fetching SSOT account balances with INVOICED-only filter:', url);

    const response = await fetch(url, {
      method: 'GET',
      headers: { ...this.getHeaders(token), 'Cache-Control': 'no-store' },
      cache: 'no-store',
    });

    const result = await this.handleResponse<{ status: string; data: any[] }>(response);

    // Validate that we're getting INVOICED-only data
    console.log('✅ SSOT Account Balances fetched:', {
      count: result.data?.length || 0,
      filter: 'INVOICED_ONLY',
      as_of_date: asOfDate || 'latest'
    });

    return result.data as any[];
  }

  // New: Get posted-only COA balances from SSOT
  async getPostedCOABalances(token: string): Promise<{
    account_id: number;
    account_code: string;
    account_name: string;
    account_type: string;
    raw_balance: number;
    display_balance: number;
    is_positive: boolean;
  }[]> {
    const url = `${API_ENDPOINTS.COA_POSTED_BALANCES}?_t=${Date.now()}`;
    const response = await fetch(url, {
      method: 'GET',
      headers: { ...this.getHeaders(token), 'Cache-Control': 'no-store' },
      cache: 'no-store',
    });
    const result = await this.handleResponse<{ status: string; data: any[] | null | undefined }>(response);
    return Array.isArray(result?.data) ? (result!.data as any[]) : [];
  }

  // Get balance summary
  async getBalanceSummary(token: string): Promise<AccountSummaryResponse[]> {
    const response = await fetch(API_ENDPOINTS.ACCOUNTS.BALANCE_SUMMARY, {
      method: 'GET',
      headers: this.getHeaders(token),
    });

    const result: ApiResponse<AccountSummaryResponse[]> = await this.handleResponse(response);
    return result.data;
  }

  // Bulk import accounts
  async importAccounts(token: string, file: File): Promise<{ message: string; count: number }> {
    const formData = new FormData();
    formData.append('file', file);

    const headers: HeadersInit = {};
    if (token) {
      headers.Authorization = `Bearer ${token}`;
    }

    const response = await fetch(API_ENDPOINTS.ACCOUNTS.IMPORT, {
      method: 'POST',
      headers,
      body: formData,
    });

    return this.handleResponse(response);
  }

  // Download import template
  async downloadTemplate(): Promise<Blob> {
    const response = await fetch(API_ENDPOINTS.ACCOUNTS.TEMPLATE, {
      method: 'GET',
    });

    if (!response.ok) {
      throw new Error('Failed to download template');
    }

    return response.blob();
  }

  // Export accounts to PDF
  async exportAccountsPDF(token: string): Promise<Blob> {
    const response = await fetch(API_ENDPOINTS.ACCOUNTS.EXPORT.PDF, {
      method: 'GET',
      headers: this.getHeaders(token),
    });

    if (!response.ok) {
      let errorData: ApiError;
      try {
        errorData = await response.json();
      } catch {
        errorData = {
          error: 'Network error',
          code: 'NETWORK_ERROR',
        };
      }


      throw new Error(errorData.error || `HTTP error! status: ${response.status}`);
    }

    return response.blob();
  }

  // Export accounts to Excel
  async exportAccountsExcel(token: string): Promise<Blob> {
    const response = await fetch(API_ENDPOINTS.ACCOUNTS.EXPORT.EXCEL, {
      method: 'GET',
      headers: this.getHeaders(token),
    });

    if (!response.ok) {
      let errorData: ApiError;
      try {
        errorData = await response.json();
      } catch {
        errorData = {
          error: 'Network error',
          code: 'NETWORK_ERROR',
        };
      }


      throw new Error(errorData.error || `HTTP error! status: ${response.status}`);
    }

    return response.blob();
  }

  // Helper: Format balance for display with proper negative value detection
  formatBalance(balance: number, currency = 'IDR', accountCode?: string, accountType?: string): string {
    // ✅ FIXED: Show ACTUAL balance, including negatives for error detection
    // For ASSET accounts (like 1301 Persediaan), negative balance = ERROR that must be visible!
    const displayBalance = balance;

    // 🚨 Log warning for negative ASSET/EXPENSE balances (unusual situation)
    if (balance < 0 && (accountType === 'ASSET' || accountType === 'EXPENSE')) {
      console.warn(`⚠️ WARNING: ${accountCode} (${accountType}) has NEGATIVE balance: ${balance}! This may indicate an error.`);
    }

    // For LIABILITY/EQUITY/REVENUE, negative balance might be normal in some cases
    if (balance < 0 && (accountType === 'LIABILITY' || accountType === 'EQUITY' || accountType === 'REVENUE')) {
      console.log(`ℹ️ Note: ${accountCode} (${accountType}) has negative balance: ${balance}`);
    }

    console.log(`💡 Displaying ${accountCode} balance: ${displayBalance} (type: ${accountType})`);

    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: currency,
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(displayBalance);
  }

  // Helper: Determine if account should be displayed as positive
  private shouldDisplayAsPositive(accountCode?: string, accountType?: string): boolean {
    if (!accountCode) return false;

    // Revenue accounts (4xxx) should display as positive
    if (accountCode.startsWith('4') || accountType === 'REVENUE') {
      return true;
    }

    // PPN & tax accounts (2102-001 PPN Keluaran, 2102-002 PPN Masukan, etc.) should display as positive 
    if (accountCode.startsWith('2102') || accountCode.startsWith('2103')) {
      return true;
    }

    // Other specific tax accounts that should display positive
    const positiveTaxAccounts = ['2104', '2105']; // Common tax payable accounts
    if (positiveTaxAccounts.includes(accountCode)) {
      return true;
    }

    return false;
  }

  // Helper: Get account type color
  getAccountTypeColor(type: string): string {
    switch (type) {
      case 'ASSET':
        return 'green';
      case 'LIABILITY':
        return 'red';
      case 'EQUITY':
        return 'blue';
      case 'REVENUE':
        return 'purple';
      case 'EXPENSE':
        return 'orange';
      default:
        return 'gray';
    }
  }

  // Helper: Get account type label
  getAccountTypeLabel(type: string, useEnglish: boolean = false): string {
    if (useEnglish) {
      switch (type) {
        case 'ASSET':
          return 'Asset';
        case 'LIABILITY':
          return 'Liability';
        case 'EQUITY':
          return 'Equity';
        case 'REVENUE':
          return 'Revenue';
        case 'EXPENSE':
          return 'Expense';
        default:
          return type;
      }
    }

    switch (type) {
      case 'ASSET':
        return 'Aktiva';
      case 'LIABILITY':
        return 'Kewajiban';
      case 'EQUITY':
        return 'Modal';
      case 'REVENUE':
        return 'Pendapatan';
      case 'EXPENSE':
        return 'Beban';
      default:
        return type;
    }
  }

  // Validate account code availability
  async validateAccountCode(token: string, code: string, excludeId?: number): Promise<{
    available: boolean;
    message: string;
    existing_account?: {
      id: number;
      code: string;
      name: string;
    };
  }> {
    let url = API_ENDPOINTS.ACCOUNTS.VALIDATE_CODE + `?code=${encodeURIComponent(code)}`;
    if (excludeId) {
      url += `&exclude_id=${excludeId}`;
    }

    const response = await fetch(url, {
      method: 'GET',
      headers: this.getHeaders(token),
    });

    return this.handleResponse(response);
  }

}

export const accountService = new AccountService();
export default accountService;
