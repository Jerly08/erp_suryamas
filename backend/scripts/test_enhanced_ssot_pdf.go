package main

import (
	"app-sistem-akuntansi/services"
	"fmt"
	"log"
	"os"
	"time"
)

// SSOTProfitLossData for testing (enhanced version with account details)
type SSOTProfitLossData struct {
	Company   CompanyInfo `json:"company"`
	StartDate time.Time   `json:"start_date"`
	EndDate   time.Time   `json:"end_date"`
	Currency  string      `json:"currency"`

	Revenue struct {
		TotalRevenue float64       `json:"total_revenue"`
		Items        []AccountItem `json:"items"`
	} `json:"revenue"`

	COGS struct {
		TotalCOGS float64       `json:"total_cogs"`
		Items     []AccountItem `json:"items"`
	} `json:"cost_of_goods_sold"`

	GrossProfit       float64 `json:"gross_profit"`
	GrossProfitMargin float64 `json:"gross_profit_margin"`

	OperatingExpenses struct {
		TotalOpEx      float64 `json:"total_opex"`
		Administrative struct {
			Items []AccountItem `json:"items"`
		} `json:"administrative"`
		SellingMarketing struct {
			Items []AccountItem `json:"items"`
		} `json:"selling_marketing"`
		General struct {
			Items []AccountItem `json:"items"`
		} `json:"general"`
	} `json:"operating_expenses"`

	OperatingIncome float64 `json:"operating_income"`
	OperatingMargin float64 `json:"operating_margin"`

	OtherIncome       float64       `json:"other_income"`
	OtherExpenses     float64       `json:"other_expenses"`
	OtherIncomeItems  []AccountItem `json:"other_income_items"`
	OtherExpenseItems []AccountItem `json:"other_expense_items"`

	IncomeBeforeTax float64 `json:"income_before_tax"`
	TaxExpense      float64 `json:"tax_expense"`
	NetIncome       float64 `json:"net_income"`
	NetIncomeMargin float64 `json:"net_income_margin"`

	GeneratedAt time.Time `json:"generated_at"`
}

type CompanyInfo struct {
	Name string `json:"name"`
}

type AccountItem struct {
	AccountCode string  `json:"account_code"`
	AccountName string  `json:"account_name"`
	Amount      float64 `json:"amount"`
}

func main() {
	// Create PDF service without database for testing
	pdfService := services.NewPDFService(nil)

	// Create sample SSOT data for testing with account details
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

	// Set revenue with account details
	sampleData.Revenue.TotalRevenue = 200000000
	sampleData.Revenue.Items = []AccountItem{
		{AccountCode: "4101", AccountName: "Sales Revenue", Amount: 180000000},
		{AccountCode: "4201", AccountName: "Service Revenue", Amount: 20000000},
	}

	// Set COGS with account details
	sampleData.COGS.TotalCOGS = 150000000
	sampleData.COGS.Items = []AccountItem{
		{AccountCode: "5101", AccountName: "Direct Materials", Amount: 100000000},
		{AccountCode: "5111", AccountName: "Direct Labor", Amount: 30000000},
		{AccountCode: "5121", AccountName: "Manufacturing Overhead", Amount: 20000000},
	}

	// Set operating expenses with account details
	sampleData.OperatingExpenses.TotalOpEx = 20000000
	sampleData.OperatingExpenses.Administrative.Items = []AccountItem{
		{AccountCode: "5201", AccountName: "Office Supplies", Amount: 5000000},
		{AccountCode: "5202", AccountName: "Utilities", Amount: 3000000},
	}
	sampleData.OperatingExpenses.SellingMarketing.Items = []AccountItem{
		{AccountCode: "5301", AccountName: "Advertising", Amount: 7000000},
		{AccountCode: "5302", AccountName: "Sales Commissions", Amount: 3000000},
	}
	sampleData.OperatingExpenses.General.Items = []AccountItem{
		{AccountCode: "5401", AccountName: "Insurance", Amount: 1000000},
		{AccountCode: "5402", AccountName: "Maintenance", Amount: 1000000},
	}

	// Set other income and expenses with account details
	sampleData.OtherIncome = 5000000
	sampleData.OtherExpenses = 2000000
	sampleData.OtherIncomeItems = []AccountItem{
		{AccountCode: "7001", AccountName: "Interest Income", Amount: 3000000},
		{AccountCode: "7002", AccountName: "Gain on Sale of Assets", Amount: 2000000},
	}
	sampleData.OtherExpenseItems = []AccountItem{
		{AccountCode: "6001", AccountName: "Interest Expense", Amount: 1000000},
		{AccountCode: "6002", AccountName: "Loss on Disposal", Amount: 1000000},
	}

	// Generate PDF
	fmt.Println("Generating Enhanced SSOT P&L PDF with Account Details...")
	pdfBytes, err := pdfService.GenerateSSOTProfitLossPDF(sampleData, "id")
	if err != nil {
		log.Fatal("Failed to generate PDF:", err)
	}

	// Save PDF to file
	filename := "test_enhanced_ssot_pl_report.pdf"
	err = os.WriteFile(filename, pdfBytes, 0644)
	if err != nil {
		log.Fatal("Failed to save PDF:", err)
	}

	fmt.Printf("✅ Enhanced PDF generated successfully: %s\n", filename)
	fmt.Printf("📄 PDF size: %d bytes\n", len(pdfBytes))
	fmt.Println("🔍 You can open the PDF file to verify the enhanced content with account details")
}
