/**
 * API Documentation & Testing Validation System
 * 
 * Validates API endpoints against Swagger documentation
 * and ensures consistency between frontend and backend
 */

import { API_ENDPOINTS } from '@/config/api';
import { validateAllEndpoints, validateCriticalEndpoints } from './apiValidation';

// Swagger API endpoints based on the documentation
export const SWAGGER_ENDPOINTS = {
  // Admin
  ADMIN_CHECK_CASHBANK_GL: '/api/admin/check-cashbank-gl-links',

  // CashBank (legacy group under v1)
  CASHBANK_ACCOUNTS: '/api/v1/cashbank/accounts',
  CASHBANK_ACCOUNT_BY_ID: (id: number) => `/api/v1/cashbank/accounts/${id}`,
  CASHBANK_ACCOUNT_TRANSACTIONS: (id: number) => `/api/v1/cashbank/accounts/${id}/transactions`,
  CASHBANK_BALANCE_SUMMARY: '/api/v1/cashbank/balance-summary',
  CASHBANK_DEPOSIT: '/api/v1/cashbank/deposit',
  CASHBANK_PAYMENT_ACCOUNTS: '/api/v1/cashbank/payment-accounts',
  CASHBANK_TRANSFER: '/api/v1/cashbank/transfer',
  CASHBANK_WITHDRAWAL: '/api/v1/cashbank/withdrawal',

  // Balance Monitoring
  MONITORING_BALANCE_HEALTH: '/api/monitoring/balance-health',
  MONITORING_BALANCE_SYNC: '/api/monitoring/balance-sync',
  MONITORING_DISCREPANCIES: '/api/monitoring/discrepancies',
  MONITORING_SYNC_STATUS: '/api/monitoring/sync-status',

  // Payment Integration
  PAYMENTS_ACCOUNT_BALANCES_REALTIME: '/api/v1/payments/account-balances/real-time',
  PAYMENTS_ACCOUNT_BALANCES_REFRESH: '/api/v1/payments/account-balances/refresh',
  PAYMENTS_ENHANCED_WITH_JOURNAL: '/api/v1/payments/enhanced-with-journal',
  PAYMENTS_INTEGRATION_METRICS: '/api/v1/payments/integration-metrics',
  PAYMENTS_JOURNAL_ENTRIES: '/api/v1/payments/journal-entries',
  PAYMENTS_PREVIEW_JOURNAL: '/api/v1/payments/preview-journal',
  PAYMENTS_ACCOUNT_UPDATES: (id: number) => `/api/v1/payments/${id}/account-updates`,
  PAYMENTS_REVERSE: (id: number) => `/api/v1/payments/${id}/reverse`,
  PAYMENTS_WITH_JOURNAL: (id: number) => `/api/v1/payments/${id}/with-journal`,

  // Payments
  PAYMENTS_ANALYTICS: '/api/v1/payments/analytics',
  PAYMENTS_DEBUG_RECENT: '/api/v1/payments/debug/recent',
  PAYMENTS_EXPORT_EXCEL: '/api/v1/payments/export/excel',
  PAYMENTS_REPORT_PDF: '/api/v1/payments/report/pdf',
  PAYMENTS_SUMMARY: '/api/v1/payments/summary',
  PAYMENTS_UNPAID_BILLS: (vendorId: number) => `/api/v1/payments/unpaid-bills/${vendorId}`,
  PAYMENTS_UNPAID_INVOICES: (customerId: number) => `/api/v1/payments/unpaid-invoices/${customerId}`,
  PAYMENTS_BY_ID: (id: number) => `/api/v1/payments/${id}`,
  PAYMENTS_CANCEL: (id: number) => `/api/v1/payments/${id}/cancel`,
  PAYMENTS_PDF: (id: number) => `/api/v1/payments/${id}/pdf`,

  // Purchases
  PURCHASES_FOR_PAYMENT: (id: number) => `/api/v1/purchases/${id}/for-payment`,
  PURCHASES_INTEGRATED_PAYMENT: (id: number) => `/api/v1/purchases/${id}/integrated-payment`,
  PURCHASES_PAYMENTS: (id: number) => `/api/v1/purchases/${id}/payments`,

  // Security
  SECURITY_ALERTS: '/api/v1/admin/security/alerts',
  SECURITY_ALERT_ACKNOWLEDGE: (id: number) => `/api/v1/admin/security/alerts/${id}/acknowledge`,
  SECURITY_CLEANUP: '/api/v1/admin/security/cleanup',
  SECURITY_CONFIG: '/api/v1/admin/security/config',
  SECURITY_INCIDENTS: '/api/v1/admin/security/incidents',
  SECURITY_INCIDENT_BY_ID: (id: number) => `/api/v1/admin/security/incidents/${id}`,
  SECURITY_INCIDENT_RESOLVE: (id: number) => `/api/v1/admin/security/incidents/${id}/resolve`,
  SECURITY_IP_WHITELIST: '/api/v1/admin/security/ip-whitelist',
  SECURITY_METRICS: '/api/v1/admin/security/metrics',

  // Journal
  JOURNALS_LIST: '/api/v1/journals',
  JOURNALS_ACCOUNT_BALANCES: '/api/v1/journals/account-balances',
  JOURNALS_REFRESH_BALANCES: '/api/v1/journals/account-balances/refresh',
  JOURNALS_SUMMARY: '/api/v1/journals/summary',
  JOURNALS_BY_ID: (id: number) => `/api/v1/journals/${id}`,

  // Optimized Reports
  REPORTS_OPTIMIZED_BALANCE_SHEET: '/api/v1/reports/optimized/balance-sheet',
  REPORTS_OPTIMIZED_PROFIT_LOSS: '/api/v1/reports/optimized/profit-loss',
  REPORTS_OPTIMIZED_REFRESH_BALANCES: '/api/v1/reports/optimized/refresh-balances',
  REPORTS_OPTIMIZED_TRIAL_BALANCE: '/api/v1/reports/optimized/trial-balance',

  // SSOT Reports
  SSOT_REPORTS_GENERAL_LEDGER: '/api/v1/ssot-reports/general-ledger',
  SSOT_REPORTS_INTEGRATED: '/api/v1/ssot-reports/integrated',
  SSOT_REPORTS_JOURNAL_ANALYSIS: '/api/v1/ssot-reports/journal-analysis',
  SSOT_REPORTS_PURCHASE_REPORT: '/api/v1/ssot-reports/purchase-report',
  SSOT_REPORTS_REFRESH: '/api/v1/ssot-reports/refresh',
  SSOT_REPORTS_SALES_SUMMARY: '/api/v1/ssot-reports/sales-summary',
  SSOT_REPORTS_STATUS: '/api/v1/ssot-reports/status',
  SSOT_REPORTS_TRIAL_BALANCE: '/api/v1/ssot-reports/trial-balance',
  SSOT_REPORTS_VENDOR_ANALYSIS: '/api/v1/ssot-reports/vendor-analysis',
  SSOT_REPORTS_PROFIT_LOSS: '/reports/ssot-profit-loss',
  SSOT_REPORTS_BALANCE_SHEET: '/reports/ssot/balance-sheet',
  SSOT_REPORTS_BALANCE_SHEET_DETAILS: '/reports/ssot/balance-sheet/account-details',
  SSOT_REPORTS_CASH_FLOW: '/reports/ssot/cash-flow',
  SSOT_REPORTS_PURCHASE_VALIDATE: '/api/v1/ssot-reports/purchase-report/validate',
  SSOT_REPORTS_PURCHASE_SUMMARY: '/api/v1/ssot-reports/purchase-summary',

  // Authentication
  AUTH_LOGIN: '/api/v1/auth/login',
  AUTH_REFRESH: '/api/v1/auth/refresh',
  AUTH_REGISTER: '/api/v1/auth/register',
  AUTH_VALIDATE_TOKEN: '/api/v1/auth/validate-token',
  PROFILE: '/api/v1/profile',

  // Dashboard
  DASHBOARD_ANALYTICS: '/api/v1/dashboard/analytics',
  DASHBOARD_FINANCE: '/api/v1/dashboard/finance',

  // Journal Drilldown
  JOURNAL_DRILLDOWN: '/api/v1/journal-drilldown',
  JOURNAL_DRILLDOWN_ACCOUNTS: '/api/v1/journal-drilldown/accounts',
  JOURNAL_DRILLDOWN_ENTRIES: '/api/v1/journal-drilldown/entries',
  JOURNAL_DRILLDOWN_ENTRY_BY_ID: (id: number) => `/api/v1/journal-drilldown/entries/${id}`,

  // Monitoring
  MONITORING_API_ANALYTICS: '/api/v1/monitoring/api-usage/analytics',
  MONITORING_API_RESET: '/api/v1/monitoring/api-usage/reset',
  MONITORING_API_STATS: '/api/v1/monitoring/api-usage/stats',
  MONITORING_API_TOP: '/api/v1/monitoring/api-usage/top',
  MONITORING_API_UNUSED: '/api/v1/monitoring/api-usage/unused'
} as const;

// Comparison interface
interface EndpointComparison {
  frontendPath: string;
  swaggerPath: string;
  status: 'match' | 'mismatch' | 'missing_frontend' | 'missing_swagger';
  suggestions?: string[];
}

/**
 * Extract all endpoint URLs from nested object structure
 */
function extractEndpointPaths(obj: any, prefix: string = ''): Record<string, string> {
  const paths: Record<string, string> = {};
  
  for (const [key, value] of Object.entries(obj)) {
    const fullKey = prefix ? `${prefix}.${key}` : key;
    
    if (typeof value === 'string' && value.startsWith('/')) {
      paths[fullKey] = value;
    } else if (typeof value === 'function') {
      try {
        // Test function endpoints with sample parameters
        const samplePath = value(1);
        if (typeof samplePath === 'string' && samplePath.startsWith('/')) {
          paths[fullKey] = samplePath;
        }
      } catch {
        // Skip functions that can't be called with sample parameters
      }
    } else if (typeof value === 'object' && value !== null) {
      Object.assign(paths, extractEndpointPaths(value, fullKey));
    }
  }
  
  return paths;
}

/**
 * Compare frontend API_ENDPOINTS with Swagger documentation
 */
export function compareEndpointsWithSwagger(): {
  totalChecked: number;
  matches: number;
  mismatches: number;
  missingInFrontend: number;
  missingInSwagger: number;
  comparisons: EndpointComparison[];
  recommendations: string[];
} {
  console.log('🔍 Comparing frontend API_ENDPOINTS with Swagger documentation...');

  const frontendPaths = extractEndpointPaths(API_ENDPOINTS);
  const swaggerPaths = extractEndpointPaths(SWAGGER_ENDPOINTS);
  
  const comparisons: EndpointComparison[] = [];
  const recommendations: string[] = [];
  
  // Check frontend endpoints against swagger
  for (const [key, frontendPath] of Object.entries(frontendPaths)) {
    const swaggerPath = swaggerPaths[key];
    
    if (!swaggerPath) {
      // Look for similar paths in swagger
      const similarSwagger = Object.entries(swaggerPaths).find(
        ([_, path]) => path === frontendPath
      );
      
      if (similarSwagger) {
        comparisons.push({
          frontendPath,
          swaggerPath: similarSwagger[1],
          status: 'match'
        });
      } else {
        comparisons.push({
          frontendPath,
          swaggerPath: 'NOT_FOUND',
          status: 'missing_swagger',
          suggestions: [
            `Add endpoint ${frontendPath} to Swagger documentation`,
            `Verify if ${frontendPath} is implemented in backend`
          ]
        });
      }
    } else if (frontendPath === swaggerPath) {
      comparisons.push({
        frontendPath,
        swaggerPath,
        status: 'match'
      });
    } else {
      comparisons.push({
        frontendPath,
        swaggerPath,
        status: 'mismatch',
        suggestions: [
          `Update frontend path from ${frontendPath} to ${swaggerPath}`,
          `Or update Swagger documentation to match ${frontendPath}`
        ]
      });
    }
  }
  
  // Check swagger endpoints missing in frontend
  for (const [key, swaggerPath] of Object.entries(swaggerPaths)) {
    const frontendPath = frontendPaths[key];
    
    if (!frontendPath) {
      // Check if path exists anywhere in frontend
      const existsInFrontend = Object.values(frontendPaths).includes(swaggerPath);
      
      if (!existsInFrontend) {
        comparisons.push({
          frontendPath: 'NOT_FOUND',
          swaggerPath,
          status: 'missing_frontend',
          suggestions: [
            `Add endpoint ${swaggerPath} to API_ENDPOINTS configuration`,
            `Consider if this endpoint is needed in frontend`
          ]
        });
      }
    }
  }
  
  // Calculate statistics
  const matches = comparisons.filter(c => c.status === 'match').length;
  const mismatches = comparisons.filter(c => c.status === 'mismatch').length;
  const missingInFrontend = comparisons.filter(c => c.status === 'missing_frontend').length;
  const missingInSwagger = comparisons.filter(c => c.status === 'missing_swagger').length;
  
  // Generate recommendations
  if (mismatches > 0) {
    recommendations.push(`Fix ${mismatches} endpoint path mismatches`);
  }
  if (missingInFrontend > 0) {
    recommendations.push(`Consider adding ${missingInFrontend} missing endpoints to frontend`);
  }
  if (missingInSwagger > 0) {
    recommendations.push(`Add ${missingInSwagger} endpoints to Swagger documentation`);
  }
  
  // Log results
  console.log('📊 Endpoint comparison results:');
  console.log(`✅ Matches: ${matches}`);
  console.log(`🔄 Mismatches: ${mismatches}`);
  console.log(`⬅️ Missing in Frontend: ${missingInFrontend}`);
  console.log(`➡️ Missing in Swagger: ${missingInSwagger}`);
  
  if (recommendations.length > 0) {
    console.log('📋 Recommendations:');
    recommendations.forEach(rec => console.log(`  • ${rec}`));
  }
  
  return {
    totalChecked: comparisons.length,
    matches,
    mismatches,
    missingInFrontend,
    missingInSwagger,
    comparisons,
    recommendations
  };
}

/**
 * Generate updated API_ENDPOINTS based on Swagger documentation
 */
export function generateUpdatedAPIEndpoints(): string {
  console.log('🔧 Generating updated API_ENDPOINTS based on Swagger...');
  
  const comparison = compareEndpointsWithSwagger();
  
  // Create updated structure based on Swagger
  const updatedEndpoints = {
    // Authentication (no /api/v1 prefix)
    LOGIN: SWAGGER_ENDPOINTS.AUTH_LOGIN,
    REGISTER: SWAGGER_ENDPOINTS.AUTH_REGISTER,
    REFRESH: SWAGGER_ENDPOINTS.AUTH_REFRESH,
    VALIDATE_TOKEN: SWAGGER_ENDPOINTS.AUTH_VALIDATE_TOKEN,
    PROFILE: SWAGGER_ENDPOINTS.PROFILE,
    
    // Dashboard
    DASHBOARD_ANALYTICS: SWAGGER_ENDPOINTS.DASHBOARD_ANALYTICS,
    DASHBOARD_FINANCE: SWAGGER_ENDPOINTS.DASHBOARD_FINANCE,
    
    // Admin
    ADMIN_CHECK_CASHBANK_GL: SWAGGER_ENDPOINTS.ADMIN_CHECK_CASHBANK_GL,
    ADMIN_FIX_CASHBANK_GL: SWAGGER_ENDPOINTS.ADMIN_FIX_CASHBANK_GL,
    
    // CashBank
    CASHBANK: SWAGGER_ENDPOINTS.CASHBANK_ACCOUNTS,
    CASHBANK_ACCOUNTS: SWAGGER_ENDPOINTS.CASHBANK_ACCOUNTS,
    CASHBANK_ACCOUNT_BY_ID: SWAGGER_ENDPOINTS.CASHBANK_ACCOUNT_BY_ID,
    CASHBANK_ACCOUNT_TRANSACTIONS: SWAGGER_ENDPOINTS.CASHBANK_ACCOUNT_TRANSACTIONS,
    CASHBANK_BALANCE_SUMMARY: SWAGGER_ENDPOINTS.CASHBANK_BALANCE_SUMMARY,
    CASHBANK_DEPOSIT: SWAGGER_ENDPOINTS.CASHBANK_DEPOSIT,
    CASHBANK_PAYMENT_ACCOUNTS: SWAGGER_ENDPOINTS.CASHBANK_PAYMENT_ACCOUNTS,
    CASHBANK_TRANSFER: SWAGGER_ENDPOINTS.CASHBANK_TRANSFER,
    CASHBANK_WITHDRAWAL: SWAGGER_ENDPOINTS.CASHBANK_WITHDRAWAL,
    
    // Balance Monitoring
    MONITORING_BALANCE_HEALTH: SWAGGER_ENDPOINTS.MONITORING_BALANCE_HEALTH,
    MONITORING_BALANCE_SYNC: SWAGGER_ENDPOINTS.MONITORING_BALANCE_SYNC,
    MONITORING_DISCREPANCIES: SWAGGER_ENDPOINTS.MONITORING_DISCREPANCIES,
    MONITORING_SYNC_STATUS: SWAGGER_ENDPOINTS.MONITORING_SYNC_STATUS,
    
    // Payments - Grouped for better organization
    PAYMENTS: {
      LIST: SWAGGER_ENDPOINTS.PAYMENTS_BY_ID(0).replace('/0', ''), // Remove sample ID
      ANALYTICS: SWAGGER_ENDPOINTS.PAYMENTS_ANALYTICS,
      SUMMARY: SWAGGER_ENDPOINTS.PAYMENTS_SUMMARY,
      EXPORT_EXCEL: SWAGGER_ENDPOINTS.PAYMENTS_EXPORT_EXCEL,
      REPORT_PDF: SWAGGER_ENDPOINTS.PAYMENTS_REPORT_PDF,
      BY_ID: SWAGGER_ENDPOINTS.PAYMENTS_BY_ID,
      CANCEL: SWAGGER_ENDPOINTS.PAYMENTS_CANCEL,
      PDF: SWAGGER_ENDPOINTS.PAYMENTS_PDF,
      UNPAID_BILLS: SWAGGER_ENDPOINTS.PAYMENTS_UNPAID_BILLS,
      UNPAID_INVOICES: SWAGGER_ENDPOINTS.PAYMENTS_UNPAID_INVOICES,
      // Payment Integration
      ACCOUNT_BALANCES_REALTIME: SWAGGER_ENDPOINTS.PAYMENTS_ACCOUNT_BALANCES_REALTIME,
      ACCOUNT_BALANCES_REFRESH: SWAGGER_ENDPOINTS.PAYMENTS_ACCOUNT_BALANCES_REFRESH,
      ENHANCED_WITH_JOURNAL: SWAGGER_ENDPOINTS.PAYMENTS_ENHANCED_WITH_JOURNAL,
      INTEGRATION_METRICS: SWAGGER_ENDPOINTS.PAYMENTS_INTEGRATION_METRICS,
      JOURNAL_ENTRIES: SWAGGER_ENDPOINTS.PAYMENTS_JOURNAL_ENTRIES,
      PREVIEW_JOURNAL: SWAGGER_ENDPOINTS.PAYMENTS_PREVIEW_JOURNAL,
      ACCOUNT_UPDATES: SWAGGER_ENDPOINTS.PAYMENTS_ACCOUNT_UPDATES,
      REVERSE: SWAGGER_ENDPOINTS.PAYMENTS_REVERSE,
      WITH_JOURNAL: SWAGGER_ENDPOINTS.PAYMENTS_WITH_JOURNAL,
    },
    
    // Purchases
    PURCHASES: {
      FOR_PAYMENT: SWAGGER_ENDPOINTS.PURCHASES_FOR_PAYMENT,
      INTEGRATED_PAYMENT: SWAGGER_ENDPOINTS.PURCHASES_INTEGRATED_PAYMENT,
      PAYMENTS: SWAGGER_ENDPOINTS.PURCHASES_PAYMENTS,
    },
    
    // Security
    SECURITY: {
      ALERTS: SWAGGER_ENDPOINTS.SECURITY_ALERTS,
      ALERT_ACKNOWLEDGE: SWAGGER_ENDPOINTS.SECURITY_ALERT_ACKNOWLEDGE,
      CLEANUP: SWAGGER_ENDPOINTS.SECURITY_CLEANUP,
      CONFIG: SWAGGER_ENDPOINTS.SECURITY_CONFIG,
      INCIDENTS: SWAGGER_ENDPOINTS.SECURITY_INCIDENTS,
      INCIDENT_BY_ID: SWAGGER_ENDPOINTS.SECURITY_INCIDENT_BY_ID,
      INCIDENT_RESOLVE: SWAGGER_ENDPOINTS.SECURITY_INCIDENT_RESOLVE,
      IP_WHITELIST: SWAGGER_ENDPOINTS.SECURITY_IP_WHITELIST,
      METRICS: SWAGGER_ENDPOINTS.SECURITY_METRICS,
    },
    
    // Journals
    JOURNALS: {
      LIST: SWAGGER_ENDPOINTS.JOURNALS_LIST,
      ACCOUNT_BALANCES: SWAGGER_ENDPOINTS.JOURNALS_ACCOUNT_BALANCES,
      REFRESH_BALANCES: SWAGGER_ENDPOINTS.JOURNALS_REFRESH_BALANCES,
      SUMMARY: SWAGGER_ENDPOINTS.JOURNALS_SUMMARY,
      BY_ID: SWAGGER_ENDPOINTS.JOURNALS_BY_ID,
    },
    
    // Optimized Reports
    REPORTS_OPTIMIZED: {
      BALANCE_SHEET: SWAGGER_ENDPOINTS.REPORTS_OPTIMIZED_BALANCE_SHEET,
      PROFIT_LOSS: SWAGGER_ENDPOINTS.REPORTS_OPTIMIZED_PROFIT_LOSS,
      REFRESH_BALANCES: SWAGGER_ENDPOINTS.REPORTS_OPTIMIZED_REFRESH_BALANCES,
      TRIAL_BALANCE: SWAGGER_ENDPOINTS.REPORTS_OPTIMIZED_TRIAL_BALANCE,
    },
    
    // SSOT Reports
    SSOT_REPORTS: {
      GENERAL_LEDGER: SWAGGER_ENDPOINTS.SSOT_REPORTS_GENERAL_LEDGER,
      INTEGRATED: SWAGGER_ENDPOINTS.SSOT_REPORTS_INTEGRATED,
      JOURNAL_ANALYSIS: SWAGGER_ENDPOINTS.SSOT_REPORTS_JOURNAL_ANALYSIS,
      PURCHASE_REPORT: SWAGGER_ENDPOINTS.SSOT_REPORTS_PURCHASE_REPORT,
      PURCHASE_VALIDATE: SWAGGER_ENDPOINTS.SSOT_REPORTS_PURCHASE_VALIDATE,
      PURCHASE_SUMMARY: SWAGGER_ENDPOINTS.SSOT_REPORTS_PURCHASE_SUMMARY,
      REFRESH: SWAGGER_ENDPOINTS.SSOT_REPORTS_REFRESH,
      SALES_SUMMARY: SWAGGER_ENDPOINTS.SSOT_REPORTS_SALES_SUMMARY,
      STATUS: SWAGGER_ENDPOINTS.SSOT_REPORTS_STATUS,
      TRIAL_BALANCE: SWAGGER_ENDPOINTS.SSOT_REPORTS_TRIAL_BALANCE,
      VENDOR_ANALYSIS: SWAGGER_ENDPOINTS.SSOT_REPORTS_VENDOR_ANALYSIS,
      PROFIT_LOSS: SWAGGER_ENDPOINTS.SSOT_REPORTS_PROFIT_LOSS,
      BALANCE_SHEET: SWAGGER_ENDPOINTS.SSOT_REPORTS_BALANCE_SHEET,
      BALANCE_SHEET_DETAILS: SWAGGER_ENDPOINTS.SSOT_REPORTS_BALANCE_SHEET_DETAILS,
      CASH_FLOW: SWAGGER_ENDPOINTS.SSOT_REPORTS_CASH_FLOW,
    },
    
    // Journal Drilldown
    JOURNAL_DRILLDOWN: {
      ROOT: SWAGGER_ENDPOINTS.JOURNAL_DRILLDOWN,
      ACCOUNTS: SWAGGER_ENDPOINTS.JOURNAL_DRILLDOWN_ACCOUNTS,
      ENTRIES: SWAGGER_ENDPOINTS.JOURNAL_DRILLDOWN_ENTRIES,
      ENTRY_BY_ID: SWAGGER_ENDPOINTS.JOURNAL_DRILLDOWN_ENTRY_BY_ID,
    },
    
    // Monitoring
    MONITORING_API: {
      ANALYTICS: SWAGGER_ENDPOINTS.MONITORING_API_ANALYTICS,
      RESET: SWAGGER_ENDPOINTS.MONITORING_API_RESET,
      STATS: SWAGGER_ENDPOINTS.MONITORING_API_STATS,
      TOP: SWAGGER_ENDPOINTS.MONITORING_API_TOP,
      UNUSED: SWAGGER_ENDPOINTS.MONITORING_API_UNUSED,
    },
  };
  
  // Generate TypeScript code
  const generatedCode = `// Updated API_ENDPOINTS based on Swagger documentation
// Generated on ${new Date().toISOString()}
// Total mismatches resolved: ${comparison.mismatches}
// New endpoints added: ${comparison.missingInFrontend}

export const API_ENDPOINTS = ${JSON.stringify(updatedEndpoints, null, 2)
  .replace(/"(\w+)":/g, '$1:') // Remove quotes from object keys
  .replace(/"/g, "'")          // Use single quotes
  .replace(/\\'/g, "'")}`;      // Fix escaped quotes
  
  console.log('✅ Updated API_ENDPOINTS generated based on Swagger documentation');
  
  return generatedCode;
}

/**
 * Run comprehensive API validation against Swagger
 */
export async function validateAPIAgainstSwagger(): Promise<{
  endpointComparison: ReturnType<typeof compareEndpointsWithSwagger>;
  healthCheck: Awaited<ReturnType<typeof validateCriticalEndpoints>>;
  recommendations: string[];
}> {
  console.log('🚀 Running comprehensive API validation against Swagger...');
  
  // Compare endpoints
  const endpointComparison = compareEndpointsWithSwagger();
  
  // Run health check
  const healthCheck = await validateCriticalEndpoints();
  
  // Generate comprehensive recommendations
  const recommendations: string[] = [
    ...endpointComparison.recommendations,
    ...(healthCheck.allEndpointsValid ? 
      ['✅ All critical endpoints are healthy'] : 
      ['❌ Fix failing critical endpoints: ' + healthCheck.failedEndpoints.join(', ')])
  ];
  
  if (endpointComparison.matches / endpointComparison.totalChecked < 0.8) {
    recommendations.push('🔄 Consider updating API_ENDPOINTS configuration to match Swagger');
  }
  
  if (!healthCheck.allEndpointsValid) {
    recommendations.push('🚨 Fix API connectivity issues before production deployment');
  }
  
  console.log('📋 Validation complete. Check recommendations for next steps.');
  
  return {
    endpointComparison,
    healthCheck,
    recommendations
  };
}

/**
 * Generate production-ready API endpoint validation report
 */
export function generateValidationReport(): string {
  const comparison = compareEndpointsWithSwagger();
  
  const report = `
# API Endpoint Validation Report
Generated: ${new Date().toISOString()}

## Summary
- Total Endpoints: ${comparison.totalChecked}
- Matches: ${comparison.matches}
- Mismatches: ${comparison.mismatches}
- Missing in Frontend: ${comparison.missingInFrontend}
- Missing in Swagger: ${comparison.missingInSwagger}
- Match Rate: ${Math.round((comparison.matches / comparison.totalChecked) * 100)}%

## Detailed Analysis

### ✅ Matching Endpoints
${comparison.comparisons
  .filter(c => c.status === 'match')
  .map(c => `- ${c.frontendPath}`)
  .join('\n')}

### 🔄 Mismatched Endpoints
${comparison.comparisons
  .filter(c => c.status === 'mismatch')
  .map(c => `- Frontend: ${c.frontendPath}\n  Swagger: ${c.swaggerPath}`)
  .join('\n')}

### ⬅️ Missing in Frontend
${comparison.comparisons
  .filter(c => c.status === 'missing_frontend')
  .map(c => `- ${c.swaggerPath}`)
  .join('\n')}

### ➡️ Missing in Swagger
${comparison.comparisons
  .filter(c => c.status === 'missing_swagger')
  .map(c => `- ${c.frontendPath}`)
  .join('\n')}

## Recommendations
${comparison.recommendations.map(r => `- ${r}`).join('\n')}

## Next Steps
1. Update API_ENDPOINTS configuration to match Swagger
2. Fix any missing endpoints in frontend or backend
3. Run health check validation
4. Update tests to cover all endpoints
5. Deploy to production when match rate > 95%
`;
  
  return report;
}
