package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	
	_ "github.com/lib/pq"
	"github.com/joho/godotenv"
)

func parsePostgresURL(dbURL string) (host, port, user, password, dbname string) {
	// Parse postgres://user:password@host:port/dbname?params
	// Supports empty password: postgres://user:@host/db
	re := regexp.MustCompile(`postgres(?:ql)?://([^:@]+):([^@]*)@([^:/]+)(?::(\d+))?/([^?]+)`)
	matches := re.FindStringSubmatch(dbURL)
	
	if len(matches) >= 6 {
		user = matches[1]
		password = matches[2] // Can be empty
		host = matches[3]
		if matches[4] != "" {
			port = matches[4]
		} else {
			port = "5432"
		}
		dbname = matches[5]
		log.Printf("🔍 Parsed from URL: user=%s, host=%s, port=%s, dbname=%s", user, host, port, dbname)
		return
	}
	// Try without password field
	re2 := regexp.MustCompile(`postgres(?:ql)?://([^:@]+)@([^:/]+)(?::(\d+))?/([^?]+)`)
	matches2 := re2.FindStringSubmatch(dbURL)
	if len(matches2) >= 5 {
		user = matches2[1]
		password = ""
		host = matches2[2]
		if matches2[3] != "" {
			port = matches2[3]
		} else {
			port = "5432"
		}
		dbname = matches2[4]
		log.Printf("🔍 Parsed from URL: user=%s, host=%s, port=%s, dbname=%s (no password)", user, host, port, dbname)
	}
	return
}

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found, using system environment variables")
	}
	
	var connStr string
	
	// First try DATABASE_URL (standard format)
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		log.Println("Using DATABASE_URL from environment")
		
		if strings.HasPrefix(dbURL, "postgres") {
			// Parse the URL and rebuild connection string
			host, port, user, password, dbname := parsePostgresURL(dbURL)
			if host != "" {
				if password != "" {
					connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
						host, port, user, password, dbname)
				} else {
					connStr = fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=disable",
						host, port, user, dbname)
				}
				dbInfo = fmt.Sprintf("%s@%s:%s/%s", user, host, port, dbname)
			}
		} else {
			// Use as-is if not a URL format
			connStr = dbURL
		}
	}
	
	// If no DATABASE_URL, try individual env vars
	if connStr == "" {
		log.Println("DATABASE_URL not found, using individual environment variables")
		
		dbHost := os.Getenv("DB_HOST")
		if dbHost == "" {
			dbHost = "localhost"
		}
		dbPort := os.Getenv("DB_PORT")
		if dbPort == "" {
			dbPort = "5432"
		}
		dbUser := os.Getenv("DB_USER")
		if dbUser == "" {
			dbUser = "postgres"
		}
		dbPassword := os.Getenv("DB_PASSWORD")
		if dbPassword == "" {
			dbPassword = "postgres"
		}
		dbName := os.Getenv("DB_NAME")
		if dbName == "" {
			dbName = "accounting_db"
		}
		
		connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			dbHost, dbPort, dbUser, dbPassword, dbName)
		log.Printf("Connecting to database: %s@%s:%s/%s", dbUser, dbHost, dbPort, dbName)
	}
	
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	
	log.Println("🔧 Fixing concurrent materialized view refresh error...")
	
	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("Failed to begin transaction: %v", err)
	}
	
	// 1. Drop the problematic trigger
	log.Println("1️⃣ Dropping problematic trigger...")
	_, err = tx.Exec(`DROP TRIGGER IF EXISTS trg_refresh_account_balances ON unified_journal_lines`)
	if err != nil {
		tx.Rollback()
		log.Fatalf("Failed to drop trigger: %v", err)
	}
	
	// 2. Create manual refresh function
	log.Println("2️⃣ Creating manual refresh function...")
	_, err = tx.Exec(`
		CREATE OR REPLACE FUNCTION manual_refresh_account_balances()
		RETURNS TABLE(
			success BOOLEAN,
			message TEXT,
			refreshed_at TIMESTAMPTZ
		) AS $$
		DECLARE
			start_time TIMESTAMPTZ;
			end_time TIMESTAMPTZ;
			duration INTERVAL;
		BEGIN
			start_time := clock_timestamp();
			
			-- Refresh the materialized view
			REFRESH MATERIALIZED VIEW account_balances;
			
			end_time := clock_timestamp();
			duration := end_time - start_time;
			
			RETURN QUERY SELECT 
				TRUE as success,
				format('Account balances refreshed in %s', duration) as message,
				end_time as refreshed_at;
				
		EXCEPTION WHEN OTHERS THEN
			RETURN QUERY SELECT 
				FALSE as success,
				format('Refresh failed: %s', SQLERRM) as message,
				clock_timestamp() as refreshed_at;
		END;
		$$ LANGUAGE plpgsql
	`)
	if err != nil {
		tx.Rollback()
		log.Fatalf("Failed to create manual refresh function: %v", err)
	}
	
	// 3. Create freshness check function
	log.Println("3️⃣ Creating freshness check function...")
	_, err = tx.Exec(`
		CREATE OR REPLACE FUNCTION check_account_balances_freshness()
		RETURNS TABLE(
			last_updated TIMESTAMPTZ,
			age_minutes INTEGER,
			needs_refresh BOOLEAN
		) AS $$
		DECLARE
			last_update TIMESTAMPTZ;
			age_mins INTEGER;
		BEGIN
			-- Get the last_updated timestamp from the materialized view
			SELECT MAX(ab.last_updated) INTO last_update
			FROM account_balances ab;
			
			-- Calculate age in minutes
			age_mins := EXTRACT(EPOCH FROM (NOW() - last_update)) / 60;
			
			RETURN QUERY SELECT 
				last_update,
				age_mins,
				age_mins > 60 as needs_refresh; -- Suggest refresh if older than 1 hour
		END;
		$$ LANGUAGE plpgsql
	`)
	if err != nil {
		tx.Rollback()
		log.Fatalf("Failed to create freshness check function: %v", err)
	}
	
	// 4. Add comment
	log.Println("4️⃣ Adding function comments...")
	_, err = tx.Exec(`
		COMMENT ON FUNCTION manual_refresh_account_balances() IS 
		'Manually refresh account_balances materialized view. Use this for scheduled jobs or API calls.'
	`)
	if err != nil {
		log.Printf("Warning: Failed to add comment to manual_refresh_account_balances: %v", err)
	}
	
	_, err = tx.Exec(`
		COMMENT ON FUNCTION check_account_balances_freshness() IS 
		'Check how old the account_balances materialized view is and if it needs refresh.'
	`)
	if err != nil {
		log.Printf("Warning: Failed to add comment to check_account_balances_freshness: %v", err)
	}
	
	// 5. Log the migration
	log.Println("5️⃣ Logging migration...")
	_, err = tx.Exec(`
		INSERT INTO migration_logs (migration_name, status, executed_at, message, description) 
		VALUES (
			'fix_concurrent_refresh_error', 
			'SUCCESS', 
			NOW(), 
			'Fixed SQLSTATE 55000 error by removing auto-refresh trigger. Use manual_refresh_account_balances() for scheduled refresh.',
			'Removed trg_refresh_account_balances trigger. Added manual_refresh_account_balances() and check_account_balances_freshness() functions.'
		) ON CONFLICT (migration_name) DO UPDATE SET
			status = 'SUCCESS',
			executed_at = NOW(),
			message = 'Fixed SQLSTATE 55000 error by removing auto-refresh trigger. Use manual_refresh_account_balances() for scheduled refresh.'
	`)
	if err != nil {
		log.Printf("Warning: Failed to log migration (table might not exist): %v", err)
	}
	
	// Commit transaction
	if err = tx.Commit(); err != nil {
		log.Fatalf("Failed to commit transaction: %v", err)
	}
	
	log.Println("✅ Fix applied successfully!")
	log.Println("")
	log.Println("=========================================================================")
	log.Println("✅ CONCURRENT REFRESH ERROR FIX APPLIED SUCCESSFULLY")
	log.Println("=========================================================================")
	log.Println("")
	log.Println("What was fixed:")
	log.Println("  - Removed trigger: trg_refresh_account_balances")
	log.Println("  - This eliminates SQLSTATE 55000 concurrent refresh conflicts")
	log.Println("")
	log.Println("Balance synchronization:")
	log.Println("  - Real-time sync: Handled by setup_automatic_balance_sync.sql triggers")
	log.Println("  - accounts.balance field is always up-to-date")
	log.Println("")
	log.Println("Materialized view refresh (for reporting):")
	log.Println("  - Manual refresh: SELECT * FROM manual_refresh_account_balances();")
	log.Println("  - Check freshness: SELECT * FROM check_account_balances_freshness();")
	log.Println("  - Recommended: Schedule refresh every 1 hour via cron or API")
	log.Println("=========================================================================")
	
	// Verify the fix
	log.Println("")
	log.Println("🔍 Verifying the fix...")
	
	var triggerExists bool
	err = db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM pg_trigger 
			WHERE tgname = 'trg_refresh_account_balances'
		)
	`).Scan(&triggerExists)
	
	if err != nil {
		log.Printf("Warning: Failed to verify trigger removal: %v", err)
	} else if triggerExists {
		log.Println("⚠️ WARNING: Trigger still exists!")
	} else {
		log.Println("✅ Trigger successfully removed")
	}
	
	// Test manual refresh function
	log.Println("")
	log.Println("🔄 Testing manual refresh function...")
	
	var success bool
	var message string
	err = db.QueryRow("SELECT success, message FROM manual_refresh_account_balances()").Scan(&success, &message)
	if err != nil {
		log.Printf("⚠️ Manual refresh test failed: %v", err)
	} else if success {
		log.Printf("✅ Manual refresh successful: %s", message)
	} else {
		log.Printf("❌ Manual refresh failed: %s", message)
	}
	
	log.Println("")
	log.Println("🎉 Fix complete! The concurrent refresh error should be resolved.")
	log.Println("📝 Note: You may want to set up a scheduled job to refresh the materialized view periodically.")
}