package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/repositories"
	"app-sistem-akuntansi/services"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPurchaseAccountingMonitorAPIFiltersListAndSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Project{},
		&models.PurchaseRequest{},
		&models.Purchase{},
		&models.PurchaseItem{},
		&models.TaxAccountSettings{},
		&models.ApprovalWorkflow{},
		&models.ApprovalRequest{},
		&models.ApprovalStep{},
		&models.ApprovalAction{},
	))

	user := models.User{
		Username:  "api-accounting-monitor",
		Email:     "api-accounting-monitor@test.local",
		Password:  "secret",
		Role:      models.RoleAccounting,
		FirstName: "Accounting",
	}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&models.TaxAccountSettings{
		SalesReceivableAccountID:  1,
		SalesCashAccountID:        1,
		SalesBankAccountID:        1,
		SalesRevenueAccountID:     1,
		SalesOutputVATAccountID:   1,
		PurchasePayableAccountID:  1,
		PurchaseCashAccountID:     1,
		PurchaseBankAccountID:     1,
		PurchaseInputVATAccountID: 1,
		PurchaseExpenseAccountID:  1,
		IsActive:                  true,
		ApplyToAllCompanies:       true,
		UpdatedBy:                 user.ID,
	}).Error)

	vendor := models.Contact{
		Code: "VEND-API-MON",
		Name: "Vendor API Monitoring",
		Type: "VENDOR",
	}
	require.NoError(t, db.Create(&vendor).Error)

	purchases := []models.Purchase{
		{
			Code:           "PO-API-READY-COA",
			VendorID:       vendor.ID,
			UserID:         user.ID,
			Date:           time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
			Status:         models.PurchaseStatusPendingAccounting,
			ApprovalStatus: models.PurchaseApprovalApproved,
			TotalAmount:    100,
		},
		{
			Code:           "PO-API-APPROVED",
			VendorID:       vendor.ID,
			UserID:         user.ID,
			Date:           time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC),
			Status:         models.PurchaseStatusApproved,
			ApprovalStatus: models.PurchaseApprovalApproved,
			TotalAmount:    200,
		},
		{
			Code:           "PO-API-READY-END-DATE-NOON",
			VendorID:       vendor.ID,
			UserID:         user.ID,
			Date:           time.Date(2026, 5, 31, 15, 30, 0, 0, time.UTC),
			Status:         models.PurchaseStatusPendingAccounting,
			ApprovalStatus: models.PurchaseApprovalApproved,
			TotalAmount:    125,
		},
	}
	require.NoError(t, db.Create(&purchases).Error)

	purchaseRepo := repositories.NewPurchaseRepository(db)
	purchaseService := services.NewPurchaseService(
		db,
		purchaseRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	controller := NewPurchaseController(purchaseService, nil, db, nil)

	router := gin.New()
	router.GET("/purchases", controller.GetPurchases)
	router.GET("/purchases/summary", controller.GetPurchasesSummary)

	listReq := httptest.NewRequest(http.MethodGet, "/purchases?accounting_status=READY_FOR_COA&start_date=2026-05-01&end_date=2026-05-31&page=1&limit=10", nil)
	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, listReq)
	require.Equal(t, http.StatusOK, listRecorder.Code)

	var listResponse struct {
		Data  []models.Purchase `json:"data"`
		Total int64             `json:"total"`
	}
	require.NoError(t, json.Unmarshal(listRecorder.Body.Bytes(), &listResponse))
	require.Equal(t, int64(2), listResponse.Total)
	require.Len(t, listResponse.Data, 2)
	require.ElementsMatch(t, []string{"PO-API-READY-COA", "PO-API-READY-END-DATE-NOON"}, []string{listResponse.Data[0].Code, listResponse.Data[1].Code})

	summaryReq := httptest.NewRequest(http.MethodGet, "/purchases/summary?accounting_status=READY_FOR_COA&start_date=2026-05-01&end_date=2026-05-31", nil)
	summaryRecorder := httptest.NewRecorder()
	router.ServeHTTP(summaryRecorder, summaryReq)
	require.Equal(t, http.StatusOK, summaryRecorder.Code)

	var summary models.PurchaseSummary
	require.NoError(t, json.Unmarshal(summaryRecorder.Body.Bytes(), &summary))
	require.Equal(t, int64(2), summary.TotalPurchases)
	require.Equal(t, float64(225), summary.TotalAmount)
	require.Equal(t, float64(225), summary.TotalApprovedAmount)
}
