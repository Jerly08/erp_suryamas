package main

import (
	"fmt"
	"log"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

func main() {
	config.LoadConfig()
	db := database.ConnectDB()

	err := db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.SSOTJournalEntry{}).Where("source_type = 'SALES'").Count(&count).Error; err != nil {
			return err
		}

		fmt.Printf("Total SALES journals in unified_journal_ledger: %d\n", count)

		if count == 0 {
			// Find how many sales exist
			var salesCount int64
			tx.Model(&models.Sale{}).Count(&salesCount)
			fmt.Printf("Total Sales in DB: %d\n", salesCount)

			// Find if there are ANY journals related to sales (maybe different source_type?)
			var anyCount int64
			tx.Model(&models.SSOTJournalEntry{}).Where("entry_number LIKE 'SALES-%'").Count(&anyCount)
			fmt.Printf("Total journals with 'SALES-' prefix: %d\n", anyCount)

			// Check what legacy journals were mapped to in unified ledger?
			// Is there a legacy journal table still?
			var tableExists int64
			tx.Raw("SELECT count(*) FROM information_schema.tables WHERE table_name = 'journals'").Scan(&tableExists)
			fmt.Printf("Does legacy 'journals' table exist? %v\n", tableExists > 0)

			if tableExists > 0 {
				var legacyCount int64
				tx.Raw("SELECT count(*) FROM journals WHERE reference_type = 'SALE'").Scan(&legacyCount)
				fmt.Printf("Count of SALE journals in legacy 'journals' table: %d\n", legacyCount)
			}
		} else {
			var journals []models.SSOTJournalEntry
			tx.Where("source_type = 'SALES'").Find(&journals)
			for _, j := range journals {
				fmt.Printf("Found SALES Journal: ID %d | Reference: %s\n", j.ID, j.Reference)
			}
		}

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
