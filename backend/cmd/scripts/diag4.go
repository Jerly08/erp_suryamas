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
	db, err := gorm.Open(postgres.Open("host=localhost user=postgres password=postgres dbname=sistem_akuntansi port=5432 sslmode=disable"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal(err)
	}

	f, _ := os.Create("C:\\temp\\diag4.txt")
	defer f.Close()

	w := func(format string, args ...interface{}) { fmt.Fprintf(f, format+"\n", args...) }

	// 1. All unified journal ledgers
	type JLInfo struct {
		ID          uint
		JournalType string
		SourceType  string
		Reference   string
		Description string
		TotalDebit  float64
		TotalCredit float64
	}
	var jls []JLInfo
	db.Raw("SELECT id, journal_type, source_type, reference, description, total_debit, total_credit FROM unified_journal_ledgers ORDER BY id").Scan(&jls)

	w("== Unified Journal Ledgers ==")
	for _, j := range jls {
		w("  J#%d Type=%s Source=%s Ref=%s D:%.2f C:%.2f | %s",
			j.ID, j.JournalType, j.SourceType, j.Reference, j.TotalDebit, j.TotalCredit, j.Description)
	}

	// 2. All unified journal lines
	type UJLInfo struct {
		ID           uint
		JournalID    uint
		AccountID    uint
		DebitAmount  float64
		CreditAmount float64
		Description  string
	}
	var ujls []UJLInfo
	db.Raw("SELECT id, journal_id, account_id, debit_amount, credit_amount, description FROM unified_journal_lines ORDER BY journal_id, id").Scan(&ujls)

	w("\n== All Unified Journal Lines ==")
	for _, u := range ujls {
		w("  UJL#%d J#%d Acc=%d D:%.2f C:%.2f | %s",
			u.ID, u.JournalID, u.AccountID, u.DebitAmount, u.CreditAmount, u.Description)
	}

	// 3. All accounts with their balance
	type AccInfo struct {
		ID      uint
		Code    string
		Name    string
		Balance float64
	}
	var accs []AccInfo
	db.Raw("SELECT id, code, name, balance FROM accounts WHERE deleted_at IS NULL AND balance != 0 ORDER BY code").Scan(&accs)

	w("\n== Accounts with Non-Zero Balance ==")
	for _, a := range accs {
		w("  ID=%d Code=%s Name=%s Balance=%.2f", a.ID, a.Code, a.Name, a.Balance)
	}

	// 4. Calculate what accounts.balance SHOULD be from journal lines
	type AccCalc struct {
		AccountID     uint
		Code          string
		Name          string
		SumDebit      float64
		SumCredit     float64
		CalcBalance   float64
		ActualBalance float64
	}
	var calcs []AccCalc
	db.Raw(`
		SELECT 
			a.id as account_id,
			a.code,
			a.name,
			COALESCE(SUM(ujl.debit_amount), 0) as sum_debit,
			COALESCE(SUM(ujl.credit_amount), 0) as sum_credit,
			COALESCE(SUM(ujl.debit_amount), 0) - COALESCE(SUM(ujl.credit_amount), 0) as calc_balance,
			a.balance as actual_balance
		FROM accounts a
		LEFT JOIN unified_journal_lines ujl ON ujl.account_id = a.id
		WHERE a.deleted_at IS NULL
		GROUP BY a.id, a.code, a.name, a.balance
		HAVING ABS(COALESCE(SUM(ujl.debit_amount), 0) - COALESCE(SUM(ujl.credit_amount), 0) - a.balance) > 0.01
		ORDER BY a.code
	`).Scan(&calcs)

	w("\n== Accounts where Calculated Balance != Actual Balance ==")
	for _, c := range calcs {
		diff := c.CalcBalance - c.ActualBalance
		w("  ID=%d Code=%s %s | Calc(D-C)=%.2f Actual=%.2f DIFF=%.2f",
			c.AccountID, c.Code, c.Name, c.CalcBalance, c.ActualBalance, diff)
	}

	w("\nDone")
}
