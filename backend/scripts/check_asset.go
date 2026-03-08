// +build ignore

package main

import (
	"app-sistem-akuntansi/database"
	"fmt"
	"log"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	db := database.ConnectDB()

	var result struct{
		ID uint
		Code string
		Name string
		Category string
		DepAccountID *uint `gorm:"column:depreciation_account_id"`
		AccAccountID *uint `gorm:"column:accumulated_depreciation_account_id"`
		DepCode string
		AccCode string
	}

	err := db.Raw(`
		SELECT 
			a.id, a.code, a.name, a.category,
			a.depreciation_account_id, a.accumulated_depreciation_account_id,
			d.code as dep_code, acc.code as acc_code
		FROM assets a 
		LEFT JOIN accounts d ON d.id = a.depreciation_account_id 
		LEFT JOIN accounts acc ON acc.id = a.accumulated_depreciation_account_id 
		WHERE a.code = 'AS-2026-002'
	`).Scan(&result).Error

	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Printf("Asset ID: %d\n", result.ID)
	fmt.Printf("Code: %s\n", result.Code)
	
	if result.DepAccountID != nil {
		fmt.Printf("Depreciation Account ID: %d (Code: %s)\n", *result.DepAccountID, result.DepCode)
	} else {
		fmt.Printf("Depreciation Account ID: NULL\n")
	}

	// Fix the typo in the previous query: it's accumulated_depreciation_account_id in the struct but column is?
	// Let's check the schema.
}
