import { API_V1_BASE } from '@/config/api';
import { ReportParameters } from './reportService';
import { getAuthHeaders } from '../utils/authTokenUtils';

export interface JournalEntryData {
  id: number;
  code: string;
  description: string;
  reference: string;
  reference_type: string;
  entry_date: string;
  status: string;
  total_debit: number;
  total_credit: number;
  account?: {
    id: number;
    code: string;
    name: string;
    account_type: string;
  };
}

export interface EnhancedPLFromJournals {
  company: {
    name: string;
    period: string;
  };
  revenue: {
    total_revenue: number;
    items: Array<{
      account_code: string;
      account_name: string;
      amount: number;
      journal_entries: JournalEntryData[];
    }>;
  };
  cost_of_goods_sold: {
    total_cogs: number;
    items: Array<{
      account_code: string;
      account_name: string;
      amount: number;
      journal_entries: JournalEntryData[];
    }>;
  };
  gross_profit: number;
  gross_profit_margin: number;
  operating_expenses: {
    total_opex: number;
    administrative: {
      subtotal: number;
      items: Array<{
        account_code: string;
        account_name: string;
        amount: number;
        journal_entries: JournalEntryData[];
      }>;
    };
    selling_marketing: {
      subtotal: number;
      items: Array<{
        account_code: string;
        account_name: string;
        amount: number;
        journal_entries: JournalEntryData[];
      }>;
    };
    general: {
      subtotal: number;
      items: Array<{
        account_code: string;
        account_name: string;
        amount: number;
        journal_entries: JournalEntryData[];
      }>;
    };
  };
  operating_income: number;
  operating_margin: number;
  ebitda: number;
  ebitda_margin: number;
  other_income: number;
  other_expenses: number;
  income_before_tax: number;
  tax_expense: number;
  net_income: number;
  net_income_margin: number;
  start_date: string;
  end_date: string;
  generated_at: string;
  enhanced: true;
}

class EnhancedPLService {
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

  // Get journal entries for the period
  private async getJournalEntries(params: ReportParameters): Promise<{entries: JournalEntryData[], total: number}> {
    const journalParams = {
      start_date: params.start_date,
      end_date: params.end_date,
      status: 'POSTED', // Only posted entries
      page: 1,
      limit: 10000 // Get all entries for comprehensive analysis
    };

    const queryString = this.buildQueryString(journalParams);
    const url = `${API_V1_BASE}/journals${queryString ? '?' + queryString : ''}`;
    
    const response = await fetch(url, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      throw new Error('Failed to fetch journal entries for P&L analysis');
    }

    const result = await response.json();
    return {
      entries: result.data || [],
      total: result.total || 0
    };
  }

  // Get account details to categorize entries
  private async getAccounts(): Promise<Array<{id: number, code: string, name: string, account_type: string}>> {
    const response = await fetch(`${API_V1_BASE}/accounts`, {
      headers: this.getAuthHeaders(),
    });

    if (!response.ok) {
      throw new Error('Failed to fetch accounts for P&L categorization');
    }

    const result = await response.json();
    return result.data || [];
  }

  // Categorize journal entries into P&L components
  private categorizeJournalEntries(entries: JournalEntryData[], accounts: Array<{id: number, code: string, name: string, account_type: string}>): EnhancedPLFromJournals {
    const accountMap = new Map(accounts.map(acc => [acc.id, acc]));
    
    console.log('🔍 DEBUG: Categorizing journal entries');
    console.log('📊 Total entries:', entries.length);
    console.log('🏦 Total accounts:', accounts.length);
    
    // Initialize P&L structure
    const plData: EnhancedPLFromJournals = {
      company: {
        name: 'PT. Accounting System',
        period: ''
      },
      revenue: {
        total_revenue: 0,
        items: []
      },
      cost_of_goods_sold: {
        total_cogs: 0,
        items: []
      },
      gross_profit: 0,
      gross_profit_margin: 0,
      operating_expenses: {
        total_opex: 0,
        administrative: { subtotal: 0, items: [] },
        selling_marketing: { subtotal: 0, items: [] },
        general: { subtotal: 0, items: [] }
      },
      operating_income: 0,
      operating_margin: 0,
      ebitda: 0,
      ebitda_margin: 0,
      other_income: 0,
      other_expenses: 0,
      income_before_tax: 0,
      tax_expense: 0,
      net_income: 0,
      net_income_margin: 0,
      start_date: '',
      end_date: '',
      generated_at: new Date().toISOString(),
      enhanced: true
    };

    // Group entries by account and type
    const accountGroups = new Map<string, {
      account: any;
      entries: JournalEntryData[];
      total_debit: number;
      total_credit: number;
    }>();

    entries.forEach((entry, index) => {
      if (index < 5) {
        console.log(`🔍 Entry ${index}:`, {
          id: entry.id,
          code: entry.code,
          description: entry.description,
          total_debit: entry.total_debit,
          total_credit: entry.total_credit,
          account: entry.account
        });
      }
      
      const account = entry.account;
      if (!account) {
        if (index < 5) console.log(`⚠️ No account found for entry ${entry.id}`);
        return;
      }

      const key = `${account.code}-${account.name}`;
      if (!accountGroups.has(key)) {
        accountGroups.set(key, {
          account,
          entries: [],
          total_debit: 0,
          total_credit: 0
        });
      }

      const group = accountGroups.get(key)!;
      group.entries.push(entry);
      group.total_debit += entry.total_debit || 0;
      group.total_credit += entry.total_credit || 0;
    });
    
    console.log('📋 Account groups found:', accountGroups.size);
    // Log first few account groups
    let groupCount = 0;
    accountGroups.forEach((group, key) => {
      if (groupCount < 3) {
        console.log(`📊 Group ${groupCount}:`, {
          key,
          account_code: group.account.code,
          account_name: group.account.name,
          entries_count: group.entries.length,
          total_debit: group.total_debit,
          total_credit: group.total_credit
        });
      }
      groupCount++;
    });

    // Additional analysis: Look for revenue/expense patterns in journal entry descriptions
    // This is important because many journal entries might not have direct account associations
    const unlinkedEntries = entries.filter(entry => !entry.account?.id && !entry.account);
    console.log(`🔍 Unlinked entries (no direct account): ${unlinkedEntries.length}`);
    
    // For unlinked entries, try to categorize based on description patterns
    unlinkedEntries.forEach(entry => {
      const desc = entry.description.toLowerCase();
      const ref = (entry.reference || '').toLowerCase();
      const refType = entry.reference_type || '';
      
      // Try to extract account info from description if it mentions account codes
      const accountCodeMatch = entry.description.match(/\b([1-6]\d{3})\b/);
      if (accountCodeMatch) {
        const foundAccount = accounts.find(acc => acc.code === accountCodeMatch[1]);
        if (foundAccount) {
          const key = `${foundAccount.code}-${foundAccount.name}`;
          if (!accountGroups.has(key)) {
            accountGroups.set(key, {
              account: foundAccount,
              entries: [],
              total_debit: 0,
              total_credit: 0
            });
          }
          const group = accountGroups.get(key)!;
          group.entries.push(entry);
          group.total_debit += entry.total_debit || 0;
          group.total_credit += entry.total_credit || 0;
        }
      }
      
      // Handle SALES/REVENUE patterns
      if (refType === 'SALE' || 
          desc.includes('sales') || desc.includes('pendapatan') || 
          desc.includes('revenue') || desc.includes('service') ||
          ref.includes('sales-') || ref.includes('service-')) {
        // This is likely revenue - find a revenue account
        let revenueAccount = accounts.find(acc => acc.code === '4101'); // Preferred sales account
        if (!revenueAccount) {
          revenueAccount = accounts.find(acc => acc.code.startsWith('4')); // Any revenue account
        }
        
        if (revenueAccount) {
          const key = `${revenueAccount.code}-${revenueAccount.name}`;
          if (!accountGroups.has(key)) {
            accountGroups.set(key, {
              account: revenueAccount,
              entries: [],
              total_debit: 0,
              total_credit: 0
            });
          }
          const group = accountGroups.get(key)!;
          group.entries.push(entry);
          group.total_debit += entry.total_debit || 0;
          group.total_credit += entry.total_credit || 0;
          console.log(`🎆 Added revenue entry: ${entry.description} -> ${revenueAccount.code}`);
        }
      }
      
      // Handle COGS patterns
      else if (desc.includes('cogs') || desc.includes('cost of goods') || 
               desc.includes('harga pokok') || ref.includes('cogs-')) {
        // This is COGS - find a COGS account
        let cogsAccount = accounts.find(acc => acc.code === '5101'); // Preferred COGS account
        if (!cogsAccount) {
          cogsAccount = accounts.find(acc => acc.code.startsWith('510')); // Any COGS account
        }
        
        if (cogsAccount) {
          const key = `${cogsAccount.code}-${cogsAccount.name}`;
          if (!accountGroups.has(key)) {
            accountGroups.set(key, {
              account: cogsAccount,
              entries: [],
              total_debit: 0,
              total_credit: 0
            });
          }
          const group = accountGroups.get(key)!;
          group.entries.push(entry);
          group.total_debit += entry.total_debit || 0;
          group.total_credit += entry.total_credit || 0;
          console.log(`🏦 Added COGS entry: ${entry.description} -> ${cogsAccount.code}`);
        }
      }
      
      // Handle OPERATING EXPENSE patterns
      else if (desc.includes('rent') || desc.includes('sewa') || 
               desc.includes('marketing') || desc.includes('advertising') ||
               desc.includes('electricity') || desc.includes('utility') ||
               desc.includes('office') || desc.includes('administrative') ||
               ref.includes('rent-') || ref.includes('marketing-') || ref.includes('utility-')) {
        // This is operating expense - categorize by type
        let expenseAccount;
        
        if (desc.includes('rent') || desc.includes('sewa') || desc.includes('office') || desc.includes('administrative')) {
          // Administrative expense
          expenseAccount = accounts.find(acc => acc.code === '5201') || // Salary/Admin
                          accounts.find(acc => acc.code.startsWith('520')); // Any admin expense
        } else if (desc.includes('marketing') || desc.includes('advertising')) {
          // Marketing expense  
          expenseAccount = accounts.find(acc => acc.code.startsWith('530')); // Marketing expense
        } else {
          // General expense
          expenseAccount = accounts.find(acc => acc.code.startsWith('520')) || // Try admin first
                          accounts.find(acc => acc.code.startsWith('59')); // General expense
        }
        
        if (expenseAccount) {
          const key = `${expenseAccount.code}-${expenseAccount.name}`;
          if (!accountGroups.has(key)) {
            accountGroups.set(key, {
              account: expenseAccount,
              entries: [],
              total_debit: 0,
              total_credit: 0
            });
          }
          const group = accountGroups.get(key)!;
          group.entries.push(entry);
          group.total_debit += entry.total_debit || 0;
          group.total_credit += entry.total_credit || 0;
          console.log(`🏢 Added expense entry: ${entry.description} -> ${expenseAccount.code}`);
        }
      }
      
      // Handle PURCHASE entries - these affect COGS or Expenses
      if (refType === 'PURCHASE' && desc.includes('purchase')) {
        // Look for account codes in description or try to categorize as expense
        if (desc.includes('fixed assets') || desc.includes('peralatan') || desc.includes('kendaraan')) {
          // This is asset purchase, not expense - skip for P&L
          return;
        }
        
        // If it's a regular purchase, it might be COGS or operating expense
        const expenseAccount = accounts.find(acc => acc.code.startsWith('5'));
        if (expenseAccount) {
          const key = `${expenseAccount.code}-${expenseAccount.name}`;
          if (!accountGroups.has(key)) {
            accountGroups.set(key, {
              account: expenseAccount,
              entries: [],
              total_debit: 0,
              total_credit: 0
            });
          }
          const group = accountGroups.get(key)!;
          group.entries.push(entry);
          group.total_debit += entry.total_debit || 0;
          group.total_credit += entry.total_credit || 0;
        }
      }
    });
    
    console.log(`📋 Updated account groups after pattern matching: ${accountGroups.size}`);
    
    // Categorize into P&L sections based on account codes and types
    accountGroups.forEach((group, key) => {
      const { account, entries, total_credit, total_debit } = group;
      const accountCode = account.code || '';
      const accountName = account.name || '';
      
      console.log(`🔍 Analyzing account ${accountCode} - ${accountName}: Debit=${total_debit}, Credit=${total_credit}`);
      
      // Revenue accounts (4xxx) - Credit increases revenue
      if (accountCode.startsWith('4')) {
        // For revenue, we want the net credit amount (credits - debits)
        // In normal accounting, sales increase revenue with credit entries
        let amount = total_credit - total_debit;
        
        // However, if this is a pattern-matched revenue from unlinked entries,
        // the total_debit might represent the total transaction amount
        if (amount <= 0 && total_debit > 0) {
          // Use total_debit as revenue amount for pattern-matched entries
          amount = total_debit;
        }
        
        console.log(`💰 Revenue account found: ${accountCode}, credit=${total_credit}, debit=${total_debit}, final amount: ${amount}`);
        
        if (amount > 0) {
          plData.revenue.items.push({
            account_code: accountCode,
            account_name: accountName,
            amount: amount,
            journal_entries: entries
          });
          plData.revenue.total_revenue += amount;
        }
      }
      
      // Cost of Goods Sold (5100-5199) - Debit increases COGS
      else if (accountCode.startsWith('510') || accountCode.startsWith('511')) {
        const amount = total_debit - total_credit; // Net debit amount
        if (amount > 0) {
          plData.cost_of_goods_sold.items.push({
            account_code: accountCode,
            account_name: accountName,
            amount: amount,
            journal_entries: entries
          });
          plData.cost_of_goods_sold.total_cogs += amount;
        }
      }
      
      // Administrative Expenses (5200-5299)
      else if (accountCode.startsWith('520') || accountCode.startsWith('521') || accountCode.startsWith('522')) {
        const amount = total_debit - total_credit; // Net debit amount
        if (amount > 0) {
          plData.operating_expenses.administrative.items.push({
            account_code: accountCode,
            account_name: accountName,
            amount: amount,
            journal_entries: entries
          });
          plData.operating_expenses.administrative.subtotal += amount;
        }
      }
      
      // Selling & Marketing Expenses (5300-5399)
      else if (accountCode.startsWith('530') || accountCode.startsWith('531') || accountCode.startsWith('532')) {
        const amount = total_debit - total_credit; // Net debit amount
        if (amount > 0) {
          plData.operating_expenses.selling_marketing.items.push({
            account_code: accountCode,
            account_name: accountName,
            amount: amount,
            journal_entries: entries
          });
          plData.operating_expenses.selling_marketing.subtotal += amount;
        }
      }
      
      // General Expenses (5400+ or 5900+)
      else if (accountCode.startsWith('54') || accountCode.startsWith('59') || 
               (accountCode.startsWith('5') && !accountCode.startsWith('51') && 
                !accountCode.startsWith('52') && !accountCode.startsWith('53'))) {
        const amount = total_debit - total_credit; // Net debit amount
        if (amount > 0) {
          plData.operating_expenses.general.items.push({
            account_code: accountCode,
            account_name: accountName,
            amount: amount,
            journal_entries: entries
          });
          plData.operating_expenses.general.subtotal += amount;
        }
      }
      
      // Other Income (4900+)
      else if (accountCode.startsWith('49')) {
        const amount = total_credit - total_debit; // Net credit amount
        plData.other_income += amount;
      }
      
      // Other Expenses or Interest Expense (6xxx)
      else if (accountCode.startsWith('6')) {
        const amount = total_debit - total_credit; // Net debit amount
        plData.other_expenses += amount;
      }
    });

    // Calculate totals and ratios
    plData.operating_expenses.total_opex = 
      plData.operating_expenses.administrative.subtotal +
      plData.operating_expenses.selling_marketing.subtotal +
      plData.operating_expenses.general.subtotal;

    plData.gross_profit = plData.revenue.total_revenue - plData.cost_of_goods_sold.total_cogs;
    plData.gross_profit_margin = plData.revenue.total_revenue > 0 ? 
      (plData.gross_profit / plData.revenue.total_revenue) * 100 : 0;

    plData.operating_income = plData.gross_profit - plData.operating_expenses.total_opex;
    plData.operating_margin = plData.revenue.total_revenue > 0 ? 
      (plData.operating_income / plData.revenue.total_revenue) * 100 : 0;

    // EBITDA = Operating Income (no depreciation data yet)
    plData.ebitda = plData.operating_income;
    plData.ebitda_margin = plData.revenue.total_revenue > 0 ? 
      (plData.ebitda / plData.revenue.total_revenue) * 100 : 0;

    plData.income_before_tax = plData.operating_income + plData.other_income - plData.other_expenses;
    
    // Assume 25% tax rate for demonstration
    plData.tax_expense = plData.income_before_tax > 0 ? plData.income_before_tax * 0.25 : 0;
    
    plData.net_income = plData.income_before_tax - plData.tax_expense;
    plData.net_income_margin = plData.revenue.total_revenue > 0 ? 
      (plData.net_income / plData.revenue.total_revenue) * 100 : 0;

    // Debug final results
    console.log('📊 FINAL P&L RESULTS:');
    console.log(`💰 Total Revenue: ${plData.revenue.total_revenue}`);
    console.log(`📉 Total COGS: ${plData.cost_of_goods_sold.total_cogs}`);
    console.log(`💚 Gross Profit: ${plData.gross_profit}`);
    console.log(`💸 Total OpEx: ${plData.operating_expenses.total_opex}`);
    console.log(`📈 Operating Income: ${plData.operating_income}`);
    console.log(`🎯 Net Income: ${plData.net_income}`);
    console.log(`📋 Revenue Items: ${plData.revenue.items.length}`);
    console.log(`📋 COGS Items: ${plData.cost_of_goods_sold.items.length}`);
    console.log(`📋 Admin Expense Items: ${plData.operating_expenses.administrative.items.length}`);
    
    // Add analysis note if no P&L relevant data found
    if (plData.revenue.total_revenue === 0 && 
        plData.cost_of_goods_sold.total_cogs === 0 && 
        plData.operating_expenses.total_opex === 0) {
      console.log('📝 NOTE: No P&L relevant data found. Most journal entries appear to be:');
      console.log('- Asset purchases (Balance Sheet items)');
      console.log('- Payment transactions (no impact on revenue/expense)');
      console.log('- Deposit transactions (Balance Sheet items)');
      console.log('💡 To generate meaningful P&L data, record:');
      console.log('- Sales transactions (revenue accounts 4xxx)');
      console.log('- Purchase expenses (expense accounts 5xxx)');
      console.log('- Operating expenses (administrative costs, etc.)');
      
      // Add some sample data for demonstration if completely empty
      plData.revenue.items.push({
        account_code: '4101',
        account_name: 'Sales Revenue (Sample)',
        amount: 0,
        journal_entries: []
      });
      
      plData.cost_of_goods_sold.items.push({
        account_code: '5101',
        account_name: 'Cost of Goods Sold (Sample)',
        amount: 0,
        journal_entries: []
      });
      
      plData.operating_expenses.administrative.items.push({
        account_code: '5201',
        account_name: 'Administrative Expenses (Sample)',
        amount: 0,
        journal_entries: []
      });
    }

    return plData;
  }

  // Generate Enhanced P&L from Journal Entries
  async generateEnhancedPLFromJournals(params: ReportParameters): Promise<EnhancedPLFromJournals> {
    if (!params.start_date || !params.end_date) {
      throw new Error('Start date and end date are required for Enhanced P&L analysis');
    }

    try {
      // Get journal entries and accounts in parallel
      const [journalData, accounts] = await Promise.all([
        this.getJournalEntries(params),
        this.getAccounts()
      ]);

      // Categorize journal entries into P&L components
      const plData = this.categorizeJournalEntries(journalData.entries, accounts);
      
      // Set period information
      plData.start_date = params.start_date;
      plData.end_date = params.end_date;
      plData.company.period = `${new Date(params.start_date).toLocaleDateString('id-ID')} - ${new Date(params.end_date).toLocaleDateString('id-ID')}`;

      return plData;
    } catch (error) {
      console.error('Failed to generate Enhanced P&L from journals:', error);
      throw error;
    }
  }
}

export const enhancedPLService = new EnhancedPLService();
