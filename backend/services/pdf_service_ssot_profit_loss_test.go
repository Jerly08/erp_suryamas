package services

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jung-kurt/gofpdf"
	"github.com/stretchr/testify/require"
)

func TestRenderSSOTFinancialSections_RendersNonOperatingDetailsOnce(t *testing.T) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(false)
	pdf.AddPage()

	service := &PDFService{}
	data := map[string]interface{}{
		"other_income":      4084356.0,
		"other_expenses":    2696100.0,
		"income_before_tax": 1388256.0,
		"net_income":        1388256.0,
		"other_income_items": []interface{}{
			map[string]interface{}{"account_code": "7103", "account_name": "LABA SELISIH KURS", "amount": 760000.0},
			map[string]interface{}{"account_code": "7104", "account_name": "PENDAPATAN DIVIDEN", "amount": 3324356.0},
		},
		"other_expense_items": []interface{}{
			map[string]interface{}{"account_code": "8101", "account_name": "BEBAN BUNGA", "amount": 2696100.0},
		},
	}

	data["OtherIncome"] = data["other_income"]
	data["OtherExpenses"] = data["other_expenses"]

	service.renderSSOTFinancialSections(pdf, data, "en")

	var buf bytes.Buffer
	err := pdf.Output(&buf)
	require.NoError(t, err)

	content := buf.String()
	require.Contains(t, content, "Section Mapping")
	require.Contains(t, content, "NON-OPERATING")
	require.Equal(t, 1, strings.Count(content, "Other Income Accounts"))
	require.Equal(t, 1, strings.Count(content, "Other Expense Accounts"))
	require.Contains(t, content, "NET OTHER INCOME/EXPENSES")
	require.Contains(t, content, "LABA SELISIH KURS")
	require.Contains(t, content, "BEBAN BUNGA")
}
