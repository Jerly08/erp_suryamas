package database

import (
	"log"

	"gorm.io/gorm"
)

func RunPaymentSnapshotBackfillMigration(db *gorm.DB) {
	log.Println("Backfilling payment cash/bank snapshot fields...")

	result := db.Exec(`
		UPDATE payments AS p
		SET
			cash_bank_code = cb.code,
			cash_bank_name = cb.name,
			cash_bank_currency = cb.currency,
			cash_bank_gl_account_id = a.id,
			cash_bank_gl_account_code = a.code,
			cash_bank_gl_account_name = a.name
		FROM cash_banks AS cb
		JOIN accounts AS a ON a.id = cb.account_id
		WHERE p.cash_bank_id = cb.id
			AND (
				COALESCE(p.cash_bank_code, '') = '' OR
				COALESCE(p.cash_bank_name, '') = '' OR
				COALESCE(p.cash_bank_currency, '') = '' OR
				p.cash_bank_gl_account_id IS NULL OR
				COALESCE(p.cash_bank_gl_account_code, '') = '' OR
				COALESCE(p.cash_bank_gl_account_name, '') = ''
			)
	`)
	if result.Error != nil {
		log.Printf("Warning: failed to backfill payments cash/bank snapshot fields: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("Backfilled payment cash/bank snapshots for %d rows", result.RowsAffected)
	}

	result = db.Exec(`
		UPDATE sale_payments AS sp
		SET
			cash_bank_code = cb.code,
			cash_bank_name = cb.name,
			cash_bank_currency = cb.currency,
			account_id = COALESCE(sp.account_id, a.id),
			cash_bank_gl_account_id = a.id,
			cash_bank_gl_account_code = a.code,
			cash_bank_gl_account_name = a.name
		FROM cash_banks AS cb
		JOIN accounts AS a ON a.id = cb.account_id
		WHERE sp.cash_bank_id = cb.id
			AND (
				COALESCE(sp.cash_bank_code, '') = '' OR
				COALESCE(sp.cash_bank_name, '') = '' OR
				COALESCE(sp.cash_bank_currency, '') = '' OR
				sp.cash_bank_gl_account_id IS NULL OR
				COALESCE(sp.cash_bank_gl_account_code, '') = '' OR
				COALESCE(sp.cash_bank_gl_account_name, '') = '' OR
				sp.account_id IS NULL
			)
	`)
	if result.Error != nil {
		log.Printf("Warning: failed to backfill sale_payments cash/bank snapshot fields: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("Backfilled sale payment cash/bank snapshots for %d rows", result.RowsAffected)
	}
}

func RenameLegacyCOGSAccountLabel(db *gorm.DB) {
	log.Println("Normalizing legacy COGS account labels...")

	result := db.Exec(`
		UPDATE accounts
		SET
			name = 'HARGA POKOK PENJUALAN',
			description = CASE
				WHEN COALESCE(description, '') = '' THEN 'Akun Harga Pokok Penjualan (COGS)'
				ELSE description
			END
		WHERE code = '5101'
			AND deleted_at IS NULL
			AND UPPER(TRIM(name)) = 'PEMBELIAN BARANG DAGANG'
	`)
	if result.Error != nil {
		log.Printf("Warning: failed to normalize account 5101 label: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("Normalized legacy COGS label for %d account(s)", result.RowsAffected)
	}
}
