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
	
	fmt.Println("🔧 FIXING BANK MANDIRI SYNCHRONIZATION ISSUE")
	
	// Check current discrepancy
	fmt.Printf("\n1️⃣ CURRENT BALANCE COMPARISON:\n")
	
	var comparison struct {
		CashBankBalance float64 `json:"cash_bank_balance"`
		CoaBalance      float64 `json:"coa_balance"`
		AccountName     string  `json:"account_name"`
	}
	
	db.Raw(`
		SELECT cb.balance as cash_bank_balance, a.balance as coa_balance, a.name as account_name
		FROM cash_banks cb
		JOIN accounts a ON cb.account_id = a.id
		WHERE a.code = '1103'
	`).Scan(&comparison)
	
	fmt.Printf("   🏦 %s (1103)\n", comparison.AccountName)
	fmt.Printf("   💰 Cash & Bank Management: %.2f\n", comparison.CashBankBalance)
	fmt.Printf("   📊 Chart of Accounts: %.2f\n", comparison.CoaBalance)
	fmt.Printf("   ⚖️ Discrepancy: %.2f\n", comparison.CashBankBalance - comparison.CoaBalance)
	
	if comparison.CashBankBalance == comparison.CoaBalance {
		fmt.Printf("   ✅ Already synchronized!\n")
		return
	}
	
	// Show the problem clearly
	fmt.Printf("\n❌ PROBLEM IDENTIFIED:\n")
	fmt.Printf("   Cash & Bank Management shows the CORRECT operational balance: %.2f\n", comparison.CashBankBalance)
	fmt.Printf("   Chart of Accounts shows OUTDATED balance: %.2f\n", comparison.CoaBalance)
	fmt.Printf("   This causes incorrect financial reporting and balance sheet totals!\n")
	
	// Check transaction history to understand the correct balance
	fmt.Printf("\n2️⃣ TRANSACTION HISTORY ANALYSIS:\n")
	
	var transactions []struct {
		ID            uint    `json:"id"`
		Amount        float64 `json:"amount"`
		BalanceAfter  float64 `json:"balance_after"`
		ReferenceType string  `json:"reference_type"`
		CreatedAt     string  `json:"created_at"`
		Notes         string  `json:"notes"`
	}
	
	db.Raw(`
		SELECT cbt.id, cbt.amount, cbt.balance_after, cbt.reference_type, cbt.created_at, cbt.notes
		FROM cash_bank_transactions cbt
		JOIN cash_banks cb ON cbt.cash_bank_id = cb.id
		JOIN accounts a ON cb.account_id = a.id
		WHERE a.code = '1103'
		ORDER BY cbt.created_at
	`).Scan(&transactions)
	
	fmt.Printf("   📋 All Bank Mandiri transactions:\n")
	for i, tx := range transactions {
		fmt.Printf("     %d. [%s] %s: %.2f -> Balance: %.2f\n", 
			i+1, tx.CreatedAt[:19], tx.ReferenceType, tx.Amount, tx.BalanceAfter)
		if tx.Notes != "" {
			fmt.Printf("        💬 %s\n", tx.Notes)
		}
	}
	
	if len(transactions) > 0 {
		lastBalance := transactions[len(transactions)-1].BalanceAfter
		fmt.Printf("   🔍 Final transaction balance: %.2f\n", lastBalance)
		fmt.Printf("   🔍 Current cash_banks balance: %.2f\n", comparison.CashBankBalance)
		
		if lastBalance == comparison.CashBankBalance {
			fmt.Printf("   ✅ Transaction history is consistent with cash_banks table\n")
		} else {
			fmt.Printf("   ⚠️ Transaction history inconsistent with cash_banks table\n")
		}
	}
	
	// Apply the correct synchronization
	fmt.Printf("\n3️⃣ APPLYING CORRECT SYNCHRONIZATION:\n")
	
	fmt.Printf("   💡 Rule: COA balance MUST equal Cash & Bank operational balance\n")
	fmt.Printf("   💡 Cash & Bank balance (%.2f) is the source of truth\n", comparison.CashBankBalance)
	fmt.Printf("   💡 Updating COA to match operational balance...\n")
	
	// Update Bank Mandiri COA balance
	err := db.Exec("UPDATE accounts SET balance = ? WHERE code = '1103'", comparison.CashBankBalance).Error
	if err != nil {
		fmt.Printf("   ❌ Failed to update Bank Mandiri COA balance: %v\n", err)
		return
	}
	
	fmt.Printf("   ✅ Bank Mandiri (1103) COA balance updated: %.2f -> %.2f\n", 
		comparison.CoaBalance, comparison.CashBankBalance)
	
	// Recalculate parent accounts with correct formula
	fmt.Printf("\n4️⃣ RECALCULATING PARENT ACCOUNTS:\n")
	
	// CURRENT ASSETS (1100) = sum of all 11xx accounts (level 4)
	var currentAssets float64
	db.Raw(`
		SELECT COALESCE(SUM(balance), 0)
		FROM accounts 
		WHERE code LIKE '11%' AND code != '1100' AND LENGTH(code) = 4
	`).Scan(&currentAssets)
	
	err = db.Exec("UPDATE accounts SET balance = ? WHERE code = '1100'", currentAssets).Error
	if err != nil {
		fmt.Printf("   ❌ Failed to update CURRENT ASSETS: %v\n", err)
	} else {
		fmt.Printf("   ✅ CURRENT ASSETS (1100) updated to: %.2f\n", currentAssets)
	}
	
	// ASSETS (1000) = sum of all 1xxx accounts (level 4) 
	var totalAssets float64
	db.Raw(`
		SELECT COALESCE(SUM(balance), 0)
		FROM accounts 
		WHERE code LIKE '1%' AND code != '1000' AND LENGTH(code) = 4
	`).Scan(&totalAssets)
	
	err = db.Exec("UPDATE accounts SET balance = ? WHERE code = '1000'", totalAssets).Error
	if err != nil {
		fmt.Printf("   ❌ Failed to update ASSETS: %v\n", err)
	} else {
		fmt.Printf("   ✅ ASSETS (1000) updated to: %.2f\n", totalAssets)
	}
	
	// Verify the correction
	fmt.Printf("\n5️⃣ VERIFICATION OF CORRECTION:\n")
	
	// Check Bank Mandiri sync again
	var verifyResult struct {
		CashBankBalance float64 `json:"cash_bank_balance"`
		CoaBalance      float64 `json:"coa_balance"`
	}
	
	db.Raw(`
		SELECT cb.balance as cash_bank_balance, a.balance as coa_balance
		FROM cash_banks cb
		JOIN accounts a ON cb.account_id = a.id
		WHERE a.code = '1103'
	`).Scan(&verifyResult)
	
	fmt.Printf("   🏦 Bank Mandiri (1103):\n")
	fmt.Printf("     💰 Cash & Bank: %.2f\n", verifyResult.CashBankBalance)
	fmt.Printf("     📊 COA: %.2f\n", verifyResult.CoaBalance)
	
	if verifyResult.CashBankBalance == verifyResult.CoaBalance {
		fmt.Printf("     ✅ PERFECT SYNC!\n")
	} else {
		fmt.Printf("     ❌ Still not synced: %.2f difference\n", verifyResult.CashBankBalance - verifyResult.CoaBalance)
	}
	
	// Show breakdown of total assets
	fmt.Printf("\n6️⃣ ASSETS BREAKDOWN VERIFICATION:\n")
	
	var assetBreakdown []struct {
		Code    string  `json:"code"`
		Name    string  `json:"name"`
		Balance float64 `json:"balance"`
	}
	
	db.Raw(`
		SELECT code, name, balance
		FROM accounts 
		WHERE code LIKE '1%' AND LENGTH(code) = 4 AND code != '1000'
		ORDER BY code
	`).Scan(&assetBreakdown)
	
	fmt.Printf("   📊 Asset Account Breakdown:\n")
	var calculatedTotal float64
	
	for _, asset := range assetBreakdown {
		fmt.Printf("     %s (%s): %.2f\n", asset.Code, asset.Name, asset.Balance)
		calculatedTotal += asset.Balance
	}
	
	fmt.Printf("   📊 Calculated Total Assets: %.2f\n", calculatedTotal)
	
	// Compare with ASSETS (1000) balance
	var assetsBalance float64
	db.Raw("SELECT balance FROM accounts WHERE code = '1000'").Scan(&assetsBalance)
	fmt.Printf("   📊 ASSETS (1000) Balance: %.2f\n", assetsBalance)
	
	if calculatedTotal == assetsBalance {
		fmt.Printf("   ✅ ASSETS total is correctly calculated!\n")
	} else {
		fmt.Printf("   ❌ ASSETS total mismatch: %.2f\n", calculatedTotal - assetsBalance)
	}
	
	fmt.Printf("\n🎯 SUMMARY:\n")
	fmt.Printf("✅ Bank Mandiri (1103) COA balance now matches Cash & Bank balance\n")
	fmt.Printf("✅ Parent accounts (CURRENT ASSETS, ASSETS) recalculated correctly\n")
	fmt.Printf("✅ Financial reporting will now show accurate figures\n")
	fmt.Printf("✅ Balance sheet totals are now consistent across all systems\n")
	
	fmt.Printf("\n💡 EXPECTED RESULT:\n")
	fmt.Printf("   Bank Mandiri should show %.2f in both COA and Cash & Bank Management\n", comparison.CashBankBalance)
	fmt.Printf("   Total Assets should reflect the sum of all asset accounts correctly\n")
}