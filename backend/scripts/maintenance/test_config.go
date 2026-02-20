package main

import (
	"log"
	"os"
	"regexp"
	"strings"
	
	"github.com/joho/godotenv"
)

func parsePostgresURL(dbURL string) (host, port, user, password, dbname string) {
	re := regexp.MustCompile(`postgres(?:ql)?://([^:]+):([^@]+)@([^:/]+)(?::(\d+))?/([^?]+)`)
	matches := re.FindStringSubmatch(dbURL)
	
	if len(matches) >= 6 {
		user = matches[1]
		password = matches[2]
		host = matches[3]
		if matches[4] != "" {
			port = matches[4]
		} else {
			port = "5432"
		}
		dbname = matches[5]
	}
	return
}

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Printf("⚠️ Warning: .env file not found")
	}
	
	log.Println("═══════════════════════════════════════════════════════════")
	log.Println("🔍 DATABASE CONFIGURATION TEST")
	log.Println("═══════════════════════════════════════════════════════════")
	log.Println("")
	
	// Test DATABASE_URL
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		log.Println("✅ DATABASE_URL found in environment")
		
		if strings.HasPrefix(dbURL, "postgres") {
			host, port, user, password, dbname := parsePostgresURL(dbURL)
			
			// Mask password
			maskedPassword := strings.Repeat("*", len(password))
			
			log.Printf("   URL: %s", dbURL)
			log.Printf("   User: %s", user)
			log.Printf("   Password: %s", maskedPassword)
			log.Printf("   Host: %s", host)
			log.Printf("   Port: %s", port)
			log.Printf("   Database: %s", dbname)
		}
	} else {
		log.Println("❌ DATABASE_URL not found")
		log.Println("")
		log.Println("Checking individual variables:")
		
		vars := []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME"}
		for _, v := range vars {
			value := os.Getenv(v)
			if value != "" {
				if v == "DB_PASSWORD" {
					value = strings.Repeat("*", len(value))
				}
				log.Printf("   ✅ %s = %s", v, value)
			} else {
				log.Printf("   ❌ %s = (not set)", v)
			}
		}
	}
	
	log.Println("")
	log.Println("═══════════════════════════════════════════════════════════")
	log.Println("")
	
	// Test with different URL formats
	testURLs := []string{
		"postgres://user1:pass1@host1/db1",
		"postgres://user2:pass2@host2:5433/db2",
		"postgresql://user3:pass3@host3/db3?sslmode=disable",
		"postgresql://user4:pass4@host4:3306/db4?sslmode=require",
	}
	
	log.Println("🧪 Testing URL Parser with different formats:")
	log.Println("")
	
	for i, url := range testURLs {
		host, port, user, _, dbname := parsePostgresURL(url)
		log.Printf("%d. %s", i+1, url)
		log.Printf("   → user=%s, host=%s, port=%s, db=%s", user, host, port, dbname)
		log.Println("")
	}
	
	log.Println("✅ Configuration test complete!")
}