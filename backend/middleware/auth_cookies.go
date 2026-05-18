package middleware

import (
	"net/http"
	"strings"

	"app-sistem-akuntansi/config"
	"app-sistem-akuntansi/models"
	"github.com/gin-gonic/gin"
)

const (
	AccessTokenCookieName  = "erp_access_token"
	RefreshTokenCookieName = "erp_refresh_token"
)

var ignoredAuthorizationTokens = map[string]struct{}{
	"":                 {},
	"null":             {},
	"undefined":        {},
	"cookie-session":   {},
	"__cookie_session__": {},
}

func SetAuthCookies(c *gin.Context, cfg *config.Config, tokens *models.TokenResponse) {
	if cfg == nil || tokens == nil {
		return
	}

	sameSite := parseCookieSameSite(cfg.CookieSameSite)
	c.SetSameSite(sameSite)
	c.SetCookie(
		AccessTokenCookieName,
		tokens.AccessToken,
		int(cfg.JWTAccessExpiry.Seconds()),
		"/",
		cfg.CookieDomain,
		cfg.CookieSecure,
		cfg.CookieHTTPOnly,
	)
	c.SetCookie(
		RefreshTokenCookieName,
		tokens.RefreshToken,
		int(cfg.JWTRefreshExpiry.Seconds()),
		"/",
		cfg.CookieDomain,
		cfg.CookieSecure,
		true,
	)
}

func ClearAuthCookies(c *gin.Context, cfg *config.Config) {
	if cfg == nil {
		return
	}

	sameSite := parseCookieSameSite(cfg.CookieSameSite)
	c.SetSameSite(sameSite)
	c.SetCookie(AccessTokenCookieName, "", -1, "/", cfg.CookieDomain, cfg.CookieSecure, cfg.CookieHTTPOnly)
	c.SetCookie(RefreshTokenCookieName, "", -1, "/", cfg.CookieDomain, cfg.CookieSecure, true)
}

func ExtractAccessTokenFromRequest(c *gin.Context) (string, string) {
	return extractTokenFromRequest(c, AccessTokenCookieName)
}

func ExtractRefreshTokenFromRequest(c *gin.Context) (string, string) {
	return extractTokenFromCookie(c, RefreshTokenCookieName)
}

func extractTokenFromRequest(c *gin.Context, cookieName string) (string, string) {
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		token := strings.TrimSpace(authHeader[len("Bearer "):])
		if _, ignored := ignoredAuthorizationTokens[strings.ToLower(token)]; !ignored {
			return token, "header"
		}
	}

	return extractTokenFromCookie(c, cookieName)
}

func extractTokenFromCookie(c *gin.Context, cookieName string) (string, string) {
	token, err := c.Cookie(cookieName)
	if err != nil {
		return "", ""
	}

	token = strings.TrimSpace(token)
	if _, ignored := ignoredAuthorizationTokens[strings.ToLower(token)]; ignored {
		return "", ""
	}

	return token, "cookie"
}

func parseCookieSameSite(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "lax":
		return http.SameSiteLaxMode
	default:
		return http.SameSiteLaxMode
	}
}
