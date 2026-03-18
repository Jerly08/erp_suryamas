/**
 * Enhanced Error Handling Utilities
 * Provides specific business error handling for sales operations
 */

import { createStandaloneToast } from '@chakra-ui/react';
import { shouldSuppressAuthErrorToast } from './authErrorHandler';

const { toast } = createStandaloneToast();

// Error types enum
export enum ErrorTypes {
  VALIDATION_ERROR = 'VALIDATION_ERROR',
  INSUFFICIENT_STOCK = 'INSUFFICIENT_STOCK',
  CREDIT_LIMIT_EXCEEDED = 'CREDIT_LIMIT_EXCEEDED',
  INVALID_CUSTOMER = 'INVALID_CUSTOMER',
  INVALID_PRODUCT = 'INVALID_PRODUCT',
  SALE_NOT_FOUND = 'SALE_NOT_FOUND',
  INVALID_STATUS_TRANSITION = 'INVALID_STATUS_TRANSITION',
  PAYMENT_AMOUNT_INVALID = 'PAYMENT_AMOUNT_INVALID',
  ACCOUNT_NOT_FOUND = 'ACCOUNT_NOT_FOUND',
  PERMISSION_DENIED = 'PERMISSION_DENIED',
  NETWORK_ERROR = 'NETWORK_ERROR',
  SERVER_ERROR = 'SERVER_ERROR',
  DUPLICATE_RECORD = 'DUPLICATE_RECORD',
  DATABASE_CONSTRAINT = 'DATABASE_CONSTRAINT',
  UNKNOWN_ERROR = 'UNKNOWN_ERROR',
}

// Error message mapping
const errorMessages: Record<string, string> = {
  [ErrorTypes.VALIDATION_ERROR]: 'Please check your input data and try again',
  [ErrorTypes.INSUFFICIENT_STOCK]: 'Not enough stock available for this product',
  [ErrorTypes.CREDIT_LIMIT_EXCEEDED]: 'Customer credit limit would be exceeded',
  [ErrorTypes.INVALID_CUSTOMER]: 'Selected customer is invalid or inactive',
  [ErrorTypes.INVALID_PRODUCT]: 'Selected product is invalid or inactive',
  [ErrorTypes.SALE_NOT_FOUND]: 'Sale record not found',
  [ErrorTypes.INVALID_STATUS_TRANSITION]: 'Cannot perform this action in current sale status',
  [ErrorTypes.PAYMENT_AMOUNT_INVALID]: 'Payment amount is invalid',
  [ErrorTypes.ACCOUNT_NOT_FOUND]: 'Selected account not found',
  [ErrorTypes.PERMISSION_DENIED]: 'You do not have permission to perform this action',
  [ErrorTypes.NETWORK_ERROR]: 'Network connection error. Please check your internet connection',
  [ErrorTypes.SERVER_ERROR]: 'Server error occurred. Please try again later',
  [ErrorTypes.DUPLICATE_RECORD]: 'A record with this information already exists. Please try again or use different values.',
  [ErrorTypes.DATABASE_CONSTRAINT]: 'Data constraint violation. Please check your input and try again.',
  [ErrorTypes.UNKNOWN_ERROR]: 'An unexpected error occurred',
};

// Sales-specific error contexts
export enum SalesOperations {
  CREATE_SALE = 'create sale',
  UPDATE_SALE = 'update sale',
  DELETE_SALE = 'delete sale',
  CONFIRM_SALE = 'confirm sale',
  CANCEL_SALE = 'cancel sale',
  CREATE_INVOICE = 'create invoice',
  CREATE_PAYMENT = 'record payment',
  CREATE_RETURN = 'create return',
  LOAD_SALES = 'load sales data',
  LOAD_SALE_DETAILS = 'load sale details',
  LOAD_ANALYTICS = 'load analytics',
  EXPORT_PDF = 'export PDF',
  SUBMIT_APPROVAL = 'submit for approval',
}

interface ApiError {
  code?: string;
  message?: string;
  details?: any;
  statusCode?: number;
}

interface ErrorHandlerOptions {
  showToast?: boolean;
  operation?: SalesOperations;
  customMessage?: string;
  onError?: (error: ApiError) => void;
}

// Main error handler function
export const handleSalesError = (
  error: any,
  options: ErrorHandlerOptions = {}
): ApiError => {
  const {
    showToast = true,
    operation,
    customMessage,
    onError
  } = options;

  let processedError: ApiError = {
    code: ErrorTypes.UNKNOWN_ERROR,
    message: 'An unexpected error occurred',
    statusCode: 500
  };

  // Parse different error formats
  if (error?.response) {
    // Axios error format
    const responseData = error.response.data;
    const errorMessage = responseData?.message || responseData?.error || error.message;
    
    processedError = {
      code: responseData?.code || responseData?.error_code || detectDatabaseError(errorMessage) || getErrorCodeFromStatus(error.response.status),
      message: errorMessage,
      details: responseData?.details || responseData?.errors,
      statusCode: error.response.status
    };
  } else if (error?.code) {
    // Custom error format
    processedError = {
      code: error.code,
      message: error.message,
      details: error.details,
      statusCode: error.statusCode || 500
    };
  } else if (typeof error === 'string') {
    // String error
    processedError = {
      code: detectDatabaseError(error) || ErrorTypes.UNKNOWN_ERROR,
      message: error,
      statusCode: 500
    };
  } else if (error?.message) {
    // Generic error object
    processedError = {
      code: detectDatabaseError(error.message) || ErrorTypes.UNKNOWN_ERROR,
      message: error.message,
      statusCode: 500
    };
  }

  // Get user-friendly message
  const userMessage = customMessage || 
    errorMessages[processedError.code as ErrorTypes] ||
    processedError.message ||
    errorMessages[ErrorTypes.UNKNOWN_ERROR];

  // Show toast notification if enabled
  if (showToast && !shouldSuppressAuthErrorToast(error)) {
    const title = operation ? `Error ${operation}` : 'Error';
    
    toast({
      title,
      description: userMessage,
      status: 'error',
      duration: 5000,
      isClosable: true,
      position: 'top-right',
    });
  }

  // Call custom error handler if provided
  if (onError) {
    onError(processedError);
  }

  // Log error for debugging
  console.error('Sales Error:', {
    operation,
    error: processedError,
    originalError: error
  });

  return processedError;
};

// Specific error handlers for different operations
export const handleCreateSaleError = (error: any, customOptions: Partial<ErrorHandlerOptions> = {}) => {
  return handleSalesError(error, {
    operation: SalesOperations.CREATE_SALE,
    ...customOptions
  });
};

export const handleUpdateSaleError = (error: any, customOptions: Partial<ErrorHandlerOptions> = {}) => {
  return handleSalesError(error, {
    operation: SalesOperations.UPDATE_SALE,
    ...customOptions
  });
};

export const handleConfirmSaleError = (error: any, customOptions: Partial<ErrorHandlerOptions> = {}) => {
  return handleSalesError(error, {
    operation: SalesOperations.CONFIRM_SALE,
    ...customOptions
  });
};

export const handlePaymentError = (error: any, customOptions: Partial<ErrorHandlerOptions> = {}) => {
  return handleSalesError(error, {
    operation: SalesOperations.CREATE_PAYMENT,
    ...customOptions
  });
};

export const handleReturnError = (error: any, customOptions: Partial<ErrorHandlerOptions> = {}) => {
  return handleSalesError(error, {
    operation: SalesOperations.CREATE_RETURN,
    ...customOptions
  });
};

export const handleLoadDataError = (error: any, dataType: string = 'data', customOptions: Partial<ErrorHandlerOptions> = {}) => {
  return handleSalesError(error, {
    customMessage: `Failed to load ${dataType}. Please refresh the page and try again.`,
    ...customOptions
  });
};

// Utility functions
const detectDatabaseError = (errorMessage: string): ErrorTypes | null => {
  if (!errorMessage) return null;

  const lowerMessage = errorMessage.toLowerCase();

  // Check for PostgreSQL duplicate key errors
  if (lowerMessage.includes('duplicate key value violates unique constraint') ||
      lowerMessage.includes('sales_code_key') ||
      lowerMessage.includes('sqlstate 23505')) {
    return ErrorTypes.DUPLICATE_RECORD;
  }

  // Check for other constraint violations
  if (lowerMessage.includes('constraint') ||
      lowerMessage.includes('violates') ||
      lowerMessage.includes('foreign key') ||
      lowerMessage.includes('check constraint')) {
    return ErrorTypes.DATABASE_CONSTRAINT;
  }

  return null;
};

const getErrorCodeFromStatus = (statusCode: number): ErrorTypes => {
  switch (statusCode) {
    case 400:
      return ErrorTypes.VALIDATION_ERROR;
    case 401:
    case 403:
      return ErrorTypes.PERMISSION_DENIED;
    case 404:
      return ErrorTypes.SALE_NOT_FOUND;
    case 422:
      return ErrorTypes.VALIDATION_ERROR;
    case 500:
    case 502:
    case 503:
    case 504:
      return ErrorTypes.SERVER_ERROR;
    default:
      return ErrorTypes.UNKNOWN_ERROR;
  }
};

// Validation error helper
export const handleValidationErrors = (errors: string[], showToast: boolean = true) => {
  const message = errors.length === 1 
    ? errors[0] 
    : `Please fix the following errors:\n• ${errors.join('\n• ')}`;

  if (showToast) {
    toast({
      title: 'Validation Error',
      description: message,
      status: 'error',
      duration: 7000,
      isClosable: true,
      position: 'top-right',
    });
  }

  return {
    code: ErrorTypes.VALIDATION_ERROR,
    message,
    details: errors
  };
};

// Success message helper
export const showSuccessMessage = (operation: SalesOperations, customMessage?: string) => {
  const successMessages: Record<SalesOperations, string> = {
    [SalesOperations.CREATE_SALE]: 'Sale created successfully',
    [SalesOperations.UPDATE_SALE]: 'Sale updated successfully',
    [SalesOperations.DELETE_SALE]: 'Sale deleted successfully',
    [SalesOperations.CONFIRM_SALE]: 'Sale confirmed successfully',
    [SalesOperations.CANCEL_SALE]: 'Sale cancelled successfully',
    [SalesOperations.CREATE_INVOICE]: 'Invoice created successfully',
    [SalesOperations.CREATE_PAYMENT]: 'Payment recorded successfully',
    [SalesOperations.CREATE_RETURN]: 'Return created successfully',
    [SalesOperations.LOAD_SALES]: 'Sales data loaded successfully',
    [SalesOperations.LOAD_SALE_DETAILS]: 'Sale details loaded successfully',
    [SalesOperations.LOAD_ANALYTICS]: 'Analytics loaded successfully',
    [SalesOperations.EXPORT_PDF]: 'PDF exported successfully',
    [SalesOperations.SUBMIT_APPROVAL]: 'Submitted for approval successfully',
  };

  toast({
    title: 'Success',
    description: customMessage || successMessages[operation] || 'Operation completed successfully',
    status: 'success',
    duration: 3000,
    isClosable: true,
    position: 'top-right',
  });
};

// Network error checker
export const isNetworkError = (error: any): boolean => {
  return !navigator.onLine || 
    error?.code === 'NETWORK_ERROR' ||
    error?.message?.includes('Network Error') ||
    error?.message?.includes('fetch');
};

// Retry utility for network errors
export const retryOperation = async <T>(
  operation: () => Promise<T>,
  maxRetries: number = 3,
  delay: number = 1000
): Promise<T> => {
  let lastError: any;

  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      return await operation();
    } catch (error) {
      lastError = error;

      if (attempt === maxRetries || !isNetworkError(error)) {
        throw error;
      }

      // Wait before retrying
      await new Promise(resolve => setTimeout(resolve, delay * attempt));
    }
  }

  throw lastError;
};
