package middleware

import (
	"app-sistem-akuntansi/models"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequireAnyPermissionManagedDenialOverridesRoleGrant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newPermissionMiddlewareTestDB(t)
	if err := db.AutoMigrate(&models.Permission{}, &models.RolePermission{}); err != nil {
		t.Fatalf("migrate legacy RBAC tables: %v", err)
	}

	permission := models.Permission{
		Name:     "purchases:update",
		Resource: "purchases",
		Action:   "update",
	}
	if err := db.Create(&permission).Error; err != nil {
		t.Fatalf("seed permission: %v", err)
	}
	if err := db.Create(&models.RolePermission{
		Role:         models.RoleAdmin,
		PermissionID: permission.ID,
	}).Error; err != nil {
		t.Fatalf("seed role permission: %v", err)
	}

	// The role grants update, but Manage Permissions explicitly denies edit.
	if err := db.Create(&models.ModulePermissionRecord{
		UserID:  1,
		Module:  "purchases",
		CanEdit: false,
	}).Error; err != nil {
		t.Fatalf("seed managed permission: %v", err)
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Set("role", models.RoleAdmin)
		c.Next()
	})
	router.PUT("/purchases/1", NewRBACManager(db).RequireAnyPermission("purchases:update"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/purchases/1", nil))

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected managed denial to override role grant, got status %d body %s", recorder.Code, recorder.Body.String())
	}
}

func TestRequireAnyPermissionManagedGrantOverridesRoleDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newPermissionMiddlewareTestDB(t)

	if err := db.Create(&models.ModulePermissionRecord{
		UserID:    2,
		Module:    "settings",
		CanDelete: true,
	}).Error; err != nil {
		t.Fatalf("seed managed permission: %v", err)
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(2))
		c.Set("role", models.RoleEmployee)
		c.Next()
	})
	router.DELETE("/settings/1", NewRBACManager(db).RequireAnyPermission("settings:delete"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/settings/1", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected managed grant to override role default, got status %d body %s", recorder.Code, recorder.Body.String())
	}
}
