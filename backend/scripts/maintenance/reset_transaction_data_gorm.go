package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/database"
	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

func main() {
	fmt.Println("=== SISTEM AKUNTANSI - RESET DATA ===")
	fmt.Println("")
	fmt.Println("Pilih mode operasi yang diinginkan:")
	fmt.Println("  1) Reset TRANSAKSI (hard delete) — mempertahankan master (DEFAULT)")
	fmt.Println("  2) Soft Delete SEMUA data — menandai semua record (deleted_at)")
	fmt.Println("  3) RECOVERY — kembalikan semua soft deleted data")
	fmt.Println("  4) RESET SALDO MASTER — Reset balance COA & Cash Bank (tanpa hapus akun)")
	fmt.Println("  5) HAPUS PERIODE CLOSING — Hapus semua periode closing yang sudah pernah di-close")
	fmt.Println("  6) RESET COMPANY TYPE — Set company_type ke NULL (untuk test onboarding flow)")
	fmt.Println("  7) TRUNCATE & RESET ID — Hapus SEMUA data transaksi + reset semua ID dari 1")
	fmt.Println("")
	mode := askResetMode()

	if mode == 1 {
		fmt.Println("⚠️  PERINGATAN: Mode 1 akan menghapus SEMUA data transaksi!")
		fmt.Println("✅ Yang akan DIPERTAHANKAN:")
		fmt.Println("   - Chart of Accounts (COA)")
		fmt.Println("   - Master data produk")
		fmt.Println("   - Data kontak/customer/vendor")
		fmt.Println("   - Data user dan permission")
		fmt.Println("   - Master data cash bank")
		fmt.Println("")
		fmt.Println("❌ Yang akan DIHAPUS:")
		fmt.Println("   - Semua transaksi penjualan/pembelian")
		fmt.Println("   - Semua classic journals & SSOT journals")
		fmt.Println("   - Payments, inventory movements, expenses, notifications, stock alerts")
		fmt.Println("   - Balance & stock akan direset ke 0, sequence direset")
	} else if mode == 2 {
		fmt.Println("⚠️  PERINGATAN: Mode 2 akan melakukan SOFT DELETE ke SEMUA record!")
		fmt.Println("   - Data ditandai 'deleted_at = NOW()' tapi TIDAK dihapus permanen")
		fmt.Println("   - Dapat dipulihkan dengan Mode 3 (Recovery)")
		fmt.Println("   - Tabel sistem/backup akan dilewati")
	} else if mode == 4 {
		fmt.Println("🧹 MODE RESET SALDO MASTER: Reset balance COA & CASH BANK (tanpa hapus akun)")
		fmt.Println("   - Menghapus SEMUA transaksi & jurnal (sama seperti Mode 1)")
		fmt.Println("   - Menetapkan saldo accounts.balance & cash_banks.balance = 0 (TANPA TRUNCATE akun)")
		fmt.Println("   - Gunakan hanya di DEV/TEST! Pastikan sudah mem-backup bila perlu")
	} else if mode == 5 {
		fmt.Println("🗑️  MODE HAPUS PERIODE CLOSING: Hapus semua periode closing yang sudah pernah di-close")
		fmt.Println("   - Menghapus SEMUA data di tabel accounting_periods (periode closing)")
		fmt.Println("   - Termasuk periode yang sudah di-close sebelumnya")
		fmt.Println("   - TIDAK menghapus transaksi atau jurnal, hanya data periode closing saja")
		fmt.Println("   - Gunakan jika ingin reset fitur period closing dari awal")
	} else if mode == 6 {
		fmt.Println("⚙️  MODE RESET COMPANY TYPE: Reset konfigurasi jenis perusahaan")
		fmt.Println("   - Mengatur kolom 'company_type' di tabel settings menjadi NULL")
		fmt.Println("   - Memungkinkan Anda mentest flow 'First Login' / pemilihan jenis usaha")
		fmt.Println("   - Data lain TIDAK akan dihapus")
	} else if mode == 7 {
		fmt.Println("🔥 MODE TRUNCATE & RESET ID: HAPUS TOTAL + RESET ID")
		fmt.Println("   - Melakukan TRUNCATE CASCADE pada SEMUA tabel transaksi")
		fmt.Println("   - Reset SEMUA sequence ID dari 1 (ID baru akan mulai dari 1)")
		fmt.Println("   - COA, Products, Contacts, Cash Banks TETAP ADA (hanya balance/stock direset)")
		fmt.Println("   - ⚠️  PERINGATAN: Ini operasi yang sangat merusak!")
	} else {
		fmt.Println("🔄 MODE RECOVERY: Akan mengembalikan semua soft deleted data")
		fmt.Println("   - Set kolom 'deleted_at = NULL' untuk semua record yang soft deleted")
		fmt.Println("   - Data akan kembali muncul di aplikasi")
	}

	// Konfirmasi dari user
	if !confirmReset() {
		fmt.Println("Reset dibatalkan oleh user.")
		return
	}

	// Load config dan connect ke database
	_ = config.LoadConfig() // Load config untuk inisialisasi environment
	db := database.ConnectDB()
	if db == nil {
		log.Fatal("Gagal koneksi ke database")
	}

	fmt.Println("🔗 Berhasil terhubung ke database")

	// Step 1: Backup COA
	fmt.Println("\n📦 STEP 1: Backup Chart of Accounts...")
	if err := backupCOAWithGORM(db); err != nil {
		log.Printf("❌ Gagal backup COA: %v", err)
		return
	}
	fmt.Println("✅ Backup COA berhasil")

	// Step 2: Show current data summary
	fmt.Println("\n📊 STEP 2: Summary data saat ini...")
	if mode == 3 {
		showSoftDeletedDataSummary(db)
	} else if mode == 5 {
		showPeriodClosingSummary(db)
	} else {
		showCurrentDataSummary(db)
	}

	// Final confirmation
	fmt.Println("\n⚠️  KONFIRMASI TERAKHIR:")
	fmt.Print("Ketik 'RESET SEKARANG' untuk melanjutkan: ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input != "RESET SEKARANG" {
		fmt.Println("Reset dibatalkan. Input tidak sesuai.")
		return
	}

	// Step 3: Execute reset/recovery
	if mode == 1 {
		fmt.Println("\n🔄 STEP 3: Mengeksekusi reset data transaksi (HARD DELETE)...")
		if err := executeTransactionResetWithGORM(db); err != nil {
			log.Printf("❌ Gagal reset data transaksi: %v", err)
			return
		}
	} else if mode == 2 {
		fmt.Println("\n🔄 STEP 3: Mengeksekusi SOFT DELETE ke semua tabel...")
		if err := executeSoftDeleteAllWithGORM(db); err != nil {
			log.Printf("❌ Gagal soft delete semua data: %v", err)
			return
		}
	} else if mode == 4 {
		fmt.Println("\n🧹 STEP 3: RESET SALDO MASTER — Hapus transaksi lalu reset saldo COA & Cash Bank (tanpa hapus akun)...")
		if err := executeMasterPurgeWithGORM(db); err != nil {
			log.Printf("❌ Gagal reset saldo master: %v", err)
			return
		}
	} else if mode == 5 {
		fmt.Println("\n🗑️  STEP 3: Menghapus semua periode closing...")
		if err := executeDeletePeriodClosingWithGORM(db); err != nil {
			log.Printf("❌ Gagal hapus periode closing: %v", err)
			return
		}
	} else if mode == 6 {
		fmt.Println("\n⚙️  STEP 3: Resetting company type...")
		if err := executeResetCompanyTypeWithGORM(db); err != nil {
			log.Printf("❌ Gagal reset company type: %v", err)
			return
		}
	} else if mode == 7 {
		fmt.Println("\n🔥 STEP 3: TRUNCATE & RESET ID...")
		if err := executeTruncateAndResetIDWithGORM(db); err != nil {
			log.Printf("❌ Gagal truncate & reset ID: %v", err)
			return
		}
	} else {
		fmt.Println("\n🔄 STEP 3: Mengeksekusi RECOVERY soft deleted data...")
		if err := executeRecoveryAllWithGORM(db); err != nil {
			log.Printf("❌ Gagal recovery data: %v", err)
			return
		}
	}

	// Step 4: Verify reset
	fmt.Println("\n✅ STEP 4: Verifikasi hasil reset...")
	showPostResetSummary(db)

	if mode == 1 {
		fmt.Println("\n🎉 HARD DELETE RESET SELESAI!")
		fmt.Println("Database siap digunakan dengan COA yang bersih.")
		fmt.Println("Anda bisa mulai input transaksi baru dari 0.")
	} else if mode == 2 {
		fmt.Println("\n🎉 SOFT DELETE SELESAI!")
		fmt.Println("Semua data telah ditandai sebagai deleted (deleted_at = NOW()).")
		fmt.Println("Data TIDAK dihapus permanen dan dapat dipulihkan dengan Mode 3 (Recovery).")
		fmt.Println("\nUntuk recovery: go run scripts/maintenance/reset_transaction_data_gorm.go (pilih opsi 3)")
	} else if mode == 4 {
		fmt.Println("\n🎉 RESET SALDO MASTER SELESAI!")
		fmt.Println("Saldo COA & Cash Bank telah direset ke 0 dan semua transaksi/jurnal telah dibersihkan.")
		fmt.Println("Akun COA & Cash Bank tetap ada (tidak dihapus).")
	} else if mode == 5 {
		fmt.Println("\n🎉 HAPUS PERIODE CLOSING SELESAI!")
		fmt.Println("Semua data periode closing telah dihapus dari tabel accounting_periods.")
		fmt.Println("Anda dapat memulai period closing dari awal.")
		fmt.Println("CATATAN: Transaksi dan jurnal TIDAK terpengaruh, hanya data periode closing yang dihapus.")
	} else if mode == 6 {
		fmt.Println("\n🎉 RESET COMPANY TYPE SELESAI!")
		fmt.Println("Company Type telah di-set ke NULL.")
		fmt.Println("Silakan login kembali untuk melihat tampilan pemilihan jenis perusahaan.")
	} else if mode == 7 {
		fmt.Println("\n🎉 TRUNCATE & RESET ID SELESAI!")
		fmt.Println("Semua tabel transaksi telah di-TRUNCATE dan ID direset dari 1.")
		fmt.Println("COA, Products, Contacts, Cash Banks tetap ada dengan balance/stock = 0.")
		fmt.Println("Transaksi baru akan dimulai dari ID = 1.")
	} else {
		fmt.Println("\n🎉 RECOVERY SELESAI!")
		fmt.Println("Semua data yang soft deleted telah dipulihkan (deleted_at = NULL).")
		fmt.Println("Data sekarang akan muncul kembali di aplikasi.")
	}
}

func askResetMode() int {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Masukkan pilihan [1/2/3/4/5/6/7] (default 1): ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "2" {
		return 2
	} else if input == "3" {
		return 3
	} else if input == "4" {
		return 4
	} else if input == "5" {
		return 5
	} else if input == "6" {
		return 6
	} else if input == "7" {
		return 7
	}
	return 1
}

func confirmReset() bool {
	fmt.Print("\nApakah Anda yakin ingin melanjutkan? (ketik 'ya' untuk konfirmasi): ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	return input == "ya" || input == "y" || input == "yes"
}

func backupCOAWithGORM(db *gorm.DB) error {
	fmt.Println("   📊 Membuat backup tabel accounts...")

	// Create backup table using raw SQL (safer approach)
	err := db.Exec(`
		CREATE TABLE IF NOT EXISTS accounts_backup AS 
		SELECT 
		    id, code, name, description, type, category, parent_id, 
		    level, is_header, is_active, balance, 
		    created_at, updated_at
		FROM accounts 
		WHERE deleted_at IS NULL
	`).Error

	if err != nil {
		return fmt.Errorf("gagal membuat backup accounts: %v", err)
	}

	fmt.Println("   📈 Membuat backup hierarki accounts...")

	// Create hierarchy backup
	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS accounts_hierarchy_backup AS
		WITH RECURSIVE account_tree AS (
		    SELECT 
		        id, code, name, parent_id, level, 
		        ARRAY[id] as path,
		        code::text as full_path
		    FROM accounts 
		    WHERE parent_id IS NULL AND deleted_at IS NULL
		    
		    UNION ALL
		    
		    SELECT 
		        a.id, a.code, a.name, a.parent_id, a.level,
		        at.path || a.id,
		        at.full_path || ' > ' || a.code
		    FROM accounts a
		    INNER JOIN account_tree at ON a.parent_id = at.id
		    WHERE a.deleted_at IS NULL
		)
		SELECT * FROM account_tree
	`).Error

	if err != nil {
		fmt.Printf("   ⚠️  Warning: Gagal backup hierarki: %v\n", err)
	}

	fmt.Println("   💰 Membuat backup balance asli...")

	// Create balance backup
	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS accounts_original_balances AS
		SELECT 
		    id, code, name, balance, 
		    'ORIGINAL' as balance_type,
		    CURRENT_TIMESTAMP as backup_date
		FROM accounts 
		WHERE deleted_at IS NULL AND balance != 0
	`).Error

	if err != nil {
		fmt.Printf("   ⚠️  Warning: Gagal backup balance: %v\n", err)
	}

	return nil
}

func executeTransactionResetWithGORM(db *gorm.DB) error {
	start := time.Now()
	fmt.Printf("⏳ Mengeksekusi reset dengan GORM... (dimulai: %s)\n", start.Format("15:04:05"))

	// Start transaction with hooks disabled
	tx := db.Session(&gorm.Session{SkipHooks: true}).Begin()

	// IMPORTANT: Temporarily disable user triggers & FK enforcement inside this TX
	// This prevents errors like: "tuple to be updated was already modified by an operation triggered by the current command"
	if err := tx.Exec("SET LOCAL session_replication_role = 'replica'").Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Gagal set session_replication_role=replica (but continuing): %v\n", err)
	}
	// If any DEFERRABLE constraints exist, defer them to end of TX
	_ = tx.Exec("SET CONSTRAINTS ALL DEFERRED").Error

	// Delete data using GORM (safer, handles missing tables automatically)
	// PENTING: Urutan delete harus mengikuti foreign key dependencies!
	// Urutan: anak-anak tabel dulu, baru induk

	// ===== SSOT JOURNAL SYSTEM RESET =====
	fmt.Println("   🔄 SSOT JOURNAL SYSTEM: Menghapus journal event logs...")
	// SSOT Journal Event Log (audit trail)
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.SSOTJournalEventLog{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete SSOTJournalEventLog error: %v\n", err)
	}

	fmt.Println("   🔄 SSOT JOURNAL SYSTEM: Menghapus unified journal lines...")
	// SSOT Journal Lines (child of unified journal entries)
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.SSOTJournalLine{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete SSOTJournalLine error: %v\n", err)
	}

	fmt.Println("   🔄 SSOT JOURNAL SYSTEM: Menghapus unified journal entries...")
	// SSOT Journal Entries (main unified ledger)
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.SSOTJournalEntry{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete SSOTJournalEntry error: %v\n", err)
	}

	// Also remove legacy/simple SSOT journal tables used by some endpoints
	fmt.Println("   🔄 SSOT JOURNAL SYSTEM: Menghapus simple_ssot_journal_items...")
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.SimpleSSOTJournalItem{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete SimpleSSOTJournalItem error: %v\n", err)
	}
	fmt.Println("   🔄 SSOT JOURNAL SYSTEM: Menghapus simple_ssot_journals...")
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.SimpleSSOTJournal{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete SimpleSSOTJournal error: %v\n", err)
	}

	// ===== CLASSIC JOURNAL SYSTEM RESET =====
	fmt.Println("   📊 CLASSIC JOURNAL: Menghapus journal lines...")
	// Classic Journal Lines (child of journal entries)
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.JournalLine{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete JournalLine error: %v\n", err)
	}

	fmt.Println("   🗑️  Menghapus payment allocations...")
	// Payment allocations (referenced by sales/purchases)
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.PaymentAllocation{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete PaymentAllocation error: %v\n", err)
	}

	fmt.Println("   🗑️  Menghapus cash bank transactions...")
	// Cash Bank Transactions (might be referenced by payments)
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.CashBankTransaction{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete CashBankTransaction error: %v\n", err)
	}

	fmt.Println("   🗑️  Menghapus sale related data...")
	// Sales related - PROPER ORDER: child records first!
	// 1. SaleReturnItem (child of SaleReturn)
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.SaleReturnItem{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete SaleReturnItem error: %v\n", err)
	}
	// 2. SaleReturn (references Sale)
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.SaleReturn{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete SaleReturn error: %v\n", err)
	}
	// 3. SalePayment (references Sale)
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.SalePayment{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete SalePayment error: %v\n", err)
	}
	// 4. SaleItem (references Sale)
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.SaleItem{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete SaleItem error: %v\n", err)
	}
	// 5. Finally, Sale (parent table)
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.Sale{}).Error; err != nil {
		fmt.Printf("   ❌ Error deleting Sale: %v\n", err)
		tx.Rollback()
		return fmt.Errorf("gagal hapus sales: %v", err)
	}

	fmt.Println("   🗑️  Menghapus purchase related data...")
	// Purchase related - PROPER ORDER: deepest child first!
	// 1. PurchaseReceiptItem (child of PurchaseReceipt, references PurchaseItem)
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.PurchaseReceiptItem{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete PurchaseReceiptItem error: %v\n", err)
	}
	// 2. PurchaseReceipt (references Purchase)
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.PurchaseReceipt{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete PurchaseReceipt error: %v\n", err)
	}
	// 3. PurchaseDocument (references Purchase)
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.PurchaseDocument{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete PurchaseDocument error: %v\n", err)
	}
	// 4. PurchaseItem (references Purchase)
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.PurchaseItem{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete PurchaseItem error: %v\n", err)
	}
	// 5. Finally, Purchase (parent table)
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.Purchase{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete Purchase error: %v\n", err)
	}

	fmt.Println("   🗑️  Menghapus approval dan workflow data...")
	// Approval system (hapus setelah purchase/sales yang mereference)
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.ApprovalHistory{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete ApprovalHistory error (maybe table not exist): %v\n", err)
	}
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.ApprovalAction{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete ApprovalAction error (maybe table not exist): %v\n", err)
	}
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.ApprovalRequest{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete ApprovalRequest error (maybe table not exist): %v\n", err)
	}

	fmt.Println("   📊 CLASSIC JOURNAL: Menghapus journal entries...")
	// Classic Journal entries
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.JournalEntry{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete JournalEntry error: %v\n", err)
	}
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.Journal{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete Journal error: %v\n", err)
	}

	fmt.Println("   🗑️  Menghapus transactions...")
	// Transactions
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.Transaction{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete Transaction error: %v\n", err)
	}

	fmt.Println("   🗑️  Menghapus payments...")
	// Payments
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.Payment{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete Payment error: %v\n", err)
	}

	fmt.Println("   🗑️  Menghapus expenses...")
	// Expenses
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.Expense{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete Expense error: %v\n", err)
	}

	fmt.Println("   🗑️  Menghapus inventory movements...")
	// Inventory
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.Inventory{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete Inventory error: %v\n", err)
	}

	// ===== NOTIFICATION SYSTEM RESET =====
	fmt.Println("   🔔 NOTIFICATION SYSTEM: Menghapus stock alerts...")
	// Stock Alerts (child of products)
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.StockAlert{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete StockAlert error: %v\n", err)
	}

	fmt.Println("   🔔 NOTIFICATION SYSTEM: Menghapus notifications...")
	// Notifications (might reference transactions or users)
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.Notification{}).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Delete Notification error: %v\n", err)
	}

	fmt.Println("   🔄 Reset balance accounts ke 0...")
	// Reset account balances
	if err := tx.Model(&models.Account{}).Where("id > 0").Update("balance", 0).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Reset balance accounts gagal: %v\n", err)
	}

	fmt.Println("   🔄 Reset balance cash banks ke 0...")
	// Reset cash bank balances
	if err := tx.Model(&models.CashBank{}).Where("id > 0").Update("balance", 0).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Reset balance cash_banks gagal: %v\n", err)
	}

	fmt.Println("   🔄 Reset stock products ke 0...")
	// Reset product stock
	if err := tx.Model(&models.Product{}).Where("id > 0").Update("stock", 0).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Reset stock products gagal: %v\n", err)
	}

	// Commit transaction (SET LOCAL will auto-revert here)
	if err := tx.Commit().Error; err != nil {
		_ = tx.Rollback().Error
		return fmt.Errorf("gagal commit reset: %v", err)
	}

	// Reset sequences for main transaction tables
	fmt.Println("   🔢 Reset sequences...")
	resetMainSequences(db)

	duration := time.Since(start)
	fmt.Printf("✅ Reset selesai dalam waktu: %s\n", duration.String())

	// Log reset activity
	logResetActivity(db)

	return nil
}

func resetMainSequences(db *gorm.DB) {
	sequences := []string{
		// Transaction sequences
		"sales_id_seq",
		"sale_items_id_seq",
		"purchases_id_seq",
		"purchase_items_id_seq",
		"purchase_receipts_id_seq",
		"purchase_receipt_items_id_seq",
		"purchase_documents_id_seq",
		"transactions_id_seq",
		"payments_id_seq",
		"expenses_id_seq",
		"inventories_id_seq",
		// Notification sequences
		"notifications_id_seq",
		"stock_alerts_id_seq",
		// Classic journal sequences
		"journals_id_seq",
		"journal_entries_id_seq",
		"journal_lines_id_seq",
		// SSOT journal sequences
		"unified_journal_ledger_id_seq",
		"unified_journal_lines_id_seq",
		"journal_event_log_id_seq",
	}

	for _, seq := range sequences {
		// Check if sequence exists before resetting
		var exists bool
		db.Raw("SELECT EXISTS (SELECT 1 FROM pg_class WHERE relname = ? AND relkind = 'S')", seq).Scan(&exists)

		if exists {
			if err := db.Exec(fmt.Sprintf("ALTER SEQUENCE %s RESTART WITH 1", seq)).Error; err != nil {
				fmt.Printf("   ⚠️  Warning: Gagal reset sequence %s: %v\n", seq, err)
			}
		}
	}
}

func executeMasterPurgeWithGORM(db *gorm.DB) error {
	start := time.Now()
	// 1) Reset transaksi & jurnal seperti Mode 1
	fmt.Println("   🔄 Menghapus semua transaksi & jurnal (langkah 1/2)...")
	if err := executeTransactionResetWithGORM(db); err != nil {
		return err
	}

	// 2) Reset saldo master (tanpa hapus akun)
	fmt.Println("   🧮 Menyetel saldo COA & Cash Bank ke 0 (langkah 2/2)...")
	// Lakukan dalam TX terpisah dengan hooks & triggers dimatikan untuk konsistensi
	tx := db.Session(&gorm.Session{SkipHooks: true}).Begin()
	if err := tx.Exec("SET LOCAL session_replication_role = 'replica'").Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Gagal set session_replication_role=replica (but continuing): %v\n", err)
	}
	_ = tx.Exec("SET CONSTRAINTS ALL DEFERRED").Error

	if err := tx.Model(&models.Account{}).Where("deleted_at IS NULL").Update("balance", 0).Error; err != nil {
		_ = tx.Rollback().Error
		return fmt.Errorf("gagal reset saldo accounts: %v", err)
	}
	if err := tx.Model(&models.CashBank{}).Where("deleted_at IS NULL").Update("balance", 0).Error; err != nil {
		_ = tx.Rollback().Error
		return fmt.Errorf("gagal reset saldo cash_banks: %v", err)
	}

	if err := tx.Commit().Error; err != nil {
		_ = tx.Rollback().Error
		return fmt.Errorf("gagal commit reset saldo master: %v", err)
	}

	// Optional: Refresh MV jika ada
	_ = db.Exec("REFRESH MATERIALIZED VIEW account_balances").Error

	dur := time.Since(start)
	fmt.Printf("✅ Reset saldo master selesai dalam %s\n", dur.String())
	return nil
}

func logResetActivity(db *gorm.DB) {
	fmt.Println("   📝 Logging reset activity...")

	// Try to log to audit_logs
	err := db.Exec(`
		INSERT INTO audit_logs (action, table_name, record_id, old_values, new_values, user_id, created_at)
		VALUES ('DELETE', 'ALL_TRANSACTION_TABLES', 0, '', 'Reset semua data transaksi dengan GORM + SSOT Journal System', 1, CURRENT_TIMESTAMP)
	`).Error

	if err != nil {
		fmt.Printf("   ⚠️  Warning: Gagal log ke audit_logs: %v\n", err)
		fmt.Println("   ℹ️  Reset tetap berhasil, hanya logging yang gagal")
	}

	// Also refresh materialized view if it exists
	fmt.Println("   🔄 Refreshing account_balances materialized view...")
	if err := db.Exec("REFRESH MATERIALIZED VIEW account_balances").Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Gagal refresh materialized view: %v\n", err)
	}
}

func executeResetCompanyTypeWithGORM(db *gorm.DB) error {
	fmt.Println("   ⚙️  Mengupdate tabel settings (company_type -> NULL)...")

	// Execute raw SQL update
	// Note: We use raw SQL because GORM might have issues updating to NULL if the model field isn't a pointer
	if err := db.Exec("UPDATE settings SET company_type = NULL").Error; err != nil {
		return fmt.Errorf("gagal update database: %v", err)
	}

	// Clean up legacy/unused accounts
	if err := cleanUnusedAccounts(db); err != nil {
		fmt.Printf("   ⚠️  Warning: Gagal membersihkan akun lama: %v\n", err)
	}

	fmt.Println("   ✅ Berhasil reset company_type")
	return nil
}

func cleanUnusedAccounts(db *gorm.DB) error {
	fmt.Println("   🧹 Cleaning up unused/legacy accounts...")

	// List of accounts to always remove (Legacy/Invalid/Duplicate)
	// ⚠️ IMPORTANT: Do NOT include accounts that are part of any active template!
	// Valid template accounts: 2000, 2100, 2200, 4102, 5000, 5101, 5102, 5103, etc.
	legacyAccounts := []string{
		"1301", // Legacy Inventory (replaced by 1103)
		"292",  // PENAMBAHAN PAJAK LAINNYA (SALES) - Invalid format
		"293",  // PENDAPATAN ONGKIR (SHIPPING) - Invalid format
		"1240", // PPN MASUKAN - Legacy, replaced by 2102-002
		"1114", // PPH 21 DIBAYAR DIMUKA - Duplicate/Legacy
		"1115", // PPH 23 DIBAYAR DIMUKA - Duplicate/Legacy
		"1116", // POTONGAN PAJAK LAINNYA - Duplicate/Legacy
		"1500", // FIXED ASSETS - Legacy header
		"1501", // PERALATAN KANTOR - Legacy
		"1502", // KENDARAAN - Legacy
		"1503", // BANGUNAN - Legacy
		"1509", // TRUK - Legacy
		"3201", // LABA DITAHAN - Duplicate of 3102
		"2104", "2107", "2108", "2111", "2112", // Legacy Tax Payables
		"4900", // OTHER INCOME - Legacy English
		"5900", // GENERAL EXPENSE - Legacy English
		"5200", "5201", "5202", "5203", "5204", // Legacy Expenses (using 5xxx instead of 6xxx)
		"4201", // PENDAPATAN LAIN-LAIN - Duplicate of 7101
	}

	// Use a transaction with FK constraints deferred for safety
	tx := db.Begin()
	if err := tx.Exec("SET LOCAL session_replication_role = 'replica'").Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Gagal set session_replication_role=replica: %v\n", err)
	}

	// Delete legacy accounts (only those NOT in any template)
	query := "DELETE FROM accounts WHERE code IN ? AND balance = 0"
	result := tx.Exec(query, legacyAccounts)

	if result.Error != nil {
		fmt.Printf("   ⚠️  Warning: Failed to delete legacy accounts: %v\n", result.Error)
	} else {
		fmt.Printf("   ✨ Legacy accounts cleaned: %d accounts removed\n", result.RowsAffected)
	}

	// Commit — SET LOCAL automatically reverts session_replication_role
	if err := tx.Commit().Error; err != nil {
		_ = tx.Rollback().Error
		return fmt.Errorf("gagal commit cleanup: %v", err)
	}

	return nil
}

func showCurrentDataSummary(db *gorm.DB) {
	type DataSummary struct {
		TotalAccounts       int64 `json:"total_accounts"`
		TotalSales          int64 `json:"total_sales"`
		TotalPurchases      int64 `json:"total_purchases"`
		TotalTransactions   int64 `json:"total_transactions"`
		TotalJournals       int64 `json:"total_journals"`
		TotalJournalEntries int64 `json:"total_journal_entries"`
		TotalJournalLines   int64 `json:"total_journal_lines"`
		TotalSSOTJournals   int64 `json:"total_ssot_journals"`
		TotalSSOTLines      int64 `json:"total_ssot_lines"`
		TotalSSOTEventLogs  int64 `json:"total_ssot_event_logs"`
		TotalExpenses       int64 `json:"total_expenses"`
		TotalPayments       int64 `json:"total_payments"`
		TotalInventory      int64 `json:"total_inventory"`
		TotalNotifications  int64 `json:"total_notifications"`
		TotalStockAlerts    int64 `json:"total_stock_alerts"`
		TotalProducts       int64 `json:"total_products"`
		TotalContacts       int64 `json:"total_contacts"`
		TotalCashBanks      int64 `json:"total_cash_banks"`
	}

	var summary DataSummary

	db.Model(&models.Account{}).Where("deleted_at IS NULL").Count(&summary.TotalAccounts)
	db.Model(&models.Sale{}).Count(&summary.TotalSales)
	db.Model(&models.Purchase{}).Count(&summary.TotalPurchases)
	db.Model(&models.Transaction{}).Count(&summary.TotalTransactions)
	db.Model(&models.Journal{}).Count(&summary.TotalJournals)
	db.Model(&models.JournalEntry{}).Count(&summary.TotalJournalEntries)
	db.Model(&models.JournalLine{}).Count(&summary.TotalJournalLines)
	db.Model(&models.SSOTJournalEntry{}).Count(&summary.TotalSSOTJournals)
	db.Model(&models.SSOTJournalLine{}).Count(&summary.TotalSSOTLines)
	db.Model(&models.SSOTJournalEventLog{}).Count(&summary.TotalSSOTEventLogs)
	db.Model(&models.Expense{}).Count(&summary.TotalExpenses)
	db.Model(&models.Payment{}).Count(&summary.TotalPayments)
	db.Model(&models.Inventory{}).Count(&summary.TotalInventory)
	db.Model(&models.Notification{}).Count(&summary.TotalNotifications)
	db.Model(&models.StockAlert{}).Count(&summary.TotalStockAlerts)
	db.Model(&models.Product{}).Where("deleted_at IS NULL").Count(&summary.TotalProducts)
	db.Model(&models.Contact{}).Where("deleted_at IS NULL").Count(&summary.TotalContacts)
	db.Model(&models.CashBank{}).Where("deleted_at IS NULL").Count(&summary.TotalCashBanks)

	fmt.Printf("📊 Data saat ini:\n")
	fmt.Printf("   COA Accounts: %d (akan DIPERTAHANKAN)\n", summary.TotalAccounts)
	fmt.Printf("   Products: %d (akan DIPERTAHANKAN, stock direset)\n", summary.TotalProducts)
	fmt.Printf("   Contacts: %d (akan DIPERTAHANKAN)\n", summary.TotalContacts)
	fmt.Printf("   Cash Banks: %d (akan DIPERTAHANKAN, balance direset)\n", summary.TotalCashBanks)
	fmt.Printf("   \n")
	fmt.Printf("   Sales: %d (akan DIHAPUS)\n", summary.TotalSales)
	fmt.Printf("   Purchases: %d (akan DIHAPUS)\n", summary.TotalPurchases)
	fmt.Printf("   Transactions: %d (akan DIHAPUS)\n", summary.TotalTransactions)
	fmt.Printf("   Classic Journals: %d (akan DIHAPUS)\n", summary.TotalJournals)
	fmt.Printf("   Classic Journal Entries: %d (akan DIHAPUS)\n", summary.TotalJournalEntries)
	fmt.Printf("   Classic Journal Lines: %d (akan DIHAPUS)\n", summary.TotalJournalLines)
	fmt.Printf("   SSOT Journal Entries: %d (akan DIHAPUS)\n", summary.TotalSSOTJournals)
	fmt.Printf("   SSOT Journal Lines: %d (akan DIHAPUS)\n", summary.TotalSSOTLines)
	fmt.Printf("   SSOT Event Logs: %d (akan DIHAPUS)\n", summary.TotalSSOTEventLogs)
	fmt.Printf("   Expenses: %d (akan DIHAPUS)\n", summary.TotalExpenses)
	fmt.Printf("   Payments: %d (akan DIHAPUS)\n", summary.TotalPayments)
	fmt.Printf("   Inventory: %d (akan DIHAPUS)\n", summary.TotalInventory)
	fmt.Printf("   Notifications: %d (akan DIHAPUS)\n", summary.TotalNotifications)
	fmt.Printf("   Stock Alerts: %d (akan DIHAPUS)\n", summary.TotalStockAlerts)
}

// executeSoftDeleteAllWithGORM melakukan soft delete (set deleted_at = NOW())
// ke semua tabel yang memiliki kolom deleted_at (kecuali tabel sistem/backup).
func executeSoftDeleteAllWithGORM(db *gorm.DB) error {
	start := time.Now()
	fmt.Printf("⏳ Menandai soft delete semua data... (dimulai: %s)\n", start.Format("15:04:05"))

	tx := db.Begin()

	// Ambil semua tabel yang memiliki kolom deleted_at
	type tbl struct{ TableName string }
	var tables []tbl
	if err := tx.Raw(`
		SELECT table_name AS table_name
		FROM information_schema.columns
		WHERE table_schema = 'public' AND column_name = 'deleted_at'
	`).Scan(&tables).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("gagal membaca metadata tabel: %v", err)
	}

	// Daftar pengecualian (tabel sistem/backup)
	exclude := map[string]bool{
		"accounts_backup":            true,
		"accounts_hierarchy_backup":  true,
		"accounts_original_balances": true,
		"schema_migrations":          true,
		"gorm_migrations":            true,
	}

	processed := 0
	for _, t := range tables {
		name := t.TableName
		if exclude[name] {
			continue
		}
		// Set deleted_at jika masih NULL
		q := fmt.Sprintf("UPDATE %s SET deleted_at = NOW() WHERE deleted_at IS NULL", name)
		if err := tx.Exec(q).Error; err != nil {
			fmt.Printf("   ⚠️  Warning: Gagal soft delete tabel %s: %v\n", name, err)
			continue
		}
		processed++
		fmt.Printf("   ✅ Soft-deleted: %s\n", name)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("gagal commit soft delete: %v", err)
	}

	dur := time.Since(start)
	fmt.Printf("✅ Soft delete selesai. %d tabel diproses dalam %s\n", processed, dur.String())
	return nil
}

// executeRecoveryAllWithGORM memulihkan semua soft deleted data
// dengan mengset deleted_at = NULL untuk semua record yang memiliki deleted_at IS NOT NULL
func executeRecoveryAllWithGORM(db *gorm.DB) error {
	start := time.Now()
	fmt.Printf("⏳ Memulihkan semua soft deleted data... (dimulai: %s)\n", start.Format("15:04:05"))

	tx := db.Begin()

	// Ambil semua tabel yang memiliki kolom deleted_at
	type tbl struct{ TableName string }
	var tables []tbl
	if err := tx.Raw(`
		SELECT table_name AS table_name
		FROM information_schema.columns
		WHERE table_schema = 'public' AND column_name = 'deleted_at'
	`).Scan(&tables).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("gagal membaca metadata tabel: %v", err)
	}

	// Daftar pengecualian (tabel sistem/backup)
	exclude := map[string]bool{
		"accounts_backup":            true,
		"accounts_hierarchy_backup":  true,
		"accounts_original_balances": true,
		"schema_migrations":          true,
		"gorm_migrations":            true,
	}

	processed := 0
	totalRecovered := 0
	for _, t := range tables {
		name := t.TableName
		if exclude[name] {
			continue
		}

		// Hitung dulu berapa record yang akan dipulihkan
		var count int64
		tx.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE deleted_at IS NOT NULL", name)).Scan(&count)

		if count > 0 {
			// Set deleted_at = NULL untuk record yang soft deleted
			q := fmt.Sprintf("UPDATE %s SET deleted_at = NULL WHERE deleted_at IS NOT NULL", name)
			if err := tx.Exec(q).Error; err != nil {
				fmt.Printf("   ⚠️  Warning: Gagal recovery tabel %s: %v\n", name, err)
				continue
			}
			totalRecovered += int(count)
			fmt.Printf("   ✅ Recovered: %s (%d records)\n", name, count)
		} else {
			fmt.Printf("   ℹ Skipped: %s (no deleted records)\n", name)
		}
		processed++
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("gagal commit recovery: %v", err)
	}

	dur := time.Since(start)
	fmt.Printf("✅ Recovery selesai. %d tabel diproses, %d record dipulihkan dalam %s\n", processed, totalRecovered, dur.String())
	return nil
}

func showPostResetSummary(db *gorm.DB) {
	type PostResetSummary struct {
		TotalAccounts           int64 `json:"total_accounts"`
		TotalProducts           int64 `json:"total_products"`
		TotalContacts           int64 `json:"total_contacts"`
		TotalCashBanks          int64 `json:"total_cash_banks"`
		RemainingSales          int64 `json:"remaining_sales"`
		RemainingPurchases      int64 `json:"remaining_purchases"`
		RemainingTransactions   int64 `json:"remaining_transactions"`
		RemainingJournals       int64 `json:"remaining_journals"`
		RemainingJournalEntries int64 `json:"remaining_journal_entries"`
		RemainingJournalLines   int64 `json:"remaining_journal_lines"`
		RemainingSSOTJournals   int64 `json:"remaining_ssot_journals"`
		RemainingSSOTLines      int64 `json:"remaining_ssot_lines"`
		RemainingSSOTEventLogs  int64 `json:"remaining_ssot_event_logs"`
		RemainingNotifications  int64 `json:"remaining_notifications"`
		RemainingStockAlerts    int64 `json:"remaining_stock_alerts"`
		AccountsWithBalance     int64 `json:"accounts_with_balance"`
		ProductsWithStock       int64 `json:"products_with_stock"`
		CashBanksWithBalance    int64 `json:"cash_banks_with_balance"`
	}

	var summary PostResetSummary

	// Count preserved data
	db.Model(&models.Account{}).Where("deleted_at IS NULL").Count(&summary.TotalAccounts)
	db.Model(&models.Product{}).Where("deleted_at IS NULL").Count(&summary.TotalProducts)
	db.Model(&models.Contact{}).Where("deleted_at IS NULL").Count(&summary.TotalContacts)
	db.Model(&models.CashBank{}).Where("deleted_at IS NULL").Count(&summary.TotalCashBanks)

	// Count remaining transaction data (should be 0)
	db.Model(&models.Sale{}).Count(&summary.RemainingSales)
	db.Model(&models.Purchase{}).Count(&summary.RemainingPurchases)
	db.Model(&models.Transaction{}).Count(&summary.RemainingTransactions)
	db.Model(&models.Journal{}).Count(&summary.RemainingJournals)
	db.Model(&models.JournalEntry{}).Count(&summary.RemainingJournalEntries)
	db.Model(&models.JournalLine{}).Count(&summary.RemainingJournalLines)
	db.Model(&models.SSOTJournalEntry{}).Count(&summary.RemainingSSOTJournals)
	db.Model(&models.SSOTJournalLine{}).Count(&summary.RemainingSSOTLines)
	db.Model(&models.SSOTJournalEventLog{}).Count(&summary.RemainingSSOTEventLogs)
	db.Model(&models.Notification{}).Count(&summary.RemainingNotifications)
	db.Model(&models.StockAlert{}).Count(&summary.RemainingStockAlerts)

	// Count data with non-zero balances (should be 0)
	db.Model(&models.Account{}).Where("balance != 0 AND deleted_at IS NULL").Count(&summary.AccountsWithBalance)
	db.Model(&models.Product{}).Where("stock != 0 AND deleted_at IS NULL").Count(&summary.ProductsWithStock)
	db.Model(&models.CashBank{}).Where("balance != 0 AND deleted_at IS NULL").Count(&summary.CashBanksWithBalance)

	fmt.Printf("📊 Hasil reset:\n")
	fmt.Printf("✅ Data yang dipertahankan:\n")
	fmt.Printf("   - COA Accounts: %d\n", summary.TotalAccounts)
	fmt.Printf("   - Products: %d\n", summary.TotalProducts)
	fmt.Printf("   - Contacts: %d\n", summary.TotalContacts)
	fmt.Printf("   - Cash Banks: %d\n", summary.TotalCashBanks)
	fmt.Printf("\n")

	if summary.RemainingSales == 0 && summary.RemainingPurchases == 0 &&
		summary.RemainingTransactions == 0 && summary.RemainingJournals == 0 &&
		summary.RemainingJournalEntries == 0 && summary.RemainingJournalLines == 0 &&
		summary.RemainingSSOTJournals == 0 && summary.RemainingSSOTLines == 0 &&
		summary.RemainingSSOTEventLogs == 0 && summary.RemainingNotifications == 0 &&
		summary.RemainingStockAlerts == 0 {
		fmt.Printf("✅ Data transaksi berhasil dihapus:\n")
		fmt.Printf("   - Sales: %d\n", summary.RemainingSales)
		fmt.Printf("   - Purchases: %d\n", summary.RemainingPurchases)
		fmt.Printf("   - Transactions: %d\n", summary.RemainingTransactions)
		fmt.Printf("   - Classic Journals: %d\n", summary.RemainingJournals)
		fmt.Printf("   - Classic Journal Entries: %d\n", summary.RemainingJournalEntries)
		fmt.Printf("   - Classic Journal Lines: %d\n", summary.RemainingJournalLines)
		fmt.Printf("   - SSOT Journal Entries: %d\n", summary.RemainingSSOTJournals)
		fmt.Printf("   - SSOT Journal Lines: %d\n", summary.RemainingSSOTLines)
		fmt.Printf("   - SSOT Event Logs: %d\n", summary.RemainingSSOTEventLogs)
		fmt.Printf("   - Notifications: %d\n", summary.RemainingNotifications)
		fmt.Printf("   - Stock Alerts: %d\n", summary.RemainingStockAlerts)
	} else {
		fmt.Printf("⚠️  Peringatan - Sisa data transaksi:\n")
		fmt.Printf("   - Sales: %d\n", summary.RemainingSales)
		fmt.Printf("   - Purchases: %d\n", summary.RemainingPurchases)
		fmt.Printf("   - Transactions: %d\n", summary.RemainingTransactions)
		fmt.Printf("   - Classic Journals: %d\n", summary.RemainingJournals)
		fmt.Printf("   - Classic Journal Entries: %d\n", summary.RemainingJournalEntries)
		fmt.Printf("   - Classic Journal Lines: %d\n", summary.RemainingJournalLines)
		fmt.Printf("   - SSOT Journal Entries: %d\n", summary.RemainingSSOTJournals)
		fmt.Printf("   - SSOT Journal Lines: %d\n", summary.RemainingSSOTLines)
		fmt.Printf("   - SSOT Event Logs: %d\n", summary.RemainingSSOTEventLogs)
		fmt.Printf("   - Notifications: %d\n", summary.RemainingNotifications)
		fmt.Printf("   - Stock Alerts: %d\n", summary.RemainingStockAlerts)
	}

	fmt.Printf("\n")

	if summary.AccountsWithBalance == 0 && summary.ProductsWithStock == 0 &&
		summary.CashBanksWithBalance == 0 {
		fmt.Printf("✅ Balance berhasil direset:\n")
		fmt.Printf("   - Accounts dengan balance > 0: %d\n", summary.AccountsWithBalance)
		fmt.Printf("   - Products dengan stock > 0: %d\n", summary.ProductsWithStock)
		fmt.Printf("   - Cash Banks dengan balance > 0: %d\n", summary.CashBanksWithBalance)
	} else {
		fmt.Printf("⚠️  Balance yang belum direset:\n")
		fmt.Printf("   - Accounts dengan balance > 0: %d\n", summary.AccountsWithBalance)
		fmt.Printf("   - Products dengan stock > 0: %d\n", summary.ProductsWithStock)
		fmt.Printf("   - Cash Banks dengan balance > 0: %d\n", summary.CashBanksWithBalance)
	}

	// Show backup info (cek dengan error handling)
	showBackupInfo(db)

	fmt.Println("\n🔧 Untuk restore COA dari backup (jika diperlukan):")
	fmt.Println("   go run cmd/restore_coa_from_backup.go")
}

// showSoftDeletedDataSummary menampilkan summary data yang soft deleted
func showSoftDeletedDataSummary(db *gorm.DB) {
	fmt.Println("📊 Data yang dapat dipulihkan (soft deleted):")

	// Ambil semua tabel yang memiliki kolom deleted_at
	type tbl struct{ TableName string }
	var tables []tbl
	db.Raw(`
		SELECT table_name AS table_name
		FROM information_schema.columns
		WHERE table_schema = 'public' AND column_name = 'deleted_at'
		ORDER BY table_name
	`).Scan(&tables)

	// Daftar pengecualian (tabel sistem/backup)
	exclude := map[string]bool{
		"accounts_backup":            true,
		"accounts_hierarchy_backup":  true,
		"accounts_original_balances": true,
		"schema_migrations":          true,
		"gorm_migrations":            true,
	}

	totalSoftDeleted := 0
	for _, t := range tables {
		name := t.TableName
		if exclude[name] {
			continue
		}

		// Hitung record yang soft deleted
		var count int64
		db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE deleted_at IS NOT NULL", name)).Scan(&count)

		if count > 0 {
			fmt.Printf("   - %s: %d record(s) dapat dipulihkan\n", name, count)
			totalSoftDeleted += int(count)
		}
	}

	if totalSoftDeleted == 0 {
		fmt.Println("   ℹ Tidak ada data yang soft deleted (tidak ada yang bisa dipulihkan)")
	} else {
		fmt.Printf("\n🔢 Total: %d record dapat dipulihkan\n", totalSoftDeleted)
	}
}

func showBackupInfo(db *gorm.DB) {
	fmt.Println("\n📦 Tabel backup yang dibuat:")

	// Check backup tables safely
	backupTables := []string{"accounts_backup", "accounts_hierarchy_backup", "accounts_original_balances"}

	for _, tableName := range backupTables {
		var count int64
		err := db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&count).Error

		if err != nil {
			fmt.Printf("   - %s: ⚠️  tidak dapat dihitung (mungkin tidak ada)\n", tableName)
		} else {
			fmt.Printf("   - %s: %d records\n", tableName, count)
		}
	}
}

// showPeriodClosingSummary menampilkan summary data periode closing
func showPeriodClosingSummary(db *gorm.DB) {
	var totalPeriods int64
	var closedPeriods int64
	var lockedPeriods int64

	db.Model(&models.AccountingPeriod{}).Count(&totalPeriods)
	db.Model(&models.AccountingPeriod{}).Where("is_closed = ?", true).Count(&closedPeriods)
	db.Model(&models.AccountingPeriod{}).Where("is_locked = ?", true).Count(&lockedPeriods)

	fmt.Printf("📊 Data Periode Closing saat ini:\n")
	fmt.Printf("   Total Periode: %d\n", totalPeriods)
	fmt.Printf("   Periode yang sudah di-close: %d\n", closedPeriods)
	fmt.Printf("   Periode yang di-lock: %d\n", lockedPeriods)

	if totalPeriods > 0 {
		fmt.Println("\n📋 Detail periode closing:")
		var periods []models.AccountingPeriod
		db.Order("start_date DESC").Limit(10).Find(&periods)

		for _, period := range periods {
			status := "Open"
			if period.IsClosed {
				status = "Closed"
			}
			if period.IsLocked {
				status = "Locked"
			}
			fmt.Printf("   - ID %d: %s s/d %s [%s] - %s\n",
				period.ID,
				period.StartDate.Format("2006-01-02"),
				period.EndDate.Format("2006-01-02"),
				status,
				period.Description)
		}

		if totalPeriods > 10 {
			fmt.Printf("   ... dan %d periode lainnya\n", totalPeriods-10)
		}
	}
}

// executeDeletePeriodClosingWithGORM menghapus semua data periode closing
func executeDeletePeriodClosingWithGORM(db *gorm.DB) error {
	start := time.Now()
	fmt.Printf("⏳ Menghapus semua data periode closing... (dimulai: %s)\n", start.Format("15:04:05"))

	tx := db.Session(&gorm.Session{SkipHooks: true}).Begin()

	// Count dulu sebelum dihapus
	var countBefore int64
	tx.Model(&models.AccountingPeriod{}).Count(&countBefore)
	fmt.Printf("   📊 Total periode closing yang akan dihapus: %d\n", countBefore)

	// Hapus dengan Unscoped (hard delete)
	if err := tx.Unscoped().Where("1 = 1").Delete(&models.AccountingPeriod{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("gagal hapus accounting_periods: %v", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("gagal commit hapus periode closing: %v", err)
	}

	// Reset sequence
	fmt.Println("   🔢 Reset sequence accounting_periods...")
	var exists bool
	db.Raw("SELECT EXISTS (SELECT 1 FROM pg_class WHERE relname = 'accounting_periods_id_seq' AND relkind = 'S')").Scan(&exists)

	if exists {
		if err := db.Exec("ALTER SEQUENCE accounting_periods_id_seq RESTART WITH 1").Error; err != nil {
			fmt.Printf("   ⚠️  Warning: Gagal reset sequence accounting_periods_id_seq: %v\n", err)
		}
	}

	duration := time.Since(start)
	fmt.Printf("✅ Hapus periode closing selesai dalam waktu: %s\n", duration.String())
	fmt.Printf("   📊 Jumlah periode yang dihapus: %d\n", countBefore)

	// Verify
	var countAfter int64
	db.Model(&models.AccountingPeriod{}).Count(&countAfter)
	if countAfter == 0 {
		fmt.Println("   ✅ Verifikasi: Tabel accounting_periods sudah bersih")
	} else {
		fmt.Printf("   ⚠️  Warning: Masih ada %d periode tersisa\n", countAfter)
	}

	return nil
}

// executeTruncateAndResetIDWithGORM melakukan TRUNCATE CASCADE pada semua tabel transaksi
// dan mereset semua sequence ID dari 1
func executeTruncateAndResetIDWithGORM(db *gorm.DB) error {
	start := time.Now()
	fmt.Printf("⏳ Melakukan TRUNCATE & RESET ID... (dimulai: %s)\n", start.Format("15:04:05"))

	// Daftar tabel yang akan di-TRUNCATE (urutan tidak penting karena CASCADE)
	tablesToTruncate := []string{
		// SSOT Journal System
		"journal_event_log",
		"unified_journal_lines",
		"unified_journal_ledger",
		"simple_ssot_journal_items",
		"simple_ssot_journals",
		// Classic Journal System
		"journal_lines",
		"journal_entries",
		"journals",
		// Sales related
		"sale_return_items",
		"sale_returns",
		"sale_payments",
		"sale_items",
		"sales",
		// Purchase related
		"purchase_receipt_items",
		"purchase_receipts",
		"purchase_documents",
		"purchase_items",
		"purchases",
		// Other transactions
		"payment_allocations",
		"cash_bank_transactions",
		"transactions",
		"payments",
		"expenses",
		"inventories",
		// Support data
		"notifications",
		"stock_alerts",
		"activity_logs",
		"error_logs",
		// Security & Auth Logs (Must be cleared if sequences are reset)
		"user_sessions",
		"auth_attempts",
		"security_incidents",
		"refresh_tokens",
		"blacklisted_tokens",
		// Approval system
		"approval_histories",
		"approval_actions",
		"approval_requests",
		// Accounting periods
		"accounting_periods",
	}

	// Disable triggers and FK checks
	fmt.Println("   🔧 Disabling triggers and FK constraints...")
	if err := db.Exec("SET session_replication_role = 'replica'").Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Gagal disable triggers: %v\n", err)
	}

	// TRUNCATE each table
	truncatedCount := 0
	for _, table := range tablesToTruncate {
		// Check if table exists first
		var exists bool
		db.Raw("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = ?)", table).Scan(&exists)

		if exists {
			fmt.Printf("   🗑️  TRUNCATE: %s...\n", table)
			if err := db.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)).Error; err != nil {
				fmt.Printf("   ⚠️  Warning: Gagal truncate %s: %v\n", table, err)
			} else {
				truncatedCount++
			}
		} else {
			fmt.Printf("   ⏭️  Skip (not exists): %s\n", table)
		}
	}

	// Re-enable triggers
	fmt.Println("   🔧 Re-enabling triggers and FK constraints...")
	if err := db.Exec("SET session_replication_role = 'origin'").Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Gagal re-enable triggers: %v\n", err)
	}

	// Reset balances on master tables
	fmt.Println("   🔄 Reset balance accounts ke 0...")
	if err := db.Model(&models.Account{}).Where("id > 0").Update("balance", 0).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Reset balance accounts gagal: %v\n", err)
	}

	fmt.Println("   🔄 Reset balance cash banks ke 0...")
	if err := db.Model(&models.CashBank{}).Where("id > 0").Update("balance", 0).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Reset balance cash_banks gagal: %v\n", err)
	}

	fmt.Println("   🔄 Reset stock products ke 0...")
	if err := db.Model(&models.Product{}).Where("id > 0").Update("stock", 0).Error; err != nil {
		fmt.Printf("   ⚠️  Warning: Reset stock products gagal: %v\n", err)
	}

	// Reset ALL sequences
	fmt.Println("   🔢 Reset ALL sequences dari 1...")
	resetAllSequences(db)

	duration := time.Since(start)
	fmt.Printf("✅ TRUNCATE & RESET ID selesai dalam waktu: %s\n", duration.String())
	fmt.Printf("   📊 Jumlah tabel di-truncate: %d\n", truncatedCount)

	// Clean up legacy accounts as well
	if err := cleanUnusedAccounts(db); err != nil {
		fmt.Printf("   ⚠️  Warning: Gagal membersihkan akun lama: %v\n", err)
	}

	// Log activity
	logResetActivity(db)

	return nil
}

// resetAllSequences resets ALL sequences in the database to 1
func resetAllSequences(db *gorm.DB) {
	// Get all sequences in public schema
	type seqInfo struct {
		SeqName string
	}
	var sequences []seqInfo
	db.Raw(`
		SELECT c.relname as seq_name
		FROM pg_class c
		WHERE c.relkind = 'S'
		  AND c.relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = 'public')
	`).Scan(&sequences)

	resetCount := 0
	for _, seq := range sequences {
		// Skip master tables (accounts, products, contacts, cash_banks, users, etc.)
		if shouldSkipSequence(seq.SeqName) {
			fmt.Printf("   ⏭️  Skip master: %s\n", seq.SeqName)
			continue
		}

		if err := db.Exec(fmt.Sprintf("ALTER SEQUENCE %s RESTART WITH 1", seq.SeqName)).Error; err != nil {
			fmt.Printf("   ⚠️  Warning: Gagal reset sequence %s: %v\n", seq.SeqName, err)
		} else {
			fmt.Printf("   ✅ Reset: %s\n", seq.SeqName)
			resetCount++
		}
	}

	fmt.Printf("   📊 Total sequences direset: %d\n", resetCount)
}

// shouldSkipSequence returns true if the sequence should not be reset (master tables)
func shouldSkipSequence(seqName string) bool {
	// Master tables that should NOT have their sequences reset
	masterSequences := []string{
		"accounts_id_seq",
		"products_id_seq",
		"contacts_id_seq",
		"cash_banks_id_seq",
		"users_id_seq",
		"roles_id_seq",
		"permissions_id_seq",
		"role_permissions_id_seq",
		"user_roles_id_seq",
		"settings_id_seq",
		"categories_id_seq",
		"units_id_seq",
		"warehouses_id_seq",
		"tax_settings_id_seq",
		"tax_account_settings_id_seq",
		"audit_logs_id_seq",
		// Backup tables
		"accounts_backup_id_seq",
		"accounts_hierarchy_backup_id_seq",
		"accounts_original_balances_id_seq",
	}

	for _, master := range masterSequences {
		if seqName == master {
			return true
		}
	}
	return false
}
