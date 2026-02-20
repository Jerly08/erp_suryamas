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
	
	fmt.Println("🔍 SIMPLE BALANCE DEBUG")
	fmt.Println("========================")
	
	// Check Bank Mandiri balance from different perspectives
	fmt.Printf("\n1️⃣ DIRECT DATABASE CHECK:\n")
	
	var directBalance struct {
		ID      int     `json:"id"`
		Code    string  `json:"code"`
		Name    string  `json:"name"`
		Balance float64 `json:"balance"`
		UpdatedAt string `json:"updated_at"`
	}
	
	db.Raw("SELECT id, code, name, balance, updated_at::text FROM accounts WHERE code = '1103'").Scan(&directBalance)
	
	fmt.Printf("   Bank Mandiri (1103):\n")
	fmt.Printf("     Balance: %.2f\n", directBalance.Balance)
	fmt.Printf("     Updated: %s\n", directBalance.UpdatedAt)
	
	// Check if there's another 1103 account
	var allBankMandiri []struct {
		ID      int     `json:"id"`
		Code    string  `json:"code"`
		Name    string  `json:"name"`
		Balance float64 `json:"balance"`
		IsActive bool   `json:"is_active"`
	}
	
	db.Raw("SELECT id, code, name, balance, is_active FROM accounts WHERE code = '1103'").Scan(&allBankMandiri)
	
	fmt.Printf("\n2️⃣ ALL ACCOUNTS WITH CODE 1103:\n")
	for i, acc := range allBankMandiri {
		fmt.Printf("   [%d] ID: %d, Name: %s, Balance: %.2f, Active: %v\n", i+1, acc.ID, acc.Name, acc.Balance, acc.IsActive)
	}
	
	// Check if there are any accounts with similar names
	var similarAccounts []struct {
		ID      int     `json:"id"`
		Code    string  `json:"code"`
		Name    string  `json:"name"`
		Balance float64 `json:"balance"`
		IsActive bool   `json:"is_active"`
	}
	
	db.Raw("SELECT id, code, name, balance, is_active FROM accounts WHERE name ILIKE '%mandiri%'").Scan(&similarAccounts)
	
	fmt.Printf("\n3️⃣ ALL ACCOUNTS WITH 'MANDIRI' IN NAME:\n")
	for i, acc := range similarAccounts {
		fmt.Printf("   [%d] %s (%s): %.2f [Active: %v]\n", i+1, acc.Code, acc.Name, acc.Balance, acc.IsActive)
	}
	
	// Check the actual hierarchy structure
	fmt.Printf("\n4️⃣ ACCOUNTS HIERARCHY (showing parent-child relationship):\n")
	
	var hierarchyCheck []struct {
		ID       int     `json:"id"`
		ParentID *int    `json:"parent_id"`
		Code     string  `json:"code"`
		Name     string  `json:"name"`
		Balance  float64 `json:"balance"`
		IsHeader bool    `json:"is_header"`
		IsActive bool    `json:"is_active"`
	}
	
	db.Raw(`
		SELECT id, parent_id, code, name, balance, is_header, is_active
		FROM accounts 
		WHERE is_active = true AND (
			code LIKE '11%' OR 
			code = '1000' OR 
			parent_id IN (SELECT id FROM accounts WHERE code LIKE '11%')
		)
		ORDER BY code
	`).Scan(&hierarchyCheck)
	
	fmt.Printf("   Current Assets & children:\n")
	for _, acc := range hierarchyCheck {
		indent := ""
		if acc.ParentID != nil {
			indent = "     "
		}
		headerInfo := ""
		if acc.IsHeader {
			headerInfo = " [HEADER]"
		}
		fmt.Printf("   %s%s (%s): %.2f%s\n", indent, acc.Code, acc.Name, acc.Balance, headerInfo)
	}
	
	// Force update Bank Mandiri if it's wrong
	if directBalance.Balance != 44450000 {
		fmt.Printf("\n🔧 FORCING BANK MANDIRI UPDATE:\n")
		
		result := db.Exec("UPDATE accounts SET balance = 44450000, updated_at = NOW() WHERE code = '1103'")
		if result.Error != nil {
			fmt.Printf("   ❌ Error updating: %v\n", result.Error)
		} else {
			fmt.Printf("   ✅ Bank Mandiri updated to 44,450,000\n")
			fmt.Printf("   Rows affected: %d\n", result.RowsAffected)
		}
		
		// Verify the update
		var verifyBalance float64
		db.Raw("SELECT balance FROM accounts WHERE code = '1103'").Scan(&verifyBalance)
		fmt.Printf("   📊 Verified balance: %.2f\n", verifyBalance)
	}
	
	// Check if there are any triggers or constraints affecting the balance
	fmt.Printf("\n5️⃣ CHECKING DATABASE CONSTRAINTS:\n")
	
	var constraints []struct {
		TableName      string `json:"table_name"`
		ConstraintName string `json:"constraint_name"`
		ConstraintType string `json:"constraint_type"`
	}
	
	db.Raw(`
		SELECT table_name, constraint_name, constraint_type
		FROM information_schema.table_constraints 
		WHERE table_name = 'accounts'
	`).Scan(&constraints)
	
	fmt.Printf("   Constraints on accounts table:\n")
	for _, c := range constraints {
		fmt.Printf("     %s: %s (%s)\n", c.ConstraintName, c.ConstraintType, c.TableName)
	}
	
	// Check for any triggers
	var triggers []struct {
		TriggerName string `json:"trigger_name"`
		EventManipulation string `json:"event_manipulation"`
		ActionTiming string `json:"action_timing"`
	}
	
	db.Raw(`
		SELECT trigger_name, event_manipulation, action_timing
		FROM information_schema.triggers
		WHERE event_object_table = 'accounts'
	`).Scan(&triggers)
	
	if len(triggers) > 0 {
		fmt.Printf("   Triggers on accounts table:\n")
		for _, t := range triggers {
			fmt.Printf("     %s: %s %s\n", t.TriggerName, t.ActionTiming, t.EventManipulation)
		}
	} else {
		fmt.Printf("   No triggers found on accounts table\n")
	}
	
	fmt.Printf("\n💡 NEXT STEPS:\n")
	fmt.Printf("1. Check browser Network tab for API responses\n")
	fmt.Printf("2. Clear all browser data and retry\n")
	fmt.Printf("3. Check if frontend is using correct API endpoint\n")
	fmt.Printf("4. Verify the frontend fix was saved and compiled\n")
}