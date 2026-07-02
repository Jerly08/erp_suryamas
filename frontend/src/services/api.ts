import axios from 'axios';
import API_BASE_URL, { API_ENDPOINTS } from '@/config/api';

const api = axios.create({
  baseURL: API_BASE_URL,
  withCredentials: true,
  timeout: 30000, // Increased to 30 seconds for complex operations
  headers: {
    'Content-Type': 'application/json',
  },
});

// Request interceptor to add auth token
api.interceptors.request.use(
  (config) => {
    config.withCredentials = true;
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// Response interceptor to handle common errors with silent refresh on 401
let isRefreshing = false;
let refreshPromise: Promise<any> | null = null;
let refreshAttempts = 0;
const MAX_REFRESH_ATTEMPTS = 3;
const failedQueue: Array<{ resolve: (token: string) => void, reject: (error: any) => void }> = [];
let authExpiredModalShown = false;
const AUTH_SESSION_EXPIRED_CODE = 'AUTH_SESSION_EXPIRED';

function createAuthExpiredError(originalError?: any) {
  const authError = new Error('Session expired. Please login again.');
  (authError as any).isAuthError = true;
  (authError as any).code = AUTH_SESSION_EXPIRED_CODE;
  (authError as any).suppressToast = true;
  (authError as any).skipToast = true;
  if (originalError) {
    (authError as any).originalError = originalError;
  }
  return authError;
}

function clearVisibleToasts() {
  if (typeof document === 'undefined') return;

  const toastManagers = document.querySelectorAll('[id^="chakra-toast-manager"]');
  toastManagers.forEach((node) => {
    if (node instanceof HTMLElement) {
      node.innerHTML = '';
      node.style.display = 'none';
    }
  });
}

function processQueue(error: any, token: string | null = null) {
  failedQueue.forEach(({ resolve, reject }) => {
    if (error) {
      reject(error);
    } else {
      resolve(token!);
    }
  });

  failedQueue.length = 0;
}

api.interceptors.response.use(
  (response) => {
    // Reset refresh attempts on successful response
    refreshAttempts = 0;
    return response;
  },
  async (error) => {
    const originalRequest = error.config || {};
    const status = error.response?.status;

    // Log for 401 and 403 errors 
    if (status === 401) {
      console.log('API Interceptor - 401 Error on:', originalRequest.url);
      // Check if it's a session-related error
      if (error.response?.data?.code === 'SESSION_EXPIRED' ||
        error.response?.data?.code === 'SESSION_DEACTIVATED' ||
        error.response?.data?.code === 'INVALID_SESSION') {
        console.log('Session-related error detected:', error.response?.data?.code);
        // Clear auth data and show modal only once
        if (typeof window !== 'undefined') {
          window.localStorage.removeItem('token');
          window.localStorage.removeItem('refreshToken');
          window.localStorage.removeItem('user');
        }
        await showAuthExpiredModal();
        return Promise.reject(createAuthExpiredError(error));
      }
    } else if (status === 403) {
      console.log('API Interceptor - 403 Forbidden on:', originalRequest.url);
      // Log current user info from localStorage for debugging
      const userData = window.localStorage.getItem('user');
      try {
        const user = userData ? JSON.parse(userData) : null;
        console.log('Current user role:', user?.role);
      } catch (e) {
        console.log('Could not parse user data');
      }
    }

    if (status === 401 && typeof window !== 'undefined' && !originalRequest._retry) {
      originalRequest._retry = true;

      // Circuit breaker: prevent infinite refresh loops
      if (refreshAttempts >= MAX_REFRESH_ATTEMPTS) {
        console.error('API Interceptor - Max refresh attempts reached, forcing logout');

        // Clear auth data
        window.localStorage.removeItem('token');
        window.localStorage.removeItem('refreshToken');
        window.localStorage.removeItem('user');

        // Reset counter
        refreshAttempts = 0;

        // Show auth expired modal
        await showAuthExpiredModal();
        return Promise.reject(createAuthExpiredError(error));
      }

      refreshAttempts++;
      console.log(`API Interceptor - Attempting token refresh for 401 (attempt ${refreshAttempts}/${MAX_REFRESH_ATTEMPTS})`);

      try {
        if (!isRefreshing) {
          isRefreshing = true;
          refreshPromise = (async () => {
            try {
              // Simple timeout wrapper
              const timeoutPromise = new Promise((_, reject) => {
                setTimeout(() => reject(new Error('Request timeout')), 10000);
              });

              const fetchPromise = fetch(`${API_BASE_URL}${API_ENDPOINTS.REFRESH}`, {
                method: 'POST',
                credentials: 'include',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({}),
              });

              const resp = await Promise.race([fetchPromise, timeoutPromise]) as Response;

              if (!resp.ok) {
                const errorText = await resp.text();
                console.error('Refresh failed with:', errorText);

                if (resp.status === 401 || resp.status === 403 || resp.status === 400) {
                  throw new Error('Refresh token expired or invalid');
                }
                throw new Error(`Failed to refresh token: ${resp.status}`);
              }

              const data = await resp.json();

              console.log('Token refresh response data:', data);

              console.log('Cookie session refresh successful');

              window.localStorage.removeItem('token');
              window.localStorage.removeItem('refreshToken');
              window.localStorage.removeItem('authToken');
              window.localStorage.removeItem('access_token');
              if (data.user) {
                window.localStorage.setItem('user', JSON.stringify(data.user));
              }

              processQueue(null, '');
              return true;
            } catch (refreshError) {
              // If refresh fails, clear auth data
              console.error('Token refresh failed:', refreshError);
              window.localStorage.removeItem('token');
              window.localStorage.removeItem('refreshToken');
              window.localStorage.removeItem('user');

              // Show auth expired modal
              await showAuthExpiredModal();

              processQueue(refreshError, null);
              throw refreshError;
            } finally {
              isRefreshing = false;
              refreshPromise = null;
            }
          })();
        }

        await refreshPromise;
        console.log('API Interceptor - Retrying request after cookie refresh');
        console.log('API Interceptor - Original request URL:', originalRequest.url);
        console.log('API Interceptor - Original request headers before update:', originalRequest.headers);

        // Create a fresh config instead of modifying the original
        const retryConfig = {
          ...originalRequest,
          headers: {
            ...originalRequest.headers,
          },
          withCredentials: true,
        };
        delete retryConfig.headers.Authorization;
        delete retryConfig.headers.authorization;
        delete retryConfig._retry;

        console.log('API Interceptor - Retry config headers:', retryConfig.headers);

        return api(retryConfig);
      } catch (refreshError) {
        // If refresh fails, handle gracefully
        console.error('API Interceptor - Token refresh completely failed:', refreshError);

        // Clear all auth data
        window.localStorage.removeItem('token');
        window.localStorage.removeItem('refreshToken');
        window.localStorage.removeItem('user');

        // Show auth expired modal
        await showAuthExpiredModal();
        return Promise.reject(createAuthExpiredError(refreshError));
      }
    }

    // For other errors, pass them through
    return Promise.reject(error);
  }
);

// Function to show auth expired modal
async function showAuthExpiredModal() {
  if (authExpiredModalShown) {
    clearVisibleToasts();
    return; // Prevent showing multiple modals
  }

  authExpiredModalShown = true;
  clearVisibleToasts();

  try {
    // Dynamically import React and ReactDOM only when needed
    const ReactMod = await import('react');
    const ReactDOMClient = await import('react-dom/client');
    const React = (ReactMod as any).default || ReactMod;
    const ReactDOM = ReactDOMClient as any;

    // Dynamically import and show the modal
    const { default: AuthExpiredModal } = await import('../components/auth/AuthExpiredModal');
    const { LanguageProvider } = await import('../contexts/LanguageContext');

    // Create a container for the modal
    const modalRoot = document.createElement('div');
    modalRoot.id = 'auth-expired-modal-root';
    document.body.appendChild(modalRoot);
    const toastCleanupInterval = window.setInterval(clearVisibleToasts, 200);

    // Render the modal
    const root = ReactDOM.createRoot(modalRoot);

    // Import ChakraProvider for the modal
    const { ChakraProvider } = await import('@chakra-ui/react');
    root.render(
      React.createElement(ChakraProvider, null,
        React.createElement(LanguageProvider, null,
          React.createElement(AuthExpiredModal, {
            isOpen: true,
            onLoginRedirect: () => {
              authExpiredModalShown = false;
              window.clearInterval(toastCleanupInterval);
              // Clean up modal
              root.unmount();
              document.body.removeChild(modalRoot);
              // Redirect to login
              window.location.href = '/login';
            }
          })
        )
      )
    );
  } catch (error) {
    console.error('Failed to load AuthExpiredModal:', error);
    authExpiredModalShown = false;
    // Fallback to direct redirect
    window.location.href = '/login';
  }
}

// Export function for manual use if needed
const isAuthExpiredModalActive = () => authExpiredModalShown;

export { showAuthExpiredModal, isAuthExpiredModalActive };

export default api;
