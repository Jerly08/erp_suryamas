package main

import (
	"fmt"
	"log"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"
)

func main() {
	config.LoadConfig()
	db := database.ConnectDB()

	// 1. Get Accounts (GL) Balance for OCBC (code 1101-005)
	var ocbcAccount models.Account
	if err := db.Where("code = ?", "1101-005").First(&ocbcAccount).Error; err != nil {
		log.Fatalf("OCBC account not found: %v", err)
	}

	// 2. Get CashBanks module Balance
	var cashBanks []models.CashBank
	if err := db.Where("account_id = ?", ocbcAccount.ID).Find(&cashBanks).Error; err != nil {
		log.Fatalf("No CashBank found for account %d: %v", ocbcAccount.ID, err)
	}

	if len(cashBanks) == 0 {
		return
	}

	cb := cashBanks[0]
	fmt.Printf("GL Account [1101-005]: Balance = %.2f\n", ocbcAccount.Balance)
	fmt.Printf("CashBank module       : Balance = %.2f\n", cb.Balance)

	if ocbcAccount.Balance != cb.Balance {
		fmt.Printf("Correcting CashBank balance from %.2f to %.2f\n", cb.Balance, ocbcAccount.Balance)
		if err := db.Model(&cb).Update("balance", ocbcAccount.Balance).Error; err != nil {
			log.Fatalf("Failed to sync: %v", err)
		}
		fmt.Println("Synced successfully!")
	} else {
		fmt.Println("Already in sync.")
	}
}
