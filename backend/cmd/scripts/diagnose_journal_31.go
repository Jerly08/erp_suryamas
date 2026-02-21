package main

import (
	"fmt"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"
)

func main() {
	config.LoadConfig()
	db := database.ConnectDB()

	var je models.SSOTJournalEntry
	if err := db.Unscoped().First(&je, 31).Error; err != nil {
		fmt.Println("JE 31 not found")
		return
	}

	fmt.Printf("Journal 31: Status=%s, SourceType=%s, SourceID=%v, DeletedAt=%v\n", je.Status, je.SourceType, *je.SourceID, je.DeletedAt != nil)

	if je.SourceType == "PAYMENT" || je.SourceType == "PURCHASE_PAYMENT" {
		var p models.Payment
		if err := db.Unscoped().First(&p, *je.SourceID).Error; err != nil {
			fmt.Println("Payment not found!")
		} else {
			fmt.Printf("Payment %d: Status=%s, Amount=%.2f, Deleted=%v\n", p.ID, p.Status, p.Amount, p.DeletedAt.Valid)
		}
	}
}
