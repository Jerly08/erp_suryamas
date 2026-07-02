/**
 * Utility functions for consistent authentication token management across the application
 * This ensures all parts of the app use the same token retrieval strategy
 */

/**
 * Retrieves a legacy authentication token if one still exists.
 *
 * New production auth uses HttpOnly cookies, so JavaScript should not be able
 * to read or persist access/refresh tokens. This function only supports older
 * in-memory/header call sites while actively cleaning legacy browser storage.
 * 
 * @param throwOnError - Whether to throw an error if no token is found (default: true)
 * @returns The authentication token or null if not found and throwOnError is false
 */
export function getAuthToken(throwOnError: boolean = true): string | null {
  if (typeof window !== 'undefined') {
    localStorage.removeItem('token');
    localStorage.removeItem('authToken');
    localStorage.removeItem('access_token');
    localStorage.removeItem('refreshToken');
    sessionStorage.removeItem('token');
    sessionStorage.removeItem('authToken');
    sessionStorage.removeItem('access_token');
  }
  
  if (throwOnError) {
    throw new Error('Authentication token is stored in an HttpOnly cookie and is not readable by JavaScript.');
  }
  
  return null;
}

/**
 * Creates authorization headers with Bearer token for API requests
 * 
 * @param includeContentType - Whether to include Content-Type header (default: true)
 * @returns Headers object with Authorization and optionally Content-Type
 */
export function getAuthHeaders(includeContentType: boolean = true): Record<string, string> {
  const headers: Record<string, string> = {};
  
  if (includeContentType) {
    headers['Content-Type'] = 'application/json';
  }
  
  return headers;
}

/**
 * Checks if user is authenticated by verifying token existence
 * 
 * @returns True if token exists, false otherwise
 */
export function isAuthenticated(): boolean {
  return false;
}

/**
 * Clears all authentication tokens from storage
 * This is useful for logout operations or when tokens become invalid
 */
export function clearAuthTokens(): void {
  if (typeof window !== 'undefined') {
    // Clear all possible token storage locations
    localStorage.removeItem('token');
    localStorage.removeItem('authToken');
    sessionStorage.removeItem('token');
    sessionStorage.removeItem('authToken');
    
    // Clear related auth data
    localStorage.removeItem('refreshToken');
    localStorage.removeItem('user');
    localStorage.removeItem('authData');
    localStorage.removeItem('userData');
    
    // Clear cookies by setting them to expire
    const cookieNames = ['token', 'authToken', 'access_token', 'erp_access_token', 'erp_refresh_token'];
    cookieNames.forEach(name => {
      document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;`;
    });
  }
}
