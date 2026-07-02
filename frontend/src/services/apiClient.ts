/**
 * Production-Ready API Client
 * 
 * Robust HTTP client with retry logic, timeout handling,
 * monitoring, caching, and comprehensive error handling
 */

import {
  ApiClientConfig,
  ApiRequestConfig,
  ServiceResponse,
  IApiService,
  HttpMethod,
  ApiMetrics,
  ApiErrorContext,
  ApiTimeoutError,
  ApiNetworkError,
  DEFAULT_API_CONFIG,
  PRODUCTION_API_CONFIG
} from '@/types/api';
import API_BASE_URL from '@/config/api';

// Simple in-memory cache for production
class ApiCache {
  private cache = new Map<string, { data: any; timestamp: number; ttl: number }>();
  private maxSize: number;

  constructor(maxSize: number = 100) {
    this.maxSize = maxSize;
  }

  set(key: string, data: any, ttl: number): void {
    // Remove oldest entries if cache is full
    if (this.cache.size >= this.maxSize) {
      const oldestKey = this.cache.keys().next().value;
      if (oldestKey) this.cache.delete(oldestKey);
    }

    this.cache.set(key, {
      data,
      timestamp: Date.now(),
      ttl
    });
  }

  get(key: string): any | null {
    const entry = this.cache.get(key);
    if (!entry) return null;

    // Check if expired
    if (Date.now() - entry.timestamp > entry.ttl) {
      this.cache.delete(key);
      return null;
    }

    return entry.data;
  }

  clear(): void {
    this.cache.clear();
  }

  size(): number {
    return this.cache.size;
  }
}

export class ProductionApiClient implements IApiService {
  private config: ApiClientConfig;
  private cache: ApiCache;
  private metrics: ApiMetrics[] = [];
  private requestId = 0;

  constructor(config?: Partial<ApiClientConfig>) {
    const baseConfig = process.env.NODE_ENV === 'production'
      ? PRODUCTION_API_CONFIG
      : DEFAULT_API_CONFIG;

    this.config = { ...baseConfig, ...config };
    this.cache = new ApiCache(this.config.cache?.maxSize || 100);

    console.log(`🚀 API Client initialized for ${process.env.NODE_ENV} environment`);
  }

  /**
   * Generate unique request ID for tracking
   */
  private generateRequestId(): string {
    return `req_${Date.now()}_${++this.requestId}`;
  }

  /**
   * Create cache key for request
   */
  private createCacheKey(method: string, endpoint: string, data?: any): string {
    const dataHash = data ? JSON.stringify(data).slice(0, 50) : '';
    return `${method}:${endpoint}:${dataHash}`;
  }

  /**
   * Get base URL based on environment
   */
  private getBaseURL(): string {
    // If explicitly configured, use that
    if (this.config.baseURL) {
      return this.config.baseURL;
    }

    // Use centralized configuration from config/api.ts
    // This correctly handles SSR vs Client in Docker & Dev environments
    return API_BASE_URL;
  }

  /**
   * Build full URL
   */
  private buildURL(endpoint: string): string {
    const baseURL = this.getBaseURL();

    // Remove trailing slash from baseURL if present
    const cleanBaseURL = baseURL.replace(/\/$/, '');

    // Ensure endpoint starts with /
    const cleanEndpoint = endpoint.startsWith('/') ? endpoint : `/${endpoint}`;

    return `${cleanBaseURL}${cleanEndpoint}`;
  }

  /**
   * Create error context for detailed logging
   */
  private createErrorContext(
    endpoint: string,
    method: HttpMethod,
    requestId: string,
    requestData?: any,
    responseData?: any
  ): ApiErrorContext {
    return {
      endpoint,
      method,
      requestId,
      userId: this.getUserId(),
      timestamp: new Date().toISOString(),
      userAgent: typeof window !== 'undefined' ? window.navigator.userAgent : undefined,
      requestData: requestData ? JSON.stringify(requestData).slice(0, 500) : undefined,
      responseData: responseData ? JSON.stringify(responseData).slice(0, 500) : undefined
    };
  }

  /**
   * Get user ID from token or session (implement based on your auth system)
   */
  private getUserId(): string | undefined {
    return undefined;
  }

  /**
   * Record API metrics for monitoring
   */
  private recordMetrics(
    endpoint: string,
    method: HttpMethod,
    responseTime: number,
    statusCode: number,
    success: boolean,
    error?: string
  ): void {
    const metric: ApiMetrics = {
      endpoint,
      method,
      responseTime,
      statusCode,
      success,
      error,
      timestamp: new Date().toISOString(),
      userAgent: typeof window !== 'undefined' ? window.navigator.userAgent : undefined,
      userId: this.getUserId()
    };

    this.metrics.push(metric);

    // Keep only last 1000 metrics to prevent memory issues
    if (this.metrics.length > 1000) {
      this.metrics = this.metrics.slice(-500);
    }

    // Log metrics in development
    if (process.env.NODE_ENV === 'development') {
      console.log(`📊 API Metric:`, {
        endpoint,
        method,
        responseTime: `${responseTime}ms`,
        status: statusCode,
        success
      });
    }

    // In production, you could send metrics to monitoring service
    if (process.env.NODE_ENV === 'production' && !success) {
      console.error('🚨 API Error Metric:', metric);
    }
  }

  /**
   * Sleep utility for retry delays
   */
  private sleep(ms: number): Promise<void> {
    return new Promise(resolve => setTimeout(resolve, ms));
  }

  /**
   * Make HTTP request with retry logic
   */
  private async makeRequest<T>(
    endpoint: string,
    method: HttpMethod,
    data?: any,
    config: ApiRequestConfig = {}
  ): Promise<ServiceResponse<T>> {
    const requestId = this.generateRequestId();
    const fullURL = this.buildURL(endpoint);
    const startTime = Date.now();

    // Check cache for GET requests
    if (method === 'GET' && this.config.cache?.enabled) {
      const cacheKey = this.createCacheKey(method, endpoint, data);
      const cached = this.cache.get(cacheKey);
      if (cached) {
        console.log(`💾 Cache hit for ${endpoint}`);
        return cached;
      }
    }

    const maxRetries = config.retries ?? this.config.retries;
    let lastError: Error | null = null;

    for (let attempt = 0; attempt <= maxRetries; attempt++) {
      try {
        // Apply retry delay (except for first attempt)
        if (attempt > 0) {
          const delay = this.config.retryDelay * Math.pow(2, attempt - 1); // Exponential backoff
          console.log(`🔄 Retry ${attempt}/${maxRetries} for ${endpoint} after ${delay}ms`);
          await this.sleep(delay);
        }

        // Create abort controller for timeout
        const controller = new AbortController();
        const timeout = config.timeout ?? this.config.timeout;
        const timeoutId = setTimeout(() => controller.abort(), timeout);

        // Prepare request configuration
        const requestConfig: RequestInit = {
          method,
          signal: controller.signal,
          credentials: 'include',
          headers: {
            'Accept': 'application/json',
            'Content-Type': 'application/json',
            'X-Request-ID': requestId,
            ...config.headers
          },
          ...config
        };

        // Add body for non-GET requests
        if (data && method !== 'GET') {
          requestConfig.body = JSON.stringify(data);
        }

        // Apply request interceptor
        if (this.config.interceptors?.request) {
          const interceptedConfig = await this.config.interceptors.request(config);
          Object.assign(requestConfig, interceptedConfig);
        }

        // Make the request
        console.log(`🌐 ${method} ${endpoint} (${requestId})`);
        console.log(`📍 Full URL: ${fullURL}`);
        console.log(`🔧 Base URL: ${this.getBaseURL()}`);
        console.log(`🎯 Environment: ${process.env.NODE_ENV}`);

        const response = await fetch(fullURL, requestConfig);
        clearTimeout(timeoutId);

        const responseTime = Date.now() - startTime;
        let responseData: any;

        // Parse response
        try {
          const text = await response.text();
          responseData = text ? JSON.parse(text) : null;
        } catch {
          responseData = null;
        }

        // Apply response interceptor
        if (this.config.interceptors?.response) {
          await this.config.interceptors.response(response);
        }

        // Validate status
        const isValidStatus = this.config.validateStatus(response.status);
        const success = response.ok && isValidStatus;

        // Record metrics
        this.recordMetrics(endpoint, method, responseTime, response.status, success);

        // Handle successful response
        if (success) {
          const result: ServiceResponse<T> = {
            data: responseData,
            success: true,
            status: response.status,
            headers: response.headers
          };

          // Cache GET requests
          if (method === 'GET' && this.config.cache?.enabled) {
            const cacheKey = this.createCacheKey(method, endpoint, data);
            this.cache.set(cacheKey, result, this.config.cache.ttl);
          }

          console.log(`✅ ${method} ${endpoint} completed in ${responseTime}ms`);
          return result;
        }

        // Handle error response
        const errorMessage = responseData?.error || responseData?.message ||
          `HTTP ${response.status}: ${response.statusText}`;

        const apiError = new Error(errorMessage);
        const context = this.createErrorContext(endpoint, method, requestId, data, responseData);

        // For client errors (4xx), don't retry
        if (response.status >= 400 && response.status < 500) {
          this.recordMetrics(endpoint, method, responseTime, response.status, false, errorMessage);
          return {
            data: null as any,
            success: false,
            error: errorMessage,
            status: response.status,
            headers: response.headers
          };
        }

        throw apiError;

      } catch (error: any) {
        const responseTime = Date.now() - startTime;
        const context = this.createErrorContext(endpoint, method, requestId, data);

        lastError = error;

        // Handle timeout
        if (error.name === 'AbortError') {
          const timeoutError = new ApiTimeoutError(
            `Request timeout after ${config.timeout ?? this.config.timeout}ms`,
            config.timeout ?? this.config.timeout,
            context
          );

          if (attempt === maxRetries) {
            this.recordMetrics(endpoint, method, responseTime, 0, false, 'Timeout');
            throw timeoutError;
          }
          continue;
        }

        // Handle network errors
        if (error.name === 'TypeError' && error.message.includes('fetch')) {
          console.error(`❌ Network error on attempt ${attempt}/${maxRetries}:`, {
            url: fullURL,
            baseURL: this.getBaseURL(),
            endpoint,
            error: error.message,
            stack: error.stack
          });

          const networkError = new ApiNetworkError(
            `Network error - unable to connect to server at ${fullURL}. ${error.message}`,
            context,
            error
          );

          if (attempt === maxRetries) {
            this.recordMetrics(endpoint, method, responseTime, 0, false, 'Network Error');
            console.error('🚫 All retry attempts failed for network connection');
            throw networkError;
          }

          console.log(`🔄 Retrying network request (${attempt + 1}/${maxRetries}) in ${this.config.retryDelay}ms...`);
          continue;
        }

        // For other errors, retry if attempts remaining
        if (attempt === maxRetries) {
          this.recordMetrics(endpoint, method, responseTime, 0, false, error.message);
          if (this.config.interceptors?.error) {
            await this.config.interceptors.error(error);
          }
          throw error;
        }
      }
    }

    // This should never be reached, but TypeScript needs it
    throw lastError || new Error('Unknown error occurred');
  }

  /**
   * GET request
   */
  async get<T>(endpoint: string, config?: ApiRequestConfig): Promise<ServiceResponse<T>> {
    return this.makeRequest<T>(endpoint, 'GET', undefined, config);
  }

  /**
   * POST request
   */
  async post<T>(endpoint: string, data?: any, config?: ApiRequestConfig): Promise<ServiceResponse<T>> {
    return this.makeRequest<T>(endpoint, 'POST', data, config);
  }

  /**
   * PUT request
   */
  async put<T>(endpoint: string, data?: any, config?: ApiRequestConfig): Promise<ServiceResponse<T>> {
    return this.makeRequest<T>(endpoint, 'PUT', data, config);
  }

  /**
   * DELETE request
   */
  async delete<T>(endpoint: string, config?: ApiRequestConfig): Promise<ServiceResponse<T>> {
    return this.makeRequest<T>(endpoint, 'DELETE', undefined, config);
  }

  /**
   * PATCH request
   */
  async patch<T>(endpoint: string, data?: any, config?: ApiRequestConfig): Promise<ServiceResponse<T>> {
    return this.makeRequest<T>(endpoint, 'PATCH', data, config);
  }

  /**
   * Get API metrics for monitoring
   */
  getMetrics(): ApiMetrics[] {
    return [...this.metrics];
  }

  /**
   * Get cache statistics
   */
  getCacheStats(): { size: number; maxSize: number; hitRate?: number } {
    return {
      size: this.cache.size(),
      maxSize: this.config.cache?.maxSize || 0
    };
  }

  /**
   * Clear cache
   */
  clearCache(): void {
    this.cache.clear();
    console.log('🗑️ API cache cleared');
  }

  /**
   * Update configuration
   */
  updateConfig(newConfig: Partial<ApiClientConfig>): void {
    this.config = { ...this.config, ...newConfig };
    console.log('⚙️ API client configuration updated');
  }

  /**
   * Health check - test connection to API
   */
  async healthCheck(): Promise<{ healthy: boolean; responseTime: number; error?: string }> {
    const startTime = Date.now();

    try {
      const response = await this.get('/api/v1/health', { timeout: 5000, retries: 1 });
      const responseTime = Date.now() - startTime;

      return {
        healthy: response.success,
        responseTime,
        error: response.error
      };
    } catch (error: any) {
      const responseTime = Date.now() - startTime;
      return {
        healthy: false,
        responseTime,
        error: error.message
      };
    }
  }
}

// Create singleton instances
export const apiClient = new ProductionApiClient();

// Development instance with more verbose logging
export const devApiClient = process.env.NODE_ENV === 'development'
  ? new ProductionApiClient({
    timeout: 5000,
    retries: 1,
    cache: { enabled: false, ttl: 0, maxSize: 0 }
  })
  : apiClient;

export default apiClient;
