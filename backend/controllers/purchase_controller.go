package controllers

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/services"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PurchaseController struct {
	purchaseService *services.PurchaseService
	paymentService  *services.PaymentService
	db              *gorm.DB
	accountRepo     repositories.AccountRepository
}

func NewPurchaseController(purchaseService *services.PurchaseService, paymentService *services.PaymentService, db *gorm.DB, accountRepo repositories.AccountRepository) *PurchaseController {
	return &PurchaseController{
		purchaseService: purchaseService,
		paymentService:  paymentService,
		db:              db,
		accountRepo:     accountRepo,
	}
}

func (pc *PurchaseController) getAccessiblePurchase(c *gin.Context, purchaseID uint) (*models.Purchase, bool) {
	purchase, err := pc.purchaseService.GetPurchaseByID(purchaseID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Purchase not found"})
		return nil, false
	}

	if models.IsStaffRole(c.GetString("role")) && purchase.UserID != c.GetUint("user_id") {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied: you can only access your own purchase data",
			"code":  "INSUFFICIENT_PERMISSION",
		})
		return nil, false
	}

	return purchase, true
}

func (pc *PurchaseController) getAccessibleReceipt(c *gin.Context, receiptID uint) (*models.PurchaseReceipt, bool) {
	var receipt models.PurchaseReceipt
	if err := pc.db.Preload("Purchase").First(&receipt, receiptID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Purchase receipt not found"})
		return nil, false
	}

	if models.IsStaffRole(c.GetString("role")) && receipt.Purchase.UserID != c.GetUint("user_id") {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied: you can only access your own purchase data",
			"code":  "INSUFFICIENT_PERMISSION",
		})
		return nil, false
	}

	return &receipt, true
}

func (pc *PurchaseController) getAccessiblePurchaseDocument(c *gin.Context, documentID uint) (*models.PurchaseDocument, bool) {
	var document models.PurchaseDocument
	if err := pc.db.Preload("Purchase").First(&document, documentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Purchase document not found"})
		return nil, false
	}

	if models.IsStaffRole(c.GetString("role")) && document.Purchase.UserID != c.GetUint("user_id") {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied: you can only access your own purchase data",
			"code":  "INSUFFICIENT_PERMISSION",
		})
		return nil, false
	}

	return &document, true
}

func (pc *PurchaseController) applyStaffPurchaseScope(c *gin.Context, filter *models.PurchaseFilter) {
	if models.IsStaffRole(c.GetString("role")) {
		filter.UserID = c.GetUint("user_id")
	}
}

// Purchase CRUD Operations

// GetPurchases godoc
// @Summary Get all purchases
// @Description Get paginated list of purchases with filters
// @Tags Purchases
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Param status query string false "Purchase status filter"
// @Param vendor_id query string false "Vendor ID filter"
// @Param start_date query string false "Start date filter (YYYY-MM-DD)"
// @Param end_date query string false "End date filter (YYYY-MM-DD)"
// @Param search query string false "Search term"
// @Param approval_status query string false "Approval status filter"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/purchases [get]
func (pc *PurchaseController) GetPurchases(c *gin.Context) {
	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	status := c.Query("status")
	vendorID := c.Query("vendor_id")
	projectID := c.Query("project_id")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	search := c.Query("search")
	approvalStatus := c.Query("approval_status")

	var requiresApproval *bool
	if reqApp := c.Query("requires_approval"); reqApp != "" {
		val := reqApp == "true"
		requiresApproval = &val
	}

	filter := models.PurchaseFilter{
		Status:           status,
		VendorID:         vendorID,
		ProjectID:        projectID,
		StartDate:        startDate,
		EndDate:          endDate,
		Search:           search,
		ApprovalStatus:   approvalStatus,
		RequiresApproval: requiresApproval,
		Page:             page,
		Limit:            limit,
	}

	if models.IsStaffRole(c.GetString("role")) {
		filter.UserID = c.MustGet("user_id").(uint)
	}

	result, err := pc.purchaseService.GetPurchases(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetPurchase godoc
// @Summary Get purchase by ID
// @Description Get a single purchase by ID
// @Tags Purchases
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Purchase ID"
// @Success 200 {object} models.Purchase
// @Failure 400 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Router /api/v1/purchases/{id} [get]
func (pc *PurchaseController) GetPurchase(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase ID"})
		return
	}

	purchase, err := pc.purchaseService.GetPurchaseByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Purchase not found"})
		return
	}

	// RBAC: Employee role can only view their own purchases
	userRole, _ := c.Get("role")
	userID, _ := c.Get("user_id")
	if userRole != nil && models.IsStaffRole(userRole.(string)) {
		if userID != nil && purchase.UserID != userID.(uint) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied: You can only view your own purchase requests",
				"code":  "INSUFFICIENT_PERMISSION",
			})
			return
		}
	}

	c.JSON(http.StatusOK, purchase)
}

// CreatePurchase godoc
// @Summary Create new purchase
// @Description Create a new purchase request
// @Tags Purchases
// @Accept json
// @Produce json
// @Security Bearer
// @Param purchase body models.PurchaseCreateRequest true "Purchase data"
// @Success 201 {object} models.Purchase
// @Failure 400 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/purchases [post]
func (pc *PurchaseController) CreatePurchase(c *gin.Context) {
	var request models.PurchaseCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Debug log the incoming request
	fmt.Printf("[DEBUG] CreatePurchase - Incoming request: %+v\n", request)
	for i, item := range request.Items {
		productID := uint(0)
		if item.ProductID != nil {
			productID = *item.ProductID
		}
		fmt.Printf("[DEBUG] Item %d: ItemType=%s, ProductID=%d, Qty=%d, Price=%.2f\n", i, item.ItemType, productID, item.Quantity, item.UnitPrice)
	}

	userID := c.MustGet("user_id").(uint)

	purchase, err := pc.purchaseService.CreatePurchase(request, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, purchase)
}

// UpdatePurchase godoc
// @Summary Update purchase
// @Description Update an existing purchase
// @Tags Purchases
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Purchase ID"
// @Param purchase body models.PurchaseUpdateRequest true "Purchase update data"
// @Success 200 {object} models.Purchase
// @Failure 400 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/purchases/{id} [put]
func (pc *PurchaseController) UpdatePurchase(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase ID"})
		return
	}

	// RBAC: Employee role can only update their own purchases
	userRole, _ := c.Get("role")
	userID := c.MustGet("user_id").(uint)
	if userRole != nil && models.IsStaffRole(userRole.(string)) {
		purchase, err := pc.purchaseService.GetPurchaseByID(uint(id))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Purchase not found"})
			return
		}
		if purchase.UserID != userID {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied: You can only update your own purchase requests",
				"code":  "INSUFFICIENT_PERMISSION",
			})
			return
		}
	}

	var request models.PurchaseUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	purchase, err := pc.purchaseService.UpdatePurchase(uint(id), request, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, purchase)
}

// DeletePurchase godoc
// @Summary Delete purchase
// @Description Delete a purchase (with permission checks)
// @Tags Purchases
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Purchase ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 403 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/purchases/{id} [delete]
func (pc *PurchaseController) DeletePurchase(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase ID"})
		return
	}

	// Get user role and ID from context
	userRole := c.MustGet("user_role").(string)
	userID := c.MustGet("user_id").(uint)

	// Check if purchase exists and get its status
	purchase, err := pc.purchaseService.GetPurchaseByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Purchase not found"})
		return
	}

	// RBAC: Employee role can only delete their own purchases
	if models.IsStaffRole(userRole) && purchase.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied: You can only delete your own purchase requests",
			"code":  "INSUFFICIENT_PERMISSION",
		})
		return
	}

	// Check if user has permission to delete this purchase
	// For APPROVED purchases, only ADMIN can delete
	if purchase.Status == models.PurchaseStatusApproved && userRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admin can delete approved purchases"})
		return
	}

	// For other non-draft purchases, admin and director can delete
	if purchase.Status != models.PurchaseStatusDraft && purchase.Status != models.PurchaseStatusApproved && userRole != "admin" && userRole != "director" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to delete this purchase"})
		return
	}

	err = pc.purchaseService.DeletePurchase(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Purchase cancelled successfully"})
}

// Approval Operations

// SubmitForApproval godoc
// @Summary Submit purchase for approval
// @Description Submit a purchase for approval workflow
// @Tags Purchases
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Purchase ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/purchases/{id}/submit-approval [post]
func (pc *PurchaseController) SubmitForApproval(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase ID"})
		return
	}
	if _, ok := pc.getAccessiblePurchase(c, uint(id)); !ok {
		return
	}

	userID := c.MustGet("user_id").(uint)

	err = pc.purchaseService.SubmitForApproval(uint(id), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Purchase submitted for approval"})
}

// ApprovePurchase godoc
// @Summary Approve purchase
// @Description Approve a purchase in the workflow
// @Tags Purchases
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Purchase ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/purchases/{id}/approve [post]
func (pc *PurchaseController) ApprovePurchase(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase ID"})
		return
	}

	userID := c.MustGet("user_id").(uint)
	userRole := c.MustGet("user_role").(string)
	roleNorm := models.NormalizeRole(userRole)

	if roleNorm == models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Admin cannot approve purchases directly. Approval must be handled by the active finance manager/director workflow step.",
		})
		return
	}

	// SAFETY CHECK: Ensure purchase is not in DRAFT status
	purchase, err := pc.purchaseService.GetPurchaseByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Purchase not found"})
		return
	}
	if err := services.CheckClosedPeriod(pc.db, purchase.Date); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Periode transaksi sudah ditutup",
			"details": err.Error(),
			"code":    "PERIOD_CLOSED",
		})
		return
	}

	if purchase.Status == models.PurchaseStatusDraft {
		if models.IsFinanceManagerRole(roleNorm) {
			// Auto-submit the purchase for approval
			if err := pc.purchaseService.SubmitForApproval(uint(id), userID); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": fmt.Sprintf("Failed to auto-submit purchase for approval: %v", err),
				})
				return
			}
			// Reload purchase after submission
			purchase, err = pc.purchaseService.GetPurchaseByID(uint(id))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reload purchase after submission"})
				return
			}
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":           "Draft purchase must be submitted for finance manager approval first",
				"current_status":  purchase.Status,
				"required_action": "Use 'Submit for Approval' button first",
			})
			return
		}
	}

	// Parse request body to check for escalation
	var request struct {
		Comments           string `json:"comments"`
		EscalateToDirector bool   `json:"escalate_to_director"`
	}
	c.ShouldBindJSON(&request)

	// Process approval with escalation logic
	result, err := pc.purchaseService.ProcessPurchaseApprovalWithEscalation(
		uint(id),
		true,
		userID,
		userRole,
		request.Comments,
		request.EscalateToDirector,
	)
	if err != nil {
		c.JSON(purchaseApprovalErrorStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// RejectPurchase godoc
// @Summary Reject purchase
// @Description Reject a purchase in the workflow
// @Tags Purchases
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Purchase ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} utils.ErrorResponse
// @Failure 500 {object} utils.ErrorResponse
// @Router /api/v1/purchases/{id}/reject [post]
func (pc *PurchaseController) RejectPurchase(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase ID"})
		return
	}

	userID := c.MustGet("user_id").(uint)
	userRole := c.MustGet("user_role").(string)
	roleNorm := models.NormalizeRole(userRole)

	if roleNorm == models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Admin cannot reject purchases directly. Rejection must be handled by the active finance manager/director workflow step.",
		})
		return
	}

	// Parse request body to get comments
	var request struct {
		Comments string `json:"comments" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Comments are required for rejection: " + err.Error()})
		return
	}

	// Process rejection with escalation logic (similar to approve but with rejection)
	result, err := pc.purchaseService.ProcessPurchaseApprovalWithEscalation(
		uint(id),
		false, // false = reject
		userID,
		userRole,
		request.Comments,
		false, // no escalation for rejection
	)
	if err != nil {
		c.JSON(purchaseApprovalErrorStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func purchaseApprovalErrorStatus(err error) int {
	if err == nil {
		return http.StatusInternalServerError
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))

	switch {
	case strings.Contains(message, "no active approval step found for your role"):
		return http.StatusBadRequest
	case strings.Contains(message, "approval request is no longer pending"):
		return http.StatusBadRequest
	case strings.Contains(message, "purchase cannot be approved in current status"):
		return http.StatusBadRequest
	case strings.Contains(message, "cannot be directly approved"):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func roundMoney2(value float64) float64 {
	return math.Round(value*100) / 100
}

// Receipt Operations

// CreatePurchaseReceipt creates a new purchase receipt
func (pc *PurchaseController) CreatePurchaseReceipt(c *gin.Context) {
	var request models.PurchaseReceiptRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	if _, ok := pc.getAccessiblePurchase(c, request.PurchaseID); !ok {
		return
	}

	// Get user ID with proper error handling
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		log.Printf("Error: user_id not found in context for receipt creation")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "User not authenticated",
			"details": "user_id not found in context",
		})
		return
	}

	userID, ok := userIDInterface.(uint)
	if !ok {
		log.Printf("Error: user_id has invalid type for receipt creation: %T", userIDInterface)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Invalid user authentication",
			"details": "user_id has invalid type",
		})
		return
	}

	log.Printf("Creating receipt for purchase %d with user ID %d", request.PurchaseID, userID)

	receipt, err := pc.purchaseService.CreatePurchaseReceipt(request, userID)
	if err != nil {
		log.Printf("Error creating receipt: %v", err)

		// Check for specific error types
		errorMessage := err.Error()
		if strings.Contains(errorMessage, "fk_purchase_receipts_receiver") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid user reference",
				"details": "The user ID in your session is not valid. Please log out and log in again.",
				"user_id": userID,
			})
			return
		}

		if strings.Contains(errorMessage, "foreign key constraint") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Database constraint violation",
				"details": "There is a reference issue in the database. Please contact administrator.",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create receipt",
			"details": err.Error(),
		})
		return
	}

	log.Printf("Receipt created successfully: ID=%d, Number=%s", receipt.ID, receipt.ReceiptNumber)
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"receipt": receipt,
		"message": "Receipt created successfully",
	})
}

// GetPurchaseReceipts returns receipts for a purchase
func (pc *PurchaseController) GetPurchaseReceipts(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase ID"})
		return
	}
	if _, ok := pc.getAccessiblePurchase(c, uint(id)); !ok {
		return
	}

	// Check if only completed receipts are requested
	completedOnly := c.Query("completed_only") == "true"

	var receipts []models.PurchaseReceipt
	if completedOnly {
		// Get only completed receipts
		receipts, err = pc.purchaseService.GetCompletedPurchaseReceipts(uint(id))
	} else {
		// Get all receipts
		receipts, err = pc.purchaseService.GetPurchaseReceipts(uint(id))
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":           receipts,
		"count":          len(receipts),
		"completed_only": completedOnly,
	})
}

// GetReceiptPDF generates PDF for a specific receipt
func (pc *PurchaseController) GetReceiptPDF(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("receipt_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid receipt ID"})
		return
	}
	if _, ok := pc.getAccessibleReceipt(c, uint(id)); !ok {
		return
	}

	// Generate PDF
	pdfBytes, receipt, err := pc.purchaseService.GenerateReceiptPDF(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Set headers for PDF download
	filename := fmt.Sprintf("receipt_%s.pdf", receipt.ReceiptNumber)
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))

	// Send PDF
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}

// GetAllReceiptsPDF generates combined PDF for all receipts of a purchase
func (pc *PurchaseController) GetAllReceiptsPDF(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase ID"})
		return
	}
	if _, ok := pc.getAccessiblePurchase(c, uint(id)); !ok {
		return
	}

	// Generate combined PDF
	pdfBytes, purchase, err := pc.purchaseService.GenerateAllReceiptsPDF(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Set headers for PDF download
	filename := fmt.Sprintf("receipts_%s.pdf", purchase.Code)
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))

	// Send PDF
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}

// Document Operations

// UploadDocument uploads a document for a purchase
func (pc *PurchaseController) UploadDocument(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase ID"})
		return
	}
	if _, ok := pc.getAccessiblePurchase(c, uint(id)); !ok {
		return
	}

	// Handle file upload
	file, header, err := c.Request.FormFile("document")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()

	documentType := c.PostForm("document_type")
	if documentType == "" {
		documentType = models.PurchaseDocumentInvoice
	}

	userID := c.MustGet("user_id").(uint)

	// In a real implementation, you would save the file to storage
	// For now, we'll simulate the file path
	filePath := "/uploads/purchases/" + header.Filename

	err = pc.purchaseService.UploadDocument(
		uint(id),
		documentType,
		header.Filename,
		filePath,
		header.Size,
		header.Header.Get("Content-Type"),
		userID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Document uploaded successfully"})
}

// GetPurchaseDocuments returns documents for a purchase
func (pc *PurchaseController) GetPurchaseDocuments(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase ID"})
		return
	}
	if _, ok := pc.getAccessiblePurchase(c, uint(id)); !ok {
		return
	}

	documents, err := pc.purchaseService.GetPurchaseDocuments(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, documents)
}

// DeleteDocument deletes a purchase document
func (pc *PurchaseController) DeleteDocument(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("document_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid document ID"})
		return
	}
	if _, ok := pc.getAccessiblePurchaseDocument(c, uint(id)); !ok {
		return
	}

	err = pc.purchaseService.DeleteDocument(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Document deleted successfully"})
}

// Three-way Matching Operations

// GetPurchaseMatching returns matching data for three-way matching
func (pc *PurchaseController) GetPurchaseMatching(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase ID"})
		return
	}
	if _, ok := pc.getAccessiblePurchase(c, uint(id)); !ok {
		return
	}

	matching, err := pc.purchaseService.GetPurchaseMatching(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, matching)
}

// ValidateThreeWayMatching validates three-way matching
func (pc *PurchaseController) ValidateThreeWayMatching(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase ID"})
		return
	}
	if _, ok := pc.getAccessiblePurchase(c, uint(id)); !ok {
		return
	}

	isValid, err := pc.purchaseService.ValidateThreeWayMatching(uint(id))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":    err.Error(),
			"is_valid": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"is_valid": isValid,
		"message":  "Three-way matching validation completed",
	})
}

// Analytics and Reporting Operations

// GetPurchasesSummary returns purchase summary statistics
func (pc *PurchaseController) GetPurchasesSummary(c *gin.Context) {
	status := c.Query("status")
	vendorID := c.Query("vendor_id")
	projectID := c.Query("project_id")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	search := c.Query("search")
	approvalStatus := c.Query("approval_status")

	var requiresApproval *bool
	if reqApp := c.Query("requires_approval"); reqApp != "" {
		val := reqApp == "true"
		requiresApproval = &val
	}

	filter := models.PurchaseFilter{
		Status:           status,
		VendorID:         vendorID,
		ProjectID:        projectID,
		StartDate:        startDate,
		EndDate:          endDate,
		Search:           search,
		ApprovalStatus:   approvalStatus,
		RequiresApproval: requiresApproval,
	}
	pc.applyStaffPurchaseScope(c, &filter)

	summary, err := pc.purchaseService.GetPurchasesSummary(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// GetVendorPurchaseSummary returns purchase summary for a specific vendor
func (pc *PurchaseController) GetVendorPurchaseSummary(c *gin.Context) {
	vendorID, err := strconv.ParseUint(c.Param("vendor_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid vendor ID"})
		return
	}

	if models.IsStaffRole(c.GetString("role")) {
		filter := models.PurchaseFilter{
			VendorID: strconv.FormatUint(vendorID, 10),
			Page:     1,
			Limit:    1,
		}
		pc.applyStaffPurchaseScope(c, &filter)

		result, err := pc.purchaseService.GetPurchases(filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		summary, err := pc.purchaseService.GetPurchasesSummary(filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		vendorName := ""
		if len(result.Data) > 0 {
			vendorName = result.Data[0].Vendor.Name
		}

		c.JSON(http.StatusOK, &models.VendorPurchaseSummary{
			VendorID:    uint(vendorID),
			VendorName:  vendorName,
			TotalAmount: summary.TotalAmount,
			TotalOrders: summary.TotalPurchases,
		})
		return
	}

	summary, err := pc.purchaseService.GetVendorPurchaseSummary(uint(vendorID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// GetPendingApprovals returns purchases pending approval for current user
func (pc *PurchaseController) GetPendingApprovals(c *gin.Context) {
	_ = c.MustGet("user_id").(uint) // userID not used in current implementation
	userRole := c.MustGet("role").(string)

	// Filter purchases requiring approval that user can approve
	filter := models.PurchaseFilter{
		ApprovalStatus: models.PurchaseApprovalPending,
		Page:           1,
		Limit:          100,
	}

	result, err := pc.purchaseService.GetPurchases(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Filter based on user role - in a real app, this would be more sophisticated
	var filteredPurchases []models.Purchase
	for _, purchase := range result.Data {
		// Finance manager can approve finance-step purchases
		if models.IsFinanceManagerRole(userRole) {
			filteredPurchases = append(filteredPurchases, purchase)
		}
		// Director can approve all purchases (removed amount restriction)
		if userRole == models.RoleDirector {
			filteredPurchases = append(filteredPurchases, purchase)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      filteredPurchases,
		"total":     len(filteredPurchases),
		"user_role": userRole,
	})
}

// Purchase Payment Integration Operations

// GetPurchaseForPayment godoc
// @Summary Get purchase for payment
// @Description Get purchase details formatted for payment processing
// @Tags Purchases
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Purchase ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/purchases/{id}/for-payment [get]
func (pc *PurchaseController) GetPurchaseForPayment(c *gin.Context) {
	purchaseID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase ID"})
		return
	}

	purchase, ok := pc.getAccessiblePurchase(c, uint(purchaseID))
	if !ok {
		return
	}

	// Check if purchase can receive payment
	canReceivePayment := models.IsPurchasePaymentEligible(purchase.Status, purchase.PaymentMethod, purchase.OutstandingAmount)

	// Format response for payment processing
	response := gin.H{
		"purchase_id": purchase.ID,
		"bill_number": purchase.Code,
		"vendor": gin.H{
			"id":   purchase.Vendor.ID,
			"name": purchase.Vendor.Name,
			"type": "VENDOR",
		},
		"total_amount":        purchase.TotalAmount,
		"paid_amount":         purchase.PaidAmount,
		"outstanding_amount":  purchase.OutstandingAmount,
		"status":              purchase.Status,
		"payment_method":      purchase.PaymentMethod,
		"date":                purchase.Date.Format("2006-01-02"),
		"can_receive_payment": canReceivePayment,
	}

	if !purchase.DueDate.IsZero() {
		response["due_date"] = purchase.DueDate.Format("2006-01-02")
	}

	if canReceivePayment {
		response["payment_url_suggestion"] = fmt.Sprintf("/api/purchases/%d/integrated-payment", purchase.ID)
	}

	c.JSON(http.StatusOK, response)
}

// CreateIntegratedPayment godoc
// @Summary Create integrated payment for purchase
// @Description Create payment record in both Purchase and Payment Management systems
// @Tags Purchases
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Purchase ID"
// @Param payment body models.APIResponse true "Payment details"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/purchases/{id}/integrated-payment [post]
func (pc *PurchaseController) CreateIntegratedPayment(c *gin.Context) {
	purchaseID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase ID"})
		return
	}
	if _, ok := pc.getAccessiblePurchase(c, uint(purchaseID)); !ok {
		return
	}

	var request struct {
		Amount     float64 `json:"amount" binding:"required,gt=0"`
		Date       string  `json:"date" binding:"required"`
		Method     string  `json:"method" binding:"required"`
		CashBankID *uint   `json:"cash_bank_id"`
		Reference  string  `json:"reference"`
		Notes      string  `json:"notes"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.MustGet("user_id").(uint)

	// Parse date
	paymentDate, err := time.Parse(time.RFC3339, request.Date)
	if err != nil {
		// Try alternative date format
		paymentDate, err = time.Parse("2006-01-02", request.Date)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format. Use RFC3339 or YYYY-MM-DD"})
			return
		}
	}

	// Create integrated payment via service
	result, err := pc.purchaseService.CreateIntegratedPayment(
		uint(purchaseID),
		request.Amount,
		paymentDate,
		request.Method,
		request.CashBankID,
		request.Reference,
		request.Notes,
		userID,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to create payment",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, result)
}

// GetPurchasePayments godoc
// @Summary Get purchase payments
// @Description Get all payments for a specific purchase
// @Tags Purchases
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "Purchase ID"
// @Success 200 {array} models.PurchasePayment
// @Router /api/v1/purchases/{id}/payments [get]
func (pc *PurchaseController) GetPurchasePayments(c *gin.Context) {
	purchaseID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase ID"})
		return
	}
	if _, ok := pc.getAccessiblePurchase(c, uint(purchaseID)); !ok {
		return
	}

	payments, err := pc.purchaseService.GetPurchasePayments(uint(purchaseID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, payments)
}

// CreatePurchasePayment creates a payment for a purchase via Payment Management
func (pc *PurchaseController) CreatePurchasePayment(c *gin.Context) {
	purchaseID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase ID"})
		return
	}
	purchase, ok := pc.getAccessiblePurchase(c, uint(purchaseID))
	if !ok {
		return
	}

	var request struct {
		Amount       float64 `json:"amount" binding:"required,gt=0"`
		ExchangeRate float64 `json:"exchange_rate"`
		Date         string  `json:"date" binding:"required"`
		Method       string  `json:"method" binding:"required"`
		CashBankID   uint    `json:"cash_bank_id" binding:"required"`
		Reference    string  `json:"reference"`
		Notes        string  `json:"notes"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		log.Printf("Payment creation validation error for purchase %d: %v", purchaseID, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
			"expected_fields": map[string]string{
				"amount":        "number (required, min=0)",
				"exchange_rate": "number (optional, required for USD cash/bank)",
				"date":          "datetime string (required, ISO format)",
				"method":        "string (required)",
				"cash_bank_id":  "number (required)",
				"reference":     "string (optional)",
				"notes":         "string (optional)",
			},
		})
		return
	}

	// Log successful request parsing
	log.Printf("Received payment request for purchase %d: amount=%.2f, method=%s, cash_bank_id=%d, date=%s", purchaseID, request.Amount, request.Method, request.CashBankID, request.Date)

	// Parse date - support both RFC3339 and YYYY-MM-DD formats
	paymentDate, err := time.Parse(time.RFC3339, request.Date)
	if err != nil {
		// Try alternative date format (YYYY-MM-DD)
		paymentDate, err = time.Parse("2006-01-02", request.Date)
		if err != nil {
			log.Printf("Date parsing error for purchase %d: %v, date=%s", purchaseID, err, request.Date)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":         "Invalid date format",
				"details":       "Date must be in RFC3339 format (2006-01-02T15:04:05Z) or YYYY-MM-DD format",
				"received_date": request.Date,
			})
			return
		}
		// If we parsed YYYY-MM-DD format, set to start of day
		paymentDate = time.Date(paymentDate.Year(), paymentDate.Month(), paymentDate.Day(), 0, 0, 0, 0, time.UTC)
	}
	log.Printf("Parsed payment date: %v", paymentDate)

	// Validate purchase status using the same payment-eligibility rule exposed to the UI.
	if !models.IsPurchasePaymentEligible(purchase.Status, purchase.PaymentMethod, purchase.OutstandingAmount) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":           "Purchase is not eligible to receive payment",
			"purchase_status": purchase.Status,
		})
		return
	}

	if !models.IsCreditPurchaseMethod(purchase.PaymentMethod) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":          "Only credit purchases can receive payments",
			"payment_method": purchase.PaymentMethod,
		})
		return
	}

	// Validate payment amount
	requestedAmount := roundMoney2(request.Amount)
	outstandingAmount := roundMoney2(purchase.OutstandingAmount)
	if requestedAmount-outstandingAmount > 0.005 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":              "Payment amount exceeds outstanding amount",
			"outstanding_amount": outstandingAmount,
			"requested_amount":   requestedAmount,
		})
		return
	}
	if math.Abs(requestedAmount-outstandingAmount) <= 0.005 {
		requestedAmount = outstandingAmount
	}

	// Get user ID with error handling
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		log.Printf("Error: user_id not found in context for purchase %d payment", purchaseID)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "User not authenticated",
			"details": "user_id not found in context",
		})
		return
	}
	userID, ok := userIDInterface.(uint)
	if !ok {
		log.Printf("Error: user_id has invalid type for purchase %d payment: %T", purchaseID, userIDInterface)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Invalid user authentication",
			"details": "user_id has invalid type",
		})
		return
	}

	// Create payment request for Payment Management service
	paymentRequest := services.PaymentCreateRequest{
		ContactID:    purchase.VendorID,
		CashBankID:   request.CashBankID,
		Date:         paymentDate, // Use the parsed paymentDate
		Amount:       requestedAmount,
		ExchangeRate: request.ExchangeRate,
		Method:       request.Method,
		Reference:    request.Reference,
		Notes:        fmt.Sprintf("Payment for Purchase %s - %s", purchase.Code, request.Notes),
		BillAllocations: []services.BillAllocation{
			{
				BillID: uint(purchaseID),
				Amount: requestedAmount,
			},
		},
	}

	// Use Payment Management service
	log.Printf("Calling PaymentService.CreatePayablePayment for purchase %d with amount %.2f", purchaseID, paymentRequest.Amount)
	payment, err := pc.paymentService.CreatePayablePayment(paymentRequest, userID)
	if err != nil {
		log.Printf("Error in CreatePayablePayment for purchase %d: %v", purchaseID, err)

		// Check if this is an insufficient balance error
		errorMessage := err.Error()
		if strings.Contains(errorMessage, "insufficient balance") {
			// Extract available balance from error message
			c.JSON(http.StatusBadRequest, gin.H{
				"success":          false,
				"error":            "Saldo rekening tidak mencukupi",
				"error_type":       "INSUFFICIENT_BALANCE",
				"details":          errorMessage,
				"requested_amount": requestedAmount,
				"status":           "error",
				"message":          "Saldo di rekening bank yang dipilih tidak mencukupi untuk melakukan pembayaran ini",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to create payment",
				"details": err.Error(),
				"status":  "error",
			})
		}
		return
	}
	log.Printf("✅ Payment created successfully: ID=%d, Code=%s", payment.ID, payment.Code)

	// 🛑 REMOVED: The duplicate SSOT journal call that was causing DOUBLE POSTING
	// CreatePayablePayment (called above at line 1073) already:
	//   1. Updates cash_banks.balance via updateCashBankBalance
	//   2. Creates journal entries AND updates accounts.balance via
	//      purchasePaymentJournalService.CreatePaymentJournalEntryWithTx → createLegacyPaymentJournalWithTx
	// Calling CreatePurchasePaymentJournal here would create a SECOND balance update = double deduction!
	log.Printf("✅ Journal entries and COA balance already handled by CreatePayablePayment")

	// CRITICAL FIX: Update purchase payment amounts after successful payment
	log.Printf("🔄 Updating purchase payment amounts for purchase %d...", purchaseID)

	// Calculate new paid and outstanding amounts
	newPaidAmount := roundMoney2(purchase.PaidAmount + requestedAmount)
	newOutstandingAmount := roundMoney2(purchase.TotalAmount - newPaidAmount)

	// DO NOT change status to PAID automatically when payment is made
	// The purchase should remain APPROVED to allow receipt creation
	// Only change to COMPLETED when both payment AND receipt are complete
	newStatus := purchase.Status
	if math.Abs(newOutstandingAmount) <= 0.005 {
		newOutstandingAmount = 0
	}
	if newOutstandingAmount < 0 {
		newOutstandingAmount = 0 // Ensure it doesn't go negative
	}
	// Status remains APPROVED - will be changed to COMPLETED only when receipt is created

	log.Printf("📊 Payment amounts: Paid %.2f → %.2f, Outstanding %.2f → %.2f, Status %s → %s",
		purchase.PaidAmount, newPaidAmount, purchase.OutstandingAmount, newOutstandingAmount, purchase.Status, newStatus)

	// Update purchase in database
	err = pc.purchaseService.UpdatePurchasePaymentAmounts(uint(purchaseID), newPaidAmount, newOutstandingAmount, newStatus)
	if err != nil {
		log.Printf("❌ Critical error: Failed to update purchase payment amounts: %v", err)
		// Payment was created successfully, but purchase amounts weren't updated
		c.JSON(http.StatusCreated, gin.H{
			"success":       true,
			"payment":       payment,
			"warning":       "Payment created but purchase amounts could not be updated",
			"error_details": err.Error(),
			"status":        "partial_success",
			"message":       "Payment recorded but purchase status may not reflect the payment. Please refresh the page.",
		})
		return
	}

	log.Printf("✅ Purchase payment amounts updated successfully")

	// Return response with both payment info and updated purchase status
	updatedPurchase, err := pc.purchaseService.GetPurchaseByID(uint(purchaseID))
	if err != nil {
		// If we can't get updated purchase info, still return success but with calculated values
		log.Printf("Warning: Could not fetch updated purchase info after payment creation: %v", err)
		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"payment": payment,
			"updated_purchase": gin.H{
				"id":                 uint(purchaseID),
				"status":             newStatus,
				"paid_amount":        newPaidAmount,
				"outstanding_amount": newOutstandingAmount,
			},
			"message": "Payment created successfully via Payment Management",
			"status":  "success",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"payment": payment,
		"updated_purchase": gin.H{
			"id":                 updatedPurchase.ID,
			"status":             updatedPurchase.Status,
			"paid_amount":        updatedPurchase.PaidAmount,
			"outstanding_amount": updatedPurchase.OutstandingAmount,
		},
		"message": "Payment created successfully via Payment Management",
		"status":  "success",
	})
}

// Dashboard endpoint for purchases
func (pc *PurchaseController) GetPurchaseDashboard(c *gin.Context) {
	userRole := c.MustGet("role").(string)
	userID := c.MustGet("user_id").(uint)

	summaryFilter := models.PurchaseFilter{}
	if models.IsStaffRole(userRole) {
		summaryFilter.UserID = userID
	}

	// Get summary
	summary, err := pc.purchaseService.GetPurchasesSummary(summaryFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get pending approvals count
	pendingFilter := models.PurchaseFilter{
		ApprovalStatus: models.PurchaseApprovalPending,
		Page:           1,
		Limit:          1000,
	}
	if models.IsStaffRole(userRole) {
		pendingFilter.UserID = userID
	}

	pendingResult, err := pc.purchaseService.GetPurchases(pendingFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := gin.H{
		"summary":       summary,
		"pending_count": len(pendingResult.Data),
		"user_role":     userRole,
		"can_approve":   models.IsFinanceManagerRole(userRole) || userRole == models.RoleDirector,
	}

	c.JSON(http.StatusOK, response)
}

// GetPurchaseJournalEntries retrieves SSOT journal entries for a purchase
func (pc *PurchaseController) GetPurchaseJournalEntries(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid purchase ID"})
		return
	}
	if _, ok := pc.getAccessiblePurchase(c, uint(id)); !ok {
		return
	}

	// Get journal entries for the purchase
	entries, err := pc.purchaseService.GetPurchaseJournalEntries(uint(id))
	if err != nil {
		log.Printf("⚠️ GetPurchaseJournalEntries failed for purchase %d: %v. Returning empty result.", id, err)
		c.JSON(http.StatusOK, gin.H{
			"purchase_id":     id,
			"journal_entries": []interface{}{},
			"count":           0,
			"status":          "empty",
			"message":         "No journal entries available or journal system disabled",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"purchase_id":     id,
		"journal_entries": entries,
		"count":           len(entries),
	})
}

// Export Operations

// ExportPurchasesReportPDF exports purchases report as PDF with filtering support
func (pc *PurchaseController) ExportPurchasesReportPDF(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	status := c.Query("status")
	vendorID := c.Query("vendor_id")
	projectID := c.Query("project_id")
	search := c.Query("search")
	approvalStatus := c.Query("approval_status")

	// If no dates provided, default to last 30 days to keep report size reasonable
	if startDate == "" && endDate == "" {
		end := time.Now()
		start := end.AddDate(0, 0, -30)
		startDate = start.Format("2006-01-02")
		endDate = end.Format("2006-01-02")
	}

	// Build filter with all parameters to match current filtering behavior
	filter := models.PurchaseFilter{
		Status:         status,
		VendorID:       vendorID,
		ProjectID:      projectID,
		StartDate:      startDate,
		EndDate:        endDate,
		Search:         search,
		ApprovalStatus: approvalStatus,
		Page:           1,
		Limit:          10000, // Large limit for export
	}
	pc.applyStaffPurchaseScope(c, &filter)

	// Get filtered purchases
	result, err := pc.purchaseService.GetPurchases(filter)
	if err != nil {
		log.Printf("❌ Error getting purchases for PDF export: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve purchases for export", "details": err.Error()})
		return
	}

	// Initialize purchase report export service
	exportService := services.NewPurchaseReportExportService(pc.purchaseService.GetDB())

	// Convert purchases to report data format
	reportData := &services.PurchaseReportData{
		Company: services.CompanyInfo{
			Name:    "PT. Sistem Akuntansi Indonesia",
			Address: "Jakarta, Indonesia",
			Phone:   "+62-21-1234567",
			Email:   "info@akuntansi.id",
		},
		StartDate:           parseDate(startDate),
		EndDate:             parseDate(endDate),
		GeneratedAt:         time.Now(),
		TotalPurchases:      int64(len(result.Data)),
		CompletedPurchases:  countCompletedPurchases(result.Data),
		TotalAmount:         calculateTotalAmount(result.Data),
		TotalPaid:           calculateTotalPaid(result.Data),
		OutstandingPayables: calculateOutstandingPayables(result.Data),
		PurchasesByVendor:   groupPurchasesByVendor(result.Data),
	}

	// Generate PDF
	userID := c.MustGet("user_id").(uint)
	pdfBytes, err := exportService.ExportToPDF(reportData, userID)
	if err != nil {
		log.Printf("❌ Error generating purchases PDF: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate purchases PDF", "details": err.Error()})
		return
	}

	// Set response headers
	filename := fmt.Sprintf("purchases-report_%s_to_%s.pdf", startDate, endDate)
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}

// ExportPurchasesReportCSV exports purchases report as CSV with filtering support
func (pc *PurchaseController) ExportPurchasesReportCSV(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	status := c.Query("status")
	vendorID := c.Query("vendor_id")
	projectID := c.Query("project_id")
	search := c.Query("search")
	approvalStatus := c.Query("approval_status")

	// If no dates provided, default to last 30 days to keep report size reasonable
	if startDate == "" && endDate == "" {
		end := time.Now()
		start := end.AddDate(0, 0, -30)
		startDate = start.Format("2006-01-02")
		endDate = end.Format("2006-01-02")
	}

	// Build filter with all parameters to match current filtering behavior
	filter := models.PurchaseFilter{
		Status:         status,
		VendorID:       vendorID,
		ProjectID:      projectID,
		StartDate:      startDate,
		EndDate:        endDate,
		Search:         search,
		ApprovalStatus: approvalStatus,
		Page:           1,
		Limit:          10000, // Large limit for export
	}
	pc.applyStaffPurchaseScope(c, &filter)

	// Get filtered purchases
	result, err := pc.purchaseService.GetPurchases(filter)
	if err != nil {
		log.Printf("❌ Error getting purchases for CSV export: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve purchases for export", "details": err.Error()})
		return
	}

	// Initialize purchase report export service
	exportService := services.NewPurchaseReportExportService(pc.purchaseService.GetDB())

	// Convert purchases to report data format
	reportData := &services.PurchaseReportData{
		Company: services.CompanyInfo{
			Name:    "PT. Sistem Akuntansi Indonesia",
			Address: "Jakarta, Indonesia",
			Phone:   "+62-21-1234567",
			Email:   "info@akuntansi.id",
		},
		StartDate:           parseDate(startDate),
		EndDate:             parseDate(endDate),
		GeneratedAt:         time.Now(),
		TotalPurchases:      int64(len(result.Data)),
		CompletedPurchases:  countCompletedPurchases(result.Data),
		TotalAmount:         calculateTotalAmount(result.Data),
		TotalPaid:           calculateTotalPaid(result.Data),
		OutstandingPayables: calculateOutstandingPayables(result.Data),
		PurchasesByVendor:   groupPurchasesByVendor(result.Data),
	}

	// Generate CSV
	userID := c.MustGet("user_id").(uint)
	csvBytes, err := exportService.ExportToCSV(reportData, userID)
	if err != nil {
		log.Printf("❌ Error generating purchases CSV: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate purchases CSV", "details": err.Error()})
		return
	}

	// Set response headers
	filename := fmt.Sprintf("purchases-report_%s_to_%s.csv", startDate, endDate)
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "text/csv", csvBytes)
}

// Helper functions for export functionality

// parseDate parses date string, returns current time if empty
func parseDate(dateStr string) time.Time {
	if dateStr == "" {
		return time.Now()
	}
	parsed, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return time.Now()
	}
	return parsed
}

// countCompletedPurchases counts purchases with status COMPLETED
func countCompletedPurchases(purchases []models.Purchase) int64 {
	count := 0
	for _, p := range purchases {
		if p.Status == models.PurchaseStatusCompleted || p.Status == models.PurchaseStatusPaid {
			count++
		}
	}
	return int64(count)
}

// calculateTotalAmount calculates total amount of all purchases
func calculateTotalAmount(purchases []models.Purchase) float64 {
	total := 0.0
	for _, p := range purchases {
		total += p.TotalAmount
	}
	return total
}

// calculateTotalPaid calculates total paid amount of all purchases
func calculateTotalPaid(purchases []models.Purchase) float64 {
	total := 0.0
	for _, p := range purchases {
		total += p.PaidAmount
	}
	return total
}

// calculateOutstandingPayables calculates total outstanding payables
func calculateOutstandingPayables(purchases []models.Purchase) float64 {
	total := 0.0
	for _, p := range purchases {
		total += p.OutstandingAmount
	}
	return total
}

// groupPurchasesByVendor groups purchases by vendor for reporting
func groupPurchasesByVendor(purchases []models.Purchase) []services.VendorPurchaseSummary {
	vendorMap := make(map[uint64]*services.VendorPurchaseSummary)

	for _, p := range purchases {
		if p.Vendor.ID == 0 {
			continue
		}

		vendorID := uint64(p.Vendor.ID)
		if existing, ok := vendorMap[vendorID]; ok {
			existing.TotalPurchases += 1
			existing.TotalAmount += p.TotalAmount
			existing.TotalPaid += p.PaidAmount
			existing.Outstanding += p.OutstandingAmount
			if p.Date.After(existing.LastPurchaseDate) {
				existing.LastPurchaseDate = p.Date
			}
		} else {
			vendorMap[vendorID] = &services.VendorPurchaseSummary{
				VendorID:         vendorID,
				VendorName:       p.Vendor.Name,
				TotalPurchases:   1, // int64 will be set below
				TotalAmount:      p.TotalAmount,
				TotalPaid:        p.PaidAmount,
				Outstanding:      p.OutstandingAmount,
				LastPurchaseDate: p.Date,
				PaymentMethod:    p.PaymentMethod,
				Status:           p.Status,
			}
		}
	}

	// Convert map to slice
	result := make([]services.VendorPurchaseSummary, 0, len(vendorMap))
	for _, vendor := range vendorMap {
		result = append(result, *vendor)
	}

	return result
}
