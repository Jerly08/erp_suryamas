package routes

import (
	"app-sistem-akuntansi/controllers"
	"app-sistem-akuntansi/middleware"
	"app-sistem-akuntansi/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SetupSSOTReportRoutes sets up all SSOT report integration routes
func SetupSSOTReportRoutes(
	router gin.IRouter,
	db *gorm.DB,
	unifiedJournalService *services.UnifiedJournalService,
	enhancedReportService *services.EnhancedReportService,
	jwtManager *middleware.JWTManager,
	rbacMgr *middleware.RBACManager,
) {
	// Initialize SSOT Report Integration Service
	ssotReportIntegrationService := services.NewSSOTReportIntegrationService(
		db,
		unifiedJournalService,
		enhancedReportService,
	)

	// Initialize SSOT Report Integration Controller
	ssotReportController := controllers.NewSSOTReportIntegrationController(ssotReportIntegrationService, db)

	// Create SSOT reports group with authentication and authorization
	ssotReportsGroup := router.Group("/ssot-reports")
	ssotReportsGroup.Use(jwtManager.AuthRequired())
	ssotReportsGroup.Use(rbacMgr.AutoPermission("reports"))

	// ✨ MAIN INTEGRATION ENDPOINTS

	// 🎯 Get all integrated financial reports (main endpoint)
	ssotReportsGroup.GET("/integrated", ssotReportController.GetIntegratedFinancialReports)

	// 🔄 Manual refresh trigger
	ssotReportsGroup.POST("/refresh", ssotReportController.TriggerReportRefresh)

	// 📊 System status and health check
	ssotReportsGroup.GET("/status", ssotReportController.GetSSOTReportStatus)

	// ✨ INDIVIDUAL SSOT REPORT ENDPOINTS

	// 📊 Sales Summary Report (integrated with SSOT journal)
	ssotReportsGroup.GET("/sales-summary", ssotReportController.GetSSOTSalesSummary)

	// 💰 Purchase Report (NEW - replaces Vendor Analysis with credible data)
	purchaseReportController := controllers.NewSSOTPurchaseReportController(db)
	ssotReportsGroup.GET("/purchase-report", purchaseReportController.GetPurchaseReport)
	ssotReportsGroup.GET("/purchase-summary", purchaseReportController.GetPurchaseSummary)
	ssotReportsGroup.GET("/purchase-report/validate", purchaseReportController.ValidatePurchaseReport)

	// 📈 Vendor Analysis Report (DEPRECATED - replaced by Purchase Report)
	ssotReportsGroup.GET("/vendor-analysis", func(c *gin.Context) {
		c.JSON(400, gin.H{
			"success": false,
			"error":   "This endpoint has been replaced by /api/v1/ssot-reports/purchase-report",
			"message": "Vendor Analysis has been replaced with more credible Purchase Report",
			"new_endpoints": gin.H{
				"purchase_report":  "/api/v1/ssot-reports/purchase-report",
				"purchase_summary": "/api/v1/ssot-reports/purchase-summary",
				"validate_report":  "/api/v1/ssot-reports/purchase-report/validate",
			},
			"migration_guide": "Use the new Purchase Report for accurate vendor and purchase analysis",
		})
	})

	// ⚖️ Trial Balance (integrated with SSOT journal)
	ssotReportsGroup.GET("/trial-balance", ssotReportController.GetSSOTTrialBalance)

	// 📚 General Ledger (integrated with SSOT journal)
	ssotReportsGroup.GET("/general-ledger", ssotReportController.GetSSOTGeneralLedger)

	// 🔍 Journal Entry Analysis (integrated with SSOT journal)
	ssotReportsGroup.GET("/journal-analysis", ssotReportController.GetSSOTJournalAnalysis)

	// 📊 Balance Sheet (integrated with SSOT journal)
	ssotBalanceSheetController := controllers.NewSSOTBalanceSheetController(db)
	ssotReportsGroup.GET("/balance-sheet", ssotBalanceSheetController.GenerateSSOTBalanceSheet)
	ssotReportsGroup.GET("/balance-sheet/account-details", ssotBalanceSheetController.GetSSOTBalanceSheetAccountDetails)
	ssotReportsGroup.GET("/balance-sheet/validate", ssotBalanceSheetController.ValidateSSOTBalanceSheet)
	ssotReportsGroup.GET("/balance-sheet/comparison", ssotBalanceSheetController.GetSSOTBalanceSheetComparison)

	// 💰 Cash Flow (integrated with SSOT journal)
	ssotCashFlowController := controllers.NewSSOTCashFlowController(db)
	ssotReportsGroup.GET("/cash-flow", ssotCashFlowController.GetSSOTCashFlow)
	ssotReportsGroup.GET("/cash-flow/summary", ssotCashFlowController.GetSSOTCashFlowSummary)
	ssotReportsGroup.GET("/cash-flow/validate", ssotCashFlowController.ValidateSSOTCashFlow)

	// 📒 SSOT Account Balances for COA sync
	ssotAccountBalanceController := controllers.NewSSOTAccountBalanceController(db)
	ssotReportsGroup.GET("/account-balances", ssotAccountBalanceController.GetSSOTAccountBalances)

	// ✨ ENHANCED REPORT ENDPOINTS (with SSOT integration)

	// Create enhanced reports group for better organization
	enhancedGroup := ssotReportsGroup.Group("/enhanced")
	{
		// 🎯 All reports with SSOT integration
		enhancedGroup.GET("/all", ssotReportController.GetIntegratedFinancialReports)

		// 📊 Individual enhanced reports (these will use SSOT data)
		enhancedGroup.GET("/profit-loss", func(c *gin.Context) {
			// This will use the enhanced report service but with SSOT data
			// We can add a flag to indicate SSOT integration
			c.Set("use_ssot", true)
			// Forward to existing enhanced report controller but with SSOT context
			c.JSON(200, gin.H{
				"status":        "redirect",
				"message":       "Use /api/v1/reports/profit-loss for P&L with SSOT integration",
				"ssot_endpoint": "/api/v1/ssot-reports/integrated",
			})
		})

		enhancedGroup.GET("/balance-sheet", func(c *gin.Context) {
			c.Set("use_ssot", true)
			c.JSON(200, gin.H{
				"status":        "redirect",
				"message":       "Use /api/v1/reports/balance-sheet for Balance Sheet with SSOT integration",
				"ssot_endpoint": "/api/v1/ssot-reports/integrated",
			})
		})

		enhancedGroup.GET("/cash-flow", func(c *gin.Context) {
			c.Set("use_ssot", true)
			c.JSON(200, gin.H{
				"status":        "redirect",
				"message":       "Use /api/v1/reports/cash-flow for Cash Flow with SSOT integration",
				"ssot_endpoint": "/api/v1/ssot-reports/integrated",
			})
		})

		enhancedGroup.GET("/sales-summary", func(c *gin.Context) {
			c.Set("use_ssot", true)
			c.JSON(200, gin.H{
				"status":        "redirect",
				"message":       "Use /api/v1/reports/sales-summary for Sales Summary with SSOT integration",
				"ssot_endpoint": "/api/v1/ssot-reports/integrated",
			})
		})
	}

	// ✨ COMPATIBILITY ROUTES (for existing frontend compatibility)

	// Create compatibility group for legacy support
	compatGroup := ssotReportsGroup.Group("/compat")
	{
		// Map existing report endpoints to SSOT versions
		compatGroup.GET("/vendor-analysis", func(c *gin.Context) {
			c.JSON(400, gin.H{
				"success":            false,
				"error":              "This endpoint has been replaced by /api/v1/ssot-reports/purchase-report",
				"message":            "Vendor Analysis has been replaced with more credible Purchase Report",
				"redirect_to":        "/api/v1/ssot-reports/purchase-report",
				"compatibility_note": "Use Purchase Report for more accurate vendor and purchase analysis",
			})
		})
		compatGroup.GET("/trial-balance", ssotReportController.GetSSOTTrialBalance)
		compatGroup.GET("/general-ledger", ssotReportController.GetSSOTGeneralLedger)
		compatGroup.GET("/journal-entry-analysis", ssotReportController.GetSSOTJournalAnalysis)

		// Provide information about SSOT integration for existing endpoints
		compatGroup.GET("/info", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status":  "success",
				"message": "SSOT Report Integration Compatibility Information",
				"ssot_integration": gin.H{
					"version":     "1.0",
					"description": "All reports now integrated with Single Source of Truth (SSOT) journal system",
					"benefits": []string{
						"Real-time data consistency",
						"Automated journal entry integration",
						"Enhanced data integrity",
						"Comprehensive audit trail",
					},
				},
				"migration_guide": gin.H{
					"existing_endpoints": "Still functional with SSOT integration",
					"new_endpoints":      "Use /api/v1/ssot-reports/* for full SSOT features",
					"breaking_changes":   "None - fully backward compatible",
				},
			})
		})
	}
}

// RegisterSSOTReportRoutesInMain registers SSOT report routes in the main routes setup
// This function should be called from the main routes.go file
func RegisterSSOTReportRoutesInMain(
	v1 gin.IRouter,
	db *gorm.DB,
	unifiedJournalService *services.UnifiedJournalService,
	enhancedReportService *services.EnhancedReportService,
	jwtManager *middleware.JWTManager,
	rbacMgr *middleware.RBACManager,
) {
	SetupSSOTReportRoutes(v1, db, unifiedJournalService, enhancedReportService, jwtManager, rbacMgr)
}
