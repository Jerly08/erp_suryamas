package main

import (
	"fmt"
	"encoding/json"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/handlers"
	"app-sistem-akuntansi/repositories"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"net/http"
)

func main() {
	// Load configuration  
	_ = config.LoadConfig()
	
	// Connect to database
	db := database.ConnectDB()
	
	fmt.Println("🔍 DIRECT API RESPONSE DEBUG")
	fmt.Println("=============================")
	
	// First, verify database balance
	fmt.Printf("\n1️⃣ DATABASE VERIFICATION:\n")
	var bankMandiri struct {
		ID      int     `json:"id"`
		Code    string  `json:"code"`
		Name    string  `json:"name"`
		Balance float64 `json:"balance"`
	}
	
	db.Raw("SELECT id, code, name, balance FROM accounts WHERE code = '1103'").Scan(&bankMandiri)
	fmt.Printf("   Database Bank Mandiri (1103): %.2f\n", bankMandiri.Balance)
	
	// Test AccountHandler.GetAccountHierarchy directly
	fmt.Printf("\n2️⃣ TESTING AccountHandler.GetAccountHierarchy():\n")
	
	accountRepo := repositories.NewAccountRepository(db)
	accountHandler := handlers.NewAccountHandler(accountRepo)
	
	// Create a mock Gin context
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	// Set up the request
	req := httptest.NewRequest("GET", "/api/v1/accounts/hierarchy", nil)
	c.Request = req
	
	// Call the handler directly
	accountHandler.GetAccountHierarchy(c)
	
	// Check the response
	fmt.Printf("   Response Status: %d\n", w.Code)
	fmt.Printf("   Response Body Length: %d bytes\n", len(w.Body.String()))
	
	if w.Code == http.StatusOK {
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		if err != nil {
			fmt.Printf("   ❌ Error parsing JSON: %v\n", err)
			fmt.Printf("   Raw response: %s\n", w.Body.String())
		} else {
			fmt.Printf("   ✅ JSON parsed successfully\n")
			
			// Look for Bank Mandiri in the response
			data, ok := response["data"].([]interface{})
			if !ok {
				fmt.Printf("   ❌ No data array in response\n")
				fmt.Printf("   Response structure: %+v\n", response)
			} else {
				fmt.Printf("   📊 Found %d root accounts\n", len(data))
				
				// Search for Bank Mandiri recursively
				bankMandiriFound := false
				var searchInAccounts func(accounts []interface{}) bool
				searchInAccounts = func(accounts []interface{}) bool {
					for _, acc := range accounts {
						account := acc.(map[string]interface{})
						code, _ := account["code"].(string)
						name, _ := account["name"].(string)
						balance, _ := account["balance"].(float64)
						
						if code == "1103" {
							fmt.Printf("   🎯 FOUND Bank Mandiri (1103):\n")
							fmt.Printf("      Name: %s\n", name)
							fmt.Printf("      Balance: %.2f\n", balance)
							fmt.Printf("      Is Header: %v\n", account["is_header"])
							
							if balance == 44450000 {
								fmt.Printf("      ✅ Balance is CORRECT!\n")
							} else {
								fmt.Printf("      ❌ Balance is WRONG! Expected: 44,450,000\n")
							}
							return true
						}
						
						// Check children recursively
						if children, ok := account["children"].([]interface{}); ok && len(children) > 0 {
							if searchInAccounts(children) {
								return true
							}
						}
					}
					return false
				}
				
				bankMandiriFound = searchInAccounts(data)
				
				if !bankMandiriFound {
					fmt.Printf("   ❌ Bank Mandiri (1103) not found in API response\n")
					
					// Print all accounts for debugging
					fmt.Printf("\n   📋 ALL ACCOUNTS IN RESPONSE:\n")
					var printAccounts func(accounts []interface{}, indent string)
					printAccounts = func(accounts []interface{}, indent string) {
						for _, acc := range accounts {
							account := acc.(map[string]interface{})
							code, _ := account["code"].(string)
							name, _ := account["name"].(string)
							balance, _ := account["balance"].(float64)
							fmt.Printf("   %s%s (%s): %.2f\n", indent, code, name, balance)
							
							if children, ok := account["children"].([]interface{}); ok && len(children) > 0 {
								printAccounts(children, indent+"  ")
							}
						}
					}
					printAccounts(data, "   ")
				}
			}
		}
	} else {
		fmt.Printf("   ❌ API call failed with status: %d\n", w.Code)
		fmt.Printf("   Response: %s\n", w.Body.String())
	}
	
	// Check account repository directly
	fmt.Printf("\n3️⃣ TESTING AccountRepository.GetAccountHierarchy():\n")
	
	hierarchyAccounts, err := accountRepo.GetAccountHierarchy()
	if err != nil {
		fmt.Printf("   ❌ Repository error: %v\n", err)
	} else {
		fmt.Printf("   ✅ Repository returned %d root accounts\n", len(hierarchyAccounts))
		
		// Search for Bank Mandiri in repository result
		var findBankMandiri func(accounts interface{}) bool
		findBankMandiri = func(accounts interface{}) bool {
			switch v := accounts.(type) {
			case []map[string]interface{}:
				for _, account := range v {
					if code, ok := account["code"].(string); ok && code == "1103" {
						fmt.Printf("   🎯 Repository Bank Mandiri (1103):\n")
						fmt.Printf("      Name: %v\n", account["name"])
						fmt.Printf("      Balance: %v\n", account["balance"])
						fmt.Printf("      Is Header: %v\n", account["is_header"])
						return true
					}
					if children, ok := account["children"]; ok {
						if findBankMandiri(children) {
							return true
						}
					}
				}
			}
			return false
		}
		
		found := findBankMandiri(hierarchyAccounts)
		if !found {
			fmt.Printf("   ❌ Bank Mandiri not found in repository result\n")
		}
	}
	
	// Test with raw SQL
	fmt.Printf("\n4️⃣ TESTING RAW SQL HIERARCHY:\n")
	
	var hierarchyRaw []struct {
		ID         int     `json:"id"`
		ParentID   *int    `json:"parent_id"`
		Code       string  `json:"code"`
		Name       string  `json:"name"`
		Type       string  `json:"type"`
		Balance    float64 `json:"balance"`
		IsHeader   bool    `json:"is_header"`
		IsActive   bool    `json:"is_active"`
		Level      int     `json:"level"`
	}
	
	result := db.Raw(`
		WITH RECURSIVE account_tree AS (
			SELECT id, parent_id, code, name, type, balance, is_header, is_active, 0 as level
			FROM accounts 
			WHERE parent_id IS NULL AND is_active = true
			
			UNION ALL
			
			SELECT a.id, a.parent_id, a.code, a.name, a.type, a.balance, a.is_header, a.is_active, at.level + 1
			FROM accounts a
			INNER JOIN account_tree at ON a.parent_id = at.id
			WHERE a.is_active = true
		)
		SELECT id, parent_id, code, name, type, balance, is_header, is_active, level
		FROM account_tree 
		ORDER BY level, code
	`).Scan(&hierarchyRaw)
	
	if result.Error != nil {
		fmt.Printf("   ❌ SQL Error: %v\n", result.Error)
	} else {
		fmt.Printf("   ✅ SQL returned %d accounts\n", len(hierarchyRaw))
		
		for _, acc := range hierarchyRaw {
			if acc.Code == "1103" {
				fmt.Printf("   🎯 SQL Bank Mandiri (1103):\n")
				fmt.Printf("      Name: %s\n", acc.Name)
				fmt.Printf("      Balance: %.2f\n", acc.Balance)
				fmt.Printf("      Is Header: %v\n", acc.IsHeader)
				fmt.Printf("      Level: %d\n", acc.Level)
				fmt.Printf("      Parent ID: %v\n", acc.ParentID)
				break
			}
		}
	}
	
	// Recommendation
	fmt.Printf("\n💡 RECOMMENDATIONS:\n")
	fmt.Printf("1. Check if the fix was applied to the correct frontend file\n")
	fmt.Printf("2. Verify frontend compilation happened after the fix\n") 
	fmt.Printf("3. Check browser console for errors\n")
	fmt.Printf("4. Look at Network tab to see actual API response data\n")
}