package main

import (
	"encoding/json"
	"fmt"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"
)

func main() {
	config.LoadConfig()
	db := database.ConnectDB()

	var j models.SSOTJournalEntry
	if err := db.First(&j, 6).Error; err != nil {
		fmt.Printf("Journal 6 not found: %v\n", err)
		return
	}

	b, _ := json.MarshalIndent(j, "", "  ")
	fmt.Println("Journal 6:")
	fmt.Println(string(b))

	var lines []models.SSOTJournalLine
	db.Where("journal_id = ?", 6).Find(&lines)
	b, _ = json.MarshalIndent(lines, "", "  ")
	fmt.Println("\nLines:")
	fmt.Println(string(b))

	// Also check the corresponding Sale
	var sale models.Sale
	if err := db.Preload("SaleItems").Where("code = ?", "SOA-00062").First(&sale).Error; err != nil {
		fmt.Printf("\nSale SOA-00062 not found: %v\n", err)
	} else {
		b, _ = json.MarshalIndent(sale, "", "  ")
		fmt.Println("\nSale SOA-00062:")
		fmt.Println(string(b))
	}
}
