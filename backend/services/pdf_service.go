package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/utils"

	"github.com/jung-kurt/gofpdf"
	"gorm.io/gorm"
)

// PDFService implements PDFServiceInterface
type PDFService struct {
	db *gorm.DB
}

type salePDFSummary struct {
	Subtotal           float64
	DiscountAmount     float64
	TaxableAmount      float64
	PPNRate            float64
	PPNAmount          float64
	PPhPercent         float64
	PPhAmount          float64
	PPh21Rate          float64
	PPh21Amount        float64
	PPh23Rate          float64
	PPh23Amount        float64
	OtherTaxAdditions  float64
	OtherTaxDeductions float64
	ShippingCost       float64
	TotalAmount        float64
}

func isEffectivelyZero(value float64) bool {
	return math.Abs(value) < 0.005
}

func firstMeaningfulValue(values ...float64) float64 {
	for _, value := range values {
		if !isEffectivelyZero(value) {
			return value
		}
	}
	return 0
}

// buildSalePDFSummary prefers persisted tax amounts over recomputing from default rates.
// This keeps commercial documents aligned with the actual transaction that was saved.
func (p *PDFService) buildSalePDFSummary(sale *models.Sale, subtotal float64) salePDFSummary {
	discountAmount := sale.DiscountAmount
	if isEffectivelyZero(discountAmount) && sale.DiscountPercent > 0 {
		discountAmount = subtotal * sale.DiscountPercent / 100
	}

	taxableAmount := sale.TaxableAmount
	if isEffectivelyZero(taxableAmount) {
		taxableAmount = subtotal - discountAmount
	}

	ppnRate := sale.PPNRate
	if isEffectivelyZero(ppnRate) {
		ppnRate = sale.PPNPercent
	}

	summary := salePDFSummary{
		Subtotal:           subtotal,
		DiscountAmount:     discountAmount,
		TaxableAmount:      taxableAmount,
		PPNRate:            ppnRate,
		PPNAmount:          firstMeaningfulValue(sale.PPNAmount, sale.PPN),
		PPhPercent:         sale.PPhPercent,
		PPhAmount:          sale.PPh,
		PPh21Rate:          sale.PPh21Rate,
		PPh21Amount:        sale.PPh21Amount,
		PPh23Rate:          sale.PPh23Rate,
		PPh23Amount:        sale.PPh23Amount,
		OtherTaxAdditions:  sale.OtherTaxAdditions,
		OtherTaxDeductions: sale.OtherTaxDeductions,
		ShippingCost:       sale.ShippingCost,
		TotalAmount:        sale.TotalAmount,
	}

	if isEffectivelyZero(summary.TotalAmount) {
		summary.TotalAmount = taxableAmount +
			summary.PPNAmount +
			summary.OtherTaxAdditions +
			summary.ShippingCost -
			summary.PPhAmount -
			summary.PPh21Amount -
			summary.PPh23Amount -
			summary.OtherTaxDeductions
	}

	return summary
}

func (p *PDFService) renderSalePaymentSchedule(pdf *gofpdf.Fpdf, sale *models.Sale, tr PDFTranslations) {
	if sale == nil || strings.ToUpper(strings.TrimSpace(sale.PaymentTerms)) != "CUSTOM" || len(sale.PaymentSchedules) == 0 {
		return
	}

	pageW, _ := pdf.GetPageSize()
	lm, _, rm, _ := pdf.GetMargins()
	contentW := pageW - lm - rm
	termWidth := contentW * 0.11
	percentageWidth := contentW * 0.17
	amountWidth := contentW * 0.22
	dueDateWidth := contentW * 0.18
	descriptionWidth := contentW - termWidth - percentageWidth - amountWidth - dueDateWidth

	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(51, 51, 51)
	pdf.Cell(45, 5, tr.PaymentSchedule+":")
	pdf.Ln(6)

	pdf.SetFont("Arial", "B", 8)
	pdf.SetFillColor(248, 249, 250)
	pdf.SetDrawColor(160, 160, 160)
	pdf.SetLineWidth(0.4)
	pdf.SetX(lm)
	pdf.CellFormat(termWidth, 6, tr.Term, "1", 0, "C", true, 0, "")
	pdf.CellFormat(percentageWidth, 6, tr.Percentage, "1", 0, "C", true, 0, "")
	pdf.CellFormat(amountWidth, 6, tr.Amount, "1", 0, "R", true, 0, "")
	pdf.CellFormat(dueDateWidth, 6, tr.DueDate, "1", 0, "C", true, 0, "")
	pdf.CellFormat(descriptionWidth, 6, tr.Description, "1", 0, "L", true, 0, "")
	pdf.Ln(6)

	pdf.SetFont("Arial", "", 8)
	pdf.SetTextColor(102, 102, 102)
	for index, schedule := range sale.PaymentSchedules {
		description := strings.TrimSpace(schedule.Description)
		if description == "" {
			description = fmt.Sprintf("%s %d", tr.Term, schedule.Sequence)
		}
		if len(description) > 42 {
			description = description[:39] + "..."
		}

		if index%2 == 0 {
			pdf.SetFillColor(255, 255, 255)
		} else {
			pdf.SetFillColor(250, 250, 250)
		}

		pdf.SetX(lm)
		pdf.CellFormat(termWidth, 5, fmt.Sprintf("%d", schedule.Sequence), "1", 0, "C", true, 0, "")
		pdf.CellFormat(percentageWidth, 5, fmt.Sprintf("%.2f%%", schedule.Percentage), "1", 0, "C", true, 0, "")
		pdf.CellFormat(amountWidth, 5, p.formatRupiah(schedule.Amount), "1", 0, "R", true, 0, "")
		pdf.CellFormat(dueDateWidth, 5, schedule.DueDate.Format("02/01/2006"), "1", 0, "C", true, 0, "")
		pdf.CellFormat(descriptionWidth, 5, description, "1", 0, "L", true, 0, "")
		pdf.Ln(5)
	}

	pdf.Ln(2)
}

// PDFTranslations holds all translatable strings for PDF generation
type PDFTranslations struct {
	Invoice                  string
	Quotation                string
	Receipt                  string
	BillTo                   string
	ShipTo                   string
	InvoiceNumber            string
	QuotationNumber          string
	ReceiptNumber            string
	Date                     string
	DueDate                  string
	ValidUntil               string
	SaleCode                 string
	Item                     string
	Description              string
	Quantity                 string
	UnitPrice                string
	Discount                 string
	Tax                      string
	Amount                   string
	Subtotal                 string
	TotalDiscount            string
	TaxableAmount            string
	PPN                      string
	PPh                      string
	PPh21                    string
	PPh23                    string
	OtherTaxAdditions        string
	OtherTaxDeductions       string
	TotalTax                 string
	ShippingCost             string
	Shipping                 string
	GrandTotal               string
	Total                    string
	PaidAmount               string
	OutstandingAmount        string
	Notes                    string
	PaymentTerms             string
	PaymentSchedule          string
	Term                     string
	Percentage               string
	PaymentMethod            string
	ThankYou                 string
	CompanyInfo              string
	Phone                    string
	Email                    string
	TransferTo               string
	BankName                 string
	AccountNumber            string
	AtasNama                 string
	BankBranch               string
	GeneratedOn              string
	CustomerInfoNotAvailable string
	Product                  string
	No                       string
	Qty                      string
	Location                 string
}

// getPDFTranslations returns translations based on language
func getPDFTranslations(lang string) PDFTranslations {
	if lang == "id" || lang == "ID" {
		return PDFTranslations{
			Invoice:                  "FAKTUR",
			Quotation:                "PENAWARAN",
			Receipt:                  "KWITANSI",
			BillTo:                   "Tagih Ke",
			ShipTo:                   "Kirim Ke",
			InvoiceNumber:            "Nomor Faktur",
			QuotationNumber:          "Nomor Penawaran",
			ReceiptNumber:            "Nomor Kwitansi",
			Date:                     "Tanggal",
			DueDate:                  "Jatuh Tempo",
			ValidUntil:               "Berlaku Hingga",
			SaleCode:                 "Kode Penjualan",
			Item:                     "Item",
			Description:              "Deskripsi",
			Quantity:                 "Jumlah",
			UnitPrice:                "Harga Satuan",
			Discount:                 "Diskon",
			Tax:                      "Pajak",
			Amount:                   "Jumlah",
			Subtotal:                 "Subtotal",
			TotalDiscount:            "Total Diskon",
			TaxableAmount:            "Jumlah Kena Pajak",
			PPN:                      "PPN",
			PPh:                      "PPh",
			PPh21:                    "PPh 21",
			PPh23:                    "PPh 23",
			OtherTaxAdditions:        "Pajak Tambahan Lainnya",
			OtherTaxDeductions:       "Potongan Pajak Lainnya",
			TotalTax:                 "Total Pajak",
			ShippingCost:             "Biaya Pengiriman",
			Shipping:                 "Pengiriman",
			GrandTotal:               "Total Keseluruhan",
			Total:                    "TOTAL",
			PaidAmount:               "Jumlah Dibayar",
			OutstandingAmount:        "Jumlah Belum Dibayar",
			Notes:                    "Catatan",
			PaymentTerms:             "Syarat Pembayaran",
			PaymentSchedule:          "Jadwal Pembayaran",
			Term:                     "Termin",
			Percentage:               "Persentase",
			PaymentMethod:            "Metode Pembayaran",
			ThankYou:                 "Terima kasih atas kepercayaan Anda!",
			CompanyInfo:              "Informasi Perusahaan",
			Phone:                    "Telepon",
			Email:                    "Email",
			TransferTo:               "Transfer ke",
			BankName:                 "Nama Bank",
			AccountNumber:            "Nomor Rekening",
			AtasNama:                 "Atas Nama",
			BankBranch:               "Cabang Bank",
			GeneratedOn:              "Dibuat pada",
			CustomerInfoNotAvailable: "Informasi pelanggan tidak tersedia",
			Product:                  "Produk",
			No:                       "No.",
			Qty:                      "Jml",
			Location:                 "Lokasi Pengiriman",
		}
	}

	// Default to English
	return PDFTranslations{
		Invoice:                  "INVOICE",
		Quotation:                "QUOTATION",
		Receipt:                  "RECEIPT",
		BillTo:                   "Bill To",
		ShipTo:                   "Ship To",
		InvoiceNumber:            "Invoice Number",
		QuotationNumber:          "Quotation Number",
		ReceiptNumber:            "Receipt Number",
		Date:                     "Date",
		DueDate:                  "Due Date",
		ValidUntil:               "Valid Until",
		SaleCode:                 "Sale Code",
		Item:                     "Item",
		Description:              "Description",
		Quantity:                 "Quantity",
		UnitPrice:                "Unit Price",
		Discount:                 "Discount",
		Tax:                      "Tax",
		Amount:                   "Amount",
		Subtotal:                 "Subtotal",
		TotalDiscount:            "Total Discount",
		TaxableAmount:            "Taxable Amount",
		PPN:                      "PPN",
		PPh:                      "PPh",
		PPh21:                    "PPh 21",
		PPh23:                    "PPh 23",
		OtherTaxAdditions:        "Other Tax Additions",
		OtherTaxDeductions:       "Other Tax Deductions",
		TotalTax:                 "Total Tax",
		ShippingCost:             "Shipping Cost",
		Shipping:                 "Shipping",
		GrandTotal:               "Grand Total",
		Total:                    "TOTAL",
		PaidAmount:               "Paid Amount",
		OutstandingAmount:        "Outstanding Amount",
		Notes:                    "Notes",
		PaymentTerms:             "Payment Terms",
		PaymentSchedule:          "Payment Schedule",
		Term:                     "Term",
		Percentage:               "Percentage",
		PaymentMethod:            "Payment Method",
		ThankYou:                 "Thank you for your business!",
		CompanyInfo:              "Company Information",
		Phone:                    "Phone",
		Email:                    "Email",
		TransferTo:               "Transfer to",
		BankName:                 "Bank Name",
		AccountNumber:            "Account Number",
		AtasNama:                 "Account Holder",
		BankBranch:               "Bank Branch",
		GeneratedOn:              "Generated on",
		CustomerInfoNotAvailable: "Customer information not available",
		Product:                  "Product",
		No:                       "No.",
		Qty:                      "Qty",
		Location:                 "Delivery Location",
	}
}

// amountToRupiahWords converts a float amount to Indonesian words (Rupiah)
// Example: 47687820 -> "Empat Puluh Tujuh Juta Enam Ratus Delapan Puluh Tujuh Ribu Delapan Ratus Dua Puluh Rupiah"
func (p *PDFService) amountToRupiahWords(amount float64) string {
	if amount < 0 {
		amount = -amount
	}
	// Ignore decimals; receipts are in whole Rupiah
	val := int64(amount + 0.5)
	words := p.numberToIndonesianWords(val)
	if words == "" {
		words = "Nol"
	}
	return words + " Rupiah"
}

func (p *PDFService) numberToIndonesianWords(n int64) string {
	units := []string{"", "Satu", "Dua", "Tiga", "Empat", "Lima", "Enam", "Tujuh", "Delapan", "Sembilan"}
	var toWords func(n int64) string
	toWords = func(n int64) string {
		switch {
		case n == 0:
			return ""
		case n < 10:
			return units[n]
		case n == 10:
			return "Sepuluh"
		case n == 11:
			return "Sebelas"
		case n < 20:
			return units[n-10] + " Belas"
		case n < 100:
			puluh := n / 10
			sisa := n % 10
			res := units[puluh] + " Puluh"
			if sisa > 0 {
				res += " " + toWords(sisa)
			}
			return res
		case n == 100:
			return "Seratus"
		case n < 200:
			return "Seratus " + toWords(n-100)
		case n < 1000:
			ratus := n / 100
			sisa := n % 100
			res := units[ratus] + " Ratus"
			if sisa > 0 {
				res += " " + toWords(sisa)
			}
			return res
		case n == 1000:
			return "Seribu"
		case n < 2000:
			return "Seribu " + toWords(n-1000)
		case n < 1000000:
			ribu := n / 1000
			sisa := n % 1000
			res := toWords(ribu) + " Ribu"
			if sisa > 0 {
				res += " " + toWords(sisa)
			}
			return res
		case n < 1000000000:
			juta := n / 1000000
			sisa := n % 1000000
			res := toWords(juta) + " Juta"
			if sisa > 0 {
				res += " " + toWords(sisa)
			}
			return res
		case n < 1000000000000:
			milyar := n / 1000000000
			sisa := n % 1000000000
			res := toWords(milyar) + " Miliar"
			if sisa > 0 {
				res += " " + toWords(sisa)
			}
			return res
		default:
			triliun := n / 1000000000000
			sisa := n % 1000000000000
			res := toWords(triliun) + " Triliun"
			if sisa > 0 {
				res += " " + toWords(sisa)
			}
			return res
		}
	}
	return strings.TrimSpace(toWords(n))
}

// getFinanceSignatoryName finds an active user with finance role to sign the receipt
// If currentUserID is provided and that user is finance, use them. Otherwise fallback to first finance user.
func (p *PDFService) getFinanceSignatoryName() string {
	return p.getFinanceSignatoryNameWithUser(0) // Default: no specific user
}

// getFinanceSignatoryNameWithUser allows specifying a current user ID
func (p *PDFService) getFinanceSignatoryNameWithUser(currentUserID uint) string {
	if p.db == nil {
		return ""
	}

	// If currentUserID is provided, check if that user is an active finance user
	if currentUserID > 0 {
		// First check if the user exists at all to avoid GORM logging "record not found" errors
		var userExists models.User
		if err := p.db.Select("id").Where("id = ?", currentUserID).First(&userExists).Error; err != nil {
			// User doesn't exist, skip to fallback
		} else {
			// User exists, now check if they are an active finance user
			var currentUser models.User
			if err := p.db.Where("id = ? AND role = ? AND is_active = ?", currentUserID, "finance", true).First(&currentUser).Error; err == nil {
				full := strings.TrimSpace(strings.TrimSpace(currentUser.FirstName + " " + currentUser.LastName))
				if full != "" {
					return full
				}
				if strings.TrimSpace(currentUser.Username) != "" {
					return currentUser.Username
				}
			}
		}
	}

	// Fallback: find any active finance user (ordered by ID)
	var u models.User
	if err := p.db.Where("role = ? AND is_active = ?", "finance", true).Order("id ASC").First(&u).Error; err == nil {
		full := strings.TrimSpace(strings.TrimSpace(u.FirstName + " " + u.LastName))
		if full != "" {
			return full
		}
		if strings.TrimSpace(u.Username) != "" {
			return u.Username
		}
	}
	return ""
}

// getCompanyCity tries to infer a city from company address (fallback Jakarta)
func (p *PDFService) getCompanyCity(addr string) string {
	a := strings.TrimSpace(addr)
	if a == "" {
		return "Jakarta"
	}
	if strings.Contains(strings.ToLower(a), "jakarta") {
		return "Jakarta"
	}
	parts := strings.Split(a, ",")
	last := strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		last = strings.TrimSpace(parts[1])
	}
	if last == "" {
		return "Jakarta"
	}
	return last
}

// NewPDFService creates a new PDF service instance
func NewPDFService(db *gorm.DB) PDFServiceInterface {
	return &PDFService{db: db}
}

// Language returns the active language from settings
func (p *PDFService) Language() string {
	return utils.GetUserLanguageFromSettings(p.db)
}

// formatRupiah formats a number as Indonesian Rupiah
func (p *PDFService) formatRupiah(amount float64) string {
	// Format number with thousand separators
	amountStr := fmt.Sprintf("%.0f", amount)
	if amount != float64(int64(amount)) {
		amountStr = fmt.Sprintf("%.2f", amount)
	}

	// Add thousand separators
	formattedAmount := p.addThousandSeparators(amountStr)

	return "Rp " + formattedAmount
}

// formatRupiahCompact formats rupiah in compact form (shorter for tight spaces)
func (p *PDFService) formatRupiahCompact(amount float64) string {
	// For very large amounts, use abbreviated format
	if amount >= 1000000000 { // >= 1 billion
		return fmt.Sprintf("%.1fB", amount/1000000000)
	}
	if amount >= 1000000 { // >= 1 million
		return fmt.Sprintf("%.1fM", amount/1000000)
	}
	if amount >= 1000 { // >= 1 thousand
		return fmt.Sprintf("%.0fK", amount/1000)
	}
	return fmt.Sprintf("%.0f", amount)
}

// getCompanyInfo retrieves company information from settings
func (p *PDFService) getCompanyInfo() (*models.Settings, error) {
	// Return default company info if database is not available
	if p.db == nil {
		return &models.Settings{
			CompanyName:    "PT. Sistem Akuntansi Indonesia",
			CompanyAddress: "Jl. Sudirman Kav. 45-46, Jakarta Pusat 10210, Indonesia",
			CompanyPhone:   "+62-21-5551234",
			CompanyEmail:   "info@sistemakuntansi.co.id",
		}, nil
	}

	var settings models.Settings
	err := p.db.First(&settings).Error
	if err != nil {
		// Return default company info if settings not found
		return &models.Settings{
			CompanyName:    "PT. Sistem Akuntansi Indonesia",
			CompanyAddress: "Jl. Sudirman Kav. 45-46, Jakarta Pusat 10210, Indonesia",
			CompanyPhone:   "+62-21-5551234",
			CompanyEmail:   "info@sistemakuntansi.co.id",
		}, nil
	}
	return &settings, nil
}

// getBankInfoForSale returns bank info to display on invoice based on sale's CashBankID.
// If CashBankID is nil or not found, it falls back to the first active BANK account.
func (p *PDFService) getBankInfoForSale(sale *models.Sale) (*models.CashBank, error) {
	if p.db == nil {
		return nil, nil
	}
	var bank models.CashBank
	// Prefer the bank selected on the sale
	if sale != nil && sale.CashBankID != nil {
		if err := p.db.Where("id = ? AND deleted_at IS NULL", *sale.CashBankID).First(&bank).Error; err == nil {
			if strings.ToUpper(bank.Type) == models.CashBankTypeBank {
				return &bank, nil
			}
		}
	}
	// Fallback: first active BANK account
	if err := p.db.Where("type = ? AND is_active = ? AND deleted_at IS NULL", models.CashBankTypeBank, true).Order("id ASC").First(&bank).Error; err != nil {
		return nil, nil // no bank available; silently skip
	}
	return &bank, nil
}

// addThousandSeparators adds dots as thousand separators for Indonesian currency format
func (p *PDFService) addThousandSeparators(s string) string {
	// Split by decimal point if exists
	parts := strings.Split(s, ".")
	integerPart := parts[0]

	// Add thousand separators (dots) to integer part
	if len(integerPart) <= 3 {
		if len(parts) > 1 {
			return integerPart + "," + parts[1]
		}
		return integerPart
	}

	var result []string
	for i, digit := range reverse(integerPart) {
		if i > 0 && i%3 == 0 {
			result = append(result, ".")
		}
		result = append(result, string(digit))
	}

	formattedInteger := reverse(strings.Join(result, ""))

	if len(parts) > 1 {
		return formattedInteger + "," + parts[1]
	}
	return formattedInteger
}

// reverse reverses a string
func reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// formatNotesAmount formats numbers in notes text by rounding and removing decimals
// Example: "Setor PPN - Terutang: 64033,20" -> "Setor PPN - Terutang: 64033"
// Example: "Setor PPN - Terutang: 64033.20" -> "Setor PPN - Terutang: 64033"
// Example: "Setor PPN - Terutang: 1.234.567,89" -> "Setor PPN - Terutang: 1234568"
func (p *PDFService) formatNotesAmount(notes string) string {
	// Simplified pattern to match numbers with decimal separator (comma or dot)
	// This matches: digits (with optional dots as thousand separators) followed by comma/dot and more digits
	// Examples: 64033.20, 64033,20, 1.234.567,89, 1,234,567.89
	re := regexp.MustCompile(`\d+(?:[\.,]\d+)*[\.,]\d+`)

	// Replace with rounded integer (remove decimal part)
	formatted := re.ReplaceAllStringFunc(notes, func(match string) string {
		// Remove all dots (thousand separators) and replace comma with dot for parsing
		numStr := strings.ReplaceAll(match, ".", "")
		numStr = strings.ReplaceAll(numStr, ",", ".")

		// Parse as float
		if val, err := strconv.ParseFloat(numStr, 64); err == nil {
			// Round and return as integer string without decimals
			return fmt.Sprintf("%.0f", val)
		}
		return match // Return original if parsing fails
	})

	return formatted
}

// addCompanyLetterhead tries to render company logo as a letterhead at top of the page
func (p *PDFService) addCompanyLetterhead(pdf *gofpdf.Fpdf) {
	settings, err := p.getCompanyInfo()
	if err != nil || settings == nil {
		return
	}
	logo := strings.TrimSpace(settings.CompanyLogo)
	if logo == "" {
		return
	}
	// Map web path "/uploads/..." to local filesystem path "./uploads/..."
	localPath := logo
	if strings.HasPrefix(localPath, "/") {
		localPath = "." + localPath
	}
	// Resolve and ensure file exists
	if _, err := os.Stat(localPath); err != nil {
		// Try joining with working dir uploads/company
		alt := filepath.Clean("./" + strings.TrimPrefix(logo, "/"))
		if _, err2 := os.Stat(alt); err2 != nil {
			return
		}
		localPath = alt
	}
	// Detect image type by magic bytes to avoid invalid JPEG/PNG errors
	imgType := detectImageType(localPath)
	if imgType == "" {
		// Unknown or unsupported image type; skip letterhead to avoid breaking PDF
		return
	}
	// Draw a compact logo at top-left (letterhead style)
	// Get margins and page size
	lm, tm, rm, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	_ = rm // right margin unused in this placement

	// Choose a reasonable logo width; larger for landscape pages
	logoW := 35.0
	if pageW > 250 { // landscape A4 ~ 297mm width
		logoW = 40.0
	}
	// Place logo at top-left, slightly below the top margin
	x := lm
	y := tm + 2.0
	// Height=0 preserves aspect ratio
	pdf.ImageOptions(localPath, x, y, logoW, 0, false, gofpdf.ImageOptions{ImageType: imgType, ReadDpi: true}, 0, "")

	// Ensure subsequent content starts below the logo area (title printed after this)
	currentY := pdf.GetY()
	// Reserve vertical space at least equal to logo height area
	minY := y + logoW + 4.0
	if currentY < minY {
		pdf.SetY(minY)
	}
}

// detectImageType inspects the file header to determine image type supported by gofpdf (JPG/PNG)
func detectImageType(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	buf := make([]byte, 8)
	if _, err := f.Read(buf); err != nil {
		return ""
	}
	// JPEG SOI marker
	if len(buf) >= 2 && buf[0] == 0xFF && buf[1] == 0xD8 {
		return "JPG"
	}
	// PNG signature
	if len(buf) >= 8 && buf[0] == 0x89 && buf[1] == 0x50 && buf[2] == 0x4E && buf[3] == 0x47 && buf[4] == 0x0D && buf[5] == 0x0A && buf[6] == 0x1A && buf[7] == 0x0A {
		return "PNG"
	}
	return ""
}

// GenerateInvoicePDF generates a clean PDF for a sale invoice with multi-language support
// drawUnifiedLetterhead renders a consistent, professional letterhead on a PDF page.
// Layout: logo at top-left, company info right-aligned, separator line underneath.
// Colors match the Cash Flow PDF style (dark navy company name, dark gray details).
// Returns the Y position after the header so content can start below it.
func drawUnifiedLetterhead(pdf *gofpdf.Fpdf, companyInfo *models.Settings, logoSize float64) float64 {
	lm, tm, rm, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()

	if logoSize <= 0 {
		logoSize = 35.0
	}

	// ── Logo (top-left) ──
	logoAdded := false
	if companyInfo != nil && strings.TrimSpace(companyInfo.CompanyLogo) != "" {
		logoPath := companyInfo.CompanyLogo
		if strings.HasPrefix(logoPath, "/") {
			logoPath = "." + logoPath
		}
		if _, err := os.Stat(logoPath); err != nil {
			alt := filepath.Clean("./" + strings.TrimPrefix(companyInfo.CompanyLogo, "/"))
			if _, err2 := os.Stat(alt); err2 == nil {
				logoPath = alt
			} else {
				logoPath = ""
			}
		}
		if logoPath != "" {
			if imgType := detectImageType(logoPath); imgType != "" {
				pdf.ImageOptions(logoPath, lm, tm, logoSize, 0, false, gofpdf.ImageOptions{ImageType: imgType, ReadDpi: true}, 0, "")
				logoAdded = true
			}
		}
	}
	// Placeholder if no logo
	if !logoAdded {
		pdf.SetDrawColor(220, 220, 220)
		pdf.SetFillColor(248, 249, 250)
		pdf.SetLineWidth(0.3)
		pdf.Rect(lm, tm, logoSize, logoSize, "FD")
		pdf.SetFont("Arial", "B", 16)
		pdf.SetTextColor(120, 120, 120)
		pdf.SetXY(lm+8, tm+19)
		pdf.CellFormat(19, 8, "</>", "", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}

	// ── Company info (right-aligned) ──
	rightX := pageW - rm
	infoY := tm

	if companyInfo != nil {
		// Company name — bold, dark navy
		name := strings.TrimSpace(companyInfo.CompanyName)
		if name != "" {
			pdf.SetFont("Arial", "B", 12)
			pdf.SetTextColor(25, 55, 109) // dark navy matching Cash Flow PDF
			w := pdf.GetStringWidth(name)
			pdf.SetXY(rightX-w, infoY)
			pdf.Cell(w, 6, name)
			infoY += 8
		}

		// Address — dark gray
		addr := strings.TrimSpace(companyInfo.CompanyAddress)
		if addr != "" {
			pdf.SetFont("Arial", "", 9)
			pdf.SetTextColor(33, 37, 41)
			w := pdf.GetStringWidth(addr)
			pdf.SetXY(rightX-w, infoY)
			pdf.Cell(w, 4, addr)
			infoY += 6
		}

		// Phone
		phone := strings.TrimSpace(companyInfo.CompanyPhone)
		if phone != "" {
			phoneText := fmt.Sprintf("Phone: %s", phone)
			pdf.SetFont("Arial", "", 9)
			pdf.SetTextColor(33, 37, 41)
			w := pdf.GetStringWidth(phoneText)
			pdf.SetXY(rightX-w, infoY)
			pdf.Cell(w, 4, phoneText)
			infoY += 6
		}

		// Email
		email := strings.TrimSpace(companyInfo.CompanyEmail)
		if email != "" {
			emailText := fmt.Sprintf("Email: %s", email)
			pdf.SetFont("Arial", "", 9)
			pdf.SetTextColor(33, 37, 41)
			w := pdf.GetStringWidth(emailText)
			pdf.SetXY(rightX-w, infoY)
			pdf.Cell(w, 4, emailText)
		}
	}

	// Reset text color
	pdf.SetTextColor(0, 0, 0)

	// ── Separator line ──
	separatorY := tm + logoSize + 10
	pdf.SetDrawColor(238, 238, 238)
	pdf.SetLineWidth(0.2)
	pdf.Line(lm, separatorY, pageW-rm, separatorY)

	// Return Y position below separator
	return separatorY + 8
}

// GenerateInvoicePDF — kept below for existing invoice generation
func (p *PDFService) GenerateInvoicePDF(invoice interface{}) ([]byte, error) {
	// Default to user's language from settings
	lang := p.Language()
	return p.GenerateInvoicePDFWithLanguage(invoice, lang)
}

// GenerateInvoicePDFWithLanguage generates a clean PDF for a sale invoice with specified language
func (p *PDFService) GenerateInvoicePDFWithLanguage(invoice interface{}, lang string) ([]byte, error) {
	utils.SanitizeExportData(invoice)
	// Extract sale from interface{}
	var sale *models.Sale
	switch v := invoice.(type) {
	case *models.Sale:
		sale = v
	case models.Sale:
		sale = &v
	default:
		return nil, fmt.Errorf("invalid invoice data type")
	}

	if sale == nil {
		return nil, fmt.Errorf("sale data is required")
	}

	// Get translations for the specified language
	tr := getPDFTranslations(lang)

	// Create new PDF document with clean margins
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// Get margins and page size for positioning
	lm, tm, rm, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	contentW := pageW - lm - rm

	// Get company info from settings
	companyInfo, err := p.getCompanyInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get company info: %v", err)
	}

	// === CLEAN HEADER SECTION ===
	// Company logo section (left side) - smaller and cleaner
	logoX := lm
	logoY := tm
	logoSize := 26.0

	// Try to add company logo
	logoAdded := false
	if companyInfo.CompanyLogo != "" {
		logoPath := companyInfo.CompanyLogo
		if strings.HasPrefix(logoPath, "/") {
			logoPath = "." + logoPath
		}
		if _, err := os.Stat(logoPath); err == nil {
			imgType := detectImageType(logoPath)
			if imgType != "" {
				pdf.ImageOptions(logoPath, logoX, logoY, logoSize, 0, false, gofpdf.ImageOptions{ImageType: imgType}, 0, "")
				logoAdded = true
			}
		}
	}

	// If no logo, add a clean placeholder with coding symbol
	if !logoAdded {
		// Clean rectangle placeholder (simple and compatible)
		pdf.SetDrawColor(220, 220, 220)
		pdf.SetFillColor(248, 249, 250)
		pdf.SetLineWidth(0.3)
		pdf.Rect(logoX, logoY, logoSize, logoSize, "FD")

		// Add clean coding symbol
		pdf.SetFont("Arial", "B", 16)
		pdf.SetTextColor(120, 120, 120)
		pdf.SetXY(logoX+8, logoY+19)
		pdf.CellFormat(19, 8, "</>", "", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}

	// Company information on the right side - clean alignment
	companyInfoX := pageW - rm
	companyInfoY := tm

	// Company name
	pdf.SetFont("Arial", "B", 12)
	companyNameWidth := pdf.GetStringWidth(companyInfo.CompanyName)
	pdf.SetXY(companyInfoX-companyNameWidth, companyInfoY)
	pdf.Cell(companyNameWidth, 6, companyInfo.CompanyName)

	// Address
	pdf.SetFont("Arial", "", 9)
	addressWidth := pdf.GetStringWidth(companyInfo.CompanyAddress)
	pdf.SetXY(companyInfoX-addressWidth, companyInfoY+8)
	pdf.Cell(addressWidth, 4, companyInfo.CompanyAddress)

	// Phone
	phoneText := fmt.Sprintf("%s: %s", tr.Phone, companyInfo.CompanyPhone)
	phoneWidth := pdf.GetStringWidth(phoneText)
	pdf.SetXY(companyInfoX-phoneWidth, companyInfoY+14)
	pdf.Cell(phoneWidth, 4, phoneText)

	// Email
	emailText := fmt.Sprintf("%s: %s", tr.Email, companyInfo.CompanyEmail)
	emailWidth := pdf.GetStringWidth(emailText)
	pdf.SetXY(companyInfoX-emailWidth, companyInfoY+20)
	pdf.Cell(emailWidth, 4, emailText)

	// NPWP (Tax Number) - only show if available
	if companyInfo.TaxNumber != "" {
		npwpText := fmt.Sprintf("NPWP: %s", companyInfo.TaxNumber)
		npwpWidth := pdf.GetStringWidth(npwpText)
		pdf.SetXY(companyInfoX-npwpWidth, companyInfoY+26)
		pdf.Cell(npwpWidth, 4, npwpText)
	}

	// Add a subtle line under header
	pdf.SetDrawColor(238, 238, 238)
	pdf.SetLineWidth(0.2)
	pdf.Line(lm, tm+36, pageW-rm, tm+36)

	// === INVOICE TITLE SECTION ===
	pdf.SetY(tm + 38) // Start below the header area
	pdf.SetX(lm)
	pdf.SetFont("Arial", "B", 22)
	pdf.SetTextColor(51, 51, 51)

	// Use InvoiceType.Name if available, fallback to generic "FAKTUR"/"INVOICE"
	invoiceTitle := tr.Invoice
	invoiceLabel := tr.InvoiceNumber
	if sale.InvoiceType != nil && strings.TrimSpace(sale.InvoiceType.Name) != "" {
		invoiceTitle = strings.ToUpper(strings.TrimSpace(sale.InvoiceType.Name))
		// Build label: "Nomor [TypeName]" or "TypeName Number"
		if lang == "id" || lang == "ID" {
			invoiceLabel = "Nomor " + strings.TrimSpace(sale.InvoiceType.Name)
		} else {
			invoiceLabel = strings.TrimSpace(sale.InvoiceType.Name) + " Number"
		}
	}

	pdf.Cell(contentW, 10, invoiceTitle)
	pdf.SetTextColor(0, 0, 0) // Reset
	pdf.Ln(10)

	// === INVOICE DETAILS SECTION ===
	pdf.SetFont("Arial", "B", 9)

	// Invoice Number and Date on same line with clean formatting
	invoiceNum := sale.InvoiceNumber
	if invoiceNum == "" {
		invoiceNum = sale.Code // fallback to sale code
	}

	// Left side details
	leftLabelWidth := 42.0
	leftValueWidth := 68.0
	rightLabelWidth := 22.0
	rightValueWidth := 38.0

	pdf.SetX(lm)
	pdf.CellFormat(leftLabelWidth, 5, invoiceLabel+":", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.CellFormat(leftValueWidth, 5, invoiceNum, "", 0, "L", false, 0, "")

	// Right side details
	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(0, 0, 0)
	dateX := lm + contentW - 60
	pdf.SetX(dateX)
	pdf.CellFormat(rightLabelWidth, 5, tr.Date+":", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.CellFormat(rightValueWidth, 5, sale.Date.Format("02/01/2006"), "", 0, "L", false, 0, "")
	pdf.Ln(6)

	// Sale Code and Due Date on second line
	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetX(lm)
	pdf.CellFormat(leftLabelWidth, 5, tr.SaleCode+":", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.CellFormat(leftValueWidth, 5, sale.Code, "", 0, "L", false, 0, "")

	if !sale.DueDate.IsZero() {
		pdf.SetFont("Arial", "B", 9)
		pdf.SetTextColor(0, 0, 0)
		pdf.SetX(dateX)
		pdf.CellFormat(rightLabelWidth, 5, tr.DueDate+":", "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(102, 102, 102)
		pdf.CellFormat(rightValueWidth, 5, sale.DueDate.Format("02/01/2006"), "", 0, "L", false, 0, "")
	}
	pdf.SetTextColor(0, 0, 0) // Reset
	pdf.Ln(8)

	// === BILL TO SECTION ===
	pdf.SetFont("Arial", "B", 10)
	pdf.SetTextColor(51, 51, 51)
	pdf.Cell(contentW, 6, tr.BillTo+":")
	pdf.Ln(6)

	// Customer details dengan jarak rapat
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	if sale.Customer.ID != 0 {
		pdf.Cell(contentW, 4, sale.Customer.Name)
		pdf.Ln(5)
		if sale.Customer.Address != "" {
			pdf.Cell(contentW, 4, sale.Customer.Address)
			pdf.Ln(5)
		}
		if sale.Customer.Phone != "" {
			pdf.Cell(contentW, 4, fmt.Sprintf("%s: %s", tr.Phone, sale.Customer.Phone))
			pdf.Ln(5)
		}
	} else {
		pdf.Cell(contentW, 4, tr.CustomerInfoNotAvailable)
		pdf.Ln(5)
	}
	pdf.SetTextColor(0, 0, 0) // Reset
	pdf.Ln(6)

	// === ITEMS TABLE SECTION ===
	// Clean table headers with modern styling
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(248, 249, 250) // Very light gray
	// Darker border color and thicker lines for stronger table borders
	pdf.SetDrawColor(160, 160, 160)
	pdf.SetTextColor(51, 51, 51)
	pdf.SetLineWidth(0.4)

	// Calculate column widths for clean layout
	numWidth := contentW * 0.05
	descWidth := contentW * 0.28
	locWidth := contentW * 0.22
	qtyWidth := contentW * 0.10
	priceWidth := contentW * 0.17
	totalWidth := contentW * 0.18

	pdf.CellFormat(numWidth, 6, tr.No, "1", 0, "C", true, 0, "")
	pdf.CellFormat(descWidth, 6, tr.Description, "1", 0, "L", true, 0, "")
	pdf.CellFormat(locWidth, 6, tr.Location, "1", 0, "L", true, 0, "")
	pdf.CellFormat(qtyWidth, 6, tr.Qty, "1", 0, "C", true, 0, "")
	pdf.CellFormat(priceWidth, 6, tr.UnitPrice, "1", 0, "R", true, 0, "")
	pdf.CellFormat(totalWidth, 6, tr.Total, "1", 0, "R", true, 0, "")
	pdf.Ln(6)

	// Table data with clean alternating rows
	pdf.SetFont("Arial", "", 8)
	pdf.SetTextColor(102, 102, 102)

	subtotal := 0.0
	for i, item := range sale.SaleItems {
		// Check if we need a new page
		if pdf.GetY() > 250 {
			pdf.AddPage()
			// Re-add headers on new page
			pdf.SetFont("Arial", "B", 9)
			pdf.SetFillColor(248, 249, 250)
			// Ensure strong borders on each new page
			pdf.SetDrawColor(160, 160, 160)
			pdf.SetLineWidth(0.4)
			pdf.SetTextColor(51, 51, 51)
			pdf.CellFormat(numWidth, 6, tr.No, "1", 0, "C", true, 0, "")
			pdf.CellFormat(descWidth, 6, tr.Description, "1", 0, "L", true, 0, "")
			pdf.CellFormat(locWidth, 6, tr.Location, "1", 0, "L", true, 0, "")
			pdf.CellFormat(qtyWidth, 6, tr.Qty, "1", 0, "C", true, 0, "")
			pdf.CellFormat(priceWidth, 6, tr.UnitPrice, "1", 0, "R", true, 0, "")
			pdf.CellFormat(totalWidth, 6, tr.Total, "1", 0, "R", true, 0, "")
			pdf.Ln(6)
			pdf.SetFont("Arial", "", 8)
			pdf.SetTextColor(102, 102, 102)
		}

		// Alternating row colors
		if i%2 == 0 {
			pdf.SetFillColor(255, 255, 255) // White
		} else {
			pdf.SetFillColor(250, 250, 250) // Very light gray
		}

		// Item data
		itemNumber := strconv.Itoa(i + 1)
		description := tr.Product
		if item.ProductName != "" {
			description = item.ProductName
			if len(description) > 45 {
				description = description[:42] + "..."
			}
		} else if item.Product.ID != 0 {
			description = item.Product.Name
			// Truncate long descriptions
			if len(description) > 45 {
				description = description[:42] + "..."
			}
		}

		quantity := strconv.Itoa(int(item.Quantity))
		unitPrice := p.formatRupiah(item.UnitPrice)

		// Use LineTotal instead of TotalPrice for better accuracy
		lineTotal := item.LineTotal
		if lineTotal == 0 {
			lineTotal = float64(item.Quantity)*item.UnitPrice - item.DiscountAmount
		}
		totalPrice := p.formatRupiah(lineTotal)

		pdf.CellFormat(numWidth, 5, itemNumber, "1", 0, "C", true, 0, "")
		pdf.CellFormat(descWidth, 5, description, "1", 0, "L", true, 0, "")
		pdf.CellFormat(locWidth, 5, item.Location, "1", 0, "L", true, 0, "")
		pdf.CellFormat(qtyWidth, 5, quantity, "1", 0, "C", true, 0, "")
		pdf.CellFormat(priceWidth, 5, unitPrice, "1", 0, "R", true, 0, "")
		pdf.CellFormat(totalWidth, 5, totalPrice, "1", 0, "R", true, 0, "")
		pdf.Ln(5)

		subtotal += lineTotal
	}

	// === SUMMARY SECTION ===
	pdf.Ln(8)
	pdf.SetTextColor(0, 0, 0) // Reset to black

	// Use persisted transaction values for commercial document rendering.
	summary := p.buildSalePDFSummary(sale, subtotal)

	// Summary positioned on the right with clean design
	summaryWidth := 85.0
	summaryX := contentW - summaryWidth
	currentY := pdf.GetY()

	// Subtotal
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(lm+summaryX, currentY)
	pdf.Cell(45, 6, tr.Subtotal+":")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.CellFormat(40, 6, p.formatRupiah(subtotal), "", 0, "R", false, 0, "")
	currentY += 7

	// Global Discount (only show if > 0)
	if summary.DiscountAmount > 0 {
		pdf.SetFont("Arial", "B", 9)
		pdf.SetTextColor(0, 0, 0)
		pdf.SetXY(lm+summaryX, currentY)
		pdf.Cell(45, 6, fmt.Sprintf("%s (%.1f%%):", tr.Discount, sale.DiscountPercent))
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(220, 53, 69) // Red for deduction
		pdf.CellFormat(40, 6, "- "+p.formatRupiah(summary.DiscountAmount), "", 0, "R", false, 0, "")
		currentY += 7
	}

	// PPN (only show if > 0)
	if summary.PPNAmount > 0 {
		pdf.SetFont("Arial", "B", 9)
		pdf.SetTextColor(0, 0, 0)
		pdf.SetXY(lm+summaryX, currentY)
		pdf.Cell(45, 6, fmt.Sprintf("%s (%.1f%%):", tr.PPN, summary.PPNRate))
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(40, 167, 69) // Green for addition
		pdf.CellFormat(40, 6, "+ "+p.formatRupiah(summary.PPNAmount), "", 0, "R", false, 0, "")
		currentY += 7
	}

	// Other Tax Additions (only show if > 0)
	if summary.OtherTaxAdditions > 0 {
		pdf.SetFont("Arial", "B", 9)
		pdf.SetTextColor(0, 0, 0)
		pdf.SetXY(lm+summaryX, currentY)
		pdf.Cell(45, 6, tr.OtherTaxAdditions+":")
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(40, 167, 69) // Green for addition
		pdf.CellFormat(40, 6, "+ "+p.formatRupiah(summary.OtherTaxAdditions), "", 0, "R", false, 0, "")
		currentY += 7
	}

	// PPh21 (only show if > 0)
	if summary.PPh21Amount > 0 {
		pdf.SetFont("Arial", "B", 9)
		pdf.SetTextColor(0, 0, 0)
		pdf.SetXY(lm+summaryX, currentY)
		pdf.Cell(45, 6, fmt.Sprintf("%s (%.1f%%):", tr.PPh21, summary.PPh21Rate))
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(220, 53, 69) // Red for deduction
		pdf.CellFormat(40, 6, "- "+p.formatRupiah(summary.PPh21Amount), "", 0, "R", false, 0, "")
		currentY += 7
	}

	// PPh23 (only show if > 0)
	if summary.PPh23Amount > 0 {
		pdf.SetFont("Arial", "B", 9)
		pdf.SetTextColor(0, 0, 0)
		pdf.SetXY(lm+summaryX, currentY)
		pdf.Cell(45, 6, fmt.Sprintf("%s (%.1f%%):", tr.PPh23, summary.PPh23Rate))
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(220, 53, 69) // Red for deduction
		pdf.CellFormat(40, 6, "- "+p.formatRupiah(summary.PPh23Amount), "", 0, "R", false, 0, "")
		currentY += 7
	}

	// Other Tax Deductions (only show if > 0)
	if summary.OtherTaxDeductions > 0 {
		pdf.SetFont("Arial", "B", 9)
		pdf.SetTextColor(0, 0, 0)
		pdf.SetXY(lm+summaryX, currentY)
		pdf.Cell(45, 6, tr.OtherTaxDeductions+":")
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(220, 53, 69) // Red for deduction
		pdf.CellFormat(40, 6, "- "+p.formatRupiah(summary.OtherTaxDeductions), "", 0, "R", false, 0, "")
		currentY += 7
	}

	// Shipping Cost (only show if > 0)
	if summary.ShippingCost > 0 {
		pdf.SetFont("Arial", "B", 9)
		pdf.SetTextColor(0, 0, 0)
		pdf.SetXY(lm+summaryX, currentY)
		pdf.Cell(45, 6, tr.ShippingCost+":")
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(40, 167, 69) // Green for addition
		pdf.CellFormat(40, 6, "+ "+p.formatRupiah(summary.ShippingCost), "", 0, "R", false, 0, "")
		currentY += 7
	}

	// Draw line above total
	pdf.SetDrawColor(221, 221, 221)
	pdf.SetLineWidth(0.3)
	pdf.Line(lm+summaryX, currentY+2, lm+summaryX+85, currentY+2)
	currentY += 6

	// ✅ FIX: Recalculate total in PDF to ensure PPh23 is properly deducted
	// This fixes the issue where sale.TotalAmount in database might not be updated
	// Total with emphasis
	pdf.SetFont("Arial", "B", 10)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetXY(lm+summaryX, currentY)
	pdf.Cell(45, 8, tr.Total+":")
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(40, 8, p.formatRupiah(summary.TotalAmount), "", 0, "R", false, 0, "")
	pdf.Ln(12)

	// === PAYMENT TERMS SECTION ===
	if sale.PaymentTerms != "" {
		pdf.SetFont("Arial", "B", 9)
		pdf.SetTextColor(51, 51, 51)
		pdf.Cell(35, 5, tr.PaymentTerms+":")
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(102, 102, 102)
		pdf.Cell(100, 5, " "+sale.PaymentTerms)
		pdf.Ln(8)
	}

	p.renderSalePaymentSchedule(pdf, sale, tr)

	// === TRANSFER TO (Bank Info) SECTION ===
	if bankInfo, _ := p.getBankInfoForSale(sale); bankInfo != nil {
		pdf.SetFont("Arial", "B", 9)
		pdf.SetTextColor(51, 51, 51)
		pdf.Cell(30, 5, tr.TransferTo+":")
		pdf.Ln(6)
		// Bank Name
		pdf.SetFont("Arial", "B", 9)
		pdf.SetTextColor(51, 51, 51)
		pdf.Cell(35, 5, tr.BankName+":")
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(102, 102, 102)
		pdf.Cell(120, 5, strings.TrimSpace(bankInfo.BankName))
		pdf.Ln(6)
		// Account Number
		pdf.SetFont("Arial", "B", 9)
		pdf.SetTextColor(51, 51, 51)
		pdf.Cell(35, 5, tr.AccountNumber+":")
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(102, 102, 102)
		pdf.Cell(120, 5, strings.TrimSpace(bankInfo.AccountNo))
		pdf.Ln(6)
		// Atas Nama
		if strings.TrimSpace(bankInfo.AccountHolderName) != "" {
			pdf.SetFont("Arial", "B", 9)
			pdf.SetTextColor(51, 51, 51)
			pdf.Cell(35, 5, tr.AtasNama+":")
			pdf.SetFont("Arial", "", 9)
			pdf.SetTextColor(102, 102, 102)
			pdf.Cell(120, 5, strings.TrimSpace(bankInfo.AccountHolderName))
			pdf.Ln(6)
		}
		// Bank Branch
		if strings.TrimSpace(bankInfo.Branch) != "" {
			pdf.SetFont("Arial", "B", 9)
			pdf.SetTextColor(51, 51, 51)
			pdf.Cell(35, 5, tr.BankBranch+":")
			pdf.SetFont("Arial", "", 9)
			pdf.SetTextColor(102, 102, 102)
			pdf.Cell(120, 5, strings.TrimSpace(bankInfo.Branch))
			pdf.Ln(6)
		}
		pdf.Ln(2)
	}

	// Notes
	if sale.Notes != "" {
		pdf.Ln(5)
		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(190, 6, tr.Notes+":")
		pdf.Ln(6)
		pdf.SetFont("Arial", "", 9)
		pdf.MultiCell(190, 4, sale.Notes, "", "", false)
	}

	// === FOOTER SECTION ===
	// Add more space before footer
	pdf.Ln(15)

	// Add subtle top border for footer
	pdf.SetDrawColor(238, 238, 238)
	pdf.SetLineWidth(0.2)
	pdf.Line(lm, pdf.GetY(), pageW-rm, pdf.GetY())
	pdf.Ln(6)

	// Footer text - centered and subtle
	pdf.SetFont("Arial", "", 8)
	pdf.SetTextColor(153, 153, 153)
	footerText := fmt.Sprintf("%s %s", tr.GeneratedOn, time.Now().Format("02/01/2006 15:04"))
	footerWidth := pdf.GetStringWidth(footerText)
	footerX := (pageW - footerWidth) / 2
	pdf.SetX(footerX)
	pdf.Cell(footerWidth, 4, footerText)

	// Output to buffer
	var buf bytes.Buffer
	err = pdf.Output(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %v", err)
	}

	return buf.Bytes(), nil
}

// GenerateInvoicePDFWithType generates a PDF for a sale with customizable document type and multi-language support
func (p *PDFService) GenerateInvoicePDFWithType(sale *models.Sale, documentType string) ([]byte, error) {
	// Default to user's language from settings
	lang := p.Language()
	return p.GenerateInvoicePDFWithTypeAndLanguage(sale, documentType, lang)
}

// GenerateInvoicePDFWithTypeAndLanguage generates a PDF for a sale with customizable document type and specified language
func (p *PDFService) GenerateInvoicePDFWithTypeAndLanguage(sale *models.Sale, documentType string, lang string) ([]byte, error) {
	utils.SanitizeExportData(sale)
	// Get translations for the specified language
	tr := getPDFTranslations(lang)

	// Translate document type if it's a standard type
	translatedDocType := documentType
	switch strings.ToUpper(documentType) {
	case "INVOICE":
		translatedDocType = tr.Invoice
	case "QUOTATION":
		translatedDocType = tr.Quotation
	case "RECEIPT":
		translatedDocType = tr.Receipt
	}
	// Create new PDF document
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	// Try adding company letterhead/logo
	p.addCompanyLetterhead(pdf)

	// Get margins and page size for positioning
	lm, tm, rm, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	logoW := 35.0
	if pageW > 250 { // landscape width threshold
		logoW = 40.0
	}

	// Get company info from settings
	companyInfo, err := p.getCompanyInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get company info: %v", err)
	}

	// Position company info at extreme right corner like BORCELLE example
	// Calculate position to align text to the right margin
	companyYStart := tm + 2 // Same vertical position as logo

	// Position each line individually at right margin for perfect right alignment
	pdf.SetXY(0, companyYStart) // Start from left to calculate properly
	pdf.SetFont("Arial", "B", 12)
	// Get text width to position it exactly at right edge
	companyNameWidth := pdf.GetStringWidth(companyInfo.CompanyName)
	companyXStart := pageW - rm - companyNameWidth
	pdf.SetXY(companyXStart, companyYStart)
	pdf.Cell(companyNameWidth, 8, companyInfo.CompanyName)

	// Address line
	pdf.SetFont("Arial", "", 10)
	addressWidth := pdf.GetStringWidth(companyInfo.CompanyAddress)
	addressXStart := pageW - rm - addressWidth
	pdf.SetXY(addressXStart, companyYStart+6)
	pdf.Cell(addressWidth, 5, companyInfo.CompanyAddress)

	// Phone line
	phoneText := fmt.Sprintf("%s: %s", tr.Phone, companyInfo.CompanyPhone)
	phoneWidth := pdf.GetStringWidth(phoneText)
	phoneXStart := pageW - rm - phoneWidth
	pdf.SetXY(phoneXStart, companyYStart+11)
	pdf.Cell(phoneWidth, 5, phoneText)

	// Email line
	emailText := fmt.Sprintf("%s: %s", tr.Email, companyInfo.CompanyEmail)
	emailWidth := pdf.GetStringWidth(emailText)
	emailXStart := pageW - rm - emailWidth
	pdf.SetXY(emailXStart, companyYStart+16)
	pdf.Cell(emailWidth, 5, emailText)

	// Ensure following content starts below the logo+info block
	minY := tm + logoW + 6
	if pdf.GetY() < minY {
		pdf.SetY(minY)
	}
	// Draw title below the logo area, left-aligned
	pdf.SetX(lm)
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(pageW-lm-rm, 10, strings.ToUpper(translatedDocType))
	pdf.Ln(10)

	// Document details with flexible labeling
	pdf.SetFont("Arial", "B", 10)

	// Show invoice number only if it exists
	if sale.InvoiceNumber != "" {
		pdf.Cell(95, 6, fmt.Sprintf("%s %s: %s", translatedDocType, tr.InvoiceNumber, sale.InvoiceNumber))
	} else {
		pdf.Cell(95, 6, fmt.Sprintf("%s: %s", tr.SaleCode, sale.Code))
	}
	pdf.Cell(95, 6, fmt.Sprintf("%s: %s", tr.Date, sale.Date.Format("02/01/2006")))
	pdf.Ln(6)

	pdf.Cell(95, 6, fmt.Sprintf("%s: %s", tr.SaleCode, sale.Code))
	if !sale.DueDate.IsZero() {
		pdf.Cell(95, 6, fmt.Sprintf("%s: %s", tr.DueDate, sale.DueDate.Format("02/01/2006")))
	}
	pdf.Ln(10)

	// Customer info
	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(190, 6, tr.BillTo+":")
	pdf.Ln(6)
	pdf.SetFont("Arial", "", 10)
	// Customer info is always loaded, check if ID is set
	if sale.Customer.ID != 0 {
		pdf.Cell(190, 5, sale.Customer.Name)
		pdf.Ln(5)
		if sale.Customer.Address != "" {
			pdf.Cell(190, 5, sale.Customer.Address)
			pdf.Ln(5)
		}
		if sale.Customer.Phone != "" {
			pdf.Cell(190, 5, fmt.Sprintf("%s: %s", tr.Phone, sale.Customer.Phone))
			pdf.Ln(5)
		}
		if sale.Customer.Email != "" {
			pdf.Cell(190, 5, fmt.Sprintf("%s: %s", tr.Email, sale.Customer.Email))
			pdf.Ln(5)
		}
	} else {
		pdf.Cell(190, 5, tr.CustomerInfoNotAvailable)
		pdf.Ln(5)
	}
	pdf.Ln(5)

	// Items table header
	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(220, 220, 220)
	pdf.CellFormat(10, 8, tr.No, "1", 0, "C", true, 0, "")
	pdf.CellFormat(50, 8, tr.Description, "1", 0, "L", true, 0, "")
	pdf.CellFormat(25, 8, tr.Location, "1", 0, "L", true, 0, "")
	pdf.CellFormat(15, 8, tr.Qty, "1", 0, "C", true, 0, "")
	pdf.CellFormat(45, 8, tr.UnitPrice, "1", 0, "R", true, 0, "")
	pdf.CellFormat(45, 8, tr.Total, "1", 0, "R", true, 0, "")
	pdf.Ln(8)

	// Items data
	pdf.SetFont("Arial", "", 9)
	pdf.SetFillColor(255, 255, 255)

	subtotal := 0.0
	for i, item := range sale.SaleItems {
		// Check if we need a new page
		if pdf.GetY() > 250 {
			pdf.AddPage()
			// Re-add headers
			pdf.SetFont("Arial", "B", 10)
			pdf.SetFillColor(220, 220, 220)
			pdf.CellFormat(10, 8, tr.No, "1", 0, "C", true, 0, "")
			pdf.CellFormat(50, 8, tr.Description, "1", 0, "L", true, 0, "")
			pdf.CellFormat(25, 8, tr.Location, "1", 0, "L", true, 0, "")
			pdf.CellFormat(15, 8, tr.Qty, "1", 0, "C", true, 0, "")
			pdf.CellFormat(45, 8, tr.UnitPrice, "1", 0, "R", true, 0, "")
			pdf.CellFormat(45, 8, tr.Total, "1", 0, "R", true, 0, "")
			pdf.Ln(8)
			pdf.SetFont("Arial", "", 9)
			pdf.SetFillColor(255, 255, 255)
		}

		// Item data
		itemNumber := strconv.Itoa(i + 1)
		description := tr.Product
		if item.ProductName != "" {
			description = item.ProductName
		} else if item.Product.ID != 0 {
			description = item.Product.Name
		}

		quantity := strconv.Itoa(int(item.Quantity))
		unitPrice := p.formatRupiah(item.UnitPrice)
		totalPrice := p.formatRupiah(item.TotalPrice)

		pdf.CellFormat(10, 6, itemNumber, "1", 0, "C", false, 0, "")
		pdf.CellFormat(50, 6, description, "1", 0, "L", false, 0, "")
		pdf.CellFormat(25, 6, item.Location, "1", 0, "L", false, 0, "")
		pdf.CellFormat(15, 6, quantity, "1", 0, "C", false, 0, "")
		pdf.CellFormat(45, 6, unitPrice, "1", 0, "R", false, 0, "")
		pdf.CellFormat(45, 6, totalPrice, "1", 0, "R", false, 0, "")
		pdf.Ln(6)

		subtotal += item.TotalPrice
	}

	// Summary section
	pdf.Ln(5)
	pdf.SetFont("Arial", "B", 10)
	summary := p.buildSalePDFSummary(sale, subtotal)

	// Subtotal
	pdf.Cell(120, 6, "")
	pdf.Cell(25, 6, tr.Subtotal+":")
	pdf.Cell(45, 6, p.formatRupiah(subtotal))
	pdf.Ln(6)

	// Discount
	if summary.DiscountAmount > 0 {
		pdf.Cell(120, 6, "")
		pdf.Cell(25, 6, fmt.Sprintf("%s (%.1f%%):", tr.Discount, sale.DiscountPercent))
		pdf.Cell(45, 6, "-"+p.formatRupiah(summary.DiscountAmount))
		pdf.Ln(6)
	}

	// Taxes
	if summary.PPNAmount > 0 {
		pdf.Cell(120, 6, "")
		pdf.Cell(25, 6, fmt.Sprintf("%s (%.1f%%):", tr.PPN, summary.PPNRate))
		pdf.Cell(45, 6, p.formatRupiah(summary.PPNAmount))
		pdf.Ln(6)
	}

	if summary.PPhAmount > 0 {
		pdf.Cell(120, 6, "")
		pdf.Cell(25, 6, fmt.Sprintf("%s (%.1f%%):", tr.PPh, summary.PPhPercent))
		pdf.Cell(45, 6, "-"+p.formatRupiah(summary.PPhAmount))
		pdf.Ln(6)
	}

	// Shipping
	if summary.ShippingCost > 0 {
		pdf.Cell(120, 6, "")
		pdf.Cell(25, 6, tr.Shipping+":")
		pdf.Cell(45, 6, p.formatRupiah(summary.ShippingCost))
		pdf.Ln(6)
	}

	// Total
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(120, 8, "")
	pdf.Cell(25, 8, tr.Total+":")
	pdf.Cell(45, 8, p.formatRupiah(summary.TotalAmount))
	pdf.Ln(10)

	// Payment info
	if sale.PaymentTerms != "" {
		pdf.SetFont("Arial", "", 10)
		pdf.Cell(190, 5, fmt.Sprintf("%s:  %s", tr.PaymentTerms, sale.PaymentTerms))
		pdf.Ln(5)
	}

	p.renderSalePaymentSchedule(pdf, sale, tr)

	// Transfer to section (bank info)
	if bankInfo, _ := p.getBankInfoForSale(sale); bankInfo != nil {
		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(190, 6, tr.TransferTo+":")
		pdf.Ln(6)
		pdf.SetFont("Arial", "", 10)
		// Bank Name
		pdf.Cell(190, 5, fmt.Sprintf("%s: %s", tr.BankName, strings.TrimSpace(bankInfo.BankName)))
		pdf.Ln(6)
		// Account Number
		pdf.Cell(190, 5, fmt.Sprintf("%s: %s", tr.AccountNumber, strings.TrimSpace(bankInfo.AccountNo)))
		pdf.Ln(6)
		// Atas Nama
		if strings.TrimSpace(bankInfo.AccountHolderName) != "" {
			pdf.Cell(190, 5, fmt.Sprintf("%s: %s", tr.AtasNama, strings.TrimSpace(bankInfo.AccountHolderName)))
			pdf.Ln(6)
		}
		// Bank Branch
		if strings.TrimSpace(bankInfo.Branch) != "" {
			pdf.Cell(190, 5, fmt.Sprintf("%s: %s", tr.BankBranch, strings.TrimSpace(bankInfo.Branch)))
			pdf.Ln(6)
		}
	}

	// Notes
	if sale.Notes != "" {
		pdf.Ln(5)
		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(190, 6, tr.Notes+":")
		pdf.Ln(6)
		pdf.SetFont("Arial", "", 9)
		pdf.MultiCell(190, 4, sale.Notes, "", "", false)
	}

	// Footer
	pdf.Ln(10)
	pdf.SetFont("Arial", "I", 8)
	pdf.Cell(190, 4, fmt.Sprintf("%s %s", tr.GeneratedOn, time.Now().Format("02/01/2006 15:04")))

	// Output to buffer
	var buf bytes.Buffer
	err = pdf.Output(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %v", err)
	}

	return buf.Bytes(), nil
}

// GenerateSalesReportPDF generates a PDF for sales report (invoice-like style)
func (p *PDFService) GenerateSalesReportPDF(sales []models.Sale, startDate, endDate string) ([]byte, error) {
	utils.SanitizeExportData(sales)
	language := utils.GetUserLanguageFromSettings(p.db)
	loc := func(key, fallback string) string {
		t := utils.T(key, language)
		if t == key {
			return fallback
		}
		return t
	}
	// Create new PDF document (portrait for invoice-like look)
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	// Header: unified letterhead with logo + company info
	companyInfo, _ := p.getCompanyInfo()
	contentY := drawUnifiedLetterhead(pdf, companyInfo, 35)
	lm, _, _, _ := pdf.GetMargins()
	pageW, pageH := pdf.GetPageSize()
	rm := float64(15)
	contentW := pageW - lm - rm

	// Title
	pdf.SetY(contentY)
	pdf.SetFont("Arial", "B", 18)
	pdf.SetTextColor(25, 55, 109)
	pdf.Cell(contentW, 10, strings.ToUpper(loc("sales_report", "SALES REPORT")))
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(10)

	// Period and generated timestamp
	pdf.SetFont("Arial", "", 11)
	if startDate != "" && endDate != "" {
		pdf.Cell(contentW, 6, fmt.Sprintf("%s: %s %s %s", loc("period", "Period"), startDate, loc("to", "to"), endDate))
	} else {
		pdf.Cell(contentW, 6, fmt.Sprintf("%s: %s", loc("period", "Period"), "All Time"))
	}
	pdf.Ln(6)
	pdf.Cell(contentW, 5, fmt.Sprintf("%s: %s", loc("generated_on", "Generated on"), time.Now().Format("02/01/2006 15:04")))
	pdf.Ln(8)

	// Ensure table starts below header area
	pdf.SetX(lm)

	// Fixed column widths for consistent layout (safe for printing)
	// Total: 12 + 18 + 20 + 35 + 12 + 13 + 15 + 15 + 15 = 155mm
	widths := []float64{12, 18, 20, 35, 12, 13, 15, 15, 15}

	drawHeader := func() {
		pdf.SetX(lm)
		pdf.SetFont("Arial", "B", 7)
		pdf.SetFillColor(220, 220, 220)
		pdf.CellFormat(widths[0], 7, loc("date", "Date"), "1", 0, "C", true, 0, "")
		pdf.CellFormat(widths[1], 7, "Sale Code", "1", 0, "C", true, 0, "")
		pdf.CellFormat(widths[2], 7, loc("invoice_number", "Invoice No."), "1", 0, "C", true, 0, "")
		pdf.CellFormat(widths[3], 7, loc("customer", "Customer"), "1", 0, "C", true, 0, "")
		pdf.CellFormat(widths[4], 7, "Type", "1", 0, "C", true, 0, "")
		pdf.CellFormat(widths[5], 7, loc("status", "Status"), "1", 0, "C", true, 0, "")
		pdf.CellFormat(widths[6], 7, loc("amount", "Amount"), "1", 0, "C", true, 0, "")
		pdf.CellFormat(widths[7], 7, loc("paid", "Paid"), "1", 0, "C", true, 0, "")
		pdf.CellFormat(widths[8], 7, loc("outstanding", "Outstanding"), "1", 0, "C", true, 0, "")
		pdf.Ln(7)
	}

	// Draw table header
	drawHeader()

	// Table data
	pdf.SetFont("Arial", "", 7)
	pdf.SetFillColor(255, 255, 255)

	totalAmount := 0.0
	totalPaid := 0.0
	totalOutstanding := 0.0

	for _, sale := range sales {
		// New page handling
		if pdf.GetY() > pageH-25 {
			pdf.AddPage()
			p.addCompanyLetterhead(pdf)
			drawHeader()
		}
		// Row data
		date := sale.Date.Format("02/01/06")
		customerName := "N/A"
		if sale.Customer.ID != 0 {
			customerName = sale.Customer.Name
			// Adjusted for narrower customer column
			if len(customerName) > 22 {
				customerName = customerName[:19] + "..."
			}
		}
		invoiceNumber := sale.InvoiceNumber
		if invoiceNumber == "" {
			invoiceNumber = "-"
		}
		// Shorten code if too long
		saleCode := sale.Code
		if len(saleCode) > 12 {
			saleCode = saleCode[:10] + ".."
		}
		// Format amounts more compact
		amount := p.formatRupiahCompact(sale.TotalAmount)
		paid := p.formatRupiahCompact(sale.PaidAmount)
		outstanding := p.formatRupiahCompact(sale.OutstandingAmount)

		pdf.SetX(lm)
		pdf.CellFormat(widths[0], 6.5, date, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[1], 6.5, saleCode, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[2], 6.5, invoiceNumber, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[3], 6.5, customerName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[4], 6.5, sale.Type, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[5], 6.5, sale.Status, "1", 0, "C", false, 0, "")
		pdf.CellFormat(widths[6], 6.5, amount, "1", 0, "R", false, 0, "")
		pdf.CellFormat(widths[7], 6.5, paid, "1", 0, "R", false, 0, "")
		pdf.CellFormat(widths[8], 6.5, outstanding, "1", 0, "R", false, 0, "")
		pdf.Ln(6.5)

		// Accumulate totals
		totalAmount += sale.TotalAmount
		totalPaid += sale.PaidAmount
		totalOutstanding += sale.OutstandingAmount
	}

	// Summary section
	pdf.Ln(3)
	pdf.SetX(lm)
	pdf.SetFont("Arial", "B", 8)
	pdf.SetFillColor(240, 240, 240)
	leftGroup := widths[0] + widths[1] + widths[2] + widths[3] + widths[4] + widths[5]
	pdf.CellFormat(leftGroup, 6, strings.ToUpper(loc("total", "TOTAL")), "1", 0, "R", true, 0, "")
	pdf.CellFormat(widths[6], 6, p.formatRupiahCompact(totalAmount), "1", 0, "R", true, 0, "")
	pdf.CellFormat(widths[7], 6, p.formatRupiahCompact(totalPaid), "1", 0, "R", true, 0, "")
	pdf.CellFormat(widths[8], 6, p.formatRupiahCompact(totalOutstanding), "1", 0, "R", true, 0, "")

	// Statistics
	pdf.Ln(10)
	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(contentW, 6, strings.ToUpper(loc("sales_summary", "SUMMARY STATISTICS")))
	pdf.Ln(6)

	pdf.SetFont("Arial", "", 9)
	pdf.Cell(contentW/2, 5, fmt.Sprintf("%s: %d", loc("total_sales", "Total Sales"), len(sales)))
	pdf.Cell(contentW/2, 5, fmt.Sprintf("%s: %s", loc("total_amount", "Total Amount"), p.formatRupiah(totalAmount)))
	pdf.Ln(5)
	pdf.Cell(contentW/2, 5, fmt.Sprintf("%s: %s", loc("total_paid", "Total Paid"), p.formatRupiah(totalPaid)))
	pdf.Cell(contentW/2, 5, fmt.Sprintf("%s: %s", loc("outstanding", "Outstanding"), p.formatRupiah(totalOutstanding)))
	pdf.Ln(5)
	if len(sales) > 0 {
		avgAmount := totalAmount / float64(len(sales))
		pdf.Cell(contentW/2, 5, fmt.Sprintf("%s: %s", loc("average_sale_amount", "Average Sale Amount"), p.formatRupiah(avgAmount)))
	}

	// Output to buffer
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to generate sales report PDF: %v", err)
	}
	return buf.Bytes(), nil
}

// GenerateSalesSummaryPDF generates a PDF for SSOT Sales Summary (similar to trial balance format)
func (p *PDFService) GenerateSalesSummaryPDF(summary interface{}) ([]byte, error) {
	utils.SanitizeExportData(summary)
	if summary == nil {
		return nil, fmt.Errorf("sales summary data is required")
	}
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// Layout helpers
	lm, tm, rm, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	contentW := pageW - lm - rm

	// Company info
	companyInfo, err := p.getCompanyInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get company info: %v", err)
	}

	// Header: logo left, text right (similar to trial balance)
	logoX, logoY, logoSize := lm, tm, 35.0
	logoAdded := false
	if strings.TrimSpace(companyInfo.CompanyLogo) != "" {
		logoPath := companyInfo.CompanyLogo
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

	companyInfoX := pageW - rm
	companyInfoY := tm
	pdf.SetFont("Arial", "B", 12)
	nameW := pdf.GetStringWidth(companyInfo.CompanyName)
	pdf.SetXY(companyInfoX-nameW, companyInfoY)
	pdf.Cell(nameW, 6, companyInfo.CompanyName)

	pdf.SetFont("Arial", "", 9)
	addrW := pdf.GetStringWidth(companyInfo.CompanyAddress)
	pdf.SetXY(companyInfoX-addrW, companyInfoY+8)
	pdf.Cell(addrW, 4, companyInfo.CompanyAddress)

	phoneText := fmt.Sprintf("Phone: %s", companyInfo.CompanyPhone)
	phoneW := pdf.GetStringWidth(phoneText)
	pdf.SetXY(companyInfoX-phoneW, companyInfoY+14)
	pdf.Cell(phoneW, 4, phoneText)

	// Divider line
	pdf.SetDrawColor(238, 238, 238)
	pdf.SetLineWidth(0.2)
	pdf.Line(lm, tm+45, pageW-rm, tm+45)

	// Title
	pdf.SetY(tm + 55)
	pdf.SetX(lm)
	pdf.SetFont("Arial", "B", 22)
	pdf.SetTextColor(51, 51, 51)
	pdf.Cell(contentW, 10, "SALES SUMMARY REPORT")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(12)

	// Details two-column
	pdf.SetFont("Arial", "B", 9)
	// Try to get start/end date from map
	var smap map[string]interface{}
	if b, err := json.Marshal(summary); err == nil {
		_ = json.Unmarshal(b, &smap)
	}
	startStr, endStr := "", ""
	if v, ok := smap["start_date"].(string); ok {
		startStr = v
	}
	if v, ok := smap["end_date"].(string); ok {
		endStr = v
	}

	if startStr != "" && endStr != "" {
		pdf.SetX(lm)
		pdf.Cell(20, 5, "Period:")
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(102, 102, 102)
		pdf.Cell(60, 5, fmt.Sprintf("%s to %s", startStr, endStr))
	} else {
		pdf.SetX(lm)
		pdf.Cell(20, 5, "Period:")
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(102, 102, 102)
		pdf.Cell(60, 5, "All Time")
	}

	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(0, 0, 0)
	rightX := lm + contentW - 60
	pdf.SetX(rightX)
	pdf.Cell(26, 5, "Generated:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(34, 5, time.Now().Format("02/01/2006 15:04"))
	pdf.Ln(12)

	// Summary section removed as per user request

	// Sales By Customer table - reduced widths: 45 + 20 + 30 + 30 = 125mm
	// Top Customers table (similar to trial balance account table)
	getList := func(keys ...string) []interface{} {
		for _, k := range keys {
			if v, ok := smap[k]; ok {
				if arr, ok := v.([]interface{}); ok {
					return arr
				}
			}
		}
		return nil
	}
	getStr := func(m map[string]interface{}, keys ...string) string {
		for _, k := range keys {
			if v, ok := m[k]; ok {
				if s, ok := v.(string); ok {
					return s
				}
			}
		}
		return ""
	}
	getNum := func(m map[string]interface{}, keys ...string) float64 {
		for _, k := range keys {
			if v, ok := m[k]; ok {
				return getNumFrom(v)
			}
		}
		return 0
	}

	// Period sales section removed as per user request

	// Top Customers table
	if customers := getList("sales_by_customer", "top_customers", "TopCustomers"); len(customers) > 0 {
		pdf.Ln(5)
		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(0, 6, "Sales By Customer")
		pdf.Ln(8)
		pdf.SetX(lm)
		pdf.SetFont("Arial", "B", 7.5)
		pdf.SetFillColor(220, 220, 220)
		pdf.CellFormat(55, 8, "Customer Name", "1", 0, "C", true, 0, "")
		pdf.CellFormat(25, 8, "Transactions", "1", 0, "C", true, 0, "")
		pdf.CellFormat(35, 8, "Total Sales", "1", 0, "C", true, 0, "")
		pdf.CellFormat(40, 8, "Average Order", "1", 0, "C", true, 0, "")
		pdf.Ln(8)

		pdf.SetFont("Arial", "", 7.5)
		pdf.SetFillColor(255, 255, 255)
		limit := len(customers)
		if limit > 15 {
			limit = 15
		}
		for i := 0; i < limit; i++ {
			if cm, ok := customers[i].(map[string]interface{}); ok {
				// Check if we need a new page
				if pdf.GetY() > 250 {
					pdf.AddPage()
					// Re-add headers
					pdf.SetX(lm)
					pdf.SetFont("Arial", "B", 7.5)
					pdf.SetFillColor(220, 220, 220)
					pdf.CellFormat(55, 8, "Customer Name", "1", 0, "C", true, 0, "")
					pdf.CellFormat(25, 8, "Transactions", "1", 0, "C", true, 0, "")
					pdf.CellFormat(35, 8, "Total Sales", "1", 0, "C", true, 0, "")
					pdf.CellFormat(40, 8, "Average Order", "1", 0, "C", true, 0, "")
					pdf.Ln(8)
					pdf.SetFont("Arial", "", 7.5)
					pdf.SetFillColor(255, 255, 255)
				}

				name := getStr(cm, "customer_name", "name")
				if len(name) > 32 {
					name = name[:29] + "..."
				}
				count := getNum(cm, "transaction_count", "count", "orders", "total_sales")
				amount := getNum(cm, "total_amount", "amount", "total_sales")
				avgOrderValue := getNum(cm, "average_order", "average_transaction", "average_order_value")

				pdf.SetX(lm)
				pdf.CellFormat(55, 6, name, "1", 0, "L", false, 0, "")
				pdf.CellFormat(25, 6, fmt.Sprintf("%.0f", count), "1", 0, "C", false, 0, "")
				pdf.CellFormat(35, 6, p.formatRupiah(amount), "1", 0, "R", false, 0, "")
				pdf.CellFormat(40, 6, p.formatRupiah(avgOrderValue), "1", 0, "R", false, 0, "")
				pdf.Ln(6)
			}
		}
	}

	// Top Products table - Items Sold section
	if products := getList("sales_by_product", "top_products", "TopProducts"); len(products) > 0 {
		pdf.Ln(10)
		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(0, 6, "Items Sold")
		pdf.Ln(8)
		pdf.SetX(lm)
		pdf.SetFont("Arial", "B", 7.5)
		pdf.SetFillColor(220, 220, 220)
		pdf.CellFormat(55, 8, "Product", "1", 0, "C", true, 0, "")
		pdf.CellFormat(25, 8, "Qty", "1", 0, "C", true, 0, "")
		pdf.CellFormat(35, 8, "Unit Price", "1", 0, "C", true, 0, "")
		pdf.CellFormat(40, 8, "Total", "1", 0, "C", true, 0, "")
		pdf.CellFormat(0, 8, "Date", "1", 1, "C", true, 0, "")

		pdf.SetFont("Arial", "", 7.5)
		pdf.SetFillColor(255, 255, 255)
		limit := len(products)
		if limit > 15 {
			limit = 15
		}
		for i := 0; i < limit; i++ {
			if pm, ok := products[i].(map[string]interface{}); ok {
				// Check if we need a new page
				if pdf.GetY() > 250 {
					pdf.AddPage()
					// Re-add headers
					pdf.SetX(lm)
					pdf.SetFont("Arial", "B", 7.5)
					pdf.SetFillColor(220, 220, 220)
					pdf.CellFormat(55, 8, "Product", "1", 0, "C", true, 0, "")
					pdf.CellFormat(25, 8, "Qty", "1", 0, "C", true, 0, "")
					pdf.CellFormat(35, 8, "Unit Price", "1", 0, "C", true, 0, "")
					pdf.CellFormat(40, 8, "Total", "1", 0, "C", true, 0, "")
					pdf.CellFormat(0, 8, "Date", "1", 1, "C", true, 0, "")
					pdf.SetFont("Arial", "", 7.5)
					pdf.SetFillColor(255, 255, 255)
				}

				name := getStr(pm, "product_name", "name")
				if len(name) > 32 {
					name = name[:29] + "..."
				}
				qty := getNum(pm, "quantity_sold", "quantity", "qty")
				unitPrice := getNum(pm, "average_price", "unit_price")
				amount := getNum(pm, "total_amount", "amount", "total_revenue")
				// Get date if available
				dateStr := getStr(pm, "date", "purchase_date", "sale_date")
				if dateStr == "" {
					dateStr = "-"
				}

				pdf.SetX(lm)
				pdf.CellFormat(55, 6, name, "1", 0, "L", false, 0, "")
				pdf.CellFormat(25, 6, fmt.Sprintf("%.0f pcs", qty), "1", 0, "C", false, 0, "")
				pdf.CellFormat(35, 6, p.formatRupiah(unitPrice), "1", 0, "R", false, 0, "")
				pdf.CellFormat(40, 6, p.formatRupiah(amount), "1", 0, "R", false, 0, "")
				pdf.CellFormat(0, 6, dateStr, "1", 1, "C", false, 0, "")
			}
		}
	}

	// Sales by Status table
	/*
		if statuses := getList("sales_by_status"); len(statuses) > 0 {
			pdf.Ln(5)
			pdf.SetFont("Arial", "B", 12)
			pdf.SetFillColor(220, 220, 220)
			pdf.CellFormat(80, 8, "Status", "1", 0, "C", true, 0, "")
			pdf.CellFormat(50, 8, "Amount", "1", 0, "C", true, 0, "")
			pdf.CellFormat(60, 8, "Percentage", "1", 0, "C", true, 0, "")
			pdf.Ln(8)

			pdf.SetFont("Arial", "", 9)
			pdf.SetFillColor(255, 255, 255)
			limit := len(statuses)
			if limit > 10 { limit = 10 }
			for i := 0; i < limit; i++ {
				if sm, ok := statuses[i].(map[string]interface{}); ok {
					// Check if we need a new page
					if pdf.GetY() > 250 {
						pdf.AddPage()
						// Re-add headers
						pdf.SetFont("Arial", "B", 12)
						pdf.SetFillColor(220, 220, 220)
						pdf.CellFormat(80, 8, "Status", "1", 0, "C", true, 0, "")
						pdf.CellFormat(50, 8, "Amount", "1", 0, "C", true, 0, "")
						pdf.CellFormat(60, 8, "Percentage", "1", 0, "C", true, 0, "")
						pdf.Ln(8)
						pdf.SetFont("Arial", "", 9)
						pdf.SetFillColor(255, 255, 255)
					}

					status := getStr(sm, "status")
					if len(status) > 35 { status = status[:32] + "..." }
					amount := getNum(sm, "amount")
					percentage := getNum(sm, "percentage")

					pdf.CellFormat(80, 6, status, "1", 0, "L", false, 0, "")
					pdf.CellFormat(50, 6, p.formatRupiah(amount), "1", 0, "R", false, 0, "")
					pdf.CellFormat(60, 6, fmt.Sprintf("%.1f%%", percentage), "1", 0, "R", false, 0, "")
					pdf.Ln(6)
				}
			}
		}
	*/

	// Items Sold section - grouped by customer (similar to purchase report)
	if customers := getList("sales_by_customer", "top_customers", "TopCustomers"); len(customers) > 0 {
		hasItems := false
		for _, c := range customers {
			if cm, ok := c.(map[string]interface{}); ok {
				if items, ok := cm["items"].([]interface{}); ok && len(items) > 0 {
					hasItems = true
					break
				}
			}
		}

		if hasItems {
			// Check if we need a new page for items section
			_, pageH := pdf.GetPageSize()
			if pdf.GetY() > pageH-80 {
				pdf.AddPage()
			}

			pdf.Ln(10)
			pdf.SetFont("Arial", "B", 12)
			pdf.Cell(0, 8, "Items Sold")
			pdf.Ln(10)

			// Loop through customers and their items
			for _, c := range customers {
				if cm, ok := c.(map[string]interface{}); ok {
					if items, ok := cm["items"].([]interface{}); ok && len(items) > 0 {
						// Check if we need a new page for this customer
						if pdf.GetY() > pageH-60 {
							pdf.AddPage()
						}

						// Customer name header - reduced width: 105 + 50 = 155mm
						customerName := getStr(cm, "customer_name", "name")
						if len(customerName) > 35 {
							customerName = customerName[:32] + "..."
						}
						pdf.SetX(lm)
						pdf.SetFont("Arial", "B", 9)
						pdf.SetFillColor(230, 240, 255)
						pdf.CellFormat(105, 7, customerName, "1", 0, "L", true, 0, "")
						pdf.CellFormat(50, 7, fmt.Sprintf("%d items", len(items)), "1", 1, "R", true, 0, "")

						// Items table header - Total: 55 + 18 + 30 + 30 + 22 = 155mm
						pdf.SetX(lm)
						pdf.SetFont("Arial", "B", 7.5)
						pdf.SetFillColor(245, 245, 245)
						pdf.CellFormat(55, 6, "Product", "1", 0, "C", true, 0, "")
						pdf.CellFormat(18, 6, "Qty", "1", 0, "C", true, 0, "")
						pdf.CellFormat(30, 6, "Unit Price", "1", 0, "C", true, 0, "")
						pdf.CellFormat(30, 6, "Total", "1", 0, "C", true, 0, "")
						pdf.CellFormat(22, 6, "Date", "1", 1, "C", true, 0, "")

						// Items rows
						pdf.SetFont("Arial", "", 8)
						for _, item := range items {
							if im, ok := item.(map[string]interface{}); ok {
								// Check if we need a new page
								if pdf.GetY() > pageH-25 {
									pdf.AddPage()
									// Reprint header
									pdf.SetFont("Arial", "B", 8)
									pdf.SetFillColor(245, 245, 245)
									pdf.CellFormat(70, 6, "Product", "1", 0, "L", true, 0, "")
									pdf.CellFormat(20, 6, "Qty", "1", 0, "R", true, 0, "")
									pdf.CellFormat(35, 6, "Unit Price", "1", 0, "R", true, 0, "")
									pdf.CellFormat(35, 6, "Total", "1", 0, "R", true, 0, "")
									pdf.CellFormat(30, 6, "Date", "1", 1, "C", true, 0, "")
									pdf.SetFont("Arial", "", 8)
								}

								productName := getStr(im, "product_name", "name")
								if len(productName) > 35 {
									productName = productName[:32] + "..."
								}

								qty := getNum(im, "quantity", "qty")
								unit := getStr(im, "unit")
								qtyStr := fmt.Sprintf("%.0f %s", qty, unit)

								unitPrice := getNum(im, "unit_price", "price")
								totalPrice := getNum(im, "total_price", "line_total", "total")

								// Parse sale_date
								dateStr := getStr(im, "sale_date", "date")
								var formattedDate string
								if dateStr != "" {
									if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
										formattedDate = t.Format("02/01/2006")
									} else {
										formattedDate = dateStr
									}
								}

								pdf.CellFormat(70, 6, productName, "1", 0, "L", false, 0, "")
								pdf.CellFormat(20, 6, qtyStr, "1", 0, "R", false, 0, "")
								pdf.CellFormat(35, 6, p.formatRupiah(unitPrice), "1", 0, "R", false, 0, "")
								pdf.CellFormat(35, 6, p.formatRupiah(totalPrice), "1", 0, "R", false, 0, "")
								pdf.CellFormat(30, 6, formattedDate, "1", 1, "C", false, 0, "")
							}
						}

						// Customer subtotal
						customerTotal := getNum(cm, "total_amount", "amount", "total_sales")
						pdf.SetFont("Arial", "B", 9)
						pdf.SetFillColor(230, 245, 255)
						pdf.CellFormat(125, 6, fmt.Sprintf("Subtotal (%s)", getStr(cm, "customer_name", "name")), "1", 0, "R", true, 0, "")
						pdf.CellFormat(65, 6, p.formatRupiah(customerTotal), "1", 1, "R", true, 0, "")
						pdf.Ln(4)
					}
				}
			}
		}
	}

	// Footer
	pdf.Ln(10)
	pdf.SetFont("Arial", "I", 8)
	pdf.Cell(contentW, 4, fmt.Sprintf("Report generated on %s", time.Now().Format("02/01/2006 15:04")))

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to generate sales summary PDF: %v", err)
	}
	return buf.Bytes(), nil
}

// GenerateGeneralLedgerPDF generates PDF for general ledger report
func (p *PDFService) GenerateGeneralLedgerPDF(ledgerData interface{}, accountInfo string, startDate, endDate string) ([]byte, error) {
	utils.SanitizeExportData(ledgerData)
	accountInfo = utils.CleanExportText(accountInfo)
	// Create new PDF document
	pdf := gofpdf.New("P", "mm", "A4", "")
	// Adjust margins/padding for consistent layout
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.SetCellMargin(1.5)
	pdf.AddPage()

	// Invoice-like header
	lm, tm, rm, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	contentW := pageW - lm - rm

	companyInfo, err := p.getCompanyInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get company info: %v", err)
	}

	// Logo left
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

	// Company text right
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

	// Divider
	pdf.SetDrawColor(238, 238, 238)
	pdf.SetLineWidth(0.2)
	pdf.Line(lm, tm+45, pageW-rm, tm+45)

	// Title
	pdf.SetY(tm + 55)
	pdf.SetFont("Arial", "B", 22)
	pdf.SetTextColor(51, 51, 51)
	pdf.Cell(contentW, 10, "GENERAL LEDGER REPORT")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(12)

	// Details two-column
	pdf.SetFont("Arial", "B", 9)
	pdf.SetX(lm)
	pdf.Cell(25, 5, "Account:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(70, 5, accountInfo)

	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(0, 0, 0)
	rightX := lm + contentW - 70
	pdf.SetX(rightX)
	pdf.Cell(22, 5, "Period:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(48, 5, fmt.Sprintf("%s to %s", startDate, endDate))
	pdf.Ln(8)

	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetX(lm)
	pdf.Cell(25, 5, "Generated:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(contentW-25, 5, time.Now().Format("02/01/2006 15:04"))
	pdf.Ln(10)

	// Table headers with optimized widths (fit 180mm content width)
	// Date: 18mm, Reference: 25mm, Description: 75mm, Debit: 20mm, Credit: 20mm, Balance: 22mm
	pdf.SetFont("Arial", "B", 8)
	pdf.SetFillColor(220, 220, 220)
	pdf.CellFormat(18, 7, "Date", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 7, "Reference", "1", 0, "C", true, 0, "")
	pdf.CellFormat(75, 7, "Description", "1", 0, "L", true, 0, "")
	pdf.CellFormat(20, 7, "Debit", "1", 0, "R", true, 0, "")
	pdf.CellFormat(20, 7, "Credit", "1", 0, "R", true, 0, "")
	pdf.CellFormat(22, 7, "Balance", "1", 0, "R", true, 0, "")
	pdf.Ln(7)

	// Process ledger data based on its structure
	// Use smaller font (6pt) for better fit
	pdf.SetFont("Arial", "", 6)
	pdf.SetFillColor(255, 255, 255)

	// Normalize input to map[string]interface{} so we can iterate regardless of struct/map input
	var ledgerMap map[string]interface{}
	if m, ok := ledgerData.(map[string]interface{}); ok {
		ledgerMap = m
	} else {
		b, _ := json.Marshal(ledgerData)
		_ = json.Unmarshal(b, &ledgerMap)
	}

	if ledgerMap != nil {
		// Handle different possible data structures
		if accounts, exists := ledgerMap["accounts"]; exists {
			// Multiple accounts structure
			if accountsSlice, ok := accounts.([]interface{}); ok {
				for _, account := range accountsSlice {
					if accountMap, ok := account.(map[string]interface{}); ok {
						p.addAccountToLedgerPDF(pdf, accountMap)
					}
				}
			}
		} else if transactions, exists := ledgerMap["transactions"]; exists {
			// New field name from backend
			if transactionsSlice, ok := transactions.([]interface{}); ok {
				openingBalance := 0.0
				if opening, exists := ledgerMap["opening_balance"]; exists {
					if openingFloat, ok := opening.(float64); ok {
						openingBalance = openingFloat
					}
				}
				p.addEntriesToLedgerPDF(pdf, transactionsSlice, openingBalance)
			}
		} else if entries, exists := ledgerMap["entries"]; exists {
			// Legacy field name
			if entriesSlice, ok := entries.([]interface{}); ok {
				openingBalance := 0.0
				if opening, exists := ledgerMap["opening_balance"]; exists {
					if openingFloat, ok := opening.(float64); ok {
						openingBalance = openingFloat
					}
				}
				p.addEntriesToLedgerPDF(pdf, entriesSlice, openingBalance)
			}
		} else {
			// Fallback: treat entire data as single account
			p.addAccountToLedgerPDF(pdf, ledgerMap)
		}
	} else {
		// Handle simple data structure
		pdf.Cell(180, 6, "No ledger data available")
		pdf.Ln(6)
	}

	// Footer
	pdf.Ln(10)
	pdf.SetFont("Arial", "I", 8)
	pdf.Cell(180, 4, fmt.Sprintf("Report generated on %s", time.Now().Format("02/01/2006 15:04")))

	// Output to buffer
	var buf bytes.Buffer
	err = pdf.Output(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to generate general ledger PDF: %v", err)
	}

	return buf.Bytes(), nil
}

// addAccountToLedgerPDF adds account data to the PDF
func (p *PDFService) addAccountToLedgerPDF(pdf *gofpdf.Fpdf, accountData map[string]interface{}) {
	accountName := "All Accounts"
	accountCode := ""

	if name, exists := accountData["account_name"]; exists {
		if nameStr, ok := name.(string); ok && strings.TrimSpace(nameStr) != "" {
			accountName = nameStr
		}
	}
	if code, exists := accountData["account_code"]; exists {
		if codeStr, ok := code.(string); ok {
			accountCode = codeStr
		}
	}

	// Account header
	headerText := accountName
	if accountCode != "" {
		headerText = fmt.Sprintf("%s - %s", accountCode, accountName)
	}
	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(240, 240, 240)
	pdf.CellFormat(180, 8, headerText, "1", 0, "L", true, 0, "")
	pdf.Ln(8)

	// Get opening balance
	openingBalance := 0.0
	if opening, exists := accountData["opening_balance"]; exists {
		if openingFloat, ok := opening.(float64); ok {
			openingBalance = openingFloat
		}
	}

	// Add opening balance row
	pdf.SetFont("Arial", "I", 8)
	pdf.SetFillColor(250, 250, 250)
	pdf.CellFormat(20, 6, "-", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 6, "-", "1", 0, "C", true, 0, "")
	pdf.CellFormat(70, 6, "Opening Balance", "1", 0, "L", true, 0, "")
	pdf.CellFormat(22, 6, "-", "1", 0, "R", true, 0, "")
	pdf.CellFormat(22, 6, "-", "1", 0, "R", true, 0, "")
	pdf.CellFormat(21, 6, p.formatRupiah(openingBalance), "1", 0, "R", true, 0, "")
	pdf.Ln(6)

	// Add entries - support both "entries" and "transactions" keys
	if entries, exists := accountData["entries"]; exists {
		if entriesSlice, ok := entries.([]interface{}); ok {
			p.addEntriesToLedgerPDF(pdf, entriesSlice, openingBalance)
		}
	} else if txs, exists := accountData["transactions"]; exists {
		if entriesSlice, ok := txs.([]interface{}); ok {
			p.addEntriesToLedgerPDF(pdf, entriesSlice, openingBalance)
		}
	}

	pdf.Ln(5)
}

// splitDescriptionToTwoLines splits description text into maximum 2 lines
// Each line fits within the given width (in characters)
func splitDescriptionToTwoLines(text string, maxCharsPerLine int) (string, string) {
	if len(text) <= maxCharsPerLine {
		return text, ""
	}

	// Try to find a good break point (space, comma, pipe, dash)
	line1 := text[:maxCharsPerLine]
	line2 := text[maxCharsPerLine:]

	// Find last delimiter in line1 for better word wrap
	// Check last 20 chars for delimiter
	lastDelimiter := -1
	for i := len(line1) - 1; i >= max(0, maxCharsPerLine-20); i-- {
		if line1[i] == ' ' || line1[i] == '|' || line1[i] == '-' || line1[i] == ',' {
			lastDelimiter = i
			break
		}
	}

	if lastDelimiter > 0 {
		line2 = line1[lastDelimiter+1:] + line2
		line1 = line1[:lastDelimiter]
	}

	// Truncate line2 if too long - be conservative
	if len(line2) > maxCharsPerLine {
		line2 = line2[:maxCharsPerLine-3] + "..."
	}

	return strings.TrimSpace(line1), strings.TrimSpace(line2)
}

// max helper function
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// truncateToWidth truncates text to fit within specified width in mm
func truncateToWidth(pdf *gofpdf.Fpdf, text string, maxWidthMM float64) string {
	// Get current font settings
	currentWidth := pdf.GetStringWidth(text)

	// If it fits, return as is
	if currentWidth <= maxWidthMM {
		return text
	}

	// Binary search for the right length
	left := 0
	right := len(text)
	result := text

	for left < right {
		mid := (left + right + 1) / 2
		testStr := text[:mid] + "..."
		testWidth := pdf.GetStringWidth(testStr)

		if testWidth <= maxWidthMM {
			result = testStr
			left = mid
		} else {
			right = mid - 1
		}
	}

	return result
}

// addEntriesToLedgerPDF adds individual entries to the PDF
func (p *PDFService) addEntriesToLedgerPDF(pdf *gofpdf.Fpdf, entries []interface{}, openingBalance float64) {
	pdf.SetFont("Arial", "", 6)
	pdf.SetFillColor(255, 255, 255)

	// Add opening balance row if non-zero
	if openingBalance != 0 {
		pdf.SetFont("Arial", "I", 6)
		pdf.SetFillColor(250, 250, 250)
		pdf.CellFormat(18, 6, "", "1", 0, "C", true, 0, "")
		pdf.CellFormat(25, 6, "", "1", 0, "L", true, 0, "")
		pdf.CellFormat(75, 6, "Opening Balance", "1", 0, "L", true, 0, "")
		pdf.CellFormat(20, 6, "", "1", 0, "R", true, 0, "")
		pdf.CellFormat(20, 6, "", "1", 0, "R", true, 0, "")
		pdf.CellFormat(22, 6, p.formatRupiah(openingBalance), "1", 0, "R", true, 0, "")
		pdf.Ln(6)
		pdf.SetFillColor(255, 255, 255)
	}

	runningBalance := openingBalance

	for _, entry := range entries {
		if entryMap, ok := entry.(map[string]interface{}); ok {
			// Extract entry data
			date := "-"
			ref := "-"
			desc := "-"
			debit := 0.0
			credit := 0.0

			if dateVal, exists := entryMap["date"]; exists {
				if dateStr, ok := dateVal.(string); ok {
					if parsedDate, err := time.Parse("2006-01-02", dateStr); err == nil {
						date = parsedDate.Format("02/01/2006")
					} else if parsedDate, err := time.Parse(time.RFC3339, dateStr); err == nil {
						date = parsedDate.Format("02/01/2006")
					} else if len(dateStr) >= 10 {
						date = dateStr[:10]
					}
				}
			}

			if refVal, exists := entryMap["reference"]; exists {
				if refStr, ok := refVal.(string); ok {
					ref = refStr
					// Smart truncation for reference (max 22 chars for 25mm width at font 7)
					if len(ref) > 22 {
						ref = ref[:19] + "..."
					}
				}
			}

			descLine1 := ""
			descLine2 := ""
			if descVal, exists := entryMap["description"]; exists {
				if descStr, ok := descVal.(string); ok {
					desc = descStr
					// Split description into max 2 lines (70 chars per line for 75mm width at font 6)
					// More conservative limit to prevent overflow
					descLine1, descLine2 = splitDescriptionToTwoLines(desc, 70)

					// Additional safety: truncate based on actual width (73mm available after padding)
					pdf.SetFont("Arial", "", 6)
					descLine1 = truncateToWidth(pdf, descLine1, 73)
					if descLine2 != "" {
						descLine2 = truncateToWidth(pdf, descLine2, 73)
					}
				}
			}

			// Debit amount: support keys "debit" and "debit_amount" and string numbers
			if debitVal, exists := entryMap["debit"]; exists {
				if v, ok := debitVal.(float64); ok {
					debit = v
				} else if s, ok := debitVal.(string); ok {
					if f, err := strconv.ParseFloat(s, 64); err == nil {
						debit = f
					}
				}
			} else if debitVal, exists := entryMap["debit_amount"]; exists {
				if v, ok := debitVal.(float64); ok {
					debit = v
				} else if s, ok := debitVal.(string); ok {
					if f, err := strconv.ParseFloat(s, 64); err == nil {
						debit = f
					}
				}
			}

			// Credit amount: support keys "credit" and "credit_amount" and string numbers
			if creditVal, exists := entryMap["credit"]; exists {
				if v, ok := creditVal.(float64); ok {
					credit = v
				} else if s, ok := creditVal.(string); ok {
					if f, err := strconv.ParseFloat(s, 64); err == nil {
						credit = f
					}
				}
			} else if creditVal, exists := entryMap["credit_amount"]; exists {
				if v, ok := creditVal.(float64); ok {
					credit = v
				} else if s, ok := creditVal.(string); ok {
					if f, err := strconv.ParseFloat(s, 64); err == nil {
						credit = f
					}
				}
			}

			// Calculate running balance
			runningBalance += debit - credit

			// Determine row height based on description lines
			rowHeight := 6.0
			if descLine2 != "" {
				rowHeight = 10.0 // Double height for 2-line description
			}

			// Add row to PDF with updated column widths
			pdf.SetFont("Arial", "", 6)
			pdf.CellFormat(18, rowHeight, date, "1", 0, "C", false, 0, "")
			pdf.CellFormat(25, rowHeight, ref, "1", 0, "L", false, 0, "")

			// Description column with multi-line support
			descX, descY := pdf.GetXY()

			// Use MultiCell for better text wrapping control
			pdf.SetXY(descX, descY)
			pdf.SetFont("Arial", "", 6)

			if descLine2 != "" {
				// Two lines - draw cell border first
				currentX, currentY := pdf.GetXY()
				pdf.Rect(currentX, currentY, 75, rowHeight, "D") // Draw border

				// Draw text inside with padding
				pdf.SetXY(currentX+1, currentY+1)
				pdf.Cell(73, 4, descLine1)
				pdf.SetXY(currentX+1, currentY+5)
				pdf.Cell(73, 4, descLine2)

				// Move to end of cell
				pdf.SetXY(currentX+75, currentY)
			} else {
				// Single line - use CellFormat with proper alignment
				pdf.CellFormat(75, rowHeight, descLine1, "1", 0, "L", false, 0, "")
			}

			// Move to next columns
			pdf.SetXY(descX+75, descY)

			// Debit column
			if debit > 0 {
				pdf.CellFormat(20, rowHeight, p.formatRupiah(debit), "1", 0, "R", false, 0, "")
			} else {
				pdf.CellFormat(20, rowHeight, "-", "1", 0, "R", false, 0, "")
			}

			// Credit column
			if credit > 0 {
				pdf.CellFormat(20, rowHeight, p.formatRupiah(credit), "1", 0, "R", false, 0, "")
			} else {
				pdf.CellFormat(20, rowHeight, "-", "1", 0, "R", false, 0, "")
			}

			// Balance column
			pdf.CellFormat(22, rowHeight, p.formatRupiah(runningBalance), "1", 0, "R", false, 0, "")
			pdf.Ln(rowHeight)

			// Check if we need a new page
			if pdf.GetY() > 260 {
				pdf.AddPage()
				// Re-add headers with matching widths and fonts
				pdf.SetFont("Arial", "B", 8)
				pdf.SetFillColor(220, 220, 220)
				pdf.CellFormat(18, 7, "Date", "1", 0, "C", true, 0, "")
				pdf.CellFormat(25, 7, "Reference", "1", 0, "C", true, 0, "")
				pdf.CellFormat(75, 7, "Description", "1", 0, "L", true, 0, "")
				pdf.CellFormat(20, 7, "Debit", "1", 0, "R", true, 0, "")
				pdf.CellFormat(20, 7, "Credit", "1", 0, "R", true, 0, "")
				pdf.CellFormat(22, 7, "Balance", "1", 0, "R", true, 0, "")
				pdf.Ln(7)
				pdf.SetFont("Arial", "", 6)
				pdf.SetFillColor(255, 255, 255)
			}
		}
	}
}

// GeneratePaymentReportPDF generates a PDF for payments report
func (p *PDFService) GeneratePaymentReportPDF(data interface{}) ([]byte, error) {
	utils.SanitizeExportData(data)
	// Extract payments data from interface{}
	var payments []models.Payment
	switch v := data.(type) {
	case []models.Payment:
		payments = v
	default:
		return nil, fmt.Errorf("invalid data format for GeneratePaymentReportPDF")
	}

	// For now, use default date range
	startDate := ""
	endDate := ""
	// Create new PDF document
	pdf := gofpdf.New("L", "mm", "A4", "") // Landscape orientation
	pdf.AddPage()

	// Unified letterhead
	companyInfo, _ := p.getCompanyInfo()
	contentY := drawUnifiedLetterhead(pdf, companyInfo, 35)

	// Title
	pdf.SetY(contentY)
	pdf.SetFont("Arial", "B", 16)
	pdf.SetTextColor(25, 55, 109)
	pdf.Cell(270, 10, "PAYMENT REPORT")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(10)

	// Date range
	pdf.SetFont("Arial", "", 12)
	if startDate != "" && endDate != "" {
		pdf.Cell(270, 6, fmt.Sprintf("Period: %s to %s", startDate, endDate))
	} else {
		pdf.Cell(270, 6, "Period: All Time")
	}
	pdf.Ln(10)

	// Report generated info
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(270, 5, fmt.Sprintf("Generated on: %s", time.Now().Format("02/01/2006 15:04")))
	pdf.Ln(10)

	// Table headers
	pdf.SetFont("Arial", "B", 8)
	pdf.SetFillColor(220, 220, 220)
	pdf.CellFormat(18, 8, "Date", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 8, "Payment Code", "1", 0, "C", true, 0, "")
	pdf.CellFormat(40, 8, "Contact", "1", 0, "L", true, 0, "")
	pdf.CellFormat(20, 8, "Method", "1", 0, "C", true, 0, "")
	pdf.CellFormat(44, 8, "Amount", "1", 0, "R", true, 0, "")
	pdf.CellFormat(18, 8, "Status", "1", 0, "C", true, 0, "")
	pdf.CellFormat(30, 8, "Reference", "1", 0, "L", true, 0, "")
	pdf.CellFormat(55, 8, "Notes", "1", 0, "L", true, 0, "")
	pdf.Ln(8)

	// Table data
	pdf.SetFont("Arial", "", 8)
	pdf.SetFillColor(255, 255, 255)

	totalAmount := 0.0
	completedCount := 0
	pendingCount := 0
	failedCount := 0

	for _, payment := range payments {
		// Check if we need a new page
		if pdf.GetY() > 180 {
			pdf.AddPage()
			// Re-add headers
			pdf.SetFont("Arial", "B", 8)
			pdf.SetFillColor(220, 220, 220)
			pdf.CellFormat(18, 8, "Date", "1", 0, "C", true, 0, "")
			pdf.CellFormat(25, 8, "Payment Code", "1", 0, "C", true, 0, "")
			pdf.CellFormat(40, 8, "Contact", "1", 0, "L", true, 0, "")
			pdf.CellFormat(20, 8, "Method", "1", 0, "C", true, 0, "")
			pdf.CellFormat(44, 8, "Amount", "1", 0, "R", true, 0, "")
			pdf.CellFormat(18, 8, "Status", "1", 0, "C", true, 0, "")
			pdf.CellFormat(30, 8, "Reference", "1", 0, "L", true, 0, "")
			pdf.CellFormat(55, 8, "Notes", "1", 0, "L", true, 0, "")
			pdf.Ln(8)
			pdf.SetFont("Arial", "", 8)
			pdf.SetFillColor(255, 255, 255)
		}

		// Payment data
		date := payment.Date.Format("02/01/06")
		contactName := "N/A"

		// Check if this is a PPN tax payment
		isPPNPayment := payment.PaymentType == models.PaymentTypeTaxPPN ||
			payment.PaymentType == models.PaymentTypeTaxPPNInput ||
			payment.PaymentType == models.PaymentTypeTaxPPNOutput ||
			strings.HasPrefix(payment.Code, "SETOR-PPN")

		if isPPNPayment {
			contactName = "Negara"
		} else if payment.Contact.ID != 0 {
			contactName = payment.Contact.Name
			// Truncate if too long
			if len(contactName) > 25 {
				contactName = contactName[:22] + "..."
			}
		}

		method := payment.Method
		if len(method) > 12 {
			method = method[:9] + "..."
		}

		amount := p.formatRupiah(payment.Amount)
		status := payment.Status
		reference := payment.Reference
		if len(reference) > 20 {
			reference = reference[:17] + "..."
		}

		notes := payment.Notes
		if len(notes) > 35 {
			notes = notes[:32] + "..."
		}

		pdf.CellFormat(18, 5, date, "1", 0, "C", false, 0, "")
		pdf.CellFormat(25, 5, payment.Code, "1", 0, "L", false, 0, "")
		pdf.CellFormat(40, 5, contactName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(20, 5, method, "1", 0, "C", false, 0, "")
		pdf.CellFormat(44, 5, amount, "1", 0, "R", false, 0, "")
		pdf.CellFormat(18, 5, status, "1", 0, "C", false, 0, "")
		pdf.CellFormat(30, 5, reference, "1", 0, "L", false, 0, "")
		pdf.CellFormat(55, 5, notes, "1", 0, "L", false, 0, "")
		pdf.Ln(5)

		// Accumulate totals
		totalAmount += payment.Amount
		switch payment.Status {
		case "COMPLETED":
			completedCount++
		case "PENDING":
			pendingCount++
		case "FAILED":
			failedCount++
		}
	}

	// Summary section
	pdf.Ln(3)
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(240, 240, 240)
	pdf.CellFormat(103, 6, "TOTAL", "1", 0, "R", true, 0, "")
	pdf.CellFormat(44, 6, p.formatRupiah(totalAmount), "1", 0, "R", true, 0, "")
	pdf.CellFormat(123, 6, fmt.Sprintf("Count: %d", len(payments)), "1", 0, "L", true, 0, "")

	// Statistics
	pdf.Ln(10)
	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(270, 6, "SUMMARY STATISTICS")
	pdf.Ln(6)

	pdf.SetFont("Arial", "", 9)
	pdf.Cell(135, 5, fmt.Sprintf("Total Payments: %d", len(payments)))
	pdf.Cell(135, 5, fmt.Sprintf("Total Amount: %s", p.formatRupiah(totalAmount)))
	pdf.Ln(5)
	pdf.Cell(135, 5, fmt.Sprintf("Completed: %d", completedCount))
	pdf.Cell(135, 5, fmt.Sprintf("Pending: %d", pendingCount))
	pdf.Ln(5)
	pdf.Cell(135, 5, fmt.Sprintf("Failed: %d", failedCount))

	if len(payments) > 0 {
		avgAmount := totalAmount / float64(len(payments))
		pdf.Cell(135, 5, fmt.Sprintf("Average Payment Amount: %s", p.formatRupiah(avgAmount)))
	}

	// Output to buffer
	var buf bytes.Buffer
	err := pdf.Output(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to generate payment report PDF: %v", err)
	}

	return buf.Bytes(), nil
}

// GeneratePaymentDetailPDF generates a PDF for a single payment detail (invoice-like style)
func (p *PDFService) GeneratePaymentDetailPDF(data interface{}) ([]byte, error) {
	utils.SanitizeExportData(data)
	// Extract payment from interface{}
	var payment *models.Payment
	switch v := data.(type) {
	case *models.Payment:
		payment = v
	case models.Payment:
		payment = &v
	default:
		return nil, fmt.Errorf("invalid data format for GeneratePaymentDetailPDF")
	}

	if payment == nil {
		return nil, fmt.Errorf("payment data is required")
	}

	// Create new PDF document with clean margins
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	// Unified letterhead
	companyInfo, err := p.getCompanyInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get company info: %v", err)
	}
	contentY := drawUnifiedLetterhead(pdf, companyInfo, 35)

	lm, _, _, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	rm := float64(15)
	contentW := pageW - lm - rm

	// === TITLE ===
	pdf.SetY(contentY)
	pdf.SetX(lm)
	pdf.SetFont("Arial", "B", 22)
	pdf.SetTextColor(25, 55, 109)
	pdf.Cell(contentW, 10, "PAYMENT RECEIPT")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(18)

	// === PAYMENT META ===
	pdf.SetFont("Arial", "B", 9)
	// Left side: Code & Reference
	pdf.SetX(lm)
	pdf.Cell(28, 5, "Payment Code:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(60, 5, payment.Code)
	pdf.SetTextColor(0, 0, 0)

	// Right side: Date & Method
	dateX := lm + contentW - 75
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(dateX, pdf.GetY())
	pdf.Cell(18, 5, "Date:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(22, 5, payment.Date.Format("02/01/2006"))
	pdf.SetTextColor(0, 0, 0)

	pdf.Ln(7)
	pdf.SetX(lm)
	pdf.SetFont("Arial", "B", 9)
	pdf.Cell(28, 5, "Reference:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	ref := payment.Reference
	if strings.TrimSpace(ref) == "" {
		ref = "-"
	}
	pdf.Cell(60, 5, ref)
	pdf.SetTextColor(0, 0, 0)

	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(dateX, pdf.GetY())
	pdf.Cell(18, 5, "Method:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(40, 5, payment.Method)
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(15)

	// === PARTY INFO ===
	pdf.SetFont("Arial", "B", 10)
	label := "Payment To:"
	if strings.ToUpper(payment.Contact.Type) == "CUSTOMER" {
		label = "Received From:"
	}
	pdf.Cell(contentW, 6, label)
	pdf.Ln(7)
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)

	// Check if this is a PPN tax payment
	isPPNPayment := payment.PaymentType == models.PaymentTypeTaxPPN ||
		payment.PaymentType == models.PaymentTypeTaxPPNInput ||
		payment.PaymentType == models.PaymentTypeTaxPPNOutput ||
		strings.HasPrefix(payment.Code, "SETOR-PPN")

	if isPPNPayment {
		// For PPN payments, display "Negara" (Government)
		pdf.Cell(contentW, 4, "Negara")
		pdf.Ln(5)
		pdf.Cell(contentW, 4, "Direktorat Jenderal Pajak (DJP)")
		pdf.Ln(5)
		pdf.Cell(contentW, 4, "Kementerian Keuangan Republik Indonesia")
		pdf.Ln(5)
	} else if payment.Contact.ID != 0 {
		// Regular payment - show contact info
		pdf.Cell(contentW, 4, payment.Contact.Name)
		pdf.Ln(5)
		if strings.TrimSpace(payment.Contact.Address) != "" {
			pdf.Cell(contentW, 4, payment.Contact.Address)
			pdf.Ln(5)
		}
		if strings.TrimSpace(payment.Contact.Phone) != "" {
			pdf.Cell(contentW, 4, fmt.Sprintf("Phone: %s", payment.Contact.Phone))
			pdf.Ln(5)
		}
	} else {
		pdf.Cell(contentW, 4, "Contact information not available")
		pdf.Ln(5)
	}
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(8)

	// === ALLOCATIONS / PAYMENT LINES ===
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(248, 249, 250)
	pdf.SetDrawColor(160, 160, 160)
	pdf.SetTextColor(51, 51, 51)
	pdf.SetLineWidth(0.4)

	// columns: No | Description | Amount
	numW := contentW * 0.10
	descW := contentW * 0.62
	amtW := contentW - numW - descW
	pdf.CellFormat(numW, 8, "No.", "1", 0, "C", true, 0, "")
	pdf.CellFormat(descW, 8, "Description", "1", 0, "L", true, 0, "")
	pdf.CellFormat(amtW, 8, "Amount", "1", 0, "R", true, 0, "")
	pdf.Ln(8)

	pdf.SetFont("Arial", "", 8)
	pdf.SetTextColor(102, 102, 102)

	// Load allocations (if db available)
	var allocations []models.PaymentAllocation
	if p.db != nil {
		_ = p.db.Where("payment_id = ?", payment.ID).Find(&allocations).Error
	}

	subtotal := 0.0
	if len(allocations) == 0 {
		// Check if this is an expense payment with PPh
		if payment.PPHAmount > 0 && payment.PPHType != "" {
			// === PPh Expense Payment: show 3-line breakdown ===
			grossAmount := payment.Amount // Amount stored IS already the gross expense amount

			// Determine PPh label from type
			pphLabel := "PPh"
			switch payment.PPHType {
			case "PPH_21":
				pphLabel = "PPh Pasal 21"
			case "PPH_23":
				pphLabel = "PPh Pasal 23"
			case "PPH_4_2":
				pphLabel = "PPh Pasal 4(2) Final"
			case "PPH_22":
				pphLabel = "PPh Pasal 22"
			}

			// Line 1: Beban (Bruto)
			pdf.CellFormat(numW, 6, "1", "1", 0, "C", false, 0, "")
			pdf.CellFormat(descW, 6, "Beban / Expense (Bruto)", "1", 0, "L", false, 0, "")
			pdf.CellFormat(amtW, 6, p.formatRupiah(grossAmount), "1", 0, "R", false, 0, "")
			pdf.Ln(6)

			// Line 2: PPh Dipotong (deduction - shown as negative)
			pdf.SetTextColor(220, 53, 69)
			pdf.CellFormat(numW, 6, "2", "1", 0, "C", false, 0, "")
			pdf.CellFormat(descW, 6, fmt.Sprintf("%s Dipotong (%.1f%%)", pphLabel, payment.PPHRate), "1", 0, "L", false, 0, "")
			pdf.CellFormat(amtW, 6, fmt.Sprintf("- %s", p.formatRupiah(payment.PPHAmount)), "1", 0, "R", false, 0, "")
			pdf.Ln(6)
			pdf.SetTextColor(102, 102, 102) // Reset color

			// Line 3: Total Dibayarkan (Neto)
			netAmount := grossAmount - payment.PPHAmount
			pdf.SetFont("Arial", "B", 8)
			pdf.SetTextColor(40, 167, 69)
			pdf.CellFormat(numW, 6, "3", "1", 0, "C", false, 0, "")
			pdf.CellFormat(descW, 6, "Jumlah Dibayarkan (Neto)", "1", 0, "L", false, 0, "")
			pdf.CellFormat(amtW, 6, p.formatRupiah(netAmount), "1", 0, "R", false, 0, "")
			pdf.Ln(6)
			pdf.SetFont("Arial", "", 8)
			pdf.SetTextColor(102, 102, 102) // Reset

			subtotal = grossAmount
		} else {
			// Single line fallback (no PPh)
			pdf.CellFormat(numW, 6, "1", "1", 0, "C", false, 0, "")
			desc := "Payment transaction"

			// Check if this is a PPN payment
			if payment.PaymentType == models.PaymentTypeTaxPPN ||
				payment.PaymentType == models.PaymentTypeTaxPPNInput ||
				payment.PaymentType == models.PaymentTypeTaxPPNOutput ||
				strings.HasPrefix(payment.Code, "SETOR-PPN") {
				desc = "Setor PPN ke Negara (Tax Remittance to Government)"
			} else if strings.ToUpper(payment.Contact.Type) == "CUSTOMER" {
				desc = "Payment received"
			} else {
				desc = "Payment made"
			}

			pdf.CellFormat(descW, 6, desc, "1", 0, "L", false, 0, "")
			pdf.CellFormat(amtW, 6, p.formatRupiah(payment.Amount), "1", 0, "R", false, 0, "")
			pdf.Ln(6)
			subtotal = payment.Amount
		}
	} else {
		for i, alloc := range allocations {
			rowNo := strconv.Itoa(i + 1)
			desc := "Allocation"
			// Try to enrich description with invoice/bill info
			if alloc.InvoiceID != nil {
				// Try to load sale code
				var sale models.Sale
				if err := p.db.Select("id, code").First(&sale, *alloc.InvoiceID).Error; err == nil && strings.TrimSpace(sale.Code) != "" {
					desc = fmt.Sprintf("Payment for Invoice %s", sale.Code)
				} else {
					desc = fmt.Sprintf("Payment for Invoice #%d", *alloc.InvoiceID)
				}
			} else if alloc.BillID != nil {
				desc = fmt.Sprintf("Payment for Bill #%d", *alloc.BillID)
			}
			amountTxt := p.formatRupiah(alloc.AllocatedAmount)
			pdf.CellFormat(numW, 6, rowNo, "1", 0, "C", false, 0, "")
			pdf.CellFormat(descW, 6, desc, "1", 0, "L", false, 0, "")
			pdf.CellFormat(amtW, 6, amountTxt, "1", 0, "R", false, 0, "")
			pdf.Ln(6)
			subtotal += alloc.AllocatedAmount
		}
	}

	// === SUMMARY ===
	pdf.Ln(5)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetTextColor(0, 0, 0)

	if payment.PPHAmount > 0 && payment.PPHType != "" {
		// PPh summary: show Subtotal → PPh → Total Dibayarkan
		grossAmount := payment.Amount // payment.Amount IS the gross amount
		netAmount := grossAmount - payment.PPHAmount

		pdf.Cell(contentW-50, 6, "")
		pdf.Cell(25, 6, "Subtotal:")
		pdf.Cell(25, 6, p.formatRupiah(grossAmount))
		pdf.Ln(6)

		// PPh line
		pphLabel := fmt.Sprintf("PPh (%s):", payment.PPHType)
		pdf.SetFont("Arial", "", 10)
		pdf.SetTextColor(220, 53, 69)
		pdf.Cell(contentW-50, 6, "")
		pdf.Cell(25, 6, pphLabel)
		pdf.Cell(25, 6, fmt.Sprintf("- %s", p.formatRupiah(payment.PPHAmount)))
		pdf.Ln(6)

		// Total Dibayarkan
		pdf.SetFont("Arial", "B", 12)
		pdf.SetTextColor(0, 0, 0)
		pdf.Cell(contentW-50, 8, "")
		pdf.Cell(25, 8, "TOTAL:")
		pdf.Cell(25, 8, p.formatRupiah(netAmount))
		pdf.Ln(5)
		pdf.SetFont("Arial", "I", 8)
		pdf.SetTextColor(102, 102, 102)
		pdf.Cell(contentW, 4, "(Jumlah yang dibayarkan setelah pemotongan pajak)")
		pdf.SetTextColor(0, 0, 0)
	} else {
		// Standard summary (no PPh)
		pdf.Cell(contentW-50, 6, "")
		pdf.Cell(25, 6, "Subtotal:")
		pdf.Cell(25, 6, p.formatRupiah(subtotal))
		pdf.Ln(6)
		pdf.SetFont("Arial", "B", 12)
		pdf.Cell(contentW-50, 8, "")
		pdf.Cell(25, 8, "TOTAL:")
		pdf.Cell(25, 8, p.formatRupiah(payment.Amount))
	}
	pdf.Ln(12)

	// Extra info
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(contentW, 5, fmt.Sprintf("Status: %s", payment.Status))
	pdf.Ln(5)
	if strings.TrimSpace(payment.Notes) != "" {
		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(contentW, 6, "Notes:")
		pdf.Ln(6)
		pdf.SetFont("Arial", "", 9)
		// Format notes to replace decimal comma with dot for better readability
		formattedNotes := p.formatNotesAmount(payment.Notes)
		pdf.MultiCell(contentW, 4, formattedNotes, "", "", false)
	}

	// Footer timestamp
	pdf.Ln(10)
	pdf.SetFont("Arial", "I", 8)
	pdf.Cell(contentW, 4, fmt.Sprintf("Generated on %s", time.Now().Format("02/01/2006 15:04")))

	// Output to buffer
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to generate payment receipt PDF: %v", err)
	}
	return buf.Bytes(), nil
}

// GeneratePurchaseOrderPDF generates a PDF for a purchase order
func (p *PDFService) GeneratePurchaseOrderPDF(purchase *models.Purchase) ([]byte, error) {
	utils.SanitizeExportData(purchase)
	if purchase == nil {
		return nil, fmt.Errorf("purchase data is required")
	}

	// For now, return a simple implementation
	return []byte("Purchase Order PDF not implemented yet"), nil
}

// GeneratePurchaseReceiptPDF generates PDF for a purchase receipt
func (p *PDFService) GeneratePurchaseReceiptPDF(receipt *models.PurchaseReceipt) ([]byte, error) {
	utils.SanitizeExportData(receipt)
	if receipt == nil {
		return nil, fmt.Errorf("receipt data is required")
	}

	// For now, return a simple implementation
	return []byte("Purchase Receipt PDF not implemented yet"), nil
}

// GenerateReceiptPDF generates PDF for a single purchase receipt
func (p *PDFService) GenerateReceiptPDF(data interface{}) ([]byte, error) {
	return p.GenerateReceiptPDFWithUser(data, 0) // Default: no specific user
}

// GenerateReceiptPDFWithUser generates PDF for a receipt with specific user context
func (p *PDFService) GenerateReceiptPDFWithUser(data interface{}, userID uint) ([]byte, error) {
	utils.SanitizeExportData(data)
	// If it's a sales receipt, delegate to sales receipt renderer with user context
	if s, ok := data.(*models.Sale); ok {
		return p.generateSaleReceiptPDFWithUser(s, userID)
	}
	if s2, ok := data.(models.Sale); ok {
		ss := s2
		return p.generateSaleReceiptPDFWithUser(&ss, userID)
	}

	// Otherwise, treat as purchase receipt (existing behavior)
	var receipt *models.PurchaseReceipt
	switch v := data.(type) {
	case *models.PurchaseReceipt:
		receipt = v
	case models.PurchaseReceipt:
		receipt = &v
	default:
		return nil, fmt.Errorf("invalid data format for GenerateReceiptPDF")
	}

	if receipt == nil {
		return nil, fmt.Errorf("receipt data is required")
	}

	// Create new PDF document with clean margins similar to Invoice
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// Layout helpers
	lm, tm, rm, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	contentW := pageW - lm - rm

	// Get company info
	companyInfo, err := p.getCompanyInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get company info: %v", err)
	}

	// === HEADER (mirror Invoice layout) ===
	// Logo on the left (with placeholder if missing)
	logoX := lm
	logoY := tm
	logoSize := 35.0

	logoAdded := false
	if strings.TrimSpace(companyInfo.CompanyLogo) != "" {
		logoPath := companyInfo.CompanyLogo
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

	// Company profile on the right, right-aligned
	companyInfoX := pageW - rm
	companyInfoY := tm
	pdf.SetFont("Arial", "B", 12)
	nameW := pdf.GetStringWidth(companyInfo.CompanyName)
	pdf.SetXY(companyInfoX-nameW, companyInfoY)
	pdf.Cell(nameW, 6, companyInfo.CompanyName)

	pdf.SetFont("Arial", "", 9)
	addrW := pdf.GetStringWidth(companyInfo.CompanyAddress)
	pdf.SetXY(companyInfoX-addrW, companyInfoY+8)
	pdf.Cell(addrW, 4, companyInfo.CompanyAddress)

	phoneText := fmt.Sprintf("Phone: %s", companyInfo.CompanyPhone)
	phoneW := pdf.GetStringWidth(phoneText)
	pdf.SetXY(companyInfoX-phoneW, companyInfoY+14)
	pdf.Cell(phoneW, 4, phoneText)

	emailText := fmt.Sprintf("Email: %s", companyInfo.CompanyEmail)
	emailW := pdf.GetStringWidth(emailText)
	pdf.SetXY(companyInfoX-emailW, companyInfoY+20)
	pdf.Cell(emailW, 4, emailText)

	// Divider line under header
	pdf.SetDrawColor(238, 238, 238)
	pdf.SetLineWidth(0.2)
	pdf.Line(lm, tm+45, pageW-rm, tm+45)

	// === TITLE ===
	pdf.SetY(tm + 55)
	pdf.SetX(lm)
	pdf.SetFont("Arial", "B", 22)
	pdf.SetTextColor(51, 51, 51)
	pdf.Cell(contentW, 10, "RECEIPT")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(20)

	// === RECEIPT DETAILS (two columns like invoice) ===
	pdf.SetFont("Arial", "B", 9)
	pdf.SetX(lm)
	pdf.Cell(30, 5, "Receipt Number:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(60, 5, receipt.ReceiptNumber)

	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(0, 0, 0)
	detailRightX := lm + contentW - 60
	pdf.SetX(detailRightX)
	pdf.Cell(20, 5, "Date:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(40, 5, receipt.ReceivedDate.Format("02/01/2006"))
	pdf.Ln(8)

	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetX(lm)
	pdf.Cell(30, 5, "Purchase Code:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(60, 5, receipt.Purchase.Code)

	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetX(detailRightX)
	pdf.Cell(20, 5, "Status:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(40, 5, receipt.Status)
	pdf.Ln(8)

	// Received by (single line)
	receiverName := ""
	if receipt.Receiver.FirstName != "" || receipt.Receiver.LastName != "" {
		receiverName = strings.TrimSpace(receipt.Receiver.FirstName + " " + receipt.Receiver.LastName)
	} else if receipt.Receiver.Username != "" {
		receiverName = receipt.Receiver.Username
	}
	if strings.TrimSpace(receiverName) != "" {
		pdf.SetFont("Arial", "B", 9)
		pdf.SetTextColor(0, 0, 0)
		pdf.SetX(lm)
		pdf.Cell(30, 5, "Received By:")
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(102, 102, 102)
		pdf.Cell(contentW-30, 5, receiverName)
		pdf.Ln(10)
	} else {
		pdf.Ln(6)
	}

	pdf.SetTextColor(0, 0, 0)

	// === VENDOR SECTION (mirrors "Bill To") ===
	pdf.SetFont("Arial", "B", 10)
	pdf.SetTextColor(51, 51, 51)
	pdf.Cell(contentW, 6, "Vendor:")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	if receipt.Purchase.Vendor.ID != 0 {
		pdf.Cell(contentW, 4, receipt.Purchase.Vendor.Name)
		pdf.Ln(5)
		if strings.TrimSpace(receipt.Purchase.Vendor.Address) != "" {
			pdf.Cell(contentW, 4, receipt.Purchase.Vendor.Address)
			pdf.Ln(5)
		}
		if strings.TrimSpace(receipt.Purchase.Vendor.Phone) != "" {
			pdf.Cell(contentW, 4, fmt.Sprintf("Phone: %s", receipt.Purchase.Vendor.Phone))
			pdf.Ln(5)
		}
	} else {
		pdf.Cell(contentW, 4, "Vendor information not available")
		pdf.Ln(5)
	}
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(8)

	// === ITEMS TABLE (styled like invoice) ===
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(248, 249, 250)
	pdf.SetDrawColor(160, 160, 160)
	pdf.SetTextColor(51, 51, 51)
	pdf.SetLineWidth(0.4)

	// Column widths
	numW := contentW * 0.08
	prodW := contentW * 0.36
	ordW := contentW * 0.16
	recvW := contentW * 0.16
	condW := contentW * 0.10
	notesW := contentW - numW - prodW - ordW - recvW - condW
	if notesW < 20 {
		notesW = 20
	} // safety

	pdf.CellFormat(numW, 8, "No.", "1", 0, "C", true, 0, "")
	pdf.CellFormat(prodW, 8, "Product", "1", 0, "L", true, 0, "")
	pdf.CellFormat(ordW, 8, "Ordered", "1", 0, "C", true, 0, "")
	pdf.CellFormat(recvW, 8, "Received", "1", 0, "C", true, 0, "")
	pdf.CellFormat(condW, 8, "Condition", "1", 0, "C", true, 0, "")
	pdf.CellFormat(notesW, 8, "Notes", "1", 0, "L", true, 0, "")
	pdf.Ln(8)

	pdf.SetFont("Arial", "", 8)
	pdf.SetTextColor(102, 102, 102)

	for i, item := range receipt.ReceiptItems {
		// Page break handling
		if pdf.GetY() > 250 {
			pdf.AddPage()
			pdf.SetFont("Arial", "B", 9)
			pdf.SetFillColor(248, 249, 250)
			pdf.SetDrawColor(160, 160, 160)
			pdf.SetLineWidth(0.4)
			pdf.SetTextColor(51, 51, 51)
			pdf.CellFormat(numW, 8, "No.", "1", 0, "C", true, 0, "")
			pdf.CellFormat(prodW, 8, "Product", "1", 0, "L", true, 0, "")
			pdf.CellFormat(ordW, 8, "Ordered", "1", 0, "C", true, 0, "")
			pdf.CellFormat(recvW, 8, "Received", "1", 0, "C", true, 0, "")
			pdf.CellFormat(condW, 8, "Condition", "1", 0, "C", true, 0, "")
			pdf.CellFormat(notesW, 8, "Notes", "1", 0, "L", true, 0, "")
			pdf.Ln(8)
			pdf.SetFont("Arial", "", 8)
			pdf.SetTextColor(102, 102, 102)
		}

		// Row values
		num := strconv.Itoa(i + 1)
		prod := "Product"
		ordered := "0"
		if item.PurchaseItem.Product != nil && item.PurchaseItem.Product.ID != 0 {
			prod = item.PurchaseItem.Product.Name
		} else if strings.TrimSpace(item.PurchaseItem.Description) != "" {
			prod = strings.TrimSpace(item.PurchaseItem.Description)
		} else if strings.TrimSpace(item.PurchaseItem.ItemType) != "" {
			prod = strings.TrimSpace(item.PurchaseItem.ItemType)
		}
		if len(prod) > 45 {
			prod = prod[:42] + "..."
		}
		if item.PurchaseItem.ID != 0 {
			ordered = strconv.Itoa(item.PurchaseItem.Quantity)
		}
		received := strconv.Itoa(item.QuantityReceived)
		cond := item.Condition
		note := strings.TrimSpace(item.Notes)
		if len(note) > 40 {
			note = note[:37] + "..."
		}

		pdf.CellFormat(numW, 6, num, "1", 0, "C", true, 0, "")
		pdf.CellFormat(prodW, 6, prod, "1", 0, "L", true, 0, "")
		pdf.CellFormat(ordW, 6, ordered, "1", 0, "C", true, 0, "")
		pdf.CellFormat(recvW, 6, received, "1", 0, "C", true, 0, "")
		pdf.CellFormat(condW, 6, cond, "1", 0, "C", true, 0, "")
		pdf.CellFormat(notesW, 6, note, "1", 0, "L", true, 0, "")
		pdf.Ln(6)
	}

	// === NOTES ===
	if strings.TrimSpace(receipt.Notes) != "" {
		pdf.Ln(10)
		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(contentW, 6, "Notes:")
		pdf.Ln(6)
		pdf.SetFont("Arial", "", 9)
		pdf.MultiCell(contentW, 4, receipt.Notes, "", "", false)
	}

	// === FOOTER ===
	pdf.Ln(15)
	pdf.SetDrawColor(238, 238, 238)
	pdf.SetLineWidth(0.2)
	pdf.Line(lm, pdf.GetY(), pageW-rm, pdf.GetY())
	pdf.Ln(6)
	pdf.SetFont("Arial", "", 8)
	pdf.SetTextColor(153, 153, 153)
	footer := fmt.Sprintf("Generated on %s", time.Now().Format("02/01/2006 15:04"))
	w := pdf.GetStringWidth(footer)
	pdf.SetX((pageW - w) / 2)
	pdf.Cell(w, 4, footer)

	// Output to buffer
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to generate receipt PDF: %v", err)
	}
	return buf.Bytes(), nil
}

// GenerateAllReceiptsPDF generates combined PDF for all receipts of a purchase
func (p *PDFService) GenerateAllReceiptsPDF(data interface{}) ([]byte, error) {
	utils.SanitizeExportData(data)
	// Extract purchase and receipts from interface{}
	var purchase *models.Purchase
	var receiptsList []models.PurchaseReceipt

	// Handle different data formats
	switch v := data.(type) {
	case map[string]interface{}:
		// Handle JSON-like data structure
		if p, ok := v["purchase"]; ok {
			if purchaseData, ok := p.(*models.Purchase); ok {
				purchase = purchaseData
			}
		}
		if r, ok := v["receipts"]; ok {
			if receiptData, ok := r.([]models.PurchaseReceipt); ok {
				receiptsList = receiptData
			}
		}
	default:
		// Try to convert using JSON marshaling as fallback
		if jsonData, err := json.Marshal(data); err == nil {
			var dataMap map[string]interface{}
			if err := json.Unmarshal(jsonData, &dataMap); err == nil {
				if p, ok := dataMap["purchase"]; ok {
					if purchaseData, ok := p.(*models.Purchase); ok {
						purchase = purchaseData
					}
				}
				if r, ok := dataMap["receipts"]; ok {
					if receiptData, ok := r.([]models.PurchaseReceipt); ok {
						receiptsList = receiptData
					}
				}
			}
		}
		if purchase == nil {
			return nil, fmt.Errorf("invalid data format for GenerateAllReceiptsPDF")
		}
	}

	if purchase == nil {
		return nil, fmt.Errorf("purchase data is required")
	}
	// Create new PDF document (mirror invoice layout)
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// Layout helpers
	lm, tm, rm, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	contentW := pageW - lm - rm

	// Get company info from settings
	companyInfo, err := p.getCompanyInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get company info: %v", err)
	}

	// Header: logo left, company info right
	logoX := lm
	logoY := tm
	logoSize := 35.0
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

	// Company info aligned to right
	companyInfoX := pageW - rm
	companyInfoY := tm
	pdf.SetFont("Arial", "B", 12)
	nameW := pdf.GetStringWidth(companyInfo.CompanyName)
	pdf.SetXY(companyInfoX-nameW, companyInfoY)
	pdf.Cell(nameW, 6, companyInfo.CompanyName)

	pdf.SetFont("Arial", "", 9)
	addrW := pdf.GetStringWidth(companyInfo.CompanyAddress)
	pdf.SetXY(companyInfoX-addrW, companyInfoY+8)
	pdf.Cell(addrW, 4, companyInfo.CompanyAddress)

	phoneText := fmt.Sprintf("Phone: %s", companyInfo.CompanyPhone)
	phoneW := pdf.GetStringWidth(phoneText)
	pdf.SetXY(companyInfoX-phoneW, companyInfoY+14)
	pdf.Cell(phoneW, 4, phoneText)

	// Divider line under header
	pdf.SetDrawColor(238, 238, 238)
	pdf.SetLineWidth(0.2)
	pdf.Line(lm, tm+45, pageW-rm, tm+45)

	// Title
	pdf.SetY(tm + 55)
	pdf.SetX(lm)
	pdf.SetFont("Arial", "B", 22)
	pdf.SetTextColor(51, 51, 51)
	pdf.Cell(contentW, 10, "PURCHASE RECEIPTS SUMMARY")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(18)

	// Purchase details (two-column, clean)
	pdf.SetFont("Arial", "B", 9)
	pdf.SetX(lm)
	pdf.Cell(30, 5, "Purchase Order:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(60, 5, purchase.Code)

	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(0, 0, 0)
	detailRightX := lm + contentW - 70
	pdf.SetX(detailRightX)
	pdf.Cell(28, 5, "Date:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(42, 5, purchase.Date.Format("02/01/2006"))
	pdf.Ln(8)

	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetX(lm)
	pdf.Cell(30, 5, "Vendor:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(60, 5, purchase.Vendor.Name)

	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetX(detailRightX)
	pdf.Cell(28, 5, "Total Receipts:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(42, 5, fmt.Sprintf("%d", len(receiptsList)))
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(12)

	// If only one receipt, render a compact one-page receipt and return
	if len(receiptsList) == 1 {
		r := receiptsList[0]
		// Title for single receipt
		pdf.SetFont("Arial", "B", 12)
		pdf.Cell(190, 8, fmt.Sprintf("Receipt: %s", r.ReceiptNumber))
		pdf.Ln(8)

		// Basic receipt meta (date, status, received by)
		pdf.SetFont("Arial", "", 10)
		pdf.Cell(95, 6, fmt.Sprintf("Date: %s", r.ReceivedDate.Format("02/01/2006")))
		pdf.Cell(95, 6, fmt.Sprintf("Status: %s", r.Status))
		pdf.Ln(6)
		receiver := ""
		if r.Receiver.FirstName != "" || r.Receiver.LastName != "" {
			receiver = strings.TrimSpace(r.Receiver.FirstName + " " + r.Receiver.LastName)
		} else if r.Receiver.Username != "" {
			receiver = r.Receiver.Username
		}
		if receiver != "" {
			pdf.Cell(190, 6, fmt.Sprintf("Received By: %s", receiver))
			pdf.Ln(6)
		}
		pdf.Ln(2)

		// Items table (same layout as per-receipt page)
		pdf.SetFont("Arial", "B", 9)
		pdf.SetFillColor(220, 220, 220)
		pdf.CellFormat(15, 7, "#", "1", 0, "C", true, 0, "")
		pdf.CellFormat(60, 7, "Product", "1", 0, "L", true, 0, "")
		pdf.CellFormat(20, 7, "Ordered", "1", 0, "C", true, 0, "")
		pdf.CellFormat(20, 7, "Received", "1", 0, "C", true, 0, "")
		pdf.CellFormat(25, 7, "Condition", "1", 0, "C", true, 0, "")
		pdf.CellFormat(50, 7, "Notes", "1", 0, "L", true, 0, "")
		pdf.Ln(7)

		pdf.SetFont("Arial", "", 8)
		pdf.SetFillColor(255, 255, 255)
		for j, item := range r.ReceiptItems {
			num := strconv.Itoa(j + 1)
			prod := "Product"
			ordered := "0"
			if item.PurchaseItem.Product.ID != 0 {
				prod = item.PurchaseItem.Product.Name
				if len(prod) > 35 {
					prod = prod[:32] + "..."
				}
				ordered = strconv.Itoa(item.PurchaseItem.Quantity)
			}
			received := strconv.Itoa(item.QuantityReceived)
			cond := item.Condition
			notes := item.Notes
			if len(notes) > 30 {
				notes = notes[:27] + "..."
			}
			pdf.CellFormat(15, 5, num, "1", 0, "C", false, 0, "")
			pdf.CellFormat(60, 5, prod, "1", 0, "L", false, 0, "")
			pdf.CellFormat(20, 5, ordered, "1", 0, "C", false, 0, "")
			pdf.CellFormat(20, 5, received, "1", 0, "C", false, 0, "")
			pdf.CellFormat(25, 5, cond, "1", 0, "C", false, 0, "")
			pdf.CellFormat(50, 5, notes, "1", 0, "L", false, 0, "")
			pdf.Ln(5)
		}

		// Optional single-receipt notes
		if strings.TrimSpace(r.Notes) != "" {
			pdf.Ln(5)
			pdf.SetFont("Arial", "B", 9)
			pdf.Cell(190, 5, "Notes:")
			pdf.Ln(5)
			pdf.SetFont("Arial", "", 8)
			pdf.MultiCell(190, 4, r.Notes, "", "", false)
		}

		// Output single-page PDF
		var buf bytes.Buffer
		if err := pdf.Output(&buf); err != nil {
			return nil, fmt.Errorf("failed to generate receipts PDF: %v", err)
		}
		return buf.Bytes(), nil
	}

	// Receipts summary table
	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(220, 220, 220)
	pdf.Cell(190, 8, "RECEIPTS SUMMARY")
	pdf.Ln(8)

	pdf.CellFormat(15, 8, "#", "1", 0, "C", true, 0, "")
	pdf.CellFormat(45, 8, "Receipt Number", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 8, "Date", "1", 0, "C", true, 0, "")
	pdf.CellFormat(40, 8, "Received By", "1", 0, "L", true, 0, "")
	pdf.CellFormat(25, 8, "Status", "1", 0, "C", true, 0, "")
	pdf.CellFormat(40, 8, "Items Count", "1", 0, "C", true, 0, "")
	pdf.Ln(8)

	// Receipts data
	pdf.SetFont("Arial", "", 9)
	pdf.SetFillColor(255, 255, 255)

	for i, receipt := range receiptsList {
		itemNumber := strconv.Itoa(i + 1)
		receiptNumber := receipt.ReceiptNumber
		date := receipt.ReceivedDate.Format("02/01/06")
		receivedBy := "N/A"
		if receipt.Receiver.FirstName != "" || receipt.Receiver.LastName != "" {
			receivedBy = strings.TrimSpace(receipt.Receiver.FirstName + " " + receipt.Receiver.LastName)
		} else if receipt.Receiver.Username != "" {
			receivedBy = receipt.Receiver.Username
		}
		if len(receivedBy) > 25 {
			receivedBy = receivedBy[:22] + "..."
		}
		status := receipt.Status
		// Items Count should reflect total units received, not number of lines
		units := 0
		for _, it := range receipt.ReceiptItems {
			units += it.QuantityReceived
		}
		itemsCount := strconv.Itoa(units)

		pdf.CellFormat(15, 6, itemNumber, "1", 0, "C", false, 0, "")
		pdf.CellFormat(45, 6, receiptNumber, "1", 0, "L", false, 0, "")
		pdf.CellFormat(25, 6, date, "1", 0, "C", false, 0, "")
		pdf.CellFormat(40, 6, receivedBy, "1", 0, "L", false, 0, "")
		pdf.CellFormat(25, 6, status, "1", 0, "C", false, 0, "")
		pdf.CellFormat(40, 6, itemsCount, "1", 0, "C", false, 0, "")
		pdf.Ln(6)
	}

	// Add each receipt as separate page
	for _, receipt := range receiptsList {
		pdf.AddPage()

		// Generate individual receipt content (simplified version)
		pdf.SetFont("Arial", "B", 14)
		pdf.Cell(190, 10, fmt.Sprintf("Receipt: %s", receipt.ReceiptNumber))
		pdf.Ln(10)

		pdf.SetFont("Arial", "", 10)
		pdf.Cell(95, 6, fmt.Sprintf("Date: %s", receipt.ReceivedDate.Format("02/01/2006")))
		pdf.Cell(95, 6, fmt.Sprintf("Status: %s", receipt.Status))
		pdf.Ln(6)
		receiverName := ""
		if receipt.Receiver.FirstName != "" || receipt.Receiver.LastName != "" {
			receiverName = strings.TrimSpace(receipt.Receiver.FirstName + " " + receipt.Receiver.LastName)
		} else if receipt.Receiver.Username != "" {
			receiverName = receipt.Receiver.Username
		}

		if receiverName != "" {
			pdf.Cell(190, 6, fmt.Sprintf("Received By: %s", receiverName))
			pdf.Ln(6)
		}
		pdf.Ln(5)

		// Items table for this receipt
		pdf.SetFont("Arial", "B", 9)
		pdf.SetFillColor(220, 220, 220)
		pdf.CellFormat(15, 7, "#", "1", 0, "C", true, 0, "")
		pdf.CellFormat(60, 7, "Product", "1", 0, "L", true, 0, "")
		pdf.CellFormat(20, 7, "Ordered", "1", 0, "C", true, 0, "")
		pdf.CellFormat(20, 7, "Received", "1", 0, "C", true, 0, "")
		pdf.CellFormat(25, 7, "Condition", "1", 0, "C", true, 0, "")
		pdf.CellFormat(50, 7, "Notes", "1", 0, "L", true, 0, "")
		pdf.Ln(7)

		pdf.SetFont("Arial", "", 8)
		pdf.SetFillColor(255, 255, 255)

		for j, item := range receipt.ReceiptItems {
			itemNumber := strconv.Itoa(j + 1)
			productName := "Product"
			orderedQty := "0"
			if item.PurchaseItem.Product.ID != 0 {
				productName = item.PurchaseItem.Product.Name
				if len(productName) > 35 {
					productName = productName[:32] + "..."
				}
				orderedQty = strconv.Itoa(item.PurchaseItem.Quantity)
			}

			receivedQty := strconv.Itoa(item.QuantityReceived)
			condition := item.Condition
			notes := item.Notes
			if len(notes) > 30 {
				notes = notes[:27] + "..."
			}

			pdf.CellFormat(15, 5, itemNumber, "1", 0, "C", false, 0, "")
			pdf.CellFormat(60, 5, productName, "1", 0, "L", false, 0, "")
			pdf.CellFormat(20, 5, orderedQty, "1", 0, "C", false, 0, "")
			pdf.CellFormat(20, 5, receivedQty, "1", 0, "C", false, 0, "")
			pdf.CellFormat(25, 5, condition, "1", 0, "C", false, 0, "")
			pdf.CellFormat(50, 5, notes, "1", 0, "L", false, 0, "")
			pdf.Ln(5)
		}

		// Notes for this receipt
		if receipt.Notes != "" {
			pdf.Ln(5)
			pdf.SetFont("Arial", "B", 9)
			pdf.Cell(190, 5, "Notes:")
			pdf.Ln(5)
			pdf.SetFont("Arial", "", 8)
			pdf.MultiCell(190, 4, receipt.Notes, "", "", false)
		}
	}

	// Final summary page
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(190, 10, "COMPLETION SUMMARY")
	pdf.Ln(15)

	// Calculate completion statistics
	totalItems := len(purchase.PurchaseItems)
	totalReceiptItems := 0
	totalReceived := 0
	totalOrdered := 0

	for _, item := range purchase.PurchaseItems {
		totalOrdered += item.Quantity
	}

	for _, receipt := range receiptsList {
		totalReceiptItems += len(receipt.ReceiptItems)
		for _, item := range receipt.ReceiptItems {
			totalReceived += item.QuantityReceived
		}
	}

	completionRate := float64(totalReceived) / float64(totalOrdered) * 100

	pdf.SetFont("Arial", "", 10)
	pdf.Cell(95, 6, fmt.Sprintf("Purchase Items: %d", totalItems))
	pdf.Cell(95, 6, fmt.Sprintf("Total Ordered: %d", totalOrdered))
	pdf.Ln(6)
	pdf.Cell(95, 6, fmt.Sprintf("Total Receipts: %d", len(receiptsList)))
	pdf.Cell(95, 6, fmt.Sprintf("Total Received: %d", totalReceived))
	pdf.Ln(6)
	pdf.Cell(190, 6, fmt.Sprintf("Completion Rate: %.1f%%", completionRate))
	pdf.Ln(10)

	// Footer
	pdf.SetFont("Arial", "I", 8)
	pdf.Cell(190, 4, fmt.Sprintf("Generated on %s", time.Now().Format("02/01/2006 15:04")))

	// Output to buffer
	var buf bytes.Buffer
	err = pdf.Output(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to generate combined receipts PDF: %v", err)
	}

	return buf.Bytes(), nil
}

// ========================================
// Sales Payment Receipt PDF (for fully paid sales)
// ========================================

// generateSaleReceiptPDF renders a payment receipt in Indonesian KWITANSI style for a fully-paid sale.
func (p *PDFService) generateSaleReceiptPDF(sale *models.Sale) ([]byte, error) {
	return p.generateSaleReceiptPDFWithUser(sale, 0) // Default: no specific user
}

// generateSaleReceiptPDFWithUser renders a payment receipt with specific user context for signature
func (p *PDFService) generateSaleReceiptPDFWithUser(sale *models.Sale, userID uint) ([]byte, error) {
	if sale == nil {
		return nil, fmt.Errorf("sale is required")
	}

	// Create new PDF document
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// Layout helpers
	lm, tm, rm, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	contentW := pageW - lm - rm

	// Company info
	companyInfo, err := p.getCompanyInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get company info: %v", err)
	}

	// === ADD HEADER WITH LOGO AND COMPANY PROFILE ===
	// Logo on the left (with placeholder if missing)
	logoX := lm
	logoY := tm
	logoSize := 35.0

	logoAdded := false
	if strings.TrimSpace(companyInfo.CompanyLogo) != "" {
		logoPath := companyInfo.CompanyLogo
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

	// Company profile on the right, right-aligned
	companyInfoX := pageW - rm
	companyInfoY := tm
	pdf.SetFont("Arial", "B", 12)
	nameW := pdf.GetStringWidth(companyInfo.CompanyName)
	pdf.SetXY(companyInfoX-nameW, companyInfoY)
	pdf.Cell(nameW, 6, companyInfo.CompanyName)

	pdf.SetFont("Arial", "", 9)
	addrW := pdf.GetStringWidth(companyInfo.CompanyAddress)
	pdf.SetXY(companyInfoX-addrW, companyInfoY+8)
	pdf.Cell(addrW, 4, companyInfo.CompanyAddress)

	phoneText := fmt.Sprintf("Phone: %s", companyInfo.CompanyPhone)
	phoneW := pdf.GetStringWidth(phoneText)
	pdf.SetXY(companyInfoX-phoneW, companyInfoY+14)
	pdf.Cell(phoneW, 4, phoneText)

	emailText := fmt.Sprintf("Email: %s", companyInfo.CompanyEmail)
	emailW := pdf.GetStringWidth(emailText)
	pdf.SetXY(companyInfoX-emailW, companyInfoY+20)
	pdf.Cell(emailW, 4, emailText)

	// Divider line below email - full width from edge to edge
	pdf.SetDrawColor(238, 238, 238)
	pdf.SetLineWidth(0.2)
	pdf.Line(0, companyInfoY+26, pageW, companyInfoY+26)

	// Move content down to make space for header - positioned closer to the line
	pdf.SetY(companyInfoY + 32) // Position closer to the new line

	// Get localization helper
	language := utils.GetUserLanguageFromSettings(p.db)
	loc := func(key, fallback string) string {
		t := utils.T(key, language)
		if t == key {
			return fallback
		}
		return t
	}

	// Title RECEIPT/KWITANSI (underlined) - centered with proper spacing
	pdf.SetFont("Times", "B", 24)
	pdf.SetTextColor(0, 0, 0)
	receiptTitle := loc("receipt", "RECEIPT")
	pdf.CellFormat(contentW, 14, receiptTitle, "", 0, "C", false, 0, "")
	pdf.Ln(15) // Reduced from 18 to 15 points for tighter spacing below title

	// Field layout sizes
	labelW := 45.0
	colonW := 4.0
	valueW := contentW - labelW - colonW

	// Received From / Sudah Terima Dari
	pdf.SetFont("Times", "", 12)
	receivedFromLabel := loc("received_from", "Received From")
	pdf.CellFormat(labelW, 7, receivedFromLabel, "", 0, "L", false, 0, "")
	pdf.CellFormat(colonW, 7, ":", "", 0, "L", false, 0, "")
	pdf.SetFont("Times", "B", 12)
	receiver := "-"
	if sale.Customer.ID != 0 && strings.TrimSpace(sale.Customer.Name) != "" {
		receiver = sale.Customer.Name
	}
	pdf.CellFormat(valueW, 7, receiver, "", 1, "L", false, 0, "")

	// Amount in Words / Banyaknya Uang
	amount := sale.TotalAmount
	words := p.amountToRupiahWords(amount)
	pdf.SetFont("Times", "", 12)
	amountInWordsLabel := loc("amount_in_words", "Amount in Words")
	pdf.CellFormat(labelW, 7, amountInWordsLabel, "", 0, "L", false, 0, "")
	pdf.CellFormat(colonW, 7, ":", "", 0, "L", false, 0, "")

	// Handle long text with auto-resize and multi-line support
	wordsLine := words + "----"
	pdf.SetFont("Times", "I", 12)

	// Calculate text width to check if it fits
	textWidth := pdf.GetStringWidth(wordsLine)
	maxWidth := valueW - 2 // Leave small margin

	// If text is too long, try smaller font first
	if textWidth > maxWidth {
		pdf.SetFont("Times", "I", 10)
		textWidth = pdf.GetStringWidth(wordsLine)
	}

	// If still too long, use MultiCell to wrap to 2 lines
	if textWidth > maxWidth {
		// Use MultiCell for automatic line breaking
		pdf.MultiCell(valueW, 5, wordsLine, "", "L", false)
	} else {
		// Single line fits, use regular cell
		pdf.CellFormat(valueW, 7, wordsLine, "", 1, "L", false, 0, "")
	}

	// For Payment / Untuk Pembayaran
	pdf.SetFont("Times", "", 12)
	forPaymentLabel := loc("for_payment", "For Payment of")
	pdf.CellFormat(labelW, 7, forPaymentLabel, "", 0, "L", false, 0, "")
	pdf.CellFormat(colonW, 7, ":", "", 0, "L", false, 0, "")
	pdf.SetFont("Times", "B", 12)
	inv := sale.InvoiceNumber
	if strings.TrimSpace(inv) == "" {
		inv = sale.Code
	}
	invoiceLabel := loc("invoice_number", "INVOICE NO")
	invText := fmt.Sprintf("%s:    %s", invoiceLabel, inv)
	pdf.CellFormat(valueW, 7, invText, "", 1, "L", false, 0, "")
	// Notes below (smaller)
	notes := strings.TrimSpace(sale.Notes)
	if notes != "" {
		pdf.SetFont("Times", "", 11)
		pdf.CellFormat(labelW+colonW, 6, "", "", 0, "L", false, 0, "")
		pdf.MultiCell(valueW, 6, notes, "", "L", false)
	}

	pdf.Ln(8)

	// === REVISED LAYOUT: Amount Rp (Left) + Signature (Right) - No Overlap ===
	currentY := pdf.GetY()

	// LEFT COLUMN: Amount Rp. / Jumlah Rp. + amount box (smaller width)
	pdf.SetXY(lm, currentY)
	pdf.SetFont("Times", "B", 14)
	amountRpLabel := loc("amount_rp", "Amount Rp.")
	pdf.CellFormat(35, 12, amountRpLabel, "", 0, "L", false, 0, "")
	pdf.SetFillColor(220, 220, 220)
	boxW := 85.0 // Reduced from 120 to 85 to prevent overlap
	pdf.Rect(lm+40, currentY-2, boxW, 14, "F")
	pdf.SetXY(lm+40, currentY-2)
	pdf.SetFont("Times", "", 14) // Slightly smaller font to fit
	pdf.CellFormat(boxW, 14, utils.FormatRupiah(amount), "1", 0, "C", false, 0, "")

	// RIGHT COLUMN: place, date, and signature (better positioned)
	pdf.SetFont("Times", "", 11) // Slightly smaller for better fit
	city := p.getCompanyCity(companyInfo.CompanyAddress)
	dateStr := utils.NewDateUtils().FormatDateWithIndonesianMonth(time.Now())
	rightX := lm + boxW + 50 // Start after the amount box with some margin
	pdf.SetXY(rightX, currentY)
	pdf.CellFormat(80, 6, fmt.Sprintf("%s, %s", city, dateStr), "", 0, "L", false, 0, "")
	pdf.SetXY(rightX, currentY+6)
	receivedByLabel := loc("received_by", "Received by")
	pdf.CellFormat(80, 6, receivedByLabel+",", "", 0, "L", false, 0, "")

	// Signature line and name (positioned to avoid overlap)
	financeName := p.getFinanceSignatoryNameWithUser(userID)
	if strings.TrimSpace(financeName) == "" {
		financeName = "Finance"
	}
	lineW := 60.0 // Slightly smaller signature line
	lineX := rightX
	// Add signature line above the finance user name
	pdf.Line(lineX, currentY+25, lineX+lineW, currentY+25)
	pdf.SetXY(lineX, currentY+27)
	pdf.SetFont("Times", "", 11)
	pdf.CellFormat(lineW, 6, financeName, "", 0, "C", false, 0, "")

	// Move to next line after both columns
	pdf.SetY(currentY + 35)

	// Footer subtle line
	pdf.Ln(4)
	pdf.SetDrawColor(200, 200, 200)
	pdf.SetLineWidth(0.2)
	pdf.Line(lm, pdf.GetY(), pageW-rm, pdf.GetY())
	pdf.Ln(4)
	pdf.SetFont("Arial", "I", 8)
	pdf.SetTextColor(153, 153, 153)
	pdf.Cell(contentW, 4, fmt.Sprintf("Generated on %s", time.Now().Format("02/01/2006 15:04")))

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to generate sales receipt PDF: %v", err)
	}
	return buf.Bytes(), nil
}

// ========================================
// Phase 2: Priority Financial Reports PDF Export
// ========================================

// GenerateTrialBalancePDF generates PDF for trial balance
func (p *PDFService) GenerateTrialBalancePDF(trialBalanceData interface{}, asOfDate string) ([]byte, error) {
	utils.SanitizeExportData(trialBalanceData)
	// Create new PDF document with invoice-like layout
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// Layout helpers
	lm, _, _, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	rm := float64(15)
	contentW := pageW - lm - rm

	// Company info
	companyInfo, err := p.getCompanyInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get company info: %v", err)
	}

	// Unified letterhead
	contentY := drawUnifiedLetterhead(pdf, companyInfo, 35)

	// Title
	pdf.SetY(contentY)
	pdf.SetX(lm)
	pdf.SetFont("Arial", "B", 22)
	pdf.SetTextColor(25, 55, 109)
	pdf.Cell(contentW, 10, "TRIAL BALANCE")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(12)

	// Details two-column
	pdf.SetFont("Arial", "B", 9)
	pdf.SetX(lm)
	pdf.Cell(20, 5, "As of:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(60, 5, asOfDate)

	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(0, 0, 0)
	rightX := lm + contentW - 60
	pdf.SetX(rightX)
	pdf.Cell(26, 5, "Generated:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(34, 5, time.Now().Format("02/01/2006 15:04"))
	pdf.Ln(12)

	// Table headers - align with left margin (same as logo)
	// Reduced column widths: 20 + 60 + 40 + 40 = 160mm (safe for A4 with 15mm margins)
	pdf.SetX(lm)
	pdf.SetFont("Arial", "B", 7.5)
	pdf.SetFillColor(220, 220, 220)
	pdf.CellFormat(20, 8, "Account Code", "1", 0, "C", true, 0, "")
	pdf.CellFormat(60, 8, "Account Name", "1", 0, "C", true, 0, "")
	pdf.CellFormat(40, 8, "Debit Balance", "1", 0, "C", true, 0, "")
	pdf.CellFormat(40, 8, "Credit Balance", "1", 0, "C", true, 0, "")
	pdf.Ln(8)

	// Process trial balance data
	pdf.SetFont("Arial", "", 7.5)
	pdf.SetFillColor(255, 255, 255)

	totalDebits := 0.0
	totalCredits := 0.0

	// Normalize input to map[string]interface{} so we can iterate regardless of struct/map input
	var tbMap map[string]interface{}
	if m, ok := trialBalanceData.(map[string]interface{}); ok {
		tbMap = m
	} else {
		b, _ := json.Marshal(trialBalanceData)
		_ = json.Unmarshal(b, &tbMap)
	}

	if tbMap != nil {
		// Display accounts
		if accounts, exists := tbMap["accounts"]; exists {
			if accountsSlice, ok := accounts.([]interface{}); ok {
				for _, account := range accountsSlice {
					if accountMap, ok := account.(map[string]interface{}); ok {
						// Check if we need a new page
						if pdf.GetY() > 250 {
							pdf.AddPage()
							// Re-add headers aligned with left margin
							pdf.SetX(lm)
							pdf.SetFont("Arial", "B", 7.5)
							pdf.SetFillColor(220, 220, 220)
							pdf.CellFormat(20, 8, "Account Code", "1", 0, "C", true, 0, "")
							pdf.CellFormat(60, 8, "Account Name", "1", 0, "C", true, 0, "")
							pdf.CellFormat(40, 8, "Debit Balance", "1", 0, "C", true, 0, "")
							pdf.CellFormat(40, 8, "Credit Balance", "1", 0, "C", true, 0, "")
							pdf.Ln(8)
							pdf.SetFont("Arial", "", 7.5)
							pdf.SetFillColor(255, 255, 255)
						}

						accountCode := ""
						accountName := "Unknown Account"
						debitBalance := 0.0
						creditBalance := 0.0

						if code, exists := accountMap["account_code"]; exists {
							if codeStr, ok := code.(string); ok {
								accountCode = codeStr
							}
						}
						if name, exists := accountMap["account_name"]; exists {
							if nameStr, ok := name.(string); ok {
								accountName = nameStr
							}
						}
						if debit, exists := accountMap["debit_balance"]; exists {
							if debitFloat, ok := debit.(float64); ok {
								debitBalance = debitFloat
								totalDebits += debitBalance
							}
						}
						if credit, exists := accountMap["credit_balance"]; exists {
							if creditFloat, ok := credit.(float64); ok {
								creditBalance = creditFloat
								totalCredits += creditBalance
							}
						}

						// Truncate account name if too long
						if len(accountName) > 38 {
							accountName = accountName[:35] + "..."
						}

						// Align each row with left margin
						pdf.SetX(lm)
						pdf.CellFormat(20, 5, accountCode, "1", 0, "C", false, 0, "")
						pdf.CellFormat(60, 5, accountName, "1", 0, "L", false, 0, "")

						// Show debit balance or dash
						if debitBalance != 0 {
							pdf.CellFormat(40, 5, p.formatRupiah(debitBalance), "1", 0, "R", false, 0, "")
						} else {
							pdf.CellFormat(40, 5, "-", "1", 0, "R", false, 0, "")
						}

						// Show credit balance or dash
						if creditBalance != 0 {
							pdf.CellFormat(40, 5, p.formatRupiah(creditBalance), "1", 0, "R", false, 0, "")
						} else {
							pdf.CellFormat(40, 5, "-", "1", 0, "R", false, 0, "")
						}
						pdf.Ln(5)
					}
				}
			}
		}
	} else {
		// Fallback: simple data display
		pdf.SetX(lm)
		pdf.Cell(contentW, 6, "Trial Balance data structure not recognized")
		pdf.Ln(6)
	}

	// Totals section - align with left margin
	pdf.Ln(3)
	pdf.SetX(lm)
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(240, 240, 240)
	pdf.CellFormat(80, 6, "TOTAL", "1", 0, "R", true, 0, "")
	pdf.CellFormat(40, 6, p.formatRupiah(totalDebits), "1", 0, "R", true, 0, "")
	pdf.CellFormat(40, 6, p.formatRupiah(totalCredits), "1", 0, "R", true, 0, "")
	pdf.Ln(8)

	// Balance verification
	isBalanced := (totalDebits == totalCredits)
	balanceStatus := "BALANCED"
	if !isBalanced {
		balanceStatus = "NOT BALANCED"
	}

	pdf.SetX(lm)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(200, 200, 200)
	pdf.Cell(160, 6, fmt.Sprintf("BALANCE VERIFICATION: %s", balanceStatus))
	pdf.Ln(8)

	if !isBalanced {
		variance := totalDebits - totalCredits
		pdf.SetX(lm)
		pdf.SetFont("Arial", "", 9)
		pdf.Cell(160, 5, fmt.Sprintf("Variance: %s", p.formatRupiah(variance)))
		pdf.Ln(5)
	}

	// Footer - align with left margin
	pdf.Ln(10)
	pdf.SetX(lm)
	pdf.SetFont("Arial", "I", 8)
	pdf.Cell(160, 4, fmt.Sprintf("Report generated on %s", time.Now().Format("02/01/2006 15:04")))

	// Output to buffer
	var buf bytes.Buffer
	err = pdf.Output(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to generate trial balance PDF: %v", err)
	}

	return buf.Bytes(), nil
}

// GenerateTrialBalanceCSV generates a CSV file for trial balance.
func (p *PDFService) GenerateTrialBalanceCSV(trialBalanceData interface{}, asOfDate string) ([]byte, error) {
	utils.SanitizeExportData(trialBalanceData)
	var tbMap map[string]interface{}
	if m, ok := trialBalanceData.(map[string]interface{}); ok {
		tbMap = m
	} else {
		payload, err := json.Marshal(trialBalanceData)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal trial balance data: %v", err)
		}
		if err := json.Unmarshal(payload, &tbMap); err != nil {
			return nil, fmt.Errorf("failed to parse trial balance data: %v", err)
		}
	}

	var buf bytes.Buffer
	buf.WriteString("\uFEFF")
	buf.WriteString("sep=;\n")

	writer := utils.NewSanitizedCSVWriter(&buf)
	writer.Comma = ';'

	companyName := "Perusahaan"
	companyAddress := ""
	if companyMap, ok := tbMap["company"].(map[string]interface{}); ok && companyMap != nil {
		if name := getString(companyMap, "name"); name != "" {
			companyName = name
		}
		companyAddress = getString(companyMap, "address")
	}

	currency := getString(tbMap, "currency")
	if currency == "" {
		currency = "IDR"
	}

	totalDebits := getNumFrom(tbMap["total_debits"])
	totalCredits := getNumFrom(tbMap["total_credits"])
	difference := getNumFrom(tbMap["difference"])
	isBalanced := false
	if value, ok := tbMap["is_balanced"].(bool); ok {
		isBalanced = value
	}

	write := func(record []string) error {
		return writer.Write(record)
	}

	if err := write([]string{companyName}); err != nil {
		return nil, fmt.Errorf("failed to write CSV header: %v", err)
	}
	if companyAddress != "" {
		if err := write([]string{companyAddress}); err != nil {
			return nil, fmt.Errorf("failed to write CSV header: %v", err)
		}
	}
	if err := write([]string{}); err != nil {
		return nil, fmt.Errorf("failed to write CSV spacer: %v", err)
	}
	if err := write([]string{"TRIAL BALANCE"}); err != nil {
		return nil, fmt.Errorf("failed to write CSV title: %v", err)
	}
	if err := write([]string{"As of", asOfDate}); err != nil {
		return nil, fmt.Errorf("failed to write CSV as-of date: %v", err)
	}
	if err := write([]string{"Currency", currency}); err != nil {
		return nil, fmt.Errorf("failed to write CSV currency: %v", err)
	}
	if err := write([]string{"Generated", time.Now().Format("2006-01-02 15:04")}); err != nil {
		return nil, fmt.Errorf("failed to write CSV generated date: %v", err)
	}
	if err := write([]string{}); err != nil {
		return nil, fmt.Errorf("failed to write CSV spacer: %v", err)
	}

	if err := write([]string{"Account Code", "Account Name", "Account Type", "Debit Balance", "Credit Balance"}); err != nil {
		return nil, fmt.Errorf("failed to write CSV columns: %v", err)
	}

	if accounts, exists := tbMap["accounts"]; exists {
		if accountSlice, ok := accounts.([]interface{}); ok {
			for _, account := range accountSlice {
				accountMap, ok := account.(map[string]interface{})
				if !ok {
					continue
				}
				if err := write([]string{
					getString(accountMap, "account_code"),
					getString(accountMap, "account_name"),
					getString(accountMap, "account_type"),
					fmt.Sprintf("%.2f", getNumFrom(accountMap["debit_balance"])),
					fmt.Sprintf("%.2f", getNumFrom(accountMap["credit_balance"])),
				}); err != nil {
					return nil, fmt.Errorf("failed to write CSV account row: %v", err)
				}
			}
		}
	}

	if err := write([]string{}); err != nil {
		return nil, fmt.Errorf("failed to write CSV spacer: %v", err)
	}
	if err := write([]string{"TOTAL", "", "", fmt.Sprintf("%.2f", totalDebits), fmt.Sprintf("%.2f", totalCredits)}); err != nil {
		return nil, fmt.Errorf("failed to write CSV total row: %v", err)
	}
	if err := write([]string{"Difference", fmt.Sprintf("%.2f", difference)}); err != nil {
		return nil, fmt.Errorf("failed to write CSV difference row: %v", err)
	}
	if err := write([]string{"Status", map[bool]string{true: "BALANCED", false: "NOT BALANCED"}[isBalanced]}); err != nil {
		return nil, fmt.Errorf("failed to write CSV status row: %v", err)
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("failed to flush trial balance CSV: %v", err)
	}

	return buf.Bytes(), nil
}

// GenerateBalanceSheetPDF generates a PDF for Balance Sheet report
func (p *PDFService) GenerateBalanceSheetPDF(balanceSheetData interface{}, asOfDate string) ([]byte, error) {
	utils.SanitizeExportData(balanceSheetData)
	// Create new PDF document with invoice-like layout
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// Layout helpers
	lm, tm, rm, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	contentW := pageW - lm - rm

	// Company info
	companyInfo, err := p.getCompanyInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get company info: %v", err)
	}

	// Header: logo left, company info right
	logoX := lm
	logoY := tm
	logoSize := 35.0
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

	// Company profile right-aligned
	companyInfoX := pageW - rm
	companyInfoY := tm
	pdf.SetFont("Arial", "B", 12)
	nameW := pdf.GetStringWidth(companyInfo.CompanyName)
	pdf.SetXY(companyInfoX-nameW, companyInfoY)
	pdf.Cell(nameW, 6, companyInfo.CompanyName)

	pdf.SetFont("Arial", "", 9)
	addrW := pdf.GetStringWidth(companyInfo.CompanyAddress)
	pdf.SetXY(companyInfoX-addrW, companyInfoY+8)
	pdf.Cell(addrW, 4, companyInfo.CompanyAddress)

	phoneText := fmt.Sprintf("Phone: %s", companyInfo.CompanyPhone)
	phoneW := pdf.GetStringWidth(phoneText)
	pdf.SetXY(companyInfoX-phoneW, companyInfoY+14)
	pdf.Cell(phoneW, 4, phoneText)

	// Divider line under header
	pdf.SetDrawColor(238, 238, 238)
	pdf.SetLineWidth(0.2)
	pdf.Line(lm, tm+45, pageW-rm, tm+45)

	// Title
	pdf.SetY(tm + 55)
	pdf.SetX(lm)
	pdf.SetFont("Arial", "B", 22)
	pdf.SetTextColor(51, 51, 51)
	pdf.Cell(contentW, 10, "BALANCE SHEET")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(18)

	// Report details (two columns)
	pdf.SetFont("Arial", "B", 9)
	pdf.SetX(lm)
	pdf.Cell(25, 5, "As of:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(80, 5, asOfDate)

	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(0, 0, 0)
	detailRightX := lm + contentW - 60
	pdf.SetX(detailRightX)
	pdf.Cell(26, 5, "Generated:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(34, 5, time.Now().Format("02/01/2006 15:04"))
	pdf.Ln(10)

	// Process balance sheet data
	if bsMap, ok := balanceSheetData.(map[string]interface{}); ok {
		p.addBalanceSheetSections(pdf, bsMap)
	} else {
		// Try to convert struct -> map[string]interface{} via JSON roundtrip
		if m := p.tryConvertBSDataToMap(balanceSheetData); m != nil {
			p.addBalanceSheetSections(pdf, m)
		} else {
			pdf.Cell(190, 6, "Balance Sheet data not available")
			pdf.Ln(6)
		}
	}

	// Footer
	pdf.Ln(10)
	pdf.SetFont("Arial", "I", 8)
	pdf.Cell(190, 4, fmt.Sprintf("Report generated on %s", time.Now().Format("02/01/2006 15:04")))

	// Output to buffer
	var buf bytes.Buffer
	err = pdf.Output(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to generate balance sheet PDF: %v", err)
	}

	return buf.Bytes(), nil
}

// tryConvertBSDataToMap attempts to convert a struct balance sheet into a generic map
func (p *PDFService) tryConvertBSDataToMap(data interface{}) map[string]interface{} {
	b, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}

// addBalanceSheetSections adds balance sheet sections to PDF
func (p *PDFService) addBalanceSheetSections(pdf *gofpdf.Fpdf, bsData map[string]interface{}) {
	pdf.SetFont("Arial", "B", 12)

	// Assets section
	if assets, exists := bsData["assets"]; exists {
		pdf.Cell(190, 8, "ASSETS")
		pdf.Ln(8)
		p.addAssetsSections(pdf, assets)
		pdf.Ln(5)
	}

	// Liabilities section
	if liabilities, exists := bsData["liabilities"]; exists {
		pdf.Cell(190, 8, "LIABILITIES")
		pdf.Ln(8)
		p.addLiabilitiesSections(pdf, liabilities)
		pdf.Ln(5)
	}

	// Equity section
	if equity, exists := bsData["equity"]; exists {
		pdf.Cell(190, 8, "EQUITY")
		pdf.Ln(8)
		p.addEquitySection(pdf, equity)
		pdf.Ln(5)
	}

	// Balance verification
	p.addBalanceVerification(pdf, bsData)
}

// addAssetsSections adds assets sections to balance sheet PDF
func (p *PDFService) addAssetsSections(pdf *gofpdf.Fpdf, assets interface{}) {
	pdf.SetFont("Arial", "B", 10)

	if assetsMap, ok := assets.(map[string]interface{}); ok {
		// Current Assets
		if currentAssets, exists := assetsMap["current_assets"]; exists {
			pdf.Cell(190, 6, "  Current Assets")
			pdf.Ln(6)
			p.addAccountItems(pdf, currentAssets, "    ")

			// Support both nested SSOT shape (assets.current_assets.total_current_assets)
			// and legacy flat key (assets.current_assets_total)
			var subtotal interface{}
			if caMap, ok := currentAssets.(map[string]interface{}); ok {
				if v, ok := caMap["total_current_assets"]; ok {
					subtotal = v
				}
			}
			if subtotal == nil {
				if v, ok := assetsMap["current_assets_total"]; ok {
					subtotal = v
				}
			}
			if subtotal != nil {
				p.addTotalLine(pdf, "  Total Current Assets", subtotal)
			}
		}

		// Non-Current Assets
		if nonCurrentAssets, exists := assetsMap["non_current_assets"]; exists {
			pdf.Ln(3)
			pdf.Cell(190, 6, "  Non-Current Assets")
			pdf.Ln(6)
			p.addAccountItems(pdf, nonCurrentAssets, "    ")

			// Support nested SSOT key total_non_current_assets and legacy flat key
			var subtotal interface{}
			if ncaMap, ok := nonCurrentAssets.(map[string]interface{}); ok {
				if v, ok := ncaMap["total_non_current_assets"]; ok {
					subtotal = v
				}
			}
			if subtotal == nil {
				if v, ok := assetsMap["non_current_assets_total"]; ok {
					subtotal = v
				}
			}
			if subtotal != nil {
				p.addTotalLine(pdf, "  Total Non-Current Assets", subtotal)
			}
		}

		// Total Assets
		if totalAssets, exists := assetsMap["total_assets"]; exists {
			pdf.Ln(3)
			pdf.SetFont("Arial", "B", 10)
			pdf.SetFillColor(240, 240, 240)
			pdf.CellFormat(145, 6, "TOTAL ASSETS", "1", 0, "L", true, 0, "")
			if totalFloat, ok := totalAssets.(float64); ok {
				pdf.CellFormat(45, 6, p.formatRupiah(totalFloat), "1", 0, "R", true, 0, "")
			} else {
				pdf.CellFormat(45, 6, fmt.Sprintf("%v", totalAssets), "1", 0, "R", true, 0, "")
			}
			pdf.Ln(6)
		}
	}
}

// addLiabilitiesSections adds liabilities sections to balance sheet PDF
func (p *PDFService) addLiabilitiesSections(pdf *gofpdf.Fpdf, liabilities interface{}) {
	pdf.SetFont("Arial", "B", 10)

	if liabilitiesMap, ok := liabilities.(map[string]interface{}); ok {
		// Current Liabilities
		if currentLiabilities, exists := liabilitiesMap["current_liabilities"]; exists {
			pdf.Cell(190, 6, "  Current Liabilities")
			pdf.Ln(6)
			p.addAccountItems(pdf, currentLiabilities, "    ")

			// Support nested SSOT key total_current_liabilities and legacy flat key
			var subtotal interface{}
			if clMap, ok := currentLiabilities.(map[string]interface{}); ok {
				if v, ok := clMap["total_current_liabilities"]; ok {
					subtotal = v
				}
			}
			if subtotal == nil {
				if v, ok := liabilitiesMap["current_liabilities_total"]; ok {
					subtotal = v
				}
			}
			if subtotal != nil {
				p.addTotalLine(pdf, "  Total Current Liabilities", subtotal)
			}
		}

		// Non-Current Liabilities
		if nonCurrentLiabilities, exists := liabilitiesMap["non_current_liabilities"]; exists {
			pdf.Ln(3)
			pdf.Cell(190, 6, "  Non-Current Liabilities")
			pdf.Ln(6)
			p.addAccountItems(pdf, nonCurrentLiabilities, "    ")

			// Support nested SSOT key total_non_current_liabilities and legacy flat key
			var subtotal interface{}
			if nclMap, ok := nonCurrentLiabilities.(map[string]interface{}); ok {
				if v, ok := nclMap["total_non_current_liabilities"]; ok {
					subtotal = v
				}
			}
			if subtotal == nil {
				if v, ok := liabilitiesMap["non_current_liabilities_total"]; ok {
					subtotal = v
				}
			}
			if subtotal != nil {
				p.addTotalLine(pdf, "  Total Non-Current Liabilities", subtotal)
			}
		}

		// Total Liabilities
		if totalLiabilities, exists := liabilitiesMap["total_liabilities"]; exists {
			pdf.Ln(3)
			pdf.SetFont("Arial", "B", 10)
			pdf.SetFillColor(240, 240, 240)
			pdf.CellFormat(145, 6, "TOTAL LIABILITIES", "1", 0, "L", true, 0, "")
			if totalFloat, ok := totalLiabilities.(float64); ok {
				pdf.CellFormat(45, 6, p.formatRupiah(totalFloat), "1", 0, "R", true, 0, "")
			} else {
				pdf.CellFormat(45, 6, fmt.Sprintf("%v", totalLiabilities), "1", 0, "R", true, 0, "")
			}
			pdf.Ln(6)
		}
	}
}

// addEquitySection adds equity section to balance sheet PDF
func (p *PDFService) addEquitySection(pdf *gofpdf.Fpdf, equity interface{}) {
	pdf.SetFont("Arial", "B", 10)

	if equityMap, ok := equity.(map[string]interface{}); ok {
		p.addAccountItems(pdf, equity, "  ")

		// Total Equity
		if totalEquity, exists := equityMap["total_equity"]; exists {
			pdf.Ln(3)
			pdf.SetFont("Arial", "B", 10)
			pdf.SetFillColor(240, 240, 240)
			pdf.CellFormat(145, 6, "TOTAL EQUITY", "1", 0, "L", true, 0, "")
			if totalFloat, ok := totalEquity.(float64); ok {
				pdf.CellFormat(45, 6, p.formatRupiah(totalFloat), "1", 0, "R", true, 0, "")
			} else {
				pdf.CellFormat(45, 6, fmt.Sprintf("%v", totalEquity), "1", 0, "R", true, 0, "")
			}
			pdf.Ln(6)
		}
	}
}

// addAccountItems adds account items to PDF with indentation
func (p *PDFService) addAccountItems(pdf *gofpdf.Fpdf, items interface{}, indent string) {
	pdf.SetFont("Arial", "", 9)

	// Helper to safely get string/float values from a map with multiple possible keys
	getString := func(m map[string]interface{}, keys ...string) string {
		for _, k := range keys {
			if v, ok := m[k]; ok {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
			}
		}
		return ""
	}
	getFloat := func(m map[string]interface{}, keys ...string) float64 {
		for _, k := range keys {
			if v, ok := m[k]; ok {
				if f, ok := v.(float64); ok {
					return f
				}
			}
		}
		return 0.0
	}

	if itemsSlice, ok := items.([]interface{}); ok {
		for _, item := range itemsSlice {
			if itemMap, ok := item.(map[string]interface{}); ok {
				// Prefer "account_name" (our SSOT struct tag), fallback to generic "name"
				name := getString(itemMap, "account_name", "name", "AccountName")
				if name == "" {
					name = "Unknown Account"
				}
				// Include account code when available (e.g., "1101 - Kas")
				code := getString(itemMap, "account_code", "code", "AccountCode")
				label := name
				if code != "" {
					label = fmt.Sprintf("%s - %s", code, name)
				}
				// Prefer "amount", but also handle "balance" or "value" if present
				amount := getFloat(itemMap, "amount", "balance", "value")

				pdf.Cell(145, 5, fmt.Sprintf("%s%s", indent, label))
				pdf.Cell(45, 5, p.formatRupiah(amount))
				pdf.Ln(5)
			}
		}
	} else if itemsMap, ok := items.(map[string]interface{}); ok {
		// Handle map structure
		for key, value := range itemsMap {
			if key == "items" {
				p.addAccountItems(pdf, value, indent)
			}
		}
	}
}

// addTotalLine adds a total line to the PDF
func (p *PDFService) addTotalLine(pdf *gofpdf.Fpdf, label string, total interface{}) {
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(250, 250, 250)
	pdf.CellFormat(145, 5, label, "1", 0, "L", true, 0, "")

	if totalFloat, ok := total.(float64); ok {
		pdf.CellFormat(45, 5, p.formatRupiah(totalFloat), "1", 0, "R", true, 0, "")
	} else {
		pdf.CellFormat(45, 5, fmt.Sprintf("%v", total), "1", 0, "R", true, 0, "")
	}
	pdf.Ln(5)
}

// addBalanceVerification adds balance verification to balance sheet PDF
func (p *PDFService) addBalanceVerification(pdf *gofpdf.Fpdf, bsData map[string]interface{}) {
	pdf.Ln(5)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(220, 220, 220)

	isBalanced := false
	balanceDifference := 0.0

	if balancedVal, exists := bsData["is_balanced"]; exists {
		if balancedBool, ok := balancedVal.(bool); ok {
			isBalanced = balancedBool
		}
	}

	if diffVal, exists := bsData["balance_difference"]; exists {
		if diffFloat, ok := diffVal.(float64); ok {
			balanceDifference = diffFloat
		}
	}

	balanceStatus := "BALANCED"
	if !isBalanced {
		balanceStatus = "NOT BALANCED"
	}

	pdf.CellFormat(190, 8, fmt.Sprintf("BALANCE VERIFICATION: %s", balanceStatus), "1", 0, "C", true, 0, "")
	pdf.Ln(8)

	if !isBalanced {
		pdf.SetFont("Arial", "", 9)
		pdf.Cell(190, 5, fmt.Sprintf("Balance Difference: %s", p.formatRupiah(balanceDifference)))
		pdf.Ln(5)
	}
}

// GenerateProfitLossPDF generates a PDF for Profit & Loss report
func (p *PDFService) GenerateProfitLossPDF(plData interface{}, startDate, endDate string) ([]byte, error) {
	utils.SanitizeExportData(plData)
	// Create new PDF document with invoice-like layout
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// Layout helpers
	lm, tm, rm, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	contentW := pageW - lm - rm

	// Company info
	companyInfo, err := p.getCompanyInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get company info: %v", err)
	}

	// Header: logo left, company profile right
	logoX := lm
	logoY := tm
	logoSize := 35.0
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

	// Company text on right
	companyInfoX := pageW - rm
	companyInfoY := tm
	pdf.SetFont("Arial", "B", 12)
	nameW := pdf.GetStringWidth(companyInfo.CompanyName)
	pdf.SetXY(companyInfoX-nameW, companyInfoY)
	pdf.Cell(nameW, 6, companyInfo.CompanyName)

	pdf.SetFont("Arial", "", 9)
	addrW := pdf.GetStringWidth(companyInfo.CompanyAddress)
	pdf.SetXY(companyInfoX-addrW, companyInfoY+8)
	pdf.Cell(addrW, 4, companyInfo.CompanyAddress)

	phoneText := fmt.Sprintf("Phone: %s", companyInfo.CompanyPhone)
	phoneW := pdf.GetStringWidth(phoneText)
	pdf.SetXY(companyInfoX-phoneW, companyInfoY+14)
	pdf.Cell(phoneW, 4, phoneText)

	// Divider under header
	pdf.SetDrawColor(238, 238, 238)
	pdf.SetLineWidth(0.2)
	pdf.Line(lm, tm+45, pageW-rm, tm+45)

	// Title
	pdf.SetY(tm + 55)
	pdf.SetX(lm)
	pdf.SetFont("Arial", "B", 22)
	pdf.SetTextColor(51, 51, 51)
	pdf.Cell(contentW, 10, "PROFIT & LOSS STATEMENT")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(18)

	// Report details (two-column style)
	pdf.SetFont("Arial", "B", 9)
	pdf.SetX(lm)
	pdf.Cell(25, 5, "Period:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(80, 5, fmt.Sprintf("%s to %s", startDate, endDate))

	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(0, 0, 0)
	detailRightX := lm + contentW - 60
	pdf.SetX(detailRightX)
	pdf.Cell(26, 5, "Generated:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(34, 5, time.Now().Format("02/01/2006 15:04"))
	pdf.Ln(10)

	// Process P&L data
	if plMap, ok := plData.(map[string]interface{}); ok {
		p.addProfitLossSections(pdf, plMap)
	} else {
		pdf.Cell(190, 6, "Profit & Loss data not available")
		pdf.Ln(6)
	}

	// Footer
	pdf.Ln(10)
	pdf.SetFont("Arial", "I", 8)
	pdf.Cell(190, 4, fmt.Sprintf("Report generated on %s", time.Now().Format("02/01/2006 15:04")))

	// Output to buffer
	var buf bytes.Buffer
	err = pdf.Output(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to generate profit & loss PDF: %v", err)
	}

	return buf.Bytes(), nil
}

// GenerateJournalAnalysisPDF generates a PDF for Journal Entry Analysis report
func (p *PDFService) GenerateJournalAnalysisPDF(journalData interface{}, startDate, endDate string) ([]byte, error) {
	utils.SanitizeExportData(journalData)
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// Layout helpers
	lm, tm, rm, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	contentW := pageW - lm - rm

	// Company info
	companyInfo, err := p.getCompanyInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get company info: %v", err)
	}

	// Header: logo left, company right
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

	// Company text on right
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

	// Divider under header
	pdf.SetDrawColor(238, 238, 238)
	pdf.SetLineWidth(0.2)
	pdf.Line(lm, tm+45, pageW-rm, tm+45)

	// Title
	pdf.SetY(tm + 55)
	pdf.SetFont("Arial", "B", 22)
	pdf.SetTextColor(51, 51, 51)
	pdf.Cell(contentW, 10, "JOURNAL ENTRY ANALYSIS")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(12)

	// Report details (two-column style)
	pdf.SetFont("Arial", "B", 9)
	pdf.SetX(lm)
	pdf.Cell(25, 5, "Period:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(70, 5, fmt.Sprintf("%s to %s", startDate, endDate))

	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(0, 0, 0)
	rightX := lm + contentW - 60
	pdf.SetX(rightX)
	pdf.Cell(26, 5, "Generated:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(102, 102, 102)
	pdf.Cell(34, 5, time.Now().Format("02/01/2006 15:04"))
	pdf.Ln(10)

	// Normalize input to generic map (structs are common here)
	var dataMap map[string]interface{}
	if m, ok := journalData.(map[string]interface{}); ok {
		dataMap = m
	} else {
		b, _ := json.Marshal(journalData)
		_ = json.Unmarshal(b, &dataMap)
	}
	if dataMap == nil {
		dataMap = map[string]interface{}{}
	}

	// SUMMARY
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(190, 8, "SUMMARY")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 10)
	pdf.SetFillColor(230, 230, 230)

	getNum := func(keys ...string) float64 {
		for _, k := range keys {
			if v, ok := dataMap[k]; ok {
				switch t := v.(type) {
				case float64:
					return t
				case int:
					return float64(t)
				case int64:
					return float64(t)
				case json.Number:
					f, _ := t.Float64()
					return f
				default:
					if f, err := strconv.ParseFloat(fmt.Sprintf("%v", t), 64); err == nil {
						return f
					}
				}
			}
		}
		return 0
	}

	left := []struct {
		label    string
		value    float64
		currency bool
	}{
		{"Total Entries", getNum("total_entries", "TotalEntries"), false},
		{"Posted Entries", getNum("posted_entries", "PostedEntries"), false},
		{"Draft Entries", getNum("draft_entries", "DraftEntries"), false},
	}
	right := []struct {
		label    string
		value    float64
		currency bool
	}{
		{"Reversed Entries", getNum("reversed_entries", "ReversedEntries"), false},
		{"Total Amount", getNum("total_amount", "TotalAmount"), true},
	}

	for i := 0; i < len(left) || i < len(right); i++ {
		if i < len(left) {
			pdf.CellFormat(60, 6, left[i].label, "1", 0, "L", true, 0, "")
			val := strconv.Itoa(int(left[i].value))
			if left[i].currency {
				val = p.formatRupiah(left[i].value)
			}
			pdf.CellFormat(35, 6, val, "1", 0, "R", true, 0, "")
		} else {
			pdf.Cell(95, 6, "")
		}
		if i < len(right) {
			pdf.CellFormat(60, 6, right[i].label, "1", 0, "L", true, 0, "")
			val := strconv.Itoa(int(right[i].value))
			if right[i].currency {
				val = p.formatRupiah(right[i].value)
			}
			pdf.CellFormat(35, 6, val, "1", 1, "R", true, 0, "")
		} else {
			pdf.Cell(95, 6, "")
			pdf.Ln(6)
		}
	}

	// ENTRIES BY TYPE
	pdf.Ln(8)
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(190, 8, "ENTRIES BY TYPE")
	pdf.Ln(8)
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(220, 220, 220)
	pdf.CellFormat(70, 8, "Source Type", "1", 0, "L", true, 0, "")
	pdf.CellFormat(40, 8, "Count", "1", 0, "C", true, 0, "")
	pdf.CellFormat(40, 8, "Amount", "1", 0, "R", true, 0, "")
	pdf.CellFormat(40, 8, "Percentage", "1", 1, "C", true, 0, "")
	pdf.SetFont("Arial", "", 9)

	entriesAny, ok := dataMap["entries_by_type"]
	if !ok {
		entriesAny, ok = dataMap["EntriesByType"]
	}
	if !ok {
		entriesAny, ok = dataMap["entriesByType"]
	}

	if ok {
		if items, ok2 := entriesAny.([]interface{}); ok2 {
			for _, it := range items {
				if m, ok3 := it.(map[string]interface{}); ok3 {
					sType := ""
					if v, ok := m["source_type"].(string); ok {
						sType = v
					} else if v, ok := m["SourceType"].(string); ok {
						sType = v
					}
					count := getNumFrom(m["count"])
					amount := getNumFrom(m["total_amount"])
					if amount == 0 {
						amount = getNumFrom(m["TotalAmount"])
					}
					perc := getNumFrom(m["percentage"])
					if perc == 0 {
						perc = getNumFrom(m["Percentage"])
					}
					pdf.CellFormat(70, 6, sType, "1", 0, "L", false, 0, "")
					pdf.CellFormat(40, 6, strconv.Itoa(int(count)), "1", 0, "C", false, 0, "")
					pdf.CellFormat(40, 6, p.formatRupiah(amount), "1", 0, "R", false, 0, "")
					pdf.CellFormat(40, 6, fmt.Sprintf("%.2f%%", perc), "1", 1, "C", false, 0, "")
				}
			}
		} else {
			pdf.SetFont("Arial", "I", 9)
			pdf.CellFormat(190, 6, "No journal entries found for the selected period", "1", 1, "C", false, 0, "")
		}
	} else {
		pdf.SetFont("Arial", "I", 9)
		pdf.CellFormat(190, 6, "Entry type breakdown data not available", "1", 1, "C", false, 0, "")
	}

	// Footer
	pdf.Ln(10)
	pdf.SetFont("Arial", "I", 8)
	pdf.Cell(190, 4, fmt.Sprintf("Report generated on %s", time.Now().Format("02/01/2006 15:04")))

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to generate journal analysis PDF: %v", err)
	}
	return buf.Bytes(), nil
}

// getMapKeys returns all keys from a map for debugging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// getNumFrom converts various number representations to float64 (helper for PDF tables)
func getNumFrom(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	case string:
		// Handle numbers encoded as strings (e.g., from decimal.Decimal JSON)
		if f, err := strconv.ParseFloat(t, 64); err == nil {
			return f
		}
		return 0
	default:
		return 0
	}
}

// renderSSOTFinancialSections renders the financial sections with extracted data
func (p *PDFService) renderSSOTFinancialSections(pdf *gofpdf.Fpdf, data map[string]interface{}, language string) {
	// Helper to get nested float values with multiple candidate paths
	get := func(candidates ...[]string) float64 {
		for _, path := range candidates {
			cur := interface{}(data)
			okPath := true
			for _, key := range path {
				m, ok := cur.(map[string]interface{})
				if !ok {
					okPath = false
					break
				}
				v, exists := m[key]
				if !exists {
					okPath = false
					break
				}
				cur = v
			}
			if okPath {
				return getNumFrom(cur)
			}
		}
		return 0
	}

	// Helper to get nested array values
	getArray := func(candidates ...[]string) []interface{} {
		for _, path := range candidates {
			cur := interface{}(data)
			okPath := true
			for _, key := range path {
				m, ok := cur.(map[string]interface{})
				if !ok {
					okPath = false
					break
				}
				v, exists := m[key]
				if !exists {
					okPath = false
					break
				}
				cur = v
			}
			if okPath {
				if arr, ok := cur.([]interface{}); ok {
					return arr
				}
			}
		}
		return nil
	}

	// REVENUE SECTION
	revenue := get([]string{"revenue", "total_revenue"}, []string{"TotalRevenue"})
	if revenue > 0 {
		p.addSSOTSection(pdf, utils.T("revenue", language), revenue, utils.T("revenue_description", language), utils.T("operating_tag", language))

		// Add revenue account details
		if revenueItems := getArray([]string{"revenue", "items"}); len(revenueItems) > 0 {
			p.addAccountDetails(pdf, utils.T("revenue_accounts", language), revenueItems)
		}
	}

	// COST OF GOODS SOLD SECTION
	cogs := get([]string{"cost_of_goods_sold", "total_cogs"}, []string{"COGS", "TotalCOGS"})
	if cogs > 0 {
		p.addSSOTSection(pdf, utils.T("cost_of_goods_sold", language), cogs, utils.T("cogs_description", language), utils.T("operating_tag", language))

		// Add COGS account details
		if cogsItems := getArray([]string{"cost_of_goods_sold", "items"}); len(cogsItems) > 0 {
			p.addAccountDetails(pdf, utils.T("cogs_accounts", language), cogsItems)
		}
	}

	// GROSS PROFIT
	gp := get([]string{"gross_profit"}, []string{"GrossProfit"})
	p.addSSOTTotalLine(pdf, utils.T("gross_profit", language), gp, utils.T("operating_tag", language))
	gpm := get([]string{"gross_profit_margin"}, []string{"GrossProfitMargin"})
	if gpm > 0 {
		pdf.SetFont("Arial", "I", 8)
		pdf.SetTextColor(100, 100, 100)
		pdf.CellFormat(180, 4, fmt.Sprintf("%s: %.1f%%", utils.T("gross_profit_margin", language), gpm), "", 0, "R", false, 0, "")
		pdf.Ln(5)
		pdf.SetTextColor(0, 0, 0)
	}

	// OPERATING EXPENSES
	opex := get([]string{"operating_expenses", "total_opex"}, []string{"OperatingExpenses", "TotalOpEx"})
	if opex > 0 {
		p.addSSOTSection(pdf, utils.T("operating_expenses", language), opex, utils.T("opex_description", language), utils.T("operating_tag", language))

		// Add operating expenses account details
		if adminItems := getArray([]string{"operating_expenses", "administrative", "items"}); len(adminItems) > 0 {
			p.addAccountDetails(pdf, utils.T("administrative_expenses", language), adminItems)
		}

		if sellItems := getArray([]string{"operating_expenses", "selling_marketing", "items"}); len(sellItems) > 0 {
			p.addAccountDetails(pdf, utils.T("selling_marketing", language), sellItems)
		}

		if genItems := getArray([]string{"operating_expenses", "general", "items"}); len(genItems) > 0 {
			p.addAccountDetails(pdf, utils.T("general_expenses", language), genItems)
		}
	}

	// OPERATING INCOME
	oi := get([]string{"operating_income"}, []string{"OperatingIncome"})
	p.addSSOTTotalLine(pdf, utils.T("operating_income", language), oi, utils.T("operating_tag", language))
	om := get([]string{"operating_margin"}, []string{"OperatingMargin"})
	if om > 0 {
		pdf.SetFont("Arial", "I", 8)
		pdf.SetTextColor(100, 100, 100)
		pdf.CellFormat(180, 4, fmt.Sprintf("%s: %.1f%%", utils.T("operating_margin", language), om), "", 0, "R", false, 0, "")
		pdf.Ln(5)
		pdf.SetTextColor(0, 0, 0)
	}

	// OTHER INCOME/EXPENSES (support snake_case and PascalCase)
	if inc := get([]string{"other_income"}, []string{"OtherIncome"}); inc != 0 {
		data["OtherIncome"] = inc
	}
	if exp := get([]string{"other_expenses"}, []string{"OtherExpenses"}); exp != 0 {
		data["OtherExpenses"] = exp
	}
	p.addOtherIncomeExpenses(pdf, data, language)

	// INCOME BEFORE TAX
	ibt := get([]string{"income_before_tax"}, []string{"IncomeBeforeTax"})
	p.addSSOTTotalLine(pdf, utils.T("income_before_tax", language), ibt, utils.T("final_tag", language))

	// TAX EXPENSE
	tax := get([]string{"tax_expense"}, []string{"TaxExpense"})
	if tax != 0 {
		pdf.SetFont("Arial", "", 9)
		pdf.SetFillColor(255, 255, 255)
		pdf.CellFormat(130, 6, "    "+utils.T("tax_expense", language), "LB", 0, "L", true, 0, "")
		pdf.CellFormat(50, 6, p.formatRupiah(tax), "RB", 0, "R", true, 0, "")
		pdf.Ln(6)
	}

	// NET INCOME — prominent green highlight
	ni := get([]string{"net_income"}, []string{"NetIncome"})
	pdf.Ln(2)
	pdf.SetFont("Arial", "B", 13)
	pdf.SetFillColor(27, 94, 32) // Dark green
	pdf.SetTextColor(255, 255, 255)
	pdf.CellFormat(130, 10, "  "+p.formatSSOTSectionTitle(utils.T("net_income", language), utils.T("final_tag", language)), "1", 0, "L", true, 0, "")
	pdf.CellFormat(50, 10, p.formatRupiah(ni), "1", 0, "R", true, 0, "")
	pdf.Ln(10)
	pdf.SetTextColor(0, 0, 0)
	if nim := get([]string{"net_income_margin"}, []string{"NetIncomeMargin"}); nim > 0 {
		pdf.SetFont("Arial", "I", 8)
		pdf.SetTextColor(100, 100, 100)
		pdf.CellFormat(180, 4, fmt.Sprintf("%s: %.1f%%", utils.T("net_income_margin", language), nim), "", 0, "R", false, 0, "")
		pdf.Ln(5)
		pdf.SetTextColor(0, 0, 0)
		data["NetIncomeMargin"] = nim
	}

	// Ensure ratios exist in data map for the summary section.
	// Compute from available totals when not provided by the service.
	if _, exists := data["GrossProfitMargin"]; !exists {
		if revenue > 0 && gp != 0 {
			data["GrossProfitMargin"] = (gp / revenue) * 100.0
		}
	}
	if _, exists := data["OperatingMargin"]; !exists {
		if revenue > 0 && oi != 0 {
			data["OperatingMargin"] = (oi / revenue) * 100.0
		}
	}
	// Best-effort EBITDA and margin: if not present, approximate EBITDA with Operating Income
	// when depreciation/amortization are not separately available.
	if _, exists := data["EBITDA"]; !exists {
		data["EBITDA"] = oi
	}
	if _, exists := data["EBITDAMargin"]; !exists {
		if revenue > 0 && oi != 0 { // use OI approximation if EBITDA not explicitly given
			if e, ok := data["EBITDA"].(float64); ok && e != 0 {
				data["EBITDAMargin"] = (e / revenue) * 100.0
			}
		}
	}
	if _, exists := data["NetIncomeMargin"]; !exists {
		if revenue > 0 && ni != 0 {
			data["NetIncomeMargin"] = (ni / revenue) * 100.0
		}
	}

	// Add financial ratios summary (function already flexible)
	p.addFinancialRatiosSummary(pdf, data, language)
}

// renderSSOTPlaceholder renders placeholder content when data extraction fails
func (p *PDFService) renderSSOTPlaceholder(pdf *gofpdf.Fpdf, _ interface{}) {
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(190, 8, "SSOT PROFIT & LOSS REPORT")
	pdf.Ln(8)

	pdf.SetFont("Arial", "", 10)
	pdf.Cell(190, 6, "This report is generated from the Single Source of Truth (SSOT) journal system.")
	pdf.Ln(6)
	pdf.Cell(190, 6, "Data includes revenue, expenses, and financial metrics from journal entries.")
	pdf.Ln(10)

	// Show that we received some data
	pdf.SetFont("Arial", "", 9)
	pdf.Cell(190, 5, "Status: PDF generation successful - SSOT data structure received")
	pdf.Ln(5)
	pdf.Cell(190, 5, "Note: Detailed financial data parsing is in progress...")
	pdf.Ln(10)

	// Add some basic structure
	p.addPlaceholderSections(pdf)
}

func (p *PDFService) formatSSOTSectionTitle(title, classification string) string {
	if strings.TrimSpace(classification) == "" {
		return title
	}

	return fmt.Sprintf("%s  [%s]", title, classification)
}

func (p *PDFService) addSSOTSectionMappingLegend(pdf *gofpdf.Fpdf, language string) {
	pdf.SetFont("Arial", "B", 8)
	pdf.SetFillColor(240, 244, 248)
	pdf.SetTextColor(41, 65, 122)
	pdf.CellFormat(180, 5, "  "+utils.T("section_mapping", language), "1", 0, "L", true, 0, "")
	pdf.Ln(5)

	pdf.SetFont("Arial", "", 7)
	pdf.SetTextColor(90, 90, 90)
	pdf.SetFillColor(250, 251, 253)
	pdf.CellFormat(180, 4, "  "+utils.T("mapping_operating_pdf", language), "LR", 0, "L", true, 0, "")
	pdf.Ln(4)
	pdf.CellFormat(180, 4, "  "+utils.T("mapping_non_operating_pdf", language), "LR", 0, "L", true, 0, "")
	pdf.Ln(4)
	pdf.CellFormat(180, 4, "  "+utils.T("mapping_final_pdf", language), "LBR", 0, "L", true, 0, "")
	pdf.Ln(7)
	pdf.SetTextColor(0, 0, 0)
}

// addSSOTSection adds a professional section header + total row
func (p *PDFService) addSSOTSection(pdf *gofpdf.Fpdf, title string, amount float64, description string, classification string) {
	// Dark blue section header
	pdf.SetFont("Arial", "B", 11)
	pdf.SetFillColor(41, 65, 122) // Dark navy blue
	pdf.SetTextColor(255, 255, 255)
	pdf.CellFormat(130, 7, "  "+p.formatSSOTSectionTitle(title, classification), "1", 0, "L", true, 0, "")
	pdf.CellFormat(50, 7, p.formatRupiah(amount), "1", 0, "R", true, 0, "")
	pdf.Ln(7)
	pdf.SetTextColor(0, 0, 0)

	if description != "" {
		pdf.SetFont("Arial", "I", 8)
		pdf.SetFillColor(240, 244, 248)
		pdf.SetTextColor(100, 100, 100)
		pdf.CellFormat(180, 5, "  "+description, "LR", 0, "L", true, 0, "")
		pdf.Ln(5)
		pdf.SetTextColor(0, 0, 0)
	}
}

// addSSOTTotalLine adds a highlighted total/subtotal row
func (p *PDFService) addSSOTTotalLine(pdf *gofpdf.Fpdf, title string, amount float64, classification string) {
	pdf.SetFont("Arial", "B", 11)
	pdf.SetFillColor(232, 240, 254) // Light blue accent
	pdf.SetTextColor(41, 65, 122)
	pdf.CellFormat(130, 8, "  "+p.formatSSOTSectionTitle(title, classification), "1", 0, "L", true, 0, "")
	pdf.CellFormat(50, 8, p.formatRupiah(amount), "1", 0, "R", true, 0, "")
	pdf.Ln(8)
	pdf.SetTextColor(0, 0, 0)
}

// addOtherIncomeExpenses adds other income and expenses section
func (p *PDFService) addOtherIncomeExpenses(pdf *gofpdf.Fpdf, data map[string]interface{}, language string) {
	otherIncome := getNumFrom(data["OtherIncome"])
	otherExpenses := getNumFrom(data["OtherExpenses"])

	otherIncomeItems, _ := data["other_income_items"].([]interface{})
	otherExpenseItems, _ := data["other_expense_items"].([]interface{})

	if otherIncome == 0 && otherExpenses == 0 && len(otherIncomeItems) == 0 && len(otherExpenseItems) == 0 {
		return
	}

	// Section header
	pdf.SetFont("Arial", "B", 11)
	pdf.SetFillColor(41, 65, 122)
	pdf.SetTextColor(255, 255, 255)
	pdf.CellFormat(180, 7, "  "+p.formatSSOTSectionTitle(utils.T("other_income_expenses", language), utils.T("non_operating_tag", language)), "1", 0, "L", true, 0, "")
	pdf.Ln(7)
	pdf.SetTextColor(0, 0, 0)

	pdf.SetFont("Arial", "", 9)
	if otherIncome != 0 || len(otherIncomeItems) > 0 {
		pdf.SetFillColor(255, 255, 255)
		pdf.CellFormat(130, 6, "    "+utils.T("other_income", language), "LB", 0, "L", true, 0, "")
		pdf.CellFormat(50, 6, p.formatRupiah(otherIncome), "RB", 0, "R", true, 0, "")
		pdf.Ln(6)
		if len(otherIncomeItems) > 0 {
			p.addAccountDetails(pdf, utils.T("other_income_accounts", language), otherIncomeItems)
		}
	}

	if otherExpenses != 0 || len(otherExpenseItems) > 0 {
		pdf.SetFillColor(252, 252, 252)
		pdf.CellFormat(130, 6, "    "+utils.T("other_expenses", language), "LB", 0, "L", true, 0, "")
		pdf.CellFormat(50, 6, p.formatRupiah(-otherExpenses), "RB", 0, "R", true, 0, "")
		pdf.Ln(6)
		if len(otherExpenseItems) > 0 {
			p.addAccountDetails(pdf, utils.T("other_expense_accounts", language), otherExpenseItems)
		}
	}

	p.addSSOTTotalLine(pdf, utils.T("net_other_income_expenses", language), otherIncome-otherExpenses, utils.T("non_operating_tag", language))
}

// addFinancialRatiosSummary adds a professional financial ratios card
func (p *PDFService) addFinancialRatiosSummary(pdf *gofpdf.Fpdf, data map[string]interface{}, language string) {
	pdf.Ln(8)

	// Section header
	pdf.SetFont("Arial", "B", 11)
	pdf.SetFillColor(41, 65, 122)
	pdf.SetTextColor(255, 255, 255)
	pdf.CellFormat(180, 7, "  "+utils.T("financial_ratios_summary", language), "1", 0, "L", true, 0, "")
	pdf.Ln(7)
	pdf.SetTextColor(0, 0, 0)

	// Ratio helper
	fmtRatio := func(label string, data map[string]interface{}, key string) string {
		if val, exists := data[key]; exists {
			if v, ok := val.(float64); ok && v > 0 {
				return fmt.Sprintf("%s: %.1f%%", label, v)
			}
		}
		return fmt.Sprintf("%s: -", label)
	}

	pdf.SetFont("Arial", "", 9)
	pdf.SetDrawColor(220, 220, 220)

	// Row 1
	pdf.SetFillColor(248, 250, 252)
	r1c1 := fmtRatio(utils.T("gross_profit_margin", language), data, "GrossProfitMargin")
	r1c2 := fmtRatio(utils.T("operating_margin", language), data, "OperatingMargin")
	pdf.CellFormat(90, 6, "  "+r1c1, "LB", 0, "L", true, 0, "")
	pdf.CellFormat(90, 6, "  "+r1c2, "RB", 0, "L", true, 0, "")
	pdf.Ln(6)

	// Row 2
	pdf.SetFillColor(255, 255, 255)
	r2c1 := fmtRatio(utils.T("ebitda_margin", language), data, "EBITDAMargin")
	r2c2 := fmtRatio(utils.T("net_income_margin", language), data, "NetIncomeMargin")
	pdf.CellFormat(90, 6, "  "+r2c1, "LB", 0, "L", true, 0, "")
	pdf.CellFormat(90, 6, "  "+r2c2, "RB", 0, "L", true, 0, "")
	pdf.Ln(8)
}

// addAccountDetails adds account detail rows with alternating stripes
func (p *PDFService) addAccountDetails(pdf *gofpdf.Fpdf, title string, items []interface{}) {
	if len(items) == 0 {
		return
	}

	// Sub-header
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(245, 247, 250)
	pdf.SetTextColor(60, 60, 60)
	pdf.CellFormat(180, 5, "    "+title+":", "LR", 0, "L", true, 0, "")
	pdf.Ln(5)
	pdf.SetTextColor(0, 0, 0)

	pdf.SetFont("Arial", "", 8)

	for i, item := range items {
		if itemMap, ok := item.(map[string]interface{}); ok {
			accountCode := ""
			accountName := "Unknown Account"
			amount := 0.0

			// Try different key formats (snake_case and PascalCase)
			if code, exists := itemMap["account_code"]; exists {
				if codeStr, ok := code.(string); ok {
					accountCode = codeStr
				}
			} else if code, exists := itemMap["AccountCode"]; exists {
				if codeStr, ok := code.(string); ok {
					accountCode = codeStr
				}
			}

			if name, exists := itemMap["account_name"]; exists {
				if nameStr, ok := name.(string); ok {
					accountName = nameStr
				}
			} else if name, exists := itemMap["AccountName"]; exists {
				if nameStr, ok := name.(string); ok {
					accountName = nameStr
				}
			}

			if amt, exists := itemMap["amount"]; exists {
				amount = getNumFrom(amt)
			} else if amt, exists := itemMap["Amount"]; exists {
				amount = getNumFrom(amt)
			}

			// Alternating row colors
			if i%2 == 0 {
				pdf.SetFillColor(255, 255, 255)
			} else {
				pdf.SetFillColor(250, 251, 253)
			}

			pdf.CellFormat(25, 5, "      "+accountCode, "L", 0, "L", true, 0, "")
			pdf.CellFormat(105, 5, accountName, "", 0, "L", true, 0, "")
			pdf.CellFormat(50, 5, p.formatRupiah(amount), "R", 0, "R", true, 0, "")
			pdf.Ln(5)
		}
	}

	// Bottom border
	pdf.SetDrawColor(220, 220, 220)
	x := pdf.GetX()
	y := pdf.GetY()
	pdf.Line(x, y, x+180, y)
	pdf.Ln(1)
}

// GenerateSSOTProfitLossPDF generates a PDF for SSOT-based Profit & Loss report
func (p *PDFService) GenerateSSOTProfitLossPDF(ssotData interface{}, language string) ([]byte, error) {
	utils.SanitizeExportData(ssotData)
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// Layout helpers
	lm, _, _, _ := pdf.GetMargins()
	pageW, _ := pdf.GetPageSize()
	rm := float64(15) // match margin
	contentW := pageW - lm - rm

	// Company info
	companyInfo, err := p.getCompanyInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get company info: %v", err)
	}

	// Unified letterhead: logo left, company info right, separator
	contentY := drawUnifiedLetterhead(pdf, companyInfo, 35)

	// Title
	pdf.SetY(contentY)
	pdf.SetX(lm)
	pdf.SetFont("Arial", "B", 20)
	pdf.SetTextColor(25, 55, 109) // dark navy, consistent with letterhead
	pdf.Cell(contentW, 10, utils.T("profit_loss_statement", language))
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(12)

	// Normalize SSOT data (struct -> map) so downstream renderer can read snake_case keys
	var dataMap map[string]interface{}
	if m, ok := ssotData.(map[string]interface{}); ok {
		dataMap = m
	} else {
		b, _ := json.Marshal(ssotData)
		_ = json.Unmarshal(b, &dataMap)
	}

	// Period info
	periodText := ""
	if dataMap != nil {
		if sd, ok := dataMap["start_date"]; ok {
			if sdStr, ok := sd.(string); ok && len(sdStr) >= 10 {
				periodText = sdStr[:10]
			}
		}
		if ed, ok := dataMap["end_date"]; ok {
			if edStr, ok := ed.(string); ok && len(edStr) >= 10 {
				if periodText != "" {
					periodText += "  -  " + edStr[:10]
				}
			}
		}
	}
	if periodText != "" {
		pdf.SetFont("Arial", "", 9)
		pdf.SetTextColor(100, 100, 100)
		pdf.Cell(contentW, 5, periodText)
		pdf.Ln(8)
		pdf.SetTextColor(0, 0, 0)
	}

	p.addSSOTSectionMappingLegend(pdf, language)

	// Table column headers
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(60, 60, 60)
	pdf.SetTextColor(255, 255, 255)
	keteranganLabel := "KETERANGAN"
	jumlahLabel := "JUMLAH"
	if language == "en" {
		keteranganLabel = "DESCRIPTION"
		jumlahLabel = "AMOUNT"
	}
	pdf.CellFormat(130, 6, "  "+keteranganLabel, "1", 0, "L", true, 0, "")
	pdf.CellFormat(50, 6, jumlahLabel+"  ", "1", 0, "R", true, 0, "")
	pdf.Ln(6)
	pdf.SetTextColor(0, 0, 0)
	if dataMap == nil {
		dataMap = map[string]interface{}{}
	}
	p.renderSSOTFinancialSections(pdf, dataMap, language)

	pdf.Ln(10)
	pdf.SetFont("Arial", "I", 8)
	pdf.Cell(190, 4, fmt.Sprintf("%s %s", utils.T("report_generated_on", language), time.Now().Format("02/01/2006 15:04")))

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to generate SSOT Profit & Loss PDF: %v", err)
	}
	return buf.Bytes(), nil
}

// addPlaceholderSections adds placeholder sections when data extraction fails
func (p *PDFService) addPlaceholderSections(pdf *gofpdf.Fpdf) {
	// Add some example sections to show the structure
	sections := []string{"REVENUE", "COST OF GOODS SOLD", "GROSS PROFIT", "OPERATING EXPENSES", "OPERATING INCOME", "NET INCOME"}

	for _, section := range sections {
		pdf.SetFont("Arial", "B", 10)
		pdf.SetFillColor(240, 240, 240)
		pdf.CellFormat(140, 6, section, "1", 0, "L", true, 0, "")
		pdf.CellFormat(50, 6, "Processing...", "1", 0, "R", true, 0, "")
		pdf.Ln(6)
	}
}

// addProfitLossSections adds P&L sections to PDF
func (p *PDFService) addProfitLossSections(pdf *gofpdf.Fpdf, plData map[string]interface{}) {
	pdf.SetFont("Arial", "B", 12)

	// Process sections if they exist
	if sections, exists := plData["sections"]; exists {
		if sectionsSlice, ok := sections.([]interface{}); ok {
			for _, section := range sectionsSlice {
				if sectionMap, ok := section.(map[string]interface{}); ok {
					p.addPLSection(pdf, sectionMap)
					pdf.Ln(3)
				}
			}
		}
	}

	// Add financial metrics summary
	if metrics, exists := plData["financialMetrics"]; exists {
		p.addFinancialMetrics(pdf, metrics)
	}
}

// addPLSection adds a P&L section to PDF
func (p *PDFService) addPLSection(pdf *gofpdf.Fpdf, section map[string]interface{}) {
	sectionName := "Unknown Section"
	sectionTotal := 0.0

	if name, exists := section["name"]; exists {
		if nameStr, ok := name.(string); ok {
			sectionName = nameStr
		}
	}

	if total, exists := section["total"]; exists {
		if totalFloat, ok := total.(float64); ok {
			sectionTotal = totalFloat
		}
	}

	// Section header
	pdf.SetFont("Arial", "B", 11)
	pdf.Cell(190, 8, sectionName)
	pdf.Ln(8)

	// Add section items
	if items, exists := section["items"]; exists {
		p.addPLSectionItems(pdf, items)
	}

	// Add subsections if they exist
	if subsections, exists := section["subsections"]; exists {
		if subsectionsSlice, ok := subsections.([]interface{}); ok {
			for _, subsection := range subsectionsSlice {
				if subsectionMap, ok := subsection.(map[string]interface{}); ok {
					p.addPLSubsection(pdf, subsectionMap)
				}
			}
		}
	}

	// Section total
	if !section["is_calculated"].(bool) {
		pdf.SetFont("Arial", "B", 10)
		pdf.SetFillColor(245, 245, 245)
		pdf.CellFormat(145, 6, fmt.Sprintf("Total %s", sectionName), "1", 0, "L", true, 0, "")
		pdf.CellFormat(45, 6, p.formatRupiah(sectionTotal), "1", 0, "R", true, 0, "")
		pdf.Ln(6)
	} else {
		// For calculated sections like Net Income, show with different formatting
		pdf.SetFont("Arial", "B", 11)
		pdf.SetFillColor(230, 230, 230)
		pdf.CellFormat(145, 8, sectionName, "1", 0, "L", true, 0, "")
		pdf.CellFormat(45, 8, p.formatRupiah(sectionTotal), "1", 0, "R", true, 0, "")
		pdf.Ln(8)
	}
}

// addPLSectionItems adds section items to PDF
func (p *PDFService) addPLSectionItems(pdf *gofpdf.Fpdf, items interface{}) {
	pdf.SetFont("Arial", "", 9)

	if itemsSlice, ok := items.([]interface{}); ok {
		for _, item := range itemsSlice {
			if itemMap, ok := item.(map[string]interface{}); ok {
				name := "Unknown Item"
				amount := 0.0
				isPercentage := false

				if nameVal, exists := itemMap["name"]; exists {
					if nameStr, ok := nameVal.(string); ok {
						name = nameStr
					}
				}

				if amountVal, exists := itemMap["amount"]; exists {
					if amountFloat, ok := amountVal.(float64); ok {
						amount = amountFloat
					}
				}

				if isPercentageVal, exists := itemMap["is_percentage"]; exists {
					if isPercentageBool, ok := isPercentageVal.(bool); ok {
						isPercentage = isPercentageBool
					}
				}

				pdf.Cell(145, 5, fmt.Sprintf("  %s", name))
				if isPercentage {
					pdf.Cell(45, 5, fmt.Sprintf("%.3f%%", amount))
				} else {
					pdf.Cell(45, 5, p.formatRupiah(amount))
				}
				pdf.Ln(5)
			}
		}
	}
}

// addPLSubsection adds a P&L subsection to PDF
func (p *PDFService) addPLSubsection(pdf *gofpdf.Fpdf, subsection map[string]interface{}) {
	subsectionName := "Unknown Subsection"
	subsectionTotal := 0.0

	if name, exists := subsection["name"]; exists {
		if nameStr, ok := name.(string); ok {
			subsectionName = nameStr
		}
	}

	if total, exists := subsection["total"]; exists {
		if totalFloat, ok := total.(float64); ok {
			subsectionTotal = totalFloat
		}
	}

	// Subsection header
	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(190, 6, fmt.Sprintf("  %s", subsectionName))
	pdf.Ln(6)

	// Add subsection items
	if items, exists := subsection["items"]; exists {
		if itemsSlice, ok := items.([]interface{}); ok {
			pdf.SetFont("Arial", "", 9)
			for _, item := range itemsSlice {
				if itemMap, ok := item.(map[string]interface{}); ok {
					name := "Unknown Item"
					amount := 0.0

					if nameVal, exists := itemMap["name"]; exists {
						if nameStr, ok := nameVal.(string); ok {
							name = nameStr
						}
					}

					if amountVal, exists := itemMap["amount"]; exists {
						if amountFloat, ok := amountVal.(float64); ok {
							amount = amountFloat
						}
					}

					pdf.Cell(145, 4, fmt.Sprintf("    %s", name))
					pdf.Cell(45, 4, p.formatRupiah(amount))
					pdf.Ln(4)
				}
			}
		}
	}

	// Subsection total
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(248, 248, 248)
	pdf.CellFormat(145, 5, fmt.Sprintf("  Total %s", subsectionName), "1", 0, "L", true, 0, "")
	pdf.CellFormat(45, 5, p.formatRupiah(subsectionTotal), "1", 0, "R", true, 0, "")
	pdf.Ln(5)
}

// addFinancialMetrics adds financial metrics summary to PDF
func (p *PDFService) addFinancialMetrics(pdf *gofpdf.Fpdf, metrics interface{}) {
	pdf.Ln(10)
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(190, 8, "FINANCIAL METRICS SUMMARY")
	pdf.Ln(10)

	if metricsMap, ok := metrics.(map[string]interface{}); ok {
		pdf.SetFont("Arial", "", 9)

		// Display key metrics
		metricItems := []string{
			"grossProfit", "grossProfitMargin", "operatingIncome",
			"operatingMargin", "netIncome", "netIncomeMargin",
		}

		metricLabels := map[string]string{
			"grossProfit":       "Gross Profit",
			"grossProfitMargin": "Gross Profit Margin",
			"operatingIncome":   "Operating Income",
			"operatingMargin":   "Operating Margin",
			"netIncome":         "Net Income",
			"netIncomeMargin":   "Net Income Margin",
		}

		for _, metricKey := range metricItems {
			if value, exists := metricsMap[metricKey]; exists {
				label := metricLabels[metricKey]
				if valueFloat, ok := value.(float64); ok {
					pdf.Cell(95, 5, label+":")
					if strings.Contains(metricKey, "Margin") {
						pdf.Cell(95, 5, fmt.Sprintf("%.3f%%", valueFloat))
					} else {
						pdf.Cell(95, 5, p.formatRupiah(valueFloat))
					}
					pdf.Ln(5)
				}
			}
		}
	}
}
