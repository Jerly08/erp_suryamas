package main

import (
	"fmt"
	"log"
	"regexp"
)

func parsePostgresURL(dbURL string) (host, port, user, password, dbname string) {
	log.Printf("Input URL: %s", dbURL)
	
	// Parse postgres://user:password@host:port/dbname?params (supports empty password)
	re := regexp.MustCompile(`postgres(?:ql)?://([^:@]+):([^@]*)@([^:/]+)(?::(\d+))?/([^?]+)`)
	matches := re.FindStringSubmatch(dbURL)
	
	log.Printf("Regex matches count: %d", len(matches))
	for i, m := range matches {
		log.Printf("  Match[%d]: %s", i, m)
	}
	
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
		log.Printf("✅ Parsed successfully")
		log.Printf("   user=%s, password=%s, host=%s, port=%s, dbname=%s", user, password, host, port, dbname)
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
		log.Printf("✅ Parsed successfully (no password)")
		log.Printf("   user=%s, password=<empty>, host=%s, port=%s, dbname=%s", user, host, port, dbname)
		return
	}
	
	log.Printf("❌ Failed to parse - not enough matches")
	return
}

func main() {
	testURLs := []string{
		"postgres://postgres:postgres@localhost/sistem_akuntansi?sslmode=disable",
		"postgres://arkaan:@localhost/sistem_akuntansi?sslmode=disable", // Empty password
		"postgres://user1:pass1@host1/db1",
		"postgres://user2:pass2@host2:5433/db2",
		"postgresql://user3:pass3@host3/db3?sslmode=disable",
		"postgres://testuser@localhost:5432/testdb", // No password field
	}
	
	for i, url := range testURLs {
		fmt.Printf("\n════════════════════════════════════════\n")
		fmt.Printf("Test %d:\n", i+1)
		fmt.Printf("════════════════════════════════════════\n")
		parsePostgresURL(url)
	}
}