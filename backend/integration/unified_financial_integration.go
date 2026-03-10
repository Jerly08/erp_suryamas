package integration

import (
	"fmt"
	"log"

	"app-sistem-akuntansi/controllers"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/routes"
	"app-sistem-akuntansi/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UnifiedFinancialIntegration handles the complete integration of unified financial reporting system
type UnifiedFinancialIntegration struct {
	DB                    *gorm.DB
	Router                *gin.Engine
	UnifiedService        *services.UnifiedFinancialReportService
	UnifiedController     *controllers.UnifiedFinancialReportController
	AccountRepo           *repositories.AccountRepository
	JournalEntryRepo      *repositories.JournalEntryRepository
	SaleRepo              *repositories.SaleRepository
	PurchaseRepo          *repositories.PurchaseRepository
	CashBankRepo          *repositories.CashBankRepository
	PaymentRepo           *repositories.PaymentRepository
}

// NewUnifiedFinancialIntegration creates a new unified financial integration instance
func NewUnifiedFinancialIntegration(db *gorm.DB, router *gin.Engine) *UnifiedFinancialIntegration {
	// Initialize repositories
	accountRepo := repositories.NewAccountRepository(db)
	journalEntryRepo := repositories.NewJournalEntryRepository(db)
	saleRepo := repositories.NewSaleRepository(db)
	purchaseRepo := repositories.NewPurchaseRepository(db)
	cashBankRepo := repositories.NewCashBankRepository(db)
	paymentRepo := repositories.NewPaymentRepository(db)

	// Initialize unified service
	unifiedService := services.NewUnifiedFinancialReportService(
		accountRepo,
		journalEntryRepo,
		saleRepo,
		purchaseRepo,
		cashBankRepo,
		paymentRepo,
		db,
	)

	// Initialize unified controller
	unifiedController := controllers.NewUnifiedFinancialReportController(unifiedService)

	return &UnifiedFinancialIntegration{
		DB:                db,
		Router:            router,
		UnifiedService:    unifiedService,
		UnifiedController: unifiedController,
		AccountRepo:       accountRepo,
		JournalEntryRepo:  journalEntryRepo,
		SaleRepo:          saleRepo,
		PurchaseRepo:      purchaseRepo,
		CashBankRepo:      cashBankRepo,
		PaymentRepo:       paymentRepo,
	}
}

// Initialize sets up the complete unified financial reporting system
func (ufi *UnifiedFinancialIntegration) Initialize() error {
	log.Println("Initializing Unified Financial Reporting System...")

	// Setup middleware
	ufi.setupMiddleware()

	// Setup routes
	ufi.setupRoutes()

	// Validate integration
	if err := ufi.validateIntegration(); err != nil {
		log.Printf("Integration validation failed: %v", err)
		return err
	}

	log.Println("Unified Financial Reporting System initialized successfully")
	return nil
}

// setupMiddleware configures middleware for the unified reporting system
func (ufi *UnifiedFinancialIntegration) setupMiddleware() {
	// Add report-specific middleware
	reportMiddleware := routes.SetupReportMiddleware()
	ufi.Router.Use(reportMiddleware)

	// Add logging middleware for financial reports
	ufi.Router.Use(ufi.loggingMiddleware())

	// Add error handling middleware
	ufi.Router.Use(ufi.errorHandlingMiddleware())
}

// setupRoutes configures all routes for the unified reporting system
func (ufi *UnifiedFinancialIntegration) setupRoutes() {
	routes.SetupUnifiedReportRoutes(ufi.Router, ufi.DB)

	// Add documentation endpoint
	ufi.Router.GET("/api/unified-reports/docs", func(c *gin.Context) {
		doc := routes.GetUnifiedReportRouteDocumentation()
		c.JSON(200, gin.H{
			"status": "success",
			"data":   doc,
		})
	})

	// Add health check endpoint
	ufi.Router.GET("/api/unified-reports/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "unified-financial-reporting",
			"version": "1.0.0",
		})
	})
}

// validateIntegration performs validation checks on the integrated system
func (ufi *UnifiedFinancialIntegration) validateIntegration() error {
	// Check database connection
	if ufi.DB == nil {
		return fmt.Errorf("database connection is nil")
	}

	// Test database connection
	sqlDB, err := ufi.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %v", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %v", err)
	}

	// Validate repositories
	if ufi.AccountRepo == nil {
		return fmt.Errorf("account repository is nil")
	}
	if ufi.JournalEntryRepo == nil {
		return fmt.Errorf("journal entry repository is nil")
	}
	if ufi.SaleRepo == nil {
		return fmt.Errorf("sale repository is nil")
	}
	if ufi.PurchaseRepo == nil {
		return fmt.Errorf("purchase repository is nil")
	}
	if ufi.CashBankRepo == nil {
		return fmt.Errorf("cash bank repository is nil")
	}
	if ufi.PaymentRepo == nil {
		return fmt.Errorf("payment repository is nil")
	}

	// Validate service
	if ufi.UnifiedService == nil {
		return fmt.Errorf("unified service is nil")
	}

	// Validate controller
	if ufi.UnifiedController == nil {
		return fmt.Errorf("unified controller is nil")
	}

	log.Println("Integration validation completed successfully")
	return nil
}

// loggingMiddleware creates logging middleware for financial reports
func (ufi *UnifiedFinancialIntegration) loggingMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
			param.ClientIP,
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	})
}

// errorHandlingMiddleware creates error handling middleware for financial reports
func (ufi *UnifiedFinancialIntegration) errorHandlingMiddleware() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		if err, ok := recovered.(string); ok {
			log.Printf("Financial reporting error: %s", err)
			c.AbortWithStatusJSON(500, gin.H{
				"status":  "error",
				"message": "Internal server error in financial reporting",
				"details": err,
			})
			return
		}
		c.AbortWithStatusJSON(500, gin.H{
			"status":  "error",
			"message": "Internal server error in financial reporting",
		})
	})
}

// GetServiceStatus returns the status of all integrated services
func (ufi *UnifiedFinancialIntegration) GetServiceStatus() map[string]interface{} {
	status := make(map[string]interface{})

	// Database status
	if ufi.DB != nil {
		if sqlDB, err := ufi.DB.DB(); err == nil {
			if err := sqlDB.Ping(); err == nil {
				status["database"] = "healthy"
			} else {
				status["database"] = "unhealthy"
			}
		} else {
			status["database"] = "error"
		}
	} else {
		status["database"] = "not_configured"
	}

	// Repository status
	status["repositories"] = map[string]interface{}{
		"account":       ufi.AccountRepo != nil,
		"journal_entry": ufi.JournalEntryRepo != nil,
		"sale":          ufi.SaleRepo != nil,
		"purchase":      ufi.PurchaseRepo != nil,
		"cash_bank":     ufi.CashBankRepo != nil,
		"payment":       ufi.PaymentRepo != nil,
	}

	// Service status
	status["unified_service"] = ufi.UnifiedService != nil

	// Controller status
	status["unified_controller"] = ufi.UnifiedController != nil

	return status
}

// PerformHealthCheck performs a comprehensive health check
func (ufi *UnifiedFinancialIntegration) PerformHealthCheck() (bool, map[string]interface{}) {
	healthDetails := make(map[string]interface{})
	allHealthy := true

	// Check database
	dbHealthy := false
	if ufi.DB != nil {
		if sqlDB, err := ufi.DB.DB(); err == nil {
			if err := sqlDB.Ping(); err == nil {
				dbHealthy = true
			}
		}
	}
	healthDetails["database"] = dbHealthy
	if !dbHealthy {
		allHealthy = false
	}

	// Check repositories
	repoHealthy := ufi.AccountRepo != nil && 
		ufi.JournalEntryRepo != nil && 
		ufi.SaleRepo != nil && 
		ufi.PurchaseRepo != nil && 
		ufi.CashBankRepo != nil && 
		ufi.PaymentRepo != nil
	healthDetails["repositories"] = repoHealthy
	if !repoHealthy {
		allHealthy = false
	}

	// Check service
	serviceHealthy := ufi.UnifiedService != nil
	healthDetails["service"] = serviceHealthy
	if !serviceHealthy {
		allHealthy = false
	}

	// Check controller
	controllerHealthy := ufi.UnifiedController != nil
	healthDetails["controller"] = controllerHealthy
	if !controllerHealthy {
		allHealthy = false
	}

	return allHealthy, healthDetails
}

// GetIntegrationSummary returns a summary of the integrated system
func (ufi *UnifiedFinancialIntegration) GetIntegrationSummary() map[string]interface{} {
	return map[string]interface{}{
		"system_name":    "Unified Financial Reporting System",
		"version":        "1.0.0",
		"description":    "Comprehensive financial reporting with unified service architecture",
		"components": map[string]interface{}{
			"repositories": []string{
				"AccountRepository",
				"JournalEntryRepository", 
				"SaleRepository",
				"PurchaseRepository",
				"CashBankRepository",
				"PaymentRepository",
			},
			"services": []string{
				"UnifiedFinancialReportService",
			},
			"controllers": []string{
				"UnifiedFinancialReportController",
			},
			"endpoints": []string{
				"/api/unified-reports/profit-loss",
				"/api/unified-reports/balance-sheet",
				"/api/unified-reports/cash-flow",
				"/api/unified-reports/trial-balance",
				"/api/unified-reports/general-ledger/:account_id",
				"/api/unified-reports/sales-summary",
				"/api/unified-reports/vendor-analysis",
				"/api/unified-reports/dashboard",
				"/api/unified-reports/available",
				"/api/unified-reports/all",
				"/api/unified-reports/validate",
			},
		},
		"features": []string{
			"Comprehensive Financial Statements",
			"Real-time Dashboard",
			"Comparative Analysis",
			"Batch Report Generation",
			"Parameter Validation",
			"Legacy Compatibility",
			"Financial Ratios Calculation",
			"Multi-period Analysis",
		},
		"status": ufi.GetServiceStatus(),
	}
}

// Shutdown gracefully shuts down the unified financial reporting system
func (ufi *UnifiedFinancialIntegration) Shutdown() error {
	log.Println("Shutting down Unified Financial Reporting System...")

	// Close database connections if needed
	if ufi.DB != nil {
		if sqlDB, err := ufi.DB.DB(); err == nil {
			if err := sqlDB.Close(); err != nil {
				log.Printf("Error closing database: %v", err)
				return err
			}
		}
	}

	log.Println("Unified Financial Reporting System shutdown completed")
	return nil
}

// SetupUnifiedFinancialReporting is a convenience function for quick setup
func SetupUnifiedFinancialReporting(db *gorm.DB, router *gin.Engine) (*UnifiedFinancialIntegration, error) {
	// Create integration instance
	integration := NewUnifiedFinancialIntegration(db, router)

	// Initialize the system
	if err := integration.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize unified financial reporting: %v", err)
	}

	return integration, nil
}

// Example usage function demonstrating how to integrate everything
func ExampleIntegration() {
	// This is an example of how to use the unified financial integration
	
	// Assume you have a database and router instance
	// db := setupDatabase() 
	// router := gin.Default()
	
	// Quick setup
	// integration, err := SetupUnifiedFinancialReporting(db, router)
	// if err != nil {
	//     log.Fatalf("Failed to setup unified financial reporting: %v", err)
	// }
	
	// Get system summary
	// summary := integration.GetIntegrationSummary()
	// log.Printf("System Summary: %+v", summary)
	
	// Perform health check
	// healthy, details := integration.PerformHealthCheck()
	// log.Printf("System Health: %t, Details: %+v", healthy, details)
	
	// Start server
	// router.Run(":8080")
	
	// Graceful shutdown
	// defer integration.Shutdown()
}

// Manual setup function for custom configurations
func ManualSetup(db *gorm.DB, router *gin.Engine) (*UnifiedFinancialIntegration, error) {
	// Create integration instance
	integration := NewUnifiedFinancialIntegration(db, router)

	// Custom middleware setup
	integration.setupMiddleware()

	// Custom routes setup
	integration.setupRoutes()

	// Add custom routes or middleware here if needed
	// integration.Router.Use(customMiddleware())
	// integration.Router.GET("/custom-endpoint", customHandler)

	// Validate integration
	if err := integration.validateIntegration(); err != nil {
		return nil, err
	}

	return integration, nil
}
