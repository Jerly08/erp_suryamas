// API Configuration
// For Server-Side Rendering (SSR) in Next.js, we MUST use the full backend URL.
// For Client-Side navigation, we use empty string '' to leverage Next.js proxy rewrites in next.config.ts
const rawApiUrl = process.env.NEXT_PUBLIC_API_URL || "";
const isServer = typeof window === "undefined";

// If running on the server (SSR in Docker), point directly to the backend container
// If running on the client (Browser), point to the Next.js frontend proxy (empty string)
export const API_BASE_URL = isServer
  ? rawApiUrl.endsWith("/")
    ? rawApiUrl.slice(0, -1)
    : rawApiUrl
  : "";

export const API_V1_BASE = `/api/v1`;

// COMPREHENSIVE API Endpoints - Based on Backend Routes Analysis
export const API_ENDPOINTS = {
  // Authentication (with /api/v1 prefix - corrected based on actual backend routes)
  LOGIN: "/api/v1/auth/login",
  REGISTER: "/api/v1/auth/register",
  REFRESH: "/api/v1/auth/refresh",
  LOGOUT: "/api/v1/auth/logout",
  VALIDATE_TOKEN: "/api/v1/auth/validate-token",
  PROFILE: "/api/v1/profile",

  // Auth nested object for services expecting AUTH.* structure
  AUTH: {
    LOGIN: "/api/v1/auth/login",
    REGISTER: "/api/v1/auth/register",
    REFRESH: "/api/v1/auth/refresh",
    LOGOUT: "/api/v1/auth/logout",
    VALIDATE_TOKEN: "/api/v1/auth/validate-token",
    PROFILE: "/api/v1/profile",
    // Note: Ensure backend supports this; keep for compatibility if implemented
    CHANGE_PASSWORD: "/api/v1/auth/change-password",
  },

  // Products (with /api/v1 prefix)
  PRODUCTS: "/api/v1/products",
  PRODUCTS_BY_ID: (id: number) => `/api/v1/products/${id}`,
  PRODUCTS_BACKFILL_FULFILLMENT: (id: number) =>
    `/api/v1/products/${id}/backfill-fulfillment`,
  PRODUCTS_ADJUST_STOCK: "/api/v1/products/adjust-stock",
  PRODUCTS_OPNAME: "/api/v1/products/opname",
  PRODUCTS_UPLOAD_IMAGE: "/api/v1/products/upload-image",
  PRODUCTS_IMPORT: "/api/v1/products/import",

  // Categories (with /api/v1 prefix)
  CATEGORIES: "/api/v1/categories",
  CATEGORIES_TREE: "/api/v1/categories/tree",
  CATEGORIES_BY_ID: (id: number) => `/api/v1/categories/${id}`,
  CATEGORIES_PRODUCTS: (id: number) => `/api/v1/categories/${id}/products`,

  // Product Units (with /api/v1 prefix)
  PRODUCT_UNITS: "/api/v1/product-units",
  PRODUCT_UNITS_BY_ID: (id: number) => `/api/v1/product-units/${id}`,

  // Warehouse Locations (with /api/v1 prefix)
  WAREHOUSE_LOCATIONS: "/api/v1/warehouse-locations",
  WAREHOUSE_LOCATIONS_BY_ID: (id: number) =>
    `/api/v1/warehouse-locations/${id}`,

  // Inventory (with /api/v1 prefix)
  INVENTORY_MOVEMENTS: "/api/v1/inventory/movements",
  INVENTORY_LOW_STOCK: "/api/v1/inventory/low-stock",
  INVENTORY_VALUATION: "/api/v1/inventory/valuation",
  INVENTORY_REPORT: "/api/v1/inventory/report",
  INVENTORY_BULK_PRICE_UPDATE: "/api/v1/inventory/bulk-price-update",

  // Users (with /api/v1 prefix)
  USERS: {
    LIST: "/api/v1/users",
    CREATE: "/api/v1/users",
    GET_BY_ID: (id: number) => `/api/v1/users/${id}`,
    UPDATE: (id: number) => `/api/v1/users/${id}`,
    DELETE: (id: number) => `/api/v1/users/${id}`,
  },

  // Permissions (with /api/v1 prefix)
  PERMISSIONS_ME: "/api/v1/permissions/me",
  PERMISSIONS_USERS: "/api/v1/permissions/users",
  PERMISSIONS_USER_BY_ID: (userId: number) =>
    `/api/v1/permissions/users/${userId}`,
  PERMISSIONS_USER_RESET: (userId: number) =>
    `/api/v1/permissions/users/${userId}/reset`,
  PERMISSIONS_CHECK: "/api/v1/permissions/check",

  // Approval Workflows (with /api/v1 prefix)
  APPROVAL_WORKFLOWS: "/api/v1/approval-workflows",

  // Contacts (with /api/v1 prefix)
  CONTACTS: "/api/v1/contacts",

  // Invoice Types (with /api/v1 prefix)
  INVOICE_TYPES: "/api/v1/invoice-types",
  INVOICE_TYPES_ACTIVE: "/api/v1/invoice-types/active",
  INVOICE_TYPES_BY_ID: (id: number) => `/api/v1/invoice-types/${id}`,
  INVOICE_TYPES_TOGGLE: (id: number) => `/api/v1/invoice-types/${id}/toggle`,
  INVOICE_TYPES_PREVIEW_NUMBER: "/api/v1/invoice-types/preview-number",
  INVOICE_TYPES_COUNTER_HISTORY: (id: number) =>
    `/api/v1/invoice-types/${id}/counter-history`,
  INVOICE_TYPES_RESET_COUNTER: (id: number) =>
    `/api/v1/invoice-types/${id}/reset-counter`,

  // Sales (with /api/v1 prefix)
  SALES: "/api/v1/sales",
  SALES_BY_ID: (id: number) => `/api/v1/sales/${id}`,
  SALES_CONVERT_CONFIRMED_TO_CREDIT: (id: number) =>
    `/api/v1/sales/${id}/convert-to-credit`,
  SALES_CONFIRM: (id: number) => `/api/v1/sales/${id}/confirm`,
  SALES_INVOICE: (id: number) => `/api/v1/sales/${id}/invoice`,
  SALES_CANCEL: (id: number) => `/api/v1/sales/${id}/cancel`,
  SALES_PAYMENTS: (id: number) => `/api/v1/sales/${id}/payments`,
  SALES_PPH23_ADD: (id: number) => `/api/v1/sales/${id}/pph23`,
  SALES_PPH23_SETTLEMENT: (id: number) =>
    `/api/v1/sales/${id}/pph23/settlement`,
  SALES_FOR_PAYMENT: (id: number) => `/api/v1/sales/${id}/for-payment`,
  SALES_INTEGRATED_PAYMENT: (id: number) =>
    `/api/v1/sales/${id}/integrated-payment`,
  SALES_RETURNS: (id: number) => `/api/v1/sales/${id}/returns`,
  SALES_ALL_RETURNS: "/api/v1/sales/returns",
  SALES_SUMMARY: "/api/v1/sales/summary",
  SALES_ANALYTICS: "/api/v1/sales/analytics",
  SALES_RECEIVABLES: "/api/v1/sales/receivables",
  SALES_INVOICE_PDF: (id: number) => `/api/v1/sales/${id}/invoice/pdf`,
  SALES_RECEIPT_PDF: (id: number) => `/api/v1/sales/${id}/receipt/pdf`,
  SALES_REPORT_PDF: "/api/v1/sales/report/pdf",
  SALES_REPORT_CSV: "/api/v1/sales/report/csv",
  SALES_CUSTOMER: (customerId: number) =>
    `/api/v1/sales/customer/${customerId}`,
  SALES_CUSTOMER_INVOICES: (customerId: number) =>
    `/api/v1/sales/customer/${customerId}/invoices`,

  // Accounts (with /api/v1 prefix) - with nested structure
  ACCOUNTS: {
    LIST: "/api/v1/accounts",
    CREATE: "/api/v1/accounts",
    HIERARCHY: "/api/v1/accounts/hierarchy",
    BALANCE_SUMMARY: "/api/v1/accounts/balance-summary",
    VALIDATE_CODE: "/api/v1/accounts/validate-code",
    FIX_HEADER_STATUS: "/api/v1/accounts/fix-header-status",
    RECALCULATE_PARENT_BALANCES: "/api/v1/accounts/recalculate-parent-balances",
    GET_BY_CODE: (code: string) => `/api/v1/accounts/${code}`,
    UPDATE: (code: string) => `/api/v1/accounts/${code}`,
    DELETE: (code: string) => `/api/v1/accounts/${code}`,
    ADMIN_DELETE: (code: string) => `/api/v1/accounts/admin/${code}`,
    IMPORT: "/api/v1/accounts/import",
    TEMPLATE: "/api/v1/accounts/template",
    EXPORT: {
      PDF: "/api/v1/accounts/export/pdf",
      EXCEL: "/api/v1/accounts/export/excel",
    },
    CATALOG: "/api/v1/accounts/catalog", // Authenticated lookup
    CREDIT: "/api/v1/accounts/credit", // Authenticated lookup
  },

  // SSOT posted-only COA balances
  COA_POSTED_BALANCES: "/api/v1/coa/posted-balances",
  // Legacy flat endpoints for backward compatibility
  ACCOUNTS_LIST: "/api/v1/accounts",
  ACCOUNTS_CREATE: "/api/v1/accounts",
  ACCOUNTS_HIERARCHY: "/api/v1/accounts/hierarchy",
  ACCOUNTS_BALANCE_SUMMARY: "/api/v1/accounts/balance-summary",
  ACCOUNTS_VALIDATE_CODE: "/api/v1/accounts/validate-code",
  ACCOUNTS_FIX_HEADER_STATUS: "/api/v1/accounts/fix-header-status",
  ACCOUNTS_BY_CODE: (code: string) => `/api/v1/accounts/${code}`,
  ACCOUNTS_ADMIN_DELETE: (code: string) => `/api/v1/accounts/admin/${code}`,
  ACCOUNTS_IMPORT: "/api/v1/accounts/import",
  ACCOUNTS_EXPORT_PDF: "/api/v1/accounts/export/pdf",
  ACCOUNTS_EXPORT_EXCEL: "/api/v1/accounts/export/excel",
  ACCOUNTS_CATALOG: "/api/v1/accounts/catalog", // Authenticated lookup
  ACCOUNTS_CREDIT: "/api/v1/accounts/credit", // Authenticated lookup

  // CASH_BANK endpoints aligned with backend routes under /api/v1
  CASH_BANK: {
    // Accounts CRUD
    ACCOUNTS: "/api/v1/cash-bank/accounts",
    GET_BY_ID: (id: number) => `/api/v1/cash-bank/accounts/${id}`,
    ACCOUNT_BY_ID: (id: number) => `/api/v1/cash-bank/accounts/${id}`,
    CREATE: "/api/v1/cash-bank/accounts",
    UPDATE: (id: number) => `/api/v1/cash-bank/accounts/${id}`,
    DELETE: (id: number) => `/api/v1/cash-bank/accounts/${id}`,

    // Transactions & history
    TRANSACTIONS: (id: number) =>
      `/api/v1/cash-bank/accounts/${id}/transactions`,
    ACCOUNT_TRANSACTIONS: (id: number) =>
      `/api/v1/cash-bank/accounts/${id}/transactions`,

    // Dropdowns / Lookups
    PAYMENT_ACCOUNTS: "/api/v1/cash-bank/reports/payment-accounts",
    REVENUE_ACCOUNTS: "/api/v1/cash-bank/revenue-accounts",
    DEPOSIT_SOURCE_ACCOUNTS: "/api/v1/cash-bank/deposit-source-accounts",
    REFERENCE_RATE: "/api/v1/cash-bank/reference-rate",

    // Summaries & reports
    BALANCE_SUMMARY: "/api/v1/cash-bank/reports/balance-summary",

    // Operations (use SSOT transaction routes to avoid 404 on some environments)
    TRANSFER: "/api/v1/cash-bank/transactions/transfer",
    DEPOSIT: "/api/v1/cash-bank/transactions/deposit",
    WITHDRAWAL: "/api/v1/cash-bank/transactions/withdrawal",
    TRANSFERS: "/api/v1/cash-bank/transfers",
    APPROVE_TRANSFER: (id: number) =>
      `/api/v1/cash-bank/transfers/${id}/approve`,
    REJECT_TRANSFER: (id: number) => `/api/v1/cash-bank/transfers/${id}/reject`,

    // Reconciliation (available under /cash-bank group)
    RECONCILE: (id: number) => `/api/v1/cash-bank/accounts/${id}/reconcile`,

    // Frozen Fund Management
    FREEZE: (id: number) => `/api/v1/cash-bank/accounts/${id}/freeze`,
    UNFREEZE: (id: number) => `/api/v1/cash-bank/accounts/${id}/unfreeze`,
    FROZEN_HISTORY: (id: number) =>
      `/api/v1/cash-bank/accounts/${id}/frozen-history`,

    // Admin diagnostics
    CHECK_GL_LINKS: "/api/v1/admin/check-cashbank-gl-links",
  },

  // Cash Bank SSOT Routes (with /api/v1 prefix)
  CASH_BANK_SSOT_ACCOUNTS: "/api/v1/cash-bank/accounts",
  CASH_BANK_SSOT_ACCOUNT_BY_ID: (id: number) =>
    `/api/v1/cash-bank/accounts/${id}`,
  CASH_BANK_SSOT_ACCOUNT_TRANSACTIONS: (id: number) =>
    `/api/v1/cash-bank/accounts/${id}/transactions`,
  CASH_BANK_SSOT_ACCOUNT_RECONCILE: (id: number) =>
    `/api/v1/cash-bank/accounts/${id}/reconcile`,
  CASH_BANK_SSOT_DEPOSIT: "/api/v1/cash-bank/transactions/deposit",
  CASH_BANK_SSOT_WITHDRAWAL: "/api/v1/cash-bank/transactions/withdrawal",
  CASH_BANK_SSOT_TRANSFER: "/api/v1/cash-bank/transactions/transfer",
  CASH_BANK_SSOT_BALANCE_SUMMARY: "/api/v1/cash-bank/reports/balance-summary",
  CASH_BANK_SSOT_PAYMENT_ACCOUNTS: "/api/v1/cash-bank/reports/payment-accounts",
  CASH_BANK_SSOT_JOURNALS: "/api/v1/cash-bank/ssot/journals",
  CASH_BANK_SSOT_VALIDATE: "/api/v1/cash-bank/ssot/validate-integrity",

  // Admin (with /api/v1 prefix)
  ADMIN_CHECK_CASHBANK_GL: "/api/v1/admin/check-cashbank-gl-links",
  // Balance Monitoring (with /api/v1 prefix)
  // These are duplicates - commenting out since they're already defined above
  // MONITORING_BALANCE_HEALTH: '/api/v1/monitoring/balance-health',
  // MONITORING_BALANCE_SYNC: '/api/v1/monitoring/balance-sync',
  // MONITORING_DISCREPANCIES: '/api/v1/monitoring/discrepancies',
  // MONITORING_SYNC_STATUS: '/api/v1/monitoring/sync-status',

  // API Usage Monitoring (with /api/v1 prefix)
  MONITORING_API_ANALYTICS: "/api/v1/monitoring/api-usage/analytics",
  MONITORING_API_STATS: "/api/v1/monitoring/api-usage/stats",
  MONITORING_API_TOP: "/api/v1/monitoring/api-usage/top",
  MONITORING_API_UNUSED: "/api/v1/monitoring/api-usage/unused",
  MONITORING_API_RESET: "/api/v1/monitoring/api-usage/reset",

  // Payments (with /api/v1 prefix based on backend routes) - with nested structure
  PAYMENTS: {
    LIST: "/api/v1/payments",
    CREATE: "/api/v1/payments",
    // Prefer SSOT detail endpoint for GET by ID as legacy GET may be disabled
    BY_ID: (id: number) => `/api/v1/payments/ssot/${id}`,
    GET_BY_ID: (id: number) => `/api/v1/payments/ssot/${id}`,
    CANCEL: (id: number) => `/api/v1/payments/${id}/cancel`,
    DELETE: (id: number | string) => `/api/v1/payments/${id}`,
    PDF: (id: number) => `/api/v1/payments/${id}/pdf`,
    ANALYTICS: "/api/v1/payments/analytics",
    SUMMARY: "/api/v1/payments/summary",
    UNPAID_BILLS: (vendorId: number) =>
      `/api/v1/payments/unpaid-bills/${vendorId}`,
    // Align with backend route: /api/v1/payments/sales/unpaid-invoices/:customer_id
    UNPAID_INVOICES: (customerId: number) =>
      `/api/v1/payments/sales/unpaid-invoices/${customerId}`,
    EXPORT_EXCEL: "/api/v1/payments/export/excel",
    REPORT: {
      PDF: "/api/v1/payments/report/pdf",
    },
    EXPORT: "/api/v1/payments/export", // If not implemented, callers should use EXPORT_EXCEL or REPORT.PDF
    GENERATE_REPORT: (reportType: string) =>
      `/api/v1/payments/report/${reportType}`,
    BULK: "/api/v1/payments/bulk",
    // SSOT endpoints (for creation and detail with journal integration)
    SSOT: {
      LIST: "/api/v1/payments/ssot",
      TRANSACTIONS: "/api/v1/payments/ssot/transactions",
      RECEIVABLE: "/api/v1/payments/ssot/receivable",
      PAYABLE: "/api/v1/payments/ssot/payable",
      EXPENSE: "/api/v1/payments/ssot/expense",
      GET_BY_ID: (id: number) => `/api/v1/payments/ssot/${id}`,
      REVERSE: (id: number) => `/api/v1/payments/ssot/${id}/reverse`,
      REJECT: (id: number) => `/api/v1/payments/ssot/${id}/reject`,
      PREVIEW_JOURNAL: "/api/v1/payments/ssot/preview-journal",
      BALANCE_UPDATES: (id: number) =>
        `/api/v1/payments/ssot/${id}/balance-updates`,
    },
  },
  // Legacy flat endpoints for backward compatibility
  PAYMENTS_ANALYTICS: "/api/v1/payments/analytics",
  PAYMENTS_SUMMARY: "/api/v1/payments/summary",
  PAYMENTS_UNPAID_BILLS: (vendorId: number) =>
    `/api/v1/payments/unpaid-bills/${vendorId}`,
  // Align with backend route: /api/v1/payments/sales/unpaid-invoices/:customer_id
  PAYMENTS_UNPAID_INVOICES: (customerId: number) =>
    `/api/v1/payments/sales/unpaid-invoices/${customerId}`,
  PAYMENTS_EXPORT_EXCEL: "/api/v1/payments/export/excel",
  PAYMENTS_REPORT_PDF: "/api/v1/payments/report/pdf",
  PAYMENTS_BY_ID: (id: number) => `/api/v1/payments/${id}`,
  PAYMENTS_CANCEL: (id: number) => `/api/v1/payments/${id}/cancel`,
  PAYMENTS_PDF: (id: number) => `/api/v1/payments/${id}/pdf`,

  // Payment Integration (with /api/v1 prefix)
  PAYMENTS_ACCOUNT_BALANCES: "/api/v1/payments/account-balances/real-time",
  PAYMENTS_REFRESH_BALANCES: "/api/v1/payments/account-balances/refresh",
  PAYMENTS_ENHANCED: "/api/v1/payments/enhanced-with-journal",
  PAYMENTS_INTEGRATION_METRICS: "/api/v1/payments/integration-metrics",
  PAYMENTS_JOURNAL_ENTRIES: "/api/v1/payments/journal-entries",
  PAYMENTS_PREVIEW_JOURNAL: "/api/v1/payments/preview-journal",
  PAYMENTS_ACCOUNT_UPDATES: (id: number) =>
    `/api/v1/payments/${id}/account-updates`,
  PAYMENTS_REVERSE: (id: number) => `/api/v1/payments/${id}/reverse`,
  PAYMENTS_WITH_JOURNAL: (id: number) => `/api/v1/payments/${id}/with-journal`,

  // Posted transaction correction workflow
  POSTED_TRANSACTION_CORRECTIONS: {
    LIST: "/api/v1/posted-transaction-corrections",
    CREATE: "/api/v1/posted-transaction-corrections",
    BY_ID: (id: number) => `/api/v1/posted-transaction-corrections/${id}`,
    ACCOUNTING_REVIEW: (id: number) =>
      `/api/v1/posted-transaction-corrections/${id}/accounting-review`,
    FINANCE_MANAGER_DECISION: (id: number) =>
      `/api/v1/posted-transaction-corrections/${id}/finance-manager-decision`,
    COMPLETE: (id: number) =>
      `/api/v1/posted-transaction-corrections/${id}/complete`,
    SALES_PAYMENT_CANDIDATES: (saleId: number) =>
      `/api/v1/posted-transaction-corrections/sales/${saleId}/payment-candidates`,
    PURCHASE_COA_RECLASSIFICATION_CANDIDATES: (purchaseId: number) =>
      `/api/v1/posted-transaction-corrections/purchases/${purchaseId}/coa-reclassification-candidates`,
    COA_RECLASSIFICATION_ACCOUNTS:
      "/api/v1/posted-transaction-corrections/coa-reclassification-accounts",
    EXECUTE_SALES_CREDIT_CORRECTION: (id: number) =>
      `/api/v1/posted-transaction-corrections/${id}/execute-sales-credit-correction`,
    DOCUMENTS: (id: number) =>
      `/api/v1/posted-transaction-corrections/${id}/documents`,
    DOCUMENT_DOWNLOAD: (documentId: number) =>
      `/api/v1/posted-transaction-corrections/documents/${documentId}/download`,
  },

  // Security (with /api/v1 prefix)
  SECURITY_ALERTS: "/api/v1/admin/security/alerts",
  SECURITY_ALERT_ACKNOWLEDGE: (id: number) =>
    `/api/v1/admin/security/alerts/${id}/acknowledge`,
  SECURITY_CLEANUP: "/api/v1/admin/security/cleanup",
  SECURITY_CONFIG: "/api/v1/admin/security/config",
  SECURITY_INCIDENTS: "/api/v1/admin/security/incidents",
  SECURITY_INCIDENT_BY_ID: (id: number) =>
    `/api/v1/admin/security/incidents/${id}`,
  SECURITY_INCIDENT_RESOLVE: (id: number) =>
    `/api/v1/admin/security/incidents/${id}/resolve`,
  SECURITY_IP_WHITELIST: "/api/v1/admin/security/ip-whitelist",
  SECURITY_METRICS: "/api/v1/admin/security/metrics",

  // Journal (with /api/v1 prefix)
  JOURNALS: {
    LIST: "/api/v1/journals",
    CREATE: "/api/v1/journals",
    GET_BY_ID: (id: number) => `/api/v1/journals/${id}`,
    UPDATE: (id: number) => `/api/v1/journals/${id}`,
    DELETE: (id: number) => `/api/v1/journals/${id}`,
    POST: (id: number) => `/api/v1/journals/${id}/post`,
    REVERSE: (id: number) => `/api/v1/journals/${id}/reverse`,
    ADJUSTMENT_CATEGORIES: "/api/v1/journals/adjustment-categories",
    SUMMARY: "/api/v1/journals/summary",
    ACCOUNT_BALANCES: "/api/v1/journals/account-balances",
    REFRESH_ACCOUNT_BALANCES: "/api/v1/journals/account-balances/refresh",
  },
  // Legacy flat keys (kept for backward compatibility in other parts of the app)
  JOURNALS_ACCOUNT_BALANCES: "/api/v1/journals/account-balances",
  JOURNALS_REFRESH_BALANCES: "/api/v1/journals/account-balances/refresh",
  JOURNALS_SUMMARY: "/api/v1/journals/summary",
  JOURNALS_BY_ID: (id: number) => `/api/v1/journals/${id}`,

  // Optimized Reports (with /api/v1 prefix)
  REPORTS_OPTIMIZED_BALANCE_SHEET: "/api/v1/reports/optimized/balance-sheet",
  REPORTS_OPTIMIZED_PROFIT_LOSS: "/api/v1/reports/optimized/profit-loss",
  REPORTS_OPTIMIZED_TRIAL_BALANCE: "/api/v1/reports/optimized/trial-balance",
  REPORTS_OPTIMIZED_REFRESH_BALANCES:
    "/api/v1/reports/optimized/refresh-balances",

  // SSOT Reports (with /api/v1 prefix)
  // Keep flat keys for backward compatibility
  SSOT_REPORTS_GENERAL_LEDGER: "/api/v1/ssot-reports/general-ledger",
  SSOT_REPORTS_INTEGRATED: "/api/v1/ssot-reports/integrated",
  SSOT_REPORTS_JOURNAL_ANALYSIS: "/api/v1/ssot-reports/journal-analysis",
  SSOT_REPORTS_PROFIT_LOSS: "/api/v1/reports/ssot-profit-loss",
  SSOT_REPORTS_PURCHASE_REPORT: "/api/v1/ssot-reports/purchase-report",
  SSOT_REPORTS_PURCHASE_VALIDATE:
    "/api/v1/ssot-reports/purchase-report/validate",
  SSOT_REPORTS_PURCHASE_SUMMARY: "/api/v1/ssot-reports/purchase-summary",
  SSOT_REPORTS_REFRESH: "/api/v1/ssot-reports/refresh",
  SSOT_REPORTS_SALES_SUMMARY: "/api/v1/ssot-reports/sales-summary",
  SSOT_REPORTS_STATUS: "/api/v1/ssot-reports/status",
  SSOT_REPORTS_TRIAL_BALANCE: "/api/v1/ssot-reports/trial-balance",
  SSOT_REPORTS_VENDOR_ANALYSIS: "/api/v1/ssot-reports/vendor-analysis",

  // NEW: Nested SSOT_REPORTS object to match service usage
  SSOT_REPORTS: {
    INTEGRATED: "/api/v1/ssot-reports/integrated",
    REFRESH: "/api/v1/ssot-reports/refresh",
    STATUS: "/api/v1/ssot-reports/status",

    // Financial reports
    SALES_SUMMARY: "/api/v1/ssot-reports/sales-summary",
    SALES_SUMMARY_EXPORT: "/api/v1/ssot-reports/sales-summary/export",
    PURCHASE_REPORT: "/api/v1/ssot-reports/purchase-report",
    PURCHASE_SUMMARY: "/api/v1/ssot-reports/purchase-summary",
    PURCHASE_VALIDATE: "/api/v1/ssot-reports/purchase-report/validate",
    PROFIT_LOSS: "/api/v1/reports/ssot-profit-loss",

    TRIAL_BALANCE: "/api/v1/ssot-reports/trial-balance",
    GENERAL_LEDGER: "/api/v1/ssot-reports/general-ledger",
    JOURNAL_ANALYSIS: "/api/v1/ssot-reports/journal-analysis",
    VENDOR_ANALYSIS: "/api/v1/ssot-reports/vendor-analysis",

    // Balance Sheet - matches backend: ssot_report_routes.go registers under /ssot-reports group
    BALANCE_SHEET: "/api/v1/ssot-reports/balance-sheet",
    BALANCE_SHEET_ARCHIVE: "/api/v1/ssot-reports/balance-sheet/archive",
    BALANCE_SHEET_DETAILS: "/api/v1/ssot-reports/balance-sheet/account-details",
    BALANCE_SHEET_VALIDATE: "/api/v1/ssot-reports/balance-sheet/validate",
    BALANCE_SHEET_COMPARISON: "/api/v1/ssot-reports/balance-sheet/comparison",

    // Cash Flow - matches backend: ssot_report_routes.go registers under /ssot-reports group
    CASH_FLOW: "/api/v1/ssot-reports/cash-flow",
    CASH_FLOW_SUMMARY: "/api/v1/ssot-reports/cash-flow/summary",
    CASH_FLOW_VALIDATE: "/api/v1/ssot-reports/cash-flow/validate",

    // Account balances for COA sync
    ACCOUNT_BALANCES: "/api/v1/ssot-reports/account-balances",
  },

  // Journal Drilldown (with /api/v1 prefix - unified)
  JOURNAL_DRILLDOWN: "/api/v1/journal-drilldown",
  JOURNAL_DRILLDOWN_ENTRIES: "/api/v1/journal-drilldown/entries",
  JOURNAL_DRILLDOWN_ENTRY_BY_ID: (id: number) =>
    `/api/v1/journal-drilldown/entries/${id}`,
  JOURNAL_DRILLDOWN_ACCOUNTS: "/api/v1/journal-drilldown/accounts",

  // 🔔 Notification endpoints
  NOTIFICATIONS: "/api/v1/notifications",
  NOTIFICATIONS_SUMMARY: "/api/v1/notifications/summary",
  NOTIFICATIONS_APPROVALS: "/api/v1/notifications/approvals",
  NOTIFICATIONS_BY_TYPE: (type: string) => `/api/v1/notifications/type/${type}`,
  NOTIFICATIONS_DELETE: (id: number) => `/api/v1/notifications/${id}`,
  NOTIFICATIONS_MARK_READ: (id: number) => `/api/v1/notifications/${id}/read`,
  NOTIFICATIONS_UNREAD_COUNT: "/api/v1/notifications/unread-count",

  // 🛒 Purchases endpoints (with /api/v1 prefix)
  PURCHASES: "/api/v1/purchases",
  PURCHASES_VENDORS: "/api/v1/purchases/vendors",
  PURCHASES_BY_ID: (id: number) => `/api/v1/purchases/${id}`,
  PURCHASES_PENDING_APPROVAL: "/api/v1/purchases/pending-approval",
  PURCHASES_APPROVE: (id: number) => `/api/v1/purchases/${id}/approve`,
  PURCHASES_REJECT: (id: number) => `/api/v1/purchases/${id}/reject`,
  PURCHASES_ACCOUNTING_COA: (id: number) =>
    `/api/v1/purchases/${id}/accounting-coa`,
  PURCHASES_COA_REVISIONS: (id: number) =>
    `/api/v1/purchases/${id}/coa-revisions`,
  PURCHASES_COA_REVISION_DECISION: (id: number, revisionId: number) =>
    `/api/v1/purchases/${id}/coa-revisions/${revisionId}/decision`,
  PURCHASES_FINAL_REVIEW_CONFIG: (id: number) =>
    `/api/v1/purchases/${id}/final-review-config`,
  PURCHASES_FINAL_APPROVAL: (id: number) =>
    `/api/v1/purchases/${id}/final-approval`,
  PURCHASES_APPROVAL_HISTORY: (id: number) =>
    `/api/v1/purchases/${id}/approval-history`,
  PURCHASES_APPROVAL_STATS: "/api/v1/purchases/approval-stats",
  PURCHASES_SUBMIT_APPROVAL: (id: number) =>
    `/api/v1/purchases/${id}/submit-approval`,
  PURCHASES_SUMMARY: "/api/v1/purchases/summary",
  PURCHASES_FOR_PAYMENT: (id: number) => `/api/v1/purchases/${id}/for-payment`,
  PURCHASES_INTEGRATED_PAYMENT: (id: number) =>
    `/api/v1/purchases/${id}/integrated-payment`,
  PURCHASES_PAYMENTS: (id: number) => `/api/v1/purchases/${id}/payments`,
  PURCHASES_PAYMENT_PROOF: (id: number) =>
    `/api/v1/purchases/${id}/payment-proof`,
  PURCHASES_PAYMENT_PROOF_FOR_PAYMENT: (
    purchaseId: number,
    paymentId: number,
  ) => `/api/v1/purchases/${purchaseId}/payments/${paymentId}/payment-proof`,
  PURCHASES_PAYMENT_PROOF_DOWNLOAD: (paymentId: number) =>
    `/api/v1/purchases/payment-proofs/${paymentId}/download`,
  PURCHASES_DOCUMENT_DOWNLOAD: (documentId: number) =>
    `/api/v1/purchases/documents/${documentId}/download`,
  PURCHASES_PAYMENT_PROOF_REVIEW: (paymentId: number) =>
    `/api/v1/purchases/payment-proofs/${paymentId}/review`,
  PURCHASE_REQUEST_PAYMENT_PROOF_DOWNLOAD: (paymentId: number) =>
    `/api/v1/purchase-requests/payment-proofs/${paymentId}/download`,
  PURCHASE_REQUEST_DOCUMENT_DOWNLOAD: (documentId: number) =>
    `/api/v1/purchase-requests/documents/${documentId}/download`,
  PAYROLL_DOCUMENT_DOWNLOAD: (documentId: number) =>
    `/api/v1/payroll/documents/${documentId}/download`,
  PAYROLL_EXPORT_EXCEL: (id: number) => `/api/v1/payroll/${id}/export/excel`,
  PURCHASES_PAY_PPH: (id: number) => `/api/v1/purchases/${id}/pay-pph`,
  PURCHASES_EXPORT_PDF: "/api/v1/purchases/export/pdf",
  PURCHASES_EXPORT_CSV: "/api/v1/purchases/export/csv",
  PURCHASES_RECEIPTS_BY_ID: (id: number) => `/api/v1/purchases/${id}/receipts`,
  PURCHASES_RECEIPT_PDF: (id: number) => `/api/v1/purchases/receipts/${id}/pdf`,
  PURCHASES_ALL_RECEIPTS_PDF: (id: number) =>
    `/api/v1/purchases/${id}/receipts/pdf`,

  // 🏢 Assets endpoints (with /api/v1 prefix)
  ASSETS: {
    LIST: "/api/v1/assets",
    CREATE: "/api/v1/assets",
    GET_BY_ID: (id: number) => `/api/v1/assets/${id}`,
    UPDATE: (id: number) => `/api/v1/assets/${id}`,
    DELETE: (id: number) => `/api/v1/assets/${id}`,
    SUMMARY: "/api/v1/assets/summary",
    RUN_DEPRECIATION: "/api/v1/assets/run-depreciation",
    DEPRECIATION_HISTORY: "/api/v1/assets/depreciation-history",
    DEPRECIATION_HISTORY_DETAIL: (period: string) =>
      `/api/v1/assets/depreciation-history/${period}`,
    DEPRECIATION_REPORT: "/api/v1/assets/depreciation-report",
    DEPRECIATION_SCHEDULE: (id: number) =>
      `/api/v1/assets/${id}/depreciation-schedule`,
    CALCULATE_DEPRECIATION: (id: number) =>
      `/api/v1/assets/${id}/calculate-depreciation`,
    UPLOAD_IMAGE: "/api/v1/assets/upload-image",
    CATEGORIES: {
      LIST: "/api/v1/assets/categories",
      CREATE: "/api/v1/assets/categories",
    },
  },

  // 📊 Dashboard endpoints
  DASHBOARD_ANALYTICS: "/api/v1/dashboard/analytics",
  DASHBOARD_FINANCE: "/api/v1/dashboard/finance",
  DASHBOARD_FINANCE_STAFF: "/api/v1/dashboard/finance-staff",
  DASHBOARD_EMPLOYEE: "/api/v1/dashboard/employee",
  DASHBOARD_EMPLOYEE_WORKFLOWS: "/api/v1/dashboard/employee/workflows",
  DASHBOARD_EMPLOYEE_PURCHASE_REQUESTS:
    "/api/v1/dashboard/employee/purchase-requests",
  DASHBOARD_EMPLOYEE_APPROVAL_NOTIFICATIONS:
    "/api/v1/dashboard/employee/approval-notifications",
  DASHBOARD_EMPLOYEE_PURCHASE_APPROVAL_STATUS:
    "/api/v1/dashboard/employee/purchase-approval-status",
  DASHBOARD_EMPLOYEE_NOTIFICATIONS_SUMMARY:
    "/api/v1/dashboard/employee/notifications-summary",
  DASHBOARD_EMPLOYEE_NOTIFICATIONS_READ: (id: number) =>
    `/api/v1/dashboard/employee/notifications/${id}/read`,
  DASHBOARD_STOCK_ALERTS: "/api/v1/dashboard/stock-alerts",
  DASHBOARD_STOCK_ALERTS_DISMISS: (id: number) =>
    `/api/v1/dashboard/stock-alerts/${id}/dismiss`,

  // Monitoring & Admin (with /api/v1 prefix)
  MONITORING_STATUS: "/api/v1/monitoring/status",
  MONITORING_RATE_LIMITS: "/api/v1/monitoring/rate-limits",
  MONITORING_SECURITY_ALERTS: "/api/v1/monitoring/security-alerts",
  MONITORING_AUDIT_LOGS: "/api/v1/monitoring/audit-logs",
  MONITORING_TOKEN_STATS: "/api/v1/monitoring/token-stats",
  MONITORING_REFRESH_EVENTS: "/api/v1/monitoring/refresh-events",
  MONITORING_USER_SECURITY: (userId: number) =>
    `/api/v1/monitoring/users/${userId}/security-summary`,
  MONITORING_STARTUP_STATUS: "/api/v1/monitoring/startup-status",
  MONITORING_FIX_ACCOUNT_HEADERS: "/api/v1/monitoring/fix-account-headers",
  MONITORING_BALANCE_SYNC: "/api/v1/monitoring/balance-sync",
  MONITORING_BALANCE_HEALTH: "/api/v1/monitoring/balance-health",
  MONITORING_DISCREPANCIES: "/api/v1/monitoring/discrepancies",
  MONITORING_SYNC_STATUS: "/api/v1/monitoring/sync-status",
  MONITORING_API_USAGE_STATS: "/api/v1/monitoring/api-usage/stats",
  MONITORING_API_USAGE_TOP: "/api/v1/monitoring/api-usage/top",
  MONITORING_API_USAGE_UNUSED: "/api/v1/monitoring/api-usage/unused",
  MONITORING_API_USAGE_ANALYTICS: "/api/v1/monitoring/api-usage/analytics",
  MONITORING_API_USAGE_RESET: "/api/v1/monitoring/api-usage/reset",
  MONITORING_PERFORMANCE_REPORT: "/api/v1/monitoring/performance/report",
  MONITORING_PERFORMANCE_METRICS: "/api/v1/monitoring/performance/metrics",
  MONITORING_PERFORMANCE_BOTTLENECKS:
    "/api/v1/monitoring/performance/bottlenecks",
  MONITORING_PERFORMANCE_RECOMMENDATIONS:
    "/api/v1/monitoring/performance/recommendations",
  MONITORING_PERFORMANCE_SYSTEM: "/api/v1/monitoring/performance/system",
  MONITORING_PERFORMANCE_CLEAR: "/api/v1/monitoring/performance/metrics/clear",
  MONITORING_PERFORMANCE_TEST: "/api/v1/monitoring/performance/test",
  MONITORING_TIMEOUT_DIAGNOSTICS: "/api/v1/monitoring/timeout/diagnostics",
  MONITORING_TIMEOUT_HEALTH: "/api/v1/monitoring/timeout/health",

  // Debug Routes (development only, /api/v1/debug)
  DEBUG_AUTH_CONTEXT: "/api/v1/debug/auth/context",
  DEBUG_AUTH_ROLE: "/api/v1/debug/auth/role",
  DEBUG_CASHBANK_PERMISSION: "/api/v1/debug/auth/test-cashbank-permission",
  DEBUG_PAYMENTS_PERMISSION: "/api/v1/debug/auth/test-payments-permission",

  // Static Files
  TEMPLATES: (filepath: string) => `/templates/${filepath}`,
  UPLOADS: (filepath: string) => `/uploads/${filepath}`,

  // Documentation - Standardized
  SWAGGER: "/swagger/index.html",
  DOCS: "/docs/index.html",
  OPENAPI_DOC: "/openapi/doc.json",

  // Settings (with /api/v1 prefix)
  SETTINGS: "/api/v1/settings",
  SETTINGS_PUBLIC: "/api/v1/settings/public",
  SETTINGS_UPDATE: "/api/v1/settings",
  SETTINGS_UPLOAD_LOGO: "/api/v1/settings/company/logo",
  SETTINGS_UPLOAD_ICON: "/api/v1/settings/company/icon",
  BRANDING_PUBLIC: "/api/v1/branding/public",
  BRANDING_LOGO: "/api/v1/branding/logo",
  BRANDING_ICON: "/api/v1/branding/icon",
  BRANDING_MANIFEST: "/api/v1/branding/manifest.webmanifest",

  // Fiscal Year Closing (with /api/v1 prefix)
  FISCAL_CLOSING: {
    PREVIEW: "/api/v1/fiscal-closing/preview",
    EXECUTE: "/api/v1/fiscal-closing/execute",
    HISTORY: "/api/v1/fiscal-closing/history",
  },

  // Period Closing (with /api/v1 prefix)
  PERIOD_CLOSING: {
    LAST_INFO: "/api/v1/period-closing/last-info",
    PREVIEW: "/api/v1/period-closing/preview",
    EXECUTE: "/api/v1/period-closing/execute",
    REOPEN: "/api/v1/period-closing/reopen",
    HISTORY: "/api/v1/period-closing/history",
    CHECK_DATE: "/api/v1/period-closing/check-date",
  },

  // Health Check
  HEALTH: "/api/v1/health",
};

export default API_BASE_URL;
