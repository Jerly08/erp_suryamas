package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Database
	DatabaseURL string

	// JWT Configuration
	JWTAccessSecret  string
	JWTRefreshSecret string
	JWTSecret        string // Backward compatibility
	JWTAccessExpiry  time.Duration
	JWTRefreshExpiry time.Duration
	JWTIssuer        string
	JWTAudience      string

	// Server
	ServerPort     string
	Environment    string
	AllowedOrigins []string
	TrustedProxies []string

	// Security
	EnableHTTPS           bool
	EnableRateLimit       bool
	EnableCSRF            bool
	EnableSecurityHeaders bool
	EnableMonitoring      bool
	MaxLoginAttempts      int
	LockoutDuration       time.Duration

	// Session
	SessionSecret      string
	SessionMaxAge      int
	SessionIdleTimeout int

	// CSRF
	CSRFSecret string

	// Rate Limiting
	RateLimitRequests     int
	RateLimitAuthRequests int
	RateLimitAPIRequests  int

	// Security Headers
	HSTSMaxAge    int
	CSPPolicy     string
	XFrameOptions string

	// Cookies
	CookieDomain   string
	CookieSecure   bool
	CookieHTTPOnly bool
	CookieSameSite string

	// Monitoring
	AlertEmail      string
	AlertWebhookURL string
	LogLevel        string
	LogFile         string

	// Development Flags
	SkipBalanceReset bool

	// Swagger Configuration
	SwaggerHost        string
	SwaggerScheme      string
	SwaggerBasePath    string
	SwaggerTitle       string
	SwaggerDescription string
}

func LoadConfig() *Config {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Try to load production env if in production
	if getEnv("ENVIRONMENT", "development") == "production" {
		if err := godotenv.Load(".env.production"); err != nil {
			log.Println("No .env.production file found")
		}
	}

	config := &Config{
		// Database
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost/sistem_akuntansi?sslmode=disable"),

		// JWT Configuration
		JWTAccessSecret:  getEnv("JWT_ACCESS_SECRET", getEnv("JWT_SECRET", generateDefaultSecret())),
		JWTRefreshSecret: getEnv("JWT_REFRESH_SECRET", getEnv("JWT_SECRET", generateDefaultSecret())),
		JWTSecret:        getEnv("JWT_SECRET", generateDefaultSecret()), // Backward compatibility
		JWTAccessExpiry:  parseDuration(getEnv("JWT_ACCESS_EXPIRY", "90m"), 90*time.Minute),
		JWTRefreshExpiry: parseDuration(getEnv("JWT_REFRESH_EXPIRY", "7d"), 7*24*time.Hour),
		JWTIssuer:        getEnv("JWT_ISSUER", "accounting-system"),
		JWTAudience:      getEnv("JWT_AUDIENCE", "accounting-app"),

		// Server
		ServerPort:     getEnv("SERVER_PORT", "8080"),
		Environment:    getEnv("ENVIRONMENT", "development"),
		AllowedOrigins: parseStringSlice(getEnv("ALLOWED_ORIGINS", "http://localhost:3000")),
		TrustedProxies: parseStringSlice(getEnv("TRUSTED_PROXIES", "")),

		// Security
		EnableHTTPS:           parseBool(getEnv("ENABLE_HTTPS", "false")),
		EnableRateLimit:       parseBool(getEnv("ENABLE_RATE_LIMIT", "true")),
		EnableCSRF:            parseBool(getEnv("ENABLE_CSRF", "false")),
		EnableSecurityHeaders: parseBool(getEnv("ENABLE_SECURITY_HEADERS", "true")),
		EnableMonitoring:      parseBool(getEnv("ENABLE_MONITORING", "true")),
		MaxLoginAttempts:      parseInt(getEnv("MAX_LOGIN_ATTEMPTS", "5"), 5),
		LockoutDuration:       parseDuration(getEnv("LOCKOUT_DURATION", "15m"), 15*time.Minute),

		// Session
		SessionSecret:      getEnv("SESSION_SECRET", generateDefaultSecret()),
		SessionMaxAge:      parseInt(getEnv("SESSION_MAX_AGE", "28800"), 28800),
		SessionIdleTimeout: parseInt(getEnv("SESSION_IDLE_TIMEOUT", "1800"), 1800),

		// CSRF
		CSRFSecret: getEnv("CSRF_SECRET", generateDefaultSecret()),

		// Rate Limiting
		RateLimitRequests:     parseInt(getEnv("RATE_LIMIT_REQUESTS_PER_MINUTE", "60"), 60),
		RateLimitAuthRequests: parseInt(getEnv("RATE_LIMIT_AUTH_REQUESTS_PER_MINUTE", "10"), 10),
		RateLimitAPIRequests:  parseInt(getEnv("RATE_LIMIT_API_REQUESTS_PER_MINUTE", "100"), 100),

		// Security Headers
		HSTSMaxAge:    parseInt(getEnv("HSTS_MAX_AGE", "31536000"), 31536000),
		CSPPolicy:     getEnv("CSP_POLICY", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline';"),
		XFrameOptions: getEnv("X_FRAME_OPTIONS", "DENY"),

		// Cookies
		CookieDomain:   getEnv("COOKIE_DOMAIN", ""),
		CookieSecure:   parseBool(getEnv("COOKIE_SECURE", "false")),
		CookieHTTPOnly: parseBool(getEnv("COOKIE_HTTP_ONLY", "true")),
		CookieSameSite: getEnv("COOKIE_SAME_SITE", "lax"),

		// Monitoring
		AlertEmail:      getEnv("ALERT_EMAIL", ""),
		AlertWebhookURL: getEnv("ALERT_WEBHOOK_URL", ""),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		LogFile:         getEnv("LOG_FILE", ""),

		// Development Flags
		SkipBalanceReset: parseBool(getEnv("SKIP_BALANCE_RESET", "false")),

		// Swagger Configuration
		SwaggerHost:        getEnv("SWAGGER_HOST", ""),       // Empty means dynamic
		SwaggerScheme:      getEnv("SWAGGER_SCHEME", "http"), // Default to http, production should set to https
		SwaggerBasePath:    getEnv("SWAGGER_BASE_PATH", "/api/v1"),
		SwaggerTitle:       getEnv("SWAGGER_TITLE", "Sistema Akuntansi API"),
		SwaggerDescription: getEnv("SWAGGER_DESCRIPTION", "API untuk aplikasi sistem akuntansi yang komprehensif dengan fitur lengkap manajemen keuangan, inventory, sales, purchases, dan reporting."),
	}

	// Production security warnings
	if config.Environment == "production" {
		if !config.EnableHTTPS {
			log.Println("⚠️  WARNING: HTTPS is disabled in production!")
		}
		if config.JWTAccessSecret == generateDefaultSecret() {
			log.Fatal("❌ FATAL: Using default JWT secret in production! Please set JWT_ACCESS_SECRET")
		}
	}

	if config.Environment == "production" {
		if config.JWTRefreshSecret == generateDefaultSecret() {
			log.Fatal("FATAL: Using default refresh JWT secret in production! Please set JWT_REFRESH_SECRET")
		}
		if config.SessionSecret == generateDefaultSecret() {
			log.Fatal("FATAL: Using default session secret in production! Please set SESSION_SECRET")
		}
	}

	return config
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseBool(value string) bool {
	boolVal, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return boolVal
}

func parseInt(value string, defaultValue int) int {
	intVal, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return intVal
}

func parseDuration(value string, defaultValue time.Duration) time.Duration {
	// Handle special cases
	if value == "7d" {
		return 7 * 24 * time.Hour
	}
	if value == "30d" {
		return 30 * 24 * time.Hour
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}
	return duration
}

func parseStringSlice(value string) []string {
	if value == "" {
		return []string{}
	}

	if strings.TrimSpace(value) == "*" {
		return []string{"*"}
	}

	var result []string
	for _, s := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func generateDefaultSecret() string {
	// This is ONLY for development. In production, this will trigger a fatal error
	// Use a consistent secret for development to prevent JWT validation issues
	return "DEVELOPMENT-ONLY-SECRET-CHANGE-IN-PRODUCTION-ACCOUNTING-SYSTEM-2024"
}
