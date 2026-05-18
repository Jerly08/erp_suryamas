package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/utils"
	"gorm.io/gorm"
)

type SmartNotificationService struct {
	db               *gorm.DB
	notificationRepo *repositories.NotificationRepository
}

func NewSmartNotificationService(db *gorm.DB, notificationRepo *repositories.NotificationRepository) *SmartNotificationService {
	return &SmartNotificationService{
		db:               db,
		notificationRepo: notificationRepo,
	}
}

// CreateSmartNotification creates notifications based on role matrix and user preferences
func (s *SmartNotificationService) CreateSmartNotification(
	notificationType string,
	title string,
	message string,
	data interface{},
	amount float64,
	targetRole string,
	targetUserID uint, // Optional: specific user override
	department string, // Optional: department filter
) error {
	// Get notification matrix rules
	matrix := models.GetNotificationMatrix()

	// Convert data to JSON string
	var dataString string
	if data != nil {
		dataBytes, err := json.Marshal(data)
		if err == nil {
			dataString = string(dataBytes)
		}
	}

	// If specific user is targeted
	if targetUserID > 0 {
		return s.createNotificationForUser(targetUserID, notificationType, title, message, dataString)
	}

	// Find eligible users based on role and rules
	eligibleUsers, err := s.getEligibleUsers(targetRole, notificationType, amount, department, matrix)
	if err != nil {
		return fmt.Errorf("failed to get eligible users: %v", err)
	}

	// Check if batch notification is needed
	if len(eligibleUsers) > 3 && s.shouldBatchNotifications(notificationType) {
		return s.createBatchNotification(eligibleUsers, notificationType, title, message, dataString, amount)
	}

	// Create individual notifications
	for _, userID := range eligibleUsers {
		// Check user preferences
		if !s.userWantsNotification(userID, notificationType) {
			continue
		}

		// Check daily limit
		if s.hasReachedDailyLimit(userID) {
			continue
		}

		// Create notification
		err := s.createNotificationForUser(userID, notificationType, title, message, dataString)
		if err != nil {
			// Log error but continue with other users
			fmt.Printf("Failed to create notification for user %d: %v\n", userID, err)
		}
	}

	return nil
}

// getEligibleUsers finds users who should receive the notification
func (s *SmartNotificationService) getEligibleUsers(
	targetRole string,
	notificationType string,
	amount float64,
	department string,
	matrix map[string]models.NotificationMatrix,
) ([]uint, error) {
	var eligibleUsers []uint

	// Get role-specific matrix
	roleMatrix, exists := matrix[strings.ToLower(targetRole)]
	if !exists {
		return nil, fmt.Errorf("no notification matrix for role: %s", targetRole)
	}

	// Check if this notification type is allowed for this role
	if !s.isNotificationTypeAllowed(notificationType, roleMatrix.AllowedTypes) {
		return nil, nil // No users eligible
	}

	// Check amount threshold
	if roleMatrix.AmountThreshold > 0 {
		// For finance: only amounts <= threshold
		if targetRole == "finance" && amount > roleMatrix.AmountThreshold {
			return nil, nil // Amount too high for finance
		}
		// For director: only amounts >= threshold
		if targetRole == "director" && amount < roleMatrix.AmountThreshold {
			return nil, nil // Amount too low for director
		}
	}

	// Query users based on criteria
	targetRoles := models.WorkflowStepAssignableRoles(targetRole)
	if len(targetRoles) == 0 {
		targetRoles = []string{targetRole}
	}
	query := s.db.Model(&models.User{}).
		Where("LOWER(role) IN ? AND is_active = ?", targetRoles, true)

	// Apply department filter if needed
	if department != "" && roleMatrix.RequiresDepartmentMatch {
		query = query.Where("department = ?", department)
	}

	var users []models.User
	if err := query.Find(&users).Error; err != nil {
		return nil, err
	}

	// Extract user IDs
	for _, user := range users {
		eligibleUsers = append(eligibleUsers, user.ID)
	}

	// Apply workload balancing for approvers
	if notificationType == models.NotificationTypeApprovalPending && len(eligibleUsers) > 1 {
		return s.balanceApprovalWorkload(eligibleUsers, targetRole)
	}

	return eligibleUsers, nil
}

// balanceApprovalWorkload distributes approval notifications evenly
func (s *SmartNotificationService) balanceApprovalWorkload(userIDs []uint, role string) ([]uint, error) {
	if len(userIDs) <= 1 {
		return userIDs, nil
	}

	// Count pending approvals for each user
	type UserWorkload struct {
		UserID       uint
		PendingCount int
	}

	var workloads []UserWorkload
	for _, userID := range userIDs {
		var count int64
		err := s.db.Model(&models.Notification{}).
			Where("user_id = ? AND type = ? AND is_read = ?",
				userID, models.NotificationTypeApprovalPending, false).
			Count(&count).Error

		if err != nil {
			continue
		}

		workloads = append(workloads, UserWorkload{
			UserID:       userID,
			PendingCount: int(count),
		})
	}

	// Find user with least workload
	if len(workloads) > 0 {
		minWorkload := workloads[0]
		for _, w := range workloads {
			if w.PendingCount < minWorkload.PendingCount {
				minWorkload = w
			}
		}
		return []uint{minWorkload.UserID}, nil
	}

	// Fallback to first user
	return []uint{userIDs[0]}, nil
}

// isNotificationTypeAllowed checks if notification type is allowed for role
func (s *SmartNotificationService) isNotificationTypeAllowed(notificationType string, allowedTypes []string) bool {
	for _, allowed := range allowedTypes {
		if allowed == "ALL" || allowed == notificationType {
			return true
		}
	}
	return false
}

// userWantsNotification checks user preferences
func (s *SmartNotificationService) userWantsNotification(userID uint, notificationType string) bool {
	var pref models.NotificationPreference
	err := s.db.Where("user_id = ?", userID).First(&pref).Error

	// If no preferences found, default to true
	if err != nil {
		return true
	}

	// Check if in quiet hours
	if s.isInQuietHours(pref) {
		return false
	}

	// Check specific notification type preferences
	switch notificationType {
	case models.NotificationTypeApprovalPending:
		return pref.ApprovalPending
	case models.NotificationTypeApprovalApproved:
		return pref.ApprovalApproved
	case models.NotificationTypeApprovalRejected:
		return pref.ApprovalRejected
	case models.NotificationTypeApprovalEscalated:
		return pref.ApprovalEscalated
	case models.NotificationTypeLowStock, models.NotificationTypeStockOut:
		return pref.StockAlerts
	case models.NotificationTypePaymentDue:
		return pref.PaymentReminders
	default:
		return pref.SystemAlerts
	}
}

// isInQuietHours checks if current time is in user's quiet hours
func (s *SmartNotificationService) isInQuietHours(pref models.NotificationPreference) bool {
	if pref.QuietHoursStart == "" || pref.QuietHoursEnd == "" {
		return false
	}

	now := time.Now()
	currentTime := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())

	// Simple comparison (doesn't handle overnight quiet hours)
	return currentTime >= pref.QuietHoursStart && currentTime <= pref.QuietHoursEnd
}

// hasReachedDailyLimit checks if user has reached daily notification limit
func (s *SmartNotificationService) hasReachedDailyLimit(userID uint) bool {
	var pref models.NotificationPreference
	err := s.db.Where("user_id = ?", userID).First(&pref).Error
	if err != nil || pref.MaxDailyNotifications == 0 {
		return false // No limit set
	}

	// Count today's notifications
	var count int64
	today := time.Now().Truncate(24 * time.Hour)
	err = s.db.Model(&models.Notification{}).
		Where("user_id = ? AND created_at >= ?", userID, today).
		Count(&count).Error

	if err != nil {
		return false
	}

	return int(count) >= pref.MaxDailyNotifications
}

// shouldBatchNotifications determines if notifications should be batched
func (s *SmartNotificationService) shouldBatchNotifications(notificationType string) bool {
	// Batch approval notifications to reduce spam
	batchableTypes := []string{
		models.NotificationTypeApprovalPending,
		models.NotificationTypeLowStock,
		models.NotificationTypePaymentDue,
	}

	for _, bType := range batchableTypes {
		if bType == notificationType {
			return true
		}
	}
	return false
}

// createBatchNotification creates a batch notification for multiple items
func (s *SmartNotificationService) createBatchNotification(
	userIDs []uint,
	notificationType string,
	title string,
	message string,
	dataString string,
	totalAmount float64,
) error {
	for _, userID := range userIDs {
		// Check if user prefers batch notifications
		var pref models.NotificationPreference
		s.db.Where("user_id = ?", userID).First(&pref)

		if !pref.BatchNotifications {
			// Create individual notification
			s.createNotificationForUser(userID, notificationType, title, message, dataString)
			continue
		}

		// Check for existing batch
		var batch models.NotificationBatch
		err := s.db.Where(
			"user_id = ? AND batch_type = ? AND is_processed = ? AND created_at >= ?",
			userID, notificationType, false, time.Now().Add(-1*time.Hour),
		).First(&batch).Error

		if err == gorm.ErrRecordNotFound {
			// Create new batch
			batch = models.NotificationBatch{
				UserID:       userID,
				BatchType:    notificationType,
				ItemCount:    1,
				TotalAmount:  totalAmount,
				Summary:      fmt.Sprintf("You have new %s", notificationType),
				DetailedData: dataString,
			}
			s.db.Create(&batch)
		} else {
			// Update existing batch
			batch.ItemCount++
			batch.TotalAmount += totalAmount
			batch.Summary = fmt.Sprintf("You have %d %s items", batch.ItemCount, notificationType)
			s.db.Save(&batch)
		}
	}

	return nil
}

// createNotificationForUser creates a notification for a specific user
func (s *SmartNotificationService) createNotificationForUser(
	userID uint,
	notificationType string,
	title string,
	message string,
	dataString string,
) error {
	// Check for duplicate notifications
	if s.isDuplicateNotification(userID, notificationType, title, dataString) {
		return nil // Skip creating duplicate
	}

	notification := &models.Notification{
		UserID:   userID,
		Type:     notificationType,
		Title:    title,
		Message:  message,
		Data:     dataString,
		Priority: s.getNotificationPriority(notificationType),
	}

	return s.notificationRepo.Create(notification)
}

// isDuplicateNotification checks if similar notification already exists
func (s *SmartNotificationService) isDuplicateNotification(userID uint, notificationType, title, dataString string) bool {
	// Extract purchase_id from data to check for duplicates
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(dataString), &data); err != nil {
		return false // If can't parse, allow creation
	}

	purchaseID, ok := data["purchase_id"].(float64)
	if !ok {
		return false // If no purchase_id, check by title only
	}

	// Check for existing notification with same purchase_id, user_id, and type within last 1 hour
	var count int64
	oneHourAgo := time.Now().Add(-1 * time.Hour)

	err := s.db.Model(&models.Notification{}).
		Where("user_id = ? AND type = ? AND created_at >= ? AND (title = ? OR data::json->>'purchase_id' = ?)",
			userID, notificationType, oneHourAgo, title, fmt.Sprintf("%.0f", purchaseID)).
		Count(&count).Error

	if err != nil {
		return false // If error, allow creation
	}

	return count > 0
}

// getPurchaseDisplayAmount gets the correct amount to display for purchase notifications
func (s *SmartNotificationService) getPurchaseDisplayAmount(purchaseID uint) float64 {
	var purchase models.Purchase
	if err := s.db.First(&purchase, purchaseID).Error; err != nil {
		// If we can't get the purchase, return 0 to avoid showing wrong amounts
		return 0
	}
	// Always use TotalAmount for notifications to ensure consistency
	return purchase.TotalAmount
}

// getNotificationPriority determines priority based on type
func (s *SmartNotificationService) getNotificationPriority(notificationType string) string {
	switch notificationType {
	case models.NotificationTypeApprovalPending,
		models.NotificationTypeApprovalEscalated,
		models.NotificationTypeCriticalAlert:
		return models.NotificationPriorityHigh
	case models.NotificationTypeApprovalRejected,
		models.NotificationTypeBudgetExceeded:
		return models.NotificationPriorityHigh
	case models.NotificationTypeApprovalApproved,
		models.NotificationTypePaymentDue:
		return models.NotificationPriorityNormal
	default:
		return models.NotificationPriorityNormal
	}
}

// CreatePurchaseNotification creates smart notifications for purchase events
func (s *SmartNotificationService) CreatePurchaseNotification(
	purchase *models.Purchase,
	eventType string,
	additionalData map[string]interface{},
) error {
	switch eventType {
	case "SUBMITTED":
		// Notify appropriate approvers based on amount
		if purchase.TotalAmount <= 25000000 {
			// Finance approval needed
			return s.CreateSmartNotification(
				models.NotificationTypeApprovalPending,
				"Purchase Approval Required",
				fmt.Sprintf("Purchase %s requires approval (Amount: %s)",
					purchase.Code, utils.FormatRupiahWithoutDecimals(purchase.TotalAmount)),
				map[string]interface{}{
					"purchase_id":   purchase.ID,
					"purchase_code": purchase.Code,
					"vendor_name":   purchase.Vendor.Name,
					"total_amount":  purchase.TotalAmount,
					"action_type":   "approval_required",
				},
				purchase.TotalAmount,
				"finance",
				0,
				purchase.User.Department,
			)
		} else {
			// Director approval needed - first notify finance for initial review
			s.CreateSmartNotification(
				models.NotificationTypeHighValuePurchase,
				"High-Value Purchase for Review",
				fmt.Sprintf("High-value purchase %s needs review before director approval (Amount: %s)",
					purchase.Code, utils.FormatRupiahWithoutDecimals(purchase.TotalAmount)),
				map[string]interface{}{
					"purchase_id":   purchase.ID,
					"purchase_code": purchase.Code,
					"vendor_name":   purchase.Vendor.Name,
					"total_amount":  purchase.TotalAmount,
					"action_type":   "review_required",
				},
				purchase.TotalAmount,
				"finance",
				0,
				"",
			)

			// Also notify director
			return s.CreateSmartNotification(
				models.NotificationTypeApprovalPending,
				"High-Value Purchase Approval Required",
				fmt.Sprintf("High-value purchase %s requires your approval (Amount: %s)",
					purchase.Code, utils.FormatRupiahWithoutDecimals(purchase.TotalAmount)),
				map[string]interface{}{
					"purchase_id":   purchase.ID,
					"purchase_code": purchase.Code,
					"vendor_name":   purchase.Vendor.Name,
					"total_amount":  purchase.TotalAmount,
					"action_type":   "approval_required",
				},
				purchase.TotalAmount,
				"director",
				0,
				"",
			)
		}

	case "APPROVED":
		// Notify the purchase creator
		return s.CreateSmartNotification(
			models.NotificationTypeApprovalApproved,
			"Purchase Approved",
			fmt.Sprintf("Your purchase request %s has been approved", purchase.Code),
			map[string]interface{}{
				"purchase_id":   purchase.ID,
				"purchase_code": purchase.Code,
				"approved_at":   purchase.ApprovedAt,
				"action_type":   "approved",
			},
			purchase.TotalAmount,
			"employee",
			purchase.UserID,
			"",
		)

	case "REJECTED":
		// Notify the purchase creator
		reason := ""
		if data, ok := additionalData["reason"].(string); ok {
			reason = data
		}

		message := fmt.Sprintf("Your purchase request %s has been rejected", purchase.Code)
		if reason != "" {
			message += fmt.Sprintf(". Reason: %s", reason)
		}

		return s.CreateSmartNotification(
			models.NotificationTypeApprovalRejected,
			"Purchase Rejected",
			message,
			map[string]interface{}{
				"purchase_id":   purchase.ID,
				"purchase_code": purchase.Code,
				"rejected_at":   time.Now(),
				"reason":        reason,
				"action_type":   "rejected",
			},
			purchase.TotalAmount,
			"employee",
			purchase.UserID,
			"",
		)

	case "ESCALATED":
		// Notify director about escalation
		escalationReason := "Finance escalation"
		if data, ok := additionalData["reason"].(string); ok {
			escalationReason = data
		}

		return s.CreateSmartNotification(
			models.NotificationTypeApprovalEscalated,
			"URGENT: Purchase Escalated for Approval",
			fmt.Sprintf("Purchase %s has been escalated: %s (Amount: %s)",
				purchase.Code, escalationReason, utils.FormatRupiahWithoutDecimals(purchase.TotalAmount)),
			map[string]interface{}{
				"purchase_id":       purchase.ID,
				"purchase_code":     purchase.Code,
				"vendor_name":       purchase.Vendor.Name,
				"total_amount":      purchase.TotalAmount,
				"escalation_reason": escalationReason,
				"action_type":       "escalated",
			},
			purchase.TotalAmount,
			"director",
			0,
			"",
		)
	}

	return nil
}

// ProcessBatchNotifications processes pending batch notifications
func (s *SmartNotificationService) ProcessBatchNotifications() error {
	var batches []models.NotificationBatch
	err := s.db.Where("is_processed = ?", false).Find(&batches).Error
	if err != nil {
		return err
	}

	for _, batch := range batches {
		// Create summary notification
		title := fmt.Sprintf("%d %s Notifications", batch.ItemCount, batch.BatchType)
		message := batch.Summary
		if batch.TotalAmount > 0 {
			message += fmt.Sprintf(" (Total: %s)", utils.FormatRupiahWithoutDecimals(batch.TotalAmount))
		}

		notification := &models.Notification{
			UserID:   batch.UserID,
			Type:     batch.BatchType,
			Title:    title,
			Message:  message,
			Data:     batch.DetailedData,
			Priority: models.NotificationPriorityNormal,
		}

		if err := s.notificationRepo.Create(notification); err != nil {
			continue
		}

		// Mark batch as processed
		now := time.Now()
		batch.IsProcessed = true
		batch.ProcessedAt = &now
		s.db.Save(&batch)
	}

	return nil
}

// GetUserNotificationPreferences gets or creates user notification preferences
func (s *SmartNotificationService) GetUserNotificationPreferences(userID uint) (*models.NotificationPreference, error) {
	var pref models.NotificationPreference
	err := s.db.Where("user_id = ?", userID).First(&pref).Error

	if err == gorm.ErrRecordNotFound {
		// Create default preferences
		pref = models.NotificationPreference{
			UserID:                userID,
			EmailEnabled:          true,
			InAppEnabled:          true,
			PushEnabled:           false,
			BatchNotifications:    false,
			MaxDailyNotifications: 100,
			ApprovalPending:       true,
			ApprovalApproved:      true,
			ApprovalRejected:      true,
			ApprovalEscalated:     true,
			StockAlerts:           true,
			PaymentReminders:      true,
			SystemAlerts:          true,
		}

		if err := s.db.Create(&pref).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	return &pref, nil
}

// UpdateUserNotificationPreferences updates user notification preferences
func (s *SmartNotificationService) UpdateUserNotificationPreferences(userID uint, updates map[string]interface{}) error {
	return s.db.Model(&models.NotificationPreference{}).
		Where("user_id = ?", userID).
		Updates(updates).Error
}
