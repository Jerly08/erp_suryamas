package main

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	dsn := "host=localhost user=postgres password=postgres dbname=sistem_akuntansi port=5432 sslmode=disable"

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// OCBC account_id = 448
	accountID := uint(448)

	fmt.Println("=== ALL Legacy Journal Lines for OCBC (account_id=448) ===")
	type JLine struct {
		JEID      uint
		JEDate    string
		JERefType string
		JERef     string
		LineDesc  string
		Debit     float64
		Credit    float64
	}
	var jlines []JLine
	db.Raw(`SELECT je.id as je_id, TO_CHAR(je.date, 'YYYY-MM-DD') as je_date,
		COALESCE(je.reference_type,'') as je_ref_type, COALESCE(je.reference,'') as je_ref,
		COALESCE(jl.description,'') as line_desc, jl.debit_amount as debit, jl.credit_amount as credit
		FROM journal_lines jl
		JOIN journal_entries je ON jl.journal_entry_id = je.id
		WHERE jl.account_id = ? AND je.deleted_at IS NULL
		ORDER BY je.id`, accountID).Scan(&jlines)

	var runD, runC float64
	for _, j := range jlines {
		runD += j.Debit
		runC += j.Credit
		fmt.Printf("JE#%d [%s] %s ref=%s | D=%.0f C=%.0f | running(D=%.0f C=%.0f net=%.0f) | %s\n",
			j.JEID, j.JEDate, j.JERefType, j.JERef, j.Debit, j.Credit, runD, runC, runD-runC, j.LineDesc)
	}
	fmt.Printf("\nTotal: %d lines, Debit=%.0f, Credit=%.0f, Net(D-C)=%.0f\n", len(jlines), runD, runC, runD-runC)

	// CashBank transactions
	fmt.Println()
	fmt.Println("=== ALL CashBank Transactions for OCBC ===")
	type CBTx struct {
		ID       uint
		RefType  string
		RefID    uint
		Amount   float64
		BalAfter float64
		TxDate   string
		Notes    string
	}
	var cbTxs []CBTx
	// Get OCBC cash_bank_id
	var cbID uint
	db.Raw(`SELECT id FROM cash_banks WHERE account_id = ?`, accountID).Scan(&cbID)
	db.Raw(`SELECT id, reference_type as ref_type, reference_id as ref_id, amount, balance_after as bal_after,
		TO_CHAR(transaction_date, 'YYYY-MM-DD') as tx_date, COALESCE(notes,'') as notes
		FROM cash_bank_transactions
		WHERE cash_bank_id = ? AND deleted_at IS NULL
		ORDER BY id`, cbID).Scan(&cbTxs)

	var runCB float64
	for _, t := range cbTxs {
		runCB += t.Amount
		fmt.Printf("TX#%d [%s] %s ref_id=%d | amount=%.0f | running=%.0f bal_after=%.0f | %s\n",
			t.ID, t.TxDate, t.RefType, t.RefID, t.Amount, runCB, t.BalAfter, t.Notes)
	}
	fmt.Printf("\nTotal: %d txs, Net=%.0f\n", len(cbTxs), runCB)

	fmt.Println()
	fmt.Printf("OCBC CashBank current balance: %.0f\n", 0.0) // Will get fresh
	var cbBal float64
	db.Raw(`SELECT balance FROM cash_banks WHERE id = ?`, cbID).Scan(&cbBal)
	fmt.Printf("OCBC CashBank balance (from DB): %.0f\n", cbBal)
	var coaBal float64
	db.Raw(`SELECT balance FROM accounts WHERE id = ?`, accountID).Scan(&coaBal)
	fmt.Printf("OCBC COA balance (from DB): %.0f\n", coaBal)
	fmt.Printf("Discrepancy: %.0f\n", cbBal-coaBal)
}
