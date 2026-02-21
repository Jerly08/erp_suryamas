package main

import (
	"encoding/json"
	"log"
	"os"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

func main() {
	config.LoadConfig()
	db := database.ConnectDB()

	var result map[string]interface{} = make(map[string]interface{})

	err := db.Transaction(func(tx *gorm.DB) error {
		var jes []models.SSOTJournalEntry
		tx.Where("id IN (31, 32)").Find(&jes)

		var ppList []map[string]interface{}
		for _, je := range jes {
			var pp models.PurchasePayment
			tx.First(&pp, *je.SourceID)

			ppList = append(ppList, map[string]interface{}{
				"journal_id":  je.ID,
				"payment_id":  pp.ID,
				"purchase_id": pp.PurchaseID,
				"amount":      pp.Amount,
			})
		}
		result["payments"] = ppList

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	os.WriteFile("diagnose_payments.json", b, 0644)
}
