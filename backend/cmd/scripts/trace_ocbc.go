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

	fmt.Println("=== OCBC BALANCE TRACE ===")
	fmt.Println()

	// 1. Show ALL accounts with code starting 1101
	fmt.Println("--- All accounts with code like 1101% ---")
	type Acct struct {
		ID       uint
		Code     string
		Name     string
		Balance  float64
		IsHeader bool
		ParentID *uint
	}
	var accts []Acct
	db.Raw(`SELECT id, code, name, balance, is_header, parent_id FROM accounts WHERE code LIKE '1101%' AND deleted_at IS NULL ORDER BY code`).Scan(&accts)
	for _, a := range accts {
		pid := uint(0)
		if a.ParentID != nil {
			pid = *a.ParentID
		}
		fmt.Printf("  ID=%d code=%s name=%s balance=%.0f is_header=%v parent_id=%d\n", a.ID, a.Code, a.Name, a.Balance, a.IsHeader, pid)
	}

	// 2. Get OCBC cash_bank and its linked account
	fmt.Println()
	fmt.Println("--- OCBC Cash Bank details ---")
	type CBDetail struct {
		ID        uint
		Name      string
		Balance   float64
		AccountID uint
		AccCode   string
		AccName   string
		AccBal    float64
	}
	var ocbc CBDetail
	db.Raw(`SELECT cb.id, cb.name, cb.balance, cb.account_id, a.code as acc_code, a.name as acc_name, a.balance as acc_bal
		FROM cash_banks cb JOIN accounts a ON cb.account_id = a.id
		WHERE cb.name LIKE '%OCBC%' AND cb.deleted_at IS NULL`).Scan(&ocbc)
	fmt.Printf("  CashBank: ID=%d, Name=%s, Balance=%.0f, AccountID=%d\n", ocbc.ID, ocbc.Name, ocbc.Balance, ocbc.AccountID)
	fmt.Printf("  Linked Account: Code=%s, Name=%s, Balance=%.0f\n", ocbc.AccCode, ocbc.AccName, ocbc.AccBal)
	fmt.Printf("  DISCREPANCY: %.0f\n", ocbc.Balance-ocbc.AccBal)

	// 3. CashBank Transactions for OCBC
	fmt.Println()
	fmt.Println("--- CashBank Transactions for OCBC (most recent 20) ---")
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
	db.Raw(`SELECT id, reference_type as ref_type, reference_id as ref_id, amount, balance_after as bal_after,
		TO_CHAR(transaction_date, 'YYYY-MM-DD HH24:MI') as tx_date, COALESCE(notes,'') as notes
		FROM cash_bank_transactions 
		WHERE cash_bank_id = ? AND deleted_at IS NULL
		ORDER BY id DESC LIMIT 20`, ocbc.ID).Scan(&cbTxs)
	var totalCBAmount float64
	for _, t := range cbTxs {
		totalCBAmount += t.Amount
		fmt.Printf("  #%d [%s] ref=%s/%d amount=%.0f bal_after=%.0f %s\n", t.ID, t.TxDate, t.RefType, t.RefID, t.Amount, t.BalAfter, t.Notes)
	}
	fmt.Printf("  Sum of shown tx amounts: %.0f\n", totalCBAmount)

	// 4. Journal Lines affecting OCBC account (account_id = ocbc.AccountID)
	fmt.Println()
	fmt.Printf("--- Legacy Journal Lines for Account ID=%d (OCBC, most recent 20) ---\n", ocbc.AccountID)
	type JLine struct {
		JEID      uint
		JEDate    string
		JEDesc    string
		JERef     string
		JERefType string
		LineDesc  string
		Debit     float64
		Credit    float64
	}
	var jlines []JLine
	db.Raw(`SELECT je.id as je_id, TO_CHAR(je.date, 'YYYY-MM-DD') as je_date, 
		je.description as je_desc, COALESCE(je.reference,'') as je_ref,
		COALESCE(je.reference_type,'') as je_ref_type,
		jl.description as line_desc, jl.debit_amount as debit, jl.credit_amount as credit
		FROM journal_lines jl
		JOIN journal_entries je ON jl.journal_entry_id = je.id
		WHERE jl.account_id = ? AND je.deleted_at IS NULL
		ORDER BY je.id DESC LIMIT 20`, ocbc.AccountID).Scan(&jlines)
	var totalDebit, totalCredit float64
	for _, j := range jlines {
		totalDebit += j.Debit
		totalCredit += j.Credit
		fmt.Printf("  JE#%d [%s] %s | ref=%s refType=%s | D=%.0f C=%.0f | %s\n",
			j.JEID, j.JEDate, j.JEDesc, j.JERef, j.JERefType, j.Debit, j.Credit, j.LineDesc)
	}
	fmt.Printf("  Totals: Debit=%.0f, Credit=%.0f, Net=%.0f\n", totalDebit, totalCredit, totalDebit-totalCredit)

	// 5. SSOT Journal Lines affecting OCBC account
	fmt.Println()
	fmt.Printf("--- SSOT Journal Lines for Account ID=%d (OCBC, most recent 20) ---\n", ocbc.AccountID)
	type SSOTLine struct {
		EntryID   uint
		EntryDate string
		EntryDesc string
		Ref       string
		SrcType   string
		LineDesc  string
		Debit     float64
		Credit    float64
		Status    string
	}
	var ssotLines []SSOTLine
	db.Raw(`SELECT e.id as entry_id, TO_CHAR(e.date, 'YYYY-MM-DD') as entry_date,
		e.description as entry_desc, COALESCE(e.reference,'') as ref,
		COALESCE(e.source_type,'') as src_type,
		COALESCE(l.description,'') as line_desc, l.debit_amount as debit, l.credit_amount as credit,
		e.status
		FROM ssot_journal_lines l
		JOIN ssot_journal_entries e ON l.journal_entry_id = e.id
		WHERE l.account_id = ? AND e.deleted_at IS NULL
		ORDER BY e.id DESC LIMIT 20`, ocbc.AccountID).Scan(&ssotLines)
	var ssotDebit, ssotCredit float64
	for _, s := range ssotLines {
		ssotDebit += s.Debit
		ssotCredit += s.Credit
		fmt.Printf("  SSOT#%d [%s] %s | ref=%s src=%s status=%s | D=%.0f C=%.0f | %s\n",
			s.EntryID, s.EntryDate, s.EntryDesc, s.Ref, s.SrcType, s.Status, s.Debit, s.Credit, s.LineDesc)
	}
	fmt.Printf("  Totals: Debit=%.0f, Credit=%.0f, Net=%.0f\n", ssotDebit, ssotCredit, ssotDebit-ssotCredit)

	// 6. Check ALL journal lines (legacy) total for this account to compute expected balance
	fmt.Println()
	fmt.Printf("--- FULL Legacy Journal Totals for Account ID=%d ---\n", ocbc.AccountID)
	type TotalInfo struct {
		TotalDebit  float64
		TotalCredit float64
		Count       int64
	}
	var legacyTotals TotalInfo
	db.Raw(`SELECT COALESCE(SUM(jl.debit_amount),0) as total_debit, 
		COALESCE(SUM(jl.credit_amount),0) as total_credit,
		COUNT(*) as count
		FROM journal_lines jl
		JOIN journal_entries je ON jl.journal_entry_id = je.id
		WHERE jl.account_id = ? AND je.deleted_at IS NULL`, ocbc.AccountID).Scan(&legacyTotals)
	fmt.Printf("  Legacy: %d lines, Debit=%.0f, Credit=%.0f, Net(D-C)=%.0f\n",
		legacyTotals.Count, legacyTotals.TotalDebit, legacyTotals.TotalCredit,
		legacyTotals.TotalDebit-legacyTotals.TotalCredit)

	// 7. Check ALL SSOT journal lines total
	var ssotTotals TotalInfo
	db.Raw(`SELECT COALESCE(SUM(l.debit_amount),0) as total_debit, 
		COALESCE(SUM(l.credit_amount),0) as total_credit,
		COUNT(*) as count
		FROM ssot_journal_lines l
		JOIN ssot_journal_entries e ON l.journal_entry_id = e.id
		WHERE l.account_id = ? AND e.deleted_at IS NULL`, ocbc.AccountID).Scan(&ssotTotals)
	fmt.Printf("  SSOT: %d lines, Debit=%.0f, Credit=%.0f, Net(D-C)=%.0f\n",
		ssotTotals.Count, ssotTotals.TotalDebit, ssotTotals.TotalCredit,
		ssotTotals.TotalDebit-ssotTotals.TotalCredit)

	// 8. CashBank transaction total
	type CBTotal struct {
		TotalIn   float64
		TotalOut  float64
		NetAmount float64
		Count     int64
	}
	var cbTotal CBTotal
	db.Raw(`SELECT 
		COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END),0) as total_in,
		COALESCE(SUM(CASE WHEN amount < 0 THEN amount ELSE 0 END),0) as total_out,
		COALESCE(SUM(amount),0) as net_amount,
		COUNT(*) as count
		FROM cash_bank_transactions
		WHERE cash_bank_id = ? AND deleted_at IS NULL`, ocbc.ID).Scan(&cbTotal)
	fmt.Printf("  CashBank Txs: %d txs, In=%.0f, Out=%.0f, Net=%.0f\n",
		cbTotal.Count, cbTotal.TotalIn, cbTotal.TotalOut, cbTotal.NetAmount)

	// 9. Check initial balance
	fmt.Println()
	fmt.Println("--- Initial/Opening Balance Info ---")
	type InitBal struct {
		InitialBalance float64
	}
	var initBal InitBal
	db.Raw(`SELECT COALESCE(initial_balance, 0) as initial_balance FROM cash_banks WHERE id = ?`, ocbc.ID).Scan(&initBal)
	fmt.Printf("  OCBC initial_balance: %.0f\n", initBal.InitialBalance)
	fmt.Printf("  Expected CB balance: initial + net_cb_txs = %.0f + %.0f = %.0f\n",
		initBal.InitialBalance, cbTotal.NetAmount, initBal.InitialBalance+cbTotal.NetAmount)
	fmt.Printf("  Actual CB balance: %.0f\n", ocbc.Balance)

	// 10. Summary
	fmt.Println()
	fmt.Println("=== SUMMARY ===")
	fmt.Printf("  CashBank balance: %.0f\n", ocbc.Balance)
	fmt.Printf("  COA balance: %.0f\n", ocbc.AccBal)
	fmt.Printf("  Discrepancy: %.0f (CB - COA)\n", ocbc.Balance-ocbc.AccBal)
	fmt.Printf("  Legacy journal net (D-C): %.0f\n", legacyTotals.TotalDebit-legacyTotals.TotalCredit)
	fmt.Printf("  SSOT journal net (D-C): %.0f\n", ssotTotals.TotalDebit-ssotTotals.TotalCredit)
	fmt.Printf("  CashBank tx net: %.0f\n", cbTotal.NetAmount)
}
