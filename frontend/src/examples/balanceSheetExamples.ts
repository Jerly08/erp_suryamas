/**
 * Balance Sheet Calculator - Contoh Penggunaan
 * 
 * File ini berisi berbagai contoh cara menggunakan Balance Sheet Calculator
 * yang telah dibuat berdasarkan SSOT Journal System
 */

import { balanceSheetCalculatorService } from '../services/balanceSheetCalculatorService';
import { 
  generateQuickBalanceSheet, 
  generateEnhancedBalanceSheet,
  validateBalanceSheet,
  calculateBalanceSheetRatios,
  compareBalanceSheets,
  exportBalanceSheetToCSV,
  getBalanceSheetSummary,
  formatCurrency,
  refreshAccountBalances,
  SimpleBalanceSheetCalculator
} from '../utils/balanceSheetUtils';
import { BalanceSheetCalculationOptions } from '../types/balanceSheet';

/**
 * Example 1: Basic Balance Sheet Generation
 * Generate balance sheet sederhana dengan default options
 */
export async function example1_BasicBalanceSheet() {
  console.log('🧮 Example 1: Basic Balance Sheet Generation');
  console.log('=' .repeat(50));

  try {
    // Generate balance sheet untuk hari ini
    const balanceSheet = await generateQuickBalanceSheet();
    
    console.log('✅ Balance Sheet generated successfully!');
    console.log(`As of: ${balanceSheet.as_of_date}`);
    console.log(`Total Assets: ${formatCurrency(balanceSheet.total_assets)}`);
    console.log(`Total Liabilities: ${formatCurrency(balanceSheet.total_liabilities)}`);
    console.log(`Total Equity: ${formatCurrency(balanceSheet.total_equity)}`);
    console.log(`Is Balanced: ${balanceSheet.is_balanced ? '✅' : '❌'}`);
    
    if (!balanceSheet.is_balanced) {
      console.log(`Balance Difference: ${formatCurrency(balanceSheet.balance_difference)}`);
    }

    return balanceSheet;
  } catch (error) {
    console.error('❌ Error generating balance sheet:', error);
    throw error;
  }
}

/**
 * Example 2: Balance Sheet dengan Custom Options
 * Generate balance sheet dengan opsi kustom
 */
export async function example2_CustomOptionsBalanceSheet() {
  console.log('\n🧮 Example 2: Balance Sheet with Custom Options');
  console.log('=' .repeat(50));

  try {
    const options: BalanceSheetCalculationOptions = {
      as_of_date: '2024-12-31', // Specific date
      include_zero_balances: true, // Include accounts with zero balance
      include_inactive_accounts: false, // Exclude inactive accounts
      group_by_category: true, // Group by account categories
      currency_format: 'IDR',
      detail_level: 'full'
    };

    const balanceSheet = await balanceSheetCalculatorService.generateBalanceSheet(options);
    
    console.log('✅ Custom Balance Sheet generated!');
    console.log(`As of: ${balanceSheet.as_of_date}`);
    console.log(`Accounts included: ${balanceSheet.metadata.accounts_included}`);
    console.log(`Journal entries: ${balanceSheet.metadata.journal_entries_count}`);
    
    // Show assets breakdown
    console.log('\n📊 Assets Breakdown:');
    balanceSheet.assets.items.forEach(item => {
      console.log(`  ${item.account_code} - ${item.account_name}: ${formatCurrency(item.balance)}`);
    });

    return balanceSheet;
  } catch (error) {
    console.error('❌ Error generating custom balance sheet:', error);
    throw error;
  }
}

/**
 * Example 3: Enhanced Balance Sheet dengan Ratios
 * Generate balance sheet dengan kategorisasi detail dan rasio keuangan
 */
export async function example3_EnhancedBalanceSheet() {
  console.log('\n🧮 Example 3: Enhanced Balance Sheet with Ratios');
  console.log('=' .repeat(50));

  try {
    const enhancedBS = await generateEnhancedBalanceSheet({
      as_of_date: new Date().toISOString().split('T')[0],
      detail_level: 'full'
    });
    
    console.log('✅ Enhanced Balance Sheet generated!');
    console.log(`Company: ${enhancedBS.company_name}`);
    console.log(`Report Date: ${enhancedBS.as_of_date}`);
    
    // Show financial ratios
    if (enhancedBS.ratios) {
      console.log('\n📈 Financial Ratios:');
      console.log(`Current Ratio: ${enhancedBS.ratios.current_ratio?.toFixed(2)}`);
      console.log(`Debt to Equity: ${enhancedBS.ratios.debt_to_equity?.toFixed(2)}`);
      console.log(`Equity Ratio: ${(enhancedBS.ratios.equity_ratio! * 100).toFixed(1)}%`);
    }
    
    // Show detailed categories
    console.log('\n🏦 Asset Categories:');
    Object.entries(enhancedBS.assets.categories).forEach(([category, data]) => {
      console.log(`  ${category}: ${formatCurrency(data.subtotal)} (${data.items.length} accounts)`);
    });

    return enhancedBS;
  } catch (error) {
    console.error('❌ Error generating enhanced balance sheet:', error);
    throw error;
  }
}

/**
 * Example 4: Balance Sheet Validation
 * Validasi balance sheet dan tampilkan hasil validasi
 */
export async function example4_BalanceSheetValidation() {
  console.log('\n🧮 Example 4: Balance Sheet Validation');
  console.log('=' .repeat(50));

  try {
    // Generate balance sheet
    const balanceSheet = await generateQuickBalanceSheet();
    
    // Validate balance sheet
    const validation = await validateBalanceSheet(balanceSheet);
    
    console.log('🔍 Balance Sheet Validation Results:');
    console.log(`Is Valid: ${validation.is_valid ? '✅' : '❌'}`);
    console.log(`Balance Check: ${validation.balance_check.is_balanced ? '✅' : '❌'}`);
    
    if (validation.errors.length > 0) {
      console.log('\n❌ Errors found:');
      validation.errors.forEach(error => {
        console.log(`  ${error.type}: ${error.message}`);
      });
    }
    
    if (validation.warnings.length > 0) {
      console.log('\n⚠️  Warnings:');
      validation.warnings.forEach(warning => {
        console.log(`  ${warning.type}: ${warning.message}`);
      });
    }
    
    console.log('\n💰 Balance Check Details:');
    console.log(`Assets Total: ${formatCurrency(validation.balance_check.assets_total)}`);
    console.log(`Liabilities + Equity Total: ${formatCurrency(validation.balance_check.liabilities_equity_total)}`);
    console.log(`Difference: ${formatCurrency(validation.balance_check.difference)}`);
    console.log(`Tolerance: ${formatCurrency(validation.balance_check.tolerance)}`);

    return validation;
  } catch (error) {
    console.error('❌ Error validating balance sheet:', error);
    throw error;
  }
}

/**
 * Example 5: Balance Sheet Comparison
 * Bandingkan dua balance sheet dari periode berbeda
 */
export async function example5_BalanceSheetComparison() {
  console.log('\n🧮 Example 5: Balance Sheet Comparison');
  console.log('=' .repeat(50));

  try {
    // Generate current month balance sheet
    const currentBS = await generateQuickBalanceSheet();
    
    // Generate previous month balance sheet (simulate)
    const lastMonth = new Date();
    lastMonth.setMonth(lastMonth.getMonth() - 1);
    const previousBS = await generateQuickBalanceSheet(lastMonth.toISOString().split('T')[0]);
    
    // Compare balance sheets
    const comparison = compareBalanceSheets(currentBS, previousBS);
    
    console.log('📊 Balance Sheet Comparison Results:');
    console.log(`Period: ${comparison.summary.period}`);
    
    console.log('\n📈 Changes Summary:');
    console.log(`Assets Change: ${formatCurrency(comparison.summary.asset_change)} (${comparison.summary.asset_change_percent.toFixed(2)}%)`);
    console.log(`Liabilities Change: ${formatCurrency(comparison.summary.liability_change)} (${comparison.summary.liability_change_percent.toFixed(2)}%)`);
    console.log(`Equity Change: ${formatCurrency(comparison.summary.equity_change)} (${comparison.summary.equity_change_percent.toFixed(2)}%)`);
    
    // Show top account changes
    console.log('\n🔄 Top Account Changes:');
    comparison.account_changes.slice(0, 5).forEach(change => {
      const changeType = change.change > 0 ? '📈' : '📉';
      console.log(`  ${changeType} ${change.account_code} - ${change.account_name}: ${formatCurrency(change.change)} (${change.change_percent.toFixed(2)}%)`);
    });

    return comparison;
  } catch (error) {
    console.error('❌ Error comparing balance sheets:', error);
    throw error;
  }
}

/**
 * Example 6: Export Balance Sheet to CSV
 * Export balance sheet ke format CSV
 */
export async function example6_ExportToCSV() {
  console.log('\n🧮 Example 6: Export Balance Sheet to CSV');
  console.log('=' .repeat(50));

  try {
    // Generate balance sheet
    const balanceSheet = await generateQuickBalanceSheet();
    
    // Export to CSV
    const csvData = exportBalanceSheetToCSV(balanceSheet);
    
    console.log('📄 CSV Export completed!');
    console.log(`CSV Data Length: ${csvData.length} characters`);
    console.log(`Lines: ${csvData.split('\n').length}`);
    
    // Show first few lines
    const lines = csvData.split('\n');
    console.log('\n📝 First 10 lines of CSV:');
    lines.slice(0, 10).forEach((line, index) => {
      console.log(`${String(index + 1).padStart(2)}: ${line}`);
    });
    
    // In real application, you would save this to file or download
    console.log('\n💾 To save to file, use:');
    console.log('const blob = new Blob([csvData], { type: "text/csv" });');
    console.log('const url = URL.createObjectURL(blob);');
    console.log('// Then trigger download');

    return csvData;
  } catch (error) {
    console.error('❌ Error exporting to CSV:', error);
    throw error;
  }
}

/**
 * Example 7: Balance Sheet Summary
 * Dapatkan ringkasan balance sheet dalam format yang mudah dibaca
 */
export async function example7_BalanceSheetSummary() {
  console.log('\n🧮 Example 7: Balance Sheet Summary');
  console.log('=' .repeat(50));

  try {
    // Generate balance sheet
    const balanceSheet = await generateQuickBalanceSheet();
    
    // Get summary
    const summary = getBalanceSheetSummary(balanceSheet);
    
    console.log('📋 Balance Sheet Summary:');
    console.log(`Company: ${summary.company_name}`);
    console.log(`As of: ${summary.as_of_date}`);
    console.log(`Generated: ${summary.data_quality.generated_at}`);
    
    console.log('\n💰 Financial Totals:');
    console.log(`Assets: ${summary.totals.assets}`);
    console.log(`Liabilities: ${summary.totals.liabilities}`);
    console.log(`Equity: ${summary.totals.equity}`);
    console.log(`Balanced: ${summary.is_balanced ? '✅' : '❌'}`);
    
    console.log('\n📊 Key Ratios:');
    console.log(`Current Ratio: ${summary.key_ratios.current_ratio}`);
    console.log(`Debt to Equity: ${summary.key_ratios.debt_to_equity_ratio}`);
    console.log(`Equity Ratio: ${summary.key_ratios.equity_ratio}`);
    
    console.log('\n🏦 Account Counts:');
    console.log(`Assets: ${summary.accounts_count.assets}`);
    console.log(`Liabilities: ${summary.accounts_count.liabilities}`);
    console.log(`Equity: ${summary.accounts_count.equity}`);
    console.log(`Total: ${summary.accounts_count.total}`);
    
    console.log('\n🔍 Data Quality:');
    console.log(`Source: ${summary.data_quality.source}`);
    console.log(`Journal Entries: ${summary.data_quality.journal_entries}`);

    return summary;
  } catch (error) {
    console.error('❌ Error generating summary:', error);
    throw error;
  }
}

/**
 * Example 8: Simple Balance Sheet Calculator
 * Menggunakan SimpleBalanceSheetCalculator class untuk perhitungan sederhana
 */
export async function example8_SimpleCalculator() {
  console.log('\n🧮 Example 8: Simple Balance Sheet Calculator');
  console.log('=' .repeat(50));

  try {
    // Method 1: Simple calculation
    console.log('Method 1: Simple Calculation');
    const balanceSheet1 = await SimpleBalanceSheetCalculator.calculate();
    console.log(`✅ Generated for: ${balanceSheet1.as_of_date}`);
    console.log(`Total Assets: ${formatCurrency(balanceSheet1.total_assets)}`);
    
    // Method 2: Calculate and validate
    console.log('\nMethod 2: Calculate and Validate');
    const result = await SimpleBalanceSheetCalculator.calculateAndValidate('2024-12-31');
    console.log(`✅ Generated and validated for: ${result.balanceSheet.as_of_date}`);
    console.log(`Valid: ${result.validation.is_valid ? '✅' : '❌'}`);
    console.log(`Balanced: ${result.balanceSheet.is_balanced ? '✅' : '❌'}`);
    
    // Method 3: Console format
    console.log('\nMethod 3: Console Format');
    const formattedOutput = SimpleBalanceSheetCalculator.formatForConsole(result.balanceSheet);
    console.log(formattedOutput);

    return result;
  } catch (error) {
    console.error('❌ Error with simple calculator:', error);
    throw error;
  }
}

/**
 * Example 9: Refresh Account Balances
 * Refresh materialized view untuk mendapatkan data terbaru
 */
export async function example9_RefreshAccountBalances() {
  console.log('\n🧮 Example 9: Refresh Account Balances');
  console.log('=' .repeat(50));

  try {
    console.log('🔄 Refreshing account balances materialized view...');
    
    const refreshResult = await refreshAccountBalances();
    
    console.log('✅ Account balances refreshed successfully!');
    console.log(`Message: ${refreshResult.message}`);
    console.log(`Updated at: ${refreshResult.updated_at}`);
    
    // Generate fresh balance sheet after refresh
    console.log('\n📊 Generating fresh balance sheet...');
    const freshBalanceSheet = await generateQuickBalanceSheet();
    
    console.log('✅ Fresh balance sheet generated!');
    console.log(`Data freshness: ${freshBalanceSheet.metadata.data_freshness}`);
    console.log(`Journal entries: ${freshBalanceSheet.metadata.journal_entries_count}`);
    console.log(`Accounts included: ${freshBalanceSheet.metadata.accounts_included}`);

    return { refreshResult, freshBalanceSheet };
  } catch (error) {
    console.error('❌ Error refreshing account balances:', error);
    throw error;
  }
}

/**
 * Example 10: Complete Balance Sheet Analysis Workflow
 * Workflow lengkap untuk analisis balance sheet
 */
export async function example10_CompleteWorkflow() {
  console.log('\n🧮 Example 10: Complete Balance Sheet Analysis Workflow');
  console.log('=' .repeat(60));

  try {
    console.log('Step 1: Refresh account balances...');
    await refreshAccountBalances();
    
    console.log('\nStep 2: Generate enhanced balance sheet...');
    const enhancedBS = await generateEnhancedBalanceSheet();
    
    console.log('\nStep 3: Validate balance sheet...');
    const validation = await validateBalanceSheet(enhancedBS);
    
    console.log('\nStep 4: Calculate financial ratios...');
    const ratios = calculateBalanceSheetRatios(enhancedBS);
    
    console.log('\nStep 5: Generate summary...');
    const summary = getBalanceSheetSummary(enhancedBS);
    
    console.log('\nStep 6: Export to CSV...');
    const csvData = exportBalanceSheetToCSV(enhancedBS);
    
    // Final Report
    console.log('\n' + '='.repeat(60));
    console.log('📊 COMPLETE BALANCE SHEET ANALYSIS REPORT');
    console.log('='.repeat(60));
    
    console.log(`\n🏢 Company: ${enhancedBS.company_name}`);
    console.log(`📅 As of: ${enhancedBS.as_of_date}`);
    console.log(`⏰ Generated: ${new Date(enhancedBS.generated_at).toLocaleString('id-ID')}`);
    
    console.log(`\n💰 Financial Position:`);
    console.log(`   Assets: ${formatCurrency(enhancedBS.total_assets)}`);
    console.log(`   Liabilities: ${formatCurrency(enhancedBS.total_liabilities)}`);
    console.log(`   Equity: ${formatCurrency(enhancedBS.total_equity)}`);
    console.log(`   Balanced: ${enhancedBS.is_balanced ? '✅' : '❌'}`);
    
    console.log(`\n📈 Key Ratios:`);
    console.log(`   Current Ratio: ${ratios.current_ratio.toFixed(2)}`);
    console.log(`   Debt to Equity: ${ratios.debt_to_equity_ratio.toFixed(2)}`);
    console.log(`   Equity Ratio: ${(ratios.equity_ratio * 100).toFixed(1)}%`);
    console.log(`   Working Capital: ${formatCurrency(ratios.working_capital)}`);
    
    console.log(`\n🔍 Data Quality:`);
    console.log(`   Validation: ${validation.is_valid ? '✅ Valid' : '❌ Invalid'}`);
    console.log(`   Errors: ${validation.errors.length}`);
    console.log(`   Warnings: ${validation.warnings.length}`);
    console.log(`   Source: ${enhancedBS.metadata.source}`);
    console.log(`   Journal Entries: ${enhancedBS.metadata.journal_entries_count}`);
    
    console.log(`\n📁 Export:`);
    console.log(`   CSV Length: ${csvData.length} characters`);
    console.log(`   CSV Lines: ${csvData.split('\n').length}`);
    
    console.log('\n' + '='.repeat(60));
    console.log('✅ ANALYSIS COMPLETED SUCCESSFULLY');
    console.log('='.repeat(60));

    return {
      balanceSheet: enhancedBS,
      validation,
      ratios,
      summary,
      csvData
    };
  } catch (error) {
    console.error('❌ Error in complete workflow:', error);
    throw error;
  }
}

/**
 * Run All Examples
 * Jalankan semua contoh secara berurutan
 */
export async function runAllExamples() {
  console.log('🚀 Running All Balance Sheet Calculator Examples');
  console.log('='.repeat(60));
  
  const results: any[] = [];
  
  try {
    results.push(await example1_BasicBalanceSheet());
    results.push(await example2_CustomOptionsBalanceSheet());
    results.push(await example3_EnhancedBalanceSheet());
    results.push(await example4_BalanceSheetValidation());
    results.push(await example5_BalanceSheetComparison());
    results.push(await example6_ExportToCSV());
    results.push(await example7_BalanceSheetSummary());
    results.push(await example8_SimpleCalculator());
    results.push(await example9_RefreshAccountBalances());
    results.push(await example10_CompleteWorkflow());
    
    console.log('\n' + '='.repeat(60));
    console.log('🎉 ALL EXAMPLES COMPLETED SUCCESSFULLY!');
    console.log(`Total examples run: ${results.length}`);
    console.log('='.repeat(60));
    
    return results;
  } catch (error) {
    console.error('❌ Error running examples:', error);
    throw error;
  }
}

