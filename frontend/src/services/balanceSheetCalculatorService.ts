/**
 * Balance Sheet Calculator Service
 * 
 * Service untuk menghitung Balance Sheet dari SSOT Journal System
 * Mengintegrasikan dengan ssotJournalService yang sudah ada
 */

import { ssotJournalService, SSOTJournalEntry } from './ssotJournalService';
import { accountService } from './accountService';
import { Account } from '../types/account';
import {
  BalanceSheet,
  BalanceSheetItem,
  BalanceSheetSection,
  BalanceSheetCalculationOptions,
  AccountBalance,
  JournalSummaryForBalanceSheet,
  BalanceSheetValidationResult,
  ValidationError,
  ValidationWarning,
  AssetCategory,
  LiabilityCategory,
  EquityCategory,
  DetailedBalanceSheetSection,
  EnhancedBalanceSheet
} from '../types/balanceSheet';

class BalanceSheetCalculatorService {
  private readonly BALANCE_TOLERANCE = 0.01; // Toleransi untuk pengecekan balance (1 sen)
  
  /**
   * Generate Balance Sheet dari SSOT Journal System
   */
  async generateBalanceSheet(options: BalanceSheetCalculationOptions = {}): Promise<BalanceSheet> {
    const startTime = Date.now();
    
    try {
      // Set default options
      const calcOptions: Required<BalanceSheetCalculationOptions> = {
        as_of_date: options.as_of_date || new Date().toISOString().split('T')[0],
        include_zero_balances: options.include_zero_balances ?? false,
        include_inactive_accounts: options.include_inactive_accounts ?? false,
        group_by_category: options.group_by_category ?? true,
        currency_format: options.currency_format || 'IDR',
        detail_level: options.detail_level || 'detail'
      };

      console.log('🧮 Generating Balance Sheet with options:', calcOptions);

      // 1. Ambil semua journal entries sampai tanggal tertentu
      const journalEntries = await this.getJournalEntriesUpToDate(calcOptions.as_of_date);
      console.log(`📋 Retrieved ${journalEntries.length} journal entries`);

      // 2. Ambil account balances dari SSOT system
      const accountBalances = await ssotJournalService.getAccountBalances();
      console.log(`💰 Retrieved ${accountBalances.length} account balances`);

      // 3. Ambil master accounts untuk detail informasi
      const accounts = await this.getAccounts();
      console.log(`🏦 Retrieved ${accounts.length} master accounts`);

      // 4. Hitung balance sheet items dari account balances
      const balanceSheetItems = this.calculateBalanceSheetItems(
        accountBalances, 
        accounts, 
        calcOptions
      );

      // 5. Kategorisasi balance sheet items
      const categorizedItems = this.categorizeBalanceSheetItems(balanceSheetItems);

      // 6. Generate summary untuk metadata
      const journalSummary = this.generateJournalSummary(journalEntries, accountBalances);

      // 7. Build balance sheet structure
      const balanceSheet: BalanceSheet = {
        company_name: 'Your Company Name', // TODO: Get from settings
        report_title: 'Balance Sheet',
        as_of_date: calcOptions.as_of_date,
        generated_at: new Date().toISOString(),
        assets: categorizedItems.assets,
        liabilities: categorizedItems.liabilities,
        equity: categorizedItems.equity,
        total_assets: categorizedItems.assets.subtotal,
        total_liabilities: categorizedItems.liabilities.subtotal,
        total_equity: categorizedItems.equity.subtotal,
        total_liabilities_equity: categorizedItems.liabilities.subtotal + categorizedItems.equity.subtotal,
        is_balanced: Math.abs(categorizedItems.assets.subtotal - (categorizedItems.liabilities.subtotal + categorizedItems.equity.subtotal)) <= this.BALANCE_TOLERANCE,
        balance_difference: categorizedItems.assets.subtotal - (categorizedItems.liabilities.subtotal + categorizedItems.equity.subtotal),
        metadata: {
          source: 'SSOT_JOURNAL',
          journal_entries_count: journalSummary.total_journal_entries,
          accounts_included: balanceSheetItems.length,
          calculation_method: 'account_balances_aggregation',
          data_freshness: new Date().toISOString()
        }
      };

      console.log(`✅ Balance Sheet generated successfully in ${Date.now() - startTime}ms`);
      return balanceSheet;

    } catch (error) {
      console.error('❌ Error generating balance sheet:', error);
      throw new Error(`Failed to generate balance sheet: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  /**
   * Generate Enhanced Balance Sheet dengan kategorisasi detail dan rasio keuangan
   */
  async generateEnhancedBalanceSheet(options: BalanceSheetCalculationOptions = {}): Promise<EnhancedBalanceSheet> {
    const basicBalanceSheet = await this.generateBalanceSheet({ ...options, group_by_category: true });
    
    // Convert to enhanced format with detailed categories
    const enhancedAssets = this.enhanceSection(basicBalanceSheet.assets, 'ASSET');
    const enhancedLiabilities = this.enhanceSection(basicBalanceSheet.liabilities, 'LIABILITY');
    const enhancedEquity = this.enhanceSection(basicBalanceSheet.equity, 'EQUITY');

    // Calculate financial ratios
    const ratios = this.calculateFinancialRatios(basicBalanceSheet);

    return {
      ...basicBalanceSheet,
      assets: enhancedAssets,
      liabilities: enhancedLiabilities,
      equity: enhancedEquity,
      ratios
    };
  }

  /**
   * Validasi Balance Sheet
   */
  async validateBalanceSheet(balanceSheet: BalanceSheet): Promise<BalanceSheetValidationResult> {
    const errors: ValidationError[] = [];
    const warnings: ValidationWarning[] = [];

    // 1. Balance check - Assets = Liabilities + Equity
    const assetTotal = balanceSheet.total_assets;
    const liabilitiesEquityTotal = balanceSheet.total_liabilities_equity;
    const difference = Math.abs(assetTotal - liabilitiesEquityTotal);
    const isBalanced = difference <= this.BALANCE_TOLERANCE;

    if (!isBalanced) {
      errors.push({
        type: 'BALANCE_MISMATCH',
        message: `Balance Sheet tidak balance. Assets: ${this.formatCurrency(assetTotal)}, Liabilities+Equity: ${this.formatCurrency(liabilitiesEquityTotal)}, Selisih: ${this.formatCurrency(difference)}`,
        details: { assetTotal, liabilitiesEquityTotal, difference }
      });
    }

    // 2. Check for negative assets
    balanceSheet.assets.items.forEach(item => {
      if (item.balance < 0) {
        warnings.push({
          type: 'NEGATIVE_ASSET',
          message: `Asset ${item.account_name} memiliki balance negatif: ${this.formatCurrency(item.balance)}`,
          details: item
        });
      }
    });

    // 3. Check for positive liabilities (should be negative in normal accounting)
    balanceSheet.liabilities.items.forEach(item => {
      if (item.balance > 0) {
        warnings.push({
          type: 'POSITIVE_LIABILITY',
          message: `Liability ${item.account_name} memiliki balance positif: ${this.formatCurrency(item.balance)}`,
          details: item
        });
      }
    });

    // 4. Check data freshness
    const dataAge = Date.now() - new Date(balanceSheet.metadata.data_freshness).getTime();
    const hoursAge = dataAge / (1000 * 60 * 60);
    
    if (hoursAge > 24) {
      warnings.push({
        type: 'DATA_FRESHNESS',
        message: `Data balance sheet sudah berumur ${Math.floor(hoursAge)} jam. Pertimbangkan untuk refresh data.`,
        details: { hoursAge }
      });
    }

    return {
      is_valid: errors.length === 0,
      errors,
      warnings,
      balance_check: {
        assets_total: assetTotal,
        liabilities_equity_total: liabilitiesEquityTotal,
        difference,
        tolerance: this.BALANCE_TOLERANCE,
        is_balanced: isBalanced
      }
    };
  }

  /**
   * Get journal entries up to specific date
   */
  private async getJournalEntriesUpToDate(asOfDate: string): Promise<SSOTJournalEntry[]> {
    try {
      const result = await ssotJournalService.getJournalEntries({
        start_date: '1900-01-01', // Get all historical entries
        end_date: asOfDate,
        status: 'POSTED',
        limit: 50000 // Large limit to get all entries
      });
      
      return result.data || [];
    } catch (error) {
      console.error('Error fetching journal entries:', error);
      throw error;
    }
  }

  /**
   * Get master accounts
   */
  private async getAccounts(): Promise<Account[]> {
    try {
      return await accountService.getAccounts('');
    } catch (error) {
      console.error('Error fetching accounts:', error);
      return []; // Return empty array if accounts cannot be fetched
    }
  }

  /**
   * Calculate balance sheet items from account balances
   */
  private calculateBalanceSheetItems(
    accountBalances: any[],
    accounts: Account[],
    options: Required<BalanceSheetCalculationOptions>
  ): BalanceSheetItem[] {
    const balanceSheetItems: BalanceSheetItem[] = [];

    // Create account lookup map
    const accountMap = new Map<number, Account>();
    accounts.forEach(account => accountMap.set(account.id, account));

    accountBalances.forEach(balance => {
      const account = accountMap.get(balance.account_id);
      
      // Skip if account not found
      if (!account) {
        console.warn(`Account not found for balance: ${balance.account_id}`);
        return;
      }

      // Filter balance sheet accounts only (ASSET, LIABILITY, EQUITY)
      if (!['ASSET', 'LIABILITY', 'EQUITY'].includes(account.type)) {
        return;
      }

      // Skip inactive accounts if not included
      if (!options.include_inactive_accounts && !account.is_active) {
        return;
      }

      // Calculate net balance based on account type
      let netBalance = 0;
      if (account.type === 'ASSET') {
        netBalance = balance.debit_balance - balance.credit_balance;
      } else if (account.type === 'LIABILITY' || account.type === 'EQUITY') {
        netBalance = balance.credit_balance - balance.debit_balance;
      }

      // Skip zero balances if not included
      if (!options.include_zero_balances && Math.abs(netBalance) < this.BALANCE_TOLERANCE) {
        return;
      }

      balanceSheetItems.push({
        id: account.id,
        account_code: account.code,
        account_name: account.name,
        account_type: account.type as 'ASSET' | 'LIABILITY' | 'EQUITY',
        category: account.category,
        balance: netBalance,
        debit_balance: balance.debit_balance,
        credit_balance: balance.credit_balance,
        is_header: account.is_header,
        level: account.level,
        parent_id: account.parent_id
      });
    });

    return balanceSheetItems;
  }

  /**
   * Categorize balance sheet items into Assets, Liabilities, and Equity
   */
  private categorizeBalanceSheetItems(items: BalanceSheetItem[]): {
    assets: BalanceSheetSection;
    liabilities: BalanceSheetSection;
    equity: BalanceSheetSection;
  } {
    const assets = items.filter(item => item.account_type === 'ASSET');
    const liabilities = items.filter(item => item.account_type === 'LIABILITY');
    const equity = items.filter(item => item.account_type === 'EQUITY');

    return {
      assets: {
        name: 'ASSETS',
        type: 'ASSET',
        items: assets,
        subtotal: assets.reduce((sum, item) => sum + item.balance, 0)
      },
      liabilities: {
        name: 'LIABILITIES',
        type: 'LIABILITY',
        items: liabilities,
        subtotal: liabilities.reduce((sum, item) => sum + item.balance, 0)
      },
      equity: {
        name: 'EQUITY',
        type: 'EQUITY',
        items: equity,
        subtotal: equity.reduce((sum, item) => sum + item.balance, 0)
      }
    };
  }

  /**
   * Generate journal summary for metadata
   */
  private generateJournalSummary(
    journalEntries: SSOTJournalEntry[],
    accountBalances: any[]
  ): JournalSummaryForBalanceSheet {
    const postedEntries = journalEntries.filter(entry => entry.status === 'POSTED');
    const dates = journalEntries
      .map(entry => new Date(entry.entry_date))
      .filter(date => !isNaN(date.getTime()))
      .sort((a, b) => a.getTime() - b.getTime());

    const totalDebits = accountBalances.reduce((sum, balance) => sum + balance.debit_balance, 0);
    const totalCredits = accountBalances.reduce((sum, balance) => sum + balance.credit_balance, 0);

    return {
      total_journal_entries: journalEntries.length,
      posted_entries: postedEntries.length,
      date_range: {
        earliest_entry: dates[0]?.toISOString() || new Date().toISOString(),
        latest_entry: dates[dates.length - 1]?.toISOString() || new Date().toISOString()
      },
      accounts_affected: accountBalances.length,
      total_debits: totalDebits,
      total_credits: totalCredits,
      is_balanced: Math.abs(totalDebits - totalCredits) <= this.BALANCE_TOLERANCE
    };
  }

  /**
   * Enhance section with detailed categories
   */
  private enhanceSection(section: BalanceSheetSection, type: 'ASSET' | 'LIABILITY' | 'EQUITY'): DetailedBalanceSheetSection {
    const categories: { [key: string]: { items: BalanceSheetItem[]; subtotal: number; } } = {};

    section.items.forEach(item => {
      let categoryName = '';
      
      // Categorize based on account code patterns
      if (type === 'ASSET') {
        categoryName = this.categorizeAsset(item.account_code);
      } else if (type === 'LIABILITY') {
        categoryName = this.categorizeLiability(item.account_code);
      } else if (type === 'EQUITY') {
        categoryName = this.categorizeEquity(item.account_code);
      }

      if (!categories[categoryName]) {
        categories[categoryName] = { items: [], subtotal: 0 };
      }

      categories[categoryName].items.push(item);
      categories[categoryName].subtotal += item.balance;
    });

    return {
      ...section,
      categories
    };
  }

  /**
   * Categorize asset based on account code
   */
  private categorizeAsset(accountCode: string): string {
    const code = accountCode.substring(0, 2);
    
    switch (code) {
      case '11':
        return AssetCategory.CURRENT_ASSETS;
      case '12':
      case '13':
        return AssetCategory.FIXED_ASSETS;
      case '14':
        return AssetCategory.INTANGIBLE_ASSETS;
      case '15':
        return AssetCategory.INVESTMENTS;
      default:
        return AssetCategory.OTHER_ASSETS;
    }
  }

  /**
   * Categorize liability based on account code
   */
  private categorizeLiability(accountCode: string): string {
    const code = accountCode.substring(0, 2);
    
    switch (code) {
      case '21':
        return LiabilityCategory.CURRENT_LIABILITIES;
      case '22':
      case '23':
        return LiabilityCategory.LONG_TERM_LIABILITIES;
      default:
        return LiabilityCategory.OTHER_LIABILITIES;
    }
  }

  /**
   * Categorize equity based on account code
   */
  private categorizeEquity(accountCode: string): string {
    const code = accountCode.substring(0, 2);
    
    switch (code) {
      case '31':
        return EquityCategory.PAID_IN_CAPITAL;
      case '32':
        return EquityCategory.RETAINED_EARNINGS;
      default:
        return EquityCategory.OTHER_EQUITY;
    }
  }

  /**
   * Calculate financial ratios
   */
  private calculateFinancialRatios(balanceSheet: BalanceSheet): any {
    const currentAssets = balanceSheet.assets.items
      .filter(item => item.account_code.startsWith('11'))
      .reduce((sum, item) => sum + item.balance, 0);
      
    const currentLiabilities = balanceSheet.liabilities.items
      .filter(item => item.account_code.startsWith('21'))
      .reduce((sum, item) => sum + item.balance, 0);

    const totalDebt = balanceSheet.total_liabilities;
    const totalEquity = balanceSheet.total_equity;
    const totalAssets = balanceSheet.total_assets;

    return {
      current_ratio: currentLiabilities !== 0 ? currentAssets / currentLiabilities : 0,
      debt_to_equity: totalEquity !== 0 ? totalDebt / totalEquity : 0,
      equity_ratio: totalAssets !== 0 ? totalEquity / totalAssets : 0
    };
  }

  /**
   * Format currency for display
   */
  private formatCurrency(amount: number, currency: 'IDR' | 'USD' = 'IDR'): string {
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

  /**
   * Refresh account balances materialized view
   */
  async refreshAccountBalances(): Promise<{ message: string; updated_at: string }> {
    return await ssotJournalService.refreshAccountBalances();
  }
}

export const balanceSheetCalculatorService = new BalanceSheetCalculatorService();
export default balanceSheetCalculatorService;
