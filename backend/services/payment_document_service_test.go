package services

import (
	"strings"
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPaymentDocumentServiceCreatesAppendOnlyHistoryAndAllowsFinanceReviewEvidence(t *testing.T) {
	db, service, creator, accounting, financeManager, payment := newPaymentDocumentServiceFixture(t)

	documents, err := service.UploadDocuments(payment.ID, creator.ID, models.RoleFinanceStaff, []PaymentDocumentUploadInput{{
		DocumentType:    models.PaymentDocumentTypeVendorInvoice,
		ReferenceNumber: "INV-VENDOR-001",
		Notes:           "Invoice vendor awal",
		FileName:        "invoice-vendor.pdf",
		FilePath:        "/uploads/payment-documents/invoice-vendor.pdf",
		FileSize:        256,
		MimeType:        "application/pdf",
	}})
	require.NoError(t, err)
	require.Len(t, documents, 1)
	require.Equal(t, models.PaymentDocumentStatusActive, documents[0].Status)
	require.Equal(t, creator.ID, documents[0].UploadedBy)
	require.Len(t, documents[0].Events, 1)
	require.Equal(t, models.PaymentDocumentEventUploaded, documents[0].Events[0].Action)
	require.Equal(t, creator.ID, documents[0].Events[0].ActorID)

	// Accounting can view the evidence history for a payment created by someone else.
	visibleToAccounting, err := service.ListDocuments(payment.ID, accounting.ID, models.RoleAccounting)
	require.NoError(t, err)
	require.Len(t, visibleToAccounting, 1)

	// Finance Manager may add a missing supporting file during review, without
	// changing the payment itself or its first document.
	financeDocuments, err := service.UploadDocuments(payment.ID, financeManager.ID, models.RoleFinanceManager, []PaymentDocumentUploadInput{{
		DocumentType: models.PaymentDocumentTypePaymentProof,
		FileName:     "bukti-transfer.pdf",
		FilePath:     "/uploads/payment-documents/bukti-transfer.pdf",
		FileSize:     512,
		MimeType:     "application/pdf",
	}})
	require.NoError(t, err)
	require.Len(t, financeDocuments, 1)
	require.Equal(t, financeManager.ID, financeDocuments[0].UploadedBy)

	var documentCount int64
	require.NoError(t, db.Model(&models.PaymentDocument{}).Where("payment_id = ?", payment.ID).Count(&documentCount).Error)
	require.Equal(t, int64(2), documentCount)

	// Uploading evidence must never mutate the financial payment record. Journal
	// posting and allocation are intentionally outside this service.
	var persistedPayment models.Payment
	require.NoError(t, db.First(&persistedPayment, payment.ID).Error)
	require.Equal(t, payment.Code, persistedPayment.Code)
	require.Equal(t, payment.Amount, persistedPayment.Amount)
	require.Equal(t, payment.Status, persistedPayment.Status)
	require.Equal(t, payment.UserID, persistedPayment.UserID)
}

func TestPaymentDocumentServiceRejectsUnauthorizedOrClosedPaymentUploads(t *testing.T) {
	db, service, creator, _, _, payment := newPaymentDocumentServiceFixture(t)

	otherUser := models.User{
		Username: "other-payment-doc-user",
		Email:    "other-payment-doc-user@test.local",
		Password: "secret",
		Role:     models.RoleEmployee,
	}
	director := models.User{
		Username: "director-payment-doc-user",
		Email:    "director-payment-doc-user@test.local",
		Password: "secret",
		Role:     models.RoleDirector,
	}
	require.NoError(t, db.Create(&otherUser).Error)
	require.NoError(t, db.Create(&director).Error)

	input := []PaymentDocumentUploadInput{{
		FileName: "support.pdf",
		FilePath: "/uploads/payment-documents/support.pdf",
		FileSize: 100,
		MimeType: "application/pdf",
	}}

	_, err := service.UploadDocuments(payment.ID, otherUser.ID, models.RoleEmployee, input)
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "access denied")

	_, err = service.UploadDocuments(payment.ID, director.ID, models.RoleDirector, input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Director can only")

	require.NoError(t, db.Model(&models.Payment{}).Where("id = ?", payment.ID).Update("status", models.PaymentStatusRejected).Error)
	_, err = service.UploadDocuments(payment.ID, creator.ID, models.RoleFinanceStaff, input)
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "cannot be added")
}

func TestPaymentDocumentServiceInvalidatesWithoutDeletingHistory(t *testing.T) {
	db, service, creator, accounting, _, payment := newPaymentDocumentServiceFixture(t)

	documents, err := service.UploadDocuments(payment.ID, creator.ID, models.RoleFinanceStaff, []PaymentDocumentUploadInput{{
		DocumentType: models.PaymentDocumentTypeTaxInvoice,
		FileName:     "faktur-lama.pdf",
		FilePath:     "/uploads/payment-documents/faktur-lama.pdf",
		FileSize:     700,
		MimeType:     "application/pdf",
	}})
	require.NoError(t, err)

	invalidated, err := service.InvalidateDocument(documents[0].ID, accounting.ID, models.RoleAccounting, "Faktur diunggah dengan nomor yang salah")
	require.NoError(t, err)
	require.Equal(t, models.PaymentDocumentStatusInvalidated, invalidated.Status)
	require.Equal(t, accounting.ID, *invalidated.InvalidatedBy)
	require.NotNil(t, invalidated.InvalidatedAt)
	require.Len(t, invalidated.Events, 2)
	require.Equal(t, models.PaymentDocumentEventInvalidated, invalidated.Events[1].Action)

	var stored models.PaymentDocument
	require.NoError(t, db.First(&stored, documents[0].ID).Error)
	require.Equal(t, models.PaymentDocumentStatusInvalidated, stored.Status)
	require.Equal(t, "faktur-lama.pdf", stored.FileName)
}

func newPaymentDocumentServiceFixture(t *testing.T) (*gorm.DB, *PaymentDocumentService, models.User, models.User, models.User, models.Payment) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Payment{},
		&models.PaymentDocument{},
		&models.PaymentDocumentEvent{},
	))

	creator := models.User{
		Username: "creator-payment-doc",
		Email:    "creator-payment-doc@test.local",
		Password: "secret",
		Role:     models.RoleFinanceStaff,
	}
	accounting := models.User{
		Username: "accounting-payment-doc",
		Email:    "accounting-payment-doc@test.local",
		Password: "secret",
		Role:     models.RoleAccounting,
	}
	financeManager := models.User{
		Username: "finance-manager-payment-doc",
		Email:    "finance-manager-payment-doc@test.local",
		Password: "secret",
		Role:     models.RoleFinanceManager,
	}
	require.NoError(t, db.Create(&creator).Error)
	require.NoError(t, db.Create(&accounting).Error)
	require.NoError(t, db.Create(&financeManager).Error)

	contact := models.Contact{Code: "VENDOR-PAYMENT-DOC", Name: "Vendor Payment Doc", Type: models.ContactTypeVendor, IsActive: true}
	require.NoError(t, db.Create(&contact).Error)
	payment := models.Payment{
		Code:      "PAYMENT-DOC-001",
		ContactID: contact.ID,
		UserID:    creator.ID,
		Date:      time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Amount:    100000,
		Method:    models.PaymentMethodBankTransfer,
		Status:    models.PaymentStatusDraft,
	}
	require.NoError(t, db.Create(&payment).Error)

	return db, NewPaymentDocumentService(db), creator, accounting, financeManager, payment
}
