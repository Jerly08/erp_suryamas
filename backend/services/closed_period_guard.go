package services

import (
	"fmt"
	"time"

	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

// CheckClosedPeriod checks if a given date falls within a closed accounting period.
// Returns a user-friendly error if the date is in a closed period, nil otherwise.
// This is a standalone helper that can be called from any service without
// needing to inject UnifiedPeriodClosingService.
func CheckClosedPeriod(db *gorm.DB, date time.Time) error {
	var count int64
	err := db.Model(&models.AccountingPeriod{}).
		Where("is_closed = ? AND ? BETWEEN start_date AND end_date", true, date).
		Count(&count).Error

	if err != nil {
		// If we can't check, log but don't block
		return nil
	}

	if count > 0 {
		return fmt.Errorf(
			"CLOSED_PERIOD:%s",
			date.Format("2006-01-02"),
		)
	}

	return nil
}
