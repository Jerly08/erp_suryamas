package services

import (
	"encoding/json"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"gorm.io/gorm"
)

type NotificationService struct {
	notificationRepo *repositories.NotificationRepository
	smartService     *SmartNotificationService
	db               *gorm.DB
}

func NewNotificationService(db *gorm.DB, notificationRepo *repositories.NotificationRepository) *NotificationService {
	return &NotificationService{
		notificationRepo: notificationRepo,
		smartService:     NewSmartNotificationService(db, notificationRepo),
		db:               db,
	}
}

// CreateNotification creates a new notification
func (s *NotificationService) CreateNotification(notification *models.Notification) error {
	return s.notificationRepo.Create(notification)
}

// GetUserNotifications gets notifications for a user
func (s *NotificationService) GetUserNotifications(userID uint, page, limit int, onlyUnread bool) ([]models.Notification, int64, error) {
	return s.notificationRepo.GetUserNotifications(userID, page, limit, onlyUnread)
}

// GetNotificationsByType gets notifications by type for a user
func (s *NotificationService) GetNotificationsByType(userID uint, notificationType string, page, limit int) ([]models.Notification, int64, error) {
	return s.notificationRepo.GetNotificationsByType(userID, notificationType, page, limit)
}

// GetNotificationsByTypes gets notifications by multiple types for a user
func (s *NotificationService) GetNotificationsByTypes(userID uint, notificationTypes []string, page, limit int) ([]models.Notification, int64, error) {
	return s.notificationRepo.GetNotificationsByTypes(userID, notificationTypes, page, limit)
}

// MarkAsRead marks a notification as read
func (s *NotificationService) MarkAsRead(notificationID, userID uint) error {
	return s.notificationRepo.MarkAsRead(notificationID, userID)
}

// MarkAllAsRead marks all notifications as read for a user
func (s *NotificationService) MarkAllAsRead(userID uint) error {
	return s.notificationRepo.MarkAllAsRead(userID)
}

// GetUnreadCount gets count of unread notifications
func (s *NotificationService) GetUnreadCount(userID uint) (int64, error) {
	return s.notificationRepo.GetUnreadCount(userID)
}

// GetUnreadCountByTypes gets unread notification count filtered by notification types
func (s *NotificationService) GetUnreadCountByTypes(userID uint, notificationTypes []string) (int64, error) {
	return s.notificationRepo.GetUnreadCountByTypes(userID, notificationTypes)
}

// DeleteNotification deletes a notification
func (s *NotificationService) DeleteNotification(notificationID, userID uint) error {
	return s.notificationRepo.Delete(notificationID, userID)
}

// CreateApprovalNotification creates approval-related notifications
func (s *NotificationService) CreateApprovalNotification(userID uint, notificationType, title, message string, data interface{}) error {
	// Convert data to JSON string
	var dataString string
	if data != nil {
		dataBytes, err := json.Marshal(data)
		if err == nil {
			dataString = string(dataBytes)
		}
	}

	notification := &models.Notification{
		UserID:   userID,
		Type:     notificationType,
		Title:    title,
		Message:  message,
		Data:     dataString,
		Priority: s.getNotificationPriority(notificationType),
	}

	return s.CreateNotification(notification)
}

// CreatePurchaseSubmissionNotification notifies when purchase is submitted for approval
func (s *NotificationService) CreatePurchaseSubmissionNotification(purchase *models.Purchase) error {
	// Use smart notification service for intelligent routing
	return s.smartService.CreatePurchaseNotification(purchase, "SUBMITTED", nil)
}

// CreatePurchaseApprovedNotification notifies when purchase is approved
func (s *NotificationService) CreatePurchaseApprovedNotification(purchase *models.Purchase, approverID uint) error {
	// Use smart notification service
	return s.smartService.CreatePurchaseNotification(purchase, "APPROVED", map[string]interface{}{
		"approver_id": approverID,
	})
}

// CreatePurchaseRejectedNotification notifies when purchase is rejected
func (s *NotificationService) CreatePurchaseRejectedNotification(purchase *models.Purchase, approverID uint, reason string) error {
	// Use smart notification service
	return s.smartService.CreatePurchaseNotification(purchase, "REJECTED", map[string]interface{}{
		"approver_id": approverID,
		"reason":      reason,
	})
}

// SendBulkNotification sends notification to multiple users
func (s *NotificationService) SendBulkNotification(userIDs []uint, notificationType, title, message string, data interface{}) error {
	for _, userID := range userIDs {
		err := s.CreateApprovalNotification(userID, notificationType, title, message, data)
		if err != nil {
			return err
		}
	}
	return nil
}

// Private helper methods

func (s *NotificationService) getNotificationPriority(notificationType string) string {
	switch notificationType {
	case models.NotificationTypeApprovalPending:
		return models.NotificationPriorityHigh
	case models.NotificationTypeApprovalRejected:
		return models.NotificationPriorityHigh
	case models.NotificationTypeApprovalApproved:
		return models.NotificationPriorityNormal
	default:
		return models.NotificationPriorityNormal
	}
}

// DEPRECATED: Use smart notification service instead
// getApproversForPurchase is now handled by SmartNotificationService
func (s *NotificationService) getApproversForPurchase(purchase *models.Purchase) []uint {
	// This method is deprecated - use SmartNotificationService.getEligibleUsers instead
	// Keeping for backward compatibility
	var users []models.User

	if purchase.TotalAmount <= 25000000 {
		// Get finance users from database
		s.db.Where("LOWER(role) = LOWER(?) AND is_active = ?", "finance", true).Find(&users)
	} else {
		// Get director users from database
		s.db.Where("LOWER(role) = LOWER(?) AND is_active = ?", "director", true).Find(&users)
	}

	var approvers []uint
	for _, user := range users {
		approvers = append(approvers, user.ID)
	}

	return approvers
}

// DEPRECATED: Use database queries instead
func (s *NotificationService) getFinanceUserIDs() []uint {
	var users []models.User
	s.db.Where("LOWER(role) = LOWER(?) AND is_active = ?", "finance", true).Find(&users)

	var ids []uint
	for _, user := range users {
		ids = append(ids, user.ID)
	}
	return ids
}

// DEPRECATED: Use database queries instead
func (s *NotificationService) getDirectorUserIDs() []uint {
	var users []models.User
	s.db.Where("LOWER(role) = LOWER(?) AND is_active = ?", "director", true).Find(&users)

	var ids []uint
	for _, user := range users {
		ids = append(ids, user.ID)
	}
	return ids
}

// CleanupOldNotifications removes old notifications
func (s *NotificationService) CleanupOldNotifications(daysOld int) error {
	cutoffDate := time.Now().AddDate(0, 0, -daysOld)
	return s.notificationRepo.DeleteOlderThan(cutoffDate)
}

// GetNotificationStats gets notification statistics
func (s *NotificationService) GetNotificationStats(userID uint) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get total notifications
	total, err := s.notificationRepo.GetTotalCount(userID)
	if err != nil {
		return nil, err
	}
	stats["total_notifications"] = total

	// Get unread count
	unread, err := s.GetUnreadCount(userID)
	if err != nil {
		return nil, err
	}
	stats["unread_notifications"] = unread

	// Get count by type
	approvalPending, _ := s.notificationRepo.GetCountByType(userID, models.NotificationTypeApprovalPending)
	approvalApproved, _ := s.notificationRepo.GetCountByType(userID, models.NotificationTypeApprovalApproved)
	approvalRejected, _ := s.notificationRepo.GetCountByType(userID, models.NotificationTypeApprovalRejected)

	stats["approval_pending"] = approvalPending
	stats["approval_approved"] = approvalApproved
	stats["approval_rejected"] = approvalRejected

	return stats, nil
}

// GetNotificationSummary gets unread notification counts grouped for UI badges
func (s *NotificationService) GetNotificationSummary(userID uint) (map[string]int64, error) {
	summary := map[string]int64{
		"approval_unread": 0,
		"stock_unread":    0,
		"total_unread":    0,
	}

	approvalTypes := []string{
		models.NotificationTypeApprovalPending,
		models.NotificationTypeApprovalApproved,
		models.NotificationTypeApprovalRejected,
	}
	stockTypes := []string{
		models.NotificationTypeLowStock,
		models.NotificationTypeReorderAlert,
		models.NotificationTypeStockOut,
	}

	approvalUnread, err := s.GetUnreadCountByTypes(userID, approvalTypes)
	if err != nil {
		return nil, err
	}

	stockUnread, err := s.GetUnreadCountByTypes(userID, stockTypes)
	if err != nil {
		return nil, err
	}

	totalUnread, err := s.GetUnreadCount(userID)
	if err != nil {
		return nil, err
	}

	summary["approval_unread"] = approvalUnread
	summary["stock_unread"] = stockUnread
	summary["total_unread"] = totalUnread

	return summary, nil
}
