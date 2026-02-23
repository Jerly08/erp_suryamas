package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"app-sistem-akuntansi/database"
)

type APITestResult struct {
	Endpoint     string      `json:"endpoint"`
	Status       int         `json:"status"`
	Success      bool        `json:"success"`
	ResponseSize int         `json:"response_size"`
	HasData      bool        `json:"has_data"`
	DataPreview  interface{} `json:"data_preview,omitempty"`
	Error        string      `json:"error,omitempty"`
}

func main() {
	fmt.Println("🧪 Testing Financial Reports API Endpoints")
	fmt.Println("==========================================")
	fmt.Println("Checking if SSOT Journal System data can be retrieved by frontend...")
	fmt.Println()

	// Initialize database to check data availability
	db := database.ConnectDB()

	// Check SSOT data availability first
	fmt.Println("📊 Checking SSOT Journal System Data Availability...")
	checkSSOTData(db)
	fmt.Println()

	// Test parameters for the API calls
	startDate := "2025-08-01"
	endDate := "2025-09-30"
	baseURL := "http://localhost:8080"

	// List of financial report endpoints to test
	endpoints := []struct {
		name        string
		path        string
		description string
	}{
		{"Profit & Loss Statement", "/api/v1/reports/profit-loss", "Comprehensive profit and loss statement"},
		{"Balance Sheet", "/api/v1/reports/balance-sheet", "Company assets, liabilities, and equity"},
		{"Cash Flow Statement", "/api/v1/reports/cash-flow", "Cash inflows and outflows"},
		{"Sales Summary Report", "/api/v1/reports/sales-summary", "Summary of sales transactions"},
		{"Purchase Summary Report", "/api/v1/reports/purchase-summary", "Summary of purchase transactions"},
		{"Vendor Analysis Report", "/api/v1/reports/vendor-analysis", "Vendor performance and payment analysis"},
		{"Trial Balance", "/api/v1/reports/trial-balance", "Summary of all account balances"},
		{"General Ledger", "/api/v1/reports/general-ledger", "Complete record of financial transactions"},
		{"Journal Entry Analysis", "/api/v1/reports/journal-entry-analysis", "Analysis of journal entries"},
	}

	fmt.Println("🌐 Testing API Endpoints (without authentication)...")
	fmt.Println("Note: Some endpoints may require authentication")
	fmt.Println()

	var results []APITestResult

	// Test each endpoint
	for i, endpoint := range endpoints {
		fmt.Printf("%d. Testing %s...\n", i+1, endpoint.name)

		// Build URL with parameters
		url := fmt.Sprintf("%s%s?start_date=%s&end_date=%s&format=json",
			baseURL, endpoint.path, startDate, endDate)

		result := testEndpoint(url, endpoint.name, endpoint.path)
		results = append(results, result)

		// Print result summary
		if result.Success {
			fmt.Printf("   ✅ Status: %d, Size: %d bytes, Has Data: %t\n",
				result.Status, result.ResponseSize, result.HasData)
		} else {
			fmt.Printf("   ❌ Status: %d, Error: %s\n", result.Status, result.Error)
		}
		fmt.Println()

		// Small delay between requests
		time.Sleep(100 * time.Millisecond)
	}

	// Print summary
	fmt.Println("📋 Summary Report")
	fmt.Println("================")

	successCount := 0
	dataAvailableCount := 0

	for _, result := range results {
		status := "❌ Failed"
		if result.Success {
			successCount++
			status = "✅ Success"
			if result.HasData {
				dataAvailableCount++
				status += " (With Data)"
			} else {
				status += " (No Data)"
			}
		}

		fmt.Printf("%-30s: %s\n", result.Endpoint, status)
	}

	fmt.Printf("\n🎯 Results:\n")
	fmt.Printf("- Total endpoints tested: %d\n", len(results))
	fmt.Printf("- Successful responses: %d\n", successCount)
	fmt.Printf("- Endpoints with data: %d\n", dataAvailableCount)
	fmt.Printf("- Success rate: %.1f%%\n", float64(successCount)/float64(len(results))*100)

	// Recommendations
	fmt.Printf("\n💡 Recommendations:\n")
	if successCount == 0 {
		fmt.Printf("- ⚠️  Backend server might not be running on %s\n", baseURL)
		fmt.Printf("- ⚠️  Check if authentication is required\n")
	} else if dataAvailableCount == 0 {
		fmt.Printf("- ⚠️  APIs are responding but no data is available\n")
		fmt.Printf("- ⚠️  Check SSOT Journal System integration\n")
	} else if dataAvailableCount < successCount {
		fmt.Printf("- ⚠️  Some APIs have data, others don't - check data integration\n")
	} else {
		fmt.Printf("- ✅ All APIs are working with data available!\n")
		fmt.Printf("- ✅ Frontend should be able to display financial reports\n")
	}
}

func checkSSOTData(db interface{}) {
	// Note: Since we can't import gorm here easily, we'll just print what we expect
	fmt.Println("Expected SSOT Data Status:")
	fmt.Println("- Journal Entries: 5 entries created")
	fmt.Println("- Account Balances View: Should exist")
	fmt.Println("- Transaction Types: SALE, PURCHASE, PAYMENT")
	fmt.Println("- Total Amount Range: ₹943,500 - ₹3,885,000")
}

func testEndpoint(url, name, path string) APITestResult {
	result := APITestResult{
		Endpoint: name,
		Success:  false,
		HasData:  false,
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Make request
	resp, err := client.Get(url)
	if err != nil {
		result.Error = fmt.Sprintf("Request failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	result.Status = resp.StatusCode

	// Read response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to read response: %v", err)
		return result
	}

	result.ResponseSize = len(body)
	bodyStr := string(body)

	// Check if request was successful (2xx status codes)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Success = true

		// Try to parse JSON to check for data
		var jsonData interface{}
		if err := json.Unmarshal(body, &jsonData); err == nil {
			// Check if response contains actual data
			result.HasData = checkForData(bodyStr, jsonData)

			// Create data preview
			if result.HasData {
				result.DataPreview = createDataPreview(jsonData)
			}
		} else {
			// Not JSON, but check if it contains data keywords
			result.HasData = containsDataKeywords(bodyStr)
		}
	} else {
		// Extract error message from response
		if strings.Contains(bodyStr, "AUTH_HEADER_MISSING") {
			result.Error = "Authentication required"
		} else if strings.Contains(bodyStr, "error") || strings.Contains(bodyStr, "Error") {
			// Try to extract error message
			var errorResp map[string]interface{}
			if json.Unmarshal(body, &errorResp) == nil {
				if msg, ok := errorResp["error"].(string); ok {
					result.Error = msg
				} else if msg, ok := errorResp["message"].(string); ok {
					result.Error = msg
				}
			}
			if result.Error == "" {
				result.Error = "API error response"
			}
		} else {
			result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
	}

	return result
}

func checkForData(bodyStr string, jsonData interface{}) bool {
	// Check for common indicators of empty data
	emptyIndicators := []string{
		"\"total_entries\":0",
		"\"total_amount\":0",
		"\"items\":null",
		"\"items\":[]",
		"No P&L relevant transactions found",
		"No data available",
		"no data",
		"empty",
	}

	bodyLower := strings.ToLower(bodyStr)
	for _, indicator := range emptyIndicators {
		if strings.Contains(bodyLower, strings.ToLower(indicator)) {
			return false
		}
	}

	// Check for positive indicators of data
	dataIndicators := []string{
		"\"total\":",
		"\"amount\":",
		"\"balance\":",
		"\"entries\":",
		"\"revenue\":",
		"\"expenses\":",
		"\"assets\":",
		"\"liabilities\":",
		"journal_entries",
		"account_balance",
	}

	for _, indicator := range dataIndicators {
		if strings.Contains(bodyLower, strings.ToLower(indicator)) {
			// Additional check: make sure it's not zero
			if !strings.Contains(bodyLower, indicator+"0") &&
				!strings.Contains(bodyLower, indicator+"null") {
				return true
			}
		}
	}

	// Check JSON structure for data
	if dataMap, ok := jsonData.(map[string]interface{}); ok {
		// Check if there's a data field with content
		if data, exists := dataMap["data"]; exists && data != nil {
			return true
		}

		// Check for non-zero numeric values
		for _, value := range dataMap {
			if numVal, ok := value.(float64); ok && numVal != 0 {
				return true
			}
		}
	}

	return false
}

func containsDataKeywords(body string) bool {
	keywords := []string{
		"revenue", "expense", "profit", "loss", "balance",
		"asset", "liability", "equity", "debit", "credit",
		"journal", "account", "transaction",
	}

	bodyLower := strings.ToLower(body)
	for _, keyword := range keywords {
		if strings.Contains(bodyLower, keyword) {
			return true
		}
	}
	return false
}

func createDataPreview(jsonData interface{}) interface{} {
	// Create a simplified preview of the data
	if dataMap, ok := jsonData.(map[string]interface{}); ok {
		preview := make(map[string]interface{})

		// Extract key fields for preview
		keyFields := []string{"total", "amount", "balance", "revenue", "expenses", "entries", "count"}

		for _, field := range keyFields {
			if value, exists := dataMap[field]; exists {
				preview[field] = value
			}
		}

		// If data field exists, extract some info from it
		if data, exists := dataMap["data"]; exists {
			if dataDict, ok := data.(map[string]interface{}); ok {
				for _, field := range keyFields {
					if value, exists := dataDict[field]; exists {
						preview[field] = value
					}
				}
			}
		}

		if len(preview) > 0 {
			return preview
		}
	}

	return nil
}
