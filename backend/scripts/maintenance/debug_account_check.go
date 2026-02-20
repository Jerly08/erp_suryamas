package main

import (
	"fmt"
	"log"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/models"
	"gorm.io/gorm"
)

func main() {
	// Initialize database connection
	db, err := config.InitDB()
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Check specific account
	var account models.Account
	result := db.Where("code = ? OR name LIKE ?", "2101", "%UTANG USAHA%").First(&account)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			fmt.Println("Account not found")
			return
		}
		log.Fatal("Database query error:", result.Error)
	}

	fmt.Printf("Account Details:\n")
	fmt.Printf("ID: %d\n", account.ID)
	fmt.Printf("Code: %s\n", account.Code)
	fmt.Printf("Name: %s\n", account.Name)
	fmt.Printf("Type: %s\n", account.Type)
	fmt.Printf("Balance: %.2f\n", account.Balance)
	fmt.Printf("IsActive: %t\n", account.IsActive)
	fmt.Printf("IsHeader: %t\n", account.IsHeader)
}