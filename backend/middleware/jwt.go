package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

type JWTManager struct {
	DB *gorm.DB
}

func NewJWTManager(db *gorm.DB) *JWTManager {
	return &JWTManager{DB: db}
}

func parseEnhancedToken(tokenString, secret string) (*jwt.Token, *EnhancedClaims, error) {
	claims := &EnhancedClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, nil, err
	}
	return token, claims, nil
}

// Enhanced Claims with additional security fields
type EnhancedClaims struct {
	UserID      uint     `json:"user_id"`
	Username    string   `json:"username"`
	Email       string   `json:"email"`
	Role        string   `json:"role"`
	SessionID   string   `json:"session_id"`
	DeviceInfo  string   `json:"device_info"`
	IPAddress   string   `json:"ip_address"`
	TokenType   string   `json:"token_type"` // access or refresh
	Permissions []string `json:"permissions,omitempty"`
	jwt.RegisteredClaims
}

// GenerateTokenPair generates both access and refresh tokens
func (jm *JWTManager) GenerateTokenPair(user models.User, deviceInfo, ipAddress string) (*models.TokenResponse, error) {
	deviceInfo = sanitizeTokenMetadata(deviceInfo, "Web Browser", 500)
	ipAddress = sanitizeTokenMetadata(ipAddress, "", 45)

	// Generate session ID
	sessionID, err := generateSecureToken(32)
	if err != nil {
		return nil, err
	}

	// Create user session
	session := models.UserSession{
		SessionID:    sessionID,
		UserID:       user.ID,
		IPAddress:    ipAddress,
		DeviceInfo:   deviceInfo,
		LastActivity: time.Now(),
		ExpiresAt:    time.Now().Add(7 * 24 * time.Hour), // 7 days
		IsActive:     true,
	}

	if err := jm.DB.Create(&session).Error; err != nil {
		return nil, err
	}

	return jm.issueTokenPair(user, sessionID, deviceInfo, ipAddress, time.Now().Add(7*24*time.Hour))
}

// RefreshAccessToken generates a new access token using refresh token.
// The refresh token is rotated on every successful refresh to reduce replay risk
// without changing the response contract used by the frontend.
func (jm *JWTManager) RefreshAccessToken(refreshTokenString, requestDeviceInfo, requestIPAddress, requestUserAgent string) (*models.TokenResponse, error) {
	var userID uint
	var username string
	var sessionID string
	var success bool
	var failureReason string
	var refreshCount int
	var monitorExpiry time.Time

	defer func() {
		if GlobalTokenMonitor != nil {
			GlobalTokenMonitor.LogRefreshAttempt(
				userID,
				username,
				sessionID,
				sanitizeTokenMetadata(requestIPAddress, "", 45),
				sanitizeTokenMetadata(requestUserAgent, "", 1000),
				success,
				failureReason,
				monitorExpiry,
				refreshCount,
			)
		}
	}()

	cfg := config.LoadConfig()

	token, claims, err := parseEnhancedToken(refreshTokenString, cfg.JWTRefreshSecret)
	if err != nil || !token.Valid || claims.TokenType != "refresh" {
		failureReason = "invalid_refresh_token"
		return nil, fmt.Errorf("invalid refresh token")
	}

	var refreshToken models.RefreshToken
	if err := jm.DB.Where("token = ? AND is_revoked = ?", refreshTokenString, false).First(&refreshToken).Error; err != nil {
		failureReason = "refresh_token_not_found"
		return nil, fmt.Errorf("refresh token not found or revoked")
	}

	if refreshToken.ExpiresAt.Before(time.Now()) {
		failureReason = "refresh_token_expired"
		return nil, fmt.Errorf("refresh token expired")
	}

	var user models.User
	if err := jm.DB.First(&user, refreshToken.UserID).Error; err != nil {
		failureReason = "user_not_found"
		return nil, fmt.Errorf("user not found")
	}

	if !user.IsActive {
		failureReason = "user_disabled"
		return nil, fmt.Errorf("user account is disabled")
	}

	userID = user.ID
	username = user.Username
	sessionID = claims.SessionID

	deviceInfo := sanitizeTokenMetadata(requestDeviceInfo, "", 500)
	if deviceInfo == "" {
		deviceInfo = sanitizeTokenMetadata(refreshToken.DeviceInfo, claims.DeviceInfo, 500)
	}

	ipAddress := sanitizeTokenMetadata(requestIPAddress, "", 45)
	if ipAddress == "" {
		ipAddress = sanitizeTokenMetadata(refreshToken.IPAddress, claims.IPAddress, 45)
	}

	var response *models.TokenResponse
	err = jm.DB.Transaction(func(tx *gorm.DB) error {
		txManager := &JWTManager{DB: tx}

		permissions, err := txManager.getUserPermissions(user.Role)
		if err != nil {
			return err
		}

		if err := tx.Model(&models.RefreshToken{}).
			Where("id = ?", refreshToken.ID).
			Updates(map[string]interface{}{"is_revoked": true, "updated_at": time.Now()}).Error; err != nil {
			return err
		}

		if err := tx.Model(&models.UserSession{}).
			Where("session_id = ?", claims.SessionID).
			Updates(map[string]interface{}{
				"last_activity": time.Now(),
				"ip_address":    ipAddress,
				"device_info":   deviceInfo,
				"user_agent":    sanitizeTokenMetadata(requestUserAgent, "", 1000),
			}).Error; err != nil {
			return err
		}

		// Ensure access token claims reflect the current refresh context, while keeping the same session.
		response, err = txManager.issueTokenPairWithPermissions(user, permissions, claims.SessionID, deviceInfo, ipAddress, refreshToken.ExpiresAt)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		failureReason = "refresh_rotation_failed"
		return nil, err
	}

	success = true
	monitorExpiry = response.ExpiresAt

	var activeRefreshCount int64
	jm.DB.Model(&models.RefreshToken{}).
		Where("user_id = ? AND is_revoked = ?", user.ID, false).
		Count(&activeRefreshCount)
	refreshCount = int(activeRefreshCount)

	return response, nil
}

func (jm *JWTManager) issueTokenPair(user models.User, sessionID, deviceInfo, ipAddress string, refreshTokenExpiry time.Time) (*models.TokenResponse, error) {
	permissions, err := jm.getUserPermissions(user.Role)
	if err != nil {
		return nil, err
	}

	return jm.issueTokenPairWithPermissions(user, permissions, sessionID, deviceInfo, ipAddress, refreshTokenExpiry)
}

func (jm *JWTManager) issueTokenPairWithPermissions(user models.User, permissions []string, sessionID, deviceInfo, ipAddress string, refreshTokenExpiry time.Time) (*models.TokenResponse, error) {
	cfg := config.LoadConfig()
	now := time.Now()

	accessTokenExpiry := now.Add(90 * time.Minute)
	accessClaims := &EnhancedClaims{
		UserID:      user.ID,
		Username:    user.Username,
		Email:       user.Email,
		Role:        user.Role,
		SessionID:   sessionID,
		DeviceInfo:  deviceInfo,
		IPAddress:   ipAddress,
		TokenType:   "access",
		Permissions: permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessTokenExpiry),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "accounting-system",
			Subject:   fmt.Sprintf("%d", user.ID),
			ID:        sessionID,
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(cfg.JWTAccessSecret))
	if err != nil {
		return nil, err
	}

	refreshClaims := &EnhancedClaims{
		UserID:     user.ID,
		Username:   user.Username,
		Email:      user.Email,
		Role:       user.Role,
		SessionID:  sessionID,
		DeviceInfo: deviceInfo,
		IPAddress:  ipAddress,
		TokenType:  "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(refreshTokenExpiry),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "accounting-system",
			Subject:   fmt.Sprintf("%d", user.ID),
			ID:        sessionID,
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(cfg.JWTRefreshSecret))
	if err != nil {
		return nil, err
	}

	refreshTokenRecord := models.RefreshToken{
		Token:      refreshTokenString,
		UserID:     user.ID,
		ExpiresAt:  refreshTokenExpiry,
		DeviceInfo: deviceInfo,
		IPAddress:  ipAddress,
	}

	if err := jm.DB.Create(&refreshTokenRecord).Error; err != nil {
		return nil, err
	}

	return &models.TokenResponse{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		TokenType:    "Bearer",
		ExpiresIn:    int64(90 * 60),
		ExpiresAt:    accessTokenExpiry,
		User:         user,
	}, nil
}

func sanitizeTokenMetadata(value, fallback string, maxLen int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = strings.TrimSpace(fallback)
	}
	if maxLen > 0 && len(value) > maxLen {
		value = value[:maxLen]
	}
	return value
}

// Enhanced JWT middleware with blacklist checking and session validation
func (jm *JWTManager) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Enhanced header debugging
		authHeader := c.GetHeader("Authorization")

		// Debug: Log all headers for troubleshooting
		if gin.Mode() == gin.DebugMode {
			fmt.Printf("🔍 [JWT DEBUG] Path: %s\n", c.Request.URL.Path)
			fmt.Printf("🔍 [JWT DEBUG] Method: %s\n", c.Request.Method)
			// Do not log raw Authorization headers; redact value if present
			if authHeader != "" {
				fmt.Printf("🔍 [JWT DEBUG] Authorization Header: [REDACTED]\n")
			}

			// Log only header names containing 'auth' (case insensitive) with redacted values
			for key := range c.Request.Header {
				if strings.Contains(strings.ToLower(key), "auth") {
					fmt.Printf("🔍 [JWT DEBUG] Header %s: [REDACTED]\n", key)
				}
			}
		}

		if authHeader == "" {
			// Enhanced error response with debugging info
			errorResponse := gin.H{
				"error":   "Authorization header required",
				"code":    "AUTH_HEADER_MISSING",
				"message": "Please include 'Authorization: Bearer <token>' header in your request",
			}

			// Add debug info in development mode
			if gin.Mode() == gin.DebugMode {
				errorResponse["debug"] = gin.H{
					"path":            c.Request.URL.Path,
					"method":          c.Request.Method,
					"user_agent":      c.GetHeader("User-Agent"),
					"content_type":    c.GetHeader("Content-Type"),
					"all_headers":     len(c.Request.Header),
					"expected_format": "Authorization: Bearer <your-jwt-token>",
				}
			}

			c.JSON(http.StatusUnauthorized, errorResponse)
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			// Enhanced format error response
			errorResponse := gin.H{
				"error":   "Invalid authorization header format",
				"code":    "INVALID_AUTH_FORMAT",
				"message": "Authorization header must start with 'Bearer ' followed by the token",
			}

			if gin.Mode() == gin.DebugMode {
				errorResponse["debug"] = gin.H{
					"received_header": authHeader,
					"expected_format": "Bearer <your-jwt-token>",
					"header_length":   len(authHeader),
				}
			}

			c.JSON(http.StatusUnauthorized, errorResponse)
			c.Abort()
			return
		}

		// Check if token is blacklisted
		if jm.isTokenBlacklisted(tokenString) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Token has been revoked",
				"code":  "TOKEN_BLACKLISTED",
			})
			c.Abort()
			return
		}

		cfg := config.LoadConfig()
		claims := &EnhancedClaims{}

		token, parsedClaims, err := parseEnhancedToken(tokenString, cfg.JWTAccessSecret)
		if err == nil && parsedClaims != nil {
			claims = parsedClaims
		}

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token",
				"code":  "INVALID_TOKEN",
			})
			c.Abort()
			return
		}

		if !token.Valid || claims.TokenType != "access" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid access token",
				"code":  "INVALID_ACCESS_TOKEN",
			})
			c.Abort()
			return
		}

		// Validate session - be more lenient with session validation
		var session models.UserSession
		if err := jm.DB.Where("session_id = ?", claims.SessionID).First(&session).Error; err != nil {
			// If session not found at all, it's invalid
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid session",
				"code":  "INVALID_SESSION",
			})
			c.Abort()
			return
		}

		// Check if session is expired first
		if session.ExpiresAt.Before(time.Now()) {
			// Mark session as inactive when expired
			jm.DB.Model(&session).Update("is_active", false)

			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Session expired",
				"code":  "SESSION_EXPIRED",
			})
			c.Abort()
			return
		}

		// Check if session is active - but don't fail if it's just marked inactive
		// This allows for more graceful handling of session state changes
		if !session.IsActive {
			// Only fail if the session was explicitly deactivated and is not expired
			// This prevents issues with cleanup services marking sessions as inactive
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Session deactivated",
				"code":  "SESSION_DEACTIVATED",
			})
			c.Abort()
			return
		}

		// Validate user is still active
		var user models.User
		if err := jm.DB.First(&user, claims.UserID).Error; err != nil || !user.IsActive {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User account is disabled",
				"code":  "ACCOUNT_DISABLED",
			})
			c.Abort()
			return
		}

		// Update session last activity
		jm.DB.Model(&session).Update("last_activity", time.Now())

		// Set user context
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
		c.Set("user_role", claims.Role) // Backward compatibility
		c.Set("session_id", claims.SessionID)
		c.Set("permissions", claims.Permissions)
		c.Set("user", user)

		c.Next()
	}
}

// BlacklistToken adds token to blacklist
func (jm *JWTManager) BlacklistToken(tokenString string, userID uint, reason string) error {
	// Parse token to get expiry
	cfg := config.LoadConfig()
	token, claims, err := parseEnhancedToken(tokenString, cfg.JWTAccessSecret)
	if err != nil {
		token, claims, err = parseEnhancedToken(tokenString, cfg.JWTRefreshSecret)
	}

	var expiresAt time.Time
	if err == nil && token != nil && claims != nil && claims.ExpiresAt != nil {
		expiresAt = time.Unix(claims.ExpiresAt.Unix(), 0)
	} else {
		// Default to 24 hours if we can't parse
		expiresAt = time.Now().Add(24 * time.Hour)
	}

	blacklistedToken := models.BlacklistedToken{
		Token:     tokenString,
		UserID:    userID,
		ExpiresAt: expiresAt,
		Reason:    reason,
	}

	return jm.DB.Create(&blacklistedToken).Error
}

// RevokeUserSessions revokes all active sessions for a user
func (jm *JWTManager) RevokeUserSessions(userID uint, reason string) error {
	// Deactivate all user sessions
	if err := jm.DB.Model(&models.UserSession{}).Where("user_id = ?", userID).Update("is_active", false).Error; err != nil {
		return err
	}

	// Revoke all refresh tokens
	if err := jm.DB.Model(&models.RefreshToken{}).Where("user_id = ?", userID).Update("is_revoked", true).Error; err != nil {
		return err
	}

	return nil
}

// Helper functions

func (jm *JWTManager) isTokenBlacklisted(tokenString string) bool {
	var count int64
	// Optimize query with index and limit
	jm.DB.Model(&models.BlacklistedToken{}).Where("token = ? AND expires_at > ?", tokenString, time.Now()).Limit(1).Count(&count)
	return count > 0
}

func (jm *JWTManager) getUserPermissions(role string) ([]string, error) {
	var rolePermissions []models.RolePermission
	if err := jm.DB.Preload("Permission").Where("role = ?", role).Find(&rolePermissions).Error; err != nil {
		return nil, err
	}

	permissions := make([]string, len(rolePermissions))
	for i, rp := range rolePermissions {
		permissions[i] = rp.Permission.Name
	}

	return permissions, nil
}

func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CleanupExpiredTokens removes expired tokens and sessions
func (jm *JWTManager) CleanupExpiredTokens() error {
	now := time.Now()

	// Remove expired blacklisted tokens
	if err := jm.DB.Where("expires_at < ?", now).Delete(&models.BlacklistedToken{}).Error; err != nil {
		return err
	}

	// Remove expired refresh tokens
	if err := jm.DB.Where("expires_at < ?", now).Delete(&models.RefreshToken{}).Error; err != nil {
		return err
	}

	// Remove expired sessions
	if err := jm.DB.Where("expires_at < ?", now).Delete(&models.UserSession{}).Error; err != nil {
		return err
	}

	return nil
}
