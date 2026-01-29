package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"
)

func main() {
	fmt.Println("🔄 Starting Journal Synchronization to unified_journal_ledger...")
	fmt.Println("=================================================================")

	// Connect to database
	db, err := connectDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Get statistics before sync
	var salesCount, purchaseCount int64
	db.Model(&models.Sale{}).Where("status IN (?, ?)", "INVOICED", "PAID").Count(&salesCount)
	db.Model(&models.Purchase{}).Where("status IN (?, ?, ?)", "APPROVED", "COMPLETED", "PAID").Count(&purchaseCount)

	var existingJournalCount int64
	db.Model(&models.SSOTJournalEntry{}).Count(&existingJournalCount)

	fmt.Printf("\n📊 Current Status:\n")
	fmt.Printf("   - Sales Transactions (INVOICED/PAID): %d\n", salesCount)
	fmt.Printf("   - Purchase Transactions (APPROVED/COMPLETED/PAID): %d\n", purchaseCount)
	fmt.Printf("   - Existing Journal Entries in unified_journal_ledger: %d\n\n", existingJournalCount)

	// Confirm before proceeding
	/*
	fmt.Print("⚠️  Do you want to regenerate journal entries? This will create missing entries. (yes/no): ")
	var confirm string
	fmt.Scanln(&confirm)
	if confirm != "yes" {
		fmt.Println("❌ Operation cancelled.")
		return
	}
	*/

	// Initialize SSOT journal services
	coaService := services.NewCOAService(db)
	salesJournalService := services.NewSalesJournalServiceSSOT(db, coaService)
	purchaseJournalService := services.NewPurchaseJournalServiceSSOT(db, coaService)

	// Process Sales Transactions
	fmt.Println("\n📝 Processing Sales Transactions...")
	fmt.Println("-----------------------------------")
	
	var sales []models.Sale
	db.Preload("Customer").
		Preload("SaleItems.Product").
		Where("status IN (?, ?)", "INVOICED", "PAID").
		Order("date ASC, id ASC").
		Find(&sales)

	salesProcessed := 0
	salesSkipped := 0
	salesErrors := 0

	for _, sale := range sales {
		// Check if journal already exists
		var existingCount int64
		db.Model(&models.SSOTJournalEntry{}).
			Where("source_type = ? AND source_id = ?", "SALE", sale.ID).
			Count(&existingCount)

		if existingCount > 0 {
			salesSkipped++
			fmt.Printf("   ⏭️  Sale #%d (%s) - Journal already exists, skipping\n", sale.ID, sale.InvoiceNumber)
			continue
		}

		// Create journal entry
		err := salesJournalService.CreateSalesJournal(&sale, nil)
		if err != nil {
			salesErrors++
			fmt.Printf("   ❌ Sale #%d (%s) - Error: %v\n", sale.ID, sale.InvoiceNumber, err)
		} else {
			salesProcessed++
			fmt.Printf("   ✅ Sale #%d (%s) - Journal created successfully\n", sale.ID, sale.InvoiceNumber)
		}
	}

	// Process Purchase Transactions
	fmt.Println("\n🛒 Processing Purchase Transactions...")
	fmt.Println("--------------------------------------")
	
	var purchases []models.Purchase
	db.Preload("Vendor").
		Preload("PurchaseItems.Product").
		Where("status IN (?, ?, ?)", "APPROVED", "COMPLETED", "PAID").
		Order("date ASC, id ASC").
		Find(&purchases)

	purchasesProcessed := 0
	purchasesSkipped := 0
	purchasesErrors := 0

	for _, purchase := range purchases {
		// Check if journal already exists
		var existingCount int64
		db.Model(&models.SSOTJournalEntry{}).
			Where("source_type = ? AND source_id = ?", "PURCHASE", purchase.ID).
			Count(&existingCount)

		if existingCount > 0 {
			purchasesSkipped++
			fmt.Printf("   ⏭️  Purchase #%d (%s) - Journal already exists, skipping\n", purchase.ID, purchase.Code)
			continue
		}

		// Create journal entry
		err := purchaseJournalService.CreatePurchaseJournal(&purchase, nil)
		if err != nil {
			purchasesErrors++
			fmt.Printf("   ❌ Purchase #%d (%s) - Error: %v\n", purchase.ID, purchase.Code, err)
		} else {
			purchasesProcessed++
			fmt.Printf("   ✅ Purchase #%d (%s) - Journal created successfully\n", purchase.ID, purchase.Code)
		}
	}

	// Get statistics after sync
	var finalJournalCount int64
	db.Model(&models.SSOTJournalEntry{}).Count(&finalJournalCount)

	// Print summary
	fmt.Println("\n" + "=================================================================")
	fmt.Println("📊 Synchronization Summary")
	fmt.Println("=================================================================")
	fmt.Printf("\n✅ SALES TRANSACTIONS:\n")
	fmt.Printf("   - Total found: %d\n", len(sales))
	fmt.Printf("   - Processed: %d\n", salesProcessed)
	fmt.Printf("   - Skipped (already exists): %d\n", salesSkipped)
	fmt.Printf("   - Errors: %d\n", salesErrors)

	fmt.Printf("\n✅ PURCHASE TRANSACTIONS:\n")
	fmt.Printf("   - Total found: %d\n", len(purchases))
	fmt.Printf("   - Processed: %d\n", purchasesProcessed)
	fmt.Printf("   - Skipped (already exists): %d\n", purchasesSkipped)
	fmt.Printf("   - Errors: %d\n", purchasesErrors)

	fmt.Printf("\n📈 JOURNAL ENTRIES:\n")
	fmt.Printf("   - Before: %d entries\n", existingJournalCount)
	fmt.Printf("   - After: %d entries\n", finalJournalCount)
	fmt.Printf("   - New entries created: %d\n", finalJournalCount-existingJournalCount)

	if salesErrors > 0 || purchasesErrors > 0 {
		fmt.Println("\n⚠️  Warning: Some transactions failed to sync. Please check the errors above.")
	} else {
		fmt.Println("\n🎉 All transactions synchronized successfully!")
	}

	fmt.Println("\n💡 Next Steps:")
	fmt.Println("   1. Check General Ledger report in the frontend")
	fmt.Println("   2. Verify that transactions now appear")
	fmt.Println("   3. Check Trial Balance to ensure debits = credits")
	fmt.Println("\n=================================================================")
}

func connectDB() (*gorm.DB, error) {
	// Get database connection from environment or use default
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=sistem_akuntansi port=5432 sslmode=disable TimeZone=Asia/Jakarta"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		NowFunc: func() time.Time {
			return time.Now().In(time.FixedZone("WIB", 7*60*60))
		},
	})

	if err != nil {
		return nil, err
	}

	return db, nil
}

