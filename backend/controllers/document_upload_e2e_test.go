package controllers

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/services"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newDocumentUploadTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Account{},
		&models.Contact{},
		&models.ProductCategory{},
		&models.WarehouseLocation{},
		&models.Product{},
		&models.Project{},
		&models.Purchase{},
		&models.PurchaseItem{},
		&models.PurchaseRequest{},
		&models.PurchaseRequestItem{},
		&models.PurchaseRequestDocument{},
		&models.PurchaseRequestRevisionSnapshot{},
		&models.PayrollRun{},
		&models.PayrollItem{},
		&models.PayrollDocument{},
		&models.PayrollSettlement{},
	))

	return db
}

func addPDFToMultipart(t *testing.T, writer *multipart.Writer, fieldName string) {
	t.Helper()

	part, err := writer.CreateFormFile(fieldName, "bukti-realisasi.pdf")
	require.NoError(t, err)
	_, err = part.Write([]byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n%%EOF"))
	require.NoError(t, err)
}

func cleanupUploadedTestFiles(t *testing.T, filePaths ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, filePath := range filePaths {
			if strings.HasPrefix(filePath, "/uploads/") {
				_ = os.Remove(strings.TrimPrefix(filePath, "/"))
			}
		}
	})
}

func TestPurchaseRequestRealizationDocumentUploadE2E(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newDocumentUploadTestDB(t)

	requester := models.User{
		Username:  "pic-realisasi",
		Email:     "pic-realisasi@example.com",
		Password:  "hashed",
		FirstName: "PIC",
		LastName:  "Requester",
		Role:      models.RoleEmployee,
		IsActive:  true,
	}
	require.NoError(t, db.Create(&requester).Error)

	pr := models.PurchaseRequest{
		PRNumber:                 "PR-E2E-REALIZATION",
		RequesterID:              requester.ID,
		Department:               "HR",
		Title:                    "Billing realisasi BPJS",
		TotalEstimatedAmount:     1200000,
		Status:                   models.PRStatusApproved,
		CurrentApprovalStage:     models.PRApprovalStageFinalized,
		ApprovalRevision:         1,
		FinanceStaffApprovedBy:   &requester.ID,
		FinanceManagerApprovedBy: &requester.ID,
		ApprovedBy:               &requester.ID,
		ApprovedAt:               ptrTime(time.Now()),
		Items: []models.PurchaseRequestItem{
			{
				ItemName:            "Billing BPJS",
				Quantity:            1,
				EstimatedPrice:      1200000,
				TotalEstimatedPrice: 1200000,
				ApprovalStatus:      models.PRItemDecisionApproved,
			},
		},
	}
	require.NoError(t, db.Create(&pr).Error)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	addPDFToMultipart(t, writer, "realization_documents")
	require.NoError(t, writer.Close())

	repo := repositories.NewPurchaseRequestRepository(db)
	service := services.NewPurchaseRequestService(db, repo, nil)
	controller := NewPurchaseRequestController(service)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: strconvUint(pr.ID)}}
	ctx.Set("user_id", requester.ID)
	ctx.Set("role", requester.Role)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/purchase-requests/"+strconvUint(pr.ID)+"/realization-documents", body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())

	controller.UploadRealizationDocuments(ctx)
	require.Equal(t, http.StatusCreated, recorder.Code, recorder.Body.String())

	var savedDocument models.PurchaseRequestDocument
	require.NoError(t, db.Where("purchase_request_id = ? AND document_type = ?", pr.ID, models.PurchaseRequestDocumentRealization).First(&savedDocument).Error)
	cleanupUploadedTestFiles(t, savedDocument.FilePath)
	require.Equal(t, "bukti-realisasi.pdf", savedDocument.FileName)

	detailRecorder := httptest.NewRecorder()
	detailCtx, _ := gin.CreateTestContext(detailRecorder)
	detailCtx.Params = gin.Params{{Key: "id", Value: strconvUint(pr.ID)}}
	detailCtx.Set("user_id", requester.ID)
	detailCtx.Set("role", requester.Role)
	detailCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/purchase-requests/"+strconvUint(pr.ID), nil)

	controller.GetByID(detailCtx)
	require.Equal(t, http.StatusOK, detailRecorder.Code, detailRecorder.Body.String())

	var detail models.PurchaseRequest
	require.NoError(t, json.Unmarshal(detailRecorder.Body.Bytes(), &detail))
	require.Len(t, detail.Documents, 1)
	require.Equal(t, models.PurchaseRequestDocumentRealization, detail.Documents[0].DocumentType)

	vendor := models.Contact{
		Code:     "VEND-E2E-REALIZATION",
		Name:     "Vendor Realisasi",
		Type:     models.ContactTypeVendor,
		IsActive: true,
	}
	require.NoError(t, db.Create(&vendor).Error)

	purchaseRequestID := pr.ID
	purchase := models.Purchase{
		Code:              "PO-E2E-REALIZATION",
		VendorID:          vendor.ID,
		UserID:            requester.ID,
		Date:              time.Now(),
		DueDate:           time.Now(),
		TotalAmount:       1200000,
		Status:            models.PurchaseStatusApproved,
		ApprovalStatus:    models.PurchaseApprovalApproved,
		PurchaseRequestID: &purchaseRequestID,
	}
	require.NoError(t, db.Create(&purchase).Error)

	purchaseRepo := repositories.NewPurchaseRepository(db)
	linkedPurchase, err := purchaseRepo.FindByID(purchase.ID)
	require.NoError(t, err)
	require.NotNil(t, linkedPurchase.PurchaseRequest)
	require.Len(t, linkedPurchase.PurchaseRequest.Documents, 1)
	require.Equal(t, models.PurchaseRequestDocumentRealization, linkedPurchase.PurchaseRequest.Documents[0].DocumentType)
	require.Equal(t, requester.Username, linkedPurchase.PurchaseRequest.Documents[0].Uploader.Username)
}

func TestPayrollBillingRealizationDocumentUploadE2E(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newDocumentUploadTestDB(t)

	hrUser := models.User{
		Username:  "hr-payroll-doc",
		Email:     "hr-payroll-doc@example.com",
		Password:  "hashed",
		FirstName: "HR",
		LastName:  "User",
		Role:      models.RoleHR,
		IsActive:  true,
	}
	require.NoError(t, db.Create(&hrUser).Error)

	run := models.PayrollRun{
		PayrollNumber: "PAYROLL-E2E-DOC",
		PeriodStart:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:     time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
		Status:        models.PayrollStatusPendingApproval,
		TotalSalary:   1000000,
		TotalOverall:  1000000,
		CreatedByID:   hrUser.ID,
	}
	require.NoError(t, db.Create(&run).Error)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	addPDFToMultipart(t, writer, "billing_documents")
	require.NoError(t, writer.Close())

	controller := NewPayrollController(db)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: strconvUint(run.ID)}}
	ctx.Set("user_id", hrUser.ID)
	ctx.Set("role", hrUser.Role)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/payroll/"+strconvUint(run.ID)+"/documents", body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())

	controller.UploadDocuments(ctx)
	require.Equal(t, http.StatusCreated, recorder.Code, recorder.Body.String())

	var savedDocument models.PayrollDocument
	require.NoError(t, db.Where("payroll_run_id = ?", run.ID).First(&savedDocument).Error)
	cleanupUploadedTestFiles(t, savedDocument.FilePath)
	require.Equal(t, models.PayrollDocumentBillingRealization, savedDocument.DocumentType)
	require.Equal(t, "bukti-realisasi.pdf", savedDocument.FileName)

	detailRecorder := httptest.NewRecorder()
	detailCtx, _ := gin.CreateTestContext(detailRecorder)
	detailCtx.Params = gin.Params{{Key: "id", Value: strconvUint(run.ID)}}
	detailCtx.Set("role", hrUser.Role)
	detailCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/payroll/"+strconvUint(run.ID), nil)

	controller.GetPayrollRun(detailCtx)
	require.Equal(t, http.StatusOK, detailRecorder.Code, detailRecorder.Body.String())

	var response struct {
		Data models.PayrollRunResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(detailRecorder.Body.Bytes(), &response))
	require.True(t, response.Data.DetailAllowed)
	require.Len(t, response.Data.Documents, 1)
	require.Equal(t, models.PayrollDocumentBillingRealization, response.Data.Documents[0].DocumentType)
}

func ptrTime(value time.Time) *time.Time {
	return &value
}

func strconvUint(value uint) string {
	return strconv.FormatUint(uint64(value), 10)
}
