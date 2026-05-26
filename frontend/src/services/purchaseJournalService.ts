import { API_V1_BASE } from '@/config/api';
import { SSOTJournalEntry } from './ssotJournalService';
import { getAuthHeaders } from '../utils/authTokenUtils';

export interface PurchaseJournalData {
  purchase_id: number;
  journal_entries: SSOTJournalEntry[];
  count: number;
}

export interface JournalEntryLine {
  id: number;
  account_id: number;
  account?: {
    id: number;
    code: string;
    name: string;
    type: string;
  };
  description: string;
  debit_amount: number;
  credit_amount: number;
  line_number: number;
}

export type JournalEntryWithDetails = Omit<SSOTJournalEntry, 'lines'> & {
  lines?: JournalEntryLine[];
};

class PurchaseJournalService {
  private getAuthHeaders() {
    // Use centralized token utility for consistency across the application
    return getAuthHeaders();
  }

  // Get journal entries for a specific purchase
  async getPurchaseJournalEntries(purchaseId: number): Promise<PurchaseJournalData> {
    const response = await fetch(`${API_V1_BASE}/purchases/${purchaseId}/journal-entries`, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      throw new Error(`Failed to fetch purchase journal entries: ${response.statusText}`);
    }

    return await response.json();
  }

  async backfillPurchaseJournalEntries(purchaseId: number): Promise<PurchaseJournalData> {
    const response = await fetch(`${API_V1_BASE}/purchases/${purchaseId}/journal-entries/backfill`, {
      method: 'POST',
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      throw new Error(error.error || error.message || `Failed to backfill purchase journal entries: ${response.statusText}`);
    }

    return await response.json();
  }

  // Get detailed information about a specific journal entry
  async getJournalEntryDetails(journalId: number): Promise<JournalEntryWithDetails> {
    const response = await fetch(`${API_V1_BASE}/journals/${journalId}`, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      throw new Error(`Failed to fetch journal entry details: ${response.statusText}`);
    }

    const result = await response.json();
    return result.data || result;
  }

  // Get account balances affected by purchase
  async getAffectedAccountBalances(accountIds: number[]): Promise<any[]> {
    const response = await fetch(`${API_V1_BASE}/journals/account-balances`, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      throw new Error('Failed to fetch account balances');
    }

    const result = await response.json();
    const allBalances = result.data || [];

    // Filter to only affected accounts
    return allBalances.filter((balance: any) => 
      accountIds.includes(balance.account_id)
    );
  }

  // Reverse a journal entry (for admin/finance users)
  async reverseJournalEntry(journalId: number, reason: string): Promise<SSOTJournalEntry> {
    const response = await fetch(`${API_V1_BASE}/journals/${journalId}/reverse`, {
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

  // Get journal entries summary for multiple purchases
  async getPurchaseJournalSummary(purchaseIds: number[]): Promise<{
    total_entries: number;
    total_debit: number;
    total_credit: number;
    entries_by_type: Record<string, number>;
  }> {
    // This would typically be a batch API call, but for now we'll aggregate client-side
    const summaryData = {
      total_entries: 0,
      total_debit: 0,
      total_credit: 0,
      entries_by_type: {} as Record<string, number>
    };

    for (const purchaseId of purchaseIds) {
      try {
        const purchaseJournals = await this.getPurchaseJournalEntries(purchaseId);
        summaryData.total_entries += purchaseJournals.count;
        
        for (const entry of purchaseJournals.journal_entries) {
          summaryData.total_debit += entry.total_debit;
          summaryData.total_credit += entry.total_credit;
          
          const sourceType = entry.source_type || 'UNKNOWN';
          summaryData.entries_by_type[sourceType] = (summaryData.entries_by_type[sourceType] || 0) + 1;
        }
      } catch (error) {
        console.warn(`Failed to get journal entries for purchase ${purchaseId}:`, error);
      }
    }

    return summaryData;
  }

  // Check if journal entries exist for a purchase
  async hasPurchaseJournalEntries(purchaseId: number): Promise<boolean> {
    try {
      const journalData = await this.getPurchaseJournalEntries(purchaseId);
      return journalData.count > 0;
    } catch (error) {
      console.warn(`Error checking journal entries for purchase ${purchaseId}:`, error);
      return false;
    }
  }

  // Format journal entry for display
  formatJournalEntryForDisplay(entry: SSOTJournalEntry): {
    title: string;
    subtitle: string;
    amount: string;
    status: string;
    date: string;
    type: string;
  } {
    const formatCurrency = (amount: number) => {
      return new Intl.NumberFormat('id-ID', {
        style: 'currency',
        currency: 'IDR',
        minimumFractionDigits: 0,
        maximumFractionDigits: 0
      }).format(amount);
    };

    const formatDate = (dateString: string) => {
      return new Date(dateString).toLocaleDateString('id-ID', {
        year: 'numeric',
        month: 'short',
        day: 'numeric'
      });
    };

    const getStatusColor = (status: string) => {
      switch (status.toUpperCase()) {
        case 'POSTED': return 'green';
        case 'DRAFT': return 'yellow';
        case 'REVERSED': return 'red';
        default: return 'gray';
      }
    };

    const getTypeLabel = (sourceType: string) => {
      switch (sourceType) {
        case 'PURCHASE': return 'Purchase Order';
        case 'PAYMENT': return 'Payment';
        case 'MANUAL': return 'Manual Entry';
        case 'ADJUSTMENT': return 'Adjustment';
        default: return sourceType;
      }
    };

    return {
      title: entry.entry_number || `Journal Entry ${entry.id}`,
      subtitle: entry.description || 'No description',
      amount: formatCurrency(entry.total_debit),
      status: entry.status || 'UNKNOWN',
      date: formatDate(entry.entry_date),
      type: getTypeLabel(entry.source_type),
    };
  }

  // Get drill-down data for accounting analysis
  async getJournalDrilldown(params: {
    purchase_id?: number;
    account_ids?: number[];
    start_date?: string;
    end_date?: string;
    source_type?: string;
  }): Promise<any> {
    const response = await fetch(`${API_V1_BASE}/journal-drilldown`, {
      method: 'POST',
      headers: this.getAuthHeaders(),
      body: JSON.stringify({
        ...params,
        source_type: params.source_type || 'PURCHASE',
      }),
    });

    if (!response.ok) {
      throw new Error('Failed to get journal drilldown data');
    }

    return await response.json();
  }
}

export const purchaseJournalService = new PurchaseJournalService();
export default purchaseJournalService;
