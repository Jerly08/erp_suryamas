package services

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"gorm.io/gorm"
	"time"
)

type EscalationService struct {
	db              *gorm.DB
	approvalService *ApprovalService
}

func NewEscalationService(db *gorm.DB, approvalService *ApprovalService) *EscalationService {
	return &EscalationService{
		db:              db,
		approvalService: approvalService,
	}
}

// CheckAndEscalateOverdueApprovals automatically escalates overdue approval requests
func (s *EscalationService) CheckAndEscalateOverdueApprovals() error {
	// Find all pending approval actions that are overdue
	var overdueActions []models.ApprovalAction

	err := s.db.Preload("Request").Preload("Step").
		Where("is_active = ? AND status = ? AND created_at < ?",
			true, models.ApprovalStatusPending, time.Now().Add(-24*time.Hour)).
		Find(&overdueActions).Error

	if err != nil {
		return fmt.Errorf("failed to fetch overdue actions: %v", err)
	}

	for _, action := range overdueActions {
		err := s.escalateApproval(&action)
		if err != nil {
			// Log error but continue with other escalations
			fmt.Printf("Failed to escalate approval %d: %v\n", action.ID, err)
		}
	}

	return nil
}

// escalateApproval escalates a single overdue approval
func (s *EscalationService) escalateApproval(action *models.ApprovalAction) error {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Mark current step as escalated/skipped
	action.Status = models.ApprovalActionSkipped
	action.Comments = "Auto-escalated due to timeout"
	now := time.Now()
	action.ActionDate = &now
	action.IsActive = false

	if err := tx.Save(action).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Create escalation history
	history := models.ApprovalHistory{
		RequestID: action.RequestID,
		UserID:    1, // System user ID
		Action:    "ESCALATED",
		Comments:  fmt.Sprintf("Step '%s' escalated due to timeout after 24 hours", action.Step.StepName),
	}

	if err := tx.Create(&history).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Find next step or escalation target
	var request models.ApprovalRequest
	err := tx.Preload("ApprovalSteps.Step").First(&request, action.RequestID).Error
	if err != nil {
		tx.Rollback()
		return err
	}

	// Try to escalate to Director or skip to next step
	escalated := false
	for i := range request.ApprovalSteps {
		step := &request.ApprovalSteps[i]
		if step.Step.ApproverRole == "director" && step.Status == models.ApprovalStatusPending {
			step.IsActive = true
			if err := tx.Save(step).Error; err != nil {
				tx.Rollback()
				return err
			}
			escalated = true
			break
		}
	}

	// If no director step available, approve automatically for low-priority requests
	if !escalated && request.Priority == models.ApprovalPriorityLow {
		request.Status = models.ApprovalStatusApproved
		now := time.Now()
		request.CompletedAt = &now

		if err := tx.Save(&request).Error; err != nil {
			tx.Rollback()
			return err
		}

		// Update entity status
		err = s.updateEntityStatusEscalation(tx, request.EntityType, request.EntityID, "APPROVED")
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	return nil
}

// updateEntityStatusEscalation updates entity status for escalated approvals
func (s *EscalationService) updateEntityStatusEscalation(tx *gorm.DB, entityType string, entityID uint, status string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":          status,
		"approval_status": status,
		"updated_at":      now,
	}

	if status == "APPROVED" {
		updates["approved_at"] = now
	}

	switch entityType {
	case models.EntityTypePurchase:
		return tx.Model(&models.Purchase{}).Where("id = ?", entityID).Updates(updates).Error
	case models.EntityTypeSale:
		return tx.Model(&models.Sale{}).Where("id = ?", entityID).Updates(updates).Error
	default:
		return fmt.Errorf("unsupported entity type: %s", entityType)
	}
}

// SetupEscalationScheduler sets up a scheduler to run escalation checks
func (s *EscalationService) SetupEscalationScheduler() {
	// Run escalation check every hour
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			if err := s.CheckAndEscalateOverdueApprovals(); err != nil {
				fmt.Printf("Escalation check failed: %v\n", err)
			}
		}
	}()
}
