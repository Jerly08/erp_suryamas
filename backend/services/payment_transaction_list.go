package services

import (
	"fmt"
	"math"
	"strings"
	"time"

	"app-sistem-akuntansi/models"
)

const (
	PaymentTransactionKindSalesReceipt    = "SALES_RECEIPT"
	PaymentTransactionKindCustomerReceipt = "CUSTOMER_RECEIPT"
	PaymentTransactionKindOutgoingPayment = "OUTGOING_PAYMENT"
	PaymentTransactionKindNonInvoice      = "NON_INVOICE"
)

// PaymentTransactionListFilter is the read-only filter for the unified payment
// activity list. It deliberately does not change how payments are created or posted.
type PaymentTransactionListFilter struct {
	Page      int
	Limit     int
	ContactID uint
	UserID    uint
	Status    string
	Method    string
	Search    string
	StartDate time.Time
	EndDate   time.Time
	Kind      string
}

// PaymentTransactionListItem keeps the existing payment response shape while
// adding enough context to show the source transaction safely in one list.
type PaymentTransactionListItem struct {
	models.Payment
	RowID            string `json:"row_id"`
	RecordType       string `json:"record_type"`
	RecordID         uint   `json:"record_id"`
	TransactionKind  string `json:"transaction_kind"`
	SourceLabel      string `json:"source_label"`
	SourceReference  string `json:"source_reference,omitempty"`
	SaleID           *uint  `json:"sale_id,omitempty"`
	IsInvoiceReceipt bool   `json:"is_invoice_receipt"`
	IsReadOnly       bool   `json:"is_read_only"`
}

type PaymentTransactionListResult struct {
	Data       []PaymentTransactionListItem `json:"data"`
	Total      int64                        `json:"total"`
	Page       int                          `json:"page"`
	Limit      int                          `json:"limit"`
	TotalPages int                          `json:"total_pages"`
}

type paymentTransactionIndexRow struct {
	RecordType       string    `gorm:"column:record_type"`
	RecordID         uint      `gorm:"column:record_id"`
	UserID           uint      `gorm:"column:user_id"`
	TransactionDate  time.Time `gorm:"column:transaction_date"`
	Status           string    `gorm:"column:status"`
	Method           string    `gorm:"column:method"`
	Code             string    `gorm:"column:code"`
	ContactID        uint      `gorm:"column:contact_id"`
	ContactName      string    `gorm:"column:contact_name"`
	ContactType      string    `gorm:"column:contact_type"`
	Reference        string    `gorm:"column:reference"`
	TransactionKind  string    `gorm:"column:transaction_kind"`
	SourceLabel      string    `gorm:"column:source_label"`
	SourceReference  string    `gorm:"column:source_reference"`
	SaleID           uint      `gorm:"column:sale_id"`
	IsInvoiceReceipt bool      `gorm:"column:is_invoice_receipt"`
}

// paymentTransactionUnionSQL returns a neutral, paginated index over two
// distinct ledgers: generic payments and invoice receipts from sale_payments.
// The latter stays a separate model so the list cannot create duplicate cash
// movements or duplicate payment journals.
const paymentTransactionUnionSQL = `
SELECT
	'PAYMENT' AS record_type,
	p.id AS record_id,
	p.user_id AS user_id,
	p.date AS transaction_date,
	p.status AS status,
	p.method AS method,
	p.code AS code,
	p.contact_id AS contact_id,
	COALESCE(c.name, '') AS contact_name,
	COALESCE(c.type, '') AS contact_type,
	COALESCE(p.reference, '') AS reference,
	CASE
		WHEN COALESCE(p.payment_type, '') = 'EXPENSE' THEN 'NON_INVOICE'
		WHEN c.type = 'CUSTOMER' AND EXISTS (
			SELECT 1
			FROM payment_allocations pa
			WHERE pa.payment_id = p.id AND pa.invoice_id IS NOT NULL
		) THEN 'SALES_RECEIPT'
		WHEN c.type = 'CUSTOMER' THEN 'CUSTOMER_RECEIPT'
		ELSE 'OUTGOING_PAYMENT'
	END AS transaction_kind,
	CASE
		WHEN COALESCE(p.payment_type, '') = 'EXPENSE' THEN 'COA / Non-Invoice'
		WHEN c.type = 'CUSTOMER' AND EXISTS (
			SELECT 1
			FROM payment_allocations pa
			WHERE pa.payment_id = p.id AND pa.invoice_id IS NOT NULL
		) THEN 'Penjualan - ' || COALESCE(
			NULLIF((
				SELECT s.invoice_number
				FROM payment_allocations pa
				JOIN sales s ON s.id = pa.invoice_id AND s.deleted_at IS NULL
				WHERE pa.payment_id = p.id AND pa.invoice_id IS NOT NULL
				ORDER BY pa.id ASC
				LIMIT 1
			), ''),
			(
				SELECT s.code
				FROM payment_allocations pa
				JOIN sales s ON s.id = pa.invoice_id AND s.deleted_at IS NULL
				WHERE pa.payment_id = p.id AND pa.invoice_id IS NOT NULL
				ORDER BY pa.id ASC
				LIMIT 1
			),
			'Invoice'
		)
		WHEN EXISTS (
			SELECT 1
			FROM payment_allocations pa
			WHERE pa.payment_id = p.id AND pa.purchase_payment_term_id IS NOT NULL
		) THEN 'Pembelian - ' || COALESCE(
			(
				SELECT pu.code || ' | Termin ' || CAST(ppt.sequence AS TEXT)
				FROM payment_allocations pa
				JOIN purchase_payment_terms ppt ON ppt.id = pa.purchase_payment_term_id AND ppt.deleted_at IS NULL
				JOIN purchases pu ON pu.id = pa.bill_id AND pu.deleted_at IS NULL
				WHERE pa.payment_id = p.id AND pa.purchase_payment_term_id IS NOT NULL
				ORDER BY pa.id ASC
				LIMIT 1
			),
			'PO / Termin'
		)
		WHEN EXISTS (
			SELECT 1
			FROM payment_allocations pa
			WHERE pa.payment_id = p.id AND pa.bill_id IS NOT NULL
		) THEN 'Pembelian - ' || COALESCE(
			NULLIF((
				SELECT pu.code
				FROM payment_allocations pa
				JOIN purchases pu ON pu.id = pa.bill_id AND pu.deleted_at IS NULL
				WHERE pa.payment_id = p.id AND pa.bill_id IS NOT NULL
				ORDER BY pa.id ASC
				LIMIT 1
			), ''),
			'PO'
		)
		WHEN c.type = 'CUSTOMER' THEN 'Penerimaan pelanggan tanpa invoice'
		WHEN COALESCE(p.payment_type, '') IN ('TAX_PPN', 'TAX_PPN_INPUT', 'TAX_PPN_OUTPUT') THEN 'Setor Pajak'
		ELSE 'Pembayaran keluar'
	END AS source_label,
	CASE
		WHEN c.type = 'CUSTOMER' AND EXISTS (
			SELECT 1
			FROM payment_allocations pa
			WHERE pa.payment_id = p.id AND pa.invoice_id IS NOT NULL
		) THEN COALESCE(
			NULLIF((
				SELECT s.invoice_number
				FROM payment_allocations pa
				JOIN sales s ON s.id = pa.invoice_id AND s.deleted_at IS NULL
				WHERE pa.payment_id = p.id AND pa.invoice_id IS NOT NULL
				ORDER BY pa.id ASC
				LIMIT 1
			), ''),
			(
				SELECT s.code
				FROM payment_allocations pa
				JOIN sales s ON s.id = pa.invoice_id AND s.deleted_at IS NULL
				WHERE pa.payment_id = p.id AND pa.invoice_id IS NOT NULL
				ORDER BY pa.id ASC
				LIMIT 1
			),
			''
		)
		WHEN EXISTS (
			SELECT 1
			FROM payment_allocations pa
			WHERE pa.payment_id = p.id AND pa.purchase_payment_term_id IS NOT NULL
		) THEN ''
		WHEN EXISTS (
			SELECT 1
			FROM payment_allocations pa
			WHERE pa.payment_id = p.id AND pa.bill_id IS NOT NULL
		) THEN COALESCE((
			SELECT pu.code
			FROM payment_allocations pa
			JOIN purchases pu ON pu.id = pa.bill_id AND pu.deleted_at IS NULL
			WHERE pa.payment_id = p.id AND pa.bill_id IS NOT NULL
			ORDER BY pa.id ASC
				LIMIT 1
			), '')
		ELSE ''
	END AS source_reference,
	COALESCE((
		SELECT pa.invoice_id
		FROM payment_allocations pa
		WHERE pa.payment_id = p.id AND pa.invoice_id IS NOT NULL
		ORDER BY pa.id ASC
		LIMIT 1
	), 0) AS sale_id,
	CASE
		WHEN c.type = 'CUSTOMER' AND EXISTS (
			SELECT 1
			FROM payment_allocations pa
			WHERE pa.payment_id = p.id AND pa.invoice_id IS NOT NULL
		) THEN 1
		ELSE 0
	END AS is_invoice_receipt
FROM payments p
LEFT JOIN contacts c ON c.id = p.contact_id AND c.deleted_at IS NULL
WHERE p.deleted_at IS NULL

UNION ALL

SELECT
	'SALE_PAYMENT' AS record_type,
	sp.id AS record_id,
	sp.user_id AS user_id,
	sp.payment_date AS transaction_date,
	COALESCE(NULLIF(sp.status, ''), 'COMPLETED') AS status,
	sp.payment_method AS method,
	'RCV-SALES-' || CAST(sp.id AS TEXT) AS code,
	s.customer_id AS contact_id,
	COALESCE(c.name, '') AS contact_name,
	COALESCE(c.type, 'CUSTOMER') AS contact_type,
	COALESCE(sp.reference, '') AS reference,
	'SALES_RECEIPT' AS transaction_kind,
	'Penjualan - ' || COALESCE(NULLIF(s.invoice_number, ''), s.code) AS source_label,
	COALESCE(NULLIF(s.invoice_number, ''), s.code) AS source_reference,
	sp.sale_id AS sale_id,
	1 AS is_invoice_receipt
FROM sale_payments sp
JOIN sales s ON s.id = sp.sale_id AND s.deleted_at IS NULL
LEFT JOIN contacts c ON c.id = s.customer_id AND c.deleted_at IS NULL
WHERE sp.deleted_at IS NULL`

// ListPaymentTransactions reads payment activity from its real sources. It is
// intentionally a read model: no record is created, converted, or re-posted.
func (eps *EnhancedPaymentServiceWithJournal) ListPaymentTransactions(filter PaymentTransactionListFilter) (*PaymentTransactionListResult, error) {
	if eps == nil || eps.db == nil {
		return nil, fmt.Errorf("payment transaction list is not configured")
	}

	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.Limit <= 0 {
		filter.Limit = 10
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	whereSQL, args := buildPaymentTransactionWhereClause(filter)
	indexedQuery := "SELECT * FROM (" + paymentTransactionUnionSQL + ") AS payment_transactions"
	if whereSQL != "" {
		indexedQuery += " WHERE " + whereSQL
	}

	var total int64
	if err := eps.db.Raw("SELECT COUNT(*) FROM ("+indexedQuery+") AS counted_transactions", args...).Scan(&total).Error; err != nil {
		return nil, fmt.Errorf("count payment transactions: %w", err)
	}

	rows := make([]paymentTransactionIndexRow, 0)
	offset := (filter.Page - 1) * filter.Limit
	listArgs := append(append([]interface{}{}, args...), filter.Limit, offset)
	if err := eps.db.Raw(indexedQuery+" ORDER BY transaction_date DESC, record_type ASC, record_id DESC LIMIT ? OFFSET ?", listArgs...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list payment transactions: %w", err)
	}

	items, err := eps.hydratePaymentTransactions(rows)
	if err != nil {
		return nil, err
	}

	totalPages := int(math.Ceil(float64(total) / float64(filter.Limit)))
	return &PaymentTransactionListResult{
		Data:       items,
		Total:      total,
		Page:       filter.Page,
		Limit:      filter.Limit,
		TotalPages: totalPages,
	}, nil
}

func buildPaymentTransactionWhereClause(filter PaymentTransactionListFilter) (string, []interface{}) {
	clauses := make([]string, 0, 8)
	args := make([]interface{}, 0, 10)

	if filter.ContactID > 0 {
		clauses = append(clauses, "contact_id = ?")
		args = append(args, filter.ContactID)
	}
	if filter.UserID > 0 {
		clauses = append(clauses, "user_id = ?")
		args = append(args, filter.UserID)
	}
	if status := strings.ToUpper(strings.TrimSpace(filter.Status)); status != "" {
		clauses = append(clauses, "UPPER(status) = ?")
		args = append(args, status)
	}
	if method := strings.ToUpper(strings.TrimSpace(filter.Method)); method != "" {
		clauses = append(clauses, "UPPER(method) = ?")
		args = append(args, method)
	}
	if !filter.StartDate.IsZero() {
		clauses = append(clauses, "transaction_date >= ?")
		args = append(args, filter.StartDate)
	}
	if !filter.EndDate.IsZero() {
		clauses = append(clauses, "transaction_date < ?")
		args = append(args, filter.EndDate.AddDate(0, 0, 1))
	}

	switch strings.ToUpper(strings.TrimSpace(filter.Kind)) {
	case "RECEIPT", "PENERIMAAN":
		clauses = append(clauses, "transaction_kind IN (?, ?)")
		args = append(args, PaymentTransactionKindSalesReceipt, PaymentTransactionKindCustomerReceipt)
	case "OUTGOING", "PAYMENT", "PEMBAYARAN_KELUAR":
		clauses = append(clauses, "transaction_kind = ?")
		args = append(args, PaymentTransactionKindOutgoingPayment)
	case "NON_INVOICE":
		clauses = append(clauses, "transaction_kind = ?")
		args = append(args, PaymentTransactionKindNonInvoice)
	}

	if search := strings.ToLower(strings.TrimSpace(filter.Search)); search != "" {
		pattern := "%" + search + "%"
		clauses = append(clauses, "(LOWER(code) LIKE ? OR LOWER(contact_name) LIKE ? OR LOWER(reference) LIKE ? OR LOWER(source_label) LIKE ? OR LOWER(source_reference) LIKE ?)")
		args = append(args, pattern, pattern, pattern, pattern, pattern)
	}

	return strings.Join(clauses, " AND "), args
}

func (eps *EnhancedPaymentServiceWithJournal) hydratePaymentTransactions(rows []paymentTransactionIndexRow) ([]PaymentTransactionListItem, error) {
	paymentIDs := make([]uint, 0)
	salePaymentIDs := make([]uint, 0)
	for _, row := range rows {
		switch row.RecordType {
		case "PAYMENT":
			paymentIDs = append(paymentIDs, row.RecordID)
		case "SALE_PAYMENT":
			salePaymentIDs = append(salePaymentIDs, row.RecordID)
		}
	}

	paymentsByID, err := eps.loadPaymentTransactionPayments(paymentIDs)
	if err != nil {
		return nil, err
	}
	salePaymentsByID, err := eps.loadPaymentTransactionSalePayments(salePaymentIDs)
	if err != nil {
		return nil, err
	}

	items := make([]PaymentTransactionListItem, 0, len(rows))
	for _, row := range rows {
		var payment models.Payment
		switch row.RecordType {
		case "PAYMENT":
			loaded, ok := paymentsByID[row.RecordID]
			if !ok {
				continue
			}
			payment = loaded
		case "SALE_PAYMENT":
			loaded, ok := salePaymentsByID[row.RecordID]
			if !ok {
				continue
			}
			payment = paymentFromSalePayment(loaded)
		default:
			continue
		}

		var saleID *uint
		if row.SaleID > 0 {
			value := row.SaleID
			saleID = &value
		}

		items = append(items, PaymentTransactionListItem{
			Payment:          payment,
			RowID:            row.RecordType + "-" + fmt.Sprintf("%d", row.RecordID),
			RecordType:       row.RecordType,
			RecordID:         row.RecordID,
			TransactionKind:  row.TransactionKind,
			SourceLabel:      row.SourceLabel,
			SourceReference:  row.SourceReference,
			SaleID:           saleID,
			IsInvoiceReceipt: row.IsInvoiceReceipt,
			IsReadOnly:       row.IsInvoiceReceipt,
		})
	}

	return items, nil
}

func (eps *EnhancedPaymentServiceWithJournal) loadPaymentTransactionPayments(ids []uint) (map[uint]models.Payment, error) {
	result := make(map[uint]models.Payment, len(ids))
	if len(ids) == 0 {
		return result, nil
	}

	var payments []models.Payment
	err := eps.db.
		Preload("Contact").
		Preload("User").
		Preload("FinanceManagerApprover").
		Preload("DirectorApprover").
		Preload("AccountingReviewer").
		Preload("Approver").
		Preload("Rejector").
		Preload("ExpenseAccount").
		Preload("CashBank").
		Preload("CashBank.Account").
		Where("id IN ?", ids).
		Find(&payments).Error
	if err != nil {
		return nil, fmt.Errorf("load generic payments: %w", err)
	}
	if err := eps.markLegacyNonInvoiceEligibilityForPayments(payments); err != nil {
		return nil, fmt.Errorf("evaluate legacy payment eligibility: %w", err)
	}
	for _, payment := range payments {
		result[payment.ID] = payment
	}
	return result, nil
}

func (eps *EnhancedPaymentServiceWithJournal) loadPaymentTransactionSalePayments(ids []uint) (map[uint]models.SalePayment, error) {
	result := make(map[uint]models.SalePayment, len(ids))
	if len(ids) == 0 {
		return result, nil
	}

	var payments []models.SalePayment
	err := eps.db.
		Preload("Sale.Customer").
		Preload("User").
		Preload("CashBank").
		Preload("CashBank.Account").
		Where("id IN ?", ids).
		Find(&payments).Error
	if err != nil {
		return nil, fmt.Errorf("load sales receipts: %w", err)
	}
	for _, payment := range payments {
		result[payment.ID] = payment
	}
	return result, nil
}

func paymentFromSalePayment(payment models.SalePayment) models.Payment {
	status := strings.TrimSpace(payment.Status)
	if status == "" {
		status = models.PaymentStatusCompleted
	}

	return models.Payment{
		ID:                    payment.ID,
		Code:                  fmt.Sprintf("RCV-SALES-%d", payment.ID),
		ContactID:             payment.Sale.CustomerID,
		UserID:                payment.UserID,
		Date:                  payment.PaymentDate,
		Amount:                payment.Amount,
		NativeAmount:          payment.NativeAmount,
		Currency:              payment.Currency,
		ExchangeRate:          payment.ExchangeRate,
		BookAmountIDR:         payment.BookAmountIDR,
		ForexGainLossAmount:   payment.ForexGainLossAmount,
		ForexGainLossType:     payment.ForexGainLossType,
		Method:                payment.PaymentMethod,
		Reference:             payment.Reference,
		Status:                status,
		PaymentType:           "SALES_RECEIPT",
		CashBankID:            payment.CashBankID,
		CashBankCode:          payment.CashBankCode,
		CashBankName:          payment.CashBankName,
		CashBankCurrency:      payment.CashBankCurrency,
		CashBankGLAccountID:   payment.CashBankGLAccountID,
		CashBankGLAccountCode: payment.CashBankGLAccountCode,
		CashBankGLAccountName: payment.CashBankGLAccountName,
		Notes:                 payment.Notes,
		CreatedAt:             payment.CreatedAt,
		UpdatedAt:             payment.UpdatedAt,
		Contact:               payment.Sale.Customer,
		User:                  payment.User,
		CashBank:              payment.CashBank,
	}
}
