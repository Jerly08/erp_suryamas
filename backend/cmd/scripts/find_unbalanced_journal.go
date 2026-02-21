package main

import (
	"fmt"
	"math"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"
)

func main() {
	config.LoadConfig()
	db := database.ConnectDB()

	var journals []models.SSOTJournalEntry
	db.Where("status = 'POSTED' AND is_balanced = false").Find(&journals)

	if len(journals) == 0 {
		// Manual calculation just to be sure
		db.Where("status = 'POSTED'").Find(&journals)
		for _, j := range journals {
			if math.Abs(j.TotalDebit.InexactFloat64()-j.TotalCredit.InexactFloat64()) > 0.01 {
				fmt.Printf("UNBALANCED JOURNAL FOUND (via manual check):\n")
				fmt.Printf("ID: %d, Reference: %s\n", j.ID, j.Reference)
				fmt.Printf("Total Debit: %.2f\n", j.TotalDebit.InexactFloat64())
				fmt.Printf("Total Credit: %.2f\n", j.TotalCredit.InexactFloat64())
				fmt.Printf("Difference: %.2f\n\n", j.TotalDebit.InexactFloat64()-j.TotalCredit.InexactFloat64())

				var lines []models.SSOTJournalLine
				db.Where("journal_id = ?", j.ID).Find(&lines)
				for _, l := range lines {
					fmt.Printf("  Line ID: %d, Acc: %d, Dr: %.2f, Cr: %.2f, Desc: %s\n", l.ID, l.AccountID, l.DebitAmount.InexactFloat64(), l.CreditAmount.InexactFloat64(), l.Description)
				}
				fmt.Println("-------------------------------------------")
			}
		}
	} else {
		for _, j := range journals {
			fmt.Printf("UNBALANCED JOURNAL FOUND (flagged is_balanced=false):\n")
			fmt.Printf("ID: %d, Reference: %s\n", j.ID, j.Reference)
			fmt.Printf("Total Debit: %.2f\n", j.TotalDebit.InexactFloat64())
			fmt.Printf("Total Credit: %.2f\n", j.TotalCredit.InexactFloat64())

			var lines []models.SSOTJournalLine
			db.Where("journal_id = ?", j.ID).Find(&lines)
			for _, l := range lines {
				fmt.Printf("  Line ID: %d, Acc: %d, Dr: %.2f, Cr: %.2f, Desc: %s\n", l.ID, l.AccountID, l.DebitAmount.InexactFloat64(), l.CreditAmount.InexactFloat64(), l.Description)
			}
			fmt.Println("-------------------------------------------")
		}
	}
}
