package main

import (
	"fmt"
	"log"
	"time"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"
)

func main() {
	config.LoadConfig()
	db := database.ConnectDB()

	// 1. Get Accounts (GL)
	var ocbcAccount models.Account
	if err := db.Where("code = ?", "1101-005").First(&ocbcAccount).Error; err != nil {
		log.Fatalf("OCBC account not found: %v", err)
	}

	// 2. Get CashBanks
	var cashBank models.CashBank
	if err := db.Where("account_id = ?", ocbcAccount.ID).First(&cashBank).Error; err != nil {
		log.Fatalf("No CashBank found for account %d: %v", ocbcAccount.ID, err)
	}

	fmt.Printf("BEFORE TEST :\n  Account.Balance: %.2f\n  CashBank.Balance: %.2f\n", ocbcAccount.Balance, cashBank.Balance)

	// Simulate an SSOT update to Account.Balance directly (e.g. +1000)
	testBal := ocbcAccount.Balance + 1000

	fmt.Printf("\nTRIGGERING UPDATE -> Account.Balance = %.2f\n", testBal)
	if err := db.Model(&ocbcAccount).Update("balance", testBal).Error; err != nil {
		log.Fatalf("Update failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond) // Give trigger a microsecond

	// Fetch both again
	var accAfter models.Account
	var cbAfter models.CashBank
	db.Where("id = ?", ocbcAccount.ID).First(&accAfter)
	db.Where("account_id = ?", ocbcAccount.ID).First(&cbAfter)

	fmt.Printf("\nAFTER TEST :\n  Account.Balance: %.2f\n  CashBank.Balance: %.2f\n", accAfter.Balance, cbAfter.Balance)
	if accAfter.Balance == cbAfter.Balance {
		fmt.Println("🚀 TRIGGER SUCCESS: CashBank automatically synced!")
	} else {
		fmt.Println("❌ TRIGGER FAILED: CashBank balance is out of sync!")
	}

	// Revert the test balance by letting the SSOT rebuild compute it
	fmt.Println("\nRestoring exact balance from SSOT...")
	db.Exec("SELECT sync_account_balance_from_ssot(?)", ocbcAccount.ID)

	// Fetch final
	db.Where("id = ?", ocbcAccount.ID).First(&accAfter)
	db.Where("account_id = ?", ocbcAccount.ID).First(&cbAfter)
	fmt.Printf("\nRESTORED :\n  Account.Balance: %.2f\n  CashBank.Balance: %.2f\n", accAfter.Balance, cbAfter.Balance)
}
