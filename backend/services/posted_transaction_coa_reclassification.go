package services

import (
	"fmt"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/utils"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PurchaseCOAReclassificationJournalLineCandidate is a non-cash/bank line
// from the original purchase journal that Accounting may reclassify.
type PurchaseCOAReclassificationJournalLineCandidate struct {
	ID                 uint64    `json:"id"`
	JournalEntryID     uint64    `json:"journal_entry_id"`
	JournalEntryNumber string    `json:"journal_entry_number"`
	JournalEntryDate   time.Time `json:"journal_entry_date"`
	LineNumber         int       `json:"line_number"`
	AccountID          uint      `json:"account_id"`
	AccountCode        string    `json:"account_code"`
	AccountName        string    `json:"account_name"`
	Description        string    `json:"description"`
	DebitAmount        float64   `json:"debit_amount"`
	CreditAmount       float64   `json:"credit_amount"`
	BulkSelectable     bool      `json:"bulk_selectable"`
}

// COAReclassificationAccountCandidate is an active leaf COA that is not linked
// to Cash & Bank. Returning this filtered list keeps the Accounting picker
// aligned with the same rule enforced again during submission and approval.
type COAReclassificationAccountCandidate struct {
	ID   uint   `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type purchaseCOAReclassificationData struct {
	purchase        models.Purchase
	originalJournal models.SSOTJournalEntry
	originalLine    models.SSOTJournalLine
	originalAccount models.Account
	proposedAccount models.Account
}

// ListPurchaseCOAReclassificationCandidates intentionally exposes only lines
// from the purchase journal itself. Payment journals and cash/bank lines cannot
// be selected because their correction requires a separate reconciliation flow.
func (s *PostedTransactionCorrectionService) ListPurchaseCOAReclassificationCandidates(
	purchaseID uint,
	userID uint,
	role string,
) ([]PurchaseCOAReclassificationJournalLineCandidate, error) {
	if !canRequestPostedCOAReclassification(role) {
		return nil, ErrPostedTransactionCorrectionDenied
	}
	if purchaseID == 0 {
		return nil, fmt.Errorf("pembelian tidak valid")
	}

	_, journal, err := loadPostedPurchaseJournalForCOAReclassification(s.db, purchaseID, false)
	if err != nil {
		return nil, err
	}
	bulkExcludedAccounts, err := purchaseCOAReclassificationBulkExcludedAccountIDs(s.db)
	if err != nil {
		return nil, err
	}

	result := make([]PurchaseCOAReclassificationJournalLineCandidate, 0, len(journal.Lines))
	for _, line := range journal.Lines {
		if line.Account == nil {
			continue
		}
		debit := line.DebitAmount.Round(2)
		credit := line.CreditAmount.Round(2)
		if (debit.GreaterThan(decimal.Zero) && credit.GreaterThan(decimal.Zero)) ||
			(!debit.GreaterThan(decimal.Zero) && !credit.GreaterThan(decimal.Zero)) {
			continue
		}
		isCashBank, err := isCashBankLinkedCorrectionAccount(s.db, line.Account.ID)
		if err != nil {
			return nil, err
		}
		if isCashBank {
			continue
		}
		result = append(result, PurchaseCOAReclassificationJournalLineCandidate{
			ID:                 line.ID,
			JournalEntryID:     journal.ID,
			JournalEntryNumber: journal.EntryNumber,
			JournalEntryDate:   journal.EntryDate,
			LineNumber:         line.LineNumber,
			AccountID:          line.Account.ID,
			AccountCode:        line.Account.Code,
			AccountName:        line.Account.Name,
			Description:        line.Description,
			DebitAmount:        debit.InexactFloat64(),
			CreditAmount:       credit.InexactFloat64(),
			BulkSelectable:     debit.GreaterThan(decimal.Zero) && !accountIDInSet(bulkExcludedAccounts, line.Account.ID),
		})
	}
	return result, nil
}

// purchaseCOAReclassificationBulkExcludedAccountIDs keeps the bulk item
// selector from silently including configured tax input. Accounting can still
// select any eligible line one-by-one when a tax reclassification is intended.
func purchaseCOAReclassificationBulkExcludedAccountIDs(tx *gorm.DB) (map[uint]struct{}, error) {
	excluded := make(map[uint]struct{})
	var settings models.TaxAccountSettings
	if err := tx.Where("is_active = ?", true).Order("id DESC").First(&settings).Error; err != nil {
		if errorsIsRecordNotFound(err) {
			return excluded, nil
		}
		return nil, err
	}
	if settings.PurchaseInputVATAccountID != 0 {
		excluded[settings.PurchaseInputVATAccountID] = struct{}{}
	}
	return excluded, nil
}

func accountIDInSet(values map[uint]struct{}, accountID uint) bool {
	_, found := values[accountID]
	return found
}

func (s *PostedTransactionCorrectionService) ListCOAReclassificationAccounts(
	userID uint,
	role string,
) ([]COAReclassificationAccountCandidate, error) {
	if !canRequestPostedCOAReclassification(role) {
		return nil, ErrPostedTransactionCorrectionDenied
	}

	cashBankAccounts := s.db.Model(&models.CashBank{}).
		Select("account_id").
		Where("deleted_at IS NULL")
	var accounts []models.Account
	if err := s.db.
		Where("is_active = ? AND is_header = ?", true, false).
		Where("id NOT IN (?)", cashBankAccounts).
		Order("code ASC, id ASC").
		Find(&accounts).Error; err != nil {
		return nil, err
	}

	result := make([]COAReclassificationAccountCandidate, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, COAReclassificationAccountCandidate{
			ID:   account.ID,
			Code: account.Code,
			Name: account.Name,
		})
	}
	return result, nil
}

func normalizePurchaseCOAReclassificationLineIDs(request models.PostedTransactionCorrectionCreateRequest) ([]uint64, error) {
	lineIDs := append([]uint64(nil), request.OriginalJournalLineIDs...)
	if len(lineIDs) == 0 && request.OriginalJournalLineID != nil {
		lineIDs = []uint64{*request.OriginalJournalLineID}
	}
	if len(lineIDs) == 0 {
		return nil, fmt.Errorf("pilih minimal satu baris jurnal asal untuk koreksi COA")
	}

	unique := make(map[uint64]struct{}, len(lineIDs))
	for _, lineID := range lineIDs {
		if lineID == 0 {
			return nil, fmt.Errorf("baris jurnal asal tidak valid")
		}
		if _, exists := unique[lineID]; exists {
			return nil, fmt.Errorf("baris jurnal asal tidak boleh dipilih lebih dari sekali")
		}
		unique[lineID] = struct{}{}
	}
	return lineIDs, nil
}

func (s *PostedTransactionCorrectionService) resolvePurchaseCOAReclassificationTargets(
	tx *gorm.DB,
	purchaseID uint,
	originalJournalLineIDs []uint64,
	proposedAccountID uint,
	lock bool,
) (*postedCorrectionTarget, []*purchaseCOAReclassificationData, error) {
	if len(originalJournalLineIDs) == 0 {
		return nil, nil, fmt.Errorf("pilih minimal satu baris jurnal asal untuk koreksi COA")
	}

	dataRows := make([]*purchaseCOAReclassificationData, 0, len(originalJournalLineIDs))
	var sourceJournalID uint64
	for _, originalJournalLineID := range originalJournalLineIDs {
		data, err := loadPurchaseCOAReclassificationData(tx, purchaseID, originalJournalLineID, proposedAccountID, lock)
		if err != nil {
			return nil, nil, err
		}
		if sourceJournalID != 0 && data.originalJournal.ID != sourceJournalID {
			return nil, nil, fmt.Errorf("semua baris koreksi harus berasal dari jurnal pembelian yang sama")
		}
		sourceJournalID = data.originalJournal.ID
		dataRows = append(dataRows, data)
	}

	first := dataRows[0]
	lineSnapshots := make([]map[string]interface{}, 0, len(dataRows))
	for _, data := range dataRows {
		lineSnapshots = append(lineSnapshots, map[string]interface{}{
			"original_journal_line_id": data.originalLine.ID,
			"line_number":              data.originalLine.LineNumber,
			"description":              data.originalLine.Description,
			"original_account": map[string]interface{}{
				"id":   data.originalAccount.ID,
				"code": data.originalAccount.Code,
				"name": data.originalAccount.Name,
			},
			"proposed_account": map[string]interface{}{
				"id":   data.proposedAccount.ID,
				"code": data.proposedAccount.Code,
				"name": data.proposedAccount.Name,
			},
			"debit_amount":  data.originalLine.DebitAmount.Round(2).InexactFloat64(),
			"credit_amount": data.originalLine.CreditAmount.Round(2).InexactFloat64(),
		})
	}

	snapshot, err := marshalCorrectionSnapshot(map[string]interface{}{
		"purchase_code":           first.purchase.Code,
		"purchase_date":           first.purchase.Date,
		"vendor":                  first.purchase.Vendor.Name,
		"original_journal_id":     first.originalJournal.ID,
		"original_journal_number": first.originalJournal.EntryNumber,
		"original_journal_date":   first.originalJournal.EntryDate,
		"line_count":              len(lineSnapshots),
		"lines":                   lineSnapshots,
	})
	if err != nil {
		return nil, nil, err
	}

	return &postedCorrectionTarget{
		targetType:        models.PostedTransactionCorrectionTargetPurchase,
		targetCode:        first.purchase.Code,
		purchaseID:        &first.purchase.ID,
		originalJournalID: first.originalJournal.ID,
		sourceSnapshot:    snapshot,
	}, dataRows, nil
}

func (s *PostedTransactionCorrectionService) ensureNoActivePurchaseCOAReclassification(
	tx *gorm.DB,
	originalJournalLineIDs []uint64,
) error {
	var legacyCount int64
	if err := tx.Model(&models.PostedTransactionCorrection{}).
		Where("correction_type = ? AND original_journal_line_id IN ? AND status IN ?", models.PostedTransactionCorrectionTypeCOAReclassification, originalJournalLineIDs, activeCorrectionStatuses()).
		Count(&legacyCount).Error; err != nil {
		return err
	}
	if legacyCount > 0 {
		return fmt.Errorf("masih ada pengajuan koreksi COA aktif untuk salah satu baris jurnal yang dipilih")
	}

	var batchCount int64
	if err := tx.Model(&models.PostedTransactionCorrectionReclassificationLine{}).
		Joins("JOIN posted_transaction_corrections correction ON correction.id = posted_transaction_correction_reclassification_lines.correction_id").
		Where("correction.deleted_at IS NULL AND correction.correction_type = ? AND correction.status IN ? AND posted_transaction_correction_reclassification_lines.original_journal_line_id IN ?", models.PostedTransactionCorrectionTypeCOAReclassification, activeCorrectionStatuses(), originalJournalLineIDs).
		Count(&batchCount).Error; err != nil {
		return err
	}
	if batchCount > 0 {
		return fmt.Errorf("masih ada pengajuan koreksi COA aktif untuk salah satu baris jurnal yang dipilih")
	}
	return nil
}

func loadPurchaseCOAReclassificationData(
	tx *gorm.DB,
	purchaseID uint,
	originalJournalLineID uint64,
	proposedAccountID uint,
	lock bool,
) (*purchaseCOAReclassificationData, error) {
	if tx == nil || purchaseID == 0 || originalJournalLineID == 0 || proposedAccountID == 0 {
		return nil, fmt.Errorf("pembelian, baris jurnal asal, dan COA pengganti wajib dipilih")
	}

	purchase, journal, err := loadPostedPurchaseJournalForCOAReclassification(tx, purchaseID, lock)
	if err != nil {
		return nil, err
	}

	lineQuery := tx.Preload("Account").Where("id = ? AND journal_id = ?", originalJournalLineID, journal.ID)
	if lock {
		lineQuery = lineQuery.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var originalLine models.SSOTJournalLine
	if err := lineQuery.First(&originalLine).Error; err != nil {
		if errorsIsRecordNotFound(err) {
			return nil, fmt.Errorf("baris jurnal asal tidak ditemukan pada jurnal pembelian ini")
		}
		return nil, err
	}
	debit := originalLine.DebitAmount.Round(2)
	credit := originalLine.CreditAmount.Round(2)
	if (debit.GreaterThan(decimal.Zero) && credit.GreaterThan(decimal.Zero)) ||
		(!debit.GreaterThan(decimal.Zero) && !credit.GreaterThan(decimal.Zero)) {
		return nil, fmt.Errorf("baris jurnal asal harus memiliki tepat satu nilai debit atau kredit")
	}
	if originalLine.Account == nil {
		return nil, fmt.Errorf("akun asal pada jurnal tidak ditemukan")
	}
	originalAccount := *originalLine.Account
	isCashBank, err := isCashBankLinkedCorrectionAccount(tx, originalAccount.ID)
	if err != nil {
		return nil, err
	}
	if isCashBank {
		return nil, fmt.Errorf("akun kas/bank tidak dapat direklasifikasi melalui koreksi COA transaksi terposting")
	}

	proposedQuery := tx.Where("id = ?", proposedAccountID)
	if lock {
		proposedQuery = proposedQuery.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var proposedAccount models.Account
	if err := proposedQuery.First(&proposedAccount).Error; err != nil {
		if errorsIsRecordNotFound(err) {
			return nil, fmt.Errorf("COA pengganti tidak ditemukan")
		}
		return nil, err
	}
	if !proposedAccount.IsActive || proposedAccount.IsHeader {
		return nil, fmt.Errorf("COA pengganti harus aktif dan bukan akun header")
	}
	if proposedAccount.ID == originalAccount.ID {
		return nil, fmt.Errorf("COA pengganti harus berbeda dari akun asal")
	}
	isCashBank, err = isCashBankLinkedCorrectionAccount(tx, proposedAccount.ID)
	if err != nil {
		return nil, err
	}
	if isCashBank {
		return nil, fmt.Errorf("COA kas/bank tidak dapat dipilih sebagai akun pengganti")
	}

	return &purchaseCOAReclassificationData{
		purchase:        *purchase,
		originalJournal: *journal,
		originalLine:    originalLine,
		originalAccount: originalAccount,
		proposedAccount: proposedAccount,
	}, nil
}

func loadPostedPurchaseJournalForCOAReclassification(
	tx *gorm.DB,
	purchaseID uint,
	lock bool,
) (*models.Purchase, *models.SSOTJournalEntry, error) {
	purchaseQuery := tx.Preload("Vendor")
	if lock {
		purchaseQuery = purchaseQuery.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var purchase models.Purchase
	if err := purchaseQuery.First(&purchase, purchaseID).Error; err != nil {
		if errorsIsRecordNotFound(err) {
			return nil, nil, fmt.Errorf("pembelian tidak ditemukan")
		}
		return nil, nil, err
	}

	sourceID := uint64(purchase.ID)
	journalQuery := tx.Preload("Lines", func(db *gorm.DB) *gorm.DB {
		return db.Order("line_number ASC, id ASC")
	}).Preload("Lines.Account").
		Where("source_type = ? AND source_id = ? AND status = ? AND deleted_at IS NULL", models.SSOTSourceTypePurchase, sourceID, models.SSOTStatusPosted).
		Order("entry_date ASC, id ASC")
	if lock {
		journalQuery = journalQuery.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var journal models.SSOTJournalEntry
	if err := journalQuery.First(&journal).Error; err != nil {
		if errorsIsRecordNotFound(err) {
			return nil, nil, fmt.Errorf("pembelian ini belum memiliki jurnal pembelian terposting yang dapat direklasifikasi")
		}
		return nil, nil, err
	}
	return &purchase, &journal, nil
}

func isCashBankLinkedCorrectionAccount(tx *gorm.DB, accountID uint) (bool, error) {
	var count int64
	if err := tx.Model(&models.CashBank{}).
		Where("account_id = ? AND deleted_at IS NULL", accountID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// executePurchaseCOAReclassification writes a balanced adjustment journal only.
// It never updates CashBank, CashBankTransaction, Purchase, or the original
// journal. The original accounting remains immutable for audit purposes.
func (s *PostedTransactionCorrectionService) executePurchaseCOAReclassification(
	tx *gorm.DB,
	correction models.PostedTransactionCorrection,
	userID uint,
) (*models.SSOTJournalEntry, string, error) {
	if correction.TargetType != models.PostedTransactionCorrectionTargetPurchase ||
		correction.CorrectionType != models.PostedTransactionCorrectionTypeCOAReclassification ||
		correction.PurchaseID == nil || correction.OriginalJournalEntryID == 0 {
		return nil, "", fmt.Errorf("data koreksi COA transaksi terposting belum lengkap")
	}

	correctionLines, err := loadPurchaseCOAReclassificationLinesForExecution(tx, correction)
	if err != nil {
		return nil, "", err
	}

	dataRows := make([]*purchaseCOAReclassificationData, 0, len(correctionLines))
	for _, correctionLine := range correctionLines {
		data, err := loadPurchaseCOAReclassificationData(
			tx,
			*correction.PurchaseID,
			correctionLine.OriginalJournalLineID,
			correctionLine.ProposedAccountID,
			true,
		)
		if err != nil {
			return nil, "", err
		}
		if data.originalJournal.ID != correction.OriginalJournalEntryID ||
			data.originalAccount.ID != correctionLine.OriginalAccountID ||
			data.proposedAccount.ID != correctionLine.ProposedAccountID {
			return nil, "", fmt.Errorf("jurnal atau COA asal berubah dari snapshot koreksi; Accounting perlu membuat pengajuan baru")
		}
		if correctionLine.ID != 0 &&
			(data.originalLine.LineNumber != correctionLine.LineNumber ||
				data.originalLine.Description != correctionLine.Description ||
				!data.originalLine.DebitAmount.Round(2).Equal(correctionLine.DebitAmount.Round(2)) ||
				!data.originalLine.CreditAmount.Round(2).Equal(correctionLine.CreditAmount.Round(2))) {
			return nil, "", fmt.Errorf("jurnal atau COA asal berubah dari snapshot koreksi; Accounting perlu membuat pengajuan baru")
		}
		dataRows = append(dataRows, data)
	}

	lines := make([]JournalLineRequest, 0, len(dataRows)*2)
	totalAmount := decimal.Zero
	for _, data := range dataRows {
		amount := data.originalLine.DebitAmount.Round(2)
		wasDebit := amount.GreaterThan(decimal.Zero)
		if !wasDebit {
			amount = data.originalLine.CreditAmount.Round(2)
		}
		if !amount.GreaterThan(decimal.Zero) {
			return nil, "", fmt.Errorf("nominal baris jurnal asal tidak valid")
		}
		totalAmount = totalAmount.Add(amount)

		if wasDebit {
			lines = append(lines,
				JournalLineRequest{
					AccountID:    uint64(data.proposedAccount.ID),
					DebitAmount:  amount,
					CreditAmount: decimal.Zero,
					Description:  fmt.Sprintf("Reklasifikasi dari %s - %s", data.originalAccount.Code, data.originalAccount.Name),
				},
				JournalLineRequest{
					AccountID:    uint64(data.originalAccount.ID),
					DebitAmount:  decimal.Zero,
					CreditAmount: amount,
					Description:  fmt.Sprintf("Reklasifikasi ke %s - %s", data.proposedAccount.Code, data.proposedAccount.Name),
				},
			)
			continue
		}

		lines = append(lines,
			JournalLineRequest{
				AccountID:    uint64(data.originalAccount.ID),
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				Description:  fmt.Sprintf("Reklasifikasi dari %s - %s", data.originalAccount.Code, data.originalAccount.Name),
			},
			JournalLineRequest{
				AccountID:    uint64(data.proposedAccount.ID),
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				Description:  fmt.Sprintf("Reklasifikasi ke %s - %s", data.proposedAccount.Code, data.proposedAccount.Name),
			},
		)
	}

	first := dataRows[0]
	adjustmentOf := first.originalJournal.ID
	journal, err := NewUnifiedJournalService(tx).CreateJournalEntryWithTx(tx, &JournalEntryRequest{
		EntryDate:                 time.Now().UTC(),
		Reference:                 fmt.Sprintf("KOREKSI-COA-%s-%d", first.purchase.Code, correction.ID),
		Description:               fmt.Sprintf("Reklasifikasi %d baris COA transaksi terposting %s", len(dataRows), first.purchase.Code),
		Notes:                     fmt.Sprintf("Koreksi transaksi terposting #%d. FM menyetujui reklasifikasi %d baris jurnal pembelian. Jurnal asal #%d tidak diubah dan tidak ada mutasi kas/bank.", correction.ID, len(dataRows), first.originalJournal.ID),
		CreatedBy:                 uint64(userID),
		SourceType:                models.SSOTSourceTypeAdjustment,
		SourceID:                  uint64(first.purchase.ID),
		AdjustmentOf:              &adjustmentOf,
		AdjustmentCategoryCode:    models.JournalAdjustmentCategoryPostedCOAReclassification,
		RequestedByFinanceManager: true,
		AutoPost:                  true,
		SystemGenerated:           true,
		Lines:                     lines,
	})
	if err != nil {
		return nil, "", fmt.Errorf("gagal membuat jurnal reklasifikasi COA: %w", err)
	}
	if err := tx.Model(journal).Update("source_code", first.purchase.Code).Error; err != nil {
		return nil, "", err
	}

	completionNotes := fmt.Sprintf(
		"FM menyetujui reklasifikasi %d baris jurnal sebesar %s. Sistem membuat jurnal koreksi #%d yang menaut ke jurnal asal #%d. Jurnal asal serta mutasi kas/bank tidak diubah.",
		len(dataRows),
		formatCorrectionDecimal(totalAmount),
		journal.ID,
		first.originalJournal.ID,
	)
	return journal, completionNotes, nil
}

func loadPurchaseCOAReclassificationLinesForExecution(
	tx *gorm.DB,
	correction models.PostedTransactionCorrection,
) ([]models.PostedTransactionCorrectionReclassificationLine, error) {
	var correctionLines []models.PostedTransactionCorrectionReclassificationLine
	if err := tx.
		Where("correction_id = ?", correction.ID).
		Order("line_number ASC, id ASC").
		Find(&correctionLines).Error; err != nil {
		return nil, err
	}
	if len(correctionLines) > 0 {
		return correctionLines, nil
	}

	// Corrections created before the batch audit table retain their original
	// one-line fields, so they remain executable without data migration.
	if correction.OriginalJournalLineID == nil || correction.OriginalAccountID == nil || correction.ProposedAccountID == nil {
		return nil, fmt.Errorf("data koreksi COA transaksi terposting belum lengkap")
	}
	return []models.PostedTransactionCorrectionReclassificationLine{{
		CorrectionID:          correction.ID,
		OriginalJournalLineID: *correction.OriginalJournalLineID,
		OriginalAccountID:     *correction.OriginalAccountID,
		ProposedAccountID:     *correction.ProposedAccountID,
	}}, nil
}

func formatCorrectionDecimal(amount decimal.Decimal) string {
	return utils.FormatRupiah(amount.InexactFloat64())
}
