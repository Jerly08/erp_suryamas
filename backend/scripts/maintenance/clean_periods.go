package main

import (
	"fmt"
	"log"
	
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	// Connect to database
	dsn := "host=localhost user=postgres password=postgres dbname=sistem_akuntansi port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	
	fmt.Println("🔗 Connected to database")
	
	// Count before deletion
	var countBefore int64
	db.Table("accounting_periods").Count(&countBefore)
	fmt.Printf("📊 Found %d records in accounting_periods table\n", countBefore)
	
	if countBefore == 0 {
		fmt.Println("✅ Table is already empty. Nothing to clean.")
		return
	}
	
	// Confirm deletion
	fmt.Printf("\n⚠️  WARNING: About to delete %d records from accounting_periods table!\n", countBefore)
	fmt.Println("   This action cannot be undone.")
	fmt.Print("\nProceed with deletion? (yes/no): ")
	
	var confirmation string
	fmt.Scanln(&confirmation)
	
	if confirmation != "yes" {
		fmt.Println("❌ Deletion cancelled by user.")
		return
	}
	
	// Delete all records
	result := db.Exec("DELETE FROM accounting_periods")
	if result.Error != nil {
		log.Fatalf("❌ Failed to delete records: %v", result.Error)
	}
	
	fmt.Printf("✅ Deleted %d records successfully\n", result.RowsAffected)
	
	// Reset sequence
	err = db.Exec("ALTER SEQUENCE accounting_periods_id_seq RESTART WITH 1").Error
	if err != nil {
		log.Printf("⚠️  Warning: Failed to reset sequence: %v", err)
	} else {
		fmt.Println("✅ Reset ID sequence to 1")
	}
	
	// Verify
	var countAfter int64
	db.Table("accounting_periods").Count(&countAfter)
	fmt.Printf("📊 Records remaining: %d\n", countAfter)
	
	if countAfter == 0 {
		fmt.Println("\n🎉 Cleanup completed successfully!")
		fmt.Println("   The accounting_periods table is now empty and ready for real data.")
	}
}
