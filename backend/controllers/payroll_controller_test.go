package controllers

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupPayrollApprovalTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := "file:payroll_approval_" + strconv.FormatInt(time.Now().UnixNano(), 10) + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.Account{},
		&models.AccountingPeriod{},
		&models.PayrollRun{},
		&models.PayrollItem{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return db
}

func createClosedPeriod(t *testing.T, db *gorm.DB, startDate, endDate time.Time) {
	t.Helper()

	closedAt := endDate
	period := models.AccountingPeriod{
		StartDate:   startDate,
		EndDate:     endDate,
		Description: "Closed payroll guard test period",
		IsClosed:    true,
		ClosedAt:    &closedAt,
	}
	if err := db.Create(&period).Error; err != nil {
		t.Fatalf("failed to create closed period: %v", err)
	}
}

func TestApprovePayrollRunFinanceManagerCreatesPPhPayableJournal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupPayrollApprovalTestDB(t)

	hrUser := models.User{
		Username: "hr-test",
		Email:    "hr-test@example.com",
		Password: "password",
		Role:     models.RoleHR,
		IsActive: true,
	}
	financeManager := models.User{
		Username: "fm-test",
		Email:    "fm-test@example.com",
		Password: "password",
		Role:     models.RoleFinanceManager,
		IsActive: true,
	}
	if err := db.Create(&hrUser).Error; err != nil {
		t.Fatalf("failed to create HR user: %v", err)
	}
	if err := db.Create(&financeManager).Error; err != nil {
		t.Fatalf("failed to create finance manager user: %v", err)
	}

	salaryExpense := models.Account{
		Code:     "6101",
		Name:     "BEBAN GAJI & TUNJANGAN",
		Type:     models.AccountTypeExpense,
		Category: models.CategoryOperatingExpense,
		IsActive: true,
		IsHeader: false,
	}
	pphPayable := models.Account{
		Code:     "2102-003",
		Name:     "HUTANG PPH PASAL 21",
		Type:     models.AccountTypeLiability,
		Category: models.CategoryCurrentLiability,
		IsActive: true,
		IsHeader: false,
	}
	pphExpense := models.Account{
		Code:     "9101",
		Name:     "BEBAN PAJAK PENGHASILAN",
		Type:     models.AccountTypeExpense,
		Category: models.CategoryTaxExpense,
		IsActive: true,
		IsHeader: false,
	}
	cashAccount := models.Account{
		Code:     "1101-003",
		Name:     "BANK GIRO",
		Type:     models.AccountTypeAsset,
		Category: models.CategoryCurrentAsset,
		IsActive: true,
		IsHeader: false,
	}
	if err := db.Create(&salaryExpense).Error; err != nil {
		t.Fatalf("failed to create salary expense account: %v", err)
	}
	if err := db.Create(&pphPayable).Error; err != nil {
		t.Fatalf("failed to create PPh payable account: %v", err)
	}
	if err := db.Create(&pphExpense).Error; err != nil {
		t.Fatalf("failed to create PPh expense account: %v", err)
	}
	if err := db.Create(&cashAccount).Error; err != nil {
		t.Fatalf("failed to create cash account: %v", err)
	}

	periodStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)
	run := models.PayrollRun{
		PayrollNumber:      "PAY/2026/05/0001",
		PeriodStart:        periodStart,
		PeriodEnd:          periodEnd,
		Status:             models.PayrollStatusPendingApproval,
		TotalSalary:        10000000,
		TotalPPh:           300000,
		TotalOverall:       10000000,
		PPhPaymentStatus:   models.PayrollPPhPaymentStatusUnpaid,
		PPhRecognitionDate: &periodEnd,
		CreatedByID:        hrUser.ID,
		Items: []models.PayrollItem{
			{
				EmployeeName: "Staff Test",
				BaseSalary:   10000000,
				PPh:          300000,
				NetPay:       10000000,
			},
		},
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatalf("failed to create payroll run: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(run.ID), 10)}}
	ctx.Set("role", models.RoleFinanceManager)
	ctx.Set("user_id", financeManager.ID)

	NewPayrollController(db).ApprovePayrollRun(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var approved models.PayrollRun
	if err := db.First(&approved, run.ID).Error; err != nil {
		t.Fatalf("failed to reload payroll run: %v", err)
	}
	if approved.Status != models.PayrollStatusApproved {
		t.Fatalf("expected approved status, got %s", approved.Status)
	}
	if approved.ApprovedByID == nil || *approved.ApprovedByID != financeManager.ID {
		t.Fatalf("expected approved_by_id %d, got %#v", financeManager.ID, approved.ApprovedByID)
	}
	if approved.PPhPayableJournalEntryID == nil {
		t.Fatal("expected PPh payable journal ID to be set")
	}

	var journal models.SSOTJournalEntry
	if err := db.Preload("Lines").First(&journal, *approved.PPhPayableJournalEntryID).Error; err != nil {
		t.Fatalf("failed to load payroll PPh payable journal: %v", err)
	}
	if journal.SourceType != "PAYROLL_PPH_PAYABLE" {
		t.Fatalf("expected PAYROLL_PPH_PAYABLE journal, got %s", journal.SourceType)
	}
	if journal.Status != models.SSOTStatusPosted {
		t.Fatalf("expected posted journal, got %s", journal.Status)
	}
	if len(journal.Lines) != 4 {
		t.Fatalf("expected 4 journal lines, got %d", len(journal.Lines))
	}

	var totalDebit, totalCredit float64
	for _, line := range journal.Lines {
		totalDebit += line.DebitAmount.InexactFloat64()
		totalCredit += line.CreditAmount.InexactFloat64()
	}
	if totalDebit != 10300000 || totalCredit != 10300000 {
		t.Fatalf("expected balanced payroll journal 10,300,000, got debit %.2f credit %.2f", totalDebit, totalCredit)
	}

	linesByAccount := make(map[uint64]models.SSOTJournalLine)
	for _, line := range journal.Lines {
		linesByAccount[line.AccountID] = line
	}
	assertLine := func(account models.Account, expectedDebit float64, expectedCredit float64) {
		t.Helper()
		line, ok := linesByAccount[uint64(account.ID)]
		if !ok {
			t.Fatalf("expected journal line for account %s - %s", account.Code, account.Name)
		}
		debit := line.DebitAmount.InexactFloat64()
		credit := line.CreditAmount.InexactFloat64()
		if debit != expectedDebit || credit != expectedCredit {
			t.Fatalf(
				"expected account %s debit %.2f credit %.2f, got debit %.2f credit %.2f",
				account.Code,
				expectedDebit,
				expectedCredit,
				debit,
				credit,
			)
		}
	}
	assertLine(salaryExpense, 10000000, 0)
	assertLine(pphExpense, 300000, 0)
	assertLine(pphPayable, 0, 300000)
	assertLine(cashAccount, 0, 10000000)
}

func TestPayrollItemTotalsDoNotReduceNetPayByPPh21(t *testing.T) {
	item := models.PayrollItemCreateRequest{
		BaseSalary:     10000000,
		PPh:            300000,
		BPJSTKJHT:      200000,
		BPJSTKJP:       100000,
		BPJSKesehatan:  100000,
		OtherDeduction: 50000,
	}

	payrollImportItemTotals(&item)

	if item.BPJS != 400000 {
		t.Fatalf("expected BPJS total 400000, got %.2f", item.BPJS)
	}
	if item.NetPay != 9550000 {
		t.Fatalf("expected net pay 9550000 without reducing PPh21, got %.2f", item.NetPay)
	}
}

func TestCreatePayrollRunRejectsRangeOverlappingClosedPeriod(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupPayrollApprovalTestDB(t)

	hrUser := models.User{
		Username: "hr-closed-period",
		Email:    "hr-closed-period@example.com",
		Password: "password",
		Role:     models.RoleHR,
		IsActive: true,
	}
	if err := db.Create(&hrUser).Error; err != nil {
		t.Fatalf("failed to create HR user: %v", err)
	}

	createClosedPeriod(
		t,
		db,
		time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC),
	)

	body := `{
		"period_start":"2026-05-01",
		"period_end":"2026-05-31",
		"payment_date":"2026-05-31",
		"items":[
			{"employee_name":"Staff Closed","base_salary":1000000,"net_pay":1000000}
		]
	}`

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/payroll", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("role", models.RoleHR)
	ctx.Set("user_id", hrUser.ID)

	NewPayrollController(db).CreatePayrollRun(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "CLOSED_PERIOD:2026-05-10") {
		t.Fatalf("expected closed period error for overlap date, got: %s", recorder.Body.String())
	}

	var count int64
	if err := db.Model(&models.PayrollRun{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count payroll runs: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no payroll run to be created, got %d", count)
	}
}

func TestApprovePayrollRunRejectsClosedPeriodEvenWithoutJournalAmount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupPayrollApprovalTestDB(t)

	hrUser := models.User{
		Username: "hr-zero",
		Email:    "hr-zero@example.com",
		Password: "password",
		Role:     models.RoleHR,
		IsActive: true,
	}
	financeManager := models.User{
		Username: "fm-zero",
		Email:    "fm-zero@example.com",
		Password: "password",
		Role:     models.RoleFinanceManager,
		IsActive: true,
	}
	if err := db.Create(&hrUser).Error; err != nil {
		t.Fatalf("failed to create HR user: %v", err)
	}
	if err := db.Create(&financeManager).Error; err != nil {
		t.Fatalf("failed to create finance manager user: %v", err)
	}

	periodStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)
	createClosedPeriod(
		t,
		db,
		time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC),
	)

	run := models.PayrollRun{
		PayrollNumber:    "PAY/2026/05/ZERO",
		PeriodStart:      periodStart,
		PeriodEnd:        periodEnd,
		Status:           models.PayrollStatusPendingApproval,
		PPhPaymentStatus: models.PayrollPPhPaymentStatusPaid,
		CreatedByID:      hrUser.ID,
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatalf("failed to create payroll run: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/payroll/"+strconv.FormatUint(uint64(run.ID), 10)+"/approve", nil)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(run.ID), 10)}}
	ctx.Set("role", models.RoleFinanceManager)
	ctx.Set("user_id", financeManager.ID)

	NewPayrollController(db).ApprovePayrollRun(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "CLOSED_PERIOD:2026-05-10") {
		t.Fatalf("expected closed period error, got: %s", recorder.Body.String())
	}

	var persisted models.PayrollRun
	if err := db.First(&persisted, run.ID).Error; err != nil {
		t.Fatalf("failed to reload payroll run: %v", err)
	}
	if persisted.Status != models.PayrollStatusPendingApproval {
		t.Fatalf("expected payroll to remain pending approval, got %s", persisted.Status)
	}
}

func TestPayPayrollPPhRejectsClosedPaymentDate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupPayrollApprovalTestDB(t)

	hrUser := models.User{
		Username: "hr-pph",
		Email:    "hr-pph@example.com",
		Password: "password",
		Role:     models.RoleHR,
		IsActive: true,
	}
	financeManager := models.User{
		Username: "fm-pph",
		Email:    "fm-pph@example.com",
		Password: "password",
		Role:     models.RoleFinanceManager,
		IsActive: true,
	}
	if err := db.Create(&hrUser).Error; err != nil {
		t.Fatalf("failed to create HR user: %v", err)
	}
	if err := db.Create(&financeManager).Error; err != nil {
		t.Fatalf("failed to create finance manager user: %v", err)
	}

	createClosedPeriod(
		t,
		db,
		time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
	)

	run := models.PayrollRun{
		PayrollNumber:    "PAY/2026/05/PPH",
		PeriodStart:      time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:        time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC),
		Status:           models.PayrollStatusApproved,
		TotalPPh:         300000,
		PPhPaymentStatus: models.PayrollPPhPaymentStatusUnpaid,
		CreatedByID:      hrUser.ID,
		ApprovedByID:     &financeManager.ID,
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatalf("failed to create payroll run: %v", err)
	}

	body := `{"payment_date":"2026-06-15","amount":100000,"cash_bank_id":1}`
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/payroll/"+strconv.FormatUint(uint64(run.ID), 10)+"/pay-pph", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(run.ID), 10)}}
	ctx.Set("role", models.RoleFinanceManager)
	ctx.Set("user_id", financeManager.ID)

	NewPayrollController(db).PayPayrollPPh(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "CLOSED_PERIOD:2026-06-15") {
		t.Fatalf("expected closed period error for payment date, got: %s", recorder.Body.String())
	}
}
