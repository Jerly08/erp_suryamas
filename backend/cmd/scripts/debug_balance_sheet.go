package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Account struct to map database columns
type Account struct {
	ID       uint `gorm:"primaryKey"`
	Code     string
	Name     string
	Type     string
	Level    int
	IsHeader bool
	Balance  float64
	ParentID *uint
}

func main() {
	// Load .env file (try multiple paths)
	if err := godotenv.Load(".env"); err != nil {
		if err := godotenv.Load("../../.env"); err != nil {
			log.Println("Warning: Error loading .env file, checking environment variables...")
		}
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		// Fallback to hardcoded DSN with CORRECT database name
		dsn = "host=localhost user=postgres password=postgres dbname=sistem_akuntansi port=5432 sslmode=disable TimeZone=Asia/Jakarta"
		fmt.Println("Using fallback DSN:", dsn)
	} else {
		fmt.Println("Using .env DSN:", dsn)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Create output file
	outFile, err := os.Create("debug_results.txt")
	if err != nil {
		log.Fatalf("Failed to create output file: %v", err)
	}
	defer outFile.Close()

	fmt.Fprintln(outFile, "=== DEBUG BALANCE SHEET SCRIPT ===")

	var accounts []Account
	if err := db.Where("deleted_at IS NULL").Order("code asc").Find(&accounts).Error; err != nil {
		log.Fatalf("Failed to fetch accounts: %v", err)
	}

	var totalAsset, totalLiability, totalEquity float64
	var totalRevenue, totalExpense float64

	fmt.Fprintf(outFile, "%-10s %-35s %-20s %-10s %-20s\n", "CODE", "NAME", "TYPE", "HEADER?", "BALANCE")
	fmt.Fprintln(outFile, strings.Repeat("-", 100))

	for _, acc := range accounts {
		// Clean type
		accType := strings.TrimSpace(strings.ToUpper(acc.Type))

		fmt.Fprintf(outFile, "%-10s %-35s %-20s %-10v %15.2f\n", acc.Code, acc.Name, acc.Type, acc.IsHeader, acc.Balance)

		// Skip counting headers for summation usually?
		// If headers have balance, that's a problem in itself, but we should count them if they hold value.
		// However, standard logic usually sums only non-headers.
		// Let's mimic the APP LOGIC (exclude headers).

		if acc.IsHeader {
			// Check if header has balance
			if acc.Balance != 0 {
				fmt.Fprintf(outFile, "  >>> WARNING: Header account has non-zero balance! Included/Excluded? App excludes entries.\n")
			}
			continue
		}

		switch {
		case accType == "ASSET" || accType == "ASET" || strings.HasPrefix(accType, "ASSET"):
			totalAsset += acc.Balance
		case accType == "LIABILITY" || accType == "KEWAJIBAN" || strings.HasPrefix(accType, "LIABILITY"):
			totalLiability += acc.Balance
		case accType == "EQUITY" || accType == "MODAL" || accType == "EKUITAS" || strings.HasPrefix(accType, "EQUITY"):
			totalEquity += acc.Balance
		case accType == "REVENUE" || accType == "PENDAPATAN" || strings.HasPrefix(accType, "PENDAPATAN") || strings.Contains(accType, "INCOME"):
			totalRevenue += acc.Balance
		case accType == "EXPENSE" || accType == "BEBAN" || strings.HasPrefix(accType, "BEBAN") || strings.Contains(accType, "EXPENSE"):
			totalExpense += acc.Balance
		}
	}

	fmt.Fprintln(outFile, strings.Repeat("-", 100))

	netIncome := totalRevenue - totalExpense

	fmt.Fprintf(outFile, "\n=== SUMMARY ===\n")
	fmt.Fprintf(outFile, "Total Assets:      %15.2f\n", totalAsset)
	fmt.Fprintf(outFile, "Total Liabilities: %15.2f\n", totalLiability)
	fmt.Fprintf(outFile, "Total Equity (Obs):%15.2f\n", totalEquity)
	fmt.Fprintf(outFile, "Total Revenue:     %15.2f\n", totalRevenue)
	fmt.Fprintf(outFile, "Total Expense:     %15.2f\n", totalExpense)
	fmt.Fprintf(outFile, "Net Income (Calc): %15.2f (Rev - Exp)\n", netIncome)

	impliedEquity := totalEquity + netIncome
	fmt.Fprintf(outFile, "Total Equity + NI: %15.2f\n", impliedEquity)

	fmt.Fprintf(outFile, "\n=== BALANCE CHECK ===\n")
	fmt.Fprintf(outFile, "Left (Assets):     %15.2f\n", totalAsset)
	fmt.Fprintf(outFile, "Right (Liab+Eq):   %15.2f (Liab + Equity + NetIncome)\n", totalLiability+impliedEquity)
	fmt.Fprintf(outFile, "Difference:        %15.2f\n", totalAsset-(totalLiability+impliedEquity))

	if totalAsset-(totalLiability+impliedEquity) == 0 {
		fmt.Fprintln(outFile, "STATUS: BALANCED ✅")
	} else {
		fmt.Fprintln(outFile, "STATUS: UNBALANCED ❌")
	}
	fmt.Println("Debug script completed. Check debug_results.txt")
}
