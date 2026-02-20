package main

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/services"
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestInvoicedOnlyLogic tests the complete DRAFT->CONFIRMED->INVOICED workflow
// ensuring journal entries and COA updates ONLY happen on INVOICED status
func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("🚀 Starting Comprehensive INVOICED-Only Logic Test")

	// Initialize database - using the actual connection
	db, err := initTestDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Initialize repositories
	salesRepo := repositories.NewSalesRepository(db)
	productRepo := repositories.NewProductRepository(db)
	contactRepo := repositories.NewContactRepository(db)
	accountRepo := repositories.NewAccountRepository(db)
	journalRepo := repositories.NewJournalEntryRepository(db)

	// Initialize services - use simpler approach
	pdfService := &MockPDFService{}         // Dummy implementation
	journalService := &MockJournalService{} // Dummy implementation
	salesService := services.NewSalesService(db, salesRepo, productRepo, contactRepo, accountRepo, journalService, pdfService)

	// Test user ID
	userID := uint(1)

	// Run comprehensive test
	if err := runComprehensiveTest(db, salesService, accountRepo, journalRepo, userID); err != nil {
		log.Fatalf("❌ Test failed: %v", err)
	}

	log.Println("✅ All tests passed! INVOICED-only logic works correctly")
}

func runComprehensiveTest(db *gorm.DB, salesService *services.SalesService, accountRepo repositories.AccountRepository, journalRepo repositories.JournalEntryRepository, userID uint) error {
	log.Println("📋 Running comprehensive test for INVOICED-only logic...")

	// Test data setup
	testCustomer, testProduct, err := setupTestData(db)
	if err != nil {
		return fmt.Errorf("failed to setup test data: %v", err)
	}
	defer cleanupTestData(db, testCustomer.ID, testProduct.ID)

	// Step 1: Create sale in DRAFT status
	log.Println("\n🔸 STEP 1: Creating sale in DRAFT status")
	saleRequest := models.SaleCreateRequest{
		CustomerID:        testCustomer.ID,
		Type:              models.SaleTypeInvoice,
		Date:              time.Now(),
		DueDate:           time.Now().Add(30 * 24 * time.Hour),
		PaymentTerms:      "NET_30",
		PaymentMethod:     "CREDIT",
		PaymentMethodType: "CREDIT",
		Items: []models.SaleItemRequest{
			{
				ProductID: testProduct.ID,
				Quantity:  2,
				UnitPrice: 100000,
				Taxable:   true,
			},
		},
	}

	createdSale, err := salesService.CreateSale(saleRequest, userID)
	if err != nil {
		return fmt.Errorf("failed to create sale: %v", err)
	}
	defer cleanupSale(db, createdSale.ID)

	log.Printf("✅ Sale created with ID %d, Status: %s", createdSale.ID, createdSale.Status)

	// Verify no journal entries exist for DRAFT
	journalCount, err := countJournalEntriesForSale(db, createdSale.ID)
	if err != nil {
		return fmt.Errorf("failed to count journal entries: %v", err)
	}
	if journalCount > 0 {
		return fmt.Errorf("❌ DRAFT sale should not have journal entries, found: %d", journalCount)
	}
	log.Printf("✅ DRAFT sale has no journal entries (count: %d)", journalCount)

	// Get initial account balances
	initialBalances, err := getAccountBalances(accountRepo)
	if err != nil {
		return fmt.Errorf("failed to get initial balances: %v", err)
	}

	// Step 2: Confirm sale (DRAFT -> CONFIRMED)
	log.Println("\n🔸 STEP 2: Confirming sale (DRAFT -> CONFIRMED)")
	err = salesService.ConfirmSale(createdSale.ID, userID)
	if err != nil {
		return fmt.Errorf("failed to confirm sale: %v", err)
	}

	// Verify sale status is CONFIRMED
	updatedSale, err := salesService.GetSaleByID(createdSale.ID)
	if err != nil {
		return fmt.Errorf("failed to get updated sale: %v", err)
	}
	if updatedSale.Status != models.SaleStatusConfirmed {
		return fmt.Errorf("❌ Expected status CONFIRMED, got: %s", updatedSale.Status)
	}
	log.Printf("✅ Sale status updated to: %s", updatedSale.Status)

	// Verify still no journal entries for CONFIRMED
	journalCount, err = countJournalEntriesForSale(db, createdSale.ID)
	if err != nil {
		return fmt.Errorf("failed to count journal entries: %v", err)
	}
	if journalCount > 0 {
		return fmt.Errorf("❌ CONFIRMED sale should not have journal entries, found: %d", journalCount)
	}
	log.Printf("✅ CONFIRMED sale has no journal entries (count: %d)", journalCount)

	// Verify account balances unchanged
	currentBalances, err := getAccountBalances(accountRepo)
	if err != nil {
		return fmt.Errorf("failed to get current balances: %v", err)
	}
	if !compareBalances(initialBalances, currentBalances) {
		return fmt.Errorf("❌ Account balances changed during CONFIRMED status - should remain unchanged")
	}
	log.Println("✅ Account balances unchanged during CONFIRMED status")

	// Step 3: Create invoice (CONFIRMED -> INVOICED)
	log.Println("\n🔸 STEP 3: Creating invoice (CONFIRMED -> INVOICED)")
	invoicedSale, err := salesService.CreateInvoiceFromSale(createdSale.ID, userID)
	if err != nil {
		return fmt.Errorf("failed to create invoice: %v", err)
	}

	if invoicedSale.Status != models.SaleStatusInvoiced && invoicedSale.Status != models.SaleStatusPaid {
		return fmt.Errorf("❌ Expected status INVOICED or PAID, got: %s", invoicedSale.Status)
	}
	log.Printf("✅ Sale status updated to: %s", invoicedSale.Status)
	log.Printf("✅ Invoice number generated: %s", invoicedSale.InvoiceNumber)

	// Verify journal entries created for INVOICED
	journalCount, err = countJournalEntriesForSale(db, createdSale.ID)
	if err != nil {
		return fmt.Errorf("failed to count journal entries: %v", err)
	}
	if journalCount == 0 {
		return fmt.Errorf("❌ INVOICED sale should have journal entries, found: %d", journalCount)
	}
	log.Printf("✅ INVOICED sale has journal entries (count: %d)", journalCount)

	// Verify account balances changed
	finalBalances, err := getAccountBalances(accountRepo)
	if err != nil {
		return fmt.Errorf("failed to get final balances: %v", err)
	}
	if compareBalances(initialBalances, finalBalances) {
		return fmt.Errorf("❌ Account balances should have changed after INVOICED status")
	}
	log.Println("✅ Account balances updated after INVOICED status")

	// Step 4: Test different payment methods
	log.Println("\n🔸 STEP 4: Testing different payment methods")

	// Test CASH payment method
	err = testCashPaymentMethod(db, salesService, testCustomer, testProduct, userID)
	if err != nil {
		return fmt.Errorf("cash payment method test failed: %v", err)
	}

	// Test BANK payment method
	err = testBankPaymentMethod(db, salesService, testCustomer, testProduct, userID)
	if err != nil {
		return fmt.Errorf("bank payment method test failed: %v", err)
	}

	log.Println("\n✅ All comprehensive tests passed!")
	return nil
}

func testCashPaymentMethod(db *gorm.DB, salesService *services.SalesService, customer *models.Contact, product *models.Product, userID uint) error {
	log.Println("🔸 Testing CASH payment method workflow...")

	// Create cash bank account for testing
	cashAccount, err := createTestCashAccount(db)
	if err != nil {
		return fmt.Errorf("failed to create test cash account: %v", err)
	}
	defer cleanupCashBank(db, cashAccount.ID)

	saleRequest := models.SaleCreateRequest{
		CustomerID:        customer.ID,
		Type:              models.SaleTypeInvoice,
		Date:              time.Now(),
		PaymentMethod:     "CASH",
		PaymentMethodType: "CASH",
		CashBankID:        &cashAccount.ID,
		Items: []models.SaleItemRequest{
			{
				ProductID: product.ID,
				Quantity:  1,
				UnitPrice: 50000,
				Taxable:   true,
			},
		},
	}

	// Create and confirm sale
	sale, err := salesService.CreateSale(saleRequest, userID)
	if err != nil {
		return fmt.Errorf("failed to create cash sale: %v", err)
	}
	defer cleanupSale(db, sale.ID)

	err = salesService.ConfirmSale(sale.ID, userID)
	if err != nil {
		return fmt.Errorf("failed to confirm cash sale: %v", err)
	}

	// Create invoice - should auto-create payment and set status to PAID
	invoicedSale, err := salesService.CreateInvoiceFromSale(sale.ID, userID)
	if err != nil {
		return fmt.Errorf("failed to create invoice for cash sale: %v", err)
	}

	if invoicedSale.Status != models.SaleStatusPaid {
		return fmt.Errorf("❌ Cash sale should automatically be PAID, got: %s", invoicedSale.Status)
	}

	log.Printf("✅ Cash sale automatically set to PAID status")
	return nil
}

func testBankPaymentMethod(db *gorm.DB, salesService *services.SalesService, customer *models.Contact, product *models.Product, userID uint) error {
	log.Println("🔸 Testing BANK payment method workflow...")

	// Create bank account for testing
	bankAccount, err := createTestBankAccount(db)
	if err != nil {
		return fmt.Errorf("failed to create test bank account: %v", err)
	}
	defer cleanupCashBank(db, bankAccount.ID)

	saleRequest := models.SaleCreateRequest{
		CustomerID:        customer.ID,
		Type:              models.SaleTypeInvoice,
		Date:              time.Now(),
		PaymentMethod:     "BANK",
		PaymentMethodType: "BANK",
		CashBankID:        &bankAccount.ID,
		Items: []models.SaleItemRequest{
			{
				ProductID: product.ID,
				Quantity:  1,
				UnitPrice: 75000,
				Taxable:   true,
			},
		},
	}

	// Create and confirm sale
	sale, err := salesService.CreateSale(saleRequest, userID)
	if err != nil {
		return fmt.Errorf("failed to create bank sale: %v", err)
	}
	defer cleanupSale(db, sale.ID)

	err = salesService.ConfirmSale(sale.ID, userID)
	if err != nil {
		return fmt.Errorf("failed to confirm bank sale: %v", err)
	}

	// Create invoice - should auto-create payment and set status to PAID
	invoicedSale, err := salesService.CreateInvoiceFromSale(sale.ID, userID)
	if err != nil {
		return fmt.Errorf("failed to create invoice for bank sale: %v", err)
	}

	if invoicedSale.Status != models.SaleStatusPaid {
		return fmt.Errorf("❌ Bank sale should automatically be PAID, got: %s", invoicedSale.Status)
	}

	log.Printf("✅ Bank sale automatically set to PAID status")
	return nil
}

// Helper functions

func setupTestData(db *gorm.DB) (*models.Contact, *models.Product, error) {
	// Create test customer
	customer := &models.Contact{
		Name:     "Test Customer for INVOICED Logic",
		Email:    "test.invoiced@example.com",
		Type:     "CUSTOMER",
		IsActive: true,
	}
	if err := db.Create(customer).Error; err != nil {
		return nil, nil, err
	}

	// Create test product
	product := &models.Product{
		Name:      "Test Product for INVOICED Logic",
		Code:      "TEST-INV-LOGIC",
		SellPrice: 100000,
		IsActive:  true,
	}
	if err := db.Create(product).Error; err != nil {
		return nil, nil, err
	}

	return customer, product, nil
}

func createTestCashAccount(db *gorm.DB) (*models.CashBank, error) {
	// First create GL account
	account := &models.Account{
		Code:        "1101-TEST-CASH",
		Name:        "Test Cash Account",
		Type:        "ASSET",
		Category:    "CURRENT_ASSET",
		IsActive:    true,
		Balance:     0,
		Description: "Test cash account for invoiced logic test",
	}
	if err := db.Create(account).Error; err != nil {
		return nil, err
	}

	// Create cash bank record
	cashBank := &models.CashBank{
		Name:      "Test Cash",
		Type:      "CASH",
		AccountID: account.ID,
		Balance:   0,
		IsActive:  true,
	}
	if err := db.Create(cashBank).Error; err != nil {
		return nil, err
	}

	return cashBank, nil
}

func createTestBankAccount(db *gorm.DB) (*models.CashBank, error) {
	// First create GL account
	account := &models.Account{
		Code:        "1102-TEST-BANK",
		Name:        "Test Bank Account",
		Type:        "ASSET",
		Category:    "CURRENT_ASSET",
		IsActive:    true,
		Balance:     0,
		Description: "Test bank account for invoiced logic test",
	}
	if err := db.Create(account).Error; err != nil {
		return nil, err
	}

	// Create cash bank record
	cashBank := &models.CashBank{
		Name:      "Test Bank",
		Type:      "BANK",
		AccountID: account.ID,
		Balance:   0,
		IsActive:  true,
		BankName:  "Test Bank",
	}
	if err := db.Create(cashBank).Error; err != nil {
		return nil, err
	}

	return cashBank, nil
}

func countJournalEntriesForSale(db *gorm.DB, saleID uint) (int64, error) {
	var count int64
	err := db.Model(&models.JournalEntry{}).
		Where("reference_type = ? AND reference_id = ?", models.JournalRefSale, saleID).
		Count(&count).Error
	return count, err
}

func getAccountBalances(accountRepo repositories.AccountRepository) (map[string]float64, error) {
	accounts, err := accountRepo.FindAll(context.Background())
	if err != nil {
		return nil, err
	}

	balances := make(map[string]float64)
	for _, account := range accounts {
		balances[account.Code] = account.Balance
	}
	return balances, nil
}

func compareBalances(before, after map[string]float64) bool {
	if len(before) != len(after) {
		return false
	}

	for code, beforeBalance := range before {
		afterBalance, exists := after[code]
		if !exists || beforeBalance != afterBalance {
			return false
		}
	}
	return true
}

// Cleanup functions

func cleanupTestData(db *gorm.DB, customerID, productID uint) {
	db.Delete(&models.Contact{}, customerID)
	db.Delete(&models.Product{}, productID)
}

func cleanupSale(db *gorm.DB, saleID uint) {
	// Delete related records first
	db.Where("sale_id = ?", saleID).Delete(&models.SaleItem{})
	db.Where("sale_id = ?", saleID).Delete(&models.SalePayment{})
	db.Where("reference_type = ? AND reference_id = ?", models.JournalRefSale, saleID).Delete(&models.JournalEntry{})

	// Delete sale
	db.Delete(&models.Sale{}, saleID)
}

func cleanupCashBank(db *gorm.DB, cashBankID uint) {
	var cashBank models.CashBank
	if err := db.First(&cashBank, cashBankID).Error; err == nil {
		db.Delete(&models.Account{}, cashBank.AccountID)
	}
	db.Delete(&models.CashBank{}, cashBankID)
}

// Mock PDF Service for testing
type MockPDFService struct{}

func (m *MockPDFService) GenerateInvoicePDF(sale *models.Sale) ([]byte, error) {
	return []byte("mock pdf content"), nil
}

func (m *MockPDFService) GenerateInvoicePDFWithType(sale *models.Sale, documentType string) ([]byte, error) {
	return []byte("mock pdf content"), nil
}

func (m *MockPDFService) GenerateSalesReportPDF(sales []models.Sale, startDate, endDate string) ([]byte, error) {
	return []byte("mock pdf content"), nil
}

func (m *MockPDFService) GeneratePaymentReportPDF(payments []models.Payment, startDate, endDate string) ([]byte, error) {
	return []byte("mock pdf content"), nil
}

func (m *MockPDFService) GeneratePaymentDetailPDF(payment *models.Payment) ([]byte, error) {
	return []byte("mock pdf content"), nil
}

func (m *MockPDFService) GenerateReceiptPDF(receipt *models.PurchaseReceipt) ([]byte, error) {
	return []byte("mock pdf content"), nil
}

func (m *MockPDFService) GenerateAllReceiptsPDF(purchase *models.Purchase, receipts []models.PurchaseReceipt) ([]byte, error) {
	return []byte("mock pdf content"), nil
}

func (m *MockPDFService) GenerateGeneralLedgerPDF(ledgerData interface{}, accountInfo string, startDate, endDate string) ([]byte, error) {
	return []byte("mock pdf content"), nil
}

func (m *MockPDFService) GenerateProfitLossPDF(profitLossData interface{}, startDate, endDate string) ([]byte, error) {
	return []byte("mock pdf content"), nil
}

func (m *MockPDFService) GenerateBalanceSheetPDF(balanceSheetData interface{}, asOfDate string) ([]byte, error) {
	return []byte("mock pdf content"), nil
}

func (m *MockPDFService) GenerateTrialBalancePDF(trialBalanceData interface{}, asOfDate string) ([]byte, error) {
	return []byte("mock pdf content"), nil
}

func (m *MockPDFService) GenerateJournalAnalysisPDF(journalData interface{}, startDate, endDate string) ([]byte, error) {
	return []byte("mock pdf content"), nil
}

func (m *MockPDFService) GenerateSSOTProfitLossPDF(ssotData interface{}, language string) ([]byte, error) {
	return []byte("mock pdf content"), nil
}

// Mock Journal Service for testing
type MockJournalService struct{}

func (m *MockJournalService) CreateSaleJournalEntries(sale *models.Sale, userID uint) error {
	return nil // Mock implementation - actual logic is in DoubleEntryService
}

func (m *MockJournalService) CreatePaymentJournalEntries(payment *models.SalePayment, userID uint) error {
	return nil // Mock implementation - actual logic is in DoubleEntryService
}

func (m *MockJournalService) CreateSaleReversalJournalEntries(sale *models.Sale, userID uint, reason string) error {
	return nil // Mock implementation - actual logic is in DoubleEntryService
}

// Simple database initialization for testing
func initTestDB() (*gorm.DB, error) {
	// Read database configuration from environment or use default test values
	dsn := "host=localhost user=postgres password=your_password dbname=accounting_test port=5432 sslmode=disable TimeZone=Asia/Jakarta"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return db, nil
}
