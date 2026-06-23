package middleware

import (
	"app-sistem-akuntansi/models"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newPermissionMiddlewareTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		if strings.Contains(err.Error(), "requires cgo") {
			t.Skip("sqlite test skipped because this environment runs with CGO disabled")
		}
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&models.ModulePermissionRecord{}); err != nil {
		t.Fatalf("migrate module permissions: %v", err)
	}

	return db
}

func TestCheckModulePermissionUsesModulePermissionForPrepaidExpenses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newPermissionMiddlewareTestDB(t)

	if err := db.Create(&models.ModulePermissionRecord{
		UserID:  uint(1),
		Module:  "prepaid_expenses",
		CanView: true,
		CanEdit: true,
	}).Error; err != nil {
		t.Fatalf("seed prepaid permission: %v", err)
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Set("role", models.RoleFinanceManager)
		c.Next()
	})
	router.PUT("/prepaid", NewPermissionMiddleware(db).CanEdit("prepaid_expenses"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/prepaid", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected prepaid edit permission to pass, got status %d body %s", recorder.Code, recorder.Body.String())
	}
}

func TestCheckModulePermissionUsesManagedJournalPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newPermissionMiddlewareTestDB(t)

	if err := db.Create(&models.ModulePermissionRecord{
		UserID:  uint(1),
		Module:  "journal",
		CanView: true,
		CanEdit: true,
	}).Error; err != nil {
		t.Fatalf("seed journal permission: %v", err)
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Set("role", models.RoleFinanceManager)
		c.Next()
	})
	router.PUT("/journal", NewPermissionMiddleware(db).CanEdit("journal"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/journal", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected managed journal edit permission to override role default, got status %d body %s", recorder.Code, recorder.Body.String())
	}
}

func TestPurchaseVendorQuickCreateUsesPurchasePermissionWithoutContactAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newPermissionMiddlewareTestDB(t)

	requirements := []models.ModulePermissionRecord{
		{UserID: uint(7), Module: "purchases", CanView: true, CanCreate: true},
		{UserID: uint(7), Module: "contacts", CanView: false, CanCreate: false},
	}
	if err := db.Create(&requirements).Error; err != nil {
		t.Fatalf("seed purchase/contact permissions: %v", err)
	}

	permissionMiddleware := NewPermissionMiddleware(db)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(7))
		c.Set("role", models.RoleFinanceStaff)
		c.Next()
	})
	router.POST("/purchases/vendors", permissionMiddleware.CanCreate("purchases"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.POST("/contacts", permissionMiddleware.CanCreate("contacts"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	purchaseRecorder := httptest.NewRecorder()
	router.ServeHTTP(purchaseRecorder, httptest.NewRequest(http.MethodPost, "/purchases/vendors", nil))
	if purchaseRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected Purchase quick-create to pass, got %d body %s", purchaseRecorder.Code, purchaseRecorder.Body.String())
	}

	contactRecorder := httptest.NewRecorder()
	router.ServeHTTP(contactRecorder, httptest.NewRequest(http.MethodPost, "/contacts", nil))
	if contactRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected Contact create to remain denied, got %d body %s", contactRecorder.Code, contactRecorder.Body.String())
	}
}

func TestCanUsePurchaseApprovalWorkflowUsesManagedPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newPermissionMiddlewareTestDB(t)

	tests := []struct {
		name       string
		role       string
		wantStatus int
	}{
		{name: "finance staff can approve active purchase workflow step", role: models.RoleFinanceStaff, wantStatus: http.StatusNoContent},
		{name: "finance manager can approve active purchase workflow step", role: models.RoleFinanceManager, wantStatus: http.StatusNoContent},
		{name: "director can approve escalated purchase workflow step", role: models.RoleDirector, wantStatus: http.StatusNoContent},
		{name: "admin follows default purchase approval permission", role: models.RoleAdmin, wantStatus: http.StatusNoContent},
		{name: "employee cannot approve purchase workflow", role: models.RoleEmployee, wantStatus: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Set("role", tt.role)
				c.Set("user_role", tt.role)
				c.Next()
			})
			router.POST("/purchases/1/approve", NewPermissionMiddleware(db).CanUsePurchaseApprovalWorkflow(), func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/purchases/1/approve", nil))

			if recorder.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d body %s", tt.wantStatus, recorder.Code, recorder.Body.String())
			}
		})
	}

	if err := db.Create(&models.ModulePermissionRecord{
		UserID:     uint(99),
		Module:     "purchases",
		CanApprove: false,
	}).Error; err != nil {
		t.Fatalf("seed managed purchase denial: %v", err)
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(99))
		c.Set("role", models.RoleFinanceManager)
		c.Next()
	})
	router.POST("/purchases/1/approve", NewPermissionMiddleware(db).CanUsePurchaseApprovalWorkflow(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/purchases/1/approve", nil))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected managed denial to override finance manager role, got %d body %s", recorder.Code, recorder.Body.String())
	}
}
