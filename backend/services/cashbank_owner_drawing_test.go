package services

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"

	"github.com/glebarez/sqlite"
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

func TestProcessWithdrawalOwnerDrawingRejectsRetainedEarningsAndKeepsGenericRules(t *testing.T) {
	db := setupOwnerDrawingTestDB(t)
	fixture := seedOwnerDrawingFixture(t, db)

	_, err := fixture.service.ProcessWithdrawal(
		ownerDrawingRequest(fixture, fixture.retainedEarnings.ID),
		fixture.financeManager.ID,
	)
	require.ErrorContains(t, err, "not an eligible prive/owner drawing equity account")
	require.EqualValues(t, 1, cashBankTransactionCount(t, db))

	// A generic withdrawal still has its established behavior: Accounting can
	// post it to an allowed expense account, but cannot use equity simply by
	// omitting the OWNER_DRAWING purpose.
	genericTargetID := fixture.expenseAccount.ID
	genericRequest := WithdrawalRequest{
		AccountID:       fixture.cashBank.ID,
		Date:            CustomDate(time.Date(2026, time.August, 21, 0, 0, 0, 0, time.UTC)),
		Amount:          25_000,
		TargetAccountID: &genericTargetID,
		Reference:       "WIT-20260821-001",
		Notes:           "Beban operasional test",
	}
	transaction, err := fixture.service.ProcessWithdrawal(genericRequest, fixture.accounting.ID)
	require.NoError(t, err)
	require.Equal(t, TransactionTypeWithdrawal, transaction.ReferenceType)

	priveTargetID := fixture.priveAccount.ID
	genericRequest.TargetAccountID = &priveTargetID
	genericRequest.Reference = "WIT-20260821-002"
	_, err = fixture.service.ProcessWithdrawal(genericRequest, fixture.accounting.ID)
	require.ErrorContains(t, err, "counter account 3102 has invalid type EQUITY")
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
