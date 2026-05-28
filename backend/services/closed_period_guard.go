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

// CheckClosedPeriodRange checks whether any date inside a transaction range
// overlaps with a closed accounting period.
func CheckClosedPeriodRange(db *gorm.DB, startDate, endDate time.Time) error {
	if endDate.Before(startDate) {
		startDate, endDate = endDate, startDate
	}

	var period models.AccountingPeriod
	err := db.Model(&models.AccountingPeriod{}).
		Where("is_closed = ? AND start_date <= ? AND end_date >= ?", true, endDate, startDate).
		Order("start_date ASC").
		First(&period).Error

	if err != nil {
		// Keep the same fail-open behavior as CheckClosedPeriod.
		return nil
	}

	blockedDate := startDate
	if period.StartDate.After(blockedDate) {
		blockedDate = period.StartDate
	}

	return fmt.Errorf(
		"CLOSED_PERIOD:%s",
		blockedDate.Format("2006-01-02"),
	)
}
