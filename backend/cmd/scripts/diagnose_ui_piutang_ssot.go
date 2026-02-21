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
		// Piutang Usaha - Pihak Ketiga (1102-001) has ID 136
		var lines []models.SSOTJournalLine
		if err := tx.Where("account_id = ?", 136).Find(&lines).Error; err != nil {
			return err
		}

		fmt.Println("--- LINE ITEMS FOR 1102-001 (Account ID: 136) ---")
		var totalDebit float64
		var totalCredit float64
		for _, l := range lines {
			d, _ := l.DebitAmount.Float64()
			c, _ := l.CreditAmount.Float64()
			totalDebit += d
			totalCredit += c
			fmt.Printf("JournalID: %d | LineID: %d | Debit: %.2f | Credit: %.2f | Desc: %s\n",
				l.JournalID, l.ID, d, c, l.Description)
		}

		fmt.Printf("TOTAL DEBIT: %.2f\n", totalDebit)
		fmt.Printf("TOTAL CREDIT: %.2f\n", totalCredit)
		fmt.Printf("SSOT CALCULATED BALANCE (Debit - Credit): %.2f\n", totalDebit-totalCredit)

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
