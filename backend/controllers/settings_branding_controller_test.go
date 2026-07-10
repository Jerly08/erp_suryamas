package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPublicBrandingEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	services.ClearSettingsCache()

	db, err := gorm.Open(sqlite.Open("file:branding-endpoints?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Settings{}))
	require.NoError(t, db.Create(&models.Settings{
		CompanyName:      "PT Cabang Contoh",
		CompanyLogo:      "/uploads/company/logo.png",
		CompanyLogoScale: 140,
		CompanyIcon:      "/uploads/company/icon.png",
	}).Error)

	controller := NewSettingsController(services.NewSettingsService(db))
	router := gin.New()
	router.GET("/api/v1/branding/public", controller.GetPublicBranding)
	router.GET("/api/v1/branding/logo", controller.GetPublicBrandingLogo)
	router.GET("/api/v1/branding/icon", controller.GetPublicBrandingIcon)
	router.GET("/api/v1/branding/manifest.webmanifest", controller.GetPublicBrandingManifest)

	t.Run("returns public branding without authentication", func(t *testing.T) {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/branding/public", nil))

		require.Equal(t, http.StatusOK, response.Code)
		require.Equal(t, "no-store, max-age=0", response.Header().Get("Cache-Control"))

		var payload struct {
			Success bool `json:"success"`
			Data    struct {
				CompanyName      string `json:"company_name"`
				CompanyLogo      string `json:"company_logo"`
				CompanyLogoScale int    `json:"company_logo_scale"`
				CompanyIcon      string `json:"company_icon"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
		require.True(t, payload.Success)
		require.Equal(t, "PT Cabang Contoh", payload.Data.CompanyName)
		require.Equal(t, "/uploads/company/logo.png", payload.Data.CompanyLogo)
		require.Equal(t, 140, payload.Data.CompanyLogoScale)
		require.Equal(t, "/uploads/company/icon.png", payload.Data.CompanyIcon)
	})

	t.Run("redirects stable logo URL to uploaded logo", func(t *testing.T) {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/branding/logo", nil))

		require.Equal(t, http.StatusTemporaryRedirect, response.Code)
		require.Equal(t, "no-store, max-age=0", response.Header().Get("Cache-Control"))
		require.Equal(t, "/uploads/company/logo.png", response.Header().Get("Location"))
	})

	t.Run("redirects stable icon URL to uploaded company icon", func(t *testing.T) {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/branding/icon", nil))

		require.Equal(t, http.StatusTemporaryRedirect, response.Code)
		require.Equal(t, "/uploads/company/icon.png", response.Header().Get("Location"))
	})

	t.Run("returns dynamic web manifest", func(t *testing.T) {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/branding/manifest.webmanifest", nil))

		require.Equal(t, http.StatusOK, response.Code)
		require.True(t, strings.HasPrefix(response.Header().Get("Content-Type"), "application/manifest+json"))

		var payload struct {
			Name      string `json:"name"`
			ShortName string `json:"short_name"`
			Icons     []struct {
				Src     string `json:"src"`
				Sizes   string `json:"sizes"`
				Type    string `json:"type"`
				Purpose string `json:"purpose"`
			} `json:"icons"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
		require.Equal(t, "PT Cabang Contoh Finance Management", payload.Name)
		require.Equal(t, "PT Cabang Contoh", payload.ShortName)
		require.Len(t, payload.Icons, 3)
		require.Contains(t, payload.Icons[0].Src, "/api/v1/branding/icon?size=192")
		require.Contains(t, payload.Icons[0].Src, "source=manifest")
		require.Contains(t, payload.Icons[0].Src, "v=")
		require.Equal(t, "192x192", payload.Icons[0].Sizes)
		require.Equal(t, "image/png", payload.Icons[0].Type)
		require.Contains(t, payload.Icons[1].Src, "/api/v1/branding/icon?size=512")
		require.Contains(t, payload.Icons[1].Src, "source=manifest")
		require.Equal(t, "512x512", payload.Icons[1].Sizes)
		require.Contains(t, payload.Icons[2].Src, "/api/v1/branding/icon?size=512")
		require.Contains(t, payload.Icons[2].Src, "source=manifest")
		require.Equal(t, "any maskable", payload.Icons[2].Purpose)
	})
}

func TestPublicBrandingFallbacksAreTenantNeutral(t *testing.T) {
	gin.SetMode(gin.TestMode)
	services.ClearSettingsCache()

	db, err := gorm.Open(sqlite.Open("file:branding-fallbacks?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Settings{}))
	require.NoError(t, db.Create(&models.Settings{
		CompanyName: "Tenant Baru",
	}).Error)

	controller := NewSettingsController(services.NewSettingsService(db))
	router := gin.New()
	router.GET("/api/v1/branding/logo", controller.GetPublicBrandingLogo)
	router.GET("/api/v1/branding/icon", controller.GetPublicBrandingIcon)
	router.GET("/api/v1/branding/manifest.webmanifest", controller.GetPublicBrandingManifest)

	t.Run("logo uses a neutral placeholder when no upload exists", func(t *testing.T) {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/branding/logo", nil))

		require.Equal(t, http.StatusTemporaryRedirect, response.Code)
		require.Equal(t, fallbackBrandingLogoPath, response.Header().Get("Location"))
	})

	t.Run("browser icon uses a neutral placeholder when no upload exists", func(t *testing.T) {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/branding/icon", nil))

		require.Equal(t, http.StatusTemporaryRedirect, response.Code)
		require.Equal(t, fallbackBrowserIconPath, response.Header().Get("Location"))
	})

	t.Run("manifest uses a neutral placeholder when no icon upload exists", func(t *testing.T) {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/branding/manifest.webmanifest", nil))

		require.Equal(t, http.StatusOK, response.Code)

		var payload struct {
			Icons []struct {
				Src  string `json:"src"`
				Type string `json:"type"`
			} `json:"icons"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
		require.Len(t, payload.Icons, 1)
		require.Equal(t, fallbackBrowserIconPath, payload.Icons[0].Src)
		require.Equal(t, "image/svg+xml", payload.Icons[0].Type)
	})
}
