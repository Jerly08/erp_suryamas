package controllers

import (
	"testing"
	"time"

	"app-sistem-akuntansi/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTransformToFrontendFormat_SeparatesOtherIncomeExpenseAndTaxLabel(t *testing.T) {
	controller := &SSOTProfitLossController{}

	data := &services.SSOTProfitLossData{
		Company: services.CompanyInfo{
			Name: "PT Test ERP",
		},
		StartDate:       time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		EndDate:         time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		Currency:        "IDR",
		DataSource:      "SSOT",
		OperatingIncome: 300,
		OtherIncome:     50,
		OtherExpenses:   200,
		IncomeBeforeTax: 150,
		TaxExpense:      100,
		NetIncome:       50,
		Enhanced:        true,
		GeneratedAt:     time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC),
	}
	data.Revenue.TotalRevenue = 1000
	data.COGS.TotalCOGS = 400
	data.OperatingExpenses.TotalOpEx = 300
	data.OtherIncomeItems = []services.PLSectionItem{
		{AccountCode: "7103", AccountName: "Laba Selisih Kurs", Amount: 50},
	}
	data.OtherExpenseItems = []services.PLSectionItem{
		{AccountCode: "8103", AccountName: "Rugi Selisih Kurs", Amount: 200},
	}

	result := controller.TransformToFrontendFormat(data, "en")

	sections, ok := result["sections"].([]gin.H)
	require.True(t, ok)

	foundOtherIncome := false
	foundOtherExpenses := false
	foundNetOther := false
	foundTaxLabel := false

	for _, section := range sections {
		name := section["name"].(string)
		switch name {
		case "OTHER INCOME":
			foundOtherIncome = true
		case "OTHER EXPENSES":
			foundOtherExpenses = true
		case "OTHER INCOME/EXPENSES (Net)":
			foundNetOther = true
		}

		items, ok := section["items"].([]gin.H)
		if !ok {
			continue
		}
		for _, item := range items {
			itemName := item["name"].(string)
			if itemName == "Tax Expense" {
				foundTaxLabel = true
			}
			require.NotContains(t, itemName, "(25%)")
		}
	}

	require.True(t, foundOtherIncome)
	require.True(t, foundOtherExpenses)
	require.True(t, foundNetOther)
	require.True(t, foundTaxLabel)
}
