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

	pr, err := c.service.CreatePurchaseRequest(payload, userID.(uint))
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

	prs, err := c.service.GetPurchaseRequests(dept, status)
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

	pr, err := c.service.GetPurchaseRequestByID(uint(id))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Purchase Request not found"})
		return
	}

	ctx.JSON(http.StatusOK, pr)
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

	err = c.service.ApprovePurchaseRequest(uint(id), approverID.(uint))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Purchase request approved successfully"})
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

	err = c.service.RejectPurchaseRequest(uint(id), approverID.(uint), payload.Reason)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Purchase request rejected successfully"})
}
