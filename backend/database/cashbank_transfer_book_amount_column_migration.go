package database

import (
	"log"

	"gorm.io/gorm"
)

// CashBankTransferBookAmountColumnMigration aligns legacy cash_bank_transfers
// schemas with the explicit *_idr column names used by the approval flow.
func CashBankTransferBookAmountColumnMigration(db *gorm.DB) {
	if db == nil || !db.Migrator().HasTable("cash_bank_transfers") {
		return
	}

	if !columnExists(db, "cash_bank_transfers", "source_book_amount_idr") {
		if err := db.Exec(`
			ALTER TABLE cash_bank_transfers
			ADD COLUMN IF NOT EXISTS source_book_amount_idr DECIMAL(20,2) DEFAULT 0
		`).Error; err != nil {
			log.Printf("Warning: failed adding cash_bank_transfers.source_book_amount_idr: %v", err)
		}
	}

	if !columnExists(db, "cash_bank_transfers", "destination_book_amount_idr") {
		if err := db.Exec(`
			ALTER TABLE cash_bank_transfers
			ADD COLUMN IF NOT EXISTS destination_book_amount_idr DECIMAL(20,2) DEFAULT 0
		`).Error; err != nil {
			log.Printf("Warning: failed adding cash_bank_transfers.destination_book_amount_idr: %v", err)
		}
	}

	if columnExists(db, "cash_bank_transfers", "source_book_amount_id_r") {
		if err := db.Exec(`
			UPDATE cash_bank_transfers
			SET source_book_amount_idr = source_book_amount_id_r
			WHERE COALESCE(source_book_amount_idr, 0) = 0
			  AND COALESCE(source_book_amount_id_r, 0) <> 0
		`).Error; err != nil {
			log.Printf("Warning: failed backfilling source_book_amount_idr from legacy column: %v", err)
		}
	}

	if columnExists(db, "cash_bank_transfers", "destination_book_amount_id_r") {
		if err := db.Exec(`
			UPDATE cash_bank_transfers
			SET destination_book_amount_idr = destination_book_amount_id_r
			WHERE COALESCE(destination_book_amount_idr, 0) = 0
			  AND COALESCE(destination_book_amount_id_r, 0) <> 0
		`).Error; err != nil {
			log.Printf("Warning: failed backfilling destination_book_amount_idr from legacy column: %v", err)
		}
	}

	log.Println("Cash bank transfer book amount column migration completed")
}
