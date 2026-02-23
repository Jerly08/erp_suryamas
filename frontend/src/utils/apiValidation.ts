/**
 * Production-Ready API Validation System
 * 
 * This module provides comprehensive API endpoint validation,
 * health checks, and error handling to prevent production issues.
 */

import API_BASE_URL, { API_ENDPOINTS } from '@/config/api';

// Type definitions for API validation
export interface APIHealthCheck {
  endpoint: string;
  status: 'healthy' | 'unhealthy' | 'timeout' | 'error';
  responseTime: number;
  error?: string;
  timestamp: string;
}

export interface APIValidationResult {
  allEndpointsValid: boolean;
  healthyEndpoints: number;
  totalEndpoints: number;
  failedEndpoints: string[];
  healthChecks: APIHealthCheck[];
  summary: {
    healthy: number;
    unhealthy: number;
    timeout: number;
    error: number;
  };
}

// Critical endpoints that must be available for the app to function
export const CRITICAL_ENDPOINTS = [
  'VALIDATE_TOKEN',
  'ACCOUNTS.LIST',
  'JOURNALS.LIST',
  'SSOT_REPORTS.TRIAL_BALANCE',
  'HEALTH'
] as const;

// Development vs Production endpoint validation
const VALIDATION_TIMEOUT = process.env.NODE_ENV === 'production' ? 10000 : 5000;
const MAX_RETRIES = process.env.NODE_ENV === 'production' ? 3 : 1;

/**
 * Recursively extract all endpoint URLs from API_ENDPOINTS object
 */
function extractEndpointURLs(obj: any, prefix: string = ''): string[] {
  const urls: string[] = [];

  for (const [key, value] of Object.entries(obj)) {
    const fullKey = prefix ? `${prefix}.${key}` : key;

    if (typeof value === 'string' && value.startsWith('/')) {
      urls.push(value);
    } else if (typeof value === 'function') {
      // For parameterized endpoints, test with sample parameters
      try {
        const sampleUrl = value(1); // Use ID = 1 for testing
        if (typeof sampleUrl === 'string' && sampleUrl.startsWith('/')) {
          urls.push(sampleUrl);
        }
      } catch {
        // Skip functions that can't be called with sample parameters
      }
    } else if (typeof value === 'object' && value !== null) {
      urls.push(...extractEndpointURLs(value, fullKey));
    }
  }

  return urls;
}

/**
 * Get base URL based on environment
 */
function getBaseURL(): string {
  return API_BASE_URL;
}

/**
 * Health check for a single endpoint
 */
async function checkEndpointHealth(
  endpoint: string,
  retries: number = MAX_RETRIES
): Promise<APIHealthCheck> {
  const baseURL = getBaseURL();
  const fullURL = `${baseURL}${endpoint}`;
  const startTime = Date.now();

  for (let attempt = 1; attempt <= retries; attempt++) {
    try {
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), VALIDATION_TIMEOUT);

      const response = await fetch(fullURL, {
        method: 'HEAD', // Use HEAD to avoid unnecessary data transfer
        signal: controller.signal,
        headers: {
          'Accept': 'application/json',
          'Cache-Control': 'no-cache'
        }
      });

      clearTimeout(timeoutId);
      const responseTime = Date.now() - startTime;

      // Consider 200-299 and 401 (auth required) as healthy
      const isHealthy = response.status < 500 || response.status === 401;

      return {
        endpoint,
        status: isHealthy ? 'healthy' : 'unhealthy',
        responseTime,
        error: isHealthy ? undefined : `HTTP ${response.status}: ${response.statusText}`,
        timestamp: new Date().toISOString()
      };

    } catch (error: any) {
      const responseTime = Date.now() - startTime;

      if (error.name === 'AbortError') {
        if (attempt === retries) {
          return {
            endpoint,
            status: 'timeout',
            responseTime: VALIDATION_TIMEOUT,
            error: `Request timeout after ${VALIDATION_TIMEOUT}ms`,
            timestamp: new Date().toISOString()
          };
        }
        continue; // Retry on timeout
      }

      if (attempt === retries) {
        return {
          endpoint,
          status: 'error',
          responseTime,
          error: error.message || 'Unknown error',
          timestamp: new Date().toISOString()
        };
      }
    }
  }

  // This should never be reached, but TypeScript needs it
  return {
    endpoint,
    status: 'error',
    responseTime: Date.now() - startTime,
    error: 'Max retries exceeded',
    timestamp: new Date().toISOString()
  };
}

/**
 * Validate all API endpoints
 */
export async function validateAllEndpoints(
  endpointsToTest?: string[]
): Promise<APIValidationResult> {
  console.log('🔍 Starting comprehensive API validation...');

  const allEndpoints = endpointsToTest || extractEndpointURLs(API_ENDPOINTS);
  const uniqueEndpoints = [...new Set(allEndpoints)]; // Remove duplicates

  console.log(`📊 Testing ${uniqueEndpoints.length} unique endpoints...`);

  // Test endpoints in batches to avoid overwhelming the server
  const batchSize = 10;
  const healthChecks: APIHealthCheck[] = [];

  for (let i = 0; i < uniqueEndpoints.length; i += batchSize) {
    const batch = uniqueEndpoints.slice(i, i + batchSize);
    const batchPromises = batch.map(endpoint => checkEndpointHealth(endpoint));

    try {
      const batchResults = await Promise.all(batchPromises);
      healthChecks.push(...batchResults);

      console.log(`✅ Completed batch ${Math.floor(i / batchSize) + 1}/${Math.ceil(uniqueEndpoints.length / batchSize)}`);
    } catch (error) {
      console.error(`❌ Error in batch ${Math.floor(i / batchSize) + 1}:`, error);
    }
  }

  // Calculate summary statistics
  const summary = {
    healthy: healthChecks.filter(h => h.status === 'healthy').length,
    unhealthy: healthChecks.filter(h => h.status === 'unhealthy').length,
    timeout: healthChecks.filter(h => h.status === 'timeout').length,
    error: healthChecks.filter(h => h.status === 'error').length,
  };

  const failedEndpoints = healthChecks
    .filter(h => h.status !== 'healthy')
    .map(h => h.endpoint);

  const result: APIValidationResult = {
    allEndpointsValid: failedEndpoints.length === 0,
    healthyEndpoints: summary.healthy,
    totalEndpoints: healthChecks.length,
    failedEndpoints,
    healthChecks,
    summary
  };

  // Log results
  console.log('📋 API Validation Results:');
  console.log(`✅ Healthy: ${summary.healthy}`);
  console.log(`❌ Unhealthy: ${summary.unhealthy}`);
  console.log(`⏰ Timeout: ${summary.timeout}`);
  console.log(`🔥 Error: ${summary.error}`);

  if (failedEndpoints.length > 0) {
    console.warn('⚠️ Failed endpoints:', failedEndpoints);
  }

  return result;
}

/**
 * Validate only critical endpoints for faster startup checks
 */
export async function validateCriticalEndpoints(): Promise<APIValidationResult> {
  console.log('🚨 Validating critical endpoints for production readiness...');

  const criticalEndpointURLs = CRITICAL_ENDPOINTS.map(path => {
    const parts = path.split('.');
    let current: any = API_ENDPOINTS;

    for (const part of parts) {
      current = current[part];
      if (!current) {
        console.warn(`⚠️ Critical endpoint ${path} not found in API_ENDPOINTS`);
        return null;
      }
    }

    return typeof current === 'string' ? current : null;
  }).filter(Boolean) as string[];

  return validateAllEndpoints(criticalEndpointURLs);
}

/**
 * Runtime API endpoint validator - prevents app from using invalid endpoints
 */
export function createSafeAPICall(endpoint: string, baseURL?: string) {
  return async function safeFetch(url: string, options: RequestInit = {}) {
    const fullURL = (baseURL || getBaseURL()) + url;

    try {
      const controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), VALIDATION_TIMEOUT);

      const response = await fetch(fullURL, {
        ...options,
        signal: controller.signal,
        headers: {
          'Accept': 'application/json',
          'Content-Type': 'application/json',
          ...options.headers
        }
      });

      clearTimeout(timeout);

      if (!response.ok && response.status >= 500) {
        throw new Error(`Server Error: ${response.status} ${response.statusText}`);
      }

      return response;

    } catch (error: any) {
      console.error(`API Call Failed for ${endpoint}:`, {
        url: fullURL,
        error: error.message,
        timestamp: new Date().toISOString()
      });

      if (error.name === 'AbortError') {
        throw new Error(`API call timeout: ${endpoint}`);
      }

      throw error;
    }
  };
}

/**
 * Production monitoring: Log API health periodically
 */
export function startAPIMonitoring(intervalMinutes: number = 5) {
  if (process.env.NODE_ENV !== 'production') {
    console.log('⏭️ API monitoring skipped in development mode');
    return;
  }

  console.log(`📊 Starting API monitoring (interval: ${intervalMinutes} minutes)`);

  const monitoringInterval = setInterval(async () => {
    try {
      const results = await validateCriticalEndpoints();

      if (!results.allEndpointsValid) {
        console.error('🚨 PRODUCTION ALERT: Critical API endpoints are failing!');
        console.error('Failed endpoints:', results.failedEndpoints);

        // Here you could integrate with monitoring services like:
        // - Sentry
        // - DataDog
        // - New Relic
        // - Custom webhook notifications
      }

    } catch (error) {
      console.error('❌ API monitoring error:', error);
    }
  }, intervalMinutes * 60 * 1000);

  // Cleanup on app shutdown
  if (typeof window !== 'undefined') {
    window.addEventListener('beforeunload', () => {
      clearInterval(monitoringInterval);
    });
  }

  return monitoringInterval;
}

/**
 * Export health check data for external monitoring systems
 */
export function getAPIHealthSummary(): Promise<APIValidationResult> {
  return validateCriticalEndpoints();
}