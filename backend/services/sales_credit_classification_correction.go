package services

import (
	"fmt"
	"math"
	"strings"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const salesCreditCorrectionTolerance = 0.01

const salesCreditCorrectionCashBankReferenceType = "SALES_CREDIT_RECLASSIFICATION"

// Legacy customer receipts were persisted with more than one reference type.
// Keep this list narrow: each value represents an inbound payment linked to the
// selected Payment record, so unrelated cash/bank movements remain ineligible.
var salesCreditCorrectionReceiptReferenceTypes = []string{
	"PAYMENT",
	"PAYMENT_RECEIVED",
	"SALES_PAYMENT",
}

// LegacySalesCreditPaymentCandidate is an existing customer receipt that can
// safely be linked to a legacy credit sale. It is intentionally narrow so the
// correction flow cannot create a second customer receipt.
type LegacySalesCreditPaymentCandidate struct {
	ID               uint      `json:"id"`
	Code             string    `json:"code"`
	Date             time.Time `json:"date"`
	Amount           float64   `json:"amount"`
	Reference        string    `json:"reference"`
	CashBankCode     string    `json:"cash_bank_code"`
	CashBankName     string    `json:"cash_bank_name"`
	CashBankCurrency string    `json:"cash_bank_currency"`
}

type legacySalesCreditCorrectionData struct {
	sale                        models.Sale
	payment                     models.Payment
	originalJournal             models.SSOTJournalEntry
	paymentJournal              models.SSOTJournalEntry
	originalCashBank            models.CashBank
	paymentCashBank             models.CashBank
	originalCashBankTransaction models.CashBankTransaction
	paymentCashBankTransaction  models.CashBankTransaction
}

func (s *PostedTransactionCorrectionService) resolveLegacySalesCreditCorrectionTarget(
	tx *gorm.DB,
	saleID uint,
	linkedPaymentID *uint,
) (*postedCorrectionTarget, error) {
	if linkedPaymentID == nil || *linkedPaymentID == 0 {
		return nil, fmt.Errorf("pilih pembayaran pelanggan yang akan ditautkan ke invoice")
	}

	data, err := s.loadLegacySalesCreditCorrectionData(tx, saleID, *linkedPaymentID, 0, false)
	if err != nil {
		return nil, err
	}

	snapshot, err := marshalCorrectionSnapshot(map[string]interface{}{
		"sale_code":                               data.sale.Code,
		"invoice_number":                          data.sale.InvoiceNumber,
		"sale_date":                               data.sale.Date,
		"payment_terms":                           data.sale.PaymentTerms,
		"payment_method_type":                     data.sale.PaymentMethodType,
		"customer":                                data.sale.Customer.Name,
		"total_amount":                            data.sale.TotalAmount,
		"original_journal_id":                     data.originalJournal.ID,
		"original_cash_bank":                      data.originalCashBank.Name,
		"original_cash_bank_transaction_id":       data.originalCashBankTransaction.ID,
		"linked_payment_id":                       data.payment.ID,
		"linked_payment_code":                     data.payment.Code,
		"linked_payment_date":                     data.payment.Date,
		"linked_payment_amount":                   data.payment.Amount,
		"linked_payment_journal":                  data.paymentJournal.ID,
		"linked_payment_cash_bank_transaction_id": data.paymentCashBankTransaction.ID,
	})
	if err != nil {
		return nil, err
	}

	return &postedCorrectionTarget{
		targetType:        models.PostedTransactionCorrectionTargetSale,
		targetCode:        saleCorrectionTargetCode(&data.sale),
		saleID:            &data.sale.ID,
		linkedPaymentID:   &data.payment.ID,
		originalJournalID: data.originalJournal.ID,
		sourceSnapshot:    snapshot,
	}, nil
}

func (s *PostedTransactionCorrectionService) ListLegacySalesCreditPaymentCandidates(
	saleID uint,
	userID uint,
	role string,
) ([]LegacySalesCreditPaymentCandidate, error) {
	if !canRequestPostedTransactionCorrection(role) && !canReviewPostedTransactionCorrectionAccounting(role) && !canReviewPostedTransactionCorrectionFinanceManager(role) {
		return nil, ErrPostedTransactionCorrectionDenied
	}

	var sale models.Sale
	if err := s.db.First(&sale, saleID).Error; err != nil {
		if errorsIsRecordNotFound(err) {
			return nil, fmt.Errorf("penjualan tidak ditemukan")
		}
		return nil, err
	}

	var payments []models.Payment
	if err := s.db.
		Where("contact_id = ? AND status = ? AND deleted_at IS NULL", sale.CustomerID, models.PaymentStatusCompleted).
		Order("date ASC, id ASC").
		Find(&payments).Error; err != nil {
		return nil, err
	}

	result := make([]LegacySalesCreditPaymentCandidate, 0, len(payments))
	for _, payment := range payments {
		data, err := s.loadLegacySalesCreditCorrectionData(s.db, saleID, payment.ID, 0, false)
		if err != nil {
			continue
		}
		result = append(result, LegacySalesCreditPaymentCandidate{
			ID:               data.payment.ID,
			Code:             data.payment.Code,
			Date:             data.payment.Date,
			Amount:           data.payment.Amount,
			Reference:        data.payment.Reference,
			CashBankCode:     data.paymentCashBank.Code,
			CashBankName:     data.paymentCashBank.Name,
			CashBankCurrency: data.paymentCashBank.Currency,
		})
	}

	return result, nil
}

// ExecuteSalesCreditCorrection creates exactly one system-generated adjustment:
// debit receivable and credit the bank/cash account that was incorrectly debited
// when the legacy credit invoice was created. It also creates one audit-only
// cash/bank reversal record to keep the operational cash/bank balance aligned
// with the correcting journal. It never creates a second customer receipt.
func (s *PostedTransactionCorrectionService) ExecuteSalesCreditCorrection(
	id uint,
	userID uint,
	role string,
) (*models.PostedTransactionCorrection, error) {
	if !canCompletePostedTransactionCorrection(role) {
		return nil, ErrPostedTransactionCorrectionDenied
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var correction models.PostedTransactionCorrection
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&correction, id).Error; err != nil {
			return normalizePostedTransactionCorrectionNotFound(err)
		}
		if correction.Status != models.PostedTransactionCorrectionStatusApprovedForExecution {
			return fmt.Errorf("koreksi ini belum disetujui untuk dieksekusi")
		}
		if correction.TargetType != models.PostedTransactionCorrectionTargetSale ||
			correction.CorrectionType != models.PostedTransactionCorrectionTypeSalesCreditClassification ||
			correction.SaleID == nil || correction.LinkedPaymentID == nil || correction.ProposedAccountID == nil {
			return fmt.Errorf("koreksi ini bukan koreksi reklasifikasi penjualan kredit yang lengkap")
		}
		if correction.AccountingResolution != models.PostedTransactionCorrectionResolutionJournalReclass {
			return fmt.Errorf("rencana Accounting untuk koreksi ini tidak sesuai")
		}

		data, err := s.loadLegacySalesCreditCorrectionData(tx, *correction.SaleID, *correction.LinkedPaymentID, correction.ID, true)
		if err != nil {
			return err
		}
		if data.originalJournal.ID != correction.OriginalJournalEntryID {
			return fmt.Errorf("jurnal asal penjualan berubah dari snapshot koreksi; buat pengajuan baru setelah Accounting meninjau ulang")
		}
		if err := s.validateSalesCreditClassificationAccount(tx, correction, correction.ProposedAccountID); err != nil {
			return err
		}
		if !journalHasCreditLine(data.paymentJournal, uint64(*correction.ProposedAccountID), data.sale.TotalAmount) {
			return fmt.Errorf("pembayaran pelanggan yang dipilih tidak mencatat kredit ke COA piutang yang dipilih")
		}

		entryDate := time.Now().UTC()
		if correction.RequestedDate != nil {
			entryDate = *correction.RequestedDate
		}
		amount := decimal.NewFromFloat(data.sale.TotalAmount).Round(2)
		amountFloat := amount.InexactFloat64()

		// Cash/bank history is the operational source for the Cash & Bank page.
		// Rebuild it before calculating the audit reversal, then rebuild it again
		// after the reversal so it matches the correcting GL journal exactly.
		cashBankGuard := NewCashBankCurrencyGuard(s.db)
		if err := cashBankGuard.EnforceAfterTransaction(tx, data.originalCashBank.ID, data.paymentCashBank.ID); err != nil {
			return fmt.Errorf("gagal menyelaraskan saldo kas/bank sebelum koreksi: %w", err)
		}

		var correctionCashBank models.CashBank
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&correctionCashBank, data.originalCashBank.ID).Error; err != nil {
			return fmt.Errorf("akun kas/bank asal tidak ditemukan saat koreksi: %w", err)
		}
		if correctionCashBank.AccountID != data.originalCashBank.AccountID {
			return fmt.Errorf("COA akun kas/bank asal berubah saat koreksi; buat pengajuan baru setelah Accounting meninjau ulang")
		}
		newCashBankBalance := correctionCashBank.Balance - amountFloat
		newCashBankBookBalance := correctionCashBank.EffectiveBookBalanceIDR() - amountFloat
		if newCashBankBalance < -salesCreditCorrectionTolerance || newCashBankBookBalance < -salesCreditCorrectionTolerance {
			return fmt.Errorf("koreksi akan membuat saldo kas/bank asal negatif; gunakan koreksi manual setelah rekonsiliasi bank")
		}

		adjustmentOf := data.originalJournal.ID
		journal, err := NewUnifiedJournalService(tx).CreateJournalEntryWithTx(tx, &JournalEntryRequest{
			EntryDate:              entryDate,
			Reference:              fmt.Sprintf("KOREKSI-PIUTANG-%s", saleCorrectionTargetCode(&data.sale)),
			Description:            fmt.Sprintf("Reklasifikasi penerimaan awal menjadi piutang usaha - %s", saleCorrectionTargetCode(&data.sale)),
			Notes:                  fmt.Sprintf("Koreksi transaksi terposting #%d. Pembayaran %s ditautkan ke invoice; mutasi kas/bank awal yang salah dibalik sebagai catatan audit tanpa membuat penerimaan baru.", correction.ID, data.payment.Code),
			CreatedBy:              uint64(userID),
			SourceType:             models.SSOTSourceTypeAdjustment,
			SourceID:               uint64(data.sale.ID),
			AdjustmentOf:           &adjustmentOf,
			AdjustmentCategoryCode: models.JournalAdjustmentCategorySalesRevenueRecognition,
			AutoPost:               true,
			SystemGenerated:        true,
			Lines: []JournalLineRequest{
				{
					AccountID:    uint64(*correction.ProposedAccountID),
					DebitAmount:  amount,
					CreditAmount: decimal.Zero,
					Description:  fmt.Sprintf("Piutang usaha %s", saleCorrectionTargetCode(&data.sale)),
				},
				{
					AccountID:    uint64(correctionCashBank.AccountID),
					DebitAmount:  decimal.Zero,
					CreditAmount: amount,
					Description:  fmt.Sprintf("Reklasifikasi saldo %s", data.originalCashBank.Name),
				},
			},
		})
		if err != nil {
			return fmt.Errorf("gagal membuat jurnal reklasifikasi piutang: %w", err)
		}
		if err := tx.Model(journal).Update("source_code", data.sale.Code).Error; err != nil {
			return err
		}

		cashBankTransaction := models.CashBankTransaction{
			CashBankID:      correctionCashBank.ID,
			ReferenceType:   salesCreditCorrectionCashBankReferenceType,
			ReferenceID:     correction.ID,
			Amount:          -amountFloat,
			Currency:        normalizeCashBankCurrency(correctionCashBank.Currency),
			ExchangeRate:    1,
			BookAmountIDR:   -amountFloat,
			BalanceAfter:    newCashBankBalance,
			TransactionDate: entryDate,
			Notes:           fmt.Sprintf("Koreksi audit #%d untuk %s: membalik penerimaan kas/bank awal yang salah tercatat sebagai penjualan tunai. Tidak ada arus kas baru.", correction.ID, saleCorrectionTargetCode(&data.sale)),
		}
		if err := tx.Create(&cashBankTransaction).Error; err != nil {
			return fmt.Errorf("gagal membuat mutasi koreksi kas/bank: %w", err)
		}
		if err := cashBankGuard.EnforceAfterTransaction(tx, correctionCashBank.ID, data.paymentCashBank.ID); err != nil {
			return fmt.Errorf("gagal menyelaraskan saldo kas/bank setelah koreksi: %w", err)
		}

		allocation := models.PaymentAllocation{
			PaymentID:       uint64(data.payment.ID),
			InvoiceID:       &data.sale.ID,
			AllocatedAmount: data.sale.TotalAmount,
		}
		if err := tx.Create(&allocation).Error; err != nil {
			return fmt.Errorf("gagal menautkan pembayaran pelanggan ke invoice: %w", err)
		}

		now := time.Now().UTC()
		completionNotes := fmt.Sprintf("Sistem membuat jurnal #%d: debit COA #%d dan kredit %s sebesar %.2f, serta mutasi Kas/Bank audit #%d untuk membalik penerimaan awal yang salah. Pembayaran %s ditautkan ke invoice tanpa penerimaan atau pengeluaran uang baru.", journal.ID, *correction.ProposedAccountID, correctionCashBank.Code, data.sale.TotalAmount, cashBankTransaction.ID, data.payment.Code)
		if err := tx.Model(&correction).Updates(map[string]interface{}{
			"status":                      models.PostedTransactionCorrectionStatusCompleted,
			"completion_notes":            completionNotes,
			"completion_journal_entry_id": journal.ID,
			"completed_by":                userID,
			"completed_at":                now,
		}).Error; err != nil {
			return err
		}
		return s.createEvent(tx, correction.ID, models.PostedTransactionCorrectionEventSalesCreditExecuted, completionNotes, userID, role)
	})
	if err != nil {
		return nil, err
	}

	return s.GetByID(id, userID, role)
}

func (s *PostedTransactionCorrectionService) validateSalesCreditClassificationAccount(
	tx *gorm.DB,
	correction models.PostedTransactionCorrection,
	accountID *uint,
) error {
	if accountID == nil || *accountID == 0 {
		return fmt.Errorf("Accounting wajib memilih COA piutang usaha untuk koreksi penjualan kredit")
	}
	var account models.Account
	if err := tx.First(&account, *accountID).Error; err != nil {
		if errorsIsRecordNotFound(err) {
			return fmt.Errorf("COA piutang yang dipilih tidak ditemukan")
		}
		return err
	}
	if !account.IsActive || account.IsHeader || !strings.EqualFold(account.Type, models.AccountTypeAsset) {
		return fmt.Errorf("COA koreksi harus akun aset/piutang yang aktif dan bukan akun header")
	}
	if correction.SaleID == nil || correction.LinkedPaymentID == nil {
		return fmt.Errorf("data penjualan atau pembayaran pelanggan pada koreksi belum lengkap")
	}
	data, err := s.loadLegacySalesCreditCorrectionData(tx, *correction.SaleID, *correction.LinkedPaymentID, correction.ID, false)
	if err != nil {
		return err
	}
	if !journalHasCreditLine(data.paymentJournal, uint64(account.ID), data.sale.TotalAmount) {
		return fmt.Errorf("COA yang dipilih harus sama dengan akun piutang yang dikredit pada pembayaran pelanggan %s", data.payment.Code)
	}
	return nil
}

func (s *PostedTransactionCorrectionService) loadLegacySalesCreditCorrectionData(
	tx *gorm.DB,
	saleID uint,
	paymentID uint,
	excludeCorrectionID uint,
	lock bool,
) (*legacySalesCreditCorrectionData, error) {
	if tx == nil || saleID == 0 || paymentID == 0 {
		return nil, fmt.Errorf("penjualan dan pembayaran pelanggan wajib dipilih")
	}

	saleQuery := tx.Preload("Customer")
	paymentQuery := tx.Preload("Contact")
	if lock {
		saleQuery = saleQuery.Clauses(clause.Locking{Strength: "UPDATE"})
		paymentQuery = paymentQuery.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	var sale models.Sale
	if err := saleQuery.First(&sale, saleID).Error; err != nil {
		if errorsIsRecordNotFound(err) {
			return nil, fmt.Errorf("penjualan tidak ditemukan")
		}
		return nil, err
	}
	if !isDeferredSalePaymentTerm(sale.PaymentTerms) || !isImmediateCashBankSale(sale.PaymentMethodType) {
		return nil, fmt.Errorf("penjualan ini bukan kasus legacy termin kredit yang tercatat sebagai kas/bank")
	}
	if !isIDRCurrency(sale.Currency) {
		return nil, fmt.Errorf("koreksi otomatis penjualan kredit saat ini hanya mendukung transaksi IDR")
	}
	if sale.TotalAmount <= salesCreditCorrectionTolerance ||
		math.Abs(sale.PaidAmount-sale.TotalAmount) > salesCreditCorrectionTolerance ||
		sale.OutstandingAmount > salesCreditCorrectionTolerance {
		return nil, fmt.Errorf("penjualan harus sudah tercatat lunas penuh sebelum dapat direklasifikasi")
	}
	if sale.CashBankID == nil || *sale.CashBankID == 0 {
		return nil, fmt.Errorf("akun kas/bank asal penjualan tidak ditemukan")
	}

	originalCashBankQuery := tx
	if lock {
		originalCashBankQuery = originalCashBankQuery.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var originalCashBank models.CashBank
	if err := originalCashBankQuery.First(&originalCashBank, *sale.CashBankID).Error; err != nil {
		return nil, fmt.Errorf("akun kas/bank asal penjualan tidak ditemukan")
	}
	if originalCashBank.AccountID == 0 {
		return nil, fmt.Errorf("akun kas/bank asal belum memiliki COA")
	}
	if !isIDRCurrency(originalCashBank.Currency) {
		return nil, fmt.Errorf("koreksi otomatis penjualan kredit saat ini hanya mendukung akun kas/bank IDR")
	}

	var originalJournal models.SSOTJournalEntry
	if err := tx.Preload("Lines").
		Where("source_type = ? AND source_id = ? AND status = ? AND deleted_at IS NULL", models.SSOTSourceTypeSale, uint64(sale.ID), models.SSOTStatusPosted).
		Order("entry_date ASC, id ASC").
		First(&originalJournal).Error; err != nil {
		if errorsIsRecordNotFound(err) {
			return nil, fmt.Errorf("penjualan ini belum memiliki jurnal penjualan terposting")
		}
		return nil, err
	}
	if !journalHasDebitLine(originalJournal, uint64(originalCashBank.AccountID), sale.TotalAmount) {
		return nil, fmt.Errorf("jurnal penjualan asal tidak menunjukkan debit kas/bank sebesar nilai invoice; gunakan koreksi manual")
	}
	originalCashBankTransaction, err := loadLegacySalesCreditCashBankTransaction(
		tx,
		originalCashBank.ID,
		[]string{"SALE"},
		sale.ID,
		sale.TotalAmount,
		"mutasi kas/bank awal dari penjualan",
		lock,
	)
	if err != nil {
		return nil, err
	}

	var payment models.Payment
	if err := paymentQuery.First(&payment, paymentID).Error; err != nil {
		if errorsIsRecordNotFound(err) {
			return nil, fmt.Errorf("pembayaran pelanggan tidak ditemukan")
		}
		return nil, err
	}
	if payment.ContactID != sale.CustomerID ||
		!strings.EqualFold(payment.Status, models.PaymentStatusCompleted) ||
		payment.Date.Before(sale.Date) ||
		math.Abs(payment.Amount-sale.TotalAmount) > salesCreditCorrectionTolerance ||
		!isIDRCurrency(payment.Currency) || payment.CashBankID == nil || *payment.CashBankID == 0 {
		return nil, fmt.Errorf("pembayaran pelanggan tidak memenuhi syarat koreksi penjualan kredit")
	}

	paymentCashBankQuery := tx
	if lock {
		paymentCashBankQuery = paymentCashBankQuery.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var paymentCashBank models.CashBank
	if err := paymentCashBankQuery.First(&paymentCashBank, *payment.CashBankID).Error; err != nil {
		return nil, fmt.Errorf("akun kas/bank pembayaran pelanggan tidak ditemukan")
	}
	if paymentCashBank.AccountID == 0 {
		return nil, fmt.Errorf("akun kas/bank pembayaran pelanggan belum memiliki COA")
	}
	if !isIDRCurrency(paymentCashBank.Currency) {
		return nil, fmt.Errorf("koreksi otomatis penjualan kredit saat ini hanya mendukung akun kas/bank IDR")
	}

	var paymentJournal models.SSOTJournalEntry
	paymentJournalQuery := tx.Preload("Lines").
		Where("source_type = ? AND source_id = ? AND status = ? AND deleted_at IS NULL",
			models.SSOTSourceTypePayment,
			uint64(payment.ID),
			models.SSOTStatusPosted,
		)
	if payment.JournalEntryID != nil && *payment.JournalEntryID > 0 {
		paymentJournalQuery = paymentJournalQuery.Where("id = ?", *payment.JournalEntryID)
	}
	if err := paymentJournalQuery.Order("entry_date ASC, id ASC").First(&paymentJournal).Error; err != nil {
		if errorsIsRecordNotFound(err) {
			return nil, fmt.Errorf("pembayaran pelanggan belum memiliki jurnal terposting yang sesuai")
		}
		return nil, err
	}
	if !journalHasDebitLine(paymentJournal, uint64(paymentCashBank.AccountID), payment.Amount) {
		return nil, fmt.Errorf("jurnal pembayaran pelanggan tidak menunjukkan debit kas/bank sebesar nilai pembayaran")
	}
	paymentCashBankTransaction, err := loadLegacySalesCreditCashBankTransaction(
		tx,
		paymentCashBank.ID,
		salesCreditCorrectionReceiptReferenceTypes,
		payment.ID,
		payment.Amount,
		"mutasi kas/bank penerimaan pelanggan",
		lock,
	)
	if err != nil {
		return nil, err
	}

	var paymentAllocationCount int64
	if err := tx.Model(&models.PaymentAllocation{}).Where("payment_id = ?", payment.ID).Count(&paymentAllocationCount).Error; err != nil {
		return nil, err
	}
	if paymentAllocationCount > 0 {
		return nil, fmt.Errorf("pembayaran pelanggan sudah memiliki alokasi sehingga tidak dapat ditautkan ulang")
	}
	var saleAllocationCount int64
	if err := tx.Model(&models.PaymentAllocation{}).Where("invoice_id = ?", sale.ID).Count(&saleAllocationCount).Error; err != nil {
		return nil, err
	}
	if saleAllocationCount > 0 {
		return nil, fmt.Errorf("invoice penjualan sudah memiliki alokasi pembayaran")
	}

	duplicateQuery := tx.Model(&models.PostedTransactionCorrection{}).
		Where("correction_type = ? AND status IN ? AND (sale_id = ? OR linked_payment_id = ?)",
			models.PostedTransactionCorrectionTypeSalesCreditClassification,
			append(activeCorrectionStatuses(), models.PostedTransactionCorrectionStatusCompleted),
			sale.ID,
			payment.ID)
	if excludeCorrectionID > 0 {
		duplicateQuery = duplicateQuery.Where("id <> ?", excludeCorrectionID)
	}
	var existingCorrectionCount int64
	if err := duplicateQuery.Count(&existingCorrectionCount).Error; err != nil {
		return nil, err
	}
	if existingCorrectionCount > 0 {
		return nil, fmt.Errorf("penjualan atau pembayaran ini sudah memiliki koreksi reklasifikasi aktif atau selesai")
	}

	return &legacySalesCreditCorrectionData{
		sale:                        sale,
		payment:                     payment,
		originalJournal:             originalJournal,
		paymentJournal:              paymentJournal,
		originalCashBank:            originalCashBank,
		paymentCashBank:             paymentCashBank,
		originalCashBankTransaction: originalCashBankTransaction,
		paymentCashBankTransaction:  paymentCashBankTransaction,
	}, nil
}

func loadLegacySalesCreditCashBankTransaction(
	tx *gorm.DB,
	cashBankID uint,
	referenceTypes []string,
	referenceID uint,
	amount float64,
	description string,
	lock bool,
) (models.CashBankTransaction, error) {
	query := tx.
		Where("cash_bank_id = ? AND reference_id = ? AND reference_type IN ? AND deleted_at IS NULL", cashBankID, referenceID, referenceTypes).
		Order("id ASC")
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	var transactions []models.CashBankTransaction
	if err := query.Find(&transactions).Error; err != nil {
		return models.CashBankTransaction{}, err
	}

	matches := make([]models.CashBankTransaction, 0, len(transactions))
	for _, transaction := range transactions {
		if transaction.Amount > salesCreditCorrectionTolerance &&
			math.Abs(transaction.Amount-amount) <= salesCreditCorrectionTolerance &&
			isIDRCurrency(transaction.Currency) {
			matches = append(matches, transaction)
		}
	}
	if len(matches) == 0 {
		return models.CashBankTransaction{}, fmt.Errorf("%s tidak ditemukan atau nominalnya tidak sesuai; gunakan koreksi manual setelah rekonsiliasi bank", description)
	}
	if len(matches) > 1 {
		return models.CashBankTransaction{}, fmt.Errorf("%s memiliki lebih dari satu catatan yang cocok; gunakan koreksi manual setelah rekonsiliasi bank", description)
	}

	return matches[0], nil
}

func journalHasDebitLine(journal models.SSOTJournalEntry, accountID uint64, amount float64) bool {
	target := decimal.NewFromFloat(amount).Round(2)
	for _, line := range journal.Lines {
		if line.AccountID == accountID && line.DebitAmount.Round(2).Equal(target) && line.CreditAmount.IsZero() {
			return true
		}
	}
	return false
}

func journalHasCreditLine(journal models.SSOTJournalEntry, accountID uint64, amount float64) bool {
	target := decimal.NewFromFloat(amount).Round(2)
	for _, line := range journal.Lines {
		if line.AccountID == accountID && line.CreditAmount.Round(2).Equal(target) && line.DebitAmount.IsZero() {
			return true
		}
	}
	return false
}

func isImmediateCashBankSale(paymentMethodType string) bool {
	switch strings.ToUpper(strings.TrimSpace(paymentMethodType)) {
	case "CASH", "BANK":
		return true
	default:
		return false
	}
}

func isIDRCurrency(currency string) bool {
	return strings.EqualFold(strings.TrimSpace(currency), "") || strings.EqualFold(strings.TrimSpace(currency), "IDR")
}

func saleCorrectionTargetCode(sale *models.Sale) string {
	if sale == nil {
		return "PENJUALAN"
	}
	if invoice := strings.TrimSpace(sale.InvoiceNumber); invoice != "" {
		return invoice
	}
	return sale.Code
}

func errorsIsRecordNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}
