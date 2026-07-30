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

type purchaseCOAReclassificationFixture struct {
	db              *gorm.DB
	service         *PostedTransactionCorrectionService
	accounting      models.User
	financeManager  models.User
	purchase        models.Purchase
	originalJournal models.SSOTJournalEntry
	originalLine    models.SSOTJournalLine
	originalAccount models.Account
	replacementCOA  models.Account
	bankAccount     models.Account
	cashBank        models.CashBank
}

func newPurchaseCOAReclassificationFixture(t *testing.T) purchaseCOAReclassificationFixture {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Account{},
		&models.CashBank{},
		&models.CashBankTransaction{},
		&models.Purchase{},
		&models.AccountingPeriod{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
		&models.PostedTransactionCorrection{},
		&models.PostedTransactionCorrectionDocument{},
		&models.PostedTransactionCorrectionEvent{},
	))

	accounting := createPurchaseCOAReclassificationUser(t, db, "coa-reclass-accounting", models.RoleAccounting)
	financeManager := createPurchaseCOAReclassificationUser(t, db, "coa-reclass-finance-manager", models.RoleFinance)

	vendor := models.Contact{
		Code:     "COA-RECLASS-VENDOR",
		Name:     "Vendor Koreksi COA",
		Type:     models.ContactTypeVendor,
		IsActive: true,
	}
	require.NoError(t, db.Create(&vendor).Error)

	originalAccount := models.Account{
		Code:     "6101-RECLASS-ORIG",
		Name:     "BEBAN ASAL KOREKSI",
		Type:     models.AccountTypeExpense,
		IsActive: true,
	}
	replacementCOA := models.Account{
		Code:     "1106-RECLASS-NEW",
		Name:     "PIUTANG INTERCOMPANY KOREKSI",
		Type:     models.AccountTypeAsset,
		IsActive: true,
	}
	bankAccount := models.Account{
		Code:     "1101-RECLASS-BANK",
		Name:     "BANK TIDAK BOLEH DIREKLASIFIKASI",
		Type:     models.AccountTypeAsset,
		IsActive: true,
	}
	require.NoError(t, db.Create(&originalAccount).Error)
	require.NoError(t, db.Create(&replacementCOA).Error)
	require.NoError(t, db.Create(&bankAccount).Error)

	cashBank := models.CashBank{
		Code:           "BNK-COA-RECLASS",
		Name:           "Bank Koreksi COA",
		Type:           models.CashBankTypeBank,
		AccountID:      bankAccount.ID,
		Currency:       "IDR",
		Balance:        1_000_000,
		BookBalanceIDR: 1_000_000,
		IsActive:       true,
		UserID:         accounting.ID,
	}
	require.NoError(t, db.Create(&cashBank).Error)

	purchaseDate := time.Date(2026, time.July, 28, 0, 0, 0, 0, time.UTC)
	purchase := models.Purchase{
		Code:              "PO-COA-RECLASS-001",
		VendorID:          vendor.ID,
		UserID:            accounting.ID,
		Date:              purchaseDate,
		DueDate:           purchaseDate.AddDate(0, 0, 30),
		TotalAmount:       100_000,
		OutstandingAmount: 0,
		PaymentMethod:     models.PurchasePaymentTransfer,
		Status:            models.PurchaseStatusCompleted,
		ApprovalStatus:    models.PurchaseApprovalApproved,
	}
	require.NoError(t, db.Create(&purchase).Error)

	sourceID := uint64(purchase.ID)
	postedAt := purchaseDate.Add(12 * time.Hour)
	postedBy := uint64(financeManager.ID)
	originalJournal := models.SSOTJournalEntry{
		EntryNumber: "JE-COA-RECLASS-ORIGINAL",
		SourceType:  models.SSOTSourceTypePurchase,
		SourceID:    &sourceID,
		SourceCode:  purchase.Code,
		EntryDate:   purchaseDate,
		Description: "Jurnal pembelian asal",
		Status:      models.SSOTStatusPosted,
		TotalDebit:  decimal.NewFromInt(100_000),
		TotalCredit: decimal.NewFromInt(100_000),
		IsBalanced:  true,
		CreatedBy:   uint64(accounting.ID),
		PostedBy:    &postedBy,
		PostedAt:    &postedAt,
	}
	require.NoError(t, db.Create(&originalJournal).Error)

	originalLine := models.SSOTJournalLine{
		JournalID:   originalJournal.ID,
		AccountID:   uint64(originalAccount.ID),
		LineNumber:  1,
		Description: "Beban yang seharusnya direklasifikasi",
		DebitAmount: decimal.NewFromInt(100_000),
	}
	bankLine := models.SSOTJournalLine{
		JournalID:    originalJournal.ID,
		AccountID:    uint64(bankAccount.ID),
		LineNumber:   2,
		Description:  "Kas/bank asal",
		CreditAmount: decimal.NewFromInt(100_000),
	}
	require.NoError(t, db.Create(&originalLine).Error)
	require.NoError(t, db.Create(&bankLine).Error)

	return purchaseCOAReclassificationFixture{
		db:              db,
		service:         NewPostedTransactionCorrectionService(db),
		accounting:      accounting,
		financeManager:  financeManager,
		purchase:        purchase,
		originalJournal: originalJournal,
		originalLine:    originalLine,
		originalAccount: originalAccount,
		replacementCOA:  replacementCOA,
		bankAccount:     bankAccount,
		cashBank:        cashBank,
	}
}

func createPurchaseCOAReclassificationUser(t *testing.T, db *gorm.DB, username, role string) models.User {
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

func TestPurchaseCOAReclassificationCreatesAuditJournalWithoutCashBankMutation(t *testing.T) {
	fixture := newPurchaseCOAReclassificationFixture(t)

	candidates, err := fixture.service.ListPurchaseCOAReclassificationCandidates(
		fixture.purchase.ID,
		fixture.accounting.ID,
		fixture.accounting.Role,
	)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, fixture.originalLine.ID, candidates[0].ID)
	require.Equal(t, fixture.originalAccount.ID, candidates[0].AccountID)

	accounts, err := fixture.service.ListCOAReclassificationAccounts(fixture.accounting.ID, fixture.accounting.Role)
	require.NoError(t, err)
	require.Contains(t, accounts, COAReclassificationAccountCandidate{
		ID: fixture.replacementCOA.ID, Code: fixture.replacementCOA.Code, Name: fixture.replacementCOA.Name,
	})
	for _, account := range accounts {
		require.NotEqual(t, fixture.bankAccount.ID, account.ID)
	}

	correction, err := fixture.service.Create(models.PostedTransactionCorrectionCreateRequest{
		PurchaseID:            &fixture.purchase.ID,
		OriginalJournalLineID: &fixture.originalLine.ID,
		ProposedAccountID:     &fixture.replacementCOA.ID,
		CorrectionType:        models.PostedTransactionCorrectionTypeCOAReclassification,
		Reason:                "COA beban pada jurnal pembelian perlu direklasifikasi ke piutang intercompany.",
	}, fixture.accounting.ID, fixture.accounting.Role)
	require.NoError(t, err)
	require.Equal(t, models.PostedTransactionCorrectionStatusPendingFinanceManager, correction.Status)
	require.Equal(t, fixture.originalJournal.ID, correction.OriginalJournalEntryID)
	require.NotNil(t, correction.OriginalJournalLineID)
	require.Equal(t, fixture.originalLine.ID, *correction.OriginalJournalLineID)
	require.NotNil(t, correction.OriginalAccountID)
	require.Equal(t, fixture.originalAccount.ID, *correction.OriginalAccountID)
	require.Len(t, correction.Events, 2)

	_, err = fixture.service.ReviewAccounting(correction.ID, models.PostedTransactionCorrectionAccountingReviewRequest{
		Resolution: models.PostedTransactionCorrectionResolutionJournalReclass,
		Notes:      "Tidak boleh meninjau ulang jalur koreksi COA langsung.",
	}, fixture.accounting.ID, fixture.accounting.Role)
	require.Error(t, err)
	require.Contains(t, err.Error(), "langsung oleh Accounting")

	var originalJournalBefore models.SSOTJournalEntry
	var originalLinesBefore []models.SSOTJournalLine
	var cashBankBefore models.CashBank
	require.NoError(t, fixture.db.First(&originalJournalBefore, fixture.originalJournal.ID).Error)
	require.NoError(t, fixture.db.Where("journal_id = ?", fixture.originalJournal.ID).Order("line_number ASC").Find(&originalLinesBefore).Error)
	require.NoError(t, fixture.db.First(&cashBankBefore, fixture.cashBank.ID).Error)

	completed, err := fixture.service.DecideFinanceManager(correction.ID, models.PostedTransactionCorrectionFinanceManagerDecisionRequest{
		Approve: true,
		Notes:   "Disetujui setelah COA dan bukti pendukung ditinjau.",
	}, fixture.financeManager.ID, fixture.financeManager.Role)
	require.NoError(t, err)
	require.Equal(t, models.PostedTransactionCorrectionStatusCompleted, completed.Status)
	require.NotNil(t, completed.CompletionJournalEntryID)
	require.Len(t, completed.Events, 4)

	var adjustmentJournal models.SSOTJournalEntry
	require.NoError(t, fixture.db.Preload("Lines").First(&adjustmentJournal, *completed.CompletionJournalEntryID).Error)
	require.Equal(t, models.SSOTStatusPosted, adjustmentJournal.Status)
	require.Equal(t, models.SSOTSourceTypeAdjustment, adjustmentJournal.SourceType)
	require.Equal(t, fixture.purchase.Code, adjustmentJournal.SourceCode)
	require.NotNil(t, adjustmentJournal.AdjustmentOf)
	require.Equal(t, fixture.originalJournal.ID, *adjustmentJournal.AdjustmentOf)
	require.Equal(t, models.JournalAdjustmentCategoryPostedCOAReclassification, adjustmentJournal.AdjustmentCategoryCode)
	require.True(t, adjustmentJournal.IsAutoGenerated)
	require.True(t, adjustmentJournal.TotalDebit.Equal(decimal.NewFromInt(100_000)))
	require.True(t, adjustmentJournal.TotalCredit.Equal(decimal.NewFromInt(100_000)))
	require.True(t, hasCOAReclassificationLine(adjustmentJournal.Lines, uint64(fixture.replacementCOA.ID), decimal.NewFromInt(100_000), decimal.Zero))
	require.True(t, hasCOAReclassificationLine(adjustmentJournal.Lines, uint64(fixture.originalAccount.ID), decimal.Zero, decimal.NewFromInt(100_000)))

	var originalJournalAfter models.SSOTJournalEntry
	var originalLinesAfter []models.SSOTJournalLine
	var cashBankAfter models.CashBank
	var cashBankTransactionCount int64
	require.NoError(t, fixture.db.First(&originalJournalAfter, fixture.originalJournal.ID).Error)
	require.NoError(t, fixture.db.Where("journal_id = ?", fixture.originalJournal.ID).Order("line_number ASC").Find(&originalLinesAfter).Error)
	require.NoError(t, fixture.db.First(&cashBankAfter, fixture.cashBank.ID).Error)
	require.NoError(t, fixture.db.Model(&models.CashBankTransaction{}).Where("cash_bank_id = ?", fixture.cashBank.ID).Count(&cashBankTransactionCount).Error)

	require.Equal(t, originalJournalBefore.Status, originalJournalAfter.Status)
	require.True(t, originalJournalBefore.TotalDebit.Equal(originalJournalAfter.TotalDebit))
	require.True(t, originalJournalBefore.TotalCredit.Equal(originalJournalAfter.TotalCredit))
	require.Equal(t, originalLinesBefore, originalLinesAfter)
	require.Equal(t, cashBankBefore.Balance, cashBankAfter.Balance)
	require.Equal(t, cashBankBefore.BookBalanceIDR, cashBankAfter.BookBalanceIDR)
	require.Zero(t, cashBankTransactionCount)
}

func hasCOAReclassificationLine(lines []models.SSOTJournalLine, accountID uint64, debitAmount, creditAmount decimal.Decimal) bool {
	for _, line := range lines {
		if line.AccountID == accountID && line.DebitAmount.Equal(debitAmount) && line.CreditAmount.Equal(creditAmount) {
			return true
		}
	}
	return false
}
