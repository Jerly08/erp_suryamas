package services

import (
	"bytes"
	"testing"

	"app-sistem-akuntansi/models"
	"github.com/jung-kurt/gofpdf"
	"github.com/stretchr/testify/require"
)

func TestDrawUnifiedLetterheadIncludesConfiguredCompanyProfile(t *testing.T) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(false)
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	settings := &models.Settings{
		CompanyName:    "Avery Jaya Permai",
		CompanyAddress: "Office88, Jl. Raya Casablanca, Jakarta Selatan, DKI Jakarta, Indonesia",
		CompanyPhone:   "+62-21-5551234",
		CompanyEmail:   "admin@averyjayapermai.id",
		TaxNumber:      "01.234.567.8-901.000",
	}

	contentY := drawUnifiedLetterhead(pdf, settings, 35)
	require.Greater(t, contentY, 55.0)

	var buf bytes.Buffer
	require.NoError(t, pdf.Output(&buf))

	output := buf.String()
	require.Contains(t, output, "Avery Jaya Permai")
	require.Contains(t, output, "Office88, Jl. Raya Casablanca")
	require.Contains(t, output, "Telepon: +62-21-5551234")
	require.Contains(t, output, "Email: admin@averyjayapermai.id")
	require.Contains(t, output, "NPWP: 01.234.567.8-901.000")
}
