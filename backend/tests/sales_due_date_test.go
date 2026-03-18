//go:build legacytests
// +build legacytests

package tests

import (
	"app-sistem-akuntansi/services"
	"testing"
	"time"
)

// TestDueDateCalculation tests various due date calculation scenarios
func TestDueDateCalculation(t *testing.T) {
	// Create a mock sales service for testing
	salesService := &services.SalesService{}
	
	// Test cases
	testCases := []struct {
		name         string
		saleDate     time.Time
		paymentTerms string
		expectedDays int
		description  string
	}{
		{
			name:         "COD Payment",
			saleDate:     time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			paymentTerms: "COD",
			expectedDays: 0,
			description:  "Cash on Delivery should have same day payment",
		},
		{
			name:         "NET15 Payment",
			saleDate:     time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			paymentTerms: "NET15",
			expectedDays: 15,
			description:  "NET15 should add 15 days to invoice date",
		},
		{
			name:         "NET30 Payment",
			saleDate:     time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			paymentTerms: "NET30",
			expectedDays: 30,
			description:  "NET30 should add 30 days to invoice date",
		},
		{
			name:         "NET45 Payment",
			saleDate:     time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			paymentTerms: "NET45",
			expectedDays: 45,
			description:  "NET45 should add 45 days to invoice date",
		},
		{
			name:         "NET60 Payment",
			saleDate:     time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			paymentTerms: "NET60",
			expectedDays: 60,
			description:  "NET60 should add 60 days to invoice date",
		},
		{
			name:         "NET90 Payment",
			saleDate:     time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			paymentTerms: "NET90",
			expectedDays: 90,
			description:  "NET90 should add 90 days to invoice date",
		},
		{
			name:         "Unknown Payment Terms Default",
			saleDate:     time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			paymentTerms: "UNKNOWN",
			expectedDays: 30,
			description:  "Unknown payment terms should default to NET30",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call the calculateDueDate function
			actualDueDate := salesService.CalculateDueDate(tc.saleDate, tc.paymentTerms)
			
			// Calculate expected due date
			expectedDueDate := tc.saleDate.AddDate(0, 0, tc.expectedDays)
			
			// Compare the dates
			if !actualDueDate.Equal(expectedDueDate) {
				t.Errorf("%s: Expected due date %v, but got %v. %s", 
					tc.name, expectedDueDate, actualDueDate, tc.description)
			}
		})
	}
}

// TestEOMDueDateCalculation tests End of Month payment term calculation
func TestEOMDueDateCalculation(t *testing.T) {
	salesService := &services.SalesService{}
	
	testCases := []struct {
		name             string
		saleDate         time.Time
		expectedDueDate  time.Time
		description      string
	}{
		{
			name:             "EOM January",
			saleDate:         time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			expectedDueDate:  time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
			description:      "January 15th invoice should be due January 31st",
		},
		{
			name:             "EOM February (Non-leap year)",
			saleDate:         time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC),
			expectedDueDate:  time.Date(2025, 2, 28, 0, 0, 0, 0, time.UTC),
			description:      "February 15th invoice should be due February 28th in non-leap year",
		},
		{
			name:             "EOM February (Leap year)",
			saleDate:         time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC),
			expectedDueDate:  time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
			description:      "February 15th invoice should be due February 29th in leap year",
		},
		{
			name:             "EOM December",
			saleDate:         time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC),
			expectedDueDate:  time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
			description:      "December 15th invoice should be due December 31st",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualDueDate := salesService.CalculateDueDate(tc.saleDate, "EOM")
			
			if !actualDueDate.Equal(tc.expectedDueDate) {
				t.Errorf("%s: Expected due date %v, but got %v. %s", 
					tc.name, tc.expectedDueDate, actualDueDate, tc.description)
			}
		})
	}
}

// TestSpecialPaymentTerms tests special payment terms like 2/10, Net 30
func TestSpecialPaymentTerms(t *testing.T) {
	salesService := &services.SalesService{}
	
	saleDate := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	expectedDueDate := time.Date(2025, 2, 14, 0, 0, 0, 0, time.UTC) // 30 days later
	
	actualDueDate := salesService.CalculateDueDate(saleDate, "2_10_NET_30")
	
	if !actualDueDate.Equal(expectedDueDate) {
		t.Errorf("2/10, Net 30: Expected due date %v, but got %v", expectedDueDate, actualDueDate)
	}
}

// TestDateFormatAmbiguity tests that our calculation prevents date format ambiguity
func TestDateFormatAmbiguity(t *testing.T) {
	salesService := &services.SalesService{}
	
	// Test the ambiguous date scenario from the user's example
	// Invoice date: January 10, 2025 (should not be confused with October 1st)
	saleDate := time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)
	
	// NET30 should result in February 9, 2025
	expectedDueDate := time.Date(2025, 2, 9, 0, 0, 0, 0, time.UTC)
	
	actualDueDate := salesService.CalculateDueDate(saleDate, "NET30")
	
	if !actualDueDate.Equal(expectedDueDate) {
		t.Errorf("Date ambiguity test failed: Expected due date %v, but got %v", expectedDueDate, actualDueDate)
	}
	
	// Verify the due date is consistently formatted
	expectedDateString := "2025-02-09"
	actualDateString := actualDueDate.Format("2006-01-02")
	
	if actualDateString != expectedDateString {
		t.Errorf("Date format test failed: Expected %s, but got %s", expectedDateString, actualDateString)
	}
}

// BenchmarkDueDateCalculation benchmarks the due date calculation performance
func BenchmarkDueDateCalculation(b *testing.B) {
	salesService := &services.SalesService{}
	saleDate := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		salesService.CalculateDueDate(saleDate, "NET30")
	}
}
