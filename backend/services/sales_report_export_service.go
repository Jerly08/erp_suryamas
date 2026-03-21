package services

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/utils"

	"github.com/jung-kurt/gofpdf"
	"gorm.io/gorm"
)

// SalesReportExportService handles export functionality for Sales Report
type SalesReportExportService struct{ db *gorm.DB }

// NewSalesReportExportService creates a new sales report export service
func NewSalesReportExportService(db *gorm.DB) *SalesReportExportService {
	return &SalesReportExportService{db: db}
}

// getCompanyInfo from settings with defaults
func (s *SalesReportExportService) getCompanyInfo() *models.Settings {
	if s.db == nil {
		return &models.Settings{CompanyName: "PT. Sistem Akuntansi Indonesia"}
	}
	var settings models.Settings
	if err := s.db.First(&settings).Error; err != nil {
		return &models.Settings{CompanyName: "PT. Sistem Akuntansi Indonesia"}
	}
	return &settings
}

// getCurrencyFromSettings gets currency from settings
func (s *SalesReportExportService) getCurrencyFromSettings() string {
	settings := s.getCompanyInfo()
	if settings.Currency != "" {
		return settings.Currency
	}
	return "IDR"
}

// ExportToCSV exports sales report to CSV bytes with localization
func (s *SalesReportExportService) ExportToCSV(data *SalesSummaryData, userID uint) ([]byte, error) {
	// Get user language preference
	language := utils.GetUserLanguageFromSettings(s.db)
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Header with localization
	w.Write([]string{utils.T("sales_report", language)})
	w.Write([]string{data.Company.Name})
	w.Write([]string{utils.T("period", language) + ":", data.StartDate.Format("2006-01-02"), utils.T("to", language), data.EndDate.Format("2006-01-02")})
	w.Write([]string{utils.T("generated_on", language) + ":", time.Now().In(time.Local).Format("2006-01-02 15:04")})
	w.Write([]string{})

	// Summary with localization
	w.Write([]string{utils.T("summary", language)})
	w.Write([]string{"Total Sales (Gross)", fmt.Sprintf("%.2f", data.TotalSales)})
	w.Write([]string{utils.T("total_revenue", language), fmt.Sprintf("%.2f", data.TotalRevenue)})
	w.Write([]string{"Total Tax", fmt.Sprintf("%.2f", data.TotalTax)})
	w.Write([]string{"Total Discount", fmt.Sprintf("%.2f", data.TotalDiscount)})
	w.Write([]string{"Total Returns", fmt.Sprintf("%.2f", data.TotalReturns)})
	w.Write([]string{"Net Revenue", fmt.Sprintf("%.2f", data.NetRevenue)})
	w.Write([]string{utils.T("total_transactions", language), fmt.Sprintf("%d", data.TotalTransactions)})
	w.Write([]string{utils.T("average_order_value", language), fmt.Sprintf("%.2f", data.AverageOrderValue)})
	w.Write([]string{utils.T("total_customers", language), fmt.Sprintf("%d", data.TotalCustomers)})
	w.Write([]string{})

	// Sales by customer (top 20)
	if len(data.SalesByCustomer) > 0 {
		w.Write([]string{utils.T("sales_by_customer", language)})
		w.Write([]string{"Customer ID", "Customer Name", "Total Sales", "Transaction Count", "Average Order", "Last Order Date"})
		for i := 0; i < len(data.SalesByCustomer); i++ {
			c := data.SalesByCustomer[i]
			w.Write([]string{
				fmt.Sprintf("%d", c.CustomerID),
				c.CustomerName,
				fmt.Sprintf("%.2f", c.TotalAmount),
				fmt.Sprintf("%d", c.TransactionCount),
				fmt.Sprintf("%.2f", c.AverageOrder),
				c.LastOrderDate.Format("2006-01-02"),
			})
		}
		w.Write([]string{})
	}

	// Items Sold section
	hasItems := false
	for _, c := range data.SalesByCustomer {
		if len(c.Items) > 0 {
			hasItems = true
			break
		}
	}

	if hasItems {
		w.Write([]string{"Items Sold"})
		w.Write([]string{"Customer", "Product Code", "Product Name", "Quantity", "Unit", "Unit Price", "Total Price", "Sale Date", "Invoice"})

		for _, customer := range data.SalesByCustomer {
			if len(customer.Items) == 0 {
				continue
			}

			for _, item := range customer.Items {
				w.Write([]string{
					customer.CustomerName,
					item.ProductCode,
					item.ProductName,
					fmt.Sprintf("%.2f", item.Quantity),
					item.Unit,
					fmt.Sprintf("%.2f", item.UnitPrice),
					fmt.Sprintf("%.2f", item.TotalPrice),
					item.SaleDate.Format("2006-01-02"),
					item.InvoiceNumber,
				})
			}

			// Customer subtotal
			w.Write([]string{
				fmt.Sprintf("Subtotal (%s)", customer.CustomerName),
				"", "", "", "", "",
				fmt.Sprintf("%.2f", customer.TotalAmount),
				"", "",
			})
		}
		w.Write([]string{})
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("failed to write CSV: %v", err)
	}
	return buf.Bytes(), nil
}

// ExportToPDF exports sales report to PDF bytes with localization
func (s *SalesReportExportService) ExportToPDF(data *SalesSummaryData, userID uint) ([]byte, error) {
	// Get user language preference
	language := utils.GetUserLanguageFromSettings(s.db)
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	lm, _, _, _ := pdf.GetMargins()
	pageW, pageH := pdf.GetPageSize()
	rm := float64(15)
	contentW := pageW - lm - rm

	// Company settings for consistent letterhead
	settings := s.getCompanyInfo()

	// Unified letterhead: logo left, company info right, separator
	contentY := drawUnifiedLetterhead(pdf, settings, 35)

	// Title
	pdf.SetY(contentY)
	pdf.SetFont("Arial", "B", 14)
	pdf.SetTextColor(25, 55, 109)
	title := utils.T("sales_report", language)
	pdf.Cell(contentW, 8, title)
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(8)

	// Period
	pdf.SetFont("Arial", "", 10)
	period := fmt.Sprintf("%s: %s %s %s",
		utils.T("period", language),
		data.StartDate.Format("02/01/2006"),
		utils.T("to", language),
		data.EndDate.Format("02/01/2006"))
	pdf.Cell(contentW, 5, period)
	pdf.Ln(5)

	// Generated at
	pdf.SetFont("Arial", "I", 8)
	pdf.SetTextColor(128, 128, 128)
	generated := fmt.Sprintf("%s: %s", utils.T("generated_on", language), time.Now().Format("02/01/2006 15:04"))
	pdf.Cell(contentW, 4, generated)
	pdf.Ln(8)
	pdf.SetTextColor(0, 0, 0)

	// Divider
	pdf.SetDrawColor(200, 200, 200)
	pdf.SetLineWidth(0.3)
	pdf.Line(lm, pdf.GetY(), pageW-rm, pdf.GetY())
	pdf.Ln(6)

	// Summary Section
	pdf.SetFont("Arial", "B", 11)
	pdf.Cell(contentW, 6, utils.T("summary", language))
	pdf.Ln(7)

	pdf.SetFont("Arial", "", 9)
	pdf.SetFillColor(245, 245, 245)

	// Summary table
	summaryItems := []struct {
		label string
		value string
	}{
		{"Total Sales (Gross)", formatRupiahSimple(data.TotalSales)},
		{utils.T("total_revenue", language), formatRupiahSimple(data.TotalRevenue)},
		{"Total Tax", formatRupiahSimple(data.TotalTax)},
		{"Total Discount", formatRupiahSimple(data.TotalDiscount)},
		{"Total Returns", formatRupiahSimple(data.TotalReturns)},
		{"Net Revenue", formatRupiahSimple(data.NetRevenue)},
		{utils.T("total_transactions", language), fmt.Sprintf("%d", data.TotalTransactions)},
		{utils.T("average_order_value", language), formatRupiahSimple(data.AverageOrderValue)},
		{utils.T("total_customers", language), fmt.Sprintf("%d", data.TotalCustomers)},
	}

	for _, item := range summaryItems {
		pdf.CellFormat(contentW*0.6, 6, item.label, "1", 0, "L", true, 0, "")
		pdf.CellFormat(contentW*0.4, 6, item.value, "1", 1, "R", true, 0, "")
	}

	pdf.Ln(8)

	// Sales by Customer section - reduced widths: 55 + 25 + 35 + 40 = 155mm
	if len(data.SalesByCustomer) > 0 {
		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(contentW, 6, utils.T("sales_by_customer", language))
		pdf.Ln(8)

		// Table header
		pdf.SetX(lm)
		pdf.SetFont("Arial", "B", 7.5)
		pdf.SetFillColor(245, 245, 245)
		pdf.CellFormat(55, 6, utils.T("customer_name", language), "1", 0, "C", true, 0, "")
		pdf.CellFormat(25, 6, utils.T("transactions", language), "1", 0, "C", true, 0, "")
		pdf.CellFormat(35, 6, utils.T("total_sales", language), "1", 0, "C", true, 0, "")
		pdf.CellFormat(40, 6, utils.T("average_order", language), "1", 1, "C", true, 0, "")

		// Customer rows
		pdf.SetFont("Arial", "", 7.5)
		for i := 0; i < len(data.SalesByCustomer); i++ {
			c := data.SalesByCustomer[i]

			// Check if we need a new page
			if pdf.GetY() > pageH-30 {
				pdf.AddPage()
				// Reprint header
				pdf.SetX(lm)
				pdf.SetFont("Arial", "B", 7.5)
				pdf.SetFillColor(245, 245, 245)
				pdf.CellFormat(55, 6, utils.T("customer_name", language), "1", 0, "C", true, 0, "")
				pdf.CellFormat(25, 6, utils.T("transactions", language), "1", 0, "C", true, 0, "")
				pdf.CellFormat(35, 6, utils.T("total_sales", language), "1", 0, "C", true, 0, "")
				pdf.CellFormat(40, 6, utils.T("average_order", language), "1", 1, "C", true, 0, "")
				pdf.SetFont("Arial", "", 7.5)
			}

			customerName := c.CustomerName
			if len(customerName) > 32 {
				customerName = customerName[:29] + "..."
			}

			pdf.SetX(lm)
			pdf.CellFormat(55, 6, customerName, "1", 0, "L", false, 0, "")
			pdf.CellFormat(25, 6, fmt.Sprintf("%d", c.TransactionCount), "1", 0, "C", false, 0, "")
			pdf.CellFormat(35, 6, formatRupiahSimple(c.TotalAmount), "1", 0, "R", false, 0, "")
			pdf.CellFormat(40, 6, formatRupiahSimple(c.AverageOrder), "1", 1, "R", false, 0, "")
		}

		pdf.Ln(6)
	}

	// Items Sold section
	hasItems := false
	for _, c := range data.SalesByCustomer {
		if len(c.Items) > 0 {
			hasItems = true
			break
		}
	}

	if hasItems {
		// Check if we need a new page for items section
		if pdf.GetY() > pageH-80 {
			pdf.AddPage()
		}

		pdf.SetFont("Arial", "B", 12)
		pdf.Cell(0, 8, utils.T("items_sold", language))
		pdf.Ln(10)

		// Loop through customers and their items
		for _, customer := range data.SalesByCustomer {
			if len(customer.Items) == 0 {
				continue
			}

			// Check if we need a new page for this customer
			if pdf.GetY() > pageH-60 {
				pdf.AddPage()
			}

			// Customer name header - reduced width: 105 + 50 = 155mm
			pdf.SetX(lm)
			pdf.SetFont("Arial", "B", 9)
			pdf.SetFillColor(230, 245, 255)
			customerName := customer.CustomerName
			if len(customerName) > 35 {
				customerName = customerName[:32] + "..."
			}
			pdf.CellFormat(105, 7, customerName, "1", 0, "L", true, 0, "")
			pdf.CellFormat(50, 7, fmt.Sprintf("%d items", len(customer.Items)), "1", 1, "R", true, 0, "")

			// Items table header - Total: 55 + 18 + 30 + 30 + 22 = 155mm
			pdf.SetX(lm)
			pdf.SetFont("Arial", "B", 7.5)
			pdf.SetFillColor(245, 245, 245)
			pdf.CellFormat(55, 6, utils.T("product", language), "1", 0, "C", true, 0, "")
			pdf.CellFormat(18, 6, utils.T("qty", language), "1", 0, "C", true, 0, "")
			pdf.CellFormat(30, 6, utils.T("unit_price", language), "1", 0, "C", true, 0, "")
			pdf.CellFormat(30, 6, utils.T("total", language), "1", 0, "C", true, 0, "")
			pdf.CellFormat(22, 6, utils.T("date", language), "1", 1, "C", true, 0, "")

			// Items rows
			pdf.SetFont("Arial", "", 7.5)
			for _, item := range customer.Items {
				// Check if we need a new page
				if pdf.GetY() > pageH-25 {
					pdf.AddPage()
					// Reprint header
					pdf.SetX(lm)
					pdf.SetFont("Arial", "B", 7.5)
					pdf.SetFillColor(245, 245, 245)
					pdf.CellFormat(55, 6, utils.T("product", language), "1", 0, "C", true, 0, "")
					pdf.CellFormat(18, 6, utils.T("qty", language), "1", 0, "C", true, 0, "")
					pdf.CellFormat(30, 6, utils.T("unit_price", language), "1", 0, "C", true, 0, "")
					pdf.CellFormat(30, 6, utils.T("total", language), "1", 0, "C", true, 0, "")
					pdf.CellFormat(22, 6, utils.T("date", language), "1", 1, "C", true, 0, "")
					pdf.SetFont("Arial", "", 7.5)
				}

				productName := item.ProductName
				if len(productName) > 32 {
					productName = productName[:29] + "..."
				}

				qtyStr := fmt.Sprintf("%.0f %s", item.Quantity, item.Unit)
				if len(qtyStr) > 12 {
					qtyStr = qtyStr[:10] + ".."
				}

				pdf.SetX(lm)
				pdf.CellFormat(55, 6, productName, "1", 0, "L", false, 0, "")
				pdf.CellFormat(18, 6, qtyStr, "1", 0, "R", false, 0, "")
				pdf.CellFormat(30, 6, formatRupiahSimple(item.UnitPrice), "1", 0, "R", false, 0, "")
				pdf.CellFormat(30, 6, formatRupiahSimple(item.TotalPrice), "1", 0, "R", false, 0, "")
				pdf.CellFormat(22, 6, item.SaleDate.Format("02/01/06"), "1", 1, "C", false, 0, "")
			}

			// Customer subtotal - Total: 103 + 52 = 155mm
			pdf.SetX(lm)
			pdf.SetFont("Arial", "B", 8)
			pdf.SetFillColor(230, 245, 255)
			custSubName := customer.CustomerName
			pdf.CellFormat(103, 6, fmt.Sprintf("Subtotal (%s)", custSubName), "1", 0, "R", true, 0, "")
			pdf.CellFormat(52, 6, formatRupiahSimple(customer.TotalAmount), "1", 1, "R", true, 0, "")
			pdf.Ln(4)
		}
	}

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, fmt.Errorf("failed to generate sales report PDF: %v", err)
	}
	return out.Bytes(), nil
}
