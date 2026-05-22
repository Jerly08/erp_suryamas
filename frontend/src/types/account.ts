export interface Account {
  id: number;
  code: string;
  name: string;
  description?: string;
  type: 'ASSET' | 'LIABILITY' | 'EQUITY' | 'REVENUE' | 'EXPENSE';
  category?: string;
  parent_id?: number;
  level: number;
  is_header: boolean;
  is_active: boolean;
  balance: number;
  total_balance?: number;  // Sum of this account and all children
  child_count?: number;    // Number of child accounts
  created_at: string;
  updated_at: string;
  parent?: Account;
  children?: Account[];
}


export interface AccountCreateRequest {
  code: string;
  name: string;
  type: 'ASSET' | 'LIABILITY' | 'EQUITY' | 'REVENUE' | 'EXPENSE';
  category?: string;
  parent_id?: number;
  description?: string;
  opening_balance?: number;
  is_header?: boolean; // Allow manual header creation
}

export interface AccountUpdateRequest {
  code?: string;
  name: string;
  type?: 'ASSET' | 'LIABILITY' | 'EQUITY' | 'REVENUE' | 'EXPENSE';
  description?: string;
  category?: string;
  parent_id?: number;
  is_active?: boolean;
  opening_balance?: number;
  is_header?: boolean; // Allow manual header update
}

export interface AccountImportRequest {
  code: string;
  name: string;
  type: 'ASSET' | 'LIABILITY' | 'EQUITY' | 'REVENUE' | 'EXPENSE';
  category?: string;
  parent_code?: string;
  description?: string;
  opening_balance?: number;
}

export interface AccountSummaryResponse {
  type: 'ASSET' | 'LIABILITY' | 'EQUITY' | 'REVENUE' | 'EXPENSE';
  total_accounts: number;
  total_balance: number;
  active_accounts: number;
}

export interface AccountTreeNode {
  id: number;
  code: string;
  name: string;
  type: 'ASSET' | 'LIABILITY' | 'EQUITY' | 'REVENUE' | 'EXPENSE';
  level: number;
  balance: number;
  is_active: boolean;
  has_children: boolean;
  children?: AccountTreeNode[];
}

// Minimal account data for catalog/combobox (used by EMPLOYEE role)
export interface AccountCatalogItem {
  id: number;
  code: string;
  name: string;
  active: boolean;
  is_header?: boolean;
  is_active?: boolean;
}

export interface ApiResponse<T> {
  data: T;
  count?: number;
  message?: string;
}

export interface ApiError {
  error: string;
  code: string;
  details?: Record<string, string>;
  trace_id?: string;
}
