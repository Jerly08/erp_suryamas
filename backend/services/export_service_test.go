package services

import (
	"testing"

	"app-sistem-akuntansi/models"
	"github.com/stretchr/testify/require"
)

func TestSortAccountsForExportSortsHierarchicalCodes(t *testing.T) {
	accounts := []models.Account{
		{Code: "5400-006"},
		{Code: "5400-003"},
		{Code: "5400-002"},
		{Code: "5400-008"},
		{Code: "5400-010"},
		{Code: "1000"},
		{Code: "5400-001"},
		{Code: "5400-009"},
		{Code: "5400-004"},
		{Code: "5400"},
		{Code: "1101-010"},
		{Code: "1101-002"},
		{Code: "1101"},
		{Code: "1101-001"},
	}

	sortAccountsForExport(accounts)

	codes := make([]string, len(accounts))
	for index, account := range accounts {
		codes[index] = account.Code
	}

	require.Equal(t, []string{
		"1000",
		"1101",
		"1101-001",
		"1101-002",
		"1101-010",
		"5400",
		"5400-001",
		"5400-002",
		"5400-003",
		"5400-004",
		"5400-006",
		"5400-008",
		"5400-009",
		"5400-010",
	}, codes)
}
