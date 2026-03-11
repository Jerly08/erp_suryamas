package main

import (
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"
	"fmt"
	"log"
)

func main() {
	db := database.ConnectDB()

	// Update all DRAFT journals from PREPAID_AMORTIZATION to POSTED
	result := db.Model(&models.SSOTJournalEntry{}).
		Where("source_type = ? AND status = ?", "PREPAID_AMORTIZATION", "DRAFT").
		Update("status", "POSTED")

	if result.Error != nil {
		log.Fatalf("❌ Error updating draft journals: %v", result.Error)
	}

	fmt.Printf("✅ Successfully updated %d DRAFT amortization journals to POSTED.\n", result.RowsAffected)
}
