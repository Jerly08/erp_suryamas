package handlers

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/services"
	"app-sistem-akuntansi/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AccountHandler handles account-related operations
type AccountHandler struct {
	repo          repositories.AccountRepository
	exportService services.ExportService
}

// NewAccountHandler creates a new account handler
func NewAccountHandler(repo repositories.AccountRepository, exportService services.ExportService) *AccountHandler {
	return &AccountHandler{
		repo:          repo,
		exportService: exportService,
	}
}

func (h *AccountHandler) requestContextWithUser(c *gin.Context) context.Context {
	ctx := c.Request.Context()
	if userID := c.GetUint("user_id"); userID > 0 {
		role, _ := c.Get("role")
		username, _ := c.Get("username")
		roleStr, _ := role.(string)
		usernameStr, _ := username.(string)
		ctx = utils.SetUserContext(ctx, userID, roleStr, usernameStr)
	}
	return ctx
}

// CreateAccount creates a new account
func (h *AccountHandler) CreateAccount(c *gin.Context) {
	var req models.AccountCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		appError := utils.NewBadRequestError("Invalid request payload")
		c.JSON(appError.StatusCode, appError.ToErrorResponse(""))
		return
	}

	account, err := h.repo.Create(h.requestContextWithUser(c), &req)
	if err != nil {
		if appErr := utils.GetAppError(err); appErr != nil {
			c.JSON(appErr.StatusCode, appErr.ToErrorResponse(""))
		} else {
			internalErr := utils.NewInternalError("Failed to create account", err)
			c.JSON(internalErr.StatusCode, internalErr.ToErrorResponse(""))
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": account})
}

// GetAccount gets a single account by code
func (h *AccountHandler) GetAccount(c *gin.Context) {
	code := c.Param("code")

	account, err := h.repo.FindByCode(c.Request.Context(), code)
	if err != nil {
		if appErr := utils.GetAppError(err); appErr != nil {
			c.JSON(appErr.StatusCode, appErr.ToErrorResponse(""))
		} else {
			internalErr := utils.NewInternalError("Failed to get account", err)
			c.JSON(internalErr.StatusCode, internalErr.ToErrorResponse(""))
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": account})
}

// UpdateAccount updates an account
func (h *AccountHandler) UpdateAccount(c *gin.Context) {
	code := c.Param("code")
	log.Printf("UpdateAccount called with code: %s", code)

	var req models.AccountUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Failed to bind JSON: %v", err)
		appError := utils.NewBadRequestError("Invalid request payload")
		c.JSON(appError.StatusCode, appError.ToErrorResponse(""))
		return
	}

	log.Printf("Update request data: %+v", req)

	account, err := h.repo.Update(c.Request.Context(), code, &req)
	if err != nil {
		log.Printf("Failed to update account: %v", err)
		if appErr := utils.GetAppError(err); appErr != nil {
			c.JSON(appErr.StatusCode, appErr.ToErrorResponse(""))
		} else {
			internalErr := utils.NewInternalError("Failed to update account", err)
			c.JSON(internalErr.StatusCode, internalErr.ToErrorResponse(""))
		}
		return
	}

	log.Printf("Account updated successfully: %+v", account)
	c.JSON(http.StatusOK, gin.H{"data": account})
}

// DeleteAccount deletes an account
func (h *AccountHandler) DeleteAccount(c *gin.Context) {
	code := c.Param("code")

	err := h.repo.Delete(c.Request.Context(), code)
	if err != nil {
		if appErr := utils.GetAppError(err); appErr != nil {
			c.JSON(appErr.StatusCode, appErr.ToErrorResponse(""))
		} else {
			internalErr := utils.NewInternalError("Failed to delete account", err)
			c.JSON(internalErr.StatusCode, internalErr.ToErrorResponse(""))
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Account deleted successfully"})
}

// AdminDeleteAccount deletes an account with admin privileges
func (h *AccountHandler) AdminDeleteAccount(c *gin.Context) {
	code := c.Param("code")

	// Parse request body for cascade options
	type AdminDeleteRequest struct {
		CascadeDelete bool  `json:"cascade_delete"`
		NewParentID   *uint `json:"new_parent_id"`
	}

	var req AdminDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Default to no cascade, move to root
		req.CascadeDelete = false
		req.NewParentID = nil
	}

	err := h.repo.(*repositories.AccountRepo).AdminDelete(c.Request.Context(), code, req.CascadeDelete, req.NewParentID)
	if err != nil {
		if appErr := utils.GetAppError(err); appErr != nil {
			c.JSON(appErr.StatusCode, appErr.ToErrorResponse(""))
		} else {
			internalErr := utils.NewInternalError("Failed to delete account", err)
			c.JSON(internalErr.StatusCode, internalErr.ToErrorResponse(""))
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Account deleted successfully (admin)", "cascade": req.CascadeDelete})
}

// ListAccounts lists all accounts with optional filtering
func (h *AccountHandler) ListAccounts(c *gin.Context) {
	accountType := c.Query("type")
	category := c.Query("category")

	var accounts []models.Account
	var err error

	if accountType != "" || category != "" {
		var typePtr *string
		var catPtr *string
		if accountType != "" {
			typePtr = &accountType
		}
		if category != "" {
			catPtr = &category
		}
		accounts, err = h.repo.FindWithFilter(c.Request.Context(), typePtr, catPtr)
	} else {
		accounts, err = h.repo.FindAll(c.Request.Context())
	}

	if err != nil {
		if appErr := utils.GetAppError(err); appErr != nil {
			c.JSON(appErr.StatusCode, appErr.ToErrorResponse(""))
		} else {
			internalErr := utils.NewInternalError("Failed to retrieve accounts", err)
			c.JSON(internalErr.StatusCode, internalErr.ToErrorResponse(""))
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": accounts, "count": len(accounts)})
}

// GetAccountHierarchy gets account hierarchy tree
func (h *AccountHandler) GetAccountHierarchy(c *gin.Context) {
	accounts, err := h.repo.GetHierarchy(c.Request.Context())
	if err != nil {
		if appErr := utils.GetAppError(err); appErr != nil {
			c.JSON(appErr.StatusCode, appErr.ToErrorResponse(""))
		} else {
			internalErr := utils.NewInternalError("Failed to get account hierarchy", err)
			c.JSON(internalErr.StatusCode, internalErr.ToErrorResponse(""))
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": accounts})
}

// GetBalanceSummary gets balance summary by account type
func (h *AccountHandler) GetBalanceSummary(c *gin.Context) {
	summary, err := h.repo.GetBalanceSummary(c.Request.Context())
	if err != nil {
		if appErr := utils.GetAppError(err); appErr != nil {
			c.JSON(appErr.StatusCode, appErr.ToErrorResponse(""))
		} else {
			internalErr := utils.NewInternalError("Failed to get balance summary", err)
			c.JSON(internalErr.StatusCode, internalErr.ToErrorResponse(""))
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": summary})
}

// ImportAccounts handles bulk import of accounts from CSV
func (h *AccountHandler) ImportAccounts(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		appError := utils.NewBadRequestError("No file uploaded")
		c.JSON(appError.StatusCode, appError.ToErrorResponse(""))
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		appError := utils.NewBadRequestError("Failed to read CSV file")
		c.JSON(appError.StatusCode, appError.ToErrorResponse(""))
		return
	}

	if len(records) < 2 {
		appError := utils.NewBadRequestError("CSV file must contain header and at least one data row")
		c.JSON(appError.StatusCode, appError.ToErrorResponse(""))
		return
	}

	// Skip header row
	var importRequests []models.AccountImportRequest
	for i, record := range records[1:] {
		if len(record) < 3 {
			appError := utils.NewBadRequestError(fmt.Sprintf("Row %d: insufficient columns", i+2))
			c.JSON(appError.StatusCode, appError.ToErrorResponse(""))
			return
		}

		openingBalance := 0.0
		if len(record) > 4 && record[4] != "" {
			if balance, err := strconv.ParseFloat(record[4], 64); err == nil {
				openingBalance = balance
			}
		}

		importReq := models.AccountImportRequest{
			Code:           strings.TrimSpace(record[0]),
			Name:           strings.TrimSpace(record[1]),
			Type:           models.AccountType(strings.TrimSpace(strings.ToUpper(record[2]))),
			Description:    "",
			OpeningBalance: openingBalance,
		}

		if len(record) > 3 && record[3] != "" {
			importReq.ParentCode = strings.TrimSpace(record[3])
		}
		if len(record) > 5 && record[5] != "" {
			importReq.Description = strings.TrimSpace(record[5])
		}

		importRequests = append(importRequests, importReq)
	}

	err = h.repo.BulkImport(h.requestContextWithUser(c), importRequests)
	if err != nil {
		if appErr := utils.GetAppError(err); appErr != nil {
			c.JSON(appErr.StatusCode, appErr.ToErrorResponse(""))
		} else {
			internalErr := utils.NewInternalError("Failed to import accounts", err)
			c.JSON(internalErr.StatusCode, internalErr.ToErrorResponse(""))
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Accounts imported successfully",
		"count":   len(importRequests),
	})
}

// ExportAccountsPDF exports accounts to PDF
func (h *AccountHandler) ExportAccountsPDF(c *gin.Context) {
	// Get user ID from context (assuming it's set by auth middleware)
	userID := uint(1) // Default fallback, should be extracted from JWT/session
	if userIDValue, exists := c.Get("user_id"); exists {
		if uid, ok := userIDValue.(uint); ok {
			userID = uid
		}
	}

	pdfData, err := h.exportService.ExportAccountsPDF(c.Request.Context(), userID)
	if err != nil {
		if appErr := utils.GetAppError(err); appErr != nil {
			c.JSON(appErr.StatusCode, appErr.ToErrorResponse(""))
		} else {
			internalErr := utils.NewInternalError("Failed to generate PDF", err)
			c.JSON(internalErr.StatusCode, internalErr.ToErrorResponse(""))
		}
		return
	}

	// Set headers for PDF download
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=chart_of_accounts_%s.pdf", time.Now().Format("20060102_150405")))
	c.Header("Content-Length", fmt.Sprintf("%d", len(pdfData)))

	// Write PDF data
	c.Data(http.StatusOK, "application/pdf", pdfData)
}

// GetAccountCatalog gets minimal account data for transaction forms (id, code, name, active).
// This endpoint is for authenticated users to select accounts in purchases, payments, and sales.
func (h *AccountHandler) GetAccountCatalog(c *gin.Context) {
	accountType := c.Query("type")

	// Allow specific types used by operational transaction forms.
	allowedTypes := []string{"EXPENSE", "ASSET", "LIABILITY", "REVENUE"}
	if accountType != "" {
		isAllowed := false
		for _, allowed := range allowedTypes {
			if accountType == allowed {
				isAllowed = true
				break
			}
		}
		if !isAllowed {
			appError := utils.NewBadRequestError(fmt.Sprintf("Account catalog only supports types: %v", allowedTypes))
			c.JSON(appError.StatusCode, appError.ToErrorResponse(""))
			return
		}
	}

	var accounts []models.Account
	var err error

	if accountType != "" {
		// Get accounts for specific type
		accounts, err = h.repo.FindByType(c.Request.Context(), accountType)
	} else {
		// Get accounts for all allowed operational types.
		for _, aType := range allowedTypes {
			typeAccounts, typeErr := h.repo.FindByType(c.Request.Context(), aType)
			if typeErr != nil {
				err = typeErr
				break
			}
			accounts = append(accounts, typeAccounts...)
		}
	}

	if err != nil {
		if appErr := utils.GetAppError(err); appErr != nil {
			c.JSON(appErr.StatusCode, appErr.ToErrorResponse(""))
		} else {
			internalErr := utils.NewInternalError("Failed to retrieve account catalog", err)
			c.JSON(internalErr.StatusCode, internalErr.ToErrorResponse(""))
		}
		return
	}

	// Return minimal data only - no balance or hierarchy
	type AccountCatalogItem struct {
		ID     uint   `json:"id"`
		Code   string `json:"code"`
		Name   string `json:"name"`
		Active bool   `json:"active"`
	}

	catalog := make([]AccountCatalogItem, 0, len(accounts))
	for _, account := range accounts {
		// Catalog is used for transaction selection, so only active detail accounts
		if account.IsActive && !account.IsHeader {
			catalog = append(catalog, AccountCatalogItem{
				ID:     account.ID,
				Code:   account.Code,
				Name:   account.Name,
				Active: account.IsActive,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": catalog, "count": len(catalog)})
}

// ExportAccountsExcel exports accounts to Excel
func (h *AccountHandler) ExportAccountsExcel(c *gin.Context) {
	// Get user ID from context (assuming it's set by auth middleware)
	userID := uint(1) // Default fallback, should be extracted from JWT/session
	if userIDValue, exists := c.Get("user_id"); exists {
		if uid, ok := userIDValue.(uint); ok {
			userID = uid
		}
	}

	excelData, err := h.exportService.ExportAccountsExcel(c.Request.Context(), userID)
	if err != nil {
		if appErr := utils.GetAppError(err); appErr != nil {
			c.JSON(appErr.StatusCode, appErr.ToErrorResponse(""))
		} else {
			internalErr := utils.NewInternalError("Failed to generate Excel", err)
			c.JSON(internalErr.StatusCode, internalErr.ToErrorResponse(""))
		}
		return
	}

	// Set headers for Excel download
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=chart_of_accounts_%s.xlsx", time.Now().Format("20060102_150405")))
	c.Header("Content-Length", fmt.Sprintf("%d", len(excelData)))

	// Write Excel data
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", excelData)
}

// ValidateAccountCode checks if an account code is available
func (h *AccountHandler) ValidateAccountCode(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		appError := utils.NewBadRequestError("Account code is required")
		c.JSON(appError.StatusCode, appError.ToErrorResponse(""))
		return
	}

	excludeId := c.Query("exclude_id") // For updates

	// Check if code exists
	var account models.Account
	query := h.repo.(*repositories.AccountRepo).DB.Where("code = ? AND deleted_at IS NULL", code)
	if excludeId != "" {
		if id, err := strconv.ParseUint(excludeId, 10, 32); err == nil {
			query = query.Where("id != ?", uint(id))
		}
	}

	err := query.First(&account).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Code is available
			c.JSON(http.StatusOK, gin.H{
				"available": true,
				"message":   "Account code is available",
			})
		} else {
			internalErr := utils.NewInternalError("Failed to validate account code", err)
			c.JSON(internalErr.StatusCode, internalErr.ToErrorResponse(""))
		}
		return
	}

	// Code is not available
	c.JSON(http.StatusOK, gin.H{
		"available": false,
		"message":   fmt.Sprintf("Account code '%s' is already used by: %s", code, account.Name),
		"existing_account": gin.H{
			"id":   account.ID,
			"code": account.Code,
			"name": account.Name,
		},
	})
}

// FixAccountHeaderStatus fixes the is_header status for all accounts
func (h *AccountHandler) FixAccountHeaderStatus(c *gin.Context) {
	err := h.repo.(*repositories.AccountRepo).FixAccountHeaderStatus(c.Request.Context())
	if err != nil {
		if appErr := utils.GetAppError(err); appErr != nil {
			c.JSON(appErr.StatusCode, appErr.ToErrorResponse(""))
		} else {
			internalErr := utils.NewInternalError("Failed to fix account header status", err)
			c.JSON(internalErr.StatusCode, internalErr.ToErrorResponse(""))
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Account header status fixed successfully",
		"success": true,
	})
}
