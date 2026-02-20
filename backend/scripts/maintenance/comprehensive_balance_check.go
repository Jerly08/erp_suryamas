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
	
	fmt.Println("🔍 COMPREHENSIVE BALANCE VERIFICATION")
	
	// Check all individual account balances
	fmt.Printf("\n1️⃣ INDIVIDUAL ACCOUNT BALANCES:\n")
	
	var accounts []struct {
		Code    string  `json:"code"`
		Name    string  `json:"name"`
		Type    string  `json:"type"`
		Balance float64 `json:"balance"`
	}
	
	db.Raw(`
		SELECT code, name, type, balance
		FROM accounts 
		WHERE code IN ('1000', '1100', '1101', '1102', '1103', '1104', '1105', '1200', '1201', '1240', '1301', '1500')
		ORDER BY code
	`).Scan(&accounts)
	
	for _, acc := range accounts {
		fmt.Printf("   %s (%s): %.2f [%s]\n", acc.Code, acc.Name, acc.Balance, acc.Type)
	}
	
	// Check Cash & Bank balances vs COA
	fmt.Printf("\n2️⃣ CASH & BANK vs COA COMPARISON:\n")
	
	var cashBankComparison []struct {
		AccountCode     string  `json:"account_code"`
		AccountName     string  `json:"account_name"`
		CoaBalance      float64 `json:"coa_balance"`
		CashBankBalance float64 `json:"cash_bank_balance"`
		Difference      float64 `json:"difference"`
	}
	
	db.Raw(`
		SELECT a.code as account_code, a.name as account_name, 
		       a.balance as coa_balance, 
		       COALESCE(cb.balance, 0) as cash_bank_balance,
		       a.balance - COALESCE(cb.balance, 0) as difference
		FROM accounts a
		LEFT JOIN cash_banks cb ON cb.account_id = a.id
		WHERE a.code LIKE '110%' AND a.code != '1100'
		ORDER BY a.code
	`).Scan(&cashBankComparison)
	
	fmt.Printf("   🏦 Bank Account Synchronization:\n")
	allBanksSynced := true
	
	for _, comp := range cashBankComparison {
		if comp.Difference == 0 {
			fmt.Printf("     ✅ %s (%s): COA=%.2f, CashBank=%.2f\n", 
				comp.AccountCode, comp.AccountName, comp.CoaBalance, comp.CashBankBalance)
		} else {
			fmt.Printf("     ❌ %s (%s): COA=%.2f, CashBank=%.2f, Diff=%.2f\n", 
				comp.AccountCode, comp.AccountName, comp.CoaBalance, comp.CashBankBalance, comp.Difference)
			allBanksSynced = false
		}
	}
	
	if !allBanksSynced {
		fmt.Printf("   🔧 Auto-correcting bank account discrepancies...\n")
		for _, comp := range cashBankComparison {
			if comp.Difference != 0 {
				err := db.Exec("UPDATE accounts SET balance = ? WHERE code = ?", comp.CashBankBalance, comp.AccountCode).Error
				if err == nil {
					fmt.Printf("     ✅ Corrected %s: %.2f -> %.2f\n", 
						comp.AccountCode, comp.CoaBalance, comp.CashBankBalance)
				}
			}
		}
	}
	
	// Calculate and verify hierarchy
	fmt.Printf("\n3️⃣ ACCOUNT HIERARCHY CALCULATION:\n")
	
	// Calculate CURRENT ASSETS (1100)
	var calculatedCurrentAssets float64
	db.Raw(`
		SELECT COALESCE(SUM(balance), 0)
		FROM accounts 
		WHERE code LIKE '11%' AND code != '1100' AND LENGTH(code) = 4
	`).Scan(&calculatedCurrentAssets)
	
	var actualCurrentAssets float64
	db.Raw("SELECT balance FROM accounts WHERE code = '1100'").Scan(&actualCurrentAssets)
	
	fmt.Printf("   📊 CURRENT ASSETS (1100):\n")
	fmt.Printf("     Calculated: %.2f\n", calculatedCurrentAssets)
	fmt.Printf("     Actual: %.2f\n", actualCurrentAssets)
	
	if calculatedCurrentAssets != actualCurrentAssets {
		fmt.Printf("     🔧 Correcting CURRENT ASSETS...\n")
		db.Exec("UPDATE accounts SET balance = ? WHERE code = '1100'", calculatedCurrentAssets)
		fmt.Printf("     ✅ Updated to %.2f\n", calculatedCurrentAssets)
	} else {
		fmt.Printf("     ✅ Correct!\n")
	}
	
	// Calculate TOTAL ASSETS (1000)
	var calculatedTotalAssets float64
	db.Raw(`
		SELECT COALESCE(SUM(balance), 0)
		FROM accounts 
		WHERE code LIKE '1%' AND code != '1000' AND LENGTH(code) = 4
	`).Scan(&calculatedTotalAssets)
	
	var actualTotalAssets float64
	db.Raw("SELECT balance FROM accounts WHERE code = '1000'").Scan(&actualTotalAssets)
	
	fmt.Printf("   📊 TOTAL ASSETS (1000):\n")
	fmt.Printf("     Calculated: %.2f\n", calculatedTotalAssets)
	fmt.Printf("     Actual: %.2f\n", actualTotalAssets)
	
	if calculatedTotalAssets != actualTotalAssets {
		fmt.Printf("     🔧 Correcting TOTAL ASSETS...\n")
		db.Exec("UPDATE accounts SET balance = ? WHERE code = '1000'", calculatedTotalAssets)
		fmt.Printf("     ✅ Updated to %.2f\n", calculatedTotalAssets)
	} else {
		fmt.Printf("     ✅ Correct!\n")
	}
	
	// Detailed breakdown
	fmt.Printf("\n4️⃣ DETAILED ASSETS BREAKDOWN:\n")
	
	var detailedBreakdown []struct {
		Code    string  `json:"code"`
		Name    string  `json:"name"`
		Balance float64 `json:"balance"`
	}
	
	db.Raw(`
		SELECT code, name, balance
		FROM accounts 
		WHERE code LIKE '1%' AND code NOT IN ('1000', '1100', '1200', '1500') AND LENGTH(code) = 4
		ORDER BY code
	`).Scan(&detailedBreakdown)
	
	fmt.Printf("   💰 Individual Asset Components:\n")
	var manualTotal float64
	
	for _, asset := range detailedBreakdown {
		fmt.Printf("     %s (%s): %.2f\n", asset.Code, asset.Name, asset.Balance)
		manualTotal += asset.Balance
	}
	
	fmt.Printf("   📊 Manual Total: %.2f\n", manualTotal)
	
	// Cash & Bank Management totals
	fmt.Printf("\n5️⃣ CASH & BANK MANAGEMENT TOTALS:\n")
	
	var cashBankTotal float64
	db.Raw("SELECT COALESCE(SUM(balance), 0) FROM cash_banks").Scan(&cashBankTotal)
	
	fmt.Printf("   💰 Total Cash & Bank Management Balance: %.2f\n", cashBankTotal)
	
	// Expected vs Actual comparison
	fmt.Printf("\n6️⃣ EXPECTED vs ACTUAL COMPARISON:\n")
	
	// Based on your observation:
	// Bank Mandiri: should be 44,450,000 (from Cash & Bank Management)
	// PPN Masukan: 550,000
	// Persediaan Barang Dagangan: 5,000,000
	// Expected Total Assets: 44,450,000 + 550,000 + 5,000,000 = 50,000,000
	
	expectedBankMandiri := 44450000.0
	expectedPPN := 550000.0
	expectedPersediaan := 5000000.0
	expectedTotalAssets := expectedBankMandiri + expectedPPN + expectedPersediaan
	
	fmt.Printf("   🎯 EXPECTED BALANCES:\n")
	fmt.Printf("     Bank Mandiri (1103): %.2f\n", expectedBankMandiri)
	fmt.Printf("     PPN Masukan (1240): %.2f\n", expectedPPN)
	fmt.Printf("     Persediaan (1301): %.2f\n", expectedPersediaan)
	fmt.Printf("     TOTAL ASSETS: %.2f\n", expectedTotalAssets)
	
	fmt.Printf("   📊 ACTUAL BALANCES:\n")
	
	var actualBankMandiri, actualPPN, actualPersediaan float64
	db.Raw("SELECT balance FROM accounts WHERE code = '1103'").Scan(&actualBankMandiri)
	db.Raw("SELECT balance FROM accounts WHERE code = '1240'").Scan(&actualPPN)
	db.Raw("SELECT balance FROM accounts WHERE code = '1301'").Scan(&actualPersediaan)
	
	fmt.Printf("     Bank Mandiri (1103): %.2f", actualBankMandiri)
	if actualBankMandiri == expectedBankMandiri {
		fmt.Printf(" ✅\n")
	} else {
		fmt.Printf(" ❌ (expected %.2f)\n", expectedBankMandiri)
	}
	
	fmt.Printf("     PPN Masukan (1240): %.2f", actualPPN)
	if actualPPN == expectedPPN {
		fmt.Printf(" ✅\n")
	} else {
		fmt.Printf(" ❌ (expected %.2f)\n", expectedPPN)
	}
	
	fmt.Printf("     Persediaan (1301): %.2f", actualPersediaan)
	if actualPersediaan == expectedPersediaan {
		fmt.Printf(" ✅\n")
	} else {
		fmt.Printf(" ❌ (expected %.2f)\n", expectedPersediaan)
	}
	
	fmt.Printf("     TOTAL ASSETS (1000): %.2f", actualTotalAssets)
	if actualTotalAssets == expectedTotalAssets {
		fmt.Printf(" ✅\n")
	} else {
		fmt.Printf(" ❌ (expected %.2f)\n", expectedTotalAssets)
	}
	
	// Final summary
	fmt.Printf("\n🎯 FINAL SUMMARY:\n")
	
	if actualBankMandiri == expectedBankMandiri && actualPPN == expectedPPN && actualPersediaan == expectedPersediaan {
		fmt.Printf("✅ All individual account balances are correct\n")
		if actualTotalAssets == expectedTotalAssets {
			fmt.Printf("✅ Total assets calculation is correct\n")
			fmt.Printf("🎉 PERFECT! All balances are synchronized and accurate\n")
		} else {
			fmt.Printf("❌ Total assets needs recalculation\n")
		}
	} else {
		fmt.Printf("❌ Some individual account balances need correction\n")
	}
	
	fmt.Printf("\n💡 INSTRUCTION FOR FRONTEND:\n")
	fmt.Printf("If the COA still shows old balances in the browser:\n")
	fmt.Printf("1. Clear browser cache (Ctrl+F5 or hard refresh)\n")
	fmt.Printf("2. Check if there's any caching mechanism in the frontend\n")
	fmt.Printf("3. Verify API endpoints are returning fresh data from database\n")
}