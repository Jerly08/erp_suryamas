package services

import (
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type salesCreditCorrectionFixture struct {
	db              *gorm.DB
	service         *PostedTransactionCorrectionService
	financeStaff    models.User
	accounting      models.User
	financeManager  models.User
	sale            models.Sale
	payment         models.Payment
	originalJournal models.SSOTJournalEntry
	paymentJournal  models.SSOTJournalEntry
	receivable      models.Account
	bankAccount     models.Account
}

func newSalesCreditCorrectionFixture(t *testing.T) salesCreditCorrectionFixture {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Account{},
		&models.CashBank{},
		&models.CashBankTransaction{},
		&models.Sale{},
		&models.Payment{},
		&models.PaymentAllocation{},
		&models.Purchase{},
		&models.PurchasePayment{},
		&models.AccountingPeriod{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
		&models.PostedTransactionCorrection{},
		&models.PostedTransactionCorrectionDocument{},
		&models.PostedTransactionCorrectionEvent{},
	))

	financeStaff := createSalesCreditCorrectionUser(t, db, "legacy-credit-finance", models.RoleFinanceStaff)
	accounting := createSalesCreditCorrectionUser(t, db, "legacy-credit-accounting", models.RoleAccounting)
	financeManager := createSalesCreditCorrectionUser(t, db, "legacy-credit-finance-manager", models.RoleFinance)

	customer := models.Contact{
		Code:     "CUSTOMER-LEGACY-CREDIT",
		Name:     "Customer Kredit Lama",
		Type:     models.ContactTypeCustomer,
		IsActive: true,
	}
	require.NoError(t, db.Create(&customer).Error)

	bankAccount := models.Account{
		Code:     "1101-003-LEGACY",
		Name:     "BANK PENERIMAAN LEGACY",
		Type:     models.AccountTypeAsset,
		IsActive: true,
		IsHeader: false,
	}
	receivable := models.Account{
		Code:     "1102-001-LEGACY",
		Name:     "PIUTANG USAHA LEGACY",
		Type:     models.AccountTypeAsset,
		IsActive: true,
		IsHeader: false,
	}
	revenue := models.Account{
		Code:     "4101-001-LEGACY",
		Name:     "PENDAPATAN PENJUALAN LEGACY",
		Type:     models.AccountTypeRevenue,
		IsActive: true,
		IsHeader: false,
	}
	require.NoError(t, db.Create(&bankAccount).Error)
	require.NoError(t, db.Create(&receivable).Error)
	require.NoError(t, db.Create(&revenue).Error)

	cashBank := models.CashBank{
		Code:      "BNK-LEGACY-CREDIT",
		Name:      "BANK LEGACY CREDIT",
		Type:      models.CashBankTypeBank,
		AccountID: bankAccount.ID,
		Currency:  "IDR",
		IsActive:  true,
		UserID:    financeStaff.ID,
	}
	require.NoError(t, db.Create(&cashBank).Error)

	saleDate := time.Date(2026, time.March, 6, 0, 0, 0, 0, time.UTC)
	sale := models.Sale{
		Code:              "SOA-LEGACY-0001",
		InvoiceNumber:     "INV-LEGACY-0001",
		CustomerID:        customer.ID,
		UserID:            financeStaff.ID,
		Date:              saleDate,
		DueDate:           saleDate.AddDate(0, 0, 90),
		Currency:          "IDR",
		TotalAmount:       100_000,
		PaidAmount:        100_000,
		OutstandingAmount: 0,
		PaymentTerms:      "NET_90",
		PaymentMethodType: "BANK",
		CashBankID:        &cashBank.ID,
		Status:            models.SaleStatusPaid,
	}
	require.NoError(t, db.Create(&sale).Error)

	originalJournal := createSalesCreditCorrectionJournal(
		t,
		db,
		"SALE-LEGACY-0001",
		models.SSOTSourceTypeSale,
		uint64(sale.ID),
		sale.Code,
		saleDate,
		financeStaff.ID,
		[]models.SSOTJournalLine{
			{AccountID: uint64(bankAccount.ID), DebitAmount: decimal.NewFromInt(100_000)},
			{AccountID: uint64(revenue.ID), CreditAmount: decimal.NewFromInt(100_000)},
		},
	)

	paymentDate := time.Date(2026, time.May, 11, 0, 0, 0, 0, time.UTC)
	payment := models.Payment{
		Code:        "RCV-LEGACY-0001",
		ContactID:   customer.ID,
		UserID:      financeStaff.ID,
		Date:        paymentDate,
		Amount:      100_000,
		Currency:    "IDR",
		Method:      models.PaymentMethodBankTransfer,
		Status:      models.PaymentStatusCompleted,
		PaymentType: PaymentTypeReceivable,
		CashBankID:  &cashBank.ID,
	}
	require.NoError(t, db.Create(&payment).Error)

	require.NoError(t, db.Create(&models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   "SALE",
		ReferenceID:     sale.ID,
		Amount:          sale.TotalAmount,
		Currency:        "IDR",
		ExchangeRate:    1,
		BookAmountIDR:   sale.TotalAmount,
		BalanceAfter:    sale.TotalAmount,
		TransactionDate: saleDate,
		Notes:           "Penerimaan awal yang salah untuk invoice kredit lama",
	}).Error)
	require.NoError(t, db.Create(&models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   "PAYMENT",
		ReferenceID:     payment.ID,
		Amount:          payment.Amount,
		Currency:        "IDR",
		ExchangeRate:    1,
		BookAmountIDR:   payment.Amount,
		BalanceAfter:    sale.TotalAmount + payment.Amount,
		TransactionDate: paymentDate,
		Notes:           "Penerimaan pelanggan aktual",
	}).Error)

	paymentJournal := createSalesCreditCorrectionJournal(
		t,
		db,
		"RCV-LEGACY-0001",
		models.SSOTSourceTypePayment,
		uint64(payment.ID),
		payment.Code,
		paymentDate,
		financeStaff.ID,
		[]models.SSOTJournalLine{
			{AccountID: uint64(bankAccount.ID), DebitAmount: decimal.NewFromInt(100_000)},
			{AccountID: uint64(receivable.ID), CreditAmount: decimal.NewFromInt(100_000)},
		},
	)
	paymentJournalID := uint(paymentJournal.ID)
	require.NoError(t, db.Model(&payment).Update("journal_entry_id", paymentJournalID).Error)
	payment.JournalEntryID = &paymentJournalID

	return salesCreditCorrectionFixture{
		db:              db,
		service:         NewPostedTransactionCorrectionService(db),
		financeStaff:    financeStaff,
		accounting:      accounting,
		financeManager:  financeManager,
		sale:            sale,
		payment:         payment,
		originalJournal: originalJournal,
		paymentJournal:  paymentJournal,
		receivable:      receivable,
		bankAccount:     bankAccount,
	}
}

func createSalesCreditCorrectionUser(t *testing.T, db *gorm.DB, username, role string) models.User {
	t.Helper()
	user := models.User{
		Username: username,
		Email:    username + "@test.local",
		Password: "secret",
		Role:     role,
		IsActive: true,
	}
	require.NoError(t, db.Create(&user).Error)
	return user
}

func createSalesCreditCorrectionJournal(
	t *testing.T,
	db *gorm.DB,
	entryNumber string,
	sourceType string,
	sourceID uint64,
	sourceCode string,
	entryDate time.Time,
	createdBy uint,
	lines []models.SSOTJournalLine,
) models.SSOTJournalEntry {
	t.Helper()
	postedAt := entryDate.Add(12 * time.Hour)
	postedBy := uint64(createdBy)
	entry := models.SSOTJournalEntry{
		EntryNumber: entryNumber,
		SourceType:  sourceType,
		SourceID:    &sourceID,
		SourceCode:  sourceCode,
		EntryDate:   entryDate,
		Description: entryNumber,
		Status:      models.SSOTStatusPosted,
		IsBalanced:  true,
		CreatedBy:   uint64(createdBy),
		PostedAt:    &postedAt,
		PostedBy:    &postedBy,
	}
	for _, line := range lines {
		entry.TotalDebit = entry.TotalDebit.Add(line.DebitAmount)
		entry.TotalCredit = entry.TotalCredit.Add(line.CreditAmount)
	}
	require.NoError(t, db.Create(&entry).Error)
	for index := range lines {
		lines[index].JournalID = entry.ID
		lines[index].LineNumber = index + 1
		require.NoError(t, db.Create(&lines[index]).Error)
	}
	return entry
}

func TestSalesCreditClassificationCorrectionPreservesOriginalEntries(t *testing.T) {
	fixture := newSalesCreditCorrectionFixture(t)

	candidates, err := fixture.service.ListLegacySalesCreditPaymentCandidates(
		fixture.sale.ID,
		fixture.financeStaff.ID,
		models.RoleFinanceStaff,
	)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, fixture.payment.ID, candidates[0].ID)

	requestedDate := "2026-05-11"
	correction, err := fixture.service.Create(models.PostedTransactionCorrectionCreateRequest{
		SaleID:          &fixture.sale.ID,
		LinkedPaymentID: &fixture.payment.ID,
		CorrectionType:  models.PostedTransactionCorrectionTypeSalesCreditClassification,
		Reason:          "Invoice kredit lama tercatat sebagai penerimaan bank sebelum dana diterima.",
		RequestedDate:   requestedDate,
	}, fixture.financeStaff.ID, models.RoleFinanceStaff)
	require.NoError(t, err)
	require.Equal(t, models.PostedTransactionCorrectionStatusPendingAccounting, correction.Status)

	reviewed, err := fixture.service.ReviewAccounting(correction.ID, models.PostedTransactionCorrectionAccountingReviewRequest{
		Resolution:        models.PostedTransactionCorrectionResolutionJournalReclass,
		Notes:             "Reklasifikasi ke Piutang Usaha dan tautkan penerimaan yang sudah ada.",
		ProposedAccountID: &fixture.receivable.ID,
	}, fixture.accounting.ID, models.RoleAccounting)
	require.NoError(t, err)
	require.Equal(t, models.PostedTransactionCorrectionStatusPendingFinanceManager, reviewed.Status)

	approved, err := fixture.service.DecideFinanceManager(correction.ID, models.PostedTransactionCorrectionFinanceManagerDecisionRequest{
		Approve: true,
		Notes:   "Disetujui setelah bukti penerimaan dan COA ditinjau.",
	}, fixture.financeManager.ID, models.RoleFinance)
	require.NoError(t, err)
	require.Equal(t, models.PostedTransactionCorrectionStatusApprovedForExecution, approved.Status)

	completed, err := fixture.service.ExecuteSalesCreditCorrection(correction.ID, fixture.accounting.ID, models.RoleAccounting)
	require.NoError(t, err)
	require.Equal(t, models.PostedTransactionCorrectionStatusCompleted, completed.Status)
	require.NotNil(t, completed.CompletionJournalEntryID)

	var correctionJournal models.SSOTJournalEntry
	require.NoError(t, fixture.db.Preload("Lines").First(&correctionJournal, *completed.CompletionJournalEntryID).Error)
	require.Equal(t, models.SSOTStatusPosted, correctionJournal.Status)
	require.True(t, correctionJournal.IsAutoGenerated)
	require.Equal(t, models.SSOTSourceTypeAdjustment, correctionJournal.SourceType)
	require.NotNil(t, correctionJournal.AdjustmentOf)
	require.Equal(t, fixture.originalJournal.ID, *correctionJournal.AdjustmentOf)
	require.Equal(t, models.JournalAdjustmentCategorySalesRevenueRecognition, correctionJournal.AdjustmentCategoryCode)
	require.True(t, journalHasDebitLine(correctionJournal, uint64(fixture.receivable.ID), fixture.sale.TotalAmount))
	require.True(t, journalHasCreditLine(correctionJournal, uint64(fixture.bankAccount.ID), fixture.sale.TotalAmount))

	var allocation models.PaymentAllocation
	require.NoError(t, fixture.db.Where("payment_id = ? AND invoice_id = ?", fixture.payment.ID, fixture.sale.ID).First(&allocation).Error)
	require.Equal(t, fixture.sale.TotalAmount, allocation.AllocatedAmount)

	var cashBankAfter models.CashBank
	var bankAccountAfter models.Account
	var cashBankCorrection models.CashBankTransaction
	require.NoError(t, fixture.db.First(&cashBankAfter, fixture.sale.CashBankID).Error)
	require.NoError(t, fixture.db.First(&bankAccountAfter, fixture.bankAccount.ID).Error)
	require.NoError(t, fixture.db.Where("reference_type = ? AND reference_id = ?", salesCreditCorrectionCashBankReferenceType, completed.ID).First(&cashBankCorrection).Error)
	require.InDelta(t, fixture.sale.TotalAmount, cashBankAfter.Balance, 0.01)
	require.InDelta(t, fixture.sale.TotalAmount, cashBankAfter.BookBalanceIDR, 0.01)
	require.InDelta(t, fixture.sale.TotalAmount, bankAccountAfter.Balance, 0.01)
	require.InDelta(t, -fixture.sale.TotalAmount, cashBankCorrection.Amount, 0.01)
	require.InDelta(t, -fixture.sale.TotalAmount, cashBankCorrection.BookAmountIDR, 0.01)
	require.InDelta(t, fixture.sale.TotalAmount, cashBankCorrection.BalanceAfter, 0.01)

	var cashBankTransactionCount int64
	require.NoError(t, fixture.db.Model(&models.CashBankTransaction{}).Where("cash_bank_id = ?", cashBankAfter.ID).Count(&cashBankTransactionCount).Error)
	require.EqualValues(t, 3, cashBankTransactionCount)

	var saleAfter models.Sale
	var originalJournalAfter models.SSOTJournalEntry
	var paymentJournalAfter models.SSOTJournalEntry
	require.NoError(t, fixture.db.First(&saleAfter, fixture.sale.ID).Error)
	require.NoError(t, fixture.db.First(&originalJournalAfter, fixture.originalJournal.ID).Error)
	require.NoError(t, fixture.db.First(&paymentJournalAfter, fixture.paymentJournal.ID).Error)
	require.Equal(t, fixture.sale.PaidAmount, saleAfter.PaidAmount)
	require.Equal(t, fixture.sale.OutstandingAmount, saleAfter.OutstandingAmount)
	require.Equal(t, models.SSOTStatusPosted, originalJournalAfter.Status)
	require.Equal(t, fixture.originalJournal.TotalDebit, originalJournalAfter.TotalDebit)
	require.Equal(t, fixture.originalJournal.TotalCredit, originalJournalAfter.TotalCredit)
	require.Equal(t, models.SSOTStatusPosted, paymentJournalAfter.Status)
	require.Equal(t, fixture.paymentJournal.TotalDebit, paymentJournalAfter.TotalDebit)
	require.Equal(t, fixture.paymentJournal.TotalCredit, paymentJournalAfter.TotalCredit)

	var journalCount int64
	require.NoError(t, fixture.db.Model(&models.SSOTJournalEntry{}).Count(&journalCount).Error)
	require.EqualValues(t, 3, journalCount)

	_, err = fixture.service.ExecuteSalesCreditCorrection(correction.ID, fixture.accounting.ID, models.RoleAccounting)
	require.Error(t, err)
	require.Contains(t, err.Error(), "belum disetujui")
}

func TestSalesCreditClassificationCorrectionAcceptsPaymentReceivedAuditTrail(t *testing.T) {
	fixture := newSalesCreditCorrectionFixture(t)

	require.NoError(t, fixture.db.Model(&models.CashBankTransaction{}).
		Where("reference_type = ? AND reference_id = ?", "PAYMENT", fixture.payment.ID).
		Update("reference_type", "PAYMENT_RECEIVED").Error)

	candidates, err := fixture.service.ListLegacySalesCreditPaymentCandidates(
		fixture.sale.ID,
		fixture.financeStaff.ID,
		models.RoleFinanceStaff,
	)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, fixture.payment.ID, candidates[0].ID)

	correction, err := fixture.service.Create(models.PostedTransactionCorrectionCreateRequest{
		SaleID:          &fixture.sale.ID,
		LinkedPaymentID: &fixture.payment.ID,
		CorrectionType:  models.PostedTransactionCorrectionTypeSalesCreditClassification,
		Reason:          "Penerimaan pelanggan lama memakai referensi PAYMENT_RECEIVED.",
		RequestedDate:   "2026-05-11",
	}, fixture.financeStaff.ID, models.RoleFinanceStaff)
	require.NoError(t, err)

	_, err = fixture.service.ReviewAccounting(correction.ID, models.PostedTransactionCorrectionAccountingReviewRequest{
		Resolution:        models.PostedTransactionCorrectionResolutionJournalReclass,
		Notes:             "COA piutang dan penerimaan pelanggan telah diverifikasi.",
		ProposedAccountID: &fixture.receivable.ID,
	}, fixture.accounting.ID, models.RoleAccounting)
	require.NoError(t, err)

	_, err = fixture.service.DecideFinanceManager(correction.ID, models.PostedTransactionCorrectionFinanceManagerDecisionRequest{
		Approve: true,
		Notes:   "Disetujui.",
	}, fixture.financeManager.ID, models.RoleFinance)
	require.NoError(t, err)

	completed, err := fixture.service.ExecuteSalesCreditCorrection(correction.ID, fixture.accounting.ID, models.RoleAccounting)
	require.NoError(t, err)
	require.Equal(t, models.PostedTransactionCorrectionStatusCompleted, completed.Status)
}

func TestSalesCreditClassificationCandidateRejectsMismatchedPaymentJournalLink(t *testing.T) {
	fixture := newSalesCreditCorrectionFixture(t)

	unrelatedJournal := createSalesCreditCorrectionJournal(
		t,
		fixture.db,
		"RCV-UNRELATED-0001",
		models.SSOTSourceTypePayment,
		999_999,
		"RCV-UNRELATED-0001",
		fixture.payment.Date,
		fixture.financeStaff.ID,
		[]models.SSOTJournalLine{
			{AccountID: uint64(fixture.bankAccount.ID), DebitAmount: decimal.NewFromInt(100_000)},
			{AccountID: uint64(fixture.receivable.ID), CreditAmount: decimal.NewFromInt(100_000)},
		},
	)
	require.NoError(t, fixture.db.Model(&models.Payment{}).
		Where("id = ?", fixture.payment.ID).
		Update("journal_entry_id", uint(unrelatedJournal.ID)).Error)

	candidates, err := fixture.service.ListLegacySalesCreditPaymentCandidates(
		fixture.sale.ID,
		fixture.financeStaff.ID,
		models.RoleFinanceStaff,
	)
	require.NoError(t, err)
	require.Empty(t, candidates)

	_, err = fixture.service.Create(models.PostedTransactionCorrectionCreateRequest{
		SaleID:          &fixture.sale.ID,
		LinkedPaymentID: &fixture.payment.ID,
		CorrectionType:  models.PostedTransactionCorrectionTypeSalesCreditClassification,
		Reason:          "Uji validasi tautan jurnal pembayaran.",
		RequestedDate:   "2026-05-11",
	}, fixture.financeStaff.ID, models.RoleFinanceStaff)
	require.Error(t, err)
	require.Contains(t, err.Error(), "jurnal terposting yang sesuai")
}

func TestSalesCreditClassificationCandidateRequiresCashBankAuditTrail(t *testing.T) {
	tests := []struct {
		name          string
		referenceType string
		referenceID   func(salesCreditCorrectionFixture) uint
		errorContains string
	}{
		{
			name:          "missing erroneous sale movement",
			referenceType: "SALE",
			referenceID:   func(fixture salesCreditCorrectionFixture) uint { return fixture.sale.ID },
			errorContains: "mutasi kas/bank awal dari penjualan",
		},
		{
			name:          "missing actual customer receipt movement",
			referenceType: "PAYMENT",
			referenceID:   func(fixture salesCreditCorrectionFixture) uint { return fixture.payment.ID },
			errorContains: "mutasi kas/bank penerimaan pelanggan",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newSalesCreditCorrectionFixture(t)
			require.NoError(t, fixture.db.Where("reference_type = ? AND reference_id = ?", testCase.referenceType, testCase.referenceID(fixture)).Delete(&models.CashBankTransaction{}).Error)

			candidates, err := fixture.service.ListLegacySalesCreditPaymentCandidates(
				fixture.sale.ID,
				fixture.financeStaff.ID,
				models.RoleFinanceStaff,
			)
			require.NoError(t, err)
			require.Empty(t, candidates)

			_, err = fixture.service.Create(models.PostedTransactionCorrectionCreateRequest{
				SaleID:          &fixture.sale.ID,
				LinkedPaymentID: &fixture.payment.ID,
				CorrectionType:  models.PostedTransactionCorrectionTypeSalesCreditClassification,
				Reason:          "Uji kelengkapan jejak audit kas/bank.",
				RequestedDate:   "2026-05-11",
			}, fixture.financeStaff.ID, models.RoleFinanceStaff)
			require.Error(t, err)
			require.Contains(t, err.Error(), testCase.errorContains)
		})
	}
}

func TestSalesCreditClassificationCandidateRejectsForeignCashBank(t *testing.T) {
	fixture := newSalesCreditCorrectionFixture(t)
	require.NoError(t, fixture.db.Model(&models.CashBank{}).
		Where("id = ?", *fixture.sale.CashBankID).
		Update("currency", "USD").Error)

	candidates, err := fixture.service.ListLegacySalesCreditPaymentCandidates(
		fixture.sale.ID,
		fixture.financeStaff.ID,
		models.RoleFinanceStaff,
	)
	require.NoError(t, err)
	require.Empty(t, candidates)

	_, err = fixture.service.Create(models.PostedTransactionCorrectionCreateRequest{
		SaleID:          &fixture.sale.ID,
		LinkedPaymentID: &fixture.payment.ID,
		CorrectionType:  models.PostedTransactionCorrectionTypeSalesCreditClassification,
		Reason:          "Uji penolakan rekening valuta asing.",
		RequestedDate:   "2026-05-11",
	}, fixture.financeStaff.ID, models.RoleFinanceStaff)
	require.Error(t, err)
	require.Contains(t, err.Error(), "akun kas/bank IDR")
}

func TestValidateSalePaymentScheduleModeRequiresReceivableForDeferredTerms(t *testing.T) {
	for _, paymentTerms := range []string{"NET_15", "NET_30", "NET_60", "NET_90", "CUSTOM"} {
		t.Run(paymentTerms, func(t *testing.T) {
			require.Error(t, validateSalePaymentScheduleMode(paymentTerms, "BANK"))
			require.NoError(t, validateSalePaymentScheduleMode(paymentTerms, "CREDIT"))
		})
	}

	require.NoError(t, validateSalePaymentScheduleMode("COD", "BANK"))
}
