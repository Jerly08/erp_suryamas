package main

import (
	"fmt"
	"log"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

func main() {
	fmt.Println("=== CLEANUP LEGACY COA ACCOUNTS ===")
	fmt.Println("")

	// Load config and connect to database
	_ = config.LoadConfig()
	db := database.ConnectDB()
	if db == nil {
		log.Fatal("Gagal koneksi ke database")
	}

	fmt.Println("🔗 Berhasil terhubung ke database")

	// List of legacy/invalid account codes to delete
	legacyAccounts := []string{
		// Invalid codes
		"292", // PENAMBAHAN PAJAK LAINNYA (SALES) - Invalid format
		"293", // PENDAPATAN ONGKIR (SHIPPING) - Invalid format

		// Legacy PPN (use 2102-002 instead)
		"1240", // PPN MASUKAN - Legacy, use 2102-002

		// Duplicate PPH accounts (use 1105-xxx instead)
		"1114", // PPH 21 DIBAYAR DIMUKA - Duplicate of 1105-001
		"1115", // PPH 23 DIBAYAR DIMUKA - Duplicate of 1105-002
		"1116", // POTONGAN PAJAK LAINNYA DIBAYAR DIMUKA - Duplicate of 1105-003

		// Legacy Fixed Assets (use 1201-xxx instead)
		"1500", // FIXED ASSETS - Legacy header
		"1501", // PERALATAN KANTOR - Duplicate of 1201-004
		"1502", // KENDARAAN - Duplicate of 1201-003
		"1503", // BANGUNAN - Duplicate of 1201-002
		"1509", // TRUK - Specific, can merge to 1201-003

		// Duplicate equity
		"3201", // LABA DITAHAN - Duplicate of 3102
	}

	// Show accounts to be deleted
	fmt.Println("\n📋 Akun yang akan dihapus:")
	for _, code := range legacyAccounts {
		var acc models.Account
		if err := db.Where("code = ?", code).First(&acc).Error; err == nil {
			fmt.Printf("   ❌ %s - %s (ID: %d, Balance: %.2f)\n", acc.Code, acc.Name, acc.ID, acc.Balance)
		} else {
			fmt.Printf("   ⏭️  %s - Tidak ditemukan\n", code)
		}
	}

	// Check if any account has non-zero balance
	var accountsWithBalance []models.Account
	db.Where("code IN ? AND balance != 0", legacyAccounts).Find(&accountsWithBalance)

	if len(accountsWithBalance) > 0 {
		fmt.Println("\n⚠️  PERINGATAN: Beberapa akun memiliki saldo tidak nol!")
		for _, acc := range accountsWithBalance {
			fmt.Printf("   ⚠️  %s - %s: Rp %.2f\n", acc.Code, acc.Name, acc.Balance)
		}
		fmt.Println("   Melanjutkan penghapusan (saldo akan hilang)...")
	}

	// Delete accounts
	fmt.Println("\n🗑️  Menghapus akun legacy...")

	tx := db.Begin()
	deletedCount := 0

	for _, code := range legacyAccounts {
		result := tx.Unscoped().Where("code = ?", code).Delete(&models.Account{})
		if result.Error != nil {
			fmt.Printf("   ❌ Gagal hapus %s: %v\n", code, result.Error)
		} else if result.RowsAffected > 0 {
			fmt.Printf("   ✅ Dihapus: %s\n", code)
			deletedCount++
		}
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		log.Fatalf("❌ Gagal commit: %v", err)
	}

	fmt.Printf("\n🎉 SELESAI! %d akun legacy berhasil dihapus.\n", deletedCount)

	// Show remaining accounts count
	var totalAccounts int64
	db.Model(&models.Account{}).Where("deleted_at IS NULL").Count(&totalAccounts)
	fmt.Printf("📊 Total akun tersisa: %d\n", totalAccounts)

	// Verify template accounts exist
	fmt.Println("\n✅ Verifikasi akun template standar:")
	verifyTemplateAccounts(db)
}

func verifyTemplateAccounts(db *gorm.DB) {
	criticalAccounts := []struct {
		Code string
		Name string
	}{
		{"1101-002", "KAS BESAR"},
		{"1101-004", "BANK – TABUNGAN"},
		{"1102-001", "PIUTANG USAHA – PIHAK KETIGA"},
		{"2101-001", "UTANG USAHA – LOKAL"},
		{"2102-001", "PPN KELUARAN"},
		{"2102-002", "PPN MASUKAN"},
		{"4101-001", "PENDAPATAN JASA KONSULTASI"},
	}

	for _, acc := range criticalAccounts {
		var found models.Account
		if err := db.Where("code = ?", acc.Code).First(&found).Error; err == nil {
			fmt.Printf("   ✅ %s - %s\n", found.Code, found.Name)
		} else {
			fmt.Printf("   ❌ MISSING: %s - %s\n", acc.Code, acc.Name)
		}
	}
}
