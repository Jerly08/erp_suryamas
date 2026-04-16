package database

import (
	"app-sistem-akuntansi/models"
	"strings"
)

// GetStandardCOA returns the list of accounts based on the company type
func GetStandardCOA(companyType string) []models.Account {
	// Base accounts common to all types (Assets, Liabilities, Equity basics)
	baseAccounts := []models.Account{
		// ASSETS (1xxx)
		{Code: "1000", Name: "ASET", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 1, IsHeader: true, IsActive: true, IsSystemCritical: true},
		{Code: "1100", Name: "ASET LANCAR", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 2, IsHeader: true, IsActive: true, IsSystemCritical: true},
		{Code: "1101", Name: "KAS DAN SETARA KAS", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 3, IsHeader: true, IsActive: true, IsSystemCritical: true},
		{Code: "1101-001", Name: "KAS KECIL", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 4, IsHeader: false, IsActive: true, IsSystemCritical: true},
		{Code: "1101-002", Name: "KAS BESAR", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 4, IsHeader: false, IsActive: true},
		{Code: "1101-003", Name: "BANK – GIRO", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 4, IsHeader: false, IsActive: true},
		{Code: "1101-004", Name: "BANK – TABUNGAN", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 4, IsHeader: false, IsActive: true},

		{Code: "1102", Name: "PIUTANG USAHA", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 3, IsHeader: true, IsActive: true, IsSystemCritical: true},
		{Code: "1102-001", Name: "PIUTANG USAHA – PIHAK KETIGA", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 4, IsHeader: false, IsActive: true, IsSystemCritical: true},
		{Code: "1102-002", Name: "CADANGAN KERUGIAN PIUTANG", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 4, IsHeader: false, IsActive: true},

		{Code: "1104", Name: "UANG MUKA & BIAYA DIBAYAR DIMUKA", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 3, IsHeader: true, IsActive: true},
		{Code: "1104-001", Name: "UANG MUKA PEMBELIAN", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 4, IsHeader: false, IsActive: true},
		{Code: "1104-002", Name: "SEWA DIBAYAR DIMUKA", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 4, IsHeader: false, IsActive: true},
		{Code: "1104-003", Name: "ASURANSI DIBAYAR DIMUKA", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 4, IsHeader: false, IsActive: true},

		{Code: "1105", Name: "PAJAK DIBAYAR DIMUKA", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 3, IsHeader: true, IsActive: true, IsSystemCritical: true},
		{Code: "1105-001", Name: "PPH 21 DIBAYAR DIMUKA", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 4, IsHeader: false, IsActive: true, IsSystemCritical: true},
		{Code: "1105-002", Name: "PPH 23 DIBAYAR DIMUKA", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 4, IsHeader: false, IsActive: true, IsSystemCritical: true},
		{Code: "1105-003", Name: "PPH FINAL DIBAYAR DIMUKA", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 4, IsHeader: false, IsActive: true},
		{Code: "1105-004", Name: "KREDIT PAJAK PPH 23", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 4, IsHeader: false, IsActive: true},

		{Code: "1200", Name: "ASET TIDAK LANCAR", Type: models.AccountTypeAsset, Category: models.CategoryFixedAsset, Level: 2, IsHeader: true, IsActive: true},
		{Code: "1201", Name: "ASET TETAP", Type: models.AccountTypeAsset, Category: models.CategoryFixedAsset, Level: 3, IsHeader: true, IsActive: true},
		{Code: "1201-001", Name: "TANAH", Type: models.AccountTypeAsset, Category: models.CategoryFixedAsset, Level: 4, IsHeader: false, IsActive: true},
		{Code: "1201-002", Name: "BANGUNAN", Type: models.AccountTypeAsset, Category: models.CategoryFixedAsset, Level: 4, IsHeader: false, IsActive: true},
		{Code: "1201-003", Name: "KENDARAAN", Type: models.AccountTypeAsset, Category: models.CategoryFixedAsset, Level: 4, IsHeader: false, IsActive: true},
		{Code: "1201-004", Name: "PERALATAN", Type: models.AccountTypeAsset, Category: models.CategoryFixedAsset, Level: 4, IsHeader: false, IsActive: true},
		{Code: "1201-005", Name: "MESIN", Type: models.AccountTypeAsset, Category: models.CategoryFixedAsset, Level: 4, IsHeader: false, IsActive: true},

		{Code: "1202", Name: "AKUMULASI PENYUSUTAN", Type: models.AccountTypeAsset, Category: models.CategoryFixedAsset, Level: 3, IsHeader: true, IsActive: true},
		{Code: "1202-002", Name: "AKUMULASI PENYUSUTAN BANGUNAN", Type: models.AccountTypeAsset, Category: models.CategoryFixedAsset, Level: 4, IsHeader: false, IsActive: true},
		{Code: "1202-003", Name: "AKUMULASI PENYUSUTAN KENDARAAN", Type: models.AccountTypeAsset, Category: models.CategoryFixedAsset, Level: 4, IsHeader: false, IsActive: true},
		{Code: "1202-004", Name: "AKUMULASI PENYUSUTAN PERALATAN", Type: models.AccountTypeAsset, Category: models.CategoryFixedAsset, Level: 4, IsHeader: false, IsActive: true},
		{Code: "1202-005", Name: "AKUMULASI PENYUSUTAN MESIN", Type: models.AccountTypeAsset, Category: models.CategoryFixedAsset, Level: 4, IsHeader: false, IsActive: true},

		// LIABILITIES (2xxx)
		{Code: "2000", Name: "LIABILITAS", Type: models.AccountTypeLiability, Category: models.CategoryCurrentLiability, Level: 1, IsHeader: true, IsActive: true, IsSystemCritical: true},
		{Code: "2100", Name: "LIABILITAS JANGKA PENDEK", Type: models.AccountTypeLiability, Category: models.CategoryCurrentLiability, Level: 2, IsHeader: true, IsActive: true, IsSystemCritical: true},
		{Code: "2101", Name: "UTANG USAHA", Type: models.AccountTypeLiability, Category: models.CategoryCurrentLiability, Level: 3, IsHeader: true, IsActive: true, IsSystemCritical: true},
		{Code: "2101-001", Name: "UTANG USAHA – LOKAL", Type: models.AccountTypeLiability, Category: models.CategoryCurrentLiability, Level: 4, IsHeader: false, IsActive: true, IsSystemCritical: true},

		{Code: "2102", Name: "UTANG PAJAK", Type: models.AccountTypeLiability, Category: models.CategoryCurrentLiability, Level: 3, IsHeader: true, IsActive: true},
		{Code: "2102-001", Name: "PPN KELUARAN", Type: models.AccountTypeLiability, Category: models.CategoryCurrentLiability, Level: 4, IsHeader: false, IsActive: true, IsSystemCritical: true},
		{Code: "2102-002", Name: "PPN MASUKAN", Type: models.AccountTypeLiability, Category: models.CategoryCurrentLiability, Level: 4, IsHeader: false, IsActive: true, IsSystemCritical: true}, // Note: Usually Contra-Asset or Liability with Debit balance, using Liability here as per user struct
		{Code: "2102-003", Name: "HUTANG PPH PASAL 21", Type: models.AccountTypeLiability, Category: models.CategoryCurrentLiability, Level: 4, IsHeader: false, IsActive: true},
		{Code: "2102-004", Name: "HUTANG PPH PASAL 23", Type: models.AccountTypeLiability, Category: models.CategoryCurrentLiability, Level: 4, IsHeader: false, IsActive: true},
		{Code: "2102-005", Name: "HUTANG PPH PASAL 4(2) FINAL", Type: models.AccountTypeLiability, Category: models.CategoryCurrentLiability, Level: 4, IsHeader: false, IsActive: true},
		{Code: "2102-006", Name: "HUTANG PPH PASAL 22", Type: models.AccountTypeLiability, Category: models.CategoryCurrentLiability, Level: 4, IsHeader: false, IsActive: true},

		{Code: "2103", Name: "BIAYA YANG MASIH HARUS DIBAYAR", Type: models.AccountTypeLiability, Category: models.CategoryCurrentLiability, Level: 3, IsHeader: true, IsActive: true},
		{Code: "2103-001", Name: "UTANG GAJI", Type: models.AccountTypeLiability, Category: models.CategoryCurrentLiability, Level: 4, IsHeader: false, IsActive: true},

		{Code: "2200", Name: "LIABILITAS JANGKA PANJANG", Type: models.AccountTypeLiability, Category: models.CategoryLongTermLiability, Level: 2, IsHeader: true, IsActive: true},
		{Code: "2201", Name: "PINJAMAN BANK JANGKA PANJANG", Type: models.AccountTypeLiability, Category: models.CategoryLongTermLiability, Level: 3, IsHeader: false, IsActive: true},

		// EQUITY (3xxx)
		{Code: "3000", Name: "EKUITAS", Type: models.AccountTypeEquity, Category: models.CategoryEquity, Level: 1, IsHeader: true, IsActive: true, IsSystemCritical: true},
		{Code: "3101", Name: "MODAL DISETOR", Type: models.AccountTypeEquity, Category: models.CategoryShareCapital, Level: 2, IsHeader: false, IsActive: true, IsSystemCritical: true},
		{Code: "3102", Name: "LABA (RUGI) DITAHAN", Type: models.AccountTypeEquity, Category: models.CategoryRetainedEarnings, Level: 2, IsHeader: false, IsActive: true, IsSystemCritical: true},
		{Code: "3103", Name: "LABA (RUGI) TAHUN BERJALAN", Type: models.AccountTypeEquity, Category: models.CategoryEquity, Level: 2, IsHeader: false, IsActive: true, IsSystemCritical: true},
		{Code: "3104", Name: "EKUITAS SALDO AWAL", Type: models.AccountTypeEquity, Category: models.CategoryEquity, Level: 2, IsHeader: false, IsActive: true, IsSystemCritical: true},
	}

	// 1240 PPN MASUKAN legacy/compatibility check -> map to 2102-002 if needed or keep separate?
	// The user request specifies 2102-002 for PPN Masukan under Liability. We will stick to that.

	// Industry Specific Accounts
	var specificAccounts []models.Account

	switch companyType {
	case models.CompanyTypeService:
		specificAccounts = getServiceAccounts()
	case models.CompanyTypeTrading:
		specificAccounts = getTradingAccounts()
	case models.CompanyTypeManufacturing:
		specificAccounts = getManufacturingAccounts()
	default:
		// Default to Service if unknown, or maybe empty?
		specificAccounts = getServiceAccounts()
	}

	// Merge lists
	return append(baseAccounts, specificAccounts...)
}

func getServiceAccounts() []models.Account {
	return []models.Account{
		// REVENUE
		{Code: "4000", Name: "PENDAPATAN", Type: models.AccountTypeRevenue, Category: models.CategoryOperatingRevenue, Level: 1, IsHeader: true, IsActive: true, IsSystemCritical: true},
		{Code: "4101", Name: "PENDAPATAN JASA", Type: models.AccountTypeRevenue, Category: models.CategoryOperatingRevenue, Level: 2, IsHeader: true, IsActive: true, IsSystemCritical: true},
		{Code: "4101-001", Name: "PENDAPATAN JASA KONSULTASI", Type: models.AccountTypeRevenue, Category: models.CategoryOperatingRevenue, Level: 3, IsHeader: false, IsActive: true},
		{Code: "4101-002", Name: "PENDAPATAN JASA PROFESIONAL", Type: models.AccountTypeRevenue, Category: models.CategoryOperatingRevenue, Level: 3, IsHeader: false, IsActive: true},

		// COST OF SERVICE (BEBAN POKOK JASA)
		{Code: "5000", Name: "BEBAN POKOK JASA", Type: models.AccountTypeExpense, Category: models.CategoryCostOfGoodsSold, Level: 1, IsHeader: true, IsActive: true, IsSystemCritical: true},
		{Code: "5101", Name: "BIAYA TENAGA KERJA LANGSUNG", Type: models.AccountTypeExpense, Category: models.CategoryDirectLabor, Level: 2, IsHeader: true, IsActive: true},
		{Code: "5101-001", Name: "GAJI TENAGA PROFESIONAL", Type: models.AccountTypeExpense, Category: models.CategoryDirectLabor, Level: 3, IsHeader: false, IsActive: true},
		{Code: "5102", Name: "BIAYA OPERASIONAL LANGSUNG", Type: models.AccountTypeExpense, Category: models.CategoryCostOfGoodsSold, Level: 2, IsHeader: true, IsActive: true},
		{Code: "5102-001", Name: "BIAYA PROYEK", Type: models.AccountTypeExpense, Category: models.CategoryCostOfGoodsSold, Level: 3, IsHeader: false, IsActive: true},

		// OPERATING EXPENSES
		{Code: "6000", Name: "BEBAN OPERASIONAL", Type: models.AccountTypeExpense, Category: models.CategoryOperatingExpense, Level: 1, IsHeader: true, IsActive: true},
		{Code: "6101", Name: "BEBAN GAJI & TUNJANGAN", Type: models.AccountTypeExpense, Category: models.CategoryOperatingExpense, Level: 2, IsHeader: false, IsActive: true},
		{Code: "6102", Name: "BEBAN SEWA", Type: models.AccountTypeExpense, Category: models.CategoryOperatingExpense, Level: 2, IsHeader: false, IsActive: true},
		{Code: "6103", Name: "BEBAN UTILITAS", Type: models.AccountTypeExpense, Category: models.CategoryOperatingExpense, Level: 2, IsHeader: false, IsActive: true},
		{Code: "6104", Name: "BEBAN PENYUSUTAN", Type: models.AccountTypeExpense, Category: models.CategoryDepreciationExp, Level: 2, IsHeader: false, IsActive: true},

		// OTHER INCOME (7xxx - pure REVENUE)
		{Code: "7000", Name: "PENDAPATAN LAIN-LAIN", Type: models.AccountTypeRevenue, Category: models.CategoryOtherIncome, Level: 1, IsHeader: true, IsActive: true},
		{Code: "7101", Name: "PENDAPATAN BUNGA", Type: models.AccountTypeRevenue, Category: models.CategoryInterestIncome, Level: 2, IsHeader: false, IsActive: true},
		{Code: "7102", Name: "PENDAPATAN LAIN-LAIN", Type: models.AccountTypeRevenue, Category: models.CategoryOtherIncome, Level: 2, IsHeader: false, IsActive: true},
		{Code: "7103", Name: "LABA SELISIH KURS", Type: models.AccountTypeRevenue, Category: models.CategoryOtherIncome, Level: 2, IsHeader: false, IsActive: true},
		{Code: "7104", Name: "PENDAPATAN DIVIDEN", Type: models.AccountTypeRevenue, Category: models.CategoryDividendIncome, Level: 2, IsHeader: false, IsActive: true},
		{Code: "7105", Name: "LABA PENJUALAN ASET", Type: models.AccountTypeRevenue, Category: models.CategoryGainOnSale, Level: 2, IsHeader: false, IsActive: true},

		// OTHER EXPENSES (8xxx - pure EXPENSE)
		{Code: "8000", Name: "BEBAN LAIN-LAIN", Type: models.AccountTypeExpense, Category: models.CategoryNonOperatingExpense, Level: 1, IsHeader: true, IsActive: true},
		{Code: "8101", Name: "BEBAN BUNGA", Type: models.AccountTypeExpense, Category: models.CategoryInterestExpense, Level: 2, IsHeader: false, IsActive: true},
		{Code: "8102", Name: "BEBAN LAIN-LAIN", Type: models.AccountTypeExpense, Category: models.CategoryOtherExpense, Level: 2, IsHeader: false, IsActive: true},
		{Code: "8103", Name: "RUGI SELISIH KURS", Type: models.AccountTypeExpense, Category: models.CategoryOtherExpense, Level: 2, IsHeader: false, IsActive: true},
		{Code: "8104", Name: "RUGI PENJUALAN ASET", Type: models.AccountTypeExpense, Category: models.CategoryLossOnSale, Level: 2, IsHeader: false, IsActive: true},
		{Code: "8105", Name: "BEBAN FINANSIAL LAINNYA", Type: models.AccountTypeExpense, Category: models.CategoryFinancialExpense, Level: 2, IsHeader: false, IsActive: true},

		// TAX (9xxx - pure EXPENSE)
		{Code: "9000", Name: "PAJAK PENGHASILAN", Type: models.AccountTypeExpense, Category: models.CategoryTaxExpense, Level: 1, IsHeader: true, IsActive: true},
		{Code: "9101", Name: "BEBAN PAJAK PENGHASILAN", Type: models.AccountTypeExpense, Category: models.CategoryTaxExpense, Level: 2, IsHeader: false, IsActive: true},
	}
}

func getTradingAccounts() []models.Account {
	return []models.Account{
		// ASSETS - Inventory
		{Code: "1103", Name: "PERSEDIAAN", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 3, IsHeader: true, IsActive: true, IsSystemCritical: true},
		{Code: "1103-001", Name: "PERSEDIAAN BARANG DAGANG", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 4, IsHeader: false, IsActive: true, IsSystemCritical: true},

		// REVENUE
		{Code: "4000", Name: "PENDAPATAN", Type: models.AccountTypeRevenue, Category: models.CategoryOperatingRevenue, Level: 1, IsHeader: true, IsActive: true, IsSystemCritical: true},
		{Code: "4101", Name: "PENJUALAN BARANG", Type: models.AccountTypeRevenue, Category: models.CategoryOperatingRevenue, Level: 2, IsHeader: true, IsActive: true, IsSystemCritical: true},
		{Code: "4101-001", Name: "PENJUALAN LOKAL", Type: models.AccountTypeRevenue, Category: models.CategoryOperatingRevenue, Level: 3, IsHeader: false, IsActive: true},
		{Code: "4102", Name: "RETUR & POTONGAN PENJUALAN", Type: models.AccountTypeRevenue, Category: models.CategoryOperatingRevenue, Level: 2, IsHeader: false, IsActive: true},

		// COGS
		{Code: "5000", Name: "HARGA POKOK PENJUALAN", Type: models.AccountTypeExpense, Category: models.CategoryCostOfGoodsSold, Level: 1, IsHeader: true, IsActive: true, IsSystemCritical: true},
		{Code: "5101", Name: "PEMBELIAN BARANG DAGANG", Type: models.AccountTypeExpense, Category: models.CategoryCostOfGoodsSold, Level: 2, IsHeader: false, IsActive: true},
		{Code: "5102", Name: "BIAYA ANGKUT PEMBELIAN", Type: models.AccountTypeExpense, Category: models.CategoryFreightIn, Level: 2, IsHeader: false, IsActive: true},
		{Code: "5103", Name: "POTONGAN PEMBELIAN", Type: models.AccountTypeExpense, Category: models.CategoryPurchaseReturns, Level: 2, IsHeader: false, IsActive: true},

		// OPERATING EXPENSES
		{Code: "6000", Name: "BEBAN OPERASIONAL", Type: models.AccountTypeExpense, Category: models.CategoryOperatingExpense, Level: 1, IsHeader: true, IsActive: true},
		{Code: "6101", Name: "BEBAN PENJUALAN", Type: models.AccountTypeExpense, Category: models.CategorySellingExpense, Level: 2, IsHeader: true, IsActive: true},
		{Code: "6101-001", Name: "BEBAN IKLAN", Type: models.AccountTypeExpense, Category: models.CategorySellingExpense, Level: 3, IsHeader: false, IsActive: true},
		{Code: "6101-002", Name: "BEBAN PENGIRIMAN", Type: models.AccountTypeExpense, Category: models.CategorySellingExpense, Level: 3, IsHeader: false, IsActive: true},
		{Code: "6102", Name: "BEBAN ADMINISTRASI & UMUM", Type: models.AccountTypeExpense, Category: models.CategoryAdministrativeExp, Level: 2, IsHeader: false, IsActive: true},
		{Code: "6103", Name: "BEBAN PENYUSUTAN", Type: models.AccountTypeExpense, Category: models.CategoryDepreciationExp, Level: 2, IsHeader: false, IsActive: true},

		// OTHER INCOME (7xxx - pure REVENUE)
		{Code: "7000", Name: "PENDAPATAN LAIN-LAIN", Type: models.AccountTypeRevenue, Category: models.CategoryOtherIncome, Level: 1, IsHeader: true, IsActive: true},
		{Code: "7101", Name: "PENDAPATAN BUNGA", Type: models.AccountTypeRevenue, Category: models.CategoryInterestIncome, Level: 2, IsHeader: false, IsActive: true},
		{Code: "7102", Name: "PENDAPATAN LAIN-LAIN", Type: models.AccountTypeRevenue, Category: models.CategoryOtherIncome, Level: 2, IsHeader: false, IsActive: true},
		{Code: "7103", Name: "LABA SELISIH KURS", Type: models.AccountTypeRevenue, Category: models.CategoryOtherIncome, Level: 2, IsHeader: false, IsActive: true},
		{Code: "7104", Name: "PENDAPATAN DIVIDEN", Type: models.AccountTypeRevenue, Category: models.CategoryDividendIncome, Level: 2, IsHeader: false, IsActive: true},
		{Code: "7105", Name: "LABA PENJUALAN ASET", Type: models.AccountTypeRevenue, Category: models.CategoryGainOnSale, Level: 2, IsHeader: false, IsActive: true},

		// OTHER EXPENSES (8xxx - pure EXPENSE)
		{Code: "8000", Name: "BEBAN LAIN-LAIN", Type: models.AccountTypeExpense, Category: models.CategoryNonOperatingExpense, Level: 1, IsHeader: true, IsActive: true},
		{Code: "8101", Name: "BEBAN BUNGA", Type: models.AccountTypeExpense, Category: models.CategoryInterestExpense, Level: 2, IsHeader: false, IsActive: true},
		{Code: "8102", Name: "BEBAN LAIN-LAIN", Type: models.AccountTypeExpense, Category: models.CategoryOtherExpense, Level: 2, IsHeader: false, IsActive: true},
		{Code: "8103", Name: "RUGI SELISIH KURS", Type: models.AccountTypeExpense, Category: models.CategoryOtherExpense, Level: 2, IsHeader: false, IsActive: true},
		{Code: "8104", Name: "RUGI PENJUALAN ASET", Type: models.AccountTypeExpense, Category: models.CategoryLossOnSale, Level: 2, IsHeader: false, IsActive: true},
		{Code: "8105", Name: "BEBAN FINANSIAL LAINNYA", Type: models.AccountTypeExpense, Category: models.CategoryFinancialExpense, Level: 2, IsHeader: false, IsActive: true},

		// TAX (9xxx - pure EXPENSE)
		{Code: "9000", Name: "PAJAK PENGHASILAN", Type: models.AccountTypeExpense, Category: models.CategoryTaxExpense, Level: 1, IsHeader: true, IsActive: true},
		{Code: "9101", Name: "BEBAN PAJAK PENGHASILAN", Type: models.AccountTypeExpense, Category: models.CategoryTaxExpense, Level: 2, IsHeader: false, IsActive: true},
	}
}

func getManufacturingAccounts() []models.Account {
	return []models.Account{
		// ASSETS - Inventory
		{Code: "1103", Name: "PERSEDIAAN", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 3, IsHeader: true, IsActive: true, IsSystemCritical: true},
		{Code: "1103-001", Name: "PERSEDIAAN BAHAN BAKU", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 4, IsHeader: false, IsActive: true, IsSystemCritical: true},
		{Code: "1103-002", Name: "PERSEDIAAN BARANG DALAM PROSES", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 4, IsHeader: false, IsActive: true, IsSystemCritical: true},
		{Code: "1103-003", Name: "PERSEDIAAN BARANG JADI", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, Level: 4, IsHeader: false, IsActive: true, IsSystemCritical: true},

		// REVENUE
		{Code: "4000", Name: "PENDAPATAN", Type: models.AccountTypeRevenue, Category: models.CategoryOperatingRevenue, Level: 1, IsHeader: true, IsActive: true, IsSystemCritical: true},
		{Code: "4101", Name: "PENJUALAN PRODUK", Type: models.AccountTypeRevenue, Category: models.CategoryOperatingRevenue, Level: 2, IsHeader: true, IsActive: true, IsSystemCritical: true},
		{Code: "4101-001", Name: "PENJUALAN PRODUK JADI", Type: models.AccountTypeRevenue, Category: models.CategoryOperatingRevenue, Level: 3, IsHeader: false, IsActive: true},

		// COGS / COGM
		{Code: "5000", Name: "HARGA POKOK PRODUKSI", Type: models.AccountTypeExpense, Category: models.CategoryCostOfGoodsSold, Level: 1, IsHeader: true, IsActive: true, IsSystemCritical: true},
		{Code: "5101", Name: "BIAYA BAHAN BAKU", Type: models.AccountTypeExpense, Category: models.CategoryDirectMaterial, Level: 2, IsHeader: false, IsActive: true},
		{Code: "5102", Name: "BIAYA TENAGA KERJA LANGSUNG", Type: models.AccountTypeExpense, Category: models.CategoryDirectLabor, Level: 2, IsHeader: false, IsActive: true},
		{Code: "5103", Name: "BIAYA OVERHEAD PABRIK", Type: models.AccountTypeExpense, Category: models.CategoryManufacturingOverhead, Level: 2, IsHeader: true, IsActive: true},
		{Code: "5103-001", Name: "LISTRIK PABRIK", Type: models.AccountTypeExpense, Category: models.CategoryManufacturingOverhead, Level: 3, IsHeader: false, IsActive: true},
		{Code: "5103-002", Name: "PENYUSUTAN MESIN", Type: models.AccountTypeExpense, Category: models.CategoryManufacturingOverhead, Level: 3, IsHeader: false, IsActive: true},
		{Code: "5103-003", Name: "PERAWATAN MESIN", Type: models.AccountTypeExpense, Category: models.CategoryManufacturingOverhead, Level: 3, IsHeader: false, IsActive: true},

		// OPERATING EXPENSES
		{Code: "6000", Name: "BEBAN OPERASIONAL", Type: models.AccountTypeExpense, Category: models.CategoryOperatingExpense, Level: 1, IsHeader: true, IsActive: true},
		{Code: "6101", Name: "BEBAN PENJUALAN", Type: models.AccountTypeExpense, Category: models.CategorySellingExpense, Level: 2, IsHeader: false, IsActive: true},
		{Code: "6102", Name: "BEBAN ADMINISTRASI & UMUM", Type: models.AccountTypeExpense, Category: models.CategoryAdministrativeExp, Level: 2, IsHeader: false, IsActive: true},

		// OTHER INCOME (7xxx - pure REVENUE)
		{Code: "7000", Name: "PENDAPATAN LAIN-LAIN", Type: models.AccountTypeRevenue, Category: models.CategoryOtherIncome, Level: 1, IsHeader: true, IsActive: true},
		{Code: "7101", Name: "PENDAPATAN BUNGA", Type: models.AccountTypeRevenue, Category: models.CategoryInterestIncome, Level: 2, IsHeader: false, IsActive: true},
		{Code: "7102", Name: "PENDAPATAN LAIN-LAIN", Type: models.AccountTypeRevenue, Category: models.CategoryOtherIncome, Level: 2, IsHeader: false, IsActive: true},
		{Code: "7103", Name: "LABA SELISIH KURS", Type: models.AccountTypeRevenue, Category: models.CategoryOtherIncome, Level: 2, IsHeader: false, IsActive: true},
		{Code: "7104", Name: "PENDAPATAN DIVIDEN", Type: models.AccountTypeRevenue, Category: models.CategoryDividendIncome, Level: 2, IsHeader: false, IsActive: true},
		{Code: "7105", Name: "LABA PENJUALAN ASET", Type: models.AccountTypeRevenue, Category: models.CategoryGainOnSale, Level: 2, IsHeader: false, IsActive: true},

		// OTHER EXPENSES (8xxx - pure EXPENSE)
		{Code: "8000", Name: "BEBAN LAIN-LAIN", Type: models.AccountTypeExpense, Category: models.CategoryNonOperatingExpense, Level: 1, IsHeader: true, IsActive: true},
		{Code: "8101", Name: "BEBAN BUNGA", Type: models.AccountTypeExpense, Category: models.CategoryInterestExpense, Level: 2, IsHeader: false, IsActive: true},
		{Code: "8102", Name: "BEBAN LAIN-LAIN", Type: models.AccountTypeExpense, Category: models.CategoryOtherExpense, Level: 2, IsHeader: false, IsActive: true},
		{Code: "8103", Name: "RUGI SELISIH KURS", Type: models.AccountTypeExpense, Category: models.CategoryOtherExpense, Level: 2, IsHeader: false, IsActive: true},
		{Code: "8104", Name: "RUGI PENJUALAN ASET", Type: models.AccountTypeExpense, Category: models.CategoryLossOnSale, Level: 2, IsHeader: false, IsActive: true},
		{Code: "8105", Name: "BEBAN FINANSIAL LAINNYA", Type: models.AccountTypeExpense, Category: models.CategoryFinancialExpense, Level: 2, IsHeader: false, IsActive: true},

		// TAX (9xxx - pure EXPENSE)
		{Code: "9000", Name: "PAJAK PENGHASILAN", Type: models.AccountTypeExpense, Category: models.CategoryTaxExpense, Level: 1, IsHeader: true, IsActive: true},
		{Code: "9101", Name: "BEBAN PAJAK PENGHASILAN", Type: models.AccountTypeExpense, Category: models.CategoryTaxExpense, Level: 2, IsHeader: false, IsActive: true},
	}
}

// Helper to normalize names
func upper(s string) string {
	return strings.ToUpper(s)
}
