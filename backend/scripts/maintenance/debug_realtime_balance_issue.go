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
	
	fmt.Println("🔍 REAL-TIME BALANCE DEBUG")
	fmt.Println("===========================")
	
	// Check current database balances
	fmt.Printf("\n1️⃣ CURRENT DATABASE BALANCES:\n")
	
	var bankMandiri struct {
		ID      int     `json:"id"`
		Code    string  `json:"code"`
		Name    string  `json:"name"`
		Balance float64 `json:"balance"`
	}
	
	db.Raw("SELECT id, code, name, balance FROM accounts WHERE code = '1103'").Scan(&bankMandiri)
	
	fmt.Printf("   Bank Mandiri (1103):\n")
	fmt.Printf("     ID: %d\n", bankMandiri.ID)
	fmt.Printf("     Balance: %.2f\n", bankMandiri.Balance)
	
	// Check CURRENT ASSETS
	var currentAssets struct {
		ID      int     `json:"id"`
		Code    string  `json:"code"`
		Name    string  `json:"name"`
		Balance float64 `json:"balance"`
	}
	
	db.Raw("SELECT id, code, name, balance FROM accounts WHERE code = '1100'").Scan(&currentAssets)
	
	fmt.Printf("   CURRENT ASSETS (1100):\n")
	fmt.Printf("     ID: %d\n", currentAssets.ID)
	fmt.Printf("     Balance: %.2f\n", currentAssets.Balance)
	
	// Check what the hierarchy endpoint returns
	fmt.Printf("\n2️⃣ HIERARCHY ENDPOINT DATA:\n")
	
	var hierarchyAccounts []map[string]interface{}
	db.Raw(`
		WITH RECURSIVE account_tree AS (
			SELECT id, parent_id, code, name, type, balance, is_header, is_active, 
			       0 as level, ARRAY[code] as path
			FROM accounts 
			WHERE parent_id IS NULL AND is_active = true
			
			UNION ALL
			
			SELECT a.id, a.parent_id, a.code, a.name, a.type, a.balance, a.is_header, a.is_active,
			       at.level + 1, at.path || a.code
			FROM accounts a
			INNER JOIN account_tree at ON a.parent_id = at.id
			WHERE a.is_active = true
		)
		SELECT id, parent_id, code, name, type, balance, is_header, level
		FROM account_tree 
		WHERE code IN ('1000', '1100', '1103', '1240', '1301')
		ORDER BY path
	`).Scan(&hierarchyAccounts)
	
	for _, acc := range hierarchyAccounts {
		fmt.Printf("   %s (%v): %.2f [header: %v]\n", 
			acc["code"], acc["name"], acc["balance"], acc["is_header"])
	}
	
	// Check Cash & Bank Management balance
	fmt.Printf("\n3️⃣ CASH & BANK MANAGEMENT:\n")
	
	var cashBankBalance float64
	db.Raw("SELECT balance FROM cash_banks WHERE account_id = ?", bankMandiri.ID).Scan(&cashBankBalance)
	
	fmt.Printf("   Cash & Bank Management for Bank Mandiri: %.2f\n", cashBankBalance)
	
	// Check if there's a discrepancy
	if bankMandiri.Balance != 44450000 {
		fmt.Printf("\n🚨 DISCREPANCY DETECTED:\n")
		fmt.Printf("   Expected: 44,450,000\n")
		fmt.Printf("   Actual: %.0f\n", bankMandiri.Balance)
		fmt.Printf("   Difference: %.0f\n", 44450000 - bankMandiri.Balance)
		
		// Check recent account updates
		fmt.Printf("\n4️⃣ RECENT ACCOUNT UPDATES:\n")
		var recentUpdates []struct {
			Code      string `json:"code"`
			Name      string `json:"name"`
			Balance   float64 `json:"balance"`
			UpdatedAt string `json:"updated_at"`
		}
		
		db.Raw(`
			SELECT code, name, balance, updated_at::text
			FROM accounts 
			WHERE code IN ('1103', '1100', '1000')
			ORDER BY updated_at DESC
		`).Scan(&recentUpdates)
		
		for _, update := range recentUpdates {
			fmt.Printf("   %s: %.2f (updated: %s)\n", update.Code, update.Balance, update.UpdatedAt)
		}
		
		// Force update Bank Mandiri to correct balance
		fmt.Printf("\n🔧 FORCING BALANCE CORRECTION:\n")
		err := db.Exec("UPDATE accounts SET balance = 44450000 WHERE code = '1103'").Error
		if err != nil {
			fmt.Printf("   ❌ Error updating: %v\n", err)
		} else {
			fmt.Printf("   ✅ Bank Mandiri balance updated to 44,450,000\n")
		}
		
		// Update CURRENT ASSETS to match children
		fmt.Printf("   🔧 Updating CURRENT ASSETS...\n")
		err = db.Exec(`
			UPDATE accounts SET balance = (
				SELECT COALESCE(SUM(balance), 0)
				FROM accounts 
				WHERE code LIKE '11%' AND code != '1100' AND LENGTH(code) = 4
			) WHERE code = '1100'
		`).Error
		if err != nil {
			fmt.Printf("   ❌ Error updating CURRENT ASSETS: %v\n", err)
		} else {
			fmt.Printf("   ✅ CURRENT ASSETS balance updated\n")
		}
		
		// Update TOTAL ASSETS  
		fmt.Printf("   🔧 Updating TOTAL ASSETS...\n")
		err = db.Exec(`
			UPDATE accounts SET balance = (
				SELECT COALESCE(SUM(balance), 0)
				FROM accounts 
				WHERE code LIKE '1%' AND code != '1000' AND LENGTH(code) = 4
			) WHERE code = '1000'
		`).Error
		if err != nil {
			fmt.Printf("   ❌ Error updating TOTAL ASSETS: %v\n", err)
		} else {
			fmt.Printf("   ✅ TOTAL ASSETS balance updated\n")
		}
		
		// Check updated balances
		fmt.Printf("\n5️⃣ UPDATED BALANCES:\n")
		db.Raw("SELECT code, name, balance FROM accounts WHERE code IN ('1000', '1100', '1103') ORDER BY code").Scan(&hierarchyAccounts)
		for _, acc := range hierarchyAccounts {
			fmt.Printf("   %s (%v): %.2f\n", acc["code"], acc["name"], acc["balance"])
		}
	} else {
		fmt.Printf("\n✅ DATABASE BALANCE IS CORRECT\n")
		fmt.Printf("   Problem might be in frontend cache or API response\n")
	}
	
	fmt.Printf("\n💡 FRONTEND DEBUGGING STEPS:\n")
	fmt.Printf("1. Open Chrome DevTools (F12)\n")
	fmt.Printf("2. Go to Application tab → Storage → Clear all data\n")
	fmt.Printf("3. Go to Network tab → Check 'Disable cache'\n")
	fmt.Printf("4. Hard refresh: Ctrl+Shift+R\n")
	fmt.Printf("5. Check Network tab for API calls to /api/v1/accounts\n")
	fmt.Printf("6. Look for response data in API calls\n")
	
	fmt.Printf("\n🔍 CHECK API ENDPOINT:\n")
	fmt.Printf("   curl -X GET \"http://localhost:8080/api/v1/accounts/hierarchy\" -H \"Authorization: Bearer YOUR_TOKEN\"\n")
}