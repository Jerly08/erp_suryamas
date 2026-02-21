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
		// Let's find the journal entry for the Sale. Sale ID: 5. Invoice 0005.
		var sale models.Sale
		if err := tx.Where("invoice_number LIKE ?", "%0005%").First(&sale).Error; err != nil {
			return err
		}

		fmt.Printf("Sale ID: %d, Invoice: %s, Total: %.2f\n", sale.ID, sale.InvoiceNumber, sale.TotalAmount)

		var journal models.SSOTJournalEntry
		if err := tx.Where("source_type = 'SALES' AND source_id = ?", sale.ID).First(&journal).Error; err != nil {
			fmt.Printf("No SSOT journal found for Sale ID %d\n", sale.ID)
			return nil
		}

		fmt.Printf("Journal ID: %d, Code: %s, TotalDebit: %.2f, TotalCredit: %.2f\n",
			journal.ID, journal.EntryNumber, journal.TotalDebit, journal.TotalCredit)

		var lines []models.SSOTJournalLine
		tx.Where("journal_id = ?", journal.ID).Find(&lines)

		fmt.Println("--- LINE ITEMS FOR SALE JOURNAL ---")
		for _, l := range lines {
			d, _ := l.DebitAmount.Float64()
			c, _ := l.CreditAmount.Float64()
			fmt.Printf("Acc ID: %d | Debit: %.2f | Credit: %.2f | Desc: %s\n",
				l.AccountID, d, c, l.Description)
		}

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
