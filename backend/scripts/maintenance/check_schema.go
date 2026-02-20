package main

import (
	"fmt"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/config"
)

func main() {
	// Load configuration  
	_ = config.LoadConfig()
	
	// Connect to database
	db := database.ConnectDB()
	
	fmt.Println("🔍 Checking table schema and data...")
	
	// Check cash_bank_transactions table structure
	fmt.Println("\n1️⃣ CASH_BANK_TRANSACTIONS table structure:")
	var columns []struct {
		ColumnName string `json:"column_name"`
		DataType   string `json:"data_type"`
	}
	
	db.Raw(`
		SELECT column_name, data_type 
		FROM information_schema.columns 
		WHERE table_name = 'cash_bank_transactions'
		ORDER BY ordinal_position
	`).Scan(&columns)
	
	for _, col := range columns {
		fmt.Printf("   %s (%s)\n", col.ColumnName, col.DataType)
	}
	
	// Check if there are any transactions
	fmt.Println("\n2️⃣ Check all cash bank transactions:")
	var txs []map[string]interface{}
	
	db.Raw("SELECT * FROM cash_bank_transactions LIMIT 5").Scan(&txs)
	
	if len(txs) == 0 {
		fmt.Println("   No transactions found in table")
	} else {
		for i, tx := range txs {
			fmt.Printf("   Transaction %d: %+v\n", i+1, tx)
		}
	}
	
	// Check cash_banks table structure
	fmt.Println("\n3️⃣ CASH_BANKS table structure:")
	var bankColumns []struct {
		ColumnName string `json:"column_name"`
		DataType   string `json:"data_type"`
	}
	
	db.Raw(`
		SELECT column_name, data_type 
		FROM information_schema.columns 
		WHERE table_name = 'cash_banks'
		ORDER BY ordinal_position
	`).Scan(&bankColumns)
	
	for _, col := range bankColumns {
		fmt.Printf("   %s (%s)\n", col.ColumnName, col.DataType)
	}
	
	// Check bank account data
	fmt.Println("\n4️⃣ Bank account 7 data:")
	var bankData map[string]interface{}
	
	db.Raw("SELECT * FROM cash_banks WHERE id = 7").Scan(&bankData)
	fmt.Printf("   Bank 7: %+v\n", bankData)
}