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

	// 1. Check ALL journal_entries (not just payment)
	fmt.Println("=== Total journal_entries count ===")
	var totalJE int64
	db.Raw(`SELECT COUNT(*) FROM journal_entries WHERE deleted_at IS NULL`).Scan(&totalJE)
	fmt.Printf("Total active journal entries: %d\n", totalJE)

	// Include soft-deleted
	var totalWithDeleted int64
	db.Raw(`SELECT COUNT(*) FROM journal_entries`).Scan(&totalWithDeleted)
	fmt.Printf("Total including soft-deleted: %d\n", totalWithDeleted)

	// 2. Check ALL journal_lines count
	var totalJL int64
	db.Raw(`SELECT COUNT(*) FROM journal_lines`).Scan(&totalJL)
	fmt.Printf("Total journal lines: %d\n", totalJL)

	// 3. Find the MOST RECENT journal entries regardless of type
	fmt.Println()
	fmt.Println("=== Most Recent 15 Journal Entries ===")
	type JE struct {
		ID      uint
		Date    string
		Desc    string
		RefType string
		RefID   *uint
		Status  string
		TotalD  float64
		TotalC  float64
		Created string
		Deleted *string
	}
	var jes []JE
	db.Raw(`SELECT id, TO_CHAR(COALESCE(entry_date, date, created_at), 'YYYY-MM-DD') as date, 
		description as "desc", COALESCE(reference_type,'') as ref_type,
		reference_id as ref_id, COALESCE(status,'') as status,
		COALESCE(total_debit,0) as total_d, COALESCE(total_credit,0) as total_c,
		TO_CHAR(created_at, 'YYYY-MM-DD HH24:MI:SS') as created,
		CASE WHEN deleted_at IS NOT NULL THEN TO_CHAR(deleted_at, 'YYYY-MM-DD HH24:MI:SS') END as deleted
		FROM journal_entries
		ORDER BY id DESC LIMIT 15`).Scan(&jes)

	for _, j := range jes {
		rid := uint(0)
		if j.RefID != nil {
			rid = *j.RefID
		}
		del := ""
		if j.Deleted != nil {
			del = " DELETED:" + *j.Deleted
		}
		fmt.Printf("JE#%d [%s] %s | ref=%s/%d status=%s | D=%.0f C=%.0f created=%s%s\n",
			j.ID, j.Date, j.Desc, j.RefType, rid, j.Status, j.TotalD, j.TotalC, j.Created, del)

		// Show lines
		type Line struct {
			AccID   uint
			AccCode string
			AccName string
			Debit   float64
			Credit  float64
		}
		var lines []Line
		db.Raw(`SELECT jl.account_id as acc_id, COALESCE(a.code,'?') as acc_code, 
			COALESCE(a.name,'?') as acc_name,
			jl.debit_amount as debit, jl.credit_amount as credit
			FROM journal_lines jl
			LEFT JOIN accounts a ON jl.account_id = a.id
			WHERE jl.journal_entry_id = ?
			ORDER BY jl.line_number`, j.ID).Scan(&lines)
		for _, l := range lines {
			fmt.Printf("  AccID=%d [%s] %s D=%.0f C=%.0f\n", l.AccID, l.AccCode, l.AccName, l.Debit, l.Credit)
		}
	}

	// 4. Check the JournalEntry model - what column is 'date'?
	fmt.Println()
	fmt.Println("=== journal_entries table columns ===")
	type ColInfo struct {
		ColumnName string
		DataType   string
	}
	var cols []ColInfo
	db.Raw(`SELECT column_name, data_type FROM information_schema.columns 
		WHERE table_name = 'journal_entries' ORDER BY ordinal_position`).Scan(&cols)
	for _, c := range cols {
		fmt.Printf("  %s (%s)\n", c.ColumnName, c.DataType)
	}

	// 5. Check CashBank Transactions with recent PAYMENTs matched to payment IDs
	fmt.Println()
	fmt.Println("=== Payment Records (payments table, most recent 10) ===")
	type PayRec struct {
		ID     uint
		Code   string
		Date   string
		Amount float64
		Status string
		Method string
		Notes  string
	}
	var payments []PayRec
	db.Raw(`SELECT id, code, TO_CHAR(date, 'YYYY-MM-DD') as date, amount, status, method, COALESCE(notes,'') as notes
		FROM payments WHERE deleted_at IS NULL ORDER BY id DESC LIMIT 10`).Scan(&payments)
	for _, p := range payments {
		fmt.Printf("PAY#%d [%s] %s | amount=%.0f status=%s method=%s | %s\n",
			p.ID, p.Date, p.Code, p.Amount, p.Status, p.Method, p.Notes)
	}
}
