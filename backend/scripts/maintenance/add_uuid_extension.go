package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
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
				// dbInfo := fmt.Sprintf("%s@%s:%s/%s", user, host, port, dbname)
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

	log.Println("🔧 Adding UUID extension to database...")

	// Add UUID extension
	_, err = db.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`)
	if err != nil {
		log.Printf("Warning: Failed to add uuid-ossp extension: %v", err)
		log.Println("Trying alternative pgcrypto extension...")

		_, err = db.Exec(`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`)
		if err != nil {
			log.Fatalf("Failed to add pgcrypto extension: %v", err)
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
			log.Printf("Warning: Failed to create uuid_generate_v4 function: %v", err)
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

	log.Println("")
	log.Println("🎉 UUID extension setup complete!")
}
