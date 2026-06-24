package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"
	"app-sistem-akuntansi/utils"

	"github.com/gin-gonic/gin"
)

type PurchaseApprovalHandler struct {
	purchaseService *services.PurchaseService
	approvalService *services.ApprovalService
}

func NewPurchaseApprovalHandler(purchaseService *services.PurchaseService, approvalService *services.ApprovalService) *PurchaseApprovalHandler {
	return &PurchaseApprovalHandler{
		purchaseService: purchaseService,
		approvalService: approvalService,
	}
}

// SubmitPurchaseForApproval submits a purchase for approval
// POST /api/purchases/:id/submit-approval
func (h *PurchaseApprovalHandler) SubmitPurchaseForApproval(c *gin.Context) {
	userID, err := utils.GetUserIDFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or missing token"})
		return
	}

	purchaseID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase ID"})
		return
	}

	err = h.purchaseService.SubmitForApproval(uint(purchaseID), userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Purchase submitted for approval successfully",
		"purchase_id": purchaseID,
	})
}

// ApprovePurchase approves a purchase via approval workflow
// POST /api/purchases/:id/approve
func (h *PurchaseApprovalHandler) ApprovePurchase(c *gin.Context) {
	userID, err := utils.GetUserIDFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or missing token"})
		return
	}

	purchaseID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase ID"})
		return
	}

	var request struct {
		Comments           string `json:"comments"`
		EscalateToDirector bool   `json:"escalate_to_director"`
	}
	// Body optional; ignore bind error when empty
	_ = c.ShouldBindJSON(&request)

	// Ensure purchase has an approval request
	purchase, err := h.purchaseService.GetPurchaseByID(uint(purchaseID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Purchase not found"})
		return
	}

	// AUTO-SUBMIT: If purchase is DRAFT and user is finance/admin/director, auto-submit for approval first
	if purchase.Status == models.PurchaseStatusDraft {
		userRole, _ := utils.GetUserRoleFromToken(c)
		roleNorm := strings.ToLower(strings.TrimSpace(userRole))
		if models.IsFinanceManagerRole(roleNorm) || roleNorm == models.RoleAdmin || roleNorm == models.RoleDirector {
			// Auto-submit the purchase for approval
			if err := h.purchaseService.SubmitForApproval(uint(purchaseID), userID); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": fmt.Sprintf("Failed to auto-submit purchase for approval: %v", err),
				})
				return
			}
			// Reload purchase after submission
			purchase, err = h.purchaseService.GetPurchaseByID(uint(purchaseID))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reload purchase after submission"})
				return
			}
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":           "Cannot approve DRAFT purchase - please submit for approval first",
				"current_status":  purchase.Status,
				"required_action": "Use 'Submit for Approval' endpoint first",
			})
			return
		}
	}

	if purchase.ApprovalRequestID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No approval request associated with this purchase"})
		return
	}

	// Get user role to check if finance
	userRole, _ := utils.GetUserRoleFromToken(c)

	// Purchase approval must go through PurchaseService so optional Director
	// escalation and Accounting routing stay consistent with purchase state.
	result, err := h.purchaseService.ProcessPurchaseApprovalWithEscalation(
		uint(purchaseID),
		true,
		userID,
		userRole,
		request.Comments,
		models.IsFinanceManagerRole(userRole) && request.EscalateToDirector,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// RejectPurchase rejects a purchase via approval workflow
// POST /api/purchases/:id/reject
func (h *PurchaseApprovalHandler) RejectPurchase(c *gin.Context) {
	userID, err := utils.GetUserIDFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or missing token"})
		return
	}

	purchaseID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase ID"})
		return
	}

	var request struct {
		Comments string `json:"comments" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user role
	userRole, _ := utils.GetUserRoleFromToken(c)

	// Use ProcessPurchaseApprovalWithEscalation for consistent handling
	result, err := h.purchaseService.ProcessPurchaseApprovalWithEscalation(
		uint(purchaseID),
		false, // rejected
		userID,
		strings.ToLower(userRole),
		request.Comments,
		false, // no escalation for rejection
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetPurchasesForApproval gets purchases pending approval for current user
// GET /api/purchases/pending-approval
func (h *PurchaseApprovalHandler) GetPurchasesForApproval(c *gin.Context) {
	userID, err := utils.GetUserIDFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or missing token"})
		return
	}

	// Get user role to determine what approvals they can see
	userRole, err := utils.GetUserRoleFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or missing token"})
		return
	}

	filter := models.PurchaseFilter{
		ApprovalStatus: models.PurchaseApprovalPending,
		Page:           1,
		Limit:          50,
	}

	// Parse query parameters
	if page := c.Query("page"); page != "" {
		if p, err := strconv.Atoi(page); err == nil && p > 0 {
			filter.Page = p
		}
	}

	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 && l <= 100 {
			filter.Limit = l
		}
	}

	purchases, err := h.purchaseService.GetPurchases(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	finalApprovalFilter := models.PurchaseFilter{
		Status: models.PurchaseStatusPendingFinalApproval,
		Page:   1,
		Limit:  filter.Limit,
	}
	finalApprovalPurchases, err := h.purchaseService.GetPurchases(finalApprovalFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Filter based on user's approval authority
	candidates := append(purchases.Data, finalApprovalPurchases.Data...)
	filteredPurchases := h.filterPurchasesByApprovalAuthority(candidates, userRole, userID)

	c.JSON(http.StatusOK, gin.H{
		"purchases": filteredPurchases,
		"total":     len(filteredPurchases),
		"page":      filter.Page,
		"limit":     filter.Limit,
	})
}

// GetApprovalHistory gets approval history for a purchase
// GET /api/purchases/:id/approval-history
func (h *PurchaseApprovalHandler) GetApprovalHistory(c *gin.Context) {
	purchaseID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase ID"})
		return
	}

	purchase, err := h.purchaseService.GetPurchaseByID(uint(purchaseID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Purchase not found"})
		return
	}

	if purchase.ApprovalRequestID == nil {
		c.JSON(http.StatusOK, gin.H{
			"approval_history": []interface{}{},
			"message":          "No approval required for this purchase",
			"status":           "NOT_STARTED",
			"debug_info": gin.H{
				"purchase_status":     purchase.Status,
				"approval_status":     purchase.ApprovalStatus,
				"approval_request_id": purchase.ApprovalRequestID,
			},
		})
		return
	}

	// Get approval history from approval service
	history, err := h.approvalService.GetApprovalHistory(*purchase.ApprovalRequestID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get approval request details for additional context
	approvalRequest, err := h.approvalService.GetApprovalRequest(*purchase.ApprovalRequestID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Enhanced response with detailed information
	response := gin.H{
		"purchase_id":      purchaseID,
		"approval_history": history,
		"approval_status":  approvalRequest.Status,
		"workflow_name":    approvalRequest.Workflow.Name,
		"current_step":     h.getCurrentApprovalStep(approvalRequest),
	}

	// Add rejection reason if rejected
	if approvalRequest.Status == models.ApprovalStatusRejected && approvalRequest.RejectReason != "" {
		response["rejection_reason"] = approvalRequest.RejectReason
	}

	// Add summary of approval/rejection comments
	if len(history) > 0 {
		response["summary"] = h.buildApprovalSummary(history)
	}

	c.JSON(http.StatusOK, response)
}

// GetApprovalWorkflows gets available approval workflows
// GET /api/approval-workflows
func (h *PurchaseApprovalHandler) GetApprovalWorkflows(c *gin.Context) {
	module := c.Query("module")
	if module == "" {
		module = models.ApprovalModulePurchase
	}

	workflows, err := h.approvalService.GetWorkflows(module)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"workflows": workflows,
	})
}

// CreateApprovalWorkflow creates a new approval workflow
// POST /api/approval-workflows
func (h *PurchaseApprovalHandler) CreateApprovalWorkflow(c *gin.Context) {
	_, err := utils.GetUserIDFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or missing token"})
		return
	}
	userRole, err := utils.GetUserRoleFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or missing token"})
		return
	}

	// Only admin can create workflows
	if userRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admin can create approval workflows"})
		return
	}

	var request models.CreateApprovalWorkflowRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	workflow, err := h.approvalService.CreateWorkflow(request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Approval workflow created successfully",
		"workflow": workflow,
	})
}

// Helper function to filter purchases based on active approval step's approver_role
// Single source of truth: who can approve is determined by the active ApprovalWorkflow step.
func (h *PurchaseApprovalHandler) filterPurchasesByApprovalAuthority(purchases []models.Purchase, userRole string, userID uint) []models.Purchase {
	// Admin is not part of the purchase approval lane. Pending queues should stay with the
	// currently assigned finance manager/director workflow step.
	if strings.ToLower(strings.TrimSpace(userRole)) == "admin" {
		return nil
	}

	var filtered []models.Purchase
	for _, p := range purchases {
		if strings.EqualFold(strings.TrimSpace(p.Status), models.PurchaseStatusPendingFinalApproval) {
			if models.IsFinanceManagerRole(userRole) {
				filtered = append(filtered, p)
			}
			continue
		}

		if p.ApprovalRequestID == nil || p.ApprovalRequest == nil {
			continue
		}
		// Prefer local check using preloaded approval steps
		allowedByLocal := false
		for _, act := range p.ApprovalRequest.ApprovalSteps {
			if act.IsActive && act.Status == models.ApprovalStatusPending {
				if models.WorkflowRoleMatchesUserRole(act.Step.ApproverRole, userRole) {
					allowedByLocal = true
					break
				}
			}
		}
		if allowedByLocal {
			filtered = append(filtered, p)
			continue
		}
	}

	// Fallback: rely on approval service query (in case steps were not loaded for some records)
	if h.approvalService == nil {
		return filtered
	}
	pendingReqs, err := h.approvalService.GetPendingApprovals(userID, userRole)
	if err != nil {
		return filtered
	}
	reqAllowed := make(map[uint]struct{}, len(pendingReqs))
	for _, req := range pendingReqs {
		reqAllowed[req.ID] = struct{}{}
	}
	for _, p := range purchases {
		if p.ApprovalRequestID != nil {
			if _, ok := reqAllowed[*p.ApprovalRequestID]; ok {
				// Avoid duplicates
				already := false
				for _, e := range filtered {
					if e.ID == p.ID {
						already = true
						break
					}
				}
				if !already {
					filtered = append(filtered, p)
				}
			}
		}
	}
	return filtered
}

// GetApprovalStats gets approval statistics
// GET /api/purchases/approval-stats
func (h *PurchaseApprovalHandler) GetApprovalStats(c *gin.Context) {
	userID, err := utils.GetUserIDFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or missing token"})
		return
	}
	userRole, err := utils.GetUserRoleFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or missing token"})
		return
	}

	// Only purchase approval workflow roles can see stats
	if models.NormalizeRole(userRole) != models.RoleAdmin &&
		!models.IsFinanceStaffRole(userRole) &&
		!models.IsFinanceManagerRole(userRole) &&
		models.NormalizeRole(userRole) != models.RoleDirector {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Base stats structure
	stats := map[string]interface{}{
		"pending_approvals":    0,
		"approved_this_month":  0,
		"rejected_this_month":  0,
		"total_amount_pending": 0.0,
	}

	// Pull all pending purchases, then filter by user's approval authority
	pendingFilter := models.PurchaseFilter{
		ApprovalStatus: models.PurchaseApprovalPending,
		Page:           1,
		Limit:          1000,
	}

	pendingPurchases, err := h.purchaseService.GetPurchases(pendingFilter)
	if err == nil {
		finalApprovalFilter := models.PurchaseFilter{
			Status: models.PurchaseStatusPendingFinalApproval,
			Page:   1,
			Limit:  1000,
		}
		candidates := pendingPurchases.Data
		if finalApprovalPurchases, finalErr := h.purchaseService.GetPurchases(finalApprovalFilter); finalErr == nil {
			candidates = append(candidates, finalApprovalPurchases.Data...)
		}
		roleFiltered := h.filterPurchasesByApprovalAuthority(candidates, userRole, userID)
		stats["pending_approvals"] = len(roleFiltered)
		// Calculate total pending amount for items the user can act on
		totalPending := 0.0
		for _, purchase := range roleFiltered {
			totalPending += purchase.TotalAmount
		}
		stats["total_amount_pending"] = totalPending
	}

	c.JSON(http.StatusOK, stats)
}

// getCurrentApprovalStep returns the current active approval step
func (h *PurchaseApprovalHandler) getCurrentApprovalStep(approvalRequest *models.ApprovalRequest) map[string]interface{} {
	for _, step := range approvalRequest.ApprovalSteps {
		if step.IsActive && step.Status == models.ApprovalStatusPending {
			return map[string]interface{}{
				"step_name":     step.Step.StepName,
				"approver_role": step.Step.ApproverRole,
				"step_order":    step.Step.StepOrder,
				"is_active":     step.IsActive,
				"status":        step.Status,
			}
		}
	}
	return map[string]interface{}{
		"step_name":     "Completed",
		"approver_role": "",
		"step_order":    0,
		"is_active":     false,
		"status":        approvalRequest.Status,
	}
}

// buildApprovalSummary creates a summary of approval/rejection comments
func (h *PurchaseApprovalHandler) buildApprovalSummary(history []models.ApprovalHistory) map[string]interface{} {
	summary := map[string]interface{}{
		"total_actions":    len(history),
		"approvals":        []map[string]interface{}{},
		"rejections":       []map[string]interface{}{},
		"latest_comment":   "",
		"latest_action":    "",
		"latest_user":      "",
		"latest_timestamp": nil,
	}

	var approvals []map[string]interface{}
	var rejections []map[string]interface{}

	for _, h := range history {
		actionData := map[string]interface{}{
			"action":    h.Action,
			"comments":  h.Comments,
			"user":      h.User.Username,
			"user_role": h.User.Role,
			"timestamp": h.CreatedAt,
		}

		switch h.Action {
		case models.ApprovalActionApproved:
			approvals = append(approvals, actionData)
		case models.ApprovalActionRejected:
			rejections = append(rejections, actionData)
		}

		// Track latest action (last item in history)
		if h.CreatedAt.After(getTimeFromInterface(summary["latest_timestamp"])) || summary["latest_timestamp"] == nil {
			summary["latest_comment"] = h.Comments
			summary["latest_action"] = h.Action
			summary["latest_user"] = h.User.Username
			summary["latest_timestamp"] = h.CreatedAt
		}
	}

	summary["approvals"] = approvals
	summary["rejections"] = rejections
	summary["approval_count"] = len(approvals)
	summary["rejection_count"] = len(rejections)

	return summary
}

// getTimeFromInterface safely extracts time from interface{}
func getTimeFromInterface(val interface{}) time.Time {
	if val == nil {
		return time.Time{}
	}
	if t, ok := val.(time.Time); ok {
		return t
	}
	return time.Time{}
}
