package main

import (
	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"
	"fmt"
	"log"
)

func main() {
	config.LoadConfig()
	db := database.ConnectDB()
	if db == nil {
		log.Fatalf("Failed to connect DB")
	}

	// 1. Find a purchase that is APPROVED
	var purchase models.Purchase
	if err := db.Preload("Vendor").Preload("PurchaseItems").Preload("PurchaseItems.Product").
		Where("status IN ('APPROVED', 'COMPLETED', 'PAID')").
		Order("id DESC").First(&purchase).Error; err != nil {
		log.Fatalf("No approved purchase found: %v", err)
	}

	fmt.Printf("Found Purchase ID %d (Status: %s)\n", purchase.ID, purchase.Status)

	// 2. Initialize SSOT journal service
	coaService := services.NewCOAService(db)
	settingsService := services.NewSettingsService(db)
	purchaseJournalServiceSSOT := services.NewPurchaseJournalServiceSSOT(db, coaService, settingsService)

	// Delete existing journal to test fresh creation
	db.Exec("DELETE FROM unified_journal_lines WHERE journal_id IN (SELECT id FROM unified_journal_ledger WHERE source_type = 'PURCHASE' AND source_id = ?)", purchase.ID)
	db.Exec("DELETE FROM unified_journal_ledger WHERE source_type = 'PURCHASE' AND source_id = ?", purchase.ID)

	// 3. Try to create the journal
	fmt.Println("Attempting to create journal entry...")
	err := purchaseJournalServiceSSOT.CreatePurchaseJournal(&purchase, nil)
	if err != nil {
		fmt.Printf("ERROR creating journal: %v\n", err)
	} else {
		fmt.Println("SUCCESS creating journal!")

		// Let's check if it actually exists and has lines
		var count int64
		db.Table("unified_journal_ledger").
			Where("source_type = 'PURCHASE' AND source_id = ?", purchase.ID).
			Count(&count)
		fmt.Printf("Found %d journal entries in unified_journal_ledger for this purchase.\n", count)

		var lineCount int64
		db.Table("unified_journal_lines").
			Where("journal_id IN (SELECT id FROM unified_journal_ledger WHERE source_type = 'PURCHASE' AND source_id = ?)", purchase.ID).
			Count(&lineCount)
		fmt.Printf("Found %d NON-DELETED journal lines for this purchase!\n", lineCount)
	}
}
