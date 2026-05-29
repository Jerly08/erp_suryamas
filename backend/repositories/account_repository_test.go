package repositories

import (
	"context"
	"testing"

	"app-sistem-akuntansi/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAccountRepoTest(t *testing.T) (*AccountRepo, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Account{}))

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
