package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/middleware"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/routes"
	"app-sistem-akuntansi/services"
	"app-sistem-akuntansi/startup"
	"app-sistem-akuntansi/utils"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// @title Sistema Akuntansi API
// @version 1.0
// @description API untuk aplikasi sistem akuntansi yang komprehensif dengan fitur lengkap manajemen keuangan, inventory, sales, purchases, dan reporting.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api/v1
// @schemes http https

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	log.SetOutput(utils.NewSanitizingWriter(os.Stdout))

	// Load configuration
	cfg := config.LoadConfig()

	// Initialize global rate limiters before routes are registered.
	middleware.InitializeRateLimiters(cfg)

	// Set Gin mode to release to hide debug logs
	gin.SetMode(gin.ReleaseMode)

	// Connect to database
	db := database.ConnectDB()

	// Auto migrate models
	database.AutoMigrate(db)
	database.PurchaseRequestInvoiceDueDateMigration(db)

	// Run auto SQL migrations (including Bank Mandiri fix)
	if err := database.RunAutoMigrations(db); err != nil {
		log.Printf("Warning: Auto-migration failed: %v", err)
	}
	if err := database.EnsureStockAlertNotificationUniqueness(db); err != nil {
		log.Printf("Warning: Stock alert uniqueness setup failed: %v", err)
	}

	// Check and verify SSOT Journal Migration
	log.Println("⚡ Starting SSOT Journal System verification...")
	if err := startup.CheckAndRunSSOTMigration(db); err != nil {
		log.Printf("❌ SSOT Migration verification failed: %v", err)
		log.Printf("⚠️  Backend will continue to run, but SSOT Journal features may not work properly")
		log.Printf("💡 Please check the database migration status manually")
	} else {
		log.Println("✅ SSOT Journal System is ready and verified!")
	}

	// Migrate to Unified Journals (if old journal_entries exist)
	log.Println("⚡ Checking migration to Unified Journals...")
	if err := startup.MigrateToUnifiedJournals(db); err != nil {
		log.Printf("❌ Migration to Unified Journals failed: %v", err)
		log.Printf("⚠️  Backend will continue to run, but data may be inconsistent")
		log.Printf("💡 Please check the migration logs and database state")
	} else {
		log.Println("✅ Unified Journals migration check completed!")
	}

	// Backfilling historical payment journals is a maintenance operation, not a
	// startup task. It can only run during an explicitly approved maintenance window.
	if os.Getenv("RUN_PAYMENT_SSOT_BACKFILL") == "true" {
		log.Println("⚡ Running approved purchase payment SSOT journal backfill...")
		accountRepo := repositories.NewAccountRepository(db)
		unifiedJournalService := services.NewUnifiedJournalService(db)
		paymentJournalSvc := services.NewPurchasePaymentJournalService(db, accountRepo, unifiedJournalService, nil)
		migrated, skipped, err := paymentJournalSvc.MigrateExistingPaymentsToSSOT()
		if err != nil {
			log.Printf("❌ Payment journal SSOT migration failed: %v", err)
		} else if migrated > 0 {
			log.Printf("✅ Payment journal migration: %d migrated, %d skipped", migrated, skipped)
		} else {
			log.Printf("✅ Payment journal SSOT migration: all up-to-date (%d already migrated)", skipped)
		}
	} else {
		log.Println("⏭️ Skipping payment journal SSOT backfill; set RUN_PAYMENT_SSOT_BACKFILL=true only for approved maintenance.")
	}

	// Balance rebuilding also writes financial data, so it must not be triggered
	// implicitly whenever the backend restarts.
	if os.Getenv("RUN_SSOT_BALANCE_SYNC") == "true" {
		balanceSyncSvc := services.NewBalanceSyncService(db)
		if err := balanceSyncSvc.SyncAccountBalancesFromSSOT(); err != nil {
			log.Printf("❌ Balance sync failed: %v", err)
		}
	} else {
		log.Println("⏭️ Skipping SSOT balance sync; set RUN_SSOT_BALANCE_SYNC=true only for approved maintenance.")
	}

	// Migrate permissions table
	if err := database.MigratePermissions(db); err != nil {
		log.Printf("Error migrating permissions: %v", err)
	}

	// Seed database with initial data
	database.SeedData(db)

	// Ensure backorder clearing liability account exists for estimated sales HPP.
	database.EnsureSalesBackorderClearingAccount(db)

	// Align journal access so employees can submit drafts while finance approves/posts them.
	database.JournalManualApprovalPermissionMigration(db)

	// Align legacy finance staff module permissions with the latest operational defaults.
	database.FinanceStaffDefaultPermissionMigration(db)

	// Align employee/staff access for purchase request and payment request workflows.
	database.EmployeeOperationalPermissionMigration(db)

	// Align legacy cash bank transfer columns with the approval workflow payload.
	database.CashBankTransferBookAmountColumnMigration(db)

	// Align cash & bank approval access so bank transfer approvals stay with finance manager.
	database.CashBankTransferApprovalPermissionMigration(db)

	// Repair legacy reversal journals that became effective before the original journal date.
	database.JournalReversalFutureDateAlignmentMigration(db)

	// Ensure default Asset Categories exist and migrate legacy asset categories
	database.AssetCategoryMigration(db)

	// Run startup tasks including fix account header status
	startupService := services.NewStartupService(db)
	startupService.RunStartupTasks()

	// Repair known legacy IDR cash/bank rows before recalculating balances.
	database.CashBankBookAmountRepairMigration(db)

	// Preserve the established startup behaviour by default, while allowing a
	// tenant with protected legacy balances to opt out explicitly.
	if os.Getenv("SKIP_CASHBANK_STARTUP_SYNC") != "true" {
		cashBankSyncSvc := services.NewCashBankCOASyncService(db)
		synced, err := cashBankSyncSvc.SyncAllAssets()
		if err != nil {
			log.Printf("❌ Cash/bank startup sync failed: %v", err)
		} else {
			log.Printf("✅ Cash/bank startup sync completed for %d accounts", synced)
		}
	} else {
		log.Println("⏭️ Skipping cash/bank startup sync for this tenant (SKIP_CASHBANK_STARTUP_SYNC=true).")
	}

	// Initialize and start session cleanup service
	sessionCleanupService := services.NewSessionCleanupService(db)
	go sessionCleanupService.StartCleanupWorker()

	// Initialize and start depreciation auto-scheduler
	// Runs daily check; auto-processes previous month's depreciation on 1st-5th of each month
	depreciationScheduler := services.NewDepreciationScheduler(db)
	go depreciationScheduler.Start()

	// Initialize Gin router without default middleware
	r := gin.New()

	// Add recovery middleware
	r.Use(gin.Recovery())

	// Add custom logger middleware only in development
	if cfg.Environment != "production" {
		r.Use(gin.Logger())
	}

	// Configure trusted proxies for security and accurate client IP detection
	trustedProxies := cfg.TrustedProxies
	if len(trustedProxies) == 0 {
		trustedProxies = []string{"127.0.0.1", "::1"}
		if cfg.Environment == "production" {
			trustedProxies = append(trustedProxies,
				"10.0.0.0/8",
				"172.16.0.0/12",
				"192.168.0.0/16",
			)
		}
	}
	if err := r.SetTrustedProxies(trustedProxies); err != nil {
		log.Printf("Warning: failed to set trusted proxies: %v", err)
	}

	// CORS middleware with dynamic origins
	allowedOrigins := config.GetAllowedOrigins(cfg)
	r.Use(cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			for _, allowedOrigin := range allowedOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					return true
				}
			}
			return false
		},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		// Include Cache-Control to satisfy browser preflight checks (Axios often sends it)
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "Cache-Control"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// Setup routes
	routes.SetupRoutes(r, db, startupService)

	// Start server
	port := cfg.ServerPort
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("Server starting on port %s", port)
		serverErrors <- srv.ListenAndServe()
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	case sig := <-signalCh:
		log.Printf("Shutdown signal received: %s", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Graceful shutdown failed: %v", err)
			if closeErr := srv.Close(); closeErr != nil {
				log.Printf("Forced shutdown failed: %v", closeErr)
			}
		}
	}

	log.Println("Server stopped cleanly")
}
