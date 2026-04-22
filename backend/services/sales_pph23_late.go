package services

import (
	"app-sistem-akuntansi/models"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type salePPh23CorrectionSource struct {
	CapacityAmount    float64
	NativeAmount      float64
	BookAmountIDR     float64
	ForexGainLoss     float64
	ForexGainLossType string
	PaymentMethod     string
	Label             string
	Snapshot          *cashBankSnapshot
}

func resolveSalesReceivableAccount(tx *gorm.DB) (*models.Account, error) {
	var acc models.Account
	if err := tx.Where("code = ? AND deleted_at IS NULL", "1102-001").First(&acc).Error; err == nil {
		return &acc, nil
	}
	if err := tx.Where("code = ? AND deleted_at IS NULL", "1102").First(&acc).Error; err == nil {
		return &acc, nil
	}
	if err := tx.Where("code = ? AND deleted_at IS NULL", "1201").First(&acc).Error; err == nil {
		return &acc, nil
	}
	return nil, fmt.Errorf("akun piutang usaha tidak ditemukan")
}

func deriveSalePaymentReceivableSettlementBookAmount(payment *models.SalePayment) float64 {
	if payment == nil {
		return 0
	}

	bankBookAmount := payment.BookAmountIDR
	if bankBookAmount == 0 {
		bankBookAmount = payment.Amount
	}

	switch strings.ToUpper(strings.TrimSpace(payment.ForexGainLossType)) {
	case "GAIN":
		return bankBookAmount - payment.ForexGainLossAmount
	case "LOSS":
		return bankBookAmount + payment.ForexGainLossAmount
	default:
		return bankBookAmount
	}
}

func roundSalePPh23Rate(value float64) float64 {
	return math.Round(value*100) / 100
}

func roundSalePPh23NativeAmount(value float64) float64 {
	return math.Round(value*1000000) / 1000000
}

func inverseReceivableFXType(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "GAIN":
		return "LOSS"
	case "LOSS":
		return "GAIN"
	default:
		return ""
	}
}

func rebalanceExistingSalePaymentSchedules(tx *gorm.DB, sale *models.Sale) error {
	if sale == nil || sale.ID == 0 || strings.ToUpper(strings.TrimSpace(sale.PaymentTerms)) != "CUSTOM" {
		return nil
	}

	var schedules []models.SalePaymentSchedule
	if err := tx.Where("sale_id = ?", sale.ID).Order("sequence ASC").Find(&schedules).Error; err != nil {
		return fmt.Errorf("failed to load sale payment schedules: %w", err)
	}
	if len(schedules) == 0 {
		return nil
	}

	allocated := 0.0
	for index := range schedules {
		newAmount := roundScheduleMoney(sale.TotalAmount * schedules[index].Percentage / 100)
		if index == len(schedules)-1 {
			newAmount = roundScheduleMoney(sale.TotalAmount - allocated)
		} else {
			allocated += newAmount
		}
		schedules[index].Amount = newAmount
		if err := tx.Model(&models.SalePaymentSchedule{}).
			Where("id = ?", schedules[index].ID).
			Update("amount", newAmount).Error; err != nil {
			return fmt.Errorf("failed to update sale payment schedule amount: %w", err)
		}
	}

	sale.PaymentSchedules = schedules
	return nil
}

func deriveLateSalePPh23Values(sale *models.Sale, request models.SaleLatePPh23Request) (float64, float64, error) {
	if sale == nil {
		return 0, 0, fmt.Errorf("sale is required")
	}

	amount := roundScheduleMoney(request.Amount)
	if amount <= 0 {
		if request.Rate == nil || *request.Rate <= 0 {
			return 0, 0, fmt.Errorf("nominal atau tarif PPh23 wajib diisi")
		}
		amount = roundScheduleMoney(sale.TaxableAmount * (*request.Rate / 100))
	}
	if amount <= 0 {
		return 0, 0, fmt.Errorf("nominal PPh23 harus lebih besar dari 0")
	}
	if sale.TaxableAmount > 0 && amount-sale.TaxableAmount > 0.005 {
		return 0, 0, fmt.Errorf("nominal PPh23 tidak boleh melebihi nilai dasar pengenaan pajak")
	}

	rate := 0.0
	if request.Rate != nil {
		rate = roundSalePPh23Rate(*request.Rate)
		if rate <= 0 {
			return 0, 0, fmt.Errorf("tarif PPh23 harus lebih besar dari 0")
		}
	} else if sale.TaxableAmount > 0 {
		rate = roundSalePPh23Rate((amount / sale.TaxableAmount) * 100)
	}

	return amount, rate, nil
}

func buildSalePPh23ReferenceNumber(sale *models.Sale) string {
	if sale == nil {
		return ""
	}

	referenceNumber := strings.TrimSpace(sale.InvoiceNumber)
	if referenceNumber == "" {
		referenceNumber = strings.TrimSpace(sale.Code)
	}
	return referenceNumber
}

func buildSalePPh23CorrectionSources(tx *gorm.DB, sale *models.Sale) ([]salePPh23CorrectionSource, error) {
	if sale == nil {
		return nil, fmt.Errorf("sale is required")
	}

	sources := make([]salePPh23CorrectionSource, 0)

	var payments []models.SalePayment
	if err := tx.
		Where("sale_id = ? AND deleted_at IS NULL AND status <> ? AND amount > 0", sale.ID, models.SalePaymentStatusCancelled).
		Order("payment_date DESC, id DESC").
		Find(&payments).Error; err != nil {
		return nil, fmt.Errorf("failed to load sale payments: %w", err)
	}

	for _, payment := range payments {
		if payment.CashBankID == nil || *payment.CashBankID == 0 {
			continue
		}

		snapshot := &cashBankSnapshot{
			CashBankID:       *payment.CashBankID,
			CashBankCode:     payment.CashBankCode,
			CashBankName:     payment.CashBankName,
			CashBankCurrency: normalizeCashBankCurrency(payment.CashBankCurrency),
			GLAccountCode:    payment.CashBankGLAccountCode,
			GLAccountName:    payment.CashBankGLAccountName,
		}
		if payment.CashBankGLAccountID != nil && *payment.CashBankGLAccountID > 0 {
			snapshot.GLAccountID = *payment.CashBankGLAccountID
		}
		if snapshot.GLAccountID == 0 {
			loadedSnapshot, err := loadCashBankSnapshot(tx, *payment.CashBankID)
			if err != nil {
				return nil, err
			}
			snapshot = loadedSnapshot
		}

		capacity := roundScheduleMoney(deriveSalePaymentReceivableSettlementBookAmount(&payment))
		if capacity <= 0.005 {
			continue
		}

		bookAmountIDR := roundScheduleMoney(payment.BookAmountIDR)
		if math.Abs(bookAmountIDR) <= 0.005 {
			bookAmountIDR = roundScheduleMoney(payment.Amount)
		}

		nativeAmount := roundSalePPh23NativeAmount(payment.NativeAmount)
		if math.Abs(nativeAmount) <= 0.000001 {
			if snapshot.CashBankCurrency == "USD" && payment.ExchangeRate > 0 {
				nativeAmount = roundSalePPh23NativeAmount(bookAmountIDR / payment.ExchangeRate)
			} else {
				nativeAmount = roundSalePPh23NativeAmount(bookAmountIDR)
			}
		}

		sources = append(sources, salePPh23CorrectionSource{
			CapacityAmount:    capacity,
			NativeAmount:      nativeAmount,
			BookAmountIDR:     bookAmountIDR,
			ForexGainLoss:     roundScheduleMoney(payment.ForexGainLossAmount),
			ForexGainLossType: strings.ToUpper(strings.TrimSpace(payment.ForexGainLossType)),
			PaymentMethod:     strings.TrimSpace(payment.PaymentMethod),
			Label:             strings.TrimSpace(payment.Reference),
			Snapshot:          snapshot,
		})
	}

	if len(sources) > 0 {
		return sources, nil
	}

	referenceNumber := buildSalePPh23ReferenceNumber(sale)

	var originalMovement models.CashBankTransaction
	err := tx.
		Where("reference_type = ? AND reference_id = ? AND deleted_at IS NULL", "SALE", sale.ID).
		Order("id DESC").
		First(&originalMovement).Error

	cashBankID := uint(0)
	bookAmountIDR := roundScheduleMoney(sale.PaidAmount)
	nativeAmount := roundSalePPh23NativeAmount(sale.PaidAmount)
	if bookAmountIDR <= 0.005 {
		bookAmountIDR = roundScheduleMoney(sale.TotalAmount)
		nativeAmount = roundSalePPh23NativeAmount(sale.TotalAmount)
	}

	if err == nil {
		cashBankID = originalMovement.CashBankID
		bookAmountIDR = roundScheduleMoney(math.Abs(originalMovement.BookAmountIDR))
		nativeAmount = roundSalePPh23NativeAmount(math.Abs(originalMovement.Amount))
	}

	if cashBankID == 0 && sale.CashBankID != nil && *sale.CashBankID > 0 {
		cashBankID = *sale.CashBankID
	}
	if cashBankID == 0 {
		return nil, fmt.Errorf("invoice lunas ini tidak memiliki histori payment kas/bank yang bisa dikoreksi untuk PPh23")
	}

	snapshot, snapshotErr := loadCashBankSnapshot(tx, cashBankID)
	if snapshotErr != nil {
		return nil, snapshotErr
	}

	if math.Abs(nativeAmount) <= 0.000001 {
		if snapshot.CashBankCurrency == "USD" && sale.ExchangeRate > 0 {
			nativeAmount = roundSalePPh23NativeAmount(bookAmountIDR / sale.ExchangeRate)
		} else {
			nativeAmount = roundSalePPh23NativeAmount(bookAmountIDR)
		}
	}

	return []salePPh23CorrectionSource{
		{
			CapacityAmount: roundScheduleMoney(sale.PaidAmount),
			NativeAmount:   nativeAmount,
			BookAmountIDR:  bookAmountIDR,
			PaymentMethod:  strings.TrimSpace(sale.PaymentMethod),
			Label:          referenceNumber,
			Snapshot:       snapshot,
		},
	}, nil
}

func (s *SalesServiceV2) createSalePPh23PaymentCorrection(
	tx *gorm.DB,
	sale *models.Sale,
	source salePPh23CorrectionSource,
	correctionAmount float64,
	correctionDate time.Time,
	reference string,
	notes string,
	userID uint,
) error {
	if sale == nil {
		return fmt.Errorf("sale is required")
	}
	if correctionAmount <= 0 {
		return fmt.Errorf("nilai koreksi payment harus lebih besar dari 0")
	}
	if source.CapacityAmount <= 0.005 || source.Snapshot == nil {
		return fmt.Errorf("sumber koreksi payment PPh23 tidak valid")
	}

	proportion := correctionAmount / source.CapacityAmount
	if proportion > 1 {
		proportion = 1
	}

	bookReduction := roundScheduleMoney(source.BookAmountIDR * proportion)
	if proportion >= 0.999999 {
		bookReduction = roundScheduleMoney(source.BookAmountIDR)
	}
	if bookReduction <= 0.005 {
		bookReduction = roundScheduleMoney(correctionAmount)
	}

	nativeReduction := roundSalePPh23NativeAmount(source.NativeAmount * proportion)
	if proportion >= 0.999999 {
		nativeReduction = roundSalePPh23NativeAmount(source.NativeAmount)
	}
	if math.Abs(nativeReduction) <= 0.000001 {
		nativeReduction = roundSalePPh23NativeAmount(bookReduction)
	}

	forexReduction := roundScheduleMoney(source.ForexGainLoss * proportion)
	if proportion >= 0.999999 {
		forexReduction = roundScheduleMoney(source.ForexGainLoss)
	}

	guard := NewCashBankCurrencyGuard(s.db)
	if err := guard.EnforceAfterTransaction(tx, source.Snapshot.CashBankID); err != nil {
		return fmt.Errorf("failed to sync cash/bank state before PPh23 correction: %w", err)
	}

	var cashBank models.CashBank
	if err := tx.First(&cashBank, source.Snapshot.CashBankID).Error; err != nil {
		return fmt.Errorf("cash/bank account not found: %w", err)
	}

	newBalance := cashBank.Balance - nativeReduction
	if newBalance < -0.01 {
		return fmt.Errorf("koreksi PPh23 akan membuat saldo kas/bank menjadi negatif")
	}

	newBookBalanceIDR := cashBank.BookBalanceIDR - bookReduction
	if !cashBank.IsForeignCurrency() {
		newBookBalanceIDR = newBalance
	}
	if newBookBalanceIDR < -0.01 {
		return fmt.Errorf("koreksi PPh23 akan membuat saldo buku kas/bank menjadi negatif")
	}

	exchangeRate := 1.0
	if source.Snapshot.CashBankCurrency == "USD" && nativeReduction > 0 {
		exchangeRate = bookReduction / nativeReduction
		if exchangeRate <= 0 {
			exchangeRate = 1
		}
	}

	referenceNumber := buildSalePPh23ReferenceNumber(sale)
	paymentReference := strings.TrimSpace(reference)
	if paymentReference == "" {
		paymentReference = fmt.Sprintf("PPH23-CORR-%s", referenceNumber)
	}

	paymentMethod := strings.TrimSpace(source.PaymentMethod)
	if paymentMethod == "" {
		paymentMethod = strings.TrimSpace(sale.PaymentMethod)
	}
	if paymentMethod == "" {
		paymentMethod = strings.TrimSpace(sale.PaymentMethodType)
	}
	if paymentMethod == "" {
		paymentMethod = "BANK"
	}

	paymentNotes := strings.TrimSpace(notes)
	if paymentNotes != "" {
		paymentNotes = fmt.Sprintf("Koreksi payment untuk PPh23 susulan. %s", paymentNotes)
	} else {
		paymentNotes = fmt.Sprintf("Koreksi payment untuk PPh23 susulan - %s", referenceNumber)
	}
	if source.Label != "" {
		paymentNotes = fmt.Sprintf("%s (sumber: %s)", paymentNotes, source.Label)
	}

	correctionPayment := &models.SalePayment{
		SaleID:              sale.ID,
		Amount:              -roundScheduleMoney(correctionAmount),
		NativeAmount:        -nativeReduction,
		Currency:            source.Snapshot.CashBankCurrency,
		ExchangeRate:        exchangeRate,
		BookAmountIDR:       -bookReduction,
		ForexGainLossAmount: forexReduction,
		ForexGainLossType:   inverseReceivableFXType(source.ForexGainLossType),
		PaymentDate:         correctionDate,
		PaymentMethod:       paymentMethod,
		Reference:           paymentReference,
		Notes:               paymentNotes,
		UserID:              userID,
		Status:              models.SalePaymentStatusCompleted,
	}
	applyCashBankSnapshotToSalePayment(correctionPayment, source.Snapshot)

	if err := tx.Create(correctionPayment).Error; err != nil {
		return fmt.Errorf("failed to create PPh23 correction payment: %w", err)
	}

	receivableAccount, err := resolveSalesReceivableAccount(tx)
	if err != nil {
		return err
	}

	journalLines := []JournalLineRequest{
		{
			AccountID:    uint64(receivableAccount.ID),
			DebitAmount:  decimal.NewFromFloat(roundScheduleMoney(correctionAmount)),
			CreditAmount: decimal.Zero,
			Description:  fmt.Sprintf("Buka kembali piutang untuk koreksi PPh23 - %s", referenceNumber),
		},
		{
			AccountID:    uint64(source.Snapshot.GLAccountID),
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.NewFromFloat(bookReduction),
			Description:  fmt.Sprintf("Koreksi kas/bank untuk PPh23 - %s", referenceNumber),
		},
	}

	if forexReduction > 0.005 {
		journalFXService := s.salesJournalService
		if journalFXService == nil {
			journalFXService = &SalesJournalServiceV2{db: tx}
		}

		switch source.ForexGainLossType {
		case "GAIN":
			accountID, accountErr := journalFXService.resolveForexAccountID(tx, true)
			if accountErr != nil {
				return fmt.Errorf("failed to resolve forex gain account for PPh23 correction: %w", accountErr)
			}
			journalLines = append(journalLines, JournalLineRequest{
				AccountID:    accountID,
				DebitAmount:  decimal.NewFromFloat(forexReduction),
				CreditAmount: decimal.Zero,
				Description:  fmt.Sprintf("Balikkan forex gain payment untuk PPh23 - %s", referenceNumber),
			})
		case "LOSS":
			accountID, accountErr := journalFXService.resolveForexAccountID(tx, false)
			if accountErr != nil {
				return fmt.Errorf("failed to resolve forex loss account for PPh23 correction: %w", accountErr)
			}
			journalLines = append(journalLines, JournalLineRequest{
				AccountID:    accountID,
				DebitAmount:  decimal.Zero,
				CreditAmount: decimal.NewFromFloat(forexReduction),
				Description:  fmt.Sprintf("Balikkan forex loss payment untuk PPh23 - %s", referenceNumber),
			})
		}
	}

	journalReference := paymentReference
	unifiedJournalService := NewUnifiedJournalService(tx)
	if _, err := unifiedJournalService.CreateJournalEntryWithTx(tx, &JournalEntryRequest{
		EntryDate:   correctionDate,
		Reference:   journalReference,
		Description: fmt.Sprintf("Koreksi payment PPh23 Sales - %s", referenceNumber),
		Notes:       paymentNotes,
		SourceType:  "SALES_PAYMENT",
		SourceID:    uint64(correctionPayment.ID),
		AutoPost:    true,
		CreatedBy:   uint64(userID),
		Lines:       journalLines,
	}); err != nil {
		return fmt.Errorf("failed to create PPh23 correction journal: %w", err)
	}

	cashBank.Balance = newBalance
	cashBank.BookBalanceIDR = newBookBalanceIDR
	if cashBank.IsForeignCurrency() {
		cashBank.LastExchangeRate = exchangeRate
	} else {
		cashBank.LastExchangeRate = 1
	}

	if err := tx.Save(&cashBank).Error; err != nil {
		return fmt.Errorf("failed to update cash/bank after PPh23 correction: %w", err)
	}

	if cashBank.AccountID > 0 {
		if err := tx.Exec(
			"UPDATE accounts SET balance = ?, updated_at = NOW() WHERE id = ? AND deleted_at IS NULL",
			cashBank.EffectiveBookBalanceIDR(),
			cashBank.AccountID,
		).Error; err != nil {
			return fmt.Errorf("failed to sync linked GL cash/bank account after PPh23 correction: %w", err)
		}
	}

	cashBankTransaction := &models.CashBankTransaction{
		CashBankID:      cashBank.ID,
		ReferenceType:   "SALES_PAYMENT",
		ReferenceID:     correctionPayment.ID,
		Amount:          -nativeReduction,
		Currency:        source.Snapshot.CashBankCurrency,
		ExchangeRate:    exchangeRate,
		BookAmountIDR:   -bookReduction,
		BalanceAfter:    cashBank.Balance,
		TransactionDate: correctionDate,
		Notes:           paymentNotes,
	}
	if err := tx.Create(cashBankTransaction).Error; err != nil {
		return fmt.Errorf("failed to create cash/bank correction transaction for PPh23: %w", err)
	}

	if err := guard.EnforceAfterTransaction(tx, cashBank.ID); err != nil {
		return fmt.Errorf("failed to enforce cash/bank invariant after PPh23 correction: %w", err)
	}

	sale.PaidAmount = roundScheduleMoney(sale.PaidAmount - correctionAmount)
	return nil
}

func (s *SalesServiceV2) createLateSalePPh23FinalJournal(
	tx *gorm.DB,
	sale *models.Sale,
	amount float64,
	request models.SaleLatePPh23Request,
	userID uint,
) (*models.SSOTJournalEntry, string, error) {
	if sale == nil {
		return nil, "", fmt.Errorf("sale is required")
	}

	referenceNumber := buildSalePPh23ReferenceNumber(sale)
	taxHelper := NewTaxAccountHelper(tx)

	pph23CreditAccount, err := taxHelper.GetPPh23TaxCreditAccount(tx)
	if err != nil {
		return nil, "", fmt.Errorf("akun Kredit Pajak PPh23 tidak ditemukan: %v", err)
	}

	receivableAccount, err := resolveSalesReceivableAccount(tx)
	if err != nil {
		return nil, "", err
	}

	journalReference := strings.TrimSpace(request.Reference)
	if journalReference == "" {
		journalReference = fmt.Sprintf("PPH23-%s", referenceNumber)
	}

	unifiedJournalService := NewUnifiedJournalService(tx)
	journalEntry, err := unifiedJournalService.CreateJournalEntryWithTx(tx, &JournalEntryRequest{
		EntryDate:   request.Date,
		Reference:   journalReference,
		Description: fmt.Sprintf("Posting PPh23 Susulan Sales - %s", referenceNumber),
		Notes:       strings.TrimSpace(request.Notes),
		SourceType:  models.SSOTSourceTypeAdjustment,
		SourceID:    uint64(sale.ID),
		AutoPost:    true,
		CreatedBy:   uint64(userID),
		Lines: []JournalLineRequest{
			{
				AccountID:    uint64(pph23CreditAccount.ID),
				DebitAmount:  decimal.NewFromFloat(amount),
				CreditAmount: decimal.Zero,
				Description:  fmt.Sprintf("Kredit Pajak PPh23 - %s", referenceNumber),
			},
			{
				AccountID:    uint64(receivableAccount.ID),
				DebitAmount:  decimal.Zero,
				CreditAmount: decimal.NewFromFloat(amount),
				Description:  fmt.Sprintf("Kurangi Piutang Usaha untuk PPh23 - %s", referenceNumber),
			},
		},
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to create late PPh23 journal: %w", err)
	}

	return journalEntry, journalReference, nil
}

func (s *SalesServiceV2) AddLatePPh23(saleID uint, request models.SaleLatePPh23Request, userID uint) (*models.Sale, error) {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var sale models.Sale
	if err := tx.Preload("SalePayments").First(&sale, saleID).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("sale not found")
	}

	if err := CheckClosedPeriod(tx, request.Date); err != nil {
		tx.Rollback()
		return nil, err
	}

	if sale.Status == models.SaleStatusCancelled {
		tx.Rollback()
		return nil, fmt.Errorf("penjualan yang dibatalkan tidak bisa ditambahkan PPh23")
	}

	switch strings.ToUpper(strings.TrimSpace(sale.Status)) {
	case models.SaleStatusDraft, models.SaleStatusPending, models.SaleStatusConfirmed:
		tx.Rollback()
		return nil, fmt.Errorf("PPh23 susulan hanya bisa ditambahkan untuk invoice yang sudah diterbitkan")
	}

	if sale.PPh23Amount > 0.005 {
		tx.Rollback()
		return nil, fmt.Errorf("invoice ini sudah memiliki PPh23, gunakan posting PPh23 yang sudah tersedia")
	}

	if sale.PPh23SettlementJournalEntryID != nil || sale.PPh23SettlementDate != nil {
		tx.Rollback()
		return nil, fmt.Errorf("PPh23 untuk invoice ini sudah pernah dipost")
	}

	amount, rate, err := deriveLateSalePPh23Values(&sale, request)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if sale.PaidAmount+amount < sale.TotalAmount-0.005 {
		tx.Rollback()
		return nil, fmt.Errorf("invoice belum lunas. Posting PPh23 susulan hanya bisa dilakukan ketika pembayaran customer ditambah PPh23 sudah menutup total invoice")
	}

	if sale.OutstandingAmount <= 0.005 {
		correctionSources, sourceErr := buildSalePPh23CorrectionSources(tx, &sale)
		if sourceErr != nil {
			tx.Rollback()
			return nil, sourceErr
		}

		remainingCorrection := amount
		for _, source := range correctionSources {
			if remainingCorrection <= 0.005 {
				break
			}

			allocation := math.Min(remainingCorrection, source.CapacityAmount)
			if allocation <= 0.005 {
				continue
			}

			if err := s.createSalePPh23PaymentCorrection(
				tx,
				&sale,
				source,
				roundScheduleMoney(allocation),
				request.Date,
				request.Reference,
				request.Notes,
				userID,
			); err != nil {
				tx.Rollback()
				return nil, err
			}

			remainingCorrection = roundScheduleMoney(remainingCorrection - allocation)
		}

		if remainingCorrection > 0.005 {
			tx.Rollback()
			return nil, fmt.Errorf("histori payment kas/bank tidak cukup untuk mengoreksi invoice lunas sebesar PPh23 yang diminta")
		}
	}

	sale.PPh23Rate = rate
	sale.PPh23Amount = amount
	sale.TotalTaxDeductions = sale.PPh + sale.PPh21Amount + sale.PPh23Amount + sale.OtherTaxDeductions
	sale.TotalTax = sale.PPN + sale.TotalTaxAdditions - sale.TotalTaxDeductions
	sale.Tax = sale.TotalTax
	sale.TotalAmount = sale.TaxableAmount + sale.PPN + sale.OtherTaxAdditions + sale.ShippingCost - sale.TotalTaxDeductions
	sale.OutstandingAmount = roundScheduleMoney(sale.TotalAmount - sale.PaidAmount)
	if sale.OutstandingAmount <= 0.005 {
		sale.OutstandingAmount = 0
		sale.Status = models.SaleStatusPaid
	} else {
		sale.Status = models.SaleStatusInvoiced
	}
	sale.SettlementStatus = models.ResolveSaleSettlementStatus(sale.Status, sale.PaidAmount, sale.OutstandingAmount)

	journalEntry, journalReference, err := s.createLateSalePPh23FinalJournal(tx, &sale, amount, request, userID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	journalEntryID := uint(journalEntry.ID)
	settlementDate := request.Date
	sale.PPh23SettlementDate = &settlementDate
	sale.PPh23SettlementReference = journalReference
	sale.PPh23SettlementNotes = strings.TrimSpace(request.Notes)
	sale.PPh23SettlementJournalEntryID = &journalEntryID
	sale.PPh23SettlementUserID = &userID

	if err := tx.Save(&sale).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update sale after adding late PPh23: %w", err)
	}

	if err := rebalanceExistingSalePaymentSchedules(tx, &sale); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := syncSalePaymentScheduleProgress(tx, &sale); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to sync sale payment schedules: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit late PPh23 posting: %w", err)
	}

	if sale.ProjectID != nil && *sale.ProjectID > 0 {
		s.recalculateProjectSalesRevenue(*sale.ProjectID)
	}

	return s.GetSaleByID(saleID)
}
