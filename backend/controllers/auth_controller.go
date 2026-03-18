package controllers

import (
	"app-sistem-akuntansi/middleware"
	"app-sistem-akuntansi/models"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthController struct {
	DB *gorm.DB
}

func NewAuthController(db *gorm.DB) *AuthController {
	return &AuthController{DB: db}
}

func requestDeviceInfo(c *gin.Context) string {
	userAgent := strings.TrimSpace(c.Request.UserAgent())
	if userAgent == "" {
		return "Web Browser"
	}
	if len(userAgent) > 500 {
		return userAgent[:500]
	}
	return userAgent
}

// Helper function to convert role to uppercase for frontend
func convertRoleToUppercase(role string) string {
	switch role {
	case "admin":
		return "ADMIN"
	case "finance":
		return "FINANCE"
	case "director":
		return "DIRECTOR"
	case "inventory_manager":
		return "INVENTORY_MANAGER"
	case "employee":
		return "EMPLOYEE"
	default:
		return strings.ToUpper(role)
	}
}

// Helper function to convert role to lowercase for backend
func convertRoleToLowercase(role string) string {
	switch role {
	case "ADMIN":
		return "admin"
	case "FINANCE":
		return "finance"
	case "DIRECTOR":
		return "director"
	case "INVENTORY_MANAGER":
		return "inventory_manager"
	case "EMPLOYEE":
		return "employee"
	default:
		return strings.ToLower(role)
	}
}

// Register creates a new user account
// @Summary User registration
// @Description Create a new user account (only available in development or when ALLOW_REGISTRATION=true)
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body models.RegisterRequest true "User registration data"
// @Success 201 {object} models.LoginResponse "User created successfully"
// @Failure 400 {object} models.ErrorResponse "Invalid request data"
// @Failure 409 {object} models.ErrorResponse "User already exists"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /auth/register [post]
func (ac *AuthController) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user already exists
	var existingUser models.User
	if err := ac.DB.Where("username = ? OR email = ?", req.Username, req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	assignedRole := "employee"
	if req.Role != "" && convertRoleToLowercase(req.Role) != assignedRole {
		c.JSON(http.StatusForbidden, gin.H{"error": "Public registration cannot assign privileged roles"})
		return
	}

	// Create user
	user := models.User{
		Username:  req.Username,
		Email:     req.Email,
		Password:  string(hashedPassword),
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Role:      assignedRole,
		IsActive:  true,
	}

	if err := ac.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Initialize JWT Manager
	jw := middleware.NewJWTManager(ac.DB)

	// Generate token pair
	tokens, err := jw.GenerateTokenPair(user, "Web Browser", c.ClientIP())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":       true,
		"message":       "User registered successfully",
		"access_token":  tokens.AccessToken,
		"token":         tokens.AccessToken, // Compatibility field
		"refresh_token": tokens.RefreshToken,
		"refreshToken":  tokens.RefreshToken, // Compatibility field
		"user":          tokens.User,
	})
}

// Login authenticates user with email/username and password
// @Summary User login
// @Description Authenticate user and return access token and refresh token
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body models.LoginRequest true "Login credentials"
// @Success 200 {object} models.LoginResponse "Successful login"
// @Failure 400 {object} models.ErrorResponse "Invalid request format"
// @Failure 401 {object} models.ErrorResponse "Invalid credentials or account disabled"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /auth/login [post]
func (ac *AuthController) Login(c *gin.Context) {
	// Use standard LoginRequest model
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Debug logging
	log.Printf("Login attempt - Email: %s, Password length: %d", req.Email, len(req.Password))

	// Use email as identifier
	identifier := req.Email
	password := req.Password
	deviceInfo := requestDeviceInfo(c)

	// Find user by email or username
	var user models.User
	if err := ac.DB.Where("username = ? OR email = ?", identifier, identifier).First(&user).Error; err != nil {
		ac.logAuthAttempt(identifier, false, models.FailureReasonInvalidCredentials, c.ClientIP(), c.Request.UserAgent())
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Check if user is active
	if !user.IsActive {
		ac.logAuthAttempt(identifier, false, models.FailureReasonAccountDisabled, c.ClientIP(), c.Request.UserAgent())
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Account is deactivated"})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		ac.logAuthAttempt(identifier, false, models.FailureReasonInvalidCredentials, c.ClientIP(), c.Request.UserAgent())
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Enforce a single active login per user by revoking older sessions first.
	// Existing devices will receive SESSION_DEACTIVATED on their next request,
	// which the frontend already maps to the session-expired flow.
	var tokens *models.TokenResponse
	err := ac.DB.Transaction(func(tx *gorm.DB) error {
		txJWT := middleware.NewJWTManager(tx)
		if err := txJWT.RevokeUserSessions(user.ID, "NEW_LOGIN"); err != nil {
			return err
		}

		generatedTokens, err := txJWT.GenerateTokenPair(user, deviceInfo, c.ClientIP())
		if err != nil {
			return err
		}

		tokens = generatedTokens
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	ac.logAuthAttempt(identifier, true, "", c.ClientIP(), c.Request.UserAgent())

	// Fetch settings to get company_type
	var settings models.Settings
	if err := ac.DB.First(&settings).Error; err != nil {
		log.Printf("Warning: Failed to fetch settings during login: %v", err)
	}

	// Return response in format expected by frontend
	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"access_token":  tokens.AccessToken,
		"token":         tokens.AccessToken, // Compatibility field
		"refresh_token": tokens.RefreshToken,
		"refreshToken":  tokens.RefreshToken, // Compatibility field
		"user":          tokens.User,
		"company_type":  settings.CompanyType,
		"message":       "Login successful",
	})
}

// Log authentication attempts
func (ac *AuthController) logAuthAttempt(identifier string, success bool, reason, ipAddress, userAgent string) {
	authAttempt := models.AuthAttempt{
		Email:         identifier,
		Username:      identifier,
		Success:       success,
		FailureReason: reason,
		IPAddress:     ipAddress,
		UserAgent:     userAgent,
		AttemptedAt:   time.Now(),
	}

	ac.DB.Create(&authAttempt)
}

// RefreshToken generates new access token from refresh token
// @Summary Refresh access token
// @Description Generate a new access token using valid refresh token
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body models.RefreshTokenRequest true "Refresh token"
// @Success 200 {object} models.LoginResponse "Token refreshed successfully"
// @Failure 400 {object} models.ErrorResponse "Invalid request format"
// @Failure 401 {object} models.ErrorResponse "Invalid or expired refresh token"
// @Router /auth/refresh [post]
func (ac *AuthController) RefreshToken(c *gin.Context) {
	var req models.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	jw := middleware.NewJWTManager(ac.DB)
	tokens, err := jw.RefreshAccessToken(req.RefreshToken, requestDeviceInfo(c), c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
		return
	}

	// Fetch settings to get company_type
	var settings models.Settings
	if err := ac.DB.First(&settings).Error; err != nil {
		log.Printf("Warning: Failed to fetch settings during refresh: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"access_token":  tokens.AccessToken,
		"token":         tokens.AccessToken, // Compatibility field
		"refresh_token": tokens.RefreshToken,
		"refreshToken":  tokens.RefreshToken, // Compatibility field
		"user":          tokens.User,
		"company_type":  settings.CompanyType,
		"message":       "Token refreshed successfully",
	})
}

// Profile gets current user profile information
// @Summary Get user profile
// @Description Retrieve current authenticated user's profile information
// @Tags Authentication
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.APIResponse{data=models.UserResponse} "Profile retrieved successfully"
// @Failure 401 {object} models.ErrorResponse "Unauthorized - invalid token"
// @Failure 404 {object} models.ErrorResponse "User not found"
// @Router /profile [get]
func (ac *AuthController) Profile(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var user models.User
	if err := ac.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Fetch settings to get company_type
	var settings models.Settings
	if err := ac.DB.First(&settings).Error; err != nil {
		log.Printf("Warning: Failed to fetch settings during profile fetch: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Profile retrieved successfully",
		"data":         user,
		"company_type": settings.CompanyType,
	})
}

// ValidateToken validates if the current token is valid and active
// @Summary Validate JWT token
// @Description Check if the current JWT token is valid and user account is active
// @Tags Authentication
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.APIResponse{data=models.UserResponse} "Token is valid"
// @Failure 401 {object} models.ErrorResponse "Invalid token or account disabled"
// @Router /auth/validate-token [get]
func (ac *AuthController) ValidateToken(c *gin.Context) {
	// If we reach this point, it means the JWT middleware has already validated the token
	// and set the user context, so the token is valid
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid token - user ID not found",
			"code":  "INVALID_TOKEN",
			"valid": false,
		})
		return
	}

	// Double-check that the user still exists and is active
	var user models.User
	if err := ac.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not found",
			"code":  "USER_NOT_FOUND",
			"valid": false,
		})
		return
	}

	// Check if user is still active
	if !user.IsActive {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User account is disabled",
			"code":  "ACCOUNT_DISABLED",
			"valid": false,
		})
		return
	}

	// Fetch settings to get company_type
	var settings models.Settings
	if err := ac.DB.First(&settings).Error; err != nil {
		log.Printf("Warning: Failed to fetch settings during token validation: %v", err)
	}

	// Token is valid
	c.JSON(http.StatusOK, gin.H{
		"message":      "Token is valid",
		"valid":        true,
		"company_type": settings.CompanyType,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"role":     user.Role,
		},
	})
}

// GetSessionInfo returns current session information for debugging
// @Summary Get session information
// @Description Get current session information for debugging purposes
// @Tags Authentication
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{} "Session information"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Router /auth/session-info [get]
func (ac *AuthController) GetSessionInfo(c *gin.Context) {
	userID, _ := c.Get("user_id")
	sessionID, _ := c.Get("session_id")

	// Get session details
	var session models.UserSession
	if err := ac.DB.Where("session_id = ?", sessionID).First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Session not found",
			"code":  "SESSION_NOT_FOUND",
		})
		return
	}

	// Get user details
	var user models.User
	if err := ac.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
			"code":  "USER_NOT_FOUND",
		})
		return
	}

	// Fetch settings
	var settings models.Settings
	ac.DB.First(&settings)

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"company_type": settings.CompanyType,
		"session": gin.H{
			"session_id":    session.SessionID,
			"is_active":     session.IsActive,
			"created_at":    session.CreatedAt,
			"expires_at":    session.ExpiresAt,
			"last_activity": session.LastActivity,
			"ip_address":    session.IPAddress,
			"device_info":   session.DeviceInfo,
		},
		"user": gin.H{
			"id":        user.ID,
			"username":  user.Username,
			"email":     user.Email,
			"role":      user.Role,
			"is_active": user.IsActive,
		},
		"timestamp": time.Now(),
	})
}
