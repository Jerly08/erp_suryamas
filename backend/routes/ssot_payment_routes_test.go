package routes

import (
	"testing"

	"app-sistem-akuntansi/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSetupSSOTPaymentRoutesRegistersPaymentDocumentRoutesWithoutPeriodMiddlewareConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:ssot-payment-document-routes?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	engine := gin.New()
	protected := engine.Group("/api/v1")
	require.NotPanics(t, func() {
		SetupSSOTPaymentRoutes(protected, db, middleware.NewJWTManager(db))
	})

	routes := engine.Routes()
	require.True(t, hasRegisteredRoute(routes, "POST", "/api/v1/payments/ssot/:id/documents"))
	require.True(t, hasRegisteredRoute(routes, "GET", "/api/v1/payments/ssot/:id/documents"))
	require.True(t, hasRegisteredRoute(routes, "GET", "/api/v1/payments/ssot/documents/:document_id/download"))
}

func hasRegisteredRoute(routes []gin.RouteInfo, method, path string) bool {
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}
