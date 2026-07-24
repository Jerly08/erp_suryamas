package services

import (
	"testing"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type legacyNonInvoiceFixture struct {
	db         *gorm.DB
	service    *EnhancedPaymentServiceWithJournal
	requester  models.User
	accounting models.User
	contact    models.Contact
	cashBank   models.CashBank
	contraCOA  models.Account
	date       time.Time
}

func newLegacyNonInvoiceFixture(t *testing.T) legacyNonInvoiceFixture {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Account{},
		&models.CashBank{},
		&models.Payment{},
		&models.PaymentAllocation{},
		&models.Purchase{},
		&models.PurchasePayment{},
		&models.SSOTJournalEntry{},
		&models.JournalEntry{},
		&models.CashBankTransaction{},
	))

	requester := models.User{
		Username: "legacy-payment-requester",
		Email:    "legacy-payment-requester@test.local",
		Password: "secret",
		Role:     models.RoleFinanceStaff,
		IsActive: true,
	}
	accounting := models.User{
		Username: "legacy-payment-accounting",
		Email:    "legacy-payment-accounting@test.local",
		Password: "secret",
		Role:     models.RoleAccounting,
		IsActive: true,
	}
	contact := models.Contact{
		Code:     "LEGACY-NON-INVOICE-CONTACT",
		Name:     "Legacy Non-Invoice Contact",
		Type:     models.ContactTypeVendor,
		IsActive: true,
	}
	require.NoError(t, db.Create(&requester).Error)
	require.NoError(t, db.Create(&accounting).Error)
	require.NoError(t, db.Create(&contact).Error)

	cashGL := models.Account{
		Code:     "1101-LEGACY-BRIDGE",
		Name:     "Bank Legacy Bridge",
		Type:     models.AccountTypeAsset,
		IsActive: true,
	}
	contraCOA := models.Account{
		Code:     "6101-LEGACY-BRIDGE",
		Name:     "Beban Legacy Bridge",
		Type:     models.AccountTypeExpense,
		IsActive: true,
	}
	require.NoError(t, db.Create(&cashGL).Error)
	require.NoError(t, db.Create(&contraCOA).Error)

	cashBank := models.CashBank{
		Code:      "BANK-LEGACY-BRIDGE",
		Name:      "Bank Legacy Bridge",
		Type:      models.CashBankTypeBank,
		AccountID: cashGL.ID,
		Currency:  "IDR",
		Balance:   1_000_000,
		IsActive:  true,
		UserID:    requester.ID,
	}
	require.NoError(t, db.Create(&cashBank).Error)

	return legacyNonInvoiceFixture{
		db: db,
		service: &EnhancedPaymentServiceWithJournal{
			db:           db,
			paymentRepo:  *repositories.NewPaymentRepository(db),
			cashBankRepo: repositories.NewCashBankRepository(db),
		},
		requester:  requester,
		accounting: accounting,
		contact:    contact,
		cashBank:   cashBank,
		contraCOA:  contraCOA,
		date:       time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
	}
}

func (f legacyNonInvoiceFixture) createCandidate(t *testing.T, code string) models.Payment {
	t.Helper()
	payment := models.Payment{
		Code:          code,
		ContactID:     f.contact.ID,
		UserID:        f.requester.ID,
		Date:          f.date,
		Amount:        150_000,
		Method:        models.PaymentMethodBankTransfer,
		Status:        models.PaymentStatusPendingAccounting,
		ApprovalStage: models.PaymentApprovalStageAccounting,
		CashBankID:    &f.cashBank.ID,
	}
	require.NoError(t, f.db.Create(&payment).Error)
	return payment
}

func TestLegacyNonInvoicePaymentCanEnterAccountingCOAReview(t *testing.T) {
	fixture := newLegacyNonInvoiceFixture(t)
	payment := fixture.createCandidate(t, "PAY-LEGACY-BRIDGE-ELIGIBLE")

	eligible, err := fixture.service.isLegacyNonInvoicePaymentEligible(fixture.db, &payment)
	require.NoError(t, err)
	require.True(t, eligible)

	list, err := fixture.service.ListPayments(repositories.PaymentFilter{
		Status: models.PaymentStatusPendingAccounting,
		Limit:  20,
	})
	require.NoError(t, err)
	require.Len(t, list.Data, 1)
	require.True(t, list.Data[0].LegacyNonInvoiceEligible)

	updated, err := fixture.service.CompleteLegacyNonInvoicePaymentAccountingReview(
		payment.ID,
		fixture.accounting.ID,
		ExpensePaymentAccountingReviewRequest{ExpenseAccountID: fixture.contraCOA.ID},
	)
	require.NoError(t, err)
	require.Equal(t, models.PaymentTypeExpense, updated.PaymentType)
	require.Equal(t, models.PaymentStatusPendingFinalReview, updated.Status)
	require.Equal(t, models.PaymentApprovalStageFinanceManagerFinal, updated.ApprovalStage)
	require.NotNil(t, updated.ExpenseAccountID)
	require.Equal(t, fixture.contraCOA.ID, *updated.ExpenseAccountID)
	require.NotNil(t, updated.AccountingReviewedBy)
	require.Equal(t, fixture.accounting.ID, *updated.AccountingReviewedBy)
	require.Nil(t, updated.JournalEntryID)

	var cashBankMovements int64
	require.NoError(t, fixture.db.Model(&models.CashBankTransaction{}).
		Where("reference_id = ?", payment.ID).
		Count(&cashBankMovements).Error)
	require.Zero(t, cashBankMovements)
}

func TestLegacyNonInvoicePaymentEligibilityFailsClosedForLinkedOrPostedData(t *testing.T) {
	fixture := newLegacyNonInvoiceFixture(t)

	tests := []struct {
		name      string
		code      string
		addRecord func(t *testing.T, payment *models.Payment)
	}{
		{
			name: "standard payment type",
			code: "PAY-LEGACY-BRIDGE-REGULAR",
			addRecord: func(t *testing.T, payment *models.Payment) {
				payment.PaymentType = models.PaymentTypeRegular
				require.NoError(t, fixture.db.Save(payment).Error)
			},
		},
		{
			name: "payment allocation",
			code: "PAY-LEGACY-BRIDGE-ALLOCATION",
			addRecord: func(t *testing.T, payment *models.Payment) {
				require.NoError(t, fixture.db.Create(&models.PaymentAllocation{
					PaymentID:       uint64(payment.ID),
					AllocatedAmount: payment.Amount,
				}).Error)
			},
		},
		{
			name: "pending allocation reference",
			code: "PAY-LEGACY-BRIDGE-PENDING-ALLOCATION",
			addRecord: func(t *testing.T, payment *models.Payment) {
				payment.Reference = models.MarkPendingDraftAllocationsReference("legacy purchase payment")
				require.NoError(t, fixture.db.Save(payment).Error)
			},
		},
		{
			name: "purchase payment link",
			code: "PAY-LEGACY-BRIDGE-PURCHASE",
			addRecord: func(t *testing.T, payment *models.Payment) {
				purchase := models.Purchase{
					Code:              "PO-LEGACY-BRIDGE-PURCHASE",
					VendorID:          fixture.contact.ID,
					UserID:            fixture.requester.ID,
					Date:              fixture.date,
					DueDate:           fixture.date,
					TotalAmount:       payment.Amount,
					OutstandingAmount: payment.Amount,
					PaymentMethod:     models.PurchasePaymentCredit,
					Status:            models.PurchaseStatusApproved,
				}
				require.NoError(t, fixture.db.Create(&purchase).Error)
				require.NoError(t, fixture.db.Create(&models.PurchasePayment{
					PurchaseID:    purchase.ID,
					PaymentNumber: payment.Code,
					Date:          fixture.date,
					Amount:        payment.Amount,
					Method:        payment.Method,
					UserID:        fixture.requester.ID,
					PaymentID:     &payment.ID,
				}).Error)
			},
		},
		{
			name: "SSOT journal",
			code: "PAY-LEGACY-BRIDGE-SSOT",
			addRecord: func(t *testing.T, payment *models.Payment) {
				sourceID := uint64(payment.ID)
				require.NoError(t, fixture.db.Create(&models.SSOTJournalEntry{
					EntryNumber: "JE-LEGACY-BRIDGE-SSOT",
					SourceType:  models.SSOTSourceTypePayment,
					SourceID:    &sourceID,
					EntryDate:   fixture.date,
					Description: "Posted legacy payment",
					Status:      models.JournalStatusPosted,
					CreatedBy:   uint64(fixture.requester.ID),
				}).Error)
			},
		},
		{
			name: "legacy journal entry",
			code: "PAY-LEGACY-BRIDGE-JOURNAL",
			addRecord: func(t *testing.T, payment *models.Payment) {
				require.NoError(t, fixture.db.Create(&models.JournalEntry{
					Code:          "JE-LEGACY-BRIDGE-JOURNAL",
					Description:   "Posted legacy payment",
					ReferenceType: models.JournalRefPayment,
					ReferenceID:   &payment.ID,
					EntryDate:     fixture.date,
					UserID:        fixture.requester.ID,
					Status:        models.JournalStatusPosted,
					TotalDebit:    1,
					TotalCredit:   1,
					IsBalanced:    true,
				}).Error)
			},
		},
		{
			name: "cash bank movement",
			code: "PAY-LEGACY-BRIDGE-CASH",
			addRecord: func(t *testing.T, payment *models.Payment) {
				require.NoError(t, fixture.db.Create(&models.CashBankTransaction{
					CashBankID:      fixture.cashBank.ID,
					ReferenceType:   "PAYMENT_MADE",
					ReferenceID:     payment.ID,
					Amount:          -payment.Amount,
					Currency:        "IDR",
					ExchangeRate:    1,
					BookAmountIDR:   -payment.Amount,
					BalanceAfter:    fixture.cashBank.Balance - payment.Amount,
					TransactionDate: fixture.date,
				}).Error)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payment := fixture.createCandidate(t, tt.code)
			tt.addRecord(t, &payment)

			eligible, err := fixture.service.isLegacyNonInvoicePaymentEligible(fixture.db, &payment)
			require.NoError(t, err)
			require.False(t, eligible)

			_, err = fixture.service.CompleteLegacyNonInvoicePaymentAccountingReview(
				payment.ID,
				fixture.accounting.ID,
				ExpensePaymentAccountingReviewRequest{ExpenseAccountID: fixture.contraCOA.ID},
			)
			require.ErrorContains(t, err, "not eligible")

			var persisted models.Payment
			require.NoError(t, fixture.db.First(&persisted, payment.ID).Error)
			require.Equal(t, payment.PaymentType, persisted.PaymentType)
			require.Equal(t, models.PaymentStatusPendingAccounting, persisted.Status)
			require.Nil(t, persisted.ExpenseAccountID)
		})
	}
}
