package controllers

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type PostedTransactionCorrectionController struct {
	service *services.PostedTransactionCorrectionService
}

func NewPostedTransactionCorrectionController(service *services.PostedTransactionCorrectionService) *PostedTransactionCorrectionController {
	return &PostedTransactionCorrectionController{service: service}
}

func (pc *PostedTransactionCorrectionController) List(c *gin.Context) {
	filter := models.PostedTransactionCorrectionFilter{
		Status: c.Query("status"),
		Page:   parseCorrectionPaginationValue(c.DefaultQuery("page", "1"), 1),
		Limit:  parseCorrectionPaginationValue(c.DefaultQuery("limit", "20"), 20),
	}
	if value := c.Query("purchase_id"); value != "" {
		id, err := parsePostedTransactionCorrectionID(value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "purchase_id tidak valid"})
			return
		}
		filter.PurchaseID = &id
	}
	if value := c.Query("payment_id"); value != "" {
		id, err := parsePostedTransactionCorrectionID(value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "payment_id tidak valid"})
			return
		}
		filter.PaymentID = &id
	}

	corrections, total, err := pc.service.List(filter, c.GetUint("user_id"), c.GetString("role"))
	if err != nil {
		pc.writeError(c, err)
		return
	}
	pageCount := (total + int64(filter.Limit) - 1) / int64(filter.Limit)
	c.JSON(http.StatusOK, gin.H{
		"data": corrections,
		"pagination": gin.H{
			"page":        filter.Page,
			"limit":       filter.Limit,
			"total":       total,
			"total_pages": pageCount,
		},
	})
}

func (pc *PostedTransactionCorrectionController) GetByID(c *gin.Context) {
	id, err := parsePostedTransactionCorrectionID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID koreksi transaksi tidak valid"})
		return
	}
	correction, err := pc.service.GetByID(id, c.GetUint("user_id"), c.GetString("role"))
	if err != nil {
		pc.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": correction})
}

func (pc *PostedTransactionCorrectionController) Create(c *gin.Context) {
	var request models.PostedTransactionCorrectionCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data pengajuan koreksi belum lengkap"})
		return
	}
	correction, err := pc.service.Create(request, c.GetUint("user_id"), c.GetString("role"))
	if err != nil {
		pc.writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"message": "Pengajuan koreksi transaksi berhasil dikirim ke Accounting",
		"data":    correction,
	})
}

func (pc *PostedTransactionCorrectionController) ReviewAccounting(c *gin.Context) {
	id, err := parsePostedTransactionCorrectionID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID koreksi transaksi tidak valid"})
		return
	}
	var request models.PostedTransactionCorrectionAccountingReviewRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Rencana penyelesaian dan catatan Accounting wajib diisi"})
		return
	}
	correction, err := pc.service.ReviewAccounting(id, request, c.GetUint("user_id"), c.GetString("role"))
	if err != nil {
		pc.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": correction})
}

func (pc *PostedTransactionCorrectionController) DecideFinanceManager(c *gin.Context) {
	id, err := parsePostedTransactionCorrectionID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID koreksi transaksi tidak valid"})
		return
	}
	var request models.PostedTransactionCorrectionFinanceManagerDecisionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Keputusan Finance Manager tidak valid"})
		return
	}
	correction, err := pc.service.DecideFinanceManager(id, request, c.GetUint("user_id"), c.GetString("role"))
	if err != nil {
		pc.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": correction})
}

func (pc *PostedTransactionCorrectionController) Complete(c *gin.Context) {
	id, err := parsePostedTransactionCorrectionID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID koreksi transaksi tidak valid"})
		return
	}
	var request models.PostedTransactionCorrectionCompleteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Catatan penyelesaian wajib diisi"})
		return
	}
	correction, err := pc.service.Complete(id, request, c.GetUint("user_id"), c.GetString("role"))
	if err != nil {
		pc.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": correction})
}

func (pc *PostedTransactionCorrectionController) UploadDocument(c *gin.Context) {
	id, err := parsePostedTransactionCorrectionID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID koreksi transaksi tidak valid"})
		return
	}
	file, header, err := c.Request.FormFile("document")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dokumen koreksi wajib dipilih"})
		return
	}
	defer file.Close()

	const uploadDir = "uploads/posted-transaction-corrections"
	filename, filePath, mimeType, err := prepareValidatedDocumentUpload(header, id, uploadDir)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := c.SaveUploadedFile(header, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Dokumen koreksi tidak dapat disimpan"})
		return
	}

	document, err := pc.service.AddDocument(
		id,
		header.Filename,
		"uploads/posted-transaction-corrections/"+filename,
		header.Size,
		mimeType,
		c.GetUint("user_id"),
		c.GetString("role"),
	)
	if err != nil {
		_ = os.Remove(filePath)
		pc.writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"message":  "Dokumen koreksi berhasil ditambahkan",
		"document": document,
	})
}

func (pc *PostedTransactionCorrectionController) DownloadDocument(c *gin.Context) {
	id, err := parsePostedTransactionCorrectionID(c.Param("document_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID dokumen koreksi tidak valid"})
		return
	}
	document, err := pc.service.GetDocument(id, c.GetUint("user_id"), c.GetString("role"))
	if err != nil {
		pc.writeError(c, err)
		return
	}
	sendStoredUploadAttachment(c, document.FilePath, document.FileName, document.MimeType, "posted-transaction-corrections")
}

func (pc *PostedTransactionCorrectionController) writeError(c *gin.Context, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, services.ErrPostedTransactionCorrectionNotFound):
		status = http.StatusNotFound
	case errors.Is(err, services.ErrPostedTransactionCorrectionDenied):
		status = http.StatusForbidden
	case strings.Contains(strings.ToLower(err.Error()), "tidak ditemukan"):
		status = http.StatusNotFound
	}
	c.JSON(status, gin.H{"error": err.Error()})
}

func parsePostedTransactionCorrectionID(value string) (uint, error) {
	id, err := strconv.ParseUint(value, 10, 32)
	if err != nil || id == 0 {
		return 0, errors.New("invalid ID")
	}
	return uint(id), nil
}

func parseCorrectionPaginationValue(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}
