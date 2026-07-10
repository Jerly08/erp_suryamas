package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"app-sistem-akuntansi/config"
	"github.com/gin-gonic/gin"
)

func TestAuthRateLimitDoesNotCountSessionValidationRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	InitializeRateLimiters(&config.Config{
		EnableRateLimit:       true,
		RateLimitAuthRequests: 2,
		LockoutDuration:       time.Minute,
	})
	t.Cleanup(func() {
		InitializeRateLimiters(&config.Config{EnableRateLimit: false})
	})

	router := gin.New()
	router.GET("/api/v1/auth/validate-token", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.POST("/api/v1/auth/login", AuthRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 12; i++ {
		response := performRateLimitRequest(router, http.MethodGet, "/api/v1/auth/validate-token")
		if response.Code != http.StatusOK {
			t.Fatalf("session validation request %d returned %d", i+1, response.Code)
		}
	}

	for i := 0; i < 2; i++ {
		response := performRateLimitRequest(router, http.MethodPost, "/api/v1/auth/login")
		if response.Code != http.StatusOK {
			t.Fatalf("allowed login request %d returned %d", i+1, response.Code)
		}
	}

	blocked := performRateLimitRequest(router, http.MethodPost, "/api/v1/auth/login")
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("expected login request to be rate limited, got %d", blocked.Code)
	}
	if !strings.Contains(blocked.Body.String(), "AUTH_RATE_LIMIT_EXCEEDED") {
		t.Fatalf("expected auth rate-limit error code, got %s", blocked.Body.String())
	}
}

func performRateLimitRequest(router http.Handler, method, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	request.RemoteAddr = "203.0.113.10:43210"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
