package main

import (
	"fmt"
	"math"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

func main() {
	config.LoadConfig()
	db := database.ConnectDB()

	// 1. Recalculate and update the unified journal totals
	err := db.Transaction(func(tx *gorm.DB) error {
		var journals []models.SSOTJournalEntry
		if err := tx.Where("status = 'POSTED'").Find(&journals).Error; err != nil {
			return err
		}

		updatedCount := 0
		for _, j := range journals {
			var totals struct {
				TotalDebit  float64
				TotalCredit float64
			}
			tx.Raw("SELECT COALESCE(SUM(debit_amount), 0) as total_debit, COALESCE(SUM(credit_amount), 0) as total_credit FROM unified_journal_lines WHERE journal_id = ?", j.ID).Scan(&totals)

			jDebit := j.TotalDebit.InexactFloat64()
			jCredit := j.TotalCredit.InexactFloat64()

			if math.Abs(jDebit-totals.TotalDebit) > 0.01 || math.Abs(jCredit-totals.TotalCredit) > 0.01 {
				fmt.Printf("Fixing Journal %d:\n", j.ID)
				fmt.Printf("  Record: Dr=%.2f, Cr=%.2f\n", jDebit, jCredit)
				fmt.Printf("  Actual: Dr=%.2f, Cr=%.2f\n", totals.TotalDebit, totals.TotalCredit)

				if err := tx.Exec("UPDATE unified_journal_ledger SET total_debit = ?, total_credit = ?, is_balanced = ? WHERE id = ?",
					totals.TotalDebit, totals.TotalCredit, math.Abs(totals.TotalDebit-totals.TotalCredit) < 0.01, j.ID).Error; err != nil {
					return err
				}
				updatedCount++
			}
		}
		fmt.Printf("Successfully updated %d unbalanced journal headers.\n", updatedCount)
		return nil
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}
