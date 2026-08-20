package services

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupCustomerSettlementTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Account{},
		&models.Settings{},
		&models.Sale{},
	))
	return db
}

func seedCustomerSettlementInvoice(t *testing.T, db *gorm.DB, outstandingAmount float64) (models.Contact, models.Sale) {
	t.Helper()

	user := models.User{
		Username: "settlement-test-user-" + strings.ReplaceAll(t.Name(), "/", "-"),
		Email:    "settlement-" + strings.ReplaceAll(t.Name(), "/", "-") + "@test.local",
		Password: "secret",
		Role:     models.RoleFinance,
		IsActive: true,
	}
	customer := models.Contact{
		Code:     "CUS-" + strings.ReplaceAll(t.Name(), "/", "-"),
		Name:     "Petani Test",
		Type:     models.ContactTypeCustomer,
		IsActive: true,
	}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&customer).Error)

	date := time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC)
	sale := models.Sale{
		Code:              "SOA-" + strings.ReplaceAll(t.Name(), "/", "-"),
		InvoiceNumber:     "INV-" + strings.ReplaceAll(t.Name(), "/", "-"),
		CustomerID:        customer.ID,
		UserID:            user.ID,
		Date:              date,
		DueDate:           date.AddDate(0, 0, 30),
		Currency:          "IDR",
		ExchangeRate:      1,
		TotalAmount:       outstandingAmount,
		OutstandingAmount: outstandingAmount,
		Status:            models.SaleStatusInvoiced,
	}
	require.NoError(t, db.Create(&sale).Error)
	require.NoError(t, db.Create(&models.Settings{
		CompanyName:              "Customer Settlement Test",
		PaymentRoundingThreshold: 1000,
	}).Error)

	return customer, sale
}

func TestPlanCustomerSettlementDefaultsExcessToCustomerAdvance(t *testing.T) {
	db := setupCustomerSettlementTestDB(t)
	customer, sale := seedCustomerSettlementInvoice(t, db, 1_936_950)
	service := &EnhancedPaymentServiceWithJournal{statusValidator: NewStatusValidationHelper()}

	plan, err := service.planCustomerSettlement(db, &PaymentWithJournalRequest{
		ContactID: customer.ID,
		Amount:    1_937_000,
		InvoiceAllocations: []AllocationItem{{
			InvoiceID: &sale.ID,
			Amount:    1_936_950,
		}},
	}, &customer)

	require.NoError(t, err)
	require.Equal(t, models.PaymentSettlementCustomerAdvance, plan.Treatment)
	require.InDelta(t, 1_936_950, plan.InvoiceSettlementAmount, 0.001)
	require.InDelta(t, 50, plan.Difference, 0.001)
	require.InDelta(t, 50, plan.CustomerAdvanceAmount, 0.001)
	require.Zero(t, plan.RoundingAmount)
	require.False(t, plan.RequiresApproval)
}

func TestPlanCustomerSettlementRoundingRequiresReasonAndThreshold(t *testing.T) {
	db := setupCustomerSettlementTestDB(t)
	customer, sale := seedCustomerSettlementInvoice(t, db, 1_936_950)
	service := &EnhancedPaymentServiceWithJournal{statusValidator: NewStatusValidationHelper()}

	baseRequest := PaymentWithJournalRequest{
		ContactID:           customer.ID,
		Amount:              1_937_000,
		SettlementTreatment: models.PaymentSettlementRounding,
		InvoiceAllocations: []AllocationItem{{
			InvoiceID: &sale.ID,
			Amount:    1_936_950,
		}},
	}

	_, err := service.planCustomerSettlement(db, &baseRequest, &customer)
	require.ErrorContains(t, err, "reason is required")

	baseRequest.SettlementReason = "Pembulatan sesuai nominal transfer bank"
	plan, err := service.planCustomerSettlement(db, &baseRequest, &customer)
	require.NoError(t, err)
	require.Equal(t, models.PaymentSettlementRounding, plan.Treatment)
	require.InDelta(t, 50, plan.RoundingAmount, 0.001)
	require.Equal(t, "OVERPAYMENT", plan.RoundingDirection)
	require.True(t, plan.RequiresApproval)
	require.InDelta(t, 1000, plan.RoundingThreshold, 0.001)

	baseRequest.Amount = 1_938_001
	_, err = service.planCustomerSettlement(db, &baseRequest, &customer)
	require.ErrorContains(t, err, "exceeds configured threshold")
}

func TestPlanCustomerSettlementShortfallRoundingUsesFullInvoiceTarget(t *testing.T) {
	db := setupCustomerSettlementTestDB(t)
	customer, sale := seedCustomerSettlementInvoice(t, db, 1_936_950)
	service := &EnhancedPaymentServiceWithJournal{statusValidator: NewStatusValidationHelper()}

	plan, err := service.planCustomerSettlement(db, &PaymentWithJournalRequest{
		ContactID:           customer.ID,
		Amount:              1_936_475,
		SettlementTreatment: models.PaymentSettlementRounding,
		SettlementReason:    "Nominal transfer kurang kecil",
		InvoiceAllocations: []AllocationItem{{
			InvoiceID: &sale.ID,
			Amount:    1_936_475, // The UI allocates the actual bank receipt.
		}},
	}, &customer)

	require.NoError(t, err)
	require.Equal(t, models.PaymentSettlementRounding, plan.Treatment)
	require.InDelta(t, sale.OutstandingAmount, plan.InvoiceSettlementAmount, 0.001)
	require.InDelta(t, -475, plan.Difference, 0.001)
	require.InDelta(t, 475, plan.RoundingAmount, 0.001)
	require.Equal(t, "SHORTFALL", plan.RoundingDirection)
	require.True(t, plan.RequiresApproval)
}

func TestBuildCustomerReceiptJournalLinesClassifiesAdvanceAndRounding(t *testing.T) {
	db := setupCustomerSettlementTestDB(t)
	service := &EnhancedPaymentServiceWithJournal{}

	accounts := []models.Account{
		{Code: "1101-001", Name: "Bank", Type: models.AccountTypeAsset, IsActive: true},
		{Code: "1102-001", Name: "Piutang Usaha", Type: models.AccountTypeAsset, IsActive: true},
		{Code: "2190-001", Name: "Uang Muka Pelanggan", Type: models.AccountTypeLiability, IsActive: true},
		{Code: "7106", Name: "Pendapatan Pembulatan", Type: models.AccountTypeRevenue, IsActive: true},
		{Code: "8106", Name: "Beban Pembulatan", Type: models.AccountTypeExpense, IsActive: true},
	}
	for i := range accounts {
		require.NoError(t, db.Create(&accounts[i]).Error)
	}

	cashBank := &models.CashBank{AccountID: accounts[0].ID}
	const invoiceAmount = 1_936_950.0

	t.Run("excess receipt credits customer advance, not receivables", func(t *testing.T) {
		payment := &models.Payment{
			Code:                "RCV-ADVANCE",
			Amount:              1_937_000,
			BookAmountIDR:       1_937_000,
			Currency:            "IDR",
			SettlementTreatment: models.PaymentSettlementCustomerAdvance,
		}
		lines, _, err := service.buildCustomerReceiptJournalLines(db, payment, cashBank, "Petani", invoiceAmount, invoiceAmount)
		require.NoError(t, err)
		requireJournalLineAmount(t, lines, uint64(accounts[0].ID), 1_937_000, 0)
		requireJournalLineAmount(t, lines, uint64(accounts[1].ID), 0, invoiceAmount)
		requireJournalLineAmount(t, lines, uint64(accounts[2].ID), 0, 50)
		requireNoJournalLineForAccount(t, lines, uint64(accounts[3].ID))
	})

	t.Run("approved excess rounding credits rounding income", func(t *testing.T) {
		payment := &models.Payment{
			Code:                "RCV-ROUND-UP",
			Amount:              1_937_000,
			BookAmountIDR:       1_937_000,
			Currency:            "IDR",
			SettlementTreatment: models.PaymentSettlementRounding,
		}
		lines, _, err := service.buildCustomerReceiptJournalLines(db, payment, cashBank, "Petani", invoiceAmount, invoiceAmount)
		require.NoError(t, err)
		requireJournalLineAmount(t, lines, uint64(accounts[0].ID), 1_937_000, 0)
		requireJournalLineAmount(t, lines, uint64(accounts[1].ID), 0, invoiceAmount)
		requireJournalLineAmount(t, lines, uint64(accounts[3].ID), 0, 50)
		requireNoJournalLineForAccount(t, lines, uint64(accounts[2].ID))
	})

	t.Run("approved shortfall rounding debits rounding expense", func(t *testing.T) {
		payment := &models.Payment{
			Code:                "RCV-ROUND-DOWN",
			Amount:              1_936_475,
			BookAmountIDR:       1_936_475,
			Currency:            "IDR",
			SettlementTreatment: models.PaymentSettlementRounding,
		}
		lines, _, err := service.buildCustomerReceiptJournalLines(db, payment, cashBank, "Petani", invoiceAmount, payment.Amount)
		require.NoError(t, err)
		requireJournalLineAmount(t, lines, uint64(accounts[0].ID), payment.Amount, 0)
		requireJournalLineAmount(t, lines, uint64(accounts[1].ID), 0, invoiceAmount)
		requireJournalLineAmount(t, lines, uint64(accounts[4].ID), 475, 0)
		requireNoJournalLineForAccount(t, lines, uint64(accounts[2].ID))
	})
}

func TestCreateCustomerOverpaymentCreatesAdvanceAndDoesNotCreditARForExcess(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.ContactAddress{},
		&models.Account{},
		&models.Settings{},
		&models.AccountingPeriod{},
		&models.CashBank{},
		&models.CashBankTransaction{},
		&models.Sale{},
		&models.SalePayment{},
		&models.SalePaymentSchedule{},
		&models.Payment{},
		&models.PaymentAllocation{},
		&models.CustomerAdvance{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
	))

	user := models.User{
		Username: "customer-overpayment-user",
		Email:    "customer-overpayment-user@test.local",
		Password: "secret",
		Role:     models.RoleFinanceManager,
		IsActive: true,
	}
	customer := models.Contact{
		Code:     "CUS-CUSTOMER-OVERPAYMENT",
		Name:     "Petani Lebih Bayar",
		Type:     models.ContactTypeCustomer,
		IsActive: true,
	}
	accounts := []models.Account{
		{Code: "1101-001", Name: "Bank Penerimaan", Type: models.AccountTypeAsset, IsActive: true},
		{Code: "1102-001", Name: "Piutang Usaha", Type: models.AccountTypeAsset, IsActive: true},
		{Code: "2190-001", Name: "Uang Muka Pelanggan", Type: models.AccountTypeLiability, IsActive: true},
	}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&customer).Error)
	for index := range accounts {
		require.NoError(t, db.Create(&accounts[index]).Error)
	}
	require.NoError(t, db.Create(&models.Settings{
		CompanyName:              "Overpayment Integration Test",
		PaymentRoundingThreshold: 1000,
	}).Error)

	transactionDate := time.Now().Add(-time.Hour).UTC()
	sale := models.Sale{
		Code:              "SOA-CUSTOMER-OVERPAYMENT",
		InvoiceNumber:     "INV-CUSTOMER-OVERPAYMENT",
		CustomerID:        customer.ID,
		UserID:            user.ID,
		Date:              transactionDate,
		DueDate:           transactionDate.AddDate(0, 0, 30),
		Currency:          "IDR",
		ExchangeRate:      1,
		TotalAmount:       1_936_950,
		OutstandingAmount: 1_936_950,
		Status:            models.SaleStatusInvoiced,
	}
	cashBank := models.CashBank{
		Code:      "BANK-CUSTOMER-OVERPAYMENT",
		Name:      "Bank Penerimaan Test",
		Type:      models.CashBankTypeBank,
		AccountID: accounts[0].ID,
		Currency:  "IDR",
		IsActive:  true,
		UserID:    user.ID,
	}
	require.NoError(t, db.Create(&sale).Error)
	require.NoError(t, db.Create(&cashBank).Error)

	paymentRepo := repositories.NewPaymentRepository(db)
	service := NewEnhancedPaymentServiceWithJournal(
		db,
		*paymentRepo,
		repositories.NewContactRepository(db),
		repositories.NewCashBankRepository(db),
		repositories.NewSalesRepository(db),
		repositories.NewPurchaseRepository(db),
		NewUnifiedJournalService(db),
	)

	response, err := service.CreatePaymentWithJournal(&PaymentWithJournalRequest{
		ContactID:         customer.ID,
		CashBankID:        cashBank.ID,
		Date:              transactionDate,
		Amount:            1_937_000,
		Method:            models.PaymentMethodBankTransfer,
		UserID:            user.ID,
		AutoCreateJournal: true,
		InvoiceAllocations: []AllocationItem{{
			InvoiceID: &sale.ID,
			Amount:    sale.OutstandingAmount,
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	require.NotNil(t, response.Payment)
	require.Equal(t, models.PaymentStatusCompleted, response.Payment.Status)
	require.NotNil(t, response.Payment.JournalEntryID)
	require.NotNil(t, response.Settlement)
	require.Equal(t, models.PaymentSettlementCustomerAdvance, response.Settlement.Treatment)
	require.InDelta(t, 50, response.Settlement.CustomerAdvanceAmount, 0.001)

	var advance models.CustomerAdvance
	require.NoError(t, db.Where("payment_id = ?", response.Payment.ID).First(&advance).Error)
	require.Equal(t, customer.ID, advance.CustomerID)
	require.InDelta(t, 50, advance.OriginalAmount, 0.001)
	require.InDelta(t, 50, advance.RemainingAmount, 0.001)
	require.Equal(t, "IDR", advance.Currency)
	require.Equal(t, models.CustomerAdvanceStatusOpen, advance.Status)
	require.NotNil(t, advance.JournalEntryID)
	require.Equal(t, uint64(*response.Payment.JournalEntryID), *advance.JournalEntryID)

	var updatedSale models.Sale
	require.NoError(t, db.First(&updatedSale, sale.ID).Error)
	require.InDelta(t, sale.TotalAmount, updatedSale.PaidAmount, 0.001)
	require.Zero(t, updatedSale.OutstandingAmount)
	require.Equal(t, models.SaleStatusPaid, updatedSale.Status)

	var journalLines []models.SSOTJournalLine
	require.NoError(t, db.Where("journal_id = ?", *response.Payment.JournalEntryID).Order("line_number ASC").Find(&journalLines).Error)
	require.Len(t, journalLines, 3)
	requireSSOTJournalLineAmount(t, journalLines, uint64(accounts[0].ID), 1_937_000, 0)
	requireSSOTJournalLineAmount(t, journalLines, uint64(accounts[1].ID), 0, 1_936_950)
	requireSSOTJournalLineAmount(t, journalLines, uint64(accounts[2].ID), 0, 50)
}

func TestCustomerRoundingWaitsForAccountingAndDifferentFinanceManager(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.ContactAddress{},
		&models.Account{},
		&models.Settings{},
		&models.AccountingPeriod{},
		&models.CashBank{},
		&models.CashBankTransaction{},
		&models.Sale{},
		&models.SalePayment{},
		&models.SalePaymentSchedule{},
		&models.Payment{},
		&models.PaymentAllocation{},
		&models.CustomerAdvance{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
	))

	creator := models.User{
		Username: "rounding-creator",
		Email:    "rounding-creator@test.local",
		Password: "secret",
		Role:     models.RoleFinanceStaff,
		IsActive: true,
	}
	accounting := models.User{
		Username: "rounding-accounting",
		Email:    "rounding-accounting@test.local",
		Password: "secret",
		Role:     models.RoleAccounting,
		IsActive: true,
	}
	financeStaff := models.User{
		Username: "rounding-non-approver",
		Email:    "rounding-non-approver@test.local",
		Password: "secret",
		Role:     models.RoleFinanceStaff,
		IsActive: true,
	}
	financeManager := models.User{
		Username: "rounding-finance-manager",
		Email:    "rounding-finance-manager@test.local",
		Password: "secret",
		Role:     models.RoleFinanceManager,
		IsActive: true,
	}
	customer := models.Contact{
		Code:     "CUS-ROUNDING-WORKFLOW",
		Name:     "Petani Pembulatan",
		Type:     models.ContactTypeCustomer,
		IsActive: true,
	}
	accounts := []models.Account{
		{Code: "1101-001", Name: "Bank Penerimaan", Type: models.AccountTypeAsset, IsActive: true},
		{Code: "1102-001", Name: "Piutang Usaha", Type: models.AccountTypeAsset, IsActive: true},
		{Code: "2190-001", Name: "Uang Muka Pelanggan", Type: models.AccountTypeLiability, IsActive: true},
		{Code: "7106", Name: "Pendapatan Pembulatan", Type: models.AccountTypeRevenue, IsActive: true},
		{Code: "8106", Name: "Beban Pembulatan", Type: models.AccountTypeExpense, IsActive: true},
	}
	for _, user := range []*models.User{&creator, &accounting, &financeStaff, &financeManager} {
		require.NoError(t, db.Create(user).Error)
	}
	require.NoError(t, db.Create(&customer).Error)
	for index := range accounts {
		require.NoError(t, db.Create(&accounts[index]).Error)
	}
	require.NoError(t, db.Create(&models.Settings{
		CompanyName:              "Rounding Workflow Test",
		PaymentRoundingThreshold: 1000,
	}).Error)

	transactionDate := time.Now().Add(-time.Hour).UTC()
	sale := models.Sale{
		Code:              "SOA-ROUNDING-WORKFLOW",
		InvoiceNumber:     "INV-ROUNDING-WORKFLOW",
		CustomerID:        customer.ID,
		UserID:            creator.ID,
		Date:              transactionDate,
		DueDate:           transactionDate.AddDate(0, 0, 30),
		Currency:          "IDR",
		ExchangeRate:      1,
		TotalAmount:       1_936_950,
		OutstandingAmount: 1_936_950,
		Status:            models.SaleStatusInvoiced,
	}
	cashBank := models.CashBank{
		Code:      "BANK-ROUNDING-WORKFLOW",
		Name:      "Bank Rounding Workflow",
		Type:      models.CashBankTypeBank,
		AccountID: accounts[0].ID,
		Currency:  "IDR",
		IsActive:  true,
		UserID:    creator.ID,
	}
	require.NoError(t, db.Create(&sale).Error)
	require.NoError(t, db.Create(&cashBank).Error)

	paymentRepo := repositories.NewPaymentRepository(db)
	service := NewEnhancedPaymentServiceWithJournal(
		db,
		*paymentRepo,
		repositories.NewContactRepository(db),
		repositories.NewCashBankRepository(db),
		repositories.NewSalesRepository(db),
		repositories.NewPurchaseRepository(db),
		NewUnifiedJournalService(db),
	)

	created, err := service.CreatePaymentWithJournal(&PaymentWithJournalRequest{
		ContactID:           customer.ID,
		CashBankID:          cashBank.ID,
		Date:                transactionDate,
		Amount:              1_937_000,
		Method:              models.PaymentMethodBankTransfer,
		UserID:              creator.ID,
		AutoCreateJournal:   true,
		SettlementTreatment: models.PaymentSettlementRounding,
		SettlementReason:    "Nilai transfer bank dibulatkan oleh customer",
		InvoiceAllocations: []AllocationItem{{
			InvoiceID: &sale.ID,
			Amount:    sale.OutstandingAmount,
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, created.Payment)
	require.Equal(t, models.PaymentStatusPendingAccounting, created.Payment.Status)
	require.Nil(t, created.Payment.JournalEntryID)

	var journalCount int64
	require.NoError(t, db.Model(&models.SSOTJournalEntry{}).Count(&journalCount).Error)
	require.Zero(t, journalCount, "rounding must not post before reviews")
	var saleBeforeReview models.Sale
	require.NoError(t, db.First(&saleBeforeReview, sale.ID).Error)
	require.InDelta(t, sale.TotalAmount, saleBeforeReview.OutstandingAmount, 0.001)
	var cashBeforeReview models.CashBank
	require.NoError(t, db.First(&cashBeforeReview, cashBank.ID).Error)
	require.Zero(t, cashBeforeReview.Balance)

	_, err = service.UpdateDraftPayment(created.Payment.ID, creator.ID, func(_ *gorm.DB, _ *models.Payment) error {
		return nil
	})
	require.ErrorContains(t, err, "other than the payment creator")

	_, err = service.UpdateDraftPayment(created.Payment.ID, financeStaff.ID, func(_ *gorm.DB, _ *models.Payment) error {
		return nil
	})
	require.ErrorContains(t, err, "active Accounting")

	accountingReviewed, err := service.UpdateDraftPayment(created.Payment.ID, accounting.ID, func(_ *gorm.DB, _ *models.Payment) error {
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, models.PaymentStatusPendingFinalReview, accountingReviewed.Status)
	require.NotNil(t, accountingReviewed.AccountingReviewedBy)
	require.Equal(t, accounting.ID, *accountingReviewed.AccountingReviewedBy)

	_, err = service.ApproveStandardPayment(created.Payment.ID, accounting.ID, false)
	require.ErrorContains(t, err, "different from its creator and Accounting reviewer")

	_, err = service.ApproveStandardPayment(created.Payment.ID, financeStaff.ID, false)
	require.ErrorContains(t, err, "active Finance Manager")

	approved, err := service.ApproveStandardPayment(created.Payment.ID, financeManager.ID, false)
	require.NoError(t, err)
	require.NotNil(t, approved.Payment)
	require.Equal(t, models.PaymentStatusCompleted, approved.Payment.Status)
	require.NotNil(t, approved.Payment.JournalEntryID)
	require.NotNil(t, approved.Settlement)
	require.Equal(t, models.PaymentSettlementRounding, approved.Settlement.Treatment)
	require.Equal(t, "OVERPAYMENT", approved.Settlement.RoundingDirection)
	require.InDelta(t, 50, approved.Settlement.RoundingAmount, 0.001)

	var updatedSale models.Sale
	require.NoError(t, db.First(&updatedSale, sale.ID).Error)
	require.InDelta(t, sale.TotalAmount, updatedSale.PaidAmount, 0.001)
	require.Zero(t, updatedSale.OutstandingAmount)
	require.Equal(t, models.SaleStatusPaid, updatedSale.Status)

	var advanceCount int64
	require.NoError(t, db.Model(&models.CustomerAdvance{}).Count(&advanceCount).Error)
	require.Zero(t, advanceCount, "approved rounding must not create customer advance")

	var journalLines []models.SSOTJournalLine
	require.NoError(t, db.Where("journal_id = ?", *approved.Payment.JournalEntryID).Order("line_number ASC").Find(&journalLines).Error)
	require.Len(t, journalLines, 3)
	requireSSOTJournalLineAmount(t, journalLines, uint64(accounts[0].ID), 1_937_000, 0)
	requireSSOTJournalLineAmount(t, journalLines, uint64(accounts[1].ID), 0, 1_936_950)
	requireSSOTJournalLineAmount(t, journalLines, uint64(accounts[3].ID), 0, 50)
}

type customerSettlementReversalFixture struct {
	db       *gorm.DB
	service  *EnhancedPaymentServiceWithJournal
	user     models.User
	sale     models.Sale
	cashBank models.CashBank
	payment  models.Payment
}

func setupCustomerSettlementReversalFixture(t *testing.T) customerSettlementReversalFixture {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.ContactAddress{},
		&models.Account{},
		&models.Settings{},
		&models.AccountingPeriod{},
		&models.CashBank{},
		&models.CashBankTransaction{},
		&models.Sale{},
		&models.SalePayment{},
		&models.SalePaymentSchedule{},
		&models.Payment{},
		&models.PaymentAllocation{},
		&models.CustomerAdvance{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
	))

	user := models.User{
		Username: "customer-settlement-reversal-user-" + strings.ReplaceAll(t.Name(), "/", "-"),
		Email:    "customer-settlement-reversal-" + strings.ReplaceAll(t.Name(), "/", "-") + "@test.local",
		Password: "secret",
		Role:     models.RoleFinanceManager,
		IsActive: true,
	}
	customer := models.Contact{
		Code:     "CUS-SETTLEMENT-REV-" + strings.ReplaceAll(t.Name(), "/", "-"),
		Name:     "Petani Reversal",
		Type:     models.ContactTypeCustomer,
		IsActive: true,
	}
	accounts := []models.Account{
		{Code: "1101-001", Name: "Bank Penerimaan", Type: models.AccountTypeAsset, IsActive: true},
		{Code: "1102-001", Name: "Piutang Usaha", Type: models.AccountTypeAsset, IsActive: true},
		{Code: "2190-001", Name: "Uang Muka Pelanggan", Type: models.AccountTypeLiability, IsActive: true},
	}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&customer).Error)
	for index := range accounts {
		require.NoError(t, db.Create(&accounts[index]).Error)
	}
	require.NoError(t, db.Create(&models.Settings{
		CompanyName:              "Customer Settlement Reversal Test",
		PaymentRoundingThreshold: 1000,
	}).Error)

	transactionDate := time.Now().Add(-time.Hour).UTC()
	sale := models.Sale{
		Code:              "SOA-SETTLEMENT-REV-" + strings.ReplaceAll(t.Name(), "/", "-"),
		InvoiceNumber:     "INV-SETTLEMENT-REV-" + strings.ReplaceAll(t.Name(), "/", "-"),
		CustomerID:        customer.ID,
		UserID:            user.ID,
		Date:              transactionDate,
		DueDate:           transactionDate.AddDate(0, 0, 30),
		Currency:          "IDR",
		ExchangeRate:      1,
		TotalAmount:       1_936_950,
		OutstandingAmount: 1_936_950,
		Status:            models.SaleStatusInvoiced,
	}
	cashBank := models.CashBank{
		Code:      "BANK-SETTLEMENT-REV-" + strings.ReplaceAll(t.Name(), "/", "-"),
		Name:      "Bank Penerimaan Reversal",
		Type:      models.CashBankTypeBank,
		AccountID: accounts[0].ID,
		Currency:  "IDR",
		IsActive:  true,
		UserID:    user.ID,
	}
	require.NoError(t, db.Create(&sale).Error)
	require.NoError(t, db.Create(&cashBank).Error)

	paymentRepo := repositories.NewPaymentRepository(db)
	service := NewEnhancedPaymentServiceWithJournal(
		db,
		*paymentRepo,
		repositories.NewContactRepository(db),
		repositories.NewCashBankRepository(db),
		repositories.NewSalesRepository(db),
		repositories.NewPurchaseRepository(db),
		NewUnifiedJournalService(db),
	)
	created, err := service.CreatePaymentWithJournal(&PaymentWithJournalRequest{
		ContactID:         customer.ID,
		CashBankID:        cashBank.ID,
		Date:              transactionDate,
		Amount:            1_937_000,
		Method:            models.PaymentMethodBankTransfer,
		UserID:            user.ID,
		AutoCreateJournal: true,
		InvoiceAllocations: []AllocationItem{{
			InvoiceID: &sale.ID,
			Amount:    sale.OutstandingAmount,
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, created.Payment)

	return customerSettlementReversalFixture{
		db:       db,
		service:  service,
		user:     user,
		sale:     sale,
		cashBank: cashBank,
		payment:  *created.Payment,
	}
}

func TestReverseCustomerSettlementIsAtomicAndCancelsAdvance(t *testing.T) {
	t.Run("reverses receipt, invoice settlement, and customer advance together", func(t *testing.T) {
		fixture := setupCustomerSettlementReversalFixture(t)

		response, err := fixture.service.ReversePayment(
			fixture.payment.ID,
			"Transfer masuk duplikat",
			time.Now().UTC(),
			fixture.user.ID,
		)
		require.NoError(t, err)
		require.NotNil(t, response)

		var payment models.Payment
		require.NoError(t, fixture.db.First(&payment, fixture.payment.ID).Error)
		require.Equal(t, models.PaymentStatusReversed, payment.Status)

		var advance models.CustomerAdvance
		require.NoError(t, fixture.db.Where("payment_id = ?", fixture.payment.ID).First(&advance).Error)
		require.Equal(t, models.CustomerAdvanceStatusCancelled, advance.Status)
		require.Zero(t, advance.RemainingAmount)

		var sale models.Sale
		require.NoError(t, fixture.db.First(&sale, fixture.sale.ID).Error)
		require.InDelta(t, 0, sale.PaidAmount, 0.001)
		require.InDelta(t, sale.TotalAmount, sale.OutstandingAmount, 0.001)
		require.Equal(t, models.SaleStatusInvoiced, sale.Status)

		var salePayment models.SalePayment
		require.NoError(t, fixture.db.Where("reference = ?", "Payment ID: "+fmt.Sprint(fixture.payment.ID)).First(&salePayment).Error)
		require.Equal(t, models.SalePaymentStatusCancelled, salePayment.Status)

		var cashBank models.CashBank
		require.NoError(t, fixture.db.First(&cashBank, fixture.cashBank.ID).Error)
		require.InDelta(t, 0, cashBank.Balance, 0.001)

		var reversalTransactions int64
		require.NoError(t, fixture.db.Model(&models.CashBankTransaction{}).
			Where("cash_bank_id = ? AND reference_type = ? AND reference_id = ?", fixture.cashBank.ID, "PAYMENT_REVERSAL", fixture.payment.ID).
			Count(&reversalTransactions).Error)
		require.Equal(t, int64(1), reversalTransactions)
	})

	t.Run("rolls back all effects when the original cash bank movement is missing", func(t *testing.T) {
		fixture := setupCustomerSettlementReversalFixture(t)
		require.NoError(t, fixture.db.Where("reference_id = ?", fixture.payment.ID).Delete(&models.CashBankTransaction{}).Error)

		var journalCountBefore int64
		require.NoError(t, fixture.db.Model(&models.SSOTJournalEntry{}).Count(&journalCountBefore).Error)
		require.Equal(t, int64(1), journalCountBefore)

		_, err := fixture.service.ReversePayment(
			fixture.payment.ID,
			"Mutasi bank asal tidak ditemukan",
			time.Now().UTC(),
			fixture.user.ID,
		)
		require.ErrorContains(t, err, "load original cash/bank movement")

		var payment models.Payment
		require.NoError(t, fixture.db.First(&payment, fixture.payment.ID).Error)
		require.Equal(t, models.PaymentStatusCompleted, payment.Status)

		var advance models.CustomerAdvance
		require.NoError(t, fixture.db.Where("payment_id = ?", fixture.payment.ID).First(&advance).Error)
		require.Equal(t, models.CustomerAdvanceStatusOpen, advance.Status)
		require.InDelta(t, 50, advance.RemainingAmount, 0.001)

		var sale models.Sale
		require.NoError(t, fixture.db.First(&sale, fixture.sale.ID).Error)
		require.InDelta(t, sale.TotalAmount, sale.PaidAmount, 0.001)
		require.Zero(t, sale.OutstandingAmount)
		require.Equal(t, models.SaleStatusPaid, sale.Status)

		var journalCountAfter int64
		require.NoError(t, fixture.db.Model(&models.SSOTJournalEntry{}).Count(&journalCountAfter).Error)
		require.Equal(t, journalCountBefore, journalCountAfter, "reversal journal must roll back with the rest of the transaction")

		var reversalTransactions int64
		require.NoError(t, fixture.db.Model(&models.CashBankTransaction{}).
			Where("cash_bank_id = ? AND reference_type = ? AND reference_id = ?", fixture.cashBank.ID, "PAYMENT_REVERSAL", fixture.payment.ID).
			Count(&reversalTransactions).Error)
		require.Zero(t, reversalTransactions)
	})
}

func requireJournalLineAmount(t *testing.T, lines []JournalLineRequest, accountID uint64, debit, credit float64) {
	t.Helper()
	for _, line := range lines {
		if line.AccountID == accountID {
			require.InDelta(t, debit, line.DebitAmount.InexactFloat64(), 0.001)
			require.InDelta(t, credit, line.CreditAmount.InexactFloat64(), 0.001)
			return
		}
	}
	t.Fatalf("journal line for account %d was not found", accountID)
}

func requireNoJournalLineForAccount(t *testing.T, lines []JournalLineRequest, accountID uint64) {
	t.Helper()
	for _, line := range lines {
		require.NotEqual(t, accountID, line.AccountID)
	}
}

func requireSSOTJournalLineAmount(t *testing.T, lines []models.SSOTJournalLine, accountID uint64, debit, credit float64) {
	t.Helper()
	for _, line := range lines {
		if line.AccountID == accountID {
			require.InDelta(t, debit, line.DebitAmount.InexactFloat64(), 0.001)
			require.InDelta(t, credit, line.CreditAmount.InexactFloat64(), 0.001)
			return
		}
	}
	t.Fatalf("SSOT journal line for account %d was not found", accountID)
}
