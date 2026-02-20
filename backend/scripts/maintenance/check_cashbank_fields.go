package main

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type CashBank struct {
	ID                uint   `gorm:"primaryKey"`
	Code              string
	Name              string
	Type              string
	BankName          string `gorm:"column:bank_name"`
	AccountNo         string `gorm:"column:account_no"`
	AccountHolderName string `gorm:"column:account_holder_name"`
	Branch            string `gorm:"column:branch"`
	Balance           float64
	IsActive          bool `gorm:"column:is_active"`
}

func main() {
	// Get database connection string from environment
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	// Connect to database
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	fmt.Println("=" + string(make([]byte, 80)) + "=")
	fmt.Println("CHECKING CASH_BANKS TABLE - ACCOUNT HOLDER NAME & BRANCH FIELDS")
	fmt.Println("=" + string(make([]byte, 80)) + "=")
	fmt.Println()

	// Query all cash bank accounts
	var accounts []CashBank
	result := db.Table("cash_banks").Where("deleted_at IS NULL").Order("id").Find(&accounts)
	
	if result.Error != nil {
		log.Fatalf("Failed to query cash_banks: %v", result.Error)
	}

	fmt.Printf("Found %d cash/bank accounts\n\n", len(accounts))

	// Display results in table format
	fmt.Println("┌──────┬────────────────┬──────────────────────┬──────────┬─────────────────────┬──────────────────────┬──────────────────────┐")
	fmt.Println("│  ID  │ CODE           │ NAME                 │ TYPE     │ BANK NAME           │ ACCOUNT HOLDER NAME  │ BRANCH               │")
	fmt.Println("├──────┼────────────────┼──────────────────────┼──────────┼─────────────────────┼──────────────────────┼──────────────────────┤")

	for _, acc := range accounts {
		// Truncate long strings for display
		name := truncate(acc.Name, 20)
		bankName := truncate(acc.BankName, 19)
		holderName := truncate(acc.AccountHolderName, 20)
		branch := truncate(acc.Branch, 20)
		
		// Show empty values as "-"
		if bankName == "" {
			bankName = "-"
		}
		if holderName == "" {
			holderName = "-"
		}
		if branch == "" {
			branch = "-"
		}

		fmt.Printf("│ %-4d │ %-14s │ %-20s │ %-8s │ %-19s │ %-20s │ %-20s │\n",
			acc.ID,
			truncate(acc.Code, 14),
			name,
			acc.Type,
			bankName,
			holderName,
			branch,
		)
	}

	fmt.Println("└──────┴────────────────┴──────────────────────┴──────────┴─────────────────────┴──────────────────────┴──────────────────────┘")
	fmt.Println()

	// Summary statistics
	bankAccountsCount := 0
	bankAccountsWithHolder := 0
	bankAccountsWithBranch := 0

	for _, acc := range accounts {
		if acc.Type == "BANK" {
			bankAccountsCount++
			if acc.AccountHolderName != "" {
				bankAccountsWithHolder++
			}
			if acc.Branch != "" {
				bankAccountsWithBranch++
			}
		}
	}

	fmt.Println("SUMMARY:")
	fmt.Printf("  Total Bank Accounts: %d\n", bankAccountsCount)
	fmt.Printf("  With Account Holder Name: %d (%.1f%%)\n", 
		bankAccountsWithHolder, 
		float64(bankAccountsWithHolder)/float64(bankAccountsCount)*100)
	fmt.Printf("  With Branch: %d (%.1f%%)\n", 
		bankAccountsWithBranch, 
		float64(bankAccountsWithBranch)/float64(bankAccountsCount)*100)
	fmt.Println()

	// Detail view for accounts with data
	fmt.Println("DETAILED VIEW (Accounts with Holder Name or Branch):")
	fmt.Println("─────────────────────────────────────────────────────────")
	hasData := false
	for _, acc := range accounts {
		if acc.AccountHolderName != "" || acc.Branch != "" {
			hasData = true
			fmt.Printf("\n📋 ID: %d - %s (%s)\n", acc.ID, acc.Name, acc.Code)
			fmt.Printf("   Bank Name: %s\n", acc.BankName)
			fmt.Printf("   Account No: %s\n", acc.AccountNo)
			fmt.Printf("   ✓ Account Holder: %s\n", acc.AccountHolderName)
			fmt.Printf("   ✓ Branch: %s\n", acc.Branch)
			fmt.Printf("   Balance: %.2f\n", acc.Balance)
			fmt.Printf("   Active: %v\n", acc.IsActive)
		}
	}

	if !hasData {
		fmt.Println("⚠️  No accounts found with Account Holder Name or Branch data.")
		fmt.Println("   Please update accounts via frontend to add this information.")
	}
	fmt.Println()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
