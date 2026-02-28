package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"app-sistem-akuntansi/services"

	"github.com/gin-gonic/gin"
)

// PeriodValidationMiddleware checks if transaction date is in closed period
type PeriodValidationMiddleware struct {
	periodService *services.UnifiedPeriodClosingService
}

// NewPeriodValidationMiddleware creates a new period validation middleware
func NewPeriodValidationMiddleware(periodService *services.UnifiedPeriodClosingService) *PeriodValidationMiddleware {
	return &PeriodValidationMiddleware{
		periodService: periodService,
	}
}

// ValidateTransactionPeriod checks if the transaction date is in a closed period
func (pvm *PeriodValidationMiddleware) ValidateTransactionPeriod() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip for GET requests only — DELETE must also be blocked in closed periods
		// to maintain financial integrity of closed accounting periods
		if c.Request.Method == "GET" {
			c.Next()
			return
		}

		// For DELETE requests, check the transaction date from the database
		// Since DELETE has no body, we need to let the controller handle validation
		// BUT: we also check query params for date if provided
		if c.Request.Method == "DELETE" {
			// Check if a date query param is provided (some endpoints use this)
			dateStr := c.Query("date")
			if dateStr == "" {
				// No date in query — let it pass to controller which should validate
				// against the existing record's date before deletion
				c.Next()
				return
			}
			if parsedDate, err := parseDate(dateStr); err == nil {
				isClosed, checkErr := pvm.periodService.IsDateInClosedPeriod(c.Request.Context(), parsedDate)
				if checkErr == nil && isClosed {
					periodInfo := pvm.periodService.GetPeriodInfoForDate(c.Request.Context(), parsedDate)
					c.JSON(http.StatusBadRequest, gin.H{
						"success": false,
						"error":   "Periode Akuntansi Sudah Ditutup",
						"message": "Tidak dapat menghapus transaksi pada periode yang sudah ditutup buku.",
						"code":    "PERIOD_CLOSED",
						"details": map[string]interface{}{
							"transaction_date": parsedDate.Format("02 January 2006"),
							"action_required":  "Hubungi administrator untuk membuka kembali periode ini.",
						},
						"period": periodInfo,
					})
					c.Abort()
					return
				}
			}
			c.Next()
			return
		}

		// Get the raw body to check for date fields
		bodyBytes, err := c.GetRawData()
		if err != nil {
			c.Next()
			return
		}

		// Restore body for next handlers so downstream binders can read it again
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		// Parse body to check for date fields
		var requestBody map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &requestBody); err != nil {
			c.Next()
			return
		}

		// Check various date field names used in the system
		dateFields := []string{
			"entry_date",       // Journal entries
			"sale_date",        // Sales
			"purchase_date",    // Purchases
			"payment_date",     // Payments
			"date",             // Generic date field
			"transaction_date", // Alternative naming
		}

		var transactionDate *time.Time
		for _, field := range dateFields {
			if dateStr, ok := requestBody[field].(string); ok && dateStr != "" {
				if parsedDate, err := parseDate(dateStr); err == nil {
					transactionDate = &parsedDate
					break
				}
			}
		}

		// If no date found, let the request proceed (will be validated by controller)
		if transactionDate == nil {
			c.Next()
			return
		}

		// Check if date is in closed period
		isClosed, err := pvm.periodService.IsDateInClosedPeriod(c.Request.Context(), *transactionDate)
		if err != nil {
			// Log error but don't block the request
			c.Next()
			return
		}

		if isClosed {
			// Find the period details for better error message
			periodInfo := pvm.periodService.GetPeriodInfoForDate(c.Request.Context(), *transactionDate)

			// Format date for display
			transactionDateStr := transactionDate.Format("02 January 2006")

			// Build user-friendly error message
			errorTitle := "Periode Akuntansi Sudah Ditutup"
			errorMessage := "Tidak dapat membuat atau mengubah transaksi pada periode yang sudah ditutup buku."

			// Build helpful details
			var periodRange string
			var actionRequired string

			if periodInfo != nil {
				if startDate, ok := periodInfo["start_date"].(time.Time); ok {
					if endDate, ok := periodInfo["end_date"].(time.Time); ok {
						periodRange = startDate.Format("02 Jan 2006") + " - " + endDate.Format("02 Jan 2006")
					}
				}

				// Check if period has closed_by info
				if _, ok := periodInfo["closed_by"].(uint); ok {
					actionRequired = "Hubungi administrator atau finance manager untuk membuka kembali periode ini jika diperlukan."
				} else {
					actionRequired = "Hubungi administrator untuk membuka kembali periode ini."
				}
			} else {
				actionRequired = "Pilih tanggal transaksi yang berada pada periode yang masih terbuka."
			}

			// Return 400 Bad Request (validation error), not 403 Forbidden (authorization error)
			// This is a business rule validation, not a permission check
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   errorTitle,
				"message": errorMessage,
				"code":    "PERIOD_CLOSED",
				"details": map[string]interface{}{
					"transaction_date": transactionDateStr,
					"closed_period":    periodRange,
					"action_required":  actionRequired,
					"iso_date":         transactionDate.Format("2006-01-02"),
				},
				"period": periodInfo,
				"help":   "Periode akuntansi yang sudah ditutup tidak dapat dimodifikasi untuk menjaga integritas laporan keuangan. Gunakan fitur 'Buka Kembali Periode' pada menu Period Closing jika perlu melakukan koreksi.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// parseDate tries to parse date string in various formats
func parseDate(dateStr string) (time.Time, error) {
	// Try common date formats
	formats := []string{
		"2006-01-02",           // ISO date
		"2006-01-02T15:04:05Z", // ISO datetime with Z
		"2006-01-02T15:04:05",  // ISO datetime without timezone
		"2006-01-02 15:04:05",  // Space separator
		time.RFC3339,           // Standard RFC3339
	}

	for _, format := range formats {
		if date, err := time.Parse(format, dateStr); err == nil {
			return date, nil
		}
	}

	return time.Time{}, &time.ParseError{
		Layout:     "various formats",
		Value:      dateStr,
		LayoutElem: "",
		ValueElem:  "",
	}
}
