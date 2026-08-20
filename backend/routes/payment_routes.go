package routes

import (
	"app-sistem-akuntansi/controllers"
	"app-sistem-akuntansi/middleware"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetupPaymentRoutes(router *gin.RouterGroup, paymentController *controllers.PaymentController, cashBankController *controllers.CashBankController, cashBankService *services.CashBankService, jwtManager *middleware.JWTManager, db *gorm.DB) {
	// Note: FixCashBankController removed - deprecated admin endpoints
	// Initialize permission middleware
	permissionMiddleware := middleware.NewPermissionMiddleware(db)

	// Initialize CashBank validation middleware and services for Phase 1 sync
	accountingService := services.NewCashBankAccountingService(db)
	validationService := services.NewCashBankValidationService(db, accountingService)
	// Get repositories for enhanced service
	cashBankRepo := repositories.NewCashBankRepository(db)
	accountRepo := repositories.NewAccountRepository(db)
	enhancedCashBankService := services.NewCashBankEnhancedService(db, cashBankRepo, accountRepo)
	cashBankValidationMiddleware := middleware.NewCashBankValidationMiddleware(validationService, enhancedCashBankService)

	// Payment routes
	payment := router.Group("/payments")
	payment.Use(middleware.PaymentRateLimit()) // Apply rate limiting to all payment endpoints
	if middleware.GlobalAuditLogger != nil {
		payment.Use(middleware.GlobalAuditLogger.PaymentAuditMiddleware()) // Apply audit logging
	}
	{
		// Payment routes with permission-based restrictions
		// Re-enable legacy GET endpoints for frontend compatibility (deprecated but safe as read-only)
		payment.GET("", permissionMiddleware.CanView("payments"), paymentController.GetPayments)
		// Optionally keep detail endpoint if frontend needs it (still read-only)
		// payment.GET("/:id", permissionMiddleware.CanView("payments"), paymentController.GetPaymentByID)
		// Deprecated: POST endpoints replaced by SSOT routes
		payment.POST("/:id/cancel", permissionMiddleware.CanEdit("payments"), paymentController.CancelPayment)
		payment.DELETE("/:id", permissionMiddleware.CanDelete("payments"), paymentController.DeletePayment)
		payment.GET("/analytics", permissionMiddleware.CanView("payments"), paymentController.GetPaymentAnalytics)

		// Sales integration routes
		payment.POST("/sales", permissionMiddleware.CanCreate("payments"), paymentController.CreateSalesPayment)
		payment.GET("/sales/unpaid-invoices/:customer_id", permissionMiddleware.CanView("payments"), paymentController.GetSalesUnpaidInvoices)

		// Debug routes removed - deprecated debug endpoints

		// Export routes
		payment.GET("/report/pdf", permissionMiddleware.CanExport("payments"), paymentController.ExportPaymentReportPDF)
		payment.GET("/export/excel", permissionMiddleware.CanExport("payments"), paymentController.ExportPaymentReportExcel)
		payment.GET("/:id/pdf", permissionMiddleware.CanExport("payments"), paymentController.ExportPaymentDetailPDF)
	}

	// Cash & Bank routes
	cashbank := router.Group("/cashbank")
	{
		// Account management
		cashbank.GET("/accounts", permissionMiddleware.CanView("cash_bank"), cashBankController.GetAccounts)

		// Payment accounts endpoint - specifically for payment form dropdowns
		cashbank.GET("/payment-accounts", permissionMiddleware.CanView("cash_bank"), cashBankController.GetPaymentAccounts)

		// Controlled owner-drawing/prive lookup. This is deliberately separate
		// from the generic payment COA selector, which must not expose equity.
		cashbank.GET("/owner-drawing-accounts", permissionMiddleware.CanView("cash_bank"), cashBankController.GetOwnerDrawingAccounts)

		// Revenue accounts endpoint - for deposit source selection
		cashbank.GET("/revenue-accounts", permissionMiddleware.CanView("cash_bank"), cashBankController.GetRevenueAccounts)

		// Deposit source accounts endpoint - for deposit form (revenue + equity)
		cashbank.GET("/deposit-source-accounts", permissionMiddleware.CanView("cash_bank"), cashBankController.GetDepositSourceAccounts)

		cashbank.GET("/accounts/:id", permissionMiddleware.CanView("cash_bank"), cashBankController.GetAccountByID)
		cashbank.POST("/accounts", permissionMiddleware.CanCreate("cash_bank"), cashBankController.CreateAccount)
		cashbank.PUT("/accounts/:id", permissionMiddleware.CanEdit("cash_bank"), cashBankController.UpdateAccount)

		// Transactions
		cashbank.POST("/transfer", permissionMiddleware.CanCreate("cash_bank"), cashBankController.ProcessTransfer)
		cashbank.POST("/deposit", permissionMiddleware.CanCreate("cash_bank"), cashBankController.ProcessDeposit)
		cashbank.POST("/withdrawal", permissionMiddleware.CanCreate("cash_bank"), cashBankController.ProcessWithdrawal)
		cashbank.GET("/accounts/:id/transactions", permissionMiddleware.CanView("cash_bank"), cashBankController.GetTransactions)

		// Reports
		cashbank.GET("/balance-summary", permissionMiddleware.CanView("cash_bank"), cashBankController.GetBalanceSummary)

		// Admin operations removed - deprecated maintenance endpoints
	}

	// 🚀 Phase 1: CashBank-COA Synchronization Routes
	// Add validation middleware routes to router for health checks and sync management
	cashBankValidationMiddleware.AddRoutes(router)

	// Apply validation middleware to cashbank operations for automatic sync checking
	cashbank.Use(cashBankValidationMiddleware.ValidateCashBankSync())
}
