//go:build legacytests
// +build legacytests

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/controllers"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/routes"
	"app-sistem-akuntansi/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Integration test suite
type ReportIntegrationTestSuite struct {
	suite.Suite
	db     *gorm.DB
	router *gin.Engine
}

// SetupSuite runs once before all tests
func (suite *ReportIntegrationTestSuite) SetupSuite() {
	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	suite.Require().NoError(err)

	// Auto-migrate models
	err = db.AutoMigrate(
		&models.Account{},
		&models.Contact{},
		&models.Product{},
		&models.Sale{},
		&models.SaleItem{},
		&models.Purchase{},
		&models.PurchaseItem{},
		&models.JournalEntry{},
		&models.JournalEntryItem{},
		&models.User{},
		&models.Company{},
	)
	suite.Require().NoError(err)

	suite.db = db

	// Seed test data
	suite.seedTestData()

	// Setup Gin router
	gin.SetMode(gin.TestMode)
	suite.router = gin.New()

	// Initialize repositories
	accountRepo := repositories.NewAccountRepository(db)
	contactRepo := repositories.NewContactRepository(db)
	productRepo := repositories.NewProductRepository(db)
	salesRepo := repositories.NewSalesRepository(db)
	purchaseRepo := repositories.NewPurchaseRepository(db)
	journalRepo := repositories.NewJournalRepository(db)

	// Initialize services
	balanceSheetService := services.NewBalanceSheetService(accountRepo, journalRepo)
	profitLossService := services.NewProfitLossService(accountRepo, journalRepo, salesRepo, purchaseRepo)
	cashFlowService := services.NewCashFlowService(accountRepo, journalRepo)
	reportService := services.NewReportService(db)

	// Initialize controller
	controller := controllers.NewUnifiedReportController(
		db,
		accountRepo,
		salesRepo,
		purchaseRepo,
		contactRepo,
		productRepo,
		reportService,
		balanceSheetService,
		profitLossService,
		cashFlowService,
	)

	// Setup routes
	routes.RegisterUnifiedReportRoutes(suite.router, controller)
}

// TearDownSuite runs once after all tests
func (suite *ReportIntegrationTestSuite) TearDownSuite() {
	// Close database connection
	sqlDB, _ := suite.db.DB()
	sqlDB.Close()
}

// SetupTest runs before each test
func (suite *ReportIntegrationTestSuite) SetupTest() {
	// Clean up data if needed
}

// TearDownTest runs after each test  
func (suite *ReportIntegrationTestSuite) TearDownTest() {
	// Clean up data if needed
}

// Seed test data
func (suite *ReportIntegrationTestSuite) seedTestData() {
	// Create test company
	company := models.Company{
		ID:      1,
		Name:    "Test Company Ltd",
		Address: "123 Test Street",
		City:    "Test City",
		Country: "Test Country",
		Phone:   "123-456-7890",
		Email:   "test@company.com",
	}
	suite.db.Create(&company)

	// Create test user
	user := models.User{
		ID:       1,
		Username: "testuser",
		Email:    "test@user.com",
		Name:     "Test User",
		Role:     models.UserRoleAdmin,
		IsActive: true,
	}
	suite.db.Create(&user)

	// Create chart of accounts
	accounts := []models.Account{
		// Assets
		{ID: 1, Code: "1101", Name: "Kas", Type: models.AccountTypeAsset, Balance: 50000000, IsActive: true},
		{ID: 2, Code: "1102", Name: "Bank BCA", Type: models.AccountTypeAsset, Balance: 100000000, IsActive: true},
		{ID: 3, Code: "1201", Name: "Piutang Usaha", Type: models.AccountTypeAsset, Balance: 25000000, IsActive: true},
		{ID: 4, Code: "1301", Name: "Persediaan", Type: models.AccountTypeAsset, Balance: 75000000, IsActive: true},
		{ID: 5, Code: "1501", Name: "Peralatan", Type: models.AccountTypeAsset, Balance: 200000000, IsActive: true},

		// Liabilities
		{ID: 6, Code: "2101", Name: "Hutang Usaha", Type: models.AccountTypeLiability, Balance: 30000000, IsActive: true},
		{ID: 7, Code: "2201", Name: "Hutang Bank", Type: models.AccountTypeLiability, Balance: 80000000, IsActive: true},

		// Equity
		{ID: 8, Code: "3101", Name: "Modal Saham", Type: models.AccountTypeEquity, Balance: 300000000, IsActive: true},
		{ID: 9, Code: "3201", Name: "Laba Ditahan", Type: models.AccountTypeEquity, Balance: 40000000, IsActive: true},

		// Revenue
		{ID: 10, Code: "4101", Name: "Pendapatan Penjualan", Type: models.AccountTypeRevenue, Balance: 0, IsActive: true},
		{ID: 11, Code: "4201", Name: "Pendapatan Jasa", Type: models.AccountTypeRevenue, Balance: 0, IsActive: true},

		// Expenses
		{ID: 12, Code: "5101", Name: "Harga Pokok Penjualan", Type: models.AccountTypeExpense, Balance: 0, IsActive: true},
		{ID: 13, Code: "5201", Name: "Beban Gaji", Type: models.AccountTypeExpense, Balance: 0, IsActive: true},
		{ID: 14, Code: "5301", Name: "Beban Sewa", Type: models.AccountTypeExpense, Balance: 0, IsActive: true},
	}

	for _, account := range accounts {
		suite.db.Create(&account)
	}

	// Create test contacts
	contacts := []models.Contact{
		{ID: 1, Name: "PT. Supplier A", Type: models.ContactTypeSupplier, Email: "supplier@a.com", Phone: "111-111-1111", IsActive: true},
		{ID: 2, Name: "CV. Customer B", Type: models.ContactTypeCustomer, Email: "customer@b.com", Phone: "222-222-2222", IsActive: true},
	}

	for _, contact := range contacts {
		suite.db.Create(&contact)
	}

	// Create test products
	products := []models.Product{
		{ID: 1, Code: "PRD001", Name: "Product A", SalePrice: 100000, CostPrice: 60000, Stock: 100, IsActive: true},
		{ID: 2, Code: "PRD002", Name: "Product B", SalePrice: 150000, CostPrice: 90000, Stock: 50, IsActive: true},
	}

	for _, product := range products {
		suite.db.Create(&product)
	}

	// Create test sales
	sale := models.Sale{
		ID:         1,
		InvoiceNo:  "INV-2024-001",
		Date:       time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		CustomerID: 2,
		Total:      250000,
		Status:     models.SaleStatusCompleted,
		Items: []models.SaleItem{
			{ID: 1, SaleID: 1, ProductID: 1, Quantity: 1, UnitPrice: 100000, Total: 100000},
			{ID: 2, SaleID: 1, ProductID: 2, Quantity: 1, UnitPrice: 150000, Total: 150000},
		},
	}
	suite.db.Create(&sale)

	// Create test purchases
	purchase := models.Purchase{
		ID:         1,
		InvoiceNo:  "PUR-2024-001",
		Date:       time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
		SupplierID: 1,
		Total:      150000,
		Status:     models.PurchaseStatusCompleted,
		Items: []models.PurchaseItem{
			{ID: 1, PurchaseID: 1, ProductID: 1, Quantity: 2, UnitPrice: 60000, Total: 120000},
			{ID: 2, PurchaseID: 1, ProductID: 2, Quantity: 1, UnitPrice: 30000, Total: 30000},
		},
	}
	suite.db.Create(&purchase)

	// Create test journal entries
	journalEntries := []models.JournalEntry{
		{
			ID:          1,
			Date:        time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Reference:   "SALE-INV-2024-001",
			Description: "Sales Invoice INV-2024-001",
			Items: []models.JournalEntryItem{
				{ID: 1, JournalEntryID: 1, AccountID: 3, Debit: 250000, Credit: 0}, // Piutang Usaha
				{ID: 2, JournalEntryID: 1, AccountID: 10, Debit: 0, Credit: 250000}, // Pendapatan Penjualan
			},
		},
		{
			ID:          2,
			Date:        time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
			Reference:   "PUR-PUR-2024-001",
			Description: "Purchase Invoice PUR-2024-001",
			Items: []models.JournalEntryItem{
				{ID: 3, JournalEntryID: 2, AccountID: 4, Debit: 150000, Credit: 0}, // Persediaan
				{ID: 4, JournalEntryID: 2, AccountID: 6, Debit: 0, Credit: 150000}, // Hutang Usaha
			},
		},
	}

	for _, entry := range journalEntries {
		suite.db.Create(&entry)
	}
}

// Test complete report generation flow
func (suite *ReportIntegrationTestSuite) TestCompleteReportFlow() {
	// Test Balance Sheet
	suite.Run("Balance Sheet Generation", func() {
		req := httptest.NewRequest("GET", "/api/reports/balance-sheet?format=json", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)

		assert.Equal(suite.T(), http.StatusOK, w.Code)

		var response models.StandardReportResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(suite.T(), err)
		assert.True(suite.T(), response.Success)
		assert.NotNil(suite.T(), response.Data)
		assert.Equal(suite.T(), "balance-sheet", response.Metadata.ReportType)

		// Verify data structure
		data, ok := response.Data.(map[string]interface{})
		assert.True(suite.T(), ok)
		assert.Contains(suite.T(), data, "assets")
		assert.Contains(suite.T(), data, "liabilities")
		assert.Contains(suite.T(), data, "equity")
	})

	// Test Profit & Loss
	suite.Run("Profit & Loss Generation", func() {
		req := httptest.NewRequest("GET", "/api/reports/profit-loss?start_date=2024-01-01&end_date=2024-01-31&format=json", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)

		assert.Equal(suite.T(), http.StatusOK, w.Code)

		var response models.StandardReportResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(suite.T(), err)
		assert.True(suite.T(), response.Success)
		assert.Equal(suite.T(), "profit-loss", response.Metadata.ReportType)

		// Verify data structure
		data, ok := response.Data.(map[string]interface{})
		assert.True(suite.T(), ok)
		assert.Contains(suite.T(), data, "revenue")
		assert.Contains(suite.T(), data, "expenses")
		assert.Contains(suite.T(), data, "net_income")
	})

	// Test Cash Flow
	suite.Run("Cash Flow Generation", func() {
		req := httptest.NewRequest("GET", "/api/reports/cash-flow?start_date=2024-01-01&end_date=2024-01-31&format=json", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)

		assert.Equal(suite.T(), http.StatusOK, w.Code)

		var response models.StandardReportResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(suite.T(), err)
		assert.True(suite.T(), response.Success)
		assert.Equal(suite.T(), "cash-flow", response.Metadata.ReportType)
	})
}

// Test all report types with different parameters
func (suite *ReportIntegrationTestSuite) TestAllReportTypes() {
	testCases := []struct {
		name        string
		endpoint    string
		params      string
		expectedType string
	}{
		{
			name:         "Trial Balance",
			endpoint:     "/api/reports/trial-balance",
			params:       "?format=json",
			expectedType: "trial-balance",
		},
		{
			name:         "General Ledger",
			endpoint:     "/api/reports/general-ledger",
			params:       "?start_date=2024-01-01&end_date=2024-01-31&format=json",
			expectedType: "general-ledger",
		},
		{
			name:         "Sales Summary",
			endpoint:     "/api/reports/sales-summary",
			params:       "?start_date=2024-01-01&end_date=2024-01-31&group_by=day&format=json",
			expectedType: "sales-summary",
		},
		{
			name:         "Vendor Analysis",
			endpoint:     "/api/reports/vendor-analysis",
			params:       "?start_date=2024-01-01&end_date=2024-01-31&format=json",
			expectedType: "vendor-analysis",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			url := tc.endpoint + tc.params
			req := httptest.NewRequest("GET", url, nil)
			req.Header.Set("Authorization", "Bearer test-token")
			w := httptest.NewRecorder()
			suite.router.ServeHTTP(w, req)

			assert.Equal(suite.T(), http.StatusOK, w.Code)

			var response models.StandardReportResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(suite.T(), err)
			assert.True(suite.T(), response.Success)
			assert.Equal(suite.T(), tc.expectedType, response.Metadata.ReportType)
		})
	}
}

// Test preview functionality
func (suite *ReportIntegrationTestSuite) TestPreviewFunctionality() {
	testCases := []string{
		"balance-sheet",
		"profit-loss",
		"cash-flow",
		"trial-balance",
	}

	for _, reportType := range testCases {
		suite.Run(fmt.Sprintf("Preview %s", reportType), func() {
			params := "?format=json"
			if reportType == "profit-loss" || reportType == "cash-flow" {
				params = "?start_date=2024-01-01&end_date=2024-01-31&format=json"
			}

			url := fmt.Sprintf("/api/reports/preview/%s%s", reportType, params)
			req := httptest.NewRequest("GET", url, nil)
			req.Header.Set("Authorization", "Bearer test-token")
			w := httptest.NewRecorder()
			suite.router.ServeHTTP(w, req)

			assert.Equal(suite.T(), http.StatusOK, w.Code)

			var response models.StandardReportResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(suite.T(), err)
			assert.True(suite.T(), response.Success)
			assert.Equal(suite.T(), reportType, response.Metadata.ReportType)
			assert.Equal(suite.T(), "json", response.Metadata.Format) // Preview should always be JSON
		})
	}
}

// Test different output formats
func (suite *ReportIntegrationTestSuite) TestOutputFormats() {
	formats := []struct {
		format      string
		contentType string
	}{
		{"json", "application/json"},
		{"pdf", "application/pdf"},
		{"excel", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{"csv", "text/csv"},
	}

	for _, fmt := range formats {
		suite.Run(fmt.Sprintf("Format %s", fmt.format), func() {
			url := fmt.Sprintf("/api/reports/balance-sheet?format=%s", fmt.format)
			req := httptest.NewRequest("GET", url, nil)
			req.Header.Set("Authorization", "Bearer test-token")
			w := httptest.NewRecorder()
			suite.router.ServeHTTP(w, req)

			assert.Equal(suite.T(), http.StatusOK, w.Code)

			if fmt.format == "json" {
				// JSON response
				assert.Contains(suite.T(), w.Header().Get("Content-Type"), fmt.contentType)
				
				var response models.StandardReportResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(suite.T(), err)
				assert.True(suite.T(), response.Success)
			} else {
				// File download
				assert.Equal(suite.T(), fmt.contentType, w.Header().Get("Content-Type"))
				assert.Contains(suite.T(), w.Header().Get("Content-Disposition"), "attachment")
				assert.Greater(suite.T(), w.Body.Len(), 0)
			}
		})
	}
}

// Test error scenarios
func (suite *ReportIntegrationTestSuite) TestErrorScenarios() {
	testCases := []struct {
		name         string
		url          string
		expectedCode int
		description  string
	}{
		{
			name:         "Invalid report type",
			url:          "/api/reports/invalid-report?format=json",
			expectedCode: http.StatusBadRequest,
			description:  "Should return 400 for invalid report type",
		},
		{
			name:         "Missing required parameters",
			url:          "/api/reports/profit-loss", // Missing start_date and end_date
			expectedCode: http.StatusBadRequest,
			description:  "Should return 400 for missing parameters",
		},
		{
			name:         "Invalid date format",
			url:          "/api/reports/profit-loss?start_date=invalid&end_date=2024-01-31&format=json",
			expectedCode: http.StatusBadRequest,
			description:  "Should return 400 for invalid date format",
		},
		{
			name:         "Invalid date range",
			url:          "/api/reports/profit-loss?start_date=2024-01-31&end_date=2024-01-01&format=json",
			expectedCode: http.StatusBadRequest,
			description:  "Should return 400 for invalid date range",
		},
		{
			name:         "Invalid format",
			url:          "/api/reports/balance-sheet?format=invalid",
			expectedCode: http.StatusBadRequest,
			description:  "Should return 400 for invalid format",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			req := httptest.NewRequest("GET", tc.url, nil)
			req.Header.Set("Authorization", "Bearer test-token")
			w := httptest.NewRecorder()
			suite.router.ServeHTTP(w, req)

			assert.Equal(suite.T(), tc.expectedCode, w.Code, tc.description)

			if w.Code >= 400 {
				var response models.StandardReportResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(suite.T(), err)
				assert.False(suite.T(), response.Success)
				assert.NotNil(suite.T(), response.Error)
			}
		})
	}
}

// Test available reports endpoint
func (suite *ReportIntegrationTestSuite) TestAvailableReports() {
	req := httptest.NewRequest("GET", "/api/reports/available", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)

	var response models.StandardReportResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), response.Success)

	reports, ok := response.Data.([]interface{})
	assert.True(suite.T(), ok)
	assert.Greater(suite.T(), len(reports), 0)

	// Check structure of first report
	if len(reports) > 0 {
		report, ok := reports[0].(map[string]interface{})
		assert.True(suite.T(), ok)
		assert.Contains(suite.T(), report, "id")
		assert.Contains(suite.T(), report, "name")
		assert.Contains(suite.T(), report, "description")
		assert.Contains(suite.T(), report, "type")
		assert.Contains(suite.T(), report, "parameters")
		assert.Contains(suite.T(), report, "endpoints")
	}
}

// Test performance with concurrent requests
func (suite *ReportIntegrationTestSuite) TestConcurrentRequests() {
	const numRequests = 10
	results := make(chan int, numRequests)

	// Launch concurrent requests
	for i := 0; i < numRequests; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/api/reports/balance-sheet?format=json", nil)
			req.Header.Set("Authorization", "Bearer test-token")
			w := httptest.NewRecorder()
			suite.router.ServeHTTP(w, req)
			results <- w.Code
		}()
	}

	// Collect results
	successCount := 0
	for i := 0; i < numRequests; i++ {
		code := <-results
		if code == http.StatusOK {
			successCount++
		}
	}

	// All requests should succeed
	assert.Equal(suite.T(), numRequests, successCount, "All concurrent requests should succeed")
}

// Test data integrity across multiple reports
func (suite *ReportIntegrationTestSuite) TestDataIntegrity() {
	// Generate balance sheet
	req1 := httptest.NewRequest("GET", "/api/reports/balance-sheet?format=json", nil)
	req1.Header.Set("Authorization", "Bearer test-token")
	w1 := httptest.NewRecorder()
	suite.router.ServeHTTP(w1, req1)

	var balanceSheet models.StandardReportResponse
	err := json.Unmarshal(w1.Body.Bytes(), &balanceSheet)
	suite.Require().NoError(err)

	// Generate trial balance
	req2 := httptest.NewRequest("GET", "/api/reports/trial-balance?format=json", nil)
	req2.Header.Set("Authorization", "Bearer test-token")
	w2 := httptest.NewRecorder()
	suite.router.ServeHTTP(w2, req2)

	var trialBalance models.StandardReportResponse
	err = json.Unmarshal(w2.Body.Bytes(), &trialBalance)
	suite.Require().NoError(err)

	// Both reports should be successful
	assert.True(suite.T(), balanceSheet.Success)
	assert.True(suite.T(), trialBalance.Success)

	// Data consistency checks can be added here based on business logic
	// For example, total assets in balance sheet should match totals in trial balance
}

// Test response timing and performance
func (suite *ReportIntegrationTestSuite) TestResponseTiming() {
	start := time.Now()

	req := httptest.NewRequest("GET", "/api/reports/balance-sheet?format=json", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	duration := time.Since(start)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Less(suite.T(), duration, 2*time.Second, "Report generation should complete within 2 seconds")

	var response models.StandardReportResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), response.Success)
	assert.NotEmpty(suite.T(), response.Metadata.GenerationTime, "Should track generation time")
}

// Run the test suite
func TestReportIntegrationSuite(t *testing.T) {
	suite.Run(t, new(ReportIntegrationTestSuite))
}

// Benchmark tests
func BenchmarkReportGeneration(b *testing.B) {
	suite := &ReportIntegrationTestSuite{}
	suite.SetupSuite()
	defer suite.TearDownSuite()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/api/reports/balance-sheet?format=json", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)
	}
}

func BenchmarkConcurrentReportGeneration(b *testing.B) {
	suite := &ReportIntegrationTestSuite{}
	suite.SetupSuite()
	defer suite.TearDownSuite()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/api/reports/balance-sheet?format=json", nil)
			req.Header.Set("Authorization", "Bearer test-token")
			w := httptest.NewRecorder()
			suite.router.ServeHTTP(w, req)
		}
	})
}
