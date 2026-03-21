package services

import (
	"fmt"
	"strings"

	"app-sistem-akuntansi/models"

	"gorm.io/gorm"
)

type cashBankSnapshot struct {
	CashBankID        uint
	CashBankCode      string
	CashBankName      string
	CashBankCurrency  string
	GLAccountID       uint
	GLAccountCode     string
	GLAccountName     string
	GLAccountCategory string
}

func loadCashBankSnapshot(tx *gorm.DB, cashBankID uint) (*cashBankSnapshot, error) {
	if tx == nil {
		return nil, fmt.Errorf("database transaction is required")
	}
	if cashBankID == 0 {
		return nil, fmt.Errorf("cash/bank id is required")
	}

	var cashBank models.CashBank
	if err := tx.Preload("Account").First(&cashBank, cashBankID).Error; err != nil {
		return nil, fmt.Errorf("cash/bank account not found: %w", err)
	}
	if cashBank.AccountID == 0 {
		return nil, fmt.Errorf("cash/bank %s has no linked GL account", cashBank.Code)
	}

	account := cashBank.Account
	if account.ID == 0 {
		if err := tx.First(&account, cashBank.AccountID).Error; err != nil {
			return nil, fmt.Errorf("linked GL account for cash/bank %s not found: %w", cashBank.Code, err)
		}
	}

	return &cashBankSnapshot{
		CashBankID:        cashBank.ID,
		CashBankCode:      cashBank.Code,
		CashBankName:      cashBank.Name,
		CashBankCurrency:  normalizeCashBankCurrency(cashBank.Currency),
		GLAccountID:       account.ID,
		GLAccountCode:     account.Code,
		GLAccountName:     account.Name,
		GLAccountCategory: account.Category,
	}, nil
}

func applyCashBankSnapshotToPayment(payment *models.Payment, snapshot *cashBankSnapshot) {
	if payment == nil || snapshot == nil {
		return
	}

	cashBankID := snapshot.CashBankID
	payment.CashBankID = &cashBankID
	glAccountID := snapshot.GLAccountID
	payment.CashBankGLAccountID = &glAccountID
	payment.CashBankCode = snapshot.CashBankCode
	payment.CashBankName = snapshot.CashBankName
	payment.CashBankCurrency = snapshot.CashBankCurrency
	payment.CashBankGLAccountCode = snapshot.GLAccountCode
	payment.CashBankGLAccountName = snapshot.GLAccountName
}

func applyCashBankSnapshotToSalePayment(payment *models.SalePayment, snapshot *cashBankSnapshot) {
	if payment == nil || snapshot == nil {
		return
	}

	cashBankID := snapshot.CashBankID
	payment.CashBankID = &cashBankID
	glAccountID := snapshot.GLAccountID
	payment.AccountID = &glAccountID
	payment.CashBankGLAccountID = &glAccountID
	payment.CashBankCode = snapshot.CashBankCode
	payment.CashBankName = snapshot.CashBankName
	payment.CashBankCurrency = snapshot.CashBankCurrency
	payment.CashBankGLAccountCode = snapshot.GLAccountCode
	payment.CashBankGLAccountName = snapshot.GLAccountName
}

func ensureCashBankSnapshotConsistency(snapshot *cashBankSnapshot, paymentGLAccountID *uint, paymentGLCode, paymentGLName string) error {
	if snapshot == nil {
		return nil
	}

	if paymentGLAccountID != nil && *paymentGLAccountID != 0 && *paymentGLAccountID != snapshot.GLAccountID {
		return fmt.Errorf(
			"cash/bank snapshot mismatch: selected %s posts to GL %s, but payment carries GL account ID %d",
			snapshot.CashBankCode,
			snapshot.GLAccountCode,
			*paymentGLAccountID,
		)
	}

	if paymentGLCode != "" && !strings.EqualFold(strings.TrimSpace(paymentGLCode), strings.TrimSpace(snapshot.GLAccountCode)) {
		return fmt.Errorf(
			"cash/bank snapshot mismatch: selected %s expects GL code %s, but payment carries %s",
			snapshot.CashBankCode,
			snapshot.GLAccountCode,
			paymentGLCode,
		)
	}

	if paymentGLName != "" && !strings.EqualFold(strings.TrimSpace(paymentGLName), strings.TrimSpace(snapshot.GLAccountName)) {
		return fmt.Errorf(
			"cash/bank snapshot mismatch: selected %s expects GL account %s, but payment carries %s",
			snapshot.CashBankCode,
			snapshot.GLAccountName,
			paymentGLName,
		)
	}

	return nil
}

func formatCashBankSnapshotSummary(code, name, currency, glCode, glName string) string {
	parts := make([]string, 0, 3)

	if code != "" || name != "" {
		label := strings.TrimSpace(strings.TrimSpace(code) + " - " + strings.TrimSpace(name))
		label = strings.Trim(label, "- ")
		if currency != "" {
			label = fmt.Sprintf("%s [%s]", label, strings.ToUpper(strings.TrimSpace(currency)))
		}
		parts = append(parts, label)
	}

	if glCode != "" || glName != "" {
		glLabel := strings.TrimSpace(strings.TrimSpace(glCode) + " - " + strings.TrimSpace(glName))
		glLabel = strings.Trim(glLabel, "- ")
		if glLabel != "" {
			parts = append(parts, "GL "+glLabel)
		}
	}

	return strings.Join(parts, " | ")
}

func buildReceivablePaymentJournalNotes(payment *models.SalePayment, sale *models.Sale, bankBookAmountValue, receivableSettlementValue, forexGainLossValue float64) string {
	if payment == nil {
		return ""
	}

	snapshotSummary := formatCashBankSnapshotSummary(
		payment.CashBankCode,
		payment.CashBankName,
		payment.CashBankCurrency,
		payment.CashBankGLAccountCode,
		payment.CashBankGLAccountName,
	)

	fxSummary := "Realized FX netral: nilai buku bank yang diposting sama dengan nilai buku historis piutang yang ditutup."
	if forexGainLossValue > 0.005 {
		fxSummary = fmt.Sprintf("Realized FX laba kurs Rp %.2f: nilai buku bank yang diposting lebih besar daripada nilai buku historis piutang.", forexGainLossValue)
	} else if forexGainLossValue < -0.005 {
		fxSummary = fmt.Sprintf("Realized FX rugi kurs Rp %.2f: nilai buku bank yang diposting lebih kecil daripada nilai buku historis piutang.", -forexGainLossValue)
	}

	notes := []string{}
	if snapshotSummary != "" {
		notes = append(notes, "Rekening final: "+snapshotSummary)
	}
	if sale != nil {
		notes = append(notes, fmt.Sprintf("Invoice %s dibukukan dalam %s.", sale.InvoiceNumber, strings.ToUpper(strings.TrimSpace(sale.Currency))))
	}
	notes = append(notes, fmt.Sprintf("Nilai buku bank Rp %.2f, nilai buku piutang historis Rp %.2f.", bankBookAmountValue, receivableSettlementValue))
	notes = append(notes, fxSummary)

	return strings.Join(notes, " ")
}
