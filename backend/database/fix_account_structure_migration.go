package database

import (
	"app-sistem-akuntansi/models"
	"gorm.io/gorm"
	"log"
)

// FixAccountStructureMigration ensures proper account structure and automatic balance sync
func FixAccountStructureMigration(db *gorm.DB) {
	log.Println("🔧 Running Account Structure Fix Migration...")

	// 1. Fix PPN account hierarchy and type
	fixPPNAccountStructure(db)

	// 2. Ensure correct account types for all accounts
	fixAccountTypes(db)

	// 3. Set up automatic balance synchronization triggers
	setupBalanceSyncTriggers(db)

	// 4. Run initial balance synchronization
	runInitialBalanceSync(db)

	log.Println("✅ Account Structure Fix Migration completed")
}

// fixPPNAccountStructure moves PPN accounts to correct hierarchy
func fixPPNAccountStructure(db *gorm.DB) {
	log.Println("  Fixing PPN account structure...")

	// Find Current Liabilities parent account
	var currentLiabilitiesID uint
	err := db.Raw(`SELECT id FROM accounts WHERE code = '2100' AND name = 'CURRENT LIABILITIES'`).Scan(&currentLiabilitiesID).Error
	if err != nil {
		log.Printf("Warning: Could not find Current Liabilities account: %v", err)
		return
	}

	// FIX: Account 2102 was incorrectly created with wrong type
	// Deactivate any existing 2102 account (it should not exist in seed)
	err = db.Exec(`
		UPDATE accounts 
		SET is_active = false, deleted_at = NOW()
		WHERE code = '2102' AND deleted_at IS NULL
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to deactivate incorrect account 2102: %v", err)
	} else {
		log.Println("    ✅ Deactivated incorrect account 2102 (PPN Masukan with liability code)")
	}

	// Ensure correct PPN accounts exist:
	// - 1240: PPN MASUKAN (Asset - Input VAT)
	// - 2103: PPN KELUARAN (Liability - Output VAT)

	// Check if we need to fix any misnamed PPN accounts
	var wrongPPNAccounts []struct {
		ID   uint
		Code string
		Name string
		Type string
	}

	db.Raw(`
		SELECT id, code, name, type 
		FROM accounts 
		WHERE deleted_at IS NULL
		  AND (
			  (code LIKE '2%' AND UPPER(name) LIKE '%PPN MASUKAN%') OR
			  (code LIKE '1%' AND UPPER(name) LIKE '%PPN KELUARAN%')
		  )
	`).Scan(&wrongPPNAccounts)

	for _, acc := range wrongPPNAccounts {
		log.Printf("    ⚠️  Found misplaced PPN account: %s - %s (Type: %s)", acc.Code, acc.Name, acc.Type)
		log.Printf("       Please review and fix manually if needed")
	}
}

// fixAccountTypes ensures all accounts have correct types
func fixAccountTypes(db *gorm.DB) {
	log.Println("  Fixing account types...")

	accountTypeFixes := []struct {
		Code string
		Type string
	}{
		{"1000", "ASSET"},     // ASSETS header
		{"1100", "ASSET"},     // CURRENT ASSETS header
		{"1200", "ASSET"},     // ACCOUNTS RECEIVABLE header
		{"1201", "ASSET"},     // Piutang Usaha
		{"1500", "ASSET"},     // FIXED ASSETS header
		{"1240", "ASSET"},     // PPN MASUKAN (Input VAT)
		{"2000", "LIABILITY"}, // LIABILITIES header
		{"2100", "LIABILITY"}, // CURRENT LIABILITIES header
		{"2101", "LIABILITY"}, // Utang Usaha
		{"2103", "LIABILITY"}, // PPN KELUARAN (Output VAT)
		{"3000", "EQUITY"},    // EQUITY header
		{"3101", "EQUITY"},    // Modal Pemilik
		{"3105", "EQUITY"},    // Ekuitas Saldo Awal
		{"3201", "EQUITY"},    // Laba Ditahan
		{"4000", "REVENUE"},   // REVENUE header
		{"4101", "REVENUE"},   // Pendapatan Penjualan
		{"4201", "REVENUE"},   // Pendapatan Lain-lain
		{"5000", "EXPENSE"},   // EXPENSES header
		{"5101", "EXPENSE"},   // Harga Pokok Penjualan
		{"5201", "EXPENSE"},   // Beban Gaji
	}

	for _, fix := range accountTypeFixes {
		err := db.Model(&models.Account{}).
			Where("code = ? AND deleted_at IS NULL", fix.Code).
			Update("type", fix.Type).Error

		if err != nil {
			log.Printf("    Warning: Failed to fix type for account %s: %v", fix.Code, err)
		}
	}

	log.Println("    ✅ Fixed account types")
}

// setupBalanceSyncTriggers creates database triggers for automatic balance sync
func setupBalanceSyncTriggers(db *gorm.DB) {
	log.Println("  Setting up balance sync triggers...")

	// Create a function that updates account balance when journal lines change
	err := db.Exec(`
		CREATE OR REPLACE FUNCTION update_account_balance_from_ssot()
		RETURNS TRIGGER AS $$
		BEGIN
			-- Update the affected account's balance
			UPDATE accounts 
			SET balance = (
				SELECT COALESCE(SUM(debit_amount) - SUM(credit_amount), 0)
				FROM unified_journal_lines ujl
				JOIN unified_journal_ledger uj ON ujl.journal_id = uj.id
				WHERE ujl.account_id = COALESCE(NEW.account_id, OLD.account_id)
				AND uj.status = 'POSTED'
			)
			WHERE id = COALESCE(NEW.account_id, OLD.account_id);

			-- Update parent account balances
			WITH RECURSIVE parent_chain AS (
				-- Start with the direct parent of the affected account
				SELECT parent_id as account_id
				FROM accounts 
				WHERE id = COALESCE(NEW.account_id, OLD.account_id) 
				AND parent_id IS NOT NULL
				
				UNION ALL
				
				-- Recursively get parent's parents
				SELECT a.parent_id 
				FROM accounts a
				JOIN parent_chain pc ON a.id = pc.account_id
				WHERE a.parent_id IS NOT NULL
			)
			UPDATE accounts 
			SET balance = (
				SELECT COALESCE(SUM(child.balance), 0)
				FROM accounts child 
				WHERE child.parent_id = accounts.id 
				AND child.deleted_at IS NULL
			)
			WHERE accounts.id IN (SELECT account_id FROM parent_chain)
			AND accounts.is_header = true;

			RETURN COALESCE(NEW, OLD);
		END;
		$$ LANGUAGE plpgsql;
	`).Error

	if err != nil {
		log.Printf("Warning: Failed to create balance sync function: %v", err)
		return
	}

	// Drop existing triggers if they exist
	db.Exec(`DROP TRIGGER IF EXISTS trigger_update_balance_on_journal_line_insert ON unified_journal_lines`)
	db.Exec(`DROP TRIGGER IF EXISTS trigger_update_balance_on_journal_line_update ON unified_journal_lines`)
	db.Exec(`DROP TRIGGER IF EXISTS trigger_update_balance_on_journal_line_delete ON unified_journal_lines`)

	// Create triggers for INSERT, UPDATE, DELETE on journal lines
	triggers := []string{
		`CREATE TRIGGER trigger_update_balance_on_journal_line_insert
		AFTER INSERT ON unified_journal_lines
		FOR EACH ROW EXECUTE FUNCTION update_account_balance_from_ssot()`,

		`CREATE TRIGGER trigger_update_balance_on_journal_line_update
		AFTER UPDATE ON unified_journal_lines
		FOR EACH ROW EXECUTE FUNCTION update_account_balance_from_ssot()`,

		`CREATE TRIGGER trigger_update_balance_on_journal_line_delete
		AFTER DELETE ON unified_journal_lines
		FOR EACH ROW EXECUTE FUNCTION update_account_balance_from_ssot()`,
	}

	for _, trigger := range triggers {
		err = db.Exec(trigger).Error
		if err != nil {
			log.Printf("Warning: Failed to create trigger: %v", err)
		}
	}

	log.Println("    ✅ Balance sync triggers created")
}

// runInitialBalanceSync runs initial balance synchronization
func runInitialBalanceSync(db *gorm.DB) {
	log.Println("  Running initial balance synchronization...")

	// 1. Reset all balances to zero
	err := db.Exec("UPDATE accounts SET balance = 0 WHERE deleted_at IS NULL").Error
	if err != nil {
		log.Printf("Warning: Failed to reset balances: %v", err)
		return
	}

	// 2. Calculate and update balances from SSOT
	var accountUpdates []struct {
		AccountID         uint    `json:"account_id"`
		CalculatedBalance float64 `json:"calculated_balance"`
	}

	err = db.Raw(`
		SELECT 
			account_id,
			SUM(debit_amount) - SUM(credit_amount) as calculated_balance
		FROM unified_journal_lines ujl
		JOIN unified_journal_ledger uj ON ujl.journal_id = uj.id
		WHERE uj.status = 'POSTED'
		GROUP BY account_id
	`).Scan(&accountUpdates).Error

	if err != nil {
		log.Printf("Warning: Failed to calculate SSOT balances: %v", err)
		return
	}

	// 3. Update account balances
	for _, update := range accountUpdates {
		err = db.Model(&models.Account{}).
			Where("id = ?", update.AccountID).
			Update("balance", update.CalculatedBalance).Error

		if err != nil {
			log.Printf("Warning: Failed to update account %d balance: %v", update.AccountID, err)
		}
	}

	// 4. Update parent balances
	for pass := 0; pass < 5; pass++ {
		err = db.Exec(`
			UPDATE accounts 
			SET balance = (
				SELECT COALESCE(SUM(child.balance), 0)
				FROM accounts child 
				WHERE child.parent_id = accounts.id AND child.deleted_at IS NULL
			)
			WHERE is_header = true AND deleted_at IS NULL
		`).Error

		if err != nil {
			log.Printf("Warning: Failed to update parent balances (pass %d): %v", pass+1, err)
		}
	}

	log.Printf("    ✅ Initial balance sync completed (%d accounts updated)", len(accountUpdates))
}
