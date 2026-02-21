package main

import (
	"fmt"
	"log"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"
)

func main() {
	config.LoadConfig()
	db := database.ConnectDB()

	var piutang models.Account
	if err := db.Where("code = ?", "1102-001").First(&piutang).Error; err != nil {
		log.Fatalf("Could not find Piutang account: %v", err)
	}

	fmt.Printf("Piutang (1102-001):\n  ID: %d\n  Name: %s\n  Balance: %.2f\n", piutang.ID, piutang.Name, piutang.Balance)

	// Since we know earlier that journal line 18 in ID 27 was updated to 137... let's check what it actually is now
	var line models.SSOTJournalLine
	if err := db.Where("journal_id = ? AND credit_amount > 0", 27).First(&line).Error; err == nil {
		fmt.Printf("\nJournal Line ID %d (Credit: %.2f) is currently pointing to Account ID: %d\n", line.ID, line.CreditAmount.InexactFloat64(), line.AccountID)
	}
}
