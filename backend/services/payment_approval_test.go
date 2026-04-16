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
