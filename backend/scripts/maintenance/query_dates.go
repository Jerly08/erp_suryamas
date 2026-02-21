package main

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := "postgres://postgres:postgres@localhost/sistem_akuntansi?sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	type Result struct {
		ID           uint
		EntryDate    string
		Status       string
		Code         string
		DebitAmount  float64
		CreditAmount float64
	}

	var results []Result

	query := `
		SELECT uje.id, uje.entry_date, uje.status, a.code, ujl.debit_amount, ujl.credit_amount
		FROM unified_journal_lines ujl
		JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
		JOIN accounts a ON a.id = ujl.account_id
		WHERE (a.code LIKE '1101%' OR a.code LIKE '3101%')
		  AND uje.deleted_at IS NULL
		  AND a.deleted_at IS NULL
		ORDER BY uje.entry_date ASC
	`

	if err := db.Raw(query).Scan(&results).Error; err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	fmt.Println("--- DEBUG RESULTS ---")
	for _, r := range results {
		fmt.Printf("JID:%d Date:%s Status:%s Acc:%s Dr:%.2f Cr:%.2f\n", r.ID, r.EntryDate[:10], r.Status, r.Code, r.DebitAmount, r.CreditAmount)
	}
	fmt.Println("---------------------")
}
