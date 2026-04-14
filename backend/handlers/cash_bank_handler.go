package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"app-sistem-akuntansi/services"
	"app-sistem-akuntansi/utils"
	"github.com/gin-gonic/gin"
)

type CashBankHandler struct {
	cashBankService *services.CashBankService
	accountService  services.AccountService
}

func NewCashBankHandler(cashBankService *services.CashBankService, accountService services.AccountService) *CashBankHandler {
	return &CashBankHandler{
		cashBankService: cashBankService,
		accountService:  accountService,
	}
}

func classifyCashBankTransactionError(err error) int {
	if err == nil {
		return http.StatusInternalServerError
	}

	if appErr := utils.GetAppError(err); appErr != nil {
		return appErr.StatusCode
	}

	errorText := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errorText, "closed_period:"):
		return http.StatusConflict
	case strings.Contains(errorText, "permission to approve"),
		strings.Contains(errorText, "permission to reject"):
		return http.StatusForbidden
	case strings.Contains(errorText, "not found"),
		strings.Contains(errorText, "inactive"),
		strings.Contains(errorText, "header account"),
		strings.Contains(errorText, "must be different"),
		strings.Contains(errorText, "use transfer instead"),
		strings.Contains(errorText, "insufficient balance"),
		strings.Contains(errorText, "saldo tersedia tidak cukup"),
		strings.Contains(errorText, "amount must be greater than zero"),
		strings.Contains(errorText, "exchange rate required"),
		strings.Contains(errorText, "already been reviewed"),
		strings.Contains(errorText, "counter account"),
		strings.Contains(errorText, "target account"),
		strings.Contains(errorText, "source account"):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// GetCashBankAccounts handles GET /api/cash-bank/accounts
func (h *CashBankHandler) GetCashBankAccounts(c *gin.Context) {
	includeHolding, _ := strconv.ParseBool(c.DefaultQuery("include_holding", "false"))
	accounts, err := h.cashBankService.GetCashBankAccounts(includeHolding)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.CreateErrorResponse("Failed to get cash-bank accounts", err))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Cash-bank accounts retrieved successfully", accounts))
}

// GetCashBankByID handles GET /api/cash-bank/accounts/:id
func (h *CashBankHandler) GetCashBankByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.CreateErrorResponse("Invalid account ID", err))
		return
	}

	account, err := h.cashBankService.GetCashBankByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, utils.CreateErrorResponse("Cash-bank account not found", err))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Cash-bank account retrieved successfully", account))
}

// CreateCashBankAccount handles POST /api/cash-bank/accounts
func (h *CashBankHandler) CreateCashBankAccount(c *gin.Context) {
	var request services.CashBankCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, utils.CreateErrorResponse("Invalid request data", err))
		return
	}

	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.CreateErrorResponse("User not authenticated", nil))
		return
	}

	account, err := h.cashBankService.CreateCashBankAccount(request, userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.CreateErrorResponse("Failed to create cash-bank account", err))
		return
	}

	c.JSON(http.StatusCreated, utils.SuccessResponse("Cash-bank account created successfully", account))
}

// UpdateCashBankAccount handles PUT /api/cash-bank/accounts/:id
func (h *CashBankHandler) UpdateCashBankAccount(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.CreateErrorResponse("Invalid account ID", err))
		return
	}

	var request services.CashBankUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, utils.CreateErrorResponse("Invalid request data", err))
		return
	}

	account, err := h.cashBankService.UpdateCashBankAccount(uint(id), request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.CreateErrorResponse("Failed to update cash-bank account", err))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Cash-bank account updated successfully", account))
}

// DeleteCashBankAccount handles DELETE /api/cash-bank/accounts/:id
func (h *CashBankHandler) DeleteCashBankAccount(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.CreateErrorResponse("Invalid account ID", err))
		return
	}

	err = h.cashBankService.DeleteCashBankAccount(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.CreateErrorResponse("Failed to delete cash-bank account", err))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Cash-bank account deleted successfully", nil))
}

// ProcessDeposit handles POST /api/cash-bank/deposit
func (h *CashBankHandler) ProcessDeposit(c *gin.Context) {
	var request services.DepositRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, utils.CreateErrorResponse("Invalid request data", err))
		return
	}

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.CreateErrorResponse("User not authenticated", nil))
		return
	}

	transaction, err := h.cashBankService.ProcessDeposit(request, userID.(uint))
	if err != nil {
		c.JSON(classifyCashBankTransactionError(err), utils.CreateErrorResponse("Failed to process deposit", err))
		return
	}

	response := gin.H{
		"transaction": transaction,
		"message":     "Deposit processed successfully with SSOT journal entry",
	}

	c.JSON(http.StatusCreated, utils.SuccessResponse("Deposit processed successfully", response))
}

// ProcessWithdrawal handles POST /api/cash-bank/withdrawal
func (h *CashBankHandler) ProcessWithdrawal(c *gin.Context) {
	var request services.WithdrawalRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, utils.CreateErrorResponse("Invalid request data", err))
		return
	}

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.CreateErrorResponse("User not authenticated", nil))
		return
	}

	transaction, err := h.cashBankService.ProcessWithdrawal(request, userID.(uint))
	if err != nil {
		c.JSON(classifyCashBankTransactionError(err), utils.CreateErrorResponse("Failed to process withdrawal", err))
		return
	}

	response := gin.H{
		"transaction": transaction,
		"message":     "Withdrawal processed successfully with SSOT journal entry",
	}

	c.JSON(http.StatusCreated, utils.SuccessResponse("Withdrawal processed successfully", response))
}

// ProcessTransfer handles POST /api/cash-bank/transfer
func (h *CashBankHandler) ProcessTransfer(c *gin.Context) {
	var request services.TransferRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, utils.CreateErrorResponse("Invalid request data", err))
		return
	}

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.CreateErrorResponse("User not authenticated", nil))
		return
	}

	role := c.GetString("role")

	transfer, err := h.cashBankService.ProcessTransfer(request, userID.(uint), role)
	if err != nil {
		c.JSON(classifyCashBankTransactionError(err), utils.CreateErrorResponse("Failed to process transfer", err))
		return
	}

	message := "Transfer processed successfully with SSOT journal entry"
	if transfer.Status == services.TransferStatusPendingApproval {
		message = "Transfer submitted for finance manager approval"
	}

	response := gin.H{
		"transfer": transfer,
		"message":  message,
	}

	c.JSON(http.StatusCreated, utils.SuccessResponse(message, response))
}

// GetTransfers handles GET /api/cash-bank/transfers
func (h *CashBankHandler) GetTransfers(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.CreateErrorResponse("User not authenticated", nil))
		return
	}

	role := c.GetString("role")
	status := c.Query("status")
	limit := utils.ParseIntWithDefault(c.DefaultQuery("limit", "20"), 20)

	transfers, err := h.cashBankService.GetTransfers(userID.(uint), role, status, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.CreateErrorResponse("Failed to retrieve transfer requests", err))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Transfer requests retrieved successfully", transfers))
}

// ApproveTransfer handles POST /api/cash-bank/transfers/:id/approve
func (h *CashBankHandler) ApproveTransfer(c *gin.Context) {
	transferID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.CreateErrorResponse("Invalid transfer ID", err))
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.CreateErrorResponse("User not authenticated", nil))
		return
	}

	role := c.GetString("role")
	transfer, err := h.cashBankService.ApproveTransfer(uint(transferID), userID.(uint), role)
	if err != nil {
		c.JSON(classifyCashBankTransactionError(err), utils.CreateErrorResponse("Failed to approve transfer request", err))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Transfer request approved successfully", gin.H{
		"transfer": transfer,
		"message":  "Transfer approved successfully",
	}))
}

// RejectTransfer handles POST /api/cash-bank/transfers/:id/reject
func (h *CashBankHandler) RejectTransfer(c *gin.Context) {
	transferID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.CreateErrorResponse("Invalid transfer ID", err))
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.CreateErrorResponse("User not authenticated", nil))
		return
	}

	var request struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&request); err != nil && !strings.Contains(strings.ToLower(err.Error()), "eof") {
		c.JSON(http.StatusBadRequest, utils.CreateErrorResponse("Invalid request data", err))
		return
	}

	role := c.GetString("role")
	transfer, err := h.cashBankService.RejectTransfer(uint(transferID), userID.(uint), role, request.Reason)
	if err != nil {
		c.JSON(classifyCashBankTransactionError(err), utils.CreateErrorResponse("Failed to reject transfer request", err))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Transfer request rejected successfully", gin.H{
		"transfer": transfer,
		"message":  "Transfer rejected successfully",
	}))
}

// GetTransactions handles GET /api/cash-bank/accounts/:id/transactions
func (h *CashBankHandler) GetTransactions(c *gin.Context) {
	accountID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.CreateErrorResponse("Invalid account ID", err))
		return
	}

	// Parse query parameters for filtering
	filter := services.TransactionFilter{
		Type:      c.Query("type"),
		SortOrder: c.DefaultQuery("sort_order", "asc"),
		Page:      utils.ParseIntWithDefault(c.Query("page"), 1),
		Limit:     utils.ParseIntWithDefault(c.Query("limit"), 20),
	}

	// Parse date filters (inclusive end-date at end of day, Jakarta timezone)
	du := utils.NewDateUtils()
	if startDate := c.Query("start_date"); startDate != "" {
		if parsedDate, err := du.ParseDateTimeWithTZ(startDate); err == nil {
			filter.StartDate = parsedDate
		}
	}

	if endDate := c.Query("end_date"); endDate != "" {
		if endOfDay, err := du.ParseEndDateTimeWithTZ(endDate); err == nil {
			filter.EndDate = endOfDay
		}
	}

	result, err := h.cashBankService.GetTransactions(uint(accountID), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.CreateErrorResponse("Failed to get transactions", err))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Transactions retrieved successfully", result))
}

// GetBalanceSummary handles GET /api/cash-bank/balance-summary
func (h *CashBankHandler) GetBalanceSummary(c *gin.Context) {
	summary, err := h.cashBankService.GetBalanceSummary()
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.CreateErrorResponse("Failed to get balance summary", err))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Balance summary retrieved successfully", summary))
}

// GetPaymentAccounts handles GET /api/cash-bank/payment-accounts
func (h *CashBankHandler) GetPaymentAccounts(c *gin.Context) {
	accounts, err := h.cashBankService.GetPaymentAccounts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.CreateErrorResponse("Failed to retrieve payment accounts", err))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Payment accounts retrieved successfully", accounts))
}

// GetDepositSourceAccounts handles GET /api/cash-bank/deposit-source-accounts
func (h *CashBankHandler) GetDepositSourceAccounts(c *gin.Context) {
	if h.accountService == nil {
		c.JSON(http.StatusInternalServerError, utils.CreateErrorResponse("Account service not available", nil))
		return
	}
	// Fetch revenue and equity accounts via account service
	revenue, err := h.accountService.GetRevenueAccounts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.CreateErrorResponse("Failed to retrieve revenue accounts", err))
		return
	}
	equity, err := h.accountService.GetEquityAccounts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.CreateErrorResponse("Failed to retrieve equity accounts", err))
		return
	}
	c.JSON(http.StatusOK, utils.SuccessResponse("Deposit source accounts retrieved successfully", gin.H{
		"revenue": revenue,
		"equity":  equity,
	}))
}

// GetReferenceExchangeRate handles GET /api/cash-bank/reference-rate
func (h *CashBankHandler) GetReferenceExchangeRate(c *gin.Context) {
	baseCurrency := c.DefaultQuery("base", "USD")
	targetCurrency := c.DefaultQuery("target", "IDR")

	rate, err := h.cashBankService.GetReferenceExchangeRate(baseCurrency, targetCurrency)
	if err != nil {
		c.JSON(http.StatusBadGateway, utils.CreateErrorResponse("Failed to retrieve exchange rate reference", err))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Exchange rate reference retrieved successfully", rate))
}

// ReconcileAccount handles POST /api/cash-bank/accounts/:id/reconcile
func (h *CashBankHandler) ReconcileAccount(c *gin.Context) {
	accountID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.CreateErrorResponse("Invalid account ID", err))
		return
	}

	var request services.ReconciliationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, utils.CreateErrorResponse("Invalid request data", err))
		return
	}

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.CreateErrorResponse("User not authenticated", nil))
		return
	}

	reconciliation, err := h.cashBankService.ReconcileAccount(uint(accountID), request, userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.CreateErrorResponse("Failed to reconcile account", err))
		return
	}

	c.JSON(http.StatusCreated, utils.SuccessResponse("Account reconciliation completed", reconciliation))
}

// GetSSOTJournalEntries handles GET /api/cash-bank/ssot-journals
func (h *CashBankHandler) GetSSOTJournalEntries(c *gin.Context) {
	// This endpoint provides visibility into SSOT journal entries for cash-bank transactions
	// This could be useful for auditing and verification purposes

	// For now, we'll return a placeholder response
	// In a full implementation, this would query the SSOT journal entries
	// filtered by source_type = 'CASH_BANK'

	c.JSON(http.StatusOK, utils.SuccessResponse("SSOT journal entries endpoint", gin.H{
		"message": "This endpoint will show SSOT journal entries for cash-bank transactions",
		"note":    "Implementation would query unified_journal_ledger where source_type = 'CASH_BANK'",
	}))
}

// ValidateIntegrity handles POST /api/cash-bank/validate-integrity
func (h *CashBankHandler) ValidateIntegrity(c *gin.Context) {
	// This endpoint validates the integrity of cash-bank SSOT integration
	// It checks that all cash-bank transactions have corresponding SSOT journal entries

	c.JSON(http.StatusOK, utils.SuccessResponse("SSOT integration validation", gin.H{
		"message": "This endpoint validates cash-bank SSOT integration integrity",
		"note":    "Implementation would use CashBankSSOTJournalAdapter.ValidateJournalIntegrity()",
	}))
}

// GetAvailableGLAccounts handles GET /api/cash-bank/available-gl-accounts
// Returns GL accounts that are not already linked to any cash/bank account
// and filtered by type (CASH shows accounts with "KAS" in name, BANK shows accounts with "BANK" in name)
func (h *CashBankHandler) GetAvailableGLAccounts(c *gin.Context) {
	accountType := c.Query("type")

	// Validate type parameter
	if accountType != "CASH" && accountType != "BANK" {
		c.JSON(http.StatusBadRequest, utils.CreateErrorResponse("Invalid account type. Must be CASH or BANK", nil))
		return
	}

	accounts, err := h.cashBankService.GetAvailableGLAccounts(accountType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.CreateErrorResponse("Failed to retrieve available GL accounts", err))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Available GL accounts retrieved successfully", accounts))
}

// FreezeAmount handles POST /api/cash-bank/accounts/:id/freeze
func (h *CashBankHandler) FreezeAmount(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.CreateErrorResponse("Invalid account ID", err))
		return
	}

	var request services.FreezeAmountRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, utils.CreateErrorResponse("Invalid request data", err))
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.CreateErrorResponse("User not authenticated", nil))
		return
	}

	account, err := h.cashBankService.FreezeAmount(uint(id), request, userID.(uint))
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.CreateErrorResponse("Failed to freeze funds", err))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Dana berhasil diblokir", account))
}

// UnfreezeAmount handles POST /api/cash-bank/accounts/:id/unfreeze
func (h *CashBankHandler) UnfreezeAmount(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.CreateErrorResponse("Invalid account ID", err))
		return
	}

	var request services.UnfreezeAmountRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, utils.CreateErrorResponse("Invalid request data", err))
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.CreateErrorResponse("User not authenticated", nil))
		return
	}

	account, err := h.cashBankService.UnfreezeAmount(uint(id), request, userID.(uint))
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.CreateErrorResponse("Failed to unfreeze funds", err))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Dana berhasil dibuka blokirnya", account))
}

// GetFrozenHistory handles GET /api/cash-bank/accounts/:id/frozen-history
func (h *CashBankHandler) GetFrozenHistory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.CreateErrorResponse("Invalid account ID", err))
		return
	}

	history, err := h.cashBankService.GetFrozenHistory(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.CreateErrorResponse("Failed to get frozen history", err))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Frozen history retrieved successfully", history))
}
