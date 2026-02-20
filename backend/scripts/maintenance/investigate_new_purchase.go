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
	
	fmt.Println("🔍 INVESTIGATING NEW PURCHASE APPROVAL FLOW")
	
	// Check purchase 1 details
	fmt.Println("\n1️⃣ PURCHASE 1 DETAILS:")
	var purchase struct {
		ID             uint    `json:"id"`
		Code           string  `json:"code"`
		Status         string  `json:"status"`
		TotalAmount    float64 `json:"total_amount"`
		PaymentMethod  string  `json:"payment_method"`
		CashBankID     *uint   `json:"bank_account_id"`
		ApprovalReqID  *uint   `json:"approval_request_id"`
		CreatedAt      string  `json:"created_at"`
		UpdatedAt      string  `json:"updated_at"`
	}
	
	db.Raw(`
		SELECT id, code, status, total_amount, payment_method, bank_account_id, 
		       approval_request_id, created_at, updated_at 
		FROM purchases 
		WHERE id = 1
	`).Scan(&purchase)
	
	fmt.Printf("   Purchase: %s (ID: %d)\n", purchase.Code, purchase.ID)
	fmt.Printf("   Status: %s\n", purchase.Status)
	fmt.Printf("   Total Amount: %.2f\n", purchase.TotalAmount)
	fmt.Printf("   Payment Method: %s\n", purchase.PaymentMethod)
	fmt.Printf("   Cash Bank ID: %v\n", purchase.CashBankID)
	fmt.Printf("   Approval Request ID: %v\n", purchase.ApprovalReqID)
	fmt.Printf("   Created: %s\n", purchase.CreatedAt)
	fmt.Printf("   Updated: %s\n", purchase.UpdatedAt)
	
	// Check approval request details
	if purchase.ApprovalReqID != nil {
		fmt.Println("\n2️⃣ APPROVAL REQUEST DETAILS:")
		var approval struct {
			ID          uint   `json:"id"`
			Status      string `json:"status"`
			CompletedAt *string `json:"completed_at"`
		}
		
		db.Raw(`
			SELECT id, status, completed_at
			FROM approval_requests 
			WHERE id = ?
		`, *purchase.ApprovalReqID).Scan(&approval)
		
		fmt.Printf("   Request ID: %d\n", approval.ID)
		fmt.Printf("   Status: %s\n", approval.Status)
		fmt.Printf("   Completed At: %v\n", approval.CompletedAt)
		
		// Check approval actions
		fmt.Println("\n   📋 Approval Actions:")
		var actions []struct {
			ID          uint   `json:"id"`
			StepName    string `json:"step_name"`
			Status      string `json:"status"`
			IsActive    bool   `json:"is_active"`
			ApprovedBy  *uint  `json:"approved_by"`
			ApprovedAt  *string `json:"approved_at"`
		}
		
		db.Raw(`
			SELECT aa.id, s.name as step_name, aa.status, aa.is_active,
			       aa.approved_by, aa.approved_at
			FROM approval_actions aa
			JOIN approval_steps s ON aa.step_id = s.id
			WHERE aa.request_id = ?
			ORDER BY s.sequence
		`, *purchase.ApprovalReqID).Scan(&actions)
		
		for _, action := range actions {
			activeText := "❌"
			if action.IsActive {
				activeText = "✅"
			}
			fmt.Printf("     %s %s: %s (Active: %s, Approved by: %v at %v)\n",
				activeText, action.StepName, action.Status, activeText, action.ApprovedBy, action.ApprovedAt)
		}
	}
	
	// Check cash bank transactions
	fmt.Println("\n3️⃣ CASH BANK TRANSACTIONS:")
	var cbTransactions []struct {
		ID            uint    `json:"id"`
		Amount        float64 `json:"amount"`
		BalanceAfter  float64 `json:"balance_after"`
		Notes         string  `json:"notes"`
		CreatedAt     string  `json:"created_at"`
	}
	
	db.Raw(`
		SELECT id, amount, balance_after, notes, created_at
		FROM cash_bank_transactions 
		WHERE reference_type = 'PURCHASE' AND reference_id = 1
		ORDER BY created_at
	`).Scan(&cbTransactions)
	
	if len(cbTransactions) == 0 {
		fmt.Printf("   ❌ NO CASH BANK TRANSACTIONS FOUND!\n")
	} else {
		for _, tx := range cbTransactions {
			fmt.Printf("   Transaction %d: %.2f -> Balance: %.2f (%s)\n",
				tx.ID, tx.Amount, tx.BalanceAfter, tx.Notes)
		}
	}
	
	// Check journal entries
	fmt.Println("\n4️⃣ JOURNAL ENTRIES:")
	var journals []struct {
		ID          uint    `json:"id"`
		Reference   string  `json:"reference"`
		Description string  `json:"description"`
		Status      string  `json:"status"`
		TotalDebit  float64 `json:"total_debit"`
		TotalCredit float64 `json:"total_credit"`
		CreatedAt   string  `json:"created_at"`
	}
	
	db.Raw(`
		SELECT id, reference, description, status, total_debit, total_credit, created_at
		FROM journal_entries 
		WHERE reference LIKE '%PO/2025/10/0015%' OR reference_id = 1
		ORDER BY created_at
	`).Scan(&journals)
	
	if len(journals) == 0 {
		fmt.Printf("   ❌ NO JOURNAL ENTRIES FOUND!\n")
	} else {
		for _, j := range journals {
			fmt.Printf("   Journal %d: %s - %s (Status: %s)\n", j.ID, j.Reference, j.Description, j.Status)
			fmt.Printf("     Debit: %.2f, Credit: %.2f\n", j.TotalDebit, j.TotalCredit)
		}
	}
	
	// Check current bank balance
	if purchase.CashBankID != nil {
		fmt.Println("\n5️⃣ CURRENT BANK BALANCE:")
		var bank struct {
			ID      uint    `json:"id"`
			Name    string  `json:"name"`
			Balance float64 `json:"balance"`
			UpdatedAt string `json:"updated_at"`
		}
		
		db.Raw(`
			SELECT id, name, balance, updated_at
			FROM cash_banks WHERE id = ?
		`, *purchase.CashBankID).Scan(&bank)
		
		fmt.Printf("   Bank %d (%s): Balance = %.2f\n", bank.ID, bank.Name, bank.Balance)
		fmt.Printf("   Last Updated: %s\n", bank.UpdatedAt)
		
		// Expected balance after purchase
		expectedBalance := bank.Balance - purchase.TotalAmount
		fmt.Printf("   Expected Balance After Purchase: %.2f\n", expectedBalance)
	}
	
	// Check if OnPurchaseApproved was called
	fmt.Println("\n6️⃣ POST-APPROVAL PROCESSING CHECK:")
	
	// Check if there are any related transactions that should have been created
	var relatedTxCount int64
	db.Raw("SELECT COUNT(*) FROM cash_bank_transactions WHERE reference_type = 'PURCHASE' AND reference_id = 1").Scan(&relatedTxCount)
	
	var relatedJournalCount int64  
	db.Raw("SELECT COUNT(*) FROM journal_entries WHERE reference LIKE '%PO/2025/10/0015%'").Scan(&relatedJournalCount)
	
	fmt.Printf("   Cash Bank Transactions: %d\n", relatedTxCount)
	fmt.Printf("   Journal Entries: %d\n", relatedJournalCount)
	
	if relatedTxCount == 0 && relatedJournalCount > 0 {
		fmt.Printf("   🔍 ISSUE: Journal created but cash bank transaction missing!\n")
	} else if relatedTxCount == 0 && relatedJournalCount == 0 {
		fmt.Printf("   🔍 ISSUE: Neither journal nor cash bank transaction created!\n")
	} else if relatedTxCount > 0 && relatedJournalCount > 0 {
		fmt.Printf("   ✅ Both journal and cash bank transaction exist\n")
	}
	
	fmt.Println("\n🎯 DIAGNOSIS:")
	if purchase.Status != "APPROVED" {
		fmt.Printf("   ❌ Purchase is not APPROVED (Status: %s)\n", purchase.Status)
	} else {
		fmt.Printf("   ✅ Purchase is APPROVED\n")
		
		if relatedTxCount == 0 {
			fmt.Printf("   ❌ Cash bank transaction not created - OnPurchaseApproved callback may not have been called\n")
		} else {
			fmt.Printf("   ✅ Cash bank transaction created\n")
		}
		
		if relatedJournalCount == 0 {
			fmt.Printf("   ❌ Journal entries not created\n") 
		} else {
			fmt.Printf("   ✅ Journal entries created but may not be posted\n")
		}
	}
}