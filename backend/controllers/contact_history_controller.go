package controllers

import (
	"net/http"
	"strconv"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ContactHistoryController struct {
	db         *gorm.DB
	pdfService services.PDFServiceInterface
}

func NewContactHistoryController(db *gorm.DB, pdfService services.PDFServiceInterface) *ContactHistoryController {
	return &ContactHistoryController{
		db:         db,
		pdfService: pdfService,
	}
}

// CustomerHistoryData represents customer transaction history
type CustomerHistoryData struct {
	Company      *models.CompanyInfo  `json:"company"`
	Customer     *CustomerInfo        `json:"customer"`
	StartDate    string               `json:"start_date"`
	EndDate      string               `json:"end_date"`
	Transactions []TransactionHistory `json:"transactions"`
	Summary      HistorySummary       `json:"summary"`
	Currency     string               `json:"currency"`
	GeneratedAt  time.Time            `json:"generated_at"`
}

// VendorHistoryData represents vendor transaction history
type VendorHistoryData struct {
	Company      *models.CompanyInfo  `json:"company"`
	Vendor       *VendorInfo          `json:"vendor"`
	StartDate    string               `json:"start_date"`
	EndDate      string               `json:"end_date"`
	Transactions []TransactionHistory `json:"transactions"`
	Summary      HistorySummary       `json:"summary"`
	Currency     string               `json:"currency"`
	GeneratedAt  time.Time            `json:"generated_at"`
}

type CustomerInfo struct {
	ID          uint    `json:"id"`
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Email       string  `json:"email"`
	Phone       string  `json:"phone"`
	CreditLimit float64 `json:"credit_limit"`
	Address     string  `json:"address"`
}

type VendorInfo struct {
	ID      uint   `json:"id"`
	Code    string `json:"code"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Address string `json:"address"`
}

type TransactionHistory struct {
	TransactionID   uint       `json:"transaction_id"`
	Date            time.Time  `json:"date"`
	TransactionType string     `json:"transaction_type"` // SALE, INVOICE, PAYMENT, PURCHASE, BILL
	TransactionCode string     `json:"transaction_code"`
	Description     string     `json:"description"`
	Reference       string     `json:"reference"`
	Amount          float64    `json:"amount"`
	PaidAmount      float64    `json:"paid_amount"`
	Outstanding     float64    `json:"outstanding"`
	Status          string     `json:"status"`
	DueDate         *time.Time `json:"due_date,omitempty"`
}

type HistorySummary struct {
	TotalTransactions int        `json:"total_transactions"`
	TotalAmount       float64    `json:"total_amount"`
	TotalPaid         float64    `json:"total_paid"`
	TotalOutstanding  float64    `json:"total_outstanding"`
	FirstTransaction  *time.Time `json:"first_transaction,omitempty"`
	LastTransaction   *time.Time `json:"last_transaction,omitempty"`
}

// GetCustomerHistory generates customer transaction history report
// @Summary Get customer transaction history
// @Description Generate comprehensive customer transaction history including sales and payments
// @Tags Contact History
// @Produce json
// @Param customer_id query string true "Customer ID"
// @Param start_date query string true "Start date (YYYY-MM-DD)"
// @Param end_date query string true "End date (YYYY-MM-DD)"
// @Param format query string false "Output format (json, pdf, csv)" default(json)
// @Success 200 {object} CustomerHistoryData
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/reports/customer-history [get]
func (c *ContactHistoryController) GetCustomerHistory(ctx *gin.Context) {
	customerIDStr := ctx.Query("customer_id")
	startDateStr := ctx.Query("start_date")
	endDateStr := ctx.Query("end_date")
	format := ctx.DefaultQuery("format", "json")

	// Validate required parameters
	if customerIDStr == "" || startDateStr == "" || endDateStr == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "customer_id, start_date, and end_date are required",
		})
		return
	}

	// Parse customer ID
	customerID, err := strconv.ParseUint(customerIDStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid customer_id format",
		})
		return
	}

	// Parse dates
	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid start_date format. Use YYYY-MM-DD",
		})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid end_date format. Use YYYY-MM-DD",
		})
		return
	}

	// Get customer info
	var customer models.Contact
	if err := c.db.Where("id = ? AND type = ?", customerID, models.ContactTypeCustomer).First(&customer).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, gin.H{
				"status":  "error",
				"message": "Customer not found",
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to retrieve customer",
			"error":   err.Error(),
		})
		return
	}

	// Generate customer history data
	historyData, err := c.generateCustomerHistoryData(uint(customerID), startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to generate customer history",
			"error":   err.Error(),
		})
		return
	}

	// Add customer info
	historyData.Customer = &CustomerInfo{
		ID:          customer.ID,
		Code:        customer.Code,
		Name:        customer.Name,
		Email:       customer.Email,
		Phone:       customer.Phone,
		CreditLimit: customer.CreditLimit,
		Address:     customer.Address,
	}

	// Handle different formats
	switch format {
	case "json":
		ctx.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data":   historyData,
		})
	case "pdf":
		pdfBytes, err := c.pdfService.GenerateCustomerHistoryPDF(historyData)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to generate PDF",
				"error":   err.Error(),
			})
			return
		}
		ctx.Header("Content-Type", "application/pdf")
		ctx.Header("Content-Disposition", "attachment; filename=Customer_History_"+customer.Name+".pdf")
		ctx.Header("Content-Length", strconv.Itoa(len(pdfBytes)))
		ctx.Data(http.StatusOK, "application/pdf", pdfBytes)
	case "csv":
		csvBytes, err := c.pdfService.GenerateCustomerHistoryCSV(historyData)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to generate CSV",
				"error":   err.Error(),
			})
			return
		}
		ctx.Header("Content-Type", "text/csv")
		ctx.Header("Content-Disposition", "attachment; filename=Customer_History_"+customer.Name+".csv")
		ctx.Header("Content-Length", strconv.Itoa(len(csvBytes)))
		ctx.Data(http.StatusOK, "text/csv", csvBytes)
	default:
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Unsupported format. Use json, pdf, or csv",
		})
	}
}

// GetVendorHistory generates vendor transaction history report
// @Summary Get vendor transaction history
// @Description Generate comprehensive vendor transaction history including purchases and payments
// @Tags Contact History
// @Produce json
// @Param vendor_id query string true "Vendor ID"
// @Param start_date query string true "Start date (YYYY-MM-DD)"
// @Param end_date query string true "End date (YYYY-MM-DD)"
// @Param format query string false "Output format (json, pdf, csv)" default(json)
// @Success 200 {object} VendorHistoryData
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/reports/vendor-history [get]
func (c *ContactHistoryController) GetVendorHistory(ctx *gin.Context) {
	vendorIDStr := ctx.Query("vendor_id")
	startDateStr := ctx.Query("start_date")
	endDateStr := ctx.Query("end_date")
	format := ctx.DefaultQuery("format", "json")

	// Validate required parameters
	if vendorIDStr == "" || startDateStr == "" || endDateStr == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "vendor_id, start_date, and end_date are required",
		})
		return
	}

	// Parse vendor ID
	vendorID, err := strconv.ParseUint(vendorIDStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid vendor_id format",
		})
		return
	}

	// Parse dates
	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid start_date format. Use YYYY-MM-DD",
		})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid end_date format. Use YYYY-MM-DD",
		})
		return
	}

	// Get vendor info
	var vendor models.Contact
	if err := c.db.Where("id = ? AND type = ?", vendorID, models.ContactTypeVendor).First(&vendor).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, gin.H{
				"status":  "error",
				"message": "Vendor not found",
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to retrieve vendor",
			"error":   err.Error(),
		})
		return
	}

	// Generate vendor history data
	historyData, err := c.generateVendorHistoryData(uint(vendorID), startDate, endDate)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to generate vendor history",
			"error":   err.Error(),
		})
		return
	}

	// Add vendor info
	historyData.Vendor = &VendorInfo{
		ID:      vendor.ID,
		Code:    vendor.Code,
		Name:    vendor.Name,
		Email:   vendor.Email,
		Phone:   vendor.Phone,
		Address: vendor.Address,
	}

	// Handle different formats
	switch format {
	case "json":
		ctx.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data":   historyData,
		})
	case "pdf":
		pdfBytes, err := c.pdfService.GenerateVendorHistoryPDF(historyData)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to generate PDF",
				"error":   err.Error(),
			})
			return
		}
		ctx.Header("Content-Type", "application/pdf")
		ctx.Header("Content-Disposition", "attachment; filename=Vendor_History_"+vendor.Name+".pdf")
		ctx.Header("Content-Length", strconv.Itoa(len(pdfBytes)))
		ctx.Data(http.StatusOK, "application/pdf", pdfBytes)
	case "csv":
		csvBytes, err := c.pdfService.GenerateVendorHistoryCSV(historyData)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to generate CSV",
				"error":   err.Error(),
			})
			return
		}
		ctx.Header("Content-Type", "text/csv")
		ctx.Header("Content-Disposition", "attachment; filename=Vendor_History_"+vendor.Name+".csv")
		ctx.Header("Content-Length", strconv.Itoa(len(csvBytes)))
		ctx.Data(http.StatusOK, "text/csv", csvBytes)
	default:
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Unsupported format. Use json, pdf, or csv",
		})
	}
}

// generateCustomerHistoryData generates transaction history for a customer
func (c *ContactHistoryController) generateCustomerHistoryData(customerID uint, startDate, endDate time.Time) (*CustomerHistoryData, error) {
	var transactions []TransactionHistory
	var summary HistorySummary

	// Get company info
	var settings models.Settings
	c.db.First(&settings)

	company := &models.CompanyInfo{
		Name:    settings.CompanyName,
		Address: settings.CompanyAddress,
		City:    "", // City not stored separately in Settings
		Phone:   settings.CompanyPhone,
		Email:   settings.CompanyEmail,
	}

	// Get sales transactions with InvoiceType preloaded
	var sales []models.Sale
	if err := c.db.Preload("InvoiceType").Where("customer_id = ? AND date BETWEEN ? AND ?", customerID, startDate, endDate).
		Order("date DESC").Find(&sales).Error; err != nil {
		return nil, err
	}

	for _, sale := range sales {
		// Build description with invoice type if available
		description := "Sales Transaction"
		if sale.InvoiceType != nil && sale.InvoiceType.Name != "" {
			description = "Sales Transaction for " + sale.InvoiceType.Name
		}

		transaction := TransactionHistory{
			TransactionID:   sale.ID,
			Date:            sale.Date,
			TransactionType: "SALE",
			TransactionCode: sale.Code,
			Description:     description,
			Reference:       sale.Reference,
			Amount:          sale.TotalAmount,
			PaidAmount:      sale.PaidAmount,
			Outstanding:     sale.TotalAmount - sale.PaidAmount,
			Status:          sale.Status,
			DueDate:         &sale.DueDate,
		}
		transactions = append(transactions, transaction)

		summary.TotalAmount += sale.TotalAmount
		summary.TotalPaid += sale.PaidAmount
		summary.TotalOutstanding += (sale.TotalAmount - sale.PaidAmount)
	}

	// Get invoices (optional - skip if table doesn't exist)
	var invoices []models.Invoice
	if err := c.db.Where("customer_id = ? AND date BETWEEN ? AND ?",
		customerID, startDate, endDate).
		Order("date DESC").Find(&invoices).Error; err != nil {
		// If invoices table doesn't exist, skip it (not critical)
		// In most cases, sales table already contains this data
		if err.Error() != "ERROR: relation \"invoices\" does not exist (SQLSTATE 42P01)" {
			// Only return error if it's not a "table doesn't exist" error
			return nil, err
		}
		// Log warning but continue
		// Note: Invoices are typically managed through Sales module
	} else {
		// Process invoices only if table exists and query succeeds
		for _, invoice := range invoices {
			transaction := TransactionHistory{
				TransactionID:   invoice.ID,
				Date:            invoice.Date,
				TransactionType: "INVOICE",
				TransactionCode: invoice.Code,
				Description:     "Customer Invoice",
				Reference:       invoice.PaymentReference,
				Amount:          invoice.TotalAmount,
				PaidAmount:      invoice.PaidAmount,
				Outstanding:     invoice.TotalAmount - invoice.PaidAmount,
				Status:          invoice.Status,
				DueDate:         &invoice.DueDate,
			}
			transactions = append(transactions, transaction)
		}
	}

	// Get payments
	var payments []models.Payment
	if err := c.db.Where("contact_id = ? AND date BETWEEN ? AND ?", customerID, startDate, endDate).
		Order("date DESC").Find(&payments).Error; err != nil {
		return nil, err
	}

	for _, payment := range payments {
		transaction := TransactionHistory{
			TransactionID:   payment.ID,
			Date:            payment.Date,
			TransactionType: "PAYMENT",
			TransactionCode: payment.Code,
			Description:     payment.Notes,
			Reference:       payment.Reference,
			Amount:          payment.Amount,
			PaidAmount:      payment.Amount,
			Outstanding:     0,
			Status:          payment.Status,
		}
		transactions = append(transactions, transaction)

		// Check if payment is unallocated (advance payment)
		var allocatedAmount float64
		c.db.Model(&models.PaymentAllocation{}).
			Where("payment_id = ?", payment.ID).
			Select("COALESCE(SUM(allocated_amount), 0)").
			Scan(&allocatedAmount)

		unallocatedAmount := payment.Amount - allocatedAmount

		// Only add unallocated payments to TotalPaid
		// Allocated payments are already counted in sale.PaidAmount
		if unallocatedAmount > 0 {
			summary.TotalPaid += unallocatedAmount
			// Unallocated payments don't affect outstanding since they're not assigned to any sale
		}
	}

	summary.TotalTransactions = len(transactions)

	// Find first and last transaction dates
	if len(transactions) > 0 {
		// Transactions are already sorted by date DESC, so last is first, first is last
		summary.LastTransaction = &transactions[0].Date
		summary.FirstTransaction = &transactions[len(transactions)-1].Date
	}

	return &CustomerHistoryData{
		Company:      company,
		StartDate:    startDate.Format("2006-01-02"),
		EndDate:      endDate.Format("2006-01-02"),
		Transactions: transactions,
		Summary:      summary,
		Currency:     "IDR",
		GeneratedAt:  time.Now(),
	}, nil
}

// generateVendorHistoryData generates transaction history for a vendor
func (c *ContactHistoryController) generateVendorHistoryData(vendorID uint, startDate, endDate time.Time) (*VendorHistoryData, error) {
	var transactions []TransactionHistory
	var summary HistorySummary

	// Get company info
	var settings models.Settings
	c.db.First(&settings)

	company := &models.CompanyInfo{
		Name:    settings.CompanyName,
		Address: settings.CompanyAddress,
		City:    "", // City not stored separately in Settings
		Phone:   settings.CompanyPhone,
		Email:   settings.CompanyEmail,
	}

	// Get purchase transactions
	var purchases []models.Purchase
	if err := c.db.Where("vendor_id = ? AND date BETWEEN ? AND ?", vendorID, startDate, endDate).
		Order("date DESC").Find(&purchases).Error; err != nil {
		return nil, err
	}

	for _, purchase := range purchases {
		transaction := TransactionHistory{
			TransactionID:   purchase.ID,
			Date:            purchase.Date,
			TransactionType: "PURCHASE",
			TransactionCode: purchase.Code,
			Description:     "Purchase Transaction",
			Reference:       purchase.PaymentReference,
			Amount:          purchase.TotalAmount,
			PaidAmount:      purchase.PaidAmount,
			Outstanding:     purchase.TotalAmount - purchase.PaidAmount,
			Status:          purchase.Status,
			DueDate:         &purchase.DueDate,
		}
		transactions = append(transactions, transaction)

		summary.TotalAmount += purchase.TotalAmount
		summary.TotalPaid += purchase.PaidAmount
		summary.TotalOutstanding += (purchase.TotalAmount - purchase.PaidAmount)
	}

	// Note: Bills for vendors are not stored separately in Invoice table
	// They are tracked through Purchase model

	// Get payments
	var payments []models.Payment
	if err := c.db.Where("contact_id = ? AND date BETWEEN ? AND ?", vendorID, startDate, endDate).
		Order("date DESC").Find(&payments).Error; err != nil {
		return nil, err
	}

	for _, payment := range payments {
		transaction := TransactionHistory{
			TransactionID:   payment.ID,
			Date:            payment.Date,
			TransactionType: "PAYMENT",
			TransactionCode: payment.Code,
			Description:     payment.Notes,
			Reference:       payment.Reference,
			Amount:          payment.Amount,
			PaidAmount:      payment.Amount,
			Outstanding:     0,
			Status:          payment.Status,
		}
		transactions = append(transactions, transaction)

		// Check if payment is unallocated (advance payment)
		var allocatedAmount float64
		c.db.Model(&models.PaymentAllocation{}).
			Where("payment_id = ?", payment.ID).
			Select("COALESCE(SUM(allocated_amount), 0)").
			Scan(&allocatedAmount)

		unallocatedAmount := payment.Amount - allocatedAmount

		// Only add unallocated payments to TotalPaid
		// Allocated payments are already counted in purchase.PaidAmount
		if unallocatedAmount > 0 {
			summary.TotalPaid += unallocatedAmount
			// Unallocated payments don't affect outstanding since they're not assigned to any purchase
		}
	}

	summary.TotalTransactions = len(transactions)

	// Find first and last transaction dates
	if len(transactions) > 0 {
		summary.LastTransaction = &transactions[0].Date
		summary.FirstTransaction = &transactions[len(transactions)-1].Date
	}

	return &VendorHistoryData{
		Company:      company,
		StartDate:    startDate.Format("2006-01-02"),
		EndDate:      endDate.Format("2006-01-02"),
		Transactions: transactions,
		Summary:      summary,
		Currency:     "IDR",
		GeneratedAt:  time.Now(),
	}, nil
}

// GetTransactionJournalDetail returns journal entry details for a specific transaction
// @Summary Get journal detail for a transaction
// @Description Look up SSOT journal entries linked to a specific source transaction (SALE, PURCHASE, PAYMENT)
// @Tags Contact History
// @Produce json
// @Param source_type query string true "Source type (SALE, PURCHASE, PAYMENT, INVOICE)"
// @Param source_id query string true "Source transaction ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/reports/transaction-journal-detail [get]
func (c *ContactHistoryController) GetTransactionJournalDetail(ctx *gin.Context) {
	sourceType := ctx.Query("source_type")
	sourceIDStr := ctx.Query("source_id")

	if sourceType == "" || sourceIDStr == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "source_type and source_id are required",
		})
		return
	}

	sourceID, err := strconv.ParseUint(sourceIDStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid source_id format",
		})
		return
	}

	// Response types
	type JournalLineDetail struct {
		AccountCode string  `json:"account_code"`
		AccountName string  `json:"account_name"`
		Description string  `json:"description"`
		Debit       float64 `json:"debit"`
		Credit      float64 `json:"credit"`
	}
	type JournalDetail struct {
		ID          uint64              `json:"id"`
		EntryNumber string              `json:"entry_number"`
		Date        string              `json:"date"`
		Description string              `json:"description"`
		Reference   string              `json:"reference"`
		Status      string              `json:"status"`
		TotalDebit  float64             `json:"total_debit"`
		TotalCredit float64             `json:"total_credit"`
		IsBalanced  bool                `json:"is_balanced"`
		Lines       []JournalLineDetail `json:"lines"`
	}

	// Look up journal entries from unified_journal_ledger
	var journals []models.SSOTJournalEntry
	if err := c.db.Where("source_type = ? AND source_id = ? AND deleted_at IS NULL", sourceType, sourceID).
		Preload("Lines", func(db *gorm.DB) *gorm.DB {
			return db.Order("line_number ASC")
		}).
		Preload("Lines.Account").
		Order("entry_date DESC").
		Find(&journals).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to retrieve journal details",
			"error":   err.Error(),
		})
		return
	}

	if len(journals) > 0 {
		var result []JournalDetail
		for _, j := range journals {
			jd := JournalDetail{
				ID:          j.ID,
				EntryNumber: j.EntryNumber,
				Date:        j.EntryDate.Format("2006-01-02"),
				Description: j.Description,
				Reference:   j.Reference,
				Status:      j.Status,
				TotalDebit:  j.TotalDebit.InexactFloat64(),
				TotalCredit: j.TotalCredit.InexactFloat64(),
				IsBalanced:  j.IsBalanced,
			}
			for _, line := range j.Lines {
				accCode := ""
				accName := ""
				if line.Account != nil {
					accCode = line.Account.Code
					accName = line.Account.Name
				}
				jd.Lines = append(jd.Lines, JournalLineDetail{
					AccountCode: accCode,
					AccountName: accName,
					Description: line.Description,
					Debit:       line.DebitAmount.InexactFloat64(),
					Credit:      line.CreditAmount.InexactFloat64(),
				})
			}
			result = append(result, jd)
		}
		ctx.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data": gin.H{
				"source_type": sourceType,
				"source_id":   sourceID,
				"journals":    result,
			},
		})
		return
	}

	// No journal entries found
	ctx.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"source_type": sourceType,
			"source_id":   sourceID,
			"journals":    []interface{}{},
			"message":     "No journal entries found for this transaction",
		},
	})
}
