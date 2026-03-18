//go:build legacytests
// +build legacytests

package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"app-sistem-akuntansi/controllers"
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/routes"
	"app-sistem-akuntansi/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

// Mock implementations
type MockAccountRepository struct {
	mock.Mock
}

func (m *MockAccountRepository) FindAll(ctx context.Context) ([]models.Account, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.Account), args.Error(1)
}

func (m *MockAccountRepository) FindByCode(ctx context.Context, code string) (*models.Account, error) {
	args := m.Called(ctx, code)
	return args.Get(0).(*models.Account), args.Error(1)
}

func (m *MockAccountRepository) Create(ctx context.Context, account *models.Account) error {
	args := m.Called(ctx, account)
	return args.Error(0)
}

func (m *MockAccountRepository) Update(ctx context.Context, account *models.Account) error {
	args := m.Called(ctx, account)
	return args.Error(0)
}

func (m *MockAccountRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// Test setup
func setupTestRouter() (*gin.Engine, *controllers.UnifiedReportController) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Mock dependencies
	mockAccountRepo := &MockAccountRepository{}
	mockDB := &gorm.DB{} // In real tests, use a test database

	// Setup mock data
	mockAccounts := []models.Account{
		{ID: 1, Code: "1101", Name: "Kas", Type: models.AccountTypeAsset, Balance: 50000000, IsActive: true},
		{ID: 2, Code: "1102", Name: "Bank BCA", Type: models.AccountTypeAsset, Balance: 600000000, IsActive: true},
		{ID: 3, Code: "4101", Name: "Pendapatan Penjualan", Type: models.AccountTypeRevenue, Balance: 0, IsActive: true},
		{ID: 4, Code: "5101", Name: "Harga Pokok Penjualan", Type: models.AccountTypeExpense, Balance: 0, IsActive: true},
	}

	mockAccountRepo.On("FindAll", mock.Anything).Return(mockAccounts, nil)
	mockAccountRepo.On("FindByCode", mock.Anything, "1101").Return(&mockAccounts[0], nil)

	// Create controller
	controller := controllers.NewUnifiedReportController(
		mockDB,
		mockAccountRepo,
		nil, // salesRepo
		nil, // purchaseRepo
		nil, // contactRepo
		nil, // productRepo
		nil, // reportService
		nil, // balanceSheetService
		nil, // profitLossService
		nil, // cashFlowService
	)

	// Setup routes
	routes.RegisterUnifiedReportRoutes(router, controller)

	return router, controller
}

// Test Cases

// Test 1: API Endpoint Mismatch Fix
func TestUnifiedReportEndpoints(t *testing.T) {
	router, _ := setupTestRouter()

	testCases := []struct {
		name     string
		endpoint string
		method   string
		params   map[string]string
	}{
		{
			name:     "Balance Sheet Direct Endpoint",
			endpoint: "/api/reports/balance-sheet",
			method:   "GET",
			params:   map[string]string{"format": "json"},
		},
		{
			name:     "Profit Loss Direct Endpoint",
			endpoint: "/api/reports/profit-loss",
			method:   "GET",
			params: map[string]string{
				"start_date": "2024-01-01",
				"end_date":   "2024-01-31",
				"format":     "json",
			},
		},
		{
			name:     "Cash Flow Direct Endpoint",
			endpoint: "/api/reports/cash-flow",
			method:   "GET",
			params: map[string]string{
				"start_date": "2024-01-01",
				"end_date":   "2024-01-31",
				"format":     "json",
			},
		},
		{
			name:     "Trial Balance Direct Endpoint",
			endpoint: "/api/reports/trial-balance",
			method:   "GET",
			params:   map[string]string{"format": "json"},
		},
		{
			name:     "General Ledger Direct Endpoint",
			endpoint: "/api/reports/general-ledger",
			method:   "GET",
			params: map[string]string{
				"start_date": "2024-01-01",
				"end_date":   "2024-01-31",
				"format":     "json",
			},
		},
		{
			name:     "Sales Summary Direct Endpoint",
			endpoint: "/api/reports/sales-summary",
			method:   "GET",
			params: map[string]string{
				"start_date": "2024-01-01",
				"end_date":   "2024-01-31",
				"format":     "json",
			},
		},
		{
			name:     "Vendor Analysis Direct Endpoint",
			endpoint: "/api/reports/vendor-analysis",
			method:   "GET",
			params: map[string]string{
				"start_date": "2024-01-01",
				"end_date":   "2024-01-31",
				"format":     "json",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Build URL with parameters
			url := tc.endpoint
			if len(tc.params) > 0 {
				url += "?"
				first := true
				for k, v := range tc.params {
					if !first {
						url += "&"
					}
					url += fmt.Sprintf("%s=%s", k, v)
					first = false
				}
			}

			req := httptest.NewRequest(tc.method, url, nil)
			req.Header.Set("Authorization", "Bearer test-token")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should not return 404 (endpoint exists)
			assert.NotEqual(t, http.StatusNotFound, w.Code, "Endpoint should exist")
			
			// Should have proper response structure
			if w.Code == http.StatusOK {
				var response models.StandardReportResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err, "Response should be valid JSON")
				assert.NotNil(t, response.Metadata, "Response should have metadata")
			}
		})
	}
}

// Test 2: Preview Endpoints
func TestPreviewEndpoints(t *testing.T) {
	router, _ := setupTestRouter()

	testCases := []struct {
		reportType string
		params     map[string]string
	}{
		{
			reportType: "balance-sheet",
			params:     map[string]string{"format": "json"},
		},
		{
			reportType: "profit-loss",
			params: map[string]string{
				"start_date": "2024-01-01",
				"end_date":   "2024-01-31",
				"format":     "json",
			},
		},
		{
			reportType: "trial-balance",
			params:     map[string]string{"format": "json"},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Preview %s", tc.reportType), func(t *testing.T) {
			// Build URL with parameters
			url := fmt.Sprintf("/api/reports/preview/%s", tc.reportType)
			if len(tc.params) > 0 {
				url += "?"
				first := true
				for k, v := range tc.params {
					if !first {
						url += "&"
					}
					url += fmt.Sprintf("%s=%s", k, v)
					first = false
				}
			}

			req := httptest.NewRequest("GET", url, nil)
			req.Header.Set("Authorization", "Bearer test-token")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should not return 404
			assert.NotEqual(t, http.StatusNotFound, w.Code, "Preview endpoint should exist")
			
			// Should force JSON format for preview
			if w.Code == http.StatusOK {
				var response models.StandardReportResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err, "Preview should return JSON")
				assert.Equal(t, "json", response.Metadata.Format, "Preview should be JSON format")
			}
		})
	}
}

// Test 3: Standard Response Structure
func TestStandardResponseStructure(t *testing.T) {
	router, _ := setupTestRouter()

	req := httptest.NewRequest("GET", "/api/reports/balance-sheet?format=json", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		var response models.StandardReportResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err, "Response should be valid JSON")

		// Check standard response structure
		assert.True(t, response.Success, "Response should indicate success")
		assert.NotNil(t, response.Data, "Response should have data")
		assert.NotNil(t, response.Metadata, "Response should have metadata")
		assert.NotZero(t, response.Timestamp, "Response should have timestamp")

		// Check metadata structure
		assert.NotEmpty(t, response.Metadata.ReportType, "Metadata should have report type")
		assert.NotZero(t, response.Metadata.GeneratedAt, "Metadata should have generated at")
		assert.NotEmpty(t, response.Metadata.GeneratedBy, "Metadata should have generated by")
		assert.NotEmpty(t, response.Metadata.Version, "Metadata should have version")
		assert.Equal(t, "json", response.Metadata.Format, "Metadata should indicate format")
	}
}

// Test 4: Error Handling
func TestErrorHandling(t *testing.T) {
	router, _ := setupTestRouter()

	testCases := []struct {
		name         string
		endpoint     string
		expectedCode int
		description  string
	}{
		{
			name:         "Invalid Report Type",
			endpoint:     "/api/reports/invalid-report",
			expectedCode: http.StatusBadRequest,
			description:  "Should return 400 for invalid report types",
		},
		{
			name:         "Missing Required Parameters",
			endpoint:     "/api/reports/profit-loss", // Missing start_date and end_date
			expectedCode: http.StatusBadRequest,
			description:  "Should return 400 for missing required parameters",
		},
		{
			name:         "Invalid Date Format",
			endpoint:     "/api/reports/profit-loss?start_date=invalid&end_date=2024-01-31",
			expectedCode: http.StatusBadRequest,
			description:  "Should return 400 for invalid date format",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.endpoint, nil)
			req.Header.Set("Authorization", "Bearer test-token")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedCode, w.Code, tc.description)

			if w.Code >= 400 {
				var response models.StandardReportResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err, "Error response should be valid JSON")
				assert.False(t, response.Success, "Error response should indicate failure")
				assert.NotNil(t, response.Error, "Error response should have error details")
				assert.NotEmpty(t, response.Error.Code, "Error should have code")
				assert.NotEmpty(t, response.Error.Message, "Error should have message")
			}
		})
	}
}

// Test 5: Parameter Validation
func TestParameterValidation(t *testing.T) {
	router, _ := setupTestRouter()

	testCases := []struct {
		name        string
		endpoint    string
		params      map[string]string
		shouldPass  bool
		description string
	}{
		{
			name:     "Valid Balance Sheet Parameters",
			endpoint: "/api/reports/balance-sheet",
			params:   map[string]string{"as_of_date": "2024-01-31", "format": "json"},
			shouldPass: true,
			description: "Should accept valid balance sheet parameters",
		},
		{
			name:     "Valid P&L Parameters",
			endpoint: "/api/reports/profit-loss",
			params: map[string]string{
				"start_date": "2024-01-01",
				"end_date":   "2024-01-31",
				"format":     "pdf",
			},
			shouldPass: true,
			description: "Should accept valid P&L parameters",
		},
		{
			name:     "Invalid Date Range",
			endpoint: "/api/reports/profit-loss",
			params: map[string]string{
				"start_date": "2024-01-31",
				"end_date":   "2024-01-01", // End before start
				"format":     "json",
			},
			shouldPass: false,
			description: "Should reject invalid date ranges",
		},
		{
			name:     "Valid Group By Parameter",
			endpoint: "/api/reports/sales-summary",
			params: map[string]string{
				"start_date": "2024-01-01",
				"end_date":   "2024-01-31",
				"group_by":   "month",
				"format":     "json",
			},
			shouldPass: true,
			description: "Should accept valid group_by values",
		},
		{
			name:     "Invalid Group By Parameter",
			endpoint: "/api/reports/sales-summary",
			params: map[string]string{
				"start_date": "2024-01-01",
				"end_date":   "2024-01-31",
				"group_by":   "invalid",
				"format":     "json",
			},
			shouldPass: false,
			description: "Should reject invalid group_by values",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Build URL with parameters
			url := tc.endpoint
			if len(tc.params) > 0 {
				url += "?"
				first := true
				for k, v := range tc.params {
					if !first {
						url += "&"
					}
					url += fmt.Sprintf("%s=%s", k, v)
					first = false
				}
			}

			req := httptest.NewRequest("GET", url, nil)
			req.Header.Set("Authorization", "Bearer test-token")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if tc.shouldPass {
				assert.NotEqual(t, http.StatusBadRequest, w.Code, tc.description)
			} else {
				assert.Equal(t, http.StatusBadRequest, w.Code, tc.description)
			}
		})
	}
}

// Test 6: Available Reports Endpoint
func TestAvailableReportsEndpoint(t *testing.T) {
	router, _ := setupTestRouter()

	req := httptest.NewRequest("GET", "/api/reports/available", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Available reports endpoint should work")

	var response models.StandardReportResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err, "Response should be valid JSON")
	assert.True(t, response.Success, "Response should indicate success")

	reports, ok := response.Data.([]interface{})
	assert.True(t, ok, "Data should be an array of reports")
	assert.Greater(t, len(reports), 0, "Should return at least one report")

	// Check first report structure
	if len(reports) > 0 {
		report, ok := reports[0].(map[string]interface{})
		assert.True(t, ok, "Report should be an object")
		assert.Contains(t, report, "id", "Report should have ID")
		assert.Contains(t, report, "name", "Report should have name")
		assert.Contains(t, report, "description", "Report should have description")
		assert.Contains(t, report, "type", "Report should have type")
		assert.Contains(t, report, "parameters", "Report should have parameters")
		assert.Contains(t, report, "endpoints", "Report should have endpoints")
	}
}

// Test 7: Different Output Formats
func TestOutputFormats(t *testing.T) {
	router, _ := setupTestRouter()

	testCases := []struct {
		format      string
		contentType string
		description string
	}{
		{
			format:      "json",
			contentType: "application/json",
			description: "Should return JSON response",
		},
		{
			format:      "pdf",
			contentType: "application/pdf",
			description: "Should return PDF file",
		},
		{
			format:      "excel",
			contentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			description: "Should return Excel file",
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Format %s", tc.format), func(t *testing.T) {
			url := fmt.Sprintf("/api/reports/balance-sheet?format=%s", tc.format)
			req := httptest.NewRequest("GET", url, nil)
			req.Header.Set("Authorization", "Bearer test-token")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				if tc.format == "json" {
					// Should be JSON response
					var response models.StandardReportResponse
					err := json.Unmarshal(w.Body.Bytes(), &response)
					assert.NoError(t, err, tc.description)
				} else {
					// Should be file response
					contentType := w.Header().Get("Content-Type")
					assert.Equal(t, tc.contentType, contentType, tc.description)
					
					disposition := w.Header().Get("Content-Disposition")
					assert.Contains(t, disposition, "attachment", "Should be downloadable file")
				}
			}
		})
	}
}

// Test 8: Authentication and Authorization
func TestAuthenticationAndAuthorization(t *testing.T) {
	router, _ := setupTestRouter()

	testCases := []struct {
		name        string
		token       string
		expectedCode int
		description string
	}{
		{
			name:        "No Token",
			token:       "",
			expectedCode: http.StatusUnauthorized,
			description: "Should require authentication",
		},
		{
			name:        "Invalid Token",
			token:       "Bearer invalid-token",
			expectedCode: http.StatusUnauthorized,
			description: "Should reject invalid tokens",
		},
		{
			name:        "Valid Token",
			token:       "Bearer test-token",
			expectedCode: http.StatusOK,
			description: "Should accept valid tokens",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/reports/balance-sheet?format=json", nil)
			if tc.token != "" {
				req.Header.Set("Authorization", tc.token)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedCode, w.Code, tc.description)
		})
	}
}

// Test 9: Response Headers
func TestResponseHeaders(t *testing.T) {
	router, _ := setupTestRouter()

	req := httptest.NewRequest("GET", "/api/reports/balance-sheet?format=json", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Check for custom headers
	assert.Equal(t, "2.0", w.Header().Get("X-Report-API-Version"), "Should have API version header")
	assert.Equal(t, "unified", w.Header().Get("X-Report-System"), "Should have system identifier header")
}

// Test 10: Performance and Timing
func TestReportGenerationTiming(t *testing.T) {
	router, _ := setupTestRouter()

	start := time.Now()
	req := httptest.NewRequest("GET", "/api/reports/balance-sheet?format=json", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	duration := time.Since(start)

	// Should complete within reasonable time (adjust as needed)
	assert.Less(t, duration, 5*time.Second, "Report generation should complete within 5 seconds")

	if w.Code == http.StatusOK {
		var response models.StandardReportResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		
		// Should have generation time in metadata
		assert.NotEmpty(t, response.Metadata.GenerationTime, "Should track generation time")
	}
}

// Benchmark tests
func BenchmarkBalanceSheetGeneration(b *testing.B) {
	router, _ := setupTestRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/api/reports/balance-sheet?format=json", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

func BenchmarkProfitLossGeneration(b *testing.B) {
	router, _ := setupTestRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/api/reports/profit-loss?start_date=2024-01-01&end_date=2024-01-31&format=json", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// Integration test helper
func TestMain(m *testing.M) {
	// Setup test database, mock services, etc.
	// Run tests
	m.Run()
}
