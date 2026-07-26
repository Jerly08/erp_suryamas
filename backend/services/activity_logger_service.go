package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"app-sistem-akuntansi/models"
	"gorm.io/gorm"
)

// ActivityLoggerService handles logging of user activities to both file and database
type ActivityLoggerService struct {
	db             *gorm.DB
	logDir         string
	currentLogFile *os.File
	fileMutex      sync.Mutex
	maxFileSize    int64 // Maximum log file size in bytes (default: 10MB)
	maxFilesPerDay int   // Maximum number of log files per day
	enableDB       bool  // Enable database logging
	enableFile     bool  // Enable file logging
}

// NewActivityLoggerService creates a new activity logger service
func NewActivityLoggerService(db *gorm.DB, logDir string) *ActivityLoggerService {
	service := &ActivityLoggerService{
		db:             db,
		logDir:         logDir,
		maxFileSize:    10 * 1024 * 1024, // 10MB default
		maxFilesPerDay: 5,
		enableDB:       true,
		enableFile:     false,
	}

	if strings.TrimSpace(logDir) == "" {
		fmt.Println("INFO: Activity file logging disabled because no log directory is configured; database logging remains enabled")
		return service
	}

	// Production runs with a read-only root filesystem. Keep the durable activity
	// record in the database and only enable the optional file mirror when its
	// directory can actually be created and written to.
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("WARNING: Activity file logging disabled because log directory is unavailable: %v; database logging remains enabled\n", err)
		return service
	}

	probe, err := os.CreateTemp(logDir, ".activity-log-write-check-*")
	if err != nil {
		fmt.Printf("WARNING: Activity file logging disabled because log directory is not writable: %v; database logging remains enabled\n", err)
		return service
	}

	probePath := probe.Name()
	_ = probe.Close()
	_ = os.Remove(probePath)
	service.enableFile = true

	return service
}

// LogActivity logs a user activity to both file and database
func (s *ActivityLoggerService) LogActivity(log *models.ActivityLog) error {
	var errs []error

	// Log to database
	if s.enableDB {
		if err := s.logToDatabase(log); err != nil {
			errs = append(errs, fmt.Errorf("database logging failed: %v", err))
		}
	}

	// Log to file
	if s.enableFile {
		if err := s.logToFile(log); err != nil {
			errs = append(errs, fmt.Errorf("file logging failed: %v", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("logging errors: %v", errs)
	}

	return nil
}

// logToDatabase saves the activity log to the database
func (s *ActivityLoggerService) logToDatabase(log *models.ActivityLog) error {
	if s.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Set created time if not set
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}

	// Create log entry in database
	if err := s.db.Create(log).Error; err != nil {
		return fmt.Errorf("failed to save to database: %v", err)
	}

	return nil
}

// logToFile writes the activity log to a file in human-readable format
func (s *ActivityLoggerService) logToFile(log *models.ActivityLog) error {
	s.fileMutex.Lock()
	defer s.fileMutex.Unlock()

	// Get or create log file for today
	logFile, err := s.getOrCreateLogFile()
	if err != nil {
		return fmt.Errorf("failed to get log file: %v", err)
	}

	// Format timestamp: YY-MM-DD HH:MM:SS.mmm
	timestamp := log.CreatedAt.Format("06-01-02 15:04:05.000")

	// Determine status indicator
	statusIcon := "✓"
	if log.IsError {
		statusIcon = "✗"
	}

	// Format: [YY-MM-DD HH:MM:SS.mmm >> MODULE::ACTION] User: username | Method: GET /path | Status: 200 | Duration: 123ms | IP: 127.0.0.1
	logLine := fmt.Sprintf("[%s >> %s::%s] %s User: %s (%s) | %s %s | Status: %d | %dms | IP: %s",
		timestamp,
		strings.ToUpper(log.Resource),
		strings.ToUpper(log.Action),
		statusIcon,
		log.Username,
		log.Role,
		log.Method,
		log.Path,
		log.StatusCode,
		log.Duration,
		log.IPAddress,
	)

	// Add query params if present
	if log.QueryParams != "" {
		logLine += fmt.Sprintf(" | Query: %s", log.QueryParams)
	}

	// Add error message if present
	if log.IsError && log.ErrorMessage != "" {
		logLine += fmt.Sprintf("\n    ⚠️  Error: %s", log.ErrorMessage)
	}

	// Add request body for POST/PUT/PATCH (truncated)
	if log.RequestBody != "" && len(log.RequestBody) > 0 && len(log.RequestBody) < 500 {
		// Clean up request body
		requestBody := strings.ReplaceAll(log.RequestBody, "\n", "")
		requestBody = strings.ReplaceAll(requestBody, "\r", "")
		if len(requestBody) > 200 {
			requestBody = requestBody[:200] + "..."
		}
		logLine += fmt.Sprintf("\n    📥 Request: %s", requestBody)
	}

	logLine += "\n"

	// Write to file
	if _, err := logFile.Write([]byte(logLine)); err != nil {
		return fmt.Errorf("failed to write to log file: %v", err)
	}

	// Check if rotation is needed
	s.rotateLogFileIfNeeded(logFile)

	return nil
}

// getOrCreateLogFile returns the current log file or creates a new one
func (s *ActivityLoggerService) getOrCreateLogFile() (*os.File, error) {
	// Generate log file name based on current date
	today := time.Now().Format("2006-01-02")
	logFileName := fmt.Sprintf("activity_%s.log", today)
	logFilePath := filepath.Join(s.logDir, logFileName)

	// Check if we need to create a new file
	if s.currentLogFile == nil || s.getCurrentLogFileName() != logFileName {
		// Close old file if open
		if s.currentLogFile != nil {
			s.currentLogFile.Close()
		}

		// Open or create new log file
		file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %v", err)
		}

		s.currentLogFile = file
	}

	return s.currentLogFile, nil
}

// getCurrentLogFileName returns the current log file name
func (s *ActivityLoggerService) getCurrentLogFileName() string {
	if s.currentLogFile == nil {
		return ""
	}
	return filepath.Base(s.currentLogFile.Name())
}

// rotateLogFileIfNeeded checks if log rotation is needed
func (s *ActivityLoggerService) rotateLogFileIfNeeded(logFile *os.File) error {
	// Get file info
	fileInfo, err := logFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %v", err)
	}

	// Check if file size exceeds maximum
	if fileInfo.Size() >= s.maxFileSize {
		// Close current file
		logFile.Close()
		s.currentLogFile = nil

		// Rename current file with sequence number
		oldPath := filepath.Join(s.logDir, fileInfo.Name())
		today := time.Now().Format("2006-01-02")

		// Find next available sequence number
		sequence := 1
		for {
			newFileName := fmt.Sprintf("activity_%s_%d.log", today, sequence)
			newPath := filepath.Join(s.logDir, newFileName)

			if _, err := os.Stat(newPath); os.IsNotExist(err) {
				// Rename file
				if err := os.Rename(oldPath, newPath); err != nil {
					return fmt.Errorf("failed to rotate log file: %v", err)
				}
				break
			}

			sequence++
			if sequence > s.maxFilesPerDay {
				return fmt.Errorf("maximum log files per day reached")
			}
		}

		fmt.Printf("📄 Log file rotated: size=%d bytes\n", fileInfo.Size())
	}

	return nil
}

// GetActivityLogs retrieves activity logs with filters
func (s *ActivityLoggerService) GetActivityLogs(filter models.ActivityLogFilter) ([]models.ActivityLog, int64, error) {
	var logs []models.ActivityLog
	var total int64

	query := s.db.Model(&models.ActivityLog{})

	// Apply filters
	if filter.UserID != nil {
		query = query.Where("user_id = ?", *filter.UserID)
	}
	if filter.Username != "" {
		query = query.Where("username ILIKE ?", "%"+filter.Username+"%")
	}
	if filter.Role != "" {
		query = query.Where("role = ?", filter.Role)
	}
	if filter.Method != "" {
		query = query.Where("method = ?", filter.Method)
	}
	if filter.Path != "" {
		query = query.Where("path ILIKE ?", "%"+filter.Path+"%")
	}
	if filter.Resource != "" {
		query = query.Where("resource = ?", filter.Resource)
	}
	if filter.StatusCode != nil {
		query = query.Where("status_code = ?", *filter.StatusCode)
	}
	if filter.IsError != nil {
		query = query.Where("is_error = ?", *filter.IsError)
	}
	if filter.IPAddress != "" {
		query = query.Where("ip_address = ?", filter.IPAddress)
	}
	if filter.StartDate != nil {
		query = query.Where("created_at >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("created_at <= ?", *filter.EndDate)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count logs: %v", err)
	}

	// Apply pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	} else {
		query = query.Limit(100) // Default limit
	}

	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	// Preload user data and order by created_at descending
	if err := query.Preload("User").Order("created_at DESC").Find(&logs).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to fetch logs: %v", err)
	}

	return logs, total, nil
}

// GetActivitySummary returns a summary of activities grouped by user and date
func (s *ActivityLoggerService) GetActivitySummary(startDate, endDate time.Time) ([]models.ActivityLogSummary, error) {
	var summaries []models.ActivityLogSummary

	err := s.db.Model(&models.ActivityLog{}).
		Select(`
			DATE(created_at) as date,
			user_id,
			username,
			role,
			COUNT(*) as total_actions,
			SUM(CASE WHEN is_error = false THEN 1 ELSE 0 END) as success_count,
			SUM(CASE WHEN is_error = true THEN 1 ELSE 0 END) as error_count
		`).
		Where("created_at BETWEEN ? AND ?", startDate, endDate).
		Group("DATE(created_at), user_id, username, role").
		Order("date DESC, total_actions DESC").
		Scan(&summaries).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get activity summary: %v", err)
	}

	return summaries, nil
}

// CleanupOldLogs removes activity logs older than specified days
func (s *ActivityLoggerService) CleanupOldLogs(daysToKeep int) (int64, error) {
	cutoffDate := time.Now().AddDate(0, 0, -daysToKeep)

	result := s.db.Where("created_at < ?", cutoffDate).Delete(&models.ActivityLog{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to cleanup old logs: %v", result.Error)
	}

	fmt.Printf("🗑️  Cleaned up %d old activity logs (older than %d days)\n", result.RowsAffected, daysToKeep)
	return result.RowsAffected, nil
}

// CleanupOldLogFiles removes old log files
func (s *ActivityLoggerService) CleanupOldLogFiles(daysToKeep int) error {
	cutoffDate := time.Now().AddDate(0, 0, -daysToKeep)

	files, err := os.ReadDir(s.logDir)
	if err != nil {
		return fmt.Errorf("failed to read log directory: %v", err)
	}

	deletedCount := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Check if it's an activity log file
		matched, err := filepath.Match("activity_*.log", file.Name())
		if err != nil || !matched {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		// Check if file is older than cutoff date
		if info.ModTime().Before(cutoffDate) {
			filePath := filepath.Join(s.logDir, file.Name())
			if err := os.Remove(filePath); err != nil {
				fmt.Printf("⚠️  Failed to delete log file %s: %v\n", file.Name(), err)
			} else {
				deletedCount++
			}
		}
	}

	fmt.Printf("🗑️  Cleaned up %d old log files (older than %d days)\n", deletedCount, daysToKeep)
	return nil
}

// SetMaxFileSize sets the maximum log file size before rotation
func (s *ActivityLoggerService) SetMaxFileSize(sizeInMB int64) {
	s.maxFileSize = sizeInMB * 1024 * 1024
}

// SetEnableDB enables or disables database logging
func (s *ActivityLoggerService) SetEnableDB(enable bool) {
	s.enableDB = enable
}

// SetEnableFile enables or disables file logging
func (s *ActivityLoggerService) SetEnableFile(enable bool) {
	s.enableFile = enable
}

// Close closes the current log file
func (s *ActivityLoggerService) Close() error {
	s.fileMutex.Lock()
	defer s.fileMutex.Unlock()

	if s.currentLogFile != nil {
		return s.currentLogFile.Close()
	}

	return nil
}
