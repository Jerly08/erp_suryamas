/**
 * Data Caching Hook
 * Provides caching functionality for frequently accessed data
 */

import { useState, useCallback, useRef } from 'react';

interface CacheItem<T> {
  data: T;
  timestamp: number;
  isLoading: boolean;
}

interface CacheOptions {
  ttl?: number; // Time to live in milliseconds
  refreshOnMount?: boolean;
  staleWhileRevalidate?: boolean;
}

export const useDataCache = <T>() => {
  const cache = useRef<Map<string, CacheItem<T>>>(new Map());
  const [loadingKeys, setLoadingKeys] = useState<Set<string>>(new Set());

  const getCachedData = useCallback(async (
    key: string,
    fetchFunction: () => Promise<T>,
    options: CacheOptions = {}
  ): Promise<T> => {
    const {
      ttl = 300000, // 5 minutes default
      refreshOnMount = false,
      staleWhileRevalidate = true
    } = options;

    const cached = cache.current.get(key);
    const now = Date.now();

    // Return fresh data immediately if available and not expired
    if (cached && !cached.isLoading && (now - cached.timestamp) < ttl) {
      return cached.data;
    }

    // Return stale data while revalidating in background
    if (cached && staleWhileRevalidate && !cached.isLoading) {
      // Start background refresh
      const backgroundRefresh = async () => {
        try {
          cache.current.set(key, { ...cached, isLoading: true });
          const freshData = await fetchFunction();
          cache.current.set(key, {
            data: freshData,
            timestamp: now,
            isLoading: false
          });
        } catch (error) {
          // Keep stale data on error
          cache.current.set(key, { ...cached, isLoading: false });
          console.warn(`Background refresh failed for key ${key}:`, error);
        }
      };
      
      backgroundRefresh();
      return cached.data;
    }

    // No cache or expired, fetch fresh data
    if (loadingKeys.has(key)) {
      // Already loading, wait for existing request
      return new Promise((resolve, reject) => {
        const checkCache = () => {
          const currentCache = cache.current.get(key);
          if (currentCache && !currentCache.isLoading) {
            resolve(currentCache.data);
          } else if (loadingKeys.has(key)) {
            setTimeout(checkCache, 100);
          } else {
            reject(new Error('Cache loading failed'));
          }
        };
        checkCache();
      });
    }

    // Start fresh fetch
    setLoadingKeys(prev => new Set(prev).add(key));
    cache.current.set(key, {
      data: cached?.data as T,
      timestamp: cached?.timestamp || 0,
      isLoading: true
    });

    try {
      const freshData = await fetchFunction();
      cache.current.set(key, {
        data: freshData,
        timestamp: now,
        isLoading: false
      });
      
      setLoadingKeys(prev => {
        const newSet = new Set(prev);
        newSet.delete(key);
        return newSet;
      });

      return freshData;
    } catch (error) {
      cache.current.delete(key);
      setLoadingKeys(prev => {
        const newSet = new Set(prev);
        newSet.delete(key);
        return newSet;
      });
      throw error;
    }
  }, []);

  const invalidateCache = useCallback((key?: string) => {
    if (key) {
      cache.current.delete(key);
    } else {
      cache.current.clear();
    }
  }, []);

  const updateCache = useCallback((key: string, data: T) => {
    cache.current.set(key, {
      data,
      timestamp: Date.now(),
      isLoading: false
    });
  }, []);

  const isLoading = useCallback((key: string): boolean => {
    return loadingKeys.has(key) || cache.current.get(key)?.isLoading || false;
  }, [loadingKeys]);

  const getCacheStats = useCallback(() => {
    return {
      size: cache.current.size,
      keys: Array.from(cache.current.keys()),
      loadingKeys: Array.from(loadingKeys)
    };
  }, [loadingKeys]);

  return {
    getCachedData,
    invalidateCache,
    updateCache,
    isLoading,
    getCacheStats
  };
};

// Specific cache hooks for sales data
export const useSalesDataCache = () => {
  const { getCachedData, invalidateCache, updateCache, isLoading } = useDataCache<any>();

  const cacheKeys = {
    customers: 'customers',
    products: 'products',
    accounts: 'accounts:cash-bank',
    salesPersons: 'sales-persons',
    paymentMethods: 'payment-methods',
    paymentTerms: 'payment-terms'
  };

  return {
    getCachedData,
    invalidateCache,
    updateCache,
    isLoading,
    cacheKeys
  };
};

// Parallel data loader with caching
export const useParallelDataLoader = () => {
  const { getCachedData } = useDataCache<any>();
  const [loadingStates, setLoadingStates] = useState<Record<string, boolean>>({});
  const [errors, setErrors] = useState<Record<string, string>>({});

  const loadDataParallel = useCallback(async (
    loaders: Record<string, () => Promise<any>>,
    cacheOptions: CacheOptions = {}
  ) => {
    const keys = Object.keys(loaders);
    
    // Set all as loading
    setLoadingStates(prev => ({
      ...prev,
      ...keys.reduce((acc, key) => ({ ...acc, [key]: true }), {})
    }));

    // Clear previous errors
    setErrors(prev => {
      const newErrors = { ...prev };
      keys.forEach(key => delete newErrors[key]);
      return newErrors;
    });

    const results = await Promise.allSettled(
      keys.map(async (key) => {
        try {
          const data = await getCachedData(key, loaders[key], cacheOptions);
          return { key, data, status: 'fulfilled' };
        } catch (error) {
          return { key, error, status: 'rejected' };
        }
      })
    );

    const successfulResults: Record<string, any> = {};
    const errorResults: Record<string, string> = {};

    results.forEach((result, index) => {
      const key = keys[index];
      
      if (result.status === 'fulfilled') {
        const { data } = result.value as { key: string; data: any; status: string };
        successfulResults[key] = data;
      } else {
        const error = result.reason as { message?: string };
        errorResults[key] = error?.message || 'Failed to load data';
      }
    });

    // Update loading states
    setLoadingStates(prev => {
      const newStates = { ...prev };
      keys.forEach(key => newStates[key] = false);
      return newStates;
    });

    // Update errors
    setErrors(prev => ({ ...prev, ...errorResults }));

    return {
      data: successfulResults,
      errors: errorResults,
      hasErrors: Object.keys(errorResults).length > 0
    };
  }, [getCachedData]);

  return {
    loadDataParallel,
    loadingStates,
    errors,
    isLoading: (key: string) => loadingStates[key] || false
  };
};
