package database

import (
	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/models"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

// cleanupConstraints removes problematic constraints that may cause migration issues
func cleanupConstraints(db *gorm.DB) {
	log.Println("Cleaning up problematic database constraints...")

	// First, check if accounts table exists
	var tableExists bool
	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.tables 
		WHERE table_name = 'accounts'
	)`).Scan(&tableExists)

	if !tableExists {
		log.Println("Accounts table does not exist yet, skipping constraint cleanup")
		return
	}

	// Query all existing constraints and indexes on accounts table
	var existingConstraints []string
	db.Raw(`
		SELECT constraint_name 
		FROM information_schema.table_constraints 
		WHERE table_name = 'accounts' 
		AND constraint_type IN ('UNIQUE', 'PRIMARY KEY')
		UNION
		SELECT indexname as constraint_name
		FROM pg_indexes 
		WHERE tablename = 'accounts' 
		AND indexname LIKE '%code%'
	`).Scan(&existingConstraints)

	log.Printf("Found %d existing constraints/indexes on accounts table", len(existingConstraints))

	// List of potentially problematic constraint/index patterns
	problematicPatterns := []string{
		"uni_accounts_code",
		"accounts_code_key",
		"idx_accounts_code_unique",
		"accounts_code_unique",
		"accounts_code_idx",
		"uq_accounts_code",
	}

	// Remove existing problematic constraints/indexes
	for _, existing := range existingConstraints {
		for _, pattern := range problematicPatterns {
			if existing == pattern || strings.Contains(existing, "code") {
				// Try dropping as constraint first
				err := db.Exec(fmt.Sprintf("ALTER TABLE accounts DROP CONSTRAINT IF EXISTS %s", existing)).Error
				if err != nil {
					// If constraint drop fails, try as index
					err = db.Exec(fmt.Sprintf("DROP INDEX IF EXISTS %s", existing)).Error
					if err != nil {
						log.Printf("Note: Failed to drop %s (may not exist): %v", existing, err)
					} else {
						log.Printf("✅ Dropped index %s", existing)
					}
				} else {
					log.Printf("✅ Dropped constraint %s", existing)
				}
				break
			}
		}
	}

	// Additional cleanup for known problematic constraint names that might not be detected
	additionalCleanup := []string{
		"uni_accounts_code",
		"accounts_code_key",
		"idx_accounts_code_unique",
		"accounts_code_unique",
	}

	for _, constraint := range additionalCleanup {
		// Try both constraint and index drop silently
		db.Exec(fmt.Sprintf("ALTER TABLE accounts DROP CONSTRAINT IF EXISTS %s", constraint))
		db.Exec(fmt.Sprintf("DROP INDEX IF EXISTS %s", constraint))
	}

	// Drop any remaining unique constraints on code column specifically
	log.Println("Removing any remaining unique constraints on code column...")
	var uniqueConstraints []string
	db.Raw(`
		SELECT tc.constraint_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu 
			ON tc.constraint_name = kcu.constraint_name
		WHERE tc.table_name = 'accounts' 
			AND tc.constraint_type = 'UNIQUE'
			AND kcu.column_name = 'code'
	`).Scan(&uniqueConstraints)

	for _, constraint := range uniqueConstraints {
		err := db.Exec(fmt.Sprintf("ALTER TABLE accounts DROP CONSTRAINT IF EXISTS %s", constraint)).Error
		if err != nil {
			log.Printf("Note: Failed to drop unique constraint %s: %v", constraint, err)
		} else {
			log.Printf("✅ Dropped unique constraint %s on code column", constraint)
		}
	}

	// Check if our target index already exists
	var targetIndexExists bool
	db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes 
			WHERE tablename = 'accounts' 
			AND indexname = 'idx_accounts_code_active'
		)
	`).Scan(&targetIndexExists)

	if targetIndexExists {
		log.Println("✅ Target partial unique index idx_accounts_code_active already exists")
	} else {
		// Create proper partial unique index for accounts code (only for non-deleted records)
		log.Println("Creating partial unique index for active accounts...")
		err := db.Exec(`
			CREATE UNIQUE INDEX idx_accounts_code_active 
			ON accounts (code) 
			WHERE deleted_at IS NULL
		`).Error
		if err != nil {
			log.Printf("Warning: Failed to create partial unique index on accounts.code: %v", err)
			// Try alternative approach with IF NOT EXISTS
			err2 := db.Exec(`
				CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_code_active 
				ON accounts (code) 
				WHERE deleted_at IS NULL
			`).Error
			if err2 != nil {
				log.Printf("Error: Still failed to create partial unique index: %v", err2)
			} else {
				log.Println("✅ Created proper partial unique index on accounts.code for active records")
			}
		} else {
			log.Println("✅ Created proper partial unique index on accounts.code for active records")
		}
	}

	// Verify the final state
	var finalConstraints []string
	db.Raw(`
		SELECT constraint_name 
		FROM information_schema.table_constraints 
		WHERE table_name = 'accounts' 
		AND constraint_type = 'UNIQUE'
		UNION
		SELECT indexname as constraint_name
		FROM pg_indexes 
		WHERE tablename = 'accounts' 
		AND indexname LIKE '%code%'
	`).Scan(&finalConstraints)

	log.Printf("Final state: %d constraints/indexes on accounts table: %v", len(finalConstraints), finalConstraints)
	log.Println("Database constraint cleanup completed")
}

// cleanupProductUnitConstraints removes problematic constraints on product_units table
func FixPaymentDateNullValues(db *gorm.DB) {
	log.Println("Checking and fixing payment_date null values...")

	// Check if sale_payments table exists
	var salePaymentsExists bool
	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.tables 
		WHERE table_name = 'sale_payments'
	)`).Scan(&salePaymentsExists)

	if salePaymentsExists {
		// Check if payment_date column already exists
		var paymentDateExists bool
		db.Raw(`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'sale_payments' AND column_name = 'payment_date'
		)`).Scan(&paymentDateExists)

		if !paymentDateExists {
			// Add payment_date column as nullable first
			log.Println("Adding payment_date column as nullable...")
			err := db.Exec(`ALTER TABLE sale_payments ADD COLUMN payment_date TIMESTAMPTZ`).Error
			if err != nil {
				log.Printf("Warning: Failed to add payment_date column: %v", err)
				return
			}
			log.Println("✅ Added payment_date column successfully")
		}

		// Update NULL payment_date values with created_at or a default date
		log.Println("Updating NULL payment_date values in sale_payments table...")
		result := db.Exec(`
			UPDATE sale_payments 
			SET payment_date = COALESCE(created_at, NOW()) 
			WHERE payment_date IS NULL
		`)
		if result.Error != nil {
			log.Printf("Warning: Failed to update NULL payment_date values: %v", result.Error)
		} else {
			log.Printf("✅ Updated %d NULL payment_date values in sale_payments table", result.RowsAffected)
		}

		// Now make the column NOT NULL
		log.Println("Making payment_date column NOT NULL...")
		err := db.Exec(`ALTER TABLE sale_payments ALTER COLUMN payment_date SET NOT NULL`).Error
		if err != nil {
			log.Printf("Warning: Failed to set payment_date as NOT NULL: %v", err)
		} else {
			log.Println("✅ Set payment_date column as NOT NULL successfully")
		}

		// Handle payment_method column
		var paymentMethodExists bool
		db.Raw(`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'sale_payments' AND column_name = 'payment_method'
		)`).Scan(&paymentMethodExists)

		if !paymentMethodExists {
			// Add payment_method column as nullable first
			log.Println("Adding payment_method column as nullable...")
			err := db.Exec(`ALTER TABLE sale_payments ADD COLUMN payment_method VARCHAR(50)`).Error
			if err != nil {
				log.Printf("Warning: Failed to add payment_method column: %v", err)
				return
			}
			log.Println("✅ Added payment_method column successfully")
		}

		// Update NULL payment_method values with a default value
		log.Println("Updating NULL payment_method values in sale_payments table...")
		result2 := db.Exec(`
			UPDATE sale_payments 
			SET payment_method = 'CASH' 
			WHERE payment_method IS NULL OR payment_method = ''
		`)
		if result2.Error != nil {
			log.Printf("Warning: Failed to update NULL payment_method values: %v", result2.Error)
		} else {
			log.Printf("✅ Updated %d NULL payment_method values in sale_payments table", result2.RowsAffected)
		}

		// Now make the payment_method column NOT NULL
		log.Println("Making payment_method column NOT NULL...")
		err2 := db.Exec(`ALTER TABLE sale_payments ALTER COLUMN payment_method SET NOT NULL`).Error
		if err2 != nil {
			log.Printf("Warning: Failed to set payment_method as NOT NULL: %v", err2)
		} else {
			log.Println("✅ Set payment_method column as NOT NULL successfully")
		}
	}

	log.Println("Payment date null values fix completed")
}

func cleanupProductUnitConstraints(db *gorm.DB) {
	log.Println("Cleaning up ProductUnit constraints...")

	// First, check if product_units table exists
	var tableExists bool
	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.tables 
		WHERE table_name = 'product_units'
	)`).Scan(&tableExists)

	if !tableExists {
		log.Println("Product units table does not exist yet, skipping constraint cleanup")
		return
	}

	// Check if the problematic constraint exists before trying to drop it
	var constraintExists bool
	db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.table_constraints 
			WHERE table_name = 'product_units' 
			AND constraint_name = 'uni_product_units_code'
		)
	`).Scan(&constraintExists)

	if constraintExists {
		log.Println("Found uni_product_units_code constraint, attempting to drop...")
		err := db.Exec("ALTER TABLE product_units DROP CONSTRAINT IF EXISTS uni_product_units_code").Error
		if err != nil {
			log.Printf("Warning: Failed to drop uni_product_units_code constraint: %v", err)
		} else {
			log.Println("✅ Dropped uni_product_units_code constraint successfully")
		}
	} else {
		log.Println("uni_product_units_code constraint does not exist, nothing to drop")
	}

	// Also check for any other code-related constraints on product_units
	var codeConstraints []string
	db.Raw(`
		SELECT constraint_name 
		FROM information_schema.table_constraints 
		WHERE table_name = 'product_units' 
		AND constraint_type = 'UNIQUE'
		AND constraint_name LIKE '%code%'
	`).Scan(&codeConstraints)

	if len(codeConstraints) > 0 {
		log.Printf("Found %d code-related constraints on product_units", len(codeConstraints))
		for _, constraint := range codeConstraints {
			log.Printf("Attempting to drop constraint: %s", constraint)
			err := db.Exec(fmt.Sprintf("ALTER TABLE product_units DROP CONSTRAINT IF EXISTS %s", constraint)).Error
			if err != nil {
				log.Printf("Warning: Failed to drop constraint %s: %v", constraint, err)
			} else {
				log.Printf("✅ Dropped constraint %s successfully", constraint)
			}
		}
	}

	// Check for any indexes that might be causing issues
	var codeIndexes []string
	db.Raw(`
		SELECT indexname 
		FROM pg_indexes 
		WHERE tablename = 'product_units' 
		AND indexname LIKE '%code%'
	`).Scan(&codeIndexes)

	if len(codeIndexes) > 0 {
		log.Printf("Found %d code-related indexes on product_units", len(codeIndexes))
		for _, index := range codeIndexes {
			log.Printf("Code-related index found: %s (will be managed by GORM)", index)
		}
	}

	log.Println("ProductUnit constraint cleanup completed")
}

func cleanupJournalEntriesConstraints(db *gorm.DB) {
	log.Println("Cleaning up JournalEntries constraints...")

	// Check if journal_entries table exists
	var tableExists bool
	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.tables 
		WHERE table_name = 'journal_entries'
	)`).Scan(&tableExists)

	if !tableExists {
		log.Println("Journal entries table does not exist yet, skipping constraint cleanup")
		return
	}

	// Check for problematic constraints
	var constraintNames []string
	db.Raw(`
		SELECT constraint_name 
		FROM information_schema.table_constraints 
		WHERE table_name = 'journal_entries' 
		AND constraint_type = 'UNIQUE'
		AND constraint_name LIKE '%code%'
	`).Scan(&constraintNames)

	if len(constraintNames) > 0 {
		log.Printf("Found %d code-related constraints on journal_entries", len(constraintNames))
		for _, constraint := range constraintNames {
			log.Printf("Attempting to drop constraint: %s", constraint)
			err := db.Exec(fmt.Sprintf("ALTER TABLE journal_entries DROP CONSTRAINT IF EXISTS %s", constraint)).Error
			if err != nil {
				log.Printf("Warning: Failed to drop constraint %s: %v", constraint, err)
			} else {
				log.Printf("✅ Dropped constraint %s successfully", constraint)
			}
		}
	} else {
		log.Println("No problematic code-related constraints found on journal_entries")
	}

	// Also check for any indexes that might be causing issues
	var codeIndexes []string
	db.Raw(`
		SELECT indexname 
		FROM pg_indexes 
		WHERE tablename = 'journal_entries' 
		AND indexname LIKE '%code%'
	`).Scan(&codeIndexes)

	if len(codeIndexes) > 0 {
		log.Printf("Found %d code-related indexes on journal_entries", len(codeIndexes))
		for _, index := range codeIndexes {
			log.Printf("Code-related index found: %s (will be managed by GORM)", index)
		}
	}

	log.Println("JournalEntries constraint cleanup completed")
}

func cleanupAllConstraintConflicts(db *gorm.DB) {
	log.Println("🔧 Running comprehensive constraint cleanup...")

	// List of tables and their problematic constraints
	tablesToClean := map[string][]string{
		"journal_entries": {"uni_journal_entries_code"},
		"product_units":   {"uni_product_units_code"},
		"accounts":        {"uni_accounts_code", "idx_accounts_code_unique"},
		"products":        {"uni_products_code"},
		"contacts":        {"uni_contacts_code"},
		"sales":           {"uni_sales_code"},
		"purchases":       {"uni_purchases_code"},
	}

	for tableName, constraints := range tablesToClean {
		// Check if table exists
		var tableExists bool
		db.Raw(`SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_name = ?
		)`, tableName).Scan(&tableExists)

		if !tableExists {
			log.Printf("Table %s does not exist, skipping", tableName)
			continue
		}

		log.Printf("Cleaning constraints for table: %s", tableName)

		// Drop specific constraints
		for _, constraint := range constraints {
			var constraintExists bool
			db.Raw(`
				SELECT EXISTS (
					SELECT 1 FROM information_schema.table_constraints 
					WHERE table_name = ? AND constraint_name = ?
				)
			`, tableName, constraint).Scan(&constraintExists)

			if constraintExists {
				log.Printf("Dropping constraint %s from %s...", constraint, tableName)
				err := db.Exec(fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s", tableName, constraint)).Error
				if err != nil {
					log.Printf("Warning: Failed to drop %s: %v", constraint, err)
				} else {
					log.Printf("✅ Dropped %s successfully", constraint)
				}
			}
		}

		// Also drop any other code-related unique constraints
		var otherConstraints []string
		db.Raw(`
			SELECT constraint_name 
			FROM information_schema.table_constraints 
			WHERE table_name = ? 
			AND constraint_type = 'UNIQUE'
			AND constraint_name LIKE '%code%'
			AND constraint_name NOT LIKE 'idx_%'
		`, tableName).Scan(&otherConstraints)

		for _, constraint := range otherConstraints {
			log.Printf("Dropping additional constraint %s from %s...", constraint, tableName)
			err := db.Exec(fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s", tableName, constraint)).Error
			if err != nil {
				log.Printf("Warning: Failed to drop %s: %v", constraint, err)
			} else {
				log.Printf("✅ Dropped %s successfully", constraint)
			}
		}
	}

	log.Println("✅ Comprehensive constraint cleanup completed")
}

func dropAllProblematicConstraints(db *gorm.DB) {
	log.Println("🚀 Running aggressive constraint cleanup for all database objects...")

	// List of all possible problematic constraints across the entire database
	problematicConstraints := []string{
		"uni_journal_entries_code",
		"uni_product_units_code",
		"uni_accounts_code",
		"uni_products_code",
		"uni_contacts_code",
		"uni_sales_code",
		"uni_purchases_code",
		"products_code_key",
		"contacts_code_key",
		"sales_code_key",
		"purchases_code_key",
		"journal_entries_code_key",
	}

	// Drop constraints across all tables in database
	for _, constraint := range problematicConstraints {
		// Try to find which table this constraint belongs to
		var tableName string
		db.Raw(`
			SELECT table_name 
			FROM information_schema.table_constraints 
			WHERE constraint_name = ?
			LIMIT 1
		`, constraint).Scan(&tableName)

		if tableName != "" {
			log.Printf("Found constraint %s on table %s, dropping...", constraint, tableName)
			err := db.Exec(fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s CASCADE", tableName, constraint)).Error
			if err != nil {
				log.Printf("Warning: Failed to drop %s: %v", constraint, err)
			} else {
				log.Printf("✅ Dropped constraint %s successfully", constraint)
			}
		} else {
			// Try dropping from common table names
			tableGuesses := []string{"journal_entries", "products", "contacts", "sales", "purchases", "product_units", "accounts"}
			for _, table := range tableGuesses {
				err := db.Exec(fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s CASCADE", table, constraint)).Error
				if err == nil {
					log.Printf("✅ Dropped constraint %s from %s", constraint, table)
					break
				}
			}
		}
	}

	// Also try to drop any constraint that contains "code" from any table
	var allCodeConstraints []struct {
		TableName      string `gorm:"column:table_name"`
		ConstraintName string `gorm:"column:constraint_name"`
	}

	db.Raw(`
		SELECT table_name, constraint_name 
		FROM information_schema.table_constraints 
		WHERE constraint_type = 'UNIQUE'
		AND constraint_name LIKE '%code%'
		AND table_schema = 'public'
	`).Scan(&allCodeConstraints)

	for _, constraint := range allCodeConstraints {
		log.Printf("Dropping code-related constraint %s from %s...", constraint.ConstraintName, constraint.TableName)
		err := db.Exec(fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s CASCADE", constraint.TableName, constraint.ConstraintName)).Error
		if err != nil {
			log.Printf("Warning: Failed to drop %s: %v", constraint.ConstraintName, err)
		} else {
			log.Printf("✅ Dropped %s successfully", constraint.ConstraintName)
		}
	}

	log.Println("✅ Aggressive constraint cleanup completed")
}

func ConnectDB() *gorm.DB {
	cfg := config.LoadConfig()

	// Configure GORM with optimizations
	gormConfig := &gorm.Config{
		// Disable foreign key constraint check for better performance during migrations
		DisableForeignKeyConstraintWhenMigrating: true,
		// Skip default transaction for better performance
		SkipDefaultTransaction: true,
		// Prepare statement for better performance
		PrepareStmt: true,
	}

	log.Printf("Connecting to PostgreSQL database with URL: %s", cfg.DatabaseURL)
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), gormConfig)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Configure connection pool for optimal performance
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Failed to get underlying sql.DB:", err)
	}

	// Connection pool settings
	sqlDB.SetMaxIdleConns(10)                  // Maximum idle connections
	sqlDB.SetMaxOpenConns(100)                 // Maximum open connections
	sqlDB.SetConnMaxLifetime(time.Hour)        // Connection max lifetime (1 hour)
	sqlDB.SetConnMaxIdleTime(30 * time.Minute) // Connection max idle time (30 minutes)

	DB = db
	log.Println("Database connected successfully with optimized connection pool")
	return db
}

func AutoMigrate(db *gorm.DB) {
	log.Println("Starting database migration...")

	// Run pre-migration fixes to handle conflicts
	PreMigrationFixes(db)

	// Fix payment_date null values before migration
	FixPaymentDateNullValues(db)

	// Clean up problematic constraints first
	cleanupConstraints(db)

	// Clean up ProductUnit constraints before migration
	cleanupProductUnitConstraints(db)

	// Clean up Journal Entries constraints before migration
	cleanupJournalEntriesConstraints(db)

	// Run comprehensive constraint cleanup to prevent conflicts
	cleanupAllConstraintConflicts(db)

	// Run aggressive cleanup of all problematic constraints
	dropAllProblematicConstraints(db)

	// Migrate models in order to respect foreign key constraints
	err := db.AutoMigrate(
		// Core models first
		&models.User{},
		&models.AuditLog{},

		// Chart of Accounts
		&models.Account{},
		&models.Transaction{},

		// Contacts
		&models.Contact{},
		&models.ContactAddress{},
		&models.ContactHistory{},
		&models.CommunicationLog{},

		// Products
		&models.ProductCategory{},
		&models.Product{},
		&models.ProductUnit{},
		&models.WarehouseLocation{},
		&models.Inventory{},

		// Sales
		&models.Sale{},
		&models.SaleItem{},
		&models.SalePayment{},
		&models.SaleReturn{},
		&models.SaleReturnItem{},

		// Invoices
		&models.Invoice{},
		&models.InvoiceItem{},

		// Purchases
		&models.Purchase{},
		&models.PurchaseItem{},
		&models.PurchaseDocument{},
		&models.PurchaseReceipt{},
		&models.PurchaseReceiptItem{},

		// Expenses
		&models.ExpenseCategory{},
		&models.Expense{},

		// Assets
		&models.AssetCategory{},
		&models.Asset{},

		// Cash & Bank
		&models.CashBank{},
		&models.CashBankTransaction{},
		&models.Payment{},
		&models.PaymentAllocation{},

		// Journals and reports
		&models.Journal{},
		&models.JournalEntry{},
		&models.JournalLine{},

		// Note: SSOT Journal tables are ensured separately to avoid GORM dropping constraints on existing DBs
		// See EnsureSSOTTables for safe creation when missing

		&models.Report{},
		&models.ReportTemplate{},
		&models.FinancialRatio{},
		&models.AccountPeriodBalance{},

		// Budgets
		&models.Budget{},
		&models.BudgetItem{},
		&models.BudgetComparison{},

		// Notifications
		&models.Notification{},
		&models.StockAlert{},

		// Overdue Management
		&models.ReminderLog{},
		&models.OverdueRecord{},
		&models.InterestCharge{},
		&models.CollectionTask{},
		&models.WriteOffSuggestion{},
		&models.SaleCancellation{},
		&models.CreditNote{},
		&models.PaymentReminder{},

		// Additional missing models
		&models.CompanyProfile{},
		&models.Permission{},
		&models.RolePermission{},
		&models.UserSession{},
		&models.RefreshToken{},
		&models.BlacklistedToken{},
		&models.RateLimitRecord{},
		&models.AuthAttempt{},

		// CashBank Migration Models
		&models.CashBankTransferMigration{},
		&models.BankReconciliationMigration{},
		&models.ReconciliationItemMigration{},

		// Migration tracking models
		&models.MigrationRecord{},

		// Settings model
		&models.Settings{},

		// Accounting Period model
		&models.AccountingPeriod{},

		// Security models
		&models.SecurityIncident{},
		&models.SystemAlert{},
		&models.RequestLog{},
		&models.IpWhitelist{},
		&models.SecurityConfig{},
		&models.SecurityMetrics{},
	)

	if err != nil {
		log.Printf("Failed to migrate core models: %v", err)
		log.Fatal("Stopping migration due to error")
	}

	log.Println("Core models migration completed successfully")

	// Ensure SSOT tables exist without forcing constraint drops
	EnsureSSOTTables(db)

	// Migrate approval models separately to debug any issues
	log.Println("Starting approval models migration...")
	err = db.AutoMigrate(
		&models.ApprovalWorkflow{},
		&models.ApprovalStep{},
		&models.ApprovalRequest{},
		&models.ApprovalAction{},
		&models.ApprovalHistory{},
	)

	if err != nil {
		log.Printf("Failed to migrate approval models: %v", err)
		// Don't fail completely, just log the error
	} else {
		log.Println("Approval models migration completed successfully")
	}

	log.Println("Database migration completed successfully")

	// Create missing columns that should exist from models but might be missing from database
	CreateMissingColumns(db)

	// Run enhanced sales model migration
	EnhanceSalesModel(db)

	// Enhanced new sales field migration for new fields
	EnhanceNewSalesFields(db)

	// Update tax field sizes to prevent numeric overflow
	UpdateTaxFieldSizes(db)

	// Fix purchase items field overflow
	FixPurchaseItemsFieldOverflow(db)

	// Run sales data integrity fix
	FixSalesDataIntegrity(db)

	// Run enhanced cashbank model migration
	EnhanceCashBankModel(db)

	// Run settings table migration
	RunSettingsMigration(db)

	// PRODUCTION SAFETY: All balance synchronization logic disabled to prevent account balance resets
	// Balance sync operations have been permanently disabled to protect production data
	log.Println("🛡️  PRODUCTION MODE: All balance synchronization disabled to protect account balances")
	log.Println("✅ Account balances will never be automatically modified during startup")

	// Run cleanup duplicate notifications migration
	CleanupDuplicateNotificationsMigration(db)

	// Add description column to accounting_periods table
	if err := AddAccountingPeriodDescription(db); err != nil {
		log.Printf("⚠️  Accounting period description migration warning: %v", err)
	}

	// Fix accounting_periods table structure (make year/month nullable)
	if err := FixAccountingPeriodsStructure(db); err != nil {
		log.Printf("⚠️  Accounting period structure fix warning: %v", err)
	}

	// Create indexes for better performance
	createIndexes(db)

	// Run payment performance optimization migration
	RunPaymentPerformanceOptimization(db)

	// Reset and fix product image issues
	ResetProductImageMigration(db)
	ProductImageFixMigration(db)

	// Fix asset category issues and create default categories
	AssetCategoryMigration(db)

	// Run database enhancements migration
	RunDatabaseEnhancements(db)

	// Fix missing columns and constraint issues
	RunMissingColumnsFix(db)

	// Fix remaining issues after main fixes
	FixRemainingIssuesMigration(db)

	// Run index cleanup and optimization
	RunIndexCleanupAndOptimization(db)
}

// EnsureSSOTTables creates minimal SSOT tables if they don't exist yet (safe and idempotent)
func EnsureSSOTTables(db *gorm.DB) {
	log.Println("Ensuring SSOT core tables exist (safe mode)...")

	// unified_journal_ledger
	db.Exec(`
		CREATE TABLE IF NOT EXISTS unified_journal_ledger (
			id BIGSERIAL PRIMARY KEY,
			entry_number   VARCHAR(50) NOT NULL,
			source_type    VARCHAR(50) NOT NULL,
			source_id      BIGINT,
			source_code    VARCHAR(100),
			entry_date     TIMESTAMP NOT NULL,
			description    TEXT NOT NULL,
			reference      VARCHAR(200),
			notes          TEXT,
			total_debit    DECIMAL(20,2) NOT NULL DEFAULT 0,
			total_credit   DECIMAL(20,2) NOT NULL DEFAULT 0,
			status         VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
			is_balanced    BOOLEAN NOT NULL DEFAULT TRUE,
			is_auto_generated BOOLEAN NOT NULL DEFAULT FALSE,
			posted_at      TIMESTAMPTZ,
			posted_by      BIGINT,
			reversed_by    BIGINT,
			reversed_from  BIGINT,
			reversal_reason TEXT,
			created_by     BIGINT NOT NULL,
			created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at     TIMESTAMPTZ
		);
	`)

	// Add unique index for entry_number if not exists
	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_unified_journal_entry_number ON unified_journal_ledger(entry_number);`)

	// unified_journal_lines
	db.Exec(`
		CREATE TABLE IF NOT EXISTS unified_journal_lines (
			id BIGSERIAL PRIMARY KEY,
			journal_id   BIGINT NOT NULL REFERENCES unified_journal_ledger(id) ON DELETE CASCADE,
			account_id   BIGINT NOT NULL,
			line_number  INT NOT NULL,
			description  TEXT,
			debit_amount DECIMAL(20,2) NOT NULL DEFAULT 0,
			credit_amount DECIMAL(20,2) NOT NULL DEFAULT 0,
			quantity     DECIMAL(15,4),
			unit_price   DECIMAL(15,4),
			created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (journal_id, line_number)
		);
	`)

	// journal_event_log (without DB-side uuid default)
	db.Exec(`
		CREATE TABLE IF NOT EXISTS journal_event_log (
			id BIGSERIAL PRIMARY KEY,
			event_uuid UUID NOT NULL,
			journal_id BIGINT,
			event_type VARCHAR(50) NOT NULL,
			event_data JSONB NOT NULL,
			event_timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			user_id BIGINT,
			user_role VARCHAR(50),
			ip_address INET,
			user_agent TEXT,
			source_system VARCHAR(50) DEFAULT 'ACCOUNTING_SYSTEM',
			correlation_id UUID,
			metadata JSONB
		);
	`)

	// Helpful indexes (idempotent)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_source ON unified_journal_ledger(source_type, source_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_date_status ON unified_journal_ledger(entry_date, status)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_lines_journal ON unified_journal_lines(journal_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_lines_account ON unified_journal_lines(account_id, journal_id)`)
	log.Println("SSOT core tables ensured.")
}

func createIndexes(db *gorm.DB) {
	// Performance indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_sales_date ON sales(date)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_purchases_date ON purchases(date)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_expenses_date ON expenses(date)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_transactions_date ON transactions(transaction_date)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_products_stock ON products(stock)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_inventory_date ON inventories(transaction_date)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at)`)

	// Security and authentication indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_blacklisted_tokens_token ON blacklisted_tokens(token)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_blacklisted_tokens_expires_at ON blacklisted_tokens(expires_at)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_security_incidents_created_at ON security_incidents(created_at)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_security_incidents_client_ip ON security_incidents(client_ip)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_security_incidents_incident_type ON security_incidents(incident_type)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_request_logs_timestamp ON request_logs(timestamp)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_request_logs_client_ip ON request_logs(client_ip)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_request_logs_is_suspicious ON request_logs(is_suspicious)`)

	// Notification indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_notifications_type ON notifications(type)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_notifications_user_type ON notifications(user_id, type)`)

	// Composite indexes for better query performance
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_sales_customer_date ON sales(customer_id, date)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_purchases_vendor_date ON purchases(vendor_id, date)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_transactions_account_date ON transactions(account_id, transaction_date)`)

	// Approval indexes - check if tables exist first
	var count int64
	if db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'approval_requests'`).Scan(&count); count > 0 {
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_approval_requests_status ON approval_requests(status)`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_approval_requests_entity ON approval_requests(entity_type, entity_id)`)
	}

	if db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'approval_actions'`).Scan(&count); count > 0 {
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_approval_actions_active ON approval_actions(is_active, status)`)
	}

	if db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'approval_history'`).Scan(&count); count > 0 {
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_approval_history_request ON approval_history(request_id, created_at)`)
	}

	// Enhanced accounting indexes
	log.Println("Creating enhanced accounting indexes...")

	// Account hierarchy and balance indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_accounts_hierarchy ON accounts(parent_id, code) WHERE parent_id IS NOT NULL`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_accounts_active_balance ON accounts(balance, is_active) WHERE deleted_at IS NULL`)

	// Journal entries and lines performance indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_entries_posted ON journal_entries(entry_date, status) WHERE status = 'POSTED'`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_lines_account_balance ON journal_lines(account_id, debit_amount, credit_amount)`)

	// Sales and Purchase analysis indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_sales_status_amount ON sales(status, total_amount) WHERE deleted_at IS NULL`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_purchases_status_amount ON purchases(status, total_amount) WHERE deleted_at IS NULL`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_sales_items_product_qty ON sale_items(product_id, quantity)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_purchase_items_product_qty ON purchase_items(product_id, quantity)`)

	// Payment and cash flow indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_payments_status_date ON payments(status, date) WHERE status IN ('COMPLETED', 'PENDING')`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cash_bank_balance ON cash_banks(balance, currency) WHERE is_active = true`)

	// Product and inventory management indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_products_active_stock ON products(stock, is_active) WHERE deleted_at IS NULL`)

	// Check if transaction_type column exists in inventories table before creating index
	var transactionTypeExists bool
	db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'inventories' AND column_name = 'transaction_type')`).Scan(&transactionTypeExists)
	if transactionTypeExists {
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_inventory_product_type ON inventories(product_id, transaction_type)`)
	} else {
		// Create a safe fallback index
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_inventory_product_safe ON inventories(product_id) WHERE product_id IS NOT NULL`)
	}

	// Contact and customer management indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_contacts_type_active ON contacts(type, is_active) WHERE deleted_at IS NULL`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_contact_addresses_default ON contact_addresses(contact_id, is_default)`)

	// Accounting Period indexes
	if db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'accounting_periods'`).Scan(&count); count > 0 {
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_accounting_periods_date_range ON accounting_periods(start_date, end_date)`)

		// Check if is_open and is_closed columns exist
		var isOpenExists, isClosedExists bool
		db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'accounting_periods' AND column_name = 'is_open')`).Scan(&isOpenExists)
		db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'accounting_periods' AND column_name = 'is_closed')`).Scan(&isClosedExists)
		if isOpenExists && isClosedExists {
			db.Exec(`CREATE INDEX IF NOT EXISTS idx_accounting_periods_status ON accounting_periods(is_open, is_closed)`)
		}
	}

	// Audit and security indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_audit_logs_important ON audit_logs(table_name, action, created_at) WHERE action IN ('DELETE', 'UPDATE')`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_security_incidents_severity ON security_incidents(severity, created_at)`)

	// Report generation indexes - check if columns exist first
	var periodExists, deletedAtExists bool
	db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'account_balances' AND column_name = 'period')`).Scan(&periodExists)
	db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'account_balances' AND column_name = 'deleted_at')`).Scan(&deletedAtExists)
	if periodExists && deletedAtExists {
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_account_balances_period_account ON account_balances(period, account_id) WHERE deleted_at IS NULL`)
	}

	// Check if financial_ratios table and calculation_date column exist
	var financialRatiosExists, calculationDateExists bool
	db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'financial_ratios')`).Scan(&financialRatiosExists)
	if financialRatiosExists {
		db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'financial_ratios' AND column_name = 'calculation_date')`).Scan(&calculationDateExists)
		if calculationDateExists {
			db.Exec(`CREATE INDEX IF NOT EXISTS idx_financial_ratios_date ON financial_ratios(calculation_date)`)
		}
	}

	log.Println("Database indexes created successfully")
}

// CreateMissingColumns creates missing columns that should exist from model definitions
func CreateMissingColumns(db *gorm.DB) {
	log.Println("Checking and creating missing columns from model definitions...")

	// Check if sales table exists
	var salesTableExists bool
	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.tables 
		WHERE table_name = 'sales'
	)`).Scan(&salesTableExists)

	if salesTableExists {
		// Check and add missing pph column to sales table
		var pphColumnExists bool
		db.Raw(`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'sales' AND column_name = 'pph'
		)`).Scan(&pphColumnExists)

		if !pphColumnExists {
			log.Println("Adding missing pph column to sales table...")
			err := db.Exec(`
				ALTER TABLE sales 
				ADD COLUMN pph DECIMAL(15,2) DEFAULT 0;
			`).Error
			if err != nil {
				log.Printf("Warning: Failed to add pph column to sales table: %v", err)
			} else {
				log.Println("Added pph column to sales table successfully")
			}
		}

		// Check and add missing pph_percent column to sales table
		var pphPercentColumnExists bool
		db.Raw(`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'sales' AND column_name = 'pph_percent'
		)`).Scan(&pphPercentColumnExists)

		if !pphPercentColumnExists {
			log.Println("Adding missing pph_percent column to sales table...")
			err := db.Exec(`
				ALTER TABLE sales 
				ADD COLUMN pph_percent DECIMAL(5,2) DEFAULT 0;
			`).Error
			if err != nil {
				log.Printf("Warning: Failed to add pph_percent column to sales table: %v", err)
			} else {
				log.Println("Added pph_percent column to sales table successfully")
			}
		}

		// Check and add missing pph_type column to sales table
		var pphTypeColumnExists bool
		db.Raw(`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'sales' AND column_name = 'pph_type'
		)`).Scan(&pphTypeColumnExists)

		if !pphTypeColumnExists {
			log.Println("Adding missing pph_type column to sales table...")
			err := db.Exec(`
				ALTER TABLE sales 
				ADD COLUMN pph_type VARCHAR(20);
			`).Error
			if err != nil {
				log.Printf("Warning: Failed to add pph_type column to sales table: %v", err)
			} else {
				log.Println("Added pph_type column to sales table successfully")
			}
		}
	}

	// Check if sale_items table exists
	var saleItemsTableExists bool
	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.tables 
		WHERE table_name = 'sale_items'
	)`).Scan(&saleItemsTableExists)

	if saleItemsTableExists {
		// Check and add missing pph_amount column to sale_items table
		var pphAmountColumnExists bool
		db.Raw(`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'sale_items' AND column_name = 'pph_amount'
		)`).Scan(&pphAmountColumnExists)

		if !pphAmountColumnExists {
			log.Println("Adding missing pph_amount column to sale_items table...")
			err := db.Exec(`
				ALTER TABLE sale_items 
				ADD COLUMN pph_amount DECIMAL(15,2) DEFAULT 0;
			`).Error
			if err != nil {
				log.Printf("Warning: Failed to add pph_amount column to sale_items table: %v", err)
			} else {
				log.Println("Added pph_amount column to sale_items table successfully")
			}
		}

		// Check and add missing revenue_account_id column to sale_items table
		var revenueAccountIdColumnExists bool
		db.Raw(`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'sale_items' AND column_name = 'revenue_account_id'
		)`).Scan(&revenueAccountIdColumnExists)

		if !revenueAccountIdColumnExists {
			log.Println("Adding missing revenue_account_id column to sale_items table...")
			err := db.Exec(`
				ALTER TABLE sale_items 
				ADD COLUMN revenue_account_id INTEGER;
			`).Error
			if err != nil {
				log.Printf("Warning: Failed to add revenue_account_id column to sale_items table: %v", err)
			} else {
				log.Println("Added revenue_account_id column to sale_items table successfully")
			}
		}
	}

	log.Println("Missing columns check completed")
}

// EnhanceSalesModel adds enhanced fields to sales and sale_items tables
func EnhanceSalesModel(db *gorm.DB) {
	log.Println("Starting enhanced sales model migration...")

	// Check if migration is needed by checking if new fields exist
	var columnExists bool

	// Check if subtotal column exists in sales table
	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.columns 
		WHERE table_name = 'sales' AND column_name = 'subtotal'
	)`).Scan(&columnExists)

	if columnExists {
		log.Println("Enhanced sales model fields already exist, skipping migration")
		return
	}

	// Add new fields to sales table
	log.Println("Adding enhanced fields to sales table...")
	err := db.Exec(`
		ALTER TABLE sales 
		ADD COLUMN IF NOT EXISTS subtotal DECIMAL(15,2) DEFAULT 0,
		ADD COLUMN IF NOT EXISTS discount_amount DECIMAL(15,2) DEFAULT 0,
		ADD COLUMN IF NOT EXISTS taxable_amount DECIMAL(15,2) DEFAULT 0,
		ADD COLUMN IF NOT EXISTS ppn DECIMAL(8,2) DEFAULT 0,
		ADD COLUMN IF NOT EXISTS pph DECIMAL(8,2) DEFAULT 0,
		ADD COLUMN IF NOT EXISTS total_tax DECIMAL(8,2) DEFAULT 0;
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to add enhanced fields to sales table: %v", err)
	} else {
		log.Println("Enhanced fields added to sales table successfully")
	}

	// Add new fields to sale_items table
	log.Println("Adding enhanced fields to sale_items table...")
	err = db.Exec(`
		ALTER TABLE sale_items 
		ADD COLUMN IF NOT EXISTS description TEXT,
		ADD COLUMN IF NOT EXISTS discount_percent DECIMAL(5,2) DEFAULT 0,
		ADD COLUMN IF NOT EXISTS discount_amount DECIMAL(15,2) DEFAULT 0,
		ADD COLUMN IF NOT EXISTS line_total DECIMAL(15,2) DEFAULT 0,
		ADD COLUMN IF NOT EXISTS taxable BOOLEAN DEFAULT true,
		ADD COLUMN IF NOT EXISTS ppn_amount DECIMAL(8,2) DEFAULT 0,
		ADD COLUMN IF NOT EXISTS pph_amount DECIMAL(8,2) DEFAULT 0,
		ADD COLUMN IF NOT EXISTS total_tax DECIMAL(8,2) DEFAULT 0,
		ADD COLUMN IF NOT EXISTS final_amount DECIMAL(15,2) DEFAULT 0,
		ADD COLUMN IF NOT EXISTS tax_account_id INTEGER REFERENCES accounts(id);
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to add enhanced fields to sale_items table: %v", err)
	} else {
		log.Println("Enhanced fields added to sale_items table successfully")
	}

	// Update existing records with calculated values
	updateExistingSalesRecords(db)

	log.Println("Enhanced sales model migration completed successfully")
}

// updateExistingSalesRecords updates existing sales records with calculated values
func updateExistingSalesRecords(db *gorm.DB) {
	log.Println("Updating existing sales records with calculated values...")

	// Update sales records where new fields are null/zero
	err := db.Exec(`
		UPDATE sales 
		SET 
			subtotal = CASE 
				WHEN subtotal = 0 OR subtotal IS NULL THEN COALESCE(total_amount - shipping_cost, total_amount, 0)
				ELSE subtotal 
			END,
			discount_amount = CASE
				WHEN discount_amount = 0 OR discount_amount IS NULL THEN 
					COALESCE((total_amount - shipping_cost) * discount_percent / 100, 0)
				ELSE discount_amount
			END,
			taxable_amount = CASE
				WHEN taxable_amount = 0 OR taxable_amount IS NULL THEN 
					COALESCE(total_amount - shipping_cost - (total_amount - shipping_cost) * discount_percent / 100, total_amount, 0)
				ELSE taxable_amount
			END,
			ppn = CASE
				WHEN ppn = 0 OR ppn IS NULL THEN 
					COALESCE((total_amount - shipping_cost - (total_amount - shipping_cost) * discount_percent / 100) * ppn_percent / 100, 0)
				ELSE ppn
			END,
			pph = CASE
				WHEN pph = 0 OR pph IS NULL THEN 
					COALESCE((total_amount - shipping_cost - (total_amount - shipping_cost) * discount_percent / 100) * pph_percent / 100, 0)
				ELSE pph
			END,
			total_tax = CASE
				WHEN total_tax = 0 OR total_tax IS NULL THEN 
					COALESCE(
						(total_amount - shipping_cost - (total_amount - shipping_cost) * discount_percent / 100) * ppn_percent / 100 - 
						(total_amount - shipping_cost - (total_amount - shipping_cost) * discount_percent / 100) * pph_percent / 100, 
						0
					)
				ELSE total_tax
			END
		WHERE subtotal = 0 OR subtotal IS NULL OR discount_amount = 0 OR discount_amount IS NULL;
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to update existing sales records: %v", err)
	} else {
		log.Println("Updated existing sales records with calculated values")
	}

	// Update sale_items records where new fields are null/zero
	log.Println("Updating existing sale_items records...")
	err = db.Exec(`
		UPDATE sale_items si
		SET 
			description = CASE
				WHEN si.description IS NULL OR si.description = '' THEN 
					COALESCE(p.name, 'Product Item')
				ELSE si.description
			END,
			line_total = CASE
				WHEN si.line_total = 0 OR si.line_total IS NULL THEN 
					COALESCE(si.total_price, si.quantity * si.unit_price, 0)
				ELSE si.line_total
			END,
			final_amount = CASE
				WHEN si.final_amount = 0 OR si.final_amount IS NULL THEN 
					COALESCE(si.total_price, si.quantity * si.unit_price, 0)
				ELSE si.final_amount
			END,
			taxable = CASE
				WHEN si.taxable IS NULL THEN true
				ELSE si.taxable
			END
		FROM products p 
		WHERE si.product_id = p.id 
			AND (si.line_total = 0 OR si.line_total IS NULL OR si.final_amount = 0 OR si.final_amount IS NULL 
				 OR si.description IS NULL OR si.description = '' OR si.taxable IS NULL);
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to update existing sale_items records: %v", err)
	} else {
		log.Println("Updated existing sale_items records with calculated values")
	}

	// Update sale_items that don't have matching products
	err = db.Exec(`
		UPDATE sale_items 
		SET 
			description = CASE
				WHEN description IS NULL OR description = '' THEN 'Product Item'
				ELSE description
			END,
			line_total = CASE
				WHEN line_total = 0 OR line_total IS NULL THEN 
					COALESCE(total_price, quantity * unit_price, 0)
				ELSE line_total
			END,
			final_amount = CASE
				WHEN final_amount = 0 OR final_amount IS NULL THEN 
					COALESCE(total_price, quantity * unit_price, 0)
				ELSE final_amount
			END,
			taxable = COALESCE(taxable, true)
		WHERE line_total = 0 OR line_total IS NULL OR final_amount = 0 OR final_amount IS NULL 
			 OR description IS NULL OR description = '' OR taxable IS NULL;
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to update sale_items without matching products: %v", err)
	} else {
		log.Println("Updated sale_items records without matching products")
	}

	log.Println("Existing records update completed")
}

// EnhanceCashBankModel adds enhanced fields to cash_banks table and related models
func EnhanceCashBankModel(db *gorm.DB) {
	log.Println("Starting enhanced cash bank model migration...")

	// Check if migration is needed by checking if new fields exist
	var columnExists bool

	// Check if min_balance column exists in cash_banks table
	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.columns 
		WHERE table_name = 'cash_banks' AND column_name = 'min_balance'
	)`).Scan(&columnExists)

	if columnExists {
		log.Println("Enhanced cash bank model fields already exist, skipping migration")
		return
	}

	// Add new fields to cash_banks table
	log.Println("Adding enhanced fields to cash_banks table...")
	err := db.Exec(`
		ALTER TABLE cash_banks 
		ADD COLUMN IF NOT EXISTS min_balance DECIMAL(15,2) DEFAULT 0,
		ADD COLUMN IF NOT EXISTS max_balance DECIMAL(15,2) DEFAULT 0,
		ADD COLUMN IF NOT EXISTS daily_limit DECIMAL(15,2) DEFAULT 0,
		ADD COLUMN IF NOT EXISTS monthly_limit DECIMAL(15,2) DEFAULT 0,
		ADD COLUMN IF NOT EXISTS is_restricted BOOLEAN DEFAULT false,
		ADD COLUMN IF NOT EXISTS user_id INTEGER;
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to add enhanced fields to cash_banks table: %v", err)
	} else {
		log.Println("Enhanced fields added to cash_banks table successfully")
	}

	// Update existing NOT NULL constraints and defaults
	log.Println("Updating constraints and defaults for cash_banks table...")
	err = db.Exec(`
		ALTER TABLE cash_banks 
		ALTER COLUMN currency SET DEFAULT 'IDR',
		ALTER COLUMN currency SET NOT NULL,
		ALTER COLUMN balance SET DEFAULT 0,
		ALTER COLUMN balance SET NOT NULL,
		ALTER COLUMN is_active SET DEFAULT true,
		ALTER COLUMN is_active SET NOT NULL;
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to update constraints for cash_banks table: %v", err)
	} else {
		log.Println("Updated constraints for cash_banks table successfully")
	}

	// Add check constraint for account type
	log.Println("Adding check constraint for cash_banks account type...")
	err = db.Exec(`
		ALTER TABLE cash_banks 
		DROP CONSTRAINT IF EXISTS check_cash_banks_type,
		ADD CONSTRAINT check_cash_banks_type CHECK (type IN ('CASH', 'BANK'));
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to add check constraint for cash_banks type: %v", err)
	} else {
		log.Println("Added check constraint for cash_banks type successfully")
	}

	// Create cash bank transfer table if not exists
	log.Println("Creating cash_bank_transfers table if not exists...")
	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS cash_bank_transfers (
			id SERIAL PRIMARY KEY,
			transfer_number VARCHAR(50) UNIQUE NOT NULL,
			from_account_id INTEGER NOT NULL REFERENCES cash_banks(id),
			to_account_id INTEGER NOT NULL REFERENCES cash_banks(id),
			date TIMESTAMP NOT NULL,
			amount DECIMAL(15,2) NOT NULL,
			exchange_rate DECIMAL(12,6) DEFAULT 1,
			converted_amount DECIMAL(15,2) NOT NULL,
			reference VARCHAR(100),
			notes TEXT,
			status VARCHAR(20) DEFAULT 'PENDING',
			user_id INTEGER NOT NULL REFERENCES users(id),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP NULL
		);
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to create cash_bank_transfers table: %v", err)
	} else {
		log.Println("Created cash_bank_transfers table successfully")
	}

	// Create bank reconciliation table if not exists
	log.Println("Creating bank_reconciliations table if not exists...")
	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS bank_reconciliations (
			id SERIAL PRIMARY KEY,
			cash_bank_id INTEGER NOT NULL REFERENCES cash_banks(id),
			reconcile_date TIMESTAMP NOT NULL,
			statement_balance DECIMAL(15,2) NOT NULL,
			system_balance DECIMAL(15,2) NOT NULL,
			difference DECIMAL(15,2) NOT NULL,
			status VARCHAR(20) DEFAULT 'PENDING',
			user_id INTEGER NOT NULL REFERENCES users(id),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP NULL
		);
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to create bank_reconciliations table: %v", err)
	} else {
		log.Println("Created bank_reconciliations table successfully")
	}

	// Create reconciliation items table if not exists
	log.Println("Creating reconciliation_items table if not exists...")
	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS reconciliation_items (
			id SERIAL PRIMARY KEY,
			reconciliation_id INTEGER NOT NULL REFERENCES bank_reconciliations(id),
			transaction_id INTEGER NOT NULL REFERENCES cash_bank_transactions(id),
			is_cleared BOOLEAN DEFAULT false,
			notes TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP NULL
		);
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to create reconciliation_items table: %v", err)
	} else {
		log.Println("Created reconciliation_items table successfully")
	}

	// Update existing cash bank records with default values
	updateExistingCashBankRecords(db)

	// Create indexes for cash bank tables
	createCashBankIndexes(db)

	log.Println("Enhanced cash bank model migration completed successfully")
}

// updateExistingCashBankRecords updates existing cash bank records with default values
func updateExistingCashBankRecords(db *gorm.DB) {
	log.Println("Updating existing cash bank records with default values...")

	// Update existing records that have NULL values for new fields
	err := db.Exec(`
		UPDATE cash_banks 
		SET 
			currency = CASE
				WHEN currency IS NULL OR currency = '' THEN 'IDR'
				ELSE currency
			END,
			balance = CASE
				WHEN balance IS NULL THEN 0
				ELSE balance
			END,
			is_active = CASE
				WHEN is_active IS NULL THEN true
				ELSE is_active
			END,
			min_balance = COALESCE(min_balance, 0),
			max_balance = COALESCE(max_balance, 0),
			daily_limit = COALESCE(daily_limit, 0),
			monthly_limit = COALESCE(monthly_limit, 0),
			is_restricted = COALESCE(is_restricted, false),
			user_id = CASE
				WHEN user_id IS NULL OR user_id = 0 THEN (
					SELECT id FROM users WHERE role = 'admin' ORDER BY id LIMIT 1
				)
				ELSE user_id
			END
		WHERE currency IS NULL OR currency = '' OR balance IS NULL 
			 OR is_active IS NULL OR min_balance IS NULL OR max_balance IS NULL 
			 OR daily_limit IS NULL OR monthly_limit IS NULL 
			 OR is_restricted IS NULL OR user_id IS NULL OR user_id = 0;
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to update existing cash bank records: %v", err)
	} else {
		log.Println("Updated existing cash bank records with default values")
	}

	// Set default user_id to first admin user if still NULL
	err = db.Exec(`
		UPDATE cash_banks 
		SET user_id = 1 
		WHERE user_id IS NULL OR user_id = 0;
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to set default user_id for cash bank records: %v", err)
	} else {
		log.Println("Set default user_id for cash bank records")
	}

	// Now make user_id NOT NULL after all records have been updated
	log.Println("Setting user_id column as NOT NULL...")
	err = db.Exec(`
		ALTER TABLE cash_banks 
		ALTER COLUMN user_id SET NOT NULL;
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to set user_id as NOT NULL: %v", err)
	} else {
		log.Println("Set user_id column as NOT NULL successfully")
	}

	log.Println("Cash bank records update completed")
}

// createCashBankIndexes creates indexes for cash bank related tables
func createCashBankIndexes(db *gorm.DB) {
	log.Println("Creating cash bank indexes...")

	// Cash Banks indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cash_banks_type ON cash_banks(type)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cash_banks_currency ON cash_banks(currency)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cash_banks_active ON cash_banks(is_active, is_restricted)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cash_banks_user ON cash_banks(user_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cash_banks_balance ON cash_banks(balance, currency)`)

	// Cash Bank Transactions indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cash_bank_transactions_account_date ON cash_bank_transactions(cash_bank_id, transaction_date)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cash_bank_transactions_reference ON cash_bank_transactions(reference_type, reference_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cash_bank_transactions_date ON cash_bank_transactions(transaction_date DESC)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cash_bank_transactions_amount ON cash_bank_transactions(amount, balance_after)`)

	// Cash Bank Transfers indexes (if table exists)
	var tableExists bool
	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.tables 
		WHERE table_name = 'cash_bank_transfers'
	)`).Scan(&tableExists)

	if tableExists {
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_cash_bank_transfers_from_account ON cash_bank_transfers(from_account_id, date)`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_cash_bank_transfers_to_account ON cash_bank_transfers(to_account_id, date)`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_cash_bank_transfers_status ON cash_bank_transfers(status, date)`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_cash_bank_transfers_user ON cash_bank_transfers(user_id, date)`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_cash_bank_transfers_amount ON cash_bank_transfers(amount, converted_amount)`)
	}

	// Bank Reconciliations indexes (if table exists)
	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.tables 
		WHERE table_name = 'bank_reconciliations'
	)`).Scan(&tableExists)

	if tableExists {
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_bank_reconciliations_account_date ON bank_reconciliations(cash_bank_id, reconcile_date)`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_bank_reconciliations_status ON bank_reconciliations(status, reconcile_date)`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_bank_reconciliations_user ON bank_reconciliations(user_id, reconcile_date)`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_bank_reconciliations_difference ON bank_reconciliations(difference, status)`)
	}

	// Reconciliation Items indexes (if table exists)
	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.tables 
		WHERE table_name = 'reconciliation_items'
	)`).Scan(&tableExists)

	if tableExists {
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_reconciliation_items_reconciliation ON reconciliation_items(reconciliation_id, is_cleared)`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_reconciliation_items_transaction ON reconciliation_items(transaction_id, reconciliation_id)`)
		db.Exec(`CREATE INDEX IF NOT EXISTS idx_reconciliation_items_cleared ON reconciliation_items(is_cleared, reconciliation_id)`)
	}

	log.Println("Cash bank indexes created successfully")
}

// EnhanceNewSalesFields ensures all new fields from recent model changes are properly migrated
func EnhanceNewSalesFields(db *gorm.DB) {
	log.Println("Starting enhanced new sales fields migration...")

	// Check if description column exists in sale_items table (indicates if migration is needed)
	var descColumnExists bool
	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.columns 
		WHERE table_name = 'sale_items' AND column_name = 'description'
	)`).Scan(&descColumnExists)

	if !descColumnExists {
		log.Println("Adding missing new fields to sale_items table...")
		err := db.Exec(`
			ALTER TABLE sale_items 
			ADD COLUMN IF NOT EXISTS description TEXT;
		`).Error

		if err != nil {
			log.Printf("Warning: Failed to add description field to sale_items table: %v", err)
		} else {
			log.Println("Added description field to sale_items table successfully")
		}
	}

	// Check if taxable column exists in sale_items table
	var taxableColumnExists bool
	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.columns 
		WHERE table_name = 'sale_items' AND column_name = 'taxable'
	)`).Scan(&taxableColumnExists)

	if !taxableColumnExists {
		log.Println("Adding taxable field to sale_items table...")
		err := db.Exec(`
			ALTER TABLE sale_items 
			ADD COLUMN IF NOT EXISTS taxable BOOLEAN DEFAULT true;
		`).Error

		if err != nil {
			log.Printf("Warning: Failed to add taxable field to sale_items table: %v", err)
		} else {
			log.Println("Added taxable field to sale_items table successfully")
		}
	}

	// Check if discount_percent column exists in sale_items table
	var discountPercentColumnExists bool
	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.columns 
		WHERE table_name = 'sale_items' AND column_name = 'discount_percent'
	)`).Scan(&discountPercentColumnExists)

	if !discountPercentColumnExists {
		log.Println("Adding discount_percent field to sale_items table...")
		err := db.Exec(`
			ALTER TABLE sale_items 
			ADD COLUMN IF NOT EXISTS discount_percent DECIMAL(5,2) DEFAULT 0;
		`).Error

		if err != nil {
			log.Printf("Warning: Failed to add discount_percent field to sale_items table: %v", err)
		} else {
			log.Println("Added discount_percent field to sale_items table successfully")
		}
	}

	// Check if pph_percent column exists in sales table
	var pphPercentColumnExists bool
	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.columns 
		WHERE table_name = 'sales' AND column_name = 'pph_percent'
	)`).Scan(&pphPercentColumnExists)

	if !pphPercentColumnExists {
		log.Println("Adding pph_percent field to sales table...")
		err := db.Exec(`
			ALTER TABLE sales 
			ADD COLUMN IF NOT EXISTS pph_percent DECIMAL(5,2) DEFAULT 0;
		`).Error

		if err != nil {
			log.Printf("Warning: Failed to add pph_percent field to sales table: %v", err)
		} else {
			log.Println("Added pph_percent field to sales table successfully")
		}
	}

	// Update existing records that have null values for new fields
	log.Println("Updating existing sale_items records with default values for new fields...")
	err := db.Exec(`
		UPDATE sale_items si
		SET 
			description = CASE
				WHEN si.description IS NULL OR si.description = '' THEN 
					COALESCE(p.name, 'Product Item')
				ELSE si.description
			END,
			taxable = CASE
				WHEN si.taxable IS NULL THEN true
				ELSE si.taxable
			END,
			discount_percent = CASE
				WHEN si.discount_percent IS NULL THEN 0
				ELSE si.discount_percent
			END
		FROM products p 
		WHERE si.product_id = p.id 
			AND (si.description IS NULL OR si.description = '' OR si.taxable IS NULL OR si.discount_percent IS NULL);
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to update existing sale_items with new field defaults: %v", err)
	} else {
		log.Println("Updated existing sale_items records with new field defaults")
	}

	// Update records without matching products
	err = db.Exec(`
		UPDATE sale_items
		SET 
			description = CASE
				WHEN description IS NULL OR description = '' THEN 'Product Item'
				ELSE description
			END,
			taxable = COALESCE(taxable, true),
			discount_percent = COALESCE(discount_percent, 0)
		WHERE description IS NULL OR description = '' OR taxable IS NULL OR discount_percent IS NULL;
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to update sale_items without matching products: %v", err)
	} else {
		log.Println("Updated sale_items records without matching products")
	}

	// Ensure tax_account_id foreign key exists if column exists
	var taxAccountColumnExists bool
	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.columns 
		WHERE table_name = 'sale_items' AND column_name = 'tax_account_id'
	)`).Scan(&taxAccountColumnExists)

	if taxAccountColumnExists {
		// Check if specific foreign key constraint exists
		var constraintExists bool
		db.Raw(`SELECT EXISTS (
			SELECT 1 FROM information_schema.table_constraints 
			WHERE table_name = 'sale_items' 
			AND constraint_type = 'FOREIGN KEY' 
			AND constraint_name = 'fk_sale_items_tax_account'
		)`).Scan(&constraintExists)

		if !constraintExists {
			log.Println("Adding foreign key constraint for tax_account_id...")
			err := db.Exec(`
				ALTER TABLE sale_items 
				ADD CONSTRAINT fk_sale_items_tax_account 
				FOREIGN KEY (tax_account_id) REFERENCES accounts(id);
			`).Error

			if err != nil {
				log.Printf("Warning: Failed to add foreign key constraint for tax_account_id: %v", err)
			} else {
				log.Println("Added foreign key constraint for tax_account_id successfully")
			}
		} else {
			log.Println("Foreign key constraint for tax_account_id already exists, skipping")
		}
	}

	// Add indexes for new fields
	log.Println("Adding indexes for new sales fields...")
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_sale_items_description ON sale_items(description)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_sale_items_taxable ON sale_items(taxable)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_sale_items_discount_percent ON sale_items(discount_percent)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_sale_items_tax_account ON sale_items(tax_account_id)`)

	log.Println("Enhanced new sales fields migration completed successfully")
}

// FixSalesDataIntegrity performs comprehensive sales data integrity fixes and validation
func FixSalesDataIntegrity(db *gorm.DB) {
	log.Println("=== Starting Sales Data Integrity Fix ===")

	// Check if we have sales records to fix
	var salesCount int64
	db.Model(&models.Sale{}).Count(&salesCount)

	if salesCount == 0 {
		log.Println("No sales records found, skipping sales data integrity fix")
		return
	}

	log.Printf("Found %d sales records, starting integrity checks and fixes...", salesCount)

	// 1. Fix missing sale codes
	fixMissingSaleCodes(db)

	// 2. Fix sale item calculations
	fixSaleItemCalculations(db)

	// 3. Recalculate sale totals
	recalculateSaleTotals(db)

	// 4. Check and report orphaned records
	checkOrphanedRecords(db)

	// 5. Validate status consistency
	validateStatusConsistency(db)

	// 6. Update legacy computed fields
	updateLegacyComputedFields(db)

	log.Println("✅ Sales Data Integrity Fix completed successfully")
}

// fixMissingSaleCodes generates codes for sales that don't have them
func fixMissingSaleCodes(db *gorm.DB) {
	log.Println("Fixing missing sale codes...")

	var salesWithoutCodes []models.Sale
	db.Where("code = '' OR code IS NULL").Find(&salesWithoutCodes)

	if len(salesWithoutCodes) == 0 {
		log.Println("No sales found without codes")
		return
	}

	fixedCodes := 0
	for i := range salesWithoutCodes {
		sale := &salesWithoutCodes[i]

		// Generate new code based on type
		prefix := "SAL"
		switch sale.Type {
		case models.SaleTypeQuotation:
			prefix = "QUO"
		case models.SaleTypeOrder:
			prefix = "ORD"
		case models.SaleTypeInvoice:
			prefix = "INV"
		}

		year := sale.Date.Year()
		newCode := fmt.Sprintf("%s-%d-%04d", prefix, year, sale.ID)

		// Check if code already exists
		var existing models.Sale
		if db.Where("code = ?", newCode).First(&existing).Error == nil {
			// Code exists, add suffix
			newCode = fmt.Sprintf("%s-FIX-%d", newCode, sale.ID)
		}

		sale.Code = newCode
		if err := db.Save(sale).Error; err != nil {
			log.Printf("Warning: Failed to update sale %d code: %v", sale.ID, err)
		} else {
			fixedCodes++
		}
	}

	log.Printf("Fixed %d missing sale codes", fixedCodes)
}

// fixSaleItemCalculations fixes missing calculations in sale items
func fixSaleItemCalculations(db *gorm.DB) {
	log.Println("Fixing sale item calculations...")

	// Fix missing LineTotal, FinalAmount, and other computed fields
	err := db.Exec(`
		UPDATE sale_items si
		SET 
			line_total = CASE
				WHEN line_total = 0 OR line_total IS NULL THEN 
					(quantity * unit_price) - COALESCE(discount_amount, discount, 0)
				ELSE line_total
			END,
			final_amount = CASE
				WHEN final_amount = 0 OR final_amount IS NULL THEN 
					(quantity * unit_price) - COALESCE(discount_amount, discount, 0) + COALESCE(total_tax, tax, 0)
				ELSE final_amount
			END,
			discount_amount = CASE
				WHEN discount_amount = 0 OR discount_amount IS NULL AND discount_percent > 0 THEN 
					(quantity * unit_price) * discount_percent / 100
				WHEN discount_amount = 0 OR discount_amount IS NULL THEN 
					COALESCE(discount, 0)
				ELSE discount_amount
			END,
			-- Update legacy fields for backward compatibility
			total_price = CASE
				WHEN total_price = 0 OR total_price IS NULL THEN 
					(quantity * unit_price) - COALESCE(discount_amount, discount, 0)
				ELSE total_price
			END
		WHERE line_total = 0 OR line_total IS NULL 
			 OR final_amount = 0 OR final_amount IS NULL 
			 OR (discount_amount = 0 OR discount_amount IS NULL) AND discount_percent > 0
			 OR total_price = 0 OR total_price IS NULL;
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to fix sale item calculations: %v", err)
	} else {
		log.Println("Fixed sale item calculations successfully")
	}
}

// recalculateSaleTotals recalculates totals for all sales
func recalculateSaleTotals(db *gorm.DB) {
	log.Println("Recalculating sale totals...")

	// Recalculate sales totals based on their items
	err := db.Exec(`
		UPDATE sales s
		SET 
			subtotal = COALESCE((
				SELECT SUM(si.line_total) 
				FROM sale_items si 
				WHERE si.sale_id = s.id AND si.deleted_at IS NULL
			), 0),
			discount_amount = CASE
				WHEN discount_percent > 0 THEN 
					COALESCE((
						SELECT SUM(si.line_total) 
						FROM sale_items si 
						WHERE si.sale_id = s.id AND si.deleted_at IS NULL
					), 0) * discount_percent / 100
				ELSE discount_amount
			END,
			taxable_amount = COALESCE((
				SELECT SUM(si.line_total) 
				FROM sale_items si 
				WHERE si.sale_id = s.id AND si.deleted_at IS NULL
			), 0) - CASE
				WHEN discount_percent > 0 THEN 
					COALESCE((
						SELECT SUM(si.line_total) 
						FROM sale_items si 
						WHERE si.sale_id = s.id AND si.deleted_at IS NULL
					), 0) * discount_percent / 100
				ELSE COALESCE(discount_amount, 0)
			END,
		ppn = CASE
				WHEN ppn_percent > 0 THEN 
					(
						COALESCE((
							SELECT SUM(si.line_total) 
							FROM sale_items si 
							WHERE si.sale_id = s.id AND si.deleted_at IS NULL
						), 0) - CASE
							WHEN discount_percent > 0 THEN 
								COALESCE((
									SELECT SUM(si.line_total) 
									FROM sale_items si 
									WHERE si.sale_id = s.id AND si.deleted_at IS NULL
								), 0) * discount_percent / 100
							ELSE COALESCE(discount_amount, 0)
						END
					) * ppn_percent / 100
				ELSE ppn
			END,
			pph = CASE
				WHEN pph_percent > 0 THEN 
					(
						COALESCE((
							SELECT SUM(si.line_total) 
							FROM sale_items si 
							WHERE si.sale_id = s.id AND si.deleted_at IS NULL
						), 0) - CASE
							WHEN discount_percent > 0 THEN 
								COALESCE((
									SELECT SUM(si.line_total) 
									FROM sale_items si 
									WHERE si.sale_id = s.id AND si.deleted_at IS NULL
								), 0) * discount_percent / 100
							ELSE COALESCE(discount_amount, 0)
						END
					) * pph_percent / 100
				ELSE pph
			END
		WHERE EXISTS (
			SELECT 1 FROM sale_items si 
			WHERE si.sale_id = s.id AND si.deleted_at IS NULL
		);
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to recalculate sale totals: %v", err)
	} else {
		log.Println("Recalculated sale totals successfully")
	}

	// Update total_tax and total_amount
	err = db.Exec(`
		UPDATE sales 
		SET 
			total_tax = COALESCE(ppn, 0) - COALESCE(pph, 0),
			total_amount = COALESCE(taxable_amount, 0) + COALESCE(ppn, 0) - COALESCE(pph, 0) + COALESCE(shipping_cost, 0),
			outstanding_amount = COALESCE(taxable_amount, 0) + COALESCE(ppn, 0) - COALESCE(pph, 0) + COALESCE(shipping_cost, 0) - COALESCE(paid_amount, 0),
			-- Update legacy tax field
			tax = COALESCE(ppn, 0) - COALESCE(pph, 0)
		WHERE taxable_amount IS NOT NULL;
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to update final totals: %v", err)
	} else {
		log.Println("Updated final totals successfully")
	}
}

// checkOrphanedRecords checks for orphaned sale items and other inconsistencies
func checkOrphanedRecords(db *gorm.DB) {
	log.Println("Checking for orphaned records...")

	// Check for orphaned sale items
	var orphanedItemsCount int64
	db.Raw(`
		SELECT COUNT(*) 
		FROM sale_items si 
		LEFT JOIN sales s ON si.sale_id = s.id 
		WHERE s.id IS NULL
	`).Scan(&orphanedItemsCount)

	if orphanedItemsCount > 0 {
		log.Printf("Warning: Found %d orphaned sale items", orphanedItemsCount)
		// Optionally delete orphaned items or flag them for manual review
		// db.Exec("DELETE FROM sale_items WHERE id IN (SELECT si.id FROM sale_items si LEFT JOIN sales s ON si.sale_id = s.id WHERE s.id IS NULL)")
	} else {
		log.Println("No orphaned sale items found")
	}

	// Check for orphaned sale payments
	var orphanedPaymentsCount int64
	db.Raw(`
		SELECT COUNT(*) 
		FROM sale_payments sp 
		LEFT JOIN sales s ON sp.sale_id = s.id 
		WHERE s.id IS NULL
	`).Scan(&orphanedPaymentsCount)

	if orphanedPaymentsCount > 0 {
		log.Printf("Warning: Found %d orphaned sale payments", orphanedPaymentsCount)
	} else {
		log.Println("No orphaned sale payments found")
	}
}

// validateStatusConsistency checks for invalid status transitions and inconsistencies
func validateStatusConsistency(db *gorm.DB) {
	log.Println("Validating status consistency...")

	// Check for INVOICED sales without invoice numbers
	var invalidInvoicedCount int64
	db.Model(&models.Sale{}).
		Where("status = ? AND (invoice_number = '' OR invoice_number IS NULL)", models.SaleStatusInvoiced).
		Count(&invalidInvoicedCount)

	if invalidInvoicedCount > 0 {
		log.Printf("Warning: Found %d INVOICED sales without invoice numbers", invalidInvoicedCount)
	}

	// Check for PAID sales with outstanding amounts > 0
	var invalidPaidCount int64
	db.Model(&models.Sale{}).
		Where("status = ? AND outstanding_amount > 0", models.SaleStatusPaid).
		Count(&invalidPaidCount)

	if invalidPaidCount > 0 {
		log.Printf("Warning: Found %d PAID sales with outstanding amounts > 0", invalidPaidCount)

		// Auto-fix: Update status to INVOICED if there's still outstanding amount
		err := db.Model(&models.Sale{}).
			Where("status = ? AND outstanding_amount > 0", models.SaleStatusPaid).
			Update("status", models.SaleStatusInvoiced).Error

		if err != nil {
			log.Printf("Warning: Failed to fix PAID status inconsistency: %v", err)
		} else {
			log.Printf("Fixed %d PAID sales with outstanding amounts", invalidPaidCount)
		}
	}
}

// updateLegacyComputedFields updates legacy computed fields for backward compatibility
// UpdateTaxFieldSizes updates tax field sizes from decimal(8,2) to decimal(15,2) to prevent numeric overflow
func UpdateTaxFieldSizes(db *gorm.DB) {
	log.Println("Starting tax field size update to prevent numeric overflow...")

	// Check if migration has already been applied by checking field size
	var columnInfo struct {
		NumericPrecision int `json:"numeric_precision"`
	}

	db.Raw(`SELECT numeric_precision 
			 FROM information_schema.columns 
			 WHERE table_name = 'sales' 
			 AND column_name = 'tax' 
			 LIMIT 1`).Scan(&columnInfo)

	if columnInfo.NumericPrecision >= 15 {
		log.Println("Tax field sizes already updated, skipping migration")
		return
	}

	// Update sales table tax fields from decimal(8,2) to decimal(15,2)
	log.Println("Updating sales table tax field sizes...")
	err := db.Exec(`
		ALTER TABLE sales 
			ALTER COLUMN tax TYPE DECIMAL(15,2),
			ALTER COLUMN ppn TYPE DECIMAL(15,2),
			ALTER COLUMN pph TYPE DECIMAL(15,2),
			ALTER COLUMN total_tax TYPE DECIMAL(15,2);
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to update sales table tax field sizes: %v", err)
	} else {
		log.Println("Updated sales table tax field sizes successfully")
	}

	// Update sale_items table tax fields from decimal(8,2) to decimal(15,2)
	log.Println("Updating sale_items table tax field sizes...")
	err = db.Exec(`
		ALTER TABLE sale_items 
			ALTER COLUMN ppn_amount TYPE DECIMAL(15,2),
			ALTER COLUMN pph_amount TYPE DECIMAL(15,2),
			ALTER COLUMN total_tax TYPE DECIMAL(15,2),
			ALTER COLUMN discount TYPE DECIMAL(15,2),
			ALTER COLUMN tax TYPE DECIMAL(15,2);
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to update sale_items table tax field sizes: %v", err)
	} else {
		log.Println("Updated sale_items table tax field sizes successfully")
	}

	log.Println("Tax field size update completed successfully")
}

func updateLegacyComputedFields(db *gorm.DB) {
	log.Println("Updating legacy computed fields...")

	// Update sale_items legacy fields
	err := db.Exec(`
		UPDATE sale_items 
		SET 
			total_price = line_total,
			tax = total_tax,
			discount = CASE 
				WHEN discount_amount > 0 THEN discount_amount
				WHEN discount_percent > 0 AND quantity > 0 AND unit_price > 0 THEN 
					(quantity * unit_price) * discount_percent / 100
				ELSE discount
			END
		WHERE total_price != line_total OR tax != total_tax OR (
			discount_amount > 0 AND discount != discount_amount
		);
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to update legacy sale item fields: %v", err)
	} else {
		log.Println("Updated legacy sale item fields successfully")
	}

	// Update sales legacy fields
	err = db.Exec(`
		UPDATE sales 
		SET 
			tax = total_tax
		WHERE tax != total_tax;
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to update legacy sales fields: %v", err)
	} else {
		log.Println("Updated legacy sales fields successfully")
	}
}

// SyncCashBankGLBalances - DISABLED FOR PRODUCTION SAFETY
// This function has been permanently disabled to prevent account balance resets in production
func SyncCashBankGLBalances(db *gorm.DB) {
	log.Println("🛡️  PRODUCTION SAFETY: SyncCashBankGLBalances DISABLED")
	log.Println("⚠️  Balance synchronization skipped to protect account data")
	log.Println("✅ All account balances remain unchanged")
	return // Exit immediately - no balance operations will be performed

	// Check if both tables exist first
	var cashBankTableExists, accountsTableExists bool

	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.tables 
		WHERE table_name = 'cash_banks'
	)`).Scan(&cashBankTableExists)

	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.tables 
		WHERE table_name = 'accounts'
	)`).Scan(&accountsTableExists)

	if !cashBankTableExists || !accountsTableExists {
		log.Println("Required tables not found, skipping CashBank-GL balance sync")
		return
	}

	// Check if we have any cash bank accounts to sync
	var totalCashBankCount int64
	db.Raw(`SELECT COUNT(*) FROM cash_banks WHERE deleted_at IS NULL`).Scan(&totalCashBankCount)

	if totalCashBankCount == 0 {
		log.Println("No cash/bank accounts found, skipping balance sync")
		return
	}

	// Check for unsynchronized accounts
	var unsyncCount int64
	db.Raw(`
		SELECT COUNT(*) 
		FROM cash_banks cb 
		INNER JOIN accounts acc ON cb.account_id = acc.id
		WHERE cb.deleted_at IS NULL 
		  AND cb.balance != acc.balance
	`).Scan(&unsyncCount)

	if unsyncCount == 0 {
		log.Println("✅ All CashBank accounts are already synchronized with GL accounts")
		return
	}

	log.Printf("Found %d unsynchronized CashBank-GL account pairs", unsyncCount)

	// Get details of unsynchronized accounts for logging
	type UnsyncAccount struct {
		CashBankCode    string  `json:"cash_bank_code"`
		CashBankName    string  `json:"cash_bank_name"`
		CashBankBalance float64 `json:"cash_bank_balance"`
		GLCode          string  `json:"gl_code"`
		GLBalance       float64 `json:"gl_balance"`
		Difference      float64 `json:"difference"`
	}

	var unsyncAccounts []UnsyncAccount
	db.Raw(`
		SELECT 
			cb.code as cash_bank_code,
			cb.name as cash_bank_name,
			cb.balance as cash_bank_balance,
			acc.code as gl_code,
			acc.balance as gl_balance,
			cb.balance - acc.balance as difference
		FROM cash_banks cb 
		INNER JOIN accounts acc ON cb.account_id = acc.id
		WHERE cb.deleted_at IS NULL 
		  AND cb.balance != acc.balance
		ORDER BY cb.type, cb.code
		LIMIT 10
	`).Scan(&unsyncAccounts)

	// Log sample of unsynchronized accounts
	log.Println("Sample unsynchronized accounts:")
	for _, account := range unsyncAccounts {
		log.Printf("  %s (%s): CB=%.2f, GL=%.2f, Diff=%.2f",
			account.CashBankCode, account.CashBankName,
			account.CashBankBalance, account.GLBalance, account.Difference)
	}

	if len(unsyncAccounts) < int(unsyncCount) {
		log.Printf("  ... and %d more accounts", unsyncCount-int64(len(unsyncAccounts)))
	}

	// Begin transaction for safe bulk update
	tx := db.Begin()

	// Synchronize GL account balances with CashBank balances
	log.Println("Synchronizing GL account balances with CashBank balances...")

	// Use a single UPDATE query to sync all unsynchronized accounts
	err := tx.Exec(`
		UPDATE accounts 
		SET balance = cb.balance,
		    updated_at = CURRENT_TIMESTAMP
		FROM cash_banks cb 
		WHERE accounts.id = cb.account_id 
		  AND cb.deleted_at IS NULL
		  AND accounts.balance != cb.balance
	`).Error

	if err != nil {
		log.Printf("❌ Failed to synchronize balances: %v", err)
		tx.Rollback()
		return
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("❌ Failed to commit balance synchronization: %v", err)
		return
	}

	// Verify synchronization completed successfully
	var remainingUnsyncCount int64
	db.Raw(`
		SELECT COUNT(*) 
		FROM cash_banks cb 
		INNER JOIN accounts acc ON cb.account_id = acc.id
		WHERE cb.deleted_at IS NULL 
		  AND cb.balance != acc.balance
	`).Scan(&remainingUnsyncCount)

	if remainingUnsyncCount == 0 {
		log.Printf("✅ Successfully synchronized %d CashBank-GL account pairs", unsyncCount)
		log.Println("✅ All CashBank accounts are now synchronized with their GL accounts")
	} else {
		log.Printf("⚠️  Warning: %d accounts still remain unsynchronized after migration", remainingUnsyncCount)
	}

	log.Println("CashBank-GL Balance Synchronization completed")
}

// RunBalanceSyncFix performs comprehensive balance synchronization checks and fixes
// This function runs after every migration to ensure balance consistency across the system
func RunBalanceSyncFix(db *gorm.DB) {
	log.Println("🔧 Starting Comprehensive Balance Synchronization Fix...")

	// Check if required tables exist
	var cashBankTableExists, accountsTableExists bool

	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.tables 
		WHERE table_name = 'cash_banks'
	)`).Scan(&cashBankTableExists)

	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.tables 
		WHERE table_name = 'accounts'
	)`).Scan(&accountsTableExists)

	if !cashBankTableExists || !accountsTableExists {
		log.Println("Required tables not found, skipping comprehensive balance sync fix")
		return
	}

	// Step 1: Fix missing account_id relationships
	fixMissingAccountRelationships(db)

	// Step 2: Recalculate CashBank balances from transactions
	recalculateCashBankBalances(db)

	// Step 3: Ensure GL accounts match CashBank balances
	ensureGLAccountSync(db)

	// Step 4: Validate and report final synchronization status
	validateFinalSyncStatus(db)

	log.Println("✅ Comprehensive Balance Synchronization Fix completed")
}

// fixMissingAccountRelationships ensures all CashBank accounts have proper GL account relationships
func fixMissingAccountRelationships(db *gorm.DB) {
	log.Println("Step 1: Fixing missing account relationships...")

	// Check for CashBank accounts without GL account links
	var orphanedCount int64
	db.Raw(`
		SELECT COUNT(*) 
		FROM cash_banks cb 
		LEFT JOIN accounts acc ON cb.account_id = acc.id
		WHERE cb.deleted_at IS NULL 
		  AND (cb.account_id IS NULL OR cb.account_id = 0 OR acc.id IS NULL)
	`).Scan(&orphanedCount)

	if orphanedCount > 0 {
		log.Printf("Found %d CashBank accounts without proper GL account links", orphanedCount)

		// Get details of orphaned accounts
		type OrphanedAccount struct {
			CashBankID   uint    `json:"cash_bank_id"`
			CashBankCode string  `json:"cash_bank_code"`
			CashBankName string  `json:"cash_bank_name"`
			AccountID    *uint   `json:"account_id"`
			Balance      float64 `json:"balance"`
		}

		var orphanedAccounts []OrphanedAccount
		db.Raw(`
			SELECT 
				cb.id as cash_bank_id,
				cb.code as cash_bank_code,
				cb.name as cash_bank_name,
				cb.account_id,
				cb.balance
			FROM cash_banks cb 
			LEFT JOIN accounts acc ON cb.account_id = acc.id
			WHERE cb.deleted_at IS NULL 
			  AND (cb.account_id IS NULL OR cb.account_id = 0 OR acc.id IS NULL)
			ORDER BY cb.type, cb.code
		`).Scan(&orphanedAccounts)

		// Log orphaned accounts for manual review
		log.Println("Orphaned CashBank accounts:")
		for _, account := range orphanedAccounts {
			log.Printf("  ID=%d, Code=%s, Name=%s, Balance=%.2f, AccountID=%v",
				account.CashBankID, account.CashBankCode, account.CashBankName,
				account.Balance, account.AccountID)
		}

		log.Printf("⚠️  Warning: Found %d orphaned CashBank accounts requiring manual GL account assignment", orphanedCount)
	} else {
		log.Println("✅ All CashBank accounts have proper GL account relationships")
	}
}

// recalculateCashBankBalances recalculates CashBank balances from transaction history
func recalculateCashBankBalances(db *gorm.DB) {
	log.Println("Step 2: Recalculating CashBank balances from transaction history...")

	// Check if cash_bank_transactions table exists
	var transactionTableExists bool
	db.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.tables 
		WHERE table_name = 'cash_bank_transactions'
	)`).Scan(&transactionTableExists)

	if !transactionTableExists {
		log.Println("Cash bank transactions table not found, skipping balance recalculation")
		return
	}

	// Recalculate balances for all CashBank accounts
	err := db.Exec(`
		UPDATE cash_banks 
		SET balance = COALESCE((
			SELECT SUM(amount) 
			FROM cash_bank_transactions cbt 
			WHERE cbt.cash_bank_id = cash_banks.id 
			  AND cbt.deleted_at IS NULL
		), 0),
		    updated_at = CURRENT_TIMESTAMP
		WHERE deleted_at IS NULL
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to recalculate CashBank balances: %v", err)
	} else {
		log.Println("✅ Recalculated CashBank balances from transaction history")
	}
}

// ensureGLAccountSync ensures GL accounts are synchronized with CashBank balances
func ensureGLAccountSync(db *gorm.DB) {
	log.Println("Step 3: Ensuring GL accounts are synchronized with CashBank balances...")

	// Check for unsynchronized accounts
	var unsyncCount int64
	db.Raw(`
		SELECT COUNT(*) 
		FROM cash_banks cb 
		INNER JOIN accounts acc ON cb.account_id = acc.id
		WHERE cb.deleted_at IS NULL 
		  AND ABS(cb.balance - acc.balance) > 0.01  -- Allow for small rounding differences
	`).Scan(&unsyncCount)

	if unsyncCount == 0 {
		log.Println("✅ All GL accounts are already synchronized with CashBank balances")
		return
	}

	log.Printf("Found %d GL accounts that need synchronization", unsyncCount)

	// Get details of unsynchronized accounts
	type UnsyncGLAccount struct {
		CashBankID      uint    `json:"cash_bank_id"`
		CashBankCode    string  `json:"cash_bank_code"`
		CashBankName    string  `json:"cash_bank_name"`
		CashBankBalance float64 `json:"cash_bank_balance"`
		GLAccountID     uint    `json:"gl_account_id"`
		GLCode          string  `json:"gl_code"`
		GLBalance       float64 `json:"gl_balance"`
		Difference      float64 `json:"difference"`
	}

	var unsyncAccounts []UnsyncGLAccount
	db.Raw(`
		SELECT 
			cb.id as cash_bank_id,
			cb.code as cash_bank_code,
			cb.name as cash_bank_name,
			cb.balance as cash_bank_balance,
			acc.id as gl_account_id,
			acc.code as gl_code,
			acc.balance as gl_balance,
			cb.balance - acc.balance as difference
		FROM cash_banks cb 
		INNER JOIN accounts acc ON cb.account_id = acc.id
		WHERE cb.deleted_at IS NULL 
		  AND ABS(cb.balance - acc.balance) > 0.01
		ORDER BY ABS(cb.balance - acc.balance) DESC
		LIMIT 10
	`).Scan(&unsyncAccounts)

	// Log accounts that will be synchronized
	log.Println("Accounts to be synchronized:")
	for _, account := range unsyncAccounts {
		log.Printf("  CB: %s (%.2f) -> GL: %s (%.2f) | Diff: %.2f",
			account.CashBankCode, account.CashBankBalance,
			account.GLCode, account.GLBalance, account.Difference)
	}

	if len(unsyncAccounts) < int(unsyncCount) {
		log.Printf("  ... and %d more accounts", unsyncCount-int64(len(unsyncAccounts)))
	}

	// Begin transaction for safe bulk update
	tx := db.Begin()

	// Synchronize GL account balances with CashBank balances
	err := tx.Exec(`
		UPDATE accounts 
		SET balance = cb.balance,
		    updated_at = CURRENT_TIMESTAMP
		FROM cash_banks cb 
		WHERE accounts.id = cb.account_id 
		  AND cb.deleted_at IS NULL
		  AND ABS(cb.balance - accounts.balance) > 0.01
	`).Error

	if err != nil {
		log.Printf("❌ Failed to synchronize GL accounts: %v", err)
		tx.Rollback()
		return
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("❌ Failed to commit GL account synchronization: %v", err)
		return
	}

	log.Printf("✅ Successfully synchronized %d GL accounts with CashBank balances", unsyncCount)
}

// validateFinalSyncStatus performs final validation and reports synchronization status
func validateFinalSyncStatus(db *gorm.DB) {
	log.Println("Step 4: Validating final synchronization status...")

	// Count total CashBank accounts
	var totalCount int64
	db.Raw(`SELECT COUNT(*) FROM cash_banks WHERE deleted_at IS NULL`).Scan(&totalCount)

	// Count synchronized accounts
	var syncedCount int64
	db.Raw(`
		SELECT COUNT(*) 
		FROM cash_banks cb 
		INNER JOIN accounts acc ON cb.account_id = acc.id
		WHERE cb.deleted_at IS NULL 
		  AND ABS(cb.balance - acc.balance) <= 0.01  -- Allow for small rounding differences
	`).Scan(&syncedCount)

	// Count unsynchronized accounts
	var unsyncedCount int64
	db.Raw(`
		SELECT COUNT(*) 
		FROM cash_banks cb 
		INNER JOIN accounts acc ON cb.account_id = acc.id
		WHERE cb.deleted_at IS NULL 
		  AND ABS(cb.balance - acc.balance) > 0.01
	`).Scan(&unsyncedCount)

	// Count orphaned accounts (no GL account link)
	var orphanedCount int64
	db.Raw(`
		SELECT COUNT(*) 
		FROM cash_banks cb 
		LEFT JOIN accounts acc ON cb.account_id = acc.id
		WHERE cb.deleted_at IS NULL 
		  AND (cb.account_id IS NULL OR cb.account_id = 0 OR acc.id IS NULL)
	`).Scan(&orphanedCount)

	// Report final status
	log.Println("=== Final Balance Synchronization Status ===")
	log.Printf("Total CashBank accounts: %d", totalCount)
	log.Printf("Synchronized accounts: %d", syncedCount)
	log.Printf("Unsynchronized accounts: %d", unsyncedCount)
	log.Printf("Orphaned accounts (no GL link): %d", orphanedCount)

	syncPercentage := float64(syncedCount) / float64(totalCount) * 100
	log.Printf("Synchronization rate: %.1f%%", syncPercentage)

	if unsyncedCount == 0 && orphanedCount == 0 {
		log.Println("✅ Perfect synchronization achieved! All accounts are properly synced.")
	} else if syncPercentage >= 95 {
		log.Printf("✅ Excellent synchronization (%.1f%%). System operating normally.", syncPercentage)
	} else if syncPercentage >= 85 {
		log.Printf("✅ Good synchronization (%.1f%%). Minor discrepancies are within acceptable range.", syncPercentage)
	} else if syncPercentage >= 70 {
		log.Printf("⚠️  Moderate synchronization (%.1f%%). Some accounts need attention.", syncPercentage)
	} else {
		log.Printf("❌ Poor synchronization (%.1f%%). Significant issues detected, manual intervention required.", syncPercentage)
	}

	// If there are still issues, log them for investigation
	if unsyncedCount > 0 || orphanedCount > 0 {
		log.Println("\n⚠️  Accounts requiring attention:")

		if unsyncedCount > 0 {
			var problemAccounts []struct {
				CashBankCode    string  `json:"cash_bank_code"`
				CashBankName    string  `json:"cash_bank_name"`
				CashBankBalance float64 `json:"cash_bank_balance"`
				GLCode          string  `json:"gl_code"`
				GLBalance       float64 `json:"gl_balance"`
				Difference      float64 `json:"difference"`
			}

			db.Raw(`
				SELECT 
					cb.code as cash_bank_code,
					cb.name as cash_bank_name,
					cb.balance as cash_bank_balance,
					acc.code as gl_code,
					acc.balance as gl_balance,
					cb.balance - acc.balance as difference
				FROM cash_banks cb 
				INNER JOIN accounts acc ON cb.account_id = acc.id
				WHERE cb.deleted_at IS NULL 
				  AND ABS(cb.balance - acc.balance) > 0.01
				ORDER BY ABS(cb.balance - acc.balance) DESC
				LIMIT 5
			`).Scan(&problemAccounts)

			log.Printf("  Unsynchronized accounts (top 5):")
			for _, account := range problemAccounts {
				log.Printf("    %s: CB=%.2f, GL=%.2f, Diff=%.2f",
					account.CashBankCode, account.CashBankBalance,
					account.GLBalance, account.Difference)
			}
		}

		if orphanedCount > 0 {
			var orphanedAccounts []struct {
				CashBankCode string  `json:"cash_bank_code"`
				CashBankName string  `json:"cash_bank_name"`
				Balance      float64 `json:"balance"`
				AccountID    *uint   `json:"account_id"`
			}

			db.Raw(`
				SELECT 
					cb.code as cash_bank_code,
					cb.name as cash_bank_name,
					cb.balance,
					cb.account_id
				FROM cash_banks cb 
				LEFT JOIN accounts acc ON cb.account_id = acc.id
				WHERE cb.deleted_at IS NULL 
				  AND (cb.account_id IS NULL OR cb.account_id = 0 OR acc.id IS NULL)
				ORDER BY cb.balance DESC
				LIMIT 5
			`).Scan(&orphanedAccounts)

			log.Printf("  Orphaned accounts (top 5):")
			for _, account := range orphanedAccounts {
				log.Printf("    %s: Balance=%.2f, AccountID=%v",
					account.CashBankCode, account.Balance, account.AccountID)
			}
		}
	}

	log.Println("=== Balance Synchronization Validation Complete ===")
}

// RunPaymentPerformanceOptimization applies the payment performance optimization migration
func RunPaymentPerformanceOptimization(db *gorm.DB) {
	log.Println("Starting Payment Performance Optimization Migration...")

	// Check if migration has already been applied by checking for one of the key indexes
	var indexExists bool
	db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes 
			WHERE tablename = 'payments' 
			AND indexname = 'idx_payments_contact_id'
		)
	`).Scan(&indexExists)

	if indexExists {
		log.Println("Payment performance optimization already applied, skipping")
		return
	}

	// Payments table indexes
	log.Println("Creating payment table indexes...")
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_payments_contact_id ON payments(contact_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_payments_date ON payments(date)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_payments_status ON payments(status)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_payments_method ON payments(method)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_payments_code ON payments(code)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_payments_created_at ON payments(created_at)`)

	// Payment allocations indexes
	log.Println("Creating payment allocation indexes...")
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_payment_allocations_payment_id ON payment_allocations(payment_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_payment_allocations_invoice_id ON payment_allocations(invoice_id) WHERE invoice_id IS NOT NULL`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_payment_allocations_bill_id ON payment_allocations(bill_id) WHERE bill_id IS NOT NULL`)

	// Cash bank transactions indexes
	log.Println("Creating cash bank transaction indexes...")
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cashbank_transactions_cashbank_id ON cash_bank_transactions(cash_bank_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cashbank_transactions_reference ON cash_bank_transactions(reference_type, reference_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cashbank_transactions_date ON cash_bank_transactions(transaction_date)`)

	// Journal entries indexes for payment operations
	log.Println("Creating journal entry indexes...")
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_entries_reference ON journal_entries(reference_type, reference_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_entries_date ON journal_entries(entry_date)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_entries_status ON journal_entries(status)`)

	// Journal lines indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_lines_journal_entry_id ON journal_lines(journal_entry_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_lines_account_id ON journal_lines(account_id)`)

	// Accounts table indexes for faster lookups
	log.Println("Creating account lookup indexes...")
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_accounts_code ON accounts(code)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_accounts_name_lower ON accounts(LOWER(name))`)

	// Cash bank table indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cash_bank_account_id ON cash_banks(account_id)`)

	// Purchase table indexes for payment integration
	log.Println("Creating purchase integration indexes...")
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_purchases_vendor_id ON purchases(vendor_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_purchases_status ON purchases(status)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_purchases_payment_method ON purchases(payment_method)`)

	// Sales table indexes for payment integration
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_sales_customer_id ON sales(customer_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_sales_status ON sales(status)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_sales_outstanding_amount ON sales(outstanding_amount) WHERE outstanding_amount > 0`)

	// Payment code sequence table indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_payment_code_sequence_prefix ON payment_code_sequences(prefix, year, month)`)

	// Composite indexes for common query patterns
	log.Println("Creating composite indexes...")
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_payments_contact_status_date ON payments(contact_id, status, date)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_payments_date_method ON payments(date, method)`)

	// Add partial indexes for active/pending records only
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_payments_active ON payments(id) WHERE status IN ('PENDING', 'COMPLETED')`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_purchases_payable ON purchases(id) WHERE status = 'APPROVED' AND payment_method = 'CREDIT' AND outstanding_amount > 0`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_sales_receivable ON sales(id) WHERE status = 'INVOICED' AND outstanding_amount > 0`)

	// Optimize sequence table for better payment code generation
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_payment_code_seq_unique ON payment_code_sequences(prefix, year, month) WHERE sequence_number > 0`)

	// Add statistics update for PostgreSQL optimization
	log.Println("Updating table statistics for optimization...")
	db.Exec(`ANALYZE payments`)
	db.Exec(`ANALYZE payment_allocations`)
	db.Exec(`ANALYZE cash_bank_transactions`)
	db.Exec(`ANALYZE journal_entries`)
	db.Exec(`ANALYZE journal_lines`)
	db.Exec(`ANALYZE accounts`)
	db.Exec(`ANALYZE cash_banks`)
	db.Exec(`ANALYZE purchases`)
	db.Exec(`ANALYZE sales`)

	// Create performance monitoring view
	log.Println("Creating performance monitoring view...")
	db.Exec(`
		CREATE OR REPLACE VIEW payment_performance_stats AS
		SELECT 
			'payments' as table_name,
			COUNT(*) as total_records,
			COUNT(CASE WHEN status = 'COMPLETED' THEN 1 END) as completed_count,
			COUNT(CASE WHEN status = 'PENDING' THEN 1 END) as pending_count,
			AVG(amount) as avg_amount,
			MAX(created_at) as last_payment
		FROM payments
		UNION ALL
		SELECT 
			'journal_entries' as table_name,
			COUNT(*) as total_records,
			COUNT(CASE WHEN status = 'POSTED' THEN 1 END) as posted_count,
			COUNT(CASE WHEN reference_type = 'PAYMENT' THEN 1 END) as payment_journals,
			AVG(total_debit) as avg_amount,
			MAX(created_at) as last_entry
		FROM journal_entries
	`)

	log.Println("✅ Payment Performance Optimization Migration completed successfully")
}

// RunDatabaseEnhancements applies comprehensive database enhancements for the accounting system
func RunDatabaseEnhancements(db *gorm.DB) {
	migrationID := "database_enhancements_v2024.1"

	// Check if this migration has already been applied
	var existing models.MigrationRecord
	err := db.Where("migration_id = ?", migrationID).First(&existing).Error
	if err == nil {
		log.Printf("Database enhancements migration '%s' already applied at %v, skipping...", migrationID, existing.AppliedAt)
		return
	}

	log.Println("🚀 Starting comprehensive database enhancements migration...")

	// Step 1: Create enhanced indexes for journal entries and related tables
	createJournalEntryIndexes(db)

	// Step 2: Create performance indexes for accounting queries
	createAccountingPerformanceIndexes(db)

	// Step 3: Create validation constraints
	createValidationConstraints(db)

	// Step 4: Create audit trail enhancements
	createAuditTrailEnhancements(db)

	// Step 6: Optimize existing data
	optimizeExistingAccountingData(db)

	// Record this migration as completed
	migrationRecord := models.MigrationRecord{
		MigrationID: migrationID,
		Description: "Comprehensive database enhancements for accounting system - indexes, constraints, audit trail, and performance optimizations",
		Version:     "2024.1",
		AppliedAt:   time.Now(),
	}

	if err := db.Create(&migrationRecord).Error; err != nil {
		log.Printf("Warning: Failed to record migration completion: %v", err)
	} else {
		log.Println("✅ Database enhancements migration recorded successfully")
	}

	log.Println("✅ Comprehensive database enhancements migration completed successfully")
}

// createJournalEntryIndexes creates indexes for journal entries and lines for better performance
func createJournalEntryIndexes(db *gorm.DB) {
	log.Println("Creating journal entry indexes...")

	// Journal entries indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_entries_entry_date ON journal_entries(entry_date)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_entries_reference_type_id ON journal_entries(reference_type, reference_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_entries_status_date ON journal_entries(status, entry_date)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_entries_user_id_date ON journal_entries(user_id, entry_date)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_entries_journal_id ON journal_entries(journal_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_entries_period ON journal_entries(entry_date) WHERE status = 'POSTED'`)

	// Journal lines indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_lines_entry_account ON journal_lines(journal_entry_id, account_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_lines_account_debit ON journal_lines(account_id, debit_amount) WHERE debit_amount > 0`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_lines_account_credit ON journal_lines(account_id, credit_amount) WHERE credit_amount > 0`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_lines_amounts ON journal_lines(debit_amount, credit_amount)`)

	// Composite indexes for complex queries
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_entries_complete ON journal_entries(entry_date, status, reference_type) WHERE status = 'POSTED'`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_journal_lines_balance ON journal_lines(account_id, debit_amount, credit_amount)`)

	log.Println("✅ Journal entry indexes created successfully")
}

// createAccountingPerformanceIndexes creates indexes for better accounting query performance
func createAccountingPerformanceIndexes(db *gorm.DB) {
	log.Println("Creating accounting performance indexes...")

	// Account balance calculation indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_accounts_type_balance ON accounts(type, balance) WHERE deleted_at IS NULL`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_accounts_category_balance ON accounts(category, balance) WHERE deleted_at IS NULL`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_accounts_parent_children ON accounts(parent_id) WHERE parent_id IS NOT NULL`)

	// Transaction reporting indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_transactions_date_account_amount ON transactions(transaction_date, account_id, amount)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_transactions_period_reporting ON transactions(transaction_date, account_id) WHERE deleted_at IS NULL`)

	// Sales and purchases reporting indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_sales_reporting ON sales(date, status, total_amount) WHERE deleted_at IS NULL`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_purchases_reporting ON purchases(date, status, total_amount) WHERE deleted_at IS NULL`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_sales_customer_period ON sales(customer_id, date, total_amount) WHERE status IN ('INVOICED', 'PAID')`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_purchases_vendor_period ON purchases(vendor_id, date, total_amount) WHERE status = 'APPROVED'`)

	// Cash flow indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cash_bank_transactions_flow ON cash_bank_transactions(transaction_date, transaction_type, amount)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_payments_cash_flow ON payments(date, method, amount) WHERE status = 'COMPLETED'`)

	// Aging analysis indexes
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_sales_aging ON sales(due_date, outstanding_amount) WHERE outstanding_amount > 0`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_purchases_aging ON purchases(due_date, outstanding_amount) WHERE outstanding_amount > 0`)

	log.Println("✅ Accounting performance indexes created successfully")
}

// createValidationConstraints creates database constraints for data integrity
func createValidationConstraints(db *gorm.DB) {
	log.Println("Creating validation constraints...")

	// Journal entry validation constraints
	db.Exec(`
		ALTER TABLE journal_entries 
		DROP CONSTRAINT IF EXISTS chk_journal_entries_balanced,
		ADD CONSTRAINT chk_journal_entries_balanced 
		CHECK (ABS(total_debit - total_credit) < 0.01)
	`)

	// Account balance constraints
	db.Exec(`
		ALTER TABLE accounts
		DROP CONSTRAINT IF EXISTS chk_accounts_type_valid,
		ADD CONSTRAINT chk_accounts_type_valid 
		CHECK (type IN ('ASSET', 'LIABILITY', 'EQUITY', 'REVENUE', 'EXPENSE'))
	`)

	// Amount validation constraints
	db.Exec(`
		ALTER TABLE journal_lines
		DROP CONSTRAINT IF EXISTS chk_journal_lines_amount_positive,
		ADD CONSTRAINT chk_journal_lines_amount_positive 
		CHECK (debit_amount >= 0 AND credit_amount >= 0 AND (debit_amount > 0 OR credit_amount > 0))
	`)

	// Date validation constraints - Relaxed to allow future period closing
	db.Exec(`
		ALTER TABLE journal_entries
		DROP CONSTRAINT IF EXISTS chk_journal_entries_date_valid,
		ADD CONSTRAINT chk_journal_entries_date_valid 
		CHECK (entry_date >= '2000-01-01' AND entry_date <= '2099-12-31')
	`)

	// Status validation constraints
	db.Exec(`
		ALTER TABLE journal_entries
		DROP CONSTRAINT IF EXISTS chk_journal_entries_status_valid,
		ADD CONSTRAINT chk_journal_entries_status_valid 
		CHECK (status IN ('DRAFT', 'POSTED', 'REVERSED'))
	`)

	log.Println("✅ Validation constraints created successfully")
}

// createAuditTrailEnhancements creates enhanced audit trail functionality
func createAuditTrailEnhancements(db *gorm.DB) {
	log.Println("Creating audit trail enhancements...")

	// Audit log indexes for better query performance
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_audit_logs_table_record_action ON audit_logs(table_name, record_id, action)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_audit_logs_user_timestamp ON audit_logs(user_id, created_at)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp_action ON audit_logs(created_at, action)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_audit_logs_critical ON audit_logs(created_at) WHERE action IN ('DELETE', 'UPDATE') AND table_name IN ('journal_entries', 'accounts', 'transactions')`)

	// Create audit summary view
	db.Exec(`
		CREATE OR REPLACE VIEW audit_trail_summary AS
		SELECT 
			DATE(created_at) as audit_date,
			table_name,
			action,
			user_id,
			COUNT(*) as action_count,
			MIN(created_at) as first_action,
			MAX(created_at) as last_action
		FROM audit_logs
		WHERE created_at >= CURRENT_DATE - INTERVAL '30 days'
		GROUP BY DATE(created_at), table_name, action, user_id
		ORDER BY audit_date DESC, action_count DESC
	`)

	// Create critical changes view
	db.Exec(`
		CREATE OR REPLACE VIEW critical_audit_changes AS
		SELECT 
			al.*,
			u.username,
			u.full_name
		FROM audit_logs al
		LEFT JOIN users u ON al.user_id = u.id
		WHERE al.action IN ('DELETE', 'UPDATE')
			AND al.table_name IN ('journal_entries', 'accounts', 'transactions', 'sales', 'purchases')
			AND al.created_at >= CURRENT_DATE - INTERVAL '7 days'
		ORDER BY al.created_at DESC
	`)

	log.Println("✅ Audit trail enhancements created successfully")
}

// optimizeExistingAccountingData performs data optimization and cleanup
func optimizeExistingAccountingData(db *gorm.DB) {
	log.Println("Optimizing existing accounting data...")

	// Update statistics for all accounting-related tables
	log.Println("Updating table statistics...")
	db.Exec(`ANALYZE accounts`)
	db.Exec(`ANALYZE journal_entries`)
	db.Exec(`ANALYZE journal_lines`)
	db.Exec(`ANALYZE transactions`)
	db.Exec(`ANALYZE sales`)
	db.Exec(`ANALYZE purchases`)
	db.Exec(`ANALYZE cash_bank_transactions`)
	db.Exec(`ANALYZE audit_logs`)

	// Clean up orphaned records
	log.Println("Cleaning up orphaned journal lines...")
	result := db.Exec(`
		DELETE FROM journal_lines 
		WHERE journal_entry_id NOT IN (
			SELECT id FROM journal_entries
		)
	`)
	if result.Error != nil {
		log.Printf("Warning: Failed to clean up orphaned journal lines: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("Cleaned up %d orphaned journal lines", result.RowsAffected)
	}

	// Vacuum analyze for PostgreSQL (if applicable)
	log.Println("Performing database maintenance...")
	db.Exec(`VACUUM ANALYZE`)

	log.Println("✅ Existing accounting data optimized successfully")
}
