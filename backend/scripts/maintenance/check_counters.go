package main

import (
	"fmt"
	"log"
	"time"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"
)

func main() {
	// Initialize database
	db := database.ConnectDB()

	fmt.Println("📊 INVOICE COUNTERS STATUS")
	fmt.Println("================================================")
	fmt.Println()

	// Get all invoice types
	var invoiceTypes []models.InvoiceType
	err := db.Where("is_active = ?", true).Order("id ASC").Find(&invoiceTypes).Error
	if err != nil {
		log.Fatalf("Failed to get invoice types: %v", err)
	}

	if len(invoiceTypes) == 0 {
		fmt.Println("❌ No active invoice types found")
		return
	}

	currentYear := time.Now().Year()
	
	for _, invoiceType := range invoiceTypes {
		fmt.Printf("📋 %s (%s) - ID: %d\n", invoiceType.Name, invoiceType.Code, invoiceType.ID)
		
		// Get counters for this invoice type
		var counters []models.InvoiceCounter
		err = db.Where("invoice_type_id = ?", invoiceType.ID).
			Order("year DESC").
			Find(&counters).Error
		if err != nil {
			fmt.Printf("   ❌ Error getting counters: %v\n", err)
			continue
		}

		if len(counters) == 0 {
			fmt.Printf("   📊 No counters found\n")
			// Preview what the next number would be
			previewNext(invoiceType, currentYear, 0)
		} else {
			for _, counter := range counters {
				status := ""
				if counter.Year == currentYear {
					status = " ← Current Year"
				}
				fmt.Printf("   📊 Year %d: Counter = %d%s\n", counter.Year, counter.Counter, status)
				
				// Show preview for current year
				if counter.Year == currentYear {
					previewNext(invoiceType, counter.Year, counter.Counter)
				}
			}
		}
		fmt.Println()
	}

	fmt.Println("💡 To reset a counter, use:")
	fmt.Println("   go run reset_counter.go <invoice_type_id> <year> <new_counter_value>")
	fmt.Println()
	fmt.Println("Example: go run reset_counter.go 1 2025 100")
}

func previewNext(invoiceType models.InvoiceType, year int, currentCounter int) {
	nextCounter := currentCounter + 1
	romanMonth := models.GetRomanMonth(int(time.Now().Month()))
	invoiceNumber := fmt.Sprintf("%04d/%s/%s-%04d", nextCounter, invoiceType.Code, romanMonth, year)
	fmt.Printf("   🔍 Next invoice: %s\n", invoiceNumber)
}