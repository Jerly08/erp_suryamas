package services

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/utils"

	"github.com/jung-kurt/gofpdf"
	"gorm.io/gorm"
)

// PurchaseReportExportService handles export functionality for Purchase Report
type PurchaseReportExportService struct{ db *gorm.DB }

// NewPurchaseReportExportService creates a new purchase report export service
func NewPurchaseReportExportService(db *gorm.DB) *PurchaseReportExportService {
	return &PurchaseReportExportService{db: db}
}

// getCompanyInfo from settings with defaults
func (s *PurchaseReportExportService) getCompanyInfo() *models.Settings {
	if s.db == nil {
		return &models.Settings{CompanyName: "PT. Sistem Akuntansi Indonesia"}
	}
	var settings models.Settings
	if err := s.db.First(&settings).Error; err != nil {
		return &models.Settings{CompanyName: "PT. Sistem Akuntansi Indonesia"}
	}
	return &settings
}

// ExportToCSV exports purchase report to CSV bytes with localization
func (s *PurchaseReportExportService) ExportToCSV(data *PurchaseReportData, userID uint) ([]byte, error) {
	utils.SanitizeExportData(data)
	// Get user language preference
	language := utils.GetUserLanguageFromSettings(s.db)
	var buf bytes.Buffer
	w := utils.NewSanitizedCSVWriter(&buf)

	// Header with localization
	w.Write([]string{utils.T("purchase_report", language)})
	w.Write([]string{data.Company.Name})
	w.Write([]string{utils.T("period", language) + ":", data.StartDate.Format("2006-01-02"), utils.T("to", language), data.EndDate.Format("2006-01-02")})
	w.Write([]string{utils.T("generated_on", language) + ":", data.GeneratedAt.In(time.Local).Format("2006-01-02 15:04")})
	w.Write([]string{})

	// Summary with localization
	w.Write([]string{utils.T("summary", language)})
	w.Write([]string{utils.T("total_purchases", language), fmt.Sprintf("%d", data.TotalPurchases)})
	w.Write([]string{utils.T("completed_purchases", language), fmt.Sprintf("%d", data.CompletedPurchases)})
	w.Write([]string{utils.T("total_amount", language), fmt.Sprintf("%.2f", data.TotalAmount)})
	w.Write([]string{utils.T("total_paid", language), fmt.Sprintf("%.2f", data.TotalPaid)})
	w.Write([]string{utils.T("outstanding_payables", language), fmt.Sprintf("%.2f", data.OutstandingPayables)})
	w.Write([]string{})

	// Purchases by vendor (top 20) with localization
	if len(data.PurchasesByVendor) > 0 {
		w.Write([]string{utils.T("purchases_by_vendor", language)})
		headers := utils.GetCSVHeaders("purchase_report", language)
		w.Write(headers)
		for i := 0; i < len(data.PurchasesByVendor); i++ {
			v := data.PurchasesByVendor[i]
			w.Write([]string{
				fmt.Sprintf("%d", v.VendorID), v.VendorName,
				fmt.Sprintf("%d", v.TotalPurchases),
				fmt.Sprintf("%.2f", v.TotalAmount),
				fmt.Sprintf("%.2f", v.TotalPaid),
				fmt.Sprintf("%.2f", v.Outstanding),
				v.LastPurchaseDate.Format("2006-01-02"),
				v.PaymentMethod, v.Status,
			})
		}
		w.Write([]string{})
	}

	// Items Purchased section
	hasItems := false
	for _, v := range data.PurchasesByVendor {
		if len(v.Items) > 0 {
			hasItems = true
			break
		}
	}

	if hasItems {
		w.Write([]string{"Items Purchased"})
		w.Write([]string{"Vendor", "Product Code", "Product Name", "Quantity", "Unit", "Unit Price", "Total Price", "Purchase Date", "Invoice"})

		for _, vendor := range data.PurchasesByVendor {
			if len(vendor.Items) == 0 {
				continue
			}

			for _, item := range vendor.Items {
				w.Write([]string{
					vendor.VendorName,
					item.ProductCode,
					item.ProductName,
					fmt.Sprintf("%.2f", item.Quantity),
					item.Unit,
					fmt.Sprintf("%.2f", item.UnitPrice),
					fmt.Sprintf("%.2f", item.TotalPrice),
					item.PurchaseDate.Format("2006-01-02"),
					item.InvoiceNumber,
				})
			}

			// Vendor subtotal
			w.Write([]string{
				fmt.Sprintf("Subtotal (%s)", vendor.VendorName),
				"", "", "", "", "",
				fmt.Sprintf("%.2f", vendor.TotalAmount),
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

// ExportToPDF exports purchase report to PDF bytes with localization
func (s *PurchaseReportExportService) ExportToPDF(data *PurchaseReportData, userID uint) ([]byte, error) {
	utils.SanitizeExportData(data)
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

	// Title (matching sales report style)
	pdf.SetY(contentY)
	pdf.SetFont("Arial", "B", 14)
	pdf.SetTextColor(25, 55, 109)
	title := utils.T("purchase_report", language)
	pdf.Cell(contentW, 8, title)
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(8)

	// Period (matching sales report date format dd/mm/yyyy)
	pdf.SetFont("Arial", "", 10)
	period := fmt.Sprintf("%s: %s %s %s",
		utils.T("period", language),
		data.StartDate.Format("02/01/2006"),
		utils.T("to", language),
		data.EndDate.Format("02/01/2006"))
	pdf.Cell(contentW, 5, period)
	pdf.Ln(5)

	// Generated at (italic gray, matching sales report)
	pdf.SetFont("Arial", "I", 8)
	pdf.SetTextColor(128, 128, 128)
	generated := fmt.Sprintf("%s: %s", utils.T("generated_on", language), time.Now().Format("02/01/2006 15:04"))
	pdf.Cell(contentW, 4, generated)
	pdf.Ln(8)
	pdf.SetTextColor(0, 0, 0)

	// Divider (matching sales report)
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
		{utils.T("total_purchases", language), fmt.Sprintf("%d", data.TotalPurchases)},
		{utils.T("completed_purchases", language), fmt.Sprintf("%d", data.CompletedPurchases)},
		{utils.T("total_amount", language), formatRupiahSimple(data.TotalAmount)},
		{utils.T("total_paid", language), formatRupiahSimple(data.TotalPaid)},
		{utils.T("outstanding_payables", language), formatRupiahSimple(data.OutstandingPayables)},
	}

	for _, item := range summaryItems {
		pdf.CellFormat(contentW*0.6, 6, item.label, "1", 0, "L", true, 0, "")
		pdf.CellFormat(contentW*0.4, 6, item.value, "1", 1, "R", true, 0, "")
	}

	pdf.Ln(8)

	// Purchases by Vendor section (matching sales report column layout: 55+25+35+40=155mm)
	if len(data.PurchasesByVendor) > 0 {
		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(contentW, 6, utils.T("purchases_by_vendor", language))
		pdf.Ln(8)

		// Table header
		pdf.SetX(lm)
		pdf.SetFont("Arial", "B", 7.5)
		pdf.SetFillColor(245, 245, 245)
		pdf.CellFormat(55, 6, utils.T("vendor", language), "1", 0, "C", true, 0, "")
		pdf.CellFormat(25, 6, utils.T("orders", language), "1", 0, "C", true, 0, "")
		pdf.CellFormat(35, 6, utils.T("total_amount", language), "1", 0, "C", true, 0, "")
		pdf.CellFormat(40, 6, utils.T("outstanding", language), "1", 1, "C", true, 0, "")

		// Vendor rows
		pdf.SetFont("Arial", "", 7.5)
		for i := 0; i < len(data.PurchasesByVendor); i++ {
			v := data.PurchasesByVendor[i]

			// Check if we need a new page
			if pdf.GetY() > pageH-30 {
				pdf.AddPage()
				// Reprint header
				pdf.SetX(lm)
				pdf.SetFont("Arial", "B", 7.5)
				pdf.SetFillColor(245, 245, 245)
				pdf.CellFormat(55, 6, utils.T("vendor", language), "1", 0, "C", true, 0, "")
				pdf.CellFormat(25, 6, utils.T("orders", language), "1", 0, "C", true, 0, "")
				pdf.CellFormat(35, 6, utils.T("total_amount", language), "1", 0, "C", true, 0, "")
				pdf.CellFormat(40, 6, utils.T("outstanding", language), "1", 1, "C", true, 0, "")
				pdf.SetFont("Arial", "", 7.5)
			}

			pdf.SetX(lm)
			pdf.CellFormat(55, 6, v.VendorName, "1", 0, "L", false, 0, "")
			pdf.CellFormat(25, 6, fmt.Sprintf("%d", v.TotalPurchases), "1", 0, "C", false, 0, "")
			pdf.CellFormat(35, 6, formatRupiahSimple(v.TotalAmount), "1", 0, "R", false, 0, "")
			pdf.CellFormat(40, 6, formatRupiahSimple(v.Outstanding), "1", 1, "R", false, 0, "")
		}

		pdf.Ln(6)
	}

	// Items Purchased section - grouped by vendor
	hasItems := false
	for _, v := range data.PurchasesByVendor {
		if len(v.Items) > 0 {
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
		pdf.Cell(0, 8, utils.T("items_purchased", language))
		pdf.Ln(10)

		// Loop through vendors and their items
		for _, vendor := range data.PurchasesByVendor {
			if len(vendor.Items) == 0 {
				continue
			}

			// Check if we need a new page for this vendor
			if pdf.GetY() > pageH-60 {
				pdf.AddPage()
			}

			// Vendor name header - reduced width: 105 + 50 = 155mm
			pdf.SetX(lm)
			pdf.SetFont("Arial", "B", 9)
			pdf.SetFillColor(230, 245, 255)
			pdf.CellFormat(105, 7, vendor.VendorName, "1", 0, "L", true, 0, "")
			pdf.CellFormat(50, 7, fmt.Sprintf("%d items", len(vendor.Items)), "1", 1, "R", true, 0, "")

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
			for _, item := range vendor.Items {
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
				pdf.CellFormat(22, 6, item.PurchaseDate.Format("02/01/06"), "1", 1, "C", false, 0, "")
			}

			// Vendor subtotal - light blue background
			pdf.SetX(lm)
			pdf.SetFont("Arial", "B", 8)
			pdf.SetFillColor(230, 245, 255)
			vendorSubName := vendor.VendorName
			pdf.CellFormat(103, 6, fmt.Sprintf("Subtotal (%s)", vendorSubName), "1", 0, "R", true, 0, "")
			pdf.CellFormat(52, 6, formatRupiahSimple(vendor.TotalAmount), "1", 1, "R", true, 0, "")
			pdf.Ln(4)
		}
	}

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, fmt.Errorf("failed to generate purchase report PDF: %v", err)
	}
	return out.Bytes(), nil
}

// formatRupiahSimple formats a number as Indonesian Rupiah (no decimals)
func formatRupiahSimple(amount float64) string {
	// Round to integer and format with thousand separators using simple logic
	s := fmt.Sprintf("%.0f", amount)
	if s == "0" {
		return "Rp 0"
	}
	var parts []string
	for i, r := range reverseString(s) {
		if i > 0 && i%3 == 0 {
			parts = append(parts, ".")
		}
		parts = append(parts, string(r))
	}
	formatted := reverseString(strings.Join(parts, ""))
	return "Rp " + formatted
}

func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// detectImageType function is defined in pdf_service.go and uses file headers for more accurate detection
