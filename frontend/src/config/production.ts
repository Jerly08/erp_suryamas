/**
 * Production Environment Configuration
 * 
 * Comprehensive configuration management and validation 
 * for production deployment
 */

import { EnvironmentVariables, ProductionEnvVars, DevelopmentEnvVars } from '@/types/api';
import API_BASE_URL from '@/config/api';

// Environment configuration interface
export interface EnvironmentConfig {
  api: {
    baseURL: string;
    timeout: number;
    retries: number;
    validateEndpoints: boolean;
  };
  monitoring: {
    enabled: boolean;
    interval: number;
    logLevel: 'error' | 'warn' | 'info' | 'debug';
  };
  cache: {
    enabled: boolean;
    ttl: number;
    maxSize: number;
  };
  security: {
    enforceHttps: boolean;
    validateOrigin: boolean;
    allowedOrigins: string[];
  };
  performance: {
    enableCompression: boolean;
    cacheStaticAssets: boolean;
    lazyLoadImages: boolean;
  };
  features: {
    enableExperimentalFeatures: boolean;
    enableDebugMode: boolean;
    enableApiMocking: boolean;
  };
}

/**
 * Validate environment variables at startup
 */
export function validateEnvironmentVariables(): {
  isValid: boolean;
  errors: string[];
  warnings: string[];
  config: EnvironmentConfig;
} {
  const errors: string[] = [];
  const warnings: string[] = [];

  console.log('🔍 Validating environment configuration...');

  // Check NODE_ENV
  const nodeEnv = process.env.NODE_ENV;
  if (!nodeEnv) {
    errors.push('NODE_ENV is not set');
  } else if (!['development', 'staging', 'production'].includes(nodeEnv)) {
    warnings.push(`NODE_ENV="${nodeEnv}" is not a standard value`);
  }

  // Production-specific validations
  if (nodeEnv === 'production') {
    // Required production environment variables
    const requiredProdVars: (keyof ProductionEnvVars)[] = [
      'NEXT_PUBLIC_API_URL',
      'API_TIMEOUT',
      'API_RETRIES',
      'MONITORING_ENABLED',
      'LOGGING_LEVEL'
    ];

    for (const varName of requiredProdVars) {
      if (!process.env[varName]) {
        errors.push(`Production environment variable ${varName} is required but not set`);
      }
    }

    // Validate API URL format
    const apiUrl = process.env.NEXT_PUBLIC_API_URL;
    if (apiUrl) {
      try {
        new URL(apiUrl);
        if (!apiUrl.startsWith('https://')) {
          warnings.push('API URL should use HTTPS in production');
        }
      } catch {
        errors.push('NEXT_PUBLIC_API_URL is not a valid URL');
      }
    }

    // Validate numeric values
    const timeout = process.env.API_TIMEOUT;
    if (timeout && (isNaN(Number(timeout)) || Number(timeout) < 1000)) {
      errors.push('API_TIMEOUT must be a number >= 1000ms');
    }

    const retries = process.env.API_RETRIES;
    if (retries && (isNaN(Number(retries)) || Number(retries) < 1)) {
      errors.push('API_RETRIES must be a number >= 1');
    }

    // Validate logging level
    const logLevel = process.env.LOGGING_LEVEL;
    if (logLevel && !['error', 'warn', 'info', 'debug'].includes(logLevel)) {
      errors.push('LOGGING_LEVEL must be one of: error, warn, info, debug');
    }
  }

  // Build configuration object
  const config: EnvironmentConfig = {
    api: {
      baseURL: API_BASE_URL || 'http://localhost:8080',
      timeout: Number(process.env.API_TIMEOUT) || (nodeEnv === 'production' ? 15000 : 10000),
      retries: Number(process.env.API_RETRIES) || (nodeEnv === 'production' ? 3 : 1),
      validateEndpoints: process.env.VALIDATE_ENDPOINTS !== 'false'
    },
    monitoring: {
      enabled: process.env.MONITORING_ENABLED === 'true' || nodeEnv === 'production',
      interval: Number(process.env.MONITORING_INTERVAL) || 5,
      logLevel: (process.env.LOGGING_LEVEL as any) || (nodeEnv === 'production' ? 'warn' : 'info')
    },
    cache: {
      enabled: process.env.CACHE_ENABLED !== 'false' && nodeEnv === 'production',
      ttl: Number(process.env.CACHE_TTL) || (nodeEnv === 'production' ? 600000 : 300000), // 10min prod, 5min dev
      maxSize: Number(process.env.CACHE_MAX_SIZE) || (nodeEnv === 'production' ? 500 : 100)
    },
    security: {
      enforceHttps: process.env.ENFORCE_HTTPS === 'true' || nodeEnv === 'production',
      validateOrigin: process.env.VALIDATE_ORIGIN === 'true' || nodeEnv === 'production',
      allowedOrigins: process.env.ALLOWED_ORIGINS?.split(',') || []
    },
    performance: {
      enableCompression: process.env.ENABLE_COMPRESSION !== 'false',
      cacheStaticAssets: process.env.CACHE_STATIC_ASSETS !== 'false',
      lazyLoadImages: process.env.LAZY_LOAD_IMAGES !== 'false'
    },
    features: {
      enableExperimentalFeatures: process.env.ENABLE_EXPERIMENTAL_FEATURES === 'true',
      enableDebugMode: process.env.ENABLE_DEBUG_MODE === 'true' || nodeEnv === 'development',
      enableApiMocking: process.env.ENABLE_API_MOCKING === 'true' && nodeEnv !== 'production'
    }
  };

  // Additional validations based on config
  if (config.security.enforceHttps && !config.api.baseURL.startsWith('https://')) {
    if (nodeEnv === 'production') {
      errors.push('HTTPS enforcement is enabled but API URL is not HTTPS');
    } else {
      warnings.push('HTTPS enforcement enabled but API URL is HTTP (acceptable in development)');
    }
  }

  if (config.monitoring.enabled && nodeEnv === 'production' && !process.env.SENTRY_DSN && !process.env.NEW_RELIC_LICENSE_KEY) {
    warnings.push('Monitoring is enabled but no monitoring service is configured (Sentry, New Relic, etc.)');
  }

  const isValid = errors.length === 0;

  // Log results
  if (isValid) {
    console.log('✅ Environment validation passed');
    if (warnings.length > 0) {
      console.warn('⚠️ Environment warnings:', warnings);
    }
  } else {
    console.error('❌ Environment validation failed:', errors);
    if (warnings.length > 0) {
      console.warn('⚠️ Additional warnings:', warnings);
    }
  }

  console.log('⚙️ Environment configuration:', {
    environment: nodeEnv,
    api: {
      baseURL: config.api.baseURL,
      timeout: `${config.api.timeout}ms`,
      retries: config.api.retries
    },
    monitoring: config.monitoring.enabled ? 'enabled' : 'disabled',
    cache: config.cache.enabled ? 'enabled' : 'disabled',
    features: {
      debug: config.features.enableDebugMode,
      experimental: config.features.enableExperimentalFeatures
    }
  });

  return { isValid, errors, warnings, config };
}

/**
 * Get environment-specific configuration
 */
export function getEnvironmentConfig(): EnvironmentConfig {
  const result = validateEnvironmentVariables();
  return result.config;
}

/**
 * Production readiness checker
 */
export async function checkProductionReadiness(): Promise<{
  ready: boolean;
  checks: {
    name: string;
    status: 'pass' | 'fail' | 'warn';
    message: string;
  }[];
  score: number;
}> {
  console.log('🚀 Checking production readiness...');

  const checks: { name: string; status: 'pass' | 'fail' | 'warn'; message: string; }[] = [];
  const config = getEnvironmentConfig();

  // Environment variables check
  const envValidation = validateEnvironmentVariables();
  checks.push({
    name: 'Environment Variables',
    status: envValidation.isValid ? 'pass' : 'fail',
    message: envValidation.isValid ?
      'All required environment variables are configured' :
      `Missing or invalid variables: ${envValidation.errors.join(', ')}`
  });

  // API connectivity check
  try {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 5000);

    const response = await fetch(`${config.api.baseURL}/api/v1/health`, {
      signal: controller.signal,
      headers: { 'Cache-Control': 'no-cache' }
    });

    clearTimeout(timeout);

    checks.push({
      name: 'API Connectivity',
      status: response.ok ? 'pass' : 'warn',
      message: response.ok ?
        'API server is reachable' :
        `API server returned ${response.status}: ${response.statusText}`
    });
  } catch (error: any) {
    checks.push({
      name: 'API Connectivity',
      status: 'fail',
      message: `Cannot connect to API: ${error.message}`
    });
  }

  // HTTPS check for production
  if (process.env.NODE_ENV === 'production') {
    checks.push({
      name: 'HTTPS Security',
      status: config.api.baseURL.startsWith('https://') ? 'pass' : 'fail',
      message: config.api.baseURL.startsWith('https://') ?
        'API uses HTTPS' :
        'API should use HTTPS in production'
    });
  }

  // Monitoring configuration check
  const hasMonitoring = !!(process.env.SENTRY_DSN || process.env.NEW_RELIC_LICENSE_KEY);
  checks.push({
    name: 'Error Monitoring',
    status: hasMonitoring || process.env.NODE_ENV !== 'production' ? 'pass' : 'warn',
    message: hasMonitoring ?
      'Error monitoring is configured' :
      'Consider configuring error monitoring (Sentry, New Relic, etc.)'
  });

  // Performance optimizations check
  const performanceOptimized = config.cache.enabled && config.performance.enableCompression;
  checks.push({
    name: 'Performance Optimizations',
    status: performanceOptimized ? 'pass' : 'warn',
    message: performanceOptimized ?
      'Caching and compression are enabled' :
      'Consider enabling caching and compression for better performance'
  });

  // Security configurations check
  const securityConfigured = config.security.enforceHttps && config.security.validateOrigin;
  checks.push({
    name: 'Security Configuration',
    status: securityConfigured || process.env.NODE_ENV !== 'production' ? 'pass' : 'warn',
    message: securityConfigured ?
      'Security features are properly configured' :
      'Consider enabling HTTPS enforcement and origin validation'
  });

  // Calculate readiness score
  const passCount = checks.filter(c => c.status === 'pass').length;
  const failCount = checks.filter(c => c.status === 'fail').length;
  const score = Math.round((passCount / checks.length) * 100);
  const ready = failCount === 0 && score >= 80;

  console.log('📊 Production readiness results:');
  checks.forEach(check => {
    const icon = check.status === 'pass' ? '✅' : check.status === 'warn' ? '⚠️' : '❌';
    console.log(`${icon} ${check.name}: ${check.message}`);
  });

  console.log(`🎯 Readiness score: ${score}% (${ready ? 'READY' : 'NOT READY'})`);

  return { ready, checks, score };
}

/**
 * Initialize application with environment validation
 */
export async function initializeProduction(): Promise<{
  success: boolean;
  config: EnvironmentConfig;
  errors: string[];
}> {
  console.log('🔧 Initializing production environment...');

  try {
    // Validate environment
    const envResult = validateEnvironmentVariables();
    if (!envResult.isValid) {
      return {
        success: false,
        config: envResult.config,
        errors: envResult.errors
      };
    }

    // Check production readiness in production mode
    if (process.env.NODE_ENV === 'production') {
      const readinessCheck = await checkProductionReadiness();
      if (!readinessCheck.ready) {
        const criticalFailures = readinessCheck.checks
          .filter(c => c.status === 'fail')
          .map(c => c.message);

        return {
          success: false,
          config: envResult.config,
          errors: criticalFailures
        };
      }
    }

    console.log('🎉 Production environment initialized successfully');

    return {
      success: true,
      config: envResult.config,
      errors: []
    };

  } catch (error: any) {
    console.error('💥 Failed to initialize production environment:', error);

    return {
      success: false,
      config: getEnvironmentConfig(),
      errors: [error.message]
    };
  }
}

// Export the configuration for use across the application
export const environment = getEnvironmentConfig();