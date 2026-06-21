package services

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/utils"
	"github.com/jung-kurt/gofpdf"
	"gorm.io/gorm"
)

// CashFlowExportService handles export functionality for Cash Flow reports
type CashFlowExportService struct {
	db *gorm.DB
}

// NewCashFlowExportService creates a new cash flow export service
func NewCashFlowExportService(db *gorm.DB) *CashFlowExportService {
	return &CashFlowExportService{db: db}
}

// getCompanyInfo retrieves company info from settings table, with sensible defaults
func (s *CashFlowExportService) getCompanyInfo() *models.Settings {
	if s.db == nil {
		return &models.Settings{
			CompanyName:    "PT. Sistem Akuntansi",
			CompanyAddress: "",
			CompanyPhone:   "",
			CompanyEmail:   "",
		}
	}
	var settings models.Settings
	if err := s.db.First(&settings).Error; err != nil {
		return &models.Settings{
			CompanyName:    "PT. Sistem Akuntansi",
			CompanyAddress: "",
			CompanyPhone:   "",
			CompanyEmail:   "",
		}
	}
	return &settings
}

// ExportToCSV exports cash flow data to CSV format with localization
func (s *CashFlowExportService) ExportToCSV(data *SSOTCashFlowData, userID uint) ([]byte, error) {
	utils.SanitizeExportData(data)
	// Get user language preference
	language := utils.GetUserLanguageFromSettings(s.db)
	var buf bytes.Buffer
	writer := utils.NewSanitizedCSVWriter(&buf)

	// Write header information with localization
	writer.Write([]string{utils.T("cash_flow_statement", language)})
	writer.Write([]string{data.Company.Name})
	writer.Write([]string{utils.T("period", language) + ":", data.StartDate.Format("02/01/2006") + " - " + data.EndDate.Format("02/01/2006")})
	writer.Write([]string{utils.T("generated_on", language) + ":", data.GeneratedAt.Format("02/01/2006 15:04")})
	writer.Write([]string{}) // Empty row

	// CSV Headers with localization
	headers := utils.GetCSVHeaders("cash_flow", language)
	writer.Write(headers)

	// Operating Activities with localization
	writer.Write([]string{utils.T("operating_activities", language), "", "", "", "", ""})

	// Net Income with localization
	writer.Write([]string{
		"Operating",
		utils.T("net_income", language),
		"",
		utils.T("net_income", language),
		s.formatAmount(data.OperatingActivities.NetIncome),
		"base",
	})

	// Adjustments with localization
	if len(data.OperatingActivities.Adjustments.Items) > 0 {
		writer.Write([]string{"Operating", utils.T("adjustments_non_cash", language), "", "", "", ""})
		for _, item := range data.OperatingActivities.Adjustments.Items {
			writer.Write([]string{
				"Operating",
				"Adjustments",
				item.AccountCode,
				item.AccountName,
				s.formatAmount(item.Amount),
				item.Type,
			})
		}
		writer.Write([]string{
			"Operating",
			utils.T("total", language) + " " + utils.T("adjustments_non_cash", language),
			"",
			"",
			s.formatAmount(data.OperatingActivities.Adjustments.TotalAdjustments),
			"subtotal",
		})
	}

	// Working Capital Changes with localization
	if len(data.OperatingActivities.WorkingCapitalChanges.Items) > 0 {
		writer.Write([]string{"Operating", utils.T("working_capital_changes", language), "", "", "", ""})
		for _, item := range data.OperatingActivities.WorkingCapitalChanges.Items {
			writer.Write([]string{
				"Operating",
				"Working Capital",
				item.AccountCode,
				item.AccountName,
				s.formatAmount(item.Amount),
				item.Type,
			})
		}
		writer.Write([]string{
			"Operating",
			utils.T("total", language) + " " + utils.T("working_capital_changes", language),
			"",
			"",
			s.formatAmount(data.OperatingActivities.WorkingCapitalChanges.TotalWorkingCapitalChanges),
			"subtotal",
		})
	}

	writer.Write([]string{
		"Operating",
		utils.T("net_cash_operating", language),
		"",
		"",
		s.formatAmount(data.OperatingActivities.TotalOperatingCashFlow),
		"total",
	})
	writer.Write([]string{}) // Empty row

	// Investing Activities with localization
	writer.Write([]string{utils.T("investing_activities", language), "", "", "", "", ""})
	if len(data.InvestingActivities.Items) > 0 {
		for _, item := range data.InvestingActivities.Items {
			writer.Write([]string{
				"Investing",
				"Investing Activities",
				item.AccountCode,
				item.AccountName,
				s.formatAmount(item.Amount),
				item.Type,
			})
		}
	}
	writer.Write([]string{
		"Investing",
		utils.T("net_cash_investing", language),
		"",
		"",
		s.formatAmount(data.InvestingActivities.TotalInvestingCashFlow),
		"total",
	})
	writer.Write([]string{}) // Empty row

	// Financing Activities with localization
	writer.Write([]string{utils.T("financing_activities", language), "", "", "", "", ""})
	if len(data.FinancingActivities.Items) > 0 {
		for _, item := range data.FinancingActivities.Items {
			writer.Write([]string{
				"Financing",
				"Financing Activities",
				item.AccountCode,
				item.AccountName,
				s.formatAmount(item.Amount),
				item.Type,
			})
		}
	}
	writer.Write([]string{
		"Financing",
		utils.T("net_cash_financing", language),
		"",
		"",
		s.formatAmount(data.FinancingActivities.TotalFinancingCashFlow),
		"total",
	})
	writer.Write([]string{}) // Empty row

	// Summary with localization
	writer.Write([]string{utils.T("cash_flow_summary", language), "", "", "", "", ""})
	writer.Write([]string{
		"Summary",
		utils.T("cash_beginning", language),
		"",
		"",
		s.formatAmount(data.CashAtBeginning),
		"summary",
	})
	writer.Write([]string{
		"Summary",
		utils.T("net_cash_flow", language),
		"",
		"",
		s.formatAmount(data.NetCashFlow),
		"summary",
	})
	writer.Write([]string{
		"Summary",
		"Cash at End of Period",
		"",
		"",
		s.formatAmount(data.CashAtEnd),
		"summary",
	})

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("failed to write CSV: %v", err)
	}

	return buf.Bytes(), nil
}

// ExportToPDF exports cash flow data to PDF format (structured like trial balance)
func (s *CashFlowExportService) ExportToPDF(data *SSOTCashFlowData) ([]byte, error) {
	utils.SanitizeExportData(data)
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// Layout helpers
	lm, tm, rm, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	contentW := pageW - lm - rm

	// Company info
	settings := s.getCompanyInfo()

	// Header: logo left, text right
	logoX, logoY, logoSize := lm, tm, 35.0
	logoAdded := false
	if strings.TrimSpace(settings.CompanyLogo) != "" {
		logoPath := settings.CompanyLogo
		if strings.HasPrefix(logoPath, "/") {
			logoPath = "." + logoPath
		}
		if _, err := os.Stat(logoPath); err == nil {
			if imgType := detectImageType(logoPath); imgType != "" {
				pdf.ImageOptions(logoPath, logoX, logoY, logoSize, 0, false, gofpdf.ImageOptions{ImageType: imgType}, 0, "")
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

	companyName := strings.TrimSpace(settings.CompanyName)
	if companyName == "" {
		companyName = data.Company.Name
	}
	pdf.SetFont("Arial", "B", 12)
	nameW := pdf.GetStringWidth(companyName)
	pdf.SetXY(pageW-rm-nameW, tm)
	pdf.Cell(nameW, 6, companyName)

	pdf.SetFont("Arial", "", 9)
	addr := strings.TrimSpace(settings.CompanyAddress)
	if addr == "" {
		addr = strings.TrimSpace(data.Company.Address)
	}
	if addr != "" {
		addrW := pdf.GetStringWidth(addr)
		pdf.SetXY(pageW-rm-addrW, tm+8)
		pdf.Cell(addrW, 4, addr)
	}

	if settings.CompanyPhone != "" {
		phoneText := fmt.Sprintf("Phone: %s", settings.CompanyPhone)
		phoneW := pdf.GetStringWidth(phoneText)
		pdf.SetXY(pageW-rm-phoneW, tm+14)
		pdf.Cell(phoneW, 4, phoneText)
	}

	// Divider line
	pdf.SetDrawColor(238, 238, 238)
	pdf.SetLineWidth(0.2)
	pdf.Line(lm, tm+45, pageW-rm, tm+45)

	// Title
	pdf.SetY(tm + 55)
	pdf.SetX(lm)
	pdf.SetFont("Arial", "B", 22)
	pdf.SetTextColor(51, 51, 51)
	pdf.Cell(contentW, 10, "CASH FLOW STATEMENT")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(12)

	// Details two-column
	pdf.SetFont("Arial", "B", 9)
	pdf.SetX(lm)
	pdf.Cell(20, 5, "Period:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(60, 5, fmt.Sprintf("%s - %s", data.StartDate.Format("02/01/2006"), data.EndDate.Format("02/01/2006")))

	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(0, 0, 0)
	rightX := lm + contentW - 60
	pdf.SetX(rightX)
	pdf.Cell(26, 5, "Generated:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(34, 5, data.GeneratedAt.Format("02/01/2006 15:04"))
	pdf.Ln(12)

	// Table headers
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(220, 220, 220)
	pdf.CellFormat(25, 8, "Activity Type", "1", 0, "C", true, 0, "")
	pdf.CellFormat(75, 8, "Account Description", "1", 0, "L", true, 0, "")
	pdf.CellFormat(45, 8, "Amount", "1", 0, "R", true, 0, "")
	pdf.CellFormat(45, 8, "Section Total", "1", 0, "R", true, 0, "")
	pdf.Ln(8)

	// Process cash flow data
	pdf.SetFont("Arial", "", 8)
	pdf.SetFillColor(255, 255, 255)

	// Operating Activities
	if data.OperatingActivities.NetIncome != 0 ||
		len(data.OperatingActivities.Adjustments.Items) > 0 ||
		len(data.OperatingActivities.WorkingCapitalChanges.Items) > 0 {

		// Section header
		pdf.SetFont("Arial", "B", 9)
		pdf.SetFillColor(240, 240, 240)
		pdf.CellFormat(190, 6, "OPERATING ACTIVITIES", "1", 0, "L", true, 0, "")
		pdf.Ln(6)

		pdf.SetFont("Arial", "", 8)
		pdf.SetFillColor(255, 255, 255)

		// Net Income
		if data.OperatingActivities.NetIncome != 0 {
			pdf.CellFormat(25, 5, "Operating", "1", 0, "C", false, 0, "")
			pdf.CellFormat(75, 5, "Net Income", "1", 0, "L", false, 0, "")
			pdf.CellFormat(45, 5, s.formatAmountAsRupiah(data.OperatingActivities.NetIncome), "1", 0, "R", false, 0, "")
			pdf.CellFormat(45, 5, "", "1", 0, "R", false, 0, "")
			pdf.Ln(5)
		}

		// Adjustments for Non-Cash Items
		if len(data.OperatingActivities.Adjustments.Items) > 0 {
			// Section header for adjustments
			pdf.SetFont("Arial", "I", 8)
			pdf.CellFormat(190, 4, "Adjustments for Non-Cash Items:", "1", 0, "L", false, 0, "")
			pdf.Ln(4)

			pdf.SetFont("Arial", "", 8)
			for _, item := range data.OperatingActivities.Adjustments.Items {
				pdf.CellFormat(25, 5, "Adjustment", "1", 0, "C", false, 0, "")
				pdf.CellFormat(75, 5, fmt.Sprintf("%s - %s", item.AccountCode, item.AccountName), "1", 0, "L", false, 0, "")
				pdf.CellFormat(45, 5, s.formatAmountAsRupiah(item.Amount), "1", 0, "R", false, 0, "")
				pdf.CellFormat(45, 5, "", "1", 0, "R", false, 0, "")
				pdf.Ln(5)
			}

			// Total adjustments
			pdf.SetFont("Arial", "B", 8)
			pdf.CellFormat(100, 5, "Total Adjustments", "1", 0, "R", false, 0, "")
			pdf.CellFormat(45, 5, s.formatAmountAsRupiah(data.OperatingActivities.Adjustments.TotalAdjustments), "1", 0, "R", false, 0, "")
			pdf.CellFormat(45, 5, "", "1", 0, "R", false, 0, "")
			pdf.Ln(5)
		}

		// Changes in Working Capital
		if len(data.OperatingActivities.WorkingCapitalChanges.Items) > 0 {
			// Section header for working capital
			pdf.SetFont("Arial", "I", 8)
			pdf.CellFormat(190, 4, "Changes in Working Capital:", "1", 0, "L", false, 0, "")
			pdf.Ln(4)

			pdf.SetFont("Arial", "", 8)
			for _, item := range data.OperatingActivities.WorkingCapitalChanges.Items {
				pdf.CellFormat(25, 5, "Working Capital", "1", 0, "C", false, 0, "")
				pdf.CellFormat(75, 5, fmt.Sprintf("%s - %s", item.AccountCode, item.AccountName), "1", 0, "L", false, 0, "")
				pdf.CellFormat(45, 5, s.formatAmountAsRupiah(item.Amount), "1", 0, "R", false, 0, "")
				pdf.CellFormat(45, 5, "", "1", 0, "R", false, 0, "")
				pdf.Ln(5)
			}

			// Total working capital changes
			pdf.SetFont("Arial", "B", 8)
			pdf.CellFormat(100, 5, "Total Working Capital Changes", "1", 0, "R", false, 0, "")
			pdf.CellFormat(45, 5, s.formatAmountAsRupiah(data.OperatingActivities.WorkingCapitalChanges.TotalWorkingCapitalChanges), "1", 0, "R", false, 0, "")
			pdf.CellFormat(45, 5, "", "1", 0, "R", false, 0, "")
			pdf.Ln(5)
		}

		// Net cash from operating activities
		pdf.SetFont("Arial", "B", 9)
		pdf.SetFillColor(240, 240, 240)
		pdf.CellFormat(145, 6, "NET CASH FROM OPERATING ACTIVITIES", "1", 0, "R", true, 0, "")
		pdf.CellFormat(45, 6, s.formatAmountAsRupiah(data.OperatingActivities.TotalOperatingCashFlow), "1", 0, "R", true, 0, "")
		pdf.Ln(8)
	}

	// Investing Activities
	if data.InvestingActivities.TotalInvestingCashFlow != 0 || len(data.InvestingActivities.Items) > 0 {
		// Section header
		pdf.SetFont("Arial", "B", 9)
		pdf.SetFillColor(240, 240, 240)
		pdf.CellFormat(190, 6, "INVESTING ACTIVITIES", "1", 0, "L", true, 0, "")
		pdf.Ln(6)

		pdf.SetFont("Arial", "", 8)
		pdf.SetFillColor(255, 255, 255)

		if len(data.InvestingActivities.Items) > 0 {
			for _, item := range data.InvestingActivities.Items {
				pdf.CellFormat(25, 5, "Investing", "1", 0, "C", false, 0, "")
				pdf.CellFormat(75, 5, fmt.Sprintf("%s - %s", item.AccountCode, item.AccountName), "1", 0, "L", false, 0, "")
				pdf.CellFormat(45, 5, s.formatAmountAsRupiah(item.Amount), "1", 0, "R", false, 0, "")
				pdf.CellFormat(45, 5, "", "1", 0, "R", false, 0, "")
				pdf.Ln(5)
			}
		} else {
			pdf.CellFormat(25, 5, "Investing", "1", 0, "C", false, 0, "")
			pdf.CellFormat(75, 5, "No investing activities", "1", 0, "L", false, 0, "")
			pdf.CellFormat(45, 5, "-", "1", 0, "R", false, 0, "")
			pdf.CellFormat(45, 5, "", "1", 0, "R", false, 0, "")
			pdf.Ln(5)
		}

		// Net cash from investing activities
		pdf.SetFont("Arial", "B", 9)
		pdf.SetFillColor(240, 240, 240)
		pdf.CellFormat(145, 6, "NET CASH FROM INVESTING ACTIVITIES", "1", 0, "R", true, 0, "")
		pdf.CellFormat(45, 6, s.formatAmountAsRupiah(data.InvestingActivities.TotalInvestingCashFlow), "1", 0, "R", true, 0, "")
		pdf.Ln(8)
	}

	// Financing Activities
	if data.FinancingActivities.TotalFinancingCashFlow != 0 || len(data.FinancingActivities.Items) > 0 {
		// Section header
		pdf.SetFont("Arial", "B", 9)
		pdf.SetFillColor(240, 240, 240)
		pdf.CellFormat(190, 6, "FINANCING ACTIVITIES", "1", 0, "L", true, 0, "")
		pdf.Ln(6)

		pdf.SetFont("Arial", "", 8)
		pdf.SetFillColor(255, 255, 255)

		if len(data.FinancingActivities.Items) > 0 {
			for _, item := range data.FinancingActivities.Items {
				pdf.CellFormat(25, 5, "Financing", "1", 0, "C", false, 0, "")
				pdf.CellFormat(75, 5, fmt.Sprintf("%s - %s", item.AccountCode, item.AccountName), "1", 0, "L", false, 0, "")
				pdf.CellFormat(45, 5, s.formatAmountAsRupiah(item.Amount), "1", 0, "R", false, 0, "")
				pdf.CellFormat(45, 5, "", "1", 0, "R", false, 0, "")
				pdf.Ln(5)
			}
		} else {
			pdf.CellFormat(25, 5, "Financing", "1", 0, "C", false, 0, "")
			pdf.CellFormat(75, 5, "No financing activities", "1", 0, "L", false, 0, "")
			pdf.CellFormat(45, 5, "-", "1", 0, "R", false, 0, "")
			pdf.CellFormat(45, 5, "", "1", 0, "R", false, 0, "")
			pdf.Ln(5)
		}

		// Net cash from financing activities
		pdf.SetFont("Arial", "B", 9)
		pdf.SetFillColor(240, 240, 240)
		pdf.CellFormat(145, 6, "NET CASH FROM FINANCING ACTIVITIES", "1", 0, "R", true, 0, "")
		pdf.CellFormat(45, 6, s.formatAmountAsRupiah(data.FinancingActivities.TotalFinancingCashFlow), "1", 0, "R", true, 0, "")
		pdf.Ln(8)
	}

	// Summary section
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(200, 200, 200)
	pdf.CellFormat(190, 6, "NET CASH FLOW SUMMARY", "1", 0, "L", true, 0, "")
	pdf.Ln(6)

	pdf.SetFont("Arial", "", 8)
	pdf.SetFillColor(255, 255, 255)

	// Cash at beginning
	pdf.CellFormat(100, 5, "Cash at Beginning of Period", "1", 0, "L", false, 0, "")
	pdf.CellFormat(45, 5, "", "1", 0, "R", false, 0, "")
	pdf.CellFormat(45, 5, s.formatAmountAsRupiah(data.CashAtBeginning), "1", 0, "R", false, 0, "")
	pdf.Ln(5)

	// Net cash from activities
	pdf.CellFormat(100, 5, "Net Cash Flow from Activities", "1", 0, "L", false, 0, "")
	pdf.CellFormat(45, 5, "", "1", 0, "R", false, 0, "")
	pdf.CellFormat(45, 5, s.formatAmountAsRupiah(data.NetCashFlow), "1", 0, "R", false, 0, "")
	pdf.Ln(5)

	// Total section
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(240, 240, 240)
	pdf.CellFormat(145, 6, "Cash at End of Period", "1", 0, "R", true, 0, "")
	pdf.CellFormat(45, 6, s.formatAmountAsRupiah(data.CashAtEnd), "1", 0, "R", true, 0, "")
	pdf.Ln(8)

	// Footer
	pdf.Ln(10)
	pdf.SetFont("Arial", "I", 8)
	pdf.Cell(190, 4, fmt.Sprintf("Report generated on %s", time.Now().Format("02/01/2006 15:04")))

	// Output to buffer
	var buf bytes.Buffer
	err := pdf.Output(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %v", err)
	}

	return buf.Bytes(), nil
}

// formatAmount formats amount for CSV export
func (s *CashFlowExportService) formatAmount(amount float64) string {
	// Format with thousand separators and 2 decimal places
	return fmt.Sprintf("%.2f", amount)
}

// formatAmountPDF formats amount for PDF export with Indonesian formatting
func (s *CashFlowExportService) formatAmountPDF(amount float64) string {
	// Convert to string with 2 decimal places
	str := fmt.Sprintf("%.2f", amount)

	// Add thousand separators
	parts := strings.Split(str, ".")
	intPart := parts[0]
	decPart := parts[1]

	// Add commas for thousands
	n := len(intPart)
	if n > 3 {
		var result strings.Builder
		for i, char := range intPart {
			if i > 0 && (n-i)%3 == 0 {
				result.WriteString(",")
			}
			result.WriteRune(char)
		}
		return fmt.Sprintf("%s.%s", result.String(), decPart)
	}

	return str
}

// formatAmountAsRupiah formats amount in Indonesian Rupiah format like the trial balance
func (s *CashFlowExportService) formatAmountAsRupiah(amount float64) string {
	// Format number with thousand separators
	amountStr := fmt.Sprintf("%.0f", amount)
	if amount != float64(int64(amount)) {
		amountStr = fmt.Sprintf("%.2f", amount)
	}

	// Add thousand separators
	var result strings.Builder
	negative := false
	if amountStr[0] == '-' {
		negative = true
		amountStr = amountStr[1:]
	}

	parts := strings.Split(amountStr, ".")
	intPart := parts[0]
	decPart := ""
	if len(parts) > 1 {
		decPart = parts[1]
	}

	// Add thousand separators to integer part
	n := len(intPart)
	for i, char := range intPart {
		if i > 0 && (n-i)%3 == 0 {
			result.WriteString(",")
		}
		result.WriteRune(char)
	}

	// Combine parts
	if decPart != "" {
		result.WriteString(".")
		result.WriteString(decPart)
	}

	// Add currency prefix
	if negative {
		return "Rp -" + result.String()
	}
	return "Rp " + result.String()
}

// GetCSVFilename generates appropriate filename for CSV export
func (s *CashFlowExportService) GetCSVFilename(data *SSOTCashFlowData) string {
	return fmt.Sprintf("cash_flow_%s_to_%s.csv",
		data.StartDate.Format("2006-01-02"),
		data.EndDate.Format("2006-01-02"))
}

// GetPDFFilename generates appropriate filename for PDF export
func (s *CashFlowExportService) GetPDFFilename(data *SSOTCashFlowData) string {
	return fmt.Sprintf("cash_flow_%s_to_%s.pdf",
		data.StartDate.Format("2006-01-02"),
		data.EndDate.Format("2006-01-02"))
}
