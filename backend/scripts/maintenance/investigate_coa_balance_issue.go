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
	
	fmt.Println("🔍 INVESTIGATING COA BALANCE SYNCHRONIZATION ISSUE")
	
	// Check Bank Mandiri (1103) balance discrepancy
	fmt.Printf("\n1️⃣ BALANCE COMPARISON FOR BANK MANDIRI (1103):\n")
	
	// Get balance from cash_banks table (actual balance)
	var cashBankBalance float64
	db.Raw(`
		SELECT balance 
		FROM cash_banks cb
		JOIN accounts a ON cb.account_id = a.id
		WHERE a.code = '1103'
	`).Scan(&cashBankBalance)
	
	// Get balance from accounts table (COA balance)
	var coaBalance float64
	db.Raw("SELECT balance FROM accounts WHERE code = '1103'").Scan(&coaBalance)
	
	fmt.Printf("   Cash & Bank Balance: %.2f\n", cashBankBalance)
	fmt.Printf("   COA Balance: %.2f\n", coaBalance)
	fmt.Printf("   Discrepancy: %.2f\n", coaBalance - cashBankBalance)
	
	// Check all transactions affecting Bank Mandiri
	fmt.Printf("\n2️⃣ BANK MANDIRI TRANSACTION HISTORY:\n")
	
	// Get all cash bank transactions
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
		SELECT cbt.id, cbt.amount, cbt.balance_after, cbt.reference_type, 
		       cbt.reference_id, cbt.notes, cbt.created_at
		FROM cash_bank_transactions cbt
		JOIN cash_banks cb ON cbt.cash_bank_id = cb.id
		JOIN accounts a ON cb.account_id = a.id
		WHERE a.code = '1103'
		ORDER BY cbt.created_at
	`).Scan(&cbTransactions)
	
	fmt.Printf("   Cash Bank Transactions (%d):\n", len(cbTransactions))
	for _, tx := range cbTransactions {
		fmt.Printf("     %s | %s %d | %.2f -> %.2f | %s\n", 
			tx.CreatedAt[:19], tx.ReferenceType, tx.ReferenceID, tx.Amount, tx.BalanceAfter, tx.Notes)
	}
	
	// Check journal entries affecting Bank Mandiri
	fmt.Printf("\n3️⃣ JOURNAL ENTRIES AFFECTING BANK MANDIRI:\n")
	
	var journalLines []struct {
		JournalID     uint    `json:"journal_id"`
		Description   string  `json:"description"`
		DebitAmount   float64 `json:"debit_amount"`
		CreditAmount  float64 `json:"credit_amount"`
		ReferenceType string  `json:"reference_type"`
		Status        string  `json:"status"`
		CreatedAt     string  `json:"created_at"`
	}
	
	db.Raw(`
		SELECT jl.journal_id, jl.description, jl.debit_amount, jl.credit_amount,
		       je.reference_type, je.status, je.created_at
		FROM journal_lines jl
		JOIN journal_entries je ON jl.journal_id = je.id
		JOIN accounts a ON jl.account_id = a.id
		WHERE a.code = '1103' AND je.status = 'POSTED'
		ORDER BY je.created_at
	`).Scan(&journalLines)
	
	fmt.Printf("   Posted Journal Entries (%d):\n", len(journalLines))
	totalDebit := 0.0
	totalCredit := 0.0
	
	for _, jl := range journalLines {
		fmt.Printf("     J%d | %s | D:%.2f C:%.2f | %s | %s\n", 
			jl.JournalID, jl.ReferenceType, jl.DebitAmount, jl.CreditAmount, jl.Status, jl.CreatedAt[:19])
		totalDebit += jl.DebitAmount
		totalCredit += jl.CreditAmount
	}
	
	fmt.Printf("   Total Journal Debit: %.2f\n", totalDebit)
	fmt.Printf("   Total Journal Credit: %.2f\n", totalCredit)
	fmt.Printf("   Net Journal Effect: %.2f (Debit - Credit)\n", totalDebit - totalCredit)
	
	// Check unified journal lines (SSOT)
	fmt.Printf("\n4️⃣ UNIFIED JOURNAL LINES (SSOT):\n")
	
	var unifiedLines []struct {
		JournalID     uint    `json:"journal_id"`
		Description   string  `json:"description"`
		DebitAmount   string  `json:"debit_amount"`
		CreditAmount  string  `json:"credit_amount"`
		CreatedAt     string  `json:"created_at"`
	}
	
	db.Raw(`
		SELECT ujl.journal_id, ujl.description, ujl.debit_amount, ujl.credit_amount, ujl.created_at
		FROM unified_journal_lines ujl
		JOIN accounts a ON ujl.account_id = a.id
		WHERE a.code = '1103'
		ORDER BY ujl.created_at
	`).Scan(&unifiedLines)
	
	fmt.Printf("   Unified Journal Lines (%d):\n", len(unifiedLines))
	for _, ul := range unifiedLines {
		fmt.Printf("     UJ%d | %s | D:%s C:%s | %s\n", 
			ul.JournalID, ul.Description, ul.DebitAmount, ul.CreditAmount, ul.CreatedAt[:19])
	}
	
	// Check simple_ssot_journals
	fmt.Printf("\n5️⃣ SIMPLE SSOT JOURNALS:\n")
	
	var ssotCount int64
	db.Raw("SELECT COUNT(*) FROM simple_ssot_journals WHERE reference LIKE '%1103%' OR description LIKE '%Bank Mandiri%'").Scan(&ssotCount)
	fmt.Printf("   Simple SSOT Journals: %d\n", ssotCount)
	
	// Analysis and recommendations
	fmt.Printf("\n🎯 ANALYSIS & DIAGNOSIS:\n")
	
	if len(cbTransactions) == 0 {
		fmt.Printf("   ❌ No cash bank transactions found for Bank Mandiri\n")
	} else {
		lastBalance := cbTransactions[len(cbTransactions)-1].BalanceAfter
		fmt.Printf("   📊 Last cash bank transaction balance: %.2f\n", lastBalance)
		
		if lastBalance != cashBankBalance {
			fmt.Printf("   ⚠️ WARNING: Transaction balance != cash_banks balance\n")
		}
	}
	
	if coaBalance != cashBankBalance {
		fmt.Printf("   ❌ COA balance is out of sync with cash & bank balance\n")
		fmt.Printf("   🔧 Difference: %.2f (COA is %.2f higher/lower)\n", 
			coaBalance - cashBankBalance, coaBalance - cashBankBalance)
		
		// Suggest correction
		fmt.Printf("\n💡 CORRECTION NEEDED:\n")
		fmt.Printf("   UPDATE accounts SET balance = %.2f WHERE code = '1103';\n", cashBankBalance)
		
		// Execute correction
		fmt.Printf("\n🔧 APPLYING CORRECTION...\n")
		err := db.Exec("UPDATE accounts SET balance = ? WHERE code = '1103'", cashBankBalance).Error
		if err != nil {
			fmt.Printf("   ❌ Failed to update COA balance: %v\n", err)
		} else {
			fmt.Printf("   ✅ COA balance corrected to match cash & bank balance: %.2f\n", cashBankBalance)
			
			// Verify correction
			var newCoaBalance float64
			db.Raw("SELECT balance FROM accounts WHERE code = '1103'").Scan(&newCoaBalance)
			fmt.Printf("   ✅ Verified new COA balance: %.2f\n", newCoaBalance)
		}
	} else {
		fmt.Printf("   ✅ COA balance is correctly synchronized\n")
	}
	
	// Check other accounts that might have similar issues
	fmt.Printf("\n6️⃣ CHECKING OTHER ACCOUNTS FOR SIMILAR ISSUES:\n")
	
	var otherAccounts []struct {
		Code          string  `json:"code"`
		Name          string  `json:"name"`
		CoaBalance    float64 `json:"coa_balance"`
		CashBankBalance float64 `json:"cash_bank_balance"`
		Difference    float64 `json:"difference"`
	}
	
	db.Raw(`
		SELECT a.code, a.name, a.balance as coa_balance, 
		       COALESCE(cb.balance, 0) as cash_bank_balance,
		       a.balance - COALESCE(cb.balance, 0) as difference
		FROM accounts a
		LEFT JOIN cash_banks cb ON cb.account_id = a.id
		WHERE a.type = 'ASSET' AND a.code LIKE '11%'
		  AND ABS(a.balance - COALESCE(cb.balance, 0)) > 0.01
		ORDER BY ABS(difference) DESC
	`).Scan(&otherAccounts)
	
	if len(otherAccounts) > 0 {
		fmt.Printf("   ⚠️ Other accounts with balance discrepancies:\n")
		for _, acc := range otherAccounts {
			fmt.Printf("     %s (%s): COA=%.2f, CashBank=%.2f, Diff=%.2f\n", 
				acc.Code, acc.Name, acc.CoaBalance, acc.CashBankBalance, acc.Difference)
		}
		
		// Correct all discrepancies
		fmt.Printf("\n🔧 CORRECTING ALL DISCREPANCIES...\n")
		for _, acc := range otherAccounts {
			err := db.Exec("UPDATE accounts SET balance = ? WHERE code = ?", acc.CashBankBalance, acc.Code).Error
			if err != nil {
				fmt.Printf("     ❌ Failed to correct %s: %v\n", acc.Code, err)
			} else {
				fmt.Printf("     ✅ Corrected %s: %.2f -> %.2f\n", acc.Code, acc.CoaBalance, acc.CashBankBalance)
			}
		}
	} else {
		fmt.Printf("   ✅ No other accounts with balance discrepancies found\n")
	}
	
	fmt.Printf("\n🎉 COA BALANCE SYNCHRONIZATION CHECK COMPLETED!\n")
}