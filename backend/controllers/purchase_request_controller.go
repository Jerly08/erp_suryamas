package controllers

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"

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
	var documentFiles []*multipart.FileHeader
	if strings.HasPrefix(strings.ToLower(ctx.GetHeader("Content-Type")), "multipart/form-data") {
		payloadJSON := ctx.PostForm("payload")
		if payloadJSON == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "payload is required"})
			return
		}

		if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload JSON", "details": err.Error()})
			return
		}

		form, err := ctx.MultipartForm()
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart form", "details": err.Error()})
			return
		}
		documentFiles = append(documentFiles, form.File["documents"]...)
		documentFiles = append(documentFiles, form.File["attachments"]...)
	} else {
		if err := ctx.ShouldBindJSON(&payload); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	if len(documentFiles) > 0 {
		if err := validatePurchaseRequestDocumentFiles(documentFiles); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	if err := validatePurchaseRequestPayload(payload); err != nil {
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

	if len(documentFiles) > 0 {
		documents, err := c.savePurchaseRequestDocuments(ctx, pr.ID, documentFiles, userID.(uint), userRole)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		pr.Documents = documents
	}

	ctx.JSON(http.StatusCreated, pr)
}

// List all PRs (finance / requester viewer)
func (c *PurchaseRequestController) GetAll(ctx *gin.Context) {
	dept := ctx.Query("department")
	status := ctx.Query("status")
	availableForPurchase := strings.EqualFold(ctx.Query("available_for_purchase"), "true") ||
		ctx.Query("available_for_purchase") == "1"
	currentPurchaseID, _ := strconv.ParseUint(ctx.Query("current_purchase_id"), 10, 32)

	userID := ctx.MustGet("user_id").(uint)
	userRole := ctx.GetString("role")

	prs, err := c.service.GetPurchaseRequests(userID, userRole, dept, status, availableForPurchase, uint(currentPurchaseID))
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

func (c *PurchaseRequestController) UploadDocuments(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID format"})
		return
	}

	userID := ctx.MustGet("user_id").(uint)
	userRole := ctx.GetString("role")

	if _, err := c.service.GetPurchaseRequestByID(uint(id), userID, userRole); err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Purchase Request not found"})
		return
	}

	form, err := ctx.MultipartForm()
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart form", "details": err.Error()})
		return
	}

	documentFiles := append([]*multipart.FileHeader{}, form.File["documents"]...)
	documentFiles = append(documentFiles, form.File["attachments"]...)
	documentFiles = append(documentFiles, form.File["document"]...)
	if len(documentFiles) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "No files uploaded"})
		return
	}

	documents, err := c.savePurchaseRequestDocuments(ctx, uint(id), documentFiles, userID, userRole)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reapprovalStarted, err := c.service.SubmitAdditionalDocumentsForApproval(uint(id), userID, userRole, len(documents))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"message":            "Documents submitted successfully",
		"documents":          documents,
		"reapproval_started": reapprovalStarted,
	})
}

func (c *PurchaseRequestController) GetDocuments(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID format"})
		return
	}

	userID := ctx.MustGet("user_id").(uint)
	userRole := ctx.GetString("role")

	documents, err := c.service.GetPurchaseRequestDocuments(uint(id), userID, userRole)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Purchase Request documents not found"})
		return
	}

	ctx.JSON(http.StatusOK, documents)
}

func (c *PurchaseRequestController) DeleteDocument(ctx *gin.Context) {
	idStr := ctx.Param("document_id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid document ID format"})
		return
	}

	userID := ctx.MustGet("user_id").(uint)
	userRole := ctx.GetString("role")

	if err := c.service.DeletePurchaseRequestDocument(uint(id), userID, userRole); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Document deleted successfully"})
}

func (c *PurchaseRequestController) savePurchaseRequestDocuments(ctx *gin.Context, purchaseRequestID uint, files []*multipart.FileHeader, userID uint, userRole string) ([]models.PurchaseRequestDocument, error) {
	const uploadDir = "./uploads/purchase-requests"
	documentType := ctx.PostForm("document_type")
	if documentType == "" {
		documentType = models.PurchaseRequestDocumentSupporting
	}

	documents := make([]models.PurchaseRequestDocument, 0, len(files))
	savedPaths := make([]string, 0, len(files))

	for _, fileHeader := range files {
		filename, filePath, mimeType, err := prepareValidatedDocumentUpload(fileHeader, purchaseRequestID, uploadDir)
		if err != nil {
			cleanupUploadedFiles(savedPaths)
			return nil, err
		}

		if err := ctx.SaveUploadedFile(fileHeader, filePath); err != nil {
			cleanupUploadedFiles(savedPaths)
			return nil, err
		}
		savedPaths = append(savedPaths, filePath)

		document, err := c.service.UploadPurchaseRequestDocument(
			purchaseRequestID,
			documentType,
			fileHeader.Filename,
			"/uploads/purchase-requests/"+filename,
			fileHeader.Size,
			mimeType,
			userID,
			userRole,
		)
		if err != nil {
			cleanupUploadedFiles(savedPaths)
			return nil, err
		}

		documents = append(documents, *document)
	}

	return documents, nil
}

func validatePurchaseRequestDocumentFiles(files []*multipart.FileHeader) error {
	const uploadDir = "./uploads/purchase-requests"
	for _, fileHeader := range files {
		if _, _, _, err := prepareValidatedDocumentUpload(fileHeader, 0, uploadDir); err != nil {
			return err
		}
	}
	return nil
}

func validatePurchaseRequestPayload(payload models.PurchaseRequestCreatePayload) error {
	if len(payload.Items) == 0 {
		return fmt.Errorf("items are required")
	}

	for index, item := range payload.Items {
		if item.ProductID == 0 && strings.TrimSpace(item.ItemName) == "" {
			return fmt.Errorf("product_id or item_name is required for item %d", index+1)
		}
		if item.Quantity < 1 {
			return fmt.Errorf("quantity must be at least 1 for item %d", index+1)
		}
		if item.EstimatedPrice < 0 {
			return fmt.Errorf("estimated_price cannot be negative for item %d", index+1)
		}
	}

	return nil
}

func cleanupUploadedFiles(paths []string) {
	for _, path := range paths {
		_ = os.Remove(path)
	}
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
	message, err := c.service.RejectPurchaseRequest(uint(id), approverID.(uint), role, payload.Reason)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": message})
}
