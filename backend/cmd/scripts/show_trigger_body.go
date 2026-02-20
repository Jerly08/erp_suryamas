package main

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	dsn := "host=localhost user=postgres password=postgres dbname=sistem_akuntansi port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		log.Fatalf("Failed: %v", err)
	}

	// Just get the update_account_balance function
	var src string
	db.Raw(`SELECT p.prosrc FROM pg_proc p WHERE p.proname = 'update_account_balance'`).Scan(&src)
	fmt.Println("=== update_account_balance ===")
	fmt.Println(src)

	fmt.Println("=============================")

	// Get sync_account_balance function
	var src2 string
	db.Raw(`SELECT p.prosrc FROM pg_proc p WHERE p.proname = 'sync_account_balance_on_status_change'`).Scan(&src2)
	fmt.Println("=== sync_account_balance_on_status_change ===")
	fmt.Println(src2)
}
