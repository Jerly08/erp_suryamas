package services

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCashBankTransferFXTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.ModulePermissionRecord{},
		&models.Account{},
		&models.AccountingPeriod{},
		&models.CashBank{},
		&models.CashBankTransaction{},
		&CashBankTransfer{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
	))
	ensureCashBankTransferApprovalColumns(t, db)

	return db
}

func TestCashBankTransferApprovalUsesManagedPermission(t *testing.T) {
	db := setupCashBankTransferFXTestDB(t)
	service := NewCashBankService(
		db,
		repositories.NewCashBankRepository(db),
		repositories.NewAccountRepository(db),
	)

	manager := models.User{
		Username: "managed-cashbank-manager",
		Email:    "managed-cashbank-manager@test.local",
		Password: "secret",
		Role:     models.RoleFinanceManager,
		IsActive: true,
	}
	require.NoError(t, db.Create(&manager).Error)
	require.NoError(t, db.Create(&models.ModulePermissionRecord{
		UserID:     manager.ID,
		Module:     "cash_bank",
		CanApprove: false,
	}).Error)

	allowed, err := service.canApproveCashBankTransfer(manager.ID, manager.Role)
	require.NoError(t, err)
	require.False(t, allowed, "managed denial must override finance manager default")

	employee := models.User{
		Username: "managed-cashbank-employee",
		Email:    "managed-cashbank-employee@test.local",
		Password: "secret",
		Role:     models.RoleEmployee,
		IsActive: true,
	}
	require.NoError(t, db.Create(&employee).Error)
	require.NoError(t, db.Create(&models.ModulePermissionRecord{
		UserID:     employee.ID,
		Module:     "cash_bank",
		CanApprove: true,
	}).Error)

	allowed, err = service.canApproveCashBankTransfer(employee.ID, employee.Role)
	require.NoError(t, err)
	require.True(t, allowed, "managed grant must override employee default")
}

func TestGetTransfers_FinanceTeamSeesSharedHistoryWithoutApprovalRights(t *testing.T) {
	db := setupCashBankTransferFXTestDB(t)
	service := NewCashBankService(
		db,
		repositories.NewCashBankRepository(db),
		repositories.NewAccountRepository(db),
	)

	financeStaffOne := models.User{
		Username: "transfer-history-finance-staff-one",
		Email:    "transfer-history-finance-staff-one@test.local",
		Password: "secret",
		Role:     models.RoleFinanceStaff,
		IsActive: true,
	}
	financeStaffTwo := models.User{
		Username: "transfer-history-finance-staff-two",
		Email:    "transfer-history-finance-staff-two@test.local",
		Password: "secret",
		Role:     models.RoleFinanceStaff,
		IsActive: true,
	}
	accounting := models.User{
		Username: "transfer-history-accounting",
		Email:    "transfer-history-accounting@test.local",
		Password: "secret",
		Role:     models.RoleAccounting,
		IsActive: true,
	}
	employee := models.User{
		Username: "transfer-history-employee",
		Email:    "transfer-history-employee@test.local",
		Password: "secret",
		Role:     models.RoleEmployee,
		IsActive: true,
	}

	for _, user := range []*models.User{&financeStaffOne, &financeStaffTwo, &accounting, &employee} {
		require.NoError(t, db.Create(user).Error)
	}

	createTransfer := func(number string, userID uint) CashBankTransfer {
		transfer := CashBankTransfer{
			TransferNumber:   number,
			FromAccountID:    1,
			ToAccountID:      2,
			Date:             time.Now(),
			Amount:           100000,
			Status:           TransferStatusPendingApproval,
			UserID:           userID,
			ApprovalRequired: true,
		}
		require.NoError(t, db.Create(&transfer).Error)
		return transfer
	}

	firstTransfer := createTransfer("TRF-SHARED-001", financeStaffOne.ID)
	createTransfer("TRF-SHARED-002", accounting.ID)
	createTransfer("TRF-SHARED-003", employee.ID)

	financeStaffHistory, err := service.GetTransfers(financeStaffTwo.ID, financeStaffTwo.Role, "", 20)
	require.NoError(t, err)
	require.Len(t, financeStaffHistory, 3)

	accountingHistory, err := service.GetTransfers(accounting.ID, accounting.Role, "", 20)
	require.NoError(t, err)
	require.Len(t, accountingHistory, 3)

	employeeHistory, err := service.GetTransfers(employee.ID, employee.Role, "", 20)
	require.NoError(t, err)
	require.Len(t, employeeHistory, 1)
	require.Equal(t, "TRF-SHARED-003", employeeHistory[0].TransferNumber)

	for _, user := range []models.User{financeStaffTwo, accounting} {
		canApprove, err := service.canApproveCashBankTransfer(user.ID, user.Role)
		require.NoError(t, err)
		require.False(t, canApprove)

		_, err = service.ApproveTransfer(firstTransfer.ID, user.ID, user.Role)
		require.EqualError(t, err, "you do not have permission to approve transfers")

		_, err = service.RejectTransfer(firstTransfer.ID, user.ID, user.Role, "not authorized")
		require.EqualError(t, err, "you do not have permission to reject transfers")
	}
}

func ensureCashBankTransferApprovalColumns(t *testing.T, db *gorm.DB) {
	t.Helper()

	type sqliteColumn struct {
		Name string `gorm:"column:name"`
	}

	var columns []sqliteColumn
	require.NoError(t, db.Raw("PRAGMA table_info(cash_bank_transfers)").Scan(&columns).Error)

	existing := make(map[string]struct{}, len(columns))
	for _, column := range columns {
		existing[column.Name] = struct{}{}
	}

	requiredColumns := map[string]string{
		"source_book_amount_idr":      "REAL DEFAULT 0",
		"destination_book_amount_idr": "REAL DEFAULT 0",
	}

	for columnName, columnType := range requiredColumns {
		if _, found := existing[columnName]; found {
			continue
		}

		require.NoError(t, db.Exec(
			fmt.Sprintf("ALTER TABLE cash_bank_transfers ADD COLUMN %s %s", columnName, columnType),
		).Error)
	}
}

func seedTransferFXUserAndAccounts(t *testing.T, db *gorm.DB) (models.User, models.Account, models.Account, models.Account, models.Account) {
	t.Helper()

	user := models.User{
		Username:  "cashbank-fx",
		Email:     "cashbank-fx@test.local",
		Password:  "secret",
		Role:      "finance",
		FirstName: "Cashbank FX",
		IsActive:  true,
	}
	require.NoError(t, db.Create(&user).Error)

	sourceGL := createTestAccount(t, db, "1102-TRF-SRC", "Transfer Source", models.AccountTypeAsset, 200000)
	destGL := createTestAccount(t, db, "1102-TRF-DST", "Transfer Destination", models.AccountTypeAsset, 1000000)
	gainGL := createTestAccount(t, db, "7103", "Laba Selisih Kurs", models.AccountTypeRevenue, 0)
	lossGL := createTestAccount(t, db, "8103", "Rugi Selisih Kurs", models.AccountTypeExpense, 0)

	return user, sourceGL, destGL, gainGL, lossGL
}

func createCashBankForTransferTest(t *testing.T, db *gorm.DB, code, name, currency string, accountID uint, balance, bookBalanceIDR, rate float64) models.CashBank {
	t.Helper()

	cashBank := models.CashBank{
		Code:                    code,
		Name:                    name,
		Type:                    models.CashBankTypeBank,
		AccountID:               accountID,
		Currency:                currency,
		Balance:                 balance,
		BookBalanceIDR:          bookBalanceIDR,
		LastExchangeRate:        rate,
		IsActive:                true,
		AllowDirectTransactions: true,
	}
	require.NoError(t, db.Create(&cashBank).Error)

	if balance > 0 {
		exchangeRate := 1.0
		bookAmountIDR := balance
		if strings.ToUpper(strings.TrimSpace(currency)) != "IDR" {
			exchangeRate = rate
			bookAmountIDR = bookBalanceIDR
		}

		openingTx := models.CashBankTransaction{
			CashBankID:      cashBank.ID,
			ReferenceType:   TransactionTypeOpeningBalance,
			ReferenceID:     0,
			Amount:          balance,
			Currency:        currency,
			ExchangeRate:    exchangeRate,
			BookAmountIDR:   bookAmountIDR,
			BalanceAfter:    balance,
			TransactionDate: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
			Notes:           "Opening Balance",
		}
		require.NoError(t, db.Create(&openingTx).Error)
	}

	return cashBank
}

func fetchTransferJournalWithLines(t *testing.T, db *gorm.DB, sourceCode string) (models.SSOTJournalEntry, []models.SSOTJournalLine) {
	t.Helper()

	var journal models.SSOTJournalEntry
	require.NoError(t, db.Where("source_type = ? AND source_code = ?", models.SSOTSourceTypeTransfer, sourceCode).First(&journal).Error)

	var lines []models.SSOTJournalLine
	require.NoError(t, db.Where("journal_id = ?", journal.ID).Order("line_number ASC").Find(&lines).Error)

	return journal, lines
}

func decimalValue(value decimal.Decimal) float64 {
	result, _ := value.Float64()
	return result
}

func TestProcessTransfer_CNYToIDR_PostsRealizedForexGainToSSOTJournal(t *testing.T) {
	db := setupCashBankTransferFXTestDB(t)
	user, sourceGL, destGL, gainGL, _ := seedTransferFXUserAndAccounts(t, db)

	sourceCashBank := createCashBankForTransferTest(t, db, "BNK-CNY-001", "Bank CNY", "CNY", sourceGL.ID, 100, 200000, 2000)
	destCashBank := createCashBankForTransferTest(t, db, "BNK-IDR-001", "Bank IDR", "IDR", destGL.ID, 1000000, 0, 1)

	service := NewCashBankService(db, repositories.NewCashBankRepository(db), repositories.NewAccountRepository(db))
	request := TransferRequest{
		FromAccountID: sourceCashBank.ID,
		ToAccountID:   destCashBank.ID,
		Date:          CustomDate(time.Date(2026, time.April, 11, 0, 0, 0, 0, time.UTC)),
		Amount:        10,
		ExchangeRate:  2100,
		Reference:     "TRF-CNY-IDR-001",
		Notes:         "Transfer CNY ke IDR",
	}

	transfer, err := service.ProcessTransfer(request, user.ID, user.Role)
	require.NoError(t, err)
	require.NotNil(t, transfer)
	require.InDelta(t, 21000, transfer.ConvertedAmount, 0.01)
	require.InDelta(t, 20000, transfer.SourceBookAmountIDR, 0.01)
	require.InDelta(t, 21000, transfer.DestinationBookAmountIDR, 0.01)
	require.InDelta(t, 1000, transfer.ForexGainLossAmount, 0.01)
	require.Equal(t, "GAIN", transfer.ForexGainLossType)

	var reloadedSource models.CashBank
	require.NoError(t, db.First(&reloadedSource, sourceCashBank.ID).Error)
	require.InDelta(t, 90, reloadedSource.Balance, 0.01)
	require.InDelta(t, 180000, reloadedSource.BookBalanceIDR, 0.01)

	var reloadedDestination models.CashBank
	require.NoError(t, db.First(&reloadedDestination, destCashBank.ID).Error)
	require.InDelta(t, 1021000, reloadedDestination.Balance, 0.01)

	journal, lines := fetchTransferJournalWithLines(t, db, transfer.TransferNumber)
	require.Equal(t, models.SSOTStatusPosted, journal.Status)
	require.Len(t, lines, 3)
	require.InDelta(t, 21000, decimalValue(journal.TotalDebit), 0.01)
	require.InDelta(t, 21000, decimalValue(journal.TotalCredit), 0.01)

	require.Equal(t, uint64(destGL.ID), lines[0].AccountID)
	require.InDelta(t, 21000, decimalValue(lines[0].DebitAmount), 0.01)
	require.True(t, lines[0].CreditAmount.IsZero())

	require.Equal(t, uint64(sourceGL.ID), lines[1].AccountID)
	require.True(t, lines[1].DebitAmount.IsZero())
	require.InDelta(t, 20000, decimalValue(lines[1].CreditAmount), 0.01)

	require.Equal(t, uint64(gainGL.ID), lines[2].AccountID)
	require.True(t, lines[2].DebitAmount.IsZero())
	require.InDelta(t, 1000, decimalValue(lines[2].CreditAmount), 0.01)
}

func TestProcessTransfer_IDRToCNY_DoesNotPostRealizedForexGainLossLine(t *testing.T) {
	db := setupCashBankTransferFXTestDB(t)
	user, sourceGL, destGL, _, _ := seedTransferFXUserAndAccounts(t, db)

	sourceCashBank := createCashBankForTransferTest(t, db, "BNK-IDR-002", "Bank IDR", "IDR", sourceGL.ID, 500000, 0, 1)
	destCashBank := createCashBankForTransferTest(t, db, "BNK-CNY-002", "Bank CNY", "CNY", destGL.ID, 0, 0, 0)

	service := NewCashBankService(db, repositories.NewCashBankRepository(db), repositories.NewAccountRepository(db))
	request := TransferRequest{
		FromAccountID: sourceCashBank.ID,
		ToAccountID:   destCashBank.ID,
		Date:          CustomDate(time.Date(2026, time.April, 11, 0, 0, 0, 0, time.UTC)),
		Amount:        42000,
		ExchangeRate:  2100,
		Reference:     "TRF-IDR-CNY-001",
		Notes:         "Transfer IDR ke CNY",
	}

	transfer, err := service.ProcessTransfer(request, user.ID, user.Role)
	require.NoError(t, err)
	require.NotNil(t, transfer)
	require.InDelta(t, 20, transfer.ConvertedAmount, 0.01)
	require.InDelta(t, 42000, transfer.SourceBookAmountIDR, 0.01)
	require.InDelta(t, 42000, transfer.DestinationBookAmountIDR, 0.01)
	require.InDelta(t, 0, transfer.ForexGainLossAmount, 0.01)
	require.Equal(t, "", transfer.ForexGainLossType)

	var reloadedDestination models.CashBank
	require.NoError(t, db.First(&reloadedDestination, destCashBank.ID).Error)
	require.InDelta(t, 20, reloadedDestination.Balance, 0.01)
	require.InDelta(t, 42000, reloadedDestination.BookBalanceIDR, 0.01)
	require.InDelta(t, 2100, reloadedDestination.LastExchangeRate, 0.01)

	journal, lines := fetchTransferJournalWithLines(t, db, transfer.TransferNumber)
	require.Equal(t, models.SSOTStatusPosted, journal.Status)
	require.Len(t, lines, 2)
	require.InDelta(t, 42000, decimalValue(journal.TotalDebit), 0.01)
	require.InDelta(t, 42000, decimalValue(journal.TotalCredit), 0.01)
}

func TestProcessTransfer_FinanceStaffCreatesPendingApprovalRequestWithoutBalanceMutation(t *testing.T) {
	db := setupCashBankTransferFXTestDB(t)
	_, sourceGL, destGL, _, _ := seedTransferFXUserAndAccounts(t, db)

	requester := models.User{
		Username:  "finance-staff",
		Email:     "finance-staff@test.local",
		Password:  "secret",
		Role:      models.RoleFinanceStaff,
		FirstName: "Finance Staff",
		IsActive:  true,
	}
	require.NoError(t, db.Create(&requester).Error)

	sourceCashBank := createCashBankForTransferTest(t, db, "BNK-IDR-010", "Bank Operasional", "IDR", sourceGL.ID, 500000, 0, 1)
	destCashBank := createCashBankForTransferTest(t, db, "BNK-IDR-011", "Bank Cadangan", "IDR", destGL.ID, 100000, 0, 1)

	service := NewCashBankService(db, repositories.NewCashBankRepository(db), repositories.NewAccountRepository(db))
	request := TransferRequest{
		FromAccountID: sourceCashBank.ID,
		ToAccountID:   destCashBank.ID,
		Date:          CustomDate(time.Date(2026, time.April, 12, 0, 0, 0, 0, time.UTC)),
		Amount:        125000,
		Reference:     "TRF-PENDING-001",
		Notes:         "Butuh approval manager",
	}

	transfer, err := service.ProcessTransfer(request, requester.ID, requester.Role)
	require.NoError(t, err)
	require.NotNil(t, transfer)
	require.Equal(t, TransferStatusPendingApproval, transfer.Status)
	require.True(t, transfer.ApprovalRequired)

	var reloadedSource models.CashBank
	require.NoError(t, db.First(&reloadedSource, sourceCashBank.ID).Error)
	require.InDelta(t, 500000, reloadedSource.Balance, 0.01)

	var reloadedDestination models.CashBank
	require.NoError(t, db.First(&reloadedDestination, destCashBank.ID).Error)
	require.InDelta(t, 100000, reloadedDestination.Balance, 0.01)

	var transactions []models.CashBankTransaction
	require.NoError(t, db.Where("reference_type = ? AND reference_id = ?", TransactionTypeTransfer, transfer.ID).Find(&transactions).Error)
	require.Len(t, transactions, 0)

	var journalCount int64
	require.NoError(t, db.Model(&models.SSOTJournalEntry{}).
		Where("source_type = ? AND source_code = ?", models.SSOTSourceTypeTransfer, transfer.TransferNumber).
		Count(&journalCount).Error)
	require.EqualValues(t, 0, journalCount)
}

func TestProcessTransfer_AdminCreatesPendingApprovalRequestWithoutBalanceMutation(t *testing.T) {
	db := setupCashBankTransferFXTestDB(t)
	_, sourceGL, destGL, _, _ := seedTransferFXUserAndAccounts(t, db)

	requester := models.User{
		Username:  "admin-transfer",
		Email:     "admin-transfer@test.local",
		Password:  "secret",
		Role:      models.RoleAdmin,
		FirstName: "Admin Transfer",
		IsActive:  true,
	}
	require.NoError(t, db.Create(&requester).Error)

	sourceCashBank := createCashBankForTransferTest(t, db, "BNK-IDR-020", "Bank Operasional", "IDR", sourceGL.ID, 500000, 0, 1)
	destCashBank := createCashBankForTransferTest(t, db, "BNK-IDR-021", "Bank Cadangan", "IDR", destGL.ID, 100000, 0, 1)

	service := NewCashBankService(db, repositories.NewCashBankRepository(db), repositories.NewAccountRepository(db))
	request := TransferRequest{
		FromAccountID: sourceCashBank.ID,
		ToAccountID:   destCashBank.ID,
		Date:          CustomDate(time.Date(2026, time.April, 12, 0, 0, 0, 0, time.UTC)),
		Amount:        75000,
		Reference:     "TRF-PENDING-ADMIN-001",
		Notes:         "Admin tetap perlu approval finance manager",
	}

	transfer, err := service.ProcessTransfer(request, requester.ID, requester.Role)
	require.NoError(t, err)
	require.NotNil(t, transfer)
	require.Equal(t, TransferStatusPendingApproval, transfer.Status)
	require.True(t, transfer.ApprovalRequired)

	var reloadedSource models.CashBank
	require.NoError(t, db.First(&reloadedSource, sourceCashBank.ID).Error)
	require.InDelta(t, 500000, reloadedSource.Balance, 0.01)

	var reloadedDestination models.CashBank
	require.NoError(t, db.First(&reloadedDestination, destCashBank.ID).Error)
	require.InDelta(t, 100000, reloadedDestination.Balance, 0.01)
}

func TestApproveTransfer_AdminIsRejected(t *testing.T) {
	db := setupCashBankTransferFXTestDB(t)
	_, sourceGL, destGL, _, _ := seedTransferFXUserAndAccounts(t, db)

	requester := models.User{
		Username:  "finance-staff-approvals",
		Email:     "finance-staff-approvals@test.local",
		Password:  "secret",
		Role:      models.RoleFinanceStaff,
		FirstName: "Finance Staff",
		IsActive:  true,
	}
	admin := models.User{
		Username:  "admin-approver",
		Email:     "admin-approver@test.local",
		Password:  "secret",
		Role:      models.RoleAdmin,
		FirstName: "Admin Approver",
		IsActive:  true,
	}
	require.NoError(t, db.Create(&requester).Error)
	require.NoError(t, db.Create(&admin).Error)

	sourceCashBank := createCashBankForTransferTest(t, db, "BNK-IDR-030", "Bank Operasional", "IDR", sourceGL.ID, 500000, 0, 1)
	destCashBank := createCashBankForTransferTest(t, db, "BNK-IDR-031", "Bank Cadangan", "IDR", destGL.ID, 100000, 0, 1)

	service := NewCashBankService(db, repositories.NewCashBankRepository(db), repositories.NewAccountRepository(db))
	request := TransferRequest{
		FromAccountID: sourceCashBank.ID,
		ToAccountID:   destCashBank.ID,
		Date:          CustomDate(time.Date(2026, time.April, 13, 0, 0, 0, 0, time.UTC)),
		Amount:        80000,
		Reference:     "TRF-PENDING-ADMIN-APPROVE-001",
		Notes:         "Admin tidak boleh approve",
	}

	transfer, err := service.ProcessTransfer(request, requester.ID, requester.Role)
	require.NoError(t, err)

	approvedTransfer, err := service.ApproveTransfer(transfer.ID, admin.ID, admin.Role)
	require.Nil(t, approvedTransfer)
	require.EqualError(t, err, "you do not have permission to approve transfers")
}

func TestApproveTransfer_FinanceManagerApprovesPendingTransferAndMutatesBalances(t *testing.T) {
	db := setupCashBankTransferFXTestDB(t)
	_, sourceGL, destGL, _, _ := seedTransferFXUserAndAccounts(t, db)

	requester := models.User{
		Username:  "finance-staff-pending",
		Email:     "finance-staff-pending@test.local",
		Password:  "secret",
		Role:      models.RoleFinanceStaff,
		FirstName: "Finance Staff",
		IsActive:  true,
	}
	approver := models.User{
		Username:  "finance-manager-approver",
		Email:     "finance-manager-approver@test.local",
		Password:  "secret",
		Role:      models.RoleFinanceManager,
		FirstName: "Finance Manager",
		IsActive:  true,
	}
	require.NoError(t, db.Create(&requester).Error)
	require.NoError(t, db.Create(&approver).Error)

	sourceCashBank := createCashBankForTransferTest(t, db, "BNK-IDR-040", "Bank Operasional", "IDR", sourceGL.ID, 500000, 0, 1)
	destCashBank := createCashBankForTransferTest(t, db, "BNK-IDR-041", "Bank Cadangan", "IDR", destGL.ID, 100000, 0, 1)

	service := NewCashBankService(db, repositories.NewCashBankRepository(db), repositories.NewAccountRepository(db))
	request := TransferRequest{
		FromAccountID: sourceCashBank.ID,
		ToAccountID:   destCashBank.ID,
		Date:          CustomDate(time.Date(2026, time.April, 14, 0, 0, 0, 0, time.UTC)),
		Amount:        90000,
		Reference:     "TRF-PENDING-FM-APPROVE-001",
		Notes:         "Finance manager approve transfer",
	}

	transfer, err := service.ProcessTransfer(request, requester.ID, requester.Role)
	require.NoError(t, err)
	require.Equal(t, TransferStatusPendingApproval, transfer.Status)

	approvedTransfer, err := service.ApproveTransfer(transfer.ID, approver.ID, approver.Role)
	require.NoError(t, err)
	require.NotNil(t, approvedTransfer)
	require.Equal(t, TransferStatusCompleted, approvedTransfer.Status)
	require.NotNil(t, approvedTransfer.ApprovedBy)
	require.Equal(t, approver.ID, *approvedTransfer.ApprovedBy)
	require.NotNil(t, approvedTransfer.ApprovedAt)

	var reloadedSource models.CashBank
	require.NoError(t, db.First(&reloadedSource, sourceCashBank.ID).Error)
	require.InDelta(t, 410000, reloadedSource.Balance, 0.01)

	var reloadedDestination models.CashBank
	require.NoError(t, db.First(&reloadedDestination, destCashBank.ID).Error)
	require.InDelta(t, 190000, reloadedDestination.Balance, 0.01)

	var transactions []models.CashBankTransaction
	require.NoError(t, db.Where("reference_type = ? AND reference_id = ?", TransactionTypeTransfer, transfer.ID).Find(&transactions).Error)
	require.Len(t, transactions, 2)

	var journalCount int64
	require.NoError(t, db.Model(&models.SSOTJournalEntry{}).
		Where("source_type = ? AND source_code = ?", models.SSOTSourceTypeTransfer, transfer.TransferNumber).
		Count(&journalCount).Error)
	require.EqualValues(t, 1, journalCount)
}
