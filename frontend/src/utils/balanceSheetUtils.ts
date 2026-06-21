/**
 * Balance Sheet Utility Functions
 * 
 * Kumpulan utility functions untuk perhitungan Balance Sheet
 * Menyediakan helper functions yang mudah digunakan
 */

import { balanceSheetCalculatorService } from '../services/balanceSheetCalculatorService';
import { 
  BalanceSheet, 
  BalanceSheetCalculationOptions, 
  EnhancedBalanceSheet,
  BalanceSheetValidationResult
} from '../types/balanceSheet';
import { cleanCsvCell } from './exportText';

/**
 * Quick Balance Sheet Generator
 * Generate balance sheet dengan opsi default
 */
export async function generateQuickBalanceSheet(asOfDate?: string): Promise<BalanceSheet> {
  const options: BalanceSheetCalculationOptions = {
    as_of_date: asOfDate || new Date().toISOString().split('T')[0],
    include_zero_balances: false,
    include_inactive_accounts: false,
    group_by_category: true,
    currency_format: 'IDR',
    detail_level: 'detail'
  };

  return await balanceSheetCalculatorService.generateBalanceSheet(options);
}

/**
 * Enhanced Balance Sheet Generator
 * Generate balance sheet dengan kategori detail dan rasio keuangan
 */
export async function generateEnhancedBalanceSheet(options?: BalanceSheetCalculationOptions): Promise<EnhancedBalanceSheet> {
  const defaultOptions: BalanceSheetCalculationOptions = {
    as_of_date: new Date().toISOString().split('T')[0],
    include_zero_balances: false,
    include_inactive_accounts: false,
    group_by_category: true,
    currency_format: 'IDR',
    detail_level: 'full',
    ...options
  };

  return await balanceSheetCalculatorService.generateEnhancedBalanceSheet(defaultOptions);
}

/**
 * Validate Balance Sheet
 * Validasi balance sheet dan return hasil validasi
 */
export async function validateBalanceSheet(balanceSheet: BalanceSheet): Promise<BalanceSheetValidationResult> {
  return await balanceSheetCalculatorService.validateBalanceSheet(balanceSheet);
}

/**
 * Format Currency untuk Display
 * Utility untuk format mata uang
 */
export function formatCurrency(
  amount: number, 
  currency: 'IDR' | 'USD' = 'IDR',
  showSymbol: boolean = true
): string {
  const options: Intl.NumberFormatOptions = {
    minimumFractionDigits: 0,
    maximumFractionDigits: currency === 'IDR' ? 0 : 2
  };

  if (showSymbol) {
    options.style = 'currency';
    options.currency = currency;
  }

  const locale = currency === 'IDR' ? 'id-ID' : 'en-US';
  return new Intl.NumberFormat(locale, options).format(amount);
}

/**
 * Calculate Balance Sheet Ratios
 * Hitung berbagai rasio keuangan dari balance sheet
 */
export function calculateBalanceSheetRatios(balanceSheet: BalanceSheet) {
  const currentAssets = balanceSheet.assets.items
    .filter(item => item.account_code.startsWith('11'))
    .reduce((sum, item) => sum + item.balance, 0);
    
  const currentLiabilities = balanceSheet.liabilities.items
    .filter(item => item.account_code.startsWith('21'))
    .reduce((sum, item) => sum + item.balance, 0);

  const fixedAssets = balanceSheet.assets.items
    .filter(item => item.account_code.startsWith('12') || item.account_code.startsWith('13'))
    .reduce((sum, item) => sum + item.balance, 0);

  const longTermLiabilities = balanceSheet.liabilities.items
    .filter(item => item.account_code.startsWith('22') || item.account_code.startsWith('23'))
    .reduce((sum, item) => sum + item.balance, 0);

  const totalAssets = balanceSheet.total_assets;
  const totalLiabilities = balanceSheet.total_liabilities;
  const totalEquity = balanceSheet.total_equity;

  return {
    // Liquidity Ratios
    current_ratio: currentLiabilities !== 0 ? currentAssets / currentLiabilities : 0,
    working_capital: currentAssets - currentLiabilities,
    
    // Leverage Ratios
    debt_to_equity_ratio: totalEquity !== 0 ? totalLiabilities / totalEquity : 0,
    debt_to_assets_ratio: totalAssets !== 0 ? totalLiabilities / totalAssets : 0,
    equity_ratio: totalAssets !== 0 ? totalEquity / totalAssets : 0,
    
    // Asset Composition
    current_assets_ratio: totalAssets !== 0 ? currentAssets / totalAssets : 0,
    fixed_assets_ratio: totalAssets !== 0 ? fixedAssets / totalAssets : 0,
    
    // Liability Composition
    current_liabilities_ratio: totalLiabilities !== 0 ? currentLiabilities / totalLiabilities : 0,
    long_term_liabilities_ratio: totalLiabilities !== 0 ? longTermLiabilities / totalLiabilities : 0
  };
}

/**
 * Compare Balance Sheets
 * Bandingkan dua balance sheet dan return analisis perubahan
 */
export function compareBalanceSheets(currentBS: BalanceSheet, previousBS: BalanceSheet) {
  const assetChange = currentBS.total_assets - previousBS.total_assets;
  const liabilityChange = currentBS.total_liabilities - previousBS.total_liabilities;
  const equityChange = currentBS.total_equity - previousBS.total_equity;

  const assetChangePercent = previousBS.total_assets !== 0 ? 
    (assetChange / previousBS.total_assets) * 100 : 0;
  const liabilityChangePercent = previousBS.total_liabilities !== 0 ? 
    (liabilityChange / previousBS.total_liabilities) * 100 : 0;
  const equityChangePercent = previousBS.total_equity !== 0 ? 
    (equityChange / previousBS.total_equity) * 100 : 0;

  // Detailed account changes
  const accountChanges: any[] = [];
  
  // Create maps for easy lookup
  const currentAccounts = new Map(currentBS.assets.items.concat(currentBS.liabilities.items, currentBS.equity.items).map(item => [item.account_code, item]));
  const previousAccounts = new Map(previousBS.assets.items.concat(previousBS.liabilities.items, previousBS.equity.items).map(item => [item.account_code, item]));

  // Find changes in accounts
  currentAccounts.forEach((currentAccount, accountCode) => {
    const previousAccount = previousAccounts.get(accountCode);
    if (previousAccount) {
      const change = currentAccount.balance - previousAccount.balance;
      const changePercent = previousAccount.balance !== 0 ? (change / previousAccount.balance) * 100 : 0;
      
      if (Math.abs(change) > 0.01) { // Only significant changes
        accountChanges.push({
          account_code: accountCode,
          account_name: currentAccount.account_name,
          account_type: currentAccount.account_type,
          previous_balance: previousAccount.balance,
          current_balance: currentAccount.balance,
          change: change,
          change_percent: changePercent
        });
      }
    } else {
      // New account
      accountChanges.push({
        account_code: accountCode,
        account_name: currentAccount.account_name,
        account_type: currentAccount.account_type,
        previous_balance: 0,
        current_balance: currentAccount.balance,
        change: currentAccount.balance,
        change_percent: 100,
        is_new: true
      });
    }
  });

  // Find removed accounts
  previousAccounts.forEach((previousAccount, accountCode) => {
    if (!currentAccounts.has(accountCode) && Math.abs(previousAccount.balance) > 0.01) {
      accountChanges.push({
        account_code: accountCode,
        account_name: previousAccount.account_name,
        account_type: previousAccount.account_type,
        previous_balance: previousAccount.balance,
        current_balance: 0,
        change: -previousAccount.balance,
        change_percent: -100,
        is_removed: true
      });
    }
  });

  return {
    summary: {
      period: `${previousBS.as_of_date} to ${currentBS.as_of_date}`,
      asset_change: assetChange,
      asset_change_percent: assetChangePercent,
      liability_change: liabilityChange,
      liability_change_percent: liabilityChangePercent,
      equity_change: equityChange,
      equity_change_percent: equityChangePercent
    },
    account_changes: accountChanges.sort((a, b) => Math.abs(b.change) - Math.abs(a.change)), // Sort by change magnitude
    ratios: {
      current: calculateBalanceSheetRatios(currentBS),
      previous: calculateBalanceSheetRatios(previousBS)
    }
  };
}

/**
 * Export Balance Sheet to CSV
 * Export balance sheet ke format CSV
 */
export function exportBalanceSheetToCSV(balanceSheet: BalanceSheet): string {
  const headers = ['Account Code', 'Account Name', 'Account Type', 'Balance', 'Debit Balance', 'Credit Balance'];
  const rows: string[][] = [headers];

  // Add assets
  rows.push(['ASSETS', '', '', '', '', '']);
  balanceSheet.assets.items.forEach(item => {
    rows.push([
      item.account_code,
      item.account_name,
      item.account_type,
      item.balance.toString(),
      item.debit_balance.toString(),
      item.credit_balance.toString()
    ]);
  });
  rows.push(['TOTAL ASSETS', '', '', balanceSheet.total_assets.toString(), '', '']);
  rows.push(['', '', '', '', '', '']);

  // Add liabilities
  rows.push(['LIABILITIES', '', '', '', '', '']);
  balanceSheet.liabilities.items.forEach(item => {
    rows.push([
      item.account_code,
      item.account_name,
      item.account_type,
      item.balance.toString(),
      item.debit_balance.toString(),
      item.credit_balance.toString()
    ]);
  });
  rows.push(['TOTAL LIABILITIES', '', '', balanceSheet.total_liabilities.toString(), '', '']);
  rows.push(['', '', '', '', '', '']);

  // Add equity
  rows.push(['EQUITY', '', '', '', '', '']);
  balanceSheet.equity.items.forEach(item => {
    rows.push([
      item.account_code,
      item.account_name,
      item.account_type,
      item.balance.toString(),
      item.debit_balance.toString(),
      item.credit_balance.toString()
    ]);
  });
  rows.push(['TOTAL EQUITY', '', '', balanceSheet.total_equity.toString(), '', '']);
  rows.push(['', '', '', '', '', '']);
  rows.push(['TOTAL LIABILITIES + EQUITY', '', '', balanceSheet.total_liabilities_equity.toString(), '', '']);

  // Convert to CSV string
  return rows.map(row => row.map(cell => `"${cleanCsvCell(cell).replace(/"/g, '""')}"`).join(',')).join('\n');
}

/**
 * Refresh Account Balances
 * Refresh materialized view untuk account balances
 */
export async function refreshAccountBalances(): Promise<{ message: string; updated_at: string }> {
  return await balanceSheetCalculatorService.refreshAccountBalances();
}

/**
 * Get Balance Sheet Summary
 * Dapatkan ringkasan balance sheet dalam format yang mudah dibaca
 */
export function getBalanceSheetSummary(balanceSheet: BalanceSheet) {
  const ratios = calculateBalanceSheetRatios(balanceSheet);
  
  return {
    company_name: balanceSheet.company_name,
    as_of_date: balanceSheet.as_of_date,
    totals: {
      assets: formatCurrency(balanceSheet.total_assets),
      liabilities: formatCurrency(balanceSheet.total_liabilities),
      equity: formatCurrency(balanceSheet.total_equity)
    },
    is_balanced: balanceSheet.is_balanced,
    balance_difference: formatCurrency(balanceSheet.balance_difference),
    key_ratios: {
      current_ratio: ratios.current_ratio.toFixed(2),
      debt_to_equity_ratio: ratios.debt_to_equity_ratio.toFixed(2),
      equity_ratio: (ratios.equity_ratio * 100).toFixed(1) + '%'
    },
    accounts_count: {
      assets: balanceSheet.assets.items.length,
      liabilities: balanceSheet.liabilities.items.length,
      equity: balanceSheet.equity.items.length,
      total: balanceSheet.metadata.accounts_included
    },
    data_quality: {
      source: balanceSheet.metadata.source,
      journal_entries: balanceSheet.metadata.journal_entries_count,
      generated_at: new Date(balanceSheet.generated_at).toLocaleString('id-ID')
    }
  };
}

/**
 * Simple Balance Sheet Calculator
 * Calculator sederhana untuk quick calculation tanpa option banyak
 */
export class SimpleBalanceSheetCalculator {
  /**
   * Hitung balance sheet untuk tanggal tertentu
   */
  static async calculate(asOfDate?: string): Promise<BalanceSheet> {
    return await generateQuickBalanceSheet(asOfDate);
  }

  /**
   * Hitung dan validasi balance sheet
   */
  static async calculateAndValidate(asOfDate?: string): Promise<{
    balanceSheet: BalanceSheet;
    validation: BalanceSheetValidationResult;
    summary: any;
  }> {
    const balanceSheet = await this.calculate(asOfDate);
    const validation = await validateBalanceSheet(balanceSheet);
    const summary = getBalanceSheetSummary(balanceSheet);

    return {
      balanceSheet,
      validation,
      summary
    };
  }

  /**
   * Format balance sheet untuk display console
   */
  static formatForConsole(balanceSheet: BalanceSheet): string {
    let output = '';
    
    output += `\n${'='.repeat(60)}\n`;
    output += `${balanceSheet.report_title}\n`;
    output += `${balanceSheet.company_name}\n`;
    output += `As of ${balanceSheet.as_of_date}\n`;
    output += `${'='.repeat(60)}\n`;
    
    // Assets
    output += `\nASSETS\n`;
    output += `${'-'.repeat(40)}\n`;
    balanceSheet.assets.items.forEach(item => {
      output += `${item.account_code.padEnd(8)} ${item.account_name.padEnd(25)} ${formatCurrency(item.balance).padStart(15)}\n`;
    });
    output += `${'-'.repeat(40)}\n`;
    output += `TOTAL ASSETS${' '.repeat(26)}${formatCurrency(balanceSheet.total_assets).padStart(15)}\n`;
    
    // Liabilities
    output += `\nLIABILITIES\n`;
    output += `${'-'.repeat(40)}\n`;
    balanceSheet.liabilities.items.forEach(item => {
      output += `${item.account_code.padEnd(8)} ${item.account_name.padEnd(25)} ${formatCurrency(item.balance).padStart(15)}\n`;
    });
    output += `${'-'.repeat(40)}\n`;
    output += `TOTAL LIABILITIES${' '.repeat(21)}${formatCurrency(balanceSheet.total_liabilities).padStart(15)}\n`;
    
    // Equity
    output += `\nEQUITY\n`;
    output += `${'-'.repeat(40)}\n`;
    balanceSheet.equity.items.forEach(item => {
      output += `${item.account_code.padEnd(8)} ${item.account_name.padEnd(25)} ${formatCurrency(item.balance).padStart(15)}\n`;
    });
    output += `${'-'.repeat(40)}\n`;
    output += `TOTAL EQUITY${' '.repeat(26)}${formatCurrency(balanceSheet.total_equity).padStart(15)}\n`;
    
    output += `\n${'-'.repeat(40)}\n`;
    output += `TOTAL LIABILITIES + EQUITY${' '.repeat(11)}${formatCurrency(balanceSheet.total_liabilities_equity).padStart(15)}\n`;
    output += `${'='.repeat(60)}\n`;
    
    // Balance check
    if (balanceSheet.is_balanced) {
      output += `✅ Balance Sheet is BALANCED\n`;
    } else {
      output += `❌ Balance Sheet is NOT BALANCED\n`;
      output += `Difference: ${formatCurrency(balanceSheet.balance_difference)}\n`;
    }
    
    return output;
  }
}
