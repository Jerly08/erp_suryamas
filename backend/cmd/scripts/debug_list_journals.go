package main

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
    "app-sistem-akuntansi/models"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=sistem_akuntansi port=5432 sslmode=disable TimeZone=Asia/Jakarta"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatal(err)
	}

	var entries []models.SSOTJournalEntry
	db.Order("id desc").Limit(5).Find(&entries)

	fmt.Printf("Found %d entries\n", len(entries))
	for _, e := range entries {
		fmt.Printf("ID: %d | No: %s | Date: %s | Status: %s | Debit: %s | CreatedBy: %d | Source: %s\n", 
            e.ID, e.EntryNumber, e.EntryDate.Format("2006-01-02"), e.Status, e.TotalDebit, e.CreatedBy, e.SourceType)
	}
}
