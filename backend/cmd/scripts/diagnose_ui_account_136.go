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
		var a models.Account
		// Use Unscoped to see if it's deleted
		if err := tx.Unscoped().First(&a, 136).Error; err != nil {
			return err
		}

		fmt.Printf("Account 136 Details:\n")
		fmt.Printf("Code: %s\n", a.Code)
		fmt.Printf("Name: %s\n", a.Name)
		fmt.Printf("Type: %s\n", a.Type)
		fmt.Printf("IsHeader: %v\n", a.IsHeader)
		fmt.Printf("Balance: %.2f\n", a.Balance)
		fmt.Printf("DeletedAt: %v\n", a.DeletedAt)

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
