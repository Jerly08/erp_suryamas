package services

import (
	"testing"

	"app-sistem-akuntansi/models"

	"github.com/stretchr/testify/require"
)

func TestValidateExpensePaymentAccountUsageRejectsLiabilityOverpayment(t *testing.T) {
	account := models.Account{
		Code:    "2102-006",
		Name:    "HUTANG PPH PASAL 22",
		Type:    models.AccountTypeLiability,
		Balance: 100000,
	}

	err := validateExpensePaymentAccountUsage(account, 150000, 0, false)

	require.Error(t, err)
	require.Contains(t, err.Error(), "overpayment")
}

func TestValidateExpensePaymentAccountUsageAllowsConfirmedLiabilityOverpayment(t *testing.T) {
	account := models.Account{
		Code:    "2102-006",
		Name:    "HUTANG PPH PASAL 22",
		Type:    models.AccountTypeLiability,
		Balance: 100000,
	}

	err := validateExpensePaymentAccountUsage(account, 150000, 0, true)

	require.NoError(t, err)
}

func TestValidateExpensePaymentAccountUsageAllowsLiabilityWithinBalance(t *testing.T) {
	account := models.Account{
		Code:    "2102-006",
		Name:    "HUTANG PPH PASAL 22",
		Type:    models.AccountTypeLiability,
		Balance: 100000,
	}

	err := validateExpensePaymentAccountUsage(account, 90000, 0, false)

	require.NoError(t, err)
}

func TestValidateExpensePaymentAccountUsageRejectsPPhWithLiabilitySettlement(t *testing.T) {
	account := models.Account{
		Code:    "2102-006",
		Name:    "HUTANG PPH PASAL 22",
		Type:    models.AccountTypeLiability,
		Balance: 100000,
	}

	err := validateExpensePaymentAccountUsage(account, 90000, 1000, true)

	require.Error(t, err)
	require.Contains(t, err.Error(), "PPh withholding")
}

func TestValidateExpensePaymentAccountUsageSkipsExpenseAccounts(t *testing.T) {
	account := models.Account{
		Code:    "6101-001",
		Name:    "BEBAN OPERASIONAL",
		Type:    models.AccountTypeExpense,
		Balance: 0,
	}

	err := validateExpensePaymentAccountUsage(account, 150000, 1000, false)

	require.NoError(t, err)
}
