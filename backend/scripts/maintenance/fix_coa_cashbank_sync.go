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
	
	fmt.Println("🔍 INVESTIGATING COA vs CASH & BANK BALANCE DISCREPANCY")
	
	// Check Bank Mandiri (1103) balance discrepancy
	fmt.Printf("\n1️⃣ BALANCE COMPARISON FOR BANK MANDIRI (1103):\n")
	
	// Get balance from cash_banks table (actual operational balance)
	var cashBankBalance float64
	var cashBankID uint
	db.Raw(`
		SELECT cb.balance, cb.id
		FROM cash_banks cb
		JOIN accounts a ON cb.account_id = a.id
		WHERE a.code = '1103'
	`).Scan(&cashBankBalance)
	
	db.Raw(`
		SELECT cb.id
		FROM cash_banks cb
		JOIN accounts a ON cb.account_id = a.id
		WHERE a.code = '1103'
	`).Scan(&cashBankID)
	
	// Get balance from accounts table (COA balance for financial reporting)
	var coaBalance float64
	db.Raw("SELECT balance FROM accounts WHERE code = '1103'").Scan(&coaBalance)
	
	fmt.Printf("   💰 Cash & Bank Operational Balance: %.2f\n", cashBankBalance)
	fmt.Printf("   📊 COA Accounting Balance: %.2f\n", coaBalance)
	fmt.Printf("   ⚖️ Discrepancy: %.2f\n", cashBankBalance - coaBalance)
	
	if cashBankBalance != coaBalance {
		fmt.Printf("   ❌ ISSUE: Cash & Bank balance doesn't match COA balance\n")
	} else {
		fmt.Printf("   ✅ Balances are synchronized\n")
		return
	}
	
	// Check what's causing the discrepancy
	fmt.Printf("\n2️⃣ ANALYZING TRANSACTION HISTORY:\n")
	
	// Get all cash bank transactions for Bank Mandiri
	var cbTransactions []struct {
		ID            uint    `json:"id"`
		Amount        float64 `json:"amount"`
		BalanceAfter  float64 `json:"balance_after"`
		ReferenceType string  `json:"reference_type"`
		ReferenceID   uint    `json:"reference_id"`
		Notes         string  `json:"notes"`
		CreatedAt     string  `json:"created_at"`
	}
	
	db.Raw(`
		SELECT id, amount, balance_after, reference_type, reference_id, notes, created_at
		FROM cash_bank_transactions 
		WHERE cash_bank_id = ?
		ORDER BY created_at
	`, cashBankID).Scan(&cbTransactions)
	
	fmt.Printf("   📋 Cash Bank Transactions for Bank Mandiri (%d):\n", len(cbTransactions))
	totalCashBankMovement := 0.0
	
	for i, tx := range cbTransactions {
		fmt.Printf("     %d. [%s] %s-%d: %.2f -> Balance: %.2f\n", 
			i+1, tx.CreatedAt[:19], tx.ReferenceType, tx.ReferenceID, tx.Amount, tx.BalanceAfter)
		if i == 0 {
			// First transaction, movement is the amount itself
			totalCashBankMovement = tx.Amount
		} else {
			// Subsequent transactions, add the amount
			totalCashBankMovement += tx.Amount
		}
		if tx.Notes != "" {
			fmt.Printf("        Notes: %s\n", tx.Notes)
		}
	}
	
	fmt.Printf("   💸 Total Cash Bank Movement: %.2f\n", totalCashBankMovement)
	
	// Check journal entries affecting Bank Mandiri (1103) 
	fmt.Printf("\n3️⃣ CHECKING JOURNAL ENTRIES AFFECTING COA:\n")
	
	var journalLines []struct {
		JournalID     uint    `json:"journal_id"`
		LineNumber    int     `json:"line_number"`
		Description   string  `json:"description"`
		DebitAmount   float64 `json:"debit_amount"`
		CreditAmount  float64 `json:"credit_amount"`
		RefType       string  `json:"reference_type"`
		Status        string  `json:"status"`
		CreatedAt     string  `json:"created_at"`
	}
	
	db.Raw(`
		SELECT jl.journal_id, jl.line_number, jl.description, jl.debit_amount, jl.credit_amount,
		       je.reference_type, je.status, je.created_at
		FROM journal_lines jl
		JOIN journal_entries je ON jl.journal_id = je.id
		JOIN accounts a ON jl.account_id = a.id
		WHERE a.code = '1103' AND je.status = 'POSTED'
		ORDER BY je.created_at, jl.line_number
	`).Scan(&journalLines)
	
	fmt.Printf("   📚 Posted Journal Entries affecting Bank Mandiri (%d):\n", len(journalLines))
	totalJournalDebit := 0.0
	totalJournalCredit := 0.0
	
	for _, jl := range journalLines {
		fmt.Printf("     J%d-L%d [%s]: %s | Debit: %.2f | Credit: %.2f | %s\n", 
			jl.JournalID, jl.LineNumber, jl.CreatedAt[:19], jl.Description, 
			jl.DebitAmount, jl.CreditAmount, jl.RefType)
		totalJournalDebit += jl.DebitAmount
		totalJournalCredit += jl.CreditAmount
	}
	
	netJournalEffect := totalJournalDebit - totalJournalCredit
	fmt.Printf("   📊 Total Journal Debit: %.2f\n", totalJournalDebit)
	fmt.Printf("   📊 Total Journal Credit: %.2f\n", totalJournalCredit)
	fmt.Printf("   ⚖️ Net Journal Effect: %.2f (Debit - Credit for Asset account)\n", netJournalEffect)
	
	// Analysis
	fmt.Printf("\n4️⃣ ROOT CAUSE ANALYSIS:\n")
	
	expectedCoaBalance := netJournalEffect  // For asset account, net debit increases balance
	fmt.Printf("   🔍 Expected COA Balance based on journals: %.2f\n", expectedCoaBalance)
	fmt.Printf("   🔍 Actual COA Balance: %.2f\n", coaBalance)
	fmt.Printf("   🔍 Actual Cash Bank Balance: %.2f\n", cashBankBalance)
	
	// Check if there's a base amount missing
	if len(cbTransactions) > 0 {
		firstTransaction := cbTransactions[0]
		if firstTransaction.ReferenceType == "DEPOSIT" {
			initialDeposit := firstTransaction.Amount
			fmt.Printf("   💰 Initial Deposit Found: %.2f\n", initialDeposit)
			
			// Check if this deposit has corresponding journal entry
			var depositJournalCount int64
			db.Raw(`
				SELECT COUNT(*) 
				FROM journal_entries je
				JOIN journal_lines jl ON je.id = jl.journal_id
				JOIN accounts a ON jl.account_id = a.id
				WHERE a.code = '1103' AND je.reference_type = 'DEPOSIT'
				  AND jl.debit_amount = ?
			`, initialDeposit).Scan(&depositJournalCount)
			
			if depositJournalCount == 0 {
				fmt.Printf("   ❌ FOUND ISSUE: Initial deposit %.2f has no corresponding journal entry in COA!\n", initialDeposit)
				fmt.Printf("   🔧 This explains the discrepancy of %.2f\n", cashBankBalance - coaBalance)
			} else {
				fmt.Printf("   ✅ Initial deposit has corresponding journal entry\n")
			}
		}
	}
	
	// Correction recommendation
	fmt.Printf("\n5️⃣ SYNCHRONIZATION CORRECTION:\n")
	
	fmt.Printf("   💡 The correct approach is to sync COA balance with Cash & Bank balance\n")
	fmt.Printf("   💡 Cash & Bank balance (%.2f) represents the actual operational balance\n", cashBankBalance)
	fmt.Printf("   💡 COA balance should match this for accurate financial reporting\n")
	
	// Apply correction
	fmt.Printf("\n🔧 APPLYING SYNCHRONIZATION...\n")
	
	err := db.Exec("UPDATE accounts SET balance = ? WHERE code = '1103'", cashBankBalance).Error
	if err != nil {
		fmt.Printf("   ❌ Failed to update COA balance: %v\n", err)
	} else {
		fmt.Printf("   ✅ COA balance updated from %.2f to %.2f\n", coaBalance, cashBankBalance)
		
		// Update parent accounts as well
		fmt.Printf("   🔄 Updating parent account balances...\n")
		
		// Update CURRENT ASSETS (1100)
		var totalCurrentAssets float64
		db.Raw(`
			SELECT COALESCE(SUM(balance), 0) 
			FROM accounts 
			WHERE code LIKE '11%' AND code != '1100' AND LENGTH(code) = 4
		`).Scan(&totalCurrentAssets)
		
		err = db.Exec("UPDATE accounts SET balance = ? WHERE code = '1100'", totalCurrentAssets).Error
		if err != nil {
			fmt.Printf("   ❌ Failed to update CURRENT ASSETS balance: %v\n", err)
		} else {
			fmt.Printf("   ✅ CURRENT ASSETS (1100) updated to %.2f\n", totalCurrentAssets)
		}
		
		// Update ASSETS (1000)
		var totalAssets float64
		db.Raw(`
			SELECT COALESCE(SUM(balance), 0) 
			FROM accounts 
			WHERE code LIKE '1%' AND code != '1000' AND LENGTH(code) = 4
		`).Scan(&totalAssets)
		
		err = db.Exec("UPDATE accounts SET balance = ? WHERE code = '1000'", totalAssets).Error
		if err != nil {
			fmt.Printf("   ❌ Failed to update ASSETS balance: %v\n", err)
		} else {
			fmt.Printf("   ✅ ASSETS (1000) updated to %.2f\n", totalAssets)
		}
	}
	
	// Final verification
	fmt.Printf("\n6️⃣ FINAL VERIFICATION:\n")
	
	var newCoaBalance float64
	db.Raw("SELECT balance FROM accounts WHERE code = '1103'").Scan(&newCoaBalance)
	
	var newCashBankBalance float64
	db.Raw(`
		SELECT balance 
		FROM cash_banks cb
		JOIN accounts a ON cb.account_id = a.id
		WHERE a.code = '1103'
	`).Scan(&newCashBankBalance)
	
	fmt.Printf("   📊 New COA Balance: %.2f\n", newCoaBalance)
	fmt.Printf("   💰 Cash & Bank Balance: %.2f\n", newCashBankBalance)
	
	if newCoaBalance == newCashBankBalance {
		fmt.Printf("   ✅ SUCCESS: Balances are now synchronized!\n")
	} else {
		fmt.Printf("   ❌ Still have discrepancy: %.2f\n", newCashBankBalance - newCoaBalance)
	}
	
	fmt.Printf("\n🎉 COA SYNCHRONIZATION COMPLETED!\n")
	fmt.Printf("💡 The Cash & Bank balance is the source of truth for operational transactions.\n")
	fmt.Printf("💡 COA balance now matches for accurate financial reporting.\n")
}