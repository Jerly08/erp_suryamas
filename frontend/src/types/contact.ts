export interface ApiError {
  error: string;
  code: string;
}

export interface ApiResponse<T> {
  data: T;
}

export interface Contact {
  id: number;
  code?: string;
  name: string;
  company_name?: string;
  type: 'CUSTOMER' | 'VENDOR' | 'EMPLOYEE';
  category?: string;
  email?: string;
  phone?: string;
  mobile?: string;
  fax?: string;
  website?: string;
  tax_number?: string;
  credit_limit?: number;
  payment_terms?: number;
  is_active: boolean;
  pic_name?: string;        // Person In Charge (for Customer/Vendor)
  external_id?: string;     // Employee ID, Vendor ID, Customer ID
  address?: string;         // Simple address field
  description?: string;
  default_expense_account_id?: number;
  notes?: string;
  created_at: string;
  updated_at: string;
  addresses?: ContactAddress[];
}

export interface ContactAddress {
  id: number;
  contact_id: number;
  type: 'BILLING' | 'SHIPPING' | 'MAILING';
  address1: string;
  address2?: string;
  city: string;
  state?: string;
  postal_code?: string;
  country: string;
  is_default: boolean;
}

