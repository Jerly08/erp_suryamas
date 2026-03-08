import { UseToastOptions } from '@chakra-ui/react';
import { ErrorDetail } from '../components/common/ErrorAlert';

export interface APIError {
  response?: {
    data?: {
      error?: string;
      message?: string;
      details?: any;
      errors?: ValidationErrorResponse[];
      validationErrors?: ValidationErrorResponse[];
    };
    status?: number;
    statusText?: string;
  };
  message?: string;
  code?: string;
}

export interface ValidationErrorResponse {
  field?: string;
  code?: string;
  message: string;
  value?: any;
  context?: Record<string, any>;
}

export interface ParsedValidationError {
  type: 'validation' | 'business' | 'network' | 'server' | 'unknown';
  title: string;
  message: string;
  errors: ErrorDetail[];
  canRetry: boolean;
  suggestions: string[];
}

export interface ErrorHandlerOptions {
  operation?: string;
  showToast?: boolean;
  logToConsole?: boolean;
  fallbackMessage?: string;
  duration?: number;
}

export class ErrorHandler {
  /**
   * Parse validation errors from backend response into structured format
   */
  static parseValidationError(error: APIError): ParsedValidationError {
    const status = error?.response?.status;
    const errorData = error?.response?.data;
    
    // Default error structure
    const parsed: ParsedValidationError = {
      type: 'unknown',
      title: 'Error',
      message: '',
      errors: [],
      canRetry: false,
      suggestions: []
    };

    // Determine error type based on status code
    if (status === 400) {
      parsed.type = 'validation';
      parsed.title = 'Validation Error';
      parsed.canRetry = true;
    } else if (status === 422) {
      parsed.type = 'business';
      parsed.title = 'Business Rule Violation';
      parsed.canRetry = true;
    } else if (status >= 500) {
      parsed.type = 'server';
      parsed.title = 'Server Error';
      parsed.canRetry = true;
      parsed.suggestions.push('Please try again in a few moments');
    } else if (!error?.response) {
      parsed.type = 'network';
      parsed.title = 'Connection Error';
      parsed.canRetry = true;
      parsed.suggestions.push('Check your internet connection');
    }

    // Parse structured validation errors from different possible locations
    const validationErrors = errorData?.errors || 
                           errorData?.validationErrors || 
                           errorData?.validation_errors || 
                           [];
    if (validationErrors.length > 0) {
      parsed.errors = validationErrors.map((err: ValidationErrorResponse) => {
        const errorDetail: ErrorDetail = {
          field: err.field,
          message: err.message,
          context: err.context
        };

        // Add specific suggestions based on error code
        if (err.code === 'INSUFFICIENT_STOCK' && err.context) {
          const productName = err.context.product_name;
          const availableStock = err.context.available_stock;
          errorDetail.suggestion = `Only ${availableStock} units of "${productName}" are available. Please reduce the quantity or add more stock first.`;
        } else if (err.code === 'NOT_FOUND') {
          errorDetail.suggestion = 'Please refresh the page and try selecting the item again.';
        } else if (err.code === 'REQUIRED') {
          errorDetail.suggestion = `Please provide a value for ${err.field?.replace(/[_-]/g, ' ')}.`;
        } else if (err.code === 'INVALID_RANGE') {
          errorDetail.suggestion = 'Please check the valid range for this field.';
        } else if (err.code === 'CREDIT_LIMIT_EXCEEDED' && err.context) {
          const availableCredit = err.context.available_credit;
          errorDetail.suggestion = `Maximum available credit: ${availableCredit}. Consider requesting a credit limit increase or reducing the order amount.`;
        }

        return errorDetail;
      });
    }

    // Extract main error message and ensure it's a string
    // Handle different error response formats from Go backend
    try {
      if (typeof errorData === 'string') {
        parsed.message = errorData;
      } else if (errorData?.error) {
        parsed.message = String(errorData.error);
      } else if (errorData?.message) {
        parsed.message = String(errorData.message);
      } else if (errorData?.details) {
        parsed.message = String(errorData.details);
      } else if (error?.message) {
        parsed.message = String(error.message);
      } else if (error?.response?.statusText) {
        parsed.message = `${error.response.status}: ${error.response.statusText}`;
      } else {
        parsed.message = 'An unexpected error occurred';
      }
    } catch (e) {
      console.warn('Error parsing message:', e);
      parsed.message = 'An unexpected error occurred';
    }

    // Parse validation errors from message if structured errors aren't available
    if (parsed.errors.length === 0 && typeof parsed.message === 'string' && parsed.message.includes('validation failed:')) {
      const errorMessages = parsed.message
        .replace('validation failed: ', '')
        .split('; ')
        .filter(msg => msg.trim().length > 0);
      
      parsed.errors = errorMessages.map(msg => {
        const errorDetail: ErrorDetail = { message: msg };
        
        // Extract field name if present
        const fieldMatch = msg.match(/^([^:]+):\s*(.+)$/);
        if (fieldMatch) {
          errorDetail.field = fieldMatch[1].trim();
          errorDetail.message = fieldMatch[2].trim();
        }
        
        // Add suggestions for common validation errors
        if (msg.toLowerCase().includes('insufficient stock')) {
          errorDetail.suggestion = 'Please check the available stock and reduce the quantity, or add more inventory first.';
        } else if (msg.toLowerCase().includes('required')) {
          errorDetail.suggestion = 'This field is mandatory. Please provide a valid value.';
        } else if (msg.toLowerCase().includes('not found')) {
          errorDetail.suggestion = 'Please refresh the page and try selecting the item again.';
        }
        
        return errorDetail;
      });
    }

    // Add general suggestions based on error type
    if (parsed.type === 'validation' && parsed.suggestions.length === 0) {
      parsed.suggestions.push('Please review and correct the highlighted fields');
    } else if (parsed.type === 'business' && parsed.suggestions.length === 0) {
      parsed.suggestions.push('Please review the business requirements and adjust your input');
    }

    return parsed;
  }

  /**
   * Extract meaningful error message from API error response
   */
  static extractErrorMessage(error: APIError, fallback: string = 'An unexpected error occurred'): string {
    // Try to get error message from various possible locations
    if (error?.response?.data?.error) {
      return error.response.data.error;
    }
    
    if (error?.response?.data?.message) {
      return error.response.data.message;
    }
    
    if (error?.message) {
      return error.message;
    }
    
    if (error?.response?.statusText) {
      return `${error.response.status}: ${error.response.statusText}`;
    }
    
    return fallback;
  }

  /**
   * Get error severity based on status code or error type
   */
  static getErrorSeverity(error: APIError): 'error' | 'warning' | 'info' {
    const status = error?.response?.status;
    
    if (status >= 500) return 'error';     // Server errors
    if (status >= 400) return 'warning';   // Client errors
    if (status >= 300) return 'info';      // Redirects
    
    return 'error'; // Default to error for unknown cases
  }

  /**
   * Handle API error with consistent formatting and logging
   */
  static handleAPIError(
    error: APIError, 
    toast: (options: UseToastOptions) => void,
    options: ErrorHandlerOptions = {}
  ): string {
    const {
      operation = 'operation',
      showToast = true,
      logToConsole = true,
      fallbackMessage,
      duration = 5000
    } = options;

    const errorMessage = this.extractErrorMessage(
      error, 
      fallbackMessage || `Failed to ${operation}`
    );
    
    const severity = this.getErrorSeverity(error);
    const status = error?.response?.status;

    // Log to console if enabled
    if (logToConsole) {
      console.group(`🚨 Error in ${operation}`);
      console.error('Error message:', errorMessage);
      console.error('Status:', status);
      console.error('Full error:', error);
      console.groupEnd();
    }

    // Show toast notification if enabled
    if (showToast) {
      const title = this.getErrorTitle(operation, severity);
      
      toast({
        title,
        description: errorMessage,
        status: severity,
        duration,
        isClosable: true,
        position: 'top-right'
      });
    }

    return errorMessage;
  }

  /**
   * Get appropriate error title based on operation and severity
   */
  static getErrorTitle(operation: string, severity: 'error' | 'warning' | 'info'): string {
    const operationName = operation.charAt(0).toUpperCase() + operation.slice(1);
    
    switch (severity) {
      case 'error':
        return `${operationName} Failed`;
      case 'warning':
        return `${operationName} Warning`;
      case 'info':
        return `${operationName} Info`;
      default:
        return `${operationName} Error`;
    }
  }

  /**
   * Handle validation errors specifically
   */
  static handleValidationError(
    errors: string[],
    toast: (options: UseToastOptions) => void,
    operation: string = 'validation'
  ): void {
    const errorMessage = errors.length === 1 
      ? errors[0] 
      : `Multiple validation errors:\n• ${errors.join('\n• ')}`;

    toast({
      title: 'Validation Error',
      description: errorMessage,
      status: 'warning',
      duration: 6000,
      isClosable: true,
      position: 'top-right'
    });

    console.warn(`Validation errors in ${operation}:`, errors);
  }

  /**
   * Handle success messages consistently
   */
  static handleSuccess(
    message: string,
    toast: (options: UseToastOptions) => void,
    operation: string = 'operation'
  ): void {
    toast({
      title: 'Success',
      description: message,
      status: 'success',
      duration: 3000,
      isClosable: true,
      position: 'top-right'
    });

    console.log(`✅ ${operation} success:`, message);
  }

  /**
   * Handle network/connection errors
   */
  static handleNetworkError(
    error: APIError,
    toast: (options: UseToastOptions) => void
  ): string {
    const isNetworkError = !error?.response || error?.code === 'NETWORK_ERROR';
    
    if (isNetworkError) {
      const message = 'Network connection failed. Please check your internet connection.';
      
      toast({
        title: 'Connection Error',
        description: message,
        status: 'error',
        duration: 8000,
        isClosable: true,
        position: 'top-right'
      });
      
      return message;
    }
    
    return this.handleAPIError(error, toast, {
      operation: 'network request',
      fallbackMessage: 'Network request failed'
    });
  }

  /**
   * Handle service unavailable errors
   */
  static handleServiceUnavailable(
    serviceName: string,
    toast: (options: UseToastOptions) => void,
    fallbackAction?: string
  ): void {
    const message = fallbackAction 
      ? `${serviceName} is currently unavailable. ${fallbackAction}`
      : `${serviceName} service is temporarily unavailable. Please try again later.`;

    toast({
      title: 'Service Unavailable',
      description: message,
      status: 'warning',
      duration: 6000,
      isClosable: true,
      position: 'top-right'
    });

    console.warn(`Service unavailable: ${serviceName}`);
  }

  /**
   * Handle loading state errors
   */
  static handleLoadingError(
    resourceName: string,
    error: APIError,
    toast: (options: UseToastOptions) => void
  ): string {
    return this.handleAPIError(error, toast, {
      operation: `load ${resourceName}`,
      fallbackMessage: `Failed to load ${resourceName}. Please refresh and try again.`
    });
  }

  /**
   * Handle save/update errors
   */
  static handleSaveError(
    resourceName: string,
    error: APIError,
    toast: (options: UseToastOptions) => void,
    isUpdate: boolean = false
  ): string {
    const operation = isUpdate ? `update ${resourceName}` : `create ${resourceName}`;
    
    return this.handleAPIError(error, toast, {
      operation,
      fallbackMessage: `Failed to ${operation}. Please check your input and try again.`
    });
  }

  /**
   * Handle delete errors
   */
  static handleDeleteError(
    resourceName: string,
    error: APIError,
    toast: (options: UseToastOptions) => void
  ): string {
    return this.handleAPIError(error, toast, {
      operation: `delete ${resourceName}`,
      fallbackMessage: `Failed to delete ${resourceName}. It may be in use by other records.`
    });
  }
}

// Export convenience functions
export const handleAPIError = ErrorHandler.handleAPIError.bind(ErrorHandler);
export const handleValidationError = ErrorHandler.handleValidationError.bind(ErrorHandler);
export const handleSuccess = ErrorHandler.handleSuccess.bind(ErrorHandler);
export const handleNetworkError = ErrorHandler.handleNetworkError.bind(ErrorHandler);
export const handleServiceUnavailable = ErrorHandler.handleServiceUnavailable.bind(ErrorHandler);
export const handleLoadingError = ErrorHandler.handleLoadingError.bind(ErrorHandler);
export const handleSaveError = ErrorHandler.handleSaveError.bind(ErrorHandler);
export const handleDeleteError = ErrorHandler.handleDeleteError.bind(ErrorHandler);

/**
 * Parse closed period error from backend.
 * Backend returns error in format: "CLOSED_PERIOD:2026-03-07"
 * Returns { date: string } if matched, null otherwise.
 */
export function parseClosedPeriodError(errorMsg: string): { date: string } | null {
  const match = errorMsg?.match(/CLOSED_PERIOD:(\d{4}-\d{2}-\d{2})/);
  if (!match) return null;
  return { date: match[1] };
}

/**
 * Format closed period date according to current language.
 */
export function formatClosedPeriodDate(dateStr: string, language: string): string {
  const dateObj = new Date(dateStr + 'T00:00:00');
  return dateObj.toLocaleDateString(language === 'id' ? 'id-ID' : 'en-US', {
    day: '2-digit',
    month: 'long',
    year: 'numeric',
  });
}

export default ErrorHandler;
