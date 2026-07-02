package services

import (
	"strings"
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestReviewPurchasePaymentProofAllowsApprovedPurchasePendingAccounting(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Purchase{},
		&models.PurchasePayment{},
	))

	manager := models.User{
		Username:  "proof-review-fm",
		Email:     "proof-review-fm@test.local",
		Password:  "secret",
		Role:      models.RoleFinanceManager,
		FirstName: "Finance",
		LastName:  "Manager",
		IsActive:  true,
	}
	staff := models.User{
		Username: "proof-upload-staff",
		Email:    "proof-upload-staff@test.local",
		Password: "secret",
		Role:     models.RoleFinanceStaff,
		IsActive: true,
	}
	require.NoError(t, db.Create(&manager).Error)
	require.NoError(t, db.Create(&staff).Error)

	vendor := models.Contact{
		Code:     "VEND-PROOF",
		Name:     "Vendor Proof",
		Type:     models.ContactTypeVendor,
		IsActive: true,
	}
	require.NoError(t, db.Create(&vendor).Error)

	purchaseDate := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	purchase := models.Purchase{
		Code:           "PO-PROOF-PENDING-ACCOUNTING",
		VendorID:       vendor.ID,
		UserID:         staff.ID,
		Date:           purchaseDate,
		DueDate:        purchaseDate,
		PaymentMethod:  models.PurchasePaymentTransfer,
		Status:         models.PurchaseStatusPendingAccounting,
		ApprovalStatus: models.PurchaseApprovalApproved,
		TotalAmount:    100000,
	}
	require.NoError(t, db.Create(&purchase).Error)

	uploadedAt := time.Now()
	payment := models.PurchasePayment{
		PurchaseID:               purchase.ID,
		PaymentNumber:            "PAY-PROOF-PENDING-ACCOUNTING",
		Date:                     purchaseDate,
		Amount:                   100000,
		Method:                   models.PurchasePaymentTransfer,
		UserID:                   staff.ID,
		PaymentProofFileName:     "bukti-bank.pdf",
		PaymentProofFilePath:     "/uploads/purchase-payment-proofs/bukti-bank.pdf",
		PaymentProofMimeType:     "application/pdf",
		PaymentProofUploadedBy:   &staff.ID,
		PaymentProofUploadedAt:   &uploadedAt,
		PaymentProofReviewStatus: models.PurchasePaymentProofPending,
		PaymentProofReviewNotes:  "",
	}
	require.NoError(t, db.Create(&payment).Error)

	service := &PurchaseService{db: db}
	reviewed, err := service.ReviewPurchasePaymentProof(
		payment.ID,
		manager.ID,
		manager.Role,
		models.PurchasePaymentProofApproved,
		"Bukti bank sesuai",
	)

	require.NoError(t, err)
	require.Equal(t, models.PurchasePaymentProofApproved, reviewed.PaymentProofReviewStatus)
	require.NotNil(t, reviewed.PaymentProofReviewedBy)
	require.Equal(t, manager.ID, *reviewed.PaymentProofReviewedBy)
	require.NotNil(t, reviewed.PaymentProofReviewedAt)
	require.Equal(t, "Bukti bank sesuai", reviewed.PaymentProofReviewNotes)
}
