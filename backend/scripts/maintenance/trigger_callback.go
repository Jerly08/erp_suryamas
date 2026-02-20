package main

import (
	"fmt"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/services"
	"app-sistem-akuntansi/repositories"
)

func main() {
	// Load configuration  
	_ = config.LoadConfig()
	
	// Connect to database
	db := database.ConnectDB()
	
	fmt.Println("🧪 TESTING ONPURCHASEAPPROVED CALLBACK")
	
	// Initialize required repositories and services
	purchaseRepo := repositories.NewPurchaseRepository(db)
	productRepo := repositories.NewProductRepository(db)
	contactRepo := repositories.NewContactRepository(db)
	accountRepo := repositories.NewAccountRepository(db)
	approvalService := services.NewApprovalService(db)
	journalRepo := repositories.NewJournalEntryRepository(db)
	pdfService := services.NewPDFService(db)
	unifiedJournalService := services.NewUnifiedJournalService(db)
	coaService := services.NewCOAService(db)
	
	// Initialize purchase service with all dependencies
	purchaseService := services.NewPurchaseService(
		db,
		purchaseRepo,
		productRepo, 
		contactRepo,
		accountRepo,
		approvalService,
		nil, // journal service - can be nil for now
		journalRepo,
		pdfService,
		unifiedJournalService,
		coaService,
	)
	
	// Test dengan purchase ID 1
	purchaseID := uint(1)
	
	// Check current status
	fmt.Printf("\n📋 Current status of Purchase %d:\n", purchaseID)
	var purchase struct {
		ID            uint    `json:"id"`
		Code          string  `json:"code"`
		Status        string  `json:"status"`
		TotalAmount   float64 `json:"total_amount"`
		PaymentMethod string  `json:"payment_method"`
		BankAccountID *uint   `json:"bank_account_id"`
	}
	
	db.Raw(`
		SELECT id, code, status, total_amount, payment_method, bank_account_id
		FROM purchases WHERE id = ?
	`, purchaseID).Scan(&purchase)
	
	fmt.Printf("   Purchase: %s, Status: %s, Amount: %.2f\n", 
		purchase.Code, purchase.Status, purchase.TotalAmount)
	fmt.Printf("   Payment Method: %s, Bank Account ID: %v\n", 
		purchase.PaymentMethod, purchase.BankAccountID)
	
	// Check cash bank transactions before
	var cbTxCountBefore int64
	db.Raw("SELECT COUNT(*) FROM cash_bank_transactions WHERE reference_type = 'PURCHASE' AND reference_id = ?", purchaseID).Scan(&cbTxCountBefore)
	fmt.Printf("   Cash Bank Transactions Before: %d\n", cbTxCountBefore)
	
	if purchase.Status != "APPROVED" {
		fmt.Printf("⚠️ Purchase is not APPROVED (Status: %s). Cannot test OnPurchaseApproved callback.\n", purchase.Status)
		return
	}
	
	// If no bank account, assign one for testing
	if purchase.BankAccountID == nil {
		fmt.Printf("🔧 Purchase has no bank account. Assigning bank account ID 7 for testing...\n")
		
		err := db.Exec("UPDATE purchases SET bank_account_id = 7 WHERE id = ?", purchaseID).Error
		if err != nil {
			fmt.Printf("❌ Failed to assign bank account: %v\n", err)
			return
		}
		purchase.BankAccountID = new(uint)
		*purchase.BankAccountID = 7
		fmt.Printf("✅ Bank account 7 assigned to purchase\n")
	}
	
	// Get bank balance before
	var balanceBefore float64
	db.Raw("SELECT balance FROM cash_banks WHERE id = ?", *purchase.BankAccountID).Scan(&balanceBefore)
	fmt.Printf("   Bank Balance Before: %.2f\n", balanceBefore)
	
	// Call OnPurchaseApproved directly
	fmt.Printf("\n🔔 Calling OnPurchaseApproved callback...\n")
	err := purchaseService.OnPurchaseApproved(purchaseID)
	if err != nil {
		fmt.Printf("❌ OnPurchaseApproved callback failed: %v\n", err)
		return
	} else {
		fmt.Printf("✅ OnPurchaseApproved callback completed successfully\n")
	}
	
	// Check results after
	fmt.Printf("\n📊 RESULTS AFTER CALLBACK:\n")
	
	// Check cash bank transactions after
	var cbTxCountAfter int64
	db.Raw("SELECT COUNT(*) FROM cash_bank_transactions WHERE reference_type = 'PURCHASE' AND reference_id = ?", purchaseID).Scan(&cbTxCountAfter)
	fmt.Printf("   Cash Bank Transactions After: %d\n", cbTxCountAfter)
	
	if cbTxCountAfter > cbTxCountBefore {
		fmt.Printf("   ✅ NEW cash bank transaction(s) created!\n")
		
		// Show the new transactions
		var newTx []struct {
			ID     uint    `json:"id"`
			Amount float64 `json:"amount"`
			Notes  string  `json:"notes"`
		}
		
		db.Raw(`
			SELECT id, amount, notes
			FROM cash_bank_transactions 
			WHERE reference_type = 'PURCHASE' AND reference_id = ?
		`, purchaseID).Scan(&newTx)
		
		for _, tx := range newTx {
			fmt.Printf("     Transaction %d: %.2f - %s\n", tx.ID, tx.Amount, tx.Notes)
		}
	} else {
		fmt.Printf("   ❌ NO new cash bank transactions created\n")
	}
	
	// Check bank balance after
	var balanceAfter float64
	db.Raw("SELECT balance FROM cash_banks WHERE id = ?", *purchase.BankAccountID).Scan(&balanceAfter)
	fmt.Printf("   Bank Balance After: %.2f\n", balanceAfter)
	
	if balanceAfter != balanceBefore {
		fmt.Printf("   ✅ Bank balance changed by: %.2f\n", balanceAfter - balanceBefore)
	} else {
		fmt.Printf("   ❌ Bank balance unchanged\n")
	}
	
	// Check journal entries
	var journalCount int64
	db.Raw("SELECT COUNT(*) FROM journal_entries WHERE reference LIKE ? OR (reference_type = 'PURCHASE' AND reference_id = ?)", 
		"%"+purchase.Code+"%", purchaseID).Scan(&journalCount)
	fmt.Printf("   Journal Entries: %d\n", journalCount)
	
	fmt.Printf("\n🎯 FINAL SUMMARY:\n")
	if cbTxCountAfter > cbTxCountBefore {
		fmt.Printf("   ✅ SUCCESS: OnPurchaseApproved callback created cash bank transactions!\n")
	} else {
		fmt.Printf("   ❌ ISSUE: OnPurchaseApproved callback did NOT create cash bank transactions\n")
		fmt.Printf("       Possible reasons:\n")
		fmt.Printf("       1. Payment method is not immediate (BANK_TRANSFER, CASH, CHECK)\n")
		fmt.Printf("       2. Missing bank account assignment\n")
		fmt.Printf("       3. Error in updateCashBankBalanceForPurchase method\n")
	}
}