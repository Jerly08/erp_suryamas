package repositories

import (
	"context"
	"testing"

	"app-sistem-akuntansi/models"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAccountRepoTest(t *testing.T) (*AccountRepo, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Account{}, &models.SSOTJournalEntry{}, &models.SSOTJournalLine{}))
	require.NoError(t, db.Create(&models.User{
		ID:       1,
		Username: "admin",
		Email:    "admin@example.com",
		Password: "test",
		Role:     "admin",
		IsActive: true,
	}).Error)

	return NewAccountRepo(db), db
}

func seedAccount(t *testing.T, db *gorm.DB, account *models.Account) {
	t.Helper()
	require.NoError(t, db.Create(account).Error)
}

func TestAccountRepoCreateNormalizesRelativeChildCode(t *testing.T) {
	repo, db := setupAccountRepoTest(t)
	ctx := context.Background()

	root := models.Account{
		Code:     "1101",
		Name:     "KAS DAN SETARA KAS",
		Type:     models.AccountTypeAsset,
		Level:    1,
		IsHeader: true,
		IsActive: true,
	}
	seedAccount(t, db, &root)

	parent := models.Account{
		Code:     "1101-003",
		Name:     "BANK - GIRO",
		Type:     models.AccountTypeAsset,
		ParentID: &root.ID,
		Level:    2,
		IsHeader: true,
		IsActive: true,
	}
	seedAccount(t, db, &parent)

	account, err := repo.Create(ctx, &models.AccountCreateRequest{
		Code:     "1001",
		Name:     "BANK MANDIRI 1030015855881",
		Type:     models.AccountTypeAsset,
		ParentID: &parent.ID,
	})

	require.NoError(t, err)
	require.Equal(t, "1101-003-1001", account.Code)
	require.Equal(t, parent.ID, *account.ParentID)
	require.Equal(t, 3, account.Level)
}

func TestAccountRepoCreateKeepsFullChildCodeWithParentPrefix(t *testing.T) {
	repo, db := setupAccountRepoTest(t)
	ctx := context.Background()

	parent := models.Account{
		Code:     "1101-003",
		Name:     "BANK - GIRO",
		Type:     models.AccountTypeAsset,
		Level:    2,
		IsHeader: true,
		IsActive: true,
	}
	seedAccount(t, db, &parent)

	account, err := repo.Create(ctx, &models.AccountCreateRequest{
		Code:     "1101-003-1002",
		Name:     "BANK BCA",
		Type:     models.AccountTypeAsset,
		ParentID: &parent.ID,
	})

	require.NoError(t, err)
	require.Equal(t, "1101-003-1002", account.Code)
}

func TestAccountRepoCreateRejectsNormalizedCodeLongerThanStorageLimit(t *testing.T) {
	repo, db := setupAccountRepoTest(t)
	ctx := context.Background()

	parent := models.Account{
		Code:     "1101-003",
		Name:     "BANK - GIRO",
		Type:     models.AccountTypeAsset,
		Level:    2,
		IsHeader: true,
		IsActive: true,
	}
	seedAccount(t, db, &parent)

	_, err := repo.Create(ctx, &models.AccountCreateRequest{
		Code:     "1101-004-1001",
		Name:     "BANK CUSTOM",
		Type:     models.AccountTypeAsset,
		ParentID: &parent.ID,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "20")
}

func TestAccountRepoCreateAllowsFlexibleChildSegment(t *testing.T) {
	repo, db := setupAccountRepoTest(t)
	ctx := context.Background()

	parent := models.Account{
		Code:     "1101-003",
		Name:     "BANK - GIRO",
		Type:     models.AccountTypeAsset,
		Level:    2,
		IsHeader: true,
		IsActive: true,
	}
	seedAccount(t, db, &parent)

	account, err := repo.Create(ctx, &models.AccountCreateRequest{
		Code:     "MANDIRI-881",
		Name:     "BANK MANDIRI",
		Type:     models.AccountTypeAsset,
		ParentID: &parent.ID,
	})

	require.NoError(t, err)
	require.Equal(t, "1101-003-MANDIRI-881", account.Code)
}

func TestAccountRepoCreatePostsOpeningBalanceJournal(t *testing.T) {
	repo, db := setupAccountRepoTest(t)
	ctx := context.Background()

	root := models.Account{
		Code:     "1101",
		Name:     "KAS DAN SETARA KAS",
		Type:     models.AccountTypeAsset,
		Level:    1,
		IsHeader: true,
		IsActive: true,
	}
	seedAccount(t, db, &root)

	parent := models.Account{
		Code:     "1101-003",
		Name:     "BANK - GIRO",
		Type:     models.AccountTypeAsset,
		ParentID: &root.ID,
		Level:    2,
		IsHeader: true,
		IsActive: true,
	}
	seedAccount(t, db, &parent)

	equityHeader := models.Account{
		Code:     "3000",
		Name:     "EKUITAS",
		Type:     models.AccountTypeEquity,
		Level:    1,
		IsHeader: true,
		IsActive: true,
	}
	seedAccount(t, db, &equityHeader)

	openingEquity := models.Account{
		Code:     "3104",
		Name:     "EKUITAS SALDO AWAL",
		Type:     models.AccountTypeEquity,
		Category: models.CategoryEquity,
		ParentID: &equityHeader.ID,
		Level:    2,
		IsActive: true,
	}
	seedAccount(t, db, &openingEquity)

	account, err := repo.Create(ctx, &models.AccountCreateRequest{
		Code:           "1003",
		Name:           "BANK BRI",
		Type:           models.AccountTypeAsset,
		ParentID:       &parent.ID,
		OpeningBalance: 2000000,
	})

	require.NoError(t, err)
	require.Equal(t, "1101-003-1003", account.Code)
	require.Equal(t, 2000000.0, account.Balance)

	var journal models.SSOTJournalEntry
	require.NoError(t, db.Preload("Lines").
		Where("source_type = ? AND source_code = ?", models.SSOTSourceTypeOpening, account.Code).
		First(&journal).Error)

	require.Equal(t, models.SSOTStatusPosted, journal.Status)
	require.True(t, journal.TotalDebit.Equal(decimal.NewFromInt(2000000)))
	require.True(t, journal.TotalCredit.Equal(decimal.NewFromInt(2000000)))
	require.Len(t, journal.Lines, 2)

	linesByAccountID := map[uint64]models.SSOTJournalLine{}
	for _, line := range journal.Lines {
		linesByAccountID[line.AccountID] = line
	}

	accountLine := linesByAccountID[uint64(account.ID)]
	require.True(t, accountLine.DebitAmount.Equal(decimal.NewFromInt(2000000)))
	require.True(t, accountLine.CreditAmount.IsZero())

	equityLine := linesByAccountID[uint64(openingEquity.ID)]
	require.True(t, equityLine.DebitAmount.IsZero())
	require.True(t, equityLine.CreditAmount.Equal(decimal.NewFromInt(2000000)))

	var persistedAccount models.Account
	require.NoError(t, db.First(&persistedAccount, account.ID).Error)
	require.Equal(t, 2000000.0, persistedAccount.Balance)

	var persistedEquity models.Account
	require.NoError(t, db.First(&persistedEquity, openingEquity.ID).Error)
	require.Equal(t, 2000000.0, persistedEquity.Balance)
}
