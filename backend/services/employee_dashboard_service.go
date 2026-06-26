package services

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"gorm.io/gorm"
	"strings"
	"time"
)

// EmployeeDashboardService provides employee-specific dashboard functionality
type EmployeeDashboardService struct {
	DB *gorm.DB
}

// NewEmployeeDashboardService creates a new employee dashboard service
func NewEmployeeDashboardService(db *gorm.DB) *EmployeeDashboardService {
	return &EmployeeDashboardService{DB: db}
}

// GetEmployeeDashboardData returns employee-specific dashboard data
func (eds *EmployeeDashboardService) GetEmployeeDashboardData(userID uint, userRole string) (map[string]interface{}, error) {
	data := make(map[string]interface{})

	// Get pending approvals for this user role
	pendingApprovals, err := eds.getPendingApprovalsForRole(userID, userRole)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending approvals: %v", err)
	}
	data["pending_approvals"] = pendingApprovals

	// Get user's submitted requests
	submittedRequests, err := eds.getUserSubmittedRequests(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get submitted requests: %v", err)
	}
	data["submitted_requests"] = submittedRequests

	// Get recent notifications
	recentNotifications, err := eds.GetRecentNotifications(userID, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent notifications: %v", err)
	}
	data["recent_notifications"] = recentNotifications

	// Get employee-specific quick stats
	quickStats, err := eds.getEmployeeQuickStats(userID, userRole)
	if err != nil {
		return nil, fmt.Errorf("failed to get quick stats: %v", err)
	}
	data["quick_stats"] = quickStats

	return data, nil
}

// getPendingApprovalsForRole gets approvals pending for a specific user role
func (eds *EmployeeDashboardService) getPendingApprovalsForRole(userID uint, userRole string) ([]map[string]interface{}, error) {
	type PendingApproval struct {
		ID                  uint      `json:"id"`
		RequestCode         string    `json:"request_code"`
		RequestTitle        string    `json:"request_title"`
		EntityType          string    `json:"entity_type"`
		Amount              float64   `json:"amount"`
		Priority            string    `json:"priority"`
		RequesterName       string    `json:"requester_name"`
		RequesterDepartment string    `json:"requester_department"`
		CreatedAt           time.Time `json:"created_at"`
		StepName            string    `json:"step_name"`
		TimeLimit           int       `json:"time_limit"`
	}

	var pendingApprovals []PendingApproval

	userRoleNorm := strings.ToLower(strings.TrimSpace(userRole))

	err := eds.DB.Raw(`
		SELECT 
			ar.id,
			ar.request_code,
			ar.request_title,
			ar.entity_type,
			ar.amount,
			ar.priority,
				CONCAT(u.first_name, ' ', u.last_name) as requester_name,
				COALESCE(u.department, '') as requester_department,
				ar.created_at,
			as_.step_name,
			as_.time_limit
		FROM approval_requests ar
		JOIN approval_actions aa ON ar.id = aa.request_id
		JOIN approval_steps as_ ON aa.step_id = as_.id
		JOIN users u ON ar.requester_id = u.id
		WHERE ar.status = 'PENDING'
			AND aa.is_active = true
			AND aa.status = 'PENDING'
			AND LOWER(as_.approver_role) = LOWER(?)
			AND ar.deleted_at IS NULL
		ORDER BY 
			CASE ar.priority 
				WHEN 'URGENT' THEN 1
				WHEN 'HIGH' THEN 2
				WHEN 'NORMAL' THEN 3
				WHEN 'LOW' THEN 4
				ELSE 5
			END,
			ar.created_at ASC
		LIMIT 20
	`, userRoleNorm).Scan(&pendingApprovals).Error

	if err != nil {
		return nil, err
	}

	// Convert to interface slice
	var result []map[string]interface{}
	for _, approval := range pendingApprovals {
		result = append(result, map[string]interface{}{
			"id":                   approval.ID,
			"request_code":         approval.RequestCode,
			"request_title":        approval.RequestTitle,
			"entity_type":          approval.EntityType,
			"amount":               approval.Amount,
			"priority":             approval.Priority,
			"requester_name":       approval.RequesterName,
			"requester_department": approval.RequesterDepartment,
			"created_at":           approval.CreatedAt,
			"step_name":            approval.StepName,
			"time_limit":           approval.TimeLimit,
			"days_pending":         int(time.Since(approval.CreatedAt).Hours() / 24),
		})
	}

	return result, nil
}

// getUserSubmittedRequests gets requests submitted by the user
func (eds *EmployeeDashboardService) getUserSubmittedRequests(userID uint) ([]map[string]interface{}, error) {
	type SubmittedRequest struct {
		ID                  uint       `json:"id"`
		RequestCode         string     `json:"request_code"`
		RequestTitle        string     `json:"request_title"`
		EntityType          string     `json:"entity_type"`
		Amount              float64    `json:"amount"`
		Status              string     `json:"status"`
		Priority            string     `json:"priority"`
		RequesterDepartment string     `json:"requester_department"`
		CreatedAt           time.Time  `json:"created_at"`
		CompletedAt         *time.Time `json:"completed_at"`
		CurrentStepName     string     `json:"current_step_name"`
	}

	var submittedRequests []SubmittedRequest

	err := eds.DB.Raw(`
		SELECT DISTINCT
			ar.id,
			ar.request_code,
			ar.request_title,
			ar.entity_type,
			ar.amount,
				ar.status,
				ar.priority,
				COALESCE(u.department, '') as requester_department,
				ar.created_at,
			ar.completed_at,
			COALESCE(as_.step_name, 'Completed') as current_step_name
		FROM approval_requests ar
			LEFT JOIN approval_actions aa ON ar.id = aa.request_id AND aa.is_active = true
			LEFT JOIN approval_steps as_ ON aa.step_id = as_.id
			LEFT JOIN users u ON ar.requester_id = u.id
			WHERE ar.requester_id = ?
			AND ar.deleted_at IS NULL
		ORDER BY ar.created_at DESC
		LIMIT 20
	`, userID).Scan(&submittedRequests).Error

	if err != nil {
		return nil, err
	}

	// Convert to interface slice
	var result []map[string]interface{}
	for _, request := range submittedRequests {
		statusColor := "info"
		switch request.Status {
		case "APPROVED":
			statusColor = "success"
		case "REJECTED":
			statusColor = "danger"
		case "PENDING":
			statusColor = "warning"
		}

		result = append(result, map[string]interface{}{
			"id":                   request.ID,
			"request_code":         request.RequestCode,
			"request_title":        request.RequestTitle,
			"entity_type":          request.EntityType,
			"amount":               request.Amount,
			"status":               request.Status,
			"status_color":         statusColor,
			"priority":             request.Priority,
			"requester_department": request.RequesterDepartment,
			"created_at":           request.CreatedAt,
			"completed_at":         request.CompletedAt,
			"current_step_name":    request.CurrentStepName,
			"days_since_submit":    int(time.Since(request.CreatedAt).Hours() / 24),
		})
	}

	return result, nil
}

// GetRecentNotifications gets recent notifications for user
func (eds *EmployeeDashboardService) GetRecentNotifications(userID uint, limit int) ([]map[string]interface{}, error) {
	type RecentNotification struct {
		ID        uint      `json:"id"`
		Type      string    `json:"type"`
		Title     string    `json:"title"`
		Message   string    `json:"message"`
		Priority  string    `json:"priority"`
		IsRead    bool      `json:"is_read"`
		CreatedAt time.Time `json:"created_at"`
		Data      string    `json:"data"`
	}

	var notifications []RecentNotification

	err := eds.DB.Raw(`
		SELECT 
			id, type, title, message, priority, is_read, created_at, data
		FROM notifications
		WHERE user_id = ?
			AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT ?
	`, userID, limit).Scan(&notifications).Error

	if err != nil {
		return nil, err
	}

	// Convert to interface slice
	var result []map[string]interface{}
	for _, notification := range notifications {
		result = append(result, map[string]interface{}{
			"id":         notification.ID,
			"type":       notification.Type,
			"title":      notification.Title,
			"message":    notification.Message,
			"priority":   notification.Priority,
			"is_read":    notification.IsRead,
			"created_at": notification.CreatedAt,
			"time_ago":   eds.formatTimeAgo(notification.CreatedAt),
			"data":       notification.Data,
		})
	}

	return result, nil
}

// getEmployeeQuickStats gets employee-specific quick statistics
func (eds *EmployeeDashboardService) getEmployeeQuickStats(userID uint, userRole string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	userRoleNorm := strings.ToLower(strings.TrimSpace(userRole))

	// Pending approvals count for this role
	var pendingApprovalsCount int64
	err := eds.DB.Raw(`
		SELECT COUNT(DISTINCT ar.id)
		FROM approval_requests ar
		JOIN approval_actions aa ON ar.id = aa.request_id
		JOIN approval_steps as_ ON aa.step_id = as_.id
		WHERE ar.status = 'PENDING'
			AND aa.is_active = true
			AND aa.status = 'PENDING'
			AND LOWER(as_.approver_role) = LOWER(?)
			AND ar.deleted_at IS NULL
	`, userRoleNorm).Scan(&pendingApprovalsCount).Error
	if err != nil {
		return nil, err
	}
	stats["pending_approvals_count"] = pendingApprovalsCount

	// User's submitted requests count
	var submittedRequestsCount int64
	err = eds.DB.Model(&models.ApprovalRequest{}).
		Where("requester_id = ? AND deleted_at IS NULL", userID).
		Count(&submittedRequestsCount).Error
	if err != nil {
		return nil, err
	}
	stats["submitted_requests_count"] = submittedRequestsCount

	// User's pending requests count
	var pendingRequestsCount int64
	err = eds.DB.Model(&models.ApprovalRequest{}).
		Where("requester_id = ? AND status = 'PENDING' AND deleted_at IS NULL", userID).
		Count(&pendingRequestsCount).Error
	if err != nil {
		return nil, err
	}
	stats["pending_requests_count"] = pendingRequestsCount

	// Unread notifications count
	var unreadNotificationsCount int64
	err = eds.DB.Model(&models.Notification{}).
		Where("user_id = ? AND is_read = false AND deleted_at IS NULL", userID).
		Count(&unreadNotificationsCount).Error
	if err != nil {
		return nil, err
	}
	stats["unread_notifications_count"] = unreadNotificationsCount

	// This month's approved requests by user (if they have approval permissions)
	if userRoleNorm == "employee" {
		// Get the employee's purchase requests count
		var purchaseCount int64
		err = eds.DB.Model(&models.Purchase{}).
			Where("user_id = ? AND deleted_at IS NULL", userID).
			Count(&purchaseCount).Error
		if err != nil {
			return nil, err
		}
		stats["purchase_requests_count"] = purchaseCount

		// Get the employee's approved purchase requests count
		var approvedCount int64
		err = eds.DB.Model(&models.Purchase{}).
			Where("user_id = ? AND status = 'APPROVED' AND deleted_at IS NULL", userID).
			Count(&approvedCount).Error
		if err != nil {
			return nil, err
		}
		stats["approved_purchase_requests_count"] = approvedCount

		// Get the employee's rejected purchase requests count
		var rejectedCount int64
		err = eds.DB.Model(&models.Purchase{}).
			Where("user_id = ? AND approval_status = 'REJECTED' AND deleted_at IS NULL", userID).
			Count(&rejectedCount).Error
		if err != nil {
			return nil, err
		}
		stats["rejected_purchase_requests_count"] = rejectedCount
	} else if userRoleNorm == "finance" || userRoleNorm == "director" || userRoleNorm == "admin" {
		// For approver roles, count their monthly approval activity
		var monthlyApprovedCount int64
		startOfMonth := time.Now().AddDate(0, 0, -time.Now().Day()+1)
		err = eds.DB.Raw(`
			SELECT COUNT(DISTINCT ar.id)
			FROM approval_requests ar
			JOIN approval_actions aa ON ar.id = aa.request_id
			WHERE aa.approver_id = ?
				AND aa.status = 'APPROVED'
				AND aa.action_date >= ?
				AND ar.deleted_at IS NULL
		`, userID, startOfMonth).Scan(&monthlyApprovedCount).Error
		if err != nil {
			return nil, err
		}
		stats["monthly_approved_count"] = monthlyApprovedCount
	}

	return stats, nil
}

// GetEmployeeApprovalWorkflows returns workflows applicable to employee
func (eds *EmployeeDashboardService) GetEmployeeApprovalWorkflows(userRole string) ([]models.ApprovalWorkflow, error) {
	userRoleNorm := strings.ToLower(strings.TrimSpace(userRole))

	var workflows []models.ApprovalWorkflow

	// First get all active workflows
	query := eds.DB.Preload("Steps").
		Where("is_active = ?", true)

	if err := query.Find(&workflows).Error; err != nil {
		return nil, err
	}

	// Filter to only include workflows where the user has a role
	var relevantWorkflows []models.ApprovalWorkflow
	for _, workflow := range workflows {
		isRelevant := false

		for _, step := range workflow.Steps {
			if strings.ToLower(step.ApproverRole) == userRoleNorm {
				isRelevant = true
				break
			}
		}

		// All employees can submit purchase requests
		if userRoleNorm == "employee" && workflow.Module == "PURCHASE" {
			isRelevant = true
		}

		if isRelevant {
			relevantWorkflows = append(relevantWorkflows, workflow)
		}
	}

	return relevantWorkflows, nil
}

// GetPurchaseRequestsForEmployee returns purchase requests submitted by employee with approval notifications
func (eds *EmployeeDashboardService) GetPurchaseRequestsForEmployee(userID uint) ([]map[string]interface{}, error) {
	type PurchaseRequest struct {
		ID                uint       `json:"id"`
		Code              string     `json:"code"`
		Date              time.Time  `json:"date"`
		TotalAmount       float64    `json:"total_amount"`
		Status            string     `json:"status"`
		ApprovalStatus    string     `json:"approval_status"`
		VendorName        string     `json:"vendor_name"`
		ApprovalRequestID *uint      `json:"approval_request_id"`
		RequestCode       *string    `json:"request_code"`
		ApprovedAt        *time.Time `json:"approved_at"`
		CurrentStepName   string     `json:"current_step_name"`
		DaysInCurrentStep int        `json:"days_in_current_step"`
	}

	var purchaseRequests []PurchaseRequest

	err := eds.DB.Raw(`
		SELECT 
			p.id,
			p.code,
			p.date,
			p.total_amount,
			p.status,
			p.approval_status,
			c.name as vendor_name,
			p.approval_request_id,
			ar.request_code,
			p.approved_at,
			COALESCE(ast.step_name, 'No Active Step') as current_step_name,
			COALESCE(EXTRACT(EPOCH FROM (NOW() - ar.created_at)) / 86400, 0)::int as days_in_current_step
		FROM purchases p
		JOIN contacts c ON p.vendor_id = c.id
		LEFT JOIN approval_requests ar ON p.approval_request_id = ar.id
		LEFT JOIN approval_actions aa ON ar.id = aa.request_id AND aa.is_active = true
		LEFT JOIN approval_steps ast ON aa.step_id = ast.id
		WHERE p.user_id = ?
			AND p.deleted_at IS NULL
		ORDER BY p.created_at DESC
		LIMIT 50
	`, userID).Scan(&purchaseRequests).Error

	if err != nil {
		return nil, err
	}

	// Convert to interface slice with enhanced approval information
	var result []map[string]interface{}
	for _, purchase := range purchaseRequests {
		statusColor := "info"
		statusMessage := purchase.Status

		switch purchase.Status {
		case "APPROVED":
			statusColor = "success"
			statusMessage = "Approved ✅"
		case "CANCELLED":
			statusColor = "danger"
			statusMessage = "Cancelled ❌"
		case "PENDING_APPROVAL":
			statusColor = "warning"
			statusMessage = fmt.Sprintf("Waiting for %s", purchase.CurrentStepName)
		case "COMPLETED":
			statusColor = "primary"
			statusMessage = "Completed 🎉"
		case "DRAFT":
			statusColor = "secondary"
			statusMessage = "Draft 📝"
		}

		// Add urgency indicator based on days pending
		urgencyLevel := "normal"
		if purchase.Status == "PENDING_APPROVAL" && purchase.DaysInCurrentStep > 3 {
			urgencyLevel = "high"
			if purchase.DaysInCurrentStep > 7 {
				urgencyLevel = "urgent"
			}
		}

		result = append(result, map[string]interface{}{
			"id":                   purchase.ID,
			"code":                 purchase.Code,
			"date":                 purchase.Date,
			"total_amount":         purchase.TotalAmount,
			"status":               purchase.Status,
			"status_color":         statusColor,
			"status_message":       statusMessage,
			"approval_status":      purchase.ApprovalStatus,
			"vendor_name":          purchase.VendorName,
			"approval_request_id":  purchase.ApprovalRequestID,
			"request_code":         purchase.RequestCode,
			"approved_at":          purchase.ApprovedAt,
			"current_step_name":    purchase.CurrentStepName,
			"days_in_current_step": purchase.DaysInCurrentStep,
			"urgency_level":        urgencyLevel,
			"needs_attention":      urgencyLevel == "urgent",
		})
	}

	return result, nil
}

// GetApprovalNotifications returns approval-related notifications for employee
func (eds *EmployeeDashboardService) GetApprovalNotifications(userID uint) (map[string]interface{}, error) {
	data := make(map[string]interface{})

	// Get approval-related notifications
	type ApprovalNotification struct {
		ID           uint      `json:"id"`
		Type         string    `json:"type"`
		Title        string    `json:"title"`
		Message      string    `json:"message"`
		Priority     string    `json:"priority"`
		IsRead       bool      `json:"is_read"`
		CreatedAt    time.Time `json:"created_at"`
		Data         string    `json:"data"`
		PurchaseCode string    `json:"purchase_code"`
		Amount       float64   `json:"amount"`
	}

	var notifications []ApprovalNotification

	err := eds.DB.Raw(`
		SELECT DISTINCT
			n.id,
			n.type,
			n.title,
			n.message,
			n.priority,
			n.is_read,
			n.created_at,
			n.data,
			COALESCE(p.code, '') as purchase_code,
			COALESCE(p.total_amount, 0) as amount
		FROM notifications n
		LEFT JOIN purchases p ON (n.data::json->>'entity_id')::int = p.id AND n.data::json->>'entity_type' = 'PURCHASE'
		WHERE n.user_id = ?
			AND n.type IN ('APPROVAL_PENDING', 'APPROVAL_APPROVED', 'APPROVAL_REJECTED')
			AND n.deleted_at IS NULL
		ORDER BY n.created_at DESC
		LIMIT 20
	`, userID).Scan(&notifications).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get approval notifications: %v", err)
	}

	// Convert to enhanced format
	var approvalNotifications []map[string]interface{}
	for _, notif := range notifications {
		notificationData := map[string]interface{}{
			"id":            notif.ID,
			"type":          notif.Type,
			"title":         notif.Title,
			"message":       notif.Message,
			"priority":      notif.Priority,
			"is_read":       notif.IsRead,
			"created_at":    notif.CreatedAt,
			"time_ago":      eds.formatTimeAgo(notif.CreatedAt),
			"data":          notif.Data,
			"purchase_code": notif.PurchaseCode,
			"amount":        notif.Amount,
		}

		// Add icon and color based on notification type
		switch notif.Type {
		case "APPROVAL_PENDING":
			notificationData["icon"] = "⏳"
			notificationData["color"] = "warning"
		case "APPROVAL_APPROVED":
			notificationData["icon"] = "✅"
			notificationData["color"] = "success"
		case "APPROVAL_REJECTED":
			notificationData["icon"] = "❌"
			notificationData["color"] = "danger"
		default:
			notificationData["icon"] = "📄"
			notificationData["color"] = "info"
		}

		approvalNotifications = append(approvalNotifications, notificationData)
	}

	data["approval_notifications"] = approvalNotifications

	// Count unread approval notifications
	var unreadCount int64
	err = eds.DB.Model(&models.Notification{}).
		Where("user_id = ? AND is_read = false AND type IN (?, ?, ?) AND deleted_at IS NULL",
			userID, "APPROVAL_PENDING", "APPROVAL_APPROVED", "APPROVAL_REJECTED").
		Count(&unreadCount).Error

	if err != nil {
		return nil, fmt.Errorf("failed to count unread approval notifications: %v", err)
	}

	data["unread_count"] = unreadCount
	data["total_count"] = len(approvalNotifications)

	return data, nil
}

// GetPurchaseApprovalStatus returns detailed approval status for employee's purchases
func (eds *EmployeeDashboardService) GetPurchaseApprovalStatus(userID uint) (map[string]interface{}, error) {
	type PurchaseApprovalStatus struct {
		ID                  uint       `json:"id"`
		Code                string     `json:"code"`
		VendorName          string     `json:"vendor_name"`
		TotalAmount         float64    `json:"total_amount"`
		Status              string     `json:"status"`
		ApprovalStatus      string     `json:"approval_status"`
		ApprovalRequestID   *uint      `json:"approval_request_id"`
		RequestCode         *string    `json:"request_code"`
		RequestStatus       *string    `json:"request_status"`
		CurrentStepName     *string    `json:"current_step_name"`
		CurrentApproverRole *string    `json:"current_approver_role"`
		DaysInCurrentStep   *int       `json:"days_in_current_step"`
		LastActionDate      *time.Time `json:"last_action_date"`
		LastActionByName    *string    `json:"last_action_by_name"`
		LastActionComments  *string    `json:"last_action_comments"`
		CreatedAt           time.Time  `json:"created_at"`
	}

	var purchases []PurchaseApprovalStatus

	err := eds.DB.Raw(`
		SELECT 
			p.id,
			p.code,
			c.name as vendor_name,
			p.total_amount,
			p.status,
			p.approval_status,
			p.approval_request_id,
			ar.request_code,
			ar.status as request_status,
			ast.step_name as current_step_name,
			ast.approver_role as current_approver_role,
			EXTRACT(EPOCH FROM (NOW() - ar.created_at)) / 86400::int as days_in_current_step,
			aa_last.action_date as last_action_date,
			CONCAT(u_last.first_name, ' ', u_last.last_name) as last_action_by_name,
			aa_last.comments as last_action_comments,
			p.created_at
		FROM purchases p
		JOIN contacts c ON p.vendor_id = c.id
		LEFT JOIN approval_requests ar ON p.approval_request_id = ar.id
		LEFT JOIN approval_actions aa_active ON ar.id = aa_active.request_id AND aa_active.is_active = true
		LEFT JOIN approval_steps ast ON aa_active.step_id = ast.id
		LEFT JOIN (
			SELECT DISTINCT ON (request_id) request_id, action_date, comments, approver_id
			FROM approval_actions 
			WHERE action_date IS NOT NULL AND status IN ('APPROVED', 'REJECTED')
			ORDER BY request_id, action_date DESC
		) aa_last ON ar.id = aa_last.request_id
		LEFT JOIN users u_last ON aa_last.approver_id = u_last.id
		WHERE p.user_id = ?
			AND p.deleted_at IS NULL
			AND p.requires_approval = true
		ORDER BY p.created_at DESC
		LIMIT 30
	`, userID).Scan(&purchases).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get purchase approval status: %v", err)
	}

	// Convert to enhanced format
	var result []map[string]interface{}
	for _, purchase := range purchases {
		statusInfo := eds.getDetailedStatusInfo(purchase.Status, purchase.ApprovalStatus, purchase.DaysInCurrentStep)

		purchaseData := map[string]interface{}{
			"id":                    purchase.ID,
			"code":                  purchase.Code,
			"vendor_name":           purchase.VendorName,
			"total_amount":          purchase.TotalAmount,
			"status":                purchase.Status,
			"approval_status":       purchase.ApprovalStatus,
			"approval_request_id":   purchase.ApprovalRequestID,
			"request_code":          purchase.RequestCode,
			"request_status":        purchase.RequestStatus,
			"current_step_name":     purchase.CurrentStepName,
			"current_approver_role": purchase.CurrentApproverRole,
			"days_in_current_step":  purchase.DaysInCurrentStep,
			"last_action_date":      purchase.LastActionDate,
			"last_action_by_name":   purchase.LastActionByName,
			"last_action_comments":  purchase.LastActionComments,
			"created_at":            purchase.CreatedAt,
			"status_info":           statusInfo,
		}

		result = append(result, purchaseData)
	}

	return map[string]interface{}{
		"purchase_approvals": result,
		"total":              len(result),
	}, nil
}

// getDetailedStatusInfo returns detailed status information with colors and messages
func (eds *EmployeeDashboardService) getDetailedStatusInfo(status, approvalStatus string, daysInStep *int) map[string]interface{} {
	statusInfo := map[string]interface{}{
		"color":       "info",
		"icon":        "📄",
		"message":     status,
		"action_text": "No action required",
		"urgency":     "normal",
	}

	switch status {
	case "DRAFT":
		statusInfo["color"] = "secondary"
		statusInfo["icon"] = "📝"
		statusInfo["message"] = "Draft - Not submitted yet"
		statusInfo["action_text"] = "Submit for approval"
	case "PENDING_APPROVAL":
		statusInfo["color"] = "warning"
		statusInfo["icon"] = "⏳"
		statusInfo["message"] = "Pending approval"
		statusInfo["action_text"] = "Waiting for approver"

		if daysInStep != nil {
			if *daysInStep > 7 {
				statusInfo["urgency"] = "urgent"
				statusInfo["message"] = fmt.Sprintf("Pending approval (%d days) - URGENT!", *daysInStep)
			} else if *daysInStep > 3 {
				statusInfo["urgency"] = "high"
				statusInfo["message"] = fmt.Sprintf("Pending approval (%d days)", *daysInStep)
			} else {
				statusInfo["message"] = fmt.Sprintf("Pending approval (%d days)", *daysInStep)
			}
		}
	case "APPROVED":
		statusInfo["color"] = "success"
		statusInfo["icon"] = "✅"
		statusInfo["message"] = "Approved"
		statusInfo["action_text"] = "Ready for processing"
	case "CANCELLED":
		statusInfo["color"] = "danger"
		statusInfo["icon"] = "❌"
		statusInfo["message"] = "Cancelled"
		statusInfo["action_text"] = "Request cancelled"
	case "COMPLETED":
		statusInfo["color"] = "primary"
		statusInfo["icon"] = "🎉"
		statusInfo["message"] = "Completed"
		statusInfo["action_text"] = "Process completed"
	}

	// Override with approval status if rejected
	if approvalStatus == "REJECTED" {
		statusInfo["color"] = "danger"
		statusInfo["icon"] = "❌"
		statusInfo["message"] = "Rejected"
		statusInfo["action_text"] = "Review and resubmit"
		statusInfo["urgency"] = "high"
	}

	return statusInfo
}

// formatTimeAgo formats time as "X minutes/hours/days ago"
func (eds *EmployeeDashboardService) formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Hour {
		minutes := int(duration.Minutes())
		if minutes < 1 {
			return "just now"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%d hours ago", hours)
	} else {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%d days ago", days)
	}
}
