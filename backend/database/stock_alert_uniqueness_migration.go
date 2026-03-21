package database

import (
	"log"

	"gorm.io/gorm"
)

// EnsureStockAlertNotificationUniqueness removes duplicate stock alerts/notifications
// and enforces uniqueness for active stock alerts and unread stock notifications.
func EnsureStockAlertNotificationUniqueness(db *gorm.DB) error {
	log.Println("Checking stock alert and stock notification uniqueness...")

	alertCleanup := db.Exec(`
		WITH ranked AS (
			SELECT
				id,
				ROW_NUMBER() OVER (
					PARTITION BY product_id, alert_type
					ORDER BY last_alert_at DESC NULLS LAST, updated_at DESC, created_at DESC, id DESC
				) AS rn
			FROM stock_alerts
			WHERE deleted_at IS NULL AND status = 'ACTIVE'
		)
		UPDATE stock_alerts sa
		SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		FROM ranked r
		WHERE sa.id = r.id AND r.rn > 1
	`)
	if alertCleanup.Error != nil {
		return alertCleanup.Error
	}

	notificationCleanup := db.Exec(`
		WITH ranked AS (
			SELECT
				id,
				ROW_NUMBER() OVER (
					PARTITION BY user_id, type, (data->>'product_id')
					ORDER BY CASE WHEN is_read THEN 1 ELSE 0 END ASC, created_at DESC, id DESC
				) AS rn
			FROM notifications
			WHERE deleted_at IS NULL
			  AND type IN ('LOW_STOCK', 'STOCK_OUT', 'REORDER_ALERT')
			  AND COALESCE(data->>'product_id', '') <> ''
		)
		UPDATE notifications n
		SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		FROM ranked r
		WHERE n.id = r.id AND r.rn > 1
	`)
	if notificationCleanup.Error != nil {
		return notificationCleanup.Error
	}

	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_stock_alerts_active_unique
		ON stock_alerts (product_id, alert_type)
		WHERE deleted_at IS NULL AND status = 'ACTIVE'
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_notifications_stock_unread_unique
		ON notifications (user_id, type, ((data->>'product_id')))
		WHERE deleted_at IS NULL
		  AND is_read = false
		  AND type IN ('LOW_STOCK', 'STOCK_OUT', 'REORDER_ALERT')
		  AND COALESCE(data->>'product_id', '') <> ''
	`).Error; err != nil {
		return err
	}

	log.Printf("Stock alert uniqueness ready - removed %d duplicate alerts and %d duplicate notifications",
		alertCleanup.RowsAffected, notificationCleanup.RowsAffected)
	return nil
}
