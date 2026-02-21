package main

import (
	"fmt"
	"log"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"

	"gorm.io/gorm"
)

func main() {
	config.LoadConfig()
	db := database.ConnectDB()

	err := db.Transaction(func(tx *gorm.DB) error {
		var uje []map[string]interface{}
		tx.Table("unified_journal_ledger").
			Select("id, entry_number, entry_date, status, is_balanced, created_at, deleted_at").
			Where("id = ?", 27).
			Find(&uje)

		if len(uje) == 0 {
			fmt.Println("Journal 27 not found!")
		} else {
			for _, j := range uje {
				fmt.Printf("Journal 27: %v\n", j)
			}
		}

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
