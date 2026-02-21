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
		var piutang models.Account
		if err := tx.Where("code = ?", "1102-001").First(&piutang).Error; err != nil {
			return err
		}

		fmt.Printf("Current Piutang Balance (1102-001): %.2f\n", piutang.Balance)

		// Check the journal line for the payment. Journal 27 is the payment journal.
		var lines []models.SSOTJournalLine
		tx.Where("journal_id = ?", 27).Find(&lines)

		var paymentCreditAmount float64
		for _, l := range lines {
			if l.CreditAmount.InexactFloat64() > 0 {
				paymentCreditAmount = l.CreditAmount.InexactFloat64()
				fmt.Printf("Found payment credit: %.2f on Acc ID: %d\n", paymentCreditAmount, l.AccountID)
			}
		}

		// The payment is 7,215,000. It SHOULD reduce the Piutang Asset.
		// If balance is currently 28,860,000 (which is exactly the full invoice amount),
		// then it means the payment reduction was never applied to 1102-001.
		if piutang.Balance == 28860000 && paymentCreditAmount == 7215000 {
			piutang.Balance -= paymentCreditAmount

			if err := tx.Save(&piutang).Error; err != nil {
				return err
			}
			fmt.Printf("Fixed Piutang Balance (1102-001) to: %.2f\n", piutang.Balance)

			// Recalculate and update parent balances
			updateParentAccountBalances(tx, piutang.ID)
		} else {
			fmt.Printf("No fix applied. Conditions not met: Piutang Balance=%.2f, PaymentCredit=%.2f\n", piutang.Balance, paymentCreditAmount)
		}

		// Let's also check if there's any other account that incorrectly received this 7,215,000.
		// In fix_sales_payment_journal.go, we did:
		// goodAcc.Balance -= creditAmount on Account 137. Wait, is 137 the ID of 1102-001?
		var acc137 models.Account
		if err := tx.First(&acc137, 137).Error; err == nil {
			fmt.Printf("Account 137 Code: %s, Name: %s, Balance: %.2f\n", acc137.Code, acc137.Name, acc137.Balance)
		}

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
