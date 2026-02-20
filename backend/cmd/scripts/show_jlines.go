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
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		log.Fatalf("Failed: %v", err)
	}

	// Simple: just show journal lines with account IDs
	fmt.Println("JE_ID | ACC_ID | ACC_CODE | DEBIT | CREDIT | JE_DESC")
	fmt.Println("------|--------|----------|-------|---------|--------")
	type Row struct {
		JEID    uint
		AccID   uint
		AccCode string
		Debit   float64
		Credit  float64
		JEDesc  string
	}
	var rows []Row
	db.Raw(`SELECT je.id as je_id, jl.account_id as acc_id, 
		COALESCE(a.code,'?') as acc_code,
		jl.debit_amount as debit, jl.credit_amount as credit,
		LEFT(je.description, 40) as je_desc
		FROM journal_lines jl
		JOIN journal_entries je ON jl.journal_entry_id = je.id
		LEFT JOIN accounts a ON jl.account_id = a.id
		WHERE je.deleted_at IS NULL
		ORDER BY je.id, jl.line_number`).Scan(&rows)

	for _, r := range rows {
		mark := ""
		if r.AccID == 448 {
			mark = " <<OCBC"
		}
		if r.AccID == 135 {
			mark = " <<KAS"
		}
		fmt.Printf("%d | %d | %s | %.0f | %.0f | %s%s\n", r.JEID, r.AccID, r.AccCode, r.Debit, r.Credit, r.JEDesc, mark)
	}

	// Count by account
	fmt.Println("\nJournal lines per account:")
	type AccSum struct {
		AccID   uint
		AccCode string
		Lines   int64
		SumD    float64
		SumC    float64
	}
	var sums []AccSum
	db.Raw(`SELECT jl.account_id as acc_id, COALESCE(a.code,'?') as acc_code,
		COUNT(*) as lines, SUM(jl.debit_amount) as sum_d, SUM(jl.credit_amount) as sum_c
		FROM journal_lines jl
		JOIN journal_entries je ON jl.journal_entry_id = je.id
		LEFT JOIN accounts a ON jl.account_id = a.id
		WHERE je.deleted_at IS NULL
		GROUP BY jl.account_id, a.code
		ORDER BY SUM(jl.credit_amount) DESC`).Scan(&sums)
	for _, s := range sums {
		fmt.Printf("AccID=%d [%s]: %d lines, D=%.0f C=%.0f net=%.0f\n", s.AccID, s.AccCode, s.Lines, s.SumD, s.SumC, s.SumD-s.SumC)
	}
}
