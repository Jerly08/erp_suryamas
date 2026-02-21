package main

import (
	"fmt"
	"log"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/services"

	"gorm.io/gorm"
)

func main() {
	config.LoadConfig()
	db := database.ConnectDB()

	err := db.Transaction(func(tx *gorm.DB) error {
		svc := services.NewSSOTBalanceSheetService(tx)
		data, err := svc.GenerateSSOTBalanceSheet("2026-02-21")
		if err != nil {
			return err
		}

		fmt.Printf("LIABILITIES:\n")
		for _, item := range data.Liabilities.CurrentLiabilities.Items {
			fmt.Printf(" - %s: %.2f\n", item.AccountCode, item.Amount)
		}
		for _, item := range data.Liabilities.NonCurrentLiabilities.Items {
			fmt.Printf(" - %s: %.2f\n", item.AccountCode, item.Amount)
		}
		fmt.Printf("TOTAL LIABILITIES: %.2f\n\n", data.Liabilities.TotalLiabilities)

		fmt.Printf("EQUITIES:\n")
		for _, item := range data.Equity.Items {
			fmt.Printf(" - %s: %.2f\n", item.AccountCode, item.Amount)
		}
		fmt.Printf("TOTAL EQUITIES: %.2f\n\n", data.Equity.TotalEquity)

		fmt.Printf("TOTAL L+E: %.2f\n", data.TotalLiabilitiesAndEquity)

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}
}
