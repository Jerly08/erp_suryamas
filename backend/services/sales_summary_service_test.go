package services

import (
	"strconv"
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

func TestGetSalesFiltersByProject(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&models.Contact{}, &models.Project{}, &models.Sale{}, &models.SaleItem{}); err != nil {
		t.Fatalf("auto migrate sales data: %v", err)
	}

	customer := models.Contact{
		Code:     "CUS-PROJECT-FILTER",
		Name:     "Customer Project Filter",
		Type:     models.ContactTypeCustomer,
		IsActive: true,
	}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}

	projectA := models.Project{Code: "PRJ-FILTER-A", Name: "Project Filter A"}
	projectB := models.Project{Code: "PRJ-FILTER-B", Name: "Project Filter B"}
	if err := db.Create(&projectA).Error; err != nil {
		t.Fatalf("create project A: %v", err)
	}
	if err := db.Create(&projectB).Error; err != nil {
		t.Fatalf("create project B: %v", err)
	}

	saleDate := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	sales := []models.Sale{
		{
			Code:              "SOA-PROJECT-A",
			CustomerID:        customer.ID,
			UserID:            1,
			ProjectID:         &projectA.ID,
			Date:              saleDate,
			Status:            models.SaleStatusConfirmed,
			TotalAmount:       1_000_000,
			OutstandingAmount: 1_000_000,
		},
		{
			Code:              "SOA-PROJECT-B",
			CustomerID:        customer.ID,
			UserID:            1,
			ProjectID:         &projectB.ID,
			Date:              saleDate,
			Status:            models.SaleStatusConfirmed,
			TotalAmount:       2_000_000,
			OutstandingAmount: 2_000_000,
		},
	}
	if err := db.Create(&sales).Error; err != nil {
		t.Fatalf("seed sales: %v", err)
	}

	service := &SalesServiceV2{db: db}
	filter := models.SalesFilter{
		ProjectID: strconv.FormatUint(uint64(projectA.ID), 10),
		Page:      1,
		Limit:     10,
	}

	result, err := service.GetSales(filter)
	if err != nil {
		t.Fatalf("get sales by project: %v", err)
	}
	if result.Total != 1 || len(result.Data) != 1 || result.Data[0].Code != "SOA-PROJECT-A" {
		t.Fatalf("expected only project A sale, got total=%d data=%+v", result.Total, result.Data)
	}

	summary, err := service.GetSalesSummary(filter)
	if err != nil {
		t.Fatalf("get sales summary by project: %v", err)
	}
	if summary.TotalSales != 1 {
		t.Fatalf("expected 1 project A sale in summary, got %d", summary.TotalSales)
	}
	if summary.TotalAmount != 1_000_000 {
		t.Fatalf("expected project A summary total 1000000, got %.2f", summary.TotalAmount)
	}
}
