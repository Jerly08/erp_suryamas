package routes

import (
	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/controllers"
	"app-sistem-akuntansi/handlers"
	"app-sistem-akuntansi/middleware"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/services"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Environment detection helper
func getEnvironment() string {
	env := strings.ToLower(os.Getenv("ENV"))
	if env == "" {
		env = strings.ToLower(os.Getenv("GO_ENV"))
	}
	if env == "" {
		env = strings.ToLower(os.Getenv("ENVIRONMENT"))
	}
	if env == "" {
		env = "development" // default
	}
	return env
}

// Check if development features should be enabled
func isDevelopmentMode() bool {
	env := getEnvironment()
	return env == "development" || env == "dev" || env == "local"
}

// Check if debug routes should be enabled (requires explicit flag)
func shouldEnableDebugRoutes() bool {
	return os.Getenv("ENABLE_DEBUG_ROUTES") == "true" && isDevelopmentMode()
}

// Dangerous accounting maintenance endpoints are disabled by default.
// Enable them only for explicit repair sessions.
func shouldEnableAccountingMaintenanceRoutes() bool {
	return strings.EqualFold(os.Getenv("ENABLE_ACCOUNTING_MAINTENANCE_API"), "true")
}

// loadOpenAPIJSON loads the single committed OpenAPI source regardless of working directory.
func loadOpenAPIJSON() ([]byte, string, error) {
	candidates := []string{
		"./openapi/openapi.json",
		"./backend/openapi/openapi.json",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			b, err := os.ReadFile(p)
			if err != nil {
				return nil, p, err
			}
			return b, p, nil
		}
	}
	// Try relative to executable directory
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		try1 := filepath.Join(dir, "openapi", "openapi.json")
		if _, err := os.Stat(try1); err == nil {
			b, err := os.ReadFile(try1)
			if err != nil {
				return nil, try1, err
			}
			return b, try1, nil
		}
		try2 := filepath.Join(dir, "backend", "openapi", "openapi.json")
		if _, err := os.Stat(try2); err == nil {
			b, err := os.ReadFile(try2)
			if err != nil {
				return nil, try2, err
			}
			return b, try2, nil
		}
	}
	return nil, "", fmt.Errorf("openapi source not found (looked for openapi/openapi.json)")
}

// minimalOpenAPIJSON returns a small, valid Swagger document so the UI can load even if the full spec isn't available.
func minimalOpenAPIJSON() []byte {
	return []byte(`{
  "swagger": "2.0",
  "info": {
    "title": "Sistema Akuntansi API",
    "version": "1.0.0",
    "description": "Minimal spec fallback so Swagger UI can load. Restore backend/openapi/openapi.json to view the committed API contract."
  },
  "basePath": "/",
  "paths": {}
}`)
}

// swaggerCSPMiddleware relaxes CSP only for Swagger UI so it can execute inline scripts
// We set these headers BEFORE calling next handlers so they override stricter global policies
func swaggerCSPMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Ensure we override any global CSP header set earlier
		c.Writer.Header().Del("Content-Security-Policy")
		// Allow CDN for Swagger UI assets and inline styles/scripts used by Swagger UI
		c.Header("Content-Security-Policy", "default-src 'self' https: data: blob:; script-src 'self' 'unsafe-inline' 'unsafe-eval' https:; style-src 'self' 'unsafe-inline' https:; img-src 'self' data: https:; font-src 'self' data: https:; connect-src 'self' data: blob: https:")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Next()
	}
}

// swaggerIndexHTML returns a minimal Swagger UI page using CDN assets
func swaggerIndexHTML(docURL string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <style>html,body,#swagger{height:100%%;margin:0;padding:0}</style>
</head>
<body>
  <div id="swagger"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
  window.onload = function() {
    window.ui = SwaggerUIBundle({ url: '%s', dom_id: '#swagger' });
  };
  </script>
</body>
</html>`, docURL)
}

func SetupRoutes(r *gin.Engine, db *gorm.DB, startupService *services.StartupService) {
	// Controllers
	authController := controllers.NewAuthController(db)
	userController := controllers.NewUserController(db)
	permissionController := controllers.NewPermissionController(db)
	categoryController := controllers.NewCategoryController(db)
	unitController := controllers.NewProductUnitController(db)
	inventoryController := controllers.NewInventoryController(db)
	assetController := controllers.NewAssetController(db)
	debugController := controllers.NewDebugController()
	monitoringController := controllers.NewMonitoringController()
	payrollController := controllers.NewPayrollController(db)

	// Initialize repositories, services and handlers
	accountRepo := repositories.NewAccountRepository(db)
	exportService := services.NewExportService(accountRepo, db)
	accountHandler := handlers.NewAccountHandler(accountRepo, exportService)

	// Initialize startup handler for startup service monitoring
	startupHandler := handlers.NewStartupHandler(startupService)

	// Contact repositories, services and controllers
	contactRepo := repositories.NewContactRepository(db)
	contactService := services.NewContactService(contactRepo)
	contactController := controllers.NewContactController(contactService)

	// Notification repositories, services and handlers
	notificationRepo := repositories.NewNotificationRepository(db)
	notificationService := services.NewNotificationService(db, notificationRepo)
	// Initialize Stock Monitoring service and Dashboard controller
	stockMonitoringService := services.NewStockMonitoringService(db, notificationService)
	notificationHandler := handlers.NewNotificationHandler(notificationService, stockMonitoringService)
	dashboardController := controllers.NewDashboardController(db, stockMonitoringService)

	// Purchase Requests Management - Init
	purchaseRequestRepo := repositories.NewPurchaseRequestRepository(db)
	purchaseRequestService := services.NewPurchaseRequestService(db, purchaseRequestRepo, notificationService)
	purchaseRequestController := controllers.NewPurchaseRequestController(purchaseRequestService)

	bomService := services.NewBOMService(db)

	// Update ProductController with stockMonitoringService and bomService
	productController := controllers.NewProductController(db, stockMonitoringService, bomService)

	// Initialize WarehouseLocationController
	warehouseLocationController := controllers.NewWarehouseLocationController(db)

	// Initialize BOMController
	bomController := controllers.NewBOMController(db)

	// Fiscal Year Closing Service removed - now using Unified Period Closing Service
	// which provides more flexibility and consistent closing logic

	// Initialize SSOT Unified Journal Service first (needed by period closing)
	unifiedJournalService := services.NewUnifiedJournalService(db)

	// Initialize Period Closing Service and Controller (Unified Journal-based)
	unifiedPeriodClosingService := services.NewUnifiedPeriodClosingService(db)
	periodClosingController := controllers.NewPeriodClosingController(unifiedPeriodClosingService)

	// Initialize Unified Closing History Controller (combines all closing sources)
	unifiedClosingHistoryController := controllers.NewUnifiedClosingHistoryController(db)

	// Initialize Period Validation Middleware
	periodValidationMiddleware := middleware.NewPeriodValidationMiddleware(unifiedPeriodClosingService)

	// Project repositories, services and controllers
	projectRepo := repositories.NewProjectRepository(db)
	projectService := services.NewProjectService(projectRepo)
	projectController := controllers.NewProjectController(projectService)

	// Initialize ProjectBOMController
	projectBOMController := controllers.NewProjectBOMController(db)
	bomProcurementController := controllers.NewBOMProcurementController(db)

	// Purchase repositories, services and controllers
	purchaseRepo := repositories.NewPurchaseRepository(db)
	productRepo := repositories.NewProductRepository(db)
	approvalService := services.NewApprovalService(db)
	// Initialize services needed for purchase service
	journalRepo := repositories.NewJournalEntryRepository(db)
	pdfService := services.NewPDFService(db)
	coaService := services.NewCOAService(db)

	// Initialize Settings Service earlier (needed for Sales/Purchase journal services)
	settingsService := services.NewSettingsService(db)

	// Initialize Purchase Journal Service SSOT (unified_journal_ledger - for Balance Sheet integration)
	purchaseJournalServiceSSOT := services.NewPurchaseJournalServiceSSOT(db, coaService, settingsService)

	purchaseService := services.NewPurchaseService(
		db,
		purchaseRepo,
		productRepo,
		contactRepo,
		accountRepo,
		approvalService,
		nil, // journal service - can be nil for now
		journalRepo,
		pdfService,
		unifiedJournalService,      // Add unified journal service for SSOT integration
		coaService,                 // Add COA service for journal V2 integration
		purchaseJournalServiceSSOT, // NEW: SSOT service for Balance Sheet integration
		stockMonitoringService,
	)
	// Handlers that depend on services (purchaseController will be initialized later)
	purchaseApprovalHandler := handlers.NewPurchaseApprovalHandler(purchaseService, approvalService)
	// Employee approval handler
	employeeDashboardService := services.NewEmployeeDashboardService(db)
	employeeApprovalHandler := handlers.NewEmployeeApprovalHandler(approvalService, employeeDashboardService)

	// Initialize security middleware
	middleware.InitAuditLogger(db)  // Initialize audit logging
	middleware.InitTokenMonitor(db) // Initialize token monitoring

	// 📝 Initialize Activity Logger Service and Middleware
	activityLoggerService := services.NewActivityLoggerService(db, "./logs")
	middleware.InitActivityLogger(activityLoggerService)
	log.Println("✅ Activity Logger initialized successfully")

	// Initialize Activity Log Controller
	activityLogController := controllers.NewActivityLogController()

	// Initialize Security controller for security dashboard
	securityController := controllers.NewSecurityController(db)

	// Initialize Journal Drilldown controller
	journalDrilldownController := controllers.NewJournalDrilldownController(db)

	// Journal Entry controller removed - migrated to SSOT unified system

	// Initialize SSOT Unified Journal Controller (service already initialized above)
	unifiedJournalController := controllers.NewUnifiedJournalController(unifiedJournalService)

	// Initialize JWT Manager
	jwtManager := middleware.NewJWTManager(db)

	// Initialize Session Cleanup Service and Controller
	sessionCleanupService := services.NewSessionCleanupService(db)
	sessionController := controllers.NewSessionController(sessionCleanupService)

	// Initialize Debug Auth Controller
	debugAuthController := controllers.NewDebugAuthController()

	// Initialize Permission Middleware
	permMiddleware := middleware.NewPermissionMiddleware(db)

	// Initialize RBACManager for dynamic module permissions (AutoPermission)
	rbacMgr := middleware.NewRBACManager(db)

	accountingMaintenanceEnabled := shouldEnableAccountingMaintenanceRoutes()
	if accountingMaintenanceEnabled {
		log.Println("⚠️ Accounting maintenance API is ENABLED. Repair/backfill endpoints are exposed.")
	} else {
		log.Println("✅ Accounting maintenance API is disabled. Repair/backfill endpoints are not exposed.")
	}

	// 🔒 Initialize Enhanced Security Middleware
	enhancedSecurity := middleware.NewEnhancedSecurityMiddleware(db)

	// 🛡️ Apply panic recovery middleware FIRST to catch all panics
	r.Use(middleware.RecoverPanic()) // 🛡️ Recover from panics and return proper error responses

	// 🎛️ Apply global security middleware
	r.Use(enhancedSecurity.SecurityHeaders())    // Security headers pada semua requests
	r.Use(enhancedSecurity.RequestMonitoring())  // Monitor semua requests untuk threats
	r.Use(middleware.APIUsageMiddleware())       // 📊 Track API usage for optimization
	r.Use(middleware.ActivityLoggerMiddleware()) // 📝 Log all user activities

	// 📋 Apply audit logging middleware for financial transactions
	if middleware.GlobalAuditLogger != nil {
		r.Use(middleware.GlobalAuditLogger.AuditMiddleware()) // 📋 Audit critical transactions
	}

	// API v1 routes
	v1 := r.Group("/api/v1")

	// Add root-level compatibility routes for frontend AFTER v1 group is created
	setupFrontendCompatibilityRoutes(r, v1)
	{
		// 🔐 Auth routes (minimal public access)
		auth := v1.Group("/auth")
		auth.Use(middleware.AuthRateLimit())         // Apply auth rate limiting
		auth.Use(enhancedSecurity.SecurityHeaders()) // Extra security for auth endpoints
		{
			auth.POST("/login", authController.Login)
			// 🔒 PRODUCTION: Disable register endpoint in production
			if isDevelopmentMode() || os.Getenv("ALLOW_REGISTRATION") == "true" {
				auth.POST("/register", authController.Register)
			}
			auth.POST("/refresh", authController.RefreshToken)

			// Token validation endpoint (requires auth)
			auth.GET("/validate-token", jwtManager.AuthRequired(), authController.ValidateToken)
			auth.GET("/session-info", jwtManager.AuthRequired(), authController.GetSessionInfo)
		}

		// 🔍 Debug auth routes (development only)
		if isDevelopmentMode() {
			debugAuth := v1.Group("/debug-auth")
			{
				// Public debug endpoint - no auth required
				debugAuth.GET("/headers", debugAuthController.DebugAuthHeader)
				// Protected debug endpoint - requires auth
				debugAuth.GET("/session", jwtManager.AuthRequired(), debugAuthController.DebugSessionValidation)
			}
		}

		// 📊 Journal Entry Drilldown routes at API level (accessible by finance, admin, director)
		// These routes are at /api/v1/journal-drilldown to match frontend expectations
		journalDrilldownAPI := v1.Group("/journal-drilldown")
		journalDrilldownAPI.Use(jwtManager.AuthRequired())
		{
			// Main drill-down endpoint for POST requests with detailed filtering
			journalDrilldownAPI.POST("", permMiddleware.CanView("journal"), journalDrilldownController.GetJournalDrilldown)

			// Alternative GET endpoint for simpler URL-based filtering
			journalDrilldownAPI.GET("/entries", permMiddleware.CanView("journal"), journalDrilldownController.GetJournalDrilldownByParams)

			// Get detailed information for a specific journal entry
			journalDrilldownAPI.GET("/entries/:id", permMiddleware.CanView("journal"), journalDrilldownController.GetJournalEntryDetail)

			// Get accounts that have activity in a period (useful for filters)
			journalDrilldownAPI.GET("/accounts", permMiddleware.CanView("journal"), journalDrilldownController.GetAccountsForPeriod)
		}

		// 📋 Journal Entry Management routes (accessible by finance, admin, director)
		journalEntriesAPI := v1.Group("/journal-entries")
		journalEntriesAPI.Use(jwtManager.AuthRequired())
		{
			// CRUD operations for journal entries

			// Summary endpoint - MISSING ROUTE ADDED
		}

		// 📓 SSOT Journal System routes (NEW - unified journal management)
		unifiedJournals := v1.Group("/journals")
		unifiedJournals.Use(jwtManager.AuthRequired())
		{
			// Main CRUD operations
			unifiedJournals.POST("", permMiddleware.CanCreate("journal"), periodValidationMiddleware.ValidateTransactionPeriod(), unifiedJournalController.CreateJournalEntry)
			unifiedJournals.GET("", permMiddleware.CanView("journal"), unifiedJournalController.GetJournalEntries)
			unifiedJournals.GET("/adjustment-categories", permMiddleware.CanView("journal"), unifiedJournalController.GetAdjustmentCategories)
			unifiedJournals.GET("/:id", permMiddleware.CanView("journal"), unifiedJournalController.GetJournalEntry)
			unifiedJournals.PUT("/:id", permMiddleware.CanEdit("journal"), periodValidationMiddleware.ValidateTransactionPeriod(), unifiedJournalController.UpdateJournalEntry)
			unifiedJournals.DELETE("/:id", permMiddleware.CanDelete("journal"), periodValidationMiddleware.ValidateTransactionPeriod(), unifiedJournalController.DeleteJournalEntry)

			// Status operations
			unifiedJournals.PUT("/:id/post", permMiddleware.CanApprove("journal"), periodValidationMiddleware.ValidateTransactionPeriod(), unifiedJournalController.PostJournalEntry)
			unifiedJournals.POST("/:id/reverse", permMiddleware.CanApprove("journal"), periodValidationMiddleware.ValidateTransactionPeriod(), unifiedJournalController.ReverseJournalEntry)

			// Balance management
			unifiedJournals.GET("/account-balances", permMiddleware.CanView("journal"), unifiedJournalController.GetAccountBalances)
			unifiedJournals.POST("/account-balances/refresh", permMiddleware.CanEdit("journal"), unifiedJournalController.RefreshAccountBalances)

			// Summary and reporting
			unifiedJournals.GET("/summary", permMiddleware.CanView("journal"), unifiedJournalController.GetJournalSummary)
		}

		// 🔒 SECURITY: Secure debug routes (development only dengan multiple security layers)
		if shouldEnableDebugRoutes() {
			debugAuth := v1.Group("/debug")
			debugAuth.Use(enhancedSecurity.EnvironmentGate("development", "dev")) // ✅ Environment restriction
			debugAuth.Use(enhancedSecurity.IPWhitelist())                         // ✅ IP whitelisting
			debugAuth.Use(jwtManager.AuthRequired())                              // ✅ Authentication required
			debugAuth.Use(middleware.RoleRequired("admin"))                       // ✅ Admin only access
			debugAuth.Use(middleware.RateLimit())                                 // ✅ Rate limiting
			{
				// 🔍 Read-only endpoints untuk debugging (safe)
				debugAuth.GET("/contacts", contactController.GetContacts)
				debugAuth.GET("/contacts/:id", contactController.GetContact)
				debugAuth.GET("/contacts/type/:type", contactController.GetContactsByType)
				debugAuth.GET("/contacts/search", contactController.SearchContacts)

				// 📊 Debug system information
				// debugAuth.GET("/system/info", debugController.GetSystemInfo)
				// debugAuth.GET("/database/health", debugController.GetDatabaseHealth)

				// ⚠️  ALL WRITE OPERATIONS COMPLETELY REMOVED FOR SECURITY
				// No CREATE/UPDATE/DELETE operations allowed in any debug route
			}
		}

		// Protected routes (auth required)
		protected := v1.Group("")
		protected.Use(jwtManager.AuthRequired())
		{
			// Profile routes
			protected.GET("/profile", authController.Profile)
			protected.GET("/accounts/catalog", accountHandler.GetAccountCatalog)
			protected.GET("/accounts/credit", accountHandler.GetAccountCatalog)

			// User management routes
			users := protected.Group("/users")
			users.Use(rbacMgr.AutoPermission("users"))
			{
				users.GET("", userController.GetUsers)
				users.GET("/:id", userController.GetUser)
				users.POST("", userController.CreateUser)
				users.PUT("/:id", userController.UpdateUser)
				users.DELETE("/:id", userController.DeleteUser)
			}

			// Permission management routes
			permissions := protected.Group("/permissions")
			{
				// Admin only routes
				permissions.GET("/users", rbacMgr.AutoPermission("users"), permissionController.GetAllUsersPermissions)
				permissions.GET("/users/:userId", rbacMgr.AutoPermission("users"), permissionController.GetUserPermissions)
				permissions.PUT("/users/:userId", rbacMgr.AutoPermission("users"), permissionController.UpdateUserPermissions)
				permissions.POST("/users/:userId/reset", rbacMgr.AutoPermission("users"), permissionController.ResetToDefaultPermissions)

				// Self permission routes (any authenticated user)
				permissions.GET("/me", permissionController.GetMyPermissions)       // User can get their own permissions
				permissions.GET("/check", permissionController.CheckUserPermission) // User can check their own permission
			}

			// Dashboard routes
			dashboard := protected.Group("/dashboard")
			{
				dashboard.GET("/analytics", permMiddleware.CanView("reports"), dashboardController.GetAnalytics)
				dashboard.GET("/finance", middleware.RoleRequired("admin", "finance", "finance_manager", "finance_staff"), dashboardController.GetFinanceDashboardData)
				dashboard.GET("/finance-staff", middleware.RoleRequired("admin", "finance", "finance_manager", "finance_staff"), dashboardController.GetFinanceStaffDashboardData)
				dashboard.GET("/stock-alerts", middleware.RoleRequired("admin", "inventory_manager", "director"), dashboardController.GetStockAlertsBanner)
				dashboard.POST("/stock-alerts/:id/dismiss", middleware.RoleRequired("admin", "inventory_manager"), dashboardController.DismissStockAlert)

				// Employee dashboard routes
				dashboard.GET("/employee", dashboardController.GetEmployeeDashboardData)
				dashboard.GET("/employee/workflows", dashboardController.GetEmployeeApprovalWorkflows)
				dashboard.GET("/employee/purchase-requests", dashboardController.GetEmployeePurchaseRequests)
				dashboard.GET("/employee/approval-notifications", dashboardController.GetEmployeeApprovalNotifications)
				dashboard.GET("/employee/purchase-approval-status", dashboardController.GetEmployeePurchaseApprovalStatus)
				dashboard.GET("/employee/notifications-summary", dashboardController.GetEmployeeNotificationsSummary)
				dashboard.PATCH("/employee/notifications/:id/read", dashboardController.MarkNotificationAsRead)
			}

			// 📦 Product routes with enhanced permission checks dan inventory monitoring
			products := protected.Group("/products")
			products.Use(enhancedSecurity.RequestMonitoring()) // 📊 Enhanced monitoring
			// if middleware.GlobalAuditLogger != nil {
			//	products.Use(middleware.GlobalAuditLogger.InventoryAuditMiddleware()) // 📋 Inventory audit
			// }
			{
				// Basic CRUD operations dengan enhanced security
				products.GET("", permMiddleware.CanView("products"), productController.GetProducts)
				products.GET("/:id", permMiddleware.CanView("products"), productController.GetProduct)
				products.POST("", permMiddleware.CanCreate("products"), productController.CreateProduct)
				products.PUT("/:id", permMiddleware.CanEdit("products"), productController.UpdateProduct)
				products.DELETE("/:id", permMiddleware.CanDelete("products"), productController.DeleteProduct)

				// 📊 Critical inventory operations dengan extra monitoring
				products.POST("/:id/backfill-fulfillment", permMiddleware.CanEdit("products"), enhancedSecurity.RequestMonitoring(), productController.BackfillFulfillment)
				products.POST("/adjust-stock", permMiddleware.CanEdit("products"), enhancedSecurity.RequestMonitoring(), periodValidationMiddleware.ValidateTransactionPeriod(), productController.AdjustStock)
				products.POST("/opname", permMiddleware.CanEdit("products"), enhancedSecurity.RequestMonitoring(), periodValidationMiddleware.ValidateTransactionPeriod(), productController.Opname)
				products.POST("/upload-image", permMiddleware.CanEdit("products"), productController.UploadProductImage)
				products.POST("/import", permMiddleware.CanCreate("products"), productController.ImportProducts)
			}

			// 📋 BOM (Bill of Materials) routes
			boms := protected.Group("/boms")
			{
				boms.GET("", permMiddleware.CanView("boms"), bomController.GetBOMs)
				boms.GET("/:id", permMiddleware.CanView("boms"), bomController.GetBOM)
				boms.POST("", permMiddleware.CanCreate("boms"), bomController.CreateBOM)
				boms.PUT("/:id", permMiddleware.CanEdit("boms"), bomController.UpdateBOM)
				boms.DELETE("/:id", permMiddleware.CanDelete("boms"), bomController.DeleteBOM)
			}

			// 📋 Component Templates routes (reusable component presets for BOM)
			ctController := controllers.NewComponentTemplateController(db)
			compTemplates := protected.Group("/component-templates")
			{
				compTemplates.GET("", permMiddleware.CanView("boms"), ctController.GetTemplates)
				compTemplates.GET("/:id", permMiddleware.CanView("boms"), ctController.GetTemplate)
				compTemplates.POST("", permMiddleware.CanCreate("boms"), ctController.CreateTemplate)
				compTemplates.PUT("/:id", permMiddleware.CanEdit("boms"), ctController.UpdateTemplate)
				compTemplates.DELETE("/:id", permMiddleware.CanDelete("boms"), ctController.DeleteTemplate)
			}

			// 📂 Category routes with enhanced permission checks
			categories := protected.Group("/categories")
			{
				categories.GET("", permMiddleware.CanView("products"), categoryController.GetCategories)
				categories.GET("/tree", permMiddleware.CanView("products"), categoryController.GetCategoryTree)
				categories.GET("/:id", permMiddleware.CanView("products"), categoryController.GetCategory)
				categories.GET("/:id/products", permMiddleware.CanView("products"), categoryController.GetCategoryProducts)
				categories.POST("", permMiddleware.CanCreate("products"), categoryController.CreateCategory)
				categories.PUT("/:id", permMiddleware.CanEdit("products"), categoryController.UpdateCategory)
				categories.DELETE("/:id", permMiddleware.CanDelete("products"), categoryController.DeleteCategory)
			}

			// 📏 Product Units routes with enhanced permission checks
			units := protected.Group("/product-units")
			{
				units.GET("", permMiddleware.CanView("products"), unitController.GetProductUnits)
				units.GET("/:id", permMiddleware.CanView("products"), unitController.GetProductUnit)
				units.POST("", permMiddleware.CanCreate("products"), unitController.CreateProductUnit)
				units.PUT("/:id", permMiddleware.CanEdit("products"), unitController.UpdateProductUnit)
				units.DELETE("/:id", permMiddleware.CanDelete("products"), unitController.DeleteProductUnit)
			}

			// 🏢 Warehouse Location routes with enhanced permission checks
			warehouseLocations := protected.Group("/warehouse-locations")
			{
				warehouseLocations.GET("", permMiddleware.CanView("products"), warehouseLocationController.GetWarehouseLocations)
				warehouseLocations.GET("/:id", permMiddleware.CanView("products"), warehouseLocationController.GetWarehouseLocation)
				warehouseLocations.POST("", permMiddleware.CanCreate("products"), warehouseLocationController.CreateWarehouseLocation)
				warehouseLocations.PUT("/:id", permMiddleware.CanEdit("products"), warehouseLocationController.UpdateWarehouseLocation)
				warehouseLocations.DELETE("/:id", permMiddleware.CanDelete("products"), warehouseLocationController.DeleteWarehouseLocation)
			}

			// 📊 Account routes (Chart of Accounts) dengan enhanced security
			accounts := protected.Group("/accounts")
			accounts.Use(enhancedSecurity.RequestMonitoring()) // 📊 Enhanced monitoring
			// if middleware.GlobalAuditLogger != nil {
			//	accounts.Use(middleware.GlobalAuditLogger.AccountAuditMiddleware()) // 📋 Financial audit
			// }
			{
				accounts.GET("", permMiddleware.CanView("accounts"), accountHandler.ListAccounts)

				accounts.GET("/hierarchy", permMiddleware.CanView("accounts"), accountHandler.GetAccountHierarchy)
				accounts.GET("/balance-summary", permMiddleware.CanView("accounts"), accountHandler.GetBalanceSummary)
				accounts.GET("/validate-code", permMiddleware.CanView("accounts"), accountHandler.ValidateAccountCode)

				if accountingMaintenanceEnabled {
					// Fix account header status
					accounts.POST("/fix-header-status", middleware.RoleRequired("admin"), accountHandler.FixAccountHeaderStatus)

					// Recalculate all parent/header account balances from children
					accounts.POST("/recalculate-parent-balances", permMiddleware.CanEdit("accounts"), func(c *gin.Context) {
						balanceSyncSvc := services.NewBalanceSyncService(db)
						count, err := balanceSyncSvc.RecalculateAllParentBalances()
						if err != nil {
							c.JSON(500, gin.H{"error": "Failed to recalculate parent balances: " + err.Error()})
							return
						}
						c.JSON(200, gin.H{
							"message":         "Parent balances recalculated successfully",
							"headers_updated": count,
						})
					})
				}

				accounts.GET("/:code", permMiddleware.CanView("accounts"), accountHandler.GetAccount)
				accounts.POST("", permMiddleware.CanCreate("accounts"), accountHandler.CreateAccount)
				accounts.PUT("/:code", permMiddleware.CanEdit("accounts"), accountHandler.UpdateAccount)
				accounts.DELETE("/:code", permMiddleware.CanDelete("accounts"), accountHandler.DeleteAccount)
				// Admin-only delete with cascade options
				accounts.DELETE("/admin/:code", middleware.RoleRequired("admin"), accountHandler.AdminDeleteAccount)
				accounts.POST("/import", permMiddleware.CanCreate("accounts"), accountHandler.ImportAccounts)

				// Export routes
				accounts.GET("/export/pdf", permMiddleware.CanExport("accounts"), accountHandler.ExportAccountsPDF)
				accounts.GET("/export/excel", permMiddleware.CanExport("accounts"), accountHandler.ExportAccountsExcel)
			}

			// 📊 COA Display routes (V2 with proper balance display)
			coaDisplayServiceV2 := services.NewCOADisplayServiceV2(db)
			coaControllerV2 := controllers.NewCOAControllerV2(coaDisplayServiceV2)
			postedCOAController := controllers.NewCOAPostedController(db)

			coadisplay := protected.Group("/coa-display")
			{
				coadisplay.GET("", permMiddleware.CanView("accounts"), coaControllerV2.GetCOAWithDisplay)
				coadisplay.GET("/:id", permMiddleware.CanView("accounts"), coaControllerV2.GetCOAByID)
				coadisplay.GET("/by-type", permMiddleware.CanView("accounts"), coaControllerV2.GetCOABalancesByType)
				coadisplay.GET("/specific", permMiddleware.CanView("accounts"), coaControllerV2.GetSpecificAccounts)
				coadisplay.GET("/sales-related", permMiddleware.CanView("accounts"), coaControllerV2.GetSalesRelatedAccounts)
			}

			// 🔒 SSOT Posted-only COA endpoint for frontend
			coaPosted := protected.Group("/coa")
			{
				coaPosted.GET("/posted-balances", permMiddleware.CanView("accounts"), postedCOAController.GetPostedBalances)
			}

			// 📞 Contact routes with enhanced permission checks dan audit logging
			contacts := protected.Group("/contacts")
			contacts.Use(enhancedSecurity.RequestMonitoring()) // 📊 Enhanced monitoring
			// if middleware.GlobalAuditLogger != nil {
			//	contacts.Use(middleware.GlobalAuditLogger.ContactAuditMiddleware()) // 📋 Audit logging
			// }
			{
				// Basic CRUD operations dengan enhanced security
				contacts.GET("", permMiddleware.CanView("contacts"), contactController.GetContacts)
				contacts.GET("/:id", permMiddleware.CanView("contacts"), contactController.GetContact)
				contacts.POST("", permMiddleware.CanCreate("contacts"), contactController.CreateContact)
				contacts.PUT("/:id", permMiddleware.CanEdit("contacts"), contactController.UpdateContact)
				contacts.DELETE("/:id", permMiddleware.CanDelete("contacts"), contactController.DeleteContact)

				// Advanced operations
				contacts.GET("/type/:type", permMiddleware.CanView("contacts"), contactController.GetContactsByType)
				contacts.GET("/search", permMiddleware.CanView("contacts"), contactController.SearchContacts)

				// Import/Export operations
				contacts.POST("/import", permMiddleware.CanCreate("contacts"), contactController.ImportContacts)
				contacts.GET("/export", permMiddleware.CanExport("contacts"), contactController.ExportContacts)

				// Address management
				contacts.POST("/:id/addresses", permMiddleware.CanEdit("contacts"), contactController.AddContactAddress)
				contacts.PUT("/:id/addresses/:address_id", permMiddleware.CanEdit("contacts"), contactController.UpdateContactAddress)
				contacts.DELETE("/:id/addresses/:address_id", permMiddleware.CanEdit("contacts"), contactController.DeleteContactAddress)
			}

			// Sales repositories, services and controllers
			salesRepo := repositories.NewSalesRepository(db)
			productRepo := repositories.NewProductRepository(db)

			// Initialize services for Sales V2
			coaService := services.NewCOAService(db)

			// Initialize Sales Journal Service V2 (clean implementation)
			salesJournalServiceV2 := services.NewSalesJournalServiceV2(db, journalRepo, coaService)

			// Initialize Sales Journal Service SSOT (unified_journal_ledger - for Balance Sheet integration)
			salesJournalServiceSSOT := services.NewSalesJournalServiceSSOT(db, coaService, settingsService)

			// Initialize Stock Service (can be nil if not available)
			stockService := services.NewStockService(db)

			// Initialize Settings Service for sales code generation - ALREADY INITIALIZED ABOVE
			// settingsService := services.NewSettingsService(db)

			// Initialize Invoice services
			invoiceNumberService := services.NewInvoiceNumberService(db)
			invoiceTypeService := services.NewInvoiceTypeService(db)

			// Initialize Sales Service V2 (clean implementation with proper status-based journal posting)
			salesServiceV2 := services.NewSalesServiceV2(db, salesRepo, salesJournalServiceV2, salesJournalServiceSSOT, stockService, stockMonitoringService, notificationService, settingsService, invoiceNumberService)

			// Initialize Payment repositories, services and controllers
			paymentRepo := repositories.NewPaymentRepository(db)
			cashBankRepo := repositories.NewCashBankRepository(db)

			// Initialize PurchasePaymentJournalService for SSOT payment journal integration
			// Note: PurchaseService will be injected later if needed for SSOT integration
			purchasePaymentJournalService := services.NewPurchasePaymentJournalService(db, accountRepo, unifiedJournalService, nil)

			// Initialize PaymentService with PurchasePaymentJournalService for proper credit purchase payment handling
			paymentService := services.NewPaymentService(db, paymentRepo, salesRepo, purchaseRepo, cashBankRepo, accountRepo, contactRepo, purchasePaymentJournalService)
			paymentController := controllers.NewPaymentController(paymentService)
			cashBankService := services.NewCashBankService(db, cashBankRepo, accountRepo)

			// Initialize additional Sales Payment services required by SalesController
			unifiedSalesPaymentService := services.NewUnifiedSalesPaymentService(db)

			// Initialize SalesController with new V2 Sales Service (inject pdfService)
			salesController := controllers.NewSalesController(salesServiceV2, paymentService, unifiedSalesPaymentService, pdfService, db, accountRepo)

			// Initialize PurchaseController with PaymentService integration (moved here after paymentService is available)
			purchaseController := controllers.NewPurchaseController(purchaseService, paymentService, db, accountRepo)

			// 🔔 Notification routes (accessible by all authenticated users)
			notifs := protected.Group("/notifications")
			{
				// Notification routes are generally accessible to all authenticated users since they're personal
				notifs.GET("", notificationHandler.GetNotifications)
				notifs.GET("/summary", notificationHandler.GetNotificationSummary)
				notifs.GET("/unread-count", notificationHandler.GetUnreadCount)
				notifs.DELETE("/:id", notificationHandler.DeleteNotification)
				notifs.PUT("/:id/read", notificationHandler.MarkNotificationAsRead)
				notifs.PUT("/read-all", notificationHandler.MarkAllNotificationsAsRead)
				notifs.GET("/type/:type", notificationHandler.GetNotificationsByType)
				notifs.GET("/approvals", notificationHandler.GetApprovalNotifications)
			}

			// Purchase Requests routes with permission checks
			purchaseRequests := protected.Group("/purchase-requests")
			{
				purchaseRequests.GET("", permMiddleware.CanView("purchase_requests"), purchaseRequestController.GetAll)
				purchaseRequests.GET("/:id", permMiddleware.CanView("purchase_requests"), purchaseRequestController.GetByID)
				purchaseRequests.POST("", permMiddleware.CanCreate("purchase_requests"), purchaseRequestController.Create)
				purchaseRequests.GET("/:id/documents", permMiddleware.CanView("purchase_requests"), purchaseRequestController.GetDocuments)
				purchaseRequests.POST("/:id/documents", permMiddleware.CanEdit("purchase_requests"), purchaseRequestController.UploadDocuments)
				purchaseRequests.DELETE("/documents/:document_id", permMiddleware.CanEdit("purchase_requests"), purchaseRequestController.DeleteDocument)

				// Approval & Rejection workflows
				purchaseRequests.PUT("/:id/review", permMiddleware.CanReviewPurchaseRequests(), purchaseRequestController.Review)
				purchaseRequests.PUT("/:id/approve", permMiddleware.CanReviewPurchaseRequests(), purchaseRequestController.Approve)
				purchaseRequests.PUT("/:id/reject", permMiddleware.CanReviewPurchaseRequests(), purchaseRequestController.Reject)
			}

			// Projects routes with permission checks
			projects := protected.Group("/projects")
			{
				projects.GET("", permMiddleware.CanView("projects"), projectController.GetAll)
				projects.GET("/:id", permMiddleware.CanView("projects"), projectController.GetByID)
				projects.POST("", permMiddleware.CanCreate("projects"), projectController.Create)
				projects.PUT("/:id", permMiddleware.CanEdit("projects"), projectController.Update)
				projects.DELETE("/:id", permMiddleware.CanDelete("projects"), projectController.Delete)

				// BOM Integration
				projects.GET("/:id/boms", permMiddleware.CanView("projects"), projectBOMController.GetProjectBOMs)
				projects.POST("/:id/boms", permMiddleware.CanEdit("projects"), projectBOMController.AddBOMToProject)
				projects.DELETE("/:id/boms/:bomId", permMiddleware.CanEdit("projects"), projectBOMController.RemoveBOMFromProject)
				projects.POST("/:id/generate-pr", permMiddleware.CanCreate("purchase_requests"), bomProcurementController.GeneratePR)
			}

			// Sales routes with permission checks
			sales := protected.Group("/sales")
			{
				// Basic CRUD operations
				sales.GET("", permMiddleware.CanView("sales"), salesController.GetSales)
				sales.GET("/:id", permMiddleware.CanView("sales"), salesController.GetSale)
				// Validate stock for sales create form
				sales.POST("/validate-stock", permMiddleware.CanCreate("sales"), salesController.ValidateSaleStock)
				sales.POST("", permMiddleware.CanCreate("sales"), periodValidationMiddleware.ValidateTransactionPeriod(), salesController.CreateSale)
				sales.PUT("/:id", permMiddleware.CanEdit("sales"), periodValidationMiddleware.ValidateTransactionPeriod(), salesController.UpdateSale)
				sales.DELETE("/:id", permMiddleware.CanDelete("sales"), salesController.DeleteSale)

				// Status management
				sales.POST("/:id/confirm", rbacMgr.RequireAnyPermission("sales:approve", "sales:edit"), periodValidationMiddleware.ValidateTransactionPeriod(), salesController.ConfirmSale)
				sales.POST("/:id/invoice", rbacMgr.RequireAnyPermission("sales:approve", "sales:edit"), periodValidationMiddleware.ValidateTransactionPeriod(), salesController.InvoiceSale)
				sales.POST("/:id/cancel", rbacMgr.RequireAnyPermission("sales:approve", "sales:edit"), periodValidationMiddleware.ValidateTransactionPeriod(), salesController.CancelSale)

				// Payment management
				sales.GET("/:id/payments", rbacMgr.RequireAnyPermission("sales:view", "payments:view"), salesController.GetSalePayments)
				sales.POST("/:id/payments", rbacMgr.RequireAnyPermission("sales:edit", "payments:create"), periodValidationMiddleware.ValidateTransactionPeriod(), salesController.CreateSalePayment)
				sales.POST("/:id/pph23", rbacMgr.RequireAnyPermission("sales:edit", "sales:approve"), periodValidationMiddleware.ValidateTransactionPeriod(), salesController.AddLateSalePPh23)
				sales.POST("/:id/pph23/settlement", rbacMgr.RequireAnyPermission("sales:edit", "sales:approve"), periodValidationMiddleware.ValidateTransactionPeriod(), salesController.SettleSalePPh23)

				// Integrated Payment Management routes
				sales.GET("/:id/for-payment", rbacMgr.RequireAnyPermission("sales:view", "payments:view"), salesController.GetSaleForPayment)
				sales.POST("/:id/integrated-payment", rbacMgr.RequireAnyPermission("sales:edit", "payments:create"), periodValidationMiddleware.ValidateTransactionPeriod(), salesController.CreateIntegratedPayment)

				// Returns management
				sales.POST("/:id/returns", rbacMgr.RequireAnyPermission("sales:edit", "sales:create"), periodValidationMiddleware.ValidateTransactionPeriod(), salesController.CreateSaleReturn)
				sales.GET("/returns", permMiddleware.CanView("sales"), salesController.GetSaleReturns)

				// Analytics and reporting
				sales.GET("/summary", permMiddleware.CanView("sales"), salesController.GetSalesSummary)
				sales.GET("/analytics", permMiddleware.CanView("sales"), salesController.GetSalesAnalytics)
				sales.GET("/receivables", permMiddleware.CanView("sales"), salesController.GetReceivablesReport)

				// PDF exports
				sales.GET("/:id/invoice/pdf", permMiddleware.CanExport("sales"), salesController.ExportSaleInvoicePDF)
				sales.GET("/:id/receipt/pdf", permMiddleware.CanExport("sales"), salesController.ExportSaleReceiptPDF)
				sales.GET("/report/pdf", permMiddleware.CanExport("sales"), salesController.ExportSalesReportPDF)
				sales.GET("/report/csv", permMiddleware.CanExport("sales"), salesController.ExportSalesReportCSV)

				// Customer portal
				sales.GET("/customer/:customer_id", permMiddleware.CanView("sales"), salesController.GetCustomerSales)
				sales.GET("/customer/:customer_id/invoices", permMiddleware.CanView("sales"), salesController.GetCustomerInvoices)
			}

			// Initialize Invoice Type Controller
			invoiceTypeController := controllers.NewInvoiceTypeController(invoiceTypeService, invoiceNumberService)

			// Invoice Types Management routes
			invoiceTypes := protected.Group("/invoice-types")
			{
				// Basic CRUD operations
				invoiceTypes.GET("", permMiddleware.CanView("settings"), invoiceTypeController.GetInvoiceTypes)
				invoiceTypes.GET("/active", permMiddleware.CanView("sales"), invoiceTypeController.GetActiveInvoiceTypes) // For dropdowns
				invoiceTypes.GET("/:id", permMiddleware.CanView("settings"), invoiceTypeController.GetInvoiceType)
				invoiceTypes.POST("", permMiddleware.CanCreate("settings"), invoiceTypeController.CreateInvoiceType)
				invoiceTypes.PUT("/:id", permMiddleware.CanEdit("settings"), invoiceTypeController.UpdateInvoiceType)
				invoiceTypes.DELETE("/:id", permMiddleware.CanDelete("settings"), invoiceTypeController.DeleteInvoiceType)

				// Status management
				invoiceTypes.POST("/:id/toggle", permMiddleware.CanEdit("settings"), invoiceTypeController.ToggleInvoiceType)

				// Invoice numbering utilities
				invoiceTypes.POST("/preview-number", permMiddleware.CanView("settings"), invoiceTypeController.PreviewInvoiceNumber)
				invoiceTypes.GET("/:id/preview", permMiddleware.CanView("settings"), invoiceTypeController.PreviewInvoiceNumberByID)
				invoiceTypes.GET("/:id/counter-history", permMiddleware.CanView("settings"), invoiceTypeController.GetCounterHistory)

				// Counter management (admin-only for safety)
				invoiceTypes.POST("/:id/reset-counter", permMiddleware.CanEdit("settings"), invoiceTypeController.ResetCounterForYear)
			}

			// 📄 Setup Invoice routes
			SetupInvoiceRoutes(protected, db)

			// Initialize Balance Monitoring service and controller
			balanceMonitoringService := services.NewBalanceMonitoringService(db)
			balanceMonitoringController := controllers.NewBalanceMonitoringController(balanceMonitoringService)

			// Initialize Balance Health controller
			balanceHealthController := controllers.NewBalanceHealthController(db)

			// Initialize FixCashBankController (using existing cashBankService)
			fixCashBankController := controllers.NewFixCashBankController(db, cashBankService)

			// Initialize API Usage Monitoring controller
			apiUsageController := controllers.NewAPIUsageController()

			// Initialize Performance Monitoring controller
			performanceController := controllers.NewPerformanceController(db)

			// Legacy payment write routes stay disabled to avoid double posting and balance drift.
			log.Printf("✅ Legacy payment routes disabled - using SalesJournalServiceV2 consistent flow only")

			// ✅ NEW: Setup SSOT Payment routes with journal integration (prevents double posting)
			SetupSSOTPaymentRoutes(protected, db, jwtManager)

			// 📄 Setup Receipt routes
			SetupReceiptRoutes(protected, db, jwtManager)

			// 📄 Export-only compatibility routes for Payments (safe, read-only)
			// These endpoints restore PDF/Excel exports without enabling legacy write routes
			paymentExports := protected.Group("/payments")
			{
				paymentExports.GET("/report/pdf", permMiddleware.CanExport("payments"), paymentController.ExportPaymentReportPDF)
				paymentExports.GET("/export/excel", permMiddleware.CanExport("payments"), paymentController.ExportPaymentReportExcel)
				paymentExports.GET("/:id/pdf", permMiddleware.CanExport("payments"), paymentController.ExportPaymentDetailPDF)
			}

			// 🔁 Compatibility read-only routes required by frontend forms
			// These are safe GET endpoints and do not perform any write operations
			paymentsCompat := protected.Group("/payments")
			{
				paymentsCompat.GET("/analytics", permMiddleware.CanView("payments"), paymentController.GetPaymentAnalytics)
				// Summary statistics for payments (matches Swagger: GET /api/v1/payments/summary)
				paymentsCompat.GET("/summary", permMiddleware.CanView("payments"), paymentController.GetPaymentSummary)
				// Delete payment follows module delete permission
				paymentsCompat.DELETE("/:id", permMiddleware.CanDelete("payments"), paymentController.DeletePayment)
				// Cancel payment (edit permissions)
				paymentsCompat.POST("/:id/cancel", permMiddleware.CanEdit("payments"), paymentController.CancelPayment)
				// Unpaid invoices for receivable payment form
				paymentsCompat.GET("/sales/unpaid-invoices/:customer_id", permMiddleware.CanView("payments"), paymentController.GetSalesUnpaidInvoices)
				// Alias without /sales prefix (as documented in Swagger)
				paymentsCompat.GET("/unpaid-invoices/:customer_id", permMiddleware.CanView("payments"), paymentController.GetSalesUnpaidInvoices)
				// Unpaid bills for payable payment form
				paymentsCompat.GET("/unpaid-bills/:vendor_id", permMiddleware.CanView("payments"), paymentController.GetUnpaidBills)
			}

			// ⚡ ULTRA-FAST: Setup Ultra-Fast Payment routes with minimal operations
			ultraFastRoutes := NewUltraFastPaymentRoutes(db)
			ultraFastRoutes.SetupUltraFastPaymentRoutes(r)

			// ⚙️ Setup Settings routes
			SetupSettingsRoutes(protected, db, rbacMgr)

			// 🔄 Setup CashBank SSOT Integration routes (NEW - Phase 1 Implementation)
			// This provides unified view of CashBank data integrated with SSOT Journal system
			SetupCashBankIntegratedRoutes(protected, db, jwtManager)

			// 💰 Setup NEW Cash-Bank routes with SSOT integration
			SetupCashBankSSOTRoutes(v1, db, jwtManager)

			// 💰 Purchases routes with enhanced permission checks
			purchases := protected.Group("/purchases")
			purchases.Use(enhancedSecurity.RequestMonitoring()) // 📊 Enhanced monitoring
			{
				// Basic CRUD operations dengan enhanced security
				purchases.GET("", permMiddleware.CanView("purchases"), purchaseController.GetPurchases)
				// Approval statistics (must be defined before parameterized "/:id" route)
				purchases.GET("/approval-stats", permMiddleware.CanUsePurchaseApprovalWorkflow(), purchaseApprovalHandler.GetApprovalStats)
				// Vendor lookup/quick-create belongs to the Purchase workflow and does
				// not require access to the standalone Contact module.
				purchases.GET("/vendors", permMiddleware.CanView("purchases"), contactController.GetVendorsForPurchase)
				purchases.POST("/vendors", permMiddleware.CanCreate("purchases"), contactController.CreateVendorForPurchase)
				purchases.GET("/:id", permMiddleware.CanView("purchases"), purchaseController.GetPurchase)
				purchases.POST("", permMiddleware.CanCreate("purchases"), periodValidationMiddleware.ValidateTransactionPeriod(), purchaseController.CreatePurchase)
				purchases.PUT("/:id", permMiddleware.CanEdit("purchases"), periodValidationMiddleware.ValidateTransactionPeriod(), purchaseController.UpdatePurchase)
				purchases.DELETE("/:id", permMiddleware.CanDelete("purchases"), periodValidationMiddleware.ValidateTransactionPeriod(), purchaseController.DeletePurchase)

				// Approval operations dengan permission checks
				purchases.POST("/:id/submit-approval", permMiddleware.CanCreate("purchases"), periodValidationMiddleware.ValidateTransactionPeriod(), purchaseController.SubmitForApproval)
				purchases.POST("/:id/approve", permMiddleware.CanUsePurchaseApprovalWorkflow(), periodValidationMiddleware.ValidateTransactionPeriod(), purchaseController.ApprovePurchase)
				purchases.POST("/:id/reject", permMiddleware.CanUsePurchaseApprovalWorkflow(), periodValidationMiddleware.ValidateTransactionPeriod(), purchaseController.RejectPurchase)
				purchases.POST("/:id/accounting-coa", rbacMgr.RequireAnyPermission("purchases:edit", "purchases:approve"), purchaseController.CompleteAccountingCOA)
				purchases.POST("/:id/final-review-config", permMiddleware.CanApprove("purchases"), periodValidationMiddleware.ValidateTransactionPeriod(), purchaseController.UpdateFinalReviewConfig)
				purchases.POST("/:id/final-approval", permMiddleware.CanApprove("purchases"), periodValidationMiddleware.ValidateTransactionPeriod(), purchaseController.FinalApprovePurchase)
				// Approval history endpoint (accessible by those who can view purchases)
				purchases.GET("/:id/approval-history", permMiddleware.CanView("purchases"), purchaseApprovalHandler.GetApprovalHistory)
				// Pending approvals (for those who can approve)
				purchases.GET("/pending-approval", permMiddleware.CanUsePurchaseApprovalWorkflow(), purchaseApprovalHandler.GetPurchasesForApproval)

				// Document management dengan permission checks
				purchases.POST("/:id/documents", permMiddleware.CanEdit("purchases"), purchaseController.UploadDocument)
				purchases.GET("/:id/documents", permMiddleware.CanView("purchases"), purchaseController.GetPurchaseDocuments)
				purchases.DELETE("/documents/:document_id", permMiddleware.CanDelete("purchases"), purchaseController.DeleteDocument)

				// Receipt operations dengan permission checks
				purchases.POST("/receipts", permMiddleware.CanEdit("purchases"), periodValidationMiddleware.ValidateTransactionPeriod(), purchaseController.CreatePurchaseReceipt)
				purchases.GET("/:id/receipts", permMiddleware.CanView("purchases"), purchaseController.GetPurchaseReceipts)

				// Receipt PDF exports dengan permission checks
				purchases.GET("/receipts/:receipt_id/pdf", permMiddleware.CanExport("purchases"), purchaseController.GetReceiptPDF)
				purchases.GET("/:id/receipts/pdf", permMiddleware.CanExport("purchases"), purchaseController.GetAllReceiptsPDF)

				// Analytics and reporting dengan permission checks
				purchases.GET("/summary", permMiddleware.CanView("purchases"), purchaseController.GetPurchasesSummary)
				purchases.GET("/pending-approvals", permMiddleware.CanUsePurchaseApprovalWorkflow(), purchaseController.GetPendingApprovals)

				// Export routes dengan permission checks
				purchases.GET("/export/pdf", permMiddleware.CanExport("purchases"), purchaseController.ExportPurchasesReportPDF)
				purchases.GET("/export/csv", permMiddleware.CanExport("purchases"), purchaseController.ExportPurchasesReportCSV)
				purchases.GET("/dashboard", permMiddleware.CanView("purchases"), purchaseController.GetPurchaseDashboard)
				purchases.GET("/vendor/:vendor_id/summary", permMiddleware.CanView("purchases"), purchaseController.GetVendorPurchaseSummary)

				// Payment management (similar to sales payment management)
				purchases.GET("/:id/payments", rbacMgr.RequireAnyPermission("purchases:view", "payments:view"), purchaseController.GetPurchasePayments)
				purchases.POST("/:id/payments", rbacMgr.RequireAnyPermission("purchases:edit", "payments:create"), periodValidationMiddleware.ValidateTransactionPeriod(), purchaseController.CreatePurchasePayment)
				purchases.POST("/:id/pay-pph", rbacMgr.RequireAnyPermission("purchases:edit", "payments:create"), periodValidationMiddleware.ValidateTransactionPeriod(), purchaseController.PayPurchasePPh)

				// Integrated Payment Management routes
				purchases.GET("/:id/for-payment", rbacMgr.RequireAnyPermission("purchases:view", "payments:view"), purchaseController.GetPurchaseForPayment)
				purchases.POST("/:id/integrated-payment", rbacMgr.RequireAnyPermission("purchases:edit", "payments:create"), periodValidationMiddleware.ValidateTransactionPeriod(), purchaseController.CreateIntegratedPayment)

				// Three-way matching dengan permission checks
				purchases.GET("/:id/matching", permMiddleware.CanView("purchases"), purchaseController.GetPurchaseMatching)
				purchases.POST("/:id/validate-matching", permMiddleware.CanApprove("purchases"), purchaseController.ValidateThreeWayMatching)

				// Journal entries integration dengan SSOT Journal System
				purchases.GET("/:id/journal-entries", permMiddleware.CanView("reports"), purchaseController.GetPurchaseJournalEntries)
				purchases.POST("/:id/journal-entries/backfill", rbacMgr.RequireAnyPermission("purchases:approve", "journal:create"), purchaseController.BackfillPurchaseJournalEntries)
			}

			// Expenses routes - REMOVED: No implementation yet

			// 🏢 Assets routes with enhanced permission checks dan audit logging
			assets := protected.Group("/assets")
			assets.Use(enhancedSecurity.RequestMonitoring()) // 📊 Enhanced monitoring
			{
				// Basic CRUD operations dengan enhanced security
				assets.GET("", permMiddleware.CanView("assets"), assetController.GetAssets)
				assets.GET("/:id", permMiddleware.CanView("assets"), assetController.GetAsset)
				assets.POST("", permMiddleware.CanCreate("assets"), periodValidationMiddleware.ValidateTransactionPeriod(), assetController.CreateAsset)
				assets.PUT("/:id", permMiddleware.CanEdit("assets"), periodValidationMiddleware.ValidateTransactionPeriod(), assetController.UpdateAsset)
				assets.DELETE("/:id", permMiddleware.CanDelete("assets"), assetController.DeleteAsset)
				assets.POST("/upload-image", permMiddleware.CanEdit("assets"), assetController.UploadAssetImage)

				// Manual capitalization endpoint
				assets.POST("/:id/capitalize", permMiddleware.CanEdit("assets"), periodValidationMiddleware.ValidateTransactionPeriod(), assetController.CapitalizeAsset)

				// Asset categories management
				assets.GET("/categories", permMiddleware.CanView("assets"), assetController.GetAssetCategories)
				assets.POST("/categories", permMiddleware.CanCreate("assets"), assetController.CreateAssetCategory)

				// 📊 Reports and calculations dengan permission checks
				assets.GET("/summary", permMiddleware.CanView("assets"), assetController.GetAssetsSummary)
				assets.GET("/depreciation-history", permMiddleware.CanView("assets"), assetController.GetDepreciationHistory)
				assets.GET("/depreciation-history/:period", permMiddleware.CanView("assets"), assetController.GetDepreciationHistoryDetail)
				assets.GET("/depreciation-report", permMiddleware.CanView("assets"), assetController.GetDepreciationReport)
				assets.GET("/:id/depreciation-schedule", permMiddleware.CanView("assets"), assetController.GetDepreciationSchedule)
				assets.GET("/:id/calculate-depreciation", permMiddleware.CanView("assets"), assetController.CalculateCurrentDepreciation)

				// Export routes - REMOVED: Not implemented yet

				// 📊 Monthly depreciation batch processing via SSOT
				assets.POST("/run-depreciation", permMiddleware.CanEdit("assets"), periodValidationMiddleware.ValidateTransactionPeriod(), assetController.RunDepreciation)

				// ✅ Activate asset after user completes depreciation setup
				assets.POST("/:id/activate", permMiddleware.CanEdit("assets"), periodValidationMiddleware.ValidateTransactionPeriod(), assetController.ActivateAsset)
			}

			// ⏳ Prepaid Expenses / Amortization routes
			prepaidSvc := services.NewPrepaidService(db, unifiedJournalService)
			prepaidController := controllers.NewPrepaidController(prepaidSvc)

			prepaid := protected.Group("/prepaid")
			{
				prepaid.GET("", permMiddleware.CanView("prepaid_expenses"), prepaidController.GetAllPrepaid)
				prepaid.GET("/:id", permMiddleware.CanView("prepaid_expenses"), prepaidController.GetPrepaidByID)
				prepaid.DELETE("/:id", permMiddleware.CanDelete("prepaid_expenses"), prepaidController.DeletePrepaid)
				prepaid.PUT("/:id/account", permMiddleware.CanEdit("prepaid_expenses"), prepaidController.SetExpenseAccount)
				prepaid.PUT("/:id/schedules/:schedule_id", permMiddleware.CanEdit("prepaid_expenses"), prepaidController.UpdateSchedule)
				prepaid.POST("/:id/amortize/:schedule_id", permMiddleware.CanEdit("prepaid_expenses"), periodValidationMiddleware.ValidateTransactionPeriod(), prepaidController.AmortizeSchedule)
				prepaid.POST("/:id/terminate", permMiddleware.CanEdit("prepaid_expenses"), periodValidationMiddleware.ValidateTransactionPeriod(), prepaidController.TerminatePrepaid)
			}

			// Payroll routes: HR creates detail, finance manager approves, finance/accounting see summary only.
			payroll := protected.Group("/payroll")
			{
				payroll.GET("", permMiddleware.CanView("payroll"), payrollController.GetPayrollRuns)
				payroll.GET("/import/template", permMiddleware.CanCreate("payroll"), payrollController.DownloadPayrollImportTemplate)
				payroll.POST("/import/preview", permMiddleware.CanCreate("payroll"), payrollController.ImportPayrollPreview)
				payroll.GET("/:id", permMiddleware.CanView("payroll"), payrollController.GetPayrollRun)
				payroll.POST("", permMiddleware.CanCreate("payroll"), payrollController.CreatePayrollRun)
				payroll.POST("/:id/approve", permMiddleware.CanApprove("payroll"), payrollController.ApprovePayrollRun)
				payroll.POST("/:id/reject", permMiddleware.CanApprove("payroll"), payrollController.RejectPayrollRun)
				payroll.POST("/:id/pay-salary", permMiddleware.CanApprove("payroll"), payrollController.PayPayrollSalary)
				payroll.POST("/:id/pay-bpjs", permMiddleware.CanApprove("payroll"), payrollController.PayPayrollBPJS)
				payroll.POST("/:id/pay-pph", permMiddleware.CanApprove("payroll"), payrollController.PayPayrollPPh)
			}

			// Note: CashBank routes are already set up via SetupPaymentRoutes

			// 📦 Inventory routes with enhanced permission checks
			inventory := protected.Group("/inventory")
			{
				// Basic inventory operations - accessible by those who can view products
				inventory.GET("/movements", permMiddleware.CanView("products"), inventoryController.GetInventoryMovements)
				inventory.GET("/low-stock", permMiddleware.CanView("products"), inventoryController.GetLowStockProducts)
				inventory.GET("/valuation", permMiddleware.CanView("products"), inventoryController.GetStockValuation)
				inventory.GET("/report", permMiddleware.CanExport("products"), inventoryController.GetStockReport)
				inventory.POST("/bulk-price-update", permMiddleware.CanEdit("products"), inventoryController.BulkPriceUpdate)
			}

			// Approval workflows routes
			workflows := protected.Group("/approval-workflows")
			{
				workflows.GET("", purchaseApprovalHandler.GetApprovalWorkflows)
				workflows.POST("", middleware.RoleRequired("admin"), purchaseApprovalHandler.CreateApprovalWorkflow)
			}

			// Employee approval routes
			employeeApprovals := protected.Group("/employee/approvals")
			{
				employeeApprovals.GET("/requests", employeeApprovalHandler.GetMyApprovalRequests)
				employeeApprovals.GET("/pending", employeeApprovalHandler.GetPendingApprovalsForMe)
				employeeApprovals.POST("/:id/process", employeeApprovalHandler.ProcessApproval)
				employeeApprovals.GET("/:id/history", employeeApprovalHandler.GetApprovalHistory)
				employeeApprovals.GET("/my-requests", employeeApprovalHandler.GetMySubmittedRequests)
				employeeApprovals.GET("/workflows", employeeApprovalHandler.GetApprovalWorkflowsForEmployee)
				employeeApprovals.GET("/statistics", employeeApprovalHandler.GetApprovalStatistics)
			}

			// ✅ CONSOLIDATED: Use only Enhanced Report Service as primary service with caching
			cacheService := services.NewReportCacheService()
			enhancedReportService := services.NewEnhancedReportService(db, accountRepo, salesRepo, purchaseRepo, productRepo, contactRepo, paymentRepo, cashBankRepo, cacheService)

			// 🔧 SIMPLIFIED: Use only Enhanced Report Service and Controller with PDF service
			enhancedReportController := controllers.NewEnhancedReportController(db)

			// 🏁 Fiscal Year-End Closing routes (USING UNIFIED SYSTEM)
			fiscalClosing := protected.Group("/fiscal-closing")
			fiscalClosing.Use(middleware.RoleRequired("admin", "director", "finance", "finance_manager")) // Finance can execute year-end closing
			{
				// Use period closing for preview and execute (more flexible)
				fiscalClosing.GET("/preview", periodClosingController.PreviewClosing)
				fiscalClosing.POST("/execute", periodClosingController.ExecuteClosing)
				// Use unified history controller that checks all sources
				fiscalClosing.GET("/history", unifiedClosingHistoryController.GetUnifiedClosingHistory)
			}

			// 📅 Period Closing routes (Unified Period Closing System)
			periodClosing := protected.Group("/period-closing")

			// /check-date should be accessible to all authenticated users for form validation
			periodClosing.GET("/check-date", periodClosingController.CheckDateInClosedPeriod)
			periodClosing.GET("/history", periodClosingController.GetClosingHistory)

			// The rest of the operations require admin/finance roles
			periodClosingAdmin := periodClosing.Group("")
			periodClosingAdmin.Use(middleware.RoleRequired("admin", "director", "finance", "finance_manager"))
			{
				periodClosingAdmin.GET("/last-info", periodClosingController.GetLastClosingInfo) // Get last closing info
				periodClosingAdmin.GET("/preview", periodClosingController.PreviewClosing)       // Preview period closing
				periodClosingAdmin.POST("/execute", periodClosingController.ExecuteClosing)      // Execute period closing
				periodClosingAdmin.POST("/reopen", periodClosingController.ReopenPeriod)         // Reopen closed period
			}

			// ✅ CONSOLIDATED ROUTES: Use only Enhanced Report Routes - UNDER V1
			RegisterEnhancedReportRoutes(v1, enhancedReportController, jwtManager)

			// 🚀 SSOT REPORT INTEGRATION ROUTES: Single Source of Truth integration with all financial reports
			RegisterSSOTReportRoutesInMain(v1, db, unifiedJournalService, enhancedReportService, jwtManager, rbacMgr)

			// 📋 CONTACT HISTORY REPORTS: Customer and Vendor transaction history reports
			RegisterContactHistoryRoutes(r, db, pdfService)

			// 🔧 COMPATIBILITY ROUTES: Add root-level aliases for SSOT reports
			// This provides backward compatibility for frontend requests to /ssot-reports/*
			// These routes redirect to the proper /api/v1/ssot-reports/* endpoints
			ssotAliasGroup := r.Group("/ssot-reports")
			ssotAliasGroup.Use(jwtManager.AuthRequired())
			ssotAliasGroup.Use(rbacMgr.AutoPermission("reports"))
			{
				// Initialize SSOT controllers for direct access using existing services
				ssotAliasReportIntegrationService := services.NewSSOTReportIntegrationService(
					db,
					unifiedJournalService,
					enhancedReportService,
				)
				ssotAliasReportController := controllers.NewSSOTReportIntegrationController(ssotAliasReportIntegrationService, db)

				// Route aliases that mirror the v1 endpoints
				ssotAliasGroup.GET("/trial-balance", ssotAliasReportController.GetSSOTTrialBalance)
				ssotAliasGroup.GET("/general-ledger", ssotAliasReportController.GetSSOTGeneralLedger)
				ssotAliasGroup.GET("/journal-analysis", ssotAliasReportController.GetSSOTJournalAnalysis)

				// Purchase report alias (already working, but add for consistency)
				purchaseReportController := controllers.NewSSOTPurchaseReportController(db)
				ssotAliasGroup.GET("/purchase-report", purchaseReportController.GetPurchaseReport)

				// Info endpoint explaining the alias routes
				ssotAliasGroup.GET("/info", func(c *gin.Context) {
					c.JSON(200, gin.H{
						"status":         "success",
						"message":        "SSOT Reports Compatibility Routes",
						"note":           "These are alias routes for backward compatibility",
						"recommendation": "Use /api/v1/ssot-reports/* for new implementations",
						"available_endpoints": []string{
							"/ssot-reports/trial-balance",
							"/ssot-reports/general-ledger",
							"/ssot-reports/journal-analysis",
							"/ssot-reports/purchase-report",
						},
						"proper_api_endpoints": []string{
							"/api/v1/ssot-reports/trial-balance",
							"/api/v1/ssot-reports/general-ledger",
							"/api/v1/ssot-reports/journal-analysis",
							"/api/v1/ssot-reports/purchase-report",
						},
					})
				})
			}

			// 📊 SSOT Profit & Loss Controller - Direct P&L endpoint for frontend
			ssotPLController := controllers.NewSSOTProfitLossController(db)

			// SSOT Report routes for frontend integration
			ssotReports := v1.Group("/reports")
			ssotReports.Use(jwtManager.AuthRequired())
			ssotReports.Use(rbacMgr.AutoPermission("reports"))
			{
				// Main P&L endpoint for frontend - matches the format expected by EnhancedProfitLossModal
				ssotReports.GET("/ssot-profit-loss", ssotPLController.GetSSOTProfitLoss)
			}

			// 💰 COGS Management Routes - Cost of Goods Sold tracking and backfill
			cogsController := controllers.NewCOGSController(db)
			cogsRoutes := v1.Group("/cogs")
			cogsRoutes.Use(jwtManager.AuthRequired())
			cogsRoutes.Use(middleware.RoleRequired("finance", "finance_manager", "admin", "director"))
			{
				cogsRoutes.GET("/summary", cogsController.GetCOGSSummary)      // Get COGS summary for period
				cogsRoutes.GET("/missing", cogsController.GetSalesWithoutCOGS) // Get sales without COGS entries
				if accountingMaintenanceEnabled {
					// 🔄 Migration: Backfill SSOT journals for existing purchase payments
					cogsRoutes.POST("/migrate-payment-journals", func(c *gin.Context) {
						log.Printf("🔄 Starting purchase payment SSOT journal migration...")
						migrationSvc := services.NewPurchasePaymentJournalService(db, accountRepo, unifiedJournalService, nil)
						migrated, skipped, err := migrationSvc.MigrateExistingPaymentsToSSOT()
						if err != nil {
							c.JSON(500, gin.H{
								"error":    "Migration failed: " + err.Error(),
								"migrated": migrated,
								"skipped":  skipped,
							})
							return
						}
						c.JSON(200, gin.H{
							"message":  "Purchase payment SSOT journal migration completed",
							"migrated": migrated,
							"skipped":  skipped,
						})
					})
				}
			}

			// 📊 SSOT Balance Sheet Controller - Direct Balance Sheet endpoint for frontend
			ssotBSController := controllers.NewSSOTBalanceSheetController(db)

			// 💰 SSOT Cash Flow Controller - Direct Cash Flow endpoint for frontend
			ssotCFController := controllers.NewSSOTCashFlowController(db)

			// Balance Sheet Report routes for frontend integration
			ssotBSReports := v1.Group("/reports/ssot")
			ssotBSReports.Use(jwtManager.AuthRequired())
			ssotBSReports.Use(rbacMgr.AutoPermission("reports"))
			{
				// Main Balance Sheet endpoint for frontend - matches the format expected by SSOT Balance Sheet Modal
				ssotBSReports.GET("/balance-sheet", ssotBSController.GenerateSSOTBalanceSheet)
				ssotBSReports.GET("/balance-sheet/account-details", ssotBSController.GetSSOTBalanceSheetAccountDetails)
				ssotBSReports.GET("/balance-sheet/validate", ssotBSController.ValidateSSOTBalanceSheet)
				ssotBSReports.GET("/balance-sheet/comparison", ssotBSController.GetSSOTBalanceSheetComparison)

				// 💰 Cash Flow Statement endpoints for frontend - matches the format expected by SSOT Cash Flow Modal
				ssotBSReports.GET("/cash-flow", ssotCFController.GetSSOTCashFlow)
				ssotBSReports.GET("/cash-flow/summary", ssotCFController.GetSSOTCashFlowSummary)
				ssotBSReports.GET("/cash-flow/validate", ssotCFController.ValidateSSOTCashFlow)
			}

			// Monitoring routes (admin only)
			monitoring := protected.Group("/monitoring")
			monitoring.Use(middleware.RoleRequired("admin")) // Only admins can access monitoring
			{
				// System monitoring
				monitoring.GET("/status", monitoringController.GetSystemSecurityStatus)
				monitoring.GET("/rate-limits", monitoringController.GetRateLimitStatus)
				monitoring.GET("/security-alerts", monitoringController.GetSecurityAlerts)

				// Audit logging
				monitoring.GET("/audit-logs", monitoringController.GetAuditLogs)

				// Token monitoring
				monitoring.GET("/token-stats", monitoringController.GetTokenStats)
				monitoring.GET("/refresh-events", monitoringController.GetRecentRefreshEvents)

				// Session management
				monitoring.GET("/sessions/stats", sessionController.GetSessionStats)
				monitoring.POST("/sessions/cleanup", sessionController.ForceCleanup)
				monitoring.GET("/sessions/active", jwtManager.AuthRequired(), sessionController.GetActiveSessions)

				// User-specific monitoring
				monitoring.GET("/users/:user_id/security-summary", monitoringController.GetUserSecuritySummary)

				// Startup service monitoring
				monitoring.GET("/startup-status", startupHandler.GetStartupStatus)
				if accountingMaintenanceEnabled {
					monitoring.POST("/fix-account-headers", startupHandler.TriggerAccountHeaderFix)
				}

				// Balance monitoring routes
				monitoring.GET("/balance-sync", balanceMonitoringController.CheckBalanceSync)
				monitoring.GET("/balance-health", balanceMonitoringController.GetBalanceHealth)
				monitoring.GET("/discrepancies", balanceMonitoringController.GetBalanceDiscrepancies)
				monitoring.GET("/sync-status", balanceMonitoringController.GetSyncStatus)

				// 📊 API Usage monitoring routes
				monitoring.GET("/api-usage/stats", apiUsageController.GetAPIUsageStats)
				monitoring.GET("/api-usage/top", apiUsageController.GetTopEndpoints)
				monitoring.GET("/api-usage/unused", apiUsageController.GetUnusedEndpoints)
				monitoring.GET("/api-usage/analytics", apiUsageController.GetUsageAnalytics)
				monitoring.POST("/api-usage/reset", apiUsageController.ResetUsageStats)

				// 🏁 Performance monitoring routes (admin only)
				monitoring.GET("/performance/report", performanceController.GetPerformanceReport)
				monitoring.GET("/performance/metrics", performanceController.GetQuickMetrics)
				monitoring.GET("/performance/bottlenecks", performanceController.GetBottlenecks)
				monitoring.GET("/performance/recommendations", performanceController.GetRecommendations)
				monitoring.GET("/performance/system", performanceController.GetSystemStatus)
				monitoring.POST("/performance/metrics/clear", performanceController.ClearMetrics)
				monitoring.GET("/performance/test", performanceController.TestUltraFastEndpoint)

				// 🚑 Payment timeout diagnostic routes (critical for troubleshooting)
				monitoring.GET("/timeout/diagnostics", performanceController.RunTimeoutDiagnostics)
				monitoring.GET("/timeout/health", performanceController.GetQuickHealthCheck)
			}

			// 🔒 Security Dashboard routes (admin only)
			security := protected.Group("/admin/security")
			security.Use(middleware.RoleRequired("admin"))     // Only admins can access security dashboard
			security.Use(enhancedSecurity.RequestMonitoring()) // Enhanced monitoring for security routes
			{
				// Security Incident Management
				security.GET("/incidents", securityController.GetSecurityIncidents)
				security.GET("/incidents/:id", securityController.GetSecurityIncident)
				security.PUT("/incidents/:id/resolve", securityController.ResolveSecurityIncident)

				// System Alerts Management
				security.GET("/alerts", securityController.GetSystemAlerts)
				security.PUT("/alerts/:id/acknowledge", securityController.AcknowledgeAlert)

				// Security Metrics & Analytics
				security.GET("/metrics", securityController.GetSecurityMetrics)

				// IP Whitelist Management
				security.GET("/ip-whitelist", securityController.GetIPWhitelist)
				security.POST("/ip-whitelist", securityController.AddIPToWhitelist)

				// Security Configuration
				security.GET("/config", securityController.GetSecurityConfig)

				// Maintenance Operations
				security.POST("/cleanup", securityController.CleanupSecurityLogs)

			}

			// 🔧 Admin CashBank GL Links routes (admin only)
			adminRoutes := protected.Group("/admin")
			adminRoutes.Use(middleware.RoleRequired("admin"))     // Only admins can access admin routes
			adminRoutes.Use(enhancedSecurity.RequestMonitoring()) // Enhanced monitoring for admin routes
			{
				// CashBank GL account links management
				adminRoutes.GET("/check-cashbank-gl-links", fixCashBankController.CheckCashBankGLLinks)

				// 🔧 Database Migration Management (Admin only)
				adminMigrationController := controllers.NewAdminMigrationController(db)
				adminRoutes.GET("/migrations/constraint-info", adminMigrationController.GetConstraintInfo)
				if accountingMaintenanceEnabled {
					adminRoutes.POST("/migrations/fix-date-constraint", adminMigrationController.ForceRunDateConstraintFix)
				}

				// 📝 Activity Logs Management (Admin only)
				adminRoutes.GET("/activity-logs", activityLogController.GetActivityLogs)
				adminRoutes.GET("/activity-logs/summary", activityLogController.GetActivitySummary)
				adminRoutes.GET("/activity-logs/stats", activityLogController.GetActivityStats)
				adminRoutes.POST("/activity-logs/cleanup", activityLogController.CleanupOldLogs)
			}

			// 📝 Activity Logs - User self-access routes (any authenticated user)
			activityLogs := protected.Group("/activity-logs")
			{
				// Users can view their own activity logs
				activityLogs.GET("/me", activityLogController.GetMyActivityLogs)
			}

			// 🏥 Balance Health routes (admin only)
			balanceHealth := protected.Group("/admin/balance-health")
			balanceHealth.Use(middleware.RoleRequired("admin"))     // Only admins can access balance health
			balanceHealth.Use(enhancedSecurity.RequestMonitoring()) // Enhanced monitoring for balance health routes
			{
				// Balance Health Management
				balanceHealth.GET("/check", balanceHealthController.HealthCheck)
				balanceHealth.GET("/detailed-report", balanceHealthController.DetailedReport)
			}
		}
	}

	// Static files (templates and uploads)
	r.Static("/templates", "./templates")
	r.Static("/uploads", "./uploads")

	// Global favicon handler to avoid 404s in all contexts (Swagger and non-Swagger)
	r.GET("/favicon.ico", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	// Health check endpoint
	v1.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Swagger documentation endpoint (accessible in development or when ENABLE_SWAGGER=true)
	if isDevelopmentMode() || os.Getenv("ENABLE_SWAGGER") == "true" {
		// Load the Swagger spec once and serve from memory to avoid path/cwd issues
		swaggerBytes, pathUsed, err := loadOpenAPIJSON()
		if err != nil {
			log.Printf("⚠️  Swagger spec not found: %v", err)
			log.Printf("ℹ️  Serving minimal embedded spec so Swagger UI can still load. Run 'swag init' to generate full docs.")
			swaggerBytes = minimalOpenAPIJSON()
			pathUsed = "embedded-minimal"
		}
		log.Printf("📄 Serving Swagger spec from: %s", pathUsed)
		r.GET("/openapi.json", func(c *gin.Context) {
			c.Data(http.StatusOK, "application/json", swaggerBytes)
		})
		r.GET("/openapi/doc.json", func(c *gin.Context) {
			c.Data(http.StatusOK, "application/json", swaggerBytes)
		})
		// Swagger UI endpoints using Enhanced Swagger content on legacy URLs
		sg := r.Group("/")
		sg.Use(swaggerCSPMiddleware()) // relax CSP for Swagger UI only
		sg.GET("/swagger", func(c *gin.Context) { c.Redirect(http.StatusFound, "/swagger/index.html") })
		sg.GET("/swagger/index.html", func(c *gin.Context) {
			// Use Enhanced Swagger HTML instead of basic swaggerIndexHTML
			html := config.GetEnhancedSwaggerHTML("/openapi.json")
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
		})
		sg.GET("/docs", func(c *gin.Context) { c.Redirect(http.StatusFound, "/docs/index.html") })
		sg.GET("/docs/index.html", func(c *gin.Context) {
			// Use Enhanced Swagger HTML instead of basic swaggerIndexHTML
			html := config.GetEnhancedSwaggerHTML("/openapi.json")
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
		})
		// Avoid favicon 404s on Swagger pages (kept for completeness)
	}

	// Debug routes for development only
	if gin.Mode() == gin.DebugMode {
		debug := v1.Group("/debug")
		{
			// Minimal debug routes for development
			debugWithAuth := debug.Group("/auth")
			debugWithAuth.Use(jwtManager.AuthRequired())
			{
				debugWithAuth.GET("/context", debugController.TestJWTContext)
				debugWithAuth.GET("/role", debugController.TestRolePermission)

				// Essential permission tests only
				debugWithAuth.GET("/test-cashbank-permission", permMiddleware.CanView("cash_bank"), debugController.TestCashBankPermission)
				debugWithAuth.GET("/test-payments-permission", permMiddleware.CanView("payments"), debugController.TestPaymentsPermission)
			}
		}
	}
}

// setupFrontendCompatibilityRoutes adds root-level routes for frontend compatibility
// These routes proxy requests to the actual /api/v1 endpoints
func setupFrontendCompatibilityRoutes(r *gin.Engine, v1 *gin.RouterGroup) {
	log.Println("🔗 Setting up frontend compatibility routes...")

	// Root-level routes that proxy to /api/v1 endpoints
	// These maintain the same authentication and permission requirements

	// Accounts compatibility routes
	r.GET("/accounts", func(c *gin.Context) {
		// Forward to the actual /api/v1/accounts endpoint
		c.Request.URL.Path = "/api/v1/accounts"
		r.HandleContext(c)
	})

	r.POST("/accounts", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/accounts"
		r.HandleContext(c)
	})

	r.GET("/accounts/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/accounts/" + c.Param("id")
		r.HandleContext(c)
	})

	r.PUT("/accounts/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/accounts/" + c.Param("id")
		r.HandleContext(c)
	})

	r.DELETE("/accounts/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/accounts/" + c.Param("id")
		r.HandleContext(c)
	})

	// Contacts compatibility routes
	r.GET("/contacts", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/contacts"
		r.HandleContext(c)
	})

	r.POST("/contacts", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/contacts"
		r.HandleContext(c)
	})

	r.GET("/contacts/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/contacts/" + c.Param("id")
		r.HandleContext(c)
	})

	r.PUT("/contacts/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/contacts/" + c.Param("id")
		r.HandleContext(c)
	})

	r.DELETE("/contacts/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/contacts/" + c.Param("id")
		r.HandleContext(c)
	})

	// Products compatibility routes
	r.GET("/products", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/products"
		r.HandleContext(c)
	})

	r.POST("/products", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/products"
		r.HandleContext(c)
	})

	r.GET("/products/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/products/" + c.Param("id")
		r.HandleContext(c)
	})

	r.PUT("/products/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/products/" + c.Param("id")
		r.HandleContext(c)
	})

	r.DELETE("/products/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/products/" + c.Param("id")
		r.HandleContext(c)
	})

	// Users compatibility routes
	r.GET("/users", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/users"
		r.HandleContext(c)
	})

	r.POST("/users", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/users"
		r.HandleContext(c)
	})

	r.GET("/users/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/users/" + c.Param("id")
		r.HandleContext(c)
	})

	r.PUT("/users/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/users/" + c.Param("id")
		r.HandleContext(c)
	})

	r.DELETE("/users/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/users/" + c.Param("id")
		r.HandleContext(c)
	})

	// Sales compatibility routes
	r.GET("/sales", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/sales"
		r.HandleContext(c)
	})

	r.POST("/sales", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/sales"
		r.HandleContext(c)
	})

	r.GET("/sales/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/sales/" + c.Param("id")
		r.HandleContext(c)
	})

	r.PUT("/sales/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/sales/" + c.Param("id")
		r.HandleContext(c)
	})

	r.DELETE("/sales/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/sales/" + c.Param("id")
		r.HandleContext(c)
	})

	r.POST("/sales/:id/approve", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/sales/" + c.Param("id") + "/approve"
		r.HandleContext(c)
	})

	r.POST("/sales/:id/invoice", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/sales/" + c.Param("id") + "/invoice"
		r.HandleContext(c)
	})

	r.DELETE("/sales/:id/cancel", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/sales/" + c.Param("id") + "/cancel"
		r.HandleContext(c)
	})

	// Purchase compatibility routes
	r.GET("/purchases", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/purchases"
		r.HandleContext(c)
	})

	r.POST("/purchases", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/purchases"
		r.HandleContext(c)
	})

	r.GET("/purchases/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/purchases/" + c.Param("id")
		r.HandleContext(c)
	})

	r.PUT("/purchases/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/purchases/" + c.Param("id")
		r.HandleContext(c)
	})

	r.DELETE("/purchases/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/purchases/" + c.Param("id")
		r.HandleContext(c)
	})

	r.POST("/purchases/:id/approve", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/purchases/" + c.Param("id") + "/approve"
		r.HandleContext(c)
	})

	r.POST("/purchases/:id/receive", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/purchases/" + c.Param("id") + "/receive"
		r.HandleContext(c)
	})

	r.DELETE("/purchases/:id/cancel", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/purchases/" + c.Param("id") + "/cancel"
		r.HandleContext(c)
	})

	// Payments compatibility routes
	r.GET("/payments", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/payments"
		r.HandleContext(c)
	})

	r.POST("/payments", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/payments"
		r.HandleContext(c)
	})

	r.GET("/payments/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/payments/" + c.Param("id")
		r.HandleContext(c)
	})

	r.PUT("/payments/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/payments/" + c.Param("id")
		r.HandleContext(c)
	})

	r.DELETE("/payments/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/payments/" + c.Param("id")
		r.HandleContext(c)
	})

	r.POST("/payments/:id/approve", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/payments/" + c.Param("id") + "/approve"
		r.HandleContext(c)
	})

	r.DELETE("/payments/:id/cancel", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/payments/" + c.Param("id") + "/cancel"
		r.HandleContext(c)
	})

	// Cash & Bank compatibility routes
	r.GET("/cash-bank", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/cash-bank"
		r.HandleContext(c)
	})

	r.POST("/cash-bank", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/cash-bank"
		r.HandleContext(c)
	})

	r.GET("/cash-bank/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/cash-bank/" + c.Param("id")
		r.HandleContext(c)
	})

	r.PUT("/cash-bank/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/cash-bank/" + c.Param("id")
		r.HandleContext(c)
	})

	r.DELETE("/cash-bank/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/cash-bank/" + c.Param("id")
		r.HandleContext(c)
	})

	r.POST("/cash-bank/:id/confirm", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/cash-bank/" + c.Param("id") + "/confirm"
		r.HandleContext(c)
	})

	r.DELETE("/cash-bank/:id/cancel", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/cash-bank/" + c.Param("id") + "/cancel"
		r.HandleContext(c)
	})

	// Journals compatibility routes
	r.GET("/journals", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/journals"
		r.HandleContext(c)
	})

	r.POST("/journals", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/journals"
		r.HandleContext(c)
	})

	r.GET("/journals/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/journals/" + c.Param("id")
		r.HandleContext(c)
	})

	r.PUT("/journals/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/journals/" + c.Param("id")
		r.HandleContext(c)
	})

	r.DELETE("/journals/:id", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/journals/" + c.Param("id")
		r.HandleContext(c)
	})

	r.POST("/journals/:id/post", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/journals/" + c.Param("id") + "/post"
		r.HandleContext(c)
	})

	r.DELETE("/journals/:id/cancel", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/journals/" + c.Param("id") + "/cancel"
		r.HandleContext(c)
	})

	// Reports compatibility routes
	r.GET("/reports/balance-sheet", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/reports/balance-sheet"
		r.HandleContext(c)
	})

	r.GET("/reports/income-statement", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/reports/income-statement"
		r.HandleContext(c)
	})

	r.GET("/reports/cash-flow", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/reports/cash-flow"
		r.HandleContext(c)
	})

	r.GET("/reports/trial-balance", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/reports/trial-balance"
		r.HandleContext(c)
	})

	r.GET("/reports/general-ledger", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/reports/general-ledger"
		r.HandleContext(c)
	})

	r.GET("/reports/accounts-receivable", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/reports/accounts-receivable"
		r.HandleContext(c)
	})

	r.GET("/reports/accounts-payable", func(c *gin.Context) {
		c.Request.URL.Path = "/api/v1/reports/accounts-payable"
		r.HandleContext(c)
	})

	log.Println("✅ Frontend compatibility routes configured successfully")
}
