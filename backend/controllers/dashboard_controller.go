package controllers

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DashboardController struct {
	DB                       *gorm.DB
	stockMonitoringService   *services.StockMonitoringService
	dashboardService         *services.DashboardService
	employeeDashboardService *services.EmployeeDashboardService
}

func NewDashboardController(db *gorm.DB, stockMonitoringService *services.StockMonitoringService) *DashboardController {
	return &DashboardController{
		DB:                       db,
		stockMonitoringService:   stockMonitoringService,
		dashboardService:         services.NewDashboardService(db),
		employeeDashboardService: services.NewEmployeeDashboardService(db),
	}
}

// GetStockAlertsBanner returns stock alerts specifically for banner display
func (dc *DashboardController) GetStockAlertsBanner(c *gin.Context) {
	userRole := c.GetString("user_role")

	// Only for authorized roles
	userRoleLower := strings.ToLower(userRole)
	if userRoleLower != "admin" && userRoleLower != "inventory_manager" && userRoleLower != "director" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized to view stock alerts"})
		return
	}

	// Get active stock alerts
	activeAlerts, err := dc.stockMonitoringService.GetActiveStockAlerts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stock alerts"})
		return
	}

	// Format alerts for banner display
	var bannerAlerts []map[string]interface{}
	for _, alert := range activeAlerts {
		bannerAlert := map[string]interface{}{
			"id":              alert.ID,
			"product_id":      alert.ProductID,
			"product_name":    alert.Product.Name,
			"product_code":    alert.Product.Code,
			"current_stock":   alert.CurrentStock,
			"threshold_stock": alert.ThresholdStock,
			"alert_type":      alert.AlertType,
			"urgency":         dc.getUrgencyLevel(alert),
			"message":         dc.formatAlertMessage(alert),
		}

		if alert.Product.Category != nil {
			bannerAlert["category_name"] = alert.Product.Category.Name
		}

		bannerAlerts = append(bannerAlerts, bannerAlert)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Stock alerts retrieved successfully",
		"data": gin.H{
			"alerts":      bannerAlerts,
			"total_count": len(bannerAlerts),
			"show_banner": len(bannerAlerts) > 0,
		},
	})
}

// DismissStockAlert allows users to dismiss a stock alert
func (dc *DashboardController) DismissStockAlert(c *gin.Context) {
	alertID := c.Param("id")
	userRole := c.GetString("user_role")

	userRoleLower := strings.ToLower(userRole)
	if userRoleLower != "admin" && userRoleLower != "inventory_manager" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized to dismiss alerts"})
		return
	}

	var alert models.StockAlert
	if err := dc.DB.First(&alert, alertID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alert not found"})
		return
	}

	alert.Status = models.StockAlertStatusDismissed
	if err := dc.DB.Save(&alert).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to dismiss alert"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Alert dismissed successfully"})
}

// GetAnalytics returns dashboard analytics data with real growth calculations
// @Summary Get analytics data
// @Description Retrieve comprehensive analytics data for admin, finance, and director roles
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.APIResponse "Analytics data retrieved successfully"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 403 {object} models.ErrorResponse "Forbidden - insufficient privileges"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /dashboard/analytics [get]
func (dc *DashboardController) GetAnalytics(c *gin.Context) {
	userRole := c.GetString("user_role")

	// Only allow admin, finance-family, and director roles
	userRoleLower := strings.ToLower(userRole)
	if userRoleLower != "admin" && userRoleLower != "finance" && userRoleLower != "finance_manager" && userRoleLower != "finance_staff" && userRoleLower != "accounting" && userRoleLower != "director" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Unauthorized to view analytics"})
		return
	}

	// Get comprehensive analytics with real growth calculations
	analytics, err := dc.dashboardService.GetDashboardAnalyticsForRole(userRoleLower)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch dashboard analytics",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, analytics)
}

// GetFinanceDashboardData returns finance-specific dashboard data
// @Summary Get finance dashboard data
// @Description Retrieve finance-specific dashboard metrics including invoices, journals, and working capital signals
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.APIResponse "Finance dashboard data retrieved successfully"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 403 {object} models.ErrorResponse "Forbidden - not finance role"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /dashboard/finance [get]
func (dc *DashboardController) GetFinanceDashboardData(c *gin.Context) {
	userRole := c.GetString("user_role")

	// Only allow finance-family roles and admin
	userRoleLower := strings.ToLower(userRole)
	if userRoleLower != "finance" && userRoleLower != "finance_manager" && userRoleLower != "finance_staff" && userRoleLower != "accounting" && userRoleLower != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied - Finance role required"})
		return
	}

	financeData := make(map[string]interface{})

	// Invoices pending payment (approved/invoiced sales only)
	var invoicesPendingPayment int64
	dc.DB.Model(&models.Sale{}).
		Where("status IN (?) AND outstanding_amount > 0 AND deleted_at IS NULL",
			[]string{"INVOICED", "OVERDUE"}).
		Count(&invoicesPendingPayment)
	financeData["invoices_pending_payment"] = invoicesPendingPayment

	// Invoices not yet paid (approved vendor bills only)
	var invoicesNotPaid int64
	dc.DB.Model(&models.Purchase{}).
		Where("status IN (?) AND outstanding_amount > 0 AND deleted_at IS NULL",
			[]string{"APPROVED", "COMPLETED", "PAID"}).
		Count(&invoicesNotPaid)
	financeData["invoices_not_paid"] = invoicesNotPaid

	// Journal entries requiring posting (unposted)
	var journalsNeedPosting int64
	dc.DB.Model(&models.JournalEntry{}).
		Where("status = ? AND deleted_at IS NULL", "DRAFT").
		Count(&journalsNeedPosting)
	financeData["journals_need_posting"] = journalsNeedPosting

	// Outstanding receivables amount from collectible sales only
	var outstandingReceivables float64
	dc.DB.Model(&models.Sale{}).
		Where("status IN (?) AND outstanding_amount > 0 AND deleted_at IS NULL", []string{"INVOICED", "OVERDUE"}).
		Select("COALESCE(SUM(outstanding_amount), 0)").
		Scan(&outstandingReceivables)
	financeData["outstanding_receivables"] = outstandingReceivables

	// Outstanding payables amount from approved purchases only
	var outstandingPayables float64
	dc.DB.Model(&models.Purchase{}).
		Where("status IN (?) AND outstanding_amount > 0 AND deleted_at IS NULL", []string{"APPROVED", "COMPLETED", "PAID"}).
		Select("COALESCE(SUM(outstanding_amount), 0)").
		Scan(&outstandingPayables)
	financeData["outstanding_payables"] = outstandingPayables

	// Purchases pending approval
	var purchasesPendingApproval int64
	dc.DB.Model(&models.Purchase{}).
		Where("approval_status = ? AND deleted_at IS NULL", models.PurchaseApprovalPending).
		Count(&purchasesPendingApproval)
	financeData["purchases_pending_approval"] = purchasesPendingApproval

	// Sales pending approval (status = PENDING)
	var salesPendingApproval int64
	dc.DB.Model(&models.Sale{}).
		Where("status = ? AND deleted_at IS NULL", "PENDING").
		Count(&salesPendingApproval)
	financeData["sales_pending_approval"] = salesPendingApproval

	// Cash and bank balance from active cash/bank records using IDR book values
	var cashBankBalance float64
	dc.DB.Model(&models.CashBank{}).
		Where("is_active = true AND deleted_at IS NULL").
		Select(`COALESCE(SUM(CASE WHEN UPPER(currency) = 'IDR' OR currency = '' THEN balance ELSE "book_balance_id_r" END), 0)`).
		Scan(&cashBankBalance)
	financeData["cash_bank_balance"] = cashBankBalance

	c.JSON(http.StatusOK, gin.H{
		"message": "Finance dashboard data retrieved successfully",
		"data":    financeData,
	})
}

// GetFinanceStaffDashboardData returns an operational dashboard tailored for
// finance staff responsibilities.
func (dc *DashboardController) GetFinanceStaffDashboardData(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	userRole := strings.ToLower(strings.TrimSpace(c.GetString("user_role")))

	if userRole != "finance_staff" && userRole != "accounting" && userRole != "finance" && userRole != "finance_manager" && userRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied - Finance staff role required"})
		return
	}

	dashboardData := make(map[string]interface{})

	var pendingPurchaseRequestsCount int64
	dc.DB.Model(&models.PurchaseRequest{}).
		Where("status = ? AND current_approval_stage = ? AND deleted_at IS NULL", models.PRStatusPendingApproval, models.PRApprovalStageFinanceStaff).
		Count(&pendingPurchaseRequestsCount)

	var draftJournalsCount int64
	dc.DB.Model(&models.JournalEntry{}).
		Where("status = ? AND deleted_at IS NULL", models.JournalStatusDraft).
		Count(&draftJournalsCount)

	var unpaidPurchaseCount int64
	dc.DB.Model(&models.Purchase{}).
		Where("status IN (?) AND outstanding_amount > 0 AND deleted_at IS NULL", []string{"APPROVED", "COMPLETED", "PAID"}).
		Count(&unpaidPurchaseCount)

	var pendingTransferRequestsCount int64
	dc.DB.Table("cash_bank_transfers").
		Where("user_id = ? AND status = ? AND deleted_at IS NULL", userID, services.TransferStatusPendingApproval).
		Count(&pendingTransferRequestsCount)

	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	var monthlyReviewedCount int64
	dc.DB.Model(&models.PurchaseRequest{}).
		Where("finance_staff_approved_by = ? AND finance_staff_approved_at >= ? AND deleted_at IS NULL", userID, startOfMonth).
		Count(&monthlyReviewedCount)

	var unreadNotificationsCount int64
	dc.DB.Model(&models.Notification{}).
		Where("user_id = ? AND is_read = false AND deleted_at IS NULL", userID).
		Count(&unreadNotificationsCount)

	var approvedPurchaseNotificationsCount int64
	dc.DB.Table("notifications n").
		Joins("JOIN purchases p ON p.id = CAST(COALESCE(NULLIF(n.data::json->>'entity_id', ''), NULLIF(n.data::json->>'purchase_id', '')) AS bigint)").
		Where(
			"n.user_id = ? AND n.type = ? AND n.is_read = false AND n.deleted_at IS NULL AND p.user_id = ? AND p.deleted_at IS NULL",
			userID,
			models.NotificationTypeApprovalApproved,
			userID,
		).
		Count(&approvedPurchaseNotificationsCount)

	var outstandingReceivables float64
	dc.DB.Model(&models.Sale{}).
		Where("status IN (?) AND outstanding_amount > 0 AND deleted_at IS NULL", []string{"INVOICED", "OVERDUE"}).
		Select("COALESCE(SUM(outstanding_amount), 0)").
		Scan(&outstandingReceivables)

	var outstandingPayables float64
	dc.DB.Model(&models.Purchase{}).
		Where("status IN (?) AND outstanding_amount > 0 AND deleted_at IS NULL", []string{"APPROVED", "COMPLETED", "PAID"}).
		Select("COALESCE(SUM(outstanding_amount), 0)").
		Scan(&outstandingPayables)

	var cashBankBalance float64
	dc.DB.Model(&models.CashBank{}).
		Where("is_active = true AND deleted_at IS NULL").
		Select(`COALESCE(SUM(CASE WHEN UPPER(currency) = 'IDR' OR currency = '' THEN balance ELSE "book_balance_id_r" END), 0)`).
		Scan(&cashBankBalance)

	type pendingPurchaseRequestItem struct {
		ID                   uint      `json:"id"`
		PRNumber             string    `json:"pr_number"`
		RequesterName        string    `json:"requester_name"`
		Department           string    `json:"department"`
		ProjectName          string    `json:"project_name"`
		TotalEstimatedAmount float64   `json:"total_estimated_amount"`
		CreatedAt            time.Time `json:"created_at"`
	}

	type recentTransferItem struct {
		ID              uint      `json:"id"`
		TransferNumber  string    `json:"transfer_number"`
		Date            time.Time `json:"date"`
		Amount          float64   `json:"amount"`
		ConvertedAmount float64   `json:"converted_amount"`
		Status          string    `json:"status"`
		Reference       string    `json:"reference"`
		RejectionReason string    `json:"rejection_reason"`
		FromAccountName string    `json:"from_account_name"`
		FromCurrency    string    `json:"from_currency"`
		ToAccountName   string    `json:"to_account_name"`
		ToCurrency      string    `json:"to_currency"`
	}

	type notificationItem struct {
		ID        uint      `json:"id"`
		Type      string    `json:"type"`
		Title     string    `json:"title"`
		Message   string    `json:"message"`
		Priority  string    `json:"priority"`
		IsRead    bool      `json:"is_read"`
		CreatedAt time.Time `json:"created_at"`
	}

	type approvedPurchaseNotificationItem struct {
		NotificationID uint       `json:"notification_id"`
		PurchaseID     uint       `json:"purchase_id"`
		PurchaseCode   string     `json:"purchase_code"`
		VendorName     string     `json:"vendor_name"`
		TotalAmount    float64    `json:"total_amount"`
		ApprovedAt     *time.Time `json:"approved_at"`
		Message        string     `json:"message"`
		IsRead         bool       `json:"is_read"`
		CreatedAt      time.Time  `json:"created_at"`
	}

	var pendingPurchaseRequests []pendingPurchaseRequestItem
	if err := dc.DB.Table("purchase_requests pr").
		Select(`
			pr.id,
			pr.pr_number,
			COALESCE(NULLIF(TRIM(CONCAT(u.first_name, ' ', u.last_name)), ''), u.username, u.email) AS requester_name,
			pr.department,
			COALESCE(p.name, '-') AS project_name,
			pr.total_estimated_amount,
			pr.created_at
		`).
		Joins("LEFT JOIN users u ON u.id = pr.requester_id").
		Joins("LEFT JOIN projects p ON p.id = pr.project_id").
		Where("pr.status = ? AND pr.current_approval_stage = ? AND pr.deleted_at IS NULL", models.PRStatusPendingApproval, models.PRApprovalStageFinanceStaff).
		Order("pr.created_at ASC").
		Limit(5).
		Scan(&pendingPurchaseRequests).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch pending purchase requests", "details": err.Error()})
		return
	}

	var recentTransfers []recentTransferItem
	if err := dc.DB.Table("cash_bank_transfers t").
		Select(`
			t.id,
			t.transfer_number,
			t.date,
			t.amount,
			t.converted_amount,
			t.status,
			t.reference,
			t.rejection_reason,
			COALESCE(f.name, '-') AS from_account_name,
			COALESCE(f.currency, 'IDR') AS from_currency,
			COALESCE(d.name, '-') AS to_account_name,
			COALESCE(d.currency, 'IDR') AS to_currency
		`).
		Joins("LEFT JOIN cash_banks f ON f.id = t.from_account_id").
		Joins("LEFT JOIN cash_banks d ON d.id = t.to_account_id").
		Where("t.user_id = ? AND t.deleted_at IS NULL", userID).
		Order("t.created_at DESC").
		Limit(5).
		Scan(&recentTransfers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch recent transfer requests", "details": err.Error()})
		return
	}

	var recentNotifications []notificationItem
	if err := dc.DB.Model(&models.Notification{}).
		Select("id, type, title, message, priority, is_read, created_at").
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Order("created_at DESC").
		Limit(5).
		Find(&recentNotifications).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch recent notifications", "details": err.Error()})
		return
	}

	var approvedPurchaseNotifications []approvedPurchaseNotificationItem
	if err := dc.DB.Table("notifications n").
		Select(`
			n.id AS notification_id,
			p.id AS purchase_id,
			COALESCE(NULLIF(p.code, ''), n.data::json->>'purchase_code', '-') AS purchase_code,
			COALESCE(v.name, '-') AS vendor_name,
			p.total_amount,
			p.approved_at,
			n.message,
			n.is_read,
			n.created_at
		`).
		Joins("JOIN purchases p ON p.id = CAST(COALESCE(NULLIF(n.data::json->>'entity_id', ''), NULLIF(n.data::json->>'purchase_id', '')) AS bigint)").
		Joins("LEFT JOIN contacts v ON v.id = p.vendor_id").
		Where(`
			n.user_id = ?
			AND n.type = ?
			AND n.is_read = false
			AND n.deleted_at IS NULL
			AND p.user_id = ?
			AND p.deleted_at IS NULL
		`, userID, models.NotificationTypeApprovalApproved, userID).
		Order("n.created_at DESC").
		Limit(5).
		Scan(&approvedPurchaseNotifications).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch approved purchase notifications", "details": err.Error()})
		return
	}

	dashboardData["quick_stats"] = gin.H{
		"pending_purchase_requests_count":       pendingPurchaseRequestsCount,
		"draft_journals_count":                  draftJournalsCount,
		"unpaid_purchase_count":                 unpaidPurchaseCount,
		"pending_transfer_requests_count":       pendingTransferRequestsCount,
		"monthly_reviewed_purchase_count":       monthlyReviewedCount,
		"unread_notifications_count":            unreadNotificationsCount,
		"approved_purchase_notifications_count": approvedPurchaseNotificationsCount,
	}
	dashboardData["snapshot"] = gin.H{
		"outstanding_receivables": outstandingReceivables,
		"outstanding_payables":    outstandingPayables,
		"cash_bank_balance":       cashBankBalance,
	}
	dashboardData["pending_purchase_requests"] = pendingPurchaseRequests
	dashboardData["recent_transfer_requests"] = recentTransfers
	dashboardData["recent_notifications"] = recentNotifications
	dashboardData["finance_manager_approved_purchases"] = approvedPurchaseNotifications

	c.JSON(http.StatusOK, gin.H{
		"message": "Finance staff dashboard data retrieved successfully",
		"data":    dashboardData,
	})
}

// GetEmployeeDashboardData returns employee-specific dashboard data
// @Summary Get employee dashboard data
// @Description Retrieve employee-specific dashboard data including pending approvals, submitted requests, and notifications
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.APIResponse "Employee dashboard data retrieved successfully"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /dashboard/employee [get]
func (dc *DashboardController) GetEmployeeDashboardData(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	userRole := c.GetString("user_role")

	// Get employee dashboard data
	data, err := dc.employeeDashboardService.GetEmployeeDashboardData(userID, userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch employee dashboard data",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Employee dashboard data retrieved successfully",
		"data":    data,
	})
}

// GetEmployeeApprovalWorkflows returns approval workflows relevant to employee role
// @Summary Get employee approval workflows
// @Description Get approval workflows where the employee has a role or can submit requests
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.APIResponse "Approval workflows retrieved successfully"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /dashboard/employee/workflows [get]
func (dc *DashboardController) GetEmployeeApprovalWorkflows(c *gin.Context) {
	userRole := c.GetString("user_role")

	// Get workflows relevant to this employee role
	workflows, err := dc.employeeDashboardService.GetEmployeeApprovalWorkflows(userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch approval workflows",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Employee approval workflows retrieved successfully",
		"data": gin.H{
			"workflows": workflows,
			"total":     len(workflows),
			"user_role": userRole,
		},
	})
}

// GetEmployeePurchaseRequests returns purchase requests submitted by employee
// @Summary Get employee purchase requests
// @Description Get purchase requests submitted by the employee with approval status
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.APIResponse "Purchase requests retrieved successfully"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /dashboard/employee/purchase-requests [get]
func (dc *DashboardController) GetEmployeePurchaseRequests(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	// Get purchase requests for this employee
	purchaseRequests, err := dc.employeeDashboardService.GetPurchaseRequestsForEmployee(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch purchase requests",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Employee purchase requests retrieved successfully",
		"data": gin.H{
			"purchase_requests": purchaseRequests,
			"total":             len(purchaseRequests),
		},
	})
}

// GetEmployeeNotificationsSummary returns notification summary for employee
// @Summary Get employee notifications summary
// @Description Get summary of notifications for the employee including unread count
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.APIResponse "Notifications summary retrieved successfully"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /dashboard/employee/notifications-summary [get]
func (dc *DashboardController) GetEmployeeNotificationsSummary(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	// Get recent notifications
	recentNotifications, err := dc.employeeDashboardService.GetRecentNotifications(userID, 20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch notifications",
			"details": err.Error(),
		})
		return
	}

	// Count unread notifications
	var unreadCount int64
	dc.DB.Model(&models.Notification{}).
		Where("user_id = ? AND is_read = false AND deleted_at IS NULL", userID).
		Count(&unreadCount)

	c.JSON(http.StatusOK, gin.H{
		"message": "Employee notifications summary retrieved successfully",
		"data": gin.H{
			"notifications": recentNotifications,
			"unread_count":  unreadCount,
			"total":         len(recentNotifications),
		},
	})
}

// GetEmployeeApprovalNotifications returns approval-specific notifications for employee
// @Summary Get employee approval notifications
// @Description Get approval-related notifications including status updates on purchase requests
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.APIResponse "Approval notifications retrieved successfully"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /dashboard/employee/approval-notifications [get]
func (dc *DashboardController) GetEmployeeApprovalNotifications(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	// Get approval notifications
	notifications, err := dc.employeeDashboardService.GetApprovalNotifications(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch approval notifications",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Employee approval notifications retrieved successfully",
		"data":    notifications,
	})
}

// GetEmployeePurchaseApprovalStatus returns detailed approval status for employee's purchases
// @Summary Get employee purchase approval status
// @Description Get detailed approval status for purchases submitted by employee
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.APIResponse "Purchase approval status retrieved successfully"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /dashboard/employee/purchase-approval-status [get]
func (dc *DashboardController) GetEmployeePurchaseApprovalStatus(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)

	// Get purchase approval status
	approvalStatus, err := dc.employeeDashboardService.GetPurchaseApprovalStatus(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch purchase approval status",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Employee purchase approval status retrieved successfully",
		"data":    approvalStatus,
	})
}

// MarkNotificationAsRead marks a notification as read
// @Summary Mark notification as read
// @Description Mark a specific notification as read for the current user
// @Tags Dashboard
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Notification ID"
// @Success 200 {object} models.APIResponse "Notification marked as read successfully"
// @Failure 400 {object} models.ErrorResponse "Invalid notification ID"
// @Failure 403 {object} models.ErrorResponse "Access denied"
// @Failure 404 {object} models.ErrorResponse "Notification not found"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /dashboard/employee/notifications/{id}/read [patch]
func (dc *DashboardController) MarkNotificationAsRead(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	notificationID := c.Param("id")

	if notificationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Notification ID is required"})
		return
	}

	// Verify notification belongs to user and update
	var notification models.Notification
	err := dc.DB.Where("id = ? AND user_id = ?", notificationID, userID).First(&notification).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Notification not found or access denied"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find notification"})
		}
		return
	}

	// Update notification as read
	notification.IsRead = true
	if err := dc.DB.Save(&notification).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notification as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Notification marked as read successfully",
		"data": gin.H{
			"notification_id": notification.ID,
			"is_read":         notification.IsRead,
		},
	})
}

// Private helper methods

func (dc *DashboardController) getGeneralStatistics(role string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Product statistics
	var totalProducts, activeProducts, inactiveProducts int64
	dc.DB.Model(&models.Product{}).Count(&totalProducts)
	dc.DB.Model(&models.Product{}).Where("is_active = ?", true).Count(&activeProducts)
	dc.DB.Model(&models.Product{}).Where("is_active = ?", false).Count(&inactiveProducts)

	stats["products"] = map[string]int64{
		"total":    totalProducts,
		"active":   activeProducts,
		"inactive": inactiveProducts,
	}

	// Contact statistics
	var totalContacts, customers, vendors int64
	dc.DB.Model(&models.Contact{}).Count(&totalContacts)
	dc.DB.Model(&models.Contact{}).Where("type = ?", models.ContactTypeCustomer).Count(&customers)
	dc.DB.Model(&models.Contact{}).Where("type = ?", models.ContactTypeVendor).Count(&vendors)

	stats["contacts"] = map[string]int64{
		"total":     totalContacts,
		"customers": customers,
		"vendors":   vendors,
	}

	return stats, nil
}

func (dc *DashboardController) getRecentActivities(userID uint, role string) ([]map[string]interface{}, error) {
	var activities []map[string]interface{}

	// Get recent audit logs
	var auditLogs []models.AuditLog
	query := dc.DB.Order("created_at DESC").Limit(10)

	if role != "admin" {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Find(&auditLogs).Error; err != nil {
		return activities, err
	}

	for _, log := range auditLogs {
		activity := map[string]interface{}{
			"id":         log.ID,
			"action":     log.Action,
			"table_name": log.TableName,
			"record_id":  log.RecordID,
			"user_id":    log.UserID,
			"created_at": log.CreatedAt,
		}
		activities = append(activities, activity)
	}

	return activities, nil
}

func (dc *DashboardController) getUrgencyLevel(alert models.StockAlert) string {
	switch alert.AlertType {
	case models.StockAlertTypeOutOfStock:
		return "critical"
	case models.StockAlertTypeReorderAlert:
		if alert.ThresholdStock <= 0 {
			return "medium"
		}
	case models.StockAlertTypeLowStock:
		if alert.ThresholdStock <= 0 {
			return "high"
		}
	default:
		if alert.ThresholdStock <= 0 {
			return "low"
		}
	}

	percentageOfMin := float64(alert.CurrentStock) / float64(alert.ThresholdStock) * 100

	if percentageOfMin <= 25 {
		return "critical"
	} else if percentageOfMin <= 50 {
		return "high"
	} else if percentageOfMin <= 75 {
		return "medium"
	}
	return "low"
}

func (dc *DashboardController) formatAlertMessage(alert models.StockAlert) string {
	switch alert.AlertType {
	case models.StockAlertTypeLowStock:
		return fmt.Sprintf("%s is running low. Current stock: %d (Min: %d)",
			alert.Product.Name, alert.CurrentStock, alert.ThresholdStock)
	case models.StockAlertTypeOutOfStock:
		return fmt.Sprintf("%s is out of stock!", alert.Product.Name)
	default:
		return fmt.Sprintf("%s requires attention. Current stock: %d",
			alert.Product.Name, alert.CurrentStock)
	}
}

// getMonthlySalesData gets sales data for the last 7 months
func (dc *DashboardController) getMonthlySalesData() []map[string]interface{} {
	type MonthlyData struct {
		Month string  `json:"month"`
		Value float64 `json:"value"`
	}

	var results []MonthlyData
	dc.DB.Raw(`
		SELECT 
			TO_CHAR(date, 'Mon') as month,
			COALESCE(SUM(total_amount), 0) as value
		FROM sales 
		WHERE date >= CURRENT_DATE - INTERVAL '7 months'
			AND deleted_at IS NULL
		GROUP BY EXTRACT(YEAR FROM date), EXTRACT(MONTH FROM date), TO_CHAR(date, 'Mon')
		ORDER BY EXTRACT(YEAR FROM date), EXTRACT(MONTH FROM date)
	`).Scan(&results)

	// Convert to interface{} slice
	var data []map[string]interface{}
	for _, result := range results {
		data = append(data, map[string]interface{}{
			"month": result.Month,
			"value": result.Value,
		})
	}

	// Return only real data from database - no dummy data

	return data
}

// getMonthlyPurchasesData gets purchase data for the last 7 months
func (dc *DashboardController) getMonthlyPurchasesData() []map[string]interface{} {
	type MonthlyData struct {
		Month string  `json:"month"`
		Value float64 `json:"value"`
	}

	var results []MonthlyData
	dc.DB.Raw(`
		SELECT 
			TO_CHAR(date, 'Mon') as month,
			COALESCE(SUM(total_amount), 0) as value
		FROM purchases 
		WHERE date >= CURRENT_DATE - INTERVAL '7 months'
			AND deleted_at IS NULL
			AND status IN ('APPROVED', 'COMPLETED')
		GROUP BY EXTRACT(YEAR FROM date), EXTRACT(MONTH FROM date), TO_CHAR(date, 'Mon')
		ORDER BY EXTRACT(YEAR FROM date), EXTRACT(MONTH FROM date)
	`).Scan(&results)

	// Convert to interface{} slice
	var data []map[string]interface{}
	for _, result := range results {
		data = append(data, map[string]interface{}{
			"month": result.Month,
			"value": result.Value,
		})
	}

	// Return only real data from database - no dummy data

	return data
}

// calculateCashFlow calculates cash flow from sales and purchases data
func (dc *DashboardController) calculateCashFlow(sales, purchases []map[string]interface{}) []map[string]interface{} {
	var cashFlow []map[string]interface{}

	maxLen := len(sales)
	if len(purchases) > maxLen {
		maxLen = len(purchases)
	}

	for i := 0; i < maxLen; i++ {
		var month string
		var salesValue, purchasesValue, balance float64

		if i < len(sales) {
			month = sales[i]["month"].(string)
			salesValue = sales[i]["value"].(float64)
		}

		if i < len(purchases) {
			if month == "" {
				month = purchases[i]["month"].(string)
			}
			purchasesValue = purchases[i]["value"].(float64)
		}

		balance = salesValue - purchasesValue

		cashFlow = append(cashFlow, map[string]interface{}{
			"month":   month,
			"inflow":  salesValue,
			"outflow": purchasesValue,
			"balance": balance,
		})
	}

	return cashFlow
}

// getTopAccounts gets the top 5 accounts by balance
func (dc *DashboardController) getTopAccounts() []map[string]interface{} {
	type AccountData struct {
		Name    string  `json:"name"`
		Balance float64 `json:"balance"`
		Type    string  `json:"type"`
	}

	var results []AccountData
	dc.DB.Raw(`
		SELECT 
			name,
			ABS(balance) as balance,
			type
		FROM accounts 
		WHERE deleted_at IS NULL 
			AND is_active = true
			AND balance != 0
			AND is_header = false
		ORDER BY ABS(balance) DESC
		LIMIT 5
	`).Scan(&results)

	// Convert to interface{} slice
	var data []map[string]interface{}
	for _, result := range results {
		data = append(data, map[string]interface{}{
			"name":    result.Name,
			"balance": result.Balance,
			"type":    result.Type,
		})
	}

	// Return empty array if no accounts found - no dummy data
	return data
}

// getRecentTransactions gets recent transactions based on user role
func (dc *DashboardController) getRecentTransactions(userRole string) []map[string]interface{} {
	type TransactionData struct {
		ID            uint    `json:"id"`
		TransactionID string  `json:"transaction_id"`
		Description   string  `json:"description"`
		Amount        float64 `json:"amount"`
		Date          string  `json:"date"`
		Type          string  `json:"type"`
		AccountName   string  `json:"account_name"`
		ContactName   *string `json:"contact_name"`
		Status        string  `json:"status"`
	}

	var results []TransactionData

	// Get recent sales data
	dc.DB.Raw(`
		SELECT 
			s.id,
			s.code as transaction_id,
			s.notes as description,
			s.total_amount as amount,
			TO_CHAR(s.date, 'YYYY-MM-DD') as date,
			'SALE' as type,
			'Sales Transaction' as account_name,
			c.name as contact_name,
			s.status
		FROM sales s 
		LEFT JOIN contacts c ON s.customer_id = c.id
		WHERE s.deleted_at IS NULL
		ORDER BY s.created_at DESC
		LIMIT 5
	`).Scan(&results)

	// Get recent purchases data and append
	var purchaseResults []TransactionData
	dc.DB.Raw(`
		SELECT 
			p.id,
			p.code as transaction_id,
			p.notes as description,
			p.total_amount as amount,
			TO_CHAR(p.date, 'YYYY-MM-DD') as date,
			'PURCHASE' as type,
			'Purchase Transaction' as account_name,
			c.name as contact_name,
			p.status
		FROM purchases p 
		LEFT JOIN contacts c ON p.vendor_id = c.id
		WHERE p.deleted_at IS NULL
		ORDER BY p.created_at DESC
		LIMIT 5
	`).Scan(&purchaseResults)

	// Combine both sales and purchases
	results = append(results, purchaseResults...)

	// Convert to interface{} slice
	var data []map[string]interface{}
	for _, result := range results {
		data = append(data, map[string]interface{}{
			"id":             result.ID,
			"transaction_id": result.TransactionID,
			"description":    result.Description,
			"amount":         result.Amount,
			"date":           result.Date,
			"type":           result.Type,
			"account_name":   result.AccountName,
			"contact_name":   result.ContactName,
			"status":         result.Status,
		})
	}

	return data
}
