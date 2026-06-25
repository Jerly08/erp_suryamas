/**
 * Data Format Standardization Utilities
 * Standardizes data formats between frontend and backend
 */

import { format, parseISO } from 'date-fns';

// Date formatting utilities
export const formatDateForBackend = (date: string | Date): string => {
  if (!date) return '';
  
  const dateObj = typeof date === 'string' ? new Date(date) : date;
  return dateObj.toISOString();
};

export const formatDateForDisplay = (date: string | Date): string => {
  if (!date) return '';
  
  const dateObj = typeof date === 'string' ? parseISO(date) : date;
  return format(dateObj, 'yyyy-MM-dd');
};

export const formatDateTimeForDisplay = (date: string | Date): string => {
  if (!date) return '';
  
  const dateObj = typeof date === 'string' ? parseISO(date) : date;
  return format(dateObj, 'yyyy-MM-dd HH:mm:ss');
};

// Indonesian month names for better clarity
const indonesianMonthNames = [
  'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
  'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember'
];

// Format date with Indonesian month names (DD Month YYYY)
export const formatDateWithIndonesianMonth = (date: string | Date | null): string => {
  if (!date) return '-';
  
  // Handle Go's zero date
  if (typeof date === 'string' && (date === '0001-01-01T00:00:00Z' || date === '0001-01-01')) {
    return '-';
  }
  
  const dateObj = typeof date === 'string' ? new Date(date) : date;
  
  // Handle invalid dates
  if (isNaN(dateObj.getTime())) return '-';
  
  // Check if it's Go's zero date after conversion
  if (dateObj.getFullYear() === 1) {
    return '-';
  }
  
  const day = dateObj.getDate();
  const month = dateObj.getMonth(); // 0-based index
  const year = dateObj.getFullYear();
  
  return `${day} ${indonesianMonthNames[month]} ${year}`;
};

// Format date for display with Indonesian format (fallback to old format if needed)
export const formatDateForIndonesianDisplay = (date: string | Date, useMonthNames: boolean = true): string => {
  if (!date) return '';
  
  if (useMonthNames) {
    return formatDateWithIndonesianMonth(date);
  } else {
    // Fallback to DD/MM/YYYY format
    const dateObj = typeof date === 'string' ? new Date(date) : date;
    if (isNaN(dateObj.getTime())) return '';
    
    return dateObj.toLocaleDateString('id-ID', {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric'
    });
  }
};

// Field name mapping utilities
export const mapSaleToBackendFormat = (frontendData: any): any => {
  return {
    customer_id: frontendData.customerId || frontendData.customer_id,
    sales_person_id: frontendData.salesPersonId || frontendData.sales_person_id,
    type: frontendData.type,
    status: frontendData.status,
    date: formatDateForBackend(frontendData.date),
    due_date: frontendData.dueDate ? formatDateForBackend(frontendData.dueDate) : null,
    validity_period: frontendData.validityPeriod || frontendData.validity_period,
    reference: frontendData.reference,
    customer_reference: frontendData.customerReference || frontendData.customer_reference,
    
    // Items mapping
    sale_items: frontendData.items?.map((item: any) => ({
      product_id: item.productId || item.product_id,
      quantity: Number(item.quantity),
      unit_price: Number(item.unitPrice || item.unit_price),
      discount_percent: Number(item.discountPercent || item.discount_percent || 0),
      tax_percent: Number(item.taxPercent || item.tax_percent || 0),
      revenue_account_id: item.revenueAccountId || item.revenue_account_id,
      tax_account_id: item.taxAccountId || item.tax_account_id,
    })) || [],
    
    // Financial data
    subtotal: Number(frontendData.subtotal || 0),
    discount_percent: Number(frontendData.discountPercent || frontendData.discount_percent || 0),
    discount_amount: Number(frontendData.discountAmount || frontendData.discount_amount || 0),
    shipping_cost: Number(frontendData.shippingCost || frontendData.shipping_cost || 0),
    ppn_percent: Number(frontendData.ppnPercent || frontendData.ppn_percent || 11),
    ppn_amount: Number(frontendData.ppnAmount || frontendData.ppn_amount || 0),
    total_amount: Number(frontendData.totalAmount || frontendData.total_amount || 0),
    
    // Additional fields
    payment_terms: frontendData.paymentTerms || frontendData.payment_terms,
    notes: frontendData.notes || '',
    internal_notes: frontendData.internalNotes || frontendData.internal_notes || '',
    
    // Currency support
    currency: frontendData.currency || 'IDR',
    exchange_rate: Number(frontendData.exchangeRate || frontendData.exchange_rate || 1),
  };
};

export const mapPaymentToBackendFormat = (frontendData: any): any => {
  return {
    amount: Number(frontendData.amount),
    payment_date: formatDateForBackend(frontendData.paymentDate || frontendData.payment_date),
    payment_method: frontendData.paymentMethod || frontendData.payment_method,
    account_id: frontendData.accountId || frontendData.account_id,
    reference_number: frontendData.referenceNumber || frontendData.reference_number,
    notes: frontendData.notes || '',
  };
};

export const mapReturnToBackendFormat = (frontendData: any): any => {
  return {
    return_date: formatDateForBackend(frontendData.returnDate || frontendData.return_date),
    return_items: frontendData.items?.map((item: any) => ({
      sale_item_id: item.saleItemId || item.sale_item_id,
      quantity: Number(item.quantity),
      reason: item.reason || '',
    })) || [],
    reason: frontendData.reason || '',
    notes: frontendData.notes || '',
  };
};

// Response data mapping from backend to frontend
export const mapSaleFromBackend = (backendData: any): any => {
  return {
    id: backendData.id,
    code: backendData.code,
    customerId: backendData.customer_id,
    customer: backendData.customer,
    salesPersonId: backendData.sales_person_id,
    salesPerson: backendData.sales_person,
    type: backendData.type,
    status: backendData.status,
    date: backendData.date,
    dueDate: backendData.due_date,
    validityPeriod: backendData.validity_period,
    reference: backendData.reference,
    customerReference: backendData.customer_reference,
    
    // Financial data
    subtotal: Number(backendData.subtotal || 0),
    discountPercent: Number(backendData.discount_percent || 0),
    discountAmount: Number(backendData.discount_amount || 0),
    shippingCost: Number(backendData.shipping_cost || 0),
    ppnPercent: Number(backendData.ppn_percent || 11),
    ppnAmount: Number(backendData.ppn_amount || 0),
    totalAmount: Number(backendData.total_amount || 0),
    outstandingAmount: Number(backendData.outstanding_amount || 0),
    
    // Currency
    currency: backendData.currency || 'IDR',
    exchangeRate: Number(backendData.exchange_rate || 1),
    
    // Additional data
    paymentTerms: backendData.payment_terms,
    notes: backendData.notes,
    internalNotes: backendData.internal_notes,
    
    // Related data
    saleItems: backendData.sale_items?.map((item: any) => ({
      id: item.id,
      productId: item.product_id,
      product: item.product,
      quantity: Number(item.quantity),
      unitPrice: Number(item.unit_price),
      discountPercent: Number(item.discount_percent || 0),
      taxPercent: Number(item.tax_percent || 0),
      lineTotal: Number(item.line_total || 0),
      revenueAccountId: item.revenue_account_id,
      taxAccountId: item.tax_account_id,
    })) || [],
    
    salePayments: backendData.sale_payments?.map((payment: any) => ({
      id: payment.id,
      amount: Number(payment.amount),
      paymentDate: payment.payment_date,
      paymentMethod: payment.payment_method,
      accountId: payment.account_id,
      account: payment.account,
      referenceNumber: payment.reference_number,
      notes: payment.notes,
    })) || [],
    
    saleReturns: backendData.sale_returns?.map((returnItem: any) => ({
      id: returnItem.id,
      returnNumber: returnItem.return_number,
      returnDate: returnItem.return_date,
      reason: returnItem.reason,
      notes: returnItem.notes,
      returnItems: returnItem.return_items || [],
    })) || [],
    
    // Timestamps
    createdAt: backendData.created_at,
    updatedAt: backendData.updated_at,
  };
};

// Number formatting utilities
export const formatCurrency = (amount: number, currency: string = 'IDR'): string => {
  // Handle null, undefined, NaN or invalid values
  if (amount === null || amount === undefined || isNaN(amount) || !isFinite(amount)) {
    amount = 0;
  }
  const hasFraction = Math.abs(amount % 1) > Number.EPSILON;
  
  const formatter = new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: currency === 'IDR' ? 'IDR' : 'USD',
    minimumFractionDigits: hasFraction ? 2 : 0,
    maximumFractionDigits: 2,
  });
  
  return formatter.format(amount);
};

export const formatNumber = (number: number): string => {
  return new Intl.NumberFormat('id-ID').format(number);
};

// Validation utilities
export const validateSaleData = (data: any): { isValid: boolean; errors: string[] } => {
  const errors: string[] = [];
  
  if (!data.customerId && !data.customer_id) {
    errors.push('Customer is required');
  }
  
  if (!data.items || data.items.length === 0) {
    errors.push('At least one sale item is required');
  }
  
  if (data.items) {
    data.items.forEach((item: any, index: number) => {
      if (!item.productId && !item.product_id) {
        errors.push(`Product is required for item ${index + 1}`);
      }
      
      if (!item.quantity || item.quantity <= 0) {
        errors.push(`Quantity must be greater than 0 for item ${index + 1}`);
      }
      
      if (!item.unitPrice && !item.unit_price || Number(item.unitPrice || item.unit_price) < 0) {
        errors.push(`Unit price must be >= 0 for item ${index + 1}`);
      }
    });
  }
  
  if (!data.date) {
    errors.push('Sale date is required');
  }
  
  return {
    isValid: errors.length === 0,
    errors
  };
};

export const validatePaymentData = (data: any, outstandingAmount: number): { isValid: boolean; errors: string[] } => {
  const errors: string[] = [];
  
  if (!data.amount || Number(data.amount) <= 0) {
    errors.push('Payment amount must be greater than 0');
  }
  
  if (Number(data.amount) > outstandingAmount) {
    errors.push('Payment amount cannot exceed outstanding amount');
  }
  
  if (!data.paymentMethod && !data.payment_method) {
    errors.push('Payment method is required');
  }
  
  if (!data.accountId && !data.account_id) {
    errors.push('Account is required');
  }
  
  if (!data.paymentDate && !data.payment_date) {
    errors.push('Payment date is required');
  }
  
  return {
    isValid: errors.length === 0,
    errors
  };
};
