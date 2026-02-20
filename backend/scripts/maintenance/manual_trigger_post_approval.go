package main

import (
	"fmt"
	"log"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/services"
	"app-sistem-akuntansi/repositories"
)

func main() {
	// Load configuration  
	_ = config.LoadConfig()
	
	// Connect to database
	db := database.ConnectDB()
	
	fmt.Println("🔧 Manual trigger for post-approval processing...")
	
	// Initialize required repositories and services
	purchaseRepo := repositories.NewPurchaseRepository(db)
	productRepo := repositories.NewProductRepository(db)
	contactRepo := repositories.NewContactRepository(db) 
	accountRepo := repositories.NewAccountRepository(db)
	journalRepo := repositories.NewJournalEntryRepository(db)
	
	// Initialize services
	approvalService := services.NewApprovalService(db)
	unifiedJournalService := services.NewUnifiedJournalService(db)
	coaService := services.NewCOAService(db)
	pdfService := services.NewPDFService(db)
	
	// Initialize purchase service with all dependencies
	purchaseService := services.NewPurchaseService(
		db,
		purchaseRepo,
		productRepo,
		contactRepo,
		accountRepo,
		approvalService,
		nil, // journal service - can be nil
		journalRepo,
		pdfService,
		unifiedJournalService,
		coaService,
	)
	
	// Get the approved purchases that need post-approval processing
	approvedPurchaseIDs := []uint{2, 3}
	
	fmt.Printf("🎯 Triggering post-approval processing for purchases: %v\n", approvedPurchaseIDs)
	
	for _, purchaseID := range approvedPurchaseIDs {
		fmt.Printf("\n📦 Processing Purchase ID: %d\n", purchaseID)
		
		// Get purchase details first
		purchase, err := purchaseRepo.FindByID(purchaseID)
		if err != nil {
			log.Printf("❌ Failed to get purchase %d: %v", purchaseID, err)
			continue
		}
		
		fmt.Printf("  Purchase: %s, Method: %s, Amount: %.2f, Status: %s\n", 
			purchase.Code, purchase.PaymentMethod, purchase.TotalAmount, purchase.Status)
		
		if purchase.Status != "APPROVED" {
			fmt.Printf("  ⚠️ Purchase %d is not APPROVED (status: %s), skipping...\n", purchaseID, purchase.Status)
			continue
		}
		
		// Check if post-approval processing already done
		var existingTxCount int64
		db.Raw("SELECT COUNT(*) FROM cash_bank_transactions WHERE reference_type = 'PURCHASE' AND reference_id = ?", purchaseID).Scan(&existingTxCount)
		
		if existingTxCount > 0 {
			fmt.Printf("  ✅ Post-approval processing already done for Purchase %d (found %d cash bank transactions)\n", purchaseID, existingTxCount)
			continue
		}
		
		// Manually trigger OnPurchaseApproved callback
		fmt.Printf("  🚀 Triggering OnPurchaseApproved callback for Purchase %d...\n", purchaseID)
		
		err = purchaseService.OnPurchaseApproved(purchaseID)
		if err != nil {
			fmt.Printf("  ❌ OnPurchaseApproved failed for Purchase %d: %v\n", purchaseID, err)
		} else {
			fmt.Printf("  ✅ OnPurchaseApproved completed successfully for Purchase %d\n", purchaseID)
		}
		
		// Verify the results
		var newTxCount int64
		db.Raw("SELECT COUNT(*) FROM cash_bank_transactions WHERE reference_type = 'PURCHASE' AND reference_id = ?", purchaseID).Scan(&newTxCount)
		
		if newTxCount > 0 {
			fmt.Printf("  ✅ Created %d cash_bank_transactions for Purchase %d\n", newTxCount, purchaseID)
		} else {
			fmt.Printf("  ❌ No cash_bank_transactions created for Purchase %d\n", purchaseID)
		}
	}
	
	// Final verification - check total impact
	fmt.Println("\n📊 FINAL VERIFICATION:")
	
	var totalTxCount int64
	db.Raw("SELECT COUNT(*) FROM cash_bank_transactions WHERE reference_type = 'PURCHASE'").Scan(&totalTxCount)
	fmt.Printf("  Total PURCHASE cash_bank_transactions: %d\n", totalTxCount)
	
	// Check bank account 7 balance after processing
	type BankBalance struct {
		ID uint `json:"id"`
		Name string `json:"name"`
		Balance float64 `json:"balance"`
		UpdatedAt string `json:"updated_at"`
	}
	
	var bankBalance BankBalance
	db.Raw(`
		SELECT id, name, balance, updated_at
		FROM cash_banks 
		WHERE id = 7
	`).Scan(&bankBalance)
	
	fmt.Printf("  Bank Account 7 (%s): Balance %.2f, Updated: %s\n",
		bankBalance.Name, bankBalance.Balance, bankBalance.UpdatedAt)
	
	fmt.Println("\n🎉 Manual post-approval processing completed!")
	fmt.Println("💡 Check the cash & bank balance now to see if the issue is resolved.")
}