import { API_V1_BASE } from '@/config/api';
import { ReportParameters } from './reportService';
import { getAuthHeaders } from '../utils/authTokenUtils';

export interface JournalEntry {
  id: number;
  code: string;
  description: string;
  reference: string;
  reference_type: string;
  entry_date: string;
  status: string;
  total_debit: number;
  total_credit: number;
  is_balanced: boolean;
  account?: {
    id: number;
    code: string;
    name: string;
    account_type?: string;
  };
  journal_lines?: JournalLine[];
}

export interface JournalLine {
  id: number;
  account_id: number;
  debit_amount: number;
  credit_amount: number;
  description: string;
  account?: {
    id: number;
    code: string;
    name: string;
    account_type: string;
  };
}

export interface Account {
  id: number;
  code: string;
  name: string;
  account_type: string;
  is_header: boolean;
  balance?: number;
}

export interface JournalBasedReportData {
  entries: JournalEntry[];
  accounts: Account[];
  totalEntries: number;
  period: {
    start_date: string;
    end_date: string;
  };
  summary: {
    total_debit: number;
    total_credit: number;
    balanced_entries: number;
    unbalanced_entries: number;
  };
}

class JournalIntegrationService {
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

  // Fetch journal entries with enhanced filtering
  async getJournalEntries(params: {
    start_date?: string;
    end_date?: string;
    status?: string;
    reference_type?: string;
    account_ids?: number[];
    limit?: number;
  }): Promise<JournalBasedReportData> {
    const queryParams: any = {
      start_date: params.start_date,
      end_date: params.end_date,
      status: params.status || 'POSTED',
      reference_type: params.reference_type,
      limit: params.limit || 10000,
      page: 1
    };

    const queryString = this.buildQueryString(queryParams);
    const url = `${API_V1_BASE}/journals${queryString ? '?' + queryString : ''}`;

    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      throw new Error('Failed to fetch journal entries');
    }

    const result = await response.json();

    // Also fetch accounts for categorization
    const accounts = await this.getAccounts();

    return {
      entries: result.data || [],
      accounts,
      totalEntries: result.total || 0,
      period: {
        start_date: params.start_date || '',
        end_date: params.end_date || ''
      },
      summary: this.calculateSummary(result.data || [])
    };
  }

  // Fetch all accounts for categorization
  async getAccounts(): Promise<Account[]> {
    const response = await fetch(`${API_V1_BASE}/accounts`, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      throw new Error('Failed to fetch accounts');
    }

    const result = await response.json();
    return result.data || [];
  }

  // Generate Balance Sheet from Journal Entries
  async generateBalanceSheetFromJournals(params: ReportParameters): Promise<any> {
    const journalData = await this.getJournalEntries({
      start_date: '2000-01-01', // Get all entries to calculate current balances
      end_date: params.as_of_date || new Date().toISOString().split('T')[0],
      status: 'POSTED'
    });

    return this.convertJournalsToBalanceSheet(journalData, params.as_of_date);
  }

  // Generate Cash Flow from Journal Entries
  async generateCashFlowFromJournals(params: ReportParameters): Promise<any> {
    const journalData = await this.getJournalEntries({
      start_date: params.start_date,
      end_date: params.end_date,
      status: 'POSTED'
    });

    return this.convertJournalsToCashFlow(journalData);
  }

  // Generate Trial Balance from Journal Entries
  async generateTrialBalanceFromJournals(params: ReportParameters): Promise<any> {
    const journalData = await this.getJournalEntries({
      start_date: '2000-01-01', // Get all entries to calculate current balances
      end_date: params.as_of_date || new Date().toISOString().split('T')[0],
      status: 'POSTED'
    });

    return this.convertJournalsToTrialBalance(journalData, params.as_of_date);
  }

  // Convert journal entries to Balance Sheet format
  private convertJournalsToBalanceSheet(journalData: JournalBasedReportData, asOfDate?: string): any {
    const accountBalances = this.calculateAccountBalances(journalData.entries, journalData.accounts);

    const assets = this.categorizeAccounts(accountBalances, ['1']); // 1xxx accounts
    const liabilities = this.categorizeAccounts(accountBalances, ['2']); // 2xxx accounts  
    const equity = this.categorizeAccounts(accountBalances, ['3']); // 3xxx accounts

    return {
      title: 'Balance Sheet (from Journal Entries)',
      period: `As of ${asOfDate || new Date().toLocaleDateString('id-ID')}`,
      sections: [
        {
          name: 'ASSETS',
          items: assets,
          total: assets.reduce((sum, item) => sum + item.amount, 0)
        },
        {
          name: 'LIABILITIES',
          items: liabilities,
          total: liabilities.reduce((sum, item) => sum + item.amount, 0)
        },
        {
          name: 'EQUITY',
          items: equity,
          total: equity.reduce((sum, item) => sum + item.amount, 0)
        }
      ],
      enhanced: true,
      generated_at: new Date().toISOString()
    };
  }

  // Convert journal entries to Cash Flow format
  private convertJournalsToCashFlow(journalData: JournalBasedReportData): any {
    const cashAccounts = ['1101', '1101-001', '1101-002', '1101-003', '1101-004']; // Cash and bank accounts (all under 1101 KAS DAN SETARA KAS)

    const operatingActivities = this.categorizeCashFlowActivities(
      journalData.entries,
      ['SALE', 'PURCHASE', 'PAYMENT'], // Operating activities
      cashAccounts
    );

    const investingActivities = this.categorizeCashFlowActivities(
      journalData.entries,
      ['ASSET_PURCHASE', 'ASSET_SALE'], // Investing activities 
      cashAccounts
    );

    const financingActivities = this.categorizeCashFlowActivities(
      journalData.entries,
      ['LOAN', 'EQUITY', 'DIVIDEND'], // Financing activities
      cashAccounts
    );

    return {
      title: 'Cash Flow Statement (from Journal Entries)',
      period: `${new Date(journalData.period.start_date).toLocaleDateString('id-ID')} - ${new Date(journalData.period.end_date).toLocaleDateString('id-ID')}`,
      sections: [
        {
          name: 'OPERATING ACTIVITIES',
          items: operatingActivities,
          total: operatingActivities.reduce((sum, item) => sum + item.amount, 0)
        },
        {
          name: 'INVESTING ACTIVITIES',
          items: investingActivities,
          total: investingActivities.reduce((sum, item) => sum + item.amount, 0)
        },
        {
          name: 'FINANCING ACTIVITIES',
          items: financingActivities,
          total: financingActivities.reduce((sum, item) => sum + item.amount, 0)
        }
      ],
      enhanced: true,
      generated_at: new Date().toISOString()
    };
  }

  // Convert journal entries to Trial Balance format
  private convertJournalsToTrialBalance(journalData: JournalBasedReportData, asOfDate?: string): any {
    const accountBalances = this.calculateAccountBalances(journalData.entries, journalData.accounts);

    const accounts = accountBalances
      .filter(account => account.debit_balance !== 0 || account.credit_balance !== 0)
      .map(account => ({
        name: `${account.code} - ${account.name}`,
        debit: account.debit_balance,
        credit: account.credit_balance,
        balance: account.debit_balance - account.credit_balance
      }));

    const totalDebits = accounts.reduce((sum, acc) => sum + acc.debit, 0);
    const totalCredits = accounts.reduce((sum, acc) => sum + acc.credit, 0);

    return {
      title: 'Trial Balance (from Journal Entries)',
      period: `As of ${asOfDate || new Date().toLocaleDateString('id-ID')}`,
      sections: [
        {
          name: 'ACCOUNTS',
          items: accounts,
          total: totalDebits,
          totalDebits,
          totalCredits,
          isBalanced: Math.abs(totalDebits - totalCredits) < 0.01
        }
      ],
      isBalanced: Math.abs(totalDebits - totalCredits) < 0.01,
      totalDebits,
      totalCredits,
      hasData: accounts.length > 0,
      enhanced: true,
      generated_at: new Date().toISOString()
    };
  }

  // Calculate account balances from journal entries
  private calculateAccountBalances(entries: JournalEntry[], accounts: Account[]): any[] {
    const balanceMap = new Map<string, {
      code: string;
      name: string;
      debit_balance: number;
      credit_balance: number;
    }>();

    // Initialize accounts
    accounts.forEach(account => {
      if (!account.is_header) {
        balanceMap.set(account.code, {
          code: account.code,
          name: account.name,
          debit_balance: 0,
          credit_balance: 0
        });
      }
    });

    // Process journal entries
    entries.forEach(entry => {
      if (entry.journal_lines && entry.journal_lines.length > 0) {
        // Use journal lines for detailed account-level data
        entry.journal_lines.forEach(line => {
          const account = line.account;
          if (account && balanceMap.has(account.code)) {
            const balance = balanceMap.get(account.code)!;
            balance.debit_balance += line.debit_amount || 0;
            balance.credit_balance += line.credit_amount || 0;
          }
        });
      } else if (entry.account) {
        // Fallback to entry-level account data
        const balance = balanceMap.get(entry.account.code);
        if (balance) {
          balance.debit_balance += entry.total_debit || 0;
          balance.credit_balance += entry.total_credit || 0;
        }
      }
    });

    return Array.from(balanceMap.values());
  }

  // Categorize accounts by account code prefixes
  private categorizeAccounts(accountBalances: any[], prefixes: string[]): any[] {
    return accountBalances
      .filter(account => prefixes.some(prefix => account.code.startsWith(prefix)))
      .filter(account => account.debit_balance !== 0 || account.credit_balance !== 0)
      .map(account => ({
        name: `${account.code} - ${account.name}`,
        amount: Math.abs(account.debit_balance - account.credit_balance),
        debit: account.debit_balance,
        credit: account.credit_balance
      }));
  }

  // Categorize cash flow activities
  private categorizeCashFlowActivities(entries: JournalEntry[], referenceTypes: string[], cashAccounts: string[]): any[] {
    return entries
      .filter(entry => referenceTypes.includes(entry.reference_type))
      .filter(entry => entry.account && cashAccounts.includes(entry.account.code))
      .map(entry => ({
        name: entry.description,
        amount: entry.total_debit - entry.total_credit,
        reference: entry.reference,
        date: entry.entry_date
      }));
  }

  // Calculate summary statistics
  private calculateSummary(entries: JournalEntry[]): any {
    const totalDebit = entries.reduce((sum, entry) => sum + (entry.total_debit || 0), 0);
    const totalCredit = entries.reduce((sum, entry) => sum + (entry.total_credit || 0), 0);
    const balancedEntries = entries.filter(entry => entry.is_balanced).length;

    return {
      total_debit: totalDebit,
      total_credit: totalCredit,
      balanced_entries: balancedEntries,
      unbalanced_entries: entries.length - balancedEntries
    };
  }

  // Get journal entries for specific P&L line items (for drilldown)
  async getJournalEntriesForPLLine(params: {
    start_date: string;
    end_date: string;
    line_item: string;
    account_codes?: string;
    page?: number;
    limit?: number;
  }): Promise<any> {
    const queryString = this.buildQueryString(params);
    const url = `${API_V1_BASE}/reports/enhanced/journal/pl-line${queryString ? '?' + queryString : ''}`;

    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      throw new Error('Failed to fetch P&L line item journal entries');
    }

    const result = await response.json();
    return result.data;
  }
}

export const journalIntegrationService = new JournalIntegrationService();
