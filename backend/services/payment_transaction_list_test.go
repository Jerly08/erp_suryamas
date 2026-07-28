package services

import (
	"fmt"
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestListPaymentTransactionsCombinesInvoiceReceiptsAndOutgoingPayments(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Account{},
		&models.CashBank{},
		&models.Payment{},
		&models.PaymentAllocation{},
		&models.Sale{},
		&models.SalePayment{},
		&models.Purchase{},
		&models.PurchasePayment{},
		&models.SSOTJournalEntry{},
		&models.JournalEntry{},
		&models.CashBankTransaction{},
	))

	user := models.User{
		Username: "payment-transaction-user",
		Email:    "payment-transaction-user@test.local",
		Password: "secret",
		Role:     models.RoleFinance,
		IsActive: true,
	}
	customer := models.Contact{
		Code:     "CUSTOMER-PAYMENT-TRANSACTIONS",
		Name:     "Kementan Test",
		Type:     models.ContactTypeCustomer,
		IsActive: true,
	}
	vendor := models.Contact{
		Code:     "VENDOR-PAYMENT-TRANSACTIONS",
		Name:     "Vendor Test",
		Type:     models.ContactTypeVendor,
		IsActive: true,
	}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&customer).Error)
	require.NoError(t, db.Create(&vendor).Error)

	baseDate := time.Date(2026, time.July, 28, 0, 0, 0, 0, time.UTC)
	sale := models.Sale{
		Code:          "SOA-TEST-0001",
		InvoiceNumber: "INV-202607-00028",
		CustomerID:    customer.ID,
		UserID:        user.ID,
		Date:          baseDate,
		DueDate:       baseDate.AddDate(0, 0, 30),
		Status:        "INVOICED",
	}
	require.NoError(t, db.Create(&sale).Error)

	saleReceipt := models.SalePayment{
		SaleID:        sale.ID,
		Amount:        200_000,
		PaymentDate:   baseDate.AddDate(0, 0, 2),
		PaymentMethod: models.PaymentMethodBankTransfer,
		Reference:     "RCV-SALES-TEST",
		UserID:        user.ID,
		Status:        models.PaymentStatusCompleted,
	}
	require.NoError(t, db.Create(&saleReceipt).Error)
	purchase := models.Purchase{
		Code:     "PO/2026/07/00028",
		VendorID: vendor.ID,
		UserID:   user.ID,
		Date:     baseDate,
		DueDate:  baseDate.AddDate(0, 0, 30),
		Status:   "APPROVED",
	}
	require.NoError(t, db.Create(&purchase).Error)

	legacyReceipt := models.Payment{
		Code:        "RCV-LEGACY-0001",
		ContactID:   customer.ID,
		UserID:      user.ID,
		Date:        baseDate.AddDate(0, 0, 1),
		Amount:      150_000,
		Method:      models.PaymentMethodBankTransfer,
		Status:      models.PaymentStatusCompleted,
		PaymentType: models.PaymentTypeRegular,
	}
	require.NoError(t, db.Create(&legacyReceipt).Error)
	require.NoError(t, db.Create(&models.PaymentAllocation{
		PaymentID:       uint64(legacyReceipt.ID),
		InvoiceID:       &sale.ID,
		AllocatedAmount: 150_000,
	}).Error)

	nonInvoicePayment := models.Payment{
		Code:        "PAY-EXPENSE-0001",
		ContactID:   vendor.ID,
		UserID:      user.ID,
		Date:        baseDate,
		Amount:      75_000,
		Method:      models.PaymentMethodCash,
		Status:      models.PaymentStatusDraft,
		PaymentType: models.PaymentTypeExpense,
	}
	require.NoError(t, db.Create(&nonInvoicePayment).Error)

	outgoingPayment := models.Payment{
		Code:        "PAY-VENDOR-0001",
		ContactID:   vendor.ID,
		UserID:      user.ID,
		Date:        baseDate.AddDate(0, 0, 3),
		Amount:      125_000,
		Method:      models.PaymentMethodBankTransfer,
		Status:      models.PaymentStatusDraft,
		PaymentType: models.PaymentTypeRegular,
	}
	require.NoError(t, db.Create(&outgoingPayment).Error)
	require.NoError(t, db.Create(&models.PaymentAllocation{
		PaymentID:       uint64(outgoingPayment.ID),
		BillID:          &purchase.ID,
		AllocatedAmount: 125_000,
	}).Error)

	service := &EnhancedPaymentServiceWithJournal{db: db}
	all, err := service.ListPaymentTransactions(PaymentTransactionListFilter{Page: 1, Limit: 10})
	require.NoError(t, err)
	require.Equal(t, int64(4), all.Total)
	require.Len(t, all.Data, 4)

	itemsByRecord := make(map[string]PaymentTransactionListItem)
	for _, item := range all.Data {
		itemsByRecord[item.RowID] = item
	}

	directReceipt := itemsByRecord["SALE_PAYMENT-"+uintToString(saleReceipt.ID)]
	require.Equal(t, PaymentTransactionKindSalesReceipt, directReceipt.TransactionKind)
	require.Equal(t, "Penjualan - INV-202607-00028", directReceipt.SourceLabel)
	require.Equal(t, sale.ID, *directReceipt.SaleID)
	require.True(t, directReceipt.IsInvoiceReceipt)
	require.True(t, directReceipt.IsReadOnly)

	legacyItem := itemsByRecord["PAYMENT-"+uintToString(legacyReceipt.ID)]
	require.Equal(t, PaymentTransactionKindSalesReceipt, legacyItem.TransactionKind)
	require.Equal(t, "Penjualan - INV-202607-00028", legacyItem.SourceLabel)
	require.True(t, legacyItem.IsInvoiceReceipt)

	nonInvoiceItem := itemsByRecord["PAYMENT-"+uintToString(nonInvoicePayment.ID)]
	require.Equal(t, PaymentTransactionKindNonInvoice, nonInvoiceItem.TransactionKind)
	require.Equal(t, "COA / Non-Invoice", nonInvoiceItem.SourceLabel)

	outgoingItem := itemsByRecord["PAYMENT-"+uintToString(outgoingPayment.ID)]
	require.Equal(t, PaymentTransactionKindOutgoingPayment, outgoingItem.TransactionKind)
	require.Equal(t, "Pembelian - PO/2026/07/00028", outgoingItem.SourceLabel)
	require.Equal(t, "PO/2026/07/00028", outgoingItem.SourceReference)

	receipts, err := service.ListPaymentTransactions(PaymentTransactionListFilter{
		Page:  1,
		Limit: 10,
		Kind:  "RECEIPT",
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), receipts.Total)
	require.Len(t, receipts.Data, 2)

	nonInvoices, err := service.ListPaymentTransactions(PaymentTransactionListFilter{
		Page:  1,
		Limit: 10,
		Kind:  "NON_INVOICE",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), nonInvoices.Total)
	require.Len(t, nonInvoices.Data, 1)
	require.Equal(t, "PAY-EXPENSE-0001", nonInvoices.Data[0].Code)

	outgoing, err := service.ListPaymentTransactions(PaymentTransactionListFilter{
		Page:  1,
		Limit: 10,
		Kind:  "OUTGOING",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), outgoing.Total)
	require.Len(t, outgoing.Data, 1)
	require.Equal(t, "PAY-VENDOR-0001", outgoing.Data[0].Code)
}

func uintToString(value uint) string {
	return fmt.Sprintf("%d", value)
}
