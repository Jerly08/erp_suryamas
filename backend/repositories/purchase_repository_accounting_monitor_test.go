package repositories

import (
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPurchaseRepositoryAccountingStatusFilter(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Project{},
		&models.PurchaseRequest{},
		&models.Purchase{},
		&models.PurchaseItem{},
		&models.ApprovalWorkflow{},
		&models.ApprovalRequest{},
		&models.ApprovalStep{},
		&models.ApprovalAction{},
	))

	user := models.User{
		Username:  "accounting-monitor",
		Email:     "accounting-monitor@test.local",
		Password:  "secret",
		Role:      models.RoleAccounting,
		FirstName: "Accounting",
	}
	require.NoError(t, db.Create(&user).Error)

	vendor := models.Contact{
		Code: "VEND-MON",
		Name: "Vendor Monitoring",
		Type: "VENDOR",
	}
	require.NoError(t, db.Create(&vendor).Error)

	purchases := []models.Purchase{
		{
			Code:           "PO-READY-COA",
			VendorID:       vendor.ID,
			UserID:         user.ID,
			Date:           time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
			Status:         models.PurchaseStatusPendingAccounting,
			ApprovalStatus: models.PurchaseApprovalApproved,
			TotalAmount:    100,
		},
		{
			Code:           "PO-COA-DONE",
			VendorID:       vendor.ID,
			UserID:         user.ID,
			Date:           time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC),
			Status:         models.PurchaseStatusApproved,
			ApprovalStatus: models.PurchaseApprovalApproved,
			TotalAmount:    200,
		},
		{
			Code:           "PO-WAITING-FM",
			VendorID:       vendor.ID,
			UserID:         user.ID,
			Date:           time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC),
			Status:         models.PurchaseStatusPendingApproval,
			ApprovalStatus: models.PurchaseApprovalPending,
			TotalAmount:    300,
		},
		{
			Code:           "PO-READY-OUTSIDE-PERIOD",
			VendorID:       vendor.ID,
			UserID:         user.ID,
			Date:           time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
			Status:         models.PurchaseStatusPendingAccounting,
			ApprovalStatus: models.PurchaseApprovalApproved,
			TotalAmount:    400,
		},
		{
			Code:           "PO-READY-END-DATE-NOON",
			VendorID:       vendor.ID,
			UserID:         user.ID,
			Date:           time.Date(2026, 5, 31, 15, 30, 0, 0, time.UTC),
			Status:         models.PurchaseStatusPendingAccounting,
			ApprovalStatus: models.PurchaseApprovalApproved,
			TotalAmount:    125,
		},
	}
	require.NoError(t, db.Create(&purchases).Error)

	repo := NewPurchaseRepository(db)
	baseFilter := models.PurchaseFilter{
		StartDate: "2026-05-01",
		EndDate:   "2026-05-31",
		Page:      1,
		Limit:     10,
	}

	readyForCOA, total, err := repo.FindWithFilter(models.PurchaseFilter{
		StartDate:        baseFilter.StartDate,
		EndDate:          baseFilter.EndDate,
		Page:             baseFilter.Page,
		Limit:            baseFilter.Limit,
		AccountingStatus: models.PurchaseAccountingStatusReadyForCOA,
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, readyForCOA, 2)
	require.ElementsMatch(t, []string{"PO-READY-COA", "PO-READY-END-DATE-NOON"}, []string{readyForCOA[0].Code, readyForCOA[1].Code})
	readySummary, err := repo.GetPurchasesSummary(models.PurchaseFilter{
		StartDate:        baseFilter.StartDate,
		EndDate:          baseFilter.EndDate,
		AccountingStatus: models.PurchaseAccountingStatusReadyForCOA,
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), readySummary.TotalPurchases)
	require.Equal(t, float64(225), readySummary.TotalAmount)
	require.Equal(t, float64(225), readySummary.TotalApprovedAmount)

	approvedByFinance, total, err := repo.FindWithFilter(models.PurchaseFilter{
		StartDate:        baseFilter.StartDate,
		EndDate:          baseFilter.EndDate,
		Page:             baseFilter.Page,
		Limit:            baseFilter.Limit,
		AccountingStatus: models.PurchaseAccountingStatusApprovedByFinance,
	})
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, approvedByFinance, 3)
	approvedSummary, err := repo.GetPurchasesSummary(models.PurchaseFilter{
		StartDate:        baseFilter.StartDate,
		EndDate:          baseFilter.EndDate,
		AccountingStatus: models.PurchaseAccountingStatusApprovedByFinance,
	})
	require.NoError(t, err)
	require.Equal(t, int64(3), approvedSummary.TotalPurchases)
	require.Equal(t, float64(425), approvedSummary.TotalAmount)
	require.Equal(t, float64(425), approvedSummary.TotalApprovedAmount)

	coaCompleted, total, err := repo.FindWithFilter(models.PurchaseFilter{
		StartDate:        baseFilter.StartDate,
		EndDate:          baseFilter.EndDate,
		Page:             baseFilter.Page,
		Limit:            baseFilter.Limit,
		AccountingStatus: models.PurchaseAccountingStatusCOACompleted,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, coaCompleted, 1)
	require.Equal(t, "PO-COA-DONE", coaCompleted[0].Code)
	completedSummary, err := repo.GetPurchasesSummary(models.PurchaseFilter{
		StartDate:        baseFilter.StartDate,
		EndDate:          baseFilter.EndDate,
		AccountingStatus: models.PurchaseAccountingStatusCOACompleted,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), completedSummary.TotalPurchases)
	require.Equal(t, float64(200), completedSummary.TotalAmount)
	require.Equal(t, float64(200), completedSummary.TotalApprovedAmount)
}
