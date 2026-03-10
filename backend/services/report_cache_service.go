package services

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// ReportCacheService provides caching functionality for financial reports
type ReportCacheService struct {
	mu       sync.RWMutex
	cache    map[string]*CachedReport
	done     chan struct{} // shutdown signal for cleanup worker
	hitCount int64        // actual cache hit counter
	missCount int64       // actual cache miss counter
}

// CachedReport represents a cached report entry
type CachedReport struct {
	Data         interface{}   `json:"data"`
	Timestamp    time.Time     `json:"timestamp"`
	TTL          time.Duration `json:"ttl"`
	ReportType   string        `json:"report_type"`
	Parameters   string        `json:"parameters"`
	DataHash     string        `json:"data_hash"`
	AccessCount  int64         `json:"access_count"`
	LastAccessed time.Time     `json:"last_accessed"`
}

// CacheStats represents cache statistics
type CacheStats struct {
	TotalEntries int64     `json:"total_entries"`
	HitCount     int64     `json:"hit_count"`
	MissCount    int64     `json:"miss_count"`
	HitRate      float64   `json:"hit_rate"`
	TotalSize    int64     `json:"total_size_bytes"`
	OldestEntry  time.Time `json:"oldest_entry"`
	NewestEntry  time.Time `json:"newest_entry"`
}

// TTL configurations for different report types
var reportTTLConfig = map[string]time.Duration{
	"balance-sheet":          15 * time.Minute, // Balance sheet changes less frequently
	"profit-loss":            10 * time.Minute, // P&L needs fresher data
	"cash-flow":              10 * time.Minute, // Cash flow changes frequently
	"trial-balance":          20 * time.Minute, // Trial balance is relatively stable
	"general-ledger":         5 * time.Minute,  // General ledger needs fresh data
	"general-ledger-all":     8 * time.Minute,  // All accounts general ledger
	"sales-summary":          12 * time.Minute, // Sales data changes moderately
	"vendor-analysis":        15 * time.Minute, // Vendor analysis is relatively stable
	"journal-entry-analysis": 8 * time.Minute,  // Journal analysis needs fresher data
	"financial-dashboard":    5 * time.Minute,  // Dashboard needs very fresh data
	"default":                10 * time.Minute, // Default TTL
}

// NewReportCacheService creates a new report cache service
func NewReportCacheService() *ReportCacheService {
	service := &ReportCacheService{
		cache: make(map[string]*CachedReport),
		done:  make(chan struct{}),
	}

	// Start cleanup goroutine
	go service.startCleanupWorker()

	return service
}

// Stop gracefully shuts down the cleanup worker goroutine
func (rcs *ReportCacheService) Stop() {
	close(rcs.done)
}

// GenerateCacheKey creates a unique cache key for report parameters
func (rcs *ReportCacheService) GenerateCacheKey(reportType string, params map[string]interface{}) string {
	// Create a consistent parameter string
	paramBytes, _ := json.Marshal(params)
	paramHash := fmt.Sprintf("%x", md5.Sum(paramBytes))

	return fmt.Sprintf("%s:%s", reportType, paramHash)
}

// Get retrieves a cached report if it exists and is still valid
func (rcs *ReportCacheService) Get(cacheKey string) (interface{}, bool) {
	rcs.mu.Lock()
	defer rcs.mu.Unlock()

	cached, exists := rcs.cache[cacheKey]
	if !exists {
		rcs.missCount++
		return nil, false
	}

	// Check if cache entry has expired
	if time.Since(cached.Timestamp) > cached.TTL {
		rcs.missCount++
		return nil, false
	}

	// Update access statistics
	cached.AccessCount++
	cached.LastAccessed = time.Now()
	rcs.hitCount++

	return cached.Data, true
}

// Set stores a report in the cache
func (rcs *ReportCacheService) Set(cacheKey string, reportType string, data interface{}, customTTL ...time.Duration) error {
	rcs.mu.Lock()
	defer rcs.mu.Unlock()

	// Determine TTL
	ttl := reportTTLConfig[reportType]
	if ttl == 0 {
		ttl = reportTTLConfig["default"]
	}

	// Override with custom TTL if provided
	if len(customTTL) > 0 && customTTL[0] > 0 {
		ttl = customTTL[0]
	}

	// Generate data hash for integrity checking
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data for caching: %v", err)
	}
	dataHash := fmt.Sprintf("%x", md5.Sum(dataBytes))

	// Create cache entry
	cached := &CachedReport{
		Data:         data,
		Timestamp:    time.Now(),
		TTL:          ttl,
		ReportType:   reportType,
		DataHash:     dataHash,
		AccessCount:  0,
		LastAccessed: time.Now(),
	}

	rcs.cache[cacheKey] = cached
	return nil
}

// Delete removes a specific cache entry
func (rcs *ReportCacheService) Delete(cacheKey string) {
	rcs.mu.Lock()
	defer rcs.mu.Unlock()

	delete(rcs.cache, cacheKey)
}

// Clear removes all cache entries
func (rcs *ReportCacheService) Clear() {
	rcs.mu.Lock()
	defer rcs.mu.Unlock()

	rcs.cache = make(map[string]*CachedReport)
	rcs.hitCount = 0
	rcs.missCount = 0
}

// ClearByReportType removes all cache entries for a specific report type
func (rcs *ReportCacheService) ClearByReportType(reportType string) {
	rcs.mu.Lock()
	defer rcs.mu.Unlock()

	for key, cached := range rcs.cache {
		if cached.ReportType == reportType {
			delete(rcs.cache, key)
		}
	}
}

// GetStats returns cache statistics
func (rcs *ReportCacheService) GetStats() CacheStats {
	rcs.mu.RLock()
	defer rcs.mu.RUnlock()

	stats := CacheStats{
		TotalEntries: int64(len(rcs.cache)),
		HitCount:     rcs.hitCount,
		MissCount:    rcs.missCount,
	}

	var totalSize int64
	var oldest, newest time.Time

	for _, cached := range rcs.cache {
		// Calculate size (approximate)
		dataBytes, _ := json.Marshal(cached.Data)
		totalSize += int64(len(dataBytes))

		// Track oldest and newest entries
		if oldest.IsZero() || cached.Timestamp.Before(oldest) {
			oldest = cached.Timestamp
		}
		if newest.IsZero() || cached.Timestamp.After(newest) {
			newest = cached.Timestamp
		}
	}

	stats.TotalSize = totalSize
	stats.OldestEntry = oldest
	stats.NewestEntry = newest

	totalRequests := rcs.hitCount + rcs.missCount
	if totalRequests > 0 {
		stats.HitRate = float64(rcs.hitCount) / float64(totalRequests) * 100
	}

	return stats
}

// InvalidateStaleEntries removes expired cache entries
func (rcs *ReportCacheService) InvalidateStaleEntries() int {
	rcs.mu.Lock()
	defer rcs.mu.Unlock()

	removedCount := 0
	now := time.Now()

	for key, cached := range rcs.cache {
		if now.Sub(cached.Timestamp) > cached.TTL {
			delete(rcs.cache, key)
			removedCount++
		}
	}

	return removedCount
}

// startCleanupWorker runs a background task to clean up expired cache entries
// It stops when the done channel is closed via Stop().
func (rcs *ReportCacheService) startCleanupWorker() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-rcs.done:
			return
		case <-ticker.C:
			removedCount := rcs.InvalidateStaleEntries()
			if removedCount > 0 {
				log.Printf("Cache cleanup: removed %d expired entries", removedCount)
			}
		}
	}
}

// WarmupCache pre-loads commonly requested reports
func (rcs *ReportCacheService) WarmupCache(warmupFunc func(reportType string) (interface{}, error)) error {
	// Common reports to pre-load
	commonReports := []string{
		"balance-sheet",
		"profit-loss",
		"financial-dashboard",
		"trial-balance",
	}

	for _, reportType := range commonReports {
		data, err := warmupFunc(reportType)
		if err != nil {
			log.Printf("Failed to warmup cache for %s: %v", reportType, err)
			continue
		}

		// Generate a generic cache key for warmup
		cacheKey := rcs.GenerateCacheKey(reportType, map[string]interface{}{
			"warmup": true,
			"date":   time.Now().Format("2006-01-02"),
		})

		if err := rcs.Set(cacheKey, reportType, data); err != nil {
			log.Printf("Failed to cache warmup data for %s: %v", reportType, err)
		}
	}

	return nil
}

// GetCacheHealth returns a health assessment of the cache
func (rcs *ReportCacheService) GetCacheHealth() map[string]interface{} {
	stats := rcs.GetStats()

	health := map[string]interface{}{
		"status":        "healthy",
		"total_entries": stats.TotalEntries,
		"hit_rate":      stats.HitRate,
		"total_size_mb": float64(stats.TotalSize) / 1024 / 1024,
	}

	// Determine health status
	if stats.HitRate < 50 {
		health["status"] = "poor"
		health["issues"] = []string{"Low hit rate"}
	} else if stats.HitRate < 75 {
		health["status"] = "fair"
	}

	var warnings []string
	if stats.TotalEntries > 1000 {
		warnings = append(warnings, "High cache entry count")
	}

	if float64(stats.TotalSize)/1024/1024 > 100 { // > 100MB
		warnings = append(warnings, "High cache memory usage")
	}

	if len(warnings) > 0 {
		health["warnings"] = warnings
	}

	return health
}
