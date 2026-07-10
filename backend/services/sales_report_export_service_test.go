package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSalesSummaryCSVIncludesShippingAndDPPBreakdown(t *testing.T) {
	service := NewSalesReportExportService(nil)
	report := &SalesSummaryData{
		Company: CompanyInfo{
			Name: "PT Test",
		},
		StartDate:            time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
		EndDate:              time.Date(2026, time.July, 31, 0, 0, 0, 0, time.UTC),
		TotalSales:           169275,
		TotalRevenue:         152500,
		TotalShippingRevenue: 32500,
		TotalTaxableBase:     152500,
		TotalTax:             16775,
		NetRevenue:           152500,
		TotalTransactions:    1,
	}

	csvData, err := service.ExportToCSV(report, 0)
	require.NoError(t, err)

	output := string(csvData)
	require.Contains(t, output, "Total Nilai Invoice,169275.00")
	require.Contains(t, output, "Total Pendapatan,152500.00")
	require.Contains(t, output, "Pendapatan Ongkir,32500.00")
	require.Contains(t, output, "DPP PPN,152500.00")
	require.Contains(t, output, "PPN Keluaran,16775.00")
}
