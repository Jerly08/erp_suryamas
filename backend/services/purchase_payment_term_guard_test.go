package services

import (
	"strings"
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestValidatePurchasePaymentTermAllocationRequiresExplicitTermForMultipleTerms(t *testing.T) {
	db := newPurchasePaymentTermGuardTestDB(t)
	purchase := createPurchasePaymentTermGuardPurchase(t, db, "PO-TERM-GUARD")
	terms := createPurchasePaymentTermGuardTerms(t, db, purchase.ID)

	err := validatePurchasePaymentTermAllocation(db, &purchase, nil, 100000, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Bayar PO / Termin")

	err = validatePurchasePaymentTermAllocation(db, &purchase, &terms[0].ID, 100000, true)
	require.NoError(t, err)

	payment := models.Payment{
		Code:      "PAY-TERM-GUARD",
		ContactID: 1,
		UserID:    1,
		Date:      time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC),
		Amount:    100000,
		Method:    models.PaymentMethodBankTransfer,
		Status:    models.PaymentStatusCompleted,
	}
	require.NoError(t, db.Create(&payment).Error)
	require.NoError(t, db.Create(&models.PaymentAllocation{
		PaymentID:             uint64(payment.ID),
		BillID:                &purchase.ID,
		PurchasePaymentTermID: &terms[0].ID,
		AllocatedAmount:       100000,
	}).Error)

	err = validatePurchasePaymentTermAllocation(db, &purchase, &terms[0].ID, 1, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "sudah lunas")
}

func TestResolvePurchasePaymentJournalTagIncludesPurchaseAndTerm(t *testing.T) {
	db := newPurchasePaymentTermGuardTestDB(t)
	purchase := createPurchasePaymentTermGuardPurchase(t, db, "PO-TERM-JOURNAL")
	terms := createPurchasePaymentTermGuardTerms(t, db, purchase.ID)
	payment := models.Payment{
		Code:      "PAY-TERM-JOURNAL",
		ContactID: 1,
		UserID:    1,
		Date:      time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC),
		Amount:    100000,
		Method:    models.PaymentMethodBankTransfer,
		Status:    models.PaymentStatusCompleted,
	}
	require.NoError(t, db.Create(&payment).Error)
	require.NoError(t, db.Create(&models.PaymentAllocation{
		PaymentID:             uint64(payment.ID),
		BillID:                &purchase.ID,
		PurchasePaymentTermID: &terms[1].ID,
		AllocatedAmount:       100000,
	}).Error)

	tag, err := resolvePurchasePaymentJournalTag(db, uint64(payment.ID))
	require.NoError(t, err)
	require.Equal(t, "PO-TERM-JOURNAL | Termin 2", tag)
	require.Equal(t, "Pelunasan Hutang Usaha | PO-TERM-JOURNAL | Termin 2", appendPurchasePaymentJournalTag("Pelunasan Hutang Usaha", tag))
	require.Equal(t, "Tag pembayaran: PO-TERM-JOURNAL | Termin 2", purchasePaymentJournalNotes(tag))
}

func TestPurchasePaymentJournalKeepsDoubleEntryAndTermTag(t *testing.T) {
	db := newPurchasePaymentTermGuardTestDB(t)
	purchase := createPurchasePaymentTermGuardPurchase(t, db, "PO-TERM-LEDGER")
	terms := createPurchasePaymentTermGuardTerms(t, db, purchase.ID)

	apAccount := models.Account{
		Code:     "2101-001",
		Name:     "UTANG USAHA - LOKAL",
		Type:     models.AccountTypeLiability,
		Category: models.CategoryCurrentLiability,
		IsActive: true,
	}
	bankAccount := models.Account{
		Code:     "1101-003-TEST",
		Name:     "BANK TERMIN TEST",
		Type:     models.AccountTypeAsset,
		Category: models.CategoryCurrentAsset,
		IsActive: true,
	}
	require.NoError(t, db.Create(&apAccount).Error)
	require.NoError(t, db.Create(&bankAccount).Error)

	cashBank := models.CashBank{
		Code:      "BNK-TERM-TEST",
		Name:      "BANK TERMIN TEST",
		Type:      models.CashBankTypeBank,
		AccountID: bankAccount.ID,
		Currency:  "IDR",
		IsActive:  true,
	}
	require.NoError(t, db.Create(&cashBank).Error)

	payment := models.Payment{
		Code:      "PAY-TERM-LEDGER",
		ContactID: 1,
		UserID:    1,
		Date:      time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC),
		Amount:    100000,
		Method:    models.PaymentMethodBankTransfer,
		Status:    models.PaymentStatusCompleted,
	}
	require.NoError(t, db.Create(&payment).Error)
	require.NoError(t, db.Create(&models.PaymentAllocation{
		PaymentID:             uint64(payment.ID),
		BillID:                &purchase.ID,
		PurchasePaymentTermID: &terms[1].ID,
		AllocatedAmount:       payment.Amount,
	}).Error)

	journalService := NewPurchasePaymentJournalService(db, nil, nil, nil)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return journalService.CreatePaymentJournalEntryWithTx(tx, &payment, cashBank.ID, payment.UserID)
	}))

	paymentID := uint64(payment.ID)
	var entry models.SSOTJournalEntry
	require.NoError(t, db.Where("source_type = ? AND source_id = ?", "PURCHASE_PAYMENT", paymentID).First(&entry).Error)
	require.Contains(t, entry.Description, "PO-TERM-LEDGER | Termin 2")
	require.Equal(t, "Tag pembayaran: PO-TERM-LEDGER | Termin 2", entry.Notes)
	require.Equal(t, "100000.00", entry.TotalDebit.StringFixed(2))
	require.Equal(t, "100000.00", entry.TotalCredit.StringFixed(2))

	var lines []models.SSOTJournalLine
	require.NoError(t, db.Where("journal_id = ?", entry.ID).Order("line_number ASC").Find(&lines).Error)
	require.Len(t, lines, 2)
	require.Equal(t, uint64(apAccount.ID), lines[0].AccountID)
	require.Equal(t, "100000.00", lines[0].DebitAmount.StringFixed(2))
	require.True(t, lines[0].CreditAmount.IsZero())
	require.Equal(t, uint64(bankAccount.ID), lines[1].AccountID)
	require.True(t, lines[1].DebitAmount.IsZero())
	require.Equal(t, "100000.00", lines[1].CreditAmount.StringFixed(2))
	for _, line := range lines {
		require.Contains(t, line.Description, "PO-TERM-LEDGER | Termin 2")
	}
}

func newPurchasePaymentTermGuardTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"),
		&gorm.Config{DisableForeignKeyConstraintWhenMigrating: true},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Purchase{},
		&models.PurchasePaymentTerm{},
		&models.Payment{},
		&models.PaymentAllocation{},
		&models.Account{},
		&models.CashBank{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
	))
	return db
}

func createPurchasePaymentTermGuardPurchase(t *testing.T, db *gorm.DB, code string) models.Purchase {
	t.Helper()
	purchase := models.Purchase{
		Code:              code,
		VendorID:          1,
		UserID:            1,
		Date:              time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		DueDate:           time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC),
		TotalAmount:       200000,
		OutstandingAmount: 200000,
		PaymentMethod:     models.PurchasePaymentCredit,
		Status:            models.PurchaseStatusApproved,
	}
	require.NoError(t, db.Create(&purchase).Error)
	return purchase
}

func createPurchasePaymentTermGuardTerms(t *testing.T, db *gorm.DB, purchaseID uint) []models.PurchasePaymentTerm {
	t.Helper()
	terms := []models.PurchasePaymentTerm{
		{PurchaseID: purchaseID, Sequence: 1, Amount: 100000, DueDate: time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC), Status: models.PurchasePaymentTermPending},
		{PurchaseID: purchaseID, Sequence: 2, Amount: 100000, DueDate: time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC), Status: models.PurchasePaymentTermPending},
	}
	require.NoError(t, db.Create(&terms).Error)
	return terms
}
