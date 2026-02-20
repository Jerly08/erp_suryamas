package main

import (
	"fmt"
	"log"
	"os"

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

	f, _ := os.Create("trace_result.md")
	defer f.Close()

	w := func(format string, args ...interface{}) {
		s := fmt.Sprintf(format, args...)
		fmt.Print(s)
		f.WriteString(s)
	}

	w("# Balance Trace Results\n\n")

	// 1. Journal entries
	w("## Journal Entries (all 6)\n\n")
	type JE struct {
		ID      uint
		Desc    string
		RefType string
		RefID   *uint
		Status  string
		Created string
	}
	var jes []JE
	db.Raw(`SELECT id, description as "desc", COALESCE(reference_type,'') as ref_type,
		reference_id as ref_id, COALESCE(status,'') as status,
		TO_CHAR(created_at, 'MM-DD HH24:MI') as created
		FROM journal_entries ORDER BY id`).Scan(&jes)

	for _, j := range jes {
		rid := uint(0)
		if j.RefID != nil {
			rid = *j.RefID
		}
		w("**JE#%d** %s | ref=%s/%d status=%s created=%s\n", j.ID, j.Desc, j.RefType, rid, j.Status, j.Created)

		type Line struct {
			AccID   uint
			AccCode string
			Debit   float64
			Credit  float64
		}
		var lines []Line
		db.Raw(`SELECT jl.account_id as acc_id, COALESCE(a.code,'?') as acc_code,
			jl.debit_amount as debit, jl.credit_amount as credit
			FROM journal_lines jl LEFT JOIN accounts a ON jl.account_id = a.id
			WHERE jl.journal_entry_id = ? ORDER BY jl.line_number`, j.ID).Scan(&lines)
		for _, l := range lines {
			mark := ""
			if l.AccID == 448 {
				mark = " **<<OCBC**"
			}
			if l.AccID == 135 {
				mark = " **<<KAS PARENT**"
			}
			w("- AccID=%d [%s] D=%.0f C=%.0f%s\n", l.AccID, l.AccCode, l.Debit, l.Credit, mark)
		}
		w("\n")
	}

	// 2. All payments
	w("## Payment Records\n\n")
	type Pay struct {
		ID     uint
		Code   string
		Amount float64
		Status string
		Date   string
	}
	var pays []Pay
	db.Raw(`SELECT id, code, amount, status, TO_CHAR(date, 'MM-DD') as date FROM payments WHERE deleted_at IS NULL ORDER BY id`).Scan(&pays)
	for _, p := range pays {
		w("PAY#%d %s [%s] amount=%.0f status=%s\n", p.ID, p.Code, p.Date, p.Amount, p.Status)
	}

	// 3. Cash bank transactions
	w("\n## CashBank Transactions (OCBC cb_id=49)\n\n")
	type TX struct {
		ID       uint
		RefType  string
		RefID    uint
		Amount   float64
		BalAfter float64
		Date     string
		Notes    string
	}
	var txs []TX
	db.Raw(`SELECT id, reference_type as ref_type, reference_id as ref_id, amount, balance_after as bal_after,
		TO_CHAR(transaction_date, 'MM-DD HH24:MI') as date, COALESCE(notes,'') as notes
		FROM cash_bank_transactions WHERE cash_bank_id = 49 AND deleted_at IS NULL ORDER BY id`).Scan(&txs)

	for _, t := range txs {
		w("TX#%d [%s] %s/%d amt=%.0f bal=%.0f %s\n", t.ID, t.Date, t.RefType, t.RefID, t.Amount, t.BalAfter, t.Notes)
	}

	// 4. Current balances
	w("\n## Current Balances\n\n")
	var cbBal, coaBal float64
	db.Raw(`SELECT balance FROM cash_banks WHERE id = 49`).Scan(&cbBal)
	db.Raw(`SELECT balance FROM accounts WHERE id = 448`).Scan(&coaBal)
	w("CB balance: %.0f\n", cbBal)
	w("COA(448) balance: %.0f\n", coaBal)
	w("Discrepancy: %.0f\n", cbBal-coaBal)

	// 5. KAS parent balance
	var kasBal float64
	db.Raw(`SELECT balance FROM accounts WHERE id = 135`).Scan(&kasBal)
	w("KAS parent(135) balance: %.0f\n", kasBal)

	w("\nDone.\n")
}
