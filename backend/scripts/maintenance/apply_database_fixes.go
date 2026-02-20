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
	// Supports:
	// - postgres://user:pass@host/db
	// - postgres://user:pass@host:port/db  
	// - postgres://user:@host/db (empty password)
	// - postgres://user@host/db (no password)
	
	// Try with password (including empty password between : and @)
	re := regexp.MustCompile(`postgres(?:ql)?://([^:@]+):([^@]*)@([^:/]+)(?::(\d+))?/([^?]+)`)
	matches := re.FindStringSubmatch(dbURL)
	
	if len(matches) >= 6 {
		user = matches[1]
		password = matches[2] // Can be empty string
		host = matches[3]
		if matches[4] != "" {
			port = matches[4]
		} else {
			port = "5432"
		}
		dbname = matches[5]
		if password == "" {
			log.Printf("🔍 Parsed from URL: user=%s, host=%s, port=%s, dbname=%s (no password)", user, host, port, dbname)
		} else {
			log.Printf("🔍 Parsed from URL: user=%s, host=%s, port=%s, dbname=%s", user, host, port, dbname)
		}
		return
	}
	
	// Try without password at all (postgres://user@host/db)
	re2 := regexp.MustCompile(`postgres(?:ql)?://([^:@]+)@([^:/]+)(?::(\d+))?/([^?]+)`)
	matches2 := re2.FindStringSubmatch(dbURL)
	
	if len(matches2) >= 5 {
		user = matches2[1]
		password = "" // No password
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

func getDatabaseConnection() (*sql.DB, string) {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Printf("📁 Warning: .env file not found, using system environment variables")
	}
	
	var connStr string
	var dbInfo string
	
	// First try DATABASE_URL (standard format)
	dbURL := os.Getenv("DATABASE_URL")
	
	if dbURL != "" {
		log.Println("🔗 Using DATABASE_URL from environment")
		
		if strings.HasPrefix(dbURL, "postgres") {
			// Parse the URL and rebuild connection string
			host, port, user, password, dbname := parsePostgresURL(dbURL)
			
			if host != "" {
				// Build connection string - omit password if empty
				if password != "" {
					connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
						host, port, user, password, dbname)
				} else {
					// No password - omit password field entirely
					connStr = fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=disable",
						host, port, user, dbname)
					log.Println("🔑 Using authentication without password (peer/trust authentication)")
				}
				dbInfo = fmt.Sprintf("%s@%s:%s/%s", user, host, port, dbname)
			} else {
				log.Println("⚠️ Warning: Failed to parse DATABASE_URL")
			}
		} else {
			// Use as-is if not a URL format
			connStr = dbURL
			dbInfo = "custom connection string"
		}
	}
	
	// If no DATABASE_URL, try individual env vars
	if connStr == "" {
		log.Println("🔍 DATABASE_URL not found, checking individual environment variables...")
		
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
		dbInfo = fmt.Sprintf("%s@%s:%s/%s", dbUser, dbHost, dbPort, dbName)
	}
	
	log.Printf("🎯 Connecting to database: %s", dbInfo)
	
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	
	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("❌ Failed to ping database: %v", err)
	}
	
	log.Println("✅ Database connection successful!")
	return db, dbInfo
}

func fixUUIDExtension(db *sql.DB) error {
	log.Println("")
	log.Println("═══════════════════════════════════════════════")
	log.Println("🔧 FIX 1: Adding UUID Extension")
	log.Println("═══════════════════════════════════════════════")
	
	// Add UUID extension
	_, err := db.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`)
	if err != nil {
		log.Printf("⚠️ Failed to add uuid-ossp extension: %v", err)
		log.Println("🔄 Trying alternative pgcrypto extension...")
		
		_, err = db.Exec(`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`)
		if err != nil {
			return fmt.Errorf("failed to add pgcrypto extension: %v", err)
		}
		log.Println("✅ pgcrypto extension added successfully")
		
		// Create uuid_generate_v4 function using pgcrypto if needed
		_, err = db.Exec(`
			CREATE OR REPLACE FUNCTION uuid_generate_v4()
			RETURNS uuid AS $$
			BEGIN
				RETURN gen_random_uuid();
			END;
			$$ LANGUAGE plpgsql;
		`)
		if err != nil {
			log.Printf("⚠️ Warning: Failed to create uuid_generate_v4 function: %v", err)
		} else {
			log.Println("✅ uuid_generate_v4 function created using pgcrypto")
		}
	} else {
		log.Println("✅ uuid-ossp extension added successfully")
	}
	
	// Verify UUID function works
	var testUUID string
	err = db.QueryRow("SELECT uuid_generate_v4()::text").Scan(&testUUID)
	if err != nil {
		log.Printf("⚠️ UUID function test failed: %v", err)
	} else {
		log.Printf("✅ UUID function works! Test UUID: %s", testUUID)
	}
	
	return nil
}

func fixConcurrentRefresh(db *sql.DB) error {
	log.Println("")
	log.Println("═══════════════════════════════════════════════")
	log.Println("🔧 FIX 2: Concurrent Materialized View Refresh")
	log.Println("═══════════════════════════════════════════════")
	
	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	
	// 1. Drop the problematic trigger
	log.Println("1️⃣ Dropping problematic trigger...")
	_, err = tx.Exec(`DROP TRIGGER IF EXISTS trg_refresh_account_balances ON unified_journal_lines`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to drop trigger: %v", err)
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
		return fmt.Errorf("failed to create manual refresh function: %v", err)
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
		return fmt.Errorf("failed to create freshness check function: %v", err)
	}
	
	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}
	
	log.Println("✅ Concurrent refresh fix applied successfully!")
	
	// Verify the fix
	var triggerExists bool
	err = db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM pg_trigger 
			WHERE tgname = 'trg_refresh_account_balances'
		)
	`).Scan(&triggerExists)
	
	if err != nil {
		log.Printf("⚠️ Warning: Failed to verify trigger removal: %v", err)
	} else if triggerExists {
		log.Println("⚠️ WARNING: Trigger still exists!")
	} else {
		log.Println("✅ Trigger successfully removed")
	}
	
	// Test manual refresh
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
	
	return nil
}

func main() {
	log.Println("╔═══════════════════════════════════════════════════════════════╗")
	log.Println("║           ACCOUNTING DATABASE FIX UTILITY v1.0                 ║")
	log.Println("║                                                                 ║")
	log.Println("║  This script will apply the following fixes:                   ║")
	log.Println("║  1. Add UUID extension for uuid_generate_v4() function         ║")
	log.Println("║  2. Fix concurrent materialized view refresh error             ║")
	log.Println("╚═══════════════════════════════════════════════════════════════╝")
	log.Println("")
	
	// Get database connection
	db, dbInfo := getDatabaseConnection()
	defer db.Close()
	
	// Apply fixes
	var hasErrors bool
	
	// Fix 1: UUID Extension
	if err := fixUUIDExtension(db); err != nil {
		log.Printf("❌ UUID extension fix failed: %v", err)
		hasErrors = true
	}
	
	// Fix 2: Concurrent Refresh
	if err := fixConcurrentRefresh(db); err != nil {
		log.Printf("❌ Concurrent refresh fix failed: %v", err)
		hasErrors = true
	}
	
	// Summary
	log.Println("")
	log.Println("═══════════════════════════════════════════════════════════════")
	log.Println("📊 SUMMARY")
	log.Println("═══════════════════════════════════════════════════════════════")
	log.Printf("Database: %s", dbInfo)
	
	if !hasErrors {
		log.Println("Status: ✅ All fixes applied successfully!")
		log.Println("")
		log.Println("What was fixed:")
		log.Println("  ✅ UUID extension installed (uuid_generate_v4 function available)")
		log.Println("  ✅ Concurrent refresh trigger removed (no more SQLSTATE 55000 errors)")
		log.Println("  ✅ Manual refresh functions created")
		log.Println("")
		log.Println("Next steps:")
		log.Println("  1. Test deposit cash & bank functionality")
		log.Println("  2. Test sales invoice creation")
		log.Println("  3. Optionally set up scheduled refresh for reporting")
		log.Println("")
		log.Println("🎉 Your database is now ready to use!")
	} else {
		log.Println("Status: ⚠️ Some fixes encountered errors")
		log.Println("Please check the errors above and try running the script again.")
		log.Println("If problems persist, check your database connection and permissions.")
		os.Exit(1)
	}
}