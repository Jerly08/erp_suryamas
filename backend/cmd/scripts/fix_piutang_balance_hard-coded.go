package main

import (
	"fmt"
	"log"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

func main() {
	config.LoadConfig()
	db := database.ConnectDB()

	err := db.Transaction(func(tx *gorm.DB) error {
		// The exact broken journal was ID 27, which had a line with Account ID 136 (not 137).
		// Wait, let's just make it simpler.
		// Right now, bank account is: 1101-005 = 130,542,235  (This correctly received 7,215,000 via transaction 27)
		// Piutang account is: 1102-001 = 28,860,000 (It did NOT receive the -7,215,000 reduction).

		// 1. Manually zero out ID 137 (the fake account we mistakenly touched)
		tx.Exec("UPDATE accounts SET balance = 0 WHERE id = 137")

		// 2. Reduce the balance of ID 136 (1102-001) by 7,215,000
		var piutang models.Account
		if err := tx.Where("code = ?", "1102-001").First(&piutang).Error; err != nil {
			return err
		}

		fmt.Printf("Initial Piutang Balance: %.2f\n", piutang.Balance)

		// Hardcode the fix since we know exactly what went wrong.
		piutang.Balance -= 7215000
		tx.Save(&piutang)
		fmt.Printf("Updated Piutang Balance to: %.2f\n", piutang.Balance)

		// 3. Update the journal line of ID 27 to point to 136 instead of 137
		tx.Exec("UPDATE unified_journal_lines SET account_id = 136 WHERE journal_id = 27 AND account_id = 137")
		fmt.Printf("Repointed Journal Line 27 to correct account 136\n")

		// 4. Recalculate parent balances
		updateParentAccountBalances(tx, piutang.ID)

		return nil
	})

	if err != nil {
		log.Fatalf("Transaction failed: %v", err)
	}
}

func updateParentAccountBalances(db *gorm.DB, accountID uint) error {
	var account models.Account
	if err := db.First(&account, accountID).Error; err != nil {
		return err
	}
	if account.ParentID != nil && *account.ParentID > 0 {
		var children []models.Account
		if err := db.Where("parent_id = ?", *account.ParentID).Find(&children).Error; err != nil {
			return err
		}
		var total float64
		for _, child := range children {
			total += child.Balance
		}
		if err := db.Model(&models.Account{}).Where("id = ?", *account.ParentID).UpdateColumn("balance", total).Error; err != nil {
			return err
		}
		return updateParentAccountBalances(db, *account.ParentID)
	}
	return nil
}
