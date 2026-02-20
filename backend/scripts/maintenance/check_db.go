package main

import (
	"fmt"
	"log"
	"os"
	
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type AccountingPeriod struct {
	ID          uint      `gorm:"primaryKey"`
	StartDate   string    `gorm:"column:start_date"`
	EndDate     string    `gorm:"column:end_date"`
	IsClosed    bool      `gorm:"column:is_closed"`
	IsLocked    bool      `gorm:"column:is_locked"`
	Description string    `gorm:"column:description"`
	CreatedAt   string    `gorm:"column:created_at"`
}

func main() {
	// Database connection string - adjust as needed
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=sistem_akuntansi port=5432 sslmode=disable"
	}
	
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	
	// Check if table exists
	var tableExists bool
	err = db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'accounting_periods')").Scan(&tableExists).Error
	if err != nil {
		log.Fatalf("Error checking table existence: %v", err)
	}
	
	fmt.Printf("📊 Table 'accounting_periods' exists: %v\n", tableExists)
	
	if !tableExists {
		fmt.Println("❌ Table does not exist yet. Need to run migration 041_create_accounting_periods_table.sql")
		return
	}
	
	// Count records
	var count int64
	err = db.Table("accounting_periods").Count(&count).Error
	if err != nil {
		log.Fatalf("Error counting records: %v", err)
	}
	
	fmt.Printf("📝 Total records in accounting_periods: %d\n", count)
	
	if count == 0 {
		fmt.Println("✅ Table is empty. No previous closing data.")
		return
	}
	
	// Get closed periods
	var periods []AccountingPeriod
	err = db.Table("accounting_periods").
		Where("is_closed = ?", true).
		Order("end_date DESC").
		Limit(10).
		Find(&periods).Error
	
	if err != nil {
		log.Fatalf("Error fetching periods: %v", err)
	}
	
	fmt.Printf("\n🔍 Found %d closed periods:\n", len(periods))
	fmt.Println("=========================================")
	for i, p := range periods {
		fmt.Printf("%d. ID: %d\n", i+1, p.ID)
		fmt.Printf("   Start: %s\n", p.StartDate)
		fmt.Printf("   End: %s\n", p.EndDate)
		fmt.Printf("   Closed: %v, Locked: %v\n", p.IsClosed, p.IsLocked)
		fmt.Printf("   Description: %s\n", p.Description)
		fmt.Printf("   Created: %s\n", p.CreatedAt)
		fmt.Println("   ---------------------------------")
	}
}
