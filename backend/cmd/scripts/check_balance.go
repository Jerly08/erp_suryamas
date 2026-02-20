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

	fmt.Println("CASH_BANK vs COA BALANCE CHECK")
	fmt.Println("===============================")

	type Row struct {
		CbName  string
		CbBal   float64
		AccCode string
		AccBal  float64
		Diff    float64
	}
	var rows []Row
	db.Raw(`
		SELECT cb.name as cb_name, cb.balance as cb_bal, 
		       a.code as acc_code, a.balance as acc_bal,
		       (cb.balance - a.balance) as diff
		FROM cash_banks cb
		JOIN accounts a ON cb.account_id = a.id
		WHERE cb.deleted_at IS NULL AND a.deleted_at IS NULL
		ORDER BY cb.id
	`).Scan(&rows)

	total := 0
	for _, r := range rows {
		mark := "OK"
		if r.Diff != 0 {
			mark = "MISMATCH"
			total++
		}
		fmt.Printf("[%s] %s: CB=%.0f | COA(%s)=%.0f | diff=%.0f\n", mark, r.CbName, r.CbBal, r.AccCode, r.AccBal, r.Diff)
	}
	fmt.Printf("\nTotal mismatches: %d / %d\n", total, len(rows))
}
