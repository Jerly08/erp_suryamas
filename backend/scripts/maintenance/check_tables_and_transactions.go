package main

import (
	"fmt"
	"log"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/config"
)

func main() {
	// Load configuration
	_ = config.LoadConfig()
	
	// Connect to database
	db := database.ConnectDB()
	
	fmt.Println("🔍 Checking database tables and cash bank transactions...")
	
	// 1. Check what approval tables exist
	fmt.Println("\n1. Checking approval-related tables:")
	var tables []string
	err := db.Raw(`
		SELECT table_name 
		FROM information_schema.tables 
		WHERE table_name LIKE '%approval%' 
		  AND table_schema = 'public'
		ORDER BY table_name
	`).Scan(&tables).Error
	
	if err != nil {
		log.Printf("Error querying tables: %v", err)
		return
	}
	
	for _, table := range tables {
		fmt.Printf("  - %s\n", table)
	}
	
	// 2. Check cash_bank_transactions specifically for PURCHASE references
	fmt.Println("\n2. Cash & Bank transactions for PURCHASE references:")
	type TransactionInfo struct {
		ID uint `json:"id"`
		CashBankName string `json:"cash_bank_name"`
		ReferenceType string `json:"reference_type"`
		ReferenceID uint `json:"reference_id"`
		Amount float64 `json:"amount"`
		BalanceAfter float64 `json:"balance_after"`
		TransactionDate string `json:"transaction_date"`
		Notes string `json:"notes"`
	}
	
	var transactions []TransactionInfo
	err = db.Raw(`
		SELECT 
			cbt.id,
			cb.name as cash_bank_name,
			cbt.reference_type,
			cbt.reference_id,
			cbt.amount,
			cbt.balance_after,
			cbt.transaction_date,
			cbt.notes
		FROM cash_bank_transactions cbt
		JOIN cash_banks cb ON cbt.cash_bank_id = cb.id
		WHERE cbt.reference_type = 'PURCHASE'
		ORDER BY cbt.transaction_date DESC
		LIMIT 10
	`).Scan(&transactions).Error
	
	if err != nil {
		log.Printf("Error querying cash bank transactions: %v", err)
		return
	}
	
	if len(transactions) == 0 {
		fmt.Println("  ❌ NO cash & bank transactions found for PURCHASE references!")
		fmt.Println("  💡 This confirms the issue - cash & bank balance is NOT being updated")
	} else {
		fmt.Printf("  ✅ Found %d cash & bank transactions:\n", len(transactions))
		for _, t := range transactions {
			fmt.Printf("    %s: Purchase ID %d, Amount: %.2f, Balance After: %.2f, Date: %s\n",
				t.CashBankName, t.ReferenceID, t.Amount, t.BalanceAfter, t.TransactionDate)
		}
	}
	
	// 3. Check the specific approved purchases from earlier
	fmt.Println("\n3. Checking specific approved purchases (ID: 2, 3):")
	type PurchaseCheck struct {
		ID uint `json:"id"`
		Code string `json:"code"`
		PaymentMethod string `json:"payment_method"`
		BankAccountID *uint `json:"bank_account_id"`
		TotalAmount float64 `json:"total_amount"`
		Status string `json:"status"`
		ApprovalStatus string `json:"approval_status"`
		ApprovedAt *string `json:"approved_at"`
	}
	
	var approvedPurchases []PurchaseCheck
	err = db.Raw(`
		SELECT id, code, payment_method, bank_account_id, total_amount, 
		       status, approval_status, approved_at
		FROM purchases 
		WHERE id IN (2, 3) AND status = 'APPROVED'
		ORDER BY id
	`).Scan(&approvedPurchases).Error
	
	if err != nil {
		log.Printf("Error querying specific purchases: %v", err)
		return
	}
	
	for _, p := range approvedPurchases {
		bankID := "NULL"
		if p.BankAccountID != nil {
			bankID = fmt.Sprintf("%d", *p.BankAccountID)
		}
		approvedAt := "NULL"
		if p.ApprovedAt != nil {
			approvedAt = *p.ApprovedAt
		}
		fmt.Printf("  Purchase %d (%s): Method=%s, Bank=%s, Amount=%.2f, Approved=%s\n",
			p.ID, p.Code, p.PaymentMethod, bankID, p.TotalAmount, approvedAt)
		
		// Check if there are any transactions for this specific purchase
		var txCount int64
		db.Raw("SELECT COUNT(*) FROM cash_bank_transactions WHERE reference_type = 'PURCHASE' AND reference_id = ?", p.ID).Scan(&txCount)
		if txCount == 0 {
			fmt.Printf("    ❌ NO cash_bank_transactions found for Purchase %d\n", p.ID)
		} else {
			fmt.Printf("    ✅ Found %d cash_bank_transactions for Purchase %d\n", txCount, p.ID)
		}
	}
	
	// 4. Check current cash_banks balance for bank account ID 7
	fmt.Println("\n4. Current balance for Bank Account ID 7:")
	type BankBalance struct {
		ID uint `json:"id"`
		Name string `json:"name"`
		Balance float64 `json:"balance"`
		UpdatedAt string `json:"updated_at"`
	}
	
	var bankBalance BankBalance
	err = db.Raw(`
		SELECT id, name, balance, updated_at
		FROM cash_banks 
		WHERE id = 7
	`).Scan(&bankBalance).Error
	
	if err != nil {
		log.Printf("Error querying bank balance: %v", err)
		return
	}
	
	fmt.Printf("  %s (ID: %d): Balance %.2f, Updated: %s\n",
		bankBalance.Name, bankBalance.ID, bankBalance.Balance, bankBalance.UpdatedAt)
	
	// Calculate expected balance if transactions were processed
	expectedDecrease := float64(0)
	for _, p := range approvedPurchases {
		if p.BankAccountID != nil && *p.BankAccountID == 7 {
			expectedDecrease += p.TotalAmount
		}
	}
	
	if expectedDecrease > 0 {
		fmt.Printf("  💡 Expected balance decrease from approved purchases: %.2f\n", expectedDecrease)
		fmt.Printf("  💡 If cash & bank balance was working correctly, balance should be %.2f lower\n", expectedDecrease)
	}
	
	// 5. Check approval_steps table name (might be different)
	fmt.Println("\n5. Checking approval steps table name:")
	var stepTables []string
	err = db.Raw(`
		SELECT table_name 
		FROM information_schema.tables 
		WHERE table_name LIKE '%step%' 
		  AND table_schema = 'public'
		ORDER BY table_name
	`).Scan(&stepTables).Error
	
	if err == nil {
		fmt.Println("  Tables containing 'step':")
		for _, table := range stepTables {
			fmt.Printf("    - %s\n", table)
		}
	}
	
	// Summary
	fmt.Println("\n📋 INVESTIGATION SUMMARY:")
	fmt.Printf("  - Found %d approved immediate payment purchases\n", len(approvedPurchases))
	fmt.Printf("  - Found %d cash_bank_transactions for PURCHASE references\n", len(transactions))
	
	if len(approvedPurchases) > 0 && len(transactions) == 0 {
		fmt.Println("\n🚨 CONFIRMED ISSUE:")
		fmt.Println("  ❌ Approved immediate payment purchases exist but NO cash_bank_transactions")
		fmt.Println("  💡 This confirms that OnPurchaseApproved() callback is NOT being triggered")
		fmt.Println("  🔧 Root cause: Post-approval processing is not executing")
	}
	
	fmt.Println("\n✅ Investigation completed!")
}