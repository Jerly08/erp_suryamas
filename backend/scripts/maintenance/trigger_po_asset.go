package main

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/services"
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Initialize DB
	dsn := "host=localhost user=postgres password=postgres dbname=sistem_erp port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	purchaseRepo := repositories.NewPurchaseRepository(db)
	productRepo := repositories.NewProductRepository(db)
	contactRepo := repositories.NewContactRepository(db)
	accountRepo := repositories.NewAccountRepository(db)
	journalRepo := repositories.NewJournalEntryRepository(db)
	
	approvalSvc := services.NewApprovalService(db)
	pdfService := services.NewPDFService(db)
	unifiedJournalService := services.NewUnifiedJournalService(db)
	coaService := services.NewCOAService(db)
	settingsService := services.NewSettingsService(db)
	purchaseJournalServiceSSOT := services.NewPurchaseJournalServiceSSOT(db, coaService, settingsService)

	purchaseSvc := services.NewPurchaseService(
		db, 
		purchaseRepo, 
		productRepo, 
		contactRepo, 
		accountRepo, 
		approvalSvc, 
		nil, 
		journalRepo, 
		pdfService, 
		unifiedJournalService, 
		coaService, 
		purchaseJournalServiceSSOT,
	)

	// purchase code PO/2026/03/0007
	var purchase models.Purchase
	if err := db.Preload("PurchaseItems").Preload("Vendor").Where("code = ?", "PO/2026/03/0007").First(&purchase).Error; err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Triggering OnPurchaseApproved for Purchase %d (%s)...\n", purchase.ID, purchase.Code)
	
	err = purchaseSvc.OnPurchaseApproved(purchase.ID)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println("Done.")
	}
}
