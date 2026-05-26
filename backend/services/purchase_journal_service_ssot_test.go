package services

import (
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/shopspring/decimal"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newPurchaseJournalSSOTTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite test unavailable: %v", err)
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.Settings{},
		&models.Account{},
		&models.CashBank{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
	); err != nil {
		t.Fatalf("auto migrate purchase ssot test db: %v", err)
	}

	user := models.User{
		ID:        1,
		Username:  "tester",
		Email:     "tester@example.com",
		Password:  "secret",
		Role:      "admin",
		FirstName: "Test",
		IsActive:  true,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed test user: %v", err)
	}

	return db
}

func createSSOTTestAccount(t *testing.T, db *gorm.DB, code, name, accountType, category string) models.Account {
	t.Helper()

	account := models.Account{
		Code:     code,
		Name:     name,
		Type:     accountType,
		Category: category,
		IsActive: true,
		Balance:  0,
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create test account %s: %v", code, err)
	}

	return account
}

func TestCreatePurchaseJournalWithPPh23UsesPayableCredit(t *testing.T) {
	db := newPurchaseJournalSSOTTestDB(t)
	service := NewPurchaseJournalServiceSSOT(db, nil, NewSettingsService(db))

	inventory := createSSOTTestAccount(t, db, "1103-001", "PERSEDIAAN BARANG DAGANG", models.AccountTypeAsset, models.CategoryCurrentAsset)
	ppnMasukan := createSSOTTestAccount(t, db, "2102-002", "PPN MASUKAN", models.AccountTypeLiability, models.CategoryCurrentLiability)
	bankGL := createSSOTTestAccount(t, db, "1101-005", "BANK - OCBC", models.AccountTypeAsset, models.CategoryCurrentAsset)
	pph23Payable := createSSOTTestAccount(t, db, "2102-004", "HUTANG PPH PASAL 23", models.AccountTypeLiability, models.CategoryCurrentLiability)

	cashBank := models.CashBank{
		Code:      "BANK-OCBC",
		Name:      "BANK - OCBC",
		Type:      models.CashBankTypeBank,
		AccountID: bankGL.ID,
		Currency:  "IDR",
		IsActive:  true,
	}
	if err := db.Create(&cashBank).Error; err != nil {
		t.Fatalf("create cash bank: %v", err)
	}

	productID := uint(1)
	purchaseID := uint(88)
	purchase := &models.Purchase{
		ID:                 purchaseID,
		Code:               "PO/TEST/PPH23",
		VendorID:           1,
		UserID:             1,
		Date:               time.Now(),
		NetBeforeTax:       12090909.09,
		PPNAmount:          1330000,
		PPh23Amount:        241818.18,
		TotalTaxDeductions: 241818.18,
		TotalAmount:        13179090.91,
		PaymentMethod:      models.PurchasePaymentTransfer,
		BankAccountID:      &cashBank.ID,
		Status:             models.PurchaseStatusApproved,
		Vendor:             models.Contact{Name: "CV Sumber Rejeki"},
		PurchaseItems: []models.PurchaseItem{
			{
				ID:         1,
				ProductID:  &productID,
				Quantity:   1,
				UnitPrice:  12090909.09,
				TotalPrice: 12090909.09,
				Product: &models.Product{
					ID:        productID,
					Name:      "Laptop Dell XPS 13",
					IsService: false,
				},
			},
		},
	}

	if err := service.CreatePurchaseJournal(purchase, nil); err != nil {
		t.Fatalf("create purchase journal: %v", err)
	}

	sourceID := uint64(purchaseID)
	var entry models.SSOTJournalEntry
	if err := db.Where("source_type = ? AND source_id = ?", models.SSOTSourceTypePurchase, sourceID).First(&entry).Error; err != nil {
		t.Fatalf("load purchase journal entry: %v", err)
	}
	if got := entry.TotalDebit.Round(2).StringFixed(2); got != "13420909.09" {
		t.Fatalf("expected debit 13420909.09, got %s", got)
	}
	if got := entry.TotalCredit.Round(2).StringFixed(2); got != "13420909.09" {
		t.Fatalf("expected credit 13420909.09, got %s", got)
	}

	var lines []models.SSOTJournalLine
	if err := db.Where("journal_id = ?", entry.ID).Find(&lines).Error; err != nil {
		t.Fatalf("load purchase journal lines: %v", err)
	}

	var bankLine, pphLine *models.SSOTJournalLine
	for i := range lines {
		switch lines[i].AccountID {
		case uint64(bankGL.ID):
			bankLine = &lines[i]
		case uint64(pph23Payable.ID):
			pphLine = &lines[i]
		}
	}
	if bankLine == nil {
		t.Fatalf("expected bank credit line for account %s", bankGL.Code)
	}
	if got := bankLine.CreditAmount.Round(2).StringFixed(2); got != "13179090.91" {
		t.Fatalf("expected bank credit 13179090.91, got %s", got)
	}
	if pphLine == nil {
		t.Fatalf("expected PPh23 payable credit line for account %s", pph23Payable.Code)
	}
	if !pphLine.DebitAmount.IsZero() {
		t.Fatalf("expected PPh23 debit to be zero, got %s", pphLine.DebitAmount.String())
	}
	if got := pphLine.CreditAmount.Round(2).StringFixed(2); got != "241818.18" {
		t.Fatalf("expected PPh23 credit 241818.18, got %s", got)
	}
	if inventory.ID == 0 || ppnMasukan.ID == 0 {
		t.Fatalf("test setup accounts were not created")
	}
}

func TestDeletePurchaseJournalRemovesLinkedReversalEntries(t *testing.T) {
	db := newPurchaseJournalSSOTTestDB(t)
	service := NewPurchaseJournalServiceSSOT(db, nil, nil)

	prepaid := createSSOTTestAccount(t, db, "1104-002", "SEWA DIBAYAR DIMUKA", models.AccountTypeAsset, models.CategoryCurrentAsset)
	ppnMasukan := createSSOTTestAccount(t, db, "2102-002", "PPN MASUKAN", models.AccountTypeLiability, models.CategoryCurrentLiability)
	bank := createSSOTTestAccount(t, db, "1101-003", "BANK - GIRO", models.AccountTypeAsset, models.CategoryCurrentAsset)

	purchaseSourceID := uint64(1)
	now := time.Now()
	postedBy := uint64(1)

	original := models.SSOTJournalEntry{
		EntryNumber:     "PURCHASE-1-1774103292",
		SourceType:      models.SSOTSourceTypePurchase,
		SourceID:        &purchaseSourceID,
		SourceCode:      "PO/2026/03/0001",
		EntryDate:       now,
		Description:     "Purchase Order #PO/2026/03/0001 - CV Sumber Rejeki",
		Reference:       "PO/2026/03/0001",
		TotalDebit:      decimal.RequireFromString("133200000"),
		TotalCredit:     decimal.RequireFromString("133200000"),
		Status:          models.SSOTStatusPosted,
		IsBalanced:      true,
		IsAutoGenerated: true,
		PostedAt:        &now,
		PostedBy:        &postedBy,
		CreatedBy:       1,
	}
	if err := db.Create(&original).Error; err != nil {
		t.Fatalf("create original journal: %v", err)
	}

	reversalSourceID := original.ID
	reversal := models.SSOTJournalEntry{
		EntryNumber:     "REV-1-20260324102525",
		SourceType:      models.SSOTSourceTypeReversal,
		SourceID:        &reversalSourceID,
		SourceCode:      "REV-PO/2026/03/0001",
		EntryDate:       now,
		Description:     "REVERSAL: Purchase Order #PO/2026/03/0001 - CV Sumber Rejeki",
		Reference:       "Reversal of PURCHASE-1-1774103292",
		TotalDebit:      decimal.RequireFromString("133200000"),
		TotalCredit:     decimal.RequireFromString("133200000"),
		Status:          models.SSOTStatusPosted,
		IsBalanced:      true,
		IsAutoGenerated: false,
		PostedAt:        &now,
		PostedBy:        &postedBy,
		ReversedFrom:    &original.ID,
		CreatedBy:       1,
	}
	if err := db.Create(&reversal).Error; err != nil {
		t.Fatalf("create reversal journal: %v", err)
	}

	if err := db.Model(&models.SSOTJournalEntry{}).
		Where("id = ?", original.ID).
		Update("reversed_by", reversal.ID).Error; err != nil {
		t.Fatalf("link original to reversal: %v", err)
	}

	lines := []models.SSOTJournalLine{
		{
			JournalID:    original.ID,
			AccountID:    uint64(prepaid.ID),
			LineNumber:   1,
			Description:  "Pembelian - SEWA KANTOR",
			DebitAmount:  decimal.RequireFromString("120000000"),
			CreditAmount: decimal.Zero,
		},
		{
			JournalID:    original.ID,
			AccountID:    uint64(ppnMasukan.ID),
			LineNumber:   2,
			Description:  "PPN Masukan - PO/2026/03/0001",
			DebitAmount:  decimal.RequireFromString("13200000"),
			CreditAmount: decimal.Zero,
		},
		{
			JournalID:    original.ID,
			AccountID:    uint64(bank.ID),
			LineNumber:   3,
			Description:  "Pembayaran Pembelian - PO/2026/03/0001",
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.RequireFromString("133200000"),
		},
		{
			JournalID:    reversal.ID,
			AccountID:    uint64(prepaid.ID),
			LineNumber:   1,
			Description:  "Reversal: Pembelian - SEWA KANTOR",
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.RequireFromString("120000000"),
		},
		{
			JournalID:    reversal.ID,
			AccountID:    uint64(ppnMasukan.ID),
			LineNumber:   2,
			Description:  "Reversal: PPN Masukan - PO/2026/03/0001",
			DebitAmount:  decimal.Zero,
			CreditAmount: decimal.RequireFromString("13200000"),
		},
		{
			JournalID:    reversal.ID,
			AccountID:    uint64(bank.ID),
			LineNumber:   3,
			Description:  "Reversal: Pembayaran Pembelian - PO/2026/03/0001",
			DebitAmount:  decimal.RequireFromString("133200000"),
			CreditAmount: decimal.Zero,
		},
	}
	if err := db.Create(&lines).Error; err != nil {
		t.Fatalf("create journal lines: %v", err)
	}

	if err := service.DeletePurchaseJournal(uint(purchaseSourceID), nil); err != nil {
		t.Fatalf("delete purchase journal chain: %v", err)
	}

	var remainingEntries int64
	if err := db.Model(&models.SSOTJournalEntry{}).Count(&remainingEntries).Error; err != nil {
		t.Fatalf("count remaining entries: %v", err)
	}
	if remainingEntries != 0 {
		t.Fatalf("expected all linked journal entries deleted, found %d", remainingEntries)
	}

	var remainingLines int64
	if err := db.Model(&models.SSOTJournalLine{}).Count(&remainingLines).Error; err != nil {
		t.Fatalf("count remaining lines: %v", err)
	}
	if remainingLines != 0 {
		t.Fatalf("expected all linked journal lines deleted, found %d", remainingLines)
	}

	var refreshedAccounts []models.Account
	if err := db.Order("code ASC").Find(&refreshedAccounts).Error; err != nil {
		t.Fatalf("reload accounts: %v", err)
	}
	for _, account := range refreshedAccounts {
		if account.Balance != 0 {
			t.Fatalf("expected account %s balance to remain neutral, got %.2f", account.Code, account.Balance)
		}
	}
}
