/**
 * Utility functions for handling authentication errors consistently across the app
 */

import { isAuthExpiredModalActive, showAuthExpiredModal } from '@/services/api';

export interface AuthError extends Error {
  isAuthError?: boolean;
  code?: string;
  suppressToast?: boolean;
  skipToast?: boolean;
  response?: {
    status: number;
    data?: any;
  };
}

/**
 * Checks if an error is an authentication-related error
 */
export const isAuthError = (error: any): error is AuthError => {
  if (!error) return false;
  
  // Check for explicit auth error markers
  if (error.isAuthError || error.code === 'AUTH_SESSION_EXPIRED') {
    return true;
  }
  
  // Check HTTP status codes
  if (error.response?.status === 401) {
    return true;
  }
  
  // Check for specific error codes from the backend
  const errorCode = error.response?.data?.code;
  if (errorCode && [
    'AUTH_HEADER_MISSING',
    'INVALID_AUTH_FORMAT',
    'TOKEN_BLACKLISTED',
    'INVALID_TOKEN',
    'INVALID_ACCESS_TOKEN',
    'INVALID_SESSION',
    'SESSION_EXPIRED',
    'ACCOUNT_DISABLED'
  ].includes(errorCode)) {
    return true;
  }
  
  return false;
};

/**
 * Checks if an error is a permission-related error (403)
 */
export const isPermissionError = (error: any): boolean => {
  return error?.response?.status === 403;
};

export const shouldSuppressAuthErrorToast = (error: any): boolean => {
  return Boolean(
    isAuthExpiredModalActive() ||
    error?.suppressToast ||
    error?.skipToast ||
    error?.code === 'AUTH_SESSION_EXPIRED' ||
    error?.isAuthError
  );
};

/**
 * Handles authentication errors with appropriate user feedback
 */
export const handleAuthError = (error: AuthError, context: string = 'API call'): void => {
  console.error(`Auth error in ${context}:`, error);
  
  // Clear potentially corrupted auth data
  if (typeof window !== 'undefined') {
    window.localStorage.removeItem('token');
    window.localStorage.removeItem('refreshToken');
    window.localStorage.removeItem('user');
  }
  
  // Show the auth expired modal
  showAuthExpiredModal();
};

/**
 * Handles permission errors with appropriate user feedback
 */
export const handlePermissionError = (error: any, context: string = 'API call'): string => {
  console.warn(`Permission error in ${context}:`, error);
  
  const errorMessage = error?.response?.data?.error || 'Access denied. You do not have permission to access this resource.';
  
  // You could also trigger a toast notification here
  return errorMessage;
};

/**
 * General error handler that routes to specific handlers based on error type
 */
export const handleApiError = (error: any, context: string = 'API call'): {
  isHandled: boolean;
  message: string;
  shouldRetry: boolean;
  suppressToast: boolean;
} => {
  if (isAuthError(error)) {
    handleAuthError(error, context);
    return {
      isHandled: true,
      message: 'Session expired. Please login again.',
      shouldRetry: false,
      suppressToast: true
    };
  }
  
  if (isPermissionError(error)) {
    const message = handlePermissionError(error, context);
    return {
      isHandled: true,
      message,
      shouldRetry: false,
      suppressToast: false
    };
  }
  
  // For other errors, let the calling code handle them
  const message = error?.response?.data?.error || error?.message || 'An unexpected error occurred';
  
  return {
    isHandled: false,
    message,
    shouldRetry: error?.code === 'NETWORK_ERROR' || error?.response?.status >= 500,
    suppressToast: shouldSuppressAuthErrorToast(error)
  };
};

/**
 * Hook-friendly version of handleApiError for use in React components
 */
export const useAuthErrorHandler = () => {
  return {
    handleAuthError,
    handlePermissionError,
    handleApiError,
    isAuthError,
    isPermissionError
  };
};
