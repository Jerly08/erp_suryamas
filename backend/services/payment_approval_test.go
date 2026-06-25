package services

import (
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRejectPaymentStoresApprovalTrail(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Account{},
		&models.CashBank{},
		&models.Payment{},
	))

	requester := models.User{
		Username:  "finance-staff",
		Email:     "finance-staff@test.local",
		Password:  "secret",
		Role:      models.RoleFinanceStaff,
		FirstName: "Finance",
		LastName:  "Staff",
	}
	require.NoError(t, db.Create(&requester).Error)

	accounting := models.User{
		Username: "accounting-reviewer",
		Email:    "accounting-reviewer@test.local",
		Password: "secret",
		Role:     models.RoleAccounting,
	}
	require.NoError(t, db.Create(&accounting).Error)

	approver := models.User{
		Username:  "finance-manager",
		Email:     "finance-manager@test.local",
		Password:  "secret",
		Role:      models.RoleFinanceManager,
		FirstName: "Finance",
		LastName:  "Manager",
	}
	require.NoError(t, db.Create(&approver).Error)

	contact := models.Contact{
		Code: "VEN-APPROVAL-001",
		Name: "Vendor Approval Test",
		Type: "VENDOR",
	}
	require.NoError(t, db.Create(&contact).Error)

	cashAccount := models.Account{
		Code:     "1101-TEST",
		Name:     "Bank Test",
		Type:     models.AccountTypeAsset,
		IsActive: true,
	}
	require.NoError(t, db.Create(&cashAccount).Error)

	cashBank := models.CashBank{
		Code:      "BANK-TEST",
		Name:      "Bank Test",
		Type:      "BANK",
		AccountID: cashAccount.ID,
		Currency:  "IDR",
		IsActive:  true,
		Balance:   250000,
		UserID:    requester.ID,
	}
	require.NoError(t, db.Create(&cashBank).Error)

	payment := models.Payment{
		Code:      "PAY/2026/04/0001",
		ContactID: contact.ID,
		UserID:    requester.ID,
		Date:      time.Date(2026, 4, 15, 9, 0, 0, 0, time.UTC),
		Amount:    125000,
		Method:    models.PaymentMethodBankTransfer,
		Status:    models.PaymentStatusDraft,
		Notes:     "Waiting finance manager review",
	}
	require.NoError(t, db.Create(&payment).Error)

	service := &EnhancedPaymentServiceWithJournal{db: db}

	rejectedPayment, err := service.RejectPayment(payment.ID, approver.ID, "Dokumen pendukung belum lengkap")
	require.NoError(t, err)
	require.Equal(t, models.PaymentStatusRejected, rejectedPayment.Status)
	require.NotNil(t, rejectedPayment.RejectedBy)
	require.Equal(t, approver.ID, *rejectedPayment.RejectedBy)
	require.NotNil(t, rejectedPayment.RejectedAt)
	require.Equal(t, "Dokumen pendukung belum lengkap", rejectedPayment.RejectionReason)
	require.Nil(t, rejectedPayment.ApprovedBy)
	require.Nil(t, rejectedPayment.ApprovedAt)
	require.NotNil(t, rejectedPayment.Rejector)
	require.Equal(t, approver.ID, rejectedPayment.Rejector.ID)
}

func TestRejectPaymentRequiresReason(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Account{},
		&models.CashBank{},
		&models.Payment{},
	))

	requester := models.User{
		Username:  "staff-no-reason",
		Email:     "staff-no-reason@test.local",
		Password:  "secret",
		Role:      models.RoleFinanceStaff,
		FirstName: "Staff",
	}
	require.NoError(t, db.Create(&requester).Error)

	approver := models.User{
		Username:  "manager-no-reason",
		Email:     "manager-no-reason@test.local",
		Password:  "secret",
		Role:      models.RoleFinanceManager,
		FirstName: "Manager",
	}
	require.NoError(t, db.Create(&approver).Error)

	contact := models.Contact{
		Code: "VEN-APPROVAL-002",
		Name: "Vendor Missing Reason",
		Type: "VENDOR",
	}
	require.NoError(t, db.Create(&contact).Error)

	payment := models.Payment{
		Code:      "PAY/2026/04/0002",
		ContactID: contact.ID,
		UserID:    requester.ID,
		Date:      time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
		Amount:    50000,
		Method:    models.PaymentMethodBankTransfer,
		Status:    models.PaymentStatusDraft,
	}
	require.NoError(t, db.Create(&payment).Error)

	service := &EnhancedPaymentServiceWithJournal{db: db}

	_, err = service.RejectPayment(payment.ID, approver.ID, "   ")
	require.Error(t, err)
	require.Contains(t, err.Error(), "rejection reason is required")
}

func TestApproveStandardPaymentRoutesDraftToAccountingWithoutPosting(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Payment{},
	))

	requester := models.User{
		Username: "finance-staff-standard",
		Email:    "finance-staff-standard@test.local",
		Password: "secret",
		Role:     models.RoleFinanceStaff,
	}
	require.NoError(t, db.Create(&requester).Error)

	approver := models.User{
		Username: "finance-manager-standard",
		Email:    "finance-manager-standard@test.local",
		Password: "secret",
		Role:     models.RoleFinanceManager,
	}
	require.NoError(t, db.Create(&approver).Error)

	contact := models.Contact{
		Code: "VEN-APPROVAL-003",
		Name: "Vendor Approval Routing",
		Type: "VENDOR",
	}
	require.NoError(t, db.Create(&contact).Error)

	payment := models.Payment{
		Code:        "PAY/2026/04/0003",
		ContactID:   contact.ID,
		UserID:      requester.ID,
		Date:        time.Date(2026, 4, 15, 11, 0, 0, 0, time.UTC),
		Amount:      75000,
		Method:      models.PaymentMethodBankTransfer,
		Status:      models.PaymentStatusDraft,
		PaymentType: models.PaymentTypeRegular,
	}
	require.NoError(t, db.Create(&payment).Error)

	service := &EnhancedPaymentServiceWithJournal{db: db}

	response, err := service.ApproveStandardPayment(payment.ID, approver.ID, false)
	require.NoError(t, err)
	require.NotNil(t, response.Payment)
	require.Equal(t, models.PaymentStatusPendingAccounting, response.Payment.Status)
	require.Equal(t, models.PaymentApprovalStageAccounting, response.Payment.ApprovalStage)
	require.NotNil(t, response.Payment.FinanceManagerApprovedBy)
	require.Equal(t, approver.ID, *response.Payment.FinanceManagerApprovedBy)
	require.NotNil(t, response.Payment.FinanceManagerApprovedAt)
	require.Nil(t, response.Payment.ApprovedBy)
	require.Nil(t, response.Payment.ApprovedAt)
	require.Nil(t, response.Payment.JournalEntryID)
}

func TestUpdatePendingAccountingPaymentMovesToFinalReview(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Payment{},
	))

	requester := models.User{
		Username: "accounting-review-staff",
		Email:    "accounting-review-staff@test.local",
		Password: "secret",
		Role:     models.RoleFinanceStaff,
	}
	require.NoError(t, db.Create(&requester).Error)

	accounting := models.User{
		Username: "accounting-reviewer",
		Email:    "accounting-reviewer@test.local",
		Password: "secret",
		Role:     models.RoleAccounting,
	}
	require.NoError(t, db.Create(&accounting).Error)

	contact := models.Contact{
		Code: "VEN-APPROVAL-004",
		Name: "Vendor Accounting Review",
		Type: "VENDOR",
	}
	require.NoError(t, db.Create(&contact).Error)

	cashAccount := models.Account{
		Code:     "1101-TEST",
		Name:     "Bank Test",
		Type:     models.AccountTypeAsset,
		IsActive: true,
	}
	require.NoError(t, db.Create(&cashAccount).Error)

	cashBank := models.CashBank{
		Code:      "BANK-TEST",
		Name:      "Bank Test",
		Type:      "BANK",
		AccountID: cashAccount.ID,
		Currency:  "IDR",
		IsActive:  true,
		Balance:   250000,
		UserID:    requester.ID,
	}
	require.NoError(t, db.Create(&cashBank).Error)

	payment := models.Payment{
		Code:          "PAY/2026/04/0004",
		ContactID:     contact.ID,
		UserID:        requester.ID,
		Date:          time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
		Amount:        91000,
		Method:        models.PaymentMethodBankTransfer,
		Status:        models.PaymentStatusPendingAccounting,
		PaymentType:   models.PaymentTypeRegular,
		ApprovalStage: models.PaymentApprovalStageAccounting,
	}
	require.NoError(t, db.Create(&payment).Error)

	service := &EnhancedPaymentServiceWithJournal{db: db}

	updatedPayment, err := service.UpdateDraftPayment(payment.ID, accounting.ID, func(tx *gorm.DB, p *models.Payment) error {
		p.CashBankID = &cashBank.ID
		p.CashBankCode = cashBank.Code
		p.CashBankName = cashBank.Name
		p.CashBankCurrency = cashBank.Currency
		p.CashBankGLAccountID = &cashAccount.ID
		p.CashBankGLAccountCode = cashAccount.Code
		p.CashBankGLAccountName = cashAccount.Name
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, models.PaymentStatusPendingFinalReview, updatedPayment.Status)
	require.Equal(t, models.PaymentApprovalStageFinanceManagerFinal, updatedPayment.ApprovalStage)
	require.NotNil(t, updatedPayment.AccountingReviewedBy)
	require.Equal(t, accounting.ID, *updatedPayment.AccountingReviewedBy)
	require.NotNil(t, updatedPayment.AccountingReviewedAt)
	require.NotNil(t, updatedPayment.AccountingReviewer)
	require.Equal(t, accounting.ID, updatedPayment.AccountingReviewer.ID)
	require.NotNil(t, updatedPayment.CashBankID)
	require.Equal(t, cashBank.ID, *updatedPayment.CashBankID)
	require.Equal(t, "BANK-TEST", updatedPayment.CashBankCode)
	require.Equal(t, "1101-TEST", updatedPayment.CashBankGLAccountCode)
}
