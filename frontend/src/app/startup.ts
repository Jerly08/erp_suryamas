/**
 * Application Startup & Production Validation
 * 
 * This module handles application initialization, environment validation,
 * and production readiness checks to prevent API issues
 */

import { 
  initializeProduction, 
  checkProductionReadiness, 
  validateEnvironmentVariables 
} from '@/config/production';

import { 
  validateCriticalEndpoints, 
  startAPIMonitoring 
} from '@/utils/apiValidation';

import { 
  compareEndpointsWithSwagger, 
  validateAPIAgainstSwagger 
} from '@/utils/apiDocumentation';

import { apiClient } from '@/services/apiClient';

// Startup result interface
export interface StartupResult {
  success: boolean;
  environment: string;
  errors: string[];
  warnings: string[];
  readinessScore: number;
  apiHealth: {
    endpointsValidated: number;
    healthyEndpoints: number;
    failedEndpoints: string[];
  };
  recommendations: string[];
}

/**
 * Initialize application with comprehensive validation
 */
export async function initializeApplication(): Promise<StartupResult> {
  console.log('🚀 Starting application initialization...');
  const startTime = Date.now();
  
  const errors: string[] = [];
  const warnings: string[] = [];
  const recommendations: string[] = [];
  
  try {
    // 1. Environment Validation
    console.log('📋 Step 1: Validating environment configuration...');
    const envValidation = validateEnvironmentVariables();
    
    if (!envValidation.isValid) {
      errors.push(...envValidation.errors);
      console.error('❌ Environment validation failed');
    }
    
    if (envValidation.warnings.length > 0) {
      warnings.push(...envValidation.warnings);
    }
    
    // 2. Production Readiness Check (for production only)
    let readinessScore = 100;
    if (process.env.NODE_ENV === 'production') {
      console.log('📋 Step 2: Checking production readiness...');
      const readinessCheck = await checkProductionReadiness();
      readinessScore = readinessCheck.score;
      
      if (!readinessCheck.ready) {
        const criticalIssues = readinessCheck.checks
          .filter(c => c.status === 'fail')
          .map(c => c.message);
        errors.push(...criticalIssues);
      }
      
      const warnings = readinessCheck.checks
        .filter(c => c.status === 'warn')
        .map(c => c.message);
      
      if (warnings.length > 0) {
        warnings.push(...warnings);
      }
    }
    
    // 3. API Endpoint Validation
    console.log('📋 Step 3: Validating API endpoints...');
    let apiHealth = {
      endpointsValidated: 0,
      healthyEndpoints: 0,
      failedEndpoints: [] as string[]
    };
    
    try {
      // Compare with Swagger documentation
      const swaggerComparison = compareEndpointsWithSwagger();
      console.log(`🔍 Swagger comparison: ${swaggerComparison.matches}/${swaggerComparison.totalChecked} endpoints match`);
      
      if (swaggerComparison.mismatches > 0) {
        warnings.push(`${swaggerComparison.mismatches} API endpoints have mismatched paths`);
      }
      
      if (swaggerComparison.missingInFrontend > 0) {
        warnings.push(`${swaggerComparison.missingInFrontend} API endpoints are missing in frontend`);
      }
      
      // Validate critical endpoints
      const endpointValidation = await validateCriticalEndpoints();
      apiHealth = {
        endpointsValidated: endpointValidation.totalEndpoints,
        healthyEndpoints: endpointValidation.healthyEndpoints,
        failedEndpoints: endpointValidation.failedEndpoints
      };
      
      if (!endpointValidation.allEndpointsValid) {
        if (process.env.NODE_ENV === 'production') {
          errors.push(`Critical API endpoints are failing: ${endpointValidation.failedEndpoints.join(', ')}`);
        } else {
          warnings.push(`Some API endpoints are not accessible (acceptable in development)`);
        }
      }
      
    } catch (error: any) {
      warnings.push(`API validation error: ${error.message}`);
    }
    
    // 4. API Client Health Check
    console.log('📋 Step 4: Testing API client connectivity...');
    try {
      const healthCheck = await apiClient.healthCheck();
      if (!healthCheck.healthy) {
        if (process.env.NODE_ENV === 'production') {
          errors.push(`API server is not responding: ${healthCheck.error}`);
        } else {
          warnings.push(`API server is not responding (acceptable in development): ${healthCheck.error}`);
        }
      } else {
        console.log(`✅ API server responded in ${healthCheck.responseTime}ms`);
      }
    } catch (error: any) {
      warnings.push(`API health check failed: ${error.message}`);
    }
    
    // 5. Start Production Monitoring (production only)
    if (process.env.NODE_ENV === 'production') {
      console.log('📋 Step 5: Starting production monitoring...');
      try {
        startAPIMonitoring(5); // Monitor every 5 minutes
        console.log('✅ Production monitoring started');
      } catch (error: any) {
        warnings.push(`Failed to start monitoring: ${error.message}`);
      }
    }
    
    // Generate recommendations
    if (errors.length > 0) {
      recommendations.push('🚨 Fix critical errors before proceeding');
      recommendations.push('📖 Check deployment documentation for troubleshooting');
    }
    
    if (warnings.length > 0) {
      recommendations.push('⚠️ Review warnings for optimal performance');
    }
    
    if (readinessScore < 90) {
      recommendations.push('🔧 Improve production readiness score');
    }
    
    if (apiHealth.failedEndpoints.length > 0) {
      recommendations.push('🔗 Verify backend API server is running and accessible');
    }
    
    // Log final results
    const initTime = Date.now() - startTime;
    const success = errors.length === 0;
    
    if (success) {
      console.log(`🎉 Application initialized successfully in ${initTime}ms`);
      console.log(`📊 Readiness score: ${readinessScore}%`);
      console.log(`🔗 API health: ${apiHealth.healthyEndpoints}/${apiHealth.endpointsValidated} endpoints healthy`);
    } else {
      console.error(`💥 Application initialization failed in ${initTime}ms`);
      console.error(`❌ Errors: ${errors.length}`);
      console.error(`⚠️ Warnings: ${warnings.length}`);
    }
    
    return {
      success,
      environment: process.env.NODE_ENV || 'unknown',
      errors,
      warnings,
      readinessScore,
      apiHealth,
      recommendations
    };
    
  } catch (error: any) {
    console.error('💥 Critical error during application initialization:', error);
    
    return {
      success: false,
      environment: process.env.NODE_ENV || 'unknown',
      errors: [`Critical initialization error: ${error.message}`],
      warnings,
      readinessScore: 0,
      apiHealth: {
        endpointsValidated: 0,
        healthyEndpoints: 0,
        failedEndpoints: ['initialization-failed']
      },
      recommendations: [
        '🚨 Check application logs for detailed error information',
        '🔧 Verify environment configuration',
        '📞 Contact development team if issue persists'
      ]
    };
  }
}

/**
 * Validate API configuration at runtime
 */
export async function validateAPIConfiguration(): Promise<{
  valid: boolean;
  issues: string[];
  suggestions: string[];
}> {
  const issues: string[] = [];
  const suggestions: string[] = [];
  
  try {
    // Run comprehensive API validation
    const validation = await validateAPIAgainstSwagger();
    
    // Check endpoint comparison
    if (validation.endpointComparison.mismatches > 0) {
      issues.push(`${validation.endpointComparison.mismatches} API endpoints have path mismatches`);
      suggestions.push('Update API_ENDPOINTS configuration to match Swagger documentation');
    }
    
    if (validation.endpointComparison.missingInFrontend > 0) {
      issues.push(`${validation.endpointComparison.missingInFrontend} API endpoints are missing in frontend`);
      suggestions.push('Consider adding missing endpoints to frontend configuration');
    }
    
    // Check health status
    if (!validation.healthCheck.allEndpointsValid) {
      issues.push(`${validation.healthCheck.failedEndpoints.length} critical endpoints are failing`);
      suggestions.push('Fix API connectivity issues before deployment');
    }
    
    // Add validation recommendations
    suggestions.push(...validation.recommendations);
    
    return {
      valid: issues.length === 0,
      issues,
      suggestions
    };
    
  } catch (error: any) {
    return {
      valid: false,
      issues: [`API validation failed: ${error.message}`],
      suggestions: [
        'Check API server connectivity',
        'Verify environment configuration',
        'Review API endpoint definitions'
      ]
    };
  }
}

/**
 * Get application health status for monitoring
 */
export async function getApplicationHealth(): Promise<{
  status: 'healthy' | 'degraded' | 'unhealthy';
  checks: {
    name: string;
    status: 'pass' | 'fail';
    message: string;
    responseTime?: number;
  }[];
  uptime: number;
  timestamp: string;
}> {
  const startTime = Date.now();
  const checks: Array<{
    name: string;
    status: 'pass' | 'fail';
    message: string;
    responseTime?: number;
  }> = [];
  
  // Environment check
  const envValidation = validateEnvironmentVariables();
  checks.push({
    name: 'Environment Configuration',
    status: envValidation.isValid ? 'pass' : 'fail',
    message: envValidation.isValid ? 'Configuration valid' : `${envValidation.errors.length} configuration errors`
  });
  
  // API connectivity check
  try {
    const healthCheck = await apiClient.healthCheck();
    checks.push({
      name: 'API Connectivity',
      status: healthCheck.healthy ? 'pass' : 'fail',
      message: healthCheck.healthy ? 'API server accessible' : `API error: ${healthCheck.error}`,
      responseTime: healthCheck.responseTime
    });
  } catch (error: any) {
    checks.push({
      name: 'API Connectivity',
      status: 'fail',
      message: `API check failed: ${error.message}`
    });
  }
  
  // Critical endpoints check
  try {
    const endpointCheck = await validateCriticalEndpoints();
    checks.push({
      name: 'Critical Endpoints',
      status: endpointCheck.allEndpointsValid ? 'pass' : 'fail',
      message: `${endpointCheck.healthyEndpoints}/${endpointCheck.totalEndpoints} endpoints healthy`
    });
  } catch (error: any) {
    checks.push({
      name: 'Critical Endpoints',
      status: 'fail',
      message: `Endpoint validation failed: ${error.message}`
    });
  }
  
  // Determine overall status
  const failedChecks = checks.filter(c => c.status === 'fail').length;
  const status = failedChecks === 0 ? 'healthy' : 
                failedChecks <= 1 ? 'degraded' : 'unhealthy';
  
  return {
    status,
    checks,
    uptime: process.uptime ? process.uptime() * 1000 : Date.now() - startTime,
    timestamp: new Date().toISOString()
  };
}

/**
 * Export startup functions for use in app initialization
 */
export default {
  initializeApplication,
  validateAPIConfiguration,
  getApplicationHealth
};
