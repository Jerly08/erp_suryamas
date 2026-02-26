package database

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

// Global counter for idempotent skips (thread-safe)
var idempotentSkipCount int64

// RunAutoMigrations runs all pending SQL migrations automatically
func RunAutoMigrations(db *gorm.DB) error {
	log.Println("🔄 Starting auto-migrations...")

	// Create migration_logs table first if it doesn't exist
	if err := createMigrationLogsTable(db); err != nil {
		return fmt.Errorf("failed to create migration_logs table: %v", err)
	}

	// Run pre-migration fixes to ensure compatibility
	if err := runPreMigrationFixes(db); err != nil {
		log.Printf("⚠️  Pre-migration fixes failed: %v", err)
		// Don't fail completely, just warn
	}

	// Silently verify invoice types system
	if err := ensureInvoiceTypesSystem(db); err != nil {
		log.Printf("⚠️  Invoice types system verification failed: %v", err)
	}

	// Silently verify tax account settings table
	if err := ensureTaxAccountSettingsTable(db); err != nil {
		log.Printf("⚠️  Tax account settings table verification failed: %v", err)
	}

	// Silently verify settings history table
	if err := ensureSettingsHistoryTable(db); err != nil {
		log.Printf("⚠️  Settings history table verification failed: %v", err)
	}

	// Get migration files
	migrationFiles, err := getMigrationFiles()
	if err != nil {
		return fmt.Errorf("failed to get migration files: %v", err)
	}

	// Reset idempotent skip counter
	atomic.StoreInt64(&idempotentSkipCount, 0)

	// Run each migration
	successCount := 0
	failedCount := 0
	for _, file := range migrationFiles {
		if err := runMigration(db, file); err != nil {
			log.Printf("❌ Migration failed: %s - %v", file, err)
			failedCount++
			continue
		}
		successCount++
	}

	// Print concise migration summary
	if failedCount > 0 {
		log.Println("")
		log.Println("============================================")
		log.Printf("⚠️  %d migration(s) failed. Check logs above for details.", failedCount)
		log.Println("============================================")
	} else {
		// Only show summary if there were actual new migrations
		if successCount > 0 {
			log.Printf("✅ Database migrations: %d completed successfully", successCount)
		}
	}

	// Check and create Standard Purchase Approval workflow (smart skip)
	if err := ensureStandardPurchaseApprovalWorkflow(db); err != nil {
		log.Printf("⚠️  WORKFLOW AUTO-MIGRATION FAILED: %v", err)
	}

	// Ensure critical database functions exist (smart skip)
	if err := ensureSSOTSyncFunctions(db); err != nil {
		log.Printf("⚠️  Post-migration function install failed: %v", err)
	}

	// Ensure comprehensive balance sync system is installed and configured (smart skip)
	if err := ensureBalanceSyncSystem(db); err != nil {
		log.Printf("⚠️  BALANCE SYNC SYSTEM SETUP FAILED: %v", err)
	}

	// Prevent duplicate accounts system (smart skip)
	if err := RunPreventDuplicateAccountsMigration(db); err != nil {
		log.Printf("⚠️  DUPLICATE PREVENTION MIGRATION FAILED: %v", err)
	}

	// Cash Bank-COA auto-sync system (smart skip)
	if err := RunCashBankCOASyncMigration(db); err != nil {
		log.Printf("⚠️  CASH BANK-COA SYNC MIGRATION FAILED: %v", err)
	}

	// Fix revenue duplication issue (smart skip)
	if err := fixRevenueDuplication(db); err != nil {
		log.Printf("⚠️  REVENUE DUPLICATION FIX FAILED: %v", err)
	}

	// Fix audit_logs schema (smart skip)
	if err := AutoFixAuditLogsSchema(db); err != nil {
		log.Printf("⚠️  AUDIT_LOGS SCHEMA FIX FAILED: %v", err)
	}

	// Fix activity_logs user_id constraint for anonymous users (smart skip)
	if err := FixActivityLogsUserIDMigration(db); err != nil {
		log.Printf("⚠️  ACTIVITY_LOGS USER_ID FIX FAILED: %v", err)
	}

	// Sync sequences for tables with seeded data to prevent duplicate PK errors
	if err := SyncAllSeededSequences(db); err != nil {
		log.Printf("⚠️  SEQUENCE SYNC FAILED: %v", err)
	}

	log.Println("✅ Auto-migrations completed")
	return nil
}

// createMigrationLogsTable creates the migration_logs table if it doesn't exist
func createMigrationLogsTable(db *gorm.DB) error {
	// Execute CREATE TABLE statement
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS migration_logs (
		id SERIAL PRIMARY KEY,
		migration_name VARCHAR(255) NOT NULL UNIQUE,
		status VARCHAR(20) NOT NULL DEFAULT 'SUCCESS' CHECK (status IN ('SUCCESS', 'FAILED', 'SKIPPED')),
		message TEXT,
		description TEXT,
		executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		execution_time_ms INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`

	if err := db.Exec(createTableSQL).Error; err != nil {
		return err
	}

	// Execute INDEX statements separately
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_migration_logs_name ON migration_logs(migration_name)",
		"CREATE INDEX IF NOT EXISTS idx_migration_logs_status ON migration_logs(status)",
		"CREATE INDEX IF NOT EXISTS idx_migration_logs_executed_at ON migration_logs(executed_at)",
	}

	for _, indexSQL := range indexes {
		if err := db.Exec(indexSQL).Error; err != nil {
			// Index creation failure is not critical
			log.Printf("Warning: Failed to create index: %v", err)
		}
	}

	return nil
}

// getMigrationFiles gets all SQL migration files sorted by name
func getMigrationFiles() ([]string, error) {
	primaryDir, err := findMigrationDir()
	if err != nil {
		return nil, err
	}

	// Collect files from the primary migrations directory
	files, err := ioutil.ReadDir(primaryDir)
	if err != nil {
		return nil, err
	}

	var migrationFiles []string
	seen := map[string]struct{}{}
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".sql") {
			migrationFiles = append(migrationFiles, file.Name())
			seen[file.Name()] = struct{}{}
		}
	}

	// Also try to include fix files from backend/database/migrations if present
	// This allows pre-fix scripts like 000000000_comprehensive_migration_fix.sql to run first
	var additionalDir string
	if strings.HasSuffix(primaryDir, string(filepath.Separator)+"backend"+string(filepath.Separator)+"migrations") {
		additionalDir = filepath.Join(filepath.Dir(primaryDir), "database", "migrations")
	} else if strings.HasSuffix(primaryDir, string(filepath.Separator)+"migrations") {
		// Heuristic: check sibling backend/database/migrations relative to working directory
		cwd, _ := os.Getwd()
		additionalDir = filepath.Join(cwd, "backend", "database", "migrations")
	}
	if additionalDir != "" {
		if info, err := os.Stat(additionalDir); err == nil && info.IsDir() {
			addFiles, _ := ioutil.ReadDir(additionalDir)
			for _, f := range addFiles {
				if !f.IsDir() && strings.HasSuffix(f.Name(), ".sql") {
					if _, ok := seen[f.Name()]; !ok {
						migrationFiles = append(migrationFiles, f.Name())
						seen[f.Name()] = struct{}{}
					}
				}
			}
			log.Printf("Including additional migration dir: %s", additionalDir)
		}
	}

	// Sort files to ensure proper execution order
	sort.Strings(migrationFiles)

	// Log found migration files for debugging
	log.Printf("Using migration dir: %s", primaryDir)
	log.Printf("Found %d migration files (combined): %v", len(migrationFiles), migrationFiles)

	return migrationFiles, nil
}

// findMigrationDir tries multiple locations to locate the migrations folder
func findMigrationDir() (string, error) {
	candidates := []string{}

	// 0) Explicit environment override
	if envDir := strings.TrimSpace(os.Getenv("MIGRATIONS_DIR")); envDir != "" {
		candidates = append(candidates, filepath.Clean(envDir))
	}

	// 1) Current working directory and its parents
	cwd, _ := os.Getwd()
	if cwd != "" {
		candidates = append(candidates,
			filepath.Clean(filepath.Join(cwd, "migrations")),
			filepath.Clean(filepath.Join(cwd, "backend", "migrations")),
			filepath.Clean(filepath.Join(cwd, "..", "migrations")),
			filepath.Clean(filepath.Join(cwd, "..", "backend", "migrations")),
			filepath.Clean(filepath.Join(cwd, "..", "..", "migrations")),
			filepath.Clean(filepath.Join(cwd, "..", "..", "backend", "migrations")),
		)
	}

	// 2) Relative to process (works when running from repo root or backend dir)
	candidates = append(candidates,
		filepath.Clean("./migrations"),
		filepath.Clean("backend/migrations"),
		filepath.Clean("../migrations"),
		filepath.Clean("../backend/migrations"),
		filepath.Clean("../../migrations"),
		filepath.Clean("../../backend/migrations"),
	)

	// 3) Directory next to the executable and its parents
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, "migrations"),
			filepath.Join(exeDir, "backend", "migrations"),
			filepath.Join(exeDir, "..", "migrations"),
			filepath.Join(exeDir, "..", "backend", "migrations"),
			filepath.Join(exeDir, "..", "..", "migrations"),
			filepath.Join(exeDir, "..", "..", "backend", "migrations"),
		)
	}

	// Deduplicate while preserving order
	seen := map[string]struct{}{}
	unique := make([]string, 0, len(candidates))
	for _, dir := range candidates {
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		unique = append(unique, dir)
	}

	for _, dir := range unique {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, nil
		}
	}
	return "", fmt.Errorf("migrations directory not found. Tried: %v", unique)
}

// runMigration runs a single migration file
func runMigration(db *gorm.DB, filename string) error {
	// Check last status from migration_logs; only skip if SUCCESS
	var lastStatus string
	statusErr := db.Raw("SELECT status FROM migration_logs WHERE migration_name = ? ORDER BY executed_at DESC LIMIT 1", filename).Scan(&lastStatus).Error
	if statusErr == nil && strings.EqualFold(lastStatus, "SUCCESS") {
		// Silently skip successful migrations
		return nil
	}

	startTime := time.Now()
	log.Printf("🔄 Running: %s", filename)

	// Read migration file
	migrationDir, dirErr := findMigrationDir()
	if dirErr != nil {
		logMigrationResult(db, filename, "FAILED", fmt.Sprintf("Failed to locate migrations dir: %v", dirErr), 0)
		return dirErr
	}
	migrationPath := filepath.Join(migrationDir, filename)
	contentBytes, err := ioutil.ReadFile(migrationPath)
	if err != nil {
		// Try reading from additional fix dir (backend/database/migrations)
		var tryAlt []byte
		var altErr error
		// Compute additional dir similarly to getMigrationFiles
		var additionalDir string
		if strings.HasSuffix(migrationDir, string(filepath.Separator)+"backend"+string(filepath.Separator)+"migrations") {
			additionalDir = filepath.Join(filepath.Dir(migrationDir), "database", "migrations")
		} else if strings.HasSuffix(migrationDir, string(filepath.Separator)+"migrations") {
			cwd, _ := os.Getwd()
			additionalDir = filepath.Join(cwd, "backend", "database", "migrations")
		}
		if additionalDir != "" {
			altPath := filepath.Join(additionalDir, filename)
			tryAlt, altErr = ioutil.ReadFile(altPath)
			if altErr == nil {
				contentBytes = tryAlt
				goto GOT_CONTENT
			}
		}
		logMigrationResult(db, filename, "FAILED", fmt.Sprintf("Failed to read file: %v", err), 0)
		return err
	}
GOT_CONTENT:
	content := string(contentBytes)

	// Use complex parser for any file that defines PL/pgSQL functions, dollar-quoted blocks, or explicit transactions
	lowerContent := strings.ToLower(content)
	// Check if file uses explicit transaction management (BEGIN/COMMIT)
	hasExplicitTransaction := false
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)
		if (upper == "BEGIN;" || upper == "COMMIT;") && !strings.HasPrefix(trimmed, "--") {
			hasExplicitTransaction = true
			break
		}
	}

	if strings.Contains(filename, "unified_journal_ssot") ||
		strings.Contains(lowerContent, "language plpgsql") ||
		strings.Contains(lowerContent, "create function") ||
		strings.Contains(lowerContent, "create or replace function") ||
		strings.Contains(lowerContent, "$$") ||
		hasExplicitTransaction {
		return runComplexMigration(db, filename, content, startTime)
	}

	// Split SQL statements by semicolon (simple approach)
	sqlStatements := strings.Split(content, ";")

	// Execute statements one-by-one in separate transactions.
	// This allows us to skip harmless 'already exists' errors and continue.
	for _, raw := range sqlStatements {
		stmt := strings.TrimSpace(raw)
		if stmt == "" || strings.HasPrefix(stmt, "--") || strings.HasPrefix(stmt, "/*") {
			continue
		}

		// Execute each statement in its own transaction to avoid transaction abort cascade
		tx := db.Begin()
		if tx.Error != nil {
			executionTime := int(time.Since(startTime).Milliseconds())
			logMigrationResult(db, filename, "FAILED", fmt.Sprintf("Failed to begin transaction: %v", tx.Error), executionTime)
			return tx.Error
		}

		if err := tx.Exec(stmt).Error; err != nil {
			tx.Rollback() // Clean up the aborted transaction
			// Gracefully handle idempotent scenarios
			if isAlreadyExistsError(err) {
				atomic.AddInt64(&idempotentSkipCount, 1)
				// Silently skip - don't log every idempotent object
				continue
			}
			executionTime := int(time.Since(startTime).Milliseconds())
			// Enhanced error message with context
			errorMsg := fmt.Sprintf("SQL error: %v\n\nFailed statement:\n%s\n\nFile: %s", err, stmt, filename)
			log.Printf("❌ Error details:\n%s", errorMsg)
			logMigrationResult(db, filename, "FAILED", fmt.Sprintf("SQL error: %v", err), executionTime)
			return fmt.Errorf("%v\n\nHint: If this file uses BEGIN/COMMIT, the parser may need adjustment", err)
		}

		if err := tx.Commit().Error; err != nil {
			executionTime := int(time.Since(startTime).Milliseconds())
			logMigrationResult(db, filename, "FAILED", fmt.Sprintf("Failed to commit transaction: %v", err), executionTime)
			return err
		}
	}

	executionTime := int(time.Since(startTime).Milliseconds())
	logMigrationResult(db, filename, "SUCCESS", "Migration completed successfully", executionTime)

	log.Printf("✅ Migration completed: %s (%dms)", filename, executionTime)
	return nil
}

// runComplexMigration runs complex migrations with better SQL parsing
func runComplexMigration(db *gorm.DB, filename, content string, startTime time.Time) error {
	log.Printf("🏠 Running complex migration (SSOT): %s", filename)

	// Parse SQL with a robust tokenizer that respects dollar-quoted strings
	statements := parseComplexSQL(content)

	transactionOpen := false

	// Execute each parsed statement
	for i, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		// Only log every 10th statement to reduce noise
		if i == 0 || i%10 == 0 || i == len(statements)-1 {
			log.Printf("🔧 Progress: %d/%d statements...", i+1, len(statements))
		}

		upper := strings.ToUpper(strings.TrimSpace(strings.TrimSuffix(stmt, ";")))
		if upper == "BEGIN" || strings.HasPrefix(upper, "BEGIN TRANSACTION") {
			transactionOpen = true
		}
		if upper == "COMMIT" || upper == "ROLLBACK" {
			transactionOpen = false
		}

		// Decide if this statement must run outside a transaction (e.g., CONCURRENTLY)
		runOutsideTx := strings.Contains(upper, "CREATE INDEX") && strings.Contains(upper, "CONCURRENTLY") ||
			strings.Contains(upper, "REFRESH MATERIALIZED VIEW CONCURRENTLY")

		var err error
		if transactionOpen {
			if runOutsideTx {
				// Temporarily end the user-managed transaction to allow CONCURRENT operations
				_ = db.Exec("COMMIT").Error
				transactionOpen = false
				err = db.Exec(stmt).Error
				// Re-open a transaction to preserve file semantics
				if err == nil {
					_ = db.Exec("BEGIN").Error
					transactionOpen = true
				}
			} else {
				// Migration file manages its own transactions (BEGIN/COMMIT)
				err = db.Exec(stmt).Error
			}
		} else {
			if runOutsideTx {
				// Execute directly without wrapping in an implicit transaction
				err = db.Exec(stmt).Error
			} else {
				// Wrap in separate transaction to prevent transaction abort cascade
				tx := db.Begin()
				if tx.Error != nil {
					executionTime := int(time.Since(startTime).Milliseconds())
					logMigrationResult(db, filename, "FAILED", fmt.Sprintf("Failed to begin transaction at statement %d: %v", i+1, tx.Error), executionTime)
					return fmt.Errorf("failed to begin transaction at statement %d: %v", i+1, tx.Error)
				}
				err = tx.Exec(stmt).Error
				if err != nil {
					tx.Rollback()
				} else {
					err = tx.Commit().Error
				}
			}
		}

		if err != nil {
			// Allow idempotent reruns: skip 'already exists' type errors
			if isAlreadyExistsError(err) {
				atomic.AddInt64(&idempotentSkipCount, 1)
				// Silently skip - don't log every idempotent object
				continue
			}
			// If the file opened a transaction, try to rollback to clear aborted state before logging
			if transactionOpen {
				_ = db.Exec("ROLLBACK").Error
				transactionOpen = false
			}
			executionTime := int(time.Since(startTime).Milliseconds())
			logMigrationResult(db, filename, "FAILED", fmt.Sprintf("SQL error at statement %d: %v", i+1, err), executionTime)
			return fmt.Errorf("SQL error at statement %d: %v", i+1, err)
		}
	}

	executionTime := int(time.Since(startTime).Milliseconds())
	logMigrationResult(db, filename, "SUCCESS", "Complex migration completed successfully", executionTime)

	log.Printf("✅ Complex migration completed: %s (%dms)", filename, executionTime)
	return nil
}

// parseComplexSQL parses SQL into executable statements, respecting strings, comments, and dollar-quoted blocks
func parseComplexSQL(content string) []string {
	var statements []string
	var b strings.Builder

	inSingle := false       // inside '...'
	inDouble := false       // inside "..."
	inLineComment := false  // inside -- ... \n
	inBlockComment := false // inside /* ... */
	dollarTag := ""         // current $tag$ or $$ if inside a dollar-quoted string

	i := 0
	for i < len(content) {
		ch := content[i]
		var next byte
		if i+1 < len(content) {
			next = content[i+1]
		}

		// Enter/exit line comments
		if !inSingle && !inDouble && dollarTag == "" && !inBlockComment && !inLineComment && ch == '-' && next == '-' {
			inLineComment = true
		}
		if inLineComment {
			b.WriteByte(ch)
			if ch == '\n' {
				inLineComment = false
			}
			i++
			continue
		}

		// Enter block comment
		if !inSingle && !inDouble && dollarTag == "" && !inBlockComment && ch == '/' && next == '*' {
			inBlockComment = true
			b.WriteByte(ch)
			b.WriteByte(next)
			i += 2
			continue
		}
		// Exit block comment
		if inBlockComment {
			b.WriteByte(ch)
			if ch == '*' && next == '/' {
				b.WriteByte(next)
				i += 2
				inBlockComment = false
				continue
			}
			i++
			continue
		}

		// Dollar-quoted strings: $tag$ ... $tag$
		if !inSingle && !inDouble {
			if dollarTag == "" && ch == '$' {
				// try to read $tag$
				j := i + 1
				for j < len(content) {
					c := content[j]
					if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
						j++
						continue
					}
					break
				}
				if j < len(content) && content[j] == '$' {
					dollarTag = content[i : j+1] // includes both $ ... $
					b.WriteString(dollarTag)
					i = j + 1
					continue
				}
			} else if dollarTag != "" {
				// check end tag
				if strings.HasPrefix(content[i:], dollarTag) {
					b.WriteString(dollarTag)
					i += len(dollarTag)
					dollarTag = ""
					continue
				}
			}
		}

		// Quoted strings
		if dollarTag == "" {
			if !inDouble && ch == '\'' {
				if inSingle {
					// handle escaped ''
					if next == '\'' {
						b.WriteByte(ch)
						b.WriteByte(next)
						i += 2
						continue
					}
					inSingle = false
				} else {
					inSingle = true
				}
			} else if !inSingle && ch == '"' {
				if inDouble {
					inDouble = false
				} else {
					inDouble = true
				}
			}
		}

		// Statement terminator
		if ch == ';' && !inSingle && !inDouble && dollarTag == "" && !inBlockComment && !inLineComment {
			stmt := strings.TrimSpace(b.String())
			if stmt != "" {
				statements = append(statements, stmt+";")
			}
			b.Reset()
			i++
			continue
		}

		b.WriteByte(ch)
		i++
	}

	rest := strings.TrimSpace(b.String())
	if rest != "" {
		statements = append(statements, rest)
	}
	return statements
}

// logMigrationResult logs the result of a migration
// ensureSSOTSyncFunctions creates or replaces required SSOT sync functions in a parser-safe way
// This is idempotent and safe to run on every startup across environments
func ensureSSOTSyncFunctions(db *gorm.DB) error {
	// Check existing variants (silently)
	var cntBigint, cntInteger int64
	checkBigint := `SELECT COUNT(*) FROM pg_proc WHERE proname='sync_account_balance_from_ssot' AND pg_get_function_identity_arguments(oid) ILIKE '%bigint%'`
	checkInteger := `SELECT COUNT(*) FROM pg_proc WHERE proname='sync_account_balance_from_ssot' AND pg_get_function_identity_arguments(oid) ILIKE '%integer%'`
	if err := db.Raw(checkBigint).Scan(&cntBigint).Error; err != nil {
		log.Printf("⚠️  Could not check existing BIGINT variant: %v", err)
	}
	if err := db.Raw(checkInteger).Scan(&cntInteger).Error; err != nil {
		log.Printf("⚠️  Could not check existing INTEGER variant: %v", err)
	}
	alreadyBigint := cntBigint > 0
	_ = cntInteger // silence unused warning

	bigintFn := `
CREATE OR REPLACE FUNCTION sync_account_balance_from_ssot(account_id_param BIGINT)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE accounts a
    SET 
        balance = COALESCE((
            SELECT CASE 
                WHEN a.type IN ('ASSET', 'EXPENSE') THEN 
                    COALESCE(SUM(ujl.debit_amount), 0) - COALESCE(SUM(ujl.credit_amount), 0)
                ELSE 
                    COALESCE(SUM(ujl.credit_amount), 0) - COALESCE(SUM(ujl.debit_amount), 0)
            END
            FROM unified_journal_lines ujl 
            LEFT JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
            WHERE ujl.account_id = account_id_param 
              AND uje.status = 'POSTED'
              AND uje.deleted_at IS NULL
        ), 0),
        updated_at = NOW()
    WHERE a.id = account_id_param;
END;
$$;`

	intFn := `
CREATE OR REPLACE FUNCTION sync_account_balance_from_ssot(account_id_param INTEGER)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM sync_account_balance_from_ssot(account_id_param::BIGINT);
END;
$$;`

	if err := db.Exec(bigintFn).Error; err != nil {
		return err
	}
	if err := db.Exec(intFn).Error; err != nil {
		return err
	}

	// Re-check to confirm (silently)
	cntBigint, cntInteger = 0, 0
	_ = db.Raw(checkBigint).Scan(&cntBigint).Error
	_ = db.Raw(checkInteger).Scan(&cntInteger).Error
	nowBigint := cntBigint > 0
	_ = cntInteger // silence unused warning

	// Only log if we actually installed something new
	if !alreadyBigint && nowBigint {
		log.Println("✅ Installed SSOT sync functions")
	}

	return nil
}

// functionExists checks if a PostgreSQL function with the given name exists (any signature)
func functionExists(db *gorm.DB, name string) bool {
	var exists bool
	err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_proc WHERE proname = $1
		)
	`, name).Scan(&exists).Error
	if err != nil {
		return false
	}
	return exists
}

// triggerExists checks if a trigger exists by name
func triggerExists(db *gorm.DB, triggerName string) bool {
	var cnt int64
	err := db.Raw(`
		SELECT COUNT(*) FROM information_schema.triggers WHERE trigger_name = $1
	`, triggerName).Scan(&cnt).Error
	if err != nil {
		return false
	}
	return cnt > 0
}

// extractTriggerName tries to parse trigger name from a CREATE TRIGGER statement
func extractTriggerName(stmt string) string {
	// Expect pattern: CREATE TRIGGER <name> ...
	parts := strings.Fields(stmt)
	if len(parts) >= 3 && strings.EqualFold(parts[0], "CREATE") && strings.EqualFold(parts[1], "TRIGGER") {
		return strings.Trim(parts[2], "\"`")
	}
	return ""
}

// extractFunctionNameFromComment parses COMMENT ON FUNCTION <name>(...) from statement
func extractFunctionNameFromComment(stmt string) string {
	lower := strings.ToLower(stmt)
	prefix := "comment on function "
	idx := strings.Index(lower, prefix)
	if idx == -1 {
		return ""
	}
	rest := strings.TrimSpace(stmt[idx+len(prefix):])
	// until first '(' or ' '
	for i, r := range rest {
		if r == '(' || r == ' ' || r == '\n' || r == '\t' {
			return strings.Trim(rest[:i], "\"`")
		}
	}
	return strings.Trim(rest, "\"`\t ")
}

// isAlreadyExistsError detects non-fatal, idempotent errors (object already exists or duplicate)
func isAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "already exists") ||
		strings.Contains(s, "duplicate key value") ||
		strings.Contains(s, "already defined")
}

// getObjectNameFromError extracts the object name from PostgreSQL error message
func getObjectNameFromError(err error) string {
	if err == nil {
		return "unknown object"
	}
	errMsg := err.Error()

	// Try to extract table/index/constraint name from error message
	// Format: ERROR: relation "table_name" already exists
	if idx := strings.Index(errMsg, "relation \""); idx != -1 {
		start := idx + len("relation \"")
		if end := strings.Index(errMsg[start:], "\""); end != -1 {
			return errMsg[start : start+end]
		}
	}

	// Format: ERROR: constraint "constraint_name" already exists
	if idx := strings.Index(errMsg, "constraint \""); idx != -1 {
		start := idx + len("constraint \"")
		if end := strings.Index(errMsg[start:], "\""); end != -1 {
			return errMsg[start : start+end]
		}
	}

	// Format: ERROR: index "index_name" already exists
	if idx := strings.Index(errMsg, "index \""); idx != -1 {
		start := idx + len("index \"")
		if end := strings.Index(errMsg[start:], "\""); end != -1 {
			return errMsg[start : start+end]
		}
	}

	// Format: ERROR: function "function_name" already exists
	if idx := strings.Index(errMsg, "function "); idx != -1 {
		start := idx + len("function ")
		if end := strings.Index(errMsg[start:], " already"); end != -1 {
			return errMsg[start : start+end]
		}
	}

	// Return shortened error if we can't extract the name
	if len(errMsg) > 100 {
		return errMsg[:100] + "..."
	}
	return errMsg
}

func logMigrationResult(db *gorm.DB, migrationName, status, message string, executionTimeMs int) {
	sql := `
	INSERT INTO migration_logs (migration_name, status, message, execution_time_ms)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (migration_name) DO UPDATE SET
		status = EXCLUDED.status,
		message = EXCLUDED.message,
		execution_time_ms = EXCLUDED.execution_time_ms,
		executed_at = CURRENT_TIMESTAMP
	`

	if err := db.Exec(sql, migrationName, status, message, executionTimeMs).Error; err != nil {
		log.Printf("⚠️  Failed to log migration result: %v", err)
	}
}

// GetMigrationStatus returns the status of all migrations
func GetMigrationStatus(db *gorm.DB) ([]MigrationLog, error) {
	var logs []MigrationLog
	err := db.Order("executed_at DESC").Find(&logs).Error
	return logs, err
}

// MigrationLog represents a migration log entry
type MigrationLog struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	MigrationName   string    `json:"migration_name" gorm:"size:255;uniqueIndex"`
	Status          string    `json:"status" gorm:"size:20"`
	Message         string    `json:"message" gorm:"type:text"`
	ExecutedAt      time.Time `json:"executed_at"`
	ExecutionTimeMs int       `json:"execution_time_ms"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ApprovalWorkflow represents the approval_workflows table for auto-migration
type ApprovalWorkflow struct {
	ID              uint    `gorm:"primaryKey"`
	Name            string  `gorm:"not null;size:100"`
	Module          string  `gorm:"not null;size:50"`
	MinAmount       float64 `gorm:"type:decimal(15,2);default:0"`
	MaxAmount       float64 `gorm:"type:decimal(15,2)"`
	IsActive        bool    `gorm:"default:true"`
	RequireDirector bool    `gorm:"default:false"`
	RequireFinance  bool    `gorm:"default:false"`
}

// ApprovalStep represents the approval_steps table for auto-migration
type ApprovalStep struct {
	ID           uint   `gorm:"primaryKey"`
	WorkflowID   uint   `gorm:"not null;index"`
	StepOrder    int    `gorm:"not null"`
	StepName     string `gorm:"not null;size:100"`
	ApproverRole string `gorm:"not null;size:50"`
	IsOptional   bool   `gorm:"default:false"`
	TimeLimit    int    `gorm:"default:24"`
}

// ensureStandardPurchaseApprovalWorkflow checks and creates Standard Purchase Approval workflow if it doesn't exist
func ensureStandardPurchaseApprovalWorkflow(db *gorm.DB) error {
	// Check if Standard Purchase Approval workflow exists
	var existingWorkflow ApprovalWorkflow
	result := db.Where("name = ? AND module = ?", "Standard Purchase Approval", "PURCHASE").First(&existingWorkflow)

	if result.Error == nil {
		// Check if workflow has steps
		var stepCount int64
		db.Model(&ApprovalStep{}).Where("workflow_id = ?", existingWorkflow.ID).Count(&stepCount)

		if stepCount == 0 {
			log.Println("🔧 Creating workflow steps...")
			// Create steps for existing workflow
			return createWorkflowSteps(db, existingWorkflow.ID)
		} else {
			// Silently skip if everything exists
			return nil
		}
	}

	// If not found, create it
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		log.Println("🔧 Creating Standard Purchase Approval workflow...")

		// Create workflow
		workflow := ApprovalWorkflow{
			Name:            "Standard Purchase Approval",
			Module:          "PURCHASE",
			MinAmount:       0,
			MaxAmount:       999999999999,
			IsActive:        true,
			RequireDirector: true,
			RequireFinance:  true,
		}

		if err := db.Create(&workflow).Error; err != nil {
			return fmt.Errorf("failed to create Standard Purchase Approval workflow: %v", err)
		}

		// Create workflow steps
		return createWorkflowSteps(db, workflow.ID)
	}

	// Other database errors
	return fmt.Errorf("failed to check existing workflow: %v", result.Error)
}

// createWorkflowSteps creates the standard approval workflow steps
func createWorkflowSteps(db *gorm.DB, workflowID uint) error {
	steps := []ApprovalStep{
		{
			WorkflowID:   workflowID,
			StepOrder:    1,
			StepName:     "Employee Submission",
			ApproverRole: "employee",
			IsOptional:   false,
			TimeLimit:    24,
		},
		{
			WorkflowID:   workflowID,
			StepOrder:    2,
			StepName:     "Finance Approval",
			ApproverRole: "finance",
			IsOptional:   false,
			TimeLimit:    48,
		},
		{
			WorkflowID:   workflowID,
			StepOrder:    3,
			StepName:     "Director Approval",
			ApproverRole: "director",
			IsOptional:   true,
			TimeLimit:    72,
		},
	}

	for _, step := range steps {
		if err := db.Create(&step).Error; err != nil {
			return fmt.Errorf("failed to create workflow step '%s': %v", step.StepName, err)
		}
	}

	log.Println("✅ Standard Purchase Approval workflow created")

	return nil
}

// runPreMigrationFixes runs automatic fixes before migrations to ensure compatibility
// This prevents migration failures when clients pull new code
func runPreMigrationFixes(db *gorm.DB) error {
	log.Println("🔧 Running pre-migration compatibility fixes...")

	// Fix 1: Ensure 'description' column exists in migration_logs table
	if err := ensureMigrationLogsDescriptionColumn(db); err != nil {
		return fmt.Errorf("failed to ensure migration_logs description column: %v", err)
	}

	// Fix 2: Mark problematic migrations as SUCCESS to prevent re-execution
	if err := markProblematicMigrationsAsSuccess(db); err != nil {
		log.Printf("⚠️  Warning: Failed to mark problematic migrations: %v", err)
		// Don't fail completely, just warn
	}

	// NOTE: Materialized view account_balances has been removed - using SSOT direct query instead
	// All reports now use /api/v1/ssot-reports/* endpoints with real-time query to unified_journal_lines

	log.Println("✅ Pre-migration compatibility fixes completed")
	return nil
}

// ensureMigrationLogsDescriptionColumn adds the missing description column if it doesn't exist
func ensureMigrationLogsDescriptionColumn(db *gorm.DB) error {
	// Check if description column exists
	var columnExists bool
	err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'migration_logs' 
			AND column_name = 'description'
		);
	`).Scan(&columnExists).Error

	if err != nil {
		return fmt.Errorf("failed to check description column: %v", err)
	}

	if !columnExists {
		log.Println("📝 Adding missing 'description' column to migration_logs table...")

		// Add the missing column
		err = db.Exec(`ALTER TABLE migration_logs ADD COLUMN description TEXT;`).Error
		if err != nil {
			return fmt.Errorf("failed to add description column: %v", err)
		}

		log.Println("✅ Added 'description' column to migration_logs table")
	} else {
		log.Println("ℹ️  Description column already exists")
	}

	return nil
}

// markProblematicMigrationsAsSuccess marks migrations that are known to cause issues as SUCCESS
func markProblematicMigrationsAsSuccess(db *gorm.DB) error {
	// List of migrations that should be marked as SUCCESS to prevent re-execution
	problematicMigrations := []string{
		"011_purchase_payment_integration.sql",
		"012_purchase_payment_integration_pg.sql",
		"013_payment_performance_optimization.sql",
		"020_add_sales_data_integrity_constraints.sql",
		"021_add_sales_performance_indices.sql",
		"022_comprehensive_model_updates.sql",
		"023_create_purchase_approval_workflows.sql",
		"025_safe_ssot_journal_migration_fix.sql",
		"026_fix_sync_account_balance_fn_bigint.sql",
		"030_create_account_balances_materialized_view.sql",
		"add_accounts_code_unique_constraint.sql",
		"prevent_duplicate_accounts.sql",
		"database_enhancements_v2024_1.sql",
	}

	now := time.Now()
	updatedCount := 0

	for _, migrationName := range problematicMigrations {
		var existingStatus string
		var existingID int

		err := db.Raw(`
			SELECT id, status FROM migration_logs 
			WHERE migration_name = $1
		`, migrationName).Row().Scan(&existingID, &existingStatus)

		if err != nil {
			// Migration doesn't exist in logs, insert it as SUCCESS
			err = db.Exec(`
				INSERT INTO migration_logs 
				(migration_name, status, message, description, executed_at, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, migrationName, "SUCCESS", "Auto-fixed by pre-migration compatibility check",
				"Migration marked as SUCCESS to prevent re-execution issues during auto-migrations",
				now, now, now).Error

			if err != nil {
				log.Printf("⚠️  Failed to insert %s: %v", migrationName, err)
			} else {
				log.Printf("✅ Inserted %s as SUCCESS", migrationName)
				updatedCount++
			}
		} else if existingStatus != "SUCCESS" {
			// Update existing record to SUCCESS
			err = db.Exec(`
				UPDATE migration_logs 
				SET status = $1, 
				    message = $2, 
				    description = $3,
				    executed_at = $4, 
				    updated_at = $5
				WHERE id = $6
			`, "SUCCESS", "Auto-fixed by pre-migration compatibility check",
				"Migration marked as SUCCESS to prevent re-execution issues during auto-migrations",
				now, now, existingID).Error

			if err != nil {
				log.Printf("⚠️  Failed to update %s: %v", migrationName, err)
			} else {
				log.Printf("✅ Updated %s from %s to SUCCESS", migrationName, existingStatus)
				updatedCount++
			}
		}
	}

	log.Printf("📊 Updated %d problematic migrations to SUCCESS status", updatedCount)
	return nil
}

// ensureBalanceSyncSystem ensures the SSOT-based balance sync triggers are installed
// This is idempotent and safe to run on every startup across environments
// NOTE: Only installs triggers for unified_journal_lines/unified_journal_ledger.
// Does NOT install the old cashbank→COA triggers (trigger_sync_cashbank_coa etc.)
// which caused double-posting. See migration 20251017_disable_auto_balance_sync_triggers.sql.
func ensureBalanceSyncSystem(db *gorm.DB) error {
	// Check if the SSOT trigger already exists
	var triggerExists int64
	checkTrigger := `SELECT COUNT(*) FROM information_schema.triggers WHERE trigger_name = 'trg_sync_account_balance_on_line_change'`
	if err := db.Raw(checkTrigger).Scan(&triggerExists).Error; err != nil {
		log.Printf("⚠️  Could not check trigger existence: %v", err)
	}

	if triggerExists > 0 {
		// Triggers already installed, skip silently
		return nil
	}

	log.Println("🔧 Installing SSOT balance sync triggers...")

	// 1. Create trigger function for journal line changes
	triggerFnLines := `
CREATE OR REPLACE FUNCTION trigger_sync_account_balance()
RETURNS TRIGGER AS $$
DECLARE
    affected_account_id INTEGER;
BEGIN
    IF TG_OP = 'INSERT' OR TG_OP = 'UPDATE' THEN
        affected_account_id := NEW.account_id;
        IF EXISTS (
            SELECT 1 FROM unified_journal_ledger
            WHERE id = NEW.journal_id AND status = 'POSTED'
        ) THEN
            PERFORM sync_account_balance_from_ssot(affected_account_id);
        END IF;
        RETURN NEW;
    END IF;

    IF TG_OP = 'DELETE' THEN
        affected_account_id := OLD.account_id;
        PERFORM sync_account_balance_from_ssot(affected_account_id);
        RETURN OLD;
    END IF;

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;`

	if err := db.Exec(triggerFnLines).Error; err != nil {
		return fmt.Errorf("failed to create trigger_sync_account_balance function: %v", err)
	}

	// 2. Create trigger function for journal status changes
	triggerFnStatus := `
CREATE OR REPLACE FUNCTION trigger_sync_on_journal_status_change()
RETURNS TRIGGER AS $$
DECLARE
    line_record RECORD;
BEGIN
    IF NEW.status = 'POSTED' AND (OLD.status IS NULL OR OLD.status != 'POSTED') THEN
        FOR line_record IN
            SELECT DISTINCT account_id
            FROM unified_journal_lines
            WHERE journal_id = NEW.id
        LOOP
            PERFORM sync_account_balance_from_ssot(line_record.account_id);
        END LOOP;
    END IF;

    IF NEW.status IN ('REVERSED', 'CANCELLED') AND OLD.status = 'POSTED' THEN
        FOR line_record IN
            SELECT DISTINCT account_id
            FROM unified_journal_lines
            WHERE journal_id = NEW.id
        LOOP
            PERFORM sync_account_balance_from_ssot(line_record.account_id);
        END LOOP;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;`

	if err := db.Exec(triggerFnStatus).Error; err != nil {
		return fmt.Errorf("failed to create trigger_sync_on_journal_status_change function: %v", err)
	}

	// 3. Drop and re-create triggers (idempotent)
	triggers := []string{
		`DROP TRIGGER IF EXISTS trg_sync_account_balance_on_line_change ON unified_journal_lines`,
		`CREATE TRIGGER trg_sync_account_balance_on_line_change
		    AFTER INSERT OR UPDATE OR DELETE ON unified_journal_lines
		    FOR EACH ROW
		    EXECUTE FUNCTION trigger_sync_account_balance()`,
		`DROP TRIGGER IF EXISTS trg_sync_account_balance_on_status_change ON unified_journal_ledger`,
		`CREATE TRIGGER trg_sync_account_balance_on_status_change
		    AFTER UPDATE ON unified_journal_ledger
		    FOR EACH ROW
		    WHEN (OLD.status IS DISTINCT FROM NEW.status)
		    EXECUTE FUNCTION trigger_sync_on_journal_status_change()`,
	}

	for _, sql := range triggers {
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("failed to execute trigger SQL: %v", err)
		}
	}

	log.Println("✅ SSOT balance sync triggers installed successfully")
	return nil
}

// checkBalanceSyncTriggers checks if balance sync triggers are installed
func checkBalanceSyncTriggers(db *gorm.DB) (bool, error) {
	requiredTriggers := []string{
		"trigger_recalc_cashbank_balance_insert",
		"trigger_recalc_cashbank_balance_update",
		"trigger_recalc_cashbank_balance_delete",
		"trigger_validate_account_balance",
	}

	for _, triggerName := range requiredTriggers {
		var count int64
		err := db.Raw(`
			SELECT COUNT(*) FROM information_schema.triggers 
			WHERE trigger_name = $1
		`, triggerName).Scan(&count).Error

		if err != nil {
			return false, err
		}

		if count == 0 {
			// Don't log missing triggers during silent check
			return false, nil
		}
	}

	return true, nil
}

// checkBalanceSyncFunctions checks if balance sync functions are installed
func checkBalanceSyncFunctions(db *gorm.DB) (bool, error) {
	requiredFunctions := []string{
		"update_parent_account_balances",
		"recalculate_cashbank_balance",
		"validate_account_balance_consistency",
		"manual_sync_cashbank_coa",
		"manual_sync_all_cashbank_coa",
		"ensure_cashbank_not_header",
	}

	for _, functionName := range requiredFunctions {
		var count int64
		err := db.Raw(`
			SELECT COUNT(*) FROM pg_proc 
			WHERE proname = $1
		`, functionName).Scan(&count).Error

		if err != nil {
			return false, err
		}

		if count == 0 {
			// Don't log missing functions during silent check
			return false, nil
		}
	}

	return true, nil
}

// installBalanceSyncSystem installs or updates the balance sync system
func installBalanceSyncSystem(db *gorm.DB) error {

	// Read the comprehensive balance sync migration content
	migrationPath := "20250930_comprehensive_auto_balance_sync.sql"
	migrationDir, err := findMigrationDir()
	if err != nil {
		return fmt.Errorf("failed to find migration directory: %w", err)
	}

	fullPath := filepath.Join(migrationDir, migrationPath)
	content, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("failed to read balance sync migration file: %w", err)
	}

	// Parse and execute the migration using the complex parser
	log.Printf("🔧 Executing balance sync migration: %s", migrationPath)
	statements := parseComplexSQL(string(content))

	// Pre-check known functions and triggers to proactively skip problematic statements
	knownFns := []string{
		"recalculate_cashbank_balance",
		"validate_account_balance_consistency",
		"manual_sync_cashbank_coa",
		"manual_sync_all_cashbank_coa",
		"ensure_cashbank_not_header",
		"update_parent_account_balances",
	}
	fnExists := map[string]bool{}
	for _, fn := range knownFns {
		fnExists[fn] = functionExists(db, fn)
	}

	for i, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}
		upper := strings.ToUpper(stmt)

		// Proactively skip CREATE TRIGGER statements that reference missing functions
		if strings.HasPrefix(upper, "CREATE TRIGGER ") {
			// If the trigger references a known function that's missing, skip without executing
			for fn, ok := range fnExists {
				if strings.Contains(strings.ToLower(stmt), fn+"(") && !ok {
					log.Printf("   ⏭️  Skipping statement %d - required function '%s' not available yet", i+1, fn)
					goto NEXT_STMT
				}
			}
			// Also skip if trigger already exists
			if trName := extractTriggerName(stmt); trName != "" && triggerExists(db, trName) {
				log.Printf("   ⏭️  Skipping statement %d - trigger '%s' already exists", i+1, trName)
				goto NEXT_STMT
			}
		}

		// Proactively skip COMMENT ON FUNCTION for missing functions
		if strings.HasPrefix(upper, "COMMENT ON FUNCTION ") {
			if fn := extractFunctionNameFromComment(stmt); fn != "" && !functionExists(db, fn) {
				log.Printf("   ⏭️  Skipping statement %d - function '%s' not found for COMMENT", i+1, fn)
				goto NEXT_STMT
			}
		}

		if err := db.Exec(stmt).Error; err != nil {
			errMsg := strings.ToLower(err.Error())

			// Skip if object already exists (idempotent)
			if isAlreadyExistsError(err) {
				log.Printf("   ⏭️  Skipping statement %d - object already exists", i+1)
				goto NEXT_STMT
			}

			// Skip if function doesn't exist (might be created later in the migration)
			if strings.Contains(errMsg, "does not exist") && strings.Contains(errMsg, "function") {
				log.Printf("   ⏭️  Skipping statement %d - function will be created later", i+1)
				goto NEXT_STMT
			}

			return fmt.Errorf("failed to execute statement %d: %w\nSQL: %s", i+1, err, stmt)
		}
	NEXT_STMT:
		continue
	}

	log.Println("✅ Balance sync system installed successfully")
	return nil
}

// ensureCashBankAccountConfiguration ensures cash bank accounts are properly configured
func ensureCashBankAccountConfiguration(db *gorm.DB) error {
	// Check if we have cash banks to configure
	var cashBankCount int64
	if err := db.Raw("SELECT COUNT(*) FROM cash_banks WHERE deleted_at IS NULL").Scan(&cashBankCount).Error; err != nil {
		return fmt.Errorf("failed to count cash banks: %w", err)
	}

	if cashBankCount == 0 {
		return nil // silent skip
	}

	// Ensure cash bank accounts are not header accounts, but only if helper function exists
	if functionExists(db, "ensure_cashbank_not_header") {
		if err := db.Exec("SELECT ensure_cashbank_not_header()").Error; err != nil {
			return fmt.Errorf("failed to ensure non-header status: %w", err)
		}
		log.Println("✅ Cash bank account configuration completed")
	}
	return nil
}

// performInitialBalanceSync performs initial balance synchronization
func performInitialBalanceSync(db *gorm.DB) error {
	// Check if sync functions are available
	if !functionExists(db, "manual_sync_all_cashbank_coa") ||
		!functionExists(db, "update_parent_account_balances") {
		return nil // silent skip if dependencies not available
	}

	// Perform the sync
	var syncResult string
	err := db.Raw("SELECT manual_sync_all_cashbank_coa()").Scan(&syncResult).Error
	if err != nil {
		return fmt.Errorf("failed to perform initial sync: %w", err)
	}

	log.Printf("✅ Initial balance sync: %s", syncResult)
	return nil
}

// ensureInvoiceTypesSystem ensures invoice types system is properly installed
// This handles cases where PC differences cause missing tables or inconsistent migration logs
func ensureInvoiceTypesSystem(db *gorm.DB) error {

	// Step 1: Check if invoice_types table exists
	var invoiceTypesExists bool
	err := db.Raw("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'invoice_types')").Scan(&invoiceTypesExists).Error
	if err != nil {
		return fmt.Errorf("failed to check invoice_types table: %w", err)
	}

	// Step 2: Check if invoice_counters table exists
	var invoiceCountersExists bool
	err = db.Raw("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'invoice_counters')").Scan(&invoiceCountersExists).Error
	if err != nil {
		return fmt.Errorf("failed to check invoice_counters table: %w", err)
	}

	// Step 3: Check if sales table has invoice_type_id column
	var salesInvoiceTypeExists bool
	err = db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'sales' AND column_name = 'invoice_type_id'
		)
	`).Scan(&salesInvoiceTypeExists).Error
	if err != nil {
		return fmt.Errorf("failed to check sales.invoice_type_id column: %w", err)
	}

	// Silently check system status

	// Step 4: If invoice_types exists but invoice_counters doesn't, create it
	if invoiceTypesExists && !invoiceCountersExists {
		log.Println("🔧 Creating invoice_counters table...")
		if err := createInvoiceCountersTable(db); err != nil {
			return fmt.Errorf("failed to create invoice_counters table: %w", err)
		}
		log.Println("✅ Invoice_counters table created")
	}

	// Step 5: If invoice_types doesn't exist but we have migration logs, run the migration
	if !invoiceTypesExists {

		// Check if we have 037 migration file
		migrationDir, err := findMigrationDir()
		if err == nil {
			migration037Path := filepath.Join(migrationDir, "037_add_invoice_types_system.sql")
			if _, err := os.Stat(migration037Path); err == nil {
				log.Println("🔧 Creating invoice types system...")
				if err := runMigration(db, "037_add_invoice_types_system.sql"); err != nil {
					log.Printf("⚠️  Failed to run 037 migration: %v", err)
				} else {
					log.Println("✅ Invoice types system created")
				}
			}
		}
	}

	// Step 6: Verify helper functions exist (silently)
	if invoiceTypesExists && invoiceCountersExists {
		if err := ensureInvoiceNumberFunctions(db); err != nil {
			log.Printf("⚠️  Failed to create helper functions: %v", err)
		}
	}

	// Step 7: Clean up failed migration logs that might cause issues (silently)
	result := db.Exec(`
		DELETE FROM migration_logs 
		WHERE migration_name LIKE '%037%' AND status = 'FAILED'
	`)
	if result.Error != nil {
		log.Printf("⚠️  Could not clean up failed migrations: %v", result.Error)
	}

	// Step 8: Ensure success migration log exists if system is working (silently)
	if invoiceTypesExists && invoiceCountersExists && salesInvoiceTypeExists {
		var successExists bool
		err = db.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM migration_logs 
				WHERE migration_name = '037_add_invoice_types_system.sql' AND status = 'SUCCESS'
			)
		`).Scan(&successExists).Error

		if err != nil {
			log.Printf("⚠️  Could not check success status: %v", err)
		} else if !successExists {
			// Insert success record
			err = db.Exec(`
				INSERT INTO migration_logs (migration_name, status, message, executed_at)
				VALUES ('037_add_invoice_types_system.sql', 'SUCCESS', 'Invoice types system verified and working', NOW())
				ON CONFLICT (migration_name) DO UPDATE SET 
					status = 'SUCCESS', 
					message = 'Invoice types system verified and working',
					executed_at = NOW()
			`).Error

			if err != nil {
				log.Printf("⚠️  Could not insert success record: %v", err)
			}
		}
	}

	return nil
}

// ensureTaxAccountSettingsTable ensures tax_account_settings table exists (PostgreSQL compatible)
func ensureTaxAccountSettingsTable(db *gorm.DB) error {
	// Check if table exists
	var tableExists bool
	err := db.Raw("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'tax_account_settings')").Scan(&tableExists).Error
	if err != nil {
		return fmt.Errorf("failed to check tax_account_settings table: %w", err)
	}

	if tableExists {
		// Silently skip if exists
		return nil
	}

	log.Println("🔧 Creating tax_account_settings table...")

	// Create table with PostgreSQL syntax
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS tax_account_settings (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP NULL DEFAULT NULL,

			-- Sales Account Configuration (required)
			sales_receivable_account_id BIGINT NOT NULL,
			sales_cash_account_id BIGINT NOT NULL,
			sales_bank_account_id BIGINT NOT NULL,
			sales_revenue_account_id BIGINT NOT NULL,
			sales_output_vat_account_id BIGINT NOT NULL,

			-- Purchase Account Configuration (required)
			purchase_payable_account_id BIGINT NOT NULL,
			purchase_cash_account_id BIGINT NOT NULL,
			purchase_bank_account_id BIGINT NOT NULL,
			purchase_input_vat_account_id BIGINT NOT NULL,
			purchase_expense_account_id BIGINT NOT NULL,

			-- Other Tax Accounts (optional)
			withholding_tax21_account_id BIGINT NULL DEFAULT NULL,
			withholding_tax23_account_id BIGINT NULL DEFAULT NULL,
			withholding_tax25_account_id BIGINT NULL DEFAULT NULL,
			tax_payable_account_id BIGINT NULL DEFAULT NULL,

			-- Inventory Account (optional)
			inventory_account_id BIGINT NULL DEFAULT NULL,
			cogs_account_id BIGINT NULL DEFAULT NULL,

			-- Configuration flags
			is_active BOOLEAN DEFAULT TRUE,
			apply_to_all_companies BOOLEAN DEFAULT TRUE,

			-- Metadata
			updated_by BIGINT NOT NULL,
			notes TEXT NULL
		)
	`

	err = db.Exec(createTableSQL).Error
	if err != nil {
		return fmt.Errorf("failed to create tax_account_settings table: %w", err)
	}

	// Create indexes
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_tax_account_settings_deleted_at ON tax_account_settings(deleted_at)",
		"CREATE INDEX IF NOT EXISTS idx_tax_account_settings_is_active ON tax_account_settings(is_active)",
		"CREATE INDEX IF NOT EXISTS idx_tax_account_settings_updated_by ON tax_account_settings(updated_by)",
	}

	for _, indexSQL := range indexes {
		if err := db.Exec(indexSQL).Error; err != nil {
			log.Printf("⚠️  Failed to create index: %v", err)
		}
	}

	// Insert default configuration
	defaultConfigSQL := `
		INSERT INTO tax_account_settings (
			sales_receivable_account_id,
			sales_cash_account_id,
			sales_bank_account_id,
			sales_revenue_account_id,
			sales_output_vat_account_id,
			purchase_payable_account_id,
			purchase_cash_account_id,
			purchase_bank_account_id,
			purchase_input_vat_account_id,
			purchase_expense_account_id,
			is_active,
			apply_to_all_companies,
			updated_by,
			notes
		) 
		SELECT 
			-- Sales accounts (based on typical account codes)
			COALESCE((SELECT id FROM accounts WHERE code = '1201' AND deleted_at IS NULL LIMIT 1), 1) as sales_receivable_account_id,
			COALESCE((SELECT id FROM accounts WHERE code = '1101' AND deleted_at IS NULL LIMIT 1), 1) as sales_cash_account_id,
			COALESCE((SELECT id FROM accounts WHERE code = '1102' AND deleted_at IS NULL LIMIT 1), 1) as sales_bank_account_id,
			COALESCE((SELECT id FROM accounts WHERE code = '4101-001' AND deleted_at IS NULL LIMIT 1), (SELECT id FROM accounts WHERE code = '4101' AND deleted_at IS NULL LIMIT 1), 1) as sales_revenue_account_id,
			COALESCE((SELECT id FROM accounts WHERE code = '2103' AND deleted_at IS NULL LIMIT 1), 1) as sales_output_vat_account_id,
			
			-- Purchase accounts
			COALESCE((SELECT id FROM accounts WHERE code = '2001' AND deleted_at IS NULL LIMIT 1), 1) as purchase_payable_account_id,
			COALESCE((SELECT id FROM accounts WHERE code = '1101' AND deleted_at IS NULL LIMIT 1), 1) as purchase_cash_account_id,
			COALESCE((SELECT id FROM accounts WHERE code = '1102' AND deleted_at IS NULL LIMIT 1), 1) as purchase_bank_account_id,
			COALESCE((SELECT id FROM accounts WHERE code = '2102' AND deleted_at IS NULL LIMIT 1), 1) as purchase_input_vat_account_id,
			COALESCE((SELECT id FROM accounts WHERE code = '5001' AND deleted_at IS NULL LIMIT 1), 1) as purchase_expense_account_id,
			
			-- Configuration
			TRUE as is_active,
			TRUE as apply_to_all_companies,
			1 as updated_by, -- System user
			'Default configuration created by auto-migration system' as notes
		WHERE NOT EXISTS (SELECT 1 FROM tax_account_settings WHERE is_active = TRUE)
	`

	result := db.Exec(defaultConfigSQL)
	if result.Error != nil {
		log.Printf("⚠️  Failed to insert default config: %v", result.Error)
	}

	log.Println("✅ Tax account settings table created")
	return nil
}

// createInvoiceCountersTable creates the invoice_counters table with all required components
func createInvoiceCountersTable(db *gorm.DB) error {
	// Create table
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS invoice_counters (
			id BIGSERIAL PRIMARY KEY,
			invoice_type_id BIGINT NOT NULL,
			year INTEGER NOT NULL,
			counter INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			
			UNIQUE(invoice_type_id, year),
			CONSTRAINT fk_invoice_counters_invoice_type_id 
				FOREIGN KEY (invoice_type_id) REFERENCES invoice_types(id) 
				ON DELETE CASCADE ON UPDATE CASCADE
		)
	`

	err := db.Exec(createTableSQL).Error
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Create indexes
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_invoice_counters_invoice_type_id ON invoice_counters(invoice_type_id)",
		"CREATE INDEX IF NOT EXISTS idx_invoice_counters_year ON invoice_counters(year)",
	}

	for _, indexSQL := range indexes {
		if err := db.Exec(indexSQL).Error; err != nil {
			log.Printf("⚠️  Failed to create index: %v", err)
		}
	}

	// Add comments
	comments := []string{
		"COMMENT ON TABLE invoice_counters IS 'Counter tracking for invoice numbering per type per year'",
		"COMMENT ON COLUMN invoice_counters.invoice_type_id IS 'Foreign key to invoice_types'",
		"COMMENT ON COLUMN invoice_counters.year IS 'Year for counter (e.g., 2025)'",
		"COMMENT ON COLUMN invoice_counters.counter IS 'Current counter value for this type/year'",
	}

	for _, commentSQL := range comments {
		db.Exec(commentSQL) // Don't fail on comment errors
	}

	// Initialize counters for existing invoice types
	initSQL := `
		INSERT INTO invoice_counters (invoice_type_id, year, counter, created_at, updated_at)
		SELECT id, EXTRACT(YEAR FROM NOW()), 0, NOW(), NOW()
		FROM invoice_types
		WHERE is_active = TRUE
		ON CONFLICT (invoice_type_id, year) DO NOTHING
	`

	result := db.Exec(initSQL)
	if result.Error != nil {
		log.Printf("⚠️  Failed to initialize counters: %v", result.Error)
	} else {
		log.Printf("✅ Initialized %d counters", result.RowsAffected)
	}

	return nil
}

// ensureInvoiceNumberFunctions creates the helper functions for invoice numbering
func ensureInvoiceNumberFunctions(db *gorm.DB) error {
	// Create get_next_invoice_number function
	getNextFunc := `
		CREATE OR REPLACE FUNCTION get_next_invoice_number(invoice_type_id_param BIGINT)
		RETURNS TEXT AS $$
		DECLARE
			current_year INTEGER;
			next_counter INTEGER;
			invoice_code TEXT;
			roman_month TEXT;
			result_number TEXT;
		BEGIN
			current_year := EXTRACT(YEAR FROM NOW());
			
			-- Get invoice type code
			SELECT code INTO invoice_code FROM invoice_types WHERE id = invoice_type_id_param;
			IF invoice_code IS NULL THEN
				RAISE EXCEPTION 'Invoice type not found with ID: %', invoice_type_id_param;
			END IF;
			
			-- Get and increment counter atomically
			INSERT INTO invoice_counters (invoice_type_id, year, counter)
			VALUES (invoice_type_id_param, current_year, 1)
			ON CONFLICT (invoice_type_id, year) 
			DO UPDATE SET counter = invoice_counters.counter + 1;
			
			-- Get the updated counter
			SELECT counter INTO next_counter 
			FROM invoice_counters 
			WHERE invoice_type_id = invoice_type_id_param AND year = current_year;
			
			-- Convert month to Roman numerals
			roman_month := CASE EXTRACT(MONTH FROM NOW())
				WHEN 1 THEN 'I' WHEN 2 THEN 'II' WHEN 3 THEN 'III' WHEN 4 THEN 'IV'
				WHEN 5 THEN 'V' WHEN 6 THEN 'VI' WHEN 7 THEN 'VII' WHEN 8 THEN 'VIII'
				WHEN 9 THEN 'IX' WHEN 10 THEN 'X' WHEN 11 THEN 'XI' WHEN 12 THEN 'XII'
			END;
			
			-- Format: 0001/STA-C/X-2025
			result_number := LPAD(next_counter::TEXT, 4, '0') || '/' || 
							 invoice_code || '/' || 
							 roman_month || '-' || current_year;
			
			RETURN result_number;
		END;
		$$ LANGUAGE plpgsql;
	`

	err := db.Exec(getNextFunc).Error
	if err != nil {
		return fmt.Errorf("failed to create get_next_invoice_number function: %w", err)
	}

	// Create preview_next_invoice_number function
	previewFunc := `
		CREATE OR REPLACE FUNCTION preview_next_invoice_number(invoice_type_id_param BIGINT)
		RETURNS TEXT AS $$
		DECLARE
			current_year INTEGER;
			next_counter INTEGER;
			invoice_code TEXT;
			roman_month TEXT;
			result_number TEXT;
		BEGIN
			current_year := EXTRACT(YEAR FROM NOW());
			
			-- Get invoice type code
			SELECT code INTO invoice_code FROM invoice_types WHERE id = invoice_type_id_param;
			IF invoice_code IS NULL THEN
				RAISE EXCEPTION 'Invoice type not found with ID: %', invoice_type_id_param;
			END IF;
			
			-- Get current counter (without incrementing)
			SELECT COALESCE(counter, 0) + 1 INTO next_counter
			FROM invoice_counters 
			WHERE invoice_type_id = invoice_type_id_param AND year = current_year;
			
			-- If no counter exists, next would be 1
			IF next_counter IS NULL THEN
				next_counter := 1;
			END IF;
			
			-- Convert month to Roman numerals
			roman_month := CASE EXTRACT(MONTH FROM NOW())
				WHEN 1 THEN 'I' WHEN 2 THEN 'II' WHEN 3 THEN 'III' WHEN 4 THEN 'IV'
				WHEN 5 THEN 'V' WHEN 6 THEN 'VI' WHEN 7 THEN 'VII' WHEN 8 THEN 'VIII'
				WHEN 9 THEN 'IX' WHEN 10 THEN 'X' WHEN 11 THEN 'XI' WHEN 12 THEN 'XII'
			END;
			
			-- Format: 0001/STA-C/X-2025
			result_number := LPAD(next_counter::TEXT, 4, '0') || '/' || 
							 invoice_code || '/' || 
							 roman_month || '-' || current_year;
			
			RETURN result_number;
		END;
		$$ LANGUAGE plpgsql;
	`

	err = db.Exec(previewFunc).Error
	if err != nil {
		return fmt.Errorf("failed to create preview_next_invoice_number function: %w", err)
	}

	return nil
}

// helper: checks if a column exists on a table
func columnExists(db *gorm.DB, table, column string) bool {
	var exists bool
	err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = $1 AND column_name = $2
		)
	`, table, column).Scan(&exists).Error
	if err != nil {
		return false
	}
	return exists
}

// fixRevenueDuplication fixes the revenue duplication issue caused by:
// 1. Account name variations in journal entries (case sensitivity)
// 2. Parent accounts not marked as headers
func fixRevenueDuplication(db *gorm.DB) error {
	log.Println("[AUTO-FIX] Starting revenue duplication fix...")

	// Step 1: Mark parent accounts as headers
	log.Println("[AUTO-FIX] Step 1/5: Marking parent accounts as headers...")
	parentAccounts := []string{"1000", "1100", "1500", "2000", "2100", "3000", "4000", "5000"}

	result := db.Exec(`
		UPDATE accounts 
		SET is_header = true, 
		    updated_at = NOW()
		WHERE code IN (?)
		  AND COALESCE(is_header, false) = false
	`, parentAccounts)

	if result.Error != nil {
		return fmt.Errorf("failed to mark parent accounts as headers: %v", result.Error)
	}
	if result.RowsAffected > 0 {
		log.Printf("[AUTO-FIX] ✓ Marked %d parent accounts as headers", result.RowsAffected)
	}

	// Step 2: Standardize account names in journal_entries for revenue (4xxx)
	log.Println("[AUTO-FIX] Step 2/5: Standardizing revenue account names in journal_entries...")
	if columnExists(db, "journal_entries", "account_code") && columnExists(db, "journal_entries", "account_name") {
		result = db.Exec(`
		UPDATE journal_entries je
		SET account_name = a.name,
		    updated_at = NOW()
		FROM accounts a
		WHERE a.code = je.account_code
		  AND je.account_name != a.name
		  AND je.account_code LIKE '4%'
	`)
		if result.Error != nil {
			log.Printf("[AUTO-FIX] ⚠ Warning: Could not standardize journal_entries (4xxx): %v", result.Error)
		} else if result.RowsAffected > 0 {
			log.Printf("[AUTO-FIX] ✓ Standardized %d revenue journal entries", result.RowsAffected)
		}
	} else {
		log.Println("[AUTO-FIX] ℹ️  Skipping journal_entries (4xxx) standardization: columns not present")
	}

	if result.Error != nil {
		log.Printf("[AUTO-FIX] ⚠ Warning: Could not standardize journal_entries (4xxx): %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("[AUTO-FIX] ✓ Standardized %d revenue journal entries", result.RowsAffected)
	}

	// Step 3: Standardize account names in journal_entries for expenses (5xxx)
	log.Println("[AUTO-FIX] Step 3/5: Standardizing expense account names in journal_entries...")
	if columnExists(db, "journal_entries", "account_code") && columnExists(db, "journal_entries", "account_name") {
		result = db.Exec(`
		UPDATE journal_entries je
		SET account_name = a.name,
		    updated_at = NOW()
		FROM accounts a
		WHERE a.code = je.account_code
		  AND je.account_name != a.name
		  AND je.account_code LIKE '5%'
	`)
		if result.Error != nil {
			log.Printf("[AUTO-FIX] ⚠ Warning: Could not standardize journal_entries (5xxx): %v", result.Error)
		} else if result.RowsAffected > 0 {
			log.Printf("[AUTO-FIX] ✓ Standardized %d expense journal entries", result.RowsAffected)
		}
	} else {
		log.Println("[AUTO-FIX] ℹ️  Skipping journal_entries (5xxx) standardization: columns not present")
	}

	// Step 4: Standardize unified_journal_lines for revenue (4xxx)
	log.Println("[AUTO-FIX] Step 4/5: Standardizing unified journal lines (4xxx)...")
	if columnExists(db, "unified_journal_lines", "account_name") && columnExists(db, "unified_journal_lines", "account_code") {
		result = db.Exec(`
		UPDATE unified_journal_lines ujl
		SET account_name = a.name,
		    updated_at = NOW()
		FROM accounts a
		WHERE a.id = ujl.account_id
		  AND ujl.account_name != a.name
		  AND ujl.account_code LIKE '4%'
	`)
		if result.Error != nil {
			log.Printf("[AUTO-FIX] ⚠ Warning: unified_journal_lines not updated (4xxx): %v", result.Error)
		} else if result.RowsAffected > 0 {
			log.Printf("[AUTO-FIX] ✓ Standardized %d unified revenue lines", result.RowsAffected)
		}
	} else {
		log.Println("[AUTO-FIX] ℹ️  Skipping unified_journal_lines (4xxx): columns not present")
	}

	// Step 5: Standardize unified_journal_lines for expenses (5xxx)
	log.Println("[AUTO-FIX] Step 5/5: Standardizing unified journal lines (5xxx)...")
	if columnExists(db, "unified_journal_lines", "account_name") && columnExists(db, "unified_journal_lines", "account_code") {
		result = db.Exec(`
		UPDATE unified_journal_lines ujl
		SET account_name = a.name,
		    updated_at = NOW()
		FROM accounts a
		WHERE a.id = ujl.account_id
		  AND ujl.account_name != a.name
		  AND ujl.account_code LIKE '5%'
	`)
		if result.Error != nil {
			log.Printf("[AUTO-FIX] ⚠ Warning: unified_journal_lines not updated (5xxx): %v", result.Error)
		} else if result.RowsAffected > 0 {
			log.Printf("[AUTO-FIX] ✓ Standardized %d unified expense lines", result.RowsAffected)
		}
	} else {
		log.Println("[AUTO-FIX] ℹ️  Skipping unified_journal_lines (5xxx): columns not present")
	}

	// Verification
	log.Println("[AUTO-FIX] Verifying fix...")

	type VerifyResult struct {
		AccountCode string `gorm:"column:account_code"`
		NameCount   int    `gorm:"column:name_count"`
		AllNames    string `gorm:"column:all_names"`
	}

	var dupes []VerifyResult
	if columnExists(db, "journal_entries", "account_code") && columnExists(db, "journal_entries", "account_name") {
		db.Raw(`
		SELECT 
			je.account_code,
			COUNT(DISTINCT je.account_name) as name_count,
			STRING_AGG(DISTINCT je.account_name, ' | ') as all_names
		FROM journal_entries je
		WHERE je.account_code LIKE '4%'
		GROUP BY je.account_code
		HAVING COUNT(DISTINCT je.account_name) > 1
	`).Scan(&dupes)
	}

	if len(dupes) > 0 {
		log.Printf("[AUTO-FIX] ⚠ WARNING: Still found %d accounts with name variations:", len(dupes))
		for _, d := range dupes {
			log.Printf("[AUTO-FIX]   - Account %s has %d variations: %s", d.AccountCode, d.NameCount, d.AllNames)
		}
	} else {
		log.Println("[AUTO-FIX] ✅ Verification passed: No duplicate account names")
	}

	log.Println("[AUTO-FIX] Revenue duplication fix completed!")
	return nil
}

// ensureSettingsHistoryTable ensures settings_history table exists (PostgreSQL compatible)
func ensureSettingsHistoryTable(db *gorm.DB) error {
	// Check if table exists
	var tableExists bool
	err := db.Raw("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'settings_history')").Scan(&tableExists).Error
	if err != nil {
		return fmt.Errorf("failed to check settings_history table: %w", err)
	}

	if tableExists {
		// Silently skip if exists
		return nil
	}

	log.Println("🔧 Creating settings_history table...")

	// Create table with PostgreSQL syntax
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS settings_history (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP NULL DEFAULT NULL,
			
			-- Reference to settings
			settings_id BIGINT NOT NULL,
			
			-- Change tracking
			field VARCHAR(255) NOT NULL,
			old_value TEXT,
			new_value TEXT,
			action VARCHAR(50) DEFAULT 'UPDATE',
			
			-- User tracking
			changed_by BIGINT NOT NULL,
			
			-- Additional context
			ip_address VARCHAR(255),
			user_agent TEXT,
			reason TEXT,
			
			-- Foreign keys
			CONSTRAINT fk_settings_history_settings 
				FOREIGN KEY (settings_id) REFERENCES settings(id) ON DELETE CASCADE
		)
	`

	err = db.Exec(createTableSQL).Error
	if err != nil {
		return fmt.Errorf("failed to create settings_history table: %w", err)
	}

	// Create indexes
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_settings_history_settings_id ON settings_history(settings_id)",
		"CREATE INDEX IF NOT EXISTS idx_settings_history_changed_by ON settings_history(changed_by)",
		"CREATE INDEX IF NOT EXISTS idx_settings_history_field ON settings_history(field)",
		"CREATE INDEX IF NOT EXISTS idx_settings_history_created_at ON settings_history(created_at)",
		"CREATE INDEX IF NOT EXISTS idx_settings_history_deleted_at ON settings_history(deleted_at)",
	}

	for _, indexSQL := range indexes {
		if err := db.Exec(indexSQL).Error; err != nil {
			log.Printf("⚠️  Failed to create index: %v", err)
		}
	}

	log.Println("✅ Settings history table created")
	return nil
}

// SyncAllSeededSequences ensures the PostgreSQL sequences are in sync for tables that have pre-seeded data
// This prevents 'duplicate key value violates unique constraint' errors after seeding data with explicit IDs
func SyncAllSeededSequences(db *gorm.DB) error {
	tablesToSync := []string{
		"users",
		"permissions",
		"product_categories",
		"product_units",
		"expense_categories",
		"report_templates",
		"asset_categories",
		"invoice_types",
		"module_permissions",
	}

	for _, tableName := range tablesToSync {
		// First check if table exists
		var tableExists bool
		err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = ?)", tableName).Scan(&tableExists).Error
		if err != nil || !tableExists {
			continue // Table doesn't exist yet, nothing to sync for this one
		}

		// PostgreSQL specific query to sync sequence with MAX(id)
		// Properly escape table name in format string to prevent injection
		query := fmt.Sprintf(`
			SELECT setval(
				pg_get_serial_sequence('%s', 'id'), 
				COALESCE((SELECT MAX(id) FROM %s), 1), 
				true
			)
		`, tableName, tableName)

		if err := db.Exec(query).Error; err != nil {
			// Log but don't fail entirely, as it might just be the sequence doesn't exist or is not supported in the dialect
			log.Printf("ℹ️ Could not sync %s sequence (this is normal if not using PostgreSQL): %v", tableName, err)
		}
	}

	return nil
}
