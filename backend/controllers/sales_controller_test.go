package controllers

import (
	"app-sistem-akuntansi/models"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMapSaleWriteErrorClassifiesRevenueCOAErrors(t *testing.T) {
	status, message, validationErrors, context, handled := mapSaleWriteError(errors.New("revenue account / COA item tidak valid untuk 'Traktor': akun COA pendapatan '4101' adalah header dan tidak memiliki akun detail aktif"))

	if !handled {
		t.Fatal("expected COA error to be handled")
	}
	if status != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, status)
	}
	if message != "Konfigurasi COA penjualan belum valid" {
		t.Fatalf("unexpected message: %s", message)
	}
	if validationErrors["revenue_account_id"] == "" {
		t.Fatalf("expected revenue_account_id validation detail, got %#v", validationErrors)
	}
	if context != nil {
		t.Fatalf("expected no business context, got %#v", context)
	}
}

func TestMapSaleWriteErrorClassifiesMissingFulfillmentSchema(t *testing.T) {
	status, message, validationErrors, context, handled := mapSaleWriteError(errors.New("failed to create sale: ERROR: column \"estimated_unit_cost\" of relation \"sale_items\" does not exist"))

	if !handled {
		t.Fatal("expected schema error to be handled")
	}
	if status != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, status)
	}
	if message != "Schema sales staging belum lengkap" {
		t.Fatalf("unexpected message: %s", message)
	}
	if validationErrors["schema"] == "" {
		t.Fatalf("expected schema validation detail, got %#v", validationErrors)
	}
	if context != nil {
		t.Fatalf("expected no business context, got %#v", context)
	}
}

func TestConvertConfirmedSaleToCreditRejectsNonFinanceManager(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/sales/1/convert-to-credit", nil)
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	ctx.Set("role", models.RoleFinanceStaff)

	(&SalesController{}).ConvertConfirmedSaleToCredit(ctx)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, recorder.Code, recorder.Body.String())
	}
}
