package routes

import (
	"app-sistem-akuntansi/handlers"
	"app-sistem-akuntansi/middleware"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SetupCashBankSSOTRoutes sets up all cash-bank routes with SSOT integration
func SetupCashBankSSOTRoutes(v1 *gin.RouterGroup, db *gorm.DB, jwtManager *middleware.JWTManager) {
	// Initialize repositories
	accountRepo := repositories.NewAccountRepository(db)
	cashBankRepo := repositories.NewCashBankRepository(db)

	// Initialize services
	cashBankService := services.NewCashBankService(db, cashBankRepo, accountRepo)

	// Initialize services depending on repositories
	accountService := services.NewAccountService(accountRepo)

	// Initialize handler
	cashBankHandler := handlers.NewCashBankHandler(cashBankService, accountService)

	// Initialize Permission Middleware
	permMiddleware := middleware.NewPermissionMiddleware(db)

	// Cash-Bank routes with SSOT integration
	cashBankGroup := v1.Group("/cash-bank")
	cashBankGroup.Use(jwtManager.AuthRequired())
	{
		// Account Management
		accounts := cashBankGroup.Group("/accounts")
		{
			accounts.GET("", permMiddleware.CanView("cash_bank"), cashBankHandler.GetCashBankAccounts)
			accounts.GET("/:id", permMiddleware.CanView("cash_bank"), cashBankHandler.GetCashBankByID)
			accounts.POST("", permMiddleware.CanCreate("cash_bank"), cashBankHandler.CreateCashBankAccount)
			accounts.PUT("/:id", permMiddleware.CanEdit("cash_bank"), cashBankHandler.UpdateCashBankAccount)
			accounts.DELETE("/:id", permMiddleware.CanDelete("cash_bank"), cashBankHandler.DeleteCashBankAccount)

			// Transaction History
			accounts.GET("/:id/transactions", permMiddleware.CanView("cash_bank"), cashBankHandler.GetTransactions)

			// Bank Reconciliation
			accounts.POST("/:id/reconcile", permMiddleware.CanEdit("cash_bank"), cashBankHandler.ReconcileAccount)

			// Frozen Fund Management
			accounts.POST("/:id/freeze", permMiddleware.CanEdit("cash_bank"), cashBankHandler.FreezeAmount)
			accounts.POST("/:id/unfreeze", permMiddleware.CanEdit("cash_bank"), cashBankHandler.UnfreezeAmount)
			accounts.GET("/:id/frozen-history", permMiddleware.CanView("cash_bank"), cashBankHandler.GetFrozenHistory)
		}

		// Transaction Processing (all with SSOT journal integration)
		transactions := cashBankGroup.Group("/transactions")
		{
			transactions.POST("/deposit", permMiddleware.CanCreate("cash_bank"), cashBankHandler.ProcessDeposit)
			transactions.POST("/withdrawal", permMiddleware.CanCreate("cash_bank"), cashBankHandler.ProcessWithdrawal)
			transactions.POST("/transfer", permMiddleware.CanCreate("cash_bank"), cashBankHandler.ProcessTransfer)
		}

		// Compatibility routes without /transactions prefix (as documented in Swagger)
		cashBankGroup.POST("/deposit", permMiddleware.CanCreate("cash_bank"), cashBankHandler.ProcessDeposit)
		cashBankGroup.POST("/withdrawal", permMiddleware.CanCreate("cash_bank"), cashBankHandler.ProcessWithdrawal)
		cashBankGroup.POST("/transfer", permMiddleware.CanCreate("cash_bank"), cashBankHandler.ProcessTransfer)

		// Reporting and Summary
		reports := cashBankGroup.Group("/reports")
		{
			reports.GET("/balance-summary", permMiddleware.CanView("cash_bank"), cashBankHandler.GetBalanceSummary)
			reports.GET("/payment-accounts", permMiddleware.CanView("cash_bank"), cashBankHandler.GetPaymentAccounts)
		}

		// Backward-compatibility routes (documented path without /reports prefix)
		cashBankGroup.GET("/balance-summary", permMiddleware.CanView("cash_bank"), cashBankHandler.GetBalanceSummary)
		cashBankGroup.GET("/payment-accounts", permMiddleware.CanView("cash_bank"), cashBankHandler.GetPaymentAccounts)
		cashBankGroup.GET("/deposit-source-accounts", permMiddleware.CanView("cash_bank"), cashBankHandler.GetDepositSourceAccounts)
		cashBankGroup.GET("/reference-rate", permMiddleware.CanView("cash_bank"), cashBankHandler.GetReferenceExchangeRate)
		cashBankGroup.GET("/revenue-accounts", permMiddleware.CanView("cash_bank"), func(c *gin.Context) {
			accounts, err := accountService.GetRevenueAccounts(c.Request.Context())
			if err != nil {
				c.JSON(500, gin.H{"error": "Failed to retrieve revenue accounts", "details": err.Error()})
				return
			}
			c.JSON(200, gin.H{"success": true, "data": accounts})
		})

		// SSOT Integration and Validation
		ssot := cashBankGroup.Group("/ssot")
		{
			ssot.GET("/journals", permMiddleware.CanView("reports"), cashBankHandler.GetSSOTJournalEntries)
			ssot.POST("/validate-integrity", permMiddleware.CanView("reports"), cashBankHandler.ValidateIntegrity)
		}

		// Available GL Accounts for Cash/Bank creation (filtered by type and not already linked)
		cashBankGroup.GET("/available-gl-accounts", permMiddleware.CanView("cash_bank"), cashBankHandler.GetAvailableGLAccounts)
	}
}
