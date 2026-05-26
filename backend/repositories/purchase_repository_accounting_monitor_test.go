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
			Code:           "PO-PENDING-FINAL",
			VendorID:       vendor.ID,
			UserID:         user.ID,
			Date:           time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC),
			Status:         models.PurchaseStatusPendingFinalApproval,
			ApprovalStatus: models.PurchaseApprovalApproved,
			TotalAmount:    150,
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
	require.Equal(t, int64(4), total)
	require.Len(t, approvedByFinance, 4)
	approvedSummary, err := repo.GetPurchasesSummary(models.PurchaseFilter{
		StartDate:        baseFilter.StartDate,
		EndDate:          baseFilter.EndDate,
		AccountingStatus: models.PurchaseAccountingStatusApprovedByFinance,
	})
	require.NoError(t, err)
	require.Equal(t, int64(4), approvedSummary.TotalPurchases)
	require.Equal(t, float64(575), approvedSummary.TotalAmount)
	require.Equal(t, float64(575), approvedSummary.TotalApprovedAmount)

	pendingFinal, total, err := repo.FindWithFilter(models.PurchaseFilter{
		StartDate:        baseFilter.StartDate,
		EndDate:          baseFilter.EndDate,
		Page:             baseFilter.Page,
		Limit:            baseFilter.Limit,
		AccountingStatus: models.PurchaseAccountingStatusPendingFinalApproval,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, pendingFinal, 1)
	require.Equal(t, "PO-PENDING-FINAL", pendingFinal[0].Code)

	coaCompleted, total, err := repo.FindWithFilter(models.PurchaseFilter{
		StartDate:        baseFilter.StartDate,
		EndDate:          baseFilter.EndDate,
		Page:             baseFilter.Page,
		Limit:            baseFilter.Limit,
		AccountingStatus: models.PurchaseAccountingStatusCOACompleted,
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, coaCompleted, 2)
	require.ElementsMatch(t, []string{"PO-COA-DONE", "PO-PENDING-FINAL"}, []string{coaCompleted[0].Code, coaCompleted[1].Code})
	completedSummary, err := repo.GetPurchasesSummary(models.PurchaseFilter{
		StartDate:        baseFilter.StartDate,
		EndDate:          baseFilter.EndDate,
		AccountingStatus: models.PurchaseAccountingStatusCOACompleted,
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), completedSummary.TotalPurchases)
	require.Equal(t, float64(350), completedSummary.TotalAmount)
	require.Equal(t, float64(350), completedSummary.TotalApprovedAmount)
}

func TestPurchaseRepositoryOutstandingFilter(t *testing.T) {
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
		Username:  "outstanding-filter",
		Email:     "outstanding-filter@test.local",
		Password:  "secret",
		Role:      models.RoleAccounting,
		FirstName: "Accounting",
	}
	require.NoError(t, db.Create(&user).Error)

	vendor := models.Contact{
		Code: "VEND-OUT",
		Name: "Vendor Outstanding",
		Type: "VENDOR",
	}
	require.NoError(t, db.Create(&vendor).Error)

	purchases := []models.Purchase{
		{
			Code:              "PO-PAID",
			VendorID:          vendor.ID,
			UserID:            user.ID,
			Date:              time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC),
			Status:            models.PurchaseStatusPaid,
			ApprovalStatus:    models.PurchaseApprovalApproved,
			TotalAmount:       100,
			PaidAmount:        100,
			OutstandingAmount: 0,
		},
		{
			Code:              "PO-PARTIAL",
			VendorID:          vendor.ID,
			UserID:            user.ID,
			Date:              time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC),
			Status:            models.PurchaseStatusApproved,
			ApprovalStatus:    models.PurchaseApprovalApproved,
			TotalAmount:       250,
			PaidAmount:        75,
			OutstandingAmount: 175,
		},
		{
			Code:              "PO-UNPAID",
			VendorID:          vendor.ID,
			UserID:            user.ID,
			Date:              time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC),
			Status:            models.PurchaseStatusApproved,
			ApprovalStatus:    models.PurchaseApprovalApproved,
			TotalAmount:       300,
			OutstandingAmount: 300,
		},
	}
	require.NoError(t, db.Create(&purchases).Error)

	outstandingOnly := true
	filter := models.PurchaseFilter{
		OutstandingOnly: &outstandingOnly,
		Page:            1,
		Limit:           10,
	}

	repo := NewPurchaseRepository(db)
	outstanding, total, err := repo.FindWithFilter(filter)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, outstanding, 2)
	require.ElementsMatch(t, []string{"PO-PARTIAL", "PO-UNPAID"}, []string{outstanding[0].Code, outstanding[1].Code})

	summary, err := repo.GetPurchasesSummary(filter)
	require.NoError(t, err)
	require.Equal(t, int64(2), summary.TotalPurchases)
	require.Equal(t, float64(550), summary.TotalAmount)
	require.Equal(t, float64(475), summary.TotalOutstanding)
}
