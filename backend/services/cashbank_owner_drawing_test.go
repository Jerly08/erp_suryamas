package services

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type ownerDrawingFixture struct {
	service          *CashBankService
	cashBank         models.CashBank
	priveAccount     models.Account
	retainedEarnings models.Account
	expenseAccount   models.Account
	financeManager   models.User
	accounting       models.User
}

func setupOwnerDrawingTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Account{},
		&models.AccountingPeriod{},
		&models.CashBank{},
		&models.CashBankTransaction{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
	))

	return db
}

func seedOwnerDrawingFixture(t *testing.T, db *gorm.DB) ownerDrawingFixture {
	t.Helper()

	cashGL := models.Account{
		Code:     "1101-TEST",
		Name:     "BANK TEST",
		Type:     models.AccountTypeAsset,
		Category: models.CategoryCurrentAsset,
		IsActive: true,
		Balance:  500_000,
	}
	prive := models.Account{
		Code:     "3102",
		Name:     "OTHER PRIVE",
		Type:     models.AccountTypeEquity,
		Category: models.CategoryEquity,
		IsActive: true,
	}
	retainedEarnings := models.Account{
		Code:     "3103",
		Name:     "LABA DITAHAN",
		Type:     models.AccountTypeEquity,
		Category: models.CategoryRetainedEarnings,
		IsActive: true,
	}
	expense := models.Account{
		Code:     "6101",
		Name:     "BEBAN OPERASIONAL TEST",
		Type:     models.AccountTypeExpense,
		Category: models.CategoryOperatingExpense,
		IsActive: true,
	}
	for _, account := range []*models.Account{&cashGL, &prive, &retainedEarnings, &expense} {
		require.NoError(t, db.Create(account).Error)
	}

	cashBank := models.CashBank{
		Code:             "BNK-OWNER-DRAWING-TEST",
		Name:             "Bank Owner Drawing Test",
		Type:             models.CashBankTypeBank,
		AccountID:        cashGL.ID,
		Currency:         "IDR",
		Balance:          500_000,
		BookBalanceIDR:   500_000,
		LastExchangeRate: 1,
		IsActive:         true,
	}
	require.NoError(t, db.Create(&cashBank).Error)

	// The currency guard derives the authoritative balance from transaction
	// history, so model the opening balance explicitly just as production does.
	require.NoError(t, db.Create(&models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   TransactionTypeOpeningBalance,
		Amount:          500_000,
		Currency:        "IDR",
		ExchangeRate:    1,
		BookAmountIDR:   500_000,
		BalanceAfter:    500_000,
		TransactionDate: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		Notes:           "Opening balance",
	}).Error)

	financeManager := models.User{
		Username: "owner-drawing-finance-manager",
		Email:    "owner-drawing-finance-manager@test.local",
		Password: "secret",
		Role:     models.RoleFinanceManager,
		IsActive: true,
	}
	accounting := models.User{
		Username: "owner-drawing-accounting",
		Email:    "owner-drawing-accounting@test.local",
		Password: "secret",
		Role:     models.RoleAccounting,
		IsActive: true,
	}
	require.NoError(t, db.Create(&financeManager).Error)
	require.NoError(t, db.Create(&accounting).Error)

	return ownerDrawingFixture{
		service: NewCashBankService(
			db,
			repositories.NewCashBankRepository(db),
			repositories.NewAccountRepository(db),
		),
		cashBank:         cashBank,
		priveAccount:     prive,
		retainedEarnings: retainedEarnings,
		expenseAccount:   expense,
		financeManager:   financeManager,
		accounting:       accounting,
	}
}

func ownerDrawingRequest(f ownerDrawingFixture, targetAccountID uint) WithdrawalRequest {
	date := time.Date(2026, time.August, 21, 0, 0, 0, 0, time.UTC)
	return WithdrawalRequest{
		AccountID:       f.cashBank.ID,
		Date:            CustomDate(date),
		Amount:          125_000,
		Purpose:         WithdrawalPurposeOwnerDrawing,
		TargetAccountID: &targetAccountID,
		Reference:       "PRV-20260821-001",
		Notes:           "Pengambilan pemilik sesuai persetujuan",
	}
}

func cashBankTransactionCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&models.CashBankTransaction{}).Count(&count).Error)
	return count
}

func TestProcessWithdrawalOwnerDrawingPostsControlledPriveJournal(t *testing.T) {
	db := setupOwnerDrawingTestDB(t)
	fixture := seedOwnerDrawingFixture(t, db)

	transaction, err := fixture.service.ProcessWithdrawal(
		ownerDrawingRequest(fixture, fixture.priveAccount.ID),
		fixture.financeManager.ID,
	)
	require.NoError(t, err)
	require.Equal(t, TransactionTypeOwnerDrawing, transaction.ReferenceType)
	require.InDelta(t, -125_000, transaction.Amount, 0.001)
	require.InDelta(t, -125_000, transaction.BookAmountIDR, 0.001)
	require.Contains(t, transaction.Notes, "Ref PRV-20260821-001")
	require.Contains(t, transaction.Notes, "Owner drawing / prive")
	require.Contains(t, transaction.Notes, fixture.priveAccount.Code)

	var reloadedCashBank models.CashBank
	require.NoError(t, db.First(&reloadedCashBank, fixture.cashBank.ID).Error)
	require.InDelta(t, 375_000, reloadedCashBank.Balance, 0.001)

	var journal models.SSOTJournalEntry
	require.NoError(t, db.Preload("Lines").Where("source_code = ?", fmt.Sprintf("OWNER_DRAWING-%d", transaction.ID)).First(&journal).Error)
	require.Equal(t, models.SSOTStatusPosted, journal.Status)
	require.Equal(t, "PRV-20260821-001", journal.Reference)
	require.Len(t, journal.Lines, 2)
	require.Equal(t, uint64(fixture.priveAccount.ID), journal.Lines[0].AccountID)
	require.True(t, journal.Lines[0].DebitAmount.Equal(journal.TotalDebit))
	require.Equal(t, uint64(fixture.cashBank.AccountID), journal.Lines[1].AccountID)
	require.True(t, journal.Lines[1].CreditAmount.Equal(journal.TotalCredit))
}

func TestProcessWithdrawalOwnerDrawingRejectsAccountingWithoutSideEffects(t *testing.T) {
	db := setupOwnerDrawingTestDB(t)
	fixture := seedOwnerDrawingFixture(t, db)

	_, err := fixture.service.ProcessWithdrawal(
		ownerDrawingRequest(fixture, fixture.priveAccount.ID),
		fixture.accounting.ID,
	)
	require.ErrorContains(t, err, "Finance Manager or Director")
	require.EqualValues(t, 1, cashBankTransactionCount(t, db), "only opening balance may exist")

	var reloadedCashBank models.CashBank
	require.NoError(t, db.First(&reloadedCashBank, fixture.cashBank.ID).Error)
	require.InDelta(t, 500_000, reloadedCashBank.Balance, 0.001)
}

func TestProcessWithdrawalOwnerDrawingAllowsDirector(t *testing.T) {
	db := setupOwnerDrawingTestDB(t)
	fixture := seedOwnerDrawingFixture(t, db)
	director := models.User{
		Username: "owner-drawing-director",
		Email:    "owner-drawing-director@test.local",
		Password: "secret",
		Role:     models.RoleDirector,
		IsActive: true,
	}
	require.NoError(t, db.Create(&director).Error)

	transaction, err := fixture.service.ProcessWithdrawal(
		ownerDrawingRequest(fixture, fixture.priveAccount.ID),
		director.ID,
	)
	require.NoError(t, err)
	require.Equal(t, TransactionTypeOwnerDrawing, transaction.ReferenceType)
}

func TestProcessWithdrawalOwnerDrawingRejectsRetainedEarningsWithoutSideEffects(t *testing.T) {
	db := setupOwnerDrawingTestDB(t)
	fixture := seedOwnerDrawingFixture(t, db)

	_, err := fixture.service.ProcessWithdrawal(
		ownerDrawingRequest(fixture, fixture.retainedEarnings.ID),
		fixture.financeManager.ID,
	)
	require.ErrorContains(t, err, "not an eligible prive/owner drawing equity account")
	require.EqualValues(t, 1, cashBankTransactionCount(t, db))

	var reloadedCashBank models.CashBank
	require.NoError(t, db.First(&reloadedCashBank, fixture.cashBank.ID).Error)
	require.InDelta(t, 500_000, reloadedCashBank.Balance, 0.001)
}

func TestProcessWithdrawalRejectsNonOwnerDrawingPurposesWithoutSideEffects(t *testing.T) {
	db := setupOwnerDrawingTestDB(t)
	fixture := seedOwnerDrawingFixture(t, db)
	targetAccountID := fixture.expenseAccount.ID

	testCases := []struct {
		name          string
		purpose       string
		expectedRoute string
	}{
		{
			name:          "missing purpose",
			purpose:       "",
			expectedRoute: "Payment workflow",
		},
		{
			name:          "operating expense",
			purpose:       WithdrawalPurposeOperatingExpense,
			expectedRoute: "Payment workflow",
		},
		{
			name:          "non-operating expense",
			purpose:       WithdrawalPurposeNonOperatingExpense,
			expectedRoute: "Payment workflow",
		},
		{
			name:          "liability payment",
			purpose:       WithdrawalPurposeLiabilityPayment,
			expectedRoute: "Payment workflow",
		},
		{
			name:          "asset acquisition",
			purpose:       WithdrawalPurposeAssetAcquisition,
			expectedRoute: "Purchase workflow",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			request := WithdrawalRequest{
				AccountID:       fixture.cashBank.ID,
				Date:            CustomDate(time.Date(2026, time.August, 21, 0, 0, 0, 0, time.UTC)),
				Amount:          25_000,
				Purpose:         testCase.purpose,
				TargetAccountID: &targetAccountID,
				Reference:       "WIT-20260821-001",
				Notes:           "Generic direct withdrawal must be rejected",
			}

			_, err := fixture.service.ProcessWithdrawal(request, fixture.accounting.ID)
			require.ErrorContains(t, err, "invalid withdrawal purpose")
			require.ErrorContains(t, err, "OWNER_DRAWING")
			require.ErrorContains(t, err, testCase.expectedRoute)
			require.EqualValues(t, 1, cashBankTransactionCount(t, db), "only opening balance may exist")

			var reloadedCashBank models.CashBank
			require.NoError(t, db.First(&reloadedCashBank, fixture.cashBank.ID).Error)
			require.InDelta(t, 500_000, reloadedCashBank.Balance, 0.001)
		})
	}
}

func TestWithdrawalJournalRequiresExplicitCounterAccount(t *testing.T) {
	db := setupOwnerDrawingTestDB(t)
	fixture := seedOwnerDrawingFixture(t, db)

	transaction := &models.CashBankTransaction{ID: 99}
	journalRequest := &CashBankJournalRequest{
		TransactionType: TransactionTypeOwnerDrawing,
		CashBankID:      uint64(fixture.cashBank.ID),
		Date:            time.Date(2026, time.August, 21, 0, 0, 0, 0, time.UTC),
		Reference:       "PRV-20260821-NO-COUNTER",
		Description:     "Must not create a default expense journal",
		CreatedBy:       uint64(fixture.financeManager.ID),
	}

	tx := db.Begin()
	require.NoError(t, tx.Error)
	defer tx.Rollback()

	_, err := fixture.service.sSOTJournalAdapter.CreateWithdrawalJournalEntryWithTx(
		tx,
		&fixture.cashBank,
		transaction,
		journalRequest,
	)
	require.ErrorContains(t, err, "requires an explicit counter account")
	require.ErrorContains(t, err, "default expense fallback is disabled")

	var journalCount int64
	require.NoError(t, db.Model(&models.SSOTJournalEntry{}).Count(&journalCount).Error)
	require.Zero(t, journalCount, "the adapter must reject before creating a journal")
}

func TestWithdrawalJournalRejectsGenericTransactionTypeEvenWithCounterAccount(t *testing.T) {
	db := setupOwnerDrawingTestDB(t)
	fixture := seedOwnerDrawingFixture(t, db)

	counterAccountID := uint64(fixture.expenseAccount.ID)
	journalRequest := &CashBankJournalRequest{
		TransactionType:  TransactionTypeWithdrawal,
		CashBankID:       uint64(fixture.cashBank.ID),
		Amount:           decimal.NewFromInt(25_000),
		Date:             time.Date(2026, time.August, 21, 0, 0, 0, 0, time.UTC),
		Reference:        "WIT-20260821-GENERIC",
		Description:      "Generic direct withdrawal must be blocked",
		CounterAccountID: &counterAccountID,
		CreatedBy:        uint64(fixture.financeManager.ID),
	}

	tx := db.Begin()
	require.NoError(t, tx.Error)
	defer tx.Rollback()

	_, err := fixture.service.sSOTJournalAdapter.CreateWithdrawalJournalEntryWithTx(
		tx,
		&fixture.cashBank,
		&models.CashBankTransaction{ID: 100},
		journalRequest,
	)
	require.ErrorContains(t, err, "restricted to OWNER_DRAWING")

	var journalCount int64
	require.NoError(t, db.Model(&models.SSOTJournalEntry{}).Count(&journalCount).Error)
	require.Zero(t, journalCount, "the adapter must reject before creating a generic withdrawal journal")
}

func TestWithdrawalJournalRejectsForgedOwnerDrawingWithNonPriveAccount(t *testing.T) {
	db := setupOwnerDrawingTestDB(t)
	fixture := seedOwnerDrawingFixture(t, db)

	counterAccountID := uint64(fixture.expenseAccount.ID)
	journalRequest := &CashBankJournalRequest{
		TransactionType:  TransactionTypeOwnerDrawing,
		CashBankID:       uint64(fixture.cashBank.ID),
		Amount:           decimal.NewFromInt(25_000),
		Date:             time.Date(2026, time.August, 21, 0, 0, 0, 0, time.UTC),
		Reference:        "PRV-20260821-FORGED",
		Description:      "Forged owner drawing must be blocked",
		CounterAccountID: &counterAccountID,
		CreatedBy:        uint64(fixture.financeManager.ID),
	}

	tx := db.Begin()
	require.NoError(t, tx.Error)
	defer tx.Rollback()

	_, err := fixture.service.sSOTJournalAdapter.CreateWithdrawalJournalEntryWithTx(
		tx,
		&fixture.cashBank,
		&models.CashBankTransaction{ID: 101},
		journalRequest,
	)
	require.ErrorContains(t, err, "not an eligible prive/owner drawing equity account")

	var journalCount int64
	require.NoError(t, db.Model(&models.SSOTJournalEntry{}).Count(&journalCount).Error)
	require.Zero(t, journalCount, "the adapter must reject before creating a forged owner drawing journal")
}

func TestLegacyWithdrawalEntryPointsDelegateToControlledOwnerDrawing(t *testing.T) {
	t.Run("simple service", func(t *testing.T) {
		db := setupOwnerDrawingTestDB(t)
		fixture := seedOwnerDrawingFixture(t, db)

		transaction, err := fixture.service.ProcessWithdrawalSimple(
			ownerDrawingRequest(fixture, fixture.priveAccount.ID),
			fixture.financeManager.ID,
		)
		require.NoError(t, err)
		require.Equal(t, TransactionTypeOwnerDrawing, transaction.ReferenceType)
	})

	t.Run("enhanced service accepts legacy owner drawing target", func(t *testing.T) {
		db := setupOwnerDrawingTestDB(t)
		fixture := seedOwnerDrawingFixture(t, db)
		enhancedService := NewCashBankEnhancedService(
			db,
			repositories.NewCashBankRepository(db),
			repositories.NewAccountRepository(db),
		)

		request := ownerDrawingRequest(fixture, fixture.priveAccount.ID)
		request.TargetAccountID = nil
		transaction, err := enhancedService.ProcessWithdrawalV2(
			request,
			fixture.financeManager.ID,
			fixture.priveAccount.ID,
		)
		require.NoError(t, err)
		require.Equal(t, TransactionTypeOwnerDrawing, transaction.ReferenceType)
	})
}

func TestLegacyWithdrawalEntryPointsRejectGenericDirectWithdrawals(t *testing.T) {
	t.Run("simple service", func(t *testing.T) {
		db := setupOwnerDrawingTestDB(t)
		fixture := seedOwnerDrawingFixture(t, db)
		request := ownerDrawingRequest(fixture, fixture.expenseAccount.ID)
		request.Purpose = WithdrawalPurposeOperatingExpense

		_, err := fixture.service.ProcessWithdrawalSimple(request, fixture.accounting.ID)
		require.ErrorContains(t, err, "invalid withdrawal purpose")
		require.ErrorContains(t, err, "Payment workflow")
		require.EqualValues(t, 1, cashBankTransactionCount(t, db))
	})

	t.Run("enhanced service", func(t *testing.T) {
		db := setupOwnerDrawingTestDB(t)
		fixture := seedOwnerDrawingFixture(t, db)
		enhancedService := NewCashBankEnhancedService(
			db,
			repositories.NewCashBankRepository(db),
			repositories.NewAccountRepository(db),
		)
		request := ownerDrawingRequest(fixture, fixture.expenseAccount.ID)
		request.Purpose = WithdrawalPurposeOperatingExpense

		_, err := enhancedService.ProcessWithdrawalV2(request, fixture.accounting.ID, fixture.expenseAccount.ID)
		require.ErrorContains(t, err, "invalid withdrawal purpose")
		require.ErrorContains(t, err, "Payment workflow")
		require.EqualValues(t, 1, cashBankTransactionCount(t, db))
	})
}

func TestOwnerDrawingValidationAndLookupOnlyExposeEligiblePriveAccounts(t *testing.T) {
	db := setupOwnerDrawingTestDB(t)
	fixture := seedOwnerDrawingFixture(t, db)

	accounts, err := fixture.service.GetOwnerDrawingAccounts()
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.Equal(t, fixture.priveAccount.ID, accounts[0].ID)

	request := ownerDrawingRequest(fixture, fixture.priveAccount.ID)
	request.Reference = ""
	_, err = fixture.service.ProcessWithdrawal(request, fixture.financeManager.ID)
	require.ErrorContains(t, err, "owner drawing reference is required")

	request = ownerDrawingRequest(fixture, fixture.priveAccount.ID)
	request.Purpose = "ARBITRARY_EQUITY"
	_, err = fixture.service.ProcessWithdrawal(request, fixture.financeManager.ID)
	require.ErrorContains(t, err, "invalid withdrawal purpose")
	require.EqualValues(t, 1, cashBankTransactionCount(t, db))
}
