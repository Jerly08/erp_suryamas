package services

import (
	"strings"
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type postedTransactionCorrectionFixture struct {
	db             *gorm.DB
	service        *PostedTransactionCorrectionService
	financeStaff   models.User
	accounting     models.User
	financeManager models.User
	purchase       models.Purchase
	journal        models.SSOTJournalEntry
	proposedCOA    models.Account
}

func newPostedTransactionCorrectionFixture(t *testing.T) postedTransactionCorrectionFixture {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Account{},
		&models.CashBank{},
		&models.Payment{},
		&models.Purchase{},
		&models.PurchasePayment{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
		&models.PostedTransactionCorrection{},
		&models.PostedTransactionCorrectionReclassificationLine{},
		&models.PostedTransactionCorrectionDocument{},
		&models.PostedTransactionCorrectionEvent{},
	))

	financeStaff := models.User{
		Username: "correction-finance-staff",
		Email:    "correction-finance-staff@test.local",
		Password: "secret",
		Role:     models.RoleFinanceStaff,
		IsActive: true,
	}
	accounting := models.User{
		Username: "correction-accounting",
		Email:    "correction-accounting@test.local",
		Password: "secret",
		Role:     models.RoleAccounting,
		IsActive: true,
	}
	financeManager := models.User{
		Username: "correction-finance-manager",
		Email:    "correction-finance-manager@test.local",
		Password: "secret",
		Role:     models.RoleFinance,
		IsActive: true,
	}
	require.NoError(t, db.Create(&financeStaff).Error)
	require.NoError(t, db.Create(&accounting).Error)
	require.NoError(t, db.Create(&financeManager).Error)

	vendor := models.Contact{
		Code:     "CORRECTION-VENDOR",
		Name:     "Vendor Koreksi",
		Type:     models.ContactTypeVendor,
		IsActive: true,
	}
	require.NoError(t, db.Create(&vendor).Error)

	proposedCOA := models.Account{
		Code:     "6101-CORRECTION",
		Name:     "Beban Koreksi",
		Type:     models.AccountTypeExpense,
		IsActive: true,
	}
	require.NoError(t, db.Create(&proposedCOA).Error)

	purchaseDate := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
	purchase := models.Purchase{
		Code:              "PO-CORRECTION-001",
		VendorID:          vendor.ID,
		UserID:            financeStaff.ID,
		Date:              purchaseDate,
		DueDate:           purchaseDate.AddDate(0, 0, 30),
		TotalAmount:       17_700_000,
		OutstandingAmount: 0,
		PaymentMethod:     models.PurchasePaymentTransfer,
		Status:            models.PurchaseStatusCompleted,
		ApprovalStatus:    models.PurchaseApprovalApproved,
	}
	require.NoError(t, db.Create(&purchase).Error)

	sourceID := uint64(purchase.ID)
	postedAt := purchaseDate.Add(10 * time.Hour)
	postedBy := uint64(financeManager.ID)
	journal := models.SSOTJournalEntry{
		EntryNumber: "JE-CORRECTION-001",
		SourceType:  models.SSOTSourceTypePurchase,
		SourceID:    &sourceID,
		SourceCode:  purchase.Code,
		EntryDate:   purchaseDate,
		Description: "Pembelian yang sudah terposting",
		Status:      models.SSOTStatusPosted,
		TotalDebit:  decimal.NewFromInt(17_700_000),
		TotalCredit: decimal.NewFromInt(17_700_000),
		IsBalanced:  true,
		CreatedBy:   uint64(financeStaff.ID),
		PostedBy:    &postedBy,
		PostedAt:    &postedAt,
	}
	require.NoError(t, db.Create(&journal).Error)

	return postedTransactionCorrectionFixture{
		db:             db,
		service:        NewPostedTransactionCorrectionService(db),
		financeStaff:   financeStaff,
		accounting:     accounting,
		financeManager: financeManager,
		purchase:       purchase,
		journal:        journal,
		proposedCOA:    proposedCOA,
	}
}

func TestPostedTransactionCorrectionLifecyclePreservesOriginalTransaction(t *testing.T) {
	fixture := newPostedTransactionCorrectionFixture(t)

	correction, err := fixture.service.Create(models.PostedTransactionCorrectionCreateRequest{
		PurchaseID:         &fixture.purchase.ID,
		CorrectionType:     models.PostedTransactionCorrectionTypeMissingPaymentProof,
		Reason:             "Bukti pembayaran bank perlu dilengkapi.",
		RequestedReference: "KOPRA-20260727-001",
	}, fixture.financeStaff.ID, models.RoleFinanceStaff)
	require.NoError(t, err)
	require.Equal(t, models.PostedTransactionCorrectionStatusPendingAccounting, correction.Status)
	require.Equal(t, fixture.journal.ID, correction.OriginalJournalEntryID)
	require.NotEmpty(t, correction.SourceSnapshot)

	// Submitting a correction only records an audit request. It must not change
	// the posted PO or its original journal.
	var purchaseAfterRequest models.Purchase
	var journalAfterRequest models.SSOTJournalEntry
	require.NoError(t, fixture.db.First(&purchaseAfterRequest, fixture.purchase.ID).Error)
	require.NoError(t, fixture.db.First(&journalAfterRequest, fixture.journal.ID).Error)
	require.Equal(t, fixture.purchase.Date, purchaseAfterRequest.Date)
	require.Equal(t, fixture.purchase.TotalAmount, purchaseAfterRequest.TotalAmount)
	require.Equal(t, fixture.purchase.Status, purchaseAfterRequest.Status)
	require.Equal(t, models.SSOTStatusPosted, journalAfterRequest.Status)
	require.Equal(t, fixture.journal.TotalDebit, journalAfterRequest.TotalDebit)
	require.Equal(t, fixture.journal.TotalCredit, journalAfterRequest.TotalCredit)

	reviewed, err := fixture.service.ReviewAccounting(correction.ID, models.PostedTransactionCorrectionAccountingReviewRequest{
		Resolution:        models.PostedTransactionCorrectionResolutionJournalReclass,
		Notes:             "Accounting akan membuat jurnal koreksi setelah bukti bank diverifikasi.",
		ProposedAccountID: &fixture.proposedCOA.ID,
	}, fixture.accounting.ID, models.RoleAccounting)
	require.NoError(t, err)
	require.Equal(t, models.PostedTransactionCorrectionStatusPendingFinanceManager, reviewed.Status)
	require.Equal(t, fixture.proposedCOA.ID, *reviewed.ProposedAccountID)

	approved, err := fixture.service.DecideFinanceManager(correction.ID, models.PostedTransactionCorrectionFinanceManagerDecisionRequest{
		Approve: true,
		Notes:   "Disetujui untuk koreksi berdasarkan bukti bank.",
	}, fixture.financeManager.ID, models.RoleFinance)
	require.NoError(t, err)
	require.Equal(t, models.PostedTransactionCorrectionStatusApprovedForExecution, approved.Status)

	_, err = fixture.service.Complete(correction.ID, models.PostedTransactionCorrectionCompleteRequest{
		Notes:                    "Tidak boleh menautkan jurnal asal sebagai koreksi.",
		CompletionJournalEntryID: &fixture.journal.ID,
	}, fixture.accounting.ID, models.RoleAccounting)
	require.Error(t, err)
	require.Contains(t, err.Error(), "jurnal asal")

	adjustmentOf := fixture.journal.ID
	correctionJournal := models.SSOTJournalEntry{
		EntryNumber:  "JE-CORRECTION-ADJUSTMENT-001",
		SourceType:   models.SSOTSourceTypeAdjustment,
		SourceCode:   fixture.purchase.Code,
		EntryDate:    fixture.purchase.Date.AddDate(0, 0, 1),
		Description:  "Jurnal koreksi pembayaran",
		Status:       models.SSOTStatusPosted,
		TotalDebit:   decimal.NewFromInt(100_000),
		TotalCredit:  decimal.NewFromInt(100_000),
		IsBalanced:   true,
		CreatedBy:    uint64(fixture.accounting.ID),
		AdjustmentOf: &adjustmentOf,
	}
	require.NoError(t, fixture.db.Create(&correctionJournal).Error)

	completed, err := fixture.service.Complete(correction.ID, models.PostedTransactionCorrectionCompleteRequest{
		Notes:                    "Jurnal koreksi telah dibuat dan direkonsiliasi oleh Accounting.",
		CompletionJournalEntryID: &correctionJournal.ID,
	}, fixture.accounting.ID, models.RoleAccounting)
	require.NoError(t, err)
	require.Equal(t, models.PostedTransactionCorrectionStatusCompleted, completed.Status)
	require.NotNil(t, completed.CompletionJournalEntryID)
	require.Equal(t, correctionJournal.ID, *completed.CompletionJournalEntryID)
	require.Len(t, completed.Events, 4)

	var purchaseAfterCompletion models.Purchase
	var journalAfterCompletion models.SSOTJournalEntry
	require.NoError(t, fixture.db.First(&purchaseAfterCompletion, fixture.purchase.ID).Error)
	require.NoError(t, fixture.db.First(&journalAfterCompletion, fixture.journal.ID).Error)
	require.Equal(t, fixture.purchase.Date, purchaseAfterCompletion.Date)
	require.Equal(t, fixture.purchase.TotalAmount, purchaseAfterCompletion.TotalAmount)
	require.Equal(t, fixture.purchase.Status, purchaseAfterCompletion.Status)
	require.Equal(t, models.SSOTStatusPosted, journalAfterCompletion.Status)
	require.Equal(t, fixture.journal.TotalDebit, journalAfterCompletion.TotalDebit)
	require.Equal(t, fixture.journal.TotalCredit, journalAfterCompletion.TotalCredit)
}

func TestPostedTransactionCorrectionRequiresPostedJournalAndAccountingRole(t *testing.T) {
	fixture := newPostedTransactionCorrectionFixture(t)

	noJournalPurchase := models.Purchase{
		Code:              "PO-CORRECTION-NO-JOURNAL",
		VendorID:          fixture.purchase.VendorID,
		UserID:            fixture.financeStaff.ID,
		Date:              fixture.purchase.Date,
		DueDate:           fixture.purchase.DueDate,
		TotalAmount:       1_000,
		OutstandingAmount: 1_000,
		PaymentMethod:     models.PurchasePaymentTransfer,
		Status:            models.PurchaseStatusCompleted,
	}
	require.NoError(t, fixture.db.Create(&noJournalPurchase).Error)

	_, err := fixture.service.Create(models.PostedTransactionCorrectionCreateRequest{
		PurchaseID:     &noJournalPurchase.ID,
		CorrectionType: models.PostedTransactionCorrectionTypeMissingPaymentProof,
		Reason:         "Tanggal transaksi perlu dicek.",
	}, fixture.financeStaff.ID, models.RoleFinanceStaff)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "belum memiliki jurnal terposting"))

	correction, err := fixture.service.Create(models.PostedTransactionCorrectionCreateRequest{
		PurchaseID:     &fixture.purchase.ID,
		CorrectionType: models.PostedTransactionCorrectionTypeMissingPaymentProof,
		Reason:         "Tanggal transaksi perlu dicek.",
	}, fixture.financeStaff.ID, models.RoleFinanceStaff)
	require.NoError(t, err)

	_, err = fixture.service.ReviewAccounting(correction.ID, models.PostedTransactionCorrectionAccountingReviewRequest{
		Resolution: models.PostedTransactionCorrectionResolutionManualAdjustment,
		Notes:      "Tidak boleh dijalankan oleh Finance Staff.",
	}, fixture.financeStaff.ID, models.RoleFinanceStaff)
	require.ErrorIs(t, err, ErrPostedTransactionCorrectionDenied)

	otherFinanceStaff := models.User{
		Username: "correction-other-finance-staff",
		Email:    "correction-other-finance-staff@test.local",
		Password: "secret",
		Role:     models.RoleFinanceStaff,
		IsActive: true,
	}
	require.NoError(t, fixture.db.Create(&otherFinanceStaff).Error)

	_, err = fixture.service.AddDocument(
		correction.ID,
		"bukti-bank.pdf",
		"uploads/posted-transaction-corrections/bukti-bank.pdf",
		1024,
		"application/pdf",
		otherFinanceStaff.ID,
		models.RoleFinanceStaff,
	)
	require.ErrorIs(t, err, ErrPostedTransactionCorrectionDenied)

	document, err := fixture.service.AddDocument(
		correction.ID,
		"bukti-bank.pdf",
		"uploads/posted-transaction-corrections/bukti-bank.pdf",
		1024,
		"application/pdf",
		fixture.financeStaff.ID,
		models.RoleFinanceStaff,
	)
	require.NoError(t, err)
	require.Equal(t, correction.ID, document.CorrectionID)
}

func TestAccountingCanSubmitPostedTransactionCorrection(t *testing.T) {
	fixture := newPostedTransactionCorrectionFixture(t)

	correction, err := fixture.service.Create(models.PostedTransactionCorrectionCreateRequest{
		PurchaseID:     &fixture.purchase.ID,
		CorrectionType: models.PostedTransactionCorrectionTypeMissingPaymentProof,
		Reason:         "Accounting menemukan tanggal purchase perlu dikoreksi melalui jurnal audit.",
	}, fixture.accounting.ID, fixture.accounting.Role)
	require.NoError(t, err)
	require.Equal(t, models.PostedTransactionCorrectionStatusPendingAccounting, correction.Status)

	reviewed, err := fixture.service.ReviewAccounting(correction.ID, models.PostedTransactionCorrectionAccountingReviewRequest{
		Resolution: models.PostedTransactionCorrectionResolutionJournalReclass,
		Notes:      "Accounting menyiapkan jurnal reklasifikasi untuk ditinjau Finance Manager.",
	}, fixture.accounting.ID, fixture.accounting.Role)
	require.NoError(t, err)
	require.Equal(t, models.PostedTransactionCorrectionStatusPendingFinanceManager, reviewed.Status)
}

func TestPostedTransactionCorrectionRejectsUnsupportedWorkflow(t *testing.T) {
	fixture := newPostedTransactionCorrectionFixture(t)
	requestedAmount := 17_600_000.0

	_, err := fixture.service.Create(models.PostedTransactionCorrectionCreateRequest{
		PurchaseID:      &fixture.purchase.ID,
		CorrectionType:  models.PostedTransactionCorrectionTypePaymentAmount,
		Reason:          "Nominal pembayaran pada bukti bank berbeda dengan transaksi asal.",
		RequestedAmount: &requestedAmount,
	}, fixture.financeStaff.ID, models.RoleFinanceStaff)
	require.Error(t, err)
	require.Contains(t, err.Error(), "belum didukung")
}
