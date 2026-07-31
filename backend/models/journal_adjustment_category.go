package models

import "strings"

const (
	JournalAdjustmentCategoryPPh21Accrual              = "PPH21_ACCRUAL"
	JournalAdjustmentCategoryPPh23Accrual              = "PPH23_ACCRUAL"
	JournalAdjustmentCategorySalesRevenueRecognition   = "SALES_REVENUE_RECOGNITION"
	JournalAdjustmentCategoryCOGSHPPRecognition        = "COGS_HPP_RECOGNITION"
	JournalAdjustmentCategoryPrepaidMonthlyRealization = "PREPAID_MONTHLY_REALIZATION"
	JournalAdjustmentCategoryPostedCOAReclassification = "POSTED_COA_RECLASSIFICATION"

	// Backward-compatible aliases for code paths created before the client
	// narrowed the adjustment scope to the five approved categories.
	JournalAdjustmentCategorySalesPPh23Settlement = JournalAdjustmentCategoryPPh23Accrual
	JournalAdjustmentCategorySalesBackorderHPP    = JournalAdjustmentCategoryCOGSHPPRecognition
)

const JournalAdjustmentRequirementTagline = "Jurnal ini masuk daftar khusus transaksi yang membutuhkan adjustment sesuai mapping client."

type JournalAdjustmentCategoryDefinition struct {
	Code                    string   `json:"code"`
	Label                   string   `json:"label"`
	Description             string   `json:"description"`
	COA                     string   `json:"coa"`
	RecommendedSourceTypes  []string `json:"recommended_source_types"`
	Examples                []string `json:"examples"`
	RequiresOriginalJournal bool     `json:"requires_original_journal"`
}

func NormalizeJournalAdjustmentCategoryCode(code string) string {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	switch normalized {
	case "SALES_PPH23_SETTLEMENT", "TAX_UNDER_OVER_PAYMENT":
		return JournalAdjustmentCategoryPPh23Accrual
	case "SALES_BACKORDER_HPP", "PURCHASE_FREIGHT_INVENTORY", "INVENTORY_STOCK_ADJUSTMENT":
		return JournalAdjustmentCategoryCOGSHPPRecognition
	case "PAYROLL_TAX_ACCRUAL":
		return JournalAdjustmentCategoryPPh21Accrual
	case "PREPAID_SCHEDULE_CORRECTION":
		return JournalAdjustmentCategoryPrepaidMonthlyRealization
	default:
		return normalized
	}
}

func GetJournalAdjustmentCategories() []JournalAdjustmentCategoryDefinition {
	return []JournalAdjustmentCategoryDefinition{
		{
			Code:                   JournalAdjustmentCategoryPPh21Accrual,
			Label:                  "Koreksi Selisih PPh 21",
			Description:            "Adjustment hanya untuk selisih antara PPh 21 payroll yang sudah diakui dan nilai final. Pembayaran PPh bukan adjustment.",
			COA:                    "Utang PPh 21 & Biaya PPh 21",
			RecommendedSourceTypes: []string{"PAYROLL", "PAYROLL_ACCRUAL", "PAYROLL_PPH_PAYABLE", SSOTSourceTypeManual},
			Examples: []string{
				"Tambahan beban dan utang saat nilai final lebih besar",
				"Reversal beban dan utang saat nilai final lebih kecil",
			},
			RequiresOriginalJournal: true,
		},
		{
			Code:                   JournalAdjustmentCategoryPPh23Accrual,
			Label:                  "Pembebanan PPh 23",
			Description:            "Accrued Utang PPh 23 dan Beban Pajak PPh 23 di bulan terkait.",
			COA:                    "Utang PPh 23 & Biaya PPh 23",
			RecommendedSourceTypes: []string{SSOTSourceTypeSale, SSOTSourceTypeManual},
			Examples: []string{
				"PPh 23 menyusul setelah invoice sales dibayar",
				"Kurang bayar PPh 23 yang diinformasikan belakangan",
			},
			RequiresOriginalJournal: true,
		},
		{
			Code:                   JournalAdjustmentCategorySalesRevenueRecognition,
			Label:                  "Pengakuan Pendapatan Sales",
			Description:            "Pengakuan pendapatan sales sebelum realisasi pemasukan uangnya diterima di bulan terkait.",
			COA:                    "Piutang Usaha",
			RecommendedSourceTypes: []string{SSOTSourceTypeSale},
			Examples: []string{
				"Invoice sales sudah diakui tetapi kas belum diterima",
				"Reclass pengakuan revenue ke piutang usaha",
			},
			RequiresOriginalJournal: true,
		},
		{
			Code:                   JournalAdjustmentCategoryCOGSHPPRecognition,
			Label:                  "Pengakuan COGS/HPP",
			Description:            "Pengakuan COGS/HPP di bulan selanjutnya jika sales di bulan selanjutnya walaupun pembayaran COGS terjadi di bulan sebelumnya.",
			COA:                    "Inventory / Release Accrued COGS / HPP",
			RecommendedSourceTypes: []string{SSOTSourceTypeSale, SSOTSourceTypePurchase},
			Examples: []string{
				"Release accrued HPP dari backorder sales",
				"Koreksi HPP saat inventory/purchase aktual sudah tersedia",
			},
			RequiresOriginalJournal: true,
		},
		{
			Code:                   JournalAdjustmentCategoryPrepaidMonthlyRealization,
			Label:                  "Realisasi Prepaid Bulanan",
			Description:            "Realisasi prepaid per bulan, sama seperti depresiasi fixed asset.",
			COA:                    "Beban & Prepaid",
			RecommendedSourceTypes: []string{SSOTSourceTypePrepaidAmortization, SSOTSourceTypePrepaidTermination, SSOTSourceTypeAsset, SSOTSourceTypeDepreciation},
			Examples: []string{
				"Amortisasi prepaid expense bulanan",
				"Realisasi beban dari saldo prepaid",
			},
			RequiresOriginalJournal: true,
		},
		{
			Code:                   JournalAdjustmentCategoryPostedCOAReclassification,
			Label:                  "Reklasifikasi COA Transaksi Terposting",
			Description:            "Memindahkan satu atau beberapa akun non kas/bank pada jurnal pembelian yang sudah terposting ke COA yang benar tanpa mengubah jurnal asal.",
			COA:                    "COA asal & COA pengganti",
			RecommendedSourceTypes: []string{SSOTSourceTypePurchase},
			Examples: []string{
				"Reklasifikasi beban pembelian yang salah akun",
				"Pemindahan piutang atau utang antar akun yang benar",
			},
			RequiresOriginalJournal: true,
		},
	}
}

func GetJournalAdjustmentCategory(code string) (JournalAdjustmentCategoryDefinition, bool) {
	normalized := NormalizeJournalAdjustmentCategoryCode(code)
	if normalized == "" {
		return JournalAdjustmentCategoryDefinition{}, false
	}

	for _, category := range GetJournalAdjustmentCategories() {
		if category.Code == normalized {
			return category, true
		}
	}

	return JournalAdjustmentCategoryDefinition{}, false
}
