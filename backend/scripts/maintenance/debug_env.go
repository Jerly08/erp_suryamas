package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	
	"github.com/joho/godotenv"
)

func main() {
	log.Println("🔍 Debugging .env loading...")
	log.Println("")
	
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Printf("❌ Failed to get current directory: %v", err)
	} else {
		log.Printf("📁 Current working directory: %s", cwd)
	}
	
	// Check if .env file exists
	envPath := filepath.Join(cwd, ".env")
	log.Printf("🔍 Looking for .env at: %s", envPath)
	
	if _, err := os.Stat(envPath); err == nil {
		log.Println("✅ .env file exists")
		
		// Read file content
		content, err := os.ReadFile(envPath)
		if err != nil {
			log.Printf("❌ Failed to read .env: %v", err)
		} else {
			log.Println("📄 .env file content:")
			log.Println(string(content))
		}
	} else {
		log.Println("❌ .env file NOT found!")
	}
	
	log.Println("")
	log.Println("🔧 Attempting to load .env...")
	
	// Try loading .env
	err = godotenv.Load()
	if err != nil {
		log.Printf("⚠️ godotenv.Load() failed: %v", err)
		
		// Try explicit path
		log.Println("🔄 Trying explicit path...")
		err = godotenv.Load(envPath)
		if err != nil {
			log.Printf("❌ godotenv.Load(explicit) also failed: %v", err)
		} else {
			log.Println("✅ godotenv.Load(explicit) succeeded!")
		}
	} else {
		log.Println("✅ godotenv.Load() succeeded!")
	}
	
	log.Println("")
	log.Println("📊 Environment variables after loading:")
	
	vars := []string{"DATABASE_URL", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME"}
	for _, v := range vars {
		value := os.Getenv(v)
		if value != "" {
			if v == "DB_PASSWORD" || v == "DATABASE_URL" {
				// Mask sensitive info
				if len(value) > 20 {
					value = value[:20] + "..."
				}
			}
			log.Printf("   %s = %s", v, value)
		} else {
			log.Printf("   %s = (not set)", v)
		}
	}
	
	log.Println("")
	
	// Test getting DATABASE_URL
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		log.Printf("✅ Successfully got DATABASE_URL: %s", dbURL)
	} else {
		log.Println("❌ DATABASE_URL is empty after loading!")
		log.Println("")
		log.Println("🔍 Checking all environment variables:")
		for _, env := range os.Environ() {
			fmt.Println("  ", env)
		}
	}
}