package database

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// RunCashBankCOASyncMigration runs the cash bank COA sync migration
// This is idempotent - safe to run multiple times
func RunCashBankCOASyncMigration(db *gorm.DB) error {
	log.Println("🔧 Running Cash Bank-COA Sync Migration...")

	// Step 1: Check if already migrated
	if isMigrationAlreadyRun(db) {
		log.Println("✅ Cash Bank-COA Sync Migration already applied, skipping...")
		return nil
	}

	// Step 2: Create unique index (idempotent)
	if err := createUniqueIndex(db); err != nil {
		return fmt.Errorf("failed to create unique index: %v", err)
	}

	// Step 3: Create validation function (idempotent)
	if err := createValidationFunction(db); err != nil {
		return fmt.Errorf("failed to create validation function: %v", err)
	}

	// Step 4: Create validation trigger (idempotent)
	if err := createValidationTrigger(db); err != nil {
		return fmt.Errorf("failed to create validation trigger: %v", err)
	}

	// Step 5: Create sync function (idempotent)
	if err := createSyncFunction(db); err != nil {
		return fmt.Errorf("failed to create sync function: %v", err)
	}

	// Step 6: Create sync trigger (idempotent)
	if err := createSyncTrigger(db); err != nil {
		return fmt.Errorf("failed to create sync trigger: %v", err)
	}

	// Step 7: Create performance index (idempotent)
	if err := createPerformanceIndex(db); err != nil {
		return fmt.Errorf("failed to create performance index: %v", err)
	}

	// Step 8: Create reverse sync function (idempotent)
	if err := createReverseSyncFunction(db); err != nil {
		return fmt.Errorf("failed to create reverse sync function: %v", err)
	}

	// Step 9: Create reverse sync trigger (idempotent)
	if err := createReverseSyncTrigger(db); err != nil {
		return fmt.Errorf("failed to create reverse sync trigger: %v", err)
	}

	log.Println("✅ Cash Bank-COA Sync Migration completed successfully!")
	return nil
}

// isMigrationAlreadyRun checks if the migration has already been applied
func isMigrationAlreadyRun(db *gorm.DB) bool {
	var exists bool

	// Check if unique index exists
	db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes 
			WHERE indexname = 'cash_banks_account_id_unique_idx'
		)
	`).Scan(&exists)

	if !exists {
		return false
	}

	// Check if sync trigger exists
	db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_trigger 
			WHERE tgname = 'trg_sync_coa_balance_from_cashbank'
		)
	`).Scan(&exists)

	if !exists {
		return false
	}

	// Check if reverse sync trigger exists
	db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_trigger 
			WHERE tgname = 'trg_sync_cashbank_balance_from_coa'
		)
	`).Scan(&exists)

	return exists
}

// createUniqueIndex creates unique index on account_id
func createUniqueIndex(db *gorm.DB) error {
	log.Println("  → Creating unique index...")

	// Drop if exists
	db.Exec("DROP INDEX IF EXISTS cash_banks_account_id_unique_idx")

	// Create index
	return db.Exec(`
		CREATE UNIQUE INDEX cash_banks_account_id_unique_idx 
		ON cash_banks (account_id) 
		WHERE (deleted_at IS NULL AND account_id IS NOT NULL)
	`).Error
}

// createValidationFunction creates validation function
func createValidationFunction(db *gorm.DB) error {
	log.Println("  → Creating validation function...")

	return db.Exec(`
		CREATE OR REPLACE FUNCTION validate_cashbank_coa_balance()
		RETURNS TRIGGER AS $$
		DECLARE
		    coa_balance DECIMAL(15,2);
		    cashbank_balance DECIMAL(15,2);
		BEGIN
		    IF NEW.account_id IS NOT NULL THEN
		        SELECT balance INTO coa_balance 
		        FROM accounts 
		        WHERE id = NEW.account_id AND deleted_at IS NULL;
		        
		        cashbank_balance := NEW.balance;
		        
		        IF coa_balance IS NOT NULL AND ABS(coa_balance - cashbank_balance) > 0.01 THEN
		            RAISE NOTICE 'WARNING: Cash Bank % balance (%) differs from COA account % balance (%)', 
		                NEW.code, cashbank_balance, NEW.account_id, coa_balance;
		        END IF;
		    END IF;
		    
		    RETURN NEW;
		END;
		$$ LANGUAGE plpgsql
	`).Error
}

// createValidationTrigger creates validation trigger
func createValidationTrigger(db *gorm.DB) error {
	log.Println("  → Creating validation trigger...")

	// Drop if exists
	db.Exec("DROP TRIGGER IF EXISTS trg_validate_cashbank_coa_balance ON cash_banks")

	// Create trigger
	return db.Exec(`
		CREATE TRIGGER trg_validate_cashbank_coa_balance
		BEFORE UPDATE OF balance ON cash_banks
		FOR EACH ROW
		WHEN (NEW.balance IS DISTINCT FROM OLD.balance)
		EXECUTE FUNCTION validate_cashbank_coa_balance()
	`).Error
}

// createSyncFunction creates auto-sync function
func createSyncFunction(db *gorm.DB) error {
	log.Println("  → Creating auto-sync function...")

	return db.Exec(`
		CREATE OR REPLACE FUNCTION sync_coa_balance_from_cashbank()
		RETURNS TRIGGER AS $$
		BEGIN
		    IF NEW.account_id IS NOT NULL AND NEW.balance IS DISTINCT FROM OLD.balance THEN
		        UPDATE accounts 
		        SET balance = NEW.balance,
		            updated_at = CURRENT_TIMESTAMP
		        WHERE id = NEW.account_id 
		          AND deleted_at IS NULL;
		        
		        RAISE NOTICE 'Synced COA account % balance to %.2f (from cash bank %)', 
		            NEW.account_id, NEW.balance, NEW.code;
		    END IF;
		    
		    RETURN NEW;
		END;
		$$ LANGUAGE plpgsql
	`).Error
}

// createSyncTrigger creates auto-sync trigger
func createSyncTrigger(db *gorm.DB) error {
	log.Println("  → Creating auto-sync trigger...")

	// Drop if exists
	db.Exec("DROP TRIGGER IF EXISTS trg_sync_coa_balance_from_cashbank ON cash_banks")

	// Create trigger
	return db.Exec(`
		CREATE TRIGGER trg_sync_coa_balance_from_cashbank
		AFTER UPDATE OF balance ON cash_banks
		FOR EACH ROW
		WHEN (NEW.balance IS DISTINCT FROM OLD.balance AND NEW.account_id IS NOT NULL)
		EXECUTE FUNCTION sync_coa_balance_from_cashbank()
	`).Error
}

// createPerformanceIndex creates performance index
func createPerformanceIndex(db *gorm.DB) error {
	log.Println("  → Creating performance index...")

	return db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_cash_banks_account_id 
		ON cash_banks(account_id) 
		WHERE deleted_at IS NULL AND account_id IS NOT NULL
	`).Error
}

// createReverseSyncFunction creates auto-sync function from accounts to cash_banks
func createReverseSyncFunction(db *gorm.DB) error {
	log.Println("  → Creating reverse auto-sync function (Accounts -> CashBanks)...")

	return db.Exec(`
		CREATE OR REPLACE FUNCTION sync_cashbank_balance_from_coa()
		RETURNS TRIGGER AS $$
		BEGIN
		    IF NEW.balance IS DISTINCT FROM OLD.balance THEN
		        UPDATE cash_banks 
		        SET balance = NEW.balance,
		            updated_at = CURRENT_TIMESTAMP
		        WHERE account_id = NEW.id 
		          AND deleted_at IS NULL
		          AND balance IS DISTINCT FROM NEW.balance;
		        
		        IF FOUND THEN
					RAISE NOTICE 'Synced cash bank balance to %.2f (from COA account %)', 
						NEW.balance, NEW.id;
		        END IF;
		    END IF;
		    
		    RETURN NEW;
		END;
		$$ LANGUAGE plpgsql
	`).Error
}

// createReverseSyncTrigger creates auto-sync trigger on accounts table
func createReverseSyncTrigger(db *gorm.DB) error {
	log.Println("  → Creating reverse auto-sync trigger on accounts...")

	// Drop if exists
	db.Exec("DROP TRIGGER IF EXISTS trg_sync_cashbank_balance_from_coa ON accounts")

	// Create trigger
	return db.Exec(`
		CREATE TRIGGER trg_sync_cashbank_balance_from_coa
		AFTER UPDATE OF balance ON accounts
		FOR EACH ROW
		WHEN (NEW.balance IS DISTINCT FROM OLD.balance)
		EXECUTE FUNCTION sync_cashbank_balance_from_coa()
	`).Error
}
