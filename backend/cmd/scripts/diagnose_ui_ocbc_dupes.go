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
		var accs []models.Account
		tx.Unscoped().Where("code = '1101-005'").Find(&accs)

		fmt.Printf("Found %d accounts with code '1101-005' (including deleted):\n", len(accs))
		for _, a := range accs {
			deleted := "NO"
			if a.DeletedAt.Valid {
				deleted = "YES"
			}

			// Let's also check how much credit is sitting on THIS EXACT account ID
			var lines []models.SSOTJournalLine
			tx.Where("account_id = ?", a.ID).Find(&lines)
			var totalD, totalC float64
			for _, l := range lines {
				d, _ := l.DebitAmount.Float64()
				c, _ := l.CreditAmount.Float64()

				var je models.SSOTJournalEntry
				tx.First(&je, l.JournalID)
				if je.Status == "POSTED" && je.DeletedAt == nil && je.EntryDate.Format("2006-01-02") <= "2026-02-21" {
					totalD += d
					totalC += c
				}
			}

			fmt.Printf("ID: %d | Code: %s | Deleted: %s | Lines D: %.2f | Lines C: %.2f\n",
				a.ID, a.Code, deleted, totalD, totalC)
		}

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
