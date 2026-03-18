//go:build legacytests
// +build legacytests

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/controllers"
	"app-sistem-akuntansi/middleware"
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/services"
)

type PaymentSalesIntegrationTestSuite struct {
	suite.Suite
	db                     *gorm.DB
	router                 *gin.Engine
	
	// Repositories
	contactRepo            repositories.ContactRepository
	cashBankRepo           *repositories.CashBankRepository
	salesRepo              *repositories.SalesRepository
	purchaseRepo           *repositories.PurchaseRepository
	paymentRepo            *repositories.PaymentRepository
	
	// Services
	paymentService         *services.PaymentService
	enhancedPaymentService *services.EnhancedPaymentService
	eventService           *services.SalesUpdateEventService
	
	// Controllers
	enhancedPaymentController *controllers.EnhancedPaymentController
	
	// Test data
	testCustomer           models.Contact
	testVendor            models.Contact
	testCashAccount       models.CashBank
	testBankAccount       models.CashBank
	testInvoice           models.Sale
	testBill              models.Purchase
}

func TestPaymentSalesIntegrationSuite(t *testing.T) {
	suite.Run(t, new(PaymentSalesIntegrationTestSuite))
}

// SetupSuite runs once before all tests
func (suite *PaymentSalesIntegrationTestSuite) SetupSuite() {
	// Setup test database
	suite.db = config.SetupTestDatabase()
	
	// Initialize repositories
	suite.contactRepo = repositories.NewContactRepository(suite.db)
	suite.cashBankRepo = repositories.NewCashBankRepository(suite.db)
	suite.salesRepo = repositories.NewSalesRepository(suite.db)
	suite.purchaseRepo = repositories.NewPurchaseRepository(suite.db)
	suite.paymentRepo = repositories.NewPaymentRepository(suite.db)
	
	// Initialize services
	suite.paymentService = services.NewPaymentService(suite.db, suite.paymentRepo)
	suite.enhancedPaymentService = services.NewEnhancedPaymentService(
		suite.db, 
		suite.paymentService,
		suite.salesRepo,
		suite.purchaseRepo,
	)
	suite.eventService = services.NewSalesUpdateEventService(
		suite.db,
		suite.salesRepo,
		suite.purchaseRepo,
		suite.paymentRepo,
		3, // 3 workers for testing
	)
	
	// Start event service
	suite.eventService.Start()
	
	// Initialize controllers
	suite.enhancedPaymentController = controllers.NewEnhancedPaymentController(
		suite.db,
		suite.paymentService,
		suite.enhancedPaymentService,
		suite.salesRepo,
		suite.purchaseRepo,
		suite.contactRepo,
		suite.cashBankRepo,
	)
	
	// Setup router
	suite.setupRouter()
	
	// Create test data
	suite.createTestData()
}

// TearDownSuite runs once after all tests
func (suite *PaymentSalesIntegrationTestSuite) TearDownSuite() {
	// Stop event service
	suite.eventService.Stop()
	
	// Clean up test database
	config.CleanupTestDatabase(suite.db)
}

// SetupTest runs before each test
func (suite *PaymentSalesIntegrationTestSuite) SetupTest() {
	// Reset test data to known state
	suite.resetTestData()
}

// Setup Gin router with middleware and routes
func (suite *PaymentSalesIntegrationTestSuite) setupRouter() {
	gin.SetMode(gin.TestMode)
	suite.router = gin.New()
	
	// Setup validation middleware
	validationConfig := middleware.ValidationConfig{
		MaxPaymentAmount:     10000000, // 10M limit for testing
		AllowNegativeBalance: false,
		RequireReference:     false,
	}
	
	paymentValidation := middleware.NewPaymentValidationMiddleware(
		suite.db,
		suite.contactRepo,
		suite.cashBankRepo,
		suite.salesRepo,
		suite.purchaseRepo,
		suite.paymentRepo,
		validationConfig,
	)
	
	// API routes
	api := suite.router.Group("/api")
	{
		payments := api.Group("/payments")
		{
			payments.POST("/enhanced", 
				paymentValidation.ValidatePaymentRequest(),
				suite.enhancedPaymentController.RecordEnhancedPayment,
			)
			payments.POST("/validate", paymentValidation.ValidatePaymentRequest())
		}
	}
}

// Create test data
func (suite *PaymentSalesIntegrationTestSuite) createTestData() {
	// Create test customer
	suite.testCustomer = models.Contact{
		Name:        "Test Customer Ltd.",
		Type:        "CUSTOMER",
		Email:       "customer@test.com",
		Phone:       "081234567890",
		Address:     "Test Address",
		IsActive:    true,
	}
	suite.db.Create(&suite.testCustomer)
	
	// Create test vendor
	suite.testVendor = models.Contact{
		Name:        "Test Vendor Inc.",
		Type:        "VENDOR", 
		Email:       "vendor@test.com",
		Phone:       "081234567891",
		Address:     "Vendor Address",
		IsActive:    true,
	}
	suite.db.Create(&suite.testVendor)
	
	// Create test cash account
	suite.testCashAccount = models.CashBank{
		Code:      "CASH001",
		Name:      "Petty Cash",
		Type:      "CASH",
		Balance:   5000000, // 5M initial balance
		IsActive:  true,
	}
	suite.db.Create(&suite.testCashAccount)
	
	// Create test bank account
	suite.testBankAccount = models.CashBank{
		Code:      "BANK001", 
		Name:      "BCA Main Account",
		Type:      "BANK",
		Balance:   50000000, // 50M initial balance
		IsActive:  true,
	}
	suite.db.Create(&suite.testBankAccount)
	
	// Create test invoice (unpaid)
	suite.testInvoice = models.Sale{
		Code:              "INV-2024/001",
		CustomerID:        suite.testCustomer.ID,
		Date:              time.Now(),
		DueDate:           time.Now().Add(30 * 24 * time.Hour),
		TotalAmount:       2000000, // 2M
		OutstandingAmount: 2000000, // Fully outstanding
		Status:            models.SaleStatusUnpaid,
	}
	suite.db.Create(&suite.testInvoice)
	
	// Create test bill (unpaid)
	suite.testBill = models.Purchase{
		Code:              "BILL-2024/001",
		VendorID:          suite.testVendor.ID,
		Date:              time.Now(),
		DueDate:           time.Now().Add(30 * 24 * time.Hour),
		TotalAmount:       1500000, // 1.5M
		OutstandingAmount: 1500000, // Fully outstanding
		Status:            models.PurchaseStatusUnpaid,
	}
	suite.db.Create(&suite.testBill)
}

// Reset test data to initial state
func (suite *PaymentSalesIntegrationTestSuite) resetTestData() {
	// Delete all payments and allocations
	suite.db.Unscoped().Delete(&models.PaymentAllocation{}, "1=1")
	suite.db.Unscoped().Delete(&models.Payment{}, "1=1")
	
	// Reset cash/bank balances
	suite.db.Model(&suite.testCashAccount).Update("balance", 5000000)
	suite.db.Model(&suite.testBankAccount).Update("balance", 50000000)
	
	// Reset invoice/bill status and outstanding
	suite.db.Model(&suite.testInvoice).Updates(map[string]interface{}{
		"outstanding_amount": 2000000,
		"status":            models.SaleStatusUnpaid,
	})
	
	suite.db.Model(&suite.testBill).Updates(map[string]interface{}{
		"outstanding_amount": 1500000,
		"status":            models.PurchaseStatusUnpaid,
	})
	
	// Give event service time to process any remaining events
	time.Sleep(100 * time.Millisecond)
}

// 🧪 TEST CASES

// Test 1: Customer Payment - Full Invoice Payment
func (suite *PaymentSalesIntegrationTestSuite) TestCustomerPayment_FullInvoicePayment() {
	paymentData := map[string]interface{}{
		"contact_id":        suite.testCustomer.ID,
		"amount":           2000000, // Full invoice amount
		"date":             time.Now().Format("2006-01-02"),
		"target_invoice_id": suite.testInvoice.ID,
		"reference":        "CUST-PAY-001",
		"notes":            "Full payment for invoice " + suite.testInvoice.Code,
	}
	
	// Send request
	response := suite.makeRequest("POST", "/api/payments/enhanced", paymentData)
	
	// Verify response
	assert.Equal(suite.T(), http.StatusCreated, response.Code)
	
	var result map[string]interface{}
	err := json.Unmarshal(response.Body.Bytes(), &result)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), result, "data")
	
	data := result["data"].(map[string]interface{})
	payment := data["payment"].(map[string]interface{})
	allocations := data["allocations"].([]interface{})
	summary := data["summary"].(map[string]interface{})
	
	// Verify payment details
	assert.Equal(suite.T(), "RECEIVABLE", payment["method"])
	assert.Equal(suite.T(), "COMPLETED", payment["status"])
	assert.Equal(suite.T(), float64(2000000), payment["amount"])
	
	// Verify allocations
	assert.Equal(suite.T(), 1, len(allocations))
	allocation := allocations[0].(map[string]interface{})
	assert.Equal(suite.T(), float64(2000000), allocation["allocated_amount"])
	assert.NotNil(suite.T(), allocation["invoice_id"])
	
	// Verify summary
	assert.Equal(suite.T(), float64(2000000), summary["total_processed"])
	assert.Equal(suite.T(), float64(2000000), summary["allocated_amount"])
	assert.Equal(suite.T(), float64(0), summary["unallocated_amount"])
	assert.Equal(suite.T(), float64(1), summary["invoices_updated"])
	assert.True(suite.T(), summary["cash_bank_updated"].(bool))
	
	// Wait for event processing
	time.Sleep(200 * time.Millisecond)
	
	// Verify database state
	suite.verifyInvoiceStatusAndOutstanding(suite.testInvoice.ID, models.SaleStatusPaid, 0)
	suite.verifyCashBankBalance(suite.testBankAccount.ID, 52000000) // +2M
}

// Test 2: Customer Payment - Partial Invoice Payment  
func (suite *PaymentSalesIntegrationTestSuite) TestCustomerPayment_PartialInvoicePayment() {
	paymentData := map[string]interface{}{
		"contact_id":        suite.testCustomer.ID,
		"amount":           1000000, // Half invoice amount
		"date":             time.Now().Format("2006-01-02"),
		"target_invoice_id": suite.testInvoice.ID,
		"reference":        "CUST-PAY-002",
	}
	
	response := suite.makeRequest("POST", "/api/payments/enhanced", paymentData)
	assert.Equal(suite.T(), http.StatusCreated, response.Code)
	
	// Wait for event processing
	time.Sleep(200 * time.Millisecond)
	
	// Verify invoice is now partial
	suite.verifyInvoiceStatusAndOutstanding(suite.testInvoice.ID, models.SaleStatusPartial, 1000000)
	suite.verifyCashBankBalance(suite.testBankAccount.ID, 51000000) // +1M
}

// Test 3: Vendor Payment - Full Bill Payment
func (suite *PaymentSalesIntegrationTestSuite) TestVendorPayment_FullBillPayment() {
	paymentData := map[string]interface{}{
		"contact_id":     suite.testVendor.ID,
		"amount":        1500000, // Full bill amount
		"date":          time.Now().Format("2006-01-02"),
		"target_bill_id": suite.testBill.ID,
		"reference":     "VENDOR-PAY-001",
		"notes":         "Full payment for bill " + suite.testBill.Code,
	}
	
	response := suite.makeRequest("POST", "/api/payments/enhanced", paymentData)
	assert.Equal(suite.T(), http.StatusCreated, response.Code)
	
	var result map[string]interface{}
	json.Unmarshal(response.Body.Bytes(), &result)
	data := result["data"].(map[string]interface{})
	payment := data["payment"].(map[string]interface{})
	
	// Verify payment method auto-detection
	assert.Equal(suite.T(), "PAYABLE", payment["method"])
	assert.Equal(suite.T(), "COMPLETED", payment["status"])
	
	// Wait for event processing
	time.Sleep(200 * time.Millisecond)
	
	// Verify bill is fully paid
	suite.verifyBillStatusAndOutstanding(suite.testBill.ID, models.PurchaseStatusPaid, 0)
	suite.verifyCashBankBalance(suite.testBankAccount.ID, 48500000) // -1.5M
}

// Test 4: Auto-allocation without specific target
func (suite *PaymentSalesIntegrationTestSuite) TestCustomerPayment_AutoAllocation() {
	paymentData := map[string]interface{}{
		"contact_id":   suite.testCustomer.ID,
		"amount":       2500000, // More than invoice amount
		"date":         time.Now().Format("2006-01-02"),
		"auto_allocate": true,
		"reference":    "AUTO-ALLOC-001",
	}
	
	response := suite.makeRequest("POST", "/api/payments/enhanced", paymentData)
	assert.Equal(suite.T(), http.StatusCreated, response.Code)
	
	var result map[string]interface{}
	json.Unmarshal(response.Body.Bytes(), &result)
	data := result["data"].(map[string]interface{})
	summary := data["summary"].(map[string]interface{})
	
	// Should allocate 2M to invoice and 0.5M unallocated
	assert.Equal(suite.T(), float64(2500000), summary["total_processed"])
	assert.Equal(suite.T(), float64(2000000), summary["allocated_amount"]) 
	assert.Equal(suite.T(), float64(500000), summary["unallocated_amount"])
	
	// Wait for event processing
	time.Sleep(200 * time.Millisecond)
	
	// Invoice should be fully paid
	suite.verifyInvoiceStatusAndOutstanding(suite.testInvoice.ID, models.SaleStatusPaid, 0)
}

// Test 5: Validation - Insufficient Balance
func (suite *PaymentSalesIntegrationTestSuite) TestVendorPayment_InsufficientBalance() {
	// First, reduce bank balance to create insufficient funds scenario
	suite.db.Model(&suite.testBankAccount).Update("balance", 1000000) // Only 1M available
	
	paymentData := map[string]interface{}{
		"contact_id": suite.testVendor.ID,
		"amount":    1500000, // Trying to pay 1.5M with only 1M available
		"date":      time.Now().Format("2006-01-02"),
		"reference": "INSUFFICIENT-FUNDS",
	}
	
	response := suite.makeRequest("POST", "/api/payments/enhanced", paymentData)
	
	// Should be rejected due to insufficient balance
	assert.Equal(suite.T(), http.StatusBadRequest, response.Code)
	
	var result map[string]interface{}
	json.Unmarshal(response.Body.Bytes(), &result)
	
	errorData := result["error"].(map[string]interface{})
	assert.Contains(suite.T(), errorData["details"].(string), "insufficient")
}

// Test 6: Validation - Invalid Contact Type Mismatch
func (suite *PaymentSalesIntegrationTestSuite) TestValidation_ContactTypeMismatch() {
	paymentData := map[string]interface{}{
		"contact_id": suite.testCustomer.ID, // Customer
		"method":     "PAYABLE",             // But trying to use vendor payment method
		"amount":     1000000,
		"date":       time.Now().Format("2006-01-02"),
	}
	
	response := suite.makeRequest("POST", "/api/payments/enhanced", paymentData)
	
	// Should be rejected due to method mismatch
	assert.Equal(suite.T(), http.StatusBadRequest, response.Code)
	
	var result map[string]interface{}
	json.Unmarshal(response.Body.Bytes(), &result)
	
	validation := result["validation"].(map[string]interface{})
	errors := validation["errors"].([]interface{})
	assert.True(suite.T(), len(errors) > 0)
	assert.Contains(suite.T(), errors[0].(string), "RECEIVABLE")
}

// Test 7: Validation - Target Invoice/Bill Mismatch
func (suite *PaymentSalesIntegrationTestSuite) TestValidation_TargetMismatch() {
	paymentData := map[string]interface{}{
		"contact_id":        suite.testVendor.ID, // Vendor
		"target_invoice_id": suite.testInvoice.ID, // But trying to pay customer invoice
		"amount":           1000000,
		"date":             time.Now().Format("2006-01-02"),
	}
	
	response := suite.makeRequest("POST", "/api/payments/enhanced", paymentData)
	
	// Should be rejected due to target mismatch
	assert.Equal(suite.T(), http.StatusBadRequest, response.Code)
	
	var result map[string]interface{}
	json.Unmarshal(response.Body.Bytes(), &result)
	
	validation := result["validation"].(map[string]interface{})
	errors := validation["errors"].([]interface{})
	assert.True(suite.T(), len(errors) > 0)
	assert.Contains(suite.T(), errors[0].(string), "customer payments")
}

// Test 8: Event Service - Real-time Updates
func (suite *PaymentSalesIntegrationTestSuite) TestEventService_RealTimeUpdates() {
	// Make multiple concurrent payments
	paymentData1 := map[string]interface{}{
		"contact_id":        suite.testCustomer.ID,
		"amount":           1000000,
		"date":             time.Now().Format("2006-01-02"),
		"target_invoice_id": suite.testInvoice.ID,
		"reference":        "CONCURRENT-1",
	}
	
	paymentData2 := map[string]interface{}{
		"contact_id":        suite.testCustomer.ID,
		"amount":           1000000,
		"date":             time.Now().Format("2006-01-02"), 
		"target_invoice_id": suite.testInvoice.ID,
		"reference":        "CONCURRENT-2",
	}
	
	// Send both requests
	response1 := suite.makeRequest("POST", "/api/payments/enhanced", paymentData1)
	response2 := suite.makeRequest("POST", "/api/payments/enhanced", paymentData2)
	
	assert.Equal(suite.T(), http.StatusCreated, response1.Code)
	assert.Equal(suite.T(), http.StatusCreated, response2.Code)
	
	// Wait for all events to be processed
	time.Sleep(500 * time.Millisecond)
	
	// Verify final state - invoice should be fully paid
	suite.verifyInvoiceStatusAndOutstanding(suite.testInvoice.ID, models.SaleStatusPaid, 0)
	
	// Verify both payments were recorded
	var paymentCount int64
	suite.db.Model(&models.Payment{}).Where("contact_id = ?", suite.testCustomer.ID).Count(&paymentCount)
	assert.Equal(suite.T(), int64(2), paymentCount)
	
	// Verify total allocations
	var totalAllocated float64
	suite.db.Model(&models.PaymentAllocation{}).
		Where("invoice_id = ?", suite.testInvoice.ID).
		Select("COALESCE(SUM(allocated_amount), 0)").
		Scan(&totalAllocated)
	assert.Equal(suite.T(), float64(2000000), totalAllocated)
}

// Test 9: Complete Sales-to-Payment Workflow
func (suite *PaymentSalesIntegrationTestSuite) TestCompleteWorkflow_SalesToPayment() {
	// Step 1: Verify initial invoice state
	suite.verifyInvoiceStatusAndOutstanding(suite.testInvoice.ID, models.SaleStatusUnpaid, 2000000)
	
	// Step 2: Make partial payment
	paymentData1 := map[string]interface{}{
		"contact_id":        suite.testCustomer.ID,
		"amount":           800000, // Partial payment
		"date":             time.Now().Format("2006-01-02"),
		"target_invoice_id": suite.testInvoice.ID,
		"reference":        "WORKFLOW-PARTIAL",
	}
	
	response1 := suite.makeRequest("POST", "/api/payments/enhanced", paymentData1)
	assert.Equal(suite.T(), http.StatusCreated, response1.Code)
	
	// Wait and verify partial state
	time.Sleep(200 * time.Millisecond)
	suite.verifyInvoiceStatusAndOutstanding(suite.testInvoice.ID, models.SaleStatusPartial, 1200000)
	
	// Step 3: Complete payment
	paymentData2 := map[string]interface{}{
		"contact_id":        suite.testCustomer.ID,
		"amount":           1200000, // Remaining amount
		"date":             time.Now().Format("2006-01-02"),
		"target_invoice_id": suite.testInvoice.ID,
		"reference":        "WORKFLOW-FINAL",
	}
	
	response2 := suite.makeRequest("POST", "/api/payments/enhanced", paymentData2)
	assert.Equal(suite.T(), http.StatusCreated, response2.Code)
	
	// Wait and verify final state
	time.Sleep(200 * time.Millisecond)
	suite.verifyInvoiceStatusAndOutstanding(suite.testInvoice.ID, models.SaleStatusPaid, 0)
	
	// Verify cash balance updated correctly
	suite.verifyCashBankBalance(suite.testBankAccount.ID, 52000000) // +2M total
}

// Test 10: System Health and Performance
func (suite *PaymentSalesIntegrationTestSuite) TestSystemHealthAndPerformance() {
	// Check event service health
	err := suite.eventService.HealthCheck()
	assert.NoError(suite.T(), err)
	
	// Check event service status
	status := suite.eventService.GetStatus()
	assert.True(suite.T(), status["is_running"].(bool))
	assert.Equal(suite.T(), 3, status["worker_pool_size"].(int))
	
	// Make multiple rapid payments to test performance
	startTime := time.Now()
	
	for i := 0; i < 10; i++ {
		paymentData := map[string]interface{}{
			"contact_id": suite.testCustomer.ID,
			"amount":     100000, // Small amounts
			"date":       time.Now().Format("2006-01-02"),
			"reference":  fmt.Sprintf("PERF-TEST-%d", i),
		}
		
		response := suite.makeRequest("POST", "/api/payments/enhanced", paymentData)
		assert.Equal(suite.T(), http.StatusCreated, response.Code)
	}
	
	processingTime := time.Since(startTime)
	suite.T().Logf("10 payments processed in %v", processingTime)
	
	// Should complete within reasonable time (less than 5 seconds)
	assert.Less(suite.T(), processingTime, 5*time.Second)
	
	// Wait for all events to be processed
	time.Sleep(1 * time.Second)
	
	// Verify all payments were recorded
	var paymentCount int64
	suite.db.Model(&models.Payment{}).Where("reference LIKE 'PERF-TEST-%'").Count(&paymentCount)
	assert.Equal(suite.T(), int64(10), paymentCount)
}

// 🛠️ HELPER METHODS

func (suite *PaymentSalesIntegrationTestSuite) makeRequest(method, url string, data interface{}) *httptest.ResponseRecorder {
	var reqBody []byte
	var err error
	
	if data != nil {
		reqBody, err = json.Marshal(data)
		assert.NoError(suite.T(), err)
	}
	
	req := httptest.NewRequest(method, url, bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	
	// Add user context for testing
	req.Header.Set("X-User-ID", "1")
	
	recorder := httptest.NewRecorder()
	suite.router.ServeHTTP(recorder, req)
	
	return recorder
}

func (suite *PaymentSalesIntegrationTestSuite) verifyInvoiceStatusAndOutstanding(invoiceID uint, expectedStatus string, expectedOutstanding float64) {
	var sale models.Sale
	err := suite.db.First(&sale, invoiceID).Error
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedStatus, sale.Status)
	assert.Equal(suite.T(), expectedOutstanding, sale.OutstandingAmount)
}

func (suite *PaymentSalesIntegrationTestSuite) verifyBillStatusAndOutstanding(billID uint, expectedStatus string, expectedOutstanding float64) {
	var purchase models.Purchase
	err := suite.db.First(&purchase, billID).Error
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedStatus, purchase.Status)
	assert.Equal(suite.T(), expectedOutstanding, purchase.OutstandingAmount)
}

func (suite *PaymentSalesIntegrationTestSuite) verifyCashBankBalance(cashBankID uint, expectedBalance float64) {
	var cashBank models.CashBank
	err := suite.db.First(&cashBank, cashBankID).Error
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedBalance, cashBank.Balance)
}
