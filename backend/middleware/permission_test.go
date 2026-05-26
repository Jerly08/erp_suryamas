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

func TestCheckModulePermissionKeepsJournalWorkflowOverride(t *testing.T) {
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

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected journal edit to stay blocked by workflow override, got status %d", recorder.Code)
	}
}

func TestCanUsePurchaseApprovalWorkflowAllowsOnlyWorkflowRoles(t *testing.T) {
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
		{name: "admin cannot bypass purchase workflow", role: models.RoleAdmin, wantStatus: http.StatusForbidden},
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
}
