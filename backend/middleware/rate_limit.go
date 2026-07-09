package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"app-sistem-akuntansi/config"
	"github.com/gin-gonic/gin"
)

// RateLimitRecord represents a rate limit entry
type RateLimitEntry struct {
	Count        int
	WindowStart  time.Time
	BlockedUntil *time.Time
}

// RateLimiter manages rate limiting
type RateLimiter struct {
	mu            sync.RWMutex
	entries       map[string]*RateLimitEntry
	limit         int
	window        time.Duration
	blockDuration time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration, blockDuration time.Duration) *RateLimiter {
	rl := &RateLimiter{
		entries:       make(map[string]*RateLimitEntry),
		limit:         limit,
		window:        window,
		blockDuration: blockDuration,
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// IsAllowed checks if a request from the given key is allowed
func (rl *RateLimiter) IsAllowed(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.entries[key]

	if !exists {
		rl.entries[key] = &RateLimitEntry{
			Count:       1,
			WindowStart: now,
		}
		return true
	}

	// Check if currently blocked
	if entry.BlockedUntil != nil && now.Before(*entry.BlockedUntil) {
		return false
	}

	// Reset window if expired
	if now.Sub(entry.WindowStart) > rl.window {
		entry.Count = 1
		entry.WindowStart = now
		entry.BlockedUntil = nil
		return true
	}

	// Increment count
	entry.Count++

	// Check if limit exceeded
	if entry.Count > rl.limit {
		blockUntil := now.Add(rl.blockDuration)
		entry.BlockedUntil = &blockUntil
		return false
	}

	return true
}

// GetRemainingRequests returns the number of remaining requests in the current window
func (rl *RateLimiter) GetRemainingRequests(key string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	entry, exists := rl.entries[key]
	if !exists {
		return rl.limit
	}

	now := time.Now()
	if now.Sub(entry.WindowStart) > rl.window {
		return rl.limit
	}

	remaining := rl.limit - entry.Count
	if remaining < 0 {
		return 0
	}
	return remaining
}

// cleanup removes expired entries
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()

		for key, entry := range rl.entries {
			// Remove entries that are old and not blocked
			if now.Sub(entry.WindowStart) > rl.window*2 &&
				(entry.BlockedUntil == nil || now.After(*entry.BlockedUntil)) {
				delete(rl.entries, key)
			}
		}
		rl.mu.Unlock()
	}
}

// Global rate limiters for different endpoints
var (
	paymentRateLimiter *RateLimiter
	authRateLimiter    *RateLimiter
	generalRateLimiter *RateLimiter
	monitoringChan     = make(chan RateLimitEvent, 1000)
)

// RateLimitEvent for monitoring
type RateLimitEvent struct {
	Timestamp         time.Time
	ClientIP          string
	Endpoint          string
	Allowed           bool
	Reason            string
	RemainingRequests int
}

// InitializeRateLimiters initializes rate limiters with config
func InitializeRateLimiters(cfg *config.Config) {
	if !cfg.EnableRateLimit {
		paymentRateLimiter = nil
		authRateLimiter = nil
		generalRateLimiter = nil
		return
	}

	// Initialize based on config
	paymentRateLimiter = NewRateLimiter(
		cfg.RateLimitAPIRequests,
		time.Minute,
		cfg.LockoutDuration,
	)

	authRateLimiter = NewRateLimiter(
		cfg.RateLimitAuthRequests,
		time.Minute,
		cfg.LockoutDuration,
	)

	generalRateLimiter = NewRateLimiter(
		cfg.RateLimitRequests,
		time.Minute,
		2*time.Minute,
	)

	// Start monitoring if enabled
	if cfg.EnableMonitoring {
		go startRateLimitMonitoring()
	}
}

// startRateLimitMonitoring monitors rate limit events
func startRateLimitMonitoring() {
	for event := range monitoringChan {
		// Log the event
		if !event.Allowed {
			logRateLimitViolation(event)
		}
	}
}

// logRateLimitViolation logs rate limit violations
func logRateLimitViolation(event RateLimitEvent) {
	// This can be enhanced to send alerts, write to database, etc.
	fmt.Printf("[RATE_LIMIT_VIOLATION] Time: %s, IP: %s, Endpoint: %s, Reason: %s\n",
		event.Timestamp.Format(time.RFC3339),
		event.ClientIP,
		event.Endpoint,
		event.Reason,
	)
}

// PaymentRateLimit middleware for payment endpoints
func PaymentRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if paymentRateLimiter == nil {
			c.Next()
			return
		}

		key := getClientKey(c)
		allowed := paymentRateLimiter.IsAllowed(key)
		remaining := paymentRateLimiter.GetRemainingRequests(key)

		// Send monitoring event
		select {
		case monitoringChan <- RateLimitEvent{
			Timestamp:         time.Now(),
			ClientIP:          key,
			Endpoint:          c.Request.URL.Path,
			Allowed:           allowed,
			Reason:            "payment_endpoint",
			RemainingRequests: remaining,
		}:
		default:
			// Channel full, skip monitoring
		}

		if !allowed {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Payment rate limit exceeded. Please try again later.",
				"code":        "PAYMENT_RATE_LIMIT_EXCEEDED",
				"retry_after": paymentRateLimiter.blockDuration.String(),
			})
			c.Abort()
			return
		}

		// Add rate limit headers
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", paymentRateLimiter.limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Header("X-RateLimit-Window", "60")
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Minute).Unix()))

		c.Next()
	}
}

// AuthRateLimit middleware for authentication endpoints
func AuthRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if authRateLimiter == nil {
			c.Next()
			return
		}

		key := getClientKey(c)
		allowed := authRateLimiter.IsAllowed(key)
		remaining := authRateLimiter.GetRemainingRequests(key)

		// Send monitoring event
		select {
		case monitoringChan <- RateLimitEvent{
			Timestamp:         time.Now(),
			ClientIP:          key,
			Endpoint:          c.Request.URL.Path,
			Allowed:           allowed,
			Reason:            "auth_endpoint",
			RemainingRequests: remaining,
		}:
		default:
			// Channel full, skip monitoring
		}

		if !allowed {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Authentication rate limit exceeded. Please try again later.",
				"code":        "AUTH_RATE_LIMIT_EXCEEDED",
				"retry_after": authRateLimiter.blockDuration.String(),
			})
			c.Abort()
			return
		}

		// Add rate limit headers
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", authRateLimiter.limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Header("X-RateLimit-Window", "60")
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Minute).Unix()))

		c.Next()
	}
}

// GeneralRateLimit middleware for general endpoints
func GeneralRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if generalRateLimiter == nil {
			c.Next()
			return
		}

		key := getClientKey(c)

		if !generalRateLimiter.IsAllowed(key) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded. Please try again later.",
				"code":        "RATE_LIMIT_EXCEEDED",
				"retry_after": "2 minutes",
			})
			c.Abort()
			return
		}

		// Add rate limit headers
		remaining := generalRateLimiter.GetRemainingRequests(key)
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", generalRateLimiter.limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Header("X-RateLimit-Window", "60")

		c.Next()
	}
}

// getClientKey generates a key for rate limiting based on IP and user
func getClientKey(c *gin.Context) string {
	// Try to get user ID from context first
	userID := c.GetUint("user_id")
	if userID != 0 {
		return fmt.Sprintf("user_%d", userID)
	}

	// Fall back to IP address
	clientIP := c.ClientIP()
	return fmt.Sprintf("ip_%s", clientIP)
}

// RateLimit is an alias for GeneralRateLimit for convenience
func RateLimit() gin.HandlerFunc {
	return GeneralRateLimit()
}

// GetRateLimitStatus returns current rate limit status for debugging
func GetRateLimitStatus(c *gin.Context) gin.H {
	key := getClientKey(c)

	if paymentRateLimiter == nil || authRateLimiter == nil || generalRateLimiter == nil {
		return gin.H{
			"enabled":    false,
			"client_key": key,
		}
	}

	return gin.H{
		"enabled":           true,
		"payment_remaining": paymentRateLimiter.GetRemainingRequests(key),
		"auth_remaining":    authRateLimiter.GetRemainingRequests(key),
		"general_remaining": generalRateLimiter.GetRemainingRequests(key),
		"client_key":        key,
	}
}
