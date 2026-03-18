package services

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"gorm.io/gorm"
	"log"
	"sync"
	"time"
)

type BalanceMonitoringService struct {
	db                 *gorm.DB
	integrityValidator *BalanceIntegrityValidator
	monitoringEnabled  bool
	mutex              sync.RWMutex
	lastCheck          time.Time
	alertThresholds    AlertThresholds
}

// AlertThresholds defines when to trigger alerts
type AlertThresholds struct {
	CriticalBalanceDifference   float64 // IDR amount
	HighBalanceDifference       float64
	MaxTolerableInconsistencies int
	ValidationFrequency         time.Duration
}

func NewBalanceMonitoringService(db *gorm.DB) *BalanceMonitoringService {
	return &BalanceMonitoringService{
		db:                 db,
		integrityValidator: NewBalanceIntegrityValidator(db),
		monitoringEnabled:  true,
		alertThresholds: AlertThresholds{
			CriticalBalanceDifference:   1000000, // 1M IDR
			HighBalanceDifference:       100000,  // 100K IDR
			MaxTolerableInconsistencies: 5,
			ValidationFrequency:         time.Minute * 15,
		},
	}
}

// BalanceDiscrepancy represents a balance mismatch between cash/bank and GL accounts
type BalanceDiscrepancy struct {
	CashBankID      uint      `json:"cash_bank_id"`
	CashBankCode    string    `json:"cash_bank_code"`
	CashBankName    string    `json:"cash_bank_name"`
	CashBankBalance float64   `json:"cash_bank_balance"`
	GLAccountID     uint      `json:"gl_account_id"`
	GLCode          string    `json:"gl_code"`
	GLName          string    `json:"gl_name"`
	GLBalance       float64   `json:"gl_balance"`
	Difference      float64   `json:"difference"`
	DetectedAt      time.Time `json:"detected_at"`
}

// DoublePostingReport represents the result of double posting detection
type DoublePostingReport struct {
	CheckTime                    time.Time              `json:"check_time"`
	TotalAccountsChecked         int                    `json:"total_accounts_checked"`
	DoublePostingPatternsFound   int                    `json:"double_posting_patterns_found"`
	DoublePostingInconsistencies []BalanceInconsistency `json:"double_posting_inconsistencies"`
	OverallHealthScore           float64                `json:"overall_health_score"`
	Status                       string                 `json:"status"` // "CLEAN", "WARNING", "CRITICAL"
}

// BalanceMonitoringResult represents the result of balance monitoring check
type BalanceMonitoringResult struct {
	CheckTime              time.Time            `json:"check_time"`
	TotalAccountsChecked   int                  `json:"total_accounts_checked"`
	SynchronizedAccounts   int                  `json:"synchronized_accounts"`
	UnsynchronizedAccounts int                  `json:"unsynchronized_accounts"`
	Discrepancies          []BalanceDiscrepancy `json:"discrepancies"`
	Status                 string               `json:"status"` // "OK", "WARNING", "ERROR"
}

// CheckBalanceSynchronization performs a comprehensive balance sync check
func (s *BalanceMonitoringService) CheckBalanceSynchronization() (*BalanceMonitoringResult, error) {
	log.Println("🔍 Starting balance synchronization check...")

	var discrepancies []BalanceDiscrepancy

	// Query to find all unsynchronized accounts
	err := s.db.Raw(`
		SELECT 
			cb.id as cash_bank_id,
			cb.code as cash_bank_code,
			cb.name as cash_bank_name,
			cb.balance as cash_bank_balance,
			cb.account_id as gl_account_id,
			acc.code as gl_code,
			acc.name as gl_name,
			acc.balance as gl_balance,
			cb.balance - acc.balance as difference
		FROM cash_banks cb 
		INNER JOIN accounts acc ON cb.account_id = acc.id
		WHERE cb.deleted_at IS NULL 
		  AND cb.balance != acc.balance
		ORDER BY cb.type, cb.code
	`).Scan(&discrepancies).Error

	if err != nil {
		log.Printf("❌ Error checking balance synchronization: %v", err)
		return nil, fmt.Errorf("failed to check balance synchronization: %w", err)
	}

	// Get total accounts count
	var totalAccounts int64
	err = s.db.Model(&models.CashBank{}).Where("deleted_at IS NULL").Count(&totalAccounts).Error
	if err != nil {
		log.Printf("⚠️  Warning: Could not get total accounts count: %v", err)
		totalAccounts = int64(len(discrepancies)) // fallback
	}

	// Set detection time
	checkTime := time.Now()
	for i := range discrepancies {
		discrepancies[i].DetectedAt = checkTime
	}

	result := &BalanceMonitoringResult{
		CheckTime:              checkTime,
		TotalAccountsChecked:   int(totalAccounts),
		SynchronizedAccounts:   int(totalAccounts) - len(discrepancies),
		UnsynchronizedAccounts: len(discrepancies),
		Discrepancies:          discrepancies,
	}

	// Determine status
	if len(discrepancies) == 0 {
		result.Status = "OK"
		log.Println("✅ All cash/bank accounts are synchronized with GL accounts")
	} else if len(discrepancies) <= 3 {
		result.Status = "WARNING"
		log.Printf("⚠️  Found %d balance discrepancies (WARNING level)", len(discrepancies))
	} else {
		result.Status = "ERROR"
		log.Printf("🚨 Found %d balance discrepancies (ERROR level)", len(discrepancies))
	}

	return result, nil
}

// AutoFixDiscrepancies automatically fixes balance discrepancies by updating GL account balances
func (s *BalanceMonitoringService) AutoFixDiscrepancies(result *BalanceMonitoringResult) error {
	if len(result.Discrepancies) == 0 {
		log.Println("✅ No discrepancies to fix")
		return nil
	}

	log.Printf("🔧 Auto-fixing %d balance discrepancies...", len(result.Discrepancies))

	tx := s.db.Begin()
	fixedCount := 0
	errorCount := 0

	for _, discrepancy := range result.Discrepancies {
		log.Printf("Fixing %s (%s): GL %.2f -> CB %.2f",
			discrepancy.CashBankCode,
			discrepancy.CashBankName,
			discrepancy.GLBalance,
			discrepancy.CashBankBalance)

		err := tx.Model(&models.Account{}).Where("id = ?", discrepancy.GLAccountID).
			Update("balance", discrepancy.CashBankBalance).Error

		if err != nil {
			log.Printf("❌ Failed to fix %s: %v", discrepancy.CashBankCode, err)
			errorCount++
		} else {
			log.Printf("✅ Fixed %s successfully", discrepancy.CashBankCode)
			fixedCount++
		}
	}

	if errorCount > 0 {
		tx.Rollback()
		return fmt.Errorf("failed to fix %d discrepancies, rolled back all changes", errorCount)
	}

	err := tx.Commit().Error
	if err != nil {
		return fmt.Errorf("failed to commit balance fixes: %w", err)
	}

	log.Printf("🎉 Successfully fixed %d balance discrepancies", fixedCount)
	return nil
}

// LogDiscrepancies logs balance discrepancies for audit purposes
func (s *BalanceMonitoringService) LogDiscrepancies(result *BalanceMonitoringResult) {
	if len(result.Discrepancies) == 0 {
		return
	}

	log.Println("📊 BALANCE DISCREPANCY REPORT")
	log.Println("=====================================")
	log.Printf("Check Time: %s", result.CheckTime.Format("2006-01-02 15:04:05"))
	log.Printf("Total Accounts: %d", result.TotalAccountsChecked)
	log.Printf("Synchronized: %d", result.SynchronizedAccounts)
	log.Printf("Unsynchronized: %d", result.UnsynchronizedAccounts)
	log.Printf("Status: %s", result.Status)
	log.Println("=====================================")

	for _, discrepancy := range result.Discrepancies {
		log.Printf("Account: %s (%s)", discrepancy.CashBankCode, discrepancy.CashBankName)
		log.Printf("  Cash/Bank Balance: %.2f", discrepancy.CashBankBalance)
		log.Printf("  GL Balance: %.2f", discrepancy.GLBalance)
		log.Printf("  Difference: %.2f", discrepancy.Difference)
		log.Println("  ---")
	}
}

// RunPeriodicCheck runs balance sync check periodically
func (s *BalanceMonitoringService) RunPeriodicCheck(intervalMinutes int, autoFix bool) {
	ticker := time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
	defer ticker.Stop()

	log.Printf("🔄 Starting periodic balance monitoring (every %d minutes, autoFix: %v)", intervalMinutes, autoFix)

	for range ticker.C {
		result, err := s.CheckBalanceSynchronization()
		if err != nil {
			log.Printf("❌ Periodic check failed: %v", err)
			continue
		}

		s.LogDiscrepancies(result)

		if autoFix && len(result.Discrepancies) > 0 {
			if err := s.AutoFixDiscrepancies(result); err != nil {
				log.Printf("❌ Auto-fix failed: %v", err)
			}
		}

		// Send alert if status is not OK
		if result.Status != "OK" {
			s.SendAlert(result)
		}
	}
}

// SendAlert sends alert when balance discrepancies are detected
func (s *BalanceMonitoringService) SendAlert(result *BalanceMonitoringResult) {
	// In a real implementation, this could send email, Slack notification, etc.
	log.Printf("🚨 ALERT: Balance sync issues detected!")
	log.Printf("Status: %s, Discrepancies: %d", result.Status, len(result.Discrepancies))

	// For now, just log the alert
	// TODO: Implement actual alerting mechanism (email, webhook, etc.)
}

// GetBalanceHealth returns overall balance health metrics
func (s *BalanceMonitoringService) GetBalanceHealth() (map[string]interface{}, error) {
	result, err := s.CheckBalanceSynchronization()
	if err != nil {
		return nil, err
	}

	totalDifference := 0.0
	maxDifference := 0.0
	for _, disc := range result.Discrepancies {
		totalDifference += disc.Difference
		if disc.Difference > maxDifference {
			maxDifference = disc.Difference
		}
	}

	health := map[string]interface{}{
		"status":                  result.Status,
		"total_accounts":          result.TotalAccountsChecked,
		"synchronized_accounts":   result.SynchronizedAccounts,
		"unsynchronized_accounts": result.UnsynchronizedAccounts,
		"sync_percentage":         float64(result.SynchronizedAccounts) / float64(result.TotalAccountsChecked) * 100,
		"total_difference_amount": totalDifference,
		"max_difference_amount":   maxDifference,
		"last_check_time":         result.CheckTime,
	}

	return health, nil
}

// DetectDoublePosting detects potential double posting patterns using integrity validator
func (s *BalanceMonitoringService) DetectDoublePosting() (*DoublePostingReport, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	log.Println("🔍 Detecting double posting patterns...")

	// Run integrity validation to detect double posting patterns
	validationResult, err := s.integrityValidator.ValidateAllBalances()
	if err != nil {
		return nil, fmt.Errorf("validation failed: %v", err)
	}

	// Convert BalanceInconsistency to BalanceDiscrepancy for compatibility
	doublePostingInconsistencies := []BalanceInconsistency{}
	for _, inconsistency := range validationResult.Inconsistencies {
		if inconsistency.Type == "DOUBLE_POSTING_PATTERN" {
			doublePostingInconsistencies = append(doublePostingInconsistencies, inconsistency)
		}
	}

	report := &DoublePostingReport{
		CheckTime:                    time.Now(),
		TotalAccountsChecked:         validationResult.TotalChecks,
		DoublePostingPatternsFound:   len(doublePostingInconsistencies),
		DoublePostingInconsistencies: doublePostingInconsistencies,
		OverallHealthScore:           float64(validationResult.TotalChecks-validationResult.FailedChecks) / float64(validationResult.TotalChecks) * 100,
	}

	// Determine severity
	if len(doublePostingInconsistencies) == 0 {
		report.Status = "CLEAN"
		log.Println("✅ No double posting patterns detected")
	} else if len(doublePostingInconsistencies) <= 2 {
		report.Status = "WARNING"
		log.Printf("⚠️ Found %d potential double posting patterns (WARNING)", len(doublePostingInconsistencies))
	} else {
		report.Status = "CRITICAL"
		log.Printf("🚨 Found %d double posting patterns (CRITICAL)", len(doublePostingInconsistencies))
	}

	return report, nil
}

// MonitorPaymentPosting monitors a specific payment for double posting
func (s *BalanceMonitoringService) MonitorPaymentPosting(paymentID uint, cashBankID uint, expectedAmount float64) {
	if !s.monitoringEnabled {
		return
	}

	log.Printf("👁️ Monitoring payment %d for double posting (Amount: %.2f)", paymentID, expectedAmount)

	// Wait for posting operations to complete
	time.Sleep(time.Second * 2)

	// Check cash bank transactions for this payment
	var transactions []models.CashBankTransaction
	err := s.db.Where("reference_type IN (?, ?) AND reference_id = ?",
		"PAYMENT", "SINGLE_SOURCE_POST", paymentID).Find(&transactions).Error

	if err != nil {
		log.Printf("❌ Failed to check transactions for payment %d: %v", paymentID, err)
		return
	}

	var totalAmount float64
	for _, tx := range transactions {
		if tx.CashBankID == cashBankID {
			totalAmount += tx.Amount
		}
	}

	// Check for double posting indicators
	if len(transactions) > 1 {
		log.Printf("🚨 ALERT: Payment %d created %d transactions - potential double posting!",
			paymentID, len(transactions))
	}

	if abs(totalAmount-expectedAmount) > 0.01 {
		log.Printf("🚨 ALERT: Payment %d amount mismatch - Expected: %.2f, Actual: %.2f",
			paymentID, expectedAmount, totalAmount)
	}

	// Check if balance is exactly double (classic double posting pattern)
	var cashBank models.CashBank
	if err := s.db.First(&cashBank, cashBankID).Error; err == nil {
		var transactionSum float64
		s.db.Model(&models.CashBankTransaction{}).
			Where("cash_bank_id = ?", cashBankID).
			Select("COALESCE(SUM(amount), 0)").
			Scan(&transactionSum)

		if abs(cashBank.Balance-transactionSum*2) < 0.01 {
			log.Printf("🚨 CRITICAL: Double posting pattern detected in CashBank %d - Balance is exactly 2x transaction sum!", cashBankID)
		}
	}
}

// StartContinuousMonitoring starts continuous balance monitoring
func (s *BalanceMonitoringService) StartContinuousMonitoring() {
	if !s.monitoringEnabled {
		log.Println("📊 Balance monitoring is disabled")
		return
	}

	log.Printf("🚀 Starting continuous balance monitoring (every %v)", s.alertThresholds.ValidationFrequency)

	ticker := time.NewTicker(s.alertThresholds.ValidationFrequency)
	go func() {
		for range ticker.C {
			if s.monitoringEnabled {
				s.runFullIntegrityCheck()
			}
		}
	}()
}

// runFullIntegrityCheck runs a complete integrity check
func (s *BalanceMonitoringService) runFullIntegrityCheck() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.lastCheck = time.Now()
	log.Println("🔍 Running full integrity check...")

	// Run balance synchronization check
	syncResult, err := s.CheckBalanceSynchronization()
	if err != nil {
		log.Printf("❌ Sync check failed: %v", err)
		return
	}

	// Run double posting detection
	doublePostingReport, err := s.DetectDoublePosting()
	if err != nil {
		log.Printf("❌ Double posting detection failed: %v", err)
		return
	}

	// Log results
	if syncResult.Status == "OK" && doublePostingReport.Status == "CLEAN" {
		log.Println("✅ Full integrity check passed - all systems healthy")
	} else {
		log.Printf("⚠️ Integrity issues detected - Sync: %s, DoublePosting: %s",
			syncResult.Status, doublePostingReport.Status)

		if doublePostingReport.Status == "CRITICAL" {
			log.Printf("🚨 CRITICAL: %d double posting patterns detected - immediate action required!",
				doublePostingReport.DoublePostingPatternsFound)
		}
	}
}

// GetMonitoringStatus returns current monitoring status
func (s *BalanceMonitoringService) GetMonitoringStatus() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return map[string]interface{}{
		"monitoring_enabled":     s.monitoringEnabled,
		"last_check":             s.lastCheck.Format(time.RFC3339),
		"validation_frequency":   s.alertThresholds.ValidationFrequency.String(),
		"critical_threshold_idr": s.alertThresholds.CriticalBalanceDifference,
		"high_threshold_idr":     s.alertThresholds.HighBalanceDifference,
		"max_inconsistencies":    s.alertThresholds.MaxTolerableInconsistencies,
		"system_status":          "OPERATIONAL",
	}
}
