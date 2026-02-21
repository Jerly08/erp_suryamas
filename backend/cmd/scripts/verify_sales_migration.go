package main

import (
	"fmt"
	"log"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"
)

func main() {
	cfg := config.LoadConfig()
	_ = cfg
	db := database.ConnectDB()

	var count int64
	err := db.Model(&models.SSOTJournalEntry{}).
		Where("source_type IN (?, ?)", "SALES", "SALES_PAYMENT").
		Count(&count).Error

	if err != nil {
		log.Fatalf("Error querying DB: %v", err)
	}

	fmt.Printf("\n==================================\n")
	fmt.Printf("✅ VERIFICATION SUCCESSFUL\n")
	fmt.Printf("Found %d SALES and SALES_PAYMENT entries in unified_journal_ledger\n", count)
	fmt.Printf("==================================\n")
}
