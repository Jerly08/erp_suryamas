import { API_ENDPOINTS } from '@/config/api';
import { getAuthHeaders } from '@/utils/authTokenUtils';

// SSOT Journal Entry Structure (aligned with backend)
export interface SSOTJournalEntry {
  id: number;
  entry_number: string;
  source_type: string;
  entry_date: string;
  description: string;
  reference?: string;
  notes?: string;
  total_debit: number;
  total_credit: number;
  status: 'DRAFT' | 'POSTED' | 'REVERSED';
  is_balanced: boolean;
  is_auto_generated: boolean;
  created_by: number;
  posted_by?: number;
  posted_at?: string;
  reversed_by?: number;
  reversed_at?: string;
  created_at: string;
  updated_at: string;
  lines?: SSOTJournalLine[];
  creator?: {
    id: number;
    username: string;
    first_name: string;
    last_name: string;
  };
  poster?: {
    id: number;
    username: string;
    first_name: string;
    last_name: string;
  };
}

export interface SSOTJournalLine {
  id: number;
  journal_id: number;
  account_id: number;
  line_number: number;
  description: string;
  debit_amount: number;
  credit_amount: number;
  quantity?: number;
  unit_price?: number;
  created_at: string;
  updated_at: string;
  account?: {
    id: number;
    code: string;
    name: string;
    type: string;
  };
}

export interface CreateJournalRequest {
  entry_date: string;
  description: string;
  reference?: string;
  notes?: string;
  lines: Array<{
    account_id: number;
    description: string;
    debit_amount?: number;
    credit_amount?: number;
    quantity?: number;
    unit_price?: number;
  }>;
}

export interface UpdateJournalRequest extends CreateJournalRequest {
  // Same structure as create request
}

class SSOTJournalService {
  private getAuthHeaders() {
    // Use centralized token utility for consistency across the application
    return getAuthHeaders();
  }

  private buildQueryString(params: Record<string, any>): string {
    const searchParams = new URLSearchParams();

    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== null && value !== '' && value !== 'ALL') {
        searchParams.append(key, value.toString());
      }
    });

    return searchParams.toString();
  }

  // Get all journal entries
  async getJournalEntries(params: {
    start_date?: string;
    end_date?: string;
    status?: string;
    source_type?: string;
    page?: number;
    limit?: number;
    search?: string;
  } = {}): Promise<{
    data: SSOTJournalEntry[];
    total: number;
    page: number;
    limit: number;
    totalPages: number;
  }> {
    const queryParams: Record<string, any> = { ...params };
    // Map frontend naming to backend expectation
    if (params.start_date) queryParams.date_from = params.start_date;
    if (params.end_date) queryParams.date_to = params.end_date;
    // Remove old keys if we want to be clean, or keep them if needed by legacy
    // But backend UnifiedJournalController looks explicitly for "date_from"

    // Also AccountID? Backend `JournalFilters` has `AccountID`. 
    // Frontend doesn't send account_id in this method usually (Journal View doesn't filter by account).

    const searchParams = new URLSearchParams();

    Object.entries(queryParams).forEach(([key, value]) => {
      if (value !== undefined && value !== null && value !== '' && value !== 'ALL') {
        // Skip mapped keys if we want to avoid double sending
        if (key === 'start_date' || key === 'end_date') return;
        searchParams.append(key, value.toString());
      }
    });

    const queryString = searchParams.toString();
    const url = API_ENDPOINTS.JOURNALS.LIST + (queryString ? '?' + queryString : '');

    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      throw new Error(`Failed to fetch journal entries: ${response.statusText}`);
    }

    return await response.json();
  }

  // Get specific journal entry
  async getJournalEntry(id: number): Promise<SSOTJournalEntry> {
    const response = await fetch(API_ENDPOINTS.JOURNALS.GET_BY_ID(id), {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      throw new Error(`Failed to fetch journal entry: ${response.statusText}`);
    }

    const result = await response.json();
    return result;
  }

  // Create new journal entry
  async createJournalEntry(data: CreateJournalRequest): Promise<SSOTJournalEntry> {
    const response = await fetch(API_ENDPOINTS.JOURNALS.CREATE, {
      method: 'POST',
      headers: this.getAuthHeaders(),
      body: JSON.stringify(data),
    });

    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || error.message || 'Failed to create journal entry');
    }

    const result = await response.json();
    return result.data;
  }

  // Update journal entry
  async updateJournalEntry(id: number, data: UpdateJournalRequest): Promise<SSOTJournalEntry> {
    const response = await fetch(API_ENDPOINTS.JOURNALS.UPDATE(id), {
      method: 'PUT',
      headers: this.getAuthHeaders(),
      body: JSON.stringify(data),
    });

    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.message || 'Failed to update journal entry');
    }

    const result = await response.json();
    return result.data;
  }

  // Delete journal entry
  async deleteJournalEntry(id: number): Promise<void> {
    const response = await fetch(API_ENDPOINTS.JOURNALS.DELETE(id), {
      method: 'DELETE',
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.message || 'Failed to delete journal entry');
    }
  }

  // Post journal entry
  async postJournalEntry(id: number): Promise<SSOTJournalEntry> {
    const response = await fetch(API_ENDPOINTS.JOURNALS.POST(id), {
      method: 'PUT',
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.message || 'Failed to post journal entry');
    }

    const result = await response.json();
    return result.data;
  }

  // Reverse journal entry
  async reverseJournalEntry(id: number, reason?: string): Promise<SSOTJournalEntry> {
    const response = await fetch(API_ENDPOINTS.JOURNALS.REVERSE(id), {
      method: 'POST',
      headers: this.getAuthHeaders(),
      body: JSON.stringify({ description: reason }),
    });

    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.message || 'Failed to reverse journal entry');
    }

    const result = await response.json();
    return result.data;
  }

  // Get journal summary
  async getJournalSummary(params: {
    start_date?: string;
    end_date?: string;
    status?: string;
  } = {}): Promise<{
    total_entries: number;
    total_debit: number;
    total_credit: number;
    posted_entries: number;
    draft_entries: number;
    reversed_entries: number;
  }> {
    const queryString = this.buildQueryString(params);
    const url = API_ENDPOINTS.JOURNALS.SUMMARY + (queryString ? '?' + queryString : '');

    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      throw new Error('Failed to fetch journal summary');
    }

    const result = await response.json();
    return result.data;
  }

  // Get account balances
  async getAccountBalances(): Promise<Array<{
    account_id: number;
    account_code: string;
    account_name: string;
    debit_balance: number;
    credit_balance: number;
    balance: number;
    last_updated: string;
  }>> {
    const response = await fetch(API_ENDPOINTS.JOURNALS.ACCOUNT_BALANCES, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      throw new Error('Failed to fetch account balances');
    }

    const result = await response.json();
    return result.data;
  }

  // Refresh materialized view for account balances
  async refreshAccountBalances(): Promise<{ message: string; updated_at: string }> {
    const response = await fetch(API_ENDPOINTS.JOURNALS.REFRESH_ACCOUNT_BALANCES, {
      method: 'POST',
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      throw new Error('Failed to refresh account balances');
    }

    const result = await response.json();
    return result.data;
  }

  // Legacy compatibility method - converts old journal entry calls to SSOT
  async getJournalEntriesLegacy(params: any): Promise<any> {
    console.warn('🔄 Using legacy compatibility method - please update to getJournalEntries()');

    // Map legacy parameters to SSOT parameters
    const ssotParams = {
      start_date: params.start_date,
      end_date: params.end_date,
      status: params.status || 'POSTED',
      page: params.page || 1,
      limit: params.limit || 100,
    };

    const result = await this.getJournalEntries(ssotParams);

    // Convert SSOT format to legacy format for backward compatibility
    return {
      data: result.data.map(entry => ({
        id: entry.id,
        code: entry.entry_number,
        description: entry.description,
        reference: entry.reference,
        reference_type: entry.source_type,
        entry_date: entry.entry_date,
        status: entry.status,
        total_debit: entry.total_debit,
        total_credit: entry.total_credit,
        is_balanced: entry.is_balanced,
        journal_lines: entry.lines
      })),
      total: result.total
    };
  }
}

export const ssotJournalService = new SSOTJournalService();
export default ssotJournalService;
