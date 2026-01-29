package utils

import (
	"fmt"
	"log"
	"sync"

	"app-sistem-akuntansi/models"
	"gorm.io/gorm"
)

// TranslationMap holds translations for both languages
type TranslationMap map[string]map[string]string

var (
	translationsCache TranslationMap
	translationsMutex sync.RWMutex
	translationsLoaded = false
)

// PDFTranslations contains all translations needed for PDF generation
var PDFTranslations = TranslationMap{
	"id": {
		// Common translations
		"company":           "Perusahaan",
		"address":           "Alamat",
		"phone":             "Telepon",
		"email":             "Email",
		"generated_on":      "Dibuat pada",
		"page":              "Halaman",
		"of":                "dari",
		"total":             "Total",
		"subtotal":          "Subtotal",
		"date":              "Tanggal",
		"amount":            "Jumlah",
		"description":       "Deskripsi",
		"status":            "Status",
		"active":            "Aktif",
		"inactive":          "Tidak Aktif",
		"pending":           "Menunggu",
		"completed":         "Selesai",
		"approved":          "Disetujui",
		"rejected":          "Ditolak",
		"paid":              "Lunas",
		"unpaid":            "Belum Lunas",
		"partial":           "Sebagian",

		// Report specific translations
		"chart_of_accounts":           "Daftar Akun",
		"account_code":               "Kode Akun",
		"account_name":               "Nama Akun",
		"account_type":               "Tipe Akun",
		"balance":                    "Saldo",
		"category":                   "Kategori",
		"created_at":                 "Dibuat pada",

		// Cash Flow translations
		"cash_flow_statement":        "Laporan Arus Kas",
		"operating_activities":       "AKTIVITAS OPERASIONAL",
		"investing_activities":       "AKTIVITAS INVESTASI", 
		"financing_activities":       "AKTIVITAS PENDANAAN",
		"net_income":                "Laba Bersih",
		"adjustments_non_cash":      "Penyesuaian untuk Item Non-Kas",
		"working_capital_changes":   "Perubahan Modal Kerja",
		"net_cash_operating":        "KAS BERSIH DARI AKTIVITAS OPERASIONAL",
		"net_cash_investing":        "KAS BERSIH DARI AKTIVITAS INVESTASI",
		"net_cash_financing":        "KAS BERSIH DARI AKTIVITAS PENDANAAN",
		"cash_beginning":            "Kas di Awal Periode",
		"net_cash_flow":             "Arus Kas Bersih",
		"cash_ending":               "Kas di Akhir Periode",
		"cash_flow_summary":         "RINGKASAN ARUS KAS",
		"period":                    "Periode",
		"to":                        "sampai",

		// Purchase Report translations
		"purchase_report":           "LAPORAN PEMBELIAN",
		"summary":                   "RINGKASAN",
		"total_purchases":           "Total Pembelian",
		"completed_purchases":       "Pembelian Selesai",
		"total_amount":              "Total Jumlah",
		"total_paid":                "Total Dibayar",
		"outstanding_payables":      "Hutang Tertunggak",
		"top_vendors":               "Vendor Teratas",
		"vendor":                    "Vendor",
		"purchases":                 "Pembelian",
		"outstanding":               "Tertunggak",
		"last_purchase":             "Pembelian Terakhir",
		"payment_method":            "Metode Pembayaran",
		"purchases_by_vendor":       "Pembelian Per Vendor (20 Teratas)",
		"items_purchased":           "Barang Dibeli",
		"vendor_id":                 "ID Vendor",
		"vendor_name":               "Nama Vendor",
		"orders":                    "Pesanan",
		"product":                   "Produk",
		"product_code":              "Kode Produk",
		"product_name":              "Nama Produk",
		"qty":                       "Jumlah",
		"unit_price":                "Harga Satuan",
		"unit":                      "Satuan",

		// Balance Sheet translations
		"balance_sheet":             "NERACA",
		"assets":                    "ASET",
		"current_assets":            "Aset Lancar",
		"fixed_assets":              "Aset Tetap",
		"total_assets":              "TOTAL ASET",
		"liabilities":               "KEWAJIBAN",
		"current_liabilities":       "Kewajiban Jangka Pendek",
		"long_term_liabilities":     "Kewajiban Jangka Panjang",
		"total_liabilities":         "TOTAL KEWAJIBAN",
		"equity":                    "EKUITAS",
		"total_equity":              "TOTAL EKUITAS",
		"total_liabilities_equity":  "TOTAL KEWAJIBAN DAN EKUITAS",

		// Profit & Loss translations
		"profit_loss_statement":     "LAPORAN LABA RUGI",
		"enhanced_profit_loss":      "Laporan Laba Rugi yang Ditingkatkan",
		"revenue":                   "PENDAPATAN",
		"total_revenue":             "Total Pendapatan",
		"cost_of_goods_sold":        "HARGA POKOK PENJUALAN",
		"gross_profit":              "LABA KOTOR",
		"gross_profit_label":        "Laba Kotor",
		"gross_profit_margin":       "Margin Laba Kotor",
		"operating_expenses":        "BIAYA OPERASIONAL",
		"total_operating_expenses":  "Total Biaya Operasional",
		"operating_profit":          "LABA OPERASIONAL",
		"operating_income":          "LABA OPERASIONAL",
		"operating_income_label":    "Laba Operasional",
		"operating_margin":          "Margin Operasional",
		"other_income":              "PENDAPATAN LAIN-LAIN",
		"other_expenses":            "BIAYA LAIN-LAIN",
		"other_income_expenses":     "PENDAPATAN/BEBAN LAIN-LAIN",
		"net_profit":                "LABA BERSIH",
		"net_income_label":          "Laba Bersih",
		"net_income_margin":         "Margin Laba Bersih",
		"income_before_tax":         "Laba Sebelum Pajak",
		"tax_expense":               "Beban Pajak",
		"direct_materials":          "Bahan Baku Langsung",
		"direct_labor":              "Tenaga Kerja Langsung",
		"manufacturing_overhead":    "Overhead Pabrik",
		"other_cogs":                "HPP Lainnya",
		"administrative_expenses":   "Beban Administrasi",
		"selling_marketing":         "Beban Penjualan & Pemasaran",
		"general_expenses":          "Beban Umum",
		"sales_revenue":             "Pendapatan Penjualan",
		"key_financial_metrics":     "Metrik Keuangan Utama",
		"detailed_breakdown":        "Rincian Detail",

		// General Ledger translations
		"general_ledger":            "BUKU BESAR",
		"account":                   "Akun",
		"debit":                     "Debit",
		"credit":                    "Kredit",
		"running_balance":           "Saldo Berjalan",
		"transaction_date":          "Tanggal Transaksi",
		"reference":                 "Referensi",

		// Trial Balance translations
		"trial_balance":             "NERACA SALDO",
		"opening_balance":           "Saldo Awal",
		"closing_balance":           "Saldo Akhir",
		"total_debit":               "Total Debit",
		"total_credit":              "Total Kredit",

		// Sales Report translations
		"sales_report":              "LAPORAN PENJUALAN",
		"sales_summary":             "RINGKASAN PENJUALAN",
		"customer":                  "Pelanggan",
		"customer_name":             "Nama Pelanggan",
		"invoice_number":            "Nomor Faktur",
		"due_date":                  "Tanggal Jatuh Tempo",
		"payment_status":            "Status Pembayaran",
		"sales_by_customer":         "Penjualan Per Pelanggan",
		"items_sold":                "Barang Terjual",
		"total_sales":               "Total Penjualan",
		"transactions":              "Transaksi",
		"total_transactions":        "Total Transaksi",
		"average_order":             "Rata-rata Order",
		"average_order_value":       "Nilai Rata-rata Order",
		"total_customers":           "Total Pelanggan",

		// Payment Report translations
		"payment_report":            "LAPORAN PEMBAYARAN",
		"payment_number":            "Nomor Pembayaran",
		"payment_date":              "Tanggal Pembayaran",
		"contact":                   "Kontak",
		"method":                    "Metode",

		// Receipt translations
		"receipt":                   "KWITANSI",
		"received_from":             "Sudah Terima Dari",
		"amount_in_words":           "Banyaknya Uang",
		"for_payment":               "Untuk Pembayaran",
		"amount_rp":                 "Jumlah Rp.",
		"received_by":               "Diterima oleh",

		// Error messages
		"no_data_available":         "Tidak ada data tersedia",
		"report_generation_error":   "Terjadi kesalahan dalam pembuatan laporan",
	},

	"en": {
		// Common translations
		"company":           "Company",
		"address":           "Address",
		"phone":             "Phone",
		"email":             "Email",
		"generated_on":      "Generated on",
		"page":              "Page",
		"of":                "of",
		"total":             "Total",
		"subtotal":          "Subtotal",
		"date":              "Date",
		"amount":            "Amount",
		"description":       "Description",
		"status":            "Status",
		"active":            "Active",
		"inactive":          "Inactive",
		"pending":           "Pending",
		"completed":         "Completed",
		"approved":          "Approved",
		"rejected":          "Rejected",
		"paid":              "Paid",
		"unpaid":            "Unpaid",
		"partial":           "Partial",

		// Report specific translations
		"chart_of_accounts":           "Chart of Accounts",
		"account_code":               "Account Code",
		"account_name":               "Account Name",
		"account_type":               "Account Type",
		"balance":                    "Balance",
		"category":                   "Category",
		"created_at":                 "Created At",

		// Cash Flow translations
		"cash_flow_statement":        "Cash Flow Statement",
		"operating_activities":       "OPERATING ACTIVITIES",
		"investing_activities":       "INVESTING ACTIVITIES",
		"financing_activities":       "FINANCING ACTIVITIES",
		"net_income":                "Net Income",
		"adjustments_non_cash":      "Adjustments for Non-Cash Items",
		"working_capital_changes":   "Changes in Working Capital",
		"net_cash_operating":        "NET CASH FROM OPERATING ACTIVITIES",
		"net_cash_investing":        "NET CASH FROM INVESTING ACTIVITIES",
		"net_cash_financing":        "NET CASH FROM FINANCING ACTIVITIES",
		"cash_beginning":            "Cash at Beginning of Period",
		"net_cash_flow":             "Net Cash Flow",
		"cash_ending":               "Cash at End of Period",
		"cash_flow_summary":         "CASH FLOW SUMMARY",
		"period":                    "Period",
		"to":                        "to",

		// Purchase Report translations
		"purchase_report":           "PURCHASE REPORT",
		"summary":                   "SUMMARY",
		"total_purchases":           "Total Purchases",
		"completed_purchases":       "Completed Purchases",
		"total_amount":              "Total Amount",
		"total_paid":                "Total Paid",
		"outstanding_payables":      "Outstanding Payables",
		"top_vendors":               "Top Vendors",
		"vendor":                    "Vendor",
		"purchases":                 "Purchases",
		"outstanding":               "Outstanding",
		"last_purchase":             "Last Purchase",
		"payment_method":            "Payment Method",
		"purchases_by_vendor":       "Purchases By Vendor (Top 20)",
		"items_purchased":           "Items Purchased",
		"vendor_id":                 "Vendor ID",
		"vendor_name":               "Vendor Name",
		"orders":                    "Orders",
		"product":                   "Product",
		"product_code":              "Product Code",
		"product_name":              "Product Name",
		"qty":                       "Qty",
		"unit_price":                "Unit Price",
		"unit":                      "Unit",

		// Balance Sheet translations
		"balance_sheet":             "BALANCE SHEET",
		"assets":                    "ASSETS",
		"current_assets":            "Current Assets",
		"fixed_assets":              "Fixed Assets",
		"total_assets":              "TOTAL ASSETS",
		"liabilities":               "LIABILITIES",
		"current_liabilities":       "Current Liabilities",
		"long_term_liabilities":     "Long-term Liabilities",
		"total_liabilities":         "TOTAL LIABILITIES",
		"equity":                    "EQUITY",
		"total_equity":              "TOTAL EQUITY",
		"total_liabilities_equity":  "TOTAL LIABILITIES AND EQUITY",

		// Profit & Loss translations
		"profit_loss_statement":     "PROFIT & LOSS STATEMENT",
		"enhanced_profit_loss":      "Enhanced Profit and Loss Statement",
		"revenue":                   "REVENUE",
		"total_revenue":             "Total Revenue",
		"cost_of_goods_sold":        "COST OF GOODS SOLD",
		"gross_profit":              "GROSS PROFIT",
		"gross_profit_label":        "Gross Profit",
		"gross_profit_margin":       "Gross Profit Margin",
		"operating_expenses":        "OPERATING EXPENSES",
		"total_operating_expenses":  "Total Operating Expenses",
		"operating_profit":          "OPERATING PROFIT",
		"operating_income":          "OPERATING INCOME",
		"operating_income_label":    "Operating Income",
		"operating_margin":          "Operating Margin",
		"other_income":              "OTHER INCOME",
		"other_expenses":            "OTHER EXPENSES",
		"other_income_expenses":     "OTHER INCOME/EXPENSES",
		"net_profit":                "NET PROFIT",
		"net_income_label":          "Net Income",
		"net_income_margin":         "Net Income Margin",
		"income_before_tax":         "Income Before Tax",
		"tax_expense":               "Tax Expense",
		"direct_materials":          "Direct Materials",
		"direct_labor":              "Direct Labor",
		"manufacturing_overhead":    "Manufacturing Overhead",
		"other_cogs":                "Other COGS",
		"administrative_expenses":   "Administrative Expenses",
		"selling_marketing":         "Selling & Marketing",
		"general_expenses":          "General Expenses",
		"sales_revenue":             "Sales Revenue",
		"key_financial_metrics":     "Key Financial Metrics",
		"detailed_breakdown":        "Detailed Breakdown",

		// General Ledger translations
		"general_ledger":            "GENERAL LEDGER",
		"account":                   "Account",
		"debit":                     "Debit",
		"credit":                    "Credit",
		"running_balance":           "Running Balance",
		"transaction_date":          "Transaction Date",
		"reference":                 "Reference",

		// Trial Balance translations
		"trial_balance":             "TRIAL BALANCE",
		"opening_balance":           "Opening Balance",
		"closing_balance":           "Closing Balance",
		"total_debit":               "Total Debit",
		"total_credit":              "Total Credit",

		// Sales Report translations
		"sales_report":              "SALES REPORT",
		"sales_summary":             "SALES SUMMARY",
		"customer":                  "Customer",
		"customer_name":             "Customer Name",
		"invoice_number":            "Invoice Number",
		"due_date":                  "Due Date",
		"payment_status":            "Payment Status",
		"sales_by_customer":         "Sales By Customer",
		"items_sold":                "Items Sold",
		"total_sales":               "Total Sales",
		"transactions":              "Transactions",
		"total_transactions":        "Total Transactions",
		"average_order":             "Average Order",
		"average_order_value":       "Average Order Value",
		"total_customers":           "Total Customers",

		// Payment Report translations
		"payment_report":            "PAYMENT REPORT",
		"payment_number":            "Payment Number",
		"payment_date":              "Payment Date",
		"contact":                   "Contact",
		"method":                    "Method",

		// Receipt translations
		"receipt":                   "RECEIPT",
		"received_from":             "Received From",
		"amount_in_words":           "Amount in Words",
		"for_payment":               "For Payment of",
		"amount_rp":                 "Amount Rp.",
		"received_by":               "Received by",

		// Error messages
		"no_data_available":         "No data available",
		"report_generation_error":   "Error occurred during report generation",
	},
}

// InitializeTranslations loads the translations into cache
func InitializeTranslations() {
	translationsMutex.Lock()
	defer translationsMutex.Unlock()

	if translationsLoaded {
		return
	}

	translationsCache = PDFTranslations
	translationsLoaded = true
}

// GetUserLanguageFromDB retrieves user's language setting from database
func GetUserLanguageFromDB(db *gorm.DB, userID uint) string {
	if db == nil {
		return "id" // Default to Indonesian
	}

	// User-specific language preference not implemented yet
	// Skip user-specific check for now

	// Fallback to system settings
	var settings models.Settings
	if err := db.First(&settings).Error; err == nil {
		if settings.Language != "" {
			return settings.Language
		}
	}

	// Default fallback
	return "id"
}

// GetUserLanguageFromSettings retrieves language from system settings
func GetUserLanguageFromSettings(db *gorm.DB) string {
	if db == nil {
		return "id" // Default to Indonesian
	}

	var settings models.Settings
	if err := db.First(&settings).Error; err == nil {
		if settings.Language != "" {
			return settings.Language
		}
	}

	// Default fallback
	return "id"
}

// T translates a key to the specified language
func T(key, language string) string {
	// Ensure translations are loaded
	if !translationsLoaded {
		InitializeTranslations()
	}

	translationsMutex.RLock()
	defer translationsMutex.RUnlock()

	// Validate language
	if language != "id" && language != "en" {
		language = "id" // Default to Indonesian
	}

	// Get translation from cache
	if langMap, ok := translationsCache[language]; ok {
		if translation, ok := langMap[key]; ok {
			return translation
		}
	}

	// Fallback: try Indonesian if requested language not found
	if language != "id" {
		if langMap, ok := translationsCache["id"]; ok {
			if translation, ok := langMap[key]; ok {
				return translation
			}
		}
	}

	// Final fallback: return the key itself
	log.Printf("Translation not found for key '%s' in language '%s'", key, language)
	return key
}

// TWithLang is a helper function that gets user language from DB and translates
func TWithLang(db *gorm.DB, userID uint, key string) string {
	language := GetUserLanguageFromDB(db, userID)
	return T(key, language)
}

// TWithSettings is a helper function that gets language from settings and translates
func TWithSettings(db *gorm.DB, key string) string {
	language := GetUserLanguageFromSettings(db)
	return T(key, language)
}

// FormatCurrencyByLanguage formats currency based on user's language preference
func FormatCurrencyByLanguage(amount float64, language string) string {
	if language == "en" {
		return fmt.Sprintf("$%.2f", amount)
	}
	// Indonesian format
	return fmt.Sprintf("Rp %.2f", amount)
}

// FormatCurrencyWithDB formats currency based on database settings
func FormatCurrencyWithDB(db *gorm.DB, amount float64) string {
	language := GetUserLanguageFromSettings(db)
	return FormatCurrencyByLanguage(amount, language)
}

// FormatDate formats date based on user's language preference
func FormatDate(date interface{}, language string) string {
	// This can be extended based on date format preferences in settings
	// For now, using standard formats
	switch v := date.(type) {
	case string:
		return v
	default:
		return fmt.Sprintf("%v", date)
	}
}

// GetCSVHeaders returns CSV headers based on report type and language
func GetCSVHeaders(reportType, language string) []string {
	switch reportType {
	case "chart_of_accounts":
		return []string{
			T("account_code", language),
			T("account_name", language),
			T("account_type", language),
			T("category", language),
			T("balance", language),
			T("status", language),
			T("description", language),
			T("created_at", language),
		}
	case "cash_flow":
		return []string{
			T("activity_type", language),
			T("category", language),
			T("account_code", language),
			T("account_name", language),
			T("amount", language),
			T("type", language),
		}
	case "purchase_report":
		return []string{
			T("vendor_id", language),
			T("vendor_name", language),
			T("total_purchases", language),
			T("total_amount", language),
			T("total_paid", language),
			T("outstanding", language),
			T("last_purchase", language),
			T("payment_method", language),
			T("status", language),
		}
	default:
		return []string{T("no_data_available", language)}
	}
}

// AddActivityTypeTranslation adds activity type translation for CSV
func init() {
	// Add specific activity type translations directly
	// No need to call T() function here as it's not initialized yet
	PDFTranslations["id"]["activity_type"] = "Tipe Aktivitas"
	PDFTranslations["en"]["activity_type"] = "Activity Type"
	PDFTranslations["id"]["type"] = "Tipe"
	PDFTranslations["en"]["type"] = "Type"
}
