package controllers

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type PurchaseRequestController struct {
	service services.PurchaseRequestService
}

func NewPurchaseRequestController(service services.PurchaseRequestService) *PurchaseRequestController {
	return &PurchaseRequestController{
		service: service,
	}
}

// Create PR
func (c *PurchaseRequestController) Create(ctx *gin.Context) {
	// Require valid payload
	var payload models.PurchaseRequestCreatePayload
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Assuming Auth middleware injects user_id into context
	userID, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized request"})
		return
	}

	userRole := ctx.GetString("role")

	pr, err := c.service.CreatePurchaseRequest(payload, userID.(uint), userRole)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, pr)
}

// List all PRs (finance / requester viewer)
func (c *PurchaseRequestController) GetAll(ctx *gin.Context) {
	dept := ctx.Query("department")
	status := ctx.Query("status")

	userID := ctx.MustGet("user_id").(uint)
	userRole := ctx.GetString("role")

	prs, err := c.service.GetPurchaseRequests(userID, userRole, dept, status)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, prs)
}

// Get Single PR Detail
func (c *PurchaseRequestController) GetByID(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID format"})
		return
	}

	userID := ctx.MustGet("user_id").(uint)
	userRole := ctx.GetString("role")

	pr, err := c.service.GetPurchaseRequestByID(uint(id), userID, userRole)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Purchase Request not found"})
		return
	}

	ctx.JSON(http.StatusOK, pr)
}

// Finance review endpoint with item-level decisions
func (c *PurchaseRequestController) Review(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID format"})
		return
	}

	var payload models.PurchaseRequestReviewPayload
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reviewerID, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: missing user context"})
		return
	}

	role := ctx.GetString("role")
	if !models.CanReviewPurchaseRequests(role) {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only finance staff, finance manager, or director can review purchase requests"})
		return
	}

	message, err := c.service.ReviewPurchaseRequest(uint(id), reviewerID.(uint), role, payload)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": message})
}

// Finance Approval Endpoint
func (c *PurchaseRequestController) Approve(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID format"})
		return
	}

	approverID, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: missing user context"})
		return
	}
	role := ctx.GetString("role")
	if !models.CanReviewPurchaseRequests(role) {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only finance staff, finance manager, or director can approve purchase requests"})
		return
	}

	message, err := c.service.ApprovePurchaseRequest(uint(id), approverID.(uint), role)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": message})
}

// Finance Rejection Endpoint
func (c *PurchaseRequestController) Reject(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID format"})
		return
	}

	var payload struct {
		Reason string `json:"reason" binding:"required"`
	}

	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	approverID, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: missing user context"})
		return
	}
	role := ctx.GetString("role")
	if !models.CanReviewPurchaseRequests(role) {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only finance staff, finance manager, or director can reject purchase requests"})
		return
	}

	message, err := c.service.RejectPurchaseRequest(uint(id), approverID.(uint), role, payload.Reason)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": message})
}
