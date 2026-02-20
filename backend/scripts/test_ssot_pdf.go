package main

import (
	"app-sistem-akuntansi/services"
	"fmt"
	"log"
	"os"
	"time"
)

// SSOTProfitLossData for testing (simplified version)
type SSOTProfitLossData struct {
	Company   CompanyInfo `json:"company"`
	StartDate time.Time   `json:"start_date"`
	EndDate   time.Time   `json:"end_date"`
	Currency  string      `json:"currency"`

	Revenue struct {
		TotalRevenue float64 `json:"total_revenue"`
	} `json:"revenue"`

	COGS struct {
		TotalCOGS float64 `json:"total_cogs"`
	} `json:"cost_of_goods_sold"`

	GrossProfit       float64 `json:"gross_profit"`
	GrossProfitMargin float64 `json:"gross_profit_margin"`

	OperatingExpenses struct {
		TotalOpEx float64 `json:"total_opex"`
	} `json:"operating_expenses"`

	OperatingIncome float64 `json:"operating_income"`
	OperatingMargin float64 `json:"operating_margin"`

	OtherIncome   float64 `json:"other_income"`
	OtherExpenses float64 `json:"other_expenses"`

	IncomeBeforeTax float64 `json:"income_before_tax"`
	TaxExpense      float64 `json:"tax_expense"`
	NetIncome       float64 `json:"net_income"`
	NetIncomeMargin float64 `json:"net_income_margin"`

	GeneratedAt time.Time `json:"generated_at"`
}

type CompanyInfo struct {
	Name string `json:"name"`
}

func main() {
	// Create PDF service without database for testing
	pdfService := services.NewPDFService(nil)

	// Create sample SSOT data for testing
	sampleData := &SSOTProfitLossData{
		Company: CompanyInfo{
			Name: "PT. Test Company",
		},
		StartDate:         time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:           time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
		Currency:          "IDR",
		GrossProfit:       50000000,
		GrossProfitMargin: 25.0,
		OperatingIncome:   30000000,
		OperatingMargin:   15.0,
		NetIncome:         20000000,
		NetIncomeMargin:   10.0,
		IncomeBeforeTax:   25000000,
		TaxExpense:        5000000,
		GeneratedAt:       time.Now(),
	}

	// Set revenue and expenses
	sampleData.Revenue.TotalRevenue = 200000000
	sampleData.COGS.TotalCOGS = 150000000
	sampleData.OperatingExpenses.TotalOpEx = 20000000
	sampleData.OtherIncome = 5000000
	sampleData.OtherExpenses = 2000000

	// Generate PDF
	fmt.Println("Generating SSOT P&L PDF...")
	pdfBytes, err := pdfService.GenerateSSOTProfitLossPDF(sampleData, "id")
	if err != nil {
		log.Fatal("Failed to generate PDF:", err)
	}

	// Save PDF to file
	filename := "test_ssot_pl_report.pdf"
	err = os.WriteFile(filename, pdfBytes, 0644)
	if err != nil {
		log.Fatal("Failed to save PDF:", err)
	}

	fmt.Printf("✅ PDF generated successfully: %s\n", filename)
	fmt.Printf("📄 PDF size: %d bytes\n", len(pdfBytes))
	fmt.Println("🔍 You can open the PDF file to verify the content")
}
