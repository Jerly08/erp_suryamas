package controllers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"
	"github.com/gin-gonic/gin"
)

// UnifiedJournalController handles REST endpoints for the SSOT journal system
type UnifiedJournalController struct {
	journalService *services.UnifiedJournalService
}

// NewUnifiedJournalController creates a new instance of UnifiedJournalController
func NewUnifiedJournalController(journalService *services.UnifiedJournalService) *UnifiedJournalController {
	return &UnifiedJournalController{
		journalService: journalService,
	}
}

// CreateJournalEntry creates a new journal entry
// @Summary Create a new journal entry
// @Description Create a new journal entry with validation and optional auto-posting
// @Tags Journal
// @Accept json
// @Produce json
// @Param journal body services.JournalEntryRequest true "Journal Entry Request"
// @Success 201 {object} services.JournalResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/journals [post]
func (c *UnifiedJournalController) CreateJournalEntry(ctx *gin.Context) {
	var req services.JournalEntryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format: " + err.Error()})
		return
	}

	// Set default values if needed
	if req.EntryDate.IsZero() {
		req.EntryDate = time.Now()
	}

	// Set CreatedBy from context
	req.CreatedBy = getJournalUserIDFromContext(ctx)
	userRole := getJournalUserRoleFromContext(ctx)
	req.RequestedByFinanceManager = models.IsFinanceManagerRole(userRole)
	if requiresJournalApprovalWorkflow(req.SourceType, req.AdjustmentOf) && req.RequestedByFinanceManager && !req.RequireDirectorApproval {
		req.AutoPost = true
	}

	if isAdjustmentSourceType(req.SourceType) && !canManageAdjustmentDraft(userRole) {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Hanya finance staff atau finance manager yang dapat membuat jurnal adjustment"})
		return
	}
	if isAdjustmentSourceType(req.SourceType) && (req.AdjustmentOf == nil || *req.AdjustmentOf == 0) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Jurnal adjustment harus dibuat dari action jurnal asal"})
		return
	}

	response, err := c.journalService.CreateJournalEntry(&req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, response)
}

// GetJournalEntry retrieves a journal entry by ID
// @Summary Get journal entry by ID
// @Description Get a specific journal entry with its lines
// @Tags Journal
// @Produce json
// @Param id path int true "Journal Entry ID"
// @Success 200 {object} services.JournalResponse
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/journals/{id} [get]
func (c *UnifiedJournalController) GetJournalEntry(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid journal entry ID"})
		return
	}

	response, err := c.journalService.GetJournalEntry(id)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, response)
}

// GetJournalEntries retrieves journal entries with filters and pagination
// @Summary Get journal entries list
// @Description Get journal entries with filtering, pagination, and sorting
// @Tags Journal
// @Produce json
// @Param source_type query string false "Source type filter"
// @Param status query string false "Status filter"
// @Param date_from query string false "Date from filter (YYYY-MM-DD)"
// @Param date_to query string false "Date to filter (YYYY-MM-DD)"
// @Param reference query string false "Reference search"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} services.JournalResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/journals [get]
func (c *UnifiedJournalController) GetJournalEntries(ctx *gin.Context) {
	var filters services.JournalFilters

	// Parse query parameters
	if err := ctx.ShouldBindQuery(&filters); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters: " + err.Error()})
		return
	}

	// Set default pagination values
	if filters.Page <= 0 {
		filters.Page = 1
	}
	if filters.Limit <= 0 {
		filters.Limit = 20
	}
	if filters.Limit > 100 {
		filters.Limit = 100 // Maximum limit
	}

	// Parse date filters if provided
	if dateFromStr := ctx.Query("date_from"); dateFromStr != "" {
		if parsedDate, err := time.Parse("2006-01-02", dateFromStr); err == nil {
			filters.DateFrom = &parsedDate
		}
	}
	if dateToStr := ctx.Query("date_to"); dateToStr != "" {
		if parsedDate, err := time.Parse("2006-01-02", dateToStr); err == nil {
			dateTo := parsedDate.AddDate(0, 0, 1) // Include the entire day
			filters.DateTo = &dateTo
		}
	}

	response, err := c.journalService.GetJournalEntries(filters)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, response)
}

// GetAccountBalances retrieves account balances
// @Summary Get account balances
// @Description Get account balances from materialized view
// @Tags Journal
// @Produce json
// @Success 200 {array} models.SSOTAccountBalance
// @Failure 500 {object} map[string]string
// @Router /api/v1/journals/account-balances [get]
func (c *UnifiedJournalController) GetAccountBalances(ctx *gin.Context) {
	balances, err := c.journalService.GetAccountBalances()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data":  balances,
		"count": len(balances),
	})
}

// RefreshAccountBalances manually refreshes account balances
// @Summary Refresh account balances
// @Description Manually refresh the materialized view for account balances
// @Tags Journal
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/journals/account-balances/refresh [post]
func (c *UnifiedJournalController) RefreshAccountBalances(ctx *gin.Context) {
	err := c.journalService.RefreshAccountBalances()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message":    "Account balances refreshed successfully",
		"updated_at": time.Now(),
	})
}

// GetJournalSummary provides summary statistics
// @Summary Get journal summary
// @Description Get summary statistics for journal entries
// @Tags Journal
// @Produce json
// @Success 200 {object} JournalSummary
// @Failure 500 {object} map[string]string
// @Router /api/v1/journals/summary [get]
func (c *UnifiedJournalController) GetJournalSummary(ctx *gin.Context) {
	var filters services.JournalFilters
	if err := ctx.ShouldBindQuery(&filters); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameters: " + err.Error()})
		return
	}

	if dateFromStr := ctx.Query("start_date"); dateFromStr != "" {
		if parsedDate, err := time.Parse("2006-01-02", dateFromStr); err == nil {
			filters.DateFrom = &parsedDate
		}
	}
	if dateToStr := ctx.Query("end_date"); dateToStr != "" {
		if parsedDate, err := time.Parse("2006-01-02", dateToStr); err == nil {
			dateTo := parsedDate.AddDate(0, 0, 1)
			filters.DateTo = &dateTo
		}
	}

	summary, err := c.journalService.GetJournalSummary(filters)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, summary)
}

func getJournalUserIDFromContext(ctx *gin.Context) uint64 {
	if userID, exists := ctx.Get("user_id"); exists {
		switch v := userID.(type) {
		case uint:
			return uint64(v)
		case uint64:
			return v
		case float64:
			return uint64(v)
		case int:
			return uint64(v)
		}
	}
	return 1
}

func getJournalUserRoleFromContext(ctx *gin.Context) string {
	if role := models.NormalizeRole(ctx.GetString("user_role")); role != "" {
		return role
	}

	return models.NormalizeRole(ctx.GetString("role"))
}

func canManageAdjustmentDraft(role string) bool {
	return models.IsFinanceStaffRole(role) || models.IsFinanceManagerRole(role)
}

func canApproveAdjustment(role string) bool {
	return models.IsFinanceManagerRole(role) || models.NormalizeRole(role) == models.RoleDirector
}

func isAdjustmentSourceType(sourceType string) bool {
	return strings.EqualFold(strings.TrimSpace(sourceType), models.SSOTSourceTypeAdjustment)
}

func requiresJournalApprovalWorkflow(sourceType string, adjustmentOf *uint64) bool {
	normalizedSourceType := strings.ToUpper(strings.TrimSpace(sourceType))
	switch normalizedSourceType {
	case "", models.SSOTSourceTypeManual:
		return true
	case models.SSOTSourceTypeAdjustment:
		return adjustmentOf != nil && *adjustmentOf != 0
	default:
		return false
	}
}

func isJournalDirectorApprovalPending(entry *models.SSOTJournalEntry) bool {
	if entry == nil {
		return false
	}

	return entry.Status == models.SSOTStatusDraft &&
		entry.RequireDirectorApproval &&
		strings.EqualFold(strings.TrimSpace(entry.ApprovalStage), models.SSOTApprovalStageDirector)
}

// Request/Response types for specific endpoints
type ReverseJournalRequest struct {
	Description string `json:"description" binding:"required"`
}

// UpdateJournalEntry updates an existing journal entry
// @Summary Update journal entry
// @Description Update an existing journal entry (only if DRAFT)
// @Tags Journal
// @Accept json
// @Produce json
// @Param id path int true "Journal Entry ID"
// @Param journal body services.JournalEntryRequest true "Journal Entry Request"
// @Success 200 {object} services.JournalResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/journals/{id} [put]
func (c *UnifiedJournalController) UpdateJournalEntry(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid journal entry ID"})
		return
	}

	var req services.JournalEntryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format: " + err.Error()})
		return
	}

	if req.EntryDate.IsZero() {
		req.EntryDate = time.Now()
	}

	existingEntry, err := c.journalService.GetJournalEntry(id)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if isAdjustmentSourceType(existingEntry.SourceType) && !canManageAdjustmentDraft(getJournalUserRoleFromContext(ctx)) {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Hanya finance staff atau finance manager yang dapat mengubah draft jurnal adjustment"})
		return
	}
	if isAdjustmentSourceType(existingEntry.SourceType) && existingEntry.AdjustmentOf != nil && req.AdjustmentOf != nil && *existingEntry.AdjustmentOf != *req.AdjustmentOf {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Referensi jurnal asal untuk adjustment tidak boleh diubah"})
		return
	}

	response, err := c.journalService.UpdateJournalEntry(id, &req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, response)
}

// DeleteJournalEntry deletes a journal entry
// @Summary Delete journal entry
// @Description Delete a journal entry (only if DRAFT)
// @Tags Journal
// @Produce json
// @Param id path int true "Journal Entry ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/journals/{id} [delete]
func (c *UnifiedJournalController) DeleteJournalEntry(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid journal entry ID"})
		return
	}

	if err := c.journalService.DeleteJournalEntry(id); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Journal entry deleted successfully"})
}

// PostJournalEntry posts a journal entry
// @Summary Post journal entry
// @Description Post a journal entry (verify and lock)
// @Tags Journal
// @Produce json
// @Param id path int true "Journal Entry ID"
// @Success 200 {object} services.JournalResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/journals/{id}/post [put]
func (c *UnifiedJournalController) PostJournalEntry(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid journal entry ID"})
		return
	}

	var req struct {
		RequireDirectorApproval bool `json:"require_director_approval"`
	}
	_ = ctx.ShouldBindJSON(&req)

	userID := getJournalUserIDFromContext(ctx)
	userRole := getJournalUserRoleFromContext(ctx)

	existingEntry, err := c.journalService.GetJournalEntry(id)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	requiresApprovalWorkflow := requiresJournalApprovalWorkflow(existingEntry.SourceType, existingEntry.AdjustmentOf)
	directorApprovalPending := isJournalDirectorApprovalPending(existingEntry)

	if requiresApprovalWorkflow {
		if directorApprovalPending {
			if models.NormalizeRole(userRole) != models.RoleDirector {
				ctx.JSON(http.StatusForbidden, gin.H{"error": "Jurnal ini sedang menunggu approval director"})
				return
			}
		} else {
			if !models.IsFinanceManagerRole(userRole) {
				ctx.JSON(http.StatusForbidden, gin.H{"error": "Draft jurnal ini hanya dapat dipost oleh finance manager"})
				return
			}
			if req.RequireDirectorApproval {
				response, err := c.journalService.SendJournalEntryToDirector(id, userID)
				if err != nil {
					ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				ctx.JSON(http.StatusOK, response)
				return
			}
		}
	} else if isAdjustmentSourceType(existingEntry.SourceType) && !canApproveAdjustment(userRole) {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Jurnal adjustment hanya dapat di-approve oleh finance manager atau director sesuai tahap approval"})
		return
	}

	response, err := c.journalService.PostJournalEntry(id, userID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, response)
}

// ReverseJournalEntry reverses a posted journal entry.
// @Summary Reverse journal entry
// @Description Create a reversal entry for a posted SSOT journal
// @Tags Journal
// @Accept json
// @Produce json
// @Param id path int true "Journal Entry ID"
// @Param request body ReverseJournalRequest true "Reverse Journal Request"
// @Success 200 {object} models.SSOTJournalEntry
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/journals/{id}/reverse [post]
func (c *UnifiedJournalController) ReverseJournalEntry(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid journal entry ID"})
		return
	}

	var req ReverseJournalRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format: " + err.Error()})
		return
	}

	response, err := c.journalService.ReverseJournalEntry(ctx.Request.Context(), id, getJournalUserIDFromContext(ctx), req.Description)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": response})
}

// RegisterRoutes registers all journal routes
func (c *UnifiedJournalController) RegisterRoutes(router *gin.Engine) {
	v1 := router.Group("/api/v1")
	{
		journals := v1.Group("/journals")
		{
			journals.POST("", c.CreateJournalEntry)
			journals.GET("", c.GetJournalEntries)
			journals.GET("/:id", c.GetJournalEntry)
			journals.PUT("/:id", c.UpdateJournalEntry)
			journals.DELETE("/:id", c.DeleteJournalEntry)
			journals.PUT("/:id/post", c.PostJournalEntry)
			journals.POST("/:id/reverse", c.ReverseJournalEntry)
			journals.GET("/account-balances", c.GetAccountBalances)
			journals.POST("/account-balances/refresh", c.RefreshAccountBalances)
			journals.GET("/summary", c.GetJournalSummary)
		}
	}
}
