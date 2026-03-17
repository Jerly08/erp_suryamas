package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"

	"github.com/gin-gonic/gin"
)

// PeriodClosingController handles flexible period closing operations
type PeriodClosingController struct {
	service *services.UnifiedPeriodClosingService
}

// NewPeriodClosingController creates a new period closing controller
func NewPeriodClosingController(service *services.UnifiedPeriodClosingService) *PeriodClosingController {
	return &PeriodClosingController{
		service: service,
	}
}

// GetLastClosingInfo godoc
// @Summary Get last period closing information
// @Description Get information about the last closed period and next period start date
// @Tags Period Closing
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/period-closing/last-info [get]
func (pcc *PeriodClosingController) GetLastClosingInfo(c *gin.Context) {
	info, err := pcc.service.GetLastClosingInfo(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get last closing info",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    info,
	})
}

// PreviewClosing godoc
// @Summary Preview period closing
// @Description Get preview of period closing with detailed breakdown and validation
// @Tags Period Closing
// @Accept json
// @Produce json
// @Param start_date query string true "Period start date (YYYY-MM-DD)"
// @Param end_date query string true "Period end date (YYYY-MM-DD)"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/period-closing/preview [get]
func (pcc *PeriodClosingController) PreviewClosing(c *gin.Context) {
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	if startDateStr == "" || endDateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "start_date and end_date parameters are required (format: YYYY-MM-DD)",
		})
		return
	}

	startDate, err := time.ParseInLocation("2006-01-02", startDateStr, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid start_date format. Use YYYY-MM-DD",
			"details": err.Error(),
		})
		return
	}

	endDate, err := time.ParseInLocation("2006-01-02", endDateStr, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid end_date format. Use YYYY-MM-DD",
			"details": err.Error(),
		})
		return
	}

	preview, err := pcc.service.PreviewPeriodClosing(c.Request.Context(), startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to generate closing preview",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    preview,
	})
}

// ExecuteClosing godoc
// @Summary Execute period closing
// @Description Execute period closing with automated closing entries
// @Tags Period Closing
// @Accept json
// @Produce json
// @Param request body models.PeriodClosingRequest true "Period closing request"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/period-closing/execute [post]
func (pcc *PeriodClosingController) ExecuteClosing(c *gin.Context) {
	var req models.PeriodClosingRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Parse dates
	startDate, err := time.ParseInLocation("2006-01-02", req.StartDate, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid start_date format",
			"details": err.Error(),
		})
		return
	}

	endDate, err := time.ParseInLocation("2006-01-02", req.EndDate, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid end_date format",
			"details": err.Error(),
		})
		return
	}

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	// Convert user_id to uint64 (context stores as uint)
	var userIDUint64 uint64
	switch v := userID.(type) {
	case uint:
		userIDUint64 = uint64(v)
	case uint64:
		userIDUint64 = v
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Invalid user_id type",
		})
		return
	}

	description := req.Description
	if description == "" {
		description = "Period Closing: " + req.StartDate + " to " + req.EndDate
	}

	err = pcc.service.ExecutePeriodClosing(c.Request.Context(), startDate, endDate, description, userIDUint64)
	if err != nil {
		if errors.Is(err, services.ErrNoBalancesToClose) {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "No balances to close for the selected period",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Failed to execute period closing",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Period closed successfully. All revenue and expense accounts have been reset and transferred to retained earnings.",
	})
}

// GetClosingHistory godoc
// @Summary Get period closing history
// @Description Get history of period closings
// @Tags Period Closing
// @Accept json
// @Produce json
// @Param limit query int false "Limit number of results (default: 20)"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/period-closing/history [get]
func (pcc *PeriodClosingController) GetClosingHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	limit := 20
	if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
		limit = parsed
	}

	history, err := pcc.service.GetClosingHistory(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get closing history",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    history,
		"total":   len(history),
	})
}

// CheckDateInClosedPeriod godoc
// @Summary Check if date is in closed period
// @Description Check if a given date falls within a closed period
// @Tags Period Closing
// @Accept json
// @Produce json
// @Param date query string true "Date to check (YYYY-MM-DD)"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/period-closing/check-date [get]
func (pcc *PeriodClosingController) CheckDateInClosedPeriod(c *gin.Context) {
	dateStr := c.Query("date")
	if dateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "date parameter is required (format: YYYY-MM-DD)",
		})
		return
	}

	date, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid date format. Use YYYY-MM-DD",
			"details": err.Error(),
		})
		return
	}

	isClosed, err := pcc.service.IsDateInClosedPeriod(c.Request.Context(), date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to check closed period",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"is_closed": isClosed,
		"date":      dateStr,
	})
}

// ReopenPeriod godoc
// @Summary Reopen a closed period
// @Description Reopen a previously closed period, reversing the closing journal entries
// @Tags Period Closing
// @Accept json
// @Produce json
// @Param request body models.PeriodReopenRequest true "Period reopen request"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/period-closing/reopen [post]
func (pcc *PeriodClosingController) ReopenPeriod(c *gin.Context) {
	var req models.PeriodReopenRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Data tidak valid. Pastikan start_date, end_date, dan reason diisi.",
			"details": err.Error(),
		})
		return
	}

	// Parse dates
	startDate, err := time.ParseInLocation("2006-01-02", req.StartDate, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Format start_date tidak valid. Gunakan YYYY-MM-DD",
		})
		return
	}

	endDate, err := time.ParseInLocation("2006-01-02", req.EndDate, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Format end_date tidak valid. Gunakan YYYY-MM-DD",
		})
		return
	}

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "User tidak terautentikasi",
		})
		return
	}

	var userIDUint64 uint64
	switch v := userID.(type) {
	case uint:
		userIDUint64 = uint64(v)
	case uint64:
		userIDUint64 = v
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Tipe user_id tidak valid",
		})
		return
	}

	err = pcc.service.ReopenPeriod(c.Request.Context(), startDate, endDate, req.Reason, userIDUint64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Gagal membuka kembali periode",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Periode berhasil dibuka kembali. Jurnal penutup telah dibalik dengan jurnal pembalik dan saldo akun telah dihitung ulang.",
	})
}
