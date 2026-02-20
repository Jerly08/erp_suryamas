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

	// Step 1: Show before
	fmt.Println("=== BEFORE FIX ===")
	type Pair struct {
		CbName  string
		CbBal   float64
		AccID   uint
		AccCode string
		AccBal  float64
	}
	var pairs []Pair
	db.Raw(`SELECT cb.name as cb_name, cb.balance as cb_bal, a.id as acc_id, a.code as acc_code, a.balance as acc_bal
		FROM cash_banks cb JOIN accounts a ON cb.account_id = a.id
		WHERE cb.deleted_at IS NULL AND a.deleted_at IS NULL ORDER BY cb.id`).Scan(&pairs)
	for _, p := range pairs {
		fmt.Printf("  %s: CB=%.0f, COA(%s)=%.0f, diff=%.0f\n", p.CbName, p.CbBal, p.AccCode, p.AccBal, p.CbBal-p.AccBal)
	}

	// Step 2: Fix - set accounts.balance = cash_banks.balance for ALL linked accounts
	fmt.Println("\n=== FIXING ===")
	for _, p := range pairs {
		if p.CbBal == p.AccBal {
			fmt.Printf("  %s: already in sync, skipping\n", p.CbName)
			continue
		}
		fmt.Printf("  %s: setting COA(%s, ID=%d) from %.0f to %.0f\n", p.CbName, p.AccCode, p.AccID, p.AccBal, p.CbBal)
		db.Exec("UPDATE accounts SET balance = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", p.CbBal, p.AccID)
	}

	// Step 3: Also fix KAS parent (1101) - it should NOT hold individual balance for a header account
	fmt.Println("\n=== Checking KAS parent (1101) ===")
	type AccInfo struct {
		ID       uint
		Code     string
		Name     string
		Balance  float64
		IsHeader bool
	}
	var kasParent AccInfo
	db.Raw("SELECT id, code, name, balance, is_header FROM accounts WHERE code = '1101' AND deleted_at IS NULL").Scan(&kasParent)
	fmt.Printf("  KAS parent: ID=%d balance=%.0f is_header=%v\n", kasParent.ID, kasParent.Balance, kasParent.IsHeader)

	// If KAS parent is a header account, its balance should be the SUM of its children
	if kasParent.IsHeader {
		var childSum float64
		db.Raw("SELECT COALESCE(SUM(balance), 0) FROM accounts WHERE parent_id = ? AND deleted_at IS NULL", kasParent.ID).Scan(&childSum)
		fmt.Printf("  Sum of children: %.0f\n", childSum)
		if kasParent.Balance != childSum {
			fmt.Printf("  Fixing KAS parent balance from %.0f to %.0f\n", kasParent.Balance, childSum)
			db.Exec("UPDATE accounts SET balance = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", childSum, kasParent.ID)
		}
	} else {
		// Check if KAS (1101) is a cash_bank - if not linked to any CB, its balance might be from direct updates
		var linkedCB uint
		db.Raw("SELECT COUNT(*) FROM cash_banks WHERE account_id = ? AND deleted_at IS NULL", kasParent.ID).Scan(&linkedCB)
		fmt.Printf("  Linked cash_banks count: %d\n", linkedCB)
	}

	// Step 4: Verify after fix
	fmt.Println("\n=== AFTER FIX ===")
	var after []Pair
	db.Raw(`SELECT cb.name as cb_name, cb.balance as cb_bal, a.id as acc_id, a.code as acc_code, a.balance as acc_bal
		FROM cash_banks cb JOIN accounts a ON cb.account_id = a.id
		WHERE cb.deleted_at IS NULL AND a.deleted_at IS NULL ORDER BY cb.id`).Scan(&after)
	for _, p := range after {
		fmt.Printf("  %s: CB=%.0f, COA(%s)=%.0f, diff=%.0f\n", p.CbName, p.CbBal, p.AccCode, p.AccBal, p.CbBal-p.AccBal)
	}

	// Show KAS parent final
	db.Raw("SELECT id, code, name, balance, is_header FROM accounts WHERE code = '1101' AND deleted_at IS NULL").Scan(&kasParent)
	fmt.Printf("  KAS parent (1101): balance=%.0f\n", kasParent.Balance)

	fmt.Println("\nDone!")
}
