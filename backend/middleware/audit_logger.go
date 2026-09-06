package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AuditLog represents an audit log entry
type AuditLog struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	UserID       *uint     `json:"user_id" gorm:"index"` // Nullable for anonymous users
	Username     string    `json:"username" gorm:"size:100"`
	Action       string    `json:"action" gorm:"size:50"`       // CREATE, UPDATE, DELETE, VIEW, LOGIN, LOGOUT
	TableName    string    `json:"table_name" gorm:"size:100"`  // Table name for database compatibility
	Resource     string    `json:"resource" gorm:"size:50"`     // payment, user, product, etc.
	ResourceID   string    `json:"resource_id" gorm:"size:100"` // ID of the affected resource
	RecordID     uint      `json:"record_id" gorm:"index"`      // For database compatibility
	Method       string    `json:"method" gorm:"size:10"`       // HTTP method
	Endpoint     string    `json:"endpoint" gorm:"size:255"`    // API endpoint
	IPAddress    string    `json:"ip_address" gorm:"size:45"`
	UserAgent    string    `json:"user_agent" gorm:"type:text"`
	RequestData  string    `json:"request_data" gorm:"type:text"` // JSON string of request body
	OldValues    string    `json:"old_values" gorm:"type:text"`   // For database compatibility
	NewValues    string    `json:"new_values" gorm:"type:text"`   // For database compatibility
	ResponseCode int       `json:"response_code"`
	Duration     int64     `json:"duration"` // Response time in milliseconds
	Success      bool      `json:"success"`
	ErrorMessage string    `json:"error_message" gorm:"type:text"`
	Notes        string    `json:"notes" gorm:"type:text"` // For SQL trigger compatibility
	Timestamp    time.Time `json:"timestamp"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AuditLogger manages audit logging
type AuditLogger struct {
	db     *gorm.DB
	logger *log.Logger
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(db *gorm.DB) *AuditLogger {
	// Create logs directory if it doesn't exist
	logDir := "logs"
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		os.MkdirAll(logDir, 0755)
	}

	// Create audit log file
	logFile, err := os.OpenFile(
		filepath.Join(logDir, "audit.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0666,
	)
	if err != nil {
		log.Printf("Failed to create audit log file: %v", err)
		logFile = os.Stdout
	}

	logger := log.New(logFile, "[AUDIT] ", log.LstdFlags|log.Lmicroseconds)

	return &AuditLogger{
		db:     db,
		logger: logger,
	}
}

// responseWriter wraps gin.ResponseWriter to capture response data
type responseWriter struct {
	gin.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// AuditMiddleware returns a middleware that logs all API activities
func (al *AuditLogger) AuditMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// Skip health checks and static files
		if shouldSkipAudit(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Only audit financial transactions (sales, purchases, payments, cash_bank)
		if !isFinancialTransaction(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Multipart uploads must remain unread until their handler parses them.
		// Reading file bodies here both duplicates large data in memory and can
		// break multipart parsing in downstream handlers. Audit only metadata for
		// uploads; JSON/form payloads retain the existing sanitized audit trail.
		requestData := captureAuditRequestData(c.Request, sanitizeRequestData)

		// Wrap response writer to capture response
		respWriter := &responseWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBufferString(""),
			statusCode:     http.StatusOK,
		}
		c.Writer = respWriter

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(startTime)

		// Create audit log entry
		resource := getResourceFromPath(c.Request.URL.Path)
		resourceID := getResourceID(c.Request.URL.Path, c.Params)

		// Get user ID, set to nil if 0 (anonymous user)
		var userID *uint
		if uid := c.GetUint("user_id"); uid > 0 {
			userID = &uid
		}

		auditLog := &AuditLog{
			UserID:       userID,
			Username:     c.GetString("username"),
			Action:       getActionFromMethod(c.Request.Method),
			TableName:    getTableNameFromResource(resource),
			Resource:     resource,
			ResourceID:   resourceID,
			RecordID:     parseRecordID(resourceID),
			Method:       c.Request.Method,
			Endpoint:     c.Request.URL.Path,
			IPAddress:    c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			RequestData:  requestData,
			OldValues:    "", // Empty for API audit logs
			NewValues:    "", // Empty for API audit logs
			ResponseCode: respWriter.statusCode,
			Duration:     duration.Milliseconds(),
			Success:      respWriter.statusCode < 400,
			ErrorMessage: getErrorMessage(respWriter.body.String(), respWriter.statusCode),
			Timestamp:    startTime,
		}

		// Log to database (non-blocking)
		go al.logToDatabase(auditLog)

		// Log to file
		al.logToFile(auditLog)
	}
}

// PaymentAuditMiddleware specifically for payment operations
func (al *AuditLogger) PaymentAuditMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// Payment document uploads use multipart/form-data. Keep their stream
		// untouched so the document controller can validate and save the file;
		// only metadata is retained in the audit record.
		requestData := captureAuditRequestData(c.Request, sanitizePaymentData)

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(startTime)

		// Create detailed payment audit log
		resourceID := getResourceID(c.Request.URL.Path, c.Params)

		// Get user ID, set to nil if 0 (anonymous user)
		var userID *uint
		if uid := c.GetUint("user_id"); uid > 0 {
			userID = &uid
		}

		auditLog := &AuditLog{
			UserID:       userID,
			Username:     c.GetString("username"),
			Action:       getPaymentAction(c.Request.Method, c.Request.URL.Path),
			TableName:    "payments",
			Resource:     "payment",
			ResourceID:   resourceID,
			RecordID:     parseRecordID(resourceID),
			Method:       c.Request.Method,
			Endpoint:     c.Request.URL.Path,
			IPAddress:    c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			RequestData:  requestData,
			OldValues:    "", // Empty for API audit logs
			NewValues:    "", // Empty for API audit logs
			ResponseCode: c.Writer.Status(),
			Duration:     duration.Milliseconds(),
			Success:      c.Writer.Status() < 400,
			Timestamp:    startTime,
		}

		// Log to database and file
		go al.logToDatabase(auditLog)
		al.logToFile(auditLog)

		// Additional security logging for failed payment attempts
		if !auditLog.Success {
			al.logSecurityEvent(auditLog)
		}
	}
}

// logToDatabase saves audit log to database
func (al *AuditLogger) logToDatabase(auditLog *AuditLog) {
	if al.db != nil {
		if err := al.db.Create(auditLog).Error; err != nil {
			log.Printf("Failed to save audit log to database: %v", err)
		}
	}
}

// logToFile writes audit log to file in human-readable format
func (al *AuditLogger) logToFile(auditLog *AuditLog) {
	// Format timestamp: YY-MM-DD HH:MM:SS.mmm
	timestamp := auditLog.Timestamp.Format("06-01-02 15:04:05.000")

	// Determine status indicator
	statusIcon := "✓"
	if !auditLog.Success {
		statusIcon = "✗"
	}

	// Build resource identifier
	resourceInfo := strings.ToUpper(auditLog.Resource)
	if auditLog.ResourceID != "" {
		resourceInfo = fmt.Sprintf("%s#%s", resourceInfo, auditLog.ResourceID)
	}

	// Format user info - handle nil UserID for anonymous users
	userInfo := "anonymous"
	if auditLog.UserID != nil {
		userInfo = fmt.Sprintf("%s (ID:%d)", auditLog.Username, *auditLog.UserID)
	} else if auditLog.Username != "" {
		userInfo = auditLog.Username
	}

	// Format: [YY-MM-DD HH:MM:SS.mmm >> RESOURCE::ACTION] ✓ User: username (ID:123) | Method /endpoint | Status: 200 | 45ms | IP: 192.168.1.100
	logLine := fmt.Sprintf("[%s >> %s::%s] %s User: %s | %s %s | Status: %d | %dms | IP: %s",
		timestamp,
		resourceInfo,
		auditLog.Action,
		statusIcon,
		userInfo,
		auditLog.Method,
		auditLog.Endpoint,
		auditLog.ResponseCode,
		auditLog.Duration,
		auditLog.IPAddress,
	)

	// Add error message if present
	if !auditLog.Success && auditLog.ErrorMessage != "" {
		logLine += fmt.Sprintf("\n    ⚠️  Error: %s", auditLog.ErrorMessage)
	}

	// Add request data for CREATE/UPDATE operations (truncated)
	if (auditLog.Action == "CREATE" || auditLog.Action == "UPDATE") && auditLog.RequestData != "" && len(auditLog.RequestData) > 0 {
		requestData := auditLog.RequestData
		if len(requestData) > 300 {
			requestData = requestData[:300] + "..."
		}
		logLine += fmt.Sprintf("\n    📝 Data: %s", requestData)
	}

	logLine += "\n"

	// Write to file (remove [AUDIT] prefix since we already have nice format)
	al.logger.SetPrefix("")
	al.logger.SetFlags(0)
	al.logger.Printf("%s", logLine)
}

// logSecurityEvent logs security-related events
func (al *AuditLogger) logSecurityEvent(auditLog *AuditLog) {
	// Format user info - handle nil UserID for anonymous users
	userInfo := "anonymous"
	if auditLog.UserID != nil {
		userInfo = fmt.Sprintf("%s (ID:%d)", auditLog.Username, *auditLog.UserID)
	} else if auditLog.Username != "" {
		userInfo = auditLog.Username
	}

	securityEvent := fmt.Sprintf("SECURITY_ALERT: Failed %s attempt on %s by user %s from %s - Response: %d",
		auditLog.Action,
		auditLog.Resource,
		userInfo,
		auditLog.IPAddress,
		auditLog.ResponseCode,
	)

	al.logger.Printf("SECURITY: %s\n", securityEvent)
}

// captureAuditRequestData captures ordinary request bodies while preserving a
// multipart stream for the actual upload handler. File content is neither read
// nor logged. This is shared by global and payment-specific audit middleware
// because both run for /api/v1/payments/* routes.
func captureAuditRequestData(request *http.Request, sanitize func(string) string) string {
	if request == nil || request.Body == nil {
		return ""
	}
	if isMultipartAuditRequest(request) {
		metadata, _ := json.Marshal(map[string]interface{}{
			"file_upload":    true,
			"content_type":   request.Header.Get("Content-Type"),
			"content_length": request.ContentLength,
		})
		return string(metadata)
	}

	requestBody, _ := io.ReadAll(request.Body)
	request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
	return sanitize(string(requestBody))
}

func isMultipartAuditRequest(request *http.Request) bool {
	return strings.Contains(strings.ToLower(request.Header.Get("Content-Type")), "multipart/form-data")
}

// Helper functions
func shouldSkipAudit(path string) bool {
	skipPaths := []string{
		"/health",
		"/metrics",
		"/static",
		"/uploads",
		"/favicon.ico",
	}

	for _, skipPath := range skipPaths {
		if strings.HasPrefix(path, skipPath) {
			return true
		}
	}
	return false
}

// isFinancialTransaction checks if the path is a financial transaction that should be audited
func isFinancialTransaction(path string) bool {
	// Only audit these financial transaction endpoints
	financialResources := []string{
		"/api/v1/sales",
		"/api/v1/purchases",
		"/api/v1/payments",
		"/api/v1/payment",
		"/api/v1/cashbank",
		"/api/v1/cash-bank",
	}

	for _, resource := range financialResources {
		if strings.HasPrefix(path, resource) {
			return true
		}
	}

	return false
}

func getActionFromMethod(method string) string {
	switch method {
	case "GET":
		return "VIEW"
	case "POST":
		return "CREATE"
	case "PUT", "PATCH":
		return "UPDATE"
	case "DELETE":
		return "DELETE"
	default:
		return method
	}
}

func getPaymentAction(method, path string) string {
	if strings.Contains(path, "/cancel") {
		return "CANCEL_PAYMENT"
	}
	if strings.Contains(path, "/receivable") {
		return "CREATE_RECEIVABLE_PAYMENT"
	}
	if strings.Contains(path, "/payable") {
		return "CREATE_PAYABLE_PAYMENT"
	}
	if strings.Contains(path, "/unpaid-invoices") {
		return "VIEW_UNPAID_INVOICES"
	}
	if strings.Contains(path, "/unpaid-bills") {
		return "VIEW_UNPAID_BILLS"
	}
	if strings.Contains(path, "/summary") {
		return "VIEW_PAYMENT_SUMMARY"
	}

	return getActionFromMethod(method) + "_PAYMENT"
}

func getResourceFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 3 {
		return parts[2] // /api/v1/resource
	}
	if len(parts) >= 1 {
		return parts[0]
	}
	return "unknown"
}

func getResourceID(path string, params gin.Params) string {
	for _, param := range params {
		if param.Key == "id" {
			return param.Value
		}
	}
	return ""
}

// getTableNameFromResource maps resource names to table names
func getTableNameFromResource(resource string) string {
	switch resource {
	case "payment", "payments":
		return "payments"
	case "user", "users":
		return "users"
	case "product", "products":
		return "products"
	case "contact", "contacts":
		return "contacts"
	case "sale", "sales":
		return "sales"
	case "purchase", "purchases":
		return "purchases"
	case "account", "accounts":
		return "accounts"
	case "cashbank":
		return "cash_banks"
	default:
		return resource + "s" // Default pluralization
	}
}

// parseRecordID converts string ID to uint, returns 0 if invalid
func parseRecordID(resourceID string) uint {
	if resourceID == "" {
		return 0
	}

	// Try to parse as uint
	var id uint
	if n, err := fmt.Sscanf(resourceID, "%d", &id); n == 1 && err == nil {
		return id
	}

	return 0
}

func sanitizeRequestData(data string) string {
	if data == "" {
		return ""
	}

	// Parse JSON and remove sensitive fields
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &jsonData); err != nil {
		return "[invalid_json]"
	}

	// Remove sensitive fields
	sensitiveFields := []string{"password", "confirm_password", "current_password", "new_password", "token", "refresh_token"}
	for _, field := range sensitiveFields {
		if _, exists := jsonData[field]; exists {
			jsonData[field] = "[REDACTED]"
		}
	}

	sanitized, _ := json.Marshal(jsonData)
	return string(sanitized)
}

func sanitizePaymentData(data string) string {
	if data == "" {
		return ""
	}

	// For payment data, we might want to log more details but still sanitize sensitive info
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &jsonData); err != nil {
		return "[invalid_json]"
	}

	// Keep payment details but remove any sensitive account info
	sensitiveFields := []string{"account_number", "card_number", "cvv", "pin"}
	for _, field := range sensitiveFields {
		if _, exists := jsonData[field]; exists {
			jsonData[field] = "[REDACTED]"
		}
	}

	sanitized, _ := json.Marshal(jsonData)
	return string(sanitized)
}

func getErrorMessage(responseBody string, statusCode int) string {
	if statusCode >= 400 && responseBody != "" {
		// Try to extract error message from JSON response
		var response map[string]interface{}
		if err := json.Unmarshal([]byte(responseBody), &response); err == nil {
			if errorMsg, exists := response["error"]; exists {
				return fmt.Sprintf("%v", errorMsg)
			}
		}
		return fmt.Sprintf("HTTP %d", statusCode)
	}
	return ""
}

// GetAuditLogs retrieves audit logs with filters
func (al *AuditLogger) GetAuditLogs(userID *uint, resource string, limit int, offset int) ([]AuditLog, int64, error) {
	var logs []AuditLog
	var total int64

	query := al.db.Model(&AuditLog{})

	if userID != nil && *userID > 0 {
		query = query.Where("user_id = ?", *userID)
	}

	if resource != "" {
		query = query.Where("resource = ?", resource)
	}

	// Get total count
	query.Count(&total)

	// Get logs with pagination
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error

	return logs, total, err
}

// Global audit logger instance
var GlobalAuditLogger *AuditLogger

// InitAuditLogger initializes the global audit logger
func InitAuditLogger(db *gorm.DB) {
	GlobalAuditLogger = NewAuditLogger(db)

	// Safe auto-migrate audit log table with conflict handling
	if err := safeAuditLogMigration(db); err != nil {
		log.Printf("Failed to migrate audit log table: %v", err)
	}
}

// safeAuditLogMigration performs safe migration for audit log table
func safeAuditLogMigration(db *gorm.DB) error {
	// Check if audit_logs table exists
	var tableExists bool
	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.tables 
		WHERE table_name = 'audit_logs'
	)`).Scan(&tableExists)

	if !tableExists {
		// Table doesn't exist, safe to create
		log.Println("Creating audit_logs table...")
		return db.AutoMigrate(&AuditLog{})
	}

	// Table exists, check if we need to add any missing columns
	log.Println("Audit logs table exists, checking for missing columns...")

	// Check for missing columns and add them if needed
	if err := addMissingAuditColumns(db); err != nil {
		return fmt.Errorf("failed to add missing audit columns: %w", err)
	}

	log.Println("✅ Audit logs table migration completed safely")
	return nil
}

// addMissingAuditColumns adds any missing columns to audit_logs table
func addMissingAuditColumns(db *gorm.DB) error {
	// List of columns that should exist in audit_logs table
	expectedColumns := map[string]string{
		"username":      "VARCHAR(100)",
		"resource":      "VARCHAR(50)",
		"resource_id":   "VARCHAR(100)",
		"method":        "VARCHAR(10)",
		"endpoint":      "VARCHAR(255)",
		"request_data":  "TEXT",
		"response_code": "INTEGER",
		"duration":      "BIGINT",
		"success":       "BOOLEAN",
		"error_message": "TEXT",
		"timestamp":     "TIMESTAMP",
	}

	for columnName, columnType := range expectedColumns {
		var columnExists bool
		db.Raw(`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'audit_logs' AND column_name = ?
		)`, columnName).Scan(&columnExists)

		if !columnExists {
			log.Printf("Adding missing column %s to audit_logs table...", columnName)
			err := db.Exec(fmt.Sprintf("ALTER TABLE audit_logs ADD COLUMN %s %s", columnName, columnType)).Error
			if err != nil {
				log.Printf("Warning: Failed to add column %s: %v", columnName, err)
			} else {
				log.Printf("✅ Added column %s to audit_logs table", columnName)
			}
		}
	}

	return nil
}
