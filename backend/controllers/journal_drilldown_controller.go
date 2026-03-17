package controllers

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"app-sistem-akuntansi/services"
	"app-sistem-akuntansi/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type JournalDrilldownController struct {
	drilldownService *services.JournalDrilldownService
}

func NewJournalDrilldownController(db *gorm.DB) *JournalDrilldownController {
	return &JournalDrilldownController{
		drilldownService: services.NewJournalDrilldownService(db),
	}
}

// GetJournalDrilldown handles journal entry drill-down requests from financial reports
// @Summary Journal Entry Drill-down
// @Description Get detailed journal entries for a specific line item from financial reports
// @Tags journal-drilldown
// @Accept json
// @Produce json
// @Param request body services.JournalDrilldownRequest true "Drill-down request parameters"
// @Success 200 {object} services.JournalDrilldownResponse
// @Failure 400 {object} utils.ErrorResponse
// @Router /journal-drilldown [post]
func (c *JournalDrilldownController) GetJournalDrilldown(ctx *gin.Context) {
	// Debug: Log raw request body
	body, _ := ctx.GetRawData()
	log.Printf("🔍 Raw request body: %s", string(body))

	// Reset body for binding
	ctx.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	var req services.JournalDrilldownRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ JSON binding error: %v", err)
		log.Printf("❌ Request body was: %s", string(body))
		appError := utils.NewBadRequestError(fmt.Sprintf("Invalid request payload: %v", err))
		ctx.JSON(appError.StatusCode, appError.ToErrorResponse(""))
		return
	}

	log.Printf("✅ Successfully parsed request: %+v", req)

	// Validate required fields
	if req.StartDate.IsZero() || req.EndDate.IsZero() {
		appError := utils.NewBadRequestError("Start date and end date are required")
		ctx.JSON(appError.StatusCode, appError.ToErrorResponse(""))
		return
	}

	// Validate date range
	if req.EndDate.Before(req.StartDate) {
		appError := utils.NewBadRequestError("End date cannot be before start date")
		ctx.JSON(appError.StatusCode, appError.ToErrorResponse(""))
		return
	}

	// Set default pagination if not provided
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}

	response, err := c.drilldownService.GetJournalEntriesForDrilldown(ctx.Request.Context(), &req)
	if err != nil {
		if appErr := utils.GetAppError(err); appErr != nil {
			ctx.JSON(appErr.StatusCode, appErr.ToErrorResponse(""))
		} else {
			internalErr := utils.NewInternalError("Failed to get journal entries for drill-down", err)
			ctx.JSON(internalErr.StatusCode, internalErr.ToErrorResponse(""))
		}
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": response})
}

// GetJournalDrilldownByParams handles drill-down using URL parameters (for simpler GET requests)
// @Summary Journal Entry Drill-down (GET)
// @Description Get journal entries using URL parameters for drill-down
// @Tags journal-drilldown
// @Produce json
// @Param account_codes query string false "Comma-separated account codes"
// @Param account_ids query string false "Comma-separated account IDs"
// @Param start_date query string true "Start date (YYYY-MM-DD)"
// @Param end_date query string true "End date (YYYY-MM-DD)"
// @Param report_type query string false "Report type (BALANCE_SHEET, PROFIT_LOSS, CASH_FLOW)"
// @Param line_item_name query string false "Name of the line item"
// @Param min_amount query number false "Minimum amount filter"
// @Param max_amount query number false "Maximum amount filter"
// @Param transaction_types query string false "Comma-separated transaction types"
// @Param page query int false "Page number (default: 1)"
// @Param limit query int false "Items per page (default: 20)"
// @Success 200 {object} services.JournalDrilldownResponse
// @Failure 400 {object} utils.ErrorResponse
// @Router /journal-drilldown/entries [get]
func (c *JournalDrilldownController) GetJournalDrilldownByParams(ctx *gin.Context) {
	req := services.JournalDrilldownRequest{}

	// Parse dates
	startDateStr := ctx.Query("start_date")
	endDateStr := ctx.Query("end_date")

	if startDateStr == "" || endDateStr == "" {
		appError := utils.NewBadRequestError("Start date and end date are required")
		ctx.JSON(appError.StatusCode, appError.ToErrorResponse(""))
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		appError := utils.NewBadRequestError("Invalid start date format. Use YYYY-MM-DD")
		ctx.JSON(appError.StatusCode, appError.ToErrorResponse(""))
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		appError := utils.NewBadRequestError("Invalid end date format. Use YYYY-MM-DD")
		ctx.JSON(appError.StatusCode, appError.ToErrorResponse(""))
		return
	}

	req.StartDate = startDate
	req.EndDate = endDate

	// Parse account codes
	if accountCodes := ctx.Query("account_codes"); accountCodes != "" {
		req.AccountCodes = parseCommaSeparatedString(accountCodes)
	}

	// Parse account IDs
	if accountIDs := ctx.Query("account_ids"); accountIDs != "" {
		req.AccountIDs = parseCommaSeparatedUints(accountIDs)
	}

	// Parse other parameters
	req.ReportType = ctx.Query("report_type")
	req.LineItemName = ctx.Query("line_item_name")

	// Parse amount filters
	if minAmountStr := ctx.Query("min_amount"); minAmountStr != "" {
		if minAmount, err := strconv.ParseFloat(minAmountStr, 64); err == nil {
			req.MinAmount = &minAmount
		}
	}

	if maxAmountStr := ctx.Query("max_amount"); maxAmountStr != "" {
		if maxAmount, err := strconv.ParseFloat(maxAmountStr, 64); err == nil {
			req.MaxAmount = &maxAmount
		}
	}

	// Parse transaction types
	if transactionTypes := ctx.Query("transaction_types"); transactionTypes != "" {
		req.TransactionTypes = parseCommaSeparatedString(transactionTypes)
	}
	if transactionType := ctx.Query("transaction_type"); transactionType != "" {
		req.TransactionType = transactionType
	}

	// Parse pagination
	if page := ctx.Query("page"); page != "" {
		if p, err := strconv.Atoi(page); err == nil && p > 0 {
			req.Page = p
		} else {
			req.Page = 1
		}
	} else {
		req.Page = 1
	}

	if limit := ctx.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			req.Limit = l
		} else {
			req.Limit = 20
		}
	} else {
		req.Limit = 20
	}

	response, err := c.drilldownService.GetJournalEntriesForDrilldown(ctx.Request.Context(), &req)
	if err != nil {
		if appErr := utils.GetAppError(err); appErr != nil {
			ctx.JSON(appErr.StatusCode, appErr.ToErrorResponse(""))
		} else {
			internalErr := utils.NewInternalError("Failed to get journal entries for drill-down", err)
			ctx.JSON(internalErr.StatusCode, internalErr.ToErrorResponse(""))
		}
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": response})
}

// GetJournalEntryDetail gets detailed information for a specific journal entry
// @Summary Get Journal Entry Detail
// @Description Get detailed information for a specific journal entry including all lines
// @Tags journal-drilldown
// @Produce json
// @Param id path int true "Journal Entry ID"
// @Success 200 {object} services.JournalDrilldownEntry
// @Failure 400 {object} utils.ErrorResponse
// @Failure 404 {object} utils.ErrorResponse
// @Router /journal-drilldown/entries/{id} [get]
func (c *JournalDrilldownController) GetJournalEntryDetail(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		appError := utils.NewBadRequestError("Invalid journal entry ID")
		ctx.JSON(appError.StatusCode, appError.ToErrorResponse(""))
		return
	}

	entry, err := c.drilldownService.GetJournalEntryDetail(ctx.Request.Context(), id)
	if err != nil {
		if appErr := utils.GetAppError(err); appErr != nil {
			ctx.JSON(appErr.StatusCode, appErr.ToErrorResponse(""))
		} else {
			internalErr := utils.NewInternalError("Failed to get journal entry detail", err)
			ctx.JSON(internalErr.StatusCode, internalErr.ToErrorResponse(""))
		}
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": entry})
}

// GetAccountsForPeriod gets all accounts that have activity in a specific period
// @Summary Get Active Accounts for Period
// @Description Get all accounts that have journal entries in a specific date range
// @Tags journal-drilldown
// @Produce json
// @Param start_date query string true "Start date (YYYY-MM-DD)"
// @Param end_date query string true "End date (YYYY-MM-DD)"
// @Success 200 {object} object{data=[]models.Account}
// @Failure 400 {object} utils.ErrorResponse
// @Router /journal-drilldown/accounts [get]
func (c *JournalDrilldownController) GetAccountsForPeriod(ctx *gin.Context) {
	startDateStr := ctx.Query("start_date")
	endDateStr := ctx.Query("end_date")

	if startDateStr == "" || endDateStr == "" {
		appError := utils.NewBadRequestError("Start date and end date are required")
		ctx.JSON(appError.StatusCode, appError.ToErrorResponse(""))
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		appError := utils.NewBadRequestError("Invalid start date format. Use YYYY-MM-DD")
		ctx.JSON(appError.StatusCode, appError.ToErrorResponse(""))
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		appError := utils.NewBadRequestError("Invalid end date format. Use YYYY-MM-DD")
		ctx.JSON(appError.StatusCode, appError.ToErrorResponse(""))
		return
	}

	accounts, err := c.drilldownService.GetAccountsForPeriod(ctx.Request.Context(), startDate, endDate)
	if err != nil {
		if appErr := utils.GetAppError(err); appErr != nil {
			ctx.JSON(appErr.StatusCode, appErr.ToErrorResponse(""))
		} else {
			internalErr := utils.NewInternalError("Failed to get accounts for period", err)
			ctx.JSON(internalErr.StatusCode, internalErr.ToErrorResponse(""))
		}
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": accounts})
}

// Helper functions
func parseCommaSeparatedString(input string) []string {
	if input == "" {
		return []string{}
	}

	var result []string
	for _, item := range strings.Split(input, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseCommaSeparatedUints(input string) []uint {
	if input == "" {
		return []uint{}
	}

	var result []uint
	for _, item := range strings.Split(input, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			if id, err := strconv.ParseUint(trimmed, 10, 32); err == nil {
				result = append(result, uint(id))
			}
		}
	}
	return result
}
