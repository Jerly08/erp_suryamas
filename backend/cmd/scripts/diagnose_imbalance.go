package main

import (
	"fmt"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"
)

func main() {
	config.LoadConfig()
	db := database.ConnectDB()

	// 1. Check if the Unified Ledger itself is balanced
	type LedgerStatus struct {
		TotalDebit  float64 `json:"total_debit"`
		TotalCredit float64 `json:"total_credit"`
	}

	var status LedgerStatus
	db.Raw("SELECT COALESCE(SUM(total_debit), 0) as total_debit, COALESCE(SUM(total_credit), 0) as total_credit FROM unified_journal_ledger WHERE status = 'POSTED'").Scan(&status)
	fmt.Printf("--- UNIFIED LEDGER STATUS ---\n")
	fmt.Printf("Total Debit:  %.2f\n", status.TotalDebit)
	fmt.Printf("Total Credit: %.2f\n", status.TotalCredit)
	fmt.Printf("Difference:   %.2f\n\n", status.TotalDebit-status.TotalCredit)

	// 2. Check individual unbalanced journals
	var unbalanced []models.SSOTJournalEntry
	db.Where("status = 'POSTED' AND is_balanced = false").Find(&unbalanced)
	if len(unbalanced) > 0 {
		fmt.Printf("--- UNBALANCED JOURNALS ---\n")
		for _, j := range unbalanced {
			fmt.Printf("Journal ID %d: Dr: %.2f, Cr: %.2f\n", j.ID, j.TotalDebit.InexactFloat64(), j.TotalCredit.InexactFloat64())
		}
	} else {
		fmt.Println("No unbalanced journals found.")
	}

	// 3. Check for specific amount 4,050,530
	var suspectLines []models.SSOTJournalLine
	db.Where("debit_amount = ? OR credit_amount = ?", 4050530, 4050530).Find(&suspectLines)
	if len(suspectLines) > 0 {
		fmt.Printf("\n--- LINES WITH EXACT AMOUNT 4,050,530 ---\n")
		for _, l := range suspectLines {
			fmt.Printf("Line ID: %d, Journal ID: %d, Account ID: %d, Dr: %.2f, Cr: %.2f\n", l.ID, l.JournalID, l.AccountID, l.DebitAmount.InexactFloat64(), l.CreditAmount.InexactFloat64())
		}
	}

	// 4. Check for half the amount (2,025,265), which might be posted to the wrong side
	db.Where("debit_amount = ? OR credit_amount = ?", 2025265, 2025265).Find(&suspectLines)
	if len(suspectLines) > 0 {
		fmt.Printf("\n--- LINES WITH HALF AMOUNT (wrong side posting) 2,025,265 ---\n")
		for _, l := range suspectLines {
			fmt.Printf("Line ID: %d, Journal ID: %d, Account ID: %d, Dr: %.2f, Cr: %.2f\n", l.ID, l.JournalID, l.AccountID, l.DebitAmount.InexactFloat64(), l.CreditAmount.InexactFloat64())
		}
	}

	// 5. Compare account balances sum vs unified ledger sum
	var accountBalancesSum float64
	db.Raw("SELECT COALESCE(SUM(balance), 0) FROM accounts WHERE deleted_at IS NULL").Scan(&accountBalancesSum)

	fmt.Printf("\n--- ACCOUNT BALANCES VS LEDGER ---\n")
	fmt.Printf("Sum of all accounts.balance: %.2f\n", accountBalancesSum)

	// Recalculate what the exact Neraca output should be
	var assets, liabilities, equities float64
	db.Raw("SELECT COALESCE(SUM(balance), 0) FROM accounts WHERE type = 'ASSET' AND is_header = false AND deleted_at IS NULL").Scan(&assets)
	db.Raw("SELECT COALESCE(SUM(balance), 0) FROM accounts WHERE type = 'LIABILITY' AND is_header = false AND deleted_at IS NULL").Scan(&liabilities)
	db.Raw("SELECT COALESCE(SUM(balance), 0) FROM accounts WHERE type = 'EQUITY' AND is_header = false AND deleted_at IS NULL").Scan(&equities)

	var revenues, expenses float64
	db.Raw("SELECT COALESCE(SUM(balance), 0) FROM accounts WHERE type = 'REVENUE' AND is_header = false AND deleted_at IS NULL").Scan(&revenues)
	db.Raw("SELECT COALESCE(SUM(balance), 0) FROM accounts WHERE type = 'EXPENSE' AND is_header = false AND deleted_at IS NULL").Scan(&expenses)

	netIncome := revenues - expenses
	totalRightSide := liabilities + equities + netIncome

	fmt.Printf("\n--- NERACA CALCULATION ---\n")
	fmt.Printf("Assets:                  %.2f\n", assets)
	fmt.Printf("Liabilities:             %.2f\n", liabilities)
	fmt.Printf("Equity (ex Net Income):  %.2f\n", equities)
	fmt.Printf("Total Revenue:           %.2f\n", revenues)
	fmt.Printf("Total Expense:           %.2f\n", expenses)
	fmt.Printf("Net Income:              %.2f\n", netIncome)
	fmt.Printf("Total Liab & Equity:     %.2f\n", totalRightSide)
	fmt.Printf("DIFFERENCE:              %.2f\n", assets-totalRightSide)
}
