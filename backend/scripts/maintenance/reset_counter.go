package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"
	"gorm.io/gorm"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: go run reset_counter.go <invoice_type_id> <year> <new_counter_value>")
		fmt.Println("Example: go run reset_counter.go 1 2025 100")
		fmt.Println("")
		fmt.Println("Available Invoice Types:")
		showAvailableInvoiceTypes()
		os.Exit(1)
	}

	invoiceTypeID, err := strconv.ParseUint(os.Args[1], 10, 32)
	if err != nil {
		log.Fatalf("Invalid invoice_type_id: %v", err)
	}

	year, err := strconv.Atoi(os.Args[2])
	if err != nil {
		log.Fatalf("Invalid year: %v", err)
	}

	newCounterValue, err := strconv.Atoi(os.Args[3])
	if err != nil {
		log.Fatalf("Invalid counter value: %v", err)
	}

	// Initialize database
	db := database.ConnectDB()

	// Reset counter
	err = resetInvoiceCounter(db, uint(invoiceTypeID), year, newCounterValue)
	if err != nil {
		log.Fatalf("Failed to reset counter: %v", err)
	}

	fmt.Printf("✅ Successfully reset counter for Invoice Type ID %d, Year %d to %d\n", invoiceTypeID, year, newCounterValue)
	
	// Show preview of next invoice number
	showPreview(db, uint(invoiceTypeID), year, newCounterValue)
}

func showAvailableInvoiceTypes() {
	db := database.ConnectDB()

	var invoiceTypes []models.InvoiceType
	err := db.Where("is_active = ?", true).Find(&invoiceTypes).Error
	if err != nil {
		log.Printf("Cannot fetch invoice types: %v", err)
		return
	}

	for _, it := range invoiceTypes {
		fmt.Printf("  ID: %d, Name: %s, Code: %s\n", it.ID, it.Name, it.Code)
	}
}

func resetInvoiceCounter(db *gorm.DB, invoiceTypeID uint, year int, newValue int) error {
	// Check if invoice type exists and is active
	var invoiceType models.InvoiceType
	err := db.Where("id = ? AND is_active = ?", invoiceTypeID, true).First(&invoiceType).Error
	if err != nil {
		return fmt.Errorf("invoice type not found or inactive: %v", err)
	}

	fmt.Printf("📋 Invoice Type: %s (%s)\n", invoiceType.Name, invoiceType.Code)

	// Get or create counter
	var counter models.InvoiceCounter
	err = db.Where("invoice_type_id = ? AND year = ?", invoiceTypeID, year).First(&counter).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create new counter
			counter = models.InvoiceCounter{
				InvoiceTypeID: invoiceTypeID,
				Year:          year,
				Counter:       newValue,
			}
			fmt.Printf("📊 Creating new counter for year %d with value %d\n", year, newValue)
			return db.Create(&counter).Error
		}
		return fmt.Errorf("failed to get counter: %v", err)
	}

	// Update existing counter
	oldValue := counter.Counter
	counter.Counter = newValue
	err = db.Save(&counter).Error
	if err != nil {
		return fmt.Errorf("failed to update counter: %v", err)
	}

	fmt.Printf("📊 Updated counter for year %d: %d → %d\n", year, oldValue, newValue)
	return nil
}

func showPreview(db *gorm.DB, invoiceTypeID uint, year int, currentCounter int) {
	var invoiceType models.InvoiceType
	err := db.Where("id = ?", invoiceTypeID).First(&invoiceType).Error
	if err != nil {
		return
	}

	// Generate preview for next invoice number
	nextCounter := currentCounter + 1
	romanMonth := models.GetRomanMonth(10) // Current month (October)
	invoiceNumber := fmt.Sprintf("%04d/%s/%s-%04d", nextCounter, invoiceType.Code, romanMonth, year)

	fmt.Printf("🔍 Next invoice number will be: %s\n", invoiceNumber)
}