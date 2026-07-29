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
		&models.PurchaseCOARevision{},
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

func TestPurchaseApprovalWorkflowWithoutDirectorRoutesToAccounting(t *testing.T) {
	fixture := newPurchaseWorkflowFixture(t)
	purchase, item := fixture.createDraftPurchase(t, "PO-WF-NO-DIRECTOR", 125000)

	require.NoError(t, fixture.service.SubmitForApproval(purchase.ID, fixture.staff.ID))

	result, err := fixture.service.ProcessPurchaseApprovalWithEscalation(
		purchase.ID,
		true,
		fixture.manager.ID,
		fixture.manager.Role,
		"Finance manager approved without director escalation",
		false,
	)
	require.NoError(t, err)
	require.Equal(t, false, result["escalated"])

	approved, err := fixture.service.GetPurchaseByID(purchase.ID)
	require.NoError(t, err)
	require.Equal(t, models.PurchaseStatusPendingAccounting, approved.Status)
	require.Equal(t, models.PurchaseApprovalApproved, approved.ApprovalStatus)

	directorAction := fixture.approvalActionByRole(t, *approved.ApprovalRequestID, models.RoleDirector)
	require.Equal(t, models.ApprovalStatusPending, directorAction.Status)
	require.False(t, directorAction.IsActive)
	require.Nil(t, directorAction.ApproverID)

	var journalCount int64
	require.NoError(t, fixture.db.Model(&models.SSOTJournalEntry{}).
		Where("source_type = ? AND source_id = ?", "PURCHASE", purchase.ID).
		Count(&journalCount).Error)
	require.Equal(t, int64(0), journalCount)

	completed, err := fixture.service.CompleteAccountingCOA(purchase.ID, models.PurchaseAccountingCOARequest{
		CreditAccountID: &fixture.liabilityAccount.ID,
		Items: []models.PurchaseAccountingCOAItemRequest{
			{PurchaseItemID: item.ID, ExpenseAccountID: fixture.expenseAccount.ID},
		},
	}, fixture.accounting.ID)
	require.NoError(t, err)
	require.Equal(t, models.PurchaseStatusPendingFinalApproval, completed.Status)
}

func TestPurchaseCOARevisionReturnsPurchaseToAccountingBeforePosting(t *testing.T) {
	fixture := newPurchaseWorkflowFixture(t)
	purchase, item := fixture.createDraftPurchase(t, "PO-WF-COA-REVISION", 125000)

	require.NoError(t, fixture.service.SubmitForApproval(purchase.ID, fixture.staff.ID))
	_, err := fixture.service.ProcessPurchaseApprovalWithEscalation(
		purchase.ID,
		true,
		fixture.manager.ID,
		fixture.manager.Role,
		"Finance Manager approved initial purchase workflow",
		false,
	)
	require.NoError(t, err)

	completed, err := fixture.service.CompleteAccountingCOA(purchase.ID, models.PurchaseAccountingCOARequest{
		CreditAccountID: &fixture.liabilityAccount.ID,
		Items: []models.PurchaseAccountingCOAItemRequest{
			{PurchaseItemID: item.ID, ExpenseAccountID: fixture.expenseAccount.ID},
		},
	}, fixture.accounting.ID)
	require.NoError(t, err)
	require.Equal(t, models.PurchaseStatusPendingFinalApproval, completed.Status)

	revision, err := fixture.service.RequestPurchaseCOARevision(
		purchase.ID,
		models.PurchaseCOARevisionCreateRequest{
			Reason: "Akun beban item perlu disesuaikan sebelum jurnal diposting.",
		},
		fixture.accounting.ID,
		fixture.accounting.Role,
	)
	require.NoError(t, err)
	require.Equal(t, models.PurchaseCOARevisionStatusPendingFinanceManager, revision.Status)
	require.NotEmpty(t, revision.COASnapshot)

	// A pending request blocks final posting and leaves the already-entered COA intact.
	_, err = fixture.service.FinalApprovePurchaseAfterAccounting(purchase.ID, fixture.manager.ID, "Attempt while revision is pending")
	require.Error(t, err)
	require.Contains(t, err.Error(), "masih ada pengajuan revisi COA")

	var itemBeforeDecision models.PurchaseItem
	require.NoError(t, fixture.db.First(&itemBeforeDecision, item.ID).Error)
	require.Equal(t, fixture.expenseAccount.ID, itemBeforeDecision.ExpenseAccountID)

	approvedRevision, err := fixture.service.DecidePurchaseCOARevision(
		purchase.ID,
		revision.ID,
		models.PurchaseCOARevisionDecisionRequest{
			Approve: true,
			Notes:   "Silakan koreksi COA dan ajukan final approval kembali.",
		},
		fixture.manager.ID,
		fixture.manager.Role,
	)
	require.NoError(t, err)
	require.Equal(t, models.PurchaseCOARevisionStatusApproved, approvedRevision.Status)

	returned, err := fixture.service.GetPurchaseByID(purchase.ID)
	require.NoError(t, err)
	require.Equal(t, models.PurchaseStatusPendingAccounting, returned.Status)
	require.Equal(t, models.PurchaseApprovalApproved, returned.ApprovalStatus)

	var journalCount int64
	require.NoError(t, fixture.db.Model(&models.SSOTJournalEntry{}).
		Where("source_type = ? AND source_id = ?", models.SSOTSourceTypePurchase, uint64(purchase.ID)).
		Count(&journalCount).Error)
	require.Equal(t, int64(0), journalCount)
}

func TestPurchaseApprovalWorkflowWithExplicitDirectorStillWaitsForAccounting(t *testing.T) {
	fixture := newPurchaseWorkflowFixture(t)
	purchase, item := fixture.createDraftPurchase(t, "PO-WF-DIRECTOR", 250000)

	require.NoError(t, fixture.service.SubmitForApproval(purchase.ID, fixture.staff.ID))
	result, err := fixture.service.ProcessPurchaseApprovalWithEscalation(
		purchase.ID,
		true,
		fixture.manager.ID,
		fixture.manager.Role,
		"Finance manager explicitly escalated",
		true,
	)
	require.NoError(t, err)
	require.Equal(t, true, result["escalated"])

	escalated, err := fixture.service.GetPurchaseByID(purchase.ID)
	require.NoError(t, err)
	require.Equal(t, models.PurchaseStatusPending, escalated.Status)
	require.Equal(t, models.PurchaseApprovalPending, escalated.ApprovalStatus)
	directorAction := fixture.approvalActionByRole(t, *escalated.ApprovalRequestID, models.RoleDirector)
	require.Equal(t, models.ApprovalStatusPending, directorAction.Status)
	require.True(t, directorAction.IsActive)

	result, err = fixture.service.ProcessPurchaseApprovalWithEscalation(
		purchase.ID,
		true,
		fixture.director.ID,
		fixture.director.Role,
		"Director approved escalation",
		false,
	)
	require.NoError(t, err)
	require.Equal(t, false, result["escalated"])

	afterDirector, err := fixture.service.GetPurchaseByID(purchase.ID)
	require.NoError(t, err)
	require.Equal(t, models.PurchaseStatusPendingAccounting, afterDirector.Status)
	require.Equal(t, models.PurchaseApprovalApproved, afterDirector.ApprovalStatus)

	completed, err := fixture.service.CompleteAccountingCOA(purchase.ID, models.PurchaseAccountingCOARequest{
		CreditAccountID: &fixture.liabilityAccount.ID,
		Items: []models.PurchaseAccountingCOAItemRequest{
			{PurchaseItemID: item.ID, ExpenseAccountID: fixture.expenseAccount.ID},
		},
	}, fixture.accounting.ID)
	require.NoError(t, err)
	require.Equal(t, models.PurchaseStatusPendingFinalApproval, completed.Status)

	finalized, err := fixture.service.FinalApprovePurchaseAfterAccounting(purchase.ID, fixture.manager.ID, "Final approval after Accounting")
	require.NoError(t, err)
	require.Equal(t, models.PurchaseStatusApproved, finalized.Status)

	var journal models.SSOTJournalEntry
	sourceID := uint64(purchase.ID)
	require.NoError(t, fixture.db.Preload("Lines").
		Where("source_type = ? AND source_id = ?", "PURCHASE", sourceID).
		First(&journal).Error)
	require.Equal(t, "POSTED", journal.Status)
	require.InDelta(t, 250000, journal.TotalDebit.InexactFloat64(), 0.01)
	require.InDelta(t, 250000, journal.TotalCredit.InexactFloat64(), 0.01)
	require.Len(t, journal.Lines, 2)
}

func TestFinanceManagerCanCorrectCOABeforeFinalApproval(t *testing.T) {
	fixture := newPurchaseWorkflowFixture(t)
	purchase, item := fixture.createDraftPurchase(t, "PO-WF-FM-COA-CORRECTION", 175000)

	require.NoError(t, fixture.service.SubmitForApproval(purchase.ID, fixture.staff.ID))
	_, err := fixture.service.ProcessPurchaseApprovalWithEscalation(
		purchase.ID,
		true,
		fixture.manager.ID,
		fixture.manager.Role,
		"Finance manager approved for accounting",
		false,
	)
	require.NoError(t, err)

	completed, err := fixture.service.CompleteAccountingCOA(purchase.ID, models.PurchaseAccountingCOARequest{
		CreditAccountID: &fixture.liabilityAccount.ID,
		Items: []models.PurchaseAccountingCOAItemRequest{
			{PurchaseItemID: item.ID, ExpenseAccountID: fixture.expenseAccount.ID},
		},
	}, fixture.accounting.ID)
	require.NoError(t, err)
	require.Equal(t, models.PurchaseStatusPendingFinalApproval, completed.Status)

	correctedExpenseAccount := models.Account{
		Code:     "6101-992",
		Name:     "Corrected Purchase Expense",
		Type:     models.AccountTypeExpense,
		IsActive: true,
	}
	require.NoError(t, fixture.db.Create(&correctedExpenseAccount).Error)
	correctedLiabilityAccount := models.Account{
		Code:     "2101-992",
		Name:     "Corrected Purchase Payable",
		Type:     models.AccountTypeLiability,
		IsActive: true,
	}
	require.NoError(t, fixture.db.Create(&correctedLiabilityAccount).Error)

	reviewed, err := fixture.service.CompleteAccountingCOA(purchase.ID, models.PurchaseAccountingCOARequest{
		CreditAccountID: &correctedLiabilityAccount.ID,
		Items: []models.PurchaseAccountingCOAItemRequest{
			{PurchaseItemID: item.ID, ExpenseAccountID: correctedExpenseAccount.ID},
		},
	}, fixture.manager.ID)
	require.NoError(t, err)
	require.Equal(t, models.PurchaseStatusPendingFinalApproval, reviewed.Status)
	require.NotNil(t, reviewed.CreditAccountID)
	require.Equal(t, correctedLiabilityAccount.ID, *reviewed.CreditAccountID)

	var savedItem models.PurchaseItem
	require.NoError(t, fixture.db.First(&savedItem, item.ID).Error)
	require.Equal(t, correctedExpenseAccount.ID, savedItem.ExpenseAccountID)

	finalized, err := fixture.service.FinalApprovePurchaseAfterAccounting(purchase.ID, fixture.manager.ID, "Final approval after COA correction")
	require.NoError(t, err)
	require.Equal(t, models.PurchaseStatusApproved, finalized.Status)

	var journal models.SSOTJournalEntry
	sourceID := uint64(purchase.ID)
	require.NoError(t, fixture.db.Preload("Lines").
		Where("source_type = ? AND source_id = ?", "PURCHASE", sourceID).
		First(&journal).Error)
	require.Len(t, journal.Lines, 2)

	var debitLine, creditLine *models.SSOTJournalLine
	for i := range journal.Lines {
		line := &journal.Lines[i]
		if line.DebitAmount.InexactFloat64() > 0 {
			debitLine = line
		}
		if line.CreditAmount.InexactFloat64() > 0 {
			creditLine = line
		}
	}
	require.NotNil(t, debitLine)
	require.NotNil(t, creditLine)
	require.Equal(t, uint64(correctedExpenseAccount.ID), debitLine.AccountID)
	require.Equal(t, uint64(correctedLiabilityAccount.ID), creditLine.AccountID)
}

func TestFinanceManagerCanCorrectTaxPaymentBeforeFinalApproval(t *testing.T) {
	fixture := newPurchaseWorkflowFixture(t)
	purchase, item := fixture.createDraftPurchase(t, "PO-WF-FM-TAX-CORRECTION", 100000)

	require.NoError(t, fixture.service.SubmitForApproval(purchase.ID, fixture.staff.ID))
	_, err := fixture.service.ProcessPurchaseApprovalWithEscalation(
		purchase.ID,
		true,
		fixture.manager.ID,
		fixture.manager.Role,
		"Finance manager approved for accounting",
		false,
	)
	require.NoError(t, err)

	_, err = fixture.service.CompleteAccountingCOA(purchase.ID, models.PurchaseAccountingCOARequest{
		CreditAccountID: &fixture.liabilityAccount.ID,
		Items: []models.PurchaseAccountingCOAItemRequest{
			{PurchaseItemID: item.ID, ExpenseAccountID: fixture.expenseAccount.ID},
		},
	}, fixture.accounting.ID)
	require.NoError(t, err)

	pph23Payable := models.Account{
		Code:     "2102-004",
		Name:     "Hutang PPh Pasal 23",
		Type:     models.AccountTypeLiability,
		IsActive: true,
	}
	require.NoError(t, fixture.db.Create(&pph23Payable).Error)

	method := models.PurchasePaymentCredit
	ppnRate := 0.0
	pph23Rate := 2.0
	reference := "FM-CORRECTED-PPH23"
	reviewed, err := fixture.service.UpdatePurchaseFinalReviewConfig(purchase.ID, models.PurchaseFinalReviewConfigRequest{
		PaymentMethod:    &method,
		CreditAccountID:  &fixture.liabilityAccount.ID,
		PaymentReference: &reference,
		PPNRate:          &ppnRate,
		PPh23Rate:        &pph23Rate,
	}, fixture.manager.ID)
	require.NoError(t, err)
	require.Equal(t, models.PurchaseStatusPendingFinalApproval, reviewed.Status)
	require.InDelta(t, 2000, reviewed.PPh23Amount, 0.01)
	require.InDelta(t, 98000, reviewed.TotalAmount, 0.01)
	require.Equal(t, models.PurchasePPhPaymentStatusUnpaid, reviewed.PPhPaymentStatus)

	finalized, err := fixture.service.FinalApprovePurchaseAfterAccounting(purchase.ID, fixture.manager.ID, "Final approval after tax/payment correction")
	require.NoError(t, err)
	require.Equal(t, models.PurchaseStatusApproved, finalized.Status)
	require.InDelta(t, 98000, finalized.OutstandingAmount, 0.01)

	var journal models.SSOTJournalEntry
	sourceID := uint64(purchase.ID)
	require.NoError(t, fixture.db.Preload("Lines").
		Where("source_type = ? AND source_id = ?", "PURCHASE", sourceID).
		First(&journal).Error)
	require.Equal(t, "POSTED", journal.Status)
	require.InDelta(t, 100000, journal.TotalDebit.InexactFloat64(), 0.01)
	require.InDelta(t, 100000, journal.TotalCredit.InexactFloat64(), 0.01)
	require.Len(t, journal.Lines, 3)

	var payableLine, pphLine *models.SSOTJournalLine
	for i := range journal.Lines {
		line := &journal.Lines[i]
		if line.AccountID == uint64(fixture.liabilityAccount.ID) {
			payableLine = line
		}
		if line.AccountID == uint64(pph23Payable.ID) {
			pphLine = line
		}
	}
	require.NotNil(t, payableLine)
	require.NotNil(t, pphLine)
	require.InDelta(t, 98000, payableLine.CreditAmount.InexactFloat64(), 0.01)
	require.InDelta(t, 2000, pphLine.CreditAmount.InexactFloat64(), 0.01)
}

func TestFinanceStaffPurchaseCreatorRoutesApprovalToFinanceManager(t *testing.T) {
	fixture := newPurchaseWorkflowFixture(t)
	purchase, _ := fixture.createDraftPurchaseForUser(t, "PO-WF-FS-CREATOR", 75000, fixture.financeStaff.ID)

	require.NoError(t, fixture.service.SubmitForApproval(purchase.ID, fixture.financeStaff.ID))

	submitted, err := fixture.service.GetPurchaseByID(purchase.ID)
	require.NoError(t, err)
	require.NotNil(t, submitted.ApprovalRequestID)

	financeAction := fixture.approvalActionByRole(t, *submitted.ApprovalRequestID, models.RoleFinance)
	require.Equal(t, models.ApprovalStatusPending, financeAction.Status)
	require.True(t, financeAction.IsActive)
	require.Nil(t, financeAction.ApproverID)

	_, err = fixture.service.ProcessPurchaseApprovalWithEscalation(
		purchase.ID,
		true,
		fixture.financeStaff.ID,
		fixture.financeStaff.Role,
		"Finance staff should not approve own purchase",
		false,
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no active approval step found for your role")
}

func TestFinanceManagerCanEscalateFromActiveFinanceManagerStep(t *testing.T) {
	fixture := newPurchaseWorkflowFixture(t)
	purchase, _ := fixture.createDraftPurchase(t, "PO-WF-FM-ESCALATION", 75000)

	require.NoError(t, fixture.service.SubmitForApproval(purchase.ID, fixture.staff.ID))
	result, err := fixture.service.ProcessPurchaseApprovalWithEscalation(
		purchase.ID,
		true,
		fixture.manager.ID,
		fixture.manager.Role,
		"Finance manager escalated to director",
		true,
	)
	require.NoError(t, err)
	require.Equal(t, true, result["escalated"])

	afterAttempt, err := fixture.service.GetPurchaseByID(purchase.ID)
	require.NoError(t, err)
	directorAction := fixture.approvalActionByRole(t, *afterAttempt.ApprovalRequestID, models.RoleDirector)
	require.Equal(t, models.ApprovalStatusPending, directorAction.Status)
	require.True(t, directorAction.IsActive)
}

type purchaseWorkflowFixture struct {
	db               *gorm.DB
	service          *PurchaseService
	staff            models.User
	financeStaff     models.User
	manager          models.User
	director         models.User
	accounting       models.User
	vendor           models.Contact
	expenseAccount   models.Account
	liabilityAccount models.Account
	purchaseDate     time.Time
}

func newPurchaseWorkflowFixture(t *testing.T) *purchaseWorkflowFixture {
	t.Helper()

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
		&models.PurchaseCOARevision{},
		&models.ApprovalWorkflow{},
		&models.ApprovalRequest{},
		&models.ApprovalStep{},
		&models.ApprovalAction{},
		&models.ApprovalHistory{},
	))

	workflow := models.ApprovalWorkflow{
		Name:            "Standard Purchase Approval",
		Module:          models.ApprovalModulePurchase,
		MinAmount:       0,
		MaxAmount:       999999999999,
		IsActive:        true,
		RequireDirector: false,
		RequireFinance:  true,
	}
	require.NoError(t, db.Create(&workflow).Error)
	steps := []models.ApprovalStep{
		{WorkflowID: workflow.ID, StepOrder: 1, StepName: "Employee Submission", ApproverRole: models.RoleEmployee, IsOptional: false},
		{WorkflowID: workflow.ID, StepOrder: 2, StepName: "Finance Manager Approval", ApproverRole: models.RoleFinance, IsOptional: false},
		{WorkflowID: workflow.ID, StepOrder: 3, StepName: "Director Approval", ApproverRole: models.RoleDirector, IsOptional: true},
	}
	require.NoError(t, db.Create(&steps).Error)

	fixture := &purchaseWorkflowFixture{
		db:           db,
		purchaseDate: time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC),
		staff: models.User{
			Username:  "workflow-staff",
			Email:     "workflow-staff@test.local",
			Password:  "secret",
			Role:      models.RoleEmployee,
			FirstName: "Staff",
			IsActive:  true,
		},
		financeStaff: models.User{
			Username:  "workflow-finance-staff",
			Email:     "workflow-finance-staff@test.local",
			Password:  "secret",
			Role:      models.RoleFinanceStaff,
			FirstName: "Finance Staff",
			IsActive:  true,
		},
		manager: models.User{
			Username:  "workflow-finance-manager",
			Email:     "workflow-finance-manager@test.local",
			Password:  "secret",
			Role:      models.RoleFinanceManager,
			FirstName: "Manager",
			IsActive:  true,
		},
		director: models.User{
			Username:  "workflow-director",
			Email:     "workflow-director@test.local",
			Password:  "secret",
			Role:      models.RoleDirector,
			FirstName: "Director",
			IsActive:  true,
		},
		accounting: models.User{
			Username:  "workflow-accounting",
			Email:     "workflow-accounting@test.local",
			Password:  "secret",
			Role:      models.RoleAccounting,
			FirstName: "Accounting",
			IsActive:  true,
		},
		vendor: models.Contact{
			Code: "VEND-WF-E2E",
			Name: "Workflow Vendor E2E",
			Type: "VENDOR",
		},
		expenseAccount: models.Account{
			Code:     "6101-991",
			Name:     "Purchase Workflow Expense",
			Type:     models.AccountTypeExpense,
			IsActive: true,
		},
		liabilityAccount: models.Account{
			Code:     "2101-991",
			Name:     "Purchase Workflow Payable",
			Type:     models.AccountTypeLiability,
			IsActive: true,
		},
	}

	require.NoError(t, db.Create(&fixture.staff).Error)
	require.NoError(t, db.Create(&fixture.financeStaff).Error)
	require.NoError(t, db.Create(&fixture.manager).Error)
	require.NoError(t, db.Create(&fixture.director).Error)
	require.NoError(t, db.Create(&fixture.accounting).Error)
	require.NoError(t, db.Create(&fixture.vendor).Error)
	require.NoError(t, db.Create(&fixture.expenseAccount).Error)
	require.NoError(t, db.Create(&fixture.liabilityAccount).Error)

	approvalService := NewApprovalService(db)
	purchaseJournalService := NewPurchaseJournalServiceSSOT(db, nil, NewSettingsService(db))
	fixture.service = NewPurchaseService(
		db,
		repositories.NewPurchaseRepository(db),
		nil,
		repositories.NewContactRepository(db),
		repositories.NewAccountRepository(db),
		approvalService,
		nil,
		nil,
		nil,
		nil,
		nil,
		purchaseJournalService,
		nil,
	)

	return fixture
}

func (f *purchaseWorkflowFixture) createDraftPurchase(t *testing.T, code string, amount float64) (*models.Purchase, models.PurchaseItem) {
	t.Helper()
	return f.createDraftPurchaseForUser(t, code, amount, f.staff.ID)
}

func (f *purchaseWorkflowFixture) createDraftPurchaseForUser(t *testing.T, code string, amount float64, userID uint) (*models.Purchase, models.PurchaseItem) {
	t.Helper()

	purchase := models.Purchase{
		Code:                   code,
		VendorID:               f.vendor.ID,
		UserID:                 userID,
		Date:                   f.purchaseDate,
		DueDate:                f.purchaseDate.AddDate(0, 0, 14),
		Status:                 models.PurchaseStatusDraft,
		ApprovalStatus:         models.PurchaseApprovalNotStarted,
		ApprovalAmountBasis:    "SUBTOTAL_BEFORE_DISCOUNT",
		SubtotalBeforeDiscount: amount,
		NetBeforeTax:           amount,
		PaymentMethod:          models.PurchasePaymentCredit,
		TotalAmount:            amount,
	}
	require.NoError(t, f.db.Create(&purchase).Error)

	item := models.PurchaseItem{
		PurchaseID:  purchase.ID,
		Description: "Workflow test item",
		Quantity:    1,
		UnitPrice:   amount,
		TotalPrice:  amount,
	}
	require.NoError(t, f.db.Create(&item).Error)

	return &purchase, item
}

func (f *purchaseWorkflowFixture) approvalActionByRole(t *testing.T, requestID uint, role string) models.ApprovalAction {
	t.Helper()

	var actions []models.ApprovalAction
	require.NoError(t, f.db.Preload("Step").
		Where("request_id = ?", requestID).
		Find(&actions).Error)

	for _, action := range actions {
		if models.NormalizeRole(action.Step.ApproverRole) == models.NormalizeRole(role) {
			return action
		}
	}

	t.Fatalf("approval action for role %s not found", role)
	return models.ApprovalAction{}
}
