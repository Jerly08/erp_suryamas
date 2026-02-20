package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"app-sistem-akuntansi/services"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: Error loading .env file, assuming env vars are set")
	}

	// Connect to DB using DATABASE_URL or component parts
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Jakarta",
			os.Getenv("DB_HOST"),
			os.Getenv("DB_USER"),
			os.Getenv("DB_PASSWORD"),
			os.Getenv("DB_NAME"),
			os.Getenv("DB_PORT"),
		)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Initialize Service
	closingService := services.NewUnifiedPeriodClosingService(db)
	ctx := context.Background()

	// 1. Get Last Closing Info
	fmt.Println("--- 1. Checking Last Closing Info ---")
	lastInfo, err := closingService.GetLastClosingInfo(ctx)
	if err != nil {
		fmt.Printf("Error getting last closing info: %v\n", err)
		// Assuming start from beginning if error
	} else {
		jsonInfo, _ := json.MarshalIndent(lastInfo, "", "  ")
		fmt.Printf("Last Closing Info: %s\n", string(jsonInfo))
	}

	var startDate, endDate time.Time

	if lastInfo != nil && lastInfo["next_start_date"] != nil {
		startDateStr := lastInfo["next_start_date"].(string)
		startDate, _ = time.Parse("2006-01-02", startDateStr)
	} else {
		// Default to first day of current year if no closing found
		startDate = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		if lastInfo != nil && lastInfo["period_start_date"] != nil {
			startDateStr := lastInfo["period_start_date"].(string)
			startDate, _ = time.Parse("2006-01-02", startDateStr)
		}
	}

	// Set End Date to end of that month (or today if earlier)
	endDate = startDate.AddDate(0, 1, -1) // Incorrect logic for end of month?
	// Better: Start Date + 1 month - 1 day?
	// Let's just pick a reasonable period: 1 month from start
	endDate = startDate.AddDate(0, 1, 0).Add(-24 * time.Hour)

	fmt.Printf("\n--- 2. Simulating Closing Preview for Period: %s to %s ---\n",
		startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	preview, err := closingService.PreviewPeriodClosing(ctx, startDate, endDate)
	if err != nil {
		log.Fatalf("❌ PREVIEW FAILED: %v", err)
	}

	// Verify Results
	fmt.Println("✅ Preview Calculated Successfully!")
	fmt.Printf("Total Revenue: %.2f\n", preview["total_revenue"])
	fmt.Printf("Total Expense: %.2f\n", preview["total_expense"])
	fmt.Printf("Net Income   : %.2f\n", preview["net_income"])
	fmt.Printf("Can Close    : %v\n", preview["can_close"])

	// Check validations
	if valMsgs, ok := preview["validation_messages"].([]string); ok && len(valMsgs) > 0 {
		fmt.Println("⚠️ Validation Messages:")
		for _, msg := range valMsgs {
			fmt.Printf(" - %s\n", msg)
		}
	}

	// Check Closing Entries
	if _, ok := preview["closing_entries"].([]services.ClosingEntryPreview); ok { // Need to check type capability
		// The service returns []models.ClosingEntryPreview actually but as interface{}
		// We'll just marshal the whole preview to see it
	}

	// Just dump the preview as JSON for inspection
	jsonPreview, _ := json.MarshalIndent(preview, "", "  ")
	fmt.Printf("\n--- Full Preview Output ---\n%s\n", string(jsonPreview))

	fmt.Println("\n✅ CONCLUSION: Service logic executes without runtime errors.")
}
