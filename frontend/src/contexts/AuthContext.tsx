'use client';

import React, { createContext, useState, useContext, useEffect } from 'react';
import { API_ENDPOINTS, API_BASE_URL } from '@/config/api';
import { normalizeAuthUser } from '@/utils/userDisplay';

// Define user type - backend sends lowercase roles
export type UserRole = 'admin' | 'finance' | 'finance_manager' | 'finance_staff' | 'accounting' | 'hr' | 'inventory_manager' | 'director' | 'employee' | 'auditor' | 'operational_user';

export interface User {
  id: string;
  email: string;
  name: string;
  username?: string;
  first_name?: string;
  last_name?: string;
  full_name?: string;
  role: UserRole | string;
  active: boolean;
  is_active?: boolean;
  company_type?: string;
}

// Define auth context type
interface AuthContextType {
  user: User | null;
  token: string | null;
  refreshToken: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (name: string, email: string, password: string) => Promise<void>;
  logout: () => void;
  checkAuth: () => Promise<boolean>;
  handleUnauthorized: () => void;
}

// Create context
export const AuthContext = createContext<AuthContextType | undefined>(undefined);

// Use API_BASE_URL from config which handles trailing slash properly

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [refreshToken, setRefreshToken] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState<boolean>(true);

  const clearStoredAuthData = (setLogoutTimestamp = false) => {
    if (typeof window === 'undefined') return;

    window.localStorage.removeItem('token');
    window.localStorage.removeItem('refreshToken');
    window.localStorage.removeItem('authToken');
    window.localStorage.removeItem('access_token');
    window.localStorage.removeItem('authData');
    window.localStorage.removeItem('userData');
    window.localStorage.removeItem('user');
    window.sessionStorage.removeItem('token');
    window.sessionStorage.removeItem('authToken');
    window.sessionStorage.removeItem('access_token');

    if (setLogoutTimestamp) {
      window.localStorage.setItem('lastLogoutTime', Date.now().toString());
    }
  };

  // Initialize auth state from HttpOnly auth cookies.
  useEffect(() => {
    const initializeAuth = async () => {
      if (typeof window !== 'undefined') {
        try {
          const lastLogoutTime = window.localStorage.getItem('lastLogoutTime');

          // Check if user explicitly logged out recently (within last 5 minutes)
          const isRecentLogout = lastLogoutTime &&
            (Date.now() - parseInt(lastLogoutTime)) < 5 * 60 * 1000; // 5 minutes

          if (isRecentLogout) {
            console.log('Recent logout detected, skipping cookie session restore');
            clearStoredAuthData();
          } else {
            clearStoredAuthData();

            const response = await fetch(`${API_BASE_URL}${API_ENDPOINTS.VALIDATE_TOKEN}`, {
              method: 'GET',
              credentials: 'include',
              headers: {
                'Content-Type': 'application/json',
              },
            });

            if (response.ok) {
              const validateData = await response.json();
              const validatedUser = normalizeAuthUser(
                validateData.user || validateData.data,
                validateData.company_type,
              );

              if (validatedUser) {
                setUser(validatedUser);
                setToken(null);
                setRefreshToken(null);
              }
            } else if (response.status === 401 || response.status === 403) {
              clearStoredAuthData();
              setUser(null);
            }
          }
        } catch (error) {
          console.log('Cookie session validation failed:', error);
          clearStoredAuthData();
        }
      }

      setIsLoading(false);
    };

    initializeAuth();
  }, []);

  // Login function
  const login = async (email: string, password: string) => {
    try {
      setIsLoading(true);

      const loginUrl = `${API_BASE_URL}${API_ENDPOINTS.LOGIN}`;
      console.log('Login URL:', loginUrl);

      const response = await fetch(loginUrl, {
        method: 'POST',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ email, password }),
      });

      if (!response.ok) {
        let errorMessage = 'Login failed';
        try {
          const errorData = await response.json();
          // Prefer backend 'error' field, then 'message'
          errorMessage = errorData.error || errorData.message || errorMessage;
        } catch (jsonError) {
          // If response is not JSON, use status text or generic message
          errorMessage = response.statusText || `HTTP ${response.status}: Login failed`;
          console.warn('Failed to parse error response as JSON:', jsonError);
        }
        // Map common status codes to clearer messages
        if (response.status === 401) {
          errorMessage = 'Invalid email or password';
        }
        throw new Error(errorMessage);
      }

      let data;
      try {
        data = await response.json();
      } catch (jsonError) {
        console.error('Failed to parse login response as JSON:', jsonError);
        throw new Error('Invalid response from server');
      }

      // Validate response structure
      if (!data || !data.user) {
        console.error('Invalid login response structure:', data);
        throw new Error('Invalid response from server');
      }

      // Save auth data - ensure role is lowercase
      const userData = normalizeAuthUser(data.user, data.company_type);

      setToken(null);
      setRefreshToken(null);
      setUser(userData);

      if (typeof window !== 'undefined') {
        clearStoredAuthData();
        window.localStorage.removeItem('lastLogoutTime');
      }
    } catch (error) {
      // Log authentication failures as warnings instead of errors since they're expected behavior
      if (error instanceof Error && error.message === 'Invalid email or password') {
        console.warn('Authentication failed:', error.message);
      } else {
        console.error('Login error:', error);
      }
      throw error;
    } finally {
      setIsLoading(false);
    }
  };

  // Register function
  const register = async (name: string, email: string, password: string) => {
    try {
      setIsLoading(true);

      const response = await fetch(`${API_BASE_URL}${API_ENDPOINTS.REGISTER}`, {
        method: 'POST',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ name, email, password }),
      });

      if (!response.ok) {
        let errorMessage = 'Registration failed';
        try {
          const errorData = await response.json();
          errorMessage = errorData.message || errorMessage;
        } catch (jsonError) {
          errorMessage = response.statusText || `HTTP ${response.status}: Registration failed`;
          console.warn('Failed to parse registration error response as JSON:', jsonError);
        }
        throw new Error(errorMessage);
      }

      let data;
      try {
        data = await response.json();
      } catch (jsonError) {
        console.error('Failed to parse registration response as JSON:', jsonError);
        throw new Error('Invalid response from server');
      }

      // Save auth data in memory only. Tokens are stored by backend as HttpOnly cookies.
      setToken(null);
      setRefreshToken(null);
      const userData = normalizeAuthUser(data.user);
      setUser(userData);

      if (typeof window !== 'undefined') {
        clearStoredAuthData();
        window.localStorage.removeItem('lastLogoutTime');
      }
    } catch (error) {
      console.error('Registration error:', error);
      throw error;
    } finally {
      setIsLoading(false);
    }
  };

  // Clear all auth data function
  const clearAuthData = (setLogoutTimestamp = false) => {
    // Clear state
    setToken(null);
    setRefreshToken(null);
    setUser(null);

    clearStoredAuthData(setLogoutTimestamp);
  };

  // Handle unauthorized access (e.g., invalid token)
  const handleUnauthorized = () => {
    console.warn('Unauthorized access detected');
    // Don't auto-logout immediately, let components handle 401s gracefully
    // Only logout if the user is actually not authenticated
    if (!token || !user) {
      console.error('No valid auth tokens found, logging out...');
      logout();
    } else {
      console.warn('Auth tokens present but 401 received - might be a permission issue');
    }
  };

  // Logout function
  const logout = () => {
    console.log('Logout initiated by user');
    void fetch(`${API_BASE_URL}${API_ENDPOINTS.LOGOUT}`, {
      method: 'POST',
      credentials: 'include',
    }).catch(() => undefined);
    clearAuthData(true); // Set logout timestamp
    if (typeof window !== 'undefined') {
      // Use setTimeout to ensure localStorage is updated before redirect
      setTimeout(() => {
        window.location.href = '/login';
      }, 100);
    }
  };

  // Check if user is authenticated
  const checkAuth = async (): Promise<boolean> => {
    try {
      const response = await fetch(`${API_BASE_URL}${API_ENDPOINTS.REFRESH}`, {
        method: 'POST',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({}),
      });

      if (response.ok) {
        const data = await response.json();
        const userData = normalizeAuthUser(data.user, data.company_type);
        setToken(null);
        setRefreshToken(null);
        setUser(userData);
        clearStoredAuthData();
        return true;
      }

      clearAuthData();
      return false;
    } catch (error) {
      console.error('Session refresh error:', error);
      clearAuthData();
      return false;
    }
  };

  return (
    <AuthContext.Provider
      value={{
        user,
        token,
        refreshToken,
        isAuthenticated: !!user,
        isLoading,
        login,
        register,
        logout,
        checkAuth,
        handleUnauthorized
      }}
    >
      {children}
    </AuthContext.Provider>
  );
};

// Custom hook to use the auth context
export const useAuth = (): AuthContextType => {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}; 
