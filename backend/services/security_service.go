package services

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"app-sistem-akuntansi/models"
	"gorm.io/gorm"
)

var suspiciousPathPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)<\s*/?\s*script\b`),
	regexp.MustCompile(`(?i)javascript:`),
	regexp.MustCompile(`(?i)vbscript:`),
	regexp.MustCompile(`(?i)\.\./|\.\.\\`),
	regexp.MustCompile(`(?i)/etc/passwd|\\windows\\`),
	regexp.MustCompile(`(?i)(?:^|[^a-z0-9_])union(?:\s|%20)+select(?:[^a-z0-9_]|$)`),
	regexp.MustCompile(`(?i)(?:^|[^a-z0-9_])select(?:\s|%20)+.+(?:\s|%20)+from(?:[^a-z0-9_]|$)`),
	regexp.MustCompile(`(?i)(?:^|[^a-z0-9_])(?:or|and)(?:\s|%20)+['"]?\d+['"]?\s*=\s*['"]?\d+['"]?(?:[^a-z0-9_]|$)`),
	regexp.MustCompile(`(?i)\bexec(?:ute)?\s*\(`),
}

var suspiciousHeaderValuePattern = regexp.MustCompile(
	`(?i)(<\s*/?\s*script\b|javascript:|vbscript:|\.\./|\.\.\\|/etc/passwd|\\windows\\|(?:^|[^a-z0-9_])union(?:\s|%20)+select(?:[^a-z0-9_]|$)|(?:^|[^a-z0-9_])select(?:\s|%20)+.+(?:\s|%20)+from(?:[^a-z0-9_]|$)|(?:^|[^a-z0-9_])(?:or|and)(?:\s|%20)+['"]?\d+['"]?\s*=\s*['"]?\d+['"]?(?:[^a-z0-9_]|$)|\bexec(?:ute)?\s*\()`,
)

var securityInspectableHeaders = map[string]struct{}{
	"origin":          {},
	"referer":         {},
	"user-agent":      {},
	"x-forwarded-for": {},
	"x-real-ip":       {},
	"x-original-url":  {},
	"x-rewrite-url":   {},
}

// SecurityService handles security incident tracking and monitoring
type SecurityService struct {
	db *gorm.DB
}

// NewSecurityService creates a new security service
func NewSecurityService(db *gorm.DB) *SecurityService {
	return &SecurityService{db: db}
}

// LogSecurityIncident logs a security incident
func (s *SecurityService) LogSecurityIncident(
	incidentType, severity, description, clientIP, userAgent, method, path, headers string,
	userID *uint, sessionID string,
) error {
	incident := &models.SecurityIncident{
		IncidentType:   incidentType,
		Severity:       severity,
		Description:    description,
		ClientIP:       clientIP,
		UserAgent:      userAgent,
		RequestMethod:  method,
		RequestPath:    path,
		RequestHeaders: headers,
		UserID:         userID,
		SessionID:      sessionID,
		Resolved:       false,
	}

	if err := s.db.Create(incident).Error; err != nil {
		log.Printf("Failed to log security incident: %v", err)
		return err
	}

	// Create system alert for high/critical incidents
	if severity == models.SeverityCritical || severity == models.SeverityHigh {
		s.CreateAlert(
			models.AlertTypeSecurityBreach,
			models.AlertLevelCritical,
			fmt.Sprintf("Security Incident: %s", incidentType),
			fmt.Sprintf("Security incident detected from IP %s: %s", clientIP, description),
		)
	}

	// Log to file for external monitoring
	s.logToFile(fmt.Sprintf("[SECURITY_INCIDENT] Type: %s, Severity: %s, IP: %s, Path: %s, Description: %s",
		incidentType, severity, clientIP, path, description))

	return nil
}

// LogSuspiciousRequest logs a suspicious request
func (s *SecurityService) LogSuspiciousRequest(
	method, path, clientIP, userAgent string, statusCode int, responseTime int64,
	userID *uint, sessionID, suspiciousReason string,
) error {
	requestLog := &models.RequestLog{
		Method:           method,
		Path:             path,
		ClientIP:         clientIP,
		UserAgent:        userAgent,
		StatusCode:       statusCode,
		ResponseTime:     responseTime,
		UserID:           userID,
		SessionID:        sessionID,
		IsSuspicious:     true,
		SuspiciousReason: suspiciousReason,
		Timestamp:        time.Now(),
	}

	if err := s.db.Create(requestLog).Error; err != nil {
		log.Printf("Failed to log suspicious request: %v", err)
		return err
	}

	// Also log as security incident if highly suspicious
	if s.isHighlySuspicious(suspiciousReason) {
		return s.LogSecurityIncident(
			models.IncidentTypeSuspiciousRequest,
			models.SeverityMedium,
			fmt.Sprintf("Suspicious request detected: %s", suspiciousReason),
			clientIP,
			userAgent,
			method,
			path,
			"",
			userID,
			sessionID,
		)
	}

	return nil
}

// LogRequest logs a normal request for monitoring
func (s *SecurityService) LogRequest(
	method, path, clientIP, userAgent string, statusCode int,
	responseTime, requestSize, responseSize int64, userID *uint, sessionID string,
) error {
	// Only log in development or if detailed logging is enabled
	if os.Getenv("DETAILED_REQUEST_LOGGING") != "true" && os.Getenv("APP_ENV") != "development" {
		return nil
	}

	requestLog := &models.RequestLog{
		Method:       method,
		Path:         path,
		ClientIP:     clientIP,
		UserAgent:    userAgent,
		StatusCode:   statusCode,
		ResponseTime: responseTime,
		RequestSize:  requestSize,
		ResponseSize: responseSize,
		UserID:       userID,
		SessionID:    sessionID,
		IsSuspicious: false,
		Timestamp:    time.Now(),
	}

	return s.db.Create(requestLog).Error
}

// CreateAlert creates a system alert
func (s *SecurityService) CreateAlert(alertType, level, title, message string) error {
	// Check if similar alert exists in last hour
	var existingAlert models.SystemAlert
	oneHourAgo := time.Now().Add(-time.Hour)

	result := s.db.Where(
		"alert_type = ? AND level = ? AND title = ? AND created_at > ? AND acknowledged = false",
		alertType, level, title, oneHourAgo,
	).First(&existingAlert)

	if result.Error == nil {
		// Update existing alert
		existingAlert.Count++
		existingAlert.LastSeen = time.Now()
		existingAlert.Message = message
		return s.db.Save(&existingAlert).Error
	}

	// Create new alert
	alert := &models.SystemAlert{
		AlertType:    alertType,
		Level:        level,
		Title:        title,
		Message:      message,
		Count:        1,
		FirstSeen:    time.Now(),
		LastSeen:     time.Now(),
		Acknowledged: false,
	}

	if err := s.db.Create(alert).Error; err != nil {
		log.Printf("Failed to create alert: %v", err)
		return err
	}

	// Log critical alerts to file
	if level == models.AlertLevelCritical || level == models.AlertLevelError {
		s.logToFile(fmt.Sprintf("[ALERT_%s] %s: %s", strings.ToUpper(level), title, message))
	}

	return nil
}

// IsIPWhitelisted checks if IP is whitelisted for the current environment
func (s *SecurityService) IsIPWhitelisted(clientIP string) bool {
	environment := os.Getenv("APP_ENV")
	if environment == "" {
		environment = "production"
	}

	var whitelist models.IpWhitelist
	result := s.db.Where(
		"(ip_address = ? OR ip_range LIKE ?) AND environment IN (?, 'all') AND is_active = true AND (expires_at IS NULL OR expires_at > ?)",
		clientIP, clientIP+"%", environment, time.Now(),
	).First(&whitelist)

	return result.Error == nil
}

// GetSecurityMetrics gets daily security metrics
func (s *SecurityService) GetSecurityMetrics(date time.Time) (*models.SecurityMetrics, error) {
	var metrics models.SecurityMetrics

	// Try to get existing metrics for the date
	result := s.db.Where("date = ?", date.Format("2006-01-02")).First(&metrics)
	if result.Error == nil {
		return &metrics, nil
	}

	// Calculate metrics for the date
	dateStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dateEnd := dateStart.Add(24 * time.Hour)

	// Count total requests
	var totalRequests int64
	s.db.Model(&models.RequestLog{}).Where("timestamp BETWEEN ? AND ?", dateStart, dateEnd).Count(&totalRequests)

	// Count suspicious requests
	var suspiciousRequests int64
	s.db.Model(&models.RequestLog{}).Where("timestamp BETWEEN ? AND ? AND is_suspicious = true", dateStart, dateEnd).Count(&suspiciousRequests)

	// Count security incidents
	var securityIncidents int64
	s.db.Model(&models.SecurityIncident{}).Where("created_at BETWEEN ? AND ?", dateStart, dateEnd).Count(&securityIncidents)

	// Calculate average response time
	var avgResponseTime float64
	s.db.Model(&models.RequestLog{}).Where("timestamp BETWEEN ? AND ?", dateStart, dateEnd).
		Select("AVG(response_time)").Scan(&avgResponseTime)

	// Create new metrics record
	metrics = models.SecurityMetrics{
		Date:                   dateStart,
		TotalRequests:          totalRequests,
		SuspiciousRequestCount: suspiciousRequests,
		SecurityIncidentCount:  securityIncidents,
		AvgResponseTime:        avgResponseTime,
	}

	if err := s.db.Create(&metrics).Error; err != nil {
		return nil, err
	}

	return &metrics, nil
}

// CleanupOldLogs cleans up old security logs and metrics
func (s *SecurityService) CleanupOldLogs() error {
	// Keep logs for 90 days in production, 30 days in development
	retentionDays := 90
	if os.Getenv("APP_ENV") == "development" {
		retentionDays = 30
	}

	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	// Delete old request logs
	if err := s.db.Where("created_at < ?", cutoffDate).Delete(&models.RequestLog{}).Error; err != nil {
		log.Printf("Failed to cleanup old request logs: %v", err)
	}

	// Delete old resolved security incidents
	if err := s.db.Where("created_at < ? AND resolved = true", cutoffDate).Delete(&models.SecurityIncident{}).Error; err != nil {
		log.Printf("Failed to cleanup old security incidents: %v", err)
	}

	// Delete old acknowledged alerts
	if err := s.db.Where("created_at < ? AND acknowledged = true", cutoffDate).Delete(&models.SystemAlert{}).Error; err != nil {
		log.Printf("Failed to cleanup old alerts: %v", err)
	}

	// Keep security metrics for 1 year
	metricsRetentionDate := time.Now().AddDate(-1, 0, 0)
	if err := s.db.Where("date < ?", metricsRetentionDate).Delete(&models.SecurityMetrics{}).Error; err != nil {
		log.Printf("Failed to cleanup old security metrics: %v", err)
	}

	return nil
}

// Helper methods

func (s *SecurityService) isHighlySuspicious(reason string) bool {
	highRiskReasons := []string{
		"SQL_INJECTION",
		"XSS_ATTEMPT",
		"DIRECTORY_TRAVERSAL",
		"MULTIPLE_AUTH_FAILURES",
		"UNAUTHORIZED_ADMIN_ACCESS",
	}

	for _, risk := range highRiskReasons {
		if strings.Contains(reason, risk) {
			return true
		}
	}
	return false
}

func (s *SecurityService) logToFile(message string) {
	logDir := os.Getenv("SECURITY_LOG_DIR")
	if logDir == "" {
		logDir = "./logs"
	}

	// Create logs directory if not exists
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("Failed to create log directory: %v", err)
		return
	}

	filename := fmt.Sprintf("%s/security_%s.log", logDir, time.Now().Format("2006-01-02"))

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Failed to open security log file: %v", err)
		return
	}
	defer file.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logMessage := fmt.Sprintf("[%s] %s\n", timestamp, message)

	if _, err := file.WriteString(logMessage); err != nil {
		log.Printf("Failed to write to security log file: %v", err)
	}
}

// GetSecurityConfig gets security configuration value
func (s *SecurityService) GetSecurityConfig(key string) (string, error) {
	environment := os.Getenv("APP_ENV")
	if environment == "" {
		environment = "production"
	}

	var config models.SecurityConfig
	result := s.db.Where(
		"key = ? AND environment IN (?, 'all')",
		key, environment,
	).First(&config)

	if result.Error != nil {
		return "", result.Error
	}

	return config.Value, nil
}

// SetSecurityConfig sets security configuration value
func (s *SecurityService) SetSecurityConfig(key, value, dataType, environment, description string, userID uint) error {
	var config models.SecurityConfig
	result := s.db.Where("key = ? AND environment = ?", key, environment).First(&config)

	if result.Error == gorm.ErrRecordNotFound {
		// Create new config
		config = models.SecurityConfig{
			Key:            key,
			Value:          value,
			DataType:       dataType,
			Environment:    environment,
			Description:    description,
			LastModifiedBy: userID,
		}
		return s.db.Create(&config).Error
	}

	// Update existing config
	config.Value = value
	config.DataType = dataType
	config.Description = description
	config.LastModifiedBy = userID

	return s.db.Save(&config).Error
}

// DetectSuspiciousPattern detects suspicious patterns in requests
func (s *SecurityService) DetectSuspiciousPattern(method, path, userAgent, clientIP string, headers map[string]string) (bool, string) {
	pathLower := strings.ToLower(path)
	userAgentLower := strings.ToLower(userAgent)

	for _, pattern := range suspiciousPathPatterns {
		if pattern.MatchString(pathLower) {
			return true, fmt.Sprintf("SUSPICIOUS_PATH_PATTERN: %s", pattern.String())
		}
	}

	// Check headers for suspicious content on user-controlled headers only.
	for key, value := range headers {
		if _, ok := securityInspectableHeaders[strings.ToLower(key)]; !ok {
			continue
		}

		valueLower := strings.ToLower(value)
		if suspiciousHeaderValuePattern.MatchString(valueLower) {
			return true, fmt.Sprintf("SUSPICIOUS_HEADER_%s", strings.ToUpper(key))
		}

		// User-Agent keeps a dedicated check below with explicit tool names.
		if strings.EqualFold(key, "User-Agent") {
			continue
		}
	}

	// Directory traversal
	if strings.Contains(path, "../") || strings.Contains(path, "..\\") {
		return true, "DIRECTORY_TRAVERSAL_ATTEMPT"
	}

	// Suspicious user agents
	suspiciousUAPatterns := []string{
		"sqlmap", "nikto", "burp", "nessus", "openvas", "w3af", "havij",
		"bsqlbf", "pangolin", "nmap", "masscan",
	}

	for _, pattern := range suspiciousUAPatterns {
		if strings.Contains(userAgentLower, pattern) {
			return true, fmt.Sprintf("SUSPICIOUS_USER_AGENT: %s", pattern)
		}
	}

	return false, ""
}
