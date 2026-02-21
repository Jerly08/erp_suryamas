package main

import (
	"fmt"
	"log"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"
)

func main() {
	config.LoadConfig()
	db := database.ConnectDB()

	// 1. Get Accounts (GL) Balance for OCBC (code 1101-005)
	var ocbcAccount models.Account
	if err := db.Where("code = ?", "1101-005").First(&ocbcAccount).Error; err != nil {
		log.Fatalf("OCBC account not found: %v", err)
	}
	fmt.Printf("1. GL Account [1101-005 %s]: Balance = %.2f\n", ocbcAccount.Name, ocbcAccount.Balance)

	// 2. Get CashBanks module Balance
	var cashBanks []models.CashBank
	if err := db.Where("account_id = ?", ocbcAccount.ID).Find(&cashBanks).Error; err != nil {
		log.Fatalf("No CashBank found for account %d: %v", ocbcAccount.ID, err)
	}

	totalCashBankBalance := 0.0
	for _, cb := range cashBanks {
		bal := cb.Balance
		fmt.Printf("2. CashBank module [%s]: Balance = %.2f\n", cb.Name, bal)
		totalCashBankBalance += bal
	}
	fmt.Printf("   Total CashBank module balance for OCBC: %.2f\n", totalCashBankBalance)

	fmt.Println("\n--- ANALYZING SSOT JOURNALS FOR 1101-005 ---")

	// 3. Sum of all POSTED journals for 1101-005
	var debitTotal, creditTotal float64

	// Query to get all lines using pure GORM to avoid syntax issues
	var ssotLines []models.SSOTJournalLine

	err := db.Joins("JOIN unified_journal_ledger ON unified_journal_ledger.id = unified_journal_lines.journal_id").
		Preload("Journal").
		Where("unified_journal_lines.account_id = ?", ocbcAccount.ID).
		Where("unified_journal_ledger.status = ?", "POSTED").
		Where("unified_journal_ledger.deleted_at IS NULL").
		Find(&ssotLines).Error

	if err != nil {
		log.Fatalf("Fail: %v", err)
	}

	for _, l := range ssotLines {
		d, _ := l.DebitAmount.Float64()
		c, _ := l.CreditAmount.Float64()

		debitTotal += d
		creditTotal += c
		fmt.Printf("   JID:%d | Src:%s(ID:%v) | D:%.2f | C:%.2f | Desc: %s\n",
			l.JournalID, l.Journal.SourceType, l.Journal.SourceID, d, c, l.Description)
	}

	netBalance := debitTotal - creditTotal
	fmt.Printf("\n3. SSOT Journal Sum: Debit=%.2f, Credit=%.2f, NET=%.2f\n", debitTotal, creditTotal, netBalance)

	// Summary
	fmt.Println("\n--- SUMMARY OF DISCREPENCIES ---")
	fmt.Printf("Neraca (GL Account) : %.2f\n", ocbcAccount.Balance)
	fmt.Printf("Dashboard CashBank  : %.2f\n", totalCashBankBalance)
	fmt.Printf("SSOT Journal Math   : %.2f\n", netBalance)
	fmt.Printf("Difference (GL - CashBank): %.2f\n", ocbcAccount.Balance-totalCashBankBalance)
}
