package services

import (
	"app-sistem-akuntansi/models"
	"gorm.io/gorm"
	"log"
	"time"
)

type SessionCleanupService struct {
	db *gorm.DB
}

func NewSessionCleanupService(db *gorm.DB) *SessionCleanupService {
	return &SessionCleanupService{db: db}
}

// GetDB returns the database instance for external access
func (scs *SessionCleanupService) GetDB() *gorm.DB {
	return scs.db
}

// StartCleanupWorker starts the background cleanup worker
func (scs *SessionCleanupService) StartCleanupWorker() {
	// Run cleanup every 6 hours instead of every hour to reduce interference
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()

	log.Println("🧹 Session cleanup worker started - running every 6 hours")

	for range ticker.C {
		scs.CleanupExpiredSessions()
	}
}

// CleanupExpiredSessions removes expired sessions and tokens
func (scs *SessionCleanupService) CleanupExpiredSessions() error {
	now := time.Now()

	// 1. Cleanup expired sessions - only mark as inactive, don't delete yet
	var expiredSessions []models.UserSession
	if err := scs.db.Where("expires_at < ? AND is_active = ?", now, true).Find(&expiredSessions).Error; err != nil {
		log.Printf("Error finding expired sessions: %v", err)
		return err
	}

	if len(expiredSessions) > 0 {
		// Mark expired sessions as inactive
		if err := scs.db.Model(&models.UserSession{}).Where("expires_at < ? AND is_active = ?", now, true).Update("is_active", false).Error; err != nil {
			log.Printf("Error deactivating expired sessions: %v", err)
			return err
		}

		log.Printf("🧹 Deactivated %d expired sessions", len(expiredSessions))
	}

	// 2. Limit active sessions per user to prevent accumulation
	// Keep only the 10 most recent active sessions per user (increased from 5)
	var users []struct {
		UserID uint
	}
	scs.db.Model(&models.UserSession{}).Select("DISTINCT user_id").Find(&users)

	for _, user := range users {
		var userSessions []models.UserSession
		scs.db.Where("user_id = ? AND is_active = ?", user.UserID, true).
			Order("created_at DESC").
			Find(&userSessions)

		if len(userSessions) > 10 {
			// Deactivate old sessions beyond the 10 most recent
			oldSessions := userSessions[10:]
			for _, session := range oldSessions {
				scs.db.Model(&session).Update("is_active", false)
			}
			log.Printf("🧹 Deactivated %d old sessions for user %d", len(oldSessions), user.UserID)
		}
	}

	// 3. Cleanup expired refresh tokens
	if err := scs.db.Where("expires_at < ?", now).Delete(&models.RefreshToken{}).Error; err != nil {
		log.Printf("Error cleaning up expired refresh tokens: %v", err)
		return err
	}

	// 4. Cleanup expired blacklisted tokens
	if err := scs.db.Where("expires_at < ?", now).Delete(&models.BlacklistedToken{}).Error; err != nil {
		log.Printf("Error cleaning up expired blacklisted tokens: %v", err)
		return err
	}

	// 5. Cleanup old inactive sessions (older than 7 days)
	oldDate := now.AddDate(0, 0, -7)
	if err := scs.db.Where("is_active = ? AND expires_at < ?", false, oldDate).Delete(&models.UserSession{}).Error; err != nil {
		log.Printf("Error cleaning up old inactive sessions: %v", err)
		return err
	}

	// 6. Cleanup old security incidents (keep only last 100)
	var incidentCount int64
	scs.db.Model(&models.SecurityIncident{}).Count(&incidentCount)
	if incidentCount > 100 {
		// Delete old security incidents
		scs.db.Where("id NOT IN (SELECT id FROM security_incidents ORDER BY created_at DESC LIMIT 100)").Delete(&models.SecurityIncident{})
		log.Printf("🧹 Cleaned up old security incidents")
	}

	return nil
}

// GetSessionStats returns statistics about sessions
func (scs *SessionCleanupService) GetSessionStats() map[string]interface{} {
	var totalSessions int64
	var activeSessions int64
	var expiredSessions int64

	scs.db.Model(&models.UserSession{}).Count(&totalSessions)
	scs.db.Model(&models.UserSession{}).Where("is_active = ?", true).Count(&activeSessions)
	scs.db.Model(&models.UserSession{}).Where("expires_at < ? AND is_active = ?", time.Now(), true).Count(&expiredSessions)

	return map[string]interface{}{
		"total_sessions":   totalSessions,
		"active_sessions":  activeSessions,
		"expired_sessions": expiredSessions,
		"cleanup_needed":   expiredSessions > 0,
	}
}

// ForceCleanup manually triggers cleanup
func (scs *SessionCleanupService) ForceCleanup() error {
	log.Println("🧹 Manual cleanup triggered")
	return scs.CleanupExpiredSessions()
}
