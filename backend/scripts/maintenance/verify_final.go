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
	
	fmt.Println("🔍 FINAL VERIFICATION - Checking if all issues are resolved...")
	
	// Check purchase status
	fmt.Println("\n1️⃣ PURCHASE STATUS:")
	var purchases []struct {
		ID          uint    `json:"id"`
		Code        string  `json:"code"`
		Status      string  `json:"status"`
		TotalAmount float64 `json:"total_amount"`
	}
	
	db.Raw(`
		SELECT id, code, status, total_amount 
		FROM purchases 
		WHERE id IN (2,3)
		ORDER BY id
	`).Scan(&purchases)
	
	for _, p := range purchases {
		fmt.Printf("   Purchase %d (%s): %s - Amount: %.2f\n", p.ID, p.Code, p.Status, p.TotalAmount)
	}
	
	// Check approval status
	fmt.Println("\n2️⃣ APPROVAL STATUS:")
	var approvals []struct {
		ID          uint   `json:"id"`
		Status      string `json:"status"`
		Completed   bool   `json:"completed"`
	}
	
	db.Raw(`
		SELECT id, status, completed_at IS NOT NULL as completed
		FROM approval_requests 
		WHERE id IN (24,25)
		ORDER BY id
	`).Scan(&approvals)
	
	for _, a := range approvals {
		completedText := "❌ No"
		if a.Completed {
			completedText = "✅ Yes"
		}
		fmt.Printf("   Request %d: %s - Completed: %s\n", a.ID, a.Status, completedText)
	}
	
	// Check cash bank transactions
	fmt.Println("\n3️⃣ CASH BANK TRANSACTIONS:")
	var transactions []struct {
		ID          uint    `json:"id"`
		Amount      float64 `json:"amount"`
		ReferenceID uint    `json:"reference_id"`
		Notes       string  `json:"notes"`
	}
	
	db.Raw(`
		SELECT id, amount, reference_id, notes
		FROM cash_bank_transactions 
		WHERE reference_type = 'PURCHASE' AND reference_id IN (2,3)
		ORDER BY reference_id
	`).Scan(&transactions)
	
	if len(transactions) == 0 {
		fmt.Printf("   ❌ No transactions found!\n")
	} else {
		for _, t := range transactions {
			fmt.Printf("   Transaction %d: %.2f for Purchase %d - %s\n", 
				t.ID, t.Amount, t.ReferenceID, t.Notes)
		}
	}
	
	// Check bank balance
	fmt.Println("\n4️⃣ BANK BALANCE:")
	var bank struct {
		ID          uint    `json:"id"`
		AccountName string  `json:"name"`
		Balance     float64 `json:"balance"`
	}
	
	db.Raw(`
		SELECT id, name, balance
		FROM cash_banks WHERE id = 7
	`).Scan(&bank)
	
	fmt.Printf("   Bank %d (%s): Balance = %.2f\n", bank.ID, bank.AccountName, bank.Balance)
	
	// Calculate expected balance
	fmt.Println("\n5️⃣ BALANCE CALCULATION:")
	var initialBalance, totalPayments float64
	
	// Get initial balance (should be 12M)
	initialBalance = 12000000.0
	
	// Calculate total payments
	for _, p := range purchases {
		if p.Status == "APPROVED" {
			totalPayments += p.TotalAmount
		}
	}
	
	expectedBalance := initialBalance - totalPayments
	
	fmt.Printf("   Initial Balance: %.2f\n", initialBalance)
	fmt.Printf("   Total Payments: %.2f\n", totalPayments)
	fmt.Printf("   Expected Balance: %.2f\n", expectedBalance)
	fmt.Printf("   Actual Balance: %.2f\n", bank.Balance)
	
	if bank.Balance == expectedBalance {
		fmt.Printf("   ✅ Balance is correct!\n")
	} else {
		fmt.Printf("   ❌ Balance mismatch! Difference: %.2f\n", bank.Balance - expectedBalance)
	}
	
	// Final summary
	fmt.Println("\n🎯 SUMMARY:")
	allPurchasesApproved := true
	allRequestsCompleted := true
	transactionsExist := len(transactions) > 0
	balanceCorrect := bank.Balance == expectedBalance
	
	for _, p := range purchases {
		if p.Status != "APPROVED" {
			allPurchasesApproved = false
		}
	}
	
	for _, a := range approvals {
		if !a.Completed {
			allRequestsCompleted = false
		}
	}
	
	if allPurchasesApproved {
		fmt.Printf("   ✅ All purchases are APPROVED\n")
	} else {
		fmt.Printf("   ❌ Some purchases are not approved\n")
	}
	
	if allRequestsCompleted {
		fmt.Printf("   ✅ All approval requests are COMPLETED\n")
	} else {
		fmt.Printf("   ❌ Some approval requests are incomplete\n")
	}
	
	if transactionsExist {
		fmt.Printf("   ✅ Cash bank transactions created\n")
	} else {
		fmt.Printf("   ❌ No cash bank transactions found\n")
	}
	
	if balanceCorrect {
		fmt.Printf("   ✅ Bank balance is accurate\n")
	} else {
		fmt.Printf("   ❌ Bank balance is incorrect\n")
	}
	
	if allPurchasesApproved && allRequestsCompleted && transactionsExist && balanceCorrect {
		fmt.Println("\n🎉 ALL ISSUES RESOLVED SUCCESSFULLY!")
		fmt.Println("   The approval workflow bug has been fixed and all data is consistent.")
	} else {
		fmt.Println("\n⚠️  Some issues may still exist - check the details above.")
	}
}