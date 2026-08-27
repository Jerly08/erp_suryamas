package services

import (
	"context"
	"math"
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSSOTPurchaseReportTaxTestDB(t *testing.T, withTaxAllocations bool) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	if err := db.Exec(`
		CREATE TABLE purchases (
			id INTEGER PRIMARY KEY,
			date DATETIME NOT NULL,
			status TEXT NOT NULL,
			approval_status TEXT NOT NULL,
			deleted_at DATETIME,
			net_before_tax DECIMAL(15,2) NOT NULL DEFAULT 0,
			tax_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
			vat_treatment TEXT,
			project_id INTEGER
		)
	`).Error; err != nil {
		t.Fatalf("create purchases table: %v", err)
	}

	if withTaxAllocations {
		if err := db.Exec(`
			CREATE TABLE purchase_tax_allocations (
				id INTEGER PRIMARY KEY,
				purchase_id INTEGER NOT NULL,
				dpp_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
				ppn_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
				is_creditable BOOLEAN NOT NULL DEFAULT TRUE,
				deleted_at DATETIME
			)
		`).Error; err != nil {
			t.Fatalf("create purchase tax allocations table: %v", err)
		}
	}

	return db
}

func assertPurchaseTaxAnalysisFloat(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.0001 {
		t.Fatalf("%s: expected %.4f, got %.4f", name, want, got)
	}
}

func TestPurchaseTaxAnalysisUsesComponentVATAllocations(t *testing.T) {
	db := newSSOTPurchaseReportTaxTestDB(t, true)
	purchaseDate := time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC)

	if err := db.Exec(
		`INSERT INTO purchases (id, date, status, approval_status, net_before_tax, tax_amount, vat_treatment) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		1,
		purchaseDate,
		models.PurchaseStatusApproved,
		models.PurchaseApprovalNotStarted,
		100000,
		11000,
		models.PurchaseVATTreatmentStandard,
	).Error; err != nil {
		t.Fatalf("create legacy purchase: %v", err)
	}

	if err := db.Exec(
		`INSERT INTO purchases (id, date, status, approval_status, net_before_tax, tax_amount, vat_treatment) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		2,
		purchaseDate,
		models.PurchaseStatusApproved,
		models.PurchaseApprovalNotStarted,
		657376,
		4643,
		models.PurchaseVATTreatmentComponentDPPNilaiLain,
	).Error; err != nil {
		t.Fatalf("create component VAT purchase: %v", err)
	}

	allocations := []struct {
		id           int
		dpp          float64
		ppn          float64
		isCreditable bool
	}{
		{id: 1, dpp: 28695, ppn: 3443, isCreditable: true},
		{id: 2, dpp: 10000, ppn: 1200, isCreditable: false},
	}
	for _, allocation := range allocations {
		if err := db.Exec(
			`INSERT INTO purchase_tax_allocations (id, purchase_id, dpp_amount, ppn_amount, is_creditable) VALUES (?, ?, ?, ?, ?)`,
			allocation.id,
			2,
			allocation.dpp,
			allocation.ppn,
			allocation.isCreditable,
		).Error; err != nil {
			t.Fatalf("create tax allocation %d: %v", allocation.id, err)
		}
	}

	analysis, err := NewSSOTPurchaseReportService(db).getTaxAnalysis(
		context.Background(),
		time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.August, 31, 23, 59, 59, 0, time.UTC),
		nil,
	)
	if err != nil {
		t.Fatalf("analyze purchase tax: %v", err)
	}

	// The component purchase must use invoice DPP (38,695), not its complete
	// purchase total (657,376), and only its creditable PPN (3,443).
	assertPurchaseTaxAnalysisFloat(t, "total taxable amount", analysis.TotalTaxableAmount, 138695)
	assertPurchaseTaxAnalysisFloat(t, "total tax amount", analysis.TotalTaxAmount, 15643)
	assertPurchaseTaxAnalysisFloat(t, "reclaimable tax amount", analysis.TaxReclaimableAmount, 14443)
	assertPurchaseTaxAnalysisFloat(t, "average tax rate", analysis.AverageTaxRate, (15643.0/138695.0)*100)

	if len(analysis.TaxByMonth) != 1 {
		t.Fatalf("expected one monthly tax summary, got %#v", analysis.TaxByMonth)
	}
	if analysis.TaxByMonth[0].Year != 2026 || analysis.TaxByMonth[0].Month != int(time.August) {
		t.Fatalf("unexpected monthly tax period: %#v", analysis.TaxByMonth[0])
	}
	assertPurchaseTaxAnalysisFloat(t, "monthly tax amount", analysis.TaxByMonth[0].TaxAmount, 15643)
}

func TestPurchaseTaxAnalysisFallsBackWhenTaxAllocationTableIsUnavailable(t *testing.T) {
	db := newSSOTPurchaseReportTaxTestDB(t, false)
	purchaseDate := time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC)

	if err := db.Exec(
		`INSERT INTO purchases (id, date, status, approval_status, net_before_tax, tax_amount, vat_treatment) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		1,
		purchaseDate,
		models.PurchaseStatusApproved,
		models.PurchaseApprovalNotStarted,
		657376,
		3443,
		models.PurchaseVATTreatmentComponentDPPNilaiLain,
	).Error; err != nil {
		t.Fatalf("create purchase: %v", err)
	}

	analysis, err := NewSSOTPurchaseReportService(db).getTaxAnalysis(
		context.Background(),
		time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.August, 31, 23, 59, 59, 0, time.UTC),
		nil,
	)
	if err != nil {
		t.Fatalf("analyze purchase tax without allocation table: %v", err)
	}

	// A partially migrated database must retain the historical header-based
	// report instead of failing because purchase_tax_allocations is unavailable.
	assertPurchaseTaxAnalysisFloat(t, "legacy fallback taxable amount", analysis.TotalTaxableAmount, 657376)
	assertPurchaseTaxAnalysisFloat(t, "legacy fallback tax amount", analysis.TotalTaxAmount, 3443)
	assertPurchaseTaxAnalysisFloat(t, "legacy fallback reclaimable amount", analysis.TaxReclaimableAmount, 3443)
}
