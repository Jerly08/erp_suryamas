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

	// The problem: Journal ID 27 has a credit line for account 169 (1201  Legacy Piutang).
	// We need to change it to account 137 (1102-001 New Piutang).

	err := db.Transaction(func(tx *gorm.DB) error {
		var line models.SSOTJournalLine
		if err := tx.Where("journal_id = ? AND account_id = ?", 27, 169).First(&line).Error; err != nil {
			return fmt.Errorf("could not find bad journal line: %v", err)
		}

		creditAmount := line.CreditAmount.InexactFloat64()
		fmt.Printf("Found bad line! Credit amount: %f\n", creditAmount)

		// 1. Update the line to use the correct Account (137)
		line.AccountID = 137
		if err := tx.Save(&line).Error; err != nil {
			return err
		}
		fmt.Println("Journal line updated.")

		// 2. Fix the balance of Account 169 (Legacy Piutang)
		// It was erroneously credited (reduced). So we need to add the amount back.
		var badAcc models.Account
		if err := tx.First(&badAcc, 169).Error; err == nil {
			badAcc.Balance += creditAmount
			tx.Save(&badAcc)
			fmt.Printf("Restored Account 169 balance to: %f\n", badAcc.Balance)
		}

		// 3. Fix the balance of Account 137 (New Piutang)
		// It SHOULD have been credited (reduced). So we subtract it.
		var goodAcc models.Account
		if err := tx.First(&goodAcc, 137).Error; err == nil {
			goodAcc.Balance -= creditAmount
			tx.Save(&goodAcc)
			fmt.Printf("Reduced Account 137 balance to: %f\n", goodAcc.Balance)

			// Update parent balances
			updateParentAccountBalances(tx, goodAcc.ID)
		}

		fmt.Println("Fix applied successfully!")
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
