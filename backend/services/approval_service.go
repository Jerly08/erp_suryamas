package services

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/utils"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// PostApprovalCallback interface for handling post-approval business logic
type PostApprovalCallback interface {
	OnPurchaseApproved(purchaseID uint) error
}

type ApprovalService struct {
	db                   *gorm.DB
	postApprovalCallback PostApprovalCallback
}

func NewApprovalService(db *gorm.DB) *ApprovalService {
	return &ApprovalService{db: db}
}

// SetPostApprovalCallback sets the callback for post-approval processing
func (s *ApprovalService) SetPostApprovalCallback(callback PostApprovalCallback) {
	s.postApprovalCallback = callback
}

// Workflow Management

// CreateWorkflow creates a new approval workflow
func (s *ApprovalService) CreateWorkflow(req models.CreateApprovalWorkflowRequest) (*models.ApprovalWorkflow, error) {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	workflow := models.ApprovalWorkflow{
		Name:            req.Name,
		Module:          req.Module,
		MinAmount:       req.MinAmount,
		MaxAmount:       req.MaxAmount,
		RequireDirector: req.RequireDirector,
		RequireFinance:  req.RequireFinance,
		IsActive:        true,
	}

	if err := tx.Create(&workflow).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Create workflow steps
	for _, stepReq := range req.Steps {
		step := models.ApprovalStep{
			WorkflowID:   workflow.ID,
			StepOrder:    stepReq.StepOrder,
			StepName:     stepReq.StepName,
			ApproverRole: stepReq.ApproverRole,
			IsOptional:   stepReq.IsOptional,
			TimeLimit:    stepReq.TimeLimit,
		}

		if err := tx.Create(&step).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	// Load with relations
	err := s.db.Preload("Steps").First(&workflow, workflow.ID).Error
	return &workflow, err
}

// GetWorkflows returns all active workflows for a module
func (s *ApprovalService) GetWorkflows(module string) ([]models.ApprovalWorkflow, error) {
	var workflows []models.ApprovalWorkflow
	query := s.db.Preload("Steps").Where("is_active = ?", true)

	if module != "" {
		query = query.Where("module = ?", module)
	}

	err := query.Find(&workflows).Error
	return workflows, err
}

// GetWorkflowByAmount finds appropriate workflow for given amount and module
func (s *ApprovalService) GetWorkflowByAmount(module string, amount float64) (*models.ApprovalWorkflow, error) {
	var workflow models.ApprovalWorkflow
	err := s.db.Preload("Steps").Where(
		"module = ? AND is_active = ? AND min_amount <= ? AND (max_amount >= ? OR max_amount = 0)",
		module, true, amount, amount,
	).Order("min_amount DESC").First(&workflow).Error

	if err != nil {
		return nil, err
	}
	return &workflow, nil
}

// Request Management

// CreateApprovalRequest creates a new approval request
func (s *ApprovalService) CreateApprovalRequest(req models.CreateApprovalRequestDTO, requesterID uint) (*models.ApprovalRequest, error) {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Find appropriate workflow
	var module string
	switch req.EntityType {
	case models.EntityTypeSale:
		module = models.ApprovalModuleSales
	case models.EntityTypePurchase:
		module = models.ApprovalModulePurchase
	default:
		return nil, errors.New("unsupported entity type")
	}

	workflow, err := s.GetWorkflowByAmount(module, req.Amount)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("no workflow found for amount %.2f: %v", req.Amount, err)
	}

	// Generate request code
	requestCode := s.generateRequestCode(req.EntityType)

	// Create approval request
	approvalReq := models.ApprovalRequest{
		RequestCode:    requestCode,
		WorkflowID:     workflow.ID,
		RequesterID:    requesterID,
		EntityType:     req.EntityType,
		EntityID:       req.EntityID,
		Amount:         req.Amount,
		Status:         models.ApprovalStatusPending,
		Priority:       req.Priority,
		RequestTitle:   req.RequestTitle,
		RequestMessage: req.RequestMessage,
	}

	if req.Priority == "" {
		approvalReq.Priority = models.ApprovalPriorityNormal
	}

	if err := tx.Create(&approvalReq).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Create approval actions for each step
	for _, step := range workflow.Steps {
		action := models.ApprovalAction{
			RequestID: approvalReq.ID,
			StepID:    step.ID,
			Status:    models.ApprovalStatusPending,
			IsActive:  step.StepOrder == 1, // First step is active
		}

		if err := tx.Create(&action).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// Create history entry
	history := models.ApprovalHistory{
		RequestID: approvalReq.ID,
		UserID:    requesterID,
		Action:    models.ApprovalActionCreated,
		Comments:  fmt.Sprintf("Approval request created for %s", req.RequestTitle),
		Metadata:  "{}",
	}

	if err := tx.Create(&history).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	// Send notifications to approvers
	go s.notifyApprovers(&approvalReq, workflow.Steps[0])

	// Load with relations
	err = s.db.Preload("Workflow").Preload("Requester").
		Preload("ApprovalSteps.Step").Preload("ApprovalSteps.Approver").
		First(&approvalReq, approvalReq.ID).Error

	return &approvalReq, err
}

// ProcessApprovalAction processes approval or rejection
func (s *ApprovalService) ProcessApprovalAction(requestID uint, userID uint, action models.ApprovalActionDTO) error {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get approval request with relations
	var approvalReq models.ApprovalRequest
	err := tx.Preload("Workflow").Preload("ApprovalSteps.Step").
		First(&approvalReq, requestID).Error
	if err != nil {
		tx.Rollback()
		return err
	}

	// Check if request is still pending
	if approvalReq.Status != models.ApprovalStatusPending {
		tx.Rollback()
		return errors.New("approval request is no longer pending")
	}

	// Find current active step assigned to this user's role (supports parallel steps)
	var currentAction *models.ApprovalAction
	for i := range approvalReq.ApprovalSteps {
		a := &approvalReq.ApprovalSteps[i]
		if a.IsActive && a.Status == models.ApprovalStatusPending {
			if s.canUserApprove(userID, a.Step.ApproverRole) {
				currentAction = a
				break
			}
		}
	}

	if currentAction == nil {
		tx.Rollback()
		return errors.New("no active approval step found for your role")
	}

	now := time.Now()
	var newStatus string
	var historyAction string

	if action.Action == "APPROVE" {
		newStatus = models.ApprovalStatusApproved
		historyAction = models.ApprovalActionApproved
	} else if action.Action == "REJECT" {
		newStatus = models.ApprovalStatusRejected
		historyAction = models.ApprovalActionRejected
	} else {
		tx.Rollback()
		return errors.New("invalid action")
	}

	// Update current action
	currentAction.Status = newStatus
	currentAction.ApproverID = &userID
	currentAction.Comments = action.Comments
	currentAction.ActionDate = &now
	currentAction.IsActive = false

	if err := tx.Save(currentAction).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Create history entry
	history := models.ApprovalHistory{
		RequestID: requestID,
		UserID:    userID,
		Action:    historyAction,
		Comments:  action.Comments,
		Metadata:  "{}",
	}

	if err := tx.Create(&history).Error; err != nil {
		tx.Rollback()
		return err
	}

	if newStatus == models.ApprovalStatusRejected {
		// Request is rejected - update main request
		approvalReq.Status = models.ApprovalStatusRejected
		approvalReq.RejectReason = action.Comments
		approvalReq.CompletedAt = &now

		if err := tx.Save(&approvalReq).Error; err != nil {
			tx.Rollback()
			return err
		}

		// Update entity status
		if err := s.updateEntityStatus(tx, approvalReq.EntityType, approvalReq.EntityID, "REJECTED"); err != nil {
			tx.Rollback()
			return err
		}
	} else {
		// FIXED LOGIC: Check if the approving user can approve any OTHER pending steps
		// This fixes the bug where finance users skip their own step approval
		var userCanApproveOtherSteps []*models.ApprovalAction
		for i := range approvalReq.ApprovalSteps {
			step := &approvalReq.ApprovalSteps[i]
			if step.Status == models.ApprovalStatusPending && !step.IsActive {
				if s.canUserApprove(userID, step.Step.ApproverRole) {
					userCanApproveOtherSteps = append(userCanApproveOtherSteps, step)
				}
			}
		}

		// Process any steps the user can approve directly
		if len(userCanApproveOtherSteps) > 0 {
			// User can approve other steps - complete them automatically
			for _, step := range userCanApproveOtherSteps {
				step.Status = models.ApprovalStatusApproved
				step.ApproverID = &userID
				step.Comments = "Auto-approved: User has multiple role permissions"
				step.ActionDate = &now
				step.IsActive = false

				if err := tx.Save(step).Error; err != nil {
					tx.Rollback()
					return err
				}

				// Create history for auto-approval
				autoHistory := models.ApprovalHistory{
					RequestID: requestID,
					UserID:    userID,
					Action:    models.ApprovalActionApproved,
					Comments:  fmt.Sprintf("Auto-approved %s step: User has %s role permissions", step.Step.ApproverRole, step.Step.ApproverRole),
					Metadata:  "{\"auto_approved\": true}",
				}

				if err := tx.Create(&autoHistory).Error; err != nil {
					tx.Rollback()
					return err
				}
			}
		}

		// Now check for next steps to activate
		var activeNextStep *models.ApprovalStep
		var tempCurrentOrder = currentAction.Step.StepOrder
		for {
			nextStep := s.getNextStep(approvalReq.ApprovalSteps, tempCurrentOrder)
			if nextStep == nil {
				break
			}

			// Should we skip this step?
			if strings.ToLower(nextStep.ApproverRole) == "director" && nextStep.IsOptional && !approvalReq.Workflow.RequireDirector {
				// Skip this nextStep and keep looking for the one after it
				tempCurrentOrder = nextStep.StepOrder
				continue
			}

			activeNextStep = nextStep
			break
		}

		if activeNextStep != nil {
			// Check if the next step was already auto-approved above
			nextStepAlreadyApproved := false
			for i := range approvalReq.ApprovalSteps {
				if approvalReq.ApprovalSteps[i].StepID == activeNextStep.ID {
					if approvalReq.ApprovalSteps[i].Status == models.ApprovalStatusApproved {
						nextStepAlreadyApproved = true
					}
					break
				}
			}

			if !nextStepAlreadyApproved {
				// Activate next step
				for i := range approvalReq.ApprovalSteps {
					if approvalReq.ApprovalSteps[i].StepID == activeNextStep.ID {
						approvalReq.ApprovalSteps[i].IsActive = true
						if err := tx.Save(&approvalReq.ApprovalSteps[i]).Error; err != nil {
							tx.Rollback()
							return err
						}
						// Notify next approvers
						go s.notifyApprovers(&approvalReq, *activeNextStep)
						break
					}
				}
			}
		}

		// Check if ALL required steps are now completed
		allStepsCompleted := true
		for i := range approvalReq.ApprovalSteps {
			step := &approvalReq.ApprovalSteps[i]
			// Skip optional director steps unless they were specifically activated or workflow requires it
			if strings.ToLower(step.Step.ApproverRole) == "director" && step.Step.IsOptional && !step.IsActive {
				if approvalReq.Workflow.RequireDirector {
					// Workflow actively requires it! Don't skip it.
				} else {
					continue
				}
			}
			if step.Status != models.ApprovalStatusApproved {
				allStepsCompleted = false
				break
			}
		}

		if allStepsCompleted {
			// All required steps are completed - mark request as approved
			approvalReq.Status = models.ApprovalStatusApproved
			approvalReq.CompletedAt = &now

			if err := tx.Save(&approvalReq).Error; err != nil {
				tx.Rollback()
				return err
			}

			// Update entity status
			if err := s.updateEntityStatus(tx, approvalReq.EntityType, approvalReq.EntityID, "APPROVED"); err != nil {
				tx.Rollback()
				return err
			}
		} else {
			// Check if there are any pending director steps that should be activated
			var pendingDirectorStep *models.ApprovalAction
			for i := range approvalReq.ApprovalSteps {
				step := &approvalReq.ApprovalSteps[i]
				if strings.ToLower(step.Step.ApproverRole) == "director" &&
					step.Status == models.ApprovalStatusPending && !step.IsActive {
					pendingDirectorStep = step
					break
				}
			}

			if pendingDirectorStep != nil {
				// Activate the pending director step
				pendingDirectorStep.IsActive = true
				if err := tx.Save(pendingDirectorStep).Error; err != nil {
					tx.Rollback()
					return err
				}
				// Notify directors
				go s.notifyApprovers(&approvalReq, pendingDirectorStep.Step)
			}
		}
	}

	var purchaseToProcess *uint
	// Check if we need to trigger post-approval processing for purchase
	if approvalReq.Status == models.ApprovalStatusApproved && approvalReq.EntityType == models.EntityTypePurchase {
		purchaseToProcess = &approvalReq.EntityID
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	// POST-APPROVAL PROCESSING: Trigger business logic for approved purchases
	if purchaseToProcess != nil && s.postApprovalCallback != nil {
		go func(purchaseID uint) {
			if err := s.postApprovalCallback.OnPurchaseApproved(purchaseID); err != nil {
				fmt.Printf("⚠️ Post-approval processing failed for purchase %d: %v\n", purchaseID, err)
			} else {
				fmt.Printf("✅ Post-approval processing completed for purchase %d\n", purchaseID)
			}
		}(*purchaseToProcess)
	}

	// Send notification to requester
	go s.notifyRequester(&approvalReq, historyAction)

	return nil
}

// GetApprovalRequests returns approval requests with filters
func (s *ApprovalService) GetApprovalRequests(userID uint, userRole string, status string, module string, page, limit int) ([]models.ApprovalRequest, int64, error) {
	query := s.db.Model(&models.ApprovalRequest{}).
		Preload("Workflow").Preload("Requester").
		Preload("ApprovalSteps.Step").Preload("ApprovalSteps.Approver")

	// Normalize role to lower-case for consistent comparisons
	userRoleNorm := strings.ToLower(strings.TrimSpace(userRole))

	// Filter by user permissions
	if userRoleNorm != "admin" && userRoleNorm != "director" {
		// Regular users can only see requests they created or need to approve (case-insensitive role match)
		query = query.Where(
			"requester_id = ? OR id IN (SELECT DISTINCT request_id FROM approval_actions WHERE is_active = ? AND step_id IN (SELECT id FROM approval_steps WHERE LOWER(approver_role) = LOWER(?)))",
			userID, true, userRoleNorm,
		)
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if module != "" {
		query = query.Joins("JOIN approval_workflows ON approval_requests.workflow_id = approval_workflows.id").
			Where("approval_workflows.module = ?", module)
	}

	var total int64
	query.Count(&total)

	var requests []models.ApprovalRequest
	offset := (page - 1) * limit
	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&requests).Error

	return requests, total, err
}

// GetPendingApprovals returns approvals pending for a specific user
func (s *ApprovalService) GetPendingApprovals(userID uint, userRole string) ([]models.ApprovalRequest, error) {
	var requests []models.ApprovalRequest

	userRoleNorm := strings.ToLower(strings.TrimSpace(userRole))

	err := s.db.Preload("Workflow").Preload("Requester").
		Preload("ApprovalSteps.Step").Preload("ApprovalSteps.Approver").
		Where("status = ? AND id IN (SELECT request_id FROM approval_actions WHERE is_active = ? AND step_id IN (SELECT id FROM approval_steps WHERE LOWER(approver_role) = LOWER(?)))",
			models.ApprovalStatusPending, true, userRoleNorm).
		Order("created_at ASC").
		Find(&requests).Error

	return requests, err
}

// GetApprovalRequest returns a single approval request by ID
func (s *ApprovalService) GetApprovalRequest(requestID uint) (*models.ApprovalRequest, error) {
	var request models.ApprovalRequest
	err := s.db.Preload("Workflow").Preload("Requester").
		Preload("ApprovalSteps.Step").Preload("ApprovalSteps.Approver").
		First(&request, requestID).Error
	if err != nil {
		return nil, err
	}
	return &request, nil
}

// UpdateApprovalRequest updates an approval request
func (s *ApprovalService) UpdateApprovalRequest(request *models.ApprovalRequest) error {
	return s.db.Save(request).Error
}

// GetApprovalHistory returns approval history for a request
func (s *ApprovalService) GetApprovalHistory(requestID uint) ([]models.ApprovalHistory, error) {
	var history []models.ApprovalHistory
	err := s.db.Preload("User").Where("request_id = ?", requestID).
		Order("created_at ASC").Find(&history).Error
	return history, err
}

// Helper Functions

// generateRequestCode generates a unique request code
func (s *ApprovalService) generateRequestCode(entityType string) string {
	var prefix string
	switch entityType {
	case models.EntityTypeSale:
		prefix = "APP-SALE"
	case models.EntityTypePurchase:
		prefix = "APP-PUR"
	default:
		prefix = "APP-REQ"
	}

	timestamp := time.Now().Format("20060102150405")
	return fmt.Sprintf("%s-%s", prefix, timestamp)
}

// canUserApprove checks if user can approve based on role (case-insensitive)
func (s *ApprovalService) canUserApprove(userID uint, requiredRole string) bool {
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return false
	}

	// Normalize roles to lower-case for comparison
	userRole := strings.ToLower(strings.TrimSpace(user.Role))
	reqRole := strings.ToLower(strings.TrimSpace(requiredRole))

	// Admin can approve anything
	if userRole == "admin" {
		return true
	}

	// Employee can approve employee steps (when they submit the purchase)
	if userRole == "employee" && reqRole == "employee" {
		return true
	}

	// Manager can approve manager steps
	if userRole == "manager" && reqRole == "manager" {
		return true
	}

	// Director can approve director and finance steps
	if userRole == "director" && (reqRole == "director" || reqRole == "finance") {
		return true
	}

	// Finance can approve finance steps
	if userRole == "finance" && reqRole == "finance" {
		return true
	}

	return false
}

// getNextStep finds the next step in approval workflow
func (s *ApprovalService) getNextStep(actions []models.ApprovalAction, currentOrder int) *models.ApprovalStep {
	nextOrder := currentOrder + 1
	for _, action := range actions {
		if action.Step.StepOrder == nextOrder {
			return &action.Step
		}
	}
	return nil
}

// updateEntityStatus updates the status of sale/purchase entity
func (s *ApprovalService) updateEntityStatus(tx *gorm.DB, entityType string, entityID uint, approvalStatus string) error {
	var status, approvalStatusField string

	switch approvalStatus {
	case "APPROVED":
		status = "APPROVED"
		approvalStatusField = "APPROVED"
	case "REJECTED":
		status = "CANCELLED"
		approvalStatusField = "REJECTED"
	default:
		return errors.New("invalid approval status")
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":          status,
		"approval_status": approvalStatusField,
		"updated_at":      now,
	}

	if approvalStatus == "APPROVED" {
		updates["approved_at"] = now
	}

	switch entityType {
	case models.EntityTypeSale:
		return tx.Model(&models.Sale{}).Where("id = ?", entityID).Updates(updates).Error
	case models.EntityTypePurchase:
		err := tx.Model(&models.Purchase{}).Where("id = ?", entityID).Updates(updates).Error
		if err != nil {
			return err
		}

		// POST-APPROVAL PROCESSING for approved purchases
		if approvalStatus == "APPROVED" {
			// Note: We'll trigger post-approval processing after transaction commits
			// This is handled by the calling function via callback
			fmt.Printf("✅ Purchase %d approved - will trigger post-approval processing\n", entityID)
		}

		return nil
	default:
		return errors.New("unsupported entity type")
	}
}

// notifyApprovers sends notifications to approvers
func (s *ApprovalService) notifyApprovers(request *models.ApprovalRequest, step models.ApprovalStep) {
	// Get users with the required role (case-insensitive)
	var users []models.User
	s.db.Where("LOWER(role) = LOWER(?) AND is_active = ?", step.ApproverRole, true).Find(&users)

	for _, user := range users {
		// Check for duplicate notifications
		if s.isDuplicateApprovalNotification(user.ID, request.ID, models.NotificationTypeApprovalPending) {
			continue // Skip creating duplicate
		}

		// Get the actual purchase to show correct amount
		var actualAmount float64 = request.Amount
		if request.EntityType == models.EntityTypePurchase {
			var purchase models.Purchase
			if err := s.db.First(&purchase, request.EntityID).Error; err == nil {
				actualAmount = purchase.TotalAmount // Use TotalAmount instead of ApprovalBaseAmount
			}
		}

		notification := models.Notification{
			UserID:   user.ID,
			Type:     models.NotificationTypeApprovalPending,
			Title:    fmt.Sprintf("Approval Required: %s", request.RequestTitle),
			Message:  fmt.Sprintf("You have a pending approval request for %s (Amount: %s)", request.RequestTitle, utils.FormatRupiahWithoutDecimals(actualAmount)),
			Priority: request.Priority,
			Data:     s.createNotificationData(request),
		}

		s.db.Create(&notification)
	}
}

// notifyRequester sends notification back to requester
func (s *ApprovalService) notifyRequester(request *models.ApprovalRequest, action string) {
	var notificationType, title, message string

	switch action {
	case models.ApprovalActionApproved:
		if request.Status == models.ApprovalStatusApproved {
			notificationType = models.NotificationTypeApprovalApproved
			title = "Request Approved"
			message = fmt.Sprintf("Your request '%s' has been approved", request.RequestTitle)
		} else {
			return // Don't notify on intermediate approvals
		}
	case models.ApprovalActionRejected:
		notificationType = models.NotificationTypeApprovalRejected
		title = "Request Rejected"
		message = fmt.Sprintf("Your request '%s' has been rejected", request.RequestTitle)
	default:
		return
	}

	// Check for duplicate notifications
	if s.isDuplicateApprovalNotification(request.RequesterID, request.ID, notificationType) {
		return // Skip creating duplicate
	}

	notification := models.Notification{
		UserID:   request.RequesterID,
		Type:     notificationType,
		Title:    title,
		Message:  message,
		Priority: request.Priority,
		Data:     s.createNotificationData(request),
	}

	s.db.Create(&notification)
}

// isDuplicateApprovalNotification checks if similar approval notification already exists
func (s *ApprovalService) isDuplicateApprovalNotification(userID uint, requestID uint, notificationType string) bool {
	// Check for existing notification with same request_id, user_id, and type within last 1 hour
	var count int64
	oneHourAgo := time.Now().Add(-1 * time.Hour)

	err := s.db.Model(&models.Notification{}).
		Where("user_id = ? AND type = ? AND created_at >= ? AND data::json->>'request_id' = ?",
			userID, notificationType, oneHourAgo, fmt.Sprintf("%d", requestID)).
		Count(&count).Error

	if err != nil {
		return false // If error, allow creation
	}

	return count > 0
}

// createNotificationData creates JSON data for notifications
func (s *ApprovalService) createNotificationData(request *models.ApprovalRequest) string {
	// Get the actual purchase amount for proper display
	var actualAmount float64 = request.Amount
	if request.EntityType == models.EntityTypePurchase {
		var purchase models.Purchase
		if err := s.db.First(&purchase, request.EntityID).Error; err == nil {
			actualAmount = purchase.TotalAmount // Use TotalAmount instead of ApprovalBaseAmount
		}
	}

	data := map[string]interface{}{
		"request_id":    request.ID,
		"entity_type":   request.EntityType,
		"entity_id":     request.EntityID,
		"amount":        actualAmount, // Use actualAmount instead of request.Amount
		"status":        request.Status,
		"purchase_code": "", // Will be filled if this is a purchase
	}

	// Add purchase code for better identification
	if request.EntityType == models.EntityTypePurchase {
		var purchase models.Purchase
		if err := s.db.Select("code").First(&purchase, request.EntityID).Error; err == nil {
			data["purchase_code"] = purchase.Code
		}
	}

	jsonData, _ := json.Marshal(data)
	return string(jsonData)
}

// EscalateToDirector escalates an approval request to Director
func (s *ApprovalService) EscalateToDirector(requestID uint, escalatedByUserID uint, reason string) error {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get the request with all related data
	var request models.ApprovalRequest
	err := tx.Preload("Workflow.Steps").Preload("ApprovalSteps.Step").
		First(&request, requestID).Error
	if err != nil {
		tx.Rollback()
		return err
	}

	// Safety check: Don't escalate if request is already approved or rejected
	if request.Status != models.ApprovalStatusPending {
		tx.Rollback()
		return fmt.Errorf("cannot escalate request with status %s - only pending requests can be escalated", request.Status)
	}

	// Check if there's a director step in the workflow
	var directorStep *models.ApprovalStep
	for _, step := range request.Workflow.Steps {
		if strings.ToLower(step.ApproverRole) == "director" {
			directorStep = &step
			break
		}
	}

	if directorStep == nil {
		// Create a new director step dynamically
		directorStep = &models.ApprovalStep{
			WorkflowID:   request.WorkflowID,
			StepOrder:    999, // High order to ensure it's last
			StepName:     "Director Approval (Escalated)",
			ApproverRole: "director",
			IsOptional:   false,
		}
		if err := tx.Create(directorStep).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	// Reload the director step to ensure we have the correct ID
	if err := tx.First(directorStep, directorStep.ID).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to reload director step: %v", err)
	}

	// Check if director step action already exists
	var directorAction models.ApprovalAction
	err = tx.Where("request_id = ? AND step_id = ?", requestID, directorStep.ID).
		First(&directorAction).Error

	if err == gorm.ErrRecordNotFound {
		// Create new action for director
		directorAction = models.ApprovalAction{
			RequestID: requestID,
			StepID:    directorStep.ID,
			Status:    models.ApprovalStatusPending,
			IsActive:  true,
		}
		if err := tx.Create(&directorAction).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create director action: %v", err)
		}
	} else if err == nil {
		// Update existing action
		directorAction.Status = models.ApprovalStatusPending
		directorAction.IsActive = true
		directorAction.ApproverID = nil
		directorAction.ActionDate = nil
		directorAction.Comments = ""
		if err := tx.Save(&directorAction).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to update director action: %v", err)
		}
	} else {
		tx.Rollback()
		return fmt.Errorf("failed to find director action: %v", err)
	}

	// Important: DO NOT deactivate all other steps immediately
	// Keep the current step active until it's properly processed
	// Only deactivate other steps if they are not the currently active finance step
	if err := tx.Model(&models.ApprovalAction{}).
		Where("request_id = ? AND id != ? AND is_active = ? AND status != ?",
			requestID, directorAction.ID, false, models.ApprovalStatusPending).
		Update("is_active", false).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Create escalation history
	history := models.ApprovalHistory{
		RequestID: requestID,
		UserID:    escalatedByUserID,
		Action:    "ESCALATED_TO_DIRECTOR",
		Comments:  reason,
		Metadata:  "{}",
	}
	if err := tx.Create(&history).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Update request priority to high
	if err := tx.Model(&request).Update("priority", models.ApprovalPriorityHigh).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	// Send notification to directors
	go s.notifyDirectors(&request, reason)

	return nil
}

// CreateApprovalHistory creates a new approval history record
func (s *ApprovalService) CreateApprovalHistory(requestID uint, userID uint, action string, comments string) error {
	history := models.ApprovalHistory{
		RequestID: requestID,
		UserID:    userID,
		Action:    action,
		Comments:  comments,
		Metadata:  "{}",
	}

	err := s.db.Create(&history).Error
	if err != nil {
		return err
	}

	return nil
}

// CreateMinimalApprovalRequestForRejection creates a minimal approval request for rejection tracking without workflow dependency
func (s *ApprovalService) CreateMinimalApprovalRequestForRejection(entityType string, entityID uint, amount float64, title string, userID uint, purchase interface{}) error {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Generate request code
	requestCode := s.generateRequestCode(entityType)

	// Create minimal approval request without workflow - use a default/dummy workflow
	// First, try to get any active workflow for the module as a fallback
	var workflowID uint = 1 // Default fallback - assume workflow ID 1 exists

	// Try to find any active workflow for the module
	var workflow models.ApprovalWorkflow
	module := "PURCHASE"
	if entityType == models.EntityTypeSale {
		module = "SALES"
	}

	err := s.db.Where("module = ? AND is_active = ?", module, true).First(&workflow).Error
	if err == nil {
		workflowID = workflow.ID
	} else {
		// If no workflow found, try to find any active workflow
		err = s.db.Where("is_active = ?", true).First(&workflow).Error
		if err == nil {
			workflowID = workflow.ID
		} else {
			// No workflow found at all - this is a critical error
			tx.Rollback()
			return fmt.Errorf("no active workflows found for rejection tracking: %v", err)
		}
	}

	approvalReq := models.ApprovalRequest{
		RequestCode:    requestCode,
		WorkflowID:     workflowID, // Use existing workflow for DB constraint
		RequesterID:    userID,
		EntityType:     entityType,
		EntityID:       entityID,
		Amount:         amount,
		Status:         models.ApprovalStatusPending, // Will be set to rejected later
		Priority:       models.ApprovalPriorityNormal,
		RequestTitle:   title,
		RequestMessage: fmt.Sprintf("Minimal approval request for %s tracking (using workflow %d)", strings.ToLower(entityType), workflowID),
	}

	if err := tx.Create(&approvalReq).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Create initial history entry
	history := models.ApprovalHistory{
		RequestID: approvalReq.ID,
		UserID:    userID,
		Action:    models.ApprovalActionCreated,
		Comments:  fmt.Sprintf("Minimal approval request created for %s tracking", strings.ToLower(entityType)),
		Metadata:  "{}",
	}

	if err := tx.Create(&history).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	// Update the purchase with the approval request ID
	if p, ok := purchase.(*models.Purchase); ok {
		p.ApprovalRequestID = &approvalReq.ID
	}

	return nil
}

// notifyDirectors sends high-priority notifications to directors
func (s *ApprovalService) notifyDirectors(request *models.ApprovalRequest, escalationReason string) {
	var directors []models.User
	err := s.db.Where("LOWER(role) = LOWER(?) AND is_active = ?", "director", true).Find(&directors).Error
	if err != nil {
		fmt.Printf("Error finding directors for notification: %v\n", err)
		return
	}

	fmt.Printf("Found %d active directors to notify for request %d\n", len(directors), request.ID)

	// Get the actual purchase to show correct amount
	var actualAmount float64 = request.Amount
	if request.EntityType == models.EntityTypePurchase {
		var purchase models.Purchase
		if err := s.db.First(&purchase, request.EntityID).Error; err == nil {
			actualAmount = purchase.TotalAmount // Use TotalAmount instead of ApprovalBaseAmount
		}
	}

	for _, director := range directors {
		notification := models.Notification{
			UserID:   director.ID,
			Type:     models.NotificationTypeApprovalPending,
			Title:    fmt.Sprintf("URGENT: Director Approval Required - %s", request.RequestTitle),
			Message:  fmt.Sprintf("Finance has escalated this request: %s. Reason: %s (Amount: %s)", request.RequestTitle, escalationReason, utils.FormatRupiahWithoutDecimals(actualAmount)),
			Priority: models.ApprovalPriorityUrgent,
			Data:     s.createNotificationData(request),
		}

		err := s.db.Create(&notification).Error
		if err != nil {
			fmt.Printf("Failed to create notification for director %d: %v\n", director.ID, err)
		} else {
			fmt.Printf("Created notification for director %s (ID: %d)\n", director.Username, director.ID)
		}
	}
}
