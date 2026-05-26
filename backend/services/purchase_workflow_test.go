package services

import (
	"strings"
	"testing"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPurchaseApprovalRoutesToAccountingThenFinalApprovalPosts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Project{},
		&models.Account{},
		&models.AccountingPeriod{},
		&models.Settings{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
		&models.Product{},
		&models.PurchaseRequest{},
		&models.PurchaseRequestItem{},
		&models.Purchase{},
		&models.PurchaseItem{},
		&models.ApprovalWorkflow{},
		&models.ApprovalRequest{},
		&models.ApprovalStep{},
		&models.ApprovalAction{},
		&models.ApprovalHistory{},
	))

	staff := models.User{
		Username:  "purchase-staff",
		Email:     "purchase-staff@test.local",
		Password:  "secret",
		Role:      models.RoleEmployee,
		FirstName: "Staff",
	}
	manager := models.User{
		Username:  "finance-manager",
		Email:     "finance-manager@test.local",
		Password:  "secret",
		Role:      models.RoleFinanceManager,
		FirstName: "Manager",
	}
	accounting := models.User{
		Username:  "accounting-user",
		Email:     "accounting-user@test.local",
		Password:  "secret",
		Role:      models.RoleAccounting,
		FirstName: "Accounting",
	}
	require.NoError(t, db.Create(&staff).Error)
	require.NoError(t, db.Create(&manager).Error)
	require.NoError(t, db.Create(&accounting).Error)

	vendor := models.Contact{
		Code: "VEND-WF",
		Name: "Workflow Vendor",
		Type: "VENDOR",
	}
	require.NoError(t, db.Create(&vendor).Error)

	expenseAccount := models.Account{
		Code:     "6101-990",
		Name:     "Purchase Test Expense",
		Type:     models.AccountTypeExpense,
		IsActive: true,
	}
	liabilityAccount := models.Account{
		Code:     "2101-990",
		Name:     "Purchase Test Payable",
		Type:     models.AccountTypeLiability,
		IsActive: true,
	}
	require.NoError(t, db.Create(&expenseAccount).Error)
	require.NoError(t, db.Create(&liabilityAccount).Error)

	purchaseJournalService := NewPurchaseJournalServiceSSOT(db, nil, NewSettingsService(db))
	service := NewPurchaseService(
		db,
		repositories.NewPurchaseRepository(db),
		nil,
		nil,
		repositories.NewAccountRepository(db),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		purchaseJournalService,
		nil,
	)

	purchaseDate := time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC)
	routedPurchase := models.Purchase{
		Code:           "PO-WF-ROUTE",
		VendorID:       vendor.ID,
		UserID:         staff.ID,
		Date:           purchaseDate,
		DueDate:        purchaseDate.AddDate(0, 0, 14),
		Status:         models.PurchaseStatusApproved,
		ApprovalStatus: models.PurchaseApprovalApproved,
		PaymentMethod:  models.PurchasePaymentCredit,
		TotalAmount:    500,
		NetBeforeTax:   500,
	}
	require.NoError(t, db.Create(&routedPurchase).Error)
	require.NoError(t, db.Create(&models.PurchaseItem{
		PurchaseID:  routedPurchase.ID,
		Description: "Workflow route item",
		Quantity:    1,
		UnitPrice:   500,
		TotalPrice:  500,
	}).Error)

	require.NoError(t, service.OnPurchaseApproved(routedPurchase.ID))

	var routedAfterApproval models.Purchase
	require.NoError(t, db.First(&routedAfterApproval, routedPurchase.ID).Error)
	require.Equal(t, models.PurchaseStatusPendingAccounting, routedAfterApproval.Status)

	purchase := models.Purchase{
		Code:           "PO-WF-FINAL",
		VendorID:       vendor.ID,
		UserID:         staff.ID,
		Date:           purchaseDate,
		DueDate:        purchaseDate.AddDate(0, 0, 14),
		Status:         models.PurchaseStatusPendingAccounting,
		ApprovalStatus: models.PurchaseApprovalApproved,
		PaymentMethod:  models.PurchasePaymentCredit,
		TotalAmount:    1000,
		NetBeforeTax:   1000,
	}
	require.NoError(t, db.Create(&purchase).Error)
	item := models.PurchaseItem{
		PurchaseID:  purchase.ID,
		Description: "Workflow final item",
		Quantity:    1,
		UnitPrice:   1000,
		TotalPrice:  1000,
	}
	require.NoError(t, db.Create(&item).Error)

	creditAccountID := liabilityAccount.ID
	completed, err := service.CompleteAccountingCOA(purchase.ID, models.PurchaseAccountingCOARequest{
		CreditAccountID: &creditAccountID,
		Items: []models.PurchaseAccountingCOAItemRequest{
			{
				PurchaseItemID:   item.ID,
				ExpenseAccountID: expenseAccount.ID,
			},
		},
	}, accounting.ID)
	require.NoError(t, err)
	require.Equal(t, models.PurchaseStatusPendingFinalApproval, completed.Status)
	require.Equal(t, models.PurchaseApprovalApproved, completed.ApprovalStatus)
	require.NotNil(t, completed.CreditAccountID)
	require.Equal(t, liabilityAccount.ID, *completed.CreditAccountID)

	var savedItem models.PurchaseItem
	require.NoError(t, db.First(&savedItem, item.ID).Error)
	require.Equal(t, expenseAccount.ID, savedItem.ExpenseAccountID)

	finalized, err := service.FinalApprovePurchaseAfterAccounting(purchase.ID, manager.ID, "Final approval after Accounting completed GL/COA")
	require.NoError(t, err)
	require.Equal(t, models.PurchaseStatusApproved, finalized.Status)
	require.Equal(t, models.PurchaseApprovalApproved, finalized.ApprovalStatus)
	require.NotNil(t, finalized.ApprovedBy)
	require.Equal(t, manager.ID, *finalized.ApprovedBy)
	require.Equal(t, float64(0), finalized.PaidAmount)
	require.Equal(t, float64(1000), finalized.OutstandingAmount)

	var journal models.SSOTJournalEntry
	sourceID := uint64(purchase.ID)
	require.NoError(t, db.Preload("Lines").
		Where("source_type = ? AND source_id = ?", "PURCHASE", sourceID).
		First(&journal).Error)
	require.Equal(t, "POSTED", journal.Status)
	require.InDelta(t, 1000, journal.TotalDebit.InexactFloat64(), 0.01)
	require.InDelta(t, 1000, journal.TotalCredit.InexactFloat64(), 0.01)
	require.Len(t, journal.Lines, 2)
}
