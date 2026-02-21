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
		var lines []models.SSOTJournalLine
		tx.Preload("Account").Where("journal_id IN (3, 5)").Find(&lines)

		var lList []map[string]interface{}
		for _, l := range lines {
			lList = append(lList, map[string]interface{}{
				"journal_id":   l.JournalID,
				"account_code": l.Account.Code,
				"account_name": l.Account.Name,
				"debit":        l.DebitAmount,
				"credit":       l.CreditAmount,
			})
		}
		result["lines"] = lList

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	os.WriteFile("diagnose_lines.json", b, 0644)
}
