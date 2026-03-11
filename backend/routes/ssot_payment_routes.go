package routes

import (
	"app-sistem-akuntansi/controllers"
	"app-sistem-akuntansi/middleware"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SetupSSOTPaymentRoutes sets up payment routes using SSOT journal integration
func SetupSSOTPaymentRoutes(router *gin.RouterGroup, db *gorm.DB, jwtManager *middleware.JWTManager) {
	// Initialize repositories
	paymentRepo := repositories.NewPaymentRepository(db)
	contactRepo := repositories.NewContactRepository(db)
	cashBankRepo := repositories.NewCashBankRepository(db)
	salesRepo := repositories.NewSalesRepository(db)
	purchaseRepo := repositories.NewPurchaseRepository(db)

	// Initialize SSOT unified journal service
	unifiedJournalService := services.NewUnifiedJournalService(db)

	// Initialize Unified Period Closing Service & Middleware for closed-period validation
	periodClosingService := services.NewUnifiedPeriodClosingService(db)
	periodValidationMiddleware := middleware.NewPeriodValidationMiddleware(periodClosingService)

	// Initialize Enhanced Payment Service with Journal integration
	enhancedPaymentService := services.NewEnhancedPaymentServiceWithJournal(
		db,
		*paymentRepo, // Dereference pointer to get interface implementation
		contactRepo,
		cashBankRepo,
		salesRepo,
		purchaseRepo,
		unifiedJournalService,
	)

	// Initialize SSOT Payment Controller
	ssotPaymentController := controllers.NewSSOTPaymentController(enhancedPaymentService)

	// Initialize Tax Payment Service and Controller
	taxPaymentService := services.NewTaxPaymentService(db)
	taxPaymentController := controllers.NewTaxPaymentController(taxPaymentService)

	// Initialize permission middleware
	permissionMiddleware := middleware.NewPermissionMiddleware(db)

	// SSOT Payment routes - replaces legacy payment routes
	ssotPayments := router.Group("/payments/ssot")
	ssotPayments.Use(middleware.PaymentRateLimit())                          // Apply rate limiting
	ssotPayments.Use(periodValidationMiddleware.ValidateTransactionPeriod()) // Block transactions in closed periods
	if middleware.GlobalAuditLogger != nil {
		ssotPayments.Use(middleware.GlobalAuditLogger.PaymentAuditMiddleware()) // Apply audit logging
	}
	{
		// SSOT Payment CRUD operations with journal integration
		ssotPayments.POST("/receivable", permissionMiddleware.CanCreate("payments"), ssotPaymentController.CreateReceivablePayment)
		ssotPayments.POST("/payable", permissionMiddleware.CanCreate("payments"), ssotPaymentController.CreatePayablePayment)
		ssotPayments.POST("/expense", permissionMiddleware.CanCreate("payments"), ssotPaymentController.CreateExpensePayment)
		ssotPayments.POST("/:id/approve-expense", permissionMiddleware.CanEdit("payments"), ssotPaymentController.ApproveExpensePayment)
		ssotPayments.POST("/:id/approve-standard", permissionMiddleware.CanEdit("payments"), ssotPaymentController.ApproveStandardPayment)
		ssotPayments.PUT("/:id/draft", permissionMiddleware.CanEdit("payments"), ssotPaymentController.UpdateDraftPayment)
		ssotPayments.GET("/:id", permissionMiddleware.CanView("payments"), ssotPaymentController.GetPaymentWithJournal)
		ssotPayments.POST("/:id/reverse", permissionMiddleware.CanEdit("payments"), ssotPaymentController.ReversePayment)

		// Journal integration endpoints
		ssotPayments.POST("/preview-journal", permissionMiddleware.CanView("payments"), ssotPaymentController.PreviewPaymentJournal)
		ssotPayments.GET("/:id/balance-updates", permissionMiddleware.CanView("payments"), ssotPaymentController.GetAccountBalanceUpdates)

		// Legacy compatibility (deprecated - returns guidance)
		ssotPayments.GET("", permissionMiddleware.CanView("payments"), ssotPaymentController.GetPayments)
	}

	// Tax Payment routes (PPN Masukan & Keluaran)
	taxPayments := router.Group("/tax-payments")
	taxPayments.Use(middleware.PaymentRateLimit())                          // Apply rate limiting
	taxPayments.Use(periodValidationMiddleware.ValidateTransactionPeriod()) // Block PPN payments in closed periods
	if middleware.GlobalAuditLogger != nil {
		taxPayments.Use(middleware.GlobalAuditLogger.PaymentAuditMiddleware()) // Apply audit logging
	}
	{
		// PPN Payment CRUD operations with journal integration
		taxPayments.POST("/ppn", permissionMiddleware.CanCreate("payments"), taxPaymentController.CreatePPNPayment)
		taxPayments.GET("/ppn", permissionMiddleware.CanView("payments"), taxPaymentController.GetPPNPayments)
		taxPayments.GET("/ppn/summary", permissionMiddleware.CanView("payments"), taxPaymentController.GetPPNPaymentSummary)
		taxPayments.GET("/ppn/masukan", permissionMiddleware.CanView("payments"), taxPaymentController.GetPPNMasukanPayments)
		taxPayments.GET("/ppn/keluaran", permissionMiddleware.CanView("payments"), taxPaymentController.GetPPNKeluaranPayments)
		taxPayments.GET("/ppn/balance", permissionMiddleware.CanView("payments"), taxPaymentController.GetPPNBalance)
	}

	// Backward-compatibility route to match Swagger-documented path
	router.POST("/payments/preview-journal", permissionMiddleware.CanView("payments"), ssotPaymentController.PreviewPaymentJournal)
	// Aliases for with-journal and account updates
	router.GET("/payments/:id/with-journal", permissionMiddleware.CanView("payments"), ssotPaymentController.GetPaymentWithJournal)
	router.GET("/payments/:id/account-updates", permissionMiddleware.CanView("payments"), ssotPaymentController.GetAccountBalanceUpdates)
	// Alias for reverse
	router.POST("/payments/:id/reverse", periodValidationMiddleware.ValidateTransactionPeriod(), permissionMiddleware.CanEdit("payments"), ssotPaymentController.ReversePayment)
	// Alias for approve expense (finance only)
	router.POST("/payments/:id/approve-expense", permissionMiddleware.CanEdit("payments"), ssotPaymentController.ApproveExpensePayment)
	// Alias for approve standard receivable/payable (finance only)
	router.POST("/payments/:id/approve-standard", permissionMiddleware.CanEdit("payments"), ssotPaymentController.ApproveStandardPayment)

	// Mark legacy payment routes as deprecated by adding a redirect route
	legacyPayments := router.Group("/payments")
	{
		legacyPayments.GET("/deprecated-notice", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"notice":    "Legacy payment endpoints are deprecated",
				"migration": "Please use /api/v1/payments/ssot/* endpoints for full SSOT journal integration",
				"warnings": []string{
					"Legacy endpoints may cause double posting",
					"SSOT endpoints prevent balance inconsistencies",
					"Full journal audit trail available in SSOT endpoints",
				},
				"available_endpoints": []string{
					"POST /api/v1/payments/ssot/receivable - Create customer payment with journal",
					"POST /api/v1/payments/ssot/payable - Create vendor payment with journal",
					"GET /api/v1/payments/ssot/:id - Get payment with journal details",
					"POST /api/v1/payments/ssot/:id/reverse - Reverse payment with journal",
					"POST /api/v1/payments/ssot/preview-journal - Preview journal entry",
					"GET /api/v1/payments/ssot/:id/balance-updates - Get account balance updates",
				},
			})
		})
	}
}
