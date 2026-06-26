package database

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"log"

	"gorm.io/gorm"
)

// BackfillPurchasePaymentLinks restores the purchase_payments cross-reference
// for completed purchase payments that already exist in payment_allocations.
func BackfillPurchasePaymentLinks(db *gorm.DB) {
	if !db.Migrator().HasTable(&models.PurchasePayment{}) {
		log.Println("Skipping purchase payment link backfill: purchase_payments table does not exist")
		return
	}

	if db.Migrator().HasTable(&models.PaymentAllocation{}) && db.Migrator().HasTable(&models.Payment{}) {
		result := db.Exec(`
			INSERT INTO purchase_payments (
				purchase_id,
				payment_number,
				date,
				amount,
				method,
				reference,
				notes,
				cash_bank_id,
				user_id,
				payment_id,
				created_at,
				updated_at
			)
			SELECT
				pa.bill_id,
				p.code,
				p.date,
				pa.allocated_amount,
				p.method,
				p.reference,
				p.notes,
				p.cash_bank_id,
				p.user_id,
				p.id,
				COALESCE(pa.created_at, CURRENT_TIMESTAMP),
				CURRENT_TIMESTAMP
			FROM payment_allocations pa
			JOIN payments p ON p.id = pa.payment_id AND p.deleted_at IS NULL
			JOIN purchases pu ON pu.id = pa.bill_id AND pu.deleted_at IS NULL
			WHERE pa.bill_id IS NOT NULL
				AND p.status = ?
				AND NOT EXISTS (
					SELECT 1
					FROM purchase_payments pp
					WHERE pp.purchase_id = pa.bill_id
						AND pp.payment_id = p.id
						AND pp.deleted_at IS NULL
				)
		`, models.PaymentStatusCompleted)

		if result.Error != nil {
			log.Printf("Warning: purchase payment link backfill failed: %v", result.Error)
		}

		if result.RowsAffected > 0 {
			log.Printf("Backfilled %d purchase payment link(s)", result.RowsAffected)
		}
	}

	if !db.Migrator().HasTable(&models.CashBankTransaction{}) {
		return
	}

	cashBankBookAmountColumn := "cbt.amount"
	if columnExists(db, "cash_bank_transactions", "book_amount_idr") {
		cashBankBookAmountColumn = "cbt.book_amount_idr"
	} else if columnExists(db, "cash_bank_transactions", "book_amount_id_r") {
		cashBankBookAmountColumn = "cbt.book_amount_id_r"
	}

	immediateResult := db.Exec(fmt.Sprintf(`
		INSERT INTO purchase_payments (
			purchase_id,
			payment_number,
			date,
			amount,
			method,
			reference,
			notes,
			cash_bank_id,
			user_id,
			payment_id,
			created_at,
			updated_at
		)
		SELECT
			p.id,
			'PAY-' || REPLACE(p.code, '/', '-'),
			COALESCE(cbt.transaction_date, p.date),
			ABS(COALESCE(NULLIF(%s, 0), NULLIF(cbt.amount, 0), NULLIF(p.paid_amount, 0), p.total_amount)),
			p.payment_method,
			p.payment_reference,
			COALESCE(NULLIF(cbt.notes, ''), 'Pembayaran langsung purchase ' || p.code),
			cbt.cash_bank_id,
			p.user_id,
			NULL,
			CURRENT_TIMESTAMP,
			CURRENT_TIMESTAMP
		FROM purchases p
		LEFT JOIN cash_bank_transactions cbt
			ON cbt.id = (
				SELECT cbt2.id
				FROM cash_bank_transactions cbt2
				WHERE cbt2.reference_type = 'PURCHASE'
					AND cbt2.reference_id = p.id
					AND cbt2.deleted_at IS NULL
				ORDER BY cbt2.id DESC
				LIMIT 1
			)
		WHERE p.deleted_at IS NULL
			AND UPPER(COALESCE(p.payment_method, '')) IN ('CASH', 'BANK_TRANSFER', 'CHECK')
			AND COALESCE(p.paid_amount, 0) > 0
			AND NOT EXISTS (
				SELECT 1
				FROM purchase_payments pp
				WHERE pp.purchase_id = p.id
					AND pp.payment_id IS NULL
					AND pp.deleted_at IS NULL
			)
	`, cashBankBookAmountColumn))
	if immediateResult.Error != nil {
		log.Printf("Warning: immediate purchase payment link backfill failed: %v", immediateResult.Error)
		return
	}
	if immediateResult.RowsAffected > 0 {
		log.Printf("Backfilled %d immediate purchase payment proof link(s)", immediateResult.RowsAffected)
	}
}
