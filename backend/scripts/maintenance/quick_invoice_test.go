package main

import (
	"fmt"
	"os"
	"time"
	
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"
)

func main() {
	// Simple database connection
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		// Default connection - adjust as needed
		dsn = "host=localhost user=postgres password=password dbname=accounting_db port=5432 sslmode=disable TimeZone=Asia/Jakarta"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Printf("❌ Database connection failed: %v\n", err)
		fmt.Println("Please check your database connection settings:")
		fmt.Println("- Set DATABASE_URL environment variable, or")
		fmt.Println("- Update the default DSN in this script")
		return
	}

	fmt.Println("✅ Database connected!")
	
	// Check invoice types
	var invoiceTypes []models.InvoiceType
	db.Where("is_active = ?", true).Find(&invoiceTypes)
	
	fmt.Printf("\n📋 Found %d active invoice types:\n", len(invoiceTypes))
	for _, invType := range invoiceTypes {
		fmt.Printf("  - ID: %d, Code: %s, Name: %s\n", invType.ID, invType.Code, invType.Name)
	}
	
	if len(invoiceTypes) == 0 {
		fmt.Println("❌ No active invoice types found!")
		fmt.Println("You need to run the migration: 037_add_invoice_types_system.sql")
		return
	}
	
	// Test invoice number generation
	invoiceService := services.NewInvoiceNumberService(db)
	
	fmt.Println("\n🧪 Testing invoice number generation:")
	
	// Test dates
	testDate := time.Date(2025, 9, 3, 12, 0, 0, 0, time.UTC) // September 2025
	fmt.Printf("Test date: %s (Month: %s)\n", testDate.Format("2006-01-02"), models.GetRomanMonth(9))
	
	// Test each invoice type
	for _, invType := range invoiceTypes {
		preview, err := invoiceService.PreviewInvoiceNumber(invType.ID, testDate)
		if err != nil {
			fmt.Printf("  ❌ %s (%d): Error - %v\n", invType.Code, invType.ID, err)
			continue
		}
		
		fmt.Printf("  ✅ %s (%d): %s\n", invType.Code, invType.ID, preview.InvoiceNumber)
		
		// Check if format looks correct
		expectedFormat := fmt.Sprintf("%04d/%s/IX-2025", preview.Counter, invType.Code)
		if preview.InvoiceNumber == expectedFormat {
			fmt.Printf("     👍 Format matches expected pattern!\n")
		} else {
			fmt.Printf("     ⚠️  Expected: %s\n", expectedFormat)
		}
	}
	
	fmt.Println("\n" + "="*50)
	fmt.Println("RECOMMENDATIONS:")
	fmt.Println("="*50)
	
	if len(invoiceTypes) > 0 {
		fmt.Println("1. ✅ Invoice types are set up correctly")
		fmt.Println("2. 🔍 Check if frontend is sending invoice_type_id properly")
		fmt.Println("3. 📋 When creating sales, make sure to select an invoice type")
		fmt.Printf("4. 🧪 Test by creating a sale with invoice_type_id = %d\n", invoiceTypes[0].ID)
	}
	
	fmt.Println("\n💡 DEBUGGING TIPS:")
	fmt.Println("- Check backend logs for: '📝 Creating sale with invoice_type_id:'")
	fmt.Println("- Look for: '🔧 Generating invoice number for sale'")
	fmt.Println("- If you see: '⚠️ No invoice type specified', the frontend isn't sending it")
	
	fmt.Println("\nTest completed! 🎉")
}