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

	// 1. Show recent payment journal entries and which accounts they target
	fmt.Println("=== ALL Recent Payment Journal Entries (Legacy) ===")
	type JEntry struct {
		ID      uint
		Date    string
		Desc    string
		RefType string
		Ref     string
		RefID   *uint
	}
	var entries []JEntry
	db.Raw(`SELECT id, TO_CHAR(date, 'YYYY-MM-DD') as date, description as "desc",
		COALESCE(reference_type,'') as ref_type, COALESCE(reference,'') as ref,
		reference_id as ref_id
		FROM journal_entries
		WHERE deleted_at IS NULL
		AND (reference_type LIKE '%PAYMENT%' OR reference_type LIKE '%payment%' OR description LIKE '%Payment%')
		ORDER BY id DESC LIMIT 20`).Scan(&entries)

	for _, e := range entries {
		rid := uint(0)
		if e.RefID != nil {
			rid = *e.RefID
		}
		fmt.Printf("\nJE#%d [%s] %s | refType=%s ref=%s refID=%d\n", e.ID, e.Date, e.Desc, e.RefType, e.Ref, rid)

		// Show lines for this entry
		type Line struct {
			AccountID uint
			AccCode   string
			AccName   string
			LineDesc  string
			Debit     float64
			Credit    float64
		}
		var lines []Line
		db.Raw(`SELECT jl.account_id, a.code as acc_code, a.name as acc_name,
			COALESCE(jl.description,'') as line_desc, jl.debit_amount as debit, jl.credit_amount as credit
			FROM journal_lines jl
			LEFT JOIN accounts a ON jl.account_id = a.id
			WHERE jl.journal_entry_id = ?
			ORDER BY jl.line_number`, e.ID).Scan(&lines)
		for _, l := range lines {
			marker := ""
			if l.AccountID == 448 {
				marker = " <<<< OCBC"
			} else if l.AccountID == 135 {
				marker = " <<<< KAS PARENT"
			}
			fmt.Printf("  AccID=%d [%s] %s | D=%.0f C=%.0f | %s%s\n",
				l.AccountID, l.AccCode, l.AccName, l.Debit, l.Credit, l.LineDesc, marker)
		}
	}

	// 2. Show which accounts have gotten balance updates via SQL UPDATE
	// Check if the resolveCashBankAccountID function is correctly resolving
	fmt.Println()
	fmt.Println("=== Cash Bank to Account mapping ===")
	type CBMapping struct {
		CBID      uint
		CBName    string
		AccountID uint
		AccCode   string
		AccName   string
	}
	var mappings []CBMapping
	db.Raw(`SELECT cb.id as cb_id, cb.name as cb_name, cb.account_id,
		a.code as acc_code, a.name as acc_name
		FROM cash_banks cb
		LEFT JOIN accounts a ON cb.account_id = a.id
		WHERE cb.deleted_at IS NULL
		ORDER BY cb.id`).Scan(&mappings)
	for _, m := range mappings {
		fmt.Printf("  CB#%d '%s' -> Account ID=%d [%s] '%s'\n", m.CBID, m.CBName, m.AccountID, m.AccCode, m.AccName)
	}

	// 3. Check if manual UPDATE accounts SET balance was applied to wrong account
	fmt.Println()
	fmt.Println("=== All accounts with non-zero balance (1101-xxx range) ===")
	type AccBal struct {
		ID       uint
		Code     string
		Name     string
		Balance  float64
		IsHeader bool
	}
	var accBals []AccBal
	db.Raw(`SELECT id, code, name, balance, is_header FROM accounts 
		WHERE code LIKE '1101%' AND deleted_at IS NULL AND balance != 0
		ORDER BY code`).Scan(&accBals)
	for _, a := range accBals {
		fmt.Printf("  ID=%d [%s] '%s' balance=%.0f is_header=%v\n", a.ID, a.Code, a.Name, a.Balance, a.IsHeader)
	}

	// 4. Check ALL journal lines that reference account 135 (KAS parent)
	fmt.Println()
	fmt.Println("=== Journal Lines targeting Account 135 (KAS parent 1101) ===")
	type JLine struct {
		JEID     uint
		JEDate   string
		JEDesc   string
		RefType  string
		Debit    float64
		Credit   float64
		LineDesc string
	}
	var kasLines []JLine
	db.Raw(`SELECT je.id as je_id, TO_CHAR(je.date, 'YYYY-MM-DD') as je_date,
		je.description as je_desc, COALESCE(je.reference_type,'') as ref_type,
		jl.debit_amount as debit, jl.credit_amount as credit,
		COALESCE(jl.description,'') as line_desc
		FROM journal_lines jl
		JOIN journal_entries je ON jl.journal_entry_id = je.id
		WHERE jl.account_id = 135 AND je.deleted_at IS NULL
		ORDER BY je.id`).Scan(&kasLines)
	var kasD, kasC float64
	for _, j := range kasLines {
		kasD += j.Debit
		kasC += j.Credit
		fmt.Printf("  JE#%d [%s] %s | D=%.0f C=%.0f | %s\n", j.JEID, j.JEDate, j.JEDesc, j.Debit, j.Credit, j.LineDesc)
	}
	fmt.Printf("  Totals: Debit=%.0f Credit=%.0f Net=%.0f\n", kasD, kasC, kasD-kasC)

	// 5. Where does the COA balance 134886686 come from for account 448?
	// If there are 0 journal lines for 448, but balance=134886686, it must come from direct SQL UPDATE
	fmt.Println()
	fmt.Println("=== Direct balance analysis ===")
	fmt.Println("Account 448 (OCBC) has 0 journal lines but balance=134886686")
	fmt.Println("This means balance was set directly via SQL UPDATE, not through journal posting")
	fmt.Println("The fix script earlier may have SET balance = cash_banks.balance")

	// Check SSOT journal lines for account 448
	var ssotCount int64
	db.Raw(`SELECT COUNT(*) FROM ssot_journal_lines l
		JOIN ssot_journal_entries e ON l.journal_entry_id = e.id
		WHERE l.account_id = 448 AND e.deleted_at IS NULL`).Scan(&ssotCount)
	fmt.Printf("SSOT journal lines for account 448: %d\n", ssotCount)
}
