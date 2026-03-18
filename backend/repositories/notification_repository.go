package repositories

import (
	"time"

	"app-sistem-akuntansi/models"
	"gorm.io/gorm"
)

type NotificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

// Create creates a new notification
func (r *NotificationRepository) Create(notification *models.Notification) error {
	return r.db.Create(notification).Error
}

// GetUserNotifications gets notifications for a user with pagination
func (r *NotificationRepository) GetUserNotifications(userID uint, page, limit int, onlyUnread bool) ([]models.Notification, int64, error) {
	var notifications []models.Notification
	var total int64

	query := r.db.Model(&models.Notification{}).Where("user_id = ?", userID)

	if onlyUnread {
		query = query.Where("is_read = ?", false)
	}

	// Get total count
	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results
	offset := (page - 1) * limit
	err = query.Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&notifications).Error

	return notifications, total, err
}

// GetNotificationsByType gets notifications by type for a user
func (r *NotificationRepository) GetNotificationsByType(userID uint, notificationType string, page, limit int) ([]models.Notification, int64, error) {
	var notifications []models.Notification
	var total int64

	query := r.db.Model(&models.Notification{}).
		Where("user_id = ? AND type = ?", userID, notificationType)

	// Get total count
	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results
	offset := (page - 1) * limit
	err = query.Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&notifications).Error

	return notifications, total, err
}

// MarkAsRead marks a notification as read
func (r *NotificationRepository) MarkAsRead(notificationID, userID uint) error {
	return r.db.Model(&models.Notification{}).
		Where("id = ? AND user_id = ?", notificationID, userID).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": time.Now(),
		}).Error
}

// MarkAllAsRead marks all notifications as read for a user
func (r *NotificationRepository) MarkAllAsRead(userID uint) error {
	return r.db.Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": time.Now(),
		}).Error
}

// GetUnreadCount gets count of unread notifications for a user
func (r *NotificationRepository) GetUnreadCount(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&count).Error
	return count, err
}

// GetUnreadCountByTypes gets unread notification count for a user filtered by one or more types
func (r *NotificationRepository) GetUnreadCountByTypes(userID uint, notificationTypes []string) (int64, error) {
	var count int64

	query := r.db.Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false)

	if len(notificationTypes) > 0 {
		query = query.Where("type IN ?", notificationTypes)
	}

	err := query.Count(&count).Error
	return count, err
}

// Delete deletes a notification
func (r *NotificationRepository) Delete(notificationID, userID uint) error {
	return r.db.Where("id = ? AND user_id = ?", notificationID, userID).
		Delete(&models.Notification{}).Error
}

// FindByID finds a notification by ID
func (r *NotificationRepository) FindByID(id uint) (*models.Notification, error) {
	var notification models.Notification
	err := r.db.Preload("User").First(&notification, id).Error
	if err != nil {
		return nil, err
	}
	return &notification, nil
}

// GetTotalCount gets total count of notifications for a user
func (r *NotificationRepository) GetTotalCount(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Notification{}).
		Where("user_id = ?", userID).
		Count(&count).Error
	return count, err
}

// GetCountByType gets count of notifications by type for a user
func (r *NotificationRepository) GetCountByType(userID uint, notificationType string) (int64, error) {
	var count int64
	err := r.db.Model(&models.Notification{}).
		Where("user_id = ? AND type = ?", userID, notificationType).
		Count(&count).Error
	return count, err
}

// DeleteOlderThan deletes notifications older than specified date
func (r *NotificationRepository) DeleteOlderThan(cutoffDate time.Time) error {
	return r.db.Where("created_at < ?", cutoffDate).
		Delete(&models.Notification{}).Error
}

// BulkCreate creates multiple notifications
func (r *NotificationRepository) BulkCreate(notifications []models.Notification) error {
	return r.db.CreateInBatches(notifications, 100).Error
}

// GetNotificationsByPriority gets notifications by priority for a user
func (r *NotificationRepository) GetNotificationsByPriority(userID uint, priority string, page, limit int) ([]models.Notification, int64, error) {
	var notifications []models.Notification
	var total int64

	query := r.db.Model(&models.Notification{}).
		Where("user_id = ? AND priority = ?", userID, priority)

	// Get total count
	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results
	offset := (page - 1) * limit
	err = query.Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&notifications).Error

	return notifications, total, err
}

// GetRecentNotifications gets recent notifications (last 24 hours)
func (r *NotificationRepository) GetRecentNotifications(userID uint, limit int) ([]models.Notification, error) {
	var notifications []models.Notification

	since := time.Now().Add(-24 * time.Hour)

	err := r.db.Where("user_id = ? AND created_at >= ?", userID, since).
		Order("created_at DESC").
		Limit(limit).
		Find(&notifications).Error

	return notifications, err
}

// UpdateReadStatus updates read status of multiple notifications
func (r *NotificationRepository) UpdateReadStatus(notificationIDs []uint, userID uint, isRead bool) error {
	updates := map[string]interface{}{
		"is_read": isRead,
	}

	if isRead {
		updates["read_at"] = time.Now()
	} else {
		updates["read_at"] = nil
	}

	return r.db.Model(&models.Notification{}).
		Where("id IN ? AND user_id = ?", notificationIDs, userID).
		Updates(updates).Error
}

// GetNotificationsGroupedByType gets notifications grouped by type
func (r *NotificationRepository) GetNotificationsGroupedByType(userID uint) (map[string][]models.Notification, error) {
	var notifications []models.Notification

	err := r.db.Where("user_id = ?", userID).
		Order("type, created_at DESC").
		Find(&notifications).Error

	if err != nil {
		return nil, err
	}

	// Group by type
	grouped := make(map[string][]models.Notification)
	for _, notification := range notifications {
		grouped[notification.Type] = append(grouped[notification.Type], notification)
	}

	return grouped, nil
}

// GetNotificationsByTypes gets notifications filtered by multiple types with global sorting and pagination
func (r *NotificationRepository) GetNotificationsByTypes(userID uint, notificationTypes []string, page, limit int) ([]models.Notification, int64, error) {
	var notifications []models.Notification
	var total int64

	query := r.db.Model(&models.Notification{}).
		Where("user_id = ?", userID)

	if len(notificationTypes) > 0 {
		query = query.Where("type IN ?", notificationTypes)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&notifications).Error

	return notifications, total, err
}
