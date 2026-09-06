package controllers

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"

	"github.com/gin-gonic/gin"
)

const (
	maxPaymentDocumentsPerRequest = 10
	paymentDocumentUploadDir      = "./uploads/payment-documents"
	paymentDocumentStoredPrefix   = "/uploads/payment-documents/"
)

// The UI submits one file at a time. Keep a retryable request close to the
// per-file limit instead of allowing a multi-hundred-MB multipart body.
var maxPaymentDocumentRequestSize = int64(maxDocumentUploadSize) + 1024*1024

// PaymentDocumentController manages the attachment history for payments. It
// deliberately does not call a posting/approval service, so uploads cannot
// alter the payment amount, allocation, or journal.
type PaymentDocumentController struct {
	service *services.PaymentDocumentService
}

func NewPaymentDocumentController(service *services.PaymentDocumentService) *PaymentDocumentController {
	return &PaymentDocumentController{service: service}
}

// ListDocuments returns the complete document history for one payment.
func (c *PaymentDocumentController) ListDocuments(ctx *gin.Context) {
	paymentID, ok := parsePaymentDocumentPaymentID(ctx)
	if !ok {
		return
	}

	documents, err := c.service.ListDocuments(paymentID, ctx.GetUint("user_id"), ctx.GetString("role"))
	if err != nil {
		writePaymentDocumentError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": documents})
}

// UploadDocuments accepts multiple supporting files after a payment exists.
// The payment is not recreated if a file upload fails; callers can retry this
// endpoint with the existing payment ID.
func (c *PaymentDocumentController) UploadDocuments(ctx *gin.Context) {
	paymentID, ok := parsePaymentDocumentPaymentID(ctx)
	if !ok {
		return
	}
	if err := c.service.EnsureCanUploadDocuments(paymentID, ctx.GetUint("user_id"), ctx.GetString("role")); err != nil {
		writePaymentDocumentError(ctx, err)
		return
	}

	// This endpoint accepts a bounded multipart request. The audit middleware
	// intentionally leaves multipart streams untouched, so the form parser sees
	// the original file body.
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxPaymentDocumentRequestSize)
	form, err := ctx.MultipartForm()
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "request body too large") {
			status = http.StatusRequestEntityTooLarge
		}
		ctx.JSON(status, gin.H{"error": "invalid multipart form", "details": err.Error()})
		return
	}

	files := collectPaymentDocumentFiles(form)
	if len(files) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "at least one document file is required"})
		return
	}
	if len(files) > maxPaymentDocumentsPerRequest {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("maximum %d documents can be uploaded at once", maxPaymentDocumentsPerRequest)})
		return
	}

	documentType := firstMultipartValue(form, "document_type")
	if _, valid := models.NormalizePaymentDocumentType(documentType); !valid {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment document type"})
		return
	}
	referenceNumber := firstMultipartValue(form, "reference_number")
	notes := firstMultipartValue(form, "notes")

	inputs := make([]services.PaymentDocumentUploadInput, 0, len(files))
	savedDiskPaths := make([]string, 0, len(files))
	for _, header := range files {
		filename, diskPath, mimeType, validationErr := prepareValidatedDocumentUpload(header, paymentID, paymentDocumentUploadDir)
		if validationErr != nil {
			cleanupPaymentDocumentFiles(savedDiskPaths)
			ctx.JSON(http.StatusBadRequest, gin.H{"error": validationErr.Error()})
			return
		}

		if err := ctx.SaveUploadedFile(header, diskPath); err != nil {
			cleanupPaymentDocumentFiles(savedDiskPaths)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "document could not be saved on the server"})
			return
		}
		savedDiskPaths = append(savedDiskPaths, diskPath)

		inputs = append(inputs, services.PaymentDocumentUploadInput{
			DocumentType:    documentType,
			ReferenceNumber: referenceNumber,
			Notes:           notes,
			FileName:        paymentDocumentDisplayName(header),
			FilePath:        paymentDocumentStoredPrefix + filename,
			FileSize:        header.Size,
			MimeType:        mimeType,
		})
	}

	documents, err := c.service.UploadDocuments(paymentID, ctx.GetUint("user_id"), ctx.GetString("role"), inputs)
	if err != nil {
		cleanupPaymentDocumentFiles(savedDiskPaths)
		writePaymentDocumentError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"message": "Supporting documents uploaded successfully",
		"data":    documents,
	})
}

// DownloadDocument sends a stored document only after payment-level access is
// checked by the service. File paths are then constrained to the dedicated
// uploads/payment-documents directory by sendStoredUploadAttachment.
func (c *PaymentDocumentController) DownloadDocument(ctx *gin.Context) {
	documentID, err := strconv.ParseUint(ctx.Param("document_id"), 10, 32)
	if err != nil || documentID == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment document ID"})
		return
	}

	document, err := c.service.GetDocumentForDownload(uint(documentID), ctx.GetUint("user_id"), ctx.GetString("role"))
	if err != nil {
		writePaymentDocumentError(ctx, err)
		return
	}

	sendStoredUploadAttachment(ctx, document.FilePath, document.FileName, document.MimeType, "payment-documents")
}

// InvalidateDocument retains a mistaken upload and records why it must no
// longer be relied on. There is intentionally no destructive delete endpoint.
func (c *PaymentDocumentController) InvalidateDocument(ctx *gin.Context) {
	documentID, err := strconv.ParseUint(ctx.Param("document_id"), 10, 32)
	if err != nil || documentID == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment document ID"})
		return
	}

	var request struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalidation reason is required"})
		return
	}

	document, err := c.service.InvalidateDocument(uint(documentID), ctx.GetUint("user_id"), ctx.GetString("role"), request.Reason)
	if err != nil {
		writePaymentDocumentError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Payment document marked as invalidated",
		"data":    document,
	})
}

func parsePaymentDocumentPaymentID(ctx *gin.Context) (uint, bool) {
	paymentID, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil || paymentID == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment ID"})
		return 0, false
	}
	return uint(paymentID), true
}

func collectPaymentDocumentFiles(form *multipart.Form) []*multipart.FileHeader {
	if form == nil {
		return nil
	}

	files := make([]*multipart.FileHeader, 0)
	files = append(files, form.File["documents"]...)
	files = append(files, form.File["attachments"]...)
	files = append(files, form.File["document"]...)
	return files
}

func firstMultipartValue(form *multipart.Form, name string) string {
	if form == nil || len(form.Value[name]) == 0 {
		return ""
	}
	return strings.TrimSpace(form.Value[name][0])
}

func paymentDocumentDisplayName(header *multipart.FileHeader) string {
	if header == nil {
		return "document"
	}
	name := strings.TrimSpace(filepath.Base(header.Filename))
	if name == "" || name == "." {
		return "document"
	}
	runes := []rune(name)
	if len(runes) > 255 {
		return string(runes[:255])
	}
	return name
}

func cleanupPaymentDocumentFiles(paths []string) {
	for _, path := range paths {
		_ = os.Remove(path)
	}
}

func writePaymentDocumentError(ctx *gin.Context, err error) {
	status := http.StatusBadRequest
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "not found") {
		status = http.StatusNotFound
	}
	if strings.Contains(message, "access denied") || strings.Contains(message, "only the payment creator") || strings.Contains(message, "only accounting") || strings.Contains(message, "director can only") {
		status = http.StatusForbidden
	}
	ctx.JSON(status, gin.H{"error": err.Error()})
}
