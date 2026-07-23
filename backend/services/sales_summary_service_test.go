package services

import (
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestGetSalesSummaryDefaultsToActiveSales(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&models.Sale{}); err != nil {
		t.Fatalf("auto migrate sales: %v", err)
	}

	saleDate := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	sales := []models.Sale{
		{
			Code:              "SOA-DRAFT",
			CustomerID:        1,
			UserID:            1,
			Date:              saleDate,
			Status:            models.SaleStatusDraft,
			TotalAmount:       1_000_000,
			OutstandingAmount: 1_000_000,
		},
		{
			Code:              "SOA-CONFIRMED",
			CustomerID:        1,
			UserID:            1,
			Date:              saleDate,
			Status:            models.SaleStatusConfirmed,
			TotalAmount:       2_000_000,
			OutstandingAmount: 1_500_000,
			PaidAmount:        500_000,
		},
		{
			Code:        "SOA-PAID",
			CustomerID:  1,
			UserID:      1,
			Date:        saleDate,
			Status:      models.SaleStatusPaid,
			TotalAmount: 500_000,
			PaidAmount:  500_000,
		},
		{
			Code:              "SOA-CANCELLED",
			CustomerID:        1,
			UserID:            1,
			Date:              saleDate,
			Status:            models.SaleStatusCancelled,
			TotalAmount:       9_000_000,
			OutstandingAmount: 9_000_000,
		},
	}

	if err := db.Create(&sales).Error; err != nil {
		t.Fatalf("seed sales: %v", err)
	}

	service := &SalesServiceV2{db: db}
	summary, err := service.GetSalesSummary(models.SalesFilter{})
	if err != nil {
		t.Fatalf("get sales summary: %v", err)
	}

	if summary.TotalSales != 3 {
		t.Fatalf("expected 3 active sales, got %d", summary.TotalSales)
	}
	if summary.TotalAmount != 3_500_000 {
		t.Fatalf("expected total amount 3500000, got %.2f", summary.TotalAmount)
	}
	if summary.TotalPaid != 1_000_000 {
		t.Fatalf("expected total paid 1000000, got %.2f", summary.TotalPaid)
	}
	if summary.TotalOutstanding != 2_500_000 {
		t.Fatalf("expected total outstanding 2500000, got %.2f", summary.TotalOutstanding)
	}
}
