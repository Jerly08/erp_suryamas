package controllers

import (
	"fmt"
	"net/http"
	"time"

	"app-sistem-akuntansi/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UnifiedClosingHistoryController handles closing history from both sources
type UnifiedClosingHistoryController struct {
	db *gorm.DB
}

// NewUnifiedClosingHistoryController creates a new unified closing history controller
func NewUnifiedClosingHistoryController(db *gorm.DB) *UnifiedClosingHistoryController {
	return &UnifiedClosingHistoryController{
		db: db,
	}
}

func historyDayStart(date time.Time) time.Time {
	return time.Date(date.In(time.Local).Year(), date.In(time.Local).Month(), date.In(time.Local).Day(), 0, 0, 0, 0, time.Local)
}

func isEffectiveClosedHistoryPeriod(period models.AccountingPeriod, now time.Time) bool {
	if !period.IsClosed {
		return false
	}
	return historyDayStart(period.EndDate).Before(historyDayStart(now))
}

// GetUnifiedClosingHistory returns combined history from all closing sources
func (c *UnifiedClosingHistoryController) GetUnifiedClosingHistory(ctx *gin.Context) {
	var history []map[string]interface{}

	// 1. Prefer accounting_periods because it is the canonical history for period closing.
	var accountingPeriods []models.AccountingPeriod
	err := c.db.Where("deleted_at IS NULL AND is_closed = ?", true).
		Order("end_date DESC").
		Find(&accountingPeriods).Error

	if err == nil && len(accountingPeriods) > 0 {
		filteredPeriods := make([]models.AccountingPeriod, 0, len(accountingPeriods))
		now := time.Now()
		for _, period := range accountingPeriods {
			if !isEffectiveClosedHistoryPeriod(period, now) {
				continue
			}
			filteredPeriods = append(filteredPeriods, period)
		}

		if len(filteredPeriods) > 20 {
			filteredPeriods = filteredPeriods[:20]
		}

		fmt.Printf("[UnifiedClosingHistory] Found %d effective closed periods in accounting_periods\n", len(filteredPeriods))
		for _, period := range filteredPeriods {
			// Generate a code for display
			periodCode := fmt.Sprintf("PC-%s", period.EndDate.Format("2006-01-02"))

			history = append(history, map[string]interface{}{
				"id":                   period.ID,
				"code":                 periodCode,
				"description":          period.Description,
				"entry_date":           period.EndDate,
				"start_date":           period.StartDate,
				"created_at":           period.CreatedAt,
				"total_debit":          period.NetIncome,
				"net_income":           period.NetIncome,
				"source":               "accounting_periods",
				"is_closed":            period.IsClosed,
				"status":               map[bool]string{true: "CLOSED", false: "REOPENED"}[period.IsClosed],
				"reopen_count":         period.ReopenCount,
				"reopen_reason":        period.ReopenReason,
				"reopening_journal_id": period.ReopeningJournalID,
			})
			if period.ClosedAt != nil {
				history[len(history)-1]["closed_at"] = period.ClosedAt
			}
			if period.ReopenedAt != nil {
				history[len(history)-1]["reopened_at"] = period.ReopenedAt
			}
		}
		if len(history) > 0 {
			fmt.Printf("[UnifiedClosingHistory] Total history entries: %d\n", len(history))

			ctx.JSON(http.StatusOK, gin.H{
				"success": true,
				"data":    history,
				"count":   len(history),
			})
			return
		}
	}

	// 2. Fallback to SSOT journal entries for legacy data that has no accounting_period record yet.
	var sootJournalEntries []models.SSOTJournalEntry
	err = c.db.Where("source_type IN ? AND status = ? AND deleted_at IS NULL",
		[]string{"CLOSING", "PERIOD_CLOSING", "FISCAL_CLOSING"},
		models.SSOTStatusPosted).
		Order("entry_date DESC").
		Limit(20).
		Find(&sootJournalEntries).Error

	if err == nil && len(sootJournalEntries) > 0 {
		fmt.Printf("[UnifiedClosingHistory] Found %d SSOT closing entries\n", len(sootJournalEntries))
		for _, entry := range sootJournalEntries {
			// Generate a code
			entryCode := fmt.Sprintf("CLO-%d", entry.ID)

			history = append(history, map[string]interface{}{
				"id":          entry.ID,
				"code":        entryCode,
				"description": entry.Description,
				"entry_date":  entry.EntryDate,
				"created_at":  entry.CreatedAt,
				"total_debit": entry.TotalDebit.InexactFloat64(),
				"source":      "ssot_journal",
			})
		}
	}

	fmt.Printf("[UnifiedClosingHistory] Total history entries: %d\n", len(history))

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    history,
		"count":   len(history),
	})
}
