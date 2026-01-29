package controllers

import (
	"net/http"
	"strconv"
	"time"

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

	ctx.JSON(http.StatusOK, gin.H{"message": "Account balances refreshed successfully"})
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
	summary, err := c.getJournalSummary()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, summary)
}

// Helper function to get user ID from context (implement based on your auth system)
// Helper function to get user ID from context
func getJournalUserIDFromContext(ctx *gin.Context) uint64 {
	// Get user_id from context (set by JWT middleware)
	if userID, exists := ctx.Get("user_id"); exists {
		// handle both uint (from jwt middleware) and float64 (from some json parsers)
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
	// Default to 1 (Admin) if not found, but log warning in real app
	return 1
}

// Helper function to get journal summary statistics
func (c *UnifiedJournalController) getJournalSummary() (*JournalSummary, error) {
	// This could be implemented as a service method
	// For now, it's a simple implementation
	summary := &JournalSummary{
		Message: "Journal summary endpoint - implement based on your needs",
	}
	return summary, nil
}

// Request/Response types for specific endpoints
type ReverseJournalRequest struct {
	Description string `json:"description" binding:"required"`
}

type JournalSummary struct {
	Message string `json:"message"`
	// Add more fields as needed:
	// TotalEntries    int64           `json:"total_entries"`
	// TotalPosted     int64           `json:"total_posted"`
	// TotalDraft      int64           `json:"total_draft"`
	// TotalAmount     decimal.Decimal `json:"total_amount"`
	// LastEntryDate   time.Time       `json:"last_entry_date"`
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

	userID := getJournalUserIDFromContext(ctx)

	response, err := c.journalService.PostJournalEntry(id, userID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, response)
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
			journals.GET("/account-balances", c.GetAccountBalances)
			journals.POST("/account-balances/refresh", c.RefreshAccountBalances)
			journals.GET("/summary", c.GetJournalSummary)
		}
	}
}
