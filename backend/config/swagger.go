package config

import (
	"fmt"
	"os"
	"strings"
)

// SwaggerConfig holds dynamic Swagger configuration
type SwaggerConfig struct {
	Host        string
	Scheme      string
	BasePath    string
	Title       string
	Description string
}

// GetSwaggerConfig returns dynamic Swagger configuration based on environment
func GetSwaggerConfig() *SwaggerConfig {
	cfg := LoadConfig()

	// Determine host dynamically
	host := cfg.SwaggerHost
	if host == "" {
		// If not explicitly set, generate from environment
		if cfg.Environment == "production" {
			// In production, try to get from various sources
			if domain := os.Getenv("DOMAIN"); domain != "" {
				host = domain
			} else if appURL := os.Getenv("APP_URL"); appURL != "" {
				// Extract host from APP_URL (remove protocol and path)
				host = strings.TrimPrefix(appURL, "https://")
				host = strings.TrimPrefix(host, "http://")
				host = strings.Split(host, "/")[0]
			} else {
				// Fallback for production
				host = "api.yourdomain.com"
			}
		} else {
			// Development default
			port := cfg.ServerPort
			if port == "" {
				port = "8080"
			}
			host = fmt.Sprintf("localhost:%s", port)
		}
	}

	// Determine scheme
	scheme := cfg.SwaggerScheme
	if scheme == "" {
		if cfg.Environment == "production" || cfg.EnableHTTPS {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	return &SwaggerConfig{
		Host:        host,
		Scheme:      scheme,
		BasePath:    cfg.SwaggerBasePath,
		Title:       cfg.SwaggerTitle,
		Description: cfg.SwaggerDescription,
	}
}

// GetSwaggerURL returns the full Swagger UI URL
func (sc *SwaggerConfig) GetSwaggerURL() string {
	return fmt.Sprintf("%s://%s/swagger/index.html", sc.Scheme, sc.Host)
}

// GetAPIBaseURL returns the full API base URL
func (sc *SwaggerConfig) GetAPIBaseURL() string {
	return fmt.Sprintf("%s://%s%s", sc.Scheme, sc.Host, sc.BasePath)
}

// GetAllowedOrigins returns CORS origins based on configuration
func GetAllowedOrigins(cfg *Config) []string {
	// Check if explicitly set in config
	if len(cfg.AllowedOrigins) > 0 {
		return cfg.AllowedOrigins
	}

	// Generate based on environment
	if cfg.Environment == "production" {
		// In production, should be explicitly set, but provide sensible defaults
		origins := []string{}

		// Try to determine from various environment variables
		if domain := os.Getenv("FRONTEND_URL"); domain != "" {
			origins = append(origins, domain)
		} else if domain := os.Getenv("DOMAIN"); domain != "" {
			if cfg.EnableHTTPS {
				origins = append(origins, fmt.Sprintf("https://%s", domain))
			} else {
				origins = append(origins, fmt.Sprintf("http://%s", domain))
			}
		} else if appURL := os.Getenv("APP_URL"); appURL != "" {
			origins = append(origins, appURL)
		}

		// If still empty, warn and provide fallback
		if len(origins) == 0 {
			fmt.Println("⚠️  WARNING: No CORS origins configured for production. Please set ALLOWED_ORIGINS or FRONTEND_URL")
			origins = []string{"https://yourdomain.com"}
		}

		return origins
	} else {
		// Development defaults
		return []string{
			"http://localhost:3000",
			"http://localhost:3001",
			"http://localhost:3002",
			"http://127.0.0.1:3000",
			"http://127.0.0.1:3001",
			"http://127.0.0.1:3002",
		}
	}
}
