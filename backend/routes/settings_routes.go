package routes

import (
	"github.com/gin-gonic/gin"
	"app-sistem-akuntansi/controllers"
	"app-sistem-akuntansi/middleware"
	"app-sistem-akuntansi/services"
	"gorm.io/gorm"
)

// SetupSettingsRoutes registers all settings-related routes
func SetupSettingsRoutes(protected *gin.RouterGroup, db *gorm.DB) {
	// Initialize settings service and controller
	settingsService := services.NewSettingsService(db)
	settingsController := controllers.NewSettingsController(settingsService)
	
	// Initialize tax account settings service and controller
	taxAccountService := services.NewTaxAccountService(db)
	coaService := services.NewCOAService(db)
	taxAccountController := controllers.NewTaxAccountController(taxAccountService, coaService)
	
	// Initialize enhanced tax account status controller
	taxAccountStatusController := controllers.NewTaxAccountStatusController(taxAccountService, coaService)
	
	// Settings routes (admin only)
	settings := protected.Group("/settings")
	settings.Use(middleware.RoleRequired("admin"))
	{
		// Main settings endpoints
		settings.GET("", settingsController.GetSettings)
		settings.PUT("", settingsController.UpdateSettings)
		
		// Specific settings endpoints
		settings.PUT("/company", settingsController.UpdateCompanyInfo)
		settings.PUT("/system", settingsController.UpdateSystemConfig)
		settings.POST("/company/logo", settingsController.UploadCompanyLogo)
		
		// Additional endpoints
		settings.POST("/reset", settingsController.ResetToDefaults)
		settings.GET("/validation-rules", settingsController.GetValidationRules)
		settings.GET("/history", settingsController.GetSettingsHistory)
		
		// Initialize COA (One-time setup)
		settings.POST("/initialize-coa", settingsController.InitializeCOA)
	}
	
	// Tax Account Settings routes (admin and finance)
	taxSettings := protected.Group("/tax-accounts")
	taxSettings.Use(middleware.RoleRequired("admin", "finance", "director"))
	{
		// Get current active tax account settings
		taxSettings.GET("/current", taxAccountController.GetCurrentSettings)
		
		// Get all tax account settings (admin only)
		taxSettings.GET("", middleware.RoleRequired("admin"), taxAccountController.GetAllSettings)
		
		// Create new tax account settings
		taxSettings.POST("", taxAccountController.CreateSettings)
		
		// Update existing tax account settings
		taxSettings.PUT("/:id", taxAccountController.UpdateSettings)
		
		// Activate specific tax account settings
		taxSettings.POST("/:id/activate", taxAccountController.ActivateSettings)
		
		// Get available accounts for dropdowns
		taxSettings.GET("/accounts", taxAccountController.GetAvailableAccounts)
		
		// Validate account configuration
		taxSettings.POST("/validate", taxAccountController.ValidateAccountConfiguration)
		
		// Refresh cache
		taxSettings.POST("/refresh-cache", middleware.RoleRequired("admin"), taxAccountController.RefreshCache)
		
		// Get account suggestions for setup wizard
		taxSettings.GET("/suggestions", taxAccountController.GetAccountSuggestions)
		
		// Enhanced status and validation endpoints
		taxSettings.GET("/status", taxAccountStatusController.GetStatus)
		taxSettings.GET("/validate", taxAccountStatusController.ValidateAccountSelection)
	}
}
