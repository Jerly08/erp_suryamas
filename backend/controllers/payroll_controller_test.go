package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"app-sistem-akuntansi/models"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

func TestParsePayrollImportNumber(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected float64
	}{
		{name: "single comma thousands", input: "500,000", expected: 500000},
		{name: "single comma non round thousands", input: "100,287", expected: 100287},
		{name: "multiple comma thousands", input: "10,000,000", expected: 10000000},
		{name: "single dot thousands", input: "500.000", expected: 500000},
		{name: "multiple dot thousands", input: "1.234.567", expected: 1234567},
		{name: "indonesian decimal", input: "1.234,56", expected: 1234.56},
		{name: "international decimal", input: "1,234.56", expected: 1234.56},
		{name: "comma decimal without thousands", input: "150,50", expected: 150.50},
		{name: "dot decimal without thousands", input: "150.50", expected: 150.50},
		{name: "rupiah prefix", input: "Rp 250,000", expected: 250000},
		{name: "idr prefix", input: "IDR 100.287", expected: 100287},
		{name: "accounting negative", input: "(50,000)", expected: -50000},
		{name: "empty marker", input: "-", expected: 0},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual := parsePayrollImportNumber(testCase.input)
			if math.Abs(actual-testCase.expected) > 0.000001 {
				t.Fatalf("parsePayrollImportNumber(%q) = %v, expected %v", testCase.input, actual, testCase.expected)
			}
		})
	}
}

func TestImportPayrollPreviewPreservesFormattedExcelAmounts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	workbook := excelize.NewFile()
	defer workbook.Close()
	workbook.SetSheetName("Sheet1", "Salary")

	for columnIndex, header := range payrollImportTemplateHeaders {
		cell, err := excelize.CoordinatesToCellName(columnIndex+1, 1)
		if err != nil {
			t.Fatalf("failed to build header cell: %v", err)
		}
		if err := workbook.SetCellValue("Salary", cell, header); err != nil {
			t.Fatalf("failed to set header %q: %v", header, err)
		}
	}

	row := []interface{}{
		"2026-06-25", "26/05/2026 to 25/06/2026", 22, "KIS-001", "Head Office", "IT", "Payroll", "1122334455",
		"Budi Santoso", "Backend Developer", 10000000, 300000, 0, 500000, 250000,
		370000, 200000, 200000, 100287, 10000, 0, 30000, 0, 400000, 100000,
		0, 50000, 125000, 0,
	}
	for columnIndex, value := range row {
		cell, err := excelize.CoordinatesToCellName(columnIndex+1, 2)
		if err != nil {
			t.Fatalf("failed to build data cell: %v", err)
		}
		if err := workbook.SetCellValue("Salary", cell, value); err != nil {
			t.Fatalf("failed to set data cell %s: %v", cell, err)
		}
	}

	numberStyle, err := workbook.NewStyle(&excelize.Style{NumFmt: 3})
	if err != nil {
		t.Fatalf("failed to create thousands number style: %v", err)
	}
	if err := workbook.SetCellStyle("Salary", "K2", "AC2", numberStyle); err != nil {
		t.Fatalf("failed to apply thousands number style: %v", err)
	}

	var excelBuffer bytes.Buffer
	if err := workbook.Write(&excelBuffer); err != nil {
		t.Fatalf("failed to write test workbook: %v", err)
	}

	var requestBody bytes.Buffer
	multipartWriter := multipart.NewWriter(&requestBody)
	filePart, err := multipartWriter.CreateFormFile("file", "payroll.xlsx")
	if err != nil {
		t.Fatalf("failed to create multipart file: %v", err)
	}
	if _, err := filePart.Write(excelBuffer.Bytes()); err != nil {
		t.Fatalf("failed to write multipart file: %v", err)
	}
	if err := multipartWriter.Close(); err != nil {
		t.Fatalf("failed to close multipart body: %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/payroll/import/preview", &requestBody)
	ctx.Request.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	NewPayrollController(nil).ImportPayrollPreview(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data models.PayrollImportPreviewResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode preview response: %v", err)
	}
	if len(response.Data.Items) != 1 {
		t.Fatalf("expected one imported row, got %d", len(response.Data.Items))
	}

	item := response.Data.Items[0]
	if response.Data.SuggestedPaymentDate != "2026-06-25" {
		t.Fatalf("suggested payment date = %q, expected 2026-06-25", response.Data.SuggestedPaymentDate)
	}
	if response.Data.Summary.TotalEmployeeLoanDeduction != 125000 {
		t.Fatalf("summary employee loan deduction = %.2f, expected 125000", response.Data.Summary.TotalEmployeeLoanDeduction)
	}
	if response.Data.Summary.TotalBPJSEmployer != 1010000 {
		t.Fatalf("summary BPJS employer = %.2f, expected 1010000", response.Data.Summary.TotalBPJSEmployer)
	}
	if response.Data.Summary.TotalBPJSEmployee != 400287 {
		t.Fatalf("summary BPJS employee = %.2f, expected 400287", response.Data.Summary.TotalBPJSEmployee)
	}
	if response.Data.SuggestedPeriodStart != "2026-05-26" || response.Data.SuggestedPeriodEnd != "2026-06-25" {
		t.Fatalf("suggested period = %q to %q, expected 2026-05-26 to 2026-06-25", response.Data.SuggestedPeriodStart, response.Data.SuggestedPeriodEnd)
	}
	assertions := []struct {
		name     string
		actual   float64
		expected float64
	}{
		{name: "base salary", actual: item.BaseSalary, expected: 10000000},
		{name: "attendance allowance", actual: item.AttendanceAllowance, expected: 300000},
		{name: "bonus", actual: item.Bonus, expected: 800000},
		{name: "overtime", actual: item.Overtime, expected: 500000},
		{name: "pph", actual: item.PPh, expected: 250000},
		{name: "bpjs jht employer", actual: item.BPJSTKJHTEmployer, expected: 370000},
		{name: "bpjs jht employee", actual: item.BPJSTKJHTEmployee, expected: 200000},
		{name: "bpjs jp employer", actual: item.BPJSTKJPEmployer, expected: 200000},
		{name: "bpjs jp employee", actual: item.BPJSTKJPEmployee, expected: 100287},
		{name: "bpjs jkk employer", actual: item.BPJSTKJKKEmployer, expected: 10000},
		{name: "bpjs jkm employer", actual: item.BPJSTKJKMEmployer, expected: 30000},
		{name: "bpjs kesehatan employer", actual: item.BPJSKesehatanEmployer, expected: 400000},
		{name: "bpjs kesehatan employee", actual: item.BPJSKesehatanEmployee, expected: 100000},
		{name: "bpjs total", actual: item.BPJS, expected: 1410287},
		{name: "other deduction", actual: item.OtherDeduction, expected: 50000},
		{name: "employee loan deduction", actual: item.EmployeeLoanDeduction, expected: 125000},
		{name: "net pay", actual: item.NetPay, expected: 10224713},
	}
	for _, assertion := range assertions {
		if assertion.actual != assertion.expected {
			t.Errorf("%s = %v, expected %v", assertion.name, assertion.actual, assertion.expected)
		}
	}
}

func TestDownloadPayrollImportTemplateMatchesRequestedHeaderOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/payroll/import/template", nil)

	NewPayrollController(nil).DownloadPayrollImportTemplate(ctx)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	workbook, err := excelize.OpenReader(bytes.NewReader(recorder.Body.Bytes()))
	require.NoError(t, err)
	defer workbook.Close()

	rows, err := workbook.GetRows("Salary")
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	require.Equal(t, payrollImportTemplateHeaders, rows[0][:len(payrollImportTemplateHeaders)])

	highlightedPaymentDateStyle, err := workbook.GetCellStyle("Salary", "A1")
	require.NoError(t, err)
	standardHeaderStyle, err := workbook.GetCellStyle("Salary", "C1")
	require.NoError(t, err)
	highlightedJHTEmployerStyle, err := workbook.GetCellStyle("Salary", "P1")
	require.NoError(t, err)
	highlightedEmployeeLoanStyle, err := workbook.GetCellStyle("Salary", "AB1")
	require.NoError(t, err)
	require.NotEqual(t, standardHeaderStyle, highlightedPaymentDateStyle)
	require.NotEqual(t, standardHeaderStyle, highlightedJHTEmployerStyle)
	require.NotEqual(t, standardHeaderStyle, highlightedEmployeeLoanStyle)
	require.Equal(t, "DEDUCTION (PINJAMAN KARYAWAN)", rows[0][27])

	paymentViaExample, err := workbook.GetCellValue("Salary", "G2")
	require.NoError(t, err)
	require.Equal(t, "Payroll", paymentViaExample)
}

func TestDownloadPayrollRunExcelExportsSavedPayrollData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupPayrollApprovalTestDB(t)

	paymentDate := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	run := models.PayrollRun{
		PayrollNumber:     "PAY/2026/07/0001",
		PeriodStart:       time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC),
		PeriodEnd:         paymentDate,
		PaymentDate:       &paymentDate,
		Status:            models.PayrollStatusPendingApproval,
		CreatedByID:       1,
		TotalSalary:       10000000,
		TotalBonus:        500000,
		TotalPPh:          250000,
		TotalBPJS:         850000,
		TotalBPJSEmployer: 600000,
		TotalBPJSEmployee: 250000,
		TotalOverall:      10000000,
		Items: []models.PayrollItem{
			{
				EmployeeName:          "Budi Santoso",
				EmployeeCode:          "EMP-001",
				WorkingDay:            22,
				WorkUnit:              "Head Office",
				PaymentVia:            "Payroll",
				AccountNo:             "1122334455",
				Division:              "Finance",
				Position:              "Staff",
				BaseSalary:            10000000,
				AttendanceAllowance:   300000,
				CompensationPKWT:      0,
				Overtime:              200000,
				PPh:                   250000,
				BPJS:                  850000,
				BPJSTKJHTEmployer:     300000,
				BPJSTKJHTEmployee:     100000,
				BPJSTKJPEmployer:      100000,
				BPJSTKJPEmployee:      50000,
				BPJSTKJKKEmployer:     25000,
				BPJSTKJKMEmployer:     25000,
				BPJSKesehatanEmployer: 150000,
				BPJSKesehatanEmployee: 100000,
				OtherDeduction:        250000,
				EmployeeLoanDeduction: 0,
				NetPay:                10000000,
			},
		},
	}
	require.NoError(t, db.Create(&run).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(run.ID), 10)}}
	ctx.Set("role", models.RoleFinanceManager)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/payroll/"+strconv.FormatUint(uint64(run.ID), 10)+"/export/excel", nil)

	NewPayrollController(db).DownloadPayrollRunExcel(ctx)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Header().Get("Content-Disposition"), "PAY-2026-07-0001_pengajuan.xlsx")

	workbook, err := excelize.OpenReader(bytes.NewReader(recorder.Body.Bytes()))
	require.NoError(t, err)
	defer workbook.Close()

	employeeName, err := workbook.GetCellValue("Salary", "I2")
	require.NoError(t, err)
	require.Equal(t, "Budi Santoso", employeeName)

	baseSalary, err := workbook.GetCellValue("Salary", "K2", excelize.Options{RawCellValue: true})
	require.NoError(t, err)
	require.Equal(t, "10000000", baseSalary)

	jhtEmployer, err := workbook.GetCellValue("Salary", "P2", excelize.Options{RawCellValue: true})
	require.NoError(t, err)
	require.Equal(t, "300000", jhtEmployer)

	jhtEmployee, err := workbook.GetCellValue("Salary", "Q2", excelize.Options{RawCellValue: true})
	require.NoError(t, err)
	require.Equal(t, "100000", jhtEmployee)

	netPay, err := workbook.GetCellValue("Salary", "AC2", excelize.Options{RawCellValue: true})
	require.NoError(t, err)
	require.Equal(t, "10000000", netPay)
}

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
		&models.CashBank{},
		&models.CashBankTransaction{},
		&models.PayrollSettlement{},
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

func TestApprovePayrollRunFinanceManagerCreatesPayrollAccrualJournal(t *testing.T) {
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
	bpjsExpense := models.Account{
		Code:     "6107",
		Name:     "BEBAN BPJS PEMBERI KERJA",
		Type:     models.AccountTypeExpense,
		Category: models.CategoryOperatingExpense,
		IsActive: true,
		IsHeader: false,
	}
	salaryPayable := models.Account{
		Code:     "2103-001",
		Name:     "UTANG GAJI",
		Type:     models.AccountTypeLiability,
		Category: models.CategoryCurrentLiability,
		IsActive: true,
		IsHeader: false,
	}
	deductionPayable := models.Account{
		Code:     "2103-002",
		Name:     "UTANG BPJS & POTONGAN PAYROLL",
		Type:     models.AccountTypeLiability,
		Category: models.CategoryCurrentLiability,
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
	if err := db.Create(&bpjsExpense).Error; err != nil {
		t.Fatalf("failed to create BPJS expense account: %v", err)
	}
	if err := db.Create(&salaryPayable).Error; err != nil {
		t.Fatalf("failed to create salary payable account: %v", err)
	}
	if err := db.Create(&deductionPayable).Error; err != nil {
		t.Fatalf("failed to create deduction payable account: %v", err)
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
		TotalBPJS:          500000,
		TotalBPJSEmployer:  350000,
		TotalBPJSEmployee:  150000,
		TotalOverall:       9850000,
		PPhPaymentStatus:   models.PayrollPPhPaymentStatusUnpaid,
		PPhRecognitionDate: &periodEnd,
		CreatedByID:        hrUser.ID,
		Items: []models.PayrollItem{
			{
				EmployeeName:          "Staff Test",
				BaseSalary:            10000000,
				PPh:                   300000,
				BPJS:                  500000,
				BPJSTKJHTEmployer:     250000,
				BPJSTKJHTEmployee:     100000,
				BPJSKesehatanEmployer: 100000,
				BPJSKesehatanEmployee: 50000,
				NetPay:                9850000,
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
	if journal.SourceType != "PAYROLL_ACCRUAL" {
		t.Fatalf("expected PAYROLL_ACCRUAL journal, got %s", journal.SourceType)
	}
	if journal.Status != models.SSOTStatusPosted {
		t.Fatalf("expected posted journal, got %s", journal.Status)
	}
	if len(journal.Lines) != 6 {
		t.Fatalf("expected 6 journal lines, got %d", len(journal.Lines))
	}

	var totalDebit, totalCredit float64
	for _, line := range journal.Lines {
		totalDebit += line.DebitAmount.InexactFloat64()
		totalCredit += line.CreditAmount.InexactFloat64()
	}
	if totalDebit != 10650000 || totalCredit != 10650000 {
		t.Fatalf("expected balanced payroll journal 10,650,000, got debit %.2f credit %.2f", totalDebit, totalCredit)
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
	assertLine(bpjsExpense, 350000, 0)
	assertLine(pphExpense, 300000, 0)
	assertLine(pphPayable, 0, 300000)
	assertLine(salaryPayable, 0, 9850000)
	assertLine(deductionPayable, 0, 500000)
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

func TestPayrollItemTotalsSeparatesEmployerBPJSFromEmployeeNetPayDeduction(t *testing.T) {
	item := models.PayrollItemCreateRequest{
		BaseSalary:            10000000,
		AttendanceAllowance:   300000,
		Overtime:              500000,
		BPJSTKJHTEmployer:     370000,
		BPJSTKJHTEmployee:     200000,
		BPJSTKJPEmployer:      200000,
		BPJSTKJPEmployee:      100000,
		BPJSTKJKKEmployer:     10000,
		BPJSTKJKMEmployer:     30000,
		BPJSKesehatanEmployer: 400000,
		BPJSKesehatanEmployee: 100000,
		OtherDeduction:        50000,
		EmployeeLoanDeduction: 125000,
	}

	payrollImportItemTotals(&item)

	require.Equal(t, float64(1410000), item.BPJS)
	require.Equal(t, float64(1010000), payrollCreateBPJSEmployerTotal(&item))
	require.Equal(t, float64(400000), payrollCreateBPJSEmployeeTotal(&item))
	require.Equal(t, float64(10225000), item.NetPay)
}

func TestValidatePayrollEmployeeLoanAccountOnlyAllowsEmployeeLoanAsset(t *testing.T) {
	validDefault := models.Account{
		Code:     "1107-001",
		Name:     "PINJAMAN KARYAWAN",
		Type:     models.AccountTypeAsset,
		IsActive: true,
	}
	validNamedAccount := models.Account{
		Code:     "1107-099",
		Name:     "PIUTANG PINJAMAN KARYAWAN",
		Type:     models.AccountTypeAsset,
		IsActive: true,
	}
	unrelatedAsset := models.Account{
		Code:     "1102-001",
		Name:     "PIUTANG USAHA",
		Type:     models.AccountTypeAsset,
		IsActive: true,
	}

	require.NoError(t, validatePayrollEmployeeLoanAccount(validDefault))
	require.NoError(t, validatePayrollEmployeeLoanAccount(validNamedAccount))
	require.Error(t, validatePayrollEmployeeLoanAccount(unrelatedAsset))
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

func TestPayrollAccountingPostsAccrualSalaryAndPPhOnDifferentDates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupPayrollApprovalTestDB(t)

	hrUser := models.User{Username: "hr-dates", Email: "hr-dates@example.com", Password: "password", Role: models.RoleHR, IsActive: true}
	financeManager := models.User{Username: "fm-dates", Email: "fm-dates@example.com", Password: "password", Role: models.RoleFinanceManager, IsActive: true}
	requireCreate := func(value interface{}) {
		t.Helper()
		if err := db.Create(value).Error; err != nil {
			t.Fatalf("failed to create fixture: %v", err)
		}
	}
	requireCreate(&hrUser)
	requireCreate(&financeManager)

	salaryExpense := models.Account{Code: "6101", Name: "BEBAN GAJI & TUNJANGAN", Type: models.AccountTypeExpense, Category: models.CategoryOperatingExpense, IsActive: true}
	pphExpense := models.Account{Code: "9101", Name: "BEBAN PAJAK PENGHASILAN", Type: models.AccountTypeExpense, Category: models.CategoryTaxExpense, IsActive: true}
	pphPayable := models.Account{Code: "2102-003", Name: "HUTANG PPH PASAL 21", Type: models.AccountTypeLiability, Category: models.CategoryCurrentLiability, IsActive: true}
	salaryPayable := models.Account{Code: "2103-001", Name: "UTANG GAJI", Type: models.AccountTypeLiability, Category: models.CategoryCurrentLiability, IsActive: true}
	deductionPayable := models.Account{Code: "2103-002", Name: "UTANG BPJS & POTONGAN PAYROLL", Type: models.AccountTypeLiability, Category: models.CategoryCurrentLiability, IsActive: true}
	employeeLoanAccount := models.Account{Code: "1107-001", Name: "PINJAMAN KARYAWAN", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, IsActive: true}
	bankGL := models.Account{Code: "1101-005", Name: "BANK - OCBC", Type: models.AccountTypeAsset, Category: models.CategoryCurrentAsset, IsActive: true}
	for _, account := range []*models.Account{&salaryExpense, &pphExpense, &pphPayable, &salaryPayable, &deductionPayable, &employeeLoanAccount, &bankGL} {
		requireCreate(account)
	}
	bank := models.CashBank{Code: "BANK-OCBC", Name: "BANK - OCBC", Type: models.CashBankTypeBank, AccountID: bankGL.ID, Currency: "IDR", Balance: 100000000, BookBalanceIDR: 100000000, LastExchangeRate: 1, IsActive: true, AllowDirectTransactions: true}
	requireCreate(&bank)
	openingBankTransaction := models.CashBankTransaction{
		CashBankID: bank.ID, ReferenceType: "OPENING_BALANCE", Amount: 100000000,
		Currency: "IDR", ExchangeRate: 1, BookAmountIDR: 100000000,
		BalanceAfter: 100000000, TransactionDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Notes: "Saldo awal test",
	}
	requireCreate(&openingBankTransaction)

	periodStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	plannedSalaryDate := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	run := models.PayrollRun{
		PayrollNumber: "PAY/2026/06/DIFF-DATES", PeriodStart: periodStart, PeriodEnd: periodEnd,
		PaymentDate: &plannedSalaryDate, Status: models.PayrollStatusPendingApproval,
		TotalSalary: 37500000, TotalBonus: 3700000, TotalPPh: 950000, TotalBPJS: 1480287,
		TotalEmployeeLoanDeduction: 750000, TotalOverall: 38919713, PPhPaymentStatus: models.PayrollPPhPaymentStatusUnpaid,
		SalaryPaymentStatus: models.PayrollSalaryPaymentStatusUnpaid, BPJSPaymentStatus: models.PayrollSalaryPaymentStatusUnpaid,
		PPhRecognitionDate: &periodEnd, CreatedByID: hrUser.ID,
	}
	requireCreate(&run)

	controller := NewPayrollController(db)
	call := func(path string, body string, handler func(*gin.Context)) *httptest.ResponseRecorder {
		t.Helper()
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(run.ID), 10)}}
		ctx.Set("role", models.RoleFinanceManager)
		ctx.Set("user_id", financeManager.ID)
		handler(ctx)
		return recorder
	}

	approveBody := fmt.Sprintf(`{"expense_account_id":%d,"posting_date":"2026-06-15"}`, salaryExpense.ID)
	approveRecorder := call("/api/v1/payroll/approve", approveBody, controller.ApprovePayrollRun)
	if approveRecorder.Code != http.StatusOK {
		t.Fatalf("approve expected 200, got %d: %s", approveRecorder.Code, approveRecorder.Body.String())
	}

	var approved models.PayrollRun
	require.NoError(t, db.First(&approved, run.ID).Error)
	require.NotNil(t, approved.PPhPayableJournalEntryID)
	var accrual models.SSOTJournalEntry
	require.NoError(t, db.Preload("Lines").First(&accrual, *approved.PPhPayableJournalEntryID).Error)
	require.Equal(t, "PAYROLL_ACCRUAL", accrual.SourceType)
	require.Equal(t, "2026-06-15", accrual.EntryDate.Format("2006-01-02"))
	require.NotNil(t, approved.PPhRecognitionDate)
	require.Equal(t, "2026-06-15", approved.PPhRecognitionDate.Format("2006-01-02"))
	require.NotNil(t, approved.EmployeeLoanAccountID)
	require.Equal(t, employeeLoanAccount.ID, *approved.EmployeeLoanAccountID)
	assertJournalLine := func(journal models.SSOTJournalEntry, accountID uint, debit, credit float64) {
		t.Helper()
		for _, line := range journal.Lines {
			if line.AccountID == uint64(accountID) {
				require.InDelta(t, debit, line.DebitAmount.InexactFloat64(), 0.01)
				require.InDelta(t, credit, line.CreditAmount.InexactFloat64(), 0.01)
				return
			}
		}
		t.Fatalf("journal %d missing account %d", journal.ID, accountID)
	}
	assertJournalLine(accrual, salaryExpense.ID, 41200000, 0)
	assertJournalLine(accrual, pphExpense.ID, 950000, 0)
	assertJournalLine(accrual, salaryPayable.ID, 0, 38919713)
	assertJournalLine(accrual, pphPayable.ID, 0, 950000)
	assertJournalLine(accrual, deductionPayable.ID, 0, 1530287)
	assertJournalLine(accrual, employeeLoanAccount.ID, 0, 750000)
	for _, line := range accrual.Lines {
		require.NotEqual(t, uint64(bankGL.ID), line.AccountID, "accrual must not touch bank")
	}

	salaryBody := fmt.Sprintf(`{"payment_date":"2026-06-20","amount":38919713,"cash_bank_id":%d,"reference":"SAL-JUN-2026"}`, bank.ID)
	salaryRecorder := call("/api/v1/payroll/pay-salary", salaryBody, controller.PayPayrollSalary)
	if salaryRecorder.Code != http.StatusOK {
		t.Fatalf("salary payment expected 200, got %d: %s", salaryRecorder.Code, salaryRecorder.Body.String())
	}

	bpjsBody := fmt.Sprintf(`{"payment_date":"2026-06-25","amount":1480287,"cash_bank_id":%d,"reference":"BPJS-JUN-2026"}`, bank.ID)
	bpjsRecorder := call("/api/v1/payroll/pay-bpjs", bpjsBody, controller.PayPayrollBPJS)
	if bpjsRecorder.Code != http.StatusOK {
		t.Fatalf("BPJS payment expected 200, got %d: %s", bpjsRecorder.Code, bpjsRecorder.Body.String())
	}

	pphBody := fmt.Sprintf(`{"payment_date":"2026-07-10","amount":950000,"cash_bank_id":%d,"reference":"CORETAX-JUL-2026"}`, bank.ID)
	pphRecorder := call("/api/v1/payroll/pay-pph", pphBody, controller.PayPayrollPPh)
	if pphRecorder.Code != http.StatusOK {
		t.Fatalf("PPh payment expected 200, got %d: %s", pphRecorder.Code, pphRecorder.Body.String())
	}

	var finalRun models.PayrollRun
	require.NoError(t, db.First(&finalRun, run.ID).Error)
	require.Equal(t, models.PayrollSalaryPaymentStatusPaid, finalRun.SalaryPaymentStatus)
	require.Equal(t, models.PayrollPPhPaymentStatusPaid, finalRun.PPhPaymentStatus)
	require.Equal(t, models.PayrollSalaryPaymentStatusPaid, finalRun.BPJSPaymentStatus)
	require.InDelta(t, 38919713, finalRun.SalaryPaidAmount, 0.01)
	require.InDelta(t, 1480287, finalRun.BPJSPaidAmount, 0.01)
	require.InDelta(t, 950000, finalRun.PPhPaidAmount, 0.01)

	var settlements []models.PayrollSettlement
	require.NoError(t, db.Where("payroll_run_id = ?", run.ID).Order("payment_date ASC").Find(&settlements).Error)
	require.Len(t, settlements, 3)
	require.Equal(t, models.PayrollSettlementTypeSalary, settlements[0].SettlementType)
	require.Equal(t, "2026-06-20", settlements[0].PaymentDate.Format("2006-01-02"))
	require.Equal(t, models.PayrollSettlementTypeBPJS, settlements[1].SettlementType)
	require.Equal(t, "2026-06-25", settlements[1].PaymentDate.Format("2006-01-02"))
	require.Equal(t, models.PayrollSettlementTypePPh21, settlements[2].SettlementType)
	require.Equal(t, "2026-07-10", settlements[2].PaymentDate.Format("2006-01-02"))

	var salaryJournal, bpjsJournal, pphJournal models.SSOTJournalEntry
	require.NoError(t, db.Preload("Lines").First(&salaryJournal, settlements[0].JournalEntryID).Error)
	require.NoError(t, db.Preload("Lines").First(&bpjsJournal, settlements[1].JournalEntryID).Error)
	require.NoError(t, db.Preload("Lines").First(&pphJournal, settlements[2].JournalEntryID).Error)
	assertJournalLine(salaryJournal, salaryPayable.ID, 38919713, 0)
	assertJournalLine(salaryJournal, bankGL.ID, 0, 38919713)
	assertJournalLine(bpjsJournal, deductionPayable.ID, 1480287, 0)
	assertJournalLine(bpjsJournal, bankGL.ID, 0, 1480287)
	assertJournalLine(pphJournal, pphPayable.ID, 950000, 0)
	assertJournalLine(pphJournal, bankGL.ID, 0, 950000)

	require.NoError(t, db.First(&bank, bank.ID).Error)
	require.InDelta(t, 58650000, bank.Balance, 0.01)

	duplicateBPJSRecorder := call("/api/v1/payroll/pay-bpjs", bpjsBody, controller.PayPayrollBPJS)
	require.Equal(t, http.StatusBadRequest, duplicateBPJSRecorder.Code)
	duplicateRecorder := call("/api/v1/payroll/pay-pph", pphBody, controller.PayPayrollPPh)
	require.Equal(t, http.StatusBadRequest, duplicateRecorder.Code)
}
