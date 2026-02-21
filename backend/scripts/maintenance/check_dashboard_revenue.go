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

	var dboardTotal float64
	err = db.Raw(`
		SELECT COALESCE(SUM(net_before_tax), 0)
		FROM sales 
		WHERE date >= '2026-01-01' AND date <= '2026-12-31'
			AND deleted_at IS NULL
			AND status NOT IN ('DRAFT', 'CANCELLED')
	`).Scan(&dboardTotal).Error
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Dashboard Sales query sum: %f\n", dboardTotal)

	var ssotTotal float64
	err = db.Raw(`
        SELECT COALESCE(SUM(ujl.credit_amount - ujl.debit_amount), 0)
		FROM unified_journal_lines ujl
		JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
		JOIN accounts a ON a.id = ujl.account_id
		WHERE a.code LIKE '4%' 
          AND a.deleted_at IS NULL
          AND uje.deleted_at IS NULL
          AND uje.status = 'POSTED'
          AND uje.entry_date >= '2026-01-01' AND uje.entry_date <= '2026-12-31'
    `).Scan(&ssotTotal).Error
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("SSOT Revenue (code 4%%) sum: %f\n", ssotTotal)

	fmt.Println("--- Sales Transactions ---")
	type SaleResult struct {
		ID          uint
		Code        string
		Status      string
		TotalAmount float64
	}
	var sales []SaleResult
	db.Raw(`SELECT id, code, status, net_before_tax FROM sales WHERE date >= '2026-01-01' AND date <= '2026-12-31' AND deleted_at IS NULL`).Scan(&sales)
	for _, s := range sales {
		fmt.Printf("Sale %d | Code: %s | Status: %s | Amount: %.2f\n", s.ID, s.Code, s.Status, s.TotalAmount)
	}

}
