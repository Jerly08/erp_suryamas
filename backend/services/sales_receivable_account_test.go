package services

import (
	"testing"

	"app-sistem-akuntansi/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestResolveSalesReceivableAccountUsesSaleSelectedAccount(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	require.NoError(t, db.AutoMigrate(&models.Account{}))

	defaultAR := models.Account{
		Code:     "1102-001",
		Name:     "PIUTANG USAHA - PIHAK KETIGA",
		Type:     models.AccountTypeAsset,
		IsActive: true,
		IsHeader: false,
	}
	selectedIntercompanyAR := models.Account{
		Code:     "1106-001",
		Name:     "INTERCOMP - TOMBROK JAYA PERMAI",
		Type:     models.AccountTypeAsset,
		IsActive: true,
		IsHeader: false,
	}

	require.NoError(t, db.Create(&defaultAR).Error)
	require.NoError(t, db.Create(&selectedIntercompanyAR).Error)

	sale := &models.Sale{
		PaymentMethodType:   "CREDIT",
		ReceivableAccountID: &selectedIntercompanyAR.ID,
	}

	account, err := resolveSalesReceivableAccount(db, sale)

	require.NoError(t, err)
	require.Equal(t, selectedIntercompanyAR.ID, account.ID)
	require.Equal(t, "1106-001", account.Code)
}
