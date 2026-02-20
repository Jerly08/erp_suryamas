package main

import (
	"database/sql"
	"embed"
	"log"
	"os"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/services"

	_ "github.com/lib/pq"
)

// Embed migration files into binary
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	log.Println("🚀 Starting Accounting System...")

	// Load configuration
	cfg := config.LoadConfig()

	// Connect to database
	gormDB, err := database.ConnectDatabase(cfg)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}

	// Get SQL DB for migrations
	sqlDB, err := gormDB.DB()
	if err != nil {
		log.Fatalf("❌ Failed to get SQL DB: %v", err)
	}

	// ✅ RUN MIGRATIONS AUTOMATICALLY
	// This is the KEY part - migrations run on every startup
	// If git pull added new migration files, they auto-apply here
	migrationService := services.NewMigrationService(sqlDB)
	if err := migrationService.RunMigrations(migrationsFS, "migrations"); err != nil {
		// Check if it's a minor error we can ignore
		if err.Error() == "no change" {
			log.Println("✅ Database already up to date")
		} else {
			log.Fatalf("❌ Failed to run migrations: %v", err)
		}
	}

	// Get current migration version for logging
	version, dirty, err := migrationService.GetCurrentVersion(migrationsFS, "migrations")
	if err != nil {
		log.Printf("⚠️ Warning: Could not get migration version: %v", err)
	} else {
		if dirty {
			log.Printf("⚠️ Database is in dirty state at version %d", version)
		} else {
			log.Printf("✅ Database migration version: %d", version)
		}
	}

	// Continue with normal application startup
	log.Println("✅ Database migrations complete, starting application...")

	// Initialize services, routes, etc.
	// ...

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🌐 Server listening on port %s", port)
	// router.Run(":" + port)
}

/*
WORKFLOW EXAMPLE:

Developer A:
1. Creates migration:
   migrate create -ext sql -dir ./backend/migrations -seq add_unique_constraint
   
2. Edits files:
   - 000001_add_unique_constraint.up.sql
   - 000001_add_unique_constraint.down.sql
   
3. Tests locally:
   go run main.go  # Migration auto-runs
   
4. Commits:
   git add backend/migrations/
   git commit -m "Add unique constraint for GL accounts"
   git push

Developer B (teammate):
1. Git pull:
   git pull origin main
   
2. Restart app:
   go run main.go  # Migration AUTOMATICALLY applies!
   
3. Database updated! ✅

Production Deployment:
1. Git pull on server
2. Restart service: systemctl restart accounting-backend
3. Migrations auto-run on startup
4. Service starts with updated schema
5. Zero downtime! ✅

NO MANUAL SQL EXECUTION NEEDED!
*/
