import { useContext, useEffect } from 'react';
import { AuthContext } from '@/contexts/AuthContext';
import { accountService } from '@/services/accountService';
import { contactService } from '@/services/contactService';
import salesService from '@/services/salesService';

/**
 * Custom hook to setup unauthorized error handling for API services
 * This should be used in the main app or layout components
 */
export const useAuthService = () => {
  const authContext = useContext(AuthContext);
  const handleUnauthorized = authContext?.handleUnauthorized;
  const accountServiceWithHandler = accountService as {
    setUnauthorizedHandler?: (handler: NonNullable<typeof handleUnauthorized>) => void;
  };
  const contactServiceWithHandler = contactService as {
    setUnauthorizedHandler?: (handler: NonNullable<typeof handleUnauthorized>) => void;
  };

  useEffect(() => {
    if (!handleUnauthorized) {
      console.warn('useAuthService: AuthContext not found');
      return;
    }

    // Set up the unauthorized handler for all services
    if (typeof accountServiceWithHandler.setUnauthorizedHandler === 'function') {
      accountServiceWithHandler.setUnauthorizedHandler(handleUnauthorized);
    }
    if (typeof contactServiceWithHandler.setUnauthorizedHandler === 'function') {
      contactServiceWithHandler.setUnauthorizedHandler(handleUnauthorized);
    }
    
    // Note: salesService uses the shared api.ts with interceptors,
    // so it doesn't need a separate unauthorized handler
    
    // Cleanup function (optional - services are singletons, so this might not be necessary)
    return () => {
      // Could clear handlers here if needed
    };
  }, [accountServiceWithHandler, contactServiceWithHandler, handleUnauthorized]);
};
