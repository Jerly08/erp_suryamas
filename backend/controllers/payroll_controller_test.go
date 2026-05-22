package controllers

import (
	"net/http"
	"net/http/httptest"
	"strconv"
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
		&models.PayrollRun{},
		&models.PayrollItem{},
		&models.SSOTJournalEntry{},
		&models.SSOTJournalLine{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return db
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
		TotalOverall:       9700000,
		PPhPaymentStatus:   models.PayrollPPhPaymentStatusUnpaid,
		PPhRecognitionDate: &periodEnd,
		CreatedByID:        hrUser.ID,
		Items: []models.PayrollItem{
			{
				EmployeeName: "Staff Test",
				BaseSalary:   10000000,
				PPh:          300000,
				NetPay:       9700000,
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
	if len(journal.Lines) != 3 {
		t.Fatalf("expected 3 journal lines, got %d", len(journal.Lines))
	}
}
