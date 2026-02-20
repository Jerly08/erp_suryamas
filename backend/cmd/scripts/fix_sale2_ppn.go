package main

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Fix SALE-2 PPN values and add missing PPN Keluaran journal entry
func main() {
	db, err := gorm.Open(postgres.Open("host=localhost user=postgres password=postgres dbname=sistem_akuntansi port=5432 sslmode=disable"), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== Fix SALE-2 PPN Bug ===")

	// Step 1: Get SALE-2 current values
	type SaleInfo struct {
		ID                uint
		Subtotal          float64
		PPN               float64
		PPNPercent        float64
		PPNAmount         float64
		PPNRate           float64
		TaxableAmount     float64
		TotalAmount       float64
		DiscountAmount    float64
		OutstandingAmount float64
		PaidAmount        float64
	}
	var sale SaleInfo
	db.Raw(`SELECT id, subtotal, ppn, ppn_percent, ppn_amount, ppn_rate, 
		taxable_amount, total_amount, discount_amount, outstanding_amount, paid_amount
		FROM sales WHERE id = 2`).Scan(&sale)
	fmt.Printf("Before: PPN=%.2f, PPNPercent=%.2f, Subtotal=%.2f, TotalAmount=%.2f\n", sale.PPN, sale.PPNPercent, sale.Subtotal, sale.TotalAmount)

	if sale.PPN > 0 {
		fmt.Println("PPN already calculated, checking journal lines...")
	} else if sale.PPNPercent > 0 {
		// Recalculate PPN
		taxableAmt := sale.TaxableAmount
		if taxableAmt == 0 {
			taxableAmt = sale.Subtotal - sale.DiscountAmount
		}
		ppn := taxableAmt * (sale.PPNPercent / 100)
		totalAmount := taxableAmt + ppn
		outstanding := totalAmount - sale.PaidAmount

		fmt.Printf("Recalculating: TaxableAmt=%.2f, PPNRate=%.2f%%, PPN=%.2f\n", taxableAmt, sale.PPNPercent, ppn)

		result := db.Exec(`
			UPDATE sales SET 
				ppn = ?, ppn_amount = ?, ppn_rate = ?,
				total_amount = ?, outstanding_amount = ?,
				updated_at = NOW()
			WHERE id = 2
		`, ppn, ppn, sale.PPNPercent, totalAmount, outstanding)
		if result.Error != nil {
			log.Printf("Failed to update sale: %v", result.Error)
		} else {
			fmt.Printf("Updated SALE-2: PPN=%.2f, TotalAmount=%.2f, Outstanding=%.2f\n", ppn, totalAmount, outstanding)
		}
	}

	// Step 2: Check if PPN Keluaran journal line exists for SALE-2
	var ppnLineCount int64
	db.Raw(`
		SELECT COUNT(*) FROM unified_journal_lines ujl
		JOIN unified_journal_ledger ujle ON ujl.journal_id = ujle.id
		WHERE ujle.source_type = 'SALE' AND ujle.source_id = 2
		  AND ujl.description LIKE '%PPN%'
	`).Scan(&ppnLineCount)
	fmt.Printf("\nExisting PPN journal lines for SALE-2: %d\n", ppnLineCount)

	if ppnLineCount == 0 {
		// Get PPN Keluaran account ID
		type AccInfo struct {
			ID uint
		}
		var ppnAcc AccInfo
		db.Raw("SELECT id FROM accounts WHERE code = '2102-001' AND deleted_at IS NULL").Scan(&ppnAcc)
		if ppnAcc.ID == 0 {
			db.Raw("SELECT id FROM accounts WHERE code = '2103' AND deleted_at IS NULL").Scan(&ppnAcc)
		}
		if ppnAcc.ID == 0 {
			log.Fatal("PPN Keluaran account not found!")
		}

		// Get the journal ID for SALE-2
		var journalID uint
		db.Raw(`SELECT id FROM unified_journal_ledger WHERE source_type = 'SALE' AND source_id = 2 ORDER BY id LIMIT 1`).Scan(&journalID)
		if journalID == 0 {
			log.Fatal("No journal found for SALE-2!")
		}

		// Get the max line_number for this journal
		var maxLineNum int
		db.Raw(`SELECT COALESCE(MAX(line_number), 0) FROM unified_journal_lines WHERE journal_id = ?`, journalID).Scan(&maxLineNum)

		// Calculate PPN value
		var ppnVal float64
		db.Raw(`SELECT ppn FROM sales WHERE id = 2`).Scan(&ppnVal)
		if ppnVal == 0 {
			// Recalculate from scratch
			var s2 SaleInfo
			db.Raw(`SELECT subtotal, ppn_percent, discount_amount, taxable_amount FROM sales WHERE id = 2`).Scan(&s2)
			taxable := s2.TaxableAmount
			if taxable == 0 {
				taxable = s2.Subtotal - s2.DiscountAmount
			}
			ppnVal = taxable * (s2.PPNPercent / 100)
		}

		fmt.Printf("Adding PPN Keluaran entry: JournalID=%d, AccountID=%d, Credit=%.2f\n", journalID, ppnAcc.ID, ppnVal)

		// Insert PPN Keluaran line
		result := db.Exec(`
			INSERT INTO unified_journal_lines (journal_id, account_id, line_number, description, debit_amount, credit_amount, created_at, updated_at)
			VALUES (?, ?, ?, 'PPN Keluaran - 0005/STA-B/II-2026', 0, ?, NOW(), NOW())
		`, journalID, ppnAcc.ID, maxLineNum+1, ppnVal)
		if result.Error != nil {
			log.Printf("Failed to insert PPN line: %v", result.Error)
		} else {
			fmt.Printf("✅ Added PPN Keluaran journal line: Credit %.2f\n", ppnVal)
		}

		// Also update the journal totals
		db.Exec(`
			UPDATE unified_journal_ledger
			SET total_credit = total_credit + ?, updated_at = NOW()
			WHERE id = ?
		`, ppnVal, journalID)

		// Also need to update debit total (receivable should increase)
		// For credit sale, debit is Piutang Usaha — need to increase it too
		var piutangLineID uint
		db.Raw(`
			SELECT ujl.id FROM unified_journal_lines ujl
			WHERE ujl.journal_id = ? AND ujl.debit_amount > 0 AND ujl.description LIKE 'Penjualan%'
			LIMIT 1
		`, journalID).Scan(&piutangLineID)

		if piutangLineID > 0 {
			db.Exec(`
				UPDATE unified_journal_lines 
				SET debit_amount = debit_amount + ?, updated_at = NOW()
				WHERE id = ?
			`, ppnVal, piutangLineID)
			db.Exec(`
				UPDATE unified_journal_ledger
				SET total_debit = total_debit + ?, updated_at = NOW()
				WHERE id = ?
			`, ppnVal, journalID)
			fmt.Printf("✅ Updated Piutang Usaha debit by +%.2f (line #%d)\n", ppnVal, piutangLineID)
		}
	}

	// Verification
	fmt.Println("\n=== Verification ===")
	var sale2 SaleInfo
	db.Raw(`SELECT id, subtotal, ppn, ppn_percent, ppn_amount, total_amount FROM sales WHERE id = 2`).Scan(&sale2)
	fmt.Printf("SALE-2: PPN=%.2f, PPNPercent=%.2f, TotalAmount=%.2f\n", sale2.PPN, sale2.PPNPercent, sale2.TotalAmount)

	type JLine struct {
		ID           uint
		AccountID    uint
		DebitAmount  float64
		CreditAmount float64
		Description  string
	}
	var lines []JLine
	db.Raw(`
		SELECT ujl.id, ujl.account_id, ujl.debit_amount, ujl.credit_amount, ujl.description
		FROM unified_journal_lines ujl
		JOIN unified_journal_ledger ujle ON ujl.journal_id = ujle.id
		WHERE ujle.source_type = 'SALE' AND ujle.source_id = 2
		ORDER BY ujl.id
	`).Scan(&lines)
	fmt.Printf("Journal lines for SALE-2: %d\n", len(lines))
	for _, line := range lines {
		var code, name string
		db.Raw("SELECT code, name FROM accounts WHERE id = ?", line.AccountID).Row().Scan(&code, &name)
		fmt.Printf("  J#%d Acc=%d (%s - %s) D:%.0f C:%.0f | %s\n", line.ID, line.AccountID, code, name, line.DebitAmount, line.CreditAmount, line.Description)
	}
}
