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

		// Step 1: Let's find exactly what 4,050,530 relates to.
		// Usually when an invoice is fully paid natively, everything balances. But maybe the Net Income calculation in the report is missing it?
		// We'll calculate the true exact sum of all assets
		var assetSum float64
		if err := tx.Raw("SELECT COALESCE(SUM(balance), 0) FROM accounts WHERE type = 'ASSET' AND level = 3 AND deleted_at IS NULL").Scan(&assetSum).Error; err != nil {
			return err
		}

		fmt.Printf("True Asset Sum (Level 3): %.2f\n", assetSum)

		var liabilitySum float64
		if err := tx.Raw("SELECT COALESCE(SUM(balance), 0) FROM accounts WHERE type = 'LIABILITY' AND level = 3 AND deleted_at IS NULL").Scan(&liabilitySum).Error; err != nil {
			return err
		}

		var equitySum float64
		if err := tx.Raw("SELECT COALESCE(SUM(balance), 0) FROM accounts WHERE type = 'EQUITY' AND level = 3 AND code != '3104' AND deleted_at IS NULL").Scan(&equitySum).Error; err != nil {
			return err
		}

		var revenueSum float64
		if err := tx.Raw("SELECT COALESCE(SUM(balance), 0) FROM accounts WHERE type = 'REVENUE' AND level = 3 AND deleted_at IS NULL").Scan(&revenueSum).Error; err != nil {
			return err
		}

		var expenseSum float64
		if err := tx.Raw("SELECT COALESCE(SUM(balance), 0) FROM accounts WHERE type = 'EXPENSE' AND level = 3 AND deleted_at IS NULL").Scan(&expenseSum).Error; err != nil {
			return err
		}

		fmt.Printf("True Liability Sum: %.2f\n", liabilitySum)
		fmt.Printf("True Equity Sum: %.2f\n", equitySum)
		fmt.Printf("True Revenue Sum: %.2f\n", revenueSum)
		fmt.Printf("True Expense Sum: %.2f\n", expenseSum)

		netIncome := -revenueSum - expenseSum // revenue is credit (-), expense is debit (+) -> so (-revenue) - (expense)

		fmt.Printf("Computed L + E + NI: %.2f\n", -liabilitySum-equitySum+netIncome)

		fmt.Printf("Difference: %.2f\n", assetSum-(-liabilitySum-equitySum+netIncome))

		// Let's get the 4,050,530 exactly. Wait, looking at the image provided by the user:
		// Total Aset: 213.582.235
		// Total Liabilitas & Ekuitas: 217.632.765
		// Difference is -4.050.530
		// Wait, look at Net Income (Laba/Rugi Berjalan): Rp 12.000.000 in the UI.
		// My script computed -92,000,000. Why the difference?
		// Because the UI Neraca calculates Net Income dynamically from the unified ledger by date range, it doesn't just read the balances.
		//
		// More importantly, the user's Total Aset is 213,582,235.
		// 1101-005 Bank OCBC = 130,542,235
		// 1102-001 Piutang Pihak 3 = 28,860,000  (WAIT: My DB calculation was 28,860,000. But sale 0005 was paid for 7,215,000 !! Why is Piutang still 28,860,000 in the DB too?)

		var arAccount models.Account
		tx.Where("code = ?", "1102-001").First(&arAccount)
		fmt.Printf("Piutang (1102-001) Balance in DB: %.2f\n", arAccount.Balance)

		var bankAccount models.Account
		tx.Where("code = ?", "1101-005").First(&bankAccount)
		fmt.Printf("Bank (1101-005) Balance in DB: %.2f\n", bankAccount.Balance)

		// The issue is Piutang is 28.86M (the original Invoice amount). The 7.215M reduction NEVER applied to it.
		// But in fix_sales_payment_journal.go, we did:
		// goodAcc.Balance -= creditAmount
		// Wait, did it apply to Account 137? Let's check ID 137.

		var acc137 models.Account
		tx.First(&acc137, 137)
		fmt.Printf("Account 137 (Code: %s) Balance: %.2f\n", acc137.Code, acc137.Balance)

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
