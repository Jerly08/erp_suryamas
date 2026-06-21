package services

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"app-sistem-akuntansi/utils"

	"github.com/jung-kurt/gofpdf"
)

// GenerateCustomerHistoryPDF generates PDF for customer transaction history
func (p *PDFService) GenerateCustomerHistoryPDF(historyData interface{}) ([]byte, error) {
	utils.SanitizeExportData(historyData)
	view, err := buildCustomerHistoryExportView(historyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse customer history data: %v", err)
	}

	// Create PDF
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	lm, tm, rm, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	contentW := pageW - lm - rm

	// Company info
	companyInfo, err := p.getCompanyInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get company info: %v", err)
	}

	// Logo and company header
	logoX, logoY, logoSize := lm, tm, 35.0
	logoAdded := false
	if strings.TrimSpace(companyInfo.CompanyLogo) != "" {
		logoPath := companyInfo.CompanyLogo
		if strings.HasPrefix(logoPath, "/") {
			logoPath = "." + logoPath
		}
		if _, err := os.Stat(logoPath); err == nil {
			if imgType := detectImageType(logoPath); imgType != "" {
				pdf.ImageOptions(logoPath, logoX, logoY, logoSize, 0, false, gofpdf.ImageOptions{ImageType: imgType, ReadDpi: true}, 0, "")
				logoAdded = true
			}
		}
	}
	if !logoAdded {
		pdf.SetDrawColor(220, 220, 220)
		pdf.SetFillColor(248, 249, 250)
		pdf.SetLineWidth(0.3)
		pdf.Rect(logoX, logoY, logoSize, logoSize, "FD")
		pdf.SetFont("Arial", "B", 16)
		pdf.SetTextColor(120, 120, 120)
		pdf.SetXY(logoX+8, logoY+19)
		pdf.CellFormat(19, 8, "</>", "", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}

	// Company name and address on right
	companyInfoX := pageW - rm
	pdf.SetFont("Arial", "B", 12)
	nameW := pdf.GetStringWidth(companyInfo.CompanyName)
	pdf.SetXY(companyInfoX-nameW, tm)
	pdf.Cell(nameW, 6, companyInfo.CompanyName)

	pdf.SetFont("Arial", "", 9)
	addrW := pdf.GetStringWidth(companyInfo.CompanyAddress)
	pdf.SetXY(companyInfoX-addrW, tm+8)
	pdf.Cell(addrW, 4, companyInfo.CompanyAddress)

	phoneText := fmt.Sprintf("Phone: %s", companyInfo.CompanyPhone)
	phoneW := pdf.GetStringWidth(phoneText)
	pdf.SetXY(companyInfoX-phoneW, tm+14)
	pdf.Cell(phoneW, 4, phoneText)

	// Divider line
	pdf.SetDrawColor(238, 238, 238)
	pdf.SetLineWidth(0.2)
	pdf.Line(lm, tm+40, pageW-rm, tm+40)

	// Title
	pdf.SetY(tm + 45)
	pdf.SetFont("Arial", "B", 18)
	pdf.SetTextColor(51, 51, 51)
	pdf.Cell(contentW, 8, "CUSTOMER TRANSACTION HISTORY")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(10)

	// Customer info and period
	pdf.SetFont("Arial", "B", 9)
	pdf.SetX(lm)
	pdf.Cell(30, 5, "Customer:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(65, 5, fmt.Sprintf("%s (%s)", view.CustomerName, view.CustomerCode))

	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(0, 0, 0)
	rightX := lm + contentW - 70
	pdf.SetX(rightX)
	pdf.Cell(22, 5, "Period:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(48, 5, fmt.Sprintf("%s to %s", view.StartDate, view.EndDate))
	pdf.Ln(8)

	// Summary box
	pdf.SetFillColor(240, 248, 255)
	pdf.SetDrawColor(150, 180, 220)
	summaryY := pdf.GetY()
	pdf.Rect(lm, summaryY, contentW, 25, "FD")

	pdf.SetFont("Arial", "B", 9)
	pdf.SetY(summaryY + 3)
	pdf.Cell(contentW/4, 5, "Total Activities")
	pdf.Cell(contentW/4, 5, "Total Sales")
	pdf.Cell(contentW/4, 5, "Total Payments")
	pdf.Cell(contentW/4, 5, "Outstanding")
	pdf.Ln(6)

	pdf.SetFont("Arial", "", 11)
	pdf.Cell(contentW/4, 5, fmt.Sprintf("%d", view.TotalActivities))
	pdf.Cell(contentW/4, 5, p.formatRupiah(view.SalesTotalAmount))
	pdf.Cell(contentW/4, 5, p.formatRupiah(view.PaymentTotalAmount))
	pdf.Cell(contentW/4, 5, p.formatRupiah(view.SalesTotalOutstanding))
	pdf.Ln(15)

	pdf.SetFillColor(246, 249, 252)
	pdf.SetDrawColor(210, 220, 235)
	noteY := pdf.GetY()
	pdf.Rect(lm, noteY, contentW, 10, "FD")
	pdf.SetXY(lm+2, noteY+2.2)
	pdf.SetFont("Arial", "", 8)
	pdf.MultiCell(contentW-4, 4.2, "Payments are shown as settlement activity and are not counted as new sales.", "", "L", false)
	pdf.Ln(2)

	ensureSpace := func(height float64) {
		if pdf.GetY()+height > 280 {
			pdf.AddPage()
		}
	}

	salesWidths := []float64{18, 28, 50, 22, 22, 22, 18}
	drawSalesSectionHeader := func(continued bool) {
		title := "SALES DOCUMENTS"
		if continued {
			title += " (continued)"
		}
		pdf.SetFont("Arial", "B", 10)
		pdf.SetTextColor(25, 55, 109)
		pdf.Cell(contentW, 6, title)
		pdf.SetTextColor(0, 0, 0)
		pdf.Ln(7)

		pdf.SetFont("Arial", "B", 7)
		pdf.SetFillColor(220, 220, 220)
		pdf.CellFormat(salesWidths[0], 7, "Date", "1", 0, "C", true, 0, "")
		pdf.CellFormat(salesWidths[1], 7, "Code", "1", 0, "C", true, 0, "")
		pdf.CellFormat(salesWidths[2], 7, "Description", "1", 0, "L", true, 0, "")
		pdf.CellFormat(salesWidths[3], 7, "Amount", "1", 0, "R", true, 0, "")
		pdf.CellFormat(salesWidths[4], 7, "Paid", "1", 0, "R", true, 0, "")
		pdf.CellFormat(salesWidths[5], 7, "Outstanding", "1", 0, "R", true, 0, "")
		pdf.CellFormat(salesWidths[6], 7, "Status", "1", 0, "C", true, 0, "")
		pdf.Ln(7)
	}

	paymentWidths := []float64{18, 28, 58, 24, 24, 28}
	drawPaymentSectionHeader := func(continued bool) {
		title := "PAYMENTS"
		if continued {
			title += " (continued)"
		}
		pdf.SetFont("Arial", "B", 10)
		pdf.SetTextColor(25, 55, 109)
		pdf.Cell(contentW, 6, title)
		pdf.SetTextColor(0, 0, 0)
		pdf.Ln(7)

		pdf.SetFont("Arial", "B", 7)
		pdf.SetFillColor(220, 220, 220)
		pdf.CellFormat(paymentWidths[0], 7, "Date", "1", 0, "C", true, 0, "")
		pdf.CellFormat(paymentWidths[1], 7, "Code", "1", 0, "C", true, 0, "")
		pdf.CellFormat(paymentWidths[2], 7, "Description", "1", 0, "L", true, 0, "")
		pdf.CellFormat(paymentWidths[3], 7, "Reference", "1", 0, "L", true, 0, "")
		pdf.CellFormat(paymentWidths[4], 7, "Amount", "1", 0, "R", true, 0, "")
		pdf.CellFormat(paymentWidths[5], 7, "Status", "1", 0, "C", true, 0, "")
		pdf.Ln(7)
	}

	if len(view.SalesTransactions) > 0 {
		ensureSpace(20)
		drawSalesSectionHeader(false)
		pdf.SetFont("Arial", "", 6.5)
		for _, tx := range view.SalesTransactions {
			if pdf.GetY()+6 > 280 {
				pdf.AddPage()
				drawSalesSectionHeader(true)
				pdf.SetFont("Arial", "", 6.5)
			}
			pdf.CellFormat(salesWidths[0], 6, exportHistoryDate(getString(tx, "date")), "1", 0, "L", false, 0, "")
			pdf.CellFormat(salesWidths[1], 6, getString(tx, "transaction_code"), "1", 0, "L", false, 0, "")
			pdf.CellFormat(salesWidths[2], 6, exportHistoryTrim(getString(tx, "description"), 42), "1", 0, "L", false, 0, "")
			pdf.CellFormat(salesWidths[3], 6, p.formatRupiahCompact(getNumFrom(tx["amount"])), "1", 0, "R", false, 0, "")
			pdf.CellFormat(salesWidths[4], 6, p.formatRupiahCompact(getNumFrom(tx["paid_amount"])), "1", 0, "R", false, 0, "")
			pdf.CellFormat(salesWidths[5], 6, p.formatRupiahCompact(getNumFrom(tx["outstanding"])), "1", 0, "R", false, 0, "")
			pdf.CellFormat(salesWidths[6], 6, getString(tx, "status"), "1", 0, "C", false, 0, "")
			pdf.Ln(6)
		}

		ensureSpace(8)
		pdf.SetFont("Arial", "B", 7)
		pdf.SetFillColor(240, 240, 240)
		leftGroup := salesWidths[0] + salesWidths[1] + salesWidths[2]
		pdf.CellFormat(leftGroup, 6.5, "TOTAL SALES", "1", 0, "R", true, 0, "")
		pdf.CellFormat(salesWidths[3], 6.5, p.formatRupiahCompact(view.SalesTotalAmount), "1", 0, "R", true, 0, "")
		pdf.CellFormat(salesWidths[4], 6.5, p.formatRupiahCompact(view.SalesTotalPaid), "1", 0, "R", true, 0, "")
		pdf.CellFormat(salesWidths[5], 6.5, p.formatRupiahCompact(view.SalesTotalOutstanding), "1", 0, "R", true, 0, "")
		pdf.CellFormat(salesWidths[6], 6.5, "", "1", 0, "C", true, 0, "")
		pdf.Ln(10)
	}

	if len(view.PaymentTransactions) > 0 {
		ensureSpace(20)
		drawPaymentSectionHeader(false)
		pdf.SetFont("Arial", "", 6.5)
		for _, tx := range view.PaymentTransactions {
			if pdf.GetY()+6 > 280 {
				pdf.AddPage()
				drawPaymentSectionHeader(true)
				pdf.SetFont("Arial", "", 6.5)
			}
			pdf.CellFormat(paymentWidths[0], 6, exportHistoryDate(getString(tx, "date")), "1", 0, "L", false, 0, "")
			pdf.CellFormat(paymentWidths[1], 6, getString(tx, "transaction_code"), "1", 0, "L", false, 0, "")
			pdf.CellFormat(paymentWidths[2], 6, exportHistoryTrim(getString(tx, "description"), 52), "1", 0, "L", false, 0, "")
			pdf.CellFormat(paymentWidths[3], 6, exportHistoryTrim(getString(tx, "reference"), 18), "1", 0, "L", false, 0, "")
			pdf.CellFormat(paymentWidths[4], 6, p.formatRupiahCompact(getNumFrom(tx["amount"])), "1", 0, "R", false, 0, "")
			pdf.CellFormat(paymentWidths[5], 6, getString(tx, "status"), "1", 0, "C", false, 0, "")
			pdf.Ln(6)
		}

		ensureSpace(8)
		pdf.SetFont("Arial", "B", 7)
		pdf.SetFillColor(240, 240, 240)
		leftGroup := paymentWidths[0] + paymentWidths[1] + paymentWidths[2] + paymentWidths[3]
		pdf.CellFormat(leftGroup, 6.5, "TOTAL PAYMENTS", "1", 0, "R", true, 0, "")
		pdf.CellFormat(paymentWidths[4], 6.5, p.formatRupiahCompact(view.PaymentTotalAmount), "1", 0, "R", true, 0, "")
		pdf.CellFormat(paymentWidths[5], 6.5, "", "1", 0, "C", true, 0, "")
		pdf.Ln(10)
	}

	// Footer
	pdf.Ln(10)
	pdf.SetFont("Arial", "I", 8)
	pdf.Cell(contentW, 4, fmt.Sprintf("Report generated on %s", time.Now().Format("02/01/2006 15:04")))

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to generate customer history PDF: %v", err)
	}
	return buf.Bytes(), nil
}

// GenerateVendorHistoryPDF generates PDF for vendor transaction history
func (p *PDFService) GenerateVendorHistoryPDF(historyData interface{}) ([]byte, error) {
	utils.SanitizeExportData(historyData)
	view, err := buildVendorHistoryExportView(historyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse vendor history data: %v", err)
	}

	// Create PDF
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	lm, tm, rm, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	contentW := pageW - lm - rm

	// Company info
	companyInfo, err := p.getCompanyInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get company info: %v", err)
	}

	// Logo and company header (similar to General Ledger)
	logoX, logoY, logoSize := lm, tm, 35.0
	logoAdded := false
	if strings.TrimSpace(companyInfo.CompanyLogo) != "" {
		logoPath := companyInfo.CompanyLogo
		if strings.HasPrefix(logoPath, "/") {
			logoPath = "." + logoPath
		}
		if _, err := os.Stat(logoPath); err == nil {
			if imgType := detectImageType(logoPath); imgType != "" {
				pdf.ImageOptions(logoPath, logoX, logoY, logoSize, 0, false, gofpdf.ImageOptions{ImageType: imgType, ReadDpi: true}, 0, "")
				logoAdded = true
			}
		}
	}
	if !logoAdded {
		pdf.SetDrawColor(220, 220, 220)
		pdf.SetFillColor(248, 249, 250)
		pdf.SetLineWidth(0.3)
		pdf.Rect(logoX, logoY, logoSize, logoSize, "FD")
		pdf.SetFont("Arial", "B", 16)
		pdf.SetTextColor(120, 120, 120)
		pdf.SetXY(logoX+8, logoY+19)
		pdf.CellFormat(19, 8, "</>", "", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}

	// Company name and address on right
	companyInfoX := pageW - rm
	pdf.SetFont("Arial", "B", 12)
	nameW := pdf.GetStringWidth(companyInfo.CompanyName)
	pdf.SetXY(companyInfoX-nameW, tm)
	pdf.Cell(nameW, 6, companyInfo.CompanyName)

	pdf.SetFont("Arial", "", 9)
	addrW := pdf.GetStringWidth(companyInfo.CompanyAddress)
	pdf.SetXY(companyInfoX-addrW, tm+8)
	pdf.Cell(addrW, 4, companyInfo.CompanyAddress)

	phoneText := fmt.Sprintf("Phone: %s", companyInfo.CompanyPhone)
	phoneW := pdf.GetStringWidth(phoneText)
	pdf.SetXY(companyInfoX-phoneW, tm+14)
	pdf.Cell(phoneW, 4, phoneText)

	// Divider line
	pdf.SetDrawColor(238, 238, 238)
	pdf.SetLineWidth(0.2)
	pdf.Line(lm, tm+40, pageW-rm, tm+40)

	// Title
	pdf.SetY(tm + 45)
	pdf.SetFont("Arial", "B", 18)
	pdf.SetTextColor(51, 51, 51)
	pdf.Cell(contentW, 8, "VENDOR TRANSACTION HISTORY")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(10)

	// Vendor info and period
	pdf.SetFont("Arial", "B", 9)
	pdf.SetX(lm)
	pdf.Cell(30, 5, "Vendor:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(65, 5, fmt.Sprintf("%s (%s)", view.VendorName, view.VendorCode))

	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(0, 0, 0)
	rightX := lm + contentW - 70
	pdf.SetX(rightX)
	pdf.Cell(22, 5, "Period:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(48, 5, fmt.Sprintf("%s to %s", view.StartDate, view.EndDate))
	pdf.Ln(8)

	// Summary box
	pdf.SetFillColor(255, 248, 240)
	pdf.SetDrawColor(220, 180, 150)
	summaryY := pdf.GetY()
	pdf.Rect(lm, summaryY, contentW, 25, "FD")

	pdf.SetFont("Arial", "B", 9)
	pdf.SetY(summaryY + 3)
	pdf.Cell(contentW/4, 5, "Total Activities")
	pdf.Cell(contentW/4, 5, "Total Purchases")
	pdf.Cell(contentW/4, 5, "Total Payments")
	pdf.Cell(contentW/4, 5, "Outstanding")
	pdf.Ln(6)

	pdf.SetFont("Arial", "", 11)
	pdf.Cell(contentW/4, 5, fmt.Sprintf("%d", view.TotalActivities))
	pdf.Cell(contentW/4, 5, p.formatRupiah(view.PurchaseTotalAmount))
	pdf.Cell(contentW/4, 5, p.formatRupiah(view.PaymentTotalAmount))
	pdf.Cell(contentW/4, 5, p.formatRupiah(view.PurchaseTotalOutstanding))
	pdf.Ln(15)

	pdf.SetFillColor(252, 246, 240)
	pdf.SetDrawColor(235, 210, 190)
	noteY := pdf.GetY()
	pdf.Rect(lm, noteY, contentW, 10, "FD")
	pdf.SetXY(lm+2, noteY+2.2)
	pdf.SetFont("Arial", "", 8)
	pdf.MultiCell(contentW-4, 4.2, "Payments are shown as settlement activity and are not counted as new purchases.", "", "L", false)
	pdf.Ln(2)

	ensureSpace := func(height float64) {
		if pdf.GetY()+height > 280 {
			pdf.AddPage()
		}
	}

	purchaseWidths := []float64{18, 28, 50, 22, 22, 22, 18}
	drawPurchaseSectionHeader := func(continued bool) {
		title := "PURCHASE DOCUMENTS"
		if continued {
			title += " (continued)"
		}
		pdf.SetFont("Arial", "B", 10)
		pdf.SetTextColor(119, 63, 0)
		pdf.Cell(contentW, 6, title)
		pdf.SetTextColor(0, 0, 0)
		pdf.Ln(7)

		pdf.SetFont("Arial", "B", 7)
		pdf.SetFillColor(220, 220, 220)
		pdf.CellFormat(purchaseWidths[0], 7, "Date", "1", 0, "C", true, 0, "")
		pdf.CellFormat(purchaseWidths[1], 7, "Code", "1", 0, "C", true, 0, "")
		pdf.CellFormat(purchaseWidths[2], 7, "Description", "1", 0, "L", true, 0, "")
		pdf.CellFormat(purchaseWidths[3], 7, "Amount", "1", 0, "R", true, 0, "")
		pdf.CellFormat(purchaseWidths[4], 7, "Paid", "1", 0, "R", true, 0, "")
		pdf.CellFormat(purchaseWidths[5], 7, "Outstanding", "1", 0, "R", true, 0, "")
		pdf.CellFormat(purchaseWidths[6], 7, "Status", "1", 0, "C", true, 0, "")
		pdf.Ln(7)
	}

	paymentWidths := []float64{18, 28, 58, 24, 24, 28}
	drawPaymentSectionHeader := func(continued bool) {
		title := "PAYMENTS"
		if continued {
			title += " (continued)"
		}
		pdf.SetFont("Arial", "B", 10)
		pdf.SetTextColor(119, 63, 0)
		pdf.Cell(contentW, 6, title)
		pdf.SetTextColor(0, 0, 0)
		pdf.Ln(7)

		pdf.SetFont("Arial", "B", 7)
		pdf.SetFillColor(220, 220, 220)
		pdf.CellFormat(paymentWidths[0], 7, "Date", "1", 0, "C", true, 0, "")
		pdf.CellFormat(paymentWidths[1], 7, "Code", "1", 0, "C", true, 0, "")
		pdf.CellFormat(paymentWidths[2], 7, "Description", "1", 0, "L", true, 0, "")
		pdf.CellFormat(paymentWidths[3], 7, "Reference", "1", 0, "L", true, 0, "")
		pdf.CellFormat(paymentWidths[4], 7, "Amount", "1", 0, "R", true, 0, "")
		pdf.CellFormat(paymentWidths[5], 7, "Status", "1", 0, "C", true, 0, "")
		pdf.Ln(7)
	}

	if len(view.PurchaseTransactions) > 0 {
		ensureSpace(20)
		drawPurchaseSectionHeader(false)
		pdf.SetFont("Arial", "", 6.5)
		for _, tx := range view.PurchaseTransactions {
			if pdf.GetY()+6 > 280 {
				pdf.AddPage()
				drawPurchaseSectionHeader(true)
				pdf.SetFont("Arial", "", 6.5)
			}
			pdf.CellFormat(purchaseWidths[0], 6, exportHistoryDate(getString(tx, "date")), "1", 0, "L", false, 0, "")
			pdf.CellFormat(purchaseWidths[1], 6, getString(tx, "transaction_code"), "1", 0, "L", false, 0, "")
			pdf.CellFormat(purchaseWidths[2], 6, exportHistoryTrim(getString(tx, "description"), 42), "1", 0, "L", false, 0, "")
			pdf.CellFormat(purchaseWidths[3], 6, p.formatRupiahCompact(getNumFrom(tx["amount"])), "1", 0, "R", false, 0, "")
			pdf.CellFormat(purchaseWidths[4], 6, p.formatRupiahCompact(getNumFrom(tx["paid_amount"])), "1", 0, "R", false, 0, "")
			pdf.CellFormat(purchaseWidths[5], 6, p.formatRupiahCompact(getNumFrom(tx["outstanding"])), "1", 0, "R", false, 0, "")
			pdf.CellFormat(purchaseWidths[6], 6, getString(tx, "status"), "1", 0, "C", false, 0, "")
			pdf.Ln(6)
		}

		ensureSpace(8)
		pdf.SetFont("Arial", "B", 7)
		pdf.SetFillColor(240, 240, 240)
		leftGroup := purchaseWidths[0] + purchaseWidths[1] + purchaseWidths[2]
		pdf.CellFormat(leftGroup, 6.5, "TOTAL PURCHASES", "1", 0, "R", true, 0, "")
		pdf.CellFormat(purchaseWidths[3], 6.5, p.formatRupiahCompact(view.PurchaseTotalAmount), "1", 0, "R", true, 0, "")
		pdf.CellFormat(purchaseWidths[4], 6.5, p.formatRupiahCompact(view.PurchaseTotalPaid), "1", 0, "R", true, 0, "")
		pdf.CellFormat(purchaseWidths[5], 6.5, p.formatRupiahCompact(view.PurchaseTotalOutstanding), "1", 0, "R", true, 0, "")
		pdf.CellFormat(purchaseWidths[6], 6.5, "", "1", 0, "C", true, 0, "")
		pdf.Ln(10)
	}

	if len(view.PaymentTransactions) > 0 {
		ensureSpace(20)
		drawPaymentSectionHeader(false)
		pdf.SetFont("Arial", "", 6.5)
		for _, tx := range view.PaymentTransactions {
			if pdf.GetY()+6 > 280 {
				pdf.AddPage()
				drawPaymentSectionHeader(true)
				pdf.SetFont("Arial", "", 6.5)
			}
			pdf.CellFormat(paymentWidths[0], 6, exportHistoryDate(getString(tx, "date")), "1", 0, "L", false, 0, "")
			pdf.CellFormat(paymentWidths[1], 6, getString(tx, "transaction_code"), "1", 0, "L", false, 0, "")
			pdf.CellFormat(paymentWidths[2], 6, exportHistoryTrim(getString(tx, "description"), 52), "1", 0, "L", false, 0, "")
			pdf.CellFormat(paymentWidths[3], 6, exportHistoryTrim(getString(tx, "reference"), 18), "1", 0, "L", false, 0, "")
			pdf.CellFormat(paymentWidths[4], 6, p.formatRupiahCompact(getNumFrom(tx["amount"])), "1", 0, "R", false, 0, "")
			pdf.CellFormat(paymentWidths[5], 6, getString(tx, "status"), "1", 0, "C", false, 0, "")
			pdf.Ln(6)
		}

		ensureSpace(8)
		pdf.SetFont("Arial", "B", 7)
		pdf.SetFillColor(240, 240, 240)
		leftGroup := paymentWidths[0] + paymentWidths[1] + paymentWidths[2] + paymentWidths[3]
		pdf.CellFormat(leftGroup, 6.5, "TOTAL PAYMENTS", "1", 0, "R", true, 0, "")
		pdf.CellFormat(paymentWidths[4], 6.5, p.formatRupiahCompact(view.PaymentTotalAmount), "1", 0, "R", true, 0, "")
		pdf.CellFormat(paymentWidths[5], 6.5, "", "1", 0, "C", true, 0, "")
		pdf.Ln(10)
	}

	// Footer
	pdf.Ln(10)
	pdf.SetFont("Arial", "I", 8)
	pdf.Cell(contentW, 4, fmt.Sprintf("Report generated on %s", time.Now().Format("02/01/2006 15:04")))

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to generate vendor history PDF: %v", err)
	}
	return buf.Bytes(), nil
}

// getString helper to get string from map with fallback
func getString(m map[string]interface{}, key string) string {
	if val, exists := m[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}
