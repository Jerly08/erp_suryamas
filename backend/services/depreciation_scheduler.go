package services

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

// DepreciationScheduler runs monthly depreciation automatically.
// It checks daily whether the current month's depreciation has been processed.
// If not, it runs the depreciation for the previous month.
//
// Schedule Logic:
//   - Checks every 24 hours (configurable)
//   - On the 1st-5th of each month, automatically runs depreciation for the PREVIOUS month
//   - If depreciation for that month already exists (duplicate check in AssetJournalServiceSSOT),
//     the run is safely skipped
//   - Uses a system user ID (configurable, default 1) for journal entries
//
// Accounting Logic (PSAK 16 / IAS 16):
//   - Straight-Line: (Cost - Salvage) / (Useful Life × 12) per month
//   - Declining Balance: (2 / Useful Life) × Book Value / 12 per month
//   - Tanah (Land) assets are never depreciated
//   - Depreciation stops when Book Value reaches Salvage Value
//   - Each asset gets its own individual SSOT journal entry
type DepreciationScheduler struct {
	db              *gorm.DB
	assetJournalSvc *AssetJournalServiceSSOT
	systemUserID    uint
	checkInterval   time.Duration
	stopCh          chan struct{}
}

// NewDepreciationScheduler creates a new depreciation scheduler
func NewDepreciationScheduler(db *gorm.DB) *DepreciationScheduler {
	return &DepreciationScheduler{
		db:              db,
		assetJournalSvc: NewAssetJournalServiceSSOT(db),
		systemUserID:    1,   // Default system user for automated entries
		checkInterval:   24 * time.Hour,
		stopCh:          make(chan struct{}),
	}
}

// SetSystemUserID sets the user ID used for automated journal entries
func (ds *DepreciationScheduler) SetSystemUserID(userID uint) {
	ds.systemUserID = userID
}

// Start begins the background depreciation scheduler
// This should be called as a goroutine: go scheduler.Start()
func (ds *DepreciationScheduler) Start() {
	log.Println("📅 Depreciation scheduler started - checking daily for pending monthly depreciation")

	// Run initial check on startup (delayed by 30 seconds to let other services initialize)
	time.Sleep(30 * time.Second)
	ds.checkAndRunDepreciation()

	ticker := time.NewTicker(ds.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ds.checkAndRunDepreciation()
		case <-ds.stopCh:
			log.Println("📅 Depreciation scheduler stopped")
			return
		}
	}
}

// Stop stops the depreciation scheduler
func (ds *DepreciationScheduler) Stop() {
	close(ds.stopCh)
}

// checkAndRunDepreciation checks if depreciation needs to be run and executes it
func (ds *DepreciationScheduler) checkAndRunDepreciation() {
	now := time.Now()
	day := now.Day()

	// Only auto-run depreciation on the 1st through 5th of each month
	// This gives a window for the system to process after month-end
	if day > 5 {
		return
	}

	// Calculate previous month
	prevMonth := now.AddDate(0, -1, 0)
	year := prevMonth.Year()
	month := int(prevMonth.Month())

	// Check if depreciation for previous month already exists
	periodRef := fmt.Sprintf("%%DEP-%04d-%02d", year, month)
	var existingCount int64
	ds.db.Raw(`
		SELECT COUNT(*) FROM unified_journal_ledger 
		WHERE source_type = 'DEPRECIATION' 
		AND reference LIKE ? 
		AND status = 'POSTED' 
		AND deleted_at IS NULL
	`, periodRef).Scan(&existingCount)

	if existingCount > 0 {
		// Already processed this month - skip
		return
	}

	log.Printf("📅 [DEPRECIATION-SCHEDULER] Running auto-depreciation for %04d-%02d...", year, month)

	result, err := ds.assetJournalSvc.RunMonthlyDepreciation(year, month, ds.systemUserID)
	if err != nil {
		log.Printf("❌ [DEPRECIATION-SCHEDULER] Auto-depreciation failed for %04d-%02d: %v", year, month, err)
		return
	}

	if result.ProcessedAssets > 0 {
		log.Printf("✅ [DEPRECIATION-SCHEDULER] Auto-depreciation completed for %s: %d assets processed, total=%.2f",
			result.Period, result.ProcessedAssets, result.TotalDepreciation)
	} else {
		log.Printf("ℹ️ [DEPRECIATION-SCHEDULER] No assets to depreciate for %s (%d total, %d skipped)",
			result.Period, result.TotalAssets, result.SkippedAssets)
	}

	if len(result.Errors) > 0 {
		log.Printf("⚠️ [DEPRECIATION-SCHEDULER] %d errors during depreciation:", len(result.Errors))
		for _, e := range result.Errors {
			log.Printf("   - %s", e)
		}
	}
}

// GetSchedulerStatus returns the current status of the scheduler for monitoring
func (ds *DepreciationScheduler) GetSchedulerStatus() map[string]interface{} {
	now := time.Now()
	prevMonth := now.AddDate(0, -1, 0)
	year := prevMonth.Year()
	month := int(prevMonth.Month())

	periodRef := fmt.Sprintf("%%DEP-%04d-%02d", year, month)
	var processedCount int64
	ds.db.Raw(`
		SELECT COUNT(*) FROM unified_journal_ledger 
		WHERE source_type = 'DEPRECIATION' 
		AND reference LIKE ? 
		AND status = 'POSTED' 
		AND deleted_at IS NULL
	`, periodRef).Scan(&processedCount)

	var activeAssets int64
	ds.db.Raw(`
		SELECT COUNT(*) FROM assets 
		WHERE is_active = true 
		AND depreciation_method != '' 
		AND depreciation_method IS NOT NULL
		AND deleted_at IS NULL
	`).Scan(&activeAssets)

	return map[string]interface{}{
		"scheduler_active":       true,
		"check_interval":         ds.checkInterval.String(),
		"system_user_id":         ds.systemUserID,
		"last_period_checked":    fmt.Sprintf("%04d-%02d", year, month),
		"last_period_processed":  processedCount > 0,
		"entries_for_last_period": processedCount,
		"total_depreciable_assets": activeAssets,
		"next_auto_run_window":   fmt.Sprintf("1st-5th of %s %d", now.Month().String(), now.Year()),
	}
}
