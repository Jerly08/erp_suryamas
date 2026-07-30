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
	ID           uint64  `json:"id"`
	LineNumber   int     `json:"line_number"`
	AccountID    uint    `json:"account_id"`
	AccountCode  string  `json:"account_code"`
	AccountName  string  `json:"account_name"`
	Description  string  `json:"description"`
	DebitAmount  float64 `json:"debit_amount"`
	CreditAmount float64 `json:"credit_amount"`
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
			ID:           line.ID,
			LineNumber:   line.LineNumber,
			AccountID:    line.Account.ID,
			AccountCode:  line.Account.Code,
			AccountName:  line.Account.Name,
			Description:  line.Description,
			DebitAmount:  debit.InexactFloat64(),
			CreditAmount: credit.InexactFloat64(),
		})
	}
	return result, nil
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

func (s *PostedTransactionCorrectionService) resolvePurchaseCOAReclassificationTarget(
	tx *gorm.DB,
	purchaseID uint,
	originalJournalLineID uint64,
	proposedAccountID uint,
	lock bool,
) (*postedCorrectionTarget, *purchaseCOAReclassificationData, error) {
	data, err := loadPurchaseCOAReclassificationData(tx, purchaseID, originalJournalLineID, proposedAccountID, lock)
	if err != nil {
		return nil, nil, err
	}

	snapshot, err := marshalCorrectionSnapshot(map[string]interface{}{
		"purchase_code":            data.purchase.Code,
		"purchase_date":            data.purchase.Date,
		"vendor":                   data.purchase.Vendor.Name,
		"original_journal_id":      data.originalJournal.ID,
		"original_journal_date":    data.originalJournal.EntryDate,
		"original_journal_line_id": data.originalLine.ID,
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
	if err != nil {
		return nil, nil, err
	}

	return &postedCorrectionTarget{
		targetType:        models.PostedTransactionCorrectionTargetPurchase,
		targetCode:        data.purchase.Code,
		purchaseID:        &data.purchase.ID,
		originalJournalID: data.originalJournal.ID,
		sourceSnapshot:    snapshot,
	}, data, nil
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
	journalQuery := tx.Preload("Lines.Account").
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
		correction.PurchaseID == nil || correction.OriginalJournalLineID == nil ||
		correction.OriginalAccountID == nil || correction.ProposedAccountID == nil {
		return nil, "", fmt.Errorf("data koreksi COA transaksi terposting belum lengkap")
	}

	data, err := loadPurchaseCOAReclassificationData(
		tx,
		*correction.PurchaseID,
		*correction.OriginalJournalLineID,
		*correction.ProposedAccountID,
		true,
	)
	if err != nil {
		return nil, "", err
	}
	if data.originalJournal.ID != correction.OriginalJournalEntryID || data.originalAccount.ID != *correction.OriginalAccountID {
		return nil, "", fmt.Errorf("jurnal atau COA asal berubah dari snapshot koreksi; Accounting perlu membuat pengajuan baru")
	}

	amount := data.originalLine.DebitAmount.Round(2)
	wasDebit := amount.GreaterThan(decimal.Zero)
	if !wasDebit {
		amount = data.originalLine.CreditAmount.Round(2)
	}
	if !amount.GreaterThan(decimal.Zero) {
		return nil, "", fmt.Errorf("nominal baris jurnal asal tidak valid")
	}

	lines := make([]JournalLineRequest, 0, 2)
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
	} else {
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

	adjustmentOf := data.originalJournal.ID
	journal, err := NewUnifiedJournalService(tx).CreateJournalEntryWithTx(tx, &JournalEntryRequest{
		EntryDate:                 time.Now().UTC(),
		Reference:                 fmt.Sprintf("KOREKSI-COA-%s-%d", data.purchase.Code, correction.ID),
		Description:               fmt.Sprintf("Reklasifikasi COA transaksi terposting %s", data.purchase.Code),
		Notes:                     fmt.Sprintf("Koreksi transaksi terposting #%d. FM menyetujui pemindahan %s (%s) ke %s (%s). Jurnal asal #%d tidak diubah dan tidak ada mutasi kas/bank.", correction.ID, data.originalAccount.Code, data.originalAccount.Name, data.proposedAccount.Code, data.proposedAccount.Name, data.originalJournal.ID),
		CreatedBy:                 uint64(userID),
		SourceType:                models.SSOTSourceTypeAdjustment,
		SourceID:                  uint64(data.purchase.ID),
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
	if err := tx.Model(journal).Update("source_code", data.purchase.Code).Error; err != nil {
		return nil, "", err
	}

	direction := "Debit"
	if !wasDebit {
		direction = "Kredit"
	}
	completionNotes := fmt.Sprintf(
		"FM menyetujui reklasifikasi %s %s (%s) ke %s (%s) sebesar %s. Sistem membuat jurnal koreksi #%d yang menaut ke jurnal asal #%d. Jurnal asal serta mutasi kas/bank tidak diubah.",
		direction,
		data.originalAccount.Code,
		data.originalAccount.Name,
		data.proposedAccount.Code,
		data.proposedAccount.Name,
		formatCorrectionDecimal(amount),
		journal.ID,
		data.originalJournal.ID,
	)
	return journal, completionNotes, nil
}

func formatCorrectionDecimal(amount decimal.Decimal) string {
	return utils.FormatRupiah(amount.InexactFloat64())
}
