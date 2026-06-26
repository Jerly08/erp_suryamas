package database

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

// InitializeDatabase runs migrations and seeds initial data
func InitializeDatabase(db *gorm.DB) {
	log.Println("🛡️  PRODUCTION DATABASE INITIALIZATION - ALL BALANCE SYNC OPERATIONS DISABLED")
	log.Println("✅ Account balances are protected from automatic modifications")
	log.Println("Initializing database...")

	// Run GORM auto migrations for basic models
	RunMigrations(db)

	// Run SQL-based auto migrations (including SSOT)
	if err := RunAutoMigrations(db); err != nil {
		log.Printf("⚠️  Auto migration warning: %v", err)
		// Continue with initialization even if auto migration fails
	}

	if err := EnsureStockAlertNotificationUniqueness(db); err != nil {
		log.Printf("âš ï¸  Stock alert uniqueness warning: %v", err)
	}

	// Seed initial data
	SeedData(db)

	// Align critical profit-loss COA categories and dedicated non-operating accounts
	ProfitLossCOAAlignmentMigration(db)

	// Prevent duplicate depreciation posting when scheduler and manual runs overlap
	DepreciationJournalGuardMigration(db)

	// NOTE: UpdateExistingPurchaseData removed — it was resetting outstanding_amount
	// for paid purchases every startup, corrupting payment data

	// Note: Balance sync migration is now handled in database.go after environment variable checks
	// to prevent balance resets during development

	// Fix concurrent refresh error (CRITICAL)
	FixConcurrentRefreshError(db)

	// Run auto-fix migration for common issues (after git pull)
	AutoFixMigration(db)

	// Backfill finalized PR budget totals to exclude rejected item rows
	PurchaseRequestBudgetTotalMigration(db)

	// Fix sales balance issues
	SalesBalanceFixMigration(db)

	// Fix product image issues
	ProductImageFixMigration(db)

	// Fix cash bank constraint issues
	CashBankConstraintMigration(db)

	// Run notification system migrations
	MigrateNotificationConfig(db)

	// Clean up duplicate notifications
	CleanupDuplicateNotificationsMigration(db)

	// Add description column to accounting_periods table
	if err := AddAccountingPeriodDescription(db); err != nil {
		log.Printf("⚠️  Accounting period description migration warning: %v", err)
	}

	// Fix accounting_periods table structure (make year/month nullable)
	if err := FixAccountingPeriodsStructure(db); err != nil {
		log.Printf("⚠️  Accounting period structure fix warning: %v", err)
	}

	// Fix journal entry date constraint for period closing
	FixJournalEntryDateConstraintMigration(db)

	// NOTE: ForceRunDateConstraintFix removed — already applied, no need to run every startup

	log.Println("Database initialization completed")
}

// RunMigrations creates all tables based on models
func RunMigrations(db *gorm.DB) {
	log.Println("Running database migrations...")

	err := db.AutoMigrate(
		// Core models
		&models.User{},
		&models.CompanyProfile{},

		// Auth models
		&models.RefreshToken{},
		&models.UserSession{},
		&models.BlacklistedToken{},
		&models.AuthAttempt{},
		&models.RateLimitRecord{},
		&models.Permission{},
		&models.RolePermission{},
		&models.ModulePermissionRecord{},

		// Approval models
		&models.ApprovalWorkflow{},
		&models.ApprovalStep{},
		&models.ApprovalRequest{},
		&models.ApprovalAction{},
		&models.ApprovalHistory{},

		// Accounting models
		&models.Account{},
		&models.Transaction{},
		&models.Journal{},
		&models.JournalEntry{},

		// Product & Inventory models
		&models.ProductCategory{},
		&models.ProductUnit{},
		&models.Product{},
		&models.Inventory{},

		// Contact models
		&models.Contact{},
		&models.ContactAddress{},
		&models.ContactHistory{},
		&models.CommunicationLog{},

		// Sales & Purchase models
		&models.Sale{},
		&models.SaleItem{},
		&models.SalePayment{},
		&models.SaleReturn{},
		&models.SaleReturnItem{},
		&models.Purchase{},
		&models.PurchaseItem{},

		// Payment & Cash Bank models
		&models.Payment{},
		&models.PaymentAllocation{},
		&models.CashBank{},
		&models.CashBankTransaction{},
		&models.PurchasePayment{},

		// Payroll models
		&models.PayrollRun{},
		&models.PayrollItem{},
		&models.PayrollSettlement{},

		// Expense models
		&models.ExpenseCategory{},
		&models.Expense{},

		// Asset models
		&models.AssetCategory{},
		&models.Asset{},

		// Budget models
		&models.Budget{},
		&models.BudgetItem{},
		&models.BudgetComparison{},

		// Report models
		&models.Report{},
		&models.ReportTemplate{},
		&models.FinancialRatio{},
		&models.AccountPeriodBalance{},

		// Audit models
		&models.AuditLog{},

		// Security models
		&models.SecurityIncident{},
		&models.SystemAlert{},
		&models.RequestLog{},
		&models.IpWhitelist{},
		&models.SecurityConfig{},
		&models.SecurityMetrics{},

		// Migration tracking
		&models.MigrationRecord{},

		// Notification models
		&models.Notification{},
		&models.StockAlert{},
	)

	if err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}

	BackfillPayrollBPJSSettlementStatus(db)
	BackfillPurchasePaymentLinks(db)

	log.Println("Database migrations completed successfully")

}

// CreateIndexes creates additional database indexes for performance optimization
func CreateIndexes(db *gorm.DB) {
	log.Println("Creating additional database indexes...")

	// Account indexes
	db.Exec("CREATE INDEX IF NOT EXISTS idx_accounts_type_category ON accounts(type, category)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_accounts_parent_level ON accounts(parent_id, level)")

	// Transaction indexes
	db.Exec("CREATE INDEX IF NOT EXISTS idx_transactions_date_account ON transactions(transaction_date, account_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_transactions_reference ON transactions(reference_type, reference_id)")

	// Journal indexes
	db.Exec("CREATE INDEX IF NOT EXISTS idx_journals_period_status ON journals(period, status)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_journal_entries_amounts ON journal_entries(debit_amount, credit_amount)")

	// Sales & Purchase indexes
	db.Exec("CREATE INDEX IF NOT EXISTS idx_sales_date_customer ON sales(date, customer_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_purchases_date_vendor ON purchases(date, vendor_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_sale_items_product ON sale_items(product_id, sale_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_purchase_items_product ON purchase_items(product_id, purchase_id)")

	// Inventory indexes
	db.Exec("CREATE INDEX IF NOT EXISTS idx_inventory_product_date ON inventories(product_id, transaction_date)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_inventory_reference ON inventories(reference_type, reference_id)")

	// Contact indexes
	db.Exec("CREATE INDEX IF NOT EXISTS idx_contacts_type_category ON contacts(type, category)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_contact_addresses_type_default ON contact_addresses(contact_id, type, is_default)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_contact_history_contact_user ON contact_histories(contact_id, user_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_contact_history_action_date ON contact_histories(action, created_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_communication_logs_contact_type ON communication_logs(contact_id, type)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_communication_logs_status_date ON communication_logs(status, created_at)")

	// Payment indexes
	db.Exec("CREATE INDEX IF NOT EXISTS idx_payments_date_contact ON payments(date, contact_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_cash_bank_transactions_date ON cash_bank_transactions(transaction_date, cash_bank_id)")

	// Expense indexes
	db.Exec("CREATE INDEX IF NOT EXISTS idx_expenses_date_category ON expenses(date, category_id)")

	// Budget indexes
	db.Exec("CREATE INDEX IF NOT EXISTS idx_budget_items_budget_month ON budget_items(budget_id, month)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_budget_items_account ON budget_items(account_id, budget_id)")

	// Report indexes
	db.Exec("CREATE INDEX IF NOT EXISTS idx_reports_type_period ON reports(type, period)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_account_balances_period ON account_balances(period, account_id)")

	// Audit Log indexes
	db.Exec("CREATE INDEX IF NOT EXISTS idx_audit_logs_table_record ON audit_logs(table_name, record_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_audit_logs_user_action ON audit_logs(user_id, action)")

	// Security indexes
	db.Exec("CREATE INDEX IF NOT EXISTS idx_security_incidents_type_severity ON security_incidents(incident_type, severity)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_security_incidents_client_ip ON security_incidents(client_ip, created_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_security_incidents_resolved ON security_incidents(resolved, created_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_system_alerts_type_level ON system_alerts(alert_type, level)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_system_alerts_acknowledged ON system_alerts(acknowledged, created_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_request_logs_client_ip ON request_logs(client_ip, timestamp)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_request_logs_suspicious ON request_logs(is_suspicious, timestamp)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_request_logs_path_method ON request_logs(path, method, timestamp)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_ip_whitelist_environment ON ip_whitelists(environment, is_active)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_security_config_key_env ON security_configs(key, environment)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_security_metrics_date ON security_metrics(date)")

	log.Println("Additional database indexes created successfully")
}

// CleanupDuplicateNotificationsMigration removes duplicate notifications
func CleanupDuplicateNotificationsMigration(db *gorm.DB) {
	migrationID := "cleanup_duplicate_notifications_v1.0"

	// Check if this migration has already been applied
	var existing models.MigrationRecord
	err := db.Where("migration_id = ?", migrationID).First(&existing).Error
	if err == nil {
		log.Printf("Migration '%s' already applied at %v, skipping...", migrationID, existing.AppliedAt)
		return
	}

	log.Println("Running duplicate notification cleanup migration...")

	// Find and remove duplicate notifications
	var duplicates []struct {
		UserID     uint
		Type       string
		PurchaseID string
		Count      int64
	}

	// Find duplicates
	err = db.Raw(`
		SELECT 
			user_id,
			type,
			data::json->>'purchase_id' as purchase_id,
			COUNT(*) as count
		FROM notifications 
		WHERE data::json->>'purchase_id' IS NOT NULL
		AND created_at >= NOW() - INTERVAL '7 days'
		GROUP BY user_id, type, data::json->>'purchase_id'
		HAVING COUNT(*) > 1
	`).Scan(&duplicates).Error

	if err != nil {
		log.Printf("Error finding duplicate notifications: %v", err)
		return
	}

	totalRemoved := 0
	for _, dup := range duplicates {
		if dup.PurchaseID == "" {
			continue
		}

		// Keep the latest notification, remove older ones
		var notifications []models.Notification
		err := db.Where("user_id = ? AND type = ? AND data::json->>'purchase_id' = ?",
			dup.UserID, dup.Type, dup.PurchaseID).
			Order("created_at DESC").
			Find(&notifications).Error

		if err != nil {
			log.Printf("Error finding notifications for cleanup: %v", err)
			continue
		}

		// Keep first (latest), remove the rest
		if len(notifications) > 1 {
			var idsToDelete []uint
			for i := 1; i < len(notifications); i++ {
				idsToDelete = append(idsToDelete, notifications[i].ID)
			}

			if len(idsToDelete) > 0 {
				err := db.Where("id IN ?", idsToDelete).Delete(&models.Notification{}).Error
				if err != nil {
					log.Printf("Error deleting duplicate notifications: %v", err)
					continue
				}

				removed := len(idsToDelete)
				totalRemoved += removed
				log.Printf("✅ Removed %d duplicate notifications for user %d, type %s, purchase %s",
					removed, dup.UserID, dup.Type, dup.PurchaseID)
			}
		}
	}

	// Also cleanup by title-based duplicates (for notifications without purchase_id)
	titleDuplicates := 0
	err = db.Raw(`
		SELECT COUNT(*) FROM (
			SELECT user_id, type, title, COUNT(*) as cnt
			FROM notifications 
			WHERE created_at >= NOW() - INTERVAL '7 days'
			GROUP BY user_id, type, title
			HAVING COUNT(*) > 1
		) as title_dups
	`).Scan(&titleDuplicates).Error

	if err == nil && titleDuplicates > 0 {
		// Remove title-based duplicates
		result := db.Exec(`
			DELETE FROM notifications 
			WHERE id NOT IN (
				SELECT DISTINCT ON (user_id, type, title) id
				FROM notifications 
				WHERE created_at >= NOW() - INTERVAL '7 days'
				ORDER BY user_id, type, title, created_at DESC
			)
			AND created_at >= NOW() - INTERVAL '7 days'
		`)

		if result.Error == nil {
			totalRemoved += int(result.RowsAffected)
			log.Printf("✅ Removed %d title-based duplicate notifications", result.RowsAffected)
		}
	}

	// Record this migration as completed
	migrationRecord := models.MigrationRecord{
		MigrationID: migrationID,
		Description: fmt.Sprintf("Cleanup duplicate notifications - removed %d duplicates", totalRemoved),
		Version:     "1.0",
		AppliedAt:   time.Now(),
	}

	if err := db.Create(&migrationRecord).Error; err != nil {
		log.Printf("Warning: Failed to record migration completion: %v", err)
	}

	log.Printf("✅ Duplicate notification cleanup migration completed - removed %d total duplicates", totalRemoved)
}
